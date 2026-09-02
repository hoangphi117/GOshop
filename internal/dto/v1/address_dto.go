package v1dto

import "goshop/internal/db/sqlc"

type AddressUUIDParam struct {
	UUID string `uri:"uuid" binding:"required,uuid"`
}

type CreateAddressInput struct {
	Label     string `json:"label"      binding:"required,max=50"`
	Name      string `json:"name"       binding:"required,min=2,max=100"`
	Phone     string `json:"phone"      binding:"required,min=9,max=20"`
	Address   string `json:"address"    binding:"required,min=5"`
	IsDefault bool   `json:"is_default"`
}

type AddressDTO struct {
	UUID      string `json:"uuid"`
	Label     string `json:"label"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Address   string `json:"address"`
	IsDefault bool   `json:"is_default"`
}

type UpdateAddressInput struct {
	Label   *string `json:"label"    binding:"omitempty,max=50"`
	Name    *string `json:"name"     binding:"omitempty,min=2,max=100"`
	Phone   *string `json:"phone"    binding:"omitempty,min=9,max=20"`
	Address *string `json:"address"  binding:"omitempty,min=5"`
}

func MapToAddressDTO(a sqlc.UserAddress) AddressDTO {
	return AddressDTO{
		UUID:      a.UaUuid.String(),
		Label:     a.UaLabel,
		Name:      a.UaName,
		Phone:     a.UaPhone,
		Address:   a.UaAddress,
		IsDefault: a.UaIsDefault,
	}
}
