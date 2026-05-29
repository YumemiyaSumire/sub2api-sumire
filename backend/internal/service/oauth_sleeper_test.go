package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type oauthSleeperRepoStub struct {
	accounts []Account

	mu        sync.Mutex
	groups    []OAuthSleeperGroup
	events    []OAuthSleeperEvent
	updates   []int64
	existing  map[int64]*time.Time
	listCalls int
	updateErr error
	eventErr  error
}

func (r *oauthSleeperRepoStub) GetOAuthSleeperAccount(_ context.Context, accountID int64) (*Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, account := range r.accounts {
		if account.ID == accountID {
			cp := account
			return &cp, nil
		}
	}
	return nil, nil
}

func (r *oauthSleeperRepoStub) ListOAuthSleeperAccounts(context.Context, []string, []int64) ([]Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listCalls++
	return append([]Account(nil), r.accounts...), nil
}

func (r *oauthSleeperRepoStub) ListOAuthSleeperGroups(_ context.Context, groupIDs []int64) ([]OAuthSleeperGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(groupIDs) == 0 {
		return []OAuthSleeperGroup{}, nil
	}
	allowed := map[int64]struct{}{}
	for _, id := range groupIDs {
		allowed[id] = struct{}{}
	}
	groups := make([]OAuthSleeperGroup, 0, len(r.groups))
	for _, group := range r.groups {
		if _, ok := allowed[group.ID]; ok {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

func (r *oauthSleeperRepoStub) CreateOAuthSleeperEventAfterRateLimit(_ context.Context, event *OAuthSleeperEvent) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return false, r.updateErr
	}
	if r.eventErr != nil {
		return false, r.eventErr
	}
	if r.existing == nil {
		r.existing = map[int64]*time.Time{}
	}
	previous := r.existing[event.AccountID]
	if previous != nil && !previous.Before(event.ResetAt) {
		return false, nil
	}
	event.PreviousRateLimitResetAt = previous
	cp := event.ResetAt
	r.existing[event.AccountID] = &cp
	r.updates = append(r.updates, event.AccountID)
	r.events = append(r.events, *event)
	return true, nil
}

func (r *oauthSleeperRepoStub) ListOAuthSleeperEvents(context.Context, pagination.PaginationParams) ([]OAuthSleeperEvent, *pagination.PaginationResult, error) {
	return append([]OAuthSleeperEvent(nil), r.events...), &pagination.PaginationResult{Total: int64(len(r.events)), Page: 1, PageSize: 20, Pages: 1}, nil
}

func (r *oauthSleeperRepoStub) ListOAuthSleeperSleepingAccounts(context.Context, []string, []int64, time.Time, int) ([]OAuthSleeperSleepingAccount, error) {
	return []OAuthSleeperSleepingAccount{}, nil
}

type oauthSleeperSettingRepoStub struct {
	data map[string]string
}

func (r *oauthSleeperSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *oauthSleeperSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if r.data == nil {
		return "", ErrSettingNotFound
	}
	v, ok := r.data[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return v, nil
}

func (r *oauthSleeperSettingRepoStub) Set(_ context.Context, key, value string) error {
	if r.data == nil {
		r.data = map[string]string{}
	}
	r.data[key] = value
	return nil
}

func (r *oauthSleeperSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *oauthSleeperSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	if r.data == nil {
		r.data = map[string]string{}
	}
	for k, v := range settings {
		r.data[k] = v
	}
	return nil
}

func (r *oauthSleeperSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *oauthSleeperSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(r.data, key)
	return nil
}

func oauthSleeperSettingRepoWithSettings(t *testing.T, settings OAuthSleeperSettings) *oauthSleeperSettingRepoStub {
	t.Helper()
	data, err := json.Marshal(settings)
	require.NoError(t, err)
	return &oauthSleeperSettingRepoStub{data: map[string]string{SettingKeyOAuthSleeperSettings: string(data)}}
}

func TestOAuthSleeperDefaultThresholdIs90(t *testing.T) {
	settings := DefaultOAuthSleeperSettings()
	require.Equal(t, 90.0, settings.ThresholdPercent)
	require.Empty(t, settings.GroupThresholdPercent)
}

func TestOAuthSleeperKeepsExplicitLegacyThreshold(t *testing.T) {
	settings := *DefaultOAuthSleeperSettings()
	settings.ThresholdPercent = 95
	settings.GroupIDs = []int64{1}
	repo := oauthSleeperSettingRepoWithSettings(t, settings)
	svc := NewOAuthSleeperService(&oauthSleeperRepoStub{}, repo)

	got, err := svc.GetSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 95.0, got.ThresholdPercent)
}

func TestOAuthSleeperEvaluateOpenAIWindows(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	reset5h := now.Add(3 * time.Hour)
	reset7d := now.Add(24 * time.Hour)

	account := Account{
		ID:       1,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Status:   StatusActive,
		Extra: map[string]any{
			"codex_5h_used_percent": 96.0,
			"codex_5h_reset_at":     reset5h.Format(time.RFC3339),
			"codex_7d_used_percent": json.Number("97.5"),
			"codex_7d_reset_at":     reset7d.Unix(),
		},
	}

	candidate, ok := evaluateOAuthSleeperAccount(account, *DefaultOAuthSleeperSettings(), now)
	require.True(t, ok)
	require.Equal(t, "codex_7d", candidate.window)
	require.Equal(t, 97.5, candidate.utilizationPercent)
	require.Equal(t, reset7d, candidate.resetAt)
}

func TestOAuthSleeperEvaluateSkipsBelowThresholdAndExpiredReset(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)

	for _, account := range []Account{
		{
			ID:       1,
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Status:   StatusActive,
			Extra: map[string]any{
				"codex_5h_used_percent": 89.9,
				"codex_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
			},
		},
		{
			ID:       2,
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Status:   StatusActive,
			Extra: map[string]any{
				"codex_5h_used_percent": 99.0,
				"codex_5h_reset_at":     now.Add(-time.Minute).Format(time.RFC3339),
			},
		},
	} {
		_, ok := evaluateOAuthSleeperAccount(account, *DefaultOAuthSleeperSettings(), now)
		require.False(t, ok)
	}
}

