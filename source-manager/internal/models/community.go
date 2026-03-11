package models

import "time"

// Community represents a Canadian community in the authoritative registry.
type Community struct {
	ID            string  `db:"id"             json:"id"`
	Name          string  `db:"name"           json:"name"`
	Slug          string  `db:"slug"           json:"slug"`
	CommunityType string  `db:"community_type" json:"community_type"`
	Province      *string `db:"province"       json:"province,omitempty"`
	Region        *string `db:"region"         json:"region,omitempty"`

	// Authoritative identifiers
	InacID        *string `db:"inac_id"         json:"inac_id,omitempty"`
	StatCanCSD    *string `db:"statcan_csd"     json:"statcan_csd,omitempty"`
	OSMRelationID *int64  `db:"osm_relation_id" json:"osm_relation_id,omitempty"`
	WikidataQID   *string `db:"wikidata_qid"    json:"wikidata_qid,omitempty"`

	// Geodata
	Latitude  *float64 `db:"latitude"  json:"latitude,omitempty"`
	Longitude *float64 `db:"longitude" json:"longitude,omitempty"`

	// Metadata
	Nation         *string `db:"nation"          json:"nation,omitempty"`
	Treaty         *string `db:"treaty"          json:"treaty,omitempty"`
	LanguageGroup  *string `db:"language_group"  json:"language_group,omitempty"`
	ReserveName    *string `db:"reserve_name"    json:"reserve_name,omitempty"`
	Population     *int    `db:"population"      json:"population,omitempty"`
	PopulationYear *int    `db:"population_year" json:"population_year,omitempty"`

	// Digital presence
	Website *string `db:"website"  json:"website,omitempty"`
	FeedURL *string `db:"feed_url" json:"feed_url,omitempty"`

	// Source attribution
	DataSource string  `db:"data_source" json:"data_source"`
	SourceID   *string `db:"source_id"   json:"source_id,omitempty"`

	// Lifecycle
	Enabled       bool       `db:"enabled"         json:"enabled"`
	CreatedAt     time.Time  `db:"created_at"      json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"      json:"updated_at"`
	LastScrapedAt *time.Time `db:"last_scraped_at" json:"last_scraped_at,omitempty"`
}

// CommunityFilter defines filters for listing communities.
type CommunityFilter struct {
	Type     string
	Province string
	Search   string
	Limit    int
	Offset   int
}

// CommunityWithDistance wraps Community with a computed distance field.
type CommunityWithDistance struct {
	Community
	DistanceKm float64 `db:"distance_km" json:"distance_km"`
}
