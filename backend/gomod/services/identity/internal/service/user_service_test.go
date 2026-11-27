package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goshop/contracts/events"
	"goshop/services/identity/internal/db/sqlc"
	dto "goshop/services/identity/internal/dto"
	"goshop/shared/pkg/utils"
)

func TestGetAllUsers_FiltersPaginatesAndCaches(t *testing.T) {
	uUUID := uuid.New()
	repoCalls := 0
	repo := &mockIdentityUserRepo{
		getAllV2Fn: func(_ context.Context, search, orderBy, sort string, limit, offset int32, level, status *int32, deleted bool) ([]sqlc.User, error) {
			repoCalls++
			assert.Equal(t, "alice", search)
			assert.Equal(t, int32(10), limit)
			assert.Equal(t, int32(10), offset)
			return []sqlc.User{
				{UserID: 1, UserUuid: uUUID, UserFullname: "Alice", UserEmail: "alice@example.com", UserStatus: 1, UserLevel: 3},
			}, nil
		},
		countUsersFn: func(context.Context, string, *int32, *int32, bool) (int64, error) {
			return 11, nil
		},
	}

	us := &userService{repo: repo, cache: newMemCache()}

	users, total, err := us.GetAllUsers(context.Background(), "alice", "", "", 2, 10, nil, nil, false)
	require.NoError(t, err)
	assert.Equal(t, int32(11), total)
	require.Len(t, users, 1)
	assert.Equal(t, "Alice", users[0].Name)
	assert.Equal(t, 1, repoCalls)

	// Second call hits cache
	users2, total2, err := us.GetAllUsers(context.Background(), "alice", "", "", 2, 10, nil, nil, false)
	require.NoError(t, err)
	assert.Equal(t, int32(11), total2)
	require.Len(t, users2, 1)
	assert.Equal(t, 1, repoCalls)
}

func TestCreateUsers_PublishesOutboxAndHandlesConflict(t *testing.T) {
	uUUID := uuid.New()
	outbox := &mockOutboxRepo{}
	broker := &mockIdentityBroker{}

	repo := &mockIdentityUserRepo{
		createFn: func(_ context.Context, input sqlc.CreateUserParams) (sqlc.User, error) {
			if input.UserEmail == "dup@example.com" {
				return sqlc.User{}, &pgconn.PgError{Code: "23505"}
			}
			return sqlc.User{
				UserID:       20,
				UserUuid:     uUUID,
				UserFullname: input.UserFullname,
				UserEmail:    input.UserEmail,
				UserStatus:   input.UserStatus,
				UserLevel:    input.UserLevel,
			}, nil
		},
	}

	us := NewUserServiceWithOutbox(repo, nil, broker, outbox, &mockTokenService{})

	created, err := us.CreateUsers(context.Background(), dto.CreateUserInput{
		Name:     "Bob",
		Email:    "BOB@example.com",
		Password: "Password123!",
		Status:   1,
		Level:    3,
	})
	require.NoError(t, err)
	assert.Equal(t, "bob@example.com", created.Email)
	assert.Equal(t, []string{events.QueueUserCreated}, outbox.inserted)
	assert.Equal(t, []int64{1}, outbox.marked)

	_, errDup := us.CreateUsers(context.Background(), dto.CreateUserInput{
		Name:   "Dup",
		Email:  "dup@example.com",
		Status: 1,
		Level:  3,
	})
	assertIdentityErrCode(t, errDup, utils.ErrCodeConflict)
}

func TestGetUserByUUID_FoundAndNotFound(t *testing.T) {
	uUUID := uuid.New()
	repo := &mockIdentityUserRepo{
		getByUUIDFn: func(_ context.Context, id uuid.UUID) (sqlc.User, error) {
			if id == uUUID {
				return sqlc.User{UserUuid: uUUID, UserFullname: "Found"}, nil
			}
			return sqlc.User{}, pgx.ErrNoRows
		},
	}
	us := NewUserService(repo, nil)

	got, err := us.GetUserByUUID(context.Background(), uUUID)
	require.NoError(t, err)
	assert.Equal(t, "Found", got.Name)

	_, errNotFound := us.GetUserByUUID(context.Background(), uuid.New())
	assertIdentityErrCode(t, errNotFound, utils.ErrCodeNotFound)
}

