package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	catalogsqlc "goshop/services/catalog/internal/db/sqlc"
	"goshop/services/catalog/internal/dto"
	"goshop/services/catalog/internal/repository"
	inventorycontract "goshop/services/inventory/contract"
	"goshop/shared/pkg/utils"
)

type qaCatalogRepo struct {
	unitCatalogRepo
	getProductsByIDsFn   func(ctx context.Context, ids []int32) ([]catalogsqlc.GetProductsByIDsRow, error)
	getAllProductsFn     func(ctx context.Context, search, catUUID string, minP, maxP, minR float64, status int32, orderBy, sort string, limit, offset int32, deleted bool, specs map[string][]string) ([]catalogsqlc.GetProductByUUIDRow, error)
	countAllProductsFn   func(ctx context.Context, search, catUUID string, minP, maxP, minR float64, status int32, deleted bool, specs map[string][]string) (int64, error)
	updateProductFn      func(ctx context.Context, p catalogsqlc.UpdateProductParams) (catalogsqlc.UpdateProductRow, error)
	updateProductSpecsFn func(ctx context.Context, u uuid.UUID, specs map[string]string) error
	softDeleteProductFn  func(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error)
	restoreProductFn     func(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error)
	trashProductFn       func(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error)
	createProductFn      func(ctx context.Context, p catalogsqlc.CreateProductParams) (catalogsqlc.CreateProductRow, error)
	insertOutboxFn       func(ctx context.Context, aggType, aggID, evtType string, payload any) (catalogsqlc.OutboxEvent, error)
	outboxEvents         []string
}

func (r *qaCatalogRepo) GetProductsByIDs(ctx context.Context, ids []int32) ([]catalogsqlc.GetProductsByIDsRow, error) {
	if r.getProductsByIDsFn != nil {
		return r.getProductsByIDsFn(ctx, ids)
	}
	return nil, nil
}
func (r *qaCatalogRepo) GetAllProducts(ctx context.Context, search, catUUID string, minP, maxP, minR float64, status int32, orderBy, sort string, limit, offset int32, deleted bool, specs map[string][]string) ([]catalogsqlc.GetProductByUUIDRow, error) {
	return r.getAllProductsFn(ctx, search, catUUID, minP, maxP, minR, status, orderBy, sort, limit, offset, deleted, specs)
}
func (r *qaCatalogRepo) CountAllProducts(ctx context.Context, search, catUUID string, minP, maxP, minR float64, status int32, deleted bool, specs map[string][]string) (int64, error) {
	return r.countAllProductsFn(ctx, search, catUUID, minP, maxP, minR, status, deleted, specs)
}
func (r *qaCatalogRepo) UpdateProduct(ctx context.Context, p catalogsqlc.UpdateProductParams) (catalogsqlc.UpdateProductRow, error) {
	return r.updateProductFn(ctx, p)
}
func (r *qaCatalogRepo) UpdateProductSpecifications(ctx context.Context, u uuid.UUID, specs map[string]string) error {
	if r.updateProductSpecsFn != nil {
		return r.updateProductSpecsFn(ctx, u, specs)
	}
	return nil
}
func (r *qaCatalogRepo) SoftDeleteProduct(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error) {
	return r.softDeleteProductFn(ctx, u)
}
func (r *qaCatalogRepo) RestoreProduct(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error) {
	return r.restoreProductFn(ctx, u)
}
func (r *qaCatalogRepo) TrashProduct(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error) {
	return r.trashProductFn(ctx, u)
}
func (r *qaCatalogRepo) CreateProduct(ctx context.Context, p catalogsqlc.CreateProductParams) (catalogsqlc.CreateProductRow, error) {
	if r.createProductFn != nil {
		return r.createProductFn(ctx, p)
	}
	return catalogsqlc.CreateProductRow{ProductID: 1, ProductUuid: uuid.New(), ProductName: p.ProductName, ProductSlug: p.ProductSlug}, nil
}
func (r *qaCatalogRepo) InsertOutboxEvent(ctx context.Context, aggType, aggID, evtType string, payload any) (catalogsqlc.OutboxEvent, error) {
	r.outboxEvents = append(r.outboxEvents, evtType)
	if r.insertOutboxFn != nil {
		return r.insertOutboxFn(ctx, aggType, aggID, evtType, payload)
	}
	return catalogsqlc.OutboxEvent{EventID: int64(len(r.outboxEvents))}, nil
}

