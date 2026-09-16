package v1service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

const (
	ProductStatusActive int32 = 1
)

type productService struct {
	repo         repository.ProductRepository
	categoryRepo repository.CategoryRepository
	cache        cache.RedisCacheService
}

func NewProductService(repo repository.ProductRepository, categoryRepo repository.CategoryRepository, redisClient *redis.Client) ProductService {
	var cacheService cache.RedisCacheService
	if redisClient != nil {
		cacheService = cache.NewRedisCacheService(redisClient)
	}
	return &productService{
		repo:         repo,
		categoryRepo: categoryRepo,
		cache:        cacheService,
	}
}

func (ps *productService) CreateProduct(ctx context.Context, input v1dto.CreateProductInput) (sqlc.CreateProductRow, error) {
	categoryUuid, err := uuid.Parse(input.CategoryUUID)
	if err != nil {
		return sqlc.CreateProductRow{}, utils.NewError(utils.ErrCodeBadRequest, "invalid category uuid")
	}

	category, err := ps.categoryRepo.GetByUUID(ctx, categoryUuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.CreateProductRow{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return sqlc.CreateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
	}
	input.Params.ProductCategoryID = category.CategoryID

	if input.Params.ProductStatus == 0 {
		input.Params.ProductStatus = ProductStatusActive
	}

	slug, err := generateUniqueSlug(ctx, input.Params.ProductName, func(slugCtx context.Context, slug string) error {
		_, err := ps.repo.GetBySlug(slugCtx, slug)
		return err
	})
	if err != nil {
		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return sqlc.CreateProductRow{}, err
		}
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

	product, err := ps.repo.Create(ctx, input.Params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return sqlc.CreateProductRow{}, utils.NewError(utils.ErrCodeConflict, "product already exists")
		}
		return sqlc.CreateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to create product", err)
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	return product, nil
}

func (ps *productService) GetAllProducts(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.GetProductByUUIDRow, int32, error) {
	if sort == "" {
		sort = "desc"
	}

	if orderBy == "" {
		orderBy = "product_created_at"
	}

	if page < 1 {
		page = 1
	}

	if limit < 1 {
		limitInt := utils.GetIntEnv("LIMIT_ITEM_PER_PAGE", 10)
		limit = int32(limitInt)
	}

	offset := (page - 1) * limit

	cacheKey := ps.generateCacheKey(search, categoryUUID, minPrice, maxPrice, status, orderBy, sort, page, limit, deleted)
	var cacheData struct {
		Products []sqlc.GetProductByUUIDRow `json:"products"`
		Total    int32                      `json:"total"`
	}

	if ps.cache != nil {
		if err := ps.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Products != nil {
			return cacheData.Products, cacheData.Total, nil
		}
	}

	listProducts, err := ps.repo.GetAll(ctx, search, categoryUUID, minPrice, maxPrice, status, orderBy, sort, limit, offset, deleted)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "failed to get all products", err)
	}

	total, err := ps.repo.CountProducts(ctx, search, categoryUUID, minPrice, maxPrice, status, deleted)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count products", err)
	}

	if ps.cache != nil {
		cacheData = struct {
			Products []sqlc.GetProductByUUIDRow `json:"products"`
			Total    int32                      `json:"total"`
		}{
			Products: listProducts,
			Total:    int32(total),
		}
		ps.cache.Set(cacheKey, cacheData, 5*time.Minute)
	}

	return listProducts, int32(total), nil
}

func (ps *productService) GetProductByUUID(ctx context.Context, productUUID uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
	product, err := ps.repo.GetByUUID(ctx, productUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetProductByUUIDRow{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return sqlc.GetProductByUUIDRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to get product", err)
	}

	return product, nil
}

func (ps *productService) GetProductBySlug(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error) {
	product, err := ps.repo.GetBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.GetProductBySlugRow{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return sqlc.GetProductBySlugRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to get product", err)
	}

	return product, nil
}

func (ps *productService) UpdateProduct(ctx context.Context, input v1dto.UpdateProductInput) (sqlc.UpdateProductRow, error) {
	if input.CategoryUUID != nil {
		categoryUuid, err := uuid.Parse(*input.CategoryUUID)
		if err != nil {
			return sqlc.UpdateProductRow{}, utils.NewError(utils.ErrCodeBadRequest, "invalid category uuid")
		}

		category, err := ps.categoryRepo.GetByUUID(ctx, categoryUuid)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
				return sqlc.UpdateProductRow{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
			}
			return sqlc.UpdateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to get category", err)
		}
		input.Params.ProductCategoryID = &category.CategoryID
	}

	if input.Params.ProductName != nil {
		slug, err := generateUniqueSlug(ctx, *input.Params.ProductName, func(slugCtx context.Context, slug string) error {
			_, err := ps.repo.GetBySlug(slugCtx, slug)
			return err
		})
		if err != nil {
			var appErr *utils.AppError
			if errors.As(err, &appErr) {
				return sqlc.UpdateProductRow{}, err
			}
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

	product, err := ps.repo.Update(ctx, input.Params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.UpdateProductRow{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return sqlc.UpdateProductRow{}, utils.NewError(utils.ErrCodeConflict, "product already exists")
		}
		return sqlc.UpdateProductRow{}, utils.WrapError(utils.ErrCodeInternal, "failed to update product", err)
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	return product, nil
}

func (ps *productService) SoftDeleteProduct(ctx context.Context, productUUID uuid.UUID) (sqlc.Product, error) {
	product, err := ps.repo.SoftDelete(ctx, productUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Product{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return sqlc.Product{}, utils.WrapError(utils.ErrCodeInternal, "failed to soft delete product", err)
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	return product, nil
}

func (ps *productService) RestoreProduct(ctx context.Context, productUUID uuid.UUID) (sqlc.Product, error) {
	product, err := ps.repo.Restore(ctx, productUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return sqlc.Product{}, utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return sqlc.Product{}, utils.WrapError(utils.ErrCodeInternal, "failed to restore product", err)
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	return product, nil
}

func (ps *productService) TrashProduct(ctx context.Context, productUUID uuid.UUID) error {
	_, err := ps.repo.Trash(ctx, productUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "product not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to trash product", err)
	}

	if ps.cache != nil {
		if err := ps.cache.Clear("products:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear product cache")
		}
	}

	return nil
}

func (ps *productService) generateCacheKey(search, categoryUUID string, minPrice, maxPrice float64, status int32, orderBy, sort string, page, limit int32, deleted bool) string {
	search = strings.TrimSpace(search)
	if search == "" {
		search = "none"
	}

	categoryUUID = strings.TrimSpace(categoryUUID)
	if categoryUUID == "" {
		categoryUUID = "none"
	}

	minPriceStr := fmt.Sprintf("%.2f", minPrice)
	maxPriceStr := fmt.Sprintf("%.2f", maxPrice)

	statusStr := fmt.Sprintf("%d", status)

	orderBy = strings.TrimSpace(orderBy)
	if orderBy == "" {
		orderBy = "product_created_at"
	}

	sort = strings.ToLower(strings.TrimSpace(sort))
	if sort == "" {
		sort = "desc"
	}
	return fmt.Sprintf("products:%s:%s:%s:%s:%s:%s:%s:%d:%d:%t", search, categoryUUID, minPriceStr, maxPriceStr, statusStr, orderBy, sort, page, limit, deleted)
}
