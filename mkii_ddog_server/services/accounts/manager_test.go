package accounts

import (
	"testing"
)

// mockStorage implements a minimal in-memory storage for testing
type mockStorage struct {
	accounts []Account
}

func (m *mockStorage) GetAll() ([]Account, error) {
	return m.accounts, nil
}

func (m *mockStorage) Count() (int, error) {
	return len(m.accounts), nil
}

func (m *mockStorage) GetByID(id int64) (*Account, error) {
	for i := range m.accounts {
		if m.accounts[i].ID == id {
			return &m.accounts[i], nil
		}
	}
	return nil, ErrAccountNotFound
}

func (m *mockStorage) GetByOrgID(orgID int64) (*Account, error) {
	for i := range m.accounts {
		if m.accounts[i].OrgID == orgID {
			return &m.accounts[i], nil
		}
	}
	return nil, ErrAccountNotFound
}

func (m *mockStorage) GetByName(name string) (*Account, error) {
	for i := range m.accounts {
		if m.accounts[i].Name == name {
			return &m.accounts[i], nil
		}
	}
	return nil, ErrAccountNotFound
}

func (m *mockStorage) GetDefault() (*Account, error) {
	for i := range m.accounts {
		if m.accounts[i].IsDefault {
			return &m.accounts[i], nil
		}
	}
	return nil, ErrAccountNotFound
}

func (m *mockStorage) Create(account Account) (*Account, error) {
	account.ID = int64(len(m.accounts) + 1)
	m.accounts = append(m.accounts, account)
	return &m.accounts[len(m.accounts)-1], nil
}

func (m *mockStorage) Update(id int64, account Account) (*Account, error) {
	for i := range m.accounts {
		if m.accounts[i].ID == id {
			m.accounts[i] = account
			m.accounts[i].ID = id
			return &m.accounts[i], nil
		}
	}
	return nil, ErrAccountNotFound
}

func (m *mockStorage) Delete(id int64) error {
	for i := range m.accounts {
		if m.accounts[i].ID == id {
			m.accounts = append(m.accounts[:i], m.accounts[i+1:]...)
			return nil
		}
	}
	return ErrAccountNotFound
}

func (m *mockStorage) SetDefault(id int64) error {
	found := false
	for i := range m.accounts {
		m.accounts[i].IsDefault = m.accounts[i].ID == id
		if m.accounts[i].ID == id {
			found = true
		}
	}
	if !found {
		return ErrAccountNotFound
	}
	return nil
}

// setupTestManager creates a manager with mock accounts for testing
func setupTestManager(accounts []Account) *AccountManager {
	mgr := &AccountManager{
		cache:     make(map[int64]*Account),
		nameCache: make(map[string]*Account),
	}

	for i := range accounts {
		acct := &accounts[i]
		if acct.OrgID > 0 {
			mgr.cache[acct.OrgID] = acct
		}
		mgr.nameCache[acct.Name] = acct
		if acct.IsDefault {
			mgr.defaultAcct = acct
		}
	}

	return mgr
}

func TestAccountManager_ResolveAccount(t *testing.T) {
	tests := []struct {
		name           string
		orgID          int64
		accountName    string
		accounts       []Account
		expectedName   string
		expectDefault  bool
	}{
		{
			name:        "resolve by orgID",
			orgID:       12345,
			accountName: "",
			accounts: []Account{
				{ID: 1, Name: "prod", OrgID: 12345, Active: true},
				{ID: 2, Name: "default", IsDefault: true, Active: true},
			},
			expectedName: "prod",
		},
		{
			name:        "resolve by name takes priority over orgID",
			orgID:       12345,
			accountName: "staging",
			accounts: []Account{
				{ID: 1, Name: "prod", OrgID: 12345, Active: true},
				{ID: 2, Name: "staging", OrgID: 67890, Active: true},
				{ID: 3, Name: "default", IsDefault: true, Active: true},
			},
			expectedName: "staging",
		},
		{
			name:        "fallback to default when orgID not found",
			orgID:       99999,
			accountName: "",
			accounts: []Account{
				{ID: 1, Name: "prod", OrgID: 12345, Active: true},
				{ID: 2, Name: "default", IsDefault: true, Active: true},
			},
			expectedName:  "default",
			expectDefault: true,
		},
		{
			name:        "fallback to default when name not found",
			orgID:       0,
			accountName: "nonexistent",
			accounts: []Account{
				{ID: 1, Name: "prod", OrgID: 12345, Active: true},
				{ID: 2, Name: "default", IsDefault: true, Active: true},
			},
			expectedName:  "default",
			expectDefault: true,
		},
		{
			name:        "return nil when no accounts configured",
			orgID:       12345,
			accountName: "",
			accounts:    []Account{},
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupTestManager(tt.accounts)
			got := mgr.ResolveAccount(tt.orgID, tt.accountName)

			if tt.expectedName == "" {
				if got != nil {
					t.Errorf("ResolveAccount() = %v, want nil", got.Name)
				}
				return
			}

			if got == nil {
				t.Errorf("ResolveAccount() = nil, want %s", tt.expectedName)
				return
			}

			if got.Name != tt.expectedName {
				t.Errorf("ResolveAccount() = %s, want %s", got.Name, tt.expectedName)
			}

			if tt.expectDefault && !got.IsDefault {
				t.Errorf("ResolveAccount() returned non-default account, expected default")
			}
		})
	}
}