type qaInvClient struct {
	inventorycontract.InventoryClient
	stock    []inventorycontract.StockItemInfo
	adjustFn func(ctx context.Context, in inventorycontract.AdjustStockInput) (*inventorycontract.AdjustStockResult, error)
}

func (c *qaInvClient) GetStock(context.Context, []string, []int32) ([]inventorycontract.StockItemInfo, error) {
	return c.stock, nil
}
func (c *qaInvClient) AdjustStock(ctx context.Context, in inventorycontract.AdjustStockInput) (*inventorycontract.AdjustStockResult, error) {
	if c.adjustFn != nil {
		return c.adjustFn(ctx, in)
	}
	return &inventorycontract.AdjustStockResult{Success: true, CurrentStock: in.QuantityChange}, nil
}

func TestCatalogService_ValidateProducts_EdgeCases(t *testing.T) {
	activeUUID := uuid.New()
	inactiveUUID := uuid.New()
	missingUUID := uuid.New()

	repo := &qaCatalogRepo{
		unitCatalogRepo: unitCatalogRepo{
			getProductsByUUIDsFn: func(context.Context, []uuid.UUID) ([]catalogsqlc.GetProductsByUUIDsRow, error) {
				return []catalogsqlc.GetProductsByUUIDsRow{
					{ProductUuid: activeUUID, ProductStatus: 1},
					{ProductUuid: inactiveUUID, ProductStatus: 2},
				}, nil
			},
		},
	}
	svc := &catalogService{repo: repo}

	t.Run("empty list returns valid", func(t *testing.T) {
		ok, missing, err := svc.ValidateProducts(context.Background(), nil)
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Empty(t, missing)
	})

	t.Run("malformed UUID fails immediately", func(t *testing.T) {
		ok, bad, err := svc.ValidateProducts(context.Background(), []string{activeUUID.String(), "bad-uuid"})
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, []string{"bad-uuid"}, bad)
	})

	t.Run("inactive and missing products are reported", func(t *testing.T) {
		ok, missing, err := svc.ValidateProducts(context.Background(), []string{activeUUID.String(), inactiveUUID.String(), missingUUID.String()})
		require.NoError(t, err)
		assert.False(t, ok)
		assert.ElementsMatch(t, []string{inactiveUUID.String(), missingUUID.String()}, missing)
	})
}

func TestCatalogService_GetProductByIDAndBatch(t *testing.T) {
	p1 := uuid.New()
	p2 := uuid.New()
	cUUID := uuid.New()

	repo := &qaCatalogRepo{
		unitCatalogRepo: unitCatalogRepo{
			getProductByIDFn: func(_ context.Context, id int32) (catalogsqlc.GetProductByIDRow, error) {
				if id == 1 {
					return catalogsqlc.GetProductByIDRow{ProductID: 1, ProductUuid: p1, ProductName: "One", CategoryUuid: cUUID}, nil
				}
				return catalogsqlc.GetProductByIDRow{}, pgx.ErrNoRows
			},
			getProductsByUUIDsFn: func(context.Context, []uuid.UUID) ([]catalogsqlc.GetProductsByUUIDsRow, error) {
				return []catalogsqlc.GetProductsByUUIDsRow{{ProductID: 1, ProductUuid: p1, ProductName: "One", CategoryUuid: cUUID}}, nil
			},
		},
		getProductsByIDsFn: func(context.Context, []int32) ([]catalogsqlc.GetProductsByIDsRow, error) {
			return []catalogsqlc.GetProductsByIDsRow{{ProductID: 2, ProductUuid: p2, ProductName: "Two", CategoryUuid: cUUID}}, nil
		},
	}
	inv := &qaInvClient{
		stock: []inventorycontract.StockItemInfo{
			{ProductUUID: p1.String(), Quantity: 5},
			{ProductUUID: p2.String(), Quantity: 9},
		},
	}
	svc := &catalogService{repo: repo, inventoryClient: inv}

	got, err := svc.GetProductByID(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, int32(5), got.Stock)

	_, errNF := svc.GetProductByID(context.Background(), 999)
	assertCatalogErrCode(t, errNF, utils.ErrCodeNotFound)

	batch, err := svc.GetProductsBatch(context.Background(), []string{p1.String(), "invalid"}, []int32{2})
	require.NoError(t, err)
	require.Len(t, batch, 2)
	assert.Equal(t, int32(5), batch[0].Stock)
	assert.Equal(t, int32(9), batch[1].Stock)
}

