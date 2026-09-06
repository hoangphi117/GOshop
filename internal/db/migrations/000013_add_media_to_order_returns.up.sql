ALTER TABLE order_returns
    ADD COLUMN IF NOT EXISTS return_images    TEXT[] DEFAULT '{}' NOT NULL,
    ADD COLUMN IF NOT EXISTS return_video_url TEXT;
