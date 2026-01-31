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
// @Accept json
// @Produce json
// @Param input body dto.RegisterRequest true "Данные для регистрации"
// @Success 201 {object} dto.TokenResponse
// @Router /auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var input dto.RegisterRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.service.Register(input.Email, input.Password, input.Name, input.DeviceToken)
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
// @Accept json
// @Produce json
// @Param input body dto.LoginRequest true "Данные для входа"
// @Success 200 {object} dto.TokenResponse
// @Router /auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var input dto.LoginRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.service.Login(input.Email, input.Password, input.DeviceToken)
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
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object} dto.UserResponse
// @Router /auth/me [get]
func (h *Handler) Me(c *gin.Context) {
	userID, exists := c.Get("userId")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	idStr, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user ID format"})
		return
	}

	user, err := h.service.GetUserByID(idStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, dto.UserResponse{
		ID:    user.ID.String(),
		Email: user.Email,
		Name:  user.Name,
		Role:  user.Role,
	})
}

// Refresh godoc
// @Summary Обновление токенов
// @Tags Auth
// @Accept json
// @Produce json
// @Param input body dto.RefreshRequest true "Refresh token"
// @Success 200 {object} dto.TokenResponse
// @Router /auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var input dto.RefreshRequest

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tokens, err := h.service.RefreshTokens(input.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	c.JSON(http.StatusOK, dto.TokenResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
	})
}

// Logout godoc
// @Summary Выход из системы
// @Security ApiKeyAuth
// @Tags Auth
// @Success 200 {object} map[string]string
// @Router /auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	if err := h.service.Logout(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// RegisterRoutes реализует маршрутизацию модуля Auth
func (h *Handler) RegisterRoutes(r *gin.Engine, middleware gin.HandlerFunc) {
	router := r.Group("/api/v1/auth")
	{
		router.POST("/register", h.Register)
		router.POST("/login", h.Login)
		router.POST("/refresh", h.Refresh)
	}

	protected := r.Group("/api/v1/auth", middleware)
	{
		protected.GET("/me", h.Me)
		protected.POST("/logout", h.Logout)
	}
}
