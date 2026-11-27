package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	catalogdto "goshop/services/catalog/internal/dto"
	cataloghandler "goshop/services/catalog/internal/handler"
	catalogservice "goshop/services/catalog/internal/service"
	"goshop/shared/pkg/utils"
)

type mockCatalogService struct {
	catalogservice.CatalogService
	createCatFn    func(ctx context.Context, input catalogdto.CreateCategoryInput) (*catalogdto.CategoryDTO, error)
	getCatUUIDFn   func(ctx context.Context, u uuid.UUID) (*catalogdto.CategoryDTO, error)
	getCatSlugFn   func(ctx context.Context, slug string) (*catalogdto.CategoryDTO, error)
	getAllCatsFn   func(ctx context.Context, activeOnly *bool) ([]catalogdto.CategoryDTO, error)
	getCatTreeFn   func(ctx context.Context) ([]catalogdto.CategoryDTO, error)
	updateCatFn    func(ctx context.Context, input catalogdto.UpdateCategoryInput) (*catalogdto.CategoryDTO, error)
	softDelCatFn   func(ctx context.Context, u uuid.UUID) (*catalogdto.CategoryDTO, error)
	restoreCatFn   func(ctx context.Context, u uuid.UUID) (*catalogdto.CategoryDTO, error)
	trashCatFn     func(ctx context.Context, u uuid.UUID) error
	getFacetsFn    func(ctx context.Context, slugOrUUID string) (*catalogdto.CategoryFacetsResponseDTO, error)
	listFAQsFn     func(ctx context.Context, q *string, limit, offset int32) ([]catalogdto.FAQDTO, error)
	createProdFn   func(ctx context.Context, input catalogdto.CreateProductInput) (*catalogdto.ProductDTO, error)
	getProdUUIDFn  func(ctx context.Context, u uuid.UUID) (*catalogdto.ProductDTO, error)
	getProdSlugFn  func(ctx context.Context, slug string) (*catalogdto.ProductDTO, error)
	getAllProdsFn  func(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, orderBy, sort string, page, limit int32, deleted bool, specs map[string][]string) ([]catalogdto.ProductDTO, int32, error)
	updateProdFn   func(ctx context.Context, input catalogdto.UpdateProductInput) (*catalogdto.ProductDTO, error)
	softDelProdFn  func(ctx context.Context, u uuid.UUID) (*catalogdto.ProductDTO, error)
	listVarsFn     func(ctx context.Context, productUUID uuid.UUID, activeOnly bool) ([]catalogdto.ProductVariantDTO, error)
	createVarFn    func(ctx context.Context, productUUID uuid.UUID, input catalogdto.CreateVariantInput) (*catalogdto.ProductVariantDTO, error)
	updateVarFn    func(ctx context.Context, productUUID, variantUUID uuid.UUID, input catalogdto.UpdateVariantInput) (*catalogdto.ProductVariantDTO, error)
	deleteVarFn    func(ctx context.Context, productUUID, variantUUID uuid.UUID) error
}

func (m *mockCatalogService) CreateCategory(ctx context.Context, input catalogdto.CreateCategoryInput) (*catalogdto.CategoryDTO, error) {
	if m.createCatFn != nil {
		return m.createCatFn(ctx, input)
	}
	return &catalogdto.CategoryDTO{UUID: uuid.New().String(), Name: input.Name, Slug: "cat-slug"}, nil
}

func (m *mockCatalogService) GetCategoryByUUID(ctx context.Context, u uuid.UUID) (*catalogdto.CategoryDTO, error) {
	if m.getCatUUIDFn != nil {
		return m.getCatUUIDFn(ctx, u)
	}
	return &catalogdto.CategoryDTO{UUID: u.String(), Name: "Electronics", Slug: "electronics"}, nil
}

func (m *mockCatalogService) GetCategoryBySlug(ctx context.Context, slug string) (*catalogdto.CategoryDTO, error) {
	if m.getCatSlugFn != nil {
		return m.getCatSlugFn(ctx, slug)
	}
	return &catalogdto.CategoryDTO{UUID: uuid.New().String(), Name: "Electronics", Slug: slug}, nil
}

