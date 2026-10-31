package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"goshop/pkg/cache"
	"goshop/pkg/logger"
	"goshop/pkg/utils"
	bannersqlc "goshop/services/banner/internal/db/sqlc"
	bannerdto "goshop/services/banner/internal/dto"
	bannerrepo "goshop/services/banner/internal/repository"

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

type BannerService interface {
	GetActiveBanners(ctx context.Context, position string) ([]bannerdto.BannerDTO, error)
	AdminGetBanners(ctx context.Context, query bannerdto.AdminGetBannersQuery) ([]bannerdto.AdminBannerDetailDTO, int64, error)
	AdminGetBannerByUUID(ctx context.Context, bannerUUID uuid.UUID) (bannerdto.AdminBannerDetailDTO, error)
	AdminCreateBanner(ctx context.Context, input bannerdto.CreateBannerRequest) (bannerdto.AdminBannerDetailDTO, error)
	AdminUpdateBanner(ctx context.Context, bannerUUID uuid.UUID, input bannerdto.UpdateBannerRequest) (bannerdto.AdminBannerDetailDTO, error)
	AdminUpdateBannerStatus(ctx context.Context, bannerUUID uuid.UUID, input bannerdto.UpdateBannerStatusRequest) (bannerdto.AdminBannerDetailDTO, error)
	AdminSoftDeleteBanner(ctx context.Context, bannerUUID uuid.UUID) error
	AdminRestoreBanner(ctx context.Context, bannerUUID uuid.UUID) (bannerdto.AdminBannerDetailDTO, error)
	AdminTrashBanner(ctx context.Context, bannerUUID uuid.UUID) error
}

type bannerService struct {
	bannerRepo bannerrepo.BannerRepository
	cache      cache.RedisCacheService
	sf         singleflight.Group
}

// NewBannerService creates a new instance of BannerService.
func NewBannerService(bannerRepo bannerrepo.BannerRepository, cache cache.RedisCacheService) BannerService {
	return &bannerService{
		bannerRepo: bannerRepo,
		cache:      cache,
	}
}

// ─────────────────────────── Public Endpoints ───────────────────────────────

func (s *bannerService) GetActiveBanners(ctx context.Context, position string) ([]bannerdto.BannerDTO, error) {
	normPos := s.normalizePosition(position)
	cacheKey := fmt.Sprintf("%s%s", bannerCacheKeyPrefix, normPos)

	if s.cache != nil {
		var cachedBanners []bannerdto.BannerDTO
		if err := s.cache.Get(cacheKey, &cachedBanners); err == nil && len(cachedBanners) > 0 {
			return cachedBanners, nil
		}
	}

	res, err, _ := s.sf.Do(cacheKey, func() (any, error) {
		// Re-check cache inside singleflight
		if s.cache != nil {
			var cb []bannerdto.BannerDTO
			if err := s.cache.Get(cacheKey, &cb); err == nil && len(cb) > 0 {
				return cb, nil
			}
		}

		rows, err := s.bannerRepo.GetActiveByPosition(ctx, normPos)
		if err != nil {
			return nil, utils.WrapError(utils.ErrCodeInternal, "failed to query active banners", err)
		}

		banners := make([]bannerdto.BannerDTO, len(rows))
		for i, r := range rows {
			banners[i] = bannerdto.MapBannerToDTO(r)
		}

		if s.cache != nil {
			_ = s.cache.Set(cacheKey, banners, bannerCacheTTL)
		}

		return banners, nil
	})

	if err != nil {
		return nil, err
	}

	return res.([]bannerdto.BannerDTO), nil
}

// ─────────────────────────── Admin Endpoints ────────────────────────────────

func (s *bannerService) AdminGetBanners(ctx context.Context, query bannerdto.AdminGetBannersQuery) ([]bannerdto.AdminBannerDetailDTO, int64, error) {
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

	params := bannersqlc.AdminListBannersParams{
		Limit:    limit,
		Offset:   offset,
		Position: position,
		IsActive: query.IsActive,
		Deleted:  deleted,
	}

	countParams := bannersqlc.AdminCountBannersParams{
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

	res := make([]bannerdto.AdminBannerDetailDTO, len(rows))
	for i, r := range rows {
		res[i] = bannerdto.MapAdminBannerDetailDTO(r)
	}

	return res, total, nil
}

func (s *bannerService) AdminGetBannerByUUID(ctx context.Context, bannerUUID uuid.UUID) (bannerdto.AdminBannerDetailDTO, error) {
	banner, err := s.getBanner(ctx, bannerUUID)
	if err != nil {
		return bannerdto.AdminBannerDetailDTO{}, err
	}

	return bannerdto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminCreateBanner(ctx context.Context, input bannerdto.CreateBannerRequest) (bannerdto.AdminBannerDetailDTO, error) {
	if err := s.validateDateRange(input.StartAt, input.EndAt); err != nil {
		return bannerdto.AdminBannerDetailDTO{}, err
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	params := bannersqlc.CreateBannerParams{
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
		return bannerdto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to create banner", err)
	}

	s.invalidateCache()

	return bannerdto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminUpdateBanner(ctx context.Context, bannerUUID uuid.UUID, input bannerdto.UpdateBannerRequest) (bannerdto.AdminBannerDetailDTO, error) {
	if err := s.validateDateRange(input.StartAt, input.EndAt); err != nil {
		return bannerdto.AdminBannerDetailDTO{}, err
	}

	var pos *string
	if input.Position != nil {
		p := strings.ToLower(strings.TrimSpace(*input.Position))
		pos = &p
	}

	params := bannersqlc.UpdateBannerParams{
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
			return bannerdto.AdminBannerDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "Banner not found or has been deleted")
		}
		return bannerdto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to update banner", err)
	}

	s.invalidateCache()

	return bannerdto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminUpdateBannerStatus(ctx context.Context, bannerUUID uuid.UUID, input bannerdto.UpdateBannerStatusRequest) (bannerdto.AdminBannerDetailDTO, error) {
	banner, err := s.bannerRepo.UpdateStatus(ctx, bannerUUID, *input.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return bannerdto.AdminBannerDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "Banner not found or has been deleted")
		}
		return bannerdto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to update banner status", err)
	}

	s.invalidateCache()

	return bannerdto.MapAdminBannerDetailDTO(banner), nil
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

func (s *bannerService) AdminRestoreBanner(ctx context.Context, bannerUUID uuid.UUID) (bannerdto.AdminBannerDetailDTO, error) {
	banner, err := s.bannerRepo.Restore(ctx, bannerUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return bannerdto.AdminBannerDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "Banner not found or not in trash")
		}
		return bannerdto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to restore banner", err)
	}

	s.invalidateCache()

	return bannerdto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminTrashBanner(ctx context.Context, bannerUUID uuid.UUID) error {
	err := s.bannerRepo.Trash(ctx, bannerUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Banner not found or not in trash")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to permanently delete banner", err)
	}

	s.invalidateCache()

	return nil
}

// ─────────────────────── Private SRP Helper Methods ─────────────────────────

func (s *bannerService) getBanner(ctx context.Context, bannerUUID uuid.UUID) (bannersqlc.Banner, error) {
	banner, err := s.bannerRepo.GetByUUID(ctx, bannerUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return bannersqlc.Banner{}, utils.NewError(utils.ErrCodeNotFound, "Banner not found")
		}
		return bannersqlc.Banner{}, utils.WrapError(utils.ErrCodeInternal, "failed to get banner", err)
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
