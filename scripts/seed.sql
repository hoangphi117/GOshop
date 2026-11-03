-- ==============================================================================
-- GOSHOP DEVELOPMENT SEED DATA (multi-database)
--
-- Master data is seeded straight into each extracted-service database.
-- The monolith `shopping` database no longer exists, so nothing is copied
-- across databases (inventory still reads product UUIDs from
-- shopping_catalog via dblink, which is seeded just before it):
--   shopping_identity  -> users, user_addresses
--   shopping_catalog   -> categories, products + product upsert outbox rows
--   shopping_coupon    -> coupons                 (service schema)
--   shopping_cart      -> cleaned
--   shopping_inventory -> stock rows + movement history from catalog products
--
-- Password for all seeded users: Password123!
-- Run with psql (it understands \connect):
--   docker exec -i goshop-postgres psql -U postgres -d shopping_identity < scripts/seed.sql
-- Or via the Go seeder (parses the "-- DB:" markers, ignores \connect):
--   cd backend && go run ./gomod/cmd/seed
-- ==============================================================================

-- ##############################################################################
-- DB: shopping_identity
-- Identity-owned users and addresses (single source of truth).
-- ##############################################################################

\connect shopping_identity

-- Clean existing data safely (preserving schema)
TRUNCATE TABLE outbox_events, user_addresses, users RESTART IDENTITY CASCADE;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Default password hash for 'Password123!' (bcrypt cost 10)


    -- v_pwd was: VARCHAR(90) := '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC';


-- ==============================================================================
-- 1. USERS (Admin, Moderator, Customers)
-- ==============================================================================
INSERT INTO users (user_id, user_uuid, user_fullname, user_email, user_password, user_age, user_status, user_level, user_avatar, user_created_at, user_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 'System Admin', 'admin@goshop.com', '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC', 30, 1, 1, 'https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=150', now() - interval '90 days', now()),
(2, gen_random_uuid(), 'Store Manager', 'manager@goshop.com', '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC', 28, 1, 2, 'https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=150', now() - interval '85 days', now()),
(3, gen_random_uuid(), 'Support Moderator', 'mod@goshop.com', '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC', 26, 1, 2, 'https://images.unsplash.com/photo-1494790108377-be9c29b29330?w=150', now() - interval '80 days', now()),
(4, gen_random_uuid(), 'Nguyen Van An', 'user1@goshop.com', '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC', 25, 1, 3, 'https://images.unsplash.com/photo-1500648767791-00dcc994a43e?w=150', now() - interval '70 days', now()),
(5, gen_random_uuid(), 'Tran Thi Binh', 'user2@goshop.com', '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC', 24, 1, 3, 'https://images.unsplash.com/photo-1438761681033-6461ffad8d80?w=150', now() - interval '65 days', now()),
(6, gen_random_uuid(), 'Le Hoang Cuong', 'user3@goshop.com', '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC', 32, 1, 3, 'https://images.unsplash.com/photo-1472099645785-5658abf4ff4e?w=150', now() - interval '60 days', now()),
(7, gen_random_uuid(), 'Pham Minh Duc', 'user4@goshop.com', '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC', 29, 1, 3, 'https://images.unsplash.com/photo-1519085360753-af0119f7cbe7?w=150', now() - interval '55 days', now()),
(8, gen_random_uuid(), 'Hoang Thu Giang', 'user5@goshop.com', '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC', 22, 1, 3, 'https://images.unsplash.com/photo-1544005313-94ddf0286df2?w=150', now() - interval '50 days', now()),
(9, gen_random_uuid(), 'Vu Dinh Hai', 'user6@goshop.com', '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC', 35, 1, 3, 'https://images.unsplash.com/photo-1506794778202-cad84cf45f1d?w=150', now() - interval '45 days', now()),
(10, gen_random_uuid(), 'Do Thi Huong', 'user7@goshop.com', '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC', 27, 1, 3, 'https://images.unsplash.com/photo-1517841905240-472988babdf9?w=150', now() - interval '40 days', now()),
(11, gen_random_uuid(), 'Local Admin', 'admin@gmail.com', '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC', 30, 1, 1, 'https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=150', now() - interval '90 days', now());

-- Generate 40 additional realistic customers (IDs 12..51)
INSERT INTO users (user_id, user_uuid, user_fullname, user_email, user_password, user_age, user_status, user_level, user_avatar, user_created_at, user_updated_at)
OVERRIDING SYSTEM VALUE
SELECT
    i,
    gen_random_uuid(),
    (ARRAY['Nguyen', 'Tran', 'Le', 'Pham', 'Hoang', 'Phan', 'Vu', 'Dang', 'Bui', 'Do'])[1 + (i % 10)] || ' ' ||
    (ARRAY['Van', 'Thi', 'Duc', 'Quoc', 'Thanh', 'Ngoc', 'Minh', 'Huu', 'Gia', 'Hai'])[1 + ((i * 3) % 10)] || ' ' ||
    (ARRAY['Anh', 'Binh', 'Chau', 'Dung', 'Em', 'Giang', 'Huy', 'Khoa', 'Linh', 'Mai', 'Nam', 'Phong', 'Quan', 'Son', 'Tuan', 'Vy'])[1 + ((i * 7) % 16)],
    'customer' || i || '@testmail.com',
    '$2a$10$fjiFPJondn6AcV/GOEFjk.oIkOhMlqMHn.qeOlxoVQ6NTXMhV4BMC',
    20 + (i % 35),
    1,
    3,
    'https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=150',
    now() - (i || ' days')::interval,
    now()
