package v1handler

import (
	"encoding/json"
	"fmt"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type OrderHandler struct {
	service v1service.OrderService
}

func NewOrderHandler(service v1service.OrderService) *OrderHandler {
	return &OrderHandler{
		service: service,
	}
}

func (oh *OrderHandler) CreateOrder(ctx *gin.Context) {
	var input v1dto.CreateOrderInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	order, orderItems, err := oh.service.CreateOrder(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	orderDTO := v1dto.MapToOrderDTO(order, orderItems)

	utils.ResponseSuccess(ctx, http.StatusCreated, "Order created successfully", orderDTO)
}

func (oh *OrderHandler) GetMyOrders(ctx *gin.Context) {
	var input v1dto.GetOrdersParam
	if err := ctx.ShouldBindQuery(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orders, totalOrders, err := oh.service.GetMyOrders(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	paginationResponse := utils.NewPaginationResponse(orders, input.Page, input.Limit, totalOrders)

	utils.ResponseSuccess(ctx, http.StatusOK, "Get my orders successfully", paginationResponse)
}

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

	order, orderItems, err := oh.service.GetOrderByUUID(ctx, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	orderDTO := v1dto.MapGetOrderByUUIDRowToOrderDTO(order, orderItems)
	utils.ResponseSuccess(ctx, http.StatusOK, "Get order by UUID successfully", orderDTO)
}

func (oh *OrderHandler) GetAllOrders(ctx *gin.Context) {
	var input v1dto.GetOrdersParam
	if err := ctx.ShouldBindQuery(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	orders, totalOrders, err := oh.service.GetAllOrders(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	paginationResponse := utils.NewPaginationResponse(orders, input.Page, input.Limit, totalOrders)

	utils.ResponseSuccess(ctx, http.StatusOK, "Get all orders successfully", paginationResponse)
}

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

	order := input.MapUpdateStatusInputToModel(orderUUID)

	updatedOrder, orderItems, err := oh.service.UpdateOrderStatus(ctx, order)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	updatedOrderDTO := v1dto.MapToOrderDTO(updatedOrder, orderItems)
	utils.ResponseSuccess(ctx, http.StatusOK, "Order status updated successfully", updatedOrderDTO)
}

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

	err = oh.service.CancelOrder(ctx, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order canceled successfully", nil)
}

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

	var input v1dto.UpdateOrderShippingInfoInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	order := input.MapUpdateShippingInputToModel(orderUUID)

	updatedOrder, updatedOrderItems, err := oh.service.UpdateOrderShippingInfo(ctx, order)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	updatedOrderDTO := v1dto.MapToOrderDTO(updatedOrder, updatedOrderItems)

	utils.ResponseSuccess(ctx, http.StatusOK, "Shipping info updated successfully", updatedOrderDTO)
}

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

	var input v1dto.UpdateOrderPaymentStatusInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	updatedOrder, updatedOrderItems, err := oh.service.UpdateOrderPaymentStatus(ctx, orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	updatedOrderDTO := v1dto.MapToOrderDTO(updatedOrder, updatedOrderItems)
	utils.ResponseSuccess(ctx, http.StatusOK, "Payment status updated successfully", updatedOrderDTO)
}

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

	pdfBytes, fileName, err := oh.service.GetOrderInvoicePDF(ctx, orderUUID)
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

	res, err := oh.service.CreateShipping(ctx, orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Shipping order created successfully", res)
}

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

	timeline, err := oh.service.GetOrderTimeline(ctx, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order timeline retrieved successfully", timeline)
}

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

	if err := oh.service.HandleShippingWebhook(ctx, rawBody, payload); err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "Webhook processed successfully",
	})
}

func (oh *OrderHandler) CalculateShippingFee(ctx *gin.Context) {
	var input v1dto.CalculateShippingFeeInput
	if err := ctx.ShouldBindJSON(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	res, err := oh.service.CalculateShippingFee(ctx, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Shipping fee calculated successfully", res)
}

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

	res, err := oh.service.CancelShipping(ctx, orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Shipping order cancelled successfully", res)
}

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

	res, err := oh.service.GetPrintBill(ctx, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Shipping bill print URLs generated successfully", res)
}

func (oh *OrderHandler) GetProvinces(ctx *gin.Context) {
	provinces, err := oh.service.GetProvinces(ctx)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Provinces retrieved successfully", provinces)
}

func (oh *OrderHandler) GetDistricts(ctx *gin.Context) {
	var query v1dto.GetDistrictsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	districts, err := oh.service.GetDistricts(ctx, query.ProvinceID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Districts retrieved successfully", districts)
}

func (oh *OrderHandler) GetWards(ctx *gin.Context) {
	var query v1dto.GetWardsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	wards, err := oh.service.GetWards(ctx, query.DistrictID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}
	utils.ResponseSuccess(ctx, http.StatusOK, "Wards retrieved successfully", wards)
}

func (oh *OrderHandler) RequestOrderReturn(ctx *gin.Context) {
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
	if err := ctx.ShouldBind(&input); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	form, _ := ctx.MultipartForm()
	var images []string
	var videoURL *string
	var savedFiles []string

	uploadDir := "./uploads/returns/"

	if form != nil && form.File["files"] != nil {
		files := form.File["files"]
		if len(files) > 6 {
			utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "You can upload a maximum of 5 images and 1 video"))
			return
		}

		for _, fileHeader := range files {
			fileName, mediaType, err := utils.ValidateAndSaveFile(fileHeader, uploadDir)
			if err != nil {
				oh.cleanupFiles(savedFiles, uploadDir)
				utils.ResponseError(ctx, err)
				return
			}
			savedFiles = append(savedFiles, fileName)
			fileRelativePath := fmt.Sprintf("/uploads/returns/%s", fileName)

			switch mediaType {
			case "image":
				if len(images) >= 5 {
					oh.cleanupFiles(savedFiles, uploadDir)
					utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "You can upload a maximum of 5 images"))
					return
				}
				images = append(images, fileRelativePath)
			case "video":
				if videoURL != nil {
					oh.cleanupFiles(savedFiles, uploadDir)
					utils.ResponseError(ctx, utils.NewError(utils.ErrCodeBadRequest, "You can upload only 1 video"))
					return
				}
				videoURL = &fileRelativePath
			}
		}
	}

	returnDTO, err := oh.service.RequestOrderReturn(ctx, orderUUID, input, images, videoURL)
	if err != nil {
		oh.cleanupFiles(savedFiles, uploadDir)
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusCreated, "Order return request created successfully", returnDTO)
}