func TestOAuthSleeperEvaluateAnthropicFraction(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	sessionReset := now.Add(2 * time.Hour)
	passiveReset := now.Add(4 * time.Hour)

	account := Account{
		ID:               2,
		Name:             "claude-oauth",
		Platform:         PlatformAnthropic,
		Type:             AccountTypeOAuth,
		Status:           StatusActive,
		SessionWindowEnd: &sessionReset,
		Extra: map[string]any{
			"session_window_utilization":   0.98,
			"passive_usage_7d_utilization": "0.96",
			"passive_usage_7d_reset":       passiveReset.Format(time.RFC3339Nano),
		},
	}

	candidate, ok := evaluateOAuthSleeperAccount(account, *DefaultOAuthSleeperSettings(), now)
	require.True(t, ok)
	require.Equal(t, "passive_usage_7d", candidate.window)
	require.Equal(t, 96.0, candidate.utilizationPercent)
	require.Equal(t, passiveReset, candidate.resetAt)
}

func TestOAuthSleeperScanUsesLowestSelectedGroupThreshold(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	repo := &oauthSleeperRepoStub{
		groups: []OAuthSleeperGroup{
			{ID: 1, Name: "OpenAI A", Platform: PlatformOpenAI},
			{ID: 2, Name: "OpenAI B", Platform: PlatformOpenAI},
		},
		accounts: []Account{withGroupIDs(openAISleeperAccount(1, 89, now.Add(time.Hour)), 1, 2)},
	}
	svc := NewOAuthSleeperService(repo, &oauthSleeperSettingRepoStub{})
	svc.now = func() time.Time { return now }
	settings := *DefaultOAuthSleeperSettings()
	settings.IncludeAnthropic = false
	settings.GroupIDs = []int64{1, 2}
	settings.GroupThresholdPercent = map[int64]float64{1: 92, 2: 88}

	result, err := svc.runScan(context.Background(), settings)
	require.NoError(t, err)
	require.Equal(t, 1, result.Scanned)
	require.Equal(t, 1, result.Triggered)
	require.Len(t, result.Events, 1)
	require.Equal(t, 88.0, result.Events[0].ThresholdPercent)
}

