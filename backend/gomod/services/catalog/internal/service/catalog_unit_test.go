package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	catalogsqlc "goshop/services/catalog/internal/db/sqlc"
	"goshop/services/catalog/internal/dto"
	"goshop/services/catalog/internal/repository"
	inventorycontract "goshop/services/inventory/contract"
	"goshop/shared/pkg/cache"
	"goshop/shared/pkg/utils"
)

func assertCatalogErrCode(t *testing.T, err error, want utils.ErrorCode) {
	t.Helper()
	var appErr *utils.AppError
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, want, appErr.Code)
}

type catalogMemCache struct {
	cache.RedisCacheService
	mu   sync.Mutex
	data map[string][]byte
}

func newCatalogMemCache() *catalogMemCache {
	return &catalogMemCache{data: make(map[string][]byte)}
}

func (c *catalogMemCache) Set(key string, value any, _ time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.data[key] = b
	c.mu.Unlock()
	return nil
}

func (c *catalogMemCache) Get(key string, dest any) error {
	c.mu.Lock()
	b, ok := c.data[key]
	c.mu.Unlock()
	if !ok {
		return redis.Nil
	}
	return json.Unmarshal(b, dest)
}

func (c *catalogMemCache) Clear(_ string) error {
	c.mu.Lock()
	c.data = make(map[string][]byte)
	c.mu.Unlock()
	return nil
}

type unitCatalogRepo struct {
	repository.CatalogRepository
	listFAQsFn              func(ctx context.Context, q *string, limit, offset int32) ([]repository.FAQ, error)
	createCategoryFn        func(ctx context.Context, params catalogsqlc.CreateCategoryParams) (catalogsqlc.Category, error)
	getCategoryBySlugFn     func(ctx context.Context, slug string) (catalogsqlc.Category, error)
	getCategoryByUUIDFn     func(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error)
	getCategoryByIDFn       func(ctx context.Context, id int32) (catalogsqlc.Category, error)
	listCategoriesFn        func(ctx context.Context, activeOnly *bool) ([]catalogsqlc.Category, error)
	getCategoriesParentFn   func(ctx context.Context) ([]catalogsqlc.Category, error)
	updateCategoryFn        func(ctx context.Context, params catalogsqlc.UpdateCategoryParams) (catalogsqlc.Category, error)
	softDeleteCategoryFn    func(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error)
	restoreCategoryFn       func(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error)
	trashCategoryFn         func(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error)
	getProductByUUIDFn      func(ctx context.Context, u uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error)
	getProductBySlugFn      func(ctx context.Context, slug string) (catalogsqlc.GetProductBySlugRow, error)
	getProductByIDFn        func(ctx context.Context, id int32) (catalogsqlc.GetProductByIDRow, error)
	getProductsByUUIDsFn    func(ctx context.Context, uuids []uuid.UUID) ([]catalogsqlc.GetProductsByUUIDsRow, error)
	getProductSpecsFn       func(ctx context.Context, u uuid.UUID) (map[string]string, error)
	getProductsSpecsBatchFn func(ctx context.Context, ids []int32) (map[int32]map[string]string, error)
	createVariantFn         func(ctx context.Context, params catalogsqlc.CreateVariantParams) (catalogsqlc.ProductVariant, error)
	listVariantsByProdFn    func(ctx context.Context, productID int32) ([]catalogsqlc.ProductVariant, error)
	listActiveVariantsFn    func(ctx context.Context, productID int32) ([]catalogsqlc.ProductVariant, error)
	getVariantByUUIDFn      func(ctx context.Context, u uuid.UUID) (catalogsqlc.ProductVariant, error)
	updateVariantFn         func(ctx context.Context, params catalogsqlc.UpdateVariantParams) (catalogsqlc.ProductVariant, error)
	deleteVariantFn         func(ctx context.Context, u uuid.UUID) (catalogsqlc.ProductVariant, error)
}

