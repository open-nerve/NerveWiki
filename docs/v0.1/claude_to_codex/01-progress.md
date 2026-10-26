# 进度（交接时，2026-10-26）

进度的唯一来源仍是 [v0.1-design.md](../v0.1-design.md) 12.6 的里程碑进度表与各 M 的 00 号设计的 Phase 进度表；这里是导读与"下一步"。

## 1. 里程碑

| M | 名称 | 状态 | 说明 |
|---|---|---|---|
| M0 | 基础骨架 | 已完成 | |
| M1 | 账户认证 | 已完成 | |
| M2 | 工作区 | 已完成 | |
| M3 | 笔记本与权限 | 已完成 | |
| M4 | 页面树与读写 | 进行中（待负责人执行输入法清单） | 实现与收尾都做完，只等人工验收 |
| M5 | 多人编辑 | 同上 | |
| M6 | 链接与 Obsidian 方言 | 同上 | |
| M7 | 附件与导入导出 | 同上 | |
| M8 | 历史、回收站与搜索 | 同上 | |
| M9 | MCP | 实现完成，待人工验收 | 收尾 `06a5554d`；负责人的 Codex 评审的修复 `374730bf`、文档 `91ee78d8` |
| M10 | 模式契约与 LLM Wiki | **进行中**（P1 的 S1、S2 已合并） | 见下 |
| M11 | 升级流程 | 未开始 | 只在负责人说开始之后 |
| M12 | 发布 | 未开始 | |

M1–M9 每个 M 收尾之后（M4 与 M5 合在一次），负责人都让本机的 Codex 做过一次独立评审（报告在各 M 的 `reviews/` 里，文件名带 `codex`，最后一节是处置）；以后反过来：Codex 推进，负责人与 Claude 在 M 收尾之后审核。

## 2. 还欠负责人的人工验收（v0.1 的最后统一做，不阻塞任何 M）

- **输入法清单** [M4-pages/manual/P6-ime-checklist.md](../M4-pages/manual/P6-ime-checklist.md)：三种浏览器的真实输入法，覆盖 M4–M8 加的步骤；结果分别写进 M4/P6、M5、M6 等收尾记录的"人工验收"一节（各记录里写着写在哪里）。通过之后 M4–M8 改为已完成，`M4-pages/handoffs/M0-P1-editor.md` 改为 `done`。
- **MCP 客户端清单** [M9-mcp/manual/P5-mcp-clients-checklist.md](../M9-mcp/manual/P5-mcp-clients-checklist.md)：真实的 Claude Code 与 Codex 各一遍；M10 的 P5 要给它加"prompts 与 LLM Wiki"一节（[00-M10](../M10-llm-wiki/00-M10-design.md) 4.19）。通过之后 M9（与 M10 的那一节）改为已完成，`M9-mcp/handoffs/M0-P1-mcp-sessions.md` 改为 `done`。
- 2026-10-07 负责人试过一次输入法清单，服务端的记录表明步骤并没有照写的做（没有一次改名请求到达服务端），所以那次不算；重做时每一步都要从服务端核对（日志、库）。

## 3. M10 的现状

**设计**（都在 `docs/v0.1/M10-llm-wiki/`）：
- `00-M10-design.md`：总设计。初稿 `ebed0e07`，三位审查者审、四轮复核之后定稿 `243bbe1c`（记录 `reviews/M10-design-review.md`）；随 P1 设计改过 81 处（`f92ecac7`）。第 11 节：负责人 2026-10-24 的四个决定（来源只写一次之后只读；`type`/`summary` 只由 lint 检查；模板随界面语言、playbook 英文；健康度面板人人可看、打开时算），主持者的决定，设计的取舍。
- 五份移交（`handoffs/`）开工时写下了"处理进展"（`d7bb5e8d`），都还是 `open`，做完的 Phase 改为 `done`。
- `01-P1-contract-frame.md`：P1"契约的框架"的设计，审查记录 `reviews/P1-design-review.md`（主持者的决定 1–22），定稿 `f92ecac7`。

**P1 的四个 Step**：

| Step | 内容 | 状态 | 合并 | 审查记录 |
|---|---|---|---|---|
| S1 模式与模板（服务端） | 00048（`notebooks.mode`、`source_root_id`）、建 LLM Wiki 的事务与模板（`TreeWrites().Seed`）、两个新扩展点（创建的播种者、模式变化的订阅者）、`shared.ClientOf`、wiki 模块的骨架、`downgradeNotebook`、事件 `mode`、MCP 答 `mode`、`checkPages` 的新规则、矩阵的 `wlab` 变体、e2e LW1 | 已合并 | `45d086a1`（文档 `f81ed860`、`78aab3cf`） | `reviews/P1-S1-mode-template-review.md`（决定 23–26） |
| S2 守卫的骨架 | `Write.Notebook`、`Write.Grants`、`page.manage_source`、`page.NewModeGuard`（排在编辑锁之前）、`CheckContract` 的 R1–R4、`Refused`、`batch` 的 `failed` 与计划的契约检查（`BatchPlan.Refuse`）、码与文字、e2e LW2 | 已合并 | `bad859c0`（文档 `94440e1a`、`f63c84b9`） | `reviews/P1-S2-guard-review.md`（决定 27–31） |
| S3 预检、加序号与后台的拒绝的去处 | 见下"下一步" | 未开始 | — | — |
| S4 网页（一） | 建笔记本的模式、设置里的降级、`mode` 事件的处理、树里 `AGENTS` 与来源根的菜单与名字 | 未开始 | — | — |

