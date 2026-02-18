package crawler

import (
	"github.com/jonesrussell/north-cloud/crawler/internal/content/rawcontent"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

// jobLoggerExtractionRecorder adapts a JobLogger to ExtractionRecorder for extraction quality metrics.
type jobLoggerExtractionRecorder struct {
	jl logs.JobLogger
}

// Ensure jobLoggerExtractionRecorder implements rawcontent.ExtractionRecorder.
var _ rawcontent.ExtractionRecorder = (*jobLoggerExtractionRecorder)(nil)

// RecordExtracted forwards to the job logger's RecordExtracted.
func (r *jobLoggerExtractionRecorder) RecordExtracted(emptyTitle, emptyBody bool) {
	r.jl.RecordExtracted(emptyTitle, emptyBody)
}

// newJobLoggerExtractionRecorder returns an ExtractionRecorder that records via the given JobLogger.
func newJobLoggerExtractionRecorder(jl logs.JobLogger) rawcontent.ExtractionRecorder {
	if jl == nil {
		return nil
	}
	return &jobLoggerExtractionRecorder{jl: jl}
}
