package v1service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"goshop/internal/db/sqlc"
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
	"golang.org/x/sync/singleflight"
)

const (
	maxSlugAttempts            = 100
	CategoryStatusActive int32 = 1
)

type categoryService struct {
	repo  repository.CategoryRepository
	cache cache.RedisCacheService
	sf    singleflight.Group
}

func NewCategoryService(repo repository.CategoryRepository, redisClient *redis.Client) CategoryService {
	var cacheSvc cache.RedisCacheService
	if redisClient != nil {
		cacheSvc = cache.NewRedisCacheService(redisClient)
	}
	return &categoryService{
		repo:  repo,
		cache: cacheSvc,
	}
}

func generateUniqueSlug(ctx context.Context, name string, getBySlug func(ctx context.Context, slug string) error) (string, error) {
	baseSlug := utils.Slugify(name)
	if baseSlug == "" {
		return "", utils.NewError(utils.ErrCodeBadRequest, "Name must contain at least one alphanumeric character to generate a slug")
	}

	slug := baseSlug

	for i := 0; i < maxSlugAttempts; i++ {
		err := getBySlug(ctx, slug)

		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			// Slug is available, return it.
			return slug, nil
		}
		if err != nil {
			// Other error occurred while checking slug availability.
			return "", utils.WrapError(utils.ErrCodeInternal, "Failed to check slug availability", err)
		}

		// Found an existing category/product with the same slug, try the next suffix.
		slug = fmt.Sprintf("%s-%d", baseSlug, i+1)
	}

	return "", utils.NewError(utils.ErrCodeInternal, fmt.Sprintf("Failed to generate unique slug after %d attempts", maxSlugAttempts))
}

func (cs *categoryService) CreateCategory(ctx context.Context, input sqlc.CreateCategoryParams) (sqlc.Category, error) {
	if input.CategoryStatus == 0 {
		input.CategoryStatus = CategoryStatusActive
	}

	slug, err := generateUniqueSlug(ctx, input.CategoryName, func(slugCtx context.Context, slug string) error {
		_, err := cs.repo.GetBySlug(slugCtx, slug)
		return err
	})
	if err != nil {
		var appErr *utils.AppError
		if errors.As(err, &appErr) {
			return sqlc.Category{}, err
		}
		return sqlc.Category{}, utils.WrapError(utils.ErrCodeInternal, "failed to generate unique slug", err)
	}
	input.CategorySlug = slug

	category, err := cs.repo.Create(ctx, input)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return sqlc.Category{}, utils.NewError(utils.ErrCodeConflict, "category already exists")
		}
		return sqlc.Category{}, utils.WrapError(utils.ErrCodeInternal, "failed to create category", err)
	}

	if cs.cache != nil {
		if err := cs.cache.Clear("categories:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear category cache")
		}
	}

	return category, nil
}

func (cs *categoryService) GetAllCategories(ctx context.Context, search, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.Category, int32, error) {
	if sort == "" {
		sort = "desc"
	}

	if orderBy == "" {
		orderBy = "category_created_at"
	}

	if page <= 0 {
		page = 1
	}

	if limit <= 0 {
		limitInt := utils.GetIntEnv("LIMIT_ITEM_PER_PAGE", 10)
		limit = int32(limitInt)
	}

	offset := (page - 1) * limit

	cacheKey := cs.generateCacheKey(search, orderBy, sort, page, limit, deleted)
	var cacheData struct {
		Categories []sqlc.Category `json:"categories"`
		Total      int32           `json:"total"`
	}

	if cs.cache != nil {
		if err := cs.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Categories != nil {
			return cacheData.Categories, cacheData.Total, nil
		}
	}

	type fetchResult struct {
		categories []sqlc.Category
		total      int64
	}

	res, err, _ := cs.sf.Do(cacheKey, func() (any, error) {
		// Re-check cache inside singleflight
		if cs.cache != nil {
			var cd struct {
				Categories []sqlc.Category `json:"categories"`
				Total      int32           `json:"total"`
			}
			if err := cs.cache.Get(cacheKey, &cd); err == nil && cd.Categories != nil {
				return fetchResult{categories: cd.Categories, total: int64(cd.Total)}, nil
			}
		}

		listCategories, err := cs.repo.GetAll(ctx, search, orderBy, sort, limit, offset, deleted)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get all categories", err)
		}

		total, err := cs.repo.CountCategories(ctx, search, deleted)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to count categories", err)
		}

		if cs.cache != nil {
			cd := struct {
				Categories []sqlc.Category `json:"categories"`
				Total      int32           `json:"total"`
			}{
				Categories: listCategories,
				Total:      int32(total),
			}
			cs.cache.Set(cacheKey, cd, 5*time.Minute)
		}

		return fetchResult{categories: listCategories, total: total}, nil
	})

	if err != nil {
		return nil, 0, err
	}

	fr := res.(fetchResult)
	return fr.categories, int32(fr.total), nil
}

func (cs *categoryService) GetCategoryByUUID(ctx context.Context, categoryUUID uuid.UUID) (sqlc.Category, error) {
	category, err := cs.repo.GetByUUID(ctx, categoryUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return sqlc.Category{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return sqlc.Category{}, utils.WrapError(utils.ErrCodeInternal, "failed to get category by UUID", err)
	}

	return category, nil
}

func (cs *categoryService) UpdateCategory(ctx context.Context, input sqlc.UpdateCategoryParams) (sqlc.Category, error) {
	category, err := cs.repo.Update(ctx, input)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return sqlc.Category{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return sqlc.Category{}, utils.WrapError(utils.ErrCodeInternal, "failed to update category", err)
	}

	if cs.cache != nil {
		if err := cs.cache.Clear("categories:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear category cache")
		}
	}

	return category, nil
}

func (cs *categoryService) SoftDeleteCategory(ctx context.Context, categoryUUID uuid.UUID) (sqlc.Category, error) {
	category, err := cs.repo.SoftDelete(ctx, categoryUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return sqlc.Category{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return sqlc.Category{}, utils.WrapError(utils.ErrCodeInternal, "failed to soft delete category", err)
	}

	if cs.cache != nil {
		if err := cs.cache.Clear("categories:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear category cache")
		}
	}

	return category, nil
}

func (cs *categoryService) RestoreCategory(ctx context.Context, categoryUUID uuid.UUID) (sqlc.Category, error) {
	category, err := cs.repo.Restore(ctx, categoryUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return sqlc.Category{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return sqlc.Category{}, utils.WrapError(utils.ErrCodeInternal, "failed to restore category", err)
	}

	if cs.cache != nil {
		if err := cs.cache.Clear("categories:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear category cache")
		}
	}

	return category, nil
}

func (cs *categoryService) TrashCategory(ctx context.Context, categoryUUID uuid.UUID) error {
	_, err := cs.repo.Trash(ctx, categoryUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to trash category", err)
	}

	if cs.cache != nil {
		if err := cs.cache.Clear("categories:*"); err != nil {
			logger.Log.Warn().Err(err).Msg("Failed to clear category cache")
		}
	}

	return nil
}

func (cs *categoryService) generateCacheKey(search, orderBy, sort string, page, limit int32, deleted bool) string {
	search = strings.TrimSpace(search)
	if search == "" {
		search = "none"
	}

	orderBy = strings.TrimSpace(orderBy)
	if orderBy == "" {
		orderBy = "category_created_at"
	}

	sort = strings.ToLower(strings.TrimSpace(sort))
	if sort == "" {
		sort = "desc"
	}
	return fmt.Sprintf("categories:%s:%s:%s:%d:%d:%t", search, orderBy, sort, page, limit, deleted)
}
