package v1service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/cache"
	"goshop/pkg/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/singleflight"
)

const (
	defaultBannerPosition = "home_main"
	bannerCacheKeyPrefix  = "banners:active:"
	bannerCachePattern    = "banners:active:*"
	bannerCacheTTL        = 10 * time.Minute
)

type bannerService struct {
	bannerRepo repository.BannerRepository
	cache      cache.RedisCacheService
	sf         singleflight.Group
}

// NewBannerService creates a new instance of BannerService.
func NewBannerService(bannerRepo repository.BannerRepository, cache cache.RedisCacheService) BannerService {
	return &bannerService{
		bannerRepo: bannerRepo,
		cache:      cache,
	}
}

// ── Public Endpoints ──────────────────────────────────────────────

func (s *bannerService) GetActiveBanners(ctx context.Context, position string) ([]v1dto.BannerDTO, error) {
	normPos := s.normalizePosition(position)
	cacheKey := fmt.Sprintf("%s%s", bannerCacheKeyPrefix, normPos)

	if s.cache != nil {
		var cachedBanners []v1dto.BannerDTO
		if err := s.cache.Get(cacheKey, &cachedBanners); err == nil && len(cachedBanners) > 0 {
			return cachedBanners, nil
		}
	}

	res, err, _ := s.sf.Do(cacheKey, func() (any, error) {
		// Re-check cache inside singleflight
		if s.cache != nil {
			var cb []v1dto.BannerDTO
			if err := s.cache.Get(cacheKey, &cb); err == nil && len(cb) > 0 {
				return cb, nil
			}
		}

		rows, err := s.bannerRepo.GetActiveByPosition(ctx, normPos)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to query active banners", err)
		}

		banners := make([]v1dto.BannerDTO, len(rows))
		for i, r := range rows {
			banners[i] = v1dto.MapBannerToDTO(r)
		}

		if s.cache != nil {
			_ = s.cache.Set(cacheKey, banners, bannerCacheTTL)
		}

		return banners, nil
	})

	if err != nil {
		return nil, err
	}

	return res.([]v1dto.BannerDTO), nil
}

// ── Admin Endpoints ───────────────────────────────────────────────

func (s *bannerService) AdminGetBanners(ctx context.Context, query v1dto.AdminGetBannersQuery) ([]v1dto.AdminBannerDetailDTO, int64, error) {
	page := query.Page
	if page <= 0 {
		page = 1
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 10
	}
	offset := (page - 1) * limit

	var position *string
	if query.Position != nil && strings.TrimSpace(*query.Position) != "" {
		p := strings.ToLower(strings.TrimSpace(*query.Position))
		position = &p
	}

	deleted := query.Deleted
	if deleted == nil {
		f := false
		deleted = &f
	}

	params := sqlc.AdminListBannersParams{
		Limit:    limit,
		Offset:   offset,
		Position: position,
		IsActive: query.IsActive,
		Deleted:  deleted,
	}

	countParams := sqlc.AdminCountBannersParams{
		Position: position,
		IsActive: query.IsActive,
		Deleted:  deleted,
	}

	rows, err := s.bannerRepo.AdminList(ctx, params)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "failed to list banners", err)
	}

	total, err := s.bannerRepo.AdminCount(ctx, countParams)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count banners", err)
	}

	res := make([]v1dto.AdminBannerDetailDTO, len(rows))
	for i, r := range rows {
		res[i] = v1dto.MapAdminBannerDetailDTO(r)
	}

	return res, total, nil
}

