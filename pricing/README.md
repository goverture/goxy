# Configurable Pricing System

This package provides a flexible, configuration-driven pricing system for AI model usage calculations.

## Features

- **YAML Configuration**: Human-readable pricing configuration files
- **Model Aliases**: Support for multiple model name variants
- **Default Fallback**: Default pricing for unknown models
- **Cached Token Discounts**: Configurable discounts for cached prompt tokens
- **Backward Compatibility**: Existing API remains unchanged

## Configuration File Format

Create a `pricing.yaml` file with the following structure:

```yaml
# Pricing configuration for different AI models
# Prices are per 1000 tokens in USD

models:
  gpt-4:
    prompt: 0.03
    completion: 0.06
    aliases:
      - "gpt-4-0613"
  
  gpt-4o:
    prompt: 0.005
    completion: 0.015
    aliases:
      - "gpt-4o-2024-08-06"

# Default pricing for unknown models (optional)
default:
  prompt: 0.01
  completion: 0.02
  
# Cached token discount (percentage of original cost)
cached_token_discount: 0.1  # 90% discount = 10% of original cost
```

## Usage

### 1. Default Configuration Loading

The system automatically loads `pricing.yaml` from the package directory:

```go
import "github.com/goverture/goxy/pricing"

usage := pricing.Usage{
    PromptTokens:     1000,
    CompletionTokens: 500,
}

result, err := pricing.ComputePrice("gpt-4o", usage)
if err != nil {
    log.Fatal(err)
}

fmt.Println(result.String())
```

### 2. Custom Configuration File

Load pricing from a custom YAML file:

```go
import "github.com/goverture/goxy/pricing"

// Load custom configuration
err := pricing.LoadConfig("/path/to/custom/pricing.yaml")
if err != nil {
    log.Fatal(err)
}

// Use as normal
result, err := pricing.ComputePrice("gpt-4", usage)
```

### 3. Programmatic Configuration

Set configuration directly in code:

```go
import "github.com/goverture/goxy/pricing"

customConfig := &pricing.PricingConfig{
    Models: map[string]pricing.ModelPricing{
        "custom-model": {
            Prompt:     0.001,
            Completion: 0.002,
        },
    },
    Default: &pricing.ModelPricing{
        Prompt:     0.005,
        Completion: 0.01,
    },
    CachedTokenDiscount: 0.05, // 95% discount
}

pricing.SetConfig(customConfig)
```

## Configuration Options

### Models

Define pricing for specific models:

- `prompt`: Cost per 1000 prompt tokens (USD)
- `completion`: Cost per 1000 completion tokens (USD)  
- `aliases`: Array of alternative model names (optional)

### Default Pricing

Fallback pricing for unknown models:

- `prompt`: Default prompt token cost
- `completion`: Default completion token cost

### Cached Token Discount

Discount rate for cached prompt tokens:

- `cached_token_discount`: Percentage of original cost (0.0 to 1.0)
- Example: `0.1` means cached tokens cost 10% of normal tokens (90% discount)

## Model Aliases

The system supports multiple names for the same model:

```yaml
models:
  gpt-4:
    prompt: 0.03
    completion: 0.06
    aliases:
      - "gpt-4-0613"
      - "gpt-4-latest"
```

This allows `ComputePrice("gpt-4-0613", usage)` to use the same pricing as `gpt-4`.

## Cached Token Handling

When `PromptCachedTokens` is set in the usage, the system applies the configured discount:

```go
usage := pricing.Usage{
    PromptTokens:       1000,
    PromptCachedTokens: 600,  // 60% cached
    CompletionTokens:   500,
}

// Effective cost = (400 non-cached + 60 cached) tokens
// Where cached tokens cost 10% of normal rate
```

## Error Handling

The system gracefully handles various scenarios:

- **Missing config file**: Uses default behavior
- **Unknown models**: Uses default pricing if configured, otherwise returns pricing result with note
- **Invalid YAML**: Returns descriptive error messages
- **Invalid discount rates**: Validates discount is between 0.0 and 1.0

## Migration from Hardcoded Pricing

The new system is backward compatible. Existing code will continue to work without changes. To migrate:

1. Create a `pricing.yaml` file with your current pricing
2. No code changes required - the system will automatically load the configuration
3. Optionally, add model aliases and default pricing for better coverage

## Testing

The package includes comprehensive tests and provides utilities for testing:

```go
// In tests, use custom configuration
func TestMyFunction(t *testing.T) {
    testConfig := &pricing.PricingConfig{
        Models: map[string]pricing.ModelPricing{
            "test-model": {Prompt: 0.001, Completion: 0.002},
        },
        CachedTokenDiscount: 0.1,
    }
    
    pricing.SetConfig(testConfig)
    defer pricing.ResetConfig() // Clean up
    
    // Your test code here
}
```

## Example

See `example/main.go` for a complete working example demonstrating all features.