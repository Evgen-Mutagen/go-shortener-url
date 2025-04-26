package config

import (
	"flag"
	"fmt"
)

type Config struct {
	ServerAddress string
	BaseURL       string
}

func LoadConfig() (*Config, error) {
	address := flag.String("a", "localhost:8080", "HTTP server address")
	baseURL := flag.String("b", "http://localhost:8080/", "Base URL for shortened links")

	flag.Parse()

	if *address == "" || *baseURL == "" {
		return nil, fmt.Errorf("адрес и урл не предоставлены")
	}

	return &Config{
		ServerAddress: *address,
		BaseURL:       *baseURL,
	}, nil
}
