package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIGatewayService_RetriesWithoutAutoPromptCacheRetentionAfterUpstreamRejection(t *testing.T) {
	t.Setenv(openAIAutoPromptCacheEnv, "1")
	t.Setenv(openAIPromptCacheRetentionEnv, "24h")
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("api_key", &APIKey{ID: 6, UserID: 1})
	originalBody := []byte(`{"model":"gpt-5.5","stream":false,"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(originalBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_bad_retention"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Unsupported parameter: prompt_cache_retention"}}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_ok"}},
			Body:       io.NopCloser(strings.NewReader(`{"output":[],"usage":{"input_tokens":1,"output_tokens":1,"input_tokens_details":{"cached_tokens":0}}}`)),
		},
	}}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}
	account := promptCacheRetentionFallbackTestAccount()

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.bodies, 2)
	require.Equal(t, "24h", gjson.GetBytes(upstream.bodies[0], "prompt_cache_retention").String())

	promptCacheKey := gjson.GetBytes(upstream.bodies[0], "prompt_cache_key").String()
	require.NotEmpty(t, promptCacheKey)
	require.False(t, gjson.GetBytes(upstream.bodies[1], "prompt_cache_retention").Exists())
	require.Equal(t, promptCacheKey, gjson.GetBytes(upstream.bodies[1], "prompt_cache_key").String())
	require.Equal(t, gjson.GetBytes(upstream.bodies[0], "input").Raw, gjson.GetBytes(upstream.bodies[1], "input").Raw)
}

func TestOpenAIGatewayService_DoesNotRetryWithoutClientPromptCacheRetention(t *testing.T) {
	t.Setenv(openAIAutoPromptCacheEnv, "1")
	t.Setenv(openAIPromptCacheRetentionEnv, "24h")
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("api_key", &APIKey{ID: 6, UserID: 1})
	originalBody := []byte(`{"model":"gpt-5.5","stream":false,"prompt_cache_retention":"24h","input":[{"type":"text","text":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(originalBody))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{responses: []*http.Response{
		{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_bad_client_retention"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Unsupported parameter: prompt_cache_retention"}}`)),
		},
		{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_unexpected"}},
			Body:       io.NopCloser(strings.NewReader(`{"output":[],"usage":{"input_tokens":1,"output_tokens":1}}`)),
		},
	}}

	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}
	account := promptCacheRetentionFallbackTestAccount()

	result, err := svc.Forward(context.Background(), c, account, originalBody)
	require.Error(t, err)
	require.Nil(t, result)
	require.Len(t, upstream.bodies, 1)
	require.Equal(t, "24h", gjson.GetBytes(upstream.bodies[0], "prompt_cache_retention").String())
	require.NotEmpty(t, gjson.GetBytes(upstream.bodies[0], "prompt_cache_key").String())
}

func promptCacheRetentionFallbackTestAccount() *Account {
	return &Account{
		ID:          123,
		Name:        "acc",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token":       "oauth-token",
			"chatgpt_account_id": "chatgpt-acc",
		},
		Extra:          map[string]any{"openai_passthrough": false, "openai_oauth_responses_websockets_v2_mode": OpenAIWSIngressModeOff},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}
}
