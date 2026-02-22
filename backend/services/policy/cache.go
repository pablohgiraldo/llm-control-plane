package policy

import (
	"container/list"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/upb/llm-control-plane/backend/models"
)

// CacheKey represents a unique key for caching policies
type CacheKey struct {
	OrgID  uuid.UUID
	AppID  uuid.UUID
	UserID *uuid.UUID
}

// String returns a string representation of the cache key
func (k CacheKey) String() string {
	if k.UserID != nil {
		return k.OrgID.String() + ":" + k.AppID.String() + ":" + k.UserID.String()
	}
	return k.OrgID.String() + ":" + k.AppID.String()
}

// cacheEntry represents a single cache entry with TTL
type cacheEntry struct {
	key        CacheKey
	policies   []*models.Policy
	insertedAt time.Time
	element    *list.Element // For LRU tracking
}

// isExpired checks if the cache entry has expired
func (e *cacheEntry) isExpired(ttl time.Duration) bool {
	return time.Since(e.insertedAt) > ttl
}

// PolicyCache is an in-memory LRU cache with TTL for policy data
// Thread-safe implementation using sync.RWMutex
type PolicyCache struct {
	mu       sync.RWMutex
	entries  map[string]*cacheEntry // Key: CacheKey.String()
	lruList  *list.List             // Doubly linked list for LRU tracking
	maxSize  int                    // Maximum number of entries
	ttl      time.Duration          // Time-to-live for entries
	hits     uint64                 // Cache hit counter
	misses   uint64                 // Cache miss counter
}

// NewPolicyCache creates a new PolicyCache with specified max size and TTL
func NewPolicyCache(maxSize int, ttl time.Duration) *PolicyCache {
	return &PolicyCache{
		entries: make(map[string]*cacheEntry),
		lruList: list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// GetPolicies retrieves policies from cache
// Returns nil if not found or expired
func (c *PolicyCache) GetPolicies(key CacheKey) []*models.Policy {
	c.mu.Lock()
	defer c.mu.Unlock()

	keyStr := key.String()
	entry, exists := c.entries[keyStr]

	// Check if entry exists and is not expired
	if !exists || entry.isExpired(c.ttl) {
		c.misses++
		if exists {
			// Remove expired entry
			c.removeEntry(keyStr)
		}
		return nil
	}

	// Move to front (most recently used)
	c.lruList.MoveToFront(entry.element)
	c.hits++

	return entry.policies
}

// SetPolicies stores policies in cache
func (c *PolicyCache) SetPolicies(key CacheKey, policies []*models.Policy) {
	c.mu.Lock()
	defer c.mu.Unlock()

	keyStr := key.String()

	// Check if entry already exists
	if entry, exists := c.entries[keyStr]; exists {
		// Update existing entry
		entry.policies = policies
		entry.insertedAt = time.Now()
		c.lruList.MoveToFront(entry.element)
		return
	}

	// Evict least recently used entry if cache is full
	if c.lruList.Len() >= c.maxSize {
		c.evictLRU()
	}

	// Create new entry
	entry := &cacheEntry{
		key:        key,
		policies:   policies,
		insertedAt: time.Now(),
	}

	// Add to front of LRU list
	entry.element = c.lruList.PushFront(keyStr)
	c.entries[keyStr] = entry
}

// Invalidate removes a specific cache entry
func (c *PolicyCache) Invalidate(key CacheKey) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.removeEntry(key.String())
}

// InvalidateOrg removes all cache entries for an organization
func (c *PolicyCache) InvalidateOrg(orgID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find and remove all entries for this org
	for keyStr, entry := range c.entries {
		if entry.key.OrgID == orgID {
			c.removeEntry(keyStr)
		}
	}
}

// InvalidateApp removes all cache entries for an application
func (c *PolicyCache) InvalidateApp(orgID, appID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find and remove all entries for this app
	for keyStr, entry := range c.entries {
		if entry.key.OrgID == orgID && entry.key.AppID == appID {
			c.removeEntry(keyStr)
		}
	}
}

// InvalidateUser removes all cache entries for a user
func (c *PolicyCache) InvalidateUser(orgID, appID uuid.UUID, userID uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Find and remove all entries for this user
	for keyStr, entry := range c.entries {
		if entry.key.OrgID == orgID && 
		   entry.key.AppID == appID && 
		   entry.key.UserID != nil && 
		   *entry.key.UserID == userID {
			c.removeEntry(keyStr)
		}
	}
}

// Clear removes all entries from the cache
func (c *PolicyCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
	c.lruList.Init()
}

// Stats returns cache statistics
func (c *PolicyCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Size:   c.lruList.Len(),
		MaxSize: c.maxSize,
		Hits:   c.hits,
		Misses: c.misses,
		HitRate: c.calculateHitRate(),
	}
}

// CacheStats represents cache statistics
type CacheStats struct {
	Size    int
	MaxSize int
	Hits    uint64
	Misses  uint64
	HitRate float64
}

// calculateHitRate calculates the cache hit rate
func (c *PolicyCache) calculateHitRate() float64 {
	total := c.hits + c.misses
	if total == 0 {
		return 0
	}
	return float64(c.hits) / float64(total)
}

// removeEntry removes an entry from the cache (must be called with lock held)
func (c *PolicyCache) removeEntry(keyStr string) {
	if entry, exists := c.entries[keyStr]; exists {
		c.lruList.Remove(entry.element)
		delete(c.entries, keyStr)
	}
}

// evictLRU evicts the least recently used entry (must be called with lock held)
func (c *PolicyCache) evictLRU() {
	if c.lruList.Len() == 0 {
		return
	}

	// Remove from back (least recently used)
	backElement := c.lruList.Back()
	if backElement != nil {
		keyStr := backElement.Value.(string)
		c.lruList.Remove(backElement)
		delete(c.entries, keyStr)
	}
}

// CleanupExpired removes all expired entries
// Should be called periodically in a background goroutine
func (c *PolicyCache) CleanupExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiredKeys := make([]string, 0)

	// Find all expired entries
	for keyStr, entry := range c.entries {
		if entry.isExpired(c.ttl) {
			expiredKeys = append(expiredKeys, keyStr)
		}
	}

	// Remove expired entries
	for _, keyStr := range expiredKeys {
		c.removeEntry(keyStr)
	}

	return len(expiredKeys)
}

// StartCleanupWorker starts a background worker to periodically clean up expired entries
func (c *PolicyCache) StartCleanupWorker(interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.CleanupExpired()
		case <-stopCh:
			return
		}
	}
}
