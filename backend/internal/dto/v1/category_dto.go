package v1dto

import (
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
)

type CategoryDTO struct {
	ID          int32  `json:"category_id"`
	UUID        string `json:"uuid"`
	Name        string `json:"category_name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

type CreateCategoryInput struct {
	Name        string  `json:"category_name" binding:"required,min=2,max=100"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	Status      int32   `json:"status" binding:"omitempty,oneof=1 2"`
}

type GetCategoryParam struct {
	Search  string `form:"search" binding:"omitempty,min=3,max=50,search"`
	Page    int32  `form:"page" binding:"omitempty,gte=1"`
	Limit   int32  `form:"limit" binding:"omitempty,gte=1,lte=500"`
	OrderBy string `form:"order_by" binding:"omitempty,oneof=category_name category_created_at"`
	Sort    string `form:"sort" binding:"omitempty,oneof=asc desc"`
}

type GetCategoryByUuidParam struct {
	Uuid string `uri:"uuid" binding:"uuid"`
}

type UpdateCategoryInput struct {
	Name        *string `json:"category_name" binding:"omitempty,min=2,max=100"`
	Description *string `json:"description" binding:"omitempty,max=500"`
	Status      *int32  `json:"status" binding:"omitempty,oneof=1 2"`
}

func (input *CreateCategoryInput) MapCreateInputToModel() sqlc.CreateCategoryParams {
	return sqlc.CreateCategoryParams{
		CategoryName:        input.Name,
		CategoryDescription: input.Description,
		CategoryStatus:      input.Status,
	}
}

func (input *UpdateCategoryInput) MapUpdateInputToModel(categoryUuid uuid.UUID) sqlc.UpdateCategoryParams {
	return sqlc.UpdateCategoryParams{
		CategoryUuid:        categoryUuid,
		CategoryName:        input.Name,
		CategoryDescription: input.Description,
		CategoryStatus:      input.Status,
	}
}

func MapCategoryToDTO(category sqlc.Category) *CategoryDTO {
	dto := &CategoryDTO{
		ID:        category.CategoryID,
		UUID:      category.CategoryUuid.String(),
		Name:      category.CategoryName,
		Slug:      category.CategorySlug,
		Status:    mapCategoryStatusText(int(category.CategoryStatus)),
		CreatedAt: category.CategoryCreatedAt.Format("2006-01-02 15:04:05"),
	}

	if category.CategoryDescription != nil {
		dto.Description = *category.CategoryDescription
	}
	return dto
}

func MapListCategoriesToDTO(categories []sqlc.Category) []CategoryDTO {
	dtos := make([]CategoryDTO, 0, len(categories))
	for _, category := range categories {
		dtos = append(dtos, *MapCategoryToDTO(category))
	}
	return dtos
}

func mapCategoryStatusText(status int) string {
	switch status {
	case 1:
		return "Active"
	case 2:
		return "Inactive"
	default:
		return "None"
	}
}
