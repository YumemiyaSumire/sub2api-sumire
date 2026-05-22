package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAutoPromptCacheMapDisabledByDefault(t *testing.T) {
	t.Setenv(openAIAutoPromptCacheEnv, "")
	body := map[string]any{"model": "gpt-5.5"}

	result := applyOpenAIAutoPromptCacheToMap(body, openAIPromptCacheOptions{
		Endpoint:       "responses",
		RequestedModel: "gpt-5.5-high",
		UpstreamModel:  "gpt-5.5",
		UserID:         1,
		APIKeyID:       6,
	})

	require.Empty(t, result.PromptCacheKey)
	require.Empty(t, result.PromptCacheRetention)
	require.False(t, result.PromptCacheKeyAutoInjected)
	require.False(t, result.PromptCacheRetentionAutoInjected)
	require.NotContains(t, body, "prompt_cache_key")
	require.NotContains(t, body, "prompt_cache_retention")
}

func TestOpenAIAutoPromptCacheMapInjectsStableKeyAndRetention(t *testing.T) {
	t.Setenv(openAIAutoPromptCacheEnv, "1")
	t.Setenv(openAIPromptCacheRetentionEnv, "")
	body := map[string]any{"model": "gpt-5.5"}

	result := applyOpenAIAutoPromptCacheToMap(body, openAIPromptCacheOptions{
		Endpoint:       "responses",
		RequestedModel: "gpt-5.5-high",
		UpstreamModel:  "gpt-5.5",
		UserID:         1,
		APIKeyID:       6,
	})

	require.Equal(t, "sub2api:openai-cache:user-1:api-key-6:responses:gpt-5.5-high", result.PromptCacheKey)
	require.Equal(t, "24h", result.PromptCacheRetention)
	require.True(t, result.PromptCacheKeyAutoInjected)
	require.True(t, result.PromptCacheRetentionAutoInjected)
	require.Equal(t, result.PromptCacheKey, body["prompt_cache_key"])
	require.Equal(t, result.PromptCacheRetention, body["prompt_cache_retention"])
}

func TestOpenAIAutoPromptCacheMapPreservesClientValues(t *testing.T) {
	t.Setenv(openAIAutoPromptCacheEnv, "1")
	body := map[string]any{
		"model":                  "gpt-5.5",
		"prompt_cache_key":       "client-key",
		"prompt_cache_retention": "in_memory",
	}

	result := applyOpenAIAutoPromptCacheToMap(body, openAIPromptCacheOptions{
		Endpoint:       "responses",
		RequestedModel: "gpt-5.5-high",
		UpstreamModel:  "gpt-5.5",
		UserID:         1,
		APIKeyID:       6,
	})

	require.Equal(t, "client-key", result.PromptCacheKey)
	require.Equal(t, "in_memory", result.PromptCacheRetention)
	require.False(t, result.PromptCacheKeyAutoInjected)
	require.False(t, result.PromptCacheRetentionAutoInjected)
	require.Equal(t, "client-key", body["prompt_cache_key"])
	require.Equal(t, "in_memory", body["prompt_cache_retention"])
}

func TestOpenAIAutoPromptCacheRestoresClientRetentionAfterFiltering(t *testing.T) {
	t.Setenv(openAIAutoPromptCacheEnv, "1")
	body := map[string]any{"model": "gpt-5.5"}
	opts := openAIPromptCacheOptions{
		Endpoint:       "responses",
		RequestedModel: "gpt-5.5-high",
		UpstreamModel:  "gpt-5.5",
		UserID:         1,
		APIKeyID:       6,
	}

	require.True(t, restoreOpenAIClientPromptCacheRetention(body, "in_memory", opts))
	result := applyOpenAIAutoPromptCacheToMap(body, opts)

	require.Equal(t, "in_memory", result.PromptCacheRetention)
	require.False(t, result.PromptCacheRetentionAutoInjected)
	require.Equal(t, "in_memory", body["prompt_cache_retention"])
}

