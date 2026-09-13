package v1service

import (
	"context"
	"fmt"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"sort"

	"github.com/google/uuid"
)

type DeductItem struct {
	ProductUUID string
	Quantity    int32
}

type RestoreItem struct {
	ProductID int32
	Quantity  int32
}

type inventoryService struct {
	productRepo repository.ProductRepository
}

func NewInventoryService(productRepo repository.ProductRepository) InventoryService {
	return &inventoryService{
		productRepo: productRepo,
	}
}

func (s *inventoryService) LockAndDeductStock(ctx context.Context, items []DeductItem) (map[string]sqlc.GetByUUIDsForUpdateRow, error) {
	if len(items) == 0 {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Item list cannot be empty")
	}

	sortedItems := make([]DeductItem, len(items))
	copy(sortedItems, items)
	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].ProductUUID < sortedItems[j].ProductUUID
	})

	uuids := make([]uuid.UUID, len(sortedItems))
	for i, item := range sortedItems {
		if item.Quantity <= 0 {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Quantity must be greater than zero")
		}
		parsed, err := uuid.Parse(item.ProductUUID)
		if err != nil {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Invalid product UUID format: "+item.ProductUUID)
		}
		uuids[i] = parsed
	}

	tx, _ := db.GetTx(ctx)

	products, err := s.productRepo.GetByUUIDs(ctx, tx, uuids)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch products", err)
	}

	if len(products) != len(items) {
		return nil, utils.NewError(utils.ErrCodeNotFound, "One or more products not found")
	}

	productMap := make(map[string]sqlc.GetByUUIDsForUpdateRow, len(products))
	for _, p := range products {
		productMap[p.ProductUuid.String()] = p
	}

	for _, item := range sortedItems {
		product, ok := productMap[item.ProductUUID]
		if !ok {
			return nil, utils.NewError(utils.ErrCodeNotFound, "Product not found: "+item.ProductUUID)
		}

		if product.ProductStatus != ProductStatusActive {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "Product is unavailable: "+item.ProductUUID)
		}

		currentStock := utils.Int32Value(product.ProductStock)
		if currentStock < item.Quantity {
			return nil, utils.NewError(utils.ErrCodeBadRequest, fmt.Sprintf("Insufficient stock for product: %s (available: %d, requested: %d)", product.ProductName, currentStock, item.Quantity))
		}

		if err := s.productRepo.DeductStock(ctx, tx, product.ProductID, item.Quantity); err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to deduct stock for product: "+item.ProductUUID, err)
		}

	}
	return productMap, nil
}

func (s *inventoryService) RestoreStock(ctx context.Context, items []RestoreItem) error {
	if len(items) == 0 {
		return nil
	}
	// Sort items by ProductID to avoid deadlocks
	sortedItems := make([]RestoreItem, len(items))
	copy(sortedItems, items)
	sort.Slice(sortedItems, func(i, j int) bool {
		return sortedItems[i].ProductID < sortedItems[j].ProductID
	})
	tx, _ := db.GetTx(ctx)
	for _, item := range sortedItems {
		if item.Quantity <= 0 {
			continue
		}
		if err := s.productRepo.RestoreStock(ctx, tx, item.ProductID, item.Quantity); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, fmt.Sprintf("Failed to restore stock for product ID: %d", item.ProductID), err)
		}
	}
	return nil
}
