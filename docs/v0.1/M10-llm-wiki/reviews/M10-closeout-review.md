# M10 LLM Wiki：收尾审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | `main` 的 `c49244e8`（M10 的五个 Phase 全部合并之后） |
| 方式 | 三位独立审查者并行：A 服务端（跨 Phase 的一致性与扩展点），B 网页与 e2e，C 完成标准、移交与文档。各在自己的 worktree 里，A 用 `go test -overlay` 的探针与混沌运行，B 用 Playwright 探针与变异，C 对着代码与 CI 日志核对 |
| 日期 | 2026-11-07 |
| 结论 | 没有行为上的 Important。A：Minor 2、Nit 13。B：Minor 6、Nit 9。C：Important 13，全在收尾手续与文档（`main` 红、handoff 未关、状态与代码不符、移交没写进 M11、M12），Minor 8、Nit 6。修复在分支 `m10-closeout` 上，原席核对见"核实修复" |

## 完成标准核对

证据是 C 在 `c49244e8` 上读代码与 CI 得的。路径写法同 00 号文档。

| 第 3 节 | 结论 | 测试 |
|---|---|---|
| 1 模式与模板、降级 | 满足 | `TestLLMWikiCreationWritesItsTemplate`、`TestTheTemplateRollsBackWithItsNotebook`、`TestTheDowngradeEndsTheContract`、`TestTheDowngradeAnswersInItsOrder`、`TestTheModeEventReachesTheStreams`、`TestImportsAndDowngradingAnLLMWiki`，LW1、LW12 |
| 2 写入的守卫 | 满足 | `TestTheModeGuardReachesEveryPath`、`TestTheModeGuardReachesEveryMCPWrite`、`TestSourceRESTWritesReachTheRegisteredGuard`、`TestSourceDuplicatesReachEveryLastHop`、`TestPrechecksAnswerTheGuardsCodes`，LW6 |
| 3 来源区 | 满足 | LW4、LW5、`TestTheSourceWritersSerializeInBothLockModes`、`TestTheSourceRebuildMatchesSuccessfulRandomSteps`、`TestSourceDeletesClearEveryHash`、`TestSourceDowngradeClearsHashesAtomically` |
| 4 保留的标题与 `AGENTS` | 满足 | `TestCheckContract`、`TestTheReservedTitlesAreNumberedAtAnLLMWikisRoot`、`TestTheSourceImportKeepsDuplicateParentsChildren`，LW7 |
| 5 lint | 满足 | `TestLintRules`、`TestLintMissingProperties`、`TestLintCountsAndPages`、`TestTheLintSnapshotAndDowngrade`、`TestLintRulesOfAHundredThousandPagesTakeTheirTime`、`TestLintPlans`，LW8 |
| 6 虚拟文件 | 满足 | `TestVirtualPaths`、`TestVirtualListing`、`TestVirtualRead`、`TestVirtualExport`，LW10 |
| 7 ingest | 满足 | `TestIngestUnit`、`TestIngestBefore`、`TestIngestHistory`、`TestIngestCleanup`、`TestIngestInvariants`，LW9 |
| 8 playbook 与说明 | 自动的部分满足，真实客户端留人工 | `TestThePromptListsAreTheContract`、`TestTheWikiPromptsAreReached`、`TestTheWikiInstructionsFollowTheirSourceRoot`、`TestTheBuiltInInstructionsAreShort`，LW11，清单 J |
| 9 页面版本的故事 | 满足 | LW 各故事的 `(page)` 版本，`problem-messages.test.ts` |
| 12.5 没有 open 的 handoff、`main` 绿 | 修复之后满足 | 见下 |

## 发现与处置

### A（服务端）

没有 Important。写路径没有绕过守卫的，REST、MCP、`batch` 的码与次序一致，扩展点照第 8 节登记。混沌运行 55 个种子，每个 300 步，每步之后核树、链接、`source_hash` 重建、保留的根名；没有 5xx，105 次导出全部成功。并发去重 15 轮、后台任务 12 轮各只留一个赢家。

