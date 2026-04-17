package orders

import (
	"context"
	"fmt"
	"kursach_backend/internal/domain"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

type NotificationProvider interface {
	Send(ctx context.Context, userID uuid.UUID, message string) error
}

type LogNotificationProvider struct{}

func (l LogNotificationProvider) Send(ctx context.Context, userID uuid.UUID, message string) error {
	log.Printf("[NOTIFICATION STUB] Отправка пользователю %s: %s", userID, message)
	return nil
}

type OrderService struct {
	repo     *OrderRepository
	notifier NotificationProvider
}

func NewOrderService(repo *OrderRepository, notifier NotificationProvider) *OrderService {
	return &OrderService{
		repo:     repo,
		notifier: notifier,
	}
}

func (s *OrderService) StartExpirationWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cancelExpiredOrders(ctx)
		}
	}
}

func (s *OrderService) cancelExpiredOrders(ctx context.Context) {
	expired, err := s.repo.GetExpiredOrders(ctx)
	if err != nil {
		log.Printf("[Cron] Failed to fetch expired orders: %v", err)
		return
	}

	for _, order := range expired {
		log.Printf("[Cron] Expiring order %s...", order.ID)

		reason := "Payment timeout"
		now := time.Now()
		order.Status = domain.OrderCancelled
		order.CancelledAt = &now
		order.CancellationReason = &reason

		err = s.repo.Transaction(func(txRepo *OrderRepository) error {
			if err := txRepo.Update(ctx, &order); err != nil {
				return err
			}
			if err := txRepo.UpdateOfferQuantity(ctx, order.OfferID, 1); err != nil {
				return err
			}
			history := domain.OrderStatusHistory{
				ID:        uuid.New(),
				OrderID:   order.ID,
				Status:    domain.OrderCancelled,
				ChangedAt: now,
			}
			return txRepo.SaveHistory(ctx, history)
		})

		if err != nil {
			log.Printf("[Cron] Failed to expire order %s: %v", order.ID, err)
		}
	}
}

func (s *OrderService) CreateOrder(ctx context.Context, userID uuid.UUID, req CreateOrderRequest) (*domain.Order, error) {
	offerID, err := uuid.Parse(req.OfferID)
	if err != nil {
		return nil, fmt.Errorf("INVALID_OFFER_ID")
	}
	var finalOrder *domain.Order
	err = s.repo.Transaction(func(txRepo *OrderRepository) error {
		offer, err := txRepo.GetOfferByIDForUpdate(ctx, offerID)
		if err != nil {
			return fmt.Errorf("OFFER_NOT_FOUND")
		}
		if offer.QuantityAvailable <= 0 {
			return fmt.Errorf("OFFER_OUT_OF_STOCK")
		}
		if !offer.IsActive {
			return fmt.Errorf("OFFER_NOT_ACTIVE")
		}
		if time.Now().After(offer.PickupEnd) {
			return fmt.Errorf("PICKUP_PERIOD_EXPIRED")
		}
		if err := txRepo.UpdateOfferQuantity(ctx, offer.ID, -1); err != nil {
			return err
		}
		if offer.QuantityAvailable-1 == 0 {
			txRepo.db.Model(&domain.Offer{}).Where("id = ?", offer.ID).Update("is_active", false)
		}
		amount := offer.Price
		now := time.Now()
		expiresAt := now.Add(15 * time.Minute)
		order := &domain.Order{
			ID:        uuid.New(),
			UserID:    userID,
			OfferID:   offer.ID,
			Status:    domain.OrderCreated,
			Amount:    amount,
			CreatedAt: now,
			ExpiresAt: &expiresAt,
		}
		if err := txRepo.CreateOrder(ctx, order); err != nil {
			return err
		}
		history := domain.OrderStatusHistory{
			ID:        uuid.New(),
			OrderID:   order.ID,
			Status:    domain.OrderCreated,
			ChangedAt: now,
		}
		if err := txRepo.SaveHistory(ctx, history); err != nil {
			return err
		}
		finalOrder = order
		return nil
	})
	return finalOrder, nil
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

	err = s.repo.Transaction(func(txRepo *OrderRepository) error {
		if err := txRepo.Update(ctx, order); err != nil {
			return err
		}
		history := domain.OrderStatusHistory{
			ID:        uuid.New(),
			OrderID:   order.ID,
			Status:    domain.OrderPaid,
			ChangedAt: now,
		}
		return txRepo.SaveHistory(ctx, history)
	})

	if err != nil {
		return nil, err
	}
	msg := fmt.Sprintf("Ваш заказ №%s успешно оплачен!", *order.OrderNumber)
	s.notifier.Send(ctx, order.UserID, msg)
	return order, nil
}

