//go:build windows

package main

// BrowserHost owns the built-in browser panel: a second WebView2 controller
// rendered in the right-side panel area, with the Wails main webview shrunk to
// make room (non-overlapping layout — overlay z-order proved unreliable in the
// Phase-0 spike).
//
// Threading model (learned the hard way in the spike):
//   - WebView2 posts its COM callbacks and window messages to the thread that
//     created the environment, so the panel webview is created AND pumped on a
//     dedicated locked OS thread (hostLoop). All panel operations are
//     serialized through that thread.
//   - The Wails main webview was created on the winc main thread. Shrinking it
//     touches its COM controller, so those calls are marshalled to the main
//     thread via SetWindowSubclass + PostMessage.
//   - The process must be DPI aware (RAW_PIXELS bounds mode), and controller
//     bounds must be set explicitly — WebView2 leaves them 0,0,0,0 on creation.
//
// Lazy loading: nothing is created until Open() (the user clicks the panel
// "+"), and Close() destroys the controller so no browser resources linger.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/wailsapp/go-webview2/pkg/edge"
)

const (
	// panelChromeHeightPx is the physical height reserved for the in-page
	// chrome bar (address bar + buttons) injected on top of every page.
	panelChromeHeightPx = 44

	// wmBrowserTask is posted to the main window to run a task on the winc
	// main thread (where the Wails main webview's COM controller lives).
	wmBrowserTask = 0x8001 // WM_APP + 1

	// wmBrowserSize is posted to the main window when the panel asks for a
	// client-size refresh (window moved/resized).
	wmBrowserSize = 0x8002
)

// BrowserHost is the desktop-side owner of the browser panel.
type BrowserHost struct {
	app *App

	// hostLoop serialization: all panel-webview work runs on one locked OS
	// thread (see file comment). tasks carries work items; the loop pumps
	// window messages in between.
	tasks chan hostTask

	// mainThread marshalling for the Wails main webview (see file comment).
	mainHWND uintptr

	mu          sync.Mutex
	open        bool
	loopAlive   bool
	closed      chan struct{}
	panelWidth  int32
	clientW     int32
	clientH     int32
	chromium    *edge.Chromium
	state       BrowserHostState
	profileDir  string
	refIndex    *refIndex // snapshot [ref] -> backend DOM node id (core)
	mainBounds  edge.Rect     // saved main-webview bounds while the panel is open
	mainBoundsOK bool
}

type hostTask struct {
	fn   func()
	done chan struct{}
}

// newBrowserHost wires the panel host to the desktop app.
func newBrowserHost(app *App) *BrowserHost {
	return &BrowserHost{
		app:       app,
		tasks:     make(chan hostTask, 64),
		closed:    make(chan struct{}, 1),
		loopAlive: true,
		panelWidth: 520,
	}
}

// BrowserEnabled reports the user preference (Settings > Browser > control).
func (h *BrowserHost) browserEnabled() bool {
	return h.app.browserControlEnabled()
}

// profileRoot returns the WebView2 user-data folder for the panel browser
// (independent login state, like ZCode's persist: partition).
func (h *BrowserHost) profileRoot() string {
	if h.profileDir == "" {
		home := configHomeDir()
		h.profileDir = filepath.Join(home, "browser-profile")
	}
	return h.profileDir
}

// --- thread plumbing -------------------------------------------------------

// submit runs fn on the hostLoop thread and waits for completion. Must not be
// called from the hostLoop thread itself (callbacks only post events).
func (h *BrowserHost) submit(fn func()) {
	done := make(chan struct{})
	h.tasks <- hostTask{fn: fn, done: done}
	<-done
}

// hostLoop is the locked OS thread that owns the panel webview: it creates the
// environment/controller, pumps its messages, and executes submitted tasks.
func (h *BrowserHost) hostLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := h.createPanelWebview(); err != nil {
		h.app.emitRuntimeEvent("browser:state", BrowserHostState{Open: false, Error: err.Error()})
		h.mu.Lock()
		h.open = false
		h.mu.Unlock()
		return
	}

	for {
		select {
		case t := <-h.tasks:
			t.fn()
			close(t.done)
		case <-h.closed:
			h.destroyPanelWebview()
			return
		default:
		}
		pumpMessagesOnce()
		time.Sleep(2 * time.Millisecond)
	}
}

// mainThreadTask runs fn on the winc main thread via window subclassing.
func (h *BrowserHost) mainThreadTask(fn func()) {
	if h.mainHWND == 0 {
		return
	}
	postMainTask(h.mainHWND, fn)
}

// --- lifecycle -------------------------------------------------------------

// Open lazily creates the panel webview and shrinks the main webview.
func (h *BrowserHost) Open(panelWidth int) error {
	h.mu.Lock()
	if h.open {
		h.mu.Unlock()
		return nil
	}
	if panelWidth >= 300 && panelWidth <= 1200 {
		h.panelWidth = int32(panelWidth)
	}
	h.open = true
	h.mu.Unlock()

	h.ensureMainWindow()
	go h.hostLoop()
	h.syncLayout()
	h.emitState()
	return nil
}

