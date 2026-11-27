package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/services/identity/internal/db/sqlc"
	dto "goshop/services/identity/internal/dto"
	"goshop/services/identity/internal/repository"
	"goshop/shared/pkg/utils"
)

type fakeAddrTx struct {
	pgx.Tx
	committed  bool
	rolledBack bool
	commitErr  error
}

func (f *fakeAddrTx) Commit(context.Context) error {
	f.committed = true
	return f.commitErr
}

func (f *fakeAddrTx) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}

type mockAddressRepo struct {
	repository.AddressRepository
	tx                *fakeAddrTx
	beginErr          error
	countFn           func(ctx context.Context, userID int32) (int64, error)
	clearDefaultFn    func(ctx context.Context, tx pgx.Tx, userID int32) error
	createFn          func(ctx context.Context, tx pgx.Tx, params sqlc.CreateUserAddressParams) (sqlc.UserAddress, error)
	getByUserIDFn     func(ctx context.Context, userID int32) ([]sqlc.UserAddress, error)
	getByUUIDFn       func(ctx context.Context, userID int32, addrUUID uuid.UUID) (sqlc.UserAddress, error)
	updateFn          func(ctx context.Context, params sqlc.UpdateUserAddressParams) (sqlc.UserAddress, error)
	setDefaultFn      func(ctx context.Context, tx pgx.Tx, userID int32, addrUUID uuid.UUID) (sqlc.UserAddress, error)
	deleteFn          func(ctx context.Context, tx pgx.Tx, userID int32, addrUUID uuid.UUID) (bool, error)
	setLatestDefFn    func(ctx context.Context, tx pgx.Tx, userID int32) error
	clearDefCalls     int
	setLatestDefCalls int
}

func (m *mockAddressRepo) BeginTx(context.Context) (pgx.Tx, error) {
	if m.beginErr != nil {
		return nil, m.beginErr
	}
	if m.tx == nil {
		m.tx = &fakeAddrTx{}
	}
	return m.tx, nil
}
func (m *mockAddressRepo) Count(ctx context.Context, userID int32) (int64, error) {
	if m.countFn != nil {
		return m.countFn(ctx, userID)
	}
	return 0, nil
}
func (m *mockAddressRepo) ClearDefault(ctx context.Context, tx pgx.Tx, userID int32) error {
	m.clearDefCalls++
	if m.clearDefaultFn != nil {
		return m.clearDefaultFn(ctx, tx, userID)
	}
	return nil
}
func (m *mockAddressRepo) Create(ctx context.Context, tx pgx.Tx, p sqlc.CreateUserAddressParams) (sqlc.UserAddress, error) {
	if m.createFn != nil {
		return m.createFn(ctx, tx, p)
	}
	return sqlc.UserAddress{
		UaUuid:         uuid.New(),
		UaLabel:        p.UaLabel,
		UaName:         p.UaName,
		UaPhone:        p.UaPhone,
		UaAddress:      p.UaAddress,
		UaProvinceName: p.UaProvinceName,
		UaDistrictName: p.UaDistrictName,
		UaWardName:     p.UaWardName,
		UaIsDefault:    p.UaIsDefault,
	}, nil
}
func (m *mockAddressRepo) GetByUserID(ctx context.Context, userID int32) ([]sqlc.UserAddress, error) {
	return m.getByUserIDFn(ctx, userID)
}
func (m *mockAddressRepo) GetByUUID(ctx context.Context, userID int32, addrUUID uuid.UUID) (sqlc.UserAddress, error) {
	return m.getByUUIDFn(ctx, userID, addrUUID)
}
func (m *mockAddressRepo) Update(ctx context.Context, p sqlc.UpdateUserAddressParams) (sqlc.UserAddress, error) {
	return m.updateFn(ctx, p)
}
func (m *mockAddressRepo) SetDefault(ctx context.Context, tx pgx.Tx, userID int32, addrUUID uuid.UUID) (sqlc.UserAddress, error) {
	return m.setDefaultFn(ctx, tx, userID, addrUUID)
}
func (m *mockAddressRepo) Delete(ctx context.Context, tx pgx.Tx, userID int32, addrUUID uuid.UUID) (bool, error) {
	return m.deleteFn(ctx, tx, userID, addrUUID)
}
func (m *mockAddressRepo) SetLatestAsDefault(ctx context.Context, tx pgx.Tx, userID int32) error {
	m.setLatestDefCalls++
	if m.setLatestDefFn != nil {
		return m.setLatestDefFn(ctx, tx, userID)
	}
	return nil
}

