package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"kursach_backend/internal/domain"
	"kursach_backend/pkg/postgres"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	demoPassword = "Password123!"
	demoDomain   = "@stillgood.test"
)

type demoData struct {
	users       []domain.User
	categories  []domain.Category
	restaurants []domain.Restaurant
	offers      []domain.Offer
	orders      []domain.Order
	histories   []domain.OrderStatusHistory
	reviews     []domain.Review
	notices     []domain.Notification
	analytics   []domain.DailyAnalytics
}

func main() {
	if os.Getenv("ALLOW_DEMO_SEED") != "true" {
		log.Fatal("refusing to seed demo data: set ALLOW_DEMO_SEED=true")
	}

	db, err := openDB()
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("failed to get sql.db:", err)
	}
	defer sqlDB.Close()
	log.Println("Applying migrations before seeding...")
	goose.SetDialect("postgres")
	dir, err := migrationsDir()
	if err != nil {
		log.Fatal(err)
	}
	if err := goose.Up(sqlDB, dir); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	data, err := buildDemoData(time.Now().UTC())
	if err != nil {
		log.Fatalf("build demo data: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := seed(ctx, db, data); err != nil {
		log.Fatalf("seed demo data: %v", err)
	}

	log.Println("Demo data seeded successfully")
	log.Printf("Demo password for all accounts: %s", demoPassword)
	log.Println("Demo accounts:")
	for _, user := range data.users {
		log.Printf("  %-28s role=%s partnerStatus=%s", user.Email, user.Role, user.PartnerStatus)
	}
}

func openDB() (*gorm.DB, error) {
	required := []string{"DB_HOST", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_PORT"}
	for _, key := range required {
		if os.Getenv(key) == "" {
			return nil, fmt.Errorf("%s is required", key)
		}
	}

	dsn := "host=" + os.Getenv("DB_HOST") +
		" user=" + os.Getenv("DB_USER") +
		" password=" + os.Getenv("DB_PASSWORD") +
		" dbname=" + os.Getenv("DB_NAME") +
		" port=" + os.Getenv("DB_PORT") +
		" sslmode=disable"

	return postgres.NewDB(dsn)
}

func migrationsDir() (string, error) {
	if dir := os.Getenv("MIGRATIONS_DIR"); dir != "" {
		if _, err := os.Stat(dir); err != nil {
			return "", fmt.Errorf("MIGRATIONS_DIR %q is not available: %w", dir, err)
		}
		return dir, nil
	}

	for _, dir := range []string{"migrations", "../../migrations"} {
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}

	return "", fmt.Errorf("migrations directory not found")
}

func seed(ctx context.Context, db *gorm.DB, data demoData) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := cleanupDemoData(tx, data); err != nil {
			return err
		}
		if err := tx.Create(&data.users).Error; err != nil {
			return fmt.Errorf("create users: %w", err)
		}
		if err := tx.Create(&data.categories).Error; err != nil {
			return fmt.Errorf("create categories: %w", err)
		}
		if err := tx.Create(&data.restaurants).Error; err != nil {
			return fmt.Errorf("create restaurants: %w", err)
		}
		if err := tx.Create(&data.offers).Error; err != nil {
			return fmt.Errorf("create offers: %w", err)
		}
		if err := tx.Create(&data.orders).Error; err != nil {
			return fmt.Errorf("create orders: %w", err)
		}
		if err := tx.Create(&data.histories).Error; err != nil {
			return fmt.Errorf("create order histories: %w", err)
		}
		if err := tx.Create(&data.reviews).Error; err != nil {
			return fmt.Errorf("create reviews: %w", err)
		}
		if err := refreshRestaurantReviewStats(tx, restaurantIDs(data.restaurants)); err != nil {
			return err
		}
		if err := tx.Create(&data.notices).Error; err != nil {
			return fmt.Errorf("create notifications: %w", err)
		}
		if err := tx.Create(&data.analytics).Error; err != nil {
			return fmt.Errorf("create analytics: %w", err)
		}
		return nil
	})
}

