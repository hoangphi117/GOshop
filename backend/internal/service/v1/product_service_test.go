package v1service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
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

func TestProductService_CreateProduct(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Invalid Category UUID - Returns 400 Bad Request", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{}
		mockCatRepo := &testutils.MockCategoryRepository{}
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil)

		_, err := svc.CreateProduct(ctx, v1dto.CreateProductInput{
			CategoryUUID: "invalid-uuid",
			Params: sqlc.CreateProductParams{
				ProductName: "Gaming Laptop",
			},
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "invalid category uuid")
	})

	t.Run("Case 2: Category Not Found (pgx.ErrNoRows) - Returns 404", func(t *testing.T) {
		categoryUUID := uuid.New()
		mockRepo := &testutils.MockProductRepository{}
		mockCatRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{}, pgx.ErrNoRows
			},
		}
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil)

		_, err := svc.CreateProduct(ctx, v1dto.CreateProductInput{
			CategoryUUID: categoryUUID.String(),
			Params: sqlc.CreateProductParams{
				ProductName: "Gaming Laptop",
			},
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
		assert.Contains(t, err.Error(), "category not found")
	})

	t.Run("Case 3: Category Repo Error - Returns 500", func(t *testing.T) {
		categoryUUID := uuid.New()
		mockRepo := &testutils.MockProductRepository{}
		mockCatRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{}, errors.New("db connection failure")
			},
		}
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil)

		_, err := svc.CreateProduct(ctx, v1dto.CreateProductInput{
			CategoryUUID: categoryUUID.String(),
			Params: sqlc.CreateProductParams{
				ProductName: "Gaming Laptop",
			},
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 4: Empty Name for Slug - Returns 400 Bad Request", func(t *testing.T) {
		categoryUUID := uuid.New()
		mockRepo := &testutils.MockProductRepository{}
		mockCatRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{CategoryID: 10, CategoryUuid: categoryUUID}, nil
			},
		}
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil)

		_, err := svc.CreateProduct(ctx, v1dto.CreateProductInput{
			CategoryUUID: categoryUUID.String(),
			Params: sqlc.CreateProductParams{
				ProductName: "   ",
			},
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "alphanumeric")
	})

	t.Run("Case 5: Unique Slug Generation on Collision - Success", func(t *testing.T) {
		categoryUUID := uuid.New()
		productUUID := uuid.New()
		var createdSlug string
		mockCatRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{CategoryID: 10, CategoryUuid: categoryUUID, CategoryName: "Electronics"}, nil
			},
		}
		mockRepo := &testutils.MockProductRepository{
			GetBySlugFunc: func(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error) {
				if slug == "gaming-mouse" {
					return sqlc.GetProductBySlugRow{ProductID: 1}, nil
				}
				return sqlc.GetProductBySlugRow{}, pgx.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, params sqlc.CreateProductParams) (sqlc.CreateProductRow, error) {
				createdSlug = params.ProductSlug
				return sqlc.CreateProductRow{
					ProductID:         2,
					ProductUuid:       productUUID,
					ProductName:       params.ProductName,
					ProductSlug:       params.ProductSlug,
					ProductCategoryID: params.ProductCategoryID,
					ProductStatus:     params.ProductStatus,
				}, nil
			},
		}

		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil)

		res, err := svc.CreateProduct(ctx, v1dto.CreateProductInput{
			CategoryUUID: categoryUUID.String(),
			Params: sqlc.CreateProductParams{
				ProductName: "Gaming Mouse",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "gaming-mouse-1", createdSlug)
		assert.Equal(t, productUUID, res.ProductUuid)
		assert.Equal(t, int32(1), res.ProductStatus)
	})

	t.Run("Case 6: Duplicate Conflict 23505 - Returns 409 Conflict", func(t *testing.T) {
		categoryUUID := uuid.New()
		mockCatRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{CategoryID: 10, CategoryUuid: categoryUUID}, nil
			},
		}
		mockRepo := &testutils.MockProductRepository{
			GetBySlugFunc: func(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error) {
				return sqlc.GetProductBySlugRow{}, pgx.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, params sqlc.CreateProductParams) (sqlc.CreateProductRow, error) {
				return sqlc.CreateProductRow{}, &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
			},
		}

		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil)

		_, err := svc.CreateProduct(ctx, v1dto.CreateProductInput{
			CategoryUUID: categoryUUID.String(),
			Params: sqlc.CreateProductParams{
				ProductName: "Gaming Mouse",
			},
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
		assert.Contains(t, err.Error(), "product already exists")
	})

	t.Run("Case 7: Repository Error on Create - Returns 500", func(t *testing.T) {
		categoryUUID := uuid.New()
		mockCatRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{CategoryID: 10, CategoryUuid: categoryUUID}, nil
			},
		}
		mockRepo := &testutils.MockProductRepository{
			GetBySlugFunc: func(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error) {
				return sqlc.GetProductBySlugRow{}, pgx.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, params sqlc.CreateProductParams) (sqlc.CreateProductRow, error) {
				return sqlc.CreateProductRow{}, errors.New("insert failed")
			},
		}

		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil)

		_, err := svc.CreateProduct(ctx, v1dto.CreateProductInput{
			CategoryUUID: categoryUUID.String(),
			Params: sqlc.CreateProductParams{
				ProductName: "Gaming Mouse",
			},
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 8: Happy Path with SKU Normalization - Success", func(t *testing.T) {
		categoryUUID := uuid.New()
		productUUID := uuid.New()
		skuInput := "  sku-123-abc  "
		var storedSKU *string

		mockCatRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{CategoryID: 5, CategoryUuid: categoryUUID}, nil
			},
		}
		mockRepo := &testutils.MockProductRepository{
			GetBySlugFunc: func(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error) {
				return sqlc.GetProductBySlugRow{}, pgx.ErrNoRows
			},
			CreateFunc: func(ctx context.Context, params sqlc.CreateProductParams) (sqlc.CreateProductRow, error) {
				storedSKU = params.ProductSku
				return sqlc.CreateProductRow{
					ProductID:         10,
					ProductUuid:       productUUID,
					ProductName:       params.ProductName,
					ProductSlug:       params.ProductSlug,
					ProductSku:        params.ProductSku,
					ProductCategoryID: params.ProductCategoryID,
					ProductStatus:     params.ProductStatus,
				}, nil
			},
		}

		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil)

		res, err := svc.CreateProduct(ctx, v1dto.CreateProductInput{
			CategoryUUID: categoryUUID.String(),
			Params: sqlc.CreateProductParams{
				ProductName:   "Mechanical Keyboard",
				ProductSku:    &skuInput,
				ProductStatus: 2, // Inactive
			},
		})
		require.NoError(t, err)
		assert.Equal(t, productUUID, res.ProductUuid)
		require.NotNil(t, storedSKU)
		assert.Equal(t, "SKU-123-ABC", *storedSKU)
		assert.Equal(t, int32(2), res.ProductStatus)
	})
}

