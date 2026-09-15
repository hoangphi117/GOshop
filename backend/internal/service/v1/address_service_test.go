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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddressService_CreateUserAddress(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewAddressService(nil, mockUserRepo)
		_, err := svc.CreateUserAddress(ctx, userUUID, v1dto.CreateAddressInput{
			Label:   "Home",
			Name:    "John Doe",
			Phone:   "0123456789",
			Address: "123 Street",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Blank Fields in Input -> Returns 400 Bad Request", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}

		svc := v1service.NewAddressService(nil, mockUserRepo)

		// Missing Label
		_, err := svc.CreateUserAddress(ctx, userUUID, v1dto.CreateAddressInput{
			Label:   "   ",
			Name:    "John Doe",
			Phone:   "0123456789",
			Address: "123 Street",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)

		// Missing Name
		_, err = svc.CreateUserAddress(ctx, userUUID, v1dto.CreateAddressInput{
			Label:   "Home",
			Name:    "  ",
			Phone:   "0123456789",
			Address: "123 Street",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)

		// Missing Phone
		_, err = svc.CreateUserAddress(ctx, userUUID, v1dto.CreateAddressInput{
			Label:   "Home",
			Name:    "John",
			Phone:   "",
			Address: "123 Street",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)

		// Missing Address
		_, err = svc.CreateUserAddress(ctx, userUUID, v1dto.CreateAddressInput{
			Label:   "Home",
			Name:    "John",
			Phone:   "0123456789",
			Address: "   ",
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("Case 3: User Repo DB Failure -> Returns 500", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, errors.New("db error")
			},
		}

		svc := v1service.NewAddressService(nil, mockUserRepo)
		_, err := svc.CreateUserAddress(ctx, userUUID, v1dto.CreateAddressInput{
			Label:   "Home",
			Name:    "John",
			Phone:   "0123456789",
			Address: "123 Street",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})
}

func TestAddressService_GetMyAddresses(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	addrUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewAddressService(nil, mockUserRepo)
		_, err := svc.GetMyAddresses(ctx, userUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Repo Error -> Returns 500", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockAddressRepository{
			GetByUserIDFunc: func(ctx context.Context, userID int32) ([]sqlc.UserAddress, error) {
				return nil, errors.New("query failed")
			},
		}

		svc := v1service.NewAddressService(mockRepo, mockUserRepo)
		_, err := svc.GetMyAddresses(ctx, userUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 3: Zero Addresses -> Returns 404 Not Found", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockAddressRepository{
			GetByUserIDFunc: func(ctx context.Context, userID int32) ([]sqlc.UserAddress, error) {
				return []sqlc.UserAddress{}, nil
			},
		}

		svc := v1service.NewAddressService(mockRepo, mockUserRepo)
		addresses, err := svc.GetMyAddresses(ctx, userUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
		assert.Empty(t, addresses)
	})

	t.Run("Case 4: Happy Path -> Returns Mapped Addresses", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockAddressRepository{
			GetByUserIDFunc: func(ctx context.Context, userID int32) ([]sqlc.UserAddress, error) {
				return []sqlc.UserAddress{
					{
						UaID:        10,
						UaUuid:      addrUUID,
						UaUserID:    1,
						UaLabel:     "Home",
						UaName:      "John Doe",
						UaPhone:     "0123456789",
						UaAddress:   "123 Main St",
						UaIsDefault: true,
						UaCreatedAt: time.Now(),
						UaUpdatedAt: time.Now(),
					},
				}, nil
			},
		}

		svc := v1service.NewAddressService(mockRepo, mockUserRepo)
		addresses, err := svc.GetMyAddresses(ctx, userUUID)

		require.NoError(t, err)
		assert.Len(t, addresses, 1)
		assert.Equal(t, addrUUID.String(), addresses[0].UUID)
		assert.Equal(t, "Home", addresses[0].Label)
		assert.True(t, addresses[0].IsDefault)
	})
}

func TestAddressService_GetAddressByUUID(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	addrUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewAddressService(nil, mockUserRepo)
		_, err := svc.GetAddressByUUID(ctx, userUUID, addrUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Address Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockAddressRepository{
			GetByUUIDFunc: func(ctx context.Context, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error) {
				return sqlc.UserAddress{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewAddressService(mockRepo, mockUserRepo)
		_, err := svc.GetAddressByUUID(ctx, userUUID, addrUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Repository Error -> Returns 500", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockAddressRepository{
			GetByUUIDFunc: func(ctx context.Context, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error) {
				return sqlc.UserAddress{}, errors.New("db error")
			},
		}

		svc := v1service.NewAddressService(mockRepo, mockUserRepo)
		_, err := svc.GetAddressByUUID(ctx, userUUID, addrUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 4: Happy Path -> Returns Mapped Address", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockAddressRepository{
			GetByUUIDFunc: func(ctx context.Context, userID int32, addressUUID uuid.UUID) (sqlc.UserAddress, error) {
				return sqlc.UserAddress{
					UaID:        5,
					UaUuid:      addrUUID,
					UaUserID:    1,
					UaLabel:     "Office",
					UaName:      "John Doe",
					UaPhone:     "0987654321",
					UaAddress:   "456 Work Ave",
					UaIsDefault: false,
					UaCreatedAt: time.Now(),
					UaUpdatedAt: time.Now(),
				}, nil
			},
		}

		svc := v1service.NewAddressService(mockRepo, mockUserRepo)
		dto, err := svc.GetAddressByUUID(ctx, userUUID, addrUUID)

		require.NoError(t, err)
		assert.Equal(t, addrUUID.String(), dto.UUID)
		assert.Equal(t, "Office", dto.Label)
		assert.False(t, dto.IsDefault)
	})
}

func TestAddressService_UpdateUserAddress(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	addrUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewAddressService(nil, mockUserRepo)
		_, err := svc.UpdateUserAddress(ctx, userUUID, addrUUID, v1dto.UpdateAddressInput{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Blank Value In Update Fields -> Returns 400 Bad Request", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}

		svc := v1service.NewAddressService(nil, mockUserRepo)

		blankStr := "   "
		_, err := svc.UpdateUserAddress(ctx, userUUID, addrUUID, v1dto.UpdateAddressInput{
			Label: &blankStr,
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)

		_, err = svc.UpdateUserAddress(ctx, userUUID, addrUUID, v1dto.UpdateAddressInput{
			Name: &blankStr,
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)

		_, err = svc.UpdateUserAddress(ctx, userUUID, addrUUID, v1dto.UpdateAddressInput{
			Phone: &blankStr,
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)

		_, err = svc.UpdateUserAddress(ctx, userUUID, addrUUID, v1dto.UpdateAddressInput{
			Address: &blankStr,
		})
		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("Case 3: Address Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockAddressRepository{
			UpdateFunc: func(ctx context.Context, params sqlc.UpdateUserAddressParams) (sqlc.UserAddress, error) {
				return sqlc.UserAddress{}, sql.ErrNoRows
			},
		}

		newLabel := "Updated Home"
		svc := v1service.NewAddressService(mockRepo, mockUserRepo)
		_, err := svc.UpdateUserAddress(ctx, userUUID, addrUUID, v1dto.UpdateAddressInput{
			Label: &newLabel,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 4: Repository Error -> Returns 500", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockRepo := &testutils.MockAddressRepository{
			UpdateFunc: func(ctx context.Context, params sqlc.UpdateUserAddressParams) (sqlc.UserAddress, error) {
				return sqlc.UserAddress{}, errors.New("update error")
			},
		}

		newLabel := "Updated Home"
		svc := v1service.NewAddressService(mockRepo, mockUserRepo)
		_, err := svc.UpdateUserAddress(ctx, userUUID, addrUUID, v1dto.UpdateAddressInput{
			Label: &newLabel,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeInternal)
	})

	t.Run("Case 5: Happy Path -> Successfully Updates Address", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}

		newLabel := "New Villa"
		newName := "Alice"
		newPhone := "0911222333"
		newAddr := "789 Ocean Blvd"
		provID := 1
		distID := 2

		mockRepo := &testutils.MockAddressRepository{
			UpdateFunc: func(ctx context.Context, params sqlc.UpdateUserAddressParams) (sqlc.UserAddress, error) {
				return sqlc.UserAddress{
					UaID:        1,
					UaUuid:      addrUUID,
					UaUserID:    1,
					UaLabel:     *params.Label,
					UaName:      *params.Name,
					UaPhone:     *params.Phone,
					UaAddress:   *params.Address,
					UaIsDefault: false,
					UaCreatedAt: time.Now(),
					UaUpdatedAt: time.Now(),
				}, nil
			},
		}

		svc := v1service.NewAddressService(mockRepo, mockUserRepo)
		dto, err := svc.UpdateUserAddress(ctx, userUUID, addrUUID, v1dto.UpdateAddressInput{
			Label:      &newLabel,
			Name:       &newName,
			Phone:      &newPhone,
			Address:    &newAddr,
			ProvinceID: &provID,
			DistrictID: &distID,
		})

		require.NoError(t, err)
		assert.Equal(t, "New Villa", dto.Label)
		assert.Equal(t, "Alice", dto.Name)
		assert.Equal(t, "0911222333", dto.Phone)
		assert.Equal(t, "789 Ocean Blvd", dto.Address)
	})
}

func TestAddressService_SetDefaultUserAddress(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	addrUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewAddressService(nil, mockUserRepo)
		err := svc.SetDefaultUserAddress(ctx, userUUID, addrUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})
}

func TestAddressService_DeleteUserAddress(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	addrUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewAddressService(nil, mockUserRepo)
		err := svc.DeleteUserAddress(ctx, userUUID, addrUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})
}
