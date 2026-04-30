package email

import (
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"
)

const (
	defaultProvider            = "log"
	defaultEmailRequestTimeout = 10 * time.Second
	defaultSenderName          = "FoodSharing"
)

func NewSenderFromEnv() (Sender, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_PROVIDER")))
	if provider == "" {
		provider = defaultProvider
	}

	switch provider {
	case "log", "mock":
		return LogSender{}, nil
	case "resend":
		return NewResendSender(resendConfigFromEnv())
	default:
		return nil, fmt.Errorf("unsupported EMAIL_PROVIDER %q", provider)
	}
}

func resendConfigFromEnv() ResendConfig {
	timeout, err := time.ParseDuration(strings.TrimSpace(os.Getenv("EMAIL_REQUEST_TIMEOUT")))
	if err != nil || timeout <= 0 {
		timeout = defaultEmailRequestTimeout
	}

	fromName := strings.TrimSpace(os.Getenv("EMAIL_FROM_NAME"))
	if fromName == "" {
		fromName = defaultSenderName
	}

	return ResendConfig{
		APIKey:  strings.TrimSpace(os.Getenv("RESEND_API_KEY")),
		BaseURL: strings.TrimSpace(os.Getenv("RESEND_BASE_URL")),
		From: Address{
			Email: strings.TrimSpace(os.Getenv("EMAIL_FROM_EMAIL")),
			Name:  fromName,
		},
		HTTPClient: &http.Client{Timeout: timeout},
	}
}

func normalizeAddress(label string, address Address) (Address, error) {
	address.Email = strings.TrimSpace(address.Email)
	address.Name = strings.TrimSpace(address.Name)
	if address.Email == "" {
		return Address{}, fmt.Errorf("%s email is required", label)
	}
	parsed, err := mail.ParseAddress(address.Email)
	if err != nil {
		return Address{}, fmt.Errorf("invalid %s email: %w", label, err)
	}
	address.Email = parsed.Address
	if address.Name == "" {
		address.Name = parsed.Name
	}
	return address, nil
}
