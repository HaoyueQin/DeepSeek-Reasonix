//go:build windows

package main

// WebView2 transport for the platform-independent browser core
// (browser_cdp_core.go). All CDP calls run through the hostLoop thread — the
// panel webview's COM object lives there, and its completion callbacks arrive
// on that thread's message pump.

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// cdpResult is the raw JSON object returned by a CDP method.
type cdpResult struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// call implements cdpSession for the WebView2 transport.
func (h *BrowserHost) call(ctx context.Context, method, params string) (json.RawMessage, error) {
	type outcome struct {
		raw json.RawMessage
		err error
	}
	ch := make(chan outcome, 1)

	h.submit(func() {
		h.mu.Lock()
		w := h.chromium
		h.mu.Unlock()
		if w == nil {
			ch <- outcome{err: errors.New("浏览器未打开")}
			return
		}
		if err := w.CallDevToolsProtocolMethod(method, params, func(resultJSON string) {
			if resultJSON == "" {
				ch <- outcome{err: errors.New("CDP 空结果")}
				return
			}
			var r cdpResult
			if err := json.Unmarshal([]byte(resultJSON), &r); err != nil {
				ch <- outcome{err: err}
				return
			}
			if r.Error != nil {
				ch <- outcome{err: errors.New(r.Error.Message)}
				return
			}
			ch <- outcome{raw: r.Data}
		}); err != nil {
			ch <- outcome{err: err}
		}
	})

	select {
	case o := <-ch:
		return o.raw, o.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(20 * time.Second):
		return nil, errors.New("CDP 调用超时")
	}
}

// Snapshot returns the compact ariaSnapshot-style tree of the current page.
// The injected chrome bar (data-reasonix-bar) is excluded.
func (h *BrowserHost) Snapshot(ctx context.Context) (string, error) {
	nodes, err := fetchAXTree(ctx, h)
	if err != nil {
		return "", err
	}
	barID := barBackendNodeID(ctx, h)
	tree, refs := snapshotTree(nodes, barID)
	h.mu.Lock()
	h.refIndex = newRefIndex()
	h.refIndex.set(refs)
	h.mu.Unlock()
	return tree, nil
}

// Click performs a real mouse click on a snapshot [ref=N] element.
func (h *BrowserHost) Click(ctx context.Context, ref int) error {
	return clickRef(ctx, h, h.refs(), ref)
}

// TypeText focuses the ref'd element and types text into it.
func (h *BrowserHost) TypeText(ctx context.Context, ref int, text string) error {
	return typeTextRef(ctx, h, h.refs(), ref, text)
}

// Press dispatches a key (Enter, Backspace, Escape, …).
func (h *BrowserHost) Press(ctx context.Context, key string) error {
	return pressKey(ctx, h, key)
}

// Scroll scrolls the viewport by (dx, dy) CSS pixels.
func (h *BrowserHost) Scroll(ctx context.Context, dx, dy int) error {
	return scrollViewport(ctx, h, dx, dy)
}

// Screenshot captures the visible viewport as PNG bytes.
func (h *BrowserHost) Screenshot(ctx context.Context) ([]byte, error) {
	return captureScreenshot(ctx, h)
}

// Evaluate runs a read-only JS expression (returns JSON value).
func (h *BrowserHost) Evaluate(ctx context.Context, js string) (string, error) {
	raw, err := evaluate(ctx, h, js)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// refs returns the current ref index (rebuilt by every Snapshot).
func (h *BrowserHost) refs() *refIndex {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.refIndex == nil {
		h.refIndex = newRefIndex()
	}
	return h.refIndex
}
