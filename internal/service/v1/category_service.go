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

	"github.com/gin-gonic/gin"
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
