package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
)

// getAllPrompts returns the list of prompt definitions (name, description, arguments).
func getAllPrompts() []Prompt {
	return []Prompt{
		{
			Name:        "onboard_new_source",
			Description: "Add a new website/source and optionally start crawling and create a publishing route.",
			Arguments: []PromptArgument{
				{Name: "name", Description: "Name of the source", Required: true, Type: "string"},
				{Name: "url", Description: "Base URL of the source to crawl", Required: true, Type: "string"},
				{Name: "source_type", Description: "Type of source (e.g. news, blog)", Required: true, Type: "string"},
				{Name: "selectors", Description: "CSS selectors for content extraction (e.g. title, body, date, author)", Required: true},
				{Name: "crawl_interval_minutes", Description: "Crawl interval value (optional)", Required: false, Type: "number"},
				{Name: "crawl_interval_type", Description: "minutes, hours, or days (required if crawl_interval_minutes set)", Required: false, Type: "string"},
				{Name: "channel_id", Description: "Channel to publish to; if set, a route will be created", Required: false, Type: "string"},
				{Name: "min_quality_score", Description: "Minimum quality score for publishing (0-100)", Required: false, Type: "number"},
				{Name: "topics", Description: "Topics to filter by when publishing", Required: false},
			},
		},
		{
			Name:        "debug_crawl_job",
			Description: "Inspect a crawl job: status, stats, and suggestions.",
			Arguments: []PromptArgument{
				{Name: "job_id", Description: "ID of the crawl job to inspect", Required: true, Type: "string"},
			},
		},
		{
			Name:        "publishing_review",
			Description: "Preview a route and review publish history and stats.",
			Arguments: []PromptArgument{
				{Name: "route_id", Description: "ID of the route to preview (optional)", Required: false, Type: "string"},
				{Name: "channel_id", Description: "Filter by channel (optional)", Required: false, Type: "string"},
			},
		},
		{
			Name:        "classify_and_search",
			Description: "Search classified content and optionally classify a sample.",
			Arguments: []PromptArgument{
				{Name: "query", Description: "Search query string", Required: true, Type: "string"},
				{Name: "min_quality_score", Description: "Minimum quality score filter", Required: false, Type: "number"},
				{Name: "topics", Description: "Topics to filter by", Required: false},
			},
		},
	}
}

const selectorCheatsheet = `Selector cheatsheet: title (e.g. h1), body (e.g. article or .content), date (e.g. time[datetime]), author (e.g. .byline).`

// getPromptByName returns the prompt messages for the given name with arguments substituted.
// If any required argument is missing, returns an error suitable for -32602 (Invalid params).
func getPromptByName(name string, arguments map[string]any) ([]PromptMessage, error) {
	prompts := getAllPrompts()
	var def *Prompt
	for i := range prompts {
		if prompts[i].Name == name {
			def = &prompts[i]
			break
		}
	}
	if def == nil {
		return nil, fmt.Errorf("unknown prompt: %s", name)
	}

	var missing []string
	for _, arg := range def.Arguments {
		if !arg.Required {
			continue
		}
		v, ok := arguments[arg.Name]
		if !ok || v == nil {
			missing = append(missing, arg.Name)
			continue
		}
		if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
			missing = append(missing, arg.Name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required arguments: %s", strings.Join(missing, ", "))
	}

	return buildPromptMessages(def, arguments)
}

func buildPromptMessages(def *Prompt, args map[string]any) ([]PromptMessage, error) {
	switch def.Name {
	case "onboard_new_source":
		return buildOnboardNewSourceMessages(args), nil
	case "debug_crawl_job":
		return buildDebugCrawlJobMessages(args), nil
	case "publishing_review":
		return buildPublishingReviewMessages(args), nil
	case "classify_and_search":
		return buildClassifyAndSearchMessages(args), nil
	default:
		return nil, fmt.Errorf("unknown prompt: %s", def.Name)
	}
}

func buildOnboardNewSourceMessages(args map[string]any) []PromptMessage {
	text := "Use the onboard_source tool with the provided arguments to add the source and optionally start crawling and create a publishing route. "
	if cid, _ := args["channel_id"].(string); cid != "" {
		text += "Since channel_id is set, a route will be created. "
	}
	text += selectorCheatsheet
	return []PromptMessage{{
		Role:    "user",
		Content: []PromptContent{{Type: "text", Text: text}},
	}}
}

func buildDebugCrawlJobMessages(args map[string]any) []PromptMessage {
	jobID, _ := args["job_id"].(string)
	text := fmt.Sprintf(
		"Use list_crawl_jobs (filter by id if supported) and get_crawl_stats for job_id %q. "+
			"Summarize the job status and success rate, and suggest common fixes (e.g. selector issues, network).",
		jobID,
	)
	return []PromptMessage{{
		Role:    "user",
		Content: []PromptContent{{Type: "text", Text: text}},
	}}
}

func buildPublishingReviewMessages(args map[string]any) []PromptMessage {
	text := "Use preview_route(route_id), get_publish_history, and get_publisher_stats to preview " +
		"what would be published and review recent activity. Summarize the findings."
	if routeID, ok := args["route_id"].(string); ok && routeID != "" {
		text = "Use preview_route with route_id \"" + routeID + "\", get_publish_history, and " +
			"get_publisher_stats. Summarize what would be published and recent activity."
	}
	return []PromptMessage{{
		Role:    "user",
		Content: []PromptContent{{Type: "text", Text: text}},
	}}
}

func buildClassifyAndSearchMessages(args map[string]any) []PromptMessage {
	query, _ := args["query"].(string)
	text := fmt.Sprintf(
		"Use search_articles with query %q. Optionally use classify_article on one result to explain quality and topics. Summarize the findings.",
		query,
	)
	return []PromptMessage{{
		Role:    "user",
		Content: []PromptContent{{Type: "text", Text: text}},
	}}
}

// promptsGetParams for prompts/get.
type promptsGetParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// parsePromptsGetParams parses params for prompts/get. Returns name, arguments, and error message if invalid.
func parsePromptsGetParams(params json.RawMessage) (name string, arguments map[string]any, errMsg string) {
	var p promptsGetParams
	if unmarshalErr := json.Unmarshal(params, &p); unmarshalErr != nil {
		return "", nil, "Invalid parameters: " + unmarshalErr.Error()
	}
	if p.Name == "" {
		return "", nil, "name is required"
	}
	if p.Arguments == nil {
		p.Arguments = map[string]any{}
	}
	return p.Name, p.Arguments, ""
}
