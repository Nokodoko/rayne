package accounts

import (
	"database/sql"
	"fmt"
	"time"
)

// AccountReader defines read operations for accounts
type AccountReader interface {
	GetByID(id int64) (*Account, error)
	GetByOrgID(orgID int64) (*Account, error)
	GetByName(name string) (*Account, error)
	GetDefault() (*Account, error)
	GetAll() ([]Account, error)
}

// AccountWriter defines write operations for accounts
type AccountWriter interface {
	Create(account Account) (*Account, error)
	Update(id int64, account Account) (*Account, error)
	Delete(id int64) error
	SetDefault(id int64) error
}

// Storage implements both AccountReader and AccountWriter
type Storage struct {
	db *sql.DB
}

// NewStorage creates a new account storage instance
func NewStorage(db *sql.DB) *Storage {
	return &Storage{db: db}
}

// InitTables creates the necessary database tables for accounts
func (s *Storage) InitTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS datadog_accounts (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) UNIQUE NOT NULL,
		org_id BIGINT UNIQUE,
		org_name VARCHAR(255),
		api_key VARCHAR(255) NOT NULL,
		app_key VARCHAR(255) NOT NULL,
		base_url VARCHAR(255) DEFAULT 'https://api.ddog-gov.com',
		is_default BOOLEAN DEFAULT false,
		active BOOLEAN DEFAULT true,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	CREATE INDEX IF NOT EXISTS idx_datadog_accounts_org_id ON datadog_accounts(org_id);
	CREATE INDEX IF NOT EXISTS idx_datadog_accounts_name ON datadog_accounts(name);
	`

	_, err := s.db.Exec(query)
	return err
}

// GetByID retrieves an account by its ID
func (s *Storage) GetByID(id int64) (*Account, error) {
	query := `
	SELECT id, name, org_id, org_name, api_key, app_key, base_url,
		   is_default, active, created_at, updated_at
	FROM datadog_accounts WHERE id = $1`

	account := &Account{}
	var orgID sql.NullInt64
	var orgName sql.NullString

	err := s.db.QueryRow(query, id).Scan(
		&account.ID, &account.Name, &orgID, &orgName,
		&account.APIKey, &account.AppKey, &account.BaseURL,
		&account.IsDefault, &account.Active, &account.CreatedAt, &account.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get account by id: %w", err)
	}

	if orgID.Valid {
		account.OrgID = orgID.Int64
	}
	if orgName.Valid {
		account.OrgName = orgName.String
	}

	return account, nil
}

// GetByOrgID retrieves an account by Datadog org_id
func (s *Storage) GetByOrgID(orgID int64) (*Account, error) {
	query := `
	SELECT id, name, org_id, org_name, api_key, app_key, base_url,
		   is_default, active, created_at, updated_at
	FROM datadog_accounts WHERE org_id = $1 AND active = true`

	account := &Account{}
	var dbOrgID sql.NullInt64
	var dbOrgName sql.NullString

	err := s.db.QueryRow(query, orgID).Scan(
		&account.ID, &account.Name, &dbOrgID, &dbOrgName,
		&account.APIKey, &account.AppKey, &account.BaseURL,
		&account.IsDefault, &account.Active, &account.CreatedAt, &account.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get account by org_id: %w", err)
	}

	if dbOrgID.Valid {
		account.OrgID = dbOrgID.Int64
	}
	if dbOrgName.Valid {
		account.OrgName = dbOrgName.String
	}

	return account, nil
}

// GetByName retrieves an account by name
func (s *Storage) GetByName(name string) (*Account, error) {
	query := `
	SELECT id, name, org_id, org_name, api_key, app_key, base_url,
		   is_default, active, created_at, updated_at
	FROM datadog_accounts WHERE name = $1`

	account := &Account{}
	var orgID sql.NullInt64
	var orgName sql.NullString

	err := s.db.QueryRow(query, name).Scan(
		&account.ID, &account.Name, &orgID, &orgName,
		&account.APIKey, &account.AppKey, &account.BaseURL,
		&account.IsDefault, &account.Active, &account.CreatedAt, &account.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get account by name: %w", err)
	}

	if orgID.Valid {
		account.OrgID = orgID.Int64
	}
	if orgName.Valid {
		account.OrgName = orgName.String
	}

	return account, nil
}

// GetDefault retrieves the default account
func (s *Storage) GetDefault() (*Account, error) {
	query := `
	SELECT id, name, org_id, org_name, api_key, app_key, base_url,
		   is_default, active, created_at, updated_at
	FROM datadog_accounts WHERE is_default = true AND active = true
	LIMIT 1`

	account := &Account{}
	var orgID sql.NullInt64
	var orgName sql.NullString

	err := s.db.QueryRow(query).Scan(
		&account.ID, &account.Name, &orgID, &orgName,
		&account.APIKey, &account.AppKey, &account.BaseURL,
		&account.IsDefault, &account.Active, &account.CreatedAt, &account.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get default account: %w", err)
	}

	if orgID.Valid {
		account.OrgID = orgID.Int64
	}
	if orgName.Valid {
		account.OrgName = orgName.String
	}

	return account, nil
}

// GetAll retrieves all accounts
func (s *Storage) GetAll() ([]Account, error) {
	query := `
	SELECT id, name, org_id, org_name, api_key, app_key, base_url,
		   is_default, active, created_at, updated_at
	FROM datadog_accounts
	ORDER BY is_default DESC, name ASC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("get all accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		account := Account{}
		var orgID sql.NullInt64
		var orgName sql.NullString

		err := rows.Scan(
			&account.ID, &account.Name, &orgID, &orgName,
			&account.APIKey, &account.AppKey, &account.BaseURL,
			&account.IsDefault, &account.Active, &account.CreatedAt, &account.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}

		if orgID.Valid {
			account.OrgID = orgID.Int64
		}
		if orgName.Valid {
			account.OrgName = orgName.String
		}

		accounts = append(accounts, account)
	}

	return accounts, nil
}

