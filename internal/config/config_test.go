package config

import (
	"testing"
	"time"
)

func TestNewServerConfigDefaults(t *testing.T) {
	cfg, err := ParseServerConfig("server", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Address != defaultServerAddress {
		t.Errorf("got %q, want %q", cfg.Address, defaultServerAddress)
	}
}

func TestServerConfigFlag(t *testing.T) {
	cfg, err := ParseServerConfig("server", []string{"-a=localhost:8888"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Address != "localhost:8888" {
		t.Errorf("got %q, want localhost:8888", cfg.Address)
	}
}

func TestServerConfigRejectsUnknownFlag(t *testing.T) {
	if _, err := ParseServerConfig("server", []string{"-x=1"}, nil); err == nil {
		t.Error("unknown flag must produce an error")
	}

	if _, err := ParseServerConfig("server", []string{"trailing"}, nil); err == nil {
		t.Error("unknown argument must produce an error")
	}
}

func TestNewAgentConfigDefaults(t *testing.T) {
	cfg, err := ParseAgentConfig("agent", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Sender.Address != defaultServerAddress {
		t.Errorf("got address %q, want %q", cfg.Sender.Address, defaultServerAddress)
	}

	if cfg.Collector.PollInterval != 2*time.Second {
		t.Errorf("got poll interval %s, want 2s", cfg.Collector.PollInterval)
	}

	if cfg.Sender.ReportInterval != 10*time.Second {
		t.Errorf("got report interval %s, want 10s", cfg.Sender.ReportInterval)
	}
}

func TestAgentConfigFlags(t *testing.T) {
	cfg, err := ParseAgentConfig("agent", []string{"-a", "127.0.0.1:8081", "-r", "5", "-p", "1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Sender.Address != "127.0.0.1:8081" {
		t.Errorf("got address %q, want 127.0.0.1:8081", cfg.Sender.Address)
	}

	if cfg.Sender.ReportInterval != 5*time.Second {
		t.Errorf("got report interval %s, want 5s", cfg.Sender.ReportInterval)
	}

	if cfg.Collector.PollInterval != time.Second {
		t.Errorf("got poll interval %s, want 1s", cfg.Collector.PollInterval)
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
		if _, err := ParseAgentConfig("agent", args, nil); err == nil {
			t.Errorf("args %v must produce an error", args)
		}
	}
}

func envMap(vars map[string]string) LookupEnvFunc {
	return func(key string) (string, bool) {
		v, ok := vars[key]
		return v, ok
	}
}

func TestServerConfigEnvPriority(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{name: "default", want: defaultServerAddress},
		{name: "flag only", args: []string{"-a=localhost:8888"}, want: "localhost:8888"},
		{name: "env only", env: map[string]string{"ADDRESS": "localhost:9999"}, want: "localhost:9999"},
		{
			name: "env beats flag",
			args: []string{"-a=localhost:8888"},
			env:  map[string]string{"ADDRESS": "localhost:9999"},
			want: "localhost:9999",
		},
		{
			name: "empty env is ignored",
			args: []string{"-a=localhost:8888"},
			env:  map[string]string{"ADDRESS": ""},
			want: "localhost:8888",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseServerConfig("server", tt.args, envMap(tt.env))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.Address != tt.want {
				t.Errorf("got %q, want %q", cfg.Address, tt.want)
			}
		})
	}
}

func TestAgentConfigEnvPriority(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		env          map[string]string
		wantAddress  string
		wantReport   time.Duration
		wantPollTime time.Duration
	}{
		{
			name:         "defaults",
			wantAddress:  defaultServerAddress,
			wantReport:   defaultReportInterval,
			wantPollTime: defaultPollInterval,
		},
		{
			name:         "env only",
			env:          map[string]string{"ADDRESS": "localhost:9999", "REPORT_INTERVAL": "7", "POLL_INTERVAL": "3"},
			wantAddress:  "localhost:9999",
			wantReport:   7 * time.Second,
			wantPollTime: 3 * time.Second,
		},
		{
			name:         "env beats flags",
			args:         []string{"-a=localhost:8888", "-r=5", "-p=1"},
			env:          map[string]string{"ADDRESS": "localhost:9999", "REPORT_INTERVAL": "7", "POLL_INTERVAL": "3"},
			wantAddress:  "localhost:9999",
			wantReport:   7 * time.Second,
			wantPollTime: 3 * time.Second,
		},
		{
			name:         "env and flags mixed",
			args:         []string{"-a=localhost:8888", "-r=5"},
			env:          map[string]string{"POLL_INTERVAL": "3"},
			wantAddress:  "localhost:8888",
			wantReport:   5 * time.Second,
			wantPollTime: 3 * time.Second,
		},
		{
			name:         "empty env is ignored",
			args:         []string{"-r=5"},
			env:          map[string]string{"ADDRESS": "", "REPORT_INTERVAL": ""},
			wantAddress:  defaultServerAddress,
			wantReport:   5 * time.Second,
			wantPollTime: defaultPollInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseAgentConfig("agent", tt.args, envMap(tt.env))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.Sender.Address != tt.wantAddress {
				t.Errorf("got address %q, want %q", cfg.Sender.Address, tt.wantAddress)
			}

			if cfg.Sender.ReportInterval != tt.wantReport {
				t.Errorf("got report interval %s, want %s", cfg.Sender.ReportInterval, tt.wantReport)
			}

			if cfg.Collector.PollInterval != tt.wantPollTime {
				t.Errorf("got poll interval %s, want %s", cfg.Collector.PollInterval, tt.wantPollTime)
			}
		})
	}
}

func TestAgentConfigRejectsBadEnv(t *testing.T) {
	cases := []map[string]string{
		{"REPORT_INTERVAL": "abc"},
		{"POLL_INTERVAL": "1.5"},
		{"REPORT_INTERVAL": "0"},
		{"POLL_INTERVAL": "-2"},
	}

	for _, env := range cases {
		// валидный флаг не должен спасать невалидную переменную окружения
		if _, err := ParseAgentConfig("agent", []string{"-r=5", "-p=1"}, envMap(env)); err == nil {
			t.Errorf("env %v must produce an error", env)
		}
	}
}

func TestServerConfigStorageDefaults(t *testing.T) {
	cfg, err := ParseServerConfig("server", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := ServerConfig{
		Address:         defaultServerAddress,
		StoreInterval:   300 * time.Second,
		FileStoragePath: defaultFileStoragePath,
		Restore:         true,
	}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestServerConfigStoragePriority(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want ServerConfig
	}{
		{
			name: "flags",
			args: []string{"-i=0", "-f=/tmp/flag.json", "-r=false"},
			want: ServerConfig{Address: defaultServerAddress, StoreInterval: 0, FileStoragePath: "/tmp/flag.json", Restore: false},
		},
		{
			name: "env only",
			env:  map[string]string{"STORE_INTERVAL": "5", "FILE_STORAGE_PATH": "/tmp/env.json", "RESTORE": "false"},
			want: ServerConfig{Address: defaultServerAddress, StoreInterval: 5 * time.Second, FileStoragePath: "/tmp/env.json", Restore: false},
		},
		{
			name: "env beats flags",
			args: []string{"-i=10", "-f=/tmp/flag.json", "-r=false"},
			env:  map[string]string{"STORE_INTERVAL": "0", "FILE_STORAGE_PATH": "/tmp/env.json", "RESTORE": "true"},
			want: ServerConfig{Address: defaultServerAddress, StoreInterval: 0, FileStoragePath: "/tmp/env.json", Restore: true},
		},
		{
			name: "env and flags mixed",
			args: []string{"-i=10", "-r=false"},
			env:  map[string]string{"FILE_STORAGE_PATH": "/tmp/env.json"},
			want: ServerConfig{Address: defaultServerAddress, StoreInterval: 10 * time.Second, FileStoragePath: "/tmp/env.json", Restore: false},
		},
		{
			name: "empty flag disables file storage",
			args: []string{"-f="},
			want: ServerConfig{Address: defaultServerAddress, StoreInterval: 300 * time.Second, FileStoragePath: "", Restore: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseServerConfig("server", tt.args, envMap(tt.env))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg != tt.want {
				t.Errorf("got %+v, want %+v", cfg, tt.want)
			}
		})
	}
}

func TestServerConfigRejectsBadStorageSettings(t *testing.T) {
	tests := []struct {
		name string
		args []string
		env  map[string]string
	}{
		{name: "negative flag interval", args: []string{"-i=-1"}},
		{name: "non-numeric flag interval", args: []string{"-i=abc"}},
		{name: "bad flag restore", args: []string{"-r=maybe"}},
		{name: "negative env interval", env: map[string]string{"STORE_INTERVAL": "-5"}},
		{name: "non-numeric env interval", env: map[string]string{"STORE_INTERVAL": "1.5"}},
		{name: "bad env restore", env: map[string]string{"RESTORE": "maybe"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseServerConfig("server", tt.args, envMap(tt.env)); err == nil {
				t.Error("expected an error")
			}
		})
	}
}
