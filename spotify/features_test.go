package spotify

import (
	"context"
	"fmt"
	"testing"

	"github.com/danfry1/waxon/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

// ---------------------------------------------------------------------------
// FeatureCache — direct cache manipulation tests
// ---------------------------------------------------------------------------

// newTestCache creates a FeatureCache with a nil client (we won't call the
// API in these tests — we manipulate the cache fields directly).
func newTestCache() *FeatureCache {
	return &FeatureCache{
		client:   nil,
		cache:    make(map[string]*source.AudioFeatures),
		inflight: make(map[string]*call),
	}
}

// insertIntoCache simulates what Get does after a successful API call:
// stores the value, appends to the order slice, and evicts if needed.
func insertIntoCache(fc *FeatureCache, id string, af *source.AudioFeatures) {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fc.cache[id] = af
	fc.order = append(fc.order, id)
	for len(fc.cache) > maxFeatureCacheSize {
		oldest := fc.order[0]
		copy(fc.order, fc.order[1:])
		fc.order = fc.order[:len(fc.order)-1]
		delete(fc.cache, oldest)
	}
}

func TestFeatureCache_CacheHit(t *testing.T) {
	fc := newTestCache()

	want := &source.AudioFeatures{
		Energy:       0.8,
		Valence:      0.6,
		Danceability: 0.75,
		Tempo:        120.0,
		Acousticness: 0.1,
	}
	insertIntoCache(fc, "hit-track", want)

	// Get should return the cached value without calling the API (client is nil,
	// so a real API call would panic).
	got, err := fc.Get(context.Background(), "hit-track")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got != want {
		t.Errorf("Get returned %+v, want %+v", got, want)
	}
}

func TestFeatureCache_CacheHitValues(t *testing.T) {
	fc := newTestCache()

	af := &source.AudioFeatures{
		Energy:       0.42,
		Valence:      0.99,
		Danceability: 0.33,
		Tempo:        88.5,
		Acousticness: 0.77,
	}
	insertIntoCache(fc, "val-check", af)

	got, err := fc.Get(context.Background(), "val-check")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Energy != 0.42 {
		t.Errorf("Energy = %f, want 0.42", got.Energy)
	}
	if got.Valence != 0.99 {
		t.Errorf("Valence = %f, want 0.99", got.Valence)
	}
	if got.Danceability != 0.33 {
		t.Errorf("Danceability = %f, want 0.33", got.Danceability)
	}
	if got.Tempo != 88.5 {
		t.Errorf("Tempo = %f, want 88.5", got.Tempo)
	}
	if got.Acousticness != 0.77 {
		t.Errorf("Acousticness = %f, want 0.77", got.Acousticness)
	}
}

func TestFeatureCache_FIFOEviction(t *testing.T) {
	fc := newTestCache()

	// Fill the cache to maxFeatureCacheSize
	for i := 0; i < maxFeatureCacheSize; i++ {
		id := fmt.Sprintf("track-%03d", i)
		insertIntoCache(fc, id, &source.AudioFeatures{Tempo: float64(i)})
	}

	// Verify the cache is full
	fc.mu.Lock()
	if len(fc.cache) != maxFeatureCacheSize {
		t.Fatalf("cache size = %d, want %d", len(fc.cache), maxFeatureCacheSize)
	}
	fc.mu.Unlock()

	// Insert one more — should evict the oldest (track-000)
	insertIntoCache(fc, "track-new", &source.AudioFeatures{Tempo: 999})

	fc.mu.Lock()
	defer fc.mu.Unlock()

	if len(fc.cache) != maxFeatureCacheSize {
		t.Errorf("cache size after eviction = %d, want %d", len(fc.cache), maxFeatureCacheSize)
	}

	// The oldest entry (track-000) should have been evicted
	if _, ok := fc.cache["track-000"]; ok {
		t.Error("track-000 should have been evicted but is still in cache")
	}

	// The newest entry should be present
	if _, ok := fc.cache["track-new"]; !ok {
		t.Error("track-new should be in cache but was not found")
	}

	// A middle entry should still be present
	if _, ok := fc.cache["track-050"]; !ok {
		t.Error("track-050 should still be in cache")
	}

	// The order slice should reflect the eviction
	if len(fc.order) != maxFeatureCacheSize {
		t.Errorf("order length = %d, want %d", len(fc.order), maxFeatureCacheSize)
	}
	if fc.order[0] != "track-001" {
		t.Errorf("order[0] = %q, want %q (should be the second-oldest after eviction)", fc.order[0], "track-001")
	}
	if fc.order[len(fc.order)-1] != "track-new" {
		t.Errorf("order[last] = %q, want %q", fc.order[len(fc.order)-1], "track-new")
	}
}

func TestFeatureCache_MultipleEvictions(t *testing.T) {
	fc := newTestCache()

	// Fill to capacity
	for i := 0; i < maxFeatureCacheSize; i++ {
		id := fmt.Sprintf("t%d", i)
		insertIntoCache(fc, id, &source.AudioFeatures{Energy: float64(i) / 100})
	}

	// Insert 5 more — should evict t0, t1, t2, t3, t4
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("new-%d", i)
		insertIntoCache(fc, id, &source.AudioFeatures{Energy: 1.0})
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	if len(fc.cache) != maxFeatureCacheSize {
		t.Errorf("cache size = %d, want %d", len(fc.cache), maxFeatureCacheSize)
	}

	// The first 5 original entries should be gone
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("t%d", i)
		if _, ok := fc.cache[id]; ok {
			t.Errorf("%s should have been evicted", id)
		}
	}

	// Entry t5 should still be present
	if _, ok := fc.cache["t5"]; !ok {
		t.Error("t5 should still be in cache after 5 evictions")
	}

	// All new entries should be present
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("new-%d", i)
		if _, ok := fc.cache[id]; !ok {
			t.Errorf("%s should be in cache", id)
		}
	}
}

