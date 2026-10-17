package handler

import (
	"net/http"

	"goshop/internal/utils"
	"goshop/pkg/middleware"
	"goshop/pkg/validation"
	"goshop/services/inventory/internal/dto"
	"goshop/services/inventory/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type InventoryHandler struct {
	svc service.InventoryService
}

func NewInventoryHandler(svc service.InventoryService) *InventoryHandler {
	return &InventoryHandler{svc: svc}
}

func (h *InventoryHandler) GetStockMovements(ctx *gin.Context) {
	var params dto.GetStockMovementsParam
	if err := ctx.ShouldBindQuery(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	movements, total, err := h.svc.ListStockMovements(ctx.Request.Context(), params)
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

	paginationResponse := utils.NewPaginationResponse(movements, int32(page), int32(limit), int32(total))
	utils.ResponseSuccess(ctx, http.StatusOK, "Stock movements retrieved", paginationResponse)
}

func (h *InventoryHandler) AdjustStock(ctx *gin.Context) {
	var actorUUID *uuid.UUID
	if userUUID, err := middleware.GetUserUUID(ctx); err == nil && userUUID != uuid.Nil {
		actorUUID = &userUUID
	}

	var input dto.AdjustStockInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	movement, err := h.svc.AdjustStock(ctx.Request.Context(), actorUUID, nil, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Stock adjusted successfully", movement)
}

func (h *InventoryHandler) GetStock(ctx *gin.Context) {
	productUUIDs := ctx.QueryArray("product_uuids")
	stocks, err := h.svc.GetStock(ctx.Request.Context(), productUUIDs, nil)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Stock retrieved successfully", stocks)
}
