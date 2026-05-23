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
	defaultOAuthSleeperThresholdPercent   = 95
	defaultOAuthSleeperScanIntervalSecond = 300
	defaultOAuthSleeperMaxSleepPerScan    = 3

	oauthSleeperMinThresholdPercent   = 1
	oauthSleeperMaxThresholdPercent   = 100
	oauthSleeperMinScanIntervalSecond = 30
	oauthSleeperMaxScanIntervalSecond = 86400
	oauthSleeperMinSleepPerScan       = 1
	oauthSleeperMaxSleepPerScan       = 100

	oauthSleeperStatusAccountsLimit = 20
)

var ErrOAuthSleeperInvalidSettings = infraerrors.BadRequest("OAUTH_SLEEPER_INVALID_SETTINGS", "invalid oauth sleeper settings")

// OAuthSleeperRepository is the narrow persistence surface used by OAuthSleeperService.
type OAuthSleeperRepository interface {
	ListOAuthSleeperAccounts(ctx context.Context, platforms []string) ([]Account, error)
	CreateOAuthSleeperEventAfterRateLimit(ctx context.Context, event *OAuthSleeperEvent) (bool, error)
	ListOAuthSleeperEvents(ctx context.Context, params pagination.PaginationParams) ([]OAuthSleeperEvent, *pagination.PaginationResult, error)
	ListOAuthSleeperSleepingAccounts(ctx context.Context, platforms []string, now time.Time, limit int) ([]OAuthSleeperSleepingAccount, error)
}

type OAuthSleeperSettings struct {
	Enabled             bool    `json:"enabled"`
	ThresholdPercent    float64 `json:"threshold_percent"`
	ScanIntervalSeconds int     `json:"scan_interval_seconds"`
	MaxSleepPerScan     int     `json:"max_sleep_per_scan"`
	IncludeOpenAI       bool    `json:"include_openai"`
	IncludeAnthropic    bool    `json:"include_anthropic"`
}

type OAuthSleeperStatus struct {
	Enabled             bool                          `json:"enabled"`
	ThresholdPercent    float64                       `json:"threshold_percent"`
	ScanIntervalSeconds int                           `json:"scan_interval_seconds"`
	MaxSleepPerScan     int                           `json:"max_sleep_per_scan"`
	IncludeOpenAI       bool                          `json:"include_openai"`
	IncludeAnthropic    bool                          `json:"include_anthropic"`
	LastScanAt          *time.Time                    `json:"last_scan_at,omitempty"`
	LastScanned         int                           `json:"last_scanned"`
	LastTriggered       int                           `json:"last_triggered"`
	LastError           string                        `json:"last_error,omitempty"`
	SleepingAccounts    []OAuthSleeperSleepingAccount `json:"sleeping_accounts"`
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
		Enabled:             false,
		ThresholdPercent:    defaultOAuthSleeperThresholdPercent,
		ScanIntervalSeconds: defaultOAuthSleeperScanIntervalSecond,
		MaxSleepPerScan:     defaultOAuthSleeperMaxSleepPerScan,
		IncludeOpenAI:       true,
		IncludeAnthropic:    true,
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
	if err := ValidateOAuthSleeperSettings(settings); err != nil {
		return nil, err
	}
	normalized := *settings
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
	if settings.ScanIntervalSeconds < oauthSleeperMinScanIntervalSecond || settings.ScanIntervalSeconds > oauthSleeperMaxScanIntervalSecond {
		return ErrOAuthSleeperInvalidSettings.WithMetadata(map[string]string{"field": "scan_interval_seconds", "reason": "must be between 30 and 86400"})
	}
	if settings.MaxSleepPerScan < oauthSleeperMinSleepPerScan || settings.MaxSleepPerScan > oauthSleeperMaxSleepPerScan {
		return ErrOAuthSleeperInvalidSettings.WithMetadata(map[string]string{"field": "max_sleep_per_scan", "reason": "must be between 1 and 100"})
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
		sleeping, err = s.repo.ListOAuthSleeperSleepingAccounts(ctx, oauthSleeperPlatforms(*settings), s.now(), oauthSleeperStatusAccountsLimit)
		if err != nil {
			return nil, fmt.Errorf("list oauth sleeper sleeping accounts: %w", err)
		}
	}

	s.statusMu.RLock()
	lastScanAt := s.lastScanAt
	status := &OAuthSleeperStatus{
		Enabled:             settings.Enabled,
		ThresholdPercent:    settings.ThresholdPercent,
		ScanIntervalSeconds: settings.ScanIntervalSeconds,
		MaxSleepPerScan:     settings.MaxSleepPerScan,
		IncludeOpenAI:       settings.IncludeOpenAI,
		IncludeAnthropic:    settings.IncludeAnthropic,
		LastScanAt:          lastScanAt,
		LastScanned:         s.lastScanned,
		LastTriggered:       s.lastTriggered,
		LastError:           s.lastError,
		SleepingAccounts:    sleeping,
	}
	s.statusMu.RUnlock()
	return status, nil
}

