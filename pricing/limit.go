package pricing

import (
	"sync"
	"time"
)

// Manager maintains per-key rolling (hour-bucket) spend tracking.
// A simple fixed 1h window that resets when an hour has elapsed since first spend
// in the window. Good enough for coarse limiting; not a precise sliding window.
// Semantics:
//
//	limit < 0  => limiter disabled (all allowed, nothing tracked)
//	limit == 0 => zero allowance (every non-anonymous key immediately blocked)
//	limit > 0  => spend allowed until accumulated >= limit
type Manager struct {
	limit  float64  // USD per hour (<=0 disables)
	perKey sync.Map // map[string]*keyWindow
}

// ManagerMoney maintains per-key rolling (hour-bucket) spend tracking using Money for precision.
// A simple fixed 1h window that resets when an hour has elapsed since first spend
// in the window. Good enough for coarse limiting; not a precise sliding window.
// Semantics:
//
//	limit < 0  => limiter disabled (all allowed, nothing tracked)
//	limit == 0 => zero allowance (every non-anonymous key immediately blocked)
//	limit > 0  => spend allowed until accumulated >= limit
type ManagerMoney struct {
	limit  Money    // Money per hour (negative disables)
	perKey sync.Map // map[string]*keyWindowMoney
}

type keyWindow struct {
	mu          sync.Mutex
	windowStart time.Time
	spentUSD    float64
}

type keyWindowMoney struct {
	mu          sync.Mutex
	windowStart time.Time
	spent       Money
}

// NewManager creates a spend limit manager with the given per-hour limit (USD).
// Pass <=0 to disable limiting.
func NewManager(limit float64) *Manager { return &Manager{limit: limit} }

// NewManagerMoney creates a spend limit manager with the given per-hour limit (Money).
// Pass negative value to disable limiting.
func NewManagerMoney(limit Money) *ManagerMoney { return &ManagerMoney{limit: limit} }

// NewManagerMoneyFromUSD creates a spend limit manager with the given per-hour limit converted from USD.
// Pass <0 to disable limiting, pass 0 for zero allowance.
func NewManagerMoneyFromUSD(limitUSD float64) *ManagerMoney {
	var limit Money
	if limitUSD < 0 {
		limit = Money(-1) // Use -1 to indicate disabled
	} else {
		limit = NewMoneyFromUSD(limitUSD)
	}
	return &ManagerMoney{limit: limit}
}

// Allow checks whether the given key is currently allowed to spend more.
// It returns allowed, windowEnd, spentSoFar, limit.
func (m *Manager) Allow(key string) (bool, time.Time, float64, float64) {
	lim := m.limit
	if key == "" { // anonymous bypasses but not tracked
		return true, time.Time{}, 0, lim
	}
	if lim < 0 { // disabled limiter
		return true, time.Time{}, 0, lim
	}
	if lim == 0 { // immediate block for any spend
		return false, time.Now().Add(time.Hour), 0, lim
	}
	kw := m.getKW(key)
	kw.mu.Lock()
	defer kw.mu.Unlock()
	now := time.Now()
	if now.Sub(kw.windowStart) >= time.Hour { // reset window
		kw.windowStart = now
		kw.spentUSD = 0
	}
	windowEnd := kw.windowStart.Add(time.Hour)
	return kw.spentUSD < lim, windowEnd, kw.spentUSD, lim
}

// AddCost adds the provided USD spend to a key's current window.
func (m *Manager) AddCost(key string, delta float64) {
	if delta <= 0 || key == "" {
		return
	}
	if m.limit < 0 {
		return
	} // disabled
	if m.limit == 0 {
		return
	} // zero allowance blocked earlier
	kw := m.getKW(key)
	kw.mu.Lock()
	now := time.Now()
	if now.Sub(kw.windowStart) >= time.Hour { // window expired
		kw.windowStart = now
		kw.spentUSD = 0
	}
	kw.spentUSD += delta
	kw.mu.Unlock()
}

func (m *Manager) getKW(key string) *keyWindow {
	if v, ok := m.perKey.Load(key); ok {
		return v.(*keyWindow)
	}
	kw := &keyWindow{windowStart: time.Now()}
	actual, _ := m.perKey.LoadOrStore(key, kw)
	return actual.(*keyWindow)
}