func cleanupDemoData(tx *gorm.DB, data demoData) error {
	userIDs := userIDs(data.users)
	categoryIDs := categoryIDs(data.categories)
	restaurantIDs := restaurantIDs(data.restaurants)
	offerIDs := offerIDs(data.offers)
	orderIDs := orderIDs(data.orders)

	var previousDemoUsers []uuid.UUID
	if err := tx.Model(&domain.User{}).
		Where("email LIKE ?", "%"+demoDomain).
		Pluck("id", &previousDemoUsers).Error; err != nil {
		return fmt.Errorf("find previous demo users: %w", err)
	}
	userIDs = append(userIDs, previousDemoUsers...)

	var previousDemoRestaurants []uuid.UUID
	if err := tx.Model(&domain.Restaurant{}).
		Where("id IN ? OR partner_id IN ?", restaurantIDs, userIDs).
		Pluck("id", &previousDemoRestaurants).Error; err != nil {
		return fmt.Errorf("find previous demo restaurants: %w", err)
	}
	restaurantIDs = append(restaurantIDs, previousDemoRestaurants...)

	var previousDemoOffers []uuid.UUID
	if err := tx.Model(&domain.Offer{}).
		Where("id IN ? OR restaurant_id IN ? OR category_id IN ?", offerIDs, restaurantIDs, categoryIDs).
		Pluck("id", &previousDemoOffers).Error; err != nil {
		return fmt.Errorf("find previous demo offers: %w", err)
	}
	offerIDs = append(offerIDs, previousDemoOffers...)

	var previousDemoOrders []uuid.UUID
	if err := tx.Model(&domain.Order{}).
		Where("id IN ? OR user_id IN ? OR offer_id IN ?", orderIDs, userIDs, offerIDs).
		Pluck("id", &previousDemoOrders).Error; err != nil {
		return fmt.Errorf("find previous demo orders: %w", err)
	}
	orderIDs = append(orderIDs, previousDemoOrders...)

	deleteSteps := []struct {
		name string
		err  func() error
	}{
		{"reviews", func() error {
			return tx.Where("order_id IN ? OR restaurant_id IN ? OR user_id IN ?", orderIDs, restaurantIDs, userIDs).Delete(&domain.Review{}).Error
		}},
		{"notifications", func() error {
			return tx.Where("user_id IN ?", userIDs).Delete(&domain.Notification{}).Error
		}},
		{"order histories", func() error {
			return tx.Where("order_id IN ?", orderIDs).Delete(&domain.OrderStatusHistory{}).Error
		}},
		{"orders", func() error {
			return tx.Where("id IN ? OR user_id IN ? OR offer_id IN ?", orderIDs, userIDs, offerIDs).Delete(&domain.Order{}).Error
		}},
		{"analytics", func() error {
			return tx.Where("restaurant_id IN ?", restaurantIDs).Delete(&domain.DailyAnalytics{}).Error
		}},
		{"offers", func() error {
			return tx.Where("id IN ? OR restaurant_id IN ? OR category_id IN ?", offerIDs, restaurantIDs, categoryIDs).Delete(&domain.Offer{}).Error
		}},
		{"restaurants", func() error {
			return tx.Where("id IN ? OR partner_id IN ?", restaurantIDs, userIDs).Delete(&domain.Restaurant{}).Error
		}},
		{"users", func() error {
			return tx.Where("id IN ? OR email LIKE ?", userIDs, "%"+demoDomain).Delete(&domain.User{}).Error
		}},
		{"categories", func() error {
			return tx.Where("id IN ?", categoryIDs).Delete(&domain.Category{}).Error
		}},
	}

	for _, step := range deleteSteps {
		if err := step.err(); err != nil {
			return fmt.Errorf("delete demo %s: %w", step.name, err)
		}
	}
	return nil
}

func refreshRestaurantReviewStats(tx *gorm.DB, ids []uuid.UUID) error {
	for _, id := range ids {
		var stats struct {
			Rating float64
			Count  int
		}
		if err := tx.Model(&domain.Review{}).
			Select("COALESCE(AVG(rating), 0) AS rating, COUNT(*) AS count").
			Where("restaurant_id = ?", id).
			Scan(&stats).Error; err != nil {
			return fmt.Errorf("calculate review stats: %w", err)
		}
		if err := tx.Model(&domain.Restaurant{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"rating":       stats.Rating,
				"review_count": stats.Count,
			}).Error; err != nil {
			return fmt.Errorf("update review stats: %w", err)
		}
	}
	return nil
}

