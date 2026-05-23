package offers

import (
	"bytes"
	"fmt"
	"kursach_backend/internal/domain"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUpdateOffer_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewOfferHandler(nil)
	router := gin.New()
	router.PATCH("/partner/offers/:id", handler.UpdateOffer)
	req := httptest.NewRequest(http.MethodPatch, "/partner/offers/not-a-uuid", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "INVALID_ID")
}

func TestCreateOfferValidationReturnsStableError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewOfferHandler(nil)
	router := gin.New()
	router.POST("/partner/offers", handler.CreateOffer)
	req := httptest.NewRequest(http.MethodPost, "/partner/offers", bytes.NewBufferString(`{"title":"Box"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{
		"error": "PRICE_REQUIRED",
		"message": "price is required"
	}`, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "CreateOfferRequest")
	assert.NotContains(t, rec.Body.String(), "Field validation")
}

func TestGetOfferByID_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &offerRepoStub{err: fmt.Errorf("not found")}
	service := NewOfferService(repo, nil)
	handler := NewOfferHandler(service)
	router := gin.New()
	router.GET("/offers/:id", handler.GetOfferByID)
	req := httptest.NewRequest(http.MethodGet, "/offers/"+uuid.NewString(), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "NOT_FOUND")
}

func TestCreateOffer_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	partnerID := uuid.New()
	offerID := uuid.New()
	repo := &offerRepoStub{
		partner:    &domain.User{ID: partnerID, PartnerStatus: "APPROVED"},
		restaurant: &domain.Restaurant{ID: uuid.New(), PartnerID: partnerID},
		offer: &domain.Offer{
			ID:         offerID,
			Title:      "Test Box",
			Price:      100,
			CategoryID: uuid.New(),
		},
	}
	service := NewOfferService(repo, nil)
	handler := NewOfferHandler(service)
	router := gin.New()
	router.POST("/partner/offers", func(c *gin.Context) {
		c.Set("user_id", partnerID.String())
		handler.CreateOffer(c)
	})
	body := `{"title":"Test Box","price":100,"originalPrice":200,"quantityTotal":5,"categoryId":"` + uuid.NewString() + `","pickupStart":"2026-10-10T10:00:00Z","pickupEnd":"2026-10-10T12:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/partner/offers", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), offerID.String())
	assert.Contains(t, rec.Body.String(), "Test Box")
}
