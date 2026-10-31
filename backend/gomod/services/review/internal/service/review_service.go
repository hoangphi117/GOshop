package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"goshop/contracts/events"
	catalogcontract "goshop/services/catalog/contract"
	reviewsqlc "goshop/services/review/internal/db/sqlc"
	reviewdto "goshop/services/review/internal/dto"
	reviewrepo "goshop/services/review/internal/repository"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/logger"
	"goshop/shared/pkg/messaging"
	pkgpgx "goshop/shared/pkg/pgx"
	"goshop/shared/pkg/utils"
)

// OrderPurchaseVerification is the trimmed purchase-check result used by the
// review domain (subset of ordercontract.PurchaseVerification).
type OrderPurchaseVerification struct {
	OrderID       int32
	OrderUUID     uuid.UUID
	IsPurchased   bool
	CanReview     bool
	CustomerName  string
	CustomerEmail string
}

const (
	ReviewStatusVisible  int16 = 1
	ReviewStatusHidden   int16 = 2
	ReviewStatusReported int16 = 3
)

type ReviewService interface {
	CreateReview(ctx context.Context, userID int32, input reviewdto.CreateReviewInput, imageURLs []string, videoURL *string) (reviewdto.ReviewDTO, error)
	GetProductReviews(ctx context.Context, productUUID uuid.UUID, query reviewdto.GetReviewsQuery) ([]reviewdto.ReviewDTO, int32, error)
	GetProductRatingSummary(ctx context.Context, productUUID uuid.UUID) (reviewdto.RatingSummaryRow, error)
	GetMyReviews(ctx context.Context, userID int32, query reviewdto.GetMyReviewsQuery) ([]reviewdto.MyReviewDTO, int32, error)
	UpdateReview(ctx context.Context, userID int32, reviewUUID uuid.UUID, input reviewdto.UpdateReviewInput) (reviewdto.ReviewDTO, error)
	UpdateReviewStatus(ctx context.Context, reviewUUID uuid.UUID, input reviewdto.UpdateReviewStatusInput) (reviewdto.ReviewDTO, error)
	DeleteReview(ctx context.Context, userID int32, reviewUUID uuid.UUID) error
	AdminGetAllReviews(ctx context.Context, query reviewdto.AdminGetReviewsQuery) ([]reviewdto.AdminReviewDTO, int32, error)
	AdminReplyReview(ctx context.Context, reviewUUID uuid.UUID, input reviewdto.ReplyReviewInput) (reviewdto.ReviewDTO, error)
	ToggleHelpfulVote(ctx context.Context, userID int32, reviewUUID uuid.UUID) (reviewdto.ToggleHelpfulResponseDTO, error)
	ReportReview(ctx context.Context, userID int32, reviewUUID uuid.UUID, input reviewdto.ReportReviewInput) error
	AdminForceDeleteReview(ctx context.Context, reviewUUID uuid.UUID) error
	GetReviewReports(ctx context.Context, reviewUUID uuid.UUID) ([]reviewdto.ReviewReportDTO, error)
	AdminDismissReports(ctx context.Context, reviewUUID uuid.UUID) error
}

type reviewService struct {
	repo        reviewrepo.ReviewRepository
	catalog     ProductCatalog
	orderReader PurchaseVerifier
	userReader  UserReader
	rabbitMQ    messaging.Broker
	cache       cache.RedisCacheService
	txManager   pkgpgx.TxManager
}

func NewReviewService(
	repo reviewrepo.ReviewRepository,
	catalog ProductCatalog,
	orderReader PurchaseVerifier,
	userReader UserReader,
	rabbitMQ messaging.Broker,
	redisClient cache.RedisCacheService,
	txManager pkgpgx.TxManager,
) ReviewService {
	return &reviewService{
		repo:        repo,
		catalog:     catalog,
		orderReader: orderReader,
		userReader:  userReader,
		rabbitMQ:    rabbitMQ,
		cache:       redisClient,
		txManager:   txManager,
	}
}

// -------------------------------------------------------------------------
// Single Responsibility Helpers
// -------------------------------------------------------------------------