FROM generate_series(12, 51) AS i;

SELECT setval('users_user_id_seq', (SELECT MAX(user_id) FROM users), true);

-- ==============================================================================
-- setval users_user_id_seq done above, (SELECT MAX(user_id) FROM users), true);

-- 2. USER ADDRESSES (Realistic Vietnamese addresses for customers)
-- ==============================================================================
INSERT INTO user_addresses (ua_id, ua_uuid, ua_user_id, ua_label, ua_name, ua_phone, ua_address, ua_is_default, ua_province_id, ua_province_name, ua_district_id, ua_district_name, ua_ward_code, ua_ward_name, ua_created_at, ua_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 4, 'Home', 'Nguyen Van An', '0912345678', 'Số 18, Ngõ 45, Đường Trần Duy Hưng, Cầu Giấy, Hà Nội', TRUE, 201, 'Hà Nội', 1488, 'Quận Cầu Giấy', '1A0101', 'Phường Trung Hòa', now() - interval '60 days', now()),
(2, gen_random_uuid(), 4, 'Office', 'Nguyen Van An', '0912345678', 'Tầng 12, Tòa nhà Landmark 72, Đường Phạm Hùng, Nam Từ Liêm, Hà Nội', FALSE, 201, 'Hà Nội', 1490, 'Quận Nam Từ Liêm', '1A0202', 'Phường Mễ Trì', now() - interval '40 days', now()),
(3, gen_random_uuid(), 5, 'Home', 'Tran Thi Binh', '0987654321', '245/12 Lê Văn Sỹ, Phường 14, Quận 3, TP. Hồ Chí Minh', TRUE, 202, 'TP. Hồ Chí Minh', 1442, 'Quận 3', '2B0301', 'Phường 14', now() - interval '58 days', now()),
(4, gen_random_uuid(), 6, 'Home', 'Le Hoang Cuong', '0903112233', '102 Nguyễn Văn Linh, Phường Nam Dương, Quận Hải Châu, Đà Nẵng', TRUE, 203, 'Đà Nẵng', 1520, 'Quận Hải Châu', '3C0101', 'Phường Nam Dương', now() - interval '55 days', now()),
(5, gen_random_uuid(), 7, 'Home', 'Pham Minh Duc', '0934556677', '56 Đường 30 Tháng 4, Phường An Phú, Quận Ninh Kiều, Cần Thơ', TRUE, 204, 'Cần Thơ', 1550, 'Quận Ninh Kiều', '4D0101', 'Phường An Phú', now() - interval '50 days', now()),
(6, gen_random_uuid(), 8, 'Home', 'Hoang Thu Giang', '0978990011', '88 Lạch Tray, Quận Ngô Quyền, Hải Phòng', TRUE, 205, 'Hải Phòng', 1580, 'Quận Ngô Quyền', '5E0101', 'Phường Lạch Tray', now() - interval '45 days', now());

-- Generate addresses for users 9 to 51
INSERT INTO user_addresses (ua_id, ua_uuid, ua_user_id, ua_label, ua_name, ua_phone, ua_address, ua_is_default, ua_province_id, ua_province_name, ua_district_id, ua_district_name, ua_ward_code, ua_ward_name, ua_created_at, ua_updated_at)
OVERRIDING SYSTEM VALUE
SELECT
    (u.user_id - 2),
    gen_random_uuid(),
    u.user_id,
    'Home',
    u.user_fullname,
    '09' || lpad((10000000 + u.user_id * 179)::text, 8, '0'),
    'Số ' || (u.user_id * 3 + 10) || ' Phố ' || (ARRAY['Nguyễn Huệ', 'Lê Lợi', 'Hoàng Hoa Thám', 'Võ Văn Kiệt', 'Hai Bà Trưng', 'Điện Biên Phủ'])[1 + (u.user_id % 6)] || ', TP. HCM',
    TRUE,
    202,
    'TP. Hồ Chí Minh',
    1440 + (u.user_id % 10),
    'Quận ' || (1 + (u.user_id % 10)),
    '2B00' || (u.user_id % 20),
    'Phường ' || (1 + (u.user_id % 15)),
    now() - (u.user_id || ' days')::interval,
    now()
FROM users u
WHERE u.user_id BETWEEN 9 AND 51;

SELECT setval('user_addresses_ua_id_seq', (SELECT MAX(ua_id) FROM user_addresses), true);

-- ==============================================================================
-- 

ANALYZE users;
ANALYZE user_addresses;

-- ##############################################################################
-- DB: shopping_catalog
-- Categories and products seed directly (no monolith copy).
-- ##############################################################################

\connect shopping_catalog

