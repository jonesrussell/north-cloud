package domain

import (
	"errors"
	"time"
)

const (
	minPollInterval = 30 * time.Minute
	maxPollInterval = 60 * time.Minute
)

// AcquisitionStrategy describes how alert-crawler fetches a source.
type AcquisitionStrategy string

const (
	AcquisitionRSS  AcquisitionStrategy = "rss"
	AcquisitionAtom AcquisitionStrategy = "atom"
	AcquisitionJSON AcquisitionStrategy = "json"
	AcquisitionHTML AcquisitionStrategy = "html"
)

// AlertSource is a configuration-time entity describing one upstream content source.
// Tags support YAML config files and env-var overrides.
type AlertSource struct {
	ID                  string              `json:"id"                  yaml:"id"`
	Name                string              `json:"name"                yaml:"name"`
	FeedURL             string              `env:"FEED_URL"             json:"feed_url"             yaml:"feed_url"`
	AcquisitionStrategy AcquisitionStrategy `env:"ACQUISITION_STRATEGY" json:"acquisition_strategy" yaml:"acquisition_strategy"`
	PollInterval        time.Duration       `env:"POLL_INTERVAL"        json:"poll_interval"        yaml:"poll_interval"`
	DefaultCategory     Category            `env:"DEFAULT_CATEGORY"     json:"default_category"     yaml:"default_category"`
	DefaultScope        []string            `env:"DEFAULT_SCOPE"        json:"default_scope"        yaml:"default_scope"`
	DefaultExpiry       time.Duration       `env:"DEFAULT_EXPIRY"       json:"default_expiry"       yaml:"default_expiry"`
	Enabled             bool                `env:"ENABLED"              json:"enabled"              yaml:"enabled"`
}

// Validate enforces config-time constraints on an AlertSource.
// FR-001: poll interval must be in [30m, 60m].
func (s *AlertSource) Validate() error {
	var errs []error

	if s.ID == "" {
		errs = append(errs, errors.New("source: id is required"))
	}

	if s.FeedURL == "" {
		errs = append(errs, errors.New("source: feed_url is required"))
	}

	if s.PollInterval < minPollInterval || s.PollInterval > maxPollInterval {
		errs = append(errs, errors.New("source: poll_interval must be between 30m and 60m (FR-001)"))
	}

	return errors.Join(errs...)
}
