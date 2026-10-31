package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	dto "goshop/services/identity/internal/dto"
	"goshop/services/identity/internal/handler"
	"goshop/shared/pkg/validation"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type mockAuthService struct {
	registerFunc              func(ctx context.Context, input dto.RegisterInput) error
	sendRegisterOTPFunc       func(ctx context.Context, input dto.SendRegisterOTPInput) error
	verifyRegisterOTPFunc     func(ctx context.Context, input dto.VerifyRegisterOTPInput) (string, string, int, error)
	resendRegisterOTPFunc     func(ctx context.Context, input dto.ResendRegisterOTPInput) error
	loginFunc                 func(ctx context.Context, ip, userAgent string, email, password string) (string, string, int, error)
	googleLoginFunc           func(ctx context.Context, ip, userAgent, idToken string) (string, string, int, error)
	logoutFunc                func(ctx context.Context, accessToken string, refreshToken string) error
	refreshTokenFunc          func(ctx context.Context, refreshToken string) (string, string, int, error)
	requestForgotPasswordFunc func(ctx context.Context, email string) error
	resetPasswordFunc         func(ctx context.Context, token, newPassword string) error
}

func (m *mockAuthService) Register(ctx context.Context, input dto.RegisterInput) error {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, input)
	}
	return nil
}

func (m *mockAuthService) SendRegisterOTP(ctx context.Context, input dto.SendRegisterOTPInput) error {
	if m.sendRegisterOTPFunc != nil {
		return m.sendRegisterOTPFunc(ctx, input)
	}
	return nil
}

func (m *mockAuthService) VerifyRegisterOTP(ctx context.Context, input dto.VerifyRegisterOTPInput) (string, string, int, error) {
	if m.verifyRegisterOTPFunc != nil {
		return m.verifyRegisterOTPFunc(ctx, input)
	}
	return "access-token", "refresh-token", 900, nil
}

func (m *mockAuthService) ResendRegisterOTP(ctx context.Context, input dto.ResendRegisterOTPInput) error {
	if m.resendRegisterOTPFunc != nil {
		return m.resendRegisterOTPFunc(ctx, input)
	}
	return nil
}

func (m *mockAuthService) Login(ctx context.Context, ip, userAgent string, email, password string) (string, string, int, error) {
	if m.loginFunc != nil {
		return m.loginFunc(ctx, ip, userAgent, email, password)
	}
	return "access-token", "refresh-token", 900, nil
}

func (m *mockAuthService) GoogleLogin(ctx context.Context, ip, userAgent, idToken string) (string, string, int, error) {
	if m.googleLoginFunc != nil {
		return m.googleLoginFunc(ctx, ip, userAgent, idToken)
	}
	return "access-token", "refresh-token", 900, nil
}

func (m *mockAuthService) Logout(ctx context.Context, accessToken string, refreshToken string) error {
	if m.logoutFunc != nil {
		return m.logoutFunc(ctx, accessToken, refreshToken)
	}
	return nil
}

func (m *mockAuthService) RefreshToken(ctx context.Context, refreshToken string) (string, string, int, error) {
	if m.refreshTokenFunc != nil {
		return m.refreshTokenFunc(ctx, refreshToken)
	}
	return "access-token", "new-refresh-token", 900, nil
}

func (m *mockAuthService) RequestForgotPassword(ctx context.Context, email string) error {
	if m.requestForgotPasswordFunc != nil {
		return m.requestForgotPasswordFunc(ctx, email)
	}
	return nil
}

func (m *mockAuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if m.resetPasswordFunc != nil {
		return m.resetPasswordFunc(ctx, token, newPassword)
	}
	return nil
}

func init() {
	gin.SetMode(gin.TestMode)
	if err := validation.InitValidator(); err != nil {
		panic(fmt.Sprintf("failed to initialize validator: %v", err))
	}
}

func TestAuthHandler_SendRegisterOTP(t *testing.T) {
	t.Run("Returns 400 when body is invalid", func(t *testing.T) {
		mockSvc := &mockAuthService{}
		h := handler.NewAuthHandler(mockSvc)

		r := gin.New()
		r.POST("/api/v1/auth/register-otp/send", h.SendRegisterOTP)

		body := []byte(`{"email": "invalid-email"}`)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register-otp/send", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Returns 200 on successful OTP send", func(t *testing.T) {
		mockSvc := &mockAuthService{
			sendRegisterOTPFunc: func(ctx context.Context, input dto.SendRegisterOTPInput) error {
				return nil
			},
		}
		h := handler.NewAuthHandler(mockSvc)

		r := gin.New()
		r.POST("/api/v1/auth/register-otp/send", h.SendRegisterOTP)

		input := dto.SendRegisterOTPInput{
			Name:     "Test User",
			Email:    "test@example.com",
			Password: "Password123!",
		}
		body, _ := json.Marshal(input)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register-otp/send", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAuthHandler_Login(t *testing.T) {
	t.Run("Returns 200 on successful login", func(t *testing.T) {
		mockSvc := &mockAuthService{
			loginFunc: func(ctx context.Context, ip, userAgent string, email, password string) (string, string, int, error) {
				return "jwt-access-token", "jwt-refresh-token", 900, nil
			},
		}
		h := handler.NewAuthHandler(mockSvc)

		r := gin.New()
		r.POST("/api/v1/auth/login", h.Login)

		input := dto.LoginInput{
			Email:    "test@example.com",
			Password: "Password123!",
		}
		body, _ := json.Marshal(input)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
