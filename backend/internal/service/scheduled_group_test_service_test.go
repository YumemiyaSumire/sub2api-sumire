package service

import (
	"context"
	"errors"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"time"
)

type scheduledGroupPlanRepoFake struct {
	created        *ScheduledGroupTestPlan
	plans          []*ScheduledGroupTestPlan
	updated        *ScheduledGroupTestPlan
	afterRunID     int64
	afterRunLast   time.Time
	afterRunNext   time.Time
	deletedID      int64
	nextID         int64
	getByID        *ScheduledGroupTestPlan
	listDueResults []*ScheduledGroupTestPlan
}

func (r *scheduledGroupPlanRepoFake) Create(_ context.Context, plan *ScheduledGroupTestPlan) (*ScheduledGroupTestPlan, error) {
	clone := *plan
	if r.nextID == 0 {
		r.nextID = 1
	}
	clone.ID = r.nextID
	r.created = &clone
	return &clone, nil
}

func (r *scheduledGroupPlanRepoFake) GetByID(_ context.Context, _ int64) (*ScheduledGroupTestPlan, error) {
	if r.getByID == nil {
		return nil, errors.New("not found")
	}
	clone := *r.getByID
	return &clone, nil
}

func (r *scheduledGroupPlanRepoFake) List(_ context.Context) ([]*ScheduledGroupTestPlan, error) {
	return r.plans, nil
}

func (r *scheduledGroupPlanRepoFake) ListDue(_ context.Context, _ time.Time) ([]*ScheduledGroupTestPlan, error) {
	return r.listDueResults, nil
}

func (r *scheduledGroupPlanRepoFake) Update(_ context.Context, plan *ScheduledGroupTestPlan) (*ScheduledGroupTestPlan, error) {
	clone := *plan
	r.updated = &clone
	return &clone, nil
}

func (r *scheduledGroupPlanRepoFake) Delete(_ context.Context, id int64) error {
	r.deletedID = id
	return nil
}

func (r *scheduledGroupPlanRepoFake) UpdateAfterRun(_ context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	r.afterRunID = id
	r.afterRunLast = lastRunAt
	r.afterRunNext = nextRunAt
	return nil
}

type scheduledGroupAccountRepoFake struct {
	groupID  int64
	accounts []Account
}

func (r *scheduledGroupAccountRepoFake) ListSchedulableByGroupID(_ context.Context, groupID int64) ([]Account, error) {
	r.groupID = groupID
	return r.accounts, nil
}

type scheduledGroupTesterFake struct {
	calls   []scheduledGroupTesterCall
	results map[int64]*ScheduledTestResult
	errs    map[int64]error
}

type scheduledGroupTesterCall struct {
	accountID int64
	modelID   string
}

func (t *scheduledGroupTesterFake) RunTestBackground(_ context.Context, accountID int64, modelID string) (*ScheduledTestResult, error) {
	t.calls = append(t.calls, scheduledGroupTesterCall{accountID: accountID, modelID: modelID})
	if err := t.errs[accountID]; err != nil {
		return t.results[accountID], err
	}
	if result := t.results[accountID]; result != nil {
		return result, nil
	}
	return &ScheduledTestResult{Status: "success", LatencyMs: 10}, nil
}

func TestScheduledGroupTestServiceCreatePlanUsesFixedModelAndImmediateRun(t *testing.T) {
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	repo := &scheduledGroupPlanRepoFake{}
	svc := NewScheduledGroupTestService(repo)
	svc.now = func() time.Time { return now }

	created, err := svc.CreatePlan(context.Background(), &ScheduledGroupTestPlan{
		GroupID:           42,
		AccountNameFilter: "  codex  ",
		Enabled:           false,
		ModelID:           "custom-model",
	})
	if err != nil {
		t.Fatalf("CreatePlan returned error: %v", err)
	}

	if created.ModelID != ScheduledGroupTestModelID {
		t.Fatalf("model id = %q, want %q", created.ModelID, ScheduledGroupTestModelID)
	}
	if created.AccountNameFilter != "codex" {
		t.Fatalf("account_name_filter = %q, want trimmed codex", created.AccountNameFilter)
	}
	if created.Enabled {
		t.Fatalf("enabled = true, want false when request disables plan")
	}
	if created.NextRunAt == nil || !created.NextRunAt.Equal(now) {
		t.Fatalf("next_run_at = %v, want %v", created.NextRunAt, now)
	}
}

