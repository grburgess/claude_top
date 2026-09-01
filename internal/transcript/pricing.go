package transcript

import "strings"

// pricing is USD per million tokens.
type pricing struct {
	in, out, cacheWrite, cacheRead float64
}

var priceTable = []struct {
	substr string
	p      pricing
}{
	{"opus", pricing{15, 75, 18.75, 1.5}},
	{"fable", pricing{15, 75, 18.75, 1.5}},
	{"sonnet", pricing{3, 15, 3.75, 0.3}},
	{"haiku", pricing{1, 5, 1.25, 0.1}},
}

// priceFor returns pricing for a model by substring match; ok=false for
// unknown models (zero cost).
func priceFor(model string) (pricing, bool) {
	for _, e := range priceTable {
		if strings.Contains(model, e.substr) {
			return e.p, true
		}
	}
	return pricing{}, false
}

// contextLimitFor returns the context window size for a model.
func contextLimitFor(model string) int64 {
	if strings.Contains(model, "[1m]") || strings.Contains(model, "fable") {
		return 1_000_000
	}
	return 200_000
}

// promoteLimit returns the next context-window tier; observed context
// exceeding a tier means the session actually runs the bigger window.
func promoteLimit(cur int64) int64 {
	if cur == 200_000 {
		return 1_000_000
	}
	return cur
}
