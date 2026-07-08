package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type batchTestAccountRepo struct {
	service.AccountRepository

	groupAccounts []service.Account
	accountsByID  map[int64]*service.Account
}

func (r *batchTestAccountRepo) ListByGroup(ctx context.Context, groupID int64) ([]service.Account, error) {
	return append([]service.Account(nil), r.groupAccounts...), nil
}

func (r *batchTestAccountRepo) GetByID(ctx context.Context, id int64) (*service.Account, error) {
	if account, ok := r.accountsByID[id]; ok {
		copied := *account
		return &copied, nil
	}
	return nil, service.ErrAccountNotFound
}

func (r *batchTestAccountRepo) SetError(ctx context.Context, id int64, errorMsg string) error {
	return nil
}

type batchTestHTTPUpstream struct {
	mu    sync.Mutex
	calls []int64
}

func (u *batchTestHTTPUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (u *batchTestHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	u.mu.Lock()
	u.calls = append(u.calls, accountID)
	u.mu.Unlock()

	if accountID == 1 {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`data: {"type":"content_block_delta","delta":{"text":"ok"}}

data: {"type":"message_stop"}

`)),
		}, nil
	}

	return &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("invalid token")),
	}, nil
}

func setupBatchTestRouter(accountTestSvc *service.AccountTestService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(newStubAdminService(), nil, nil, nil, nil, nil, nil, accountTestSvc, nil, nil, nil, nil, nil)
	router.POST("/api/v1/admin/accounts/batch-test", handler.BatchTest)
	return router
}

func newBatchTestService(accounts []service.Account, upstream service.HTTPUpstream) *service.AccountTestService {
	accountsByID := make(map[int64]*service.Account, len(accounts))
	for i := range accounts {
		account := accounts[i]
		accountsByID[account.ID] = &account
	}
	return service.NewAccountTestService(
		&batchTestAccountRepo{groupAccounts: accounts, accountsByID: accountsByID},
		nil,
		nil,
		nil,
		nil,
		upstream,
		&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		nil,
	)
}

func performBatchTestRequest(router *gin.Engine, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/batch-test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

func TestAccountHandlerBatchTestRejectsMissingInputs(t *testing.T) {
	router := setupBatchTestRouter(nil)

	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "missing group", body: `{"model_id":"claude-sonnet-4-5"}`},
		{name: "missing model", body: `{"group_id":1}`},
		{name: "empty model", body: `{"group_id":1,"model_id":"   "}`},
		{name: "invalid group", body: `{"group_id":0,"model_id":"claude-sonnet-4-5"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := performBatchTestRequest(router, tc.body)
			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestAccountHandlerBatchTestRequiresService(t *testing.T) {
	router := setupBatchTestRouter(nil)

	rec := performBatchTestRequest(router, `{"group_id":1,"model_id":"claude-sonnet-4-5"}`)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAccountHandlerBatchTestEmptyGroupReturnsZeroSummary(t *testing.T) {
	router := setupBatchTestRouter(newBatchTestService(nil, &batchTestHTTPUpstream{}))

	rec := performBatchTestRequest(router, `{"group_id":9,"model_id":"claude-sonnet-4-5"}`)

	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		Data service.BatchAccountTestResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, int64(9), payload.Data.GroupID)
	require.Equal(t, "claude-sonnet-4-5", payload.Data.ModelID)
	require.Equal(t, 0, payload.Data.Total)
	require.Equal(t, 0, payload.Data.Success)
	require.Equal(t, 0, payload.Data.Failed)
	require.Empty(t, payload.Data.Results)
}

func TestAccountHandlerBatchTestKeepsFailuresAndStableResults(t *testing.T) {
	upstream := &batchTestHTTPUpstream{}
	accounts := []service.Account{
		{
			ID:       2,
			Name:     "Beta",
			Platform: service.PlatformAnthropic,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusActive,
			Credentials: map[string]any{
				"access_token": "bad-token",
			},
		},
		{
			ID:       1,
			Name:     "Alpha",
			Platform: service.PlatformAnthropic,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusActive,
			Credentials: map[string]any{
				"access_token": "good-token",
			},
		},
	}
	router := setupBatchTestRouter(newBatchTestService(accounts, upstream))

	rec := performBatchTestRequest(router, `{"group_id":7,"model_id":"claude-sonnet-4-5"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload struct {
		Data service.BatchAccountTestResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, 2, payload.Data.Total)
	require.Equal(t, 1, payload.Data.Success)
	require.Equal(t, 1, payload.Data.Failed)
	require.Len(t, payload.Data.Results, 2)

	require.Equal(t, int64(1), payload.Data.Results[0].AccountID)
	require.Equal(t, "Alpha", payload.Data.Results[0].AccountName)
	require.Equal(t, "success", payload.Data.Results[0].Status)
	require.Equal(t, "ok", payload.Data.Results[0].ResponseText)

	require.Equal(t, int64(2), payload.Data.Results[1].AccountID)
	require.Equal(t, "Beta", payload.Data.Results[1].AccountName)
	require.Equal(t, "failed", payload.Data.Results[1].Status)
	require.Contains(t, payload.Data.Results[1].ErrorMessage, "invalid token")

	require.ElementsMatch(t, []int64{1, 2}, upstream.calls)
}

func TestAccountHandlerBatchTestDoesNotCancelRemainingAccountsOnFailure(t *testing.T) {
	upstream := &batchTestHTTPUpstream{}
	accounts := make([]service.Account, 0, 6)
	for i := int64(1); i <= 6; i++ {
		accounts = append(accounts, service.Account{
			ID:       i,
			Name:     fmt.Sprintf("Account %02d", i),
			Platform: service.PlatformAnthropic,
			Type:     service.AccountTypeOAuth,
			Status:   service.StatusActive,
			Credentials: map[string]any{
				"access_token": fmt.Sprintf("token-%d", i),
			},
		})
	}
	router := setupBatchTestRouter(newBatchTestService(accounts, upstream))

	rec := performBatchTestRequest(router, `{"group_id":8,"model_id":"claude-sonnet-4-5"}`)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload struct {
		Data service.BatchAccountTestResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, 6, payload.Data.Total)
	require.Equal(t, 1, payload.Data.Success)
	require.Equal(t, 5, payload.Data.Failed)
	require.Len(t, upstream.calls, 6)
}
