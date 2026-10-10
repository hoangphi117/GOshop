package v1service

import (
	"context"
	"crypto/rand"
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
	"goshop/pkg/ws"
	"math/big"
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

	registerOTPTTL       = 5 * time.Minute
	registerOTPResendTTL = 60 * time.Second
	maxOTPAttempts       = 5

	UserStatusActive   = 1
	UserStatusInactive = 2
	UserStatusBanned   = 3
	UserLevelUser      = 3
)

type pendingRegistrationData struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	HashedPassword string `json:"hashed_password"`
	Age            *int32 `json:"age,omitempty"`
	OTP            string `json:"otp"`
	Attempts       int    `json:"attempts"`
}

func generateNumericOTP(length int) (string, error) {
	const digits = "0123456789"
	b := make([]byte, length)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		b[i] = digits[num.Int64()]
	}
	return string(b), nil
}

type authService struct {
	userRepo         repository.UserRepository
	notificationRepo repository.NotificationRepository
	tokenService     auth.TokenService
	cacheService     cache.RedisCacheService
	mailService      mail.EmailProviderService
	rabbitmq         rabbitmq.RabbitMQService
	wsHub            ws.Hub
	redisBridge      ws.RedisBridge
	storefrontURL    string
}

func NewAuthService(
	userRepo repository.UserRepository,
	notificationRepo repository.NotificationRepository,
	tokenService auth.TokenService,
	cacheService cache.RedisCacheService,
	mailService mail.EmailProviderService,
	rabbitmqService rabbitmq.RabbitMQService,
	wsHub ws.Hub,
	redisBridge ws.RedisBridge,
	storefrontURL string,
) AuthService {
	return &authService{
		userRepo:         userRepo,
		notificationRepo: notificationRepo,
		tokenService:     tokenService,
		cacheService:     cacheService,
		mailService:      mailService,
		rabbitmq:         rabbitmqService,
		wsHub:            wsHub,
		redisBridge:      redisBridge,
		storefrontURL:    strings.TrimRight(storefrontURL, "/"),
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

func (as *authService) Login(ctx context.Context, ip, userAgent string, email, password string) (string, string, int, error) {
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

	refreshToken, err := as.tokenService.GenerateRefreshToken(toUserTokenPayload(user))
	if err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to generate refresh token", err)
	}

	if err := as.tokenService.StoreRefreshToken(refreshToken); err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to store refresh token", err)
	}

	familyID := ""
	if as.tokenService != nil {
		familyID, _ = as.tokenService.GetTokenFamilyID(refreshToken.Token)
	}

	accessToken, err := as.tokenService.GenerateAccessTokenWithFamily(toUserTokenPayload(user), familyID)
	if err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to generate access token", err)
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

	familyID := ""
	if as.tokenService != nil {
		familyID, _ = as.tokenService.GetTokenFamilyID(newRefreshToken)
	}

	accessToken, err := as.tokenService.GenerateAccessTokenWithFamily(toUserTokenPayload(user), familyID)
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

func (as *authService) SendRegisterOTP(ctx context.Context, input v1dto.SendRegisterOTPInput) error {
	email := utils.NormalizeString(input.Email)

	// 1. Check if email already exists in Postgres
	existingUser, err := as.userRepo.GetByEmail(ctx, email)
	if err == nil && existingUser.UserID > 0 {
		return utils.NewError(utils.ErrCodeConflict, "email already exists")
	}

	// 2. Check Resend Cooldown
	cooldownKey := "auth:otp_resend:" + email
	if as.cacheService != nil {
		var cd string
		if err := as.cacheService.Get(cooldownKey, &cd); err == nil && cd != "" {
			return utils.NewError(utils.ErrCodeTooManyRequests, "Please wait before requesting another verification code")
		}
	}

	// 3. Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to hash password", err)
	}

	// 4. Generate 6-digit numeric OTP
	otp, err := generateNumericOTP(6)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to generate verification code", err)
	}

	// 5. Store pending registration in Redis (TTL: 5m)
	pending := pendingRegistrationData{
		Name:           input.Name,
		Email:          email,
		HashedPassword: string(hashedPassword),
		Age:            input.Age,
		OTP:            otp,
		Attempts:       0,
	}

	if as.cacheService != nil {
		if err := as.cacheService.Set("auth:register:"+email, pending, registerOTPTTL); err != nil {
			return utils.WrapError(utils.ErrCodeInternal, "Failed to store registration session", err)
		}
		_ = as.cacheService.Set(cooldownKey, "1", registerOTPResendTTL)
	}

	// 6. Send OTP Email via RabbitMQ queue
	htmlBody, textBody, err := mail.RegisterOTPTemplate(input.Name, otp, registerOTPTTL, time.Now().Year())
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to render verification email", err)
	}

	mailContent := &mail.Email{
		To:       []mail.Address{{Email: email}},
		Subject:  "Your GOshop Verification Code",
		Text:     textBody,
		HTML:     htmlBody,
		Category: "register-otp",
	}

	if as.rabbitmq != nil {
		_ = as.rabbitmq.Publish(ctx, "auth_email_queue", mailContent)
	} else if as.mailService != nil {
		_ = as.mailService.SendEmail(ctx, mailContent)
	}

	return nil
}

