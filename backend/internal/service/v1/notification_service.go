package v1service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/logger"
	"goshop/pkg/ws"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type notificationService struct {
	notificationRepo repository.NotificationRepository
	userRepo         repository.UserRepository
	wsHub            ws.Hub
}

// NewNotificationService creates a new instance of NotificationService.
func NewNotificationService(
	notificationRepo repository.NotificationRepository,
	userRepo repository.UserRepository,
	wsHub ws.Hub,
) NotificationService {
	return &notificationService{
		notificationRepo: notificationRepo,
		userRepo:         userRepo,
		wsHub:            wsHub,
	}
}

// ── Service Implementation ────────────────────────────────────────

func (s *notificationService) Send(ctx context.Context, input v1dto.SendNotificationInput) error {
	dataBytes, _ := s.marshalData(input.Data)

	params := sqlc.CreateNotificationParams{
		NotificationUserID: input.UserID,
		NotificationType:   input.Type,
		NotificationTitle:  input.Title,
		NotificationBody:   input.Body,
		NotificationData:   dataBytes,
	}

	created, err := s.notificationRepo.Create(ctx, params)
	if err != nil {
		logger.Log.Error().Err(err).
			Int32("user_id", input.UserID).
			Str("type", input.Type).
			Msg("Failed to persist in-app notification")
		return err
	}

	s.broadcastToUser(input.UserID, created)
	return nil
}

func (s *notificationService) CreateNotification(ctx context.Context, input v1dto.CreateNotificationInput) (v1dto.NotificationDTO, error) {
	user, err := s.getUser(ctx, input.UserUUID)
	if err != nil {
		return v1dto.NotificationDTO{}, err
	}

	dataBytes, err := s.marshalData(input.Data)
	if err != nil {
		return v1dto.NotificationDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to marshal notification data payload")
	}

	params := sqlc.CreateNotificationParams{
		NotificationUserID: user.UserID,
		NotificationType:   input.Type,
		NotificationTitle:  input.Title,
		NotificationBody:   input.Body,
		NotificationData:   dataBytes,
	}

	created, err := s.notificationRepo.Create(ctx, params)
	if err != nil {
		logger.Log.Error().Err(err).Msg("failed to create notification via admin endpoint")
		return v1dto.NotificationDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to persist notification")
	}

	s.broadcastToUser(user.UserID, created)
	return v1dto.MapToNotificationDTO(created), nil
}

func (s *notificationService) BroadcastNotification(ctx context.Context, input v1dto.BroadcastNotificationInput) error {
	dataBytes, err := s.marshalData(input.Data)
	if err != nil {
		return utils.NewError(utils.ErrCodeInternal, "Failed to marshal notification data payload")
	}

	err = s.notificationRepo.CreateForAllUsers(ctx, input.Type, input.Title, input.Body, dataBytes)
	if err != nil {
		logger.Log.Error().Err(err).Msg("failed to persist broadcast notification")
		return utils.NewError(utils.ErrCodeInternal, "Failed to create broadcast notification")
	}

	s.broadcastToAll(input)
	return nil
}

func (s *notificationService) GetMyNotifications(ctx context.Context, userUUID uuid.UUID, query v1dto.GetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error) {
	user, err := s.getUser(ctx, userUUID)
	if err != nil {
		return nil, 0, err
	}

	limit, offset := s.normalizePagination(query)

	notifications, err := s.notificationRepo.GetByUserID(ctx, user.UserID, limit, offset)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", user.UserID).Msg("Failed to get notifications")
		return nil, 0, utils.NewError(utils.ErrCodeInternal, "Failed to retrieve notifications")
	}

	total, err := s.notificationRepo.CountByUserID(ctx, user.UserID)
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

func (s *notificationService) GetUnreadNotificationCount(ctx context.Context, userUUID uuid.UUID) (v1dto.UnreadCountDTO, error) {
	user, err := s.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.UnreadCountDTO{}, err
	}

	count, err := s.notificationRepo.CountUnreadByUserID(ctx, user.UserID)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", user.UserID).Msg("Failed to count unread notifications")
		return v1dto.UnreadCountDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to retrieve unread count")
	}

	return v1dto.UnreadCountDTO{UnreadCount: count}, nil
}

func (s *notificationService) MarkNotificationAsRead(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) (v1dto.NotificationDTO, error) {
	user, err := s.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.NotificationDTO{}, err
	}

	updated, err := s.notificationRepo.MarkAsRead(ctx, notificationUUID, user.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
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

func (s *notificationService) MarkAllNotificationsAsRead(ctx context.Context, userUUID uuid.UUID) error {
	user, err := s.getUser(ctx, userUUID)
	if err != nil {
		return err
	}

	err = s.notificationRepo.MarkAllAsRead(ctx, user.UserID)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", user.UserID).Msg("Failed to mark all notifications as read")
		return utils.NewError(utils.ErrCodeInternal, "Failed to mark all notifications as read")
	}

	return nil
}

