package v1service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"goshop/internal/db"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	maxAddressesPerUser = 5
	addressTxTimeout    = 5 * time.Second
)

type addressService struct {
	repo     repository.AddressRepository
	userRepo repository.UserReader
}

// NewAddressService creates a new instance of AddressService.
func NewAddressService(repo repository.AddressRepository, userRepo repository.UserReader) AddressService {
	return &addressService{
		repo:     repo,
		userRepo: userRepo,
	}
}

// ── Service Implementation ────────────────────────────────────────

func (as *addressService) CreateUserAddress(ctx context.Context, userUUID uuid.UUID, input v1dto.CreateAddressInput) (v1dto.AddressDTO, error) {
	user, err := as.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.AddressDTO{}, err
	}

	if err := as.validateCreateInput(&input); err != nil {
		return v1dto.AddressDTO{}, err
	}

	txCtx, cancel := context.WithTimeout(ctx, addressTxTimeout)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to begin transaction")
	}
	defer tx.Rollback(txCtx)

	count, err := as.repo.Count(txCtx, user.UserID)
	if err != nil {
		return v1dto.AddressDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to count user addresses", err)
	}

	if count >= maxAddressesPerUser {
		return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Maximum of 5 addresses allowed")
	}

	if input.IsDefault || count == 0 {
		if err := as.repo.ClearDefault(txCtx, tx, user.UserID); err != nil {
			return v1dto.AddressDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to clear default address", err)
		}
		input.IsDefault = true
	}

	params := as.buildCreateParams(user.UserID, input)
	address, err := as.repo.Create(txCtx, tx, params)
	if err != nil {
		return v1dto.AddressDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to create address", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return v1dto.AddressDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	return v1dto.MapToAddressDTO(address), nil
}

func (as *addressService) GetMyAddresses(ctx context.Context, userUUID uuid.UUID) ([]v1dto.AddressDTO, error) {
	user, err := as.getUser(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	addresses, err := as.repo.GetByUserID(ctx, user.UserID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to retrieve addresses", err)
	}

	if len(addresses) == 0 {
		return []v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeNotFound, "No addresses found")
	}

	return v1dto.MapToAddressDTOs(addresses), nil
}

func (as *addressService) GetAddressByUUID(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) (v1dto.AddressDTO, error) {
	user, err := as.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.AddressDTO{}, err
	}

	address, err := as.repo.GetByUUID(ctx, user.UserID, addressUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeNotFound, "Address not found")
		}
		return v1dto.AddressDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to retrieve address", err)
	}

	return v1dto.MapToAddressDTO(address), nil
}

func (as *addressService) UpdateUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID, input v1dto.UpdateAddressInput) (v1dto.AddressDTO, error) {
	user, err := as.getUser(ctx, userUUID)
	if err != nil {
		return v1dto.AddressDTO{}, err
	}

	params, err := as.buildUpdateParams(user.UserID, addressUUID, input)
	if err != nil {
		return v1dto.AddressDTO{}, err
	}

	updatedAddress, err := as.repo.Update(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return v1dto.AddressDTO{}, utils.NewError(utils.ErrCodeNotFound, "Address not found")
		}
		return v1dto.AddressDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update address", err)
	}

	return v1dto.MapToAddressDTO(updatedAddress), nil
}

func (as *addressService) SetDefaultUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) error {
	user, err := as.getUser(ctx, userUUID)
	if err != nil {
		return err
	}

	txCtx, cancel := context.WithTimeout(ctx, addressTxTimeout)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return utils.NewError(utils.ErrCodeInternal, "Failed to begin transaction")
	}
	defer tx.Rollback(txCtx)

	if err := as.repo.ClearDefault(txCtx, tx, user.UserID); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to clear default address", err)
	}

	if _, err = as.repo.SetDefault(txCtx, tx, user.UserID, addressUUID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Address not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to set default address", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	return nil
}

func (as *addressService) DeleteUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) error {
	user, err := as.getUser(ctx, userUUID)
	if err != nil {
		return err
	}

	txCtx, cancel := context.WithTimeout(ctx, addressTxTimeout)
	defer cancel()

	tx, err := db.BeginTx(txCtx)
	if err != nil {
		return utils.NewError(utils.ErrCodeInternal, "Failed to begin transaction")
	}
	defer tx.Rollback(txCtx)

	wasDefault, err := as.repo.Delete(txCtx, tx, user.UserID, addressUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Address not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to delete address", err)
	}

	if wasDefault {
		if err := as.repo.SetLatestAsDefault(txCtx, tx, user.UserID); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to set latest address as default", err)
		}
	}

	if err := tx.Commit(txCtx); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
	}

	return nil
}

