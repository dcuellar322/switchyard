package application

import "sync"

type byteRing struct {
	mu        sync.Mutex
	data      []byte
	limit     int
	truncated bool
}

func newByteRing(limit int) *byteRing { return &byteRing{limit: limit} }

func (r *byteRing) Write(value []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(value) >= r.limit {
		r.data = append(r.data[:0], value[len(value)-r.limit:]...)
		r.truncated = true
		return
	}
	overflow := len(r.data) + len(value) - r.limit
	if overflow > 0 {
		copy(r.data, r.data[overflow:])
		r.data = r.data[:len(r.data)-overflow]
		r.truncated = true
	}
	r.data = append(r.data, value...)
}

func (r *byteRing) Snapshot() ([]byte, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]byte(nil), r.data...), r.truncated
}
