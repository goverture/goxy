package limit

import (
	"sync"
	"time"
)

// manager maintains per-key rolling (hour-bucket) spend tracking.
// A simple fixed 1h window that resets when an hour has elapsed since first spend
// in the window. Good enough for coarse limiting; not a precise sliding window.
type Manager struct {
	limit  float64  // USD per hour (<=0 disables)
	perKey sync.Map // map[string]*keyWindow
}

type keyWindow struct {
	mu          sync.Mutex
	windowStart time.Time
	spentUSD    float64
}

// NewManager creates a spend limit manager with the given per-hour limit (USD).
// Pass <=0 to disable limiting.
func NewManager(limit float64) *Manager { return &Manager{limit: limit} }

// Allow checks whether the given key is currently allowed to spend more.
// It returns allowed, windowEnd, spentSoFar, limit.
func (m *Manager) Allow(key string) (bool, time.Time, float64, float64) {
	lim := m.limit
	if lim <= 0 || key == "" { // disabled or anonymous key
		return true, time.Time{}, 0, lim
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
	if delta <= 0 || key == "" || m.limit <= 0 {
		return
	}
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
