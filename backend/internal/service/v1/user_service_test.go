package v1service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestUserService_GetAllUsers(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()

	t.Run("Case 1: Happy Path - Queries Repo and Returns Users with Total", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetAllV2Func: func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error) {
				assert.Equal(t, "john", search)
				assert.Equal(t, "user_id", orderBy)
				assert.Equal(t, "asc", sort)
				assert.Equal(t, int32(10), limit)
				assert.Equal(t, int32(0), offset)
				assert.False(t, deleted)
				return []sqlc.User{
					{
						UserID:       1,
						UserUuid:     userUUID,
						UserFullname: "John Doe",
						UserEmail:    "john@example.com",
					},
				}, nil
			},
			CountUsersFunc: func(ctx context.Context, search string, deleted bool) (int64, error) {
				return 1, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		users, total, err := svc.GetAllUsers(ctx, "john", "user_id", "asc", 1, 10, false)
		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Len(t, users, 1)
		assert.Equal(t, userUUID, users[0].UserUuid)
	})

	t.Run("Case 2: Default Parameter Fallbacks", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetAllV2Func: func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error) {
				assert.Equal(t, "desc", sort)
				assert.Equal(t, "user_created_at", orderBy)
				assert.Equal(t, int32(0), offset)
				return []sqlc.User{}, nil
			},
			CountUsersFunc: func(ctx context.Context, search string, deleted bool) (int64, error) {
				return 0, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		users, total, err := svc.GetAllUsers(ctx, "", "", "", 0, 0, false)
		require.NoError(t, err)
		assert.Equal(t, int32(0), total)
		assert.Empty(t, users)
	})

	t.Run("Case 3: Repo GetAllV2 Error - Returns 500", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetAllV2Func: func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error) {
				return nil, errors.New("db query error")
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		_, _, err := svc.GetAllUsers(ctx, "", "", "", 1, 10, false)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 4: Repo CountUsers Error - Returns 500", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetAllV2Func: func(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error) {
				return []sqlc.User{}, nil
			},
			CountUsersFunc: func(ctx context.Context, search string, deleted bool) (int64, error) {
				return 0, errors.New("count query error")
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		_, _, err := svc.GetAllUsers(ctx, "", "", "", 1, 10, false)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})
}

