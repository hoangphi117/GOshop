package contract

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"goshop/shared/pkg/grpcclient"
	identityv1 "goshop/shared/proto/identity/v1"
)

// GRPCIdentityClient implements IdentityClient over gRPC with local Redis caching
type GRPCIdentityClient struct {
	client   identityv1.IdentityServiceClient
	conn     *grpc.ClientConn
	redis    *redis.Client
	cacheTTL time.Duration
}

// GRPCOption allows configuring GRPCIdentityClient
type GRPCOption func(*GRPCIdentityClient)

// WithGRPCCacheTTL sets custom Redis cache duration
func WithGRPCCacheTTL(ttl time.Duration) GRPCOption {
	return func(c *GRPCIdentityClient) {
		if ttl > 0 {
			c.cacheTTL = ttl
		}
	}
}

// NewGRPCIdentityClient creates a new gRPC-backed IdentityClient
func NewGRPCIdentityClient(target string, redisClient *redis.Client, opts ...GRPCOption) (IdentityClient, error) {
	conn, err := grpcclient.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to identity grpc server at %s: %w", target, err)
	}

	c := &GRPCIdentityClient{
		client:   identityv1.NewIdentityServiceClient(conn),
		conn:     conn,
		redis:    redisClient,
		cacheTTL: 1 * time.Hour,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// Close closes the underlying gRPC connection
func (c *GRPCIdentityClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetUserByID retrieves a user by integer ID, checking Redis first
func (c *GRPCIdentityClient) GetUserByID(ctx context.Context, userID int32) (*InternalUserDTO, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id: %d", userID)
	}

	cacheKey := fmt.Sprintf("user:profile:%d", userID)

	// 1. Try reading from Redis cache first
	if c.redis != nil {
		if val, err := c.redis.Get(ctx, cacheKey).Result(); err == nil && val != "" {
			var user InternalUserDTO
			if err := json.Unmarshal([]byte(val), &user); err == nil {
				return &user, nil
			}
		}
	}

	// 2. Cache miss -> Call gRPC IdentityService
	res, err := c.client.GetUserByID(ctx, &identityv1.GetUserByIDRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("identity grpc GetUserByID failed: %w", err)
	}

	user := protoToUserDTO(res.GetUser())
	if user == nil {
		return nil, fmt.Errorf("user not found: %d", userID)
	}

	// 3. Write back to Redis
	if c.redis != nil {
		if data, err := json.Marshal(user); err == nil {
			_ = c.redis.Set(ctx, cacheKey, data, c.cacheTTL).Err()
			if user.UserUUID != uuid.Nil {
				_ = c.redis.Set(ctx, fmt.Sprintf("user:profile:uuid:%s", user.UserUUID.String()), data, c.cacheTTL).Err()
			}
		}
	}

	return user, nil
}

// GetUserByUUID retrieves a user by UUID, checking Redis first
func (c *GRPCIdentityClient) GetUserByUUID(ctx context.Context, userUUID uuid.UUID) (*InternalUserDTO, error) {
	if userUUID == uuid.Nil {
		return nil, fmt.Errorf("invalid user uuid")
	}

	cacheKey := fmt.Sprintf("user:profile:uuid:%s", userUUID.String())

	// 1. Try reading from Redis cache first
	if c.redis != nil {
		if val, err := c.redis.Get(ctx, cacheKey).Result(); err == nil && val != "" {
			var user InternalUserDTO
			if err := json.Unmarshal([]byte(val), &user); err == nil {
				return &user, nil
			}
		}
	}

	// 2. Cache miss -> Call gRPC IdentityService
	res, err := c.client.GetUserByUUID(ctx, &identityv1.GetUserByUUIDRequest{
		UserUuid: userUUID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("identity grpc GetUserByUUID failed: %w", err)
	}

	user := protoToUserDTO(res.GetUser())
	if user == nil {
		return nil, fmt.Errorf("user not found: %s", userUUID.String())
	}

	// 3. Write back to Redis
	if c.redis != nil {
		if data, err := json.Marshal(user); err == nil {
			_ = c.redis.Set(ctx, cacheKey, data, c.cacheTTL).Err()
			_ = c.redis.Set(ctx, fmt.Sprintf("user:profile:%d", user.UserID), data, c.cacheTTL).Err()
		}
	}

	return user, nil
}

// GetActiveAdminUserIDs retrieves all active admin user IDs from Identity Service
func (c *GRPCIdentityClient) GetActiveAdminUserIDs(ctx context.Context) ([]int32, error) {
	cacheKey := "user:admins:active"

	// 1. Try reading from Redis cache first
	if c.redis != nil {
		if val, err := c.redis.Get(ctx, cacheKey).Result(); err == nil && val != "" {
			var adminIDs []int32
			if err := json.Unmarshal([]byte(val), &adminIDs); err == nil {
				return adminIDs, nil
			}
		}
	}

	// 2. Cache miss -> Call gRPC IdentityService
	res, err := c.client.GetActiveAdminUserIDs(ctx, &identityv1.GetActiveAdminUserIDsRequest{})
	if err != nil {
		return nil, fmt.Errorf("identity grpc GetActiveAdminUserIDs failed: %w", err)
	}

	adminIDs := res.GetAdminUserIds()

	// 3. Write back to Redis (TTL 10m)
	if c.redis != nil {
		if data, err := json.Marshal(adminIDs); err == nil {
			_ = c.redis.Set(ctx, cacheKey, data, 10*time.Minute).Err()
		}
	}

	return adminIDs, nil
}

// GetUsersBatch fetches multiple users by IDs, checking Redis first and batch querying missing ones via gRPC
func (c *GRPCIdentityClient) GetUsersBatch(ctx context.Context, userIDs []int32) (map[int32]InternalUserDTO, error) {
	if len(userIDs) == 0 {
		return make(map[int32]InternalUserDTO), nil
	}

	results := make(map[int32]InternalUserDTO, len(userIDs))
	var missingIDs []int32

	// 1. Check Redis MGet
	if c.redis != nil {
		keys := make([]string, len(userIDs))
		for i, id := range userIDs {
			keys[i] = fmt.Sprintf("user:profile:%d", id)
		}

		vals, err := c.redis.MGet(ctx, keys...).Result()
		if err == nil {
			for i, val := range vals {
				id := userIDs[i]
				if str, ok := val.(string); ok && str != "" {
					var u InternalUserDTO
					if err := json.Unmarshal([]byte(str), &u); err == nil {
						results[id] = u
						continue
					}
				}
				missingIDs = append(missingIDs, id)
			}
		} else {
			missingIDs = userIDs
		}
	} else {
		missingIDs = userIDs
	}

	if len(missingIDs) == 0 {
		return results, nil
	}

	// 2. Fetch missing IDs from gRPC batch endpoint
	res, err := c.client.GetBatchUsers(ctx, &identityv1.GetBatchUsersRequest{
		UserIds: missingIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("identity grpc GetBatchUsers failed: %w", err)
	}

	// 3. Populate results and cache into Redis
	for id, protoUser := range res.GetUsers() {
		user := protoToUserDTO(protoUser)
		if user == nil {
			continue
		}
		results[id] = *user
		if c.redis != nil {
			if data, err := json.Marshal(*user); err == nil {
				_ = c.redis.Set(ctx, fmt.Sprintf("user:profile:%d", id), data, c.cacheTTL).Err()
				if user.UserUUID != uuid.Nil {
					_ = c.redis.Set(ctx, fmt.Sprintf("user:profile:uuid:%s", user.UserUUID.String()), data, c.cacheTTL).Err()
				}
			}
		}
	}

	return results, nil
}

// GetAddressByUUID retrieves an address by UUID from Identity Service
func (c *GRPCIdentityClient) GetAddressByUUID(ctx context.Context, userID int32, addressUUID uuid.UUID) (*AddressDTO, error) {
	if addressUUID == uuid.Nil {
		return nil, fmt.Errorf("invalid address uuid")
	}

	cacheKey := fmt.Sprintf("user:address:%s", addressUUID.String())

	// 1. Try reading from Redis cache first
	if c.redis != nil {
		if val, err := c.redis.Get(ctx, cacheKey).Result(); err == nil && val != "" {
			var addr AddressDTO
			if err := json.Unmarshal([]byte(val), &addr); err == nil {
				return &addr, nil
			}
		}
	}

	// 2. Cache miss -> Call gRPC IdentityService
	res, err := c.client.GetAddressByUUID(ctx, &identityv1.GetAddressByUUIDRequest{
		UserId:      userID,
		AddressUuid: addressUUID.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("identity grpc GetAddressByUUID failed: %w", err)
	}

	addr := protoToAddressDTO(res.GetAddress())
	if addr == nil {
		return nil, fmt.Errorf("address not found: %s", addressUUID.String())
	}

	// 3. Write back to Redis (TTL 30m)
	if c.redis != nil {
		if data, err := json.Marshal(addr); err == nil {
			_ = c.redis.Set(ctx, cacheKey, data, 30*time.Minute).Err()
		}
	}

	return addr, nil
}

// GetAddressesByUserID retrieves all addresses of a user from Identity Service
func (c *GRPCIdentityClient) GetAddressesByUserID(ctx context.Context, userID int32) ([]AddressDTO, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id: %d", userID)
	}

	cacheKey := fmt.Sprintf("user:addresses:%d", userID)

	// 1. Try reading from Redis cache first
	if c.redis != nil {
		if val, err := c.redis.Get(ctx, cacheKey).Result(); err == nil && val != "" {
			var dtos []AddressDTO
			if err := json.Unmarshal([]byte(val), &dtos); err == nil {
				return dtos, nil
			}
		}
	}

	// 2. Cache miss -> Call gRPC IdentityService
	res, err := c.client.GetAddressesByUserID(ctx, &identityv1.GetAddressesByUserIDRequest{
		UserId: userID,
	})
	if err != nil {
		return nil, fmt.Errorf("identity grpc GetAddressesByUserID failed: %w", err)
	}

	protoAddresses := res.GetAddresses()
	dtos := make([]AddressDTO, len(protoAddresses))
	for i, a := range protoAddresses {
		addr := protoToAddressDTO(a)
		if addr != nil {
			dtos[i] = *addr
		}
	}

	// 3. Write back to Redis (TTL 15m)
	if c.redis != nil {
		if data, err := json.Marshal(dtos); err == nil {
			_ = c.redis.Set(ctx, cacheKey, data, 15*time.Minute).Err()
		}
	}

	return dtos, nil
}

func protoToUserDTO(u *identityv1.UserDTO) *InternalUserDTO {
	if u == nil {
		return nil
	}
	parsedUUID, _ := uuid.Parse(u.GetUserUuid())
	dto := &InternalUserDTO{
		UserID:       u.GetUserId(),
		UserUUID:     parsedUUID,
		UserFullname: u.GetUserFullname(),
		UserEmail:    u.GetUserEmail(),
		UserAvatar:   u.GetUserAvatar(),
		UserStatus:   u.GetUserStatus(),
		UserLevel:    u.GetUserLevel(),
	}
	if u.UserAvatarUrl != nil {
		dto.UserAvatarUrl = u.UserAvatarUrl
	}
	return dto
}

func protoToAddressDTO(a *identityv1.AddressDTO) *AddressDTO {
	if a == nil {
		return nil
	}
	dto := &AddressDTO{
		UUID:         a.GetUuid(),
		Label:        a.GetLabel(),
		Name:         a.GetName(),
		Phone:        a.GetPhone(),
		Address:      a.GetAddress(),
		ProvinceID:   a.ProvinceId,
		ProvinceName: a.GetProvinceName(),
		DistrictID:   a.DistrictId,
		DistrictName: a.GetDistrictName(),
		WardCode:     a.WardCode,
		WardName:     a.GetWardName(),
		FullAddress:  a.GetFullAddress(),
		IsDefault:    a.GetIsDefault(),
		AddressType:  a.GetAddressType(),
	}
	if a.DeliveryNote != nil {
		dto.DeliveryNote = a.DeliveryNote
	}
	return dto
}

// CountCustomersCreatedBetween counts registered customers (user_level 3, not
// soft-deleted) created inside [fromTime, toTime]. Analytics uses it for the
// new-customer KPI without touching the identity database.
func (c *GRPCIdentityClient) CountCustomersCreatedBetween(ctx context.Context, fromTime, toTime time.Time) (int64, error) {
	resp, err := c.client.CountCustomersCreatedBetween(ctx, &identityv1.CountCustomersCreatedBetweenRequest{
		FromTime: fromTime.UTC().Format(time.RFC3339),
		ToTime:   toTime.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return 0, fmt.Errorf("grpc CountCustomersCreatedBetween failed: %w", err)
	}
	return resp.CustomerCount, nil
}
