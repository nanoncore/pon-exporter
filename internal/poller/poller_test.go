package poller

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanoncore/pon-exporter/internal/config"
)

func TestManager_StartStop(t *testing.T) {
	store := NewSnapshotStore()
	var pollCount atomic.Int64

	fakePoll := func(ctx context.Context, cfg config.TargetConfig, logger *slog.Logger) *TargetSnapshot {
		pollCount.Add(1)
		return &TargetSnapshot{
			Target:   cfg.Name,
			Up:       true,
			DriverV2: true,
			PollTime: time.Now(),
			Duration: time.Millisecond,
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := NewManager(store, fakePoll, logger)

	cfg := &config.Config{
		PollInterval: config.Duration(100 * time.Millisecond),
		Targets: []config.TargetConfig{
			{Name: "olt-1", Vendor: "mock", Address: "127.0.0.1", Port: 22},
			{Name: "olt-2", Vendor: "mock", Address: "127.0.0.2", Port: 22},
		},
	}

	ctx := context.Background()
	mgr.Start(ctx, cfg)

	// Wait for at least one poll per target
	time.Sleep(300 * time.Millisecond)

	mgr.Stop()

	if pollCount.Load() < 2 {
		t.Errorf("expected at least 2 polls, got %d", pollCount.Load())
	}

	if !store.HasData() {
		t.Error("expected store to have data after polling")
	}
}

func TestManager_Reload(t *testing.T) {
	store := NewSnapshotStore()

	fakePoll := func(ctx context.Context, cfg config.TargetConfig, logger *slog.Logger) *TargetSnapshot {
		return &TargetSnapshot{
			Target:   cfg.Name,
			Up:       true,
			DriverV2: true,
			PollTime: time.Now(),
			Duration: time.Millisecond,
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := NewManager(store, fakePoll, logger)

	cfg1 := &config.Config{
		PollInterval: config.Duration(100 * time.Millisecond),
		Targets: []config.TargetConfig{
			{Name: "olt-a", Vendor: "mock", Address: "127.0.0.1", Port: 22},
			{Name: "olt-b", Vendor: "mock", Address: "127.0.0.2", Port: 22},
		},
	}

	ctx := context.Background()
	mgr.Start(ctx, cfg1)
	time.Sleep(200 * time.Millisecond)

	// Reload with only olt-a — olt-b should be removed
	cfg2 := &config.Config{
		PollInterval: config.Duration(100 * time.Millisecond),
		Targets: []config.TargetConfig{
			{Name: "olt-a", Vendor: "mock", Address: "127.0.0.1", Port: 22},
		},
	}

	mgr.Reload(ctx, cfg2)
	time.Sleep(200 * time.Millisecond)
	mgr.Stop()

	// olt-b should have been removed from the store
	all := store.GetAll()
	for _, s := range all {
		if s.Target == "olt-b" {
			t.Error("olt-b should have been removed after reload")
		}
	}
}

func TestManager_ErrorAccumulation(t *testing.T) {
	store := NewSnapshotStore()

	fakePoll := func(ctx context.Context, cfg config.TargetConfig, logger *slog.Logger) *TargetSnapshot {
		return &TargetSnapshot{
			Target:     cfg.Name,
			Up:         true,
			DriverV2:   true,
			ErrorCount: 1, // each poll has 1 error
			PollTime:   time.Now(),
			Duration:   time.Millisecond,
		}
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	mgr := NewManager(store, fakePoll, logger)

	cfg := &config.Config{
		PollInterval: config.Duration(50 * time.Millisecond),
		Targets: []config.TargetConfig{
			{Name: "olt-err", Vendor: "mock", Address: "127.0.0.1", Port: 22},
		},
	}

	ctx := context.Background()
	mgr.Start(ctx, cfg)
	time.Sleep(200 * time.Millisecond)
	mgr.Stop()

	// Error count should be cumulative (> 1)
	snaps := store.GetAll()
	for _, s := range snaps {
		if s.Target == "olt-err" && s.ErrorCount <= 1 {
			t.Errorf("expected cumulative error count > 1, got %d", s.ErrorCount)
		}
	}
}
