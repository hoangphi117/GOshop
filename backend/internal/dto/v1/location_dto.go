package v1dto

type GetDistrictsQuery struct {
	ProvinceID int `form:"province_id" binding:"required,gt=0"`
}

type GetWardsQuery struct {
	DistrictID int `form:"district_id" binding:"required,gt=0"`
}

type ProvinceDTO struct {
	ProvinceID   int    `json:"province_id"`
	ProvinceName string `json:"province_name"`
	Code         string `json:"code,omitempty"`
}

type DistrictDTO struct {
	DistrictID   int    `json:"district_id"`
	ProvinceID   int    `json:"province_id"`
	DistrictName string `json:"district_name"`
	Code         string `json:"code,omitempty"`
}

type WardDTO struct {
	WardCode   string `json:"ward_code"`
	DistrictID int    `json:"district_id"`
	WardName   string `json:"ward_name"`
}
