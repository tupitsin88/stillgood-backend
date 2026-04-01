package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func AuthMiddleware(signingKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid auth header format"})
			return
		}

		tokenString := headerParts[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(signingKey), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token is invalid", "details": err.Error()})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}

		// Достаем ID из 'sub' (как мы видели в логах)
		sub, _ := claims["sub"]
		userIDStr := fmt.Sprintf("%v", sub)

		if userIDStr == "" || userIDStr == "<nil>" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "USER_ID_NOT_FOUND_IN_TOKEN"})
			return
		}
		userUUID, _ := uuid.Parse(userIDStr)
		c.Set("user_id", userIDStr)
		c.Set("userId", userIDStr)
		if userUUID != uuid.Nil {
			c.Set("user_uuid", userUUID)
		}
		role := ""
		if r, ok := claims["role"]; ok {
			role = fmt.Sprintf("%v", r)
			c.Set("role", role)
		}
		if role == "PARTNER" {
			restID, ok := claims["restaurant_id"]
			if !ok || fmt.Sprintf("%v", restID) == "<nil>" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "RESTAURANT_ID_REQUIRED_FOR_PARTNER"})
				return
			}
			c.Set("restaurant_id", fmt.Sprintf("%v", restID))
		}
		c.Next()
	}
}