func TestScheduledGroupTestServiceRejectsMissingGroupID(t *testing.T) {
	svc := NewScheduledGroupTestService(&scheduledGroupPlanRepoFake{})

	if _, err := svc.CreatePlan(context.Background(), &ScheduledGroupTestPlan{}); err == nil {
		t.Fatal("CreatePlan returned nil error, want group_id validation error")
	}
}

func TestFilterScheduledGroupTestAccountsMatchesNameAndSkipsUnschedulable(t *testing.T) {
	accounts := []Account{
		{ID: 1, Name: "Codex Primary", Status: StatusActive, Schedulable: true},
		{ID: 2, Name: "codex stopped", Status: StatusActive, Schedulable: false},
		{ID: 3, Name: "Other", Status: StatusActive, Schedulable: true},
		{ID: 4, Name: "CODEX Error", Status: StatusError, Schedulable: true},
	}

	filtered := filterScheduledGroupTestAccounts(accounts, "codex")
	gotIDs := accountIDs(filtered)

	if !reflect.DeepEqual(gotIDs, []int64{1}) {
		t.Fatalf("filtered IDs = %v, want [1]", gotIDs)
	}
}

func TestFilterScheduledGroupTestAccountsEmptyFilterUsesWholeSchedulableGroup(t *testing.T) {
	accounts := []Account{
		{ID: 1, Name: "A", Status: StatusActive, Schedulable: true},
		{ID: 2, Name: "B", Status: StatusActive, Schedulable: false},
		{ID: 3, Name: "C", Status: StatusActive, Schedulable: true},
	}

	filtered := filterScheduledGroupTestAccounts(accounts, "")
	gotIDs := accountIDs(filtered)

	if !reflect.DeepEqual(gotIDs, []int64{1, 3}) {
		t.Fatalf("filtered IDs = %v, want [1 3]", gotIDs)
	}
}

func TestScheduledGroupTestRunnerRunOnePlanTestsMatchedAccountsAndContinuesAfterFailure(t *testing.T) {
	planRepo := &scheduledGroupPlanRepoFake{}
	accountRepo := &scheduledGroupAccountRepoFake{
		accounts: []Account{
			{ID: 3, Name: "Codex C", Status: StatusActive, Schedulable: true},
			{ID: 1, Name: "Codex A", Status: StatusActive, Schedulable: true},
			{ID: 2, Name: "Other B", Status: StatusActive, Schedulable: true},
			{ID: 4, Name: "Codex stopped", Status: StatusActive, Schedulable: false},
		},
	}
	tester := &scheduledGroupTesterFake{
		results: map[int64]*ScheduledTestResult{
			1: {Status: "success", LatencyMs: 11},
			3: {Status: "failed", LatencyMs: 12, ErrorMessage: "boom"},
		},
		errs: map[int64]error{
			3: errors.New("boom"),
		},
	}
	runner := NewScheduledGroupTestRunnerService(planRepo, accountRepo, tester, nil)
	runner.rand = rand.New(rand.NewSource(1))

	runner.runOnePlan(context.Background(), &ScheduledGroupTestPlan{
		ID:                9,
		GroupID:           7,
		AccountNameFilter: "codex",
	})

	if accountRepo.groupID != 7 {
		t.Fatalf("ListSchedulableByGroupID groupID = %d, want 7", accountRepo.groupID)
	}
	gotCallIDs := make([]int64, 0, len(tester.calls))
	for _, call := range tester.calls {
		gotCallIDs = append(gotCallIDs, call.accountID)
		if call.modelID != ScheduledGroupTestModelID {
			t.Fatalf("model ID = %q, want %q", call.modelID, ScheduledGroupTestModelID)
		}
	}
	sort.Slice(gotCallIDs, func(i, j int) bool { return gotCallIDs[i] < gotCallIDs[j] })
	if !reflect.DeepEqual(gotCallIDs, []int64{1, 3}) {
		t.Fatalf("tested account IDs = %v, want [1 3]", gotCallIDs)
	}
	if planRepo.afterRunID != 9 {
		t.Fatalf("UpdateAfterRun id = %d, want 9", planRepo.afterRunID)
	}
	delta := planRepo.afterRunNext.Sub(planRepo.afterRunLast)
	if delta < 55*time.Minute || delta > 65*time.Minute {
		t.Fatalf("next run delta = %s, want 55-65m", delta)
	}
}

func accountIDs(accounts []Account) []int64 {
	ids := make([]int64, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.ID)
	}
	return ids
}