func (rs *reviewService) getProduct(ctx context.Context, productUUID uuid.UUID) (*catalogcontract.ProductItemInfo, error) {
	if rs.catalog == nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "Catalog client not configured")
	}
	p, err := rs.catalog.GetProductByUUID(ctx, productUUID.String())
	if err != nil || p == nil {
		return nil, utils.NewError(utils.ErrCodeNotFound, "Product not found")
	}
	return p, nil
}

func (rs *reviewService) verifyOrderPurchase(ctx context.Context, orderUUID uuid.UUID, userID, productID int32) (*OrderPurchaseVerification, error) {
	if rs.orderReader == nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "Order client not configured")
	}
	verification, err := rs.orderReader.VerifyPurchase(ctx, orderUUID, userID, productID, nil, nil)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to verify purchase", err)
	}
	return &OrderPurchaseVerification{
		OrderID:       verification.OrderID,
		OrderUUID:     verification.OrderUUID,
		IsPurchased:   verification.IsPurchased,
		CanReview:     verification.CanReview,
		CustomerName:  verification.CustomerName,
		CustomerEmail: verification.CustomerEmail,
	}, nil
}

func (rs *reviewService) getReview(ctx context.Context, reviewUUID uuid.UUID) (reviewsqlc.ProductReview, error) {
	review, err := rs.repo.GetByUUID(ctx, reviewUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return reviewsqlc.ProductReview{}, utils.NewError(utils.ErrCodeNotFound, "Review not found")
		}
		return reviewsqlc.ProductReview{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get review", err)
	}
	return review, nil
}

func (rs *reviewService) validateReviewEditEligibility(review reviewsqlc.ProductReview, userID int32) error {
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
	}
}

// publishRatingUpdate recomputes the product rating aggregate from the review
// database and notifies catalog via a work-queue event (the products table now
// lives in the catalog database).
func (rs *reviewService) publishRatingUpdate(productID int32, productUUID string) {
	if rs.rabbitMQ == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		agg, err := rs.repo.GetRatingAggregate(ctx, productID)
		if err != nil {
			logger.Log.Warn().Err(err).Int32("product_id", productID).Msg("Failed to compute rating aggregate after review change")
			return
		}

		event := events.NewReviewRatingUpdatedEvent(productID, productUUID, agg.AvgRating, agg.ReviewCount)
		if err := rs.rabbitMQ.Publish(ctx, events.QueueReviewRatingUpdated, event); err != nil {
			logger.Log.Warn().Err(err).Int32("product_id", productID).Msg("Failed to publish review rating update event")
		}
	}()
}

