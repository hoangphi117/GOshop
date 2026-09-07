package v1service

import (
	"context"
	"encoding/json"
	"errors"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func (s *notificationService) GetMyNotifications(ctx *gin.Context, query v1dto.GetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error) {
	reqCtx := ctx.Request.Context()

	userUUIDStr, ok := ctx.Get("user_uuid")
	if !ok {
		return nil, 0, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return nil, 0, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID")
	}

	user, err := s.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return nil, 0, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	page := query.Page
	if page <= 0 {
		page = 1
	}

	limit := query.Limit
	if limit < 1 {
		limit = 10
	}

	notifications, err := s.notificationRepo.GetByUserID(reqCtx, user.UserID, page, limit)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", user.UserID).Msg("Failed to get notifications")
		return nil, 0, utils.NewError(utils.ErrCodeInternal, "Failed to retrieve notifications")
	}

	total, err := s.notificationRepo.CountByUserID(reqCtx, user.UserID)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", user.UserID).Msg("Failed to count notifications")
		return nil, 0, utils.NewError(utils.ErrCodeInternal, "Failed to count notifications")
	}

	dtos := make([]v1dto.NotificationDTO, 0, len(notifications))
	for _, n := range notifications {
		dtos = append(dtos, v1dto.MapToNotificationDTO(n))
	}

	return dtos, int32(total), nil
}

func (s *notificationService) GetUnreadNotificationCount(ctx *gin.Context) (v1dto.UnreadCountDTO, error) {
	reqCtx := ctx.Request.Context()

	userUUIDStr, exists := ctx.Get("user_uuid")
	if !exists {
		return v1dto.UnreadCountDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "User unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return v1dto.UnreadCountDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid user UUID")
	}

	user, err := s.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return v1dto.UnreadCountDTO{}, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	count, err := s.notificationRepo.CountUnreadByUserID(reqCtx, user.UserID)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", user.UserID).Msg("Failed to count unread notifications")
		return v1dto.UnreadCountDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to retrieve unread count")
	}

	return v1dto.UnreadCountDTO{UnreadCount: count}, nil
}

func (s *notificationService) MarkNotificationAsRead(ctx *gin.Context, notificationUUID uuid.UUID) (v1dto.NotificationDTO, error) {
	reqCtx := ctx.Request.Context()
	userUUIDStr, exists := ctx.Get("user_uuid")
	if !exists {
		return v1dto.NotificationDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "User unauthorized")
	}
	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return v1dto.NotificationDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid user UUID")
	}
	user, err := s.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return v1dto.NotificationDTO{}, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	updated, err := s.notificationRepo.MarkAsRead(reqCtx, notificationUUID, user.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return v1dto.NotificationDTO{}, utils.NewError(utils.ErrCodeNotFound, "Notification not found")
		}
		logger.Log.Error().Err(err).
			Str("notification_uuid", notificationUUID.String()).
			Int32("user_id", user.UserID).
			Msg("Failed to mark notification as read")
		return v1dto.NotificationDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to mark notification as read")
	}

	return v1dto.MapToNotificationDTO(updated), nil
}

func (s *notificationService) MarkAllNotificationsAsRead(ctx *gin.Context) error {
	reqCtx := ctx.Request.Context()

	userUUIDStr, exists := ctx.Get("user_uuid")
	if !exists {
		return utils.NewError(utils.ErrCodeUnauthorized, "User unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return utils.NewError(utils.ErrCodeBadRequest, "Invalid user UUID")
	}

	user, err := s.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	err = s.notificationRepo.MarkAllAsRead(reqCtx, user.UserID)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", user.UserID).Msg("Failed to mark all notifications as read")
		return utils.NewError(utils.ErrCodeInternal, "Failed to mark all notifications as read")
	}

	return nil
}

func (s *notificationService) DeleteNotification(ctx *gin.Context, notificationUUID uuid.UUID) error {
	reqCtx := ctx.Request.Context()

	userUUIDStr, exists := ctx.Get("user_uuid")
	if !exists {
		return utils.NewError(utils.ErrCodeUnauthorized, "User unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return utils.NewError(utils.ErrCodeBadRequest, "Invalid user UUID")
	}

	user, err := s.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	err = s.notificationRepo.Delete(reqCtx, notificationUUID, user.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Notification not found")
		}
		logger.Log.Error().Err(err).
			Str("notification_uuid", notificationUUID.String()).
			Int32("user_id", user.UserID).
			Msg("Failed to delete notification")
		return utils.NewError(utils.ErrCodeInternal, "Failed to delete notification")
	}

	return nil
}
