package main

// Platform-independent browser automation core. Both backends — the Windows
// WebView2 panel and the cross-platform system-Chrome CDP session — drive the
// same semantics:
//
//	Snapshot:  Accessibility.getFullAXTree -> compact ariaSnapshot-style YAML
//	           with [ref=N] tokens (Codex/ZCode convention); the injected
//	           chrome bar is excluded when it exists.
//	Actions:   resolve ref -> backend DOM node -> DOM.getBoxModel center ->
//	           Input.dispatchMouseEvent (real input), DOM.focus + insertText,
//	           Input.dispatchKeyEvent, mouse wheel scroll.
//
// A backend only provides call(ctx, method, params): the transport (WebView2
// CallDevToolsProtocolMethod or a Chrome DevTools WebSocket) is its business.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// cdpSession is a minimal Chrome DevTools Protocol transport.
type cdpSession interface {
	// call executes one CDP method and returns its result object (the JSON
	// payload WITHOUT the {"id":…} envelope).
	call(ctx context.Context, method, params string) (json.RawMessage, error)
}

// --- AX tree snapshot ------------------------------------------------------

// axID tolerates both numeric and string-encoded CDP node ids (Chromium
// serializes them as numbers, some Edge builds as strings).
type axID int64

func (i *axID) UnmarshalJSON(b []byte) error {
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	v, err := n.Int64()
	if err != nil {
		return err
	}
	*i = axID(v)
	return nil
}

// axNode mirrors the CDP Accessibility.getFullAXTree node shape.
type axNode struct {
	NodeID      axID  `json:"nodeId"`
	Ignored     bool  `json:"ignored"`
	Role        axVal `json:"role"`
	Name        axVal `json:"name"`
	Value       axVal `json:"value"`
	Properties  []struct {
		Name  string `json:"name"`
		Value axVal  `json:"value"`
	} `json:"properties"`
	BackendDOMNodeID axID   `json:"backendDOMNodeId"`
	ChildIDs         []axID `json:"childIds"`
}

type axVal struct {
	Value any    `json:"value"`
	Type  string `json:"type"`
}

// str renders the CDP value as text (values may be string, bool, or number).
func (a axVal) str() string {
	if a.Value == nil {
		return ""
	}
	if s, ok := a.Value.(string); ok {
		return s
	}
	return fmt.Sprint(a.Value)
}

// refIndex maps snapshot [ref=N] tokens to backend DOM node ids. It is built
// by Snapshot and consumed by the click/type actions; the caller owns it.
type refIndex struct {
	mu   sync.Mutex
	refs map[int]int64
}

func newRefIndex() *refIndex { return &refIndex{refs: map[int]int64{}} }

func (r *refIndex) set(refs map[int]int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refs = refs
}

func (r *refIndex) lookup(ref int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.refs[ref]
	if !ok || id == 0 {
		return 0, fmt.Errorf("ref=%d 不存在：快照已过期，请重新获取快照", ref)
	}
	return id, nil
}

// snapshotTree serializes an AX tree into the compact ariaSnapshot-style YAML.
// barBackendID (0 = none) excludes the injected chrome bar subtree.
func snapshotTree(nodes []axNode, barBackendID int64) (string, map[int]int64) {
	byID := make(map[axID]*axNode, len(nodes))
	roots := make([]axID, 0)
	for i := range nodes {
		n := &nodes[i]
		byID[n.NodeID] = n
		if n.Ignored {
			continue
		}
		child := false
		for j := range nodes {
			for _, c := range nodes[j].ChildIDs {
				if c == n.NodeID {
					child = true
					break
				}
			}
			if child {
				break
			}
		}
		if !child {
			roots = append(roots, n.NodeID)
		}
	}

	var b strings.Builder
	refs := make(map[int]int64)
	counter := 0
	var walk func(id axID, depth int)
	walk = func(id axID, depth int) {
		n := byID[id]
		if n == nil {
			return
		}
		if n.Ignored {
			// Ignored containers (e.g. Chromium's hidden wrapper nodes) emit no
			// line but their subtree is real content — keep descending.
			for _, c := range n.ChildIDs {
				walk(c, depth)
			}
			return
		}
		if barBackendID != 0 && int64(n.BackendDOMNodeID) == barBackendID {
			return
		}
		if !axEmitWorthy(n) {
			for _, c := range n.ChildIDs {
				walk(c, depth)
			}
			return
		}
		ref := counter
		counter++
		refs[ref] = int64(n.BackendDOMNodeID)
		indent := strings.Repeat("  ", depth)
		label := n.Role.str()
		if n.Name.str() != "" {
			label += " " + axQuote(n.Name.str())
		}
		b.WriteString(indent + "- " + label + fmt.Sprintf(" [ref=%d]", ref))
		for _, e := range axExtraAttrs(n) {
			b.WriteString(" " + e)
		}
		b.WriteString("\n")
		for _, c := range n.ChildIDs {
			walk(c, depth+1)
		}
	}
	for _, r := range roots {
		walk(r, 0)
	}
	return b.String(), refs
}

