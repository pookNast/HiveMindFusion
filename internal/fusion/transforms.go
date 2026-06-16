package fusion

import (
	"fmt"
	"regexp"
	"strings"
)

// familyParams holds per-family sampling defaults.
var familyParams = map[string]map[string]any{
	"claude": {"effort": "high"},
	"gpt":    {"reasoning_effort": "medium"},
	"glm":    {"temperature": 1.0, "top_p": 0.95}, // ponytail: dropped repetition_penalty (llama.cpp-ism, not honored by Z.AI cloud) — values per GLM-5.2 blog (temp=1.0, top_p=0.95)
	"qwen":   {"temperature": 0.7, "top_p": 0.8},
}

// variantEffort maps specific model slugs to effort overrides.
var variantEffort = map[string]string{
	"claude-opus-4-8":   "xhigh",
	"claude-sonnet-4-6": "high",
	"claude-haiku-4-5":  "medium",
	"gpt-5.5":           "high",
	"gpt-5.4":           "medium",
	"gpt-5.4-mini":      "medium",
	"gpt-5.3-codex":     "high",
	"gpt-5.2":           "medium",
	"glm-5.2":           "high", // GLM-5.2 supports High/Max effort via thinking object (verified on coding/paas endpoint)
}

// FamilyOf detects model family from the slug.
// Order matters: qwopus contains "opus" so qwen is checked before claude.
func FamilyOf(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "qwen") || strings.Contains(m, "qwopus") ||
		strings.Contains(m, "ollama") || strings.Contains(m, "turboquant"):
		return "qwen"
	case strings.Contains(m, "claude") || strings.Contains(m, "opus") ||
		strings.Contains(m, "sonnet") || strings.Contains(m, "haiku"):
		return "claude"
	case strings.Contains(m, "gpt"):
		return "gpt"
	case strings.Contains(m, "glm"):
		return "glm"
	}
	// ponytail: unknown → claude-style — upgrade: add explicit fallback policy
	return "claude"
}

// TransformResult holds the per-model prompt and sampling params.
type TransformResult struct {
	System string
	User   string
	Params map[string]any
}

// TransformForModel renders a task spec into (system, user, params) for the model's family.
func TransformForModel(spec map[string]any, model string) TransformResult {
	family := FamilyOf(model)
	tmpl, ok := familyTemplates[family]
	if !ok {
		tmpl = familyTemplates["claude"]
	}

	subs := map[string]string{
		"role":            coerceStr(spec["role"], "an expert assistant"),
		"role_expansion":  coerceStr(spec["role"], "an expert assistant"),
		"task":            coerceStr(firstOf(spec, "task", "input"), ""),
		"context":         coerceStr(spec["context"], ""),
		"constraints":     coerceStr(spec["constraints"], ""),
		"output_format":   coerceStr(spec["output_format"], "Respond clearly and concisely."),
		"guidance_blocks": coerceStr(spec["guidance_blocks"], ""),
		"input":           coerceStr(spec["input"], ""),
	}

	rendered := renderTemplate(tmpl, subs)
	system, user := splitSystemUser(rendered)

	params := copyParams(family)
	if effort, ok := variantEffort[model]; ok {
		switch family {
		case "claude":
			params["effort"] = effort
		case "gpt":
			params["reasoning_effort"] = effort
		case "glm":
			// GLM-5.2 effort via thinking object — verified on Z.AI coding/paas endpoint.
			params["thinking"] = map[string]any{"type": "enabled", "effort": effort}
		}
	}

	return TransformResult{System: system, User: user, Params: params}
}

func coerceStr(value any, fallback string) string {
	if value == nil {
		return fallback
	}
	switch v := value.(type) {
	case string:
		if v == "" {
			return fallback
		}
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, fmt.Sprintf("- %v", item))
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func firstOf(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

var placeholderRe = regexp.MustCompile(`\{(\w+)\}`)

func renderTemplate(tmpl string, subs map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(tmpl, func(match string) string {
		key := match[1 : len(match)-1]
		if v, ok := subs[key]; ok {
			return v
		}
		return ""
	})
}

func splitSystemUser(rendered string) (string, string) {
	if idx := strings.Index(rendered, "[USER]"); idx >= 0 {
		system := strings.Replace(rendered[:idx], "[SYSTEM]", "", 1)
		user := strings.TrimSpace(rendered[idx+len("[USER]"):])
		return strings.TrimSpace(system), user
	}
	return strings.TrimSpace(strings.Replace(rendered, "[SYSTEM]", "", 1)), ""
}

func copyParams(family string) map[string]any {
	base, ok := familyParams[family]
	if !ok {
		base = familyParams["claude"]
	}
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = v
	}
	return out
}
