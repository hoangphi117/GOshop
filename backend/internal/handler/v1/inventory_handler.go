package v1handler

import (
	v1dto "goshop/internal/dto/v1"
	"goshop/pkg/middleware"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"

	"github.com/gin-gonic/gin"
)

type InventoryHandler struct {
	inventoryService v1service.InventoryService
}

func NewInventoryHandler(inventoryService v1service.InventoryService) *InventoryHandler {
	return &InventoryHandler{
		inventoryService: inventoryService,
	}
}

// GetStockMovements godoc
// @Summary      Get stock movement history
// @Description  Retrieve paginated stock movement audit trail with filters (Admin/Warehouse)
// @Tags         Admin - Inventory
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        query query v1dto.GetStockMovementsParam false "Filter params"
// @Success      200 {object} utils.APIResponse{data=[]v1dto.StockMovementDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/inventory/movements [get]
func (h *InventoryHandler) GetStockMovements(ctx *gin.Context) {
	var params v1dto.GetStockMovementsParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	movements, total, err := h.inventoryService.ListStockMovements(ctx.Request.Context(), params)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	limit := params.Limit
	if limit < 1 || limit > 100 {
		limit = 10
	}
	page := params.Page
	if page < 1 {
		page = 1
	}

	paginationResponse := utils.NewPaginationResponse(movements, page, limit, int32(total))

	utils.ResponseSuccess(ctx, http.StatusOK, "Stock movements retrieved", paginationResponse)
}

// AdjustStock godoc
// @Summary      Adjust product stock
// @Description  Manually add or deduct inventory with reason/audit note (Admin/Warehouse)
// @Tags         Admin - Inventory
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body v1dto.AdjustStockInput true "Stock adjustment payload"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/inventory/adjust [post]
func (h *InventoryHandler) AdjustStock(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized"))
		return
	}

	var input v1dto.AdjustStockInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	movement, err := h.inventoryService.AdjustStock(ctx.Request.Context(), userUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Stock adjusted successfully", movement)
}
