package config

import (
	"testing"
	"time"
)

func TestNewServerConfigDefaults(t *testing.T) {
	cfg, err := ParseServerConfig("server", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Address != defaultServerAddress {
		t.Errorf("got %q, want %q", cfg.Address, defaultServerAddress)
	}
}

func TestServerConfigFlag(t *testing.T) {
	cfg, err := ParseServerConfig("server", []string{"-a=localhost:8888"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Address != "localhost:8888" {
		t.Errorf("got %q, want localhost:8888", cfg.Address)
	}
}

func TestServerConfigRejectsUnknownFlag(t *testing.T) {
	if _, err := ParseServerConfig("server", []string{"-x=1"}); err == nil {
		t.Error("unknown flag must produce an error")
	}

	if _, err := ParseServerConfig("server", []string{"trailing"}); err == nil {
		t.Error("unknown argument must produce an error")
	}
}

func TestNewAgentConfigDefaults(t *testing.T) {
	cfg, err := ParseAgentConfig("agent", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

func TestAgentConfigFlags(t *testing.T) {
	cfg, err := ParseAgentConfig("agent", []string{"-a", "127.0.0.1:8081", "-r", "5", "-p", "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Address != "127.0.0.1:8081" {
		t.Errorf("got address %q, want 127.0.0.1:8081", cfg.Address)
	}

	if cfg.ReportInterval != 5*time.Second {
		t.Errorf("got report interval %s, want 5s", cfg.ReportInterval)
	}

	if cfg.PollInterval != time.Second {
		t.Errorf("got poll interval %s, want 1s", cfg.PollInterval)
	}
}

func TestAgentConfigRejectsBadFlags(t *testing.T) {
	cases := [][]string{
		{"-unknown"},
		{"-r=0"},
		{"-p=-1"},
		{"-r=abc"},
		{"stray"},
	}

	for _, args := range cases {
		if _, err := ParseAgentConfig("agent", args); err == nil {
			t.Errorf("args %v must produce an error", args)
		}
	}
}
