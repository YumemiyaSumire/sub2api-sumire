package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

// ScheduledGroupTestService provides CRUD operations for group scheduled tests.
type ScheduledGroupTestService struct {
	repo ScheduledGroupTestPlanRepository
	now  func() time.Time
}

func NewScheduledGroupTestService(repo ScheduledGroupTestPlanRepository) *ScheduledGroupTestService {
	return &ScheduledGroupTestService{
		repo: repo,
		now:  time.Now,
	}
}

func (s *ScheduledGroupTestService) CreatePlan(ctx context.Context, plan *ScheduledGroupTestPlan) (*ScheduledGroupTestPlan, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("scheduled group test service is not configured")
	}
	if plan == nil {
		return nil, errors.New("plan is required")
	}
	if plan.GroupID <= 0 {
		return nil, errors.New("group_id is required")
	}
	plan.AccountNameFilter = strings.TrimSpace(plan.AccountNameFilter)
	plan.ModelID = ScheduledGroupTestModelID
	nextRun := s.now()
	plan.NextRunAt = &nextRun
	return s.repo.Create(ctx, plan)
}

func (s *ScheduledGroupTestService) GetPlan(ctx context.Context, id int64) (*ScheduledGroupTestPlan, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("scheduled group test service is not configured")
	}
	return s.repo.GetByID(ctx, id)
}

func (s *ScheduledGroupTestService) ListPlans(ctx context.Context) ([]*ScheduledGroupTestPlan, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("scheduled group test service is not configured")
	}
	return s.repo.List(ctx)
}

func (s *ScheduledGroupTestService) UpdatePlan(ctx context.Context, plan *ScheduledGroupTestPlan, wasEnabled bool) (*ScheduledGroupTestPlan, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("scheduled group test service is not configured")
	}
	if plan == nil {
		return nil, errors.New("plan is required")
	}
	if plan.ID <= 0 {
		return nil, errors.New("plan id is required")
	}
	if plan.GroupID <= 0 {
		return nil, errors.New("group_id is required")
	}
	plan.AccountNameFilter = strings.TrimSpace(plan.AccountNameFilter)
	plan.ModelID = ScheduledGroupTestModelID
	if plan.Enabled && !wasEnabled {
		nextRun := s.now()
		plan.NextRunAt = &nextRun
	}
	return s.repo.Update(ctx, plan)
}

func (s *ScheduledGroupTestService) DeletePlan(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return errors.New("scheduled group test service is not configured")
	}
	return s.repo.Delete(ctx, id)
}
