package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RoleMiddleware(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role != requiredRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "FORBIDDEN",
				"message": "Доступ запрещен: требуется роль " + requiredRole,
			})
			return
		}
		c.Next()
	}
}

func AnyRoleMiddleware(requiredRoles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(requiredRoles))
	for _, role := range requiredRoles {
		allowed[role] = struct{}{}
	}

	return func(c *gin.Context) {
		role := c.GetString("role")
		if _, ok := allowed[role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "FORBIDDEN",
				"message": "Доступ запрещен: требуется одна из разрешенных ролей",
			})
			return
		}
		c.Next()
	}
}