func (rs *reviewService) dispatchReplyNotification(review reviewsqlc.ProductReview, reply string) {
	if rs.rabbitMQ != nil {
		go func() {
			replySnippet := reply
			if len(replySnippet) > 100 {
				replySnippet = replySnippet[:97] + "..."
			}

			event := events.NewCustomerNotificationEvent(
				uuid.New().String(),
				review.ReviewUserID,
				"review_reply",
				"reply",
				"Shop replied to your review",
				fmt.Sprintf("Shop has replied to your review: \"%s\"", replySnippet),
				map[string]any{
					"review_uuid":  review.ReviewUuid.String(),
					"product_uuid": review.ReviewProductUuid.String(),
				},
			)

			// Must go through the topic exchange: the notification service binds
			// its queue to goshop.events and never consumes notification_events directly.
			if err := rs.rabbitMQ.PublishTopic(context.Background(), events.RoutingKeyNotificationEvent, event); err != nil {
				logger.Log.Warn().Err(err).Str("review_uuid", review.ReviewUuid.String()).Msg("Failed to publish review_reply notification event")
			}
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

func formatTimePtr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	formatted := t.Time.Format("2006-01-02 15:04:05")
	return &formatted
}

func formatTimePtrValue(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.Format("2006-01-02 15:04:05")
	return &formatted
}

func reviewListDTO(r reviewdto.ReviewRow) reviewdto.ReviewDTO {
	images := r.ReviewImages
	if images == nil {
		images = []string{}
	}
	return reviewdto.ReviewDTO{
		UUID:         r.ReviewUUID.String(),
		ReviewerName: utils.MaskName(r.ReviewerName),
		Rating:       int32(r.ReviewRating),
		Title:        r.ReviewTitle,
		Comment:      r.ReviewComment,
		Images:       images,
		VideoURL:     r.ReviewVideoURL,
		HasMedia:     r.ReviewHasMedia,
		Reply:        r.ReviewReply,
		ReplyAt:      formatTimePtrValue(r.ReviewReplyAt),
		HelpfulCount: r.ReviewHelpfulCount,
		CreatedAt:    r.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    r.ReviewUpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

func reviewDetailDTO(r reviewsqlc.ProductReview) reviewdto.ReviewDTO {
	images := r.ReviewImages
	if images == nil {
		images = []string{}
	}
	return reviewdto.ReviewDTO{
		UUID:         r.ReviewUuid.String(),
		ProductUUID:  r.ReviewProductUuid.String(),
		OrderUUID:    r.ReviewOrderUuid.String(),
		Rating:       int32(r.ReviewRating),
		Title:        r.ReviewTitle,
		Comment:      r.ReviewComment,
		Images:       images,
		VideoURL:     r.ReviewVideoUrl,
		HasMedia:     r.ReviewHasMedia,
		Reply:        r.ReviewReply,
		ReplyAt:      formatTimePtr(r.ReviewReplyAt),
		HelpfulCount: r.ReviewHelpfulCount,
		CreatedAt:    r.ReviewCreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:    r.ReviewUpdatedAt.Format("2006-01-02 15:04:05"),
	}
}

// -------------------------------------------------------------------------
// Public ReviewService Implementation
// -------------------------------------------------------------------------

func (rs *reviewService) CreateReview(ctx context.Context, userID int32, input reviewdto.CreateReviewInput, imageURLs []string, videoURL *string) (reviewdto.ReviewDTO, error) {
	productUUID, err := uuid.Parse(input.ProductUUID)
	if err != nil {
		return reviewdto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID")
	}

	product, err := rs.getProduct(ctx, productUUID)
	if err != nil {
		return reviewdto.ReviewDTO{}, err
	}

	orderUUID, err := uuid.Parse(input.OrderUUID)
	if err != nil {
		return reviewdto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID")
	}

	verification, err := rs.verifyOrderPurchase(ctx, orderUUID, userID, product.ProductID)
	if err != nil {
		return reviewdto.ReviewDTO{}, err
	}
	if !verification.IsPurchased {
		return reviewdto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "User has not purchased this product")
	}
	if !verification.CanReview {
		return reviewdto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "The review window for this order has expired")
	}

	isExists, err := rs.repo.CheckExists(ctx, userID, product.ProductID, verification.OrderUUID)
	if err != nil {
		return reviewdto.ReviewDTO{}, utils.WrapError(utils.ErrCodeInternal, "Error checking review status", err)
	}

	if isExists {
		return reviewdto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Review already exists for this product")
	}

	if err := utils.ValidateReviewContent(input.Title, input.Comment); err != nil {
		return reviewdto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, err.Error())
	}

	userName := verification.CustomerName
	userEmail := verification.CustomerEmail
	userAvatar := ""

	if u, uErr := rs.userReader.GetUserByID(ctx, userID); uErr == nil && u != nil {
		if u.UserFullname != "" {
			userName = u.UserFullname
		}
		if u.UserEmail != "" {
			userEmail = u.UserEmail
		}
		if u.UserAvatarUrl != nil && *u.UserAvatarUrl != "" {
			userAvatar = *u.UserAvatarUrl
		} else if u.UserAvatar != "" {
			userAvatar = u.UserAvatar
		}
	}

	if userName == "" {
		userName = "Customer"
	}

	if imageURLs == nil {
		imageURLs = []string{}
	}
	hasMedia := len(imageURLs) > 0 || videoURL != nil

	var orderID *int32
	if verification.OrderID > 0 {
		orderID = &verification.OrderID
	}

	var review reviewsqlc.ProductReview
	txErr := rs.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		review, err = rs.repo.Create(txCtx, reviewsqlc.CreateReviewParams{
			ReviewUserID:      userID,
			ReviewProductID:   product.ProductID,
			ReviewProductUuid: productUUID,
			ReviewOrderID:     orderID,
			ReviewOrderUuid:   verification.OrderUUID,
			ReviewRating:      int16(input.Rating),
			ReviewTitle:       input.Title,
			ReviewComment:     input.Comment,
			ReviewImages:      imageURLs,
			ReviewVideoUrl:    videoURL,
			ReviewHasMedia:    hasMedia,
			ReviewUserName:    userName,
			ReviewUserEmail:   userEmail,
			ReviewUserAvatar:  userAvatar,
		})
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to create review", err)
		}
		return nil
	})
	if txErr != nil {
		return reviewdto.ReviewDTO{}, txErr
	}

	rs.invalidateProductReviewCache(productUUID.String())
	rs.publishRatingUpdate(product.ProductID, productUUID.String())

	if rs.rabbitMQ != nil {
		commentSnippet := ""
		if input.Comment != nil {
			commentSnippet = *input.Comment
			if len(commentSnippet) > 80 {
				commentSnippet = commentSnippet[:77] + "..."
			}
		}

		event := events.NewAdminNotificationEvent(
			uuid.New().String(),
			"review",
			"created",
			"New Product Review",
			fmt.Sprintf("New %d-star review for \"%s\": %s", input.Rating, product.ProductName, commentSnippet),
			map[string]any{
				"review_uuid":  review.ReviewUuid.String(),
				"product_uuid": productUUID.String(),
				"rating":       input.Rating,
			},
		)
		// Topic exchange — notification service consumes goshop.events with
		// routing pattern "notification_events" (see notification app topicPatterns).
		if err := rs.rabbitMQ.PublishTopic(ctx, events.RoutingKeyNotificationEvent, event); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to publish new review admin notification event")
		}
	}

	dto := reviewDetailDTO(review)
	dto.ReviewerName = utils.MaskName(userName)
	return dto, nil
}

