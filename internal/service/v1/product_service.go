package v1service

import (
	"context"
	"database/sql"
	"errors"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

type productService struct {
	repo         repository.ProductRepository
	categoryRepo repository.CategoryRepository
	cache        cache.RedisCacheService
}

func NewProductService(repo repository.ProductRepository, categoryRepo repository.CategoryRepository, redisClient *redis.Client) ProductService {
	return &productService{
		repo:         repo,
		categoryRepo: categoryRepo,
		cache:        cache.NewRedisCacheService(redisClient),
	}
}

func (ps *productService) CreateProduct(ctx *gin.Context, input v1dto.CreateProductInput) (sqlc.CreateProductRow, error) {
	reqCtx := ctx.Request.Context()

	categoryUuid, err := uuid.Parse(input.CategoryUUID)
	if err != nil {
		return sqlc.CreateProductRow{}, utils.NewError(utils.ErrCodeBadRequest, "invalid category uuid")
	}

	category, err := ps.categoryRepo.GetByUUID(reqCtx, categoryUuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.CreateProductRow{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return sqlc.CreateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
	}
	input.Params.ProductCategoryID = category.CategoryID

	if input.Params.ProductStatus == 0 {
		input.Params.ProductStatus = 1
	}

	slug, err := generateUniqueSlug(reqCtx, input.Params.ProductName, func(slugCtx context.Context, slug string) error {
		_, err := ps.repo.GetBySlug(slugCtx, slug)
		return err
	})
	if err != nil {
		return sqlc.CreateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to generate slug", err)
	}
	input.Params.ProductSlug = slug

	if input.Params.ProductSku != nil {
		sku := strings.ToUpper(strings.TrimSpace(*input.Params.ProductSku))
		if sku == "" {
			input.Params.ProductSku = nil
		} else {
			input.Params.ProductSku = &sku
		}
	}

	product, err := ps.repo.Create(reqCtx, input.Params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return sqlc.CreateProductRow{}, utils.NewError(utils.ErrCodeConflict, "product already exists")
		}
		return sqlc.CreateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to create product", err)
	}

	if err := ps.cache.Clear("products:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
	}

	return product, nil
}

func (ps *productService) GetProductByUUID(ctx *gin.Context, productUuid uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
	reqCtx := ctx.Request.Context()

	product, err := ps.repo.GetByUUID(reqCtx, productUuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.GetProductByUUIDRow{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return sqlc.GetProductByUUIDRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to get product", err)
	}

	return product, nil
}

func (ps *productService) UpdateProduct(ctx *gin.Context, input v1dto.UpdateProductInput) (sqlc.UpdateProductRow, error) {
	reqCtx := ctx.Request.Context()

	if input.CategoryUUID != nil {
		categoryUuid, err := uuid.Parse(*input.CategoryUUID)

		if err != nil {
			return sqlc.UpdateProductRow{}, utils.NewError(utils.ErrCodeBadRequest, "invalid category uuid")
		}

		category, err := ps.categoryRepo.GetByUUID(reqCtx, categoryUuid)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return sqlc.UpdateProductRow{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
			}
			return sqlc.UpdateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
		}
		input.Params.ProductCategoryID = &category.CategoryID
	}

	if input.Params.ProductName != nil {
		slug, err := generateUniqueSlug(reqCtx, *input.Params.ProductName, func(slugCtx context.Context, slug string) error {
			_, err := ps.repo.GetBySlug(slugCtx, slug)
			return err
		})
		if err != nil {
			return sqlc.UpdateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to generate slug", err)
		}

		input.Params.ProductSlug = &slug
	}

	if input.Params.ProductSku != nil {
		sku := strings.ToUpper(strings.TrimSpace(*input.Params.ProductSku))
		if sku == "" {
			input.Params.ProductSku = nil
		} else {
			input.Params.ProductSku = &sku
		}
	}

	product, err := ps.repo.Update(reqCtx, input.Params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.UpdateProductRow{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
			return sqlc.UpdateProductRow{}, utils.NewError(utils.ErrCodeConflict, "product already exists")
		}
		return sqlc.UpdateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to update product", err)
	}

	if err := ps.cache.Clear("products:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
	}

	return product, nil
}

func (ps *productService) SoftDeleteProduct(ctx *gin.Context, uuid uuid.UUID) (sqlc.Product, error) {
	context := ctx.Request.Context()

	product, err := ps.repo.SoftDelete(context, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.Product{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return sqlc.Product{}, utils.WrapError(utils.ErrCodeInternal, "failed to soft delete product", err)
	}

	if err := ps.cache.Clear("products:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
	}

	return product, nil
}

func (ps *productService) RestoreProduct(ctx *gin.Context, uuid uuid.UUID) (sqlc.Product, error) {
	context := ctx.Request.Context()

	product, err := ps.repo.Restore(context, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.Product{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return sqlc.Product{}, utils.WrapError(utils.ErrCodeInternal, "failed to restore product", err)
	}
	if err := ps.cache.Clear("products:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
	}
	return product, nil
}

func (ps *productService) TrashProduct(ctx *gin.Context, uuid uuid.UUID) error {
	context := ctx.Request.Context()

	_, err := ps.repo.Trash(context, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to trash product", err)
	}
	if err := ps.cache.Clear("products:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
	}
	return nil
}
