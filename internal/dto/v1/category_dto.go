package v1dto

import "goshop/internal/db/sqlc"

type CategoryDTO struct {
	UUID        string `json:"uuid"`
	Name        string `json:"category_name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

type CreateCategoryInput struct {
	Name        string `json:"category_name" binding:"required,min=2,max=100"`
	Description string `json:"description" binding:"omitempty,max=500"`
	Status      int32  `json:"status" binding:"omitempty,oneof=1 2"`
}

func (input *CreateCategoryInput) MapCreateInputToModel() sqlc.CreateCategoryParams {
	return sqlc.CreateCategoryParams{
		CategoryName:        input.Name,
		CategoryDescription: input.Description,
		CategoryStatus:      input.Status,
	}
}

func MapCategoryToDTO(category sqlc.Category) CategoryDTO {
	return CategoryDTO{
		UUID:        category.CategoryUuid.String(),
		Name:        category.CategoryName,
		Slug:        category.CategorySlug,
		Description: category.CategoryDescription,
		Status:      mapCategoryStatusText(int(category.CategoryStatus)),
		CreatedAt:   category.CategoryCreatedAt.Format("2006-01-02 15:04:05"),
	}
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
