package feed

import (
	"context"
	"time"
)

// DueFeed represents a feed that is due for polling.
type DueFeed struct {
	SourceID string
	FeedURL  string
}

// RunPollingLoop polls all due feeds on a fixed interval.
// It blocks until ctx is cancelled and returns nil on clean shutdown.
//
// On each tick, listDue is called to get feeds that need polling.
// Each due feed is polled via PollFeed; errors are logged but do not
// stop the loop.
func (p *Poller) RunPollingLoop(
	ctx context.Context,
	interval time.Duration,
	listDue func(ctx context.Context) ([]DueFeed, error),
) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Poll once immediately at startup.
	p.pollDueFeeds(ctx, listDue)

	for {
		select {
		case <-ctx.Done():
			p.log.Info("feed polling loop stopped")
			return nil
		case <-ticker.C:
			p.pollDueFeeds(ctx, listDue)
		}
	}
}

// pollDueFeeds fetches the list of due feeds and polls each one.
func (p *Poller) pollDueFeeds(
	ctx context.Context,
	listDue func(ctx context.Context) ([]DueFeed, error),
) {
	feeds, err := listDue(ctx)
	if err != nil {
		p.log.Error("failed to list due feeds", "error", err.Error())
		return
	}

	if len(feeds) == 0 {
		return
	}

	p.log.Info("polling due feeds", "count", len(feeds))

	for i := range feeds {
		if pollErr := p.PollFeed(ctx, feeds[i].SourceID, feeds[i].FeedURL); pollErr != nil {
			p.log.Error("feed poll failed",
				"source_id", feeds[i].SourceID,
				"feed_url", feeds[i].FeedURL,
				"error", pollErr.Error(),
			)
		}
	}
}
