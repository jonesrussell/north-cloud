package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/leadership"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const defaultWorkerCount = 4

// Config holds scraper configuration.
type Config struct {
	SourceManagerURL string
	JWTToken         string
	Workers          int
	DryRun           bool
	CommunityID      string
}

// Result tracks what happened for a single community.
type Result struct {
	CommunityID   string `json:"community_id"`
	CommunityName string `json:"community_name"`
	PeopleAdded   int    `json:"people_added"`
	PeopleSkipped int    `json:"people_skipped"`
	OfficeUpdated bool   `json:"office_updated"`
	OfficeSkipped bool   `json:"office_skipped"`
	Error         string `json:"error,omitempty"`
}

// Scraper orchestrates leadership/contact page scraping.
type Scraper struct {
	client  *Client
	fetcher *PageFetcher
	config  Config
	logger  infralogger.Logger
}

// New creates a new Scraper.
func New(cfg Config, log infralogger.Logger) *Scraper {
	if cfg.Workers <= 0 {
		cfg.Workers = defaultWorkerCount
	}
	return &Scraper{
		client:  NewClient(cfg.SourceManagerURL, cfg.JWTToken),
		fetcher: NewPageFetcher(),
		config:  cfg,
		logger:  log,
	}
}

// Run scrapes all (or a single) community using a worker pool.
func (s *Scraper) Run(ctx context.Context) ([]Result, error) {
	communities, err := s.fetchCommunities(ctx)
	if err != nil {
		return nil, err
	}

	if len(communities) == 0 {
		s.logger.Info("no communities to scrape")
		return nil, nil
	}

	s.logger.Info("starting leadership scrape",
		infralogger.Int("communities", len(communities)),
		infralogger.Int("workers", s.config.Workers),
		infralogger.Bool("dry_run", s.config.DryRun),
	)

	jobs := make(chan Community, len(communities))
	results := make(chan Result, len(communities))

	var wg sync.WaitGroup
	for range s.config.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for community := range jobs {
				results <- s.scrapeCommunity(ctx, community)
			}
		}()
	}

	for _, c := range communities {
		jobs <- c
	}
	close(jobs)

	go func() { wg.Wait(); close(results) }()

	allResults := make([]Result, 0, len(communities))
	for r := range results {
		allResults = append(allResults, r)
	}

	return allResults, nil
}

// fetchCommunities returns the list of communities to scrape.
func (s *Scraper) fetchCommunities(ctx context.Context) ([]Community, error) {
	communities, err := s.client.ListCommunitiesWithSource(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch communities: %w", err)
	}

	if s.config.CommunityID != "" {
		for _, c := range communities {
			if c.ID == s.config.CommunityID {
				return []Community{c}, nil
			}
		}
		return nil, fmt.Errorf("community %s not found in source list", s.config.CommunityID)
	}

	return communities, nil
}

// scrapeCommunity processes a single community.
func (s *Scraper) scrapeCommunity(ctx context.Context, community Community) Result {
	result := Result{
		CommunityID:   community.ID,
		CommunityName: community.Name,
	}

	if community.Website == nil || *community.Website == "" {
		result.Error = "no website URL"
		return result
	}

	website := *community.Website
	s.logger.Info("scraping community",
		infralogger.String("community_id", community.ID),
		infralogger.String("name", community.Name),
		infralogger.String("website", website),
	)

	// Step 1: Fetch homepage and discover pages
	links, fetchErr := s.fetcher.FetchLinks(ctx, website)
	if fetchErr != nil {
		result.Error = fmt.Sprintf("fetch homepage: %v", fetchErr)
		s.logger.Warn("failed to fetch homepage",
			infralogger.String("community_id", community.ID),
			infralogger.Error(fetchErr),
		)
		return result
	}

	pages := leadership.DiscoverPages(website, links)
	if len(pages) == 0 {
		result.Error = "no leadership or contact pages discovered"
		s.logger.Warn("no pages discovered",
			infralogger.String("community_id", community.ID),
		)
		return result
	}

	// Step 2: Process discovered pages
	s.processDiscoveredPages(ctx, community, pages, &result)

	// Step 3: Update last_scraped_at (skip in dry-run)
	if !s.config.DryRun && result.Error == "" {
		if scrapedErr := s.client.UpdateScrapedAt(
			ctx, community.ID, time.Now(),
		); scrapedErr != nil {
			s.logger.Warn("failed to update scraped_at",
				infralogger.String("community_id", community.ID),
				infralogger.Error(scrapedErr),
			)
		}
	}

	return result
}

// processDiscoveredPages handles leadership and contact pages.
func (s *Scraper) processDiscoveredPages(
	ctx context.Context,
	community Community,
	pages []leadership.DiscoveredPage,
	result *Result,
) {
	for _, page := range pages {
		text, textErr := s.fetcher.FetchText(ctx, page.URL)
		if textErr != nil {
			s.logger.Warn("failed to fetch page text",
				infralogger.String("url", page.URL),
				infralogger.Error(textErr),
			)
			continue
		}

		switch page.PageType {
		case leadership.PageTypeLeadership:
			s.processLeadershipPage(ctx, community, page.URL, text, result)
		case leadership.PageTypeContact:
			s.processContactPage(ctx, community, page.URL, text, result)
		}
	}
}

