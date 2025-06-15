package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
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
        id VARCHAR(255) PRIMARY KEY,
        original_url TEXT NOT NULL,
        user_id VARCHAR(255) NOT NULL,
        created_at TIMESTAMP DEFAULT NOW()
    );
    CREATE UNIQUE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);
    CREATE INDEX IF NOT EXISTS idx_user_id ON urls(user_id);`

	_, err := r.db.ExecContext(ctx, query)
	return err
}

func (r *PostgresRepository) SaveURL(ctx context.Context, id, originalURL, userID string) error {
	query := `INSERT INTO urls (id, original_url, user_id) VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(ctx, query, id, originalURL, userID)

	if err != nil {
		fmt.Printf("SaveURL error: %v\n", err) // Логирование ошибки

		if pgErr, ok := err.(*pgconn.PgError); ok {
			fmt.Printf("PG Error Details: Code=%s, Message=%s\n", pgErr.Code, pgErr.Message)
			if pgErr.Code == pgerrcode.UniqueViolation {
				return storage.ErrURLConflict
			}
		}
		return err
	}
	fmt.Printf("Successfully saved URL: %s\n", originalURL)
	return nil
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

type Tx interface {
	SaveURL(ctx context.Context, id, originalURL, userID string) error
	Commit() error
	Rollback() error
}

type pgTx struct {
	tx *sql.Tx
}

func (t *pgTx) SaveURL(ctx context.Context, id, originalURL, userID string) error {
	query := `INSERT INTO urls (id, original_url, user_id) VALUES ($1, $2, $3)`
	_, err := t.tx.ExecContext(ctx, query, id, originalURL, userID)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == pgerrcode.UniqueViolation {
			return storage.ErrURLConflict
		}
		return err
	}
	return nil
}

func (t *pgTx) Commit() error {
	return t.tx.Commit()
}

func (t *pgTx) Rollback() error {
	return t.tx.Rollback()
}

func (r *PostgresRepository) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &pgTx{tx: tx}, nil
}

func (r *PostgresRepository) FindExistingURL(ctx context.Context, originalURL string) (string, error) {
	var id string
	query := `SELECT id FROM urls WHERE original_url = $1`
	err := r.db.QueryRowContext(ctx, query, originalURL).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

func (r *PostgresRepository) GetUserURLs(ctx context.Context, userID string) (map[string]string, error) {
	query := `SELECT id, original_url FROM urls WHERE user_id = $1`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var id, originalURL string
		if err := rows.Scan(&id, &originalURL); err != nil {
			return nil, err
		}
		result[id] = originalURL
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
