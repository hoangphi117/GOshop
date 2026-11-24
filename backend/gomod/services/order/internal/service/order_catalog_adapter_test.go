package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderCatalogAdapter_GetVariantsForCheckout(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()
	variantUUID := uuid.New()

	t.Run("Success - 200 OK returns variants snapshot", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			expectedPath := "/api/v1/products/uuid/" + productUUID.String() + "/variants"
			assert.Equal(t, expectedPath, r.URL.Path)
			assert.Equal(t, http.MethodGet, r.Method)

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			resp := map[string]any{
				"status":  "success",
				"message": "variants fetched",
				"data": []map[string]any{
					{
						"uuid":        variantUUID.String(),
						"sku":         "IP16-BLK-256",
						"title":       "Titanium Black 256GB",
						"price":       32990000.0,
						"status":      "active",
						"attributes": map[string]string{
							"color":   "Black",
							"storage": "256GB",
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		adapter := &orderCatalogAdapter{
			httpClient: server.Client(),
			baseURL:    server.URL,
		}

		variants, err := adapter.GetVariantsForCheckout(ctx, productUUID)
		require.NoError(t, err)
		require.Len(t, variants, 1)
		assert.Equal(t, variantUUID, variants[0].VariantUUID)
		assert.Equal(t, "IP16-BLK-256", variants[0].SKU)
		assert.Equal(t, "Titanium Black 256GB", variants[0].Title)
		assert.Equal(t, 32990000.0, variants[0].Price)
		assert.Equal(t, "active", variants[0].Status)
	})

	t.Run("Not Found - 404 returns nil without error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		adapter := &orderCatalogAdapter{
			httpClient: server.Client(),
			baseURL:    server.URL,
		}

		variants, err := adapter.GetVariantsForCheckout(ctx, productUUID)
		require.NoError(t, err)
		assert.Nil(t, variants)
	})

	t.Run("Server Error - 500 returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		adapter := &orderCatalogAdapter{
			httpClient: server.Client(),
			baseURL:    server.URL,
		}

		variants, err := adapter.GetVariantsForCheckout(ctx, productUUID)
		require.Error(t, err)
		assert.Nil(t, variants)
		assert.Contains(t, err.Error(), "500")
	})

	t.Run("Client timeout - returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(50 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		adapter := &orderCatalogAdapter{
			httpClient: &http.Client{Timeout: 5 * time.Millisecond},
			baseURL:    server.URL,
		}

		variants, err := adapter.GetVariantsForCheckout(ctx, productUUID)
		require.Error(t, err)
		assert.Nil(t, variants)
	})
}
