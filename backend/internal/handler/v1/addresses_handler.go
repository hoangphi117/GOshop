package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/middleware"
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

// CreateUserAddress godoc
// @Summary      Create user address
// @Description  Add a new shipping address to user profile
// @Tags         Addresses
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body v1dto.CreateAddressInput true "Address payload"
// @Success      201 {object} utils.APIResponse{data=v1dto.AddressDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /addresses [post]
func (ah *AddressHandler) CreateUserAddress(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var input v1dto.CreateAddressInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	addressDTO, err := ah.service.CreateUserAddress(ctx.Request.Context(), userUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Address created successfully", addressDTO)
}

// GetMyAddresses godoc
// @Summary      Get user addresses
// @Description  Retrieve all shipping addresses of current user
// @Tags         Addresses
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} utils.APIResponse{data=[]v1dto.AddressDTO}
// @Failure      401 {object} utils.ErrorResponse
// @Router       /addresses [get]
func (ah *AddressHandler) GetMyAddresses(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	addresses, err := ah.service.GetMyAddresses(ctx.Request.Context(), userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Addresses retrieved successfully", addresses)
}

// GetAddressByUUID godoc
// @Summary      Get address by UUID
// @Description  Retrieve single user address by UUID
// @Tags         Addresses
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Address UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.AddressDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /addresses/{uuid} [get]
func (ah *AddressHandler) GetAddressByUUID(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

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

	addressDTO, err := ah.service.GetAddressByUUID(ctx.Request.Context(), userUUID, addressUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Address retrieved successfully", addressDTO)
}

// UpdateUserAddress godoc
// @Summary      Update user address
// @Description  Update shipping address details by UUID
// @Tags         Addresses
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Address UUID"
// @Param        body body v1dto.UpdateAddressInput true "Update address payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.AddressDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /addresses/{uuid} [put]
func (ah *AddressHandler) UpdateUserAddress(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

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

	addressDTO, err := ah.service.UpdateUserAddress(ctx.Request.Context(), userUUID, addressUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Address updated successfully", addressDTO)
}

// SetDefaultUserAddress godoc
// @Summary      Set default address
// @Description  Set this address as primary default shipping address
// @Tags         Addresses
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Address UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /addresses/{uuid}/default [patch]
func (ah *AddressHandler) SetDefaultUserAddress(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

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

	if err := ah.service.SetDefaultUserAddress(ctx.Request.Context(), userUUID, addressUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Default address updated successfully", nil)
}

// DeleteUserAddress godoc
// @Summary      Delete user address
// @Description  Delete address by UUID
// @Tags         Addresses
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Address UUID"
// @Success      204 "No Content"
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /addresses/{uuid} [delete]
func (ah *AddressHandler) DeleteUserAddress(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

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

	if err := ah.service.DeleteUserAddress(ctx.Request.Context(), userUUID, addressUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseStatusCode(ctx, http.StatusNoContent)
}
