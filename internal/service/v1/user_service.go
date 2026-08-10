package v1service

import (
	"errors"
	"goshop/internal/db/sqlc"
	"goshop/internal/repository"
	"goshop/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

type userService struct {
	repo repository.UserRepository
}

func NewUserService(repo repository.UserRepository) UserService {
	return &userService{
		repo: repo,
	}
}

func (us *userService) GetAllUsers(search string, page, limit int) {

}

func (us *userService) CreateUsers(ctx *gin.Context, input sqlc.CreateUserParams) (sqlc.User, error) {

	context := ctx.Request.Context()

	input.UserEmail = utils.NormalizeString(input.UserEmail)

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.UserPassword), bcrypt.DefaultCost)
	if err != nil {
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to hash password", err)
	}
	input.UserPassword = string(hashedPassword)

	user, err := us.repo.Create(context, input)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return sqlc.User{}, utils.NewError(utils.ErrCodeConflict, "email already exists")
		}
		return sqlc.User{}, utils.WrapError(utils.ErrCodeInternal, "failed to create user", err)
	}

	return user, nil
}

func (us *userService) GetUserByUUID(uuid string) {

}

func (us *userService) UpdateUser(uuid string) {
}

func (us *userService) DeleteUser(uuid string) {
}
