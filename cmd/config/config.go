package config

import (
	"flag"
	"fmt"
	"strings"
)

type Config struct {
	ServerAddress string
	BaseURL       string
}

func LoadConfig() (*Config, error) {
	address := flag.String("a", "localhost:8080", "HTTP server address (host:port)")
	baseURL := flag.String("b", "http://localhost:8080/", "Base URL for shortened links")

	flag.Parse()

	serverAddr := strings.TrimPrefix(*address, "http://")
	serverAddr = strings.TrimPrefix(serverAddr, "https://")
	serverAddr = strings.TrimSuffix(serverAddr, "/")

	if serverAddr == "" || *baseURL == "" {
		return nil, fmt.Errorf("адрес и урл не предоставлены")
	}

	return &Config{
		ServerAddress: serverAddr,
		BaseURL:       *baseURL,
	}, nil
}
