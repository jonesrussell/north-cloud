# Payload to Drupal Field Mapping Guide

This guide shows how to transform your Elasticsearch payload into Drupal JSON:API format for creating articles.

## Field Mapping Table

| Payload Field (`_source`) | Drupal Field | Field Type | Notes |
|---------------------------|--------------|------------|-------|
| `id` | `field_external_id` | String | Store original Elasticsearch ID |
| `title` | `title` | String | Standard Drupal field |
| `body` | `body` | Text (formatted, long) | Main article content |
| `author` | `field_author` | String | Author name |
| `byline_name` | `field_byline_name` | String | Byline name |
| `published_date` | `field_published_date` | DateTime | ISO 8601 format |
| `source` | `field_source` | Link | Source URL |
| `intro` | `field_intro` | Text (long) | Article introduction |
| `description` | `field_description` | String (long) | Article description |
| `og_title` | `field_og_title` | String | Open Graph title |
| `og_description` | `field_og_description` | Text (long) | Open Graph description |
| `og_image` | `field_og_image` | Link | Open Graph image URL |
| `og_url` | `field_og_url` | Link | Open Graph URL |
| `canonical_url` | `field_canonical_url` | Link | Canonical URL |
| `word_count` | `field_word_count` | Integer | Word count |
| `category` | `field_category` | String (long) | Category string |
| `section` | `field_section` | String (long) | Section string |
| `keywords` | `field_keywords` | String (long) | Keywords string |
| `created_at` | `created` | DateTime | Use Drupal's created field |
| `updated_at` | `changed` | DateTime | Use Drupal's changed field |
| - | `field_group` | Entity Reference | **Required** - Must be provided |
| - | `field_url` | Link | **Required** - Must be provided |

## Complete JSON:API Payload Example

```json
{
  "data": {
    "type": "node--article",
    "attributes": {
      "title": "Canada News | Latest National Headlines | Mid-North Monitor",
      "body": {
        "value": "Curling legend Colleen Jones remembered for living with 'joy and gratitude' Top-calibre curler was also a reporter, coach, mother and avid cyclist   Canada",
        "format": "basic_html"
      },
      "field_author": "Author Name",
      "field_byline_name": "Byline Name",
      "field_published_date": "2025-12-07T04:54:02",
      "field_intro": {
        "value": "Top-calibre curler was also a reporter, coach, mother and avid cyclist...",
        "format": "plain_text"
      },
      "field_description": "Watch news, exclusive videos and updates on national issues from all over Canada.",
      "field_og_title": "Mid-North Monitor",
      "field_og_description": {
        "value": "Top-calibre curler was also a reporter, coach, mother and avid cyclist...",
        "format": "plain_text"
      },
      "field_word_count": 0,
      "field_category": "Canada    Canada    Canada    Canada...",
      "field_section": "NewsCanadian PoliticsCanadian Politics...",
      "field_keywords": null,
      "field_external_id": "ff45d76c-61af-459e-971f-bcd66c0195a4",
      "field_url": {
        "uri": "https://www.midnorthmonitor.com/category/news/national/",
        "title": ""
      },
      "field_source": {
        "uri": "https://www.midnorthmonitor.com/category/news/national/",
        "title": ""
      },
      "field_og_image": {
        "uri": "https://dcs-static.gprod.postmedia.digital/20.1.2/websites/images/ogimage.png",
        "title": ""
      },
      "field_og_url": {
        "uri": "",
        "title": ""
      },
      "field_canonical_url": {
        "uri": "https://www.midnorthmonitor.com/category/news/national/",
        "title": ""
      },
      "status": true,
      "created": "2025-12-07T04:54:02",
      "changed": "2025-12-07T04:54:02"
    },
    "relationships": {
      "field_group": {
        "data": {
          "type": "group--news",
          "id": "e3d024a6-5f6f-4be8-8f3d-75639075959c"
        }
      }
    }
  }
}
```

## Golang Transformation Example

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

