package auth

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// Register godoc
// @Summary Регистрация USER
// @Tags Auth
// @Accept json
// @Produce json
// @Param input body RegisterRequest true "Данные для регистрации"
// @Success 201 {object} TokenResponse
// @Failure 409 {object} map[string]string
// @Router /auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var input RegisterRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.service.Register(input.Email, input.Password, input.Name, input.DeviceToken)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register"})
		return
	}

	c.JSON(http.StatusCreated, TokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// RegisterPartner godoc
// @Summary Регистрация PARTNER (заявка)
// @Tags Auth
// @Accept json
// @Produce json
// @Param input body PartnerRegisterRequest true "Данные заявки партнера"
// @Success 201 {object} TokenResponse
// @Failure 409 {object} map[string]string
// @Router /auth/register/partner [post]
func (h *Handler) RegisterPartner(c *gin.Context) {
	var input PartnerRegisterRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.service.RegisterPartner(input)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register partner"})
		return
	}

	c.JSON(http.StatusCreated, TokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// Login godoc
// @Summary Вход (USER/PARTNER)
// @Tags Auth
// @Accept json
// @Produce json
// @Param input body LoginRequest true "Данные для входа"
// @Success 200 {object} TokenResponse
// @Failure 401 {object} map[string]string
// @Router /auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var input LoginRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.service.Login(input.Email, input.Password, input.DeviceToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// OAuth godoc
// @Summary OAuth вход (Google/Apple) — только USER
// @Tags Auth
// @Accept json
// @Produce json
// @Param input body OAuthRequest true "OAuth provider и idToken"
// @Success 200 {object} OAuthResponse
// @Success 201 {object} OAuthResponse
// @Failure 409 {object} map[string]string
// @Router /auth/oauth [post]
func (h *Handler) OAuth(c *gin.Context) {
	var input OAuthRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, isNewUser, err := h.service.OAuthLogin(input.Provider, input.IDToken, input.DeviceToken)
	if err != nil {
		switch {
		case errors.Is(err, ErrAuthProviderConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "Email already linked to another auth provider"})
		case errors.Is(err, ErrInvalidOAuthProvider):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OAuth provider"})
		case errors.Is(err, ErrInvalidOAuthToken):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OAuth token"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to login via OAuth"})
		}
		return
	}

	status := http.StatusOK
	if isNewUser {
		status = http.StatusCreated
	}

	c.JSON(status, OAuthResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		IsNewUser:    isNewUser,
	})
}

// Me godoc
// @Summary Текущий пользователь
// @Tags Auth
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} UserResponse
// @Router /auth/me [get]
func (h *Handler) Me(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	idStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	user, err := h.service.GetUserByID(idStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, UserResponse{
		ID:            user.ID.String(),
		Email:         user.Email,
		Name:          user.Name,
		Role:          user.Role,
		AuthProvider:  user.AuthProvider,
		PartnerStatus: user.PartnerStatus,
		CreatedAt:     user.CreatedAt,
	})
}

// DeleteAccount godoc
// @Summary Удаление аккаунта (GDPR)
// @Tags Users
// @Security ApiKeyAuth
// @Accept json
// @Param input body DeleteAccountRequest false "Пароль (только для email-аккаунтов)"
// @Success 204
// @Failure 400 {object} map[string]string
// @Router /users/me [delete]
func (h *Handler) DeleteAccount(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	var input DeleteAccountRequest
	if err := c.ShouldBindJSON(&input); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.DeleteAccount(userIDStr, input.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrActiveOrdersExist):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Active orders exist"})
		case errors.Is(err, ErrPasswordRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password is required for email accounts"})
		case errors.Is(err, ErrInvalidCurrentPassword):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid password"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account"})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// ChangePassword godoc
// @Summary Смена пароля (авторизован)
// @Tags Auth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param input body ChangePasswordRequest true "Текущий и новый пароль"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /auth/change-password [post]
func (h *Handler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	userIDStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	var input ChangePasswordRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.ChangePassword(userIDStr, input.CurrentPassword, input.NewPassword)
	if err != nil {
		if errors.Is(err, ErrInvalidCurrentPassword) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid current password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// ForgotPassword godoc
// @Summary Запрос OTP для сброса пароля
// @Tags Auth
// @Accept json
// @Produce json
// @Param input body ForgotPasswordRequest true "Email для сброса"
// @Success 200 {object} ForgotPasswordResponse
// @Router /auth/forgot-password [post]
func (h *Handler) ForgotPassword(c *gin.Context) {
	var input ForgotPasswordRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	expiresIn, err := h.service.ForgotPassword(input.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process forgot password"})
		return
	}

	c.JSON(http.StatusOK, ForgotPasswordResponse{
		Message:   "OTP sent if email exists",
		ExpiresIn: expiresIn,
	})
}

// VerifyResetCode godoc
// @Summary Проверка OTP-кода
// @Tags Auth
// @Accept json
// @Produce json
// @Param input body VerifyResetCodeRequest true "Email и OTP-код"
// @Success 200 {object} VerifyResetCodeResponse
// @Failure 400 {object} map[string]string
// @Router /auth/verify-reset-code [post]
func (h *Handler) VerifyResetCode(c *gin.Context) {
	var input VerifyResetCodeRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resetToken, err := h.service.VerifyResetCode(input.Email, input.Code)
	if err != nil {
		if errors.Is(err, ErrInvalidResetCode) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reset code"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify reset code"})
		return
	}

	c.JSON(http.StatusOK, VerifyResetCodeResponse{ResetToken: resetToken})
}

// ResetPassword godoc
// @Summary Установка нового пароля
// @Tags Auth
// @Accept json
// @Produce json
// @Param input body ResetPasswordRequest true "Reset token и новый пароль"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /auth/reset-password [post]
func (h *Handler) ResetPassword(c *gin.Context) {
	var input ResetPasswordRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.ResetPassword(input.ResetToken, input.NewPassword)
	if err != nil {
		if errors.Is(err, ErrInvalidResetToken) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reset token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

// Refresh godoc
// @Summary Обновление токенов
// @Tags Auth
// @Accept json
// @Produce json
// @Param input body RefreshRequest true "Refresh token"
// @Success 200 {object} TokenResponse
// @Failure 401 {object} map[string]string
// @Router /auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var input RefreshRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(input.RefreshToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refreshToken is required"})
		return
	}

	tokens, err := h.service.RefreshTokens(input.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// Logout godoc
// @Summary Выход (инвалидация refresh token)
// @Security ApiKeyAuth
// @Tags Auth
// @Accept json
// @Param input body RefreshRequest true "Refresh token"
// @Success 204
// @Failure 400 {object} map[string]string
// @Router /auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	var input RefreshRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(input.RefreshToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "refreshToken is required"})
		return
	}

	if err := h.service.Logout(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.Status(http.StatusNoContent)
}
