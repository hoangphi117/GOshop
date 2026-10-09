package v1handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	v1dto "goshop/internal/dto/v1"
	v1handler "goshop/internal/handler/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
	validation.InitValidator()
}

func TestAuthHandler_SendRegisterOTP(t *testing.T) {
	t.Run("Returns 400 when body is invalid", func(t *testing.T) {
		mockSvc := &testutils.MockAuthService{}
		handler := v1handler.NewAuthHandler(mockSvc)

		r := gin.New()
		r.POST("/api/v1/auth/register-otp/send", handler.SendRegisterOTP)

		body := []byte(`{"email": "invalid-email"}`)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register-otp/send", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Returns 200 on successful OTP send", func(t *testing.T) {
		mockSvc := &testutils.MockAuthService{
			SendRegisterOTPFunc: func(ctx context.Context, input v1dto.SendRegisterOTPInput) error {
				return nil
			},
		}
		handler := v1handler.NewAuthHandler(mockSvc)

		r := gin.New()
		r.POST("/api/v1/auth/register-otp/send", handler.SendRegisterOTP)

		input := v1dto.SendRegisterOTPInput{
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

func TestAuthHandler_VerifyRegisterOTP(t *testing.T) {
	t.Run("Returns 400 when OTP length is not 6", func(t *testing.T) {
		mockSvc := &testutils.MockAuthService{}
		handler := v1handler.NewAuthHandler(mockSvc)

		r := gin.New()
		r.POST("/api/v1/auth/register-otp/verify", handler.VerifyRegisterOTP)

		body := []byte(`{"email": "test@example.com", "otp": "123"}`)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register-otp/verify", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Returns 201 with tokens on valid verification", func(t *testing.T) {
		mockSvc := &testutils.MockAuthService{
			VerifyRegisterOTPFunc: func(ctx context.Context, input v1dto.VerifyRegisterOTPInput) (string, string, int, error) {
				return "access-token-123", "refresh-token-456", 900, nil
			},
		}
		handler := v1handler.NewAuthHandler(mockSvc)

		r := gin.New()
		r.POST("/api/v1/auth/register-otp/verify", handler.VerifyRegisterOTP)

		input := v1dto.VerifyRegisterOTPInput{
			Email: "test@example.com",
			OTP:   "123456",
		}
		body, _ := json.Marshal(input)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register-otp/verify", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		var resp utils.APIResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Registration verified successfully", resp.Message)
	})
}

func TestAuthHandler_ResendRegisterOTP(t *testing.T) {
	t.Run("Returns 200 on successful resend", func(t *testing.T) {
		mockSvc := &testutils.MockAuthService{
			ResendRegisterOTPFunc: func(ctx context.Context, input v1dto.ResendRegisterOTPInput) error {
				return nil
			},
		}
		handler := v1handler.NewAuthHandler(mockSvc)

		r := gin.New()
		r.POST("/api/v1/auth/register-otp/resend", handler.ResendRegisterOTP)

		input := v1dto.ResendRegisterOTPInput{
			Email: "test@example.com",
		}
		body, _ := json.Marshal(input)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register-otp/resend", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
