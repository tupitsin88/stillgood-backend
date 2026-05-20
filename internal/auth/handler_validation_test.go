package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestLoginValidationReturnsStableErrorCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(nil)
	router := gin.New()
	router.POST("/auth/login", handler.Login)

	req := httptest.NewRequest(
		http.MethodPost,
		"/auth/login",
		strings.NewReader(`{"email":"partner@example.com","password":"Password!1"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{
		"error": "DEVICE_TOKEN_REQUIRED",
		"message": "deviceToken is required"
	}`, rec.Body.String())
}

func TestLoginValidationDoesNotExposeValidatorText(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(nil)
	router := gin.New()
	router.POST("/auth/login", handler.Login)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"partner@example.com"`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{
		"error": "INVALID_JSON",
		"message": "Request body must be valid JSON"
	}`, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "LoginRequest")
	assert.NotContains(t, rec.Body.String(), "Field validation")
}
