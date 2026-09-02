package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AddressHandler struct {
	service v1service.AddressService
}

func NewAddressHandler(service v1service.AddressService) *AddressHandler {
	return &AddressHandler{
		service: service,
	}
}

func (ah *AddressHandler) CreateUserAddress(ctx *gin.Context) {
	var input v1dto.CreateAddressInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	addressDTO, err := ah.service.CreateUserAddress(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Address created successfully", addressDTO)
}
