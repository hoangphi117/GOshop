package v1service

import (
	"context"
	"encoding/json"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/logger"

	"github.com/gin-gonic/gin"
)

type notificationService struct {
	notificationRepo repository.NotificationRepository
	userRepo         repository.UserRepository
}

func NewNotificationService(
	notificationRepo repository.NotificationRepository,
	userRepo repository.UserRepository,
) NotificationService {
	return &notificationService{
		notificationRepo: notificationRepo,
		userRepo:         userRepo,
	}
}

func (s *notificationService) Send(ctx context.Context, input v1dto.SendNotificationInput) error {
	var dataBytes []byte
	if input.Data != nil {
		b, err := json.Marshal(input.Data)
		if err != nil {
			logger.Log.Error().Err(err).Msg("failed to marshal notification data payload")
			dataBytes = []byte("{}")
		} else {
			dataBytes = b
		}
	} else {
		dataBytes = []byte("{}")
	}

	params := sqlc.CreateNotificationParams{
		NotificationUserID: input.UserID,
		NotificationType:   input.Type,
		NotificationTitle:  input.Title,
		NotificationBody:   input.Body,
		NotificationData:   dataBytes,
	}

	_, err := s.notificationRepo.Create(ctx, params)
	if err != nil {
		logger.Log.Error().Err(err).
			Int32("user_id", input.UserID).
			Str("type", input.Type).
			Msg("Failed to persist in-app notification")
		return err
	}
	return nil
}

func (s *notificationService) CreateNotification(ctx *gin.Context, input v1dto.CreateNotificationInput) (v1dto.NotificationDTO, error) {
	reqCtx := ctx.Request.Context()

	user, err := s.userRepo.GetByUUID(reqCtx, input.UserUUID)
	if err != nil {
		return v1dto.NotificationDTO{}, utils.NewError(utils.ErrCodeNotFound, "Recipient user not found")
	}

	var dataBytes []byte
	if input.Data != nil {
		b, err := json.Marshal(input.Data)
		if err != nil {
			return v1dto.NotificationDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to marshal notification data payload")
		}
		dataBytes = b
	} else {
		dataBytes = []byte("{}")
	}

	// Persist Notification in the database
	params := sqlc.CreateNotificationParams{
		NotificationUserID: user.UserID,
		NotificationType:   input.Type,
		NotificationTitle:  input.Title,
		NotificationBody:   input.Body,
		NotificationData:   dataBytes,
	}

	created, err := s.notificationRepo.Create(reqCtx, params)
	if err != nil {
		logger.Log.Error().Err(err).Msg("failed to create notification via admin endpoint")
		return v1dto.NotificationDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to persist notification")
	}

	var responseData map[string]any
	if len(created.NotificationData) > 0 {
		_ = json.Unmarshal(created.NotificationData, &responseData)
	}

	return v1dto.NotificationDTO{
		UUID:      created.NotificationUuid,
		Type:      created.NotificationType,
		Title:     created.NotificationTitle,
		Body:      created.NotificationBody,
		Data:      responseData,
		IsRead:    created.NotificationIsRead,
		ReadAt:    &created.NotificationReadAt.Time,
		CreatedAt: created.NotificationCreatedAt,
	}, nil
}
