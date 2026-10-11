package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	v1dto "goshop/internal/dto/v1"
	"goshop/pkg/logger"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// IdentityClient defines inter-service calls to the Identity microservice
type IdentityClient interface {
	GetUserByID(ctx context.Context, userID int32) (*v1dto.InternalUserDTO, error)
	GetUserByUUID(ctx context.Context, userUUID uuid.UUID) (*v1dto.InternalUserDTO, error)
	GetActiveAdminUserIDs(ctx context.Context) ([]int32, error)
	GetUsersBatch(ctx context.Context, userIDs []int32) (map[int32]v1dto.InternalUserDTO, error)
	GetAddressByUUID(ctx context.Context, userID int32, addressUUID uuid.UUID) (*v1dto.AddressDTO, error)
	GetAddressesByUserID(ctx context.Context, userID int32) ([]v1dto.AddressDTO, error)
}

// HTTPIdentityClient implements IdentityClient over lightweight HTTP REST with connection pooling and Redis caching
type HTTPIdentityClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
	redis      *redis.Client
	cacheTTL   time.Duration
}

// NewHTTPIdentityClient creates a new client with persistent connection keep-alive
func NewHTTPIdentityClient(baseURL, secret string, redisClient *redis.Client) *HTTPIdentityClient {
	cleanBaseURL := strings.TrimRight(baseURL, "/")
	if cleanBaseURL == "" {
		cleanBaseURL = "http://localhost:8081"
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}

	return &HTTPIdentityClient{
		baseURL: cleanBaseURL,
		secret:  secret,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   5 * time.Second,
		},
		redis:    redisClient,
		cacheTTL: 1 * time.Hour,
	}
}

// SetHTTPClient allows overriding the internal http.Client (e.g. for testing)
func (c *HTTPIdentityClient) SetHTTPClient(client *http.Client) {
	c.httpClient = client
}

// GetUserByID retrieves a user by ID, checking Redis first before falling back to internal REST
func (c *HTTPIdentityClient) GetUserByID(ctx context.Context, userID int32) (*v1dto.InternalUserDTO, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id: %d", userID)
	}

	cacheKey := fmt.Sprintf("user:profile:%d", userID)

	// 1. Try reading from Redis cache first (Zero-Network inter-service latency)
	if c.redis != nil {
		if val, err := c.redis.Get(ctx, cacheKey).Result(); err == nil && val != "" {
			var user v1dto.InternalUserDTO
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
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found: %d", userID)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity service returned status: %d", resp.StatusCode)
	}

	var user v1dto.InternalUserDTO
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 3. Write back to Redis
	if c.redis != nil {
		if data, err := json.Marshal(user); err != nil {
			logger.Log.Warn().Err(err).Int32("user_id", user.UserID).Msg("Failed to marshal user dto for client caching")
		} else if err := c.redis.Set(ctx, cacheKey, data, c.cacheTTL).Err(); err != nil {
			logger.Log.Warn().Err(err).Int32("user_id", user.UserID).Msg("Failed to write user profile to client cache")
		}
	}

	return &user, nil
}

