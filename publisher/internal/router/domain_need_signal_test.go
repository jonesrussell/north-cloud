//nolint:testpackage // Testing internal routing domain requires same package access
package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNeedSignalDomain_Name(t *testing.T) {
	t.Helper()
	d := NewNeedSignalDomain()
	assert.Equal(t, "need_signal", d.Name())
}

func TestNeedSignalDomain_Routes_WithSignal(t *testing.T) {
	t.Helper()
	d := NewNeedSignalDomain()
	item := &ContentItem{
		NeedSignal: &NeedSignalData{
			SignalType: "funding",
			Province:   "on",
			Sector:     "healthcare",
		},
	}
	routes := d.Routes(item)
	channels := needSignalRouteNames(routes)

	assert.Contains(t, channels, "content:need-signals")
	assert.Contains(t, channels, "need-signal:type:funding")
	assert.Contains(t, channels, "need-signal:province:ON")
	assert.Contains(t, channels, "need-signal:sector:healthcare")
}

func TestNeedSignalDomain_Routes_NoSignal(t *testing.T) {
	t.Helper()
	d := NewNeedSignalDomain()
	routes := d.Routes(&ContentItem{})
	assert.Empty(t, routes)
}

func TestNeedSignalDomain_MinimalFields(t *testing.T) {
	t.Helper()
	d := NewNeedSignalDomain()
	item := &ContentItem{
		NeedSignal: &NeedSignalData{},
	}
	routes := d.Routes(item)
	channels := needSignalRouteNames(routes)

	assert.Contains(t, channels, "content:need-signals")
	assert.Len(t, channels, 1, "Only catch-all channel when no fields set")
}

// needSignalRouteNames extracts channel names from routes for test assertions.
func needSignalRouteNames(routes []ChannelRoute) []string {
	t := make([]string, len(routes))
	for i, r := range routes {
		t[i] = r.Channel
	}
	return t
}
