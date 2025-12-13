# Drupal JSON:API Security Guide for Golang Microservice

This guide explains how to secure your Drupal JSON:API and allow your Golang microservice to POST articles.

## Overview

Your setup:
- **JSON:API endpoint**: `/jsonapi/node/article`
- **Authentication module**: `rest_api_authentication` (installed)
- **Required**: Create article content permission

## Step 1: Configure API Authentication

### Option A: API Key Authentication (Recommended for Microservices)

1. **Enable API Key authentication**:
   ```bash
   ddev drush en rest_api_authentication -y
   ```

2. **Configure via UI**:
   - Go to: `/admin/config/services/api_authentication`
   - Select **"API Key"** as the authentication provider
   - Enable it for JSON:API routes

3. **Or configure via Drush**:
   ```bash
   # Set API Key authentication as default
   ddev drush config:set rest_api_authentication.settings provider api_key
   ddev drush cr
   ```

### Option B: OAuth 2.0 (More Secure, More Complex)

If you need OAuth 2.0:
1. Install Simple OAuth: `ddev drush en simple_oauth -y`
2. Configure OAuth clients at `/admin/config/services/oauth`
3. Use client credentials flow for microservices

## Step 2: Create Service Account User

Create a dedicated user account for your Golang microservice:

```bash
# Create a service account user
ddev drush user:create api_service \
  --mail="api-service@example.com" \
  --password="$(openssl rand -base64 32)"
```

## Step 3: Create API Service Role

Create a custom role with minimal required permissions:

```bash
# Create the role
ddev drush role:create api_service_role "API Service Role"

# Grant article creation permission
ddev drush role:perm-add api_service_role "create article content"
ddev drush role:perm-add api_service_role "edit own article content"
ddev drush role:perm-add api_service_role "access jsonapi resource list"
ddev drush role:perm-add api_service_role "access content"

# Assign role to service account
ddev drush user:role:add api_service_role api_service
```

## Step 4: Generate API Key

### Via UI:
1. Go to `/user/[uid]/edit` (for your service account user)
2. Navigate to "API Keys" tab
3. Generate a new API key
4. **Save this key securely** - you'll need it in your Golang service

### Via Drush (if module supports it):
```bash
# Check if there's a drush command for API key generation
ddev drush help | grep api
```

## Step 5: Configure API Key in Drupal

1. Go to `/admin/config/services/api_authentication`
2. Configure:
   - **API Key Header Name**: `X-API-Key` (or `Authorization`)
   - **API Key Location**: Header
   - **Enable for**: JSON:API routes

## Step 6: Golang Implementation Example

Here's a complete example for your Golang microservice:

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

type Article struct {
    Data struct {
        Type       string                 `json:"type"`
        Attributes map[string]interface{} `json:"attributes"`
    } `json:"data"`
}

type ArticleResponse struct {
    Data struct {
        ID         string                 `json:"id"`
        Type       string                 `json:"type"`
        Attributes map[string]interface{} `json:"attributes"`
    } `json:"data"`
}

type DrupalClient struct {
    BaseURL    string
    APIKey     string
    HTTPClient *http.Client
}

func NewDrupalClient(baseURL, apiKey string) *DrupalClient {
    return &DrupalClient{
        BaseURL: baseURL,
        APIKey:  apiKey,
        HTTPClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (c *DrupalClient) CreateArticle(title, body string, additionalFields map[string]interface{}) (*ArticleResponse, error) {
    // Build article payload
    article := Article{
        Data: struct {
            Type       string                 `json:"type"`
            Attributes map[string]interface{} `json:"attributes"`
        }{
            Type: "node--article",
            Attributes: map[string]interface{}{
                "title": title,
                "body": map[string]interface{}{
                    "value":  body,
                    "format": "basic_html",
                },
                "status": true, // Published
            },
        },
    }

    // Add any additional fields
    for k, v := range additionalFields {
        article.Data.Attributes[k] = v
    }

    // Marshal to JSON
    jsonData, err := json.Marshal(article)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal article: %w", err)
    }

    // Create request
    url := fmt.Sprintf("%s/jsonapi/node/article", c.BaseURL)
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    // Set headers
    req.Header.Set("Content-Type", "application/vnd.api+json")
    req.Header.Set("Accept", "application/vnd.api+json")
    req.Header.Set("X-API-Key", c.APIKey) // Or use Authorization header

    // Make request
    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to make request: %w", err)
    }
    defer resp.Body.Close()

    // Read response
    bodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    // Check status code
    if resp.StatusCode != http.StatusCreated {
        return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(bodyBytes))
    }

    // Parse response
    var articleResp ArticleResponse
    if err := json.Unmarshal(bodyBytes, &articleResp); err != nil {
        return nil, fmt.Errorf("failed to unmarshal response: %w", err)
    }

    return &articleResp, nil
}

// Example usage
func main() {
    client := NewDrupalClient(
        "https://your-drupal-site.com",
        "your-api-key-here",
    )

    article, err := client.CreateArticle(
        "My Article Title",
        "<p>Article body content here</p>",
        map[string]interface{}{
            "promote": true,
            "sticky":  false,
        },
    )

    if err != nil {
        fmt.Printf("Error creating article: %v\n", err)
        return
    }

    fmt.Printf("Article created successfully! ID: %s\n", article.Data.ID)
}
```

## Step 7: Security Best Practices

### 1. Use HTTPS Only
- Always use HTTPS in production
- Never send API keys over HTTP

### 2. Rotate API Keys Regularly
- Set up a process to rotate API keys every 90 days
- Revoke old keys immediately

### 3. Limit Permissions
- Only grant the minimum permissions needed
- Don't give the service account admin rights

### 4. Rate Limiting
Consider implementing rate limiting:
```php
// In a custom module or settings.php
$settings['rest_api_authentication_rate_limit'] = [
  'api_key' => [
    'requests' => 100,
    'window' => 3600, // per hour
  ],
];
```

### 5. IP Whitelisting (Optional)
Restrict API access to specific IPs:
```php
// In settings.php or custom module
$settings['rest_api_authentication_allowed_ips'] = [
  '10.0.0.0/8',      // Internal network
  '192.168.1.100',   // Specific microservice IP
];
```

### 6. Monitor API Usage
- Enable Drupal's watchdog logging
- Monitor for suspicious activity
- Set up alerts for failed authentication attempts

## Step 8: Testing

Test your setup:

```bash
# Test with curl
curl -X POST https://your-site.com/jsonapi/node/article \
  -H "Content-Type: application/vnd.api+json" \
  -H "Accept: application/vnd.api+json" \
  -H "X-API-Key: your-api-key-here" \
  -d '{
    "data": {
      "type": "node--article",
      "attributes": {
        "title": "Test Article",
        "body": {
          "value": "<p>Test content</p>",
          "format": "basic_html"
        },
        "status": true
      }
    }
  }'
```

## Troubleshooting

### 403 Forbidden
- Check user has "create article content" permission
- Verify API key is correct
- Check API key is associated with correct user

### 401 Unauthorized
- Verify API key authentication is enabled
- Check API key header name matches configuration
- Ensure API key hasn't been revoked

### 422 Unprocessable Entity
- Check required fields are included
- Verify field format matches Drupal's expectations
- Check field values are valid

## Additional Resources

- [Drupal JSON:API Documentation](https://www.drupal.org/docs/core-modules-and-themes/core-modules/jsonapi-module)
- [REST API Authentication Module](https://www.drupal.org/project/rest_api_authentication)
- [JSON:API Specification](https://jsonapi.org/)