TRUNCATE TABLE outbox_events, products, categories RESTART IDENTITY CASCADE;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 1. CATEGORIES
-- ==============================================================================
INSERT INTO categories (category_id, category_uuid, category_name, category_slug, category_description, category_status, category_created_at, category_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 'Smartphones & Tablets', 'smartphones-tablets', 'High-end flagship smartphones, tablets, and mobile devices from top global brands.', 1, now() - interval '90 days', now()),
(2, gen_random_uuid(), 'Laptops & Computers', 'laptops-computers', 'Ultralight laptops, gaming rigs, workstations, and high-performance PC components.', 1, now() - interval '90 days', now()),
(3, gen_random_uuid(), 'Audio & Sound', 'audio-sound', 'Wireless earbuds, noise-cancelling headphones, soundbars, and premium speakers.', 1, now() - interval '90 days', now()),
(4, gen_random_uuid(), 'Smartwatches & Wearables', 'smartwatches-wearables', 'Next-generation fitness trackers, luxury smartwatches, and smart health wearables.', 1, now() - interval '90 days', now()),
(5, gen_random_uuid(), 'Gaming & Consoles', 'gaming-consoles', 'Next-gen consoles, mechanical gaming keyboards, mice, and immersive accessories.', 1, now() - interval '90 days', now()),
(6, gen_random_uuid(), 'Cameras & Photography', 'cameras-photography', 'Mirrorless cameras, 4K action cams, studio lighting, and pro lenses.', 1, now() - interval '90 days', now()),
(7, gen_random_uuid(), 'Smart Home & Appliances', 'smart-home-appliances', 'Robot vacuums, smart air purifiers, intelligent lighting, and home automation.', 1, now() - interval '90 days', now()),
(8, gen_random_uuid(), 'Accessories & Cables', 'accessories-cables', 'Fast chargers, GaN power adapters, braided cables, power banks, and hubs.', 1, now() - interval '90 days', now());

SELECT setval('categories_category_id_seq', (SELECT MAX(category_id) FROM categories), true);

-- ==============================================================================
-- 2. PRODUCTS (5000 generated products across 8 categories)
-- ==============================================================================
-- 32 curated rows keep hand-written flavor for the storefront homepage, then
-- generate_series fills to 5000 so pagination, loadtest and search behave like
-- a large store. Name pattern Brand + Line + Model + Variant guarantees unique
-- (product_name, category) and unique slug/sku without collisions.
-- ==============================================================================

INSERT INTO products (product_id, product_uuid, product_name, product_slug, product_description, product_price, product_sku, product_category_id, product_image_url, product_status, product_avg_rating, product_review_count, product_created_at, product_updated_at)
OVERRIDING SYSTEM VALUE VALUES
-- Category 1: Smartphones
(1, gen_random_uuid(), 'iPhone 16 Pro Max 256GB Titanium', 'iphone-16-pro-max-256gb', 'Apple A18 Pro chip, 6.9-inch Super Retina XDR display, Grade 5 titanium design, 48MP Fusion camera system with 5x telephoto.', 34990000.00, 'IPH16PM-256-TI', 1, 'https://images.unsplash.com/photo-1695048133142-1a20484d2569?w=800', 1, 4.9, 12, now() - interval '60 days', now()),
(2, gen_random_uuid(), 'Samsung Galaxy S24 Ultra 512GB Titanium Gray', 'samsung-galaxy-s24-ultra-512gb', 'Snapdragon 8 Gen 3 for Galaxy, Galaxy AI built-in, 200MP camera, Quad Tele System, Titanium frame with embedded S Pen.', 31990000.00, 'SGS24U-512-TG', 1, 'https://images.unsplash.com/photo-1610945265064-0e34e5519bbf?w=800', 1, 4.8, 10, now() - interval '55 days', now()),
(3, gen_random_uuid(), 'Xiaomi 14 Ultra 512GB Leica Edition', 'xiaomi-14-ultra-512gb-leica', 'Snapdragon 8 Gen 3, All Around Liquid Display, Leica quad-camera setup with 1-inch sensor, 90W HyperCharge wired + 80W wireless.', 27990000.00, 'XM14U-512-BLK', 1, 'https://images.unsplash.com/photo-1598327105666-5b89351aff97?w=800', 1, 4.7, 8, now() - interval '50 days', now()),
(4, gen_random_uuid(), 'Google Pixel 9 Pro XL 256GB Obsidian', 'google-pixel-9-pro-xl-256gb', 'Google Tensor G4 with Gemini Nano, Super Actua display, triple pro camera system with 30x Super Res Zoom, 7 years of OS updates.', 28490000.00, 'PIX9PXL-256-OB', 1, 'https://images.unsplash.com/photo-1592899677977-9c10ca588bbd?w=800', 1, 4.8, 6, now() - interval '45 days', now()),
(5, gen_random_uuid(), 'iPad Pro 13-inch M4 Ultra Retina OLED', 'ipad-pro-13-inch-m4-oled', 'Breakthrough Apple M4 chip, Ultra Retina XDR with Tandem OLED technology, 5.1mm ultra-thin enclosure, Apple Pencil Pro support.', 37990000.00, 'IPP13-M4-256-SLV', 1, 'https://images.unsplash.com/photo-1544244015-0df4b3ffc6b0?w=800', 1, 4.9, 5, now() - interval '40 days', now()),
(6, gen_random_uuid(), 'Samsung Galaxy Tab S9 Ultra 256GB 5G', 'samsung-galaxy-tab-s9-ultra-5g', '14.6-inch Dynamic AMOLED 2X 120Hz display, IP68 water & dust resistance, Snapdragon 8 Gen 2, included S Pen with ultra-low latency.', 25990000.00, 'SGT-S9U-256-GR', 1, 'https://images.unsplash.com/photo-1561154464-82e9adf32764?w=800', 1, 4.6, 4, now() - interval '35 days', now()),

