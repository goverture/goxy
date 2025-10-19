package config

import (
	"fmt"
	"os"

	"github.com/goverture/goxy/pricing"
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
	AdminPort         int
	SpendLimitPerHour float64 // USD per API key per rolling hour (0 or <0 disables)
}

// ParseConfig parses command-line flags into a Config struct.
func ParseConfig() *Config {
	cfg := &Config{}
	// Short forms: -u, -p, -a, -l ; Long forms: --openai-base-url, --port, --admin-port, --spend-limit-per-hour
	pflag.StringVarP(&cfg.OpenAIBaseURL, "openai-base-url", "u", "https://api.openai.com", "OpenAI API base URL")
	pflag.IntVarP(&cfg.Port, "port", "p", 8080, "Port to listen on")
	pflag.IntVarP(&cfg.AdminPort, "admin-port", "a", 8081, "Admin API port for usage monitoring and limit updates")
	pflag.Float64VarP(&cfg.SpendLimitPerHour, "spend-limit-per-hour", "l", 2.0, "Per-API-key spend limit USD per hour ( <0 disable, 0 block all )")

	var showVersion bool
	pflag.BoolVarP(&showVersion, "version", "v", false, "Show version and exit")
	pflag.Parse()

	if showVersion {
		// Version info will be set during build via -ldflags
		println("GoXY version:", Version)
		os.Exit(0)
	}

	// Validate spend limit against maximum representable money amount
	if cfg.SpendLimitPerHour > 0 && cfg.SpendLimitPerHour > pricing.MaxMoneyUSD() {
		fmt.Fprintf(os.Stderr, "Error: spend-limit-per-hour (%.2f) exceeds maximum representable amount (%.2f USD)\n",
			cfg.SpendLimitPerHour, pricing.MaxMoneyUSD())
		os.Exit(1)
	}

	return cfg
}
