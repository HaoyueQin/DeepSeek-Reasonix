package builtin

import (
	"reasonix/internal/tool"
)

// Browser tools: the agent surface for driving the built-in browser panel
// (open a URL, read a DOM snapshot, click/type/scroll, screenshot). The
// platform backend is injected by the desktop shell via SetBrowserSession;
// the CLI has no backend, so these tools report "not enabled" there.
//
// The tool schemas are registered as regular built-ins but only reach the
// system prompt when a frontend passes them through boot.Options.ExtraTools —
// the CLI build never adds them, so the cache-stable prefix is untouched.

import (
	"encoding/base64"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// BrowserSession is the transport-agnostic browser backend a session drives.
// The desktop BrowserHost implements it; nil provider = unavailable.
type BrowserSession interface {
	// Open lazily creates the panel webview (no-op when already open).
	Open() error
	// Close destroys the panel webview.
	Close()
	// Navigate opens a URL (fire-and-forget; state arrives via events).
	Navigate(url string)
	// Snapshot returns the compact ariaSnapshot-style DOM tree.
	Snapshot(ctx context.Context) (string, error)
	// Click performs a real mouse click on a snapshot [ref=N] element.
	Click(ctx context.Context, ref int) error
	// TypeText focuses a [ref=N] element and types text into it.
	TypeText(ctx context.Context, ref int, text string) error
	// Press dispatches a key (Enter, Backspace, Escape, ArrowDown, …).
	Press(ctx context.Context, key string) error
	// Scroll scrolls the viewport by (dx, dy) CSS pixels.
	Scroll(ctx context.Context, dx, dy int) error
	// Screenshot captures the visible viewport as PNG bytes.
	Screenshot(ctx context.Context) ([]byte, error)
}

// BrowserSessionProvider returns the current session backend (nil when the
// host has no browser — e.g. the CLI).
var browserSessionProvider func() BrowserSession

// SetBrowserSession installs the platform browser backend provider. The
// desktop shell calls this at startup; the CLI never does.
func SetBrowserSession(p func() BrowserSession) {
	browserSessionProvider = p
}

func currentBrowserSession() BrowserSession {
	if browserSessionProvider == nil {
		return nil
	}
	return browserSessionProvider()
}

func browserErr(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("browser: %w", err)
}

// --- browser_open ----------------------------------------------------------

type browserOpenTool struct{}

func (browserOpenTool) Name() string        { return "browser_open" }
func (browserOpenTool) ReadOnly() bool      { return false }
func (browserOpenTool) Description() string { return "打开/显示内置浏览器面板（未打开时不加载任何浏览器资源）。面板打开后可用 browser_navigate 等工具驱动。" }
func (browserOpenTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}
func (browserOpenTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	s := currentBrowserSession()
	if s == nil {
		return "", browserErr(errors.New("内置浏览器未启用：请在设置中开启「内置浏览器控制」并新建会话"))
	}
	if err := s.Open(); err != nil {
		return "", browserErr(err)
	}
	return "浏览器面板已打开。", nil
}

// --- browser_close ---------------------------------------------------------

type browserCloseTool struct{}

func (browserCloseTool) Name() string        { return "browser_close" }
func (browserCloseTool) ReadOnly() bool      { return false }
func (browserCloseTool) Description() string { return "关闭内置浏览器面板并释放浏览器资源。" }
func (browserCloseTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}
func (browserCloseTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	if s := currentBrowserSession(); s != nil {
		s.Close()
		return "浏览器面板已关闭。", nil
	}
	return "浏览器未打开。", nil
}

// --- browser_navigate ------------------------------------------------------

type browserNavigateTool struct{}

func (browserNavigateTool) Name() string   { return "browser_navigate" }
func (browserNavigateTool) ReadOnly() bool { return false }
func (browserNavigateTool) Description() string {
	return "在内置浏览器中打开一个 URL（支持 http/https/localhost）。若面板未打开会自动打开。"
}
func (browserNavigateTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"要打开的完整网址，例如 https://localhost:5173/settings"}},"required":["url"]}`)
}
func (browserNavigateTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", browserErr(err)
	}
	url := strings.TrimSpace(in.URL)
	if url == "" {
		return "", browserErr(errors.New("url 不能为空"))
	}
	s := currentBrowserSession()
	if s == nil {
		return "", browserErr(errors.New("内置浏览器未启用：请在设置中开启「内置浏览器控制」并新建会话"))
	}
	if err := s.Open(); err != nil {
		return "", browserErr(err)
	}
	s.Navigate(url)
	return "已开始加载 " + url + "。可调用 browser_snapshot 观察页面状态。", nil
}

// --- browser_snapshot ------------------------------------------------------

type browserSnapshotTool struct{}

func (browserSnapshotTool) Name() string   { return "browser_snapshot" }
func (browserSnapshotTool) ReadOnly() bool { return true }
func (browserSnapshotTool) Description() string {
	return "获取当前页面的可访问性快照（ariaSnapshot 风格树，每行带 [ref=N] 定位符）。" +
		"基于快照中的事实构造后续 browser_click / browser_type / browser_press 的目标。快照排除浏览器自身的地址栏。"
}
func (browserSnapshotTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}
func (browserSnapshotTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	s := currentBrowserSession()
	if s == nil {
		return "", browserErr(errors.New("内置浏览器未启用：请在设置中开启「内置浏览器控制」并新建会话"))
	}
	tree, err := s.Snapshot(ctx)
	if err != nil {
		return "", browserErr(err)
	}
	if strings.TrimSpace(tree) == "" {
		return "页面无可访问内容（可能是空白页或仍在加载）。", nil
	}
	return tree, nil
}

// --- browser_click ---------------------------------------------------------

type browserClickTool struct{}

func (browserClickTool) Name() string   { return "browser_click" }
func (browserClickTool) ReadOnly() bool { return false }
func (browserClickTool) Description() string {
	return "在浏览器中点击快照里的元素。ref 必须来自最近一次 browser_snapshot 输出。"
}
func (browserClickTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"ref":{"type":"integer","description":"快照中的 [ref=N] 数字"}},"required":["ref"]}`)
}
func (browserClickTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Ref int `json:"ref"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", browserErr(err)
	}
	s := currentBrowserSession()
	if s == nil {
		return "", browserErr(errors.New("内置浏览器未启用"))
	}
	if err := s.Click(ctx, in.Ref); err != nil {
		return "", browserErr(err)
	}
	return fmt.Sprintf("已点击 ref=%d。用 browser_snapshot 确认页面变化。", in.Ref), nil
}

// --- browser_type ----------------------------------------------------------

type browserTypeTool struct{}

func (browserTypeTool) Name() string   { return "browser_type" }
func (browserTypeTool) ReadOnly() bool { return false }
func (browserTypeTool) Description() string {
	return "在快照 ref 指定的输入框中输入文本（先聚焦再键入）。"
}
func (browserTypeTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"ref":{"type":"integer","description":"快照中的 [ref=N] 数字"},"text":{"type":"string","description":"要输入的文本"}},"required":["ref","text"]}`)
}
func (browserTypeTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Ref  int    `json:"ref"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", browserErr(err)
	}
	s := currentBrowserSession()
	if s == nil {
		return "", browserErr(errors.New("内置浏览器未启用"))
	}
	if err := s.TypeText(ctx, in.Ref, in.Text); err != nil {
		return "", browserErr(err)
	}
	return fmt.Sprintf("已输入 %d 个字符。", len(in.Text)), nil
}

// --- browser_press ---------------------------------------------------------

type browserPressTool struct{}

func (browserPressTool) Name() string   { return "browser_press" }
func (browserPressTool) ReadOnly() bool { return false }
func (browserPressTool) Description() string {
	return "在浏览器中发送按键（Enter / Backspace / Tab / Escape / ArrowDown / ArrowUp / F5 等）。"
}
func (browserPressTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"key":{"type":"string","description":"按键名，如 Enter、Backspace、Escape、ArrowDown"}},"required":["key"]}`)
}
func (browserPressTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", browserErr(err)
	}
	s := currentBrowserSession()
	if s == nil {
		return "", browserErr(errors.New("内置浏览器未启用"))
	}
	if err := s.Press(ctx, in.Key); err != nil {
		return "", browserErr(err)
	}
	return "已发送按键 " + in.Key + "。", nil
}

