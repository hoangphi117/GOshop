package handler_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	analyticsdto "goshop/services/analytics/internal/dto"
	analyticshandler "goshop/services/analytics/internal/handler"
	"goshop/services/analytics/internal/repository"
	analyticsservice "goshop/services/analytics/internal/service"
)

type mockEventService struct {
	analyticsservice.EventService
	ingestFn  func(ctx context.Context, anonID string, userID int32, events []analyticsdto.EngagementEventInput) (int, error)
	historyFn func(ctx context.Context, anonID string, userID int32, limit int32) ([]repository.SearchHistoryEntry, error)
	dismissFn func(ctx context.Context, anonID string, userID int32, q string) error
	clearFn   func(ctx context.Context, anonID string, userID int32) error
}

func (m *mockEventService) Ingest(ctx context.Context, anonID string, userID int32, events []analyticsdto.EngagementEventInput) (int, error) {
	if m.ingestFn != nil {
		return m.ingestFn(ctx, anonID, userID, events)
	}
	return len(events), nil
}
func (m *mockEventService) SearchHistory(ctx context.Context, anonID string, userID int32, limit int32) ([]repository.SearchHistoryEntry, error) {
	if m.historyFn != nil {
		return m.historyFn(ctx, anonID, userID, limit)
	}
	return []repository.SearchHistoryEntry{{Query: "phone", SearchCnt: 5}}, nil
}
func (m *mockEventService) DismissSearch(ctx context.Context, anonID string, userID int32, q string) error {
	if m.dismissFn != nil {
		return m.dismissFn(ctx, anonID, userID, q)
	}
	return nil
}
func (m *mockEventService) DismissAllSearches(ctx context.Context, anonID string, userID int32) error {
	if m.clearFn != nil {
		return m.clearFn(ctx, anonID, userID)
	}
	return nil
}

func setupEventRouter(h *analyticshandler.EventHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/events", h.Ingest)
	r.GET("/api/v1/search/history", h.GetSearchHistory)
	r.DELETE("/api/v1/search/history", h.DeleteSearchHistory)
	return r
}

func TestEventHandler_Endpoints(t *testing.T) {
	mockSvc := &mockEventService{}
	r := setupEventRouter(analyticshandler.NewEventHandler(mockSvc))

	t.Run("Ingest valid events returns 202 Accepted", func(t *testing.T) {
		body := `{"events":[{"event_id":"11111111-1111-1111-1111-111111111111","event_type":"view_item","product_uuid":"00000000-0000-0000-0000-000000000001"}]}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusAccepted, w.Code)
		assert.Contains(t, w.Body.String(), `"accepted":1`)
	})

	t.Run("GetSearchHistory returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search/history?limit=5", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "phone")
	})

	t.Run("DeleteSearchHistory clears query or all", func(t *testing.T) {
		reqOne := httptest.NewRequest(http.MethodDelete, "/api/v1/search/history?query=phone", nil)
		wOne := httptest.NewRecorder()
		r.ServeHTTP(wOne, reqOne)
		assert.Equal(t, http.StatusOK, wOne.Code)

		reqAll := httptest.NewRequest(http.MethodDelete, "/api/v1/search/history", nil)
		wAll := httptest.NewRecorder()
		r.ServeHTTP(wAll, reqAll)
		assert.Equal(t, http.StatusOK, wAll.Code)
	})
}
