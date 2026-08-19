package app

import (
	"bytes"
	"context"
	"crypto/sha1" //nolint:gosec // cache key, not security
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Image caching: every cover waxon renders — sidebar icons, header thumbs,
// the status-bar art, the Now Playing view — used to be downloaded and decoded
// again on every use and every launch. The cache has two layers:
//
//   - memory: decoded images, bounded LRU, shared by all views in a session
//   - disk:   raw encoded bytes under the XDG cache dir, so a second launch
//     paints the sidebar without touching the network
//
// Network fetches are bounded by a semaphore so a library with hundreds of
// playlists doesn't open hundreds of connections at once.

const (
	imageMemCacheSize   = 96 // decoded images kept in memory
	imageFetchParallel  = 6  // concurrent downloads
	imageDiskCacheMaxMB = 64 // prune the disk cache above this size
)

var (
	imageCacheMu    sync.Mutex
	imageMem        = map[string]*memImage{}
	imageMemOrder   []string // LRU: oldest first
	imageDiskDir    string   // "" = disk cache disabled
	imageFetchSem   = make(chan struct{}, imageFetchParallel)
	imageInflight   = map[string]*inflightImage{}
	imageInflightMu sync.Mutex
)

type memImage struct {
	img image.Image
}

type inflightImage struct {
	done chan struct{}
	img  image.Image
	err  error
}

// SetImageCacheDir enables the on-disk image cache at dir (created on
// demand). An empty dir disables it. Call once at startup.
func SetImageCacheDir(dir string) {
	imageCacheMu.Lock()
	defer imageCacheMu.Unlock()
	imageDiskDir = dir
}

// DefaultImageCacheDir returns $XDG_CACHE_HOME/waxon/images (or the OS user
// cache dir equivalent). Returns "" if no cache dir can be determined.
func DefaultImageCacheDir() string {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "waxon", "images")
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "waxon", "images")
}

// ClearImageCache drops the in-memory cache (tests).
func ClearImageCache() {
	imageCacheMu.Lock()
	defer imageCacheMu.Unlock()
	imageMem = map[string]*memImage{}
	imageMemOrder = nil
}

func cacheKey(url string) string {
	sum := sha1.Sum([]byte(url)) //nolint:gosec // cache key, not security
	return hex.EncodeToString(sum[:])
}

func memGet(url string) (image.Image, bool) {
	imageCacheMu.Lock()
	defer imageCacheMu.Unlock()
	m, ok := imageMem[url]
	if !ok {
		return nil, false
	}
	// Move to most-recent.
	for i, u := range imageMemOrder {
		if u == url {
			imageMemOrder = append(imageMemOrder[:i], imageMemOrder[i+1:]...)
			break
		}
	}
	imageMemOrder = append(imageMemOrder, url)
	return m.img, true
}

func memPut(url string, img image.Image) {
	imageCacheMu.Lock()
	defer imageCacheMu.Unlock()
	if _, ok := imageMem[url]; !ok {
		imageMemOrder = append(imageMemOrder, url)
	}
	imageMem[url] = &memImage{img: img}
	for len(imageMemOrder) > imageMemCacheSize {
		oldest := imageMemOrder[0]
		imageMemOrder = imageMemOrder[1:]
		delete(imageMem, oldest)
	}
}

func diskPath(url string) string {
	imageCacheMu.Lock()
	dir := imageDiskDir
	imageCacheMu.Unlock()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, cacheKey(url))
}

func diskGet(url string) (image.Image, bool) {
	path := diskPath(url)
	if path == "" {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		_ = os.Remove(path) // corrupt entry; refetch next time
		return nil, false
	}
	return img, true
}

func diskPut(url string, data []byte) {
	path := diskPath(url)
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		slog.Debug("image cache dir", "error", err)
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// PruneImageCache deletes the oldest files in the disk cache until it is
// under imageDiskCacheMaxMB. Cheap enough to run once per launch in the
// background.
func PruneImageCache() {
	imageCacheMu.Lock()
	dir := imageDiskDir
	imageCacheMu.Unlock()
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type fileInfo struct {
		path string
		mod  time.Time
		size int64
	}
	var files []fileInfo
	var total int64
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || e.IsDir() {
			continue
		}
		files = append(files, fileInfo{filepath.Join(dir, e.Name()), info.ModTime(), info.Size()})
		total += info.Size()
	}
	limit := int64(imageDiskCacheMaxMB) << 20
	if total <= limit {
		return
	}
	// Oldest first.
	for i := 1; i < len(files); i++ {
		for j := i; j > 0 && files[j].mod.Before(files[j-1].mod); j-- {
			files[j], files[j-1] = files[j-1], files[j]
		}
	}
	for _, f := range files {
		if total <= limit {
			break
		}
		if err := os.Remove(f.path); err == nil {
			total -= f.size
		}
	}
}

// FetchImage returns the decoded image at url, serving from the memory cache,
// then the disk cache, then the network (bounded concurrency, de-duplicated
// so concurrent requests for the same URL share one download).
func FetchImage(ctx context.Context, url string) (image.Image, error) {
	if url == "" {
		return nil, errors.New("fetch image: empty url")
	}
	if img, ok := memGet(url); ok {
		return img, nil
	}

	// Coalesce concurrent fetches for the same URL.
	imageInflightMu.Lock()
	if f, ok := imageInflight[url]; ok {
		imageInflightMu.Unlock()
		select {
		case <-f.done:
			return f.img, f.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	f := &inflightImage{done: make(chan struct{})}
	imageInflight[url] = f
	imageInflightMu.Unlock()

	f.img, f.err = fetchImageUncached(ctx, url)
	close(f.done)
	imageInflightMu.Lock()
	delete(imageInflight, url)
	imageInflightMu.Unlock()
	return f.img, f.err
}

func fetchImageUncached(ctx context.Context, url string) (image.Image, error) {
	if img, ok := diskGet(url); ok {
		memPut(url, img)
		return img, nil
	}

	select {
	case imageFetchSem <- struct{}{}:
		defer func() { <-imageFetchSem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpImageClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch image: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes))
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	memPut(url, img)
	diskPut(url, data)
	return img, nil
}
