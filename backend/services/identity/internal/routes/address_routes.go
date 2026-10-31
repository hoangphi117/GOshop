package routes

import (
	"goshop/services/identity/internal/handler"

	"github.com/gin-gonic/gin"
)

type AddressRoutes struct {
	handler *handler.AddressHandler
}

func NewAddressRoutes(handler *handler.AddressHandler) *AddressRoutes {
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
