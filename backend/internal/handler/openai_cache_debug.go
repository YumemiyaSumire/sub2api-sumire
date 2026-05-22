package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const debugCacheKeysEnv = "SUB2API_DEBUG_CACHE_KEYS"

func openAICacheDebugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(debugCacheKeysEnv))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func logOpenAICacheDebugIngress(reqLog *zap.Logger, c *gin.Context, endpoint string, body []byte) {
	if reqLog == nil || !openAICacheDebugEnabled() {
		return
	}

	itemSource, items := openAICacheDebugItems(body)
	itemTypes, itemLengths := summarizeOpenAICacheDebugItems(items)
	cacheTraceID := ensureOpenAICacheDebugTraceID(c, endpoint, body)
	promptCacheKey := strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String())

	reqLog.Info("openai.cache_debug_ingress",
		zap.String("cache_trace_id", cacheTraceID),
		zap.String("endpoint", endpoint),
		zap.Int("body_bytes", len(body)),
		zap.String("full_body_hash", openAICacheDebugHashBytes(body)),
		zap.String("input_hash", openAICacheDebugHashRawItems(items)),
		zap.String("input_prefix_hash", openAICacheDebugHashRawItems(openAICacheDebugPrefixItems(items))),
		zap.String("input_item_source", itemSource),
		zap.Strings("input_item_type_list", itemTypes),
		zap.Ints("input_item_length_list", itemLengths),
		zap.String("session_id", strings.TrimSpace(c.GetHeader("session_id"))),
		zap.String("conversation_id", strings.TrimSpace(c.GetHeader("conversation_id"))),
		zap.String("prompt_cache_key", promptCacheKey),
		zap.String("prompt_cache_key_sha256", openAICacheDebugHashStringShort(promptCacheKey)),
		zap.String("metadata_user_id", strings.TrimSpace(gjson.GetBytes(body, "metadata.user_id").String())),
		zap.Int("messages_count", len(gjson.GetBytes(body, "messages").Array())),
		zap.Int("input_count", len(gjson.GetBytes(body, "input").Array())),
		zap.String("reasoning_effort", strings.TrimSpace(gjson.GetBytes(body, "reasoning.effort").String())),
		zap.Int("tools_count", len(gjson.GetBytes(body, "tools").Array())),
	)
}

func logOpenAICacheDebugResult(reqLog *zap.Logger, c *gin.Context, endpoint string, result *service.OpenAIForwardResult, accountID int64) {
	if reqLog == nil || result == nil || !openAICacheDebugEnabled() {
		return
	}

	reqLog.Info("openai.cache_debug_result",
		zap.String("cache_trace_id", openAICacheDebugTraceID(c)),
		zap.String("endpoint", endpoint),
		zap.String("usage_request_id", strings.TrimSpace(result.RequestID)),
		zap.Int64("account_id", accountID),
		zap.String("upstream_model", strings.TrimSpace(result.UpstreamModel)),
		zap.Int("cache_read_input_tokens", result.Usage.CacheReadInputTokens),
		zap.Int("cache_creation_input_tokens", result.Usage.CacheCreationInputTokens),
	)
}

func logOpenAICacheDebugSticky(reqLog *zap.Logger, c *gin.Context, endpoint string, sessionHash string, scheduleDecision service.OpenAIAccountScheduleDecision, accountID int64, accountName string, switchCount int) {
	if reqLog == nil || !openAICacheDebugEnabled() {
		return
	}

	reqLog.Info("openai.cache_debug_sticky",
		zap.String("cache_trace_id", openAICacheDebugTraceID(c)),
		zap.String("endpoint", endpoint),
		zap.String("session_hash", strings.TrimSpace(sessionHash)),
		zap.String("schedule_layer", strings.TrimSpace(scheduleDecision.Layer)),
		zap.Bool("sticky_previous_hit", scheduleDecision.StickyPreviousHit),
		zap.Bool("sticky_session_hit", scheduleDecision.StickySessionHit),
		zap.Int("candidate_count", scheduleDecision.CandidateCount),
		zap.Int("top_k", scheduleDecision.TopK),
		zap.Int64("selected_account_id", accountID),
		zap.String("selected_account_name", strings.TrimSpace(accountName)),
		zap.String("selected_account_type", strings.TrimSpace(scheduleDecision.SelectedAccountType)),
		zap.Int("switch_count", switchCount),
	)
}

func ensureOpenAICacheDebugTraceID(c *gin.Context, endpoint string, body []byte) string {
	if existing := openAICacheDebugTraceID(c); existing != "" {
		return existing
	}

	seed := strings.Join([]string{
		strings.TrimSpace(endpoint),
		strings.TrimSpace(openAICacheDebugHashBytes(body)),
		strings.TrimSpace(c.GetHeader("session_id")),
		strings.TrimSpace(c.GetHeader("conversation_id")),
		strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String()),
	}, "|")
	traceID := openAICacheDebugHashStringShort(seed)
	if traceID != "" && c != nil {
		c.Set(service.OpenAICacheTraceIDContextKey, traceID)
	}
	return traceID
}

func openAICacheDebugTraceID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	value, ok := c.Get(service.OpenAICacheTraceIDContextKey)
	if !ok {
		return ""
	}
	traceID, _ := value.(string)
	return strings.TrimSpace(traceID)
}

func openAICacheDebugHashBytes(value []byte) string {
	if len(value) == 0 {
		return ""
	}

	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func openAICacheDebugHashStringShort(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func openAICacheDebugHashRawItems(items []gjson.Result) string {
	if len(items) == 0 {
		return ""
	}

	hash := sha256.New()
	for _, item := range items {
		_, _ = hash.Write([]byte(item.Raw))
		_, _ = hash.Write([]byte{'\n'})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func openAICacheDebugItems(body []byte) (string, []gjson.Result) {
	input := gjson.GetBytes(body, "input")
	if input.Exists() {
		return "input", input.Array()
	}

	messages := gjson.GetBytes(body, "messages")
	if messages.Exists() {
		return "messages", messages.Array()
	}

	return "", nil
}

func openAICacheDebugPrefixItems(items []gjson.Result) []gjson.Result {
	if len(items) <= 1 {
		return nil
	}

	return items[:len(items)-1]
}

func summarizeOpenAICacheDebugItems(items []gjson.Result) ([]string, []int) {
	types := make([]string, 0, len(items))
	lengths := make([]int, 0, len(items))

	for _, item := range items {
		types = append(types, openAICacheDebugItemType(item))
		lengths = append(lengths, len(item.Raw))
	}

	return types, lengths
}

func openAICacheDebugItemType(item gjson.Result) string {
	itemType := strings.TrimSpace(item.Get("type").String())
	role := strings.TrimSpace(item.Get("role").String())

	switch {
	case itemType != "" && role != "":
		return itemType + ":" + role
	case itemType != "":
		return itemType
	case role != "":
		return role
	default:
		return openAICacheDebugGJSONType(item.Type)
	}
}

func openAICacheDebugGJSONType(itemType gjson.Type) string {
	switch itemType {
	case gjson.Null:
		return "null"
	case gjson.False:
		return "false"
	case gjson.Number:
		return "number"
	case gjson.String:
		return "string"
	case gjson.True:
		return "true"
	case gjson.JSON:
		return "json"
	default:
		return "unknown"
	}
}
