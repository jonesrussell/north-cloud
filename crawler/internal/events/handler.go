// Package events provides event handling for source lifecycle events.
package events

import (
	"context"

	infraevents "github.com/north-cloud/infrastructure/events"
)

// EventHandler processes source lifecycle events.
type EventHandler interface {
	HandleSourceCreated(ctx context.Context, event infraevents.SourceEvent) error
	HandleSourceUpdated(ctx context.Context, event infraevents.SourceEvent) error
	HandleSourceDeleted(ctx context.Context, event infraevents.SourceEvent) error
	HandleSourceEnabled(ctx context.Context, event infraevents.SourceEvent) error
	HandleSourceDisabled(ctx context.Context, event infraevents.SourceEvent) error
}