func TestCatalogService_GetAllProducts_CachesAndEnrichesSpecsAndStock(t *testing.T) {
	p1 := uuid.New()
	cUUID := uuid.New()
	repoCalls := 0

	repo := &qaCatalogRepo{
		unitCatalogRepo: unitCatalogRepo{
			getProductsSpecsBatchFn: func(context.Context, []int32) (map[int32]map[string]string, error) {
				return map[int32]map[string]string{1: {"Color": "Black"}}, nil
			},
		},
		getAllProductsFn: func(context.Context, string, string, float64, float64, float64, int32, string, string, int32, int32, bool, map[string][]string) ([]catalogsqlc.GetProductByUUIDRow, error) {
			repoCalls++
			return []catalogsqlc.GetProductByUUIDRow{
				{ProductID: 1, ProductUuid: p1, ProductName: "Laptop", CategoryUuid: cUUID},
			}, nil
		},
		countAllProductsFn: func(context.Context, string, string, float64, float64, float64, int32, bool, map[string][]string) (int64, error) {
			return 1, nil
		},
	}
	inv := &qaInvClient{stock: []inventorycontract.StockItemInfo{{ProductUUID: p1.String(), Quantity: 7}}}
	svc := &catalogService{repo: repo, cache: newCatalogMemCache(), inventoryClient: inv}

	specs := map[string][]string{"Color": {"Black", "White"}}
	list1, total1, err := svc.GetAllProducts(context.Background(), "", "", 0, 0, 0, 1, "", "", 0, 0, false, specs)
	require.NoError(t, err)
	assert.Equal(t, int32(1), total1)
	require.Len(t, list1, 1)
	assert.Equal(t, "Black", list1[0].Specifications["Color"])
	assert.Equal(t, int32(7), list1[0].Stock)

	// Second call hits cache, does not re-query repo
	list2, total2, err := svc.GetAllProducts(context.Background(), "", "", 0, 0, 0, 1, "", "", 1, 10, false, specs)
	require.NoError(t, err)
	assert.Equal(t, int32(1), total2)
	assert.Len(t, list2, 1)
	assert.Equal(t, 1, repoCalls)
}

