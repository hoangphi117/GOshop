package v1dto

import (
	"time"

	"goshop/internal/db/sqlc"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type ProductDTO struct {
	UUID         string  `json:"uuid"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Description  string  `json:"description,omitempty"`
	Price        float64 `json:"price"`
	Stock        int32   `json:"stock"`
	SKU          string  `json:"sku,omitempty"`
	CategoryUUID string  `json:"category_uuid"`
	ImageURL     string  `json:"image_url,omitempty"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
}

type CreateProductInput struct {
	Params       sqlc.CreateProductParams
	CategoryUUID string
}

type CreateProductRequest struct {
	Name         string  `json:"name" binding:"required,min=2,max=200"`
	Description  string  `json:"description" binding:"omitempty,max=2000"`
	Price        float64 `json:"price" binding:"required,min_float=0.01"`
	Stock        int32   `json:"stock" binding:"omitempty,gte=0"`
	SKU          *string `json:"sku" binding:"omitempty,max=50"`
	CategoryUUID string  `json:"category_uuid" binding:"required,uuid"`
	ImageURL     *string `json:"image_url" binding:"omitempty,url"`
	Status       int32   `json:"status" binding:"omitempty,oneof=1 2 3"`
}

type UpdateProductRequest struct {
	Name         *string  `json:"name" binding:"omitempty,min=2,max=200"`
	Description  *string  `json:"description" binding:"omitempty,max=2000"`
	Price        *float64 `json:"price" binding:"omitempty,gt=0"`
	Stock        *int32   `json:"stock" binding:"omitempty,gte=0"`
	SKU          *string  `json:"sku" binding:"omitempty,max=50"`
	CategoryUUID *string  `json:"category_uuid" binding:"omitempty,uuid"`
	ImageURL     *string  `json:"image_url" binding:"omitempty,url"`
	Status       *int32   `json:"status" binding:"omitempty,oneof=1 2 3"`
}

type UpdateProductInput struct {
	Params       sqlc.UpdateProductParams
	CategoryUUID *string
}

type GetProductByUuidParam struct {
	Uuid string `uri:"uuid" binding:"uuid"`
}

func (input *CreateProductRequest) MapCreateToServiceInput() CreateProductInput {
	params := sqlc.CreateProductParams{
		ProductName:        input.Name,
		ProductDescription: &input.Description,
		ProductStock:       &input.Stock,
		ProductSku:         input.SKU,
		ProductImageUrl:    input.ImageURL,
		ProductStatus:      input.Status,
	}
	if price, err := utils.FloatToNumeric(input.Price); err == nil {
		params.ProductPrice = price
	}
	return CreateProductInput{
		Params:       params,
		CategoryUUID: input.CategoryUUID,
	}
}

func (input *UpdateProductRequest) MapUpdateToServiceInput(productUuid uuid.UUID) UpdateProductInput {
	params := sqlc.UpdateProductParams{
		ProductUuid:        productUuid,
		ProductName:        input.Name,
		ProductDescription: input.Description,
		ProductStock:       input.Stock,
		ProductSku:         input.SKU,
		ProductImageUrl:    input.ImageURL,
		ProductStatus:      input.Status,
	}

	if input.Price != nil {
		price, err := utils.FloatToNumeric(*input.Price)
		if err == nil {
			params.ProductPrice = price
		}
	}
	return UpdateProductInput{
		Params:       params,
		CategoryUUID: input.CategoryUUID,
	}
}

// MapGetProductByUUIDRowToDTO maps GetProductByUUIDRow (used by GET endpoints) to ProductDTO.
func MapGetProductByUUIDRowToDTO(product sqlc.GetProductByUUIDRow) *ProductDTO {
	return mapToProductDTO(
		product.ProductUuid.String(),
		product.ProductName,
		product.ProductSlug,
		product.ProductDescription,
		product.ProductPrice,
		product.ProductStock,
		product.ProductSku,
		product.CategoryUuid.String(),
		product.ProductImageUrl,
		product.ProductStatus,
		product.ProductCreatedAt,
	)
}

// MapCreateProductRowToDTO maps CreateProductRow (used by POST create endpoint) to ProductDTO.
func MapCreateProductRowToDTO(product sqlc.CreateProductRow) *ProductDTO {
	return mapToProductDTO(
		product.ProductUuid.String(),
		product.ProductName,
		product.ProductSlug,
		product.ProductDescription,
		product.ProductPrice,
		product.ProductStock,
		product.ProductSku,
		product.CategoryUuid.String(),
		product.ProductImageUrl,
		product.ProductStatus,
		product.ProductCreatedAt,
	)
}

func MapUpdateProductRowToDTO(product sqlc.UpdateProductRow) *ProductDTO {
	return mapToProductDTO(
		product.ProductUuid.String(),
		product.ProductName,
		product.ProductSlug,
		product.ProductDescription,
		product.ProductPrice,
		product.ProductStock,
		product.ProductSku,
		product.CategoryUuid.String(),
		product.ProductImageUrl,
		product.ProductStatus,
		product.ProductCreatedAt,
	)
}

// mapToProductDTO là private helper chứa logic mapping chung cho mọi product row.
func mapToProductDTO(
	uuid, name, slug string,
	description *string,
	price pgtype.Numeric,
	stock *int32,
	sku *string,
	categoryUUID string,
	imageURL *string,
	status int32,
	createdAt time.Time,
) *ProductDTO {
	dto := &ProductDTO{
		UUID:         uuid,
		Name:         name,
		Slug:         slug,
		Price:        utils.NumericToFloat(price),
		CategoryUUID: categoryUUID,
		Status:       mapProductStatusText(int(status)),
		CreatedAt:    createdAt.Format("2006-01-02 15:04:05"),
	}
	if description != nil {
		dto.Description = *description
	}
	if stock != nil {
		dto.Stock = *stock
	}
	if sku != nil {
		dto.SKU = *sku
	}
	if imageURL != nil {
		dto.ImageURL = *imageURL
	}
	return dto
}

// func MapListProductsToDTO(products []sqlc.Product) []ProductDTO {
// 	dtos := make([]ProductDTO, 0, len(products))
// 	for _, product := range products {
// 		dtos = append(dtos, *MapProductToDTO(product))
// 	}
// 	return dtos
// }

func mapProductStatusText(status int) string {
	switch status {
	case 1:
		return "Active"
	case 2:
		return "Inactive"
	case 3:
		return "Out of Stock"
	default:
		return "None"
	}
}