func buildDemoData(now time.Time) (demoData, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(demoPassword), bcrypt.DefaultCost)
	if err != nil {
		return demoData{}, err
	}

	passwordHash := string(hash)
	today := dayStart(now)
	past := now.Add(-48 * time.Hour)
	futureStart := now.Add(2 * time.Hour)
	futureEnd := now.Add(6 * time.Hour)

	adminID := mustUUID("11111111-1111-1111-1111-111111111001")
	userAnnaID := mustUUID("11111111-1111-1111-1111-111111111011")
	userMaxID := mustUUID("11111111-1111-1111-1111-111111111012")
	userBlockedID := mustUUID("11111111-1111-1111-1111-111111111013")
	partnerBakeryID := mustUUID("11111111-1111-1111-1111-111111111021")
	partnerSushiID := mustUUID("11111111-1111-1111-1111-111111111022")
	partnerGroceryID := mustUUID("11111111-1111-1111-1111-111111111023")
	partnerPendingID := mustUUID("11111111-1111-1111-1111-111111111024")

	catBakeryID := mustUUID("22222222-2222-2222-2222-222222222001")
	catMealsID := mustUUID("22222222-2222-2222-2222-222222222002")
	catSushiID := mustUUID("22222222-2222-2222-2222-222222222003")
	catDessertsID := mustUUID("22222222-2222-2222-2222-222222222004")
	catGroceriesID := mustUUID("22222222-2222-2222-2222-222222222005")
	catDrinksID := mustUUID("22222222-2222-2222-2222-222222222006")

	restBakeryID := mustUUID("33333333-3333-3333-3333-333333333001")
	restSushiID := mustUUID("33333333-3333-3333-3333-333333333002")
	restGroceryID := mustUUID("33333333-3333-3333-3333-333333333003")
	restInactiveID := mustUUID("33333333-3333-3333-3333-333333333004")

	offerCroissantID := mustUUID("44444444-4444-4444-4444-444444444001")
	offerDinnerID := mustUUID("44444444-4444-4444-4444-444444444002")
	offerSushiID := mustUUID("44444444-4444-4444-4444-444444444003")
	offerDessertID := mustUUID("44444444-4444-4444-4444-444444444004")
	offerGroceriesID := mustUUID("44444444-4444-4444-4444-444444444005")
	offerDrinksID := mustUUID("44444444-4444-4444-4444-444444444006")
	offerSoldOutID := mustUUID("44444444-4444-4444-4444-444444444007")
	offerExpiredID := mustUUID("44444444-4444-4444-4444-444444444008")

	orderCreatedID := mustUUID("55555555-5555-5555-5555-555555555001")
	orderPaidID := mustUUID("55555555-5555-5555-5555-555555555002")
	orderCompletedID := mustUUID("55555555-5555-5555-5555-555555555003")
	orderCancelledID := mustUUID("55555555-5555-5555-5555-555555555004")
	orderExpiredID := mustUUID("55555555-5555-5555-5555-555555555005")
	orderCompletedSushiID := mustUUID("55555555-5555-5555-5555-555555555006")
	orderCompletedGroceryID := mustUUID("55555555-5555-5555-5555-555555555007")

	users := []domain.User{
		user(adminID, "admin@stillgood.test", "Админ StillGood", "ADMIN", "", passwordHash, nil, true, false, now),
		user(userAnnaID, "anna.user@stillgood.test", "Анна Покупатель", "USER", "", passwordHash, stringPtr("+79990000001"), true, false, now),
		user(userMaxID, "max.user@stillgood.test", "Максим Покупатель", "USER", "", passwordHash, stringPtr("+79990000002"), true, false, now),
		user(userBlockedID, "blocked.user@stillgood.test", "Заблокированный пользователь", "USER", "", passwordHash, stringPtr("+79990000003"), true, true, now),
		user(partnerBakeryID, "partner.bakery@stillgood.test", "Партнер Пекарня", "PARTNER", "APPROVED", passwordHash, stringPtr("+79990000101"), true, false, now),
		user(partnerSushiID, "partner.sushi@stillgood.test", "Партнер Суши", "PARTNER", "APPROVED", passwordHash, stringPtr("+79990000102"), true, false, now),
		user(partnerGroceryID, "partner.grocery@stillgood.test", "Партнер Маркет", "PARTNER", "APPROVED", passwordHash, stringPtr("+79990000103"), true, false, now),
		user(partnerPendingID, "partner.pending@stillgood.test", "Партнер на модерации", "PARTNER", "PENDING", passwordHash, stringPtr("+79990000104"), false, false, now),
	}
	users[4].RestaurantID = &restBakeryID
	users[5].RestaurantID = &restSushiID
	users[6].RestaurantID = &restGroceryID

	categories := []domain.Category{
		{ID: catBakeryID, Name: "Выпечка", IconURL: stringPtr("https://picsum.photos/seed/stillgood-bakery/128")},
		{ID: catMealsID, Name: "Готовые блюда", IconURL: stringPtr("https://picsum.photos/seed/stillgood-meals/128")},
		{ID: catSushiID, Name: "Суши и роллы", IconURL: stringPtr("https://picsum.photos/seed/stillgood-sushi/128")},
		{ID: catDessertsID, Name: "Десерты", IconURL: stringPtr("https://picsum.photos/seed/stillgood-desserts/128")},
		{ID: catGroceriesID, Name: "Продукты", IconURL: stringPtr("https://picsum.photos/seed/stillgood-groceries/128")},
		{ID: catDrinksID, Name: "Напитки", IconURL: stringPtr("https://picsum.photos/seed/stillgood-drinks/128")},
	}

	restaurants := []domain.Restaurant{
		restaurant(restBakeryID, partnerBakeryID, "Хлеб и Кофе", "ООО Хлеб и Кофе", "7701000001", "Москва, ул. Тверская, 7", "Пекарня с вечерними наборами свежей выпечки.", "+74950000001", 55.7621, 37.6092, 12, true, now),
		restaurant(restSushiID, partnerSushiID, "Sakura Box", "ООО Сакура Бокс", "7701000002", "Москва, ул. Покровка, 18", "Роллы, боулы и готовые наборы после обеда.", "+74950000002", 55.7602, 37.6478, 15, true, now),
		restaurant(restGroceryID, partnerGroceryID, "Green Market", "ООО Грин Маркет", "7701000003", "Москва, Ленинградский проспект, 35", "Продукты с коротким сроком годности по сниженной цене.", "+74950000003", 55.7867, 37.5626, 10, true, now),
		restaurant(restInactiveID, partnerPendingID, "Draft Kitchen", "ООО Драфт Китчен", "7701000004", "Москва, ул. Лесная, 20", "Неактивный ресторан для проверки админки.", "+74950000004", 55.7808, 37.5908, 0, false, now),
	}

	offers := []domain.Offer{
		offer(offerCroissantID, restBakeryID, catBakeryID, "Вечерний набор круассанов", "4 круассана и 2 булочки дня.", 290, 620, 6, 8, futureStart, futureEnd, true, "https://picsum.photos/seed/stillgood-croissant/800/600", now),
		offer(offerDinnerID, restBakeryID, catMealsID, "Ланч-бокс с кишем", "Киш, салат и маленький десерт.", 360, 760, 3, 5, now.Add(90*time.Minute), now.Add(5*time.Hour), true, "https://picsum.photos/seed/stillgood-lunch/800/600", now),
		offer(offerSushiID, restSushiID, catSushiID, "Сет роллов Surprise", "Ассорти роллов дня, 24 штуки.", 690, 1390, 4, 6, now.Add(3*time.Hour), now.Add(7*time.Hour), true, "https://picsum.photos/seed/stillgood-sushi-set/800/600", now),
		offer(offerDessertID, restSushiID, catDessertsID, "Матча десерты", "Два десерта матча и чизкейк.", 330, 740, 2, 4, now.Add(2*time.Hour), now.Add(8*time.Hour), true, "https://picsum.photos/seed/stillgood-dessert/800/600", now),
		offer(offerGroceriesID, restGroceryID, catGroceriesID, "Овощной набор", "Овощи, зелень и фрукты для ужина.", 410, 980, 5, 7, now.Add(1*time.Hour), now.Add(4*time.Hour), true, "https://picsum.photos/seed/stillgood-grocery-box/800/600", now),
		offer(offerDrinksID, restGroceryID, catDrinksID, "Набор смузи", "3 бутылки смузи со сроком до завтра.", 250, 540, 5, 5, now.Add(1*time.Hour), now.Add(5*time.Hour), true, "https://picsum.photos/seed/stillgood-smoothie/800/600", now),
		offer(offerSoldOutID, restBakeryID, catDessertsID, "Распроданный чизкейк", "Кейс для отображения sold out.", 199, 450, 0, 3, futureStart, futureEnd, true, "https://picsum.photos/seed/stillgood-soldout/800/600", now.Add(-24*time.Hour)),
		offer(offerExpiredID, restSushiID, catMealsID, "Вчерашний ужин", "Кейс для истекшего pickup window.", 300, 700, 2, 2, past, past.Add(4*time.Hour), true, "https://picsum.photos/seed/stillgood-expired/800/600", now.Add(-72*time.Hour)),
	}

	orders := []domain.Order{
		order(orderCreatedID, userAnnaID, offerGroceriesID, "SG-1001", domain.OrderCreated, 410, now.Add(-20*time.Minute), nil, nil, nil, timePtr(now.Add(40*time.Minute)), nil),
		order(orderPaidID, userMaxID, offerSushiID, "SG-1002", domain.OrderPaid, 690, now.Add(-2*time.Hour), timePtr(now.Add(-90*time.Minute)), nil, nil, timePtr(now.Add(30*time.Minute)), nil),
		order(orderCompletedID, userAnnaID, offerCroissantID, "SG-1003", domain.OrderCompleted, 290, now.Add(-48*time.Hour), timePtr(now.Add(-47*time.Hour)), timePtr(now.Add(-44*time.Hour)), nil, nil, nil),
		order(orderCancelledID, userMaxID, offerDessertID, "SG-1004", domain.OrderCancelled, 330, now.Add(-24*time.Hour), timePtr(now.Add(-23*time.Hour)), nil, timePtr(now.Add(-22*time.Hour)), nil, stringPtr("Пользователь отменил заказ")),
		order(orderExpiredID, userAnnaID, offerExpiredID, "SG-1005", domain.OrderCancelled, 300, now.Add(-72*time.Hour), nil, nil, timePtr(now.Add(-68*time.Hour)), nil, stringPtr("expired")),
		order(orderCompletedSushiID, userAnnaID, offerSushiID, "SG-1006", domain.OrderCompleted, 690, now.Add(-96*time.Hour), timePtr(now.Add(-95*time.Hour)), timePtr(now.Add(-92*time.Hour)), nil, nil, nil),
		order(orderCompletedGroceryID, userMaxID, offerGroceriesID, "SG-1007", domain.OrderCompleted, 410, now.Add(-120*time.Hour), timePtr(now.Add(-119*time.Hour)), timePtr(now.Add(-116*time.Hour)), nil, nil, nil),
	}

	histories := []domain.OrderStatusHistory{}
	histories = append(histories, history(orderCreatedID, domain.OrderCreated, now.Add(-20*time.Minute)))
	histories = append(histories, history(orderPaidID, domain.OrderCreated, now.Add(-2*time.Hour)), history(orderPaidID, domain.OrderPaid, now.Add(-90*time.Minute)))
	histories = append(histories, history(orderCompletedID, domain.OrderCreated, now.Add(-48*time.Hour)), history(orderCompletedID, domain.OrderPaid, now.Add(-47*time.Hour)), history(orderCompletedID, domain.OrderCompleted, now.Add(-44*time.Hour)))
	histories = append(histories, history(orderCancelledID, domain.OrderCreated, now.Add(-24*time.Hour)), history(orderCancelledID, domain.OrderPaid, now.Add(-23*time.Hour)), history(orderCancelledID, domain.OrderCancelled, now.Add(-22*time.Hour)))
	histories = append(histories, history(orderExpiredID, domain.OrderCreated, now.Add(-72*time.Hour)), history(orderExpiredID, domain.OrderCancelled, now.Add(-68*time.Hour)))
	histories = append(histories, history(orderCompletedSushiID, domain.OrderCreated, now.Add(-96*time.Hour)), history(orderCompletedSushiID, domain.OrderPaid, now.Add(-95*time.Hour)), history(orderCompletedSushiID, domain.OrderCompleted, now.Add(-92*time.Hour)))
	histories = append(histories, history(orderCompletedGroceryID, domain.OrderCreated, now.Add(-120*time.Hour)), history(orderCompletedGroceryID, domain.OrderPaid, now.Add(-119*time.Hour)), history(orderCompletedGroceryID, domain.OrderCompleted, now.Add(-116*time.Hour)))

	reviews := []domain.Review{
		{ID: mustUUID("66666666-6666-6666-6666-666666666001"), OrderID: orderCompletedID, RestaurantID: restBakeryID, UserID: userAnnaID, Rating: 5, Comment: "Свежая выпечка, удобно забрать после работы.", CreatedAt: now.Add(-43 * time.Hour)},
		{ID: mustUUID("66666666-6666-6666-6666-666666666002"), OrderID: orderCompletedSushiID, RestaurantID: restSushiID, UserID: userAnnaID, Rating: 4, Comment: "Хороший сет, часть роллов была с острой начинкой.", CreatedAt: now.Add(-91 * time.Hour)},
		{ID: mustUUID("66666666-6666-6666-6666-666666666003"), OrderID: orderCompletedGroceryID, RestaurantID: restGroceryID, UserID: userMaxID, Rating: 5, Comment: "Овощи свежие, набор оказался больше ожидаемого.", CreatedAt: now.Add(-115 * time.Hour)},
	}

	notices := []domain.Notification{
		notice(userAnnaID, "Заказ создан", "Овощной набор ожидает оплаты.", "/orders/"+orderCreatedID.String(), "order_created", now.Add(-20*time.Minute)),
		notice(userMaxID, "Заказ оплачен", "Сет роллов Surprise оплачен и ожидает выдачи.", "/orders/"+orderPaidID.String(), "order_paid", now.Add(-90*time.Minute)),
		notice(userAnnaID, "Спасибо за отзыв", "Ваш отзыв помогает другим пользователям.", "/restaurants/"+restBakeryID.String()+"/reviews", "review_created", now.Add(-42*time.Hour)),
		notice(userMaxID, "Заказ отменен", "Матча десерты были отменены.", "/orders/"+orderCancelledID.String(), "order_cancelled", now.Add(-22*time.Hour)),
		notice(userAnnaID, "Заказ завершен", "Sakura Box отметил заказ как выданный.", "/orders/"+orderCompletedSushiID.String(), "order_completed", now.Add(-92*time.Hour)),
		notice(userMaxID, "Заказ завершен", "Green Market отметил заказ как выданный.", "/orders/"+orderCompletedGroceryID.String(), "order_completed", now.Add(-116*time.Hour)),
	}

	analytics := analyticsRows(today, restBakeryID, restSushiID, restGroceryID)

	return demoData{
		users:       users,
		categories:  categories,
		restaurants: restaurants,
		offers:      offers,
		orders:      orders,
		histories:   histories,
		reviews:     reviews,
		notices:     notices,
		analytics:   analytics,
	}, nil
}

