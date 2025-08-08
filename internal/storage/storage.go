package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

// URLRecord представляет запись о URL в хранилище
type URLRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id"`
}

// Storage определяет интерфейс для работы с хранилищем URL
type Storage struct {
	filePath string
	urls     map[string]URLRecord
	mu       sync.RWMutex
	file     *os.File
	encoder  *json.Encoder
}

// ErrURLConflict урл уже существует
var (
	ErrURLConflict = fmt.Errorf("URL already exists")
)

// NewStorage создает новое хранилище
func NewStorage(filePath string) (*Storage, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	s := &Storage{
		filePath: filePath,
		urls:     make(map[string]URLRecord),
		file:     file,
		encoder:  json.NewEncoder(file),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

// Save сохраняет связь между коротким и оригинальным URL
func (s *Storage) Save(shortURL, originalURL, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := URLRecord{
		UUID:        shortURL,
		ShortURL:    shortURL,
		OriginalURL: originalURL,
		UserID:      userID,
	}

	s.urls[shortURL] = record
	return s.encoder.Encode(record)
}

// Close закрывает при чтении
func (s *Storage) Close() error {
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

// Get возвращает оригинальный URL по короткому идентификатору
func (s *Storage) Get(shortURL string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if record, exists := s.urls[shortURL]; exists {
		return record.OriginalURL, true
	}

	file, err := os.Open(s.filePath)
	if err != nil {
		return "", false
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	for decoder.More() {
		var record URLRecord
		if err := decoder.Decode(&record); err != nil {
			continue
		}
		if record.ShortURL == shortURL {
			return record.OriginalURL, true
		}
	}

	return "", false
}

func (s *Storage) save() error {
	file, err := os.Create(s.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, record := range s.urls {
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}

	return nil
}

func (s *Storage) load() error {
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(s.filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	for decoder.More() {
		var record URLRecord
		if err := decoder.Decode(&record); err != nil {
			return err
		}
		s.urls[record.ShortURL] = record
	}

	return nil
}

// SaveBatch сохранение урл в батчовом режиме
func (s *Storage) SaveBatch(urls map[string]string, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for shortURL, originalURL := range urls {
		record := URLRecord{
			UUID:        shortURL,
			ShortURL:    shortURL,
			OriginalURL: originalURL,
			UserID:      userID,
		}
		s.urls[shortURL] = record
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}

	return nil
}

// GetUserURLs возвращает все URL пользователя
func (s *Storage) GetUserURLs(userID string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string)

	for _, record := range s.urls {
		if record.UserID == userID {
			result[record.ShortURL] = record.OriginalURL
		}
	}

	file, err := os.Open(s.filePath)
	if err == nil {
		defer file.Close()
		decoder := json.NewDecoder(file)
		for {
			var record URLRecord
			if err := decoder.Decode(&record); err != nil {
				if err == io.EOF {
					break
				}
				continue
			}
			if record.UserID == userID {
				result[record.ShortURL] = record.OriginalURL
			}
		}
	}

	return result
}
