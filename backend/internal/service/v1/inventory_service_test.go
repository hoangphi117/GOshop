package v1service_test

import (
	"context"
	"testing"

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

func TestInventoryService_LockAndDeductStock(t *testing.T) {
	ctx := context.Background()
	productUUID1 := uuid.New()

	t.Run("Case 1: Sufficient Stock - Deduct stock successfully", func(t *testing.T) {
		deductedIDs := make([]int32, 0)

		mockRepo := &testutils.MockProductRepository{
			GetByUUIDsFunc: func(ctx context.Context, tx pgx.Tx, uuids []uuid.UUID) ([]sqlc.GetByUUIDsForUpdateRow, error) {
				return []sqlc.GetByUUIDsForUpdateRow{
					{
						ProductID:     101,
						ProductUuid:   productUUID1,
						ProductName:   "iPhone 15",
						ProductStock:  utils.ConvertToInt32Pointer(10), // 10 left
						ProductStatus: v1service.ProductStatusActive,
					},
				}, nil
			},
			DeductStockFunc: func(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) error {
				deductedIDs = append(deductedIDs, productID)
				assert.Equal(t, int32(101), productID)
				assert.Equal(t, int32(2), quantity)
				return nil
			},
		}

		svc := v1service.NewInventoryService(mockRepo, nil, nil, nil)
		items := []v1service.DeductItem{
			{ProductUUID: productUUID1.String(), Quantity: 2},
		}

		productMap, err := svc.LockAndDeductStock(ctx, items)

		require.NoError(t, err)
		assert.Len(t, productMap, 1)
		assert.Contains(t, productMap, productUUID1.String())
		assert.Equal(t, []int32{101}, deductedIDs)
	})

	t.Run("Case 2: Out of Stock - Return Insufficient stock", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			GetByUUIDsFunc: func(ctx context.Context, tx pgx.Tx, uuids []uuid.UUID) ([]sqlc.GetByUUIDsForUpdateRow, error) {
				return []sqlc.GetByUUIDsForUpdateRow{
					{
						ProductID:     102,
						ProductUuid:   productUUID1,
						ProductName:   "MacBook Pro",
						ProductStock:  utils.ConvertToInt32Pointer(0), // Out of stock
						ProductStatus: v1service.ProductStatusActive,
					},
				}, nil
			},
		}

		svc := v1service.NewInventoryService(mockRepo, nil, nil, nil)
		items := []v1service.DeductItem{
			{ProductUUID: productUUID1.String(), Quantity: 1},
		}

		_, err := svc.LockAndDeductStock(ctx, items)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Insufficient stock for product: MacBook Pro")
		assert.Contains(t, err.Error(), "available: 0, requested: 1")
	})

	t.Run("Case 3: Insufficient Stock - Return Insufficient stock", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			GetByUUIDsFunc: func(ctx context.Context, tx pgx.Tx, uuids []uuid.UUID) ([]sqlc.GetByUUIDsForUpdateRow, error) {
				return []sqlc.GetByUUIDsForUpdateRow{
					{
						ProductID:     103,
						ProductUuid:   productUUID1,
						ProductName:   "iPad Air",
						ProductStock:  utils.ConvertToInt32Pointer(3), // Only 3 left
						ProductStatus: v1service.ProductStatusActive,
					},
				}, nil
			},
		}

		svc := v1service.NewInventoryService(mockRepo, nil, nil, nil)
		items := []v1service.DeductItem{
			{ProductUUID: productUUID1.String(), Quantity: 5}, // Requesting 5, but only 3 available
		}

		_, err := svc.LockAndDeductStock(ctx, items)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Insufficient stock for product: iPad Air")
		assert.Contains(t, err.Error(), "available: 3, requested: 5")
	})

	t.Run("Case 4: Inactive Product", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			GetByUUIDsFunc: func(ctx context.Context, tx pgx.Tx, uuids []uuid.UUID) ([]sqlc.GetByUUIDsForUpdateRow, error) {
				return []sqlc.GetByUUIDsForUpdateRow{
					{
						ProductID:     104,
						ProductUuid:   productUUID1,
						ProductName:   "Discontinued Phone",
						ProductStock:  utils.ConvertToInt32Pointer(50),
						ProductStatus: 2, // Inactive
					},
				}, nil
			},
		}

		svc := v1service.NewInventoryService(mockRepo, nil, nil, nil)
		items := []v1service.DeductItem{
			{ProductUUID: productUUID1.String(), Quantity: 1},
		}

		_, err := svc.LockAndDeductStock(ctx, items)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Product is unavailable")
	})

	t.Run("Case 5: Product not found in DB", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			GetByUUIDsFunc: func(ctx context.Context, tx pgx.Tx, uuids []uuid.UUID) ([]sqlc.GetByUUIDsForUpdateRow, error) {
				return []sqlc.GetByUUIDsForUpdateRow{}, nil // No products found
			},
		}

		svc := v1service.NewInventoryService(mockRepo, nil, nil, nil)
		items := []v1service.DeductItem{
			{ProductUUID: productUUID1.String(), Quantity: 1},
		}

		_, err := svc.LockAndDeductStock(ctx, items)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "One or more products not found")
	})

	t.Run("Case 6: Invalid UUID format", func(t *testing.T) {
		svc := v1service.NewInventoryService(&testutils.MockProductRepository{}, nil, nil, nil)
		items := []v1service.DeductItem{
			{ProductUUID: "invalid-uuid-format", Quantity: 1},
		}

		_, err := svc.LockAndDeductStock(ctx, items)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid product UUID format")
	})

	t.Run("Case 7: Low Stock (<= 5) Triggers Admin Notification", func(t *testing.T) {
		mockRepo := &testutils.MockProductRepository{
			GetByUUIDsFunc: func(ctx context.Context, tx pgx.Tx, uuids []uuid.UUID) ([]sqlc.GetByUUIDsForUpdateRow, error) {
				return []sqlc.GetByUUIDsForUpdateRow{
					{
						ProductID:     105,
						ProductUuid:   productUUID1,
						ProductName:   "Keychron Q1",
						ProductStock:  utils.ConvertToInt32Pointer(6),
						ProductStatus: v1service.ProductStatusActive,
					},
				}, nil
			},
			DeductStockFunc: func(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) error {
				return nil
			},
		}

		notified := false
		mockNotif := &testutils.MockNotificationService{
			NotifyAdminsAsyncFunc: func(ctx context.Context, input v1dto.AdminNotifyInput) error {
				assert.Equal(t, "stock", input.Type)
				assert.Equal(t, "Low Stock Alert", input.Title)
				assert.Equal(t, int32(4), input.Data["stock"])
				notified = true
				return nil
			},
		}

		svc := v1service.NewInventoryService(mockRepo, nil, nil, nil, mockNotif)
		items := []v1service.DeductItem{
			{ProductUUID: productUUID1.String(), Quantity: 2}, // 6 - 2 = 4 (<= 5)
		}

		_, err := svc.LockAndDeductStock(ctx, items)
		require.NoError(t, err)
		assert.True(t, notified, "Expected admin low stock notification to be sent")
	})
}

func TestInventoryService_RestoreStock(t *testing.T) {
	ctx := context.Background()

	t.Run("Success - Restore stock", func(t *testing.T) {
		restored := make(map[int32]int32)
		mockRepo := &testutils.MockProductRepository{
			RestoreStockFunc: func(ctx context.Context, tx pgx.Tx, productID int32, quantity int32) error {
				restored[productID] = quantity
				return nil
			},
		}

		svc := v1service.NewInventoryService(mockRepo, nil, nil, nil)
		items := []v1service.RestoreItem{
			{ProductID: 101, Quantity: 2},
			{ProductID: 102, Quantity: 3},
		}

		err := svc.RestoreStock(ctx, items)

		require.NoError(t, err)
		assert.Equal(t, int32(2), restored[101])
		assert.Equal(t, int32(3), restored[102])
	})
}
