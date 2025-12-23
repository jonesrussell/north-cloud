package domain

// HealthStatus represents the health status of the service
type HealthStatus struct {
	Status    string            `json:"status"` // healthy, unhealthy, degraded
	Version   string            `json:"version"`
	Checks    map[string]string `json:"checks"`
	Timestamp string            `json:"timestamp"`
}