func (as *authService) VerifyRegisterOTP(ctx context.Context, input v1dto.VerifyRegisterOTPInput) (string, string, int, error) {
	if as.cacheService == nil {
		return "", "", 0, utils.NewError(utils.ErrCodeInternal, "Cache service is not available")
	}

	email := utils.NormalizeString(input.Email)
	regKey := "auth:register:" + email

	var pending pendingRegistrationData
	err := as.cacheService.Get(regKey, &pending)
	if err == redis.Nil || pending.Email == "" {
		return "", "", 0, utils.NewError(utils.ErrCodeBadRequest, "Verification code expired or not found. Please register again.")
	}
	if err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to retrieve registration session", err)
	}

	// Check attempts
	if pending.Attempts >= maxOTPAttempts {
		_ = as.cacheService.Del(regKey)
		_ = as.cacheService.Del("auth:otp_resend:" + email)
		return "", "", 0, utils.NewError(utils.ErrCodeBadRequest, "Too many failed attempts. Please request a new verification code.")
	}

	// Validate OTP
	if pending.OTP != strings.TrimSpace(input.OTP) {
		pending.Attempts++
		_ = as.cacheService.Set(regKey, pending, registerOTPTTL)
		return "", "", 0, utils.NewError(utils.ErrCodeBadRequest, "Invalid verification code")
	}

	// OTP is valid -> Create user in PostgreSQL
	hashed := pending.HashedPassword
	userParams := sqlc.CreateUserParams{
		UserFullname: pending.Name,
		UserEmail:    pending.Email,
		UserPassword: &hashed,
		UserAge:      pending.Age,
		UserStatus:   UserStatusActive,
		UserLevel:    UserLevelUser,
	}

	createdUser, err := as.userRepo.Create(ctx, userParams)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return "", "", 0, utils.NewError(utils.ErrCodeConflict, "email already exists")
		}
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "failed to register user", err)
	}

	// Clean up Redis session
	_ = as.cacheService.Del(regKey)
	_ = as.cacheService.Del("auth:otp_resend:" + email)

	// Generate access token and refresh token
	refreshToken, err := as.tokenService.GenerateRefreshToken(toUserTokenPayload(createdUser))
	if err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to generate refresh token", err)
	}

	if err := as.tokenService.StoreRefreshToken(refreshToken); err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to store refresh token", err)
	}

	familyID := ""
	if as.tokenService != nil {
		familyID, _ = as.tokenService.GetTokenFamilyID(refreshToken.Token)
	}

	accessToken, err := as.tokenService.GenerateAccessTokenWithFamily(toUserTokenPayload(createdUser), familyID)
	if err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to generate access token", err)
	}

	return accessToken, refreshToken.Token, int(auth.AccessTokenTTL.Seconds()), nil
}

func (as *authService) ResendRegisterOTP(ctx context.Context, input v1dto.ResendRegisterOTPInput) error {
	if as.cacheService == nil {
		return utils.NewError(utils.ErrCodeInternal, "Cache service is not available")
	}

	email := utils.NormalizeString(input.Email)
	regKey := "auth:register:" + email

	var pending pendingRegistrationData
	err := as.cacheService.Get(regKey, &pending)
	if err == redis.Nil || pending.Email == "" {
		return utils.NewError(utils.ErrCodeBadRequest, "No pending registration found for this email. Please register again.")
	}
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to retrieve registration session", err)
	}

	// Check Cooldown
	cooldownKey := "auth:otp_resend:" + email
	var cd string
	if err := as.cacheService.Get(cooldownKey, &cd); err == nil && cd != "" {
		return utils.NewError(utils.ErrCodeTooManyRequests, "Please wait before requesting another verification code")
	}

	// Generate new 6-digit OTP
	newOTP, err := generateNumericOTP(6)
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to generate verification code", err)
	}

	pending.OTP = newOTP
	pending.Attempts = 0
	if err := as.cacheService.Set(regKey, pending, registerOTPTTL); err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to update registration session", err)
	}
	_ = as.cacheService.Set(cooldownKey, "1", registerOTPResendTTL)

	// Send OTP Email
	htmlBody, textBody, err := mail.RegisterOTPTemplate(pending.Name, newOTP, registerOTPTTL, time.Now().Year())
	if err != nil {
		return utils.WrapError(utils.ErrCodeInternal, "Failed to render verification email", err)
	}

	mailContent := &mail.Email{
		To:       []mail.Address{{Email: email}},
		Subject:  "Your GOshop Verification Code",
		Text:     textBody,
		HTML:     htmlBody,
		Category: "register-otp",
	}

	if as.rabbitmq != nil {
		_ = as.rabbitmq.Publish(ctx, "auth_email_queue", mailContent)
	} else if as.mailService != nil {
		_ = as.mailService.SendEmail(ctx, mailContent)
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

func (as *authService) GoogleLogin(ctx context.Context, ip, userAgent, idToken string) (string, string, int, error) {
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
	refreshToken, err := as.tokenService.GenerateRefreshToken(toUserTokenPayload(user))
	if err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to generate refresh token", err)
	}

	if err := as.tokenService.StoreRefreshToken(refreshToken); err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to store refresh token", err)
	}

	familyID := ""
	if as.tokenService != nil {
		familyID, _ = as.tokenService.GetTokenFamilyID(refreshToken.Token)
	}

	accessToken, err := as.tokenService.GenerateAccessTokenWithFamily(toUserTokenPayload(user), familyID)
	if err != nil {
		return "", "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to generate access token", err)
	}

	// Reset rate limit after successful login
	as.resetLoginRateLimit(ip)

	return accessToken, refreshToken.Token, int(auth.AccessTokenTTL.Seconds()), nil
}

func toUserTokenPayload(user sqlc.User) auth.UserTokenPayload {
	return auth.UserTokenPayload{
		UserUUID: user.UserUuid.String(),
		Email:    user.UserEmail,
		Role:     user.UserLevel,
	}
}