// Close destroys the panel webview and restores the main webview.
func (h *BrowserHost) Close() {
	h.mu.Lock()
	if !h.open {
		h.mu.Unlock()
		return
	}
	h.open = false
	h.mu.Unlock()

	select {
	case h.closed <- struct{}{}:
	default:
	}
	// Wait for the loop to tear down the webview.
	deadline := time.After(3 * time.Second)
	for {
		h.mu.Lock()
		done := !h.loopAlive
		h.mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			h.mu.Lock()
			h.loopAlive = false
			h.mu.Unlock()
			return
		case <-time.After(10 * time.Millisecond):
		}
	}
	h.restoreMainWebview()
	h.emitState()
}

// --- layout ----------------------------------------------------------------

// syncLayout recomputes both webviews' bounds from the current client size.
// Must run after Open, on resize, and on panel-width drag.
func (h *BrowserHost) syncLayout() {
	h.mu.Lock()
	cw, ch, panelW := h.clientW, h.clientH, h.panelWidth
	chromium := h.chromium
	h.mu.Unlock()
	if cw <= 0 || ch <= 0 {
		return
	}
	mainW := cw - panelW
	if mainW < 200 {
		mainW = 200
		panelW = cw - mainW
	}

	// Panel webview bounds (hostLoop thread owns it).
	h.submit(func() {
		if chromium != nil {
			if ctrl := chromium.GetController(); ctrl != nil {
				_ = ctrl.PutBounds(edge.Rect{Left: mainW, Top: 0, Right: cw, Bottom: ch})
			}
		}
	})

	// Main webview bounds (winc main thread owns it).
	h.mainThreadTask(func() {
		main := edge.MainChromium()
		if main == nil {
			return
		}
		if ctrl := main.GetController(); ctrl != nil {
			_ = ctrl.PutBounds(edge.Rect{Left: 0, Top: 0, Right: mainW, Bottom: ch})
		}
	})
}

// SetPanelWidth updates the panel width after a frontend drag and re-lays out.
func (h *BrowserHost) SetPanelWidth(width int) {
	h.mu.Lock()
	if width >= 300 && width <= 1200 {
		h.panelWidth = int32(width)
	}
	h.mu.Unlock()
	h.syncLayout()
}

// restoreMainWebview returns the main webview to full-window bounds.
func (h *BrowserHost) restoreMainWebview() {
	h.mainThreadTask(func() {
		main := edge.MainChromium()
		if main == nil {
			return
		}
		if ctrl := main.GetController(); ctrl != nil {
			h.mu.Lock()
			cw, ch := h.clientW, h.clientH
			h.mu.Unlock()
			if cw > 0 && ch > 0 {
				_ = ctrl.PutBounds(edge.Rect{Left: 0, Top: 0, Right: cw, Bottom: ch})
			}
		}
	})
}

// onMainSize is called (on the winc main thread) when the app window resizes.
func (h *BrowserHost) onMainSize(cw, ch int32) {
	h.mu.Lock()
	h.clientW, h.clientH = cw, ch
	open := h.open
	h.mu.Unlock()
	if open {
		h.syncLayout()
	}
}

// --- navigation ------------------------------------------------------------

func (h *BrowserHost) Navigate(url string) {
	h.submit(func() {
		if h.chromium != nil {
			h.chromium.Navigate(url)
		}
	})
}

func (h *BrowserHost) Back() {
	h.submit(func() {
		if h.chromium != nil {
			_ = h.chromium.EvalJS(`history.back()`)
		}
	})
}

func (h *BrowserHost) Forward() {
	h.submit(func() {
		if h.chromium != nil {
			_ = h.chromium.EvalJS(`history.forward()`)
		}
	})
}

func (h *BrowserHost) Reload() {
	h.submit(func() {
		if h.chromium != nil {
			_ = h.chromium.Reload()
		}
	})
}

// Address returns the current URL and title.
func (h *BrowserHost) Address() (string, string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.state.URL, h.state.Title, h.state.Open
}

// --- page messages (chrome bar -> Go) --------------------------------------

// onPageMessage handles postMessage from the injected chrome bar.
func (h *BrowserHost) onPageMessage(message string) {
	var m struct {
		Kind string `json:"kind"`
		Act  string `json:"act"`
		URL  string `json:"url"`
	}
	if json.Unmarshal([]byte(message), &m) != nil || m.Kind != "bar" {
		return
	}
	switch m.Act {
	case "navigate":
		if m.URL != "" {
			h.Navigate(m.URL)
		}
	case "back":
		h.Back()
	case "forward":
		h.Forward()
	case "reload":
		h.Reload()
	case "close":
		h.Close()
	}
}

// onNavigationCompleted updates state after a page load.
func (h *BrowserHost) onNavigationCompleted(url string) {
	title := ""
	if h.chromium != nil {
		if w2, err := h.chromium.Webview2().QueryInterface2(); err == nil {
			title, _ = w2.GetDocumentTitle()
		}
	}
	h.mu.Lock()
	h.state.URL = url
	h.state.Title = title
	h.state.Loading = false
	h.state.Open = true
	h.mu.Unlock()
	h.emitState()
}