func user(id uuid.UUID, email, name, role, partnerStatus, passwordHash string, phone *string, verified, blocked bool, now time.Time) domain.User {
	deviceToken := "demo-device-token-" + id.String()
	return domain.User{
		ID:            id,
		Email:         email,
		Phone:         phone,
		Role:          role,
		PartnerStatus: partnerStatus,
		IsVerified:    verified,
		IsBlocked:     blocked,
		DeviceToken:   &deviceToken,
		AuthProvider:  "email",
		PasswordHash:  passwordHash,
		Name:          name,
		CreatedAt:     now.Add(-7 * 24 * time.Hour),
		UpdatedAt:     now,
	}
}

func restaurant(id, partnerID uuid.UUID, name, company, inn, address, description, phone string, lat, lng, commission float64, active bool, now time.Time) domain.Restaurant {
	return domain.Restaurant{
		ID:           id,
		PartnerID:    partnerID,
		Name:         name,
		CompanyName:  company,
		Inn:          inn,
		Address:      address,
		Description:  &description,
		Commission:   commission,
		LogoURL:      stringPtr("https://picsum.photos/seed/" + id.String() + "-logo/256/256"),
		CoverURL:     stringPtr("https://picsum.photos/seed/" + id.String() + "-cover/1200/600"),
		Phone:        &phone,
		Latitude:     lat,
		Longitude:    lng,
		IsActive:     active,
		WorkingHours: "Пн-Вс 09:00-22:00",
		CreatedAt:    now.Add(-7 * 24 * time.Hour),
		UpdatedAt:    now,
	}
}

