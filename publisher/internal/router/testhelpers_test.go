// publisher/internal/router/testhelpers_test.go
//
//nolint:testpackage // Testing internal router requires same package access
package router

// routeChannelNames extracts the Channel field from each ChannelRoute.
// Use in tests that need to assert on channel name strings after a domain.Routes() call.
func routeChannelNames(routes []ChannelRoute) []string {
	names := make([]string, len(routes))
	for i, r := range routes {
		names[i] = r.Channel
	}
	return names
}
