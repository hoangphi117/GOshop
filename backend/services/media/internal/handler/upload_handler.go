package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"goshop/pkg/utils"
	"goshop/pkg/logger"
	"goshop/pkg/middleware"
	"goshop/pkg/rabbitmq"
	"goshop/pkg/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UploadHandler struct {
	storage storage.StorageService
	mq      rabbitmq.RabbitMQService
}

func NewUploadHandler(storageService storage.StorageService, mq rabbitmq.RabbitMQService) *UploadHandler {
	return &UploadHandler{
		storage: storageService,
		mq:      mq,
	}
}

// publishMediaUploaded enqueues post-processing (resize/re-encode) for a
// confirmed upload. Best-effort: confirm must not fail because the queue is down.
func (uh *UploadHandler) publishMediaUploaded(ctx context.Context, fileKey string, size int64) {
	if uh.mq == nil {
		return
	}
	evt := rabbitmq.MediaUploadedEvent{FileKey: fileKey, Size: size}
	if err := uh.mq.Publish(ctx, rabbitmq.QueueMediaUploaded, evt); err != nil {
		logger.Log.Warn().Err(err).Str("file_key", fileKey).Msg("Failed to publish media.uploaded event")
	}
}

// PresignRequest represents payload for direct presigned upload URL generation.
type PresignRequest struct {
	Folder      string `json:"folder" form:"folder"`
	Filename    string `json:"filename" form:"filename"`
	ContentType string `json:"content_type" form:"content_type"`
}

// GetPresignedURL godoc
// @Summary      Get Direct-to-S3 Presigned Upload URL
// @Description  Generates a presigned URL allowing the client to upload files directly to S3 / Cloudflare R2
// @Tags         Upload
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        request body PresignRequest false "Presign request body"
// @Param        folder query string false "Destination folder" default(common)
// @Param        filename query string false "Original file name"
// @Param        content_type query string false "MIME type (e.g. image/jpeg)"
// @Success      200 {object} utils.APIResponse{data=map[string]any}
// @Router       /upload/presign [get]
// @Router       /upload/presign [post]
func (uh *UploadHandler) GetPresignedURL(ctx *gin.Context) {
	var req PresignRequest
	if ctx.Request.Method == http.MethodPost && ctx.Request.Header.Get("Content-Type") != "" {
		_ = ctx.ShouldBind(&req)
	}

	folder := req.Folder
	if folder == "" {
		folder = ctx.DefaultQuery("folder", "")
	}
	folder = strings.ToLower(strings.TrimSpace(folder))

	// Presign is stricter than a fallback: an invalid folder is an error, not a
	// silent fallback, otherwise a typo lands objects under an unowned prefix.
	if !allowedFolders[folder] {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "invalid folder (allowed: products, categories, banners, avatars, reviews, returns, common, chat)"))
		return
	}
	if err := checkFolderPermission(ctx, folder); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	filename := req.Filename
	if filename == "" {
		filename = ctx.Query("filename")
	}
	filename = strings.TrimSpace(filename)

	contentType := req.ContentType
	if contentType == "" {
		contentType = ctx.Query("content_type")
	}
	contentType = strings.TrimSpace(contentType)

	if filename == "" {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "filename is required"))
		return
	}

	// The presigned PUT goes straight to storage, so the allowlist
	// here is the only gate before bytes reach the bucket.
	allowedContentType, maxSize, ok := uploadTypeForExt(filepath.Ext(filename))
	if !ok {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "unsupported file type (allowed: jpg, jpeg, png, webp, mp4, webm, mov)"))
		return
	}
	if contentType == "" {
		contentType = allowedContentType
	}
	if contentType != allowedContentType {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("content_type %q does not match extension (expected %q)", contentType, allowedContentType)))
		return
	}

	ext := strings.ToLower(filepath.Ext(filename))
	uniqueFileName := fmt.Sprintf("%s%s", uuid.New().String(), ext)

	uploadURL, publicURL, fileKey, err := uh.storage.GeneratePresignedUploadURL(ctx.Request.Context(), folder, uniqueFileName, contentType, 15*time.Minute)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeInternal, "Failed to generate presigned upload URL: "+err.Error()))
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Presigned upload URL generated successfully", gin.H{
		"upload_url":   uploadURL,
		"public_url":   publicURL,
		"file_key":     fileKey,
		"expires_in":   900,
		"content_type": contentType,
		"max_size":     maxSize,
	})
}

// sniffMagicBytes reads the first 512 bytes of an object and returns the MIME
// type http.DetectContentType infers from the file signature.
func sniffMagicBytes(ctx context.Context, svc storage.StorageService, fileKey string) (string, error) {
	reader, err := svc.RangeReader(ctx, fileKey, 512)
	if err != nil {
		return "", err
	}
	defer func() { _ = reader.Close() }()

	head := make([]byte, 512)
	n, err := io.ReadFull(reader, head)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", fmt.Errorf("cannot read object head: %w", err)
	}
	if n == 0 {
		return "", fmt.Errorf("object %q is empty", fileKey)
	}
	return http.DetectContentType(head[:n]), nil
}

