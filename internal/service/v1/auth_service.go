package v1service

import (
	"goshop/internal/repository"
	"goshop/internal/utils"
	"goshop/pkg/auth"

	"github.com/gin-gonic/gin"
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

func (as *authService) Login(ctx *gin.Context, email, password string) (string, int, error) {

	context := ctx.Request.Context()

	email = utils.NormalizeString(email)

	user, err := as.userRepo.GetByEmail(context, email)
	if err != nil {
		return "", 0, utils.NewError(utils.ErrCodeUnauthorized, "Invalid email or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.UserPassword), []byte(password))
	if err != nil {
		return "", 0, utils.NewError(utils.ErrCodeUnauthorized, "Invalid email or password")
	}

	accessToken, err := as.tokenService.GenerateAccessToken(user)
	if err != nil {
		return "", 0, utils.WrapError(utils.ErrCodeInternal, "Failed to generate access token", err)
	}

	return accessToken, int(auth.AccessTokenTTL.Seconds()), nil
}

func (as *authService) Logout(ctx *gin.Context, token string) error {
	return nil
}
