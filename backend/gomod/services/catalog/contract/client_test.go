package contract_test

import (
	"context"
	"io"
	"net"
	"testing"

	"goshop/services/catalog/contract"
	"goshop/services/catalog/internal/dto"
	cataloggrpc "goshop/services/catalog/internal/grpc"
	catalogv1 "goshop/shared/proto/catalog/v1"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type mockCatalogService struct {
	getProductByUUIDFn  func(ctx context.Context, u uuid.UUID) (*dto.ProductDTO, error)
	getProductByIDFn    func(ctx context.Context, id int32) (*dto.ProductDTO, error)
	getProductBySlugFn  func(ctx context.Context, slug string) (*dto.ProductDTO, error)
	getProductsBatchFn  func(ctx context.Context, uuids []string, ids []int32) ([]dto.ProductDTO, error)
	validateProductsFn  func(ctx context.Context, uuids []string) (bool, []string, error)
	getCategoryByUUIDFn func(ctx context.Context, u uuid.UUID) (*dto.CategoryDTO, error)
	listCategoriesFn    func(ctx context.Context, activeOnly *bool) ([]dto.CategoryDTO, error)
}

func (m *mockCatalogService) CreateCategory(ctx context.Context, input dto.CreateCategoryInput) (*dto.CategoryDTO, error) {
	return nil, nil
}
func (m *mockCatalogService) GetCategoryByUUID(ctx context.Context, u uuid.UUID) (*dto.CategoryDTO, error) {
	if m.getCategoryByUUIDFn != nil {
		return m.getCategoryByUUIDFn(ctx, u)
	}
	return &dto.CategoryDTO{CategoryID: 1, UUID: u.String(), Name: "Electronics", Slug: "electronics"}, nil
}
func (m *mockCatalogService) GetCategoryBySlug(ctx context.Context, slug string) (*dto.CategoryDTO, error) {
	return &dto.CategoryDTO{CategoryID: 1, UUID: uuid.New().String(), Name: "Electronics", Slug: slug}, nil
}
func (m *mockCatalogService) GetCategoryByID(ctx context.Context, id int32) (*dto.CategoryDTO, error) {
	return &dto.CategoryDTO{CategoryID: id, UUID: uuid.New().String(), Name: "Electronics", Slug: "electronics"}, nil
}
func (m *mockCatalogService) GetAllCategories(ctx context.Context, activeOnly *bool) ([]dto.CategoryDTO, error) {
	if m.listCategoriesFn != nil {
		return m.listCategoriesFn(ctx, activeOnly)
	}
	return []dto.CategoryDTO{{CategoryID: 1, UUID: uuid.New().String(), Name: "Electronics", Slug: "electronics"}}, nil
}
func (m *mockCatalogService) ListFAQs(ctx context.Context, q *string, limit, offset int32) ([]dto.FAQDTO, error) {
	return nil, nil
}

func (m *mockCatalogService) UpdateCategory(ctx context.Context, input dto.UpdateCategoryInput) (*dto.CategoryDTO, error) {
	return nil, nil
}
func (m *mockCatalogService) SoftDeleteCategory(ctx context.Context, u uuid.UUID) (*dto.CategoryDTO, error) {
	return nil, nil
}
func (m *mockCatalogService) RestoreCategory(ctx context.Context, u uuid.UUID) (*dto.CategoryDTO, error) {
	return nil, nil
}
func (m *mockCatalogService) TrashCategory(ctx context.Context, u uuid.UUID) error {
	return nil
}

func (m *mockCatalogService) CreateProduct(ctx context.Context, input dto.CreateProductInput) (*dto.ProductDTO, error) {
	return nil, nil
}
func (m *mockCatalogService) GetProductByUUID(ctx context.Context, u uuid.UUID) (*dto.ProductDTO, error) {
	if m.getProductByUUIDFn != nil {
		return m.getProductByUUIDFn(ctx, u)
	}
	return &dto.ProductDTO{ID: 10, UUID: u.String(), Name: "Keyboard", Slug: "keyboard", Price: 99.99, Stock: 25}, nil
}
func (m *mockCatalogService) GetProductBySlug(ctx context.Context, slug string) (*dto.ProductDTO, error) {
	if m.getProductBySlugFn != nil {
		return m.getProductBySlugFn(ctx, slug)
	}
	return &dto.ProductDTO{ID: 10, UUID: uuid.New().String(), Name: "Keyboard", Slug: slug, Price: 99.99, Stock: 25}, nil
}
func (m *mockCatalogService) GetProductByID(ctx context.Context, id int32) (*dto.ProductDTO, error) {
	if m.getProductByIDFn != nil {
		return m.getProductByIDFn(ctx, id)
	}
	return &dto.ProductDTO{ID: id, UUID: uuid.New().String(), Name: "Keyboard", Slug: "keyboard", Price: 99.99, Stock: 25}, nil
}
func (m *mockCatalogService) GetProductsBatch(ctx context.Context, uuids []string, ids []int32) ([]dto.ProductDTO, error) {
	if m.getProductsBatchFn != nil {
		return m.getProductsBatchFn(ctx, uuids, ids)
	}
	var res []dto.ProductDTO
	for _, u := range uuids {
		res = append(res, dto.ProductDTO{ID: 1, UUID: u, Name: "Item", Price: 50.0, Stock: 10})
	}
	return res, nil
}
func (m *mockCatalogService) ValidateProducts(ctx context.Context, uuids []string) (bool, []string, error) {
	if m.validateProductsFn != nil {
		return m.validateProductsFn(ctx, uuids)
	}
	return true, nil, nil
}
func (m *mockCatalogService) GetAllProducts(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, orderBy, sort string, page, limit int32, deleted bool) ([]dto.ProductDTO, int32, error) {
	return nil, 0, nil
}
func (m *mockCatalogService) SearchProducts(ctx context.Context, q, categoryUUID string, minPrice, maxPrice, minRating float64, sort string, page, limit int32) ([]dto.ProductDTO, int64, error) {
	return nil, 0, nil
}
func (m *mockCatalogService) SuggestProducts(ctx context.Context, q string, limit int32) ([]dto.ProductSuggestionDTO, error) {
	return nil, nil
}
func (m *mockCatalogService) SuggestProductsFiltered(ctx context.Context, q, filter string, limit int32) ([]dto.ProductSuggestionDTO, error) {
	return nil, nil
}
func (m *mockCatalogService) UpdateProduct(ctx context.Context, input dto.UpdateProductInput) (*dto.ProductDTO, error) {
	return nil, nil
}
func (m *mockCatalogService) SoftDeleteProduct(ctx context.Context, u uuid.UUID) (*dto.ProductDTO, error) {
	return nil, nil
}
func (m *mockCatalogService) RestoreProduct(ctx context.Context, u uuid.UUID) (*dto.ProductDTO, error) {
	return nil, nil
}
func (m *mockCatalogService) TrashProduct(ctx context.Context, u uuid.UUID) error {
	return nil
}
func (m *mockCatalogService) ImportProductsCSV(ctx context.Context, file io.Reader) (*dto.ProductCSVImportResponse, error) {
	return nil, nil
}
func (m *mockCatalogService) StartOutboxWorker(ctx context.Context) {}
func (m *mockCatalogService) ProcessOutboxBatch(ctx context.Context) (int, error) {
	return 0, nil
}

func setupTestGRPCServer(t *testing.T, mock *mockCatalogService) (*contract.GRPCCatalogClient, func()) {
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	catalogv1.RegisterCatalogServiceServer(s, cataloggrpc.NewCatalogGRPCServer(mock))

	go func() {
		_ = s.Serve(lis)
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := contract.NewGRPCCatalogClientWithConn(conn)

	cleanup := func() {
		_ = client.Close()
		s.Stop()
		_ = lis.Close()
	}

	return client, cleanup
}

func TestCatalogContract_GetProductByUUID(t *testing.T) {
	pUUID := uuid.New().String()
	mock := &mockCatalogService{}
	client, cleanup := setupTestGRPCServer(t, mock)
	defer cleanup()

	prod, err := client.GetProductByUUID(context.Background(), pUUID)
	require.NoError(t, err)
	assert.NotNil(t, prod)
	assert.Equal(t, pUUID, prod.ProductUUID)
	assert.Equal(t, "Keyboard", prod.ProductName)
	assert.Equal(t, 99.99, prod.ProductPrice)
	assert.Equal(t, int32(25), prod.ProductStock)
}

func TestCatalogContract_GetProductsBatch(t *testing.T) {
	u1 := uuid.New().String()
	u2 := uuid.New().String()
	mock := &mockCatalogService{}
	client, cleanup := setupTestGRPCServer(t, mock)
	defer cleanup()

	prods, err := client.GetProductsBatch(context.Background(), []string{u1, u2}, nil)
	require.NoError(t, err)
	assert.Len(t, prods, 2)
	assert.Equal(t, u1, prods[0].ProductUUID)
	assert.Equal(t, u2, prods[1].ProductUUID)
}

func TestCatalogContract_ValidateProducts(t *testing.T) {
	u1 := uuid.New().String()
	mock := &mockCatalogService{}
	client, cleanup := setupTestGRPCServer(t, mock)
	defer cleanup()

	valid, missing, err := client.ValidateProducts(context.Background(), []string{u1})
	require.NoError(t, err)
	assert.True(t, valid)
	assert.Empty(t, missing)
}

func TestCatalogContract_ListCategories(t *testing.T) {
	mock := &mockCatalogService{}
	client, cleanup := setupTestGRPCServer(t, mock)
	defer cleanup()

	cats, err := client.ListCategories(context.Background(), true)
	require.NoError(t, err)
	assert.Len(t, cats, 1)
	assert.Equal(t, "Electronics", cats[0].CategoryName)
}
