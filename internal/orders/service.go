package orders

import (
	"context"
	"fmt"
	"kursach_backend/internal/domain"
	"kursach_backend/internal/notifications"
	"log"
	"math/rand"
	"time"

	"github.com/google/uuid"
)

type OrderService struct {
	repo          *OrderRepository
	notifications notifications.Service
}

func NewOrderService(repo *OrderRepository, notificationsService notifications.Service) *OrderService {
	return &OrderService{
		repo:          repo,
		notifications: notificationsService,
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
	return finalOrder, err
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
	s.sendNotification(ctx, order.UserID, orderPaidNotificationPayload(order))
	return order, nil
}

func (s *OrderService) CancelOrder(ctx context.Context, orderID, actorID uuid.UUID, role string, reason string) (*domain.Order, float64, error) {
	order, err := s.repo.GetByIDWithDetails(ctx, orderID)
	if err != nil {
		return nil, 0, fmt.Errorf("not found")
	}
	switch role {
	case "USER":
		if order.UserID != actorID {
			return nil, 0, fmt.Errorf("unauthorized")
		}
	case "PARTNER":
		if order.Offer.RestaurantID != actorID {
			return nil, 0, fmt.Errorf("unauthorized: not your restaurant's order")
		}
	}
	if order.Status == domain.OrderCompleted || order.Status == domain.OrderCancelled {
		return nil, 0, fmt.Errorf("CANNOT_CANCEL")
	}
	refundAmount := 0.0

	if order.Status == domain.OrderPaid {
		if time.Until(order.Offer.PickupStart) < 1*time.Hour {
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
	s.sendNotification(ctx, order.UserID, orderCancelledNotificationPayload(order, refundAmount))
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
	s.sendNotification(ctx, order.UserID, orderCompletedNotificationPayload(order))
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

func (s *OrderService) GetNotifications(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Notification, error) {
	return s.notifications.ListForUser(ctx, userID, limit, offset)
}

func (s *OrderService) StartNotificationWorker(ctx context.Context) {
	if s.notifications == nil {
		return
	}
	s.notifications.StartCleanupWorker(ctx)
}

func (s *OrderService) sendNotification(ctx context.Context, userID uuid.UUID, payload notifications.Payload) {
	if s.notifications == nil {
		return
	}
	if err := s.notifications.SendToUser(ctx, userID, payload); err != nil {
		log.Printf("[OrderService] notification failed for user %s: %v", userID, err)
	}
}

func (s *OrderService) CreateReview(ctx context.Context, orderID, userID uuid.UUID, req CreateReviewRequest) (*domain.Review, error) {
	order, err := s.repo.GetByIDWithDetails(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, fmt.Errorf("unauthorized")
	}
	if order.Status != domain.OrderCompleted {
		return nil, fmt.Errorf("ORDER_NOT_COMPLETED")
	}
	review := &domain.Review{
		OrderID:      order.ID,
		UserID:       userID,
		RestaurantID: order.Offer.RestaurantID,
		Rating:       req.Rating,
		Comment:      req.Comment,
		CreatedAt:    time.Now(),
	}
	if err := s.repo.CreateReview(ctx, review); err != nil {
		return nil, err
	}

	return review, nil
}
