package config

import (
	"flag"
)

var (
	// App-wide globals, set once in main().
	Cfg *Config
)

// Config holds the app configuration.
type Config struct {
	OpenAIBaseURL     string
	Port              int
	SpendLimitPerHour float64 // USD per API key per rolling hour (0 or <0 disables)
}

// ParseConfig parses command-line flags into a Config struct.
func ParseConfig() *Config {
	cfg := &Config{}
	flag.StringVar(&cfg.OpenAIBaseURL, "openai-base-url", "https://api.openai.com", "OpenAI API base URL")
	flag.IntVar(&cfg.Port, "port", 8080, "Port to listen on")
	flag.Float64Var(&cfg.SpendLimitPerHour, "spend-limit-per-hour", 2.0, "Per-API-key spend limit in USD per hour (<=0 to disable)")
	flag.Parse()
	return cfg
}
