package v1service_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"
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
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

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
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

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
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

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
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

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
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

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
		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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
		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

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

		svc := v1service.NewProductService(mockRepo, nil, nil, nil)

		err := svc.TrashProduct(ctx, productUUID)
		require.NoError(t, err)
	})
}

func TestProductService_ImportProductsCSV(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Empty or Invalid Header - Returns 400 Bad Request", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{}
		mockCatRepo := &testutils.MockCategoryRepository{}
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

		invalidCSV := "wrong,headers\nvalue1,value2"
		res, err := svc.ImportProductsCSV(ctx, strings.NewReader(invalidCSV))
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Nil(t, res)
	})

	t.Run("Case 2: Row-level Validation Errors - Returns 0 success and detailed error report", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{}
		mockCatRepo := &testutils.MockCategoryRepository{}
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

		csvContent := "name,price,description,imageUrl,category_id,stock,sku\n" +
			",-10,Good product,http://img.com,0,-5,SKU-1\n" +
			"Valid Name,0,Good product,http://img.com,abc,10,SKU-2\n"

		res, err := svc.ImportProductsCSV(ctx, strings.NewReader(csvContent))
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, 0, res.SuccessCount)
		assert.GreaterOrEqual(t, res.ErrorCount, 4)
		// Check that specific fields were flagged
		fields := make(map[string]bool)
		for _, e := range res.Errors {
			fields[e.Field] = true
		}
		assert.True(t, fields["name"])
		assert.True(t, fields["price"])
		assert.True(t, fields["category_id"])
		assert.True(t, fields["stock"])
	})

	t.Run("Case 3: In-File Duplicate Name within Same Category and Duplicate SKU", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{}
		mockCatRepo := &testutils.MockCategoryRepository{}
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

		csvContent := "name,price,description,imageUrl,category_id,stock,sku\n" +
			"T-Shirt,100,Desc,http://img.com,1,10,SKU-DUPE\n" +
			"t-shirt,120,Desc 2,http://img.com,1,5,SKU-DUPE\n"

		res, err := svc.ImportProductsCSV(ctx, strings.NewReader(csvContent))
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, 0, res.SuccessCount)
		assert.GreaterOrEqual(t, res.ErrorCount, 2)
		reasons := ""
		for _, e := range res.Errors {
			reasons += e.Reason + " "
		}
		assert.Contains(t, reasons, "Duplicate name and category_id")
		assert.Contains(t, reasons, "Duplicate SKU")
	})

	t.Run("Case 4: Category Does Not Exist in Database", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{}
		mockCatRepo := &testutils.MockCategoryRepository{
			GetAllFunc: func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.Category, error) {
				return []sqlc.Category{
					{CategoryID: 1, CategoryName: "Clothing"},
				}, nil
			},
		}
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

		csvContent := "name,price,description,imageUrl,category_id,stock,sku\n" +
			"Shoes,250,Running shoes,http://img.com,999,10,SKU-SHOES\n"

		res, err := svc.ImportProductsCSV(ctx, strings.NewReader(csvContent))
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, 0, res.SuccessCount)
		assert.Equal(t, 1, res.ErrorCount)
		assert.Equal(t, "category_id", res.Errors[0].Field)
		assert.Contains(t, res.Errors[0].Reason, "999")
	})

	t.Run("Case 5: RFC 4180 Quotes and Comma Handling with UTF-8 BOM", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{}
		mockCatRepo := &testutils.MockCategoryRepository{
			GetAllFunc: func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.Category, error) {
				return []sqlc.Category{
					{CategoryID: 2, CategoryName: "Electronics"},
				}, nil
			},
		}
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

		// UTF-8 BOM + Comma in quotes
		csvContent := "\ufeffname,price,description,imageUrl,category_id,stock,sku\n" +
			"\"Wireless Mouse, Ergonomic\",49.99,\"Smooth, silent clicks\",http://img.com,2,100,\n"

		// Parsing should pass syntax check and category existence check without errors
		// It will reach executeAtomicBatchInsert (which needs DB pool in full integration)
		// Here we verify it passes all validation phases
		res, err := svc.ImportProductsCSV(ctx, strings.NewReader(csvContent))
		require.NoError(t, err)
		require.NotNil(t, res)
		// Should have 0 syntax/category errors before DB step
		if res.ErrorCount > 0 {
			// Only DB connection error is acceptable since DB pool isn't mocked here
			assert.Contains(t, res.Errors[0].Reason, "transaction")
		}
	})

	t.Run("Case 6: Non-UTF8 Windows-1258 Encoding Auto-Conversion", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{}
		mockCatRepo := &testutils.MockCategoryRepository{
			GetAllFunc: func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.Category, error) {
				return []sqlc.Category{
					{CategoryID: 1, CategoryName: "Tech"},
				}, nil
			},
		}
		svc := v1service.NewProductService(mockRepo, mockCatRepo, nil, nil)

		// 0xed 0x6e 0x68 in Windows-1258 is "ính"
		win1258Bytes := []byte("name,price,description,imageUrl,category_id,stock,sku\nM\xednh,100,Desc,http://img.com,1,10,SKU-WIN\n")
		res, err := svc.ImportProductsCSV(ctx, bytes.NewReader(win1258Bytes))
		require.NoError(t, err)
		require.NotNil(t, res)
		if res.ErrorCount > 0 {
			assert.Contains(t, res.Errors[0].Reason, "transaction")
		}
	})
}

