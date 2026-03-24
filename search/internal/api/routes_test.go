//nolint:testpackage // tests package-level setup functions
package api

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetupRoutes_RegistersExpectedPaths(t *testing.T) {
	t.Helper()

	router := gin.New()
	handler := &Handler{}

	SetupRoutes(router, handler)

	expectedRoutes := map[string]bool{
		"GET /health":                false,
		"GET /ready":                 false,
		"GET /health/memory":         false,
		"GET /api/v1/health":         false,
		"GET /api/v1/ready":          false,
		"GET /api/v1/search":         false,
		"POST /api/v1/search":        false,
		"GET /api/v1/search/suggest": false,
		"GET /api/v1/feeds/latest":   false,
		"GET /api/v1/feeds/:slug":    false,
	}

	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := expectedRoutes[key]; ok {
			expectedRoutes[key] = true
		}
	}

	for route, found := range expectedRoutes {
		if !found {
			t.Errorf("expected route %q to be registered", route)
		}
	}
}

func TestSetupServiceRoutes_RegistersExpectedPaths(t *testing.T) {
	t.Helper()

	router := gin.New()
	handler := &Handler{}

	SetupServiceRoutes(router, handler)

	expectedRoutes := map[string]bool{
		"GET /ready":                  false,
		"GET /feed.json":              false,
		"GET /api/communities/search": false,
		"GET /api/v1/health":          false,
		"GET /api/v1/ready":           false,
		"GET /api/v1/search":          false,
		"POST /api/v1/search":         false,
		"GET /api/v1/feeds/:slug":     false,
	}

	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := expectedRoutes[key]; ok {
			expectedRoutes[key] = true
		}
	}

	for route, found := range expectedRoutes {
		if !found {
			t.Errorf("expected route %q to be registered", route)
		}
	}
}