// GetUsersBatch fetches a batch of users by IDs, leveraging Redis cache where available
func (c *HTTPIdentityClient) GetUsersBatch(ctx context.Context, userIDs []int32) (map[int32]v1dto.InternalUserDTO, error) {
	if len(userIDs) == 0 {
		return make(map[int32]v1dto.InternalUserDTO), nil
	}

	// Deduplicate IDs
	uniqueMap := make(map[int32]struct{}, len(userIDs))
	var uniqueIDs []int32
	for _, id := range userIDs {
		if id > 0 {
			if _, exists := uniqueMap[id]; !exists {
				uniqueMap[id] = struct{}{}
				uniqueIDs = append(uniqueIDs, id)
			}
		}
	}

	if len(uniqueIDs) == 0 {
		return make(map[int32]v1dto.InternalUserDTO), nil
	}

	results := make(map[int32]v1dto.InternalUserDTO, len(uniqueIDs))
	var missingIDs []int32

	// 1. Try reading each from Redis cache
	for _, id := range uniqueIDs {
		cacheKey := fmt.Sprintf("user:profile:%d", id)
		hit := false
		if c.redis != nil {
			if val, err := c.redis.Get(ctx, cacheKey).Result(); err == nil && val != "" {
				var u v1dto.InternalUserDTO
				if err := json.Unmarshal([]byte(val), &u); err == nil {
					results[id] = u
					hit = true
				}
			}
		}
		if !hit {
			missingIDs = append(missingIDs, id)
		}
	}

	// If all were cached, return immediately
	if len(missingIDs) == 0 {
		return results, nil
	}

	// 2. Fetch missing users from Identity Service
	url := fmt.Sprintf("%s/internal/v1/users/batch", c.baseURL)
	payload, err := json.Marshal(v1dto.InternalBatchUsersRequest{UserIDs: missingIDs})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal batch request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", c.secret)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call identity batch endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity batch endpoint returned status: %d", resp.StatusCode)
	}

	var batchResp v1dto.InternalBatchUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("failed to decode batch response: %w", err)
	}

	// 3. Cache fetched users into Redis and populate results
	for id, u := range batchResp.Users {
		results[id] = u
		if c.redis != nil {
			if data, err := json.Marshal(u); err != nil {
				logger.Log.Warn().Err(err).Int32("user_id", id).Msg("Failed to marshal user dto for batch caching")
			} else if err := c.redis.Set(ctx, fmt.Sprintf("user:profile:%d", id), data, c.cacheTTL).Err(); err != nil {
				logger.Log.Warn().Err(err).Int32("user_id", id).Msg("Failed to write batch user profile to client cache")
			}
		}
	}

	return results, nil
}

// GetUserByUUID retrieves a user by UUID, checking Redis first before falling back to internal REST
func (c *HTTPIdentityClient) GetUserByUUID(ctx context.Context, userUUID uuid.UUID) (*v1dto.InternalUserDTO, error) {
	if userUUID == uuid.Nil {
		return nil, fmt.Errorf("invalid user uuid")
	}

	cacheKey := fmt.Sprintf("user:profile:uuid:%s", userUUID.String())

	// 1. Try reading from Redis cache first
	if c.redis != nil {
		if val, err := c.redis.Get(ctx, cacheKey).Result(); err == nil && val != "" {
			var user v1dto.InternalUserDTO
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
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("user not found: %s", userUUID.String())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity service returned status: %d", resp.StatusCode)
	}

	var user v1dto.InternalUserDTO
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
	defer resp.Body.Close()

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

// GetAddressByUUID retrieves an address by UUID from Identity Service
func (c *HTTPIdentityClient) GetAddressByUUID(ctx context.Context, userID int32, addressUUID uuid.UUID) (*v1dto.AddressDTO, error) {
	if addressUUID == uuid.Nil {
		return nil, fmt.Errorf("invalid address uuid")
	}

	cacheKey := fmt.Sprintf("user:address:%s", addressUUID.String())

	// 1. Try reading from Redis cache first
	if c.redis != nil {
		if val, err := c.redis.Get(ctx, cacheKey).Result(); err == nil && val != "" {
			var addr v1dto.AddressDTO
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
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("address not found: %s", addressUUID.String())
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity service returned status: %d", resp.StatusCode)
	}

	var addr v1dto.AddressDTO
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
func (c *HTTPIdentityClient) GetAddressesByUserID(ctx context.Context, userID int32) ([]v1dto.AddressDTO, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("invalid user id: %d", userID)
	}

	cacheKey := fmt.Sprintf("user:addresses:%d", userID)

	// 1. Try reading from Redis cache first
	if c.redis != nil {
		if val, err := c.redis.Get(ctx, cacheKey).Result(); err == nil && val != "" {
			var dtos []v1dto.AddressDTO
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("identity service returned status: %d", resp.StatusCode)
	}

	var res struct {
		Addresses []v1dto.AddressDTO `json:"addresses"`
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
