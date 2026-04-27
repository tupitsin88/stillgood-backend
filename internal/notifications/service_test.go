package notifications

import (
	"context"
	"errors"
	"kursach_backend/internal/domain"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	deviceToken      string
	hasDeviceToken   bool
	notifications    []domain.Notification
	cleanupOlderThan *time.Time
}

func (s *fakeStore) Create(ctx context.Context, notification *domain.Notification) error {
	s.notifications = append(s.notifications, *notification)
	return nil
}

func (s *fakeStore) GetDeviceToken(ctx context.Context, userID uuid.UUID) (string, bool, error) {
	return s.deviceToken, s.hasDeviceToken, nil
}

func (s *fakeStore) ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Notification, error) {
	return s.notifications, nil
}

func (s *fakeStore) CleanupOld(ctx context.Context, olderThan time.Time) error {
	s.cleanupOlderThan = &olderThan
	return nil
}

type fakePushProvider struct {
	err         error
	deviceToken string
	payload     Payload
	sendCount   int
}

func (p *fakePushProvider) Send(ctx context.Context, deviceToken string, payload Payload) error {
	p.deviceToken = deviceToken
	p.payload = payload
	p.sendCount++
	return p.err
}

func TestServiceSendToUserStoresNotificationAndSendsPush(t *testing.T) {
	store := &fakeStore{deviceToken: "device-token", hasDeviceToken: true}
	push := &fakePushProvider{}
	service := NewService(store, push)
	userID := uuid.New()

	err := service.SendToUser(context.Background(), userID, Payload{
		Title:    "Заказ оплачен",
		Body:     "Ваш заказ успешно оплачен",
		DeepLink: "/orders/123",
	})

	require.NoError(t, err)
	require.Len(t, store.notifications, 1)
	assert.Equal(t, userID, store.notifications[0].UserID)
	assert.Equal(t, "Ваш заказ успешно оплачен", store.notifications[0].Message)
	assert.Equal(t, 1, push.sendCount)
	assert.Equal(t, "device-token", push.deviceToken)
	assert.Equal(t, "/orders/123", push.payload.DeepLink)
}

func TestServiceSendToUserSkipsPushWithoutDeviceToken(t *testing.T) {
	store := &fakeStore{}
	push := &fakePushProvider{}
	service := NewService(store, push)

	err := service.SendToUser(context.Background(), uuid.New(), Payload{Body: "Message"})

	require.NoError(t, err)
	require.Len(t, store.notifications, 1)
	assert.Equal(t, 0, push.sendCount)
}

func TestServiceSendToUserDoesNotFailOnPushError(t *testing.T) {
	store := &fakeStore{deviceToken: "device-token", hasDeviceToken: true}
	push := &fakePushProvider{err: errors.New("fcm unavailable")}
	service := NewService(store, push)

	err := service.SendToUser(context.Background(), uuid.New(), Payload{Body: "Message"})

	require.NoError(t, err)
	assert.Equal(t, 1, push.sendCount)
	require.Len(t, store.notifications, 1)
}
