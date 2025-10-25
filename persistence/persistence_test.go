package persistence

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/goverture/goxy/pricing"
	_ "modernc.org/sqlite"
)

func TestPersistentLimitManager_Basic(t *testing.T) {
	// Create temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create persistent manager with $1.00 limit
	mgr, err := NewPersistentLimitManager(1.00, dbPath)
	if err != nil {
		t.Fatalf("Failed to create persistent manager: %v", err)
	}
	defer mgr.Close()

	// Test basic operations
	testKey := "test-key-1"
	cost := pricing.NewMoneyFromUSD(0.25)

	// Check initial state
	allowed, _, spent, limit := mgr.Allow(testKey)
	if !allowed {
		t.Error("Expected key to be allowed initially")
	}
	if !spent.IsZero() {
		t.Error("Expected zero initial spend")
	}
	expectedLimit := pricing.NewMoneyFromUSD(1.00)
	if limit != expectedLimit {
		t.Errorf("Expected limit %v, got %v", expectedLimit, limit)
	}

	// Add some cost
	mgr.AddCost(testKey, cost)

	// Check updated state
	allowed, _, spent, _ = mgr.Allow(testKey)
	if !allowed {
		t.Error("Expected key to still be allowed after small cost")
	}
	if spent != cost {
		t.Errorf("Expected spent %v, got %v", cost, spent)
	}

	// Add more cost to exceed limit
	largeCost := pricing.NewMoneyFromUSD(0.80)
	mgr.AddCost(testKey, largeCost)

	// Should now be blocked
	allowed, _, spent, _ = mgr.Allow(testKey)
	if allowed {
		t.Error("Expected key to be blocked after exceeding limit")
	}
	expectedSpent := cost.Add(largeCost)
	if spent != expectedSpent {
		t.Errorf("Expected total spent %v, got %v", expectedSpent, spent)
	}
}

func TestPersistentLimitManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "persistence_test.db")

	testKey := "persistent-key"
	cost1 := pricing.NewMoneyFromUSD(0.30)
	cost2 := pricing.NewMoneyFromUSD(0.20)

	// Create first manager and add some costs
	mgr1, err := NewPersistentLimitManager(1.00, dbPath)
	if err != nil {
		t.Fatalf("Failed to create first manager: %v", err)
	}

	mgr1.AddCost(testKey, cost1)
	mgr1.AddCost(testKey, cost2)

	// Data should be automatically saved on AddCost, so just close
	mgr1.Close()

	// Create second manager (should load persisted data)
	mgr2, err := NewPersistentLimitManager(1.00, dbPath)
	if err != nil {
		t.Fatalf("Failed to create second manager: %v", err)
	}
	defer mgr2.Close()

	// Check that data was restored
	_, _, spent, _ := mgr2.Allow(testKey)
	expectedSpent := cost1.Add(cost2)
	if spent != expectedSpent {
		t.Errorf("Expected restored spent %v, got %v", expectedSpent, spent)
	}

	// Verify usage info
	usage := mgr2.GetUsage(testKey)
	if usage.Spent != expectedSpent {
		t.Errorf("Expected usage spent %v, got %v", expectedSpent, usage.Spent)
	}
	if usage.Key != testKey {
		t.Errorf("Expected usage key %s, got %s", testKey, usage.Key)
	}
}

func TestPersistentLimitManager_ExpiredWindows(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "expired_test.db")

	testKey := "expired-key"
	cost := pricing.NewMoneyFromUSD(0.50)

	// Create manager and add cost
	mgr1, err := NewPersistentLimitManager(1.00, dbPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	mgr1.AddCost(testKey, cost)

	// Data is automatically saved, just close
	mgr1.Close()

	// Directly access database to update timestamp to be old (simulating expired data)
	// This simulates data from more than 1 hour ago
	oldTimestamp := time.Now().Add(-2 * time.Hour).Unix()

	// Open database directly to modify the timestamp
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Update the timestamp to be old
	_, err = db.Exec("UPDATE usage_tracking SET window_start = ? WHERE key = ?", oldTimestamp, testKey)
	if err != nil {
		db.Close()
		t.Fatalf("Failed to update timestamp: %v", err)
	}
	db.Close()

	// Create new manager - should not load expired data
	mgr2, err := NewPersistentLimitManager(1.00, dbPath)
	if err != nil {
		t.Fatalf("Failed to create second manager: %v", err)
	}
	defer mgr2.Close()

	// Check that expired data was not loaded
	_, _, spent, _ := mgr2.Allow(testKey)
	if !spent.IsZero() {
		t.Errorf("Expected zero spent for expired window, got %v", spent)
	}
}

