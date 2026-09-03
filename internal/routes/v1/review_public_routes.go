package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type ReviewPublicRoutes struct {
	handler *v1handler.ReviewHandler
}

func NewReviewPublicRoutes(handler *v1handler.ReviewHandler) *ReviewPublicRoutes {
	return &ReviewPublicRoutes{handler: handler}
}

func (rpr *ReviewPublicRoutes) Register(r *gin.RouterGroup) {
	products := r.Group("/products")
	{
		products.GET("/:uuid/reviews", rpr.handler.GetProductReviews)
		products.GET("/:uuid/reviews/summary", rpr.handler.GetProductRatingSummary)
	}
}
