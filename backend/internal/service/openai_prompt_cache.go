package service

import (
	"fmt"
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

const (
	openAIAutoPromptCacheEnv      = "SUB2API_OPENAI_AUTO_PROMPT_CACHE"
	openAIPromptCacheRetentionEnv = "SUB2API_OPENAI_PROMPT_CACHE_RETENTION"

	openAIPromptCacheRetentionDisabled = ""
	openAIPromptCacheRetentionDefault  = "24h"
	openAIPromptCacheRetentionMemory   = "in_memory"
)

type openAIPromptCacheOptions struct {
	Endpoint       string
	RequestedModel string
	UpstreamModel  string
	UserID         int64
	APIKeyID       int64
	ImageIntent    bool
}

type openAIPromptCacheApplyResult struct {
	PromptCacheKey                   string
	PromptCacheRetention             string
	PromptCacheKeyAutoInjected       bool
	PromptCacheRetentionAutoInjected bool
}

func openAIAutoPromptCacheEnabled() bool {
	return parseDebugEnvBool(os.Getenv(openAIAutoPromptCacheEnv))
}

func openAIPromptCacheForwardDebugEnabled() bool {
	return parseDebugEnvBool(os.Getenv("SUB2API_DEBUG_CACHE_KEYS"))
}

func openAIPromptCacheRetentionValue() (string, bool) {
	raw := strings.TrimSpace(os.Getenv(openAIPromptCacheRetentionEnv))
	if raw == "" {
		return openAIPromptCacheRetentionDefault, true
	}

	switch strings.ToLower(raw) {
	case "0", "false", "no", "off", "none", "disable", "disabled":
		return openAIPromptCacheRetentionDisabled, false
	case openAIPromptCacheRetentionDefault:
		return openAIPromptCacheRetentionDefault, true
	case openAIPromptCacheRetentionMemory:
		return openAIPromptCacheRetentionMemory, true
	default:
		return openAIPromptCacheRetentionDisabled, false
	}
}

func applyOpenAIAutoPromptCacheToMap(reqBody map[string]any, opts openAIPromptCacheOptions) openAIPromptCacheApplyResult {
	result := openAIPromptCacheApplyResult{}
	if reqBody == nil {
		return result
	}

	if existing, ok := reqBody["prompt_cache_key"].(string); ok {
		result.PromptCacheKey = strings.TrimSpace(existing)
	}
	if existing, ok := reqBody["prompt_cache_retention"].(string); ok {
		result.PromptCacheRetention = strings.TrimSpace(existing)
	}
	if !openAIAutoPromptCacheEnabled() {
		return result
	}

	if result.PromptCacheKey == "" {
		if key := buildOpenAIAutoPromptCacheKey(opts); key != "" {
			reqBody["prompt_cache_key"] = key
			result.PromptCacheKey = key
			result.PromptCacheKeyAutoInjected = true
		}
	}

	if result.PromptCacheRetention == "" && shouldInjectOpenAIPromptCacheRetention(opts) {
		if retention, ok := openAIPromptCacheRetentionValue(); ok {
			reqBody["prompt_cache_retention"] = retention
			result.PromptCacheRetention = retention
			result.PromptCacheRetentionAutoInjected = true
		}
	}

	return result
}

func restoreOpenAIClientPromptCacheRetention(reqBody map[string]any, clientRetention string, opts openAIPromptCacheOptions) bool {
	if reqBody == nil || !openAIAutoPromptCacheEnabled() || !shouldInjectOpenAIPromptCacheRetention(opts) {
		return false
	}

	retention := strings.TrimSpace(clientRetention)
	if retention == "" {
		return false
	}
	if existing, ok := reqBody["prompt_cache_retention"].(string); ok && strings.TrimSpace(existing) != "" {
		return false
	}

	reqBody["prompt_cache_retention"] = retention
	return true
}

func applyOpenAIAutoPromptCacheToResponsesRequest(req *apicompat.ResponsesRequest, opts openAIPromptCacheOptions) openAIPromptCacheApplyResult {
	result := openAIPromptCacheApplyResult{}
	if req == nil {
		return result
	}

	result.PromptCacheKey = strings.TrimSpace(req.PromptCacheKey)
	result.PromptCacheRetention = strings.TrimSpace(req.PromptCacheRetention)
	if !openAIAutoPromptCacheEnabled() {
		return result
	}

	if result.PromptCacheKey == "" {
		if key := buildOpenAIAutoPromptCacheKey(opts); key != "" {
			req.PromptCacheKey = key
			result.PromptCacheKey = key
			result.PromptCacheKeyAutoInjected = true
		}
	}

	if result.PromptCacheRetention == "" && shouldInjectOpenAIPromptCacheRetention(opts) {
		if retention, ok := openAIPromptCacheRetentionValue(); ok {
			req.PromptCacheRetention = retention
			result.PromptCacheRetention = retention
			result.PromptCacheRetentionAutoInjected = true
		}
	}

	return result
}

func buildOpenAIAutoPromptCacheKey(opts openAIPromptCacheOptions) string {
	endpoint := strings.TrimSpace(opts.Endpoint)
	requestedModel := strings.TrimSpace(opts.RequestedModel)
	if endpoint == "" || requestedModel == "" {
		return ""
	}
	return fmt.Sprintf("sub2api:openai-cache:user-%d:api-key-%d:%s:%s", opts.UserID, opts.APIKeyID, endpoint, requestedModel)
}

func shouldInjectOpenAIPromptCacheRetention(opts openAIPromptCacheOptions) bool {
	if opts.ImageIntent {
		return false
	}
	return isOpenAIPromptCacheGPTTextModel(opts.UpstreamModel)
}

func isOpenAIPromptCacheGPTTextModel(model string) bool {
	modelID := strings.ToLower(strings.TrimSpace(model))
	if modelID == "" {
		return false
	}
	if strings.Contains(modelID, "/") {
		parts := strings.Split(modelID, "/")
		modelID = strings.TrimSpace(parts[len(parts)-1])
	}
	if !strings.HasPrefix(modelID, "gpt-") {
		return false
	}
	return !isOpenAIImageBillingModelAlias(modelID)
}

func logOpenAIPromptCacheForwardDebug(result openAIPromptCacheApplyResult, opts openAIPromptCacheOptions) {
	if !openAIPromptCacheForwardDebugEnabled() {
		return
	}

	logger.L().Info("openai.cache_debug_forward",
		zap.String("endpoint", strings.TrimSpace(opts.Endpoint)),
		zap.String("prompt_cache_key", result.PromptCacheKey),
		zap.String("prompt_cache_retention", result.PromptCacheRetention),
		zap.Bool("prompt_cache_key_auto_injected", result.PromptCacheKeyAutoInjected),
		zap.Bool("prompt_cache_retention_auto_injected", result.PromptCacheRetentionAutoInjected),
		zap.String("upstream_model", strings.TrimSpace(opts.UpstreamModel)),
		zap.Bool("image_intent", opts.ImageIntent),
		zap.Int64("user_id", opts.UserID),
		zap.Int64("api_key_id", opts.APIKeyID),
		zap.String("requested_model", strings.TrimSpace(opts.RequestedModel)),
	)
}
