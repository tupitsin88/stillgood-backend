package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"kursach_backend/internal/domain"
	emaildelivery "kursach_backend/internal/email"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sentCode struct {
	to   emaildelivery.Address
	code string
	ttl  time.Duration
}

type emailSenderSpy struct {
	err          error
	verification []sentCode
	reset        []sentCode
}

func (s *emailSenderSpy) SendEmailVerificationCode(ctx context.Context, to emaildelivery.Address, code string, ttl time.Duration) error {
	s.verification = append(s.verification, sentCode{to: to, code: code, ttl: ttl})
	return s.err
}

func (s *emailSenderSpy) SendPasswordResetCode(ctx context.Context, to emaildelivery.Address, code string, ttl time.Duration) error {
	s.reset = append(s.reset, sentCode{to: to, code: code, ttl: ttl})
	return s.err
}

type authEmailRepoStub struct {
	users          map[string]*domain.User
	verifiedEmails map[string]bool
}

func newAuthEmailRepoStub(users ...*domain.User) *authEmailRepoStub {
	repo := &authEmailRepoStub{
		users:          make(map[string]*domain.User),
		verifiedEmails: make(map[string]bool),
	}
	for _, user := range users {
		repo.users[strings.ToLower(user.Email)] = user
	}
	return repo
}

func (r *authEmailRepoStub) CreateUser(user *domain.User) error {
	r.users[strings.ToLower(user.Email)] = user
	return nil
}

func (r *authEmailRepoStub) GetUserByEmail(email string) (*domain.User, error) {
	user, ok := r.users[strings.ToLower(email)]
	if !ok {
		return nil, ErrUserNotFound
	}
	return user, nil
}

func (r *authEmailRepoStub) GetByID(id uuid.UUID) (*domain.User, error) {
	return nil, errors.New("not implemented")
}

func (r *authEmailRepoStub) IsUserBlocked(id uuid.UUID) (bool, error) {
	return false, errors.New("not implemented")
}

func (r *authEmailRepoStub) ListPartnersByStatus(status string, limit, offset int) ([]domain.User, int64, error) {
	return nil, 0, errors.New("not implemented")
}

func (r *authEmailRepoStub) ListUsersByRoles(roles []string, limit, offset int, search string) ([]domain.User, int64, error) {
	return nil, 0, errors.New("not implemented")
}

func (r *authEmailRepoStub) UpdatePartnerStatus(userID uuid.UUID, status string) error {
	return errors.New("not implemented")
}

func (r *authEmailRepoStub) UpdateBlockedStatus(userID uuid.UUID, isBlocked bool) error {
	return errors.New("not implemented")
}

func (r *authEmailRepoStub) UpdateDeviceToken(userID uuid.UUID, token string) error {
	return errors.New("not implemented")
}

func (r *authEmailRepoStub) UpdatePasswordHash(userID uuid.UUID, passwordHash string) error {
	return errors.New("not implemented")
}

func (r *authEmailRepoStub) UpdateName(userID uuid.UUID, name string) error {
	return errors.New("not implemented")
}

func (r *authEmailRepoStub) UpdatePhone(userID uuid.UUID, phone *string) error {
	return errors.New("not implemented")
}

func (r *authEmailRepoStub) UpdateVerifiedStatusByEmail(email string, isVerified bool) error {
	r.verifiedEmails[strings.ToLower(email)] = isVerified
	return nil
}

func (r *authEmailRepoStub) UpdateEmailAndResetVerification(userID uuid.UUID, email string) error {
	return errors.New("not implemented")
}

func (r *authEmailRepoStub) CountActiveOrdersByUserID(userID uuid.UUID) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *authEmailRepoStub) AnonymizeAccount(userID uuid.UUID) error {
	return errors.New("not implemented")
}

func (r *authEmailRepoStub) ExistsByEmail(email string) (bool, error) {
	_, ok := r.users[strings.ToLower(email)]
	return ok, nil
}

func TestRequestEmailVerificationSendsCodeAndAllowsVerification(t *testing.T) {
	repo := newAuthEmailRepoStub(&domain.User{
		Email: "user@example.com",
		Name:  "Alice",
	})
	sender := &emailSenderSpy{}
	service := NewService(repo, nil, time.Minute, time.Hour, "", sender)

	expiresIn, err := service.RequestEmailVerification("USER@example.com")

	require.NoError(t, err)
	assert.Equal(t, int(emailVerificationCodeTTL.Seconds()), expiresIn)
	require.Len(t, sender.verification, 1)
	assert.Equal(t, emaildelivery.Address{Email: "user@example.com", Name: "Alice"}, sender.verification[0].to)
	assert.Len(t, sender.verification[0].code, 6)
	assert.Equal(t, emailVerificationCodeTTL, sender.verification[0].ttl)

	err = service.VerifyEmail("user@example.com", sender.verification[0].code)

	require.NoError(t, err)
	assert.True(t, repo.verifiedEmails["user@example.com"])
}

func TestRequestEmailVerificationDoesNotSendForUnknownEmail(t *testing.T) {
	repo := newAuthEmailRepoStub()
	sender := &emailSenderSpy{}
	service := NewService(repo, nil, time.Minute, time.Hour, "", sender)

	expiresIn, err := service.RequestEmailVerification("missing@example.com")

	require.NoError(t, err)
	assert.Equal(t, int(emailVerificationCodeTTL.Seconds()), expiresIn)
	assert.Empty(t, sender.verification)
}

func TestRequestEmailVerificationRemovesCodeWhenSendFails(t *testing.T) {
	repo := newAuthEmailRepoStub(&domain.User{Email: "user@example.com"})
	sender := &emailSenderSpy{err: errors.New("resend unavailable")}
	service := NewService(repo, nil, time.Minute, time.Hour, "", sender)

	_, err := service.RequestEmailVerification("user@example.com")

	require.Error(t, err)
	require.Len(t, sender.verification, 1)
	err = service.VerifyEmail("user@example.com", sender.verification[0].code)
	assert.ErrorIs(t, err, ErrInvalidVerificationCode)
}

func TestForgotPasswordSendsResetCode(t *testing.T) {
	repo := newAuthEmailRepoStub(&domain.User{
		Email: "user@example.com",
		Name:  "Alice",
	})
	sender := &emailSenderSpy{}
	service := NewService(repo, nil, time.Minute, time.Hour, "", sender)

	expiresIn, err := service.ForgotPassword("user@example.com")

	require.NoError(t, err)
	assert.Equal(t, int(passwordResetCodeTTL.Seconds()), expiresIn)
	require.Len(t, sender.reset, 1)
	assert.Equal(t, emaildelivery.Address{Email: "user@example.com", Name: "Alice"}, sender.reset[0].to)
	assert.Len(t, sender.reset[0].code, 6)

	resetToken, err := service.VerifyResetCode("user@example.com", sender.reset[0].code)
	require.NoError(t, err)
	assert.NotEmpty(t, resetToken)
}
