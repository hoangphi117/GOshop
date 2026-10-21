package dto

type BulkItemError struct {
	OrderUUID string `json:"order_uuid"`
	Error     string `json:"error"`
}

type BulkOperationResultDTO struct {
	Total     int             `json:"total"`
	Succeeded int             `json:"succeeded"`
	Failed    int             `json:"failed"`
	Errors    []BulkItemError `json:"errors,omitempty"`
}

type BulkUpdateOrderStatusInput struct {
	OrderUUIDs []string `json:"order_uuids" binding:"required,min=1,dive,uuid"`
	Status     int32    `json:"status" binding:"required,oneof=1 2 3 4 5 6"`
}

type BulkCancelOrdersInput struct {
	OrderUUIDs []string `json:"order_uuids" binding:"required,min=1,dive,uuid"`
	Reason     *string  `json:"reason" binding:"omitempty,max=500"`
}
