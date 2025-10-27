package pricing

import (
	"fmt"
	"math"
)

// MonetaryUnit defines the precision for monetary calculations.
// Using nano cents (1/10,000,000,000 USD) provides 10 decimal places of precision.
// This handles even the smallest possible API costs while maintaining integer precision.
// This constant can be easily changed to adjust precision.
const MonetaryUnit = 10_000_000_000 // 1 USD = 10,000,000,000 nano cents

// MaxMoney represents the maximum representable monetary value.
// With int64 and MonetaryUnit=10^10, this is approximately $922,337,203.68
const MaxMoney Money = math.MaxInt64

// MinMoney represents the minimum positive representable monetary value.
// This is 1 nano cent = $0.0000000001
const MinMoney Money = 1

// MaxMoneyUSD returns the maximum representable USD amount as a float64.
// This is approximately $922,337,203.68
func MaxMoneyUSD() float64 {
	return float64(MaxMoney) / MonetaryUnit
}

// MinMoneyUSD returns the minimum positive representable USD amount as a float64.
// This is exactly $0.0000000001 (1 nano cent)
func MinMoneyUSD() float64 {
	return float64(MinMoney) / MonetaryUnit
}

// Money represents a monetary amount in nano cents (integer-based for precision)
type Money int64

// NewMoneyFromUSD creates a Money value from USD (float64)
func NewMoneyFromUSD(usd float64) Money {
	return Money(math.Round(usd * MonetaryUnit))
}

// ToUSD converts Money to USD (float64) for display/API compatibility
func (m Money) ToUSD() float64 {
	return float64(m) / MonetaryUnit
}

// Add adds two Money values
func (m Money) Add(other Money) Money {
	return m + other
}

// Multiply multiplies Money by a scalar (for token calculations)
func (m Money) Multiply(factor int64) Money {
	return m * Money(factor)
}

// String returns the Money as a formatted USD string
func (m Money) String() string {
	return fmt.Sprintf("$%.8f", m.ToUSD())
}

// IsZero returns true if the money amount is zero
func (m Money) IsZero() bool {
	return m == 0
}

// IsNegative returns true if the money amount is negative
func (m Money) IsNegative() bool {
	return m < 0
}

// LessThan returns true if this money amount is less than the other
func (m Money) LessThan(other Money) bool {
	return m < other
}

// GreaterThan returns true if this money amount is greater than the other
func (m Money) GreaterThan(other Money) bool {
	return m > other
}
