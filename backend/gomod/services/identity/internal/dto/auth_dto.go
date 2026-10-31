package dto

type LoginInput struct {
	Email    string `json:"email" binding:"required,email,email_advanced"`
	Password string `json:"password" binding:"required,min=6"`
}

type RefreshTokenInput struct {
	RefreshToken string `json:"refresh_token" binding:"omitempty"`
}

type RequestForgotPasswordInput struct {
	Email string `json:"email" binding:"required,email,email_advanced"`
}

type ResetPasswordInput struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type RegisterInput struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email,email_advanced"`
	Password string `json:"password" binding:"required,min=6,password_strong"`
	Age      *int32 `json:"age" binding:"omitempty,gt=0"`
}

type SendRegisterOTPInput struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email,email_advanced"`
	Password string `json:"password" binding:"required,min=6,password_strong"`
	Age      *int32 `json:"age" binding:"omitempty,gt=0"`
}

type VerifyRegisterOTPInput struct {
	Email string `json:"email" binding:"required,email,email_advanced"`
	OTP   string `json:"otp" binding:"required,len=6,numeric"`
}

type ResendRegisterOTPInput struct {
	Email string `json:"email" binding:"required,email,email_advanced"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type GoogleTokenInfo struct {
	Iss           string `json:"iss"`
	Sub           string `json:"sub"` // Google User ID unique
	Azp           string `json:"azp"`
	Aud           string `json:"aud"` // Client ID
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"` // "true" or "false"
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
}
