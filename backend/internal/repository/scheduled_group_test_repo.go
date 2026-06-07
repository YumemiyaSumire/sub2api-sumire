package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type scheduledGroupTestPlanRepository struct {
	db *sql.DB
}

func NewScheduledGroupTestPlanRepository(db *sql.DB) service.ScheduledGroupTestPlanRepository {
	return &scheduledGroupTestPlanRepository{db: db}
}

func (r *scheduledGroupTestPlanRepository) Create(ctx context.Context, plan *service.ScheduledGroupTestPlan) (*service.ScheduledGroupTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO scheduled_group_test_plans (group_id, account_name_filter, model_id, enabled, next_run_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		RETURNING id, group_id, account_name_filter, model_id, enabled, last_run_at, next_run_at, created_at, updated_at
	`, plan.GroupID, plan.AccountNameFilter, plan.ModelID, plan.Enabled, plan.NextRunAt)
	return scanScheduledGroupTestPlan(row)
}

func (r *scheduledGroupTestPlanRepository) GetByID(ctx context.Context, id int64) (*service.ScheduledGroupTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, group_id, account_name_filter, model_id, enabled, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_group_test_plans
		WHERE id = $1
	`, id)
	return scanScheduledGroupTestPlan(row)
}

func (r *scheduledGroupTestPlanRepository) List(ctx context.Context) ([]*service.ScheduledGroupTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, group_id, account_name_filter, model_id, enabled, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_group_test_plans
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanScheduledGroupTestPlans(rows)
}

func (r *scheduledGroupTestPlanRepository) ListDue(ctx context.Context, now time.Time) ([]*service.ScheduledGroupTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, group_id, account_name_filter, model_id, enabled, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_group_test_plans
		WHERE enabled = true AND next_run_at <= $1
		ORDER BY next_run_at ASC, id ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanScheduledGroupTestPlans(rows)
}

func (r *scheduledGroupTestPlanRepository) Update(ctx context.Context, plan *service.ScheduledGroupTestPlan) (*service.ScheduledGroupTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE scheduled_group_test_plans
		SET group_id = $2, account_name_filter = $3, model_id = $4, enabled = $5, next_run_at = $6, updated_at = NOW()
		WHERE id = $1
		RETURNING id, group_id, account_name_filter, model_id, enabled, last_run_at, next_run_at, created_at, updated_at
	`, plan.ID, plan.GroupID, plan.AccountNameFilter, plan.ModelID, plan.Enabled, plan.NextRunAt)
	return scanScheduledGroupTestPlan(row)
}

func (r *scheduledGroupTestPlanRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM scheduled_group_test_plans WHERE id = $1`, id)
	return err
}

func (r *scheduledGroupTestPlanRepository) UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_group_test_plans SET last_run_at = $2, next_run_at = $3, updated_at = NOW() WHERE id = $1
	`, id, lastRunAt, nextRunAt)
	return err
}

func scanScheduledGroupTestPlan(row scannable) (*service.ScheduledGroupTestPlan, error) {
	p := &service.ScheduledGroupTestPlan{}
	if err := row.Scan(
		&p.ID,
		&p.GroupID,
		&p.AccountNameFilter,
		&p.ModelID,
		&p.Enabled,
		&p.LastRunAt,
		&p.NextRunAt,
		&p.CreatedAt,
		&p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return p, nil
}

func scanScheduledGroupTestPlans(rows *sql.Rows) ([]*service.ScheduledGroupTestPlan, error) {
	var plans []*service.ScheduledGroupTestPlan
	for rows.Next() {
		p, err := scanScheduledGroupTestPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}