func offer(id, restaurantID, categoryID uuid.UUID, title, description string, price, originalPrice float64, qtyAvailable, qtyTotal int, pickupStart, pickupEnd time.Time, active bool, imageURL string, createdAt time.Time) domain.Offer {
	return domain.Offer{
		ID:                id,
		RestaurantID:      restaurantID,
		CategoryID:        categoryID,
		Title:             title,
		Description:       description,
		ImageURL:          &imageURL,
		Price:             price,
		OriginalPrice:     originalPrice,
		QuantityAvailable: qtyAvailable,
		QuantityTotal:     qtyTotal,
		PickupStart:       pickupStart,
		PickupEnd:         pickupEnd,
		CreatedAt:         createdAt,
		IsActive:          active,
	}
}

func order(id, userID, offerID uuid.UUID, number string, status domain.OrderStatus, amount float64, createdAt time.Time, paidAt, completedAt, cancelledAt, expiresAt *time.Time, cancelReason *string) domain.Order {
	serviceFee := amount * 0.15
	return domain.Order{
		ID:                 id,
		UserID:             userID,
		OfferID:            offerID,
		OrderNumber:        &number,
		Status:             status,
		Amount:             amount,
		ServiceFee:         serviceFee,
		NetPayout:          amount - serviceFee,
		CreatedAt:          createdAt,
		PaidAt:             paidAt,
		CompletedAt:        completedAt,
		CancelledAt:        cancelledAt,
		ExpiresAt:          expiresAt,
		CancellationReason: cancelReason,
	}
}

