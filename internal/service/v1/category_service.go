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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
)

const maxSlugAttempts = 100

type categoryService struct {
	repo  repository.CategoryRepository
	cache cache.RedisCacheService
}

func NewCategoryService(repo repository.CategoryRepository, redisClient *redis.Client) CategoryService {
	return &categoryService{
		repo:  repo,
		cache: cache.NewRedisCacheService(redisClient),
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

		if errors.Is(err, sql.ErrNoRows) {
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

func (cs *categoryService) CreateCategory(ctx *gin.Context, input sqlc.CreateCategoryParams) (sqlc.Category, error) {
	reqCtx := ctx.Request.Context()

	if input.CategoryStatus == 0 {
		input.CategoryStatus = 1
	}

	slug, err := generateUniqueSlug(reqCtx, input.CategoryName, func(slugCtx context.Context, slug string) error {
		_, err := cs.repo.GetBySlug(slugCtx, slug)
		return err
	})
	if err != nil {
		return sqlc.Category{}, utils.WrapError(utils.ErrCodeInternal, "failed to generate unique slug", err)
	}
	input.CategorySlug = slug

	category, err := cs.repo.Create(reqCtx, input)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return sqlc.Category{}, utils.NewError(utils.ErrCodeConflict, "category already exists")
		}
		return sqlc.Category{}, utils.WrapError(utils.ErrCodeInternal, "failed to create category", err)
	}

	if err := cs.cache.Clear("categories:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear category cache")
	}

	return category, nil
}

func (cs *categoryService) GetAllCategories(ctx *gin.Context, search, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.Category, int32, error) {
	context := ctx.Request.Context()

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

	if err := cs.cache.Get(cacheKey, &cacheData); err == nil && cacheData.Categories != nil {
		return cacheData.Categories, cacheData.Total, nil
	}

	listCategories, err := cs.repo.GetAll(context, search, orderBy, sort, limit, offset, deleted)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "failed to get all categories", err)
	}

	total, err := cs.repo.CountCategories(context, search, deleted)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count categories", err)
	}

	cacheData = struct {
		Categories []sqlc.Category `json:"categories"`
		Total      int32           `json:"total"`
	}{
		Categories: listCategories,
		Total:      int32(total),
	}
	cs.cache.Set(cacheKey, cacheData, 5*time.Minute)

	return listCategories, cacheData.Total, nil
}

func (cs *categoryService) GetCategoryByUUID(ctx *gin.Context, uuid uuid.UUID) (sqlc.Category, error) {
	context := ctx.Request.Context()

	category, err := cs.repo.GetByUUID(context, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.Category{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return sqlc.Category{}, utils.WrapError(utils.ErrCodeInternal, "failed to get category by UUID", err)
	}

	return category, nil
}

func (cs *categoryService) UpdateCategory(ctx *gin.Context, input sqlc.UpdateCategoryParams) (sqlc.Category, error) {

	context := ctx.Request.Context()

	category, err := cs.repo.Update(context, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.Category{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return sqlc.Category{}, utils.WrapError(utils.ErrCodeInternal, "failed to update category", err)
	}

	if err := cs.cache.Clear("categories:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear category cache")
	}

	return category, nil
}

func (cs *categoryService) SoftDeleteCategory(ctx *gin.Context, uuid uuid.UUID) (sqlc.Category, error) {
	context := ctx.Request.Context()

	category, err := cs.repo.SoftDelete(context, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.Category{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return sqlc.Category{}, utils.WrapError(utils.ErrCodeInternal, "failed to soft delete category", err)
	}

	if err := cs.cache.Clear("categories:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear category cache")
	}

	return category, nil
}

func (cs *categoryService) RestoreCategory(ctx *gin.Context, uuid uuid.UUID) (sqlc.Category, error) {
	context := ctx.Request.Context()

	category, err := cs.repo.Restore(context, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sqlc.Category{}, utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return sqlc.Category{}, utils.WrapError(utils.ErrCodeInternal, "failed to restore category", err)
	}
	if err := cs.cache.Clear("categories:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear category cache")
	}
	return category, nil
}

func (cs *categoryService) TrashCategory(ctx *gin.Context, uuid uuid.UUID) error {
	context := ctx.Request.Context()

	_, err := cs.repo.Trash(context, uuid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "category not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to trash category", err)
	}
	if err := cs.cache.Clear("categories:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear category cache")
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
