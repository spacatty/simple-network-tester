package ua

import (
	"math/rand"
	"sync"
	"time"

	"loadtester/backend/internal/config"
	"loadtester/backend/internal/model"
)

type Provider struct {
	mu   sync.Mutex
	rand *rand.Rand
}

func NewProvider() *Provider {
	return &Provider{rand: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (p *Provider) Next(cfg model.UserAgentConfig) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch cfg.Mode {
	case model.UserAgentModeFixed:
		return cfg.FixedValue
	case model.UserAgentModeCustomRotate:
		if len(cfg.CustomList) == 0 {
			return config.DefaultUserAgents[p.rand.Intn(len(config.DefaultUserAgents))]
		}
		return cfg.CustomList[p.rand.Intn(len(cfg.CustomList))]
	default:
		return config.DefaultUserAgents[p.rand.Intn(len(config.DefaultUserAgents))]
	}
}
