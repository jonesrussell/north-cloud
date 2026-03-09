package drift

const (
	// doubleFactor is the multiplier for determining high severity on single breaches.
	doubleFactor = 2.0
	// multiBreachMin is the minimum breach count that triggers high severity.
	multiBreachMin = 2
)

// SeverityFromSignals computes the overall severity based on drift signals.
//
// Rules:
//   - No breaches -> "low"
//   - 1 breach, value < 2x threshold -> "medium"
//   - 1 breach, value >= 2x threshold -> "high"
//   - 2+ breaches -> "high"
func SeverityFromSignals(signals []DriftSignal) string {
	var breachCount int
	var hasDoubleBreach bool

	for _, s := range signals {
		if !s.Breached {
			continue
		}
		breachCount++
		if s.Value >= s.Threshold*doubleFactor {
			hasDoubleBreach = true
		}
	}

	switch {
	case breachCount == 0:
		return "low"
	case breachCount >= multiBreachMin || hasDoubleBreach:
		return "high"
	default:
		return "medium"
	}
}
