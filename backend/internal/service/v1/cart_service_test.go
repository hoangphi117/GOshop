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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCartService_GetMyCart(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	cartUUID := uuid.New()
	itemUUID := uuid.New()
	productUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewCartService(nil, mockUserRepo, nil, nil)
		cart, err := svc.GetMyCart(ctx, userUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
		assert.Empty(t, cart.UUID)
	})

	t.Run("Case 2: GetOrCreateCart Fails -> Returns Internal Error", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{}, errors.New("db connection failure")
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, nil, nil)
		cart, err := svc.GetMyCart(ctx, userUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
		assert.Empty(t, cart.UUID)
	})

	t.Run("Case 3: GetCartItems Fails -> Returns Internal Error", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			GetCartItemsFunc: func(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error) {
				return nil, errors.New("failed to query items")
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, nil, nil)
		cart, err := svc.GetMyCart(ctx, userUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
		assert.Empty(t, cart.UUID)
	})

	t.Run("Case 4: Happy Path -> Returns CartDTO with Calculated Items", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			GetCartItemsFunc: func(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error) {
				stock := int32(50)
				return []sqlc.GetCartItemsWithProductDetailsRow{
					{
						CartItemUuid:      itemUUID,
						CartItemQuantity:  2,
						CartItemCreatedAt: time.Now(),
						ProductUuid:       productUUID,
						ProductName:       "Ergonomic Chair",
						ProductSlug:       "ergonomic-chair",
						ProductPrice:      utils.FloatToNumeric(99.99),
						ProductStock:      &stock,
						ProductStatus:     1, // Active
					},
				}, nil
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, nil, nil)
		cart, err := svc.GetMyCart(ctx, userUUID)

		require.NoError(t, err)
		assert.Equal(t, cartUUID.String(), cart.UUID)
		assert.Len(t, cart.Items, 1)
		assert.Equal(t, itemUUID.String(), cart.Items[0].UUID)
		assert.Equal(t, int32(2), cart.Items[0].Quantity)
		assert.Equal(t, 199.98, cart.TotalAmount)
		assert.Equal(t, int32(1), cart.TotalItems)
	})
}

func TestCartService_AddItemToCart(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	cartUUID := uuid.New()
	itemUUID := uuid.New()
	productUUID := uuid.New()

	t.Run("Case 1: Invalid Product UUID -> Returns 400", func(t *testing.T) {
		svc := v1service.NewCartService(nil, nil, nil, nil)
		cart, err := svc.AddItemToCart(ctx, userUUID, v1dto.AddCartItemInput{
			ProductUUID: "invalid-uuid",
			Quantity:    1,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Empty(t, cart.UUID)
	})

	t.Run("Case 2: Product Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
		}
		mockProductRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, mockProductRepo, nil)
		cart, err := svc.AddItemToCart(ctx, userUUID, v1dto.AddCartItemInput{
			ProductUUID: productUUID.String(),
			Quantity:    1,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
		assert.Empty(t, cart.UUID)
	})

	t.Run("Case 3: Product Inactive/Unavailable -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
		}
		mockProductRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{
					ProductID:     100,
					ProductUuid:   productUUID,
					ProductStatus: 2, // Inactive/Draft
				}, nil
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, mockProductRepo, nil)
		cart, err := svc.AddItemToCart(ctx, userUUID, v1dto.AddCartItemInput{
			ProductUUID: productUUID.String(),
			Quantity:    1,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "unavailable for purchase")
		assert.Empty(t, cart.UUID)
	})

	t.Run("Case 4: Insufficient Stock (Requested + Existing > Stock) -> Returns 400", func(t *testing.T) {
		stock := int32(5)
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			GetCartItemQuantityFunc: func(ctx context.Context, cartID, productID int32) (int32, error) {
				return 3, nil // Already 3 in cart
			},
		}
		mockProductRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{
					ProductID:     100,
					ProductUuid:   productUUID,
					ProductStock:  &stock,
					ProductStatus: 1,
				}, nil
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, mockProductRepo, nil)
		cart, err := svc.AddItemToCart(ctx, userUUID, v1dto.AddCartItemInput{
			ProductUUID: productUUID.String(),
			Quantity:    3, // 3 + 3 = 6 > 5
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "insufficient stock")
		assert.Empty(t, cart.UUID)
	})

	t.Run("Case 5: Happy Path -> Adds Item and Returns Updated CartDTO", func(t *testing.T) {
		stock := int32(10)
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			GetCartItemQuantityFunc: func(ctx context.Context, cartID, productID int32) (int32, error) {
				return 0, nil
			},
			AddOrUpdateItemFunc: func(ctx context.Context, params sqlc.AddOrUpdateCartItemParams) (sqlc.CartItem, error) {
				assert.Equal(t, int32(10), params.CartID)
				assert.Equal(t, int32(100), params.ProductID)
				assert.Equal(t, int32(2), params.Quantity)
				return sqlc.CartItem{CartItemID: 1, CartItemUuid: itemUUID, CartItemCartID: 10, CartItemProductID: 100, CartItemQuantity: 2}, nil
			},
			GetCartItemsFunc: func(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error) {
				return []sqlc.GetCartItemsWithProductDetailsRow{
					{
						CartItemUuid:      itemUUID,
						CartItemQuantity:  2,
						CartItemCreatedAt: time.Now(),
						ProductUuid:       productUUID,
						ProductName:       "Mechanical Keyboard",
						ProductSlug:       "mechanical-keyboard",
						ProductPrice:      utils.FloatToNumeric(50.00),
						ProductStock:      &stock,
						ProductStatus:     1,
					},
				}, nil
			},
		}
		mockProductRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{
					ProductID:     100,
					ProductUuid:   productUUID,
					ProductStock:  &stock,
					ProductStatus: 1,
				}, nil
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, mockProductRepo, nil)
		cart, err := svc.AddItemToCart(ctx, userUUID, v1dto.AddCartItemInput{
			ProductUUID: productUUID.String(),
			Quantity:    2,
		})

		require.NoError(t, err)
		assert.Equal(t, cartUUID.String(), cart.UUID)
		assert.Len(t, cart.Items, 1)
		assert.Equal(t, int32(2), cart.Items[0].Quantity)
		assert.Equal(t, 100.0, cart.TotalAmount)
	})
}