func history(orderID uuid.UUID, status domain.OrderStatus, changedAt time.Time) domain.OrderStatusHistory {
	return domain.OrderStatusHistory{
		ID:        uuid.New(),
		OrderID:   orderID,
		Status:    status,
		ChangedAt: changedAt,
	}
}

func notice(userID uuid.UUID, title, body, deepLink, notificationType string, createdAt time.Time) domain.Notification {
	return domain.Notification{
		ID:        uuid.New(),
		UserID:    userID,
		Title:     title,
		Body:      body,
		DeepLink:  deepLink,
		Type:      notificationType,
		CreatedAt: createdAt,
	}
}

func analyticsRows(today time.Time, bakeryID, sushiID, groceryID uuid.UUID) []domain.DailyAnalytics {
	type row struct {
		restaurantID uuid.UUID
		category     string
		bookings     int
		completed    int
		cancelled    int
		expired      int
		revenue      float64
	}

	rows := []domain.DailyAnalytics{}
	baseRows := []row{
		{bakeryID, "Выпечка", 14, 11, 2, 1, 6380},
		{bakeryID, "Готовые блюда", 8, 6, 1, 1, 3280},
		{sushiID, "Суши и роллы", 11, 9, 1, 0, 8210},
		{sushiID, "Десерты", 6, 5, 1, 0, 2260},
		{groceryID, "Продукты", 10, 8, 1, 1, 4920},
		{groceryID, "Напитки", 7, 6, 0, 0, 1750},
	}

	for day := 0; day < 5; day++ {
		date := today.AddDate(0, 0, -day)
		decay := float64(5-day) / 5
		for _, item := range baseRows {
			revenue := item.revenue * decay
			serviceFee := revenue * 0.15
			rows = append(rows, domain.DailyAnalytics{
				ID:              uuid.New(),
				RestaurantID:    item.restaurantID,
				Date:            date,
				CategoryName:    item.category,
				TotalBookings:   maxInt(1, int(float64(item.bookings)*decay)),
				CompletedOrders: maxInt(1, int(float64(item.completed)*decay)),
				CancelledOrders: int(float64(item.cancelled) * decay),
				ExpiredOrders:   int(float64(item.expired) * decay),
				GrossRevenue:    revenue,
				ServiceFee:      serviceFee,
				NetPayout:       revenue - serviceFee,
				CreatedAt:       time.Now().UTC(),
			})
		}
	}
	return rows
}

func userIDs(users []domain.User) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(users))
	for _, item := range users {
		ids = append(ids, item.ID)
	}
	return ids
}

func categoryIDs(categories []domain.Category) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(categories))
	for _, item := range categories {
		ids = append(ids, item.ID)
	}
	return ids
}

func restaurantIDs(restaurants []domain.Restaurant) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(restaurants))
	for _, item := range restaurants {
		ids = append(ids, item.ID)
	}
	return ids
}

func offerIDs(offers []domain.Offer) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(offers))
	for _, item := range offers {
		ids = append(ids, item.ID)
	}
	return ids
}

func orderIDs(orders []domain.Order) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(orders))
	for _, item := range orders {
		ids = append(ids, item.ID)
	}
	return ids
}

func mustUUID(value string) uuid.UUID {
	return uuid.MustParse(value)
}

func stringPtr(value string) *string {
	return &value
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func dayStart(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
