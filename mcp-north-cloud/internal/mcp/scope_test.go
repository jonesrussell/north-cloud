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