// Create creates a new account
func (s *Storage) Create(account Account) (*Account, error) {
	// Set defaults
	if account.BaseURL == "" {
		account.BaseURL = BaseURLGov
	}

	query := `
	INSERT INTO datadog_accounts (name, org_id, org_name, api_key, app_key, base_url, is_default, active)
	VALUES ($1, NULLIF($2, 0), NULLIF($3, ''), $4, $5, $6, $7, $8)
	RETURNING id, created_at, updated_at`

	err := s.db.QueryRow(
		query,
		account.Name, account.OrgID, account.OrgName,
		account.APIKey, account.AppKey, account.BaseURL,
		account.IsDefault, account.Active,
	).Scan(&account.ID, &account.CreatedAt, &account.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("create account: %w", err)
	}

	return &account, nil
}

// Update updates an existing account
func (s *Storage) Update(id int64, account Account) (*Account, error) {
	query := `
	UPDATE datadog_accounts
	SET org_id = NULLIF($1, 0),
		org_name = NULLIF($2, ''),
		api_key = $3,
		app_key = $4,
		base_url = $5,
		active = $6,
		updated_at = NOW()
	WHERE id = $7
	RETURNING id, name, org_id, org_name, api_key, app_key, base_url,
			  is_default, active, created_at, updated_at`

	updated := &Account{}
	var orgID sql.NullInt64
	var orgName sql.NullString

	err := s.db.QueryRow(
		query,
		account.OrgID, account.OrgName, account.APIKey, account.AppKey,
		account.BaseURL, account.Active, id,
	).Scan(
		&updated.ID, &updated.Name, &orgID, &orgName,
		&updated.APIKey, &updated.AppKey, &updated.BaseURL,
		&updated.IsDefault, &updated.Active, &updated.CreatedAt, &updated.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update account: %w", err)
	}

	if orgID.Valid {
		updated.OrgID = orgID.Int64
	}
	if orgName.Valid {
		updated.OrgName = orgName.String
	}

	return updated, nil
}

// Delete deletes an account by ID
func (s *Storage) Delete(id int64) error {
	result, err := s.db.Exec(`DELETE FROM datadog_accounts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return ErrAccountNotFound
	}

	return nil
}

// SetDefault sets an account as the default (and unsets any existing default)
func (s *Storage) SetDefault(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Unset current default
	_, err = tx.Exec(`UPDATE datadog_accounts SET is_default = false WHERE is_default = true`)
	if err != nil {
		return fmt.Errorf("unset current default: %w", err)
	}

	// Set new default
	result, err := tx.Exec(`UPDATE datadog_accounts SET is_default = true, updated_at = $1 WHERE id = $2`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("set new default: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rows == 0 {
		return ErrAccountNotFound
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// Count returns the number of accounts
func (s *Storage) Count() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM datadog_accounts`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count accounts: %w", err)
	}
	return count, nil
}
