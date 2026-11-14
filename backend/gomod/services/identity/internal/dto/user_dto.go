package dto

import (
	"os"

	"goshop/services/identity/internal/db/sqlc"
	"goshop/shared/pkg/utils"

	"github.com/google/uuid"
)

type UserDTO struct {
	UUID      string `json:"uuid"`
	Name      string `json:"full_name"`
	Email     string `json:"email"`
	Age       *int   `json:"age"`
	Status    string `json:"status"`
	Level     string `json:"level"`
	CreatedAt string `json:"created_at"`
	// AvatarURL is the edge-relative media path ("/goshop-media/avatars/x.png"),
	// or an external URL when the account signed in with a provider that
	// supplied one. Empty when the user never set an avatar.
	AvatarURL string `json:"avatar_url"`
}

type CreateUserInput struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email,email_advanced"`
	Age      int32  `json:"age" binding:"omitempty,gt=0"`
	Password string `json:"password" binding:"required,min=6,password_strong"`
	Status   int32  `json:"status" binding:"required,oneof=1 2 3"`
	Level    int32  `json:"level" binding:"required,oneof=1 2 3"`
}

type UpdateUserInput struct {
	Name     *string `json:"name" binding:"omitempty"`
	Age      *int32  `json:"age" binding:"omitempty,gt=0"`
	Password *string `json:"password" binding:"omitempty,min=6,password_strong"`
	Status   *int32  `json:"status" binding:"omitempty,oneof=1 2"`
	Level    *int32  `json:"level" binding:"omitempty,oneof=1 2"`
}

type UpdateProfileInput struct {
	Name     *string `json:"name" binding:"omitempty,min=2,max=100"`
	Age      *int32  `json:"age" binding:"omitempty,gt=0"`
	Password *string `json:"password" binding:"omitempty,min=6,password_strong"`
}

// UpdateAvatarInput carries the storage key returned by /upload/confirm, never
// a client-built URL: the identity service trusts only objects the media
// service already verified (size, magic bytes) under the avatars/ prefix.
// An empty value clears the avatar.
type UpdateAvatarInput struct {
	FileKey string `json:"file_key" binding:"omitempty,max=255"`
}

type GoogleLoginInput struct {
	IDToken string `json:"id_token" validate:"required"`
}

type ChangePasswordRequest struct {
	OldPassword     string `json:"old_password" binding:"required,min=8,max=32"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=32"`
	ConfirmPassword string `json:"confirm_password" binding:"required,eqfield=NewPassword"`
}

type GetUserByUuidParam struct {
	Uuid string `uri:"uuid" binding:"uuid"`
}

type GetUserParam struct {
	Search  string `form:"search" binding:"omitempty,min=3,max=50,search"`
	Page    int32  `form:"page" binding:"omitempty,gte=1"`
	Limit   int32  `form:"limit" binding:"omitempty,gte=1,lte=500"`
	OrderBy string `form:"order_by" binding:"omitempty,oneof=user_id user_created_at"`
	Sort    string `form:"sort" binding:"omitempty,oneof=asc desc"`
}

func (input *CreateUserInput) MapCreateInputToModel() sqlc.CreateUserParams {
	var userPassword *string
	if input.Password != "" {
		userPassword = &input.Password
	}
	return sqlc.CreateUserParams{
		UserFullname: input.Name,
		UserEmail:    input.Email,
		UserAge:      utils.ConvertToInt32Pointer(input.Age),
		UserPassword: userPassword,
		UserStatus:   input.Status,
		UserLevel:    input.Level,
	}
}

func (input *UpdateUserInput) MapUpdateInputToModel(userUuid uuid.UUID) sqlc.UpdateUserParams {
	return sqlc.UpdateUserParams{
		UserUuid:     userUuid,
		UserFullname: input.Name,
		UserAge:      input.Age,
		UserPassword: input.Password,
		UserStatus:   input.Status,
		UserLevel:    input.Level,
	}
}

func (input *UpdateProfileInput) MapUpdateProfileToModel(userUuid uuid.UUID) sqlc.UpdateUserParams {
	return sqlc.UpdateUserParams{
		UserUuid:     userUuid,
		UserFullname: input.Name,
		UserAge:      input.Age,
		UserPassword: input.Password,
	}
}

func MapUserToDTO(user sqlc.User) *UserDTO {
	dto := &UserDTO{
		UUID:      user.UserUuid.String(),
		Name:      user.UserFullname,
		Email:     user.UserEmail,
		Status:    mapStatusText(int(user.UserStatus)),
		Level:     mapLevelText(int(user.UserLevel)),
		CreatedAt: user.UserCreatedAt.Format("2006-01-02 15:04:05"),
		AvatarURL: AvatarPath(user.UserAvatarUrl),
	}

	if user.UserAge != nil {
		age := int(*user.UserAge)
		dto.Age = &age
	}

	return dto
}

func MapListUsersToDTO(users []sqlc.User) []UserDTO {
	dtos := make([]UserDTO, 0, len(users))

	for _, user := range users {
		dtos = append(dtos, *MapUserToDTO(user))
	}

	return dtos
}

// AvatarPath flattens the nullable avatar column into the DTO's single string
// field. Storage may hold the absolute URL it minted at upload time; the DTO
// carries the edge-relative path so the browser (and a later domain change)
// resolve it. Exported because both mappers (dto and service) need it.
func AvatarPath(raw *string) string {
	if raw == nil {
		return ""
	}
	return utils.PublicMediaPath(os.Getenv("S3_PUBLIC_BASE_URL"), *raw)
}

func mapStatusText(status int) string {
	switch status {
	case 1:
		return "Active"
	case 2:
		return "Inactive"
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

// InternalUserDTO is used for lightweight inter-service REST IPC
type InternalUserDTO struct {
	UserID        int32     `json:"user_id"`
	UserUUID      uuid.UUID `json:"user_uuid"`
	UserFullname  string    `json:"user_fullname"`
	UserEmail     string    `json:"user_email"`
	UserAvatar    string    `json:"user_avatar"`
	UserAvatarUrl *string   `json:"user_avatar_url,omitempty"`
	UserStatus    int32     `json:"user_status"`
	UserLevel     int32     `json:"user_level"`
}

// InternalBatchUsersRequest is the payload for batch fetching users
type InternalBatchUsersRequest struct {
	UserIDs []int32 `json:"user_ids" binding:"required"`
}

// InternalBatchUsersResponse is the payload returned by internal batch endpoint
type InternalBatchUsersResponse struct {
	Users map[int32]InternalUserDTO `json:"users"`
}
