// Package elasticsearch provides raw-HTTP operations against the community_alerts ES index.
// Layer: L1. Imports: domain (L0) + stdlib only.
package elasticsearch

import _ "embed"

//go:embed mapping.json
var indexMapping []byte

// CommunityAlertsMapping returns the JSON mapping for the community_alerts index.
// Source: kitty-specs/community-alert-pipeline-01KQZC7A/contracts/es-index-mapping.json
// _comment keys have been stripped; ES rejects unknown top-level keys at PUT time.
func CommunityAlertsMapping() []byte { return indexMapping }
