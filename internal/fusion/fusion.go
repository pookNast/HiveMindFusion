// Package fusion implements the multi-model consensus orchestrator.
//
// Pipeline: deliberate → fan-out → judge → synthesize.
// Each step calls backends directly via an injected BackendCaller.
package fusion

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/pooknast/HiveMindFusion/internal/config"
)

// Message is an OpenAI-compatible chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// BackendResponse holds one model's reply.
type BackendResponse struct {
	Model     string `json:"model"`
	Content   string `json:"content"`
	Tokens    int    `json:"tokens"`
	LatencyMs int    `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// BackendCaller calls a specific backend model directly.
// The gateway injects this so fusion routes through the proxy's own transport.
type BackendCaller func(ctx context.Context, model string, messages []Message) (BackendResponse, error)

// StreamChunk is a single SSE delta from the synthesizer.
type StreamChunk struct {
	Content      string
	FinishReason string // "stop" on final, empty otherwise
}

// Engine holds resolved panels and the backend caller.
type Engine struct {
	panels map[string]Panel
	call   BackendCaller
}

// New creates a fusion engine from config.
func New(cfg config.Fusion, caller BackendCaller) *Engine {
	panels := make(map[string]Panel, len(cfg.Panels))
	for name, cp := range cfg.Panels {
		panels[name] = ResolvePanel(name, cp)
	}
	return &Engine{panels: panels, call: caller}
}

// Reload updates panels from new config (called on SIGHUP).
func (e *Engine) Reload(cfg config.Fusion) {
	panels := make(map[string]Panel, len(cfg.Panels))
	for name, cp := range cfg.Panels {
		panels[name] = ResolvePanel(name, cp)
	}
	e.panels = panels
}

// HasTier returns true if the tier exists.
func (e *Engine) HasTier(tier string) bool {
	_, ok := e.panels[tier]
	return ok
}

// TierNames returns all configured tier names.
func (e *Engine) TierNames() []string {
	names := make([]string, 0, len(e.panels))
	for name := range e.panels {
		names = append(names, name)
	}
	return names
}

// Panels returns panel details for the /panels endpoint.
func (e *Engine) Panels() map[string]Panel {
	return e.panels
}

// maxPanelistChars truncates panelist responses before judge to avoid context overflow.
const maxPanelistChars = 12000 // ~3000 tokens * 4 chars

// RunFusion executes the full pipeline and returns an OpenAI-compatible response.
func (e *Engine) RunFusion(ctx context.Context, tier string, messages []Message) (map[string]any, error) {
	panel, ok := e.panels[tier]
	if !ok {
		return nil, fmt.Errorf("unknown fusion tier: %s", tier)
	}

	timeout := time.Duration(panel.TimeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	question := extractQuestion(messages)
	start := time.Now()

	// 0. Deliberate
	taskSpec := e.deliberate(ctx, messages, panel.Deliberator)

	// 1. Fan out
	responses := e.fanOut(ctx, panel, taskSpec)
	valid := filterValid(responses)

	if len(valid) < panel.MinQuorum {
		log.Printf("[fusion] quorum not met: %d/%d valid, need %d", len(valid), len(responses), panel.MinQuorum)
		return map[string]any{
			"error": fmt.Sprintf("fusion quorum not met: %d/%d panelists answered, need %d",
				len(valid), len(responses), panel.MinQuorum),
		}, nil
	}

	// 2. Judge
	analysis, judgeOK := e.runJudge(ctx, question, valid, panel.Judge)

	// 3. Synthesize (non-streaming: collect full text)
	synthText := e.synthesize(ctx, question, valid, analysis, panel.Synth)

	elapsedMs := int(time.Since(start).Milliseconds())
	logSummary(tier, elapsedMs, responses, panel.Judge, judgeOK, panel.Synth)

	totalPromptTokens := 0
	for _, r := range responses {
		totalPromptTokens += r.Tokens
	}
	completionTokens := len(synthText) / 4

	return map[string]any{
		"id":      fmt.Sprintf("fusion-%d", time.Now().UnixNano()%1e12),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   "hivemind/fusion-" + tier,
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]string{"role": "assistant", "content": synthText},
			"finish_reason": "stop",
		}},
		"usage": map[string]int{
			"prompt_tokens":     totalPromptTokens,
			"completion_tokens": completionTokens,
			"total_tokens":      totalPromptTokens + completionTokens,
		},
		"_meta": map[string]any{
			"tier":       tier,
			"elapsed_ms": elapsedMs,
			"judge":      panel.Judge,
			"synth":      panel.Synth,
		},
	}, nil
}

// RunFusionStream executes the pipeline and sends SSE chunks to the channel.
// The channel is closed when done. Deliberate/fan-out/judge are non-streaming;
// only the synthesizer streams.
func (e *Engine) RunFusionStream(ctx context.Context, tier string, messages []Message, ch chan<- StreamChunk) error {
	defer close(ch)

	panel, ok := e.panels[tier]
	if !ok {
		return fmt.Errorf("unknown fusion tier: %s", tier)
	}

	timeout := time.Duration(panel.TimeoutMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	question := extractQuestion(messages)
	start := time.Now()

	// 0. Deliberate
	taskSpec := e.deliberate(ctx, messages, panel.Deliberator)

	// 1. Fan out
	responses := e.fanOut(ctx, panel, taskSpec)
	valid := filterValid(responses)

	if len(valid) < panel.MinQuorum {
		log.Printf("[fusion] quorum not met (stream): %d/%d valid, need %d", len(valid), len(responses), panel.MinQuorum)
		ch <- StreamChunk{Content: fmt.Sprintf("[Fusion error: quorum not met — %d/%d panelists answered, need %d]",
			len(valid), len(responses), panel.MinQuorum)}
		ch <- StreamChunk{FinishReason: "stop"}
		return nil
	}

	// 2. Judge
	analysis, judgeOK := e.runJudge(ctx, question, valid, panel.Judge)

	// 3. Synthesize — stream the result
	synthText := e.synthesize(ctx, question, valid, analysis, panel.Synth)

	// ponytail: chunk the synth output into word-boundary deltas — upgrade: true streaming from backend
	for _, word := range strings.Fields(synthText) {
		ch <- StreamChunk{Content: word + " "}
	}
	ch <- StreamChunk{FinishReason: "stop"}

	elapsedMs := int(time.Since(start).Milliseconds())
	logSummary(tier, elapsedMs, responses, panel.Judge, judgeOK, panel.Synth)
	return nil
}

// deliberate calls the deliberator model to parse intent into a canonical task spec.
func (e *Engine) deliberate(ctx context.Context, messages []Message, deliberatorModel string) map[string]any {
	rawInput := extractQuestion(messages)
	filled := strings.Replace(deliberatorPrompt, "{input}", rawInput, 1)

	resp, err := e.call(ctx, deliberatorModel, []Message{{Role: "user", Content: filled}})
	if err != nil || resp.Error != "" || resp.Content == "" {
		log.Printf("[fusion] deliberator failed: err=%v resp_err=%s — using fallback spec", err, resp.Error)
		return fallbackSpec(rawInput)
	}

	raw := strings.TrimSpace(resp.Content)
	// Strip code fences
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		if len(lines) > 2 && strings.HasPrefix(lines[len(lines)-1], "```") {
			raw = strings.Join(lines[1:len(lines)-1], "\n")
		} else if len(lines) > 1 {
			raw = strings.Join(lines[1:], "\n")
		}
	}
	// Find first { ... last }
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}

	var taskSpec map[string]any
	if err := json.Unmarshal([]byte(raw), &taskSpec); err != nil {
		log.Printf("[fusion] deliberator parse error: %s — using fallback spec", err)
		return fallbackSpec(rawInput)
	}
	if _, ok := taskSpec["input"]; !ok {
		taskSpec["input"] = rawInput
	}
	log.Printf("[fusion] deliberator OK (model=%s, tokens=%d)", deliberatorModel, resp.Tokens)
	return taskSpec
}

