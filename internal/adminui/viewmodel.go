package adminui

import (
	"time"

	"kursach_backend/internal/domain"
)

type loginPageData struct {
	Title string
	Error string
	Email string
	Next  string
}

type viewData struct {
	Base             baseData
	Partners         []userRow
	Users            []userRow
	Reviews          []reviewRow
	Categories       []categoryRow
	Role             string
	RestaurantFilter string
	Pagination       paginationData
}

type baseData struct {
	Title      string
	Active     string
	AdminEmail string
	Notice     string
	Error      string
}

type paginationData struct {
	Total   int64
	Limit   int
	Offset  int
	HasPrev bool
	PrevURL string
	HasNext bool
	NextURL string
}

type userRow struct {
	ID            string
	Email         string
	Name          string
	Phone         string
	Role          string
	PartnerStatus string
	IsBlocked     bool
	AccountStatus string
	CreatedAt     string
}

type reviewRow struct {
	ID           string
	RestaurantID string
	Rating       int
	Comment      string
	UserName     string
	UserEmail    string
	CreatedAt    string
}

type categoryRow struct {
	ID      string
	Name    string
	IconURL string
}

func userRows(users []*domain.User) []userRow {
	rows := make([]userRow, 0, len(users))
	for _, user := range users {
		if user == nil {
			continue
		}
		phone := ""
		if user.Phone != nil {
			phone = *user.Phone
		}
		rows = append(rows, userRow{
			ID:            user.ID.String(),
			Email:         user.Email,
			Name:          user.Name,
			Phone:         phone,
			Role:          user.Role,
			PartnerStatus: user.PartnerStatus,
			IsBlocked:     user.IsBlocked,
			AccountStatus: accountStatus(user),
			CreatedAt:     formatTime(user.CreatedAt),
		})
	}
	return rows
}

func reviewRows(reviews []domain.Review) []reviewRow {
	rows := make([]reviewRow, 0, len(reviews))
	for _, review := range reviews {
		rows = append(rows, reviewRow{
			ID:           review.ID.String(),
			RestaurantID: review.RestaurantID.String(),
			Rating:       review.Rating,
			Comment:      review.Comment,
			UserName:     review.User.Name,
			UserEmail:    review.User.Email,
			CreatedAt:    formatTime(review.CreatedAt),
		})
	}
	return rows
}

func categoryRows(categories []domain.Category) []categoryRow {
	rows := make([]categoryRow, 0, len(categories))
	for _, category := range categories {
		iconURL := ""
		if category.IconURL != nil {
			iconURL = *category.IconURL
		}
		rows = append(rows, categoryRow{
			ID:      category.ID.String(),
			Name:    category.Name,
			IconURL: iconURL,
		})
	}
	return rows
}

func accountStatus(user *domain.User) string {
	switch {
	case user.DeletedAt != nil:
		return "deleted"
	case user.IsBlocked:
		return "blocked"
	default:
		return "active"
	}
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04")
}