func TestPersistentLimitManager_MultipleKeys(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "multiple_keys_test.db")

	mgr, err := NewPersistentLimitManager(1.00, dbPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	// Test multiple keys
	keys := []string{"key1", "key2", "key3"}
	costs := []pricing.Money{
		pricing.NewMoneyFromUSD(0.10),
		pricing.NewMoneyFromUSD(0.25),
		pricing.NewMoneyFromUSD(0.50),
	}

	// Add costs for each key
	for i, key := range keys {
		mgr.AddCost(key, costs[i])
	}

	// Verify each key's usage
	for i, key := range keys {
		_, _, spent, _ := mgr.Allow(key)
		if spent != costs[i] {
			t.Errorf("Key %s: expected spent %v, got %v", key, costs[i], spent)
		}
	}

	// Test GetAllUsage
	allUsage := mgr.GetAllUsage()
	if len(allUsage) != len(keys) {
		t.Errorf("Expected %d usage records, got %d", len(keys), len(allUsage))
	}

	// Verify all keys are present
	usageMap := make(map[string]pricing.Money)
	for _, usage := range allUsage {
		usageMap[usage.Key] = usage.Spent
	}

	for i, key := range keys {
		if spent, exists := usageMap[key]; !exists {
			t.Errorf("Key %s not found in usage data", key)
		} else if spent != costs[i] {
			t.Errorf("Key %s: expected spent %v, got %v", key, costs[i], spent)
		}
	}
}

func TestPersistentLimitManager_DatabaseOperations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "db_ops_test.db")

	mgr, err := NewPersistentLimitManager(1.00, dbPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Test schema initialization by querying the table
	rows, err := mgr.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='usage_tracking'")
	if err != nil {
		t.Fatalf("Failed to query schema: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Error("usage_tracking table was not created")
	}

	// Test basic operations - let background sync handle the saving
	testKey := "db-test-key"
	mgr.AddCost(testKey, pricing.NewMoneyFromUSD(0.10))

	// Wait for background sync to save the data
	time.Sleep(100 * time.Millisecond)

	// Verify the manager works correctly
	usage := mgr.GetUsage(testKey)
	if usage.Spent.IsZero() {
		t.Error("Expected non-zero usage after AddCost")
	}
}

func TestPersistentLimitManager_CleanupOldRecords(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "cleanup_test.db")

	mgr, err := NewPersistentLimitManager(1.00, dbPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	// Add current data
	currentKey := "current-key"
	mgr.AddCost(currentKey, pricing.NewMoneyFromUSD(0.10))

	// Close manager to avoid conflicts (data is automatically saved)
	mgr.Close()

	// Add old record directly to database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	oldTimestamp := time.Now().Add(-25 * time.Hour).Unix()
	_, err = db.Exec("INSERT INTO usage_tracking (key, window_start, spent, last_updated) VALUES (?, ?, ?, ?)",
		"old-key", oldTimestamp, 1000, time.Now().Unix())
	if err != nil {
		t.Fatalf("Failed to insert old record: %v", err)
	}
	db.Close()

	// Create new manager (should trigger cleanup)
	mgr2, err := NewPersistentLimitManager(1.00, dbPath)
	if err != nil {
		t.Fatalf("Failed to create second manager: %v", err)
	}
	defer mgr2.Close()

	// Manually run cleanup
	if err := mgr2.cleanupOldRecords(); err != nil {
		t.Fatalf("Failed to cleanup: %v", err)
	}

	// Verify old record was removed
	var count int
	err = mgr2.db.QueryRow("SELECT COUNT(*) FROM usage_tracking WHERE key = ?", "old-key").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query old record count: %v", err)
	}
	if count != 0 {
		t.Error("Old record was not cleaned up")
	}

	// Verify current record still exists
	err = mgr2.db.QueryRow("SELECT COUNT(*) FROM usage_tracking WHERE key = ?", currentKey).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query current record count: %v", err)
	}
	if count != 1 {
		t.Error("Current record was incorrectly cleaned up")
	}
}

