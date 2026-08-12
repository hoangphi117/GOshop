package v1service

import (
	"goshop/internal/db/sqlc"

	"github.com/gin-gonic/gin"
)

type UserService interface {
	GetAllUsers(ctx *gin.Context, search, orderBy, sort string, page, limit int32) ([]sqlc.User, int32, error)
	CreateUsers(ctx *gin.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	GetUserByUUID(uuid string)
	UpdateUser(ctx *gin.Context, input sqlc.UpdateUserParams) (sqlc.User, error)
	DeleteUser(uuid string)
}
