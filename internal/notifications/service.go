package notifications

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"kursach_backend/internal/domain"

	"github.com/google/uuid"
)

var ErrStoreRequired = errors.New("notifications store is required")

type Service interface {
	SendToUser(ctx context.Context, userID uuid.UUID, payload Payload) error
	ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Notification, error)
	CleanupOld(ctx context.Context, olderThan time.Time) error
	StartCleanupWorker(ctx context.Context)
}

type service struct {
	store Store
	push  PushProvider
}

func NewService(store Store, push PushProvider) Service {
	if push == nil {
		push = LogPushProvider{}
	}
	return &service{store: store, push: push}
}

func (s *service) SendToUser(ctx context.Context, userID uuid.UUID, payload Payload) error {
	if s.store == nil {
		return ErrStoreRequired
	}

	payload = normalizePayload(payload)
	if err := s.store.Create(ctx, &domain.Notification{
		UserID:    userID,
		Message:   payload.Body,
		CreatedAt: time.Now(),
	}); err != nil {
		return err
	}

	deviceToken, ok, err := s.store.GetDeviceToken(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		log.Printf("[Notifications] user %s has no device token", userID)
		return nil
	}

	if err := s.push.Send(ctx, deviceToken, payload); err != nil {
		log.Printf("[Notifications] push send failed for user %s: %v", userID, err)
	}
	return nil
}

func (s *service) ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Notification, error) {
	if s.store == nil {
		return nil, ErrStoreRequired
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.store.ListForUser(ctx, userID, limit, offset)
}

func (s *service) CleanupOld(ctx context.Context, olderThan time.Time) error {
	if s.store == nil {
		return ErrStoreRequired
	}
	return s.store.CleanupOld(ctx, olderThan)
}

func (s *service) StartCleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.CleanupOld(ctx, time.Now().AddDate(0, 0, -30)); err != nil {
				log.Printf("[Notifications] cleanup failed: %v", err)
			}
		}
	}
}

func normalizePayload(payload Payload) Payload {
	payload.Title = strings.TrimSpace(payload.Title)
	payload.Body = strings.TrimSpace(payload.Body)
	payload.DeepLink = strings.TrimSpace(payload.DeepLink)

	if payload.Title == "" {
		payload.Title = "Уведомление"
	}
	if payload.Body == "" {
		payload.Body = payload.Title
	}
	return payload
}
