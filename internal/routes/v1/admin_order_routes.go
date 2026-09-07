package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type AdminOrderRoutes struct {
	handler *v1handler.OrderHandler
}

func NewAdminOrderRoutes(handler *v1handler.OrderHandler) *AdminOrderRoutes {
	return &AdminOrderRoutes{
		handler: handler,
	}
}

func (or *AdminOrderRoutes) Register(r *gin.RouterGroup) {
	orders := r.Group("/orders")
	{
		orders.GET("", or.handler.GetAllOrders)
		orders.GET("/:uuid", or.handler.GetOrderByUUID)
		orders.PATCH("/:uuid", or.handler.UpdateOrderStatus)
		orders.PATCH("/:uuid/payment-status", or.handler.UpdateOrderPaymentStatus)

		// Shipping
		orders.POST("/:uuid/shipping", or.handler.CreateShipping)
		orders.POST("/:uuid/shipping/cancel", or.handler.CancelShipping)
		orders.GET("/:uuid/shipping/print-bill", or.handler.GetPrintBill)
		orders.POST("/:uuid/refund", or.handler.AdminProcessReturnRefund) // POST /api/v1/admin/orders/:uuid/refund
		orders.POST("/:uuid/sync-tracking", or.handler.SyncOrderTracking) // POST /api/v1/admin/orders/:uuid/sync-tracking
	}

	returns := r.Group("/returns")
	{
		returns.GET("", or.handler.AdminGetReturns)                        // GET /api/v1/admin/returns
		returns.GET("/:uuid", or.handler.AdminGetReturnDetail)             // GET /api/v1/admin/returns/:uuid
		returns.PATCH("/:uuid", or.handler.AdminUpdateReturnStatus)        // PATCH /api/v1/admin/returns/:uuid
		returns.POST("/:uuid/refund", or.handler.AdminProcessReturnRefund) // POST /api/v1/admin/returns/:uuid/refund
	}
}
