package v1service

import (
	v1dto "goshop/internal/dto/v1"
	"goshop/internal/db/sqlc"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UserService interface {
	GetAllUsers(ctx *gin.Context, search, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.User, int32, error)
	CreateUsers(ctx *gin.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	GetUserByUUID(ctx *gin.Context, uuid uuid.UUID) (sqlc.User, error)
	UpdateUser(ctx *gin.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	GetProfile(ctx *gin.Context) (sqlc.User, error)
	UpdateProfile(ctx *gin.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	SoftDeleteUser(ctx *gin.Context, uuid uuid.UUID) (sqlc.User, error)
	RestoreUser(ctx *gin.Context, uuid uuid.UUID) (sqlc.User, error)
	TrashUser(ctx *gin.Context, uuid uuid.UUID) error
}

type AuthService interface {
	Register(ctx *gin.Context, input v1dto.RegisterInput) error
	Login(ctx *gin.Context, email, password string) (string, string, int, error)
	Logout(ctx *gin.Context, refreshToken string) error
	RefreshToken(ctx *gin.Context, refreshToken string) (string, string, int, error)
	RequestForgotPassword(ctx *gin.Context, email string) error
	ResetPassword(ctx *gin.Context, token, newPassword string) error
}

type CategoryService interface {
	CreateCategory(ctx *gin.Context, input sqlc.CreateCategoryParams) (sqlc.Category, error)
	GetAllCategories(ctx *gin.Context, search, orderBy, sort string, page, limit int32, deleted bool) ([]sqlc.Category, int32, error)
	GetCategoryByUUID(ctx *gin.Context, uuid uuid.UUID) (sqlc.Category, error)
	UpdateCategory(ctx *gin.Context, input sqlc.UpdateCategoryParams) (sqlc.Category, error)
	SoftDeleteCategory(ctx *gin.Context, uuid uuid.UUID) (sqlc.Category, error)
	RestoreCategory(ctx *gin.Context, uuid uuid.UUID) (sqlc.Category, error)
	TrashCategory(ctx *gin.Context, uuid uuid.UUID) (sqlc.Category, error)
}
