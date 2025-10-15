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