func (rs *reviewService) GetProductReviews(ctx context.Context, productUUID uuid.UUID, query reviewdto.GetReviewsQuery) ([]reviewdto.ReviewDTO, int32, error) {
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
		Reviews []reviewdto.ReviewDTO `json:"reviews"`
		Total   int32                 `json:"total"`
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

	// Count always comes from the review DB — catalog's aggregate is a
	// denormalized copy that lags behind event propagation.
	total64, err := rs.repo.CountByProduct(ctx, product.ProductID, query.Rating, query.HasMedia)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count reviews", err)
	}
	total := int32(total64)

	reviewRows, err := rs.repo.GetByProduct(ctx, product.ProductID, query.Rating, query.HasMedia, query.OrderBy, query.Sort, limit, offset)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to get reviews", err)
	}

	reviewDTOs := make([]reviewdto.ReviewDTO, len(reviewRows))
	for i, row := range reviewRows {
		reviewDTOs[i] = reviewListDTO(row)
	}

	if rs.cache != nil {
		cacheData = struct {
			Reviews []reviewdto.ReviewDTO `json:"reviews"`
			Total   int32                 `json:"total"`
		}{Reviews: reviewDTOs, Total: total}
		_ = rs.cache.Set(cacheKey, cacheData, 5*time.Minute)
	}

	return reviewDTOs, total, nil
}

