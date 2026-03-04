package target

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/nanoncore/pon-exporter/internal/config"
)

func TestPoll_InvalidVendor(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := config.TargetConfig{
		Name:    "test-invalid",
		Vendor:  "nonexistent",
		Address: "127.0.0.1",
		Port:    22,
		Timeout: config.Duration(5 * time.Second),
	}

	snap := Poll(context.Background(), cfg, logger)
	if snap.Up {
		t.Error("expected Up=false for invalid vendor")
	}
	if snap.Target != "test-invalid" {
		t.Errorf("Target = %q, want %q", snap.Target, "test-invalid")
	}
}

func TestPoll_MockDriver(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	cfg := config.TargetConfig{
		Name:     "test-mock",
		Vendor:   "mock",
		Protocol: "cli",
		Address:  "127.0.0.1",
		Port:     22,
		Username: "admin",
		Password: "admin",
		Timeout:  config.Duration(10 * time.Second),
		Labels:   map[string]string{"site": "test"},
	}

	snap := Poll(context.Background(), cfg, logger)
	if !snap.Up {
		t.Error("expected Up=true for mock driver")
	}
	// Mock driver implements base Driver but not DriverV2, which is expected.
	// The exporter handles this gracefully by reporting driver_v2_supported=0.
	if snap.Labels["site"] != "test" {
		t.Errorf("Labels[site] = %q, want %q", snap.Labels["site"], "test")
	}
}
