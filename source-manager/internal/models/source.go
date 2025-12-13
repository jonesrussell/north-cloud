package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// Source represents a content source configuration
type Source struct {
	ID           string         `json:"id" db:"id"`
	Name         string         `json:"name" db:"name"`
	URL          string         `json:"url" db:"url"`
	ArticleIndex string         `json:"article_index" db:"article_index"`
	PageIndex    string         `json:"page_index" db:"page_index"`
	RateLimit    string         `json:"rate_limit" db:"rate_limit"`
	MaxDepth     int            `json:"max_depth" db:"max_depth"`
	Time         StringArray    `json:"time" db:"time"`
	Selectors    SelectorConfig `json:"selectors" db:"selectors"`
	CityName     *string        `json:"city_name,omitempty" db:"city_name"` // Optional mapping to gopost city
	GroupID      *string        `json:"group_id,omitempty" db:"group_id"`   // Optional Drupal group UUID
	Enabled      bool           `json:"enabled" db:"enabled"`
	CreatedAt    time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at" db:"updated_at"`
}

// SelectorConfig represents CSS selector configuration
type SelectorConfig struct {
	Article ArticleSelectors `json:"article"`
	List    ListSelectors    `json:"list"`
	Page    PageSelectors    `json:"page"`
}

// ArticleSelectors defines CSS selectors for article extraction
type ArticleSelectors struct {
	Container     string   `json:"container,omitempty"`
	Title         string   `json:"title,omitempty"`
	Body          string   `json:"body,omitempty"`
	Intro         string   `json:"intro,omitempty"`
	Link          string   `json:"link,omitempty"`
	Image         string   `json:"image,omitempty"`
	Byline        string   `json:"byline,omitempty"`
	PublishedTime string   `json:"published_time,omitempty"`
	TimeAgo       string   `json:"time_ago,omitempty"`
	Section       string   `json:"section,omitempty"`
	Category      string   `json:"category,omitempty"`
	ArticleID     string   `json:"article_id,omitempty"`
	JSONLD        string   `json:"json_ld,omitempty"`
	Keywords      string   `json:"keywords,omitempty"`
	Description   string   `json:"description,omitempty"`
	OGTitle       string   `json:"og_title,omitempty"`
	OGDescription string   `json:"og_description,omitempty"`
	OGImage       string   `json:"og_image,omitempty"`
	OGURL         string   `json:"og_url,omitempty"`
	OGType        string   `json:"og_type,omitempty"`
	OGSiteName    string   `json:"og_site_name,omitempty"`
	Canonical     string   `json:"canonical,omitempty"`
	Author        string   `json:"author,omitempty"`
	Exclude       []string `json:"exclude,omitempty"`
}

// ListSelectors defines CSS selectors for list page extraction
type ListSelectors struct {
	Container       string   `json:"container,omitempty"`
	ArticleCards    string   `json:"article_cards,omitempty"`
	ArticleList     string   `json:"article_list,omitempty"`
	ExcludeFromList []string `json:"exclude_from_list,omitempty"`
}

// PageSelectors defines CSS selectors for page content extraction
type PageSelectors struct {
	Container     string   `json:"container,omitempty"`
	Title         string   `json:"title,omitempty"`
	Content       string   `json:"content,omitempty"`
	Description   string   `json:"description,omitempty"`
	Keywords      string   `json:"keywords,omitempty"`
	OGTitle       string   `json:"og_title,omitempty"`
	OGDescription string   `json:"og_description,omitempty"`
	OGImage       string   `json:"og_image,omitempty"`
	OGURL         string   `json:"og_url,omitempty"`
	Canonical     string   `json:"canonical,omitempty"`
	Exclude       []string `json:"exclude,omitempty"`
}

// StringArray is a custom type for PostgreSQL string arrays
type StringArray []string

var (
	// ErrEmptyStringArray is returned when trying to value an empty or nil StringArray
	ErrEmptyStringArray = errors.New("string array is empty or nil")
)

// Value implements driver.Valuer for database storage
func (a *StringArray) Value() (driver.Value, error) {
	if a == nil || len(*a) == 0 {
		return nil, ErrEmptyStringArray
	}
	return json.Marshal(*a)
}

// Scan implements sql.Scanner for database retrieval
func (a *StringArray) Scan(value any) error {
	if value == nil {
		*a = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, a)
}

// City represents a city configuration for gopost
type City struct {
	Name    string `json:"name"`
	Index   string `json:"index"`
	GroupID string `json:"group_id,omitempty"`
}
