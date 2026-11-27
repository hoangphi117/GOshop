package dto

import (
	"encoding/json"
	catalogsqlc "goshop/services/catalog/internal/db/sqlc"
	"goshop/shared/pkg/utils"
	"time"
)

// =============================================================================
// PRODUCT VARIANTS
// =============================================================================

// ProductVariantDTO is the outbound representation of a SKU variant.
type ProductVariantDTO struct {
	UUID           string            `json:"uuid"`
	SKU            string            `json:"sku"`
	Title          string            `json:"title"`
	Price          float64           `json:"price"`
	CompareAtPrice *float64          `json:"compare_at_price,omitempty"`
	ImageURL       *string           `json:"image_url,omitempty"`
	Attributes     map[string]string `json:"attributes"`
	Position       int32             `json:"position"`
	Status         string            `json:"status"`
	CreatedAt      string            `json:"created_at"`
	UpdatedAt      string            `json:"updated_at"`
}

func (v ProductVariantDTO) MarshalJSON() ([]byte, error) {
	type Alias ProductVariantDTO
	return json.Marshal(&struct {
		Alias
		ImageUrlAlias *string `json:"imageUrl,omitempty"`
	}{
		Alias:         Alias(v),
		ImageUrlAlias: v.ImageURL,
	})
}

// CreateVariantInput is the request body for POST /products/:uuid/variants.
type CreateVariantInput struct {
	SKU            string            `json:"sku" binding:"required,min=1,max=100"`
	Title          string            `json:"title" binding:"required,min=1,max=255"`
	Price          float64           `json:"price" binding:"required,gt=0"`
	CompareAtPrice *float64          `json:"compare_at_price" binding:"omitempty,gt=0"`
	ImageURL       *string           `json:"image_url"`
	Attributes     map[string]string `json:"attributes"`
	Position       int32             `json:"position" binding:"gte=0"`
	Status         int32             `json:"status" binding:"omitempty,oneof=1 2"`
}

// UpdateVariantInput is the request body for PUT /products/:uuid/variants/:variantUUID.
type UpdateVariantInput struct {
	VariantUUID    string
	SKU            *string           `json:"sku" binding:"omitempty,min=1,max=100"`
	Title          *string           `json:"title" binding:"omitempty,min=1,max=255"`
	Price          *float64          `json:"price" binding:"omitempty,gt=0"`
	CompareAtPrice *float64          `json:"compare_at_price" binding:"omitempty,gt=0"`
	ImageURL       *string           `json:"image_url"`
	Attributes     map[string]string `json:"attributes"`
	Position       *int32            `json:"position" binding:"omitempty,gte=0"`
	Status         *int32            `json:"status" binding:"omitempty,oneof=1 2"`
}

// MapVariantToDTO converts a ProductVariant DB model to DTO.
func MapVariantToDTO(v catalogsqlc.ProductVariant) ProductVariantDTO {
	status := "Active"
	if v.VariantStatus == 2 {
		status = "Inactive"
	}
	attrs := v.VariantAttributes
	if attrs == nil {
		attrs = make(map[string]string)
	}

	var compareAtPrice *float64
	if v.VariantCompareAtPrice.Valid {
		f := utils.NumericToFloat(v.VariantCompareAtPrice)
		compareAtPrice = &f
	}

	return ProductVariantDTO{
		UUID:           v.VariantUuid.String(),
		SKU:            v.VariantSku,
		Title:          v.VariantTitle,
		Price:          utils.NumericToFloat(v.VariantPrice),
		CompareAtPrice: compareAtPrice,
		ImageURL:       v.VariantImageUrl,
		Attributes:     attrs,
		Position:       v.VariantPosition,
		Status:         status,
		CreatedAt:      v.VariantCreatedAt.Format(time.RFC3339),
		UpdatedAt:      v.VariantUpdatedAt.Format(time.RFC3339),
	}
}

// MapVariantsToDTOs converts a slice.
func MapVariantsToDTOs(variants []catalogsqlc.ProductVariant) []ProductVariantDTO {
	result := make([]ProductVariantDTO, len(variants))
	for i, v := range variants {
		result[i] = MapVariantToDTO(v)
	}
	return result
}

