package handler

import (
	"fmt"
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
	if fields["cache_trace_id"] == "" {
		t.Fatalf("cache_trace_id should be non-empty")
	}
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
	if fields["prompt_cache_key_sha256"] == "" {
		t.Fatalf("prompt_cache_key_sha256 should be non-empty")
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
	if fields["full_body_hash"] == "" {
		t.Fatalf("full_body_hash should be non-empty")
	}
	if fields["input_hash"] == "" {
		t.Fatalf("input_hash should be non-empty")
	}
	if fields["input_prefix_hash"] == "" {
		t.Fatalf("input_prefix_hash should be non-empty")
	}
	if fields["input_item_source"] != "input" {
		t.Fatalf("input_item_source = %v, want input", fields["input_item_source"])
	}
	assertStringSliceField(t, fields["input_item_type_list"], []string{"user", "assistant"})
	assertIntSliceField(t, fields["input_item_length_list"], []int{15, 20})
	if fields["reasoning_effort"] != "" {
		t.Fatalf("reasoning_effort = %v, want empty", fields["reasoning_effort"])
	}
	if fields["tools_count"] != int64(0) {
		t.Fatalf("tools_count = %v, want 0", fields["tools_count"])
	}
}

func TestLogOpenAICacheDebugResult(t *testing.T) {
	t.Setenv(debugCacheKeysEnv, "1")

	core, observed := observer.New(zap.InfoLevel)
	reqLog := zap.New(core)

	result := &service.OpenAIForwardResult{
		RequestID:     " generated:req-1 ",
		UpstreamModel: " gpt-5.5 ",
		Usage: service.OpenAIUsage{
			CacheReadInputTokens:     123,
			CacheCreationInputTokens: 45,
		},
	}
	c := &gin.Context{}
	c.Set(service.OpenAICacheTraceIDContextKey, "trace-1")

	logOpenAICacheDebugResult(reqLog, c, "chat_completions", result, 47)

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Message != "openai.cache_debug_result" {
		t.Fatalf("unexpected message: %s", entry.Message)
	}

	fields := entry.ContextMap()
	if fields["cache_trace_id"] != "trace-1" {
		t.Fatalf("cache_trace_id = %v, want trace-1", fields["cache_trace_id"])
	}
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

func TestLogOpenAICacheDebugSticky(t *testing.T) {
	t.Setenv(debugCacheKeysEnv, "1")

	core, observed := observer.New(zap.InfoLevel)
	reqLog := zap.New(core)
	c := &gin.Context{}
	c.Set(service.OpenAICacheTraceIDContextKey, "trace-sticky")

	logOpenAICacheDebugSticky(reqLog, c, "responses", "session-hash-1", service.OpenAIAccountScheduleDecision{
		Layer:               "session_hash",
		StickySessionHit:    true,
		CandidateCount:      3,
		TopK:                2,
		SelectedAccountType: service.AccountTypeOAuth,
	}, 47, " account-name ", 1)

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Message != "openai.cache_debug_sticky" {
		t.Fatalf("unexpected message: %s", entry.Message)
	}

	fields := entry.ContextMap()
	if fields["cache_trace_id"] != "trace-sticky" {
		t.Fatalf("cache_trace_id = %v, want trace-sticky", fields["cache_trace_id"])
	}
	if fields["session_hash"] != "session-hash-1" {
		t.Fatalf("session_hash = %v, want session-hash-1", fields["session_hash"])
	}
	if fields["sticky_session_hit"] != true {
		t.Fatalf("sticky_session_hit = %v, want true", fields["sticky_session_hit"])
	}
	if fields["selected_account_id"] != int64(47) {
		t.Fatalf("selected_account_id = %v, want 47", fields["selected_account_id"])
	}
	if fields["selected_account_name"] != "account-name" {
		t.Fatalf("selected_account_name = %v, want account-name", fields["selected_account_name"])
	}
	if fields["switch_count"] != int64(1) {
		t.Fatalf("switch_count = %v, want 1", fields["switch_count"])
	}
}

func TestLogOpenAICacheDebugIngressFallsBackToMessages(t *testing.T) {
	t.Setenv(debugCacheKeysEnv, "1")

	core, observed := observer.New(zap.InfoLevel)
	reqLog := zap.New(core)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	body := []byte(`{"reasoning":{"effort":"high"},"tools":[{"type":"function"}],"messages":[{"role":"system"},{"role":"user","content":"hello"}]}`)

	logOpenAICacheDebugIngress(reqLog, c, "chat_completions", body)

	entries := observed.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	fields := entries[0].ContextMap()
	if fields["input_count"] != int64(0) {
		t.Fatalf("input_count = %v, want 0", fields["input_count"])
	}
	if fields["messages_count"] != int64(2) {
		t.Fatalf("messages_count = %v, want 2", fields["messages_count"])
	}
	if fields["input_item_source"] != "messages" {
		t.Fatalf("input_item_source = %v, want messages", fields["input_item_source"])
	}
	assertStringSliceField(t, fields["input_item_type_list"], []string{"system", "user"})
	assertIntSliceField(t, fields["input_item_length_list"], []int{17, 33})
	if fields["input_hash"] == "" {
		t.Fatalf("input_hash should be non-empty")
	}
	if fields["input_prefix_hash"] == "" {
		t.Fatalf("input_prefix_hash should be non-empty")
	}
	if fields["reasoning_effort"] != "high" {
		t.Fatalf("reasoning_effort = %v, want high", fields["reasoning_effort"])
	}
	if fields["tools_count"] != int64(1) {
		t.Fatalf("tools_count = %v, want 1", fields["tools_count"])
	}
}

func assertStringSliceField(t *testing.T, got any, want []string) {
	t.Helper()

	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("field = %v, want %v", got, want)
	}
}

func assertIntSliceField(t *testing.T, got any, want []int) {
	t.Helper()

	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("field = %v, want %v", got, want)
	}
}
