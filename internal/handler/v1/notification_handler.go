package v1handler

import (
	"net/http"

	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"

	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	notificationService v1service.NotificationService
}

func NewNotificationHandler(service v1service.NotificationService) *NotificationHandler {
	return &NotificationHandler{notificationService: service}
}

func (h *NotificationHandler) CreateNotification(ctx *gin.Context) {
	var input v1dto.CreateNotificationInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	res, err := h.notificationService.CreateNotification(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Notification created successfully", res)
}

func (h *NotificationHandler) GetMyNotifications(ctx *gin.Context) {
	var query v1dto.GetNotificationsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	res, total, err := h.notificationService.GetMyNotifications(ctx, query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	page := max(query.Page, 1)

	limit := query.Limit
	if limit < 1 {
		limit = 10
	}

	paginationResponse := utils.NewPaginationResponse(res, page, limit, total)
	utils.ResponseSuccess(ctx, http.StatusOK, "Notifications retrieved successfully", paginationResponse)
}

func (h *NotificationHandler) GetUnreadCount(ctx *gin.Context) {
	res, err := h.notificationService.GetUnreadNotificationCount(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Unread notification count retrieved successfully", res)
}
