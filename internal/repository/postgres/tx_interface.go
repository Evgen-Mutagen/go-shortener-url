package postgres

import "context"

// Tx представляет интерфейс для работы с транзакциями
type Tx interface {
	// SaveURL сохраняет URL в рамках транзакции
	SaveURL(ctx context.Context, id, originalURL, userID string) error
	// Commit подтверждает транзакцию
	Commit() error
	// Rollback откатывает транзакцию
	Rollback() error
	// MarkURLsAsDeleted помечает URL как удаленные
	MarkURLsAsDeleted(ctx context.Context, userID string, urlIDs []string) error
}
