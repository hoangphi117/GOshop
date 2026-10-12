package utils

import (
	"strconv"
)

type Pagination struct {
	Page         int32 `json:"page"`
	Limit        int32 `json:"limit"`
	TotalRecords int32 `json:"total_records"`
	TotalPages   int32 `json:"total_pages"`
	HasNext      bool  `json:"has_next"`
	HasPrev      bool  `json:"has_prev"`
}

func NewPagination(page, limit, totalRecords int32) *Pagination {

	if page <= 0 {
		page = 1
	}

	if limit <= 0 {
		envLimit := GetEnv("DEFAULT_LIMIT", "10")
		limitInt, err := strconv.Atoi(envLimit)
		if err != nil || limitInt <= 0 {
			limitInt = 10
		}

		limit = int32(limitInt)
	}

	totalPages := (totalRecords + limit - 1) / limit

	if totalPages < 1 {
		totalPages = 1
	}

	return &Pagination{
		Page:         page,
		Limit:        limit,
		TotalRecords: totalRecords,
		TotalPages:   totalPages,
		HasNext:      page < totalPages,
		HasPrev:      page > 1,
	}
}

func NewPaginationResponse(data any, page, limit, totalRecords int32) map[string]any {

	return map[string]any{
		"data":       data,
		"pagination": NewPagination(page, limit, totalRecords),
	}
}