-- Category 2: Laptops & Computers
(7, gen_random_uuid(), 'MacBook Pro 16-inch M3 Max 36GB / 1TB Space Black', 'macbook-pro-16-m3-max-36gb', '16-core CPU, 40-core GPU, Liquid Retina XDR display with ProMotion 120Hz, up to 22 hours battery life, hardware ray tracing.', 89990000.00, 'MBP16-M3MAX-1TB', 2, 'https://images.unsplash.com/photo-1517336714731-489689fd1ca8?w=800', 1, 5.0, 7, now() - interval '50 days', now()),
(8, gen_random_uuid(), 'Dell XPS 16 OLED 9640 Core Ultra 9 / RTX 4070', 'dell-xps-16-oled-9640-core-ultra-9', 'Intel Core Ultra 9 185H, 32GB LPDDR5x, 1TB SSD, 16.3-inch 4K+ OLED Touch display, CNC machined aluminum chassis.', 68500000.00, 'DELL-XPS16-9640', 2, 'https://images.unsplash.com/photo-1593642632823-8f785ba67e45?w=800', 1, 4.7, 5, now() - interval '45 days', now()),
(9, gen_random_uuid(), 'ASUS ROG Zephyrus G16 OLED Ryzen AI 9 / RTX 4080', 'asus-rog-zephyrus-g16-oled-2024', 'AMD Ryzen AI 9 HX 370, 32GB LPDDR5X, 2TB PCIe 4.0 NVMe, 2.5K 240Hz ROG Nebula OLED, Slash Lighting CNC unibody.', 72990000.00, 'ROG-G16-GA605', 2, 'https://images.unsplash.com/photo-1603302576837-37561b2e2302?w=800', 1, 4.8, 9, now() - interval '40 days', now()),
(10, gen_random_uuid(), 'MacBook Air 15-inch M3 16GB / 512GB Midnight', 'macbook-air-15-m3-16gb-512gb', 'Incredibly thin, fanless silent operation, Liquid Retina display, MagSafe 3 charging, dual USB-C Thunderbolt 4 ports.', 36990000.00, 'MBA15-M3-16-512', 2, 'https://images.unsplash.com/photo-1611186871348-b1ce696e52c9?w=800', 1, 4.9, 11, now() - interval '38 days', now()),
(11, gen_random_uuid(), 'Lenovo ThinkPad X1 Carbon Gen 12 Core Ultra 7', 'lenovo-thinkpad-x1-carbon-gen12', 'Intel Core Ultra 7 155H, 32GB RAM, 1TB SSD, 2.8K OLED Display, legendary ergonomic TrackPoint keyboard, MIL-STD 810H certified.', 54900000.00, 'TP-X1C-G12-U7', 2, 'https://images.unsplash.com/photo-1588872657578-7efd1f1555ed?w=800', 1, 4.7, 4, now() - interval '32 days', now()),

-- Category 3: Audio & Sound
(12, gen_random_uuid(), 'Sony WH-1000XM5 Wireless Noise Cancelling Headphones', 'sony-wh-1000xm5-wireless-black', 'Industry-leading noise cancellation with 8 mics and Auto NC Optimizer, LDAC High-Res audio, 30 hours battery life with quick charging.', 7990000.00, 'SONY-WH1000XM5-B', 3, 'https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=800', 1, 4.8, 15, now() - interval '60 days', now()),
(13, gen_random_uuid(), 'Apple AirPods Pro 2 USB-C with MagSafe Case', 'apple-airpods-pro-2-usb-c', 'Apple H2 chip, Pro-level Active Noise Cancellation, Adaptive Audio, Transparency mode, Personalized Spatial Audio with dynamic head tracking.', 5890000.00, 'APP2-USBC-WHT', 3, 'https://images.unsplash.com/photo-1600294037681-c80b4cb5b434?w=800', 1, 4.9, 18, now() - interval '55 days', now()),
(14, gen_random_uuid(), 'Bose QuietComfort Ultra Headphones Spatial Audio', 'bose-quietcomfort-ultra-headphones', 'CustomTune technology, world-class noise cancelling, Bose Immersive Audio, luxurious ultra-soft protein leather ear cushions.', 9490000.00, 'BOSE-QCU-BLK', 3, 'https://images.unsplash.com/photo-1546435770-a3e426bf472b?w=800', 1, 4.7, 8, now() - interval '42 days', now()),
(15, gen_random_uuid(), 'Marshall Stanmore III Bluetooth Home Speaker', 'marshall-stanmore-iii-bluetooth-black', 'Iconic vintage Marshall design, room-filling stereo soundstage, Dynamic Loudness, Bluetooth 5.2, 3.5mm AUX & RCA inputs.', 8990000.00, 'MARSH-STAN3-BLK', 3, 'https://images.unsplash.com/photo-1545454675-3531b543be5d?w=800', 1, 4.9, 10, now() - interval '40 days', now()),
(16, gen_random_uuid(), 'Sennheiser Momentum True Wireless 4 Earbuds', 'sennheiser-momentum-tw-4-copper', 'TrueResponse transducer system, Lossless audio with aptX Adaptive, Auracast support, Adaptive ANC with smart anti-wind mode.', 7490000.00, 'SENN-MTW4-COP', 3, 'https://images.unsplash.com/photo-1590658268037-6bf12165a8df?w=800', 1, 4.6, 6, now() - interval '30 days', now()),

