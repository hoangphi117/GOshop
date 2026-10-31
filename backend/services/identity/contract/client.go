package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// IdentityClient defines the public inter-service interface to the Identity microservice
type IdentityClient interface {
	GetUserByID(ctx context.Context, userID int32) (*InternalUserDTO, error)
	GetUserByUUID(ctx context.Context, userUUID uuid.UUID) (*InternalUserDTO, error)
	GetActiveAdminUserIDs(ctx context.Context) ([]int32, error)
	GetUsersBatch(ctx context.Context, userIDs []int32) (map[int32]InternalUserDTO, error)
	GetAddressByUUID(ctx context.Context, userID int32, addressUUID uuid.UUID) (*AddressDTO, error)
	GetAddressesByUserID(ctx context.Context, userID int32) ([]AddressDTO, error)
}

// HTTPIdentityClient implements IdentityClient over lightweight HTTP REST with connection pooling and Redis caching
type HTTPIdentityClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
	redis      *redis.Client
	cacheTTL   time.Duration
}

// ClientOption configures HTTPIdentityClient
type ClientOption func(*HTTPIdentityClient)

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *HTTPIdentityClient) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithCacheTTL sets custom cache duration
func WithCacheTTL(ttl time.Duration) ClientOption {
	return func(c *HTTPIdentityClient) {
		if ttl > 0 {
			c.cacheTTL = ttl
		}
	}
}

// NewHTTPIdentityClient creates a new IdentityClient
func NewHTTPIdentityClient(baseURL string, secret string, redisClient *redis.Client, opts ...ClientOption) IdentityClient {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}

	c := &HTTPIdentityClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		secret:   secret,
		redis:    redisClient,
		cacheTTL: 1 * time.Hour,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// GetUserByID retrieves a user by ID, checking Redis first before falling back to internal REST
func (c *HTTPIdentityClient) GetUserByID(ctx context.Context, userID int32) (*InternalUserDTO, error) {
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

	// 2. Cache miss -> Call internal REST endpoint
	url := fmt.Sprintf("%s/internal/v1/users/%d", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("X-Internal-Token", c.secret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call identity service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found: %d", userID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity service returned status: %d", resp.StatusCode)
	}

	var user InternalUserDTO
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
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

	return &user, nil
}

// GetUserByUUID retrieves a user by UUID, checking Redis first before falling back to internal REST
func (c *HTTPIdentityClient) GetUserByUUID(ctx context.Context, userUUID uuid.UUID) (*InternalUserDTO, error) {
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

	// 2. Cache miss -> Call internal REST endpoint
	url := fmt.Sprintf("%s/internal/v1/users/uuid/%s", c.baseURL, userUUID.String())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("X-Internal-Token", c.secret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call identity service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found: %s", userUUID.String())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity service returned status: %d", resp.StatusCode)
	}

	var user InternalUserDTO
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 3. Write back to Redis
	if c.redis != nil {
		if data, err := json.Marshal(user); err == nil {
			_ = c.redis.Set(ctx, cacheKey, data, c.cacheTTL).Err()
			_ = c.redis.Set(ctx, fmt.Sprintf("user:profile:%d", user.UserID), data, c.cacheTTL).Err()
		}
	}

	return &user, nil
}

// GetActiveAdminUserIDs retrieves all active admin user IDs from Identity Service
func (c *HTTPIdentityClient) GetActiveAdminUserIDs(ctx context.Context) ([]int32, error) {
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

	// 2. Cache miss -> Call internal REST endpoint
	url := fmt.Sprintf("%s/internal/v1/users/admins", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("X-Internal-Token", c.secret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call identity service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity service returned status: %d", resp.StatusCode)
	}

	var res struct {
		AdminIDs []int32 `json:"admin_ids"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode admin ids: %w", err)
	}

	// 3. Write back to Redis (TTL 10m)
	if c.redis != nil {
		if data, err := json.Marshal(res.AdminIDs); err == nil {
			_ = c.redis.Set(ctx, cacheKey, data, 10*time.Minute).Err()
		}
	}

	return res.AdminIDs, nil
}

// GetUsersBatch fetches multiple users by IDs, checking Redis first and only fetching missing ones via REST
func (c *HTTPIdentityClient) GetUsersBatch(ctx context.Context, userIDs []int32) (map[int32]InternalUserDTO, error) {
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

	// 2. Fetch missing IDs from internal batch endpoint
	url := fmt.Sprintf("%s/internal/v1/users/batch", c.baseURL)
	payload := map[string][]int32{"user_ids": missingIDs}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("X-Internal-Token", c.secret)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call identity service batch endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity service returned status %d on batch request", resp.StatusCode)
	}

	var batchResp InternalBatchUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("failed to decode batch response: %w", err)
	}

	// 3. Cache fetched users into Redis and populate results
	for id, u := range batchResp.Users {
		results[id] = u
		if c.redis != nil {
			if data, err := json.Marshal(u); err == nil {
				_ = c.redis.Set(ctx, fmt.Sprintf("user:profile:%d", id), data, c.cacheTTL).Err()
				if u.UserUUID != uuid.Nil {
					_ = c.redis.Set(ctx, fmt.Sprintf("user:profile:uuid:%s", u.UserUUID.String()), data, c.cacheTTL).Err()
				}
			}
		}
	}

	return results, nil
}

// GetAddressByUUID retrieves an address by UUID from Identity Service
func (c *HTTPIdentityClient) GetAddressByUUID(ctx context.Context, userID int32, addressUUID uuid.UUID) (*AddressDTO, error) {
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

	// 2. Cache miss -> Call internal REST endpoint
	url := fmt.Sprintf("%s/internal/v1/addresses/%s?user_id=%d", c.baseURL, addressUUID.String(), userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("X-Internal-Token", c.secret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call identity service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("address not found: %s", addressUUID.String())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity service returned status: %d", resp.StatusCode)
	}

	var addr AddressDTO
	if err := json.NewDecoder(resp.Body).Decode(&addr); err != nil {
		return nil, fmt.Errorf("failed to decode address response: %w", err)
	}

	// 3. Write back to Redis (TTL 30m)
	if c.redis != nil {
		if data, err := json.Marshal(addr); err == nil {
			_ = c.redis.Set(ctx, cacheKey, data, 30*time.Minute).Err()
		}
	}

	return &addr, nil
}

// GetAddressesByUserID retrieves all addresses of a user from Identity Service
func (c *HTTPIdentityClient) GetAddressesByUserID(ctx context.Context, userID int32) ([]AddressDTO, error) {
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

	// 2. Cache miss -> Call internal REST endpoint
	url := fmt.Sprintf("%s/internal/v1/users/%d/addresses", c.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("X-Internal-Token", c.secret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call identity service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity service returned status: %d", resp.StatusCode)
	}

	var res struct {
		Addresses []AddressDTO `json:"addresses"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode addresses response: %w", err)
	}

	// 3. Write back to Redis (TTL 15m)
	if c.redis != nil {
		if data, err := json.Marshal(res.Addresses); err == nil {
			_ = c.redis.Set(ctx, cacheKey, data, 15*time.Minute).Err()
		}
	}

	return res.Addresses, nil
}
