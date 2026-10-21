package changes

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryUnlocks keeps session unlocks in memory; they never outlive the process.
type MemoryUnlocks struct {
	mu    sync.Mutex
	until map[uuid.UUID]time.Time
	now   func() time.Time
}

// NewMemoryUnlocks builds an empty unlock table.
func NewMemoryUnlocks() *MemoryUnlocks {
	return &MemoryUnlocks{until: map[uuid.UUID]time.Time{}, now: time.Now}
}

// Until reports when the unlock for id ends, if one is active.
func (u *MemoryUnlocks) Until(id uuid.UUID) (time.Time, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	t, ok := u.until[id]
	if !ok || !t.After(u.now()) {
		delete(u.until, id)
		return time.Time{}, false
	}
	return t, true
}

// Unlock allows writes on id for ttl and returns when that ends.
func (u *MemoryUnlocks) Unlock(id uuid.UUID, ttl time.Duration) time.Time {
	u.mu.Lock()
	defer u.mu.Unlock()
	t := u.now().Add(ttl)
	u.until[id] = t
	return t
}

// Lock ends the unlock for id.
func (u *MemoryUnlocks) Lock(id uuid.UUID) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.until, id)
}

// IsUnlocked reports whether writes are currently allowed on id.
func (u *MemoryUnlocks) IsUnlocked(id uuid.UUID) bool {
	_, ok := u.Until(id)
	return ok
}
