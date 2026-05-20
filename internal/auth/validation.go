package auth

import (
	"kursach_backend/internal/pkg/httputil"

	"github.com/gin-gonic/gin"
)

func bindAuthJSON(c *gin.Context, dst any) bool {
	return httputil.BindJSON(c, dst)
}
