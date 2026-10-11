package client

import (
	"context"
	"errors"
	"testing"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type mockIdentityClient struct {
	getUserByIDFunc           func(ctx context.Context, userID int32) (*v1dto.InternalUserDTO, error)
	getUserByUUIDFunc         func(ctx context.Context, userUUID uuid.UUID) (*v1dto.InternalUserDTO, error)
	getActiveAdminUserIDsFunc func(ctx context.Context) ([]int32, error)
	getUsersBatchFunc         func(ctx context.Context, userIDs []int32) (map[int32]v1dto.InternalUserDTO, error)
	getAddressByUUIDFunc      func(ctx context.Context, userID int32, addressUUID uuid.UUID) (*v1dto.AddressDTO, error)
	getAddressesByUserIDFunc  func(ctx context.Context, userID int32) ([]v1dto.AddressDTO, error)
}

func (m *mockIdentityClient) GetUserByID(ctx context.Context, userID int32) (*v1dto.InternalUserDTO, error) {
	if m.getUserByIDFunc != nil {
		return m.getUserByIDFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockIdentityClient) GetUserByUUID(ctx context.Context, userUUID uuid.UUID) (*v1dto.InternalUserDTO, error) {
	if m.getUserByUUIDFunc != nil {
		return m.getUserByUUIDFunc(ctx, userUUID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockIdentityClient) GetActiveAdminUserIDs(ctx context.Context) ([]int32, error) {
	if m.getActiveAdminUserIDsFunc != nil {
		return m.getActiveAdminUserIDsFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockIdentityClient) GetUsersBatch(ctx context.Context, userIDs []int32) (map[int32]v1dto.InternalUserDTO, error) {
	if m.getUsersBatchFunc != nil {
		return m.getUsersBatchFunc(ctx, userIDs)
	}
	return nil, errors.New("not implemented")
}

func (m *mockIdentityClient) GetAddressByUUID(ctx context.Context, userID int32, addressUUID uuid.UUID) (*v1dto.AddressDTO, error) {
	if m.getAddressByUUIDFunc != nil {
		return m.getAddressByUUIDFunc(ctx, userID, addressUUID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockIdentityClient) GetAddressesByUserID(ctx context.Context, userID int32) ([]v1dto.AddressDTO, error) {
	if m.getAddressesByUserIDFunc != nil {
		return m.getAddressesByUserIDFunc(ctx, userID)
	}
	return nil, errors.New("not implemented")
}

type mockFallbackReader struct {
	getByIDFunc               func(ctx context.Context, id int32) (sqlc.User, error)
	getByUUIDFunc             func(ctx context.Context, u uuid.UUID) (sqlc.User, error)
	getActiveAdminUserIDsFunc func(ctx context.Context) ([]int32, error)
}

func (m *mockFallbackReader) GetByID(ctx context.Context, id int32) (sqlc.User, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return sqlc.User{}, errors.New("fallback not implemented")
}

func (m *mockFallbackReader) GetByUUID(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
	if m.getByUUIDFunc != nil {
		return m.getByUUIDFunc(ctx, u)
	}
	return sqlc.User{}, errors.New("fallback not implemented")
}

func (m *mockFallbackReader) GetActiveAdminUserIDs(ctx context.Context) ([]int32, error) {
	if m.getActiveAdminUserIDsFunc != nil {
		return m.getActiveAdminUserIDsFunc(ctx)
	}
	return nil, errors.New("fallback not implemented")
}

func TestIdentityUserReader_GetByID(t *testing.T) {
	testUUID := uuid.New()

	t.Run("primary client succeeds", func(t *testing.T) {
		mockClient := &mockIdentityClient{
			getUserByIDFunc: func(ctx context.Context, userID int32) (*v1dto.InternalUserDTO, error) {
				return &v1dto.InternalUserDTO{
					UserID:       1,
					UserUUID:     testUUID,
					UserFullname: "Primary User",
					UserEmail:    "primary@example.com",
					UserStatus:   1,
					UserLevel:    3,
				}, nil
			},
		}

		reader := NewIdentityUserReader(mockClient, nil)
		user, err := reader.GetByID(context.Background(), 1)
		assert.NoError(t, err)
		assert.Equal(t, int32(1), user.UserID)
		assert.Equal(t, "Primary User", user.UserFullname)
	})

	t.Run("fallback when client fails", func(t *testing.T) {
		mockClient := &mockIdentityClient{
			getUserByIDFunc: func(ctx context.Context, userID int32) (*v1dto.InternalUserDTO, error) {
				return nil, errors.New("network down")
			},
		}
		mockFallback := &mockFallbackReader{
			getByIDFunc: func(ctx context.Context, id int32) (sqlc.User, error) {
				return sqlc.User{
					UserID:       1,
					UserUuid:     testUUID,
					UserFullname: "Fallback User",
					UserEmail:    "fallback@example.com",
				}, nil
			},
		}

		reader := NewIdentityUserReader(mockClient, mockFallback)
		user, err := reader.GetByID(context.Background(), 1)
		assert.NoError(t, err)
		assert.Equal(t, "Fallback User", user.UserFullname)
	})
}

func TestIdentityUserReader_GetByUUID(t *testing.T) {
	testUUID := uuid.New()

	t.Run("primary client succeeds", func(t *testing.T) {
		mockClient := &mockIdentityClient{
			getUserByUUIDFunc: func(ctx context.Context, userUUID uuid.UUID) (*v1dto.InternalUserDTO, error) {
				return &v1dto.InternalUserDTO{
					UserID:       5,
					UserUUID:     testUUID,
					UserFullname: "UUID Primary",
				}, nil
			},
		}

		reader := NewIdentityUserReader(mockClient, nil)
		user, err := reader.GetByUUID(context.Background(), testUUID)
		assert.NoError(t, err)
		assert.Equal(t, int32(5), user.UserID)
	})

	t.Run("fallback when client fails", func(t *testing.T) {
		mockClient := &mockIdentityClient{
			getUserByUUIDFunc: func(ctx context.Context, userUUID uuid.UUID) (*v1dto.InternalUserDTO, error) {
				return nil, errors.New("service timeout")
			},
		}
		mockFallback := &mockFallbackReader{
			getByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{
					UserID:       5,
					UserUuid:     u,
					UserFullname: "UUID Fallback",
				}, nil
			},
		}

		reader := NewIdentityUserReader(mockClient, mockFallback)
		user, err := reader.GetByUUID(context.Background(), testUUID)
		assert.NoError(t, err)
		assert.Equal(t, "UUID Fallback", user.UserFullname)
	})
}

func TestIdentityUserReader_GetActiveAdminUserIDs(t *testing.T) {
	t.Run("primary client succeeds", func(t *testing.T) {
		mockClient := &mockIdentityClient{
			getActiveAdminUserIDsFunc: func(ctx context.Context) ([]int32, error) {
				return []int32{1, 2}, nil
			},
		}

		reader := NewIdentityUserReader(mockClient, nil)
		admins, err := reader.GetActiveAdminUserIDs(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, []int32{1, 2}, admins)
	})

	t.Run("fallback succeeds", func(t *testing.T) {
		mockClient := &mockIdentityClient{
			getActiveAdminUserIDsFunc: func(ctx context.Context) ([]int32, error) {
				return nil, errors.New("client error")
			},
		}
		mockFallback := &mockFallbackReader{
			getActiveAdminUserIDsFunc: func(ctx context.Context) ([]int32, error) {
				return []int32{1, 3}, nil
			},
		}

		reader := NewIdentityUserReader(mockClient, mockFallback)
		admins, err := reader.GetActiveAdminUserIDs(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, []int32{1, 3}, admins)
	})
}
