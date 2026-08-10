package v1dto

import (
	"goshop/internal/db/sqlc"
	"goshop/internal/utils"
)

type UserDTO struct {
	UUID      string `json:"uuid"`
	Name      string `json:"full_name"`
	Email     string `json:"email"`
	Age       *int   `json:"age"`
	Status    string `json:"status"`
	Level     string `json:"level"`
	CreatedAt string `json:"created_at"`
}

type CreateUserInput struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email,email_advanced"`
	Age      int    `json:"age" binding:"omitempty,gt=0"`
	Password string `json:"password" binding:"required,min=6,password_strong"`
	Status   int    `json:"status" binding:"required,oneof=1 2 3"`
	Level    int    `json:"level" binding:"required,oneof=1 2 3"`
}

type UpdateUserInput struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email,email_advanced"`
	Age      int    `json:"age" binding:"required,gt=0"`
	Password string `json:"password" binding:"omitempty,min=6,password_strong"`
	Status   int    `json:"status" binding:"required,oneof=1 2"`
	Level    int    `json:"level" binding:"required,oneof=1 2"`
}

func (input *CreateUserInput) MapCreateInputToModel() sqlc.CreateUserParams {
	return sqlc.CreateUserParams{
		UserFullname: input.Name,
		UserEmail:    input.Email,
		UserAge:      utils.ConvertToInt32Pointer(input.Age),
		UserPassword: input.Password,
		UserStatus:   int32(input.Status),
		UserLevel:    int32(input.Level),
	}
}

func (input *UpdateUserInput) MapUpdateInputToModel() {

}

func MapUserToDTO(user sqlc.User) *UserDTO {
	dto := &UserDTO{
		UUID:      user.UserUuid.String(),
		Name:      user.UserFullname,
		Email:     user.UserEmail,
		Status:    mapStatusText(int(user.UserStatus)),
		Level:     mapLevelText(int(user.UserLevel)),
		CreatedAt: user.UserCreatedAt.Format("2006-01-02 15:04:05"),
	}

	if user.UserAge != nil {
		age := int(*user.UserAge)
		dto.Age = &age
	}

	return dto
}

func mapStatusText(status int) string {
	switch status {
	case 1:
		return "Show"
	case 2:
		return "Hide"
	case 3:
		return "Banned"
	default:
		return "None"
	}
}

func mapLevelText(level int) string {
	switch level {
	case 1:
		return "Admin"
	case 2:
		return "Moderator"
	case 3:
		return "Member"
	default:
		return "None"
	}
}