func TestCartService_UpdateCartItemQuantity(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	cartUUID := uuid.New()
	itemUUID := uuid.New()
	productUUID := uuid.New()

	t.Run("Case 1: Quantity <= 0 -> Calls RemoveItemFromCart", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		deleted := false
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			DeleteItemFunc: func(ctx context.Context, cartID int32, itemU uuid.UUID) error {
				assert.Equal(t, itemUUID, itemU)
				deleted = true
				return nil
			},
			GetCartItemsFunc: func(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error) {
				return []sqlc.GetCartItemsWithProductDetailsRow{}, nil
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, nil, nil)
		zeroQty := int32(0)
		cart, err := svc.UpdateCartItemQuantity(ctx, userUUID, itemUUID, v1dto.UpdateCartItemQuantityInput{
			Quantity: &zeroQty,
		})

		require.NoError(t, err)
		assert.True(t, deleted)
		assert.Empty(t, cart.Items)
	})

	t.Run("Case 2: Cart Item Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			GetCartItemsFunc: func(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error) {
				return []sqlc.GetCartItemsWithProductDetailsRow{}, nil
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, nil, nil)
		qty := int32(3)
		cart, err := svc.UpdateCartItemQuantity(ctx, userUUID, itemUUID, v1dto.UpdateCartItemQuantityInput{
			Quantity: &qty,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
		assert.Empty(t, cart.UUID)
	})

	t.Run("Case 3: Product Inactive -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			GetCartItemsFunc: func(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error) {
				return []sqlc.GetCartItemsWithProductDetailsRow{
					{
						CartItemUuid:  itemUUID,
						ProductStatus: 2, // Inactive
					},
				}, nil
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, nil, nil)
		qty := int32(3)
		cart, err := svc.UpdateCartItemQuantity(ctx, userUUID, itemUUID, v1dto.UpdateCartItemQuantityInput{
			Quantity: &qty,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Empty(t, cart.UUID)
	})

	t.Run("Case 4: Insufficient Stock -> Returns 400", func(t *testing.T) {
		stock := int32(4)
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			GetCartItemsFunc: func(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error) {
				return []sqlc.GetCartItemsWithProductDetailsRow{
					{
						CartItemUuid:  itemUUID,
						ProductStock:  &stock,
						ProductStatus: 1,
					},
				}, nil
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, nil, nil)
		qty := int32(10)
		cart, err := svc.UpdateCartItemQuantity(ctx, userUUID, itemUUID, v1dto.UpdateCartItemQuantityInput{
			Quantity: &qty,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "insufficient stock")
		assert.Empty(t, cart.UUID)
	})

	t.Run("Case 5: Happy Path -> Updates Item Quantity and Returns Updated CartDTO", func(t *testing.T) {
		stock := int32(20)
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			GetCartItemsFunc: func(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error) {
				return []sqlc.GetCartItemsWithProductDetailsRow{
					{
						CartItemUuid:      itemUUID,
						CartItemQuantity:  5,
						CartItemCreatedAt: time.Now(),
						ProductUuid:       productUUID,
						ProductName:       "Desk Lamp",
						ProductSlug:       "desk-lamp",
						ProductPrice:      utils.FloatToNumeric(25.00),
						ProductStock:      &stock,
						ProductStatus:     1,
					},
				}, nil
			},
			SetCartItemQuantityFunc: func(ctx context.Context, params sqlc.SetCartItemQuantityParams) (sqlc.CartItem, error) {
				assert.Equal(t, int32(5), params.Quantity)
				assert.Equal(t, itemUUID, params.CartItemUuid)
				assert.Equal(t, int32(10), params.CartID)
				return sqlc.CartItem{CartItemID: 1, CartItemUuid: itemUUID, CartItemQuantity: 5}, nil
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, nil, nil)
		qty := int32(5)
		cart, err := svc.UpdateCartItemQuantity(ctx, userUUID, itemUUID, v1dto.UpdateCartItemQuantityInput{
			Quantity: &qty,
		})

		require.NoError(t, err)
		assert.Equal(t, cartUUID.String(), cart.UUID)
		assert.Len(t, cart.Items, 1)
		assert.Equal(t, int32(5), cart.Items[0].Quantity)
		assert.Equal(t, 125.0, cart.TotalAmount)
	})
}

