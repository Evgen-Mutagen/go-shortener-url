package configs

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/caarlos0/env/v6"
)

// JSONConfig структура для загрузки конфигурации из JSON файла
type JSONConfig struct {
	ServerAddress   string `json:"server_address"`
	BaseURL         string `json:"base_url"`
	FileStoragePath string `json:"file_storage_path"`
	DatabaseDSN     string `json:"database_dsn"`
	EnableHTTPS     bool   `json:"enable_https"`
}

// loadJSONConfig загружает конфигурацию из JSON файла
func loadJSONConfig(filename string) (*JSONConfig, error) {
	if filename == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("ошибка при чтении файла конфигурации %s: %w", filename, err)
	}

	var jsonConfig JSONConfig
	if err := json.Unmarshal(data, &jsonConfig); err != nil {
		return nil, fmt.Errorf("ошибка при парсинге JSON файла %s: %w", filename, err)
	}

	return &jsonConfig, nil
}

// Config содержит настройки приложения
// Может быть загружен из переменных окружения, флагов командной строки или JSON файла
type Config struct {
	ServerAddress   string `env:"SERVER_ADDRESS" envDefault:"localhost:8080"`        // Адрес HTTP сервера
	BaseURL         string `env:"BASE_URL" envDefault:"http://localhost:8080/"`      // Базовый URL для сокращённых ссылок
	FileStoragePath string `env:"FILE_STORAGE_PATH" envDefault:"./url_storage.json"` // Путь к файлу хранилища
	DatabaseDSN     string `env:"DATABASE_DSN" envDefault:""`                        // DSN для подключения к БД
	EnableHTTPS     bool   `env:"ENABLE_HTTPS" envDefault:"false"`                   // Включить HTTPS
	ConfigFile      string `env:"CONFIG" envDefault:""`                              // Путь к JSON файлу конфигурации
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
	configFlag := flag.String("c", "", "Path to JSON config file")
	configFlagLong := flag.String("config", "", "Path to JSON config file")

	flag.Parse()

	// Определяем путь к конфигурационному файлу (флаги имеют приоритет над переменными окружения)
	configFile := cfg.ConfigFile
	if *configFlag != "" {
		configFile = *configFlag
	}
	if *configFlagLong != "" {
		configFile = *configFlagLong
	}

	// Загружаем конфигурацию из JSON файла (если указан)
	jsonConfig, err := loadJSONConfig(configFile)
	if err != nil {
		return nil, err
	}

	// Применяем значения из JSON файла (если они не пустые)
	if jsonConfig != nil {
		if jsonConfig.ServerAddress != "" {
			cfg.ServerAddress = jsonConfig.ServerAddress
		}
		if jsonConfig.BaseURL != "" {
			cfg.BaseURL = jsonConfig.BaseURL
		}
		if jsonConfig.FileStoragePath != "" {
			cfg.FileStoragePath = jsonConfig.FileStoragePath
		}
		if jsonConfig.DatabaseDSN != "" {
			cfg.DatabaseDSN = jsonConfig.DatabaseDSN
		}
		// Для булевых значений проверяем, что они были явно установлены в JSON
		// (поскольку false является значением по умолчанию)
		cfg.EnableHTTPS = jsonConfig.EnableHTTPS
	}

	// Применяем флаги командной строки (они имеют наивысший приоритет)
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