func (r *unitCatalogRepo) ListFAQs(ctx context.Context, q *string, limit, offset int32) ([]repository.FAQ, error) {
	return r.listFAQsFn(ctx, q, limit, offset)
}
func (r *unitCatalogRepo) CreateCategory(ctx context.Context, p catalogsqlc.CreateCategoryParams) (catalogsqlc.Category, error) {
	return r.createCategoryFn(ctx, p)
}
func (r *unitCatalogRepo) GetCategoryBySlug(ctx context.Context, s string) (catalogsqlc.Category, error) {
	if r.getCategoryBySlugFn != nil {
		return r.getCategoryBySlugFn(ctx, s)
	}
	return catalogsqlc.Category{}, pgx.ErrNoRows
}
func (r *unitCatalogRepo) GetCategoryByUUID(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
	if r.getCategoryByUUIDFn != nil {
		return r.getCategoryByUUIDFn(ctx, u)
	}
	return catalogsqlc.Category{}, pgx.ErrNoRows
}
func (r *unitCatalogRepo) GetCategoryByID(ctx context.Context, id int32) (catalogsqlc.Category, error) {
	if r.getCategoryByIDFn != nil {
		return r.getCategoryByIDFn(ctx, id)
	}
	return catalogsqlc.Category{}, pgx.ErrNoRows
}
func (r *unitCatalogRepo) ListCategories(ctx context.Context, activeOnly *bool) ([]catalogsqlc.Category, error) {
	return r.listCategoriesFn(ctx, activeOnly)
}
func (r *unitCatalogRepo) GetCategoriesWithParent(ctx context.Context) ([]catalogsqlc.Category, error) {
	return r.getCategoriesParentFn(ctx)
}
func (r *unitCatalogRepo) UpdateCategory(ctx context.Context, p catalogsqlc.UpdateCategoryParams) (catalogsqlc.Category, error) {
	return r.updateCategoryFn(ctx, p)
}
func (r *unitCatalogRepo) SoftDeleteCategory(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
	return r.softDeleteCategoryFn(ctx, u)
}
func (r *unitCatalogRepo) RestoreCategory(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
	return r.restoreCategoryFn(ctx, u)
}
func (r *unitCatalogRepo) TrashCategory(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
	return r.trashCategoryFn(ctx, u)
}
func (r *unitCatalogRepo) GetProductByUUID(ctx context.Context, u uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
	if r.getProductByUUIDFn != nil {
		return r.getProductByUUIDFn(ctx, u)
	}
	return catalogsqlc.GetProductByUUIDRow{}, pgx.ErrNoRows
}
func (r *unitCatalogRepo) GetProductBySlug(ctx context.Context, slug string) (catalogsqlc.GetProductBySlugRow, error) {
	if r.getProductBySlugFn != nil {
		return r.getProductBySlugFn(ctx, slug)
	}
	return catalogsqlc.GetProductBySlugRow{}, pgx.ErrNoRows
}
func (r *unitCatalogRepo) GetProductByID(ctx context.Context, id int32) (catalogsqlc.GetProductByIDRow, error) {
	if r.getProductByIDFn != nil {
		return r.getProductByIDFn(ctx, id)
	}
	return catalogsqlc.GetProductByIDRow{}, pgx.ErrNoRows
}
func (r *unitCatalogRepo) GetProductsByUUIDs(ctx context.Context, uuids []uuid.UUID) ([]catalogsqlc.GetProductsByUUIDsRow, error) {
	if r.getProductsByUUIDsFn != nil {
		return r.getProductsByUUIDsFn(ctx, uuids)
	}
	return nil, nil
}
func (r *unitCatalogRepo) GetProductSpecifications(ctx context.Context, u uuid.UUID) (map[string]string, error) {
	if r.getProductSpecsFn != nil {
		return r.getProductSpecsFn(ctx, u)
	}
	return nil, nil
}
func (r *unitCatalogRepo) GetProductsSpecificationsBatch(ctx context.Context, ids []int32) (map[int32]map[string]string, error) {
	if r.getProductsSpecsBatchFn != nil {
		return r.getProductsSpecsBatchFn(ctx, ids)
	}
	return map[int32]map[string]string{}, nil
}
func (r *unitCatalogRepo) CreateVariant(ctx context.Context, p catalogsqlc.CreateVariantParams) (catalogsqlc.ProductVariant, error) {
	return r.createVariantFn(ctx, p)
}
func (r *unitCatalogRepo) ListVariantsByProductID(ctx context.Context, id int32) ([]catalogsqlc.ProductVariant, error) {
	return r.listVariantsByProdFn(ctx, id)
}
func (r *unitCatalogRepo) ListActiveVariantsByProductID(ctx context.Context, id int32) ([]catalogsqlc.ProductVariant, error) {
	return r.listActiveVariantsFn(ctx, id)
}
func (r *unitCatalogRepo) GetVariantByUUID(ctx context.Context, u uuid.UUID) (catalogsqlc.ProductVariant, error) {
	if r.getVariantByUUIDFn != nil {
		return r.getVariantByUUIDFn(ctx, u)
	}
	return catalogsqlc.ProductVariant{}, pgx.ErrNoRows
}
func (r *unitCatalogRepo) UpdateVariant(ctx context.Context, p catalogsqlc.UpdateVariantParams) (catalogsqlc.ProductVariant, error) {
	return r.updateVariantFn(ctx, p)
}
func (r *unitCatalogRepo) DeleteVariant(ctx context.Context, u uuid.UUID) (catalogsqlc.ProductVariant, error) {
	if r.deleteVariantFn != nil {
		return r.deleteVariantFn(ctx, u)
	}
	return catalogsqlc.ProductVariant{}, nil
}

