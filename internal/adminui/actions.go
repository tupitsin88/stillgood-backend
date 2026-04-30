package adminui

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) ApprovePartner(c *gin.Context) {
	_, err := h.authService.ApprovePartner(c.Param("id"))
	h.redirectBack(c, defaultAdminPath, "Partner approved", actionError(err))
}

func (h *Handler) RejectPartner(c *gin.Context) {
	_, err := h.authService.RejectPartner(c.Param("id"))
	h.redirectBack(c, defaultAdminPath, "Partner rejected", actionError(err))
}

func (h *Handler) BlockUser(c *gin.Context) {
	_, err := h.authService.BlockUser(c.Param("id"))
	h.redirectBack(c, "/admin/users", "User blocked", actionError(err))
}

func (h *Handler) UnblockUser(c *gin.Context) {
	_, err := h.authService.UnblockUser(c.Param("id"))
	h.redirectBack(c, "/admin/users", "User unblocked", actionError(err))
}

func (h *Handler) DeleteReview(c *gin.Context) {
	err := h.restaurantsService.DeleteReview(c.Param("id"))
	h.redirectBack(c, "/admin/reviews", "Review deleted", actionError(err))
}

func (h *Handler) CreateCategory(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		h.redirectBack(c, "/admin/categories", "", "Category name is required")
		return
	}

	_, err := h.categoriesService.Create(name, optionalFormString(c.PostForm("iconUrl")))
	h.redirectBack(c, "/admin/categories", "Category created", actionError(err))
}

func (h *Handler) UpdateCategory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.redirectBack(c, "/admin/categories", "", "Invalid category id")
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		h.redirectBack(c, "/admin/categories", "", "Category name is required")
		return
	}

	_, err = h.categoriesService.Update(id, name, optionalFormString(c.PostForm("iconUrl")))
	h.redirectBack(c, "/admin/categories", "Category updated", actionError(err))
}

func (h *Handler) DeleteCategory(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		h.redirectBack(c, "/admin/categories", "", "Invalid category id")
		return
	}

	err = h.categoriesService.Delete(id)
	h.redirectBack(c, "/admin/categories", "Category deleted", actionError(err))
}