func TestOAuthSleeperScanCapsPerPlatformAndSortsCandidates(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	repo := &oauthSleeperRepoStub{
		groups: []OAuthSleeperGroup{
			{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI},
			{ID: 2, Name: "Anthropic", Platform: PlatformAnthropic},
		},
		accounts: []Account{
			openAISleeperAccount(1, 98, now.Add(1*time.Hour)),
			openAISleeperAccount(2, 99, now.Add(2*time.Hour)),
			openAISleeperAccount(3, 97, now.Add(3*time.Hour)),
			anthropicSleeperAccount(10, 0.96, now.Add(4*time.Hour)),
			anthropicSleeperAccount(11, 0.99, now.Add(5*time.Hour)),
		},
	}
	svc := NewOAuthSleeperService(repo, &oauthSleeperSettingRepoStub{})
	svc.now = func() time.Time { return now }
	settings := *DefaultOAuthSleeperSettings()
	settings.GroupIDs = []int64{1, 2}

	result, err := svc.runScan(context.Background(), settings)
	require.NoError(t, err)
	require.Equal(t, 5, result.Scanned)
	require.Equal(t, 5, result.Triggered)
	require.Equal(t, []int64{2, 1, 3, 11, 10}, repo.updates)
}

func TestOAuthSleeperScanDoesNotRecordEventWhenExistingResetIsLater(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	later := now.Add(24 * time.Hour)
	repo := &oauthSleeperRepoStub{
		accounts: []Account{openAISleeperAccount(1, 99, now.Add(time.Hour))},
		existing: map[int64]*time.Time{1: &later},
		groups:   []OAuthSleeperGroup{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI}},
	}
	svc := NewOAuthSleeperService(repo, &oauthSleeperSettingRepoStub{})
	svc.now = func() time.Time { return now }

	settings := *DefaultOAuthSleeperSettings()
	settings.GroupIDs = []int64{1}
	result, err := svc.runScan(context.Background(), settings)
	require.NoError(t, err)
	require.Equal(t, 1, result.Scanned)
	require.Equal(t, 0, result.Triggered)
	require.Empty(t, repo.events)
	require.Empty(t, repo.updates)
}

func TestOAuthSleeperSnapshotIgnoresDisabledSetting(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	settings := *DefaultOAuthSleeperSettings()
	settings.Enabled = false
	settings.GroupIDs = []int64{1}
	data, err := json.Marshal(settings)
	require.NoError(t, err)

	repo := &oauthSleeperRepoStub{
		accounts: []Account{openAISleeperAccount(1, 99, now.Add(time.Hour))},
		groups:   []OAuthSleeperGroup{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI}},
	}
	svc := NewOAuthSleeperService(repo, &oauthSleeperSettingRepoStub{data: map[string]string{SettingKeyOAuthSleeperSettings: string(data)}})
	svc.now = func() time.Time { return now }

	groupID := int64(1)
	svc.ObserveUsageLogInserted(&UsageLog{AccountID: 1, GroupID: &groupID})
	require.Empty(t, repo.events)
}

