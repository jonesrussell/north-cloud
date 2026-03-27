package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// listClaudrielLeads serves GET /api/leads for Claudriel's NorthCloudLeadFetcher.
// Optional bearer auth when Auth.LeadsAPIKey is configured.
func (r *Router) listClaudrielLeads(c *gin.Context) {
	key := strings.TrimSpace(r.cfg.Auth.LeadsAPIKey)
	if key != "" {
		auth := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})

			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
		if len(token) != len(key) || subtle.ConstantTimeCompare([]byte(token), []byte(key)) != 1 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})

			return
		}
	}

	items, err := r.repo.ListClaudrielLeads(c.Request.Context())
	if err != nil {
		r.log.Error("Failed to list Claudriel leads",
			infralogger.Error(err),
			infralogger.String("path", c.Request.URL.Path),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list leads"})

		return
	}

	out := make([]gin.H, 0, len(items))
	for i := range items {
		out = append(out, gin.H{
			"id":            items[i].ID.String(),
			"title":         items[i].Title,
			"name":          items[i].Title,
			"description":   items[i].Description,
			"contact_name":  items[i].ContactName,
			"contact_email": items[i].ContactEmail,
			"url":           items[i].URL,
			"source_url":    items[i].URL,
			"closing_date":  items[i].ClosingDate,
			"budget":        items[i].Budget,
			"value":         items[i].Budget,
			"sector":        items[i].Sector,
			"category":      items[i].Sector,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}
