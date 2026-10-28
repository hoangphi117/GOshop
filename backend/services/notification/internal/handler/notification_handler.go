package handler

import (
	"net/http"

	"goshop/pkg/middleware"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	notificationdto "goshop/services/notification/internal/dto"
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

// CreateNotification godoc
// @Summary      Create notification
// @Description  Create a notification targeted to a specific user (Admin/Moderator)
// @Tags         Admin - Notifications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body notificationdto.CreateNotificationInput true "Notification payload"
// @Success      201 {object} utils.APIResponse{data=notificationdto.NotificationDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/notifications [post]
func (h *NotificationHandler) CreateNotification(ctx *gin.Context) {
	var input notificationdto.CreateNotificationInput
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

// BroadcastNotification godoc
// @Summary      Broadcast notification
// @Description  Broadcast system notification to all active users (Admin/Moderator)
// @Tags         Admin - Notifications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body notificationdto.BroadcastNotificationInput true "Broadcast payload"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/notifications/broadcast [post]
func (h *NotificationHandler) BroadcastNotification(ctx *gin.Context) {
	var input notificationdto.BroadcastNotificationInput
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

// GetMyNotifications godoc
// @Summary      Get current user notifications
// @Description  Retrieve paginated notifications for current authenticated user
// @Tags         Notifications
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Success      200 {object} utils.APIResponse{data=[]notificationdto.NotificationDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /notifications/me [get]
func (h *NotificationHandler) GetMyNotifications(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var query notificationdto.GetNotificationsQuery
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

// GetUnreadCount godoc
// @Summary      Get unread notification count
// @Description  Get number of unread notifications for current user
// @Tags         Notifications
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} utils.APIResponse{data=notificationdto.UnreadCountDTO}
// @Failure      401 {object} utils.ErrorResponse
// @Router       /notifications/me/unread [get]
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

// MarkNotificationAsRead godoc
// @Summary      Mark notification as read
// @Description  Mark a specific notification as read by UUID
// @Tags         Notifications
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Notification UUID"
// @Success      200 {object} utils.APIResponse{data=notificationdto.NotificationDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /notifications/{uuid}/read [patch]
func (h *NotificationHandler) MarkNotificationAsRead(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var uri notificationdto.NotificationUUID
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

// MarkAllNotificationsAsRead godoc
// @Summary      Mark all notifications as read
// @Description  Mark all unread notifications of current user as read
// @Tags         Notifications
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} utils.APIResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /notifications/read-all [patch]
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

// DeleteNotification godoc
// @Summary      Delete notification
// @Description  Delete a notification by UUID
// @Tags         Notifications
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Notification UUID"
// @Success      204 "No Content"
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /notifications/{uuid} [delete]
func (h *NotificationHandler) DeleteNotification(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var uri notificationdto.NotificationUUID
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

	var query notificationdto.AdminGetNotificationsQuery
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

// GetAdminUnreadCount godoc
// @Summary      Get admin unread notification count
// @Description  Get number of unread admin notifications for current admin user
// @Tags         Admin Notifications
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} utils.APIResponse{data=notificationdto.UnreadCountDTO}
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/notifications/unread [get]
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

// MarkAllAdminNotificationsAsRead godoc
// @Summary      Mark all admin notifications as read
// @Description  Mark all unread admin notifications of current user as read
// @Tags         Admin Notifications
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} utils.APIResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/notifications/read-all [patch]
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
