//go:build darwin

package media

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// macOS: MPRemoteCommandCenter delivers the media keys and Control Center's
// buttons, and MPNowPlayingInfoCenter shows what's playing. Both need an
// AppKit event loop on the main thread (no window or Dock icon), which
// RunMain provides; every call into them is made on that thread.

func init() { runtime.LockOSThread() } // keep main on the main thread for RunMain

// MPNowPlayingPlaybackState values.
const (
	stateUnknown = 0
	statePlaying = 1
	statePaused  = 2
)

var (
	loadOnce sync.Once
	loadErr  error
	mpLib    uintptr
	// Autorelease pools for calls made off the main thread's run loop,
	// which has its own.
	poolPush func() uintptr
	poolPop  func(uintptr)
	// hasArtwork: MPMediaItemArtwork's initWithImage: (declared for iOS)
	// exists on this macOS.
	hasArtwork bool

	// Calls queued for the main thread, run by the WaxonMain helper class.
	mainMu    sync.Mutex
	mainQueue []func()
	mainObj   objc.ID
	selRun    = objc.RegisterName("run:")
	selPerf   = objc.RegisterName("performSelectorOnMainThread:withObject:waitUntilDone:")
)

func load() error {
	loadOnce.Do(func() {
		libobjc, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_GLOBAL)
		if err != nil {
			loadErr = err
			return
		}
		purego.RegisterLibFunc(&poolPush, libobjc, "objc_autoreleasePoolPush")
		purego.RegisterLibFunc(&poolPop, libobjc, "objc_autoreleasePoolPop")
		if _, err = purego.Dlopen("/System/Library/Frameworks/AppKit.framework/AppKit", purego.RTLD_GLOBAL); err != nil {
			loadErr = err
			return
		}
		lib, err := purego.Dlopen("/System/Library/Frameworks/MediaPlayer.framework/MediaPlayer", purego.RTLD_GLOBAL)
		if err != nil {
			loadErr = err
			return
		}
		mpLib = lib
		cls, err := objc.RegisterClass("WaxonMain", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{{
			Cmd: selRun,
			Fn: func(objc.ID, objc.SEL, objc.ID) {
				mainMu.Lock()
				q := mainQueue
				mainQueue = nil
				mainMu.Unlock()
				for _, f := range q {
					f()
				}
			},
		}})
		if err != nil {
			loadErr = err
			return
		}
		mainObj = objc.ID(cls).Send(objc.RegisterName("new"))
		hasArtwork = objc.Send[bool](objc.ID(objc.GetClass("MPMediaItemArtwork")),
			objc.RegisterName("instancesRespondToSelector:"), objc.RegisterName("initWithImage:"))
	})
	return loadErr
}

// onMain runs f on the main thread; with wait, before returning.
func onMain(f func(), wait bool) {
	mainMu.Lock()
	mainQueue = append(mainQueue, f)
	mainMu.Unlock()
	withPool(func() { mainObj.Send(selPerf, selRun, objc.ID(0), wait) })
}

// withPool runs f inside an autorelease pool on one OS thread, so objects
// Objective-C autoreleases during it are freed when it ends (a goroutine's
// thread has no pool of its own).
func withPool(f func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := poolPush()
	defer poolPop(pool)
	f()
}

func runMain(fn func() int) int {
	if err := load(); err != nil {
		return fn() // no AppKit: run without media controls
	}
	var app objc.ID
	withPool(func() {
		app = objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
		app.Send(objc.RegisterName("setActivationPolicy:"), 1) // accessory: no Dock icon or menu bar
	})
	go func() {
		// The event loop below never returns; the program ends here.
		os.Exit(fn())
	}()
	app.Send(objc.RegisterName("run"))
	return 0
}

type darwinController struct {
	mu      sync.Mutex
	pending *State
	artURL  string
	artwork objc.ID
	blocks  []objc.Block
	closed  bool
}

func start(onEvent func(Event)) (Controller, error) {
	if err := load(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	c := &darwinController{}
	onMain(func() {
		center := objc.ID(objc.GetClass("MPRemoteCommandCenter")).Send(objc.RegisterName("sharedCommandCenter"))
		on := func(sel string, handle func(event objc.ID)) {
			b := objc.NewBlock(func(_ objc.Block, event objc.ID) int {
				handle(event)
				return 0 // MPRemoteCommandHandlerStatusSuccess
			})
			c.blocks = append(c.blocks, b)
			cmd := center.Send(objc.RegisterName(sel))
			cmd.Send(objc.RegisterName("setEnabled:"), true)
			cmd.Send(objc.RegisterName("addTargetWithHandler:"), b)
		}
		send := func(cmd Command) func(objc.ID) { return func(objc.ID) { onEvent(Event{Command: cmd}) } }
		on("togglePlayPauseCommand", send(Toggle))
		on("playCommand", send(Play))
		on("pauseCommand", send(Pause))
		on("nextTrackCommand", send(Next))
		on("previousTrackCommand", send(Previous))
		on("changePlaybackPositionCommand", func(event objc.ID) {
			secs := objc.Send[float64](event, objc.RegisterName("positionTime"))
			onEvent(Event{Command: Seek, Position: seconds(secs)})
		})
	}, true)
	return c, nil
}

// Update shows s in Now Playing. Updates are coalesced: only the newest
// state is applied.
func (c *darwinController) Update(s State) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	first := c.pending == nil
	c.pending = &s
	c.mu.Unlock()
	if first {
		onMain(c.apply, false)
	}
}

