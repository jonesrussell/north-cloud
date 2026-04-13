//nolint:testpackage // testing unexported getToolsForEnv and scope internals
package mcp

import "testing"

func TestToolScope_IsAllowed(t *testing.T) {
	tests := []struct {
		name    string
		scope   ToolScope
		env     string
		allowed bool
	}{
		{"shared in local", ScopeShared, EnvLocal, true},
		{"shared in prod", ScopeShared, EnvProd, true},
		{"local in local", ScopeLocal, EnvLocal, true},
		{"local in prod", ScopeLocal, EnvProd, false},
		{"prod in prod", ScopeProd, EnvProd, true},
		{"prod in local", ScopeProd, EnvLocal, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.scope.IsAllowed(tt.env); got != tt.allowed {
				t.Errorf("ToolScope(%q).IsAllowed(%q) = %v, want %v", tt.scope, tt.env, got, tt.allowed)
			}
		})
	}
}

func TestGetToolsForEnv_Counts(t *testing.T) {
	localTools := getToolsForEnv(EnvLocal)
	prodTools := getToolsForEnv(EnvProd)

	// Local = 23 shared + 3 local = 26
	expectedLocal := 26
	if len(localTools) != expectedLocal {
		t.Errorf("local tools = %d, want %d", len(localTools), expectedLocal)
	}

	// Prod = 23 shared + 14 prod = 37
	expectedProd := 37
	if len(prodTools) != expectedProd {
		t.Errorf("prod tools = %d, want %d", len(prodTools), expectedProd)
	}
}
