package pricing

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"gopkg.in/yaml.v3"
)

// TierPricing represents pricing for a specific service tier
type TierPricing struct {
	Prompt     float64 `yaml:"prompt"`
	Completion float64 `yaml:"completion"`
}

// ModelPricing represents pricing for a single model with different service tiers
type ModelPricing struct {
	Prompt     float64      `yaml:"prompt"`
	Completion float64      `yaml:"completion"`
	Flex       *TierPricing `yaml:"flex,omitempty"`
	Priority   *TierPricing `yaml:"priority,omitempty"`
	Batch      *TierPricing `yaml:"batch,omitempty"`
	Aliases    []string     `yaml:"aliases,omitempty"`
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

// FindModelPricing looks up pricing for a model, checking for matching prefixes
func (cfg *PricingConfig) FindModelPricing(modelName string) (*ModelPricing, bool) {
	// Direct lookup first
	if pricing, exists := cfg.Models[modelName]; exists {
		return &pricing, true
	}

	// Check for prefix matches - find the longest matching prefix
	var bestMatch string
	var bestPricing *ModelPricing

	for configModel, pricing := range cfg.Models {
		// Check if the input model name starts with this config model name
		if len(configModel) > len(bestMatch) && len(modelName) >= len(configModel) &&
			modelName[:len(configModel)] == configModel {
			bestMatch = configModel
			pricingCopy := pricing // Create a copy to avoid pointer issues
			bestPricing = &pricingCopy
		}
	}

	if bestPricing != nil {
		return bestPricing, true
	}

	return nil, false
}

// GetTierPricing returns pricing for a specific service tier, falling back to standard if tier not available
func (mp *ModelPricing) GetTierPricing(serviceTier string) (prompt, completion float64, tier string) {
	switch serviceTier {
	case "flex":
		if mp.Flex != nil {
			return mp.Flex.Prompt, mp.Flex.Completion, "flex"
		}
	case "priority":
		if mp.Priority != nil {
			return mp.Priority.Prompt, mp.Priority.Completion, "priority"
		}
	case "batch":
		if mp.Batch != nil {
			return mp.Batch.Prompt, mp.Batch.Completion, "batch"
		}
	}
	// Fallback to standard pricing
	return mp.Prompt, mp.Completion, "standard"
}
