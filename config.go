package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Browser       BrowserConfig       `yaml:"browser"`
	Files         FilesConfig         `yaml:"files"`
	Retry         RetryConfig         `yaml:"retry"`
	RateLimiting  RateLimitingConfig  `yaml:"rate_limiting"`
	Logging       LoggingConfig       `yaml:"logging"`
}

type BrowserConfig struct {
	Headless           bool   `yaml:"headless"`
	UserDataDir        string `yaml:"user_data_dir"`
	QRTimeoutSeconds   int    `yaml:"qr_timeout_seconds"`
	PageLoadTimeout    int    `yaml:"page_load_timeout"`
}

type FilesConfig struct {
	CSVPath          string `yaml:"csv_path"`
	TemplatePath     string `yaml:"template_path"`
	CompletedCSVPath string `yaml:"completed_csv_path"`
}

type RetryConfig struct {
	MaxRetries         int     `yaml:"max_retries"`
	InitialDelaySeconds int    `yaml:"initial_delay_seconds"`
	MaxDelaySeconds    int     `yaml:"max_delay_seconds"`
	BackoffMultiplier  float64 `yaml:"backoff_multiplier"`
}

type RateLimitingConfig struct {
	MessagesPerSecond int  `yaml:"messages_per_second"`
	Enabled           bool `yaml:"enabled"`
}

type LoggingConfig struct {
	Level      string `yaml:"level"`
	OutputFile string `yaml:"output_file"`
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults if not specified
	if config.Browser.UserDataDir == "" {
		config.Browser.UserDataDir = "./chrome-data"
	}
	if config.Browser.QRTimeoutSeconds == 0 {
		config.Browser.QRTimeoutSeconds = 60
	}
	if config.Browser.PageLoadTimeout == 0 {
		config.Browser.PageLoadTimeout = 30
	}
	if config.Files.CompletedCSVPath == "" {
		config.Files.CompletedCSVPath = "completed.csv"
	}

	return &config, nil
}
