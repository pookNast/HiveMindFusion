package fusion

import (
	"testing"

	"github.com/pooknast/HiveMindFusion/internal/config"
)

func TestFamilyOf(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"claude-opus-4-8", "claude"},
		{"claude-sonnet-4-6", "claude"},
		{"claude-haiku-4-5", "claude"},
		{"gpt-5.5", "gpt"},
		{"gpt-5.4-mini", "gpt"},
		{"glm-5.1", "glm"},
		{"glm-4.5-air", "glm"},
		{"qwopus3.6-27b-v2", "qwen"},     // qwopus → qwen, not claude
		{"qwen2.5:0.5b", "qwen"},
		{"turboquant-primary", "qwen"},
		{"unknown-model", "claude"},       // fallback
	}
	for _, tt := range tests {
		if got := FamilyOf(tt.model); got != tt.expected {
			t.Errorf("FamilyOf(%q) = %q, want %q", tt.model, got, tt.expected)
		}
	}
}

func TestExtractTier(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"hivemind/fusion-frontier", "frontier"},
		{"hivemind/fusion-balanced", "balanced"},
		{"fusion-test", "test"},
		{"fusion-improve-advisor", "improve-advisor"},
		{"claude-opus-4-8", ""},
		{"gpt-5.5", ""},
	}
	for _, tt := range tests {
		if got := ExtractTier(tt.model); got != tt.expected {
			t.Errorf("ExtractTier(%q) = %q, want %q", tt.model, got, tt.expected)
		}
	}
}

func TestIsFusionModel(t *testing.T) {
	if !IsFusionModel("hivemind/fusion-frontier") {
		t.Error("expected hivemind/fusion-frontier to be a fusion model")
	}
	if !IsFusionModel("fusion-test") {
		t.Error("expected fusion-test to be a fusion model")
	}
	if IsFusionModel("claude-opus-4-8") {
		t.Error("expected claude-opus-4-8 to NOT be a fusion model")
	}
}

func TestResolvePanel(t *testing.T) {
	cp := config.FusionPanel{
		Panelists: []string{"a", "b", "c"},
		Judge:     "j",
		Synth:     "s",
		TimeoutMs: 90000,
	}
	p := ResolvePanel("frontier", cp)

	if p.MinQuorum != 2 {
		t.Errorf("MinQuorum = %d, want 2 (floor-of-2 for 3 panelists)", p.MinQuorum)
	}
	if p.Deliberator != "glm-5.1" {
		t.Errorf("Deliberator = %q, want default 'glm-5.1'", p.Deliberator)
	}

	// Single panelist → quorum 1
	cp2 := config.FusionPanel{
		Panelists: []string{"only-one"},
		Judge:     "j",
		Synth:     "s",
		TimeoutMs: 30000,
	}
	p2 := ResolvePanel("single", cp2)
	if p2.MinQuorum != 1 {
		t.Errorf("MinQuorum for single panelist = %d, want 1", p2.MinQuorum)
	}
}

func TestTransformForModel(t *testing.T) {
	spec := map[string]any{
		"role":          "a code reviewer",
		"task":          "review this code",
		"context":       "Go project",
		"constraints":   "be concise",
		"output_format": "bullet list",
		"input":         "func main() {}",
	}

	// Claude
	tr := TransformForModel(spec, "claude-opus-4-8")
	if tr.System == "" {
		t.Error("expected non-empty system prompt for claude")
	}
	if tr.User == "" {
		t.Error("expected non-empty user prompt for claude")
	}
	if tr.Params["effort"] != "xhigh" {
		t.Errorf("claude-opus effort = %v, want xhigh", tr.Params["effort"])
	}

	// GLM — blog-aligned params (temp=1.0, top_p=0.95, no repetition_penalty)
	tr2 := TransformForModel(spec, "glm-5.1")
	if tr2.Params["temperature"] != 1.0 {
		t.Errorf("glm temperature = %v, want 1.0", tr2.Params["temperature"])
	}
	if tr2.Params["top_p"] != 0.95 {
		t.Errorf("glm top_p = %v, want 0.95", tr2.Params["top_p"])
	}

	// GLM-5.2 effort via thinking object
	tr5 := TransformForModel(spec, "glm-5.2")
	thinking, ok := tr5.Params["thinking"].(map[string]any)
	if !ok {
		t.Fatal("expected thinking object for glm-5.2")
	}
	if thinking["effort"] != "high" {
		t.Errorf("glm-5.2 thinking effort = %v, want high", thinking["effort"])
	}

	// Qwen/qwopus
	tr3 := TransformForModel(spec, "qwopus3.6-27b-v2")
	if _, ok := tr3.Params["top_p"]; !ok {
		t.Error("expected top_p in qwen params")
	}
}

func TestExtractQuestion(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "you are helpful"},
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "first answer"},
		{Role: "user", Content: "second question"},
	}
	if got := extractQuestion(msgs); got != "second question" {
		t.Errorf("extractQuestion = %q, want 'second question'", got)
	}
}

func TestFormatResponses(t *testing.T) {
	responses := []BackendResponse{
		{Model: "a", Content: "response from a"},
		{Model: "b", Error: "timeout"},
	}
	out := formatResponses(responses)
	if !contains(out, "### a") || !contains(out, "response from a") {
		t.Error("expected model a content in formatted output")
	}
	if !contains(out, "### b") || !contains(out, "NO RESPONSE") {
		t.Error("expected model b error in formatted output")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
