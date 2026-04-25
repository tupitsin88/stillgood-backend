package restaurants

type RestaurantResponse struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Address         string   `json:"address"`
	Latitude        float64  `json:"latitude"`
	Longitude       float64  `json:"longitude"`
	ImageURL        *string  `json:"imageUrl,omitempty"`
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

type PartnerRestaurantUpdateRequest struct {
	Description  *string `json:"description"`
	WorkingHours *string `json:"workingHours"`
	ImageURL     *string `json:"imageUrl"`
}

type CreateRestaurantRequest struct {
	PartnerID    *string `json:"partnerId,omitempty"`
	Name         string  `json:"name" binding:"required"`
	CompanyName  string  `json:"companyName" binding:"required"`
	Inn          string  `json:"inn" binding:"required"`
	Address      string  `json:"address" binding:"required"`
	Description  *string `json:"description,omitempty"`
	Phone        *string `json:"phone,omitempty"`
	ImageURL     *string `json:"imageUrl,omitempty"`
	Latitude     float64 `json:"latitude" binding:"required"`
	Longitude    float64 `json:"longitude" binding:"required"`
	WorkingHours *string `json:"workingHours,omitempty"`
}
