package auth

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

// BoltBlacklist is a durable Blacklist backed by a single bbolt file. It
// survives Stromboli restarts so a logged-out token stays logged out, at
// the cost of a disk write on every Add.
//
// Reads are cheap (memory-mapped page cache hits in steady state); writes
// are durable up to bolt's fsync semantics. Suitable for single-instance
// deployments — bbolt is process-local, so use Redis (or another shared
// backend) when you scale Stromboli horizontally.
type BoltBlacklist struct {
	db        *bolt.DB
	stopChan  chan struct{}
	cleanupMu sync.Mutex
}

// boltBucket holds {jti: int64-be expiresAt-unix-nano}.
var boltBucket = []byte("blacklist")

// NewBoltBlacklist opens (or creates) a bolt DB at path and returns a
// blacklist backed by it. The parent directory is created with 0700; the
// file itself is created with 0600 — JTIs are not secret per se, but
// they're an audit trail of who logged out and when, so keep the file
// owner-only.
//
// On open, expired entries are reaped synchronously so a fresh process
// doesn't appear to deny tokens that should already be valid again.
func NewBoltBlacklist(path string) (*BoltBlacklist, error) {
	if path == "" {
		return nil, fmt.Errorf("auth: bolt blacklist path is empty")
	}
	if dir := filepath.Dir(path); dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("auth: create bolt dir %q: %w", dir, err)
		}
	}

	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("auth: open bolt db at %q: %w", path, err)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(boltBucket)
		return err
	}); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("auth: create bolt bucket: %w", err)
	}

	b := &BoltBlacklist{db: db}
	if err := b.cleanup(); err != nil {
		// Non-fatal: a startup reap failure shouldn't prevent the server
		// from booting. The next periodic tick will retry.
		slog.Warn("bolt blacklist startup cleanup failed", "error", err)
	}
	return b, nil
}

// Add registers a JTI as blacklisted until expiresAt. Overwrites any prior
// entry for the same JTI.
func (b *BoltBlacklist) Add(jti string, expiresAt time.Time) error {
	if jti == "" {
		return fmt.Errorf("auth: jti is empty")
	}
	value := make([]byte, 8)
	binary.BigEndian.PutUint64(value, uint64(expiresAt.UnixNano()))
	return b.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(boltBucket).Put([]byte(jti), value)
	})
}

// IsBlacklisted reports whether jti is currently denied. Lazily filters
// expired entries — a positive-but-stale return would otherwise reject a
// re-issued token that happens to reuse the same JTI (extremely rare with
// UUIDs but possible after a clock skew event).
func (b *BoltBlacklist) IsBlacklisted(jti string) (bool, error) {
	var blacklisted bool
	err := b.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket(boltBucket).Get([]byte(jti))
		if v == nil {
			return nil
		}
		if len(v) != 8 {
			// Corrupt entry — treat as blacklisted to fail closed; the
			// cleanup pass will repair the store on next tick.
			blacklisted = true
			return nil
		}
		expiresAt := int64(binary.BigEndian.Uint64(v))
		blacklisted = time.Now().UnixNano() < expiresAt
		return nil
	})
	return blacklisted, err
}

// Size returns the current number of blacklisted JTIs (including any not-
// yet-reaped expired ones).
func (b *BoltBlacklist) Size() (int, error) {
	var count int
	err := b.db.View(func(tx *bolt.Tx) error {
		count = tx.Bucket(boltBucket).Stats().KeyN
		return nil
	})
	return count, err
}

// Close stops the cleanup goroutine and closes the underlying bolt DB.
// Idempotent.
func (b *BoltBlacklist) Close() error {
	b.StopCleanup()
	return b.db.Close()
}

// StartCleanup starts a background goroutine that removes expired entries
// at the specified interval. Idempotent.
func (b *BoltBlacklist) StartCleanup(interval time.Duration) {
	b.cleanupMu.Lock()
	defer b.cleanupMu.Unlock()
	if b.stopChan != nil {
		return
	}
	stop := make(chan struct{})
	b.stopChan = stop
	go b.cleanupLoop(interval, stop)
}

// StopCleanup stops the cleanup goroutine. Idempotent.
func (b *BoltBlacklist) StopCleanup() {
	b.cleanupMu.Lock()
	defer b.cleanupMu.Unlock()
	if b.stopChan != nil {
		close(b.stopChan)
		b.stopChan = nil
	}
}

func (b *BoltBlacklist) cleanupLoop(interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := b.cleanup(); err != nil {
				slog.Warn("bolt blacklist cleanup failed", "error", err)
			}
		case <-stop:
			return
		}
	}
}

// cleanup deletes every entry whose expiry timestamp is in the past.
// Two-phase: first collect expired keys under View, then delete under a
// single Update. Bolt only allows mutating writes inside Update, and
// iterating a bucket while writing is unsafe.
func (b *BoltBlacklist) cleanup() error {
	now := time.Now().UnixNano()
	var expired [][]byte

	if err := b.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(boltBucket).ForEach(func(k, v []byte) error {
			if len(v) != 8 {
				// Reap corrupt entries.
				expired = append(expired, append([]byte(nil), k...))
				return nil
			}
			expiresAt := int64(binary.BigEndian.Uint64(v))
			if expiresAt < now {
				expired = append(expired, append([]byte(nil), k...))
			}
			return nil
		})
	}); err != nil {
		return fmt.Errorf("auth: bolt cleanup scan: %w", err)
	}

	if len(expired) == 0 {
		return nil
	}

	return b.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(boltBucket)
		for _, k := range expired {
			if err := bucket.Delete(k); err != nil {
				return err
			}
		}
		return nil
	})
}
