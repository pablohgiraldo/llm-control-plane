package policy

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/upb/llm-control-plane/backend/models"
)

func TestCacheKey_String(t *testing.T) {
	orgID := uuid.New()
	appID := uuid.New()
	userID := uuid.New()

	t.Run("without user ID", func(t *testing.T) {
		key := CacheKey{
			OrgID: orgID,
			AppID: appID,
		}
		expected := orgID.String() + ":" + appID.String()
		assert.Equal(t, expected, key.String())
	})

	t.Run("with user ID", func(t *testing.T) {
		key := CacheKey{
			OrgID:  orgID,
			AppID:  appID,
			UserID: &userID,
		}
		expected := orgID.String() + ":" + appID.String() + ":" + userID.String()
		assert.Equal(t, expected, key.String())
	})
}

func TestPolicyCache_GetSetPolicies(t *testing.T) {
	cache := NewPolicyCache(10, 5*time.Minute)
	orgID := uuid.New()
	appID := uuid.New()

	key := CacheKey{
		OrgID: orgID,
		AppID: appID,
	}

	// Test cache miss
	policies := cache.GetPolicies(key)
	assert.Nil(t, policies)

	// Test cache set and hit
	testPolicies := []*models.Policy{
		{
			ID:         uuid.New(),
			OrgID:      orgID,
			PolicyType: models.PolicyTypeRateLimit,
			Enabled:    true,
		},
	}

	cache.SetPolicies(key, testPolicies)
	cachedPolicies := cache.GetPolicies(key)
	assert.NotNil(t, cachedPolicies)
	assert.Equal(t, len(testPolicies), len(cachedPolicies))
	assert.Equal(t, testPolicies[0].ID, cachedPolicies[0].ID)

	// Check stats
	stats := cache.Stats()
	assert.Equal(t, 1, stats.Size)
	assert.Equal(t, uint64(1), stats.Hits)
	assert.Equal(t, uint64(1), stats.Misses)
	assert.Equal(t, 0.5, stats.HitRate)
}

func TestPolicyCache_TTLExpiration(t *testing.T) {
	cache := NewPolicyCache(10, 100*time.Millisecond)
	orgID := uuid.New()
	appID := uuid.New()

	key := CacheKey{
		OrgID: orgID,
		AppID: appID,
	}

	testPolicies := []*models.Policy{
		{
			ID:         uuid.New(),
			OrgID:      orgID,
			PolicyType: models.PolicyTypeRateLimit,
			Enabled:    true,
		},
	}

	cache.SetPolicies(key, testPolicies)

	// Should be available immediately
	cachedPolicies := cache.GetPolicies(key)
	assert.NotNil(t, cachedPolicies)

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Should be expired now
	cachedPolicies = cache.GetPolicies(key)
	assert.Nil(t, cachedPolicies)

	// Check that expired entry was removed
	stats := cache.Stats()
	assert.Equal(t, 0, stats.Size)
}

func TestPolicyCache_LRUEviction(t *testing.T) {
	cache := NewPolicyCache(3, 5*time.Minute)

	// Add 4 entries (should evict the first one)
	keys := make([]CacheKey, 4)
	for i := 0; i < 4; i++ {
		keys[i] = CacheKey{
			OrgID: uuid.New(),
			AppID: uuid.New(),
		}
		testPolicies := []*models.Policy{
			{ID: uuid.New()},
		}
		cache.SetPolicies(keys[i], testPolicies)
	}

	// Cache size should be 3 (max size)
	stats := cache.Stats()
	assert.Equal(t, 3, stats.Size)

	// First entry should be evicted
	policies := cache.GetPolicies(keys[0])
	assert.Nil(t, policies)

	// Other entries should still exist
	for i := 1; i < 4; i++ {
		policies := cache.GetPolicies(keys[i])
		assert.NotNil(t, policies)
	}
}

