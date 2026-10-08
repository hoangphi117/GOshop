-- ==============================================================================
-- GOSHOP LOAD TEST & DEVELOPMENT SEED DATA
-- Purpose: Rich, realistic mock data for load testing, stress testing & dev
-- Password for all generated users: Password123!
-- Sections 1-12: curated base data (32 hand-written products, 50 users, ~50 orders)
-- Section 13   : scale-up (default ~5k products, ~2k users, ~8k orders). Tune v_extra_* below.
-- ==============================================================================

BEGIN;

-- Clean existing data safely (preserving schema)
TRUNCATE TABLE 
    chat_messages,
    chat_conversations,
    notifications,
    stock_movements,
    product_review_reports,
    product_review_helpful_votes,
    product_reviews,
    order_returns,
    order_tracking_events,
    payment_transactions,
    order_items,
    orders,
    coupon_usages,
    coupons,
    cart_items,
    carts,
    user_addresses,
    banners,
    products,
    categories,
    users
RESTART IDENTITY CASCADE;

-- Default password hash for 'Password123!' (bcrypt cost 10)
-- $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
DO $$
DECLARE
    v_pwd VARCHAR(90) := '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy';
    -- Quy mô dữ liệu sinh thêm (section 13)
    v_extra_users    INT := 2000;
    v_extra_products INT := 5000;
    v_extra_orders   INT := 8000;
    v_extra_chats    INT := 150;
    v_user_base INT; v_prod_base INT; v_order_base INT; v_conv_base INT; v_total_products INT;
BEGIN

-- ==============================================================================
-- 1. USERS (Admin, Moderator, Customers)
-- ==============================================================================
INSERT INTO users (user_id, user_uuid, user_fullname, user_email, user_password, user_age, user_status, user_level, user_avatar, user_created_at, user_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 'System Admin', 'admin@goshop.com', v_pwd, 30, 1, 1, 'https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=150', now() - interval '90 days', now()),
(2, gen_random_uuid(), 'Store Manager', 'manager@goshop.com', v_pwd, 28, 1, 2, 'https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=150', now() - interval '85 days', now()),
(3, gen_random_uuid(), 'Support Moderator', 'mod@goshop.com', v_pwd, 26, 1, 2, 'https://images.unsplash.com/photo-1494790108377-be9c29b29330?w=150', now() - interval '80 days', now()),
(4, gen_random_uuid(), 'Nguyen Van An', 'user1@goshop.com', v_pwd, 25, 1, 3, 'https://images.unsplash.com/photo-1500648767791-00dcc994a43e?w=150', now() - interval '70 days', now()),
(5, gen_random_uuid(), 'Tran Thi Binh', 'user2@goshop.com', v_pwd, 24, 1, 3, 'https://images.unsplash.com/photo-1438761681033-6461ffad8d80?w=150', now() - interval '65 days', now()),
(6, gen_random_uuid(), 'Le Hoang Cuong', 'user3@goshop.com', v_pwd, 32, 1, 3, 'https://images.unsplash.com/photo-1472099645785-5658abf4ff4e?w=150', now() - interval '60 days', now()),
(7, gen_random_uuid(), 'Pham Minh Duc', 'user4@goshop.com', v_pwd, 29, 1, 3, 'https://images.unsplash.com/photo-1519085360753-af0119f7cbe7?w=150', now() - interval '55 days', now()),
(8, gen_random_uuid(), 'Hoang Thu Giang', 'user5@goshop.com', v_pwd, 22, 1, 3, 'https://images.unsplash.com/photo-1544005313-94ddf0286df2?w=150', now() - interval '50 days', now()),
(9, gen_random_uuid(), 'Vu Dinh Hai', 'user6@goshop.com', v_pwd, 35, 1, 3, 'https://images.unsplash.com/photo-1506794778202-cad84cf45f1d?w=150', now() - interval '45 days', now()),
(10, gen_random_uuid(), 'Do Thi Huong', 'user7@goshop.com', v_pwd, 27, 1, 3, 'https://images.unsplash.com/photo-1517841905240-472988babdf9?w=150', now() - interval '40 days', now());

-- Generate 40 additional realistic customers (IDs 11..50)
INSERT INTO users (user_id, user_uuid, user_fullname, user_email, user_password, user_age, user_status, user_level, user_avatar, user_created_at, user_updated_at)
OVERRIDING SYSTEM VALUE
SELECT 
    i,
    gen_random_uuid(),
    (ARRAY['Nguyen', 'Tran', 'Le', 'Pham', 'Hoang', 'Phan', 'Vu', 'Dang', 'Bui', 'Do'])[1 + (i % 10)] || ' ' ||
    (ARRAY['Van', 'Thi', 'Duc', 'Quoc', 'Thanh', 'Ngoc', 'Minh', 'Huu', 'Gia', 'Hai'])[1 + ((i * 3) % 10)] || ' ' ||
    (ARRAY['Anh', 'Binh', 'Chau', 'Dung', 'Em', 'Giang', 'Huy', 'Khoa', 'Linh', 'Mai', 'Nam', 'Phong', 'Quan', 'Son', 'Tuan', 'Vy'])[1 + ((i * 7) % 16)],
    'customer' || i || '@testmail.com',
    v_pwd,
    20 + (i % 35),
    1,
    3,
    'https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=150',
    now() - (i || ' days')::interval,
    now()
FROM generate_series(11, 50) AS i;

PERFORM setval('users_user_id_seq', (SELECT MAX(user_id) FROM users), true);

-- ==============================================================================
-- 2. CATEGORIES
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

PERFORM setval('categories_category_id_seq', (SELECT MAX(category_id) FROM categories), true);

-- ==============================================================================
-- 3. PRODUCTS (32 realistic products with high quality specs & images)
-- ==============================================================================
INSERT INTO products (product_id, product_uuid, product_name, product_slug, product_description, product_price, product_stock, product_sku, product_category_id, product_image_url, product_status, product_avg_rating, product_review_count, product_created_at, product_updated_at)
OVERRIDING SYSTEM VALUE VALUES
-- Category 1: Smartphones
(1, gen_random_uuid(), 'iPhone 16 Pro Max 256GB Titanium', 'iphone-16-pro-max-256gb', 'Apple A18 Pro chip, 6.9-inch Super Retina XDR display, Grade 5 titanium design, 48MP Fusion camera system with 5x telephoto.', 34990000.00, 150, 'IPH16PM-256-TI', 1, 'https://images.unsplash.com/photo-1695048133142-1a20484d2569?w=800', 1, 4.9, 12, now() - interval '60 days', now()),
(2, gen_random_uuid(), 'Samsung Galaxy S24 Ultra 512GB Titanium Gray', 'samsung-galaxy-s24-ultra-512gb', 'Snapdragon 8 Gen 3 for Galaxy, Galaxy AI built-in, 200MP camera, Quad Tele System, Titanium frame with embedded S Pen.', 31990000.00, 120, 'SGS24U-512-TG', 1, 'https://images.unsplash.com/photo-1610945265064-0e34e5519bbf?w=800', 1, 4.8, 10, now() - interval '55 days', now()),
(3, gen_random_uuid(), 'Xiaomi 14 Ultra 512GB Leica Edition', 'xiaomi-14-ultra-512gb-leica', 'Snapdragon 8 Gen 3, All Around Liquid Display, Leica quad-camera setup with 1-inch sensor, 90W HyperCharge wired + 80W wireless.', 27990000.00, 85, 'XM14U-512-BLK', 1, 'https://images.unsplash.com/photo-1598327105666-5b89351aff97?w=800', 1, 4.7, 8, now() - interval '50 days', now()),
(4, gen_random_uuid(), 'Google Pixel 9 Pro XL 256GB Obsidian', 'google-pixel-9-pro-xl-256gb', 'Google Tensor G4 with Gemini Nano, Super Actua display, triple pro camera system with 30x Super Res Zoom, 7 years of OS updates.', 28490000.00, 60, 'PIX9PXL-256-OB', 1, 'https://images.unsplash.com/photo-1592899677977-9c10ca588bbd?w=800', 1, 4.8, 6, now() - interval '45 days', now()),
(5, gen_random_uuid(), 'iPad Pro 13-inch M4 Ultra Retina OLED', 'ipad-pro-13-inch-m4-oled', 'Breakthrough Apple M4 chip, Ultra Retina XDR with Tandem OLED technology, 5.1mm ultra-thin enclosure, Apple Pencil Pro support.', 37990000.00, 45, 'IPP13-M4-256-SLV', 1, 'https://images.unsplash.com/photo-1544244015-0df4b3ffc6b0?w=800', 1, 4.9, 5, now() - interval '40 days', now()),
(6, gen_random_uuid(), 'Samsung Galaxy Tab S9 Ultra 256GB 5G', 'samsung-galaxy-tab-s9-ultra-5g', '14.6-inch Dynamic AMOLED 2X 120Hz display, IP68 water & dust resistance, Snapdragon 8 Gen 2, included S Pen with ultra-low latency.', 25990000.00, 30, 'SGT-S9U-256-GR', 1, 'https://images.unsplash.com/photo-1561154464-82e9adf32764?w=800', 1, 4.6, 4, now() - interval '35 days', now()),

