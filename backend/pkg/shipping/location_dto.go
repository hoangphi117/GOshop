package shipping

// Administrative hierarchy DTOs returned by the GHN provider.
//
// They live here rather than in the monolith's internal/dto package so that
// pkg/shipping stays reusable by the extracted services (order-service keeps
// its own copies that it maps from these).

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
