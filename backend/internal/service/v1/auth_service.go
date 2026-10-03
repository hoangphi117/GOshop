package v1service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/mail"
	"goshop/pkg/rabbitmq"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	passwordResetTokenTTL = time.Hour
	loginRateLimitTTL     = 5 * time.Minute
	maxLoginAttempts      = 5

	UserStatusActive   = 1
	UserStatusInactive = 2
	UserStatusBanned   = 3
	UserLevelUser      = 3
)

type authService struct {
	userRepo      repository.UserRepository
	tokenService  auth.TokenService
	cacheService  cache.RedisCacheService
	mailService   mail.EmailProviderService
	rabbitmq      rabbitmq.RabbitMQService
	storefrontURL string
}

func NewAuthService(
	userRepo repository.UserRepository,
	tokenService auth.TokenService,
	cacheService cache.RedisCacheService,
	mailService mail.EmailProviderService,
	rabbitmqService rabbitmq.RabbitMQService,
	storefrontURL string,
) AuthService {
	return &authService{
		userRepo:      userRepo,
		tokenService:  tokenService,
		cacheService:  cacheService,
		mailService:   mailService,
		rabbitmq:      rabbitmqService,
		storefrontURL: strings.TrimRight(storefrontURL, "/"),
	}
}

// ----------------- Redis Rate Limiter Helpers -----------------

func (as *authService) checkLoginRateLimit(ip string) error {
	if ip == "" || as.cacheService == nil {
		return nil
	}

	key := fmt.Sprintf("auth:ratelimit:login:%s", ip)
	var attempts int
	if err := as.cacheService.Get(key, &attempts); err == nil && attempts >= maxLoginAttempts {
		return utils.NewError(utils.ErrCodeTooManyRequests, "Too many login attempts. Please try again later.")
	}

	return nil
}

func (as *authService) recordFailedLogin(ip string) {
	if ip == "" || as.cacheService == nil {
		return
	}
	key := fmt.Sprintf("auth:ratelimit:login:%s", ip)
	var attempts int
	_ = as.cacheService.Get(key, &attempts)
	attempts++
	_ = as.cacheService.Set(key, attempts, loginRateLimitTTL)
}

func (as *authService) resetLoginRateLimit(ip string) {
	if ip == "" || as.cacheService == nil {
		return
	}

	key := fmt.Sprintf("auth:ratelimit:login:%s", ip)
	_ = as.cacheService.Del(key)
}

// ----------------- Core Auth Methods -----------------