func (m *mockCatalogService) GetAllCategories(ctx context.Context, activeOnly *bool) ([]catalogdto.CategoryDTO, error) {
	if m.getAllCatsFn != nil {
		return m.getAllCatsFn(ctx, activeOnly)
	}
	return []catalogdto.CategoryDTO{{UUID: uuid.New().String(), Name: "Cat1"}}, nil
}

func (m *mockCatalogService) GetCategoryTree(ctx context.Context) ([]catalogdto.CategoryDTO, error) {
	if m.getCatTreeFn != nil {
		return m.getCatTreeFn(ctx)
	}
	return []catalogdto.CategoryDTO{{UUID: uuid.New().String(), Name: "TreeCat"}}, nil
}

func (m *mockCatalogService) UpdateCategory(ctx context.Context, input catalogdto.UpdateCategoryInput) (*catalogdto.CategoryDTO, error) {
	if m.updateCatFn != nil {
		return m.updateCatFn(ctx, input)
	}
	if _, err := uuid.Parse(input.UUID); err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "invalid category UUID")
	}
	name := "UpdatedCat"
	if input.Name != nil {
		name = *input.Name
	}
	return &catalogdto.CategoryDTO{UUID: input.UUID, Name: name}, nil
}

func (m *mockCatalogService) SoftDeleteCategory(ctx context.Context, u uuid.UUID) (*catalogdto.CategoryDTO, error) {
	if m.softDelCatFn != nil {
		return m.softDelCatFn(ctx, u)
	}
	return &catalogdto.CategoryDTO{UUID: u.String(), Status: "Deleted"}, nil
}

func (m *mockCatalogService) RestoreCategory(ctx context.Context, u uuid.UUID) (*catalogdto.CategoryDTO, error) {
	if m.restoreCatFn != nil {
		return m.restoreCatFn(ctx, u)
	}
	return &catalogdto.CategoryDTO{UUID: u.String(), Status: "Active"}, nil
}

func (m *mockCatalogService) TrashCategory(ctx context.Context, u uuid.UUID) error {
	if m.trashCatFn != nil {
		return m.trashCatFn(ctx, u)
	}
	return nil
}

func (m *mockCatalogService) GetCategoryFacets(ctx context.Context, slugOrUUID string) (*catalogdto.CategoryFacetsResponseDTO, error) {
	if m.getFacetsFn != nil {
		return m.getFacetsFn(ctx, slugOrUUID)
	}
	return &catalogdto.CategoryFacetsResponseDTO{Category: catalogdto.CategoryDTO{UUID: uuid.New().String()}}, nil
}

func (m *mockCatalogService) ListFAQs(ctx context.Context, q *string, limit, offset int32) ([]catalogdto.FAQDTO, error) {
	if m.listFAQsFn != nil {
		return m.listFAQsFn(ctx, q, limit, offset)
	}
	return []catalogdto.FAQDTO{{UUID: uuid.New().String(), Question: "How to buy?", Answer: "Click purchase."}}, nil
}

func (m *mockCatalogService) CreateProduct(ctx context.Context, input catalogdto.CreateProductInput) (*catalogdto.ProductDTO, error) {
	if m.createProdFn != nil {
		return m.createProdFn(ctx, input)
	}
	return &catalogdto.ProductDTO{UUID: uuid.New().String(), Name: input.Name, Price: input.Price}, nil
}

func (m *mockCatalogService) GetProductByUUID(ctx context.Context, u uuid.UUID) (*catalogdto.ProductDTO, error) {
	if m.getProdUUIDFn != nil {
		return m.getProdUUIDFn(ctx, u)
	}
	return &catalogdto.ProductDTO{UUID: u.String(), Name: "Laptop", Price: 999.0}, nil
}

func (m *mockCatalogService) GetProductBySlug(ctx context.Context, slug string) (*catalogdto.ProductDTO, error) {
	if m.getProdSlugFn != nil {
		return m.getProdSlugFn(ctx, slug)
	}
	return &catalogdto.ProductDTO{UUID: uuid.New().String(), Name: "Laptop", Slug: slug}, nil
}

