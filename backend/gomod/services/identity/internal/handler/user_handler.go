package handler

import (
	"net/http"

	dto "goshop/services/identity/internal/dto"
	"goshop/services/identity/internal/service"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserHandler struct {
	service service.UserService
}

func NewUserHandler(service service.UserService) *UserHandler {
	return &UserHandler{
		service: service,
	}
}

// GetAllUsers godoc
// @Summary      Get all users
// @Description  Retrieve paginated list of users (Admin/Moderator)
// @Tags         Admin - Users
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        search query string false "Search keyword"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Param        order_by query string false "Order by field" Enums(user_id, user_created_at)
// @Param        sort query string false "Sort direction (asc/desc)" Enums(asc, desc)
// @Success      200 {object} utils.APIResponse{data=[]dto.UserDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/users [get]
func (uh *UserHandler) GetAllUsers(ctx *gin.Context) {
	var params dto.GetUserParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	listUserDTO, total, err := uh.service.GetAllUsers(ctx.Request.Context(), params.Search, params.OrderBy, params.Sort, params.Page, params.Limit, false)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	paginationResponse := utils.NewPaginationResponse(listUserDTO, params.Page, params.Limit, total)
	utils.ResponseSuccess(ctx, http.StatusOK, "Get users successfully", paginationResponse)
}

// CreateUsers godoc
// @Summary      Create user
// @Description  Create a new user account (Admin/Moderator)
// @Tags         Admin - Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body dto.CreateUserInput true "User payload"
// @Success      201 {object} utils.APIResponse{data=dto.UserDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/users [post]
func (uh *UserHandler) CreateUsers(ctx *gin.Context) {
	var input dto.CreateUserInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	createdUserDTO, err := uh.service.CreateUsers(ctx.Request.Context(), input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "User created successfully", createdUserDTO)
}

// GetUserByUUID godoc
// @Summary      Get user by UUID
// @Description  Retrieve user detail by UUID (Admin/Moderator)
// @Tags         Admin - Users
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "User UUID"
// @Success      200 {object} utils.APIResponse{data=dto.UserDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/users/{uuid} [get]
func (uh *UserHandler) GetUserByUUID(ctx *gin.Context) {
	var params dto.GetUserByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	userDTO, err := uh.service.GetUserByUUID(ctx.Request.Context(), userUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Get user successfully", userDTO)
}

// UpdateUser godoc
// @Summary      Update user
// @Description  Update user profile and status by UUID (Admin/Moderator)
// @Tags         Admin - Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "User UUID"
// @Param        body body dto.UpdateUserInput true "Update user payload"
// @Success      200 {object} utils.APIResponse{data=dto.UserDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/users/{uuid} [put]
func (uh *UserHandler) UpdateUser(ctx *gin.Context) {
	var params dto.GetUserByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var input dto.UpdateUserInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	updatedUserDTO, err := uh.service.UpdateUser(ctx.Request.Context(), userUuid, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "User updated successfully", updatedUserDTO)
}

// ChangePassword godoc
// @Summary      Change password
// @Description  Change authenticated user password
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body dto.ChangePasswordRequest true "Change password payload"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /users/me/change-password [put]
func (uh *UserHandler) ChangePassword(ctx *gin.Context) {
	var input dto.ChangePasswordRequest
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUuid, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	err = uh.service.ChangePassword(ctx.Request.Context(), userUuid, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Password changed successfully", nil)
}

// GetProfile godoc
// @Summary      Get current profile
// @Description  Get current authenticated user profile
// @Tags         Users
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} utils.APIResponse{data=dto.UserDTO}
// @Failure      401 {object} utils.ErrorResponse
// @Router       /users/me [get]
func (uh *UserHandler) GetProfile(ctx *gin.Context) {
	userUuid, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	userDTO, err := uh.service.GetProfile(ctx.Request.Context(), userUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Get profile successfully", userDTO)
}

// UpdateProfile godoc
// @Summary      Update current profile
// @Description  Update current authenticated user details
// @Tags         Users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body dto.UpdateProfileInput true "Profile payload"
// @Success      200 {object} utils.APIResponse{data=dto.UserDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /users/me [put]
func (uh *UserHandler) UpdateProfile(ctx *gin.Context) {
	var input dto.UpdateProfileInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUuid, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	updatedUserDTO, err := uh.service.UpdateProfile(ctx.Request.Context(), userUuid, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Profile updated successfully", updatedUserDTO)
}

// SoftDeleteUser godoc
// @Summary      Soft delete user
// @Description  Soft delete user by UUID (Admin/Moderator)
// @Tags         Admin - Users
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "User UUID"
// @Success      200 {object} utils.APIResponse{data=dto.UserDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/users/{uuid}/soft-delete [delete]
func (uh *UserHandler) SoftDeleteUser(ctx *gin.Context) {
	var params dto.GetUserByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	softDeletedUserDTO, err := uh.service.SoftDeleteUser(ctx.Request.Context(), userUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "User soft deleted successfully", softDeletedUserDTO)
}

// RestoreUser godoc
// @Summary      Restore user
// @Description  Restore a soft-deleted user by UUID (Admin/Moderator)
// @Tags         Admin - Users
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "User UUID"
// @Success      200 {object} utils.APIResponse{data=dto.UserDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/users/{uuid}/restore [put]
func (uh *UserHandler) RestoreUser(ctx *gin.Context) {
	var params dto.GetUserByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	restoredUserDTO, err := uh.service.RestoreUser(ctx.Request.Context(), userUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "User restored successfully", restoredUserDTO)
}

// TrashUser godoc
// @Summary      Permanently delete user
// @Description  Permanently delete a user from database (Admin only)
// @Tags         Admin - Users
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "User UUID"
// @Success      204 "No Content"
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/users/{uuid}/trash [delete]
func (uh *UserHandler) TrashUser(ctx *gin.Context) {
	var params dto.GetUserByUuidParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUuid, err := uuid.Parse(params.Uuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	err = uh.service.TrashUser(ctx.Request.Context(), userUuid)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseStatusCode(ctx, http.StatusNoContent)
}

// GetSoftDeletedUsers godoc
// @Summary      Get soft-deleted users
// @Description  Retrieve paginated list of soft-deleted users (Admin/Moderator)
// @Tags         Admin - Users
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        search query string false "Search keyword"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Param        order_by query string false "Order by field" Enums(user_id, user_created_at)
// @Param        sort query string false "Sort direction (asc/desc)" Enums(asc, desc)
// @Success      200 {object} utils.APIResponse{data=[]dto.UserDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/users/soft-deleted [get]
func (uh *UserHandler) GetSoftDeletedUsers(ctx *gin.Context) {
	var params dto.GetUserParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	listUserDTO, total, err := uh.service.GetAllUsers(ctx.Request.Context(), params.Search, params.OrderBy, params.Sort, params.Page, params.Limit, true)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	paginationResponse := utils.NewPaginationResponse(listUserDTO, params.Page, params.Limit, total)
	utils.ResponseSuccess(ctx, http.StatusOK, "Get soft deleted users successfully", paginationResponse)
}
