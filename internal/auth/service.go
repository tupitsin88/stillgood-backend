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
	"net/mail"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var ErrEmailAlreadyExists = errors.New("email already exists")
var ErrInvalidCurrentPassword = errors.New("invalid current password")
var ErrInvalidResetCode = errors.New("invalid reset code")
var ErrInvalidResetToken = errors.New("invalid reset token")
var ErrAuthProviderConflict = errors.New("email is linked to another auth provider")
var ErrInvalidOAuthToken = errors.New("invalid oauth token")
var ErrInvalidOAuthProvider = errors.New("invalid oauth provider")
var ErrInvalidEmail = errors.New("invalid email")
var ErrPartnerNotFound = errors.New("partner not found")
var ErrInvalidPartnerStatusTransition = errors.New("invalid partner status transition")
var ErrActiveOrdersExist = errors.New("active orders exist")
var ErrPasswordRequired = errors.New("password required")
var ErrWeakPassword = errors.New("password must be at least 8 characters and include a digit and a special character")
var ErrInvalidRefreshToken = errors.New("invalid refresh token")
var ErrDeviceTokenRequired = errors.New("device token is required")
var ErrUserNotFound = errors.New("user not found")
var ErrInvalidUserRoleFilter = errors.New("invalid user role filter")
var ErrCannotBlockAdmin = errors.New("cannot block admin user")
var ErrDeletedAccount = errors.New("account is deleted")
var ErrUserBlocked = errors.New("user is blocked")
var ErrInvalidVerificationCode = errors.New("invalid verification code")
var ErrEmailChangeNotAllowed = errors.New("email change is allowed only for email auth provider")
var ErrInvalidName = errors.New("invalid name")
var ErrEmptyProfileUpdate = errors.New("at least one profile field must be provided")

const (
	emailVerificationCodeTTL = 10 * time.Minute
	passwordResetCodeTTL     = 10 * time.Minute
	passwordResetTokenTTL    = 15 * time.Minute
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
	UpdateProfile(userID string, name, phone, email *string) (*domain.User, error)
	ChangeEmail(userID, newEmail string) (*domain.User, error)
	ListUsers(limit, offset int, roleFilter string, search string) ([]*domain.User, int64, error)
	BlockUser(userID string) (*domain.User, error)
	UnblockUser(userID string) (*domain.User, error)
	ListPendingPartners(limit, offset int) ([]*domain.User, int64, error)
	ApprovePartner(partnerID string) (*domain.User, error)
	RejectPartner(partnerID string) (*domain.User, error)
	ForgotPassword(email string) (int, error)
	RequestEmailVerification(email string) (int, error)
	VerifyEmail(email, code string) error
	VerifyResetCode(email, code string) (string, error)
	ResetPassword(resetToken, newPassword string) error
	DeleteAccount(userID, password string) error
	Logout(refreshToken string) error
	GetUserByID(id string) (*domain.User, error)
	IsUserBlocked(id string) (bool, error)
}

type resetCodeEntry struct {
	Code      string
	ExpiresAt time.Time
}

type resetTokenEntry struct {
	Email     string
	ExpiresAt time.Time
}

type emailVerificationCodeEntry struct {
	Code      string
	ExpiresAt time.Time
}

type service struct {
	repo         Repository
	tokenManager *TokenManager
	accessTTL    time.Duration
	refreshTTL   time.Duration

	mu                     sync.Mutex
	resetCodes             map[string]resetCodeEntry
	emailVerificationCodes map[string]emailVerificationCodeEntry
	resetTokens            map[string]resetTokenEntry
	revokedRT              map[string]int64
}

func NewService(repo Repository, tokenManager *TokenManager, accessTTL, refreshTTL time.Duration) Service {
	return &service{
		repo:                   repo,
		tokenManager:           tokenManager,
		accessTTL:              accessTTL,
		refreshTTL:             refreshTTL,
		resetCodes:             make(map[string]resetCodeEntry),
		emailVerificationCodes: make(map[string]emailVerificationCodeEntry),
		resetTokens:            make(map[string]resetTokenEntry),
		revokedRT:              make(map[string]int64),
	}
}

