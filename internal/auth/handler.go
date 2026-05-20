package auth

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"kursach_backend/internal/domain"
	"kursach_backend/internal/pkg/httputil"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func accountStatusFromUser(user *domain.User) string {
	if user.DeletedAt != nil {
		return "deleted"
	}
	if user.IsBlocked {
		return "blocked"
	}
	return "active"
}

func toUserResponse(user *domain.User) UserResponse {
	return UserResponse{
		ID:            user.ID.String(),
		Email:         user.Email,
		Name:          user.Name,
		Phone:         user.Phone,
		Role:          user.Role,
		IsVerified:    user.IsVerified,
		IsBlocked:     user.IsBlocked,
		AccountStatus: accountStatusFromUser(user),
		AuthProvider:  user.AuthProvider,
		PartnerStatus: user.PartnerStatus,
		CreatedAt:     user.CreatedAt,
	}
}

func (h *Handler) requireAdmin(c *gin.Context) bool {
	if c.GetString("role") != RoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "FORBIDDEN", "message": "Admin role required"})
		return false
	}
	return true
}

func (h *Handler) Register(c *gin.Context) {
	var input RegisterRequest

	if !bindAuthJSON(c, &input) {
		return
	}

	tokens, user, err := h.service.Register(input.Email, input.Password, input.Name, input.DeviceToken)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		if errors.Is(err, ErrInvalidEmail) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format (RFC 5322 expected)"})
			return
		}
		if errors.Is(err, ErrWeakPassword) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters and include a digit and a special character"})
			return
		}
		if errors.Is(err, ErrDeviceTokenRequired) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "DEVICE_TOKEN_REQUIRED", "message": "deviceToken is required"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register"})
		return
	}

	responseUser := toUserResponse(user)

	c.JSON(http.StatusCreated, AuthResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		User:         responseUser,
	})
}

func (h *Handler) RegisterPartner(c *gin.Context) {
	var input PartnerRegisterRequest

	if !bindAuthJSON(c, &input) {
		return
	}

	tokens, user, err := h.service.RegisterPartner(input)
	if err != nil {
		if errors.Is(err, ErrEmailAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
		if errors.Is(err, ErrInvalidEmail) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format (RFC 5322 expected)"})
			return
		}
		if errors.Is(err, ErrWeakPassword) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters and include a digit and a special character"})
			return
		}
		if errors.Is(err, ErrDeviceTokenRequired) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "DEVICE_TOKEN_REQUIRED", "message": "deviceToken is required"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register partner"})
		return
	}

	responseUser := toUserResponse(user)

	c.JSON(http.StatusCreated, AuthResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		User:         responseUser,
	})
}

func (h *Handler) Login(c *gin.Context) {
	var input LoginRequest

	if !bindAuthJSON(c, &input) {
		return
	}

	tokens, user, err := h.service.Login(input.Email, input.Password, input.DeviceToken)
	if err != nil {
		if errors.Is(err, ErrInvalidEmail) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format (RFC 5322 expected)"})
			return
		}
		if errors.Is(err, ErrDeviceTokenRequired) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "DEVICE_TOKEN_REQUIRED", "message": "deviceToken is required"})
			return
		}
		if errors.Is(err, ErrUserBlocked) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Account is blocked"})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	responseUser := toUserResponse(user)

	c.JSON(http.StatusOK, AuthResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		User:         responseUser,
	})
}

