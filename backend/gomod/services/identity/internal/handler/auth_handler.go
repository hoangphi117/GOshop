package handler

import (
	"net/http"
	"os"
	"strings"

	dto "goshop/services/identity/internal/dto"
	"goshop/services/identity/internal/service"
	"goshop/shared/pkg/auth"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	service service.AuthService
}

func NewAuthHandler(service service.AuthService) *AuthHandler {
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
// @Param        request body dto.RegisterInput true "Registration credentials"
// @Success      201 {object} utils.APIResponse "Register successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      409 {object} utils.ErrorResponse "Email already registered"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/register [post]
func (ah *AuthHandler) Register(ctx *gin.Context) {
	var input dto.RegisterInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if err := ah.service.Register(ctx.Request.Context(), input); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Register successfully")
}

// SendRegisterOTP godoc
// @Summary      Send registration verification OTP
// @Description  Validate registration input, hash password, and send 6-digit OTP code to email
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body dto.SendRegisterOTPInput true "Registration credentials"
// @Success      200 {object} utils.APIResponse "Verification code sent to email"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      409 {object} utils.ErrorResponse "Email already registered"
// @Failure      429 {object} utils.ErrorResponse "Cooldown active"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/register-otp/send [post]
func (ah *AuthHandler) SendRegisterOTP(ctx *gin.Context) {
	var input dto.SendRegisterOTPInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if err := ah.service.SendRegisterOTP(ctx.Request.Context(), input); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Verification code sent to email")
}

// VerifyRegisterOTP godoc
// @Summary      Verify registration OTP and create account
// @Description  Verify the 6-digit OTP code sent to user email and return access/refresh token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body dto.VerifyRegisterOTPInput true "Verification payload"
// @Success      201 {object} utils.APIResponse{data=dto.LoginResponse} "Registration verified successfully"
// @Failure      400 {object} utils.ErrorResponse "Invalid or expired OTP"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/register-otp/verify [post]
func (ah *AuthHandler) VerifyRegisterOTP(ctx *gin.Context) {
	var input dto.VerifyRegisterOTPInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	accessToken, refreshToken, expiresIn, err := ah.service.VerifyRegisterOTP(ctx.Request.Context(), input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	cookieSecure := os.Getenv("APP_ENV") == "production"
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("refresh_token", refreshToken, int(auth.RefreshTokenTTL.Seconds()), "/", "", cookieSecure, true)

	response := dto.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Registration verified successfully", response)
}

// ResendRegisterOTP godoc
// @Summary      Resend registration verification OTP
// @Description  Resend a new 6-digit OTP code to the user email if cooldown elapsed
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body dto.ResendRegisterOTPInput true "Resend payload"
// @Success      200 {object} utils.APIResponse "Verification code resent successfully"
// @Failure      400 {object} utils.ErrorResponse "No pending registration found"
// @Failure      429 {object} utils.ErrorResponse "Cooldown active"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/register-otp/resend [post]
func (ah *AuthHandler) ResendRegisterOTP(ctx *gin.Context) {
	var input dto.ResendRegisterOTPInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if err := ah.service.ResendRegisterOTP(ctx.Request.Context(), input); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Verification code resent successfully")
}

// Login godoc
// @Summary      User login
// @Description  Authenticate user with email and password, returns access token and refresh token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body dto.LoginInput true "Login credentials"
// @Success      200 {object} utils.APIResponse{data=dto.LoginResponse} "Login successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      401 {object} utils.ErrorResponse "Invalid email or password"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/login [post]
func (ah *AuthHandler) Login(ctx *gin.Context) {
	var input dto.LoginInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	ip := ctx.ClientIP()
	if ip == "" {
		ip = ctx.Request.RemoteAddr
	}

	userAgent := ctx.GetHeader("User-Agent")
	accessToken, refreshToken, expiresIn, err := ah.service.Login(ctx.Request.Context(), ip, userAgent, input.Email, input.Password)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	cookieSecure := os.Getenv("APP_ENV") == "production"
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("refresh_token", refreshToken, int(auth.RefreshTokenTTL.Seconds()), "/", "", cookieSecure, true)

	response := dto.LoginResponse{
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
// @Param        request body dto.RefreshTokenInput false "Refresh token"
// @Success      200 {object} utils.APIResponse "Logout successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/logout [post]
func (ah *AuthHandler) Logout(ctx *gin.Context) {
	var input dto.RefreshTokenInput
	_ = ctx.ShouldBindJSON(&input)

	refreshToken := input.RefreshToken
	if refreshToken == "" {
		if cookieVal, err := ctx.Cookie("refresh_token"); err == nil && cookieVal != "" {
			refreshToken = cookieVal
		}
	}

	cookieSecure := os.Getenv("APP_ENV") == "production"
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("refresh_token", "", -1, "/", "", cookieSecure, true)
	ctx.SetCookie("refresh_token", "", -1, "/api/v1/auth", "", cookieSecure, true)

	authHeader := ctx.GetHeader("Authorization")
	accessToken := strings.TrimPrefix(authHeader, "Bearer ")

	if refreshToken != "" || accessToken != "" {
		if err := ah.service.Logout(ctx.Request.Context(), accessToken, refreshToken); err != nil {
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
// @Param        request body dto.RefreshTokenInput false "Refresh token"
// @Success      200 {object} utils.APIResponse{data=dto.LoginResponse} "Token refreshed successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      401 {object} utils.ErrorResponse "Invalid or expired refresh token"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/refresh [post]
func (ah *AuthHandler) RefreshToken(ctx *gin.Context) {
	var input dto.RefreshTokenInput
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

	accessToken, newRefreshToken, expiresIn, err := ah.service.RefreshToken(ctx.Request.Context(), refreshToken)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	cookieSecure := os.Getenv("APP_ENV") == "production"
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie("refresh_token", newRefreshToken, int(auth.RefreshTokenTTL.Seconds()), "/", "", cookieSecure, true)

	response := dto.LoginResponse{
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
// @Param        request body dto.RequestForgotPasswordInput true "User email"
// @Success      200 {object} utils.APIResponse "Password reset link sent successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      404 {object} utils.ErrorResponse "User with this email not found"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/forgot-password [post]
func (ah *AuthHandler) RequestForgotPassword(ctx *gin.Context) {
	var input dto.RequestForgotPasswordInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if err := ah.service.RequestForgotPassword(ctx.Request.Context(), input.Email); err != nil {
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
// @Param        request body dto.ResetPasswordInput true "Token and new password"
// @Success      200 {object} utils.APIResponse "Password reset successfully"
// @Failure      400 {object} utils.ErrorResponse "Validation error or invalid token"
// @Failure      500 {object} utils.ErrorResponse "Internal server error"
// @Router       /api/v1/auth/reset-password [post]
func (ah *AuthHandler) ResetPassword(ctx *gin.Context) {
	var input dto.ResetPasswordInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	err := ah.service.ResetPassword(ctx.Request.Context(), input.Token, input.NewPassword)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Password reset successfully")
}

// GoogleLogin godoc
// @Summary      Login with Google
// @Description  Authenticate user using Google OAuth ID token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        request body dto.GoogleLoginInput true "Google ID token"
// @Success      200 {object} utils.APIResponse "Login with Google successful"
// @Failure      400 {object} utils.ErrorResponse "Validation error"
// @Failure      401 {object} utils.ErrorResponse "Unauthorized"
// @Router       /api/v1/auth/google-login [post]
func (ah *AuthHandler) GoogleLogin(ctx *gin.Context) {
	var input dto.GoogleLoginInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	ip := ctx.ClientIP()
	if ip == "" {
		ip = ctx.Request.RemoteAddr
	}
	userAgent := ctx.GetHeader("User-Agent")
	accessToken, refreshToken, expiresIn, err := ah.service.GoogleLogin(ctx.Request.Context(), ip, userAgent, input.IDToken)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	cookieSecure := os.Getenv("APP_ENV") == "production"
	ctx.SetSameSite(http.SameSiteLaxMode)
	ctx.SetCookie(
		"refresh_token",
		refreshToken,
		int(auth.RefreshTokenTTL.Seconds()),
		"/",
		"",
		cookieSecure,
		true,
	)

	utils.ResponseSuccess(ctx, http.StatusOK, "Login with Google successful", gin.H{
		"access_token": accessToken,
		"expires_in":   expiresIn,
	})
}