func TestOAuthSleeperBackgroundLoopSkipsWhenDisabled(t *testing.T) {
	settings := *DefaultOAuthSleeperSettings()
	settings.Enabled = false
	data, err := json.Marshal(settings)
	require.NoError(t, err)

	repo := &oauthSleeperRepoStub{
		accounts: []Account{openAISleeperAccount(1, 99, time.Now().Add(time.Hour))},
		groups:   []OAuthSleeperGroup{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI}},
	}
	svc := NewOAuthSleeperService(repo, &oauthSleeperSettingRepoStub{
		data: map[string]string{SettingKeyOAuthSleeperSettings: string(data)},
	})

	svc.Start()
	svc.Stop()

	repo.mu.Lock()
	defer repo.mu.Unlock()
	require.Equal(t, 0, repo.listCalls)
	require.Empty(t, repo.events)
	require.Empty(t, repo.updates)
}

func TestOAuthSleeperAtomicPathDoesNotRecordEventOnEventFailure(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	repo := &oauthSleeperRepoStub{
		accounts: []Account{openAISleeperAccount(1, 99, now.Add(time.Hour))},
		eventErr: errors.New("insert failed"),
		groups:   []OAuthSleeperGroup{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI}},
	}
	svc := NewOAuthSleeperService(repo, &oauthSleeperSettingRepoStub{})
	svc.now = func() time.Time { return now }

	settings := *DefaultOAuthSleeperSettings()
	settings.GroupIDs = []int64{1}
	_, err := svc.runScan(context.Background(), settings)
	require.Error(t, err)
	require.Empty(t, repo.events)
}

func TestOAuthSleeperSetSettingsRequiresGroupsWhenEnabled(t *testing.T) {
	svc := NewOAuthSleeperService(&oauthSleeperRepoStub{}, &oauthSleeperSettingRepoStub{})
	settings := *DefaultOAuthSleeperSettings()
	settings.Enabled = true
	settings.GroupIDs = nil

	_, err := svc.SetSettings(context.Background(), &settings)
	require.ErrorIs(t, err, ErrOAuthSleeperInvalidSettings)
}

func TestOAuthSleeperSetSettingsRejectsDisabledPlatformGroup(t *testing.T) {
	repo := &oauthSleeperRepoStub{
		groups: []OAuthSleeperGroup{{ID: 2, Name: "Anthropic", Platform: PlatformAnthropic}},
	}
	svc := NewOAuthSleeperService(repo, &oauthSleeperSettingRepoStub{})
	settings := *DefaultOAuthSleeperSettings()
	settings.Enabled = true
	settings.IncludeAnthropic = false
	settings.GroupIDs = []int64{2}

	_, err := svc.SetSettings(context.Background(), &settings)
	require.ErrorIs(t, err, ErrOAuthSleeperInvalidSettings)
}

func TestOAuthSleeperSetSettingsRejectsInvalidGroupThreshold(t *testing.T) {
	repo := &oauthSleeperRepoStub{
		groups: []OAuthSleeperGroup{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI}},
	}
	svc := NewOAuthSleeperService(repo, &oauthSleeperSettingRepoStub{})
	settings := *DefaultOAuthSleeperSettings()
	settings.Enabled = true
	settings.GroupIDs = []int64{1}
	settings.GroupThresholdPercent = map[int64]float64{1: 101}

	_, err := svc.SetSettings(context.Background(), &settings)
	require.ErrorIs(t, err, ErrOAuthSleeperInvalidSettings)
}

func TestOAuthSleeperSetSettingsRejectsUnselectedGroupThreshold(t *testing.T) {
	repo := &oauthSleeperRepoStub{
		groups: []OAuthSleeperGroup{{ID: 1, Name: "OpenAI", Platform: PlatformOpenAI}},
	}
	svc := NewOAuthSleeperService(repo, &oauthSleeperSettingRepoStub{})
	settings := *DefaultOAuthSleeperSettings()
	settings.Enabled = true
	settings.GroupIDs = []int64{1}
	settings.GroupThresholdPercent = map[int64]float64{2: 85}

	_, err := svc.SetSettings(context.Background(), &settings)
	require.ErrorIs(t, err, ErrOAuthSleeperInvalidSettings)
}