func (rs *reviewService) GetProductRatingSummary(ctx context.Context, productUUID uuid.UUID) (reviewdto.RatingSummaryRow, error) {
	cacheKey := "reviews:summary:" + productUUID.String()
	var summaryDTO reviewdto.RatingSummaryRow

	if rs.cache != nil {
		if err := rs.cache.Get(cacheKey, &summaryDTO); err == nil && summaryDTO.TotalReviews > 0 {
			return summaryDTO, nil
		}
	}

	product, err := rs.getProduct(ctx, productUUID)
	if err != nil {
		return reviewdto.RatingSummaryRow{}, err
	}

	summary, err := rs.repo.GetRatingSummary(ctx, product.ProductID)
	if err != nil {
		return reviewdto.RatingSummaryRow{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get rating summary", err)
	}

	summaryDTO = reviewdto.RatingSummaryRow{
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

func (rs *reviewService) GetMyReviews(ctx context.Context, userID int32, query reviewdto.GetMyReviewsQuery) ([]reviewdto.MyReviewDTO, int32, error) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	count, err := rs.repo.CountByUserID(ctx, userID)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count reviews", err)
	}
	total := int32(count)

	rows, err := rs.repo.GetByUserID(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to get reviews", err)
	}

	myReviews := reviewdto.MapToMyReviewDTOs(rows)

	// Enrich with product info via catalog gRPC (no joinable products table here).
	if rs.catalog != nil && len(rows) > 0 {
		uuids := make([]string, 0, len(rows))
		for _, r := range rows {
			uuids = append(uuids, r.ReviewProductUuid.String())
		}
		if products, batchErr := rs.catalog.GetProductsBatch(ctx, uuids, nil); batchErr == nil {
			infoByUUID := make(map[string]*catalogcontract.ProductItemInfo, len(products))
			for i := range products {
				infoByUUID[products[i].ProductUUID] = &products[i]
			}
			for i := range myReviews {
				if p, ok := infoByUUID[myReviews[i].ProductUUID]; ok {
					myReviews[i].ProductName = p.ProductName
					myReviews[i].ProductSlug = p.ProductSlug
					if p.ProductImageURL != "" {
						img := p.ProductImageURL
						myReviews[i].ProductImageURL = &img
					}
				}
			}
		}
	}

	return myReviews, total, nil
}

// -------------------------------------------------------------------------
// Owner mutations (one edit, 30-day window)
// -------------------------------------------------------------------------

func (rs *reviewService) UpdateReview(ctx context.Context, userID int32, reviewUUID uuid.UUID, input reviewdto.UpdateReviewInput) (reviewdto.ReviewDTO, error) {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return reviewdto.ReviewDTO{}, err
	}

	if err := rs.validateReviewEditEligibility(review, userID); err != nil {
		return reviewdto.ReviewDTO{}, err
	}

	if err := utils.ValidateReviewContent(input.Title, input.Comment); err != nil {
		return reviewdto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, err.Error())
	}

	var updated reviewsqlc.ProductReview
	txErr := rs.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		updated, err = rs.repo.Update(txCtx, reviewsqlc.UpdateReviewParams{
			ReviewID:      review.ReviewID,
			ReviewRating:  int16(input.Rating),
			ReviewTitle:   input.Title,
			ReviewComment: input.Comment,
		})
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to update review", err)
		}
		return nil
	})
	if txErr != nil {
		return reviewdto.ReviewDTO{}, txErr
	}

	rs.invalidateProductReviewCache(review.ReviewProductUuid.String())
	rs.publishRatingUpdate(review.ReviewProductID, review.ReviewProductUuid.String())

	return reviewDetailDTO(updated), nil
}

func (rs *reviewService) DeleteReview(ctx context.Context, userID int32, reviewUUID uuid.UUID) error {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return err
	}

	if review.ReviewUserID != userID {
		return utils.NewError(utils.ErrCodeForbidden, "You do not have permission to delete this review")
	}

	txErr := rs.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := rs.repo.Delete(txCtx, review.ReviewID); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to delete review", err)
		}
		return nil
	})
	if txErr != nil {
		return txErr
	}

	rs.invalidateProductReviewCache(review.ReviewProductUuid.String())
	rs.publishRatingUpdate(review.ReviewProductID, review.ReviewProductUuid.String())

	// ponytail: deleted reviews leave their media objects in the bucket; wire an
	// orphan-GC job (or a media-service delete event) before storage costs matter.

	return nil
}

