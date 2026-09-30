package v1handler

import (
	"fmt"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/middleware"
	"goshop/internal/utils"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct{}

func NewUploadHandler() *UploadHandler {
	return &UploadHandler{}
}

// UploadSingleFile godoc
// @Summary      Upload single file
// @Description  Upload image or video file with optional destination folder
// @Tags         Upload
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        file formData file true "File to upload"
// @Param        folder formData string false "Destination folder" Enums(products, categories, banners, avatars, common) default(common)
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

	// Kiểm tra quyền hạn theo Role người dùng (RoleAdmin, RoleModerator, RoleMember)
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

	uploadDir := filepath.Join(".", "uploads", folder)

	fileName, mediaType, err := utils.ValidateAndSaveFile(fileHeader, uploadDir)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, err.Error()))
		return
	}

	publicURL := fmt.Sprintf("/uploads/%s/%s", folder, fileName)

	response := v1dto.UploadResponseDTO{
		FileName:  fileName,
		URL:       publicURL,
		Size:      fileHeader.Size,
		MediaType: string(mediaType),
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "File uploaded successfully", response)
}
