package grpc_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/services/identity/internal/db/sqlc"
	identitygrpc "goshop/services/identity/internal/grpc"
	"goshop/services/identity/internal/repository"
	identityv1 "goshop/shared/proto/identity/v1"
)

type mockUserRepo struct {
	repository.UserRepository
	getByIDFunc               func(ctx context.Context, id int32) (sqlc.User, error)
	getByUUIDFunc             func(ctx context.Context, u uuid.UUID) (sqlc.User, error)
	getByIDsFunc              func(ctx context.Context, ids []int32) ([]sqlc.GetUsersByIDsRow, error)
	getActiveAdminUserIDsFunc func(ctx context.Context) ([]int32, error)
}

func (m *mockUserRepo) GetByID(ctx context.Context, id int32) (sqlc.User, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return sqlc.User{}, nil
}

func (m *mockUserRepo) GetByUUID(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
	if m.getByUUIDFunc != nil {
		return m.getByUUIDFunc(ctx, u)
	}
	return sqlc.User{}, nil
}

func (m *mockUserRepo) GetByIDs(ctx context.Context, ids []int32) ([]sqlc.GetUsersByIDsRow, error) {
	if m.getByIDsFunc != nil {
		return m.getByIDsFunc(ctx, ids)
	}
	return nil, nil
}

func (m *mockUserRepo) GetActiveAdminUserIDs(ctx context.Context) ([]int32, error) {
	if m.getActiveAdminUserIDsFunc != nil {
		return m.getActiveAdminUserIDsFunc(ctx)
	}
	return nil, nil
}

type mockAddressRepo struct {
	repository.AddressRepository
	getByUUIDFunc   func(ctx context.Context, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error)
	getByUserIDFunc func(ctx context.Context, userID int32) ([]sqlc.UserAddress, error)
}

func (m *mockAddressRepo) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return nil, nil
}

func (m *mockAddressRepo) GetByUUID(ctx context.Context, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error) {
	if m.getByUUIDFunc != nil {
		return m.getByUUIDFunc(ctx, userID, addressUUID)
	}
	return sqlc.UserAddress{}, nil
}

func (m *mockAddressRepo) GetByUserID(ctx context.Context, userID int32) ([]sqlc.UserAddress, error) {
	if m.getByUserIDFunc != nil {
		return m.getByUserIDFunc(ctx, userID)
	}
	return nil, nil
}

func TestServer_GetUserByID(t *testing.T) {
	testUUID := uuid.New()
	userRepo := &mockUserRepo{
		getByIDFunc: func(ctx context.Context, id int32) (sqlc.User, error) {
			return sqlc.User{
				UserID:       id,
				UserUuid:     testUUID,
				UserFullname: "Test User",
				UserEmail:    "test@example.com",
				UserStatus:   1,
				UserLevel:    3,
			}, nil
		},
	}
	srv := identitygrpc.NewServer(userRepo, &mockAddressRepo{})

	res, err := srv.GetUserByID(context.Background(), &identityv1.GetUserByIDRequest{
		UserId: 42,
	})
	require.NoError(t, err)
	require.NotNil(t, res.GetUser())
	assert.Equal(t, int32(42), res.GetUser().GetUserId())
	assert.Equal(t, testUUID.String(), res.GetUser().GetUserUuid())
	assert.Equal(t, "Test User", res.GetUser().GetUserFullname())
	assert.Equal(t, "test@example.com", res.GetUser().GetUserEmail())
}

func TestServer_GetUserByUUID(t *testing.T) {
	testUUID := uuid.New()
	userRepo := &mockUserRepo{
		getByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
			return sqlc.User{
				UserID:       100,
				UserUuid:     u,
				UserFullname: "UUID User",
				UserEmail:    "uuid@example.com",
			}, nil
		},
	}
	srv := identitygrpc.NewServer(userRepo, &mockAddressRepo{})

	res, err := srv.GetUserByUUID(context.Background(), &identityv1.GetUserByUUIDRequest{
		UserUuid: testUUID.String(),
	})
	require.NoError(t, err)
	require.NotNil(t, res.GetUser())
	assert.Equal(t, int32(100), res.GetUser().GetUserId())
	assert.Equal(t, testUUID.String(), res.GetUser().GetUserUuid())
}

