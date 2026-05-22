ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS custom_models jsonb NOT NULL DEFAULT '[]'::jsonb;
