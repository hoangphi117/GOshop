package v1service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

const (
	ReviewStatusVisible  int16 = 1
	ReviewStatusHidden   int16 = 2
	ReviewStatusReported int16 = 3
)

type reviewService struct {
	repo                repository.ReviewRepository
	userRepo            repository.UserRepository
	productRepo         repository.ProductRepository
	orderRepo           repository.OrderRepository
	notificationService NotificationService
	cache               cache.RedisCacheService
}

func NewReviewService(
	repo repository.ReviewRepository,
	userRepo repository.UserRepository,
	productRepo repository.ProductRepository,
	orderRepo repository.OrderRepository,
	notificationService NotificationService,
	redisClient *redis.Client,
) ReviewService {
	var cacheService cache.RedisCacheService
	if redisClient != nil {
		cacheService = cache.NewRedisCacheService(redisClient)
	}
	return &reviewService{
		repo:                repo,
		userRepo:            userRepo,
		productRepo:         productRepo,
		orderRepo:           orderRepo,
		notificationService: notificationService,
		cache:               cacheService,
	}
}

// -------------------------------------------------------------------------
// Single Responsibility Helpers
// -------------------------------------------------------------------------

func (rs *reviewService) getUser(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error) {
	user, err := rs.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "User not found")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get user", err)
	}
	return user, nil
}

func (rs *reviewService) getProduct(ctx context.Context, productUUID uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
	product, err := rs.productRepo.GetByUUID(ctx, productUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetProductByUUIDRow{}, utils.NewError(utils.ErrCodeNotFound, "Product not found")
		}
		return sqlc.GetProductByUUIDRow{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get product", err)
	}
	return product, nil
}

func (rs *reviewService) getOrder(ctx context.Context, orderUUID uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
	order, err := rs.orderRepo.GetByUUID(ctx, orderUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetOrderByUUIDRow{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
		}
		return sqlc.GetOrderByUUIDRow{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get order", err)
	}
	return order, nil
}

func (rs *reviewService) getReview(ctx context.Context, reviewUUID uuid.UUID) (sqlc.GetReviewByUUIDRow, error) {
	review, err := rs.repo.GetByUUID(ctx, reviewUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetReviewByUUIDRow{}, utils.NewError(utils.ErrCodeNotFound, "Review not found")
		}
		return sqlc.GetReviewByUUIDRow{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get review", err)
	}
	return review, nil
}

const maxReviewWindow = 30 * 24 * time.Hour

func (rs *reviewService) validateReviewEligibility(ctx context.Context, order sqlc.GetOrderByUUIDRow, userID, productID int32) error {
	if order.OrderDeliveredAt.Valid {
		if time.Since(order.OrderDeliveredAt.Time) > maxReviewWindow {
			return utils.NewError(utils.ErrCodeBadRequest, "The review window for this order has expired (allowed within 30 days of delivery)")
		}
	} else if order.OrderStatus == 4 && !order.OrderUpdatedAt.IsZero() && time.Since(order.OrderUpdatedAt) > maxReviewWindow {
		return utils.NewError(utils.ErrCodeBadRequest, "The review window for this order has expired (allowed within 30 days of delivery)")
	}

	isPurchased, err := rs.repo.CheckPurchased(ctx, order.OrderID, userID, productID)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Error checking purchase status", err)
	}
	if !isPurchased {
		return utils.NewError(utils.ErrCodeBadRequest, "User has not purchased this product")
	}

	isExists, err := rs.repo.CheckExists(ctx, userID, productID, order.OrderID)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Error checking review status", err)
	}
	if isExists {
		return utils.NewError(utils.ErrCodeBadRequest, "Review already exists for this product")
	}

	return nil
}

func (rs *reviewService) validateReviewEditEligibility(review sqlc.GetReviewByUUIDRow, userID int32) error {
	if review.ReviewUserID != userID {
		return utils.NewError(utils.ErrCodeForbidden, "You do not have permission to update this review")
	}

	if review.ReviewEditCount >= 1 {
		return utils.NewError(utils.ErrCodeBadRequest, "You can only edit your review once")
	}

	const maxEditDuration = 30 * 24 * time.Hour
	if time.Since(review.ReviewCreatedAt) > maxEditDuration {
		return utils.NewError(utils.ErrCodeBadRequest, "You can only edit your review within 30 days of posting")
	}

	return nil
}

