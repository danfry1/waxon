package app

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.Set(x, y, color.RGBA{R: 200, G: 10, B: 10, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func imageServer(t *testing.T, hits *int32) *httptest.Server {
	t.Helper()
	data := pngBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		if r.URL.Path == "/missing" {
			w.WriteHeader(404)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func resetImageCache(t *testing.T, dir string) {
	t.Helper()
	ClearImageCache()
	SetImageCacheDir(dir)
	t.Cleanup(func() { ClearImageCache(); SetImageCacheDir("") })
}

func TestFetchImageMemoryCache(t *testing.T) {
	var hits int32
	srv := imageServer(t, &hits)
	resetImageCache(t, "")
	ctx := context.Background()
	for range 3 {
		if _, err := FetchImage(ctx, srv.URL+"/a.png"); err != nil {
			t.Fatal(err)
		}
	}
	if hits != 1 {
		t.Errorf("expected one network fetch, got %d", hits)
	}
}

func TestFetchImageDiskCacheSurvivesMemoryClear(t *testing.T) {
	var hits int32
	srv := imageServer(t, &hits)
	dir := t.TempDir()
	resetImageCache(t, dir)
	ctx := context.Background()
	if _, err := FetchImage(ctx, srv.URL+"/b.png"); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected one cached file, got %d", len(entries))
	}
	ClearImageCache() // simulate a new launch
	if _, err := FetchImage(ctx, srv.URL+"/b.png"); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Errorf("second launch should be served from disk; network hits = %d", hits)
	}
}

func TestFetchImageCorruptDiskEntryIsRefetched(t *testing.T) {
	var hits int32
	srv := imageServer(t, &hits)
	dir := t.TempDir()
	resetImageCache(t, dir)
	url := srv.URL + "/c.png"
	if err := os.WriteFile(filepath.Join(dir, cacheKey(url)), []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := FetchImage(context.Background(), url); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Errorf("corrupt entry should fall through to network, hits=%d", hits)
	}
}

func TestFetchImageCoalescesConcurrentRequests(t *testing.T) {
	var hits int32
	data := pngBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	resetImageCache(t, "")
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := FetchImage(context.Background(), srv.URL+"/same.png"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if hits != 1 {
		t.Errorf("8 concurrent requests for one URL should make one fetch, got %d", hits)
	}
}

func TestFetchImageErrorsAreNotCached(t *testing.T) {
	var hits int32
	srv := imageServer(t, &hits)
	resetImageCache(t, t.TempDir())
	for range 2 {
		if _, err := FetchImage(context.Background(), srv.URL+"/missing"); err == nil {
			t.Fatal("expected error for 404")
		}
	}
	if hits != 2 {
		t.Errorf("failures must not be cached, hits=%d", hits)
	}
	if _, err := FetchImage(context.Background(), ""); err == nil {
		t.Error("empty url should error")
	}
}

func TestMemoryCacheEvictsOldest(t *testing.T) {
	resetImageCache(t, "")
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	for i := range imageMemCacheSize + 5 {
		memPut(string(rune('a'+i%26))+string(rune('A'+i/26)), img)
	}
	if len(imageMem) != imageMemCacheSize || len(imageMemOrder) != imageMemCacheSize {
		t.Errorf("mem cache size = %d/%d, want %d", len(imageMem), len(imageMemOrder), imageMemCacheSize)
	}
	if _, ok := memGet("aA"); ok {
		t.Error("oldest entry should have been evicted")
	}
}

func TestPruneImageCache(t *testing.T) {
	dir := t.TempDir()
	resetImageCache(t, dir)
	// Write files totalling > limit with distinct mtimes; oldest must go first.
	big := bytes.Repeat([]byte("x"), 1<<20) // 1 MB
	for i := range imageDiskCacheMaxMB + 3 {
		p := filepath.Join(dir, "f"+string(rune('a'+i%26))+string(rune('a'+i/26)))
		if err := os.WriteFile(p, big, 0o600); err != nil {
			t.Fatal(err)
		}
		mt := time.Now().Add(time.Duration(i-100) * time.Minute)
		_ = os.Chtimes(p, mt, mt)
	}
	PruneImageCache()
	entries, _ := os.ReadDir(dir)
	var total int64
	for _, e := range entries {
		info, _ := e.Info()
		total += info.Size()
	}
	if total > int64(imageDiskCacheMaxMB)<<20 {
		t.Errorf("cache not pruned under limit: %d bytes, %d files", total, len(entries))
	}
	if _, err := os.Stat(filepath.Join(dir, "faa")); err == nil {
		t.Error("oldest file should have been pruned first")
	}
}

func TestDefaultImageCacheDirHonoursXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdgtest")
	if got := DefaultImageCacheDir(); got != filepath.Join("/tmp/xdgtest", "waxon", "images") {
		t.Errorf("got %q", got)
	}
}

func TestSidebarIconsStreamPerPlaylist(t *testing.T) {
	var hits int32
	srv := imageServer(t, &hits)
	resetImageCache(t, "")
	m := newTestModel(&StubSource{})
	pls := []source.Playlist{
		{ID: "p1", Name: "A", ImageURL: srv.URL + "/1.png"},
		{ID: "p2", Name: "B"}, // no image → no command
		{ID: "p3", Name: "C", ImageURL: srv.URL + "/3.png"},
	}
	m.sidebar.SetPlaylists(pls)
	cmd := m.fetchSidebarIcons(pls)
	if cmd == nil {
		t.Fatal("expected commands")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("expected a batch of 2 per-playlist commands, got %T len=%d", cmd(), len(batch))
	}
	for _, c := range batch {
		msg := c()
		im, ok := msg.(sidebarIconLoadedMsg)
		if !ok || im.icon == "" {
			t.Fatalf("got %#v, want sidebarIconLoadedMsg with icon", msg)
		}
		result, _ := m.Update(im)
		m = result.(Model)
	}
	for _, item := range m.sidebar.allItems {
		si := item.(sidebarItem)
		if (si.playlist.ID == "p1" || si.playlist.ID == "p3") && si.icon == "" {
			t.Errorf("icon for %s not applied", si.playlist.ID)
		}
		if si.playlist.ID == "p2" && si.icon != "" {
			t.Error("p2 has no image and should keep the fallback")
		}
	}
}
