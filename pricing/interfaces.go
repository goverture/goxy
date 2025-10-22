package pricing

import "time"

// LimitManager defines the interface for usage tracking and limiting
type LimitManager interface {
	// Allow checks whether the given key is currently allowed to spend more
	Allow(key string) (allowed bool, windowEnd time.Time, spentSoFar Money, limit Money)

	// AddCost adds the provided spend to a key's current window
	AddCost(key string, delta Money)

	// AddCostFromUSD adds the provided USD spend converted to Money to a key's current window
	AddCostFromUSD(key string, deltaUSD float64)

	// GetUsage returns usage information for a specific key
	GetUsage(key string) UsageInfoMoney

	// GetAllUsage returns usage information for all tracked keys
	GetAllUsage() []UsageInfoMoney

	// UpdateLimit updates the spending limit using Money
	UpdateLimit(newLimit Money)

	// UpdateLimitFromUSD updates the spending limit from USD
	UpdateLimitFromUSD(newLimitUSD float64)
}

// PersistentLimitManager extends LimitManager with persistence-specific functionality
type PersistentLimitManager interface {
	LimitManager

	// AddCostWithMaskedKey adds cost with both hashed key (for tracking) and masked key (for display)
	AddCostWithMaskedKey(key string, maskedKey string, delta Money)

	// GetAllUsageWithMaskedKeys returns usage information with original masked keys from database
	GetAllUsageWithMaskedKeys() []UsageInfoMoney

	// Close shuts down the persistent manager gracefully
	Close() error
}
