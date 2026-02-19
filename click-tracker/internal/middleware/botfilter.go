package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// botPatterns are known bot User-Agent substrings (lowercase).
var botPatterns = []string{
	"googlebot", "bingbot", "slurp", "duckduckbot",
	"baiduspider", "yandexbot", "facebookexternalhit",
	"twitterbot", "rogerbot", "linkedinbot", "embedly",
	"quora link preview", "showyoubot", "outbrain",
	"pinterest", "applebot", "semrushbot", "ahrefsbot",
	"mj12bot", "dotbot", "petalbot", "bytespider",
}

// BotFilter sets c.Set("is_bot", true) for known bot user agents.
// The handler can check this flag to skip event logging while still redirecting.
func BotFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		ua := strings.ToLower(c.Request.UserAgent())
		if ua == "" || isBot(ua) {
			c.Set("is_bot", true)
		}
		c.Next()
	}
}

func isBot(ua string) bool {
	for _, pattern := range botPatterns {
		if strings.Contains(ua, pattern) {
			return true
		}
	}
	return false
}
