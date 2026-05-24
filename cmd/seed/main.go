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
	demoPassword      = "Password123!"
	demoDomain        = "@stillgood.test"
	analyticsSeedDays = 14
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
	now = now.UTC()
	hash, err := bcrypt.GenerateFromPassword([]byte(demoPassword), bcrypt.DefaultCost)
	if err != nil {
		return demoData{}, err
	}

	passwordHash := string(hash)
	today := dayStart(now)
	futureStart := now.Add(2 * time.Hour)
	futureEnd := now.Add(6 * time.Hour)

	adminID := demoUUID(1, 1)
	userAnnaID := demoUUID(1, 11)
	userMaxID := demoUUID(1, 12)
	userDaryaID := demoUUID(1, 13)
	userIlyaID := demoUUID(1, 14)
	userKateID := demoUUID(1, 15)
	userBlockedID := demoUUID(1, 16)
	partnerBakeryID := demoUUID(1, 21)
	partnerSushiID := demoUUID(1, 22)
	partnerGroceryID := demoUUID(1, 23)
	partnerBowlID := demoUUID(1, 24)
	partnerPendingBistroID := demoUUID(1, 31)
	partnerPendingDessertID := demoUUID(1, 32)
	partnerRejectedFastID := demoUUID(1, 41)
	partnerRejectedCateringID := demoUUID(1, 42)

	catBakeryID := mustUUID("22222222-2222-2222-2222-222222222001")
	catMealsID := mustUUID("22222222-2222-2222-2222-222222222002")
	catSushiID := mustUUID("22222222-2222-2222-2222-222222222003")
	catDessertsID := mustUUID("22222222-2222-2222-2222-222222222004")
	catGroceriesID := mustUUID("22222222-2222-2222-2222-222222222005")
	catDrinksID := mustUUID("22222222-2222-2222-2222-222222222006")
	catBowlsID := mustUUID("22222222-2222-2222-2222-222222222007")

	restBakeryID := demoUUID(3, 1)
	restSushiID := demoUUID(3, 2)
	restGroceryID := demoUUID(3, 3)
	restBowlID := demoUUID(3, 4)

	users := []domain.User{
		user(adminID, "admin@stillgood.test", "Администратор StillGood", "ADMIN", "", passwordHash, nil, true, false, now),
		user(userAnnaID, "anna.belova@stillgood.test", "Анна Белова", "USER", "", passwordHash, stringPtr("+79990000001"), true, false, now),
		user(userMaxID, "maxim.sokolov@stillgood.test", "Максим Соколов", "USER", "", passwordHash, stringPtr("+79990000002"), true, false, now),
		user(userDaryaID, "darya.kim@stillgood.test", "Дарья Ким", "USER", "", passwordHash, stringPtr("+79990000003"), true, false, now),
		user(userIlyaID, "ilya.orlov@stillgood.test", "Илья Орлов", "USER", "", passwordHash, stringPtr("+79990000004"), true, false, now),
		user(userKateID, "ekaterina.volkova@stillgood.test", "Екатерина Волкова", "USER", "", passwordHash, stringPtr("+79990000005"), true, false, now),
		user(userBlockedID, "blocked.user@stillgood.test", "Заблокированный пользователь", "USER", "", passwordHash, stringPtr("+79990000006"), true, true, now),
		user(partnerBakeryID, "partner.bakery@stillgood.test", "Партнер Хлеб и Кофе", "PARTNER", "APPROVED", passwordHash, stringPtr("+79990000101"), true, false, now),
		user(partnerSushiID, "partner.sushi@stillgood.test", "Партнер Sakura Box", "PARTNER", "APPROVED", passwordHash, stringPtr("+79990000102"), true, false, now),
		user(partnerGroceryID, "partner.market@stillgood.test", "Партнер Green Market", "PARTNER", "APPROVED", passwordHash, stringPtr("+79990000103"), true, false, now),
		user(partnerBowlID, "partner.bowl@stillgood.test", "Партнер Daily Bowl", "PARTNER", "APPROVED", passwordHash, stringPtr("+79990000104"), true, false, now),
		user(partnerPendingBistroID, "partner.pending.bistro@stillgood.test", "Партнер Bistro Point", "PARTNER", "PENDING", passwordHash, stringPtr("+79990000111"), false, false, now),
		user(partnerPendingDessertID, "partner.pending.dessert@stillgood.test", "Партнер Sweet Draft", "PARTNER", "PENDING", passwordHash, stringPtr("+79990000112"), false, false, now),
		user(partnerRejectedFastID, "partner.rejected.fast@stillgood.test", "Партнер Fast Corner", "PARTNER", "REJECTED", passwordHash, stringPtr("+79990000121"), true, false, now),
		user(partnerRejectedCateringID, "partner.rejected.catering@stillgood.test", "Партнер Catering Lab", "PARTNER", "REJECTED", passwordHash, stringPtr("+79990000122"), true, false, now),
	}
	for i := range users {
		switch users[i].ID {
		case partnerBakeryID:
			users[i].RestaurantID = &restBakeryID
		case partnerSushiID:
			users[i].RestaurantID = &restSushiID
		case partnerGroceryID:
			users[i].RestaurantID = &restGroceryID
		case partnerBowlID:
			users[i].RestaurantID = &restBowlID
		}
	}

	categories := []domain.Category{
		{ID: catBakeryID, Name: "Выпечка", IconURL: stringPtr("https://picsum.photos/seed/stillgood-bakery/128")},
		{ID: catMealsID, Name: "Готовые блюда", IconURL: stringPtr("https://picsum.photos/seed/stillgood-meals/128")},
		{ID: catSushiID, Name: "Суши и роллы", IconURL: stringPtr("https://picsum.photos/seed/stillgood-sushi/128")},
		{ID: catDessertsID, Name: "Десерты", IconURL: stringPtr("https://picsum.photos/seed/stillgood-desserts/128")},
		{ID: catGroceriesID, Name: "Продукты", IconURL: stringPtr("https://picsum.photos/seed/stillgood-groceries/128")},
		{ID: catDrinksID, Name: "Напитки", IconURL: stringPtr("https://picsum.photos/seed/stillgood-drinks/128")},
		{ID: catBowlsID, Name: "Боулы и салаты", IconURL: stringPtr("https://picsum.photos/seed/stillgood-bowls/128")},
	}

	restaurants := []domain.Restaurant{
		restaurant(restBakeryID, partnerBakeryID, "Хлеб и Кофе", "ООО Хлеб и Кофе", "7701000001", "Москва, ул. Тверская, 7", "Пекарня с вечерними наборами свежей выпечки.", "+74950000001", 55.7621, 37.6092, 12, true, now),
		restaurant(restSushiID, partnerSushiID, "Sakura Box", "ООО Сакура Бокс", "7701000002", "Москва, ул. Покровка, 18", "Роллы, боулы и готовые наборы после обеда.", "+74950000002", 55.7602, 37.6478, 15, true, now),
		restaurant(restGroceryID, partnerGroceryID, "Green Market", "ООО Грин Маркет", "7701000003", "Москва, Ленинградский проспект, 35", "Продукты с коротким сроком годности по сниженной цене.", "+74950000003", 55.7867, 37.5626, 10, true, now),
		restaurant(restBowlID, partnerBowlID, "Daily Bowl", "ООО Дейли Боул", "7701000004", "Москва, ул. Лесная, 20", "Боулы, супы и салаты с дневной витрины.", "+74950000004", 55.7808, 37.5908, 14, true, now),
	}

	offers := []domain.Offer{}
	addOffer := func(index int, restaurantID, categoryID uuid.UUID, title, description string, price, originalPrice float64, total int, pickupStart, pickupEnd time.Time, active bool, seed string) uuid.UUID {
		id := demoUUID(4, index)
		imageURL := "https://picsum.photos/seed/" + seed + "/800/600"
		offers = append(offers, offer(id, restaurantID, categoryID, title, description, price, originalPrice, total, total, pickupStart, pickupEnd, active, imageURL, pickupStart.Add(-8*time.Hour)))
		return id
	}
	historicOffer := func(index int, daysAgo int, restaurantID, categoryID uuid.UUID, title, description string, price, originalPrice float64, total int, seed string) uuid.UUID {
		start := dayAt(today, daysAgo, 18, 0)
		return addOffer(index, restaurantID, categoryID, title, description, price, originalPrice, total, start, start.Add(3*time.Hour), false, seed)
	}

	offerBakeryD13 := historicOffer(1, 13, restBakeryID, catBakeryID, "Вечерний набор круассанов", "4 круассана и 2 булочки дня.", 290, 620, 6, "stillgood-croissant-d13")
	offerBakeryD12 := historicOffer(2, 12, restBakeryID, catBakeryID, "Корзина выпечки", "Слойки, круассаны и хлеб дня.", 290, 620, 6, "stillgood-croissant-d12")
	offerBakeryD8 := historicOffer(3, 8, restBakeryID, catMealsID, "Ланч-бокс с кишем", "Киш, салат и маленький десерт.", 360, 760, 7, "stillgood-lunch-d8")
	offerBakeryD4 := historicOffer(4, 4, restBakeryID, catDessertsID, "Чизкейк дня", "Кусочки чизкейка и тарталетки.", 199, 450, 4, "stillgood-cheesecake-d4")
	offerSushiD11 := historicOffer(5, 11, restSushiID, catSushiID, "Сет роллов Surprise", "Ассорти роллов дня, 24 штуки.", 690, 1390, 5, "stillgood-sushi-d11")
	offerSushiD10 := historicOffer(6, 10, restSushiID, catDessertsID, "Матча десерты", "Два десерта матча и чизкейк.", 330, 740, 5, "stillgood-matcha-d10")
	offerSushiD6 := historicOffer(7, 6, restSushiID, catSushiID, "Сет роллов Classic", "Роллы с лососем, овощами и сливочным сыром.", 690, 1390, 5, "stillgood-sushi-d6")
	offerSushiD2 := historicOffer(8, 2, restSushiID, catDessertsID, "Матча десерты", "Десерты матча после дневной витрины.", 330, 740, 5, "stillgood-matcha-d2")
	offerMarketD9 := historicOffer(9, 9, restGroceryID, catGroceriesID, "Овощной набор", "Овощи, зелень и фрукты для ужина.", 410, 980, 6, "stillgood-veg-d9")
	offerMarketD7 := historicOffer(10, 7, restGroceryID, catDrinksID, "Набор смузи", "3 бутылки смузи со сроком до завтра.", 250, 540, 6, "stillgood-smoothie-d7")
	offerMarketD3 := historicOffer(11, 3, restGroceryID, catGroceriesID, "Овощной набор", "Овощи, зелень и фрукты для ужина.", 410, 980, 6, "stillgood-veg-d3")
	offerMarketD1 := historicOffer(12, 1, restGroceryID, catDrinksID, "Набор смузи", "3 бутылки смузи со сроком до завтра.", 250, 540, 6, "stillgood-smoothie-d1")
	offerBowlD5 := historicOffer(13, 5, restBowlID, catBowlsID, "Боул с курицей", "Боул, суп дня и салат.", 390, 850, 5, "stillgood-bowl-d5")
	offerBowlD1 := historicOffer(14, 1, restBowlID, catBowlsID, "Боул с киноа", "Боул с киноа и овощами.", 390, 850, 5, "stillgood-bowl-d1")
	offerBowlToday := addOffer(15, restBowlID, catBowlsID, "Дневной боул", "Боул с курицей после обеденной витрины.", 390, 850, 5, now.Add(-3*time.Hour), now.Add(-1*time.Hour), false, "stillgood-bowl-today")
	currentBakery := addOffer(101, restBakeryID, catBakeryID, "Свежая выпечка сегодня", "Круассаны, булочки и хлеб к вечернему самовывозу.", 310, 640, 8, futureStart, futureEnd, true, "stillgood-current-bakery")
	addOffer(102, restBakeryID, catMealsID, "Киш и салат", "Киш, салат и десерт из дневного меню.", 370, 780, 6, now.Add(90*time.Minute), now.Add(5*time.Hour), true, "stillgood-current-lunch")
	currentSushi := addOffer(103, restSushiID, catSushiID, "Сет роллов Evening", "Ассорти роллов дня, 24 штуки.", 690, 1390, 7, now.Add(3*time.Hour), now.Add(7*time.Hour), true, "stillgood-current-sushi")
	addOffer(104, restSushiID, catDessertsID, "Матча сет", "Десерты матча и чизкейк.", 340, 760, 4, now.Add(2*time.Hour), now.Add(8*time.Hour), true, "stillgood-current-dessert")
	currentGroceries := addOffer(105, restGroceryID, catGroceriesID, "Овощи и фрукты", "Набор овощей, зелени и фруктов.", 420, 990, 8, now.Add(1*time.Hour), now.Add(4*time.Hour), true, "stillgood-current-grocery")
	addOffer(106, restGroceryID, catDrinksID, "Смузи микс", "3 бутылки смузи со сроком до завтра.", 260, 560, 6, now.Add(1*time.Hour), now.Add(5*time.Hour), true, "stillgood-current-smoothie")
	addOffer(107, restBowlID, catBowlsID, "Боул и суп", "Боул, суп дня и салат.", 390, 850, 6, now.Add(90*time.Minute), now.Add(5*time.Hour), true, "stillgood-current-bowl")
	addOffer(108, restBowlID, catMealsID, "Суп и салат", "Суп дня, салат и хлеб.", 240, 520, 5, now.Add(2*time.Hour), now.Add(6*time.Hour), true, "stillgood-current-soup")

	restaurantByID := restaurantsByID(restaurants)
	offerByID := offersByID(offers)
	orders := []domain.Order{}
	histories := []domain.OrderStatusHistory{}
	addOrder := func(index int, userID, offerID uuid.UUID, status domain.OrderStatus, createdAt time.Time, paidAt, completedAt, cancelledAt, expiresAt *time.Time, cancelReason *string) uuid.UUID {
		id := demoUUID(5, index)
		var number *string
		if paidAt != nil {
			numberValue := fmt.Sprintf("SG-%04d", 2000+index)
			number = &numberValue
		}
		offerItem := offerByID[offerID]
		restaurantItem := restaurantByID[offerItem.RestaurantID]
		orders = append(orders, order(id, userID, offerID, number, status, offerItem.Price, restaurantItem.Commission, createdAt, paidAt, completedAt, cancelledAt, expiresAt, cancelReason))
		histories = append(histories, history(id, domain.OrderCreated, createdAt))
		if paidAt != nil {
			histories = append(histories, history(id, domain.OrderPaid, *paidAt))
		}
		if completedAt != nil {
			histories = append(histories, history(id, domain.OrderCompleted, *completedAt))
		}
		if cancelledAt != nil {
			histories = append(histories, history(id, domain.OrderCancelled, *cancelledAt))
		}
		return id
	}
	completedOrder := func(index int, userID, offerID uuid.UUID) uuid.UUID {
		offerItem := offerByID[offerID]
		createdAt := offerItem.PickupStart.Add(-6 * time.Hour)
		paidAt := createdAt.Add(25 * time.Minute)
		completedAt := offerItem.PickupStart.Add(90 * time.Minute)
		return addOrder(index, userID, offerID, domain.OrderCompleted, createdAt, &paidAt, &completedAt, nil, nil, nil)
	}
	cancelledPaidOrder := func(index int, userID, offerID uuid.UUID, reason string) uuid.UUID {
		offerItem := offerByID[offerID]
		createdAt := offerItem.PickupStart.Add(-7 * time.Hour)
		paidAt := createdAt.Add(20 * time.Minute)
		cancelledAt := createdAt.Add(2 * time.Hour)
		return addOrder(index, userID, offerID, domain.OrderCancelled, createdAt, &paidAt, nil, &cancelledAt, nil, stringPtr(reason))
	}
	expiredOrder := func(index int, userID, offerID uuid.UUID) uuid.UUID {
		offerItem := offerByID[offerID]
		createdAt := offerItem.PickupStart.Add(-8 * time.Hour)
		expiresAt := createdAt.Add(15 * time.Minute)
		cancelledAt := createdAt.Add(30 * time.Minute)
		return addOrder(index, userID, offerID, domain.OrderCancelled, createdAt, nil, nil, &cancelledAt, &expiresAt, stringPtr("expired"))
	}
	createdOrder := func(index int, userID, offerID uuid.UUID) uuid.UUID {
		createdAt := now.Add(-5 * time.Minute)
		expiresAt := createdAt.Add(15 * time.Minute)
		return addOrder(index, userID, offerID, domain.OrderCreated, createdAt, nil, nil, nil, &expiresAt, nil)
	}
	paidOrder := func(index int, userID, offerID uuid.UUID) uuid.UUID {
		createdAt := now.Add(-90 * time.Minute)
		paidAt := createdAt.Add(10 * time.Minute)
		expiresAt := createdAt.Add(15 * time.Minute)
		return addOrder(index, userID, offerID, domain.OrderPaid, createdAt, &paidAt, nil, nil, &expiresAt, nil)
	}

	orderBakery1 := completedOrder(1, userAnnaID, offerBakeryD13)
	orderBakery2 := completedOrder(2, userMaxID, offerBakeryD12)
	orderSushi1 := completedOrder(3, userDaryaID, offerSushiD11)
	orderSushi2 := completedOrder(4, userIlyaID, offerSushiD10)
	orderMarket1 := completedOrder(5, userAnnaID, offerMarketD9)
	orderBakery3 := completedOrder(6, userDaryaID, offerBakeryD8)
	orderBakery4 := completedOrder(7, userIlyaID, offerBakeryD8)
	cancelledPaidOrder(8, userMaxID, offerBakeryD8, "Пользователь отменил заказ заранее")
	orderMarket2 := completedOrder(9, userMaxID, offerMarketD7)
	orderSushi3 := completedOrder(10, userAnnaID, offerSushiD6)
	orderBowl1 := completedOrder(11, userIlyaID, offerBowlD5)
	orderBowlExpired := expiredOrder(12, userDaryaID, offerBowlD5)
	orderBakery5 := completedOrder(13, userKateID, offerBakeryD4)
	orderMarket3 := completedOrder(14, userAnnaID, offerMarketD3)
	orderMarketCancelled := cancelledPaidOrder(15, userIlyaID, offerMarketD3, "Планы изменились")
	orderSushi4 := completedOrder(16, userMaxID, offerSushiD2)
	orderSushi5 := completedOrder(17, userDaryaID, offerSushiD2)
	orderMarket4 := completedOrder(18, userKateID, offerMarketD1)
	orderBowl2 := completedOrder(19, userAnnaID, offerBowlD1)
	orderBowl3 := completedOrder(20, userMaxID, offerBowlToday)
	orderPaidID := paidOrder(21, userMaxID, currentSushi)
	orderCreatedID := createdOrder(22, userAnnaID, currentGroceries)
	createdOrder(23, userDaryaID, currentBakery)

	applyOfferAvailability(offers, orders)

	orderByID := ordersByID(orders)
	reviewForOrder := func(index int, orderID uuid.UUID, rating int, comment string) domain.Review {
		orderItem := orderByID[orderID]
		offerItem := offerByID[orderItem.OfferID]
		createdAt := orderItem.CreatedAt.Add(2 * time.Hour)
		if orderItem.CompletedAt != nil {
			createdAt = orderItem.CompletedAt.Add(45 * time.Minute)
		}
		return domain.Review{
			ID:           demoUUID(6, index),
			OrderID:      orderID,
			RestaurantID: offerItem.RestaurantID,
			UserID:       orderItem.UserID,
			Rating:       rating,
			Comment:      comment,
			CreatedAt:    createdAt,
		}
	}
	reviews := []domain.Review{
		reviewForOrder(1, orderBakery1, 5, "Свежая выпечка, удобно забрать после работы."),
		reviewForOrder(2, orderBakery2, 5, "Корзина была полной, хлеб еще теплый."),
		reviewForOrder(3, orderBakery3, 4, "Киш вкусный, десерт был чуть меньше ожиданий."),
		reviewForOrder(4, orderBakery4, 4, "Хороший набор для ужина, забрали без очереди."),
		reviewForOrder(5, orderBakery5, 5, "Чизкейк отличный, цена очень приятная."),
		reviewForOrder(6, orderSushi1, 5, "Сет свежий, роллов хватило на двоих."),
		reviewForOrder(7, orderSushi2, 4, "Хорошие десерты, матча яркая."),
		reviewForOrder(8, orderSushi3, 4, "Вкусно, но часть роллов была острой."),
		reviewForOrder(9, orderSushi4, 5, "Забрал быстро, упаковка аккуратная."),
		reviewForOrder(10, orderSushi5, 4, "Набор хороший, хотелось бы больше соуса."),
		reviewForOrder(11, orderMarket1, 5, "Овощи свежие, набор оказался больше ожидаемого."),
		reviewForOrder(12, orderMarket2, 5, "Смузи были холодные и с нормальным сроком."),
		reviewForOrder(13, orderMarket3, 4, "Все свежее, только зелень немного помялась."),
		reviewForOrder(14, orderMarket4, 5, "Отличный набор напитков по цене одного."),
		reviewForOrder(15, orderBowl1, 4, "Боул сытный, суп был еще теплый."),
		reviewForOrder(16, orderBowl2, 5, "Очень удачный набор после работы."),
		reviewForOrder(17, orderBowl3, 4, "Вкусно и быстро, салат мог быть больше."),
	}

	notices := []domain.Notification{
		notice(userAnnaID, "Заказ создан", "Овощи и фрукты ожидают оплаты.", "/orders/"+orderCreatedID.String(), "order_created", now.Add(-5*time.Minute)),
		notice(userMaxID, "Заказ оплачен", "Сет роллов Evening оплачен и ожидает выдачи.", "/orders/"+orderPaidID.String(), "order_paid", now.Add(-80*time.Minute)),
		notice(userKateID, "Спасибо за отзыв", "Ваш отзыв помогает другим пользователям.", "/restaurants/"+restBakeryID.String()+"/reviews", "review_created", orderByID[orderBakery5].CompletedAt.Add(45*time.Minute)),
		notice(userIlyaID, "Заказ отменен", "Овощной набор был отменен заранее.", "/orders/"+orderMarketCancelled.String(), "order_cancelled", orderByID[orderMarketCancelled].CancelledAt.Add(5*time.Minute)),
		notice(userDaryaID, "Заказ истек", "Резерв боула был отменен из-за истечения времени оплаты.", "/orders/"+orderBowlExpired.String(), "order_cancelled", orderByID[orderBowlExpired].CancelledAt.Add(5*time.Minute)),
		notice(userMaxID, "Заказ завершен", "Daily Bowl отметил заказ как выданный.", "/orders/"+orderBowl3.String(), "order_completed", orderByID[orderBowl3].CompletedAt.Add(5*time.Minute)),
	}

	analytics := analyticsRows(today, restaurants, offers, categories, orders, histories)

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

