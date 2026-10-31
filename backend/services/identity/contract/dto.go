package contract

import (
	"time"

	"github.com/google/uuid"
)

// InternalUserDTO is the public DTO returned by Identity Service for other services
type InternalUserDTO struct {
	UserID        int32     `json:"user_id"`
	UserUUID      uuid.UUID `json:"user_uuid"`
	UserFullname  string    `json:"user_fullname"`
	UserEmail     string    `json:"user_email"`
	UserAvatar    string    `json:"user_avatar"`
	UserAvatarUrl *string   `json:"user_avatar_url"`
	UserStatus    int32     `json:"user_status"`
	UserLevel     int32     `json:"user_level"`
}

// InternalBatchUsersResponse is the response for batch user queries
type InternalBatchUsersResponse struct {
	Users map[int32]InternalUserDTO `json:"users"`
}

// AddressDTO represents an address returned by Identity Service
type AddressDTO struct {
	UUID         string     `json:"uuid"`
	Label        string     `json:"label,omitempty"`
	Name         string     `json:"name"`
	Phone        string     `json:"phone"`
	Address      string     `json:"address"`
	ProvinceID   *int32     `json:"province_id"`
	ProvinceName string     `json:"province_name,omitempty"`
	DistrictID   *int32     `json:"district_id"`
	DistrictName string     `json:"district_name,omitempty"`
	WardCode     *string    `json:"ward_code"`
	WardName     string     `json:"ward_name,omitempty"`
	FullAddress  string     `json:"full_address"`
	IsDefault    bool       `json:"is_default"`
	AddressType  string     `json:"address_type"`
	DeliveryNote *string    `json:"delivery_note"`
	CreatedAt    *time.Time `json:"created_at,omitempty"`
}
