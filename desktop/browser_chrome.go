package main

// chromeCDP drives a system Chrome/Chromium over the DevTools Protocol
// (headless=new, dedicated user-data dir = independent login state). It is
// the panel backend on macOS/Linux, and it is also directly testable on any
// platform (integration tests launch the local Chrome/Edge binary).

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// chromeCDP owns the browser process and its CDP WebSocket.
type chromeCDP struct {
	mu      sync.Mutex
	proc    *exec.Cmd
	ws      *websocket.Conn
	nextID  int
	pending map[int]chan json.RawMessage
	profile string
	closed  chan struct{}
}

// newChromeCDP builds a driver using profile as its user-data dir.
func newChromeCDP(profile string) *chromeCDP {
	return &chromeCDP{
		pending: map[int]chan json.RawMessage{},
		profile: profile,
		closed:  make(chan struct{}),
	}
}

// ChromeBinary resolves a system Chrome/Chromium/Edge executable.
func ChromeBinary() (string, error) {
	if p := os.Getenv("REASONIX_BROWSER_PATH"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	candidates := []string{}
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	case "linux":
		candidates = []string{
			"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge",
		}
	case "windows":
		candidates = []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		}
	}
	for _, c := range candidates {
		if strings.Contains(c, "/") || strings.Contains(c, `\`) {
			if _, err := os.Stat(c); err == nil {
				return c, nil
			}
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", errors.New("未找到 Chrome/Chromium/Edge：请安装浏览器，或用 REASONIX_BROWSER_PATH 指定路径")
}

// Start launches the browser and connects its DevTools endpoint.
func (c *chromeCDP) Start() error {
	bin, err := ChromeBinary()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.profile, 0o755); err != nil {
		return fmt.Errorf("browser profile dir: %w", err)
	}
	args := []string{
		"--headless=new",
		"--remote-debugging-port=0",
		"--user-data-dir=" + c.profile,
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		"--disable-gpu",
		"--hide-scrollbars",
		"--window-size=1280,900",
	}
	cmd := exec.Command(bin, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动浏览器: %w", err)
	}
	c.mu.Lock()
	c.proc = cmd
	c.mu.Unlock()

	portFile := filepath.Join(c.profile, "DevToolsActivePort")
	var port string
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(portFile); err == nil {
			sc := bufio.NewScanner(strings.NewReader(string(data)))
			if sc.Scan() {
				port = strings.TrimSpace(sc.Text())
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if port == "" {
		_ = cmd.Process.Kill()
		return errors.New("DevTools 端口未就绪")
	}

	wsURL, err := c.findPageTarget(port)
	if err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		_ = cmd.Process.Kill()
		return fmt.Errorf("连接 DevTools: %w", err)
	}
	c.mu.Lock()
	c.ws = conn
	c.mu.Unlock()
	go c.readLoop(conn)
	return nil
}

func (c *chromeCDP) findPageTarget(port string) (string, error) {
	httpc := &http.Client{Timeout: 3 * time.Second}
	resp, err := httpc.Get("http://127.0.0.1:" + port + "/json/list")
	if err != nil {
		return "", fmt.Errorf("DevTools 列表: %w", err)
	}
	defer resp.Body.Close()
	var targets []struct {
		Type              string `json:"type"`
		WebSocketDebugURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return "", fmt.Errorf("DevTools 列表解析: %w", err)
	}
	for _, t := range targets {
		if t.Type == "page" && t.WebSocketDebugURL != "" {
			return t.WebSocketDebugURL, nil
		}
	}
	return "", errors.New("DevTools 无 page target")
}

func (c *chromeCDP) readLoop(conn *websocket.Conn) {
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			c.failAll(errors.New("DevTools 连接断开"))
			return
		}
		var msg struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &msg) != nil || msg.ID == 0 {
			continue
		}
		c.mu.Lock()
		ch := c.pending[msg.ID]
		delete(c.pending, msg.ID)
		c.mu.Unlock()
		if ch == nil {
			continue
		}
		if msg.Error != nil {
			close(ch)
			continue
		}
		ch <- msg.Result
		close(ch)
	}
}

func (c *chromeCDP) failAll(err error) {
	c.mu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.ws = nil
	c.mu.Unlock()
}

// call implements cdpSession.
func (c *chromeCDP) call(ctx context.Context, method, params string) (json.RawMessage, error) {
	c.mu.Lock()
	if c.ws == nil {
		c.mu.Unlock()
		return nil, errors.New("浏览器未打开")
	}
	c.nextID++
	id := c.nextID
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch
	ws := c.ws
	c.mu.Unlock()

	payload, _ := json.Marshal(map[string]any{
		"id": id, "method": method, "params": json.RawMessage(params),
	})
	if err := ws.WriteMessage(websocket.TextMessage, payload); err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("CDP 发送: %w", err)
	}

	select {
	case result, ok := <-ch:
		if !ok {
			return nil, errors.New("DevTools 连接已断开")
		}
		return result, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(20 * time.Second):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, errors.New("CDP 调用超时")
	}
}

// Navigate opens a URL.
func (c *chromeCDP) Navigate(ctx context.Context, url string) error {
	p, _ := json.Marshal(map[string]any{"url": url})
	_, err := c.call(ctx, "Page.navigate", string(p))
	return err
}

// Back / Forward / Reload via history API and Page.reload.
func (c *chromeCDP) Back(ctx context.Context) error {
	_, err := c.call(ctx, "Runtime.evaluate", `{"expression":"history.back()"}`)
	return err
}

func (c *chromeCDP) Forward(ctx context.Context) error {
	_, err := c.call(ctx, "Runtime.evaluate", `{"expression":"history.forward()"}`)
	return err
}

func (c *chromeCDP) Reload(ctx context.Context) error {
	_, err := c.call(ctx, "Page.reload", `{}`)
	return err
}

// PageState returns the current URL and title.
func (c *chromeCDP) PageState(ctx context.Context) (url, title string, err error) {
	raw, err := evaluate(ctx, c, `({url: location.href, title: document.title})`)
	if err != nil {
		return "", "", err
	}
	var st struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return "", "", err
	}
	return st.URL, st.Title, nil
}

// Snapshot returns the ariaSnapshot-style tree (no chrome bar on remote pages).
func (c *chromeCDP) Snapshot(ctx context.Context) (string, *refIndex, error) {
	nodes, err := fetchAXTree(ctx, c)
	if err != nil {
		return "", nil, err
	}
	tree, refs := snapshotTree(nodes, 0)
	ri := newRefIndex()
	ri.set(refs)
	return tree, ri, nil
}

// Click / TypeText / Press / Scroll / Screenshot delegate to the core.
func (c *chromeCDP) Click(ctx context.Context, refs *refIndex, ref int) error {
	return clickRef(ctx, c, refs, ref)
}

func (c *chromeCDP) TypeText(ctx context.Context, refs *refIndex, ref int, text string) error {
	return typeTextRef(ctx, c, refs, ref, text)
}

func (c *chromeCDP) Press(ctx context.Context, key string) error {
	return pressKey(ctx, c, key)
}

func (c *chromeCDP) Scroll(ctx context.Context, dx, dy int) error {
	return scrollViewport(ctx, c, dx, dy)
}

func (c *chromeCDP) Screenshot(ctx context.Context) ([]byte, error) {
	return captureScreenshot(ctx, c)
}

// Close kills the browser process tree (only the process this driver started —
// never by image name, so user-owned browsers are untouched).
func (c *chromeCDP) Close() {
	c.mu.Lock()
	proc := c.proc
	c.proc = nil
	c.mu.Unlock()
	if proc == nil || proc.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		// Kill the tree rooted at our own child PID.
		_ = exec.Command("taskkill", "/PID", fmt.Sprint(proc.Process.Pid), "/T", "/F").Run()
	} else {
		_ = proc.Process.Kill()
	}
	select {
	case <-c.closed:
	default:
		close(c.closed)
	}
}

// base64DataURL renders bytes as a data URL for the frontend screenshot feed.
func base64DataURL(b []byte) string {
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
}

// clearChromeProfileCache removes HTTP cache and service workers while keeping
// cookies and local site data (Chrome profile layout).
func clearChromeProfileCache(profile string) error {
	var failed []string
	for _, dir := range []string{"Cache", "Code Cache", "GPUCache", "Service Worker", "ShaderCache", "DawnCache", "DawnGraphiteCache", "GraphiteDawnCache", "SharedDictionary"} {
		if err := os.RemoveAll(filepath.Join(profile, dir)); err != nil {
			failed = append(failed, dir)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("部分缓存目录清理失败: %v", failed)
	}
	return nil
}

// removeAll is the alias used by the !windows host for full profile cleanup.
func removeAll(p string) error {
	return os.RemoveAll(p)
}