func (h *Handler) OAuth(c *gin.Context) {
	var input OAuthRequest
	if !bindAuthJSON(c, &input) {
		return
	}

	tokens, user, isNewUser, err := h.service.OAuthLogin(input.Provider, input.IDToken, input.DeviceToken)
	if err != nil {
		switch {
		case errors.Is(err, ErrAuthProviderConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "Email already linked to another auth provider"})
		case errors.Is(err, ErrInvalidOAuthProvider):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OAuth provider"})
		case errors.Is(err, ErrInvalidOAuthToken):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OAuth token"})
		case errors.Is(err, ErrDeviceTokenRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": "DEVICE_TOKEN_REQUIRED", "message": "deviceToken is required"})
		case errors.Is(err, ErrUserBlocked):
			c.JSON(http.StatusForbidden, gin.H{"error": "Account is blocked"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to login via OAuth"})
		}
		return
	}

	status := http.StatusOK
	if isNewUser {
		status = http.StatusCreated
	}

	responseUser := toUserResponse(user)

	c.JSON(status, OAuthResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		User:         responseUser,
		IsNewUser:    isNewUser,
	})
}

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

	c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *Handler) UpdateProfile(c *gin.Context) {
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

	var input UpdateProfileRequest
	if !bindAuthJSON(c, &input) {
		return
	}

	user, err := h.service.UpdateProfile(userIDStr, input.Name, input.Phone, input.Email)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyProfileUpdate):
			c.JSON(http.StatusBadRequest, gin.H{"error": "At least one of name, phone, email must be provided"})
		case errors.Is(err, ErrInvalidName):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Name must not be empty"})
		case errors.Is(err, ErrInvalidEmail):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format (RFC 5322 expected)"})
		case errors.Is(err, ErrEmailAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
		case errors.Is(err, ErrEmailChangeNotAllowed):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email change is allowed only for email auth provider"})
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		}
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

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
		code, message := httputil.BindingError(err, &input)
		c.JSON(http.StatusBadRequest, gin.H{"error": code, "message": message})
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

func (h *Handler) UpdateDeviceToken(c *gin.Context) {
	userID := c.GetString("user_id")
	if strings.TrimSpace(userID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "message": "Invalid or missing User ID"})
		return
	}

	var input UpdateDeviceTokenRequest
	if !bindAuthJSON(c, &input) {
		return
	}

	if err := h.service.UpdateDeviceToken(userID, input.DeviceToken, input.Platform); err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "UNAUTHORIZED", "message": "Invalid or missing User ID"})
		case errors.Is(err, ErrDeviceTokenRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": "DEVICE_TOKEN_REQUIRED", "message": "deviceToken is required"})
		case errors.Is(err, ErrInvalidDevicePlatform):
			c.JSON(http.StatusBadRequest, gin.H{"error": "INVALID_PLATFORM", "message": "platform must be one of: android ios"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "DEVICE_TOKEN_UPDATE_FAILED", "message": "Failed to update device token"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Device token updated"})
}

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
	if !bindAuthJSON(c, &input) {
		return
	}

	err := h.service.ChangePassword(userIDStr, input.CurrentPassword, input.NewPassword)
	if err != nil {
		if errors.Is(err, ErrInvalidCurrentPassword) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid current password"})
			return
		}
		if errors.Is(err, ErrWeakPassword) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters and include a digit and a special character"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

func (h *Handler) RequestEmailVerification(c *gin.Context) {
	var input RequestEmailVerificationRequest
	if !bindAuthJSON(c, &input) {
		return
	}

	expiresIn, err := h.service.RequestEmailVerification(input.Email)
	if err != nil {
		if errors.Is(err, ErrVerificationCodeTooFrequent) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "TOO_MANY_REQUESTS",
				"message": "Please wait at least 1 minute before requesting a new code",
			})
			return
		}
		if errors.Is(err, ErrInvalidEmail) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format (RFC 5322 expected)"})
			return
		}
		if errors.Is(err, ErrEmailDeliveryFailed) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "EMAIL_DELIVERY_FAILED",
				"message": "Failed to send verification code",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to request email verification"})
		return
	}

	c.JSON(http.StatusOK, RequestEmailVerificationResponse{
		Message:   "Verification code sent if email exists",
		ExpiresIn: expiresIn,
	})
}

func (h *Handler) VerifyEmail(c *gin.Context) {
	var input VerifyEmailRequest
	if !bindAuthJSON(c, &input) {
		return
	}

	if err := h.service.VerifyEmail(input.Email, input.Code); err != nil {
		if errors.Is(err, ErrInvalidEmail) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format (RFC 5322 expected)"})
			return
		}
		if errors.Is(err, ErrInvalidVerificationCode) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid verification code"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully"})
}

func (h *Handler) ForgotPassword(c *gin.Context) {
	var input ForgotPasswordRequest
	if !bindAuthJSON(c, &input) {
		return
	}

	expiresIn, err := h.service.ForgotPassword(input.Email)
	if err != nil {
		if errors.Is(err, ErrInvalidEmail) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format (RFC 5322 expected)"})
			return
		}
		if errors.Is(err, ErrEmailDeliveryFailed) {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "EMAIL_DELIVERY_FAILED",
				"message": "Failed to send password reset code",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process forgot password"})
		return
	}

	c.JSON(http.StatusOK, ForgotPasswordResponse{
		Message:   "OTP sent if email exists",
		ExpiresIn: expiresIn,
	})
}