func (rs *reviewService) invalidateProductReviewCache(productUUID string) {
	if rs.cache != nil {
		if err := rs.cache.Clear("reviews:product:" + productUUID + ":*"); err != nil {
			logger.Log.Warn().Err(err).Str("product_uuid", productUUID).Msg("Failed to clear product reviews cache")
		}
		if err := rs.cache.Del("reviews:summary:" + productUUID); err != nil {
			logger.Log.Warn().Err(err).Str("product_uuid", productUUID).Msg("Failed to clear review summary cache")
		}
		if err := rs.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear products cache")
		}
	}
}

func (rs *reviewService) dispatchReplyNotification(review sqlc.GetReviewByUUIDRow, reply string) {
	if rs.notificationService != nil {
		go func() {
			logger.Log.Info().
				Int32("recipient_user_id", review.ReviewUserID).
				Str("review_uuid", review.ReviewUuid.String()).
				Msg("Dispatching review_reply notification")

			replySnippet := reply
			if len(replySnippet) > 100 {
				replySnippet = replySnippet[:97] + "..."
			}

			_ = rs.notificationService.Send(context.Background(), v1dto.SendNotificationInput{
				UserID: review.ReviewUserID,
				Type:   "review_reply",
				Title:  "Shop replied to your review",
				Body:   fmt.Sprintf("Shop has replied to your review: \"%s\"", replySnippet),
				Data: map[string]any{
					"review_uuid":  review.ReviewUuid.String(),
					"product_uuid": review.ProductUuid.String(),
				},
			})
		}()
	}
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

// -------------------------------------------------------------------------
// ReviewService Implementation
// -------------------------------------------------------------------------

func (rs *reviewService) CreateReview(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateReviewInput, imageURLs []string, videoURL *string) (v1dto.ReviewDTO, error) {
	user, err := rs.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, err
	}

	productUUID, err := uuid.Parse(input.ProductUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID")
	}

	product, err := rs.getProduct(ctx, productUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, err
	}

	orderUUID, err := uuid.Parse(input.OrderUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID")
	}

	order, err := rs.getOrder(ctx, orderUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, err
	}

	if err := rs.validateReviewEligibility(ctx, order, user.UserID, product.ProductID); err != nil {
		return v1dto.ReviewDTO{}, err
	}

	if err := utils.ValidateReviewContent(input.Title, input.Comment); err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, err.Error())
	}

	txCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
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

	rs.invalidateProductReviewCache(input.ProductUUID)

	return v1dto.MapCreatedReviewToDTO(review, user.UserFullname, product.ProductUuid.String(), order.OrderUuid.String()), nil
}

