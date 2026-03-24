package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Buffer additional tests ---

func TestNewBuffer_Capacity(t *testing.T) {
	t.Helper()

	buf := NewBuffer(5)

	assert.NotNil(t, buf)
	assert.Equal(t, 0, buf.Len())
}

func TestBuffer_Len_Tracks(t *testing.T) {
	t.Helper()

	buf := NewBuffer(5)
	assert.Equal(t, 0, buf.Len())

	buf.Send(testEvent("q1", "r1"))
	assert.Equal(t, 1, buf.Len())

	buf.Send(testEvent("q2", "r2"))
	assert.Equal(t, 2, buf.Len())
}

func TestBuffer_Close_Idempotent(t *testing.T) {
	t.Helper()

	buf := NewBuffer(1)

	// Must not panic on multiple closes.
	buf.Close()
	buf.Close()
	buf.Close()
}

// --- writeValueTuple tests ---

func TestWriteValueTuple_FirstRow(t *testing.T) {
	t.Helper()

	var sb strings.Builder
	writeValueTuple(&sb, 0)

	assert.Equal(t, "($1, $2, $3, $4, $5, $6, $7, $8, $9)", sb.String())
}

func TestWriteValueTuple_SecondRow(t *testing.T) {
	t.Helper()

	var sb strings.Builder
	writeValueTuple(&sb, 1)

	assert.Equal(t, "($10, $11, $12, $13, $14, $15, $16, $17, $18)", sb.String())
}

func TestWriteValueTuple_ThirdRow(t *testing.T) {
	t.Helper()

	var sb strings.Builder
	writeValueTuple(&sb, 2)

	assert.Equal(t, "($19, $20, $21, $22, $23, $24, $25, $26, $27)", sb.String())
}

// --- Store constructor test ---

func TestNewStore_Fields(t *testing.T) {
	t.Helper()

	db, _, setupErr := sqlmock.New()
	require.NoError(t, setupErr)

	defer db.Close()

	buf := NewBuffer(10)
	log := infralogger.NewNop()

	store := NewStore(db, buf, log, 2*time.Second, 100)

	assert.Equal(t, 2*time.Second, store.flushInterval)
	assert.Equal(t, 100, store.flushThreshold)
}

// --- batchInsert tests ---

