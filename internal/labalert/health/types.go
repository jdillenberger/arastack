package health

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
