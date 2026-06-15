package gateway

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/pook/hivemind/internal/config"
)

// buildFallbackChain groups backends by model name and sorts each group by priority
// (lower priority number = higher precedence). Backends with priority 0 sort last.
func buildFallbackChain(backends []config.Backend) map[string][]*config.Backend {
	chains := make(map[string][]*config.Backend)
	for i := range backends {
		b := &backends[i]
		chains[b.Model] = append(chains[b.Model], b)
	}
	for model := range chains {
		sort.SliceStable(chains[model], func(i, j int) bool {
			pi := chains[model][i].Priority
			pj := chains[model][j].Priority
			if pi == 0 {
				pi = 1<<31 - 1
			}
			if pj == 0 {
				pj = 1<<31 - 1
			}
			return pi < pj
		})
	}
	return chains
}

// logFallback records a fallback event.
func logFallback(model, skipped, selected, reason string) {
	log.Printf("[hivemind] fallback: model=%q skipped=%q selected=%q reason=%s",
		model, skipped, selected, reason)
}

// chainDesc returns a human-readable description of the fallback chain order.
func chainDesc(chain []*config.Backend) string {
	names := make([]string, len(chain))
	for i, b := range chain {
		names[i] = b.Name
	}
	return strings.Join(names, " → ")
}

// allUnavailableMsg returns a descriptive 503 message when every backend in the chain fails.
func allUnavailableMsg(model string, chain []*config.Backend) string {
	return fmt.Sprintf("all backends unavailable for model %q (chain: %s)", model, chainDesc(chain))
}