func validAddressInput() dto.CreateAddressInput {
	return dto.CreateAddressInput{
		Label:        "Home",
		Name:         "Alice",
		Phone:        "0901234567",
		Address:      "123 Le Loi",
		ProvinceID:   202,
		ProvinceName: "Ho Chi Minh",
		DistrictID:   1442,
		DistrictName: "District 1",
		WardCode:     "20101",
		WardName:     "Ben Nghe",
	}
}

func TestAddressService_CreateUserAddress_BoundariesAndFirstDefault(t *testing.T) {
	uUUID := uuid.New()
	userRepo := &mockIdentityUserRepo{
		getByUUIDFn: func(_ context.Context, id uuid.UUID) (sqlc.User, error) {
			if id == uUUID {
				return sqlc.User{UserID: 10, UserUuid: uUUID}, nil
			}
			return sqlc.User{}, pgx.ErrNoRows
		},
	}

	t.Run("first address automatically becomes default", func(t *testing.T) {
		repo := &mockAddressRepo{
			countFn: func(context.Context, int32) (int64, error) { return 0, nil },
		}
		svc := NewAddressService(repo, userRepo)
		res, err := svc.CreateUserAddress(context.Background(), uUUID, validAddressInput())

		require.NoError(t, err)
		assert.True(t, res.IsDefault)
		assert.Equal(t, 1, repo.clearDefCalls)
		assert.True(t, repo.tx.committed)
		assert.Contains(t, res.FullAddress, "Ben Nghe, District 1, Ho Chi Minh")
	})

	t.Run("rejects when user already has maxAddressesPerUser (5)", func(t *testing.T) {
		repo := &mockAddressRepo{
			countFn: func(context.Context, int32) (int64, error) { return 5, nil },
		}
		svc := NewAddressService(repo, userRepo)
		_, err := svc.CreateUserAddress(context.Background(), uUUID, validAddressInput())

		assertIdentityErrCode(t, err, utils.ErrCodeBadRequest)
		assert.False(t, repo.tx.committed)
		assert.True(t, repo.tx.rolledBack)
	})

	t.Run("user not found returns ErrCodeNotFound", func(t *testing.T) {
		svc := NewAddressService(&mockAddressRepo{}, userRepo)
		_, err := svc.CreateUserAddress(context.Background(), uuid.New(), validAddressInput())
		assertIdentityErrCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("rejects blank required fields", func(t *testing.T) {
		svc := NewAddressService(&mockAddressRepo{}, userRepo)
		bad := validAddressInput()
		bad.Phone = "   "
		_, err := svc.CreateUserAddress(context.Background(), uUUID, bad)
		assertIdentityErrCode(t, err, utils.ErrCodeBadRequest)
	})
}

func TestAddressService_ReadAndUpdate(t *testing.T) {
	uUUID := uuid.New()
	aUUID := uuid.New()
	userRepo := &mockIdentityUserRepo{
		getByUUIDFn: func(context.Context, uuid.UUID) (sqlc.User, error) {
			return sqlc.User{UserID: 10, UserUuid: uUUID}, nil
		},
	}

	t.Run("GetMyAddresses returns ErrCodeNotFound when empty", func(t *testing.T) {
		repo := &mockAddressRepo{
			getByUserIDFn: func(context.Context, int32) ([]sqlc.UserAddress, error) { return nil, nil },
		}
		svc := NewAddressService(repo, userRepo)
		_, err := svc.GetMyAddresses(context.Background(), uUUID)
		assertIdentityErrCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("GetAddressByUUID maps not found and found", func(t *testing.T) {
		repo := &mockAddressRepo{
			getByUUIDFn: func(_ context.Context, _ int32, id uuid.UUID) (sqlc.UserAddress, error) {
				if id == aUUID {
					return sqlc.UserAddress{UaUuid: aUUID, UaLabel: "Work"}, nil
				}
				return sqlc.UserAddress{}, pgx.ErrNoRows
			},
		}
		svc := NewAddressService(repo, userRepo)

		got, err := svc.GetAddressByUUID(context.Background(), uUUID, aUUID)
		require.NoError(t, err)
		assert.Equal(t, "Work", got.Label)

		_, errNF := svc.GetAddressByUUID(context.Background(), uUUID, uuid.New())
		assertIdentityErrCode(t, errNF, utils.ErrCodeNotFound)
	})

	t.Run("UpdateUserAddress patches fields and handles not found", func(t *testing.T) {
		newLabel := "Office"
		repo := &mockAddressRepo{
			updateFn: func(_ context.Context, p sqlc.UpdateUserAddressParams) (sqlc.UserAddress, error) {
				if p.UaUuid == aUUID {
					return sqlc.UserAddress{UaUuid: aUUID, UaLabel: *p.Label}, nil
				}
				return sqlc.UserAddress{}, pgx.ErrNoRows
			},
		}
		svc := NewAddressService(repo, userRepo)
		got, err := svc.UpdateUserAddress(context.Background(), uUUID, aUUID, dto.UpdateAddressInput{Label: &newLabel})
		require.NoError(t, err)
		assert.Equal(t, "Office", got.Label)

		_, errNF := svc.UpdateUserAddress(context.Background(), uUUID, uuid.New(), dto.UpdateAddressInput{Label: &newLabel})
		assertIdentityErrCode(t, errNF, utils.ErrCodeNotFound)
	})
}

func TestAddressService_SetDefaultAndDelete(t *testing.T) {
	uUUID := uuid.New()
	aUUID := uuid.New()
	userRepo := &mockIdentityUserRepo{
		getByUUIDFn: func(context.Context, uuid.UUID) (sqlc.User, error) {
			return sqlc.User{UserID: 10, UserUuid: uUUID}, nil
		},
	}

	t.Run("SetDefaultUserAddress commits on success and rolls back on ErrNoRows", func(t *testing.T) {
		repo := &mockAddressRepo{
			setDefaultFn: func(_ context.Context, _ pgx.Tx, _ int32, id uuid.UUID) (sqlc.UserAddress, error) {
				if id == aUUID {
					return sqlc.UserAddress{UaUuid: aUUID, UaIsDefault: true}, nil
				}
				return sqlc.UserAddress{}, pgx.ErrNoRows
			},
		}
		svc := NewAddressService(repo, userRepo)
		require.NoError(t, svc.SetDefaultUserAddress(context.Background(), uUUID, aUUID))
		assert.True(t, repo.tx.committed)

		repo.tx = &fakeAddrTx{}
		err := svc.SetDefaultUserAddress(context.Background(), uUUID, uuid.New())
		assertIdentityErrCode(t, err, utils.ErrCodeNotFound)
		assert.False(t, repo.tx.committed)
		assert.True(t, repo.tx.rolledBack)
	})

	t.Run("DeleteUserAddress promotes latest when deleting default address", func(t *testing.T) {
		repo := &mockAddressRepo{
			getByUUIDFn: func(context.Context, int32, uuid.UUID) (sqlc.UserAddress, error) {
				return sqlc.UserAddress{UaUuid: aUUID, UaIsDefault: true}, nil
			},
			deleteFn: func(context.Context, pgx.Tx, int32, uuid.UUID) (bool, error) {
				return true, nil
			},
		}
		svc := NewAddressService(repo, userRepo)
		require.NoError(t, svc.DeleteUserAddress(context.Background(), uUUID, aUUID))
		assert.Equal(t, 1, repo.setLatestDefCalls)
		assert.True(t, repo.tx.committed)
	})

	t.Run("DeleteUserAddress skips SetLatestAsDefault for non-default and handles DB error", func(t *testing.T) {
		repo := &mockAddressRepo{
			getByUUIDFn: func(context.Context, int32, uuid.UUID) (sqlc.UserAddress, error) {
				return sqlc.UserAddress{UaUuid: aUUID, UaIsDefault: false}, nil
			},
			deleteFn: func(context.Context, pgx.Tx, int32, uuid.UUID) (bool, error) {
				return false, errors.New("db error")
			},
		}
		svc := NewAddressService(repo, userRepo)
		err := svc.DeleteUserAddress(context.Background(), uUUID, aUUID)
		assertIdentityErrCode(t, err, utils.ErrCodeInternal)
		assert.Equal(t, 0, repo.setLatestDefCalls)
	})
}
