package mcp

// getAllTools returns all available MCP tools grouped by service
func getAllTools() []Tool {
	const (
		toolGroupCount         = 10
		estimatedToolsPerGroup = 8
	)
	tools := make([]Tool, 0, toolGroupCount*estimatedToolsPerGroup)
	tools = append(tools, getWorkflowTools()...)
	tools = append(tools, getCrawlerTools()...)
	tools = append(tools, getSourceManagerTools()...)
	tools = append(tools, getCommunityTools()...)
	tools = append(tools, getPeopleTools()...)
	tools = append(tools, getPublisherTools()...)
	tools = append(tools, getSearchTools()...)
	tools = append(tools, getClassifierTools()...)
	tools = append(tools, getIndexManagerTools()...)
	tools = append(tools, getAuthTools()...)
	tools = append(tools, getObservabilityTools()...)
	tools = append(tools, getFetchTools()...)
	tools = append(tools, getDevelopmentTools()...)
	tools = append(tools, getSystemTools()...)
	return tools
}

// getSystemTools returns system-level tools for health checking and diagnostics.
func getSystemTools() []Tool {
	return []Tool{
		{
			Name:  "health_check",
			Scope: ScopeShared,
			Description: "Check connectivity to all North Cloud backend services. " +
				"Use when: You suspect a service is down, experiencing errors, or want to verify " +
				"the system is healthy before performing operations. " +
				"Returns: status of each service (reachable/unreachable) with response times.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// getToolsForEnv returns tools filtered by environment scope.
func getToolsForEnv(env string) []Tool {
	all := getAllTools()
	filtered := make([]Tool, 0, len(all))
	for _, t := range all {
		if t.Scope.IsAllowed(env) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}