func TestOAuthSleeperLegacyConfigWithoutGroupsScansNothing(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	repo := &oauthSleeperRepoStub{accounts: []Account{openAISleeperAccount(1, 99, now.Add(time.Hour))}}
	svc := NewOAuthSleeperService(repo, &oauthSleeperSettingRepoStub{})
	svc.now = func() time.Time { return now }

	settings := *DefaultOAuthSleeperSettings()
	settings.Enabled = true
	settings.GroupIDs = nil
	result, err := svc.runScan(context.Background(), settings)

	require.NoError(t, err)
	require.Equal(t, 0, result.Scanned)
	require.Equal(t, 0, result.Triggered)
	require.Empty(t, repo.events)
	require.Zero(t, repo.listCalls)
}

func TestOAuthSleeperScanCapsPerSelectedGroupWithInternalDefault(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	repo := &oauthSleeperRepoStub{
		groups: []OAuthSleeperGroup{
			{ID: 1, Name: "OpenAI A", Platform: PlatformOpenAI},
			{ID: 2, Name: "OpenAI B", Platform: PlatformOpenAI},
		},
		accounts: []Account{
			withGroupIDs(openAISleeperAccount(1, 99, now.Add(6*time.Hour)), 1),
			withGroupIDs(openAISleeperAccount(2, 98, now.Add(5*time.Hour)), 1),
			withGroupIDs(openAISleeperAccount(3, 97, now.Add(4*time.Hour)), 1),
			withGroupIDs(openAISleeperAccount(4, 96, now.Add(3*time.Hour)), 1),
			withGroupIDs(openAISleeperAccount(5, 95, now.Add(2*time.Hour)), 2),
			withGroupIDs(openAISleeperAccount(6, 94, now.Add(1*time.Hour)), 2),
		},
	}
	svc := NewOAuthSleeperService(repo, &oauthSleeperSettingRepoStub{})
	svc.now = func() time.Time { return now }
	settings := *DefaultOAuthSleeperSettings()
	settings.IncludeAnthropic = false
	settings.GroupIDs = []int64{1, 2}

	result, err := svc.runScan(context.Background(), settings)
	require.NoError(t, err)
	require.Equal(t, 6, result.Scanned)
	require.Equal(t, 5, result.Triggered)
	require.Equal(t, []int64{1, 2, 3, 5, 6}, repo.updates)
}

func TestOAuthSleeperSnapshotTriggersSleepWhenThresholdReached(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	groupID := int64(1)
	repo := &oauthSleeperRepoStub{
		groups:   []OAuthSleeperGroup{{ID: groupID, Name: "OpenAI", Platform: PlatformOpenAI}},
		accounts: []Account{withGroupIDs(openAISleeperAccount(1, 90, now.Add(time.Hour)), groupID)},
	}
	settings := *DefaultOAuthSleeperSettings()
	settings.Enabled = true
	settings.GroupIDs = []int64{groupID}
	svc := NewOAuthSleeperService(repo, oauthSleeperSettingRepoWithSettings(t, settings))
	svc.now = func() time.Time { return now }

	svc.ObserveAccountUsageSnapshotUpdated(1)

	require.Equal(t, []int64{1}, repo.updates)
	require.Len(t, repo.events, 1)
	require.Equal(t, 90.0, repo.events[0].ThresholdPercent)
	require.Equal(t, "codex_5h", repo.events[0].Window)
}

