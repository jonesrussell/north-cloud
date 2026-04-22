// Package esmapping is the single source of truth for Elasticsearch field
// definitions of *_raw_content and *_classified_content indices.
//
// Services import this package; do not duplicate property maps under
// classifier/internal/elasticsearch/mappings or index-manager/.../mappings
// (enforced by task esmapping:check and tools/drift-detector.sh).
//
// Drift history: docs/generated/es-mapping-divergence.md
package esmapping