| 发现 | 处置 |
|---|---|
| M1 撤销的恢复不判保留的标题，升级之后才走得到 | 写进 [M11 的移交](../../M11-upgrade/handoffs/M10-mode.md)。M10 里走不到，不改代码 |
| M2 `SourcePrecheck` 的 nil 分支只在测试里跑，任务的性质测试跑的是回退 | 删掉回退，fixture 用真的 `SourcePrecheck`（普通笔记本，到 `Check` 的模式判断为止）。`CheckJob` 由 `TestARevertsChecksRefuseTheContract`、`TestARevertsExtraDecisionKeepsItsErrors` 盖住 |
| Nit：注释过时或写着 Step 的历史（`notebook.go`、`contract.go`、`lint_errors.go`、`mode_guard.go`、`apply_batch.go`、`extension.go`、`refusal.go`、`wiki/module.go`、`linking/module.go`、`purge.sql`） | 都改了 |
| Nit：4.8 只写了链接改写让两个来源相同，撤销的写回同样 | 补进 4.8 |
| Nit：`unit_revert.go:98` 另造一个 `NewModeGuard`；未加锁地先读模式再读根，降级加删 `raw` 落在两次读之间时答错误 | 接受。前者不绕过规则；后者靠推理得出，15 轮没有复现，握手下次就好 |

### B（网页与 e2e）

没有 Important。`stories/wiki` 在 Chromium 上重复四遍 340/340。

| 发现 | 处置 |
|---|---|
| M1 重复来源的路径不折行，对话框的按钮跑到外面 | `TargetLink` 折行；LW3 在 375 宽下量按钮，变异抓到 |
| M2 `sr-only` 的后缀逃出 `truncate`，长来源让整页横向滚动 | 树的行与附件的行加 `relative`；同一故事量页宽，变异抓到 |
| M3 三条路建出写者动不了的空来源 | 决定 171：落在来源区的链接不再给"新建"，空的粘贴留作文件夹 |
| M4 打开导入报告阻塞主线程几秒（问题 × 节点） | 路径按一次树的读取建一张表；vitest 数走树的次数 |
| M5 历史的拒绝给写者建议一件服务端拒绝的事 | 只在 `page.source_immutable` 时给这句话；LW6 断言 |
| M6 故事的行承诺了没人断言的事 | 补 LW4 的阅读说明、LW6 的写者撤销与管理员写回、LW7 的根下导入；后台撤销的 `refused` 改写行文，由 `TestTheSourceRootsPutBackIsRefusedInALaterBatch` 盖住 |
| Nit：LW1 的 `raw` 语言、`source.newlineHint`、孤立页的说明、`lw6:302` 失败不了的断言、17 处 `EW0x`、00 里的"新建页面"、`PasteSource` 的 `AbortError`、LW3 的 PDF | 都改了 |
| Nit：`{ times: 1 }` 的路由、`sourceAtPath` 只给测试导出 | 接受。340 遍没有抖动 |
| 打磨十条 | 写进 [M5-polish](../../M12-release/handoffs/M5-polish.md) 第 38 条 |

### C（完成标准与文档）

