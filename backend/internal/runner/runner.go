package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"loadtester/backend/internal/config"
	"loadtester/backend/internal/model"
	"loadtester/backend/internal/proxy"
	"loadtester/backend/internal/reporting"
	"loadtester/backend/internal/storage"
	"loadtester/backend/internal/ua"
)

type RunUpdate struct {
	Type    string            `json:"type"`
	RunID   string            `json:"runId"`
	Sample  *model.RunSample  `json:"sample,omitempty"`
	Summary *model.RunSummary `json:"summary,omitempty"`
	Message string            `json:"message,omitempty"`
}

type Service struct {
	store    *storage.Store
	proxies  *proxy.Manager
	ua       *ua.Provider
	mu       sync.RWMutex
	liveChan map[string]chan RunUpdate
}

func NewService(store *storage.Store, proxies *proxy.Manager) *Service {
	return &Service{
		store:    store,
		proxies:  proxies,
		ua:       ua.NewProvider(),
		liveChan: map[string]chan RunUpdate{},
	}
}

func (s *Service) Subscribe(runID string) <-chan RunUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.liveChan[runID]
	if ok {
		return ch
	}
	ch = make(chan RunUpdate, 256)
	s.liveChan[runID] = ch
	return ch
}

func (s *Service) publish(runID string, update RunUpdate) {
	s.mu.RLock()
	ch, ok := s.liveChan[runID]
	s.mu.RUnlock()
	if !ok {
		return
	}
	select {
	case ch <- update:
	default:
	}
}

func (s *Service) closeChannel(runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.liveChan[runID]; ok {
		close(ch)
		delete(s.liveChan, runID)
	}
}

func (s *Service) Start(runID string, cfg model.LoadTestConfig) {
	config.NormalizeConfig(&cfg)
	go s.execute(runID, cfg)
}

func (s *Service) execute(runID string, cfg model.LoadTestConfig) {
	ctx := context.Background()
	start := time.Now()
	if err := s.store.CreateRun(ctx, runID, cfg); err != nil {
		s.publish(runID, RunUpdate{Type: "error", RunID: runID, Message: err.Error()})
		return
	}
	acc := reporting.NewAccumulator(runID, start, cfg)
	successStatuses := toStatusMap(cfg.SuccessStatuses)

	var sent atomic.Int64
	var shouldStop atomic.Bool
	deadline := time.Time{}
	if cfg.DurationSeconds > 0 {
		deadline = start.Add(time.Duration(cfg.DurationSeconds) * time.Second)
	}

	reqCh := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range reqCh {
				if shouldStop.Load() {
					continue
				}
				now := time.Now()
				if !deadline.IsZero() && now.After(deadline) {
					shouldStop.Store(true)
					continue
				}
				if cfg.Requests > 0 && int(sent.Load()) >= cfg.Requests {
					shouldStop.Store(true)
					continue
				}
				sample := s.makeRequest(ctx, runID, cfg, successStatuses)
				if cfg.Requests > 0 {
					sent.Add(1)
				}
				acc.Add(sample)
				_ = s.store.SaveRunSample(ctx, sample)
				s.publish(runID, RunUpdate{Type: "sample", RunID: runID, Sample: &sample})
			}
		}()
	}

	interval := time.NewTicker(10 * time.Millisecond)
	defer interval.Stop()

loop:
	for {
		if shouldStop.Load() {
			break loop
		}
		if cfg.Requests > 0 && int(sent.Load()) >= cfg.Requests {
			break loop
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			break loop
		}
		<-interval.C
		for i := 0; i < cfg.Concurrency; i++ {
			reqCh <- struct{}{}
		}
	}
	close(reqCh)
	wg.Wait()

	summary := acc.Summary(model.RunStateCompleted, time.Now())
	_ = s.store.UpdateRunSummary(ctx, runID, summary)
	s.publish(runID, RunUpdate{Type: "summary", RunID: runID, Summary: &summary})
	s.closeChannel(runID)
}

