-- +goose Up
-- +goose StatementBegin

-- 1. Расширения
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "postgis";

-- 2. Таблица Категорий
CREATE TABLE IF NOT EXISTS categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    icon_url TEXT
    );

-- 3. Таблица Пользователей
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    phone TEXT,
    role TEXT NOT NULL,
    restaurant_id UUID,
    partner_status TEXT,
    is_verified BOOLEAN NOT NULL DEFAULT false,
    is_blocked BOOLEAN NOT NULL DEFAULT false,
    device_token TEXT,
    auth_provider TEXT,
    password_hash TEXT,
    name TEXT NOT NULL,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
    );
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

-- 4. Таблица Ресторанов
CREATE TABLE IF NOT EXISTS restaurants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    partner_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    company_name TEXT,
    inn TEXT,
    address TEXT NOT NULL,
    description TEXT,
    commission DECIMAL(10,2) NOT NULL DEFAULT 0,
    rating DECIMAL(3,2) DEFAULT 0,
    review_count INT DEFAULT 0,
    logo_url TEXT,
    cover_url TEXT,
    phone TEXT,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    working_hours TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT restaurants_latitude_range CHECK (latitude >= -90 AND latitude <= 90),
    CONSTRAINT restaurants_longitude_range CHECK (longitude >= -180 AND longitude <= 180),
    location geography(Point, 4326) GENERATED ALWAYS AS (
         ST_SetSRID(ST_MakePoint(longitude, latitude), 4326)::geography
    ) STORED
    );
CREATE INDEX IF NOT EXISTS idx_restaurants_location_geog ON restaurants USING GIST (location);

-- 5. Таблица Предложений
CREATE TABLE IF NOT EXISTS offers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id),
    category_id UUID NOT NULL REFERENCES categories(id),
    title TEXT NOT NULL,
    description TEXT,
    image_url TEXT,
    price DECIMAL(10,2) NOT NULL,
    original_price DECIMAL(10,2) NOT NULL,
    quantity_available INT NOT NULL,
    quantity_total INT NOT NULL,
    pickup_time_start TIMESTAMPTZ NOT NULL,
    pickup_time_end TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true
    );

-- 6. Таблица Заказов
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    offer_id UUID NOT NULL REFERENCES offers(id),
    order_number VARCHAR(20),
    status VARCHAR(20) NOT NULL,
    amount DECIMAL(10,2) NOT NULL,
    service_fee DECIMAL(10,2) NOT NULL DEFAULT 0,
    net_payout DECIMAL(10,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    cancellation_reason TEXT
    );
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
CREATE INDEX IF NOT EXISTS idx_orders_offer_id ON orders(offer_id);

-- 7. История статусов
CREATE TABLE IF NOT EXISTS order_status_histories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id),
    status VARCHAR(20) NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL
    );

-- 8. Уведомления
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    title TEXT,
    body TEXT,
    deep_link TEXT,
    type TEXT,
    created_at TIMESTAMPTZ NOT NULL
    );

-- 9. Отзывы
CREATE TABLE IF NOT EXISTS reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID UNIQUE NOT NULL REFERENCES orders(id),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id),
    user_id UUID NOT NULL REFERENCES users(id),
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL
    );

-- 10. Аналитика
CREATE TABLE IF NOT EXISTS daily_analytics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id UUID NOT NULL REFERENCES restaurants(id),
    date DATE NOT NULL,
    category_name TEXT NOT NULL,
    total_bookings INT DEFAULT 0,
    completed_orders INT DEFAULT 0,
    cancelled_orders INT DEFAULT 0,
    expired_orders INT DEFAULT 0,
    gross_revenue DECIMAL(10,2) DEFAULT 0,
    service_fee DECIMAL(10,2) DEFAULT 0,
    net_payout DECIMAL(10,2) DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE(restaurant_id, date, category_name)
    );

-- +goose StatementEnd

-- +goose Down
-- (Здесь можно прописать DROP TABLE, если захочешь чистить базу)