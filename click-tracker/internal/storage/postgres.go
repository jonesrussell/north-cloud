package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/click-tracker/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Named constants to avoid magic numbers.
const (
	// columnsPerRow is the number of columns inserted per click event row.
	columnsPerRow = 9

	// insertBatchSize is the maximum number of rows per INSERT statement.
	insertBatchSize = 50

	// flushTimeout is the context timeout for each flush operation.
	flushTimeout = 5 * time.Second
)

// Buffer is a channel-based event buffer for non-blocking click event ingestion.
type Buffer struct {
	events chan domain.ClickEvent
	closed chan struct{}
	once   sync.Once
}

// NewBuffer creates a buffer with a buffered channel of the given capacity.
func NewBuffer(capacity int) *Buffer {
	return &Buffer{
		events: make(chan domain.ClickEvent, capacity),
		closed: make(chan struct{}),
	}
}

// Send performs a non-blocking send of an event into the buffer.
// It returns false if the buffer channel is full.
func (b *Buffer) Send(event domain.ClickEvent) bool {
	select {
	case b.events <- event:
		return true
	default:
		return false
	}
}

// Len returns the number of events currently in the buffer channel.
func (b *Buffer) Len() int {
	return len(b.events)
}

// Close signals the buffer to stop accepting events.
// It is safe to call multiple times.
func (b *Buffer) Close() {
	b.once.Do(func() {
		close(b.closed)
	})
}

// Store manages buffered writes of click events to PostgreSQL.
type Store struct {
	db             *sql.DB
	buffer         *Buffer
	log            infralogger.Logger
	flushInterval  time.Duration
	flushThreshold int
	wg             sync.WaitGroup
}

// NewStore creates a new Store that reads events from buffer and batch-inserts them.
func NewStore(
	db *sql.DB,
	buffer *Buffer,
	log infralogger.Logger,
	flushInterval time.Duration,
	flushThreshold int,
) *Store {
	return &Store{
		db:             db,
		buffer:         buffer,
		log:            log,
		flushInterval:  flushInterval,
		flushThreshold: flushThreshold,
	}
}

// Start launches the background goroutine that reads events and flushes batches.
func (s *Store) Start() {
	s.wg.Add(1)
	go s.flushLoop()
}

// Stop signals the buffer to close and waits for the flush goroutine to finish.
func (s *Store) Stop() {
	s.buffer.Close()
	s.wg.Wait()
}

// flushLoop reads events from the buffer, accumulates a batch, and flushes
// when the batch reaches flushThreshold or the flushInterval ticker fires.
func (s *Store) flushLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	batch := make([]domain.ClickEvent, 0, s.flushThreshold)

	for {
		select {
		case event := <-s.buffer.events:
			batch = append(batch, event)
			if len(batch) >= s.flushThreshold {
				s.flush(batch)
				batch = make([]domain.ClickEvent, 0, s.flushThreshold)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				s.flush(batch)
				batch = make([]domain.ClickEvent, 0, s.flushThreshold)
			}

		case <-s.buffer.closed:
			s.drain(&batch)
			if len(batch) > 0 {
				s.flush(batch)
			}
			return
		}
	}
}

// drain reads all remaining events from the buffer channel into the batch.
func (s *Store) drain(batch *[]domain.ClickEvent) {
	for {
		select {
		case event := <-s.buffer.events:
			*batch = append(*batch, event)
		default:
			return
		}
	}
}

// flush writes a batch of events to PostgreSQL in chunks of insertBatchSize.
func (s *Store) flush(batch []domain.ClickEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), flushTimeout)
	defer cancel()

	for start := 0; start < len(batch); start += insertBatchSize {
		end := start + insertBatchSize
		if end > len(batch) {
			end = len(batch)
		}

		if err := s.batchInsert(ctx, batch[start:end]); err != nil {
			s.log.Error("Failed to insert click events",
				infralogger.Error(err),
				infralogger.Int("batch_size", end-start),
			)
		}
	}

	s.log.Debug("Flushed click events",
		infralogger.Int("total", len(batch)),
	)
}

// batchInsert builds and executes a single INSERT statement with multiple value tuples.
func (s *Store) batchInsert(ctx context.Context, events []domain.ClickEvent) error {
	if len(events) == 0 {
		return nil
	}

	args := make([]any, 0, len(events)*columnsPerRow)
	var sb strings.Builder

	sb.WriteString("INSERT INTO click_events (query_id, result_id, position, page, " +
		"destination_hash, session_id, user_agent_hash, generated_at, clicked_at) VALUES ")

	for i := range events {
		if i > 0 {
			sb.WriteString(", ")
		}

		writeValueTuple(&sb, i)

		args = append(args,
			events[i].QueryID, events[i].ResultID, events[i].Position, events[i].Page,
			events[i].DestinationHash, events[i].SessionID, events[i].UserAgentHash,
			events[i].GeneratedAt, events[i].ClickedAt,
		)
	}

	_, err := s.db.ExecContext(ctx, sb.String(), args...)
	if err != nil {
		return fmt.Errorf("exec batch insert: %w", err)
	}

	return nil
}

// Placeholder column offsets within a single row tuple (1-indexed for PostgreSQL $N params).
const (
	colQueryID         = 1
	colResultID        = 2
	colPosition        = 3
	colPage            = 4
	colDestinationHash = 5
	colSessionID       = 6
	colUserAgentHash   = 7
	colGeneratedAt     = 8
	colClickedAt       = 9
)

// writeValueTuple writes a single ($1, $2, ..., $9) placeholder tuple to the builder,
// offset by the row index.
func writeValueTuple(sb *strings.Builder, rowIndex int) {
	base := rowIndex * columnsPerRow
	fmt.Fprintf(sb, "($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
		base+colQueryID, base+colResultID, base+colPosition, base+colPage,
		base+colDestinationHash, base+colSessionID, base+colUserAgentHash,
		base+colGeneratedAt, base+colClickedAt,
	)
}
