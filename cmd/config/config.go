package config

import (
	"flag"
	"fmt"
	"github.com/caarlos0/env/v6"
	"strings"
)

type Config struct {
	ServerAddress string `env:"SERVER_ADDRESS" envDefault:"localhost:8080"`
	BaseURL       string `env:"BASE_URL" envDefault:"http://localhost:8080/"`
}

func LoadConfig() (*Config, error) {
	cfg := &Config{}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("ошибка при парсинге переменных окружения: %v", err)
	}

	addressFlag := flag.String("a", "", "HTTP server address (host:port)")
	baseURLFlag := flag.String("b", "", "Base URL for shortened links")

	flag.Parse()

	if *addressFlag != "" && cfg.ServerAddress == "localhost:8080" {
		cfg.ServerAddress = *addressFlag
	}
	if *baseURLFlag != "" && cfg.BaseURL == "http://localhost:8080/" {
		cfg.BaseURL = *baseURLFlag
	}

	serverAddr := strings.TrimPrefix(cfg.ServerAddress, "http://")
	serverAddr = strings.TrimPrefix(serverAddr, "https://")
	serverAddr = strings.TrimSuffix(serverAddr, "/")

	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")

	if serverAddr == "" || baseURL == "" {
		return nil, fmt.Errorf("адрес и урл не предоставлены")
	}

	return &Config{
		ServerAddress: serverAddr,
		BaseURL:       baseURL,
	}, nil
}
