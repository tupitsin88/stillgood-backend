package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/resend/resend-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResendSenderSendsVerificationPayload(t *testing.T) {
	var captured resend.SendEmailRequest
	var authHeader string
	var userAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("authorization")
		userAgent = r.Header.Get("user-agent")
		assert.Equal(t, "application/json", r.Header.Get("content-type"))
		assert.Equal(t, "application/json", r.Header.Get("accept"))
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/emails", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"49a3999c-0ce1-4ea6-ab68-afcd6dc2e794"}`))
	}))
	defer server.Close()

	from := Address{Email: "onboarding@resend.dev", Name: defaultSenderName}
	sender, err := NewResendSender(ResendConfig{
		APIKey:     "test-api-key",
		BaseURL:    server.URL,
		From:       from,
		HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	err = sender.SendEmailVerificationCode(
		context.Background(),
		Address{Email: "user@example.com", Name: "Alice"},
		"123456",
		10*time.Minute,
	)
	require.NoError(t, err)

	assert.Equal(t, "Bearer test-api-key", authHeader)
	assert.Contains(t, userAgent, "resend-go/")
	assert.Equal(t, formatAddress(from), captured.From)
	assert.Equal(t, []string{"user@example.com"}, captured.To)
	assert.Equal(t, "Код подтверждения "+defaultSenderName, captured.Subject)
	assert.Contains(t, captured.Html, "123456")
	assert.Contains(t, captured.Html, "Код действует 10 минут")
	assert.Equal(t, []resend.Tag{{Name: "type", Value: "email_verification"}}, captured.Tags)
}

func TestResendSenderReturnsProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"domain not verified"}`))
	}))
	defer server.Close()

	sender, err := NewResendSender(ResendConfig{
		APIKey:     "test-api-key",
		BaseURL:    server.URL,
		From:       Address{Email: "onboarding@resend.dev"},
		HTTPClient: server.Client(),
	})
	require.NoError(t, err)

	err = sender.SendPasswordResetCode(context.Background(), Address{Email: "user@example.com"}, "654321", 10*time.Minute)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain not verified")
}

func TestResendSenderValidatesConfig(t *testing.T) {
	_, err := NewResendSender(ResendConfig{
		APIKey: "test-api-key",
		From:   Address{Email: "not-an-email"},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sender email")
}