func (c *darwinController) apply() {
	c.mu.Lock()
	s := c.pending
	c.pending = nil
	c.mu.Unlock()
	if s == nil {
		return
	}
	info := objc.ID(objc.GetClass("MPNowPlayingInfoCenter")).Send(objc.RegisterName("defaultCenter"))
	if s.Empty {
		info.Send(objc.RegisterName("setNowPlayingInfo:"), objc.ID(0))
		info.Send(objc.RegisterName("setPlaybackState:"), stateUnknown)
		return
	}
	dict := objc.ID(objc.GetClass("NSMutableDictionary")).Send(objc.RegisterName("dictionary"))
	put := func(key string, v objc.ID) {
		if k := symbol(key); k != 0 && v != 0 {
			dict.Send(objc.RegisterName("setObject:forKey:"), v, k)
		}
	}
	put("MPMediaItemPropertyTitle", nsString(s.Title))
	put("MPMediaItemPropertyArtist", nsString(s.Artist))
	put("MPMediaItemPropertyAlbumTitle", nsString(s.Album))
	put("MPMediaItemPropertyPlaybackDuration", nsNumber(s.Duration.Seconds()))
	put("MPNowPlayingInfoPropertyElapsedPlaybackTime", nsNumber(s.Position.Seconds()))
	rate := 0.0
	if s.Playing {
		rate = 1
	}
	put("MPNowPlayingInfoPropertyPlaybackRate", nsNumber(rate))
	var replaced objc.ID
	if s.ArtworkURL != c.artURL {
		replaced = c.artwork
		c.artURL, c.artwork = s.ArtworkURL, artwork(s.Artwork)
	}
	// Without artwork macOS shows the program's icon, a bare terminal for
	// an unbundled binary, so a record stands in until the cover loads.
	art := c.artwork
	if art == 0 {
		art = placeholderArtwork()
	}
	put("MPMediaItemPropertyArtwork", art)
	info.Send(objc.RegisterName("setNowPlayingInfo:"), dict)
	if replaced != 0 {
		// The new dictionary took the old one's place; ours is the last
		// reference to the old cover.
		replaced.Send(objc.RegisterName("release"))
	}
	state := statePaused
	if s.Playing {
		state = statePlaying
	}
	info.Send(objc.RegisterName("setPlaybackState:"), state)
}

func (c *darwinController) Close() {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	onMain(func() {
		info := objc.ID(objc.GetClass("MPNowPlayingInfoCenter")).Send(objc.RegisterName("defaultCenter"))
		info.Send(objc.RegisterName("setNowPlayingInfo:"), objc.ID(0))
		info.Send(objc.RegisterName("setPlaybackState:"), stateUnknown)
	}, true)
}

// artwork wraps img as an MPMediaItemArtwork (0 when there's none).
func artwork(img image.Image) objc.ID {
	if img == nil || !hasArtwork {
		return 0
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil || buf.Len() == 0 {
		return 0
	}
	b := buf.Bytes()
	// NSData copies the bytes before this returns.
	data := objc.ID(objc.GetClass("NSData")).Send(objc.RegisterName("dataWithBytes:length:"), unsafe.Pointer(&b[0]), uint(len(b))) //nolint:gosec // a pointer to bytes Objective-C copies
	nsImage := objc.ID(objc.GetClass("NSImage")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithData:"), data)
	if nsImage == 0 {
		return 0
	}
	art := objc.ID(objc.GetClass("MPMediaItemArtwork")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithImage:"), nsImage)
	nsImage.Send(objc.RegisterName("release")) // the artwork holds it now
	runtime.KeepAlive(b)
	return art
}

var placeholder objc.ID

// placeholderArtwork is the record shown while a cover loads (made once,
// kept for the program's life).
func placeholderArtwork() objc.ID {
	if placeholder == 0 {
		placeholder = artwork(recordImage(256))
	}
	return placeholder
}

// symbol reads one of MediaPlayer's NSString key constants.
func symbol(name string) objc.ID {
	p, err := purego.Dlsym(mpLib, name)
	if err != nil || p == 0 {
		return 0
	}
	// p is the address of the constant's NSString pointer.
	return **(**objc.ID)(unsafe.Pointer(&p)) //nolint:gosec // reading a framework constant by its address
}

func nsString(s string) objc.ID {
	if s == "" {
		return 0
	}
	return objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), s)
}

func nsNumber(f float64) objc.ID {
	return objc.ID(objc.GetClass("NSNumber")).Send(objc.RegisterName("numberWithDouble:"), f)
}