func (s *bannerService) AdminGetBannerByUUID(ctx context.Context, bannerUUID uuid.UUID) (v1dto.AdminBannerDetailDTO, error) {
	banner, err := s.getBanner(ctx, bannerUUID)
	if err != nil {
		return v1dto.AdminBannerDetailDTO{}, err
	}

	return v1dto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminCreateBanner(ctx context.Context, input v1dto.CreateBannerRequest) (v1dto.AdminBannerDetailDTO, error) {
	if err := s.validateDateRange(input.StartAt, input.EndAt); err != nil {
		return v1dto.AdminBannerDetailDTO{}, err
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	params := sqlc.CreateBannerParams{
		BannerTitle:    strings.TrimSpace(input.Title),
		BannerImageUrl: strings.TrimSpace(input.ImageURL),
		BannerLinkUrl:  input.LinkURL,
		BannerPosition: s.normalizePosition(input.Position),
		BannerOrder:    input.Order,
		BannerIsActive: isActive,
		BannerStartAt:  s.toPgTimestamptz(input.StartAt),
		BannerEndAt:    s.toPgTimestamptz(input.EndAt),
	}

	banner, err := s.bannerRepo.Create(ctx, params)
	if err != nil {
		return v1dto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to create banner", err)
	}

	s.invalidateCache()

	return v1dto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminUpdateBanner(ctx context.Context, bannerUUID uuid.UUID, input v1dto.UpdateBannerRequest) (v1dto.AdminBannerDetailDTO, error) {
	if err := s.validateDateRange(input.StartAt, input.EndAt); err != nil {
		return v1dto.AdminBannerDetailDTO{}, err
	}

	var pos *string
	if input.Position != nil {
		p := strings.ToLower(strings.TrimSpace(*input.Position))
		pos = &p
	}

	params := sqlc.UpdateBannerParams{
		BannerUuid:     bannerUUID,
		BannerTitle:    input.Title,
		BannerImageUrl: input.ImageURL,
		BannerLinkUrl:  input.LinkURL,
		BannerPosition: pos,
		BannerOrder:    input.Order,
		BannerIsActive: input.IsActive,
		BannerStartAt:  s.toPgTimestamptz(input.StartAt),
		BannerEndAt:    s.toPgTimestamptz(input.EndAt),
	}

	banner, err := s.bannerRepo.Update(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return v1dto.AdminBannerDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "Banner not found or has been deleted")
		}
		return v1dto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to update banner", err)
	}

	s.invalidateCache()

	return v1dto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminUpdateBannerStatus(ctx context.Context, bannerUUID uuid.UUID, input v1dto.UpdateBannerStatusRequest) (v1dto.AdminBannerDetailDTO, error) {
	banner, err := s.bannerRepo.UpdateStatus(ctx, bannerUUID, *input.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return v1dto.AdminBannerDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "Banner not found or has been deleted")
		}
		return v1dto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to update banner status", err)
	}

	s.invalidateCache()

	return v1dto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminSoftDeleteBanner(ctx context.Context, bannerUUID uuid.UUID) error {
	_, err := s.bannerRepo.SoftDelete(ctx, bannerUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Banner not found or already deleted")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to delete banner", err)
	}

	s.invalidateCache()

	return nil
}

func (s *bannerService) AdminRestoreBanner(ctx context.Context, bannerUUID uuid.UUID) (v1dto.AdminBannerDetailDTO, error) {
	banner, err := s.bannerRepo.Restore(ctx, bannerUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return v1dto.AdminBannerDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "Banner not found or not in trash")
		}
		return v1dto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to restore banner", err)
	}

	s.invalidateCache()

	return v1dto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminTrashBanner(ctx context.Context, bannerUUID uuid.UUID) error {
	err := s.bannerRepo.Trash(ctx, bannerUUID)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to permanently delete banner", err)
	}

	s.invalidateCache()

	return nil
}

// ── Private SRP Helper Methods ────────────────────────────────────

func (s *bannerService) getBanner(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error) {
	banner, err := s.bannerRepo.GetByUUID(ctx, bannerUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return sqlc.Banner{}, utils.NewError(utils.ErrCodeNotFound, "Banner not found")
		}
		return sqlc.Banner{}, utils.WrapError(utils.ErrCodeInternal, "failed to get banner", err)
	}
	return banner, nil
}

func (s *bannerService) normalizePosition(position string) string {
	pos := strings.ToLower(strings.TrimSpace(position))
	if pos == "" {
		return defaultBannerPosition
	}
	return pos
}

func (s *bannerService) validateDateRange(startAt, endAt *time.Time) error {
	if startAt != nil && endAt != nil && endAt.Before(*startAt) {
		return utils.NewError(utils.ErrCodeBadRequest, "End date must be after start date")
	}
	return nil
}

func (s *bannerService) toPgTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func (s *bannerService) invalidateCache() {
	if s.cache == nil {
		return
	}
	if err := s.cache.Clear(bannerCachePattern); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear banners cache")
	}
}
