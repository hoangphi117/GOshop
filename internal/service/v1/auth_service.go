package v1service

import (
	"goshop/internal/repository"
	"goshop/internal/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	userRepo repository.UserRepository
}

func NewAuthService(userRepo repository.UserRepository) *authService {
	return &authService{
		userRepo: userRepo,
	}
}

func (as *authService) Login(ctx *gin.Context, email, password string) error {

	context := ctx.Request.Context()

	email = utils.NormalizeString(email)

	user, err := as.userRepo.GetByEmail(context, email)
	if err != nil {
		return utils.NewError(utils.ErrCodeUnauthorized, "Invalid email or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.UserPassword), []byte(password))
	if err != nil {
		return utils.NewError(utils.ErrCodeUnauthorized, "Invalid email or password")
	}

	return nil
}

func (as *authService) Logout(ctx *gin.Context, token string) error {
	return nil
}