func TestCartService_RemoveItemFromCart(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	cartUUID := uuid.New()
	itemUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewCartService(nil, mockUserRepo, nil, nil)
		cart, err := svc.RemoveItemFromCart(ctx, userUUID, itemUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
		assert.Empty(t, cart.UUID)
	})

	t.Run("Case 2: DeleteItem Fails with NoRows -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			DeleteItemFunc: func(ctx context.Context, cartID int32, itemU uuid.UUID) error {
				return sql.ErrNoRows
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, nil, nil)
		cart, err := svc.RemoveItemFromCart(ctx, userUUID, itemUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
		assert.Empty(t, cart.UUID)
	})

	t.Run("Case 3: Happy Path -> Removes Item and Returns Empty CartDTO", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			DeleteItemFunc: func(ctx context.Context, cartID int32, itemU uuid.UUID) error {
				assert.Equal(t, int32(10), cartID)
				assert.Equal(t, itemUUID, itemU)
				return nil
			},
			GetCartItemsFunc: func(ctx context.Context, cartID int32) ([]sqlc.GetCartItemsWithProductDetailsRow, error) {
				return []sqlc.GetCartItemsWithProductDetailsRow{}, nil
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, nil, nil)
		cart, err := svc.RemoveItemFromCart(ctx, userUUID, itemUUID)

		require.NoError(t, err)
		assert.Equal(t, cartUUID.String(), cart.UUID)
		assert.Empty(t, cart.Items)
		assert.Equal(t, 0.0, cart.TotalAmount)
		assert.Equal(t, int32(0), cart.TotalItems)
	})
}

func TestCartService_ClearMyCart(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	cartUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewCartService(nil, mockUserRepo, nil, nil)
		err := svc.ClearMyCart(ctx, userUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: ClearCart DB Fails -> Returns Internal Error", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			ClearCartFunc: func(ctx context.Context, cartID int32) error {
				return errors.New("failed to truncate cart items")
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, nil, nil)
		err := svc.ClearMyCart(ctx, userUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 3: Happy Path -> Clears Cart Successfully", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		cleared := false
		mockCartRepo := &testutils.MockCartRepository{
			GetOrCreateCartFunc: func(ctx context.Context, userID int32) (sqlc.Cart, error) {
				return sqlc.Cart{CartID: 10, CartUuid: cartUUID, CartUserID: 1}, nil
			},
			ClearCartFunc: func(ctx context.Context, cartID int32) error {
				assert.Equal(t, int32(10), cartID)
				cleared = true
				return nil
			},
		}

		svc := v1service.NewCartService(mockCartRepo, mockUserRepo, nil, nil)
		err := svc.ClearMyCart(ctx, userUUID)

		require.NoError(t, err)
		assert.True(t, cleared)
	})
}
