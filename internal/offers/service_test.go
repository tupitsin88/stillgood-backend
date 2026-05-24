package offers

import (
	"context"
	"testing"
	"time"

	"kursach_backend/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type offerRepoStub struct {
	OfferRepository
	partner      *domain.User
	restaurant   *domain.Restaurant
	offer        *domain.Offer
	publicParams []FilterParams
	publicOffers []domain.Offer
	publicTotal  int64
	err          error
}

func (s *offerRepoStub) GetPartnerByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return s.partner, s.err
}

func (s *offerRepoStub) GetRestaurantByPartnerID(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error) {
	return s.restaurant, s.err
}

func (s *offerRepoStub) Create(ctx context.Context, offer *domain.Offer) error {
	return s.err
}

func (s *offerRepoStub) GetByID(ctx context.Context, id uuid.UUID) (*domain.Offer, error) {
	return s.offer, s.err
}

func (s *offerRepoStub) Update(ctx context.Context, offer *domain.Offer) error {
	return s.err
}

func (s *offerRepoStub) GetPartnerOffers(ctx context.Context, restaurantID uuid.UUID, limit, offset int) ([]domain.Offer, int64, error) {
	return nil, 0, s.err
}

func (s *offerRepoStub) GetPublicOffers(ctx context.Context, params FilterParams) ([]domain.Offer, int64, error) {
	s.publicParams = append(s.publicParams, params)
	return s.publicOffers, s.publicTotal, s.err
}

func (s *offerRepoStub) Delete(ctx context.Context, id uuid.UUID) error {
	return s.err
}

func TestCreateOffer_Validation(t *testing.T) {
	partnerID := uuid.New()
	restID := uuid.New()

	approvedPartner := &domain.User{ID: partnerID, PartnerStatus: "APPROVED"}
	activeRest := &domain.Restaurant{ID: restID, PartnerID: partnerID}

	repo := &offerRepoStub{partner: approvedPartner, restaurant: activeRest}
	service := NewOfferService(repo, nil)
	t.Run("Rejects invalid price (price > original)", func(t *testing.T) {
		req := CreateOfferRequest{
			Price:         500,
			OriginalPrice: 400,
			PickupStart:   time.Now().Add(time.Hour),
			PickupEnd:     time.Now().Add(2 * time.Hour),
			QuantityTotal: 10,
			CategoryID:    uuid.NewString(),
		}
		_, err := service.CreateOffer(context.Background(), partnerID, req)
		assert.Error(t, err)
		assert.Equal(t, "INVALID_PRICE", err.Error())
	})
	t.Run("Rejects invalid time range (end < start)", func(t *testing.T) {
		repo := &offerRepoStub{partner: approvedPartner, restaurant: activeRest}
		service := NewOfferService(nil, nil)
		service.repo = repo
		req := CreateOfferRequest{
			Price:         100,
			OriginalPrice: 200,
			PickupStart:   time.Now().Add(2 * time.Hour),
			PickupEnd:     time.Now().Add(1 * time.Hour),
			QuantityTotal: 5,
			CategoryID:    uuid.NewString(),
		}
		_, err := service.CreateOffer(context.Background(), partnerID, req)
		assert.Error(t, err)
		assert.Equal(t, "INVALID_TIME_RANGE", err.Error())
	})
}

func TestUpdateOffer_Security(t *testing.T) {
	offerID := uuid.New()
	ownerID := uuid.New()
	otherPartnerID := uuid.New()

	existingOffer := &domain.Offer{
		ID:           offerID,
		RestaurantID: uuid.New(),
		Restaurant:   domain.Restaurant{PartnerID: ownerID},
	}

	t.Run("Fails if partner is not the owner", func(t *testing.T) {
		repo := &offerRepoStub{offer: existingOffer}
		service := NewOfferService(nil, nil)
		service.repo = repo

		req := UpdateOfferRequest{}
		_, err := service.UpdateOffer(context.Background(), offerID, otherPartnerID, req)

		assert.Error(t, err)
		assert.Equal(t, "FORBIDDEN", err.Error())
	})
}

