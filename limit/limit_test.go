package limit

import (
	"math"
	"sync"
	"testing"
	"time"
)

func almostEqual(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestAllowAndAddCostProgression(t *testing.T) {
	m := NewManager(1.0) // $1 per hour
	key := "k1"
	allowed, _, spent, lim := m.Allow(key)
	if !allowed || spent != 0 || lim != 1.0 {
		t.Fatalf("expected initial allowance with 0 spent")
	}

	m.AddCost(key, 0.25)
	allowed, _, spent, _ = m.Allow(key)
	if !allowed || !almostEqual(spent, 0.25) {
		t.Fatalf("expected spent=0.25 allowed=true got spent=%f allowed=%v", spent, allowed)
	}

	m.AddCost(key, 0.74) // total 0.99 < 1.0 still allowed
	allowed, _, spent, _ = m.Allow(key)
	if !allowed || !almostEqual(spent, 0.99) {
		t.Fatalf("expected allowed at 0.99 spent got allowed=%v spent=%f", allowed, spent)
	}
	m.AddCost(key, 0.02) // now 1.01 > 1.0 should block
	allowed, _, spent, _ = m.Allow(key)
	if allowed || spent < 1.0 {
		t.Fatalf("expected disallowed after exceeding limit; allowed=%v spent=%f", allowed, spent)
	}
}

func TestLimitExceeded(t *testing.T) {
	m := NewManager(0.5)
	key := "k2"
	m.AddCost(key, 0.5)
	allowed, _, spent, _ := m.Allow(key)
	if allowed {
		t.Fatalf("should be blocked when spent==limit; spent=%f", spent)
	}
}

func TestWindowReset(t *testing.T) {
	m := NewManager(1.0)
	key := "k3"
	m.AddCost(key, 0.4)
	allowed, windowEnd, spent, _ := m.Allow(key)
	if !allowed || !almostEqual(spent, 0.4) {
		t.Fatalf("precondition failed; spent=%f allowed=%v", spent, allowed)
	}
	// Force window expiration by manipulating internal state.
	v, ok := m.perKey.Load(key)
	if !ok {
		t.Fatalf("expected key window present")
	}
	kw := v.(*keyWindow)
	kw.mu.Lock()
	kw.windowStart = time.Now().Add(-time.Hour - time.Second)
	kw.mu.Unlock()
	allowed2, _, spent2, _ := m.Allow(key)
	if !allowed2 || spent2 != 0 {
		t.Fatalf("expected reset window spent=0 allowed2=%v spent2=%f (old windowEnd=%s)", allowed2, spent2, windowEnd)
	}
}

func TestDisabledLimit(t *testing.T) {
	m := NewManager(-1) // disabled
	key := "k4"
	m.AddCost(key, 10) // should be ignored
	allowed, _, spent, lim := m.Allow(key)
	if !allowed || spent != 0 || lim != -1 {
		t.Fatalf("disabled limit should always allow; spent=%f lim=%f", spent, lim)
	}
}

func TestZeroLimitBlocksAll(t *testing.T) {
	m := NewManager(0)
	key := "zero"
	allowed, _, spent, lim := m.Allow(key)
	if allowed || spent != 0 || lim != 0 {
		t.Fatalf("zero limit should block immediately; allowed=%v spent=%f lim=%f", allowed, spent, lim)
	}
	m.AddCost(key, 1.0) // no-op
	allowed2, _, spent2, _ := m.Allow(key)
	if allowed2 || spent2 != 0 {
		t.Fatalf("zero limit should still block; allowed2=%v spent2=%f", allowed2, spent2)
	}
}

func TestAnonymousKey(t *testing.T) {
	m := NewManager(0.5)
	allowed, _, spent, _ := m.Allow("")
	if !allowed || spent != 0 {
		t.Fatalf("anonymous key should be allowed and untracked")
	}
}

func TestConcurrentAddCost(t *testing.T) {
	m := NewManager(10.0)
	key := "conc"
	var wg sync.WaitGroup
	iters := 100
	wg.Add(iters)
	for i := 0; i < iters; i++ {
		go func() { defer wg.Done(); m.AddCost(key, 0.01) }()
	}
	wg.Wait()
	allowed, _, spent, _ := m.Allow(key)
	if !allowed {
		t.Fatalf("should still be allowed; limit not reached")
	}
	// Spent should be ~1.0
	if math.Abs(spent-1.0) > 1e-6 {
		t.Fatalf("expected spent near 1.0 got %f", spent)
	}
}

func TestWindowResetAfterExceeded(t *testing.T) {
	m := NewManager(1.0)
	key := "exceeded"
	// Drive spend over the limit
	m.AddCost(key, 0.6)
	m.AddCost(key, 0.5) // total 1.1 > 1.0
	allowed, windowEnd, spent, lim := m.Allow(key)
	if allowed || spent <= lim {
		t.Fatalf("expected blocked after exceeding limit; allowed=%v spent=%f lim=%f", allowed, spent, lim)
	}
	// Force window expiry
	v, ok := m.perKey.Load(key)
	if !ok {
		t.Fatalf("expected key window present")
	}
	kw := v.(*keyWindow)
	kw.mu.Lock()
	kw.windowStart = time.Now().Add(-time.Hour - time.Second)
	kw.mu.Unlock()
	allowed2, _, spent2, _ := m.Allow(key)
	if !allowed2 || spent2 != 0 {
		t.Fatalf("expected allowance reset after window; allowed2=%v spent2=%f (oldWindowEnd=%s)", allowed2, spent2, windowEnd)
	}
}
