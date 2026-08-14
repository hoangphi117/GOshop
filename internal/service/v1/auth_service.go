package v1service

import (
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	userRepo     repository.UserRepository
	tokenService auth.TokenService
}

func NewAuthService(userRepo repository.UserRepository, tokenService auth.TokenService) *authService {
	return &authService{
		userRepo:     userRepo,
		tokenService: tokenService,
	}
}

func (as *authService) Login(ctx *gin.Context, email, password string) (string, string, int, error) {

	context := ctx.Request.Context()

	email = utils.NormalizeString(email)

	user, err := as.userRepo.GetByEmail(context, email)
	if err != nil {
		return "", "", 0, utils.NewError(utils.ErrCodeUnauthorized, "Invalid email or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.UserPassword), []byte(password))
	if err != nil {
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

	return accessToken, refreshToken.Token, int(auth.AccessTokenTTL.Seconds()), nil
}

func (as *authService) Logout(ctx *gin.Context, token string) error {
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
