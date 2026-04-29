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
	partner, err := s.repo.GetPartnerByID(ctx, partnerID)
	if err != nil || partner.PartnerStatus != "APPROVED" {
		return nil, fmt.Errorf("PARTNER_NOT_APPROVED")
	}
	restaurant, err := s.repo.GetRestaurantByPartnerID(ctx, partnerID)
	if err != nil {
		return nil, fmt.Errorf("RESTAURANT_NOT_FOUND")
	}
	if req.Price > req.OriginalPrice {
		return nil, fmt.Errorf("INVALID_PRICE")
	}
	if req.PickupEnd.Before(req.PickupStart) {
		return nil, fmt.Errorf("INVALID_TIME_RANGE")
	}
	if req.PickupStart.Before(time.Now().Add(-5 * time.Minute)) {
		return nil, fmt.Errorf("START_TIME_IN_PAST")
	}
	if req.Quantity <= 0 {
		return nil, fmt.Errorf("INVALID_QUANTITY")
	}

	catID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("INVALID_CATEGORY_ID")
	}
	imgUrl := req.ImageURL
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
	return s.repo.GetByID(ctx, offer.ID)
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
		if *req.Price < 0 {
			return nil, fmt.Errorf("PRICE_MUST_BE_POSITIVE")
		}
		if *req.Price > offer.OriginalPrice {
			return nil, fmt.Errorf("PRICE_TOO_HIGH")
		}
		offer.Price = *req.Price
	}
	if req.OriginalPrice != nil {
		offer.OriginalPrice = *req.OriginalPrice
	}
	if req.Quantity != nil {
		if *req.Quantity <= 0 {
			return nil, fmt.Errorf("INVALID_QUANTITY")
		}
		diff := *req.Quantity - offer.QuantityTotal
		offer.QuantityAvailable += diff
		offer.QuantityTotal = *req.Quantity
		if offer.QuantityAvailable < 0 {
			offer.QuantityAvailable = 0
		}
	}
	if req.IsActive != nil {
		offer.IsActive = *req.IsActive
	}
	if req.PickupStart != nil {
		offer.PickupStart = *req.PickupStart
	}
	if req.PickupEnd != nil {
		offer.PickupEnd = *req.PickupEnd
	}
	if req.ImageURL != nil {
		offer.ImageURL = req.ImageURL
	}
	if req.CategoryID != nil {
		catID, _ := uuid.Parse(*req.CategoryID)
		offer.CategoryID = catID
	}

	if err := s.repo.Update(ctx, offer); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, offer.ID)
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
		dtos[i] = s.mapToPreviewDTO(&o)
	}
	return dtos, total, nil
}

func (s *OfferService) GetOfferByID(ctx context.Context, id uuid.UUID) (*OfferDetailDTO, error) {
	offer, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("OFFER_NOT_FOUND")
	}
	if !offer.IsActive || offer.QuantityAvailable <= 0 {
		return nil, fmt.Errorf("OFFER_NOT_AVAILABLE")
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

func (s *OfferService) mapToPreviewDTO(o *domain.Offer) OfferPreviewDTO {
	discount := 0
	if o.OriginalPrice > 0 {
		discount = int(math.Round((1 - o.Price/o.OriginalPrice) * 100))
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
		Distance:          o.DistanceMeters,
		PickupStart:       o.PickupStart,
		PickupEnd:         o.PickupEnd,
		QuantityAvailable: o.QuantityAvailable,
		Category: CategoryDTO{
			ID:   o.CategoryID.String(),
			Name: o.Category.Name,
		},
	}
}
