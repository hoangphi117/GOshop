package repository

import (
	"context"
	"goshop/internal/db/sqlc"

	"github.com/google/uuid"
)

type SqlBannerRepository struct {
	db sqlc.Querier
}

func NewSqlBannerRepository(db sqlc.Querier) BannerRepository {
	return &SqlBannerRepository{db: db}
}

func (r *SqlBannerRepository) GetActiveByPosition(ctx context.Context, position string) ([]sqlc.GetActiveBannersByPositionRow, error) {
	return r.db.GetActiveBannersByPosition(ctx, position)
}

func (r *SqlBannerRepository) Create(ctx context.Context, params sqlc.CreateBannerParams) (sqlc.Banner, error) {
	return r.db.CreateBanner(ctx, params)
}

func (r *SqlBannerRepository) GetByUUID(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error) {
	return r.db.GetBannerByUUID(ctx, bannerUUID)
}

func (r *SqlBannerRepository) AdminList(ctx context.Context, params sqlc.AdminListBannersParams) ([]sqlc.Banner, error) {
	return r.db.AdminListBanners(ctx, params)
}

func (r *SqlBannerRepository) AdminCount(ctx context.Context, params sqlc.AdminCountBannersParams) (int64, error) {
	return r.db.AdminCountBanners(ctx, params)
}

func (r *SqlBannerRepository) Update(ctx context.Context, params sqlc.UpdateBannerParams) (sqlc.Banner, error) {
	return r.db.UpdateBanner(ctx, params)
}

func (r *SqlBannerRepository) UpdateStatus(ctx context.Context, bannerUUID uuid.UUID, isActive bool) (sqlc.Banner, error) {
	return r.db.UpdateBannerStatus(ctx, sqlc.UpdateBannerStatusParams{
		BannerUuid:     bannerUUID,
		BannerIsActive: isActive,
	})
}

func (r *SqlBannerRepository) SoftDelete(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error) {
	return r.db.SoftDeleteBanner(ctx, bannerUUID)
}

func (r *SqlBannerRepository) Restore(ctx context.Context, bannerUUID uuid.UUID) (sqlc.Banner, error) {
	return r.db.RestoreBanner(ctx, bannerUUID)
}

func (r *SqlBannerRepository) Trash(ctx context.Context, bannerUUID uuid.UUID) error {
	_, err := r.db.TrashBanner(ctx, bannerUUID)
	return err
}