func (as *authService) Login(ctx context.Context, ip string, email, password string) (string, string, int, error) {
	if err := as.checkLoginRateLimit(ip); err != nil {
		return "", "", 0, err
	}

	email = utils.NormalizeString(email)
	user, err := as.userRepo.GetByEmail(ctx, email)
	if err != nil {
		as.recordFailedLogin(ip)
		return "", "", 0, utils.NewError(utils.ErrCodeUnauthorized, "Invalid email or password")
	}

	if user.UserPassword == nil || *user.UserPassword == "" {
		as.recordFailedLogin(ip)
		return "", "", 0, utils.NewError(utils.ErrCodeUnauthorized, "Invalid email or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(*user.UserPassword), []byte(password))
	if err != nil {
		as.recordFailedLogin(ip)
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

	as.resetLoginRateLimit(ip)

	return accessToken, refreshToken.Token, int(auth.AccessTokenTTL.Seconds()), nil
}

func (as *authService) Logout(ctx context.Context, accessToken string, refreshToken string) error {
	if accessToken != "" {
		_, claims, err := as.tokenService.ParseToken(accessToken)
		if err == nil && claims != nil {
			if jti, ok := claims["jti"].(string); ok {
				expUnix, _ := claims["exp"].(float64)
				exp := time.Unix(int64(expUnix), 0)
				key := "blacklist:" + jti
				ttl := time.Until(exp)
				if as.cacheService != nil && ttl > 0 {
					_ = as.cacheService.Set(key, "revoked", ttl)
				}
			}
		}
	}

	if refreshToken != "" {
		_, err := as.tokenService.ValidateRefreshToken(refreshToken)
		if err != nil {
			return utils.NewError(utils.ErrCodeUnauthorized, "Refresh token is invalid or revoked")
		}

		if err := as.tokenService.RevokeRefreshToken(refreshToken); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to revoke old refresh token", err)
		}
	}

	return nil
}

func (as *authService) RefreshToken(ctx context.Context, refreshToken string) (string, string, int, error) {
	userUUIDStr, newRefreshToken, err := as.tokenService.RotateRefreshToken(refreshToken)
	if err != nil {
		return "", "", 0, err
	}

	userUuid, err := uuid.Parse(userUUIDStr)
	if err != nil {
		return "", "", 0, utils.NewError(utils.ErrCodeUnauthorized, "Invalid user UUID in token")
	}

	user, err := as.userRepo.GetByUUID(ctx, userUuid)
	if err != nil {
		return "", "", 0, utils.NewError(utils.ErrCodeNotFound, "User not found")
	}

	accessToken, err := as.tokenService.GenerateAccessToken(user)
	if err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to generate access token", err)
	}

	return accessToken, newRefreshToken, int(auth.AccessTokenTTL.Seconds()), nil
}

func (as *authService) RequestForgotPassword(ctx context.Context, email string) error {
	email = utils.NormalizeString(email)

	rateLimitKey := fmt.Sprintf("reset:ratelimit:%s", email)
	if as.cacheService != nil {
		if exists, err := as.cacheService.Exists(rateLimitKey); err == nil && exists {
			return utils.NewError(utils.ErrCodeTooManyRequests, "You can only request a password reset once every 5 minutes.")
		}
	}

	user, err := as.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return utils.NewError(utils.ErrCodeNotFound, "Email not found")
	}

	token, err := utils.GenerateRandomString(16)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to generate reset token", err)
	}

	if as.cacheService != nil {
		err = as.cacheService.Set("reset:"+token, user.UserUuid.String(), passwordResetTokenTTL)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to store reset token", err)
		}

		err = as.cacheService.Set(rateLimitKey, "1", 5*time.Minute)
		if err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to store rate limit reset password", err)
		}
	}

	resetLink := fmt.Sprintf("%s/reset-password?token=%s", as.storefrontURL, token)

	htmlBody, textBody, err := mail.ResetPasswordTemplate(
		user.UserFullname,
		resetLink,
		passwordResetTokenTTL,
		time.Now().Year(),
	)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to render reset password email", err)
	}

	mailContent := &mail.Email{
		To:       []mail.Address{{Email: user.UserEmail}},
		Subject:  "Reset Your Password",
		Text:     textBody,
		HTML:     htmlBody,
		Category: "password-reset",
	}

	if as.rabbitmq != nil {
		if err := as.rabbitmq.Publish(ctx, "auth_email_queue", mailContent); err != nil {
			return utils.NewError(utils.ErrCodeInternal, "Failed to publish reset password email to queue")
		}
	}

	return nil
}

func (as *authService) ResetPassword(ctx context.Context, token, newPassword string) error {
	if as.cacheService == nil {
		return utils.NewError(utils.ErrCodeInternal, "Cache service is not available")
	}

	var userUuidStr string
	err := as.cacheService.Get("reset:"+token, &userUuidStr)
	if err == redis.Nil || userUuidStr == "" {
		return utils.NewError(utils.ErrCodeNotFound, "Invalid or expired reset token")
	}

	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to retrieve reset token", err)
	}

	userUuid, err := uuid.Parse(userUuidStr)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to parse user UUID", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to hash password", err)
	}

	hashed := string(hashedPassword)
	var input = sqlc.UpdatePasswordParams{
		UserUuid:    userUuid,
		NewPassword: &hashed,
	}

	_, err = as.userRepo.UpdatePassword(ctx, input)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to update password", err)
	}

	_ = as.cacheService.Del("reset:" + token)

	return nil
}