func TestProductService_GetAllProducts(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Happy Path - Queries Repo and Returns Products with Total", func(t *testing.T) {
		productUUID := uuid.New()
		mockRepo := &testutils.MockProductRepository{
			GetAllFunc: func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.GetProductByUUIDRow, error) {
				assert.Equal(t, "laptop", search)
				assert.Equal(t, int32(10), limit)
				assert.Equal(t, int32(0), offset)
				assert.Equal(t, "product_price", orderBy)
				assert.Equal(t, "asc", sort)
				return []sqlc.GetProductByUUIDRow{
					{
						ProductID:   1,
						ProductUuid: productUUID,
						ProductName: "Laptop Pro",
					},
				}, nil
			},
			CountProductsFunc: func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, deleted bool) (int64, error) {
				return 1, nil
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		products, total, err := svc.GetAllProducts(ctx, "laptop", "", 100, 1000, 1, "product_price", "asc", 1, 10, false)
		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Len(t, products, 1)
		assert.Equal(t, productUUID, products[0].ProductUuid)
	})

	t.Run("Case 2: Default Parameter Fallbacks", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			GetAllFunc: func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.GetProductByUUIDRow, error) {
				assert.Equal(t, "desc", sort)
				assert.Equal(t, "product_created_at", orderBy)
				assert.Equal(t, int32(0), offset)
				return []sqlc.GetProductByUUIDRow{}, nil
			},
			CountProductsFunc: func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, deleted bool) (int64, error) {
				return 0, nil
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		products, total, err := svc.GetAllProducts(ctx, "", "", 0, 0, 0, "", "", 0, 0, false)
		require.NoError(t, err)
		assert.Equal(t, int32(0), total)
		assert.Empty(t, products)
	})

	t.Run("Case 3: Repo GetAll Error - Returns 500", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			GetAllFunc: func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.GetProductByUUIDRow, error) {
				return nil, errors.New("db query error")
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		_, _, err := svc.GetAllProducts(ctx, "", "", 0, 0, 0, "", "", 1, 10, false)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 4: Repo CountProducts Error - Returns 500", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			GetAllFunc: func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.GetProductByUUIDRow, error) {
				return []sqlc.GetProductByUUIDRow{}, nil
			},
			CountProductsFunc: func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice float64, status int32, deleted bool) (int64, error) {
				return 0, errors.New("count query error")
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		_, _, err := svc.GetAllProducts(ctx, "", "", 0, 0, 0, "", "", 1, 10, false)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})
}

