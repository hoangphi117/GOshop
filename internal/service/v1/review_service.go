package v1service

import (
	"context"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type reviewService struct {
	repo        repository.ReviewRepository
	userRepo    repository.UserRepository
	productRepo repository.ProductRepository
	orderRepo   repository.OrderRepository
	cache       cache.RedisCacheService
}

func NewReviewService(repo repository.ReviewRepository, userRepo repository.UserRepository, productRepo repository.ProductRepository, orderRepo repository.OrderRepository, redisClient *redis.Client) ReviewService {
	return &reviewService{
		repo:        repo,
		userRepo:    userRepo,
		productRepo: productRepo,
		orderRepo:   orderRepo,
		cache:       cache.NewRedisCacheService(redisClient),
	}
}

func (rs *reviewService) CreateReview(ctx *gin.Context, input v1dto.CreateReviewInput, imageURLs []string, videoURL *string) (v1dto.ReviewDTO, error) {
	reqCtx := ctx.Request.Context()

	userUUIDStr, ok := ctx.Get("user_uuid")
	if !ok {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID")
	}

	user, err := rs.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "User not found")
	}

	productUUID, err := uuid.Parse(input.ProductUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID")
	}

	product, err := rs.productRepo.GetByUUID(reqCtx, productUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeNotFound, "Product not found")
	}

	orderUUID, err := uuid.Parse(input.OrderUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID")
	}

	order, err := rs.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
	}

	isPurchased, err := rs.repo.CheckPurchased(reqCtx, order.OrderID, user.UserID, product.ProductID)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Error checking purchase status", err)
	}
	if !isPurchased {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "User has not purchased this product")
	}

	isExists, err := rs.repo.CheckExists(reqCtx, user.UserID, product.ProductID, order.OrderID)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Error checking review status", err)
	}
	if isExists {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Review already exists for this product")
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
	}
	defer tx.Rollback(txCtx)

	hasMedia := len(imageURLs) > 0 || videoURL != nil

	createParams := sqlc.CreateReviewParams{
		ReviewUserID:    user.UserID,
		ReviewProductID: product.ProductID,
		ReviewOrderID:   order.OrderID,
		ReviewRating:    int16(input.Rating),
		ReviewTitle:     input.Title,
		ReviewComment:   input.Comment,
		ReviewImages:    imageURLs,
		ReviewVideoUrl:  videoURL,
		ReviewHasMedia:  hasMedia,
	}

	review, err := rs.repo.Create(txCtx, tx, createParams)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to create review", err)
	}

	if err := rs.repo.RecalcRating(txCtx, tx, product.ProductID); err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to recalculate product rating", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	go func(pUUID string) {
		if err := rs.cache.Clear("reviews:product:" + pUUID + ":*"); err != nil {
			logger.Log.Warn().Err(err).Str("product_uuid", pUUID).Msg("Failed to clear product reviews cache")
		}
		if err := rs.cache.Del("reviews:summary:" + pUUID); err != nil {
			logger.Log.Warn().Err(err).Str("product_uuid", pUUID).Msg("Failed to clear review summary cache")
		}
		if err := rs.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear products cache")
		}
	}(input.ProductUUID)

	return v1dto.MapCreatedReviewToDTO(review, user.UserFullname, product.ProductUuid.String(), order.OrderUuid.String()), nil
}