type CategoryDTO struct {
	CategoryID    int32         `json:"category_id"`
	UUID          string        `json:"uuid"`
	Name          string        `json:"category_name"`
	Slug          string        `json:"slug"`
	Description   *string       `json:"description,omitempty"`
	Status        string        `json:"status"`
	CreatedAt     string        `json:"created_at"`
	UpdatedAt     string        `json:"updated_at,omitempty"`
	ParentID      *int32        `json:"parent_id,omitempty"`
	CategoryLevel int32         `json:"category_level"`
	Children      []CategoryDTO `json:"children,omitempty"`
}

func (c CategoryDTO) MarshalJSON() ([]byte, error) {
	type Alias CategoryDTO
	return json.Marshal(&struct {
		Alias
		NameAlias          string        `json:"name,omitempty"`
		CategoryIdAlias    int32         `json:"categoryId,omitempty"`
		CreatedAtAlias     string        `json:"createdAt,omitempty"`
		ParentIdAlias      *int32        `json:"parentId,omitempty"`
		CategoryLevelAlias int32         `json:"categoryLevel,omitempty"`
		ChildrenAlias      []CategoryDTO `json:"children,omitempty"`
	}{
		Alias:              Alias(c),
		NameAlias:          c.Name,
		CategoryIdAlias:    c.CategoryID,
		CreatedAtAlias:     c.CreatedAt,
		ParentIdAlias:      c.ParentID,
		CategoryLevelAlias: c.CategoryLevel,
		ChildrenAlias:      c.Children,
	})
}

type CreateCategoryInput struct {
	Name        string  `json:"category_name"`
	AltName     string  `json:"name"`
	Description *string `json:"description"`
	Status      int32   `json:"status" binding:"omitempty,oneof=1 2"`
	ParentID    *int32  `json:"parent_id"`
	ParentUUID  *string `json:"parent_uuid"`
}

type UpdateCategoryInput struct {
	UUID        string
	Name        *string `json:"category_name"`
	AltName     *string `json:"name"`
	Description *string `json:"description"`
	Status      *int32  `json:"status" binding:"omitempty,oneof=1 2"`
	ParentID    *int32  `json:"parent_id"`
	ParentUUID  *string `json:"parent_uuid"`
}

type ProductDTO struct {
	ID             int32               `json:"id,omitempty"`
	UUID           string              `json:"uuid"`
	Name           string              `json:"name"`
	Slug           string              `json:"slug"`
	Description    *string             `json:"description,omitempty"`
	Price          float64             `json:"price"`
	Stock          int32               `json:"stock"`
	SKU            *string             `json:"sku,omitempty"`
	CategoryID     int32               `json:"category_id,omitempty"`
	CategoryUUID   string              `json:"category_uuid,omitempty"`
	CategoryName   string              `json:"category_name,omitempty"`
	ImageURL       *string             `json:"image_url,omitempty"`
	Status         string              `json:"status"`
	AvgRating      float64             `json:"rating"`
	ReviewCount    int32               `json:"reviews_count"`
	CreatedAt      string              `json:"created_at"`
	UpdatedAt      string              `json:"updated_at,omitempty"`
	Specifications map[string]string   `json:"specifications,omitempty"`
	Variants       []ProductVariantDTO `json:"variants,omitempty"`
}

func (p ProductDTO) MarshalJSON() ([]byte, error) {
	type Alias ProductDTO
	return json.Marshal(&struct {
		Alias
		ImageUrlAlias     *string `json:"imageUrl,omitempty"`
		CategoryUuidAlias string  `json:"categoryUuid,omitempty"`
		CategoryNameAlias string  `json:"categoryName,omitempty"`
		CreatedAtAlias    string  `json:"createdAt,omitempty"`
		AvgRatingAlias    float64 `json:"avgRating,omitempty"`
		ReviewCountAlias  int32   `json:"reviewCount,omitempty"`
	}{
		Alias:             Alias(p),
		ImageUrlAlias:     p.ImageURL,
		CategoryUuidAlias: p.CategoryUUID,
		CategoryNameAlias: p.CategoryName,
		CreatedAtAlias:    p.CreatedAt,
		AvgRatingAlias:    p.AvgRating,
		ReviewCountAlias:  p.ReviewCount,
	})
}

