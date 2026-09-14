package v1service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCategoryService_CreateCategory(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Empty or Invalid Name for Slug - Returns 400 Bad Request", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{}
		svc := v1service.NewCategoryService(mockRepo, nil)

		_, err := svc.CreateCategory(ctx, sqlc.CreateCategoryParams{
			CategoryName: "   ",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "alphanumeric")
	})

	t.Run("Case 2: Unique Slug Generation on Collision - Success", func(t *testing.T) {
		categoryUUID := uuid.New()
		var createdSlug string

		mockRepo := &testutils.MockCategoryRepository{
			GetBySlugFunc: func(ctx context.Context, slug string) (sqlc.Category, error) {
				if slug == "smartphones" {
					// First attempt collides with existing
					return sqlc.Category{CategoryID: 1, CategorySlug: "smartphones"}, nil
				}
				// Next attempt "smartphones-1" is available
				return sqlc.Category{}, pgx.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, params sqlc.CreateCategoryParams) (sqlc.Category, error) {
				createdSlug = params.CategorySlug
				assert.Equal(t, "smartphones-1", params.CategorySlug)
				assert.Equal(t, v1service.CategoryStatusActive, params.CategoryStatus)
				return sqlc.Category{
					CategoryID:        10,
					CategoryUuid:      categoryUUID,
					CategoryName:      params.CategoryName,
					CategorySlug:      params.CategorySlug,
					CategoryStatus:    params.CategoryStatus,
					CategoryCreatedAt: time.Now(),
					CategoryUpdatedAt: time.Now(),
				}, nil
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		res, err := svc.CreateCategory(ctx, sqlc.CreateCategoryParams{
			CategoryName: "Smartphones",
		})
		require.NoError(t, err)
		assert.Equal(t, categoryUUID, res.CategoryUuid)
		assert.Equal(t, "smartphones-1", createdSlug)
		assert.Equal(t, v1service.CategoryStatusActive, res.CategoryStatus)
	})

	t.Run("Case 3: Slug Generation Collision Exceeds Max Attempts - Returns 500", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			GetBySlugFunc: func(ctx context.Context, slug string) (sqlc.Category, error) {
				// All attempts collide
				return sqlc.Category{CategoryID: 1, CategorySlug: slug}, nil
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		_, err := svc.CreateCategory(ctx, sqlc.CreateCategoryParams{
			CategoryName: "Laptops",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
		assert.Contains(t, err.Error(), "Failed to generate unique slug")
	})

	t.Run("Case 4: Unique Constraint Conflict 23505 - Returns 409 Conflict", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			GetBySlugFunc: func(ctx context.Context, slug string) (sqlc.Category, error) {
				return sqlc.Category{}, pgx.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, params sqlc.CreateCategoryParams) (sqlc.Category, error) {
				return sqlc.Category{}, &pgconn.PgError{Code: "23505"}
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		_, err := svc.CreateCategory(ctx, sqlc.CreateCategoryParams{
			CategoryName: "Electronics",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("Case 5: Repository Error on Create - Returns 500 Internal", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			GetBySlugFunc: func(ctx context.Context, slug string) (sqlc.Category, error) {
				return sqlc.Category{}, pgx.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, params sqlc.CreateCategoryParams) (sqlc.Category, error) {
				return sqlc.Category{}, errors.New("db connection lost")
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		_, err := svc.CreateCategory(ctx, sqlc.CreateCategoryParams{
			CategoryName: "Audio",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 6: Happy Path - Creates Category with Given Status", func(t *testing.T) {
		categoryUUID := uuid.New()
		desc := "Audio accessories and headphones"

		mockRepo := &testutils.MockCategoryRepository{
			GetBySlugFunc: func(ctx context.Context, slug string) (sqlc.Category, error) {
				return sqlc.Category{}, pgx.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, params sqlc.CreateCategoryParams) (sqlc.Category, error) {
				assert.Equal(t, "audio", params.CategorySlug)
				assert.Equal(t, int32(2), params.CategoryStatus)
				return sqlc.Category{
					CategoryID:          20,
					CategoryUuid:        categoryUUID,
					CategoryName:        params.CategoryName,
					CategorySlug:        params.CategorySlug,
					CategoryDescription: params.CategoryDescription,
					CategoryStatus:      params.CategoryStatus,
					CategoryCreatedAt:   time.Now(),
					CategoryUpdatedAt:   time.Now(),
				}, nil
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		res, err := svc.CreateCategory(ctx, sqlc.CreateCategoryParams{
			CategoryName:        "Audio",
			CategoryDescription: &desc,
			CategoryStatus:      2,
		})
		require.NoError(t, err)
		assert.Equal(t, categoryUUID, res.CategoryUuid)
		assert.Equal(t, "audio", res.CategorySlug)
		assert.Equal(t, int32(2), res.CategoryStatus)
	})
}

func TestCategoryService_GetAllCategories(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Happy Path - Queries Repo and Returns List with Total", func(t *testing.T) {
		catUUID1 := uuid.New()
		catUUID2 := uuid.New()

		mockRepo := &testutils.MockCategoryRepository{
			GetAllFunc: func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.Category, error) {
				assert.Equal(t, "phone", search)
				assert.Equal(t, "category_created_at", orderBy)
				assert.Equal(t, "desc", sort)
				assert.Equal(t, int32(10), limit)
				assert.Equal(t, int32(0), offset)
				assert.False(t, deleted)
				return []sqlc.Category{
					{CategoryID: 1, CategoryUuid: catUUID1, CategoryName: "Phone 1"},
					{CategoryID: 2, CategoryUuid: catUUID2, CategoryName: "Phone 2"},
				}, nil
			},
			CountCategoriesFunc: func(ctx context.Context, search string, deleted bool) (int64, error) {
				assert.Equal(t, "phone", search)
				assert.False(t, deleted)
				return 2, nil
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		items, total, err := svc.GetAllCategories(ctx, "phone", "", "", 1, 10, false)
		require.NoError(t, err)
		assert.Equal(t, int32(2), total)
		assert.Len(t, items, 2)
		assert.Equal(t, catUUID1, items[0].CategoryUuid)
	})

	t.Run("Case 2: Default Fallback Parameters", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			GetAllFunc: func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.Category, error) {
				assert.Equal(t, "", search)
				assert.Equal(t, "category_created_at", orderBy)
				assert.Equal(t, "desc", sort)
				assert.Equal(t, int32(10), limit)
				assert.Equal(t, int32(0), offset)
				return []sqlc.Category{}, nil
			},
			CountCategoriesFunc: func(ctx context.Context, search string, deleted bool) (int64, error) {
				return 0, nil
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		items, total, err := svc.GetAllCategories(ctx, "", "", "", 0, 0, false)
		require.NoError(t, err)
		assert.Equal(t, int32(0), total)
		assert.Empty(t, items)
	})

	t.Run("Case 3: Repo GetAll Error - Returns 500", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			GetAllFunc: func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.Category, error) {
				return nil, errors.New("db query error")
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		_, _, err := svc.GetAllCategories(ctx, "", "", "", 1, 10, false)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 4: Repo CountCategories Error - Returns 500", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			GetAllFunc: func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.Category, error) {
				return []sqlc.Category{}, nil
			},
			CountCategoriesFunc: func(ctx context.Context, search string, deleted bool) (int64, error) {
				return 0, errors.New("count query error")
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		_, _, err := svc.GetAllCategories(ctx, "", "", "", 1, 10, false)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})
}

func TestCategoryService_GetCategoryByUUID(t *testing.T) {
	ctx := context.Background()
	categoryUUID := uuid.New()

	t.Run("Case 1: Category Not Found (pgx.ErrNoRows) - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		_, err := svc.GetCategoryByUUID(ctx, categoryUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Category Not Found (sql.ErrNoRows) - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		_, err := svc.GetCategoryByUUID(ctx, categoryUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Repository Error - Returns 500", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{}, errors.New("connection failed")
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		_, err := svc.GetCategoryByUUID(ctx, categoryUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 4: Happy Path - Returns Category", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{
					CategoryID:   1,
					CategoryUuid: categoryUUID,
					CategoryName: "Keyboards",
					CategorySlug: "keyboards",
				}, nil
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		res, err := svc.GetCategoryByUUID(ctx, categoryUUID)
		require.NoError(t, err)
		assert.Equal(t, categoryUUID, res.CategoryUuid)
		assert.Equal(t, "Keyboards", res.CategoryName)
	})
}

func TestCategoryService_UpdateCategory(t *testing.T) {
	ctx := context.Background()
	categoryUUID := uuid.New()
	newName := "Mechanical Keyboards"

	t.Run("Case 1: Category Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			UpdateFunc: func(ctx context.Context, input sqlc.UpdateCategoryParams) (sqlc.Category, error) {
				return sqlc.Category{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		_, err := svc.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
			CategoryUuid: categoryUUID,
			CategoryName: &newName,
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Repository Error - Returns 500", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			UpdateFunc: func(ctx context.Context, input sqlc.UpdateCategoryParams) (sqlc.Category, error) {
				return sqlc.Category{}, errors.New("update timeout")
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		_, err := svc.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
			CategoryUuid: categoryUUID,
			CategoryName: &newName,
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 3: Happy Path - Successfully Updates Category", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			UpdateFunc: func(ctx context.Context, input sqlc.UpdateCategoryParams) (sqlc.Category, error) {
				assert.Equal(t, categoryUUID, input.CategoryUuid)
				assert.Equal(t, &newName, input.CategoryName)
				return sqlc.Category{
					CategoryID:   1,
					CategoryUuid: categoryUUID,
					CategoryName: newName,
					CategorySlug: "keyboards",
				}, nil
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		res, err := svc.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
			CategoryUuid: categoryUUID,
			CategoryName: &newName,
		})
		require.NoError(t, err)
		assert.Equal(t, categoryUUID, res.CategoryUuid)
		assert.Equal(t, newName, res.CategoryName)
	})
}

func TestCategoryService_SoftDeleteCategory(t *testing.T) {
	ctx := context.Background()
	categoryUUID := uuid.New()

	t.Run("Case 1: Category Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			SoftDeleteFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		_, err := svc.SoftDeleteCategory(ctx, categoryUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - Soft Deletes Category", func(t *testing.T) {
		now := time.Now()
		mockRepo := &testutils.MockCategoryRepository{
			SoftDeleteFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				assert.Equal(t, categoryUUID, u)
				return sqlc.Category{
					CategoryID:   1,
					CategoryUuid: categoryUUID,
					CategoryName: "Monitors",
					CategoryDeletedAt: pgtype.Timestamptz{
						Time:  now,
						Valid: true,
					},
				}, nil
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		res, err := svc.SoftDeleteCategory(ctx, categoryUUID)
		require.NoError(t, err)
		assert.Equal(t, categoryUUID, res.CategoryUuid)
		assert.True(t, res.CategoryDeletedAt.Valid)
	})
}

func TestCategoryService_RestoreCategory(t *testing.T) {
	ctx := context.Background()
	categoryUUID := uuid.New()

	t.Run("Case 1: Category Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			RestoreFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		_, err := svc.RestoreCategory(ctx, categoryUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - Restores Category", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			RestoreFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				assert.Equal(t, categoryUUID, u)
				return sqlc.Category{
					CategoryID:        1,
					CategoryUuid:      categoryUUID,
					CategoryName:      "Monitors",
					CategoryDeletedAt: pgtype.Timestamptz{Valid: false},
				}, nil
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		res, err := svc.RestoreCategory(ctx, categoryUUID)
		require.NoError(t, err)
		assert.Equal(t, categoryUUID, res.CategoryUuid)
		assert.False(t, res.CategoryDeletedAt.Valid)
	})
}

func TestCategoryService_TrashCategory(t *testing.T) {
	ctx := context.Background()
	categoryUUID := uuid.New()

	t.Run("Case 1: Category Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockCategoryRepository{
			TrashFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		err := svc.TrashCategory(ctx, categoryUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - Trashes Category Permanently", func(t *testing.T) {
		var trashedUUID uuid.UUID
		mockRepo := &testutils.MockCategoryRepository{
			TrashFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				trashedUUID = u
				return sqlc.Category{CategoryID: 1, CategoryUuid: u}, nil
			},
		}

		svc := v1service.NewCategoryService(mockRepo, nil)

		err := svc.TrashCategory(ctx, categoryUUID)
		require.NoError(t, err)
		assert.Equal(t, categoryUUID, trashedUUID)
	})
}
