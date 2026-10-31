package handler

import (
	"net/http"

	"goshop/pkg/middleware"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	reviewdto "goshop/services/review/internal/dto"
	"goshop/services/review/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ReviewHandler struct {
	service service.ReviewService
}

func NewReviewHandler(service service.ReviewService) *ReviewHandler {
	return &ReviewHandler{
		service: service,
	}
}

// CreateReview godoc
// @Summary      Create review
// @Description  Create a product review with rating, comments, images and video
// @Tags         Reviews
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        request body reviewdto.CreateReviewInput true "Review request body"
// @Success      201 {object} utils.APIResponse{data=reviewdto.ReviewDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /reviews [post]
func (rh *ReviewHandler) CreateReview(ctx *gin.Context) {
	userID, err := middleware.MustGetUserID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var input reviewdto.CreateReviewInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if len(input.Images) > 5 {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "You can upload a maximum of 5 images"))
		return
	}

	review, err := rh.service.CreateReview(ctx.Request.Context(), userID, input, input.Images, input.VideoURL)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Review created successfully", review)
}

// GetProductReviews godoc
// @Summary      Get product reviews
// @Description  Get paginated reviews for a specific product
// @Tags         Reviews
// @Produce      json
// @Param        uuid path string true "Product UUID"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Param        rating query int false "Filter rating" Enums(1, 2, 3, 4, 5)
// @Param        has_media query bool false "Filter reviews with photos/videos"
// @Param        order_by query string false "Order by field" Enums(created_at, rating, helpful)
// @Param        sort query string false "Sort order" Enums(asc, desc)
// @Success      200 {object} utils.APIResponse{data=[]reviewdto.ReviewDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /products/{uuid}/reviews [get]
func (rh *ReviewHandler) GetProductReviews(ctx *gin.Context) {
	var param reviewdto.ProductReviewsParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	productUUID, err := uuid.Parse(param.ProductUUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID"))
		return
	}

	var query reviewdto.GetReviewsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviews, total, err := rh.service.GetProductReviews(ctx.Request.Context(), productUUID, query)
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

// GetProductRatingSummary godoc
// @Summary      Get product rating summary
// @Description  Get star distribution and average rating for a product
// @Tags         Reviews
// @Produce      json
// @Param        uuid path string true "Product UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Router       /products/{uuid}/reviews/summary [get]
func (rh *ReviewHandler) GetProductRatingSummary(ctx *gin.Context) {
	var param reviewdto.ProductReviewsParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	productUUID, err := uuid.Parse(param.ProductUUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID"))
		return
	}

	summary, err := rh.service.GetProductRatingSummary(ctx.Request.Context(), productUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Get rating summary successfully", summary)
}

// GetMyReviews godoc
// @Summary      Get current user reviews
// @Description  Retrieve paginated list of reviews authored by the authenticated user
// @Tags         Reviews
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Success      200 {object} utils.APIResponse{data=[]reviewdto.MyReviewDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /reviews/me [get]
func (rh *ReviewHandler) GetMyReviews(ctx *gin.Context) {
	userID, err := middleware.MustGetUserID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var query reviewdto.GetMyReviewsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviews, total, err := rh.service.GetMyReviews(ctx.Request.Context(), userID, query)
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

// UpdateReview godoc
// @Summary      Update review
// @Description  Update rating, title, or comment of an existing review
// @Tags         Reviews
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Review UUID"
// @Param        body body reviewdto.UpdateReviewInput true "Review update payload"
// @Success      200 {object} utils.APIResponse{data=reviewdto.ReviewDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /reviews/{uuid} [put]
func (rh *ReviewHandler) UpdateReview(ctx *gin.Context) {
	userID, err := middleware.MustGetUserID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var param reviewdto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	var input reviewdto.UpdateReviewInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	updatedReview, err := rh.service.UpdateReview(ctx.Request.Context(), userID, reviewUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Review updated successfully", updatedReview)
}

// DeleteReview godoc
// @Summary      Delete review
// @Description  Delete own review by review UUID
// @Tags         Reviews
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Review UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /reviews/{uuid} [delete]
func (rh *ReviewHandler) DeleteReview(ctx *gin.Context) {
	userID, err := middleware.MustGetUserID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var param reviewdto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	if err := rh.service.DeleteReview(ctx.Request.Context(), userID, reviewUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Review deleted successfully", nil)
}

// UpdateReviewStatus godoc
// @Summary      Update review status
// @Description  Change review visibility (1: Visible, 2: Hidden, 3: Reported) (Admin/Moderator)
// @Tags         Admin - Reviews
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Review UUID"
// @Param        body body reviewdto.UpdateReviewStatusInput true "Status payload"
// @Success      200 {object} utils.APIResponse{data=reviewdto.ReviewDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/reviews/{uuid}/status [patch]
func (rh *ReviewHandler) UpdateReviewStatus(ctx *gin.Context) {
	var param reviewdto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	var input reviewdto.UpdateReviewStatusInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	updatedReview, err := rh.service.UpdateReviewStatus(ctx.Request.Context(), reviewUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Review status updated successfully", updatedReview)
}

// AdminGetAllReviews godoc
// @Summary      Get all reviews
// @Description  Retrieve all reviews with filters for moderation (Admin/Moderator)
// @Tags         Admin - Reviews
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Param        status query int false "Review status (1: Visible, 2: Hidden, 3: Reported)" Enums(1, 2, 3)
// @Param        rating query int false "Filter rating" Enums(1, 2, 3, 4, 5)
// @Param        product_uuid query string false "Product UUID"
// @Param        search query string false "Search query"
// @Param        order_by query string false "Order by" Enums(created_at, rating)
// @Param        sort query string false "Sort order" Enums(asc, desc)
// @Success      200 {object} utils.APIResponse{data=[]reviewdto.ReviewDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/reviews [get]
func (rh *ReviewHandler) AdminGetAllReviews(ctx *gin.Context) {
	var query reviewdto.AdminGetReviewsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviews, total, err := rh.service.AdminGetAllReviews(ctx.Request.Context(), query)
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

// AdminReplyReview godoc
// @Summary      Reply to review
// @Description  Add official store reply to customer review (Admin/Moderator)
// @Tags         Admin - Reviews
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Review UUID"
// @Param        body body reviewdto.ReplyReviewInput true "Reply payload"
// @Success      200 {object} utils.APIResponse{data=reviewdto.ReviewDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/reviews/{uuid}/reply [patch]
func (rh *ReviewHandler) AdminReplyReview(ctx *gin.Context) {
	var param reviewdto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	var input reviewdto.ReplyReviewInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	updatedReview, err := rh.service.AdminReplyReview(ctx.Request.Context(), reviewUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Reply posted successfully", updatedReview)
}

// ToggleHelpfulVote godoc
// @Summary      Toggle helpful vote
// @Description  Vote or unvote that a review was helpful
// @Tags         Reviews
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Review UUID"
// @Success      200 {object} utils.APIResponse{data=reviewdto.ToggleHelpfulResponseDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /reviews/{uuid}/helpful [post]
func (rh *ReviewHandler) ToggleHelpfulVote(ctx *gin.Context) {
	userID, err := middleware.MustGetUserID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var param reviewdto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	response, err := rh.service.ToggleHelpfulVote(ctx.Request.Context(), userID, reviewUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Helpful vote toggled successfully", response)
}

// ReportReview godoc
// @Summary      Report review
// @Description  Report a review for spam, inappropriate content, etc.
// @Tags         Reviews
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Review UUID"
// @Param        body body reviewdto.ReportReviewInput true "Report payload"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /reviews/{uuid}/report [post]
func (rh *ReviewHandler) ReportReview(ctx *gin.Context) {
	userID, err := middleware.MustGetUserID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var param reviewdto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	var input reviewdto.ReportReviewInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if err := rh.service.ReportReview(ctx.Request.Context(), userID, reviewUUID, input); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Review reported successfully", nil)
}

// AdminForceDeleteReview godoc
// @Summary      Force delete review
// @Description  Permanently delete review from database (Admin/Moderator)
// @Tags         Admin - Reviews
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Review UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/reviews/{uuid} [delete]
func (rh *ReviewHandler) AdminForceDeleteReview(ctx *gin.Context) {
	var param reviewdto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}
	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	if err := rh.service.AdminForceDeleteReview(ctx.Request.Context(), reviewUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Review permanently deleted by admin", nil)
}

// GetReviewReports godoc
// @Summary      Get review reports
// @Description  Get all user reports filed against a specific review (Admin/Moderator)
// @Tags         Admin - Reviews
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Review UUID"
// @Success      200 {object} utils.APIResponse{data=[]reviewdto.ReviewReportDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/reviews/{uuid}/reports [get]
func (rh *ReviewHandler) GetReviewReports(ctx *gin.Context) {
	var param reviewdto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}
	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	reports, err := rh.service.GetReviewReports(ctx.Request.Context(), reviewUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Review reports retrieved successfully", reports)
}

// AdminDismissReports godoc
// @Summary      Dismiss review reports
// @Description  Dismiss all reports for a review and restore its visibility (Admin/Moderator)
// @Tags         Admin - Reviews
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Review UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/reviews/{uuid}/dismiss-reports [post]
func (rh *ReviewHandler) AdminDismissReports(ctx *gin.Context) {
	var param reviewdto.ReviewUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}
	reviewUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid review UUID"))
		return
	}

	if err := rh.service.AdminDismissReports(ctx.Request.Context(), reviewUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Review reports dismissed and visibility restored", nil)
}
