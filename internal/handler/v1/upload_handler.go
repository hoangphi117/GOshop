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

// Allow users to upload files to an optional directory
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

	if (folder == "products" || folder == "categories") && !isAdminOrMod {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeForbidden, "Only admin and moderator can upload to product/category folders"))
		return
	}

	allowedFolders := map[string]bool{
		"products":   true,
		"categories": true,
		"avatars":    true,
		"common":     true,
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
