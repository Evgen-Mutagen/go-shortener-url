package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Evgen-Mutagen/go-shortener-url/internal/configs"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/repository/postgres"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/storage"
	"github.com/Evgen-Mutagen/go-shortener-url/internal/util"
)

// ShortenerService общий сервис с бизнес-логикой
type ShortenerService struct {
	cfg       *configs.Config
	storage   *storage.Storage
	generator *util.IDGenerator
	Repo      *postgres.PostgresRepository
}

// New создает новый экземпляр ShortenerService
func New(cfg *configs.Config, storage *storage.Storage, repo *postgres.PostgresRepository) *ShortenerService {
	return &ShortenerService{
		cfg:       cfg,
		storage:   storage,
		generator: util.NewIDGenerator(),
		Repo:      repo,
	}
}

// ShortenURLResult результат сокращения URL
type ShortenURLResult struct {
	ShortURL string
	Error    string
	Status   int
}

// GetURLResult результат получения URL
type GetURLResult struct {
	OriginalURL string
	IsDeleted   bool
	Error       string
	Status      int
}

// BatchItem элемент пакетного запроса
type BatchItem struct {
	CorrelationID string
	OriginalURL   string
}

// BatchResult результат пакетного запроса
type BatchResult struct {
	Items  []BatchResponseItem
	Error  string
	Status int
}

// BatchResponseItem элемент ответа на пакетный запрос
type BatchResponseItem struct {
	CorrelationID string
	ShortURL      string
}

// GetUserURLsResult результат получения URL пользователя
type GetUserURLsResult struct {
	URLs   []UserURL
	Error  string
	Status int
}

// UserURL URL пользователя
type UserURL struct {
	ShortURL    string
	OriginalURL string
}

// DeleteUserURLsResult результат удаления URL
type DeleteUserURLsResult struct {
	Error  string
	Status int
}

// PingResult результат проверки доступности
type PingResult struct {
	Error  string
	Status int
}

// GetStatsResult результат получения статистики
type GetStatsResult struct {
	URLs   int
	Users  int
	Error  string
	Status int
}

// ShortenURL сокращает URL
func (s *ShortenerService) ShortenURL(ctx context.Context, url, userID string) ShortenURLResult {
	if url == "" {
		return ShortenURLResult{
			Error:  "URL cannot be empty",
			Status: 400,
		}
	}

	var id string

	if s.Repo != nil {
		// Проверяем, существует ли URL
		existingID, err := s.Repo.FindExistingURL(ctx, url)
		if err != nil {
			return ShortenURLResult{
				Error:  "Error checking URL",
				Status: 500,
			}
		}
		if existingID != "" {
			return ShortenURLResult{
				ShortURL: fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), existingID),
				Status:   409,
			}
		}

		id = s.generator.Generate()
		if err := s.Repo.SaveURL(ctx, id, url, userID); err != nil {
			// Если получили конфликт, пытаемся найти существующий URL
			existingID, err := s.Repo.FindExistingURL(ctx, url)
			if err != nil {
				return ShortenURLResult{
					Error:  "Error checking URL",
					Status: 500,
				}
			}
			return ShortenURLResult{
				ShortURL: fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), existingID),
				Status:   409,
			}
		}

		// Сохраняем в файловое хранилище для совместимости
		if err := s.storage.Save(id, url, userID); err != nil && err != storage.ErrURLConflict {
			// Логируем ошибку, но не прерываем выполнение
		}
	} else {
		id = s.generator.Generate()
		if err := s.storage.Save(id, url, userID); err != nil {
			if err == storage.ErrURLConflict {
				return ShortenURLResult{
					Error:  "URL already exists",
					Status: 409,
				}
			}
			return ShortenURLResult{
				Error:  "Error saving URL",
				Status: 500,
			}
		}
	}

	return ShortenURLResult{
		ShortURL: fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), id),
		Status:   201,
	}
}

// GetURL получает оригинальный URL по ID
func (s *ShortenerService) GetURL(ctx context.Context, id string) GetURLResult {
	if id == "" {
		return GetURLResult{
			Error:  "ID is required",
			Status: 400,
		}
	}

	var url string
	var isDeleted bool
	var exists bool

	if s.Repo != nil {
		originalURL, deleted, err := s.Repo.GetURL(ctx, id)
		if err != nil {
			return GetURLResult{
				Error:  "Error getting URL",
				Status: 500,
			}
		}
		url, exists, isDeleted = originalURL, originalURL != "", deleted
	} else {
		url, exists = s.storage.Get(id)
		isDeleted = false
	}

	if !exists {
		return GetURLResult{
			Error:  "URL not found",
			Status: 400,
		}
	}

	return GetURLResult{
		OriginalURL: url,
		IsDeleted:   isDeleted,
		Status:      200,
	}
}

