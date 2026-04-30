package restaurants

import (
	"context"
	"math"
	"testing"

	"kursach_backend/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type adminUpdateRepo struct {
	Repository
	id  uuid.UUID
	req AdminRestaurantUpdateRequest
}

func (r *adminUpdateRepo) UpdateAdminFields(id uuid.UUID, req AdminRestaurantUpdateRequest) (*domain.Restaurant, error) {
	r.id = id
	r.req = req

	restaurant := &domain.Restaurant{
		ID:       id,
		Name:     "Test Restaurant",
		IsActive: true,
	}
	if req.Commission != nil {
		restaurant.Commission = *req.Commission
	}
	if req.IsActive != nil {
		restaurant.IsActive = *req.IsActive
	}

	return restaurant, nil
}

func TestUpdateAdminRestaurantAllowsZeroCommissionAndInactiveStatus(t *testing.T) {
	repo := &adminUpdateRepo{}
	service := NewService(repo, nil)
	restaurantID := uuid.New()
	commission := 0.0
	isActive := false

	restaurant, err := service.UpdateAdminRestaurant(restaurantID.String(), AdminRestaurantUpdateRequest{
		Commission: &commission,
		IsActive:   &isActive,
	})

	require.NoError(t, err)
	assert.Equal(t, restaurantID, repo.id)
	require.NotNil(t, repo.req.Commission)
	require.NotNil(t, repo.req.IsActive)
	assert.Equal(t, 0.0, *repo.req.Commission)
	assert.False(t, *repo.req.IsActive)
	assert.Equal(t, 0.0, restaurant.Commission)
	assert.False(t, restaurant.IsActive)
}

func TestUpdateAdminRestaurantRejectsInvalidCommission(t *testing.T) {
	testCases := []struct {
		name       string
		commission float64
	}{
		{name: "negative", commission: -0.01},
		{name: "above maximum", commission: 100.01},
		{name: "nan", commission: math.NaN()},
		{name: "infinite", commission: math.Inf(1)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &adminUpdateRepo{}
			service := NewService(repo, nil)

			restaurant, err := service.UpdateAdminRestaurant(uuid.NewString(), AdminRestaurantUpdateRequest{
				Commission: &tc.commission,
			})

			require.ErrorIs(t, err, ErrInvalidCommission)
			assert.Nil(t, restaurant)
			assert.Equal(t, uuid.Nil, repo.id)
		})
	}
}

func TestUpdateAdminRestaurantRejectsInvalidID(t *testing.T) {
	repo := &adminUpdateRepo{}
	service := NewService(repo, nil)
	commission := 10.0

	restaurant, err := service.UpdateAdminRestaurant("not-a-uuid", AdminRestaurantUpdateRequest{
		Commission: &commission,
	})

	require.ErrorIs(t, err, ErrInvalidRestaurantID)
	assert.Nil(t, restaurant)
	assert.Equal(t, uuid.Nil, repo.id)
}

type deleteReviewRepo struct {
	Repository
	deleteCalled bool
}

func (r *deleteReviewRepo) DeleteReview(_ context.Context, _ uuid.UUID) error {
	r.deleteCalled = true
	return nil
}

func TestDeleteReviewRejectsInvalidID(t *testing.T) {
	repo := &deleteReviewRepo{}
	service := NewService(repo, nil)

	err := service.DeleteReview("not-a-uuid")

	require.ErrorIs(t, err, ErrInvalidReviewID)
	assert.False(t, repo.deleteCalled)
}
