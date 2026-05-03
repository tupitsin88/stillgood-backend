package notifications

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestService_SendToUser_Async(t *testing.T) {
	store := &fakeStore{deviceToken: "test-token", hasDeviceToken: true}
	push := &fakeAsyncPushProvider{}
	s := NewService(store, push).(*service)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)

	userID := uuid.New()
	err := s.SendToUser(ctx, userID, Payload{Title: "Test", Body: "Hello"})

	assert.NoError(t, err)
	assert.Len(t, store.notifications, 1)

	assert.Eventually(t, func() bool {
		return push.count() == 1
	}, time.Second, 10*time.Millisecond)

	sendCount, lastTokens := push.snapshot()
	assert.Equal(t, 1, sendCount)
	assert.Equal(t, []string{"test-token"}, lastTokens)
}

func TestService_SendToUser_NoToken(t *testing.T) {
	store := &fakeStore{hasDeviceToken: false}
	push := &fakeAsyncPushProvider{}
	s := NewService(store, push).(*service)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	err := s.SendToUser(ctx, uuid.New(), Payload{Body: "No token"})

	assert.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, push.count())
}

func TestService_FirebaseError_DoesNotAffectFlow(t *testing.T) {
	store := &fakeStore{deviceToken: "token", hasDeviceToken: true}
	push := &fakeAsyncPushProvider{err: context.DeadlineExceeded}
	s := NewService(store, push).(*service)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	err := s.SendToUser(ctx, uuid.New(), Payload{Body: "Failing push"})
	assert.NoError(t, err, "Ошибка Firebase не должна прокидываться в бизнес-логику")
}

func TestService_SendToTokens_Async(t *testing.T) {
	push := &fakeAsyncPushProvider{}
	s := NewService(nil, push).(*service)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)

	tokens := []string{"token-1", "token-2", "token-3"}
	payload := Payload{Title: "Broadcast", Body: "Hello everyone"}

	err := s.SendToTokens(ctx, tokens, payload)

	assert.NoError(t, err)

	assert.Eventually(t, func() bool {
		return push.count() == 1
	}, time.Second, 10*time.Millisecond)

	sendCount, lastTokens := push.snapshot()
	assert.Equal(t, 1, sendCount)
	assert.Equal(t, tokens, lastTokens)
}

type fakeAsyncPushProvider struct {
	mu         sync.Mutex
	err        error
	sendCount  int
	lastTokens []string
}

func (f *fakeAsyncPushProvider) SendBatch(ctx context.Context, tokens []string, payload Payload) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sendCount++
	f.lastTokens = append([]string(nil), tokens...)
	return f.err
}

func (f *fakeAsyncPushProvider) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sendCount
}

func (f *fakeAsyncPushProvider) snapshot() (int, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sendCount, append([]string(nil), f.lastTokens...)
}
