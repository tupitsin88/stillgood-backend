package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"kursach_backend/internal/domain"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var ErrEmailAlreadyExists = errors.New("email already exists")
var ErrInvalidCurrentPassword = errors.New("invalid current password")
var ErrInvalidResetCode = errors.New("invalid reset code")
var ErrInvalidResetToken = errors.New("invalid reset token")
var ErrAuthProviderConflict = errors.New("email is linked to another auth provider")
var ErrInvalidOAuthToken = errors.New("invalid oauth token")
var ErrInvalidOAuthProvider = errors.New("invalid oauth provider")
var ErrActiveOrdersExist = errors.New("active orders exist")
var ErrPasswordRequired = errors.New("password required")
var ErrInvalidRefreshToken = errors.New("invalid refresh token")

const (
	passwordResetCodeTTL  = 10 * time.Minute
	passwordResetTokenTTL = 15 * time.Minute
)

type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
}

type Service interface {
	Register(email, password, name, deviceToken string) (Tokens, *domain.User, error)
	RegisterPartner(input PartnerRegisterRequest) (Tokens, *domain.User, error)
	Login(email, password, deviceToken string) (Tokens, *domain.User, error)
	OAuthLogin(provider, idToken, deviceToken string) (Tokens, *domain.User, bool, error)
	RefreshTokens(refreshToken string) (Tokens, error)
	ChangePassword(userID, currentPassword, newPassword string) error
	ForgotPassword(email string) (int, error)
	VerifyResetCode(email, code string) (string, error)
	ResetPassword(resetToken, newPassword string) error
	DeleteAccount(userID, password string) error
	Logout(refreshToken string) error
	GetUserByID(id string) (*domain.User, error)
}

type resetCodeEntry struct {
	Code      string
	ExpiresAt time.Time
}

type resetTokenEntry struct {
	Email     string
	ExpiresAt time.Time
}

type service struct {
	repo         Repository
	tokenManager *TokenManager
	accessTTL    time.Duration
	refreshTTL   time.Duration

	mu          sync.Mutex
	resetCodes  map[string]resetCodeEntry
	resetTokens map[string]resetTokenEntry
	revokedRT   map[string]int64
}

func NewService(repo Repository, tokenManager *TokenManager, accessTTL, refreshTTL time.Duration) Service {
	return &service{
		repo:         repo,
		tokenManager: tokenManager,
		accessTTL:    accessTTL,
		refreshTTL:   refreshTTL,
		resetCodes:   make(map[string]resetCodeEntry),
		resetTokens:  make(map[string]resetTokenEntry),
		revokedRT:    make(map[string]int64),
	}
}

func (s *service) Register(email, password, name, deviceToken string) (Tokens, *domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	exists, err := s.repo.ExistsByEmail(email)
	if err != nil {
		return Tokens{}, nil, err
	}
	if exists {
		return Tokens{}, nil, ErrEmailAlreadyExists
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Tokens{}, nil, err
	}

	user := &domain.User{
		Email:        email,
		PasswordHash: string(hashedPass),
		Name:         name,
		Role:         "USER",
		AuthProvider: "email",
	}

	if deviceToken != "" {
		user.DeviceToken = &deviceToken
	}

	if err = s.repo.CreateUser(user); err != nil {
		return Tokens{}, nil, err
	}

	restID := ""
	if user.RestaurantID != nil {
		restID = user.RestaurantID.String()
	}

	tokens, err := s.generateTokens(user.ID.String(), user.Role, restID)
	if err != nil {
		return Tokens{}, nil, err
	}
	return tokens, user, nil
}

func (s *service) RegisterPartner(input PartnerRegisterRequest) (Tokens, *domain.User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	exists, err := s.repo.ExistsByEmail(email)
	if err != nil {
		return Tokens{}, nil, err
	}
	if exists {
		return Tokens{}, nil, ErrEmailAlreadyExists
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return Tokens{}, nil, err
	}

	user := &domain.User{
		Email:         email,
		PasswordHash:  string(hashedPass),
		Name:          input.Name,
		Role:          "PARTNER",
		PartnerStatus: "PENDING",
		AuthProvider:  "email",
	}

	if input.DeviceToken != "" {
		user.DeviceToken = &input.DeviceToken
	}

	if err = s.repo.CreateUser(user); err != nil {
		return Tokens{}, nil, err
	}

	restID := ""
	if user.RestaurantID != nil {
		restID = user.RestaurantID.String()
	}

	tokens, err := s.generateTokens(user.ID.String(), user.Role, restID)
	if err != nil {
		return Tokens{}, nil, err
	}
	return tokens, user, nil
}

