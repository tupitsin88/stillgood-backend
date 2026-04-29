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

func (p *fakePushProvider) SendBatch(ctx context.Context, tokens []string, payload Payload) error {
	if len(tokens) > 0 {
		p.deviceToken = tokens[0]
	}
	p.payload = payload
	p.sendCount++
	return p.err
}

func TestServiceSendToUserStoresNotificationAndSendsPush(t *testing.T) {
	store := &fakeStore{deviceToken: "device-token", hasDeviceToken: true}
	push := &fakePushProvider{}
	s := NewService(store, push).(*service)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	userID := uuid.New()
	err := s.SendToUser(ctx, userID, Payload{
		Title:    "Заказ оплачен",
		Body:     "Ваш заказ успешно оплачен",
		DeepLink: "/orders/123",
	})

	require.NoError(t, err)
	require.Len(t, store.notifications, 1)
	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, userID, store.notifications[0].UserID)
	assert.Equal(t, "Ваш заказ успешно оплачен", store.notifications[0].Message)
	assert.Equal(t, 1, push.sendCount)
	assert.Equal(t, "device-token", push.deviceToken)
	assert.Equal(t, "/orders/123", push.payload.DeepLink)
}

func TestServiceSendToUserSkipsPushWithoutDeviceToken(t *testing.T) {
	store := &fakeStore{}
	push := &fakePushProvider{}
	s := NewService(store, push).(*service)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	err := s.SendToUser(ctx, uuid.New(), Payload{Body: "Message"})

	require.NoError(t, err)
	require.Len(t, store.notifications, 1)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, push.sendCount)
}

func TestServiceSendToUserDoesNotFailOnPushError(t *testing.T) {
	store := &fakeStore{deviceToken: "device-token", hasDeviceToken: true}
	push := &fakePushProvider{err: errors.New("fcm unavailable")}
	s := NewService(store, push).(*service)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	err := s.SendToUser(ctx, uuid.New(), Payload{Body: "Message"})

	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, push.sendCount)
	require.Len(t, store.notifications, 1)
}
