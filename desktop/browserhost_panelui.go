//go:build windows

package main

// The injected chrome bar: a fixed top strip (address bar + back/forward/
// reload/close) rendered inside every page the panel webview loads. It talks
// to Go through window.chrome.webview.postMessage; the host replies by
// navigating or running JS. Marked with data-reasonix-bar so the DOM snapshot
// generator can exclude it from what the agent sees.
const panelChromeBarJS = `
(() => {
  if (window.__reasonixBarInstalled) return;
  window.__reasonixBarInstalled = true;
  const post = (obj) => {
    try { window.chrome.webview.postMessage(JSON.stringify(Object.assign({kind:'bar'}, obj))); } catch (e) {}
  };
  const bar = document.createElement('div');
  bar.setAttribute('data-reasonix-bar', '');
  bar.style.cssText = [
    'position:fixed', 'top:0', 'left:0', 'right:0', 'height:44px', 'z-index:2147483647',
    'display:flex', 'align-items:center', 'gap:6px', 'padding:0 8px',
    'background:#161b2b', 'color:#d1d5db', 'font:13px/1.4 system-ui,sans-serif',
    'box-sizing:border-box', 'border-bottom:1px solid #2a3350',
  ].join(';');
  const mk = (label, title, act) => {
    const b = document.createElement('button');
    b.textContent = label;
    b.title = title;
    b.style.cssText = 'border:1px solid #2a3350;background:#212a45;color:#d1d5db;border-radius:4px;padding:3px 9px;cursor:pointer;flex:0 0 auto;';
    b.addEventListener('click', () => post({act}));
    return b;
  };
  const input = document.createElement('input');
  input.setAttribute('data-reasonix-bar', 'address');
  input.type = 'text';
  input.placeholder = '输入网址并回车，例如 https://localhost:5173';
  input.style.cssText = 'flex:1 1 auto;min-width:60px;border:1px solid #2a3350;background:#0d1220;color:#e5e7eb;border-radius:4px;padding:4px 8px;font:inherit;';
  input.value = location.href;
  input.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      let url = input.value.trim();
      if (!url) return;
      if (!/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(url)) url = 'https://' + url;
      post({act:'navigate', url});
    }
  });
  bar.appendChild(mk('←', '后退', 'back'));
  bar.appendChild(mk('→', '前进', 'forward'));
  bar.appendChild(mk('⟳', '刷新', 'reload'));
  bar.appendChild(input);
  bar.appendChild(mk('×', '关闭浏览器', 'close'));
  document.documentElement.appendChild(bar);
  document.body.style.setProperty('scroll-padding-top', '44px', 'important');
  // Keep the address bar in sync with the current page.
  const sync = () => { if (location.href !== input.value) input.value = location.href; };
  window.addEventListener('load', sync);
  const iv = setInterval(() => { if (!document.body) return; sync(); }, 2000);
  window.addEventListener('unload', () => clearInterval(iv));
})();
`

// panelWelcomePage is the initial blank page shown before the first
// navigation; the chrome bar still appears on top of it.
const panelWelcomePage = `<html><head><meta charset="utf-8"><style>
html,body{height:100%;margin:0;background:#0d1220}
.wrap{display:flex;align-items:center;justify-content:center;height:100%;color:#4b5678;font:16px/1.6 system-ui,sans-serif;text-align:center;padding:0 24px}
</style></head><body><div class="wrap">在上方地址栏输入网址开始浏览<br>或让 Agent 打开页面（需在设置中开启「内置浏览器控制」）</div></body></html>`
