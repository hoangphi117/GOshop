package service

import (
	"context"
	"fmt"
	"time"

	v1dto "goshop/services/order/internal/dto"
	"goshop/pkg/utils"
	"goshop/pkg/cache"
	"goshop/pkg/shipping"

	"golang.org/x/sync/singleflight"
)

const locationCacheTTL = 30 * 24 * time.Hour

type locationService struct {
	shippingFactory *shipping.ShippingFactory
	cache           cache.RedisCacheService
	sf              singleflight.Group
}

// NewLocationService creates a focused service for retrieving administrative address hierarchies (Provinces, Districts, Wards).
func NewLocationService(
	shippingFactory *shipping.ShippingFactory,
	redisCache cache.RedisCacheService,
) LocationService {
	return &locationService{
		shippingFactory: shippingFactory,
		cache:           redisCache,
	}
}

func (s *locationService) getGHNAdapter() (*shipping.GHNAdapter, error) {
	if s.shippingFactory == nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "Shipping provider factory not configured")
	}

	provider, err := s.shippingFactory.GetProvider("ghn")
	if err != nil {
		return nil, utils.NewError(utils.ErrCodeInternal, "GHN provider not configured")
	}

	ghnAdapter, ok := provider.(*shipping.GHNAdapter)
	if !ok {
		return nil, utils.NewError(utils.ErrCodeInternal, "Invalid GHN provider adapter")
	}

	return ghnAdapter, nil
}

func (s *locationService) GetProvinces(ctx context.Context) ([]v1dto.ProvinceDTO, error) {
	cacheKey := "shipping:ghn:provinces"
	var cached []v1dto.ProvinceDTO
	if s.cache != nil {
		if err := s.cache.Get(cacheKey, &cached); err == nil && len(cached) > 0 {
			return cached, nil
		}
	}

	res, err, _ := s.sf.Do(cacheKey, func() (any, error) {
		// Re-check cache inside singleflight
		if s.cache != nil {
			var c []v1dto.ProvinceDTO
			if err := s.cache.Get(cacheKey, &c); err == nil && len(c) > 0 {
				return c, nil
			}
		}

		ghnAdapter, err := s.getGHNAdapter()
		if err != nil {
			return nil, err
		}

		rawProvinces, err := ghnAdapter.GetProvinces(ctx)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to get provinces from GHN", err)
		}

		provinces := make([]v1dto.ProvinceDTO, len(rawProvinces))
		for i, p := range rawProvinces {
			provinces[i] = v1dto.ProvinceDTO{
				ProvinceID:   p.ProvinceID,
				ProvinceName: p.ProvinceName,
				Code:         p.Code,
			}
		}

		if s.cache != nil {
			_ = s.cache.Set(cacheKey, provinces, 24*time.Hour)
		}

		return provinces, nil
	})

	if err != nil {
		return nil, err
	}

	return res.([]v1dto.ProvinceDTO), nil
}

func (s *locationService) GetDistricts(ctx context.Context, provinceID int) ([]v1dto.DistrictDTO, error) {
	cacheKey := fmt.Sprintf("shipping:ghn:districts:%d", provinceID)
	var cached []v1dto.DistrictDTO
	if s.cache != nil {
		if err := s.cache.Get(cacheKey, &cached); err == nil && len(cached) > 0 {
			return cached, nil
		}
	}

	res, err, _ := s.sf.Do(cacheKey, func() (any, error) {
		// Re-check cache inside singleflight
		if s.cache != nil {
			var c []v1dto.DistrictDTO
			if err := s.cache.Get(cacheKey, &c); err == nil && len(c) > 0 {
				return c, nil
			}
		}

		ghnAdapter, err := s.getGHNAdapter()
		if err != nil {
			return nil, err
		}

		rawDistricts, err := ghnAdapter.GetDistricts(ctx, provinceID)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch districts from GHN", err)
		}

		districts := make([]v1dto.DistrictDTO, len(rawDistricts))
		for i, d := range rawDistricts {
			districts[i] = v1dto.DistrictDTO{
				DistrictID:   d.DistrictID,
				ProvinceID:   d.ProvinceID,
				DistrictName: d.DistrictName,
				Code:         d.Code,
			}
		}

		if s.cache != nil {
			_ = s.cache.Set(cacheKey, districts, locationCacheTTL)
		}

		return districts, nil
	})

	if err != nil {
		return nil, err
	}

	return res.([]v1dto.DistrictDTO), nil
}

func (s *locationService) GetWards(ctx context.Context, districtID int) ([]v1dto.WardDTO, error) {
	cacheKey := fmt.Sprintf("shipping:ghn:wards:%d", districtID)
	var cached []v1dto.WardDTO
	if s.cache != nil {
		if err := s.cache.Get(cacheKey, &cached); err == nil && len(cached) > 0 {
			return cached, nil
		}
	}

	res, err, _ := s.sf.Do(cacheKey, func() (any, error) {
		// Re-check cache inside singleflight
		if s.cache != nil {
			var c []v1dto.WardDTO
			if err := s.cache.Get(cacheKey, &c); err == nil && len(c) > 0 {
				return c, nil
			}
		}

		ghnAdapter, err := s.getGHNAdapter()
		if err != nil {
			return nil, err
		}

		rawWards, err := ghnAdapter.GetWards(ctx, districtID)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to fetch wards from GHN", err)
		}

		wards := make([]v1dto.WardDTO, len(rawWards))
		for i, w := range rawWards {
			wards[i] = v1dto.WardDTO{
				WardCode:   w.WardCode,
				DistrictID: w.DistrictID,
				WardName:   w.WardName,
			}
		}

		if s.cache != nil {
			_ = s.cache.Set(cacheKey, wards, locationCacheTTL)
		}

		return wards, nil
	})

	if err != nil {
		return nil, err
	}

	return res.([]v1dto.WardDTO), nil
}

