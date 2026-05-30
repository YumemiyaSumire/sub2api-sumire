package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	defaultOAuthSleeperThresholdPercent   = 90
	defaultOAuthSleeperScanIntervalSecond = 300
	defaultOAuthSleeperMaxSleepPerScan    = 3

	oauthSleeperMinThresholdPercent = 1
	oauthSleeperMaxThresholdPercent = 100

	oauthSleeperStatusAccountsLimit = 20

	OAuthSleeperExtraSleepKey       = "oauth_sleeper_sleep"
	OAuthSleeperExtraStickyGraceKey = "oauth_sleeper_sticky_grace_until"
	OAuthSleeperExtraResetAtKey     = "oauth_sleeper_reset_at"
	OAuthSleeperStickyGraceDuration = 30 * time.Second
)

var ErrOAuthSleeperInvalidSettings = infraerrors.BadRequest("OAUTH_SLEEPER_INVALID_SETTINGS", "invalid oauth sleeper settings")

// OAuthSleeperRepository is the narrow persistence surface used by OAuthSleeperService.
type OAuthSleeperRepository interface {
	GetOAuthSleeperAccount(ctx context.Context, accountID int64) (*Account, error)
	ListOAuthSleeperAccounts(ctx context.Context, platforms []string, groupIDs []int64) ([]Account, error)
	ListOAuthSleeperGroups(ctx context.Context, groupIDs []int64) ([]OAuthSleeperGroup, error)
	CreateOAuthSleeperEventAfterRateLimit(ctx context.Context, event *OAuthSleeperEvent) (bool, error)
	ListOAuthSleeperEvents(ctx context.Context, params pagination.PaginationParams) ([]OAuthSleeperEvent, *pagination.PaginationResult, error)
	ListOAuthSleeperSleepingAccounts(ctx context.Context, platforms []string, groupIDs []int64, now time.Time, limit int) ([]OAuthSleeperSleepingAccount, error)
}

type OAuthSleeperSettings struct {
	Enabled               bool              `json:"enabled"`
	ThresholdPercent      float64           `json:"threshold_percent"`
	GroupThresholdPercent map[int64]float64 `json:"group_threshold_percent"`
	IncludeOpenAI         bool              `json:"include_openai"`
	IncludeAnthropic      bool              `json:"include_anthropic"`
	GroupIDs              []int64           `json:"group_ids"`
}

type OAuthSleeperStatus struct {
	Enabled               bool                          `json:"enabled"`
	ThresholdPercent      float64                       `json:"threshold_percent"`
	GroupThresholdPercent map[int64]float64             `json:"group_threshold_percent"`
	IncludeOpenAI         bool                          `json:"include_openai"`
	IncludeAnthropic      bool                          `json:"include_anthropic"`
	GroupIDs              []int64                       `json:"group_ids"`
	LastScanAt            *time.Time                    `json:"last_scan_at,omitempty"`
	LastScanned           int                           `json:"last_scanned"`
	LastTriggered         int                           `json:"last_triggered"`
	LastError             string                        `json:"last_error,omitempty"`
	SleepingAccounts      []OAuthSleeperSleepingAccount `json:"sleeping_accounts"`
}

type OAuthSleeperScanResult struct {
	Scanned   int                 `json:"scanned"`
	Triggered int                 `json:"triggered"`
	Events    []OAuthSleeperEvent `json:"events"`
}

