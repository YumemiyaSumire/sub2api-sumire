package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestOpenAICacheDebugEnabled(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "empty", value: "", want: false},
		{name: "zero", value: "0", want: false},
		{name: "debug", value: "debug", want: false},
		{name: "one", value: "1", want: true},
		{name: "true mixed case", value: "TrUe", want: true},
		{name: "yes", value: "yes", want: true},
		{name: "on", value: "on", want: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(debugCacheKeysEnv, tc.value)
			if got := openAICacheDebugEnabled(); got != tc.want {
				t.Fatalf("openAICacheDebugEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLogOpenAICacheDebugIngress(t *testing.T) {
	t.Setenv(debugCacheKeysEnv, "1")

	core, observed := observer.New(zap.InfoLevel)
	reqLog := zap.New(core)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req.Header.Set("session_id", " sess-1 ")
	req.Header.Set("conversation_id", " conv-1 ")
	c.Request = req

	body := []byte(`{"prompt_cache_key":" key-1 ","metadata":{"user_id":" user-1 "},"messages":[{"role":"user"}],"input":[{"role":"user"},{"role":"assistant"}]}`)

	logOpenAICacheDebugIngress(reqLog, c, "responses", body)

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Message != "openai.cache_debug_ingress" {
		t.Fatalf("unexpected message: %s", entry.Message)
	}

	fields := entry.ContextMap()
	if fields["endpoint"] != "responses" {
		t.Fatalf("endpoint = %v, want responses", fields["endpoint"])
	}
	if fields["session_id"] != "sess-1" {
		t.Fatalf("session_id = %v, want sess-1", fields["session_id"])
	}
	if fields["conversation_id"] != "conv-1" {
		t.Fatalf("conversation_id = %v, want conv-1", fields["conversation_id"])
	}
	if fields["prompt_cache_key"] != "key-1" {
		t.Fatalf("prompt_cache_key = %v, want key-1", fields["prompt_cache_key"])
	}
	if fields["metadata_user_id"] != "user-1" {
		t.Fatalf("metadata_user_id = %v, want user-1", fields["metadata_user_id"])
	}
	if fields["messages_count"] != int64(1) {
		t.Fatalf("messages_count = %v, want 1", fields["messages_count"])
	}
	if fields["input_count"] != int64(2) {
		t.Fatalf("input_count = %v, want 2", fields["input_count"])
	}
	if fields["body_bytes"] == int64(0) {
		t.Fatalf("body_bytes should be non-zero")
	}
}

func TestLogOpenAICacheDebugResult(t *testing.T) {
	t.Setenv(debugCacheKeysEnv, "1")

	core, observed := observer.New(zap.InfoLevel)
	reqLog := zap.New(core)

	result := &service.OpenAIForwardResult{
		RequestID:     " generated:req-1 ",
		UpstreamModel: " gpt-5.5 ",
		Usage: service.ClaudeUsage{
			CacheReadInputTokens:     123,
			CacheCreationInputTokens: 45,
		},
	}

	logOpenAICacheDebugResult(reqLog, "chat_completions", result, 47)

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Message != "openai.cache_debug_result" {
		t.Fatalf("unexpected message: %s", entry.Message)
	}

	fields := entry.ContextMap()
	if fields["endpoint"] != "chat_completions" {
		t.Fatalf("endpoint = %v, want chat_completions", fields["endpoint"])
	}
	if fields["usage_request_id"] != "generated:req-1" {
		t.Fatalf("usage_request_id = %v, want generated:req-1", fields["usage_request_id"])
	}
	if fields["account_id"] != int64(47) {
		t.Fatalf("account_id = %v, want 47", fields["account_id"])
	}
	if fields["upstream_model"] != "gpt-5.5" {
		t.Fatalf("upstream_model = %v, want gpt-5.5", fields["upstream_model"])
	}
	if fields["cache_read_input_tokens"] != int64(123) {
		t.Fatalf("cache_read_input_tokens = %v, want 123", fields["cache_read_input_tokens"])
	}
	if fields["cache_creation_input_tokens"] != int64(45) {
		t.Fatalf("cache_creation_input_tokens = %v, want 45", fields["cache_creation_input_tokens"])
	}
}