func (s *OrderService) CancelOrder(ctx context.Context, orderID, actorID uuid.UUID, role string, reason string) (*domain.Order, float64, error) {
	order, err := s.repo.GetByIDWithDetails(ctx, orderID)
	if err != nil {
		return nil, 0, fmt.Errorf("not found")
	}
	if role == "USER" {
		if order.UserID != actorID {
			return nil, 0, fmt.Errorf("unauthorized")
		}
	} else if role == "PARTNER" {
		if order.Offer.RestaurantID != actorID {
			return nil, 0, fmt.Errorf("unauthorized: not your restaurant's order")
		}
	}
	if order.Status == domain.OrderCompleted || order.Status == domain.OrderCancelled {
		return nil, 0, fmt.Errorf("CANNOT_CANCEL")
	}
	refundAmount := 0.0

	if order.Status == domain.OrderPaid {
		if order.Offer.PickupStart.Sub(time.Now()) < 1*time.Hour {
			return nil, 0, fmt.Errorf("CANCELLATION_WINDOW_CLOSED")
		}
		refundAmount = order.Amount
	}

	now := time.Now()
	order.Status = domain.OrderCancelled
	order.CancelledAt = &now
	order.CancellationReason = &reason

	err = s.repo.Transaction(func(txRepo *OrderRepository) error {
		if err := txRepo.Update(ctx, order); err != nil {
			return err
		}
		if err := txRepo.UpdateOfferQuantity(ctx, order.OfferID, 1); err != nil {
			return err
		}
		history := domain.OrderStatusHistory{
			ID:        uuid.New(),
			OrderID:   order.ID,
			Status:    domain.OrderCancelled,
			ChangedAt: now,
		}
		return txRepo.SaveHistory(ctx, history)
	})

	if err != nil {
		return nil, 0, err
	}
	s.notifier.Send(ctx, order.UserID, "Ваш заказ был отменен. Средства будут возвращены.")
	return order, refundAmount, nil
}

func (s *OrderService) CompleteOrder(ctx context.Context, orderID uuid.UUID, restaurantID uuid.UUID) (*domain.Order, error) {
	order, err := s.repo.GetByIDWithDetails(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("not found")
	}

	if order.Offer.RestaurantID != restaurantID {
		return nil, fmt.Errorf("unauthorized")
	}
	if order.Status != domain.OrderPaid {
		return nil, fmt.Errorf("INVALID_ORDER_STATUS")
	}

	now := time.Now()
	order.Status = domain.OrderCompleted
	order.CompletedAt = &now

	grossRevenue := order.Amount
	serviceFee := grossRevenue * 0.15
	netPayout := grossRevenue - serviceFee
	order.ServiceFee = order.Amount * 0.15
	order.NetPayout = order.Amount - order.ServiceFee
	log.Printf("[OrderService] Order %s COMPLETED. Gross: %.2f, Fee: %.2f, Net: %.2f", order.ID, grossRevenue, serviceFee, netPayout)

	err = s.repo.Transaction(func(txRepo *OrderRepository) error {
		if err := txRepo.Update(ctx, order); err != nil {
			return err
		}
		history := domain.OrderStatusHistory{
			ID:        uuid.New(),
			OrderID:   order.ID,
			Status:    domain.OrderCompleted,
			ChangedAt: now,
		}
		return txRepo.SaveHistory(ctx, history)
	})

	if err != nil {
		return nil, err
	}
	s.notifier.Send(ctx, order.UserID, "Заказ выдан! Приятного аппетита.")
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
