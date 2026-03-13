package models

import "time"

// DictionaryEntry represents a single entry in the OPD dictionary.
// All consent flags default to false — content is not public until explicitly authorized.
type DictionaryEntry struct {
	ID                     string    `db:"id"                       json:"id"`
	Lemma                  string    `db:"lemma"                    json:"lemma"`
	WordClass              *string   `db:"word_class"               json:"word_class,omitempty"`
	WordClassNormalized    *string   `db:"word_class_normalized"    json:"word_class_normalized,omitempty"`
	Definitions            string    `db:"definitions"              json:"definitions"`
	Inflections            string    `db:"inflections"              json:"inflections"`
	Examples               string    `db:"examples"                 json:"examples"`
	WordFamily             string    `db:"word_family"              json:"word_family"`
	Media                  string    `db:"media"                    json:"media"`
	Attribution            *string   `db:"attribution"              json:"attribution,omitempty"`
	License                string    `db:"license"                  json:"license"`
	ConsentPublicDisplay   bool      `db:"consent_public_display"   json:"consent_public_display"`
	ConsentAITraining      bool      `db:"consent_ai_training"      json:"consent_ai_training"`
	ConsentDerivativeWorks bool      `db:"consent_derivative_works" json:"consent_derivative_works"`
	ContentHash            *string   `db:"content_hash"             json:"content_hash,omitempty"`
	SourceURL              *string   `db:"source_url"               json:"source_url,omitempty"`
	CreatedAt              time.Time `db:"created_at"               json:"created_at"`
	UpdatedAt              time.Time `db:"updated_at"               json:"updated_at"`
}

// DictionaryEntryFilter holds query parameters for listing dictionary entries.
type DictionaryEntryFilter struct {
	Limit  int
	Offset int
}
