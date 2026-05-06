package model

import "time"

type BodyType string

const (
	BodyTypeNone      BodyType = "none"
	BodyTypeJSON      BodyType = "json"
	BodyTypeForm      BodyType = "form"
	BodyTypeMultipart BodyType = "multipart"
)

type BodyPartFile struct {
	FieldName string `json:"fieldName"`
	FileID    string `json:"fileId"`
	FileName  string `json:"fileName"`
}

type RequestBody struct {
	Type            BodyType          `json:"type"`
	JSON            string            `json:"json"`
	Form            map[string]string `json:"form"`
	MultipartFields map[string]string `json:"multipartFields"`
	MultipartFiles  []BodyPartFile    `json:"multipartFiles"`
}

type UserAgentMode string

const (
	UserAgentModeDefaultRotate UserAgentMode = "default_rotate"
	UserAgentModeCustomRotate  UserAgentMode = "custom_rotate"
	UserAgentModeFixed         UserAgentMode = "fixed"
)

type UserAgentConfig struct {
	Mode       UserAgentMode `json:"mode"`
	FixedValue string        `json:"fixedValue"`
	CustomList []string      `json:"customList"`
}

type ProxyConfig struct {
	Enabled bool `json:"enabled"`
}

type LoadTestConfig struct {
	Name            string            `json:"name"`
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Headers         map[string]string `json:"headers"`
	Body            RequestBody       `json:"body"`
	SuccessStatuses []int             `json:"successStatuses"`
	TimeoutMS       int               `json:"timeoutMs"`
	Concurrency     int               `json:"concurrency"`
	Requests        int               `json:"requests"`
	DurationSeconds int               `json:"durationSeconds"`
	RateLimitRPS    int               `json:"rateLimitRps"`
	UserAgent       UserAgentConfig   `json:"userAgent"`
	Proxy           ProxyConfig       `json:"proxy"`
}

type RunState string

const (
	RunStateRunning   RunState = "running"
	RunStateCompleted RunState = "completed"
	RunStateFailed    RunState = "failed"
)

type RunSample struct {
	RunID         string    `json:"runId"`
	Timestamp     time.Time `json:"timestamp"`
	LatencyMS     int64     `json:"latencyMs"`
	StatusCode    int       `json:"statusCode"`
	Success       bool      `json:"success"`
	ErrorCategory string    `json:"errorCategory"`
	ErrorMessage  string    `json:"errorMessage"`
	ProxyAddress  string    `json:"proxyAddress"`
	BytesSent     int64     `json:"bytesSent"`
	BytesReceived int64     `json:"bytesReceived"`
}

type RunSummary struct {
	RunID          string         `json:"runId"`
	State          RunState       `json:"state"`
	StartedAt      time.Time      `json:"startedAt"`
	CompletedAt    *time.Time     `json:"completedAt"`
	TotalRequests  int            `json:"totalRequests"`
	SuccessCount   int            `json:"successCount"`
	ErrorCount     int            `json:"errorCount"`
	AvgLatencyMS   float64        `json:"avgLatencyMs"`
	MinLatencyMS   int64          `json:"minLatencyMs"`
	MaxLatencyMS   int64          `json:"maxLatencyMs"`
	P50LatencyMS   int64          `json:"p50LatencyMs"`
	P95LatencyMS   int64          `json:"p95LatencyMs"`
	P99LatencyMS   int64          `json:"p99LatencyMs"`
	RPS            float64        `json:"rps"`
	StatusCounts   map[int]int    `json:"statusCounts"`
	ErrorBreakdown map[string]int `json:"errorBreakdown"`
	Config         LoadTestConfig `json:"config"`
}

type ProxyRecord struct {
	Address      string     `json:"address"`
	Active       bool       `json:"active"`
	LastChecked  *time.Time `json:"lastChecked"`
	LastLatency  int64      `json:"lastLatency"`
	LastError    string     `json:"lastError"`
	SuccessCount int        `json:"successCount"`
	FailureCount int        `json:"failureCount"`
}
