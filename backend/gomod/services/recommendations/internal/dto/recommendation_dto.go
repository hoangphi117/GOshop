package dto

// RecommendationItem is one product card in a recommendation shelf.
//
// Keys are snake_case and match the catalog's public product shape
// (uuid, name, slug, image_url, category_name, rating) so the storefront can
// reuse the same card component instead of mapping a second vocabulary.
type RecommendationItem struct {
	UUID         string  `json:"uuid"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Description  string  `json:"description,omitempty"`
	Price        float64 `json:"price"`
	Stock        int32   `json:"stock"`
	SKU          string  `json:"sku,omitempty"`
	CategoryUUID string  `json:"category_uuid,omitempty"`
	CategoryName string  `json:"category_name,omitempty"`
	ImageURL     string  `json:"image_url,omitempty"`
	AvgRating    float64 `json:"rating"`
	ReviewCount  int32   `json:"reviews_count"`
	UnitsSold    int64   `json:"units_sold,omitempty"`
	// Similarity is set only on vector-backed shelves (personalized, similar),
	// so a client can explain a ranking without a second call.
	Similarity float64 `json:"similarity,omitempty"`
}

// Shelf is the response of GET /api/v1/recommendations.
//
// Source is always present: a promoted fallback must never be presented as
// measured best sellers, so the client can label the shelf honestly.
type Shelf struct {
	Source string               `json:"source"`
	Items  []RecommendationItem `json:"items"`
	// Total is the number of matches across all pages, set by ranked search
	// only. A page-number row cannot show the last page without it, and
	// deriving it from a page boundary reveals the pagination one page at a
	// time instead of up front.
	Total int `json:"total,omitempty"`
}
