package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/rs/zerolog"
)

// S3Config holds configuration parameters for S3/MinIO/R2 providers.
type S3Config struct {
	Endpoint      string // Custom endpoint URL (e.g. http://minio:9000 or https://<account-id>.r2.cloudflarestorage.com)
	Region        string // AWS Region (e.g. ap-southeast-1 or auto for R2/MinIO)
	Bucket        string // S3 Bucket name
	AccessKey     string // S3 Access Key / Cloudflare R2 Access Key ID
	SecretKey     string // S3 Secret Key / Cloudflare R2 Secret Access Key
	PublicBaseURL string // Public CDN base URL (e.g. https://pub-xxx.r2.dev or http://localhost:9000/goshop-media)
	UsePathStyle  bool   // Set true for MinIO, false for AWS S3 / Cloudflare R2
	ForceSSL      bool   // Use HTTPS
}

type s3StorageService struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	cfg           S3Config
	logger        *zerolog.Logger
	isR2          bool
}

// presignEndpointFor derives the browser-reachable endpoint to sign upload
// URLs with so the browser connects to the PUBLIC host, not the compose-
// internal endpoint (minio:9000 does not resolve outside Docker). The SigV4
// signature covers Host + URI, so nothing downstream may rewrite them: the
// public proxy must forward the path verbatim (no rewrite) and pass Host
// through. The SDK appends /<bucket>/<key> itself (path-style), so for
// MinIO/self-hosted behind an edge the endpoint is the public HOST only:
// PublicBaseURL "http://host[/anything]" -> "http://host".
// Cloudflare R2 presigning must go through its S3 API endpoint (the public
// dev domain only serves anonymous reads), which equals cfg.Endpoint.
func presignEndpointFor(cfg S3Config) string {
	if cfg.PublicBaseURL == "" || cfg.Bucket == "" ||
		strings.Contains(strings.ToLower(cfg.Endpoint), "r2.cloudflarestorage.com") {
		return cfg.Endpoint
	}
	base := strings.TrimRight(cfg.PublicBaseURL, "/")
	rest := base
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	if j := strings.Index(rest, "/"); j > 0 {
		return base[:len(base)-(len(rest)-j)]
	}
	return base
}