func (rs *reviewService) ToggleHelpfulVote(ctx context.Context, userID int32, reviewUUID uuid.UUID) (reviewdto.ToggleHelpfulResponseDTO, error) {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return reviewdto.ToggleHelpfulResponseDTO{}, err
	}

	if review.ReviewUserID == userID {
		return reviewdto.ToggleHelpfulResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "You cannot vote for your own review")
	}

	hasVoted, err := rs.repo.HasHelpfulVote(ctx, userID, review.ReviewID)
	if err != nil {
		return reviewdto.ToggleHelpfulResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to check vote status", err)
	}

	var newCount int32
	var voted bool
	txErr := rs.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		if hasVoted {
			newCount, err = rs.repo.RemoveHelpfulVote(txCtx, userID, review.ReviewID)
			voted = false
			if err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "Failed to remove helpful vote", err)
			}
		} else {
			newCount, err = rs.repo.AddHelpfulVote(txCtx, userID, review.ReviewID)
			voted = true
			if err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "Failed to add helpful vote", err)
			}
		}
		return nil
	})
	if txErr != nil {
		return reviewdto.ToggleHelpfulResponseDTO{}, txErr
	}

	if rs.cache != nil {
		_ = rs.cache.Clear("reviews:product:" + review.ReviewProductUuid.String() + ":*")
	}

	return reviewdto.ToggleHelpfulResponseDTO{Voted: voted, HelpfulCount: newCount}, nil
}

func (rs *reviewService) ReportReview(ctx context.Context, userID int32, reviewUUID uuid.UUID, input reviewdto.ReportReviewInput) error {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return err
	}

	if review.ReviewUserID == userID {
		return utils.NewError(utils.ErrCodeBadRequest, "You cannot report your own review")
	}

	if input.Reason != "spam" && input.Reason != "inappropriate" && input.Reason != "fake" && input.Reason != "other" {
		return utils.NewError(utils.ErrCodeBadRequest, "Invalid report reason")
	}

	hasReported, err := rs.repo.HasReported(ctx, userID, review.ReviewID)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to check report status", err)
	}
	if hasReported {
		return utils.NewError(utils.ErrCodeBadRequest, "You have already reported this review")
	}

	statusChanged := false
	txErr := rs.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := rs.repo.CreateReport(txCtx, reviewsqlc.InsertReviewReportParams{
			UserID:   userID,
			ReviewID: review.ReviewID,
			Reason:   input.Reason,
		}); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to report review", err)
		}

		count, err := rs.repo.CountReports(txCtx, review.ReviewID)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to count reports", err)
		}
		if count >= 3 && review.ReviewStatus == ReviewStatusVisible {
			if _, err := rs.repo.UpdateStatus(txCtx, review.ReviewID, ReviewStatusReported); err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "Failed to update review status", err)
			}
			statusChanged = true
		}
		return nil
	})
	if txErr != nil {
		return txErr
	}

	rs.invalidateProductReviewCache(review.ReviewProductUuid.String())
	if statusChanged {
		rs.publishRatingUpdate(review.ReviewProductID, review.ReviewProductUuid.String())
	}

	return nil
}

// -------------------------------------------------------------------------
// Admin
// -------------------------------------------------------------------------

func (rs *reviewService) AdminGetAllReviews(ctx context.Context, query reviewdto.AdminGetReviewsQuery) ([]reviewdto.AdminReviewDTO, int32, error) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	var productUUID *uuid.UUID
	if query.ProductUUID != nil && *query.ProductUUID != "" {
		pu, err := uuid.Parse(*query.ProductUUID)
		if err != nil {
			return nil, 0, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID")
		}
		productUUID = &pu
	}

	orderBy := query.OrderBy
	if orderBy != "rating" {
		orderBy = "created_at"
	}

	total, err := rs.repo.AdminCount(ctx, query.Status, query.Rating, productUUID, query.Search)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to count reviews", err)
	}
	if total == 0 {
		return []reviewdto.AdminReviewDTO{}, 0, nil
	}

	rows, err := rs.repo.AdminGetAll(ctx, query.Status, query.Rating, productUUID, query.Search, orderBy, query.Sort, limit, offset)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "Failed to get reviews", err)
	}

	// Enrich product names via catalog gRPC (batch, avoids N+1).
	if rs.catalog != nil && len(rows) > 0 {
		ids := make([]int32, 0, len(rows))
		seen := make(map[int32]struct{}, len(rows))
		for _, r := range rows {
			if _, ok := seen[r.ProductID]; !ok {
				ids = append(ids, r.ProductID)
				seen[r.ProductID] = struct{}{}
			}
		}
		if products, batchErr := rs.catalog.GetProductsBatch(ctx, nil, ids); batchErr == nil {
			infoByID := make(map[int32]catalogcontract.ProductItemInfo, len(products))
			for _, p := range products {
				infoByID[p.ProductID] = p
			}
			for i := range rows {
				if p, ok := infoByID[rows[i].ProductID]; ok {
					rows[i].ProductName = p.ProductName
				}
			}
		}
	}

	return reviewdto.MapToAdminReviewDTOs(rows), int32(total), nil
}

