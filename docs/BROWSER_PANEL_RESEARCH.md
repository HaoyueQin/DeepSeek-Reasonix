# Reasonix 浏览器面板（Browser Panel）可行性研究报告

> 调研日期：2026-08-06
> 目标：评估给 Reasonix 加入 ZCode 式「侧边栏内置浏览器面板 + Agent 一句话驱动浏览网页」功能
> 调研对象：ZCode browser-use（本机插件源码 + 本机应用二进制实证）、OpenAI Codex、其他开源编码智能体
> 分支：feat/browser-panel（基于 fork main-v2 @ 44f749eae）
> 证据级别标注：✅ 第一手实证（本机源码/二进制）｜📖 公开文档/仓库 ｜🧩 推断（未验证）

## 1. 背景与目标

为 Reasonix（Go 语言编码智能体，桌面端 Wails + WebView2，CLI/TUI/SSE 三前端共享
`internal/control.Controller` 内核）评估加入 ZCode 式浏览器能力：

- **用户视角**：应用内打开侧边栏浏览器面板，可直接浏览本地开发地址或线上页面；
- **Agent 驱动**：Agent 用自然语言指令驱动浏览器——打开网址、点击、填表、滚动、截图，再根据页面真实状态决定下一步；
- **元素点选**：把页面元素直接选成上下文（`@element` 引用）告诉 Agent；
- **验证闭环**：前端改动后 Agent 自己打开页面验证效果，而不是只报告「应该好了」。

本文档结构：ZCode 实现解析（第 2 章）、外部同类项目调研（第 3 章）、模式归纳（第 4 章）、
Reasonix 可行性评估（第 5 章）、结论（第 6 章）。

## 2. ZCode browser-use 功能解析

### 2.1 用户可见功能面

ZCode 文档（https://zcode.z.ai/cn/docs/browser-use）描述的功能：

| 功能 | 说明 |
|------|------|
| 浏览器面板 | 桌面端侧边栏内置浏览器，地址栏/前进/后退/刷新/开发者工具，支持 http/https/file |
| Agent 驱动 | 一句话打开网址、点击、填表、滚动、截图并验证结果 |
| 标签管理 | Agent 只操作自己打开的标签；标签跨轮次保留；可显式认领用户标签 |
| 视口控制 | `setViewportSize`（320–3840 × 320–2160 CSS px）验证窄屏/移动端，自动进入自由尺寸模式 |
| 登录态 | 独立登录态；可一次性导入 Chrome Cookie/LocalStorage（macOS；Windows 暂不支持） |
| 数据清理 | 清除登录态 / HTTP 缓存 / 全部浏览器数据 |
| 元素点选 | 工具栏点选按钮 → 采集元素文本、选择器、位置、安全 HTML 摘要 → 加入当前聊天 |
| 安全边界 | 页面文字不当作指令；IAB 不能上传文件（filechooser 报 capability_unsupported）；操作受执行模式约束 |

### 2.2 插件架构（本机源码实证，✅）

ZCode 的 browser-use 是**官方内置插件** `@zcode/browser-use-plugin` v0.1.2（本机缓存于
`C:\Users\DF4B-9326.LAPTOP-KHNMRDVI\.zcode\cli\plugins\cache\zcode-plugins-official\browser-use\0.1.2\`），
由四类资产组成：

```
browser-use-plugin/0.1.2/
├── .zcode-plugin/plugin.json      # 插件清单（author: Z.ai, license MIT）
├── dist/mcp/server.js             # node_repl MCP server（esbuild 打包，133K 行）
├── scripts/browser-client.mjs     # 浏览器客户端引导模块（2099 行，纯 JS）
├── docs/                          # 能力文档（api.json / documents.json / *.md）
└── skills/
    ├── control-browser/SKILL.md   # 教 Agent 如何引导和驱动浏览器
    └── web-gui-tester/SKILL.md    # 纯 GUI 黑盒测试方法论（截图+只读 DOM 交叉验证）
