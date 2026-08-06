//go:build !windows

package main

// BrowserHost on macOS/Linux wraps the chromeCDP driver (browser_chrome.go):
// a headless system Chrome driven over CDP, mirrored into the main webview as
// a screenshot feed plus a React address bar. Windows uses the WebView2
// backend instead (browserhost_windows.go) for a live panel.

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"
)

type BrowserHost struct {
	app *App

	mu           sync.Mutex
	open         bool
	state        BrowserHostState
	profileDir   string
	cdp          *chromeCDP
	refIndex     *refIndex
	closeCh      chan struct{}
	screenshotWg sync.WaitGroup
}

func newBrowserHost(app *App) *BrowserHost {
	return &BrowserHost{
		app:     app,
		closeCh: make(chan struct{}),
	}
}

// browserEnabled reports the user preference (Settings > Browser).
func (h *BrowserHost) browserEnabled() bool {
	return h.app.browserControlEnabled()
}

func (h *BrowserHost) profileRoot() string {
	if h.profileDir == "" {
		h.profileDir = filepath.Join(configHomeDir(), "browser-profile")
	}
	return h.profileDir
}

// Open lazily launches the headless Chrome and connects its CDP endpoint.
func (h *BrowserHost) Open(_ int) error {
	h.mu.Lock()
	if h.open {
		h.mu.Unlock()
		return nil
	}
	h.open = true
	h.mu.Unlock()

	c := newChromeCDP(h.profileRoot())
	if err := c.Start(); err != nil {
		h.mu.Lock()
		h.open = false
		h.mu.Unlock()
		return err
	}
	h.mu.Lock()
	h.cdp = c
	h.mu.Unlock()
	h.emitState()
	go h.screenshotLoop(c)
	go h.stateLoop(c)
	return nil
}

func (h *BrowserHost) isOpen() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.open
}

// Close kills Chrome and clears panel state.
func (h *BrowserHost) Close() {
	h.mu.Lock()
	if !h.open {
		h.mu.Unlock()
		return
	}
	h.open = false
	close(h.closeCh)
	c := h.cdp
	h.cdp = nil
	h.state = BrowserHostState{}
	h.mu.Unlock()

	if c != nil {
		c.Close()
	}
	h.screenshotWg.Wait()
	h.emitState()
}

func (h *BrowserHost) Navigate(url string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if c := h.current(); c != nil {
		_ = c.Navigate(ctx, url)
	}
}

func (h *BrowserHost) Back() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if c := h.current(); c != nil {
		_ = c.Back(ctx)
	}
}

func (h *BrowserHost) Forward() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if c := h.current(); c != nil {
		_ = c.Forward(ctx)
	}
}

func (h *BrowserHost) Reload() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if c := h.current(); c != nil {
		_ = c.Reload(ctx)
	}
}

// Address returns the current URL and title.
func (h *BrowserHost) Address() (string, string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state.URL, h.state.Title, h.state.Open
}

// Snapshot / Click / TypeText / Press / Scroll / Screenshot delegate to the
// platform-independent core through the Chrome transport.
func (h *BrowserHost) Snapshot(ctx context.Context) (string, error) {
	c := h.current()
	if c == nil {
		return "", errors.New("浏览器未打开")
	}
	tree, ri, err := c.Snapshot(ctx)
	if err != nil {
		return "", err
	}
	h.mu.Lock()
	h.refIndex = ri
	h.mu.Unlock()
	return tree, nil
}

func (h *BrowserHost) Click(ctx context.Context, ref int) error {
	c := h.current()
	if c == nil {
		return errors.New("浏览器未打开")
	}
	return c.Click(ctx, h.refs(), ref)
}

func (h *BrowserHost) TypeText(ctx context.Context, ref int, text string) error {
	c := h.current()
	if c == nil {
		return errors.New("浏览器未打开")
	}
	return c.TypeText(ctx, h.refs(), ref, text)
}

func (h *BrowserHost) Press(ctx context.Context, key string) error {
	c := h.current()
	if c == nil {
		return errors.New("浏览器未打开")
	}
	return c.Press(ctx, key)
}

func (h *BrowserHost) Scroll(ctx context.Context, dx, dy int) error {
	c := h.current()
	if c == nil {
		return errors.New("浏览器未打开")
	}
	return c.Scroll(ctx, dx, dy)
}

func (h *BrowserHost) Screenshot(ctx context.Context) ([]byte, error) {
	c := h.current()
	if c == nil {
		return nil, errors.New("浏览器未打开")
	}
	return c.Screenshot(ctx)
}

func (h *BrowserHost) current() *chromeCDP {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cdp
}

func (h *BrowserHost) refs() *refIndex {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.refIndex == nil {
		h.refIndex = newRefIndex()
	}
	return h.refIndex
}

// screenshotLoop pushes a PNG feed of the page to the frontend panel.
func (h *BrowserHost) screenshotLoop(c *chromeCDP) {
	h.screenshotWg.Add(1)
	defer h.screenshotWg.Done()
	tick := time.NewTicker(1200 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-h.closeCh:
			return
		case <-tick.C:
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			png, err := c.Screenshot(ctx)
			cancel()
			if err != nil {
				continue
			}
			h.app.emitRuntimeEvent("browser:screenshot", base64DataURL(png))
		}
	}
}

// stateLoop polls URL/title so the React address bar stays in sync.
func (h *BrowserHost) stateLoop(c *chromeCDP) {
	tick := time.NewTicker(800 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-h.closeCh:
			return
		case <-tick.C:
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			url, title, err := c.PageState(ctx)
			cancel()
			if err != nil {
				continue
			}
			h.mu.Lock()
			h.state.URL = url
			h.state.Title = title
			h.state.Open = true
			h.mu.Unlock()
			h.emitState()
		}
	}
}

func (h *BrowserHost) emitState() {
	h.mu.Lock()
	st := h.state
	h.mu.Unlock()
	h.app.emitRuntimeEvent("browser:state", st)
}

// SetPanelWidth is a no-op on non-Windows: the panel is a React layout.
func (h *BrowserHost) SetPanelWidth(int) {}

// --- data cleanup (Chrome profile layout differs from WebView2) -------------

func (h *BrowserHost) clearCache() error {
	if h.isOpen() {
		return errors.New("请先关闭浏览器面板，再清除缓存")
	}
	return clearChromeProfileCache(h.profileRoot())
}

func (h *BrowserHost) clearAllData() error {
	if h.isOpen() {
		return errors.New("请先关闭浏览器面板，再清除浏览器数据")
	}
	return removeAll(h.profileRoot())
}
