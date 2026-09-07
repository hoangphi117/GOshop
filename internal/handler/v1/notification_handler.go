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

// CreateNotification godoc
// @Summary      Create and send a notification
// @Description  Allows admin/system to send an in-app notification to a user
// @Tags         Admin Notifications
// @Accept       json
// @Produce      json
// @Param        input body v1dto.CreateNotificationInput true "Notification payload"
// @Success      201 {object} utils.Response{data=v1dto.NotificationDTO}
// @Failure      400 {object} utils.Response
// @Failure      404 {object} utils.Response
// @Failure      500 {object} utils.Response
// @Router       /admin/notifications [post]
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