// --- browser_scroll --------------------------------------------------------

type browserScrollTool struct{}

func (browserScrollTool) Name() string   { return "browser_scroll" }
func (browserScrollTool) ReadOnly() bool { return false }
func (browserScrollTool) Description() string {
	return "滚动浏览器视口。dy 为负向上、为正向下（CSS 像素）。"
}
func (browserScrollTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"dy":{"type":"integer","description":"垂直滚动量，如 400"},"dx":{"type":"integer","description":"水平滚动量，默认 0"}},"required":["dy"]}`)
}
func (browserScrollTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var in struct {
		DX int `json:"dx"`
		DY int `json:"dy"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", browserErr(err)
	}
	s := currentBrowserSession()
	if s == nil {
		return "", browserErr(errors.New("内置浏览器未启用"))
	}
	if err := s.Scroll(ctx, in.DX, in.DY); err != nil {
		return "", browserErr(err)
	}
	return "已滚动。", nil
}

// --- browser_screenshot ----------------------------------------------------

type browserScreenshotTool struct{}

func (browserScreenshotTool) Name() string   { return "browser_screenshot" }
func (browserScreenshotTool) ReadOnly() bool { return true }
func (browserScreenshotTool) Description() string {
	return "截取浏览器当前视口的截图（PNG）。需要支持视觉的模型才能看到图片内容；" +
		"普通 DOM 观察请优先使用 browser_snapshot。"
}
func (browserScreenshotTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{},"required":[]}`)
}
func (browserScreenshotTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	s := currentBrowserSession()
	if s == nil {
		return "", browserErr(errors.New("内置浏览器未启用"))
	}
	png, err := s.Screenshot(ctx)
	if err != nil {
		return "", browserErr(err)
	}
	_ = png // images travel via ExecuteWithImages
	return "截图已捕获（随图片通道返回）。", nil
}

// ExecuteWithImages implements tool.ImageTool: the screenshot rides alongside
// the text result so vision-capable providers see the page.
func (browserScreenshotTool) ExecuteWithImages(ctx context.Context, args json.RawMessage) (string, []string, error) {
	s := currentBrowserSession()
	if s == nil {
		return "", nil, browserErr(errors.New("内置浏览器未启用"))
	}
	png, err := s.Screenshot(ctx)
	if err != nil {
		return "", nil, browserErr(err)
	}
	dataURL := "data:image/png;base64," + base64StdEncoding(png)
	return "截图已捕获。", []string{dataURL}, nil
}

func init() {
	tool.RegisterBuiltin(browserOpenTool{})
	tool.RegisterBuiltin(browserCloseTool{})
	tool.RegisterBuiltin(browserNavigateTool{})
	tool.RegisterBuiltin(browserSnapshotTool{})
	tool.RegisterBuiltin(browserClickTool{})
	tool.RegisterBuiltin(browserTypeTool{})
	tool.RegisterBuiltin(browserPressTool{})
	tool.RegisterBuiltin(browserScrollTool{})
	tool.RegisterBuiltin(browserScreenshotTool{})
}

// base64StdEncoding wraps encoding/base64 for the screenshot data URL.
func base64StdEncoding(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