// UsageInfo holds information about a key's current usage window
type UsageInfo struct {
	Key         string    `json:"key"`
	SpentUSD    float64   `json:"spent_usd"`
	LimitUSD    float64   `json:"limit_usd"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	Remaining   float64   `json:"remaining_usd"`
	Allowed     bool      `json:"allowed"`
}

// GetUsage returns usage information for a specific key
func (m *Manager) GetUsage(key string) UsageInfo {
	lim := m.limit
	now := time.Now()

	if key == "" {
		return UsageInfo{
			Key:         "anonymous",
			SpentUSD:    0,
			LimitUSD:    lim,
			WindowStart: now,
			WindowEnd:   now.Add(time.Hour),
			Remaining:   lim,
			Allowed:     lim != 0,
		}
	}

	if lim < 0 {
		return UsageInfo{
			Key:         key,
			SpentUSD:    0,
			LimitUSD:    lim,
			WindowStart: now,
			WindowEnd:   now.Add(time.Hour),
			Remaining:   -1, // unlimited
			Allowed:     true,
		}
	}

	kw := m.getKW(key)
	kw.mu.Lock()
	if now.Sub(kw.windowStart) >= time.Hour {
		kw.windowStart = now
		kw.spentUSD = 0
	}
	windowEnd := kw.windowStart.Add(time.Hour)
	spent := kw.spentUSD
	kw.mu.Unlock()

	remaining := lim - spent
	if remaining < 0 {
		remaining = 0
	}

	return UsageInfo{
		Key:         key,
		SpentUSD:    spent,
		LimitUSD:    lim,
		WindowStart: kw.windowStart,
		WindowEnd:   windowEnd,
		Remaining:   remaining,
		Allowed:     spent < lim,
	}
}

// GetAllUsage returns usage information for all tracked keys
func (m *Manager) GetAllUsage() []UsageInfo {
	var usage []UsageInfo
	now := time.Now()

	m.perKey.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		kw := value.(*keyWindow)

		kw.mu.Lock()
		if now.Sub(kw.windowStart) >= time.Hour {
			kw.windowStart = now
			kw.spentUSD = 0
		}
		windowEnd := kw.windowStart.Add(time.Hour)
		spent := kw.spentUSD
		kw.mu.Unlock()

		remaining := m.limit - spent
		if remaining < 0 {
			remaining = 0
		}

		usage = append(usage, UsageInfo{
			Key:         keyStr,
			SpentUSD:    spent,
			LimitUSD:    m.limit,
			WindowStart: kw.windowStart,
			WindowEnd:   windowEnd,
			Remaining:   remaining,
			Allowed:     m.limit < 0 || spent < m.limit,
		})
		return true
	})

	return usage
}

// UpdateLimit updates the spending limit for the manager
func (m *Manager) UpdateLimit(newLimit float64) {
	m.limit = newLimit
}

// Allow checks whether the given key is currently allowed to spend more using Money precision.
// It returns allowed, windowEnd, spentSoFar, limit.
func (m *ManagerMoney) Allow(key string) (bool, time.Time, Money, Money) {
	lim := m.limit
	if key == "" { // anonymous bypasses but not tracked
		return true, time.Time{}, Money(0), lim
	}
	if lim.IsNegative() { // disabled limiter
		return true, time.Time{}, Money(0), lim
	}
	if lim.IsZero() { // immediate block for any spend
		return false, time.Now().Add(time.Hour), Money(0), lim
	}
	kw := m.getKWMoney(key)
	kw.mu.Lock()
	defer kw.mu.Unlock()
	now := time.Now()
	if now.Sub(kw.windowStart) >= time.Hour { // reset window
		kw.windowStart = now
		kw.spent = Money(0)
	}
	windowEnd := kw.windowStart.Add(time.Hour)
	return kw.spent.LessThan(lim), windowEnd, kw.spent, lim
}

// AddCost adds the provided Money spend to a key's current window.
func (m *ManagerMoney) AddCost(key string, delta Money) {
	if delta.IsZero() || delta.IsNegative() || key == "" {
		return
	}
	if m.limit.IsNegative() {
		return
	} // disabled
	if m.limit.IsZero() {
		return
	} // zero allowance blocked earlier
	kw := m.getKWMoney(key)
	kw.mu.Lock()
	now := time.Now()
	if now.Sub(kw.windowStart) >= time.Hour { // window expired
		kw.windowStart = now
		kw.spent = Money(0)
	}
	kw.spent = kw.spent.Add(delta)
	kw.mu.Unlock()
}

// AddCostFromUSD adds the provided USD spend converted to Money to a key's current window.
func (m *ManagerMoney) AddCostFromUSD(key string, deltaUSD float64) {
	if deltaUSD <= 0 {
		return
	}
	m.AddCost(key, NewMoneyFromUSD(deltaUSD))
}

func (m *ManagerMoney) getKWMoney(key string) *keyWindowMoney {
	if v, ok := m.perKey.Load(key); ok {
		return v.(*keyWindowMoney)
	}
	kw := &keyWindowMoney{windowStart: time.Now()}
	actual, _ := m.perKey.LoadOrStore(key, kw)
	return actual.(*keyWindowMoney)
}

// UsageInfoMoney holds information about a key's current usage window using Money
type UsageInfoMoney struct {
	Key         string    `json:"key"`
	Spent       Money     `json:"spent"`
	Limit       Money     `json:"limit"`
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	Remaining   Money     `json:"remaining"`
	Allowed     bool      `json:"allowed"`
}

// GetUsage returns usage information for a specific key using Money precision
func (m *ManagerMoney) GetUsage(key string) UsageInfoMoney {
	lim := m.limit
	now := time.Now()

	if key == "" {
		allowed := !lim.IsZero()
		remaining := lim
		if lim.IsNegative() {
			remaining = Money(-1) // unlimited
		}
		return UsageInfoMoney{
			Key:         "anonymous",
			Spent:       Money(0),
			Limit:       lim,
			WindowStart: now,
			WindowEnd:   now.Add(time.Hour),
			Remaining:   remaining,
			Allowed:     allowed,
		}
	}

	if lim.IsNegative() {
		return UsageInfoMoney{
			Key:         key,
			Spent:       Money(0),
			Limit:       lim,
			WindowStart: now,
			WindowEnd:   now.Add(time.Hour),
			Remaining:   Money(-1), // unlimited
			Allowed:     true,
		}
	}

	// Handle zero limit case
	if lim.IsZero() {
		return UsageInfoMoney{
			Key:         key,
			Spent:       Money(0),
			Limit:       lim,
			WindowStart: now,
			WindowEnd:   now.Add(time.Hour),
			Remaining:   Money(0),
			Allowed:     false,
		}
	}

	kw := m.getKWMoney(key)
	kw.mu.Lock()
	if now.Sub(kw.windowStart) >= time.Hour {
		kw.windowStart = now
		kw.spent = Money(0)
	}
	windowEnd := kw.windowStart.Add(time.Hour)
	spent := kw.spent
	kw.mu.Unlock()

	remaining := Money(0)
	if lim.GreaterThan(spent) {
		remaining = Money(int64(lim) - int64(spent))
	}

	return UsageInfoMoney{
		Key:         key,
		Spent:       spent,
		Limit:       lim,
		WindowStart: kw.windowStart,
		WindowEnd:   windowEnd,
		Remaining:   remaining,
		Allowed:     spent.LessThan(lim),
	}
}

// GetAllUsage returns usage information for all tracked keys using Money precision
func (m *ManagerMoney) GetAllUsage() []UsageInfoMoney {
	var usage []UsageInfoMoney
	now := time.Now()

	m.perKey.Range(func(key, value interface{}) bool {
		keyStr := key.(string)
		kw := value.(*keyWindowMoney)

		kw.mu.Lock()
		if now.Sub(kw.windowStart) >= time.Hour {
			kw.windowStart = now
			kw.spent = Money(0)
		}
		windowEnd := kw.windowStart.Add(time.Hour)
		spent := kw.spent
		kw.mu.Unlock()

		remaining := Money(0)
		if m.limit.GreaterThan(spent) {
			remaining = Money(int64(m.limit) - int64(spent))
		}

		usage = append(usage, UsageInfoMoney{
			Key:         keyStr,
			Spent:       spent,
			Limit:       m.limit,
			WindowStart: kw.windowStart,
			WindowEnd:   windowEnd,
			Remaining:   remaining,
			Allowed:     m.limit.IsNegative() || spent.LessThan(m.limit),
		})
		return true
	})

	return usage
}

// UpdateLimit updates the spending limit for the manager using Money
func (m *ManagerMoney) UpdateLimit(newLimit Money) {
	m.limit = newLimit
}

// UpdateLimitFromUSD updates the spending limit for the manager from USD
func (m *ManagerMoney) UpdateLimitFromUSD(newLimitUSD float64) {
	if newLimitUSD < 0 {
		m.limit = Money(-1) // disabled
	} else {
		m.limit = NewMoneyFromUSD(newLimitUSD)
	}
}

// ToLegacy converts UsageInfoMoney to UsageInfo for backward compatibility
func (ui UsageInfoMoney) ToLegacy() UsageInfo {
	remainingUSD := ui.Remaining.ToUSD()
	if ui.Remaining.IsNegative() {
		remainingUSD = -1
	}

	return UsageInfo{
		Key:         ui.Key,
		SpentUSD:    ui.Spent.ToUSD(),
		LimitUSD:    ui.Limit.ToUSD(),
		WindowStart: ui.WindowStart,
		WindowEnd:   ui.WindowEnd,
		Remaining:   remainingUSD,
		Allowed:     ui.Allowed,
	}
}