func TestBatchInsert_SingleEvent(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	require.NoError(t, setupErr)

	defer db.Close()

	store := NewStore(db, NewBuffer(10), infralogger.NewNop(), time.Second, 5)

	mock.ExpectExec("INSERT INTO click_events").
		WithArgs(
			"q1", "r1", 1, 1,
			"desthash", "sess1", "uahash",
			sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	events := []domain.ClickEvent{testEvent("q1", "r1")}
	store.flush(events)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchInsert_MultipleEvents(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	require.NoError(t, setupErr)

	defer db.Close()

	store := NewStore(db, NewBuffer(10), infralogger.NewNop(), time.Second, 5)

	mock.ExpectExec("INSERT INTO click_events").
		WithArgs(
			"q1", "r1", 1, 1, "desthash", "sess1", "uahash", sqlmock.AnyArg(), sqlmock.AnyArg(),
			"q2", "r2", 2, 1, "desthash", "sess2", "uahash", sqlmock.AnyArg(), sqlmock.AnyArg(),
			"q3", "r3", 3, 1, "desthash", "sess3", "uahash", sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 3))

	events := []domain.ClickEvent{
		testEvent("q1", "r1"),
		testEventWithPos("q2", "r2", "sess2", 2),
		testEventWithPos("q3", "r3", "sess3", 3),
	}
	store.flush(events)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchInsert_EmptySlice(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	require.NoError(t, setupErr)

	defer db.Close()

	store := NewStore(db, NewBuffer(10), infralogger.NewNop(), time.Second, 5)

	// No DB calls expected for empty batch.
	store.flush([]domain.ClickEvent{})

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchInsert_DBError(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	require.NoError(t, setupErr)

	defer db.Close()

	store := NewStore(db, NewBuffer(10), infralogger.NewNop(), time.Second, 5)

	mock.ExpectExec("INSERT INTO click_events").
		WithArgs(
			"q1", "r1", 1, 1,
			"desthash", "sess1", "uahash",
			sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnError(assert.AnError)

	events := []domain.ClickEvent{testEvent("q1", "r1")}
	// flush logs the error but must not panic.
	store.flush(events)

	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- drain tests ---

func TestDrain_EmptyBuffer(t *testing.T) {
	t.Helper()

	db, _, setupErr := sqlmock.New()
	require.NoError(t, setupErr)

	defer db.Close()

	store := NewStore(db, NewBuffer(10), infralogger.NewNop(), time.Second, 5)

	batch := make([]domain.ClickEvent, 0, 5)
	store.drain(&batch)

	assert.Empty(t, batch)
}

func TestDrain_WithEvents(t *testing.T) {
	t.Helper()

	db, _, setupErr := sqlmock.New()
	require.NoError(t, setupErr)

	defer db.Close()

	buf := NewBuffer(10)
	store := NewStore(db, buf, infralogger.NewNop(), time.Second, 5)

	buf.Send(testEvent("q1", "r1"))
	buf.Send(testEventWithPos("q2", "r2", "sess2", 2))

	batch := make([]domain.ClickEvent, 0, 5)
	store.drain(&batch)

	require.Len(t, batch, 2)
	assert.Equal(t, "q1", batch[0].QueryID)
	assert.Equal(t, "q2", batch[1].QueryID)
}

// --- Start/Stop integration tests ---

func TestStore_StartStop_DrainsPending(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	require.NoError(t, setupErr)

	defer db.Close()

	buf := NewBuffer(100)

	// High threshold so only drain-on-close triggers flush.
	store := NewStore(db, buf, infralogger.NewNop(), 50*time.Millisecond, 1000)

	buf.Send(testEvent("q1", "r1"))
	buf.Send(testEventWithPos("q2", "r2", "sess2", 2))

	mock.ExpectExec("INSERT INTO click_events").
		WithArgs(
			"q1", "r1", 1, 1, "desthash", "sess1", "uahash", sqlmock.AnyArg(), sqlmock.AnyArg(),
			"q2", "r2", 2, 1, "desthash", "sess2", "uahash", sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 2))

	store.Start()
	store.Stop()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStore_FlushLoop_ThresholdTrigger(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	require.NoError(t, setupErr)

	defer db.Close()

	buf := NewBuffer(100)

	// Threshold of 2, long interval — threshold should trigger flush.
	store := NewStore(db, buf, infralogger.NewNop(), 10*time.Second, 2)

	mock.ExpectExec("INSERT INTO click_events").
		WithArgs(
			"q1", "r1", 1, 1, "desthash", "sess1", "uahash", sqlmock.AnyArg(), sqlmock.AnyArg(),
			"q2", "r2", 2, 1, "desthash", "sess2", "uahash", sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 2))

	store.Start()

	buf.Send(testEvent("q1", "r1"))
	buf.Send(testEventWithPos("q2", "r2", "sess2", 2))

	// Give the flush loop time to process.
	time.Sleep(100 * time.Millisecond)

	store.Stop()

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStore_FlushLoop_IntervalTrigger(t *testing.T) {
	t.Helper()

	db, mock, setupErr := sqlmock.New()
	require.NoError(t, setupErr)

	defer db.Close()

	buf := NewBuffer(100)

	// High threshold, short interval — interval should trigger flush.
	store := NewStore(db, buf, infralogger.NewNop(), 50*time.Millisecond, 1000)

	mock.ExpectExec("INSERT INTO click_events").
		WithArgs(
			"q1", "r1", 1, 1, "desthash", "sess1", "uahash", sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	store.Start()

	buf.Send(testEvent("q1", "r1"))

	// Wait for the interval ticker to fire.
	time.Sleep(150 * time.Millisecond)

	store.Stop()

	assert.NoError(t, mock.ExpectationsWereMet())
}

// --- test helpers ---

func testEvent(queryID, resultID string) domain.ClickEvent {
	return domain.ClickEvent{
		QueryID:         queryID,
		ResultID:        resultID,
		Position:        1,
		Page:            1,
		DestinationHash: "desthash",
		SessionID:       "sess1",
		UserAgentHash:   "uahash",
		GeneratedAt:     time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC),
		ClickedAt:       time.Date(2026, 3, 23, 10, 0, 1, 0, time.UTC),
	}
}

func testEventWithPos(queryID, resultID, sessionID string, position int) domain.ClickEvent {
	return domain.ClickEvent{
		QueryID:         queryID,
		ResultID:        resultID,
		Position:        position,
		Page:            1,
		DestinationHash: "desthash",
		SessionID:       sessionID,
		UserAgentHash:   "uahash",
		GeneratedAt:     time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC),
		ClickedAt:       time.Date(2026, 3, 23, 10, 0, 1, 0, time.UTC),
	}
}