func fallbackSpec(rawInput string) map[string]any {
	return map[string]any{
		"role":            "an expert assistant",
		"task":            rawInput,
		"context":         "",
		"constraints":     []any{},
		"output_format":   "Respond clearly and concisely.",
		"guidance_blocks": "",
		"input":           rawInput,
	}
}

// fanOut calls all panelists in parallel with per-model transforms.
func (e *Engine) fanOut(ctx context.Context, panel Panel, taskSpec map[string]any) []BackendResponse {
	results := make([]BackendResponse, len(panel.Panelists))
	var wg sync.WaitGroup

	for i, model := range panel.Panelists {
		wg.Add(1)
		go func(idx int, m string) {
			defer wg.Done()
			tr := TransformForModel(taskSpec, m)
			msgs := []Message{{Role: "user", Content: tr.User}}
			if tr.System != "" {
				msgs = append([]Message{{Role: "system", Content: tr.System}}, msgs...)
			}
			resp, err := e.call(ctx, m, msgs)
			if err != nil {
				results[idx] = BackendResponse{
					Model: m,
					Error: fmt.Sprintf("call failed: %s", err),
				}
				return
			}
			resp.Model = m // ensure model name is set
			results[idx] = resp
		}(i, model)
	}
	wg.Wait()

	succeeded := 0
	for _, r := range results {
		if r.Error == "" && r.Content != "" {
			succeeded++
		}
	}
	log.Printf("[fusion] fan_out complete: %d/%d panelists succeeded", succeeded, len(results))
	return results
}