func TestFeatureCache_EmptyCache_Miss(t *testing.T) {
	fc := newTestCache()

	fc.mu.Lock()
	_, ok := fc.cache["nonexistent"]
	fc.mu.Unlock()

	if ok {
		t.Error("empty cache should not contain any entries")
	}
}

func TestFeatureCache_OrderIntegrity(t *testing.T) {
	fc := newTestCache()

	ids := []string{"alpha", "beta", "gamma", "delta"}
	for _, id := range ids {
		insertIntoCache(fc, id, &source.AudioFeatures{})
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()

	if len(fc.order) != len(ids) {
		t.Fatalf("order length = %d, want %d", len(fc.order), len(ids))
	}
	for i, id := range ids {
		if fc.order[i] != id {
			t.Errorf("order[%d] = %q, want %q", i, fc.order[i], id)
		}
	}
}

// ---------------------------------------------------------------------------
// NewFeatureCache constructor
// ---------------------------------------------------------------------------

func TestNewFeatureCache(t *testing.T) {
	// Use a nil-transport client just to verify constructor fields
	client := spotifyapi.New(nil)
	fc := NewFeatureCache(client)

	if fc == nil {
		t.Fatal("NewFeatureCache returned nil")
	}
	if fc.client == nil {
		t.Error("client field should not be nil")
	}
	if fc.cache == nil {
		t.Error("cache map should not be nil")
	}
	if fc.inflight == nil {
		t.Error("inflight map should not be nil")
	}
	if len(fc.cache) != 0 {
		t.Errorf("cache should start empty, got %d entries", len(fc.cache))
	}
	if len(fc.inflight) != 0 {
		t.Errorf("inflight should start empty, got %d entries", len(fc.inflight))
	}
	if len(fc.order) != 0 {
		t.Errorf("order should start empty, got %d entries", len(fc.order))
	}
}

// ---------------------------------------------------------------------------
// maxFeatureCacheSize constant
// ---------------------------------------------------------------------------

func TestMaxFeatureCacheSizeValue(t *testing.T) {
	if maxFeatureCacheSize != 100 {
		t.Errorf("maxFeatureCacheSize = %d, want 100", maxFeatureCacheSize)
	}
}
