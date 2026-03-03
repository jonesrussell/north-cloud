//nolint:testpackage // Testing internal routing domain requires same package access
package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRFPDomain_NilRFP(t *testing.T) {
	t.Helper()
	d := NewRFPDomain()
	routes := d.Routes(&ContentItem{})
	assert.Nil(t, routes)
}

func TestRFPDomain_BasicRouting(t *testing.T) {
	t.Helper()
	d := NewRFPDomain()
	item := &ContentItem{
		RFP: &RFPData{
			Province:        "ON",
			Country:         "CA",
			Categories:      []string{"IT", "consulting"},
			ProcurementType: "services",
		},
	}
	routes := d.Routes(item)
	channels := rfpRouteNames(routes)

	assert.Contains(t, channels, "content:rfps")
	assert.Contains(t, channels, "rfp:country:ca")
	assert.Contains(t, channels, "rfp:province:on")
	assert.Contains(t, channels, "rfp:sector:it")
	assert.Contains(t, channels, "rfp:sector:consulting")
	assert.Contains(t, channels, "rfp:type:services")
}

func TestRFPDomain_Name(t *testing.T) {
	t.Helper()
	d := NewRFPDomain()
	assert.Equal(t, "rfp", d.Name())
}

func TestRFPDomain_MinimalFields(t *testing.T) {
	t.Helper()
	d := NewRFPDomain()
	item := &ContentItem{
		RFP: &RFPData{},
	}
	routes := d.Routes(item)
	channels := rfpRouteNames(routes)

	assert.Contains(t, channels, "content:rfps")
	assert.Len(t, channels, 1, "Only catch-all channel when no fields set")
}

// rfpRouteNames extracts channel names from routes for test assertions.
func rfpRouteNames(routes []ChannelRoute) []string {
	t := make([]string, len(routes))
	for i, r := range routes {
		t[i] = r.Channel
	}
	return t
}