-- Category 4: Smartwatches & Wearables
(17, gen_random_uuid(), 'Apple Watch Ultra 2 GPS + Cellular 49mm Titanium', 'apple-watch-ultra-2-49mm-titanium', 'S9 SiP chip, 3000 nits brightness, dual-frequency precision GPS, 36 hours normal battery (up to 72 hrs Low Power), 100m water resistant.', 21490000.00, 'AWU2-49-TI-OR', 4, 'https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=800', 1, 4.9, 13, now() - interval '50 days', now()),
(18, gen_random_uuid(), 'Garmin Fenix 8 Solar Sapphire AMOLED 47mm', 'garmin-fenix-8-solar-sapphire-47mm', 'Solar charging lens, built-in mic & speaker for calls, top-tier topographic maps, multisport endurance metrics, LED flashlight.', 26990000.00, 'GARMIN-FNX8-47', 4, 'https://images.unsplash.com/photo-1508685096489-7aacd43bd3b1?w=800', 1, 5.0, 9, now() - interval '45 days', now()),
(19, gen_random_uuid(), 'Samsung Galaxy Watch Ultra 47mm Titanium Silver', 'samsung-galaxy-watch-ultra-47mm', 'Exynos W1000 3nm processor, Sapphire crystal, BioActive Sensor, dual-frequency GPS, Emergency Siren, 100-hour battery life.', 16490000.00, 'SGW-U-47-SLV', 4, 'https://images.unsplash.com/photo-1579586337278-3befd40fd17a?w=800', 1, 4.6, 7, now() - interval '38 days', now()),

-- Category 5: Gaming & Consoles
(20, gen_random_uuid(), 'PlayStation 5 Pro Console 2TB Digital Edition', 'playstation-5-pro-console-2tb', 'PlayStation Spectral Super Resolution (PSSR), advanced ray tracing, 60FPS high-fidelity gaming, custom 2TB ultra-fast SSD.', 19990000.00, 'PS5-PRO-2TB', 5, 'https://images.unsplash.com/photo-1606813907291-d86efa9b94db?w=800', 1, 4.9, 16, now() - interval '35 days', now()),
(21, gen_random_uuid(), 'Nintendo Switch OLED Model Mario Red Edition', 'nintendo-switch-oled-mario-red', '7-inch vibrant OLED screen, wide adjustable stand, wired LAN dock, 64GB internal storage, enhanced audio.', 8490000.00, 'NSW-OLED-MRED', 5, 'https://images.unsplash.com/photo-1578303512597-81e6cc155b3e?w=800', 1, 4.8, 12, now() - interval '48 days', now()),
(22, gen_random_uuid(), 'Logitech G PRO X SUPERLIGHT 2 Wireless Gaming Mouse', 'logitech-g-pro-x-superlight-2-magenta', 'LIGHTFORCE hybrid optical-mechanical switches, HERO 2 sensor with 32,000 DPI and 4,000Hz polling rate, ultra-lightweight 60 grams.', 3690000.00, 'LOGI-GPX2-MAG', 5, 'https://images.unsplash.com/photo-1615663245857-ac93bb7c39e7?w=800', 1, 4.8, 14, now() - interval '52 days', now()),
(23, gen_random_uuid(), 'Keychron Q1 Pro Wireless Custom Mechanical Keyboard', 'keychron-q1-pro-wireless-qmk', 'Full CNC aluminum body, double-gasket design, hot-swappable Keychron K Pro Banana switches, QMK/VIA programmable, Mac/Win support.', 4590000.00, 'KC-Q1PRO-BAN', 5, 'https://images.unsplash.com/photo-1587829741301-dc798b83add3?w=800', 1, 4.9, 10, now() - interval '44 days', now()),

-- Category 6: Cameras
(24, gen_random_uuid(), 'Sony Alpha A7 IV Mirrorless Camera Body', 'sony-alpha-a7-iv-body', '33MP full-frame Exmor R CMOS sensor, 4K 60p 10-bit video, real-time Eye AF for humans/animals/birds, 7K oversampled recording.', 53990000.00, 'SONY-ILCE-7M4', 6, 'https://images.unsplash.com/photo-1516035069371-29a1b244cc32?w=800', 1, 4.9, 5, now() - interval '46 days', now()),
(25, gen_random_uuid(), 'Fujifilm X100VI Digital Camera Silver Edition', 'fujifilm-x100vi-silver', '40.2MP X-Trans CMOS 5 HR sensor, 6.0 stops in-body image stabilization (IBIS), 20 Film Simulation modes including REALA ACE.', 44990000.00, 'FUJI-X100VI-SLV', 6, 'https://images.unsplash.com/photo-1502920917128-1aa500764cbd?w=800', 1, 5.0, 8, now() - interval '30 days', now()),
(26, gen_random_uuid(), 'DJI Pocket 3 Creator Combo 4K 120fps Vlog Camera', 'dji-pocket-3-creator-combo', '1-inch CMOS sensor, 2-inch rotatable OLED touchscreen, 3-axis mechanical gimbal, D-Log M 10-bit color, included DJI Mic 2 transmitter.', 15890000.00, 'DJI-OP3-COMBO', 6, 'https://images.unsplash.com/photo-1526170375885-4d8ecf77b99f?w=800', 1, 4.9, 12, now() - interval '36 days', now()),

