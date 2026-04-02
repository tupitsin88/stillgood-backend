package auth

type RegisterRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	Name        string `json:"name" binding:"required"`
	DeviceToken string `json:"device_token"`
}

type PartnerRegisterRequest struct {
	Email                string `json:"email" binding:"required,email"`
	Password             string `json:"password" binding:"required,min=8"`
	Name                 string `json:"name" binding:"required"`
	Phone                string `json:"phone" binding:"required"`
	CompanyName          string `json:"companyName" binding:"required"`
	Inn                  string `json:"inn" binding:"required"`
	EstablishmentName    string `json:"establishmentName" binding:"required"`
	EstablishmentAddress string `json:"establishmentAddress" binding:"required"`
	Description          string `json:"description"`
	DeviceToken          string `json:"deviceToken"`
}

type LoginRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	DeviceToken string `json:"device_token"`
}

type OAuthRequest struct {
	Provider    string `json:"provider" binding:"required,oneof=google apple"`
	IDToken     string `json:"idToken" binding:"required"`
	DeviceToken string `json:"deviceToken"`
}

type OAuthResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	IsNewUser    bool   `json:"isNewUser"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=8"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ForgotPasswordResponse struct {
	Message   string `json:"message"`
	ExpiresIn int    `json:"expiresIn"`
}

type VerifyResetCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6,numeric"`
}

type VerifyResetCodeResponse struct {
	ResetToken string `json:"resetToken"`
}

type ResetPasswordRequest struct {
	ResetToken  string `json:"resetToken" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=8"`
}

type DeleteAccountRequest struct {
	Password string `json:"password"`
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
