package v1handler

import (
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserHandler struct {
	service v1service.UserService
}

func NewUserHandler(service v1service.UserService) *UserHandler {
	return &UserHandler{
		service: service,
	}
}

func (uh *UserHandler) GetAllUsers(ctx *gin.Context) {

	var params v1dto.GetUserParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	listUser, total, err := uh.service.GetAllUsers(ctx, params.Search, params.OrderBy, params.Sort, params.Page, params.Limit, false)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	listUserDTO := v1dto.MapListUsersToDTO(listUser)

	paginationResponse := utils.NewPaginationResponse(listUserDTO, params.Page, params.Limit, total)

	utils.ResponseSuccess(ctx, http.StatusOK, "Get users successfully", paginationResponse)
}

func (uh *UserHandler) CreateUsers(ctx *gin.Context) {
	var input v1dto.CreateUserInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	user := input.MapCreateInputToModel()

	createdUser, err := uh.service.CreateUsers(ctx, user)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	createdUserDTO := v1dto.MapUserToDTO(createdUser)

	utils.ResponseSuccess(ctx, http.StatusCreated, "User created successfully", createdUserDTO)
}

func (uh *UserHandler) GetUserByUUID(ctx *gin.Context) {
	var params v1dto.GetUserByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	user, err := uh.service.GetUserByUUID(ctx, userUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	userDTO := v1dto.MapUserToDTO(user)

	utils.ResponseSuccess(ctx, http.StatusOK, "Get user successfully", userDTO)
}

func (uh *UserHandler) UpdateUser(ctx *gin.Context) {
	var params v1dto.GetUserByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var input v1dto.UpdateUserInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	user := input.MapUpdateInputToModel(userUuid)

	updatedUser, err := uh.service.UpdateUser(ctx, user)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	updatedUserDTO := v1dto.MapUserToDTO(updatedUser)

	utils.ResponseSuccess(ctx, http.StatusOK, "User updated successfully", updatedUserDTO)
}

func (uh *UserHandler) ChangePassword(ctx *gin.Context) {
	var input v1dto.ChangePasswordRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	err := uh.service.ChangePassword(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Password changed successfully", nil)
}

func (uh *UserHandler) GetProfile(ctx *gin.Context) {
	user, err := uh.service.GetProfile(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	userDTO := v1dto.MapUserToDTO(user)

	utils.ResponseSuccess(ctx, http.StatusOK, "Get profile successfully", userDTO)
}

func (uh *UserHandler) UpdateProfile(ctx *gin.Context) {
	var input v1dto.UpdateProfileInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	params := sqlc.UpdateUserParams{
		UserFullname: input.Name,
		UserAge:      input.Age,
		UserPassword: input.Password,
	}

	updatedUser, err := uh.service.UpdateProfile(ctx, params)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	updatedUserDTO := v1dto.MapUserToDTO(updatedUser)

	utils.ResponseSuccess(ctx, http.StatusOK, "Profile updated successfully", updatedUserDTO)
}

func (uh *UserHandler) SoftDeleteUser(ctx *gin.Context) {
	var params v1dto.GetUserByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	softDeletedUser, err := uh.service.SoftDeleteUser(ctx, userUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	softDeletedUserDTO := v1dto.MapUserToDTO(softDeletedUser)

	utils.ResponseSuccess(ctx, http.StatusOK, "User soft deleted successfully", softDeletedUserDTO)
}

func (uh *UserHandler) RestoreUser(ctx *gin.Context) {
	var params v1dto.GetUserByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	restoredUser, err := uh.service.RestoreUser(ctx, userUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	restoredUserDTO := v1dto.MapUserToDTO(restoredUser)

	utils.ResponseSuccess(ctx, http.StatusOK, "User restored successfully", restoredUserDTO)
}

func (uh *UserHandler) TrashUser(ctx *gin.Context) {
	var params v1dto.GetUserByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	err = uh.service.TrashUser(ctx, userUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseStatusCode(ctx, http.StatusNoContent)
}

func (uh *UserHandler) GetSoftDeletedUsers(ctx *gin.Context) {
	var params v1dto.GetUserParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	listUser, total, err := uh.service.GetAllUsers(ctx, params.Search, params.OrderBy, params.Sort, params.Page, params.Limit, true)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	listUserDTO := v1dto.MapListUsersToDTO(listUser)

	paginationResponse := utils.NewPaginationResponse(listUserDTO, params.Page, params.Limit, total)

	utils.ResponseSuccess(ctx, http.StatusOK, "Get soft deleted users successfully", paginationResponse)
}
