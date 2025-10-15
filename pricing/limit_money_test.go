package pricing

import (
	"testing"
)

func TestManagerMoney_BasicOperations(t *testing.T) {
	limit := NewMoneyFromUSD(10.0) // $10 limit
	mgr := NewManagerMoney(limit)

	key := "test-key"

	// Initial state - should be allowed
	allowed, _, spent, returnedLimit := mgr.Allow(key)
	if !allowed {
		t.Error("Should be allowed initially")
	}
	if !spent.IsZero() {
		t.Errorf("Initial spent should be zero, got %s", spent.String())
	}
	if returnedLimit != limit {
		t.Errorf("Limit should be %s, got %s", limit.String(), returnedLimit.String())
	}

	// Add some cost
	cost1 := NewMoneyFromUSD(3.5)
	mgr.AddCost(key, cost1)

	// Check usage
	usage := mgr.GetUsage(key)
	if usage.Spent != cost1 {
		t.Errorf("Spent should be %s, got %s", cost1.String(), usage.Spent.String())
	}
	if !usage.Allowed {
		t.Error("Should still be allowed after spending $3.5 of $10 limit")
	}

	expectedRemaining := NewMoneyFromUSD(6.5)
	if usage.Remaining != expectedRemaining {
		t.Errorf("Remaining should be %s, got %s", expectedRemaining.String(), usage.Remaining.String())
	}

	// Add more cost that exceeds limit
	cost2 := NewMoneyFromUSD(7.0)
	mgr.AddCost(key, cost2)

	// Should now be blocked
	allowed, _, spent, _ = mgr.Allow(key)
	if allowed {
		t.Error("Should be blocked after exceeding limit")
	}

	expectedTotal := cost1.Add(cost2)
	if spent != expectedTotal {
		t.Errorf("Total spent should be %s, got %s", expectedTotal.String(), spent.String())
	}
}

func TestManagerMoney_USDConvenience(t *testing.T) {
	mgr := NewManagerMoneyFromUSD(5.0) // $5 limit

	key := "test-key"

	// Add cost using USD method
	mgr.AddCostFromUSD(key, 2.5)

	usage := mgr.GetUsage(key)
	expected := NewMoneyFromUSD(2.5)
	if usage.Spent != expected {
		t.Errorf("Spent should be %s, got %s", expected.String(), usage.Spent.String())
	}

	// Test updating limit from USD
	mgr.UpdateLimitFromUSD(10.0)

	usage = mgr.GetUsage(key)
	expectedLimit := NewMoneyFromUSD(10.0)
	if usage.Limit != expectedLimit {
		t.Errorf("Updated limit should be %s, got %s", expectedLimit.String(), usage.Limit.String())
	}
}

func TestManagerMoney_DisabledLimiter(t *testing.T) {
	// Test disabled limiter (negative limit)
	mgr := NewManagerMoneyFromUSD(-1.0)

	key := "test-key"

	// Should always be allowed
	allowed, _, _, _ := mgr.Allow(key)
	if !allowed {
		t.Error("Disabled limiter should always allow")
	}

	// Add large cost
	mgr.AddCostFromUSD(key, 1000000.0)

	// Should still be allowed
	allowed, _, _, _ = mgr.Allow(key)
	if !allowed {
		t.Error("Disabled limiter should still allow after large spend")
	}

	usage := mgr.GetUsage(key)
	if !usage.Allowed {
		t.Error("Usage info should show allowed for disabled limiter")
	}
	if !usage.Remaining.IsNegative() {
		t.Error("Remaining should be negative (unlimited) for disabled limiter")
	}
}

func TestManagerMoney_ZeroLimit(t *testing.T) {
	mgr := NewManagerMoneyFromUSD(0.0) // Zero limit - blocks everything

	key := "test-key"

	// Should be blocked immediately
	allowed, _, _, _ := mgr.Allow(key)
	if allowed {
		t.Error("Zero limit should immediately block")
	}

	usage := mgr.GetUsage(key)
	if usage.Allowed {
		t.Error("Zero limit usage should show not allowed")
	}
}

func TestManagerMoney_AnonymousKey(t *testing.T) {
	mgr := NewManagerMoneyFromUSD(10.0)

	// Anonymous key (empty string) should bypass tracking
	allowed, _, spent, _ := mgr.Allow("")
	if !allowed {
		t.Error("Anonymous key should be allowed")
	}
	if !spent.IsZero() {
		t.Error("Anonymous key should show zero spent")
	}

	// Adding cost to anonymous key should be ignored
	mgr.AddCostFromUSD("", 1000.0)

	// Still should show zero spent
	allowed, _, spent, _ = mgr.Allow("")
	if !allowed {
		t.Error("Anonymous key should still be allowed")
	}
	if !spent.IsZero() {
		t.Error("Anonymous key should still show zero spent")
	}
}

func TestManagerMoney_WindowReset(t *testing.T) {
	limit := NewMoneyFromUSD(10.0)
	mgr := NewManagerMoney(limit)

	key := "test-key"

	// Add cost that exceeds limit
	mgr.AddCostFromUSD(key, 15.0)

	// Should be blocked
	allowed, _, _, _ := mgr.Allow(key)
	if allowed {
		t.Error("Should be blocked after exceeding limit")
	}

	// Simulate time passing (we can't easily test this without modifying the window start time)
	// This test would require either dependency injection of time or a way to manipulate time
	// For now, we'll just verify the window tracking works correctly

	usage := mgr.GetUsage(key)
	if usage.WindowStart.IsZero() {
		t.Error("Window start should be set")
	}
	if usage.WindowEnd.IsZero() {
		t.Error("Window end should be set")
	}
	if usage.WindowEnd.Before(usage.WindowStart) {
		t.Error("Window end should be after window start")
	}
}

func TestManagerMoney_BackwardCompatibility(t *testing.T) {
	mgr := NewManagerMoneyFromUSD(10.0)
	key := "test-key"

	mgr.AddCostFromUSD(key, 3.5)

	// Test Money usage info
	usageMoney := mgr.GetUsage(key)

	// Convert to legacy format
	usageLegacy := usageMoney.ToLegacy()

	if usageLegacy.Key != usageMoney.Key {
		t.Error("Key should match in legacy conversion")
	}

	expectedSpentUSD := 3.5
	if abs(usageLegacy.SpentUSD-expectedSpentUSD) > 1e-8 {
		t.Errorf("Legacy spent USD should be %f, got %f", expectedSpentUSD, usageLegacy.SpentUSD)
	}

	expectedLimitUSD := 10.0
	if abs(usageLegacy.LimitUSD-expectedLimitUSD) > 1e-8 {
		t.Errorf("Legacy limit USD should be %f, got %f", expectedLimitUSD, usageLegacy.LimitUSD)
	}

	expectedRemainingUSD := 6.5
	if abs(usageLegacy.Remaining-expectedRemainingUSD) > 1e-8 {
		t.Errorf("Legacy remaining USD should be %f, got %f", expectedRemainingUSD, usageLegacy.Remaining)
	}
}