func TestCatalogService_UpdateDeleteRestoreTrashProduct(t *testing.T) {
	pUUID := uuid.New()
	cUUID := uuid.New()

	repo := &qaCatalogRepo{
		unitCatalogRepo: unitCatalogRepo{
			getCategoryByUUIDFn: func(context.Context, uuid.UUID) (catalogsqlc.Category, error) {
				return catalogsqlc.Category{CategoryID: 3, CategoryUuid: cUUID}, nil
			},
			getProductByUUIDFn: func(context.Context, uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
				return catalogsqlc.GetProductByUUIDRow{ProductID: 10, ProductUuid: pUUID, ProductName: "Updated", CategoryUuid: cUUID}, nil
			},
		},
		updateProductFn: func(_ context.Context, p catalogsqlc.UpdateProductParams) (catalogsqlc.UpdateProductRow, error) {
			return catalogsqlc.UpdateProductRow{ProductID: 10, ProductUuid: p.ProductUuid, ProductName: "Updated", CategoryUuid: cUUID}, nil
		},
		softDeleteProductFn: func(_ context.Context, u uuid.UUID) (catalogsqlc.Product, error) {
			return catalogsqlc.Product{ProductID: 10, ProductUuid: u, ProductName: "Updated"}, nil
		},
		restoreProductFn: func(_ context.Context, u uuid.UUID) (catalogsqlc.Product, error) {
			return catalogsqlc.Product{ProductID: 10, ProductUuid: u, ProductName: "Updated"}, nil
		},
		trashProductFn: func(_ context.Context, u uuid.UUID) (catalogsqlc.Product, error) {
			return catalogsqlc.Product{ProductID: 10, ProductUuid: u}, nil
		},
	}

	t.Run("UpdateProduct rejects invalid UUID and propagates inventory failure", func(t *testing.T) {
		svc := &catalogService{repo: repo}
		_, errBad := svc.UpdateProduct(context.Background(), dto.UpdateProductInput{UUID: "not-a-uuid"})
		assertCatalogErrCode(t, errBad, utils.ErrCodeBadRequest)

		stock := int32(25)
		svcFailInv := &catalogService{
			repo: repo,
			inventoryClient: &qaInvClient{
				adjustFn: func(context.Context, inventorycontract.AdjustStockInput) (*inventorycontract.AdjustStockResult, error) {
					return nil, errors.New("inv down")
				},
			},
		}
		_, errInv := svcFailInv.UpdateProduct(context.Background(), dto.UpdateProductInput{
			UUID:  pUUID.String(),
			Stock: &stock,
		})
		assertCatalogErrCode(t, errInv, utils.ErrCodeServiceUnavailable)
	})

	t.Run("UpdateProduct, SoftDelete, Restore, Trash emit outbox events", func(t *testing.T) {
		stock := int32(15)
		name := "Updated"
		sku := " sku-10 "
		catStr := cUUID.String()
		svc := &catalogService{
			repo:            repo,
			cache:           newCatalogMemCache(),
			inventoryClient: &qaInvClient{},
		}

		res, err := svc.UpdateProduct(context.Background(), dto.UpdateProductInput{
			UUID:           pUUID.String(),
			Name:           &name,
			SKU:            &sku,
			CategoryUUID:   &catStr,
			Stock:          &stock,
			Specifications: map[string]string{"Weight": "1kg"},
		})
		require.NoError(t, err)
		assert.Equal(t, int32(15), res.Stock)
		assert.Equal(t, "1kg", res.Specifications["Weight"])

		sd, err := svc.SoftDeleteProduct(context.Background(), pUUID)
		require.NoError(t, err)
		assert.Equal(t, "Inactive", sd.Status)

		rs, err := svc.RestoreProduct(context.Background(), pUUID)
		require.NoError(t, err)
		assert.Equal(t, "Active", rs.Status)

		require.NoError(t, svc.TrashProduct(context.Background(), pUUID))
		assert.Contains(t, repo.outboxEvents, "PRODUCT_UPSERT")
		assert.Contains(t, repo.outboxEvents, "PRODUCT_DELETE")
	})
}

func TestCatalogService_DeleteVariant_OwnershipAndSuccess(t *testing.T) {
	pUUID := uuid.New()
	vUUID := uuid.New()

	repo := &unitCatalogRepo{
		getProductByUUIDFn: func(context.Context, uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
			return catalogsqlc.GetProductByUUIDRow{ProductID: 10, ProductUuid: pUUID}, nil
		},
		getVariantByUUIDFn: func(_ context.Context, u uuid.UUID) (catalogsqlc.ProductVariant, error) {
			if u == vUUID {
				return catalogsqlc.ProductVariant{VariantID: 1, VariantUuid: vUUID, ProductID: 10}, nil
			}
			return catalogsqlc.ProductVariant{VariantID: 2, VariantUuid: u, ProductID: 999}, nil
		},
	}
	svc := &catalogService{repo: repo}

	require.NoError(t, svc.DeleteVariant(context.Background(), pUUID, vUUID))
	errWrongOwner := svc.DeleteVariant(context.Background(), pUUID, uuid.New())
	assertCatalogErrCode(t, errWrongOwner, utils.ErrCodeForbidden)
}

