package aiverify

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// Action represents what to do with a verified record.
type Action string

const (
	// ActionAutoVerify marks a record as verified automatically.
	ActionAutoVerify Action = "auto_verify"
	// ActionAutoReject marks a record as rejected automatically.
	ActionAutoReject Action = "auto_reject"
	// ActionQueue leaves a record for manual review.
	ActionQueue Action = "queue"

	// entityTypePerson is the entity type constant for person records.
	entityTypePerson = "person"

	delayBetweenCalls = 500 * time.Millisecond
)

// Verifier abstracts the LLM call for testing.
type Verifier interface {
	Verify(ctx context.Context, input VerifyInput) (*VerifyResult, error)
}

// VerificationRecord holds a record fetched for verification.
type VerificationRecord struct {
	ID         string
	EntityType string // "person" or "band_office"
	Input      VerifyInput
}

// Repository abstracts DB operations for the worker.
type Repository interface {
	ListUnverifiedUnscoredPeople(ctx context.Context, limit int) ([]VerificationRecord, error)
	ListUnverifiedUnscoredBandOffices(ctx context.Context, limit int) ([]VerificationRecord, error)
	UpdatePersonVerificationResult(ctx context.Context, id string, confidence float64, issues string) error
	UpdateBandOfficeVerificationResult(ctx context.Context, id string, confidence float64, issues string) error
	VerifyPerson(ctx context.Context, id string) error
	VerifyBandOffice(ctx context.Context, id string) error
	AutoRejectPerson(ctx context.Context, id string) error
	AutoRejectBandOffice(ctx context.Context, id string) error
}

// WorkerConfig holds verification worker settings.
type WorkerConfig struct {
	Interval            time.Duration
	BatchSize           int
	AutoVerifyThreshold float64
	AutoRejectThreshold float64
}

// Worker runs the AI verification loop.
type Worker struct {
	repo     Repository
	verifier Verifier
	config   WorkerConfig
	logger   infralogger.Logger
}

// NewWorker creates a new verification worker.
func NewWorker(
	repo Repository,
	verifier Verifier,
	cfg WorkerConfig,
	log infralogger.Logger,
) *Worker {
	return &Worker{
		repo:     repo,
		verifier: verifier,
		config:   cfg,
		logger:   log,
	}
}

// ClassifyAction determines the action based on confidence and thresholds.
func ClassifyAction(confidence, verifyThreshold, rejectThreshold float64) Action {
	if confidence >= verifyThreshold {
		return ActionAutoVerify
	}
	if confidence < rejectThreshold {
		return ActionAutoReject
	}
	return ActionQueue
}

// Run starts the verification ticker. Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	w.logger.Info("verification worker started",
		infralogger.String("interval", w.config.Interval.String()),
		infralogger.Int("batch_size", w.config.BatchSize),
	)

	ticker := time.NewTicker(w.config.Interval)
	defer ticker.Stop()

	// Run immediately on start, then on each tick.
	w.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("verification worker stopped")
			return
		case <-ticker.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	start := time.Now()

	people, err := w.repo.ListUnverifiedUnscoredPeople(ctx, w.config.BatchSize)
	if err != nil {
		w.logger.Error("verification: list unscored people", infralogger.Error(err))
		return
	}

	offices, officesErr := w.repo.ListUnverifiedUnscoredBandOffices(ctx, w.config.BatchSize)
	if officesErr != nil {
		w.logger.Error("verification: list unscored band offices", infralogger.Error(officesErr))
		return
	}

	records := make([]VerificationRecord, 0, len(people)+len(offices))
	records = append(records, people...)
	records = append(records, offices...)

	processed := 0

	for i := range records {
		if ctx.Err() != nil {
			return
		}
		if i > 0 {
			time.Sleep(delayBetweenCalls)
		}
		w.processRecord(ctx, &records[i])
		processed++
	}

	w.logger.Info("verification.tick",
		infralogger.Int("batch_size", len(records)),
		infralogger.Int("processed", processed),
		infralogger.String("duration", time.Since(start).String()),
	)
}

func (w *Worker) processRecord(ctx context.Context, rec *VerificationRecord) {
	result, err := w.verifier.Verify(ctx, rec.Input)
	if err != nil {
		w.logger.Error("verification.error",
			infralogger.String("id", rec.ID),
			infralogger.String("type", rec.EntityType),
			infralogger.Error(err),
		)
		return
	}

	issuesJSON, marshalErr := json.Marshal(result.Issues)
	if marshalErr != nil {
		w.logger.Error("verification: marshal issues", infralogger.Error(marshalErr))
		return
	}

	if writeErr := w.updateResult(ctx, rec, result.Confidence, string(issuesJSON)); writeErr != nil {
		w.logger.Error("verification: update result", infralogger.Error(writeErr))
		return
	}

	action := ClassifyAction(
		result.Confidence,
		w.config.AutoVerifyThreshold,
		w.config.AutoRejectThreshold,
	)
	w.applyAction(ctx, rec, action, result)
}

func (w *Worker) updateResult(
	ctx context.Context,
	rec *VerificationRecord,
	confidence float64,
	issues string,
) error {
	if rec.EntityType == entityTypePerson {
		return w.repo.UpdatePersonVerificationResult(ctx, rec.ID, confidence, issues)
	}
	return w.repo.UpdateBandOfficeVerificationResult(ctx, rec.ID, confidence, issues)
}

func (w *Worker) applyAction(
	ctx context.Context,
	rec *VerificationRecord,
	action Action,
	result *VerifyResult,
) {
	var err error

	switch action {
	case ActionAutoVerify:
		err = w.autoVerify(ctx, rec)
		if err == nil {
			w.logger.Info("verification.auto_verified",
				infralogger.String("id", rec.ID),
				infralogger.String("type", rec.EntityType),
				infralogger.Float64("confidence", result.Confidence),
			)
		}
	case ActionAutoReject:
		err = w.autoReject(ctx, rec)
		if err == nil {
			w.logger.Info("verification.auto_rejected",
				infralogger.String("id", rec.ID),
				infralogger.String("type", rec.EntityType),
				infralogger.Float64("confidence", result.Confidence),
			)
		}
	case ActionQueue:
		w.logger.Info("verification.queued",
			infralogger.String("id", rec.ID),
			infralogger.String("type", rec.EntityType),
			infralogger.Float64("confidence", result.Confidence),
			infralogger.Int("issue_count", len(result.Issues)),
		)
	}

	if err != nil {
		w.logger.Error(fmt.Sprintf("verification: %s failed", action),
			infralogger.String("id", rec.ID),
			infralogger.Error(err),
		)
	}
}

func (w *Worker) autoVerify(ctx context.Context, rec *VerificationRecord) error {
	if rec.EntityType == entityTypePerson {
		return w.repo.VerifyPerson(ctx, rec.ID)
	}
	return w.repo.VerifyBandOffice(ctx, rec.ID)
}

func (w *Worker) autoReject(ctx context.Context, rec *VerificationRecord) error {
	if rec.EntityType == entityTypePerson {
		return w.repo.AutoRejectPerson(ctx, rec.ID)
	}
	return w.repo.AutoRejectBandOffice(ctx, rec.ID)
}
