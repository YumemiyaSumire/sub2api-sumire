package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type oauthSleeperHandlerRepoStub struct {
	mu       sync.Mutex
	accounts []service.Account
	events   []service.OAuthSleeperEvent
}

func (r *oauthSleeperHandlerRepoStub) ListOAuthSleeperAccounts(context.Context, []string) ([]service.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]service.Account(nil), r.accounts...), nil
}

func (r *oauthSleeperHandlerRepoStub) CreateOAuthSleeperEventAfterRateLimit(_ context.Context, event *service.OAuthSleeperEvent) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	event.ID = int64(len(r.events) + 1)
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	r.events = append(r.events, *event)
	return true, nil
}

func (r *oauthSleeperHandlerRepoStub) ListOAuthSleeperEvents(_ context.Context, params pagination.PaginationParams) ([]service.OAuthSleeperEvent, *pagination.PaginationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pageSize := params.Limit()
	return append([]service.OAuthSleeperEvent(nil), r.events...), &pagination.PaginationResult{
		Total:    int64(len(r.events)),
		Page:     params.Page,
		PageSize: pageSize,
		Pages:    1,
	}, nil
}

func (r *oauthSleeperHandlerRepoStub) ListOAuthSleeperSleepingAccounts(context.Context, []string, time.Time, int) ([]service.OAuthSleeperSleepingAccount, error) {
	return []service.OAuthSleeperSleepingAccount{
		{
			AccountID:        9,
			AccountName:      "sleeping-openai",
			Platform:         service.PlatformOpenAI,
			RateLimitResetAt: time.Date(2026, 5, 24, 18, 0, 0, 0, time.UTC),
			RemainingSeconds: 3600,
		},
	}, nil
}

type oauthSleeperHandlerSettingRepoStub struct {
	mu   sync.Mutex
	data map[string]string
}

func (r *oauthSleeperHandlerSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	return nil, service.ErrSettingNotFound
}

func (r *oauthSleeperHandlerSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.data == nil {
		return "", service.ErrSettingNotFound
	}
	v, ok := r.data[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return v, nil
}

func (r *oauthSleeperHandlerSettingRepoStub) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.data == nil {
		r.data = map[string]string{}
	}
	r.data[key] = value
	return nil
}

func (r *oauthSleeperHandlerSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *oauthSleeperHandlerSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.data == nil {
		r.data = map[string]string{}
	}
	for k, v := range settings {
		r.data[k] = v
	}
	return nil
}

func (r *oauthSleeperHandlerSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *oauthSleeperHandlerSettingRepoStub) Delete(_ context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.data, key)
	return nil
}

func setupOAuthSleeperHandlerRouter(repo *oauthSleeperHandlerRepoStub, settingRepo *oauthSleeperHandlerSettingRepoStub) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := service.NewOAuthSleeperService(repo, settingRepo)
	handler := NewOAuthSleeperHandler(svc)
	router := gin.New()
	router.GET("/admin/oauth-sleeper/status", handler.GetStatus)
	router.GET("/admin/oauth-sleeper/settings", handler.GetSettings)
	router.PUT("/admin/oauth-sleeper/settings", handler.UpdateSettings)
	router.POST("/admin/oauth-sleeper/scan-once", handler.ScanOnce)
	router.GET("/admin/oauth-sleeper/events", handler.ListEvents)
	return router
}

