package service

import (
	"net/http"
	"strings"

	"github.com/tidwall/sjson"
)

type openAIModelTuningAlias struct {
	BaseModel   string
	Effort      string
	ServiceTier string
}

func resolveOpenAIModelTuningAlias(model string) openAIModelTuningAlias {
	if alias := parseOpenAIReasoningModelAlias(model); alias.Effort != "" {
		return openAIModelTuningAlias{
			BaseModel: alias.BaseModel,
			Effort:    alias.Effort,
		}
	}

	baseModel, suffix, ok := splitOpenAIModelAliasSuffix(model)
	if !ok {
		return openAIModelTuningAlias{}
	}
	if !strings.EqualFold(suffix, "fast") {
		return openAIModelTuningAlias{}
	}
	if normalizeKnownOpenAICodexModel(baseModel) != "gpt-5.4-mini" {
		return openAIModelTuningAlias{}
	}
	return openAIModelTuningAlias{
		BaseModel:   baseModel,
		Effort:      "low",
		ServiceTier: "priority",
	}
}

func splitOpenAIModelAliasSuffix(model string) (baseModel, suffix string, ok bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", "", false
	}

	lastSep := -1
	for _, sep := range []string{"-", "_", " "} {
		if idx := strings.LastIndex(model, sep); idx > lastSep {
			lastSep = idx
		}
	}
	if lastSep <= 0 || lastSep >= len(model)-1 {
		return "", "", false
	}

	baseModel = strings.TrimSpace(model[:lastSep])
	suffix = strings.TrimSpace(model[lastSep+1:])
	if baseModel == "" || strings.HasSuffix(baseModel, "/") || suffix == "" {
		return "", "", false
	}
	return baseModel, suffix, true
}

func injectOpenAIServiceTier(reqBody map[string]any, tier string) bool {
	tierPtr := normalizeOpenAIServiceTier(tier)
	if reqBody == nil || tierPtr == nil {
		return false
	}
	if extractOpenAIServiceTier(reqBody) != nil {
		return false
	}
	reqBody["service_tier"] = *tierPtr
	return true
}

func injectOpenAIServiceTierBytes(body []byte, tier string) ([]byte, bool, error) {
	tierPtr := normalizeOpenAIServiceTier(tier)
	if len(body) == 0 || tierPtr == nil {
		return body, false, nil
	}
	if extractOpenAIServiceTierFromBody(body) != nil {
		return body, false, nil
	}
	updated, err := sjson.SetBytes(body, "service_tier", *tierPtr)
	if err != nil {
		return body, false, err
	}
	return updated, true, nil
}

func isOpenAIUnsupportedServiceTier(statusCode int, upstreamMsg string, upstreamBody []byte) bool {
	if statusCode != http.StatusBadRequest {
		return false
	}

	combined := strings.ToLower(strings.TrimSpace(upstreamMsg + "\n" + string(upstreamBody)))
	if !strings.Contains(combined, "service_tier") {
		return false
	}
	return strings.Contains(combined, "unsupported parameter") ||
		strings.Contains(combined, "not supported") ||
		strings.Contains(combined, "unknown parameter") ||
		strings.Contains(combined, "unknown field")
}