-- Category 7: Smart Home & Appliances
(27, gen_random_uuid(), 'Roborock S8 Pro Ultra Robot Vacuum and Mop', 'roborock-s8-pro-ultra-robot-vacuum', 'RockDock Ultra all-in-one station with auto-washing, auto-drying, auto-emptying and auto-refilling. 6000Pa HyperForce suction.', 26990000.00, 'ROBO-S8PU-BLK', 7, 'https://images.unsplash.com/photo-1558317374-067fb5f30001?w=800', 1, 4.8, 7, now() - interval '40 days', now()),
(28, gen_random_uuid(), 'Dyson Purifier Hot+Cool Formaldehyde HP09 Air Purifier', 'dyson-purifier-hot-cool-hp09', 'Destroys formaldehyde permanently, HEPA H13 fully sealed filtration, cools in summer and heats in winter, Air Multiplier tech.', 19990000.00, 'DYSON-HP09-WHT', 7, 'https://images.unsplash.com/photo-1585771724684-38269d6639fd?w=800', 1, 4.7, 6, now() - interval '34 days', now()),
(29, gen_random_uuid(), 'Philips Hue Smart Bridge & White and Color Starter Kit', 'philips-hue-starter-kit-e27', '16 million colors and 50,000 shades of warm-to-cool white light. Includes 3 smart bulbs, Hue Bridge, and smart dimmer switch.', 4890000.00, 'PHILIPS-HUE-KIT', 7, 'https://images.unsplash.com/photo-1550751827-4bd374c3f58b?w=800', 1, 4.8, 9, now() - interval '42 days', now()),

-- Category 8: Accessories & Cables
(30, gen_random_uuid(), 'Anker Prime 27,650mAh Power Bank (250W 3-Port)', 'anker-prime-250w-power-bank-27650mah', 'Massive 250W multi-device fast charging output, smart digital display with live wattage, Anker app Bluetooth pairing.', 3490000.00, 'ANKER-PRIME-250W', 8, 'https://images.unsplash.com/photo-1609091839311-d5365f9ff1c5?w=800', 1, 4.9, 20, now() - interval '55 days', now()),
(31, gen_random_uuid(), 'Belkin BoostCharge Pro 3-in-1 MagSafe Wireless Charger', 'belkin-boostcharge-pro-3-in-1-magsafe', 'Official 15W MagSafe fast charging for iPhone, fast charge module for Apple Watch Ultra, dedicated Qi pad for AirPods.', 3890000.00, 'BELKIN-WIZ017-BLK', 8, 'https://images.unsplash.com/photo-1583863788434-e58a36330cf0?w=800', 1, 4.7, 11, now() - interval '48 days', now()),
(32, gen_random_uuid(), 'UGREEN Nexode 300W GaN 5-Port Desktop Charger', 'ugreen-nexode-300w-gan-5port', 'World first 300W 5-port GaN fast charger with PD 3.1 140W single port power, thermal guard safety protection system.', 2790000.00, 'UGREEN-CD333-300W', 8, 'https://images.unsplash.com/photo-1544717305-2782549b5136?w=800', 1, 4.8, 14, now() - interval '44 days', now());

