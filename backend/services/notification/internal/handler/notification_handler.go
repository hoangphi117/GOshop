package handler

import (
	"net/http"

	"goshop/pkg/middleware"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	"goshop/services/notification/internal/dto"
	"goshop/services/notification/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type NotificationHandler struct {
	notificationService service.NotificationService
}

func NewNotificationHandler(service service.NotificationService) *NotificationHandler {
	return &NotificationHandler{notificationService: service}
}

func (h *NotificationHandler) CreateNotification(ctx *gin.Context) {
	var input dto.CreateNotificationInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	res, err := h.notificationService.CreateNotification(ctx.Request.Context(), input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Notification created successfully", res)
}

func (h *NotificationHandler) BroadcastNotification(ctx *gin.Context) {
	var input dto.BroadcastNotificationInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if err := h.notificationService.BroadcastNotification(ctx.Request.Context(), input); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Broadcast notification sent successfully to all users", nil)
}

func (h *NotificationHandler) GetMyNotifications(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var query dto.GetNotificationsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	notifications, total, err := h.notificationService.GetMyNotifications(ctx.Request.Context(), userUUID, query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	page := max(query.Page, 1)
	limit := query.Limit
	if limit < 1 {
		limit = 10
	}

	paginationResponse := utils.NewPaginationResponse(notifications, page, limit, total)
	utils.ResponseSuccess(ctx, http.StatusOK, "Notifications retrieved successfully", paginationResponse)
}

func (h *NotificationHandler) GetUnreadCount(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	res, err := h.notificationService.GetUnreadNotificationCount(ctx.Request.Context(), userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Unread count retrieved successfully", res)
}

func (h *NotificationHandler) MarkNotificationAsRead(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var uri dto.NotificationUUID
	if err := ctx.ShouldBindUri(&uri); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	notifUUID, err := uuid.Parse(uri.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid notification UUID"))
		return
	}

	res, err := h.notificationService.MarkNotificationAsRead(ctx.Request.Context(), userUUID, notifUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Notification marked as read successfully", res)
}

func (h *NotificationHandler) MarkAllNotificationsAsRead(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	if err := h.notificationService.MarkAllNotificationsAsRead(ctx.Request.Context(), userUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "All notifications marked as read successfully", nil)
}

func (h *NotificationHandler) DeleteNotification(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var uri dto.NotificationUUID
	if err := ctx.ShouldBindUri(&uri); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	notifUUID, err := uuid.Parse(uri.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid notification UUID"))
		return
	}

	if err := h.notificationService.DeleteNotification(ctx.Request.Context(), userUUID, notifUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Notification deleted successfully", nil)
}

// ── Admin Handlers ──────────────────────────────────────────

func (h *NotificationHandler) GetAdminNotifications(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var query dto.AdminGetNotificationsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	notifications, total, err := h.notificationService.GetAdminNotifications(ctx.Request.Context(), userUUID, query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	page := max(query.Page, 1)
	limit := query.Limit
	if limit < 1 {
		limit = 10
	}

	paginationResponse := utils.NewPaginationResponse(notifications, page, limit, total)
	utils.ResponseSuccess(ctx, http.StatusOK, "Admin notifications retrieved successfully", paginationResponse)
}

func (h *NotificationHandler) GetAdminUnreadCount(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	res, err := h.notificationService.GetAdminUnreadNotificationCount(ctx.Request.Context(), userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Admin unread count retrieved successfully", res)
}

func (h *NotificationHandler) MarkAllAdminNotificationsAsRead(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	if err := h.notificationService.MarkAllAdminNotificationsAsRead(ctx.Request.Context(), userUUID); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "All admin notifications marked as read successfully", nil)
}
