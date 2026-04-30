package email

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/resend/resend-go/v3"
)

type ResendConfig struct {
	APIKey     string
	BaseURL    string
	From       Address
	HTTPClient *http.Client
}

type ResendSender struct {
	client *resend.Client
	from   Address
}

func NewResendSender(cfg ResendConfig) (*ResendSender, error) {
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("RESEND_API_KEY is required")
	}
	from, err := normalizeAddress("sender", cfg.From)
	if err != nil {
		return nil, err
	}
	cfg.From = from

	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: defaultEmailRequestTimeout}
	}
	client := resend.NewCustomClient(cfg.HTTPClient, cfg.APIKey)
	if strings.TrimSpace(cfg.BaseURL) != "" {
		baseURL, err := normalizeBaseURL(cfg.BaseURL)
		if err != nil {
			return nil, err
		}
		client.BaseURL = baseURL
	}

	return &ResendSender{
		client: client,
		from:   cfg.From,
	}, nil
}

func (s *ResendSender) SendEmailVerificationCode(ctx context.Context, to Address, code string, ttl time.Duration) error {
	return s.send(ctx, to, &resend.SendEmailRequest{
		Subject: "Код подтверждения FoodSharing",
		Html:    codeEmailHTML("Подтверждение email", "Введите этот код в приложении, чтобы подтвердить email.", code, ttl),
		Tags:    []resend.Tag{{Name: "type", Value: "email_verification"}},
	})
}

func (s *ResendSender) SendPasswordResetCode(ctx context.Context, to Address, code string, ttl time.Duration) error {
	return s.send(ctx, to, &resend.SendEmailRequest{
		Subject: "Код для сброса пароля FoodSharing",
		Html:    codeEmailHTML("Сброс пароля", "Введите этот код в приложении, чтобы сбросить пароль.", code, ttl),
		Tags:    []resend.Tag{{Name: "type", Value: "password_reset"}},
	})
}

func (s *ResendSender) send(ctx context.Context, to Address, req *resend.SendEmailRequest) error {
	normalizedTo, err := normalizeAddress("recipient", to)
	if err != nil {
		return err
	}

	req.From = formatAddress(s.from)
	req.To = []string{normalizedTo.Email}

	_, err = s.client.Emails.SendWithContext(ctx, req)
	if err != nil {
		return fmt.Errorf("send email through Resend: %w", err)
	}

	return nil
}

func normalizeBaseURL(rawURL string) (*url.URL, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("RESEND_BASE_URL is required")
	}
	if !strings.HasSuffix(rawURL, "/") {
		rawURL += "/"
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid RESEND_BASE_URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid RESEND_BASE_URL: absolute URL is required")
	}
	return parsed, nil
}

func formatAddress(address Address) string {
	if strings.TrimSpace(address.Name) == "" {
		return address.Email
	}
	return fmt.Sprintf("%s <%s>", address.Name, address.Email)
}
