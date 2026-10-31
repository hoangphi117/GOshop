package contract

// Identity event topic constants
const (
	EventUserCreated = "user.created"
	EventUserUpdated = "user.updated"
	EventUserDeleted = "user.deleted"
)

// UserSyncEvent is the public event payload published by Identity Service on user lifecycle changes
type UserSyncEvent struct {
	EventType     string  `json:"event_type"`
	UserID        int32   `json:"user_id"`
	UserUUID      string  `json:"user_uuid"`
	UserFullname  string  `json:"user_fullname"`
	UserEmail     string  `json:"user_email"`
	UserAvatar    string  `json:"user_avatar"`
	UserAvatarUrl *string `json:"user_avatar_url"`
	UserStatus    int32   `json:"user_status"`
	UserLevel     int32   `json:"user_level"`
}

func NewUserSyncEvent(
	eventType string,
	userID int32,
	userUUID, fullname, email, avatar string,
	avatarURL *string,
	status, level int32,
) UserSyncEvent {
	return UserSyncEvent{
		EventType:     eventType,
		UserID:        userID,
		UserUUID:      userUUID,
		UserFullname:  fullname,
		UserEmail:     email,
		UserAvatar:    avatar,
		UserAvatarUrl: avatarURL,
		UserStatus:    status,
		UserLevel:     level,
	}
}
