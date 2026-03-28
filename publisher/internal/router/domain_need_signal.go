package router

import "strings"

// NeedSignalDomain routes need-signal-classified content to need-signal:* channels.
// Channels produced:
//   - content:need-signals (catch-all)
//   - need-signal:type:{signal_type} (per signal type, lowercase)
//   - need-signal:province:{province} (per province, lowercase)
//   - need-signal:sector:{sector} (per sector, lowercase)
type NeedSignalDomain struct{}

// NewNeedSignalDomain creates a NeedSignalDomain.
func NewNeedSignalDomain() *NeedSignalDomain { return &NeedSignalDomain{} }

// Name returns the domain identifier.
func (d *NeedSignalDomain) Name() string { return "need_signal" }

// Routes returns need-signal channels for the content item.
func (d *NeedSignalDomain) Routes(item *ContentItem) []ChannelRoute {
	if item.NeedSignal == nil {
		return nil
	}

	channels := []string{"content:need-signals"}

	if item.NeedSignal.SignalType != "" {
		channels = append(channels, "need-signal:type:"+strings.ToLower(item.NeedSignal.SignalType))
	}

	if item.NeedSignal.Province != "" {
		channels = append(channels, "need-signal:province:"+strings.ToLower(item.NeedSignal.Province))
	}

	if item.NeedSignal.Sector != "" {
		channels = append(channels, "need-signal:sector:"+strings.ToLower(item.NeedSignal.Sector))
	}

	return channelRoutesFromSlice(channels)
}

// compile-time interface check
var _ RoutingDomain = (*NeedSignalDomain)(nil)