| 发现 | 处置 |
|---|---|
| I-1 `main` 红；`first-sources` 之前也红过，决定 168 的初稿放不过它 | 补进决定 168，`first-sources` 36、24 毫秒 |
| I-2 第五份两次 main 均值 453 秒 | 决定 169，见"分份的数" |
| I-3 两份 handoff 还是 `open` | 改 `done`，证据见"handoff" |
| I-4 P5 没有关 | `docs: M10/P5 done` |
| I-5 00 与 Phase 文档的状态互相矛盾 | 改了十处，头的状态改"实现完成，待人工验收" |
| I-6 M11 的移交缺 `raw 2`、保留标题、prompt、闸门 | 写进 M11 的 `M10-mode.md` |
| I-7 M12 的移交缺 30 万页的 lint 等 | `M4-performance.md` 第 19 条，`M5-polish.md` 第 36、37 条 |
| I-8 两个守卫的 CI 记录，`IndexItems` 余量不到 5% | 决定 170，见"两个守卫" |
| I-9 13.4 第 8 条与代码不符 | 改了九处，加决定 150、168、170 |
| I-10 总体设计把做完的写成没做 | 改了十来处，去掉十处"待闭环"的尾巴 |
| I-11 223 个链接指向本机的目录 | 改成文字；`check_links.py` 报告以 `/` 开头的目标；`docs/README.md` 说明这些证据不在仓库 |
| I-12 决定 144 不在第 11 节 | 补上，第 11 节开头加编号的说明 |
| I-13 README 缺 M10 的用法 | `### LLM Wiki 与 playbook` 加来源、健康度与目录、降级的接口，事件加 `mode`，导出加 `index.md` |
| M-1 `notebook.go` 的注释 | 改了 |
| M-2 13.1 缺 M10 的几处 | 补了第 1、8、10、11、13、16 条与树写入、导入两段，13.2 第 10 条。13.2 第 14 条 M10 没有改变写法，不加 |
| M-3、M-4 过时的签名，缺 `page.source_too_large` | 改了 |
| M-5 编号有三套 | 第 11 节开头加说明，决定 62、133、134 提上来 |
| M-6 清单没写 J 的结果放哪 | 改了 |
| M-7 输入法清单没有 M10 的步 | 加第 22、23 步 |
| M-8 真实的读屏软件没有去处 | [M5-polish](../../M12-release/handoffs/M5-polish.md) 第 39 条 |
| N-1 坏的锚点、N-3 README 的 51/52 | 改了 |
| N-2 第 11 节的次序、N-4、N-5、N-6 | 接受，不影响判断 |

## 负责人可以改判的决定

都写在 [00 号文档](../00-M10-design.md)第 11 节，这里只列号：144（00054 用 LZ4，Codex 加的），150（两个 ingest 用例 30、29 秒），151（未改 stale 来源的旧 F），152（不改写 S2 的历史），153（恢复与撤销 14 秒），154（不留 EW 的脚手架），155（整套 Chromium，S3 的故事三个浏览器），156（目录后续页失败不自动重试），157（More 的焦点），158（落点读模式，**推翻了 Codex 的决定 120 的后半**），159（grep 守卫 3.25 秒），160（lint 成本用例 6、3 秒），161–165（P5 的取舍），166、169（第五、六份），167（`/readyz` 不等任务），168、170（成本的界），171（来源区里的新建）。

## handoff

`handoffs/M5-events.md`、`M6-links.md`、`M8-history.md` 已是 done，这次核过。

[`M7-transfer.md`](../handoffs/M7-transfer.md)：

| 项 | 证据 |
|---|---|
| 1 | `TestVirtualExport/whole_last_hop`，`bootstrap/wire.go` |
| 2 | `virtualExportMetaSkip`、`virtualExportFallbackConflict`，`transfer/domain/meta.go`，LW10 |
| 3 | `page/app/unit_sources.go`、`source_precheck.go`、`unit_import.go`，`TestSourceDuplicatesReachEveryLastHop`，LW3 |
| 4 | 没有代码，第 11 节主持者的决定 14 |
| 5 | `lint.sql` 的 `resolved_asset`，`TestLintRules`、`TestLintSources` |
| 6 | `wiki/app/virtual_files.go`，`TestVirtualExportOfAHundredThousandPagesTakesItsTime` |

[`M9-P1-mcp-registry.md`](../handoffs/M9-P1-mcp-registry.md)：

