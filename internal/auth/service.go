package auth

import (
	"crypto/rand"
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

const (
	passwordResetCodeTTL  = 10 * time.Minute
	passwordResetTokenTTL = 15 * time.Minute
)

type Tokens struct {
	AccessToken  string
	RefreshToken string
}

type Service interface {
	Register(email, password, name, deviceToken string) (Tokens, error)
	RegisterPartner(input PartnerRegisterRequest) (Tokens, error)
	Login(email, password, deviceToken string) (Tokens, error)
	RefreshTokens(refreshToken string) (Tokens, error)
	ChangePassword(userID, currentPassword, newPassword string) error
	ForgotPassword(email string) (int, error)
	VerifyResetCode(email, code string) (string, error)
	ResetPassword(resetToken, newPassword string) error
	Logout() error
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
}

func NewService(repo Repository, tokenManager *TokenManager, accessTTL, refreshTTL time.Duration) Service {
	return &service{
		repo:         repo,
		tokenManager: tokenManager,
		accessTTL:    accessTTL,
		refreshTTL:   refreshTTL,
		resetCodes:   make(map[string]resetCodeEntry),
		resetTokens:  make(map[string]resetTokenEntry),
	}
}

func (s *service) Register(email, password, name, deviceToken string) (Tokens, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	exists, err := s.repo.ExistsByEmail(email)
	if err != nil {
		return Tokens{}, err
	}
	if exists {
		return Tokens{}, ErrEmailAlreadyExists
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Tokens{}, err
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
		return Tokens{}, err
	}

	return s.generateTokens(user.ID.String(), user.Role)
}

func (s *service) RegisterPartner(input PartnerRegisterRequest) (Tokens, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))

	exists, err := s.repo.ExistsByEmail(email)
	if err != nil {
		return Tokens{}, err
	}
	if exists {
		return Tokens{}, ErrEmailAlreadyExists
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return Tokens{}, err
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
		return Tokens{}, err
	}

	return s.generateTokens(user.ID.String(), user.Role)
}

func (s *service) Login(email, password, deviceToken string) (Tokens, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		return Tokens{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return Tokens{}, err
	}

	// Update Device Token if provided
	if deviceToken != "" {
		if err := s.repo.UpdateDeviceToken(user.ID, deviceToken); err != nil {
			// Log error but don't fail login? Or fail? Usually, we just log it.
			// For simplicity nicely, we can ignore it or return error. Let's return error for now to be safe.
			return Tokens{}, err
		}
	}

	return s.generateTokens(user.ID.String(), user.Role)
}

func (s *service) RefreshTokens(refreshToken string) (Tokens, error) {
	claims, err := s.tokenManager.Parse(refreshToken)
	if err != nil {
		return Tokens{}, err
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return Tokens{}, errors.New("invalid sub claim")
	}

	// Security check: Verify user exists and is active in DB
	user, err := s.GetUserByID(sub)
	if err != nil {
		return Tokens{}, err // User probably deleted or ID changed
	}

	return s.generateTokens(user.ID.String(), user.Role)
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

func (s *service) Logout() error {
	// Stateless JWTs don't support true server-side logout without a blacklist/redis.
	// We just return nil as requested.
	return nil
}

func (s *service) GetUserByID(id string) (*domain.User, error) {
	uuidID, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	return s.repo.GetByID(uuidID)
}

func (s *service) generateTokens(userID, role string) (Tokens, error) {
	accessToken, err := s.tokenManager.NewAccessToken(userID, role, s.accessTTL)
	if err != nil {
		return Tokens{}, err
	}

	refreshToken, err := s.tokenManager.NewRefreshToken(userID, s.refreshTTL)
	if err != nil {
		return Tokens{}, err
	}

	return Tokens{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func generateSixDigitCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
