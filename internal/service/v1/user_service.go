package v1service

import (
	"errors"
	"goshop/internal/db/sqlc"
	"goshop/internal/repository"
	"goshop/internal/utils"
	"strconv"

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

func (us *userService) GetAllUsers(ctx *gin.Context, search, orderBy, sort string, page, limit int32) ([]sqlc.User, int32, error) {

	context := ctx.Request.Context()

	if sort == "" {
		sort = "desc"
	}

	if orderBy == "" {
		orderBy = "user_created_at"
	}

	if page <= 0 {
		page = 1
	}

	if limit <= 0 {
		envLimit := utils.GetEnv("DEFAULT_LIMIT", "10")
		limitInt, err := strconv.Atoi(envLimit)
		if err != nil || limitInt <= 0 {
			limitInt = 10
		}

		limit = int32(limitInt)
	}

	offset := (page - 1) * limit

	listUsers, err := us.repo.GetAll(context, search, orderBy, sort, limit, offset)
	if err != nil {
		return []sqlc.User{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to fetch user list", err)
	}

	total, err := us.repo.CountUsers(ctx, search)
	if err != nil {
		return []sqlc.User{}, 0, utils.WrapError(utils.ErrCodeInternal, "failed to count users", err)
	}

	return listUsers, int32(total), nil
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
