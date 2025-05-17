package util

import (
	"fmt"
	"sync"
)

type IDGenerator struct {
	counter int
	mutex   sync.Mutex
}

func NewIDGenerator() *IDGenerator {
	return &IDGenerator{}
}

func (g *IDGenerator) Generate() string {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.counter++
	return fmt.Sprintf("%X", g.counter)
}