func (s *Service) makeRequest(ctx context.Context, runID string, cfg model.LoadTestConfig, successStatuses map[int]struct{}) model.RunSample {
	sample := model.RunSample{
		RunID:     runID,
		Timestamp: time.Now().UTC(),
	}
	client, proxyAddr := s.clientForRequest(ctx, cfg)
	sample.ProxyAddress = proxyAddr

	req, bodySize, err := s.buildRequest(ctx, cfg)
	if err != nil {
		sample.ErrorCategory = "build_request"
		sample.ErrorMessage = err.Error()
		return sample
	}
	sample.BytesSent = bodySize

	start := time.Now()
	resp, err := client.Do(req)
	sample.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		sample.ErrorCategory = categorizeError(err)
		sample.ErrorMessage = err.Error()
		if proxyAddr != "" {
			_ = s.store.UpdateProxyHealth(ctx, proxyAddr, false, sample.LatencyMS, sample.ErrorMessage)
		}
		return sample
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	sample.BytesReceived = int64(len(body))
	sample.StatusCode = resp.StatusCode
	_, sample.Success = successStatuses[resp.StatusCode]

	if proxyAddr != "" {
		_ = s.store.UpdateProxyHealth(ctx, proxyAddr, true, sample.LatencyMS, "")
	}
	if !sample.Success {
		sample.ErrorCategory = "status_mismatch"
		sample.ErrorMessage = "status " + strconv.Itoa(resp.StatusCode) + " is not in success list"
	}
	return sample
}

func (s *Service) clientForRequest(ctx context.Context, cfg model.LoadTestConfig) (*http.Client, string) {
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: timeout}).DialContext,
	}
	proxyAddr := ""
	if cfg.Proxy.Enabled {
		p, err := s.proxies.NextProxy(ctx)
		if err == nil && p != nil {
			if parsed, parseErr := url.Parse(p.Address); parseErr == nil {
				proxyAddr = p.Address
				transport.Proxy = http.ProxyURL(parsed)
			}
		}
	}
	return &http.Client{Timeout: timeout, Transport: transport}, proxyAddr
}

func (s *Service) buildRequest(ctx context.Context, cfg model.LoadTestConfig) (*http.Request, int64, error) {
	method := strings.ToUpper(cfg.Method)
	var body io.Reader
	var bodySize int64
	headers := map[string]string{}
	for k, v := range cfg.Headers {
		headers[k] = v
	}

	switch cfg.Body.Type {
	case model.BodyTypeJSON:
		payload := cfg.Body.JSON
		bodySize = int64(len(payload))
		body = strings.NewReader(payload)
		headers["Content-Type"] = "application/json"
	case model.BodyTypeForm:
		form := url.Values{}
		for k, v := range cfg.Body.Form {
			form.Set(k, v)
		}
		encoded := form.Encode()
		bodySize = int64(len(encoded))
		body = strings.NewReader(encoded)
		headers["Content-Type"] = "application/x-www-form-urlencoded"
	case model.BodyTypeMultipart:
		buf := bytes.NewBuffer(nil)
		w := multipart.NewWriter(buf)
		for k, v := range cfg.Body.MultipartFields {
			_ = w.WriteField(k, v)
		}
		for _, f := range cfg.Body.MultipartFiles {
			name, path, err := s.store.UploadedFilePath(ctx, f.FileID)
			if err != nil {
				return nil, 0, err
			}
			file, err := os.Open(path)
			if err != nil {
				return nil, 0, err
			}
			part, err := w.CreateFormFile(f.FieldName, fileNameOrFallback(f.FileName, name))
			if err != nil {
				file.Close()
				return nil, 0, err
			}
			if _, err := io.Copy(part, file); err != nil {
				file.Close()
				return nil, 0, err
			}
			file.Close()
		}
		_ = w.Close()
		bodySize = int64(buf.Len())
		body = buf
		headers["Content-Type"] = w.FormDataContentType()
	default:
		body = http.NoBody
	}

	req, err := http.NewRequestWithContext(ctx, method, cfg.URL, body)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", s.ua.Next(cfg.UserAgent))
	return req, bodySize, nil
}

func fileNameOrFallback(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func toStatusMap(statuses []int) map[int]struct{} {
	m := make(map[int]struct{}, len(statuses))
	for _, s := range statuses {
		m[s] = struct{}{}
	}
	return m
}

func categorizeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "proxyconnect"):
		return "proxy_error"
	case strings.Contains(msg, "timeout"):
		return "timeout"
	case strings.Contains(msg, "connection refused"):
		return "connect_refused"
	default:
		return "transport_error"
	}
}

func NewRunID() string {
	return fmt.Sprintf("run_%d", time.Now().UnixNano())
}
