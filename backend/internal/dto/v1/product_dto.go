package v1dto

import (
	"strings"
	"time"

	"goshop/internal/db/sqlc"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ============================================================================
// 1. RESPONSE DTOs
// ============================================================================

type ProductDTO struct {
	UUID         string  `json:"uuid"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Description  string  `json:"description,omitempty"`
	Price        float64 `json:"price"`
	Stock        int32   `json:"stock"`
	SKU          string  `json:"sku,omitempty"`
	CategoryUUID string  `json:"category_uuid,omitempty"`
	CategoryName string  `json:"category_name,omitempty"`
	ImageURL     string  `json:"image_url,omitempty"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
	Rating       float64 `json:"rating"`
	ReviewsCount int32   `json:"reviews_count"`
}

// ============================================================================
// 2. REQUEST & QUERY PARAM DTOs
// ============================================================================

type CreateProductRequest struct {
	Name         string  `json:"name" binding:"required,min=2,max=200"`
	Description  string  `json:"description" binding:"omitempty,max=2000"`
	Price        float64 `json:"price" binding:"required,min_float=0.01"`
	Stock        int32   `json:"stock" binding:"omitempty,gte=0"`
	SKU          *string `json:"sku" binding:"omitempty,max=50"`
	CategoryUUID string  `json:"category_uuid" binding:"required,uuid"`
	ImageURL     *string `json:"image_url" binding:"omitempty"`
	Status       int32   `json:"status" binding:"omitempty,oneof=1 2 3"`
}

type CreateProductInput struct {
	Params       sqlc.CreateProductParams
	CategoryUUID string
}

type UpdateProductRequest struct {
	Name         *string  `json:"name" binding:"omitempty,min=2,max=200"`
	Description  *string  `json:"description" binding:"omitempty,max=2000"`
	Price        *float64 `json:"price" binding:"omitempty,gt=0"`
	Stock        *int32   `json:"stock" binding:"omitempty,gte=0"`
	SKU          *string  `json:"sku" binding:"omitempty,max=50"`
	CategoryUUID *string  `json:"category_uuid" binding:"omitempty,uuid"`
	ImageURL     *string  `json:"image_url" binding:"omitempty"`
	Status       *int32   `json:"status" binding:"omitempty,oneof=1 2 3"`
}

type UpdateProductInput struct {
	Params       sqlc.UpdateProductParams
	CategoryUUID *string
}

type GetProductsParam struct {
	Search       string  `form:"search"        binding:"omitempty,min=3,max=50,search"`
	Page         int32   `form:"page"          binding:"omitempty,gte=1"`
	Limit        int32   `form:"limit"         binding:"omitempty,gte=1,lte=500"`
	OrderBy      string  `form:"order_by"      binding:"omitempty,oneof=product_name product_price product_created_at"`
	Sort         string  `form:"sort"          binding:"omitempty,oneof=asc desc"`
	CategoryUUID string  `form:"category_uuid" binding:"omitempty,uuid"`
	MinPrice     float64 `form:"min_price"     binding:"omitempty,gte=0"`
	MaxPrice     float64 `form:"max_price"     binding:"omitempty,gtefield=MinPrice"`
	Status       int32   `form:"status"        binding:"omitempty,oneof=1 2 3"`
}

type GetProductByUuidParam struct {
	Uuid string `uri:"uuid" binding:"uuid"`
}

// ==================================
// 3. REQUEST TO SERVICE MAPPERS
// ==================================

func (input *CreateProductRequest) MapCreateToServiceInput() CreateProductInput {
	var imgURL *string
	if input.ImageURL != nil && strings.TrimSpace(*input.ImageURL) != "" {
		img := strings.TrimSpace(*input.ImageURL)
		imgURL = &img
	}

	var sku *string
	if input.SKU != nil && strings.TrimSpace(*input.SKU) != "" {
		s := strings.TrimSpace(*input.SKU)
		sku = &s
	}

	var desc *string
	if strings.TrimSpace(input.Description) != "" {
		d := strings.TrimSpace(input.Description)
		desc = &d
	}

	return CreateProductInput{
		Params: sqlc.CreateProductParams{
			ProductName:        strings.TrimSpace(input.Name),
			ProductDescription: desc,
			ProductPrice:       utils.FloatToNumeric(input.Price),
			ProductStock:       &input.Stock,
			ProductSku:         sku,
			ProductImageUrl:    imgURL,
			ProductStatus:      input.Status,
		},
		CategoryUUID: input.CategoryUUID,
	}
}

func (input *UpdateProductRequest) MapUpdateToServiceInput(productUuid uuid.UUID) UpdateProductInput {
	var imgURL *string
	if input.ImageURL != nil {
		if strings.TrimSpace(*input.ImageURL) != "" {
			img := strings.TrimSpace(*input.ImageURL)
			imgURL = &img
		}
	}

	params := sqlc.UpdateProductParams{
		ProductUuid:        productUuid,
		ProductName:        input.Name,
		ProductDescription: input.Description,
		ProductStock:       input.Stock,
		ProductSku:         input.SKU,
		ProductImageUrl:    imgURL,
		ProductStatus:      input.Status,
	}

	if input.Price != nil {
		params.ProductPrice = utils.FloatToNumeric(*input.Price)
	}

	return UpdateProductInput{
		Params:       params,
		CategoryUUID: input.CategoryUUID,
	}
}

// =================================
// MODEL TO DTO MAPPERS
// =================================

// Mapper for GET Single / List
func MapGetProductByUUIDRowToDTO(p sqlc.GetProductByUUIDRow) *ProductDTO {
	dto := mapFieldsToDTO(
		p.ProductUuid, p.ProductName, p.ProductSlug, p.ProductDescription,
		p.ProductPrice, p.ProductStock, p.ProductSku, p.CategoryUuid,
		p.CategoryName, p.ProductImageUrl, p.ProductStatus, p.ProductCreatedAt,
		p.ProductAvgRating, p.ProductReviewCount,
	)
	return &dto
}

// Mapper for CreateProduct
func MapCreateProductRowToDTO(p sqlc.CreateProductRow) *ProductDTO {
	dto := mapFieldsToDTO(
		p.ProductUuid, p.ProductName, p.ProductSlug, p.ProductDescription,
		p.ProductPrice, p.ProductStock, p.ProductSku, p.CategoryUuid,
		p.CategoryName, p.ProductImageUrl, p.ProductStatus, p.ProductCreatedAt,
		p.ProductAvgRating, p.ProductReviewCount,
	)
	return &dto
}

// Mapper for UpdateProduct
func MapUpdateProductRowToDTO(p sqlc.UpdateProductRow) *ProductDTO {
	dto := mapFieldsToDTO(
		p.ProductUuid, p.ProductName, p.ProductSlug, p.ProductDescription,
		p.ProductPrice, p.ProductStock, p.ProductSku, p.CategoryUuid,
		p.CategoryName, p.ProductImageUrl, p.ProductStatus, p.ProductCreatedAt,
		p.ProductAvgRating, p.ProductReviewCount,
	)
	return &dto
}

// Mapper for SoftDelete / Restore / Trash (no JOIN)
func MapProductToDTO(p sqlc.Product) *ProductDTO {
	dto := mapFieldsToDTO(
		p.ProductUuid, p.ProductName, p.ProductSlug, p.ProductDescription,
		p.ProductPrice, p.ProductStock, p.ProductSku, uuid.Nil,
		"", p.ProductImageUrl, p.ProductStatus, p.ProductCreatedAt,
		p.ProductAvgRating, p.ProductReviewCount,
	)
	return &dto
}

// Optimized memory allocation
func MapListProductsToDTO(products []sqlc.GetProductByUUIDRow) []ProductDTO {
	if len(products) == 0 {
		return []ProductDTO{}
	}
	dtos := make([]ProductDTO, len(products))
	for i, p := range products {
		dtos[i] = mapFieldsToDTO(
			p.ProductUuid, p.ProductName, p.ProductSlug, p.ProductDescription,
			p.ProductPrice, p.ProductStock, p.ProductSku, p.CategoryUuid,
			p.CategoryName, p.ProductImageUrl, p.ProductStatus, p.ProductCreatedAt,
			p.ProductAvgRating, p.ProductReviewCount,
		)
	}
	return dtos
}

// Private helper duy nhất map các trường chung vào ProductDTO (trả về Value để tối ưu Stack Allocation)
func mapFieldsToDTO(
	pUUID uuid.UUID, name, slug string, desc *string,
	price pgtype.Numeric, stock *int32, sku *string,
	catUUID uuid.UUID, catName string, imgURL *string,
	status int32, createdAt time.Time,
	avgRating pgtype.Numeric, reviewCount int32,
) ProductDTO {
	catUUIDStr := ""
	if catUUID != uuid.Nil {
		catUUIDStr = catUUID.String()
	}

	dto := ProductDTO{
		UUID:         pUUID.String(),
		Name:         name,
		Slug:         slug,
		Price:        utils.NumericToFloat(price),
		CategoryUUID: catUUIDStr,
		CategoryName: catName,
		Status:       mapProductStatusText(int(status)),
		CreatedAt:    createdAt.Format("2006-01-02 15:04:05"),
		Rating:       utils.NumericToFloat(avgRating),
		ReviewsCount: reviewCount,
	}

	if desc != nil {
		dto.Description = *desc
	}
	if stock != nil {
		dto.Stock = *stock
	}
	if sku != nil {
		dto.SKU = *sku
	}
	if imgURL != nil {
		dto.ImageURL = *imgURL
	}

	return dto
}

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