// emitState pushes the current panel state to the frontend.
func (h *BrowserHost) emitState() {
	h.mu.Lock()
	st := h.state
	h.mu.Unlock()
	h.app.emitRuntimeEvent("browser:state", st)
}

// --- webview plumbing ------------------------------------------------------

// createPanelWebview builds the panel Chromium on the hostLoop thread.
func (h *BrowserHost) createPanelWebview() error {
	h.mu.Lock()
	panelWidth := h.panelWidth
	profile := h.profileRoot()
	h.mu.Unlock()

	if err := os.MkdirAll(profile, 0o755); err != nil {
		return fmt.Errorf("browser profile dir: %w", err)
	}

	w := edge.NewChromium()
	w.DataPath = profile
	w.SetErrorCallback(func(err error) { h.app.emitRuntimeEvent("browser:error", err.Error()) })
	w.MessageCallback = func(message string, _ *edge.ICoreWebView2, _ *edge.ICoreWebView2WebMessageReceivedEventArgs) {
		h.onPageMessage(message)
	}
	w.NavigationCompletedCallback = func(sender *edge.ICoreWebView2, _ *edge.ICoreWebView2NavigationCompletedEventArgs) {
		src, err := sender.GetSource()
		if err != nil {
			src = ""
		}
		h.onNavigationCompleted(src)
	}
	w.ProcessFailedCallback = func(_ *edge.ICoreWebView2, _ *edge.ICoreWebView2ProcessFailedEventArgs) {
		h.app.emitRuntimeEvent("browser:error", "browser process failed; close and reopen the panel")
	}

	if !w.Embed(h.mainHWND) {
		return errors.New("panel webview Embed failed")
	}
	// Bounds are 0,0,0,0 after creation — set them explicitly (spike finding).
	h.mu.Lock()
	cw, ch := h.clientW, h.clientH
	h.mu.Unlock()
	if cw > 0 && ch > 0 {
		mainW := cw - panelWidth
		if ctrl := w.GetController(); ctrl != nil {
			_ = ctrl.PutBounds(edge.Rect{Left: mainW, Top: 0, Right: cw, Bottom: ch})
		}
	}
	// Injected chrome bar on every page (address bar + controls).
	w.Init(panelChromeBarJS)
	if err := w.Show(); err != nil {
		return fmt.Errorf("panel webview Show: %w", err)
	}
	w.NavigateToString(panelWelcomePage)

	h.mu.Lock()
	h.chromium = w
	h.state.Open = true
	h.mu.Unlock()
	return nil
}

// destroyPanelWebview tears the panel webview down on the hostLoop thread.
func (h *BrowserHost) destroyPanelWebview() {
	h.mu.Lock()
	w := h.chromium
	h.chromium = nil
	h.state = BrowserHostState{}
	h.loopAlive = false
	h.mu.Unlock()
	if w != nil {
		w.Destroy()
	}
}

// ensureMainWindow locates the Wails main window and installs the subclass
// that marshals main-thread tasks and tracks client size.
func (h *BrowserHost) ensureMainWindow() {
	if h.mainHWND != 0 {
		return
	}
	hwnd := currentProcessTopLevelWindow()
	if hwnd == 0 {
		return
	}
	h.mainHWND = hwnd
	h.installSubclass(hwnd)
	var cr bhRect
	if getClientRect(hwnd, &cr) {
		h.onMainSize(cr.right-cr.left, cr.bottom-cr.top)
	}
}

// --- data cleanup ----------------------------------------------------------

// isOpen reports whether the panel webview currently exists.
func (h *BrowserHost) isOpen() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.open && h.chromium != nil
}

// clearCache removes HTTP cache, Cache Storage, and Service Workers while
// keeping cookies and local site data (Settings > Browser > 清除内置浏览器缓存).
// The panel must be closed so the browser process does not hold file locks.
func (h *BrowserHost) clearCache() error {
	if h.isOpen() {
		return errors.New("请先关闭浏览器面板，再清除缓存")
	}
	eb := filepath.Join(h.profileRoot(), "EBWebView")
	var failed []string
	for _, dir := range []string{"Cache", "Code Cache", "GPUCache", "Service Worker", "SharedDictionary", "DawnCache", "ShaderCache", "DawnGraphiteCache", "GraphiteDawnCache"} {
		if err := os.RemoveAll(filepath.Join(eb, dir)); err != nil {
			failed = append(failed, dir)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("部分缓存目录清理失败: %v", failed)
	}
	return nil
}

// clearAllData removes cookies, site data, and cache entirely
// (Settings > Browser > 清除全部浏览器数据, irreversible).
func (h *BrowserHost) clearAllData() error {
	if h.isOpen() {
		return errors.New("请先关闭浏览器面板，再清除浏览器数据")
	}
	return os.RemoveAll(filepath.Join(h.profileRoot(), "EBWebView"))
}
