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

	"github.com/redis/go-redis/v9"
)

// IdentityClient defines inter-service calls to the Identity microservice
type IdentityClient interface {
	GetUserByID(ctx context.Context, userID int32) (*v1dto.InternalUserDTO, error)
	GetUsersBatch(ctx context.Context, userIDs []int32) (map[int32]v1dto.InternalUserDTO, error)
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
