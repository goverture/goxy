package pricing

import (
	"math"
	"testing"
)

func TestMoney_Precision(t *testing.T) {
	tests := []struct {
		name     string
		usd      float64
		expected Money
	}{
		{"Zero", 0.0, Money(0)},
		{"Simple dollar", 1.0, Money(10_000_000_000)},
		{"Small amount", 0.00000001, Money(100)},
		{"Complex amount", 12.345678, Money(123_456_780_000)},
		{"Very precise", 0.12345678, Money(1_234_567_800)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			money := NewMoneyFromUSD(tt.usd)
			if money != tt.expected {
				t.Errorf("NewMoneyFromUSD(%f) = %d, want %d", tt.usd, money, tt.expected)
			}

			// Test round-trip conversion
			backToUSD := money.ToUSD()
			if math.Abs(backToUSD-tt.usd) > 1e-10 {
				t.Errorf("Round-trip conversion failed: %f -> %d -> %f", tt.usd, money, backToUSD)
			}
		})
	}
}

func TestMoney_SmallestUnitAccumulation(t *testing.T) {
	// Test that we can accumulate the smallest possible monetary unit without losing precision
	// This demonstrates the advantage of integer arithmetic over floating-point

	// The smallest unit is 1 nano cent = 1/MonetaryUnit USD = 1/10,000,000,000 USD
	smallestUnit := Money(1)            // 1 nano cent
	smallestUSD := smallestUnit.ToUSD() // Should be 0.0000000001 USD

	t.Logf("Smallest unit: %s (%.10f USD)", smallestUnit.String(), smallestUSD)

	// Accumulate 1 billion smallest units - should equal exactly $0.10
	iterations := 1_000_000_000
	var total Money

	for i := 0; i < iterations; i++ {
		total = total.Add(smallestUnit)
	}

	expected := NewMoneyFromUSD(0.1) // Should be exactly $0.10

	if total != expected {
		t.Errorf("Smallest unit accumulation failed: got %s, want %s", total.String(), expected.String())
		t.Errorf("Got: %d nano cents, Want: %d nano cents", int64(total), int64(expected))
	}

	// Verify the exact USD conversion
	totalUSD := total.ToUSD()
	if totalUSD != 0.1 {
		t.Errorf("USD conversion failed: got %.10f, want 0.1000000000", totalUSD)
	}

	t.Logf("✅ Perfect precision: %d × %s = %s", iterations, smallestUnit.String(), total.String())

	// Compare with equivalent float64 arithmetic to show the difference
	var floatTotal float64
	smallestFloat := 0.0000000001 // 1/10,000,000,000

	for i := 0; i < iterations; i++ {
		floatTotal += smallestFloat
	}

	floatDiff := math.Abs(floatTotal - 0.1)
	t.Logf("Float64 result: %.10f (difference from 0.1: %e)", floatTotal, floatDiff)

	if floatDiff > 0 {
		t.Logf("✅ Money type maintains perfect precision while float64 loses precision")
	} else {
		t.Logf("⚠️  Float64 surprisingly maintained precision in this case")
	}
}

func TestMoney_Arithmetic(t *testing.T) {
	money1 := NewMoneyFromUSD(10.5)
	money2 := NewMoneyFromUSD(5.25)

	// Test addition
	sum := money1.Add(money2)
	expectedSum := NewMoneyFromUSD(15.75)
	if sum != expectedSum {
		t.Errorf("Add: got %d, want %d", sum, expectedSum)
	}

	// Test multiplication
	product := money1.Multiply(2.0)
	expectedProduct := NewMoneyFromUSD(21.0)
	if product != expectedProduct {
		t.Errorf("Multiply: got %d, want %d", product, expectedProduct)
	}

	// Test comparison methods
	if !money1.GreaterThan(money2) {
		t.Error("GreaterThan: expected money1 > money2")
	}

	if money2.GreaterThan(money1) {
		t.Error("GreaterThan: expected money2 < money1")
	}

	if !money2.LessThan(money1) {
		t.Error("LessThan: expected money2 < money1")
	}
}

