package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"
	"os"

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

// Register godoc
// @Summary      Register a new account
// @Description  Create a new user account with email, password, name and phone
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body v1dto.RegisterInput true "Registration credentials"
// @Success      201 {object} utils.APIResponse "Register successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      409 {object} utils.ErrorResponse "Email already registered"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/register [post]
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

// Login godoc
// @Summary      User login
// @Description  Authenticate user with email and password, returns access token and refresh token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body v1dto.LoginInput true "Login credentials"
// @Success      200 {object} utils.APIResponse{data=v1dto.LoginResponse} "Login successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      401 {object} utils.ErrorResponse "Invalid email or password"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/login [post]
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

	cookieSecure := os.Getenv("APP_ENV") == "production"
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("refresh_token", refreshToken, 86400, "/api/v1/auth", "", cookieSecure, true)

	response := v1dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Login successfully", response)

}

// Logout godoc
// @Summary      User logout
// @Description  Revoke user's refresh token session
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body v1dto.RefreshTokenInput false "Refresh token"
// @Success      200 {object} utils.APIResponse "Logout successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/logout [post]
func (ah *AuthHandler) Logout(ctx *gin.Context) {
	var input v1dto.RefreshTokenInput
	_ = ctx.ShouldBindJSON(&input)

	refreshToken := input.RefreshToken
	if refreshToken == "" {
		if cookieVal, err := ctx.Cookie("refresh_token"); err == nil && cookieVal != "" {
			refreshToken = cookieVal
		}
	}

	cookieSecure := os.Getenv("APP_ENV") == "production"
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("refresh_token", "", -1, "/api/v1/auth", "", cookieSecure, true)

	if refreshToken != "" {
		if err := ah.service.Logout(ctx, refreshToken); err != nil {
			utils.ResponseError(ctx, err)
			return
		}
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Logout successfully")

}

// RefreshToken godoc
// @Summary      Refresh access token
// @Description  Generate a new access token using a valid refresh token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body v1dto.RefreshTokenInput false "Refresh token"
// @Success      200 {object} utils.APIResponse{data=v1dto.LoginResponse} "Token refreshed successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      401 {object} utils.ErrorResponse "Invalid or expired refresh token"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/refresh [post]
func (ah *AuthHandler) RefreshToken(ctx *gin.Context) {
	var input v1dto.RefreshTokenInput
	_ = ctx.ShouldBindJSON(&input)

	refreshToken := input.RefreshToken
	if refreshToken == "" {
		if cookieVal, err := ctx.Cookie("refresh_token"); err == nil && cookieVal != "" {
			refreshToken = cookieVal
		}
	}

	if refreshToken == "" {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Refresh token is required"))
		return
	}

	accessToken, newRefreshToken, expiresIn, err := ah.service.RefreshToken(ctx, refreshToken)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	cookieSecure := os.Getenv("APP_ENV") == "production"
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("refresh_token", newRefreshToken, 86400, "/api/v1/auth", "", cookieSecure, true)

	response := v1dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    expiresIn,
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Token refreshed successfully", response)
}

// RequestForgotPassword godoc
// @Summary      Request password reset
// @Description  Send password reset link to user's registered email
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body v1dto.RequestForgotPasswordInput true "User email"
// @Success      200 {object} utils.APIResponse "Password reset link sent successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      404 {object} utils.ErrorResponse "User with this email not found"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/forgot-password [post]
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

// ResetPassword godoc
// @Summary      Reset password
// @Description  Reset password using the reset token received via email
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body v1dto.ResetPasswordInput true "Token and new password"
// @Success      200 {object} utils.APIResponse "Password reset successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error or invalid token"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/reset-password [post]
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