// NewS3StorageService initializes an S3-compatible storage service client.
func NewS3StorageService(ctx context.Context, cfg S3Config, logger *zerolog.Logger) (StorageService, error) {
	if cfg.Region == "" {
		cfg.Region = "auto"
	}

	isR2 := strings.Contains(strings.ToLower(cfg.Endpoint), "r2.cloudflarestorage.com")

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) { //nolint:staticcheck // Required by current AWS SDK compatibility layer
		if cfg.Endpoint != "" {
			return aws.Endpoint{ //nolint:staticcheck // Required by current AWS SDK compatibility layer
				PartitionID:       "aws",
				URL:               cfg.Endpoint,
				SigningRegion:     cfg.Region,
				HostnameImmutable: cfg.UsePathStyle,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{} //nolint:staticcheck // Required by current AWS SDK compatibility layer
	})

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		awsconfig.WithEndpointResolverWithOptions(customResolver), //nolint:staticcheck // Required by current AWS SDK compatibility layer
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load S3 SDK configuration: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})

	// Presigned URLs are consumed by the BROWSER, so they must be signed
	// against a host the browser can reach. cfg.Endpoint is usually a
	// compose-internal name (minio:9000) that does not resolve outside — the
	// signature covers the Host header, so the URL cannot simply be
	// rewritten client-side. When PublicBaseURL is set, derive an external
	// endpoint from it (e.g. http://localhost/minio/goshop-media ->
	// http://localhost/minio) and sign with that instead.
	presignEndpoint := presignEndpointFor(cfg)
	presignAWS := awsCfg
	if presignEndpoint != cfg.Endpoint && presignEndpoint != "" {
		pubResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) { //nolint:staticcheck // Required by current AWS SDK compatibility layer
			return aws.Endpoint{ //nolint:staticcheck // Required by current AWS SDK compatibility layer
				PartitionID:       "aws",
				URL:               presignEndpoint,
				SigningRegion:     cfg.Region,
				HostnameImmutable: true,
			}, nil
		})
		pubCfg, pubErr := awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(cfg.Region),
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
			awsconfig.WithEndpointResolverWithOptions(pubResolver), //nolint:staticcheck // Required by current AWS SDK compatibility layer
		)
		if pubErr != nil {
			return nil, fmt.Errorf("failed to load S3 SDK configuration for presigning: %w", pubErr)
		}
		presignAWS = pubCfg
	}

	presignClient := s3.NewPresignClient(s3.NewFromConfig(presignAWS, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	}))

	// For MinIO or self-hosted S3 endpoints, auto-create the bucket if it does not exist yet
	if cfg.Endpoint != "" && !isR2 && cfg.Bucket != "" {
		checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if _, headErr := client.HeadBucket(checkCtx, &s3.HeadBucketInput{Bucket: aws.String(cfg.Bucket)}); headErr != nil {
			if _, createErr := client.CreateBucket(checkCtx, &s3.CreateBucketInput{Bucket: aws.String(cfg.Bucket)}); createErr == nil && logger != nil {
				logger.Info().Str("bucket", cfg.Bucket).Msg("Auto-created S3/MinIO bucket successfully")
			}
		}
	}

	if logger != nil {
		logger.Info().
			Str("bucket", cfg.Bucket).
			Str("endpoint", cfg.Endpoint).
			Str("public_base_url", cfg.PublicBaseURL).
			Bool("is_r2", isR2).
			Msg("Initialized S3/R2 Storage Service successfully")
	}

	return &s3StorageService{
		client:        client,
		presignClient: presignClient,
		cfg:           cfg,
		logger:        logger,
		isR2:          isR2,
	}, nil
}

func (s *s3StorageService) buildKey(folder, filename string) string {
	cleanFolder := strings.Trim(strings.TrimSpace(folder), "/")
	cleanFilename := strings.Trim(strings.TrimSpace(filename), "/")
	if cleanFolder == "" {
		return cleanFilename
	}
	return path.Join(cleanFolder, cleanFilename)
}

// RangeReader fetches the first n bytes of an object so callers can sniff magic
// bytes without pulling the whole file (direct uploads never pass through us).
func (s *s3StorageService) RangeReader(ctx context.Context, fileKey string, n int64) (io.ReadCloser, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)

	cleanKey := strings.Trim(strings.TrimSpace(fileKey), "/")
	out, err := s.client.GetObject(ctxTimeout, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(cleanKey),
		Range:  aws.String(fmt.Sprintf("bytes=0-%d", n-1)),
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to read object head: %w", err)
	}
	// The body is consumed by the caller: cancel on Close, not on return, or
	// the stream is killed before a single byte is read.
	return &cancelOnCloseCloser{ReadCloser: out.Body, cancel: cancel}, nil
}

// Open streams the whole object for server-side processing (e.g. re-encoding).
func (s *s3StorageService) Open(ctx context.Context, fileKey string) (io.ReadCloser, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 60*time.Second)

	cleanKey := strings.Trim(strings.TrimSpace(fileKey), "/")
	out, err := s.client.GetObject(ctxTimeout, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(cleanKey),
	})
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to open object: %w", err)
	}
	return &cancelOnCloseCloser{ReadCloser: out.Body, cancel: cancel}, nil
}

// PublicURL resolves the public URL for a stored object key.
func (s *s3StorageService) PublicURL(fileKey string) string {
	return s.buildPublicURL(fileKey)
}