func (rs *reviewService) GetProductReviews(ctx *gin.Context, productUUID uuid.UUID, query v1dto.GetReviewsQuery) ([]v1dto.ReviewDTO, int32, error) {
	reqCtx := ctx.Request.Context()

	page := query.Page
	if page <= 0 {
		page = 1
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}

	offset := (page - 1) * limit
	cacheKey := rs.generateReviewCacheKey(productUUID.String(), query.Rating, query.HasMedia, query.OrderBy, query.Sort, page, limit)
	var cacheData struct {
		Reviews []v1dto.ReviewDTO `json:"reviews"`
		Total   int32             `json:"total"`
	}

	if err := rs.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Reviews != nil {
		return cacheData.Reviews, cacheData.Total, nil
	}

	product, err := rs.productRepo.GetByUUID(reqCtx, productUUID)
	if err != nil {
		return nil, 0, utils.NewError(utils.ErrCodeNotFound, "Product not found")
	}

	var total int32
	if query.Rating == nil && query.HasMedia == nil {
		total = product.ProductReviewCount
	} else {
		count, err := rs.repo.CountByProduct(reqCtx, product.ProductID, query.Rating, query.HasMedia)
		if err != nil {
			return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count reviews", err)
		}
		total = int32(count)
	}

	reviewRows, err := rs.repo.GetByProduct(reqCtx, product.ProductID, query.Rating, query.HasMedia, query.OrderBy, query.Sort, limit, offset)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to get reviews", err)
	}

	reviewDTOs := make([]v1dto.ReviewDTO, len(reviewRows))
	for i, r := range reviewRows {
		images := r.ReviewImages
		if images == nil {
			images = []string{}
		}
		reviewDTOs[i] = v1dto.ReviewDTO{
			UUID:         r.ReviewUUID.String(),
			ReviewerName: utils.MaskName(r.ReviewerName),
			Rating:       int32(r.ReviewRating),
			Title:        r.ReviewTitle,
			Comment:      r.ReviewComment,
			Images:       images,
			VideoURL:     r.ReviewVideoURL,
			HasMedia:     r.ReviewHasMedia,
			CreatedAt:    r.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:    r.ReviewUpdatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	cacheData = struct {
		Reviews []v1dto.ReviewDTO `json:"reviews"`
		Total   int32             `json:"total"`
	}{
		Reviews: reviewDTOs,
		Total:   total,
	}

	_ = rs.cache.Set(cacheKey, cacheData, 5*time.Minute)

	return reviewDTOs, total, nil
}

func (rs *reviewService) generateReviewCacheKey(productUUID string, rating *int32, hasMedia *bool, orderBy, sort string, page, limit int32) string {
	ratingStr := "all"
	if rating != nil {
		ratingStr = fmt.Sprintf("%d", *rating)
	}
	mediaStr := "all"
	if hasMedia != nil {
		mediaStr = fmt.Sprintf("%t", *hasMedia)
	}
	return fmt.Sprintf("reviews:product:%s:rating:%s:media:%s:order:%s:sort:%s:p:%d:l:%d",
		productUUID, ratingStr, mediaStr, orderBy, sort, page, limit)
}

func (rs *reviewService) GetProductRatingSummary(ctx *gin.Context, productUUID uuid.UUID) (v1dto.RatingSummaryRow, error) {
	reqCtx := ctx.Request.Context()

	cacheKey := "reviews:summary:" + productUUID.String()
	var summaryDTO v1dto.RatingSummaryRow
	if err := rs.cache.Get(cacheKey, &summaryDTO); err == nil {
		return summaryDTO, nil
	}

	product, err := rs.productRepo.GetByUUID(reqCtx, productUUID)
	if err != nil {
		return v1dto.RatingSummaryRow{}, utils.NewError(utils.ErrCodeNotFound, "Product not found")
	}

	summary, err := rs.repo.GetRatingSummary(reqCtx, product.ProductID)
	if err != nil {
		return v1dto.RatingSummaryRow{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get rating summary", err)
	}

	summaryDTO = v1dto.RatingSummaryRow{
		TotalReviews:   summary.TotalReviews,
		AvgRating:      summary.AvgRating,
		WithMediaCount: summary.WithMediaCount,
		FiveStar:       summary.FiveStar,
		FourStar:       summary.FourStar,
		ThreeStar:      summary.ThreeStar,
		TwoStar:        summary.TwoStar,
		OneStar:        summary.OneStar,
	}

	_ = rs.cache.Set(cacheKey, summaryDTO, 5*time.Minute)

	return summaryDTO, nil
}

func (rs *reviewService) GetMyReviews(ctx *gin.Context, query v1dto.GetMyReviewsQuery) ([]v1dto.MyReviewDTO, int32, error) {
	reqCtx := ctx.Request.Context()
	userUUIDStr, ok := ctx.Get("user_uuid")
	if !ok {
		return nil, 0, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}
	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return nil, 0, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID")
	}
	user, err := rs.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return nil, 0, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}
	page := query.Page
	if page <= 0 {
		page = 1
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}

	offset := (page - 1) * limit

	total, err := rs.repo.CountByUserID(reqCtx, user.UserID)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count user reviews", err)
	}
	if total == 0 {
		return []v1dto.MyReviewDTO{}, 0, nil
	}

	rows, err := rs.repo.GetByUserID(reqCtx, user.UserID, limit, offset)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to get user reviews", err)
	}

	return v1dto.MapToMyReviewDTOs(rows), int32(total), nil
}