func (as *authService) Register(ctx context.Context, input v1dto.RegisterInput) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to hash password", err)
	}

	hashed := string(hashedPassword)
	user := sqlc.CreateUserParams{
		UserFullname: input.Name,
		UserEmail:    utils.NormalizeString(input.Email),
		UserPassword: &hashed,
		UserAge:      input.Age,
		UserStatus:   UserStatusActive,
		UserLevel:    UserLevelUser,
	}

	if _, err := as.userRepo.Create(ctx, user); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return utils.NewError(utils.ErrCodeConflict, "email already exists")
		}
		return utils.WrapError(utils.ErrCodeInternal, "failed to register user", err)
	}

	return nil
}

// -------------- OAuth / Social Login Methods --------------

func verifyGoogleIDToken(ctx context.Context, idToken string) (*v1dto.GoogleTokenInfo, error) {
	if strings.TrimSpace(idToken) == "" {
		return nil, utils.NewError(utils.ErrCodeBadRequest, "Google ID token is required")
	}

	// Call Google's tokeninfo endpoint
	url := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", url.QueryEscape(idToken))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "Failed to create request to Google tokeninfo endpoint", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, utils.WrapError(utils.ErrCodeExternalService, "failed to verify token with Google", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, utils.NewError(utils.ErrCodeUnauthorized, "invalid Google ID token")
	}

	var tokenInfo v1dto.GoogleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err != nil {
		return nil, utils.WrapError(utils.ErrCodeInternal, "failed to decode Google token info", err)
	}

	// Validate Client ID
	expectedClientID := utils.GetEnv("GOOGLE_CLIENT_ID", "")
	if expectedClientID != "" && tokenInfo.Aud != expectedClientID {
		return nil, utils.NewError(utils.ErrCodeUnauthorized, "Google ID token audience mismatch")
	}

	if tokenInfo.EmailVerified != "true" {
		return nil, utils.NewError(utils.ErrCodeUnauthorized, "Google account email is not verified")
	}

	return &tokenInfo, nil
}

func isNotFound(err error) bool {
	return errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sql.ErrNoRows)
}

func (as *authService) findOrCreateGoogleUser(ctx context.Context, payload *v1dto.GoogleTokenInfo) (sqlc.User, error) {
	provider := "google"
	oauthID := payload.Sub
	email := utils.NormalizeString(payload.Email)

	var user sqlc.User

	// Find user by OAuth provider and ID (already registered)
	user, err := as.userRepo.GetByOAuth(ctx, provider, oauthID)
	if err == nil {
		return user, nil
	}
	if !isNotFound(err) {
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to query user by OAuth provider and ID", err)
	}

	// User not found by OAuth, try to find by email (existing user)
	user, err = as.userRepo.GetByEmail(ctx, email)
	if err == nil {
		return user, nil
	}
	if !isNotFound(err) {
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to query user by email", err)
	}

	// User not found, create a new user
	var avatarURL *string
	if payload.Picture != "" {
		avatarURL = &payload.Picture
	}

	newUser, err := as.userRepo.CreateOAuthUser(ctx, sqlc.CreateOAuthUserParams{
		Fullname:  payload.Name,
		Email:     email,
		Provider:  &provider,
		OauthID:   &oauthID,
		AvatarUrl: avatarURL,
	})
	if err != nil {
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to create new OAuth user", err)
	}

	return newUser, nil
}

func (as *authService) GoogleLogin(ctx context.Context, ip, idToken string) (string, string, int, error) {
	if err := as.checkLoginRateLimit(ip); err != nil {
		return "", "", 0, err
	}

	// Verify the Google ID token
	payload, err := verifyGoogleIDToken(ctx, idToken)
	if err != nil {
		as.recordFailedLogin(ip)
		return "", "", 0, err
	}

	user, err := as.findOrCreateGoogleUser(ctx, payload)
	if err != nil {
		return "", "", 0, err
	}

	if user.UserStatus == UserStatusInactive || user.UserStatus == UserStatusBanned {
		return "", "", 0, utils.NewError(utils.ErrCodeForbidden, "User account is inactive or banned")
	}

	// Generate access and refresh tokens
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

	// Reset rate limit after successful login
	as.resetLoginRateLimit(ip)

	return accessToken, refreshToken.Token, int(auth.AccessTokenTTL.Seconds()), nil

}