// magicMatchesExt reports whether a sniffed MIME type is acceptable for the
// extension. Video containers are frequently detected as octet-stream (moov atom
// later in the file), so octet-stream is tolerated for video only.
func magicMatchesExt(ext, sniffed string) bool {
	expected, _, ok := uploadTypeForExt(ext)
	if !ok {
		return false
	}
	if sniffed == expected {
		return true
	}
	if strings.HasPrefix(expected, "video/") && sniffed == "application/octet-stream" {
		return true
	}
	return false
}

// allowedFolders is the closed set of bucket prefixes clients may target.
var allowedFolders = map[string]bool{
	"products":   true,
	"categories": true,
	"banners":    true,
	"avatars":    true,
	"reviews":    true,
	"returns":    true,
	"common":     true,
	"chat":       true,
}

// adminOnlyFolders can only be written by admins/moderators — they render on
// public storefront pages, so any user must not be able to plant content there.
var adminOnlyFolders = map[string]bool{
	"products":   true,
	"categories": true,
	"banners":    true,
}

// checkFolderPermission validates the requested folder and enforces the admin
// gate. Unknown folders fall back to "common". Shared by presign
// so direct uploads cannot bypass the role checks.
func checkFolderPermission(ctx *gin.Context, folder string) error {
	if !allowedFolders[folder] {
		return nil // caller proceeds with "common" (folder is normalized by the sender)
	}

	isAdminOrMod := false
	if userRoleVal, exists := ctx.Get("user_role"); exists {
		if role, ok := userRoleVal.(int32); ok {
			isAdminOrMod = role == middleware.RoleAdmin || role == middleware.RoleModerator
		}
	}

	if adminOnlyFolders[folder] && !isAdminOrMod {
		return utils.NewError(utils.ErrCodeForbidden, "Only admin and moderator can upload to product/category/banner folders")
	}
	return nil
}

// Upload size caps enforced at presign and re-checked at confirm.
const (
	maxImageSize = 5 << 20
	maxVideoSize = 30 << 20
)

// uploadTypeForExt maps a file extension to its enforced content type and size cap.
// Direct uploads cannot be magic-byte scanned server-side, so extension and
// content type must match exactly at presign time and are re-checked at confirm.
func uploadTypeForExt(ext string) (contentType string, maxSize int64, ok bool) {
	switch strings.ToLower(ext) {
	case ".jpg", ".jpeg":
		return "image/jpeg", maxImageSize, true
	case ".png":
		return "image/png", maxImageSize, true
	case ".webp":
		return "image/webp", maxImageSize, true
	case ".mp4":
		return "video/mp4", maxVideoSize, true
	case ".webm":
		return "video/webm", maxVideoSize, true
	case ".mov":
		return "video/quicktime", maxVideoSize, true
	default:
		return "", 0, false
	}
}

// ConfirmUploadRequest is the payload the client sends once the direct PUT to
// storage succeeded.
type ConfirmUploadRequest struct {
	FileKey string `json:"file_key" binding:"required"`
}

// ConfirmUploadResponseDTO reports the verified object.
type ConfirmUploadResponseDTO struct {
	FileKey     string `json:"file_key"`
	PublicURL   string `json:"public_url"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// ConfirmUpload godoc
// @Summary      Confirm a direct-to-storage upload
// @Description  HEADs the object in storage and returns its verified URL. Call this after the presigned PUT succeeds; the domain API that stores the URL must come from this response, not from client input.
// @Tags         Upload
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        request body ConfirmUploadRequest true "Confirmed upload"
// @Success      200 {object} utils.APIResponse{data=ConfirmUploadResponseDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /upload/confirm [post]
func (uh *UploadHandler) ConfirmUpload(ctx *gin.Context) {
	var req ConfirmUploadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "file_key is required"))
		return
	}

	fileKey := strings.Trim(strings.TrimSpace(req.FileKey), "/")
	if fileKey == "" || strings.Contains(fileKey, "..") {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "invalid file_key"))
		return
	}

	expectedContentType, maxSize, ok := uploadTypeForExt(filepath.Ext(fileKey))
	if !ok {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "unsupported file key extension"))
		return
	}

	size, actualContentType, err := uh.storage.Stat(ctx.Request.Context(), fileKey)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeNotFound, "Uploaded object not found in storage"))
		return
	}
	if size <= 0 {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Uploaded object is empty"))
		return
	}
	if size > maxSize {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Uploaded object is too large (max %dMB)", maxSize/(1<<20))))
		return
	}
	if actualContentType != "" && actualContentType != expectedContentType && actualContentType != "application/octet-stream" {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Uploaded object content type %q does not match extension (expected %q)", actualContentType, expectedContentType)))
		return
	}

	// The presigned PUT put arbitrary bytes in the bucket under our key, so
	// sniff the leading bytes: a rename cannot fake the file signature.
	sniffed, err := sniffMagicBytes(ctx, uh.storage, fileKey)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Uploaded object failed content verification"))
		return
	}
	if !magicMatchesExt(filepath.Ext(fileKey), sniffed) {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Uploaded object content does not match its file type"))
		return
	}

	uh.publishMediaUploaded(ctx, fileKey, size)

	utils.ResponseSuccess(ctx, http.StatusOK, "Upload confirmed", ConfirmUploadResponseDTO{
		FileKey:     fileKey,
		PublicURL:   uh.storage.PublicURL(fileKey),
		ContentType: expectedContentType,
		Size:        size,
	})
}