func (rs *reviewService) UpdateReview(ctx *gin.Context, reviewUUID uuid.UUID, input v1dto.UpdateReviewInput) (v1dto.ReviewDTO, error) {
	reqCtx := ctx.Request.Context()

	userUUIDStr, ok := ctx.Get("user_uuid")
	if !ok {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID")
	}

	user, err := rs.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "User not found")
	}

	review, err := rs.repo.GetByUUID(reqCtx, reviewUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeNotFound, "Review not found")
	}

	if review.ReviewUserID != user.UserID {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeForbidden, "You do not have permission to update this review")
	}

	if review.ReviewEditCount >= 1 {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "You can only edit your review once")
	}

	const maxEditDuration = 30 * 24 * time.Hour
	if time.Since(review.ReviewCreatedAt) > maxEditDuration {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "You can only edit your review within 30 days of posting")
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
	}
	defer tx.Rollback(txCtx)

	updateParams := sqlc.UpdateReviewParams{
		ReviewID:      review.ReviewID,
		ReviewRating:  int16(input.Rating),
		ReviewTitle:   input.Title,
		ReviewComment: input.Comment,
	}

	updatedReview, err := rs.repo.Update(txCtx, tx, updateParams)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update review", err)
	}

	if err := rs.repo.RecalcRating(txCtx, tx, review.ReviewProductID); err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to recalculate product rating", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	// Async clear cache
	go func(pUUID string) {
		_ = rs.cache.Clear("reviews:product:" + pUUID + ":*")
		_ = rs.cache.Del("reviews:summary:" + pUUID)
		_ = rs.cache.Clear("products:*")
	}(review.ProductUuid.String())

	images := updatedReview.ReviewImages
	if images == nil {
		images = []string{}
	}

	return v1dto.ReviewDTO{
		UUID:         updatedReview.ReviewUuid.String(),
		ReviewerName: user.UserFullname,
		Rating:       int32(updatedReview.ReviewRating),
		Title:        updatedReview.ReviewTitle,
		Comment:      updatedReview.ReviewComment,
		Images:       images,
		VideoURL:     updatedReview.ReviewVideoUrl,
		HasMedia:     updatedReview.ReviewHasMedia,
		CanEdit:      false,
		CreatedAt:    updatedReview.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    updatedReview.ReviewUpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (rs *reviewService) DeleteReview(ctx *gin.Context, reviewUUID uuid.UUID) error {
	reqCtx := ctx.Request.Context()

	userUUIDStr, ok := ctx.Get("user_uuid")
	if !ok {
		return utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID")
	}

	user, err := rs.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return utils.NewError(utils.ErrCodeUnauthorized, "User not found")
	}

	review, err := rs.repo.GetByUUID(reqCtx, reviewUUID)
	if err != nil {
		return utils.NewError(utils.ErrCodeNotFound, "Review not found")
	}

	if review.ReviewUserID != user.UserID {
		return utils.NewError(utils.ErrCodeForbidden, "You do not have permission to delete this review")
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
	}
	defer tx.Rollback(txCtx)

	if err := rs.repo.Delete(txCtx, tx, review.ReviewID); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to delete review", err)
	}

	if err := rs.repo.RecalcRating(txCtx, tx, review.ReviewProductID); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to recalculate product rating", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	// Async clear cache
	go func(pUUID string) {
		_ = rs.cache.Clear("reviews:product:" + pUUID + ":*")
		_ = rs.cache.Del("reviews:summary:" + pUUID)
		_ = rs.cache.Clear("products:*")
	}(review.ProductUuid.String())

	go func(images []string, videoURL *string) {
		filesToDelete := make([]string, 0, len(images)+1)
		filesToDelete = append(filesToDelete, images...)
		if videoURL != nil {
			filesToDelete = append(filesToDelete, *videoURL)
		}
		utils.DeleteFiles(filesToDelete...)
	}(review.ReviewImages, review.ReviewVideoUrl)

	return nil
}