func (m *mockCatalogService) GetAllProducts(ctx context.Context, search, categoryUUID string, minPrice, maxPrice, minRating float64, status int32, orderBy, sort string, page, limit int32, deleted bool, specs map[string][]string) ([]catalogdto.ProductDTO, int32, error) {
	if m.getAllProdsFn != nil {
		return m.getAllProdsFn(ctx, search, categoryUUID, minPrice, maxPrice, minRating, status, orderBy, sort, page, limit, deleted, specs)
	}
	return []catalogdto.ProductDTO{{UUID: uuid.New().String(), Name: "Prod1", Price: 100.0}}, 1, nil
}

func (m *mockCatalogService) UpdateProduct(ctx context.Context, input catalogdto.UpdateProductInput) (*catalogdto.ProductDTO, error) {
	if m.updateProdFn != nil {
		return m.updateProdFn(ctx, input)
	}
	if _, err := uuid.Parse(input.UUID); err != nil {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "invalid product UUID")
	}
	return &catalogdto.ProductDTO{UUID: input.UUID, Name: "UpdatedProd"}, nil
}

func (m *mockCatalogService) SoftDeleteProduct(ctx context.Context, u uuid.UUID) (*catalogdto.ProductDTO, error) {
	if m.softDelProdFn != nil {
		return m.softDelProdFn(ctx, u)
	}
	return &catalogdto.ProductDTO{UUID: u.String(), Status: "Deleted"}, nil
}

func (m *mockCatalogService) GetVariantsByProduct(ctx context.Context, productUUID uuid.UUID, activeOnly bool) ([]catalogdto.ProductVariantDTO, error) {
	if m.listVarsFn != nil {
		return m.listVarsFn(ctx, productUUID, activeOnly)
	}
	return []catalogdto.ProductVariantDTO{{UUID: uuid.New().String(), SKU: "SKU-1", Title: "Blue"}}, nil
}

func (m *mockCatalogService) CreateVariant(ctx context.Context, productUUID uuid.UUID, input catalogdto.CreateVariantInput) (*catalogdto.ProductVariantDTO, error) {
	if m.createVarFn != nil {
		return m.createVarFn(ctx, productUUID, input)
	}
	return &catalogdto.ProductVariantDTO{UUID: uuid.New().String(), SKU: input.SKU, Title: input.Title, Price: input.Price}, nil
}

func (m *mockCatalogService) UpdateVariant(ctx context.Context, productUUID, variantUUID uuid.UUID, input catalogdto.UpdateVariantInput) (*catalogdto.ProductVariantDTO, error) {
	if m.updateVarFn != nil {
		return m.updateVarFn(ctx, productUUID, variantUUID, input)
	}
	return &catalogdto.ProductVariantDTO{UUID: variantUUID.String(), SKU: "SKU-UPDATED"}, nil
}

func (m *mockCatalogService) DeleteVariant(ctx context.Context, productUUID, variantUUID uuid.UUID) error {
	if m.deleteVarFn != nil {
		return m.deleteVarFn(ctx, productUUID, variantUUID)
	}
	return nil
}

func setupCatalogRouter(h *cataloghandler.CatalogHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Categories
	r.POST("/admin/categories", h.CreateCategory)
	r.GET("/categories", h.GetAllCategories)
	r.GET("/categories/tree", h.GetCategoryTree)
	r.GET("/categories/:slug", h.GetCategoryBySlug)
	r.GET("/categories/uuid/:uuid", h.GetCategoryByUUID)
	r.PUT("/admin/categories/:uuid", h.UpdateCategory)
	r.DELETE("/admin/categories/:uuid", h.SoftDeleteCategory)
	r.PUT("/admin/categories/:uuid/restore", h.RestoreCategory)
	r.DELETE("/admin/categories/:uuid/trash", h.TrashCategory)
	r.GET("/categories/:slug/facets", h.GetCategoryFacets)

	// FAQs
	r.GET("/faqs", h.ListFAQs)
	r.GET("/faqs/search", h.SearchFAQs)

	// Products
	r.POST("/admin/products", h.CreateProduct)
	r.GET("/products/:uuid", h.GetProductByUUID)
	r.GET("/products/slug/:slug", h.GetProductBySlug)
	r.GET("/products", h.GetAllProducts)
	r.PUT("/admin/products/:uuid", h.UpdateProduct)
	r.DELETE("/admin/products/:uuid", h.SoftDeleteProduct)

	// Variants
	r.GET("/products/:uuid/variants", h.ListVariants)
	r.POST("/admin/products/:uuid/variants", h.CreateVariant)
	r.PUT("/admin/products/:uuid/variants/:variantUUID", h.UpdateVariant)
	r.DELETE("/admin/products/:uuid/variants/:variantUUID", h.DeleteVariant)

	return r
}

