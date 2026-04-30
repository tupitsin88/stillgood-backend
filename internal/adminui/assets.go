package adminui

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) Stylesheet(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.FileFromFS("assets/admin.css", http.FS(adminFS))
}
