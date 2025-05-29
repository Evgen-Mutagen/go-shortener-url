package storage

import (
	"encoding/json"
	"os"
	"sync"
)

type URLRecord struct {
	UUID        string `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

type Storage struct {
	filePath string
	urls     map[string]string
	mu       sync.RWMutex
}

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

func (s *Storage) Save(shortURL, originalURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.urls[shortURL] = originalURL
	return s.save()
}

func (s *Storage) Get(shortURL string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	url, exists := s.urls[shortURL]
	return url, exists
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

func (s *Storage) SaveBatch(urls map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for shortURL, originalURL := range urls {
		s.urls[shortURL] = originalURL
	}
	return s.save()
}
