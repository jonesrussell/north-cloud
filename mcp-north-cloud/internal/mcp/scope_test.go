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

	// Local = 15 shared + 3 local = 18
	expectedLocal := 18
	if len(localTools) != expectedLocal {
		t.Errorf("local tools = %d, want %d", len(localTools), expectedLocal)
	}

	// Prod = 15 shared + 9 prod = 24
	expectedProd := 24
	if len(prodTools) != expectedProd {
		t.Errorf("prod tools = %d, want %d", len(prodTools), expectedProd)
	}
}