// Admin
func (rs *reviewService) AdminGetAllReviews(ctx *gin.Context, query v1dto.AdminGetReviewsQuery) ([]v1dto.AdminReviewRow, int32, error) {
	reqCtx := ctx.Request.Context()

	page := query.Page
	if page <= 0 {
		page = 1
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}

	offset := (page - 1) * limit

	var productID *int32
	if query.ProductUUID != nil && *query.ProductUUID != "" {
		pUUID, err := uuid.Parse(*query.ProductUUID)
		if err != nil {
			return nil, 0, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID")
		}
		product, err := rs.productRepo.GetByUUID(reqCtx, pUUID)
		if err != nil {
			return nil, 0, utils.NewError(utils.ErrCodeNotFound, "Product not found")
		}
		productID = &product.ProductID
	}

	total, err := rs.repo.AdminCount(reqCtx, query.Status, query.Rating, productID, query.Search)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count reviews", err)
	}

	if total == 0 {
		return []v1dto.AdminReviewRow{}, 0, nil
	}

	rows, err := rs.repo.AdminGetAll(reqCtx, query.Status, query.Rating, query.ProductUUID, query.Search, "r.review_created_at", query.Sort, limit, offset)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to get reviews", err)
	}
	return rows, int32(total), nil
}

func (rs *reviewService) UpdateReviewStatus(ctx *gin.Context, reviewUUID uuid.UUID, input v1dto.UpdateReviewStatusInput) (v1dto.ReviewDTO, error) {
	reqCtx := ctx.Request.Context()

	review, err := rs.repo.GetByUUID(reqCtx, reviewUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeNotFound, "Review not found")
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
	}
	defer tx.Rollback(txCtx)

	updatedReview, err := rs.repo.UpdateStatus(txCtx, tx, review.ReviewID, input.Status)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update review status", err)
	}

	if err := rs.repo.RecalcRating(txCtx, tx, review.ReviewProductID); err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to recalculate product rating", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	go func(pUUID string) {
		_ = rs.cache.Clear("reviews:product:" + pUUID + ":*")
		_ = rs.cache.Del("reviews:summary:" + pUUID)
		_ = rs.cache.Clear("products:*")
	}(review.ProductUuid.String())

	images := updatedReview.ReviewImages
	if images == nil {
		images = []string{}
	}

	return v1dto.ReviewDTO{
		UUID:      updatedReview.ReviewUuid.String(),
		Rating:    int32(updatedReview.ReviewRating),
		Title:     updatedReview.ReviewTitle,
		Comment:   updatedReview.ReviewComment,
		Images:    images,
		VideoURL:  updatedReview.ReviewVideoUrl,
		HasMedia:  updatedReview.ReviewHasMedia,
		CreatedAt: updatedReview.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: updatedReview.ReviewUpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}
