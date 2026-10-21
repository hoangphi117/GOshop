package handler

import (
	"context"
	"encoding/json"
	"fmt"
	v1dto "goshop/services/order/internal/dto"
	"goshop/pkg/middleware"
	orderservice "goshop/services/order/internal/service"
	"goshop/pkg/utils"
	"goshop/pkg/validation"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"strconv"
)

type OrderHandler struct {
	customerService orderservice.CustomerOrderService
	shippingService orderservice.OrderShippingService
	locationService orderservice.LocationService
	returnService   orderservice.OrderReturnService
	invoiceService  orderservice.OrderInvoiceService
	adminService    orderservice.OrderAdminService
}

func NewOrderHandler(
	customerService orderservice.CustomerOrderService,
	shippingService orderservice.OrderShippingService,
	returnService orderservice.OrderReturnService,
) *OrderHandler {
	var invSvc orderservice.OrderInvoiceService
	var admSvc orderservice.OrderAdminService
	var locSvc orderservice.LocationService
	if asInv, ok := customerService.(orderservice.OrderInvoiceService); ok {
		invSvc = asInv
	}
	if asAdm, ok := customerService.(orderservice.OrderAdminService); ok {
		admSvc = asAdm
	}
	if shippingService != nil {
		locSvc = shippingService
	}
	return &OrderHandler{
		customerService: customerService,
		shippingService: shippingService,
		locationService: locSvc,
		returnService:   returnService,
		invoiceService:  invSvc,
		adminService:    admSvc,
	}
}

func (oh *OrderHandler) WithCustomerService(customerService orderservice.CustomerOrderService) *OrderHandler {
	if customerService != nil {
		oh.customerService = customerService
	}
	return oh
}

func (oh *OrderHandler) WithInvoiceService(invoiceService orderservice.OrderInvoiceService) *OrderHandler {
	if invoiceService != nil {
		oh.invoiceService = invoiceService
	}
	return oh
}

func (oh *OrderHandler) WithAdminService(adminService orderservice.OrderAdminService) *OrderHandler {
	if adminService != nil {
		oh.adminService = adminService
	}
	return oh
}

func (oh *OrderHandler) WithLocationService(locationService orderservice.LocationService) *OrderHandler {
	if locationService != nil {
		oh.locationService = locationService
	}
	return oh
}

func (oh *OrderHandler) getCustomerService() orderservice.CustomerOrderService {
	return oh.customerService
}

func (oh *OrderHandler) getAdminService() orderservice.OrderAdminService {
	return oh.adminService
}

func (oh *OrderHandler) getRequestContext(ctx *gin.Context, userUUID uuid.UUID) context.Context {
	reqCtx := ctx.Request.Context()
	if userID, ok := middleware.GetUserID(ctx); ok && userID > 0 {
		role, _ := middleware.GetUserRole(ctx)
		customer := orderservice.NewOrderCustomer(userID, userUUID, "", "", role, 1)
		reqCtx = orderservice.WithOrderCustomer(reqCtx, customer)
	}
	return reqCtx
}

