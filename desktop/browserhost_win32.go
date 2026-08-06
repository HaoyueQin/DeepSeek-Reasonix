//go:build windows

package main

// Win32 helpers for the browser panel: a non-blocking message pump for the
// hostLoop thread, window-subclass marshalling to the winc main thread, and
// tiny rect/window queries. Kept separate from browserhost_windows.go so the
// panel lifecycle reads as plain Go.
//
// Pointer safety: Win32 messages carry uintptr payloads, so instead of
// round-tripping Go pointers through lParam (which go vet flags as unsafe),
// main-thread tasks travel as numeric ids into a process-global registry, and
// the BrowserHost itself is a process singleton reached directly.

import (
	"sync"
	"syscall"
	"unsafe"
)

var (
	bhUser32 = syscall.NewLazyDLL("user32.dll")
	bhComctl = syscall.NewLazyDLL("comctl32.dll")

	bhPeekMessageW      = bhUser32.NewProc("PeekMessageW")
	bhTranslateMessage  = bhUser32.NewProc("TranslateMessage")
	bhDispatchMessageW  = bhUser32.NewProc("DispatchMessageW")
	bhPostMessageW      = bhUser32.NewProc("PostMessageW")
	bhGetClientRect     = bhUser32.NewProc("GetClientRect")
	bhGetWindowRect     = bhUser32.NewProc("GetWindowRect")
	bhSetWindowSubclass = bhComctl.NewProc("SetWindowSubclass")
	bhDefSubclassProc   = bhComctl.NewProc("DefSubclassProc")
)

const (
	bhPMRemove = 0x0001
	bhWMQuit   = 0x0012
	bhWM_SIZE  = 0x0005
)

type bhMsg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type bhRect struct {
	left, top, right, bottom int32
}

// pumpMessagesOnce drains all pending window messages on the current thread.
func pumpMessagesOnce() {
	var m bhMsg
	r, _, _ := bhPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, bhPMRemove)
	for r != 0 {
		if m.message == bhWMQuit {
			return
		}
		bhTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		bhDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		r, _, _ = bhPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, bhPMRemove)
	}
}

// postMessage posts a message to a window's queue (thread-safe).
func postMessage(hwnd uintptr, msg, wParam, lParam uintptr) bool {
	r, _, _ := bhPostMessageW.Call(hwnd, msg, wParam, lParam)
	return r != 0
}

// getClientRect returns the client-area size of hwnd.
func getClientRect(hwnd uintptr, r *bhRect) bool {
	ok, _, _ := bhGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(r)))
	return ok != 0
}

// mainThreadTask carries a closure across the PostMessage boundary.
type mainThreadTask struct {
	fn   func()
	done chan struct{}
}

// bhTaskRegistry maps numeric message ids to pending main-thread tasks. The
// ids cross the message boundary (never raw pointers — vet-safe).
var (
	bhTaskMu sync.Mutex
	bhTasks  = map[uintptr]*mainThreadTask{}
	bhTaskID uintptr
)

// postMainTask queues fn for the winc main thread and blocks until it runs.
func postMainTask(hwnd uintptr, fn func()) {
	t := &mainThreadTask{fn: fn, done: make(chan struct{})}
	bhTaskMu.Lock()
	bhTaskID++
	id := bhTaskID
	bhTasks[id] = t
	bhTaskMu.Unlock()

	postMessage(hwnd, wmBrowserTask, 0, id)
	<-t.done

	bhTaskMu.Lock()
	delete(bhTasks, id)
	bhTaskMu.Unlock()
}

// globalBrowserHost is the process singleton the subclass reaches on the main
// thread (desktop runs exactly one BrowserHost).
var globalBrowserHost *BrowserHost

// subclassProc intercepts wmBrowserTask (run the task, signal done) and
// wmBrowserSize (client-size refresh) on the winc main thread.
func subclassProc(hwnd uintptr, msg uint32, wParam, lParam uintptr, _ uintptr, _ uintptr) uintptr {
	switch msg {
	case wmBrowserTask:
		bhTaskMu.Lock()
		t := bhTasks[lParam]
		bhTaskMu.Unlock()
		if t != nil {
			t.fn()
			close(t.done)
		}
		return 0
	case wmBrowserSize:
		if globalBrowserHost != nil {
			var cr bhRect
			if getClientRect(hwnd, &cr) {
				globalBrowserHost.onMainSize(cr.right-cr.left, cr.bottom-cr.top)
			}
		}
		return 0
	}
	ret, _, _ := bhDefSubclassProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

// installSubclass attaches the browser-panel subclass to the main window.
// The subclass runs on the winc main thread, which owns the Wails webview's
// COM controller — that is why panel code routes main-webview bounds changes
// through PostMessage(wmBrowserTask) instead of calling COM directly.
func (h *BrowserHost) installSubclass(hwnd uintptr) {
	globalBrowserHost = h
	bhSetWindowSubclass.Call(
		hwnd,
		syscall.NewCallback(subclassProc),
		uintptr(1), // subclass id
		0,          // ref data unused (singleton via globalBrowserHost)
	)
	// Ask for the current client size once (also covers the Open path where
	// the window never resizes after startup).
	postMessage(hwnd, wmBrowserSize, 0, 0)
}
