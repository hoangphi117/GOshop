package service

import (
	"context"
	catalogsqlc "goshop/services/catalog/internal/db/sqlc"
	"goshop/services/catalog/internal/dto"
	"goshop/shared/pkg/utils"

	"github.com/google/uuid"
)

// GetCategoryTree constructs a nested hierarchy of all active categories.
func (s *catalogService) GetCategoryTree(ctx context.Context) ([]dto.CategoryDTO, error) {
	cats, err := s.repo.GetCategoriesWithParent(ctx)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get categories for tree", err)
	}

	// Map of categoryID -> CategoryDTO
	dtoMap := make(map[int32]*dto.CategoryDTO, len(cats))
	for _, c := range cats {
		d := dto.MapCategoryToDTO(c)
		d.Children = []dto.CategoryDTO{}
		dtoMap[c.CategoryID] = &d
	}

	var rootCategories []dto.CategoryDTO
	for _, c := range cats {
		curr := dtoMap[c.CategoryID]
		if c.ParentID != nil && *c.ParentID > 0 {
			if parent, exists := dtoMap[*c.ParentID]; exists {
				parent.Children = append(parent.Children, *curr)
				continue
			}
		}
	}

	for _, c := range cats {
		if c.ParentID == nil || *c.ParentID == 0 {
			rootCategories = append(rootCategories, *dtoMap[c.CategoryID])
		}
	}

	return rootCategories, nil
}

// Helper to assign parent to category on create/update
func (s *catalogService) assignCategoryParent(ctx context.Context, catUUID uuid.UUID, parentUUID *string, parentID *int32) {
	var pid *int32
	level := int32(1)

	if parentUUID != nil && *parentUUID != "" {
		pUUID, err := uuid.Parse(*parentUUID)
		if err == nil {
			parentCat, pErr := s.repo.GetCategoryByUUID(ctx, pUUID)
			if pErr == nil {
				pid = &parentCat.CategoryID
				level = parentCat.CategoryLevel + 1
			}
		}
	} else if parentID != nil && *parentID > 0 {
		parentCat, pErr := s.repo.GetCategoryByID(ctx, *parentID)
		if pErr == nil {
			pid = &parentCat.CategoryID
			level = parentCat.CategoryLevel + 1
		}
	}

	if pid != nil {
		_ = s.repo.UpdateCategoryParent(ctx, catUUID, pid, level)
	}
}

// GetCategoryFacets aggregates product specifications and subcategories for dynamic faceted filtering.
func (s *catalogService) GetCategoryFacets(ctx context.Context, categorySlugOrUUID string) (*dto.CategoryFacetsResponseDTO, error) {
	var cat catalogsqlc.Category
	var err error

	if u, parseErr := uuid.Parse(categorySlugOrUUID); parseErr == nil {
		cat, err = s.repo.GetCategoryByUUID(ctx, u)
	} else {
		cat, err = s.repo.GetCategoryBySlug(ctx, categorySlugOrUUID)
	}
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeNotFound, "category not found", err)
	}

	catDTO := dto.MapCategoryToDTO(cat)

	facetRows, subRows, err := s.repo.GetCategoryFacets(ctx, cat.CategoryUuid)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to get category facets", err)
	}

	// Group facet rows by Key
	facetGroupMap := make(map[string][]dto.FacetOptionDTO)
	var orderedKeys []string
	for _, row := range facetRows {
		if _, exists := facetGroupMap[row.Key]; !exists {
			orderedKeys = append(orderedKeys, row.Key)
		}
		facetGroupMap[row.Key] = append(facetGroupMap[row.Key], dto.FacetOptionDTO{
			Value: row.Value,
			Count: row.Count,
		})
	}

	facetGroups := make([]dto.FacetGroupDTO, 0, len(orderedKeys))
	for _, k := range orderedKeys {
		facetGroups = append(facetGroups, dto.FacetGroupDTO{
			Key:     k,
			Options: facetGroupMap[k],
		})
	}

	subDTOs := make([]dto.SubcategoryCountDTO, len(subRows))
	for i, sub := range subRows {
		subDTOs[i] = dto.SubcategoryCountDTO{
			CategoryID:   sub.CategoryID,
			UUID:         sub.CategoryUUID.String(),
			Name:         sub.CategoryName,
			Slug:         sub.CategorySlug,
			ProductCount: sub.ProductCount,
		}
	}

	return &dto.CategoryFacetsResponseDTO{
		Category:      catDTO,
		Subcategories: subDTOs,
		Facets:        facetGroups,
	}, nil
}
