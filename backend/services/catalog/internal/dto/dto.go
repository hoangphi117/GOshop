package dto

import (
	"encoding/json"
	"goshop/internal/utils"
	catalogsqlc "goshop/services/catalog/internal/db/sqlc"
	"time"
)

type CategoryDTO struct {
	CategoryID  int32   `json:"category_id"`
	UUID        string  `json:"uuid"`
	Name        string  `json:"category_name"`
	Slug        string  `json:"slug"`
	Description *string `json:"description,omitempty"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at,omitempty"`
}

func (c CategoryDTO) MarshalJSON() ([]byte, error) {
	type Alias CategoryDTO
	return json.Marshal(&struct {
		Alias
		NameAlias       string `json:"name,omitempty"`
		CategoryIdAlias int32  `json:"categoryId,omitempty"`
		CreatedAtAlias  string `json:"createdAt,omitempty"`
	}{
		Alias:           Alias(c),
		NameAlias:       c.Name,
		CategoryIdAlias: c.CategoryID,
		CreatedAtAlias:  c.CreatedAt,
	})
}

type CreateCategoryInput struct {
	Name        string  `json:"category_name"`
	AltName     string  `json:"name"`
	Description *string `json:"description"`
	Status      int32   `json:"status" binding:"omitempty,oneof=1 2"`
}

type UpdateCategoryInput struct {
	UUID        string
	Name        *string `json:"category_name"`
	AltName     *string `json:"name"`
	Description *string `json:"description"`
	Status      *int32  `json:"status" binding:"omitempty,oneof=1 2"`
}

type ProductDTO struct {
	ID           int32   `json:"id,omitempty"`
	UUID         string  `json:"uuid"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Description  *string `json:"description,omitempty"`
	Price        float64 `json:"price"`
	Stock        int32   `json:"stock"`
	SKU          *string `json:"sku,omitempty"`
	CategoryID   int32   `json:"category_id,omitempty"`
	CategoryUUID string  `json:"category_uuid,omitempty"`
	CategoryName string  `json:"category_name,omitempty"`
	ImageURL     *string `json:"image_url,omitempty"`
	Status       string  `json:"status"`
	AvgRating    float64 `json:"rating"`
	ReviewCount  int32   `json:"reviews_count"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at,omitempty"`
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
	CategoryUUID string  `json:"category_uuid"`
	AltCatUUID   string  `json:"categoryUuid"`
	Name         string  `json:"name" binding:"required,min=2,max=200"`
	Description  *string `json:"description"`
	Price        float64 `json:"price" binding:"required,gt=0"`
	Stock        int32   `json:"stock" binding:"gte=0"`
	SKU          *string `json:"sku"`
	ImageURL     *string `json:"image_url"`
	AltImageURL  *string `json:"imageUrl"`
	Status       int32   `json:"status" binding:"omitempty,oneof=1 2 3"`
}

type UpdateProductInput struct {
	UUID         string
	CategoryUUID *string  `json:"category_uuid"`
	AltCatUUID   *string  `json:"categoryUuid"`
	Name         *string  `json:"name" binding:"omitempty,min=2,max=200"`
	Description  *string  `json:"description"`
	Price        *float64 `json:"price" binding:"omitempty,gt=0"`
	Stock        *int32   `json:"stock" binding:"omitempty,gte=0"`
	SKU          *string  `json:"sku"`
	ImageURL     *string  `json:"image_url"`
	AltImageURL  *string  `json:"imageUrl"`
	Status       *int32   `json:"status" binding:"omitempty,oneof=1 2 3"`
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

func MapCategoryToDTO(c catalogsqlc.Category) CategoryDTO {
	status := "Active"
	if c.CategoryStatus == 2 {
		status = "Inactive"
	}
	return CategoryDTO{
		CategoryID:  c.CategoryID,
		UUID:        c.CategoryUuid.String(),
		Name:        c.CategoryName,
		Slug:        c.CategorySlug,
		Description: c.CategoryDescription,
		Status:      status,
		CreatedAt:   c.CategoryCreatedAt.Format(time.RFC3339),
		UpdatedAt:   c.CategoryUpdatedAt.Format(time.RFC3339),
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

	stock := int32(0)
	if row.ProductStock != nil {
		stock = *row.ProductStock
	}

	return ProductDTO{
		ID:           row.ProductID,
		UUID:         row.ProductUuid.String(),
		Name:         row.ProductName,
		Slug:         row.ProductSlug,
		Description:  row.ProductDescription,
		Price:        utils.NumericToFloat(row.ProductPrice),
		Stock:        stock,
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

	stock := int32(0)
	if row.ProductStock != nil {
		stock = *row.ProductStock
	}

	return ProductDTO{
		ID:           row.ProductID,
		UUID:         row.ProductUuid.String(),
		Name:         row.ProductName,
		Slug:         row.ProductSlug,
		Description:  row.ProductDescription,
		Price:        utils.NumericToFloat(row.ProductPrice),
		Stock:        stock,
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

	stock := int32(0)
	if row.ProductStock != nil {
		stock = *row.ProductStock
	}

	return ProductDTO{
		ID:           row.ProductID,
		UUID:         row.ProductUuid.String(),
		Name:         row.ProductName,
		Slug:         row.ProductSlug,
		Description:  row.ProductDescription,
		Price:        utils.NumericToFloat(row.ProductPrice),
		Stock:        stock,
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
