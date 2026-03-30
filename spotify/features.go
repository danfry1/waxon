package spotify

import (
	"context"
	"sync"

	"github.com/danielfry/spotui/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

type FeatureCache struct {
	client *spotifyapi.Client
	cache  map[string]*source.AudioFeatures
	mu     sync.Mutex
}

func NewFeatureCache(client *spotifyapi.Client) *FeatureCache {
	return &FeatureCache{
		client: client,
		cache:  make(map[string]*source.AudioFeatures),
	}
}

func (fc *FeatureCache) Get(trackID string) (*source.AudioFeatures, error) {
	fc.mu.Lock()
	if cached, ok := fc.cache[trackID]; ok {
		fc.mu.Unlock()
		return cached, nil
	}
	fc.mu.Unlock()

	features, err := fc.client.GetAudioFeatures(context.Background(), spotifyapi.ID(trackID))
	if err != nil {
		return nil, err
	}
	if len(features) == 0 || features[0] == nil {
		return nil, nil
	}

	f := features[0]
	af := &source.AudioFeatures{
		Energy:       float64(f.Energy),
		Valence:      float64(f.Valence),
		Danceability: float64(f.Danceability),
		Tempo:        float64(f.Tempo),
		Acousticness: float64(f.Acousticness),
	}

	fc.mu.Lock()
	fc.cache[trackID] = af
	if len(fc.cache) > 100 {
		for k := range fc.cache {
			delete(fc.cache, k)
			break
		}
	}
	fc.mu.Unlock()

	return af, nil
}