-- Generated catalogue: ids 33..5000 (4,968 rows).
-- Each category gets its own brand/line vocabulary so search results look
-- plausible instead of 5,000 copies of the same label.
INSERT INTO products (product_id, product_uuid, product_name, product_slug, product_description, product_price, product_sku, product_category_id, product_image_url, product_status, product_avg_rating, product_review_count, product_created_at, product_updated_at)
OVERRIDING SYSTEM VALUE
WITH vocab AS (
    SELECT * FROM (VALUES
        (1, ARRAY['Samsung','Xiaomi','OPPO','vivo','Google','OnePlus','Honor','Realme','Nokia','Sony','ZTE','Motorola'],
               ARRAY['Galaxy','Redmi','Find','X','Pixel','Nord','Magic','GT','G','Xperia','Axon','Edge'],
               ARRAY['128GB','256GB','512GB','1TB','Lite','Pro','Ultra','Plus']),
        (2, ARRAY['ASUS','Lenovo','HP','Acer','MSI','Dell','Apple','Razer','Huawei','LG'],
               ARRAY['Vivobook','IdeaPad','Pavilion','Aspire','Raider','XPS','MacBook','Blade','MateBook','Gram'],
               ARRAY['14 Core i5','14 Core i7','15 Ryzen 5','15 Ryzen 7','16 RTX 4060','16 RTX 4070','13 Ultra 5']),
        (3, ARRAY['Sony','JBL','Bose','Sennheiser','Marshall','Anker','Edifier','Beats','Shure','Audio-Technica'],
               ARRAY['WH','Charge','QuietComfort','Momentum','Emberton','Soundcore','R','Studio','AONIC','ATH'],
               ARRAY['ANC','Wireless','Sport','Studio','Portable','Bass']),
        (4, ARRAY['Apple','Garmin','Samsung','Huawei','Amazfit','Fitbit','Suunto','Polar'],
               ARRAY['Watch','Fenix','Galaxy Watch','Watch GT','GTS','Versa','Spartan','Vantage'],
               ARRAY['41mm','43mm','45mm','47mm','GPS','Solar','Titanium']),
        (5, ARRAY['Logitech','Razer','SteelSeries','Corsair','HyperX','ASUS ROG','MSI','Sony','Microsoft','Nintendo'],
               ARRAY['G','BlackWidow','Apex','K70','Cloud','Strix','MAG','DualSense','Xbox','Switch'],
               ARRAY['Keyboard','Mouse','Headset','Controller','Console','Pad']),
        (6, ARRAY['Canon','Nikon','Sony','Fujifilm','Panasonic','GoPro','DJI','Insta360','Olympus','Leica'],
               ARRAY['EOS','Z','Alpha','X-T','Lumix','HERO','Osmo','X','OM','Q'],
               ARRAY['Body','Kit 18-55mm','4K Video','Action Cam','Travel Kit']),
        (7, ARRAY['Xiaomi','Roborock','Dyson','Philips','Ecovacs','Dreame','Panasonic','LG','Sharp','TCL'],
               ARRAY['Mi','S','V','Hue','Deebot','Bot','Air','ThinQ','Spot','P'],
               ARRAY['Robot Vacuum','Air Purifier','Smart Bulb Kit','Smart Plug','Humidifier']),
        (8, ARRAY['Anker','UGREEN','Belkin','Baseus','Apple','Samsung','Spigen','JBL','Logitech','Sandisk'],
               ARRAY['PowerCore','Nexode','BoostCharge','GaN','MagSafe','Travel','Armor','Flip','MX','Extreme'],
               ARRAY['65W Charger','100W Charger','USB-C 2m','Power Bank 20000mAh','Hub 7-in-1','SSD 1TB'])
    ) AS v(cat_id, brands, lines, variants)
)
SELECT
    32 + n,
    gen_random_uuid(),
    v.brands[1 + (n % array_length(v.brands, 1))] || ' ' ||
        v.lines[1 + ((n / 5) % array_length(v.lines, 1))] || ' ' ||
        v.variants[1 + ((n / 11) % array_length(v.variants, 1))] || ' ' ||
        model.model_label || ' #' || n::text,
    lower(regexp_replace(
        v.brands[1 + (n % array_length(v.brands, 1))] || '-' ||
        v.lines[1 + ((n / 5) % array_length(v.lines, 1))] || '-' ||
        v.variants[1 + ((n / 11) % array_length(v.variants, 1))] || '-' ||
        model.model_label || '-' || (32 + n)::text, '[^a-zA-Z0-9]+', '-', 'g')),
    v.brands[1 + (n % array_length(v.brands, 1))] || ' ' ||
        v.lines[1 + ((n / 5) % array_length(v.lines, 1))] ||
        ': everyday device for daily use, warranty 24 months, fulfilled by GOshop.',
    (400000 + ((n * 7919) % 640)) * 5000,
    upper(left(regexp_replace(v.brands[1 + (n % array_length(v.brands, 1))], '[^a-zA-Z0-9]+', '', 'g'), 3)) ||
        (32 + n)::text || '-' || lpad(n::text, 4, '0'),
    v.cat_id,
    'https://picsum.photos/seed/goshop-' || (32 + n) || '/800/800',
    1,
    round((3.5 + ((n * 37) % 15)::numeric / 10)::numeric, 1),
    (n * 13) % 240,
    now() - ((n % 400) || ' days')::interval,
    now()
FROM generate_series(1, 4968) AS n
JOIN vocab v ON v.cat_id = 1 + (n % 8)
CROSS JOIN LATERAL (
    SELECT
        (ARRAY['S24','SE','SE2','SE3','Plus','Prime','Max','Air','Neo','Eco','Lite','Turbo'])[1 + (n % 12)] ||
        ' ' || (100 + (n % 900)) AS model_label
) AS model;

SELECT setval('products_product_id_seq', (SELECT MAX(product_id) FROM products), true);

-- Legacy monolith stock column mirrors the inventory quantities seeded below
-- (inventory is the source of truth; this only keeps old search docs accurate).

SELECT setval('products_product_id_seq', (SELECT MAX(product_id) FROM products), true);

-- Queue a search reindex for every seeded product. The catalog outbox worker
-- (running in the catalog container) consumes these, rebuilds the Meilisearch
-- documents from the product rows and marks them PROCESSED. Without this the
-- index keeps UUIDs from a previous seed and product search returns rows the
-- inventory service can no longer resolve.
INSERT INTO outbox_events (event_type, aggregate_type, aggregate_id, payload, status)
SELECT 'PRODUCT_UPSERT', 'product', p.product_uuid::text, '{}'::jsonb, 'PENDING'
FROM products p
WHERE p.product_deleted_at IS NULL;