func TestProductService_GetProductByUUID(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()

	t.Run("Case 1: Product Not Found (pgx.ErrNoRows) - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		_, err := svc.GetProductByUUID(ctx, productUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
		assert.Contains(t, err.Error(), "product not found")
	})

	t.Run("Case 2: Product Not Found (sql.ErrNoRows) - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		_, err := svc.GetProductByUUID(ctx, productUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Repository Error - Returns 500", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{}, errors.New("db connection failure")
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		_, err := svc.GetProductByUUID(ctx, productUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 4: Happy Path - Returns Product", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				assert.Equal(t, productUUID, u)
				return sqlc.GetProductByUUIDRow{
					ProductID:   1,
					ProductUuid: productUUID,
					ProductName: "Gaming Laptop",
				}, nil
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		res, err := svc.GetProductByUUID(ctx, productUUID)
		require.NoError(t, err)
		assert.Equal(t, productUUID, res.ProductUuid)
		assert.Equal(t, "Gaming Laptop", res.ProductName)
	})
}

func TestProductService_UpdateProduct(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()

	t.Run("Case 1: Invalid Category UUID - Returns 400 Bad Request", func(t *testing.T) {
		invalidCatStr := "invalid-uuid"
		mockRepo := &testutils.MockProductRepository{}
		mockCatRepo := &testutils.MockCategoryRepository{}
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil)

		_, err := svc.UpdateProduct(ctx, v1dto.UpdateProductInput{
			CategoryUUID: &invalidCatStr,
			Params: sqlc.UpdateProductParams{
				ProductUuid: productUUID,
			},
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "invalid category uuid")
	})

	t.Run("Case 2: Category Not Found - Returns 404", func(t *testing.T) {
		categoryUUID := uuid.New()
		catStr := categoryUUID.String()
		mockRepo := &testutils.MockProductRepository{}
		mockCatRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{}, pgx.ErrNoRows
			},
		}
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil)

		_, err := svc.UpdateProduct(ctx, v1dto.UpdateProductInput{
			CategoryUUID: &catStr,
			Params: sqlc.UpdateProductParams{
				ProductUuid: productUUID,
			},
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
		assert.Contains(t, err.Error(), "category not found")
	})

	t.Run("Case 3: Product Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			UpdateFunc: func(ctx context.Context, params sqlc.UpdateProductParams) (sqlc.UpdateProductRow, error) {
				return sqlc.UpdateProductRow{}, pgx.ErrNoRows
			},
		}
		svc := v1service.NewProductService(mockRepo, nil, nil)

		_, err := svc.UpdateProduct(ctx, v1dto.UpdateProductInput{
			Params: sqlc.UpdateProductParams{
				ProductUuid: productUUID,
			},
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
		assert.Contains(t, err.Error(), "product not found")
	})

	t.Run("Case 4: Unique Constraint Conflict 23505 - Returns 409", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			UpdateFunc: func(ctx context.Context, params sqlc.UpdateProductParams) (sqlc.UpdateProductRow, error) {
				return sqlc.UpdateProductRow{}, &pgconn.PgError{Code: "23505", Message: "duplicate key"}
			},
		}
		svc := v1service.NewProductService(mockRepo, nil, nil)

		_, err := svc.UpdateProduct(ctx, v1dto.UpdateProductInput{
			Params: sqlc.UpdateProductParams{
				ProductUuid: productUUID,
			},
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
		assert.Contains(t, err.Error(), "product already exists")
	})

	t.Run("Case 5: Happy Path with Name and SKU Update - Success", func(t *testing.T) {
		categoryUUID := uuid.New()
		catStr := categoryUUID.String()
		newName := "Updated Gaming Laptop"
		newSKU := "  sku-updated-999  "
		var storedSlug *string
		var storedSKU *string

		mockCatRepo := &testutils.MockCategoryRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Category, error) {
				return sqlc.Category{CategoryID: 15, CategoryUuid: categoryUUID}, nil
			},
		}
		mockRepo := &testutils.MockProductRepository{
			GetBySlugFunc: func(ctx context.Context, slug string) (sqlc.GetProductBySlugRow, error) {
				return sqlc.GetProductBySlugRow{}, pgx.ErrNoRows
			},
			UpdateFunc: func(ctx context.Context, params sqlc.UpdateProductParams) (sqlc.UpdateProductRow, error) {
				storedSlug = params.ProductSlug
				storedSKU = params.ProductSku
				return sqlc.UpdateProductRow{
					ProductID:         1,
					ProductUuid:       productUUID,
					ProductName:       newName,
					ProductSlug:       *params.ProductSlug,
					ProductSku:        params.ProductSku,
					ProductCategoryID: *params.ProductCategoryID,
				}, nil
			},
		}

		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil)

		res, err := svc.UpdateProduct(ctx, v1dto.UpdateProductInput{
			CategoryUUID: &catStr,
			Params: sqlc.UpdateProductParams{
				ProductUuid: productUUID,
				ProductName: &newName,
				ProductSku:  &newSKU,
			},
		})
		require.NoError(t, err)
		assert.Equal(t, productUUID, res.ProductUuid)
		require.NotNil(t, storedSlug)
		assert.Equal(t, "updated-gaming-laptop", *storedSlug)
		require.NotNil(t, storedSKU)
		assert.Equal(t, "SKU-UPDATED-999", *storedSKU)
	})
}

