package main

// Wails-bound surface for the built-in browser panel. These methods are the
// frontend's command channel (BrowserOpen/BrowserNavigate/… plus the Settings
// page's control toggle and data cleanup), mirroring the "浏览器" section of
// the ZCode settings docs.

import (
	"context"
	"errors"
	"time"

	"reasonix/internal/config"
)

// configHomeDir resolves the Reasonix home directory (honors REASONIX_HOME,
// falls back to the platform convention), used for the browser profile.
func configHomeDir() string {
	if dir := config.ReasonixHomeDir(); dir != "" {
		return dir
	}
	return config.MemoryUserDir()
}

// browserControlEnabled reports the user preference for the built-in browser
// control (Settings > Browser > 开启内置浏览器控制). New sessions only.
func (a *App) browserControlEnabled() bool {
	if a == nil {
		return false
	}
	cfg, err := config.Load()
	if err != nil {
		return false
	}
	return cfg.BrowserControlEnabled()
}

// SetBrowserControlEnabled persists the browser control preference.
func (a *App) SetBrowserControlEnabled(enabled bool) error {
	return a.applyConfigOnly(func(c *config.Config) error {
		return c.SetBrowserControlEnabled(enabled)
	})
}

// BrowserControlEnabled returns the current preference for the settings UI.
func (a *App) BrowserControlEnabled() bool {
	return a.browserControlEnabled()
}

// BrowserOpen lazily creates the panel webview and shrinks the main webview.
func (a *App) BrowserOpen(panelWidth int) error {
	if a.browserHost == nil {
		return errors.New("浏览器面板不可用")
	}
	return a.browserHost.Open(panelWidth)
}

// BrowserClose destroys the panel webview and restores the main webview.
func (a *App) BrowserClose() {
	if a.browserHost != nil {
		a.browserHost.Close()
	}
}

// BrowserNavigate opens a URL in the panel.
func (a *App) BrowserNavigate(url string) {
	if a.browserHost != nil {
		a.browserHost.Navigate(url)
	}
}

// BrowserBack navigates back in history.
func (a *App) BrowserBack() {
	if a.browserHost != nil {
		a.browserHost.Back()
	}
}

// BrowserForward navigates forward in history.
func (a *App) BrowserForward() {
	if a.browserHost != nil {
		a.browserHost.Forward()
	}
}

// BrowserReload reloads the current page.
func (a *App) BrowserReload() {
	if a.browserHost != nil {
		a.browserHost.Reload()
	}
}

// BrowserSetPanelWidth re-lays out after a frontend width drag.
func (a *App) BrowserSetPanelWidth(width int) {
	if a.browserHost != nil {
		a.browserHost.SetPanelWidth(width)
	}
}

// BrowserAddress returns the current URL/title/open state.
func (a *App) BrowserAddress() (string, string, bool) {
	if a.browserHost == nil {
		return "", "", false
	}
	return a.browserHost.Address()
}

// BrowserSnapshot returns the ariaSnapshot-style DOM tree (used by the agent
// tools and by the settings/diagnostics surface).
func (a *App) BrowserSnapshot() (string, error) {
	if a.browserHost == nil {
		return "", errors.New("浏览器面板不可用")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return a.browserHost.Snapshot(ctx)
}

// BrowserClearCache removes HTTP cache, Cache Storage, and Service Workers,
// keeping cookies and local site data.
func (a *App) BrowserClearCache() error {
	if a.browserHost == nil {
		return errors.New("浏览器面板不可用")
	}
	return a.browserHost.clearCache()
}

// BrowserClearAllData removes cookies, site data, and cache (irreversible).
func (a *App) BrowserClearAllData() error {
	if a.browserHost == nil {
		return errors.New("浏览器面板不可用")
	}
	return a.browserHost.clearAllData()
}