func TestOAuthSleeperHandlerSettingsDefaultAndUpdate(t *testing.T) {
	repo := &oauthSleeperHandlerRepoStub{}
	settingRepo := &oauthSleeperHandlerSettingRepoStub{}
	router := setupOAuthSleeperHandlerRouter(repo, settingRepo)

	getReq := httptest.NewRequest(http.MethodGet, "/admin/oauth-sleeper/settings", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	require.Equal(t, http.StatusOK, getRec.Code)

	var getResp struct {
		Code int                          `json:"code"`
		Data service.OAuthSleeperSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(getRec.Body.Bytes(), &getResp))
	require.False(t, getResp.Data.Enabled)
	require.Equal(t, 95.0, getResp.Data.ThresholdPercent)
	require.Equal(t, 300, getResp.Data.ScanIntervalSeconds)

	payload := service.OAuthSleeperSettings{
		Enabled:             true,
		ThresholdPercent:    90,
		ScanIntervalSeconds: 120,
		MaxSleepPerScan:     5,
		IncludeOpenAI:       true,
		IncludeAnthropic:    false,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	putReq := httptest.NewRequest(http.MethodPut, "/admin/oauth-sleeper/settings", bytes.NewReader(raw))
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	router.ServeHTTP(putRec, putReq)
	require.Equal(t, http.StatusOK, putRec.Code)

	var putResp struct {
		Code int                          `json:"code"`
		Data service.OAuthSleeperSettings `json:"data"`
	}
	require.NoError(t, json.Unmarshal(putRec.Body.Bytes(), &putResp))
	require.Equal(t, payload, putResp.Data)
}

func TestOAuthSleeperHandlerRejectsInvalidSettings(t *testing.T) {
	router := setupOAuthSleeperHandlerRouter(&oauthSleeperHandlerRepoStub{}, &oauthSleeperHandlerSettingRepoStub{})

	req := httptest.NewRequest(http.MethodPut, "/admin/oauth-sleeper/settings", bytes.NewBufferString(`{
		"enabled": true,
		"threshold_percent": 101,
		"scan_interval_seconds": 120,
		"max_sleep_per_scan": 5,
		"include_openai": true,
		"include_anthropic": true
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOAuthSleeperHandlerScanOnceAndEvents(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	repo := &oauthSleeperHandlerRepoStub{
		accounts: []service.Account{
			{
				ID:       7,
				Name:     "openai-oauth",
				Platform: service.PlatformOpenAI,
				Type:     service.AccountTypeOAuth,
				Status:   service.StatusActive,
				Extra: map[string]any{
					"codex_5h_used_percent": 99.0,
					"codex_5h_reset_at":     now.Add(time.Hour).Format(time.RFC3339),
				},
			},
		},
	}
	router := setupOAuthSleeperHandlerRouter(repo, &oauthSleeperHandlerSettingRepoStub{})

	scanReq := httptest.NewRequest(http.MethodPost, "/admin/oauth-sleeper/scan-once", nil)
	scanRec := httptest.NewRecorder()
	router.ServeHTTP(scanRec, scanReq)
	require.Equal(t, http.StatusOK, scanRec.Code)

	var scanResp struct {
		Code int                            `json:"code"`
		Data service.OAuthSleeperScanResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(scanRec.Body.Bytes(), &scanResp))
	require.Equal(t, 1, scanResp.Data.Scanned)
	require.Equal(t, 1, scanResp.Data.Triggered)
	require.Len(t, scanResp.Data.Events, 1)
	require.Equal(t, "openai-oauth", scanResp.Data.Events[0].AccountName)

	eventsReq := httptest.NewRequest(http.MethodGet, "/admin/oauth-sleeper/events?page=2&page_size=10", nil)
	eventsRec := httptest.NewRecorder()
	router.ServeHTTP(eventsRec, eventsReq)
	require.Equal(t, http.StatusOK, eventsRec.Code)

	var eventsResp struct {
		Code int `json:"code"`
		Data struct {
			Items    []service.OAuthSleeperEvent `json:"items"`
			Total    int64                       `json:"total"`
			Page     int                         `json:"page"`
			PageSize int                         `json:"page_size"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(eventsRec.Body.Bytes(), &eventsResp))
	require.Len(t, eventsResp.Data.Items, 1)
	require.Equal(t, int64(1), eventsResp.Data.Total)
	require.Equal(t, 2, eventsResp.Data.Page)
	require.Equal(t, 10, eventsResp.Data.PageSize)
}