func (s *service) Login(email, password, deviceToken string) (Tokens, *domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		return Tokens{}, nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return Tokens{}, nil, err
	}

	// Update Device Token if provided
	if deviceToken != "" {
		if err := s.repo.UpdateDeviceToken(user.ID, deviceToken); err != nil {
			// Log error but don't fail login? Or fail? Usually, we just log it.
			// For simplicity nicely, we can ignore it or return error. Let's return error for now to be safe.
			return Tokens{}, nil, err
		}
	}
	restID := ""
	if user.RestaurantID != nil {
		restID = user.RestaurantID.String()
	}

	tokens, err := s.generateTokens(user.ID.String(), user.Role, restID)
	if err != nil {
		return Tokens{}, nil, err
	}
	return tokens, user, nil
}

func (s *service) OAuthLogin(provider, idToken, deviceToken string) (Tokens, *domain.User, bool, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "google" && provider != "apple" {
		return Tokens{}, nil, false, ErrInvalidOAuthProvider
	}

	email, name, err := extractOAuthIdentity(idToken)
	if err != nil {
		return Tokens{}, nil, false, err
	}

	exists, err := s.repo.ExistsByEmail(email)
	if err != nil {
		return Tokens{}, nil, false, err
	}

	if exists {
		user, err := s.repo.GetUserByEmail(email)
		if err != nil {
			return Tokens{}, nil, false, err
		}

		if user.AuthProvider != provider || user.Role != "USER" {
			return Tokens{}, nil, false, ErrAuthProviderConflict
		}

		if deviceToken != "" {
			if err := s.repo.UpdateDeviceToken(user.ID, deviceToken); err != nil {
				return Tokens{}, nil, false, err
			}
		}

		restID := ""
		if user.RestaurantID != nil {
			restID = user.RestaurantID.String()
		}

		tokens, err := s.generateTokens(user.ID.String(), user.Role, restID)
		return tokens, user, false, err
	}

	user := &domain.User{
		Email:        email,
		Name:         name,
		Role:         "USER",
		AuthProvider: provider,
	}
	if deviceToken != "" {
		user.DeviceToken = &deviceToken
	}

	if err := s.repo.CreateUser(user); err != nil {
		return Tokens{}, nil, false, err
	}

	restID := ""
	if user.RestaurantID != nil {
		restID = user.RestaurantID.String()
	}

	tokens, err := s.generateTokens(user.ID.String(), user.Role, restID)
	return tokens, user, true, err
}

func (s *service) RefreshTokens(refreshToken string) (Tokens, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return Tokens{}, ErrInvalidRefreshToken
	}

	if s.isRefreshTokenRevoked(refreshToken) {
		return Tokens{}, ErrInvalidRefreshToken
	}

	claims, err := s.tokenManager.Parse(refreshToken)
	if err != nil {
		return Tokens{}, ErrInvalidRefreshToken
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return Tokens{}, ErrInvalidRefreshToken
	}

	// Security check: Verify user exists and is active in DB
	user, err := s.GetUserByID(sub)
	if err != nil {
		return Tokens{}, err // User probably deleted or ID changed
	}
	restID := ""
	if user.RestaurantID != nil {
		restID = user.RestaurantID.String()
	}

	return s.generateTokens(user.ID.String(), user.Role, restID)
}

func (s *service) ChangePassword(userID, currentPassword, newPassword string) error {
	uuidID, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return err
	}

	user, err := s.repo.GetByID(uuidID)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrInvalidCurrentPassword
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.repo.UpdatePasswordHash(uuidID, string(hashedPass))
}

func (s *service) ForgotPassword(email string) (int, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	exists, err := s.repo.ExistsByEmail(email)
	if err != nil {
		return 0, err
	}

	// Всегда возвращаем одинаковый ответ, чтобы не раскрывать наличие email.
	if !exists {
		return int(passwordResetCodeTTL.Seconds()), nil
	}

	code, err := generateSixDigitCode()
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	s.resetCodes[email] = resetCodeEntry{
		Code:      code,
		ExpiresAt: time.Now().Add(passwordResetCodeTTL),
	}
	s.mu.Unlock()

	// Для MVP выводим OTP в лог до интеграции email/SMS.
	log.Printf("Password reset code for %s: %s", email, code)

	return int(passwordResetCodeTTL.Seconds()), nil
}

func (s *service) VerifyResetCode(email, code string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.resetCodes[email]
	if !ok {
		return "", ErrInvalidResetCode
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.resetCodes, email)
		return "", ErrInvalidResetCode
	}
	if entry.Code != code {
		return "", ErrInvalidResetCode
	}

	resetToken := uuid.NewString()
	s.resetTokens[resetToken] = resetTokenEntry{
		Email:     email,
		ExpiresAt: time.Now().Add(passwordResetTokenTTL),
	}
	delete(s.resetCodes, email)

	return resetToken, nil
}

