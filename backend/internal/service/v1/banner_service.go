package v1service

import (
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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type bannerService struct {
	bannerRepo repository.BannerRepository
	cache      cache.RedisCacheService
}

func NewBannerService(bannerRepo repository.BannerRepository, cache cache.RedisCacheService) BannerService {
	return &bannerService{
		bannerRepo: bannerRepo,
		cache:      cache,
	}
}

func (s *bannerService) GetActiveBanners(ctx *gin.Context, position string) ([]v1dto.BannerDTO, error) {
	reqCtx := ctx.Request.Context()

	position = strings.ToLower(strings.TrimSpace(position))
	if position == "" {
		position = "home_main"
	}

	cacheKey := fmt.Sprintf("banners:active:%s", position)
	var cachedBanners []v1dto.BannerDTO
	if err := s.cache.Get(cacheKey, &cachedBanners); err == nil && len(cachedBanners) > 0 {
		return cachedBanners, nil
	}

	rows, err := s.bannerRepo.GetActiveByPosition(reqCtx, position)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to query active banners", err)
	}

	banners := make([]v1dto.BannerDTO, len(rows))
	for i, r := range rows {
		banners[i] = v1dto.MapBannerToDTO(r)
	}

	_ = s.cache.Set(cacheKey, banners, 10*time.Minute)

	return banners, nil
}

func (s *bannerService) AdminGetBanners(ctx *gin.Context, query v1dto.AdminGetBannersQuery) ([]v1dto.AdminBannerDetailDTO, int64, error) {
	reqCtx := ctx.Request.Context()

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

	params := sqlc.AdminListBannersParams{
		Limit:    limit,
		Offset:   offset,
		Position: position,
		IsActive: query.IsActive,
		Deleted:  query.Deleted,
	}

	countParams := sqlc.AdminCountBannersParams{
		Position: position,
		IsActive: query.IsActive,
		Deleted:  query.Deleted,
	}

	rows, err := s.bannerRepo.AdminList(reqCtx, params)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "failed to list banners", err)
	}

	total, err := s.bannerRepo.AdminCount(reqCtx, countParams)
	if err != nil {
		return nil, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count banners", err)
	}

	res := make([]v1dto.AdminBannerDetailDTO, len(rows))
	for i, r := range rows {
		res[i] = v1dto.MapAdminBannerDetailDTO(r)
	}

	return res, total, nil
}

func (s *bannerService) AdminGetBannerByUUID(ctx *gin.Context, bannerUUID uuid.UUID) (v1dto.AdminBannerDetailDTO, error) {
	reqCtx := ctx.Request.Context()

	banner, err := s.bannerRepo.GetByUUID(reqCtx, bannerUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return v1dto.AdminBannerDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "Banner not found")
		}
		return v1dto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to get banner", err)
	}

	return v1dto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminCreateBanner(ctx *gin.Context, input v1dto.CreateBannerRequest) (v1dto.AdminBannerDetailDTO, error) {
	reqCtx := ctx.Request.Context()

	if input.StartAt != nil && input.EndAt != nil && input.EndAt.Before(*input.StartAt) {
		return v1dto.AdminBannerDetailDTO{}, utils.NewError(utils.ErrCodeBadRequest, "End date must be after start date")
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	var startAt, endAt pgtype.Timestamptz
	if input.StartAt != nil {
		startAt = pgtype.Timestamptz{Time: *input.StartAt, Valid: true}
	}
	if input.EndAt != nil {
		endAt = pgtype.Timestamptz{Time: *input.EndAt, Valid: true}
	}

	params := sqlc.CreateBannerParams{
		BannerTitle:     strings.TrimSpace(input.Title),
		BannerImageUrl:  strings.TrimSpace(input.ImageURL),
		BannerLinkUrl:   input.LinkURL,
		BannerPosition:  strings.ToLower(strings.TrimSpace(input.Position)),
		BannerOrder:     input.Order,
		BannerIsActive:  isActive,
		BannerStartAt:   startAt,
		BannerEndAt:     endAt,
	}

	banner, err := s.bannerRepo.Create(reqCtx, params)
	if err != nil {
		return v1dto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to create banner", err)
	}

	s.invalidateCache()

	return v1dto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminUpdateBanner(ctx *gin.Context, bannerUUID uuid.UUID, input v1dto.UpdateBannerRequest) (v1dto.AdminBannerDetailDTO, error) {
	reqCtx := ctx.Request.Context()

	if input.StartAt != nil && input.EndAt != nil && input.EndAt.Before(*input.StartAt) {
		return v1dto.AdminBannerDetailDTO{}, utils.NewError(utils.ErrCodeBadRequest, "End date must be after start date")
	}

	var startAt, endAt pgtype.Timestamptz
	if input.StartAt != nil {
		startAt = pgtype.Timestamptz{Time: *input.StartAt, Valid: true}
	}
	if input.EndAt != nil {
		endAt = pgtype.Timestamptz{Time: *input.EndAt, Valid: true}
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
		BannerStartAt:  startAt,
		BannerEndAt:    endAt,
	}

	banner, err := s.bannerRepo.Update(reqCtx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return v1dto.AdminBannerDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "Banner not found or has been deleted")
		}
		return v1dto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to update banner", err)
	}

	s.invalidateCache()

	return v1dto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminUpdateBannerStatus(ctx *gin.Context, bannerUUID uuid.UUID, input v1dto.UpdateBannerStatusRequest) (v1dto.AdminBannerDetailDTO, error) {
	reqCtx := ctx.Request.Context()

	banner, err := s.bannerRepo.UpdateStatus(reqCtx, bannerUUID, *input.IsActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return v1dto.AdminBannerDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "Banner not found or has been deleted")
		}
		return v1dto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to update banner status", err)
	}

	s.invalidateCache()

	return v1dto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminSoftDeleteBanner(ctx *gin.Context, bannerUUID uuid.UUID) error {
	reqCtx := ctx.Request.Context()

	_, err := s.bannerRepo.SoftDelete(reqCtx, bannerUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Banner not found or already deleted")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to delete banner", err)
	}

	s.invalidateCache()

	return nil
}

func (s *bannerService) AdminRestoreBanner(ctx *gin.Context, bannerUUID uuid.UUID) (v1dto.AdminBannerDetailDTO, error) {
	reqCtx := ctx.Request.Context()

	banner, err := s.bannerRepo.Restore(reqCtx, bannerUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return v1dto.AdminBannerDetailDTO{}, utils.NewError(utils.ErrCodeNotFound, "Banner not found or not in trash")
		}
		return v1dto.AdminBannerDetailDTO{}, utils.WrapError(utils.ErrCodeInternal, "failed to restore banner", err)
	}

	s.invalidateCache()

	return v1dto.MapAdminBannerDetailDTO(banner), nil
}

func (s *bannerService) AdminTrashBanner(ctx *gin.Context, bannerUUID uuid.UUID) error {
	reqCtx := ctx.Request.Context()

	err := s.bannerRepo.Trash(reqCtx, bannerUUID)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "failed to permanently delete banner", err)
	}

	s.invalidateCache()

	return nil
}

func (s *bannerService) invalidateCache() {
	if err := s.cache.Clear("banners:active:*"); err != nil {
		logger.Log.Warn().Err(err).Msg("Failed to clear banners cache")
	}
}
