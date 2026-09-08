ALTER TABLE order_returns
    DROP COLUMN IF EXISTS return_images,
    DROP COLUMN IF EXISTS return_video_url;