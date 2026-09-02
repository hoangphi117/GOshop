package v1service

import (
	"context"
	"database/sql"
	"errors"
	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"strings"
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

	// Sanitize inputs
	label := strings.TrimSpace(input.Label)
	name := strings.TrimSpace(input.Name)
	phone := strings.TrimSpace(input.Phone)
	addressText := strings.TrimSpace(input.Address)

	if label == "" || name == "" || phone == "" || addressText == "" {
		return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Address fields cannot be blank")
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
		UaLabel:     label,
		UaName:      name,
		UaPhone:     phone,
		UaAddress:   addressText,
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

func (as *addressService) UpdateUserAddress(ctx *gin.Context, addressUUID uuid.UUID, input v1dto.UpdateAddressInput) (v1dto.AddressDTO, error) {
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

	// Sanitize and validate inputs if provided
	var label, name, phone, addressText *string

	if input.Label != nil {
		trimmed := strings.TrimSpace(*input.Label)
		if trimmed == "" {
			return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Label cannot be blank")
		}
		label = &trimmed
	}

	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" {
			return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Name cannot be blank")
		}
		name = &trimmed
	}

	if input.Phone != nil {
		trimmed := strings.TrimSpace(*input.Phone)
		if trimmed == "" {
			return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Phone cannot be blank")
		}
		phone = &trimmed
	}

	if input.Address != nil {
		trimmed := strings.TrimSpace(*input.Address)
		if trimmed == "" {
			return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Address cannot be blank")
		}
		addressText = &trimmed
	}

	updateParams := sqlc.UpdateUserAddressParams{
		UaUuid:   addressUUID,
		UaUserID: user.UserID,
		Label:    label,
		Name:     name,
		Phone:    phone,
		Address:  addressText,
	}

	updatedAddress, err := as.repo.Update(reqCtx, updateParams)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeNotFound, "Address not found")
		}
		return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to update address")
	}

	return v1dto.MapToAddressDTO(updatedAddress), nil
}

func (as *addressService) SetDefaultUserAddress(ctx *gin.Context, addressUUID uuid.UUID) error {
	reqCtx := ctx.Request.Context()

	userUUIDStr, ok := ctx.Get("user_uuid")
	if !ok {
		return utils.NewError(utils.ErrCodeUnauthorized, "Unauthorized")
	}

	userUUID, err := uuid.Parse(userUUIDStr.(string))
	if err != nil {
		return utils.NewError(utils.ErrCodeBadRequest, "Invalid user UUID")
	}

	user, err := as.userRepo.GetByUUID(reqCtx, userUUID)
	if err != nil {
		return utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	txCtx, cancel := context.WithTimeout(reqCtx, 5*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return utils.NewError(utils.ErrCodeInternal, "Failed to begin transaction")
	}
	defer tx.Rollback(txCtx)

	// 1. Clear previous default address (if any)
	if err := as.repo.ClearDefault(txCtx, tx, user.UserID); err != nil {
		return utils.NewError(utils.ErrCodeInternal, "Failed to clear default address")
	}

	// 2. Set target address as default & check ownership simultaneously
	_, err = as.repo.SetDefault(txCtx, tx, user.UserID, addressUUID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Address not found")
		}
		return utils.NewError(utils.ErrCodeInternal, "Failed to set default address")
	}

	// 3. Commit transaction
	if err := tx.Commit(txCtx); err != nil {
		return utils.NewError(utils.ErrCodeInternal, "Failed to commit transaction")
	}

	return nil
}
