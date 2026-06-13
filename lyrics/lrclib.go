// Package lyrics fetches song lyrics from lrclib.net, a free, no-auth lyrics
// API. It returns synced (timestamped) lyrics when available, falling back to
// plain lyrics, and parses the LRC timestamp format into structured lines.
package lyrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/danfry1/waxon/source"
)

const (
	defaultBaseURL = "https://lrclib.net"
	// lrclib asks clients to identify themselves with a descriptive User-Agent.
	userAgent     = "waxon (https://github.com/danfry1/waxon)"
	maxCacheSize  = 100
	requestExpiry = 10 * time.Second
)

// Client fetches and caches lyrics. The zero value is not usable; call New.
type Client struct {
	http    *http.Client
	baseURL string

	mu       sync.Mutex
	cache    map[string]*source.Lyrics
	order    []string // insertion order for FIFO eviction
	inflight map[string]*call
}

type call struct {
	wg  sync.WaitGroup
	val *source.Lyrics
	err error
}

// New returns a Client backed by lrclib.net.
func New() *Client {
	return &Client{
		http:     &http.Client{Timeout: requestExpiry},
		baseURL:  defaultBaseURL,
		cache:    make(map[string]*source.Lyrics),
		inflight: make(map[string]*call),
	}
}

// lrclibResponse mirrors the fields waxon uses from lrclib's /api/get response.
type lrclibResponse struct {
	Instrumental bool   `json:"instrumental"`
	PlainLyrics  string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
}

// Get returns lyrics for track, or (nil, nil) when none are found. Results are
// cached by track ID; concurrent calls for the same track share one request.
func (c *Client) Get(ctx context.Context, track source.Track) (*source.Lyrics, error) {
	key := track.ID
	if key == "" {
		// Fall back to a name-based key so demo/stub tracks without IDs still cache.
		key = track.Artist + "\x00" + track.Name
	}

	c.mu.Lock()
	if cached, ok := c.cache[key]; ok {
		c.mu.Unlock()
		return cached, nil
	}
	if existing, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		existing.wg.Wait()
		return existing.val, existing.err
	}
	cl := &call{}
	cl.wg.Add(1)
	c.inflight[key] = cl
	c.mu.Unlock()

	lyr, err := c.fetch(ctx, track)

	c.mu.Lock()
	delete(c.inflight, key)
	if err == nil {
		c.cache[key] = lyr
		c.order = append(c.order, key)
		for len(c.order) > maxCacheSize {
			oldest := c.order[0]
			c.order = c.order[1:]
			delete(c.cache, oldest)
		}
	}
	c.mu.Unlock()

	cl.val, cl.err = lyr, err
	cl.wg.Done()
	return lyr, err
}

func (c *Client) fetch(ctx context.Context, track source.Track) (*source.Lyrics, error) {
	q := url.Values{}
	q.Set("track_name", track.Name)
	q.Set("artist_name", track.Artist)
	if track.Album != "" {
		q.Set("album_name", track.Album)
	}
	if track.Duration > 0 {
		q.Set("duration", strconv.Itoa(int(track.Duration.Round(time.Second).Seconds())))
	}
	endpoint := c.baseURL + "/api/get?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lrclib request: %w", err)
	}
	defer resp.Body.Close()

	// No match for this track is a normal outcome, not an error.
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lrclib status %d", resp.StatusCode)
	}

	var body lrclibResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("lrclib decode: %w", err)
	}
	return fromResponse(body), nil
}

// fromResponse converts an lrclib payload into source.Lyrics, preferring synced
// lyrics over plain. Returns nil when there is nothing to show.
func fromResponse(body lrclibResponse) *source.Lyrics {
	if body.Instrumental {
		return &source.Lyrics{Lines: []source.LyricLine{{Text: "♪ instrumental ♪"}}}
	}
	if synced := ParseLRC(body.SyncedLyrics); len(synced) > 0 {
		return &source.Lyrics{Synced: true, Lines: synced, Plain: body.PlainLyrics}
	}
	if plain := strings.TrimRight(body.PlainLyrics, "\n"); plain != "" {
		lines := strings.Split(plain, "\n")
		out := make([]source.LyricLine, len(lines))
		for i, l := range lines {
			out[i] = source.LyricLine{Text: l}
		}
		return &source.Lyrics{Lines: out, Plain: plain}
	}
	return nil
}

// lrcLineRE matches one [mm:ss.xx] timestamp tag. A line may carry several.
var lrcLineRE = regexp.MustCompile(`\[(\d+):(\d{1,2})(?:[.:](\d{1,3}))?\]`)

// ParseLRC parses LRC-formatted synced lyrics into timestamped lines, sorted by
// time. Metadata tags (e.g. [ar:...]) and lines without a timestamp are
// skipped. Lines with empty text are kept — they represent musical gaps.
func ParseLRC(s string) []source.LyricLine {
	if s == "" {
		return nil
	}
	var lines []source.LyricLine
	for _, raw := range strings.Split(s, "\n") {
		matches := lrcLineRE.FindAllStringSubmatchIndex(raw, -1)
		if len(matches) == 0 {
			continue
		}
		// Text is everything after the final timestamp tag.
		text := strings.TrimSpace(raw[matches[len(matches)-1][1]:])
		for _, m := range matches {
			minStr := raw[m[2]:m[3]]
			secStr := raw[m[4]:m[5]]
			minVal, err1 := strconv.Atoi(minStr)
			secVal, err2 := strconv.Atoi(secStr)
			if err1 != nil || err2 != nil {
				continue
			}
			d := time.Duration(minVal)*time.Minute + time.Duration(secVal)*time.Second
			if m[6] >= 0 { // fractional part present
				fracStr := raw[m[6]:m[7]]
				d += fracDuration(fracStr)
			}
			lines = append(lines, source.LyricLine{Time: d, Text: text})
		}
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Time < lines[j].Time })
	return lines
}

// fracDuration converts a fractional-seconds string ("5", "12", "120") into a
// Duration, interpreting it as tenths/hundredths/thousandths by digit count.
func fracDuration(frac string) time.Duration {
	n, err := strconv.Atoi(frac)
	if err != nil {
		return 0
	}
	switch len(frac) {
	case 1:
		return time.Duration(n) * 100 * time.Millisecond
	case 2:
		return time.Duration(n) * 10 * time.Millisecond
	default:
		return time.Duration(n) * time.Millisecond
	}
}