func (rs *reviewService) GetProductReviews(ctx context.Context, productUUID uuid.UUID, query v1dto.GetReviewsQuery) ([]v1dto.ReviewDTO, int32, error) {
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

	if rs.cache != nil {
		if err := rs.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Reviews != nil {
			return cacheData.Reviews, cacheData.Total, nil
		}
	}

	product, err := rs.getProduct(ctx, productUUID)
	if err != nil {
		return nil, 0, err
	}

	var total int32
	if query.Rating == nil && query.HasMedia == nil {
		total = product.ProductReviewCount
	} else {
		count, err := rs.repo.CountByProduct(ctx, product.ProductID, query.Rating, query.HasMedia)
		if err != nil {
			return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count reviews", err)
		}
		total = int32(count)
	}

	reviewRows, err := rs.repo.GetByProduct(ctx, product.ProductID, query.Rating, query.HasMedia, query.OrderBy, query.Sort, limit, offset)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to get reviews", err)
	}

	reviewDTOs := make([]v1dto.ReviewDTO, len(reviewRows))
	for i, r := range reviewRows {
		images := r.ReviewImages
		if images == nil {
			images = []string{}
		}
		var replyAt *string
		if r.ReviewReplyAt != nil {
			formatted := r.ReviewReplyAt.Format("2006-01-02 15:04:05")
			replyAt = &formatted
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
			Reply:        r.ReviewReply,
			ReplyAt:      replyAt,
			HelpfulCount: r.ReviewHelpfulCount,
			CreatedAt:    r.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:    r.ReviewUpdatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	if rs.cache != nil {
		cacheData = struct {
			Reviews []v1dto.ReviewDTO `json:"reviews"`
			Total   int32             `json:"total"`
		}{
			Reviews: reviewDTOs,
			Total:   total,
		}
		_ = rs.cache.Set(cacheKey, cacheData, 5*time.Minute)
	}

	return reviewDTOs, total, nil
}

func (rs *reviewService) GetProductRatingSummary(ctx context.Context, productUUID uuid.UUID) (v1dto.RatingSummaryRow, error) {
	cacheKey := "reviews:summary:" + productUUID.String()
	var summaryDTO v1dto.RatingSummaryRow
	if rs.cache != nil {
		if err := rs.cache.Get(cacheKey, &summaryDTO); err == nil && summaryDTO.TotalReviews > 0 {
			return summaryDTO, nil
		}
	}

	product, err := rs.getProduct(ctx, productUUID)
	if err != nil {
		return v1dto.RatingSummaryRow{}, err
	}

	summary, err := rs.repo.GetRatingSummary(ctx, product.ProductID)
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

	if rs.cache != nil {
		_ = rs.cache.Set(cacheKey, summaryDTO, 5*time.Minute)
	}

	return summaryDTO, nil
}

func (rs *reviewService) GetMyReviews(ctx context.Context, userUUID uuid.UUID, query v1dto.GetMyReviewsQuery) ([]v1dto.MyReviewDTO, int32, error) {
	user, err := rs.getUser(ctx, userUUID)
	if err != nil {
		return nil, 0, err
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

	total, err := rs.repo.CountByUserID(ctx, user.UserID)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count user reviews", err)
	}
	if total == 0 {
		return []v1dto.MyReviewDTO{}, 0, nil
	}

	rows, err := rs.repo.GetByUserID(ctx, user.UserID, limit, offset)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to get user reviews", err)
	}

	return v1dto.MapToMyReviewDTOs(rows), int32(total), nil
}

func (rs *reviewService) UpdateReview(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID, input v1dto.UpdateReviewInput) (v1dto.ReviewDTO, error) {
	user, err := rs.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, err
	}

	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, err
	}

	if err := rs.validateReviewEditEligibility(review, user.UserID); err != nil {
		return v1dto.ReviewDTO{}, err
	}

	if err := utils.ValidateReviewContent(input.Title, input.Comment); err != nil {
		return v1dto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, err.Error())
	}

	txCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
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

	rs.invalidateProductReviewCache(review.ProductUuid.String())

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

func (rs *reviewService) UpdateReviewStatus(ctx context.Context, reviewUUID uuid.UUID, input v1dto.UpdateReviewStatusInput) (v1dto.ReviewDTO, error) {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, err
	}

	txCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
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

	rs.invalidateProductReviewCache(review.ProductUuid.String())

	images := updatedReview.ReviewImages
	if images == nil {
		images = []string{}
	}

	var replyAt *string
	if updatedReview.ReviewReplyAt.Valid {
		formatted := updatedReview.ReviewReplyAt.Time.Format("2006-01-02 15:04:05")
		replyAt = &formatted
	}

	return v1dto.ReviewDTO{
		UUID:         updatedReview.ReviewUuid.String(),
		Rating:       int32(updatedReview.ReviewRating),
		Title:        updatedReview.ReviewTitle,
		Comment:      updatedReview.ReviewComment,
		Images:       images,
		VideoURL:     updatedReview.ReviewVideoUrl,
		HasMedia:     updatedReview.ReviewHasMedia,
		Reply:        updatedReview.ReviewReply,
		ReplyAt:      replyAt,
		HelpfulCount: updatedReview.ReviewHelpfulCount,
		CreatedAt:    updatedReview.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    updatedReview.ReviewUpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (rs *reviewService) DeleteReview(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID) error {
	user, err := rs.getUser(ctx, userUUID)
	if err != nil {
		return err
	}

	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return err
	}

	if review.ReviewUserID != user.UserID {
		return utils.NewError(utils.ErrCodeForbidden, "You do not have permission to delete this review")
	}

	txCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
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

	rs.invalidateProductReviewCache(review.ProductUuid.String())

	go func(images []string, videoURL *string) {
		filesToDelete := make([]string, 0, len(images)+1)
		filesToDelete = append(filesToDelete, images...)
		if videoURL != nil {
			filesToDelete = append(filesToDelete, *videoURL)
		}
		utils.DeleteUploadedFiles(filesToDelete...)
	}(review.ReviewImages, review.ReviewVideoUrl)

	return nil
}