// runJudge runs the judge pass for structural extraction.
func (e *Engine) runJudge(ctx context.Context, question string, responses []BackendResponse, judgeModel string) (map[string]any, bool) {
	responsesText := formatResponses(responses)
	filled := strings.Replace(
		strings.Replace(judgePrompt, "{question}", question, 1),
		"{responses}", responsesText, 1,
	)

	judgeSpec := map[string]any{
		"role":          "a consensus analyst evaluating panelist responses",
		"task":          "Analyze panelist responses and extract consensus, contradictions, and insights.",
		"output_format": "JSON object per the schema in the instructions",
		"input":         filled,
	}
	tr := TransformForModel(judgeSpec, judgeModel)
	msgs := []Message{{Role: "user", Content: tr.User}}
	if tr.System != "" {
		msgs = append([]Message{{Role: "system", Content: tr.System}}, msgs...)
	}

	resp, err := e.call(ctx, judgeModel, msgs)
	if err != nil || resp.Error != "" || resp.Content == "" {
		log.Printf("[fusion] judge failed: err=%v resp_err=%s", err, resp.Error)
		return nil, false
	}

	raw := strings.TrimSpace(resp.Content)
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		if len(lines) > 2 && strings.HasPrefix(lines[len(lines)-1], "```") {
			raw = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var analysis map[string]any
	if err := json.Unmarshal([]byte(raw), &analysis); err != nil {
		log.Printf("[fusion] judge returned non-JSON, using raw text")
		return map[string]any{"raw_text": resp.Content, "parse_error": true}, true
	}
	log.Printf("[fusion] judge OK (model=%s, tokens=%d)", judgeModel, resp.Tokens)
	return analysis, true
}

// synthesize runs the synthesizer and returns the full text.
func (e *Engine) synthesize(ctx context.Context, question string, responses []BackendResponse, analysis map[string]any, synthModel string) string {
	responsesText := formatResponses(responses)
	analysisText := "Judge analysis unavailable — synthesize directly from responses."
	if analysis != nil {
		if b, err := json.MarshalIndent(analysis, "", "  "); err == nil {
			analysisText = string(b)
		}
	}

	filled := strings.Replace(
		strings.Replace(
			strings.Replace(synthesizerPrompt, "{question}", question, 1),
			"{responses}", responsesText, 1,
		),
		"{analysis}", analysisText, 1,
	)

	synthSpec := map[string]any{
		"role":          "a synthesis expert producing the final unified answer",
		"task":          "Synthesize the panelist responses and judge analysis into one coherent answer.",
		"output_format": "The refined, coherent final answer in natural prose",
		"input":         filled,
	}
	tr := TransformForModel(synthSpec, synthModel)
	msgs := []Message{{Role: "user", Content: tr.User}}
	if tr.System != "" {
		msgs = append([]Message{{Role: "system", Content: tr.System}}, msgs...)
	}

	resp, err := e.call(ctx, synthModel, msgs)
	if err != nil || resp.Error != "" {
		log.Printf("[fusion] synthesizer failed: err=%v resp_err=%s", err, resp.Error)
		return "[Fusion synthesis failed]"
	}
	return resp.Content
}

// --- helpers ---

func extractQuestion(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func filterValid(responses []BackendResponse) []BackendResponse {
	var valid []BackendResponse
	for _, r := range responses {
		if r.Error == "" && r.Content != "" {
			valid = append(valid, r)
		}
	}
	return valid
}

func formatResponses(responses []BackendResponse) string {
	var parts []string
	for _, r := range responses {
		if r.Error != "" || r.Content == "" {
			parts = append(parts, fmt.Sprintf("### %s\n[NO RESPONSE — error: %s]", r.Model, r.Error))
		} else {
			content := r.Content
			if len(content) > maxPanelistChars {
				content = content[:maxPanelistChars] + "\n[...truncated...]"
			}
			parts = append(parts, fmt.Sprintf("### %s\n%s", r.Model, content))
		}
	}
	return strings.Join(parts, "\n\n---\n\n")
}

func logSummary(tier string, elapsedMs int, responses []BackendResponse, judgeModel string, judgeOK bool, synthModel string) {
	panelists := make([]string, len(responses))
	for i, r := range responses {
		status := "ok"
		if r.Error != "" {
			status = "FAIL"
		}
		panelists[i] = fmt.Sprintf("%s=%s", r.Model, status)
	}
	log.Printf("[fusion] tier=%s elapsed=%dms panelists=[%s] judge=%s(ok=%v) synth=%s",
		tier, elapsedMs, strings.Join(panelists, ", "), judgeModel, judgeOK, synthModel)
}
