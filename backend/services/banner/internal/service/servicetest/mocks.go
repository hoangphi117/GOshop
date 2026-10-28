// Package servicetest holds test doubles for the banner service layer.
// ponytail: local copies of the monolith's mocks (internal/testutils) so the
// service does not depend on monolith-internal packages in tests; once more
// services need them, move to a shared pkg/testutils.
package servicetest

import (
	"context"
	"time"

	bannersqlc "goshop/services/banner/internal/db/sqlc"

	"github.com/google/uuid"
)

type MockBannerRepository struct {
	GetActiveByPositionFunc func(ctx context.Context, position string) ([]bannersqlc.GetActiveBannersByPositionRow, error)
	CreateFunc              func(ctx context.Context, params bannersqlc.CreateBannerParams) (bannersqlc.Banner, error)
	GetByUUIDFunc           func(ctx context.Context, bannerUUID uuid.UUID) (bannersqlc.Banner, error)
	AdminListFunc           func(ctx context.Context, params bannersqlc.AdminListBannersParams) ([]bannersqlc.Banner, error)
	AdminCountFunc          func(ctx context.Context, params bannersqlc.AdminCountBannersParams) (int64, error)
	UpdateFunc              func(ctx context.Context, params bannersqlc.UpdateBannerParams) (bannersqlc.Banner, error)
	UpdateStatusFunc        func(ctx context.Context, bannerUUID uuid.UUID, isActive bool) (bannersqlc.Banner, error)
	SoftDeleteFunc          func(ctx context.Context, bannerUUID uuid.UUID) (bannersqlc.Banner, error)
	RestoreFunc             func(ctx context.Context, bannerUUID uuid.UUID) (bannersqlc.Banner, error)
	TrashFunc               func(ctx context.Context, bannerUUID uuid.UUID) error
}

func (m *MockBannerRepository) GetActiveByPosition(ctx context.Context, position string) ([]bannersqlc.GetActiveBannersByPositionRow, error) {
	if m.GetActiveByPositionFunc != nil {
		return m.GetActiveByPositionFunc(ctx, position)
	}
	return nil, nil
}

func (m *MockBannerRepository) Create(ctx context.Context, params bannersqlc.CreateBannerParams) (bannersqlc.Banner, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, params)
	}
	return bannersqlc.Banner{}, nil
}

func (m *MockBannerRepository) GetByUUID(ctx context.Context, bannerUUID uuid.UUID) (bannersqlc.Banner, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, bannerUUID)
	}
	return bannersqlc.Banner{}, nil
}

func (m *MockBannerRepository) AdminList(ctx context.Context, params bannersqlc.AdminListBannersParams) ([]bannersqlc.Banner, error) {
	if m.AdminListFunc != nil {
		return m.AdminListFunc(ctx, params)
	}
	return nil, nil
}

func (m *MockBannerRepository) AdminCount(ctx context.Context, params bannersqlc.AdminCountBannersParams) (int64, error) {
	if m.AdminCountFunc != nil {
		return m.AdminCountFunc(ctx, params)
	}
	return 0, nil
}

func (m *MockBannerRepository) Update(ctx context.Context, params bannersqlc.UpdateBannerParams) (bannersqlc.Banner, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, params)
	}
	return bannersqlc.Banner{}, nil
}

func (m *MockBannerRepository) UpdateStatus(ctx context.Context, bannerUUID uuid.UUID, isActive bool) (bannersqlc.Banner, error) {
	if m.UpdateStatusFunc != nil {
		return m.UpdateStatusFunc(ctx, bannerUUID, isActive)
	}
	return bannersqlc.Banner{}, nil
}

func (m *MockBannerRepository) SoftDelete(ctx context.Context, bannerUUID uuid.UUID) (bannersqlc.Banner, error) {
	if m.SoftDeleteFunc != nil {
		return m.SoftDeleteFunc(ctx, bannerUUID)
	}
	return bannersqlc.Banner{}, nil
}

func (m *MockBannerRepository) Restore(ctx context.Context, bannerUUID uuid.UUID) (bannersqlc.Banner, error) {
	if m.RestoreFunc != nil {
		return m.RestoreFunc(ctx, bannerUUID)
	}
	return bannersqlc.Banner{}, nil
}

func (m *MockBannerRepository) Trash(ctx context.Context, bannerUUID uuid.UUID) error {
	if m.TrashFunc != nil {
		return m.TrashFunc(ctx, bannerUUID)
	}
	return nil
}

type MockRedisCacheService struct {
	SetFunc    func(key string, value interface{}, expiration time.Duration) error
	SetNXFunc  func(key string, value interface{}, expiration time.Duration) (bool, error)
	GetFunc    func(key string, target interface{}) error
	ClearFunc  func(pattern string) error
	ExistsFunc func(key string) (bool, error)
	DelFunc    func(key string) error
	IncrFunc   func(key string) (int64, error)
}

func (m *MockRedisCacheService) Set(key string, value interface{}, expiration time.Duration) error {
	if m.SetFunc != nil {
		return m.SetFunc(key, value, expiration)
	}
	return nil
}

func (m *MockRedisCacheService) SetNX(key string, value interface{}, expiration time.Duration) (bool, error) {
	if m.SetNXFunc != nil {
		return m.SetNXFunc(key, value, expiration)
	}
	return false, nil
}

func (m *MockRedisCacheService) Get(key string, target interface{}) error {
	if m.GetFunc != nil {
		return m.GetFunc(key, target)
	}
	return nil
}

func (m *MockRedisCacheService) Clear(pattern string) error {
	if m.ClearFunc != nil {
		return m.ClearFunc(pattern)
	}
	return nil
}

func (m *MockRedisCacheService) Exists(key string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(key)
	}
	return false, nil
}

func (m *MockRedisCacheService) Del(key string) error {
	if m.DelFunc != nil {
		return m.DelFunc(key)
	}
	return nil
}

func (m *MockRedisCacheService) Incr(key string) (int64, error) {
	if m.IncrFunc != nil {
		return m.IncrFunc(key)
	}
	return 0, nil
}

func (m *MockRedisCacheService) Expire(key string, ttl time.Duration) error {
	return nil
}

func (m *MockRedisCacheService) TTL(key string) (time.Duration, error) {
	return 0, nil
}
