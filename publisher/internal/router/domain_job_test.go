package router_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/publisher/internal/router"
)

func TestJobDomain_NilJob(t *testing.T) {
	d := router.NewJobDomain()
	routes := d.Routes(&router.Article{})
	if routes != nil {
		t.Error("expected nil routes for article without job data")
	}
}

func TestJobDomain_WithTypeAndIndustry(t *testing.T) {
	d := router.NewJobDomain()
	article := &router.Article{
		Job: &router.JobData{
			ExtractionMethod: "schema_org",
			EmploymentType:   "full_time",
			Industry:         "Technology",
		},
	}
	routes := d.Routes(article)
	if len(routes) == 0 {
		t.Fatal("expected routes")
	}

	channels := make(map[string]bool)
	for _, r := range routes {
		channels[r.Channel] = true
	}

	if !channels["articles:jobs"] {
		t.Error("expected articles:jobs channel")
	}
	if !channels["jobs:type:full-time"] {
		t.Error("expected jobs:type:full-time channel")
	}
	if !channels["jobs:industry:technology"] {
		t.Error("expected jobs:industry:technology channel")
	}
}

func TestJobDomain_Name(t *testing.T) {
	d := router.NewJobDomain()
	if d.Name() != "job" {
		t.Errorf("expected name 'job', got %q", d.Name())
	}
}

func TestJobDomain_OnlyIndustry(t *testing.T) {
	d := router.NewJobDomain()
	article := &router.Article{
		Job: &router.JobData{
			Industry: "Healthcare",
		},
	}
	routes := d.Routes(article)
	channels := make(map[string]bool)
	for _, r := range routes {
		channels[r.Channel] = true
	}
	if !channels["articles:jobs"] {
		t.Error("expected articles:jobs")
	}
	if !channels["jobs:industry:healthcare"] {
		t.Error("expected jobs:industry:healthcare")
	}

	const expectedRouteCount = 2
	if len(routes) != expectedRouteCount {
		t.Errorf("expected %d routes, got %d", expectedRouteCount, len(routes))
	}
}