ANALYZE categories;
ANALYZE products;

-- ##############################################################################
-- DB: shopping_coupon
-- Coupons use the coupon-service schema directly.
-- ##############################################################################

\connect shopping_coupon

TRUNCATE TABLE coupon_usages, coupons RESTART IDENTITY CASCADE;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

INSERT INTO coupons (coupon_id, coupon_uuid, coupon_code, coupon_name, coupon_description, coupon_type, coupon_discount_value, coupon_max_discount_amount, coupon_min_order_amount, coupon_usage_limit, coupon_used_count, coupon_start_date, coupon_end_date, coupon_is_active, coupon_created_at, coupon_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 'WELCOME10', 'Welcome New Shopper 10%', 'Enjoy 10% discount on your first tech gadget purchase.', 1, 10.00, 500000.00, 1000000.00, 1000, 24, now() - interval '60 days', now() + interval '180 days', TRUE, now() - interval '60 days', now()),
(2, gen_random_uuid(), 'GOSHOP500K', 'Super Mega 500K Off', 'Direct 500,000 VND discount for orders over 15,000,000 VND.', 2, 500000.00, 500000.00, 15000000.00, 500, 48, now() - interval '45 days', now() + interval '120 days', TRUE, now() - interval '45 days', now()),
(3, gen_random_uuid(), 'TECHVIP15', 'VIP Tech Enthusiast 15%', 'Exclusive 15% discount capped at 2,000,000 VND.', 1, 15.00, 2000000.00, 5000000.00, 200, 15, now() - interval '30 days', now() + interval '90 days', TRUE, now() - interval '30 days', now()),
(4, gen_random_uuid(), 'FLASH100K', 'Flash Sale 100K Voucher', 'Direct 100,000 VND discount on accessories.', 2, 100000.00, 100000.00, 1000000.00, 2000, 85, now() - interval '15 days', now() + interval '45 days', TRUE, now() - interval '15 days', now()),
(5, gen_random_uuid(), 'BLACKFRIDAY', 'Black Friday Ultra 20%', 'Black Friday 20% discount on all flagship laptops.', 1, 20.00, 3000000.00, 10000000.00, 100, 100, now() - interval '100 days', now() - interval '70 days', FALSE, now() - interval '105 days', now());

SELECT setval('coupons_coupon_id_seq', (SELECT MAX(coupon_id) FROM coupons), true);

ANALYZE coupons;

-- ##############################################################################
-- DB: shopping_cart
-- Clean slate; carts are created by user activity.
-- ##############################################################################

\connect shopping_cart

TRUNCATE TABLE cart_items, carts RESTART IDENTITY CASCADE;

-- ##############################################################################
-- DB: shopping_inventory
-- One stock row + INITIAL_STOCK movement per catalog product, so the audit log
-- and dashboards have realistic history. Quantities match the legacy
-- product_stock mirror in the monolith.
-- ##############################################################################

\connect shopping_inventory

TRUNCATE TABLE stock_movements, inventory_stocks RESTART IDENTITY CASCADE;
CREATE EXTENSION IF NOT EXISTS dblink;

INSERT INTO inventory_stocks (product_id, product_uuid, quantity)
SELECT product_id, product_uuid, 20 + (product_id * 13) % 100
FROM dblink('dbname=shopping_catalog', 'SELECT product_id, product_uuid FROM products WHERE product_deleted_at IS NULL')
AS t(product_id int, product_uuid uuid);

INSERT INTO stock_movements (product_id, product_uuid, change_type, quantity_change, stock_before, stock_after, reference_type, reference_id, note, created_at)
SELECT product_id, product_uuid, 'INITIAL_STOCK', quantity, 0, quantity, 'INITIAL_STOCK', 'seed-' || product_uuid::text, 'Seeded initial stock', now() - (product_id || ' hours')::interval
FROM inventory_stocks;

-- A few extra movement types so the dashboard log and filters have variety.
INSERT INTO stock_movements (product_id, product_uuid, change_type, quantity_change, stock_before, stock_after, reference_type, note, created_at)
SELECT product_id, product_uuid, 'RESTOCK', 10, quantity, quantity + 10, 'manual', 'Seeded restock example', now() - interval '30 minutes'
FROM inventory_stocks WHERE product_id IN (1, 2, 12);
UPDATE inventory_stocks SET quantity = quantity + 10, updated_at = now() WHERE product_id IN (1, 2, 12);

INSERT INTO stock_movements (product_id, product_uuid, change_type, quantity_change, stock_before, stock_after, reference_type, note, created_at)
SELECT product_id, product_uuid, 'DAMAGED_DISCARD', -3, quantity, quantity - 3, 'manual', 'Seeded damaged goods example', now() - interval '15 minutes'
FROM inventory_stocks WHERE product_id IN (5, 20);
UPDATE inventory_stocks SET quantity = quantity - 3, updated_at = now() WHERE product_id IN (5, 20);

ANALYZE inventory_stocks;
ANALYZE stock_movements;
