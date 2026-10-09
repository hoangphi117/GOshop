package v1handler

import (
	"fmt"
	"net/http"

	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/utils"
	"goshop/internal/validation"

	"github.com/gin-gonic/gin"
)

// LocationHandler manages administrative address hierarchy queries independently from Order domain.
type LocationHandler struct {
	locationService v1service.LocationService
}

func NewLocationHandler(locationService v1service.LocationService) *LocationHandler {
	return &LocationHandler{
		locationService: locationService,
	}
}

// GetProvinces godoc
// @Summary      Get administrative provinces
// @Description  Get list of provinces from location provider (cached)
// @Tags         Location
// @Produce      json
// @Success      200 {object} utils.APIResponse{data=[]v1dto.ProvinceDTO}
// @Failure      500 {object} utils.ErrorResponse
// @Router       /location/provinces [get]
// @Router       /shipping/provinces [get]
func (lh *LocationHandler) GetProvinces(ctx *gin.Context) {
	if lh.locationService == nil {
		utils.ResponseError(ctx, fmt.Errorf("location service unavailable"))
		return
	}

	provinces, err := lh.locationService.GetProvinces(ctx.Request.Context())
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Provinces retrieved successfully", provinces)
}

// GetDistricts godoc
// @Summary      Get administrative districts
// @Description  Get list of districts by province ID (cached)
// @Tags         Location
// @Produce      json
// @Param        province_id query int true "Province ID"
// @Success      200 {object} utils.APIResponse{data=[]v1dto.DistrictDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /location/districts [get]
// @Router       /shipping/districts [get]
func (lh *LocationHandler) GetDistricts(ctx *gin.Context) {
	var query v1dto.GetDistrictsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if lh.locationService == nil {
		utils.ResponseError(ctx, fmt.Errorf("location service unavailable"))
		return
	}

	districts, err := lh.locationService.GetDistricts(ctx.Request.Context(), query.ProvinceID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Districts retrieved successfully", districts)
}

// GetWards godoc
// @Summary      Get administrative wards
// @Description  Get list of wards by district ID (cached)
// @Tags         Location
// @Produce      json
// @Param        district_id query int true "District ID"
// @Success      200 {object} utils.APIResponse{data=[]v1dto.WardDTO}
// @Failure      400 {object} utils.ErrorResponse
// @Failure      500 {object} utils.ErrorResponse
// @Router       /location/wards [get]
// @Router       /shipping/wards [get]
func (lh *LocationHandler) GetWards(ctx *gin.Context) {
	var query v1dto.GetWardsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		utils.ResponseValidator(ctx, validation.HandleValidationErrors(err))
		return
	}

	if lh.locationService == nil {
		utils.ResponseError(ctx, fmt.Errorf("location service unavailable"))
		return
	}

	wards, err := lh.locationService.GetWards(ctx.Request.Context(), query.DistrictID)
	if err != nil {
		utils.ResponseError(ctx, err)
		return
	}

	utils.ResponseSuccess(ctx, http.StatusOK, "Wards retrieved successfully", wards)
}
