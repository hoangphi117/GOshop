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
	Endpoint        string // Custom endpoint URL (e.g. http://minio:9000 or https://<account-id>.r2.cloudflarestorage.com)
	Region          string // AWS Region (e.g. ap-southeast-1 or auto for R2/MinIO)
	Bucket          string // S3 Bucket name
	AccessKey       string // S3 Access Key / Cloudflare R2 Access Key ID
	SecretKey       string // S3 Secret Key / Cloudflare R2 Secret Access Key
	PublicBaseURL   string // Public CDN base URL (e.g. https://cdn.example.com or http://localhost:9000/goshop-media)
	UsePathStyle    bool   // Set true for MinIO, false for AWS S3 / Cloudflare R2
	ForceSSL        bool   // Use HTTPS
}

type s3StorageService struct {
	client         *s3.Client
	presignClient  *s3.PresignClient
	cfg            S3Config
	logger         *zerolog.Logger
}

// NewS3StorageService initializes an S3-compatible storage service client.
func NewS3StorageService(ctx context.Context, cfg S3Config, logger *zerolog.Logger) (StorageService, error) {
	if cfg.Region == "" {
		cfg.Region = "auto"
	}

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if cfg.Endpoint != "" {
			return aws.Endpoint{
				PartitionID:       "aws",
				URL:               cfg.Endpoint,
				SigningRegion:     cfg.Region,
				HostnameImmutable: cfg.UsePathStyle,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load S3 SDK configuration: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})

	presignClient := s3.NewPresignClient(client)

	return &s3StorageService{
		client:        client,
		presignClient: presignClient,
		cfg:           cfg,
		logger:        logger,
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
		ACL:           types.ObjectCannedACLPublicRead, // ponytail: default public-read, override via presign if private bucket
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		if s.logger != nil {
			s.logger.Error().Err(err).Str("bucket", s.cfg.Bucket).Str("key", fileKey).Msg("Failed to upload object to S3")
		}
		return "", "", fmt.Errorf("failed to upload object to storage: %w", err)
	}

	publicURL := s.buildPublicURL(fileKey)
	return publicURL, fileKey, nil
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
			s.logger.Warn().Err(err).Str("bucket", s.cfg.Bucket).Str("key", cleanKey).Msg("Failed to delete object from S3")
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

func (s *s3StorageService) IsHealthy(ctx context.Context) bool {
	ctxTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.client.HeadBucket(ctxTimeout, &s3.HeadBucketInput{
		Bucket: aws.String(s.cfg.Bucket),
	})
	return err == nil
}
