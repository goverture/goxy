package persistence

import (
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/goverture/goxy/pricing"
	_ "modernc.org/sqlite"
)

// PersistentLimitManager wraps a pricing.ManagerMoney and adds SQLite persistence
type PersistentLimitManager struct {
	*pricing.ManagerMoney
	db       *sql.DB
	mu       sync.RWMutex
	stopChan chan struct{}
	quiet    bool // If true, suppresses non-error logging
}

// UsageRecord represents a usage record in the database
type UsageRecord struct {
	Key         string
	MaskedKey   string
	WindowStart time.Time
	Spent       pricing.Money
	LastUpdated time.Time
}

// NewPersistentLimitManager creates a new persistent limit manager
func NewPersistentLimitManager(limitUSD float64, dbPath string) (*PersistentLimitManager, error) {
	return NewPersistentLimitManagerWithOptions(limitUSD, dbPath, false)
}

// NewPersistentLimitManagerQuiet creates a persistent limit manager with minimal logging
func NewPersistentLimitManagerQuiet(limitUSD float64, dbPath string) (*PersistentLimitManager, error) {
	return NewPersistentLimitManagerWithOptions(limitUSD, dbPath, true)
}

// NewPersistentLimitManagerWithOptions creates a new persistent limit manager with configuration options
func NewPersistentLimitManagerWithOptions(limitUSD float64, dbPath string, quiet bool) (*PersistentLimitManager, error) {
	// Create the underlying manager
	mgr := pricing.NewLimitManager(limitUSD)

	// Open database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	plm := &PersistentLimitManager{
		ManagerMoney: mgr,
		db:           db,
		stopChan:     make(chan struct{}),
		quiet:        quiet,
	}

	// Initialize database schema
	if err := plm.initSchema(); err != nil {
		return nil, err
	}

	// Load existing usage data
	if err := plm.loadUsageData(); err != nil {
		log.Printf("Warning: failed to load usage data: %v", err)
	}

	return plm, nil
}