-- Category 2: Laptops & Computers
(7, gen_random_uuid(), 'MacBook Pro 16-inch M3 Max 36GB / 1TB Space Black', 'macbook-pro-16-m3-max-36gb', '16-core CPU, 40-core GPU, Liquid Retina XDR display with ProMotion 120Hz, up to 22 hours battery life, hardware ray tracing.', 89990000.00, 25, 'MBP16-M3MAX-1TB', 2, 'https://images.unsplash.com/photo-1517336714731-489689fd1ca8?w=800', 1, 5.0, 7, now() - interval '50 days', now()),
(8, gen_random_uuid(), 'Dell XPS 16 OLED 9640 Core Ultra 9 / RTX 4070', 'dell-xps-16-oled-9640-core-ultra-9', 'Intel Core Ultra 9 185H, 32GB LPDDR5x, 1TB SSD, 16.3-inch 4K+ OLED Touch display, CNC machined aluminum chassis.', 68500000.00, 20, 'DELL-XPS16-9640', 2, 'https://images.unsplash.com/photo-1593642632823-8f785ba67e45?w=800', 1, 4.7, 5, now() - interval '45 days', now()),
(9, gen_random_uuid(), 'ASUS ROG Zephyrus G16 OLED Ryzen AI 9 / RTX 4080', 'asus-rog-zephyrus-g16-oled-2024', 'AMD Ryzen AI 9 HX 370, 32GB LPDDR5X, 2TB PCIe 4.0 NVMe, 2.5K 240Hz ROG Nebula OLED, Slash Lighting CNC unibody.', 72990000.00, 15, 'ROG-G16-GA605', 2, 'https://images.unsplash.com/photo-1603302576837-37561b2e2302?w=800', 1, 4.8, 9, now() - interval '40 days', now()),
(10, gen_random_uuid(), 'MacBook Air 15-inch M3 16GB / 512GB Midnight', 'macbook-air-15-m3-16gb-512gb', 'Incredibly thin, fanless silent operation, Liquid Retina display, MagSafe 3 charging, dual USB-C Thunderbolt 4 ports.', 36990000.00, 70, 'MBA15-M3-16-512', 2, 'https://images.unsplash.com/photo-1611186871348-b1ce696e52c9?w=800', 1, 4.9, 11, now() - interval '38 days', now()),
(11, gen_random_uuid(), 'Lenovo ThinkPad X1 Carbon Gen 12 Core Ultra 7', 'lenovo-thinkpad-x1-carbon-gen12', 'Intel Core Ultra 7 155H, 32GB RAM, 1TB SSD, 2.8K OLED Display, legendary ergonomic TrackPoint keyboard, MIL-STD 810H certified.', 54900000.00, 35, 'TP-X1C-G12-U7', 2, 'https://images.unsplash.com/photo-1588872657578-7efd1f1555ed?w=800', 1, 4.7, 4, now() - interval '32 days', now()),

-- Category 3: Audio & Sound
(12, gen_random_uuid(), 'Sony WH-1000XM5 Wireless Noise Cancelling Headphones', 'sony-wh-1000xm5-wireless-black', 'Industry-leading noise cancellation with 8 mics and Auto NC Optimizer, LDAC High-Res audio, 30 hours battery life with quick charging.', 7990000.00, 90, 'SONY-WH1000XM5-B', 3, 'https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=800', 1, 4.8, 15, now() - interval '60 days', now()),
(13, gen_random_uuid(), 'Apple AirPods Pro 2 USB-C with MagSafe Case', 'apple-airpods-pro-2-usb-c', 'Apple H2 chip, Pro-level Active Noise Cancellation, Adaptive Audio, Transparency mode, Personalized Spatial Audio with dynamic head tracking.', 5890000.00, 200, 'APP2-USBC-WHT', 3, 'https://images.unsplash.com/photo-1600294037681-c80b4cb5b434?w=800', 1, 4.9, 18, now() - interval '55 days', now()),
(14, gen_random_uuid(), 'Bose QuietComfort Ultra Headphones Spatial Audio', 'bose-quietcomfort-ultra-headphones', 'CustomTune technology, world-class noise cancelling, Bose Immersive Audio, luxurious ultra-soft protein leather ear cushions.', 9490000.00, 50, 'BOSE-QCU-BLK', 3, 'https://images.unsplash.com/photo-1546435770-a3e426bf472b?w=800', 1, 4.7, 8, now() - interval '42 days', now()),
(15, gen_random_uuid(), 'Marshall Stanmore III Bluetooth Home Speaker', 'marshall-stanmore-iii-bluetooth-black', 'Iconic vintage Marshall design, room-filling stereo soundstage, Dynamic Loudness, Bluetooth 5.2, 3.5mm AUX & RCA inputs.', 8990000.00, 40, 'MARSH-STAN3-BLK', 3, 'https://images.unsplash.com/photo-1545454675-3531b543be5d?w=800', 1, 4.9, 10, now() - interval '40 days', now()),
(16, gen_random_uuid(), 'Sennheiser Momentum True Wireless 4 Earbuds', 'sennheiser-momentum-tw-4-copper', 'TrueResponse transducer system, Lossless audio with aptX Adaptive, Auracast support, Adaptive ANC with smart anti-wind mode.', 7490000.00, 45, 'SENN-MTW4-COP', 3, 'https://images.unsplash.com/photo-1590658268037-6bf12165a8df?w=800', 1, 4.6, 6, now() - interval '30 days', now()),

-- Category 4: Smartwatches & Wearables
(17, gen_random_uuid(), 'Apple Watch Ultra 2 GPS + Cellular 49mm Titanium', 'apple-watch-ultra-2-49mm-titanium', 'S9 SiP chip, 3000 nits brightness, dual-frequency precision GPS, 36 hours normal battery (up to 72 hrs Low Power), 100m water resistant.', 21490000.00, 55, 'AWU2-49-TI-OR', 4, 'https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=800', 1, 4.9, 13, now() - interval '50 days', now()),
(18, gen_random_uuid(), 'Garmin Fenix 8 Solar Sapphire AMOLED 47mm', 'garmin-fenix-8-solar-sapphire-47mm', 'Solar charging lens, built-in mic & speaker for calls, top-tier topographic maps, multisport endurance metrics, LED flashlight.', 26990000.00, 30, 'GARMIN-FNX8-47', 4, 'https://images.unsplash.com/photo-1508685096489-7aacd43bd3b1?w=800', 1, 5.0, 9, now() - interval '45 days', now()),
(19, gen_random_uuid(), 'Samsung Galaxy Watch Ultra 47mm Titanium Silver', 'samsung-galaxy-watch-ultra-47mm', 'Exynos W1000 3nm processor, Sapphire crystal, BioActive Sensor, dual-frequency GPS, Emergency Siren, 100-hour battery life.', 16490000.00, 65, 'SGW-U-47-SLV', 4, 'https://images.unsplash.com/photo-1579586337278-3befd40fd17a?w=800', 1, 4.6, 7, now() - interval '38 days', now()),

-- Category 5: Gaming & Consoles
(20, gen_random_uuid(), 'PlayStation 5 Pro Console 2TB Digital Edition', 'playstation-5-pro-console-2tb', 'PlayStation Spectral Super Resolution (PSSR), advanced ray tracing, 60FPS high-fidelity gaming, custom 2TB ultra-fast SSD.', 19990000.00, 40, 'PS5-PRO-2TB', 5, 'https://images.unsplash.com/photo-1606813907291-d86efa9b94db?w=800', 1, 4.9, 16, now() - interval '35 days', now()),
(21, gen_random_uuid(), 'Nintendo Switch OLED Model Mario Red Edition', 'nintendo-switch-oled-mario-red', '7-inch vibrant OLED screen, wide adjustable stand, wired LAN dock, 64GB internal storage, enhanced audio.', 8490000.00, 80, 'NSW-OLED-MRED', 5, 'https://images.unsplash.com/photo-1578303512597-81e6cc155b3e?w=800', 1, 4.8, 12, now() - interval '48 days', now()),
(22, gen_random_uuid(), 'Logitech G PRO X SUPERLIGHT 2 Wireless Gaming Mouse', 'logitech-g-pro-x-superlight-2-magenta', 'LIGHTFORCE hybrid optical-mechanical switches, HERO 2 sensor with 32,000 DPI and 4,000Hz polling rate, ultra-lightweight 60 grams.', 3690000.00, 110, 'LOGI-GPX2-MAG', 5, 'https://images.unsplash.com/photo-1615663245857-ac93bb7c39e7?w=800', 1, 4.8, 14, now() - interval '52 days', now()),
(23, gen_random_uuid(), 'Keychron Q1 Pro Wireless Custom Mechanical Keyboard', 'keychron-q1-pro-wireless-qmk', 'Full CNC aluminum body, double-gasket design, hot-swappable Keychron K Pro Banana switches, QMK/VIA programmable, Mac/Win support.', 4590000.00, 75, 'KC-Q1PRO-BAN', 5, 'https://images.unsplash.com/photo-1587829741301-dc798b83add3?w=800', 1, 4.9, 10, now() - interval '44 days', now()),

-- Category 6: Cameras
(24, gen_random_uuid(), 'Sony Alpha A7 IV Mirrorless Camera Body', 'sony-alpha-a7-iv-body', '33MP full-frame Exmor R CMOS sensor, 4K 60p 10-bit video, real-time Eye AF for humans/animals/birds, 7K oversampled recording.', 53990000.00, 18, 'SONY-ILCE-7M4', 6, 'https://images.unsplash.com/photo-1516035069371-29a1b244cc32?w=800', 1, 4.9, 5, now() - interval '46 days', now()),
(25, gen_random_uuid(), 'Fujifilm X100VI Digital Camera Silver Edition', 'fujifilm-x100vi-silver', '40.2MP X-Trans CMOS 5 HR sensor, 6.0 stops in-body image stabilization (IBIS), 20 Film Simulation modes including REALA ACE.', 44990000.00, 10, 'FUJI-X100VI-SLV', 6, 'https://images.unsplash.com/photo-1502920917128-1aa500764cbd?w=800', 1, 5.0, 8, now() - interval '30 days', now()),
(26, gen_random_uuid(), 'DJI Pocket 3 Creator Combo 4K 120fps Vlog Camera', 'dji-pocket-3-creator-combo', '1-inch CMOS sensor, 2-inch rotatable OLED touchscreen, 3-axis mechanical gimbal, D-Log M 10-bit color, included DJI Mic 2 transmitter.', 15890000.00, 45, 'DJI-OP3-COMBO', 6, 'https://images.unsplash.com/photo-1526170375885-4d8ecf77b99f?w=800', 1, 4.9, 12, now() - interval '36 days', now()),

-- Category 7: Smart Home & Appliances
(27, gen_random_uuid(), 'Roborock S8 Pro Ultra Robot Vacuum and Mop', 'roborock-s8-pro-ultra-robot-vacuum', 'RockDock Ultra all-in-one station with auto-washing, auto-drying, auto-emptying and auto-refilling. 6000Pa HyperForce suction.', 26990000.00, 25, 'ROBO-S8PU-BLK', 7, 'https://images.unsplash.com/photo-1558317374-067fb5f30001?w=800', 1, 4.8, 7, now() - interval '40 days', now()),
(28, gen_random_uuid(), 'Dyson Purifier Hot+Cool Formaldehyde HP09 Air Purifier', 'dyson-purifier-hot-cool-hp09', 'Destroys formaldehyde permanently, HEPA H13 fully sealed filtration, cools in summer and heats in winter, Air Multiplier tech.', 19990000.00, 30, 'DYSON-HP09-WHT', 7, 'https://images.unsplash.com/photo-1585771724684-38269d6639fd?w=800', 1, 4.7, 6, now() - interval '34 days', now()),
(29, gen_random_uuid(), 'Philips Hue Smart Bridge & White and Color Starter Kit', 'philips-hue-starter-kit-e27', '16 million colors and 50,000 shades of warm-to-cool white light. Includes 3 smart bulbs, Hue Bridge, and smart dimmer switch.', 4890000.00, 60, 'PHILIPS-HUE-KIT', 7, 'https://images.unsplash.com/photo-1550751827-4bd374c3f58b?w=800', 1, 4.8, 9, now() - interval '42 days', now()),

