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

func (p *FCMPushProvider) Send(ctx context.Context, deviceToken string, payload Payload) error {
	data := make(map[string]string, len(payload.Data)+1)
	for key, value := range payload.Data {
		data[key] = value
	}
	if payload.DeepLink != "" {
		data["deepLink"] = payload.DeepLink
	}

	message := &messaging.Message{
		Token: deviceToken,
		Notification: &messaging.Notification{
			Title: payload.Title,
			Body:  payload.Body,
		},
		Data: data,
	}

	_, err := p.client.Send(ctx, message)
	return err
}