// ShortenURLBatch пакетное сокращение URL
func (s *ShortenerService) ShortenURLBatch(ctx context.Context, items []BatchItem, userID string) BatchResult {
	if len(items) == 0 {
		return BatchResult{
			Error:  "Empty batch",
			Status: 400,
		}
	}

	// Проверяем все URL на пустоту
	for _, item := range items {
		if item.OriginalURL == "" {
			return BatchResult{
				Error:  "URL cannot be empty",
				Status: 400,
			}
		}
	}

	response := make([]BatchResponseItem, 0, len(items))
	itemsToSave := make(map[string]string)

	for _, item := range items {
		id := s.generator.Generate()
		itemsToSave[id] = item.OriginalURL
		response = append(response, BatchResponseItem{
			CorrelationID: item.CorrelationID,
			ShortURL:      fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), id),
		})
	}

	if s.Repo != nil {
		tx, err := s.Repo.BeginTx(ctx)
		if err != nil {
			return BatchResult{
				Error:  "Failed to start transaction",
				Status: 500,
			}
		}

		for id, url := range itemsToSave {
			if err := tx.SaveURL(ctx, id, url, userID); err != nil {
				tx.Rollback()
				if err == storage.ErrURLConflict {
					existingID, err := s.Repo.FindExistingURL(ctx, url)
					if err != nil {
						return BatchResult{
							Error:  "Failed to check URL existence",
							Status: 500,
						}
					}
					// Обновляем response с существующим URL
					for i, item := range response {
						if item.ShortURL == fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), id) {
							response[i].ShortURL = fmt.Sprintf("%s/%s", strings.TrimSuffix(s.cfg.BaseURL, "/"), existingID)
							break
						}
					}
					continue
				}
				return BatchResult{
					Error:  "Failed to save URL",
					Status: 500,
				}
			}
		}

		if err := tx.Commit(); err != nil {
			return BatchResult{
				Error:  "Failed to commit transaction",
				Status: 500,
			}
		}
	} else {
		if err := s.storage.SaveBatch(itemsToSave, userID); err != nil {
			return BatchResult{
				Error:  "Failed to save URLs",
				Status: 500,
			}
		}
	}

	return BatchResult{
		Items:  response,
		Status: 201,
	}
}

// GetUserURLs получает все URL пользователя
func (s *ShortenerService) GetUserURLs(ctx context.Context, userID string) GetUserURLsResult {
	if userID == "" {
		return GetUserURLsResult{
			Error:  "User ID is required",
			Status: 401,
		}
	}

	var urls map[string]string
	var err error

	if s.Repo != nil {
		urls, err = s.Repo.GetUserURLs(ctx, userID)
		if err != nil {
			return GetUserURLsResult{
				Error:  "Database error",
				Status: 500,
			}
		}
	} else {
		urls = make(map[string]string)
	}

	// Добавляем URL из файлового хранилища
	fileUrls := s.storage.GetUserURLs(userID)
	for k, v := range fileUrls {
		urls[k] = v
	}

	if len(urls) == 0 {
		return GetUserURLsResult{
			Status: 204,
		}
	}

	result := make([]UserURL, 0, len(urls))
	for id, original := range urls {
		result = append(result, UserURL{
			ShortURL:    fmt.Sprintf("%s%s", s.cfg.BaseURL, id),
			OriginalURL: original,
		})
	}

	return GetUserURLsResult{
		URLs:   result,
		Status: 200,
	}
}

// DeleteUserURLs помечает URL как удаленные
func (s *ShortenerService) DeleteUserURLs(ctx context.Context, userID string, urlIDs []string) DeleteUserURLsResult {
	if userID == "" {
		return DeleteUserURLsResult{
			Error:  "User ID is required",
			Status: 401,
		}
	}

	if len(urlIDs) == 0 {
		return DeleteUserURLsResult{
			Error:  "URL IDs are required",
			Status: 400,
		}
	}

	if s.Repo != nil {
		if err := s.Repo.MarkURLsAsDeleted(ctx, userID, urlIDs); err != nil {
			return DeleteUserURLsResult{
				Error:  "Failed to mark URLs as deleted",
				Status: 500,
			}
		}
	} else {
		// Для файлового хранилища удаление не поддерживается
		return DeleteUserURLsResult{
			Error:  "Delete operation not supported for file storage",
			Status: 500,
		}
	}

	return DeleteUserURLsResult{
		Status: 202,
	}
}

// Ping проверяет доступность сервиса
func (s *ShortenerService) Ping(ctx context.Context) PingResult {
	if s.Repo == nil {
		return PingResult{
			Error:  "Database not configured",
			Status: 500,
		}
	}

	if err := s.Repo.Ping(ctx); err != nil {
		return PingResult{
			Error:  "Database ping failed",
			Status: 500,
		}
	}

	return PingResult{
		Status: 200,
	}
}

// GetStats получает статистику сервиса
func (s *ShortenerService) GetStats(ctx context.Context) GetStatsResult {
	var urlCount, userCount int
	var err error

	if s.Repo != nil {
		urlCount, userCount, err = s.Repo.GetStats(ctx)
		if err != nil {
			return GetStatsResult{
				Error:  "Failed to get stats from database",
				Status: 500,
			}
		}
	} else {
		urlCount, userCount, err = s.storage.GetStats()
		if err != nil {
			return GetStatsResult{
				Error:  "Failed to get stats from storage",
				Status: 500,
			}
		}
	}

	return GetStatsResult{
		URLs:   urlCount,
		Users:  userCount,
		Status: 200,
	}
}
