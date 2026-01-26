package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(signingKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			return
		}

		// Bearer <token>
		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || headerParts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid auth header format"})
			return
		}

		if len(headerParts[1]) == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token is empty"})
			return
		}

		tokenString := headerParts[1]

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(signingKey), nil
		})

		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			// Extract User ID
			sub, ok := claims["sub"]
			if !ok {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token invalid: sub claim missing"})
				return
			}

			// Handle sub as string or float64 (JWT numbers are floats)
			var userID int
			switch v := sub.(type) {
			case string:
				id, err := strconv.Atoi(v)
				if err != nil {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token invalid: sub is not a valid int"})
					return
				}
				userID = id
			case float64:
				userID = int(v)
			default:
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Token invalid: invalid sub type"})
				return
			}

			// Extract Role
			role, ok := claims["role"].(string)
			if !ok {
				// Role might be optional depending on logic, but requested to extract.
				// If not present, maybe default or error? Assuming error for now or empty.
			}

			c.Set("userId", userID)
			c.Set("role", role)
			c.Next()
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			return
		}
	}
}
