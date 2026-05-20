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

type deviceTokenServiceStub struct {
	Service
	userID      string
	deviceToken string
	platform    string
	err         error
}

func (s *deviceTokenServiceStub) UpdateDeviceToken(userID, deviceToken, platform string) error {
	s.userID = userID
	s.deviceToken = deviceToken
	s.platform = platform
	return s.err
}

func TestUpdateDeviceTokenHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &deviceTokenServiceStub{}
	handler := NewHandler(service)
	router := gin.New()
	router.PUT("/api/v1/users/me/device-token", func(c *gin.Context) {
		c.Set("user_id", "user-id")
		handler.UpdateDeviceToken(c)
	})

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/users/me/device-token",
		strings.NewReader(`{"deviceToken":"fresh-token","platform":"android"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "user-id", service.userID)
	assert.Equal(t, "fresh-token", service.deviceToken)
	assert.Equal(t, "android", service.platform)
	assert.JSONEq(t, `{"message":"Device token updated"}`, rec.Body.String())
}

func TestUpdateDeviceTokenHandlerValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&deviceTokenServiceStub{})
	router := gin.New()
	router.PUT("/api/v1/users/me/device-token", func(c *gin.Context) {
		c.Set("user_id", "user-id")
		handler.UpdateDeviceToken(c)
	})

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/users/me/device-token",
		strings.NewReader(`{"platform":"android"}`),
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

func TestUpdateDeviceTokenHandlerMapsServiceErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&deviceTokenServiceStub{err: ErrInvalidDevicePlatform})
	router := gin.New()
	router.PUT("/api/v1/users/me/device-token", func(c *gin.Context) {
		c.Set("user_id", "user-id")
		handler.UpdateDeviceToken(c)
	})

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/users/me/device-token",
		strings.NewReader(`{"deviceToken":"fresh-token","platform":"android"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{
		"error": "INVALID_PLATFORM",
		"message": "platform must be one of: android ios"
	}`, rec.Body.String())
}

func TestUpdateDeviceTokenHandlerKeepsUnexpectedErrorsInternal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewHandler(&deviceTokenServiceStub{err: errors.New("database unavailable")})
	router := gin.New()
	router.PUT("/api/v1/users/me/device-token", func(c *gin.Context) {
		c.Set("user_id", "user-id")
		handler.UpdateDeviceToken(c)
	})

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/users/me/device-token",
		strings.NewReader(`{"deviceToken":"fresh-token","platform":"android"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.JSONEq(t, `{
		"error": "DEVICE_TOKEN_UPDATE_FAILED",
		"message": "Failed to update device token"
	}`, rec.Body.String())
}
