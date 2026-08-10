package middleware

import (
	"goshop/internal/utils"
	"net/http"
	"strconv"
	"sync"
	"time"

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
)

func getClientIP(ctx *gin.Context) string {
	ip := ctx.ClientIP()
	if ip == "" {
		ip = ctx.Request.RemoteAddr
	}

	return ip
}

func GetRateLimiter(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	client, exists := clients[ip]
	if !exists {

		requestSecStr := utils.GetEnv("RATE_LIMITER_REQUESTS_SEC", "5")
		requestBurstStr := utils.GetEnv("RATE_LIMITER_REQUESTS_BURST", "10")

		requestSec, err := strconv.Atoi(requestSecStr)
		if err != nil {
			panic("Invalid RATE_LIMITER_REQUESTS_SEC value: " + err.Error())
		}

		burst, err := strconv.Atoi(requestBurstStr)
		if err != nil {
			panic("Invalid RATE_LIMITER_REQUESTS_BURST value: " + err.Error())
		}

		limiter := rate.NewLimiter(rate.Limit(requestSec), burst)
		newClient := &Client{limiter, time.Now()}
		clients[ip] = newClient

		return limiter
	}

	client.lastSeen = time.Now()
	return client.limiter
}

func CleanupClients() {
	for {
		time.Sleep(time.Minute)
		mu.Lock()
		for ip, client := range clients {
			if time.Since(client.lastSeen) > 3*time.Minute {
				delete(clients, ip)
			}
		}
		mu.Unlock()
	}
}

func RateLimiterMiddleware(rateLimiterLogger *zerolog.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ip := getClientIP(ctx)

		limiter := GetRateLimiter(ip)

		if !limiter.Allow() {
			if shouldLogRateLimit(ip) {

				rateLimiterLogger.Warn().
					Str("path", ctx.Request.URL.Path).
					Str("method", ctx.Request.Method).
					Str("query", ctx.Request.URL.RawQuery).
					Str("client_ip", ctx.ClientIP()).
					Str("user_agent", ctx.Request.UserAgent()).
					Str("referer", ctx.Request.Referer()).
					Str("protocol", ctx.Request.Proto).
					Str("host", ctx.Request.Host).
					Str("remote_addr", ctx.Request.RemoteAddr).
					Str("request_uri", ctx.Request.RequestURI).
					Interface("headers", ctx.Request.Header).
					Msg("rate limit exceeded")
			}
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "too many requests",
				"message": "Bạn gửi quá nhiều request. Hãy thử lại sau",
			})

			return
		}
		ctx.Next()
	}
}

var rateLimitLogCache = sync.Map{}

const rateLimitLogTTL = 10 * time.Second

func shouldLogRateLimit(ip string) bool {
	now := time.Now()

	if val, ok := rateLimitLogCache.Load(ip); ok {
		if t, ok := val.(time.Time); ok && now.Sub(t) < rateLimitLogTTL {
			return false
		}
	}
	rateLimitLogCache.Store(ip, now)
	return true
}
