package v1handler

import (
	"net/http"

	"goshop/internal/middleware"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type SessionHandler struct {
	sessionService v1service.SessionService
}

func NewSessionHandler(sessionService v1service.SessionService) *SessionHandler {
	return &SessionHandler{
		sessionService: sessionService,
	}
}

func getRefreshTokenFromRequest(ctx *gin.Context) string {
	if cookieVal, err := ctx.Cookie("refresh_token"); err == nil && cookieVal != "" {
		return cookieVal
	}
	return ctx.Query("refresh_token")
}

// ListSessions godoc
// @Summary      Get active login sessions
// @Description  Retrieve all active devices/sessions for the authenticated user
// @Tags         Users - Sessions
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} utils.APIResponse{data=[]v1dto.SessionDTO}
// @Failure      401 {object} utils.ErrorResponse
// @Router       /users/sessions [get]
func (sh *SessionHandler) ListSessions(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	currentRefreshToken := getRefreshTokenFromRequest(ctx)
	sessions, err := sh.sessionService.ListUserSessions(ctx.Request.Context(), userUUID, currentRefreshToken)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Get active sessions successfully", sessions)
}

// RevokeSession godoc
// @Summary      Revoke specific session
// @Description  Terminate a specific active login session by its UUID
// @Tags         Users - Sessions
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Session UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      403 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /users/sessions/{uuid} [delete]
func (sh *SessionHandler) RevokeSession(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	sessionUUIDStr := ctx.Param("uuid")
	sessionUUID, err := uuid.Parse(sessionUUIDStr)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid session UUID format"))
		return
	}

	err = sh.sessionService.RevokeSession(ctx.Request.Context(), userUUID, sessionUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Session terminated successfully")
}

// RevokeAllOtherSessions godoc
// @Summary      Revoke all other sessions
// @Description  Log out of all other devices except the current active session
// @Tags         Users - Sessions
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /users/sessions [delete]
func (sh *SessionHandler) RevokeAllOtherSessions(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	currentRefreshToken := getRefreshTokenFromRequest(ctx)
	err = sh.sessionService.RevokeAllOtherSessions(ctx.Request.Context(), userUUID, currentRefreshToken)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "All other sessions terminated successfully")
}