func (rs *reviewService) ToggleHelpfulVote(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID) (v1dto.ToggleHelpfulResponseDTO, error) {
	user, err := rs.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.ToggleHelpfulResponseDTO{}, err
	}

	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return v1dto.ToggleHelpfulResponseDTO{}, err
	}

	if review.ReviewUserID == user.UserID {
		return v1dto.ToggleHelpfulResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "You cannot vote for your own review")
	}

	txCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.ToggleHelpfulResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
	}
	defer tx.Rollback(txCtx)

	hasVoted, err := rs.repo.HasHelpfulVote(txCtx, user.UserID, review.ReviewID)
	if err != nil {
		return v1dto.ToggleHelpfulResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to check vote status", err)
	}

	var newCount int32
	var voted bool

	if hasVoted {
		newCount, err = rs.repo.RemoveHelpfulVote(txCtx, tx, user.UserID, review.ReviewID)
		if err != nil {
			return v1dto.ToggleHelpfulResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to remove helpful vote", err)
		}
		voted = false
	} else {
		newCount, err = rs.repo.AddHelpfulVote(txCtx, tx, user.UserID, review.ReviewID)
		if err != nil {
			return v1dto.ToggleHelpfulResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to add helpful vote", err)
		}
		voted = true
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.ToggleHelpfulResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	if rs.cache != nil {
		_ = rs.cache.Clear("reviews:product:" + review.ProductUuid.String() + ":*")
	}

	return v1dto.ToggleHelpfulResponseDTO{
		Voted:        voted,
		HelpfulCount: newCount,
	}, nil
}

func (rs *reviewService) ReportReview(ctx context.Context, userUUID uuid.UUID, reviewUUID uuid.UUID, input v1dto.ReportReviewInput) error {
	user, err := rs.getUser(ctx, userUUID)
	if err != nil {
		return err
	}

	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return err
	}

	if review.ReviewUserID == user.UserID {
		return utils.NewError(utils.ErrCodeBadRequest, "You cannot report your own review")
	}

	reported, err := rs.repo.HasReported(ctx, user.UserID, review.ReviewID)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to check report status", err)
	}
	if reported {
		return utils.NewError(utils.ErrCodeBadRequest, "You have already reported this review")
	}

	txCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
	}
	defer tx.Rollback(txCtx)

	if err := rs.repo.CreateReport(txCtx, tx, sqlc.InsertReviewReportParams{
		UserID:   user.UserID,
		ReviewID: review.ReviewID,
		Reason:   input.Reason,
	}); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to report review", err)
	}

	count, err := rs.repo.CountReports(txCtx, tx, review.ReviewID)
	if err == nil && count >= 3 && review.ReviewStatus == ReviewStatusVisible {
		if _, err := rs.repo.UpdateStatus(txCtx, tx, review.ReviewID, ReviewStatusReported); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to update review status", err)
		}
		if err := rs.repo.RecalcRating(txCtx, tx, review.ReviewProductID); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to recalculate product rating", err)
		}
	}

	if err := tx.Commit(txCtx); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	rs.invalidateProductReviewCache(review.ProductUuid.String())

	return nil
}