// ── Private SRP Helper Methods ────────────────────────────────────

func (as *addressService) getUser(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error) {
	user, err := as.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "User not found")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "Failed to get user", err)
	}
	return user, nil
}

func (as *addressService) validateCreateInput(input *v1dto.CreateAddressInput) error {
	input.Label = strings.TrimSpace(input.Label)
	input.Name = strings.TrimSpace(input.Name)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Address = strings.TrimSpace(input.Address)

	if input.Label == "" || input.Name == "" || input.Phone == "" || input.Address == "" {
		return utils.NewError(utils.ErrCodeBadRequest, "Address fields cannot be blank")
	}
	return nil
}

func (as *addressService) buildCreateParams(userID int32, input v1dto.CreateAddressInput) sqlc.CreateUserAddressParams {
	var provID, distID *int32
	if input.ProvinceID > 0 {
		p := int32(input.ProvinceID)
		provID = &p
	}
	if input.DistrictID > 0 {
		d := int32(input.DistrictID)
		distID = &d
	}

	var provName, distName, wardCode, wardName *string
	if s := strings.TrimSpace(input.ProvinceName); s != "" {
		provName = &s
	}
	if s := strings.TrimSpace(input.DistrictName); s != "" {
		distName = &s
	}
	if s := strings.TrimSpace(input.WardCode); s != "" {
		wardCode = &s
	}
	if s := strings.TrimSpace(input.WardName); s != "" {
		wardName = &s
	}

	return sqlc.CreateUserAddressParams{
		UaUserID:       userID,
		UaLabel:        input.Label,
		UaName:         input.Name,
		UaPhone:        input.Phone,
		UaAddress:      input.Address,
		UaProvinceID:   provID,
		UaProvinceName: provName,
		UaDistrictID:   distID,
		UaDistrictName: distName,
		UaWardCode:     wardCode,
		UaWardName:     wardName,
		UaIsDefault:    input.IsDefault,
	}
}

func (as *addressService) buildUpdateParams(userID int32, addressUUID uuid.UUID, input v1dto.UpdateAddressInput) (sqlc.UpdateUserAddressParams, error) {
	var label, name, phone, addressText *string

	if input.Label != nil {
		t := strings.TrimSpace(*input.Label)
		if t == "" {
			return sqlc.UpdateUserAddressParams{}, utils.NewError(utils.ErrCodeBadRequest, "Label cannot be blank")
		}
		label = &t
	}
	if input.Name != nil {
		t := strings.TrimSpace(*input.Name)
		if t == "" {
			return sqlc.UpdateUserAddressParams{}, utils.NewError(utils.ErrCodeBadRequest, "Name cannot be blank")
		}
		name = &t
	}
	if input.Phone != nil {
		t := strings.TrimSpace(*input.Phone)
		if t == "" {
			return sqlc.UpdateUserAddressParams{}, utils.NewError(utils.ErrCodeBadRequest, "Phone cannot be blank")
		}
		phone = &t
	}
	if input.Address != nil {
		t := strings.TrimSpace(*input.Address)
		if t == "" {
			return sqlc.UpdateUserAddressParams{}, utils.NewError(utils.ErrCodeBadRequest, "Address cannot be blank")
		}
		addressText = &t
	}

	var provID, distID *int32
	if input.ProvinceID != nil {
		p := int32(*input.ProvinceID)
		provID = &p
	}
	if input.DistrictID != nil {
		d := int32(*input.DistrictID)
		distID = &d
	}

	return sqlc.UpdateUserAddressParams{
		UaUuid:       addressUUID,
		UaUserID:     userID,
		Label:        label,
		Name:         name,
		Phone:        phone,
		Address:      addressText,
		ProvinceID:   provID,
		ProvinceName: input.ProvinceName,
		DistrictID:   distID,
		DistrictName: input.DistrictName,
		WardCode:     input.WardCode,
		WardName:     input.WardName,
	}, nil
}
