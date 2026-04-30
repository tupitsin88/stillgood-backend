package adminui

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"kursach_backend/internal/auth"
	"kursach_backend/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var errUnauthorizedAdmin = errors.New("unauthorized admin")

func (h *Handler) LoginPage(c *gin.Context) {
	h.render(c, "login", loginPageData{
		Title: "Admin login",
		Error: c.Query("error"),
		Next:  safeAdminPath(c.Query("next"), defaultAdminPath),
	})
}

func (h *Handler) Login(c *gin.Context) {
	email := c.PostForm("email")
	next := safeAdminPath(c.PostForm("next"), defaultAdminPath)

	tokens, user, err := h.authService.Login(email, c.PostForm("password"), adminDevice)
	if err != nil {
		h.renderLoginError(c, email, next, "Invalid email or password")
		return
	}
	if user.Role != auth.RoleAdmin {
		h.renderLoginError(c, email, next, "Admin role is required")
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     adminCookieName,
		Value:    tokens.AccessToken,
		Path:     "/admin",
		MaxAge:   tokens.ExpiresIn,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request.TLS != nil,
	})
	c.Redirect(seeOther, next)
}

func (h *Handler) Logout(c *gin.Context) {
	h.clearAdminCookie(c)
	c.Redirect(seeOther, "/admin/login")
}

func (h *Handler) requireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := h.adminFromRequest(c)
		if err != nil {
			h.clearAdminCookie(c)
			next := c.Request.URL.RequestURI()
			if c.Request.Method != http.MethodGet {
				next = defaultAdminPath
			}
			c.Redirect(seeOther, "/admin/login?"+url.Values{"next": {safeAdminPath(next, defaultAdminPath)}}.Encode())
			c.Abort()
			return
		}

		c.Set(adminUserKey, user)
		c.Next()
	}
}

func (h *Handler) adminFromRequest(c *gin.Context) (*domain.User, error) {
	rawToken, err := c.Cookie(adminCookieName)
	if err != nil || rawToken == "" {
		return nil, errUnauthorizedAdmin
	}

	token, err := jwt.Parse(rawToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(h.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, errUnauthorizedAdmin
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || fmt.Sprintf("%v", claims["role"]) != auth.RoleAdmin {
		return nil, errUnauthorizedAdmin
	}

	userID := fmt.Sprintf("%v", claims["sub"])
	if userID == "" || userID == "<nil>" {
		return nil, errUnauthorizedAdmin
	}

	blocked, err := h.authService.IsUserBlocked(userID)
	if err != nil || blocked {
		return nil, errUnauthorizedAdmin
	}

	user, err := h.authService.GetUserByID(userID)
	if err != nil || user.Role != auth.RoleAdmin {
		return nil, errUnauthorizedAdmin
	}

	return user, nil
}

func (h *Handler) clearAdminCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     adminCookieName,
		Value:    "",
		Path:     "/admin",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   c.Request.TLS != nil,
	})
}

func (h *Handler) renderLoginError(c *gin.Context, email, next, message string) {
	h.render(c, "login", loginPageData{
		Title: "Admin login",
		Error: message,
		Email: email,
		Next:  next,
	})
}
