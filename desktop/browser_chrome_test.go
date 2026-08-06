package main

// Integration tests for the CDP browser core using a real system browser
// (Chrome/Chromium/Edge). Skipped when no browser binary is available — CI
// machines without a browser still build fine.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestPage(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Test Landing</title></head><body>
<h1>Test Landing</h1>
<a href="/form">Open the form</a>
</body></html>`))
	})
	mux.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><head><title>Form Page</title></head><body>
<h1>Form Page</h1>
<input id="q" aria-label="Search" placeholder="search">
<button id="go">Submit</button>
</body></html>`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func startTestBrowser(t *testing.T) *chromeCDP {
	t.Helper()
	if _, err := ChromeBinary(); err != nil {
		t.Skipf("no system browser available: %v", err)
	}
	profile := filepath.Join(t.TempDir(), "profile")
	c := newChromeCDP(profile)
	if err := c.Start(); err != nil {
		t.Fatalf("start browser: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

func waitFor(t *testing.T, timeout time.Duration, what string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

func TestChromeCDPNavigateSnapshot(t *testing.T) {
	srv := newTestPage(t)
	c := startTestBrowser(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := c.Navigate(ctx, srv.URL+"/"); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	time.Sleep(2 * time.Second)
	waitFor(t, 10*time.Second, "page title", func() bool {
		url, _, err := c.PageState(ctx)
		return err == nil && strings.HasSuffix(url, "/")
	})

	tree, ri, err := c.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if !strings.Contains(tree, "Test Landing") {
		t.Fatalf("snapshot missing heading text:\n%s", tree)
	}
	if !strings.Contains(tree, "[ref=") {
		t.Fatalf("snapshot missing ref tokens:\n%s", tree)
	}
	if ri == nil {
		t.Fatal("ref index is nil")
	}
	t.Logf("snapshot:\n%s", tree)
}

func TestChromeCDPClickLinkAndType(t *testing.T) {
	srv := newTestPage(t)
	c := startTestBrowser(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := c.Navigate(ctx, srv.URL+"/"); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	waitFor(t, 10*time.Second, "landing page", func() bool {
		_, title, err := c.PageState(ctx)
		return err == nil && strings.Contains(title, "Test Landing")
	})

	tree, ri, err := c.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	// Find the link's ref from the snapshot.
	linkRef := -1
	for _, line := range strings.Split(tree, "\n") {
		if strings.Contains(line, "Open the form") {
			if _, err := fmtSscanfRef(line, &linkRef); err != nil {
				t.Fatalf("parse ref from %q: %v", line, err)
			}
			break
		}
	}
	if linkRef < 0 {
		t.Fatalf("link ref not found in:\n%s", tree)
	}

	if err := c.Click(ctx, ri, linkRef); err != nil {
		t.Fatalf("click: %v", err)
	}
	waitFor(t, 10*time.Second, "form page navigation", func() bool {
		url, _, err := c.PageState(ctx)
		return err == nil && strings.HasSuffix(url, "/form")
	})

	tree2, ri2, err := c.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot after click: %v", err)
	}
	if !strings.Contains(tree2, "Form Page") {
		t.Fatalf("form page not visible:\n%s", tree2)
	}
	// Type into the search input.
	inputRef := -1
	for _, line := range strings.Split(tree2, "\n") {
		if strings.Contains(line, "Search") || strings.Contains(line, "search") {
			if _, err := fmtSscanfRef(line, &inputRef); err != nil {
				t.Fatalf("parse input ref: %v", err)
			}
			break
		}
	}
	if inputRef < 0 {
		t.Fatalf("input ref not found in:\n%s", tree2)
	}
	if err := c.TypeText(ctx, ri2, inputRef, "hello world"); err != nil {
		t.Fatalf("type: %v", err)
	}
	// Verify the text landed in the input.
	raw, err := evaluate(ctx, c, `document.getElementById('q').value`)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	var val string
	if err := json.Unmarshal(raw, &val); err != nil {
		t.Fatalf("value parse: %v", err)
	}
	if val != "hello world" {
		t.Fatalf("typed value = %q, want %q", val, "hello world")
	}
}

func TestChromeCDPScreenshot(t *testing.T) {
	srv := newTestPage(t)
	c := startTestBrowser(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := c.Navigate(ctx, srv.URL+"/"); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	waitFor(t, 10*time.Second, "page load", func() bool {
		_, title, err := c.PageState(ctx)
		return err == nil && strings.Contains(title, "Test Landing")
	})

	png, err := c.Screenshot(ctx)
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	if len(png) < 1000 {
		t.Fatalf("screenshot too small: %d bytes", len(png))
	}
	if string(png[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("screenshot is not PNG (magic mismatch)")
	}
	t.Logf("screenshot %d bytes", len(png))
}

func TestChromeProfileCleanup(t *testing.T) {
	dir := t.TempDir()
	// Fake a Chrome profile tree.
	for _, d := range []string{"Cache", "Code Cache", "GPUCache", "Service Worker", "Cookies"} {
		sub := filepath.Join(dir, d)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sub, "f.dat"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := clearChromeProfileCache(dir); err != nil {
		t.Fatalf("clearChromeProfileCache: %v", err)
	}
	for _, d := range []string{"Cache", "Code Cache", "GPUCache", "Service Worker"} {
		if _, err := os.Stat(filepath.Join(dir, d)); !os.IsNotExist(err) {
			t.Fatalf("cache dir %s should be removed", d)
		}
	}
	// Cookies must survive the cache-only cleanup.
	if _, err := os.Stat(filepath.Join(dir, "Cookies")); err != nil {
		t.Fatalf("Cookies should be kept: %v", err)
	}
}

// fmtSscanfRef extracts the [ref=N] token from a snapshot line.
func fmtSscanfRef(line string, ref *int) (int, error) {
	start := strings.Index(line, "[ref=")
	if start < 0 {
		return 0, os.ErrNotExist
	}
	end := strings.Index(line[start:], "]")
	if end < 0 {
		return 0, os.ErrNotExist
	}
	_, err := fmt.Sscanf(line[start+5:start+end], "%d", ref)
	return 0, err
}