func TestChangePassword_ValidationAndRevokesSessions(t *testing.T) {
	uUUID := uuid.New()
	tokens := &mockTokenService{}
	var savedHash string

	repo := &mockIdentityUserRepo{
		getByUUIDFn: func(_ context.Context, id uuid.UUID) (sqlc.User, error) {
			return sqlc.User{
				UserUuid:     id,
				UserPassword: hashedPass(t, "OldPass123!"),
			}, nil
		},
		updatePasswordFn: func(_ context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error) {
			savedHash = *input.NewPassword
			return sqlc.User{UserUuid: input.UserUuid}, nil
		},
	}

	us := NewUserServiceWithOutbox(repo, nil, nil, nil, tokens)

	errWrong := us.ChangePassword(context.Background(), uUUID, dto.ChangePasswordRequest{
		OldPassword: "WrongOld!",
		NewPassword: "NewPass123!",
	})
	assertIdentityErrCode(t, errWrong, utils.ErrCodeUnauthorized)

	errSame := us.ChangePassword(context.Background(), uUUID, dto.ChangePasswordRequest{
		OldPassword: "OldPass123!",
		NewPassword: "OldPass123!",
	})
	assertIdentityErrCode(t, errSame, utils.ErrCodeBadRequest)

	require.NoError(t, us.ChangePassword(context.Background(), uUUID, dto.ChangePasswordRequest{
		OldPassword: "OldPass123!",
		NewPassword: "NewPass123!",
	}))
	assert.NotEmpty(t, savedHash)
	assert.Equal(t, []string{uUUID.String()}, tokens.revokedUsers)
}

func TestSoftDeleteRestoreAndTrashUser(t *testing.T) {
	uUUID := uuid.New()
	outbox := &mockOutboxRepo{}
	repo := &mockIdentityUserRepo{
		softDeleteFn: func(_ context.Context, id uuid.UUID) (sqlc.User, error) {
			return sqlc.User{UserID: 7, UserUuid: id, UserEmail: "del@example.com"}, nil
		},
		restoreFn: func(_ context.Context, id uuid.UUID) (sqlc.User, error) {
			return sqlc.User{UserID: 7, UserUuid: id, UserEmail: "del@example.com"}, nil
		},
	}

	us := NewUserServiceWithOutbox(repo, nil, &mockIdentityBroker{}, outbox, nil)

	_, err := us.SoftDeleteUser(context.Background(), uUUID)
	require.NoError(t, err)
	assert.Contains(t, outbox.inserted, events.QueueUserDeleted)

	_, err = us.RestoreUser(context.Background(), uUUID)
	require.NoError(t, err)
	assert.Contains(t, outbox.inserted, events.QueueUserUpdated)

	require.NoError(t, us.TrashUser(context.Background(), uUUID))
}

func TestUpdateUserAndProfile_SuccessNotFoundAndConflict(t *testing.T) {
	uUUID := uuid.New()
	outbox := &mockOutboxRepo{}
	repo := &mockIdentityUserRepo{
		getByUUIDFn: func(_ context.Context, id uuid.UUID) (sqlc.User, error) {
			if id == uUUID {
				return sqlc.User{UserID: 5, UserUuid: uUUID, UserFullname: "Alice", UserEmail: "alice@example.com"}, nil
			}
			return sqlc.User{}, pgx.ErrNoRows
		},
		updateFn: func(_ context.Context, p sqlc.UpdateUserParams) (sqlc.User, error) {
			if p.UserUuid != uUUID {
				return sqlc.User{}, pgx.ErrNoRows
			}
			if p.UserFullname != nil && *p.UserFullname == "DupName" {
				return sqlc.User{}, &pgconn.PgError{Code: "23505"}
			}
			name := "Alice"
			if p.UserFullname != nil {
				name = *p.UserFullname
			}
			return sqlc.User{UserID: 5, UserUuid: uUUID, UserFullname: name, UserEmail: "alice@example.com"}, nil
		},
	}

	us := NewUserServiceWithOutbox(repo, nil, &mockIdentityBroker{}, outbox, nil)

	prof, err := us.GetProfile(context.Background(), uUUID)
	require.NoError(t, err)
	assert.Equal(t, "Alice", prof.Name)

	_, errNF := us.GetProfile(context.Background(), uuid.New())
	assertIdentityErrCode(t, errNF, utils.ErrCodeNotFound)

	newName := "Alice Updated"
	newPass := "NewSecret123!"
	updated, err := us.UpdateUser(context.Background(), uUUID, dto.UpdateUserInput{Name: &newName, Password: &newPass})
	require.NoError(t, err)
	assert.Equal(t, "Alice Updated", updated.Name)

	profUpdated, err := us.UpdateProfile(context.Background(), uUUID, dto.UpdateProfileInput{Name: &newName, Password: &newPass})
	require.NoError(t, err)
	assert.Equal(t, "Alice Updated", profUpdated.Name)

	dup := "DupName"
	_, errConflict := us.UpdateUser(context.Background(), uUUID, dto.UpdateUserInput{Name: &dup})
	assertIdentityErrCode(t, errConflict, utils.ErrCodeConflict)
}