func TestUserService_CreateUsers(t *testing.T) {
	ctx := context.Background()

	t.Run("Case 1: Duplicate Email 23505 - Returns 409 Conflict", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			CreateFunc: func(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error) {
				return sqlc.User{}, &pgconn.PgError{Code: "23505", Message: "duplicate key"}
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		pass123 := "password123"
		_, err := svc.CreateUsers(ctx, sqlc.CreateUserParams{
			UserFullname: "Alice",
			UserEmail:    "alice@example.com",
			UserPassword: &pass123,
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
		assert.Contains(t, err.Error(), "email already exists")
	})

	t.Run("Case 2: Repo Error on Create - Returns 500", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			CreateFunc: func(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error) {
				return sqlc.User{}, errors.New("db insert failed")
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		pass123 := "password123"
		_, err := svc.CreateUsers(ctx, sqlc.CreateUserParams{
			UserFullname: "Alice",
			UserEmail:    "alice@example.com",
			UserPassword: &pass123,
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 3: Happy Path - Hashes Password and Creates User", func(t *testing.T) {
		userUUID := uuid.New()
		var capturedParams sqlc.CreateUserParams

		mockRepo := &testutils.MockUserRepository{
			CreateFunc: func(ctx context.Context, input sqlc.CreateUserParams) (sqlc.User, error) {
				capturedParams = input
				return sqlc.User{
					UserID:       1,
					UserUuid:     userUUID,
					UserFullname: input.UserFullname,
					UserEmail:    input.UserEmail,
					UserPassword: input.UserPassword,
				}, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		plainPass := "plainpassword"
		res, err := svc.CreateUsers(ctx, sqlc.CreateUserParams{
			UserFullname: "Alice",
			UserEmail:    "  ALICE@example.com  ",
			UserPassword: &plainPass,
		})
		require.NoError(t, err)
		assert.Equal(t, userUUID, res.UserUuid)
		assert.Equal(t, "alice@example.com", capturedParams.UserEmail)
		require.NotNil(t, capturedParams.UserPassword)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(*capturedParams.UserPassword), []byte("plainpassword")))
	})
}

func TestUserService_GetUserByUUID(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()

	t.Run("Case 1: User Not Found (pgx.ErrNoRows) - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		_, err := svc.GetUserByUUID(ctx, userUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
		assert.Contains(t, err.Error(), "user not found")
	})

	t.Run("Case 2: User Not Found (sql.ErrNoRows) - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		_, err := svc.GetUserByUUID(ctx, userUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Happy Path - Returns User", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				assert.Equal(t, userUUID, u)
				return sqlc.User{
					UserID:       1,
					UserUuid:     userUUID,
					UserFullname: "Bob",
				}, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		res, err := svc.GetUserByUUID(ctx, userUUID)
		require.NoError(t, err)
		assert.Equal(t, userUUID, res.UserUuid)
		assert.Equal(t, "Bob", res.UserFullname)
	})
}

func TestUserService_UpdateUser(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	newName := "Updated Bob"
	plainPassword := "newpass123"

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			UpdateFunc: func(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		_, err := svc.UpdateUser(ctx, sqlc.UpdateUserParams{
			UserUuid: userUUID,
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Duplicate Email Conflict 23505 - Returns 409", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			UpdateFunc: func(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error) {
				return sqlc.User{}, &pgconn.PgError{Code: "23505", Message: "duplicate key"}
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		_, err := svc.UpdateUser(ctx, sqlc.UpdateUserParams{
			UserUuid: userUUID,
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeConflict)
	})

	t.Run("Case 3: Happy Path with Password Hashing - Success", func(t *testing.T) {
		var capturedParams sqlc.UpdateUserParams

		mockRepo := &testutils.MockUserRepository{
			UpdateFunc: func(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error) {
				capturedParams = input
				return sqlc.User{
					UserID:       1,
					UserUuid:     userUUID,
					UserFullname: *input.UserFullname,
				}, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		res, err := svc.UpdateUser(ctx, sqlc.UpdateUserParams{
			UserUuid:     userUUID,
			UserFullname: &newName,
			UserPassword: &plainPassword,
		})
		require.NoError(t, err)
		assert.Equal(t, userUUID, res.UserUuid)
		assert.Equal(t, newName, res.UserFullname)
		require.NotNil(t, capturedParams.UserPassword)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(*capturedParams.UserPassword), []byte(plainPassword)))
	})
}

func TestUserService_GetProfile(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		_, err := svc.GetProfile(ctx, userUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - Returns Profile", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				assert.Equal(t, userUUID, u)
				return sqlc.User{
					UserID:       1,
					UserUuid:     userUUID,
					UserFullname: "Charlie",
				}, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		user, err := svc.GetProfile(ctx, userUUID)
		require.NoError(t, err)
		assert.Equal(t, userUUID, user.UserUuid)
		assert.Equal(t, "Charlie", user.UserFullname)
	})
}

func TestUserService_UpdateProfile(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	newName := "Charlie Updated"

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			UpdateFunc: func(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		_, err := svc.UpdateProfile(ctx, userUUID, sqlc.UpdateUserParams{
			UserFullname: &newName,
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - Updates Profile Sanitizing Status/Level", func(t *testing.T) {
		var capturedParams sqlc.UpdateUserParams

		mockRepo := &testutils.MockUserRepository{
			UpdateFunc: func(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error) {
				capturedParams = input
				return sqlc.User{
					UserID:       1,
					UserUuid:     userUUID,
					UserFullname: *input.UserFullname,
				}, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		fakeStatus := int32(99)
		fakeLevel := int32(99)
		res, err := svc.UpdateProfile(ctx, userUUID, sqlc.UpdateUserParams{
			UserFullname: &newName,
			UserStatus:   &fakeStatus,
			UserLevel:    &fakeLevel,
		})
		require.NoError(t, err)
		assert.Equal(t, userUUID, res.UserUuid)
		assert.Nil(t, capturedParams.UserStatus)
		assert.Nil(t, capturedParams.UserLevel)
	})
}

func TestUserService_ChangePassword(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	oldPass := "oldpassword123"
	hashedOldPass, _ := bcrypt.GenerateFromPassword([]byte(oldPass), bcrypt.DefaultCost)
	hashedOldStr := string(hashedOldPass)

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		err := svc.ChangePassword(ctx, userUUID, v1dto.ChangePasswordRequest{
			OldPassword: oldPass,
			NewPassword: "newpassword123",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Incorrect Old Password - Returns 401 Unauthorized", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{
					UserID:       1,
					UserUuid:     userUUID,
					UserPassword: &hashedOldStr,
				}, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		err := svc.ChangePassword(ctx, userUUID, v1dto.ChangePasswordRequest{
			OldPassword: "wrongpassword",
			NewPassword: "newpassword123",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeUnauthorized)
		assert.Contains(t, err.Error(), "old password is incorrect")
	})

	t.Run("Case 3: New Password Same As Old - Returns 400 Bad Request", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{
					UserID:       1,
					UserUuid:     userUUID,
					UserPassword: &hashedOldStr,
				}, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		err := svc.ChangePassword(ctx, userUUID, v1dto.ChangePasswordRequest{
			OldPassword: oldPass,
			NewPassword: oldPass,
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "cannot be the same")
	})

	t.Run("Case 4: Happy Path - Updates Password Successfully", func(t *testing.T) {
		var capturedParams sqlc.UpdatePasswordParams

		mockRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{
					UserID:       1,
					UserUuid:     userUUID,
					UserPassword: &hashedOldStr,
				}, nil
			},
			UpdatePasswordFunc: func(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error) {
				capturedParams = input
				return sqlc.User{
					UserID:   1,
					UserUuid: userUUID,
				}, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		err := svc.ChangePassword(ctx, userUUID, v1dto.ChangePasswordRequest{
			OldPassword: oldPass,
			NewPassword: "newbrandpassword456",
		})
		require.NoError(t, err)
		assert.Equal(t, userUUID, capturedParams.UserUuid)
		require.NotNil(t, capturedParams.NewPassword)
		assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(*capturedParams.NewPassword), []byte("newbrandpassword456")))
	})
}

func TestUserService_SoftDeleteUser(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			SoftDeleteFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		_, err := svc.SoftDeleteUser(ctx, userUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - Soft Deletes User", func(t *testing.T) {
		now := time.Now()
		mockRepo := &testutils.MockUserRepository{
			SoftDeleteFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				assert.Equal(t, userUUID, u)
				return sqlc.User{
					UserID:        1,
					UserUuid:      userUUID,
					UserDeletedAt: pgtype.Timestamptz{Time: now, Valid: true},
				}, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		res, err := svc.SoftDeleteUser(ctx, userUUID)
		require.NoError(t, err)
		assert.Equal(t, userUUID, res.UserUuid)
		assert.True(t, res.UserDeletedAt.Valid)
	})
}

func TestUserService_RestoreUser(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			RestoreFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		_, err := svc.RestoreUser(ctx, userUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - Restores User", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			RestoreFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				assert.Equal(t, userUUID, u)
				return sqlc.User{
					UserID:        1,
					UserUuid:      userUUID,
					UserDeletedAt: pgtype.Timestamptz{Valid: false},
				}, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		res, err := svc.RestoreUser(ctx, userUUID)
		require.NoError(t, err)
		assert.Equal(t, userUUID, res.UserUuid)
		assert.False(t, res.UserDeletedAt.Valid)
	})
}

func TestUserService_TrashUser(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()

	t.Run("Case 1: User Not Found - Returns 404", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			TrashFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		err := svc.TrashUser(ctx, userUUID)
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path - Trashes User Permanently", func(t *testing.T) {
		mockRepo := &testutils.MockUserRepository{
			TrashFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				assert.Equal(t, userUUID, u)
				return sqlc.User{
					UserID:   1,
					UserUuid: userUUID,
				}, nil
			},
		}

		svc := v1service.NewUserService(mockRepo, nil)

		err := svc.TrashUser(ctx, userUUID)
		require.NoError(t, err)
	})
}
