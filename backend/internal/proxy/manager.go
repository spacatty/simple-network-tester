package proxy

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"loadtester/backend/internal/model"
	"loadtester/backend/internal/storage"
)

type Manager struct {
	mu       sync.Mutex
	index    int
	store    *storage.Store
	checkURL string
}

func NewManager(store *storage.Store) *Manager {
	return &Manager{store: store, checkURL: "https://httpbin.org/ip"}
}

func ParseProxyList(raw string) []string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, "://") {
			line = "http://" + line
		}
		out = append(out, line)
	}
	return out
}

func (m *Manager) NextProxy(ctx context.Context) (*model.ProxyRecord, error) {
	active, err := m.store.ListActiveProxies(ctx)
	if err != nil {
		return nil, err
	}
	if len(active) == 0 {
		return nil, errors.New("no active proxies")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p := active[m.index%len(active)]
	m.index++
	return &p, nil
}

func (m *Manager) ValidateOne(ctx context.Context, address string, timeout time.Duration) model.ProxyRecord {
	result := model.ProxyRecord{Address: address, Active: false}
	start := time.Now()

	u, err := url.Parse(address)
	if err != nil {
		result.LastError = err.Error()
		return result
	}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(u),
			DialContext: (&net.Dialer{
				Timeout: timeout,
			}).DialContext,
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.checkURL, nil)
	if err != nil {
		result.LastError = err.Error()
		return result
	}
	resp, err := client.Do(req)
	if err != nil {
		result.LastError = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.LastLatency = time.Since(start).Milliseconds()
	result.Active = resp.StatusCode >= 200 && resp.StatusCode < 500
	return result
}

func (m *Manager) ValidateAndPersist(ctx context.Context, timeout time.Duration) ([]model.ProxyRecord, error) {
	all, err := m.store.ListProxies(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.ProxyRecord, 0, len(all))
	for _, p := range all {
		r := m.ValidateOne(ctx, p.Address, timeout)
		if err := m.store.UpdateProxyHealth(ctx, r.Address, r.Active, r.LastLatency, r.LastError); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}
