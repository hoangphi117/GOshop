package dto

import (
	"fmt"
	"goshop/services/identity/internal/db/sqlc"
	"goshop/shared/pkg/utils"
)

type AddressUUIDParam struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

type CreateAddressInput struct {
	Label        string `json:"label"      binding:"required,max=50"`
	Name         string `json:"name"       binding:"required,min=2,max=100"`
	Phone        string `json:"phone"      binding:"required,min=9,max=20"`
	Address      string `json:"address"    binding:"required,min=5"`
	ProvinceID   int    `json:"province_id"   binding:"required,gt=0"`
	ProvinceName string `json:"province_name" binding:"required"`
	DistrictID   int    `json:"district_id"   binding:"required,gt=0"`
	DistrictName string `json:"district_name" binding:"required"`
	WardCode     string `json:"ward_code"     binding:"required"`
	WardName     string `json:"ward_name"     binding:"required"`
	IsDefault    bool   `json:"is_default"`
}

type AddressDTO struct {
	UUID         string `json:"uuid"`
	Label        string `json:"label"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Address      string `json:"address"`
	ProvinceID   *int   `json:"province_id,omitempty"`
	ProvinceName string `json:"province_name,omitempty"`
	DistrictID   *int   `json:"district_id,omitempty"`
	DistrictName string `json:"district_name,omitempty"`
	WardCode     string `json:"ward_code,omitempty"`
	WardName     string `json:"ward_name,omitempty"`
	FullAddress  string `json:"full_address"`
	IsDefault    bool   `json:"is_default"`
}

type UpdateAddressInput struct {
	Label        *string `json:"label"         binding:"omitempty,max=50"`
	Name         *string `json:"name"          binding:"omitempty,min=2,max=100"`
	Phone        *string `json:"phone"         binding:"omitempty,min=9,max=20"`
	Address      *string `json:"address"       binding:"omitempty,min=5"`
	ProvinceID   *int    `json:"province_id"   binding:"omitempty,gt=0"`
	ProvinceName *string `json:"province_name" binding:"omitempty"`
	DistrictID   *int    `json:"district_id"   binding:"omitempty,gt=0"`
	DistrictName *string `json:"district_name" binding:"omitempty"`
	WardCode     *string `json:"ward_code"     binding:"omitempty"`
	WardName     *string `json:"ward_name"     binding:"omitempty"`
}

func MapToAddressDTO(a sqlc.UserAddress) AddressDTO {
	fullAddress := a.UaAddress
	if a.UaWardName != nil && a.UaDistrictName != nil && a.UaProvinceName != nil {
		fullAddress = fmt.Sprintf("%s, %s, %s, %s", a.UaAddress, *a.UaWardName, *a.UaDistrictName, *a.UaProvinceName)
	}

	return AddressDTO{
		UUID:         a.UaUuid.String(),
		Label:        a.UaLabel,
		Name:         a.UaName,
		Phone:        a.UaPhone,
		Address:      a.UaAddress,
		ProvinceID:   utils.Ptr32ToPtrInt(a.UaProvinceID),
		ProvinceName: utils.NilToString(a.UaProvinceName),
		DistrictID:   utils.Ptr32ToPtrInt(a.UaDistrictID),
		DistrictName: utils.NilToString(a.UaDistrictName),
		WardCode:     utils.NilToString(a.UaWardCode),
		WardName:     utils.NilToString(a.UaWardName),
		FullAddress:  fullAddress,
		IsDefault:    a.UaIsDefault,
	}
}

func MapToAddressDTOs(addresses []sqlc.UserAddress) []AddressDTO {
	result := make([]AddressDTO, len(addresses))
	for i, a := range addresses {
		result[i] = MapToAddressDTO(a)
	}
	return result
}
