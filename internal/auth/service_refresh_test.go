package auth

import (
	"errors"
	"testing"
	"time"

	"kursach_backend/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type refreshRepoStub struct {
	*authEmailRepoStub
	usersByID map[uuid.UUID]*domain.User
	sessions  map[uuid.UUID]*domain.RefreshSession
}

func newRefreshRepoStub(users ...*domain.User) *refreshRepoStub {
	repo := &refreshRepoStub{
		authEmailRepoStub: newAuthEmailRepoStub(users...),
		usersByID:         make(map[uuid.UUID]*domain.User),
		sessions:          make(map[uuid.UUID]*domain.RefreshSession),
	}
	for _, user := range users {
		repo.usersByID[user.ID] = user
	}
	return repo
}

func (r *refreshRepoStub) GetByID(id uuid.UUID) (*domain.User, error) {
	user, ok := r.usersByID[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (r *refreshRepoStub) CreateRefreshSession(session *domain.RefreshSession) error {
	if session == nil {
		return errors.New("refresh session is nil")
	}
	copied := *session
	r.sessions[session.JTI] = &copied
	return nil
}

func (r *refreshRepoStub) IsRefreshSessionActive(jti, userID uuid.UUID, now time.Time) (bool, error) {
	session, ok := r.sessions[jti]
	if !ok {
		return false, nil
	}
	if session.UserID != userID || session.RevokedAt != nil {
		return false, nil
	}
	return session.ExpiresAt.After(now), nil
}

func (r *refreshRepoStub) RevokeRefreshSession(jti uuid.UUID, revokedAt time.Time) error {
	session, ok := r.sessions[jti]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	session.RevokedAt = &revokedAt
	return nil
}

func TestRefreshTokenRevocationPersistsAcrossServiceInstances(t *testing.T) {
	user := &domain.User{
		ID:    uuid.New(),
		Email: "user@example.com",
		Role:  RoleUser,
		Name:  "User",
	}
	repo := newRefreshRepoStub(user)
	tokenManager, err := NewTokenManager("test-secret")
	require.NoError(t, err)

	service1 := NewService(repo, tokenManager, time.Minute, time.Hour, "").(*service)
	tokens, err := service1.generateTokens(user.ID.String(), user.Role, "", "")
	require.NoError(t, err)
	require.Len(t, repo.sessions, 1)

	require.NoError(t, service1.Logout(tokens.RefreshToken))
	_, err = service1.RefreshTokens(tokens.RefreshToken)
	require.ErrorIs(t, err, ErrInvalidRefreshToken)

	service2 := NewService(repo, tokenManager, time.Minute, time.Hour, "").(*service)
	_, err = service2.RefreshTokens(tokens.RefreshToken)
	require.ErrorIs(t, err, ErrInvalidRefreshToken)
}

func TestRefreshTokensRejectsAccessToken(t *testing.T) {
	user := &domain.User{
		ID:    uuid.New(),
		Email: "user@example.com",
		Role:  RoleUser,
		Name:  "User",
	}
	repo := newRefreshRepoStub(user)
	tokenManager, err := NewTokenManager("test-secret")
	require.NoError(t, err)
	service := NewService(repo, tokenManager, time.Minute, time.Hour, "").(*service)

	accessToken, err := tokenManager.NewAccessToken(user.ID.String(), user.Role, "", "", time.Minute)
	require.NoError(t, err)

	_, err = service.RefreshTokens(accessToken)
	require.ErrorIs(t, err, ErrInvalidRefreshToken)
}

func TestRefreshTokensCreatesNextPersistentSession(t *testing.T) {
	user := &domain.User{
		ID:    uuid.New(),
		Email: "user@example.com",
		Role:  RoleUser,
		Name:  "User",
	}
	repo := newRefreshRepoStub(user)
	tokenManager, err := NewTokenManager("test-secret")
	require.NoError(t, err)
	service := NewService(repo, tokenManager, time.Minute, time.Hour, "").(*service)

	tokens, err := service.generateTokens(user.ID.String(), user.Role, "", "")
	require.NoError(t, err)

	refreshed, err := service.RefreshTokens(tokens.RefreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, refreshed.RefreshToken)
	require.NotEqual(t, tokens.RefreshToken, refreshed.RefreshToken)
	require.Len(t, repo.sessions, 2)
}