// ElasticsearchPayload represents the incoming payload structure
type ElasticsearchPayload struct {
    Source struct {
        ID            string    `json:"id"`
        Title         string    `json:"title"`
        Body          string    `json:"body"`
        Author        string    `json:"author"`
        BylineName    string    `json:"byline_name"`
        PublishedDate string    `json:"published_date"`
        Source        string    `json:"source"`
        Tags          *string   `json:"tags"`
        Intro         string    `json:"intro"`
        Description   string    `json:"description"`
        OGTitle       string    `json:"og_title"`
        OGDescription string    `json:"og_description"`
        OGImage       string    `json:"og_image"`
        OGURL         string    `json:"og_url"`
        CanonicalURL  string    `json:"canonical_url"`
        WordCount     int       `json:"word_count"`
        Category      string    `json:"category"`
        Section       string    `json:"section"`
        Keywords      *string   `json:"keywords"`
        CreatedAt     string    `json:"created_at"`
        UpdatedAt     string    `json:"updated_at"`
    } `json:"_source"`
}

// DrupalArticle represents the Drupal JSON:API format
type DrupalArticle struct {
    Data struct {
        Type       string                 `json:"type"`
        Attributes map[string]interface{} `json:"attributes"`
        Relationships map[string]interface{} `json:"relationships"`
    } `json:"data"`
}

// TransformPayload converts Elasticsearch payload to Drupal format
func TransformPayload(esPayload ElasticsearchPayload, groupUUID string) (*DrupalArticle, error) {
    article := &DrupalArticle{}
    article.Data.Type = "node--article"
    article.Data.Attributes = make(map[string]interface{})
    article.Data.Relationships = make(map[string]interface{})

    // Basic attributes
    article.Data.Attributes["title"] = esPayload.Source.Title
    article.Data.Attributes["status"] = true

    // Body field (formatted text)
    if esPayload.Source.Body != "" {
        article.Data.Attributes["body"] = map[string]interface{}{
            "value":  esPayload.Source.Body,
            "format": "basic_html",
        }
    }

    // Text fields
    if esPayload.Source.Author != "" {
        article.Data.Attributes["field_author"] = esPayload.Source.Author
    }
    if esPayload.Source.BylineName != "" {
        article.Data.Attributes["field_byline_name"] = esPayload.Source.BylineName
    }
    if esPayload.Source.Intro != "" {
        article.Data.Attributes["field_intro"] = map[string]interface{}{
            "value":  esPayload.Source.Intro,
            "format": "plain_text",
        }
    }
    if esPayload.Source.Description != "" {
        article.Data.Attributes["field_description"] = esPayload.Source.Description
    }
    if esPayload.Source.OGTitle != "" {
        article.Data.Attributes["field_og_title"] = esPayload.Source.OGTitle
    }
    if esPayload.Source.OGDescription != "" {
        article.Data.Attributes["field_og_description"] = map[string]interface{}{
            "value":  esPayload.Source.OGDescription,
            "format": "plain_text",
        }
    }
    if esPayload.Source.Category != "" {
        article.Data.Attributes["field_category"] = esPayload.Source.Category
    }
    if esPayload.Source.Section != "" {
        article.Data.Attributes["field_section"] = esPayload.Source.Section
    }
    if esPayload.Source.Keywords != nil && *esPayload.Source.Keywords != "" {
        article.Data.Attributes["field_keywords"] = *esPayload.Source.Keywords
    }
    if esPayload.Source.ID != "" {
        article.Data.Attributes["field_external_id"] = esPayload.Source.ID
    }

    // Integer field
    article.Data.Attributes["field_word_count"] = esPayload.Source.WordCount

    // Date fields
    if esPayload.Source.PublishedDate != "" && esPayload.Source.PublishedDate != "0001-01-01T00:00:00Z" {
        // Parse and format date
        if t, err := time.Parse(time.RFC3339, esPayload.Source.PublishedDate); err == nil {
            article.Data.Attributes["field_published_date"] = t.Format("2006-01-02T15:04:05")
        }
    }

    // Created and updated dates
    if esPayload.Source.CreatedAt != "" {
        if t, err := time.Parse(time.RFC3339, esPayload.Source.CreatedAt); err == nil {
            article.Data.Attributes["created"] = t.Format("2006-01-02T15:04:05")
        }
    }
    if esPayload.Source.UpdatedAt != "" {
        if t, err := time.Parse(time.RFC3339, esPayload.Source.UpdatedAt); err == nil {
            article.Data.Attributes["changed"] = t.Format("2006-01-02T15:04:05")
        }
    }

    // Relationships - Link fields
    // Required: field_group
    article.Data.Relationships["field_group"] = map[string]interface{}{
        "data": map[string]interface{}{
            "type": "group--news",
            "id":   groupUUID,
        },
    }

    // Required: field_url (use canonical_url or source as fallback)
    urlValue := esPayload.Source.CanonicalURL
    if urlValue == "" {
        urlValue = esPayload.Source.Source
    }
    if urlValue != "" {
        article.Data.Attributes["field_url"] = map[string]interface{}{
            "uri":   urlValue,
            "title": "",
        }
    }

    // Optional link fields (these are attributes, not relationships)
    if esPayload.Source.Source != "" {
        article.Data.Attributes["field_source"] = map[string]interface{}{
            "uri":   esPayload.Source.Source,
            "title": "",
        }
    }

    if esPayload.Source.OGImage != "" {
        article.Data.Attributes["field_og_image"] = map[string]interface{}{
            "uri":   esPayload.Source.OGImage,
            "title": "",
        }
    }

    if esPayload.Source.OGURL != "" {
        article.Data.Attributes["field_og_url"] = map[string]interface{}{
            "uri":   esPayload.Source.OGURL,
            "title": "",
        }
    }

    if esPayload.Source.CanonicalURL != "" {
        article.Data.Attributes["field_canonical_url"] = map[string]interface{}{
            "uri":   esPayload.Source.CanonicalURL,
            "title": "",
        }
    }

    return article, nil
}

