package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	catalogcontract "goshop/services/catalog/contract"
	"goshop/shared/pkg/utils"

	"github.com/google/uuid"
)

type orderCatalogAdapter struct {
	catalogClient catalogcontract.CatalogClient
	httpClient    *http.Client
	baseURL       string
}

// NewOrderCatalogAdapter creates an adapter exposing CatalogClient as CatalogPort (UUID-only, no local DB fallback).
func NewOrderCatalogAdapter(catalogClient catalogcontract.CatalogClient) CatalogPort {
	catalogURL := utils.GetEnv("CATALOG_SERVICE_URL", "http://catalog:8086")
	if catalogURL == "http://localhost:8086" || catalogURL == "http://127.0.0.1:8086" {
		// Inside docker networks, use cluster service name
		catalogURL = "http://catalog:8086"
	}
	return &orderCatalogAdapter{
		catalogClient: catalogClient,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		baseURL:       catalogURL,
	}
}

func (a *orderCatalogAdapter) GetProductsForCheckout(ctx context.Context, productUUIDs []uuid.UUID) (map[string]CheckoutProductSnapshot, error) {
	if len(productUUIDs) == 0 {
		return make(map[string]CheckoutProductSnapshot), nil
	}

	if a.catalogClient == nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "Catalog client is not initialized")
	}

	uuidStrs := make([]string, len(productUUIDs))
	for i, u := range productUUIDs {
		uuidStrs[i] = u.String()
	}

	products, err := a.catalogClient.GetProductsBatch(ctx, uuidStrs, nil)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch products for checkout from catalog service", err)
	}

	snapshotMap := make(map[string]CheckoutProductSnapshot, len(products))
	for _, p := range products {
		pu, _ := uuid.Parse(p.ProductUUID)
		sku := p.ProductSKU
		img := p.ProductImageURL
		snapshotMap[p.ProductUUID] = CheckoutProductSnapshot{
			ProductID:       p.ProductID,
			ProductUUID:     pu,
			ProductName:     p.ProductName,
			ProductSlug:     p.ProductSlug,
			ProductSku:      &sku,
			ProductImageURL: &img,
			Price:           p.ProductPrice,
			Status:          p.ProductStatus,
		}
	}
	return snapshotMap, nil
}

type catalogVariantsAPIResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    []struct {
		UUID           string            `json:"uuid"`
		SKU            string            `json:"sku"`
		Title          string            `json:"title"`
		Price          float64           `json:"price"`
		CompareAtPrice *float64          `json:"compare_at_price,omitempty"`
		ImageURL       *string           `json:"image_url,omitempty"`
		Status         string            `json:"status"`
		Attributes     map[string]string `json:"attributes"`
	} `json:"data"`
}

func (a *orderCatalogAdapter) GetVariantsForCheckout(ctx context.Context, productUUID uuid.UUID) ([]CheckoutVariantSnapshot, error) {
	reqURL := fmt.Sprintf("%s/api/v1/products/uuid/%s/variants", a.baseURL, productUUID.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog variants request failed with status: %d", resp.StatusCode)
	}

	var apiResp catalogVariantsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	results := make([]CheckoutVariantSnapshot, len(apiResp.Data))
	for i, v := range apiResp.Data {
		vUUID, _ := uuid.Parse(v.UUID)
		results[i] = CheckoutVariantSnapshot{
			VariantUUID:    vUUID,
			SKU:            v.SKU,
			Title:          v.Title,
			Price:          v.Price,
			CompareAtPrice: v.CompareAtPrice,
			ImageURL:       v.ImageURL,
			Status:         v.Status,
		}
	}
	return results, nil
}
