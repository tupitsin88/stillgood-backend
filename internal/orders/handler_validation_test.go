package orders

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCreateOrderValidationReturnsStableError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewOrderHandler(nil)
	router := gin.New()
	router.POST("/orders", handler.CreateOrder)
	req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{
		"error": "OFFER_ID_REQUIRED",
		"message": "offerId is required"
	}`, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "CreateOrderRequest")
	assert.NotContains(t, rec.Body.String(), "Field validation")
}

func TestCreateReviewValidationReturnsStableError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewOrderHandler(nil)
	router := gin.New()
	router.POST("/orders/:id/review", handler.CreateReview)
	req := httptest.NewRequest(http.MethodPost, "/orders/00000000-0000-0000-0000-000000000001/review", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{
		"error": "RATING_REQUIRED",
		"message": "rating is required"
	}`, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "CreateReviewRequest")
	assert.NotContains(t, rec.Body.String(), "Field validation")
}
