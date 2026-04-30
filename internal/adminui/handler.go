package adminui

import (
	"html/template"

	"kursach_backend/internal/auth"
	"kursach_backend/internal/categories"
	"kursach_backend/internal/restaurants"

	"github.com/gin-gonic/gin"
)

const (
	adminCookieName = "admin_access_token"
	adminDevice     = "admin-web"
	adminUserKey    = "admin_user"
)

type Handler struct {
	authService        auth.Service
	categoriesService  categories.Service
	restaurantsService restaurants.Service
	jwtSecret          string
	templates          *template.Template
}

func NewHandler(authService auth.Service, categoriesService categories.Service, restaurantsService restaurants.Service, jwtSecret string) *Handler {
	return &Handler{
		authService:        authService,
		categoriesService:  categoriesService,
		restaurantsService: restaurantsService,
		jwtSecret:          jwtSecret,
		templates:          parseTemplates(),
	}
}

func RegisterRoutes(r *gin.Engine, h *Handler) {
	r.GET("/admin/assets/admin.css", h.Stylesheet)
	r.GET("/admin/login", h.LoginPage)
	r.POST("/admin/login", h.Login)

	admin := r.Group("/admin")
	admin.Use(h.requireAdmin())
	{
		admin.GET("", h.Home)
		admin.GET("/", h.Home)
		admin.POST("/logout", h.Logout)

		admin.GET("/partners/pending", h.PendingPartnersPage)
		admin.POST("/partners/:id/approve", h.ApprovePartner)
		admin.POST("/partners/:id/reject", h.RejectPartner)

		admin.GET("/users", h.UsersPage)
		admin.POST("/users/:id/block", h.BlockUser)
		admin.POST("/users/:id/unblock", h.UnblockUser)

		admin.GET("/reviews", h.ReviewsPage)
		admin.POST("/reviews/:id/delete", h.DeleteReview)

		admin.GET("/categories", h.CategoriesPage)
		admin.POST("/categories", h.CreateCategory)
		admin.POST("/categories/:id/update", h.UpdateCategory)
		admin.POST("/categories/:id/delete", h.DeleteCategory)
	}
}

func (h *Handler) Home(c *gin.Context) {
	c.Redirect(seeOther, "/admin/partners/pending")
}
