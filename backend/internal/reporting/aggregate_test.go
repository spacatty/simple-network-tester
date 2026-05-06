package reporting

import (
	"testing"
	"time"

	"loadtester/backend/internal/model"
)

func TestAccumulatorSummary(t *testing.T) {
	start := time.Now().Add(-2 * time.Second)
	acc := NewAccumulator("run_1", start, model.LoadTestConfig{})
	acc.Add(model.RunSample{LatencyMS: 100, Success: true, StatusCode: 200})
	acc.Add(model.RunSample{LatencyMS: 300, Success: false, StatusCode: 500, ErrorCategory: "status_mismatch"})
	acc.Add(model.RunSample{LatencyMS: 200, Success: true, StatusCode: 200})

	s := acc.Summary(model.RunStateCompleted, time.Now())
	if s.TotalRequests != 3 {
		t.Fatalf("expected 3 requests, got %d", s.TotalRequests)
	}
	if s.P95LatencyMS != 300 {
		t.Fatalf("expected p95 300, got %d", s.P95LatencyMS)
	}
	if s.ErrorBreakdown["status_mismatch"] != 1 {
		t.Fatalf("expected one status mismatch")
	}
}
