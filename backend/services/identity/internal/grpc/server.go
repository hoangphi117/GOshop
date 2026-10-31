package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "goshop/proto/identity/v1"
	"goshop/services/identity/internal/db/sqlc"
	"goshop/services/identity/internal/repository"
)

// Server implements identityv1.IdentityServiceServer
type Server struct {
	identityv1.UnimplementedIdentityServiceServer
	userRepo    repository.UserRepository
	addressRepo repository.AddressRepository
}

// NewServer creates a new Identity gRPC Server instance
func NewServer(userRepo repository.UserRepository, addressRepo repository.AddressRepository) *Server {
	return &Server{
		userRepo:    userRepo,
		addressRepo: addressRepo,
	}
}

// GetUserByID retrieves a user by integer ID
func (s *Server) GetUserByID(ctx context.Context, req *identityv1.GetUserByIDRequest) (*identityv1.GetUserByIDResponse, error) {
	if req.UserId <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user id: %d", req.UserId)
	}

	user, err := s.userRepo.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	return &identityv1.GetUserByIDResponse{
		User: toProtoUser(&user),
	}, nil
}

// GetUserByUUID retrieves a user by string UUID
func (s *Server) GetUserByUUID(ctx context.Context, req *identityv1.GetUserByUUIDRequest) (*identityv1.GetUserByUUIDResponse, error) {
	u, err := uuid.Parse(req.UserUuid)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user uuid: %v", err)
	}

	user, err := s.userRepo.GetByUUID(ctx, u)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", err)
	}

	return &identityv1.GetUserByUUIDResponse{
		User: toProtoUser(&user),
	}, nil
}

// GetBatchUsers retrieves multiple users by a slice of integer IDs
func (s *Server) GetBatchUsers(ctx context.Context, req *identityv1.GetBatchUsersRequest) (*identityv1.GetBatchUsersResponse, error) {
	if len(req.UserIds) == 0 {
		return &identityv1.GetBatchUsersResponse{
			Users: make(map[int32]*identityv1.UserDTO),
		}, nil
	}

	rows, err := s.userRepo.GetByIDs(ctx, req.UserIds)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get users: %v", err)
	}

	users := make(map[int32]*identityv1.UserDTO, len(rows))
	for _, row := range rows {
		users[row.UserID] = toProtoBatchUser(&row)
	}

	return &identityv1.GetBatchUsersResponse{
		Users: users,
	}, nil
}

// GetActiveAdminUserIDs retrieves all admin user IDs that are currently active
func (s *Server) GetActiveAdminUserIDs(ctx context.Context, _ *identityv1.GetActiveAdminUserIDsRequest) (*identityv1.GetActiveAdminUserIDsResponse, error) {
	adminIDs, err := s.userRepo.GetActiveAdminUserIDs(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get active admin user IDs: %v", err)
	}

	return &identityv1.GetActiveAdminUserIDsResponse{
		AdminUserIds: adminIDs,
	}, nil
}

// CountCustomersCreatedBetween counts new customers (user_level 3, not
// soft-deleted) registered in the given window. Consumed by the analytics
// service for new-customer stats.
func (s *Server) CountCustomersCreatedBetween(ctx context.Context, req *identityv1.CountCustomersCreatedBetweenRequest) (*identityv1.CountCustomersCreatedBetweenResponse, error) {
	fromTime, err := time.Parse(time.RFC3339, req.FromTime)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid from_time: %v", err)
	}
	toTime, err := time.Parse(time.RFC3339, req.ToTime)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid to_time: %v", err)
	}

	count, err := s.userRepo.CountCustomersCreatedBetween(ctx, fromTime, toTime)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to count customers: %v", err)
	}

	return &identityv1.CountCustomersCreatedBetweenResponse{CustomerCount: count}, nil
}

// GetAddressByUUID retrieves a specific address by user ID and address UUID
func (s *Server) GetAddressByUUID(ctx context.Context, req *identityv1.GetAddressByUUIDRequest) (*identityv1.GetAddressByUUIDResponse, error) {
	addrUUID, err := uuid.Parse(req.AddressUuid)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid address uuid: %v", err)
	}

	row, err := s.addressRepo.GetByUUID(ctx, req.UserId, addrUUID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "address not found: %v", err)
	}

	return &identityv1.GetAddressByUUIDResponse{
		Address: toProtoAddress(row),
	}, nil
}

// GetAddressesByUserID retrieves all addresses belonging to a given user
func (s *Server) GetAddressesByUserID(ctx context.Context, req *identityv1.GetAddressesByUserIDRequest) (*identityv1.GetAddressesByUserIDResponse, error) {
	if req.UserId <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "invalid user id: %d", req.UserId)
	}

	rows, err := s.addressRepo.GetByUserID(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get addresses: %v", err)
	}

	dtos := make([]*identityv1.AddressDTO, len(rows))
	for i, row := range rows {
		dtos[i] = toProtoAddress(row)
	}

	return &identityv1.GetAddressesByUserIDResponse{
		Addresses: dtos,
	}, nil
}

func toProtoUser(u *sqlc.User) *identityv1.UserDTO {
	if u == nil {
		return nil
	}
	dto := &identityv1.UserDTO{
		UserId:       u.UserID,
		UserUuid:     u.UserUuid.String(),
		UserFullname: u.UserFullname,
		UserEmail:    u.UserEmail,
		UserAvatar:   u.UserAvatar,
		UserStatus:   u.UserStatus,
		UserLevel:    u.UserLevel,
	}
	if u.UserAvatarUrl != nil {
		dto.UserAvatarUrl = u.UserAvatarUrl
	}
	return dto
}

func toProtoBatchUser(u *sqlc.GetUsersByIDsRow) *identityv1.UserDTO {
	if u == nil {
		return nil
	}
	dto := &identityv1.UserDTO{
		UserId:       u.UserID,
		UserUuid:     u.UserUuid.String(),
		UserFullname: u.UserFullname,
		UserEmail:    u.UserEmail,
		UserAvatar:   u.UserAvatar,
		UserStatus:   u.UserStatus,
		UserLevel:    u.UserLevel,
	}
	if u.UserAvatarUrl != nil {
		dto.UserAvatarUrl = u.UserAvatarUrl
	}
	return dto
}

func toProtoAddress(a sqlc.UserAddress) *identityv1.AddressDTO {
	fullAddress := a.UaAddress
	if a.UaWardName != nil && a.UaDistrictName != nil && a.UaProvinceName != nil {
		fullAddress = fmt.Sprintf("%s, %s, %s, %s", a.UaAddress, *a.UaWardName, *a.UaDistrictName, *a.UaProvinceName)
	}

	dto := &identityv1.AddressDTO{
		Uuid:        a.UaUuid.String(),
		Label:       a.UaLabel,
		Name:        a.UaName,
		Phone:       a.UaPhone,
		Address:     a.UaAddress,
		FullAddress: fullAddress,
		IsDefault:   a.UaIsDefault,
		AddressType: a.UaLabel,
	}
	if a.UaProvinceID != nil {
		pID := *a.UaProvinceID
		dto.ProvinceId = &pID
	}
	if a.UaProvinceName != nil {
		dto.ProvinceName = *a.UaProvinceName
	}
	if a.UaDistrictID != nil {
		dID := *a.UaDistrictID
		dto.DistrictId = &dID
	}
	if a.UaDistrictName != nil {
		dto.DistrictName = *a.UaDistrictName
	}
	if a.UaWardCode != nil {
		dto.WardCode = a.UaWardCode
	}
	if a.UaWardName != nil {
		dto.WardName = *a.UaWardName
	}
	return dto
}
