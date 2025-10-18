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
	Prompt       float64 `yaml:"prompt"`
	CachedPrompt float64 `yaml:"cached_prompt"`
	Completion   float64 `yaml:"completion"`
}

// TierPricingMoney represents pricing for a specific service tier using Money type
type TierPricingMoney struct {
	Prompt       Money
	CachedPrompt Money
	Completion   Money
}

// ModelPricing represents pricing for a single model with different service tiers
type ModelPricing struct {
	Prompt       float64      `yaml:"prompt"`
	CachedPrompt float64      `yaml:"cached_prompt"`
	Completion   float64      `yaml:"completion"`
	Flex         *TierPricing `yaml:"flex,omitempty"`
	Priority     *TierPricing `yaml:"priority,omitempty"`
	Batch        *TierPricing `yaml:"batch,omitempty"`
	Aliases      []string     `yaml:"aliases,omitempty"`
}

// ModelPricingMoney represents pricing for a single model using Money type
type ModelPricingMoney struct {
	Prompt       Money
	CachedPrompt Money
	Completion   Money
	Flex         *TierPricingMoney
	Priority     *TierPricingMoney
	Batch        *TierPricingMoney
	Aliases      []string
}

// PricingConfig represents the entire pricing configuration
type PricingConfig struct {
	Models  map[string]ModelPricing `yaml:"models"`
	Default *ModelPricing           `yaml:"default,omitempty"`
}

// PricingConfigMoney represents the entire pricing configuration using Money type
type PricingConfigMoney struct {
	Models  map[string]ModelPricingMoney
	Default *ModelPricingMoney
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
func (mp *ModelPricing) GetTierPricing(serviceTier string) (prompt, cachedPrompt, completion float64, tier string) {
	switch serviceTier {
	case "flex":
		if mp.Flex != nil {
			return mp.Flex.Prompt, mp.Flex.CachedPrompt, mp.Flex.Completion, "flex"
		}
	case "priority":
		if mp.Priority != nil {
			return mp.Priority.Prompt, mp.Priority.CachedPrompt, mp.Priority.Completion, "priority"
		}
	case "batch":
		if mp.Batch != nil {
			return mp.Batch.Prompt, mp.Batch.CachedPrompt, mp.Batch.Completion, "batch"
		}
	}
	// Fallback to standard pricing
	return mp.Prompt, mp.CachedPrompt, mp.Completion, "standard"
}

// ToMoney converts float64-based ModelPricing to Money-based ModelPricingMoney
func (mp *ModelPricing) ToMoney() ModelPricingMoney {
	result := ModelPricingMoney{
		Prompt:       NewMoneyFromUSD(mp.Prompt / 1000000.0),
		CachedPrompt: NewMoneyFromUSD(mp.CachedPrompt / 1000000.0),
		Completion:   NewMoneyFromUSD(mp.Completion / 1000000.0),
		Aliases:      mp.Aliases,
	}

	if mp.Flex != nil {
		result.Flex = &TierPricingMoney{
			Prompt:       NewMoneyFromUSD(mp.Flex.Prompt / 1000000.0),
			CachedPrompt: NewMoneyFromUSD(mp.Flex.CachedPrompt / 1000000.0),
			Completion:   NewMoneyFromUSD(mp.Flex.Completion / 1000000.0),
		}
	}

	if mp.Priority != nil {
		result.Priority = &TierPricingMoney{
			Prompt:       NewMoneyFromUSD(mp.Priority.Prompt / 1000000.0),
			CachedPrompt: NewMoneyFromUSD(mp.Priority.CachedPrompt / 1000000.0),
			Completion:   NewMoneyFromUSD(mp.Priority.Completion / 1000000.0),
		}
	}

	if mp.Batch != nil {
		result.Batch = &TierPricingMoney{
			Prompt:       NewMoneyFromUSD(mp.Batch.Prompt / 1000000.0),
			CachedPrompt: NewMoneyFromUSD(mp.Batch.CachedPrompt / 1000000.0),
			Completion:   NewMoneyFromUSD(mp.Batch.Completion / 1000000.0),
		}
	}

	return result
}

// ToMoney converts float64-based PricingConfig to Money-based PricingConfigMoney
func (cfg *PricingConfig) ToMoney() PricingConfigMoney {
	result := PricingConfigMoney{
		Models: make(map[string]ModelPricingMoney),
	}

	for name, pricing := range cfg.Models {
		result.Models[name] = pricing.ToMoney()
	}

	if cfg.Default != nil {
		defaultMoney := cfg.Default.ToMoney()
		result.Default = &defaultMoney
	}

	return result
}

// GetTierPricingMoney returns Money-based pricing for a specific service tier
func (mp *ModelPricingMoney) GetTierPricingMoney(serviceTier string) (prompt, cachedPrompt, completion Money, tier string) {
	switch serviceTier {
	case "flex":
		if mp.Flex != nil {
			return mp.Flex.Prompt, mp.Flex.CachedPrompt, mp.Flex.Completion, "flex"
		}
	case "priority":
		if mp.Priority != nil {
			return mp.Priority.Prompt, mp.Priority.CachedPrompt, mp.Priority.Completion, "priority"
		}
	case "batch":
		if mp.Batch != nil {
			return mp.Batch.Prompt, mp.Batch.CachedPrompt, mp.Batch.Completion, "batch"
		}
	}
	// Fallback to standard pricing
	return mp.Prompt, mp.CachedPrompt, mp.Completion, "standard"
}

// FindModelPricingMoney looks up Money-based pricing for a model
func (cfg *PricingConfigMoney) FindModelPricingMoney(modelName string) (*ModelPricingMoney, bool) {
	// Direct lookup first
	if pricing, exists := cfg.Models[modelName]; exists {
		return &pricing, true
	}

	// Check for prefix matches - find the longest matching prefix
	var bestMatch string
	var bestPricing *ModelPricingMoney

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