// axEmitWorthy decides whether a node produces a snapshot line.
func axEmitWorthy(n *axNode) bool {
	switch n.Role.Value {
	case "button", "link", "textbox", "searchbox", "checkbox", "combobox",
		"listbox", "option", "menuitem", "menuitemcheckbox", "menuitemradio",
		"radio", "tab", "switch", "heading", "text", "img", "form", "dialog",
		"navigation", "banner", "main", "complementary", "table", "row",
		"columnheader", "rowheader", "cell", "grid", "list", "listitem",
		"article", "alert", "status", "timer", "progressbar", "slider",
		"spinbutton", "tree", "treeitem", "math", "meter", "separator",
		"contentinfo", "search", "region", "document", "webview", "iframe":
		return true
	case "generic", "group", "none", "unknown", "presentation", "paragraph",
		"section", "staticText":
		return n.Name.str() != "" || n.Value.str() != ""
	}
	return n.Name.str() != "" || n.Value.str() != ""
}

// axExtraAttrs extracts compact [attr=value] tokens for stateful roles.
func axExtraAttrs(n *axNode) []string {
	var out []string
	seen := map[string]string{}
	for _, p := range n.Properties {
		if p.Value.str() == "" {
			continue
		}
		switch p.Name {
		case "checked", "disabled", "expanded", "selected", "pressed", "level", "valuetext", "multiselectable", "focused", "editable":
			seen[p.Name] = p.Value.str()
		}
	}
	if n.Value.str() != "" {
		seen["value"] = n.Value.str()
	}
	for _, k := range []string{"checked", "disabled", "expanded", "selected", "pressed", "level", "value", "focused"} {
		if v, ok := seen[k]; ok {
			out = append(out, fmt.Sprintf("[%s=%s]", k, axQuote(v)))
		}
	}
	return out
}

func axQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return `"` + s + `"`
}