func (s *service) Register(email, password, name, deviceToken string) (Tokens, *domain.User, error) {
	var err error
	email, err = normalizeAndValidateEmail(email)
	if err != nil {
		return Tokens{}, nil, err
	}
	deviceToken = strings.TrimSpace(deviceToken)
	if deviceToken == "" {
		return Tokens{}, nil, ErrDeviceTokenRequired
	}

	exists, err := s.repo.ExistsByEmail(email)
	if err != nil {
		return Tokens{}, nil, err
	}
	if exists {
		return Tokens{}, nil, ErrEmailAlreadyExists
	}
	if err := validatePasswordComplexity(password); err != nil {
		return Tokens{}, nil, err
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Tokens{}, nil, err
	}

	user := &domain.User{
		Email:        email,
		PasswordHash: string(hashedPass),
		Name:         name,
		Role:         RoleUser,
		AuthProvider: "email",
	}
	user.DeviceToken = &deviceToken

	if err = s.repo.CreateUser(user); err != nil {
		return Tokens{}, nil, err
	}
	if _, err := s.RequestEmailVerification(email); err != nil {
		log.Printf("failed to issue email verification code for %s: %v", email, err)
	}

	restID := ""
	if user.RestaurantID != nil {
		restID = user.RestaurantID.String()
	}

	tokens, err := s.generateTokens(user.ID.String(), user.Role, restID, user.PartnerStatus)
	if err != nil {
		return Tokens{}, nil, err
	}
	return tokens, user, nil
}

func (s *service) RegisterPartner(input PartnerRegisterRequest) (Tokens, *domain.User, error) {
	email, err := normalizeAndValidateEmail(input.Email)
	if err != nil {
		return Tokens{}, nil, err
	}
	input.DeviceToken = strings.TrimSpace(input.DeviceToken)
	if input.DeviceToken == "" {
		return Tokens{}, nil, ErrDeviceTokenRequired
	}

	exists, err := s.repo.ExistsByEmail(email)
	if err != nil {
		return Tokens{}, nil, err
	}
	if exists {
		return Tokens{}, nil, ErrEmailAlreadyExists
	}
	if err := validatePasswordComplexity(input.Password); err != nil {
		return Tokens{}, nil, err
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return Tokens{}, nil, err
	}

	user := &domain.User{
		Email:         email,
		PasswordHash:  string(hashedPass),
		Name:          input.Name,
		Role:          RolePartner,
		PartnerStatus: PartnerStatusPending,
		AuthProvider:  "email",
	}

	phone := strings.TrimSpace(input.Phone)
	if phone != "" {
		user.Phone = &phone
	}
	user.DeviceToken = &input.DeviceToken

	if err = s.repo.CreateUser(user); err != nil {
		return Tokens{}, nil, err
	}
	if _, err := s.RequestEmailVerification(email); err != nil {
		log.Printf("failed to issue email verification code for %s: %v", email, err)
	}

	restID := ""
	if user.RestaurantID != nil {
		restID = user.RestaurantID.String()
	}

	tokens, err := s.generateTokens(user.ID.String(), user.Role, restID, user.PartnerStatus)
	if err != nil {
		return Tokens{}, nil, err
	}
	return tokens, user, nil
}