func TestPolicyCache_LRUOrdering(t *testing.T) {
	cache := NewPolicyCache(3, 5*time.Minute)

	// Add 3 entries
	keys := make([]CacheKey, 3)
	for i := 0; i < 3; i++ {
		keys[i] = CacheKey{
			OrgID: uuid.New(),
			AppID: uuid.New(),
		}
		testPolicies := []*models.Policy{
			{ID: uuid.New()},
		}
		cache.SetPolicies(keys[i], testPolicies)
	}

	// Access first entry (moves to front)
	cache.GetPolicies(keys[0])

	// Add a new entry (should evict keys[1], not keys[0])
	newKey := CacheKey{
		OrgID: uuid.New(),
		AppID: uuid.New(),
	}
	cache.SetPolicies(newKey, []*models.Policy{{ID: uuid.New()}})

	// keys[0] should still exist
	policies := cache.GetPolicies(keys[0])
	assert.NotNil(t, policies)

	// keys[1] should be evicted (was least recently used)
	policies = cache.GetPolicies(keys[1])
	assert.Nil(t, policies)

	// keys[2] and newKey should exist
	policies = cache.GetPolicies(keys[2])
	assert.NotNil(t, policies)
	policies = cache.GetPolicies(newKey)
	assert.NotNil(t, policies)
}

func TestPolicyCache_Invalidate(t *testing.T) {
	cache := NewPolicyCache(10, 5*time.Minute)
	orgID := uuid.New()
	appID := uuid.New()

	key := CacheKey{
		OrgID: orgID,
		AppID: appID,
	}

	testPolicies := []*models.Policy{
		{ID: uuid.New()},
	}

	cache.SetPolicies(key, testPolicies)

	// Verify it's cached
	policies := cache.GetPolicies(key)
	assert.NotNil(t, policies)

	// Invalidate
	cache.Invalidate(key)

	// Should be gone
	policies = cache.GetPolicies(key)
	assert.Nil(t, policies)

	stats := cache.Stats()
	assert.Equal(t, 0, stats.Size)
}

func TestPolicyCache_InvalidateOrg(t *testing.T) {
	cache := NewPolicyCache(10, 5*time.Minute)
	orgID1 := uuid.New()
	orgID2 := uuid.New()
	appID := uuid.New()

	key1 := CacheKey{OrgID: orgID1, AppID: appID}
	key2 := CacheKey{OrgID: orgID2, AppID: appID}

	testPolicies := []*models.Policy{{ID: uuid.New()}}

	cache.SetPolicies(key1, testPolicies)
	cache.SetPolicies(key2, testPolicies)

	// Invalidate org1
	cache.InvalidateOrg(orgID1)

	// key1 should be gone
	policies := cache.GetPolicies(key1)
	assert.Nil(t, policies)

	// key2 should still exist
	policies = cache.GetPolicies(key2)
	assert.NotNil(t, policies)
}

func TestPolicyCache_InvalidateApp(t *testing.T) {
	cache := NewPolicyCache(10, 5*time.Minute)
	orgID := uuid.New()
	appID1 := uuid.New()
	appID2 := uuid.New()

	key1 := CacheKey{OrgID: orgID, AppID: appID1}
	key2 := CacheKey{OrgID: orgID, AppID: appID2}

	testPolicies := []*models.Policy{{ID: uuid.New()}}

	cache.SetPolicies(key1, testPolicies)
	cache.SetPolicies(key2, testPolicies)

	// Invalidate app1
	cache.InvalidateApp(orgID, appID1)

	// key1 should be gone
	policies := cache.GetPolicies(key1)
	assert.Nil(t, policies)

	// key2 should still exist
	policies = cache.GetPolicies(key2)
	assert.NotNil(t, policies)
}

func TestPolicyCache_InvalidateUser(t *testing.T) {
	cache := NewPolicyCache(10, 5*time.Minute)
	orgID := uuid.New()
	appID := uuid.New()
	userID1 := uuid.New()
	userID2 := uuid.New()

	key1 := CacheKey{OrgID: orgID, AppID: appID, UserID: &userID1}
	key2 := CacheKey{OrgID: orgID, AppID: appID, UserID: &userID2}

	testPolicies := []*models.Policy{{ID: uuid.New()}}

	cache.SetPolicies(key1, testPolicies)
	cache.SetPolicies(key2, testPolicies)

	// Invalidate user1
	cache.InvalidateUser(orgID, appID, userID1)

	// key1 should be gone
	policies := cache.GetPolicies(key1)
	assert.Nil(t, policies)

	// key2 should still exist
	policies = cache.GetPolicies(key2)
	assert.NotNil(t, policies)
}

