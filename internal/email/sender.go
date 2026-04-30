package email

import (
	"context"
	"time"
)

type Address struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type Sender interface {
	SendEmailVerificationCode(ctx context.Context, to Address, code string, ttl time.Duration) error
	SendPasswordResetCode(ctx context.Context, to Address, code string, ttl time.Duration) error
}
