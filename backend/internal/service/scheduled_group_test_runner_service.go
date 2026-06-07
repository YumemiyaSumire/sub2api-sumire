package service

import (
	"context"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/robfig/cron/v3"
	"golang.org/x/sync/errgroup"
)

const (
	scheduledGroupTestMaxAccountWorkers = 5
	scheduledGroupTestMaxPlanWorkers    = 5
)

// ScheduledGroupTestRunnerService periodically scans due group plans and executes them.
type ScheduledGroupTestRunnerService struct {
	planRepo       ScheduledGroupTestPlanRepository
	accountRepo    ScheduledGroupAccountRepository
	accountTestSvc ScheduledGroupAccountTester
	cfg            *config.Config
	rand           *rand.Rand
	randMu         sync.Mutex

	cron      *cron.Cron
	startOnce sync.Once
	stopOnce  sync.Once
}

func NewScheduledGroupTestRunnerService(
	planRepo ScheduledGroupTestPlanRepository,
	accountRepo ScheduledGroupAccountRepository,
	accountTestSvc ScheduledGroupAccountTester,
	cfg *config.Config,
) *ScheduledGroupTestRunnerService {
	return &ScheduledGroupTestRunnerService{
		planRepo:       planRepo,
		accountRepo:    accountRepo,
		accountTestSvc: accountTestSvc,
		cfg:            cfg,
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *ScheduledGroupTestRunnerService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		loc := time.Local
		if s.cfg != nil {
			if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}

		c := cron.New(cron.WithParser(scheduledTestCronParser), cron.WithLocation(loc))
		_, err := c.AddFunc("* * * * *", func() { s.runScheduled() })
		if err != nil {
			logger.LegacyPrintf("service.scheduled_group_test_runner", "[ScheduledGroupTestRunner] not started (invalid schedule): %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
		logger.LegacyPrintf("service.scheduled_group_test_runner", "[ScheduledGroupTestRunner] started (tick=every minute)")
	})
}

func (s *ScheduledGroupTestRunnerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf("service.scheduled_group_test_runner", "[ScheduledGroupTestRunner] cron stop timed out")
			}
		}
	})
}

func (s *ScheduledGroupTestRunnerService) runScheduled() {
	time.Sleep(15 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	plans, err := s.planRepo.ListDue(ctx, time.Now())
	if err != nil {
		logger.LegacyPrintf("service.scheduled_group_test_runner", "[ScheduledGroupTestRunner] ListDue error: %v", err)
		return
	}
	if len(plans) == 0 {
		return
	}

	logger.LegacyPrintf("service.scheduled_group_test_runner", "[ScheduledGroupTestRunner] found %d due plans", len(plans))

	sem := make(chan struct{}, scheduledGroupTestMaxPlanWorkers)
	var wg sync.WaitGroup
	for _, plan := range plans {
		sem <- struct{}{}
		wg.Add(1)
		go func(p *ScheduledGroupTestPlan) {
			defer wg.Done()
			defer func() { <-sem }()
			s.runOnePlan(ctx, p)
		}(plan)
	}
	wg.Wait()
}

func (s *ScheduledGroupTestRunnerService) runOnePlan(ctx context.Context, plan *ScheduledGroupTestPlan) {
	if s == nil || s.planRepo == nil || s.accountRepo == nil || s.accountTestSvc == nil || plan == nil {
		return
	}

	startedAt := time.Now()
	accounts, err := s.accountRepo.ListSchedulableByGroupID(ctx, plan.GroupID)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_group_test_runner", "[ScheduledGroupTestRunner] plan=%d group=%d ListSchedulableByGroupID error: %v", plan.ID, plan.GroupID, err)
		return
	}
	accounts = filterScheduledGroupTestAccounts(accounts, plan.AccountNameFilter)
	sort.SliceStable(accounts, func(i, j int) bool {
		left := strings.ToLower(accounts[i].Name)
		right := strings.ToLower(accounts[j].Name)
		if left == right {
			return accounts[i].ID < accounts[j].ID
		}
		return left < right
	})

	total := len(accounts)
	success := 0
	failed := 0

	if total > 0 {
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(scheduledGroupTestMaxAccountWorkers)
		var mu sync.Mutex
		for _, account := range accounts {
			account := account
			g.Go(func() error {
				result, testErr := s.accountTestSvc.RunTestBackground(gctx, account.ID, ScheduledGroupTestModelID)
				status := "failed"
				errorMessage := ""
				latencyMs := int64(0)
				if result != nil {
					status = result.Status
					errorMessage = result.ErrorMessage
					latencyMs = result.LatencyMs
				}
				if testErr != nil && errorMessage == "" {
					errorMessage = testErr.Error()
				}
				if status != "success" && errorMessage == "" {
					errorMessage = "account test failed"
				}

				mu.Lock()
				if status == "success" {
					success++
				} else {
					failed++
					logger.LegacyPrintf("service.scheduled_group_test_runner", "[ScheduledGroupTestRunner] plan=%d group=%d account=%d name=%q failed latency_ms=%d error=%s", plan.ID, plan.GroupID, account.ID, account.Name, latencyMs, errorMessage)
				}
				mu.Unlock()
				return nil
			})
		}
		_ = g.Wait()
	}

	nextRun := s.nextRunAfter(startedAt)
	if err := s.planRepo.UpdateAfterRun(ctx, plan.ID, startedAt, nextRun); err != nil {
		logger.LegacyPrintf("service.scheduled_group_test_runner", "[ScheduledGroupTestRunner] plan=%d UpdateAfterRun error: %v", plan.ID, err)
	}

	logger.LegacyPrintf("service.scheduled_group_test_runner", "[ScheduledGroupTestRunner] plan=%d group=%d filter=%q model=%s total=%d success=%d failed=%d next_run_at=%s", plan.ID, plan.GroupID, plan.AccountNameFilter, ScheduledGroupTestModelID, total, success, failed, nextRun.Format(time.RFC3339))
}

func filterScheduledGroupTestAccounts(accounts []Account, nameFilter string) []Account {
	filter := strings.ToLower(strings.TrimSpace(nameFilter))
	out := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if !account.IsSchedulable() {
			continue
		}
		if filter == "" || strings.Contains(strings.ToLower(account.Name), filter) {
			out = append(out, account)
		}
	}
	return out
}

func (s *ScheduledGroupTestRunnerService) nextRunAfter(from time.Time) time.Time {
	if s == nil || s.rand == nil {
		return from.Add(55 * time.Minute)
	}
	// Inclusive range: 55-65 minutes.
	s.randMu.Lock()
	offsetMinutes := 55 + s.rand.Intn(11)
	s.randMu.Unlock()
	return from.Add(time.Duration(offsetMinutes) * time.Minute)
}
