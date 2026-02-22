package feed

import "context"

// SourceFeedDisabler manages feed disable/enable state via the source-manager API.
type SourceFeedDisabler interface {
	DisableFeed(ctx context.Context, sourceID, reason string) error
	EnableFeed(ctx context.Context, sourceID string) error
}
