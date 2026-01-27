package orders

import (
	"context"
	"fmt"
	"kursach_backend/internal/domain"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

type OrderService struct {
	repo *OrderRepository
}

func NewOrderService(repo *OrderRepository) *OrderService {
	return &OrderService{repo: repo}
}

func (s *OrderService) CreateOrder(ctx context.Context, userID uuid.UUID, req CreateOrderRequest) (*domain.Order, error) {
	offerUUID, err := uuid.Parse(req.OfferID)
	if err != nil {
		return nil, fmt.Errorf("invalid offer id")
	}
	offer, err := s.repo.GetOfferByID(ctx, offerUUID)
	if err != nil {
		return nil, fmt.Errorf("offer not found")
	}
	if offer.QuantityAvailable <= 0 {
		return nil, fmt.Errorf("INSUFFICIENT_QUANTITY")
	}
	amount := offer.Price
	expiresAt := time.Now().Add(15 * time.Minute)

	order := &domain.Order{
		UserID:    userID,
		OfferID:   offerUUID,
		Status:    domain.OrderCreated,
		Amount:    amount,
		CreatedAt: time.Now(),
		ExpiresAt: &expiresAt,
	}
	// (В идеале это должно быть в транзакции БД, но для MVP сделаем последовательно)
	if err := s.repo.UpdateOfferQuantity(ctx, offer.ID, -1); err != nil {
		return nil, err
	}
	if err := s.repo.CreateOrder(ctx, order); err != nil {
		s.repo.UpdateOfferQuantity(ctx, offer.ID, 1)
		return nil, err
	}
	order.Offer = *offer
	return order, nil
}

func (s *OrderService) PayOrder(ctx context.Context, orderID, userID uuid.UUID) (*domain.Order, error) {
	order, err := s.repo.GetByIDWithDetails(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("not found")
	}

	if order.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}
	if order.Status != domain.OrderCreated {
		return nil, fmt.Errorf("INVALID_ORDER_STATUS")
	}
	if order.ExpiresAt != nil && time.Now().After(*order.ExpiresAt) {
		return nil, fmt.Errorf("ORDER_EXPIRED")
	}

	num := fmt.Sprintf("%06d", rand.Intn(1000000))

	now := time.Now()
	order.Status = domain.OrderPaid
	order.PaidAt = &now
	order.OrderNumber = &num

	if err := s.repo.Update(ctx, order); err != nil {
		return nil, err
	}

	return order, nil
}

func (s *OrderService) CancelOrder(ctx context.Context, orderID, userID uuid.UUID, reason string) (*domain.Order, float64, error) {
	order, err := s.repo.GetByIDWithDetails(ctx, orderID)
	if err != nil {
		return nil, 0, fmt.Errorf("not found")
	}
	if order.UserID != userID {
		return nil, 0, fmt.Errorf("unauthorized")
	}

	refundAmount := 0.0

	if order.Status == domain.OrderPaid {
		if order.PaidAt != nil && time.Since(*order.PaidAt) > 1*time.Hour {
			return nil, 0, fmt.Errorf("CANCELLATION_WINDOW_CLOSED")
		}
		refundAmount = order.Amount
	} else if order.Status != domain.OrderCreated {
		return nil, 0, fmt.Errorf("CANNOT_CANCEL")
	}

	now := time.Now()
	order.Status = domain.OrderCancelled
	order.CancelledAt = &now
	order.CancellationReason = &reason

	if err := s.repo.Update(ctx, order); err != nil {
		return nil, 0, err
	}

	if err := s.repo.UpdateOfferQuantity(ctx, order.OfferID, 1); err != nil {
		fmt.Printf("Failed to return quantity for order %d: %v\n", order.ID, err)
	}
	return order, refundAmount, nil
}

func (s *OrderService) CompleteOrder(ctx context.Context, orderID uuid.UUID) (*domain.Order, error) {
	order, err := s.repo.GetByIDWithDetails(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("not found")
	}

	if order.Status != domain.OrderPaid {
		return nil, fmt.Errorf("INVALID_ORDER_STATUS")
	}

	now := time.Now()
	order.Status = domain.OrderCompleted
	order.CompletedAt = &now

	if err := s.repo.Update(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *OrderService) GetOrderById(ctx context.Context, orderID, userID uuid.UUID) (*domain.Order, error) {
	order, err := s.repo.GetByIDWithDetails(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("not found")
	}
	if order.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}
	return order, nil
}
