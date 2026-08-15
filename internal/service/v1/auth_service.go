package v1service

import (
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
)

type authService struct {
	userRepo     repository.UserRepository
	tokenService auth.TokenService
	cache        cache.RedisCacheService
}

type LoginAttempt struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	mu               sync.Mutex
	clients          = make(map[string]*LoginAttempt)
	LoginAttemptTTL  = 5 * time.Minute
	MaxLoginAttempts = 5
)

func NewAuthService(userRepo repository.UserRepository, tokenService auth.TokenService, cache cache.RedisCacheService) *authService {
	return &authService{
		userRepo:     userRepo,
		tokenService: tokenService,
		cache:        cache,
	}
}

func (as *authService) getClientIP(ctx *gin.Context) string {
	ip := ctx.ClientIP()
	if ip == "" {
		ip = ctx.Request.RemoteAddr
	}

	return ip
}

func (as *authService) getLoginAttempt(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	client, exists := clients[ip]
	if !exists {

		limiter := rate.NewLimiter(rate.Limit(float32(MaxLoginAttempts)/float32(LoginAttemptTTL.Seconds())), MaxLoginAttempts)
		newClient := &LoginAttempt{limiter, time.Now()}
		clients[ip] = newClient

		return limiter
	}

	client.lastSeen = time.Now()
	return client.limiter
}

func (as *authService) CheckLoginAttempt(ip string) error {
	limiter := as.getLoginAttempt(ip)
	if !limiter.Allow() {
		return utils.NewError(utils.ErrCodeTooManyRequests, "Too many login attempts. Please try again later.")
	}
	return nil
}

func (as *authService) CleanupClients(ip string) {
	mu.Lock()
	defer mu.Unlock()

	delete(clients, ip)
}

func (as *authService) Login(ctx *gin.Context, email, password string) (string, string, int, error) {

	context := ctx.Request.Context()

	ip := as.getClientIP(ctx)

	if err := as.CheckLoginAttempt(ip); err != nil {
		return "", "", 0, err
	}

	email = utils.NormalizeString(email)
	user, err := as.userRepo.GetByEmail(context, email)
	if err != nil {
		as.getLoginAttempt(ip)
		return "", "", 0, utils.NewError(utils.ErrCodeUnauthorized, "Invalid email or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.UserPassword), []byte(password))
	if err != nil {
		as.getLoginAttempt(ip)
		return "", "", 0, utils.NewError(utils.ErrCodeUnauthorized, "Invalid email or password")
	}

	accessToken, err := as.tokenService.GenerateAccessToken(user)
	if err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to generate access token", err)
	}

	refreshToken, err := as.tokenService.GenerateRefreshToken(user)
	if err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to generate refresh token", err)
	}

	if err := as.tokenService.StoreRefreshToken(refreshToken); err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to store refresh token", err)
	}

	as.CleanupClients(ip)

	return accessToken, refreshToken.Token, int(auth.AccessTokenTTL.Seconds()), nil
}

func (as *authService) Logout(ctx *gin.Context, refreshToken string) error {
	authHeader := ctx.GetHeader("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return utils.NewError(utils.ErrCodeUnauthorized, "Authorization header missing or invalid")
	}

	accessToken := strings.TrimPrefix(authHeader, "Bearer ")

	_, claims, err := as.tokenService.ParseToken(accessToken)
	if err != nil {
		return utils.NewError(utils.ErrCodeUnauthorized, "Invalid access token")
	}

	if jti, ok := claims["jti"].(string); ok {
		expUnix, _ := claims["exp"].(float64)
		exp := time.Unix(int64(expUnix), 0)
		key := "blacklist:" + jti
		ttl := time.Until(exp)
		as.cache.Set(key, "revoked", ttl)
	}

	_, err = as.tokenService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return utils.NewError(utils.ErrCodeUnauthorized, "Refresh token is invalid or revoked")
	}

	if err := as.tokenService.RevokeRefreshToken(refreshToken); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to revoke old refresh token", err)
	}

	return nil
}

func (as *authService) RefreshToken(ctx *gin.Context, refreshToken string) (string, string, int, error) {

	context := ctx.Request.Context()

	token, err := as.tokenService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", "", 0, utils.NewError(utils.ErrCodeUnauthorized, "Refresh token is invalid or revoked")
	}

	userUuid, _ := uuid.Parse(token.UserUUID)
	user, err := as.userRepo.GetByUUID(context, userUuid)
	if err != nil {
		return "", "", 0, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	accessToken, err := as.tokenService.GenerateAccessToken(user)
	if err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to generate access token", err)
	}

	refreshTokenNew, err := as.tokenService.GenerateRefreshToken(user)
	if err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to generate refresh token", err)
	}

	if err := as.tokenService.RevokeRefreshToken(refreshToken); err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to revoke old refresh token", err)
	}

	if err := as.tokenService.StoreRefreshToken(refreshTokenNew); err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to store refresh token", err)
	}

	return accessToken, refreshTokenNew.Token, int(auth.AccessTokenTTL.Seconds()), nil
}
