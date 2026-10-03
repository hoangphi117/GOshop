package middleware

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"goshop/internal/utils"
	"goshop/pkg/cache"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

type Client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	mu      sync.Mutex
	clients = make(map[string]*Client)

	rateLimitLogCache = sync.Map{}

	// cleanupOnce đảm bảo các goroutine dọn dẹp nền (clients map + log cache)
	// chỉ được khởi động đúng 1 lần, bất kể RateLimiterMiddleware() được gọi
	// bao nhiêu lần — không còn phụ thuộc vào việc main.go/app.go có nhớ gọi
	// CleanupClients() thủ công hay không.
	cleanupOnce sync.Once
)

const rateLimitLogTTL = 10 * time.Second

func getClientIP(ctx *gin.Context) string {
	ip := ctx.ClientIP()
	if ip == "" {
		ip = ctx.Request.RemoteAddr
	}
	return ip
}

func getRateLimiter(key string, reqSec, burst int) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	client, exists := clients[key]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(reqSec), burst)
		newClient := &Client{limiter, time.Now()}
		clients[key] = newClient
		return limiter
	}

	client.lastSeen = time.Now()
	return client.limiter
}

// startCleanupGoroutines starts the background cleanup goroutines. This function is NOT
// exported and NOT called directly from outside — only called via
// cleanupOnce.Do() inside RateLimiterMiddleware(), ensuring it runs automatically
// exactly once so nobody has to remember to wire it up in main.go/app.go.
func startCleanupGoroutines() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			for key, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					delete(clients, key)
				}
			}
			mu.Unlock()
		}
	}()

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			rateLimitLogCache.Range(func(key, value any) bool {
				if t, ok := value.(time.Time); ok && now.Sub(t) > rateLimitLogTTL {
					rateLimitLogCache.Delete(key)
				}
				return true
			})
		}
	}()
}

func RateLimiterMiddleware(rateLimiterLogger *zerolog.Logger) gin.HandlerFunc {
	// Ensure the cleanup goroutine starts only once, at the exact moment
	// middleware is registered — no need for anyone to remember to call CleanupClients()
	// separately elsewhere.
	cleanupOnce.Do(startCleanupGoroutines)

	expectedApiKey := os.Getenv("API_KEY")

	publicSec := utils.GetIntEnv("RATE_LIMITER_PUBLIC_SEC", utils.GetIntEnv("RATE_LIMITER_REQUESTS_SEC", 20))
	publicBurst := utils.GetIntEnv("RATE_LIMITER_PUBLIC_BURST", utils.GetIntEnv("RATE_LIMITER_REQUESTS_BURST", 50))

	internalSec := utils.GetIntEnv("RATE_LIMITER_INTERNAL_SEC", 200)
	internalBurst := utils.GetIntEnv("RATE_LIMITER_INTERNAL_BURST", 500)

	return func(ctx *gin.Context) {
		ip := getClientIP(ctx)

		isInternalService := false
		if expectedApiKey != "" {
			apiKey := ctx.GetHeader("X-API-KEY")
			if apiKey == "" {
				apiKey = ctx.GetHeader("x-api-key")
			}
			if apiKey != "" && subtle.ConstantTimeCompare([]byte(apiKey), []byte(expectedApiKey)) == 1 {
				isInternalService = true
			}
		}

		var limiter *rate.Limiter
		var limiterKey string

		if isInternalService {
			limiterKey = "internal:" + ip
			limiter = getRateLimiter(limiterKey, internalSec, internalBurst)
		} else {
			limiterKey = "public:" + ip
			limiter = getRateLimiter(limiterKey, publicSec, publicBurst)
		}

		if !limiter.Allow() {
			if shouldLogRateLimit(limiterKey) {
				rateLimiterLogger.Warn().
					Str("path", ctx.Request.URL.Path).
					Str("method", ctx.Request.Method).
					Str("query", ctx.Request.URL.RawQuery).
					Str("client_ip", ip).
					Bool("is_internal", isInternalService).
					Str("user_agent", ctx.Request.UserAgent()).
					Msg("rate limit exceeded")
			}

			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "too many requests",
				"message": "Too many requests. Please try again later.",
			})
			return
		}

		ctx.Next()
	}
}

func shouldLogRateLimit(key string) bool {
	now := time.Now()

	if val, ok := rateLimitLogCache.Load(key); ok {
		if t, ok := val.(time.Time); ok && now.Sub(t) < rateLimitLogTTL {
			return false
		}
	}
	rateLimitLogCache.Store(key, now)
	return true
}

// RateLimit returns a Gin middleware that enforces a sliding window rate limit per client IP
// on specific sensitive endpoints using Redis atomic INCR and TTL.
func RateLimit(cacheService cache.RedisCacheService, action string, limit int64, window time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if cacheService == nil {
			ctx.Next()
			return
		}

		ip := getClientIP(ctx)
		key := fmt.Sprintf("ratelimit:auth:%s:%s", action, ip)

		count, err := cacheService.Incr(key)
		if err != nil {
			// Fail open on Redis communication error to prevent locking out valid users
			ctx.Next()
			return
		}

		// First request in this window -> establish expiry
		if count == 1 {
			_ = cacheService.Expire(key, window)
		}

		if count > limit {
			ttl, err := cacheService.TTL(key)
			retryAfterSec := int(window.Seconds())
			if err == nil && ttl > 0 {
				retryAfterSec = int(ttl.Seconds())
			}
			if retryAfterSec < 1 {
				retryAfterSec = 1
			}

			ctx.Header("Retry-After", strconv.Itoa(retryAfterSec))
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    utils.ErrCodeTooManyRequests,
				"error":   fmt.Sprintf("Too many requests for %s. Please try again in %d seconds.", action, retryAfterSec),
				"message": "Too many requests. Please try again later.",
			})
			return
		}

		ctx.Next()
	}
}