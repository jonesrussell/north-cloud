package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/jonesrussell/north-cloud/enrichment/internal/api"
)

const (
	contentTypeHeader = "Content-Type"
	jsonContentType   = "application/json"
)

// Runner accepts validated enrichment work for asynchronous processing.
type Runner interface {
	Enqueue(ctx context.Context, request api.EnrichmentRequest) error
}

// NoopRunner satisfies the API contract until orchestration is wired in WP04.
type NoopRunner struct{}

// Enqueue records successful validation without starting enrichment work.
func (NoopRunner) Enqueue(context.Context, api.EnrichmentRequest) error {
	return nil
}

// Server owns HTTP routing for the enrichment service.
type Server struct {
	logger *slog.Logger
	runner Runner
}

// New constructs an HTTP server with injectable async work handling.
func New(logger *slog.Logger, runner Runner) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	if runner == nil {
		runner = NoopRunner{}
	}

	return &Server{
		logger: logger,
		runner: runner,
	}
}

// Handler returns the service HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /api/v1/enrich", s.handleEnrich)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "enrichment",
	})
}

func (s *Server) handleEnrich(w http.ResponseWriter, r *http.Request) {
	var request api.EnrichmentRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{Error: "invalid JSON request body"})
		return
	}

	if err := request.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, api.ErrorResponse{
			Error:  "validation failed",
			Fields: api.ValidationFields(err),
		})
		return
	}

	if err := s.runner.Enqueue(r.Context(), request); err != nil {
		s.logger.Error("failed to enqueue enrichment", slog.String("lead_id", request.LeadID), slog.Any("error", err))
		writeJSON(w, http.StatusInternalServerError, api.ErrorResponse{Error: "failed to accept enrichment request"})
		return
	}

	writeJSON(w, http.StatusAccepted, api.AcceptedResponse{Status: "accepted", LeadID: request.LeadID})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set(contentTypeHeader, jsonContentType)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil && !errors.Is(err, http.ErrAbortHandler) {
		slog.Default().Error("write response", slog.Any("error", err))
	}
}