type unitInvClient struct {
	inventorycontract.InventoryClient
	stock []inventorycontract.StockItemInfo
}

func (c *unitInvClient) GetStock(context.Context, []string, []int32) ([]inventorycontract.StockItemInfo, error) {
	return c.stock, nil
}

func TestCatalogService_CategoriesCRUDAndTree(t *testing.T) {
	cUUID := uuid.New()
	childUUID := uuid.New()
	parentID := int32(1)
	listCalls := 0

	repo := &unitCatalogRepo{
		createCategoryFn: func(_ context.Context, p catalogsqlc.CreateCategoryParams) (catalogsqlc.Category, error) {
			if p.CategoryName == "Dup" {
				return catalogsqlc.Category{}, &pgconn.PgError{Code: "23505"}
			}
			return catalogsqlc.Category{CategoryID: 1, CategoryUuid: cUUID, CategoryName: p.CategoryName, CategorySlug: p.CategorySlug, CategoryStatus: 1}, nil
		},
		getCategoryByUUIDFn: func(_ context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
			if u == cUUID {
				return catalogsqlc.Category{CategoryID: 1, CategoryUuid: cUUID, CategoryName: "Electronics"}, nil
			}
			return catalogsqlc.Category{}, pgx.ErrNoRows
		},
		listCategoriesFn: func(context.Context, *bool) ([]catalogsqlc.Category, error) {
			listCalls++
			return []catalogsqlc.Category{{CategoryID: 1, CategoryUuid: cUUID, CategoryName: "Electronics"}}, nil
		},
		getCategoriesParentFn: func(context.Context) ([]catalogsqlc.Category, error) {
			return []catalogsqlc.Category{
				{CategoryID: 1, CategoryUuid: cUUID, CategoryName: "Electronics"},
				{CategoryID: 2, CategoryUuid: childUUID, CategoryName: "Phones", ParentID: &parentID},
			}, nil
		},
		updateCategoryFn: func(_ context.Context, p catalogsqlc.UpdateCategoryParams) (catalogsqlc.Category, error) {
			return catalogsqlc.Category{CategoryID: 1, CategoryUuid: p.CategoryUuid, CategoryName: "Electronics Pro"}, nil
		},
		softDeleteCategoryFn: func(_ context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
			return catalogsqlc.Category{CategoryID: 1, CategoryUuid: u}, nil
		},
		restoreCategoryFn: func(_ context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
			return catalogsqlc.Category{CategoryID: 1, CategoryUuid: u}, nil
		},
		trashCategoryFn: func(_ context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
			return catalogsqlc.Category{CategoryID: 1, CategoryUuid: u}, nil
		},
	}

	svc := &catalogService{repo: repo, cache: newCatalogMemCache()}

	cat, err := svc.CreateCategory(context.Background(), dto.CreateCategoryInput{Name: "Electronics"})
	require.NoError(t, err)
	assert.Equal(t, "electronics", cat.Slug)

	_, errDup := svc.CreateCategory(context.Background(), dto.CreateCategoryInput{Name: "Dup"})
	assertCatalogErrCode(t, errDup, utils.ErrCodeConflict)

	_, errNotFound := svc.GetCategoryByUUID(context.Background(), uuid.New())
	assertCatalogErrCode(t, errNotFound, utils.ErrCodeNotFound)

	active := true
	all1, err := svc.GetAllCategories(context.Background(), &active)
	require.NoError(t, err)
	require.Len(t, all1, 1)
	_, err = svc.GetAllCategories(context.Background(), &active)
	require.NoError(t, err)
	assert.Equal(t, 1, listCalls) // second call served from cache

	tree, err := svc.GetCategoryTree(context.Background())
	require.NoError(t, err)
	require.Len(t, tree, 1)
	require.Len(t, tree[0].Children, 1)
	assert.Equal(t, "Phones", tree[0].Children[0].Name)

	updated, err := svc.UpdateCategory(context.Background(), dto.UpdateCategoryInput{UUID: cUUID.String()})
	require.NoError(t, err)
	assert.Equal(t, "Electronics Pro", updated.Name)

	_, err = svc.SoftDeleteCategory(context.Background(), cUUID)
	require.NoError(t, err)
	_, err = svc.RestoreCategory(context.Background(), cUUID)
	require.NoError(t, err)
	require.NoError(t, svc.TrashCategory(context.Background(), cUUID))
}