// CreateArticleInDrupal sends the article to Drupal
func CreateArticleInDrupal(drupalURL, apiKey string, article *DrupalArticle) error {
    jsonData, err := json.Marshal(article)
    if err != nil {
        return fmt.Errorf("failed to marshal article: %w", err)
    }

    url := fmt.Sprintf("%s/jsonapi/node/article", drupalURL)
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/vnd.api+json")
    req.Header.Set("Accept", "application/vnd.api+json")
    req.Header.Set("X-API-Key", apiKey)

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("failed to make request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
    }

    return nil

    return nil
}

// Example usage
func main() {
    // Parse your Elasticsearch payload
    var esPayload ElasticsearchPayload
    // ... load from your source ...

    // Transform to Drupal format
    groupUUID := "e3d024a6-5f6f-4be8-8f3d-75639075959c" // Your group UUID
    drupalArticle, err := TransformPayload(esPayload, groupUUID)
    if err != nil {
        fmt.Printf("Error transforming payload: %v\n", err)
        return
    }

    // Send to Drupal
    err = CreateArticleInDrupal(
        "https://your-drupal-site.com",
        "your-api-key",
        drupalArticle,
    )
    if err != nil {
        fmt.Printf("Error creating article: %v\n", err)
        return
    }

    fmt.Println("Article created successfully!")
}
```

## Important Notes

### Required Fields
- `field_group` - **MUST** be provided (entity reference to Group)
- `field_url` - **MUST** be provided (link field)
- `title` - Standard Drupal field, required

### Date Handling
- Dates should be in ISO 8601 format: `2006-01-02T15:04:05`
- Invalid dates like `0001-01-01T00:00:00Z` should be skipped
- Drupal will use current time if dates are not provided

### Link Fields Format
Link fields are attributes (not relationships) and use this format:
```json
"field_url": {
  "uri": "https://example.com",
  "title": "Optional Title"
}
```

### Text Format Fields
Fields that support text formats (`body`, `field_intro`, `field_og_description`) need:
```json
{
  "value": "Text content",
  "format": "basic_html"  // or "plain_text"
}
```

### Empty/Null Values
- Empty strings can be omitted from the payload
- Null values should be omitted
- Drupal will use defaults for optional fields

## Testing

Use the test script to verify your payload:

```bash
./scripts/test-api.sh https://your-site.com your-api-key
```

Or use curl directly:

```bash
curl -X POST https://your-site.com/jsonapi/node/article \
  -H "Content-Type: application/vnd.api+json" \
  -H "Accept: application/vnd.api+json" \
  -H "X-API-Key: your-api-key" \
  -d @payload.json
```