func TestPolicyCache_Clear(t *testing.T) {
	cache := NewPolicyCache(10, 5*time.Minute)

	// Add multiple entries
	for i := 0; i < 5; i++ {
		key := CacheKey{
			OrgID: uuid.New(),
			AppID: uuid.New(),
		}
		cache.SetPolicies(key, []*models.Policy{{ID: uuid.New()}})
	}

	stats := cache.Stats()
	assert.Equal(t, 5, stats.Size)

	// Clear all
	cache.Clear()

	stats = cache.Stats()
	assert.Equal(t, 0, stats.Size)
}

func TestPolicyCache_CleanupExpired(t *testing.T) {
	cache := NewPolicyCache(10, 100*time.Millisecond)

	// Add entries
	keys := make([]CacheKey, 3)
	for i := 0; i < 3; i++ {
		keys[i] = CacheKey{
			OrgID: uuid.New(),
			AppID: uuid.New(),
		}
		cache.SetPolicies(keys[i], []*models.Policy{{ID: uuid.New()}})
	}

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	// Cleanup expired entries
	removed := cache.CleanupExpired()
	assert.Equal(t, 3, removed)

	stats := cache.Stats()
	assert.Equal(t, 0, stats.Size)
}

func TestPolicyCache_UpdateExistingEntry(t *testing.T) {
	cache := NewPolicyCache(10, 5*time.Minute)
	orgID := uuid.New()
	appID := uuid.New()

	key := CacheKey{
		OrgID: orgID,
		AppID: appID,
	}

	// Set initial policies
	policies1 := []*models.Policy{
		{ID: uuid.New(), Priority: 1},
	}
	cache.SetPolicies(key, policies1)

	// Update with new policies
	policies2 := []*models.Policy{
		{ID: uuid.New(), Priority: 2},
		{ID: uuid.New(), Priority: 3},
	}
	cache.SetPolicies(key, policies2)

	// Should get updated policies
	cached := cache.GetPolicies(key)
	assert.NotNil(t, cached)
	assert.Equal(t, 2, len(cached))
	assert.Equal(t, 2, cached[0].Priority)

	// Size should still be 1 (updated, not added new)
	stats := cache.Stats()
	assert.Equal(t, 1, stats.Size)
}

func TestPolicyCache_ConcurrentAccess(t *testing.T) {
	cache := NewPolicyCache(100, 5*time.Minute)
	orgID := uuid.New()
	appID := uuid.New()

	key := CacheKey{
		OrgID: orgID,
		AppID: appID,
	}

	testPolicies := []*models.Policy{
		{ID: uuid.New()},
	}

	// Concurrent writes and reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				cache.SetPolicies(key, testPolicies)
				cache.GetPolicies(key)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should not panic and should have the entry
	policies := cache.GetPolicies(key)
	assert.NotNil(t, policies)
}

func TestPolicyCache_Stats(t *testing.T) {
	cache := NewPolicyCache(10, 5*time.Minute)

	// Initial stats
	stats := cache.Stats()
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, 10, stats.MaxSize)
	assert.Equal(t, uint64(0), stats.Hits)
	assert.Equal(t, uint64(0), stats.Misses)
	assert.Equal(t, 0.0, stats.HitRate)

	key := CacheKey{
		OrgID: uuid.New(),
		AppID: uuid.New(),
	}

	// Miss
	cache.GetPolicies(key)

	// Set
	cache.SetPolicies(key, []*models.Policy{{ID: uuid.New()}})

	// Hit
	cache.GetPolicies(key)
	cache.GetPolicies(key)

	stats = cache.Stats()
	assert.Equal(t, 1, stats.Size)
	assert.Equal(t, uint64(2), stats.Hits)
	assert.Equal(t, uint64(1), stats.Misses)
	assert.Equal(t, 2.0/3.0, stats.HitRate)
}