func TestOAuthSleeperSnapshotUsesGroupThresholdAndDeduplicates(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	groupID := int64(1)
	resetAt := now.Add(time.Hour)
	repo := &oauthSleeperRepoStub{
		groups:   []OAuthSleeperGroup{{ID: groupID, Name: "OpenAI", Platform: PlatformOpenAI}},
		accounts: []Account{withGroupIDs(openAISleeperAccount(1, 88, resetAt), groupID)},
	}
	settings := *DefaultOAuthSleeperSettings()
	settings.Enabled = true
	settings.GroupIDs = []int64{groupID}
	settings.GroupThresholdPercent = map[int64]float64{groupID: 87}
	svc := NewOAuthSleeperService(repo, oauthSleeperSettingRepoWithSettings(t, settings))
	svc.now = func() time.Time { return now }

	svc.ObserveAccountUsageSnapshotUpdated(1)
	svc.ObserveAccountUsageSnapshotUpdated(1)

	require.Equal(t, []int64{1}, repo.updates)
	require.Len(t, repo.events, 1)
	require.Equal(t, 87.0, repo.events[0].ThresholdPercent)
}

func TestOAuthSleeperSnapshotIsScopedToSelectedGroups(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	group1 := int64(1)
	group2 := int64(2)
	repo := &oauthSleeperRepoStub{
		groups: []OAuthSleeperGroup{
			{ID: group1, Name: "OpenAI A", Platform: PlatformOpenAI},
			{ID: group2, Name: "OpenAI B", Platform: PlatformOpenAI},
		},
		accounts: []Account{
			withGroupIDs(openAISleeperAccount(1, 90, now.Add(time.Hour)), group2),
		},
	}
	settings := *DefaultOAuthSleeperSettings()
	settings.Enabled = true
	settings.GroupIDs = []int64{group1}
	svc := NewOAuthSleeperService(repo, oauthSleeperSettingRepoWithSettings(t, settings))
	svc.now = func() time.Time { return now }

	svc.ObserveUsageLogInserted(&UsageLog{AccountID: 1, GroupID: &group2})
	require.Empty(t, repo.events)

	settings.GroupIDs = []int64{group1, group2}
	svc = NewOAuthSleeperService(repo, oauthSleeperSettingRepoWithSettings(t, settings))
	svc.now = func() time.Time { return now }
	svc.ObserveUsageLogInserted(&UsageLog{AccountID: 1, GroupID: &group2})
	require.Equal(t, []int64{1}, repo.updates)
}

func TestOAuthSleeperSnapshotIgnoresBelowThresholdAndMissingGroup(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	groupID := int64(1)
	repo := &oauthSleeperRepoStub{
		groups:   []OAuthSleeperGroup{{ID: groupID, Name: "OpenAI", Platform: PlatformOpenAI}},
		accounts: []Account{withGroupIDs(openAISleeperAccount(1, 89.9, now.Add(time.Hour)), groupID)},
	}
	settings := *DefaultOAuthSleeperSettings()
	settings.Enabled = true
	settings.GroupIDs = []int64{groupID}
	svc := NewOAuthSleeperService(repo, oauthSleeperSettingRepoWithSettings(t, settings))
	svc.now = func() time.Time { return now }

	svc.ObserveUsageLogInserted(&UsageLog{AccountID: 1, GroupID: &groupID})
	svc.ObserveUsageLogInserted(&UsageLog{AccountID: 1, GroupID: nil})
	require.Empty(t, repo.events)
}

func openAISleeperAccount(id int64, utilization float64, resetAt time.Time) Account {
	return Account{
		ID:       id,
		Name:     "openai",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Status:   StatusActive,
		GroupIDs: []int64{1},
		Extra: map[string]any{
			"codex_5h_used_percent": utilization,
			"codex_5h_reset_at":     resetAt.Format(time.RFC3339),
		},
	}
}

func anthropicSleeperAccount(id int64, utilization float64, resetAt time.Time) Account {
	return Account{
		ID:               id,
		Name:             "anthropic",
		Platform:         PlatformAnthropic,
		Type:             AccountTypeOAuth,
		Status:           StatusActive,
		GroupIDs:         []int64{2},
		SessionWindowEnd: &resetAt,
		Extra: map[string]any{
			"session_window_utilization": utilization,
		},
	}
}

func withGroupIDs(account Account, groupIDs ...int64) Account {
	account.GroupIDs = append([]int64(nil), groupIDs...)
	return account
}
