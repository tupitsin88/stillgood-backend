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

type PushJob struct {
	Tokens  []string
	Payload Payload
}

var ErrStoreRequired = errors.New("notifications store is required")

type Service interface {
	SendToUser(ctx context.Context, userID uuid.UUID, payload Payload) error
	ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Notification, error)
	CleanupOld(ctx context.Context, olderThan time.Time) error
	StartCleanupWorker(ctx context.Context)
	Start(ctx context.Context)
	SendToTokens(ctx context.Context, tokens []string, payload Payload) error
}

type service struct {
	store   Store
	push    PushProvider
	jobChan chan PushJob
}

func NewService(store Store, push PushProvider) Service {
	if push == nil {
		push = LogPushProvider{}
	}
	return &service{
		store:   store,
		push:    push,
		jobChan: make(chan PushJob, 1000),
	}
}

func (s *service) Start(ctx context.Context) {
	log.Println("[Notifications] Worker pool started")
	for i := 0; i < 3; i++ {
		go s.worker(ctx)
	}
}

func (s *service) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-s.jobChan:
			if err := s.push.SendBatch(ctx, job.Tokens, job.Payload); err != nil {
				log.Printf("[Notifications] Batch push failed: %v", err)
			}
		}
	}
}

func (s *service) SendToUser(ctx context.Context, userID uuid.UUID, payload Payload) error {
	if s.store == nil {
		return ErrStoreRequired
	}

	payload = normalizePayload(payload)
	notificationType := payload.Data["type"]
	if err := s.store.Create(ctx, &domain.Notification{
		UserID:    userID,
		Title:     payload.Title,
		Body:      payload.Body,
		DeepLink:  payload.DeepLink,
		Type:      notificationType,
		CreatedAt: time.Now(),
	}); err != nil {
		return err
	}

	deviceToken, ok, err := s.store.GetDeviceToken(ctx, userID)
	if err != nil {
		return err
	}
	if !ok {
		log.Printf("[Notifications] user %s has no device token, skipping push", userID)
		return nil
	}
	job := PushJob{
		Tokens:  []string{deviceToken},
		Payload: payload,
	}

	select {
	case s.jobChan <- job:
	default:
		log.Println("[Notifications] Queue is full, dropping push notification")
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

func (s *service) SendToTokens(ctx context.Context, tokens []string, payload Payload) error {
	if len(tokens) == 0 {
		return nil
	}
	job := PushJob{
		Tokens:  tokens,
		Payload: payload,
	}
	select {
	case s.jobChan <- job:
		log.Printf("[Notifications] Enqueued batch for %d tokens", len(tokens))
	default:
		log.Println("[Notifications] Queue is full, dropping batch push")
	}
	return nil
}