func TestProductService_SoftDeleteProduct(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()

	t.Run("Case 1: Product Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			SoftDeleteFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Product, error) {
				return sqlc.Product{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		_, err := svc.SoftDeleteProduct(ctx, productUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - Soft Deletes Product", func(t *testing.T) {
		now := time.Now()
		mockRepo := &testutils.MockProductRepository{
			SoftDeleteFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Product, error) {
				assert.Equal(t, productUUID, u)
				return sqlc.Product{
					ProductID:        1,
					ProductUuid:      productUUID,
					ProductName:      "Gaming Mouse",
					ProductDeletedAt: pgtype.Timestamptz{Time: now, Valid: true},
				}, nil
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		res, err := svc.SoftDeleteProduct(ctx, productUUID)
		require.NoError(t, err)
		assert.Equal(t, productUUID, res.ProductUuid)
		assert.True(t, res.ProductDeletedAt.Valid)
	})
}

func TestProductService_RestoreProduct(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()

	t.Run("Case 1: Product Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			RestoreFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Product, error) {
				return sqlc.Product{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		_, err := svc.RestoreProduct(ctx, productUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - Restores Product", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			RestoreFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Product, error) {
				assert.Equal(t, productUUID, u)
				return sqlc.Product{
					ProductID:        1,
					ProductUuid:      productUUID,
					ProductName:      "Gaming Mouse",
					ProductDeletedAt: pgtype.Timestamptz{Valid: false},
				}, nil
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		res, err := svc.RestoreProduct(ctx, productUUID)
		require.NoError(t, err)
		assert.Equal(t, productUUID, res.ProductUuid)
		assert.False(t, res.ProductDeletedAt.Valid)
	})
}

func TestProductService_TrashProduct(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()

	t.Run("Case 1: Product Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			TrashFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Product, error) {
				return sqlc.Product{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		err := svc.TrashProduct(ctx, productUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - Trashes Product Permanently", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			TrashFunc: func(ctx context.Context, u uuid.UUID) (sqlc.Product, error) {
				assert.Equal(t, productUUID, u)
				return sqlc.Product{
					ProductID:   1,
					ProductUuid: productUUID,
					ProductName: "Gaming Mouse",
				}, nil
			},
		}

		svc := v1service.NewProductService(mockRepo, nil, nil)

		err := svc.TrashProduct(ctx, productUUID)
		require.NoError(t, err)
	})
}
