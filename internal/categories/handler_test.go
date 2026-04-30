package categories

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type deleteCategoryService struct {
	Service
	deleteCalled bool
}

func (s *deleteCategoryService) Delete(id uuid.UUID) error {
	s.deleteCalled = true
	return nil
}

func TestDeleteRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &deleteCategoryService{}
	handler := NewHandler(service)
	router := gin.New()
	router.DELETE("/categories/:id", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/categories/not-a-uuid", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"INVALID_ID"}`, rec.Body.String())
	assert.False(t, service.deleteCalled)
}
