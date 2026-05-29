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
	groups   []service.OAuthSleeperGroup
	accounts []service.Account
	events   []service.OAuthSleeperEvent
}

func (r *oauthSleeperHandlerRepoStub) GetOAuthSleeperAccount(_ context.Context, accountID int64) (*service.Account, error) {
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

func (r *oauthSleeperHandlerRepoStub) ListOAuthSleeperAccounts(context.Context, []string, []int64) ([]service.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]service.Account(nil), r.accounts...), nil
}

func (r *oauthSleeperHandlerRepoStub) ListOAuthSleeperGroups(_ context.Context, groupIDs []int64) ([]service.OAuthSleeperGroup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(groupIDs) == 0 {
		return []service.OAuthSleeperGroup{}, nil
	}
	allowed := map[int64]struct{}{}
	for _, id := range groupIDs {
		allowed[id] = struct{}{}
	}
	groups := make([]service.OAuthSleeperGroup, 0, len(r.groups))
	for _, group := range r.groups {
		if _, ok := allowed[group.ID]; ok {
			groups = append(groups, group)
		}
	}
	return groups, nil
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

func (r *oauthSleeperHandlerRepoStub) ListOAuthSleeperSleepingAccounts(context.Context, []string, []int64, time.Time, int) ([]service.OAuthSleeperSleepingAccount, error) {
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
	router.GET("/admin/oauth-sleeper/events", handler.ListEvents)
	return router
}

func TestOAuthSleeperHandlerSettingsDefaultAndUpdate(t *testing.T) {
	repo := &oauthSleeperHandlerRepoStub{
		groups: []service.OAuthSleeperGroup{{ID: 1, Name: "OpenAI", Platform: service.PlatformOpenAI}},
	}
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
	require.Equal(t, 90.0, getResp.Data.ThresholdPercent)

	payload := service.OAuthSleeperSettings{
		Enabled:          true,
		ThresholdPercent: 90,
		IncludeOpenAI:    true,
		IncludeAnthropic: false,
		GroupIDs:         []int64{1},
		GroupThresholdPercent: map[int64]float64{
			1: 88,
		},
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
		"include_openai": true,
		"include_anthropic": true
	}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOAuthSleeperHandlerEvents(t *testing.T) {
	resetAt := time.Now().UTC().Add(time.Hour)
	repo := &oauthSleeperHandlerRepoStub{
		groups: []service.OAuthSleeperGroup{{ID: 1, Name: "OpenAI", Platform: service.PlatformOpenAI}},
		events: []service.OAuthSleeperEvent{
			{
				ID:                 1,
				AccountID:          7,
				AccountName:        "openai-oauth",
				Platform:           service.PlatformOpenAI,
				Window:             "codex_5h",
				UtilizationPercent: 99,
				ThresholdPercent:   95,
				ResetAt:            resetAt,
				CreatedAt:          time.Now().UTC(),
			},
		},
	}
	router := setupOAuthSleeperHandlerRouter(repo, &oauthSleeperHandlerSettingRepoStub{})

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
