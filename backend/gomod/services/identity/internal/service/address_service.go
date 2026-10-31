package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"goshop/services/identity/internal/db/sqlc"
	dto "goshop/services/identity/internal/dto"
	"goshop/services/identity/internal/repository"
	"goshop/shared/pkg/utils"

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

func (as *addressService) CreateUserAddress(ctx context.Context, userUUID uuid.UUID, input dto.CreateAddressInput) (dto.AddressDTO, error) {
	user, err := as.getUser(ctx, userUUID)
	if err != nil {
		return dto.AddressDTO{}, err
	}

	if err := as.validateCreateInput(&input); err != nil {
		return dto.AddressDTO{}, err
	}

	txCtx, cancel := context.WithTimeout(ctx, addressTxTimeout)
	defer cancel()

	tx, err := as.repo.BeginTx(txCtx)
	if err != nil {
		return dto.AddressDTO{}, utils.NewError(utils.ErrCodeInternal, "Failed to begin transaction")
	}
	if tx != nil {
		defer func() { _ = tx.Rollback(txCtx) }()
	}

	count, err := as.repo.Count(txCtx, user.UserID)
	if err != nil {
		return dto.AddressDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to count user addresses", err)
	}

	if count >= maxAddressesPerUser {
		return dto.AddressDTO{}, utils.NewError(utils.ErrCodeBadRequest, "Maximum of 5 addresses allowed")
	}

	if input.IsDefault || count == 0 {
		if err := as.repo.ClearDefault(txCtx, tx, user.UserID); err != nil {
			return dto.AddressDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to clear default address", err)
		}
		input.IsDefault = true
	}

	params := as.buildCreateParams(user.UserID, input)
	address, err := as.repo.Create(txCtx, tx, params)
	if err != nil {
		return dto.AddressDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to create address", err)
	}

	if tx != nil {
		if err := tx.Commit(txCtx); err != nil {
			return dto.AddressDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
		}
	}

	return mapToAddressDTO(address), nil
}

func (as *addressService) GetMyAddresses(ctx context.Context, userUUID uuid.UUID) ([]dto.AddressDTO, error) {
	user, err := as.getUser(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	addresses, err := as.repo.GetByUserID(ctx, user.UserID)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to retrieve addresses", err)
	}

	if len(addresses) == 0 {
		return []dto.AddressDTO{}, utils.NewError(utils.ErrCodeNotFound, "No addresses found")
	}

	dtos := make([]dto.AddressDTO, len(addresses))
	for i, addr := range addresses {
		dtos[i] = mapToAddressDTO(addr)
	}
	return dtos, nil
}

func (as *addressService) GetAddressByUUID(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) (dto.AddressDTO, error) {
	user, err := as.getUser(ctx, userUUID)
	if err != nil {
		return dto.AddressDTO{}, err
	}

	address, err := as.repo.GetByUUID(ctx, user.UserID, addressUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return dto.AddressDTO{}, utils.NewError(utils.ErrCodeNotFound, "Address not found")
		}
		return dto.AddressDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to retrieve address", err)
	}

	return mapToAddressDTO(address), nil
}

func (as *addressService) UpdateUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID, input dto.UpdateAddressInput) (dto.AddressDTO, error) {
	user, err := as.getUser(ctx, userUUID)
	if err != nil {
		return dto.AddressDTO{}, err
	}

	params, err := as.buildUpdateParams(user.UserID, addressUUID, input)
	if err != nil {
		return dto.AddressDTO{}, err
	}

	updatedAddress, err := as.repo.Update(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return dto.AddressDTO{}, utils.NewError(utils.ErrCodeNotFound, "Address not found")
		}
		return dto.AddressDTO{}, utils.WrapError(utils.ErrCodeInternal, "Failed to update address", err)
	}

	return mapToAddressDTO(updatedAddress), nil
}

