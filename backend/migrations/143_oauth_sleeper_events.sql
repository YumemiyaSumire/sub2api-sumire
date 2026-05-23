-- OAuth Sleeper audit trail.
-- The scanner writes one row only after an account rate_limit_reset_at update succeeds.
CREATE TABLE IF NOT EXISTS oauth_sleeper_events (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL,
    account_name VARCHAR(100) NOT NULL DEFAULT '',
    platform VARCHAR(50) NOT NULL,
    window VARCHAR(64) NOT NULL,
    utilization_percent NUMERIC(8,4) NOT NULL,
    threshold_percent NUMERIC(8,4) NOT NULL,
    reset_at TIMESTAMPTZ NOT NULL,
    previous_rate_limit_reset_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_oauth_sleeper_events_created_at
    ON oauth_sleeper_events (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_oauth_sleeper_events_account_id
    ON oauth_sleeper_events (account_id);

CREATE INDEX IF NOT EXISTS idx_oauth_sleeper_events_platform_created_at
    ON oauth_sleeper_events (platform, created_at DESC);
