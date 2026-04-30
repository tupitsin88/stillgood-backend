package restaurants

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kursach_backend/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type adminReviewsService struct {
	Service
	called       bool
	restaurantID *uuid.UUID
	limit        int
	offset       int
}

func (s *adminReviewsService) GetAdminReviews(restaurantID *uuid.UUID, limit, offset int) ([]domain.Review, int64, error) {
	s.called = true
	s.restaurantID = restaurantID
	s.limit = limit
	s.offset = offset

	return []domain.Review{
		{
			ID:           uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			RestaurantID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			Rating:       5,
			Comment:      "good",
			User: domain.User{
				Name:  "Alice",
				Email: "alice@example.com",
			},
			CreatedAt: time.Date(2026, 4, 30, 10, 0, 0, 0, time.UTC),
		},
	}, 1, nil
}

func TestGetAdminReviewListUsesPaginationAndRestaurantFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &adminReviewsService{}
	handler := NewHandler(service)
	router := gin.New()
	router.GET("/admin/reviews", handler.GetAdminReviewList)

	restaurantID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	req := httptest.NewRequest(http.MethodGet, "/admin/reviews?restaurantId="+restaurantID.String()+"&limit=10&offset=5", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.True(t, service.called)
	require.NotNil(t, service.restaurantID)
	assert.Equal(t, restaurantID, *service.restaurantID)
	assert.Equal(t, 10, service.limit)
	assert.Equal(t, 5, service.offset)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"data": [{
			"id": "11111111-1111-1111-1111-111111111111",
			"restaurantId": "22222222-2222-2222-2222-222222222222",
			"rating": 5,
			"comment": "good",
			"userName": "Alice",
			"userEmail": "alice@example.com",
			"createdAt": "2026-04-30T10:00:00Z"
		}],
		"pagination": {"total": 1, "limit": 10, "offset": 5}
	}`, rec.Body.String())
}

func TestGetAdminReviewListRejectsInvalidRestaurantFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &adminReviewsService{}
	handler := NewHandler(service)
	router := gin.New()
	router.GET("/admin/reviews", handler.GetAdminReviewList)

	req := httptest.NewRequest(http.MethodGet, "/admin/reviews?restaurantId=bad", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.False(t, service.called)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"INVALID_RESTAURANT_ID"}`, rec.Body.String())
}
