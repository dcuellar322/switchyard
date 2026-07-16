package application

import (
	"context"
	"sync"
)

type gateEntry struct {
	semaphore chan struct{}
	users     int
}

type keyedGate struct {
	mu      sync.Mutex
	entries map[string]*gateEntry
}

func newKeyedGate() *keyedGate {
	return &keyedGate{entries: make(map[string]*gateEntry)}
}

func (g *keyedGate) acquire(ctx context.Context, key string) (func(), error) {
	g.mu.Lock()
	entry := g.entries[key]
	if entry == nil {
		entry = &gateEntry{semaphore: make(chan struct{}, 1)}
		g.entries[key] = entry
	}
	entry.users++
	g.mu.Unlock()

	select {
	case entry.semaphore <- struct{}{}:
		return func() { g.release(key, entry) }, nil
	case <-ctx.Done():
		g.removeUser(key, entry)
		return nil, ctx.Err()
	}
}

func (g *keyedGate) release(key string, entry *gateEntry) {
	<-entry.semaphore
	g.removeUser(key, entry)
}

func (g *keyedGate) removeUser(key string, entry *gateEntry) {
	g.mu.Lock()
	defer g.mu.Unlock()
	entry.users--
	if entry.users == 0 {
		delete(g.entries, key)
	}
}
