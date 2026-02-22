package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// Source represents a content source configuration
type Source struct {
	ID                      string         `db:"id"                         json:"id"`
	Name                    string         `db:"name"                       json:"name"`
	URL                     string         `db:"url"                        json:"url"`
	RateLimit               string         `db:"rate_limit"                 json:"rate_limit"`
	MaxDepth                int            `db:"max_depth"                  json:"max_depth"`
	Time                    StringArray    `db:"time"                       json:"time"`
	Selectors               SelectorConfig `db:"selectors"                  json:"selectors"`
	Enabled                 bool           `db:"enabled"                    json:"enabled"`
	FeedURL                 *string        `db:"feed_url"                   json:"feed_url,omitempty"`
	SitemapURL              *string        `db:"sitemap_url"                json:"sitemap_url,omitempty"`
	IngestionMode           string         `db:"ingestion_mode"             json:"ingestion_mode"`
	FeedPollIntervalMinutes int            `db:"feed_poll_interval_minutes" json:"feed_poll_interval_minutes"`
	FeedDisabledAt          *time.Time     `db:"feed_disabled_at"           json:"feed_disabled_at,omitempty"`
	FeedDisableReason       *string        `db:"feed_disable_reason"        json:"feed_disable_reason,omitempty"`
	CreatedAt               time.Time      `db:"created_at"                 json:"created_at"`
	UpdatedAt               time.Time      `db:"updated_at"                 json:"updated_at"`
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

// Default returns default article selectors.
func (s *ArticleSelectors) Default() ArticleSelectors {
	return ArticleSelectors{
		Container:     "article",
		Title:         "h1",
		Body:          "article > div",
		Intro:         "p.lead",
		Byline:        ".byline",
		PublishedTime: "time[datetime]",
		TimeAgo:       "time.ago",
		JSONLD:        "script[type='application/ld+json']",
		Description:   "meta[name='description']",
		Section:       ".section",
		Keywords:      "meta[name='keywords']",
		OGTitle:       "meta[property='og:title']",
		OGDescription: "meta[property='og:description']",
		OGImage:       "meta[property='og:image']",
		OGURL:         "meta[property='og:url']",
		OGSiteName:    "meta[property='og:site_name']",
		Canonical:     "link[rel='canonical']",
		Category:      ".category",
		Author:        ".author",
	}
}

// MergeWithDefaults merges the current selectors with default values.
// Fields that are empty in the current selectors will be filled with defaults.
func (s *ArticleSelectors) MergeWithDefaults() ArticleSelectors {
	defaults := s.Default()
	result := *s

	if result.Container == "" {
		result.Container = defaults.Container
	}
	if result.Title == "" {
		result.Title = defaults.Title
	}
	if result.Body == "" {
		result.Body = defaults.Body
	}
	if result.Intro == "" {
		result.Intro = defaults.Intro
	}
	if result.Byline == "" {
		result.Byline = defaults.Byline
	}
	if result.PublishedTime == "" {
		result.PublishedTime = defaults.PublishedTime
	}
	if result.TimeAgo == "" {
		result.TimeAgo = defaults.TimeAgo
	}
	if result.JSONLD == "" {
		result.JSONLD = defaults.JSONLD
	}
	if result.Description == "" {
		result.Description = defaults.Description
	}
	if result.Section == "" {
		result.Section = defaults.Section
	}
	if result.Keywords == "" {
		result.Keywords = defaults.Keywords
	}
	if result.OGTitle == "" {
		result.OGTitle = defaults.OGTitle
	}
	if result.OGDescription == "" {
		result.OGDescription = defaults.OGDescription
	}
	if result.OGImage == "" {
		result.OGImage = defaults.OGImage
	}
	if result.OGURL == "" {
		result.OGURL = defaults.OGURL
	}
	if result.OGSiteName == "" {
		result.OGSiteName = defaults.OGSiteName
	}
	if result.Canonical == "" {
		result.Canonical = defaults.Canonical
	}
	if result.Category == "" {
		result.Category = defaults.Category
	}
	if result.Author == "" {
		result.Author = defaults.Author
	}

	return result
}

// Default returns default list selectors.
func (s *ListSelectors) Default() ListSelectors {
	return ListSelectors{
		Container:    ".article-list, .articles, main",
		ArticleCards: ".article-card, article, .post",
		ArticleList:  ".article-list > li, .articles > article",
	}
}

// MergeWithDefaults merges the current selectors with default values.
// Fields that are empty in the current selectors will be filled with defaults.
func (s *ListSelectors) MergeWithDefaults() ListSelectors {
	defaults := s.Default()
	result := *s

	if result.Container == "" {
		result.Container = defaults.Container
	}
	if result.ArticleCards == "" {
		result.ArticleCards = defaults.ArticleCards
	}
	if result.ArticleList == "" {
		result.ArticleList = defaults.ArticleList
	}

	return result
}

// Default returns default page selectors.
func (s *PageSelectors) Default() PageSelectors {
	return PageSelectors{
		Container:     "main, article, body",
		Title:         "h1, title",
		Content:       "main, article, .content",
		Description:   "meta[name='description']",
		Keywords:      "meta[name='keywords']",
		OGTitle:       "meta[property='og:title']",
		OGDescription: "meta[property='og:description']",
		OGImage:       "meta[property='og:image']",
		OGURL:         "meta[property='og:url']",
		Canonical:     "link[rel='canonical']",
		Exclude: []string{
			"script, style, noscript",
			".ad, .advertisement, [class*='ad']",
			".header, .footer, nav",
			"button, form",
			".sidebar, .comments",
		},
	}
}

// MergeWithDefaults merges the current selectors with default values.
// Fields that are empty in the current selectors will be filled with defaults.
func (s *PageSelectors) MergeWithDefaults() PageSelectors {
	defaults := s.Default()
	result := *s

	if result.Container == "" {
		result.Container = defaults.Container
	}
	if result.Title == "" {
		result.Title = defaults.Title
	}
	if result.Content == "" {
		result.Content = defaults.Content
	}
	if result.Description == "" {
		result.Description = defaults.Description
	}
	if result.Keywords == "" {
		result.Keywords = defaults.Keywords
	}
	if result.OGTitle == "" {
		result.OGTitle = defaults.OGTitle
	}
	if result.OGDescription == "" {
		result.OGDescription = defaults.OGDescription
	}
	if result.OGImage == "" {
		result.OGImage = defaults.OGImage
	}
	if result.OGURL == "" {
		result.OGURL = defaults.OGURL
	}
	if result.Canonical == "" {
		result.Canonical = defaults.Canonical
	}
	if len(result.Exclude) == 0 {
		result.Exclude = defaults.Exclude
	}

	return result
}

// MergeWithDefaults merges the selector config with default values.
func (s *SelectorConfig) MergeWithDefaults() SelectorConfig {
	return SelectorConfig{
		Article: s.Article.MergeWithDefaults(),
		List:    s.List.MergeWithDefaults(),
		Page:    s.Page.MergeWithDefaults(),
	}
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
	Name  string `json:"name"`
	Index string `json:"index"`
}
