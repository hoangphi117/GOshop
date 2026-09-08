package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service v1service.AuthService
}

func NewAuthHandler(service v1service.AuthService) *AuthHandler {
	return &AuthHandler{
		service: service,
	}
}

func (ah *AuthHandler) Register(ctx *gin.Context) {
	var input v1dto.RegisterInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if err := ah.service.Register(ctx, input); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Register successfully")
}

func (ah *AuthHandler) Login(ctx *gin.Context) {
	var input v1dto.LoginInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	accessToken, refreshToken, expiresIn, err := ah.service.Login(ctx, input.Email, input.Password)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	response := v1dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Login successfully", response)

}

func (ah *AuthHandler) Logout(ctx *gin.Context) {
	var input v1dto.RefreshTokenInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if err := ah.service.Logout(ctx, input.RefreshToken); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Logout successfully")

}

func (ah *AuthHandler) RefreshToken(ctx *gin.Context) {
	var input v1dto.RefreshTokenInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	accessToken, refreshToken, expiresIn, err := ah.service.RefreshToken(ctx, input.RefreshToken)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	response := v1dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Token refreshed successfully", response)
}

func (ah *AuthHandler) RequestForgotPassword(ctx *gin.Context) {
	var input v1dto.RequestForgotPasswordInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if err := ah.service.RequestForgotPassword(ctx, input.Email); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Password reset link sent successfully")
}

func (ah *AuthHandler) ResetPassword(ctx *gin.Context) {
	var input v1dto.ResetPasswordInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	err := ah.service.ResetPassword(ctx, input.Token, input.NewPassword)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Password reset successfully")
}