func TestCatalogService_ProductReadsAndStockEnrichment(t *testing.T) {
	pUUID := uuid.New()
	cUUID := uuid.New()
	img := "https://cdn.example.com/media/products/phone.png"

	repo := &unitCatalogRepo{
		getProductByUUIDFn: func(_ context.Context, u uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
			if u == pUUID {
				return catalogsqlc.GetProductByUUIDRow{
					ProductID:       10,
					ProductUuid:     pUUID,
					ProductName:     "Phone",
					ProductSlug:     "phone",
					CategoryUuid:    cUUID,
					ProductImageUrl: &img,
				}, nil
			}
			return catalogsqlc.GetProductByUUIDRow{}, pgx.ErrNoRows
		},
		getProductBySlugFn: func(_ context.Context, slug string) (catalogsqlc.GetProductBySlugRow, error) {
			return catalogsqlc.GetProductBySlugRow{
				ProductID:       10,
				ProductUuid:     pUUID,
				ProductName:     "Phone",
				ProductSlug:     slug,
				CategoryUuid:    cUUID,
				ProductImageUrl: &img,
			}, nil
		},
		getProductSpecsFn: func(context.Context, uuid.UUID) (map[string]string, error) {
			return map[string]string{"RAM": "8GB"}, nil
		},
	}

	inv := &unitInvClient{
		stock: []inventorycontract.StockItemInfo{{ProductID: 10, ProductUUID: pUUID.String(), Quantity: 42, AvailableQuantity: 42}},
	}
	svc := &catalogService{
		repo:            repo,
		cache:           newCatalogMemCache(),
		inventoryClient: inv,
		mediaBase:       "https://cdn.example.com/media",
	}

	p, err := svc.GetProductByUUID(context.Background(), pUUID)
	require.NoError(t, err)
	assert.Equal(t, int32(42), p.Stock)
	assert.Equal(t, "8GB", p.Specifications["RAM"])

	_, errNF := svc.GetProductByUUID(context.Background(), uuid.New())
	assertCatalogErrCode(t, errNF, utils.ErrCodeNotFound)

	pSlug, err := svc.GetProductBySlug(context.Background(), "phone")
	require.NoError(t, err)
	assert.Equal(t, int32(42), pSlug.Stock)
}

func TestCatalogService_VariantsCRUDAndOwnership(t *testing.T) {
	pUUID := uuid.New()
	vUUID := uuid.New()

	repo := &unitCatalogRepo{
		getProductByUUIDFn: func(_ context.Context, u uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
			return catalogsqlc.GetProductByUUIDRow{ProductID: 10, ProductUuid: u}, nil
		},
		createVariantFn: func(_ context.Context, p catalogsqlc.CreateVariantParams) (catalogsqlc.ProductVariant, error) {
			return catalogsqlc.ProductVariant{VariantID: 1, VariantUuid: vUUID, ProductID: p.ProductID, VariantSku: p.VariantSku, VariantTitle: p.VariantTitle}, nil
		},
		listActiveVariantsFn: func(context.Context, int32) ([]catalogsqlc.ProductVariant, error) {
			return []catalogsqlc.ProductVariant{{VariantID: 1, VariantUuid: vUUID, ProductID: 10, VariantTitle: "Blue"}}, nil
		},
		getVariantByUUIDFn: func(_ context.Context, u uuid.UUID) (catalogsqlc.ProductVariant, error) {
			if u == vUUID {
				return catalogsqlc.ProductVariant{VariantID: 1, VariantUuid: vUUID, ProductID: 999}, nil // belongs to another product
			}
			return catalogsqlc.ProductVariant{}, pgx.ErrNoRows
		},
	}

	svc := &catalogService{repo: repo}

	v, err := svc.CreateVariant(context.Background(), pUUID, dto.CreateVariantInput{SKU: "SKU-1", Title: "Blue", Price: 100})
	require.NoError(t, err)
	assert.Equal(t, "SKU-1", v.SKU)

	list, err := svc.GetVariantsByProduct(context.Background(), pUUID, true)
	require.NoError(t, err)
	require.Len(t, list, 1)

	_, errForbidden := svc.UpdateVariant(context.Background(), pUUID, vUUID, dto.UpdateVariantInput{})
	assertCatalogErrCode(t, errForbidden, utils.ErrCodeForbidden)
}

func TestCatalogService_ListFAQsClampsLimit(t *testing.T) {
	var gotLimit int32
	repo := &unitCatalogRepo{
		listFAQsFn: func(_ context.Context, _ *string, limit, _ int32) ([]repository.FAQ, error) {
			gotLimit = limit
			return []repository.FAQ{{Question: "Q1", Answer: "A1"}}, nil
		},
	}
	svc := &catalogService{repo: repo}
	faqs, err := svc.ListFAQs(context.Background(), nil, 500, -1)
	require.NoError(t, err)
	assert.Equal(t, int32(100), gotLimit)
	require.Len(t, faqs, 1)
}