// processLeadershipPage extracts leaders and creates people records.
func (s *Scraper) processLeadershipPage(
	ctx context.Context,
	community Community,
	pageURL, text string,
	result *Result,
) {
	leaders := leadership.ExtractLeaders(text)
	if len(leaders) == 0 {
		return
	}

	existing, listErr := s.client.ListPeople(ctx, community.ID)
	if listErr != nil {
		s.logger.Warn("failed to list existing people",
			infralogger.String("community_id", community.ID),
			infralogger.Error(listErr),
		)
	}

	existingSet := buildPeopleSet(existing)

	for _, leader := range leaders {
		key := personKey(leader.Name, leader.Role)
		if existingSet[key] {
			result.PeopleSkipped++
			continue
		}

		if s.config.DryRun {
			result.PeopleAdded++
			continue
		}

		person := buildPersonFromLeader(leader, pageURL)
		if createErr := s.client.CreatePerson(
			ctx, community.ID, person,
		); createErr != nil {
			s.logger.Warn("failed to create person",
				infralogger.String("name", leader.Name),
				infralogger.Error(createErr),
			)
			continue
		}
		result.PeopleAdded++
	}
}

// buildPersonFromLeader converts a leadership.Person to a scraper.Person.
func buildPersonFromLeader(leader leadership.Person, pageURL string) Person {
	sourceURL := pageURL
	person := Person{
		Name:       leader.Name,
		Role:       leader.Role,
		DataSource: "crawler",
		Verified:   false,
		IsCurrent:  true,
		SourceURL:  &sourceURL,
	}
	if leader.Email != "" {
		person.Email = &leader.Email
	}
	if leader.Phone != "" {
		person.Phone = &leader.Phone
	}
	return person
}

// processContactPage extracts contact info and upserts band office.
func (s *Scraper) processContactPage(
	ctx context.Context,
	community Community,
	pageURL, text string,
	result *Result,
) {
	contact := leadership.ExtractContact(text)
	if contact.Phone == "" && contact.Email == "" && contact.PostalCode == "" {
		return
	}

	existing, getErr := s.client.GetBandOffice(ctx, community.ID)
	if getErr != nil {
		s.logger.Warn("failed to get existing band office",
			infralogger.String("community_id", community.ID),
			infralogger.Error(getErr),
		)
	}

	if existing != nil && bandOfficeUnchanged(existing, contact) {
		result.OfficeSkipped = true
		return
	}

	if s.config.DryRun {
		result.OfficeUpdated = true
		return
	}

	office := buildBandOfficeFromContact(contact, pageURL)
	if upsertErr := s.client.UpsertBandOffice(
		ctx, community.ID, office,
	); upsertErr != nil {
		s.logger.Warn("failed to upsert band office",
			infralogger.String("community_id", community.ID),
			infralogger.Error(upsertErr),
		)
		return
	}
	result.OfficeUpdated = true
}

// buildBandOfficeFromContact creates a BandOffice from extracted ContactInfo.
func buildBandOfficeFromContact(contact leadership.ContactInfo, pageURL string) BandOffice {
	sourceURL := pageURL
	office := BandOffice{
		DataSource: "crawler",
		Verified:   false,
		SourceURL:  &sourceURL,
	}
	if contact.Phone != "" {
		office.Phone = &contact.Phone
	}
	if contact.Fax != "" {
		office.Fax = &contact.Fax
	}
	if contact.Email != "" {
		office.Email = &contact.Email
	}
	if contact.TollFree != "" {
		office.TollFree = &contact.TollFree
	}
	if contact.PostalCode != "" {
		office.PostalCode = &contact.PostalCode
	}
	return office
}

// buildPeopleSet creates a set of "name|role" keys from existing people.
func buildPeopleSet(people []Person) map[string]bool {
	set := make(map[string]bool, len(people))
	for _, p := range people {
		set[personKey(p.Name, p.Role)] = true
	}
	return set
}

// personKey creates a normalized lookup key from name and role.
func personKey(name, role string) string {
	return strings.ToLower(strings.TrimSpace(name)) + "|" +
		strings.ToLower(strings.TrimSpace(role))
}

// bandOfficeUnchanged compares extracted contact info against existing band office.
func bandOfficeUnchanged(existing *BandOffice, contact leadership.ContactInfo) bool {
	return ptrEquals(existing.Phone, contact.Phone) &&
		ptrEquals(existing.Email, contact.Email) &&
		ptrEquals(existing.Fax, contact.Fax) &&
		ptrEquals(existing.TollFree, contact.TollFree) &&
		ptrEquals(existing.PostalCode, contact.PostalCode)
}

// ptrEquals compares a *string pointer with a string value.
func ptrEquals(ptr *string, val string) bool {
	if ptr == nil {
		return val == ""
	}
	return *ptr == val
}

// PrintDryRunResults outputs results as JSON to stdout.
func PrintDryRunResults(results []Result) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		return fmt.Errorf("encode dry-run results: %w", err)
	}
	return nil
}
