package routes

import (
	"goshop/services/catalog/internal/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, h *handler.CatalogHandler) {
	// Admin Categories
	adminCat := r.Group("/api/v1/admin/categories")
	{
		adminCat.GET("/:uuid", h.GetCategoryByUUID)
		adminCat.POST("", h.CreateCategory)
		adminCat.PUT("/:uuid", h.UpdateCategory)
		adminCat.DELETE("/:uuid", h.SoftDeleteCategory)
		adminCat.POST("/:uuid/restore", h.RestoreCategory)
		adminCat.DELETE("/:uuid/trash", h.TrashCategory)
	}

	// Admin Products
	adminProd := r.Group("/api/v1/admin/products")
	{
		adminProd.GET("/:uuid", h.GetProductByUUID)
		adminProd.POST("", h.CreateProduct)
		adminProd.PUT("/:uuid", h.UpdateProduct)
		adminProd.DELETE("/:uuid", h.SoftDeleteProduct)
		adminProd.POST("/:uuid/restore", h.RestoreProduct)
		adminProd.DELETE("/:uuid/trash", h.TrashProduct)
		adminProd.POST("/import-csv", h.ImportProductsCSV)
	}

	// Public Categories
	pubCat := r.Group("/api/v1/categories")
	{
		pubCat.GET("", h.GetAllCategories)
		pubCat.GET("/:slug", h.GetCategoryBySlug)
	}

	// Public Products
	pubProd := r.Group("/api/v1/products")
	{
		pubProd.GET("", h.GetAllProducts)
		// /products/search is intentionally absent: the API gateway proxies that
		// path to the recommendations service (hybrid keyword + vector search).
		// register it here and Gin matches this route before the gateway's
		// NoRoute dispatch, so search silently answers keyword-only.
		pubProd.GET("/suggest", h.SuggestProducts)
		pubProd.GET("/suggestions", h.SuggestProducts)
		pubProd.GET("/:slug", h.GetProductBySlug)
		pubProd.GET("/uuid/:uuid", h.GetProductByUUID)
	}

	// Public FAQs
	pubFAQ := r.Group("/api/v1/faqs")
	{
		pubFAQ.GET("", h.ListFAQs)
		pubFAQ.GET("/search", h.SearchFAQs)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "catalog"})
	})
}
