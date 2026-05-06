package reporting

import (
	"math"
	"sort"
	"time"

	"loadtester/backend/internal/model"
)

type Accumulator struct {
	runID       string
	startedAt   time.Time
	config      model.LoadTestConfig
	latencies   []int64
	successes   int
	errors      int
	statuses    map[int]int
	errorByType map[string]int
	total       int
	min         int64
	max         int64
	sum         int64
}

func NewAccumulator(runID string, startedAt time.Time, cfg model.LoadTestConfig) *Accumulator {
	return &Accumulator{
		runID:       runID,
		startedAt:   startedAt,
		config:      cfg,
		statuses:    map[int]int{},
		errorByType: map[string]int{},
		min:         math.MaxInt64,
	}
}

func (a *Accumulator) Add(sample model.RunSample) {
	a.total++
	if sample.Success {
		a.successes++
	} else {
		a.errors++
	}
	a.statuses[sample.StatusCode]++
	if sample.ErrorCategory != "" {
		a.errorByType[sample.ErrorCategory]++
	}
	a.latencies = append(a.latencies, sample.LatencyMS)
	a.sum += sample.LatencyMS
	if sample.LatencyMS < a.min {
		a.min = sample.LatencyMS
	}
	if sample.LatencyMS > a.max {
		a.max = sample.LatencyMS
	}
}

func (a *Accumulator) Summary(state model.RunState, completedAt time.Time) model.RunSummary {
	lat := make([]int64, len(a.latencies))
	copy(lat, a.latencies)
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })

	avg := 0.0
	if a.total > 0 {
		avg = float64(a.sum) / float64(a.total)
	}
	dur := completedAt.Sub(a.startedAt).Seconds()
	if dur <= 0 {
		dur = 1
	}
	comp := completedAt
	out := model.RunSummary{
		RunID:          a.runID,
		State:          state,
		StartedAt:      a.startedAt,
		CompletedAt:    &comp,
		TotalRequests:  a.total,
		SuccessCount:   a.successes,
		ErrorCount:     a.errors,
		AvgLatencyMS:   avg,
		MinLatencyMS:   valueOrZero(a.min),
		MaxLatencyMS:   a.max,
		P50LatencyMS:   percentile(lat, 50),
		P95LatencyMS:   percentile(lat, 95),
		P99LatencyMS:   percentile(lat, 99),
		RPS:            float64(a.total) / dur,
		StatusCounts:   a.statuses,
		ErrorBreakdown: a.errorByType,
		Config:         a.config,
	}
	return out
}

func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil((float64(p)/100.0)*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func valueOrZero(v int64) int64 {
	if v == math.MaxInt64 {
		return 0
	}
	return v
}
