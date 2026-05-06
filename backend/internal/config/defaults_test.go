package config

import (
	"testing"

	"loadtester/backend/internal/model"
)

func TestNormalizeConfig(t *testing.T) {
	cfg := model.LoadTestConfig{}
	NormalizeConfig(&cfg)
	if cfg.Method != "GET" {
		t.Fatalf("expected GET method")
	}
	if cfg.TimeoutMS == 0 || cfg.Concurrency == 0 {
		t.Fatalf("expected defaults for timeout and concurrency")
	}
	if len(cfg.SuccessStatuses) != 1 || cfg.SuccessStatuses[0] != 200 {
		t.Fatalf("expected default success statuses")
	}
}
