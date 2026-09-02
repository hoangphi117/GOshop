package v1service

import (
	"context"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type addressService struct {
	repo     repository.AddressRepository
	userRepo repository.UserRepository
}

func NewAddressService(repo repository.AddressRepository, userRepo repository.UserRepository) AddressService {
	return &addressService{
		repo:     repo,
		userRepo: userRepo,
	}
}

func (as *addressService) CreateUserAddress(ctx *gin.Context, input v1dto.CreateAddressInput) (v1dto.AddressDTO, error) {
	reqCtx := ctx.Request.Context()

	userUUIDStr, ok := ctx.Get("user_uuid")
	if !ok {
		return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Invalid user UUID")
	}

	user, err := as.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to begin transaction")
	}
	defer tx.Rollback(txCtx)

	count, err := as.repo.Count(txCtx, user.UserID)
	if err != nil {
		return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to count user addresses")
	}

	if count >= 5 {
		return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Maximum of 5 addresses allowed")
	}

	if input.IsDefault || count == 0 {
		err := as.repo.ClearDefault(txCtx, tx, user.UserID)
		if err != nil {
			return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to clear default address")
		}

		input.IsDefault = true
	}

	createAddressParams := sqlc.CreateUserAddressParams{
		UaUserID:    user.UserID,
		UaLabel:     input.Label,
		UaName:      input.Name,
		UaPhone:     input.Phone,
		UaAddress:   input.Address,
		UaIsDefault: input.IsDefault,
	}

	address, err := as.repo.Create(txCtx, tx, createAddressParams)
	if err != nil {
		return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to create address")
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to commit transaction")
	}

	addressDTO := v1dto.MapToAddressDTO(address)

	return addressDTO, nil
}
