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
	OpenAIBaseURL string
	Port          int
}

// ParseConfig parses command-line flags into a Config struct.
func ParseConfig() *Config {
	cfg := &Config{}
	flag.StringVar(&cfg.OpenAIBaseURL, "openai-base-url", "https://api.openai.com", "OpenAI API base URL")
	flag.IntVar(&cfg.Port, "port", 8080, "Port to listen on")
	flag.Parse()
	return cfg
}
