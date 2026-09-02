package v1routes

import (
	v1handler "goshop/internal/handler/v1"

	"github.com/gin-gonic/gin"
)

type AddressRoutes struct {
	handler *v1handler.AddressHandler
}

func NewAddressRoutes(handler *v1handler.AddressHandler) *AddressRoutes {
	return &AddressRoutes{
		handler: handler,
	}
}

func (ar *AddressRoutes) Register(r *gin.RouterGroup) {
	addresses := r.Group("/addresses")
	{
		addresses.POST("", ar.handler.CreateUserAddress)
		addresses.GET("", ar.handler.GetMyAddresses)
		addresses.GET("/:uuid", ar.handler.GetAddressByUUID)
		addresses.PUT("/:uuid", ar.handler.UpdateUserAddress)
		addresses.PATCH("/:uuid/default", ar.handler.SetDefaultUserAddress)
		addresses.DELETE("/:uuid", ar.handler.DeleteUserAddress)
	}
}
