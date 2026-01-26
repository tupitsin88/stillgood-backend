package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
	var input struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
		Name     string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.service.Register(input.Email, input.Password, input.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"accessToken":  tokens.AccessToken,
		"refreshToken": tokens.RefreshToken,
	})
}

// Login godoc
// @Summary Вход пользователя
// @Tags Auth
func (h *Handler) Login(c *gin.Context) {
	var input struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=8"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.service.Login(input.Email, input.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accessToken":  tokens.AccessToken,
		"refreshToken": tokens.RefreshToken,
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

	c.JSON(http.StatusOK, gin.H{
		"id": userID,
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
