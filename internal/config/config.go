package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration.
type Config struct {
	PollInterval Duration         `yaml:"poll_interval"`
	Targets      []TargetConfig   `yaml:"targets"`
}

// TargetConfig defines a single OLT target.
type TargetConfig struct {
	Name          string            `yaml:"name"`
	Vendor        string            `yaml:"vendor"`
	Protocol      string            `yaml:"protocol"`
	Address       string            `yaml:"address"`
	Port          int               `yaml:"port"`
	Username      string            `yaml:"username,omitempty"`
	Password      string            `yaml:"password,omitempty"`
	SNMPCommunity string            `yaml:"snmp_community,omitempty"`
	SNMPVersion   string            `yaml:"snmp_version,omitempty"`
	Timeout       Duration          `yaml:"timeout,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
}

// Duration wraps time.Duration for YAML unmarshalling.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(dur)
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}

// Load reads and parses a config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	cfg := &Config{
		PollInterval: Duration(5 * time.Minute),
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks configuration for errors.
func (c *Config) Validate() error {
	if time.Duration(c.PollInterval) < 10*time.Second {
		return fmt.Errorf("poll_interval must be >= 10s")
	}
	if len(c.Targets) == 0 {
		return fmt.Errorf("at least one target is required")
	}
	seen := make(map[string]bool)
	for i, t := range c.Targets {
		if t.Name == "" {
			return fmt.Errorf("target[%d]: name is required", i)
		}
		if seen[t.Name] {
			return fmt.Errorf("target[%d]: duplicate name %q", i, t.Name)
		}
		seen[t.Name] = true
		if t.Vendor == "" {
			return fmt.Errorf("target %q: vendor is required", t.Name)
		}
		if t.Address == "" {
			return fmt.Errorf("target %q: address is required", t.Name)
		}
		if t.Port <= 0 || t.Port > 65535 {
			return fmt.Errorf("target %q: port must be 1-65535", t.Name)
		}
	}
	return nil
}
