package accounts

import (
	"fmt"
	"log"
	"os"
	"sync"
)

// AccountManager provides cached account lookups with thread-safe access
type AccountManager struct {
	storage     *Storage
	cache       map[int64]*Account  // orgID -> Account
	nameCache   map[string]*Account // name -> Account
	defaultAcct *Account
	mu          sync.RWMutex
}

// NewAccountManager creates a new account manager
func NewAccountManager(storage *Storage) *AccountManager {
	return &AccountManager{
		storage:   storage,
		cache:     make(map[int64]*Account),
		nameCache: make(map[string]*Account),
	}
}

// Initialize loads accounts from database and creates default if needed
func (m *AccountManager) Initialize() error {
	// Create default account from environment variables if no accounts exist
	count, err := m.storage.Count()
	if err != nil {
		return fmt.Errorf("count accounts: %w", err)
	}

	if count == 0 {
		apiKey := os.Getenv("DD_API_KEY")
		appKey := os.Getenv("DD_APP_KEY")

		if apiKey != "" && appKey != "" {
			log.Println("[ACCOUNTS] No accounts found, creating default account from environment variables")

			account := Account{
				Name:      "default",
				APIKey:    apiKey,
				AppKey:    appKey,
				BaseURL:   BaseURLGov,
				IsDefault: true,
				Active:    true,
			}

			if _, err := m.storage.Create(account); err != nil {
				return fmt.Errorf("create default account: %w", err)
			}
		} else {
			log.Println("[ACCOUNTS] Warning: No DD_API_KEY/DD_APP_KEY set and no accounts in database")
		}
	}

	// Refresh cache from database
	if err := m.Refresh(); err != nil {
		return fmt.Errorf("refresh cache: %w", err)
	}

	log.Printf("[ACCOUNTS] Loaded %d accounts into cache", len(m.nameCache))
	return nil
}

// GetByOrgID returns cached account by OrgID (RLock for concurrent reads)
func (m *AccountManager) GetByOrgID(orgID int64) (*Account, error) {
	m.mu.RLock()
	acct, ok := m.cache[orgID]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrAccountNotFound
	}
	return acct, nil
}

// GetByName returns cached account by name
func (m *AccountManager) GetByName(name string) (*Account, error) {
	m.mu.RLock()
	acct, ok := m.nameCache[name]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrAccountNotFound
	}
	return acct, nil
}

// GetByID returns account by ID (reads from storage, not cache)
func (m *AccountManager) GetByID(id int64) (*Account, error) {
	return m.storage.GetByID(id)
}

// GetDefault returns the default account
func (m *AccountManager) GetDefault() *Account {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.defaultAcct
}

// ResolveAccount finds best matching account or returns default
func (m *AccountManager) ResolveAccount(orgID int64, accountName string) *Account {
	// Try explicit name first
	if accountName != "" {
		if acct, err := m.GetByName(accountName); err == nil {
			return acct
		}
	}

	// Try OrgID lookup
	if orgID > 0 {
		if acct, err := m.GetByOrgID(orgID); err == nil {
			return acct
		}
	}

	// Fall back to default
	return m.GetDefault()
}

// Refresh reloads all accounts from database (Lock for exclusive write)
func (m *AccountManager) Refresh() error {
	accounts, err := m.storage.GetAll()
	if err != nil {
		return fmt.Errorf("refresh accounts: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear and rebuild caches
	m.cache = make(map[int64]*Account)
	m.nameCache = make(map[string]*Account)
	m.defaultAcct = nil

	for i := range accounts {
		acct := &accounts[i]
		if acct.OrgID > 0 {
			m.cache[acct.OrgID] = acct
		}
		m.nameCache[acct.Name] = acct
		if acct.IsDefault {
			m.defaultAcct = acct
		}
	}

	return nil
}

// Create creates an account and refreshes the cache
func (m *AccountManager) Create(account Account) (*Account, error) {
	created, err := m.storage.Create(account)
	if err != nil {
		return nil, err
	}

	// Refresh cache
	if err := m.Refresh(); err != nil {
		log.Printf("[ACCOUNTS] Warning: cache refresh failed after create: %v", err)
	}

	return created, nil
}

// Update updates an account and refreshes the cache
func (m *AccountManager) Update(id int64, account Account) (*Account, error) {
	updated, err := m.storage.Update(id, account)
	if err != nil {
		return nil, err
	}

	// Refresh cache
	if err := m.Refresh(); err != nil {
		log.Printf("[ACCOUNTS] Warning: cache refresh failed after update: %v", err)
	}

	return updated, nil
}

// Delete deletes an account and refreshes the cache
func (m *AccountManager) Delete(id int64) error {
	if err := m.storage.Delete(id); err != nil {
		return err
	}

	// Refresh cache
	if err := m.Refresh(); err != nil {
		log.Printf("[ACCOUNTS] Warning: cache refresh failed after delete: %v", err)
	}

	return nil
}

// SetDefault sets an account as default and refreshes the cache
func (m *AccountManager) SetDefault(id int64) error {
	if err := m.storage.SetDefault(id); err != nil {
		return err
	}

	// Refresh cache
	if err := m.Refresh(); err != nil {
		log.Printf("[ACCOUNTS] Warning: cache refresh failed after set default: %v", err)
	}

	return nil
}

// GetAll returns all accounts from storage
func (m *AccountManager) GetAll() ([]Account, error) {
	return m.storage.GetAll()
}

// Stats returns cache statistics
func (m *AccountManager) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	defaultName := ""
	if m.defaultAcct != nil {
		defaultName = m.defaultAcct.Name
	}

	return map[string]interface{}{
		"cached_by_org_id": len(m.cache),
		"cached_by_name":   len(m.nameCache),
		"default_account":  defaultName,
	}
}
