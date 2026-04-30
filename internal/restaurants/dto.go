package restaurants

import "time"

type RestaurantResponse struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Address         string   `json:"address"`
	Latitude        float64  `json:"latitude"`
	Longitude       float64  `json:"longitude"`
	LogoURL         *string  `json:"logoUrl,omitempty"`
	CoverURL        *string  `json:"coverUrl,omitempty"`
	Phone           *string  `json:"phone,omitempty"`
	Rating          float64  `json:"rating"`
	ReviewCount     int      `json:"reviewCount"`
	Categories      []string `json:"categories,omitempty"`
	Distance        *int     `json:"distance,omitempty"`
	HasActiveOffers bool     `json:"hasActiveOffers"`
	Description     *string  `json:"description,omitempty"`
	WorkingHours    string   `json:"workingHours,omitempty"`
}

type Pagination struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

type RestaurantListResponse struct {
	Data       []RestaurantResponse `json:"data"`
	Pagination Pagination           `json:"pagination"`
}

type UploadRestaurantImageResponse struct {
	Kind  string `json:"kind"`
	Field string `json:"field"`
	URL   string `json:"url"`
}

type PartnerRestaurantUpdateRequest struct {
	Description  *string `json:"description"`
	WorkingHours *string `json:"workingHours"`
	LogoURL      *string `json:"logoUrl"`
	CoverURL     *string `json:"coverUrl"`
}

type AdminRestaurantUpdateRequest struct {
	Commission *float64 `json:"commission"`
	IsActive   *bool    `json:"isActive"`
}

type AdminRestaurantResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Commission float64 `json:"commission"`
	IsActive   bool    `json:"isActive"`
}

type CreateRestaurantRequest struct {
	PartnerID    *string `json:"partnerId,omitempty"`
	Name         string  `json:"name" binding:"required"`
	CompanyName  string  `json:"companyName" binding:"required"`
	Inn          string  `json:"inn" binding:"required"`
	Address      string  `json:"address" binding:"required"`
	Description  *string `json:"description,omitempty"`
	Phone        *string `json:"phone,omitempty"`
	LogoURL      *string `json:"logoUrl,omitempty"`
	CoverURL     *string `json:"coverUrl,omitempty"`
	Latitude     float64 `json:"latitude" binding:"required"`
	Longitude    float64 `json:"longitude" binding:"required"`
	WorkingHours *string `json:"workingHours,omitempty"`
}

type ReviewDTO struct {
	ID        string    `json:"id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
}

type AdminReviewDTO struct {
	ID           string    `json:"id"`
	RestaurantID string    `json:"restaurantId"`
	Rating       int       `json:"rating"`
	Comment      string    `json:"comment"`
	UserName     string    `json:"userName"`
	UserEmail    string    `json:"userEmail"`
	CreatedAt    time.Time `json:"createdAt"`
}

type RestaurantReviewsResponse struct {
	Data       []ReviewDTO `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

type AdminReviewsResponse struct {
	Data       []AdminReviewDTO `json:"data"`
	Pagination Pagination       `json:"pagination"`
}
