package offers

import (
	"context"
	"fmt"
	"kursach_backend/internal/domain"
	"math"
	"time"

	"github.com/google/uuid"
)

type OfferService struct {
	repo *OfferRepository
}

func NewOfferService(repo *OfferRepository) *OfferService {
	return &OfferService{repo: repo}
}

func (s *OfferService) CreateOffer(ctx context.Context, partnerID uuid.UUID, req CreateOfferRequest) (*domain.Offer, error) {
	restaurant, err := s.repo.GetRestaurantByPartnerID(ctx, partnerID)
	if err != nil {
		return nil, fmt.Errorf("RESTAURANT_NOT_FOUND")
	}
	if req.Price >= req.OriginalPrice {
		return nil, fmt.Errorf("INVALID_PRICE")
	}
	if req.PickupEnd.Before(req.PickupStart) {
		return nil, fmt.Errorf("INVALID_TIME_RANGE")
	}
	if req.PickupStart.Before(time.Now().Add(-5 * time.Minute)) {
		return nil, fmt.Errorf("START_TIME_IN_PAST")
	}

	imgUrl := req.ImageURL

	catID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("INVALID_CATEGORY_ID")
	}
	offer := &domain.Offer{
		RestaurantID:      restaurant.ID,
		Title:             req.Title,
		Description:       req.Description,
		CategoryID:        catID,
		Price:             req.Price,
		OriginalPrice:     req.OriginalPrice,
		QuantityAvailable: req.Quantity,
		QuantityTotal:     req.Quantity,
		PickupStart:       req.PickupStart,
		PickupEnd:         req.PickupEnd,
		ImageURL:          &imgUrl,
		IsActive:          true,
		CreatedAt:         time.Now(),
	}

	if err := s.repo.Create(ctx, offer); err != nil {
		return nil, err
	}

	fullOffer, err := s.repo.GetByID(ctx, offer.ID)
	if err != nil {
		offer.Restaurant = *restaurant
		return offer, nil
	}
	return fullOffer, nil
}

func (s *OfferService) UpdateOffer(ctx context.Context, id, partnerID uuid.UUID, req UpdateOfferRequest) (*domain.Offer, error) {
	offer, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("OFFER_NOT_FOUND")
	}

	if offer.Restaurant.PartnerID != partnerID {
		return nil, fmt.Errorf("FORBIDDEN")
	}

	if req.Title != nil {
		offer.Title = *req.Title
	}
	if req.Description != nil {
		offer.Description = *req.Description
	}
	if req.Price != nil {
		if *req.Price >= offer.OriginalPrice {
			return nil, fmt.Errorf("INVALID_PRICE")
		}
		offer.Price = *req.Price
	}
	if req.Quantity != nil {
		offer.QuantityAvailable = *req.Quantity
		offer.QuantityTotal = *req.Quantity
	}
	if req.IsActive != nil {
		offer.IsActive = *req.IsActive
	}

	if err := s.repo.Update(ctx, offer); err != nil {
		return nil, err
	}
	return offer, nil
}

func (s *OfferService) GetPartnerOffers(ctx context.Context, partnerID uuid.UUID, limit, offset int) ([]OfferDetailDTO, int64, error) {
	restaurant, err := s.repo.GetRestaurantByPartnerID(ctx, partnerID)
	if err != nil {
		return nil, 0, fmt.Errorf("RESTAURANT_NOT_FOUND")
	}

	offers, total, err := s.repo.GetPartnerOffers(ctx, restaurant.ID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]OfferDetailDTO, len(offers))
	for i, o := range offers {
		dtos[i] = s.mapToDetailDTO(&o)
	}
	return dtos, total, nil
}

func (s *OfferService) GetPublicOffers(ctx context.Context, params FilterParams) ([]OfferPreviewDTO, int64, error) {
	offers, total, err := s.repo.GetPublicOffers(ctx, params)
	if err != nil {
		return nil, 0, err
	}

	dtos := make([]OfferPreviewDTO, len(offers))
	for i, o := range offers {
		dtos[i] = s.mapToPreviewDTO(&o, params.Lat, params.Lng)
	}
	return dtos, total, nil
}

func (s *OfferService) GetOfferByID(ctx context.Context, id uuid.UUID) (*OfferDetailDTO, error) {
	offer, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("OFFER_NOT_FOUND")
	}
	dto := s.mapToDetailDTO(offer)
	return &dto, nil
}

func (s *OfferService) DeleteOffer(ctx context.Context, id, partnerID uuid.UUID) error {
	offer, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("OFFER_NOT_FOUND")
	}
	if offer.Restaurant.PartnerID != partnerID {
		return fmt.Errorf("FORBIDDEN")
	}
	return s.repo.Delete(ctx, id)
}

func (s *OfferService) mapToDetailDTO(o *domain.Offer) OfferDetailDTO {
	discount := 0
	if o.OriginalPrice > 0 {
		discount = int(math.Round((1 - o.Price/o.OriginalPrice) * 100))
	}
	return OfferDetailDTO{
		ID:                o.ID.String(),
		Title:             o.Title,
		Description:       o.Description,
		Price:             o.Price,
		OriginalPrice:     o.OriginalPrice,
		Discount:          discount,
		ImageURL:          o.ImageURL,
		QuantityAvailable: o.QuantityAvailable,
		QuantityTotal:     o.QuantityTotal,
		PickupStart:       o.PickupStart,
		PickupEnd:         o.PickupEnd,
		IsActive:          o.IsActive,
		Category:          CategoryDTO{ID: o.CategoryID.String(), Name: o.Category.Name},
		Restaurant: RestaurantShortDTO{
			ID:        o.Restaurant.ID.String(),
			Name:      o.Restaurant.Name,
			Address:   o.Restaurant.Address,
			Latitude:  o.Restaurant.Latitude,
			Longitude: o.Restaurant.Longitude,
			Phone:     o.Restaurant.Phone,
		},
	}
}

func (s *OfferService) mapToPreviewDTO(o *domain.Offer, userLat, userLng *float64) OfferPreviewDTO {
	discount := 0
	if o.OriginalPrice > 0 {
		discount = int(math.Round((1 - o.Price/o.OriginalPrice) * 100))
	}

	var dist *int
	if userLat != nil && userLng != nil {
		d := calculateDistance(*userLat, *userLng, o.Restaurant.Latitude, o.Restaurant.Longitude)
		val := int(d)
		dist = &val
	}

	return OfferPreviewDTO{
		ID:                o.ID.String(),
		Title:             o.Title,
		Price:             o.Price,
		OriginalPrice:     o.OriginalPrice,
		Discount:          discount,
		ImageURL:          o.ImageURL,
		RestaurantID:      o.RestaurantID.String(),
		RestaurantName:    o.Restaurant.Name,
		Distance:          dist,
		PickupStart:       o.PickupStart,
		PickupEnd:         o.PickupEnd,
		QuantityAvailable: o.QuantityAvailable,
		Category: CategoryDTO{
			ID:   o.CategoryID.String(),
			Name: o.Category.Name,
		},
	}
}

func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	deltaPhi := (lat2 - lat1) * math.Pi / 180
	deltaLambda := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaPhi/2)*math.Sin(deltaPhi/2) +
		math.Cos(phi1)*math.Cos(phi2)*
			math.Sin(deltaLambda/2)*math.Sin(deltaLambda/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}