func (s *service) ResetPassword(resetToken, newPassword string) error {
	resetToken = strings.TrimSpace(resetToken)

	s.mu.Lock()
	entry, ok := s.resetTokens[resetToken]
	if !ok {
		s.mu.Unlock()
		return ErrInvalidResetToken
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.resetTokens, resetToken)
		s.mu.Unlock()
		return ErrInvalidResetToken
	}
	delete(s.resetTokens, resetToken)
	s.mu.Unlock()

	user, err := s.repo.GetUserByEmail(entry.Email)
	if err != nil {
		return ErrInvalidResetToken
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.repo.UpdatePasswordHash(user.ID, string(hashedPass))
}

func (s *service) DeleteAccount(userID, password string) error {
	uid, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return err
	}

	user, err := s.repo.GetByID(uid)
	if err != nil {
		return err
	}

	// Для email-аккаунтов требуем пароль, чтобы подтвердить владение аккаунтом.
	if user.AuthProvider == "email" {
		if strings.TrimSpace(password) == "" {
			return ErrPasswordRequired
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
			return ErrInvalidCurrentPassword
		}
	}

	activeOrders, err := s.repo.CountActiveOrdersByUserID(uid)
	if err != nil {
		return err
	}
	if activeOrders > 0 {
		return ErrActiveOrdersExist
	}

	return s.repo.DeleteAccount(uid)
}

func (s *service) Logout(refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return ErrInvalidRefreshToken
	}

	claims, err := s.tokenManager.Parse(refreshToken)
	if err != nil {
		return ErrInvalidRefreshToken
	}

	expUnix, err := extractExpUnix(claims)
	if err != nil {
		return ErrInvalidRefreshToken
	}

	s.mu.Lock()
	s.cleanupRevokedTokensLocked(time.Now().Unix())
	s.revokedRT[refreshToken] = expUnix
	s.mu.Unlock()

	return nil
}

func (s *service) GetUserByID(id string) (*domain.User, error) {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	return s.repo.GetByID(uuidID)
}

func (s *service) generateTokens(userID, role, restaurantID string) (Tokens, error) {
	accessToken, err := s.tokenManager.NewAccessToken(userID, role, restaurantID, s.accessTTL)
	if err != nil {
		return Tokens{}, err
	}

	refreshToken, err := s.tokenManager.NewRefreshToken(userID, s.refreshTTL)
	if err != nil {
		return Tokens{}, err
	}

	return Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(s.accessTTL.Seconds()),
	}, nil
}

func generateSixDigitCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func extractExpUnix(claims map[string]interface{}) (int64, error) {
	rawExp, ok := claims["exp"]
	if !ok {
		return 0, ErrInvalidRefreshToken
	}

	switch v := rawExp.(type) {
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	default:
		return 0, ErrInvalidRefreshToken
	}
}

func (s *service) isRefreshTokenRevoked(refreshToken string) bool {
	nowUnix := time.Now().Unix()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupRevokedTokensLocked(nowUnix)
	exp, exists := s.revokedRT[refreshToken]
	return exists && exp > nowUnix
}

func (s *service) cleanupRevokedTokensLocked(nowUnix int64) {
	for token, exp := range s.revokedRT {
		if exp <= nowUnix {
			delete(s.revokedRT, token)
		}
	}
}

func extractOAuthIdentity(idToken string) (string, string, error) {
	idToken = strings.TrimSpace(idToken)
	if idToken == "" {
		return "", "", ErrInvalidOAuthToken
	}

	// MVP-режим: для локальных тестов разрешаем передавать email напрямую в idToken.
	if strings.Contains(idToken, "@") && !strings.Contains(idToken, " ") {
		email := strings.ToLower(strings.TrimSpace(idToken))
		name := strings.Split(email, "@")[0]
		return email, name, nil
	}

	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", "", ErrInvalidOAuthToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", ErrInvalidOAuthToken
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", "", ErrInvalidOAuthToken
	}

	emailRaw, ok := claims["email"].(string)
	if !ok || strings.TrimSpace(emailRaw) == "" {
		return "", "", ErrInvalidOAuthToken
	}
	email := strings.ToLower(strings.TrimSpace(emailRaw))

	name := strings.Split(email, "@")[0]
	if claimName, ok := claims["name"].(string); ok && strings.TrimSpace(claimName) != "" {
		name = strings.TrimSpace(claimName)
	}

	return email, name, nil
}
