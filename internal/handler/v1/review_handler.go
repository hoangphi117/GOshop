package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

type ReviewHandler struct {
	service v1service.ReviewService
}

func NewReviewHandler(service v1service.ReviewService) *ReviewHandler {
	return &ReviewHandler{
		service: service,
	}
}

func (rh *ReviewHandler) CreateReview(ctx *gin.Context) {
	var input v1dto.CreateReviewInput
	if err := ctx.ShouldBind(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	form, _ := ctx.MultipartForm()
	var imageURLs []string
	var videoURL *string
	var savedFiles []string // rollback if error occurs in DB

	uploadDir := "./uploads/reviews/"

	if form != nil && form.File["files"] != nil {
		files := form.File["files"]

		if len(files) > 6 {
			utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "You can upload a maximum of 5 images and 1 video"))
			return
		}

		for _, fileHeader := range files {
			fileName, mediaType, err := utils.ValidateAndSaveFile(fileHeader, uploadDir)
			if err != nil {
				rh.cleanupFiles(savedFiles, uploadDir)
				utils.ResponseError(ctx, err)
				return
			}
			savedFiles = append(savedFiles, fileName)
			fileRelativePath := "/uploads/reviews/" + fileName

			switch mediaType {
			case "image":
				if len(imageURLs) >= 5 {
					rh.cleanupFiles(savedFiles, uploadDir)
					utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "You can upload a maximum of 5 images"))
					return
				}
				imageURLs = append(imageURLs, fileRelativePath)
			case "video":
				if videoURL != nil {
					rh.cleanupFiles(savedFiles, uploadDir)
					utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "You can upload a maximum of 1 video"))
					return
				}
				videoURL = &fileRelativePath
			}
		}
	}

	review, err := rh.service.CreateReview(ctx, input, imageURLs, videoURL)
	if err != nil {
		rh.cleanupFiles(savedFiles, uploadDir)
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Review created successfully", review)
}

func (h *ReviewHandler) cleanupFiles(fileNames []string, uploadDir string) {
	for _, name := range fileNames {
		_ = os.Remove(filepath.Join(uploadDir, name))
	}
}