func TestPersistentLimitManager_HighValueImmediateSave(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "high_value_test.db")

	mgr, err := NewPersistentLimitManager(10.00, dbPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	testKey := "high-value-key"
	highCost := pricing.NewMoneyFromUSD(5.00) // > $0.01 threshold

	// Add high-value cost (should trigger immediate save)
	mgr.AddCost(testKey, highCost)

	// Give it a moment for the goroutine to complete
	time.Sleep(100 * time.Millisecond)

	// Verify data was saved immediately
	var count int
	err = mgr.db.QueryRow("SELECT COUNT(*) FROM usage_tracking WHERE key = ?", testKey).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query count: %v", err)
	}
	if count != 1 {
		t.Error("High-value transaction was not immediately saved")
	}

	// Verify the amount is correct
	var savedAmount int64
	err = mgr.db.QueryRow("SELECT spent FROM usage_tracking WHERE key = ?", testKey).Scan(&savedAmount)
	if err != nil {
		t.Fatalf("Failed to query saved amount: %v", err)
	}
	if pricing.Money(savedAmount) != highCost {
		t.Errorf("Expected saved amount %v, got %v", highCost, pricing.Money(savedAmount))
	}
}

func TestPersistentLimitManager_UpdateLimit(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "update_limit_test.db")

	mgr, err := NewPersistentLimitManager(1.00, dbPath)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	testKey := "limit-test-key"
	cost := pricing.NewMoneyFromUSD(0.80)

	// Add cost close to limit
	mgr.AddCost(testKey, cost)

	// Should be allowed
	allowed, _, _, _ := mgr.Allow(testKey)
	if !allowed {
		t.Error("Expected key to be allowed before limit update")
	}

	// Update limit to lower value
	mgr.UpdateLimitFromUSD(0.50)

	// Should now be blocked
	allowed, _, _, _ = mgr.Allow(testKey)
	if allowed {
		t.Error("Expected key to be blocked after limit update")
	}

	// Update limit to higher value
	mgr.UpdateLimitFromUSD(2.00)

	// Should be allowed again
	allowed, _, _, _ = mgr.Allow(testKey)
	if !allowed {
		t.Error("Expected key to be allowed after increasing limit")
	}
}

// Benchmark tests
func BenchmarkPersistentLimitManager_AddCost(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	// Use quiet mode to suppress logging during benchmarks
	mgr, err := NewPersistentLimitManager(100.00, dbPath)
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	cost := pricing.NewMoneyFromUSD(0.001) // Small cost to avoid immediate saves
	key := "bench-key"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mgr.AddCost(key, cost)
		}
	})
}

func BenchmarkPersistentLimitManager_Allow(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench_allow.db")

	// Use quiet mode to suppress logging during benchmarks
	mgr, err := NewPersistentLimitManager(100.00, dbPath)
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	key := "bench-key"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mgr.Allow(key)
		}
	})
}

func BenchmarkPersistentLimitManager_MultipleKeys(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench_multi.db")

	mgr, err := NewPersistentLimitManager(100.00, dbPath)
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Close()

	cost := pricing.NewMoneyFromUSD(0.001)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%100) // Rotate through 100 different keys
			mgr.AddCost(key, cost)
			mgr.Allow(key)
			i++
		}
	})
}
