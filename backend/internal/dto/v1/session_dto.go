package v1dto

import "time"

type SessionDTO struct {
	UUID         string    `json:"uuid"`
	IPAddress    string    `json:"ip_address"`
	DeviceName   string    `json:"device_name"`
	UserAgent    string    `json:"user_agent"`
	LastActiveAt time.Time `json:"last_active_at"`
	CreatedAt    time.Time `json:"created_at"`
	IsCurrent    bool      `json:"is_current"`
}