func (oh *OrderHandler) GetOrderReturnDetails(ctx *gin.Context) {
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

	res, err := oh.service.GetOrderReturnDetails(ctx, orderUUID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order return details retrieved successfully", res)
}

func (oh *OrderHandler) GetMyOrderReturns(ctx *gin.Context) {
	var query v1dto.GetMyOrderReturnsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	returns, total, err := oh.service.GetMyOrderReturns(ctx, query)
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

func (oh *OrderHandler) cleanupFiles(fileNames []string, uploadDir string) {
	for _, name := range fileNames {
		_ = os.Remove(filepath.Join(uploadDir, name))
	}
}

func (oh *OrderHandler) AdminGetReturns(ctx *gin.Context) {
	var query v1dto.AdminGetReturnsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	returns, total, err := oh.service.AdminGetReturns(ctx, query)
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

func (oh *OrderHandler) AdminUpdateReturnStatus(ctx *gin.Context) {
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

	res, err := oh.service.AdminUpdateReturnStatus(ctx, returnUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Return request status updated successfully", res)
}

func (oh *OrderHandler) CancelOrderReturn(ctx *gin.Context) {
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

	res, err := oh.service.CancelOrderReturn(ctx, orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Order return cancelled successfully", res)
}

func (oh *OrderHandler) SubmitReturnShipping(ctx *gin.Context) {
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

	res, err := oh.service.SubmitReturnShipping(ctx, orderUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Return shipping information submitted successfully", res)
}

func (oh *OrderHandler) AdminProcessReturnRefund(ctx *gin.Context) {
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

	res, err := oh.service.AdminProcessReturnRefund(ctx, returnUUID, input)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Return refund processed successfully", res)
}
