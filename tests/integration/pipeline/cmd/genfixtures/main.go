// Command genfixtures generates nc-http-proxy fixture files for pipeline
// integration tests. It creates the .json metadata and .body files with the
// correct cache key hashes.
//
// Run from repository root:
//
//	GOWORK=off go run ./tests/integration/pipeline/cmd/genfixtures
//
// Or from module directory:
//
//	cd tests/integration/pipeline && GOWORK=off go run ./cmd/genfixtures
//
// The output directory defaults to crawler/fixtures/ relative to the
// repository root. Pass -dir to override.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const hashPrefixLen = 12

type fixture struct {
	Domain    string
	URL       string
	Method    string
	UserAgent string
	Body      string
}

type cacheMetadata struct {
	Request    requestMeta  `json:"request"`
	Response   responseMeta `json:"response"`
	RecordedAt string       `json:"recorded_at"`
	CacheKey   string       `json:"cache_key"`
}

type requestMeta struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type responseMeta struct {
	Status        int               `json:"status"`
	Headers       map[string]string `json:"headers"`
	WasCompressed bool              `json:"was_compressed"`
}

func generateCacheKey(method, rawURL, userAgent string) string {
	headerHash := hashHeaders(userAgent)
	combined := rawURL + "\n" + headerHash
	hash := sha256.Sum256([]byte(combined))
	shortHash := hex.EncodeToString(hash[:])[:hashPrefixLen]
	return method + "_" + shortHash
}

func hashHeaders(userAgent string) string {
	combined := userAgent + "\n"
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

//nolint:funlen // generator script, not production code
func main() {
	fixturesDir := flag.String("dir", "crawler/fixtures", "output directory for fixtures")
	flag.Parse()

	fixtures := []fixture{
		{
			Domain:    "fixture-news-site-com",
			URL:       "https://fixture-news-site.com/article/local-tech-company-expands",
			Method:    "GET",
			UserAgent: "",
			Body:      newsArticleHTML,
		},
		{
			Domain:    "fixture-news-site-com",
			URL:       "https://fixture-news-site.com/listings/businesses",
			Method:    "GET",
			UserAgent: "",
			Body:      listingPageHTML,
		},
		{
			Domain:    "fixture-news-site-com",
			URL:       "https://fixture-news-site.com/article/downtown-robbery-arrests",
			Method:    "GET",
			UserAgent: "",
			Body:      crimeArticleHTML,
		},
	}

	for _, f := range fixtures {
		domainDir := filepath.Join(*fixturesDir, f.Domain)

		if mkdirErr := os.MkdirAll(domainDir, 0o755); mkdirErr != nil {
			fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", domainDir, mkdirErr)
			os.Exit(1)
		}

		cacheKey := generateCacheKey(f.Method, f.URL, f.UserAgent)
		now := time.Now().UTC().Format(time.RFC3339)

		meta := cacheMetadata{
			Request: requestMeta{
				Method: f.Method,
				URL:    f.URL,
				Headers: map[string]string{
					"User-Agent": f.UserAgent,
				},
			},
			Response: responseMeta{
				Status: 200,
				Headers: map[string]string{
					"Content-Type": "text/html; charset=utf-8",
				},
				WasCompressed: false,
			},
			RecordedAt: now,
			CacheKey:   cacheKey,
		}

		metaJSON, marshalErr := json.MarshalIndent(meta, "", "  ")
		if marshalErr != nil {
			fmt.Fprintf(os.Stderr, "marshal metadata: %v\n", marshalErr)
			os.Exit(1)
		}

		metaPath := filepath.Join(domainDir, cacheKey+".json")
		bodyPath := filepath.Join(domainDir, cacheKey+".body")

		if writeErr := os.WriteFile(metaPath, metaJSON, 0o644); writeErr != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", metaPath, writeErr)
			os.Exit(1)
		}

		if writeErr := os.WriteFile(bodyPath, []byte(f.Body), 0o644); writeErr != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", bodyPath, writeErr)
			os.Exit(1)
		}

		fmt.Printf("created %s (.json + .body)\n", cacheKey)
	}

	fmt.Println("done")
}

// newsArticleHTML is a minimal news article that should classify as:
// - content_type: article
// - quality_score: medium-high (has title, body, metadata)
// - topics: technology, local
const newsArticleHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Local Tech Company Expands Operations to Northern Ontario</title>
  <meta name="description" content="A Sudbury-based technology company announced plans to expand operations across Northern Ontario, creating 200 new jobs.">
  <meta property="og:title" content="Local Tech Company Expands Operations to Northern Ontario">
  <meta property="og:description" content="A Sudbury-based technology company announced plans to expand operations across Northern Ontario, creating 200 new jobs.">
  <meta property="og:type" content="article">
  <meta property="og:url" content="https://fixture-news-site.com/article/local-tech-company-expands">
  <meta name="author" content="Jane Smith">
  <link rel="canonical" href="https://fixture-news-site.com/article/local-tech-company-expands">
