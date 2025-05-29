package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type PostgresRepository struct {
	db *sql.DB
}

func New(dsn string) (*PostgresRepository, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Проверяем соединение с базой
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresRepository{db: db}, nil
}

func (r *PostgresRepository) Close() error {
	return r.db.Close()
}

func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *PostgresRepository) InitTable(ctx context.Context) error {
	query := `
    CREATE TABLE IF NOT EXISTS urls (
        id VARCHAR(36) PRIMARY KEY,
        original_url TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT NOW()
    )`

	_, err := r.db.ExecContext(ctx, query)
	return err
}

func (r *PostgresRepository) SaveURL(ctx context.Context, id, originalURL string) error {
	query := `INSERT INTO urls (id, original_url) VALUES ($1, $2)`
	_, err := r.db.ExecContext(ctx, query, id, originalURL)
	return err
}

func (r *PostgresRepository) GetURL(ctx context.Context, id string) (string, error) {
	var originalURL string
	query := `SELECT original_url FROM urls WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&originalURL)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return originalURL, err
}