type OAuthSleeperEvent struct {
	ID                       int64      `json:"id"`
	AccountID                int64      `json:"account_id"`
	AccountName              string     `json:"account_name"`
	Platform                 string     `json:"platform"`
	Window                   string     `json:"window"`
	UtilizationPercent       float64    `json:"utilization_percent"`
	ThresholdPercent         float64    `json:"threshold_percent"`
	ResetAt                  time.Time  `json:"reset_at"`
	PreviousRateLimitResetAt *time.Time `json:"previous_rate_limit_reset_at,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
}

type OAuthSleeperGroup struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

type OAuthSleeperSleepingAccount struct {
	AccountID        int64     `json:"account_id"`
	AccountName      string    `json:"account_name"`
	Platform         string    `json:"platform"`
	RateLimitResetAt time.Time `json:"rate_limit_reset_at"`
	RemainingSeconds int64     `json:"remaining_seconds"`
}

type OAuthSleeperService struct {
	repo        OAuthSleeperRepository
	settingRepo SettingRepository

	now func() time.Time

	stopCh    chan struct{}
	stopOnce  sync.Once
	startOnce sync.Once
	wg        sync.WaitGroup

	scanMu sync.Mutex

	statusMu      sync.RWMutex
	lastScanAt    *time.Time
	lastScanned   int
	lastTriggered int
	lastError     string
}

type oauthSleeperCandidate struct {
	account            Account
	window             string
	utilizationPercent float64
	thresholdPercent   float64
	resetAt            time.Time
}

func DefaultOAuthSleeperSettings() *OAuthSleeperSettings {
	return &OAuthSleeperSettings{
		Enabled:               false,
		ThresholdPercent:      defaultOAuthSleeperThresholdPercent,
		GroupThresholdPercent: map[int64]float64{},
		IncludeOpenAI:         true,
		IncludeAnthropic:      true,
		GroupIDs:              []int64{},
	}
}

func NewOAuthSleeperService(repo OAuthSleeperRepository, settingRepo SettingRepository) *OAuthSleeperService {
	return &OAuthSleeperService{
		repo:        repo,
		settingRepo: settingRepo,
		now:         func() time.Time { return time.Now().UTC() },
		stopCh:      make(chan struct{}),
	}
}

func (s *OAuthSleeperService) Start() {
	if s == nil || s.repo == nil || s.settingRepo == nil {
		return
	}
	s.startOnce.Do(func() {
		s.wg.Add(1)
		go s.loop()
	})
}

func (s *OAuthSleeperService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *OAuthSleeperService) GetSettings(ctx context.Context) (*OAuthSleeperSettings, error) {
	if s == nil || s.settingRepo == nil {
		return DefaultOAuthSleeperSettings(), nil
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyOAuthSleeperSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultOAuthSleeperSettings(), nil
		}
		return nil, fmt.Errorf("get oauth sleeper settings: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		return DefaultOAuthSleeperSettings(), nil
	}

	settings := DefaultOAuthSleeperSettings()
	if err := json.Unmarshal([]byte(value), settings); err != nil {
		slog.Warn("oauth_sleeper: invalid settings json, falling back to disabled defaults", "error", err)
		return DefaultOAuthSleeperSettings(), nil
	}
	normalizeOAuthSleeperSettingsForRead(settings)
	return settings, nil
}

func (s *OAuthSleeperService) SetSettings(ctx context.Context, settings *OAuthSleeperSettings) (*OAuthSleeperSettings, error) {
	if s == nil || s.settingRepo == nil {
		return nil, fmt.Errorf("oauth sleeper service is not initialized")
	}
	if settings == nil {
		return nil, ErrOAuthSleeperInvalidSettings.WithMetadata(map[string]string{"field": "settings", "reason": "required"})
	}
	normalized := *settings
	normalizeOAuthSleeperSettingsForRead(&normalized)
	if err := ValidateOAuthSleeperSettings(&normalized); err != nil {
		return nil, err
	}
	if err := s.validateOAuthSleeperSettingsScope(ctx, normalized); err != nil {
		return nil, err
	}
	data, err := json.Marshal(&normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal oauth sleeper settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyOAuthSleeperSettings, string(data)); err != nil {
		return nil, fmt.Errorf("set oauth sleeper settings: %w", err)
	}
	return &normalized, nil
}

func ValidateOAuthSleeperSettings(settings *OAuthSleeperSettings) error {
	if settings == nil {
		return ErrOAuthSleeperInvalidSettings.WithMetadata(map[string]string{"field": "settings", "reason": "required"})
	}
	if settings.ThresholdPercent < oauthSleeperMinThresholdPercent || settings.ThresholdPercent > oauthSleeperMaxThresholdPercent {
		return ErrOAuthSleeperInvalidSettings.WithMetadata(map[string]string{"field": "threshold_percent", "reason": "must be between 1 and 100"})
	}
	for groupID, threshold := range settings.GroupThresholdPercent {
		if groupID <= 0 {
			return ErrOAuthSleeperInvalidSettings.WithMetadata(map[string]string{"field": "group_threshold_percent", "reason": "group id must be positive"})
		}
		if threshold < oauthSleeperMinThresholdPercent || threshold > oauthSleeperMaxThresholdPercent {
			return ErrOAuthSleeperInvalidSettings.WithMetadata(map[string]string{"field": "group_threshold_percent", "reason": "must be between 1 and 100"})
		}
	}
	return nil
}

func (s *OAuthSleeperService) GetStatus(ctx context.Context) (*OAuthSleeperStatus, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}

	var sleeping []OAuthSleeperSleepingAccount
	if s != nil && s.repo != nil {
		sleeping, err = s.repo.ListOAuthSleeperSleepingAccounts(ctx, oauthSleeperPlatforms(*settings), settings.GroupIDs, s.now(), oauthSleeperStatusAccountsLimit)
		if err != nil {
			return nil, fmt.Errorf("list oauth sleeper sleeping accounts: %w", err)
		}
	}

	s.statusMu.RLock()
	var lastScanAt *time.Time
	if s.lastScanAt != nil {
		t := *s.lastScanAt
		lastScanAt = &t
	}
	lastScanned := s.lastScanned
	lastTriggered := s.lastTriggered
	lastError := s.lastError
	s.statusMu.RUnlock()
	status := &OAuthSleeperStatus{
		Enabled:               settings.Enabled,
		ThresholdPercent:      settings.ThresholdPercent,
		GroupThresholdPercent: copyOAuthSleeperGroupThresholds(settings.GroupThresholdPercent),
		IncludeOpenAI:         settings.IncludeOpenAI,
		IncludeAnthropic:      settings.IncludeAnthropic,
		GroupIDs:              append([]int64(nil), settings.GroupIDs...),
		LastScanAt:            lastScanAt,
		LastScanned:           lastScanned,
		LastTriggered:         lastTriggered,
		LastError:             lastError,
		SleepingAccounts:      sleeping,
	}
	return status, nil
}

func (s *OAuthSleeperService) ListEvents(ctx context.Context, params pagination.PaginationParams) ([]OAuthSleeperEvent, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, fmt.Errorf("oauth sleeper service is not initialized")
	}
	return s.repo.ListOAuthSleeperEvents(ctx, params)
}

func (s *OAuthSleeperService) loop() {
	defer s.wg.Done()
	for {
		settings, err := s.GetSettings(context.Background())
		if err != nil {
			s.recordScanStatus(0, 0, err)
		} else if settings.Enabled {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(defaultOAuthSleeperScanIntervalSecond)*time.Second)
			_, err := s.runScan(ctx, *settings)
			cancel()
			if err != nil {
				slog.Warn("oauth_sleeper: background scan failed", "error", err)
			}
		}

		interval := time.Duration(defaultOAuthSleeperScanIntervalSecond) * time.Second

		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
		case <-s.stopCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		}
	}
}

func (s *OAuthSleeperService) runScan(ctx context.Context, settings OAuthSleeperSettings) (*OAuthSleeperScanResult, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("oauth sleeper service is not initialized")
	}
	normalizeOAuthSleeperSettingsForRead(&settings)
	if err := ValidateOAuthSleeperSettings(&settings); err != nil {
		return nil, err
	}

	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	result := &OAuthSleeperScanResult{Events: []OAuthSleeperEvent{}}
	platforms := oauthSleeperPlatforms(settings)
	if len(platforms) == 0 {
		now := s.now()
		s.recordScanStatusAt(now, 0, 0, nil)
		return result, nil
	}
	groups, err := s.resolveOAuthSleeperGroups(ctx, settings)
	if err != nil {
		s.recordScanStatus(0, 0, err)
		return nil, fmt.Errorf("resolve oauth sleeper groups: %w", err)
	}
	groupIDs := oauthSleeperGroupIDs(groups)
	if len(groupIDs) == 0 {
		now := s.now()
		s.recordScanStatusAt(now, 0, 0, nil)
		return result, nil
	}
	settings.GroupIDs = groupIDs
	groupIDSet := int64Set(groupIDs)
	triggeredByGroup := make(map[int64]int, len(groupIDs))

	accounts, err := s.repo.ListOAuthSleeperAccounts(ctx, platforms, groupIDs)
	if err != nil {
		s.recordScanStatus(0, 0, err)
		return nil, fmt.Errorf("list oauth sleeper accounts: %w", err)
	}
	result.Scanned = len(accounts)

	now := s.now()
	candidates := make(map[string][]oauthSleeperCandidate)
	for _, account := range accounts {
		candidate, ok := evaluateOAuthSleeperAccount(account, settings, now)
		if !ok {
			continue
		}
		candidates[candidate.account.Platform] = append(candidates[candidate.account.Platform], candidate)
	}

	for _, platform := range platforms {
		platformCandidates := candidates[platform]
		if len(platformCandidates) == 0 {
			continue
		}
		sortOAuthSleeperCandidates(platformCandidates)
		for _, candidate := range platformCandidates {
			candidateGroupIDs := oauthSleeperSelectedAccountGroupIDs(candidate.account.GroupIDs, groupIDSet)
			if len(candidateGroupIDs) == 0 || oauthSleeperAnyGroupAtLimit(candidateGroupIDs, triggeredByGroup, defaultOAuthSleeperMaxSleepPerScan) {
				continue
			}
			event := OAuthSleeperEvent{
				AccountID:          candidate.account.ID,
				AccountName:        candidate.account.Name,
				Platform:           candidate.account.Platform,
				Window:             candidate.window,
				UtilizationPercent: candidate.utilizationPercent,
				ThresholdPercent:   candidate.thresholdPercent,
				ResetAt:            oauthSleeperRecoveryResetAt(candidate.account, candidate.resetAt, now),
			}
			updated, err := s.repo.CreateOAuthSleeperEventAfterRateLimit(ctx, &event)
			if err != nil {
				s.recordScanStatus(result.Scanned, result.Triggered, err)
				return nil, fmt.Errorf("create oauth sleeper event after rate limit: account=%d platform=%s: %w", candidate.account.ID, platform, err)
			}
			if !updated {
				continue
			}
			result.Triggered++
			for _, groupID := range candidateGroupIDs {
				triggeredByGroup[groupID]++
			}
			result.Events = append(result.Events, event)
		}
	}

	s.recordScanStatusAt(now, result.Scanned, result.Triggered, nil)
	return result, nil
}

func (s *OAuthSleeperService) ObserveUsageLogInserted(log *UsageLog) {
	if s == nil || s.repo == nil || log == nil || log.AccountID <= 0 || log.GroupID == nil || *log.GroupID <= 0 {
		return
	}
	s.observeAccountUsageSnapshotUpdated(log.AccountID, *log.GroupID)
}

func (s *OAuthSleeperService) ObserveAccountUsageSnapshotUpdated(accountID int64) {
	if s == nil || s.repo == nil || accountID <= 0 {
		return
	}
	s.observeAccountUsageSnapshotUpdated(accountID, 0)
}

func (s *OAuthSleeperService) observeAccountUsageSnapshotUpdated(accountID int64, sourceGroupID int64) {
	settings, err := s.GetSettings(context.Background())
	if err != nil || settings == nil || !settings.Enabled {
		if err != nil {
			slog.Debug("oauth_sleeper: skip automatic account evaluation after usage snapshot because settings could not be loaded", "error", err)
		}
		return
	}
	if sourceGroupID > 0 && !int64SetContains(settings.GroupIDs, sourceGroupID) {
		return
	}
	account, err := s.repo.GetOAuthSleeperAccount(context.Background(), accountID)
	if err != nil {
		slog.Debug("oauth_sleeper: skip automatic account evaluation after usage snapshot because account could not be loaded", "account_id", accountID, "error", err)
		return
	}
	if account == nil {
		return
	}
	groups, err := s.resolveOAuthSleeperGroups(context.Background(), *settings)
	if err != nil {
		slog.Debug("oauth_sleeper: skip automatic account evaluation after usage snapshot because groups could not be resolved", "account_id", accountID, "error", err)
		return
	}
	settings.GroupIDs = oauthSleeperGroupIDs(groups)
	if len(settings.GroupIDs) == 0 {
		return
	}
	selectedGroups := oauthSleeperSelectedAccountGroupIDs(account.GroupIDs, int64Set(settings.GroupIDs))
	if len(selectedGroups) == 0 || (sourceGroupID > 0 && !int64SetContains(selectedGroups, sourceGroupID)) {
		return
	}
	if _, err := s.runScanForAccount(context.Background(), *settings, *account); err != nil {
		slog.Debug("oauth_sleeper: automatic account evaluation after usage snapshot failed", "account_id", accountID, "error", err)
	}
}

func (s *OAuthSleeperService) runScanForAccount(ctx context.Context, settings OAuthSleeperSettings, account Account) (bool, error) {
	if s == nil || s.repo == nil {
		return false, fmt.Errorf("oauth sleeper service is not initialized")
	}
	normalizeOAuthSleeperSettingsForRead(&settings)
	if err := ValidateOAuthSleeperSettings(&settings); err != nil {
		return false, err
	}

	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	now := s.now()
	candidate, ok := evaluateOAuthSleeperAccount(account, settings, now)
	if !ok {
		s.recordScanStatusAt(now, 1, 0, nil)
		return false, nil
	}
	groupIDSet := int64Set(settings.GroupIDs)
	if len(oauthSleeperSelectedAccountGroupIDs(candidate.account.GroupIDs, groupIDSet)) == 0 {
		s.recordScanStatusAt(now, 1, 0, nil)
		return false, nil
	}
	event := OAuthSleeperEvent{
		AccountID:          candidate.account.ID,
		AccountName:        candidate.account.Name,
		Platform:           candidate.account.Platform,
		Window:             candidate.window,
		UtilizationPercent: candidate.utilizationPercent,
		ThresholdPercent:   candidate.thresholdPercent,
		ResetAt:            oauthSleeperRecoveryResetAt(candidate.account, candidate.resetAt, now),
	}
	updated, err := s.repo.CreateOAuthSleeperEventAfterRateLimit(ctx, &event)
	if err != nil {
		s.recordScanStatusAt(now, 1, 0, err)
		return false, fmt.Errorf("create oauth sleeper event after usage snapshot: account=%d platform=%s: %w", candidate.account.ID, candidate.account.Platform, err)
	}
	triggered := 0
	if updated {
		triggered = 1
	}
	s.recordScanStatusAt(now, 1, triggered, nil)
	return updated, nil
}

func (s *OAuthSleeperService) recordScanStatus(scanned, triggered int, err error) {
	s.recordScanStatusAt(s.now(), scanned, triggered, err)
}

func (s *OAuthSleeperService) recordScanStatusAt(now time.Time, scanned, triggered int, err error) {
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	scannedAt := now.UTC()
	s.lastScanAt = &scannedAt
	s.lastScanned = scanned
	s.lastTriggered = triggered
	if err != nil {
		s.lastError = err.Error()
	} else {
		s.lastError = ""
	}
}

func normalizeOAuthSleeperSettingsForRead(settings *OAuthSleeperSettings) {
	if settings == nil {
		return
	}
	if settings.ThresholdPercent <= 0 {
		settings.ThresholdPercent = defaultOAuthSleeperThresholdPercent
	}
	settings.GroupThresholdPercent = normalizeOAuthSleeperGroupThresholds(settings.GroupThresholdPercent)
	settings.GroupIDs = normalizeOAuthSleeperGroupIDs(settings.GroupIDs)
}

func (s *OAuthSleeperService) validateOAuthSleeperSettingsScope(ctx context.Context, settings OAuthSleeperSettings) error {
	if settings.Enabled && len(settings.GroupIDs) == 0 {
		return ErrOAuthSleeperInvalidSettings.WithMetadata(map[string]string{"field": "group_ids", "reason": "at least one group is required when enabled"})
	}
	if len(settings.GroupIDs) == 0 {
		return nil
	}
	if s == nil || s.repo == nil {
		return fmt.Errorf("oauth sleeper service is not initialized")
	}
	groups, err := s.repo.ListOAuthSleeperGroups(ctx, settings.GroupIDs)
	if err != nil {
		return fmt.Errorf("list oauth sleeper groups: %w", err)
	}
	byID := make(map[int64]OAuthSleeperGroup, len(groups))
	for _, group := range groups {
		byID[group.ID] = group
	}
	for _, groupID := range settings.GroupIDs {
		group, ok := byID[groupID]
		if !ok {
			return ErrOAuthSleeperInvalidSettings.WithMetadata(map[string]string{"field": "group_ids", "reason": "group not found or inactive"})
		}
		if !oauthSleeperGroupPlatformAllowed(group.Platform, settings) {
			return ErrOAuthSleeperInvalidSettings.WithMetadata(map[string]string{"field": "group_ids", "reason": "group platform is not enabled for oauth sleeper"})
		}
	}
	selected := int64Set(settings.GroupIDs)
	for groupID := range settings.GroupThresholdPercent {
		if _, ok := selected[groupID]; !ok {
			return ErrOAuthSleeperInvalidSettings.WithMetadata(map[string]string{"field": "group_threshold_percent", "reason": "group threshold must reference a selected group"})
		}
	}
	return nil
}

func (s *OAuthSleeperService) resolveOAuthSleeperGroups(ctx context.Context, settings OAuthSleeperSettings) ([]OAuthSleeperGroup, error) {
	if len(settings.GroupIDs) == 0 {
		return []OAuthSleeperGroup{}, nil
	}
	groups, err := s.repo.ListOAuthSleeperGroups(ctx, settings.GroupIDs)
	if err != nil {
		return nil, err
	}
	out := make([]OAuthSleeperGroup, 0, len(groups))
	for _, group := range groups {
		if oauthSleeperGroupPlatformAllowed(group.Platform, settings) {
			out = append(out, group)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func oauthSleeperGroupPlatformAllowed(platform string, settings OAuthSleeperSettings) bool {
	switch platform {
	case PlatformOpenAI:
		return settings.IncludeOpenAI
	case PlatformAnthropic:
		return settings.IncludeAnthropic
	default:
		return false
	}
}

func normalizeOAuthSleeperGroupIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return []int64{}
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func normalizeOAuthSleeperGroupThresholds(thresholds map[int64]float64) map[int64]float64 {
	if len(thresholds) == 0 {
		return map[int64]float64{}
	}
	out := make(map[int64]float64, len(thresholds))
	for groupID, threshold := range thresholds {
		out[groupID] = threshold
	}
	return out
}

func copyOAuthSleeperGroupThresholds(thresholds map[int64]float64) map[int64]float64 {
	if len(thresholds) == 0 {
		return map[int64]float64{}
	}
	out := make(map[int64]float64, len(thresholds))
	for groupID, threshold := range thresholds {
		out[groupID] = threshold
	}
	return out
}

func oauthSleeperGroupIDs(groups []OAuthSleeperGroup) []int64 {
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group.ID > 0 {
			ids = append(ids, group.ID)
		}
	}
	return normalizeOAuthSleeperGroupIDs(ids)
}

func int64Set(ids []int64) map[int64]struct{} {
	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func int64SetContains(ids []int64, want int64) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func oauthSleeperSelectedAccountGroupIDs(accountGroupIDs []int64, selected map[int64]struct{}) []int64 {
	if len(accountGroupIDs) == 0 || len(selected) == 0 {
		return nil
	}
	out := make([]int64, 0, len(accountGroupIDs))
	seen := make(map[int64]struct{}, len(accountGroupIDs))
	for _, groupID := range accountGroupIDs {
		if _, ok := selected[groupID]; !ok {
			continue
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		out = append(out, groupID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func oauthSleeperAnyGroupAtLimit(groupIDs []int64, triggeredByGroup map[int64]int, limit int) bool {
	for _, groupID := range groupIDs {
		if triggeredByGroup[groupID] >= limit {
			return true
		}
	}
	return false
}

func oauthSleeperPlatforms(settings OAuthSleeperSettings) []string {
	platforms := make([]string, 0, 2)
	if settings.IncludeOpenAI {
		platforms = append(platforms, PlatformOpenAI)
	}
	if settings.IncludeAnthropic {
		platforms = append(platforms, PlatformAnthropic)
	}
	return platforms
}

func evaluateOAuthSleeperAccount(account Account, settings OAuthSleeperSettings, now time.Time) (oauthSleeperCandidate, bool) {
	normalizeOAuthSleeperSettingsForRead(&settings)
	threshold := oauthSleeperEffectiveThresholdForAccount(account, settings)
	return evaluateOAuthSleeperAccountWithThreshold(account, settings, threshold, now)
}

func evaluateOAuthSleeperAccountWithThreshold(account Account, settings OAuthSleeperSettings, threshold float64, now time.Time) (oauthSleeperCandidate, bool) {
	if account.Status != StatusActive || account.Type != AccountTypeOAuth {
		return oauthSleeperCandidate{}, false
	}

	var candidates []oauthSleeperCandidate
	switch account.Platform {
	case PlatformOpenAI:
		if !settings.IncludeOpenAI {
			return oauthSleeperCandidate{}, false
		}
		candidates = append(candidates, evaluateOAuthSleeperWindow(account, "codex_5h", threshold, "codex_5h_used_percent", "codex_5h_reset_at", now, false)...)
		candidates = append(candidates, evaluateOAuthSleeperWindow(account, "codex_7d", threshold, "codex_7d_used_percent", "codex_7d_reset_at", now, false)...)
	case PlatformAnthropic:
		if !settings.IncludeAnthropic {
			return oauthSleeperCandidate{}, false
		}
		if account.SessionWindowEnd != nil {
			candidates = append(candidates, evaluateOAuthSleeperFixedResetWindow(account, "session_window", threshold, "session_window_utilization", *account.SessionWindowEnd, now, true)...)
		}
		candidates = append(candidates, evaluateOAuthSleeperWindow(account, "passive_usage_7d", threshold, "passive_usage_7d_utilization", "passive_usage_7d_reset", now, true)...)
	default:
		return oauthSleeperCandidate{}, false
	}

	if len(candidates) == 0 {
		return oauthSleeperCandidate{}, false
	}
	sortOAuthSleeperCandidatesByReset(candidates)
	return candidates[0], true
}

func oauthSleeperRecoveryResetAt(account Account, fallback time.Time, now time.Time) time.Time {
	if account.RateLimitResetAt != nil && account.RateLimitResetAt.After(now) {
		return account.RateLimitResetAt.UTC()
	}

	for _, key := range oauthSleeperAccountManagement7dResetKeys(account.Platform) {
		resetAt, ok := extraTime(account.Extra, key)
		if ok && resetAt.After(now) {
			return resetAt.UTC()
		}
	}

	return fallback.UTC()
}

func oauthSleeperAccountManagement7dResetKeys(platform string) []string {
	switch platform {
	case PlatformOpenAI:
		return []string{"codex_7d_reset_at"}
	case PlatformAnthropic:
		return []string{"passive_usage_7d_reset"}
	default:
		return nil
	}
}

func oauthSleeperEffectiveThresholdForAccount(account Account, settings OAuthSleeperSettings) float64 {
	threshold := settings.ThresholdPercent
	if threshold <= 0 {
		threshold = defaultOAuthSleeperThresholdPercent
	}
	if len(settings.GroupIDs) == 0 || len(settings.GroupThresholdPercent) == 0 || len(account.GroupIDs) == 0 {
		return threshold
	}
	selected := oauthSleeperSelectedAccountGroupIDs(account.GroupIDs, int64Set(settings.GroupIDs))
	for _, groupID := range selected {
		if groupThreshold, ok := settings.GroupThresholdPercent[groupID]; ok && groupThreshold > 0 && groupThreshold < threshold {
			threshold = groupThreshold
		}
	}
	return threshold
}

func evaluateOAuthSleeperWindow(account Account, window string, threshold float64, utilizationKey, resetKey string, now time.Time, fraction bool) []oauthSleeperCandidate {
	resetAt, ok := extraTime(account.Extra, resetKey)
	if !ok {
		return nil
	}
	return evaluateOAuthSleeperFixedResetWindow(account, window, threshold, utilizationKey, resetAt, now, fraction)
}

func evaluateOAuthSleeperFixedResetWindow(account Account, window string, threshold float64, utilizationKey string, resetAt time.Time, now time.Time, fraction bool) []oauthSleeperCandidate {
	utilization, ok := extraFloat(account.Extra, utilizationKey)
	if !ok {
		return nil
	}
	if fraction {
		utilization *= 100
	}
	if utilization < threshold || !resetAt.After(now) {
		return nil
	}
	return []oauthSleeperCandidate{{
		account:            account,
		window:             window,
		utilizationPercent: utilization,
		thresholdPercent:   threshold,
		resetAt:            resetAt.UTC(),
	}}
}

func sortOAuthSleeperCandidates(candidates []oauthSleeperCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].utilizationPercent != candidates[j].utilizationPercent {
			return candidates[i].utilizationPercent > candidates[j].utilizationPercent
		}
		if !candidates[i].resetAt.Equal(candidates[j].resetAt) {
			return candidates[i].resetAt.After(candidates[j].resetAt)
		}
		return candidates[i].account.ID < candidates[j].account.ID
	})
}

func sortOAuthSleeperCandidatesByReset(candidates []oauthSleeperCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if !candidates[i].resetAt.Equal(candidates[j].resetAt) {
			return candidates[i].resetAt.After(candidates[j].resetAt)
		}
		if candidates[i].utilizationPercent != candidates[j].utilizationPercent {
			return candidates[i].utilizationPercent > candidates[j].utilizationPercent
		}
		return candidates[i].window < candidates[j].window
	})
}

func extraFloat(extra map[string]any, key string) (float64, bool) {
	if extra == nil {
		return 0, false
	}
	switch v := extra[key].(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func extraTime(extra map[string]any, key string) (time.Time, bool) {
	if extra == nil {
		return time.Time{}, false
	}
	return parseOAuthSleeperTime(extra[key])
}

func parseOAuthSleeperTime(value any) (time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		if v.IsZero() {
			return time.Time{}, false
		}
		return v.UTC(), true
	case string:
		raw := strings.TrimSpace(v)
		if raw == "" {
			return time.Time{}, false
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
			if parsed, err := time.Parse(layout, raw); err == nil {
				return parsed.UTC(), true
			}
		}
		if unix, err := strconv.ParseInt(raw, 10, 64); err == nil && unix > 0 {
			return time.Unix(unix, 0).UTC(), true
		}
	case json.Number:
		if unix, err := v.Int64(); err == nil && unix > 0 {
			return time.Unix(unix, 0).UTC(), true
		}
	case int64:
		if v > 0 {
			return time.Unix(v, 0).UTC(), true
		}
	case int:
		if v > 0 {
			return time.Unix(int64(v), 0).UTC(), true
		}
	case float64:
		if v > 0 {
			return time.Unix(int64(v), 0).UTC(), true
		}
	}
	return time.Time{}, false
}
