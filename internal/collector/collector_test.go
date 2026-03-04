package collector

import (
	"testing"
	"time"

	"github.com/nanoncore/pon-exporter/internal/poller"
	"github.com/prometheus/client_golang/prometheus"
)

func gatherNames(t *testing.T, c prometheus.Collector) map[string]bool {
	t.Helper()
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather error: %v", err)
	}
	names := make(map[string]bool, len(families))
	for _, f := range families {
		names[f.GetName()] = true
	}
	return names
}

func gatherCount(t *testing.T, c prometheus.Collector) int {
	t.Helper()
	reg := prometheus.NewRegistry()
	reg.MustRegister(c)
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather error: %v", err)
	}
	count := 0
	for _, f := range families {
		count += len(f.GetMetric())
	}
	return count
}

func TestCollector_EmptyStore(t *testing.T) {
	store := poller.NewSnapshotStore()
	c := New(store)

	count := gatherCount(t, c)
	if count != 0 {
		t.Errorf("expected 0 metrics from empty store, got %d", count)
	}
}

func TestCollector_TargetDown(t *testing.T) {
	store := poller.NewSnapshotStore()
	store.Set("test-olt", &poller.TargetSnapshot{
		Target:     "test-olt",
		Up:         false,
		DriverV2:   false,
		Duration:   2 * time.Second,
		ErrorCount: 3,
		Labels:     map[string]string{"site": "main-pop"},
		PollTime:   time.Now(),
	})

	c := New(store)
	names := gatherNames(t, c)

	// Down target should emit: up, driver_v2_supported, scrape_duration, scrape_errors, target_info
	want := []string{
		"gpon_up", "gpon_driver_v2_supported",
		"gpon_scrape_duration_seconds", "gpon_scrape_errors_total",
		"gpon_target_info",
	}
	for _, n := range want {
		if !names[n] {
			t.Errorf("missing metric %q for down target", n)
		}
	}
	if len(names) != len(want) {
		t.Errorf("expected %d metric families, got %d: %v", len(want), len(names), names)
	}
}

func TestCollector_FullSnapshot(t *testing.T) {
	store := poller.NewSnapshotStore()
	store.Set("olt-1", &poller.TargetSnapshot{
		Target:   "olt-1",
		Up:       true,
		DriverV2: true,
		Duration: time.Second,
		Labels:   map[string]string{"rack": "1", "site": "dc1"},
		OLT: &poller.OLTStatusSnapshot{
			Vendor:        "vsol",
			Model:         "V1600G",
			Firmware:      "v2.1",
			SerialNumber:  "SN123",
			CPUPercent:    45.2,
			MemoryPercent: 62.1,
			Temperature:   55.0,
			ActiveONUs:    100,
			TotalONUs:     128,
			UptimeSeconds: 86400,
		},
		PONPorts: []poller.PONPortSnapshot{
			{Port: "0/1", ONUCount: 32, RxPowerDBm: -18.5, TxPowerDBm: 3.1},
		},
		ONUs: []poller.ONUSnapshot{
			{
				PONPort:      "0/1",
				ONUID:        1,
				Serial:       "VSOL00000001",
				Vendor:       "vsol",
				Model:        "HG325",
				AdminState:   "enabled",
				OperState:    "online",
				IsOnline:     true,
				RxPowerDBm:   -18.3,
				TxPowerDBm:   2.5,
				OLTRxDBm:     -19.1,
				DistanceM:    1200,
				Temperature:  42.5,
				IsWithinSpec: true,
				BytesUp:      1000000,
				BytesDown:    5000000,
			},
		},
		Alarms: []poller.AlarmSnapshot{
			{Severity: "critical"},
			{Severity: "minor"},
			{Severity: "minor"},
		},
		ErrorCount: 0,
		PollTime:   time.Now(),
	})

	c := New(store)
	names := gatherNames(t, c)

	expectedNames := []string{
		"gpon_up",
		"gpon_driver_v2_supported",
		"gpon_scrape_duration_seconds",
		"gpon_scrape_errors_total",
		"gpon_target_info",
		"gpon_olt_info",
		"gpon_olt_cpu_percent",
		"gpon_olt_memory_percent",
		"gpon_olt_temperature_celsius",
		"gpon_olt_active_onus",
		"gpon_olt_total_onus",
		"gpon_olt_uptime_seconds",
		"gpon_olt_active_alarms",
		"gpon_olt_alarms_by_severity",
		"gpon_pon_port_onu_count",
		"gpon_pon_port_rx_power_dbm",
		"gpon_pon_port_tx_power_dbm",
		"gpon_onu_info",
		"gpon_onu_online",
		"gpon_onu_rx_power_dbm",
		"gpon_onu_tx_power_dbm",
		"gpon_onu_olt_rx_power_dbm",
		"gpon_onu_distance_meters",
		"gpon_onu_temperature_celsius",
		"gpon_onu_signal_within_spec",
		"gpon_onu_bytes_up_total",
		"gpon_onu_bytes_down_total",
	}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("missing metric family %q", name)
		}
	}
}

func TestCollector_NoONUs(t *testing.T) {
	store := poller.NewSnapshotStore()
	store.Set("olt-empty", &poller.TargetSnapshot{
		Target:   "olt-empty",
		Up:       true,
		DriverV2: true,
		Duration: time.Second,
		OLT: &poller.OLTStatusSnapshot{
			Vendor:   "huawei",
			Model:    "MA5800",
			Firmware: "V800R021",
		},
		PollTime: time.Now(),
	})

	c := New(store)
	count := gatherCount(t, c)
	if count == 0 {
		t.Error("expected metrics even with no ONUs")
	}
}