func TestAccountManager_GetByOrgID(t *testing.T) {
	tests := []struct {
		name        string
		orgID       int64
		accounts    []Account
		expectedID  int64
		expectError bool
	}{
		{
			name:  "found by orgID",
			orgID: 12345,
			accounts: []Account{
				{ID: 1, Name: "prod", OrgID: 12345},
			},
			expectedID: 1,
		},
		{
			name:  "not found",
			orgID: 99999,
			accounts: []Account{
				{ID: 1, Name: "prod", OrgID: 12345},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupTestManager(tt.accounts)
			got, err := mgr.GetByOrgID(tt.orgID)

			if tt.expectError {
				if err == nil {
					t.Errorf("GetByOrgID() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetByOrgID() error = %v", err)
				return
			}

			if got.ID != tt.expectedID {
				t.Errorf("GetByOrgID() ID = %d, want %d", got.ID, tt.expectedID)
			}
		})
	}
}

func TestAccountManager_GetByName(t *testing.T) {
	tests := []struct {
		name        string
		accountName string
		accounts    []Account
		expectedID  int64
		expectError bool
	}{
		{
			name:        "found by name",
			accountName: "prod",
			accounts: []Account{
				{ID: 1, Name: "prod", OrgID: 12345},
				{ID: 2, Name: "staging", OrgID: 67890},
			},
			expectedID: 1,
		},
		{
			name:        "not found",
			accountName: "nonexistent",
			accounts: []Account{
				{ID: 1, Name: "prod", OrgID: 12345},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := setupTestManager(tt.accounts)
			got, err := mgr.GetByName(tt.accountName)

			if tt.expectError {
				if err == nil {
					t.Errorf("GetByName() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetByName() error = %v", err)
				return
			}

			if got.ID != tt.expectedID {
				t.Errorf("GetByName() ID = %d, want %d", got.ID, tt.expectedID)
			}
		})
	}
}

func TestCredentials_BuildURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		path     string
		expected string
	}{
		{
			name:     "gov endpoint",
			baseURL:  BaseURLGov,
			path:     PathDowntime,
			expected: "https://api.ddog-gov.com/api/v2/downtime",
		},
		{
			name:     "commercial endpoint",
			baseURL:  BaseURLCommercial,
			path:     PathHosts,
			expected: "https://api.datadoghq.com/api/v1/hosts",
		},
		{
			name:     "EU endpoint",
			baseURL:  BaseURLEU,
			path:     PathMonitors,
			expected: "https://api.datadoghq.eu/api/v1/monitor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := Credentials{BaseURL: tt.baseURL}
			got := creds.BuildURL(tt.path)

			if got != tt.expected {
				t.Errorf("BuildURL() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestAccount_ToCredentials(t *testing.T) {
	account := Account{
		ID:      1,
		Name:    "test",
		APIKey:  "test-api-key",
		AppKey:  "test-app-key",
		BaseURL: BaseURLGov,
	}

	creds := account.ToCredentials()

	if creds.APIKey != account.APIKey {
		t.Errorf("ToCredentials() APIKey = %s, want %s", creds.APIKey, account.APIKey)
	}
	if creds.AppKey != account.AppKey {
		t.Errorf("ToCredentials() AppKey = %s, want %s", creds.AppKey, account.AppKey)
	}
	if creds.BaseURL != account.BaseURL {
		t.Errorf("ToCredentials() BaseURL = %s, want %s", creds.BaseURL, account.BaseURL)
	}
}

func TestAccount_ToResponse(t *testing.T) {
	account := Account{
		ID:        1,
		Name:      "test",
		OrgID:     12345,
		OrgName:   "Test Org",
		APIKey:    "secret-api-key",
		AppKey:    "secret-app-key",
		BaseURL:   BaseURLGov,
		IsDefault: true,
		Active:    true,
	}

	resp := account.ToResponse()

	// Check that sensitive fields are not exposed
	if resp.ID != account.ID {
		t.Errorf("ToResponse() ID = %d, want %d", resp.ID, account.ID)
	}
	if resp.Name != account.Name {
		t.Errorf("ToResponse() Name = %s, want %s", resp.Name, account.Name)
	}
	if resp.OrgID != account.OrgID {
		t.Errorf("ToResponse() OrgID = %d, want %d", resp.OrgID, account.OrgID)
	}
	if resp.BaseURL != account.BaseURL {
		t.Errorf("ToResponse() BaseURL = %s, want %s", resp.BaseURL, account.BaseURL)
	}
	if resp.IsDefault != account.IsDefault {
		t.Errorf("ToResponse() IsDefault = %v, want %v", resp.IsDefault, account.IsDefault)
	}
}