func (s *service) Login(email, password, deviceToken string) (Tokens, *domain.User, error) {
	var err error
	email, err = normalizeAndValidateEmail(email)
	if err != nil {
		return Tokens{}, nil, err
	}

	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		return Tokens{}, nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return Tokens{}, nil, err
	}
	if user.IsBlocked {
		return Tokens{}, nil, ErrUserBlocked
	}

	deviceToken = strings.TrimSpace(deviceToken)
	if deviceToken == "" {
		return Tokens{}, nil, ErrDeviceTokenRequired
	}

	if err := s.repo.UpdateDeviceToken(user.ID, deviceToken); err != nil {
		return Tokens{}, nil, err
	}

	restID := ""
	if user.RestaurantID != nil {
		restID = user.RestaurantID.String()
	}

	tokens, err := s.generateTokens(user.ID.String(), user.Role, restID, user.PartnerStatus)
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
	deviceToken = strings.TrimSpace(deviceToken)
	if deviceToken == "" {
		return Tokens{}, nil, false, ErrDeviceTokenRequired
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

		if user.AuthProvider != provider || user.Role != RoleUser {
			return Tokens{}, nil, false, ErrAuthProviderConflict
		}
		if user.IsBlocked {
			return Tokens{}, nil, false, ErrUserBlocked
		}
		if err := s.repo.UpdateDeviceToken(user.ID, deviceToken); err != nil {
			return Tokens{}, nil, false, err
		}

		restID := ""
		if user.RestaurantID != nil {
			restID = user.RestaurantID.String()
		}

		tokens, err := s.generateTokens(user.ID.String(), user.Role, restID, user.PartnerStatus)
		return tokens, user, false, err
	}

	user := &domain.User{
		Email:        email,
		Name:         name,
		Role:         RoleUser,
		AuthProvider: provider,
	}
	user.DeviceToken = &deviceToken

	if err := s.repo.CreateUser(user); err != nil {
		return Tokens{}, nil, false, err
	}

	restID := ""
	if user.RestaurantID != nil {
		restID = user.RestaurantID.String()
	}

	tokens, err := s.generateTokens(user.ID.String(), user.Role, restID, user.PartnerStatus)
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
	if user.IsBlocked {
		return Tokens{}, ErrUserBlocked
	}
	restID := ""
	if user.RestaurantID != nil {
		restID = user.RestaurantID.String()
	}

	return s.generateTokens(user.ID.String(), user.Role, restID, user.PartnerStatus)
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
	if err := validatePasswordComplexity(newPassword); err != nil {
		return err
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.repo.UpdatePasswordHash(uuidID, string(hashedPass))
}

func (s *service) UpdateProfile(userID string, name, phone, email *string) (*domain.User, error) {
	uid, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return nil, ErrUserNotFound
	}

	user, err := s.repo.GetByID(uid)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if name == nil && phone == nil && email == nil {
		return nil, ErrEmptyProfileUpdate
	}

	if email != nil {
		trimmedEmail := strings.TrimSpace(*email)
		if trimmedEmail == "" {
			return nil, ErrInvalidEmail
		}
		if !strings.EqualFold(user.Email, trimmedEmail) {
			if _, err := s.ChangeEmail(userID, trimmedEmail); err != nil {
				return nil, err
			}
		}
	}

	if name != nil {
		trimmedName := strings.TrimSpace(*name)
		if trimmedName == "" {
			return nil, ErrInvalidName
		}
		if user.Name != trimmedName {
			if err := s.repo.UpdateName(uid, trimmedName); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, ErrUserNotFound
				}
				return nil, err
			}
		}
	}

	if phone != nil {
		trimmedPhone := strings.TrimSpace(*phone)
		var phoneValue *string
		if trimmedPhone != "" {
			phoneValue = &trimmedPhone
		}
		if !sameOptionalString(user.Phone, phoneValue) {
			if err := s.repo.UpdatePhone(uid, phoneValue); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil, ErrUserNotFound
				}
				return nil, err
			}
		}
	}

	return s.repo.GetByID(uid)
}

func (s *service) ChangeEmail(userID, newEmail string) (*domain.User, error) {
	uuidID, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return nil, ErrUserNotFound
	}

	normalizedEmail, err := normalizeAndValidateEmail(newEmail)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetByID(uuidID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if user.AuthProvider != "email" {
		return nil, ErrEmailChangeNotAllowed
	}

	if strings.EqualFold(user.Email, normalizedEmail) {
		return nil, ErrEmailAlreadyExists
	}

	exists, err := s.repo.ExistsByEmail(normalizedEmail)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrEmailAlreadyExists
	}

	if err := s.repo.UpdateEmailAndResetVerification(uuidID, normalizedEmail); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	if _, err := s.RequestEmailVerification(normalizedEmail); err != nil {
		log.Printf("failed to issue email verification code for %s: %v", normalizedEmail, err)
	}

	return s.repo.GetByID(uuidID)
}

