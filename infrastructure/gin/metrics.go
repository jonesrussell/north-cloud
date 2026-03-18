package gin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RegisterMetricsRoute adds the /metrics endpoint for Prometheus scraping.
func RegisterMetricsRoute(router *gin.Engine) {
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
}

// MetricsHandler returns the Prometheus HTTP handler for custom registration.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
