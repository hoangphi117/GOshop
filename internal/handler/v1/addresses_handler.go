package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func (ah *AddressHandler) UpdateUserAddress(ctx *gin.Context) {
	var param v1dto.AddressUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	addressUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid address UUID"))
		return
	}

	var input v1dto.UpdateAddressInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	addressDTO, err := ah.service.UpdateUserAddress(ctx, addressUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Address updated successfully", addressDTO)
}
