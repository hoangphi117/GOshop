package v1handler

import (
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/ws"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	UserLevelAdmin     = 1
	UserLevelModerator = 2
)

type WSHandler struct {
	hub          ws.Hub
	jwtService   auth.TokenService
	cacheService cache.RedisCacheService
	userRepo     repository.UserRepository
}

func NewWSHandler(
	hub ws.Hub,
	jwtService auth.TokenService,
	cacheService cache.RedisCacheService,
	userRepo repository.UserRepository,
) *WSHandler {
	return &WSHandler{
		hub:          hub,
		jwtService:   jwtService,
		cacheService: cacheService,
		userRepo:     userRepo,
	}
}

// HandleWS godoc
// @Summary      WebSocket connection
// @Description  Establish persistent bidirectional WebSocket connection for real-time order and notification events
// @Tags         WebSocket
// @Param        token query string false "JWT Access Token"
// @Success      101 "Switching Protocols"
// @Failure      401 {object} utils.ErrorResponse
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

	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := ws.Upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		logger.Log.Error().Err(err).Int32("user_id", user.UserID).Msg("Failed to upgrade connection to WebSocket")
		return
	}

	// Create a new WebSocket client and register it with the hub
	client := ws.NewClient(h.hub, conn, user.UserID, isAdmin)
	h.hub.Register(client)

	// Start client pumps in separate goroutines
	go client.ReadPump()
	go client.WritePump()
}
