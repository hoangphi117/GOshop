package sqlc

import "github.com/google/uuid"

// User is a lightweight representation of User for compatibility with customer queries and test mocks.
type User struct {
	UserID        int32     `json:"user_id"`
	UserUuid      uuid.UUID `json:"user_uuid"`
	UserFullname  string    `json:"user_fullname"`
	UserEmail     string    `json:"user_email"`
	UserStatus    int32     `json:"user_status"`
	UserLevel     int32     `json:"user_level"`
	UserAvatar    *string   `json:"user_avatar,omitempty"`
	UserAvatarUrl *string   `json:"user_avatar_url,omitempty"`
}
