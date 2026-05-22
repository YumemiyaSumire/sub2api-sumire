package handler

import (
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

	reqLog.Info("openai.cache_debug_ingress",
		zap.String("endpoint", endpoint),
		zap.Int("body_bytes", len(body)),
		zap.String("session_id", strings.TrimSpace(c.GetHeader("session_id"))),
		zap.String("conversation_id", strings.TrimSpace(c.GetHeader("conversation_id"))),
		zap.String("prompt_cache_key", strings.TrimSpace(gjson.GetBytes(body, "prompt_cache_key").String())),
		zap.String("metadata_user_id", strings.TrimSpace(gjson.GetBytes(body, "metadata.user_id").String())),
		zap.Int("messages_count", len(gjson.GetBytes(body, "messages").Array())),
		zap.Int("input_count", len(gjson.GetBytes(body, "input").Array())),
	)
}

func logOpenAICacheDebugResult(reqLog *zap.Logger, endpoint string, result *service.ForwardResult, accountID int64) {
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