func (s *OAuthSleeperService) ScanOnce(ctx context.Context) (*OAuthSleeperScanResult, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	return s.runScan(ctx, *settings)
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
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(settings.ScanIntervalSeconds)*time.Second)
			_, err := s.runScan(ctx, *settings)
			cancel()
			if err != nil {
				slog.Warn("oauth_sleeper: background scan failed", "error", err)
			}
		}

		interval := time.Duration(defaultOAuthSleeperScanIntervalSecond) * time.Second
		if settings != nil && settings.ScanIntervalSeconds > 0 {
			interval = time.Duration(settings.ScanIntervalSeconds) * time.Second
		}

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

	accounts, err := s.repo.ListOAuthSleeperAccounts(ctx, platforms)
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
		triggeredForPlatform := 0
		for _, candidate := range platformCandidates {
			if triggeredForPlatform >= settings.MaxSleepPerScan {
				break
			}
			event := OAuthSleeperEvent{
				AccountID:          candidate.account.ID,
				AccountName:        candidate.account.Name,
				Platform:           candidate.account.Platform,
				Window:             candidate.window,
				UtilizationPercent: candidate.utilizationPercent,
				ThresholdPercent:   candidate.thresholdPercent,
				ResetAt:            candidate.resetAt,
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
			triggeredForPlatform++
			result.Events = append(result.Events, event)
		}
	}

	s.recordScanStatusAt(now, result.Scanned, result.Triggered, nil)
	return result, nil
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
	if settings.ScanIntervalSeconds <= 0 {
		settings.ScanIntervalSeconds = defaultOAuthSleeperScanIntervalSecond
	}
	if settings.MaxSleepPerScan <= 0 {
		settings.MaxSleepPerScan = defaultOAuthSleeperMaxSleepPerScan
	}
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
	if account.Status != StatusActive || account.Type != AccountTypeOAuth {
		return oauthSleeperCandidate{}, false
	}

	var candidates []oauthSleeperCandidate
	switch account.Platform {
	case PlatformOpenAI:
		if !settings.IncludeOpenAI {
			return oauthSleeperCandidate{}, false
		}
		candidates = append(candidates, evaluateOAuthSleeperWindow(account, "codex_5h", settings.ThresholdPercent, "codex_5h_used_percent", "codex_5h_reset_at", now, false)...)
		candidates = append(candidates, evaluateOAuthSleeperWindow(account, "codex_7d", settings.ThresholdPercent, "codex_7d_used_percent", "codex_7d_reset_at", now, false)...)
	case PlatformAnthropic:
		if !settings.IncludeAnthropic {
			return oauthSleeperCandidate{}, false
		}
		if account.SessionWindowEnd != nil {
			candidates = append(candidates, evaluateOAuthSleeperFixedResetWindow(account, "session_window", settings.ThresholdPercent, "session_window_utilization", *account.SessionWindowEnd, now, true)...)
		}
		candidates = append(candidates, evaluateOAuthSleeperWindow(account, "passive_usage_7d", settings.ThresholdPercent, "passive_usage_7d_utilization", "passive_usage_7d_reset", now, true)...)
	default:
		return oauthSleeperCandidate{}, false
	}

	if len(candidates) == 0 {
		return oauthSleeperCandidate{}, false
	}
	sortOAuthSleeperCandidatesByReset(candidates)
	return candidates[0], true
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
