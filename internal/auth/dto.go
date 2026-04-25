package auth

import "time"

type RegisterRequest struct {
	Email       string `json:"email" binding:"required"`
	Password    string `json:"password" binding:"required,min=8"`
	Name        string `json:"name" binding:"required"`
	DeviceToken string `json:"deviceToken"`
}

type PartnerRegisterRequest struct {
	Email                string `json:"email" binding:"required"`
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
	Email       string `json:"email" binding:"required"`
	Password    string `json:"password" binding:"required,min=8"`
	DeviceToken string `json:"deviceToken" binding:"required"`
}

type OAuthRequest struct {
	Provider    string `json:"provider" binding:"required,oneof=google apple"`
	IDToken     string `json:"idToken" binding:"required"`
	DeviceToken string `json:"deviceToken"`
}

type AuthResponse struct {
	AccessToken  string       `json:"accessToken"`
	RefreshToken string       `json:"refreshToken"`
	ExpiresIn    int          `json:"expiresIn"`
	User         UserResponse `json:"user"`
}

type OAuthResponse struct {
	AccessToken  string       `json:"accessToken"`
	RefreshToken string       `json:"refreshToken"`
	ExpiresIn    int          `json:"expiresIn"`
	User         UserResponse `json:"user"`
	IsNewUser    bool         `json:"isNewUser"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=8"`
}

type UpdateProfileRequest struct {
	Name  *string `json:"name"`
	Phone *string `json:"phone"`
	Email *string `json:"email"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required"`
}

type ForgotPasswordResponse struct {
	Message   string `json:"message"`
	ExpiresIn int    `json:"expiresIn"`
}

type RequestEmailVerificationRequest struct {
	Email string `json:"email" binding:"required"`
}

type RequestEmailVerificationResponse struct {
	Message   string `json:"message"`
	ExpiresIn int    `json:"expiresIn"`
}

type VerifyEmailRequest struct {
	Email string `json:"email" binding:"required"`
	Code  string `json:"code" binding:"required,len=6,numeric"`
}

type VerifyResetCodeRequest struct {
	Email string `json:"email" binding:"required"`
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
	ExpiresIn    int    `json:"expiresIn"`
}

type UserResponse struct {
	ID            string    `json:"id"`
	Email         string    `json:"email"`
	Name          string    `json:"name"`
	Phone         *string   `json:"phone,omitempty"`
	Role          string    `json:"role"`
	IsVerified    bool      `json:"isVerified"`
	IsBlocked     bool      `json:"isBlocked"`
	AccountStatus string    `json:"accountStatus"`
	AuthProvider  string    `json:"authProvider"`
	PartnerStatus string    `json:"partnerStatus,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}
