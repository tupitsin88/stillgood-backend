package notifications

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

type FCMPushProvider struct {
	client *messaging.Client
}

func NewPushProviderFromEnv(ctx context.Context) PushProvider {
	enabled, err := strconv.ParseBool(os.Getenv("FCM_ENABLED"))
	if err != nil || !enabled {
		return LogPushProvider{}
	}

	credentialsFile := strings.TrimSpace(os.Getenv("FIREBASE_CREDENTIALS_FILE"))
	if credentialsFile == "" {
		log.Println("[Notifications] FCM_ENABLED=true, but FIREBASE_CREDENTIALS_FILE is empty; using log provider")
		return LogPushProvider{}
	}

	provider, err := NewFCMPushProvider(ctx, credentialsFile)
	if err != nil {
		log.Printf("[Notifications] failed to initialize FCM provider: %v; using log provider", err)
		return LogPushProvider{}
	}
	return provider
}

func NewFCMPushProvider(ctx context.Context, credentialsFile string) (*FCMPushProvider, error) {
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		return nil, err
	}

	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}

	return &FCMPushProvider{client: client}, nil
}

func (p *FCMPushProvider) SendBatch(ctx context.Context, tokens []string, payload Payload) error {
	if len(tokens) == 0 {
		return nil
	}

	fcmData := make(map[string]string, len(payload.Data)+1)
	for key, value := range payload.Data {
		fcmData[key] = value
	}
	if payload.DeepLink != "" {
		fcmData["deepLink"] = payload.DeepLink
	}

	for i := 0; i < len(tokens); i += 500 {
		end := i + 500
		if end > len(tokens) {
			end = len(tokens)
		}
		batch := tokens[i:end]
		var messages []*messaging.Message
		for _, t := range batch {
			messages = append(messages, &messaging.Message{
				Token: t,
				Notification: &messaging.Notification{
					Title: payload.Title,
					Body:  payload.Body,
				},
				Data: fcmData,
			})
		}
		response, err := p.client.SendEach(ctx, messages)
		if err != nil {
			log.Printf("[FCM] Batch send error: %v", err)
			continue
		}
		log.Printf("[FCM] Batch sent: %d success", response.SuccessCount)
	}
	return nil
}
