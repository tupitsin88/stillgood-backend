package adminui

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"kursach_backend/internal/auth"
	"kursach_backend/internal/domain"
	"kursach_backend/internal/restaurants"

	"github.com/gin-gonic/gin"
)

const (
	defaultAdminPath = "/admin/partners/pending"
	seeOther         = http.StatusSeeOther
)

func (h *Handler) base(c *gin.Context, title, active string) baseData {
	user, _ := c.Get(adminUserKey)
	adminEmail := ""
	if adminUser, ok := user.(*domain.User); ok {
		adminEmail = adminUser.Email
	}

	return baseData{
		Title:      title,
		Active:     active,
		AdminEmail: adminEmail,
		Notice:     c.Query("notice"),
		Error:      c.Query("error"),
	}
}

func (h *Handler) render(c *gin.Context, name string, data any) {
	var buf bytes.Buffer
	if err := h.templates.ExecuteTemplate(&buf, name, data); err != nil {
		c.String(http.StatusInternalServerError, "failed to render admin page")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", buf.Bytes())
}

func (h *Handler) redirectBack(c *gin.Context, fallback, notice, errorMessage string) {
	target := withoutFlashParams(safeReferer(c, fallback))
	values := url.Values{}
	if notice != "" && errorMessage == "" {
		values.Set("notice", notice)
	}
	if errorMessage != "" {
		values.Set("error", errorMessage)
	}
	if encoded := values.Encode(); encoded != "" {
		separator := "?"
		if strings.Contains(target, "?") {
			separator = "&"
		}
		target += separator + encoded
	}
	c.Redirect(seeOther, target)
}

func htmlPagination(c *gin.Context, defaultLimit int) (int, int, error) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if err != nil || limit <= 0 {
		return defaultLimit, 0, errors.New("Invalid limit")
	}
	if limit > 100 {
		limit = 100
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil || offset < 0 {
		return limit, 0, errors.New("Invalid offset")
	}
	return limit, offset, nil
}

func pagination(c *gin.Context, total int64, limit, offset int) paginationData {
	data := paginationData{
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	if offset > 0 {
		prev := offset - limit
		if prev < 0 {
			prev = 0
		}
		data.HasPrev = true
		data.PrevURL = pageURL(c, limit, prev)
	}
	if int64(offset+limit) < total {
		data.HasNext = true
		data.NextURL = pageURL(c, limit, offset+limit)
	}
	return data
}

func pageURL(c *gin.Context, limit, offset int) string {
	values := c.Request.URL.Query()
	values.Del("error")
	values.Del("notice")
	values.Set("limit", strconv.Itoa(limit))
	values.Set("offset", strconv.Itoa(offset))
	return c.Request.URL.Path + "?" + values.Encode()
}

func safeReferer(c *gin.Context, fallback string) string {
	ref := c.Request.Referer()
	if ref == "" {
		return fallback
	}

	parsed, err := url.Parse(ref)
	if err != nil {
		return fallback
	}
	if parsed.IsAbs() && parsed.Host != c.Request.Host {
		return fallback
	}
	if parsed.Path == "" || !strings.HasPrefix(parsed.Path, "/admin") || strings.HasPrefix(parsed.Path, "/admin/login") {
		return fallback
	}
	target := parsed.RequestURI()
	if target == "" {
		return fallback
	}
	return target
}

func safeAdminPath(raw, fallback string) string {
	if raw == "" {
		return fallback
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || !strings.HasPrefix(parsed.Path, "/admin") || strings.HasPrefix(parsed.Path, "/admin/login") {
		return fallback
	}
	return parsed.RequestURI()
}

func actionError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, auth.ErrCannotBlockAdmin):
		return "ADMIN users cannot be blocked"
	case errors.Is(err, auth.ErrDeletedAccount):
		return "Deleted accounts cannot be changed"
	case errors.Is(err, auth.ErrInvalidPartnerStatusTransition):
		return "Only pending partners can be approved or rejected"
	case errors.Is(err, auth.ErrPartnerNotFound), errors.Is(err, auth.ErrUserNotFound):
		return "Record not found"
	case errors.Is(err, restaurants.ErrInvalidReviewID):
		return "Invalid review id"
	case strings.Contains(err.Error(), "CANNOT_DELETE_CATEGORY_WITH_ACTIVE_OFFERS"):
		return "Cannot delete category with active offers"
	default:
		return "Action failed"
	}
}

func withoutFlashParams(target string) string {
	parsed, err := url.Parse(target)
	if err != nil {
		return target
	}
	values := parsed.Query()
	values.Del("error")
	values.Del("notice")
	parsed.RawQuery = values.Encode()
	return parsed.RequestURI()
}

func optionalFormString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
