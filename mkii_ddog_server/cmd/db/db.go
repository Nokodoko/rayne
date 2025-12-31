package db

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/Nokodoko/mkii_ddog_server/cmd/config"
	_ "github.com/lib/pq"
)

// SqlStorage creates a PostgreSQL database connection using environment variables
func SqlStorage(cfg config.Config) (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.SSLMode,
	)
	log.Printf("Connecting to database at %s:%s", cfg.DBHost, cfg.DBPort)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection established")
	return db, nil
}