func TestServer_GetBatchUsers(t *testing.T) {
	u1 := uuid.New()
	u2 := uuid.New()
	userRepo := &mockUserRepo{
		getByIDsFunc: func(ctx context.Context, ids []int32) ([]sqlc.GetUsersByIDsRow, error) {
			return []sqlc.GetUsersByIDsRow{
				{UserID: 1, UserUuid: u1, UserFullname: "User 1"},
				{UserID: 2, UserUuid: u2, UserFullname: "User 2"},
			}, nil
		},
	}
	srv := identitygrpc.NewServer(userRepo, &mockAddressRepo{})

	res, err := srv.GetBatchUsers(context.Background(), &identityv1.GetBatchUsersRequest{
		UserIds: []int32{1, 2},
	})
	require.NoError(t, err)
	assert.Len(t, res.GetUsers(), 2)
	assert.Equal(t, "User 1", res.GetUsers()[1].GetUserFullname())
	assert.Equal(t, "User 2", res.GetUsers()[2].GetUserFullname())
}

func TestServer_GetActiveAdminUserIDs(t *testing.T) {
	userRepo := &mockUserRepo{
		getActiveAdminUserIDsFunc: func(ctx context.Context) ([]int32, error) {
			return []int32{1, 5, 9}, nil
		},
	}
	srv := identitygrpc.NewServer(userRepo, &mockAddressRepo{})

	res, err := srv.GetActiveAdminUserIDs(context.Background(), &identityv1.GetActiveAdminUserIDsRequest{})
	require.NoError(t, err)
	assert.Equal(t, []int32{1, 5, 9}, res.GetAdminUserIds())
}

func TestServer_GetAddressByUUID(t *testing.T) {
	addrUUID := uuid.New()
	pID := int32(79)
	pName := "TP Ho Chi Minh"
	addressRepo := &mockAddressRepo{
		getByUUIDFunc: func(ctx context.Context, userID int32, u uuid.UUID) (sqlc.UserAddress, error) {
			return sqlc.UserAddress{
				UaID:           1,
				UaUuid:         u,
				UaUserID:       userID,
				UaName:         "Nguyen Van A",
				UaPhone:        "0901234567",
				UaAddress:      "123 Nguyen Hue",
				UaProvinceID:   &pID,
				UaProvinceName: &pName,
				UaIsDefault:    true,
			}, nil
		},
	}
	srv := identitygrpc.NewServer(&mockUserRepo{}, addressRepo)

	res, err := srv.GetAddressByUUID(context.Background(), &identityv1.GetAddressByUUIDRequest{
		UserId:      10,
		AddressUuid: addrUUID.String(),
	})
	require.NoError(t, err)
	require.NotNil(t, res.GetAddress())
	assert.Equal(t, addrUUID.String(), res.GetAddress().GetUuid())
	assert.Equal(t, "Nguyen Van A", res.GetAddress().GetName())
	assert.True(t, res.GetAddress().GetIsDefault())
	assert.Equal(t, int32(79), res.GetAddress().GetProvinceId())
}

func TestServer_GetAddressesByUserID(t *testing.T) {
	addr1 := uuid.New()
	addr2 := uuid.New()
	addressRepo := &mockAddressRepo{
		getByUserIDFunc: func(ctx context.Context, userID int32) ([]sqlc.UserAddress, error) {
			return []sqlc.UserAddress{
				{UaID: 1, UaUuid: addr1, UaUserID: userID, UaName: "Home"},
				{UaID: 2, UaUuid: addr2, UaUserID: userID, UaName: "Office"},
			}, nil
		},
	}
	srv := identitygrpc.NewServer(&mockUserRepo{}, addressRepo)

	res, err := srv.GetAddressesByUserID(context.Background(), &identityv1.GetAddressesByUserIDRequest{
		UserId: 5,
	})
	require.NoError(t, err)
	assert.Len(t, res.GetAddresses(), 2)
	assert.Equal(t, "Home", res.GetAddresses()[0].GetName())
	assert.Equal(t, "Office", res.GetAddresses()[1].GetName())
}
