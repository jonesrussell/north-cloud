package router

import "strings"

// JobDomain routes job-classified content to jobs:* channels.
// Channels produced:
//   - articles:jobs (catch-all)
//   - jobs:type:{slug} (per employment type)
//   - jobs:industry:{slug} (per industry)
type JobDomain struct{}

// NewJobDomain creates a JobDomain.
func NewJobDomain() *JobDomain { return &JobDomain{} }

// Name returns the domain identifier.
func (d *JobDomain) Name() string { return "job" }

// Routes returns job channels for the article.
func (d *JobDomain) Routes(a *Article) []ChannelRoute {
	if a.Job == nil {
		return nil
	}

	channels := []string{"articles:jobs"}

	if a.Job.EmploymentType != "" {
		slug := strings.ToLower(strings.ReplaceAll(a.Job.EmploymentType, "_", "-"))
		channels = append(channels, "jobs:type:"+slug)
	}

	if a.Job.Industry != "" {
		slug := strings.ToLower(strings.ReplaceAll(a.Job.Industry, " ", "-"))
		channels = append(channels, "jobs:industry:"+slug)
	}

	return channelRoutesFromSlice(channels)
}

// compile-time interface check
var _ RoutingDomain = (*JobDomain)(nil)
