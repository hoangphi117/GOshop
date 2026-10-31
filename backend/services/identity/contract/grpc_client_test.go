package contract_test

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	identityv1 "goshop/proto/identity/v1"
	"goshop/services/identity/contract"
)

type mockGRPCServer struct {
	identityv1.UnimplementedIdentityServiceServer
}

func (m *mockGRPCServer) GetUserByID(ctx context.Context, req *identityv1.GetUserByIDRequest) (*identityv1.GetUserByIDResponse, error) {
	uUUID := uuid.New()
	return &identityv1.GetUserByIDResponse{
		User: &identityv1.UserDTO{
			UserId:       req.UserId,
			UserUuid:     uUUID.String(),
			UserFullname: "Test User",
			UserEmail:    "test@example.com",
			UserStatus:   1,
			UserLevel:    3,
		},
	}, nil
}

func (m *mockGRPCServer) GetUserByUUID(ctx context.Context, req *identityv1.GetUserByUUIDRequest) (*identityv1.GetUserByUUIDResponse, error) {
	return &identityv1.GetUserByUUIDResponse{
		User: &identityv1.UserDTO{
			UserId:       99,
			UserUuid:     req.UserUuid,
			UserFullname: "UUID User",
			UserEmail:    "uuid@example.com",
		},
	}, nil
}

func (m *mockGRPCServer) GetBatchUsers(ctx context.Context, req *identityv1.GetBatchUsersRequest) (*identityv1.GetBatchUsersResponse, error) {
	res := make(map[int32]*identityv1.UserDTO)
	for _, id := range req.UserIds {
		res[id] = &identityv1.UserDTO{
			UserId:       id,
			UserUuid:     uuid.New().String(),
			UserFullname: "Batch User",
		}
	}
	return &identityv1.GetBatchUsersResponse{Users: res}, nil
}

func (m *mockGRPCServer) GetActiveAdminUserIDs(ctx context.Context, _ *identityv1.GetActiveAdminUserIDsRequest) (*identityv1.GetActiveAdminUserIDsResponse, error) {
	return &identityv1.GetActiveAdminUserIDsResponse{
		AdminUserIds: []int32{1, 2, 3},
	}, nil
}

func (m *mockGRPCServer) GetAddressByUUID(ctx context.Context, req *identityv1.GetAddressByUUIDRequest) (*identityv1.GetAddressByUUIDResponse, error) {
	pID := int32(79)
	return &identityv1.GetAddressByUUIDResponse{
		Address: &identityv1.AddressDTO{
			Uuid:        req.AddressUuid,
			Name:        "Nguyen Van A",
			Phone:       "0901234567",
			Address:     "123 Street",
			ProvinceId:  &pID,
			FullAddress: "123 Street, District 1, TP HCM",
			IsDefault:   true,
		},
	}, nil
}

func (m *mockGRPCServer) GetAddressesByUserID(ctx context.Context, req *identityv1.GetAddressesByUserIDRequest) (*identityv1.GetAddressesByUserIDResponse, error) {
	return &identityv1.GetAddressesByUserIDResponse{
		Addresses: []*identityv1.AddressDTO{
			{
				Uuid:        uuid.New().String(),
				Name:        "Address 1",
				FullAddress: "Full Address 1",
			},
		},
	}, nil
}

func TestGRPCIdentityClient_EndToEnd(t *testing.T) {
	buffer := 1024 * 1024
	lis := bufconn.Listen(buffer)

	baseServer := grpc.NewServer()
	identityv1.RegisterIdentityServiceServer(baseServer, &mockGRPCServer{})

	go func() {
		_ = baseServer.Serve(lis)
	}()

	t.Cleanup(func() {
		baseServer.Stop()
		_ = lis.Close()
	})

	// Start local TCP listener on random port to test NewGRPCIdentityClient directly
	tcpLis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = tcpLis.Close() }()

	tcpServer := grpc.NewServer()
	identityv1.RegisterIdentityServiceServer(tcpServer, &mockGRPCServer{})
	go func() {
		_ = tcpServer.Serve(tcpLis)
	}()
	defer tcpServer.Stop()

	client, err := contract.NewGRPCIdentityClient(tcpLis.Addr().String(), nil)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("GetUserByID", func(t *testing.T) {
		user, err := client.GetUserByID(ctx, 42)
		require.NoError(t, err)
		assert.Equal(t, int32(42), user.UserID)
		assert.Equal(t, "Test User", user.UserFullname)
		assert.Equal(t, "test@example.com", user.UserEmail)
	})

	t.Run("GetUserByUUID", func(t *testing.T) {
		testUUID := uuid.New()
		user, err := client.GetUserByUUID(ctx, testUUID)
		require.NoError(t, err)
		assert.Equal(t, int32(99), user.UserID)
		assert.Equal(t, testUUID, user.UserUUID)
	})

	t.Run("GetUsersBatch", func(t *testing.T) {
		users, err := client.GetUsersBatch(ctx, []int32{1, 2})
		require.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, "Batch User", users[1].UserFullname)
	})

	t.Run("GetActiveAdminUserIDs", func(t *testing.T) {
		ids, err := client.GetActiveAdminUserIDs(ctx)
		require.NoError(t, err)
		assert.Equal(t, []int32{1, 2, 3}, ids)
	})

	t.Run("GetAddressByUUID", func(t *testing.T) {
		addrUUID := uuid.New()
		addr, err := client.GetAddressByUUID(ctx, 10, addrUUID)
		require.NoError(t, err)
		assert.Equal(t, addrUUID.String(), addr.UUID)
		assert.Equal(t, "Nguyen Van A", addr.Name)
		assert.True(t, addr.IsDefault)
	})

	t.Run("GetAddressesByUserID", func(t *testing.T) {
		addrs, err := client.GetAddressesByUserID(ctx, 10)
		require.NoError(t, err)
		assert.Len(t, addrs, 1)
		assert.Equal(t, "Address 1", addrs[0].Name)
	})
}
