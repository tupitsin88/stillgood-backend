package adminui

import (
	"errors"
	"strings"

	"kursach_backend/internal/auth"
	"kursach_backend/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *Handler) PendingPartnersPage(c *gin.Context) {
	limit, offset, pageErr := htmlPagination(c, 20)
	partners, total, err := h.authService.ListPendingPartners(limit, offset)

	data := viewData{
		Base:       h.base(c, "Pending partners", "partners"),
		Partners:   userRows(partners),
		Pagination: pagination(c, total, limit, offset),
	}
	setPageError(&data, pageErr, err, "Failed to load pending partners")

	h.render(c, "partners", data)
}

func (h *Handler) UsersPage(c *gin.Context) {
	limit, offset, pageErr := htmlPagination(c, 20)
	role := strings.TrimSpace(c.Query("role"))
	users, total, err := h.authService.ListUsers(limit, offset, role)

	data := viewData{
		Base:       h.base(c, "Users", "users"),
		Users:      userRows(users),
		Role:       role,
		Pagination: pagination(c, total, limit, offset),
	}
	if errors.Is(err, auth.ErrInvalidUserRoleFilter) {
		err = errors.New("Invalid role filter")
	}
	setPageError(&data, pageErr, err, "Failed to load users")

	h.render(c, "users", data)
}

func (h *Handler) ReviewsPage(c *gin.Context) {
	limit, offset, pageErr := htmlPagination(c, 20)
	restaurantFilter := strings.TrimSpace(c.Query("restaurantId"))
	restaurantID, filterErr := parseOptionalUUID(restaurantFilter)

	var (
		reviews []domain.Review
		total   int64
		err     error
	)
	if filterErr == nil {
		reviews, total, err = h.restaurantsService.GetAdminReviews(restaurantID, limit, offset)
	}

	data := viewData{
		Base:             h.base(c, "Reviews", "reviews"),
		Reviews:          reviewRows(reviews),
		RestaurantFilter: restaurantFilter,
		Pagination:       pagination(c, total, limit, offset),
	}
	setPageError(&data, pageErr, firstErr(filterErr, err), "Failed to load reviews")

	h.render(c, "reviews", data)
}

func (h *Handler) CategoriesPage(c *gin.Context) {
	items, err := h.categoriesService.GetAll()
	data := viewData{
		Base:       h.base(c, "Categories", "categories"),
		Categories: categoryRows(items),
	}
	setPageError(&data, nil, err, "Failed to load categories")

	h.render(c, "categories", data)
}

func setPageError(data *viewData, pageErr, err error, fallback string) {
	switch {
	case pageErr != nil:
		data.Base.Error = pageErr.Error()
	case err != nil:
		if err.Error() != "" && err.Error() != "record not found" {
			data.Base.Error = err.Error()
			return
		}
		data.Base.Error = fallback
	}
}

func parseOptionalUUID(value string) (*uuid.UUID, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil, errors.New("Invalid restaurant id")
	}
	return &parsed, nil
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
