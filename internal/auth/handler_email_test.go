package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type forgotPasswordServiceStub struct {
	Service
	expiresIn int
	err       error
}

func (s forgotPasswordServiceStub) ForgotPassword(_ string) (int, error) {
	return s.expiresIn, s.err
}

func TestForgotPasswordReturnsServiceUnavailableWhenEmailDeliveryFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(forgotPasswordServiceStub{err: ErrEmailDeliveryFailed})
	router := gin.New()
	router.POST("/auth/forgot-password", handler.ForgotPassword)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/forgot-password",
		strings.NewReader(`{"email":"user@example.com"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.JSONEq(t, `{
		"error": "EMAIL_DELIVERY_FAILED",
		"message": "Failed to send password reset code"
	}`, rec.Body.String())
}

func TestForgotPasswordReturnsTooManyRequestsWhenCodeWasSentRecently(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(forgotPasswordServiceStub{err: ErrVerificationCodeTooFrequent})
	router := gin.New()
	router.POST("/auth/forgot-password", handler.ForgotPassword)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/forgot-password",
		strings.NewReader(`{"email":"user@example.com"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.JSONEq(t, `{
		"error": "TOO_MANY_REQUESTS",
		"message": "Please wait at least 1 minute before requesting a new code"
	}`, rec.Body.String())
}

func TestForgotPasswordKeepsUnexpectedErrorsInternal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(forgotPasswordServiceStub{err: errors.New("database unavailable")})
	router := gin.New()
	router.POST("/auth/forgot-password", handler.ForgotPassword)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/forgot-password",
		strings.NewReader(`{"email":"user@example.com"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{"error":"Failed to process forgot password"}`, rec.Body.String())
}
