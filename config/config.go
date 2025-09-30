package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Workers    WorkersConfig       `yaml:"workers"`
	Bulk       BulkConfig          `yaml:"bulk"`
	Strategy   string              `yaml:"strategy"`
	API        APIConfig           `yaml:"api"`
	Elastic    ElasticsearchConfig `yaml:"elasticsearch"`
	JSON       JSONConfig          `yaml:"json"`
	Retry      RetryConfig         `yaml:"retry"`
	Logging    LoggingConfig       `yaml:"logging"`
	Checkpoint CheckpointConfig    `yaml:"checkpoint"`
}

type WorkersConfig struct {
	Fetchers   int `yaml:"fetchers"`
	Processors int `yaml:"processors"`
}

type BulkConfig struct {
	Size    int           `yaml:"size"`
	Timeout time.Duration `yaml:"timeout"`
}

type APIConfig struct {
	BaseURL    string        `yaml:"base_url"`
	RateLimit  time.Duration `yaml:"rate_limit"`
	Timeout    time.Duration `yaml:"timeout"`
	PageSize   int           `yaml:"page_size"`
	MaxRetries int           `yaml:"max_retries"`
}

type ElasticsearchConfig struct {
	Hosts         []string      `yaml:"hosts"`
	Index         string        `yaml:"index"`
	Username      string        `yaml:"username"`
	Password      string        `yaml:"password"`
	Timeout       time.Duration `yaml:"timeout"`
	SkipTLSVerify bool          `yaml:"skip_tls_verify"`
	CACertPath    string        `yaml:"ca_cert_path"`
}

type JSONConfig struct {
	OutputDir   string `yaml:"output_dir"`
	FilePattern string `yaml:"file_pattern"`
	Format      string `yaml:"format"` // "array" or "ndjson"
}

type RetryConfig struct {
	MaxRetries int           `yaml:"max_retries"`
	Delay      time.Duration `yaml:"delay"`
	Backoff    float64       `yaml:"backoff"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`
	OutputFile string `yaml:"output_file"`
}

type CheckpointConfig struct {
	Enabled  bool   `yaml:"enabled"`
	FilePath string `yaml:"file_path"`
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Override with environment variables
	config.overrideWithEnv()

	// Validate configuration
	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

func (c *Config) overrideWithEnv() {
	if val := os.Getenv("EPSS_ELASTIC_HOSTS"); val != "" {
		c.Elastic.Hosts = []string{val}
	}
	if val := os.Getenv("EPSS_ELASTIC_USERNAME"); val != "" {
		c.Elastic.Username = val
	}
	if val := os.Getenv("EPSS_ELASTIC_PASSWORD"); val != "" {
		c.Elastic.Password = val
	}
	if val := os.Getenv("EPSS_ELASTIC_INDEX"); val != "" {
		c.Elastic.Index = val
	}
	if val := os.Getenv("EPSS_ELASTIC_SKIP_TLS_VERIFY"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			c.Elastic.SkipTLSVerify = b
		}
	}
	if val := os.Getenv("EPSS_ELASTIC_CA_CERT_PATH"); val != "" {
		c.Elastic.CACertPath = val
	}
	if val := os.Getenv("EPSS_WORKERS_FETCHERS"); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			c.Workers.Fetchers = i
		}
	}
	if val := os.Getenv("EPSS_WORKERS_PROCESSORS"); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			c.Workers.Processors = i
		}
	}
	if val := os.Getenv("EPSS_STRATEGY"); val != "" {
		c.Strategy = val
	}
}

func (c *Config) validate() error {
	if c.Workers.Fetchers <= 0 {
		return fmt.Errorf("workers.fetchers must be > 0")
	}
	if c.Workers.Processors <= 0 {
		return fmt.Errorf("workers.processors must be > 0")
	}
	if c.Bulk.Size <= 0 {
		return fmt.Errorf("bulk.size must be > 0")
	}
	if c.Strategy != "elasticsearch" && c.Strategy != "json" {
		return fmt.Errorf("strategy must be 'elasticsearch' or 'json'")
	}
	if c.API.PageSize <= 0 {
		return fmt.Errorf("api.page_size must be > 0")
	}

	if c.Strategy == "elasticsearch" {
		if len(c.Elastic.Hosts) == 0 {
			return fmt.Errorf("elasticsearch.hosts cannot be empty")
		}
		if c.Elastic.Index == "" {
			return fmt.Errorf("elasticsearch.index cannot be empty")
		}
	}

	if c.Strategy == "json" {
		if c.JSON.OutputDir == "" {
			return fmt.Errorf("json.output_dir cannot be empty")
		}
		if c.JSON.Format != "array" && c.JSON.Format != "ndjson" {
			return fmt.Errorf("json.format must be 'array' or 'ndjson'")
		}
	}

	return nil
}
