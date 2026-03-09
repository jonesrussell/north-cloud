package drift

// DriftSignal represents the result of evaluating a single drift metric.
type DriftSignal struct {
	// Metric is the type of drift metric: "kl_divergence", "psi", "cross_matrix".
	Metric string
	// Scope identifies what was measured: "global", "region:north_america", "domain:crime".
	Scope string
	// Value is the computed metric value.
	Value float64
	// Threshold is the configured threshold for this metric.
	Threshold float64
	// Breached is true if Value exceeds Threshold.
	Breached bool
	// Details holds metric-specific breakdown data.
	Details map[string]any
}

// Baseline holds precomputed distributions for drift comparison.
type Baseline struct {
	// ComputedAt is when this baseline was generated.
	ComputedAt string `json:"computed_at"`
	// WindowDays is the number of days in the rolling window.
	WindowDays int `json:"window_days"`
	// SampleCount is the total number of docs sampled.
	SampleCount int `json:"sample_count"`
	// CategoryDistribution maps scope (e.g. "global", "north_america") to topic->probability.
	CategoryDistribution map[string]map[string]float64 `json:"category_distribution"`
	// ConfidenceHistograms maps scope (e.g. "global", "crime") to 10-bin histogram.
	ConfidenceHistograms map[string][]float64 `json:"confidence_histograms"`
	// CrossMatrix maps region to category->proportion.
	CrossMatrix map[string]map[string]float64 `json:"cross_matrix"`
	// CrossMatrixCounts maps region to category->doc count (for sparse cell filtering).
	CrossMatrixCounts map[string]map[string]int `json:"cross_matrix_counts"`
}

// Thresholds holds the configured drift thresholds.
type Thresholds struct {
	KLDivergence    float64
	PSI             float64
	MatrixDeviation float64
}
