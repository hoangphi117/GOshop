package testutils

import (
	"context"

	"goshop/internal/db/sqlc"
	"goshop/internal/repository"

	"github.com/google/uuid"
)

// Ensure the remaining mock implements the repository interfaces at compile time
var (
	_ repository.UserRepository = (*MockUserRepository)(nil)
	_ repository.UserReader     = (*MockUserRepository)(nil)
)

// =========================================================================
// 3. MockProductRepository
// =========================================================================
type MockUserRepository struct {
	GetAllFunc                func(ctx context.Context, search, orderBy, sort string, limit, offset int32) ([]sqlc.User, error)
	GetAllV2Func              func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error)
	CountUsersFunc            func(ctx context.Context, search string, deleted bool) (int64, error)
	CreateFunc                func(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	GetByIDFunc               func(ctx context.Context, id int32) (sqlc.User, error)
	GetByUUIDFunc             func(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	UpdateFunc                func(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	SoftDeleteFunc            func(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	RestoreFunc               func(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	TrashFunc                 func(ctx context.Context, uuid uuid.UUID) (sqlc.User, error)
	GetByEmailFunc            func(ctx context.Context, email string) (sqlc.User, error)
	GetActiveAdminUserIDsFunc func(ctx context.Context) ([]int32, error)
	UpdatePasswordFunc        func(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error)
	GetByOAuthFunc            func(ctx context.Context, provider, oauthID string) (sqlc.User, error)
	CreateOAuthUserFunc       func(ctx context.Context, params sqlc.CreateOAuthUserParams) (sqlc.User, error)
	GetByIDsFunc              func(ctx context.Context, ids []int32) ([]sqlc.GetUsersByIDsRow, error)
}

func (m *MockUserRepository) GetByIDs(ctx context.Context, ids []int32) ([]sqlc.GetUsersByIDsRow, error) {
	if m.GetByIDsFunc != nil {
		return m.GetByIDsFunc(ctx, ids)
	}
	return []sqlc.GetUsersByIDsRow{}, nil
}

func (m *MockUserRepository) GetByID(ctx context.Context, id int32) (sqlc.User, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) GetAll(ctx context.Context, search, orderBy, sort string, limit, offset int32) ([]sqlc.User, error) {
	if m.GetAllFunc != nil {
		return m.GetAllFunc(ctx, search, orderBy, sort, limit, offset)
	}
	return nil, nil
}

func (m *MockUserRepository) GetAllV2(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error) {
	if m.GetAllV2Func != nil {
		return m.GetAllV2Func(ctx, search, orderBy, sort, limit, offset, deleted)
	}
	return nil, nil
}

func (m *MockUserRepository) CountUsers(ctx context.Context, search string, deleted bool) (int64, error) {
	if m.CountUsersFunc != nil {
		return m.CountUsersFunc(ctx, search, deleted)
	}
	return 0, nil
}

func (m *MockUserRepository) Create(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, input)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.User, error) {
	if m.GetByUUIDFunc != nil {
		return m.GetByUUIDFunc(ctx, uuid)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) Update(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, input)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.User, error) {
	if m.SoftDeleteFunc != nil {
		return m.SoftDeleteFunc(ctx, uuid)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) Restore(ctx context.Context, uuid uuid.UUID) (sqlc.User, error) {
	if m.RestoreFunc != nil {
		return m.RestoreFunc(ctx, uuid)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) Trash(ctx context.Context, uuid uuid.UUID) (sqlc.User, error) {
	if m.TrashFunc != nil {
		return m.TrashFunc(ctx, uuid)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (sqlc.User, error) {
	if m.GetByEmailFunc != nil {
		return m.GetByEmailFunc(ctx, email)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) GetActiveAdminUserIDs(ctx context.Context) ([]int32, error) {
	if m.GetActiveAdminUserIDsFunc != nil {
		return m.GetActiveAdminUserIDsFunc(ctx)
	}
	return nil, nil
}

func (m *MockUserRepository) UpdatePassword(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error) {
	if m.UpdatePasswordFunc != nil {
		return m.UpdatePasswordFunc(ctx, input)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) GetByOAuth(ctx context.Context, provider, oauthID string) (sqlc.User, error) {
	if m.GetByOAuthFunc != nil {
		return m.GetByOAuthFunc(ctx, provider, oauthID)
	}
	return sqlc.User{}, nil
}

func (m *MockUserRepository) CreateOAuthUser(ctx context.Context, params sqlc.CreateOAuthUserParams) (sqlc.User, error) {
	if m.CreateOAuthUserFunc != nil {
		return m.CreateOAuthUserFunc(ctx, params)
	}
	return sqlc.User{}, nil
}

// =========================================================================
// =========================================================================
