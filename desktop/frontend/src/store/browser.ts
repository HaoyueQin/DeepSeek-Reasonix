import { create } from "zustand";
import * as App from "../../wailsjs/go/main/App";

/**
 * Browser panel store: mirrors the Go BrowserHostState pushed via the
 * "browser:state" runtime event, plus the panel width (physical px — the Go
 * side lays out WebView2 bounds in raw pixels).
 */

export interface BrowserState {
  open: boolean;
  url: string;
  title: string;
  loading: boolean;
  error?: string;
}

interface BrowserStore extends BrowserState {
  /** Physical-pixel panel width; the Go side uses it for WebView2 bounds. */
  width: number;
  /** Set from the runtime event emitted by the Go host. */
  applyState: (s: BrowserState) => void;
  /** User toggled the panel "+" button. */
  toggle: () => Promise<void>;
  /** User dragged the panel resizer; width is in physical pixels. */
  setWidth: (physicalWidth: number) => Promise<void>;
  navigate: (url: string) => Promise<void>;
  back: () => Promise<void>;
  forward: () => Promise<void>;
  reload: () => Promise<void>;
}

// Keep the width sticky across sessions (physical px; clamped by the host).
const WIDTH_KEY = "reasonix.browser.panelWidth";
const DEFAULT_WIDTH = 520;

function loadWidth(): number {
  const raw = Number(localStorage.getItem(WIDTH_KEY));
  if (!Number.isFinite(raw) || raw < 300 || raw > 2400) return DEFAULT_WIDTH;
  return raw;
}

export const useBrowserStore = create<BrowserStore>((set, get) => ({
  open: false,
  url: "",
  title: "",
  loading: false,
  width: loadWidth(),
  applyState: (s) => set({ ...s }),
  toggle: async () => {
    if (get().open) {
      await App.BrowserClose();
      set({ open: false, url: "", title: "", loading: false });
    } else {
      await App.BrowserOpen(get().width);
    }
  },
  setWidth: async (physicalWidth) => {
    localStorage.setItem(WIDTH_KEY, String(physicalWidth));
    set({ width: physicalWidth });
    await App.BrowserSetPanelWidth(physicalWidth);
  },
  navigate: async (url) => {
    await App.BrowserNavigate(url);
  },
  back: async () => {
    await App.BrowserBack();
  },
  forward: async () => {
    await App.BrowserForward();
  },
  reload: async () => {
    await App.BrowserReload();
  },
}));

/** Converts a CSS-pixel width to physical pixels for the Go bounds API. */
export function cssToPhysical(cssWidth: number): number {
  return Math.round(cssWidth * (window.devicePixelRatio || 1));
}

/** Maximum panel width: two thirds of the physical viewport (ZCode-style). */
export function maxBrowserPanelWidthCss(): number {
  const viewportCss = window.innerWidth;
  const twoThirds = Math.floor((viewportCss * 2) / 3);
  return Math.max(420, twoThirds);
}
