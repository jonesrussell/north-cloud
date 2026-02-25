package router

// layer1SkipTopics lists topics handled by dedicated routing layers.
// These topics are excluded from Layer 1 auto-routing to prevent bypassing
// their specialised classifiers (e.g., coforge → Layer 8).
var layer1SkipTopics = map[string]bool{
	"mining":      true,
	"anishinaabe": true,
	"coforge":     true,
	"recipe":      true,
	"jobs":        true,
}

// TopicDomain routes content items to content:{topic} for each non-skipped topic tag.
// This is Layer 1 in the routing pipeline.
type TopicDomain struct{}

// NewTopicDomain creates a TopicDomain.
func NewTopicDomain() *TopicDomain { return &TopicDomain{} }

// Name returns the domain identifier.
func (d *TopicDomain) Name() string { return "topic" }

// Routes returns a content:{topic} channel for each topic not in layer1SkipTopics.
func (d *TopicDomain) Routes(item *ContentItem) []ChannelRoute {
	names := make([]string, 0, len(item.Topics))
	for _, topic := range item.Topics {
		if layer1SkipTopics[topic] {
			continue
		}
		names = append(names, "content:"+topic)
	}
	return channelRoutesFromSlice(names)
}
