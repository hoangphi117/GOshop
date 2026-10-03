package v1handler

import (
	"errors"
	"fmt"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/middleware"
	"goshop/internal/utils"
	"goshop/pkg/storage"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UploadHandler struct {
	storage storage.StorageService
}

func NewUploadHandler(storageService storage.StorageService) *UploadHandler {
	return &UploadHandler{
		storage: storageService,
	}
}

// validateUploadedFile validates size, extension and detected MIME type.
func validateUploadedFile(fileHeader *multipart.FileHeader) (string, utils.MediaType, string, error) {
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))

	var mediaType utils.MediaType
	var maxSize int64

	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
		mediaType = utils.MediaTypeImage
		maxSize = utils.MaxImageSize
	case ".mp4", ".webm", ".mov":
		mediaType = utils.MediaTypeVideo
		maxSize = utils.MaxVideoSize
	default:
		return "", "", "", errors.New("unsupported file type (allowed: jpg, jpeg, png, webp, mp4, webm, mov)")
	}

	if fileHeader.Size > maxSize {
		return "", "", "", fmt.Errorf("file is too large (max %dMB for %s)", maxSize/(1<<20), mediaType)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", "", "", errors.New("cannot open file")
	}
	defer file.Close()

	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", "", "", errors.New("cannot read file")
	}

	mimeType := http.DetectContentType(buffer[:n])
	if mediaType == utils.MediaTypeImage {
		if mimeType != "image/jpeg" && mimeType != "image/png" && mimeType != "image/webp" {
			return "", "", "", fmt.Errorf("invalid image MIME type: %s", mimeType)
		}
	} else if mediaType == utils.MediaTypeVideo {
		if mimeType != "video/mp4" && mimeType != "video/webm" && mimeType != "video/quicktime" && mimeType != "application/octet-stream" {
			return "", "", "", fmt.Errorf("invalid video MIME type: %s", mimeType)
		}
	}

	fileName := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	return fileName, mediaType, mimeType, nil
}

// UploadSingleFile godoc
// @Summary      Upload single file
// @Description  Upload image or video file with optional destination folder to Object Storage (S3/MinIO/Local)
// @Tags         Upload
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        file formData file true "File to upload"
// @Param        folder formData string false "Destination folder" Enums(products, categories, banners, avatars, common, chat) default(common)
// @Success      200 {object} utils.APIResponse{data=v1dto.UploadResponseDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Router       /upload [post]
func (uh *UploadHandler) UploadSingleFile(ctx *gin.Context) {
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "File is required (form field: 'file')"))
		return
	}

	folder := strings.ToLower(strings.TrimSpace(ctx.DefaultPostForm("folder", "common")))

	// Check upload permissions based on User Role
	userRoleVal, exists := ctx.Get("user_role")
	role := middleware.RoleMember
	if exists {
		if r, ok := userRoleVal.(int32); ok {
			role = r
		}
	}
	isAdminOrMod := role == middleware.RoleAdmin || role == middleware.RoleModerator

	if (folder == "products" || folder == "categories" || folder == "banners") && !isAdminOrMod {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeForbidden, "Only admin and moderator can upload to product/category/banner folders"))
		return
	}

	allowedFolders := map[string]bool{
		"products":   true,
		"categories": true,
		"banners":    true,
		"avatars":    true,
		"common":     true,
		"chat":       true,
	}

	if !allowedFolders[folder] {
		folder = "common"
	}

	fileName, mediaType, mimeType, err := validateUploadedFile(fileHeader)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, err.Error()))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Cannot open uploaded file"))
		return
	}
	defer file.Close()

	publicURL, _, err := uh.storage.Upload(ctx.Request.Context(), folder, fileName, file, fileHeader.Size, mimeType)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeInternal, "Failed to upload file to storage: "+err.Error()))
		return
	}

	response := v1dto.UploadResponseDTO{
		FileName:  fileName,
		URL:       publicURL,
		Size:      fileHeader.Size,
		MediaType: string(mediaType),
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "File uploaded successfully", response)
}

// GetPresignedURL godoc
// @Summary      Get Direct-to-S3 Presigned Upload URL
// @Description  Generates a presigned URL allowing the client to upload files directly to S3 / Cloudflare R2
// @Tags         Upload
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        folder query string false "Destination folder" default(common)
// @Param        filename query string true "Original file name"
// @Param        content_type query string true "MIME type (e.g. image/jpeg)"
// @Success      200 {object} utils.APIResponse{data=map[string]any}
// @Router       /upload/presign [get]
func (uh *UploadHandler) GetPresignedURL(ctx *gin.Context) {
	folder := strings.ToLower(strings.TrimSpace(ctx.DefaultQuery("folder", "common")))
	filename := strings.TrimSpace(ctx.Query("filename"))
	contentType := strings.TrimSpace(ctx.Query("content_type"))

	if filename == "" {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "filename query parameter is required"))
		return
	}
	if contentType == "" {
		contentType = "application/octet-stream"
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
	})
}

