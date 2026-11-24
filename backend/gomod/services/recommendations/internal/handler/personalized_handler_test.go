package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	recssvc "goshop/services/recommendations/internal/service"
)

// capturePersonalized records the visitor key the handler forwarded, so the tier
// override can be asserted without a database.
type capturePersonalized struct {
	recssvc.PersonalizedService
	key string
}

func (c *capturePersonalized) ForVisitor(_ context.Context, visitorKey string, _ int) (recssvc.Shelf, bool, error) {
	c.key = visitorKey
	return recssvc.Shelf{Source: recssvc.SourceOrders}, true, nil
}

type fixedBestSellers struct{ recssvc.RecommendationService }

func (fixedBestSellers) Trending(context.Context, int) (recssvc.Shelf, bool, error) {
	return recssvc.Shelf{}, false, nil
}

func (fixedBestSellers) BestSellers(context.Context, int) (recssvc.Shelf, error) {
	return recssvc.Shelf{Source: recssvc.SourceOrders}, nil
}

func (fixedBestSellers) NewArrivals(context.Context, int) (recssvc.Shelf, error) {
	return recssvc.Shelf{Source: recssvc.SourceNewArrivals}, nil
}

func runForVisitor(t *testing.T, query string) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	personalized := &capturePersonalized{}
	h := NewPersonalizedHandler(fixedBestSellers{}, personalized)

	r := gin.New()
	r.GET("/api/v1/recommendations/personalized", func(ctx *gin.Context) {
		// Stands in for middleware.AnonCookie, which is what gives a visitor an
		// identity in production.
		ctx.Set("anon_id", "dddd0001-dddd-4ddd-8ddd-ddddddddddd1")
		h.ForVisitor(ctx)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/recommendations/personalized"+query, nil))
	return w.Code, personalized.key
}

func TestForVisitorUsesAnonHistoryByDefault(t *testing.T) {
	_, key := runForVisitor(t, "?limit=12")
	if key != "a:dddd0001-dddd-4ddd-8ddd-ddddddddddd1" {
		t.Fatalf("visitor key = %q, want the anon key", key)
	}
}

func TestForVisitorCrowdTierIgnoresHistory(t *testing.T) {
	code, key := runForVisitor(t, "?limit=12&tier=crowd")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if key != "" {
		t.Fatalf("visitor key = %q, want empty so the service answers with a crowd shelf", key)
	}
}

func TestForVisitorTrendingTierBypassesPersonal(t *testing.T) {
	code, key := runForVisitor(t, "?limit=12&tier=trending")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if key != "" {
		t.Fatalf("visitor key = %q, want empty because trending skips personalization", key)
	}
}

func TestForVisitorPersonalTierUsesHistory(t *testing.T) {
	code, key := runForVisitor(t, "?limit=12&tier=personal")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if key != "a:dddd0001-dddd-4ddd-8ddd-ddddddddddd1" {
		t.Fatalf("visitor key = %q, want personal key", key)
	}
}

func TestForVisitorRejectsUnknownTier(t *testing.T) {
	if code, _ := runForVisitor(t, "?tier=invalid"); code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", code)
	}
}
