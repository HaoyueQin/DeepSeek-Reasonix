package builtin

// Unit tests for the browser tool family with a fake session backend:
// schema/registration shape, the not-enabled path, and the happy path for
// every tool. The real CDP behaviors are covered by the desktop integration
// tests (desktop/browser_chrome_test.go).

import (
	"context"
	"reasonix/internal/tool"
	"errors"
	"testing"
)

// fakeBrowserSession implements BrowserSession with scripted results.
type fakeBrowserSession struct {
	opened    bool
	snapshot  string
	snapErr   error
	clicked   []int
	typed     [][2]string
	pressed   []string
	scrolls   [][2]int
	screenshot []byte
	shotErr   error
	navigated []string
}

func (f *fakeBrowserSession) Open() error                          { f.opened = true; return nil }
func (f *fakeBrowserSession) Close()                               { f.opened = false }
func (f *fakeBrowserSession) Navigate(url string)                  { f.navigated = append(f.navigated, url) }
func (f *fakeBrowserSession) Snapshot(ctx context.Context) (string, error) {
	return f.snapshot, f.snapErr
}
func (f *fakeBrowserSession) Click(ctx context.Context, ref int) error {
	f.clicked = append(f.clicked, ref)
	return nil
}
func (f *fakeBrowserSession) TypeText(ctx context.Context, ref int, text string) error {
	f.typed = append(f.typed, [2]string{itoa(ref), text})
	return nil
}
func (f *fakeBrowserSession) Press(ctx context.Context, key string) error {
	f.pressed = append(f.pressed, key)
	return nil
}
func (f *fakeBrowserSession) Scroll(ctx context.Context, dx, dy int) error {
	f.scrolls = append(f.scrolls, [2]int{dx, dy})
	return nil
}
func (f *fakeBrowserSession) Screenshot(ctx context.Context) ([]byte, error) {
	return f.screenshot, f.shotErr
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func withFakeSession(f *fakeBrowserSession, t *testing.T) {
	prev := browserSessionProvider
	SetBrowserSession(func() BrowserSession { return f })
	t.Cleanup(func() { browserSessionProvider = prev })
}

func TestBrowserToolsRequireSession(t *testing.T) {
	// No provider installed: every tool reports not-enabled, never panics.
	prev := browserSessionProvider
	browserSessionProvider = nil
	t.Cleanup(func() { browserSessionProvider = prev })

	for _, name := range []string{
		"browser_open", "browser_navigate", "browser_snapshot",
		"browser_click", "browser_type", "browser_press", "browser_scroll",
		"browser_screenshot",
	} {
		bt, ok := tool.LookupBuiltin(name)
		if !ok {
			t.Fatalf("builtin %s not registered", name)
		}
		if _, err := bt.Execute(context.Background(), nil); err == nil {
			t.Errorf("%s: expected not-enabled error without a session", name)
		}
	}
}

func TestBrowserToolsRegisteredAsBuiltins(t *testing.T) {
	for _, name := range []string{
		"browser_open", "browser_close", "browser_navigate", "browser_snapshot",
		"browser_click", "browser_type", "browser_press", "browser_scroll",
		"browser_screenshot",
	} {
		if _, ok := tool.LookupBuiltin(name); !ok {
			t.Errorf("builtin %s missing", name)
		}
	}
}

func TestBrowserNavigateHappyPath(t *testing.T) {
	f := &fakeBrowserSession{}
	withFakeSession(f, t)

	bt, _ := tool.LookupBuiltin("browser_navigate")
	out, err := bt.Execute(context.Background(), []byte(`{"url":"https://example.com"}`))
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if !f.opened {
		t.Error("navigate should open the panel first")
	}
	if len(f.navigated) != 1 || f.navigated[0] != "https://example.com" {
		t.Errorf("navigated = %v", f.navigated)
	}
	if out == "" {
		t.Error("empty result")
	}
}

func TestBrowserNavigateRequiresURL(t *testing.T) {
	f := &fakeBrowserSession{}
	withFakeSession(f, t)
	bt, _ := tool.LookupBuiltin("browser_navigate")
	if _, err := bt.Execute(context.Background(), []byte(`{}`)); err == nil {
		t.Error("empty url must fail")
	}
}

func TestBrowserSnapshotHappyPath(t *testing.T) {
	f := &fakeBrowserSession{snapshot: "- heading \"Hi\" [ref=0]\n"}
	withFakeSession(f, t)

	bt, _ := tool.LookupBuiltin("browser_snapshot")
	out, err := bt.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if out != f.snapshot {
		t.Errorf("snapshot = %q", out)
	}
}

func TestBrowserClickTypePressScroll(t *testing.T) {
	f := &fakeBrowserSession{}
	withFakeSession(f, t)

	click, _ := tool.LookupBuiltin("browser_click")
	if _, err := click.Execute(context.Background(), []byte(`{"ref":3}`)); err != nil {
		t.Fatalf("click: %v", err)
	}
	if len(f.clicked) != 1 || f.clicked[0] != 3 {
		t.Errorf("clicked = %v", f.clicked)
	}

	typ, _ := tool.LookupBuiltin("browser_type")
	if _, err := typ.Execute(context.Background(), []byte(`{"ref":1,"text":"hi"}`)); err != nil {
		t.Fatalf("type: %v", err)
	}
	if len(f.typed) != 1 || f.typed[0] != [2]string{"1", "hi"} {
		t.Errorf("typed = %v", f.typed)
	}

	press, _ := tool.LookupBuiltin("browser_press")
	if _, err := press.Execute(context.Background(), []byte(`{"key":"Enter"}`)); err != nil {
		t.Fatalf("press: %v", err)
	}
	if len(f.pressed) != 1 || f.pressed[0] != "Enter" {
		t.Errorf("pressed = %v", f.pressed)
	}

	scroll, _ := tool.LookupBuiltin("browser_scroll")
	if _, err := scroll.Execute(context.Background(), []byte(`{"dy":400}`)); err != nil {
		t.Fatalf("scroll: %v", err)
	}
	if len(f.scrolls) != 1 || f.scrolls[0] != [2]int{0, 400} {
		t.Errorf("scrolls = %v", f.scrolls)
	}
}

func TestBrowserScreenshotImageTool(t *testing.T) {
	f := &fakeBrowserSession{screenshot: []byte("PNGDATA")}
	withFakeSession(f, t)

	bt, _ := tool.LookupBuiltin("browser_screenshot")
	imgTool, ok := bt.(tool.ImageTool)
	if !ok {
		t.Fatal("browser_screenshot must implement ImageTool")
	}
	text, images, err := imgTool.ExecuteWithImages(context.Background(), nil)
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	if len(images) != 1 || images[0] != "data:image/png;base64,UE5HREFUQQ==" {
		t.Errorf("images = %v (text=%q)", images, text)
	}
}

func TestBrowserCloseTool(t *testing.T) {
	f := &fakeBrowserSession{opened: true}
	withFakeSession(f, t)
	bt, _ := tool.LookupBuiltin("browser_close")
	if _, err := bt.Execute(context.Background(), nil); err != nil {
		t.Fatalf("close: %v", err)
	}
	if f.opened {
		t.Error("panel should be closed")
	}
}

func TestBrowserOpenToolIdempotent(t *testing.T) {
	f := &fakeBrowserSession{}
	withFakeSession(f, t)
	bt, _ := tool.LookupBuiltin("browser_open")
	if _, err := bt.Execute(context.Background(), nil); err != nil {
		t.Fatalf("open: %v", err)
	}
	if !f.opened {
		t.Error("panel should be open")
	}
}

func TestBrowserSnapshotPropagatesBackendError(t *testing.T) {
	f := &fakeBrowserSession{snapErr: errors.New("boom")}
	withFakeSession(f, t)
	bt, _ := tool.LookupBuiltin("browser_snapshot")
	_, err := bt.Execute(context.Background(), nil)
	if err == nil || err.Error() != "browser: boom" {
		t.Errorf("err = %v", err)
	}
}