**P2–P5 与收尾**（00-M10 第 7 节的表）：P2 来源区（S1 来源的规则、S2 去重、S3 网页）；P3 lint 与健康度（S1 服务端、S2 网页）；P4 虚拟的 `index`/`log` 与 ingest（S1–S3）；P5 playbook、说明与收尾的故事（S1）；收尾（三位审查者：服务端、网页与 e2e、完成标准与文档）。每个 Phase 开工时先写它的 Phase 设计（`02-P2-…md` 等），见 [02-workflow.md](02-workflow.md)。

## 4. 下一步：M10/P1 的 S3

照 [01-P1](../M10-llm-wiki/01-P1-contract-frame.md) 第 0 节的 S3、3.25–3.28、3.29 的 S3 的行、4.3、6.3 的提交次序、7.3 的第 42–53 条变异与第 8 节的 S3 的条目。另外要带上 S2 留下的（S2 的审查记录的主持者的决定 29，01-P1 第 9 节的"S3"一条）：

- S3 的合并关上 S2 与 S3 之间的四个窗口（01-P1 第 0 节）：导入与恢复给根下的保留标题加序号、`checkPages` 的"根下没有保留的键"（3.27，也关上 S2 审查的 A-M2）；撤销的两遍预检（3.25）关上 A-M3。
- `TestPrechecksAnswerTheGuardsCodes` 加一行钉住 A-M3 的形状：同步的撤销里前一步碰到编辑锁、后一步被模式拒绝，答模式的码（只有预检做得到）。
- 后台撤销遇到同样的形状时，由开始时的预检先拒绝整个任务。

分支 `m10-p1-s3`，从 `main` 开。S3 之后是 S4（6.4），S4 合并之后写 `docs: M10/P1 done (M10/P1)`：00-M10 的头、第 12 节的 P1 一行"完成"、第 13 节一行、01-P1 第 10 节的总结、`handoffs/M5-events.md` 改为 `done`、新写 `docs/v0.1/M11-upgrade/handoffs/M10-mode.md`（01-P1 第 8 节写了它要收的几项）。

## 5. 要盯着的

- **组合根第一份的时间**：S2 合并那次 CI 的测试一步是 7:17（线是 7:30）。第一份没有 M10 的测试，CI 上它在 5–7.5 分钟之间波动。每次合并之后照 [Makefile](../../../Makefile) 第 195–230 行的注释与 M9/P2 6.5 的规则读两次运行的平均：一份过了线，只在邻居收下之后两份都在线之下时挪界线，否则按测试名再分一份，**从不提高 `-timeout`**。
- **M9 的 grep 计时守卫** `TestGrepOfAHundredThousandCandidatesTakesItsTime`：S2 的第一次 CI 在 guards 一份里量到中位数 388 ms（界线 270 ms），重跑通过。再红时照 M9 修它的同类界线的做法改（量 CI 上的中位数，或取五次的中位数），这已记作 M10 收尾的一项（00-M10"写给后面的 M"的最后一条）。
- **第三份的估计**：S1 的审查发现 7.5 没有算 `prepareMatrix` 在每个用者里种 `wlab` 的代价，P1 结束时第三份约 6:45–7:00（S1 记录的决定 26）。

## 6. 未结的 handoff（`status: open`）

- M10：`M5-events`、`M6-links`、`M7-transfer`、`M8-history`、`M9-P1-mcp-registry`（各自的"处理进展"写着在哪个 Step 做）。
- M11：`M5-events`、`M8-P4-background`、`M9-mcp-upgrade`（P1 完成时加 `M10-mode`）。
- M12：`M0-P6-image-release`、`M4-performance`、`M5-performance`、`M5-polish`（v0.1 之后的打磨清单）、`M7-transfer`、`M8-history`。
- 等人工验收的：`M4-pages/handoffs/M0-P1-editor`、`M9-mcp/handoffs/M0-P1-mcp-sessions`。

## 7. 远端的分支

`m10-p1-s1`、`m10-p1-s2` 已合并进 `main`，留着无妨。之前各 M 的分支同样都已合并。交接时没有在途的分支。
