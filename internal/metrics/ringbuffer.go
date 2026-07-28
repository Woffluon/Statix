package metrics

import "sync"

// RingBuffer is a fixed-capacity circular buffer of Snapshots.
// At default config (2s interval, 6h history, cap=10800), with ~512 bytes per Snapshot
// (including per-core slices), estimated memory footprint is ~5.4 MB.
type RingBuffer struct {
	mu   sync.RWMutex
	data []Snapshot
	head int // next write index
	size int // current fill count
	cap  int // max capacity
}

// NewRingBuffer allocates a ring buffer with fixed capacity.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1
	}
	return &RingBuffer{
		data: make([]Snapshot, capacity),
		cap:  capacity,
	}
}

// Push adds a new Snapshot to the ring buffer.
// If capacity is reached, the oldest snapshot is silently overwritten.
func (r *RingBuffer) Push(s Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.data[r.head] = s
	r.head = (r.head + 1) % r.cap
	if r.size < r.cap {
		r.size++
	}
}

// Latest returns the most recently pushed Snapshot, if any.
func (r *RingBuffer) Latest() (Snapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.size == 0 {
		return Snapshot{}, false
	}

	idx := (r.head - 1 + r.cap) % r.cap
	return r.data[idx], true
}

// All returns a slice containing a copy of all current Snapshots in chronological order (oldest to newest).
func (r *RingBuffer) All() []Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.size == 0 {
		return nil
	}

	result := make([]Snapshot, r.size)
	if r.size < r.cap {
		// Buffer not full yet, data is at [0..size-1]
		copy(result, r.data[:r.size])
	} else {
		// Buffer full: oldest item is at r.head
		n1 := copy(result, r.data[r.head:])
		copy(result[n1:], r.data[:r.head])
	}

	return result
}

// Size returns current fill count.
func (r *RingBuffer) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.size
}

// Capacity returns max capacity.
func (r *RingBuffer) Capacity() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cap
}