func (s *service) ListPendingPartners(limit, offset int) ([]*domain.User, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	users, total, err := s.repo.ListPartnersByStatus(PartnerStatusPending, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*domain.User, 0, len(users))
	for i := range users {
		result = append(result, &users[i])
	}
	return result, total, nil
}

func (s *service) ListUsers(limit, offset int, roleFilter string, search string) ([]*domain.User, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	roles, err := resolveUserRoleFilter(roleFilter)
	if err != nil {
		return nil, 0, err
	}

	users, total, err := s.repo.ListUsersByRoles(roles, limit, offset, strings.TrimSpace(search))
	if err != nil {
		return nil, 0, err
	}

	result := make([]*domain.User, 0, len(users))
	for i := range users {
		result = append(result, &users[i])
	}

	return result, total, nil
}

func (s *service) BlockUser(userID string) (*domain.User, error) {
	return s.setUserBlocked(userID, true)
}

func (s *service) UnblockUser(userID string) (*domain.User, error) {
	return s.setUserBlocked(userID, false)
}

func (s *service) ApprovePartner(partnerID string) (*domain.User, error) {
	return s.setPartnerStatus(partnerID, PartnerStatusApproved)
}

func (s *service) RejectPartner(partnerID string) (*domain.User, error) {
	return s.setPartnerStatus(partnerID, PartnerStatusRejected)
}

func (s *service) RequestEmailVerification(email string) (int, error) {
	normalizedEmail, err := normalizeAndValidateEmail(email)
	if err != nil {
		return 0, err
	}

	exists, err := s.repo.ExistsByEmail(normalizedEmail)
	if err != nil {
		return 0, err
	}

	if !exists {
		return int(emailVerificationCodeTTL.Seconds()), nil
	}
	user, err := s.repo.GetUserByEmail(normalizedEmail)
	if err != nil {
		return 0, err
	}
	if user.IsVerified {
		return int(emailVerificationCodeTTL.Seconds()), nil
	}

	code, err := generateSixDigitCode()
	if err != nil {
		return 0, err
	}

	s.mu.Lock()
	s.emailVerificationCodes[normalizedEmail] = emailVerificationCodeEntry{
		Code:      code,
		ExpiresAt: time.Now().Add(emailVerificationCodeTTL),
	}
	s.mu.Unlock()

	log.Printf("Email verification code for %s: %s", normalizedEmail, code)

	return int(emailVerificationCodeTTL.Seconds()), nil
}

func (s *service) VerifyEmail(email, code string) error {
	normalizedEmail, err := normalizeAndValidateEmail(email)
	if err != nil {
		return err
	}
	code = strings.TrimSpace(code)

	s.mu.Lock()
	entry, ok := s.emailVerificationCodes[normalizedEmail]
	if !ok {
		s.mu.Unlock()
		return ErrInvalidVerificationCode
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.emailVerificationCodes, normalizedEmail)
		s.mu.Unlock()
		return ErrInvalidVerificationCode
	}
	if entry.Code != code {
		s.mu.Unlock()
		return ErrInvalidVerificationCode
	}
	delete(s.emailVerificationCodes, normalizedEmail)
	s.mu.Unlock()

	if err := s.repo.UpdateVerifiedStatusByEmail(normalizedEmail, true); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidVerificationCode
		}
		return err
	}

	return nil
}

