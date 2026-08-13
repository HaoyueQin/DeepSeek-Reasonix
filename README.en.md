# My Contributions to DeepSeek-Reasonix

English | [中文](README.md)

This fork showcases my contributions to the upstream
[esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix)
project; installation, releases, and canonical documentation belong there.

## Landed Contributions

| PR | Status | Date | Contribution | PR metadata |
| --- | --- | --- | --- | --- |
| [#8099](https://github.com/esengine/DeepSeek-Reasonix/pull/8099) | Merged | 2026-08-09 | Fixed the refresh granularity introduced in #6931: the status bar now refreshes throughput after every executor `usage` event with the latest request rate, falling back to the completed-turn TPS only before the first request; measurable slow requests render as `<1 t/s` instead of stale values, and unmeasurable latest requests show `-`. | 5 files, +176/-32 |
| [#6931](https://github.com/esengine/DeepSeek-Reasonix/pull/6931) | Merged | 2026-08-08 | Added tok/s throughput, cache token counts, and output tokens to the status bar with streaming estimation in the run strip. | 18 files, +569/-65 |
| [#7503](https://github.com/esengine/DeepSeek-Reasonix/pull/7503) | Merged | 2026-08-05 | Replaced the model usage chart monochrome ramp with GitHub Primer's two-set categorical palette: top 5 models each get a series colour (--chart-1..5), the rest collapse into gray Other; fixed donut hover overflow clipping; added usage-stats command palette entry. | 9 files, +310/-134 |
| [#7362](https://github.com/esengine/DeepSeek-Reasonix/pull/7362) | Merged | 2026-08-04 | Fixed three settings-page layout problems at ≤900px: subtabs stretching full width, workspace selector separated from Suggestions button on the memory page, and built-in override cards overflowing the subagents column. | 4 files, +108/-20 |
| [#7238](https://github.com/esengine/DeepSeek-Reasonix/pull/7238) | Merged | 2026-08-03 | Added a usage statistics panel with per-day token heatmap, daily stacked trend chart, and model-usage donut chart; supports 7/14/30/90-day and custom date ranges; all entry points (desktop/CLI/HTTP/bot/Remote Workbench) record through the same `stats.Recorder`; hand-drawn SVG, no third-party chart library; average cache hit rate and model attribution are included. | 64 files, +4332/-155 |
| [#7072](https://github.com/esengine/DeepSeek-Reasonix/pull/7072) | Merged | 2026-07-31 | Separated the terminal as an independent bottom drawer with a resize handle and accordion animation, repaired layout conflicts with right dock. Fixes #7046, #7047. | 7 files, +420/-51 |
| [#7069](https://github.com/esengine/DeepSeek-Reasonix/pull/7069) | Merged | 2026-07-30 | Fixed decision card overflow being hidden behind the status bar by restructuring the layout with a dedicated status bar grid row. Fixes #7030. | 4 files, +95/-56 |
| [#7064](https://github.com/esengine/DeepSeek-Reasonix/pull/7064) | Merged | 2026-07-30 | Fixed pasted text and file ref inline cards losing collapse state across session switches and app restarts. Closes #7051. | 4 files, +235/-25 |
| [#6995](https://github.com/esengine/DeepSeek-Reasonix/pull/6995) | Merged | 2026-07-30 | Fixed right-panel and status-bar data loss across session switches by persisting per-turn token breakdown and fixing hydration race conditions. Fixes #5335, #5766, #7068. | 6 files, +203/-19 |
| [#6645](https://github.com/esengine/DeepSeek-Reasonix/pull/6645) | Merged | 2026-07-19 | Added per-scene pane opacity controls and fixed the save/apply state flow for custom themes. | 26 files, +456/-72 |
| [#6539](https://github.com/esengine/DeepSeek-Reasonix/pull/6539) | Merged | 2026-07-20 | Fixed model list option overlap when a provider returns many models or long names. Closes #5563, #5585, #5785, #6480, #6723. | 2 files, +6/-2 |
| [#6252](https://github.com/esengine/DeepSeek-Reasonix/pull/6252) | Merged | 2026-07-09 | Fixed bot settings field overflow in Creation style and right-side overflow in the QR setup panel. Closes #6064, #6196. | 1 file, +25/-11 |
| [#6019](https://github.com/esengine/DeepSeek-Reasonix/pull/6019) | Merged | 2026-07-08 | Fixed the settings sidebar flipping to horizontal tabs on narrow windows via continuous `clamp()`-based responsive transitions. Closes #5985. | 1 file, +7/-60 |
| [#6004](https://github.com/esengine/DeepSeek-Reasonix/pull/6004) | Merged | 2026-07-05 | Added `REASONIX_HOME` environment variable support for isolated configuration, skills, and output-style directory scanning. Closes #5988. | 19 files, +322/-49 |
| [#5906](https://github.com/esengine/DeepSeek-Reasonix/pull/5906) | Merged | 2026-07-04 | Added click-to-preview for image attachments in composer and message bubbles. Closes #5832. | 10 files, +421/-27 |
| [#5887](https://github.com/esengine/DeepSeek-Reasonix/pull/5887) | Merged | 2026-07-03 | Fixed pasted text showing only fold labels instead of content in message bubbles. Closes #5863. | 7 files, +211/-4 |

Merged total from the upstream PR metadata above: 16 PRs, 187 changed-file entries,
+7896/-782 lines.

## Open Contributions

| PR | Status | Date | Contribution |
| --- | --- | --- | --- |
| [#8784](https://github.com/esengine/DeepSeek-Reasonix/pull/8784) | Open | 2026-08-13 | Wired clipboard interaction into the integrated terminal: Ctrl+C/Cmd+C copy a live selection and swallow the chord (without a selection the key still reaches the PTY as SIGINT), a right-click menu offers Copy / Paste / Add-to-chat, and selecting output raises the same floating "Add to chat" action used by the transcript; fixed the near-invisible light-mode selection highlight and set an explicit selectionForeground for ≥4.3:1 WCAG contrast in every theme. Fixes #7990, #8474, #8475, #7845. |
| [#7980](https://github.com/esengine/DeepSeek-Reasonix/pull/7980) | Open | 2026-08-08 | Added an opt-in "auto-generate session titles" desktop setting: each new session's sidebar title comes from one short LLM request (off by default, optional dedicated title model), fixed missing titles on Goal first turns, and extracted the title-generation core shared by Serve and the desktop into internal/title (per-protocol reasoning disablement, think-block stripping, empty-result retries). Closes #7858. |
| [#7868](https://github.com/esengine/DeepSeek-Reasonix/pull/7868) | Open | 2026-08-07 | Fixed bubble copy button copying placeholders instead of content and steer messages leaking raw transport framing: the copy button now expands folded paste/selection labels to full text, and steer messages recover through the shared display-recovery chain into inline expandable cards. Follow-up to #7064. |
| [#6084](https://github.com/esengine/DeepSeek-Reasonix/pull/6084) | Open | 2026-07-06 | Replaced lexicographic file sorting with natural sort across the entire codebase (sidebar, CLI, file references). Closes #6042. |

## Contributions Absorbed by Maintainer

These PRs were closed by me after the maintainer incorporated the work into their own PRs with acknowledgment:

| My PR | Maintainer PR | Date | Relationship |
| --- | --- | --- | --- |
| [#7631](https://github.com/esengine/DeepSeek-Reasonix/pull/7631) | [#7737](https://github.com/esengine/DeepSeek-Reasonix/pull/7737) | 2026-08-06 | Diagnosis and per-attempt billing design for the recovery-usage doubling were reviewed and adopted as the basis for the official atomic stream-replay implementation that supersedes #7631; no code was directly incorporated. |
| [#6009](https://github.com/esengine/DeepSeek-Reasonix/pull/6009) | [#6764](https://github.com/esengine/DeepSeek-Reasonix/pull/6764) | 2026-07-21 | Shared `UpdaterProvider` update-state design explicitly adapted onto current `main-v2` (the static contract test was replaced with a rendered two-consumer state-sharing regression test); the adaptation commits carry `Co-authored-by` trailers. |
| [#6726](https://github.com/esengine/DeepSeek-Reasonix/pull/6726) | [#6821](https://github.com/esengine/DeepSeek-Reasonix/pull/6821) | 2026-07-22 | Equal-width segmented-button selector explicitly incorporated with `Co-authored-by` trailer. |
| [#5943](https://github.com/esengine/DeepSeek-Reasonix/pull/5943) | [#6677](https://github.com/esengine/DeepSeek-Reasonix/pull/6677) | 2026-07-19 | Per-model `context_window` override feature rewritten and landed as the official implementation. |
| [#5872](https://github.com/esengine/DeepSeek-Reasonix/pull/5872) | [#6889](https://github.com/esengine/DeepSeek-Reasonix/pull/6889) | 2026-07-24 | MCP persistent disable concept incorporated into the broader "default trust" redesign. |
| [#6783](https://github.com/esengine/DeepSeek-Reasonix/pull/6783) | [#7159](https://github.com/esengine/DeepSeek-Reasonix/pull/7159) | 2026-08-02 | 5-tier pane transparency and Windows frameless dock compression fully integrated: the original commit was cherry-picked verbatim with authorship preserved (321 of 327 lines intact), and the adaptation commit carries a `Co-authored-by` trailer. Closes #5825. |
| [#6104](https://github.com/esengine/DeepSeek-Reasonix/pull/6104) | [#7159](https://github.com/esengine/DeepSeek-Reasonix/pull/7159) | 2026-08-02 | Dock-tab compression approach folded into the theme pane transparency work and landed upstream via #7159. |

## Bug Reports Acknowledged in Maintainer PRs

| Issue / Report | Maintainer PR | Date | Description |
| --- | --- | --- | --- |
| [#6590](https://github.com/esengine/DeepSeek-Reasonix/issues/6590) | [#6694](https://github.com/esengine/DeepSeek-Reasonix/pull/6694) | 2026-07-19 | Reported theme palette regression after settings refresh; fix landed with explicit credit. |

## Release Notes Credits

My contributions have been acknowledged in the following release notes:

- [v1.22.0](https://github.com/esengine/DeepSeek-Reasonix/pull/8121) — 2026-08-10 (status bar TPS refresh after each request; credits list includes HaoyueQin)
- [v1.21.4](https://github.com/esengine/DeepSeek-Reasonix/pull/8039) — 2026-08-09 (status bar throughput display)
- [v1.20.0](https://github.com/esengine/DeepSeek-Reasonix/pull/7622) — 2026-08-05 (Primer palette charts)
- [v1.19.6](https://github.com/esengine/DeepSeek-Reasonix/pull/7471) — 2026-08-04 (responsive settings layout)
- [v1.19.5](https://github.com/esengine/DeepSeek-Reasonix/pull/7364) — 2026-08-03 (usage statistics panel)
- [v1.19.0](https://github.com/esengine/DeepSeek-Reasonix/pull/7117) — 2026-08-01 (terminal drawer redesign, pasted-text & file-ref cards, right-panel & status-bar persistence, decision card overflow)
- [v1.19.0-preview.1](https://github.com/esengine/DeepSeek-Reasonix/pull/7116) — 2026-08-01 (same four entries)
- [v1.17.17](https://github.com/esengine/DeepSeek-Reasonix/pull/6743) — 2026-07-22
- [v1.17.16](https://github.com/esengine/DeepSeek-Reasonix/pull/6709) — 2026-07-20

## Contribution Themes

- **Desktop theme system**: pane opacity controls, scene-level transparency tiers (landed via #7159), save/apply state correctness, and dock-tab compression on Windows frameless.
- **Usage statistics panel**: hand-drawn SVG heatmap, trend chart, and donut chart for per-day token usage, cache hit rate, and model distribution across all product entry points.
- **UI bug fixes**: model list overlap, pasted text display, image preview, safe-area button width, decision card overflow, natural file sorting, and settings-page narrow-window responsive layout.
- **Terminal drawer**: re-architected the terminal as an independent bottom drawer with resize handle and accordion animation.
- **Configuration isolation**: `REASONIX_HOME` environment variable for isolated config, skills, and output-style scanning.
- **Feature proposals**: per-model context window overrides, MCP persistent disable, and status bar throughput display.

## Branch Purpose

This branch is designed as the landing page for my fork. The regular development
branch can stay close to upstream, while this branch gives visitors a quick, readable
view of the concrete contributions that landed in the official repository.

## Data Source

Closed upstream PRs authored by `HaoyueQin`:

https://github.com/esengine/DeepSeek-Reasonix/pulls?q=is%3Apr+is%3Aclosed+author%3AHaoyueQin

Last refreshed: 2026-08-13.
