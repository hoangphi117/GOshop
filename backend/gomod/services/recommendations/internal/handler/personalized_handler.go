package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	recssvc "goshop/services/recommendations/internal/service"
	"goshop/shared/pkg/middleware"
	"goshop/shared/pkg/utils"
)

// PersonalizedHandler serves the two vector-backed shelves.
type PersonalizedHandler struct {
	bestSellers  recssvc.RecommendationService
	personalized recssvc.PersonalizedService
}

func NewPersonalizedHandler(
	bestSellers recssvc.RecommendationService,
	personalized recssvc.PersonalizedService,
) *PersonalizedHandler {
	return &PersonalizedHandler{bestSellers: bestSellers, personalized: personalized}
}

// ForVisitor godoc
// @Summary      Get personalized recommendations
// @Description  Products near the centroid of the caller's interaction history. Falls back (and reports its own source) when the visitor has no usable history.
// @Tags         Recommendations
// @Produce      json
// @Param        limit query int false "Number of products (default 12, max 50)"
// @Success      200 {object} recssvc.Shelf
// @Failure      400 {object} utils.ErrorResponse
// @Router       /api/v1/recommendations/personalized [get]
func (h *PersonalizedHandler) ForVisitor(ctx *gin.Context) {
	limit, ok := parseLimit(ctx)
	if !ok {
		return
	}

	shelf, reported, err := h.personalized.ForVisitor(ctx.Request.Context(), visitorKey(ctx), limit)
	if err != nil {
		// A missing vector table must not 500 the homepage shelf.
		utils.ResponseError(ctx, err)
		return
	}
	if !reported {
		shelf, err = h.bestSellers.BestSellers(ctx.Request.Context(), limit)
		if err != nil {
			utils.ResponseError(ctx, err)
			return
		}
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Recommendations retrieved successfully", shelf)
}

// Similar godoc
// @Summary      Get products similar to one product
// @Description  Nearest neighbours in embedding space, excluding the product itself and anything out of stock.
// @Tags         Recommendations
// @Produce      json
// @Param        uuid query string true "Product UUID"
// @Param        limit query int false "Number of products (default 12, max 50)"
// @Success      200 {object} recssvc.Shelf
// @Failure      400 {object} utils.ErrorResponse
// @Router       /api/v1/products/similar [get]
func (h *PersonalizedHandler) Similar(ctx *gin.Context) {
	limit, ok := parseLimit(ctx)
	if !ok {
		return
	}

	raw := strings.TrimSpace(ctx.Query("uuid"))
	id, err := uuid.Parse(raw)
	if err != nil {
		utils.ResponseValidator(ctx, map[string]string{"uuid": "uuid must be a valid product UUID"})
		return
	}

	shelf, err := h.personalized.Similar(ctx.Request.Context(), id, limit)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Similar products retrieved successfully", shelf)
}

// Search godoc
// @Summary      Hybrid product search
// @Description  Ranks the catalog by keyword relevance and by embedding similarity, fused with reciprocal rank fusion. The response always reports in `source` which halves were available (hybrid, keyword, semantic). Filters are applied inside both halves, so `total` counts only matches the shopper would actually accept.
// @Tags         Recommendations
// @Produce      json
// @Param        q query string true "Search query"
// @Param        page query int false "Page number (default 1)"
// @Param        limit query int false "Page size (default 20, max 50)"
// @Param        category_uuid query string false "Restrict to one category UUID"
// @Param        min_price query number false "Minimum product price (inclusive)"
// @Param        max_price query number false "Maximum product price (inclusive)"
// @Param        min_rating query number false "Minimum average rating (inclusive)"
// @Success      200 {object} recssvc.Shelf
// @Failure      400 {object} utils.ErrorResponse
// @Router       /api/v1/products/search [get]
func (h *PersonalizedHandler) Search(ctx *gin.Context) {
	query := strings.TrimSpace(ctx.Query("q"))
	if query == "" {
		utils.ResponseValidator(ctx, map[string]string{"q": "q is required"})
		return
	}

	page := 1
	if raw := ctx.Query("page"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			utils.ResponseValidator(ctx, map[string]string{"page": "Page must be a positive integer"})
			return
		}
		page = parsed
	}
	limit := 20
	if raw := ctx.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 50 {
			utils.ResponseValidator(ctx, map[string]string{"limit": "Limit must be an integer between 1 and 50"})
			return
		}
		limit = parsed
	}

	narrowing, ok := parseSearchNarrowing(ctx)
	if !ok {
		return
	}

	shelf, err := h.personalized.HybridSearch(ctx.Request.Context(), recssvc.HybridQuery{
		Query:     query,
		Page:      page,
		Limit:     limit,
		Narrowing: narrowing,
	})
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Products searched successfully", shelf)
}

// parseSearchNarrowing reads the facet query params the storefront already puts
// on every listing request, so the search box and the filter sidebar agree on
// names. Malformed values are a 400 rather than a silently dropped filter: a
// shopper who typed a maximum price must not receive results above it.
func parseSearchNarrowing(ctx *gin.Context) (recssvc.SearchNarrowing, bool) {
	var (
		out     recssvc.SearchNarrowing
		invalid = map[string]string{}
	)

	if raw := strings.TrimSpace(ctx.Query("category_uuid")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			invalid["category_uuid"] = "category_uuid must be a valid UUID"
		} else {
			out.CategoryUUID = id
		}
	}
	for name, dst := range map[string]*float64{
		"min_price":  &out.MinPrice,
		"max_price":  &out.MaxPrice,
		"min_rating": &out.MinRating,
	} {
		raw := strings.TrimSpace(ctx.Query(name))
		if raw == "" {
			continue
		}
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed < 0 {
			invalid[name] = name + " must be a non-negative number"
			continue
		}
		*dst = parsed
	}

	if len(invalid) > 0 {
		utils.ResponseValidator(ctx, invalid)
		return recssvc.SearchNarrowing{}, false
	}
	if out.MaxPrice > 0 && out.MinPrice > out.MaxPrice {
		utils.ResponseValidator(ctx, map[string]string{"min_price": "min_price must not exceed max_price"})
		return recssvc.SearchNarrowing{}, false
	}
	return out, true
}

// visitorKey mirrors the affinity rollup's key scheme exactly ('u:<id>' when
// signed in, 'a:<anon uuid>' otherwise). A mismatch would silently return an
// empty profile for every visitor.
func visitorKey(ctx *gin.Context) string {
	if userID, ok := middleware.GetUserID(ctx); ok && userID != 0 {
		return "u:" + strconv.Itoa(int(userID))
	}
	if anon := middleware.GetAnonID(ctx); anon != "" {
		return "a:" + anon
	}
	return ""
}

func parseLimit(ctx *gin.Context) (int, bool) {
	limit := 12
	if raw := ctx.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 50 {
			utils.ResponseValidator(ctx, map[string]string{"limit": "Limit must be an integer between 1 and 50"})
			return 0, false
		}
		limit = parsed
	}
	return limit, true
}
