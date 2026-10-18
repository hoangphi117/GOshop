package routes

import (
	"strings"
	"time"

	"goshop/pkg/cache"
	"goshop/pkg/middleware"
	"goshop/pkg/utils"
	"goshop/services/identity/internal/handler"

	"github.com/gin-gonic/gin"
)

type AuthRoutes struct {
	handler      *handler.AuthHandler
	cacheService cache.RedisCacheService
}

func NewAuthRoutes(handler *handler.AuthHandler, cacheService cache.RedisCacheService) *AuthRoutes {
	return &AuthRoutes{
		handler:      handler,
		cacheService: cacheService,
	}
}

func (ar *AuthRoutes) Register(r *gin.RouterGroup) {
	auth := r.Group("/auth")
	{
		// Direct registration is strictly restricted to test/local/development environments to prevent bypassing OTP verification in production
		appEnv := strings.ToLower(utils.GetEnv("APP_ENV", "development"))
		if appEnv != "production" && appEnv != "prod" {
			auth.POST("/register", middleware.RateLimit(ar.cacheService, "register", 15, 5*time.Minute), ar.handler.Register)
		} else {
			auth.POST("/register", func(ctx *gin.Context) {
				utils.ResponseError(ctx, utils.NewError(utils.ErrCodeForbidden, "Direct registration is disabled in production. Please use OTP verification flow (/api/v1/auth/register-otp/send)."))
			})
		}
		auth.POST("/register-otp/send", middleware.RateLimit(ar.cacheService, "register_otp_send", 10, 15*time.Minute), ar.handler.SendRegisterOTP)
		auth.POST("/register-otp/verify", middleware.RateLimit(ar.cacheService, "register_otp_verify", 15, 5*time.Minute), ar.handler.VerifyRegisterOTP)
		auth.POST("/register-otp/resend", middleware.RateLimit(ar.cacheService, "register_otp_resend", 5, 15*time.Minute), ar.handler.ResendRegisterOTP)
		auth.POST("/login", middleware.RateLimit(ar.cacheService, "login", 15, 5*time.Minute), ar.handler.Login)
		auth.POST("/google-login", middleware.RateLimit(ar.cacheService, "google_login", 10, time.Minute), ar.handler.GoogleLogin)
		auth.POST("/logout", ar.handler.Logout)
		auth.POST("/refresh", middleware.RateLimit(ar.cacheService, "refresh", 30, time.Minute), ar.handler.RefreshToken)
		auth.POST("/forgot-password", middleware.RateLimit(ar.cacheService, "forgot_password", 3, 15*time.Minute), ar.handler.RequestForgotPassword)
		auth.POST("/reset-password", middleware.RateLimit(ar.cacheService, "reset_password", 5, 15*time.Minute), ar.handler.ResetPassword)
	}
}
