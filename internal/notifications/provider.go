package notifications

import (
	"context"
	"log"
)

type Payload struct {
	Title    string
	Body     string
	DeepLink string
	Data     map[string]string
}

type PushProvider interface {
	Send(ctx context.Context, deviceToken string, payload Payload) error
}

type LogPushProvider struct{}

func (LogPushProvider) Send(ctx context.Context, deviceToken string, payload Payload) error {
	log.Printf("[NOTIFICATION STUB] deviceTokenLen=%d title=%q body=%q deepLink=%q", len(deviceToken), payload.Title, payload.Body, payload.DeepLink)
	return nil
}
