package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type ReviewRoutes struct {
	handler *v1handler.ReviewHandler
}

func NewReviewRoutes(handler *v1handler.ReviewHandler) *ReviewRoutes {
	return &ReviewRoutes{
		handler: handler,
	}
}

func (rr *ReviewRoutes) Register(r *gin.RouterGroup) {
	reviews := r.Group("/reviews")
	{
		reviews.POST("", rr.handler.CreateReview)
		reviews.GET("/me", rr.handler.GetMyReviews)
		reviews.PUT("/:uuid", rr.handler.UpdateReview)
		reviews.DELETE("/:uuid", rr.handler.DeleteReview)
	}
}
