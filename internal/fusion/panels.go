package fusion

import (
	"strings"

	"github.com/pooknast/HiveMindFusion/internal/config"
)

// Panel holds a resolved fusion tier ready for orchestration.
type Panel struct {
	Name        string
	Panelists   []string
	Judge       string
	Synth       string
	Deliberator string
	TimeoutMs   int
	MinQuorum   int
}

// ResolvePanel converts a config panel into a runtime Panel with defaults applied.
func ResolvePanel(name string, cp config.FusionPanel) Panel {
	p := Panel{
		Name:        name,
		Panelists:   cp.Panelists,
		Judge:       cp.Judge,
		Synth:       cp.Synth,
		Deliberator: cp.Deliberator,
		TimeoutMs:   cp.TimeoutMs,
		MinQuorum:   cp.MinQuorum,
	}
	if p.Deliberator == "" {
		p.Deliberator = "glm-5.1"
	}
	if p.TimeoutMs == 0 {
		p.TimeoutMs = 60000
	}
	if p.MinQuorum == 0 {
		// Floor-of-2 rule: single-model panels need 1; multi-model needs >=2
		p.MinQuorum = min(2, len(p.Panelists))
	}
	return p
}

// IsFusionModel returns true if the model name routes to the fusion engine.
func IsFusionModel(model string) bool {
	return len(ExtractTier(model)) > 0
}

// ExtractTier strips "hivemind/fusion-" or "fusion-" prefix to get the tier name.
func ExtractTier(model string) string {
	const prefixLong = "hivemind/fusion-"
	const prefixShort = "fusion-"
	switch {
	case strings.HasPrefix(model, prefixLong):
		return model[len(prefixLong):]
	case strings.HasPrefix(model, prefixShort):
		return model[len(prefixShort):]
	}
	return ""
}