func (h *Handler) VerifyResetCode(c *gin.Context) {
	var input VerifyResetCodeRequest
	if !bindAuthJSON(c, &input) {
		return
	}

	resetToken, err := h.service.VerifyResetCode(input.Email, input.Code)
	if err != nil {
		if errors.Is(err, ErrInvalidEmail) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format (RFC 5322 expected)"})
			return
		}
		if errors.Is(err, ErrInvalidResetCode) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reset code"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify reset code"})
		return
	}

	c.JSON(http.StatusOK, VerifyResetCodeResponse{ResetToken: resetToken})
}

func (h *Handler) ResetPassword(c *gin.Context) {
	var input ResetPasswordRequest
	if !bindAuthJSON(c, &input) {
		return
	}

	err := h.service.ResetPassword(input.ResetToken, input.NewPassword)
	if err != nil {
		if errors.Is(err, ErrInvalidResetToken) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reset token"})
			return
		}
		if errors.Is(err, ErrWeakPassword) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters and include a digit and a special character"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset successfully"})
}

func (h *Handler) Refresh(c *gin.Context) {
	var input RefreshRequest

	if !bindAuthJSON(c, &input) {
		return
	}
	if strings.TrimSpace(input.RefreshToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "REFRESH_TOKEN_REQUIRED", "message": "refreshToken is required"})
		return
	}

	tokens, err := h.service.RefreshTokens(input.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrUserBlocked) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Account is blocked"})
			return
		}
		if errors.Is(err, ErrInvalidRefreshToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh tokens"})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	})
}

func (h *Handler) GetUsers(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
		return
	}
	if limit > 100 {
		limit = 100
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset"})
		return
	}

	users, total, err := h.service.ListUsers(limit, offset, c.Query("role"), c.Query("q"))
	if err != nil {
		if errors.Is(err, ErrInvalidUserRoleFilter) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role filter. Use USER or PARTNER"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	response := make([]UserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, toUserResponse(user))
	}

	c.JSON(http.StatusOK, gin.H{
		"data": response,
		"pagination": gin.H{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

func (h *Handler) Logout(c *gin.Context) {
	var input RefreshRequest
	if !bindAuthJSON(c, &input) {
		return
	}
	if strings.TrimSpace(input.RefreshToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "REFRESH_TOKEN_REQUIRED", "message": "refreshToken is required"})
		return
	}

	if err := h.service.Logout(input.RefreshToken); err != nil {
		if errors.Is(err, ErrInvalidRefreshToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) GetPendingPartners(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit"})
		return
	}
	if limit > 100 {
		limit = 100
	}
	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid offset"})
		return
	}

	partners, total, err := h.service.ListPendingPartners(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch pending partners"})
		return
	}

	response := make([]UserResponse, 0, len(partners))
	for _, user := range partners {
		response = append(response, toUserResponse(user))
	}

	c.JSON(http.StatusOK, gin.H{
		"data": response,
		"pagination": gin.H{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

func (h *Handler) ApprovePartner(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	user, err := h.service.ApprovePartner(c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, ErrPartnerNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Partner not found"})
		case errors.Is(err, ErrInvalidPartnerStatusTransition):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Only PENDING partner can be approved or rejected"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve partner"})
		}
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *Handler) RejectPartner(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	user, err := h.service.RejectPartner(c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, ErrPartnerNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "Partner not found"})
		case errors.Is(err, ErrInvalidPartnerStatusTransition):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Only PENDING partner can be approved or rejected"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject partner"})
		}
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *Handler) BlockUser(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	user, err := h.service.BlockUser(c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		case errors.Is(err, ErrCannotBlockAdmin):
			c.JSON(http.StatusBadRequest, gin.H{"error": "ADMIN users cannot be blocked"})
		case errors.Is(err, ErrDeletedAccount):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Deleted accounts cannot be blocked"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to block user"})
		}
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *Handler) UnblockUser(c *gin.Context) {
	if !h.requireAdmin(c) {
		return
	}

	user, err := h.service.UnblockUser(c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		case errors.Is(err, ErrCannotBlockAdmin):
			c.JSON(http.StatusBadRequest, gin.H{"error": "ADMIN users cannot be unblocked"})
		case errors.Is(err, ErrDeletedAccount):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Deleted accounts cannot be unblocked"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unblock user"})
		}
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}
