package dto

type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type UserResponse struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}
