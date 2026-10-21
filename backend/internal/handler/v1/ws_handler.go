package v1handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/ws"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	UserLevelAdmin     = 1
	UserLevelModerator = 2
)

type WSHandler struct {
	hub          ws.Hub
	jwtService   auth.TokenService
	cacheService cache.RedisCacheService
	userRepo     repository.UserReader
	chatRepo     repository.ChatRepository
}

func NewWSHandler(
	hub ws.Hub,
	jwtService auth.TokenService,
	cacheService cache.RedisCacheService,
	userRepo repository.UserReader,
) *WSHandler {
	return &WSHandler{
		hub:          hub,
		jwtService:   jwtService,
		cacheService: cacheService,
		userRepo:     userRepo,
	}
}

// SetChatRepo helps DI injection for ChatRepository without causing breaking changes in the constructor
func (h *WSHandler) SetChatRepo(chatRepo repository.ChatRepository) {
	h.chatRepo = chatRepo
}

// HandleWS godoc
// @Summary      WebSocket connection
// @Description  Establish persistent bidirectional WebSocket connection for real-time order, notifications, and chat rooms
// @Tags         WebSocket
// @Param        token             query string false "JWT Access Token"
// @Param        conversation_uuid query string false "Conversation UUID to join chat room"
// @Success      101 "Switching Protocols"
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /ws [get]
func (h *WSHandler) HandleWS(ctx *gin.Context) {
	tokenString := ctx.Query("token")
	if tokenString == "" {
		authHeader := ctx.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if tokenString == "" {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeUnauthorized, "Missing authentication token"))
		return
	}

	// Validate and check blacklist
	_, claims, err := h.jwtService.ParseToken(tokenString)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeUnauthorized, "Invalid authentication token"))
		return
	}

	if jti, ok := claims["jti"].(string); ok {
		key := "blacklist:" + jti
		exists, err := h.cacheService.Exists(key)
		if err != nil && exists {
			utils.ResponseError(ctx, utils.NewError(utils.ErrCodeUnauthorized, "Token has been revoked"))
			return
		}
	}

	payload, err := h.jwtService.DecryptAccessTokenPayload(tokenString)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeUnauthorized, "Failed to decrypt access token payload"))
		return
	}

	userUUID, err := uuid.Parse(payload.UserUUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID in token payload"))
		return
	}

	user, err := h.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeUnauthorized, "User not found"))
		return
	}

	isAdmin := user.UserLevel == UserLevelAdmin || user.UserLevel == UserLevelModerator

	// Resolve Chat Room if conversation_uuid query parameter is provided
	var targetConvID *int32
	convUUIDStr := strings.TrimSpace(ctx.Query("conversation_uuid"))
	if convUUIDStr != "" {
		convUUID, parseErr := uuid.Parse(convUUIDStr)
		if parseErr != nil {
			utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid conversation UUID"))
			return
		}

		if h.chatRepo != nil {
			conv, convErr := h.chatRepo.GetConversationByUUID(ctx, convUUID)
			if convErr != nil {
				if errors.Is(convErr, sql.ErrNoRows) || errors.Is(convErr, pgx.ErrNoRows) {
					utils.ResponseError(ctx, utils.NewError(utils.ErrCodeNotFound, "Conversation not found"))
					return
				}
				utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeInternal, "Failed to resolve conversation", convErr))
				return
			}

			// Authorization check: Admin can join any conversation room, regular user can only join their own
			if !isAdmin && conv.UserID != user.UserID {
				utils.ResponseError(ctx, utils.NewError(utils.ErrCodeForbidden, "You are not authorized to join this conversation room"))
				return
			}

			// If the connected user is the conversation owner, they participate as customer ("user")
			if conv.UserID == user.UserID {
				isAdmin = false
			}

			targetConvID = &conv.ConversationID
		}
	}

	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := ws.Upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", user.UserID).Msg("Failed to upgrade connection to WebSocket")
		return
	}

	// Create a new WebSocket client and register it with the hub
	client := ws.NewClient(h.hub, conn, user.UserID, isAdmin)
	client.ConversationID = targetConvID
	h.hub.Register(client)

	// Broadcast online presence to other active connections
	if isAdmin {
		h.hub.BroadcastAdminPresence(true)
	} else {
		h.hub.BroadcastUserPresence(user.UserID, targetConvID, true)
	}

	// Send initial presence directly to this client so they don't have to wait for an event
	go func() {
		// 1. Send shop admin online status
		hasAdmin := h.hub.HasOnlineAdmin()
		shopPresence, _ := json.Marshal(ws.WSMessage{
			Type: ws.EventChatPresence,
			Data: ws.ChatPresencePayload{
				Role:     "admin",
				IsOnline: hasAdmin,
			},
		})
		client.SafeSend(shopPresence)

		// 2. If client is admin and target conversation is set, send customer presence
		if isAdmin && targetConvID != nil && h.chatRepo != nil {
			conv, convErr := h.chatRepo.GetConversationByID(context.Background(), *targetConvID)
			if convErr == nil {
				custOnline := h.hub.IsUserOnline(conv.UserID)
				custPresence, _ := json.Marshal(ws.WSMessage{
					Type: ws.EventChatPresence,
					Data: ws.ChatPresencePayload{
						ConversationUUID: conv.ConversationUuid.String(),
						UserID:           conv.UserID,
						Role:             "user",
						IsOnline:         custOnline,
					},
				})
				client.SafeSend(custPresence)
			}
		}
	}()

	// Start client pumps in separate goroutines
	go client.ReadPump()
	go client.WritePump()
}