func (as *addressService) SetDefaultUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) error {
	user, err := as.getUser(ctx, userUUID)
	if err != nil {
		return err
	}

	txCtx, cancel := context.WithTimeout(ctx, addressTxTimeout)
	defer cancel()

	tx, err := as.repo.BeginTx(txCtx)
	if err != nil {
		return utils.NewError(utils.ErrCodeInternal, "Failed to begin transaction")
	}
	if tx != nil {
		defer func() { _ = tx.Rollback(txCtx) }()
	}

	if err := as.repo.ClearDefault(txCtx, tx, user.UserID); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to clear default address", err)
	}

	if _, err := as.repo.SetDefault(txCtx, tx, user.UserID, addressUUID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Address not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to set default address", err)
	}

	if tx != nil {
		if err := tx.Commit(txCtx); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
		}
	}

	return nil
}

func (as *addressService) DeleteUserAddress(ctx context.Context, userUUID uuid.UUID, addressUUID uuid.UUID) error {
	user, err := as.getUser(ctx, userUUID)
	if err != nil {
		return err
	}

	current, err := as.repo.GetByUUID(ctx, user.UserID, addressUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return utils.NewError(utils.ErrCodeNotFound, "Address not found")
		}
		return utils.WrapError(utils.ErrCodeInternal, "Failed to retrieve address", err)
	}

	txCtx, cancel := context.WithTimeout(ctx, addressTxTimeout)
	defer cancel()

	tx, err := as.repo.BeginTx(txCtx)
	if err != nil {
		return utils.NewError(utils.ErrCodeInternal, "Failed to begin transaction")
	}
	if tx != nil {
		defer func() { _ = tx.Rollback(txCtx) }()
	}

	deleted, err := as.repo.Delete(txCtx, tx, user.UserID, addressUUID)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to delete address", err)
	}
	if !deleted {
		return utils.NewError(utils.ErrCodeNotFound, "Address not found")
	}

	if current.UaIsDefault {
		if err := as.repo.SetLatestAsDefault(txCtx, tx, user.UserID); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to set new default address", err)
		}
	}

	if tx != nil {
		if err := tx.Commit(txCtx); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to commit transaction", err)
		}
	}

	return nil
}

// ── Helper Methods ────────────────────────────────────────────────

func (as *addressService) getUser(ctx context.Context, userUUID uuid.UUID) (sqlc.User, error) {
	user, err := as.userRepo.GetByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows) {
			return sqlc.User{}, utils.NewError(utils.ErrCodeNotFound, "User not found")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "Failed to retrieve user", err)
	}
	return user, nil
}

func (as *addressService) validateCreateInput(input *dto.CreateAddressInput) error {
	input.Label = strings.TrimSpace(input.Label)
	input.Name = strings.TrimSpace(input.Name)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Address = strings.TrimSpace(input.Address)

	if input.Label == "" || input.Name == "" || input.Phone == "" || input.Address == "" {
		return utils.NewError(utils.ErrCodeBadRequest, "Address fields cannot be blank")
	}
	return nil
}

func (as *addressService) buildCreateParams(userID int32, input dto.CreateAddressInput) sqlc.CreateUserAddressParams {
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

func (as *addressService) buildUpdateParams(userID int32, addressUUID uuid.UUID, input dto.UpdateAddressInput) (sqlc.UpdateUserAddressParams, error) {
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

func mapToAddressDTO(a sqlc.UserAddress) dto.AddressDTO {
	fullAddress := a.UaAddress
	if a.UaWardName != nil && a.UaDistrictName != nil && a.UaProvinceName != nil {
		fullAddress = fmt.Sprintf("%s, %s, %s, %s", a.UaAddress, *a.UaWardName, *a.UaDistrictName, *a.UaProvinceName)
	}

	return dto.AddressDTO{
		UUID:         a.UaUuid.String(),
		Label:        a.UaLabel,
		Name:         a.UaName,
		Phone:        a.UaPhone,
		Address:      a.UaAddress,
		ProvinceID:   utils.Ptr32ToPtrInt(a.UaProvinceID),
		ProvinceName: utils.NilToString(a.UaProvinceName),
		DistrictID:   utils.Ptr32ToPtrInt(a.UaDistrictID),
		DistrictName: utils.NilToString(a.UaDistrictName),
		WardCode:     utils.NilToString(a.UaWardCode),
		WardName:     utils.NilToString(a.UaWardName),
		FullAddress:  fullAddress,
		IsDefault:    a.UaIsDefault,
	}
}
