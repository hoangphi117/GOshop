package handler

import (
	"net/http"
	"strconv"

	recssvc "goshop/services/recommendations/internal/service"
	"goshop/shared/pkg/utils"

	"github.com/gin-gonic/gin"
)

type RecommendationHandler struct {
	service recssvc.RecommendationService
}

func NewRecommendationHandler(service recssvc.RecommendationService) *RecommendationHandler {
	return &RecommendationHandler{service: service}
}

// BestSellers godoc
// @Summary      Get best-selling products
// @Description  Top-selling products over the popularity window. When the store has no sales yet, falls back to top-rated products and reports source=fallback_rating.
// @Tags         Recommendations
// @Produce      json
// @Param        limit query int false "Number of products (default 12, max 50)"
// @Success      200 {object} recssvc.Shelf
// @Failure      400 {object} utils.ErrorResponse
// @Router       /api/v1/recommendations [get]
func (h *RecommendationHandler) BestSellers(ctx *gin.Context) {
	limit := 12
	if raw := ctx.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 50 {
			utils.ResponseValidator(ctx, map[string]string{"limit": "Limit must be an integer between 1 and 50"})
			return
		}
		limit = parsed
	}

	shelf, err := h.service.BestSellers(ctx.Request.Context(), limit)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Recommendations retrieved successfully", shelf)
}