// initSchema creates the necessary database tables
func (p *PersistentLimitManager) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS usage_tracking (
		key TEXT PRIMARY KEY,
		masked_key TEXT NOT NULL DEFAULT '',
		window_start INTEGER NOT NULL,
		spent INTEGER NOT NULL,
		last_updated INTEGER NOT NULL
	);
	
	CREATE INDEX IF NOT EXISTS idx_window_start ON usage_tracking(window_start);
	CREATE INDEX IF NOT EXISTS idx_last_updated ON usage_tracking(last_updated);
	`

	_, err := p.db.Exec(query)
	return err
}

// loadUsageData loads usage data from database and restores active windows
func (p *PersistentLimitManager) loadUsageData() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Hour) // Only load windows that could still be active

	query := `
	SELECT key, masked_key, window_start, spent, last_updated 
	FROM usage_tracking 
	WHERE window_start > ?
	ORDER BY key
	`

	rows, err := p.db.Query(query, cutoff.Unix())
	if err != nil {
		return err
	}
	defer rows.Close()

	loadedCount := 0
	for rows.Next() {
		var record UsageRecord
		var windowStartUnix, lastUpdatedUnix int64

		err := rows.Scan(&record.Key, &record.MaskedKey, &windowStartUnix, &record.Spent, &lastUpdatedUnix)
		if err != nil {
			log.Printf("Warning: failed to scan usage record: %v", err)
			continue
		}

		record.WindowStart = time.Unix(windowStartUnix, 0)
		record.LastUpdated = time.Unix(lastUpdatedUnix, 0)

		// Check if window is still active (within the last hour)
		if now.Sub(record.WindowStart) < time.Hour {
			// Restore this usage data by adding the cost
			p.ManagerMoney.AddCost(record.Key, record.Spent)
			loadedCount++
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	if !p.quiet {
		log.Printf("Loaded %d active usage records from database", loadedCount)
	}
	return nil
}

// cleanupOldRecords removes old usage records from the database
func (p *PersistentLimitManager) cleanupOldRecords() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Remove records older than 24 hours
	cutoff := time.Now().Add(-24 * time.Hour)

	result, err := p.db.Exec("DELETE FROM usage_tracking WHERE window_start < ?", cutoff.Unix())
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	if affected > 0 && !p.quiet {
		log.Printf("Cleaned up %d old usage records", affected)
	}

	return nil
}

// Close shuts down the persistent manager gracefully
func (p *PersistentLimitManager) Close() error {
	// Prevent multiple closes
	select {
	case <-p.stopChan:
		return nil // Already closed
	default:
		close(p.stopChan)
	}

	// Final cleanup
	if err := p.cleanupOldRecords(); err != nil {
		log.Printf("Warning: failed to cleanup old records: %v", err)
	}

	// Close database
	return p.db.Close()
}

// AddCost overrides the base AddCost and immediately saves to database
// TODO: Not a fan, consider removing me later
func (p *PersistentLimitManager) AddCost(key string, delta pricing.Money) {
	// For backward compatibility, call AddCostWithMaskedKey with empty masked key
	p.AddCostWithMaskedKey(key, "", delta)
}

// AddCostWithMaskedKey adds cost and saves to database with both hashed and masked keys
func (p *PersistentLimitManager) AddCostWithMaskedKey(key string, maskedKey string, delta pricing.Money) {
	// First update in-memory state
	p.ManagerMoney.AddCost(key, delta)

	// Skip saving for zero/negative deltas or empty keys
	if delta.IsZero() || delta.IsNegative() || key == "" {
		return
	}

	// Check if manager is still open
	select {
	case <-p.stopChan:
		return // Manager is closing/closed
	default:
	}

	// Save this specific key's usage to database immediately
	if err := p.saveKeyUsageWithMasked(key, maskedKey); err != nil {
		log.Printf("Warning: failed to save usage for key %s: %v", key, err)
	}
}

// saveKeyUsageWithMasked saves a single key's usage to database with masked key for display
func (p *PersistentLimitManager) saveKeyUsageWithMasked(key string, maskedKey string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Get current usage for this key
	usage := p.ManagerMoney.GetUsage(key)

	// Only save if there's actual spending
	if usage.Spent.IsZero() {
		return nil
	}

	// Begin transaction
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert or replace the record for this key
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO usage_tracking (key, masked_key, window_start, spent, last_updated)
		VALUES (?, ?, ?, ?, ?)
	`, key, maskedKey, usage.WindowStart.Unix(), int64(usage.Spent), time.Now().Unix())

	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetAllUsageWithMaskedKeys returns usage information with original masked keys from database
func (p *PersistentLimitManager) GetAllUsageWithMaskedKeys() []pricing.UsageInfoMoney {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Get the current in-memory usage data
	inMemoryUsage := p.ManagerMoney.GetAllUsage()

	// Create a map for quick lookup
	usageMap := make(map[string]pricing.UsageInfoMoney)
	for _, usage := range inMemoryUsage {
		usageMap[usage.Key] = usage
	}

	// Query database for masked keys
	query := `SELECT key, masked_key FROM usage_tracking WHERE masked_key != ''`
	rows, err := p.db.Query(query)
	if err != nil {
		log.Printf("Warning: failed to query masked keys: %v", err)
		return inMemoryUsage // fallback to in-memory data
	}
	defer rows.Close()

	// Build result with masked keys where available
	var result []pricing.UsageInfoMoney
	processedKeys := make(map[string]bool)

	for rows.Next() {
		var key, maskedKey string
		if err := rows.Scan(&key, &maskedKey); err != nil {
			continue
		}

		if usage, exists := usageMap[key]; exists {
			// Replace the hash key with the masked key for display
			usage.Key = maskedKey
			result = append(result, usage)
			processedKeys[key] = true
		}
	}

	// Add any remaining usage that wasn't in the database (shouldn't happen normally)
	for _, usage := range inMemoryUsage {
		if !processedKeys[usage.Key] {
			result = append(result, usage)
		}
	}

	return result
}
