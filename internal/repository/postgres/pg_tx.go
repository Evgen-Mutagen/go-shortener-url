package postgres

import (
	"context"
	"database/sql"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
)

type pgTx struct {
	tx *sql.Tx
}

// SaveURL сохраняет урл
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

// Commit подтверждает транзакцию
func (t *pgTx) Commit() error {
	return t.tx.Commit()
}

// Rollback откатывает транзакцию
func (t *pgTx) Rollback() error {
	return t.tx.Rollback()
}

// MarkURLsAsDeleted помечает URL пользователя как удаленные в рамках транзакции
func (t *pgTx) MarkURLsAsDeleted(ctx context.Context, userID string, urlIDs []string) error {
	if len(urlIDs) == 0 {
		return nil
	}

	query := `UPDATE urls SET is_deleted = TRUE 
              WHERE id = ANY($1) AND user_id = $2 AND is_deleted = FALSE`

	_, err := t.tx.ExecContext(ctx, query, pq.Array(urlIDs), userID)
	return err
}
