# 我对 DeepSeek-Reasonix 的贡献

[English](README.en.md) | 中文

此 fork 展示了我对上游项目
[esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)
的贡献；安装、发布和正式文档请前往上游仓库。

## 已合并的贡献

| PR | 状态 | 日期 | 贡献内容 | PR 元数据 |
| --- | --- | --- | --- | --- |
| [#8784](https://github.com/esengine/DeepSeek-Reasonix/pull/8784) | 已合并 | 2026-08-16 | 为内置终端接入完整的剪贴板工作流：Ctrl+C/Cmd+C 精确复制活动选区（无选区时仍作为 SIGINT 直达 PTY），右键菜单提供复制/粘贴/添加到聊天，选区以带 `source: "terminal"` 的结构化引用进入 composer；异步复制清理同时校验终端 generation 与选区 revision，选区状态按标签隔离；修复浅色模式近白选区不可见，显式设置 selectionForeground 保证各主题 WCAG 对比度 ≥ 4.3:1。修复 #8475, #7845（另引用 #7990, #8474）。集成 commit 带 #7580 的 Co-authored-by 记录。 | 27 文件, +1223/-169 |
| [#8099](https://github.com/esengine/DeepSeek-Reasonix/pull/8099) | 已合并 | 2026-08-09 | 修正 #6931 引入的刷新粒度：状态栏吞吐改为每次 executor `usage` 事件后按最新请求更新，仅在会话尚无请求时以已完成回合 TPS 兜底；慢请求（<1 t/s）显示 `<1 t/s` 而非旧值，无法测得的请求显示 `-`。 | 5 文件, +176/-32 |
| [#6931](https://github.com/esengine/DeepSeek-Reasonix/pull/6931) | 已合并 | 2026-08-08 | 底部状态栏新增 tok/s 吞吐、缓存 token 和输出 token 显示，输入框上方 run strip 新增流式吞吐量估算。 | 18 文件, +569/-65 |
| [#7503](https://github.com/esengine/DeepSeek-Reasonix/pull/7503) | 已合并 | 2026-08-05 | 使用统计图表改用 Primer 深浅两套配色：前 5 模型按排名各占系列色（`--chart-1`~`--chart-5`），其余归入灰色 Other；修复环形图 hover 溢出裁剪；命令面板新增使用统计入口。 | 9 文件, +310/-134 |
| [#7362](https://github.com/esengine/DeepSeek-Reasonix/pull/7362) | 已合并 | 2026-08-04 | 修复设置页窄窗口（≤900px）下的三个布局问题：子标签页撑满整行、记忆页面工作区选择器与建议按钮分离、子代理内置覆盖卡片溢出设置列。 | 4 文件, +108/-20 |
| [#7238](https://github.com/esengine/DeepSeek-Reasonix/pull/7238) | 已合并 | 2026-08-03 | 新增用量统计面板：含日内 token 热力图、趋势折线堆叠图、模型用量饼图；支持 7/14/30/90 天及自定义时间范围；所有入口（桌面端/CLI/HTTP/bot/Remote Workbench）统一通过 `stats.Recorder` 记录；纯 SVG 手绘无第三方图表库；平均缓存命中率与模型归属统计。 | 64 文件, +4332/-155 |
| [#7072](https://github.com/esengine/DeepSeek-Reasonix/pull/7072) | 已合并 | 2026-07-31 | 将终端重构为独立底部抽屉，添加拖拽调整大小手柄和手风琴动画，修复与右侧面板的布局冲突。修复 #7046, #7047。 | 7 文件, +420/-51 |
| [#7069](https://github.com/esengine/DeepSeek-Reasonix/pull/7069) | 已合并 | 2026-07-30 | 通过重构布局为状态栏分配独立 grid 行，修复决策卡片溢出被底部信息栏遮挡的问题。修复 #7030。 | 4 文件, +95/-56 |
| [#7064](https://github.com/esengine/DeepSeek-Reasonix/pull/7064) | 已合并 | 2026-07-30 | 修复桌面端切换会话和重启后粘贴文本和文件引用内联卡片丢失折叠状态。关闭 #7051。 | 4 文件, +235/-25 |
| [#6995](https://github.com/esengine/DeepSeek-Reasonix/pull/6995) | 已合并 | 2026-07-30 | 修复切换会话后右侧栏和状态栏数据丢失，持久化单轮 token 构成明细并修复 hydration 竞态条件。修复 #5335, #5766, #7068。 | 6 文件, +203/-19 |
| [#6645](https://github.com/esengine/DeepSeek-Reasonix/pull/6645) | 已合并 | 2026-07-19 | 新增按场景的面板透明度控制，修复自定义主题保存后基础风格回退问题。 | 26 文件, +456/-72 |
| [#6539](https://github.com/esengine/DeepSeek-Reasonix/pull/6539) | 已合并 | 2026-07-20 | 修复供应商模型较多或名称较长时设置页面模型列表选项重叠。关闭 #5563, #5585, #5785, #6480, #6723。 | 2 文件, +6/-2 |
| [#6252](https://github.com/esengine/DeepSeek-Reasonix/pull/6252) | 已合并 | 2026-07-09 | 修复创作风格下机器人设置字段和扫码设置面板右侧溢出。关闭 #6064, #6196。 | 1 文件, +25/-11 |
| [#6019](https://github.com/esengine/DeepSeek-Reasonix/pull/6019) | 已合并 | 2026-07-08 | 修复窄窗口下设置侧边栏突变为横向排列，改用 `clamp()` 连续响应式过渡。关闭 #5985。 | 1 文件, +7/-60 |
| [#6004](https://github.com/esengine/DeepSeek-Reasonix/pull/6004) | 已合并 | 2026-07-05 | 新增 `REASONIX_HOME` 环境变量支持，用于隔离配置、技能和输出风格目录扫描。关闭 #5988。 | 19 文件, +322/-49 |
| [#5906](https://github.com/esengine/DeepSeek-Reasonix/pull/5906) | 已合并 | 2026-07-04 | 为输入框和消息气泡中的图片附件增加点击预览功能。关闭 #5832。 | 10 文件, +421/-27 |
| [#5887](https://github.com/esengine/DeepSeek-Reasonix/pull/5887) | 已合并 | 2026-07-03 | 修复桌面端粘贴文本在消息气泡中仅显示折叠标签而非实际内容。关闭 #5863。 | 7 文件, +211/-4 |

已合并总计：17 个 PR，214 个变更文件，+9119/-951 行。

## 待审核的贡献

| PR | 状态 | 日期 | 贡献内容 |
| --- | --- | --- | --- |
| [#7980](https://github.com/esengine/DeepSeek-Reasonix/pull/7980) | 待审核 | 2026-08-08 | 桌面端新增可选"自动生成会话标题"：每次新会话由模型生成简短侧栏标题（默认关闭，可单独指定标题模型），修复 Goal 模式首轮标题缺失，将 Serve 与桌面共享的标题生成提取为 internal/title 核心（按协议禁用思考、剥离 think 块、空结果重试）。关闭 #7858。维护者 #8980（v1.25.4 已合并）在 body 中将本 PR 引为更宽泛的自动标题架构探索，但其更窄的显式 AI 重命名加固未采用本 PR 代码。 |
| [#7868](https://github.com/esengine/DeepSeek-Reasonix/pull/7868) | 待审核 | 2026-08-07 | 修复气泡复制按钮复制占位符而非实际内容、运行中引导消息泄漏原始传输围栏的问题：复制时展开折叠的粘贴/选区块为完整文本，steer 消息经共享显示恢复链路折叠为内联可展开卡片。跟进 #7064。 |
| [#6084](https://github.com/esengine/DeepSeek-Reasonix/pull/6084) | 待审核 | 2026-07-06 | 将整个代码库的文件排序从字典序替换为自然排序（侧边栏、CLI、文件引用）。关闭 #6042。 |

## 被维护者吸收的贡献

这些 PR 在维护者将其内容整合到自己的 PR 中后由我关闭：

| 我的 PR | 维护者 PR | 日期 | 关系 |
| --- | --- | --- | --- |
| [#7631](https://github.com/esengine/DeepSeek-Reasonix/pull/7631) | [#7737](https://github.com/esengine/DeepSeek-Reasonix/pull/7737) | 2026-08-06 | 恢复重试 usage 翻倍的诊断与分尝试计费设计被维护者审查采纳，重写为官方"原子重放中断流"实现并取代 #7631；未直接采用代码。 |
| [#6009](https://github.com/esengine/DeepSeek-Reasonix/pull/6009) | [#6764](https://github.com/esengine/DeepSeek-Reasonix/pull/6764) | 2026-07-21 | 共享 UpdaterProvider 更新状态设计被明确采纳并适配到当前 main-v2（以渲染双消费者状态共享回归测试替换静态契约测试），适配提交带 `Co-authored-by` 记录。 |
| [#6726](https://github.com/esengine/DeepSeek-Reasonix/pull/6726) | [#6821](https://github.com/esengine/DeepSeek-Reasonix/pull/6821) | 2026-07-22 | 分段按钮等宽方案被明确整合，并附带 `Co-authored-by` 提交记录。 |
| [#5943](https://github.com/esengine/DeepSeek-Reasonix/pull/5943) | [#6677](https://github.com/esengine/DeepSeek-Reasonix/pull/6677) | 2026-07-19 | 逐模型 `context_window` 覆盖功能被重写并作为官方实现落地。 |
| [#5872](https://github.com/esengine/DeepSeek-Reasonix/pull/5872) | [#6889](https://github.com/esengine/DeepSeek-Reasonix/pull/6889) | 2026-07-24 | MCP 持久化禁用概念被整合到更广泛的"默认信任"重构中。 |
| [#6783](https://github.com/esengine/DeepSeek-Reasonix/pull/6783) | [#7159](https://github.com/esengine/DeepSeek-Reasonix/pull/7159) | 2026-08-02 | 5 层透明度体系与 Windows 无边框侧栏压缩被完整整合：原始提交逐行原样并入（327 行中 321 行保留，作者署名保留），适配提交带 `Co-authored-by` 记录。关闭 #5825。 |
| [#6104](https://github.com/esengine/DeepSeek-Reasonix/pull/6104) | [#7159](https://github.com/esengine/DeepSeek-Reasonix/pull/7159) | 2026-08-02 | 侧栏标签压缩方案并入主题面板透明度体系，最终随 #7159 完整落地。 |

## 维护者 PR 中致谢的 Bug 报告

| Issue / 报告 | 维护者 PR | 日期 | 描述 |
| --- | --- | --- | --- |
| [#6590](https://github.com/esengine/DeepSeek-Reasonix/issues/6590) | [#6694](https://github.com/esengine/DeepSeek-Reasonix/pull/6694) | 2026-07-19 | 报告了设置刷新后主题配色回退的问题；修复已合并并明确致谢。 |

## 发布说明致谢

我的贡献在以下发布说明中获得致谢：

- [v1.25.4](https://github.com/esengine/DeepSeek-Reasonix/pull/8984) — 2026-08-16（集成终端剪贴板与选中内容添加到聊天，#8784 列为重点内容，致谢列表包含 HaoyueQin）
- [v1.22.0](https://github.com/esengine/DeepSeek-Reasonix/pull/8121) — 2026-08-10（状态栏 TPS 每次请求后更新，致谢列表包含 HaoyueQin）
- [v1.21.4](https://github.com/esengine/DeepSeek-Reasonix/pull/8039) — 2026-08-09（状态栏吞吐显示）
- [v1.20.0](https://github.com/esengine/DeepSeek-Reasonix/pull/7622) — 2026-08-05（Primer 配色图表）
- [v1.19.6](https://github.com/esengine/DeepSeek-Reasonix/pull/7471) — 2026-08-04（设置页响应式布局）
- [v1.19.5](https://github.com/esengine/DeepSeek-Reasonix/pull/7364) — 2026-08-03（用量统计面板）
- [v1.19.0](https://github.com/esengine/DeepSeek-Reasonix/pull/7117) — 2026-08-01（终端抽屉重新设计、粘贴文本与文件引用卡片、右侧栏和状态栏数据持久化、决策卡片溢出）
- [v1.19.0-preview.1](https://github.com/esengine/DeepSeek-Reasonix/pull/7116) — 2026-08-01（同上四条）
- [v1.17.17](https://github.com/esengine/DeepSeek-Reasonix/pull/6743) — 2026-07-22
- [v1.17.16](https://github.com/esengine/DeepSeek-Reasonix/pull/6709) — 2026-07-20

## 贡献主题

- **桌面主题系统**：面板透明度控制、场景级透明度分层（经 #7159 完整落地）、保存/应用状态正确性、Windows 无边框窗口下侧栏标签压缩。
- **使用统计面板**：纯前端 SVG 手绘热力图、趋势图与饼图，统一统计所有产品入口的 token 用量、缓存命中率与模型分布。
- **UI Bug 修复**：模型列表重叠、粘贴文本显示、图片预览、安全区域按钮宽度、决策卡片溢出、自然文件排序、设置页窄窗口响应式布局。
- **终端抽屉与剪贴板**：将终端重构为独立底部抽屉，支持拖拽调整大小与手风琴动画；接入复制/粘贴与选中内容添加到聊天，选区状态按标签隔离。
- **配置隔离**：`REASONIX_HOME` 环境变量支持隔离配置、技能和输出风格扫描。
- **功能提案**：逐模型上下文窗口覆盖、MCP 持久化禁用、状态栏吞吐量显示、自动生成会话标题。

## 分支用途

此分支作为我 fork 的展示页面。常规开发分支可以保持与上游同步，而此分支为访问者提供一个快速、可读的视图，展示已在官方仓库落地的具体贡献。

## 数据来源

由 `HaoyueQin` 提交的上游已关闭 PR：

https://github.com/esengine/DeepSeek-Reasonix/pulls?q=is%3Apr+is%3Aclosed+author%3AHaoyueQin

最后更新：2026-08-18。
