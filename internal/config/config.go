package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultServerAddress   = "localhost:8080"
	defaultPollInterval    = 2 * time.Second
	defaultReportInterval  = 10 * time.Second
	defaultStoreInterval   = 300 * time.Second
	defaultFileStoragePath = "data/metrics-db.json"
	defaultRestore         = true
)

const (
	envAddress         = "ADDRESS"
	envReportInterval  = "REPORT_INTERVAL"
	envPollInterval    = "POLL_INTERVAL"
	envStoreInterval   = "STORE_INTERVAL"
	envFileStoragePath = "FILE_STORAGE_PATH"
	envRestore         = "RESTORE"
	envDatabaseDSN     = "DATABASE_DSN"
)

type LookupEnvFunc func(key string) (string, bool)

type ServerConfig struct {
	Address         string
	StoreInterval   time.Duration
	FileStoragePath string
	Restore         bool
	DatabaseDSN     string
}

func NewServerConfig() (ServerConfig, error) {
	return ParseServerConfig(os.Args[0], os.Args[1:], os.LookupEnv)
}

func ParseServerConfig(name string, args []string, lookupEnv LookupEnvFunc) (ServerConfig, error) {
	cfg := ServerConfig{
		Address:         defaultServerAddress,
		FileStoragePath: defaultFileStoragePath,
		Restore:         defaultRestore,
	}

	fs := newFlagSet(name)
	fs.StringVar(&cfg.Address, "a", cfg.Address, "address of the HTTP server endpoint (env "+envAddress+")")
	storeInterval := fs.Int("i", seconds(defaultStoreInterval), "metrics store interval, in seconds; 0 saves on every update (env "+envStoreInterval+")")
	fs.StringVar(&cfg.FileStoragePath, "f", cfg.FileStoragePath, "file to store metrics in; empty disables storing (env "+envFileStoragePath+")")
	fs.BoolVar(&cfg.Restore, "r", cfg.Restore, "restore metrics from the file on start (env "+envRestore+")")
	fs.StringVar(&cfg.DatabaseDSN, "d", cfg.DatabaseDSN, "PostgreSQL connection string; empty disables the database (env "+envDatabaseDSN+")")

	if err := parse(fs, args); err != nil {
		return ServerConfig{}, err
	}

	if v, ok := envValue(lookupEnv, envAddress); ok {
		cfg.Address = v
	}

	if err := envSeconds(lookupEnv, envStoreInterval, storeInterval); err != nil {
		return ServerConfig{}, err
	}

	if v, ok := envValue(lookupEnv, envFileStoragePath); ok {
		cfg.FileStoragePath = v
	}

	if err := envBool(lookupEnv, envRestore, &cfg.Restore); err != nil {
		return ServerConfig{}, err
	}

	if v, ok := envValue(lookupEnv, envDatabaseDSN); ok {
		cfg.DatabaseDSN = v
	}

	if *storeInterval < 0 {
		return ServerConfig{}, fmt.Errorf("invalid store interval %d (flag -i or %s): must not be negative", *storeInterval, envStoreInterval)
	}

	cfg.StoreInterval = time.Duration(*storeInterval) * time.Second

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
	return ParseAgentConfig(os.Args[0], os.Args[1:], os.LookupEnv)
}

func ParseAgentConfig(name string, args []string, lookupEnv LookupEnvFunc) (AgentConfig, error) {
	cfg := AgentConfig{
		Collector: CollectorConfig{PollInterval: defaultPollInterval},
		Sender: SenderConfig{
			Address:        defaultServerAddress,
			ReportInterval: defaultReportInterval,
		},
	}

	fs := newFlagSet(name)
	fs.StringVar(&cfg.Sender.Address, "a", cfg.Sender.Address, "address of the HTTP server endpoint (env "+envAddress+")")
	reportInterval := fs.Int("r", seconds(cfg.Sender.ReportInterval), "metrics reporting interval, in seconds (env "+envReportInterval+")")
	pollInterval := fs.Int("p", seconds(cfg.Collector.PollInterval), "runtime metrics polling interval, in seconds (env "+envPollInterval+")")

	if err := parse(fs, args); err != nil {
		return AgentConfig{}, err
	}

	if v, ok := envValue(lookupEnv, envAddress); ok {
		cfg.Sender.Address = v
	}

	if err := envSeconds(lookupEnv, envReportInterval, reportInterval); err != nil {
		return AgentConfig{}, err
	}

	if err := envSeconds(lookupEnv, envPollInterval, pollInterval); err != nil {
		return AgentConfig{}, err
	}

	if *reportInterval <= 0 {
		return AgentConfig{}, fmt.Errorf("invalid report interval %d (flag -r or %s): must be positive", *reportInterval, envReportInterval)
	}

	if *pollInterval <= 0 {
		return AgentConfig{}, fmt.Errorf("invalid poll interval %d (flag -p or %s): must be positive", *pollInterval, envPollInterval)
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

func envValue(lookupEnv LookupEnvFunc, key string) (string, bool) {
	if lookupEnv == nil {
		return "", false
	}

	v, ok := lookupEnv(key)
	if !ok || v == "" {
		return "", false
	}

	return v, true
}

func envSeconds(lookupEnv LookupEnvFunc, key string, dst *int) error {
	v, ok := envValue(lookupEnv, key)
	if !ok {
		return nil
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("invalid %s=%q: must be an integer number of seconds", key, v)
	}

	*dst = n

	return nil
}

func envBool(lookupEnv LookupEnvFunc, key string, dst *bool) error {
	v, ok := envValue(lookupEnv, key)
	if !ok {
		return nil
	}

	b, err := strconv.ParseBool(v)
	if err != nil {
		return fmt.Errorf("invalid %s=%q: must be true or false", key, v)
	}

	*dst = b

	return nil
}

func seconds(d time.Duration) int {
	return int(d / time.Second)
}
