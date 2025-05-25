package util

import (
	"github.com/google/uuid"
)

type IDGenerator struct {
}

func NewIDGenerator() *IDGenerator {
	return &IDGenerator{}
}

func (g *IDGenerator) Generate() string {
	return uuid.New().String()
}
