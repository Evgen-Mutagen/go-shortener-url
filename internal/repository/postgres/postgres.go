package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"

	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
)

// PostgresRepository предоставляет доступ к данным URL в PostgreSQL
//
// Поля:
//   - db: подключение к базе данных
type PostgresRepository struct {
	db *sql.DB
}

// New создает новый экземпляр PostgresRepository
//
// Параметры:
//   - dsn: строка подключения к PostgreSQL (Data Source Name)
//
// Возвращает:
//   - *PostgresRepository: инициализированный репозиторий
//   - error: ошибка подключения к БД
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

// Close освобождает ресурсы подключения к БД
//
// Возвращает:
//   - error: ошибка при закрытии подключения
func (r *PostgresRepository) Close() error {
	return r.db.Close()
}

// Ping проверяет доступность базы данных
//
// Параметры:
//   - ctx: контекст выполнения
//
// Возвращает:
//   - error: ошибка при проверке подключения
func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// InitTable создает необходимые таблицы и индексы, если они не существуют
//
// Параметры:
//   - ctx: контекст выполнения
//
// Возвращает:
//   - error: ошибка при инициализации таблиц
func (r *PostgresRepository) InitTable(ctx context.Context) error {
	query := `
    CREATE TABLE IF NOT EXISTS urls (
        id VARCHAR(255) PRIMARY KEY,
        original_url TEXT NOT NULL,
        user_id VARCHAR(255) NOT NULL,
        is_deleted BOOLEAN DEFAULT FALSE,
        created_at TIMESTAMP DEFAULT NOW()
    );
    CREATE UNIQUE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);
    CREATE INDEX IF NOT EXISTS idx_user_id ON urls(user_id);`

	_, err := r.db.ExecContext(ctx, query)
	return err
}

// SaveURL сохраняет связь между коротким и оригинальным URL
//
// Параметры:
//   - ctx: контекст выполнения
//   - id: короткий идентификатор URL
//   - originalURL: оригинальный URL
//   - userID: идентификатор пользователя
//
// Возвращает:
//   - error: ErrURLConflict если URL уже существует, либо другая ошибка БД
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

// GetURL возвращает оригинальный URL по его идентификатору
//
// Параметры:
//   - ctx: контекст выполнения
//   - id: короткий идентификатор URL
//
// Возвращает:
//   - string: оригинальный URL
//   - bool: флаг удаления URL (true если URL помечен как удаленный)
//   - error: ошибка при выполнении запроса
func (r *PostgresRepository) GetURL(ctx context.Context, id string) (string, bool, error) {
	var originalURL string
	var isDeleted bool
	query := `SELECT original_url, is_deleted FROM urls WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&originalURL, &isDeleted)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return originalURL, isDeleted, err
}

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

// pgTx реализует Tx для PostgreSQL
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

// BeginTx начинает новую транзакцию
//
// Параметры:
//   - ctx: контекст выполнения
//
// Возвращает:
//   - Tx: интерфейс транзакции
//   - error: ошибка при начале транзакции
func (r *PostgresRepository) BeginTx(ctx context.Context) (Tx, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &pgTx{tx: tx}, nil
}

// FindExistingURL ищет существующий короткий URL для оригинального URL
//
// Параметры:
//   - ctx: контекст выполнения
//   - originalURL: оригинальный URL для поиска
//
// Возвращает:
//   - string: найденный короткий идентификатор (пустая строка если не найден)
//   - error: ошибка при выполнении запроса
func (r *PostgresRepository) FindExistingURL(ctx context.Context, originalURL string) (string, error) {
	var id string
	query := `SELECT id FROM urls WHERE original_url = $1`
	err := r.db.QueryRowContext(ctx, query, originalURL).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// GetUserURLs возвращает все URL пользователя
//
// Параметры:
//   - ctx: контекст выполнения
//   - userID: идентификатор пользователя
//
// Возвращает:
//   - map[string]string: карта [короткий URL]оригинальный URL
//   - error: ошибка при выполнении запроса
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

// MarkURLsAsDeleted помечает URL пользователя как удаленные
//
// Параметры:
//   - ctx: контекст выполнения
//   - userID: идентификатор пользователя
//   - urlIDs: список идентификаторов URL для удаления
//
// Возвращает:
//   - error: ошибка при выполнении запроса
func (r *PostgresRepository) MarkURLsAsDeleted(ctx context.Context, userID string, urlIDs []string) error {
	if len(urlIDs) == 0 {
		return nil
	}

	query := `UPDATE urls SET is_deleted = TRUE 
              WHERE id = ANY($1) AND user_id = $2 AND is_deleted = FALSE`

	_, err := r.db.ExecContext(ctx, query, pq.Array(urlIDs), userID)
	return err
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