-- Category 8: Accessories & Cables
(30, gen_random_uuid(), 'Anker Prime 27,650mAh Power Bank (250W 3-Port)', 'anker-prime-250w-power-bank-27650mah', 'Massive 250W multi-device fast charging output, smart digital display with live wattage, Anker app Bluetooth pairing.', 3490000.00, 140, 'ANKER-PRIME-250W', 8, 'https://images.unsplash.com/photo-1609091839311-d5365f9ff1c5?w=800', 1, 4.9, 20, now() - interval '55 days', now()),
(31, gen_random_uuid(), 'Belkin BoostCharge Pro 3-in-1 MagSafe Wireless Charger', 'belkin-boostcharge-pro-3-in-1-magsafe', 'Official 15W MagSafe fast charging for iPhone, fast charge module for Apple Watch Ultra, dedicated Qi pad for AirPods.', 3890000.00, 85, 'BELKIN-WIZ017-BLK', 8, 'https://images.unsplash.com/photo-1583863788434-e58a36330cf0?w=800', 1, 4.7, 11, now() - interval '48 days', now()),
(32, gen_random_uuid(), 'UGREEN Nexode 300W GaN 5-Port Desktop Charger', 'ugreen-nexode-300w-gan-5port', 'World first 300W 5-port GaN fast charger with PD 3.1 140W single port power, thermal guard safety protection system.', 2790000.00, 100, 'UGREEN-CD333-300W', 8, 'https://images.unsplash.com/photo-1544717305-2782549b5136?w=800', 1, 4.8, 14, now() - interval '44 days', now());

PERFORM setval('products_product_id_seq', (SELECT MAX(product_id) FROM products), true);

-- ==============================================================================
-- 4. USER ADDRESSES (Realistic Vietnamese addresses for customers)
-- ==============================================================================
INSERT INTO user_addresses (ua_id, ua_uuid, ua_user_id, ua_label, ua_name, ua_phone, ua_address, ua_is_default, ua_province_id, ua_province_name, ua_district_id, ua_district_name, ua_ward_code, ua_ward_name, ua_created_at, ua_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 4, 'Home', 'Nguyen Van An', '0912345678', 'Số 18, Ngõ 45, Đường Trần Duy Hưng, Cầu Giấy, Hà Nội', TRUE, 201, 'Hà Nội', 1488, 'Quận Cầu Giấy', '1A0101', 'Phường Trung Hòa', now() - interval '60 days', now()),
(2, gen_random_uuid(), 4, 'Office', 'Nguyen Van An', '0912345678', 'Tầng 12, Tòa nhà Landmark 72, Đường Phạm Hùng, Nam Từ Liêm, Hà Nội', FALSE, 201, 'Hà Nội', 1490, 'Quận Nam Từ Liêm', '1A0202', 'Phường Mễ Trì', now() - interval '40 days', now()),
(3, gen_random_uuid(), 5, 'Home', 'Tran Thi Binh', '0987654321', '245/12 Lê Văn Sỹ, Phường 14, Quận 3, TP. Hồ Chí Minh', TRUE, 202, 'TP. Hồ Chí Minh', 1442, 'Quận 3', '2B0301', 'Phường 14', now() - interval '58 days', now()),
(4, gen_random_uuid(), 6, 'Home', 'Le Hoang Cuong', '0903112233', '102 Nguyễn Văn Linh, Phường Nam Dương, Quận Hải Châu, Đà Nẵng', TRUE, 203, 'Đà Nẵng', 1520, 'Quận Hải Châu', '3C0101', 'Phường Nam Dương', now() - interval '55 days', now()),
(5, gen_random_uuid(), 7, 'Home', 'Pham Minh Duc', '0934556677', '56 Đường 30 Tháng 4, Phường An Phú, Quận Ninh Kiều, Cần Thơ', TRUE, 204, 'Cần Thơ', 1550, 'Quận Ninh Kiều', '4D0101', 'Phường An Phú', now() - interval '50 days', now()),
(6, gen_random_uuid(), 8, 'Home', 'Hoang Thu Giang', '0978990011', '88 Lạch Tray, Quận Ngô Quyền, Hải Phòng', TRUE, 205, 'Hải Phòng', 1580, 'Quận Ngô Quyền', '5E0101', 'Phường Lạch Tray', now() - interval '45 days', now());

-- Generate addresses for users 9 to 50 (IDs 7..48)
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
WHERE u.user_id BETWEEN 9 AND 50;

PERFORM setval('user_addresses_ua_id_seq', (SELECT MAX(ua_id) FROM user_addresses), true);

-- ==============================================================================
-- 5. COUPONS
-- ==============================================================================
INSERT INTO coupons (coupon_id, coupon_uuid, coupon_code, coupon_name, coupon_description, coupon_type, coupon_discount_value, coupon_max_discount_amount, coupon_min_order_amount, coupon_usage_limit, coupon_used_count, coupon_user_usage_limit, coupon_start_date, coupon_end_date, coupon_is_active, coupon_created_at, coupon_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 'WELCOME10', 'Welcome New Shopper 10%', 'Enjoy 10% discount on your first tech gadget purchase.', 1, 10.00, 500000.00, 1000000.00, 1000, 24, 1, now() - interval '60 days', now() + interval '180 days', TRUE, now() - interval '60 days', now()),
(2, gen_random_uuid(), 'GOSHOP500K', 'Super Mega 500K Off', 'Direct 500,000 VND discount for orders over 15,000,000 VND.', 2, 500000.00, 500000.00, 15000000.00, 500, 48, 2, now() - interval '45 days', now() + interval '120 days', TRUE, now() - interval '45 days', now()),
(3, gen_random_uuid(), 'TECHVIP15', 'VIP Tech Enthusiast 15%', 'Exclusive 15% discount capped at 2,000,000 VND.', 1, 15.00, 2000000.00, 5000000.00, 200, 15, 1, now() - interval '30 days', now() + interval '90 days', TRUE, now() - interval '30 days', now()),
(4, gen_random_uuid(), 'FLASH100K', 'Flash Sale 100K Voucher', 'Direct 100,000 VND discount on accessories.', 2, 100000.00, 100000.00, 1000000.00, 2000, 85, 3, now() - interval '15 days', now() + interval '45 days', TRUE, now() - interval '15 days', now()),
(5, gen_random_uuid(), 'BLACKFRIDAY', 'Black Friday Ultra 20%', 'Black Friday 20% discount on all flagship laptops.', 1, 20.00, 3000000.00, 10000000.00, 100, 100, 1, now() - interval '100 days', now() - interval '70 days', FALSE, now() - interval '105 days', now());

PERFORM setval('coupons_coupon_id_seq', (SELECT MAX(coupon_id) FROM coupons), true);

-- ==============================================================================
-- 6. ORDERS & ORDER ITEMS & PAYMENTS & TRACKING
-- ==============================================================================
-- Order 1: Delivered (iPhone 16 Pro Max + AirPods Pro 2)
INSERT INTO orders (order_id, order_uuid, order_number, order_user_id, order_shipping_name, order_shipping_phone, order_shipping_address, order_subtotal_amount, order_discount_amount, order_total_amount, order_refunded_amount, order_coupon_id, order_coupon_code, order_payment_method, order_payment_status, order_status, order_shipping_provider, order_shipping_code, order_shipping_fee, order_expected_delivery, order_delivered_at, order_created_at, order_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 'ORD-20241001-001', 4, 'Nguyen Van An', '0912345678', 'Số 18, Ngõ 45, Đường Trần Duy Hưng, Cầu Giấy, Hà Nội', 40880000.00, 500000.00, 40380000.00, 0.00, 1, 'WELCOME10', 2, 2, 4, 'ghn', 'GHN982314112VN', 0.00, now() - interval '25 days', now() - interval '24 days', now() - interval '28 days', now() - interval '24 days');

INSERT INTO order_items (order_item_id, order_item_uuid, order_item_order_id, order_item_product_id, order_item_product_name, order_item_price, order_item_quantity, order_item_subtotal, order_item_created_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 1, 1, 'iPhone 16 Pro Max 256GB Titanium', 34990000.00, 1, 34990000.00, now() - interval '28 days'),
(2, gen_random_uuid(), 1, 13, 'Apple AirPods Pro 2 USB-C with MagSafe Case', 5890000.00, 1, 5890000.00, now() - interval '28 days');

INSERT INTO payment_transactions (pt_id, pt_uuid, pt_order_id, pt_idempotency_key, pt_provider, pt_txn_ref, pt_provider_txn_id, pt_amount, pt_currency, pt_status, pt_paid_at, pt_created_at, pt_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 1, 'IDEM-PAY-001-VNPAY', 'vnpay', 'TXN-VNPAY-20241001-001', 'VNP148829910', 40380000.00, 'VND', 2, now() - interval '28 days', now() - interval '28 days', now() - interval '28 days');

INSERT INTO coupon_usages (coupon_usage_coupon_id, coupon_usage_user_id, coupon_usage_order_id, coupon_usage_used_at)
VALUES (1, 4, 1, now() - interval '28 days');

INSERT INTO order_tracking_events (event_order_id, event_status, event_note, event_source, event_created_at)
VALUES
(1, 'pending', 'Order placed successfully by customer', 'system', now() - interval '28 days'),
(1, 'confirmed', 'Order confirmed by store warehouse', 'admin', now() - interval '27 days' + interval '2 hours'),
(1, 'shipping', 'Package handed over to GHN courier (Code: GHN982314112VN)', 'ghn', now() - interval '26 days'),
(1, 'delivered', 'Order successfully delivered to customer', 'ghn', now() - interval '24 days');

