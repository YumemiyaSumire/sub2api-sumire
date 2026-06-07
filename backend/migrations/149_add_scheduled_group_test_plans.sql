-- 149_add_scheduled_group_test_plans.sql
-- Group-level scheduled account connection tests.

CREATE TABLE IF NOT EXISTS scheduled_group_test_plans (
    id                  BIGSERIAL PRIMARY KEY,
    group_id            BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    account_name_filter TEXT NOT NULL DEFAULT '',
    model_id            VARCHAR(100) NOT NULL DEFAULT 'gpt-5.5',
    enabled             BOOLEAN NOT NULL DEFAULT true,
    last_run_at         TIMESTAMPTZ,
    next_run_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sgtp_group_id ON scheduled_group_test_plans(group_id);
CREATE INDEX IF NOT EXISTS idx_sgtp_enabled_next_run ON scheduled_group_test_plans(enabled, next_run_at) WHERE enabled = true;