func TestOpenAIAutoPromptCacheMapSkipsRetentionForImageIntent(t *testing.T) {
	t.Setenv(openAIAutoPromptCacheEnv, "1")
	body := map[string]any{"model": "gpt-5.5"}

	result := applyOpenAIAutoPromptCacheToMap(body, openAIPromptCacheOptions{
		Endpoint:       "responses",
		RequestedModel: "gpt-5.5-high",
		UpstreamModel:  "gpt-5.5",
		UserID:         1,
		APIKeyID:       6,
		ImageIntent:    true,
	})

	require.True(t, result.PromptCacheKeyAutoInjected)
	require.False(t, result.PromptCacheRetentionAutoInjected)
	require.NotContains(t, body, "prompt_cache_retention")
}

func TestOpenAIAutoPromptCacheMapSkipsRetentionForImageModel(t *testing.T) {
	t.Setenv(openAIAutoPromptCacheEnv, "1")
	body := map[string]any{"model": "gpt-image-2"}

	result := applyOpenAIAutoPromptCacheToMap(body, openAIPromptCacheOptions{
		Endpoint:       "responses",
		RequestedModel: "gpt-image-2",
		UpstreamModel:  "gpt-image-2",
		UserID:         1,
		APIKeyID:       6,
	})

	require.True(t, result.PromptCacheKeyAutoInjected)
	require.False(t, result.PromptCacheRetentionAutoInjected)
	require.NotContains(t, body, "prompt_cache_retention")
}

func TestOpenAIAutoPromptCacheMapCanDisableRetention(t *testing.T) {
	t.Setenv(openAIAutoPromptCacheEnv, "1")
	t.Setenv(openAIPromptCacheRetentionEnv, "off")
	body := map[string]any{"model": "gpt-5.5"}

	result := applyOpenAIAutoPromptCacheToMap(body, openAIPromptCacheOptions{
		Endpoint:       "chat_completions",
		RequestedModel: "gpt-5.5-high",
		UpstreamModel:  "gpt-5.5",
		UserID:         1,
		APIKeyID:       6,
	})

	require.True(t, result.PromptCacheKeyAutoInjected)
	require.False(t, result.PromptCacheRetentionAutoInjected)
	require.NotContains(t, body, "prompt_cache_retention")
}

func TestOpenAIAutoPromptCacheResponsesRequest(t *testing.T) {
	t.Setenv(openAIAutoPromptCacheEnv, "yes")
	req := &apicompat.ResponsesRequest{Model: "gpt-5.5"}

	result := applyOpenAIAutoPromptCacheToResponsesRequest(req, openAIPromptCacheOptions{
		Endpoint:       "chat_completions",
		RequestedModel: "gpt-5.5-high",
		UpstreamModel:  "gpt-5.5",
		UserID:         1,
		APIKeyID:       6,
	})

	require.Equal(t, "sub2api:openai-cache:user-1:api-key-6:chat_completions:gpt-5.5-high", req.PromptCacheKey)
	require.Equal(t, "24h", req.PromptCacheRetention)
	require.True(t, result.PromptCacheKeyAutoInjected)
	require.True(t, result.PromptCacheRetentionAutoInjected)
}

func TestOpenAIPromptCacheForwardDebugIncludesTraceAndHashesKey(t *testing.T) {
	t.Setenv("SUB2API_DEBUG_CACHE_KEYS", "1")

	sink, cleanup := captureStructuredLog(t)
	defer cleanup()

	logOpenAIPromptCacheForwardDebug(openAIPromptCacheApplyResult{
		PromptCacheKey:                   "cache-key-1",
		PromptCacheRetention:             "24h",
		PromptCacheKeyAutoInjected:       true,
		PromptCacheRetentionAutoInjected: true,
	}, openAIPromptCacheOptions{
		Endpoint:       "responses",
		RequestedModel: "gpt-5.5-high",
		UpstreamModel:  "gpt-5.5",
		UserID:         1,
		APIKeyID:       6,
		CacheTraceID:   "trace-forward",
	})

	require.True(t, sink.ContainsMessage("openai.cache_debug_forward"))
	require.True(t, sink.ContainsFieldValue("cache_trace_id", "trace-forward"))
	require.True(t, sink.ContainsFieldValue("prompt_cache_key", "cache-key-1"))
	require.True(t, sink.ContainsField("prompt_cache_key_sha256"))
}
