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
	"github.com/google/uuid"
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

func (rh *ReviewHandler) GetProductReviews(ctx *gin.Context) {
	var param v1dto.ProductReviewsParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	productUUID, err := uuid.Parse(param.ProductUUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID"))
		return
	}

	var query v1dto.GetReviewsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviews, total, err := rh.service.GetProductReviews(ctx, productUUID, query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}

	paginationResponse := utils.NewPaginationResponse(reviews, page, limit, total)
	utils.ResponseSuccess(ctx, http.StatusOK, "Product reviews retrieved successfully", paginationResponse)
}

func (rh *ReviewHandler) GetProductRatingSummary(ctx *gin.Context) {
	var param v1dto.ProductReviewsParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	productUUID, err := uuid.Parse(param.ProductUUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID"))
		return
	}

	summary, err := rh.service.GetProductRatingSummary(ctx, productUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Get rating summary successfully", summary)
}

func (rh *ReviewHandler) GetMyReviews(ctx *gin.Context) {
	var query v1dto.GetMyReviewsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviews, total, err := rh.service.GetMyReviews(ctx, query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}

	paginationResponse := utils.NewPaginationResponse(reviews, page, limit, total)
	utils.ResponseSuccess(ctx, http.StatusOK, "Get my reviews successfully", paginationResponse)
}

func (rh *ReviewHandler) UpdateReview(ctx *gin.Context) {
	var param v1dto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	var input v1dto.UpdateReviewInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	updatedReview, err := rh.service.UpdateReview(ctx, reviewUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Review updated successfully", updatedReview)
}

func (rh *ReviewHandler) DeleteReview(ctx *gin.Context) {
	var param v1dto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	if err := rh.service.DeleteReview(ctx, reviewUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Review deleted successfully", nil)
}

func (rh *ReviewHandler) UpdateReviewStatus(ctx *gin.Context) {
	var param v1dto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	var input v1dto.UpdateReviewStatusInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	updatedReview, err := rh.service.UpdateReviewStatus(ctx, reviewUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Review status updated successfully", updatedReview)
}

func (rh *ReviewHandler) AdminGetAllReviews(ctx *gin.Context) {
	var query v1dto.AdminGetReviewsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviews, total, err := rh.service.AdminGetAllReviews(ctx, query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}

	paginationResponse := utils.NewPaginationResponse(reviews, page, limit, total)
	utils.ResponseSuccess(ctx, http.StatusOK, "Admin reviews retrieved successfully", paginationResponse)
}

func (rh *ReviewHandler) AdminReplyReview(ctx *gin.Context) {
	var param v1dto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	var input v1dto.ReplyReviewInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	updatedReview, err := rh.service.AdminReplyReview(ctx, reviewUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Reply posted successfully", updatedReview)
}

func (rh *ReviewHandler) ToggleHelpfulVote(ctx *gin.Context) {
	var param v1dto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	response, err := rh.service.ToggleHelpfulVote(ctx, reviewUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Helpful vote toggled successfully", response)
}

func (rh *ReviewHandler) ReportReview(ctx *gin.Context) {
	var param v1dto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	var input v1dto.ReportReviewInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if err := rh.service.ReportReview(ctx, reviewUUID, input); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Review reported successfully", nil)
}

func (rh *ReviewHandler) AdminForceDeleteReview(ctx *gin.Context) {
	var param v1dto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}
	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	if err := rh.service.AdminForceDeleteReview(ctx, reviewUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Review permanently deleted by admin", nil)
}
