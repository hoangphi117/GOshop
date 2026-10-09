package v1routes

import (
	"time"

	v1handler "goshop/internal/handler/v1"
	"goshop/internal/middleware"
	"goshop/pkg/cache"

	"github.com/gin-gonic/gin"
)

type AuthRoutes struct {
	handler      *v1handler.AuthHandler
	cacheService cache.RedisCacheService
}

func NewAuthRoutes(handler *v1handler.AuthHandler, cacheService cache.RedisCacheService) *AuthRoutes {
	return &AuthRoutes{
		handler:      handler,
		cacheService: cacheService,
	}
}

func (ar *AuthRoutes) Register(r *gin.RouterGroup) {
	auth := r.Group("/auth")
	{
		auth.POST("/register", middleware.RateLimit(ar.cacheService, "register", 15, 5*time.Minute), ar.handler.Register)
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