func (s *service) ForgotPassword(email string) (int, error) {
	var err error
	email, err = normalizeAndValidateEmail(email)
	if err != nil {
		return 0, err
	}

	exists, err := s.repo.ExistsByEmail(email)
	if err != nil {
		return 0, err
	}

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
	var err error
	email, err = normalizeAndValidateEmail(email)
	if err != nil {
		return "", err
	}
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
	if err := validatePasswordComplexity(newPassword); err != nil {
		return err
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

	s.forgetAccountSecrets(user.Email)
	return s.repo.AnonymizeAccount(uid)
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

func (s *service) IsUserBlocked(id string) (bool, error) {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return false, ErrUserNotFound
	}

	return s.repo.IsUserBlocked(uuidID)
}

func (s *service) generateTokens(userID, role, restaurantID, partnerStatus string) (Tokens, error) {
	accessToken, err := s.tokenManager.NewAccessToken(userID, role, restaurantID, partnerStatus, s.accessTTL)
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
		email, err := normalizeAndValidateEmail(idToken)
		if err != nil {
			return "", "", ErrInvalidOAuthToken
		}
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
	email, err := normalizeAndValidateEmail(emailRaw)
	if err != nil {
		return "", "", ErrInvalidOAuthToken
	}

	name := strings.Split(email, "@")[0]
	if claimName, ok := claims["name"].(string); ok && strings.TrimSpace(claimName) != "" {
		name = strings.TrimSpace(claimName)
	}

	return email, name, nil
}

func validatePasswordComplexity(password string) error {
	if utf8.RuneCountInString(password) < 8 {
		return ErrWeakPassword
	}

	hasDigit := false
	hasSpecial := false

	for _, r := range password {
		if unicode.IsDigit(r) {
			hasDigit = true
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			hasSpecial = true
		}
	}

	if !hasDigit || !hasSpecial {
		return ErrWeakPassword
	}

	return nil
}

func normalizeAndValidateEmail(email string) (string, error) {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return "", ErrInvalidEmail
	}
	// We accept plain mailbox only, without display name wrappers.
	if strings.ContainsAny(trimmed, "<>") {
		return "", ErrInvalidEmail
	}

	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || parsed == nil || parsed.Address == "" || parsed.Name != "" {
		return "", ErrInvalidEmail
	}

	return strings.ToLower(trimmed), nil
}

func sameOptionalString(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func (s *service) setPartnerStatus(partnerID, nextStatus string) (*domain.User, error) {
	uid, err := uuid.Parse(strings.TrimSpace(partnerID))
	if err != nil {
		return nil, ErrPartnerNotFound
	}

	user, err := s.repo.GetByID(uid)
	if err != nil || user.Role != RolePartner {
		return nil, ErrPartnerNotFound
	}
	if user.DeletedAt != nil {
		return nil, ErrPartnerNotFound
	}
	if user.PartnerStatus != PartnerStatusPending {
		return nil, ErrInvalidPartnerStatusTransition
	}

	if err := s.repo.UpdatePartnerStatus(uid, nextStatus); err != nil {
		return nil, err
	}

	return s.repo.GetByID(uid)
}

func resolveUserRoleFilter(roleFilter string) ([]string, error) {
	role := strings.ToUpper(strings.TrimSpace(roleFilter))
	switch role {
	case "":
		return []string{RoleUser, RolePartner}, nil
	case RoleUser, RolePartner:
		return []string{role}, nil
	default:
		return nil, ErrInvalidUserRoleFilter
	}
}

func (s *service) setUserBlocked(userID string, isBlocked bool) (*domain.User, error) {
	uid, err := uuid.Parse(strings.TrimSpace(userID))
	if err != nil {
		return nil, ErrUserNotFound
	}

	user, err := s.repo.GetByID(uid)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if user.Role == RoleAdmin {
		return nil, ErrCannotBlockAdmin
	}
	if user.Role != RoleUser && user.Role != RolePartner {
		return nil, ErrUserNotFound
	}
	if user.DeletedAt != nil {
		return nil, ErrDeletedAccount
	}

	if err := s.repo.UpdateBlockedStatus(uid, isBlocked); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return s.repo.GetByID(uid)
}

func (s *service) forgetAccountSecrets(email string) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.emailVerificationCodes, normalizedEmail)
	delete(s.resetCodes, normalizedEmail)
	for token, entry := range s.resetTokens {
		if entry.Email == normalizedEmail {
			delete(s.resetTokens, token)
		}
	}
}
