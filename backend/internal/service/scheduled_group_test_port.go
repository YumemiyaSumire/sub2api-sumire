package service

import (
	"context"
	"time"
)

const ScheduledGroupTestModelID = "gpt-5.5"

// ScheduledGroupTestPlan represents a group-level scheduled connection test.
type ScheduledGroupTestPlan struct {
	ID                int64      `json:"id"`
	GroupID           int64      `json:"group_id"`
	AccountNameFilter string     `json:"account_name_filter"`
	ModelID           string     `json:"model_id"`
	Enabled           bool       `json:"enabled"`
	LastRunAt         *time.Time `json:"last_run_at"`
	NextRunAt         *time.Time `json:"next_run_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// ScheduledGroupTestPlanRepository defines persistence for group scheduled tests.
type ScheduledGroupTestPlanRepository interface {
	Create(ctx context.Context, plan *ScheduledGroupTestPlan) (*ScheduledGroupTestPlan, error)
	GetByID(ctx context.Context, id int64) (*ScheduledGroupTestPlan, error)
	List(ctx context.Context) ([]*ScheduledGroupTestPlan, error)
	ListDue(ctx context.Context, now time.Time) ([]*ScheduledGroupTestPlan, error)
	Update(ctx context.Context, plan *ScheduledGroupTestPlan) (*ScheduledGroupTestPlan, error)
	Delete(ctx context.Context, id int64) error
	UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error
}

// ScheduledGroupAccountRepository provides the schedulable accounts for a group plan.
type ScheduledGroupAccountRepository interface {
	ListSchedulableByGroupID(ctx context.Context, groupID int64) ([]Account, error)
}

// ScheduledGroupAccountTester runs the account connectivity test used by group plans.
type ScheduledGroupAccountTester interface {
	RunTestBackground(ctx context.Context, accountID int64, modelID string) (*ScheduledTestResult, error)
}
