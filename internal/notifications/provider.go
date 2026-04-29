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
	SendBatch(ctx context.Context, tokens []string, payload Payload) error
}

type LogPushProvider struct{}

func (LogPushProvider) SendBatch(ctx context.Context, tokens []string, payload Payload) error {
	log.Printf("[NOTIFICATION STUB] BATCH SEND to %d tokens: %q", len(tokens), payload.Title)
	return nil
}