func (rs *reviewService) AdminGetAllReviews(ctx context.Context, query v1dto.AdminGetReviewsQuery) ([]v1dto.AdminReviewDTO, int32, error) {
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
		product, err := rs.getProduct(ctx, pUUID)
		if err != nil {
			return nil, 0, err
		}
		productID = &product.ProductID
	}

	total, err := rs.repo.AdminCount(ctx, query.Status, query.Rating, productID, query.Search)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count reviews", err)
	}

	if total == 0 {
		return []v1dto.AdminReviewDTO{}, 0, nil
	}

	rows, err := rs.repo.AdminGetAll(ctx, query.Status, query.Rating, query.ProductUUID, query.Search, "r.review_created_at", query.Sort, limit, offset)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to get reviews", err)
	}

	return v1dto.MapToAdminReviewDTOs(rows), int32(total), nil
}

func (rs *reviewService) AdminReplyReview(ctx context.Context, reviewUUID uuid.UUID, input v1dto.ReplyReviewInput) (v1dto.ReviewDTO, error) {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return v1dto.ReviewDTO{}, err
	}

	txCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
	}
	defer tx.Rollback(txCtx)

	updatedReview, err := rs.repo.Reply(txCtx, tx, review.ReviewID, input.Reply)
	if err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to reply to review", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	if rs.cache != nil {
		_ = rs.cache.Clear("reviews:product:" + review.ProductUuid.String() + ":*")
	}

	rs.dispatchReplyNotification(review, input.Reply)

	images := updatedReview.ReviewImages
	if images == nil {
		images = []string{}
	}

	var replyAt *string
	if updatedReview.ReviewReplyAt.Valid {
		formatted := updatedReview.ReviewReplyAt.Time.Format("2006-01-02 15:04:05")
		replyAt = &formatted
	}

	return v1dto.ReviewDTO{
		UUID:         updatedReview.ReviewUuid.String(),
		Rating:       int32(updatedReview.ReviewRating),
		Title:        updatedReview.ReviewTitle,
		Comment:      updatedReview.ReviewComment,
		Images:       images,
		VideoURL:     updatedReview.ReviewVideoUrl,
		HasMedia:     updatedReview.ReviewHasMedia,
		Reply:        updatedReview.ReviewReply,
		ReplyAt:      replyAt,
		HelpfulCount: updatedReview.ReviewHelpfulCount,
		CreatedAt:    updatedReview.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    updatedReview.ReviewUpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (rs *reviewService) AdminForceDeleteReview(ctx context.Context, reviewUUID uuid.UUID) error {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return err
	}

	txCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to begin transaction", err)
	}
	defer tx.Rollback(txCtx)

	if err := rs.repo.Delete(txCtx, tx, review.ReviewID); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to force delete review", err)
	}

	if err := rs.repo.RecalcRating(txCtx, tx, review.ReviewProductID); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to recalculate product rating", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	rs.invalidateProductReviewCache(review.ProductUuid.String())

	go func(images []string, videoURL *string) {
		files := make([]string, 0, len(images)+1)
		files = append(files, images...)
		if videoURL != nil && *videoURL != "" {
			files = append(files, *videoURL)
		}
		utils.DeleteUploadedFiles(files...)
	}(review.ReviewImages, review.ReviewVideoUrl)

	return nil
}

func (rs *reviewService) GetReviewReports(ctx context.Context, reviewUUID uuid.UUID) ([]v1dto.ReviewReportDTO, error) {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return nil, err
	}

	rows, err := rs.repo.GetReportsByReviewID(ctx, review.ReviewID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get review reports", err)
	}

	return v1dto.MapToReviewReportDTOs(rows), nil
}

func (rs *reviewService) AdminDismissReports(ctx context.Context, reviewUUID uuid.UUID) error {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return err
	}

	txCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
	}
	defer tx.Rollback(txCtx)

	if err := rs.repo.DeleteReportsByReviewID(txCtx, tx, review.ReviewID); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to delete reports", err)
	}

	// If the review was marked as Reported, restore to Visible and recalc rating
	if review.ReviewStatus == ReviewStatusReported {
		if _, err := rs.repo.UpdateStatus(txCtx, tx, review.ReviewID, ReviewStatusVisible); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to restore review status", err)
		}
		if err := rs.repo.RecalcRating(txCtx, tx, review.ReviewProductID); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to recalculate product rating", err)
		}
	}

	if err := tx.Commit(txCtx); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	rs.invalidateProductReviewCache(review.ProductUuid.String())

	return nil
}