type CreateProductInput struct {
	CategoryUUID   string               `json:"category_uuid"`
	AltCatUUID     string               `json:"categoryUuid"`
	Name           string               `json:"name" binding:"required,min=2,max=200"`
	Description    *string              `json:"description"`
	Price          float64              `json:"price" binding:"required,gt=0"`
	Stock          int32                `json:"stock" binding:"gte=0"`
	SKU            *string              `json:"sku"`
	ImageURL       *string              `json:"image_url"`
	AltImageURL    *string              `json:"imageUrl"`
	Status         int32                `json:"status" binding:"omitempty,oneof=1 2 3"`
	Specifications map[string]string    `json:"specifications"`
	Variants       []CreateVariantInput `json:"variants"`
}

type UpdateProductInput struct {
	UUID           string
	CategoryUUID   *string           `json:"category_uuid"`
	AltCatUUID     *string           `json:"categoryUuid"`
	Name           *string           `json:"name" binding:"omitempty,min=2,max=200"`
	Description    *string           `json:"description"`
	Price          *float64          `json:"price" binding:"omitempty,gt=0"`
	Stock          *int32            `json:"stock" binding:"omitempty,gte=0"`
	SKU            *string           `json:"sku"`
	ImageURL       *string           `json:"image_url"`
	AltImageURL    *string           `json:"imageUrl"`
	Status         *int32            `json:"status" binding:"omitempty,oneof=1 2 3"`
	Specifications map[string]string `json:"specifications"`
}

type CSVRowError struct {
	Row    int    `json:"row"`
	Field  string `json:"field,omitempty"`
	Reason string `json:"reason"`
}

type ProductCSVImportResponse struct {
	SuccessCount int           `json:"success_count"`
	ErrorCount   int           `json:"error_count"`
	Errors       []CSVRowError `json:"errors"`
}

func (p ProductCSVImportResponse) MarshalJSON() ([]byte, error) {
	type Alias ProductCSVImportResponse
	return json.Marshal(&struct {
		Alias
		SuccessCountAlias int `json:"successCount"`
		ErrorCountAlias   int `json:"errorCount"`
	}{
		Alias:             Alias(p),
		SuccessCountAlias: p.SuccessCount,
		ErrorCountAlias:   p.ErrorCount,
	})
}

type ProductSuggestionDTO struct {
	UUID         string  `json:"uuid"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Price        float64 `json:"price"`
	ImageURL     *string `json:"image_url,omitempty"`
	CategoryName string  `json:"category_name,omitempty"`
}

func (p ProductSuggestionDTO) MarshalJSON() ([]byte, error) {
	type Alias ProductSuggestionDTO
	return json.Marshal(&struct {
		Alias
		ImageUrlAlias     *string `json:"imageUrl,omitempty"`
		CategoryNameAlias string  `json:"categoryName,omitempty"`
	}{
		Alias:             Alias(p),
		ImageUrlAlias:     p.ImageURL,
		CategoryNameAlias: p.CategoryName,
	})
}

// Facets DTOs
type FacetOptionDTO struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

type FacetGroupDTO struct {
	Key     string           `json:"key"`
	Options []FacetOptionDTO `json:"options"`
}

type SubcategoryCountDTO struct {
	CategoryID   int32  `json:"category_id"`
	UUID         string `json:"uuid"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	ProductCount int64  `json:"product_count"`
}

type CategoryFacetsResponseDTO struct {
	Category      CategoryDTO           `json:"category"`
	Subcategories []SubcategoryCountDTO `json:"subcategories"`
	Facets        []FacetGroupDTO       `json:"facets"`
}

