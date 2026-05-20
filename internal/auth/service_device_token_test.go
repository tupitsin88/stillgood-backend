package auth

import (
	"testing"
	"time"

	"kursach_backend/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type deviceTokenRepoStub struct {
	*authEmailRepoStub
	updatedUserID uuid.UUID
	updatedToken  string
}

func (r *deviceTokenRepoStub) UpdateDeviceToken(userID uuid.UUID, token string) error {
	r.updatedUserID = userID
	r.updatedToken = token
	return nil
}

func TestUpdateDeviceToken(t *testing.T) {
	userID := uuid.New()
	repo := &deviceTokenRepoStub{
		authEmailRepoStub: newAuthEmailRepoStub(&domain.User{ID: userID, Email: "user@example.com"}),
	}
	service := NewService(repo, nil, time.Minute, time.Hour, "")

	err := service.UpdateDeviceToken(userID.String(), "  fresh-token  ", "android")

	require.NoError(t, err)
	assert.Equal(t, userID, repo.updatedUserID)
	assert.Equal(t, "fresh-token", repo.updatedToken)
}

func TestUpdateDeviceTokenValidation(t *testing.T) {
	repo := &deviceTokenRepoStub{authEmailRepoStub: newAuthEmailRepoStub()}
	service := NewService(repo, nil, time.Minute, time.Hour, "")

	err := service.UpdateDeviceToken(uuid.NewString(), " ", "android")
	assert.ErrorIs(t, err, ErrDeviceTokenRequired)

	err = service.UpdateDeviceToken(uuid.NewString(), "token", "web")
	assert.ErrorIs(t, err, ErrInvalidDevicePlatform)
}
