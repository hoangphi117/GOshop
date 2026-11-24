package service

import (
	"context"
	"encoding/json"
	"strings"

	catalogsqlc "goshop/services/catalog/internal/db/sqlc"
	"goshop/services/catalog/internal/dto"
	"goshop/services/catalog/internal/repository"
	"goshop/shared/pkg/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// CatalogVariantService contains variant-related methods separated for clarity.
// These are implemented on *catalogService which already satisfies CatalogService.

// CatalogService variant interface additions — added to the interface in catalog_service.go.

// CreateVariant validates and creates a new product variant.
func (s *catalogService) CreateVariant(ctx context.Context, productUUID uuid.UUID, input dto.CreateVariantInput) (*dto.ProductVariantDTO, error) {
	// Resolve product_id from UUID
	product, err := s.repo.GetProductByUUID(ctx, productUUID)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeNotFound, "product not found")
	}

	attrsJSON, err := json.Marshal(input.Attributes)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "invalid attributes format")
	}

	status := int32(1)
	if input.Status != 0 {
		status = input.Status
	}
	position := input.Position

	v, err := s.repo.CreateVariant(ctx, catalogsqlc.CreateVariantParams{
		ProductID:             product.ProductID,
		VariantSku:            input.SKU,
		VariantTitle:          input.Title,
		VariantPrice:          utils.FloatToNumeric(input.Price),
		VariantCompareAtPrice: utils.FloatPointerToNumeric(input.CompareAtPrice),
		VariantImageUrl:       input.ImageURL,
		VariantAttributes:     attrsJSON,
		VariantPosition:       position,
		VariantStatus:         status,
	})
	if err != nil {
		return nil, mapVariantDBError(err)
	}

	result := dto.MapVariantToDTO(v)
	return &result, nil
}

// GetVariantsByProduct returns all variants (active-only for storefront, all for admin).
func (s *catalogService) GetVariantsByProduct(ctx context.Context, productUUID uuid.UUID, activeOnly bool) ([]dto.ProductVariantDTO, error) {
	product, err := s.repo.GetProductByUUID(ctx, productUUID)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeNotFound, "product not found")
	}

	var variants []catalogsqlc.ProductVariant
	if activeOnly {
		variants, err = s.repo.ListActiveVariantsByProductID(ctx, product.ProductID)
	} else {
		variants, err = s.repo.ListVariantsByProductID(ctx, product.ProductID)
	}
	if err != nil {
		return nil, err
	}
	return dto.MapVariantsToDTOs(variants), nil
}

// UpdateVariant patches a variant by its UUID.
func (s *catalogService) UpdateVariant(ctx context.Context, productUUID, variantUUID uuid.UUID, input dto.UpdateVariantInput) (*dto.ProductVariantDTO, error) {
	// Confirm product exists (authorization guard)
	product, err := s.repo.GetProductByUUID(ctx, productUUID)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeNotFound, "product not found")
	}

	existing, err := s.repo.GetVariantByUUID(ctx, variantUUID)
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeNotFound, "variant not found")
	}
	if existing.ProductID != product.ProductID {
		return nil, utils.NewError(utils.ErrCodeForbidden, "variant does not belong to this product")
	}

	var attrsJSON []byte
	if input.Attributes != nil {
		attrsJSON, err = json.Marshal(input.Attributes)
		if err != nil {
			return nil, utils.NewError(utils.ErrCodeBadRequest, "invalid attributes format")
		}
	}

	var priceNum, compareNum pgtype.Numeric
	if input.Price != nil {
		priceNum = utils.FloatToNumeric(*input.Price)
	}
	if input.CompareAtPrice != nil {
		compareNum = utils.FloatToNumeric(*input.CompareAtPrice)
	}

	params := catalogsqlc.UpdateVariantParams{
		VariantUuid:           variantUUID,
		VariantSku:            input.SKU,
		VariantTitle:          input.Title,
		VariantPrice:          priceNum,
		VariantCompareAtPrice: compareNum,
		VariantImageUrl:       input.ImageURL,
		VariantAttributes:     attrsJSON,
		VariantPosition:       input.Position,
		VariantStatus:         input.Status,
	}

	v, err := s.repo.UpdateVariant(ctx, params)
	if err != nil {
		return nil, mapVariantDBError(err)
	}
	result := dto.MapVariantToDTO(v)
	return &result, nil
}

// DeleteVariant removes a variant permanently.
func (s *catalogService) DeleteVariant(ctx context.Context, productUUID, variantUUID uuid.UUID) error {
	product, err := s.repo.GetProductByUUID(ctx, productUUID)
	if err != nil {
		return utils.NewError(utils.ErrCodeNotFound, "product not found")
	}

	existing, err := s.repo.GetVariantByUUID(ctx, variantUUID)
	if err != nil {
		return utils.NewError(utils.ErrCodeNotFound, "variant not found")
	}
	if existing.ProductID != product.ProductID {
		return utils.NewError(utils.ErrCodeForbidden, "variant does not belong to this product")
	}

	_, err = s.repo.DeleteVariant(ctx, variantUUID)
	return err
}

// mapVariantDBError converts pgx unique-violation errors to user-friendly ones.
func mapVariantDBError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if containsAny(msg, "idx_variants_sku_unique", "variant_sku") {
		return utils.NewError(utils.ErrCodeConflict, "variant SKU already exists")
	}
	return err
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) && containsStr(s, sub) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}

// Satisfy the unused import of repository for IDE tools.
var _ repository.CatalogRepository
