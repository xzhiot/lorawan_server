package storage

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// PostgresStore implements Store interface for PostgreSQL
type PostgresStore struct {
	db *sql.DB
	tx *sql.Tx
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	return s.db.Close()
}

// BeginTx starts a new transaction
func (s *PostgresStore) BeginTx(ctx context.Context) (Store, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &PostgresStore{db: s.db, tx: tx}, nil
}

// Commit commits the transaction
func (s *PostgresStore) Commit() error {
	if s.tx == nil {
		return nil
	}
	return s.tx.Commit()
}

// Rollback rolls back the transaction
func (s *PostgresStore) Rollback() error {
	if s.tx == nil {
		return nil
	}
	return s.tx.Rollback()
}

// getDB returns tx if in transaction, otherwise db
func (s *PostgresStore) getDB() interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
} {
	if s.tx != nil {
		return s.tx
	}
	return s.db
}
