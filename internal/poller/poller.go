package poller

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/nanoncore/pon-exporter/internal/config"
)

// PollFunc is the function signature for polling a single target.
type PollFunc func(ctx context.Context, cfg config.TargetConfig, logger *slog.Logger) *TargetSnapshot

// Manager runs background polling goroutines for each target.
type Manager struct {
	store    *SnapshotStore
	pollFn   PollFunc
	logger   *slog.Logger

	mu       sync.Mutex
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  bool
	errors   map[string]uint64 // cumulative error counters per target
}

// NewManager creates a new polling manager.
func NewManager(store *SnapshotStore, pollFn PollFunc, logger *slog.Logger) *Manager {
	return &Manager{
		store:  store,
		pollFn: pollFn,
		logger: logger,
		errors: make(map[string]uint64),
	}
}

// Start begins polling all targets with staggered goroutines.
func (m *Manager) Start(ctx context.Context, cfg *config.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}

	pollCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.running = true

	interval := time.Duration(cfg.PollInterval)
	targets := cfg.Targets

	// Stagger start times to avoid thundering herd
	stagger := interval / time.Duration(max(len(targets), 1))
	if stagger > 30*time.Second {
		stagger = 30 * time.Second
	}
	if stagger < time.Second {
		stagger = time.Second
	}

	m.logger.Info("starting poller", "targets", len(targets), "interval", interval, "stagger", stagger)

	for i, tgt := range targets {
		m.wg.Add(1)
		delay := stagger * time.Duration(i)
		go m.pollLoop(pollCtx, tgt, interval, delay)
	}
}

// Stop halts all polling goroutines and waits for them to finish.
func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	m.running = false
	m.cancel()
	m.mu.Unlock()

	m.wg.Wait()
	m.logger.Info("poller stopped")
}

// Reload stops current polling and starts with a new config.
func (m *Manager) Reload(ctx context.Context, cfg *config.Config) {
	m.logger.Info("reloading poller configuration")
	m.Stop()

	// Remove snapshots for targets no longer in config
	newTargets := make(map[string]bool)
	for _, t := range cfg.Targets {
		newTargets[t.Name] = true
	}
	for _, snap := range m.store.GetAll() {
		if !newTargets[snap.Target] {
			m.store.Remove(snap.Target)
		}
	}

	m.Start(ctx, cfg)
}

func (m *Manager) pollLoop(ctx context.Context, cfg config.TargetConfig, interval, initialDelay time.Duration) {
	defer m.wg.Done()

	// Initial stagger delay
	if initialDelay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(initialDelay):
		}
	}

	// First poll immediately
	m.doPoll(ctx, cfg)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.doPoll(ctx, cfg)
		}
	}
}

func (m *Manager) doPoll(ctx context.Context, cfg config.TargetConfig) {
	snap := m.pollFn(ctx, cfg, m.logger)

	// Accumulate error count
	m.mu.Lock()
	m.errors[cfg.Name] += snap.ErrorCount
	snap.ErrorCount = m.errors[cfg.Name]
	m.mu.Unlock()

	m.store.Set(cfg.Name, snap)
	m.logger.Debug("poll complete",
		"target", cfg.Name,
		"up", snap.Up,
		"driver_v2", snap.DriverV2,
		"onus", len(snap.ONUs),
		"duration", snap.Duration,
	)
}