func MapCategoryToDTO(c catalogsqlc.Category) CategoryDTO {
	status := "Active"
	if c.CategoryStatus == 2 {
		status = "Inactive"
	}
	return CategoryDTO{
		CategoryID:    c.CategoryID,
		UUID:          c.CategoryUuid.String(),
		Name:          c.CategoryName,
		Slug:          c.CategorySlug,
		Description:   c.CategoryDescription,
		Status:        status,
		CreatedAt:     c.CategoryCreatedAt.Format(time.RFC3339),
		UpdatedAt:     c.CategoryUpdatedAt.Format(time.RFC3339),
		ParentID:      c.ParentID,
		CategoryLevel: c.CategoryLevel,
	}
}

func MapCategoriesToDTOs(rows []catalogsqlc.Category) []CategoryDTO {
	result := make([]CategoryDTO, len(rows))
	for i, r := range rows {
		result[i] = MapCategoryToDTO(r)
	}
	return result
}

func MapProductRowToDTO(row catalogsqlc.GetProductByUUIDRow) ProductDTO {
	status := "Active"
	switch row.ProductStatus {
	case 2:
		status = "Inactive"
	case 3:
		status = "Out of Stock"
	}

	return ProductDTO{
		ID:           row.ProductID,
		UUID:         row.ProductUuid.String(),
		Name:         row.ProductName,
		Slug:         row.ProductSlug,
		Description:  row.ProductDescription,
		Price:        utils.NumericToFloat(row.ProductPrice),
		Stock:        0,
		SKU:          row.ProductSku,
		CategoryID:   row.ProductCategoryID,
		CategoryUUID: row.CategoryUuid.String(),
		CategoryName: row.CategoryName,
		ImageURL:     row.ProductImageUrl,
		Status:       status,
		AvgRating:    utils.NumericToFloat(row.ProductAvgRating),
		ReviewCount:  row.ProductReviewCount,
		CreatedAt:    row.ProductCreatedAt.Format(time.RFC3339),
		UpdatedAt:    row.ProductUpdatedAt.Format(time.RFC3339),
	}
}

func MapCreateProductRowToDTO(row catalogsqlc.CreateProductRow) ProductDTO {
	status := "Active"
	switch row.ProductStatus {
	case 2:
		status = "Inactive"
	case 3:
		status = "Out of Stock"
	}

	return ProductDTO{
		ID:           row.ProductID,
		UUID:         row.ProductUuid.String(),
		Name:         row.ProductName,
		Slug:         row.ProductSlug,
		Description:  row.ProductDescription,
		Price:        utils.NumericToFloat(row.ProductPrice),
		Stock:        0,
		SKU:          row.ProductSku,
		CategoryID:   row.ProductCategoryID,
		CategoryUUID: row.CategoryUuid.String(),
		CategoryName: row.CategoryName,
		ImageURL:     row.ProductImageUrl,
		Status:       status,
		AvgRating:    utils.NumericToFloat(row.ProductAvgRating),
		ReviewCount:  row.ProductReviewCount,
		CreatedAt:    row.ProductCreatedAt.Format(time.RFC3339),
		UpdatedAt:    row.ProductUpdatedAt.Format(time.RFC3339),
	}
}

func MapUpdateProductRowToDTO(row catalogsqlc.UpdateProductRow) ProductDTO {
	status := "Active"
	switch row.ProductStatus {
	case 2:
		status = "Inactive"
	case 3:
		status = "Out of Stock"
	}

	return ProductDTO{
		ID:           row.ProductID,
		UUID:         row.ProductUuid.String(),
		Name:         row.ProductName,
		Slug:         row.ProductSlug,
		Description:  row.ProductDescription,
		Price:        utils.NumericToFloat(row.ProductPrice),
		Stock:        0,
		SKU:          row.ProductSku,
		CategoryID:   row.ProductCategoryID,
		CategoryUUID: row.CategoryUuid.String(),
		CategoryName: row.CategoryName,
		ImageURL:     row.ProductImageUrl,
		Status:       status,
		AvgRating:    utils.NumericToFloat(row.ProductAvgRating),
		ReviewCount:  row.ProductReviewCount,
		CreatedAt:    row.ProductCreatedAt.Format(time.RFC3339),
		UpdatedAt:    row.ProductUpdatedAt.Format(time.RFC3339),
	}
}
