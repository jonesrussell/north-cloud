package mcp

import (
	"fmt"
	"strings"
)

const northcloudScheme = "northcloud://"

// getAllResources returns the list of static resource metadata.
func getAllResources() []ResourceListItem {
	return []ResourceListItem{
		{
			URI:         "northcloud://docs/tool-reference",
			Name:        "North Cloud Tool Reference",
			Description: "List of MCP tools and when to use them",
			MimeType:    "text/plain",
		},
		{
			URI:         "northcloud://docs/selectors",
			Name:        "Selector Cheatsheet",
			Description: "CSS selectors for source extraction",
			MimeType:    "text/plain",
		},
		{
			URI:         "northcloud://docs/pipeline",
			Name:        "Pipeline Overview",
			Description: "Crawl → Classify → Publish flow",
			MimeType:    "text/plain",
		},
	}
}

// readResource returns content for a known URI. For unknown URI returns error with ResourceNotFound.
func readResource(uri string) ([]ResourceContent, error) {
	if !strings.HasPrefix(uri, northcloudScheme) {
		return nil, resourceNotFoundError(uri)
	}
	path := strings.TrimPrefix(uri, northcloudScheme)
	path = strings.Trim(path, "/")
	// Disallow path traversal
	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") {
		return nil, resourceNotFoundError(uri)
	}
	switch path {
	case "docs/tool-reference":
		return []ResourceContent{{URI: uri, MimeType: "text/plain", Text: staticToolReference}}, nil
	case "docs/selectors":
		return []ResourceContent{{URI: uri, MimeType: "text/plain", Text: staticSelectors}}, nil
	case "docs/pipeline":
		return []ResourceContent{{URI: uri, MimeType: "text/plain", Text: staticPipeline}}, nil
	default:
		return nil, resourceNotFoundError(uri)
	}
}

func resourceNotFoundError(uri string) error {
	return &ResourceNotFoundError{URI: uri}
}

// ResourceNotFoundError is returned for unknown resource URIs.
type ResourceNotFoundError struct {
	URI string
}

func (e *ResourceNotFoundError) Error() string {
	return fmt.Sprintf("resource not found: %s", e.URI)
}

// Static doc content (short, 1–2 lines per tool or 5–8 selector examples or one short paragraph per stage).
//
//nolint:lll // long single-line content strings for static docs
const staticToolReference = `get_auth_token: Get JWT for API calls. onboard_source: Add source, optional crawl and route. start_crawl: Create one-off crawl job (source_id, url). schedule_crawl: Create recurring job (source_id, url, interval_minutes, interval_type). list_crawl_jobs: List jobs (optional status, limit, offset). control_crawl_job: Pause/resume/cancel (job_id, action). get_crawl_stats: Job stats (job_id). add_source, list_sources, update_source, delete_source, test_source: Source CRUD and test crawl. create_route, list_routes, delete_route, preview_route: Route CRUD and preview. create_channel, list_channels: Channel CRUD. get_publish_history, get_publisher_stats: History and stats. search_articles: Full-text search (query, filters). classify_article: Classify one article (title, raw_text, url). list_indexes, delete_index: Index list/delete. lint_file, build_service, test_service: Dev helpers (file_path or service_name).`

//nolint:lll // static doc string for selector cheatsheet
const staticSelectors = `title: h1 or .headline. body: article or .content or main. date: time[datetime] or .date. author: .byline or [rel="author"]. link: a[href]. image: img or picture source.`

//nolint:lll // static doc string for pipeline overview
const staticPipeline = `Crawl: Crawler fetches pages and writes to Elasticsearch indexes named {source}_raw_content with classification_status=pending. Classify: Classifier reads raw content, assigns type/quality/topics/crime, writes to {source}_classified_content. Publish: Publisher filters classified content by route (quality, topics), publishes matching articles to Redis channels (e.g. articles:crime, articles:news).`