func (rs *reviewService) AdminReplyReview(ctx context.Context, reviewUUID uuid.UUID, input reviewdto.ReplyReviewInput) (reviewdto.ReviewDTO, error) {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return reviewdto.ReviewDTO{}, err
	}

	var updated reviewsqlc.ProductReview
	txErr := rs.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		updated, err = rs.repo.Reply(txCtx, review.ReviewID, input.Reply)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to reply to review", err)
		}
		return nil
	})
	if txErr != nil {
		return reviewdto.ReviewDTO{}, txErr
	}

	rs.invalidateProductReviewCache(review.ReviewProductUuid.String())
	rs.dispatchReplyNotification(review, input.Reply)

	return reviewDetailDTO(updated), nil
}

func (rs *reviewService) UpdateReviewStatus(ctx context.Context, reviewUUID uuid.UUID, input reviewdto.UpdateReviewStatusInput) (reviewdto.ReviewDTO, error) {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return reviewdto.ReviewDTO{}, err
	}

	if input.Status != ReviewStatusVisible && input.Status != ReviewStatusHidden && input.Status != ReviewStatusReported {
		return reviewdto.ReviewDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid status value")
	}

	var updated reviewsqlc.ProductReview
	txErr := rs.txManager.WithTx(ctx, func(txCtx context.Context) error {
		var err error
		updated, err = rs.repo.UpdateStatus(txCtx, review.ReviewID, input.Status)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to update review status", err)
		}
		return nil
	})
	if txErr != nil {
		return reviewdto.ReviewDTO{}, txErr
	}

	rs.invalidateProductReviewCache(review.ReviewProductUuid.String())
	rs.publishRatingUpdate(review.ReviewProductID, review.ReviewProductUuid.String())

	return reviewDetailDTO(updated), nil
}

func (rs *reviewService) AdminForceDeleteReview(ctx context.Context, reviewUUID uuid.UUID) error {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return err
	}

	txErr := rs.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := rs.repo.Delete(txCtx, review.ReviewID); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to delete review", err)
		}
		return nil
	})
	if txErr != nil {
		return txErr
	}

	rs.invalidateProductReviewCache(review.ReviewProductUuid.String())
	rs.publishRatingUpdate(review.ReviewProductID, review.ReviewProductUuid.String())

	// ponytail: deleted reviews leave their media objects in the bucket; wire an
	// orphan-GC job (or a media-service delete event) before storage costs matter.

	return nil
}

func (rs *reviewService) GetReviewReports(ctx context.Context, reviewUUID uuid.UUID) ([]reviewdto.ReviewReportDTO, error) {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return nil, err
	}

	rows, err := rs.repo.GetReportsByReviewID(ctx, review.ReviewID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get review reports", err)
	}

	return reviewdto.MapToReviewReportDTOs(rows), nil
}

func (rs *reviewService) AdminDismissReports(ctx context.Context, reviewUUID uuid.UUID) error {
	review, err := rs.getReview(ctx, reviewUUID)
	if err != nil {
		return err
	}

	statusRestored := false
	txErr := rs.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := rs.repo.DeleteReportsByReviewID(txCtx, review.ReviewID); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to delete reports", err)
		}
		if review.ReviewStatus == ReviewStatusReported {
			if _, err := rs.repo.UpdateStatus(txCtx, review.ReviewID, ReviewStatusVisible); err != nil {
				return utils.WrapError(utils.ErrCodeInternal, "Failed to restore review status", err)
			}
			statusRestored = true
		}
		return nil
	})
	if txErr != nil {
		return txErr
	}

	rs.invalidateProductReviewCache(review.ReviewProductUuid.String())
	if statusRestored {
		rs.publishRatingUpdate(review.ReviewProductID, review.ReviewProductUuid.String())
	}

	return nil
}
