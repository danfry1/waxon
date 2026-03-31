package spotify

import (
	"context"
	"sync"

	"github.com/danielfry/waxon/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

const maxFeatureCacheSize = 100

// FeatureCache caches audio feature lookups with FIFO eviction.
type FeatureCache struct {
	client   *spotifyapi.Client
	cache    map[string]*source.AudioFeatures
	order    []string // insertion order for FIFO eviction
	inflight map[string]*call
	mu       sync.Mutex
}

// call represents an in-flight or completed API request.
type call struct {
	wg  sync.WaitGroup
	val *source.AudioFeatures
	err error
}

func NewFeatureCache(client *spotifyapi.Client) *FeatureCache {
	return &FeatureCache{
		client:   client,
		cache:    make(map[string]*source.AudioFeatures),
		inflight: make(map[string]*call),
	}
}

func (fc *FeatureCache) Get(ctx context.Context, trackID string) (*source.AudioFeatures, error) {
	fc.mu.Lock()
	if cached, ok := fc.cache[trackID]; ok {
		fc.mu.Unlock()
		return cached, nil
	}

	// Check if there's already an in-flight request for this track
	if c, ok := fc.inflight[trackID]; ok {
		fc.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}

	// Register this as an in-flight request
	c := &call{}
	c.wg.Add(1)
	fc.inflight[trackID] = c
	fc.mu.Unlock()

	// Make the API call outside the lock
	features, err := fc.client.GetAudioFeatures(ctx, spotifyapi.ID(trackID))
	if err != nil {
		c.err = err
		c.wg.Done()
		fc.mu.Lock()
		delete(fc.inflight, trackID)
		fc.mu.Unlock()
		return nil, err
	}

	var af *source.AudioFeatures
	if len(features) > 0 && features[0] != nil {
		f := features[0]
		af = &source.AudioFeatures{
			Energy:       float64(f.Energy),
			Valence:      float64(f.Valence),
			Danceability: float64(f.Danceability),
			Tempo:        float64(f.Tempo),
			Acousticness: float64(f.Acousticness),
		}
	}

	// Populate cache before unblocking waiters so they see the result
	fc.mu.Lock()
	delete(fc.inflight, trackID)
	if af != nil {
		fc.cache[trackID] = af
		fc.order = append(fc.order, trackID)
		for len(fc.cache) > maxFeatureCacheSize {
			oldest := fc.order[0]
			copy(fc.order, fc.order[1:])
			fc.order = fc.order[:len(fc.order)-1]
			delete(fc.cache, oldest)
		}
	}
	fc.mu.Unlock()

	c.val = af
	c.wg.Done()

	return af, nil
}
