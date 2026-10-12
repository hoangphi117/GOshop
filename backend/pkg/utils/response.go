package utils

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorCode string

type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"error"`
	Err     error     `json:"detail,omitempty"`
}

type ErrorResponse struct {
	Error  string    `json:"error" example:"Invalid input format"`
	Code   ErrorCode `json:"code" example:"BAD_REQUEST"`
	Detail string    `json:"detail,omitempty" example:"Detailed explanation"`
}

type APIResponse struct {
	Status     string      `json:"status" example:"success"`
	Message    string      `json:"message,omitempty" example:"Operation completed successfully"`
	Data       any         `json:"data,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

const (
	ErrCodeBadRequest      ErrorCode = "BAD_REQUEST"
	ErrCodeNotFound        ErrorCode = "NOT_FOUND"
	ErrCodeConflict        ErrorCode = "CONFLICT"
	ErrCodeInternal        ErrorCode = "INTERNAL_SERVER_ERROR"
	ErrCodeUnauthorized    ErrorCode = "UNAUTHORIZED"
	ErrCodeTooManyRequests ErrorCode = "TOO_MANY_REQUESTS"
	ErrCodeForbidden       ErrorCode = "FORBIDDEN"
	ErrCodeBadGateway         ErrorCode = "BAD_GATEWAY"
	ErrCodeExternalService    ErrorCode = "EXTERNAL_SERVICE_ERROR"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
)

func (ae *AppError) Error() string {
	if ae.Err != nil {
		return fmt.Sprintf("%s: %v", ae.Message, ae.Err)
	}
	return ae.Message
}

func (ae *AppError) Unwrap() error {
	return ae.Err
}

func NewError(code ErrorCode, message string) error {
	return &AppError{
		Message: message,
		Code:    code,
	}
}

func WrapError(code ErrorCode, message string, err error) error {
	return &AppError{
		Message: message,
		Code:    code,
		Err:     err,
	}
}

func ResponseSuccess(ctx *gin.Context, status int, message string, data ...any) {
	resp := APIResponse{
		Status:  "success",
		Message: CapitalizeFirstLetter(message),
	}

	if len(data) > 0 && data[0] != nil {
		if m, ok := data[0].(map[string]any); ok {
			if p, exists := m["pagination"]; exists {
				if pag, ok := p.(*Pagination); ok {
					resp.Pagination = pag
				}
			}

			if d, exists := m["data"]; exists {
				resp.Data = d
			} else {
				resp.Data = m
			}
		} else {
			resp.Data = data[0]
		}
	}

	ctx.JSON(status, resp)
}

func ResponseStatusCode(ctx *gin.Context, status int) {
	ctx.Status(status)
}

func ResponseError(ctx *gin.Context, err error) {
	if appErr, ok := err.(*AppError); ok {
		status := httpStatusFromCode(appErr.Code)
		response := gin.H{
			"error": appErr.Message,
			"code":  appErr.Code,
		}

		if appErr.Err != nil {
			response["detail"] = appErr.Err.Error()
		}

		ctx.JSON(status, response)
		return
	}
	ctx.JSON(http.StatusInternalServerError, gin.H{
		"error": err.Error(),
		"code":  ErrCodeInternal,
	})
}

func ResponseValidator(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusBadRequest, data)

}

func httpStatusFromCode(code ErrorCode) int {
	switch code {
	case ErrCodeBadRequest:
		return http.StatusBadRequest
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeConflict:
		return http.StatusConflict
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeTooManyRequests:
		return http.StatusTooManyRequests
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeBadGateway:
		return http.StatusBadGateway
	case ErrCodeExternalService:
		return http.StatusBadGateway
	case ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