func TestCatalogHandler_Categories(t *testing.T) {
	mockSvc := &mockCatalogService{}
	r := setupCatalogRouter(cataloghandler.NewCatalogHandler(mockSvc))

	t.Run("CreateCategory success", func(t *testing.T) {
		body := `{"category_name":"Electronics","description":"Gadgets"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/categories", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "Category created successfully")
	})

	t.Run("CreateCategory missing name returns 400", func(t *testing.T) {
		body := `{"description":"no name"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/categories", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("CreateCategory service error", func(t *testing.T) {
		errSvc := &mockCatalogService{
			createCatFn: func(ctx context.Context, input catalogdto.CreateCategoryInput) (*catalogdto.CategoryDTO, error) {
				return nil, errors.New("service failure")
			},
		}
		errR := setupCatalogRouter(cataloghandler.NewCatalogHandler(errSvc))
		body := `{"category_name":"FailCategory"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/categories", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		errR.ServeHTTP(w, req)
		assert.NotEqual(t, http.StatusCreated, w.Code)
	})

	t.Run("GetAllCategories returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/categories?active_only=true", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Categories retrieved successfully")
	})

	t.Run("GetCategoryTree returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/categories/tree", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "TreeCat")
	})

	t.Run("GetCategoryBySlug with slug returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/categories/electronics", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "electronics")
	})

	t.Run("GetCategoryBySlug with UUID returns 200", func(t *testing.T) {
		catUUID := uuid.New().String()
		req := httptest.NewRequest(http.MethodGet, "/categories/"+catUUID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("GetCategoryByUUID valid and invalid", func(t *testing.T) {
		reqOK := httptest.NewRequest(http.MethodGet, "/categories/uuid/"+uuid.New().String(), nil)
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodGet, "/categories/uuid/invalid-uuid", nil)
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("UpdateCategory valid and invalid", func(t *testing.T) {
		u := uuid.New().String()
		body := `{"category_name":"Updated Name"}`
		reqOK := httptest.NewRequest(http.MethodPut, "/admin/categories/"+u, bytes.NewReader([]byte(body)))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodPut, "/admin/categories/bad-uuid", bytes.NewReader([]byte(body)))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("DeleteCategory, RestoreCategory, TrashCategory", func(t *testing.T) {
		u := uuid.New().String()
		// Soft delete
		reqDel := httptest.NewRequest(http.MethodDelete, "/admin/categories/"+u, nil)
		wDel := httptest.NewRecorder()
		r.ServeHTTP(wDel, reqDel)
		assert.Equal(t, http.StatusOK, wDel.Code)

		// Restore
		reqRes := httptest.NewRequest(http.MethodPut, "/admin/categories/"+u+"/restore", nil)
		wRes := httptest.NewRecorder()
		r.ServeHTTP(wRes, reqRes)
		assert.Equal(t, http.StatusOK, wRes.Code)

		// Trash
		reqTrash := httptest.NewRequest(http.MethodDelete, "/admin/categories/"+u+"/trash", nil)
		wTrash := httptest.NewRecorder()
		r.ServeHTTP(wTrash, reqTrash)
		assert.Equal(t, http.StatusOK, wTrash.Code)
	})

	t.Run("GetCategoryFacets returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/categories/electronics/facets", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Category facets retrieved successfully")
	})
}

func TestCatalogHandler_FAQs(t *testing.T) {
	mockSvc := &mockCatalogService{}
	r := setupCatalogRouter(cataloghandler.NewCatalogHandler(mockSvc))

	t.Run("ListFAQs returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/faqs?limit=10&offset=0", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "How to buy?")
	})

	t.Run("SearchFAQs with query returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/faqs/search?q=buy&limit=5", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "How to buy?")
	})
}

func TestCatalogHandler_Products(t *testing.T) {
	mockSvc := &mockCatalogService{}
	r := setupCatalogRouter(cataloghandler.NewCatalogHandler(mockSvc))

	t.Run("CreateProduct success", func(t *testing.T) {
		catUUID := uuid.New().String()
		body := `{"category_uuid":"` + catUUID + `","name":"iPhone 15","price":999.99,"stock":10}`
		req := httptest.NewRequest(http.MethodPost, "/admin/products", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Body.String(), "Product created successfully")
	})

	t.Run("CreateProduct missing category returns 400", func(t *testing.T) {
		body := `{"name":"iPhone 15","price":999.99,"stock":10}`
		req := httptest.NewRequest(http.MethodPost, "/admin/products", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("GetProduct by UUID valid and invalid", func(t *testing.T) {
		pUUID := uuid.New().String()
		reqOK := httptest.NewRequest(http.MethodGet, "/products/"+pUUID, nil)
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodGet, "/products/invalid-uuid", nil)
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("GetProductBySlug returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products/slug/macbook-pro", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "macbook-pro")
	})

	t.Run("GetAllProducts returns 200 with pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/products?page=1&limit=10&search=lap", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "Products retrieved successfully")
	})

	t.Run("UpdateProduct valid and invalid", func(t *testing.T) {
		pUUID := uuid.New().String()
		body := `{"name":"MacBook Air M3"}`
		reqOK := httptest.NewRequest(http.MethodPut, "/admin/products/"+pUUID, bytes.NewReader([]byte(body)))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodPut, "/admin/products/bad-uuid", bytes.NewReader([]byte(body)))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("DeleteProduct valid and invalid", func(t *testing.T) {
		pUUID := uuid.New().String()
		reqOK := httptest.NewRequest(http.MethodDelete, "/admin/products/"+pUUID, nil)
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodDelete, "/admin/products/bad-uuid", nil)
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})
}

func TestCatalogHandler_Variants(t *testing.T) {
	mockSvc := &mockCatalogService{}
	r := setupCatalogRouter(cataloghandler.NewCatalogHandler(mockSvc))

	pUUID := uuid.New().String()
	vUUID := uuid.New().String()

	t.Run("ListVariants valid and invalid product UUID", func(t *testing.T) {
		reqOK := httptest.NewRequest(http.MethodGet, "/products/"+pUUID+"/variants", nil)
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodGet, "/products/invalid-uuid/variants", nil)
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("CreateVariant valid and invalid", func(t *testing.T) {
		input := catalogdto.CreateVariantInput{
			SKU:   "SKU-100",
			Title: "Space Gray",
			Price: 199.99,
		}
		raw, _ := json.Marshal(input)
		reqOK := httptest.NewRequest(http.MethodPost, "/admin/products/"+pUUID+"/variants", bytes.NewReader(raw))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusCreated, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodPost, "/admin/products/bad-uuid/variants", bytes.NewReader(raw))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("UpdateVariant valid and invalid", func(t *testing.T) {
		title := "Silver"
		input := catalogdto.UpdateVariantInput{Title: &title}
		raw, _ := json.Marshal(input)
		reqOK := httptest.NewRequest(http.MethodPut, "/admin/products/"+pUUID+"/variants/"+vUUID, bytes.NewReader(raw))
		reqOK.Header.Set("Content-Type", "application/json")
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodPut, "/admin/products/"+pUUID+"/variants/bad-uuid", bytes.NewReader(raw))
		reqBad.Header.Set("Content-Type", "application/json")
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})

	t.Run("DeleteVariant valid and invalid", func(t *testing.T) {
		reqOK := httptest.NewRequest(http.MethodDelete, "/admin/products/"+pUUID+"/variants/"+vUUID, nil)
		wOK := httptest.NewRecorder()
		r.ServeHTTP(wOK, reqOK)
		assert.Equal(t, http.StatusOK, wOK.Code)

		reqBad := httptest.NewRequest(http.MethodDelete, "/admin/products/"+pUUID+"/variants/bad-uuid", nil)
		wBad := httptest.NewRecorder()
		r.ServeHTTP(wBad, reqBad)
		assert.Equal(t, http.StatusBadRequest, wBad.Code)
	})
}