// CreateOrder godoc
// @Summary      Create order
// @Description  Place a new order for current user
// @Tags         Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body v1dto.CreateOrderInput true "Order creation payload"
// @Success      201 {object} utils.APIResponse{data=v1dto.OrderDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders [post]
func (oh *OrderHandler) CreateOrder(ctx *gin.Context) {

	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var input v1dto.CreateOrderInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if headerKey := strings.TrimSpace(ctx.GetHeader("X-Idempotency-Key")); headerKey != "" {
		input.IdempotencyKey = headerKey
	} else if headerKey := strings.TrimSpace(ctx.GetHeader("Idempotency-Key")); headerKey != "" {
		input.IdempotencyKey = headerKey
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)

	orderDTO, err := oh.getCustomerService().CreateOrder(reqCtx, userUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Order created successfully", orderDTO)
}

// GetMyOrders godoc
// @Summary      Get my orders
// @Description  Retrieve paginated orders for the authenticated user
// @Tags         Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        search query string false "Search keyword"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Param        order_by query string false "Order by field" Enums(order_created_at, order_total_amount)
// @Param        sort query string false "Sort order" Enums(asc, desc)
// @Param        status query int false "Order status" Enums(1, 2, 3, 4, 5, 6)
// @Success      200 {object} utils.APIResponse{data=[]v1dto.OrderDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders [get]
func (oh *OrderHandler) GetMyOrders(ctx *gin.Context) {
	var input v1dto.GetOrdersParam
	if err := ctx.ShouldBindQuery(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	orders, totalOrders, err := oh.getCustomerService().GetMyOrders(reqCtx, userUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	paginationResponse := utils.NewPaginationResponse(orders, input.Page, input.Limit, totalOrders)

	utils.ResponseSuccess(ctx, http.StatusOK, "Get my orders successfully", paginationResponse)
}

// GetOrderByUUID godoc
// @Summary      Get order by UUID
// @Description  Retrieve detailed order information by UUID
// @Tags         Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.OrderDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Failure      404 {object} utils.ErrorResponse
// @Router       /orders/{uuid} [get]
func (oh *OrderHandler) GetOrderByUUID(ctx *gin.Context) {
	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	orderDTO, err := oh.getCustomerService().GetOrderByUUID(reqCtx, userUUID, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Get order by UUID successfully", orderDTO)
}

// GetAllOrders godoc
// @Summary      Get all orders
// @Description  Retrieve all orders with pagination and filtering (Admin/Moderator)
// @Tags         Admin - Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        search query string false "Search keyword"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Param        order_by query string false "Order by field" Enums(order_created_at, order_total_amount)
// @Param        sort query string false "Sort order" Enums(asc, desc)
// @Param        status query int false "Order status" Enums(1, 2, 3, 4, 5, 6)
// @Success      200 {object} utils.APIResponse{data=[]v1dto.OrderDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/orders [get]
func (oh *OrderHandler) GetAllOrders(ctx *gin.Context) {
	var input v1dto.GetOrdersParam
	if err := ctx.ShouldBindQuery(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orders, totalOrders, err := oh.getAdminService().GetAllOrders(ctx.Request.Context(), input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	paginationResponse := utils.NewPaginationResponse(orders, input.Page, input.Limit, totalOrders)

	utils.ResponseSuccess(ctx, http.StatusOK, "Get all orders successfully", paginationResponse)
}

// UpdateOrderStatus godoc
// @Summary      Update order status
// @Description  Update order status by UUID (Admin/Moderator)
// @Tags         Admin - Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Param        body body v1dto.UpdateOrderStatusInput true "Status payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.OrderDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/orders/{uuid} [patch]
func (oh *OrderHandler) UpdateOrderStatus(ctx *gin.Context) {
	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	var input v1dto.UpdateOrderStatusInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	updatedOrderDTO, err := oh.getAdminService().UpdateOrderStatus(ctx.Request.Context(), orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order status updated successfully", updatedOrderDTO)
}

// CancelOrder godoc
// @Summary      Cancel order
// @Description  Cancel an order before shipping
// @Tags         Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders/{uuid}/cancel [patch]
func (oh *OrderHandler) CancelOrder(ctx *gin.Context) {
	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var body struct {
		Reason *string `json:"reason"`
	}
	_ = ctx.ShouldBindJSON(&body)

	reqCtx := oh.getRequestContext(ctx, userUUID)
	cancelledOrderDTO, err := oh.getCustomerService().CancelOrder(reqCtx, userUUID, orderUUID, body.Reason)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order canceled successfully", cancelledOrderDTO)
}

// UpdateOrderShippingInfo godoc
// @Summary      Update order shipping info
// @Description  Update recipient name, phone, address, and notes for an order
// @Tags         Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Param        body body v1dto.UpdateOrderShippingInfoInput true "Shipping info payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.OrderDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders/{uuid}/shipping-info [put]
func (oh *OrderHandler) UpdateOrderShippingInfo(ctx *gin.Context) {
	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var input v1dto.UpdateOrderShippingInfoInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	updatedOrderDTO, err := oh.getCustomerService().UpdateOrderShippingInfo(reqCtx, userUUID, orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Shipping info updated successfully", updatedOrderDTO)
}

// UpdateOrderPaymentStatus godoc
// @Summary      Update order payment status
// @Description  Update payment status of an order (Admin/Moderator)
// @Tags         Admin - Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Param        body body v1dto.UpdateOrderPaymentStatusInput true "Payment status payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.OrderDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/orders/{uuid}/payment-status [patch]
func (oh *OrderHandler) UpdateOrderPaymentStatus(ctx *gin.Context) {
	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var input v1dto.UpdateOrderPaymentStatusInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	updatedOrderDTO, err := oh.getCustomerService().UpdateOrderPaymentStatus(reqCtx, userUUID, orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Payment status updated successfully", updatedOrderDTO)
}

// DownloadInvoice godoc
// @Summary      Download order invoice PDF
// @Description  Generate and download or view order invoice in PDF format
// @Tags         Orders
// @Produce      application/pdf
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Param        action query string false "Action (download or inline)" Enums(download, inline)
// @Success      200 {file} binary
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders/{uuid}/invoice [get]
func (oh *OrderHandler) DownloadInvoice(ctx *gin.Context) {
	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	pdfBytes, fileName, err := oh.invoiceService.GetOrderInvoicePDF(reqCtx, userUUID, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	disposition := "inline"
	if ctx.Query("action") == "download" {
		disposition = "attachment"
	}

	ctx.Header("Content-Disposition", fmt.Sprintf("%s; filename=\"%s\"", disposition, fileName))
	ctx.Header("Content-Type", "application/pdf")
	ctx.Header("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))
	ctx.Header("Cache-Control", "private, max-age=86400")
	ctx.Data(http.StatusOK, "application/pdf", pdfBytes)
}

// SHIPPING

// CreateShipping godoc
// @Summary      Create shipping order
// @Description  Create third-party delivery order for shipping (Admin/Moderator)
// @Tags         Admin - Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Param        body body v1dto.CreateShippingInput true "Shipping order payload"
// @Success      201 {object} utils.APIResponse{data=v1dto.ShippingResponseDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/orders/{uuid}/shipping [post]
func (oh *OrderHandler) CreateShipping(ctx *gin.Context) {
	var params v1dto.CreateShippingParams
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	var input v1dto.CreateShippingInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	res, err := oh.shippingService.CreateShipping(ctx.Request.Context(), orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Shipping order created successfully", res)
}

// GetOrderTimeline godoc
// @Summary      Get order tracking timeline
// @Description  Get chronological event tracking timeline for an order
// @Tags         Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.OrderTimelineResponseDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders/{uuid}/timeline [get]
func (oh *OrderHandler) GetOrderTimeline(ctx *gin.Context) {
	var params v1dto.GetOrderTimeLineParams
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized"))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	timeline, err := oh.shippingService.GetOrderTimeline(reqCtx, userUUID, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order timeline retrieved successfully", timeline)
}

// HandleShippingWebhook godoc
// @Summary      GHN shipping webhook
// @Description  Receive and process tracking updates from GHN delivery webhook
// @Tags         Shipping
// @Accept       json
// @Produce      json
// @Param        body body v1dto.GHNWebhookPayload true "GHN webhook payload"
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /shipping/webhook/ghn [post]
func (oh *OrderHandler) HandleShippingWebhook(ctx *gin.Context) {
	rawBody, err := ctx.GetRawData()
	if err != nil {
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeBadRequest, "Failed to read request body", err))
		return
	}

	var payload v1dto.GHNWebhookPayload
	if err := json.Unmarshal(rawBody, &payload); err != nil {
		utils.ResponseError(ctx, utils.WrapError(utils.ErrCodeBadRequest, "Invalid JSON payload", err))
		return
	}

	if payload.OrderCode == "" || payload.Status == "" {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "OrderCode and Status are required"))
		return
	}

	if err := oh.shippingService.HandleShippingWebhook(ctx.Request.Context(), rawBody, payload); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Webhook processed successfully",
	})
}

// CalculateShippingFee godoc
// @Summary      Calculate shipping fee
// @Description  Calculate estimated shipping fee from carrier
// @Tags         Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body v1dto.CalculateShippingFeeInput true "Calculate fee payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.ShippingFeeResponseDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /orders/calculate-fee [post]
func (oh *OrderHandler) CalculateShippingFee(ctx *gin.Context) {
	var input v1dto.CalculateShippingFeeInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	res, err := oh.shippingService.CalculateShippingFee(ctx.Request.Context(), input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Shipping fee calculated successfully", res)
}

// CancelShipping godoc
// @Summary      Cancel shipping order
// @Description  Cancel active shipment with carrier (Admin/Moderator)
// @Tags         Admin - Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Param        body body v1dto.CancelShippingInput false "Cancel reason payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.CancelShippingResponseDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /admin/orders/{uuid}/shipping/cancel [post]
func (oh *OrderHandler) CancelShipping(ctx *gin.Context) {
	var params v1dto.CancelShippingParams
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	var input v1dto.CancelShippingInput
	_ = ctx.ShouldBindJSON(&input)

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	res, err := oh.shippingService.CancelShipping(ctx.Request.Context(), orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Shipping order cancelled successfully", res)
}

// GetPrintBill godoc
// @Summary      Get shipping bill print URL
// @Description  Generate URL to print carrier shipping label/bill (Admin/Moderator)
// @Tags         Admin - Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Router       /admin/orders/{uuid}/shipping/print-bill [get]
func (oh *OrderHandler) GetPrintBill(ctx *gin.Context) {
	var params v1dto.GetPrintBillParams
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	res, err := oh.shippingService.GetPrintBill(ctx.Request.Context(), orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Shipping bill print URLs generated successfully", res)
}

// GetProvinces godoc
// @Summary      Get shipping provinces
// @Description  Get list of provinces from shipping provider
// @Tags         Shipping
// @Produce      json
// @Success      200 {object} utils.APIResponse{data=[]v1dto.ProvinceDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /shipping/provinces [get]
func (oh *OrderHandler) GetProvinces(ctx *gin.Context) {
	if oh.locationService == nil && oh.shippingService == nil {
		utils.ResponseError(ctx, fmt.Errorf("location service unavailable"))
		return
	}
	var (
		provinces []v1dto.ProvinceDTO
		err       error
	)
	if oh.locationService != nil {
		provinces, err = oh.locationService.GetProvinces(ctx.Request.Context())
	} else {
		provinces, err = oh.shippingService.GetProvinces(ctx.Request.Context())
	}
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Provinces retrieved successfully", provinces)
}

// GetDistricts godoc
// @Summary      Get shipping districts
// @Description  Get list of districts by province ID
// @Tags         Shipping
// @Produce      json
// @Param        province_id query int true "Province ID"
// @Success      200 {object} utils.APIResponse{data=[]v1dto.DistrictDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /shipping/districts [get]
func (oh *OrderHandler) GetDistricts(ctx *gin.Context) {
	var query v1dto.GetDistrictsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		if pidStr := ctx.Param("province_id"); pidStr != "" {
			if pid, parseErr := strconv.Atoi(pidStr); parseErr == nil && pid > 0 {
				query.ProvinceID = pid
			} else {
				utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
				return
			}
		} else {
			utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
			return
		}
	}

	if oh.locationService == nil && oh.shippingService == nil {
		utils.ResponseError(ctx, fmt.Errorf("location service unavailable"))
		return
	}
	var (
		districts []v1dto.DistrictDTO
		err       error
	)
	if oh.locationService != nil {
		districts, err = oh.locationService.GetDistricts(ctx.Request.Context(), query.ProvinceID)
	} else {
		districts, err = oh.shippingService.GetDistricts(ctx.Request.Context(), query.ProvinceID)
	}
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Districts retrieved successfully", districts)
}

// GetWards godoc
// @Summary      Get shipping wards
// @Description  Get list of wards by district ID
// @Tags         Shipping
// @Produce      json
// @Param        district_id query int true "District ID"
// @Success      200 {object} utils.APIResponse{data=[]v1dto.WardDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Router       /shipping/wards [get]
func (oh *OrderHandler) GetWards(ctx *gin.Context) {
	var query v1dto.GetWardsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		if didStr := ctx.Param("district_id"); didStr != "" {
			if did, parseErr := strconv.Atoi(didStr); parseErr == nil && did > 0 {
				query.DistrictID = did
			} else {
				utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
				return
			}
		} else {
			utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
			return
		}
	}

	if oh.locationService == nil && oh.shippingService == nil {
		utils.ResponseError(ctx, fmt.Errorf("location service unavailable"))
		return
	}
	var (
		wards []v1dto.WardDTO
		err   error
	)
	if oh.locationService != nil {
		wards, err = oh.locationService.GetWards(ctx.Request.Context(), query.DistrictID)
	} else {
		wards, err = oh.shippingService.GetWards(ctx.Request.Context(), query.DistrictID)
	}
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Wards retrieved successfully", wards)
}

// RequestOrderReturn godoc
// @Summary      Request order return / refund
// @Description  Submit a request to return an order with reason and proof files
// @Tags         Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Param        request body v1dto.CreateOrderReturnInput true "Return request body"
// @Success      201 {object} utils.APIResponse{data=v1dto.OrderReturnDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders/{uuid}/return [post]
func (oh *OrderHandler) RequestOrderReturn(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	var input v1dto.CreateOrderReturnInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if len(input.Images) > 5 {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "You can upload a maximum of 5 images"))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	returnDTO, err := oh.returnService.RequestOrderReturn(reqCtx, userUUID, orderUUID, input, input.Images, input.VideoURL)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Order return request created successfully", returnDTO)
}

// GetOrderReturnDetails godoc
// @Summary      Get order return details
// @Description  Retrieve return request details for an order
// @Tags         Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.OrderReturnDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders/{uuid}/return [get]
func (oh *OrderHandler) GetOrderReturnDetails(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	res, err := oh.returnService.GetOrderReturnDetails(reqCtx, userUUID, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order return details retrieved successfully", res)
}

// GetMyOrderReturns godoc
// @Summary      Get user return requests
// @Description  Get paginated list of return requests for current user
// @Tags         Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        status query int false "Return status (1: Pending, 2: Approved, 3: Rejected, 4: Completed, 5: Cancelled)" Enums(1, 2, 3, 4, 5)
// @Param        search query string false "Search keyword"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Success      200 {object} utils.APIResponse{data=[]v1dto.OrderReturnDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders/returns [get]
func (oh *OrderHandler) GetMyOrderReturns(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var query v1dto.GetMyOrderReturnsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	returns, total, err := oh.returnService.GetMyOrderReturns(reqCtx, userUUID, query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	page := query.Page
	if page < 1 {
		page = 1
	}
	limit := query.Limit
	if limit < 1 {
		limit = 10
	}

	paginationResponse := utils.NewPaginationResponse(returns, page, limit, int32(total))

	utils.ResponseSuccess(ctx, http.StatusOK, "Return requests retrieved successfully", paginationResponse)
}



// AdminGetReturns godoc
// @Summary      Get all return requests
// @Description  Retrieve all return requests with pagination and filtering (Admin/Moderator)
// @Tags         Admin - Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        status query int false "Return status (1: Pending, 2: Approved, 3: Rejected, 4: Completed, 5: Cancelled)" Enums(1, 2, 3, 4, 5)
// @Param        search query string false "Search keyword"
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(10)
// @Success      200 {object} utils.APIResponse{data=[]v1dto.OrderReturnDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/returns [get]
func (oh *OrderHandler) AdminGetReturns(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var query v1dto.AdminGetReturnsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	returns, total, err := oh.returnService.AdminGetReturns(reqCtx, userUUID, query)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	page := query.Page
	if page < 1 {
		page = 1
	}
	limit := query.Limit
	if limit < 1 {
		limit = 10
	}

	paginationResponse := utils.NewPaginationResponse(returns, page, limit, int32(total))

	utils.ResponseSuccess(ctx, http.StatusOK, "Return requests retrieved successfully", paginationResponse)
}

// AdminGetReturnDetail godoc
// @Summary      Get return request detail
// @Description  Get return request by return UUID (Admin/Moderator)
// @Tags         Admin - Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Return UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.OrderReturnDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/returns/{uuid} [get]
func (oh *OrderHandler) AdminGetReturnDetail(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var param v1dto.ReturnUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	returnUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid return UUID"))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	res, err := oh.returnService.AdminGetReturnDetail(reqCtx, userUUID, returnUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Return request details retrieved successfully", res)
}

// AdminUpdateReturnStatus godoc
// @Summary      Update return request status
// @Description  Approve, reject, or update return status (Admin/Moderator)
// @Tags         Admin - Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Return UUID"
// @Param        body body v1dto.AdminUpdateReturnInput true "Update return status payload"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/returns/{uuid} [patch]
func (oh *OrderHandler) AdminUpdateReturnStatus(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var param v1dto.ReturnUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	returnUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid return UUID"))
		return
	}

	var input v1dto.AdminUpdateReturnInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	res, err := oh.returnService.AdminUpdateReturnStatus(reqCtx, userUUID, returnUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Return request status updated successfully", res)
}

// CancelOrderReturn godoc
// @Summary      Cancel order return request
// @Description  Cancel an existing return request
// @Tags         Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Param        body body v1dto.CancelOrderReturnInput false "Cancel reason"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders/{uuid}/return/cancel [patch]
func (oh *OrderHandler) CancelOrderReturn(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	var input v1dto.CancelOrderReturnInput
	_ = ctx.ShouldBindJSON(&input)

	reqCtx := oh.getRequestContext(ctx, userUUID)
	res, err := oh.returnService.CancelOrderReturn(reqCtx, userUUID, orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order return cancelled successfully", res)
}

// SubmitReturnShipping godoc
// @Summary      Submit return shipment info
// @Description  Submit courier and tracking code for returned package
// @Tags         Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Param        body body v1dto.SubmitReturnShippingInput true "Return shipment info"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders/{uuid}/return/shipping-info [post]
func (oh *OrderHandler) SubmitReturnShipping(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	var input v1dto.SubmitReturnShippingInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	res, err := oh.returnService.SubmitReturnShipping(reqCtx, userUUID, orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Return shipping information submitted successfully", res)
}

// UpdateReturnBankAccount godoc
// @Summary      Update return refund bank account
// @Description  Update bank account information for return refund
// @Tags         Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Param        body body v1dto.UpdateReturnBankAccountInput true "Bank account payload"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders/{uuid}/return/bank-account [put]
func (oh *OrderHandler) UpdateReturnBankAccount(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	var input v1dto.UpdateReturnBankAccountInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	res, err := oh.returnService.UpdateReturnBankAccount(reqCtx, userUUID, orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Return refund bank account updated successfully", res)
}

// AdminProcessReturnRefund godoc
// @Summary      Process return refund
// @Description  Process refund payment for approved return request (Admin/Moderator)
// @Tags         Admin - Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Return UUID"
// @Param        body body v1dto.AdminReturnRefundInput false "Refund process payload"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/returns/{uuid}/refund [post]
func (oh *OrderHandler) AdminProcessReturnRefund(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	var param v1dto.ReturnUUIDParam
	if err := ctx.ShouldBindUri(&param); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	returnUUID, err := uuid.Parse(param.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid return UUID"))
		return
	}

	var input v1dto.AdminReturnRefundInput
	_ = ctx.ShouldBindJSON(&input)

	reqCtx := oh.getRequestContext(ctx, userUUID)
	res, err := oh.returnService.AdminProcessReturnRefund(reqCtx, userUUID, returnUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Return refund processed successfully", res)
}

// ConfirmOrderDelivery godoc
// @Summary      Confirm order delivery
// @Description  Customer confirms receipt of order package
// @Tags         Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Success      200 {object} utils.APIResponse
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders/{uuid}/confirm-delivery [post]
func (oh *OrderHandler) ConfirmOrderDelivery(ctx *gin.Context) {
	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized"))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	res, err := oh.shippingService.ConfirmOrderDelivery(reqCtx, userUUID, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order delivery confirmed successfully", res)
}

// SyncOrderTracking godoc
// @Summary      Sync order tracking
// @Description  Manually trigger sync of tracking details from shipping carrier
// @Tags         Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        uuid path string true "Order UUID"
// @Success      200 {object} utils.APIResponse{data=v1dto.SyncTrackingResponseDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /orders/{uuid}/sync-tracking [post]
func (oh *OrderHandler) SyncOrderTracking(ctx *gin.Context) {
	var params v1dto.GetOrderByUUIDParam
	if err := ctx.ShouldBindUri(&params); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orderUUID, err := uuid.Parse(params.UUID)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "Invalid order UUID"))
		return
	}

	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized"))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	result, err := oh.shippingService.SyncOrderTracking(reqCtx, userUUID, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Sync tracking successfully", result)
}

// AdminSyncActiveTracking godoc
// @Summary      Batch sync active shipping tracking
// @Description  Batch synchronize all currently active shipments with carrier (Admin/Moderator)
// @Tags         Admin - Orders
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Success      200 {object} utils.APIResponse{data=v1dto.SyncActiveShippingStatsDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/orders/sync-active-tracking [post]
func (oh *OrderHandler) AdminSyncActiveTracking(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized"))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	stats, err := oh.shippingService.AdminSyncActiveShippingOrders(reqCtx, userUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Batch active shipping tracking synchronized successfully", stats)
}

// BulkUpdateOrderStatus godoc
// @Summary      Bulk update order statuses
// @Description  Update the status of multiple orders at once (Admin/Moderator)
// @Tags         Admin - Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body v1dto.BulkUpdateOrderStatusInput true "Bulk update payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.BulkOperationResultDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/orders/bulk-status [post]
func (oh *OrderHandler) BulkUpdateOrderStatus(ctx *gin.Context) {
	var input v1dto.BulkUpdateOrderStatusInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	result, err := oh.adminService.BulkUpdateStatus(ctx.Request.Context(), input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Bulk order status updated", result)
}

// BulkCancelOrders godoc
// @Summary      Bulk cancel orders
// @Description  Cancel multiple orders with inventory rollback (Admin/Moderator)
// @Tags         Admin - Orders
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Security     ApiKeyAuth
// @Param        body body v1dto.BulkCancelOrdersInput true "Bulk cancel payload"
// @Success      200 {object} utils.APIResponse{data=v1dto.BulkOperationResultDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      401 {object} utils.ErrorResponse
// @Router       /admin/orders/bulk-cancel [post]
func (oh *OrderHandler) BulkCancelOrders(ctx *gin.Context) {
	userUUID, err := middleware.GetUserUUID(ctx)
	if err != nil {
		utils.ResponseError(ctx, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized"))
		return
	}

	var input v1dto.BulkCancelOrdersInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	reqCtx := oh.getRequestContext(ctx, userUUID)
	result, err := oh.adminService.BulkCancel(reqCtx, userUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Bulk orders cancelled", result)
}
