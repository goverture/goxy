package config

import (
	"github.com/spf13/pflag"
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
	// Short forms: -u, -p, -l ; Long forms: --openai-base-url, --port, --spend-limit-per-hour
	pflag.StringVarP(&cfg.OpenAIBaseURL, "openai-base-url", "u", "https://api.openai.com", "OpenAI API base URL")
	pflag.IntVarP(&cfg.Port, "port", "p", 8080, "Port to listen on")
	pflag.Float64VarP(&cfg.SpendLimitPerHour, "spend-limit-per-hour", "l", 2.0, "Per-API-key spend limit USD per hour ( <0 disable, 0 block all )")
	pflag.Parse()
	return cfg
}
