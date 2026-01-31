package auth

type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	Name        string `json:"name" binding:"required"`
	DeviceToken string `json:"device_token"`
}

type LoginRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DeviceToken string `json:"device_token"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}