func TestUpdateOffer_QuantityLogic(t *testing.T) {
	offerID := uuid.New()
	partnerID := uuid.New()
	existingOffer := &domain.Offer{
		ID:                offerID,
		QuantityTotal:     10,
		QuantityAvailable: 10,
		Restaurant:        domain.Restaurant{PartnerID: partnerID},
	}

	t.Run("Successfully updates total and available quantity", func(t *testing.T) {
		offer := *existingOffer
		repo := &offerRepoStub{offer: &offer}
		service := NewOfferService(repo, nil)

		newTotal := 15
		newAvailable := 12
		req := UpdateOfferRequest{
			QuantityTotal:     &newTotal,
			QuantityAvailable: &newAvailable,
		}
		updated, err := service.UpdateOffer(context.Background(), offerID, partnerID, req)

		require.NoError(t, err)
		assert.Equal(t, 15, updated.QuantityTotal)
		assert.Equal(t, 12, updated.QuantityAvailable)
	})

	t.Run("Rejects when available is greater than total", func(t *testing.T) {
		offer := *existingOffer
		repo := &offerRepoStub{offer: &offer}
		service := NewOfferService(repo, nil)
		newAvailable := 15
		req := UpdateOfferRequest{QuantityAvailable: &newAvailable}
		_, err := service.UpdateOffer(context.Background(), offerID, partnerID, req)
		require.Error(t, err)
		assert.Equal(t, "INVALID_QUANTITY", err.Error())
	})

	t.Run("Rejects negative available quantity", func(t *testing.T) {
		offer := *existingOffer
		repo := &offerRepoStub{offer: &offer}
		service := NewOfferService(repo, nil)

		newAvailable := -1
		req := UpdateOfferRequest{QuantityAvailable: &newAvailable}

		_, err := service.UpdateOffer(context.Background(), offerID, partnerID, req)

		require.Error(t, err)
		assert.Equal(t, "INVALID_QUANTITY", err.Error())
	})
}

func TestUpdateOffer_RejectsInvalidCategoryID(t *testing.T) {
	offerID := uuid.New()
	partnerID := uuid.New()
	existingOffer := &domain.Offer{
		ID:         offerID,
		Restaurant: domain.Restaurant{PartnerID: partnerID},
	}
	repo := &offerRepoStub{offer: existingOffer}
	service := NewOfferService(repo, nil)
	badCategoryID := "not-a-uuid"

	_, err := service.UpdateOffer(context.Background(), offerID, partnerID, UpdateOfferRequest{
		CategoryID: &badCategoryID,
	})

	require.Error(t, err)
	assert.Equal(t, "INVALID_CATEGORY_ID", err.Error())
}

func TestGetOfferByID_Availability(t *testing.T) {
	offerID := uuid.New()

	t.Run("Returns error for out-of-stock offer", func(t *testing.T) {
		repo := &offerRepoStub{offer: &domain.Offer{
			ID:                offerID,
			QuantityAvailable: 0,
			IsActive:          true,
		}}
		service := NewOfferService(repo, nil)

		_, err := service.GetOfferByID(context.Background(), offerID)
		assert.Error(t, err)
		assert.Equal(t, "OFFER_NOT_AVAILABLE", err.Error())
	})

	t.Run("Returns error for inactive offer", func(t *testing.T) {
		repo := &offerRepoStub{offer: &domain.Offer{
			ID:                offerID,
			QuantityAvailable: 10,
			IsActive:          false,
		}}
		service := NewOfferService(repo, nil)

		_, err := service.GetOfferByID(context.Background(), offerID)
		assert.Error(t, err)
		assert.Equal(t, "OFFER_NOT_AVAILABLE", err.Error())
	})
}
