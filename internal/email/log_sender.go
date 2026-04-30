package email

import (
	"context"
	"log"
	"time"
)

type LogSender struct{}

func (LogSender) SendEmailVerificationCode(ctx context.Context, to Address, code string, ttl time.Duration) error {
	log.Printf("[Email] verification code for %s: %s", to.Email, code)
	return nil
}

func (LogSender) SendPasswordResetCode(ctx context.Context, to Address, code string, ttl time.Duration) error {
	log.Printf("[Email] password reset code for %s: %s", to.Email, code)
	return nil
}
