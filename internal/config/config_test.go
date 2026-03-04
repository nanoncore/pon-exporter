package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad(t *testing.T) {
	yaml := `
poll_interval: 5m
targets:
  - name: olt-rack1
    vendor: vsol
    protocol: cli
    address: 192.168.1.1
    port: 22
    username: admin
    password: admin
    timeout: 30s
    labels:
      site: main-pop
      rack: "1"
  - name: olt-rack2
    vendor: huawei
    protocol: snmp
    address: 192.168.1.2
    port: 161
    snmp_community: public
    snmp_version: "2c"
    timeout: 60s
    labels:
      site: main-pop
      rack: "2"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if time.Duration(cfg.PollInterval) != 5*time.Minute {
		t.Errorf("PollInterval = %v, want 5m", time.Duration(cfg.PollInterval))
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("Targets len = %d, want 2", len(cfg.Targets))
	}

	tgt := cfg.Targets[0]
	if tgt.Name != "olt-rack1" {
		t.Errorf("Targets[0].Name = %q, want %q", tgt.Name, "olt-rack1")
	}
	if tgt.Vendor != "vsol" {
		t.Errorf("Targets[0].Vendor = %q, want %q", tgt.Vendor, "vsol")
	}
	if tgt.Labels["site"] != "main-pop" {
		t.Errorf("Targets[0].Labels[site] = %q, want %q", tgt.Labels["site"], "main-pop")
	}
	if time.Duration(tgt.Timeout) != 30*time.Second {
		t.Errorf("Targets[0].Timeout = %v, want 30s", time.Duration(tgt.Timeout))
	}
}

func TestValidate_Errors(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{
			name: "no targets",
			cfg:  Config{PollInterval: Duration(time.Minute)},
		},
		{
			name: "poll interval too low",
			cfg: Config{
				PollInterval: Duration(time.Second),
				Targets:      []TargetConfig{{Name: "a", Vendor: "vsol", Address: "1.2.3.4", Port: 22}},
			},
		},
		{
			name: "missing name",
			cfg: Config{
				PollInterval: Duration(time.Minute),
				Targets:      []TargetConfig{{Vendor: "vsol", Address: "1.2.3.4", Port: 22}},
			},
		},
		{
			name: "duplicate name",
			cfg: Config{
				PollInterval: Duration(time.Minute),
				Targets: []TargetConfig{
					{Name: "a", Vendor: "vsol", Address: "1.2.3.4", Port: 22},
					{Name: "a", Vendor: "vsol", Address: "1.2.3.5", Port: 22},
				},
			},
		},
		{
			name: "missing vendor",
			cfg: Config{
				PollInterval: Duration(time.Minute),
				Targets:      []TargetConfig{{Name: "a", Address: "1.2.3.4", Port: 22}},
			},
		},
		{
			name: "missing address",
			cfg: Config{
				PollInterval: Duration(time.Minute),
				Targets:      []TargetConfig{{Name: "a", Vendor: "vsol", Port: 22}},
			},
		},
		{
			name: "invalid port",
			cfg: Config{
				PollInterval: Duration(time.Minute),
				Targets:      []TargetConfig{{Name: "a", Vendor: "vsol", Address: "1.2.3.4", Port: 0}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); err == nil {
				t.Error("Validate() expected error, got nil")
			}
		})
	}
}
