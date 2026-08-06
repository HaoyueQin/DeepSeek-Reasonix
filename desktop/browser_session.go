package main

// BrowserSession adapter: exposes the desktop BrowserHost to the agent tool
// layer (internal/tool/builtin.BrowserSession) with the user preference gate —
// when 「内置浏览器控制」 is off, tools report why instead of acting.

import (
	"reasonix/internal/tool"
	"context"
	"errors"

	"reasonix/internal/tool/builtin"
)

const defaultPanelWidth = 520

// browserSessionAdapter adapts *BrowserHost to the tool contract.
type browserSessionAdapter struct {
	host *BrowserHost
}

func (s *browserSessionAdapter) checkEnabled() error {
	if s.host == nil || !s.host.app.browserControlEnabled() {
		return errors.New("内置浏览器控制未开启：请在 设置 → 浏览器 中开启「开启内置浏览器控制」，并在新会话中使用")
	}
	return nil
}

func (s *browserSessionAdapter) Open() error {
	if err := s.checkEnabled(); err != nil {
		return err
	}
	return s.host.Open(defaultPanelWidth)
}

func (s *browserSessionAdapter) Close() {
	if s.host != nil {
		s.host.Close()
	}
}

func (s *browserSessionAdapter) Navigate(url string) {
	if err := s.checkEnabled(); err != nil {
		return
	}
	s.host.Navigate(url)
}

func (s *browserSessionAdapter) Snapshot(ctx context.Context) (string, error) {
	if err := s.checkEnabled(); err != nil {
		return "", err
	}
	if !s.host.isOpen() {
		return "", errors.New("浏览器面板未打开，请先调用 browser_open 或 browser_navigate")
	}
	return s.host.Snapshot(ctx)
}

func (s *browserSessionAdapter) Click(ctx context.Context, ref int) error {
	if err := s.checkEnabled(); err != nil {
		return err
	}
	if !s.host.isOpen() {
		return errors.New("浏览器面板未打开，请先调用 browser_open 或 browser_navigate")
	}
	return s.host.Click(ctx, ref)
}

func (s *browserSessionAdapter) TypeText(ctx context.Context, ref int, text string) error {
	if err := s.checkEnabled(); err != nil {
		return err
	}
	if !s.host.isOpen() {
		return errors.New("浏览器面板未打开，请先调用 browser_open 或 browser_navigate")
	}
	return s.host.TypeText(ctx, ref, text)
}

func (s *browserSessionAdapter) Press(ctx context.Context, key string) error {
	if err := s.checkEnabled(); err != nil {
		return err
	}
	if !s.host.isOpen() {
		return errors.New("浏览器面板未打开，请先调用 browser_open 或 browser_navigate")
	}
	return s.host.Press(ctx, key)
}

func (s *browserSessionAdapter) Scroll(ctx context.Context, dx, dy int) error {
	if err := s.checkEnabled(); err != nil {
		return err
	}
	if !s.host.isOpen() {
		return errors.New("浏览器面板未打开，请先调用 browser_open 或 browser_navigate")
	}
	return s.host.Scroll(ctx, dx, dy)
}

func (s *browserSessionAdapter) Screenshot(ctx context.Context) ([]byte, error) {
	if err := s.checkEnabled(); err != nil {
		return nil, err
	}
	if !s.host.isOpen() {
		return nil, errors.New("浏览器面板未打开，请先调用 browser_open 或 browser_navigate")
	}
	return s.host.Screenshot(ctx)
}

// installBrowserSession wires the tool layer to the desktop browser host and
// must be called once at startup, after browserHost exists.
func (a *App) installBrowserSession() {
	if a.browserHost == nil {
		return
	}
	builtin.SetBrowserSession(func() builtin.BrowserSession {
		return &browserSessionAdapter{host: a.browserHost}
	})
}

// extraBrowserTools are the agent tools the desktop shell adds to every
// session registry (browser_open/navigate/snapshot/…). The CLI never sets
// them, so CLI prompts keep the stable built-in tool surface. The browser
// tools are registered as built-ins in the tool package; this filters the
// compiled registry to that family.
func (a *App) extraBrowserTools() []tool.Tool {
	var out []tool.Tool
	for _, t := range tool.Builtins() {
		if len(t.Name()) > len("browser_") && t.Name()[:len("browser_")] == "browser_" {
			out = append(out, t)
		}
	}
	return out
}
