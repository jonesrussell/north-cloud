package router

import "strings"

// JobDomain routes job-classified content to jobs:* channels.
// Channels produced:
//   - content:jobs (catch-all)
//   - jobs:type:{slug} (per employment type)
//   - jobs:industry:{slug} (per industry)
type JobDomain struct{}

// NewJobDomain creates a JobDomain.
func NewJobDomain() *JobDomain { return &JobDomain{} }

// Name returns the domain identifier.
func (d *JobDomain) Name() string { return "job" }

// Routes returns job channels for the content item.
func (d *JobDomain) Routes(item *ContentItem) []ChannelRoute {
	if item.Job == nil {
		return nil
	}

	channels := []string{"content:jobs"}

	if item.Job.EmploymentType != "" {
		slug := strings.ToLower(strings.ReplaceAll(item.Job.EmploymentType, "_", "-"))
		channels = append(channels, "jobs:type:"+slug)
	}

	if item.Job.Industry != "" {
		slug := strings.ToLower(strings.ReplaceAll(item.Job.Industry, " ", "-"))
		channels = append(channels, "jobs:industry:"+slug)
	}

	return channelRoutesFromSlice(channels)
}

// compile-time interface check
var _ RoutingDomain = (*JobDomain)(nil)
