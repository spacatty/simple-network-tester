package config

import "loadtester/backend/internal/model"

var DefaultUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 13_6_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64; rv:126.0) Gecko/20100101 Firefox/126.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
}

func NormalizeConfig(cfg *model.LoadTestConfig) {
	if cfg.Method == "" {
		cfg.Method = "GET"
	}
	if cfg.TimeoutMS <= 0 {
		cfg.TimeoutMS = 5000
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 5
	}
	if cfg.Requests <= 0 && cfg.DurationSeconds <= 0 {
		cfg.Requests = 100
	}
	if len(cfg.SuccessStatuses) == 0 {
		cfg.SuccessStatuses = []int{200}
	}
	if cfg.UserAgent.Mode == "" {
		cfg.UserAgent.Mode = model.UserAgentModeDefaultRotate
	}
	if cfg.Headers == nil {
		cfg.Headers = map[string]string{}
	}
}