-- Order 2: Delivered (MacBook Pro 16 M3 Max + Anker Power Bank)
INSERT INTO orders (order_id, order_uuid, order_number, order_user_id, order_shipping_name, order_shipping_phone, order_shipping_address, order_subtotal_amount, order_discount_amount, order_total_amount, order_refunded_amount, order_coupon_id, order_coupon_code, order_payment_method, order_payment_status, order_status, order_shipping_provider, order_shipping_code, order_shipping_fee, order_expected_delivery, order_delivered_at, order_created_at, order_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(2, gen_random_uuid(), 'ORD-20241002-002', 5, 'Tran Thi Binh', '0987654321', '245/12 Lê Văn Sỹ, Phường 14, Quận 3, TP. Hồ Chí Minh', 93480000.00, 500000.00, 92980000.00, 0.00, 2, 'GOSHOP500K', 2, 2, 4, 'ghn', 'GHN982314889VN', 0.00, now() - interval '20 days', now() - interval '19 days', now() - interval '22 days', now() - interval '19 days');

INSERT INTO order_items (order_item_id, order_item_uuid, order_item_order_id, order_item_product_id, order_item_product_name, order_item_price, order_item_quantity, order_item_subtotal, order_item_created_at)
OVERRIDING SYSTEM VALUE VALUES
(3, gen_random_uuid(), 2, 7, 'MacBook Pro 16-inch M3 Max 36GB / 1TB Space Black', 89990000.00, 1, 89990000.00, now() - interval '22 days'),
(4, gen_random_uuid(), 2, 30, 'Anker Prime 27,650mAh Power Bank (250W 3-Port)', 3490000.00, 1, 3490000.00, now() - interval '22 days');

INSERT INTO payment_transactions (pt_id, pt_uuid, pt_order_id, pt_idempotency_key, pt_provider, pt_txn_ref, pt_provider_txn_id, pt_amount, pt_currency, pt_status, pt_paid_at, pt_created_at, pt_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(2, gen_random_uuid(), 2, 'IDEM-PAY-002-MOMO', 'momo', 'TXN-MOMO-20241002-002', 'MOMO99182312', 92980000.00, 'VND', 2, now() - interval '22 days', now() - interval '22 days', now() - interval '22 days');

INSERT INTO coupon_usages (coupon_usage_coupon_id, coupon_usage_user_id, coupon_usage_order_id, coupon_usage_used_at)
VALUES (2, 5, 2, now() - interval '22 days');

-- Order 3: Shipped (Sony WH-1000XM5 + Belkin Charger)
INSERT INTO orders (order_id, order_uuid, order_number, order_user_id, order_shipping_name, order_shipping_phone, order_shipping_address, order_subtotal_amount, order_discount_amount, order_total_amount, order_refunded_amount, order_payment_method, order_payment_status, order_status, order_shipping_provider, order_shipping_code, order_shipping_fee, order_expected_delivery, order_created_at, order_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(3, gen_random_uuid(), 'ORD-20241003-003', 6, 'Le Hoang Cuong', '0903112233', '102 Nguyễn Văn Linh, Phường Nam Dương, Quận Hải Châu, Đà Nẵng', 11880000.00, 0.00, 11880000.00, 0.00, 1, 1, 3, 'ghn', 'GHN771829100VN', 30000.00, now() + interval '2 days', now() - interval '2 days', now());

INSERT INTO order_items (order_item_id, order_item_uuid, order_item_order_id, order_item_product_id, order_item_product_name, order_item_price, order_item_quantity, order_item_subtotal, order_item_created_at)
OVERRIDING SYSTEM VALUE VALUES
(5, gen_random_uuid(), 3, 12, 'Sony WH-1000XM5 Wireless Noise Cancelling Headphones', 7990000.00, 1, 7990000.00, now() - interval '2 days'),
(6, gen_random_uuid(), 3, 31, 'Belkin BoostCharge Pro 3-in-1 MagSafe Wireless Charger', 3890000.00, 1, 3890000.00, now() - interval '2 days');

INSERT INTO payment_transactions (pt_id, pt_uuid, pt_order_id, pt_idempotency_key, pt_provider, pt_amount, pt_currency, pt_status, pt_created_at, pt_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(3, gen_random_uuid(), 3, 'IDEM-PAY-003-COD', 'cod', 11880000.00, 'VND', 1, now() - interval '2 days', now() - interval '2 days');

-- Order 4: Confirmed (PlayStation 5 Pro + Keychron Q1 Pro)
INSERT INTO orders (order_id, order_uuid, order_number, order_user_id, order_shipping_name, order_shipping_phone, order_shipping_address, order_subtotal_amount, order_discount_amount, order_total_amount, order_refunded_amount, order_coupon_id, order_coupon_code, order_payment_method, order_payment_status, order_status, order_created_at, order_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(4, gen_random_uuid(), 'ORD-20241004-004', 7, 'Pham Minh Duc', '0934556677', '56 Đường 30 Tháng 4, Phường An Phú, Quận Ninh Kiều, Cần Thơ', 24580000.00, 500000.00, 24080000.00, 0.00, 2, 'GOSHOP500K', 2, 2, 2, now() - interval '1 day', now());

INSERT INTO order_items (order_item_id, order_item_uuid, order_item_order_id, order_item_product_id, order_item_product_name, order_item_price, order_item_quantity, order_item_subtotal, order_item_created_at)
OVERRIDING SYSTEM VALUE VALUES
(7, gen_random_uuid(), 4, 20, 'PlayStation 5 Pro Console 2TB Digital Edition', 19990000.00, 1, 19990000.00, now() - interval '1 day'),
(8, gen_random_uuid(), 4, 23, 'Keychron Q1 Pro Wireless Custom Mechanical Keyboard', 4590000.00, 1, 4590000.00, now() - interval '1 day');

INSERT INTO payment_transactions (pt_id, pt_uuid, pt_order_id, pt_idempotency_key, pt_provider, pt_txn_ref, pt_provider_txn_id, pt_amount, pt_currency, pt_status, pt_paid_at, pt_created_at, pt_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(4, gen_random_uuid(), 4, 'IDEM-PAY-004-VNPAY', 'vnpay', 'TXN-VNPAY-20241004-004', 'VNP887129033', 24080000.00, 'VND', 2, now() - interval '1 day', now() - interval '1 day', now() - interval '1 day');

-- Order 5: Pending
INSERT INTO orders (order_id, order_uuid, order_number, order_user_id, order_shipping_name, order_shipping_phone, order_shipping_address, order_subtotal_amount, order_discount_amount, order_total_amount, order_refunded_amount, order_payment_method, order_payment_status, order_status, order_created_at, order_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(5, gen_random_uuid(), 'ORD-20241005-005', 8, 'Hoang Thu Giang', '0978990011', '88 Lạch Tray, Quận Ngô Quyền, Hải Phòng', 16490000.00, 0.00, 16490000.00, 0.00, 2, 1, 1, now() - interval '3 hours', now());

INSERT INTO order_items (order_item_id, order_item_uuid, order_item_order_id, order_item_product_id, order_item_product_name, order_item_price, order_item_quantity, order_item_subtotal, order_item_created_at)
OVERRIDING SYSTEM VALUE VALUES
(9, gen_random_uuid(), 5, 19, 'Samsung Galaxy Watch Ultra 47mm Titanium Silver', 16490000.00, 1, 16490000.00, now() - interval '3 hours');

-- Generate 45 additional realistic orders across customers (IDs 6..50)
INSERT INTO orders (order_id, order_uuid, order_number, order_user_id, order_shipping_name, order_shipping_phone, order_shipping_address, order_subtotal_amount, order_discount_amount, order_total_amount, order_refunded_amount, order_payment_method, order_payment_status, order_status, order_shipping_provider, order_shipping_code, order_created_at, order_updated_at)
OVERRIDING SYSTEM VALUE
SELECT
    i,
    gen_random_uuid(),
    'ORD-2024' || lpad((100 + i)::text, 5, '0'),
    u.user_id,
    u.user_fullname,
    '09' || lpad((20000000 + i * 333)::text, 8, '0'),
    'Số ' || (i * 2) || ' Đường Nguyễn Trãi, Quận Thanh Xuân, Hà Nội',
    p.product_price * ((i % 2) + 1),
    0.00,
    p.product_price * ((i % 2) + 1),
    0.00,
    1 + (i % 3),
    CASE WHEN (i % 5) = 0 THEN 1 ELSE 2 END,
    CASE 
        WHEN (i % 6) = 0 THEN 1 -- Pending
        WHEN (i % 6) = 1 THEN 2 -- Confirmed
        WHEN (i % 6) = 2 THEN 3 -- Shipped
        WHEN (i % 6) = 3 THEN 4 -- Delivered
        WHEN (i % 6) = 4 THEN 5 -- Cancelled
        ELSE 4
    END,
    'ghn',
    'GHN' || (800000000 + i * 147)::text || 'VN',
    now() - (i || ' days')::interval,
    now() - (i || ' days')::interval
FROM generate_series(6, 50) AS i
JOIN users u ON u.user_id = (4 + (i % 40))
JOIN products p ON p.product_id = (1 + (i % 32));

-- Populate order_items for generated orders (IDs 10..54)
INSERT INTO order_items (order_item_id, order_item_uuid, order_item_order_id, order_item_product_id, order_item_product_name, order_item_price, order_item_quantity, order_item_subtotal, order_item_created_at)
OVERRIDING SYSTEM VALUE
SELECT 
    (4 + o.order_id),
    gen_random_uuid(),
    o.order_id,
    p.product_id,
    p.product_name,
    p.product_price,
    1,
    p.product_price,
    o.order_created_at
FROM orders o
JOIN products p ON p.product_id = (1 + (o.order_id % 32))
WHERE o.order_id >= 6;

PERFORM setval('orders_order_id_seq', (SELECT MAX(order_id) FROM orders), true);
PERFORM setval('order_items_order_item_id_seq', (SELECT MAX(order_item_id) FROM order_items), true);
PERFORM setval('payment_transactions_pt_id_seq', (SELECT MAX(pt_id) FROM payment_transactions), true);

-- ==============================================================================
-- 7. PRODUCT REVIEWS & HELPFUL VOTES
-- ==============================================================================
INSERT INTO product_reviews (review_id, review_uuid, review_user_id, review_product_id, review_order_id, review_rating, review_title, review_comment, review_images, review_has_media, review_status, review_helpful_count, review_created_at, review_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 4, 1, 1, 5, 'Best smartphone of 2024!', 'The Titanium finish feels amazingly lightweight in hand. Camera zoom and video quality in ProRes are unreal.', ARRAY['https://images.unsplash.com/photo-1695048133142-1a20484d2569?w=500'], TRUE, 1, 18, now() - interval '20 days', now()),
(2, gen_random_uuid(), 4, 13, 1, 5, 'Superior ANC and sound profile', 'Active noise cancellation completely blocks out street noise. Battery lasts all day with fast MagSafe charge.', ARRAY[]::text[], FALSE, 1, 9, now() - interval '20 days', now()),
(3, gen_random_uuid(), 5, 7, 2, 5, 'MacBook Pro M3 Max is a powerhouse', 'Render 4K videos in seconds without hearing fan noise once. The Liquid Retina XDR screen is stunning.', ARRAY['https://images.unsplash.com/photo-1517336714731-489689fd1ca8?w=500'], TRUE, 1, 24, now() - interval '18 days', now()),
(4, gen_random_uuid(), 5, 30, 2, 5, 'Charges my laptop and phone simultaneously', 'The 250W output easily charges my MacBook Pro at full speed while powering my iPad. Very sturdy.', ARRAY[]::text[], FALSE, 1, 7, now() - interval '18 days', now());

-- Generate 30 additional reviews for products (IDs 5..34)
INSERT INTO product_reviews (review_id, review_uuid, review_user_id, review_product_id, review_order_id, review_rating, review_title, review_comment, review_has_media, review_status, review_helpful_count, review_created_at, review_updated_at)
OVERRIDING SYSTEM VALUE
SELECT
    (i - 1),
    gen_random_uuid(),
    o.order_user_id,
    oi.order_item_product_id,
    o.order_id,
    (4 + (i % 2))::smallint, -- 4 or 5 stars
    (ARRAY['Excellent build quality!', 'Worth every single penny', 'Super fast delivery and pristine packaging', 'Authentic product with full warranty', 'Exceeded my expectations'])[1 + (i % 5)],
    (ARRAY[
        'Highly recommended for anyone looking for reliable premium performance.',
        'The design and aesthetics match the photos perfectly. Very happy with this purchase.',
        'Customer support was very helpful when answering my questions prior to checkout.',
        'Battery life and build material feel top of the line.',
        'Works right out of the box without any issues whatsoever.'
    ])[1 + (i % 5)],
    FALSE,
    1,
    (i * 3) % 15,
    o.order_created_at + interval '1 day',
    o.order_created_at + interval '1 day'
FROM generate_series(6, 35) AS i
JOIN orders o ON o.order_id = i
JOIN order_items oi ON oi.order_item_order_id = o.order_id
ON CONFLICT DO NOTHING;

PERFORM setval('product_reviews_review_id_seq', (SELECT MAX(review_id) FROM product_reviews), true);

-- Update average rating and review counts on products
UPDATE products p
SET 
    product_avg_rating = COALESCE(sub.avg_r, 5.0),
    product_review_count = COALESCE(sub.cnt, 0)
FROM (
    SELECT review_product_id, ROUND(AVG(review_rating)::numeric, 1) AS avg_r, COUNT(*) AS cnt
    FROM product_reviews
    WHERE review_status = 1
    GROUP BY review_product_id
) sub
WHERE p.product_id = sub.review_product_id;

-- ==============================================================================
-- 8. CARTS & CART ITEMS
-- ==============================================================================
INSERT INTO carts (cart_id, cart_uuid, cart_user_id, cart_created_at, cart_updated_at)
OVERRIDING SYSTEM VALUE
SELECT 
    user_id,
    gen_random_uuid(),
    user_id,
    now() - interval '10 days',
    now()
FROM users
WHERE user_id BETWEEN 1 AND 50;

INSERT INTO cart_items (cart_item_id, cart_item_uuid, cart_item_cart_id, cart_item_product_id, cart_item_quantity, cart_item_created_at, cart_item_updated_at)
OVERRIDING SYSTEM VALUE
SELECT 
    (c.cart_id - 3),
    gen_random_uuid(),
    c.cart_id,
    p.product_id,
    1 + (c.cart_id % 3),
    now() - interval '1 day',
    now()
FROM carts c
JOIN products p ON p.product_id = (1 + (c.cart_id * 3) % 32)
WHERE c.cart_id BETWEEN 4 AND 20;

PERFORM setval('carts_cart_id_seq', (SELECT MAX(cart_id) FROM carts), true);
PERFORM setval('cart_items_cart_item_id_seq', (SELECT MAX(cart_item_id) FROM cart_items), true);

-- ==============================================================================
-- 9. STOCK MOVEMENTS (Auditing inventory changes)
-- ==============================================================================
INSERT INTO stock_movements (product_id, change_type, quantity_change, stock_before, stock_after, reference_id, reference_type, actor_id, note, created_at)
SELECT 
    p.product_id,
    'INITIAL_STOCK',
    p.product_stock + 50,
    0,
    p.product_stock + 50,
    'INIT-BATCH-2024',
    'product_edit',
    1,
    'Initial inventory intake from distributor warehouse',
    now() - interval '70 days'
FROM products p;

INSERT INTO stock_movements (product_id, change_type, quantity_change, stock_before, stock_after, reference_id, reference_type, actor_id, note, created_at)
SELECT 
    oi.order_item_product_id,
    'ORDER_PLACED',
    -oi.order_item_quantity,
    p.product_stock + oi.order_item_quantity,
    p.product_stock,
    o.order_number,
    'order',
    o.order_user_id,
    'Automated stock reduction on order placement',
    o.order_created_at
FROM order_items oi
JOIN orders o ON o.order_id = oi.order_item_order_id
JOIN products p ON p.product_id = oi.order_item_product_id;

-- ==============================================================================
-- 10. BANNERS (Marketing Hero and Promotions)
-- ==============================================================================
INSERT INTO banners (banner_id, banner_uuid, banner_title, banner_image_url, banner_link_url, banner_position, banner_order, banner_is_active, banner_start_at, banner_end_at, banner_created_at, banner_updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 'Apple Autumn Mega Launch Event 2024', 'https://images.unsplash.com/photo-1695048133142-1a20484d2569?w=1600', '/category/smartphones-tablets', 'home_main', 1, TRUE, now() - interval '30 days', now() + interval '90 days', now() - interval '30 days', now()),
(2, gen_random_uuid(), 'MacBook Pro M3 Max Powerhouse Collection', 'https://images.unsplash.com/photo-1517336714731-489689fd1ca8?w=1600', '/category/laptops-computers', 'home_main', 2, TRUE, now() - interval '30 days', now() + interval '90 days', now() - interval '30 days', now()),
(3, gen_random_uuid(), 'Audiophile Grade Wireless Audio Experience', 'https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=1600', '/category/audio-sound', 'home_main', 3, TRUE, now() - interval '30 days', now() + interval '90 days', now() - interval '30 days', now()),
(4, gen_random_uuid(), 'Exclusive 500K Off Voucher on Flagship Orders', 'https://images.unsplash.com/photo-1607082348824-0a96f2a4b9da?w=1200', '/promotions', 'home_sub', 1, TRUE, now() - interval '20 days', now() + interval '60 days', now() - interval '20 days', now());

PERFORM setval('banners_banner_id_seq', (SELECT MAX(banner_id) FROM banners), true);

-- ==============================================================================
-- 11. NOTIFICATIONS
-- ==============================================================================
INSERT INTO notifications (notification_uuid, notification_user_id, notification_type, notification_title, notification_body, notification_data, notification_is_read, notification_created_at)
VALUES
(gen_random_uuid(), 4, 'order_confirmed', 'Order #ORD-20241001-001 Confirmed', 'Your order containing iPhone 16 Pro Max has been verified and is being packaged.', '{"order_id": 1, "order_number": "ORD-20241001-001"}'::jsonb, TRUE, now() - interval '27 days'),
(gen_random_uuid(), 4, 'order_shipped', 'Order #ORD-20241001-001 is on its way', 'GHN courier has picked up your package. Tracking: GHN982314112VN.', '{"order_id": 1, "tracking_code": "GHN982314112VN"}'::jsonb, TRUE, now() - interval '26 days'),
(gen_random_uuid(), 4, 'order_delivered', 'Order #ORD-20241001-001 Delivered Successfully', 'Package delivered. Thank you for shopping with GOshop!', '{"order_id": 1}'::jsonb, FALSE, now() - interval '24 days'),
(gen_random_uuid(), 5, 'order_confirmed', 'Order #ORD-20241002-002 Confirmed', 'Your MacBook Pro order has been packed and prepared for shipping.', '{"order_id": 2}'::jsonb, TRUE, now() - interval '21 days'),
(gen_random_uuid(), 6, 'order_shipped', 'Order #ORD-20241003-003 is In Transit', 'GHN tracking code GHN771829100VN is active.', '{"order_id": 3}'::jsonb, FALSE, now() - interval '1 day');

-- ==============================================================================
-- 12. CHAT CONVERSATIONS & MESSAGES
-- ==============================================================================
INSERT INTO chat_conversations (conversation_id, conversation_uuid, user_id, admin_id, status, last_message_at, created_at, updated_at)
OVERRIDING SYSTEM VALUE VALUES
(1, gen_random_uuid(), 4, 1, 'open', now() - interval '2 hours', now() - interval '3 days', now()),
(2, gen_random_uuid(), 5, 2, 'open', now() - interval '1 day', now() - interval '5 days', now()),
(3, gen_random_uuid(), 6, 3, 'closed', now() - interval '4 days', now() - interval '6 days', now());

INSERT INTO chat_messages (conversation_id, sender_id, sender_role, content, content_type, is_read, created_at)
VALUES
(1, 4, 'user', 'Hi, is the iPhone 16 Pro Max eligible for 1-hour express shipping in Hanoi?', 'text', TRUE, now() - interval '3 days'),
(1, 1, 'admin', 'Hello An! Yes, we offer express 2-hour courier delivery within inner Hanoi districts.', 'text', TRUE, now() - interval '3 days' + interval '10 minutes'),
(1, 4, 'user', 'Awesome! I have placed my order. Looking forward to receiving it.', 'text', TRUE, now() - interval '2 hours'),
(2, 5, 'user', 'Does the MacBook Pro M3 Max come with international AppleCare warranty?', 'text', TRUE, now() - interval '5 days'),
(2, 2, 'admin', 'Hi Binh, yes! All our Apple products are 100% genuine VN/A with global Apple warranty support.', 'text', TRUE, now() - interval '5 days' + interval '15 minutes');

PERFORM setval('chat_conversations_conversation_id_seq', (SELECT MAX(conversation_id) FROM chat_conversations), true);
PERFORM setval('chat_messages_message_id_seq', (SELECT MAX(message_id) FROM chat_messages), true);

-- ==============================================================================
-- 13. SCALE-UP DATA (shop điện tử quy mô nhỏ-vừa: ~5k SKU, ~2k khách, ~8k đơn)
--     Dữ liệu sinh tự động, deterministic (setseed) nên chạy lại cho kết quả giống nhau.
--     Muốn đổi quy mô: sửa 4 biến v_extra_* ở phần DECLARE phía trên.
-- ==============================================================================
PERFORM setseed(0.42);

SELECT MAX(user_id)         INTO v_user_base  FROM users;
SELECT MAX(product_id)      INTO v_prod_base  FROM products;
SELECT MAX(order_id)        INTO v_order_base FROM orders;
SELECT MAX(conversation_id) INTO v_conv_base  FROM chat_conversations;

-- ------------------------------------------------------------------------------
-- 13.1 USERS (tạo trong ~120 ngày gần đây)
-- ------------------------------------------------------------------------------
INSERT INTO users (user_id, user_uuid, user_fullname, user_email, user_password, user_age, user_status, user_level, user_avatar, user_created_at, user_updated_at)
OVERRIDING SYSTEM VALUE
SELECT
    v_user_base + s,
    gen_random_uuid(),
    (ARRAY['Nguyen', 'Tran', 'Le', 'Pham', 'Hoang', 'Phan', 'Vu', 'Dang', 'Bui', 'Do'])[1 + (s % 10)] || ' ' ||
    (ARRAY['Van', 'Thi', 'Duc', 'Quoc', 'Thanh', 'Ngoc', 'Minh', 'Huu', 'Gia', 'Hai'])[1 + ((s * 3) % 10)] || ' ' ||
    (ARRAY['Anh', 'Binh', 'Chau', 'Dung', 'Em', 'Giang', 'Huy', 'Khoa', 'Linh', 'Mai', 'Nam', 'Phong', 'Quan', 'Son', 'Tuan', 'Vy'])[1 + ((s * 7) % 16)],
    'customer' || (v_user_base + s) || '@testmail.com',
    v_pwd,
    18 + (s % 45),
    1,
    3,
    'https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=150',
    now() - random() * interval '120 days',
    now()
FROM generate_series(1, v_extra_users) AS s;

PERFORM setval('users_user_id_seq', (SELECT MAX(user_id) FROM users), true);

-- ------------------------------------------------------------------------------
-- 13.2 USER ADDRESSES (1 địa chỉ mặc định / user, phân bố theo tỉnh thành)
-- ------------------------------------------------------------------------------
INSERT INTO user_addresses (ua_uuid, ua_user_id, ua_label, ua_name, ua_phone, ua_address, ua_is_default, ua_province_id, ua_province_name, ua_district_id, ua_district_name, ua_ward_code, ua_ward_name, ua_created_at, ua_updated_at)
SELECT
    gen_random_uuid(),
    u.user_id,
    'Home',
    u.user_fullname,
    '09' || lpad((10000000 + u.user_id * 179)::text, 8, '0'),
    'Số ' || (u.user_id % 300 + 1) || ' ' ||
        (ARRAY['Nguyễn Huệ', 'Lê Lợi', 'Hoàng Hoa Thám', 'Võ Văn Kiệt', 'Hai Bà Trưng', 'Điện Biên Phủ', 'Trần Hưng Đạo', 'Lý Thường Kiệt'])[1 + (u.user_id % 8)] ||
        ', Phường ' || (1 + u.user_id % 15) || ', Quận ' || (1 + u.user_id % 12) || ', ' ||
        (ARRAY['TP. Hồ Chí Minh', 'Hà Nội', 'Đà Nẵng', 'Cần Thơ', 'Hải Phòng'])[u.pv],
    TRUE,
    (ARRAY[202, 201, 203, 204, 205])[u.pv],
    (ARRAY['TP. Hồ Chí Minh', 'Hà Nội', 'Đà Nẵng', 'Cần Thơ', 'Hải Phòng'])[u.pv],
    1440 + (u.user_id % 12),
    'Quận ' || (1 + u.user_id % 12),
    '2B' || lpad((u.user_id % 50)::text, 4, '0'),
    'Phường ' || (1 + u.user_id % 15),
    u.user_created_at,
    u.user_created_at
FROM (
    SELECT user_id, user_fullname, user_created_at,
           CASE WHEN r < 0.40 THEN 1 WHEN r < 0.70 THEN 2 WHEN r < 0.80 THEN 3 WHEN r < 0.90 THEN 4 ELSE 5 END AS pv
    FROM (SELECT user_id, user_fullname, user_created_at, random() AS r
          FROM users WHERE user_id > v_user_base) t
) u;

-- ------------------------------------------------------------------------------
-- 13.3 PRODUCTS (brand thật + tên dòng giả để không giả mạo sản phẩm có thật)
--      Phân bố category không đều, giá lệch về phân khúc thấp, ~5% hết hàng.
-- ------------------------------------------------------------------------------
WITH imgs AS (
    SELECT product_category_id AS cat, array_agg(product_image_url ORDER BY product_id) AS arr
    FROM products
    WHERE product_id <= v_prod_base
    GROUP BY product_category_id
), g AS (
    SELECT s AS i,
           (ARRAY[1,1,1,2,2,2,2,3,3,3,3,3,4,4,5,5,5,6,7,7,8,8,8,8,8])[1 + ((s * 13) % 25)] AS cat,
           random() AS r_price,
           random() AS r_stock,
           random() AS r_date
    FROM generate_series(1, v_extra_products) s
), n AS (
    SELECT g.*,
        (CASE g.cat
            WHEN 1 THEN ARRAY['Apple','Samsung','Xiaomi','OPPO','Vivo','Realme','Google','Honor']
            WHEN 2 THEN ARRAY['Apple','Dell','HP','Lenovo','ASUS','Acer','MSI','LG']
            WHEN 3 THEN ARRAY['Sony','Bose','JBL','Sennheiser','Marshall','Soundcore','Edifier','Harman Kardon']
            WHEN 4 THEN ARRAY['Apple','Samsung','Garmin','Xiaomi','Huawei','Amazfit','Fitbit','Coros']
            WHEN 5 THEN ARRAY['Sony','Nintendo','Logitech','Razer','Corsair','SteelSeries','HyperX','Keychron']
            WHEN 6 THEN ARRAY['Sony','Canon','Fujifilm','Nikon','DJI','GoPro','Insta360','Panasonic']
            WHEN 7 THEN ARRAY['Roborock','Dyson','Philips','Xiaomi','Ecovacs','Samsung','Panasonic','Sharp']
            ELSE        ARRAY['Anker','Belkin','UGREEN','Baseus','Samsung','Apple','Aukey','Satechi']
         END)[1 + ((g.i * 3) % 8)] AS brand,
        (ARRAY['Aura','Nova','Edge','Prime','Vertex','Pulse','Zen','Orbit','Apex','Flux'])[1 + ((g.i * 7) % 10)]
            || ' ' || (1 + (g.i % 15)) AS series,
        (CASE g.cat
            WHEN 1 THEN ARRAY['Smartphone','Tablet','5G Smartphone','Foldable Phone']
            WHEN 2 THEN ARRAY['Laptop','Gaming Laptop','Ultrabook','Mini PC']
            WHEN 3 THEN ARRAY['Wireless Headphones','True Wireless Earbuds','Bluetooth Speaker','Soundbar']
            WHEN 4 THEN ARRAY['Smartwatch','Fitness Band','Sports Watch','Kids Watch']
            WHEN 5 THEN ARRAY['Gaming Mouse','Mechanical Keyboard','Gaming Headset','Game Console']
            WHEN 6 THEN ARRAY['Mirrorless Camera','Action Camera','Prime Lens','Gimbal']
            WHEN 7 THEN ARRAY['Robot Vacuum','Air Purifier','Smart Bulb Kit','Smart Plug']
            ELSE        ARRAY['Power Bank','GaN Charger','USB-C Cable','Wireless Charger']
         END)[1 + (g.i % 4)] AS ptype,
        CASE
            WHEN g.cat = 1 THEN (ARRAY['128GB','256GB','512GB','1TB'])[1 + ((g.i * 11) % 4)]
            WHEN g.cat = 2 THEN (ARRAY['16GB / 512GB','16GB / 1TB','32GB / 1TB','8GB / 256GB'])[1 + ((g.i * 11) % 4)]
            ELSE (ARRAY['Black','White','Silver','Blue','Graphite','Midnight'])[1 + ((g.i * 11) % 6)]
        END AS spec,
        -- Hệ số giá theo loại sản phẩm (cùng thứ tự với mảng ptype ở trên)
        (CASE g.cat
            WHEN 1 THEN ARRAY[1.0, 0.8, 0.7, 1.0]
            WHEN 2 THEN ARRAY[1.0, 1.0, 0.8, 0.4]
            WHEN 3 THEN ARRAY[0.8, 0.6, 0.5, 1.0]
            WHEN 4 THEN ARRAY[1.0, 0.15, 0.8, 0.12]
            WHEN 5 THEN ARRAY[0.2, 0.3, 0.25, 1.0]
            WHEN 6 THEN ARRAY[1.0, 0.35, 0.6, 0.2]
            WHEN 7 THEN ARRAY[1.0, 0.6, 0.1, 0.03]
            ELSE        ARRAY[0.6, 0.5, 0.1, 0.5]
         END)[1 + (g.i % 4)] AS pf,
        (ARRAY[3000000, 12000000,  500000, 1000000,  400000, 1000000,  300000, 100000])[g.cat] AS lo,
        (ARRAY[40000000, 90000000, 10000000, 27000000, 20000000, 55000000, 27000000, 4000000])[g.cat] AS hi
    FROM g
), f AS (
    SELECT n.*,
           n.brand || ' ' || n.series || ' ' || n.ptype || ' ' || n.spec || ' #' || n.i AS name,
           (round((n.lo + power(n.r_price, 1.6) * (n.hi - n.lo) * n.pf) / 100000) * 100000 - 10000)::numeric(14,2) AS price,
           (CASE WHEN n.r_stock < 0.05 THEN 0 ELSE 5 + floor(n.r_stock * 295)::int END) AS stock,
           now() - (120 + n.r_date * 280) * interval '1 day' AS created
    FROM n
)
INSERT INTO products (product_uuid, product_name, product_slug, product_description, product_price, product_stock, product_sku, product_category_id, product_image_url, product_status, product_avg_rating, product_review_count, product_created_at, product_updated_at)
SELECT
    gen_random_uuid(),
    f.name,
    regexp_replace(lower(f.name), '[^a-z0-9]+', '-', 'g') || '-' || f.i,
    f.brand || ' ' || f.ptype || ' ' || f.spec || '. Genuine product with 12-month official warranty, free 7-day returns and nationwide delivery.',
    f.price,
    f.stock,
    'GEN-' || upper(left(f.brand, 3)) || '-' || f.cat || '-' || lpad(f.i::text, 5, '0'),
    f.cat,
    imgs.arr[1 + (f.i % array_length(imgs.arr, 1))],
    1,
    0,
    0,
    f.created,
    f.created
FROM f
JOIN imgs ON imgs.cat = f.cat
ORDER BY f.i;

SELECT MAX(product_id) INTO v_total_products FROM products;

-- ------------------------------------------------------------------------------
-- 13.4 ORDERS (khách mua lặp lại nhiều: phân bố lệch về một nhóm user)
--      Ngày đặt luôn sau ngày tạo tài khoản; trạng thái phụ thuộc độ tuổi đơn.
-- ------------------------------------------------------------------------------
WITH base AS (
    SELECT s,
           (4 + floor(power(random(), 1.5) * (v_user_base + v_extra_users - 3)))::int AS uid,
           random() AS r_time, random() AS r_status, random() AS r_pay
    FROM generate_series(1, v_extra_orders) s
), o AS (
    SELECT b.s, b.uid, b.r_status, b.r_pay,
           a.ua_name, a.ua_phone, a.ua_address,
           u.user_created_at + b.r_time * (now() - u.user_created_at) AS created_at
    FROM base b
    JOIN users u ON u.user_id = b.uid
    JOIN (SELECT DISTINCT ON (ua_user_id) ua_user_id, ua_name, ua_phone, ua_address
          FROM user_addresses
          ORDER BY ua_user_id, ua_is_default DESC, ua_id) a ON a.ua_user_id = b.uid
), st AS (
    SELECT o.*,
           CASE WHEN o.r_pay < 0.40 THEN 1 ELSE 2 END AS pm,        -- 1 = COD, 2 = online
           CASE
               WHEN now() - o.created_at < interval '2 days' THEN
                    CASE WHEN o.r_status < 0.5 THEN 1 ELSE 2 END
               WHEN now() - o.created_at < interval '7 days' THEN
                    CASE WHEN o.r_status < 0.2 THEN 2 WHEN o.r_status < 0.6 THEN 3
                         WHEN o.r_status < 0.9 THEN 4 ELSE 5 END
               ELSE CASE WHEN o.r_status < 0.88 THEN 4 ELSE 5 END   -- 88% delivered, 12% cancelled
           END AS status
    FROM o
)
INSERT INTO orders (order_id, order_uuid, order_number, order_user_id, order_shipping_name, order_shipping_phone, order_shipping_address,
                    order_subtotal_amount, order_discount_amount, order_total_amount, order_refunded_amount,
                    order_payment_method, order_payment_status, order_status, order_shipping_provider, order_shipping_code,
                    order_created_at, order_updated_at)
OVERRIDING SYSTEM VALUE
SELECT
    v_order_base + st.s,
    gen_random_uuid(),
    'ORD-' || to_char(st.created_at, 'YYYYMMDD') || '-' || lpad(st.s::text, 6, '0'),
    st.uid,
    st.ua_name,
    st.ua_phone,
    st.ua_address,
    0, 0, 0, 0,                                   -- cập nhật lại từ order_items ở bước sau
    st.pm,
    CASE WHEN st.status = 4 THEN 2
         WHEN st.status IN (2, 3) AND st.pm = 2 THEN 2
         ELSE 1 END,
    st.status,
    CASE WHEN st.status IN (3, 4) THEN 'ghn' END,
    CASE WHEN st.status IN (3, 4) THEN 'GHN' || (900000000 + (v_order_base + st.s) * 131)::text || 'VN' END,
    st.created_at,
    CASE WHEN st.status = 1 THEN st.created_at ELSE LEAST(st.created_at + interval '3 days', now()) END
FROM st;

PERFORM setval('orders_order_id_seq', (SELECT MAX(order_id) FROM orders), true);

-- 1-4 sản phẩm / đơn (55% / 25% / 12% / 8%); sản phẩm bán chạy được "rải" đều khắp catalog
WITH oo AS (
    SELECT t.order_id, t.order_created_at,
           CASE WHEN t.r < 0.55 THEN 1 WHEN t.r < 0.80 THEN 2 WHEN t.r < 0.92 THEN 3 ELSE 4 END AS n,
           ((floor(power(random(), 2) * v_total_products)::bigint * 7919) % v_total_products)::int AS base_p
    FROM (SELECT order_id, order_created_at, random() AS r
          FROM orders WHERE order_id > v_order_base) t
)
INSERT INTO order_items (order_item_uuid, order_item_order_id, order_item_product_id, order_item_product_name, order_item_price, order_item_quantity, order_item_subtotal, order_item_created_at)
SELECT
    gen_random_uuid(),
    oo.order_id,
    p.product_id,
    p.product_name,
    p.product_price,
    q.qty,
    p.product_price * q.qty,
    oo.order_created_at
FROM oo
CROSS JOIN LATERAL generate_series(1, oo.n) AS k
JOIN products p ON p.product_id = 1 + ((oo.base_p + (k - 1) * 3571) % v_total_products)
CROSS JOIN LATERAL (SELECT 1 + (random() < 0.15)::int + (random() < 0.04)::int AS qty) q;

-- Tính lại tiền: miễn phí ship từ 2.000.000đ, ngược lại 30.000đ
UPDATE orders o
SET order_subtotal_amount = s.sub,
    order_shipping_fee    = CASE WHEN s.sub >= 2000000 THEN 0 ELSE 30000 END,
    order_total_amount    = s.sub + CASE WHEN s.sub >= 2000000 THEN 0 ELSE 30000 END
FROM (SELECT order_item_order_id AS oid, SUM(order_item_subtotal) AS sub
      FROM order_items
      WHERE order_item_order_id > v_order_base
      GROUP BY order_item_order_id) s
WHERE o.order_id = s.oid;

-- Giao dịch thanh toán: mỗi đơn 1 giao dịch (COD / VNPay / MoMo)
INSERT INTO payment_transactions (pt_uuid, pt_order_id, pt_idempotency_key, pt_provider, pt_txn_ref, pt_provider_txn_id, pt_amount, pt_currency, pt_status, pt_paid_at, pt_created_at, pt_updated_at)
SELECT
    gen_random_uuid(),
    o.order_id,
    'IDEM-GEN-' || o.order_id,
    CASE WHEN o.order_payment_method = 1 THEN 'cod'
         WHEN o.order_id % 2 = 0 THEN 'vnpay' ELSE 'momo' END,
    CASE WHEN o.order_payment_method = 2 THEN 'TXN-' || o.order_number END,
    CASE WHEN o.order_payment_method = 2 AND o.order_payment_status = 2 THEN 'PRV' || (100000000 + o.order_id * 17)::text END,
    o.order_total_amount,
    'VND',
    CASE WHEN o.order_payment_status = 2 THEN 2 ELSE 1 END,
    CASE WHEN o.order_payment_status = 2
         THEN LEAST(o.order_created_at + CASE WHEN o.order_payment_method = 1 THEN interval '3 days' ELSE interval '5 minutes' END, now()) END,
    o.order_created_at,
    o.order_updated_at
FROM orders o
WHERE o.order_id > v_order_base;

-- Lịch sử vận chuyển (đơn hủy chỉ có sự kiện 'pending')
INSERT INTO order_tracking_events (event_order_id, event_status, event_note, event_source, event_created_at)
SELECT o.order_id, e.status, e.note, e.src, LEAST(o.order_created_at + e.off, now())
FROM orders o
CROSS JOIN LATERAL (VALUES
    (1, 'pending',   'Order placed successfully by customer',          'system', interval '0 minutes'),
    (2, 'confirmed', 'Order confirmed by store warehouse',             'admin',  interval '2 hours'),
    (3, 'shipping',  'Package handed over to GHN courier',             'ghn',    interval '1 day'),
    (4, 'delivered', 'Order successfully delivered to customer',       'ghn',    interval '3 days')
) AS e(lvl, status, note, src, off)
WHERE o.order_id > v_order_base
  AND (e.lvl = 1 OR (o.order_status <> 5 AND o.order_status >= e.lvl));

-- ------------------------------------------------------------------------------
-- 13.5 REVIEWS (~35% đơn đã giao có 1 đánh giá; lệch về 4-5 sao như thực tế)
-- ------------------------------------------------------------------------------
INSERT INTO product_reviews (review_uuid, review_user_id, review_product_id, review_order_id, review_rating, review_title, review_comment, review_has_media, review_status, review_helpful_count, review_created_at, review_updated_at)
SELECT
    gen_random_uuid(),
    y.uid,
    y.pid,
    y.oid,
    y.rating::smallint,
    CASE WHEN y.rating >= 4 THEN (ARRAY['Excellent build quality!', 'Worth every single penny', 'Super fast delivery and pristine packaging', 'Authentic product with full warranty', 'Exceeded my expectations'])[1 + (y.oid % 5)]
         WHEN y.rating = 3  THEN (ARRAY['Decent, but not perfect', 'Good for the price', 'Average experience', 'Okay overall', 'Does the job'])[1 + (y.oid % 5)]
         ELSE                    (ARRAY['Not as expected', 'Disappointed with quality', 'Had issues after a week', 'Packaging was damaged', 'Would not buy again'])[1 + (y.oid % 5)]
    END,
    CASE WHEN y.rating >= 4 THEN (ARRAY['Highly recommended for anyone looking for reliable premium performance.', 'The design and aesthetics match the photos perfectly. Very happy with this purchase.', 'Customer support was very helpful when answering my questions prior to checkout.', 'Battery life and build material feel top of the line.', 'Works right out of the box without any issues whatsoever.'])[1 + (y.oid % 5)]
         WHEN y.rating = 3  THEN (ARRAY['Works fine, though a few details could be better for this price.', 'Performance is okay, delivery took longer than promised.', 'Matches the description, nothing special but no major complaints.', 'Build is fine, software could use some polish.', 'Reasonable value, I expected slightly better finishing.'])[1 + (y.oid % 5)]
         ELSE                    (ARRAY['The product did not match my expectations, I contacted support for a return.', 'Quality felt below the price point and one feature stopped working.', 'Arrived with scratches on the box and a loose accessory.', 'Battery drains much faster than advertised.', 'Support was slow to respond and the issue was not fully solved.'])[1 + (y.oid % 5)]
    END,
    FALSE,
    1,
    floor(power(random(), 3) * 25)::int,
    y.created,
    y.created
FROM (
    SELECT x.oid, x.uid, x.pid,
           CASE WHEN x.r < 0.55 THEN 5 WHEN x.r < 0.83 THEN 4 WHEN x.r < 0.93 THEN 3 WHEN x.r < 0.97 THEN 2 ELSE 1 END AS rating,
           LEAST(x.created0 + (1 + random() * 6) * interval '1 day', now()) AS created
    FROM (
        SELECT DISTINCT ON (o.order_id)
               o.order_id AS oid, o.order_user_id AS uid, oi.order_item_product_id AS pid,
               o.order_created_at AS created0, random() AS r
        FROM orders o
        JOIN order_items oi ON oi.order_item_order_id = o.order_id
        WHERE o.order_id > v_order_base AND o.order_status = 4 AND random() < 0.35
        ORDER BY o.order_id, oi.order_item_id
    ) x
) y
ON CONFLICT DO NOTHING;

UPDATE products p
SET product_avg_rating  = sub.avg_r,
    product_review_count = sub.cnt
FROM (
    SELECT review_product_id, ROUND(AVG(review_rating)::numeric, 1) AS avg_r, COUNT(*) AS cnt
    FROM product_reviews
    WHERE review_status = 1
    GROUP BY review_product_id
) sub
WHERE p.product_id = sub.review_product_id;

-- ------------------------------------------------------------------------------
-- 13.6 CARTS (~40% user mới có giỏ hàng, 1-3 món)
-- ------------------------------------------------------------------------------
INSERT INTO carts (cart_id, cart_uuid, cart_user_id, cart_created_at, cart_updated_at)
OVERRIDING SYSTEM VALUE
SELECT user_id, gen_random_uuid(), user_id, now() - random() * interval '10 days', now()
FROM users
WHERE user_id > v_user_base AND random() < 0.4;

PERFORM setval('carts_cart_id_seq', (SELECT MAX(cart_id) FROM carts), true);

INSERT INTO cart_items (cart_item_uuid, cart_item_cart_id, cart_item_product_id, cart_item_quantity, cart_item_created_at, cart_item_updated_at)
SELECT
    gen_random_uuid(),
    c.cart_id,
    p.product_id,
    1 + (random() < 0.2)::int,
    now() - random() * interval '5 days',
    now()
FROM carts c
CROSS JOIN LATERAL generate_series(1, 1 + (c.cart_id % 3)) AS k
JOIN products p ON p.product_id = 1 + ((c.cart_id * 37 + k * 7919) % v_total_products)
WHERE c.cart_id > v_user_base
ON CONFLICT (cart_item_cart_id, cart_item_product_id) DO NOTHING;

-- ------------------------------------------------------------------------------
-- 13.7 STOCK MOVEMENTS (chỉ cho sản phẩm sinh thêm; số liệu khớp với product_stock hiện tại)
--      intake = tồn hiện tại + tổng đã bán (đơn không hủy)
-- ------------------------------------------------------------------------------
INSERT INTO stock_movements (product_id, change_type, quantity_change, stock_before, stock_after, reference_id, reference_type, actor_id, note, created_at)
SELECT
    p.product_id,
    'INITIAL_STOCK',
    p.product_stock + COALESCE(s.qty, 0),
    0,
    p.product_stock + COALESCE(s.qty, 0),
    'INIT-BATCH-GEN',
    'product_edit',
    1,
    'Initial inventory intake from distributor warehouse',
    p.product_created_at
FROM products p
LEFT JOIN (
    SELECT oi.order_item_product_id AS pid, SUM(oi.order_item_quantity) AS qty
    FROM order_items oi
    JOIN orders o ON o.order_id = oi.order_item_order_id
    WHERE o.order_id > v_order_base AND o.order_status <> 5
    GROUP BY oi.order_item_product_id
) s ON s.pid = p.product_id
WHERE p.product_id > v_prod_base;

INSERT INTO stock_movements (product_id, change_type, quantity_change, stock_before, stock_after, reference_id, reference_type, actor_id, note, created_at)
SELECT
    t.pid,
    'ORDER_PLACED',
    -t.qty,
    t.intake - (t.cum - t.qty),
    t.intake - t.cum,
    t.order_number,
    'order',
    t.uid,
    'Automated stock reduction on order placement',
    t.created
FROM (
    SELECT oi.order_item_product_id AS pid,
           oi.order_item_quantity   AS qty,
           o.order_number,
           o.order_user_id          AS uid,
           o.order_created_at       AS created,
           p.product_stock + SUM(oi.order_item_quantity) OVER (PARTITION BY oi.order_item_product_id) AS intake,
           SUM(oi.order_item_quantity) OVER (PARTITION BY oi.order_item_product_id
                                             ORDER BY o.order_created_at, oi.order_item_id) AS cum
    FROM order_items oi
    JOIN orders o   ON o.order_id = oi.order_item_order_id
    JOIN products p ON p.product_id = oi.order_item_product_id
    WHERE o.order_id > v_order_base
      AND o.order_status <> 5
      AND oi.order_item_product_id > v_prod_base
) t;

-- ------------------------------------------------------------------------------
-- 13.8 NOTIFICATIONS (đơn trong 60 ngày gần đây đã xác nhận / đang giao / đã giao)
-- ------------------------------------------------------------------------------
INSERT INTO notifications (notification_uuid, notification_user_id, notification_type, notification_title, notification_body, notification_data, notification_is_read, notification_created_at)
SELECT
    gen_random_uuid(),
    o.order_user_id,
    CASE o.order_status WHEN 2 THEN 'order_confirmed' WHEN 3 THEN 'order_shipped' ELSE 'order_delivered' END,
    'Order #' || o.order_number || CASE o.order_status WHEN 2 THEN ' Confirmed' WHEN 3 THEN ' is on its way' ELSE ' Delivered Successfully' END,
    CASE o.order_status WHEN 2 THEN 'Your order has been verified and is being packaged.'
                        WHEN 3 THEN 'Your package has been handed over to the courier.'
                        ELSE 'Package delivered. Thank you for shopping with GOshop!' END,
    jsonb_build_object('order_id', o.order_id, 'order_number', o.order_number),
    (o.order_created_at < now() - interval '3 days' AND random() < 0.85),
    LEAST(o.order_created_at + interval '1 day', now())
FROM orders o
WHERE o.order_id > v_order_base
  AND o.order_status IN (2, 3, 4)
  AND o.order_created_at > now() - interval '60 days';

-- ------------------------------------------------------------------------------
-- 13.9 CHAT (v_extra_chats hội thoại x 3 tin nhắn)
-- ------------------------------------------------------------------------------
INSERT INTO chat_conversations (conversation_id, conversation_uuid, user_id, admin_id, status, last_message_at, created_at, updated_at)
OVERRIDING SYSTEM VALUE
SELECT v_conv_base + t.s, gen_random_uuid(), 10 + t.s, 1 + (t.s % 3),
       CASE WHEN t.s % 4 = 0 THEN 'open' ELSE 'closed' END,
       t.ts, t.ts - interval '1 hour', t.ts
FROM (SELECT s, now() - random() * interval '60 days' AS ts
      FROM generate_series(1, v_extra_chats) s) t;

PERFORM setval('chat_conversations_conversation_id_seq', (SELECT MAX(conversation_id) FROM chat_conversations), true);

INSERT INTO chat_messages (conversation_id, sender_id, sender_role, content, content_type, is_read, created_at)
SELECT c.conversation_id,
       CASE m.role WHEN 'user' THEN c.user_id ELSE c.admin_id END,
       m.role, m.content, 'text', TRUE, c.created_at + m.off
FROM chat_conversations c
CROSS JOIN LATERAL (VALUES
    ('user',  (ARRAY['Is this product still in stock?', 'Can I get an invoice for my order?', 'How long does delivery to my province take?', 'Does it come with an official warranty?', 'Can I return it if I change my mind?'])[1 + (c.conversation_id % 5)], interval '0 minutes'),
    ('admin', (ARRAY['Hi! Yes, it is in stock and ready to ship today.', 'Sure, we will email the VAT invoice within 24 hours.', 'Usually 2-4 business days depending on the courier.', 'Yes, all items come with the manufacturer warranty.', 'Absolutely, returns are free within 7 days.'])[1 + (c.conversation_id % 5)], interval '10 minutes'),
    ('user',  'Thank you, that is very helpful!', interval '20 minutes')
) AS m(role, content, off)
WHERE c.conversation_id > v_conv_base;

RAISE NOTICE 'Seed done: % users, % categories, % products, % orders, % order_items, % reviews',
    (SELECT COUNT(*) FROM users), (SELECT COUNT(*) FROM categories), (SELECT COUNT(*) FROM products),
    (SELECT COUNT(*) FROM orders), (SELECT COUNT(*) FROM order_items), (SELECT COUNT(*) FROM product_reviews);

END $$;

COMMIT;

-- Analyze tables for fast query planner statistics
ANALYZE users;
ANALYZE categories;
ANALYZE products;
ANALYZE user_addresses;
ANALYZE coupons;
ANALYZE coupon_usages;
ANALYZE orders;
ANALYZE order_items;
ANALYZE order_tracking_events;
ANALYZE payment_transactions;
ANALYZE product_reviews;
ANALYZE product_review_helpful_votes;
ANALYZE carts;
ANALYZE cart_items;
ANALYZE stock_movements;
ANALYZE banners;
ANALYZE notifications;
ANALYZE chat_conversations;
ANALYZE chat_messages;