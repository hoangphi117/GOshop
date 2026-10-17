package service

import (
	"context"
	"testing"

	"goshop/services/cart/internal/db/sqlc"
	"goshop/services/cart/internal/dto"
	catalogcontract "goshop/services/catalog/contract"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockCartRepository struct {
	mock.Mock
}

func (m *mockCartRepository) GetOrCreateCartByUserUUID(ctx context.Context, userUUID uuid.UUID, userID *int32) (sqlc.Cart, error) {
	args := m.Called(ctx, userUUID, userID)
	return args.Get(0).(sqlc.Cart), args.Error(1)
}

func (m *mockCartRepository) GetCartByUserUUID(ctx context.Context, userUUID uuid.UUID) (sqlc.Cart, error) {
	args := m.Called(ctx, userUUID)
	return args.Get(0).(sqlc.Cart), args.Error(1)
}

func (m *mockCartRepository) GetCartByUserID(ctx context.Context, userID int32) (sqlc.Cart, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(sqlc.Cart), args.Error(1)
}

func (m *mockCartRepository) GetCartByCartID(ctx context.Context, cartID int32) (sqlc.Cart, error) {
	args := m.Called(ctx, cartID)
	return args.Get(0).(sqlc.Cart), args.Error(1)
}

func (m *mockCartRepository) GetCartByCartUUID(ctx context.Context, cartUUID uuid.UUID) (sqlc.Cart, error) {
	args := m.Called(ctx, cartUUID)
	return args.Get(0).(sqlc.Cart), args.Error(1)
}

func (m *mockCartRepository) GetCartItems(ctx context.Context, cartID int32) ([]sqlc.CartItem, error) {
	args := m.Called(ctx, cartID)
	return args.Get(0).([]sqlc.CartItem), args.Error(1)
}

func (m *mockCartRepository) GetCartItemByUUID(ctx context.Context, itemUUID uuid.UUID, cartID int32) (sqlc.CartItem, error) {
	args := m.Called(ctx, itemUUID, cartID)
	return args.Get(0).(sqlc.CartItem), args.Error(1)
}

func (m *mockCartRepository) GetCartItemQuantity(ctx context.Context, cartID int32, productUUID uuid.UUID) (int32, error) {
	args := m.Called(ctx, cartID, productUUID)
	return args.Get(0).(int32), args.Error(1)
}

func (m *mockCartRepository) AddOrUpdateCartItem(ctx context.Context, arg sqlc.AddOrUpdateCartItemParams) (sqlc.CartItem, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.CartItem), args.Error(1)
}

func (m *mockCartRepository) SetCartItemQuantity(ctx context.Context, arg sqlc.SetCartItemQuantityParams) (sqlc.CartItem, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(sqlc.CartItem), args.Error(1)
}

func (m *mockCartRepository) DeleteCartItemByUUID(ctx context.Context, itemUUID uuid.UUID, cartID int32) (sqlc.CartItem, error) {
	args := m.Called(ctx, itemUUID, cartID)
	return args.Get(0).(sqlc.CartItem), args.Error(1)
}

func (m *mockCartRepository) DeleteCartItemsByProductUUIDs(ctx context.Context, cartID int32, productUUIDs []uuid.UUID) error {
	args := m.Called(ctx, cartID, productUUIDs)
	return args.Error(0)
}

func (m *mockCartRepository) DeleteCartItemsByProductIDs(ctx context.Context, cartID int32, productIDs []int32) error {
	args := m.Called(ctx, cartID, productIDs)
	return args.Error(0)
}

func (m *mockCartRepository) ClearCartByCartID(ctx context.Context, cartID int32) error {
	args := m.Called(ctx, cartID)
	return args.Error(0)
}

func (m *mockCartRepository) CountCartItems(ctx context.Context, cartID int32) (int64, error) {
	args := m.Called(ctx, cartID)
	return args.Get(0).(int64), args.Error(1)
}

type mockCatalogClient struct {
	mock.Mock
}

func (m *mockCatalogClient) GetProductByUUID(ctx context.Context, u string) (*catalogcontract.ProductItemInfo, error) {
	args := m.Called(ctx, u)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*catalogcontract.ProductItemInfo), args.Error(1)
}

func (m *mockCatalogClient) GetProductByID(ctx context.Context, id int32) (*catalogcontract.ProductItemInfo, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*catalogcontract.ProductItemInfo), args.Error(1)
}

func (m *mockCatalogClient) GetProductBySlug(ctx context.Context, slug string) (*catalogcontract.ProductItemInfo, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*catalogcontract.ProductItemInfo), args.Error(1)
}

func (m *mockCatalogClient) GetProductsBatch(ctx context.Context, uuids []string, ids []int32) ([]catalogcontract.ProductItemInfo, error) {
	args := m.Called(ctx, uuids, ids)
	return args.Get(0).([]catalogcontract.ProductItemInfo), args.Error(1)
}

func (m *mockCatalogClient) ValidateProducts(ctx context.Context, uuids []string) (bool, []string, error) {
	args := m.Called(ctx, uuids)
	return args.Bool(0), args.Get(1).([]string), args.Error(2)
}

func (m *mockCatalogClient) GetCategoryByUUID(ctx context.Context, u string) (*catalogcontract.CategoryItemInfo, error) {
	args := m.Called(ctx, u)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*catalogcontract.CategoryItemInfo), args.Error(1)
}

func (m *mockCatalogClient) ListCategories(ctx context.Context, activeOnly bool) ([]catalogcontract.CategoryItemInfo, error) {
	args := m.Called(ctx, activeOnly)
	return args.Get(0).([]catalogcontract.CategoryItemInfo), args.Error(1)
}

