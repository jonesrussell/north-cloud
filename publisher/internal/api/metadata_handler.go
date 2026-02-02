package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// decodeJSONBody decodes a JSON response body into the target interface
func decodeJSONBody(body io.Reader, target any) error {
	return json.NewDecoder(body).Decode(target)
}

// Known topics that the system recognizes
// These correspond to the classifier's topic detection
var knownTopics = []string{
	// Crime sub-categories
	"violent_crime",
	"property_crime",
	"drug_crime",
	"organized_crime",
	"criminal_justice",
	// General topics
	"local_news",
	"politics",
	"business",
	"sports",
	"entertainment",
	"weather",
	"health",
	"education",
	"technology",
}

// listTopics returns the list of known topics that articles can be tagged with
// GET /api/v1/topics
func (r *Router) listTopics(c *gin.Context) {
	// Sort topics alphabetically for consistent output
	sortedTopics := make([]string, len(knownTopics))
	copy(sortedTopics, knownTopics)
	sort.Strings(sortedTopics)

	// Build topic info with Layer 1 channel names
	topicInfo := make([]gin.H, len(sortedTopics))
	for i, topic := range sortedTopics {
		topicInfo[i] = gin.H{
			"name":           topic,
			"layer1_channel": "articles:" + topic,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"topics": topicInfo,
		"count":  len(topicInfo),
		"note":   "Layer 1 channels are automatically created for each topic (articles:{topic})",
	})
}

// listIndexes returns the list of discovered Elasticsearch indexes
// GET /api/v1/indexes
func (r *Router) listIndexes(c *gin.Context) {
	if r.esClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Elasticsearch client not configured",
		})
		return
	}

	const indexQueryTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(c.Request.Context(), indexQueryTimeout)
	defer cancel()

	// Query Elasticsearch for indexes matching *_classified_content
	res, err := r.esClient.Cat.Indices(
		r.esClient.Cat.Indices.WithContext(ctx),
		r.esClient.Cat.Indices.WithIndex("*_classified_content"),
		r.esClient.Cat.Indices.WithFormat("json"),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to query Elasticsearch indexes",
		})
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Elasticsearch returned an error",
		})
		return
	}

	// Parse the response
	var indexes []map[string]any
	if decodeErr := decodeJSONBody(res.Body, &indexes); decodeErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to parse Elasticsearch response",
		})
		return
	}

	// Extract index names and build response
	indexInfo := make([]gin.H, 0, len(indexes))
	for _, idx := range indexes {
		name, ok := idx["index"].(string)
		if !ok || name == "" {
			continue
		}

		// Extract source name from index (e.g., "sudbury_classified_content" -> "sudbury")
		sourceName := strings.TrimSuffix(name, "_classified_content")

		info := gin.H{
			"name":   name,
			"source": sourceName,
		}

		// Add optional fields if present
		if health, hasHealth := idx["health"].(string); hasHealth {
			info["health"] = health
		}
		if status, hasStatus := idx["status"].(string); hasStatus {
			info["status"] = status
		}
		if docsCount, hasDocs := idx["docs.count"].(string); hasDocs {
			info["docs_count"] = docsCount
		}

		indexInfo = append(indexInfo, info)
	}

	// Sort by name for consistent output
	sort.Slice(indexInfo, func(i, j int) bool {
		nameI, _ := indexInfo[i]["name"].(string)
		nameJ, _ := indexInfo[j]["name"].(string)
		return nameI < nameJ
	})

	c.JSON(http.StatusOK, gin.H{
		"indexes": indexInfo,
		"count":   len(indexInfo),
		"note":    "Indexes matching *_classified_content pattern",
	})
}
