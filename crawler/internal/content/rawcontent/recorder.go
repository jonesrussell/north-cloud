// Package rawcontent provides extraction and indexing of raw content from HTML.
package rawcontent

// ExtractionRecorder records extraction quality for indexed items (e.g. empty title/body).
// Used to detect selector drift. Callers should use nil-safe pattern: if recorder != nil { recorder.RecordExtracted(...) }
type ExtractionRecorder interface {
	// RecordExtracted records one successfully indexed item. emptyTitle/emptyBody indicate
	// whether title or body was missing or negligible when the item was indexed.
	RecordExtracted(emptyTitle, emptyBody bool)
}