| 项 | 证据 |
|---|---|
| 1 | `platform/httpserver/mcpserver/prompt.go`，`TestPromptEntries` |
| 2 | `TestThePromptListsAreTheContract`、`TestTheWikiPromptsAreReached`，`api/mcp/prompts.json` |
| 3 | `TestVirtualPaths`、`TestVirtualListing`、`TestVirtualRead` |
| 4 | `TestTheModeGuardReachesEveryPath`、`TestTheModeGuardReachesEveryMCPWrite` |
| 5 | `prompt_bounds_test.go` |
| 6 | `TestTheBuiltInInstructionsAreShort`（notebook 3044/3093，account 3113/3163），LW 的 MCP 故事，`wiki/domain/playbook-*.md` |
| 7 | `CheckContract`，`TestSourceRESTWritesReachTheRegisteredGuard`，`source_mcp_precheck_test.go` |
| 8 | `TestTheWikiToolsReadLLMWikiMatrix`，`permission_matrix_mcp_test.go` |

## 两个守卫

第 7 节要收尾核对这两个，分别记。C 扫了 2026-10-26 以来 119 次 CI 里失败的 31 次。

- **`TestGrepOfAHundredThousandCandidatesTakesItsTime`**：唯一的失败是 `b112450f` 的 40378004872，在 600 毫秒的界之前。`789b4071` 之后没有再失败，最近七次中位数 105–193 毫秒，最慢样本 218 毫秒，余量约三倍。稳定。
- **`TestIndexOfAHundredThousandPagesTakesItsTime`**：30 毫秒的 `IndexItems` 红在 `ca831b58` 的 40325348498，`5a97e969` 的 41370811260 红的是另一条界（`efadfcb6` 修了）。之后 30 毫秒没有再红，但 EPYC 7763 上中位数 25.8–28.7 毫秒，一个样本 30.8 毫秒，余量不到 5%，不算稳定。决定 170 提到 60 毫秒，整值读取的反例本机 2.76–7.62 秒，抓得到；`TestIndexPlans` 不动。

## 分份的数

【分份】

## 核实修复

【核实】

## 门禁

【门禁】

## 没能验证的风险

- 真实客户端，见"人工验收"。
- 十万页以上的负载、停机窗口，在 [M12 的性能移交](../../M12-release/handoffs/M4-performance.md)第 19 条。
- `ubuntu-24.04` 的 CPU 由不得我们：六份在 EPYC 7763 上合计约 2500 秒，平均约 420 秒。再加测试就要分第七份。成本守卫的余量没有全部扫过。看过的里面最小的，都在 EPYC 7763 上：`ListIndex` 3.01–3.03 秒，界 3.5 秒（0.87）；撤销 11.7–12.05 秒，界 14 秒（0.86，决定 153）；`IndexPages` 0.82 秒，界 1.025 秒（0.80）。costs-lint 的各组至多 0.69（`rules/wanted/100k-excluded-and-outside`）。
- 单个提交 `d3e1eb75` 曾单独不绿（历史，见 P4/S3 记录）。`3b0d55f2` 是一个不完整的推送，下一个提交补上（[P5 设计审查](P5-design-review.md)）。`026c4a37` 也是：往 `main` 上接分支时 rebase 停在 00 的冲突上，连着的命令没有看返回码，把冲突标记和收尾的文档一起提交推送到了 `m10-closeout`，它的说明只写了状态；`ccdf8c8c` 补上。三处都不改写历史。

## 人工验收

这些由负责人在 v0.1 的最后执行，通过之后 M10 改为已完成：

1. [真实客户端的清单](../../M9-mcp/manual/P5-mcp-clients-checklist.md) J1–J11，Claude Code 与 Codex 各一遍。J6 判决定 161，J1 判总体设计 5.9 的斜杠命令。其余 A–I 是 M9 的，同一次跑。
2. [输入法清单](../../M4-pages/manual/P6-ime-checklist.md)，M10 是第 22、23 步。
3. Safari：375 宽的搜索页宽到 704（SE1），第二个标签页退出之后的事件流计数（C8），见 [M5-polish](../../M12-release/handoffs/M5-polish.md) 第 33 条。

结果写进这一节，playbook 照记下的改；之后改 00 号文档的头与第 12 节、总体设计 12.6 与第 15 节。

| 部分 | Claude Code | Codex |
|---|---|---|
| J1–J11 | 待执行 | 待执行 |