func TestParseCSVAttributes_AllFormats(t *testing.T) {
	assert.Nil(t, parseCSVAttributes("  "))
	assert.Equal(t, map[string]string{"RAM": "16GB"}, parseCSVAttributes(`{"RAM":"16GB"}`))
	assert.Equal(t, map[string]string{"Color": "Red", "Size": "L"}, parseCSVAttributes("Color: Red; Size=L"))
	assert.Equal(t, map[string]string{"Material": "Cotton", "Fit": "Slim"}, parseCSVAttributes("Material:Cotton, Fit:Slim"))
	assert.Nil(t, parseCSVAttributes("invalid-pair"))
}

func TestImportProductsCSV_ValidAndInvalidRows(t *testing.T) {
	cUUID := uuid.New()
	repo := &qaCatalogRepo{
		unitCatalogRepo: unitCatalogRepo{
			listCategoriesFn: func(context.Context, *bool) ([]catalogsqlc.Category, error) {
				return []catalogsqlc.Category{{CategoryID: 1, CategoryUuid: cUUID, CategoryName: "Electronics", CategorySlug: "electronics"}}, nil
			},
			getCategoryByUUIDFn: func(context.Context, uuid.UUID) (catalogsqlc.Category, error) {
				return catalogsqlc.Category{CategoryID: 1, CategoryUuid: cUUID, CategoryName: "Electronics"}, nil
			},
			getProductByUUIDFn: func(_ context.Context, u uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
				return catalogsqlc.GetProductByUUIDRow{ProductID: 1, ProductUuid: u, ProductName: "Gaming Mouse"}, nil
			},
			createVariantFn: func(_ context.Context, p catalogsqlc.CreateVariantParams) (catalogsqlc.ProductVariant, error) {
				return catalogsqlc.ProductVariant{VariantID: 1, VariantUuid: uuid.New(), ProductID: p.ProductID, VariantSku: p.VariantSku, VariantTitle: p.VariantTitle}, nil
			},
		},
	}
	svc := &catalogService{repo: repo, inventoryClient: &qaInvClient{}}

	t.Run("rejects empty CSV and missing required columns", func(t *testing.T) {
		_, errEmpty := svc.ImportProductsCSV(context.Background(), strings.NewReader(""))
		assertCatalogErrCode(t, errEmpty, utils.ErrCodeBadRequest)

		_, errMissingCol := svc.ImportProductsCSV(context.Background(), strings.NewReader("name,sku\nPhone,SKU1\n"))
		assertCatalogErrCode(t, errMissingCol, utils.ErrCodeBadRequest)
	})

	t.Run("imports valid rows and collects row-level errors", func(t *testing.T) {
		csvData := strings.Join([]string{
			"name,price,category_id,sku,stock,variant_title,variant_sku,variant_attributes",
			"Gaming Mouse,49.99,1,GM-01,10,,,",
			",49.99,1,,,Black,GM-01-BLK,Color:Black",
			",20.00,1,BAD-1,5,,,",          // missing name -> row error
			"Bad Price,-5,1,BAD-2,5,,,",    // negative price -> row error
			"Bad Cat,10.00,abc,BAD-3,5,,,", // non-numeric category_id -> row error
		}, "\n")

		res, err := svc.ImportProductsCSV(context.Background(), strings.NewReader(csvData))
		require.NoError(t, err)
		assert.Equal(t, 1, res.SuccessCount)
		assert.Equal(t, 3, res.ErrorCount)
		require.Len(t, res.Errors, 3)
	})
}

var _ repository.CatalogRepository = (*qaCatalogRepo)(nil)
