package mcp

//nolint:funlen // Tool definitions are data structures, not complex logic
func getPeopleTools() []Tool {
	return []Tool{
		{
			Name:  "list_people",
			Scope: ScopeShared,
			Description: "List people (leaders, officials) for a community with pagination. " +
				"Use when: You need to see who holds leadership roles in a community. " +
				"Requires community_id. Optional: current_only (default true), role filter.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"community_id": map[string]any{
						"type":        "string",
						"description": "UUID of the community",
					},
					"current_only": map[string]any{
						"type":        "boolean",
						"description": "Only return current officeholders (default: true)",
					},
					"limit": map[string]any{
						"type":        "integer",
						"description": "Maximum results (default: 20, max: 100)",
					},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Pagination offset (default: 0)",
					},
				},
				"required": []string{"community_id"},
			},
		},
		{
			Name:  "get_person",
			Scope: ScopeShared,
			Description: "Get a person by ID with full details including role, contact, and term info. " +
				"Use when: You need details about a specific community leader or official.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"person_id": map[string]any{
						"type":        "string",
						"description": "UUID of the person",
					},
				},
				"required": []string{"person_id"},
			},
		},
		{
			Name:  "add_person",
			Scope: ScopeProd,
			Description: "Add a person (leader/official) to a community. " +
				"Use when: Recording a new community leader or official. " +
				"Requires community_id, name, role.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"community_id": map[string]any{
						"type":        "string",
						"description": "UUID of the community",
					},
					"name": map[string]any{
						"type":        "string",
						"description": "Person's full name",
					},
					"role": map[string]any{
						"type":        "string",
						"description": "Role (e.g., 'chief', 'councillor')",
					},
					"role_title": map[string]any{
						"type":        "string",
						"description": "Formal title (e.g., 'Chief', 'Band Councillor')",
					},
					"email": map[string]any{
						"type":        "string",
						"description": "Contact email",
					},
					"phone": map[string]any{
						"type":        "string",
						"description": "Contact phone number",
					},
				},
				"required": []string{"community_id", "name", "role"},
			},
		},
		{
			Name:  "get_band_office",
			Scope: ScopeShared,
			Description: "Get the band office details for a community including address, contact info, and hours. " +
				"Use when: You need the physical office location or contact details for a community.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"community_id": map[string]any{
						"type":        "string",
						"description": "UUID of the community",
					},
				},
				"required": []string{"community_id"},
			},
		},
		{
			Name:  "upsert_band_office",
			Scope: ScopeProd,
			Description: "Create or update the band office for a community. " +
				"Use when: Adding or updating band office address, contact, or hours. " +
				"Requires community_id.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"community_id": map[string]any{
						"type":        "string",
						"description": "UUID of the community",
					},
					"address_line1": map[string]any{
						"type":        "string",
						"description": "Street address",
					},
					"city": map[string]any{
						"type":        "string",
						"description": "City",
					},
					"province": map[string]any{
						"type":        "string",
						"description": "Province or territory",
					},
					"postal_code": map[string]any{
						"type":        "string",
						"description": "Postal code",
					},
					"phone": map[string]any{
						"type":        "string",
						"description": "Phone number",
					},
					"email": map[string]any{
						"type":        "string",
						"description": "Email address",
					},
					"office_hours": map[string]any{
						"type":        "string",
						"description": "Office hours (e.g., 'Mon-Fri 9am-5pm')",
					},
				},
				"required": []string{"community_id"},
			},
		},
	}
}
