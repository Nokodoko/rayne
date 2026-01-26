package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/config"
	"github.com/lib/pq"
	sqltrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/database/sql"
)

// SqlStorage creates a PostgreSQL database connection using environment variables
// with Datadog APM tracing enabled for all SQL queries
func SqlStorage(cfg config.Config) (*sql.DB, error) {
	// Register the pq driver with Datadog tracing
	sqltrace.Register("postgres", &pq.Driver{}, sqltrace.WithServiceName("rayne-db"))

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

	// Use sqltrace.Open instead of sql.Open for traced connections
	db, err := sqltrace.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool for concurrent workload
	// Prevents pool exhaustion under load while maintaining efficiency
	db.SetMaxOpenConns(25)                  // Max concurrent connections
	db.SetMaxIdleConns(5)                   // Keep some connections warm
	db.SetConnMaxLifetime(5 * time.Minute)  // Recycle connections periodically
	db.SetConnMaxIdleTime(1 * time.Minute)  // Close idle connections quickly

	log.Println("Database connection established with APM tracing and connection pooling")
	return db, nil
}