func TestMoney_Accumulation_Precision(t *testing.T) {
	// This test demonstrates the precision benefit of using integers
	// Simulate adding many small amounts (like micro-transactions)

	smallAmount := NewMoneyFromUSD(0.00000123) // Very small amount
	total := Money(0)

	// Add it 1 million times
	for i := 0; i < 1_000_000; i++ {
		total = total.Add(smallAmount)
	}

	expected := NewMoneyFromUSD(1.23) // 0.00000123 * 1,000,000

	// With Money (integer), we should get exact precision
	if total != expected {
		t.Errorf("Money accumulation: got %d (%s), want %d (%s)", total, total.String(), expected, expected.String())
	}

	// Compare with float64 accumulation (which would lose precision)
	var floatTotal float64
	smallFloat := 0.00000123
	for i := 0; i < 1_000_000; i++ {
		floatTotal += smallFloat
	}

	// This should demonstrate floating point precision loss
	floatDiff := math.Abs(floatTotal - 1.23)
	if floatDiff < 1e-10 {
		t.Logf("Surprisingly, float64 maintained precision: %f (diff: %e)", floatTotal, floatDiff)
	} else {
		t.Logf("As expected, float64 lost precision: %f (diff: %e)", floatTotal, floatDiff)
		t.Logf("Money maintained exact precision: %s", total.String())
	}
}

func TestMaxMoney(t *testing.T) {
	// Test that MaxMoney constants are correct
	expectedMaxUSD := float64(math.MaxInt64) / MonetaryUnit
	actualMaxUSD := MaxMoneyUSD()

	if actualMaxUSD != expectedMaxUSD {
		t.Errorf("MaxMoneyUSD() mismatch: got %.10f, want %.10f", actualMaxUSD, expectedMaxUSD)
	}

	t.Logf("Maximum representable amount: $%.2f", actualMaxUSD)
	t.Logf("Maximum Money value: %d nano-cents", int64(MaxMoney))

	// Test that we can actually create and use MaxMoney
	maxMoney := MaxMoney
	if maxMoney.ToUSD() != expectedMaxUSD {
		t.Errorf("MaxMoney.ToUSD() mismatch: got %.10f, want %.10f", maxMoney.ToUSD(), expectedMaxUSD)
	}

	// Test that trying to create money beyond max would overflow (conceptually)
	// We can't easily test overflow since NewMoneyFromUSD uses math.Round
	// But we can verify the MaxMoneyUSD value is reasonable
	if actualMaxUSD < 900_000_000 {
		t.Errorf("MaxMoneyUSD seems too small: %.2f (expected > 900 million)", actualMaxUSD)
	}

	if actualMaxUSD > 1_000_000_000 {
		t.Errorf("MaxMoneyUSD seems too large: %.2f (expected < 1 billion)", actualMaxUSD)
	}
}

func TestMinMoney(t *testing.T) {
	// Test that MinMoney constants are correct
	expectedMinUSD := float64(1) / MonetaryUnit // 1 nano cent
	actualMinUSD := MinMoneyUSD()

	if actualMinUSD != expectedMinUSD {
		t.Errorf("MinMoneyUSD() mismatch: got %.15f, want %.15f", actualMinUSD, expectedMinUSD)
	}

	t.Logf("Minimum representable amount: $%.15f", actualMinUSD)
	t.Logf("Minimum Money value: %d nano-cents", int64(MinMoney))

	// Test that we can actually create and use MinMoney
	minMoney := MinMoney
	if minMoney.ToUSD() != expectedMinUSD {
		t.Errorf("MinMoney.ToUSD() mismatch: got %.15f, want %.15f", minMoney.ToUSD(), expectedMinUSD)
	}

	// Verify MinMoney is exactly 1 nano cent
	if MinMoney != 1 {
		t.Errorf("MinMoney should be 1 nano cent, got %d", int64(MinMoney))
	}

	// Test that MinMoney is the smallest positive value we can represent
	expectedValue := 0.0000000001 // 1/10^10
	if actualMinUSD != expectedValue {
		t.Errorf("MinMoneyUSD should be exactly %.15f, got %.15f", expectedValue, actualMinUSD)
	}

	// Test precision: adding MinMoney many times should work correctly
	var accumulated Money
	iterations := 1000
	for i := 0; i < iterations; i++ {
		accumulated = accumulated.Add(MinMoney)
	}

	expectedAccumulated := MinMoney.Multiply(float64(iterations))
	if accumulated != expectedAccumulated {
		t.Errorf("MinMoney accumulation failed: got %d, want %d", int64(accumulated), int64(expectedAccumulated))
	}

	t.Logf("✅ Successfully accumulated %d × MinMoney = %s", iterations, accumulated.String())
}
