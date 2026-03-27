package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list leads"})

		return
	}

	out := make([]gin.H, 0, len(items))
	for _, row := range items {
		out = append(out, gin.H{
			"id":            row.ID.String(),
			"title":         row.Title,
			"name":          row.Title,
			"description":   row.Description,
			"contact_name":  row.ContactName,
			"contact_email": row.ContactEmail,
			"url":           row.URL,
			"source_url":    row.URL,
			"closing_date":  row.ClosingDate,
			"budget":        row.Budget,
			"value":         row.Budget,
			"sector":        row.Sector,
			"category":      row.Sector,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}
