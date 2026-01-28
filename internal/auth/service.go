package auth

import (
	"errors"
	"kursach_backend/internal/domain"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Tokens struct {
	AccessToken  string
	RefreshToken string
}

type Service interface {
	Register(email, password, name string) (Tokens, error)
	Login(email, password string) (Tokens, error)
	RefreshTokens(refreshToken string) (Tokens, error)
	Logout() error
}

type service struct {
	repo         Repository
	tokenManager *TokenManager
	accessTTL    time.Duration
	refreshTTL   time.Duration
}

func NewService(repo Repository, tokenManager *TokenManager, accessTTL, refreshTTL time.Duration) Service {
	return &service{
		repo:         repo,
		tokenManager: tokenManager,
		accessTTL:    accessTTL,
		refreshTTL:   refreshTTL,
	}
}

func (s *service) Register(email, password, name string) (Tokens, error) {
	hashedPass, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	user := &domain.User{
		Email:        email,
		PasswordHash: string(hashedPass),
		Name:         name,
		Role:         "USER",
		AuthProvider: "email",
	}

	if err := s.repo.CreateUser(user); err != nil {
		return Tokens{}, err
	}

	return s.generateTokens(user.ID.String(), user.Role)
}

func (s *service) Login(email, password string) (Tokens, error) {
	user, err := s.repo.GetUserByEmail(email)
	if err != nil {
		return Tokens{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return Tokens{}, err
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
	user, err := s.repo.GetUserByID(sub)
	if err != nil {
		return Tokens{}, err // User probably deleted or ID changed
	}

	return s.generateTokens(user.ID.String(), user.Role)
}

func (s *service) Logout() error {
	// Stateless JWTs don't support true server-side logout without a blacklist/redis.
	// We just return nil as requested.
	return nil
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
