package storage

import (
	"os"
	"testing"
)

func TestStorage_SaveBatch(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_storage_*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, err := NewStorage(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	urls := map[string]string{
		"id1": "https://example.com/1",
		"id2": "https://example.com/2",
	}

	err = s.SaveBatch(urls, "user123")
	if err != nil {
		t.Errorf("SaveBatch failed: %v", err)
	}

	for id, url := range urls {
		if got, exists := s.Get(id); !exists || got != url {
			t.Errorf("Get(%q) = %q, %v, want %q, true", id, got, exists, url)
		}
	}
}

func TestStorage_Close(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_storage_*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, err := NewStorage(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}
}

func TestStorage_Save(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_storage_*.json")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	s, err := NewStorage(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	err = s.Save("test1", "https://example.com/1", "user1")
	if err != nil {
		t.Errorf("Save() failed: %v", err)
	}

	// Проверяем, что URL сохранился
	if url, exists := s.Get("test1"); !exists || url != "https://example.com/1" {
		t.Errorf("Get() returned wrong result: %s, %v", url, exists)
	}
}
