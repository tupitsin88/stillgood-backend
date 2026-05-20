package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAnyRoleMiddlewareAllowsConfiguredRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/upload",
		func(c *gin.Context) {
			c.Set("role", c.Query("role"))
		},
		AnyRoleMiddleware("PARTNER", "ADMIN"),
		func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		},
	)

	for _, role := range []string{"PARTNER", "ADMIN"} {
		t.Run(role, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/upload?role="+role, nil)

			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusNoContent, rec.Code)
		})
	}
}

func TestAnyRoleMiddlewareRejectsOtherRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/upload",
		func(c *gin.Context) {
			c.Set("role", "USER")
		},
		AnyRoleMiddleware("PARTNER", "ADMIN"),
		func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/upload", nil)

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
