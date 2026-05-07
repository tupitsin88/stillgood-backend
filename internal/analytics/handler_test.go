package analytics

import (
	"kursach_backend/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGetPartnerAnalytics_Handler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Returns 401 if user_id is missing in context", func(t *testing.T) {
		handler := NewAnalyticsHandler(nil)
		router := gin.New()
		router.GET("/analytics", handler.GetPartnerAnalytics)

		req := httptest.NewRequest(http.MethodGet, "/analytics", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Returns 403 if restaurant not found", func(t *testing.T) {
		// Репозиторий вернет ошибку, что ресторана нет
		repo := &analyticsRepoStub{err: http.ErrNoLocation}
		service := NewAnalyticsService(repo)
		handler := NewAnalyticsHandler(service)

		router := gin.New()
		router.GET("/analytics", func(c *gin.Context) {
			c.Set("user_id", uuid.NewString()) // Эмулируем авторизацию
			handler.GetPartnerAnalytics(c)
		})

		req := httptest.NewRequest(http.MethodGet, "/analytics", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "RESTAURANT_NOT_FOUND")
	})

	t.Run("Returns 400 for invalid groupBy parameter", func(t *testing.T) {
		repo := &analyticsRepoStub{rest: &domain.Restaurant{ID: uuid.New()}}
		service := NewAnalyticsService(repo)
		handler := NewAnalyticsHandler(service)

		router := gin.New()
		router.GET("/analytics", func(c *gin.Context) {
			c.Set("user_id", uuid.NewString())
			handler.GetPartnerAnalytics(c)
		})

		req := httptest.NewRequest(http.MethodGet, "/analytics?groupBy=year", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "INVALID_GROUP_BY")
	})
}

func TestGetPartnerAnalytics_InvalidDates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := &analyticsRepoStub{rest: &domain.Restaurant{ID: uuid.New()}}
	service := NewAnalyticsService(repo)
	handler := NewAnalyticsHandler(service)

	router := gin.New()
	router.GET("/analytics", func(c *gin.Context) {
		c.Set("user_id", uuid.NewString())
		handler.GetPartnerAnalytics(c)
	})

	t.Run("Uses default dates if format is wrong", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/analytics?startDate=12-05-2026", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
