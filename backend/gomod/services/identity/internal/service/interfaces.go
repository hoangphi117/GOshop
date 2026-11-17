package service

import (
	"context"
	dto "goshop/services/identity/internal/dto"

	"github.com/google/uuid"
)

type UserService interface {
	GetAllUsers(ctx context.Context, search, orderBy, sort string, page, limit int32, deleted bool) ([]dto.UserDTO, int32, error)
	CreateUsers(ctx context.Context, input dto.CreateUserInput) (*dto.UserDTO, error)
	GetUserByUUID(ctx context.Context, uuid uuid.UUID) (*dto.UserDTO, error)
	UpdateUser(ctx context.Context, uuid uuid.UUID, input dto.UpdateUserInput) (*dto.UserDTO, error)
	GetProfile(ctx context.Context, uuid uuid.UUID) (*dto.UserDTO, error)
	UpdateProfile(ctx context.Context, uuid uuid.UUID, input dto.UpdateProfileInput) (*dto.UserDTO, error)
	UpdateAvatar(ctx context.Context, uuid uuid.UUID, fileKey string) (*dto.UserDTO, error)
	ChangePassword(ctx context.Context, uuid uuid.UUID, input dto.ChangePasswordRequest) error
	SoftDeleteUser(ctx context.Context, uuid uuid.UUID) (*dto.UserDTO, error)
	RestoreUser(ctx context.Context, uuid uuid.UUID) (*dto.UserDTO, error)
	TrashUser(ctx context.Context, uuid uuid.UUID) error
}

type AuthService interface {
	Register(ctx context.Context, input dto.RegisterInput) error
	SendRegisterOTP(ctx context.Context, input dto.SendRegisterOTPInput) error
	VerifyRegisterOTP(ctx context.Context, input dto.VerifyRegisterOTPInput) (string, string, int, error)
	ResendRegisterOTP(ctx context.Context, input dto.ResendRegisterOTPInput) error
	Login(ctx context.Context, ip, userAgent string, email, password string) (string, string, int, error)
	GoogleLogin(ctx context.Context, ip, userAgent, idToken string) (string, string, int, error)
	Logout(ctx context.Context, accessToken string, refreshToken string) error
	RefreshToken(ctx context.Context, refreshToken string) (string, string, int, error)
	RequestForgotPassword(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
}

type AddressService interface {
	CreateUserAddress(ctx context.Context, userUUID uuid.UUID, input dto.CreateAddressInput) (dto.AddressDTO, error)
	GetMyAddresses(ctx context.Context, userUUID uuid.UUID) ([]dto.AddressDTO, error)
	GetAddressByUUID(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) (dto.AddressDTO, error)
	UpdateUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID, input dto.UpdateAddressInput) (dto.AddressDTO, error)
	SetDefaultUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) error
	DeleteUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) error
}
