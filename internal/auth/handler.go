package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"kursach_backend/internal/auth/dto"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// Register godoc
// @Summary Регистрация пользователя
// @Tags Auth
func (h *Handler) Register(c *gin.Context) {
	var input dto.RegisterRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.service.Register(input.Email, input.Password, input.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register"})
		return
	}

	c.JSON(http.StatusCreated, dto.TokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// Login godoc
// @Summary Вход пользователя
// @Tags Auth
func (h *Handler) Login(c *gin.Context) {
	var input dto.LoginRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.service.Login(input.Email, input.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, dto.TokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// Me godoc
// @Summary Получение текущего пользователя
// @Tags Auth
func (h *Handler) Me(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	role, _ := c.Get("role")

	c.JSON(http.StatusOK, dto.UserResponse{
		ID:   userID.(int),
		Role: role.(string),
		// Email will be added later when we fetch full user from DB if needed
		// For now we fulfill the struct requirements with available context data
	})
}

// InitRoutes регистрирует пути
func (h *Handler) InitRoutes(api *gin.RouterGroup, middleware gin.HandlerFunc) {
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/register", h.Register)
		authGroup.POST("/login", h.Login)

		protected := authGroup.Group("/", middleware)

		protected.GET("/me", h.Me)
	}
}
