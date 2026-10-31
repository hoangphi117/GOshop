package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"goshop/pkg/utils"
	bannersqlc "goshop/services/banner/internal/db/sqlc"
	bannerdto "goshop/services/banner/internal/dto"
	bannersvc "goshop/services/banner/internal/service"
	"goshop/services/banner/internal/service/servicetest"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBannerService_GetActiveBanners(t *testing.T) {
	ctx := context.Background()
	bannerUUID := uuid.New()

	t.Run("Case 1: Cache Hit -> Returns Cached Banners Without DB Call", func(t *testing.T) {
		mockCache := &servicetest.MockRedisCacheService{
			GetFunc: func(key string, target interface{}) error {
				ptr, ok := target.(*[]bannerdto.BannerDTO)
				if ok {
					*ptr = []bannerdto.BannerDTO{
						{
							UUID:     bannerUUID.String(),
							Title:    "Cached Promo",
							ImageURL: "https://example.com/cached.jpg",
							Position: "home_main",
							Order:    1,
						},
					}
					return nil
				}
				return errors.New("type mismatch")
			},
		}

		dbCalled := false
		mockRepo := &servicetest.MockBannerRepository{
			GetActiveByPositionFunc: func(ctx context.Context, position string) ([]bannersqlc.GetActiveBannersByPositionRow, error) {
				dbCalled = true
				return nil, nil
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, mockCache)
		banners, err := svc.GetActiveBanners(ctx, "")

		require.NoError(t, err)
		assert.False(t, dbCalled)
		assert.Len(t, banners, 1)
		assert.Equal(t, "Cached Promo", banners[0].Title)
	})

	t.Run("Case 2: Cache Miss, Default Position -> Queries home_main and Caches Result", func(t *testing.T) {
		mockCache := &servicetest.MockRedisCacheService{
			GetFunc: func(key string, target interface{}) error {
				return errors.New("cache miss")
			},
			SetFunc: func(key string, value interface{}, expiration time.Duration) error {
				return nil
			},
		}

		queriedPos := ""
		mockRepo := &servicetest.MockBannerRepository{
			GetActiveByPositionFunc: func(ctx context.Context, position string) ([]bannersqlc.GetActiveBannersByPositionRow, error) {
				queriedPos = position
				return []bannersqlc.GetActiveBannersByPositionRow{
					{
						BannerID:       1,
						BannerUuid:     bannerUUID,
						BannerTitle:    "Spring Sale",
						BannerImageUrl: "https://example.com/banner.jpg",
						BannerPosition: "home_main",
						BannerOrder:    1,
						BannerIsActive: true,
					},
				}, nil
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, mockCache)
		banners, err := svc.GetActiveBanners(ctx, "")

		require.NoError(t, err)
		assert.Equal(t, "home_main", queriedPos)
		assert.Len(t, banners, 1)
		assert.Equal(t, bannerUUID.String(), banners[0].UUID)
		assert.Equal(t, "Spring Sale", banners[0].Title)
	})

	t.Run("Case 3: Custom Position -> Normalized and Queried", func(t *testing.T) {
		queriedPos := ""
		mockRepo := &servicetest.MockBannerRepository{
			GetActiveByPositionFunc: func(ctx context.Context, position string) ([]bannersqlc.GetActiveBannersByPositionRow, error) {
				queriedPos = position
				return []bannersqlc.GetActiveBannersByPositionRow{}, nil
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		banners, err := svc.GetActiveBanners(ctx, "  SIDEBAR  ")

		require.NoError(t, err)
		assert.Equal(t, "sidebar", queriedPos)
		assert.Empty(t, banners)
	})

	t.Run("Case 4: Repository Error -> Returns Internal Server Error", func(t *testing.T) {
		mockRepo := &servicetest.MockBannerRepository{
			GetActiveByPositionFunc: func(ctx context.Context, position string) ([]bannersqlc.GetActiveBannersByPositionRow, error) {
				return nil, errors.New("db connection failure")
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		_, err := svc.GetActiveBanners(ctx, "home_main")

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})
}

func TestBannerService_AdminGetBanners(t *testing.T) {
	ctx := context.Background()
	bannerUUID := uuid.New()

	t.Run("Case 1: Default Pagination and Filter Values", func(t *testing.T) {
		var capturedParams bannersqlc.AdminListBannersParams
		var capturedCountParams bannersqlc.AdminCountBannersParams

		mockRepo := &servicetest.MockBannerRepository{
			AdminListFunc: func(ctx context.Context, params bannersqlc.AdminListBannersParams) ([]bannersqlc.Banner, error) {
				capturedParams = params
				return []bannersqlc.Banner{
					{
						BannerID:       1,
						BannerUuid:     bannerUUID,
						BannerTitle:    "Admin Banner",
						BannerImageUrl: "https://example.com/banner.jpg",
						BannerPosition: "home_main",
						BannerOrder:    1,
						BannerIsActive: true,
					},
				}, nil
			},
			AdminCountFunc: func(ctx context.Context, params bannersqlc.AdminCountBannersParams) (int64, error) {
				capturedCountParams = params
				return 1, nil
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		banners, total, err := svc.AdminGetBanners(ctx, bannerdto.AdminGetBannersQuery{})

		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, banners, 1)
		assert.Equal(t, int32(10), capturedParams.Limit)
		assert.Equal(t, int32(0), capturedParams.Offset)
		assert.Nil(t, capturedParams.Position)
		assert.False(t, *capturedParams.Deleted)
		assert.False(t, *capturedCountParams.Deleted)
	})

	t.Run("Case 2: Custom Position and Deleted Filters", func(t *testing.T) {
		pos := "HOME_SUB"
		isDeleted := true
		isActive := true

		var capturedParams bannersqlc.AdminListBannersParams
		mockRepo := &servicetest.MockBannerRepository{
			AdminListFunc: func(ctx context.Context, params bannersqlc.AdminListBannersParams) ([]bannersqlc.Banner, error) {
				capturedParams = params
				return []bannersqlc.Banner{}, nil
			},
			AdminCountFunc: func(ctx context.Context, params bannersqlc.AdminCountBannersParams) (int64, error) {
				return 0, nil
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		_, _, err := svc.AdminGetBanners(ctx, bannerdto.AdminGetBannersQuery{
			Page:     2,
			Limit:    5,
			Position: &pos,
			IsActive: &isActive,
			Deleted:  &isDeleted,
		})

		require.NoError(t, err)
		assert.Equal(t, int32(5), capturedParams.Limit)
		assert.Equal(t, int32(5), capturedParams.Offset)
		require.NotNil(t, capturedParams.Position)
		assert.Equal(t, "home_sub", *capturedParams.Position)
		assert.True(t, *capturedParams.Deleted)
	})

	t.Run("Case 3: AdminList Error -> Returns 500", func(t *testing.T) {
		mockRepo := &servicetest.MockBannerRepository{
			AdminListFunc: func(ctx context.Context, params bannersqlc.AdminListBannersParams) ([]bannersqlc.Banner, error) {
				return nil, errors.New("list query failed")
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		_, _, err := svc.AdminGetBanners(ctx, bannerdto.AdminGetBannersQuery{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 4: AdminCount Error -> Returns 500", func(t *testing.T) {
		mockRepo := &servicetest.MockBannerRepository{
			AdminListFunc: func(ctx context.Context, params bannersqlc.AdminListBannersParams) ([]bannersqlc.Banner, error) {
				return []bannersqlc.Banner{}, nil
			},
			AdminCountFunc: func(ctx context.Context, params bannersqlc.AdminCountBannersParams) (int64, error) {
				return 0, errors.New("count query failed")
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		_, _, err := svc.AdminGetBanners(ctx, bannerdto.AdminGetBannersQuery{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})
}

func TestBannerService_AdminGetBannerByUUID(t *testing.T) {
	ctx := context.Background()
	bannerUUID := uuid.New()

	t.Run("Case 1: Banner Not Found -> Returns 404", func(t *testing.T) {
		mockRepo := &servicetest.MockBannerRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (bannersqlc.Banner, error) {
				return bannersqlc.Banner{}, pgx.ErrNoRows
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		_, err := svc.AdminGetBannerByUUID(ctx, bannerUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Returns Banner Detail", func(t *testing.T) {
		mockRepo := &servicetest.MockBannerRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (bannersqlc.Banner, error) {
				return bannersqlc.Banner{
					BannerID:        10,
					BannerUuid:      bannerUUID,
					BannerTitle:     "Black Friday Promo",
					BannerImageUrl:  "https://example.com/bf.jpg",
					BannerPosition:  "home_main",
					BannerOrder:     1,
					BannerIsActive:  true,
					BannerCreatedAt: time.Now(),
					BannerUpdatedAt: time.Now(),
				}, nil
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		detail, err := svc.AdminGetBannerByUUID(ctx, bannerUUID)

		require.NoError(t, err)
		assert.Equal(t, bannerUUID.String(), detail.UUID)
		assert.Equal(t, "Black Friday Promo", detail.Title)
	})
}

func TestBannerService_AdminCreateBanner(t *testing.T) {
	ctx := context.Background()
	bannerUUID := uuid.New()
	now := time.Now()
	startAt := now.Add(24 * time.Hour)
	endAt := now.Add(1 * time.Hour) // end is before start!

	t.Run("Case 1: Invalid Date Range -> Returns 400 Bad Request", func(t *testing.T) {
		svc := bannersvc.NewBannerService(nil, nil)
		_, err := svc.AdminCreateBanner(ctx, bannerdto.CreateBannerRequest{
			Title:    "Invalid Dates",
			ImageURL: "https://example.com/img.jpg",
			Position: "home_main",
			StartAt:  &startAt,
			EndAt:    &endAt,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "End date must be after start date")
	})

	t.Run("Case 2: Happy Path -> Creates Banner and Clears Cache", func(t *testing.T) {
		validStart := now.Add(-1 * time.Hour)
		validEnd := now.Add(48 * time.Hour)
		cacheCleared := false

		mockCache := &servicetest.MockRedisCacheService{
			ClearFunc: func(pattern string) error {
				cacheCleared = true
				return nil
			},
		}

		var capturedParams bannersqlc.CreateBannerParams
		mockRepo := &servicetest.MockBannerRepository{
			CreateFunc: func(ctx context.Context, params bannersqlc.CreateBannerParams) (bannersqlc.Banner, error) {
				capturedParams = params
				return bannersqlc.Banner{
					BannerID:        1,
					BannerUuid:      bannerUUID,
					BannerTitle:     params.BannerTitle,
					BannerImageUrl:  params.BannerImageUrl,
					BannerPosition:  params.BannerPosition,
					BannerOrder:     params.BannerOrder,
					BannerIsActive:  params.BannerIsActive,
					BannerStartAt:   params.BannerStartAt,
					BannerEndAt:     params.BannerEndAt,
					BannerCreatedAt: time.Now(),
					BannerUpdatedAt: time.Now(),
				}, nil
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, mockCache)
		detail, err := svc.AdminCreateBanner(ctx, bannerdto.CreateBannerRequest{
			Title:    "Summer Festival",
			ImageURL: "https://example.com/summer.jpg",
			Position: "  HOME_MAIN  ",
			Order:    2,
			StartAt:  &validStart,
			EndAt:    &validEnd,
		})

		require.NoError(t, err)
		assert.True(t, cacheCleared)
		assert.Equal(t, bannerUUID.String(), detail.UUID)
		assert.Equal(t, "Summer Festival", detail.Title)
		assert.Equal(t, "home_main", capturedParams.BannerPosition)
		assert.True(t, capturedParams.BannerIsActive) // Default active is true
	})

	t.Run("Case 3: Repository Error -> Returns 500", func(t *testing.T) {
		mockRepo := &servicetest.MockBannerRepository{
			CreateFunc: func(ctx context.Context, params bannersqlc.CreateBannerParams) (bannersqlc.Banner, error) {
				return bannersqlc.Banner{}, errors.New("insert failed")
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		_, err := svc.AdminCreateBanner(ctx, bannerdto.CreateBannerRequest{
			Title:    "Failed Banner",
			ImageURL: "https://example.com/failed.jpg",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})
}

func TestBannerService_AdminUpdateBanner(t *testing.T) {
	ctx := context.Background()
	bannerUUID := uuid.New()
	now := time.Now()
	startAt := now.Add(24 * time.Hour)
	endAt := now.Add(1 * time.Hour) // end is before start

	t.Run("Case 1: Invalid Date Range -> Returns 400 Bad Request", func(t *testing.T) {
		svc := bannersvc.NewBannerService(nil, nil)
		_, err := svc.AdminUpdateBanner(ctx, bannerUUID, bannerdto.UpdateBannerRequest{
			StartAt: &startAt,
			EndAt:   &endAt,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("Case 2: Banner Not Found -> Returns 404", func(t *testing.T) {
		mockRepo := &servicetest.MockBannerRepository{
			UpdateFunc: func(ctx context.Context, params bannersqlc.UpdateBannerParams) (bannersqlc.Banner, error) {
				return bannersqlc.Banner{}, sql.ErrNoRows
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		_, err := svc.AdminUpdateBanner(ctx, bannerUUID, bannerdto.UpdateBannerRequest{
			Title: utils.StringToPointer("Updated Title"),
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Happy Path -> Updates Banner and Clears Cache", func(t *testing.T) {
		cacheCleared := false
		mockCache := &servicetest.MockRedisCacheService{
			ClearFunc: func(pattern string) error {
				cacheCleared = true
				return nil
			},
		}

		newTitle := "Updated Title"
		newPos := "HOME_SUB"
		mockRepo := &servicetest.MockBannerRepository{
			UpdateFunc: func(ctx context.Context, params bannersqlc.UpdateBannerParams) (bannersqlc.Banner, error) {
				return bannersqlc.Banner{
					BannerID:        1,
					BannerUuid:      bannerUUID,
					BannerTitle:     *params.BannerTitle,
					BannerPosition:  *params.BannerPosition,
					BannerCreatedAt: time.Now(),
					BannerUpdatedAt: time.Now(),
				}, nil
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, mockCache)
		detail, err := svc.AdminUpdateBanner(ctx, bannerUUID, bannerdto.UpdateBannerRequest{
			Title:    &newTitle,
			Position: &newPos,
		})

		require.NoError(t, err)
		assert.True(t, cacheCleared)
		assert.Equal(t, "Updated Title", detail.Title)
	})
}

func TestBannerService_AdminUpdateBannerStatus(t *testing.T) {
	ctx := context.Background()
	bannerUUID := uuid.New()
	isActive := false

	t.Run("Case 1: Banner Not Found -> Returns 404", func(t *testing.T) {
		mockRepo := &servicetest.MockBannerRepository{
			UpdateStatusFunc: func(ctx context.Context, u uuid.UUID, active bool) (bannersqlc.Banner, error) {
				return bannersqlc.Banner{}, pgx.ErrNoRows
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		_, err := svc.AdminUpdateBannerStatus(ctx, bannerUUID, bannerdto.UpdateBannerStatusRequest{
			IsActive: &isActive,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Deactivates Banner and Clears Cache", func(t *testing.T) {
		cacheCleared := false
		mockCache := &servicetest.MockRedisCacheService{
			ClearFunc: func(pattern string) error {
				cacheCleared = true
				return nil
			},
		}

		mockRepo := &servicetest.MockBannerRepository{
			UpdateStatusFunc: func(ctx context.Context, u uuid.UUID, active bool) (bannersqlc.Banner, error) {
				return bannersqlc.Banner{
					BannerID:        1,
					BannerUuid:      bannerUUID,
					BannerTitle:     "Deactivated Banner",
					BannerIsActive:  active,
					BannerCreatedAt: time.Now(),
					BannerUpdatedAt: time.Now(),
				}, nil
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, mockCache)
		detail, err := svc.AdminUpdateBannerStatus(ctx, bannerUUID, bannerdto.UpdateBannerStatusRequest{
			IsActive: &isActive,
		})

		require.NoError(t, err)
		assert.True(t, cacheCleared)
		assert.False(t, detail.IsActive)
	})
}

func TestBannerService_AdminSoftDeleteBanner(t *testing.T) {
	ctx := context.Background()
	bannerUUID := uuid.New()

	t.Run("Case 1: Banner Not Found -> Returns 404", func(t *testing.T) {
		mockRepo := &servicetest.MockBannerRepository{
			SoftDeleteFunc: func(ctx context.Context, u uuid.UUID) (bannersqlc.Banner, error) {
				return bannersqlc.Banner{}, pgx.ErrNoRows
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		err := svc.AdminSoftDeleteBanner(ctx, bannerUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Soft Deletes and Clears Cache", func(t *testing.T) {
		cacheCleared := false
		mockCache := &servicetest.MockRedisCacheService{
			ClearFunc: func(pattern string) error {
				cacheCleared = true
				return nil
			},
		}

		mockRepo := &servicetest.MockBannerRepository{
			SoftDeleteFunc: func(ctx context.Context, u uuid.UUID) (bannersqlc.Banner, error) {
				return bannersqlc.Banner{
					BannerID:   1,
					BannerUuid: bannerUUID,
					BannerDeletedAt: pgtype.Timestamptz{
						Time:  time.Now(),
						Valid: true,
					},
				}, nil
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, mockCache)
		err := svc.AdminSoftDeleteBanner(ctx, bannerUUID)

		require.NoError(t, err)
		assert.True(t, cacheCleared)
	})
}

func TestBannerService_AdminRestoreBanner(t *testing.T) {
	ctx := context.Background()
	bannerUUID := uuid.New()

	t.Run("Case 1: Banner Not Found -> Returns 404", func(t *testing.T) {
		mockRepo := &servicetest.MockBannerRepository{
			RestoreFunc: func(ctx context.Context, u uuid.UUID) (bannersqlc.Banner, error) {
				return bannersqlc.Banner{}, pgx.ErrNoRows
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		_, err := svc.AdminRestoreBanner(ctx, bannerUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Restores Banner and Clears Cache", func(t *testing.T) {
		cacheCleared := false
		mockCache := &servicetest.MockRedisCacheService{
			ClearFunc: func(pattern string) error {
				cacheCleared = true
				return nil
			},
		}

		mockRepo := &servicetest.MockBannerRepository{
			RestoreFunc: func(ctx context.Context, u uuid.UUID) (bannersqlc.Banner, error) {
				return bannersqlc.Banner{
					BannerID:        1,
					BannerUuid:      bannerUUID,
					BannerTitle:     "Restored Banner",
					BannerIsActive:  true,
					BannerCreatedAt: time.Now(),
					BannerUpdatedAt: time.Now(),
				}, nil
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, mockCache)
		detail, err := svc.AdminRestoreBanner(ctx, bannerUUID)

		require.NoError(t, err)
		assert.True(t, cacheCleared)
		assert.Equal(t, bannerUUID.String(), detail.UUID)
	})
}

func TestBannerService_AdminTrashBanner(t *testing.T) {
	ctx := context.Background()
	bannerUUID := uuid.New()

	t.Run("Case 0: Banner Not In Trash -> Returns 404", func(t *testing.T) {
		mockRepo := &servicetest.MockBannerRepository{
			TrashFunc: func(ctx context.Context, u uuid.UUID) error {
				return pgx.ErrNoRows
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		err := svc.AdminTrashBanner(ctx, bannerUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 1: Repository Error -> Returns 500", func(t *testing.T) {
		mockRepo := &servicetest.MockBannerRepository{
			TrashFunc: func(ctx context.Context, u uuid.UUID) error {
				return errors.New("foreign key constraint error")
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, nil)
		err := svc.AdminTrashBanner(ctx, bannerUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 2: Happy Path -> Permanently Deletes Banner and Clears Cache", func(t *testing.T) {
		cacheCleared := false
		mockCache := &servicetest.MockRedisCacheService{
			ClearFunc: func(pattern string) error {
				cacheCleared = true
				return nil
			},
		}

		mockRepo := &servicetest.MockBannerRepository{
			TrashFunc: func(ctx context.Context, u uuid.UUID) error {
				return nil
			},
		}

		svc := bannersvc.NewBannerService(mockRepo, mockCache)
		err := svc.AdminTrashBanner(ctx, bannerUUID)

		require.NoError(t, err)
		assert.True(t, cacheCleared)
	})
}
