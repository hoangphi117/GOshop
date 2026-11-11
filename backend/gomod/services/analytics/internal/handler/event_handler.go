package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	analyticsdto "goshop/services/analytics/internal/dto"
	"goshop/services/analytics/internal/repository"
	"goshop/services/analytics/internal/service"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/utils"
	"goshop/shared/pkg/validation"
)

// EventHandler is the public interaction surface: event ingestion plus the
// search history the storefront dropdown reads.
type EventHandler struct {
	service service.EventService
}

func NewEventHandler(svc service.EventService) *EventHandler {
	return &EventHandler{service: svc}
}

// Ingest godoc
// @Summary      Ingest interaction events
// @Description  Accepts a batch of client events (search, search_click, view_item, add_to_cart, purchase). Identity always comes from the session cookie or bearer token, never from the body.
// @Tags         Events
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body body analyticsdto.EngagementBatch true "Batch of events"
// @Success      202 {object} map[string]any
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /api/v1/events [post]
func (h *EventHandler) Ingest(ctx *gin.Context) {
	var body analyticsdto.EngagementBatch
	if err := ctx.ShouldBindJSON(&body); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	anonID := middleware.GetAnonID(ctx)
	userID, _ := middleware.GetUserID(ctx)

	accepted, err := h.service.Ingest(ctx.Request.Context(), anonID, userID, body.Events)
	if err != nil {
		writeEventError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusAccepted, "Events accepted", gin.H{"accepted": accepted})
}

// GetSearchHistory godoc
// @Summary      Recent search keywords
// @Description  Distinct keywords from the caller's event log, newest first. Empty results and dismissed keywords are excluded.
// @Tags         Events
// @Produce      json
// @Security     ApiKeyAuth
// @Param        limit query int false "Maximum keywords (default 10)" default(10)
// @Success      200 {object} utils.APIResponse{data=[]repository.SearchHistoryEntry}
// @Failure      401 {object} utils.ErrorResponse
// @Router       /api/v1/search/history [get]
func (h *EventHandler) GetSearchHistory(ctx *gin.Context) {
	var query analyticsdto.SearchHistoryQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	entries, err := h.service.SearchHistory(ctx.Request.Context(), middleware.GetAnonID(ctx), mustUserID(ctx), query.Limit)
	if err != nil {
		writeEventError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Search history retrieved successfully", entries)
}

// DeleteSearchHistory godoc
// @Summary      Clear search history
// @Description  Hides one keyword when ?query= is given, otherwise every keyword seen so far. The raw event log stays immutable.
// @Tags         Events
// @Produce      json
// @Security     ApiKeyAuth
// @Param        query query string false "Keyword to remove"
// @Success      200 {object} utils.APIResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /api/v1/search/history [delete]
func (h *EventHandler) DeleteSearchHistory(ctx *gin.Context) {
	anonID := middleware.GetAnonID(ctx)
	userID := mustUserID(ctx)

	var err error
	if q := ctx.Query("query"); q != "" {
		err = h.service.DismissSearch(ctx.Request.Context(), anonID, userID, q)
	} else {
		err = h.service.DismissAllSearches(ctx.Request.Context(), anonID, userID)
	}
	if err != nil {
		writeEventError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Search history cleared")
}

func mustUserID(ctx *gin.Context) int32 {
	id, _ := middleware.GetUserID(ctx)
	return id
}

// writeEventError maps validation failures to 400/401 so a bad client payload
// is never reported as a server fault.
func writeEventError(ctx *gin.Context, err error) {
	var rejected *repository.ErrEventRejected
	switch {
	case errors.Is(err, repository.ErrSearchHistorySubject):
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "No identity for this request", "code": utils.ErrCodeUnauthorized})
	case errors.As(err, &rejected):
		ctx.JSON(http.StatusBadRequest, gin.H{"error": rejected.Reason, "code": utils.ErrCodeBadRequest})
	default:
		utils.ResponseError(ctx, err)
	}
}
