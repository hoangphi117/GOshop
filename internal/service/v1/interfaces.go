package v1service

import (
	"goshop/internal/db/sqlc"

	"github.com/gin-gonic/gin"
)

type UserService interface {
	GetAllUsers(search string, page, limit int)
	CreateUsers(ctx *gin.Context, input sqlc.CreateUserParams) (sqlc.User, error)
	GetUserByUUID(uuid string)
	UpdateUser(uuid string)
	DeleteUser(uuid string)
}
