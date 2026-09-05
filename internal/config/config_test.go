package config

import (
	"testing"
	"time"
)

func TestNewServerConfigDefaults(t *testing.T) {
	if got := NewServerConfig(); got.Address != defaultServerAddress {
		t.Errorf("got %q, want %q", got.Address, defaultServerAddress)
	}
}

func TestNewServerConfigFromEnv(t *testing.T) {
	t.Setenv("ADDRESS", "0.0.0.0:9090")

	if got := NewServerConfig(); got.Address != "0.0.0.0:9090" {
		t.Errorf("got %q, want 0.0.0.0:9090", got.Address)
	}
}

func TestNewAgentConfigDefaults(t *testing.T) {
	cfg := NewAgentConfig()

	if cfg.Address != defaultServerAddress {
		t.Errorf("got address %q, want %q", cfg.Address, defaultServerAddress)
	}

	if cfg.PollInterval != 2*time.Second {
		t.Errorf("got poll interval %s, want 2s", cfg.PollInterval)
	}

	if cfg.ReportInterval != 10*time.Second {
		t.Errorf("got report interval %s, want 10s", cfg.ReportInterval)
	}
}

func TestNewAgentConfigFromEnv(t *testing.T) {
	t.Setenv("ADDRESS", "localhost:9999")
	t.Setenv("POLL_INTERVAL", "1")
	t.Setenv("REPORT_INTERVAL", "3")

	cfg := NewAgentConfig()

	if cfg.Address != "localhost:9999" {
		t.Errorf("got address %q, want localhost:9999", cfg.Address)
	}

	if cfg.PollInterval != time.Second {
		t.Errorf("got poll interval %s, want 1s", cfg.PollInterval)
	}

	if cfg.ReportInterval != 3*time.Second {
		t.Errorf("got report interval %s, want 3s", cfg.ReportInterval)
	}
}

func TestNewAgentConfigIgnoresInvalidIntervals(t *testing.T) {
	t.Setenv("POLL_INTERVAL", "abc")
	t.Setenv("REPORT_INTERVAL", "-5")

	cfg := NewAgentConfig()

	if cfg.PollInterval != defaultPollInterval || cfg.ReportInterval != defaultReportInterval {
		t.Errorf("invalid env values must be ignored: %+v", cfg)
	}
}
