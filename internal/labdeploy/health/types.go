package health

import "time"

// Status represents the health state of an app.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusStarting  Status = "starting"
	StatusUnknown   Status = "unknown"
	StatusNone      Status = "none"
)

// Result holds the outcome of a health check.
type Result struct {
	App    string `json:"app"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// Timeout returns the configured timeout for an app's health check, defaulting to 5 seconds.
func Timeout(checkTimeout string) time.Duration {
	if checkTimeout != "" {
		if d, err := time.ParseDuration(checkTimeout); err == nil && d > 0 {
			return d
		}
	}
	return 5 * time.Second
}
