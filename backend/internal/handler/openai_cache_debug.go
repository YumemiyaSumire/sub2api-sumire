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

	reqLog.Info("openai.cache_debug_ingress",
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
		zap.String("prompt_cache_key", strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String())),
		zap.String("metadata_user_id", strings.TrimSpace(gjson.GetBytes(body, "metadata.user_id").String())),
		zap.Int("messages_count", len(gjson.GetBytes(body, "messages").Array())),
		zap.Int("input_count", len(gjson.GetBytes(body, "input").Array())),
		zap.String("reasoning_effort", strings.TrimSpace(gjson.GetBytes(body, "reasoning.effort").String())),
		zap.Int("tools_count", len(gjson.GetBytes(body, "tools").Array())),
	)
}

func logOpenAICacheDebugResult(reqLog *zap.Logger, endpoint string, result *service.OpenAIForwardResult, accountID int64) {
	if reqLog == nil || result == nil || !openAICacheDebugEnabled() {
		return
	}

	reqLog.Info("openai.cache_debug_result",
		zap.String("endpoint", endpoint),
		zap.String("usage_request_id", strings.TrimSpace(result.RequestID)),
		zap.Int64("account_id", accountID),
		zap.String("upstream_model", strings.TrimSpace(result.UpstreamModel)),
		zap.Int("cache_read_input_tokens", result.Usage.CacheReadInputTokens),
		zap.Int("cache_creation_input_tokens", result.Usage.CacheCreationInputTokens),
	)
}

func openAICacheDebugHashBytes(value []byte) string {
	if len(value) == 0 {
		return ""
	}

	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
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
