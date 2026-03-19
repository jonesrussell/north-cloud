package mcp

//nolint:funlen // Tool definitions are data structures, not complex logic
func getCommunityTools() []Tool {
	return []Tool{
		{
			Name:  "list_communities",
			Scope: ScopeShared,
			Description: "List First Nations communities with pagination. " +
				"Use when: You need to browse communities, find a community_id, or check community data. " +
				"Returns: paginated list with id, name, slug, province, nation (default 20, max 100).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of communities to return (default: 20, max: 100)",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of communities to skip for pagination (default: 0)",
					},
				},
			},
		},
		{
			Name:  "get_community",
			Scope: ScopeShared,
			Description: "Get a single community by ID or slug. " +
				"Use when: You need full details for a specific community. " +
				"Provide either community_id or slug (not both).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"community_id": map[string]any{
						"type":        "string",
						"description": "UUID of the community",
					},
					"slug": map[string]any{
						"type":        "string",
						"description": "URL-friendly slug of the community (alternative to community_id)",
					},
				},
			},
		},
		{
			Name:  "find_nearby_communities",
			Scope: ScopeShared,
			Description: "Find communities near a geographic point. " +
				"Use when: You need to discover communities within a radius of a lat/lng coordinate. " +
				"Requires lat and lng. Optional: radius_km (default 50), limit (default 10).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"lat": map[string]any{
						"type":        "number",
						"description": "Latitude of the center point",
					},
					"lng": map[string]any{
						"type":        "number",
						"description": "Longitude of the center point",
					},
					"radius_km": map[string]any{
						"type":        "number",
						"description": "Search radius in kilometers (default: 50)",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum number of results (default: 10)",
					},
				},
				"required": []string{"lat", "lng"},
			},
		},
		{
			Name:  "add_community",
			Scope: ScopeProd,
			Description: "Create a new First Nations community record. " +
				"Use when: Adding a community to the database. Requires name and slug. " +
				"Optional: province, region, nation, tribal_council, latitude, longitude, population, website.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Community name",
					},
					"slug": map[string]any{
						"type":        "string",
						"description": "URL-friendly slug",
					},
					"province": map[string]any{
						"type":        "string",
						"description": "Province or territory",
					},
					"region": map[string]any{
						"type":        "string",
						"description": "Geographic region",
					},
					"nation": map[string]any{
						"type":        "string",
						"description": "Nation affiliation",
					},
					"tribal_council": map[string]any{
						"type":        "string",
						"description": "Tribal council affiliation",
					},
					"latitude": map[string]any{
						"type":        "number",
						"description": "Latitude coordinate",
					},
					"longitude": map[string]any{
						"type":        "number",
						"description": "Longitude coordinate",
					},
					"population": map[string]any{
						"type":        "integer",
						"description": "Total population",
					},
					"website": map[string]any{
						"type":        "string",
						"description": "Community website URL",
					},
				},
				"required": []string{"name", "slug"},
			},
		},
		{
			Name:  "update_community",
			Scope: ScopeProd,
			Description: "Update an existing community record. " +
				"Use when: Modifying community data. Requires community_id. " +
				"Only provided fields are updated.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"community_id": map[string]any{
						"type":        "string",
						"description": "UUID of the community to update",
					},
					"name": map[string]any{
						"type":        "string",
						"description": "New community name",
					},
					"slug": map[string]any{
						"type":        "string",
						"description": "New URL-friendly slug",
					},
					"province": map[string]any{
						"type":        "string",
						"description": "Province or territory",
					},
					"nation": map[string]any{
						"type":        "string",
						"description": "Nation affiliation",
					},
					"website": map[string]any{
						"type":        "string",
						"description": "Community website URL",
					},
				},
				"required": []string{"community_id"},
			},
		},
		{
			Name:  "link_sources",
			Scope: ScopeProd,
			Description: "Match news sources to communities by name similarity. " +
				"Use when: You want to auto-link indigenous news sources to their communities. " +
				"Defaults to dry_run=true (preview only). Set dry_run=false to persist links.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"dry_run": map[string]any{
						"type":        "boolean",
						"description": "Preview matches without saving (default: true)",
					},
				},
			},
		},
	}
}
