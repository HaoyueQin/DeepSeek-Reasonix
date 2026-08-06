import { useEffect, useRef, useState } from "react";
import { useBrowserStore, cssToPhysical, maxBrowserPanelWidthCss } from "../store/browser";
import "./BrowserPanel.css";

/**
 * BrowserPanel is the frontend companion of the built-in browser.
 *
 * Native mode (Windows): the page lives in a second WebView2 that the Go host
 * lays out to the right of this (shrunk) webview. This component only renders
 * the width-drag resizer pinned to the right edge.
 *
 * Remote mode (macOS/Linux): a system Chrome is driven over CDP and its page
 * is echoed here as a screenshot feed, with the address bar rendered in React
 * (the Go host emits "browser:state" and "browser:screenshot" events).
 */
export function BrowserPanel() {
  const { open, url, applyState, setWidth, navigate, back, forward, reload, toggle } = useBrowserStore();
  const [dragging, setDragging] = useState(false);
  const dragRef = useRef<{ startX: number; startWidth: number } | null>(null);
  const [native, setNative] = useState(true);
  const [shot, setShot] = useState<string | null>(null);
  const [address, setAddress] = useState("");

  useEffect(() => {
    // Detect the panel backend: Windows uses the native WebView2 panel; other
    // platforms echo the CDP-driven Chrome as screenshots.
    let alive = true;
    void window.go?.main?.App?.Platform?.()
      .then((p: string) => {
        if (alive) setNative(p === "windows");
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  useEffect(() => {
    if (!open) return;
    const offState = window.runtime?.EventsOn("browser:state", (state: unknown) => {
      if (state && typeof state === "object") {
        const s = state as { open?: boolean; url?: string; title?: string; loading?: boolean; error?: string };
        applyState({
          open: Boolean(s.open),
          url: s.url ?? "",
          title: s.title ?? "",
          loading: Boolean(s.loading),
          error: s.error,
        });
      }
    });
    const offShot = window.runtime?.EventsOn("browser:screenshot", (dataURL: unknown) => {
      if (typeof dataURL === "string") setShot(dataURL);
    });
    return () => {
      offState?.();
      offShot?.();
    };
  }, [open, applyState]);

  // Keep the address input in sync with the live URL.
  useEffect(() => {
    setAddress(url);
  }, [url]);

  // Pointer drag on the right-edge resizer (native mode only).
  const onResizerPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    e.preventDefault();
    dragRef.current = { startX: e.clientX, startWidth: useBrowserStore.getState().width };
    setDragging(true);
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
  };
  const onResizerPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!dragging || !dragRef.current) return;
    const deltaCss = e.clientX - dragRef.current.startX;
    const nextCss = dragRef.current.startWidth / (window.devicePixelRatio || 1) + deltaCss;
    const maxCss = maxBrowserPanelWidthCss();
    const clampedCss = Math.max(300, Math.min(maxCss, nextCss));
    void setWidth(cssToPhysical(clampedCss));
  };
  const endDrag = () => {
    dragRef.current = null;
    setDragging(false);
  };

  const submitAddress = () => {
    let value = address.trim();
    if (!value) return;
    if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(value)) value = "https://" + value;
    void navigate(value);
  };

  if (!open) return null;

  return (
    <>
      {native && (
        <div
          className={`browser-panel-resizer${dragging ? " browser-panel-resizer--active" : ""}`}
          role="separator"
          aria-orientation="vertical"
          aria-label="调整浏览器面板宽度"
          onPointerDown={onResizerPointerDown}
          onPointerMove={onResizerPointerMove}
          onPointerUp={endDrag}
          onPointerCancel={endDrag}
        />
      )}
      {!native && (
        <aside className="browser-panel-remote" aria-label="内置浏览器">
          <div className="browser-panel-remote__bar">
            <button type="button" title="后退" onClick={() => void back()}>
              ←
            </button>
            <button type="button" title="前进" onClick={() => void forward()}>
              →
            </button>
            <button type="button" title="刷新" onClick={() => void reload()}>
              ⟳
            </button>
            <input
              className="browser-panel-remote__address"
              type="text"
              placeholder="输入网址并回车"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") submitAddress();
              }}
            />
            <button type="button" title="关闭浏览器" onClick={() => void toggle()}>
              ×
            </button>
          </div>
          <div className="browser-panel-remote__body">
            {shot ? (
              <img src={shot} alt="浏览器页面" className="browser-panel-remote__shot" />
            ) : (
              <div className="browser-panel-remote__placeholder">正在启动浏览器…</div>
            )}
          </div>
        </aside>
      )}
    </>
  );
}
