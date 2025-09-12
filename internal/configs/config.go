package configs

import (
	"flag"
	"fmt"
	"strings"

	"github.com/caarlos0/env/v6"
)

// Config содержит настройки приложения
// Может быть загружен из переменных окружения или флагов командной строки
type Config struct {
	ServerAddress   string `env:"SERVER_ADDRESS" envDefault:"localhost:8080"`        // Адрес HTTP сервера
	BaseURL         string `env:"BASE_URL" envDefault:"http://localhost:8080/"`      // Базовый URL для сокращённых ссылок
	FileStoragePath string `env:"FILE_STORAGE_PATH" envDefault:"./url_storage.json"` // Путь к файлу хранилища
	DatabaseDSN     string `env:"DATABASE_DSN" envDefault:""`                        // DSN для подключения к БД
	EnableHTTPS     bool   `env:"ENABLE_HTTPS" envDefault:"false"`                   // Включить HTTPS
}

// LoadConfig используется для загрузки конфига
func LoadConfig() (*Config, error) {
	cfg := &Config{}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("ошибка при парсинге переменных окружения: %v", err)
	}

	addressFlag := flag.String("a", "", "HTTP server address (host:port)")
	baseURLFlag := flag.String("b", "", "Base URL for shortened links")
	fileStorageFlag := flag.String("f", "", "Path to file storage")
	databaseFlag := flag.String("d", "", "Database connection string")
	enableHTTPSFlag := flag.Bool("s", false, "Enable HTTPS")

	flag.Parse()

	if *addressFlag != "" {
		cfg.ServerAddress = *addressFlag
	}
	if *baseURLFlag != "" {
		cfg.BaseURL = *baseURLFlag
	}
	if *fileStorageFlag != "" {
		cfg.FileStoragePath = *fileStorageFlag
	}
	if *databaseFlag != "" {
		cfg.DatabaseDSN = *databaseFlag
	}
	if *enableHTTPSFlag {
		cfg.EnableHTTPS = true
	}

	serverAddr := strings.TrimPrefix(cfg.ServerAddress, "http://")
	serverAddr = strings.TrimPrefix(serverAddr, "https://")
	serverAddr = strings.TrimSuffix(serverAddr, "/")

	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")

	if serverAddr == "" || baseURL == "" {
		return nil, fmt.Errorf("адрес и урл не предоставлены")
	}

	return cfg, nil
}
