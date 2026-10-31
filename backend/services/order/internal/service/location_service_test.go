package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"goshop/internal/testutils"
	"goshop/pkg/shipping"
	v1dto "goshop/services/order/internal/dto"
	orderservice "goshop/services/order/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocationService_GetProvinces(t *testing.T) {
	ctx := context.Background()

	t.Run("CacheHit_ReturnsCachedProvincesWithoutProviderCall", func(t *testing.T) {
		cachedProvinces := []v1dto.ProvinceDTO{
			{ProvinceID: 201, ProvinceName: "Hà Nội", Code: "HN"},
			{ProvinceID: 202, ProvinceName: "Hồ Chí Minh", Code: "HCM"},
		}

		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				assert.Equal(t, "shipping:ghn:provinces", key)
				pDest, ok := dest.(*[]v1dto.ProvinceDTO)
				require.True(t, ok)
				*pDest = cachedProvinces
				return nil
			},
		}

		svc := orderservice.NewLocationService(nil, mockCache)
		res, err := svc.GetProvinces(ctx)

		require.NoError(t, err)
		assert.Equal(t, cachedProvinces, res)
	})

	t.Run("CacheMiss_FetchesFromGHNAndCaches", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/shiip/public-api/master-data/province", r.URL.Path)
			resp := map[string]any{
				"code":    200,
				"message": "Success",
				"data": []map[string]any{
					{"ProvinceID": 201, "ProvinceName": "Hà Nội", "Code": "HN"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		ghnAdapter := shipping.NewGHNAdapter("test-token", 123, server.URL)
		factory := shipping.NewShippingFactory(ghnAdapter)

		cacheSetCalled := false
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return nil // simulate cache miss
			},
			SetFunc: func(key string, value any, ttl time.Duration) error {
				if key == "shipping:ghn:provinces" {
					cacheSetCalled = true
					assert.Equal(t, 24*time.Hour, ttl)
				}
				return nil
			},
		}

		svc := orderservice.NewLocationService(factory, mockCache)
		res, err := svc.GetProvinces(ctx)

		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, 201, res[0].ProvinceID)
		assert.Equal(t, "Hà Nội", res[0].ProvinceName)
		assert.True(t, cacheSetCalled)
	})

	t.Run("NoProviderFactory_ReturnsError", func(t *testing.T) {
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return nil
			},
		}
		svc := orderservice.NewLocationService(nil, mockCache)
		res, err := svc.GetProvinces(ctx)

		require.Error(t, err)
		assert.Nil(t, res)
	})

	t.Run("GHNError_ReturnsError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]any{
				"code":    400,
				"message": "Bad Request",
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		ghnAdapter := shipping.NewGHNAdapter("test-token", 123, server.URL)
		factory := shipping.NewShippingFactory(ghnAdapter)

		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return nil
			},
		}

		svc := orderservice.NewLocationService(factory, mockCache)
		res, err := svc.GetProvinces(ctx)

		require.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestLocationService_GetDistricts(t *testing.T) {
	ctx := context.Background()

	t.Run("CacheHit_ReturnsCachedDistricts", func(t *testing.T) {
		cachedDistricts := []v1dto.DistrictDTO{
			{DistrictID: 1442, ProvinceID: 201, DistrictName: "Ba Đình", Code: "BD"},
		}

		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				assert.Equal(t, "shipping:ghn:districts:201", key)
				dDest, ok := dest.(*[]v1dto.DistrictDTO)
				require.True(t, ok)
				*dDest = cachedDistricts
				return nil
			},
		}

		svc := orderservice.NewLocationService(nil, mockCache)
		res, err := svc.GetDistricts(ctx, 201)

		require.NoError(t, err)
		assert.Equal(t, cachedDistricts, res)
	})

	t.Run("CacheMiss_FetchesFromGHNAndCachesWith30DayTTL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/shiip/public-api/master-data/district", r.URL.Path)
			resp := map[string]any{
				"code":    200,
				"message": "Success",
				"data": []map[string]any{
					{"DistrictID": 1442, "ProvinceID": 201, "DistrictName": "Ba Đình", "Code": "BD"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		ghnAdapter := shipping.NewGHNAdapter("test-token", 123, server.URL)
		factory := shipping.NewShippingFactory(ghnAdapter)

		cacheSetCalled := false
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return nil
			},
			SetFunc: func(key string, value any, ttl time.Duration) error {
				if key == "shipping:ghn:districts:201" {
					cacheSetCalled = true
					assert.Equal(t, 30*24*time.Hour, ttl)
				}
				return nil
			},
		}

		svc := orderservice.NewLocationService(factory, mockCache)
		res, err := svc.GetDistricts(ctx, 201)

		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, 1442, res[0].DistrictID)
		assert.Equal(t, "Ba Đình", res[0].DistrictName)
		assert.True(t, cacheSetCalled)
	})

	t.Run("GHNError_ReturnsError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]any{
				"code":    500,
				"message": "Internal error",
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		ghnAdapter := shipping.NewGHNAdapter("test-token", 123, server.URL)
		factory := shipping.NewShippingFactory(ghnAdapter)

		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return nil
			},
		}

		svc := orderservice.NewLocationService(factory, mockCache)
		res, err := svc.GetDistricts(ctx, 201)

		require.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestLocationService_GetWards(t *testing.T) {
	ctx := context.Background()

	t.Run("CacheHit_ReturnsCachedWards", func(t *testing.T) {
		cachedWards := []v1dto.WardDTO{
			{DistrictID: 1442, WardName: "Cống Vị", WardCode: "1A0101"},
		}

		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				assert.Equal(t, "shipping:ghn:wards:1442", key)
				wDest, ok := dest.(*[]v1dto.WardDTO)
				require.True(t, ok)
				*wDest = cachedWards
				return nil
			},
		}

		svc := orderservice.NewLocationService(nil, mockCache)
		res, err := svc.GetWards(ctx, 1442)

		require.NoError(t, err)
		assert.Equal(t, cachedWards, res)
	})

	t.Run("CacheMiss_FetchesFromGHNAndCachesWith30DayTTL", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/shiip/public-api/master-data/ward")
			assert.Equal(t, "1442", r.URL.Query().Get("district_id"))
			resp := map[string]any{
				"code":    200,
				"message": "Success",
				"data": []map[string]any{
					{"DistrictID": 1442, "WardName": "Cống Vị", "WardCode": "1A0101"},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		ghnAdapter := shipping.NewGHNAdapter("test-token", 123, server.URL)
		factory := shipping.NewShippingFactory(ghnAdapter)

		cacheSetCalled := false
		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return nil
			},
			SetFunc: func(key string, value any, ttl time.Duration) error {
				if key == "shipping:ghn:wards:1442" {
					cacheSetCalled = true
					assert.Equal(t, 30*24*time.Hour, ttl)
				}
				return nil
			},
		}

		svc := orderservice.NewLocationService(factory, mockCache)
		res, err := svc.GetWards(ctx, 1442)

		require.NoError(t, err)
		require.Len(t, res, 1)
		assert.Equal(t, "Cống Vị", res[0].WardName)
		assert.Equal(t, "1A0101", res[0].WardCode)
		assert.True(t, cacheSetCalled)
	})

	t.Run("GHNError_ReturnsError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := map[string]any{
				"code":    500,
				"message": "Internal error",
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		ghnAdapter := shipping.NewGHNAdapter("test-token", 123, server.URL)
		factory := shipping.NewShippingFactory(ghnAdapter)

		mockCache := &testutils.MockRedisCacheService{
			GetFunc: func(key string, dest any) error {
				return nil
			},
		}

		svc := orderservice.NewLocationService(factory, mockCache)
		res, err := svc.GetWards(ctx, 1442)

		require.Error(t, err)
		assert.Nil(t, res)
	})
}
