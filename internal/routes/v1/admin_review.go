package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type AdminReviewRoutes struct {
	handler *v1handler.ReviewHandler
}

func NewAdminReviewRoutes(handler *v1handler.ReviewHandler) *AdminReviewRoutes {
	return &AdminReviewRoutes{
		handler: handler,
	}
}

func (ur *AdminReviewRoutes) Register(r *gin.RouterGroup) {
	reviews := r.Group("/reviews")
	{
		reviews.GET("", ur.handler.AdminGetAllReviews)
		reviews.PATCH("/:uuid/status", ur.handler.UpdateReviewStatus)
		reviews.PATCH("/:uuid/reply", ur.handler.AdminReplyReview)
	}
}
