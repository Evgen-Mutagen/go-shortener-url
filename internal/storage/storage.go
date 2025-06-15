package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

type URLRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      string `json:"user_id"`
}

type Storage struct {
	filePath string
	urls     map[string]string
	mu       sync.RWMutex
}

var (
	ErrURLConflict = fmt.Errorf("URL already exists")
)

func NewStorage(filePath string) (*Storage, error) {
	s := &Storage{
		filePath: filePath,
		urls:     make(map[string]string),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Storage) Save(shortURL, originalURL, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := URLRecord{
		UUID:        shortURL,
		ShortURL:    shortURL,
		OriginalURL: originalURL,
		UserID:      userID,
	}

	file, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(record); err != nil {
		return err
	}

	return nil
}

func (s *Storage) Get(shortURL string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if url, exists := s.urls[shortURL]; exists {
		return url, true
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
	for shortURL, originalURL := range s.urls {
		record := URLRecord{
			UUID:        shortURL,
			ShortURL:    shortURL,
			OriginalURL: originalURL,
		}
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
		s.urls[record.ShortURL] = record.OriginalURL
	}

	return nil
}

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
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}

	return nil
}

func (s *Storage) GetUserURLs(userID string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string)
	for shortURL, originalURL := range s.urls {
		result[shortURL] = originalURL
	}

	file, err := os.Open(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return result
		}
		return result
	}
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

	return result
}
