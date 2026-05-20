package auth

import (
	"testing"
	"time"

	"kursach_backend/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type partnerJWTRepoStub struct {
	*refreshRepoStub
	restaurantByPartner map[uuid.UUID]uuid.UUID
}

func newPartnerJWTRepoStub(users ...*domain.User) *partnerJWTRepoStub {
	return &partnerJWTRepoStub{
		refreshRepoStub:     newRefreshRepoStub(users...),
		restaurantByPartner: make(map[uuid.UUID]uuid.UUID),
	}
}

func (r *partnerJWTRepoStub) UpdateDeviceToken(userID uuid.UUID, token string) error {
	user, ok := r.usersByID[userID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	user.DeviceToken = &token
	return nil
}

func (r *partnerJWTRepoStub) UpdatePartnerStatus(userID uuid.UUID, status string) error {
	user, ok := r.usersByID[userID]
	if !ok || user.Role != RolePartner {
		return gorm.ErrRecordNotFound
	}
	user.PartnerStatus = status
	return nil
}

func (r *partnerJWTRepoStub) SyncPartnerRestaurantID(userID uuid.UUID) error {
	restaurantID, ok := r.restaurantByPartner[userID]
	if !ok {
		return nil
	}
	user, ok := r.usersByID[userID]
	if !ok || user.Role != RolePartner {
		return gorm.ErrRecordNotFound
	}
	user.RestaurantID = &restaurantID
	return nil
}

func TestLoginIncludesRestaurantIDForApprovedPartner(t *testing.T) {
	restaurantID := uuid.New()
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("Password!1"), bcrypt.DefaultCost)
	require.NoError(t, err)
	user := &domain.User{
		ID:            uuid.New(),
		Email:         "partner@example.com",
		PasswordHash:  string(passwordHash),
		Role:          RolePartner,
		PartnerStatus: PartnerStatusApproved,
		RestaurantID:  &restaurantID,
	}
	repo := newPartnerJWTRepoStub(user)
	tokenManager, err := NewTokenManager("test-secret")
	require.NoError(t, err)
	service := NewService(repo, tokenManager, time.Minute, time.Hour, "")

	tokens, _, err := service.Login("partner@example.com", "Password!1", "device-token")

	require.NoError(t, err)
	claims, err := tokenManager.Parse(tokens.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, restaurantID.String(), claims["restaurant_id"])
	assert.Equal(t, PartnerStatusApproved, claims["partner_status"])
}

func TestRefreshTokensUsesLatestPartnerRestaurantID(t *testing.T) {
	restaurantID := uuid.New()
	user := &domain.User{
		ID:            uuid.New(),
		Email:         "partner@example.com",
		Role:          RolePartner,
		PartnerStatus: PartnerStatusApproved,
	}
	repo := newPartnerJWTRepoStub(user)
	tokenManager, err := NewTokenManager("test-secret")
	require.NoError(t, err)
	service := NewService(repo, tokenManager, time.Minute, time.Hour, "").(*service)
	oldTokens, err := service.generateTokens(user.ID.String(), user.Role, "", user.PartnerStatus)
	require.NoError(t, err)

	oldClaims, err := tokenManager.Parse(oldTokens.AccessToken)
	require.NoError(t, err)
	assert.NotContains(t, oldClaims, "restaurant_id")
	user.RestaurantID = &restaurantID

	refreshed, err := service.RefreshTokens(oldTokens.RefreshToken)

	require.NoError(t, err)
	newClaims, err := tokenManager.Parse(refreshed.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, restaurantID.String(), newClaims["restaurant_id"])
	assert.Equal(t, PartnerStatusApproved, newClaims["partner_status"])
}

func TestApprovePartnerSyncsExistingRestaurantID(t *testing.T) {
	restaurantID := uuid.New()
	user := &domain.User{
		ID:            uuid.New(),
		Email:         "partner@example.com",
		Role:          RolePartner,
		PartnerStatus: PartnerStatusPending,
	}
	repo := newPartnerJWTRepoStub(user)
	repo.restaurantByPartner[user.ID] = restaurantID
	tokenManager, err := NewTokenManager("test-secret")
	require.NoError(t, err)
	service := NewService(repo, tokenManager, time.Minute, time.Hour, "")

	approved, err := service.ApprovePartner(user.ID.String())

	require.NoError(t, err)
	require.NotNil(t, approved.RestaurantID)
	assert.Equal(t, PartnerStatusApproved, approved.PartnerStatus)
	assert.Equal(t, restaurantID, *approved.RestaurantID)
}
