package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type UploadRoutes struct {
	handler *v1handler.UploadHandler
}

func NewUploadRoutes(handler *v1handler.UploadHandler) *UploadRoutes {
	return &UploadRoutes{handler: handler}
}

func (ur *UploadRoutes) Register(r *gin.RouterGroup) {
	r.POST("/upload", ur.handler.UploadSingleFile) // POST /api/v1/upload
}