// fetchAXTree pulls the full accessibility tree through the session.
func fetchAXTree(ctx context.Context, s cdpSession) ([]axNode, error) {
	raw, err := s.call(ctx, "Accessibility.getFullAXTree", `{}`)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Nodes []axNode `json:"nodes"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("AX 树解析: %w", err)
	}
	return resp.Nodes, nil
}

// --- actions ---------------------------------------------------------------

// clickNode clicks the element at backendID with real mouse input.
func clickNode(ctx context.Context, s cdpSession, backendID int64) error {
	params, _ := json.Marshal(map[string]any{"backendNodeId": backendID})
	raw, err := s.call(ctx, "DOM.getBoxModel", string(params))
	if err != nil {
		return fmt.Errorf("定位元素失败: %w", err)
	}
	var box struct {
		Model struct {
			Content []float64 `json:"content"`
		} `json:"model"`
	}
	if err := json.Unmarshal(raw, &box); err != nil {
		return fmt.Errorf("box 解析: %w", err)
	}
	if len(box.Model.Content) < 4 {
		return errors.New("元素无可见区域")
	}
	c := box.Model.Content
	cx := (c[0] + c[2]) / 2
	cy := (c[1] + c[5]) / 2
	for _, evt := range []string{"mousePressed", "mouseReleased"} {
		p, _ := json.Marshal(map[string]any{
			"type": evt, "x": cx, "y": cy, "button": "left", "clickCount": 1,
		})
		if _, err := s.call(ctx, "Input.dispatchMouseEvent", string(p)); err != nil {
			return err
		}
	}
	return nil
}

// clickRef clicks the element addressed by a snapshot ref.
func clickRef(ctx context.Context, s cdpSession, refs *refIndex, ref int) error {
	id, err := refs.lookup(ref)
	if err != nil {
		return err
	}
	return clickNode(ctx, s, id)
}

// typeTextRef focuses the ref'd element and inserts text.
func typeTextRef(ctx context.Context, s cdpSession, refs *refIndex, ref int, text string) error {
	id, err := refs.lookup(ref)
	if err != nil {
		return err
	}
	params, _ := json.Marshal(map[string]any{"backendNodeId": id})
	if _, err := s.call(ctx, "DOM.focus", string(params)); err != nil {
		return fmt.Errorf("聚焦失败: %w", err)
	}
	p, _ := json.Marshal(map[string]any{"text": text})
	if _, err := s.call(ctx, "Input.insertText", string(p)); err != nil {
		return fmt.Errorf("输入失败: %w", err)
	}
	return nil
}

// pressKey dispatches a key combination.
func pressKey(ctx context.Context, s cdpSession, key string) error {
	keyCode := keyCodeFor(key)
	for _, evt := range []string{"keyDown", "keyUp"} {
		p, _ := json.Marshal(map[string]any{
			"type": evt, "key": key, "code": keyCode, "windowsVirtualKeyCode": keyCode,
		})
		if _, err := s.call(ctx, "Input.dispatchKeyEvent", string(p)); err != nil {
			return err
		}
	}
	return nil
}

// scrollViewport scrolls with a mouse wheel event.
func scrollViewport(ctx context.Context, s cdpSession, dx, dy int) error {
	p, _ := json.Marshal(map[string]any{
		"type": "mouseWheel", "x": 10, "y": 10, "deltaX": dx, "deltaY": dy,
	})
	_, err := s.call(ctx, "Input.dispatchMouseEvent", string(p))
	return err
}

// captureScreenshot captures the visible viewport as PNG bytes.
func captureScreenshot(ctx context.Context, s cdpSession) ([]byte, error) {
	raw, err := s.call(ctx, "Page.captureScreenshot", `{"format":"png"}`)
	if err != nil {
		return nil, err
	}
	var r struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("截图解析: %w", err)
	}
	return base64.StdEncoding.DecodeString(r.Data)
}

// evaluate runs a read-only JS expression and returns its JSON value.
func evaluate(ctx context.Context, s cdpSession, js string) (json.RawMessage, error) {
	params, _ := json.Marshal(map[string]any{
		"expression":    js,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	raw, err := s.call(ctx, "Runtime.evaluate", string(params))
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Exception *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("evaluate 解析: %w", err)
	}
	if resp.Exception != nil {
		return nil, fmt.Errorf("页面脚本异常: %s", resp.Exception.Text)
	}
	return resp.Result.Value, nil
}

// keyCodeFor maps a logical key name to its Windows virtual key code.
func keyCodeFor(key string) int {
	switch strings.ToLower(key) {
	case "enter", "\r":
		return 13
	case "backspace":
		return 8
	case "tab":
		return 9
	case "escape", "esc":
		return 27
	case "space", " ":
		return 32
	case "arrowup":
		return 38
	case "arrowdown":
		return 40
	case "arrowleft":
		return 37
	case "arrowright":
		return 39
	case "delete":
		return 46
	case "home":
		return 36
	case "end":
		return 35
	case "pageup":
		return 33
	case "pagedown":
		return 34
	case "f5":
		return 116
	}
	if len(key) == 1 {
		return int(key[0])
	}
	return 0
}

// barBackendNodeID finds the injected chrome bar's backend DOM node id via
// DOM.querySelector, or 0 when the bar is not on the page. The bar is the
// reasonix panel's own chrome — excluded from snapshots so the agent never
// targets it.
func barBackendNodeID(ctx context.Context, s cdpSession) int64 {
	raw, err := evaluate(ctx, s, `(() => !!document.querySelector('[data-reasonix-bar]'))()`)
	if err != nil || string(raw) != "true" {
		return 0
	}
	docRaw, err := s.call(ctx, "DOM.getDocument", `{}`)
	if err != nil {
		return 0
	}
	var doc struct {
		Root struct {
			NodeID int64 `json:"nodeId"`
		} `json:"root"`
	}
	if json.Unmarshal(docRaw, &doc) != nil {
		return 0
	}
	q, _ := json.Marshal(map[string]any{
		"nodeId": doc.Root.NodeID, "selector": "[data-reasonix-bar]",
	})
	qRaw, err := s.call(ctx, "DOM.querySelector", string(q))
	if err != nil {
		return 0
	}
	var qr struct {
		NodeID int64 `json:"nodeId"`
	}
	if json.Unmarshal(qRaw, &qr) != nil || qr.NodeID == 0 {
		return 0
	}
	d, _ := json.Marshal(map[string]any{"nodeId": qr.NodeID})
	dRaw, err := s.call(ctx, "DOM.describeNode", string(d))
	if err != nil {
		return 0
	}
	var dr struct {
		Node struct {
			BackendNodeID int64 `json:"backendNodeId"`
		} `json:"node"`
	}
	if json.Unmarshal(dRaw, &dr) != nil {
		return 0
	}
	return dr.Node.BackendNodeID
}

// BrowserHostState mirrors the frontend-visible panel state.
type BrowserHostState struct {
	Open    bool   `json:"open"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Loading bool   `json:"loading"`
	Error   string `json:"error,omitempty"`
}
