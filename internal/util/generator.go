package util

import (
	"github.com/google/uuid"
)

// IDGenerator генерирует уникальные ID для URL
type IDGenerator struct {
}

// NewIDGenerator создает новый генератор ID
func NewIDGenerator() *IDGenerator {
	return &IDGenerator{}
}

// Generate генерирует новый уникальный ID
func (g *IDGenerator) Generate() string {
	return uuid.New().String()
}
