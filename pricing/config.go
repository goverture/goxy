package pricing

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"gopkg.in/yaml.v3"
)

// ModelPricing represents pricing for a single model
type ModelPricing struct {
	Prompt     float64  `yaml:"prompt"`
	Completion float64  `yaml:"completion"`
	Aliases    []string `yaml:"aliases,omitempty"`
}

// PricingConfig represents the entire pricing configuration
type PricingConfig struct {
	Models              map[string]ModelPricing `yaml:"models"`
	Default             *ModelPricing           `yaml:"default,omitempty"`
	CachedTokenDiscount float64                 `yaml:"cached_token_discount"`
}

var (
	config     *PricingConfig
	configOnce sync.Once
	configErr  error
)

// LoadConfig loads pricing configuration from a YAML file
func LoadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var cfg PricingConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", configPath, err)
	}

	// Validate config
	if cfg.CachedTokenDiscount < 0 || cfg.CachedTokenDiscount > 1 {
		return fmt.Errorf("cached_token_discount must be between 0 and 1, got %f", cfg.CachedTokenDiscount)
	}

	config = &cfg
	return nil
}

// GetConfig returns the loaded pricing configuration
// If no config has been loaded, it attempts to load from the default location
func GetConfig() (*PricingConfig, error) {
	configOnce.Do(func() {
		if config == nil {
			// Try to load from default location (same directory as this file)
			defaultPath := filepath.Join(filepath.Dir(getCurrentFilePath()), "pricing.yaml")
			configErr = LoadConfig(defaultPath)
		}
	})

	if configErr != nil {
		return nil, configErr
	}

	return config, nil
}

// SetConfig allows setting the configuration directly (useful for testing)
func SetConfig(cfg *PricingConfig) {
	config = cfg
}

// ResetConfig clears the loaded configuration (useful for testing)
func ResetConfig() {
	config = nil
	configOnce = sync.Once{}
	configErr = nil
}

// getCurrentFilePath returns the path of this source file
func getCurrentFilePath() string {
	// Use runtime.Caller to get the actual file path
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		// Fallback - try relative to working directory
		if wd, err := os.Getwd(); err == nil {
			return filepath.Join(wd, "pricing", "config.go")
		}
		return "config.go" // last resort
	}
	return filename
}

// FindModelPricing looks up pricing for a model, checking aliases
func (cfg *PricingConfig) FindModelPricing(modelName string) (*ModelPricing, bool) {
	// Direct lookup
	if pricing, exists := cfg.Models[modelName]; exists {
		return &pricing, true
	}

	// Check aliases
	for _, pricing := range cfg.Models {
		for _, alias := range pricing.Aliases {
			if alias == modelName {
				return &pricing, true
			}
		}
	}

	return nil, false
}