</head>
<body>
  <article>
    <h1>Local Tech Company Expands Operations to Northern Ontario</h1>
    <p class="byline">By Jane Smith | Published January 15, 2026</p>
    <p>A Sudbury-based technology company announced major expansion plans today that will bring hundreds of new jobs to Northern Ontario communities. The company, which specializes in cloud computing and data analytics, plans to open new offices in Timmins, Sault Ste. Marie, and Thunder Bay.</p>
    <p>The expansion is expected to create approximately 200 new positions over the next two years, with roles ranging from software development to customer support. Company CEO John Anderson said the move reflects growing demand for tech services in northern communities.</p>
    <p>"We've seen tremendous growth in the past three years, and we believe there's significant untapped talent in Northern Ontario," Anderson said in a statement. "This expansion allows us to serve our clients better while investing in communities that have been underserved by the tech sector."</p>
    <p>The provincial government has pledged $5 million in support through its Northern Development Fund to assist with the expansion. Minister of Northern Development Sarah Chen praised the company's commitment to the region.</p>
    <p>"This investment shows that Northern Ontario is an attractive destination for technology companies," Chen said. "We're committed to building the infrastructure and workforce needed to support continued growth in our northern communities."</p>
    <p>The first new office is expected to open in Timmins by June 2026, with the remaining locations following by the end of the year. The company has already begun recruitment efforts and plans to partner with local colleges and universities for training programs.</p>
    <p>Industry analysts say the expansion is part of a broader trend of technology companies moving operations outside of major urban centres, driven by lower costs and improved internet infrastructure in smaller communities.</p>
  </article>
</body>
</html>`

// listingPageHTML is a page with listings that should classify as:
// - content_type: page or listing (not article)
// - publisher should skip it
const listingPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Business Directory - Northern Ontario</title>
  <meta name="description" content="Browse businesses in Northern Ontario.">
  <meta property="og:title" content="Business Directory - Northern Ontario">
  <meta property="og:type" content="website">
</head>
<body>
  <h1>Business Directory</h1>
  <ul>
    <li><a href="/business/acme-corp">Acme Corp</a> - Mining Equipment</li>
    <li><a href="/business/northern-tech">Northern Tech Solutions</a> - IT Services</li>
    <li><a href="/business/timber-co">Timber Co</a> - Forestry</li>
    <li><a href="/business/lake-lodge">Lake Lodge</a> - Tourism</li>
    <li><a href="/business/gold-mining">Gold Mining Inc</a> - Mining</li>
  </ul>
  <nav>
    <a href="?page=2">Next Page</a>
  </nav>
</body>
</html>`

// crimeArticleHTML is a crime news article that should classify as:
// - content_type: article
// - topics: crime (violent_crime or property_crime)
// - crime detection should trigger
// - should route to crime channel
const crimeArticleHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Three Arrested Following Downtown Robbery Spree</title>
  <meta name="description" content="Police have arrested three suspects in connection with a series of armed robberies in downtown Sudbury over the past week.">
  <meta property="og:title" content="Three Arrested Following Downtown Robbery Spree">
  <meta property="og:description" content="Police have arrested three suspects in connection with a series of armed robberies in downtown Sudbury over the past week.">
  <meta property="og:type" content="article">
  <meta property="og:url" content="https://fixture-news-site.com/article/downtown-robbery-arrests">
  <meta name="author" content="Mike Johnson">
  <meta name="keywords" content="crime, robbery, arrest, Sudbury, police">
  <link rel="canonical" href="https://fixture-news-site.com/article/downtown-robbery-arrests">
</head>
<body>
  <article>
    <h1>Three Arrested Following Downtown Robbery Spree</h1>
    <p class="byline">By Mike Johnson | Published January 20, 2026</p>
    <p>Greater Sudbury Police have arrested three suspects in connection with a string of armed robberies that targeted downtown businesses over the past week. The arrests were made during an early morning operation on Tuesday.</p>
    <p>Police say the suspects, aged 22, 25, and 31, are believed responsible for at least five separate robbery incidents at convenience stores and gas stations in the downtown core. In each case, the suspects allegedly brandished weapons and demanded cash from employees.</p>
    <p>"These arrests represent a significant breakthrough in our investigation," said Inspector David Williams of the Greater Sudbury Police. "We take violent crime very seriously, and our officers worked around the clock to identify and apprehend these individuals."</p>
    <p>No injuries were reported during any of the robberies, though several employees reported being traumatized by the incidents. Victim services have been offered to all affected workers.</p>
    <p>The three suspects face multiple charges including armed robbery, uttering threats, and possession of weapons. They are scheduled to appear in court later this week. Police say the investigation is ongoing and additional charges may be laid.</p>
    <p>Downtown business owners expressed relief at the arrests. The Sudbury Downtown Business Association had been calling for increased police presence in the area following the crime spree.</p>
    <p>"Our members were frightened and some were considering closing early," said association president Lisa Park. "We're grateful to the police for their swift action in this matter."</p>
  </article>
</body>
</html>`
