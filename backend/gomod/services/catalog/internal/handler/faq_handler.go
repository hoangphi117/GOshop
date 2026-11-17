package handler

import (
	"net/http"
	"strconv"
	"strings"

	"goshop/shared/pkg/utils"

	"github.com/gin-gonic/gin"
)

// ListFAQs godoc
// @Summary      List active FAQs
// @Description  Public FAQ knowledge base used by the storefront assistant.
// @Tags         FAQs
// @Produce      json
// @Param        limit query int false "Max rows (default 100, max 100)"
// @Param        offset query int false "Offset"
// @Success      200 {object} utils.APIResponse{data=[]catalogdto.FAQDTO}
// @Router       /api/v1/faqs [get]
func (h *CatalogHandler) ListFAQs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))

	items, err := h.svc.ListFAQs(c.Request.Context(), nil, int32(limit), int32(offset))
	if err != nil {
		utils.ResponseError(c, err)
		return
	}
	utils.ResponseSuccess(c, http.StatusOK, "FAQs retrieved successfully", items)
}

// SearchFAQs godoc
// @Summary      Search active FAQs
// @Description  Case-insensitive substring match over question and answer.
// @Tags         FAQs
// @Produce      json
// @Param        q query string false "Search text; empty returns every active FAQ"
// @Param        limit query int false "Max rows (default 5, max 100)"
// @Success      200 {object} utils.APIResponse{data=[]catalogdto.FAQDTO}
// @Router       /api/v1/faqs/search [get]
func (h *CatalogHandler) SearchFAQs(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 {
		limit = 5
	}

	var query *string
	if q != "" {
		query = &q
	}

	items, err := h.svc.ListFAQs(c.Request.Context(), query, int32(limit), 0)
	if err != nil {
		utils.ResponseError(c, err)
		return
	}
	utils.ResponseSuccess(c, http.StatusOK, "FAQs searched successfully", items)
}