func (s *notificationService) DeleteNotification(ctx context.Context, userUUID uuid.UUID, notificationUUID uuid.UUID) error {
	user, err := s.getUser(ctx, userUUID)
	if err != nil {
		return err
	}

	err = s.notificationRepo.Delete(ctx, notificationUUID, user.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
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

// ── Methods ────────────────────────────────────

func (s *notificationService) getUser(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error) {
	user, err := s.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "Recipient user not found")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get user", err)
	}
	return user, nil
}

func (s *notificationService) marshalData(data any) ([]byte, error) {
	if data == nil {
		return []byte("{}"), nil
	}
	b, err := json.Marshal(data)
	if err != nil {
		return []byte("{}"), err
	}
	return b, nil
}

func (s *notificationService) broadcastToUser(userID int32, created sqlc.Notification) {
	if s.wsHub != nil {
		go s.wsHub.BroadcastToUser(userID, ws.WSMessage{
			Type: ws.EventTypeNotification,
			Data: v1dto.MapToNotificationDTO(created),
		})
	}
}

func (s *notificationService) broadcastToAll(input v1dto.BroadcastNotificationInput) {
	if s.wsHub != nil {
		go s.wsHub.BroadcastToAll(ws.WSMessage{
			Type: ws.EventTypeNotification,
			Data: map[string]any{
				"type":       input.Type,
				"title":      input.Title,
				"body":       input.Body,
				"data":       input.Data,
				"is_read":    false,
				"created_at": time.Now().UTC(),
			},
		})
	}
}

func (s *notificationService) normalizePagination(query v1dto.GetNotificationsQuery) (limit, offset int32) {
	page := query.Page
	if page <= 0 {
		page = 1
	}

	limit = query.Limit
	if limit < 1 {
		limit = 10
	}

	offset = (page - 1) * limit
	return limit, offset
}

// ── Admin ────────────────────────────────────

func (s *notificationService) NotifyAdmins(ctx context.Context, input v1dto.AdminNotifyInput) error {
	dataBytes, err := s.marshalData(input.Data)
	if err != nil {
		return utils.NewError(utils.ErrCodeInternal, "Failed to marshal notification data payload")
	}

	if err := s.notificationRepo.CreateForAdmins(ctx, input.Type, input.Title, input.Body, dataBytes); err != nil {
		logger.Log.Error().Err(err).Msg("failed to persist admin notification")
		return utils.NewError(utils.ErrCodeInternal, "Failed to create admin notification")
	}

	// Real-time broadcast to all connected admin sockets
	adminIDs, err := s.userRepo.GetActiveAdminUserIDs(ctx)
	if err != nil {
		logger.Log.Error().Err(err).Msg("failed to retrieve active admin user IDs")
		return nil
	}

	if s.wsHub != nil {
		wsMsg := ws.WSMessage{
			Type: ws.EventTypeNotification,
			Data: map[string]any{
				"type":       input.Type,
				"title":      input.Title,
				"body":       input.Body,
				"data":       input.Data,
				"is_read":    false,
				"created_at": time.Now().UTC(),
			},
		}

		for _, adminID := range adminIDs {
			go s.wsHub.BroadcastToUser(adminID, wsMsg)
		}
	}
	return nil
}

func (s *notificationService) GetAdminNotifications(ctx context.Context, userUUID uuid.UUID, query v1dto.AdminGetNotificationsQuery) ([]v1dto.NotificationDTO, int32, error) {
	user, err := s.getUser(ctx, userUUID)
	if err != nil {
		return nil, 0, err
	}
	page := query.Page
	if page <= 0 {
		page = 1
	}
	limit := query.Limit
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit
	notifications, err := s.notificationRepo.GetAdminNotifications(ctx, user.UserID, query.Type, query.IsRead, limit, offset)
	if err != nil {
		logger.Log.Error().Err(err).Int32("admin_id", user.UserID).Msg("Failed to fetch admin notifications")
		return nil, 0, utils.NewError(utils.ErrCodeInternal, "Failed to retrieve admin notifications")
	}
	total, err := s.notificationRepo.CountAdminNotifications(ctx, user.UserID, query.Type, query.IsRead)
	if err != nil {
		logger.Log.Error().Err(err).Int32("admin_id", user.UserID).Msg("Failed to count admin notifications")
		return nil, 0, utils.NewError(utils.ErrCodeInternal, "Failed to count admin notifications")
	}
	dtos := make([]v1dto.NotificationDTO, 0, len(notifications))
	for _, n := range notifications {
		dtos = append(dtos, v1dto.MapToNotificationDTO(n))
	}
	return dtos, int32(total), nil
}
