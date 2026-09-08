ALTER TABLE user_addresses
    DROP COLUMN IF EXISTS ua_province_id,
    DROP COLUMN IF EXISTS ua_province_name,
    DROP COLUMN IF EXISTS ua_district_id,
    DROP COLUMN IF EXISTS ua_district_name,
    DROP COLUMN IF EXISTS ua_ward_code,
    DROP COLUMN IF EXISTS ua_ward_name;