func (m *mockCatalogClient) Close() error {
	return nil
}

func TestCartService_GetMyCart(t *testing.T) {
	userUUID := uuid.New()
	cartUUID := uuid.New()
	productUUID := uuid.New()
	itemUUID := uuid.New()

	mockRepo := new(mockCartRepository)
	mockCatalog := new(mockCatalogClient)

	mockRepo.On("GetOrCreateCartByUserUUID", mock.Anything, userUUID, (*int32)(nil)).Return(sqlc.Cart{
		CartID:   1,
		CartUuid: cartUUID,
		UserUuid: userUUID,
	}, nil)

	mockRepo.On("GetCartItems", mock.Anything, int32(1)).Return([]sqlc.CartItem{
		{
			CartItemID:   10,
			CartItemUuid: itemUUID,
			CartID:       1,
			ProductUuid:  productUUID,
			Quantity:     2,
		},
	}, nil)

	mockCatalog.On("GetProductsBatch", mock.Anything, []string{productUUID.String()}, []int32(nil)).Return([]catalogcontract.ProductItemInfo{
		{
			ProductID:     101,
			ProductUUID:   productUUID.String(),
			ProductName:   "Test Phone",
			ProductSlug:   "test-phone",
			ProductPrice:  99.99,
			ProductStock:  10,
			ProductStatus: 1,
		},
	}, nil)

	svc := NewCartService(mockRepo, mockCatalog, nil)
	cartDTO, err := svc.GetMyCart(context.Background(), userUUID, nil)

	assert.NoError(t, err)
	assert.Equal(t, cartUUID.String(), cartDTO.UUID)
	assert.Equal(t, int32(1), cartDTO.TotalItems)
	assert.Equal(t, 199.98, cartDTO.TotalAmount)
	assert.Len(t, cartDTO.Items, 1)
	assert.Equal(t, "Test Phone", cartDTO.Items[0].ProductName)
	assert.True(t, cartDTO.Items[0].IsAvailable)
}

func TestCartService_AddItemToCart(t *testing.T) {
	userUUID := uuid.New()
	cartUUID := uuid.New()
	productUUID := uuid.New()
	itemUUID := uuid.New()
	prodID := int32(101)

	mockRepo := new(mockCartRepository)
	mockCatalog := new(mockCatalogClient)

	mockRepo.On("GetOrCreateCartByUserUUID", mock.Anything, userUUID, (*int32)(nil)).Return(sqlc.Cart{
		CartID:   1,
		CartUuid: cartUUID,
		UserUuid: userUUID,
	}, nil)

	mockCatalog.On("GetProductByUUID", mock.Anything, productUUID.String()).Return(&catalogcontract.ProductItemInfo{
		ProductID:     prodID,
		ProductUUID:   productUUID.String(),
		ProductName:   "Test Phone",
		ProductPrice:  99.99,
		ProductStock:  10,
		ProductStatus: 1,
	}, nil)

	mockRepo.On("GetCartItemQuantity", mock.Anything, int32(1), productUUID).Return(int32(0), nil)

	mockRepo.On("AddOrUpdateCartItem", mock.Anything, sqlc.AddOrUpdateCartItemParams{
		CartID:      1,
		ProductUuid: productUUID,
		ProductID:   &prodID,
		Quantity:    2,
	}).Return(sqlc.CartItem{
		CartItemID:   10,
		CartItemUuid: itemUUID,
		CartID:       1,
		ProductUuid:  productUUID,
		ProductID:   &prodID,
		Quantity:    2,
	}, nil)

	mockRepo.On("GetCartItems", mock.Anything, int32(1)).Return([]sqlc.CartItem{
		{
			CartItemID:   10,
			CartItemUuid: itemUUID,
			CartID:       1,
			ProductUuid:  productUUID,
			ProductID:   &prodID,
			Quantity:     2,
		},
	}, nil)

	mockCatalog.On("GetProductsBatch", mock.Anything, []string{productUUID.String()}, []int32(nil)).Return([]catalogcontract.ProductItemInfo{
		{
			ProductID:     prodID,
			ProductUUID:   productUUID.String(),
			ProductName:   "Test Phone",
			ProductPrice:  99.99,
			ProductStock:  10,
			ProductStatus: 1,
		},
	}, nil)

	svc := NewCartService(mockRepo, mockCatalog, nil)
	cartDTO, err := svc.AddItemToCart(context.Background(), userUUID, nil, dto.AddCartItemInput{
		ProductUUID: productUUID.String(),
		Quantity:    2,
	})

	assert.NoError(t, err)
	assert.Equal(t, int32(1), cartDTO.TotalItems)
	assert.Equal(t, 199.98, cartDTO.TotalAmount)
}

func TestCartService_RemovePurchasedItems(t *testing.T) {
	userUUID := uuid.New()
	productUUID := uuid.New()

	mockRepo := new(mockCartRepository)
	mockCatalog := new(mockCatalogClient)

	mockRepo.On("GetCartByUserUUID", mock.Anything, userUUID).Return(sqlc.Cart{
		CartID:   1,
		UserUuid: userUUID,
	}, nil)

	mockRepo.On("DeleteCartItemsByProductUUIDs", mock.Anything, int32(1), []uuid.UUID{productUUID}).Return(nil)

	svc := NewCartService(mockRepo, mockCatalog, nil)
	err := svc.RemovePurchasedItems(context.Background(), &userUUID, nil, []uuid.UUID{productUUID}, nil)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}
