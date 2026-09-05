ALTER TABLE user_addresses
    ADD COLUMN ua_province_id   INT,
    ADD COLUMN ua_province_name VARCHAR(100),
    ADD COLUMN ua_district_id   INT,
    ADD COLUMN ua_district_name VARCHAR(100),
    ADD COLUMN ua_ward_code     VARCHAR(50),
    ADD COLUMN ua_ward_name     VARCHAR(100);