func order(id, userID, offerID uuid.UUID, number *string, status domain.OrderStatus, amount, commission float64, createdAt time.Time, paidAt, completedAt, cancelledAt, expiresAt *time.Time, cancelReason *string) domain.Order {
	serviceFee := 0.0
	netPayout := 0.0
	if status == domain.OrderCompleted {
		serviceFee = amount * commission / 100.0
		netPayout = amount - serviceFee
	}
	return domain.Order{
		ID:                 id,
		UserID:             userID,
		OfferID:            offerID,
		OrderNumber:        number,
		Status:             status,
		Amount:             amount,
		ServiceFee:         serviceFee,
		NetPayout:          netPayout,
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

func analyticsRows(today time.Time, restaurants []domain.Restaurant, offers []domain.Offer, categories []domain.Category, orders []domain.Order, histories []domain.OrderStatusHistory) []domain.DailyAnalytics {
	type analyticsKey struct {
		date         time.Time
		restaurantID uuid.UUID
		categoryID   uuid.UUID
	}
	type stat struct {
		bookings  int
		completed int
		cancelled int
		expired   int
		revenue   float64
	}

	offerByID := offersByID(offers)
	orderByID := ordersByID(orders)
	categoryByID := categoriesByID(categories)
	restaurantByID := restaurantsByID(restaurants)
	stats := map[analyticsKey]*stat{}

	statFor := func(date time.Time, offerID uuid.UUID) *stat {
		offerItem := offerByID[offerID]
		key := analyticsKey{date: dayStart(date.UTC()), restaurantID: offerItem.RestaurantID, categoryID: offerItem.CategoryID}
		if stats[key] == nil {
			stats[key] = &stat{}
		}
		return stats[key]
	}

	for _, orderItem := range orders {
		if orderItem.Status == domain.OrderCreated {
			continue
		}
		statFor(orderItem.CreatedAt, orderItem.OfferID).bookings++
	}
	for _, item := range histories {
		orderItem, ok := orderByID[item.OrderID]
		if !ok {
			continue
		}
		dayStat := statFor(item.ChangedAt, orderItem.OfferID)
		switch item.Status {
		case domain.OrderCompleted:
			dayStat.completed++
			dayStat.revenue += orderItem.Amount
		case domain.OrderCancelled:
			dayStat.cancelled++
			if orderItem.CancellationReason != nil && *orderItem.CancellationReason == "expired" {
				dayStat.expired++
			}
		}
	}

	rows := []domain.DailyAnalytics{}
	for day := analyticsSeedDays - 1; day >= 0; day-- {
		date := today.AddDate(0, 0, -day)
		seen := map[analyticsKey]struct{}{}
		for _, offerItem := range offers {
			key := analyticsKey{date: date, restaurantID: offerItem.RestaurantID, categoryID: offerItem.CategoryID}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			item, ok := stats[key]
			if !ok || (item.bookings == 0 && item.completed == 0 && item.cancelled == 0 && item.expired == 0 && item.revenue == 0) {
				continue
			}
			restaurantItem := restaurantByID[key.restaurantID]
			serviceFee := item.revenue * restaurantItem.Commission / 100.0
			rows = append(rows, domain.DailyAnalytics{
				ID:              uuid.New(),
				RestaurantID:    key.restaurantID,
				Date:            date,
				CategoryName:    categoryByID[key.categoryID].Name,
				TotalBookings:   item.bookings,
				CompletedOrders: item.completed,
				CancelledOrders: item.cancelled,
				ExpiredOrders:   item.expired,
				GrossRevenue:    item.revenue,
				ServiceFee:      serviceFee,
				NetPayout:       item.revenue - serviceFee,
				CreatedAt:       analyticsGeneratedAt(date),
			})
		}
	}
	return rows
}

func applyOfferAvailability(offers []domain.Offer, orders []domain.Order) {
	reservedOrSold := map[uuid.UUID]int{}
	for _, item := range orders {
		if item.Status == domain.OrderCreated || item.Status == domain.OrderPaid || item.Status == domain.OrderCompleted {
			reservedOrSold[item.OfferID]++
		}
	}
	for i := range offers {
		remaining := offers[i].QuantityTotal - reservedOrSold[offers[i].ID]
		if remaining < 0 {
			remaining = 0
		}
		offers[i].QuantityAvailable = remaining
		if remaining == 0 {
			offers[i].IsActive = false
		}
	}
}

func restaurantsByID(restaurants []domain.Restaurant) map[uuid.UUID]domain.Restaurant {
	items := make(map[uuid.UUID]domain.Restaurant, len(restaurants))
	for _, item := range restaurants {
		items[item.ID] = item
	}
	return items
}

func offersByID(offers []domain.Offer) map[uuid.UUID]domain.Offer {
	items := make(map[uuid.UUID]domain.Offer, len(offers))
	for _, item := range offers {
		items[item.ID] = item
	}
	return items
}

func ordersByID(orders []domain.Order) map[uuid.UUID]domain.Order {
	items := make(map[uuid.UUID]domain.Order, len(orders))
	for _, item := range orders {
		items[item.ID] = item
	}
	return items
}

func categoriesByID(categories []domain.Category) map[uuid.UUID]domain.Category {
	items := make(map[uuid.UUID]domain.Category, len(categories))
	for _, item := range categories {
		items[item.ID] = item
	}
	return items
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

func demoUUID(group, index int) uuid.UUID {
	return uuid.MustParse(fmt.Sprintf("%08d-%04d-%04d-%04d-%012d", group, group, group, group, index))
}

func stringPtr(value string) *string {
	return &value
}

func dayStart(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func dayAt(today time.Time, daysAgo int, hour, minute int) time.Time {
	date := today.AddDate(0, 0, -daysAgo)
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, time.UTC)
}

func analyticsGeneratedAt(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day()+1, 2, 0, 0, 0, time.UTC)
}
