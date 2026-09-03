package v1service

import (
	"context"
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

func (rs *reviewService) CreateReview(ctx *gin.Context, input v1dto.CreateReviewInput, imageURLs []string, videoURL *string) (v1dto.ReviewResponseDTO, error) {
	reqCtx := ctx.Request.Context()

	userUUIDStr, ok := ctx.Get("user_uuid")
	if !ok {
		return v1dto.ReviewResponseDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return v1dto.ReviewResponseDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID")
	}

	user, err := rs.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return v1dto.ReviewResponseDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "User not found")
	}

	productUUID, err := uuid.Parse(input.ProductUUID)
	if err != nil {
		return v1dto.ReviewResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID")
	}

	product, err := rs.productRepo.GetByUUID(reqCtx, productUUID)
	if err != nil {
		return v1dto.ReviewResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Product not found")
	}

	orderUUID, err := uuid.Parse(input.OrderUUID)
	if err != nil {
		return v1dto.ReviewResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID")
	}

	order, err := rs.orderRepo.GetByUUID(reqCtx, orderUUID)
	if err != nil {
		return v1dto.ReviewResponseDTO{}, utils.NewError(utils.ErrCodeNotFound, "Order not found")
	}

	isPurchased, err := rs.repo.CheckPurchased(reqCtx, order.OrderID, user.UserID, product.ProductID)
	if err != nil {
		return v1dto.ReviewResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Error checking purchase status", err)
	}
	if !isPurchased {
		return v1dto.ReviewResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "User has not purchased this product")
	}

	isExists, err := rs.repo.CheckExists(reqCtx, user.UserID, product.ProductID, order.OrderID)
	if err != nil {
		return v1dto.ReviewResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Error checking review status", err)
	}
	if isExists {
		return v1dto.ReviewResponseDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Review already exists for this product")
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.ReviewResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to start transaction", err)
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
		return v1dto.ReviewResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to create review", err)
	}

	if err := rs.repo.RecalcRating(txCtx, tx, product.ProductID); err != nil {
		return v1dto.ReviewResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to recalculate product rating", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.ReviewResponseDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
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
