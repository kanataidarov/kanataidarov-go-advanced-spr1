package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

const (
	defaultServerAddress  = "localhost:8080"
	defaultPollInterval   = 2 * time.Second
	defaultReportInterval = 10 * time.Second
)

type ServerConfig struct {
	Address string
}

func NewServerConfig() (ServerConfig, error) {
	return ParseServerConfig(os.Args[0], os.Args[1:])
}

func ParseServerConfig(name string, args []string) (ServerConfig, error) {
	cfg := ServerConfig{Address: defaultServerAddress}

	fs := newFlagSet(name)
	fs.StringVar(&cfg.Address, "a", cfg.Address, "address of the HTTP server endpoint")

	if err := parse(fs, args); err != nil {
		return ServerConfig{}, err
	}

	return cfg, nil
}

type CollectorConfig struct {
	PollInterval time.Duration
}

type SenderConfig struct {
	Address        string
	ReportInterval time.Duration
}

type AgentConfig struct {
	Collector CollectorConfig
	Sender    SenderConfig
}

func NewAgentConfig() (AgentConfig, error) {
	return ParseAgentConfig(os.Args[0], os.Args[1:])
}

func ParseAgentConfig(name string, args []string) (AgentConfig, error) {
	cfg := AgentConfig{
		Collector: CollectorConfig{PollInterval: defaultPollInterval},
		Sender: SenderConfig{
			Address:        defaultServerAddress,
			ReportInterval: defaultReportInterval,
		},
	}

	fs := newFlagSet(name)
	fs.StringVar(&cfg.Sender.Address, "a", cfg.Sender.Address, "address of the HTTP server endpoint")
	reportInterval := fs.Int("r", seconds(cfg.Sender.ReportInterval), "metrics reporting interval, in seconds")
	pollInterval := fs.Int("p", seconds(cfg.Collector.PollInterval), "runtime metrics polling interval, in seconds")

	if err := parse(fs, args); err != nil {
		return AgentConfig{}, err
	}

	if *reportInterval <= 0 {
		return AgentConfig{}, fmt.Errorf("invalid flag -r=%d: report interval must be positive", *reportInterval)
	}

	if *pollInterval <= 0 {
		return AgentConfig{}, fmt.Errorf("invalid flag -p=%d: poll interval must be positive", *pollInterval)
	}

	cfg.Sender.ReportInterval = time.Duration(*reportInterval) * time.Second
	cfg.Collector.PollInterval = time.Duration(*pollInterval) * time.Second

	return cfg, nil
}

func newFlagSet(name string) *flag.FlagSet {
	if name == "" {
		name = "app"
	}

	return flag.NewFlagSet(name, flag.ContinueOnError)
}

func parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return err
	}

	if rest := fs.Args(); len(rest) > 0 {
		return fmt.Errorf("unknown argument %q", rest[0])
	}

	return nil
}

func seconds(d time.Duration) int {
	return int(d / time.Second)
}