```

**三层架构**（✅ 源码实证）：

```
┌──────────────────────────────────────────────────────────────┐
│ 指令层：control-browser / web-gui-tester skill（模型指导）     │
├──────────────────────────────────────────────────────────────┤
│ 控制层：node_repl MCP server 的 js 工具（持久 Node.js VM）     │
│         agent.browsers / Tab / playwright 子集对象模型         │
│         manifest 能力门控（不支持的 API 成员对模型隐藏）         │
├──────────────────────────────────────────────────────────────┤
│ 运行时层：宿主注入 bridge（Symbol.for("zcode.node-repl.       │
│           browser-control-bridge")）→ iab / cdp / extension   │
│           三种浏览器后端                                     │
└──────────────────────────────────────────────────────────────┘
```

**关键机制**（✅ 均从插件源码逐行确认）：

1. **后端类型**（`docs/overview.md`）：`iab`（桌面应用内浏览器）、`cdp`（CLI 显式
   `--browser-use=headless` 的无头 Chromium）、`extension`（Chrome 扩展）。Playwright 是 Tab 的
   API 面，不是第四种后端。

2. **控制通道**：`node_repl` MCP server 暴露 `js` / `js_reset` / `js_add_node_module_dir`。
   Agent 通过 `mcp__node_repl__js` 在**持久 Node VM** 里执行 JS；宿主导入 bridge 对象
   （`Symbol.for("zcode.node-repl.browser-control-bridge")`），提供 `list()` 与
   `execute(browserId, generation, command)` 两个 RPC，命令格式如 `{ method: "playwright", action }`。

3. **对象模型**（`browser-client.mjs` + `docs/api.json`，自称 Codex 兼容契约）：
   - `agent.browsers`：`list / get / getDefault / getForUrl`；
   - `Browser`：`tabs`（list/get/new/selected/finalize）、`user`（openTabs/claimTab）、`capabilities`；
   - `Tab`：`goto / back / forward / reload / close / screenshot / setViewportSize / viewportSize / getJsDialog / markDeliverable / markHandoff`；
   - `tab.playwright`：受限 Playwright 子集 —— `domSnapshot()`、`getByRole/getByText/getByLabel/
     getByPlaceholder/getByTestId/locator/frameLocator`、locator 动作、`evaluate`（只读最后手段）、
     `waitForURL/waitForLoadState/waitForTimeout/expectNavigation/waitForEvent`；
   - `tab.cua` / `tab.dom_cua`：坐标路径（canvas/自定义控件兜底）与 DOM 节点路径。

4. **DOM 观察**：`tab.playwright.domSnapshot()` 返回**紧凑 AI/ARIA 树**（计算角色、可访问名、状态、
   内联 iframe 内容），而非 outerHTML。快照是**定位器唯一事实来源**：只准用快照中出现的事实构造
   locator；禁止猜测 label/选择器/URL。

5. **工作流纪律**（`docs/workflow.md` / `docs/playwright.md`）：每批操作先 `tabs.list()` 返回完整
   列表 → 按稳定 id/url/title 匹配 → `domSnapshot()` → 唯一 locator → 动作 → 最小观察。每个观察
   周期**最多一个状态变更动作**；超时后**重建快照而非重试 locator**；可能弹出新标签时必须同时读
   受控标签与用户标签（`{controlledTabs, userTabs}` 同一 cell 返回）。

6. **能力门控（manifest）**：`api.json` 声明完整对象面，后端描述符带 `capabilities.browser/tab`
   与 `apiSupportOverrides`；Proxy 实现让**后端不支持的成员直接对模型隐藏**（`in` 为 false），
   而非调用后失败。

7. **截图管道**：`tab.screenshot()` 返回 PNG 字节，**必须**在同一 JS cell 里
   `nodeRepl.emitImage(...)` 转成 image 内容块给模型（`browserScreenshotPaths` 另存 artifact）。
   快照能回答的问题不截图（token 成本纪律）。

8. **可见性/生命周期**：`browser.capabilities.get("visibility").set(false|true)` 显示/隐藏面板；
   新建 IAB 标签自动展开右侧面板；标签跨轮次保留；后台会话不抢占前台 UI。

9. **安全边界**（`docs/safety.md`）：页面内容视为不可信，仅用于定位；`evaluate` 只读；IAB 上传
   明确不支持；坐标 CUA 仅在快照无法表达时使用。

### 2.3 宿主侧实现（本机应用二进制实证，✅）

> 证据来源：本机 `AppData\Local\Programs\ZCode\resources\app.asar` 解包（Electron 应用本体），
> 由调研子代理逐文件核验。

- **ZCode 是 Electron 应用**（非 Tauri/WebView2）：`app.asar/package.json` 为 `@zcode/desktop`
  v3.6.5，依赖 electron-updater、node-pty、react 19、**playwright-core 1.59.1**；electron-builder
  打包；主进程创建 BrowserWindow（`contextIsolation:true, nodeIntegration:false, webviewTag:true`）。
- **IAB 渲染层**：renderer 用 React 渲染原生 `<webview>` 元素，`partition="persist:zcode-embedded-browser"`
  （**独立持久 session = 独立登录态**）、`allowpopups`；含自由视口/缩放逻辑。
- **IAB 管理**：主进程 `BrowserViewManager` 经 `webContents.getType()==="webview"` 识别 guest，
  IPC 通道 `zcode:browser-view-ready/operation/visibility/viewport-changed/...`；连续截图泵
  `browser-screenshot-activity` 用 `webContents.capturePage()` 抓屏。
- **DOM 自动化引擎**：主进程读取 `playwright-core/lib/generated/injectedScriptSource.js`，以 IIFE
  注入 `globalThis.__zcodePlaywrightInjected = new (module.exports.InjectedScript())(...)`（testId
  属性 `data-testid`）——即 **IAB 复用 Playwright-core 的浏览器内注入脚本生成紧凑 AI/ARIA 树**。
- **输入事件**：`webContents.cdp.send("Input.dispatchMouseEvent"/"Input.dispatchKeyEvent"/...)`
  **CDP 派发**，非坐标注入页面。
- **登录态导入**：主进程 `importChromeCookies`（macOS 支持、Windows 暂不支持，与文档一致）。
- **cdp 后端（CLI 无头）**：`runChromeHelper` 以 `--headless=new --remote-debugging-port=0
  --user-data-dir=...` 拉起系统 Chrome，经 `http://host/json/list` 找 page target 的
  `webSocketDebuggerUrl` 再 WebSocket 直连。
- **extension 后端**：桌面/CLI bundle 内**未找到**实现细节，仅在协议 schema 中声明（后端类型
  枚举含 `iab|extension|cdp`）。
- **插件机制**：官方内置 7 个插件（browser-use / android-emulator / ios-simulator / document-skills /
  skill-creator / zcode-guide / restore-legacy-sessions）；插件清单查找
  `.zcode-plugin/plugin.json`（推荐）→ `.claude-plugin/plugin.json`（Claude Code 兼容回退）；
  注入 `ZCODE_PLUGIN_ROOT`（`CLAUDE_PLUGIN_ROOT` 互为别名）等环境变量 —— **显式兼容 Claude Code
  插件规范**；`node_repl` MCP 由宿主在插件列表中解析 browser-use 插件派生。

### 2.4 小结

ZCode 的浏览器自动化与开源界主流「**DOM 语义快照 → 语义定位 → 动作 → 最小观察**」范式完全一致。
它的特殊之处：浏览器是宿主应用的一部分（Electron `<webview>`，用户同屏可见）；控制协议自称
**Codex 兼容**（详见 3.2 验证）；指令层（skill）与运行时（bridge）分离，桌面 IAB 与 CLI 无头
复用同一套 `agent.browsers` API。

## 3. 外部同类项目调研

### 3.1 ZCode 公开资料

- 官网文档（https://zcode.z.ai/cn/docs/browser-use、/cn/docs/ADE-tools）描述功能面，**未公开底层技术**；
- 官网营销页/社区文章**未出现「Codex」字样**；「Codex 兼容」仅出现在随产品分发的插件文档里；
- GitHub 上无 ZCode/z.ai 桌面端开源仓库（z.ai 组织 `zai-org` 49 个仓库全为模型/反馈仓库）；
- 社区仅有用户级教程与评测（知乎/博客园/CSDN/segmentfault），无原理分析；
- 结论：ZCode 浏览器实现无公开源码，本文 2.2/2.3 节的本机实证是最可靠证据。

### 3.2 OpenAI Codex（📖 openai/codex 仓库 + 第三方复刻 + 泄漏 skill）

**核心结论：Codex 的浏览器能力不在开源 CLI 中，而在闭源 Electron 桌面 App 内；但对外契约是公开的。**

1. **CLI 无浏览器引擎**：`codex-rs/Cargo.toml` 与浏览器相关的仅 `webbrowser`（打开登录 URL）；
   仅有特性开关 `browser_use` / `browser_use_full_cdp_access` / `browser_use_external` /
   `computer_use`（`codex-rs/features/src/lib.rs`），配置经 app-server 透传给桌面 App。
2. **Codex Browser 对象模型（公开契约）**：`agent.browsers.get(id)/getDefault()/getForUrl()` →
   `Browser.tabs/user/capabilities` → `Tab.goto/url/title/screenshot/reload/dev/cua/playwright`。
   完整 API 定义闭源，但被多处复刻/引用：`storybookjs/storybook/agent-eval/lib/mcp/codex-browser-api.json`
   （1373 行，含 `browser.capabilities.get("visibility").set()`）；官方
   `openai/plugins` 仓库 skill 要求 `tab.playwright.domSnapshot()` / `screenshot()` / `tab.dev.logs()`。
3. **domSnapshot 格式有公开定义**：= Playwright `locator('body').ariaSnapshot()` 输出的
   **ariaSnapshot YAML 树**，逐 frame 拼接（iframe 加前缀）。格式规范见 Playwright 官方文档
   `docs/src/aria-snapshots.md`（`- role "name" [attr=value]`）。**这对自研实现极有价值**：
   不依赖 Playwright 本体也能产出兼容格式。
4. **双后端**：内置浏览器（iab）走 App 内 native pipe，由 `mcp__node_repl` 驱动（与 ZCode 同构）；
   Chrome 扩展（extension）经 native messaging host + 本地 Unix socket（`/tmp/codex-browser-use`，
   4 字节长度前缀 + JSON 帧）。CDP 仅作为能力出现（`tab.capabilities.cdp.send()`），非主路径。
5. **「3000ms 超时预算」需修正**：ZCode 插件文档称「Codex 的 3000ms」，但 OpenAI 全部公开来源中
   **无此说法**（Codex 侧实际数字：node_repl 默认 30s、storybook mock 动作 5s/导航 30s、Chrome
   native host 12s）。→ 3000ms 应视为 ZCode 自己的约定，Reasonix 采用时按自身模型/延迟调优。
6. **插件系统**：声明式 `.codex-plugin/plugin.json`（manifest 含 skills/mcp_servers/apps/hooks），
   marketplace 分发 —— ZCode 的插件机制是对它的兼容。
7. **Computer Use 独立**：桌面 GUI 自动化（截图→坐标动作），官方文档明确「本地构建 Web App 应
   优先使用内置浏览器」，不是 browser use 的底座。

### 3.3 其他开源编码智能体（📖 各仓库源码/文档）

| 项目 | 运行时 | 控制通道 | 观察方式 | 关键点 |
|------|--------|----------|----------|--------|
| **OpenHands** | Python browser-use 库（Playwright+Chromium，探测系统 Chrome 回退） | 原生 14 个 tool（browser_navigate/click(index)/type/get_state/...） | DOM 索引状态为主 + base64 截图多模态 + rrweb 录制 | 前端浏览器画面 = **截图回显**（`<img>`），非 iframe；V0.x 用 BrowserGym |
| **browser-use**（Python） | Playwright 驱动 Chromium（支持 connect 已有浏览器） | LLM 结构化动作（click_element 按 index 或坐标） | CDP Accessibility 树 + DOMSnapshot 序列化成带索引可交互元素清单（默认截断 40k 字符）；视觉可选 | 语义 index 点击为主、坐标为辅的典型混合方案 |
| **Chrome DevTools MCP** | Puppeteer CDP WebSocket（可启动或连接已有 Chrome） | MCP 工具（click/fill 用快照 uid；click_at 用坐标） | `take_snapshot` = a11y 树快照逐节点分配 uid + ExtraHandles 补全；screencast 帧流可实时回显 | 文档明确「Prefer snapshot over screenshot」 |
| **Playwright MCP** | Playwright 库（chromium/firefox/webkit/msedge） | MCP 工具（target 为快照 ref 或选择器） | 结构化 aria snapshot；`--snapshot-boxes` 可选坐标框 | **明确「不能基于截图做动作，用 snapshot」**；可选 vision/pdf/devtools caps |
| **Claude Code** | CLI 无内置浏览器（仅 WebFetch/WebSearch）；computer-use 工具存在（截图+坐标） | 「Claude in Chrome」扩展：MCP↔native host↔MV3 扩展（chrome.debugger） | content script 生成 a11y 树分配 ref_*（WeakRef 持久）；computer 工具截图+坐标 | 闭源但社区逆向（open-claude-in-chrome）完全揭示；58 域名黑名单 |
| **Goose**（Rust） | 无内置浏览器 | 纯 MCP 扩展（官方推荐 Playwright MCP / Chrome DevTools MCP / Browserbase MCP） | 同对应 MCP | 无内置=架构最简路径 |
| **opencode** | 无内置（webfetch/websearch）；生态插件 opencode-browser **CDP 直连运行中 Chrome**（9222 端点），无需扩展 | 插件工具 browser_list/navigate/snapshot/click/fill/eval/screenshot | `Accessibility.getFullAXTree` 递归文本树，节点带 `[uid]`/backendNodeId | **「CDP 直连、无扩展」最简参考实现**，snapshot.ts 全文可读 |
| **Qwen Code** | cua-driver（trycua/cua ~20MB 原生二进制）computer use + chrome 扩展 | 扩展 = **CDP 隧道**：service worker 经 `/acp` WebSocket 反向连本地守护进程，桥接 chrome.debugger；工具面复用 chrome-devtools-mcp | 窗口 a11y 树渲染 Markdown + 稳定 element_index；截图给视觉模型 | 「扩展 CDP 隧道免 native messaging」省去签名分发 |
| **Cursor**（IDE） | 内嵌 secure web view + MCP server 扩展控制 | agent 可 navigate/click/hover/type/scroll/screenshot | **观察以截图为主**（集成进读文件工具）+ console/network 文件 | 嵌入式 WebView + MCP + 截图多模态的代表 |
| **Windsurf**（IDE） | 内嵌 Chromium 预览（代理本地 dev server） | **agent 不能点击**；用户「Send element」点选 → @mention 注入 | 页面→agent 单向 | 预览+用户拾取型，不是 agent 驱动 |
| **Devin**（闭源） | Interactive Browser（截图/视频证据）+ Computer Use（1024×768 截图→动作） | 闭源；Chrome 经 **CDP 端口 29229** 暴露给外部脚本 | 截图/视频为主 | 唯一公开细节：CDP 端口暴露 |
| **Aider / Zed / Continue / Cherry Studio / Gemini CLI** | Aider/Zed/Continue 无浏览器（靠 MCP 或纯文本）；Gemini CLI 的 browser agent 直接代理 chrome-devtools-mcp（headless Chrome）+ 截图发给视觉模型换坐标 | — | — | Gemini 是「语义 agent（a11y 树）+ 视觉 agent（截图→坐标）」双通道 |

## 4. 模式归纳

### 4.1 分类表

| 模式 | 代表 | 优点 | 缺点 | 对 Go/Wails+WebView2 的参考 |
|------|------|------|------|------------------------------|
| **① CDP 驱动 Chromium + 自动化库** | browser-use、Playwright MCP、Chrome DevTools MCP、OpenHands、opencode-browser、Devin | 生态最成熟、跨平台、能力全（多标签/网络/console/截图）、与用户环境隔离 | 需分发浏览器或驱动用户 Chrome；无 GUI 面板需自做回显 | **最高**：Go 有成熟 CDP 库（chromedp/go-rod）；opencode-browser 的极小工具面可照抄 |
| **② 浏览器扩展 + 宿主桥接** | Claude in Chrome（native messaging）、Qwen chrome-bridge（WS 隧道免 native host） | 复用用户登录态与可见性；DOM 控制最强 | 用户装扩展、权限大、仅 Chromium 系、合规/安全面 | **中高**：Go 只写本地桥服务；Qwen 式 WS/CDP 隧道免签名分发 |
| **③ MCP 工具化** | Chrome DevTools MCP、Playwright MCP、Goose | 即插即用、跨客户端复用 | 多一层间接、schema+快照 token 开销 | **中**：Go 有 mcp-go；可作「原生工具为主 + 可选 MCP 兼容」双轨 |
| **④ 桌面内嵌 WebView + 宿主桥** | Cursor（agent 可驱动）、Windsurf（仅预览+拾取）、OpenHands 前端（截图回显） | 用户同屏可见、深度集成、无需额外装浏览器 | 多数项目 agent 不能真正驱动；webview 与真实站点兼容性受限 | **对 Wails 最贴合**：WebView2 面板可做回显/预览；agent 驱动走 WebView2 自身能力 |

### 4.2 观察方式收敛结论

所有成熟项目已收敛到同一范式：**a11y 树/DOM 语义快照为主（uid/ref/index 定位）+ 截图多模态为辅
（视觉兜底出坐标）+ console/network 补充**。Playwright MCP 直言「不能基于截图做动作」；
Chrome DevTools MCP / OpenHands / Claude in Chrome 均建议优先快照（token 成本低一个数量级、免视觉
模型）。ZCode/Codex 的快照格式 = Playwright ariaSnapshot YAML（公开规范）。

## 5. Reasonix 可行性评估

### 5.1 现状盘点（✅ 代码实证，commit 44f749eae）

**架构**：`internal/control.Controller`（controller.go，6689 行）为唯一传输无关内核，TUI /
HTTP-SSE / Wails desktop 三前端共享（REASONIX.md：「Add behavior to the controller, not a
frontend」）。新浏览器能力写在 controller 与 `internal/tool` 层，三前端自动继承。

**desktop 是嵌套 Go module（`module reasonix/desktop`）**，Wails v2.12.0 + `wailsapp/go-webview2
v1.0.23` 被 `replace` 到**本地 fork** `desktop/third_party/go-webview2`（desktop/go.mod）。
`webview2_patch_test.go` 用 AST 断言 fork 内 `chromium.go` 的 DPI patch 不被回退 —— **WebView2
COM 绑定完全在项目控制下，可自由扩展**。

**桌面绑定层**：`desktop/app.go` 的 `App` 方法经 Wails 绑定暴露；前端 `bridge.ts` 调
`window.go.main.App.*`，事件经 `window.runtime.EventsOn("agent:event")`（desktop/README.md 架构图）。

**WebView2 绑定清单**（fork 内 `pkg/edge/`）：

| 能力 | 状态 | 位置 |
|------|------|------|
| `ExecuteScript`（异步带回调） | ✅ 已绑定 | `corewebview2.go:295`（`Chromium.Eval` 封装 chromium.go:288） |
| `PostWebMessageAsString/JSON`（Go→页面） | ✅ 已绑定 | `corewebview2.go:386`、`ICoreWebView2_2.go:46-47` |
| `AddWebMessageReceived`（页面→Go） | ✅ 已绑定 | `corewebview2.go:251` |
| `AddScriptToExecuteOnDocumentCreated` | ✅ 已绑定 | `corewebview2.go:277`（`Chromium.Init`） |
| `Navigate` / `NavigateToString` | ✅ 已绑定 | `corewebview2.go:352,369` |
| Controller `PutBounds/PutIsVisible/MoveFocus/PutZoomFactor` | ✅ 已绑定 | `ICoreWebView2Controller.go:81,117,92,153` |
| `NotifyParentWindowPositionChanged` | ✅ 已绑定 | `ICoreWebView2Controller.go:142` |
| `createCoreWebView2EnvironmentWithOptions`（独立 user data folder） | ✅ 已绑定 | `create_env_native.go:15` |
| `GetCookieManager` | ✅ 已绑定 | chromium.go:704 |
| **`CallDevToolsProtocolMethod`（WebView2 的 CDP 通道）** | ⚠️ vtbl 已声明，封装方法缺 | `ICoreWebView2_2.go:50`（需补 ~30 行） |
| `GetDevToolsProtocolEventReceiver` | ⚠️ 同上 | `ICoreWebView2_2.go:56` |
| `CapturePreview`（截图） | ⚠️ vtbl 已声明，封装方法缺 | `corewebview2.go:150`（需补 ~20 行） |
| `CreateCoreWebView2ControllerWithOptions` | ❌ 未绑定（走默认 controller 创建） | chromium.go:341 |
| 独立多 webview 并存 | ❌ Wails 无官方支持 | 见 5.4 风险 |

**工具层**：
- `tool.Tool` 接口 + `tool.RegisterBuiltin(t)`（tool.go:241，init 注册）；`tool.ImageTool` 可选接口
  （tool.go:73，`ExecuteWithImages(ctx, args) (string, []string, error)`，图片 data URL 随结果回传），
  agent 层消费（`internal/agent/execute_one.go:644`、`usecapability.go:471,1142`）——
  **截图→vision provider 管道现成**（`toolimages_test.go` 独立测试）。
- 工具白名单 `[tools] enabled = []`（reasonix.example.toml:121，空=全部启用）——**新浏览器工具
  默认不进白名单 = 不进 system prompt = 缓存前缀（Cache-impact）零影响**，用户配置启用才生效。

**事件流**：`internal/event` 类型化事件 + Sink（event.go:434,547），desktop 经 eventSink 桥接
`runtime.EventsEmit`。浏览器状态（标签/标题/快照）可走同一通道推前端。

**Win32 先例**：`desktop/hang_watchdog_windows.go` 用 `EnumWindows` + `GetWindowThreadProcessId`
定位本进程 `wailsWindow` HWND —— 已有 Go 侧操作 Win32 窗口句柄的先例。

**前端面板体系**：React + Vite；右侧 dock `RightDockMode = "context" | "files" | "changed" |
"remote"`（frontend/src/store/layout.ts:141）+ `ResizableDrawer`；已有 WorkspacePanel / TerminalPanel /
RemotePanel 等。**无 BrowserPanel**，dock 模式加一档即可。

### 5.2 方案对比

| 方案 | 描述 | 判定 |
|------|------|------|
| **A. 第二 WebView2 控制器（推荐）** | 用自有 go-webview2 fork 创建第二个 Chromium，父窗口为 Win32 子窗口叠加到侧边栏区域；ExecuteScript + WebMessage 双向桥 + `CallDevToolsProtocolMethod`（CDP：输入派发/a11y 树/截图）；独立 userDataFolder = 独立登录态 | **选**：零新增依赖、复用现有绑定、保持单二进制分发哲学；与 ZCode/Codex 的 iab 语义最接近 |
| B. 打包 Chromium + CDP | 内置完整 Chromium（+120~170MB） | 否：违背「零摩擦单二进制」分发哲学；体积爆炸 |
| C. Python browser-use sidecar | browser-use（Python+Playwright）外部进程 | 否：违背纯 Go 哲学；额外运行时 |
| D. CDP 外接用户已装浏览器 | `--remote-debugging-port` 驱动系统 Chrome/Edge | 部分：作为 **CLI 无头模式运行时**有价值（对齐 zcode `cdp` 后端，opencode-browser 式），但不解决桌面面板 |
| E. Chrome 扩展 | MV3 扩展控制用户真实浏览器 | 部分：zcode 有 `extension` 后端；可作为远期补充（Qwen 式 WS 隧道），不是桌面面板方案 |

### 5.3 推荐方案 A 的架构设计

```
┌─────────────────────────────────────────────────────────────┐
│ React 前端（Wails 主 webview）                                │
│   BrowserPanel.tsx（新 dock 模式）+ bridge.ts                 │
│   事件：agent:event 新增 browser:* 类型                        │
├─────────────────────────────────────────────────────────────┤
│ desktop/app.go                                               │
│   BrowserHost：Win32 子窗口 + 第二 Chromium 生命周期/布局跟随   │
│   BrowserCommandBridge：命令 RPC ↔ ExecuteScript / WebMessage │
│     / CallDevToolsProtocolMethod（CDP）                      │
├─────────────────────────────────────────────────────────────┤
│ internal/tool/builtin/browser_*.go（新工具集，默认不进白名单） │
│   BrowserSession 接口（无实现不注册 → CLI 不受影响）            │
│   navigate / get_state(domSnapshot) / click / type / scroll  │
│   / screenshot(ImageTool) / pick_element（复用元素点选采集）   │
├─────────────────────────────────────────────────────────────┤
│ internal/control.Controller（三前端自动继承）                  │
└─────────────────────────────────────────────────────────────┘
```

**对照 ZCode/Codex 契约的实现映射**（建议 API 对齐 `agent.browsers`，保持生态兼容）：

| Codex/ZCode 契约 | Reasonix 实现路径 |
|------------------|-------------------|
| `domSnapshot()`（ariaSnapshot YAML） | **默认：CDP `Accessibility.getFullAXTree`**（浏览器原生，微软官方支持 WebView2 调用）+ 自写序列化层转紧凑 YAML（格式公开，零第三方代码）；备选：注入 playwright-core 310KB 注入脚本（ZCode 做法，Apache-2.0，合规=附 LICENSE/NOTICE） |
| locator 动作（getByRole/click/fill…） | 快照内嵌 ref（AX 树 backendNodeId 或生成 CSS 选择器），Go 侧 ExecuteScript 执行动作 |
| 输入（ZCode 用 CDP Input.* 派发） | `CallDevToolsProtocolMethod("Input.dispatchMouseEvent/KeyEvent")`（补封装即可）；或注入 `element.dispatchEvent` 合成事件 |
| `screenshot()` | `CapturePreview`（补封装）；或 CDP `Page.captureScreenshot` |
| 视口 setViewportSize | `PutBounds` + controller 缩放（自由尺寸模式语义） |
| 独立登录态 | 独立 userDataFolder（`createCoreWebView2EnvironmentWithOptions` 已绑定） |
| jsDialog 处理 | WebView2 `AddScriptDialogOpening`（需补绑定） |
| 标签持久/认领 | BrowserHost 内 Tab 注册表 + 事件推前端 |

**阶段划分**：

- **Phase 0 — Spike（先验证最高风险）**：在 Wails 窗口内创建第二 WebView2 控制器，叠加到前端
  侧边栏区域；验证 z-order（主 webview 是否覆盖子 webview）、焦点路由、DPI、frameless 窗口移动/
  缩放跟随。**通过才进入 Phase 1。**
- **Phase 1 — MVP（Windows only）**：`internal/tool/builtin/browser_*` 工具集（BrowserSession
  接口，desktop 注册、CLI 无实现不注册）+ `BrowserPanel.tsx` + goto/domSnapshot/click/type/
  screenshot + `@element` 点选（与 pick_element 共享采集 JS，成本≈0）。
- **Phase 2 — 跨平台**：macOS（WKWebView）与 Linux（WebKitGTK）各一套；或先做 CLI 无头 `cdp`
  后端（D 方案，opencode-browser 式直连系统 Chrome）对齐 zcode 的 `--browser-use=headless`。
- **Phase 3 — 增强**：Cookie 导入/清理（GetCookieManager 已绑定）、开发者工具入口
  （OpenDevToolsWindow 已绑定）、file:// 协议。

### 5.4 风险与未决问题

1. **多 webview z-order/焦点/DPI（最高风险）**：Wails v2.12 无官方多 webview 支持。但已实证
   **Wails 主 webview 就是主窗口的 WS_CHILD 子窗口**（`internal/frontend/desktop/windows/frontend.go:549`
   `chromium.Embed(f.mainWindow.Handle())` + `chromium.Resize()` 填满客户区）——第二个 webview 同为
   WS_CHILD 时 **z-order 可控（SetWindowPos）**，方向乐观，但鼠标命中/焦点/DPI 仍需实测。
   → Phase 0 spike 先行。降级路径：OpenHands 式「面板内截图回显」。
2. **跨平台注入差异（高）**：Windows WebView2（COM+CDP）与 macOS WKWebView / Linux WebKitGTK
   能力面不同；快照/截图/输入各写一套（工作量需各自代码级评估，Phase 2 独立立项）。
   CLI 无头 `cdp` 后端是天然跨平台替代。
3. **缓存前缀纪律**：浏览器工具 schema 进 system prompt 影响缓存前缀 → 按 REASONIX.md 默认关闭、
   配置启用（`[tools] enabled` 白名单天然支持）；PR 填 `Cache-impact:` 元数据。
4. **DOM 快照成本**：紧凑 ariaSnapshot（对齐公开格式）而非 outerHTML；截图仅视觉验证时走
   ImageTool 管道，受 provider 视觉能力约束。
5. **安全边界**：页面内容不可信（对齐 zcode safety.md）；evaluate 只读；动作审批复用现有
   `control` 层审批机制。
6. **远程工作区**：SSH/Docker 远程不提供浏览器面板（浏览器渲染在本地 webview，与 ZCode 一致）；
   远程会话中的浏览器工具报 capability_unsupported。
7. **未决**：WebView2 上 CDP domain 的完整度（微软官方支持 `CallDevToolsProtocolMethod`，CDP 由
   Chromium 维护，但个别 domain/方法可能有版本门槛——Phase 1 第一周验证 Input/Accessibility/
   Page 三域）；CDP 输入 vs 合成事件对 React/Web Components 页面的保真度；iframe 快照展开策略；
   下载/上传边界（对齐 ZCode 的 filechooser 限制）。

### 5.5 补充验证（2026-08-06 第二轮补查，回应「调研是否足够」）

针对第一版报告的薄弱点做了三项关键验证：

1. **Playwright 注入脚本可复刻性（✅ 实证）**：从 npm 拉取 playwright-core 1.59.1（ZCode 所用版本），
   确认 `lib/generated/injectedScriptSource.js` = **310KB CommonJS 产物**（`"use strict"` +
   `__toCommonJS` 包装，末尾 `module.exports`），ZCode 在主进程做 `__toCommonJS(injectedScript_exports)`
   处理后以 IIFE 注入页面实例化 `InjectedScript`。Reasonix 复刻此路径可行：Go 侧把 JS 文本经
   `ExecuteScript`/`AddScriptToExecuteOnDocumentCreated` 注入 WebView2 页面即可，无需 Node 运行时。
   **许可结论（2026-08-06 核实）**：playwright-core 为 Apache-2.0（附 NOTICE + ThirdPartyNotices.txt），
   嵌入 MIT 项目合法（非 copyleft），义务仅为保留版权声明/附 LICENSE 全文/NOTICE（加两个文本文件）。
   **但默认路线不嵌它**：改用浏览器原生 `Accessibility.getFullAXTree`（CDP，微软官方支持 WebView2 调用）
   + 自写序列化层转 ariaSnapshot 风格紧凑 YAML（格式公开）——Chrome DevTools MCP / browser-use /
   opencode-browser 均为此做法，**零第三方代码、零许可问题**；playwright 注入脚本仅作 CDP 树质量
   不足时的备选（合规动作已明确）。
2. **Wails 主 webview 窗口层级（✅ 实证）**：wails v2.12.0 源码（Go module cache）确认主 webview
   为 `edge.NewChromium()` + `chromium.Embed(f.mainWindow.Handle())`，即**主窗口的 WS_CHILD 子窗口**
   （`DataPath` 由 `WebviewUserDataPath` 配置，未设置时默认 `%AppData%/<exeName>`）。第二 webview
   的 z-order 竞争可以 `SetWindowPos(HWND_TOP)` 处理——从「未知」降级为「可 spike 验证的高风险点」。
3. **WebView2 的 CDP 支持（✅ 官方文档实证）**：微软官方文档
   （learn.microsoft.com/en-us/microsoft-edge/webview2/how-to/chromium-devtools-protocol）明确
   **WebView2 官方支持 CDP**（Win32 路径即 `CallDevToolsProtocolMethod` + 事件接收器），且 CDP 由
   Chromium 维护。方案 A 的「CDP 输入派发 + Accessibility 树 + Page.captureScreenshot」是官方
   supported 路径，不是 hack。个别 domain 的版本门槛列入 Phase 1 首周验证。

### 5.6 调研完备性声明（诚实边界）

**已实证（✅）**：ZCode 完整实现（插件源码 + 应用二进制）、Codex 契约与格式（ariaSnapshot 公开规范）、
行业范式收敛、Reasonix 全部相关代码面（tool/ImageTool/control/前端/go-webview2 fork 绑定清单）、
Wails 主 webview 窗口结构、Playwright 注入脚本可复刻性、WebView2 CDP 官方支持。

**只能靠 spike 验证（无法静态消除，已列入 Phase 0/1）**：① Wails 窗口内第二 webview 的
z-order/焦点/DPI 实测；② WebView2 上 CDP 三域（Input/Accessibility/Page）的实际行为与版本门槛；
③ CDP 输入对现代前端框架页面的保真度。

**未评估到代码级（实现阶段再做，不阻塞 Phase 0）**：跨平台 WKWebView/WebKitGTK 工作量、
审批机制与浏览器工具的具体挂钩、浏览器快照与 compaction/上下文压缩的交互、e2e 测试策略
（desktop 已有测试基础设施，desktop/README.md「make desktop-test」）。


## 6. 结论与建议

1. **行业范式已收敛**：所有主流实现（ZCode/Codex/OpenHands/browser-use/Chrome DevTools MCP/
   Playwright MCP/Claude in Chrome/Qwen Code）都采用「**a11y 树语义快照 → 语义定位 → 动作 →
   最小观察**」，截图只作视觉兜底。Reasonix 照此设计不会走弯路；快照格式直接对齐 Playwright
   ariaSnapshot（公开规范），兼容 Codex/ZCode 契约。
2. **ZCode 的实现 = Electron `<webview>` + Playwright-core 注入 + CDP 输入 + node_repl bridge**；
   Reasonix 的 Wails+WebView2 与之同构（WebView2 也是 Chromium，且 `CallDevToolsProtocolMethod`
   提供 CDP 通道），**方案 A（第二 WebView2 控制器）在技术上是成立的**，且零新增依赖。
3. **已绑定的 COM 接口覆盖 80% 需求**：ExecuteScript / 双向 WebMessage / 独立 env / Cookie
   管理 / 布局控制全部现成；只需补 `CallDevToolsProtocolMethod`、`CapturePreview` 两个薄封装
   （~50 行）与 script dialog 绑定。
4. **唯一真正的不确定性是「Wails 单窗口内叠加第二个 webview」**——z-order/焦点/DPI 手写方案
   是否可靠，必须 Phase 0 spike 实测；若失败，降级为「截图回显面板 + CDP 无头后端」（OpenHands
   路线）依然能交付 90% 的功能价值。
5. **许可无阻塞**：DOM 快照默认走浏览器原生 CDP `Accessibility.getFullAXTree`（零第三方代码，
   无 Apache-2.0 合规问题）；playwright 注入脚本仅作备选，其 Apache-2.0 嵌入合规成本 = 附带
   LICENSE/NOTICE 两个文本文件（已核实，非 copyleft）。
6. **工程路径建议**：Phase 0 spike（Windows）→ Phase 1 MVP（工具集 + BrowserPanel + 元素点选，
   默认关闭不碰缓存前缀）→ Phase 2 跨平台/CLI 无头 → Phase 3 增强。

---

## 附录 A：调研证据索引

| 证据 | 来源 | 类型 |
|------|------|------|
| ZCode 插件全部源码/文档 | 本机 `~/.zcode/cli/plugins/cache/zcode-plugins-official/browser-use/0.1.2/` | ✅ |
| ZCode 应用二进制（app.asar 解包） | 本机 `AppData\Local\Programs\ZCode\resources\` | ✅ |
| Codex 特性开关/配置 | github.com/openai/codex（codex-rs/features、core/config.schema.json、app-server） | 📖 |
| Codex Browser API 复刻 | github.com/storybookjs/storybook（agent-eval/lib/mcp/codex-browser-api.json） | 📖 |
| Codex 官方插件 skill | github.com/openai/plugins | 📖 |
| 泄漏 skill（control-in-app-browser / control-chrome） | github.com/asgeirtj/system_prompts_leaks | 📖 |
| Codex 桌面 App 逆向 | github.com/kkkzbh/linux-codex-app | 📖 |
| ariaSnapshot 格式规范 | playwright.dev（docs/src/aria-snapshots.md） | 📖 |
| OpenHands | github.com/OpenHands/OpenHands、OpenHands/software-agent-sdk | 📖 |
| browser-use | github.com/browser-use/browser-use | 📖 |
| Chrome DevTools MCP | github.com/ChromeDevTools/chrome-devtools-mcp | 📖 |
| Playwright MCP | github.com/microsoft/playwright-mcp | 📖 |
| Claude in Chrome 逆向 | github.com/noemica-io/open-claude-in-chrome | 📖 |
| opencode-browser | github.com/different-ai/opencode-browser | 📖 |
| Qwen Code | github.com/QwenLM/qwen-code（packages/chrome-extension） | 📖 |
| Cursor / Windsurf / Devin | cursor.com/docs/agent/tools/browser、docs.devin.ai | 📖 |
| Reasonix 代码 | 本仓库 commit 44f749eae（desktop/、internal/tool、internal/control） | ✅ |
| playwright-core 1.59.1 注入脚本（310KB CommonJS） | npm registry（playwright-core@1.59.1） | ✅ |
| Wails v2.12.0 主 webview 创建方式 | Go module cache `wails/v2@v2.12.0/internal/frontend/desktop/windows/frontend.go:549` | ✅ |
| WebView2 CDP 官方支持 | learn.microsoft.com/en-us/microsoft-edge/webview2/how-to/chromium-devtools-protocol | 📖 |

## 7. 实现结果与实测记录（2026-08-06 完成）

> 分支 feat/browser-panel 已实现完整功能并通过测试。本节记录实现内容与实测证据。

### 7.1 交付内容

**Go 侧（desktop/ + internal/）**：
- `desktop/browserhost_windows.go` — Windows WebView2 原生面板：自建 hostLoop 线程（锁定 OS 线程 + 消息泵）承载第二 WebView2；主 webview 收缩让位（非重叠布局）；懒加载（点加号才创建，关闭即销毁）；subclass + 任务注册表把主 webview 操作投递到 winc 主线程（零指针跨界，vet 安全）；WM_SIZE 同步
- `desktop/browsercdp_windows.go` — WebView2 CDP 传输层（`CallDevToolsProtocolMethod` 封装，补 ~50 行 COM 绑定：handler/方法/chromium 封装 + `RegisterMainChromium` 全局注册 + `Reload/GetDocumentTitle/EvalJS/Destroy`）
- `desktop/browser_cdp_core.go` — 平台无关 CDP 核心：AX 树 → ariaSnapshot 风格 YAML（[ref=N]）+ 点击/键入/按键/滚动/截图/评估；`axID` 兼容 string/number 编码的节点 id（Edge 实测发现）；ignored 节点透传（Chromium 隐藏包装节点）
- `desktop/browser_chrome.go` — 跨平台 CDP 后端（macOS/Linux）：headless 系统 Chrome/Edge + WebSocket 客户端（gorilla/websocket 提升为直接依赖，零新依赖）+ 截图回显 + 状态轮询；进程树按 PID 清理
- `desktop/browserhost_cdp_other.go` — !windows 的 BrowserHost 壳（截图回显面板模式）
- `desktop/browser_app.go` + `browser_session.go` — Wails 绑定（BrowserOpen/Close/Navigate/…/ClearCache/ClearAllData/ControlEnabled）+ Agent 工具 session adapter（开关门控）
- `internal/tool/builtin/browser.go` + `browser_test.go` — 9 个 browser_* 工具 + BrowserSession 接口 + 10 个单测；`internal/boot` 加 `ExtraTools`（CLI 不传 = 缓存前缀零影响）；`internal/config` 加 `[desktop] browser_enabled`
- fork 扩展：`CallDevToolsProtocolMethod`（+handler）、`RegisterMainChromium`、`Reload/GetDocumentTitle/EvalJS/Destroy`

**前端（desktop/frontend/）**：
- 右侧 dock tabs 新增「浏览器」按钮（+ 图标新建 / Globe 高亮关闭）
- `BrowserPanel.tsx` 双模式：native（Windows，宽度拖拽 resizer，上限 2/3 视口）/ remote（截图回显 + React 地址栏）
- `store/browser.ts` + `browser:state` / `browser:screenshot` 事件
- 设置页新增「浏览器」tab：开启内置浏览器控制开关（新会话生效）+ 清除缓存（保留 Cookie）/ 清除全部（确认按钮）
- 页面内注入 chrome bar（地址栏/后退/前进/刷新/关闭，`data-reasonix-bar` 标记，快照排除）

### 7.2 测试证据

| 测试 | 结果 |
|------|------|
| desktop 全量 `go test .`（462s，含新 CDP 集成测试与全部既有测试） | ✅ PASS |
| `TestChromeCDPNavigateSnapshot`（真实 Edge：导航→AX 快照→ref 索引） | ✅ PASS |
| `TestChromeCDPClickLinkAndType`（真实点击链接→导航→聚焦→键入→值验证） | ✅ PASS |
| `TestChromeCDPScreenshot`（PNG 魔数 + 尺寸） | ✅ PASS |
| `TestChromeProfileCleanup`（清缓存保留 Cookie） | ✅ PASS |
| `TestBrowser*` 10 个工具单测（fake session：开关门控/参数校验/ImageTool 管道） | ✅ 10/10 PASS |
| `go vet ./...`（desktop + root） | ✅ 全绿 |
| 前端 `tsc --noEmit` + `pnpm build`（lint/CSS/bundle 预算） | ✅ 通过 |
| GOOS=linux / darwin 交叉编译（仅剩 cgo 环境固有错误，browser 代码零错误） | ✅ |
| CLI `go build ./cmd/reasonix` | ✅ 通过 |
| 桌面应用启动实测（REASONIX_HOME 隔离实例） | ✅ 正常启动 |

### 7.3 实测中发现并修复的关键问题（均为真实浏览器验证）

1. **WebView2 消息线程**：必须与创建线程同线程泵消息（否则窗口无响应 + webview 不可见）→ hostLoop 线程模型
2. **WebView2 controller bounds 初始为 0,0,0,0**：必须显式 PutBounds（否则内容超出父窗口被裁剪）
3. **DPI awareness**：fork 用 RAW_PIXELS 模式，进程必须 Per-Monitor DPI aware
4. **双 WebView2 叠加 z-order 不可靠**（SetWindowPos(HWND_TOP) 无效）→ 非重叠布局（主 webview 收缩让位）最终方案
5. **Edge headless 的 AX 树 nodeId 为字符串** → axID 灵活解析
6. **Chromium 的 ignored 中间节点**会阻断遍历 → 透传子树
7. **browserhost_js.go 文件名陷阱**：`_js` 是合法 GOARCH（wasm），文件被构建排除 → 改名
8. **CDP 属性值可为 bool/number** → axVal 任意类型 + str()

### 7.4 平台支持与已知边界

- **Windows**：WebView2 原生面板（实时、可交互、独立登录态、宽度拖拽）
- **macOS/Linux**：CDP 驱动系统 Chrome/Chromium/Edge（headless）+ 截图回显面板（地址栏 React 渲染）；需本机安装浏览器（或 `REASONIX_BROWSER_PATH`）
- **未在本机实测**：macOS/Linux 真机构建（需要对应平台环境与 GTK/WebKit 工具链）——代码已通过交叉编译验证，真机验证留待有环境时进行
- **数据清理**：Windows 清 WebView2 的 EBWebView 缓存目录；macOS/Linux 清 Chrome profile 缓存目录（Cookie 均保留）
- **UI 交互实测**：本机验证受限（测试期间窗口被其他程序遮挡），dock 按钮/面板交互建议用户实机点按确认

## 附录 B：实现文件清单

| 文件 | 说明 |
|------|------|
| desktop/browserhost_windows.go | Windows WebView2 面板宿主（线程模型/生命周期/布局/导航/清理） |
| desktop/browserhost_win32.go | Win32 辅助（消息泵/任务注册表/subclass） |
| desktop/browserhost_panelui.go | 页面内 chrome bar JS + 欢迎页 |
| desktop/browsercdp_windows.go | WebView2 CDP 传输层 |
| desktop/browser_cdp_core.go | 平台无关 CDP 核心（快照/动作/截图） |
| desktop/browser_chrome.go | Chrome/Edge CDP 驱动（跨平台后端） |
| desktop/browserhost_cdp_other.go | !windows BrowserHost（截图回显模式） |
| desktop/browser_app.go | Wails 绑定方法 |
| desktop/browser_session.go | Agent 工具 session adapter |
| desktop/browser_chrome_test.go | CDP 集成测试（真实浏览器） |
| internal/tool/builtin/browser.go | browser_* 工具集 + BrowserSession 接口 |
| internal/tool/builtin/browser_test.go | 工具单测 |
| internal/boot/boot.go | ExtraTools 字段 |
| internal/config/config.go + edit.go | browser_enabled 设置 |
| desktop/frontend/src/components/BrowserPanel.tsx + .css | 面板组件（双模式） |
| desktop/frontend/src/store/browser.ts | 浏览器 store |
| desktop/frontend/src/components/SettingsPanel.tsx | 设置页浏览器 tab |
| desktop/third_party/go-webview2/pkg/edge/*.go | fork 扩展（CDP/注册/销毁） |