func (s *s3StorageService) buildPublicURL(fileKey string) string {
	if s.cfg.PublicBaseURL != "" {
		baseURL := strings.TrimRight(s.cfg.PublicBaseURL, "/")
		return fmt.Sprintf("%s/%s", baseURL, fileKey)
	}
	if s.cfg.Endpoint != "" {
		endpoint := strings.TrimRight(s.cfg.Endpoint, "/")
		if s.cfg.UsePathStyle {
			return fmt.Sprintf("%s/%s/%s", endpoint, s.cfg.Bucket, fileKey)
		}
		return fmt.Sprintf("%s/%s", endpoint, fileKey)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.cfg.Bucket, s.cfg.Region, fileKey)
}

func (s *s3StorageService) Upload(ctx context.Context, folder, filename string, reader io.Reader, size int64, contentType string) (string, string, error) {
	fileKey := s.buildKey(folder, filename)

	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.cfg.Bucket),
		Key:           aws.String(fileKey),
		Body:          reader,
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}

	// Cloudflare R2 does not support Object ACLs, only apply for standard S3 / MinIO
	if !s.isR2 {
		input.ACL = types.ObjectCannedACLPublicRead
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		if s.logger != nil {
			s.logger.Error().Err(err).Str("bucket", s.cfg.Bucket).Str("key", fileKey).Msg("Failed to upload object to S3/R2")
		}
		return "", "", fmt.Errorf("failed to upload object to storage: %w", err)
	}

	publicURL := s.buildPublicURL(fileKey)
	if s.logger != nil {
		s.logger.Info().Str("key", fileKey).Str("url", publicURL).Msg("Uploaded file successfully to S3/R2")
	}
	return publicURL, fileKey, nil
}

// cancelOnCloseCloser releases the request context when the response body is
// closed, so callers may stream the body after the accessor has returned.
type cancelOnCloseCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnCloseCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

func (s *s3StorageService) Delete(ctx context.Context, fileKey string) error {
	cleanKey := strings.Trim(strings.TrimSpace(fileKey), "/")
	if cleanKey == "" {
		return nil
	}

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(cleanKey),
	})
	if err != nil {
		if s.logger != nil {
			s.logger.Warn().Err(err).Str("bucket", s.cfg.Bucket).Str("key", cleanKey).Msg("Failed to delete object from S3/R2")
		}
		return fmt.Errorf("failed to delete object from storage: %w", err)
	}
	return nil
}

func (s *s3StorageService) GeneratePresignedUploadURL(ctx context.Context, folder, filename, contentType string, expire time.Duration) (string, string, string, error) {
	if expire <= 0 {
		expire = 15 * time.Minute
	}
	fileKey := s.buildKey(folder, filename)

	req, err := s.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.cfg.Bucket),
		Key:         aws.String(fileKey),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(expire))
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate presigned upload URL: %w", err)
	}

	publicURL := s.buildPublicURL(fileKey)
	return req.URL, publicURL, fileKey, nil
}

// Stat HEADs an object so callers can verify a direct upload actually landed.
func (s *s3StorageService) Stat(ctx context.Context, fileKey string) (int64, string, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cleanKey := strings.Trim(strings.TrimSpace(fileKey), "/")
	head, err := s.client.HeadObject(ctxTimeout, &s3.HeadObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(cleanKey),
	})
	if err != nil {
		s.logger.Warn().Err(err).Str("bucket", s.cfg.Bucket).Str("key", cleanKey).Msg("Object not found in storage")
		return 0, "", fmt.Errorf("object not found in storage: %w", err)
	}
	var contentType string
	if head.ContentType != nil {
		contentType = *head.ContentType
	}
	if head.ContentLength == nil {
		return 0, contentType, fmt.Errorf("object %q has no content length", cleanKey)
	}
	return *head.ContentLength, contentType, nil
}

func (s *s3StorageService) IsHealthy(ctx context.Context) bool {
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.client.HeadBucket(ctxTimeout, &s3.HeadBucketInput{
		Bucket: aws.String(s.cfg.Bucket),
	})
	return err == nil
}
