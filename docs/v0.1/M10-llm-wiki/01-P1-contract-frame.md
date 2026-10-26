# M10/P1 契约的框架：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M10/P1 契约的框架（四个 Step：S1 模式与模板（服务端）；S2 守卫的骨架；S3 预检、加序号与后台的拒绝的去处；S4 网页（一）） |
| 日期 | 2026-10-24（照 S1、S2 修订 2026-10-25；S3 修订 2026-10-26；S4 修订 2026-10-27） |
| 状态 | 已完成（2026-10-27；S1–S4 均已合并，审查、修复、变异与门禁，以及合并后的 CI 样本已核对；合并见 00 号文档第 12 节，完成总结见 10.5）；设计已审查，[审查记录](reviews/P1-design-review.md)；Step 审查见 [S1](reviews/P1-S1-mode-template-review.md)、[S2](reviews/P1-S2-guard-review.md)、[S3](reviews/P1-S3-prechecks-refusals-numbering-review.md)、[S4](reviews/P1-S4-web-review.md)；实施对设计的修正见 3.30–3.33，结果见 10.1–10.5 |
| 上级文档 | [M10 总设计](00-M10-design.md) 4.1–4.9、4.15–4.18、第 5、7、8、9、10、11 节与"总体设计的修订"、"移交的落实"；[M10 设计审查](reviews/M10-design-review.md)；[移交](handoffs/)：M5-events、M8-history、M9-P1-mcp-registry；[v0.1 总体设计](../v0.1-design.md) 5.1–5.9、12.4、13.1–13.4；[M9/P3](../M9-mcp/03-P3-batch-revert.md)（写法） |
| 代码位置的写法 | 同 00 号文档：模块写作 `page/app/unit.go:185`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…`、`archtest/…` 在 `server/internal/` 之下；迁移写作 `00009_notebook_notebooks.sql:12`（`server/migrations/sql/`），迁移的测试写作 `server/migrations/schema_test.go:163`；前端写作 `W/stores/notebook.store.ts:45` 或 `web/…/notebook.store.ts:45`（`W/` 是 `web/apps/web/src/`）；e2e 写作 `e2e/fixtures/notebooks.ts:9`；契约写作 `api/modules/notebook.yaml:421`。代码的行号是 `d7bb5e8d` 的（`main`，M10 总设计的文档提交；代码与 M9 的 Codex 评审之后的文档提交 `91ee78d8` 相同），00 号文档与 v0.1 总体设计的行号也是它的 |
| 工作笔记 | `scratchpad/m10/p1/notes.md`；代码的核对分五份（网页、e2e、契约与 CI、守卫与矩阵、transfer 与后台）在 `scratchpad/m10/p1/h/`，本文引的每个行号都在其中或附录里核对过。P1 不量要数据库的东西（第 7.6 节写实施时怎样量） |

---

## 0. 分四个 Step

照 00 号文档第 7 节，P1 分四个 Step，各自一个分支、一次审查与修复核对、一次合并；后一个从前一个合并之后的 `main` 开始。每个 Step 里的每个提交都是绿的（服务端的测试、网页的五步、e2e），每个 Step 带上它改动的契约在网页与 e2e 上要的改动（类型、假的服务端的字段、码的文字、钉住旧行为的测试），不留到 S4（第 4 节）。

- **S1 模式与模板（服务端）**（分支 `m10-p1-s1`）：
  - 迁移 00048（`notebooks.mode`、`source_root_id` 与两个检查），`schema_test` 的名称与反例，`TestTheNotebooksModeGoesDown`，notebook 的 sqlc 一项（3.2）；
  - notebook 的 `Mode`、`Locale`、`CheckDraft` 的两个新值、读笔记本的五条查询与 `CreateNotebook` 带两列，根的 `State` 与 `StateOf`（3.3）；
  - 建的事务的次序：一次读时钟、先定下来源根的 id、插入笔记本与管理员、播种者、核对恰有一个来源根、可见性（3.4）；两个新的扩展点（播种者、模式变化的订阅者）、模块根的替身与次序的测试、`notebook.New` 在没有播种者时 panic（3.5）；
  - page 的 `Writer.Seed` 与 `TreeWrites().Seed`、`ParseTemplate`（3.6）；定客户端的函数 `shared.ClientOf`，四个适配器都用它（3.7）；
  - wiki 模块的骨架（契约表、两种语言的模板、播种者、`Actions()`）、组合根的次序（3.8）；模板的全文与它们的测试（3.9）；
  - `downgradeNotebook`、`notebook.change_mode`、`transfer.NewWritingJobs`（3.10）；事件 `mode` 与它的发布者、契约的 `EventMode`（3.11）；page 的 `StateOf` 与 `open_notebook`、`notebooks` 的 `mode`（3.12）；契约与 `tools.json`（3.13）；
  - 日志、`checkPages` 的新规则与它的反例的办法、权限矩阵的 LLM Wiki 的变体（3.14）；建的交错（3.15）；整个程序的测试（3.16）；
  - 网页：生成的类型、两处字面量、`notebook.not_llm_wiki` 的文字（4.1）；e2e：夹具、LW1 (API)、LW1 (MCP)、MC1 的期望（第 5 节）。
- **S2 守卫的骨架**（分支 `m10-p1-s2`）：
  - `Write.Notebook`：page 的端口的 `ShareByID`、`LockByID` 交回状态（3.18）；`page.manage_source` 与单元的 `Write.Grants`（3.19）；
  - `page.NewModeGuard(authorizer)` 登记在锁之前、`pageRegistrants(pool, authorizer)` 与 `pageSessionSubscribers()`（3.20；不收连接池，S2 的偏离 5）；
  - `CheckContract` 与 P1 的四条规则（`AGENTS` 的保护、来源根的保护与它的管理员规则、根下的四个保留标题）、步 × 节点的表（3.21）；
  - 拒绝的标出（`Refused`、`page.Refusal`）与 `batch` 的 `failed`；`batch` 的计划的树的一遍调 `CheckContract`（本在 S3，主持者挪进 S2）；P1 的码进各操作的 `x-problem-codes` 与 `mcpToolCodes()`、它们的文字；矩阵的变体的行（3.22）；
  - P1 里没有可拒的路径；降级与 `AGENTS` 的删除的交错（3.23）；整个程序的测试，含 `batch` 的只有守卫拒绝的情形（3.24）；
  - 网页：三个码的文字（4.2）；e2e：LW2 (API)、LW2 (MCP)。
- **S3 预检、加序号与后台的拒绝的去处**（分支 `m10-p1-s3`）：
  - 三处预检调 `CheckContract` 并标出拒绝：上传的 `Check`、撤销的两遍（`revertChecks.check` 一处给同步的撤销、`startRevert` 与任务的计划；`Unit.Revert` 的锁下的一遍）；后台撤销的每一批不另检查，由那一批的守卫判（主持者的决定）；`startRevert` 的码（3.25）；
  - 后台的拒绝的去处：`revertDefect` 认出它，`treeError` 先认已有的哨兵、再认 `page.Refusal`，transfer 的 `Refused`、`unitFailure` 的 `refused`、报告的 `refusal`（`code` 是自由的字符串）、契约（3.26）；
  - 导入、同步与后台的恢复给四个保留标题加序号，`checkPages` 的"根下没有保留的键"（3.27）；测试（3.28）；
  - 网页：`transfer-report.tsx` 的 `refused` 与报告里的拒绝（4.3）；e2e：TR1、TR2、TR3、TR4 的期望。
  - 开始时的预检、任务的计划与去处在同一个 Step：P1 的规则已经能让后台的撤销被拒，分开就有一段把它报成 `internal`（00-M10 审查 C2-M4）。
- **S4 网页（一）**（分支 `m10-p1-s4`）：树里 `AGENTS` 与来源根的菜单、拖放与带文字等价的图标（排在最前：LW1 的页面版本按它的名字找）、建笔记本的对话框的模式、设置的"模式"一节与降级的普通的确认、`NotebookStore.downgrade`、`mode` 事件的处理（重读笔记本的列表）（4.4）；e2e：LW1、LW2 的页面版本，LW12 的 P1 的几步（第 5 节）。

**S1 为什么不再分**：它最重，但它的几块互相要：一本没有模板的 `llm_wiki` 笔记本违反 `checkPages` 的新规则与建的原子性（00-M10 4.4），先合并模式、后合并模板，中间的 `main` 上就能建出坏的笔记本；降级要一本建得出来的 LLM Wiki 才测得了；`mode` 事件的唯一发布路径是降级。所以它是一个 Step，提交按 6.1 的次序排，每个提交都绿。S2 碰 page、notebook 的读（`ShareNotebook` 的两列）、access 的一行、组合根的登记、asset 的码与几个操作的契约、网页的文字与 e2e（审查 B-N8、C-N1）；S3 碰 page、asset（上传的拒绝的测试）、transfer、组合根的去处、契约、网页的报告与 e2e 的四个故事；S4 只碰网页与 e2e：照 00 号文档的四个 Step，不再加。

**Step 之间的窗口**（主持者对 B-Q2 = C-Q2 的决定）：四个 Step 一个接一个合并，中间不合并别的分支、不发布（6.5），所以下面的窗口只在两次合并之间的 `main` 上，各由下一个 Step 的合并关上：

- **S1→S2**：建得出 LLM Wiki，还没有守卫：写者可以删除、改名、移动 `AGENTS`，删除 `raw`；这样的数据让 `checkPages` 的 3.14 的规则失败，S1 的测试不这样做（审查 A-N13）。
- **S2→S3**：导入与恢复照今天把 `index` 放到根下（3.21，S3 加序号与 `checkPages` 的规则）；`startRevert` 开始一个会被拒的后台撤销，它以 `internal` 停下（3.23，S3 的去处）。S2 的审查另找到两个（审查 A-M2、A-M3，[S2 的审查记录](reviews/P1-S2-guard-review.md)的主持者的决定 29）：
  - 恢复或导入放到根下的 `index` 改名之后，撤销这次改名在根下放回 `index`，答 `revertChangeset` 没有声明的 409 `page.virtual_file`（3.21 的表的"撤销的放回"的 R3 一格在这一段到得了）；S3 加序号之后根下没有保留的键，它又到不了，所以不为它声明这个码。
  - 同步的撤销在 S2 没有契约的预检，守卫 `[mode, lock]` 逐步判：前一步碰到编辑锁时答 `page.locked`，不答后一步的 `page.source_admin_only`（`revertChangeset` 的说明与 13.1 第 4 条写的"模式在锁之前"对这个形状不成立）。S3 的撤销的两遍预检关上它，3.28 的 `TestPrechecksAnswerTheGuardsCodes` 钉住这个形状（第 9 节）。

`batch` 的计划的预检在 S2（3.22），所以 S2 与 S3 之间 MCP 的 `batch` 不先答 `page.locked`、后答 `page.protected`：它的码的次序与 REST、MCP 的 `delete` 一样（审查 B-M8a，主持者为此把它挪进 S2）。

## 1. 基线

从 `main` 的 `d7bb5e8d` 开始（M10 总设计的文档提交；代码同 `91ee78d8`）。

- **建笔记本**（M3）：`CreateNotebook.Execute(ctx, slug, name, access)`（`notebook/app/create_notebook.go:42-89`）：事务之外不加锁 `FindBySlug`（`:47`）；`WithinTx`（`:55`）里工作区行 `FOR SHARE`（`:56`）→ 判定 `notebook.create`（`:59`）→ `CheckDraft(name, access)`（`:63`）→ 读一次时钟（`:67`）→ `CreateNotebook`（`:70`）→ `AddMember`（`:73`）→ 可见性的订阅者（`:76-79`）；日志 "notebook created" 只带 id（`:86-87`）。笔记本的创建没有扩展点（`notebook/module.go:52-59`）；`New` 在没有 `AccountWorkspaces` 时 panic（`:88-91`）。笔记本的管理写的锁序是工作区行 `FOR SHARE` → `LockNotebook` `FOR NO KEY UPDATE` → 判定（`notebook/app/manage.go:26-41`）。
- **笔记本的表与读**：00009 建 `notebooks`（没有模式）；读笔记本的查询都写明列（`notebooks.sql:7-20`、`:42-55`，`cascade.sql:37-44`，`ownerless.sql:1-9`），`ShareNotebook` 只选 `id`（`:22-28`）。根的 `notebook.NewNotebooks(pool)`（`notebook/notebooks.go:20-85`：`WorkspaceOf`、`ShareByID`、`LockByID`、`NameOf`）同时给 page（`bootstrap/deps.go:153`）、asset（`:190`）、linking（`:210`、`bootstrap/reindex.go:176`）与 transfer（`:234`）。
- **写入单元**（M4–M9）：`Writer.Run`（`page/app/unit.go:185-251`）拒绝已在事务里的 ctx（`:186-188`），客户端与种类的核对（`:189-196`），执行者取 ctx 的（`:197`），事务之外读工作区（`:201`），`WithinTx` 里工作区行 `FOR SHARE`（`:214`）、笔记本行 `ShareByID` 或（`Tree`）`LockByID`（`:217-223`）、判定（`:224-227`，`Grant` 丢掉）、读一次时钟建 `Unit`（`:228-232`）。每个操作经 `apply`（`:299-330`）：守卫的循环（`:303-307`，第一个拒绝原样交回）→ 变更集 → 写 → 参与者。`Write`（`page/app/extension.go:27-36`）没有模式与授权的备忘；`Step` 按值嵌着它（`:41-55`）。`WithinTx` 在 ctx 已带事务时加入它（`platform/postgres/tx.go:121-129`）；`postgres.DB(ctx, pool)` 交回 ctx 的事务（`:36-44`）。守卫今天只有编辑锁（`bootstrap/registrants.go:253-264` 的 `pageRegistrants(pool)`，命令行经 `notebookRegistrants(pool)` 的 `:188` 也走到它），`TreeWrites`（`page/tree_writes.go:72-121`）没有写模板的入口。
- **客户端**：由凭据定客户端的函数有三份：`page/adapter/http/handler.go:351-362`、`asset/adapter/http/upload.go:161-171`、`transfer/adapter/http/server.go:103-113`，都读 `shared.RequireActor` 的 `Actor`（`APITokenID` 设着是 `api`，没有 `Actor` 是 401），notebook 的适配器没有；`shared` 里已有变更集的客户端的名称的清理（`shared/client_name.go:26`）；平台不许导入 `shared`（`archtest/rules_test.go:40`、`:238-244`）。
- **后台的拒绝**（M8）：`unitFailure`（`transfer/app/batches.go:83-96`）把 403、404 报成 `forbidden`，别的落到 `internal`；`revertDefect`（`page/app/unit_revert.go:913-919`）把后台撤销的一步里除 `page.locked`、`page.trash_changed`、503 之外的有类的错误都变成 500；`treeError`（`bootstrap/transfer.go:218-247`）认九个哨兵，`ErrLocked` 在最后。写入任务的探查是 `jobs.sql:28-35` 的 `Writing`（`transfer/app/start_import.go:132-140`）。
- **事件**：events 模块没有类型的登记（`events/domain/event.go:22-36`），流在打开时定下它转发的笔记本（`events/app/open_stream.go:30-67`），帧把 `workspace_id`、`notebook_id` 写进数据（`events/domain/frame.go:38-57`）；可见性与删除的发布者是 `notebookEvents`（`bootstrap/events_registrants.go:42-55`）。
- **MCP**：`notebooks` 的每项是 `{id, name, workspace, role}`（`notebook/adapter/mcp/notebooks.go:46-51`），`open_notebook` 的答复是 `{notebook, instructions, instructions_from, overview, truncated}`（`page/adapter/mcp/open_notebook.go:33-39`），`tools.json` 的两个对象逐个比较整个对象（`bootstrap/mcp_tools_test.go:109-110`）。
- **网页**：`NotebookStore` 的写经 `inTurn`（`W/stores/notebook.store.ts:45`）；建笔记本的对话框只有名称与开放程度（`W/pages/workspace/create-notebook-dialog.tsx:90-131`），请求体被 `notebook-nav.test.tsx:182` 钉住；设置的"常规"一页没有可聚焦的标题（`W/pages/notebook/general-page.tsx:31`）；事件的处理表是 `pages`、`lock`、`links`（`W/events/handlers.ts:276-282`），`mode` 只是一个替身测试的类型（`W/app/event-stream.test.tsx:204-218`）；页面树每行对写者都可拖、菜单相同（`W/pages/notebook/page-tree-item.tsx:163-198`、`:256-261`）。
- **e2e**：`createNotebook` 的第五个参数是开放程度，57 处以字面量传（`e2e/fixtures/notebooks.ts:9-26`）；`expectNewNotebook`、`expectNotebook` 不读模式（`e2e/fixtures/assert/notebook.ts:15-60`）；套件里没有 `test.use`；worker 的服务照默认 `jobs.import_workers: 1`（`server/configs/config.yaml:121`）。
- **CI**（M9 的 Codex 评审之后，run 40011635327，`docs/v0.1/M9-mcp/reviews/M9-codex-review.md:290`）：组合根三份的作业 5:28、6:33、6:21，e2e 的作业 4:37；M9 收尾的 run 39950787055 是第一份 7:26（测试一步 7:07）、第二份 6:32（6:12）、第三份 5:49（5:25）、e2e 3:50（E2E 一步 2:56）（`M9-closeout-review.md:256`）。7:30 的线是测试一步的（`Makefile:204-205`），作业的时间不与它比（审查 C-N6）。第一份离线最近（`Makefile:204-219`，M9 总设计第 110 条）：新的整个程序的测试不落在它里面（7.5）。

## 2. 目标与范围

做的事对应 00 号文档第 7 节 P1 一行（`:531`）：模式与来源根的数据、LLM Wiki 的建与降级、模板、模式的事件；写入单元读模式、守卫与 P1 的规则（`AGENTS`、来源根、四个保留标题）；预检与后台的拒绝的去处；导入与恢复的加序号；网页的模式、降级、树的菜单与图标。

不做（P2–P5）：来源区的规则（正文只写一次、来源区里的树操作要管理员、新增的各种）与它的会话否决者、祖先的一线交给守卫、`Step.Content` 与 `Step.Rewrite`（P2/S1）；去重、`source_hash`、降级的 `Forget`（P2/S2）；来源区的网页（P2/S3）；lint、健康度、`lint.read`（P3）；虚拟的 `index`、`log`、ingest（P4）；prompt、按模式拼的说明、README（P5）。

**移交的落实**（00-M10 第 7 节"移交的落实"里 P1 的部分）：

| 移交的项 | P1 的落实 |
|---|---|
| [M5-events](handoffs/M5-events.md)：类型 `mode`、发布者、`EventMode`；前端的处理 | S1：3.11（`notebookEvents.ModeChanged`、`events.yaml`）；S4：4.4（`modeChanged` 与 `EventMode` 的导出，替身测试换成 `later`）；最后一跳 `TestTheModeEventReachesTheStreams`（服务端）与 `event-stream.test.tsx` 经应用的表（前端） |
| [M8-history](handoffs/M8-history.md) 第 1–3 项：守卫看到 `Kind`、撤销的每一步经守卫；守卫拒绝时同步的整组拒绝、后台在那一批停下；码进 `x-problem-codes` | S2：守卫与保护的页（3.20–3.22），撤销的放回照普通的改名、移动判（3.21 的表）；S3：预检与后台的去处（3.25、3.26）。来源的规则（第 3 项的四个问题）是 P2/S1 的 |
| [M9-P1-mcp-registry](handoffs/M9-P1-mcp-registry.md) 第 4 项：`AGENTS` 在 `llm_wiki` 下的保护 | S2：R1（3.21），整个程序经 REST 与 MCP（3.24） |
| 同上第 7 项：守卫要认得的四种步 | S2：一步里同时的移动与改名（R1、R3、R4 同时看父节点与名称）、撤销里的临时名称（照普通的改名、移动判，不凭名称认，00-M10 4.7）；`batch` 的合成的正文与先恢复再写回是正文的规则，P2/S1 |

### 2.1 对 00 号文档的修订

照代码与上面的设计，00 号文档有下面几处要改（第 8 节在设计被接受的那个文档提交里一并改 00 号文档；行号是 `d7bb5e8d` 的）：

1. **定客户端的函数**（4.4 的"客户端"一段，`:124`；第 7 节 P1 的 S1，`:531`"定客户端的函数挪到 `platform/httpserver`"）：今天有三份，不只 page 的（`page/adapter/http/handler.go:351-362`、`asset/adapter/http/upload.go:161-171`、`transfer/adapter/http/server.go:103-113`），都读 `Actor`；平台不导入 `shared`（`archtest/rules_test.go:40`、`:238-244`），挪进平台就得改读认证的限流的键、把"PAT 是 `api`"放进平台。所以照主持者对审查 A-Q1 的决定，它是 `shared` 里、`CleanClientName` 旁边的 `shared.ClientOf(ctx) (string, error)`：读 `Actor`（输入与今天的三份相同），没有 `Actor` 照旧 401；page、asset、transfer、notebook 四个适配器都调它、各自转换为自己的类型，三份都删（3.7）。4.4 的"今天只有 page 的 HTTP 适配器由凭据定出客户端（`page/adapter/http/handler.go:351-353`），notebook 的没有；……这个函数挪到 `platform/httpserver`，page 与 notebook 的适配器都调它"改为"今天三个 HTTP 适配器各有一份（page、asset、transfer：`page/adapter/http/handler.go:351-362`、`asset/adapter/http/upload.go:161-171`、`transfer/adapter/http/server.go:103-113`），notebook 的没有；为了不出现 13.1 第 2 条的第二份实现（13.4 第 13 条），这个函数挪到 `shared`（`ClientOf`，在 `CleanClientName` 旁边，读请求的 `Actor`），四个适配器都调它、各自转换为自己的类型，三份都删"；第 7 节的那一句改为"定客户端的函数挪到 `shared`（`ClientOf`，page、asset、transfer 的三份都删）"。同样改的还有第 7 节"总体设计的修订"的 13.1 第 2 条（`:616`"定客户端的函数在 `platform/httpserver`，page 与 notebook 的适配器都用它"改为"定客户端的函数在 `shared`（`ClientOf`），page、asset、transfer、notebook 的适配器都用它"）与第 8 节的 notebook 一行（`:640`"经 `platform/httpserver` 的函数定客户端"改为"经 `shared.ClientOf` 定客户端"）、平台一行（`:645`"、定客户端的函数"去掉：它在 `shared`）。13.1 第 2 条的新文字见第 8 节。
2. **`SeedSpec`**（4.4，`:122`；第 8 节，`:628`）：`Seed(ctx, SeedSpec{At, Client, Pages})` 改为 `SeedSpec{NotebookID, At, Client, Message, Pages}`：单元要知道锁哪本笔记本、判定在哪里（`UnitSpec.NotebookID`，`page/app/unit.go:66-118`），变更集的说明是所选语言的"LLM Wiki 模板"（4.4"变更集"一段）。`TreeWrites` 另加 `ParseTemplate(content) (Facts, error)`：模板在启动时解析一次的入口（3.6），4.4 的"模板的正文在启动时解析一次"写出它，第 7 节 P1 的 S1（`:531`）的"`TreeWrites().Seed(SeedSpec)`（声明的例外）"之后加"与 `ParseTemplate`"。
3. **两个扩展点的值的名字**（第 8 节，`:622-649`）：值叫 `NotebookCreation`、`ModeChange`，方法叫 `NotebookCreated`、`ModeChanged`，照 notebook 已有的两对（`NotebookDeletion`/`NotebookDeleted`、`VisibilityChange`/`VisibilityChanged`，`notebook/app/extension.go:17-33`、`:80-97`）（3.5）。第 8 节的 notebook 一行（`:640`）的"`notebook.New` 的两个新的 `Deps`"改为"`notebook.New` 的三个新的 `Deps`（另有 `WritingJobs`：`transfer.NewWritingJobs`，没有它时 `New` panic）"（3.5 的 `TestTheModuleRefusesNoWritingJobs`，审查 AC2-N1）。
4. **`Write.Notebook` 的来路**（4.6、第 7 节的 S2，`:531`）："`ShareByID`、`LockByID` 交回状态"指 page 的端口：方法名不变、交回 `domain.NotebookState`；notebook 的根另加 `StateOf`（S1）、`ShareState`、`LockState`（S2），它的 `ShareByID`、`LockByID` 不改，因为 transfer 与 linking 的端口用它们（`transfer/app/ports.go:30-40`、`linking/app/reindex.go:36-43`）；page 经组合根的 `pageNotebooks` 逐字段转换（3.3、3.12、3.18）。`StateOf` 在 S1 就要：`open_notebook` 答 `mode`，而它的用例与门今天都不读笔记本的行（`page/app/open_notebook.go:30-38`、`gate.go:17-36`）。4.6 的表的 `Write.Notebook` 一行（`:156`）照此写出两边；第 7 节 P1 的 S1 加"`open_notebook` 经 notebook 的根的 `StateOf` 读它"，S2 的"`ShareByID`、`LockByID` 交回状态"写明是 page 的端口、notebook 的根加 `ShareState`、`LockState`；第 8 节的 notebook 一行（`:640`）的"`notebook.NewNotebooks` 的 `ShareByID`、`LockByID` 交回状态"改为"加 `StateOf`、`ShareState`、`LockState`（`ShareByID`、`LockByID` 不改）"。
5. **祖先的一线与会话的否决者在 P2/S1**（4.6"守卫要的祖先"，第 7 节的 S2"`page.NewModeGuard` 与它的否决者登记在锁之前"）：P1 的规则不读祖先（`AGENTS` 在根下；来源根按 id 认，删除与移动的一步本就列出整棵子树，`unit_delete.go:43-48`、`unit_move.go:73-88`；保留的标题只看根下），也没有会话的规则（只有来源上不开会话，4.7）；一个永远放行的否决者的次序没有测试看得出。所以两者随它们的第一条规则在 P2/S1（00 号文档的 P2/S1 一行本就写着"来源的会话否决者"）；P1/S2 一行改为"`page.NewModeGuard` 登记在锁之前"（3.20、3.21）；4.6 的"守卫要的祖先"（`:163`）与"位置与次序"的 `sessionVetoers`（`:165`）各写明是 P2/S1 的。
6. **守卫的构造与登记**（4.1 的表 `:73`、`:84`，4.6 的"位置与次序" `:165`，第 7 节 P1 的 S2 `:531`，第 8 节 `:632`；13.1 第 21 条，12.4）：P1 的守卫是 `page.NewModeGuard(authorizer)`，P2/S1 读来源区时加连接池（`NewModeGuard(pool, authorizer)` 是那时的签名），P1 不收一个不用的参数。授权者只有 serve 有：`pageRegistrants(pool)` 改为 `pageRegistrants(pool, authorizer)`，命令行经 `notebookRegistrants` 要的会话的订阅者抽成 `pageSessionSubscribers(pool)`（命令行的路径上不许建 `access.New`，`archtest/composition_test.go:100-117`；`bootstrap/registrants.go:188`）（3.20）。S2 照实不收连接池：它唯一的订阅者是事件流的发布者，只要它的 `NOTIFY`（`events.NewPublisher()`），所以是 `pageSessionSubscribers()`（S2 的偏离 5，3.31）；00 号文档 4.1 与第 7 节的 P1 一行随之写作不带池。
7. **两处预检的位置**（4.6 的预检的表；第 5 节的码的表）：上传的检查排在 `storage_full` **之前**，不是之后：`storage_full` 是 asset 模块在 page 的整个 `Check` 之后查的（`asset/app/upload.go:67-76`），两者都不写；撤销的检查在 `revertChecks.check` 里、`page.locked` 之前，所以也在 503 **之前**：同一个函数给同步的撤销、`startRevert` 的 `CheckRevert` 与任务的 `PlanRevert`（`revert_changeset.go:94`、`:104-108`，`jobs.go:169`、`:315`），被拒的撤销不占解析的预算（3.25）。`uploadAsset`、`revertChangeset`、`startRevert` 的码的次序照此写（3.22、3.25），第 5 节的表里这三行各加一句次序。撤销的次序由 3.28 的 `TestTheServersLimitsComeAfterThePrechecks` 在整个程序里核对：预算占着时被拒的撤销立刻答它自己的码、放过的答 503（审查 B-I3）；上传的次序是结构的（asset 在 page 的整个 `Check` 之后才查空间），由 `asset/app/upload_test.go` 的 `TestCheckDecidesTheNodeThenTheFreeSpace` 的一种核对（3.28，审查 B-M4、B3-N2；第一轮复核取 C 的建议，去掉了整个程序里的磁盘的一半）。
8. **后台与预检的几处说法**（4.6、第 7 节的 S3）："`treeError` 先认它已有的哨兵（`ErrLocked` 在最前）"：`ErrLocked` 是已有的九个里的最后一个（`bootstrap/transfer.go:243-244`），意思是已有的哨兵都在认拒绝之前（3.26），第 7 节 S3 的"`treeError` 先认 `ErrLocked`、再认 `page.Refusal`"同样写明；"后台每批的 `CheckRevertPart`"去掉（第 19 条；草稿把这一批的检查接在 `CheckRevertSteps` 之后，因为 `CheckRevertPart` 没有操作与目标，`domain/revert_part.go:22-30`）；"报告 `renamed`"在后台的恢复里是 `counts.renamed`，没有 `renamed` 问题（`transfer/app/background.go:215-218`，3.27），第 3 节第 4 条（`:59`）的同一句同样写明；`:191` 的"换成 transfer 的 `ErrRefused{Code, NodeID}`"改为"换成 transfer 的 `Refused{Code, NodeID, Err}`（`Err` 是 page 的拒绝，`Unwrap` 到它）"（3.26 第 3 条：带字段的错误类型不叫 `Err…`；审查 C2-N3）；"有预检的路径要到 P2/S2 的重复才有只有守卫拒绝的情形"：P1 里已有两种，都是执行者被降为写者：`batch` 的，降级落在计划与单元之间（3.24 的 `TestTheGuardRefusesWhatThePlanLetBy`，经 MCP 的 `batch`，`guards: []` 时同样的交错让移动成功；草稿以 REST 的 `moveNode` 写它，那条路径没有预检，审查 B-I1 = C-I4）；后台撤销的，降级落在两批之间（第 19 条：每一批由它的守卫判）。上传、同步的撤销与 `startRevert` 在 P1 没有，到 P2/S2 的重复才有（3.25）。改为："有预检的路径上只有守卫拒绝的情形：P1 有 `batch` 的计划与单元之间、后台撤销的两批之间的降级；上传、同步的撤销与 `startRevert` 的到 P2/S2 的重复才有"。第 9 节的"有预检的路径"（`:654`）同样："P2/S2 补上"之后加"（P1 已有 `batch` 的计划与单元之间的降级，P1 设计 3.24）"（审查 C3-N3）。
9. **降级的探查看不到的**（4.5）：加一句："任务的行之外，一次正在上传的导入与一次正在开始的任务只是进程里的占位（`transfer/app/start_import.go:82-86`、`start_background.go:125-136`），降级看不到它们、照样成功；它们写下任务的行时以 `FOR SHARE` 锁笔记本行，排在降级之后，按普通的契约走"（3.10）。
10. **`mode` 的载荷**（4.5"事件"一段，`:145`）："载荷 `{workspace_id, notebook_id}`"指帧：发布的数据是 `{}`，两个 id 由流写进帧（`events/domain/frame.go:38-57`）（3.11）。
11. **矩阵的 LLM Wiki 的变体**（4.17，`:405`）：变体的目标在它自己的工作区里，矩阵因此要按行的工作区（`targetViolation` 今天要求每个目标在 `workspaceOf(c)` 里、它对每个笔记本的列都答 `lab`，`bootstrap/permission_matrix_coverage_test.go:125-150`、`permission_matrix_test.go:127-136`），P1/S1 加上（3.14）；"`listedNotebooks` 与 MCP 的 `notebooks` 的期望不变"只对前者成立：MCP 的 `notebooks` 的期望（`accountNotebooksOf`）是手写的，播下变体的那个提交给它手写加上变体的笔记本，MCP 的 `mode` 落下之后它们带 `mode`（审查 C-M1）；MCP 的矩阵每个工具恰一行（`mcpMatrixViolations`，`permission_matrix_mcp_test.go:382-427`），它的变体的行随 P3 的 `lint`、`pending_sources`；"来源上的……删除（写者答 `page.source_admin_only`、管理员成功）"是 P2 的，P1 里来源根谁也删不掉（3.22）；所以那一句的"跑在变体上的行"按 Phase 写：来源根的改名、移动是 P1 的，来源上的改名、移动、删除是 P2 的，MCP 的变体的行随 P3。
12. **网页的一步与 `EventMode` 的导出**（第 7 节 P1 的 S1；4.16）：网页的 CI 另有 `make knip`（`.github/workflows/ci.yml:156-157`），未用的导出类型过不了它（knip 6.38.0 的默认问题里有 `types`），所以 `event.service.ts` 导出 `EventMode` 在 S4，与它的处理同一个提交（M6 的 `EventLinks` 也是，`52ac9870`）；契约与生成的类型在 S1（4.1）。第 7 节 P1 的 S1 的"`EventMode` 与 `event.service.ts` 导出的类型"改为"契约的 `EventMode`"，S4 的"`mode` 的处理（重读笔记本的列表）"之后加"与 `event.service.ts` 导出的 `EventMode`（与处理同一个提交：网页的 CI 的 `make knip` 不收未用的导出类型）"；4.16 的"文字"一行（`:400`）列的网页的一步漏了它："跑 `gen-check-web`、lint、测试与构建"改为"跑 `gen-check-web`、lint、`make knip`、测试与构建"。
13. **来源根的树的菜单**（4.16 的"页面树"一行，`:391`）："来源根与来源……管理员全有"对来源根不成立：4.9 写着来源根不能删除、"来源根的行没有删除"；管理员有改名、移动，没有删除（4.4）。
14. **`AGENTS` 与来源根的排序**（4.9 的表）：同一父节点下的排序是一步 `OpMove`（4.7 的表"移动、排序"），所以排序 `AGENTS` 答 `page.protected`，写者排序来源根答 `page.source_admin_only`；兄弟的重新编号不是步，不受影响（`page/app/unit_place.go:132-153`）（3.21）。表的 `AGENTS` 一行的"不能……移动"加"（含排序）"。
15. **00048 与 sqlc**（4.2 的 00048 一行）：加"notebook 的 sqlc 一项列出它（`server/sqlc.yaml:55-58`；`archtest/sqlc_test.go:267-271` 要每一项列出它的模块的每个迁移）"；4.1 的"wiki 要碰的"里的 `archtest/composition_test.go:50-83` 不必改（它不按模块名列），起作用的是命令行的路径上不许建模块的 `New`（3.8）。
16. **测试名**（第 9 节）：`TestTheImportRunsTheGuards` 不存在，导入到达守卫的模块测试是 `TestAnImportsUnitsCreatePagesAndAttachments`（`page/app/import_test.go:52-105`）。
17. **P2 的 `Step.Content` 的类型名**（4.6 的表 `:157`）：`page/app` 已有一个 `ContentMeta`（`ports.go:104-109`，版本号、字节数与最后写的人与时刻），P2 的那个另取一个名字（如 `StepContent`），交给 P2（第 9 节）。
18. **树的图标的文字等价的位置与写法**（4.16 的"页面树"一行，`:391`）：例子"来源：…""AGENTS：…"把它放在可读的名字的前面；13.2 第 17 条要可读的名字以看得见的文字开头，所以放在后面，照 `NewTab`（`W/pages/page/attachment-row.tsx:143-152`）：一个真的空格在 `sr-only` 之外、后缀以括号起头，可读的名字是 `raw (source area)`、`raw （来源区）`；为什么这样写见 4.4（审查 C-I2、C2-M1）。改为"（如 `raw (source area)`：后缀在名字之后、空格在 `sr-only` 之外）"。
19. **后台撤销的每一批不另检查**（主持者对审查 B-O1 的决定；00 号文档的十一处，审查 B2-N1 = C2-M2 补上 `:181` 与 `:191` 的括号里的 `CheckRevertPart`）：一批是一个单元，守卫在它的步里以同样的码与节点拒绝、整个单元回滚；草稿自己的反向对照（"去掉每批的预检，结果相同"）就说明它不加什么；后台的路径要的是 `revertDefect` 认出标出的拒绝。所以两批之间被降为写者的执行者由那一批的守卫拒绝（3.25、3.28）。00 号文档里写"每批的预检"的各处照此改：第 1 节第 3 条（`:18`"不论是每批的预检还是守卫拒绝的"改为"不论是开始时的预检还是守卫拒绝的"）、第 3 节第 2 条（`:57`"每批的预检拒绝的与只有守卫拒绝的各一例"改为"两批之间执行者被降为写者与两批之间出现的重复各一例，都由那一批的守卫拒绝"）、4.6 的预检的表（`:180`"后台每批的 `CheckRevertPart`"与"后台的在每批之前"去掉；`:181`"后台任务的开始"一行的"每批的检查照留"改为"每一批由它的单元里的守卫判"）、4.6 的"后台撤销与恢复"（`:190`"每批的预检拒绝的（如两批之间执行者的角色被降低）同样标出"改为"两批之间执行者的角色被降低时，那一批的守卫拒绝，同样标出"）、"transfer 的任务"（`:191` 的"今天每批的预检的 403 会报成 `forbidden`（"你不能再写这本笔记本"，不对），守卫的会成为 `internal`"整句改为"今天那一批的守卫的拒绝会成为 `internal`"，"同每批的预检标出拒绝的那个 Step"改为"同开始时的预检"，同一句的括号里的"下一批的 `CheckRevertPart` 答 `page.source_admin_only`"改为"下一批的守卫答 `page.source_admin_only`"）、第 7 节 P1 一行（`:531` 的"、后台每批的 `CheckRevertPart`"改为"；后台撤销的每一批不另检查，由那一批的守卫判"，"同每批的预检的标出在一个 Step"改为"同开始时的预检在一个 Step"，S3 的验证的"下一批以 `refused`/`page.source_admin_only` 停下"改为"下一批的守卫拒绝、以 `refused`/`page.source_admin_only` 停下"）、P2 一行（`:532`"（每批的预检拒绝的那一种，来源的情形）"改为"（那一批的守卫拒绝，来源的情形）"）、"大小与绿"（`:537`"去处与每批的预检的标出在同一个 Step"改为"去处与开始时的预检在同一个 Step"）、移交的落实的 M8-history 一行（`:558`"（每批的预检与守卫拒绝的都是）"改为"（由那一批的守卫拒绝）"）、第 9 节的"后台"（`:654`"一个每批的预检拒绝的（两批之间执行者被降为写者）与一个只有守卫拒绝的（两批之间出现的重复）"改为"两批之间执行者被降为写者（P1）与两批之间出现的重复（P2/S2），都由那一批的守卫拒绝"）。P2 若要一批在读写之前的检查，由 P2 的设计加回。
20. **`batch` 的计划的预检在 S2**（第 7 节 P1 一行，`:531`；主持者对审查 B-Q2 = C-Q2 的决定）：S3 的"`batch` 的计划的树的一遍"挪进 S2 的"守卫的骨架"，S2 的验证加"计划的预检与守卫同码，计划与单元之间的降级由守卫拒绝"（3.24 的 `TestTheGuardRefusesWhatThePlanLetBy`）与"模块一级以假的守卫测 `replans`"（`batch` 的 `failed` 先认标出的拒绝，3.22、3.28），S3 的验证的"模块一级以假的守卫测 `replans`、`revertDefect`"去掉 `replans`（审查 C3-N2）。没有它，S2 与 S3 之间 MCP 的 `batch` 先由编辑锁的预检答 `page.locked`、后由守卫答 `page.protected`，与 REST、MCP 的 `delete` 的次序不同（审查 B-M8a）。别的两个窗口照收：S1→S2（`AGENTS` 与 `raw` 还没有保护）与 S2→S3（加序号、后台撤销报成 `internal`），由下一个 Step 的合并关上，四个 Step 一个接一个合并、中间不合并别的、不发布（第 0 节）。
21. **R4 与祖先的排序**（4.7 的表的"移动、排序"一行，`:205`，"移动的子树里有来源根（移动来源根或它的祖先，来源跟着换位置）：管理员"；主持者对审查 B-Q1 的决定）：写者可以在同一父节点下排序来源根的祖先，来源区没有换位置（4.7 自己的理由）。"移动的子树里有来源根（移动来源根或它的祖先，来源跟着换位置）：管理员，来源区是空的也一样"改为"移动或排序来源根自己，或把它的祖先移到别的父节点下（来源跟着换位置）：管理员，来源区是空的也一样；同一父节点下排序它的祖先不要（来源区没有换位置）"（3.21 的 R4）。单元的移动的一步列出被移动的节点的整棵子树（`unit_move.go:73-88`），所以照机理是"出现在 `OpMove` 里"；R4 由此看步的第一个节点与它换没换父节点。
22. **播种者的错误**（4.4，`:120`"任何一步失败，整个创建回滚，答它的错误"；主持者对审查 A-Q2 的决定）：播种者交回的有类的错误（守卫的拒绝也是）是缺陷：模板是服务端自己的，`createNotebook` 不声明别的模块的码（M11 的冻结的码也不），所以创建答 500、日志记下这个码；ctx 的取消与期限照原样交回，同别处（3.4 第 9 步、3.5）。改为"任何一步失败，整个创建回滚；播种者的有类的错误答 500 并记下它的码，ctx 的取消与期限照原样"。
23. **降级的对话框是持有的**（4.5 的"网页"一段，`:143`"成功之后焦点移到这一节的标题（`ConfirmDialog` 的 `focusAfter`，13.2 第 17 条）"；4.16 引它；审查 C-M11）：带触发按钮的对话框随 403 之后重读的列表卸下、带走对话框里的拒绝（`W/app/confirm-dialog.tsx:19-25`、`:110-118`），所以是持有的 `ConfirmDialog`（`held`），`onClosed` 把焦点移到这一节的标题；404 也重读（同离开，4.4）。"成功之后焦点移到这一节的标题（`ConfirmDialog` 的 `focusAfter`，13.2 第 17 条）"改为"对话框是持有的 `ConfirmDialog`（`held`），成功之后 `onClosed` 把焦点移到这一节的标题：带触发按钮的对话框会随 403 之后重读的列表卸下（13.2 第 17 条）"，"403 与 409 `notebook.not_llm_wiki`"改为"403、404 与 409 `notebook.not_llm_wiki`"；4.16 的"设置"一行（`:389`）的"403 与 409 时重读列表、焦点"改为"403、404 与 409 时重读列表、持有的对话框与焦点"。
24. **谁写 `Write.Grants`**（4.6，`:161`"只有模式的守卫写它"；第 11 节主持者的决定第 39 条，`:802`"只有模式的守卫写的指针"；审查 B 的"核对过、没有问题的"一条）：单元自己的锁下的检查（`Unit.Revert` 的锁下的一遍）也问它，所以同一个单元的守卫不再问；单元之前的预检（`batch` 的计划、撤销的不加锁的一遍）另问、不写它（3.19，主持者对审查 B-M2 的决定取"一个单元问一次数单元里的问"）。改为"模式的守卫与单元自己的锁下的检查写它，单元之前的预检另问、不写它"。这细化了第 39 条：锁下的检查与守卫同在单元里、问同一个动作、看同一份成员，所以单元里仍是一个备忘、一次问；`:802` 的"只有模式的守卫写的指针"改为"模式的守卫（与单元自己的锁下的检查，M10/P1 设计 2.1 第 24 条）写的指针"，决定的记录留着、注明细化。

### 2.2 需要负责人确认

无。第 11 节的负责人与主持者的决定在本文里都照做。2.1 的第 2–18 条是照代码改说法或位置，不改那些决定（第 5、6 条把两样东西挪到它们的第一条规则所在的 Phase，第 7 条的两个位置都不写、都是拒绝，只差答哪个码）；第 1 条与第 19–24 条里的选择，与第 7、8、18 条的做法（能失败的预算的测试、经 MCP 的 `batch` 的只有守卫拒绝的情形、`NewTab` 的写法），是本文的设计审查的主持者定的（[审查记录](reviews/P1-design-review.md)"主持者的决定"），负责人可以改判。除第 24 条之外它们不碰第 11 节的决定：第 19 条去掉的是总设计的一道检查而不是一条规则，第 21 条照 4.7 自己的理由，第 22 条只定一个不该出现的错误怎样答；第 24 条细化第 11 节主持者的决定第 39 条（单元自己的锁下的检查也写 `Write.Grants`，第 24 条写明理由，`:802` 加注）。

## 3. 服务端

### 3.1 S1 的文件

| 包 | 文件 | 新或改 | 内容 |
|---|---|---|---|
| 迁移 | `server/migrations/sql/00048_notebook_notebooks_mode.sql` | 新 | 3.2 |
| 迁移 | `server/migrations/schema_test.go` | 改 | `TestConstraintAndIndexNames` 的 `notebooks` 一项加两个检查的名字；`TestChecksRejectCounterexamples` 加三个反例与一个照收的（3.2） |
| sqlc | `server/sqlc.yaml` | 改 | notebook 一条的 `schema` 加 `migrations/sql/00048_notebook_notebooks_mode.sql`（`server/sqlc.yaml:55-58` 今天只列 00009–00011） |
| `shared` | `client_name.go`、`client_name_test.go` | 改 | `ClientOf`，在 `CleanClientName` 旁边（3.7）；模式不进 `shared`：13.1 第 11 条"两个模块同形的值在组合根逐字段转换……不为它们在 shared 里建公共类型"（3.3） |
| notebook | `domain/notebook.go`、`domain/notebook_test.go` | 改 | `Mode`、`Locale`、`Notebook.Mode`、`Notebook.SourceRootID`、`Draft` 与 `CheckDraft` 的两个新值、`Downgraded`（3.3） |
| notebook | `domain/errors.go`、`domain/actions.go` | 改 | `ErrNotLLMWiki`、`ErrJobsWriting`；`ActionChangeMode`（3.3、3.10） |
| notebook | `app/extension.go` | 改 | `NotebookCreation`、`Seeder`、`seed`（播种者的有类的错误是缺陷）；`ModeChange`、`ModeSubscriber`、`publishMode`（3.5） |
| notebook | `app/create_notebook.go`、`app/write_test.go` | 改 | `NewNotebook`、事务的次序（3.4）；`TestTheSeedersRunInTheirOrder`（3.5） |
| notebook | `app/downgrade_notebook.go`、`app/mode_test.go` | 新 | `DowngradeNotebook`（3.10）；`TestTheModeSubscribersRunInTheirOrder`（3.5） |
| notebook | `app/ports.go` | 改 | `NotebookWriter.SetMode`、新端口 `WritingJobs`（3.10） |
| notebook | `app/fakes_test.go` | 改 | 假的仓库记下 `SetMode`，另有一张 `locked`（`LockNotebook` 先读它，再读 `notebooks`：找到与锁下可以答不同的模式，3.10；今天两者读同一张表，`fakes_test.go:98-114`），假的 `WritingJobs`、播种者与订阅者；假的 `CreateNotebook` 与 `SetMode` 经同一个 `checked(n)` 拒绝表的两个检查拒绝的（两种之外的模式、没有来源根的 LLM Wiki），替身在真的失败处失败（S1 的偏离 13，审查 A-N2） |
| notebook | `adapter/postgres/queries/notebooks.sql`、`cascade.sql`、`ownerless.sql`、`store.go`、`ownerless.go`、`store_test.go`、`gen/` | 改 | 读笔记本的五条查询选出两列，两处手写的转换，`CreateNotebook` 写入，新的 `SetNotebookMode`，`TestTheNotebooksAnswerTheirMode`（3.3；`ShareNotebook` 的两列在 S2） |
| notebook | `adapter/http/handler.go`、`handler_test.go` | 改 | `shared.ClientOf`、`NotebookCreate` 的两个新值、`downgradeNotebook`、`notebookOf` 带两列（3.13） |
| notebook | `adapter/mcp/notebooks.go`、`notebooks_test.go` | 改 | 每项带 `mode`，描述照 `tools.json`（3.12） |
| notebook | `module.go`、`notebooks.go`、`extension_test.go`、`tools_test.go`、`activity_test.go`、`visibility_test.go` | 改 | `Deps.Seeders`（为空时 panic）、`Deps.ModeSubscribers`、`Deps.WritingJobs`；根的 `State`、`StateOf`（`ShareState`、`LockState` 在 S2）；模块根的替身测试（3.5）；建模块的四个测试交一个什么也不写的播种者 |
| page | `domain/mode.go` | 新 | `Mode`、`NotebookState`（3.12）；没有 `mode_test.go`：两个常量与一个结构，没有行为，notebook 的模式到 page 的转换由 `TestPageNotebooksConvertTheState` 与 `TestNotebooksAnswerTheirModeOverMCP` 钉住（S1 的偏离 5） |
| page | `app/ports.go` | 改 | `Notebooks.StateOf`（3.12） |
| page | `app/unit.go`、`app/seed_writes.go` | 改、新 | `Writer.Seed` 与 `run`；`SeedWrites` 的 `Seed`、`ParseTemplate`（3.6） |
| page | `tree_writes.go`、`tree_writes_test.go`、`module.go` | 改 | `TreeWrites` 加 `Seed`、`ParseTemplate`，`SeedSpec`、`PageDraft` 的别名（3.6） |
| page | `app/open_notebook.go`、`adapter/mcp/open_notebook.go` 与它们的测试 | 改 | `Opened.Mode`、答复的 `mode`（3.12） |
| page | `adapter/http/handler.go`、`history.go`、`trash.go`、`versions.go` | 改 | `clientOf` 换成 `shared.ClientOf`，转换为 `domain.Client`（3.7）；测试不改 |
| page | `app/fakes_test.go`、`extension_test.go`、`adapter/postgres/paths_property_test.go` | 改 | 假的 `Notebooks` 加 `StateOf` |
| asset | `adapter/http/upload.go` | 改 | 同 page，照旧用字符串（3.7） |
| transfer | `adapter/http/server.go` | 改 | 同 page（3.7） |
| transfer | `writing_jobs.go`、`writing_jobs_test.go` | 新 | `transfer.NewWritingJobs(pool)`（3.10） |
| access | `domain/rules.go` | 改 | `notebook.change_mode` 一行（3.10） |
| wiki（新模块） | `module.go` | 新 | 模块根：`NewTemplate`、`Actions()`、`TemplatePages`、类型别名、常量 `ModeLLMWiki`、`LocaleEn`、`LocaleZhCN`（3.8） |
| wiki | `domain/contract.go`、`domain/contract_test.go` | 新 | 模式的契约表（3.8） |
| wiki | `domain/template.go`、`domain/template_test.go`、`domain/template-en-raw.md`、`domain/template-en-agents.md`、`domain/template-zh-CN-raw.md`、`domain/template-zh-CN-agents.md` | 新 | 模板的四页、变更集的说明 `TemplateMessage` 与它们的测试（3.9；平铺在 `domain` 里，同 3.8 的树） |
| wiki | `app/ports.go`、`app/template.go`、`app/template_test.go` | 新 | 端口 `Seeds`；播种者 `Template`（3.8） |
| wiki | `template_test.go` | 新 | 模块根上以假的 `Seeds` 证明播种者到达、只在 `llm_wiki` 写、交回来源根（3.8） |
| bootstrap | `wire.go`、`deps.go` | 改 | 组合根的次序（3.8）、`notebookDeps` 交播种者与写入任务的端口、`pageDeps` 交 `pageNotebooks` |
| bootstrap | `wiki.go`、`page_notebooks.go`、`page_notebooks_test.go` | 新 | `wikiSeeds`、`wikiTemplate`；`pageNotebooks` 与 `TestPageNotebooksConvertTheState`（3.8、3.12） |
| bootstrap | `registrants.go`、`events_registrants.go` | 改 | `notebookExtensions.modeSubscribers`；`notebookEvents.ModeChanged`（3.11） |
| bootstrap | `actions_test.go` | 改 | 并集加 `wiki.Actions()`（3.8） |
| bootstrap | `wiki_template_test.go`、`wiki_downgrade_test.go`、`interleavings_wiki_test.go` 与已有的 `interleavings_page_test.go` 的 `checkPages`、`events_stream_test.go`、`mcp_account_test.go`、`permission_matrix_notebook_test.go`、`permission_matrix_wiki_test.go`、`notebook_visibility_test.go`（读 `wlab` 的两个测试，3.14） | 新、改 | 第 7 节；取名照 7.5 |
| 契约 | `api/modules/notebook.yaml`、`api/modules/events.yaml`、`api/mcp/tools.json` | 改 | 3.13 |
| 网页 | 生成的类型、`test/fakes.ts`、`stores/notebook.store.test.ts`、`app/problem-messages.ts`、`i18n/messages/{en,zh-CN}.ts`、`services/event.service.ts` | 改 | 第 4.1 节 |
| e2e | `fixtures/notebooks.ts`、`fixtures/assert/notebook.ts`、`stories/wiki/lw1-create.spec.ts` | 改、新 | 第 5 节 |

### 3.2 S1：迁移 00048

```sql
-- notebooks.mode and source_root_id: a notebook's mode, standard or llm_wiki, and an LLM Wiki's source root, the page
-- its template wrote, whose subtree is its source area (M10 design 4.2, 4.3). The source root has no key, as
-- transfer_jobs.root_id has none: a key would have the notebook and page modules reference each other and break the
-- purge's order, leaf to root (bootstrap/registrants.go, purgers); a notebook's deletion deletes the page with it,
-- and a downgrade sets it NULL. The check asks an LLM Wiki for a source root, not a standard notebook for none: M11's
-- upgrade may set one before the mode. M11's upgrade states are its own, never a mode.

-- +goose Up
ALTER TABLE notebooks ADD COLUMN mode text NOT NULL DEFAULT 'standard',
    ADD CONSTRAINT notebooks_mode_check CHECK (mode IN ('standard', 'llm_wiki')),
    ADD COLUMN source_root_id uuid,
    ADD CONSTRAINT notebooks_source_root_check CHECK (mode <> 'llm_wiki' OR source_root_id IS NOT NULL);

-- +goose Down
-- A program before it knows no mode: an LLM Wiki reads as a standard notebook, its pages kept, raw and AGENTS among
-- them, its source root's id dropped with the column.
ALTER TABLE notebooks DROP CONSTRAINT notebooks_source_root_check, DROP COLUMN source_root_id,
    DROP CONSTRAINT notebooks_mode_check, DROP COLUMN mode;
```

- 写法照 00028（`00028_linking_page_links_resolved_asset.sql:6-13`：列与它的检查在一条 `ALTER TABLE` 里，Down 的第一句说之前的程序看到什么）与 00034（`00034_page_changesets_history.sql:12-17`：Down 先 `DROP CONSTRAINT` 再 `DROP COLUMN`）。`DEFAULT 'standard'` 是常量，PostgreSQL 11 起加列只改元数据；两个检查各扫一遍 `notebooks`（小表，00-M10 4.2 不计）。只写明列的 `INSERT INTO notebooks` 照旧：测试里 36 处，另两处是模块自己的 `CreateNotebook` 与它生成的副本（00-M10 4.2 的"测试里 38 处"含这两处，2.1 不列：不改结论）。
- `schema_test.go` 的 `TestConstraintAndIndexNames`（`:163`）的 `want` 加两项，照排序插在 `notebooks` 那几行里（`:305-314`）：`"notebooks_mode_check c"` 在 `"notebooks_former_owner_id_fkey f a"` 之后，`"notebooks_source_root_check c"` 在 `"notebooks_pkey p"` 之后。`checkedRows`（`:465-468`）的笔记本那一行照建 LLM Wiki 的写法写 `mode = 'llm_wiki'` 与一个 `source_root_id`（"照领域的写法写一行"的那一半；普通笔记本由其余 36 处的默认值写）。`TestChecksRejectCounterexamples`（`:582-587` 那几行旁边）加三个反例，各答 23514 与它的约束名：`{"a third mode", "UPDATE notebooks SET mode = 'upgrading'", "notebooks_mode_check"}`、`{"an upper-case mode", "UPDATE notebooks SET mode = 'LLM_WIKI'", "notebooks_mode_check"}`、`{"an LLM Wiki without its source root", "UPDATE notebooks SET source_root_id = NULL", "notebooks_source_root_check"}`。
- Down：`TestMigrationsGoUpDownAndUpAgain`（`:66-106`）只比较对象的列表，看不到列。所以照 00045 的测试（`server/migrations/changesets_moves_test.go:135-140`，`downTo`，`background_jobs_test.go:14-26`）加 `server/migrations/notebooks_mode_test.go` 的 `TestTheNotebooksModeGoesDown`：迁到最新，写一本 `llm_wiki`（带来源根）与一本带 `source_root_id` 的 `standard`（照收：检查只要求 LLM Wiki 有它，M11 的升级可以先写来源根；把检查写成 `(mode = 'llm_wiki') = (source_root_id IS NOT NULL)` 时这一步失败），`downTo(t, m, 48)`（撤掉 00048 自己，`background_jobs_test.go:15-26` 在撤掉的版本等于参数时停）之后 `information_schema.columns` 里 `notebooks` 的 `mode`、`source_root_id` 是 0 列、两本都还在。

### 3.3 S1：notebook 的模式、核对与查询

**domain**（`notebook/domain/notebook.go`）：

```go
// Mode is a notebook's mode (M10 design 4.3): standard, or llm_wiki. An
// upgrade's states are M11's own, never a mode.
type Mode string

const (
	ModeStandard Mode = "standard"
	ModeLLMWiki  Mode = "llm_wiki"
)

// Locale is the language of an LLM Wiki's template (M10 design 4.4): the
// web sends its own; en when a request gives none.
type Locale string

const (
	LocaleEn   Locale = "en"
	LocaleZhCN Locale = "zh-CN"
)

type Notebook struct {
	// … the fields of today (notebook/domain/notebook.go:14-23)
	// Mode is the notebook's mode; SourceRootID an LLM Wiki's source root,
	// the zero id for a standard notebook (notebooks_source_root_check).
	Mode         Mode
	SourceRootID uuid.UUID
}

// Draft is a new notebook's checked values.
type Draft struct {
	Name   string
	Access shared.WorkspaceAccess
	Mode   Mode
	Locale Locale
}

// CheckDraft checks a new notebook's name, workspace access, mode and
// template language, every problem at once (422): the name by
// shared.CheckTitle, the access one of the three, the mode one of the two,
// the language zh-CN or en, whatever the mode; none, standard and en when
// they are nil.
func CheckDraft(name string, access, mode, locale *string) (Draft, error)

// Downgraded is n a standard notebook at at (M10 design 4.5): no source
// root; ErrNotLLMWiki for a notebook that is none.
func (n Notebook) Downgraded(at time.Time) (Notebook, error)
```

- `SourceRootID` 是值、零值为没有，不是指针：`notebook/app/write_test.go:46`、`:146` 一类以 `!=` 比较整个 `domain.Notebook`，指针会比地址。适配器在 `NULL` 与零值之间转换（`store.go:90-100` 的 `notebookOf`）。
- 两个新值的错照 `checkAccess`（`notebook/domain/notebook.go:98-104`）：`{Field: "mode", Code: invalid_format, Message: "must be standard or llm_wiki"}`、`{Field: "template_locale", Code: invalid_format, Message: "must be zh-CN or en"}`；契约的 `enum` 不在请求体的结构检查里核对（那条注释的规则），所以 `"fr"` 走到 domain，答 422（3.13 的测试）。
- `ErrNotLLMWiki = shared.NewError(shared.KindConflict, "notebook.not_llm_wiki", "The notebook is not an LLM Wiki.")`；`ErrJobsWriting = shared.NewError(shared.KindConflict, "transfer.busy", "An import, a restore or a revert of the notebook is queued or running: downgrade it once it is done.")`：跨前缀的码同 `ErrWorkspaceNotFound`（`workspace.not_found`，`notebook/domain/errors.go`）。
- `ActionChangeMode shared.Action = "notebook.change_mode"`，进 `Actions()`；规则表 `"notebook.change_mode": {Level: LevelNotebook, Notebook: notebookAdmins()}`（`access/domain/rules.go:73-77` 的那几行旁边）。M11 的发起与批准升级可以用它（00-M10 4.5）。

**查询**（`notebook/adapter/postgres/queries/notebooks.sql` 等）：

```sql
-- name: CreateNotebook :exec
-- The audit columns come from the use case's clock and caller; an LLM Wiki is inserted with its mode and source
-- root at once (M10 design 4.4), so that notebooks_source_root_check holds from its first row.
INSERT INTO notebooks (id, workspace_id, name, workspace_access, mode, source_root_id, created_by_id, updated_by_id,
    created_at, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(workspace_id), sqlc.arg(name), sqlc.arg(workspace_access), sqlc.arg(mode),
    sqlc.narg(source_root_id), sqlc.arg(by), sqlc.arg(by), sqlc.arg(now), sqlc.arg(now));

-- name: ShareNotebook :one   (M10/P1/S2, 3.18)
-- The notebook not deleted locked FOR SHARE until the transaction ends … (as today), with its mode and source root,
-- which the page write reads under this lock (M10 design 4.6): a downgrade takes the row FOR NO KEY UPDATE, so a unit's
-- mode does not change while it runs.
SELECT id, mode, source_root_id FROM notebooks
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
FOR SHARE;

-- name: SetNotebookMode :exec
-- A notebook's mode and source root, by by at now (M10 design 4.5): the downgrade's write, under the row's lock.
UPDATE notebooks
SET mode = sqlc.arg(mode), source_root_id = sqlc.narg(source_root_id), updated_by_id = sqlc.arg(by),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);
```

`FindNotebook`、`LockNotebook`（`notebooks.sql:7-20`）、`ListNotebooks`（`:42-55`）、`LockOwnerlessOf`（`cascade.sql:37-44`）、`ListOwnerless`（`ownerless.sql:1-9`）的选择列表各在 `former_owner_id` 之后加 `mode, source_root_id`（`ListNotebooks` 加 `n.mode, n.source_root_id`）：读笔记本的列的查询读的相同。转换有三种：`FindNotebook` 的行直接给 `notebookOf`（`store.go:67`），`LockNotebook`、`LockOwnerlessOf` 的行以类型转换 `gen.FindNotebookRow(row)` 给它（`store.go:76`、`cascade.go:57`：列不同就编译不过）；**两处是手写的**：`ListNotebooks` 在 `store.go:126-131` 当场建 `domain.Notebook`，`ListOwnerless` 在 `ownerless.go:20-23` 逐字段填一个 `gen.FindNotebookRow`。这两处各加 `Mode`、`SourceRootID`（`ListNotebooks` 照 `notebookOf` 转换 `NULL`；或改为也经 `notebookOf`），漏了编译照过、答复里写出空的 `mode`（契约的枚举之外）：`TestTheNotebooksAnswerTheirMode`（`notebook/adapter/postgres/store_test.go`，真库）对一本 LLM Wiki 经 `FindNotebook`、`LockNotebook`、`ListNotebooks`、`LockOwnerlessOf`、`ListOwnerless` 各读一次，五次都是 `llm_wiki` 与它的来源根（无主的两条在它的管理员离开之后读）。`UpdateNotebook`（`:30-34`）不碰两列：`NotebookUpdate` 是关着的（3.13），改模式只经降级。

**模块根的读**（`notebook/notebooks.go`）：`Notebooks` 加三个方法（`StateOf` 在 S1；`ShareState`、`LockState` 与 `ShareNotebook` 的两列在 S2，3.18，这里一并写出），`ShareByID`、`LockByID` 照旧（transfer 与 linking 的端口用它们：`transfer/app/ports.go:36`、`transfer/app/start.go:109`、`transfer/app/export.go:405`、`linking/app/reindex.go:42`、`:87`；改了它们的签名就要给那两个模块各一个转换，而它们不读模式）：

```go
// State is a notebook's mode and source root, as the modules inside it
// read them (M10 design 4.3, 4.6): the zero id for none.
type State struct {
	Mode         Mode
	SourceRootID uuid.UUID
}

// Notebooks (as today), and:
	// StateOf is the state of the notebook not deleted with id, unlocked,
	// in the caller's transaction when it has one, and whether there is
	// one: the reads that hold no lock (MCP's open_notebook; M10's list,
	// read, instructions, lint) see one side of a downgrade or the other.
	StateOf(ctx context.Context, id uuid.UUID) (State, bool, error)
	// ShareState is ShareByID with the state read under the lock, and
	// LockState LockByID's: a unit's mode does not change while it runs.
	ShareState(ctx context.Context, id uuid.UUID) (State, bool, error)
	LockState(ctx context.Context, id uuid.UUID) (State, bool, error)
```

`StateOf` 读 `FindNotebook`；`ShareState`、`LockState` 照 `ShareByID`、`LockByID`（`notebooks.go:64-85`）先核对在事务里，再读 `ShareNotebook`、`LockNotebook`；`ShareByID`、`LockByID` 改为调用它们、丢掉状态。`notebook/notebooks_test.go:122-131` 的两个事务之外报错的核对各加新的一对。根上另以别名公开 `Mode`、`ModeStandard`、`ModeLLMWiki`（组合根的转换用）。page 读到的是它自己的 `domain.NotebookState`，由组合根的 `pageNotebooks` 逐字段转换（3.12）。

### 3.4 S1：建的事务

**用例**（`notebook/app/create_notebook.go`）：

```go
// NewNotebook is a creation's request: the name, and the access, mode and
// template language when given; Client is where the request came from,
// which a template's changeset records (M10 design 4.4).
type NewNotebook struct {
	Name                 string
	Access, Mode, Locale *string
	Client               string
}

// CreateNotebookDeps: today's, and Seeders, the registrants of the
// creation (Seeder), which New refuses to be empty.

// Execute creates the notebook r names in the workspace of slug, its
// caller its admin (as today), and, for an LLM Wiki, its template, in the
// one transaction: …
func (c *CreateNotebook) Execute(ctx context.Context, slug string, r NewNotebook) (View, error)
```

**次序**，一个列表（`[事务外]` 之后都在 `Tx.WithinTx` 里；每一步答的码在括号里）：

1. `[事务外]` `shared.RequireActor`；`Workspaces.FindBySlug`，不加锁（404 `workspace.not_found`）——今天的 `:43-53`。
2. 工作区行 `FOR SHARE`（`Workspaces.ShareByID`，`:56-58`；等锁期间删了的答 404）。成员关系的一切变化与工作区的删除持它的 `FOR NO KEY UPDATE`，与这个事务串行（3.15）。
3. 判定 `notebook.create`（`:59-62`；看不到的 404、访客的 403）。
4. 取值的核对 `CheckDraft(r.Name, r.Access, r.Mode, r.Locale)`（422 `validation_failed`，`name`、`workspace_access`、`mode`、`template_locale` 一起答）：排在 404、403 之后（13.1 第 4 条；今天 `:63-66` 的位置）。
5. 读一次时钟：`now := c.d.Clock.Now()`（`:67`）。这是这次创建唯一的一次读：笔记本行、管理员行、交给播种者的值与模板的变更集、节点、正文、版本、条目都是它（13.1 第 9 条）。
6. 定下 id：笔记本的 `uuid.NewV7()`；`llm_wiki` 时来源根的 id 也在这里定下（`uuid.NewV7()`，在笔记本的之后，所以按 id 排序时笔记本在前、来源根在它的页里最早，3.28 用到）。
7. 插入笔记本（`CreateNotebook`，带 `mode` 与 `source_root_id`；`notebooks_source_root_check` 由第 6 步成立）。
8. 插入管理员（`AddMember`，`:73-75`）。
9. 播种者（`seed`，3.5）：按登记的次序调用每一个，第一个错误即停、整个回滚。播种者交回的有类的错误（`*shared.Error`，守卫的拒绝也是）是缺陷：模板是服务端自己的，`createNotebook` 不声明 page 的任何码（M11 的冻结的码也不），所以 `seed` 把它换成不带类的错误、码写进错误的文字，答 500，平台照 `FaultUnknown` 记 error 一行，带着这个文字（`platform/httpserver/apierrors.go:222-223`）；ctx 的取消与期限照原样交回（`ProblemOf` 照它们自己的办法答，`apierrors.go:270-273`），别的错误照原样（主持者对 A-Q2 的决定，3.5）。wiki 的模板在 `llm_wiki` 时在这里经 `TreeWrites().Seed` 跑一个改树的单元（3.6），**锁笔记本行**：
   - 单元先在事务之外的位置读工作区（`Writer.workspaceOf`，`page/app/unit.go:167-176`）：`notebook.NewNotebooks(pool).WorkspaceOf` 经 `FindNotebook`，`postgres.DB(ctx, pool)` 在 ctx 带着事务时用这个事务（`platform/postgres/tx.go:36-44`），所以读得到第 7 步刚插入、还没提交的行。
   - `Tx.WithinTx` 加入外层的事务：ctx 已带事务时它直接调用 `fn(ctx)`（`platform/postgres/tx.go:121-129`），不开新的、不提交。
   - 工作区行 `FOR SHARE` 再取一次（`page/app/unit.go:214`）：同一个事务已持有同样的锁，不等。
   - 笔记本行 `FOR NO KEY UPDATE`（`LockByID`，`:217-223`）：这一行是这个事务插入的，提交之前别的事务看不到它、锁不到它，同一个事务锁自己插入的行不等。13.1 第 5 条的"建笔记本只锁工作区行"由此改写（第 8 节）。
   - 判定 `page.create`（`:224-227`）：access 的 `Authorizer` 经 `notebook.NewFacts(pool)` 读笔记本的事实（`notebook/facts.go:29-31` → `store.go:173-182`，同样经 `postgres.DB`）与 `workspace.NewMemberships(pool)` 的工作区角色，都在这个事务里：第 8 步的管理员行在，判定得到 `admin`。
   - 单元的时刻是交来的 `now`，不读 page 的时钟（3.6）。
10. 核对来源根（`seed` 的返回）：`llm_wiki` 时恰有一个播种者交回非零的 id，且等于第 6 步的；`standard` 时没有一个交回非零的 id。播种者交回的是它写下的：wiki 的模板交回 `Seed` 写下 `raw` 的那个 id（3.8），所以这一条核对的是写下的来源根（S1 审查 A-N3）。否则是缺陷：`fmt.Errorf("notebook: the seeders answered source roots %v for notebook %s of mode %s; want exactly its own", …)`，不带名称，500，整个回滚（00-M10 4.4、A-M7）。
11. 可见性的事件（`publishVisibility`，`:76-79`）：在播种者之后，同 00-M10 4.4。它的注册者（事件流）只 `NOTIFY`。
12. 答复的 `View`（`:80`），带 `Mode`、`SourceRootID`。
13. 提交（`WithinTx` 返回）；日志一行 info "notebook created"，照旧只带 `workspace_id`、`notebook_id`、`user_id`（`:86-87`），另加 `mode`（不含个人信息的取值，13.1 第 10 条）。

`Writer.Run` 拒绝已在事务里的 ctx（`page/app/unit.go:186-188`）照旧：只有 `Seed` 加入调用方的事务（3.6）。模板的单元的观察者（页面事件的 `NOTIFY`、链接索引、附件的观察者）与搜索的入队都在这个事务里，回滚时一起没有；页面事件的 `NOTIFY` 照样发出、随提交送达，但没有一条流转发它：流在打开时定下它转发的笔记本（`events/app/open_stream.go:30-67` 的 `seenBy`；`events/app/stream.go:89`、`:120-127`），新的笔记本不在任何一条里面（00-M10 4.4）；建的人与工作区的成员的流随第 11 步的可见性事件重连，之后看得到它。

**为什么第 9 步在第 8 步之后、第 11 步之前**：播种者要读到管理员行（第 9 步的判定）；可见性的订阅者让流重连，重连的流读到的必须是带着模板的笔记本（同一个事务，提交之后才看得到，次序只关乎值里的东西，可见性的值不带页）。

**不另锁笔记本行**：用例自己不在第 7 步之后锁它：插入的行在提交之前只有这个事务看得到，锁它什么也不挡；锁出现在模板的单元里，只因为单元对每个改树的写都这样（`UnitSpec.Tree`）。

### 3.5 S1：播种者与模式变化的订阅者（两个新的扩展点）

`notebook/app/extension.go` 加（照 `NotebookDeletion` 与 `NotebookDeletionSubscriber` 的形状，`:17-43`；值的类型与方法同名的一对照 `VisibilityChange`/`VisibilityChanged`）：

```go
// NotebookCreation is a notebook being created (M10 design 8): its id and
// workspace, its mode, its source root (the zero id for a standard one),
// its template's language, who created it, from where, at the creation's
// one time.
type NotebookCreation struct {
	NotebookID   uuid.UUID
	WorkspaceID  uuid.UUID
	Mode         domain.Mode
	SourceRootID uuid.UUID
	Locale       domain.Locale
	By           uuid.UUID
	Client       string
	At           time.Time
}

// Seeder writes what a notebook is created with, after the notebook and its
// admin were inserted and before the visibility's event, in the
// creation's transaction: it answers the source root it wrote, the zero id
// for none, and any error rolls the creation back. Exactly one seeder
// answers an LLM Wiki's source root, the one the creation decided; none
// answers one for a standard notebook. M10's is the wiki module's template.
type Seeder interface {
	NotebookCreated(ctx context.Context, c NotebookCreation) (uuid.UUID, error)
}

// seed calls the seeders in order, the first error stopping it, and
// answers the source roots they wrote, the zero ids left out. A seeder's
// problem (a *shared.Error, a guard's refusal among them) is a defect: the
// template is the server's own, and createNotebook declares no code of
// another module's (M10/P1 design 3.4). It is answered as an error
// without a kind, the problem's code in its text, which the 500's log
// carries; any other error passes as it is, ctx's cancellation and
// deadline among them (no *shared.Error wraps one).
func seed(ctx context.Context, seeders []Seeder, c NotebookCreation) ([]uuid.UUID, error) {
	var roots []uuid.UUID
	for i, s := range seeders {
		root, err := s.NotebookCreated(ctx, c)
		var problem *shared.Error
		switch {
		case err == nil:
		case errors.As(err, &problem):
			return nil, fmt.Errorf("notebook: seeder %d refused notebook %s with %s, a defect: the template is the server's own",
				i, c.NotebookID, problem.Code)
		default:
			return nil, err
		}
		if root != (uuid.UUID{}) {
			roots = append(roots, root)
		}
	}
	return roots, nil
}

// ModeChange is a notebook's mode changing (M10 design 8): from and to,
// who changed it, at the change's one time.
type ModeChange struct {
	WorkspaceID uuid.UUID
	NotebookID  uuid.UUID
	From, To    domain.Mode
	By          uuid.UUID
	At          time.Time
}

// ModeSubscriber follows a mode change, after the notebook's row was
// written and in its transaction: an error rolls everything back. M10's is
// the event stream's mode; M11's upgrade changes the mode through it too.
type ModeSubscriber interface {
	ModeChanged(ctx context.Context, m ModeChange) error
}

// publishMode calls the subscribers in order; the first error stops it.
func publishMode(ctx context.Context, subscribers []ModeSubscriber, m ModeChange) error
```

- 00-M10 第 8 节把值叫作 `NotebookCreated`、`ModeChanged`；照 notebook 已有的两对（`NotebookDeletion`/`NotebookDeleted`、`VisibilityChange`/`VisibilityChanged`，`extension.go:17-33`、`:80-97`），值是名词、方法是过去分词（2.1 第 3 条）。
- 播种者交回它写下的，与 13.1 第 21 条"订阅者只交回错误"不同，是新的扩展点的形状（12.4 的那一行已写，00-M10 4.4）。
- ctx 的取消与期限不另写一支：`shared.Error` 没有 `Unwrap`（`shared/error.go:111-133`、`:208-214`），这条路上唯一的 `errors.Join`（`platform/postgres/tx.go:170`）只合 ctx 与回滚的错误，所以没有 `*shared.Error` 包着 ctx 的错误，它们走 `default` 照原样交回；多写一支是等价的变异（审查 AC-N4）。`TestTheSeedersRunInTheirOrder` 的两行 ctx 照留：`default` 也被当作缺陷时它们失败。
- `seed` 的有类的错误一支不以 `%w` 包它：包了，`ProblemOf` 经 `errors.As` 照样找到它，答它的码（`platform/httpserver/apierrors.go:260-262`）。码进文字，不带 `Detail`（守卫的说明里有页的名称，日志不记取值，13.1 第 10 条）。
- 模块根（`notebook/module.go`）以别名公开 `NotebookCreation`、`Seeder`、`ModeChange`、`ModeSubscriber`；`Deps` 加：

```go
	// Seeders are the creation's registrants (Seeder): New panics without
	// one, a fault of its wiring, as without AccountWorkspaces; ModeSubscribers
	// follow a mode change; WritingJobs tell whether a notebook has a job
	// that writes queued or running, which a downgrade waits for.
	Seeders         []Seeder
	ModeSubscribers []ModeSubscriber
	WritingJobs     WritingJobs
```

  `New` 的检查（`module.go:88-91`）改为一个 `switch`，`AccountWorkspaces` 在先（`TestTheModuleRefusesNoAccountWorkspaces`，`tools_test.go:21-28`，以 `Deps{}` 期待那一句，不改），再 `len(d.Seeders) == 0`（"notebook: no seeders"）、`d.WritingJobs == nil`（"notebook: no writing jobs"）。模块根新测试 `TestTheModuleRefusesNoSeeders`、`TestTheModuleRefusesNoWritingJobs`（照 `tools_test.go:21-28`）；建模块的四处（`tools_test.go:33`、`activity_test.go:84`、`visibility_test.go:110`、`extension_test.go:224`）交 `Seeders: []notebook.Seeder{noSeed{}}`（什么也不写、交回零）与 `WritingJobs: noJobs{}`。模式变化的订阅者可以为空（降级不需要它就成立）；组合根交空时由 3.11 的最后一跳抓到。
- **替身与次序的测试**（13.1 第 21 条，照 `TestANotebookDeletionPassesTheSubscriber`，`notebook/extension_test.go:200-246`，真库、照组合根接的模块）：
  - `TestACreationPassesTheSeeders`：两个播种者的替身在建的事务里依次调用（记下次序与 `postgres.InTx`），各读一次笔记本行与管理员行（读得到）；值等于请求的（`Mode`、`Locale`、`By`、`Client` 是 `web`、`At` 是 `tickingClock` 的第一次读，`extension_test.go:36-44`）。表格：standard 两个都交回零 → 201；llm_wiki 第一个交回值里的 id → 201、行的 `source_root_id` 是它；llm_wiki 两个都交回零、两个都交回、交回一个别的 id、standard 而交回一个 → 500，之后没有笔记本、成员；第一个失败 → 500、第二个没被调用、什么也没写；第一个交回 `shared.NewError(shared.KindConflict, "page.protected", "…")` → 500 `internal_error`（不是 409），什么也没写。这里不核对日志：500 的 error 一行由 API 的 `APIErrors` 以 `httpservertest.APIOptions.Logger` 写（`httpservertest/api.go:25`、`:44-53`，`apierrors.go:222-223`），不经模块的 `Deps.Logger`；码在错误的文字里由 `TestTheSeedersRunInTheirOrder` 核对，记下那一行是平台自己的、测过的事，整个程序的 `TestTheTemplateRollsBackWithItsNotebook` 读它（审查 AC-N3）。授权者：`adminsAuthorizer`（`extension_test.go:48-62`）对 `notebook.create` 答 `ErrNotVisible`（建的人还不是那一本的管理员），所以这个测试另交 `creatorsAuthorizer`：`notebook.create` 一律答 `WorkspaceMember`（判定不是这个测试要看的；`newFixture` 不写 `workspace_members` 的行，`extension_test.go:113-127`，读成员关系就会拒绝 alice），别的交给 `adminsAuthorizer`。
  - `TestADowngradePassesTheModeSubscribers`：两个订阅者的替身依次调用，在降级的事务里，读到 `mode` 已是 `standard`；值 `{From: llm_wiki, To: standard, By, At}` 的 `At` 等于行的 `updated_at`；第一个失败 → 500、行仍是 `llm_wiki`、第二个没被调用。行在降级之前由 bob 改过（`updated_by_id`）：alice 降级之后是 alice，失败的一行回滚之后仍是 bob（S1 审查 B-M2：夹具的笔记本都是 alice 的，不先这样设，SQL 不写 `updated_by_id` 也看不出）。
  - `notebook/app/write_test.go` 的 `TestCreateNotebookMakesTheCallerItsAdmin`（`:36`）的调用次序：假的播种者与假的可见性的订阅者记进同一个列表，`"Seed … in tx"` 在 `"AddMember …"` 之后、`"VisibilityChanged … in tx"` 之前（第 1 条变异）。
  - `notebook/app` 的分发循环：`TestTheSeedersRunInTheirOrder`（`write_test.go`，经 `CreateNotebook`）、`TestTheModeSubscribersRunInTheirOrder`（`mode_test.go`，经 `DowngradeNotebook`）：两个替身、第一个错误即停、事务回滚，照 `app/visibility_test.go:146` 的 `TestTwoVisibilitySubscribers`。`TestTheSeedersRunInTheirOrder` 另有三行：第一个交回一个 `*shared.Error` → 错误里找不到 `*shared.Error`（`errors.As` 答假）、文字里有它的码；交回 `context.Canceled`、包着 `context.DeadlineExceeded` 的 → `errors.Is` 认得出它们、文字里没有 "defect"（7.3 的变异"播种者的问题照原样交回"）。

### 3.6 S1：page 的 `TreeWrites().Seed`

**单元**（`page/app/unit.go`）：`Run` 拆成两个入口与一份共用的 `run`，`Run` 的一切照旧：

```go
// Run (as today, page/app/unit.go:178-251): it refuses a ctx that carries a
// transaction, and reads the clock once in its unit.
func (w *Writer) Run(ctx context.Context, spec UnitSpec, do func(ctx context.Context, u *Unit) error) (Outcome, error) {
	if w.d.Tx.InTx(ctx) {
		return Outcome{}, errors.New("a page write unit opens its own transaction: ctx carries one already")
	}
	return w.run(ctx, spec, time.Time{}, do)
}

// Seed runs do in a unit of spec within the transaction ctx carries, its
// caller's, at at: the one exception of v0.1 design 13.1 item 1, a
// notebook's template, written in its creation's transaction (M10 design
// 4.4). It refuses a ctx that carries none, and a zero at; the unit reads
// no clock, so that the notebook, its changeset and its pages have one
// time. The rest is Run's: the workspace's row FOR SHARE and the
// notebook's, both re-taken in the caller's transaction, the decision, the
// unit, the search's queue, the observers. Its caller's transaction
// commits or rolls back the unit; the functions its participants deferred
// run when Seed returns, its observers done.
func (w *Writer) Seed(ctx context.Context, spec UnitSpec, at time.Time, do func(ctx context.Context, u *Unit) error) (Outcome, error) {
	switch {
	case !w.d.Tx.InTx(ctx):
		return Outcome{}, errors.New("a seed's unit joins its caller's transaction: ctx carries none")
	case at.IsZero():
		return Outcome{}, errors.New("a seed's unit is at its caller's time: none given")
	}
	return w.run(ctx, spec, at, do)
}

// run is Run's body (unit.go:189-250), the unit's time at, or the clock's
// one read when at is zero.
func (w *Writer) run(ctx context.Context, spec UnitSpec, at time.Time, do func(ctx context.Context, u *Unit) error) (Outcome, error)
```

`run` 里唯一不同的一行是 `At:`（`unit.go:230`）：`at` 为零时 `w.d.Clock.Now()`，否则 `at`。其余（客户端与种类的核对 `:189-196`、执行者 `:197-200`、工作区 `:201-204`、`deferred` 的调用 `:205-212`、`WithinTx` 里的一切 `:213-246`）不变；`WithinTx` 在 `Seed` 里加入外层的事务（3.4 第 9 步）。`deferred`（参与者经 `Appender.Defer` 交的，今天只有链接改写放还解析预算，`extension.go:162-166`）在 `Seed` 返回时调用：那时单元的观察者已做完，事务还没结束；它们放还的是观察者读过的提取结果的份额，不依赖提交（模板的两页是新建，链接改写的参与者对新建什么也不做，所以今天没有）。`Appender.Defer` 的注释加一句"in a seed's unit, when Seed returns"。

**入口**（`page/app/seed_writes.go`，新）：

```go
// SeedWrites are the template's writes (M10 design 4.4): a notebook's pages
// written in its creation's transaction, their contents parsed once, at
// start-up.
type SeedWrites struct {
	writer *Writer
	md     Markdown
}

// SeedSpec is a template's unit: in NotebookID, at At, from Client, its
// changeset's message Message, writing Pages in order, each created last
// among its siblings. The actor is ctx's, as Run's (v0.1 design 13.1
// item 1; M10 design review A2-N3).
type SeedSpec struct {
	NotebookID uuid.UUID
	At         time.Time
	Client     domain.Client
	Message    *string
	Pages      []PageDraft
}

// Seed writes spec's pages in one unit that changes the tree, of the edit
// kind, deciding page.create, within the transaction ctx carries: one
// changeset, one time. It answers the ids of the pages it wrote, in
// spec's order. A notebook the caller cannot see is notebook.not_found;
// any refusal of the unit, a guard's too, is answered as it is, its
// caller's transaction then to roll back.
func (s *SeedWrites) Seed(ctx context.Context, spec SeedSpec) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	_, err := s.writer.Seed(ctx, UnitSpec{
		NotebookID: spec.NotebookID, Action: domain.ActionCreate, Tree: true, Client: spec.Client,
		NotFound: domain.ErrNotebookNotFound, Message: spec.Message,
	}, spec.At, func(ctx context.Context, u *Unit) error {
		ids = ids[:0]
		for _, p := range spec.Pages {
			n, err := u.CreatePage(ctx, p)
			if err != nil {
				return err
			}
			ids = append(ids, n.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// ParseTemplate checks and parses content, a template's page that the
// server writes at each creation of its kind, once, at start-up: outside
// the parse budget, before any decision (v0.1 design 13.3 item 3's
// exception for it; 13.1 item 19: the creation's transaction only locks,
// compares and writes). Its facts are static: a unit's guards,
// participants and observers read them, never write them. 422 for a
// content CheckContent refuses.
func (s *SeedWrites) ParseTemplate(content string) (Facts, error) {
	if err := domain.CheckContent("content", content); err != nil {
		return nil, err
	}
	return s.md.Facts(content), nil
}
```

（交回 `Facts`，不是导入的 `Parsed`：`Parsed` 带着预算的放还（`page/app/import_writes.go:40-44`），模板的解析不取预算，`PageDraft.Facts` 要的就是 `Facts`，`page/app/unit_create.go:16-25`。）

- 同 `Run` 共用的：锁、判定 `page.create`（`domain.ActionCreate`）、单元、观察者、搜索的准确入队（`Background`、`BulkSearch` 都不设：两页，建好之后马上搜得到）；`CreatePage`（`page/app/unit_create.go:31-56`）照常做名称、父页、兄弟的标题与层级的检查，`PageDraft.ID` 本就收调用方的 id（`:17-19`、`:65-68`）：来源根以第 6 步定下的 id 写下。与 `Run` 不同的只有两样：要求在事务里（相反），时刻取交来的。
- `Seed` 交回它写下的页的 id（照 `spec.Pages` 的次序；出错时为 `nil`）：模板交回它写下 `raw` 的那一个，建的"恰有一个、等于先定下的"（3.4 第 10 步）因此核对的是写下的来源根，不是交给 `Seed` 的（S1 审查 A-N3，主持者的决定 23）。
- `ParseTemplate` 不经预算、不判定：它在启动时、任何请求之前调用，对象是服务端自己的文字（四页共约 5.4 KiB：英文的 `AGENTS` 2,209 字节、中文的 2,107，英文的 `raw` 648、中文的 593，3.9）。13.3 第 3 条照第 8 节写明这一处例外。`Markdown.Facts` 是不取预算的解析（`page/app/ports.go:737-740`，空正文的 `Decided` 就用它，`page/app/content.go:56-59`）。提取结果在几次创建之间共用：`app.Facts` 是不透明的（`ports.go:763-766`），守卫、参与者与观察者只读（下面的 `TestATemplatesParseIsItsContents` 核对两次创建收到的相同）。
- **根**（`page/tree_writes.go`）：`TreeWrites`（`:72-121`）加两个方法：

  ```go
  	// Seed writes a notebook's template, spec's pages in one unit of the
  	// edit kind, within the transaction of the notebook's creation, which
  	// ctx carries (M10 design 4.4): the one exception of v0.1 design 13.1
  	// item 1. The unit is at spec.At, from spec.Client, by ctx's actor.
  	// It answers the ids of the pages it wrote, in spec's order.
  	Seed(ctx context.Context, spec SeedSpec) ([]uuid.UUID, error)
  	// ParseTemplate parses a template's page once, at start-up, outside
  	// the parse budget: its facts are passed in each Seed's pages.
  	ParseTemplate(content string) (Facts, error)
  ```

  它们之外，别名 `SeedSpec = app.SeedSpec`、`PageDraft = app.PageDraft`（`Position` 的零值是"在最后"，模板不设它）；`treeWrites` 加 `seeds *app.SeedWrites`，`Module` 加 `seeds` 一格，`New` 里 `app.NewSeedWrites(writer, md)`（`module.go:324` 那一行旁边）。`TreeWrites` 的注释（`:12-16`、`:70-71`）加"the template's pages, in a notebook's creation"。13.1 第 1 条的"单元拒绝已在事务里的 ctx"由此有唯一的例外（第 8 节的文字）；它不是给别的 M 的扩展点，12.4 不列。
- **测试**（`page/tree_writes_test.go`，真库，照 `TestTheImportsWritesReachTheUnit`，`:74`）：
  - `TestASeedJoinsItsCallersTransaction`：测试以 `TxManager.WithinTx` 开一个事务，在里面 `Seed` 两页；提交之前另一个连接看不到它们；外层回滚之后什么也没有（节点、正文、版本、条目、变更集、搜索的队列）；外层提交之后两页都在、一个变更集、说明是交来的；`Seed` 交回的 id 是两页写下的（S1 审查 A-N3）。
  - `TestASeedReadsNoClock`：模块的时钟是 `steppingClock`（`tree_writes_test.go:155-163`，每读一次走一秒），`At` 是另一个时刻；变更集、节点、正文、版本、条目的时刻都等于 `At`（搜索的队列没有时刻的列，`00040_page_search.sql:58-64`），时钟一次也没被读（`steppingClock` 的 `at` 不变）。反向对照：`run` 照旧读时钟时这一条失败（7.3 的变异）。
  - `TestASeedRefusesNoTransactionAndNoTime`：事务之外的 ctx、零的 `At` 各答错误，什么也没写。
  - `TestASeedRunsTheGuards`：模块交一个记录的守卫（`extension_test.go:261-273` 的 `guard`），两步 `OpCreate` 都到达它，`Kind` 是 `edit`、`Client` 是交来的；守卫拒绝第二页时外层回滚、两页都没有。
  - `TestATemplatesParseIsItsContents`：模块交一个记录的观察者，Markdown 带随正文变的测试扩展 `pageMark()`（同 `TestTheContentsWritesReachTheUnit`；不带扩展时每份没有 frontmatter 的正文都解析成同样的事实，比较什么也不核对，S1 审查 A-M3）；以 `ParseTemplate` 的事实 `Seed` 一页，与以同一份正文经 REST 的 `createPage` 建一页，观察者收到的 `Change.Facts` 相同（`reflect.DeepEqual`），其中的 `page-mark` 是正文的字节数；同一份事实 `Seed` 两次（两本笔记本），两次收到的相同（事实没有被谁改写）；没有 id 的一页，`Seed` 答它写下的新 id（审查 A-N3）；5 MiB 之上与带 NUL 的正文答 422。
  - `TestASeedPassesTheFactsItIsGiven`：以另一份带 frontmatter 的正文的事实 `Seed` `raw`（先核对两份正文的事实不同），观察者收到的是交来的事实，不是 `raw` 自己的正文的：`Seed` 不再解析（"在启动时解析一次"的 page 的一半；wiki 的一半是 `TestTheTemplateSeedsAnLLMWiki` 在创建之后数解析，3.8；S1 审查 A-M2）。

### 3.7 S1：定客户端的函数挪到 `shared`

**代码查出的**：今天由凭据定客户端的函数有三份，不只 page 的一份（00-M10 4.4 说"今天只有 page 的 HTTP 适配器"，2.1 第 1 条）：`page/adapter/http/handler.go:351-362` 的 `clientOf`、`asset/adapter/http/upload.go:161-171` 的 `clientOf`（答字符串）、`transfer/adapter/http/server.go:103-113` 的 `clientOf`。三份同一条规则，读的都是 `shared.RequireActor` 交的 `Actor`：`APITokenID` 不为零是 `api`，否则 `web`；没有 `Actor` 是 401（`shared/actor.go:44-53`）。没有核对它们一致的测试（13.4 第 13 条）。notebook 要第四份。所以 S1 把它挪进 `shared`，四个适配器都调它，一份也不留。

**放在 `shared`、读 `Actor`**（主持者对 A-Q1 的决定）：输入与今天的三份相同（`Actor`，不是认证交给平台的限流的键），没有 `Actor` 照旧答 401；"PAT 是 `api`"这条规则因此不进平台（平台不导入 `shared`，`archtest/rules_test.go:40`、`:238-244`）。它在 `shared/client_name.go` 里、`CleanClientName` 的旁边：两者都是变更集的客户端的事（13.1 第 2 条）。

```go
// ClientOf is where the request ctx carries came from, as a write's
// changeset records it (v0.1 design 13.1 item 2): a personal access
// token's request is the API's, "api"; a sign-in session's access token's
// the web's, "web". It reads the request's Actor: 401 unauthorized when
// there is none, as RequireActor, which with the deny-by-default
// authentication only happens on a route wired wrongly. Each module's
// adapter converts the answer to its own type where it has one.
func ClientOf(ctx context.Context) (string, error) {
	actor, err := RequireActor(ctx)
	switch {
	case err != nil:
		return "", err
	case actor.APITokenID != uuid.UUID{}:
		return "api", nil
	}
	return "web", nil
}
```

- page 与 transfer 的适配器把答复转换为各自的 `domain.Client`（`page/domain/client.go:16-17`、`transfer/domain/job.go:53-54`：值相同），asset 照旧用字符串，notebook 把它放进 `NewNotebook.Client`；三个 `clientOf` 删掉。后台任务的 `Actor`（`JobID` 设着）不经 HTTP，不调它。
- **测试**：`shared/client_name_test.go` 的 `TestClientOfIsTheCredential`：会话的 `Actor`（`SessionID`）答 `web`，PAT 的（`APITokenID`）答 `api`，没有 `Actor` 的 ctx 答 401（`errors.Is(err, shared.Unauthenticated())`）。各模块适配器的测试照旧核对客户端（page 的 `handler_test.go:257-283` 的 `domain.ClientWeb`、`domain.ClientAPI` 各行；asset、transfer 的假的认证交的 `Actor` 已分会话与 PAT），不改：输入没有变。notebook 的假的认证今天只交会话（`notebook/adapter/http/handler_test.go:26-33`），加一个交 PAT 的 `Actor`，`createNotebook` 的表格核对交给用例的 `Client`（`web`、`api` 各一行）。13.1 第 2 条照第 8 节改写；13.4 第 13 条不必加一处：只有一份。

### 3.8 S1：wiki 模块的骨架与组合根的次序

P1 的 wiki 只有建的那一面：没有表、没有路由、没有码、没有工具（P3 起才有），所以没有 `wiki.New`、`Register`、`Tools()`，也不进 `server/sqlc.yaml`、`api/`、`deploy/runtime-grants.sql`（`archtest/sqlc_test.go:273-277` 只要有 `adapter/postgres/queries` 的模块有一项；`apitest` 按 `api/modules/<m>.yaml` 找模块的操作，没有文件就没有操作）。码的前缀 `wiki.` 由目录成立（`apitest/rules_test.go:38-54`），P3 起用。

```
server/internal/modules/wiki/
  module.go                         模块根：NewTemplate、Actions()、TemplatePages、别名、三个常量
  template_test.go                  模块根的替身测试
  domain/contract.go                模式的契约表
  domain/contract_test.go
  domain/template.go                模板：嵌入的四页、Template(locale)、TemplateMessage(locale)
  domain/template-en-raw.md         英文的 raw
  domain/template-en-agents.md      英文的 AGENTS
  domain/template-zh-CN-raw.md      中文的 raw
  domain/template-zh-CN-agents.md   中文的 AGENTS
  domain/template_test.go           段落的对应、代码之外没有链接、大小
  app/ports.go                      Seeds（page 的模板写入，经组合根）
  app/template.go                   播种者 Template
  app/template_test.go
```

- 四页放在 `domain` 里、以 `//go:embed` 读入，照已有的两处（`identity/domain/password.go:22` 的 `common_passwords.txt`、`workspace/domain/reserved.go:13` 的 `reserved_slugs.txt`）；`embed` 是标准库，`domain` 的纯净规则照收（`archtest/rules_test.go:201-214`）。文件名不叫 `AGENTS.md`：仓库里给编码 agent 读的约定文件就叫这个名字，macOS 的文件系统又不分大小写，`agents.md` 也一样会被当成它（13.4 第 1 条之外的一处实际的考虑，第 8 节不改文档）。

**契约表**（`wiki/domain/contract.go`）：

```go
// Mode is a notebook's mode, as the wiki module knows it: the composition
// root converts the notebook module's, field by field (v0.1 design 13.1
// item 11).
type Mode string

const (
	ModeStandard Mode = "standard"
	ModeLLMWiki  Mode = "llm_wiki"
)

// Locale is a template's language: en, the default, or zh-CN.
type Locale string

const (
	LocaleEn   Locale = "en"
	LocaleZhCN Locale = "zh-CN"
)

// Contract is what a mode asks of a notebook (M10 design 4.1): the pages
// its creation writes, in each language. M10 has two; M11's upgrade reads
// the same table, and a later mode is a row more.
type Contract struct {
	Mode      Mode
	Templates map[Locale][]TemplatePage
}

// TemplatePage is a page a mode's creation writes at the notebook's root,
// in order: the source root first.
type TemplatePage struct {
	Title      string
	Content    string
	SourceRoot bool
}

// ContractOf is mode's contract: standard writes nothing; llm_wiki writes
// raw, its source root, then AGENTS. false for a mode there is none of.
func ContractOf(mode Mode) (Contract, bool)
```

`Template`（`domain/template.go`）以 `//go:embed` 读入四页，`ContractOf(ModeLLMWiki).Templates[l]` 是 `[{"raw", <raw 页>, true}, {"AGENTS", <AGENTS 页>, false}]`；`raw`、`AGENTS` 两个标题在两种语言里相同（`AGENTS` 是说明的找法认的标题，`page/domain/instructions.go:11`；系统的说明与 `AGENTS` 的正文都写 `raw/`）。`contract_test.go` 钉住：`standard` 的模板为空；`llm_wiki` 的两种语言各两页、来源根恰一页且在最前；不认识的模式答 `false`。

**端口与播种者**（`wiki/app`）：

```go
// Seeds are the page module's template writes, as the composition root
// passes them (M10 design 4.4): a notebook's pages in its creation's
// transaction, one unit, answering the ids of the pages written in s's
// order; and a template's contents parsed once.
type Seeds interface {
	Seed(ctx context.Context, s Seed) ([]uuid.UUID, error)
	Parse(content string) (Parsed, error)
}

// Seed is a template's unit: notebook, time, client, the changeset's
// message, its pages in order.
type Seed struct {
	NotebookID uuid.UUID
	At         time.Time
	Client     string
	Message    string
	Pages      []SeedPage
}

// SeedPage is a page of a seed: its id, the zero one for a new id, its
// title and content, and the facts Parse answered for it.
type SeedPage struct {
	ID      uuid.UUID
	Title   string
	Content string
	Parsed  Parsed
}

// Parsed is a parsed content as the page module answers it, which the
// wiki module passes back and never reads.
type Parsed struct{ Facts any }

// (wiki's Parsed holds page.Facts, itself an opaque any: page/app/ports.go:763-766.)

// Creation is a notebook being created, as the composition root converts
// notebook.NotebookCreation.
type Creation struct {
	NotebookID   uuid.UUID
	Mode         domain.Mode
	SourceRootID uuid.UUID
	Locale       domain.Locale
	Client       string
	At           time.Time
}

// Template is the creation's seeder (M10 design 4.4): for an LLM Wiki it
// writes its mode's template in its language, the source root with the
// creation's id, and answers the id of the source root it wrote; for a
// standard notebook it writes nothing and answers the zero id.
type Template struct {
	seeds  Seeds
	parsed map[domain.Locale][]SeedPage
}

// NewTemplate parses the templates once, at start-up (v0.1 design 13.3
// item 3's exception): an error is the server's fault, a template the
// page module refuses.
func NewTemplate(seeds Seeds) (*Template, error)

// NotebookCreated writes c's template: the zero id and nothing for a
// standard notebook; for an LLM Wiki its pages in one unit, the source
// root's id the one c decided, the changeset's message the template's in
// its language, "LLM Wiki template" or "LLM Wiki 模板", and answers the id
// the seed wrote the source root with, the zero id when it wrote none:
// the notebook module's check that it is the notebook's own then means
// written. A mode or a language there is no contract of, or a seed that
// answers another count of pages, is a defect: the notebook module
// checked both, and only a conversion of the composition root's would
// answer one.
func (t *Template) NotebookCreated(ctx context.Context, c Creation) (uuid.UUID, error)
```

- 一个 `Seed` 是一个单元（00-M10 4.4"一次调用一个单元、一个变更集"）；`raw` 的 `ID` 是 `c.SourceRootID`，`AGENTS` 的是零（单元给它一个 V7，在 `raw` 之后）。两页都在根下（`ParentID` 为空），各在兄弟的最后（`Position` 的零值），所以根下是 `raw`、`AGENTS`。
- 不认识的模式（`ContractOf` 答 `false`）或语言（`Templates[l]` 没有）是缺陷：`fmt.Errorf`，500，整个回滚。notebook 的 `CheckDraft` 已核对过两者，这一条只防组合根转换写错。
- `NotebookCreated` 交回 `Seed` 写下标着 `SourceRoot` 的那一页的 id，没有写下时是零，不是交来的 `c.SourceRootID`；`Seed` 交回的 id 的个数不等于页数是缺陷（`fmt.Errorf`，500）。建的事务（3.4 第 10 步）由此核对写下的来源根：组合根若把 `raw` 以别的 id 写下，建答 500、整个回滚（S1 审查 A-N3，主持者的决定 23）。
- 变更集的说明是 `wiki/domain` 的 `TemplateMessage(locale)`（"LLM Wiki template"、"LLM Wiki 模板"），与模板放在一起；`contract_test` 钉住每种语言的不为空、不认识的语言的为空（S1 的偏离 2）。
- `Parsed` 是不透明的：wiki 不读 page 的提取结果，只把它交回（同 `page.ImportedPage` 带着 `Parsed` 走，`page/tree_writes.go:143-146`）。
- `app/template_test.go`：假的 `Seeds` 记下调用，照 page 模块交回页的 id（零 id 的给一个新的）。`TestTheTemplateSeedsAnLLMWiki`：`standard` 不调用 `Seed`、答零；`llm_wiki` 调用一次、带两页、`raw` 的 id 是值里的、说明随语言、`At`、`Client` 是值里的，答来源根的 id；`NewTemplate` 解析四页，三次创建之后仍是四次（"在启动时解析一次"的 wiki 的一半，S1 审查 A-M2）。`TestTheTemplateAnswersTheSourceRootWritten`：答 `Seed` 写下的 id，不是交来的；两页只交回一个 id 是缺陷（审查 A-N3）。`TestTheTemplateFails`：`Seed` 的错误原样答出；`NewTemplate` 在 `Parse` 拒绝一页时、不认识的模式或语言答错误。

**模块根**（`wiki/module.go`）：

```go
// Template is the creation's seeder of the LLM Wiki mode (M10 design 4.4).
type Template = app.Template

// Seeds, Seed, SeedPage, Parsed and Creation are its port and values,
// which the composition root fills from the page and notebook modules.
type (
	Seeds    = app.Seeds
	Seed     = app.Seed
	SeedPage = app.SeedPage
	Parsed   = app.Parsed
	Creation = app.Creation
	Mode     = domain.Mode
	Locale   = domain.Locale
	// TemplatePage is a page a mode's creation writes: its title, its
	// content, and whether it is the source root.
	TemplatePage = domain.TemplatePage
)

// The mode and the languages the composition root names: an alias carries
// no constants, and bootstrap may not import wiki/domain.
const (
	ModeLLMWiki = domain.ModeLLMWiki
	LocaleEn    = domain.LocaleEn
	LocaleZhCN  = domain.LocaleZhCN
)

// NewTemplate is the seeder over seeds, its templates parsed now.
func NewTemplate(seeds Seeds) (*Template, error) { return app.NewTemplate(seeds) }

// TemplatePages are the pages mode's creation writes in locale l, in
// order, as the contract holds them: none for a standard notebook, a mode
// or language there is none of. The composition root's tests compare a
// created notebook with them (M10/P1 design 3.16).
func TemplatePages(mode Mode, l Locale) []TemplatePage {
	c, _ := domain.ContractOf(mode)
	return slices.Clone(c.Templates[l])
}

// Actions are the wiki module's actions in the rule table: none in P1;
// P3 adds lint.read (M10 design 4.17).
func Actions() []shared.Action { return nil }
```

`module.go` 因此导入 `slices`。三个常量是给组合根的测试的（`wiki.TemplatePages(wiki.ModeLLMWiki, wiki.LocaleZhCN)`；类型别名不带常量，审查 AC-N1）。`TemplatePages` 是组合根的测试读模板的唯一的入口：`TestLLMWikiCreationWritesItsTemplate` 一类拿建出来的两页的标题与正文与它比（3.16），不另抄一份正文，也不从 `wiki/domain` 导入（组合根只经模块根，`archtest`）。它交回副本，调用方改不了契约表。

`bootstrap/actions_test.go:21` 的并集加 `wiki.Actions()`（P1 是空的，P3 的 `lint.read` 落进来时这一行不再改）。模块根的替身测试 `wiki/template_test.go` 的 `TestTheTemplateReachesItsSeeds`：以一个记录的 `Seeds` 建 `NewTemplate`，经模块根的 `NotebookCreated` 到达 `Seed`（13.1 第 21 条"模块根的替身测试"）。

**组合根**（`bootstrap/wiki.go`，新）：

```go
// wikiSeeds are the page module's template writes as the wiki module's
// template calls them: the two modules do not import each other, so their
// values, alike field by field, meet here.
type wikiSeeds struct {
	tree page.TreeWrites
}

func (s wikiSeeds) Seed(ctx context.Context, x wiki.Seed) ([]uuid.UUID, error) {
	pages := make([]page.PageDraft, len(x.Pages))
	for i, p := range x.Pages {
		pages[i] = page.PageDraft{ID: p.ID, Title: p.Title, Content: p.Content, Facts: p.Parsed.Facts}
	}
	return s.tree.Seed(ctx, page.SeedSpec{NotebookID: x.NotebookID, At: x.At, Client: page.Client(x.Client),
		Message: &x.Message, Pages: pages})
}

func (s wikiSeeds) Parse(content string) (wiki.Parsed, error) {
	facts, err := s.tree.ParseTemplate(content)
	return wiki.Parsed{Facts: facts}, err
}

// wikiTemplate is the wiki module's template as the notebook module's
// seeder: a notebook.NotebookCreation converted, the mode and language
// by their values.
type wikiTemplate struct {
	template *wiki.Template
}

func (t wikiTemplate) NotebookCreated(ctx context.Context, c notebook.NotebookCreation) (uuid.UUID, error) {
	return t.template.NotebookCreated(ctx, wiki.Creation{NotebookID: c.NotebookID, Mode: wiki.Mode(c.Mode),
		SourceRootID: c.SourceRootID, Locale: wiki.Locale(c.Locale), Client: c.Client, At: c.At})
}
```

（`PageDraft.Facts` 是 `app.Facts`，即 `type Facts any`（`page/app/unit_create.go:16-25`、`ports.go:763-766`），`any` 的值可以直接赋给它，不要类型断言；page 的模块根加别名 `Facts = app.Facts`。）

`bootstrap/wire.go` 的次序（今天 `:144-174`）：

```
authorizer := access.New(…)                       // 今天 :144-147
ws := workspace.New(…)                            // :148
contentKey, embeds := …                           // :153-154
md, budget, err := parsing(…)                     // :155-158
pd := pageDeps(cfg, pool, logger, authorizer, md, budget)             // :159
if w.page != nil { w.page(&pd) }                  // :160-162，照旧：测试改 page 的依赖的钩子
pg := page.New(pd)                                // :163
as := asset.New(…)                                // :164
ln := linking.New(…)                              // :165
template, err := wiki.NewTemplate(wikiSeeds{pg.TreeWrites()})         // 新：模板在这里解析，错误让 serve 起不来
inserter, err := jobs.NewInserter(pool, logger)   // :168-171
trDeps := transferDeps(…)                         // :172
trDeps.Batch = w.batch                            // :173，照旧
tr := transfer.New(trDeps)                        // :174
nb := notebook.New(notebookDeps(cfg, pool, logger, authorizer,
	[]notebook.Seeder{wikiTemplate{template}}))   // 从 :149 挪到这里
```

- `nb` 只在 `:205`（`Register`）与 `:211`（`mcpTools`）用到，`pg`、`as`、`ln`、`tr` 都不要它（它们读笔记本经 `notebook.NewNotebooks(pool)` 一类只凭连接池的端口），所以往后挪不改别的。00-M10 4.1 的"page → asset → wiki → transfer → notebook"照此，`linking` 在 asset 与 wiki 之间（P1 的 wiki 不要它；P3 起的读经只凭连接池的端口）。
- `notebookDeps` 加一个参数 `seeders []notebook.Seeder` 放进 `Deps.Seeders`，另放 `WritingJobs: transfer.NewWritingJobs(pool)`（3.10）。播种者不进 `notebookRegistrants(pool)`：命令行的 `Users`、`Workspaces` 经 `workspaceRegistrants` 走到它（`bootstrap/registrants.go:34-38`、`:76-78`、`:187-188`），那里不许调用模块根的 `New`（`archtest/composition_test.go:100-117`），而播种者要 `page.New` 的 `TreeWrites()`。
- `NewTemplate` 在启动时失败（模板解析不过）是 serve 的错误，照 `parsing` 的写法交回（`:155-158`）。

### 3.9 S1：模板的全文与它们的测试

下面是四页的全文，即 `wiki/domain/template-*.md` 的内容（每个文件以一个换行结尾）。两种语言一块一块对应（3.9 末的测试）；行内代码在每一块里两种语言相同；代码之外没有 `[[`、`](`、`#标签`。`AGENTS` 讲到的来源区的规则、去重、生成的两页与三个 prompt 是 P2–P5 做成的契约：模板在 P1 就写下 M10 建成之后的约定，`AGENTS` 本来就由用户改写（00-M10 4.4）；P1 与 P5 之间建的 LLM Wiki 只在开发的分支上（v0.1 在 M10 之后才发布）。

**`template-en-raw.md`**

```markdown
# Sources

This page is the root of the notebook's source area. Put the original material here: articles, papers, meeting notes, PDFs and images, one source to a page or an attachment. A source is written once and kept as it is, so that the wiki's pages can cite it.

On the web, add one with “Paste or upload as source”. An agent adds one with a single `write` of `raw/<title>.md`.

The pages and attachments under this page cannot be changed once written. Only the notebook's admins can rename, move or delete them; anyone who can write may add more.

This page's own text is yours to change: say what your sources are and how you name them.
```

**`template-zh-CN-raw.md`**

```markdown
# 来源

这一页是这本笔记本的来源区的根。原始资料放在这里：文章、论文、会议记录、PDF 与图片，一个来源一页或一个附件。来源写进来之后保持原样，wiki 的页才能引用它。

在网页上用“粘贴或上传为来源”添加。agent 用一次 `write` 写 `raw/<title>.md` 添加。

这一页之下的页与附件写进来之后不能修改。改名、移动、删除只有这本笔记本的管理员能做；能写的人都可以继续添加。

这一页自己的正文由你改：写明你的来源是什么、怎样命名。
```

**`template-en-agents.md`**

````markdown
# How this LLM Wiki is kept

This notebook is an LLM Wiki: people add sources, and agents write and keep the wiki's pages from them. This page is the agreement both follow. An agent reads it when it connects, before it writes anything.

## Pages

Every page outside the source area is a wiki page about one thing: a concept, a person, an event, the summary of a source. Give it frontmatter with three keys:

```yaml
---
type: concept
summary: One sentence that says what the page is about.
sources: ["[[raw/Some article]]"]
---
```

- `type` puts the page in a group of the index. Use few types, and the same words each time, such as `concept`, `person`, `event` and `summary`.
- `summary` is one sentence. The index shows it next to the title.
- `sources` lists the sources the page draws on, each a link in quotes. Without the quotes, `sources: [[raw/Some article]]` is a nested list in YAML, not a link, and cites nothing.

Where pages mention each other, link them with `[[Title]]`, so that no page is left without a link to it.

## Sources

The page `raw` and everything under it is the source area. A source is added once, as a page or an attachment, and never changed after: to correct one, add a new source. Only the notebook's admins can rename, move or delete sources. A new source that is byte for byte one already there is refused, with the path of the one there.

## Generated pages

At the root, `index.md` and `log.md` are not pages anyone writes: the server writes them each time they are read. `index.md` lists the wiki's pages by type with their summaries; `log.md` lists the latest changes. No page or attachment at the root may be named `index` or `log`.

## Working with an agent

Connect an agent to this notebook at `/mcp?notebook=<id>`. Three prompts are served there:

- `ingest` reads a source, then writes or updates the pages it touches, in one change.
- `query` answers a question from the wiki's pages, citing them and their sources.
- `lint` checks the wiki and fixes what can be fixed without asking.

## This page

While the notebook is an LLM Wiki, this page cannot be deleted, renamed or moved. Its text is yours: rewrite it for your field, your types and the way you work.
````

**`template-zh-CN-agents.md`**

````markdown
# 这本 LLM Wiki 的约定

这本笔记本是一本 LLM Wiki：人添加来源，agent 据此写出并维护 wiki 的页。这一页是双方都遵守的约定。agent 连上时先读它，再动手写。

## 页面

来源区以外的每一页都是讲一件事的 wiki 页：一个概念、一个人、一件事、一个来源的摘要。给它带三个键的 frontmatter：

```yaml
---
type: concept
summary: 一句话说这一页讲什么。
sources: ["[[raw/Some article]]"]
---
```

- `type` 决定这一页在目录里归在哪一组。类型少用几种，每次用同样的词，如 `concept`、`person`、`event`、`summary`。
- `summary` 是一句话，目录把它显示在标题旁边。
- `sources` 列出这一页取材的来源，每项是加引号的链接。不加引号的 `sources: [[raw/Some article]]` 在 YAML 里是嵌套的列表，不是链接，什么也没有引用。

页与页互相提到的地方用 `[[Title]]` 链接，不让哪一页没有链接指向它。

## 来源

`raw` 这一页与它之下的一切是来源区。来源以一页或一个附件添加一次，之后不再修改：要更正一个来源，就添加一个新的。只有这本笔记本的管理员能改名、移动、删除来源。与已有的一个逐字节相同的新来源会被拒绝，并给出已有的那一个的路径。

## 生成的页

根下的 `index.md` 与 `log.md` 不是谁写的页：服务器在每次读它们时写出。`index.md` 按类型列出 wiki 的页与它们的摘要；`log.md` 列出最近的改动。根下的页与附件不能叫 `index` 或 `log`。

## 与 agent 一起工作

在 `/mcp?notebook=<id>` 把 agent 连到这本笔记本。那里有三个 prompt：

- `ingest` 读一个来源，再写出或更新它涉及的页，一次改完。
- `query` 用 wiki 的页回答一个问题，引用这些页与它们的来源。
- `lint` 检查这本 wiki，能直接修的就修。

## 这一页

这本笔记本是 LLM Wiki 时，这一页不能删除、改名或移动。它的正文归你：按你的领域、你的类型与你的工作方式改写它。
````

- 大小（上面的全文，以一个换行结尾，UTF-8）：英文的 `AGENTS` 2,209 字节，中文的 2,107，英文的 `raw` 648，中文的 593，四页共约 5.4 KiB，远在 8 KiB 之下；测试钉住 8 KiB（`MaxTemplateBytes`，00-M10 4.18 的 `AGENTS` 至多 8 KiB；测试写出 `8 << 10`，不取这个常量），实现时 `wc -c` 量得的正是这四个数（共 5,557 字节，10.1）。
- 两页都不带 frontmatter（第一行是标题）：说明的找法去掉 frontmatter（`page/domain/instructions.go:35-36`），lint 对两页豁免类型与摘要（4.10），带了反而让人以为要写。`yaml` 围栏里的 `---` 不在正文的开头，不是 frontmatter（`tools/md-fixtures/cases/019-frontmatter-not-at-start.md`）。
- "粘贴或上传为来源"/“Paste or upload as source” 是 P3 之后网页的那个对话框的名字（00-M10 4.16）；文字由网页那一步定，定下的不同时改这里的一句（第 9 节写给 P3–P4 的）。
- 代码里的链接不算：行内代码与围栏里的 `[[…]]` 与 `#…` 不产生链接与标签（`tools/md-fixtures/cases/003-code-excluded.md`），所以新建的 LLM Wiki 的 `page_links`、`page_tags` 里两页一行也没有，lint 干净（4.11）。

**`wiki/domain/template_test.go`**（纯函数，没有库）：

- `TestTheTemplatesCorrespond`：每种模式、两种语言的每一页，按"围栏之外的空行"切成块；每块的种类（标题与它的级、围栏与它的信息串与行数、列表与它的项数、段落）两种语言逐块相同，块数相同；每块的行内代码（反引号之间的文字）作为多重集合相同。反向对照（13.4 第 1 条），每一种以测试里改过的中文的 `AGENTS` 跑同一个比较函数，不改文件：块数由"a paragraph gone"（去掉"## 生成的页"之后的那一段）与"one more at the end"（末尾多一段）钉住，只有后一种离了块数的比较就过不了：去掉它时循环照英文的块走，中文多出的一块看不见，少一块则在第一处错开的块失败（S1 的偏离 14）；行内代码由"a list item gone"（删掉的一项带着 `` `lint` ``，所以它不钉住项数）与"an inline code changed"（一处 `` `log.md` `` 改成 `` `log` ``）钉住；每种形状各一行、都保留行内代码：列表的项数"two items on one line"、标题的级"a heading a level down"、围栏的行数"a fence a line short"、段落的行数"a paragraph in two lines"（S1 审查 A-M1）。
- `TestTheTemplatesLinkNothingOutsideCode`：去掉围栏与行内代码之后，没有 `[[`、`![`、`](` 与 `#` 后接字母或数字（标签的起头，`tools/md-fixtures/cases/014-tags-basic.md` 的规则）；反向对照：在正文里加一句 `see [[raw/x]]`、一个 `#ingest` 各让它失败。
- `TestTheTemplatesAreSmall`：每页至多 `MaxTemplateBytes` = 8 KiB，且是合法的 UTF-8、没有 NUL、以换行结尾、第一行以 `# ` 开头（不是 frontmatter）。反向对照以写出的界线（`const bound = 8 << 10`，不取 `MaxTemplateBytes`：取它时把常量提高测试照绿，S1 审查 C-N3）：恰 8 KiB 的一页照收，8,193 字节的与以 `---` 起头的各被拒。
- 整个程序里另有一道核对（3.16）：建好的两页的 `page_links`、`page_tags` 是零行，`open_notebook` 的 `instructions` 等于 `AGENTS` 的正文去掉首尾空白（`instructions.go:35-36`）。

### 3.10 S1：降级

**用例**（`notebook/app/downgrade_notebook.go`，新；照 `update_notebook.go` 的形状：找、锁、核对、写、订阅者、数成员、日志）：

```go
// DowngradeNotebookDeps: the manager's (Tx, Clock, Logger, Authorizer,
// Workspaces, the repository), the writing jobs and the mode subscribers.

// Execute makes the LLM Wiki id a standard notebook (M10 design 4.5), in
// the order of a notebook's admin write (v0.1 design 13.1 item 5):
//
//  1. the notebook, unlocked: notebook.not_found for none, a deleted one,
//     or the zero id;
//  2. in one transaction: its workspace's row FOR SHARE, its row FOR NO KEY
//     UPDATE, the decision of notebook.change_mode (404 for a caller who
//     cannot see it, 403 for one who is no admin);
//  3. notebook.not_llm_wiki (409) for a standard notebook, another admin's
//     downgrade first among them;
//  4. transfer.busy (409) when a job that writes its tree, an import, a
//     trash restore or a revert, is queued or running: one read;
//  5. the clock, once; the row standard, its source root NULL, by the
//     caller at that time; [M10/P2/S2: the source area's hashes forgotten,
//     page.NewSourceArea(pool).Forget, here];
//  6. the mode subscribers, in order, in the transaction;
//  7. the member count, for the answer; the commit; a log line.
func (d *DowngradeNotebook) Execute(ctx context.Context, id uuid.UUID) (View, error)
```

- 第 2 步是 `manager.lock(ctx, id, domain.ActionChangeMode)`（`notebook/app/manage.go:26-41`：工作区行 `FOR SHARE` → `LockNotebook` → 判定），与改名、改开放程度、删除同一个函数，所以码的次序与它们一致（404 → 403），等锁期间删了的笔记本答 404。
- 第 3 步在锁下读模式（`LockNotebook` 带两列，3.3），所以两个管理员同时降级时后到的那个答 409 `notebook.not_llm_wiki`，不会写两次、不会发两个事件。第 1 步的不加锁的一读只为 404 的次序，用例不看它的模式：`TestDowngradeNotebookRefuses` 的一行让假的仓库在 `FindNotebook` 答 `llm_wiki`、在 `LockNotebook` 答 `standard`（另一个管理员的降级在两者之间提交；假的仓库为此加一张 `locked`，`LockNotebook` 先读它，3.1），期待 409、没有 `Writing`、没有 `SetMode`。
- 第 4 步经 `WritingJobs`（新端口，`app/ports.go`）：

  ```go
  // WritingJobs tells whether a notebook has a job that writes its tree,
  // an import, a trash restore or a revert, queued or running, as the
  // transfer module keeps them (M10 design 4.5): in the caller's
  // transaction. A downgrade between two of a job's batches would have the
  // job write half under one contract and half under the other.
  type WritingJobs interface {
  	Writing(ctx context.Context, notebookID uuid.UUID) (bool, error)
  }
  ```

  transfer 的模块根（`transfer/writing_jobs.go`，新）照 `transfer.NewNotebookDeletion(pool)` 的写法（`transfer/lifecycle.go:14-26`）：`func NewWritingJobs(pool *pgxpool.Pool) *WritingJobs`，`Writing` 调 `postgresadapter.New(pool)` 的 `Writing`，即开始导入用的那一条（`transfer/adapter/postgres/queries/jobs.sql:28-35`：这本笔记本的 `import`、`trash_restore`、`revert` 而 `queued` 或 `running`、没有删除的行，谁的都算，走 `transfer_jobs_writing_key` 的唯一索引，`00039_transfer_background_jobs.sql:26-27`）；它经 `postgres.DB(ctx, pool)` 读，所以在降级的事务里（`platform/postgres/tx.go:36-44`）。方法的签名与 notebook 的端口相同，组合根直接把它放进 `Deps.WritingJobs`，不要转换。导出不算（它们有自己的、按账户的 `Exporting`，`jobs.sql:20-26`）。
- **它看不到的**：一次正在上传的导入与一次正在开始的任务只是进程里的占位（`transfer/app/start_import.go:82-86`、`start_background.go:125-136`），还没有行，降级照样成功。这是安全的：任务的行在一个以 `FOR SHARE` 锁笔记本行的事务里写下（`transfer/app/start.go:103-113`、`start_import.go:248-280`、`start_background.go:158-178`），与降级的 `FOR NO KEY UPDATE` 串行，降级之后才写下的任务从头到尾按普通的契约走；之前写下的让降级答 409。00-M10 4.5 没写这一句，2.1 第 9 条补上。
- 第 5 步：`domain.Notebook.Downgraded(now)` 给出新的值（`Mode: standard`、`SourceRootID` 为零、`UpdatedAt: now`），`NotebookWriter.SetMode(ctx, n, by)` 经 `SetNotebookMode` 写（`source_root_id` 写 `NULL`）。P2/S2 的 `Forget` 插在这一步与第 6 步之间（00-M10 4.5），P1 的用例在那里留一句注释，不留空的端口。
- 第 6 步：`publishMode(ctx, d.subscribers, ModeChange{WorkspaceID, NotebookID, From: llm_wiki, To: standard, By, At: now})`，第一个错误回滚。
- 不写变更集（没有页面的写），没有审计行（`notebook_audit_events` 只记归属的事，`00011_notebook_notebook_audit_events.sql:15-16`）。
- **日志**：info 一行 "notebook downgraded"，`workspace_id`、`notebook_id`、`user_id`，不带名称（13.1 第 10 条；`update_notebook.go` 的 "notebook updated" 同样带这三个）。`notebook/app/mode_test.go` 的 `TestDowngradeNotebookLogs` 核对这一行与它的三个值，并核对没有名称（照建的那一行的核对，它在 `TestCreateNotebookMakesTheCallerItsAdmin` 里：没有 `TestCreateNotebookLogs`，S1 的偏离 1、审查 A）。

**`notebook/app/mode_test.go`**（假的仓库，照 `write_test.go`）：

- `TestDowngradeNotebookMakesItStandard`：调用的次序 `["FindNotebook", "ShareByID in tx", "LockNotebook in tx", "Authorize notebook.change_mode in tx", "Writing in tx", "SetMode standard by <alice> in tx", "ModeChanged a llm_wiki standard in tx", "CountMembers in tx"]`（照已有的替身的写法：notebook 的替身不记 id；要紧的 id 在别处钉住：授权者的目标、写下的 `moded`、假的任务按 id 答，S1 的偏离 6）；答复的 `Mode` 是 `standard`、`SourceRootID` 为零、`UpdatedAt` 是时钟的唯一一次读、等于 `ModeChange.At`。
- `TestDowngradeNotebookRefuses`（表格，照 `TestCreateNotebookRefuses`）：零 id、没有、删了的 → 404；看不到的 → 404；写者 → 403；管理员而普通的 → 409 `notebook.not_llm_wiki`；管理员而有写入的任务 → 409 `transfer.busy`；次序：写者在普通的笔记本上答 403（不是 409）、管理员在有任务的普通笔记本上答 409 `notebook.not_llm_wiki`（不是 `transfer.busy`）；找到时是 `llm_wiki`、锁下读到 `standard` → 409 `notebook.not_llm_wiki`（两个管理员同时降级的后一个，在模块一级；7.3 的变异"降级看锁之前读的模式"）；每一种都没有 `SetMode`、没有订阅者的调用。
- `TestADowngradesSubscriberFailureRollsItBack`：订阅者的错误原样答出，`SetMode` 在事务里（回滚由真库的模块根测试核对，3.5）。

**HTTP 适配器**（`notebook/adapter/http/handler.go`）：`DowngradeNotebook` 调用例、答 200 `notebookOf(v)`；端口 `DowngradeNotebookUseCase`。`handler_test.go` 的表格加四行：404 `notebook.not_found`、403 `forbidden`、409 `notebook.not_llm_wiki`、409 `transfer.busy`，经 `CheckResponse` 答出（`apitest.Main` 要模块自己的测试答出它声明的每个码，`apitest/problems.go:171-228`）。

### 3.11 S1：模式的事件

- **发布者**（`bootstrap/events_registrants.go`）：`notebookEvents` 加一个方法，它因此也是模式变化的订阅者：

  ```go
  // ModeChanged publishes a notebook's mode change, the type mode (M10
  // design 4.5): its frame is the notebook's ids, which the stream puts in
  // the data itself (events/domain/frame.go:38-57), so its own data is
  // empty. Every stream that sees the notebook gets it.
  func (n notebookEvents) ModeChanged(ctx context.Context, m notebook.ModeChange) error {
  	return n.publisher.Publish(ctx, events.Event{Type: "mode", WorkspaceID: m.WorkspaceID, NotebookID: m.NotebookID,
  		Data: json.RawMessage(`{}`)})
  }
  ```

  类型是写在发布处的字符串，同 `links`（`bootstrap/linking.go:388`）；events 模块没有类型的登记（`events/domain/event.go:22-28` 只拒绝空的、`hello`、`reset` 与带 `\r`、`\n`、`:` 的），不改它，也不改它的测试（它们用编造的类型 `later`，`events/app/publisher_test.go:199-219`、`hub_test.go:103-137`）。数据要是一个 JSON 对象：`Decode` 核对它以 `{` 开头（`events/domain/payload.go:63`），`EventFrame` 把它解成对象（`frame.go:42-45`）；7999 字节的上限对的是整个载荷，不只是数据（`payload.go:12`、`:50`）。`{}` 三样都照收。
- **登记**（`bootstrap/registrants.go`）：`notebookExtensions` 加 `modeSubscribers []notebook.ModeSubscriber`，`notebookRegistrants(pool)` 给 `[streams]`（同 `visibilitySubscribers`，`:196`；`events.NewPublisher()` 不是模块根的 `New`，命令行的路径照收）；`notebookDeps` 放进 `Deps.ModeSubscribers`。`notebookRegistrants` 的注释加"M10's event stream follows a mode change (M10/P1)"。
- **帧**：`{"workspace_id": …, "notebook_id": …}`（00-M10 4.5 的"载荷"说的是帧；发布的数据是 `{}`，2.1 第 10 条写明）。按笔记本过滤：看得到这本笔记本的每一条流都收到（`events/app/stream.go:89`、`:120-127`），读者也收到（降级改变的是读者看到的东西）。
- **最后一跳**：3.16 的 `TestTheModeEventReachesTheStreams`，组合根不登记 `streams` 时失败。

### 3.12 S1：page 读模式（`StateOf`）与 `open_notebook` 的 `mode`

`Write.Notebook` 是 S2 的（3.17）；S1 只要一处不加锁的读：`open_notebook` 答 `mode`。`Opened` 与 `NotebookGate` 今天都不读模式：门经 `WorkspaceOf` 读笔记本的行（`page/app/gate.go:35` → `decideNotebook`；`notebook/notebooks.go:48-54`），只取它的工作区（`page/app/open_notebook.go:30-38`、`page/app/gate.go:17-36`）。

- **page 的领域**（`page/domain/mode.go`，新）：

  ```go
  // Mode is a notebook's mode, as the page module knows it (M10 design
  // 4.3): the composition root converts the notebook module's.
  type Mode string

  const (
  	ModeStandard Mode = "standard"
  	ModeLLMWiki  Mode = "llm_wiki"
  )

  // NotebookState is a notebook's mode and source root, the zero id for
  // none: what a write reads of its notebook under the notebook's row lock
  // (Write.Notebook, M10/P1/S2), and an unlocked read reads alone.
  type NotebookState struct {
  	Mode         Mode
  	SourceRootID uuid.UUID
  }
  ```

  S1 只用 `Mode`；`NotebookState` 在 S1 定下，是因为端口的 `StateOf` 交回它（S2 的 `ShareByID`、`LockByID` 交回同一个类型）。
- **端口**（`page/app/ports.go:54-63` 的 `Notebooks`）加：

  ```go
  	// StateOf is the state of the notebook not deleted with id, unlocked,
  	// and whether there is one: a read that holds no lock sees one side of
  	// a downgrade or the other (M10 design 4.3).
  	StateOf(ctx context.Context, id uuid.UUID) (domain.NotebookState, bool, error)
  ```

  组合根今天把 `notebook.NewNotebooks(pool)` 直接交给 page（`bootstrap/deps.go:153`）；`StateOf` 交回的类型不同（notebook 的 `State`、page 的 `NotebookState`），所以加一个适配 `bootstrap/page_notebooks.go` 的 `pageNotebooks{notebook.NewNotebooks(pool)}`：`WorkspaceOf`、`ShareByID`、`LockByID` 照转，`StateOf` 逐字段转换（`domain.Mode(s.Mode)`）；`TestPageNotebooksConvertTheState`（`bootstrap/page_notebooks_test.go`，不要库：替身嵌着 `notebook.Notebooks`，只答 `StateOf`）钉住它：LLM Wiki 与普通的逐字段转换，找不到答 `ok` 为假与零值，错误原样（S1 审查 C-N4：去掉找不到的一支时 `open_notebook` 会答 `"mode": ""`）。S2 把 `ShareByID`、`LockByID` 也换成交回状态的（3.17）。page 的三个假的 `Notebooks`（`page/app/fakes_test.go:103-124`、`page/extension_test.go:67-82` 的 `sqlNotebooks`、`page/adapter/postgres/paths_property_test.go:28-33`；`git grep "LockByID("` 恰找到这三个，最后一个的方法参数没有名字，带 `ctx context.Context` 的写法找不到它；`search_test.go:194` 的 `searchWorkspace` 是工作区的端口的假的，不是这一个，审查 AC-N6）加 `StateOf`；`sqlNotebooks` 以 SQL 读两列。
- **用例**：`OpenNotebook` 在门之后读 `StateOf`（门已答过 404；门与这一读之间删了的，`ok` 为假，答门答的同一个 `domain.ErrNotebookNotFound`：MCP 的 `toolError` 认作 `notebook.not_found`，即 REST 的 404），`Opened` 加 `Mode domain.Mode`。次序（M9/P1 3.17.3）因此是：执行者与笔记本、模式、计数、最后的改动、根下的节点、说明。`NewOpenNotebook` 收 `d.Notebooks`（`page/module.go` 里 `m.open` 那一行）。
- **MCP 的答复**（`page/adapter/mcp/open_notebook.go:33-39`）：`openAnswer` 在 `notebook` 之后加 `Mode string \`json:"mode"\``；骨架的量法（`:66-80`）照旧先量不带说明与节点的答复，多出的十几个字节随之计入。`openNotebookDescription`（`:18-24`）与 `tools.json` 照 3.13 改。
- **测试**：`page/app/open_notebook_test.go` 的次序测试加 `"StateOf nb"` 一项（门之后、计数之前），`ok` 为假时答 `notebook.not_found`；`page/adapter/mcp/open_notebook_test.go` 的答复带 `mode`，`TestTheOpenNotebookAnswer` 另核对适配器原样交回 `ErrNotebookNotFound`（S1 审查 C-N4）；整个程序的 `TestOpenNotebookOfAnEmptyNotebook`（`bootstrap/mcp_account_test.go:19-30`，整个答复的 `DeepEqual`）加 `"mode": "standard"`，`mcp_instructions_test.go:188` 的骨架加 `mode`。

**MCP 的 `notebooks`**（notebook 的工具）：`notebookItem` 在 `role` 之后加 `Mode string \`json:"mode"\``，取 `n.Notebook.Mode`（`ListNotebooks` 已选出它，3.3）。`notebook/adapter/mcp/notebooks_test.go:47-53` 的字面 JSON 加 `"mode":"standard"`；`TestNotebooksListsTheAccountsWorkspaces`（`bootstrap/mcp_account_test.go:36-54`）的项加 `Mode`；矩阵的 `accountNotebooksOf`（`permission_matrix_mcp_notebooks_test.go:35-53`）的 `accountNotebook` 加 `mode`：它是手写的（`listedNotebooks(c)` 给出 `lab` 的，`callerOutsideWorkspace` 另加 `other-nb`），`wlab` 的笔记本由变体落下的那个提交手写加进去（3.14、6.1），这个提交只加 `mode`，所以这一行也钉住 `llm_wiki`。e2e 的 MC1 以 `toEqual` 比较整个列表（`e2e/stories/mcp/mc1-connect.spec.ts:89-99`），加 `mode: "standard"`（第 5 节）。

### 3.13 S1：契约

**`api/modules/notebook.yaml`**

`Notebook`（`:421-451`）的 `required` 加 `mode`、`source_root_id`，`properties` 在 `workspace_access` 之后加：

```yaml
        mode:
          $ref: "#/components/schemas/NotebookMode"
        source_root_id:
          description: >-
            An LLM Wiki's source root: the page its template wrote, under
            which its sources are. Null for a standard notebook.
          type: [string, "null"]
          format: uuid
```

`NotebookCreate`（`:461-470`）的 `description` 改为 "A new notebook; its workspace access is none, its mode standard and its template's language en when absent."，`properties` 加：

```yaml
        mode:
          $ref: "#/components/schemas/NotebookMode"
        template_locale:
          description: >-
            The language of an LLM Wiki's template, the client's: its two
            pages are written in it, to be rewritten by their users after.
            Checked whatever the mode; a standard notebook writes nothing.
          type: string
          enum: [zh-CN, en]
```

新的两个枚举放在 `WorkspaceAccess`（`:413-420`）之后：

```yaml
    NotebookMode:
      description: >-
        A notebook's mode. standard is a notebook as it always was; llm_wiki
        is an LLM Wiki: its source area holds sources written once, its root
        page AGENTS and source root cannot be deleted, index and log at its
        root are generated, and its agents are served the prompts ingest,
        query and lint. A downgrade makes an LLM Wiki standard; an upgrade's
        states are no mode.
      type: string
      enum: [standard, llm_wiki]
```

`createNotebook` 的 `description`（`:38-42`）末尾加一句："An LLM Wiki is created with its template, the pages raw, its source root, and AGENTS, in one change; the mode and the template's language are checked with the other values."（原来的最后一句"the values are checked after both"已说在 404 与 403 之后；S1 审查 B-N2 = C-N6 去掉了草稿这一句里的重复）；`x-problem-codes` 不变（422 已在其中）。`NotebookUpdate` 不变：它是关着的，`PATCH {"mode": …}` 由结构检查答 400 `bad_request`、字段码 `not_allowed`（`bodyshape.go:65`、`:146-157`），改模式只经降级（3.16 的一个核对）。

新的路径（放在 `/api/v0/notebooks/{notebook_id}/leave` 之后）：

```yaml
  /api/v0/notebooks/{notebook_id}/downgrade:
    parameters:
      - $ref: "#/components/parameters/NotebookID"
    post:
      operationId: downgradeNotebook
      tags: [notebook]
      summary: Downgrade an LLM Wiki
      description: >-
        Makes the LLM Wiki a standard notebook. Its pages stay as they are,
        raw and AGENTS among them, which anyone who can write may then
        rename, move or delete; it has no source root, no generated pages
        and no prompts after. No page is written. Its admins can (forbidden).
        A notebook that does not exist, is deleted, or that the caller cannot
        see is notebook.not_found; a standard one is notebook.not_llm_wiki;
        one with an import, a restore or an undo queued or running is
        transfer.busy, to downgrade once it has ended. An upgrade, the way
        back, is another process.
      security: [{ bearer: [] }]
      x-problem-codes: [notebook.not_found, forbidden, notebook.not_llm_wiki, transfer.busy]
      responses:
        "200":
          description: The notebook, now standard.
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Notebook"
        default:
          $ref: "#/components/responses/Problem"
```

`api/openapi.yaml` 的路径表（`:80-97`）在 `leave` 那一行之后加 `/api/v0/notebooks/{notebook_id}/downgrade: $ref: "modules/notebook.yaml#/paths/~1api~1v0~1notebooks~1{notebook_id}~1downgrade"`（`TestRootListsEveryModulePath`，`apitest/rules_test.go:93-104`）。`make gen` 重新生成 notebook 的 `server.gen.go`、`bodyshape.gen.go`、`api/dist/openapi.yaml` 与网页的类型（`gen-check-go` 在成本的一份，`gen-check-web` 在网页的一份）。新的可选成员由 `bootstrap/contract_test.go:113` 的请求体的派生情形自动覆盖（`apitest/operations.go:189-300` 的"null for optional mode"答 400 `invalid_format`，由结构检查答出）。

**`api/modules/events.yaml`**：帧的列表（`:21-55`）在 `links` 之后加一项：

```yaml
        - mode: a notebook's mode changed, an LLM Wiki downgraded to a
          standard notebook (and, later, a notebook upgraded): its data is
          the notebook's ids. Read the notebook again, its mode and the
          caller's role in it with it.
```

`oneOf`（`:77-82`）加 `- $ref: "#/components/schemas/EventMode"`，`schemas` 加（照 `EventLock` 的形状，`:157-174`）：

```yaml
    EventMode:
      description: The data of mode, a change of a notebook's mode.
      type: object
      additionalProperties: false
      required: [workspace_id, notebook_id]
      properties:
        workspace_id:
          type: string
          format: uuid
        notebook_id:
          type: string
          format: uuid
```

进了 `oneOf` 才进打包的文件（打包的文件里每个事件的结构都由它够到，`api/dist/openapi.yaml:1917-1922`），`CheckSchema(t, "EventMode", …)` 才找得到（3.16）。生成的 TypeScript 里有 `EventMode`；`web/…/services/event.service.ts` 导出它在 S4，与它的处理同一个提交（4.1：S1 导出了没有人用，`make knip` 报出未用的导出类型，`ci.yml:156-157`；M6 的 `EventLinks` 也是在网页的 Step 里与处理一起导出的，`52ac9870`）。

**`api/mcp/tools.json`**（`account` 里的两个对象，`:913-947`；Go 的常量 `notebooksDescription`、`openNotebookDescription` 同一个提交改成同样的字，`TestTheToolListsAreTheContract` 比较整个对象，`bootstrap/mcp_tools_test.go:109-110`）：

- `notebooks` 的 `description`："List the notebooks you can see in every workspace you belong to, each with its id, its name, its workspace's slug and name, your role (admin, editor or reader) and its mode (standard, or llm_wiki for an LLM Wiki), by workspace, then by name. Open one with open_notebook, then pass its id as notebook to the other tools. truncated is true when there were more than one answer holds."
- `open_notebook` 的 `description`："Open a notebook: its mode (standard, or llm_wiki for an LLM Wiki), its instructions for agents (its root page AGENTS without its frontmatter, or the server's built-in ones when it has none) and an overview: how many pages and attachments it holds, when it last changed (null when nothing was ever written in it), and the nodes at its root. Follow its instructions while you work in it, and pass its id as notebook to the other tools. truncated is true when the root's nodes were cut short."

两个工具的 `inputSchema`、`annotations` 与 `mcpToolCodes()` 的行不变。

### 3.14 S1：日志、配置、`checkPages`、权限矩阵

- **日志**：建的 "notebook created" 加 `mode`（取值，不含个人信息，13.1 第 10 条；`TestCreateNotebookMakesTheCallerItsAdmin` 的两行（普通的、LLM Wiki）跟着核对，同已有的"不带名称"：建的那一行一直在这个测试里核对，没有 `TestCreateNotebookLogs`，S1 的偏离 1）；降级的 "notebook downgraded"（3.10）。模板的单元不另记（单元今天不记日志）。
- **配置**：P1 不加配置。
- **`checkPages`**（`bootstrap/interleavings_page_test.go:44-111`）加一条规则：

  ```sql
  -- "live LLM Wikis without their source root or root AGENTS page"
  SELECT count(*) FROM notebooks n
  WHERE n.mode = 'llm_wiki' AND n.deleted_at IS NULL AND (
      NOT EXISTS (SELECT 1 FROM nodes r WHERE r.id = n.source_root_id AND r.notebook_id = n.id
          AND r.kind = 'page' AND r.deleted_at IS NULL)
      OR NOT EXISTS (SELECT 1 FROM nodes a WHERE a.notebook_id = n.id AND a.parent_id IS NULL
          AND a.name_key = 'agents' AND a.kind = 'page' AND a.deleted_at IS NULL))
  ```

  （`name_key` 是 `shared.TitleKey`，`AGENTS` 的是 `agents`。）今天的 `checkPages` 没有反例的办法：它收 `*testing.T`、每条规则一个 `t.Errorf`（`:105-109`），没有一处测试拿一个违反的种子核对它报出来。所以 S1 先把它拆成 `pageViolations(t testing.TB, pool) []string`（照 `matrixViolations`，`bootstrap/permission_matrix_coverage_test.go:21-100`：交回每条非零的规则的一句）与 `checkPages`（对它 `t.Errorf`），再加 `TestPageViolationsCatchEachLLMWikiGap`（在 `interleavings_page_test.go`，`pageViolations` 旁边）：一个新的程序经一次 `POST` 建一本 LLM Wiki（种子由应用写，别的规则要的正文、版本、条目、变更集都齐），再以 SQL 改它，每一行之后改回，再断言违反为空（一行的改回漏不到下一行，S1 的偏离 4）：

  - 没改 → 违反为空（13.4 第 1 条的反向对照）；
  - 只把 `raw` 的 `nodes.deleted_at` 设上 → 新规则的一句在违反里（这样的软删除同时违反"deleted, with what follows it not deleted"与回收站的根的一条，`interleavings_page_test.go:63-66`、`:71-74`，所以测试只要求新的一句**在其中**，不要求只有它）；
  - 只把 `AGENTS` 的 `deleted_at` 设上 → 在其中；
  - 把 `raw` 改成附件、把 `AGENTS` 改成附件（`UPDATE nodes SET kind = 'asset'`，`nodes_kind_check` 收它，`00012_page_nodes.sql:12`），各一行 → 在其中（规则的两处 `kind = 'page'`，审查 AC-N5）；
  - 把 `AGENTS` 移到 `raw` 之下（`parent_id = raw`，以 `parent_id = NULL` 改回）→ 在其中（规则的 `a.parent_id IS NULL`：S2 的守卫不许 `AGENTS` 离开根下，`checkPages` 是它后面的网，S1 审查 B-M1 = C-M1）；
  - `source_root_id` 指别的笔记本的一页 → 在其中；
  - 同一本 `raw` 删着而改成 `standard`（`mode`、`source_root_id` 一起改）→ 新规则的一句**不在**违反里；
  - 最后一行（之后不再改回）：经 `DELETE /api/v0/notebooks/{id}` 把这本 LLM Wiki 连同它的页删掉 → 违反为空（`n.deleted_at IS NULL` 那一半；删除由应用做）。

  这条测试在第二份（`TestPage…`；7.5 的表）。
- **权限矩阵**（`bootstrap/permission_matrix_*`）：
  - `downgradeNotebook` 的普通的一行（`permission_matrix_notebook_test.go`，照 `updateNotebook`，`:126-142`）：`write: true`，`POST /api/v0/notebooks/{id}/downgrade`，`cells: adminsOnly(cell{409, "notebook.not_llm_wiki"})`：普通的目标上管理员答 409，其余照 `adminsOnly` 答 403 或 404。
  - **LLM Wiki 的变体**（00-M10 4.17）：今天每一行的目标都在 `lab` 里，`targetViolation`（`permission_matrix_coverage_test.go:125-150`）要求每个 `{…_id}` 的种子在 `workspaceOf(c)` 里，而它对每个笔记本的列都答 `lab`（`permission_matrix_test.go:127-136`）。所以 S1 改三处：`matrixRow` 加 `wiki bool`（"它的目标是变体的种子"）；`targetViolation` 取这一行的列所在的工作区的 slug（`workspaceOfIn(c, r)`，`r.wiki` 时答变体的工作区 `wlab`），`workspaceOf(c)` 留给各行自己的请求（变体的行从 `s.wiki` 建路径；S1 的偏离 9）；`prepareMatrix`（`permission_matrix_seeded_test.go:576-750`）以 `seedWikiVariant`（`permission_matrix_wiki_test.go`）在 `wlab` 里播下同样的成员（每个在 `lab` 里的列在 `wlab` 里有同样的工作区角色与同样的结束状态）、同样名字与开放程度的几本笔记本（删掉的那一本同样删掉：删一本种下的笔记本分成 `deleteNotebook`，`lab` 与 `wlab` 共用）与同样的笔记本成员，每本 `mode = 'llm_wiki'`、`source_root_id` 是它的 `raw`，根下有 `raw`、`AGENTS`（带正文、版本与变更集，照种子的页的写法 `:636`；正文是英文的模板的两页 `wiki.TemplatePages`，不是矩阵的标记：`lab` 的搜索的行找不到它们，页也正是一本 LLM Wiki 里有的，偏离 10）。变体由它自己的表种下（`wikiMemberships`、`wikiNotebooks`、`wikiNotebookMembers`，都由 `lab` 的派生，两边不会走样），种进它自己的 id（`seeded` 的 `wiki`），不加进 `matrixWorkspaces()`、`matrixMemberships()`、`matrixNotebooks()`：`prepareMatrix` 的别的用者遍历它们（`TestTheSearchIsWhatEachReadAllows` 搜每个 `matrixWorkspaces()`，`events_lab_test` 给每个 `matrixNotebooks()` 发布），`lab` 与 `wlab` 的笔记本又同名、`seeded.notebooks` 按名称取（偏离 7）；`s.wiki.notebook(name)`、`s.wiki.raw(name)`、`s.wiki.agents(name)` 给出 id，`workspaceOfRow` 先认它们。另两个读一列的每个工作区的测试也读 `wlab`，否则在第 5 步失败：`TestTheStreamSeesWhatEachReadAllows` 与 `TestTheAccountsNotebooksAreWhatEachReadAllows`（`notebook_visibility_test.go`）读 `seededNotebooks(s)`（`lab`、`other` 与 `wlab` 的），事件流的那一个的工作区的循环读 `seededWorkspaces(s)`（偏离 8）。`checkPages` 在种子上跑（含 3.14 的新规则）。`TestMatrixViolationsCatchesEachGap`（`:160-…`）加一个反例：一行 `wiki: true` 而目标是 `lab` 的笔记本，报出来；`TestThePermissionMatrixCoversEveryOperation` 在每一行上跑同一个核对，`workspaceOfIn` 照旧只认 `lab` 时它把变体的行报出（第 22 条变异）。
  - 变体上的行：S1 一行 `downgradeNotebook, LLM Wiki`（`wiki: true`、`variant: "LLM Wiki"`、`cells: adminsOnly(cellOK())`，`check` 核对答复的 `mode` 是 `standard`、`source_root_id` 为 `null`，`after` 再 `GET` 这本笔记本、核对同样的两列：答复之外也看行）；S2 的几行见 3.22。`listNotebooks` 的期望不变（它按工作区列，变体在 `wlab`）。MCP 的 `notebooks` 的期望是手写的 `accountNotebooksOf`（`permission_matrix_mcp_notebooks_test.go:35-53`：`lab` 的取 `listedNotebooks(c)`，`callerOutsideWorkspace` 另加 `other-nb`），而 MCP 的矩阵用同一个 `prepareMatrix`（`permission_matrix_mcp_test.go:544-600`）：播下 `wlab` 的那个提交就给它手写加上每一列在 `wlab` 里看得到的笔记本（slug `wlab`），不然每个笔记本的列的 `slices.Equal` 在那个提交就失败；MCP 的 `mode` 落下的提交再给每项加 `mode`（3.12、6.1）。00-M10 4.17 说两者的期望都不变，后一句不成立，2.1 第 11 条写明。
  - MCP 的矩阵每个工具恰一行（`mcpMatrixViolations`，`permission_matrix_mcp_test.go:382-427`），P1 不给 MCP 的矩阵加变体的行：P1 的码在 MCP 上经 `TestTheMCPToolsErrorsCarryTheirCodes` 的调用答出（3.22）；变体的 MCP 的行随 P3 的 `lint`、`pending_sources`（它们的那一行本就在变体上）一起加，P3 设计那时定写法（第 9 节）。

### 3.15 S1：建的交错

三条，都在 `bootstrap/interleavings_wiki_test.go`（新），照 Interleaving 15（`interleavings_notebook_test.go:57-90`）：`tm.interleave(t, first, second)` 以测试自己的事务持着 acme 的工作区行 `FOR NO KEY UPDATE`，依次发出两步、每步等到它在行锁上等着再发下一步（`interleavings_workspace_test.go:150-200`），然后放开，两步按到达的次序拿到锁。建的一步是 `POST /api/v0/workspaces/acme/notebooks` `{"name":"Field notes","mode":"llm_wiki","template_locale":"en"}`，它的事务先取工作区行 `FOR SHARE`（3.4 第 2 步），所以排在持锁的事务之后。每条以 `checkPages`、`checkNotebooks` 结束。测试名 `TestWorkspaceChangesAndCreatingAnLLMWiki`（第三份），三条是它的子测试（S1 的修复一轮另加第四个，下面）：

1. **工作区的删除**：删除在先 → 建答 404 `workspace.not_found`，没有笔记本、节点、变更集；建在先 → 201，删除随后成功，笔记本与它的两页一起删除（`page.NewNotebookDeletion` 的 `DeleteNotebooksPages`，不经单元），`checkPages` 的新规则只看活着的笔记本，照过。
2. **建的人的成员身份结束**（管理员移出 bob，bob 建）：移出在先 → 建答 404；建在先 → 201，移出随后成功，bob 的笔记本成员身份随之结束，这本 LLM Wiki 成为无主的（M3/P3 的规则照旧），两页还在、`checkPages` 照过。
3. **建的人的角色改变**（bob 由成员降为访客）：降级在先 → 建答 403 `forbidden`（访客不能建，`notebook.create` 给工作区的管理员与成员），什么也没写；建在先 → 201，降级随后成功，bob 仍是这本笔记本的管理员（笔记本的成员身份不随工作区的角色变）。

每个子测试核对答复的码与之后的行（笔记本、成员、节点、变更集的数目）。为什么是工作区行：建的事务里能与成员身份冲突的只有它（建的人的判定在它的 `FOR SHARE` 之下读成员关系，成员关系的每一种变化持它的 `FOR NO KEY UPDATE`，13.1 第 5 条）；模板的单元重入同一把锁（3.4 第 9 步），不多出一个可以交错的点。

第四个子测试"the template's unit behind a queued deletion"钉住这一句（S1 审查 A-N4，主持者的决定 24）：测试库里 `notebook_members` 上的触发器等测试持着的 advisory lock，建停在管理员的插入之后、持着 acme 的行 `FOR SHARE`（`pgtest.WaitForAdvisoryLockWaits`）；工作区的删除发出、在 `workspaces` 上等（`WaitForLockWaitsOn`），然后放开：模板的单元在排着的、更强的等待者之后重入 acme 的行，建 201，删除随后 204，这本 LLM Wiki 的行随 acme 删掉，`checkPages`、`checkNotebooks`。它钉住的是 PostgreSQL：已持的锁交给唯一的持有者，不让它等在等待者之后；我们的代码没有一处改动让它失败（修复核对 A 的 h7：page 的单元在删除排着时以 `FOR NO KEY UPDATE` 升级 acme 的行，照样通过），所以它是平台的回归测试，不是我们的锁序的检查。移出与降级在 acme 的行上排的是同一个 `FOR NO KEY UPDATE`，不另写。竞态下约 0.5 秒。

降级的两条交错（与开始导入、与 `AGENTS` 的删除）分别在 3.16 与 3.23。

### 3.16 S1：整个程序的测试

| 测试（文件） | 份 | 核对 |
|---|---|---|
| `TestLLMWikiCreationWritesItsTemplate`（`bootstrap/wiki_template_test.go`） | 二 | 一个 PAT 建 `template_locale: zh-CN` 的、一个会话建 `en` 的、一个会话建不带 `template_locale` 的（得 `en`）三本 LLM Wiki，与一本普通的：答复的 `mode`、`source_root_id`；根下恰是 `raw`、`AGENTS`，`raw` 的 id 是 `source_root_id`；两页的标题与正文逐字节是那种语言的模板（测试经 `wiki.TemplatePages(wiki.ModeLLMWiki, l)` 读，3.8：bootstrap 只导入模块根）；一个变更集（经 `coalesce` 读：没有变更集或没有说明时在比较处失败、说出是什么，S1 审查 A-N1 = C-N2），种类 `edit`、客户端是 `api`/`web`、执行者是建的人、说明是那种语言的、时刻等于笔记本的 `created_at`，两项、两个版本；`page_links`、`page_tags` 里两页零行，`indexed_pages` 里两行（链接索引对模板的两页的最后一跳；它不钉住事实是正文的，那是 3.6 的 `TestATemplatesParseIsItsContents`：S1 的偏离 3，理由照审查 A 改正）；搜索的队列里有两页（准确的入队）；普通的那本没有节点、没有变更集、`source_root_id` 为 `null`；`checkPages` |
| 同上 | 二 | `template_locale: "fr"`、`mode: "LLM_WIKI"` 各答 422，字段码 `invalid_format`，字段 `template_locale`、`mode`；一个访客带坏的 `mode` 答 403（次序：值的核对在 403 之后，13.1 第 4 条）；`PATCH {"mode":"llm_wiki"}` 答 400 `not_allowed`；都没有写下笔记本 |
| `TestTheTemplateRollsBackWithItsNotebook`（同上） | 三 | 测试库里建一个序列 `seen_nodes` 与一个 `BEFORE INSERT ON nodes` 的触发器：每次插入先 `nextval('seen_nodes')`，`NEW.name = 'AGENTS'` 时 `RAISE EXCEPTION 'the test refused AGENTS'`（照 `cmd/nervewiki/reindex_test.go:132-139` 的 `refuse_queue`，经 `tm.pool.Exec`，库随测试删掉）。建 LLM Wiki 答 500 `internal_error`；记下的 error 一行（`newAcmeTeamLogging`，`interleavings_workspace_test.go:70-112`）带 `the test refused AGENTS`（照 `reindex_test.go:144` 核对触发器自己的话：拦下的是 `AGENTS`，不是别的失败）；`SELECT last_value, is_called FROM seen_nodes` 是 `(2, true)`（序列不随回滚退回：`raw` 写过、`AGENTS` 被拦，回滚收回的是单元已写下的一行。不读 `currval`：它只认本会话调过的 `nextval`，触发器在应用的连接上调、测试读在 `tm.pool` 上，读就报 55000；默认的 `CACHE 1` 下每个会话读到的 `last_value` 相同，审查 AC-M1）；之后没有笔记本、笔记本成员、节点、正文、版本、变更集、搜索的队列的行；同一个库里建普通的笔记本照常 201 |
| `TestTheModeEventReachesTheStreams`（`bootstrap/wiki_downgrade_test.go`） | 三 | 先 `settle`（`events_stream_test.go:112-132`：之前的通知都到了 hub，之后开的流收不到它们），这本 LLM Wiki 是私有的，再由它的管理员 alice、加作它的读者的 bob 与看不到它的 acme 的成员 carol 各开一条流（`openStream`，`events_stream_test.go:17-173`；这个形状分得出按笔记本的过滤与按工作区的，S1 的偏离 12），降级，再在三人都看得到的另一本笔记本（`Eng`，对 acme 的成员开放）里写一页作标记（`quiet` 的办法，`events_pages_test.go:67-76`）：前两条恰收到一个 `mode`，帧经 `apitest.Load(t).CheckSchema(t, "EventMode", …)`（照 `links_test.go:113-131`），是这本笔记本的两个 id，下一帧是标记；第三条的下一帧就是标记。组合根不登记 `streams` 为模式的订阅者时失败（最后一跳） |
| `TestTheDowngradeAnswersInItsOrder`（同上） | 二 | 整个程序里降级的码：普通的笔记本 409 `notebook.not_llm_wiki`；有排着队的导入（`tm.holdImportWorker`，`interleavings_import_test.go:174`，让导入停在 `queued`）的 LLM Wiki 409 `transfer.busy`，放开、导入结束之后 200；200 的答复与 `getNotebook` 相同；日志的一行 "notebook downgraded"（`newAcmeTeamLogging`，`interleavings_workspace_test.go:70-112`） |
| `TestImportsAndDowngradingAnLLMWiki`（`bootstrap/interleavings_wiki_test.go`） | 二 | 交错：`tm.interleaveOn(t, notebookRow(nb), …)` 持着这本笔记本的行（`interleavings_trash_test.go:28`）。开始导入在先 → 导入 202、降级 409 `transfer.busy`；降级在先 → 降级 200、导入 202，任务按普通的契约跑完（`succeeded`）。开始导入的事务以 `FOR SHARE` 锁笔记本行（`transfer/app/start.go:103-113`），降级以 `FOR NO KEY UPDATE`，所以两者按到达的次序串行。两种次序都持住导入的唯一的 worker（`holdImportWorker`）：否则"开始在先"里 worker 可能在开始的提交与降级的读之间把任务做完，那是测试没有逼出的次序；之后放开 worker，等导入以 `succeeded` 结束（S1 的偏离 11） |
| `TestNotebooksAnswerTheirModeOverMCP`（`bootstrap/mcp_account_test.go`） | 二 | MCP 的 `notebooks` 每项带 `mode`；`open_notebook` 对 LLM Wiki 答 `llm_wiki`、`instructions` 是 `AGENTS` 的正文（`instructions_from` 是 `AGENTS.md`）；降级之后的下一次 `open_notebook` 与 `notebooks` 都答 `standard`（`notebooks` 在降级之前与之后经同一个帮手读，S1 审查 B-N1） |

两个管理员同时降级不另写整个程序的交错（C 的过度设计，主持者照收）：后一个在锁下读到 `standard` 而答 409 由 `TestDowngradeNotebookRefuses` 的一行核对（找到时 `llm_wiki`、锁下 `standard`，3.10），降级只发一个 `mode` 由 `TestTheModeEventReachesTheStreams`（`mode` 之后下一帧是标记）核对；00-M10 4.17 与第 9 节都没有要它。

`TestTheTemplateRollsBackWithItsNotebook` 也是"一个什么也没写的播种者不会提交一本坏的笔记本"的最后一跳的一半；另一半（播种者交回的 id 不对时 500）在 notebook 的模块根测试里（3.5），整个程序里组合根只有一个播种者、不会答错的 id。"同一时刻"在模块一级（3.5 的 `tickingClock`、3.6 的 `steppingClock`）：整个程序里每个依赖都是 `clock.System{}`（`bootstrap/deps.go:43` 等），`wiring` 没有时钟（`bootstrap/wire.go:58-75`），为这一条加一个时钟的接缝不值：整个程序里只核对"变更集、节点、正文、版本的时刻都等于笔记本的 `created_at`"（同一次读的结果），这一条在真的时钟下也能分出两次读（两次读之间至少差一微秒的精度时，PostgreSQL 的 `timestamptz` 存到微秒，`time.Now()` 在 macOS 与 Linux 上都到纳秒）。

### 3.17 S2 的文件

| 包 | 文件 | 新或改 | 内容 |
|---|---|---|---|
| notebook | `adapter/postgres/queries/notebooks.sql`、`store.go`、`gen/`；`notebooks.go`、`notebooks_test.go` | 改 | `ShareNotebook` 选出两列；根的 `ShareState`、`LockState`（3.18） |
| page | `domain/contract.go`、`domain/contract_test.go` | 新 | `StepFacts`、`NodeFacts`、`NodePlace`、`FactsOf`、`Verdict`、`CheckContract`、`ReservedKeys()`、`Reserved`（3.21；`page/domain` 已有 `Place`，lint 不许包级的变量，S2 的偏离 2、3） |
| page | `domain/batch_plan.go`、`domain/batch_plan_test.go` | 改 | `PlanBatch` 收笔记本的状态，树的一遍调 `CheckContract`，`Needs`、`Refuse`（3.22 的"`batch` 的计划"） |
| page | `domain/errors.go`、`domain/refused.go`、`domain/refused_test.go`、`domain/actions.go` | 改、新 | `ErrProtected`、`ErrVirtualFile`、`ErrSourceAdminOnly`；`Refused`、`Refuse` 与 `TestARefusalIsMarkedOnce`；`ActionManageSource`（3.19、3.22） |
| page | `app/extension.go`、`app/grants.go`、`app/grants_test.go` | 改、新 | `Write.Notebook`、`Write.Grants`；`Grants` 与 `TestAGrantIsAskedOnceAUnit`（3.18、3.19） |
| page | `app/ports.go`、`app/unit.go`、`app/unit_test.go` | 改 | `Notebooks.ShareByID`、`LockByID` 交回状态；`run` 把状态与备忘放进 `Write`；`apply` 标出守卫的拒绝与 `TestAUnitMarksItsGuardsRefusals`（3.18、3.22） |
| page | `app/mode_guard.go`、`app/mode_guard_test.go`；`mode_guard.go`、`refusal.go`、`refusal_test.go` | 新 | 守卫 `ModeGuard` 与它的 `TestTheModeGuardLetsAStandardNotebookBy`、`TestTheModeGuardMarksTheNodeItRefuses`（S2 的修复一轮加，审查 C-M1）、`TestTheModeGuardAsksOnceAUnit`（假的存储与授权者）；模块根的 `NewModeGuard`、`Refusal` 与 `TestRefusalReadsTheMark`（3.19、3.20、3.22） |
| page | `app/apply_batch.go`、`app/apply_batch_test.go`；`app/authorize.go` | 改 | `failed` 先认 `Refused`；计划的预检：不加锁读 `StateOf`、问 `page.manage_source`（3.22）；没有钉住整个调用列表的 `batch` 测试，`TestABatchsPlanRefusesBeforeItsLock` 的最后一行核对 `StateOf` 紧跟 `ListNodes`、不在事务里（S2 的偏离 11）；`decideNotebook` 包着新的 `decidedNotebook`，它另答笔记本的工作区，计划问 `page.manage_source` 用它（S2 的偏离 8） |
| page | `app/write_test.go` | 改 | `TestTheGuardSeesTheCheckedChange`（`:309-311`）以 `s.Write != want` 比较整个 `Write`：每个单元一个新的 `Grants` 指针让它失败，改为 `want.Grants = s.Grants` 之后再比（3.18） |
| page | `app/fakes_test.go`、`extension_test.go`、`adapter/postgres/paths_property_test.go` 等 | 改 | 假的 `Notebooks` 交回状态 |
| page | `adapter/http/handler_test.go`、`history_test.go` | 改 | 新码的行（3.22） |
| asset | `adapter/http/upload_test.go` | 改 | `page.virtual_file` 的行 |
| access | `domain/rules.go` | 改 | `page.manage_source` 一行（3.19） |
| bootstrap | `page_notebooks.go`、`page_notebooks_test.go`、`registrants.go`、`deps.go` | 改 | `ShareByID`、`LockByID` 经 `toPageState` 的转换，`TestPageNotebooksConvertTheState` 读三种读法、各记下它调的读；`pageRegistrants(pool, authorizer)`、`pageSessionSubscribers()`（3.20；不收连接池，S2 的偏离 5） |
| bootstrap | `wiki_guard_test.go`、`mcp_wiki_guard_test.go`、`wiki_downgrade_test.go`、`interleavings_wiki_test.go`（含 `TestTheGuardRefusesWhatThePlanLetBy`）、`interleavings_workspace_test.go`（收 `wiring` 的 `newAcmeTeamWiring`）、`permission_matrix_*`、`mcp_write_codes_test.go`、`mcp_reads_test.go`、`page_lock_test.go`（`pageRegistrants(pool, authorizer)`） | 新、改 | 第 7 节 |
| 契约 | `api/modules/page.yaml`、`api/modules/asset.yaml` | 改 | `x-problem-codes`（3.22） |
| 网页 | `app/problem-messages.ts`、`i18n/messages/{en,zh-CN}.ts` | 改 | 三个码的文字（4.2） |
| e2e | `stories/wiki/lw2-protected.spec.ts` | 新 | LW2 的 API、MCP（第 5 节） |

### 3.18 S2：`Write.Notebook`

- **notebook**：`ShareNotebook`（`notebooks.sql:22-28`）选出 `id, mode, source_root_id`（3.3 的写法；`store.go:81-86` 的 `ShareNotebook` 今天只答错误）。照实，存储的 `ShareNotebook` 答 `(domain.Mode, uuid.UUID, error)`，不答 `State`：适配器不能导入模块根的 `State`，根由这两个值建它；`LockState` 读 `LockNotebook` 已选的列（S2 的偏离 1）。根的 `Notebooks` 加 `ShareState`、`LockState`（3.3 的签名），`ShareByID`、`LockByID` 改为调用它们、丢掉状态，所以 transfer 与 linking 的端口（`transfer/app/ports.go:30-40`、`linking/app/reindex.go:36-43`）不动。`notebook/notebooks_test.go:21`、`:122-131` 的表格加两个新方法（事务之外报错、读到两列：一本 LLM Wiki、一本普通的、一本删掉的与没有的四行）。
- **page 的端口**（`page/app/ports.go:54-63`）：

  ```go
  	// ShareByID locks the notebook not deleted with id FOR SHARE until
  	// the transaction ends, and answers its state read under the lock, and
  	// whether there is one; LockByID FOR NO KEY UPDATE, for a write of the
  	// tree (v0.1 design 13.1 item 5). A unit reads its notebook's mode and
  	// source root here once (M10 design 4.6): a downgrade, which takes the
  	// row FOR NO KEY UPDATE, never lands inside a unit.
  	ShareByID(ctx context.Context, id uuid.UUID) (domain.NotebookState, bool, error)
  	LockByID(ctx context.Context, id uuid.UUID) (domain.NotebookState, bool, error)
  ```

  唯一的调用者是 `Writer.run`（今天 `unit.go:217-223`）；页的二十来个用例只调 `WorkspaceOf`（`page/module.go:269-346`），不受影响。组合根的 `pageNotebooks`（3.12）改为调 `ShareState`、`LockState` 并逐字段转换。
- **`Write`**（`page/app/extension.go:27-36`）加两格：

  ```go
  	// Notebook is the notebook's mode and source root, read once under
  	// the notebook's row lock as the unit began (M10 design 4.6): the
  	// mode guard's contract and its prechecks'.
  	Notebook domain.NotebookState
  	// Grants is the unit's memo of the actions it asked beyond its own
  	// decision (Grants): the unit's own, a pointer, since each step and
  	// each guard has the Write by value.
  	Grants *Grants
  ```

  `run`：`state, ok, err := lock(ctx, spec.NotebookID)`，`ok` 为假答 `spec.NotFound`（同今天）；`u.write.Notebook = state`、`u.write.Grants = &Grants{}`（`unit.go:228-232` 建 `Unit` 的那一处）。`Seed` 与 `Run` 都经 `run`，所以模板的单元读到的是 `llm_wiki` 与它的来源根（3.4 第 9 步，00-M10 4.4）。`Write` 仍可比较（指针可比），但 `page/app/write_test.go:309-311` 的 `TestTheGuardSeesTheCheckedChange` 以 `s.Write != want` 比较整个值，每个单元的新指针让它失败：S2 的第一个提交里改为先 `want.Grants = s.Grants` 再比（另核对 `s.Grants != nil`）。
- **假的**：`page/app/fakes_test.go:112-124` 的 `fakeNotebooks` 加 `states map[uuid.UUID]domain.NotebookState`（没有的是 `standard`），调用记下的名字照旧（`"ShareNotebook in tx"` 一类钉在约 17 个测试文件里，不改）；`page/extension_test.go:67-82` 的 `sqlNotebooks` 以 SQL 读两列；`paths_property_test.go:28-33` 答 `standard`。`TestAGuardRefusesTheUnit` 一类（`extension_test.go:304-337`）照旧。

### 3.19 S2：`page.manage_source` 与 `Write.Grants`

- **动作**：`page/domain/actions.go` 加 `ActionManageSource shared.Action = "page.manage_source"`，进 `Actions()`；规则表 `"page.manage_source": {Level: LevelNotebook, Notebook: notebookAdmins()}`，放在 `page.release_edit_lock`（`access/domain/rules.go:102-103`）之后，注释 "an LLM Wiki's source area's tree operations: the mode guard asks it (M10 design 4.7)"。`TestTheRuleTableIsTheModulesActions`（`bootstrap/actions_test.go:20-26`）由此要两边都有。它不是接口的操作：矩阵的覆盖检查不认它（`matrixViolations` 按操作），盖住它的是变体上 `raw` 的改名、移动的行（3.22；00-M10 4.17，审查 A2-N9）。
- **备忘**（`page/app/grants.go`）：

  ```go
  // Grants is a unit's memo of the actions asked beyond its own decision
  // (M10 design 4.6): the mode guard is one registrant that every unit
  // shares, which cannot keep an answer, so the unit keeps it. Ask asks
  // an action at most once a unit, the first time a step needs it; a step
  // of a standard notebook, or one that touches no source, never does.
  // Only the mode guard and the unit's own locked checks ask; any
  // extension may read the answers. It is read in the unit's transaction,
  // so it sees the members the unit's own decision saw. A precheck before
  // the unit asks on its own and keeps nothing here: the unit asks again
  // under its locks.
  type Grants struct {
  	allowed map[shared.Action]bool
  }

  // Ask is ask's answer for action, ask called the first time only: nil
  // for allowed, shared.Forbidden() for denied (ask answered it, or
  // shared.ErrNotVisible), both kept for the unit; any other error, the
  // database's, is answered as it is and kept not, since it rolls the unit
  // back.
  func (g *Grants) Ask(ctx context.Context, action shared.Action, ask func(ctx context.Context) error) error
  ```

  **为什么是指针**（审查 A2-N4、A3-N2）：`Step` 按值嵌着 `Write`（`page/app/extension.go:41-42`，`u.step` 照值复制 `u.write`，`unit.go:292-294`），守卫按值拿到 `Step`（`extension.go:142`、`unit.go:303-307`）；一个值的备忘在每一步的副本里填了又丢掉，守卫就每一步都问一次。指针让同一个单元的每一步看到同一份答案。`Grants{}` 的零值可用（`allowed` 在第一次 `Ask` 时建）。
- **测试**（`page/app/grants_test.go`）：`TestAGrantIsAskedOnceAUnit`：同一个 `Grants` 问两次 `page.manage_source`、`ask` 只被调用一次、两次答同样的；问两个动作各调用一次；`ask` 答 `shared.Forbidden()` 或 `ErrNotVisible` 时两次都答 `shared.Forbidden()`、只问一次（拒绝记下）；答一个数据库的错误时原样答出、下一次再问（没有记下，主持者对 B-O3 的决定）。照实，第 1 个提交的这个测试以 `node.move`、`node.rename` 作问的动作：`Grants` 不看动作，`page.manage_source`（动作、`Actions()` 与规则的一行）落在第 2 个提交，因为 `CheckContract` 的 `Needs` 要它，`TestTheRuleTableIsTheModulesActions` 又要两边在同一个提交（S2 的偏离 4）。
- **"一个单元问一次"数的是单元里的问**（主持者对 B-M2 的决定取第一种）：`batch` 的计划在单元之前、锁之外另问一次（3.22），它的答案不交给单元。交给单元就让一个在计划与单元之间被降为写者的执行者照计划时的权限写下去（`TestTheGuardRefusesWhatThePlanLetBy` 正是这个情形）；00-M10 4.6 要守卫的读"在单元的事务里"。所以 `page/app/mode_guard_test.go` 的 `TestTheModeGuardAsksOnceAUnit`（假的存储、事务与授权者，`fakes_test.go` 的 `recorder` 给事务里的调用记下 `" in tx"`，`:52-69`；守卫是真的 `ModeGuard`，交给 `fixture` 的 `guards`）：一个 `batch` 两次移动 `raw`（`apply_batch_test.go` 的写法；假的授权者放行 `batch` 自己的动作与 `page.manage_source`（`f.grant(domain.ActionBatch)` 之外再 `grant` 它），两次移动都写下。不放行时 `fakeAuthorizer` 答 `ErrNotVisible`、`Grants.Ask` 把它换成 `Forbidden()`，守卫在第一次移动就拒绝、单元停下，指针与值都只问一次，测试分不出它们，审查 B3-N1），记下的调用里 `"Authorize page.manage_source in tx"` 恰一次；分开的两次移动（两个单元）两次。`Write.Grants` 是值时（第 32 条变异）步按值带着 `Write`，每一步都问，记下两次，测试失败（审查 B 的第三条轻的意见：守卫在 `page/app`，不必真库）。事务外的问是计划的（S2 的第 4 个提交起恰一次），这个测试不数它们，`TestABatchsPlanRefusesBeforeItsLock` 数（3.22）。照实（S2 的偏离 12）：第 4 个提交起计划在任何单元之前拒绝被拒的 `batch`，所以拒绝的一行改为计划放行、锁下拒绝（记录器在 `"LockNotebook in tx"` 时把放行改成拒绝，等于其间角色变了），仍是守卫的拒绝，核对单元里问一次、计划问一次；替身授权者对没放行的动作默认答 `ErrNotVisible`（第 3 个提交里守卫的 `Grants` 把它当拒绝，第 4 个提交起计划把它答成笔记本的 404），另加 `failing`（数据库的错误的一行）。审查 C：这比设计的写法好，分得出单元的问与计划的问（抓到 c04 与第 40 条），第 32 条照旧由第一行抓到。

### 3.20 S2：模式的守卫与它的登记

```go
// ModeGuard keeps a notebook's mode's contract in each of its writes
// (M10 design 4.6): a standard notebook's steps it lets by at once,
// reading nothing; an LLM Wiki's it judges by CheckContract on the step's
// facts, and asks page.manage_source, once a unit, of a step that needs
// it. Its refusals are marked (Refused) with the node they are about.
type ModeGuard struct {
	auth shared.Authorizer
}

// NewModeGuard is the guard over the composition root's authorizer, the
// one the units decide with (v0.1 design 13.1 item 21's exception: a
// registrant not built from the pool alone). M10/P2/S1 adds the pool, for
// the reads of the source area's rules.
func NewModeGuard(auth shared.Authorizer) *ModeGuard

func (g *ModeGuard) GuardWrite(ctx context.Context, s Step) error {
	if s.Notebook.Mode != domain.ModeLLMWiki {
		return nil
	}
	v := domain.CheckContract(s.Notebook, domain.FactsOf(s.Operation, s.Kind, s.Changes))
	switch {
	case v.Err != nil:
		return domain.Refuse(v.Err, v.NodeID)
	case v.Needs != "":
		err := s.Grants.Ask(ctx, v.Needs, g.decide(s, v.Needs))
		if isForbidden(err) {
			return domain.Refuse(domain.ErrSourceAdminOnly, v.NodeID)
		}
		return err
	}
	return nil
}
```

- `decide(s, action)` 以 `shared.RequireActor(ctx)`（单元的执行者是 ctx 的，`unit.go:197`）与 `shared.Target{WorkspaceID: s.WorkspaceID, NotebookID: s.NotebookID}` 调 `g.auth.Authorize`；看不到（不会发生：单元刚在同一个事务里判定过它看得到）由 `Grants.Ask` 换成 `shared.Forbidden()`（3.19），所以与 403 一样答 `page.source_admin_only`，守卫不另认 `ErrNotVisible`（审查 B2-N5）。别的错误（数据库）原样答出，单元回滚。
- **"普通的立刻放行"**：`Mode` 不是 `llm_wiki` 时第一行返回，不建事实、不问授权者。单去掉这一行在 P1 看不出（审查 B-M6）：`CheckContract` 自己对普通的笔记本答零，普通的笔记本又没有来源根，什么也不会问；所以这一行是等价的变异，不列。P1 的 `TestTheModeGuardLetsAStandardNotebookBy`（`app/mode_guard_test.go`）照主持者对 B-O2 的决定改为能失败的：一本普通的笔记本里交给守卫的，是 LLM Wiki 会拒的步（手写的 `Step`：删除根下叫 `AGENTS` 的页、排序它、根下新建 `index`、把一页改名为根下的 `index.md`），每一步答 `nil`；守卫与 `CheckContract` 都不看模式（7.3 的变异）时这几步被拒、测试失败。不核对"授权者一次也没被问"：普通的笔记本没有来源根，R4 从不要问，这一句在 P1 与 P2 都失败不了（审查 B 的第一条轻的意见）。P2/S1 的守卫读连接池（祖先与哈希）之后，"普通的笔记本什么也不读"才看得出：那时这个测试加一个计数的存储（第 9 节）。
- **标出的节点**（S2 的修复一轮加，审查 C-M1）：`TestTheModeGuardMarksTheNodeItRefuses`（在它旁边，假的授权者一律拒绝 `page.manage_source`）：删除 `Notes`（`raw` 在它之下）答以 `raw` 标出的 `page.protected`；把 `Notes` 移到 `Other` 之下（`raw` 在它之下）答以 `raw` 标出的 `page.source_admin_only`。这两种被拒的节点都不是步的第一个节点，守卫不标出时 `Unit.apply` 的后备以第一个节点 `Notes` 标出，也是错的（第 83、84 条变异）；S2 里生产的代码还不读这个节点（S3 的 `treeError` 与报告的 `refusal.node_id` 读），整个程序的测试看不出它。
- **构造**：P1 的守卫只要授权者；P2/S1 的来源区的规则要读祖先与正文的哈希时加连接池（00-M10 4.1 写的 `page.NewModeGuard(pool, authorizer)` 是 P2 之后的签名，P1 不收一个不用的参数，2.1 第 6 条）。`NewModeGuard(nil)` 不 panic：`bootstrap/trash_test.go:453`、`:472` 以 nil 的连接池与授权者调 `pageDeps`，守卫只在一步要问时才用授权者。
- **登记**（`bootstrap/registrants.go`）：

  ```go
  // pageRegistrants are … (as today); M10's mode guard comes first, before
  // the lock, so that a write both refuse answers the mode's code (v0.1
  // design 12.4, M10 design 4.6). Its authorizer is the composition
  // root's, which only serve has: the command line takes the edit
  // sessions' subscribers alone (pageSessionSubscribers).
  func pageRegistrants(pool *pgxpool.Pool, authorizer shared.Authorizer) pageExtensions {
  	lock := page.NewEditLock(pool, displayNames{identity.NewDirectory(pool)})
  	mode := page.NewModeGuard(authorizer)
  	…
  	return pageExtensions{
  		guards:             []page.WriteGuard{mode, lock},
  		…
  		sessionSubscribers: pageSessionSubscribers(),
  	}
  }

  // pageSessionSubscribers are the subscribers of the page module's edit
  // sessions: M5's event stream, whose publisher holds nothing but its
  // NOTIFY. A notebook's deletion ends its sessions through them, the
  // command line's among them.
  func pageSessionSubscribers() []page.EditSessionSubscriber
  ```

  `notebookRegistrants` 的 `page.NewNotebookDeletion(pool, pageRegistrants(pool).sessionSubscribers)`（`:188`）改为 `pageSessionSubscribers()`（不收连接池：它唯一的订阅者是事件流的发布者，只要它的 `NOTIFY`，`events.NewPublisher()`，收池是一个没用的参数，S2 的偏离 5）：命令行的 `Users`、`Workspaces` 经它走到这里（`archtest/composition_test.go:31-84`），而那里没有授权者，也不许建 `access.New`（模块根的 `New`，`:100-117`）。`pageDeps`（`bootstrap/deps.go:141-173`）本就收 `authorizer`，交给 `pageRegistrants(pool, authorizer)`。13.1 第 21 条的"两处都是 `pageRegistrants(pool).sessionSubscribers`"与 12.4 的 `pageRegistrants(pool)` 照此改（第 8 节）。
- **编辑会话的否决者**：P1 没有它的规则（00-M10 4.7：只有来源上不开会话，来源是 P2 的），一个永远放行的否决者的次序没有一个测试看得出，登记它只是未经测试的代码。所以否决者随它的第一条规则在 P2/S1 写、登记在锁的否决者之前、带它的次序测试（2.1 第 5 条）。
- **次序的测试**：
  - 模块一级照旧：`TestTheRegistrantsRunInTheirOrder`（`page/app/unit_test.go:266-302`）是替身，不改。
  - 整个程序：`TestModeComesBeforeTheLock`（3.24）：别人的会话持着 `AGENTS` 时，管理员删除它答 409 `page.protected`，不是 `page.locked`；降级之后同样的删除答 `page.locked`（两个守卫都在、次序是 `[mode, lock]`）。组合根把次序写反时第一条失败，漏登记模式的守卫时也失败。

### 3.21 S2：`CheckContract`

**事实**（`page/domain/contract.go`；守卫与预检（S2 的 `batch` 的计划、S3 的三处）用同一个构造，所以两者判的是同一份事实，13.4 第 13 条）：

```go
// StepFacts are what CheckContract judges of a step (M10 design 4.6): its
// operation, its unit's kind, and each node it changes, where the node was
// and where it is after. FactsOf builds them for the guard from a Step,
// and each precheck from the step it plans, so that both judge alike.
type StepFacts struct {
	Operation Operation
	Kind      ChangesetKind
	Nodes     []NodeFacts
}

// NodeFacts is a node of a step: Before nil for a creation, After nil for
// a deletion. M10/P2/S1 adds whether each is in the source area, with
// the line of ancestors it is read from; P2/S2 the content's hash and
// size, the rewrite's mark and the edit session.
type NodeFacts struct {
	ID     uuid.UUID
	Before *NodePlace
	After  *NodePlace
}

// NodePlace is where a node is: its parent, nil at the notebook's root,
// and its name's title key. (Place is an order among siblings.)
type NodePlace struct {
	ParentID *uuid.UUID
	Key      string
}

// FactsOf is a step's facts from its operation, its unit's kind and its
// changes, each TreeState's name by its title key, in the changes'
// order: the first is the node the step is about (the node moved, its
// descendants after it; the root of a deleted subtree; the node renamed).
func FactsOf(op Operation, kind ChangesetKind, changes []Change) StepFacts

// Verdict is CheckContract's answer: Err the step's refusal and NodeID the
// node it is about; or Needs an action the step needs beyond its unit's
// own decision, of NodeID, for the caller to ask; neither when the
// contract lets the step by.
type Verdict struct {
	Err    error
	NodeID uuid.UUID
	Needs  shared.Action
}

// ReservedKeys are the title keys no node may take at an LLM Wiki's root
// (M10 design 4.9): its generated index.md and log.md, by either name.
func ReservedKeys() []string { return []string{"index", "log", "index.md", "log.md"} }

// Reserved reports whether key is one of ReservedKeys.
func Reserved(key string) bool

// CheckContract judges a step of the notebook n by its facts (M10 design
// 4.6, 4.9): a standard notebook's at once, reading nothing; an LLM
// Wiki's by the rules below, the first one a node breaks answering, in
// the order of their codes: page.protected, page.virtual_file, then the
// need of page.manage_source.
func CheckContract(n NotebookState, s StepFacts) Verdict
```

- 照实（S2 的偏离 2、3）：事实的位置叫 `NodePlace`，不叫 `Place`：`page/domain` 已有 `Place`（兄弟间的次序，`order.go:13`）；`ReservedKeys()` 是函数，不是变量：lint 的 `gochecknoglobals`（同 `Actions()`）。守卫经 `FactsOf` 从步的 `Change` 建事实；`batch` 的计划没有 `Change`，在它的树的一遍里以 `nodePlace` 照 `FactsOf` 的写法（父节点与标题键）从模拟的树建同样的 `NodeFacts`（3.22），两者判的仍是同一份事实。
- `ReservedKeys` 的四个值是 `shared.TitleKey` 的结果（折叠大小写、NFC：`NFC(cases.Fold(NFC(s)))`，`shared/title_key.go:15-17`），`contract_test.go` 的 `TestTheReservedKeysAreTitleKeys` 钉住 `shared.TitleKey(k) == k`；`Index`、`LOG`、`Log.MD` 都折到它们（`[[index.md.md]]` 那一页的名字是 `index.md`，00-M10 4.9）。四个键是 ASCII，没有别的字符折到它们的字母（U+0130 折成 `i` 加 U+0307）；折叠不映射兼容的形式，全角的 `ｉｎｄｅｘ` 不保留，它也不是虚拟的链接（4.9 用同一个键）。S3 的加序号用同一个表。
- 步的第一个节点是它所关于的那个：移动的一步先列被移动的节点、再列它的每个后代（`unit_move.go:73-77`：后代的前后位置相同），删除的一步先列子树的根（`unit_delete.go:43-48`），改名的一步只有那个节点。R4 由此分得出"移动来源根自己"与"移动它的祖先"。
- `NodePlace` 没有种类：P1 的规则对页与附件一样（保留的标题对附件也保留，4.9），`AGENTS` 由"根下、键是 `agents`"认出：LLM Wiki 的根下的键唯一、跨页与附件（`nodes_notebook_id_name_key_root_key`，`page/adapter/postgres/store.go:27-28`），`checkPages` 守着活着的 LLM Wiki 有根下的 `AGENTS` 页（3.14），所以根下键是 `agents` 的节点就是它，不读 `AgentsPage`（`nodes.sql:84-95`）。也没有次序值：同一父节点下的排序是一步 `OpMove`（`unit_move.go:40-44`、`:76-82`），兄弟的重新编号不是步（`unit_place.go:132-153`："it moves no one"），所以"出现在一步 `OpMove` 里"就是移动或排序。
- **P1 不读祖先**：`AGENTS` 在根下（`Before.ParentID` 为空），来源根按 id 认（删除的一步列出整棵子树，`unit_delete.go:43-48`；移动的一步列出被移动的节点与它的每个后代，`unit_move.go:73-88`），保留的标题看 `After.ParentID` 为空。祖先的一线（`Ancestors`，`nodes.sql:129-141`）是 P2 的来源区的规则才要的：`NodeFacts` 的 `InArea` 与交出祖先的那几处读（`unit_create.go:36`、`unit_move.go:26`、`unit_relocate.go:51` 已读的一线，与改名、删除、上传、同父节点的 `Relocate`、撤销的放回补一次读）在 P2/S1 加（2.1 第 5 条）。

**规则**（只在 `n.Mode == ModeLLMWiki`；"出现"指 `NodeFacts` 里有它）：

| 编号 | 规则 | 码 |
|---|---|---|
| R1 | `AGENTS`（`Before` 在根下、键是 `agents`）出现在 `OpRename`、`OpMove`、`OpDelete` 里：改名、移动（含排序、移动加改名）、删除 | 409 `page.protected` |
| R2 | 来源根（`ID == n.SourceRootID`）出现在 `OpDelete` 里：删除它或它的祖先 | 409 `page.protected` |
| R3 | 一个节点的 `After` 在根下、键是保留的，而 `Before` 为空、或不在根下、或键不同，且步是 `OpCreate`（单元不是导入）、`OpRename` 或 `OpMove` | 409 `page.virtual_file` |
| R4 | 来源根是 `OpMove` 的第一个节点（移动、排序、移动加改名它自己）或 `OpRename` 的节点（改它的名）；或来源根在 `OpMove` 的其余节点里（被移动的节点的后代）**而第一个节点换了父节点**（`Before.ParentID` 与 `After.ParentID` 不同：一个祖先移到别的父节点下，来源跟着换位置）。同一父节点下排序它的祖先、改它的祖先的名都不要它：来源区没有换位置（00-M10 4.7 的理由，主持者对 B-Q1 的决定） | 要 `page.manage_source`，没有时 403 `page.source_admin_only` |

R3 的"单元不是导入"只在 S2 与 S3 之间用得上：S3 起导入在它的单元写之前就给根下的四个保留的键加序号（3.27），R3 没有可拒的。它留作规则的文字（导入不由 R3 拒绝，是加序号），`TestCheckContract` 的导入的一行是第 28 条变异唯一的抓手（审查 B2-N3 与 B 的第二条轻的意见）。

码的次序：R1、R2 → R3 → R4（13.1 第 4 条："守卫的 403 在领域的 422、409 之后"；P2 的 `page.source_immutable`、`page.source_duplicate` 排在 R3 与 R4 之间）。

**步 × 节点的表**（"—"是这样的步碰不到这样的节点；"放行"是 P1 不拒；P2 的格子写明 P2 才有的规则，P1 照放行）：

| 步 \ 节点 | `AGENTS` | 来源根 | 子树里有来源根的节点 | 落到根下而键是保留的 | 来源（P2） | 别的 |
|---|---|---|---|---|---|---|
| 新建页面（REST、MCP 的 `write`、`batch`；补建的父页） | —（根下已有 `agents`，`page.title_taken` 在先） | — | — | R3 | P2：去重 | 放行 |
| 导入的单元的新建（`Kind` 是导入） | —（导入给它加序号，`AGENTS 2`） | — | — | 放行（S3 加序号） | P2：去重 | 放行 |
| 上传附件 | — | — | — | R3（`index.md`、`log.md` 先被 `CheckAssetName` 以 422 拒绝，`page/domain/name.go:20-29`） | P2：去重 | 放行 |
| 写正文（PUT、自动保存、勾选、还原版本、MCP 的 `edit`、`write` 覆盖、`batch`） | 放行 | 放行 | — | — | P2：只许两种 | 放行 |
| 改名（REST、MCP 的同父节点的 `move`、`batch`） | R1 | R4 | 放行 | R3 | P2：管理员 | 放行 |
| 移动到别的父节点 | R1 | R4 | R4 | R3 | P2：移入新增、移出与在里面移动要管理员 | 放行 |
| 同一父节点下排序 | R1 | R4 | 放行（来源区没有换位置） | — | 同上 | 放行 |
| 移动加改名（`Relocate`，换父节点；同一父节点的是改名，`unit_relocate.go:48-50`） | R1 | R4 | R4 | R3 | 同上 | 放行 |
| 删除 | R1 | R2 | R2 | — | P2：子树里有来源要管理员 | 放行 |
| 从回收站恢复（同步、后台） | —（活着的 LLM Wiki 里它从不在回收站） | — | — | 放行（S3 加序号） | P2：去重 | 放行 |
| 撤销的放回（改名、移动、移动加改名、临时的改名） | R1（到不了：没有变更集碰过它的树） | R4 | 换父节点时 R4，同一父节点下的放回放行 | R3（到不了：根下从来没有保留的键，临时名称带 ` ~<hex>`，`page/domain/name.go:95-119`；S2 与 S3 之间到得了：恢复或导入放到根下的 `index` 改名之后，撤销这次改名答没有声明的 `page.virtual_file`，第 0 节，S2 审查 A-M2） | P2：照各操作 | 放行 |
| 撤销的删除与恢复 | R1 | R2 | R2 | 放行（恢复） | P2 | 放行 |

- 导入与恢复不拒保留的标题，是因为它们加序号（4.9："恢复、导入（加序号）"）；S2 与 S3 之间这两条路径照今天把一个 `index` 放到根下（S3 合并时补上加序号与 `checkPages` 的"活着的 LLM Wiki 的根下没有保留的键"，3.27），这一段只在 Step 的分支与 `main` 的两次合并之间，没有发布。若在 S2 里也拒它们，导入会以 `internal` 失败、`restoreNode` 会答一个它不声明的码（`page.yaml:767`）。
- `contract_test.go` 的 `TestCheckContract`：上表的每个非"—"的格子一行（`FactsOf` 从手写的 `Change` 建事实，次序照单元的），外加：普通的笔记本对每一行都答零；`AGENTS` 的键大小写（`Agents`）；保留的键的四种写法与大小写；子文件夹里的 `index` 放行；一步同时犯两条时答次序在前的码（把来源根改名为根下的 `index`：R3 与 R4 都犯，答 R3 的 `page.virtual_file`）；R4 的 `Verdict.NodeID` 是来源根。另有 `TestFactsOfAStep`：事实是改动的节点、照改动的次序，每个位置是父节点与名字的标题键，新建没有"前"、删除没有"后"。B-Q1 的几行：同一父节点下排序 `Notes`（`raw` 在它之下：第一个节点 `Notes` 的父节点不变，`raw` 的前后相同）→ 零；把 `Notes` 移到另一页之下 → `Needs` `page.manage_source`、节点是 `raw`；同一父节点下排序 `raw` 自己 → `Needs`；`Relocate` `Notes`（换父节点与名字）→ `Needs`；`raw` 的祖先改名 → 零。

### 3.22 S2：拒绝的标出、码与文字

**错误**（`page/domain/errors.go`）：

```go
	// ErrProtected is an LLM Wiki's AGENTS page or source root that a step
	// would delete, or AGENTS one it would rename or move (M10 design 4.9).
	ErrProtected = shared.NewError(shared.KindConflict, "page.protected",
		"An LLM Wiki keeps its AGENTS page and its source root: neither can be deleted, and AGENTS cannot be renamed or moved.")
	// ErrVirtualFile is a node a step would put at an LLM Wiki's root under
	// the name of a generated page (M10 design 4.9, 4.13).
	ErrVirtualFile = shared.NewError(shared.KindConflict, "page.virtual_file",
		"index.md and log.md at an LLM Wiki's root are generated: the index follows the pages, the log is the changesets. No page or attachment at its root may be named index or log.")
	// ErrSourceAdminOnly is a step in an LLM Wiki's source area, its source
	// root's own included, that only the notebook's admins may make (M10
	// design 4.7).
	ErrSourceAdminOnly = shared.NewError(shared.KindForbidden, "page.source_admin_only",
		"Only the notebook's admins can rename, move or delete in an LLM Wiki's source area, its source root included.")
```

MCP 照通用的写法答它们（"`<code>: <Detail>`"，`mcpserver/errors.go:129-…`；`toolErr`、`writeErr`、`moveErr` 只特别处理它们自己的类型，`page/adapter/mcp/text.go:22-43`），`batch` 里带 `At` 与 `op`（`batch.go:165-199`），不加 MCP 的措辞代码。

**标出**（`page/domain/refused.go`）：

```go
// Refused is a guard's or a precheck's refusal of a step, marked with the
// node it is about (M10 design 4.6): a batch answers it at its operation
// rather than read it as a tree that changed, and a background job stops
// with it as refused, its code and node in the report. It unwraps to the
// refusal, so every errors.Is and errors.As of today sees through it.
type Refused struct {
	Err    error
	NodeID uuid.UUID
}

func (r *Refused) Error() string { return r.Err.Error() }
func (r *Refused) Unwrap() error { return r.Err }

// Refuse marks err as a refusal of the node id; an err marked already is
// as it was.
func Refuse(err error, id uuid.UUID) error
```

- `Unit.apply`（`unit.go:303-307`）：守卫的错误是 `*shared.Error` 而还没有标出时，以这一步的第一个节点（`s.Changes[0].NodeID`）标出：编辑锁的 `page.locked` 也标出，模式的守卫的已带它自己的节点。不是 `*shared.Error` 的（数据库的错误）照原样。`page/app/unit_test.go` 的 `TestAUnitMarksItsGuardsRefusals`：一个替身守卫交回 `*shared.Error` 时出来的是以这一步的第一个节点标出的 `Refused`（`page.Refusal` 认得出、`errors.Is` 照样认得问题）；交回一个已标出的时节点照旧；交回一个非问题的错误时原样（第 35 条变异）。`Refused` 不实现 `ProblemError`，`httpserver.ProblemOf` 经 `errors.As` 找到里面的问题（`apierrors.go:256-270`），所以 REST 与 MCP 的答复不变；今天认守卫的错误的每一处都用 `errors.Is`/`errors.As`（`apply_batch.go:576-600`、`:797-800`，`path_writes.go:23-42`，`unit_revert.go:155`、`:165`、`:913-919`，`asset/app/upload.go:116-121`，`bootstrap/transfer.go:225-246`、`bootstrap/linking.go:257-267`，`transfer/app/background.go:241-271`、`batches.go:69-96`，`page/adapter/mcp/locks.go:89`），所以都照旧（`TestTheRegistrantsRunInTheirOrder` 的 `errors.Is(err, refusal)` 照过，`unit_test.go:299`）。
- **模块根**（`page/refusal.go`）：

  ```go
  // Refusal is the code of err's refusal and the node it is about, when
  // err is one a guard or a precheck marked (M10 design 4.6): the
  // transfer module's jobs read it through the composition root.
  func Refusal(err error) (code string, nodeID uuid.UUID, ok bool)
  ```

  S2 只有它与它的测试（`page/refusal_test.go`）；S3 的 `treeError` 用它（3.26）。
- **`batch` 的 `failed`**（`apply_batch.go:573-600`）：先认 `Refused`，再看 `replans`：

  ```go
  func (a *batchAttempt) failed(i int, err error) error {
  	var refused *domain.Refused
  	var problem *shared.Error
  	switch {
  	case errors.As(err, &refused):
  		return &domain.OpFailed{Op: i, Tool: a.steps[i].Kind, Err: err}
  	case !errors.As(err, &problem):
  		return err
  	case replans(problem):
  		return a.differs(i, "tree", err, nil)
  	}
  	return &domain.OpFailed{Op: i, Tool: a.steps[i].Kind, Err: err}
  }
  ```

  P1 的三个码都不在 `replans` 的集合里（`not_found`、`validation_failed`、`title_taken`、`cycle`、`too_deep`），今天就会落到 `OpFailed{Op: i}`；这一改让一个守卫的拒绝永远不被当作"树变了"重新计划、最后答成 `page.revision_mismatch`（00-M10 4.6），也是下面计划的预检答在第几项的同一条路。`replans` 的注释"A guard's refusal is none: it is answered as it is"改为"A guard's refusal is marked Refused and answered at its operation"。模块测试 `TestABatchAnswersARefusalAtItsOperation`（`apply_batch_test.go`）：一个替身守卫在第二项以 `Refuse(domain.ErrTitleTaken, id)` 拒绝（一个在 `replans` 里的码），答 `OpFailed{Op: 1}` 带 `page.title_taken`，没有第二次计划；没标出的 `ErrTitleTaken` 照旧重新计划（反向对照）。00-M10 第 7 节把这条测试放在 S2 的"`batch` 指出被拒的那一项"，与它一起。

**`batch` 的计划的预检**（在 S2，不在 S3：主持者对 B-Q2 的决定。没有它，S2 与 S3 之间 MCP 的 `batch` 先由编辑锁的预检 `apply_batch.go:294` 答 `page.locked`、后由守卫答 `page.protected`，与 REST、MCP 的 `delete` 的次序不同，审查 B-M8a）：

- `domain.PlanBatch(nodes, steps, ids)`（`page/domain/batch_plan.go:301-321`）是纯函数，加一个参数 `n NotebookState`。树的一遍（`treePass.plan`，`:336-346`）模拟每一项之后，以这一项改动的节点的前后位置建 `StepFacts`（新建：补建的每个父页与那一页各一个节点；改名：那个节点；移动：被移动的节点在先，再是模拟的树里它的每个后代；删除：子树的每个节点，根在先；正文：那一页），调 `CheckContract`：`Err` 记成这一项的失败 `p.Failed = &OpFailed{Op: i, Tool: s.Kind, Err: Refuse(err, node)}`（与今天的领域的失败同一个格子，这一遍到此为止）；`Needs` 记下第一个要它的项 `p.Needs = &Need{Op: i, Action, NodeID}`，这一遍照常往下走（计划还不知道答复）。照实（S2 的偏离 9）：每一项在它自己的方法里、它自己的检查之后、它记下改动之前判它的步（`treePass.judge`，单元的种类当作 `edit`）：事实与模拟之后的相同，被拒的一项不在计划的树里留下改动；所以一项的拒绝在同一项上批量自己的界线（移动带着的节点、项数）之前，同分开的调用（它们没有这些界线）。
- `BatchPlan` 加一个方法：

  ```go
  // Refuse records err, a refusal its caller decided of the op at, as the
  // plan's failure when it comes before the plan's own (M10/P1 design
  // 3.22): Reads, CheckReads and Contents then stop before at, as they do
  // at a failure of the tree's pass, so that a batch answers the op that
  // separate calls would be refused at first. A failure of the plan's at
  // the same op, a bound of the batch's, gives way to it: separate calls
  // have none.
  func (p *BatchPlan) Refuse(at int, err error)
  ```

  它设 `Failed`，并把第一项在 `at` 或之后的页从 `Reads` 的次序（`p.order`）里去掉；`Contents` 本就只走到 `Failed.Op`（`batch_contents.go:40-44`）。照实（S2 的偏离 10）：`Failed` 在 `at` 之前时不动；在 `at` 或之后时取代它，同一项上批量自己的界线（项数、移动带着的节点）也让给它，因为分开的调用没有这些界线；并清掉 `final`（项数与读的界线让正文的一遍不跑的那个标记），所以正文的一遍走到被拒的那一项之前，更早的一项的正文的失败照旧在先。两种行为由 `TestThePlanChecksTheContract` 的三行钉住（S2 的修复一轮加，审查 A-M1 = C-M2，第 81、82 条变异）。
- 应用里（`apply_batch.go` 的 `planned`，`:261-312`）：`ListNodes` 之后不加锁读 `StateOf`（`ok` 为假答 `notebook.not_found`），`PlanBatch` 之后、`a.read` 之前问：

  ```go
  	a.plan = domain.PlanBatch(nodes, a.steps, ids, state)
  	if need := a.plan.Needs; need != nil {
  		err := a.grant(ctx, need.Action)
  		switch {
  		case errors.Is(err, shared.ErrNotVisible):
  			// The caller lost the notebook since Allowed: the operation's
  			// 404, as authorize answers it (authorize.go:14-22).
  			return domain.ErrNotebookNotFound
  		case isForbidden(err):
  			a.plan.Refuse(need.Op, domain.Refuse(domain.ErrSourceAdminOnly, need.NodeID))
  		case err != nil:
  			return err // the database's, as it is: no refusal
  		}
  	}
  	texts, heads, err := a.read(ctx)
  ```

  `a.grant` 以请求的执行者与这本笔记本问 `a.b.auth`（`Allowed` 判定 `page.batch` 用的同一个授权者）。目标的工作区是判定 `page.batch` 时读到的：`decideNotebook` 包着新的 `decidedNotebook`，它另答笔记本的工作区，`Execute` 把它留在这一次尝试里，不多读一次（`page/app/authorize.go`，S2 的偏离 8）。
- **码按项的次序**（审查 B-M3）：拒绝在 `a.read` 之前记下，所以 `Reads`、`CheckReads` 与 `a.read` 的 `differs`（一页不见了）只看它之前的项：写者的 `[move raw.md → Notes/raw.md, edit Big.md …]` 在第 1 项读的页过了 `MaxBatchReadBytes` 时答第 0 项的 403，同分开调用时第一个被拒的一样。更早的一项的正文的失败照旧在先（`Contents` 走到第 0 项之前的每一项）；树的一遍的失败总在 `Needs` 之后（那一遍在失败处结束，`Needs` 只记它之前的）。
- 所以一个 `batch` 答的是"按次序分开调用"时第一个会被拒的那一项：写者的 `[move raw, delete AGENTS]` 答第 0 项的 403，管理员的答第 1 项的 409。计划是权威（00-M10 4.6），守卫在单元里是后备；计划的问不进单元的 `Grants`（3.19）。
- **测试**：`page/domain/batch_plan_test.go` 的 `TestThePlanChecksTheContract`：普通的笔记本什么也不记；LLM Wiki 里每种操作的一项（R1–R4 各一，含 B-Q1 的排序祖先放行）记在它的项上；`Needs` 与一个更晚的 `Failed` 都记下；补建的父页 `index` 在根下也拒；`Refuse` 在一个更晚的失败之前时取代它、之后时不取代；`Refuse` 之后 `Reads` 只列它之前的项的页。`page/app/apply_batch_test.go` 的 `TestABatchsPlanRefusesBeforeItsLock`：写者的 `[move raw, delete AGENTS]` 答第 0 项的 `page.source_admin_only`，管理员的答第 1 项的 `page.protected`；`AGENTS` 被别人的会话持着时仍答 `page.protected`（不是 `page.locked`：计划在编辑锁的预检之前）；记下的调用里没有 `"LockNotebook in tx"`（单元没有开始）；`page.manage_source` 只被问一次（`page.batch` 由 `Allowed` 另问，不算）；B-M3 的两行：写者的 `[move raw → Notes/raw, edit Big]` 而 `Big` 的头过了 `MaxBatchReadBytes` → 第 0 项的 403、没有读 `Big` 的正文（没有 `PageContentWithin`），`Big` 在计划之后不见了 → 仍是第 0 项的 403、没有第二次计划；授权者答 `ErrNotVisible` → 404 `notebook.not_found`；答一个数据库的错误 → 原样、不是 `OpFailed`。照实：没有 `batch` 的测试钉住整个调用列表（计划开始读 `StateOf` 时一个也没有失败），设计的 `"StateOf nb"` 没有地方加；替身记 `"StateOf"`、不带 id（同 S1 的偏离 6），最后一行核对它紧跟 `ListNodes`、不在事务里（S2 的偏离 11）。照实另有：`TestThePlanChecksTheContract` 的 B-Q1 的排序祖先写成祖先在同一父节点下的改名与移到原处（`batch` 的移动没有兄弟间的位置，S2 的偏离 13）；它另有修复一轮的三行：`Refuse` 落在同一项上批量的界线（项数的、移动带着的节点的）之上时答那一项的 `page.source_admin_only`，`Refuse` 清掉 `final` 之后 `Reads` 列出更早的一项的页、`Contents` 答第 0 项的正文的失败（S2 审查 A-M1 = C-M2）。`TestABatchsPlanRefusesBeforeItsLock` 另有一行"写者的 `[move raw, edit AGENTS]`，`AGENTS` 被别人的会话持着"，答第 0 项的 403：上面被持着的 `AGENTS` 的删除在树的一遍就被拒（第 1 项的 409），到不了编辑锁的预检，这一行才钉住计划的问在编辑锁的预检之前（第 38 条变异，S2 的偏离 14）。

**契约的码**（00-M10 第 5 节的 P1/S2 的格子；新码放在领域的码之后、编辑锁与链接的码之前）：

| 操作（文件） | `x-problem-codes`（改后） |
|---|---|
| `createPage`（`page.yaml:59`） | `[notebook.not_found, forbidden, validation_failed, page.title_taken, page.too_deep, page.virtual_file, server_busy]` |
| `renameNode`（`:578`） | `[page.not_found, forbidden, validation_failed, page.title_taken, page.protected, page.virtual_file, page.source_admin_only, linking.pages_locked, server_busy]` |
| `moveNode`（`:647`） | `[page.not_found, forbidden, validation_failed, page.cycle, page.title_taken, page.too_deep, page.protected, page.virtual_file, page.source_admin_only, linking.pages_locked, server_busy]` |
| `deleteNode`（`:611`） | `[page.not_found, forbidden, page.protected, page.locked]` |
| `revertChangeset`（`:957`） | `[validation_failed, page.changeset_not_found, forbidden, page.changeset_trimmed, page.revert_too_large, page.revert_conflict, page.protected, page.source_admin_only, server_busy, page.locked]` |
| `uploadAsset`（`asset.yaml:45`） | `[notebook.not_found, forbidden, validation_failed, page.title_taken, page.virtual_file, storage_full]` |

每个操作的 `description` 加一句它在 LLM Wiki 里拒绝什么，例如 `deleteNode`："In an LLM Wiki, its AGENTS page and its source root cannot be deleted, nor a page whose subtree holds the source root (page.protected)."；`renameNode`、`moveNode`："In an LLM Wiki, AGENTS cannot be renamed or moved (page.protected), no node at the root may take the name index or log, nor index.md or log.md (page.virtual_file), and only its admins can rename or move the source root, or move a page whose subtree holds it under another parent (page.source_admin_only)."；`createPage`、`uploadAsset`：保留的标题那一句；`revertChangeset`：前两句的"一次撤销的每一步照它的操作判"。照实（S2 的偏离 7）：`uploadAsset` 的一句放在"The checks are made again as the attachment is created."之后：S2 里上传的预检不判契约（S3 的 `Check` 判，3.25），只有单元里的建拒绝它；`revertChangeset` 的一句只说每一步照它的操作判，不说与 S3 的预检的次序，它之后已有的"Then a page whose lock an edit session holds … is page.locked"要到 S3 的预检才对每个形状成立（第 0 节，S2 审查 A-M3）；`deleteNode` 的一句另说"before any lock is looked at"。

`apitest.Main` 两个方向都核对（`apitest/problems.go:79-104`、`:171-228`）：`page/adapter/http/handler_test.go` 的 `TestTheOperationsAnswerEachProblem`（`:339-…`）加 createPage 的 `page.virtual_file`、renameNode 与 moveNode 的三个、deleteNode 的 `page.protected` 各一行（假的用例答 `domain.Refuse(domain.ErrProtected, id(12))` 一类，同时核对标出不改答复）；`history_test.go` 的 revertChangeset 两行；`asset/adapter/http/upload_test.go:344-352` 一行 `h.tree.createErr = shared.NewError(shared.KindConflict, "page.virtual_file", "…")`（asset 不导入 page，照 `:344-346` 的 `page.title_taken` 那一行自己造问题；S3 再加 `checkErr` 的一行：`Check` 答 `page.virtual_file` 时答 409、不调 `Store`（`stored`、`kept` 都是 `false`），同 `TestUploadAnswersEachFailure` 的 `checkErr` 的几行（`:337-346`），3.28）。

**MCP 的码**（`bootstrap/permission_matrix_mcp_test.go:84-108` 的 `mcpToolCodes()`）：`write` 加 `page.virtual_file`；`move` 加 `page.protected`、`page.virtual_file`、`page.source_admin_only`；`delete` 加 `page.protected`；`batch` 加这三个；`edit` 不加（P1 里正文的写没有可拒的）。`TestTheMCPToolsErrorsCarryTheirCodes`（`bootstrap/mcp_reads_test.go:426-473`）的应用今天只有普通笔记本，加 `wikiCodeCalls`（`mcp_write_codes_test.go`，照 `writeCodeCalls`，`:45-112`）：在应用里经 REST 建一本 LLM Wiki（`workspace_access: editor`，所以工作区的成员是写者；照实，写者是经 SQL 加进 acme 的新账户 erin，读的应用里的 carol 只能看，erin 的 PAT 发写者的那一次 `move`，S2 的偏离 6），然后 `write {"path":"index/x.md"}`（补建的父页 `index` 在根下）→ `page.virtual_file`；`move {"from":"AGENTS.md","to":"Notes/AGENTS.md"}` → `page.protected`；写者 `move {"from":"raw.md","to":"Notes/raw.md"}` → `page.source_admin_only`；`move {"from":"Notes.md","to":"log.md"}` → `page.virtual_file`（MCP 的 `move` 的参数是 `from`、`to`，`api/mcp/tools.json`；设计起初写的是 `path`）；`delete {"path":"AGENTS.md"}` → `page.protected`；`batch` 的三项各一次。`mcpAnswer.toolError`（`mcp_client_test.go:304-313`）让任何看到一个不在它的工具的行里的码的测试失败，所以这些行要与守卫同一个提交。

**权限矩阵的变体的行**（3.14 的变体）：

| 行 | 目标 | 格子 |
|---|---|---|
| `renameNode, LLM Wiki source root` | `wlab` 里那本笔记本的 `raw`，改名为 `sources` | 管理员 200（`check`：名称是 `sources`）；别的写者 403 `page.source_admin_only`；读者 403 `forbidden`；看不到的 404 `page.not_found` |
| `moveNode, LLM Wiki source root` | 把 `raw` 移到同一本里另一页之下 | 同上 |
| `deleteNode, LLM Wiki AGENTS` | `AGENTS` | 写者与管理员 409 `page.protected`；读者 403；看不到的 404 |
| `deleteNode, LLM Wiki source root` | `raw` | 同上（P1 里来源根谁也删不掉；P2 的来源上的删除那一行才有"管理员成功"，00-M10 4.17 那一句是 P2 的） |
| `createPage, LLM Wiki reserved title` | 根下建 `index` | 写者与管理员 409 `page.virtual_file`；读者 403；看不到的 404 `notebook.not_found` |
| `uploadAsset, LLM Wiki reserved title` | 根下传 `log` | 同上 |

需要一个新的格子的帮手 `adminsOnlyOr(writer cell)`（照 `adminsOnly`，`permission_matrix_page_test.go:626-639`）：管理员 `cellOK()`，别的写者 `writer`，读者 `cellForbidden()`，看不到的 404。

**网页的文字**随这个 Step（`problem-messages.test.ts` 读打包的契约，每个操作的每个码要有文字，`web/…/app/problem-messages.test.ts:55-62`）：三个码进 `problemMessages`，两种语言的文字见 4.2。

### 3.23 S2：P1 里没有可拒的路径与降级的交错

**P1 里没有可拒的路径**（00-M10 第 7 节要写明；每一条由 3.24 的整个程序的测试核对它在 LLM Wiki 里照常成功）：

| 路径 | 为什么 |
|---|---|
| `putPageContent`（`put_page_content.go:64`）、`toggleTask`（`toggle_task.go:86`）、`restorePageVersion`（`restore_version.go:65`）、MCP 的 `edit`（`edit_page.go:177`）、`write` 覆盖已有的页 | 只有 `OpContent`（`unit_content.go:77`）；P1 许 `AGENTS` 与来源根的正文照常写（4.9）；来源上的规则是 P2 的 |
| `openEditSession`（`open_edit_session.go:42` → `unit_session.go:25-59`） | 只过否决者；P1 没有会话的规则（3.20） |
| `restoreNode`、后台的恢复 | `AGENTS` 与来源根在活着的 LLM Wiki 里从不进回收站；一页叫 `AGENTS` 的回到根下由 `restoredNames` 加序号（根下的兄弟持着这个键）；保留的标题 S3 加序号，不拒（3.21） |
| 导入 | 只新建；`AGENTS.md`、`raw/` 因为名字已被占而加序号（`children.free`，`unit_import.go:172-194`）；保留的标题 S3 加序号 |

其余的写在 P1 都可能被拒：`createPage`、MCP 的 `write`（新建）与 `batch` 的写（根下的保留的键，含补建的父页，`write_path.go:150`、`apply_batch.go:620`）；`uploadAsset`；`renameNode`、`moveNode`、MCP 的 `move`、`batch` 的 `move`；`deleteNode`、MCP 的 `delete`、`batch` 的 `delete`；`revertChangeset` 与 `startRevert`（撤销模板的变更集要删掉 `AGENTS` 与 `raw`；写者撤销管理员对 `raw` 的改名）。S2 与 S3 之间 `startRevert` 照样开始一个会被拒的后台撤销，它在第一批以 `internal` 停下（`revertDefect`，`unit_revert.go:913-919`），S3 补上开始时的预检与 `refused`（3.25、3.26）：这是第 0 节写明的 S2→S3 的窗口之一，由 S3 的合并关上。S2 的审查另找到同步的撤销的一个（A-M3，第 0 节）：S2 里它没有契约的预检，守卫逐步判，前一步碰到编辑锁时答 `page.locked`、不答后一步的 `page.source_admin_only`；S3 的两遍预检关上它。

**降级与 `AGENTS` 的删除的交错**（`bootstrap/interleavings_wiki_test.go` 的 `TestTheProtectionAndADowngrade`，第三份）：`tm.interleaveOn(t, notebookRow(nb), first, second)`：删除 `AGENTS` 的单元与降级都以 `FOR NO KEY UPDATE` 锁笔记本行（`unit.go:217-223`、`manage.go:26-41`），持着行的测试事务让它们按发出的次序排队。删除在先 → 409 `page.protected`（单元读到的是 `llm_wiki`），降级 200，`AGENTS` 还在；降级在先 → 降级 200，删除 204（单元读到的是 `standard`），`AGENTS` 进了回收站。两种都以 `checkPages` 结束（降级之后的笔记本不再受 3.14 的规则约束）。它证明模式在锁下读一次：单元读到的模式与它写的时候的一致。

### 3.24 S2：整个程序的测试

| 测试（文件） | 份 | 核对 |
|---|---|---|
| `TestTheModeGuardReachesEveryPath`（`bootstrap/wiki_guard_test.go`） | 三 | 一本 LLM Wiki（管理员 alice、写者 bob 经开放程度 `editor`）与一本内容相同的普通笔记本，同样的请求，依次：（1）撤销模板的变更集（409 `page.protected`），这时 `raw` 与 `AGENTS` 还是它留下的样子；（2）alice 把 `raw` 改名为 `sources`（200），bob 撤销这次改名（403 `page.source_admin_only`：放回是来源根的 `OpRename`），alice 撤销它（200，`raw` 回到原名：反向对照）；两次撤销都在 `raw` 还是那个变更集留下的样子时，之后的移动与 PUT 会让它们答 409 `page.revert_conflict`（撤销的计划先核对每个碰过的节点，`page/domain/revert.go:450-457`，审查 B-M1）；（3）根下建 `index`、`index.md`（409 `page.virtual_file`），子页下建 `index`（201）；（4）改名 `AGENTS`（409 `page.protected`），bob 改名 `raw`（403），一页改名为根下的 `log.md`（409 `page.virtual_file`）；（5）排序 `AGENTS`（`moveNode` 同一父节点、另一个位置，409 `page.protected`）、把 `AGENTS` 移到一页之下（409）；（6）bob 移动 `raw`（403），alice 把 `raw` 移到 `Notes` 之下（200），之后 bob 在同一父节点下排序 `Notes`（200：来源区没有换位置，主持者对 B-Q1 的决定），bob 把 `Notes` 移到另一页之下（403 `page.source_admin_only`），bob 改名 `Notes`（200，祖先的改名放行）；（7）根下传一个叫 `LOG` 的附件（409 `page.virtual_file`：键是 `log`，大小写折叠的一处核对）、叫 `log.png` 的（201）；四个保留的键各只用一次（`index`、`index.md` 在第 3 步，`log.md` 在第 4 步，`log` 在第 7 步），所以普通的那一本的根下不撞名（审查 B2-M2）；（8）`AGENTS` 与 `raw` 的 PUT 正文、勾选任务、还原版本、开启编辑会话都成功（3.23 的表）；（9）最后是删除，依次 `AGENTS`、`raw`、`Notes`（409 `page.protected`，alice 也是；`Notes` 的子树里有来源根）。每个拒绝之后节点、变更集、版本的数目不变。普通的笔记本里同样的请求全部成功（守卫立刻放行），只有两处不同：第 1 步没有对应（没有模板），第 2 步 alice 的撤销不做（bob 的撤销已成功）；删除放在最后、`raw` 在 `Notes` 之前，所以普通的那一本里每一个删除都还找得到它的页。组合根不登记守卫时失败（最后一跳）。S3 之后上传与撤销的那几行先由预检答出（3.25），这几条路径上守卫自己的最后一跳要等 P2/S2 的只有守卫拒绝的重复（审查 C-I4）；`batch` 的由下面的 `TestTheGuardRefusesWhatThePlanLetBy` 守着 |
| `TestTheModeGuardReachesEveryMCPWrite`（`bootstrap/mcp_wiki_guard_test.go`） | 三 | 经笔记本入口：`write index/x.md`、`move AGENTS.md`、写者的 `move raw.md`、`delete AGENTS.md` 的码与 `structuredContent.error.code`；`edit AGENTS.md` 成功，变更集的客户端是 `mcp:<name>`；之后以 `edit` 答的版本 `write` 覆盖 `AGENTS.md`：写下、什么也没建，agent 的变更集是 `["edit mcp:whole-program", "edit mcp:whole-program"]`（3.23 的"`write` 覆盖已有的页"，S2 的修复一轮加，审查 C-N1）；`batch [edit Notes, delete AGENTS.md]` 答第 1 项（`op: 1`、`At: "ops[1] (delete)"`），`Notes` 没有改（整个回滚）。照实，agent 的变更集按 `client LIKE 'mcp%'` 找：`acmeTeam` 的 REST 请求带登录会话，它们的变更集的客户端是 `web`，不是 `api`；之后要的恰是那几个，多一个被拒的变更集就失败（审查 C）；`move` 发 `from` |
| `TestModeComesBeforeTheLock`（同上 `wiki_guard_test.go`） | 二 | 3.20 的次序：bob 的会话持着 `AGENTS`，alice 删除它答 `page.protected`；降级之后同样的删除答 `page.locked`、带 bob 的锁 |
| `TestTheProtectionAndADowngrade`（`interleavings_wiki_test.go`） | 三 | 3.23 的交错 |
| `TestTheGuardRefusesWhatThePlanLetBy`（`interleavings_wiki_test.go`） | 三 | 只有守卫拒绝的情形（主持者对 B-I1 = C-I4 的决定）。bob 是这本 LLM Wiki 的笔记本管理员。交错：`tm.interleaveOn(t, notebookRow(nb), first, second)`；first 是 alice 把 bob 降为写者（`PATCH /api/v0/notebook-members/{bob 的成员关系的 id}` `{"role":"editor"}`，`api/modules/notebook.yaml:182-186`；它以 `FOR NO KEY UPDATE` 锁笔记本行，等在测试持着的行上），second 是 bob 的 MCP 的 `batch [move raw.md → Notes/raw.md]`（`POST /mcp?notebook=<id>`，bob 的 PAT；交错的一步可以是任何 HTTP 请求，`interleavings_workspace_test.go:130-134`、`:258-288`）：它的计划在锁外读到 bob 还是管理员、`page.manage_source` 放过，单元等在笔记本行上。放开之后降级先提交；bob 的单元判定 `page.batch` 照过（写者能做），守卫在锁下问 `page.manage_source`，答 `op: 0` 的 `page.source_admin_only`，`raw` 没动。反向对照：同样的交错在 `wiring{page: func(d *page.Deps) { d.Guards = nil }}` 的程序里（收 `wiring` 的 `acmeTeam`，`interleavings_workspace_test.go:70-90` 加一个变体 `newAcmeTeamWiring`，它在 `wireApp` 里 `pageDeps` 之后施加）答成功、`raw` 到了 `Notes` 之下：计划确实放过了它，拒绝是守卫的。反过来的次序（`batch` 在先）：成功，之后的降级照常。`batch` 的守卫的最后一跳由它补上 |
| `TestTheDowngradeEndsTheContract`（`bootstrap/wiki_downgrade_test.go`） | 二 | 降级之后同一本笔记本：写者改名、移动 `raw` 成功，`AGENTS` 的改名与删除成功，根下可以建 `index`；经 MCP 的 `delete AGENTS.md` 成功。它是 LW12 的 MCP 豁免所指的那条测试（00-M10 第 9 节）：P2 加来源成为可写、重复不再被拒，P4 加 `list` 没有虚拟的两项，P5 加握手没有系统的一段 |
| `TestPermissionMatrix`（已有的，加行） | 二 | 3.22 的变体的行 |

### 3.25 S3：预检

S3 的三处预检（上传的 `Check`、撤销的不加锁的一遍、撤销的加锁的一遍；`batch` 的计划在 S2，3.22）都调 `CheckContract`（3.21）、以 `FactsOf` 从它们计划的那一步建事实、以 `Refuse` 标出拒绝，所以与单元里守卫的拒绝同码、同节点（13.4 第 13 条：一份规则，不另写）。不加锁的两处以 `Notebooks.StateOf`（S1，3.12）读模式与来源根，每次检查一读，普通的笔记本也读（7.6 写明这一读）；单元里的守卫在锁下再判一次，所以预检放过的、锁下变了的仍被拒绝。要 `page.manage_source` 的，不加锁的预检经单元判定用的同一个授权者问（同预检里今天的 `Allowed`），答案不进单元的 `Grants`：单元在它自己的事务里再问一次（00-M10 4.6："在单元的事务里读"；3.19）。

| 预检 | 在今天的次序里的位置 | 之前的码 → 新的码 → 之后的码 |
|---|---|---|
| `batch` 的计划的树的一遍 | S2（3.22） | 领域的 422、404、409（每项的）→ 409 `page.protected`、`page.virtual_file`、403 `page.source_admin_only`（每项的）→ 409 `page.locked` → 503 |
| 上传的 `Check`（`page/app/asset_writes.go:48-54`） | `assetPlace` 之后 | 404、403、422、409 `page.title_taken` → 409 `page.virtual_file` → asset 模块的 507 `storage_full`（`asset/app/upload.go:67-76`，在 page 的 `Check` 之后） |
| 撤销的不加锁的一遍（`revertChecks.check`，`page/app/revert_changeset.go:188-249`） | `planned` 与步数的上限（`:236-242`）之后、`Locks`（`:243-247`）之前 | 422、404、403、409 trimmed、422 too large、（409 `transfer.busy`，开始时）、409 `page.revert_conflict`、409 trimmed、422 `page.revert_too_large`（计划的步数过了 `b.Steps`，`:239-241`）→ 409 `page.protected`、403 `page.source_admin_only` → 409 `page.locked` → 503 |
| 撤销的加锁的一遍（`Unit.Revert`，`unit_revert.go:69-127`） | 计划（`:86`）之后、第一步（`:97`）之前 | 同上，在单元里，问 `u.write.Grants` |

**后台撤销的每一批不另检查**（主持者对 B-O1 的决定，2.1 第 19 条）：一批是一个单元，守卫在它的步里以同样的码与节点拒绝，整个单元回滚，"这一批一步也没做"不靠一道预检；草稿自己的反向对照也是"去掉每批的预检，结果相同"。后台的路径要的是 `revertDefect` 认出标出的拒绝（3.26 第 1 条），所以两批之间的降级由那一批的守卫拒绝（3.28 的 `TestTheSourceRootsPutBackIsRefusedInALaterBatch`）。00-M10 里写着每批的检查的十一处随之改（2.1 第 19 条，含 4.6 的表的 `:181` 与"transfer 的任务"的括号里的 `CheckRevertPart`，审查 B2-N1 = C2-M2）。

**上传的 `Check`**：`assetPlace` 之后读 `StateOf`（`ok` 为假答 404 `notebook.not_found`），以 `FactsOf(domain.OpCreate, domain.ChangesetEdit, []domain.Change{{NodeID: uuid.UUID{}, After: &state}})` 判（附件的 id 在单元里才定，标出的节点是零 id；P1 的唯一可能的码是 `page.virtual_file`）。拒绝时文件还没传（asset 模块先 `Check`、再 `Store`，`upload.go:66-90`），所以根下传 `index` 不写一个字节就答 409。00-M10 4.6 说这一检查"排在 `storage_full` 之后"：`storage_full` 是 asset 模块在 page 的整个 `Check` 之后查的（`upload.go:70-76`），排在它之后就要 asset 再调一次 page；排在它之前只是让一个磁盘满的服务器对一个保留的标题答 409 而不是 507，两者都不写，2.1 第 7 条改为"之前"。这个次序是结构的（asset 调完 page 的整个 `Check` 才查空间），3.28 的 `asset/app/upload_test.go` 的 `TestCheckDecidesTheNodeThenTheFreeSpace` 的一种钉住它：整个程序里的磁盘的一半照 C 的建议去掉了（第一轮复核），它多一个程序，查的是同一件事。

**撤销的两遍**：`revertChecks.check` 在 `planned` 交回的计划上、步数的上限之后，对它的每一个操作（放回、临时的改名、删除、恢复、写回）建事实（放回：从此刻的位置到计划的位置，移动的连同子树、被移动的在先；删除：子树，根在先；恢复：`OpRestore`；写回：`OpContent`）判；要 `page.manage_source` 的经 `spec` 的执行者问一次。`check` 也是 `startRevert` 的 `CheckRevert`（`jobs.go:148-172`，`:169`）与任务开始时的 `PlanRevert`（`jobs.go:300-320`，`:315`）的那一遍，所以一处加上，三条路径都有：同步的撤销在请求里、开始后台撤销在 `startRevert` 的请求里（"开始时的预检"）、任务真开始跑时在它的计划里（错误经 `treeError`，`bootstrap/transfer.go:136`，以 `refused` 停下，3.26）。00-M10 4.6 说撤销的预检"在 503 之后"：503 是解析的预算（`revert_changeset.go:104-108`），在 `check` 之后；放进 `check` 让被拒的撤销不占预算，`startRevert` 的 503（`start_background.go:151`）本就在 `CheckRevert` 之后。2.1 第 7 条一并改；"被拒的撤销不占解析的预算"由 3.28 的 `TestTheServersLimitsComeAfterThePrechecks` 核对（主持者对 B-I3 的决定取能失败的测试：变更集也写回另一页的正文，预算被占着时被拒的撤销立刻答自己的码，放过的答 503）。加锁的一遍在 `Unit.Revert` 的计划之后照同样的事实判，问 `u.write.Grants`。照实（S3 的偏离 2）：两遍共用 `checkRevertContract`（`page/app/revert_contract.go`），它在普通模式先返回；LLM Wiki 的事实由纯领域的 `RevertFacts`（`page/domain/revert_facts.go`）交出，复用调度器的 `revertSim`、`place`、`delete` 与 `set`。每个操作用前面操作留下的树建 `Change`，再经 `FactsOf`；移动与删除用抽出的 `orderedSubtree` 保持根在先、层级/顺序/id 的次序，移动并改名仍是一项 `OpMove`。恢复、删除与临时改名都更新这份模拟，后面的事实不会看旧树；没有另一套调度或任何存储读取。`TestARevertsFactsFollowItsSteps` 与只写正文的对照核对它，第 85 条变异去掉 `sim.place` 后失败。

**读的调用**（审查 B-N6；S3 审查 Q-A1，主持者的决定 32）：上传与撤销的检查各多一次不加锁的 `StateOf`，普通的笔记本也读；替身的调用名是 `"StateOf"`，不带 nb，钉住整个调用列表的测试已随之更新。锁外撤销只有规则需要 `page.manage_source` 时才在这一遍的局部 `Grants` 回调里再读 `WorkspaceOf`，同一遍至多一次：现有 `Allowed` 丢弃工作区，而额外的授权仍需工作区与笔记本两个目标。`revert_changeset.go` 用同一授权者和请求执行者；失去可见性经 `authorize` 答这一操作的 404，数据库错误原样交回。锁下读 `Write.Notebook`，问单元自己的 `Write.Grants`，不复用锁外的放行。`TestARevertsExtraDecisionKeepsItsErrors` 与 `TestARevertsSourceGrantIsAskedOnceEachPass` 核对额外判定和每遍的次数；这是条件读取，不把所有预检概括为只多一次点查。

**`startRevert` 的契约**：`x-problem-codes`（`transfer.yaml:255-267`）在 `page.revert_conflict` 之后加 `page.protected`、`page.source_admin_only`；`description` 加一句 "In an LLM Wiki, an undo whose steps the notebook's contract refuses is refused at its start, with the code its first refused step would get (page.protected, page.source_admin_only)."。`transfer/adapter/http/start_background_test.go:105` 的表格加两行：`CheckRevert` 的假的答 `shared.NewError(shared.KindConflict, "page.protected", "…")` 与 `shared.NewError(shared.KindForbidden, "page.source_admin_only", "…")`（transfer 不导入 page，照这张表已有的行自己造问题），照原样答出：开始的预检的拒绝不经 `treeError`（`bootstrap/transfer.go:113-123`）。`startTrashRestore` 在 P1 没有可拒的（恢复加序号，3.27），不改。

**有预检的路径上只有守卫拒绝的情形**：00-M10 第 7 节说要到 P2/S2 的去重才有。P1 里有两种：`batch` 的，降级落在计划与单元之间（3.24 的 `TestTheGuardRefusesWhatThePlanLetBy`，S2）；后台撤销的，降级落在两批之间：任务的计划在开始时放过，之后每一批的单元里只有守卫判（3.28 的 `TestTheSourceRootsPutBackIsRefusedInALaterBatch`）。上传、同步的撤销与 `startRevert` 在 P1 没有，到 P2/S2 的重复才有（00-M10 第 9 节）：上传的唯一的码 `page.virtual_file` 只看树与模式，而 M10 里模式只会由 `llm_wiki` 变成 `standard`；同步的撤销在单元里有加锁的一遍，在守卫之前判同一个状态；`startRevert` 不跑单元，之后的事归任务。除此之外，P1 的规则只看树、模式与执行者的角色，预检与守卫看到的同一个状态下结果相同（3.28 的 `TestPrechecksAnswerTheGuardsCodes` 逐条核对）。2.1 第 8 条写明。

### 3.26 S3：后台的拒绝的去处

```
page 的单元                              组合根                     transfer
Unit.apply 的守卫 ─┐
                   ├→ *page.Refused ─→ treeError ─────────────→ *transfer.Refused ─→ stopped / importRun.unit
PlanRevert 的预检 ─┘   (Unwrap 到问题)   先认已有的哨兵（ErrLocked    (Code, NodeID, Unwrap)     → report.Refusal
revertDefect 原样交回                     在它们最后），再认 Refusal                         → unitFailure: refused
```

1. **`revertDefect`**（`page/app/unit_revert.go:913-919`）：第一件事是 `var r *domain.Refused; if errors.As(err, &r) { return err }`，所以一批的一步里守卫的拒绝不再成为 `unclassified` 的 500：这是后台撤销在两批之间被拒的唯一的路（上面的"后台撤销的每一批不另检查"）。`taken`（`:175-180`）先只把 `page.title_taken` 改写，标出的拒绝从它旁边过（它不是 `title_taken`）。
2. **`treeError`**（`bootstrap/transfer.go:218-247`）：已有的九个哨兵照旧在前，编辑锁的 `page.locked` 由单元标出后照样被 `errors.Is(err, page.ErrLocked)` 认出（`Refused` 解开到它），所以照旧是 `page_locked` 带锁（`TestABackgroundRevertAndAnEditSession`，`bootstrap/interleavings_background_stops_test.go:282-317`，钉住）；`switch` 之后：

   ```go
   	var refused *transfer.Refused
   	if errors.As(err, &refused) {
   		return err
   	}
   	if code, node, ok := page.Refusal(err); ok {
   		return &transfer.Refused{Code: code, NodeID: node, Err: err}
   	}
   	return err
   ```

   第一句让它是幂等的：一个导入的节点的错误经 `treeError` 两次（`:202`、`:211` 再 `:110`），第二次交回同一个 `*transfer.Refused`。00-M10 4.6 说"`ErrLocked` 在最前"：今天它是九个里的最后一个（`:243-244`），这里的意思是"在认拒绝之前"，2.1 第 8 条改字。
3. **transfer**（`transfer/app/import_ports.go`，与 `:150-180` 的哨兵在一起；模块根 `transfer/module.go:107-121` 加别名）：

   00-M10 4.6 把它写作 `ErrRefused{Code, NodeID}`：带字段的错误类型照 Go 的写法不以 `Err` 起头（`Err…` 是哨兵的值，如 `ErrLocked`），多的 `Err` 是 page 的拒绝，`Unwrap` 到它，`uncertain` 与 `errors.As` 由此照旧认出页的问题（2.1 第 8 条，审查 C2-N3）。

   ```go
   // Refused is a write of a job that the notebook's contract refused (M10
   // design 4.6): the page module's code, and the node it was about. The
   // job stops with it as refused, its report's refusal those two. It
   // unwraps to the page module's problem.
   type Refused struct {
   	Code   string
   	NodeID uuid.UUID
   	Err    error
   }

   func (r *Refused) Error() string { return r.Code + ": " + r.Err.Error() }
   func (r *Refused) Unwrap() error { return r.Err }
   ```

4. **`unitFailure`**（`transfer/app/batches.go:83-96`）：第一个分支是 `errors.As(err, &refused)` → `failed(domain.FailureRefused, err)`，在 `forbidden` 之前：`page.source_admin_only` 是 `KindForbidden`，排在后面就报成 `forbidden`（"发起人已不能编辑这本笔记本"，不对）。`uncertain`（`:74-81`）不改：`Refused` 解开到页的 `*shared.Error`，所以照旧算拒绝、不算不确定，导入的单元被拒时照旧删掉它传的文件（`import_write.go:190-196`）。
5. **报告**：`unitFailure` 拿不到报告，`refusal` 在报告在手的两处写下（同 `Lock`）：`backgroundRun.stopped`（`transfer/app/background.go:239-271`）在 `ErrLocked` 的分支之后加 `case errors.As(err, &refused): r.report.Refusal = &domain.Refusal{Code: refused.Code, NodeID: refused.NodeID}`、再交给 `unitFailure`；`importRun.unit`（`import_write.go:193-198`）在 `return unitFailure(err)` 之前同样写。`transfer/domain/report.go`：`FailureRefused Failure = "refused"`（放在后台的那一组，`:58-82`）、`Report.Refusal *Refusal`（`:174-188`）、`Refusal{Code string; NodeID uuid.UUID}`；存储的 JSON（`adapter/postgres/store.go:351-445`）加 `refusal,omitempty`，`ListJobs` 照旧带它（只去掉 `problems`、`conflicts`，`jobs.sql:53-65`）；HTTP 的 `reportOf`（`adapter/http/server.go:163-185`）照 `lock` 写出（`null` 时 `null`）。照实（S3 审查 B-M1，修复 `ba5cded6`）：后台与导入共用的 `jobRun.finish`（`transfer/app/job_run.go:207`）先选定最终的 `report.Failure`，若它不是 `domain.FailureRefused` 就把 `report.Refusal` 清空，再交给 `FinishJob`。这样 timeout、interrupted 或 cancelled 覆盖原拒绝时遵守非 refused 为 null 的契约；普通拒绝仍保留码与节点，已提交批次的进度、计数及既有 Lock/Conflicts 行为不改。
6. **契约**（`api/modules/transfer.yaml`）：`TransferReport`（`:824-850`）的 `required` 加 `refusal`，`properties` 加

   ```yaml
        refusal:
          description: >-
            When the failure is refused, the write of the job the notebook's
            contract refused: the code, and the page or attachment it was
            about. Null for every other job and failure.
          anyOf:
            - $ref: "#/components/schemas/TransferRefusal"
            - type: "null"
   ```

   与新的 `TransferRefusal`（`additionalProperties: false`，`required: [code, node_id]`，`node_id` 是 uuid）。**`code` 是自由的字符串**（`type: string`，主持者对 B-I2 = C-I3 的决定，照 00-M10 第 5 节的 `TransferReport` 一行与它的审查 C-M23）：`description` 写 "The code of the refusal, as the write's own answer would carry it: page.protected, page.virtual_file or page.source_admin_only today; later milestones add others without a change to this contract."。P2 的 `page.source_immutable`、`page.source_duplicate` 与 M11 的冻结的码由此都不改契约；一个版本存下、下一个版本读的报告里的码也照收（`report` 存成 JSON，`transfer/adapter/postgres/store.go:347-445`）。`TransferFailure` 的枚举（`:886-907`）加 `refused`，说明（`:852-884`）加 "refused (a write of the job the notebook's contract as an LLM Wiki refused: its refusal says which and where)"。
   **以 `toEqual` 比较整个报告的 e2e**（`git grep` 了 `e2e/` 里每个 `report` 与 `placed: null`、`lock: null`）有四处，都加 `refusal: null`：`e2e/stories/transfer/tr1-export.spec.ts:66-80`、`tr2-import.spec.ts:114-129`、`tr3-roundtrip.spec.ts:52-70`、`tr4-malicious.spec.ts:75-93`（审查 B-I4 = C-I1）。其余对报告的核对逐个字段，不受影响：`tr2-import.spec.ts:140`、`:174`、`:223`、`:278` 与 `tr3-roundtrip.spec.ts:96`、`:145`、`tr4-malicious.spec.ts:125`、`:144`、`:196`、`:238`（`expectImported` 与 `report?.failure`），`tb2-background-restore.spec.ts:150`、`hi5-revert-import.spec.ts:77`、`:120`（计数与 `placed`），`e2e/fixtures/assert/transfer.ts:60-80`、`:215-235`（按列读库）。
7. **网页**（随这个 Step，4.3）：`transfer-report.tsx` 的 `failures` 表加 `refused`（`Record<TransferFailure, PlainKey>`，不加 tsc 就失败，`:23-44`），报告里写出被拒的码的文字与那一页：`code` 是 `string`，网页以 `Object.hasOwn(problemMessages, code)` 认它知道的码，不认得的码写 `problem.other` 带这个码（同 `errorText` 对不认得的码的退路，`W/app/problem-messages.ts:115-118`）；`web/…/test/jobs-server.ts:35` 的 `reportOf` 加 `refusal: null`（每个网页测试的报告都由它展开；网页里没有别的整个报告的字面量）。

**整个 Step 的理由**（00-M10 审查 C2-M4）：P1 的规则已经能让后台的撤销被拒（任务的计划拒绝撤销模板的变更集；两批之间被降为写者的执行者的那一批由守卫拒绝），开始时的预检、任务的计划与去处不在同一个 Step 里，中间就有一段把它报成 `internal`。

### 3.27 S3：保留的标题加序号

三处都以单元的 `Write.Notebook`（S2）判模式，以 `domain.ReservedKeys()`（3.21）为表；只在 `llm_wiki` 的根下（父节点为空）：

- **导入**：`ImportUnit.children(ctx, nil)`（`page/app/unit_import.go:150-166`）在 `llm_wiki` 时把四个键加进根的 `keys`，`free`（`:172-194`）照旧给"名字已被占"的加序号：库的根下的 `index.md`（导入成名叫 `index` 的页，`transfer/domain/import_plan.go:184`）成为 `index 2`，`log/` 文件夹成为 `log 2`，一个叫 `index` 的附件成为 `index 2`；报告的 `renamed` 问题照旧由 `tell` 写下（`transfer/app/import_write.go:332-335`）。库里的文件夹并进同键的页（`import_plan.go:191-197`：`Index/` 归 `index.md`，`TitleKey` 相同），所以一个 `index.md` 与一个 `Index/` 是一页 `index`、它的子页在它之下。`Reserved` 钩子（`import_ports.go:124-127`）不动：它只决定序号跳过哪些候选（`free` 的 `:189`），不让原来的名字加序号（00-M10 4.9）。
- **同步的恢复**：`restoredNames(group, siblings)`（`unit_restore.go:177-210`，唯一的调用者 `:83`）加一个参数 `held []string`：根（第 0 个成员）的兄弟之外另算被占的键；`Restore` 在 `place.ParentID == nil` 且 `llm_wiki` 时交 `ReservedKeys()`，否则 `nil`。答复的 `renamed: true`（`restore_node.go:128`）。
- **后台的恢复**：`RestorePart` 的根的分支（`:298-305`）在 `kids = places.under(-1, siblings)` 之后、`place.ParentID == nil` 且 `llm_wiki` 时把四个键加进 `kids.keys`（不在 `under` 里加：`under` 也给非根的父节点用，`:386-395`、`:406`、`:423`）。一个后台的恢复恢复回收站的一项，只有它的根（第 0 个成员）落到根下，别的成员回到它们的父节点之下（`partPlaces.of`，`:399-420`），所以一个任务至多数一个保留的改名。报告照今天的"兄弟持着它的名字"只数 `counts.renamed`（`background.go:215-218`），没有 `renamed` 问题：`TransferProblem.renamed` 是导入与导出的（`transfer.yaml:970-986`）。00-M10 第 7 节的"报告 `renamed`"在后台的恢复里是这个计数，2.1 第 8 条写明。
- 撤销的放回不加序号：名字被占是冲突（`unit_revert.go:426-429`、`revert_schedule.go:261-279`），根下的保留的键又从来不会有人持着（3.21 的表）。
- **`checkPages`** 加一条："live LLM Wikis' root nodes with a reserved title key"：

  ```sql
  SELECT count(*) FROM nodes x JOIN notebooks n ON n.id = x.notebook_id
  WHERE n.mode = 'llm_wiki' AND n.deleted_at IS NULL AND x.deleted_at IS NULL AND x.parent_id IS NULL
      AND x.name_key IN ('index', 'log', 'index.md', 'log.md')
  ```

  `TestPageViolationsCatchEachLLMWikiGap`（3.14）加它的反例，都在删除这本 LLM Wiki 的最后一行之前：以 SQL 把 LLM Wiki 的根下一页的 `name`、`name_key` 改成 `Index`、`index` → 这一句在违反里；同样改子页下的一页 → 不在；在普通的笔记本的根下 → 不在。S2 的守卫与 S3 的加序号合起来让这条在每个测试之后成立。
- **测试**（模块一级，真的单元，`page/app/import_test.go`、`restore_test.go` 的写法）：`TestAnImportNumbersTheReservedTitlesAtAnLLMWikisRoot`（根下的 `index`、`LOG`、`index.md` 的页与 `index` 的附件，各加序号；子页下的 `index` 不加；普通的笔记本里都不加）；`TestARestoreNumbersTheReservedTitlesAtAnLLMWikisRoot`（父节点在回收站里、落到根下的 `index`：同步的答 `renamed: true`，后台的那一部分数 `Renamed`；后台的一部分里叫 `index` 的成员随它恢复了的父节点回到那一页之下、一部分的根回到一页活着的子页之下，都留着 `index`、不数 `Renamed`（第 50 条变异"加序号写在 `under` 里"让它们成为 `index 2`，审查 B2-N2）；普通的笔记本里不加）。

### 3.28 S3：测试

**模块一级**（替身的守卫，照 00-M10 第 7 节 S3 的"模块一级以假的守卫测 `revertDefect`、`treeError` 的标出与 `ErrLocked` 在前"；`replans` 的那一条与 `batch` 的计划的两条是 S2 的，3.22，2.1 第 20 条把 `replans` 挪进 S2 的验证，审查 C3-N2）：

- `page/app/asset_test.go` 的 `TestAnUploadsCheckRefusesAReservedTitle`：根下的 `index` 答 `page.virtual_file`；名字不合法的先答 422、被占的先答 409 `page.title_taken`；子页下的 `index` 照过。`asset/app/upload_test.go` 的 `TestCheckDecidesTheNodeThenTheFreeSpace`（`:72-101`）加一种：page 的 `Check` 答 `page.virtual_file` 而磁盘不够时，`Check` 答 `page.virtual_file`（空间在 page 的整个 `Check` 之后才查）；"不调 `Store`"在 HTTP 适配器那一层核对（`Upload.Check` 本就不调 `Store`，是处理函数先 `Check`、再 `Store`，`asset/adapter/http/upload.go:63-68`）：3.22 的 S3 的 `checkErr` 一行（审查 B3-N2）。
- `page/app/revert_test.go` 的 `TestARevertsChecksRefuseTheContract`：撤销模板的变更集在不加锁的一遍答 `page.protected`（没有单元）；它的 `AGENTS` 被别人的会话持着时仍答 `page.protected`；写者撤销管理员对 `raw` 的改名答 `page.source_admin_only`，授权者被问一次；`CheckRevert`（开始）与 `PlanRevert`（任务的计划）答同样的码；过了计划的步数的仍先答 422 `page.revert_too_large`（审查 B-N2 的次序）；**锁下的一遍**（这一行的写者不登记会拒的守卫，交一个记下步、什么也不拒的替身：登记真的 `ModeGuard` 时它在第一步以同样的码拒绝，这一行就分不出是谁拒的，审查 B2-N4）：假的授权者第一次（不加锁的一遍）答管理员、之后答写者，写者撤销管理员对 `raw` 的改名时 `Unit.Revert` 的锁下的一遍答 `page.source_admin_only`、一步也没做，替身守卫一步也没看到，授权者在事务里被问一次（第 53 条变异）。
- `page/app/jobs_test.go`（照 `TestARevertJobsBatchErrors`，`:990-1050`）的 `TestARevertJobsBatchPassesAGuardsRefusal`：一个替身守卫在一批的一步里的标出的拒绝经 `revertDefect` 原样交回（`page.Refusal` 认得出），这一批一步也没留下（单元回滚）；编辑锁的 `page.locked` 照旧原样交回；没标出的 `page.title_taken` 照旧成为缺陷（反向对照）。
- `bootstrap/transfer_tree_test.go`（新，纯函数）的 `TestTreeErrorKeepsTheLockFirst`：标出的 `page.locked` → `transfer.ErrLocked`；标出的 `page.protected` → `*transfer.Refused` 带码与节点；再经一次 → 交回同一个 `*transfer.Refused`（`got == in`：再包一层的码与节点相同，只比字段看不出，审查 C2-N5）；没标出的 `page.protected` → 原样（不是拒绝）。
- `transfer/app/background_test.go` 的 `TestABackgroundJobStopsWithItsReason`（`:224-290`）加两行（计划与一批各一）：`*Refused` → `refused`，报告的 `Refusal` 是它的码与节点，`Lock` 为 `nil`；默认分支的核对（`:283-286`）加 `Refusal == nil`。`import_test.go` 的 `TestAnImportsUnitFails`（`:715-742`）加 "refused"：`refused`、文件删掉、报告带拒绝。照实：在指定的 `adapter/postgres/store_test.go` 与 `adapter/http/start_background_test.go` 里另设相邻的 `TestARefusedJobsReportReadsBackAsWritten`、`TestARefusedJobReadsWithItsReport`，核对详情/列表的拒绝、旧报告、普通报告显式 null 与自由码 `page.some_later_code`；原冲突/锁的往返保留，不往非 refused 的夹具里塞拒绝元数据。另有 `TestAJobsRefusalBelongsToItsFinalFailure`（`transfer/app/background_test.go`）：第一批提交、第二批返回拒绝，普通拒绝保留码与节点；timeout、interrupted、cancelled 三种最终结果清掉 refusal，四行都核对 Done=1/Total=3、计数=1、只尝试两批，第 86 条变异抓到漏清理（B-M1）。

**整个程序**：

| 测试（文件） | 份 | 核对 |
|---|---|---|
| `TestTheSourceRootsPutBackIsRefusedInALaterBatch`（`bootstrap/wiki_background_test.go`） | 三 | `newAcmeTeamWired(t, …, transfer.Batch{Nodes: 1})`（`interleavings_workspace_test.go:65-86`，每批一步），照 `TestARevertJobAndAPageCreatedUnderATopItDeletes`（`interleavings_background_test.go:351-378`）的循环：`for try := range betweenUnits`，每一次一本新的 LLM Wiki（alice 建，bob 加为笔记本管理员；alice 再建根下的 `Notes`，id 比 `raw` 大），bob 经 MCP 的一个 `batch` 把 `raw` 改名为 `sources`、`Notes` 改名为 `raw`（一个变更集 C）。撤销的计划：第一轮先试 `raw`（id 最小，3.4 第 6 步定下的次序），名字 `raw` 被 `Notes` 持着，等；`Notes` 放回 `Notes`；第二轮 `raw` 放回 `raw`（`revert_schedule.go:290-292`、`:313-321`、`:232-234`），所以两步分在两批，`raw` 的在第二批。`tm.behindFirstBatch(t, nb, revertStart("bob", nb, C, ""), demotion)`（`bootstrap/background_test.go:196-210`）开始撤销（202：bob 是管理员，开始时的预检与任务的计划都放过），让 alice 的降级 `PATCH /api/v0/notebook-members/{bob 的成员关系的 id}` `{"role":"editor"}`（`api/modules/notebook.yaml:182-186`，以 `FOR NO KEY UPDATE` 锁笔记本行）排在第一批之后；它在第二批之前还是之后是竞争。**输的那一支**（任务 `succeeded`：降级在第二批之后）：`counts.put_back` 是 2、`raw` 叫 `raw`，`continue`。**赢的那一支**：任务 `failed`，`failure` 是 `refused`，`refusal` 是 `{code: page.source_admin_only, node_id: raw 的 id}`；`counts.put_back` 是 1（第一批留着：`Notes` 叫 `Notes`）；`raw` 仍叫 `sources`；第二批的单元对写者照常判定通过 `page.revert`，拒绝来自这一批的守卫（每批没有另外的预检，3.25）；`checkPages`；`return`。`betweenUnits` 次都输时 `t.Fatalf`。反向对照：去掉 `revertDefect` 认 `Refused` 的那一句，赢的那一支失败成 `internal` |
| `TestPrechecksAnswerTheGuardsCodes`（`bootstrap/wiki_guard_test.go`） | 二 | 3.24 的每一条经预检的路径（`batch`、上传、同步撤销、`startRevert`）答与守卫相同的码与同样的项；`startRevert` 被拒时没有任务的行。另钉住 S2 审查 A-M3 的形状（S2 的审查记录的主持者的决定 29）：alice 的一个 MCP 的 `batch` 把 `raw` 改名为 `sources`、在根下写一页新的 `raw`（变更集 C）；carol 在新的 `raw` 上开编辑会话；bob（写者）`revertChangeset C`：撤销的第一轮删除新的 `raw`（碰到 carol 的锁），第二轮把 `sources` 放回为 `raw`（要 `page.manage_source`）。答 403 `page.source_admin_only`，什么也没写；没有预检时守卫逐步判、答 409 `page.locked`（S2 的样子，第 0 节），所以这一行只有 S3 的预检过得了 |
| `TestTheServersLimitsComeAfterThePrechecks`（同上） | 三 | 2.1 第 7 条的撤销的次序，在一个自己的程序里。**解析的预算**：`newAcmeTeamWith` 的 `c.Page.ParseBudgetBytes, c.Page.ParseMaxWait = 64<<10, 300*time.Millisecond`，预算由另一本普通笔记本里一个等着页锁的大写入占着（`whileAWriteWaitsForItsLock`，`bootstrap/page_content_test.go:363`；照 `busyWrites` 的写法，`mcp_write_codes_test.go:155-190`）。在这之前，LLM Wiki 里 `Notes` 有约 1 KiB 的正文，alice 经 MCP 的 `batch [edit Notes.md "x", move raw.md → sources.md]` 写下变更集 C（它的撤销写回 `Notes` 的正文，要解析）。预算占着时：bob（写者）`revertChangeset C` 立刻答 403 `page.source_admin_only`（检查在解析之前）；反向对照：alice 撤销 C 答 503 `server_busy`（解析会等满 300 ms）。上传的次序不在这里：它是结构的，由 `asset/app/upload_test.go` 的一行核对（3.25；第一轮复核去掉了磁盘的一半） |
| `TestTheReservedTitlesAreNumberedAtAnLLMWikisRoot`（`bootstrap/wiki_numbering_test.go`） | 三 | 导入一个根下有 `index.md`、`Index/a.md`、`log/b.md` 的库：`index 2`（`a` 在它之下：文件夹并进同键的页）、`log 2`（`b` 在它之下），报告两个 `renamed` 问题；同一个库导进普通的笔记本不加序号；父节点在回收站里的 `log`，同步的恢复落到根下成为 `log 3`（`log` 是保留的，`log 2` 是导入的）、答 `renamed: true`；后台的恢复（`startTrashRestore`）一项，它的根落到根下而叫 `index`：`index 3`（同上），`counts.renamed` 是 1。都在同一本笔记本里：保留的键与兄弟的键一起被当作已被占（`restoredNames`，`unit_restore.go:177-185`；后台的 `under`，`:298-305`、`:386-395`），所以序号跳过两者（审查 B2-M1）；`checkPages`（3.27 的新规则） |

S3 的网页、e2e：4.3 与第 5 节（S3 没有新的故事；TR1、TR2、TR3、TR4 加 `refusal: null`）。

### 3.29 每个新检查的反向对照

13.4 第 1 条：每个新的检查都有一个让它失败的反例，证明它查的是它说的东西。反例不改仓库里的东西就能跑的，写成测试里的一行（改过的字符串、一个 SQL 的种子、一个替身）；要改实现才能看到的，列在 7.3 的变异里。

| 检查 | 反例 | Step |
|---|---|---|
| `notebooks_mode_check`、`notebooks_source_root_check` | `'upgrading'`、`'LLM_WIKI'`、`source_root_id = NULL` 的 LLM Wiki 各答 23514 与约束名（3.2） | S1 |
| `TestTheNotebooksModeGoesDown` | 同一条查询在 `downTo` 之前数到两列（测试先看到它之后要看不到的东西，照 `changesets_moves_test.go:135-140`）；Down 漏删一列时数到 1 | S1 |
| 读笔记本的五条查询都带两列 | `TestTheNotebooksAnswerTheirMode`：手写的转换漏了 `Mode` 时那一条读到空的模式（3.3） | S1 |
| `CheckDraft` 的两个新值 | `"fr"`、`"LLM_WIKI"`、`""` 各答 `invalid_format`；`mode: "standard"` 带 `"fr"` 照样答 422（两种模式都核对，00-M10 4.4） | S1 |
| `notebook.New` 没有播种者、写入任务时 panic | `TestTheModuleRefusesNoSeeders`、`TestTheModuleRefusesNoWritingJobs` | S1 |
| 恰有一个播种者交回那个来源根 | 3.5 的表的五种错的交回各答 500、什么也没写；假的 `CreateNotebook` 照表的检查拒绝，建 LLM Wiki 不取来源根时 `TestCreateNotebookMakesTheCallerItsAdmin` 失败（S1 审查 A-N2） | S1 |
| 播种者的问题是缺陷 | 替身播种者交回 `page.protected`：答 500、不是 409，错误的文字带这个码（`TestTheSeedersRunInTheirOrder`）；交回 `context.Canceled` 时照原样（3.5） | S1 |
| `Writer.Seed` 要求在事务里、给了时刻 | `TestASeedRefusesNoTransactionAndNoTime` | S1 |
| `Writer.Seed` 不读时钟 | `steppingClock` 被读一次就让 `TestASeedReadsNoClock` 失败 | S1 |
| 模板在启动时解析一次 | `TestTheTemplateSeedsAnLLMWiki` 在三次创建之后仍数到四次解析；`TestASeedPassesTheFactsItIsGiven` 以另一份正文的事实 `Seed`，观察者收到交来的（先核对两份事实不同）（3.6、3.8；S1 审查 A-M2） | S1 |
| 模板的事实是它的正文的 | `TestATemplatesParseIsItsContents` 以随正文变的扩展 `pageMark()` 解析，`page-mark` 是正文的字节数；不看正文的 `ParseTemplate` 让它失败（3.6；S1 审查 A-M3） | S1 |
| 模板交回写下的来源根 | `TestTheTemplateAnswersTheSourceRootWritten`：`Seed` 以别的 id 写下 `raw` 时答那个 id，两页只交回一个 id 是缺陷；`TestATemplatesParseIsItsContents`：没有 id 的一页答它写下的新 id（3.6、3.8；S1 审查 A-N3） | S1 |
| 模板的回滚是单元写到一半的回滚 | 触发器只拦 `AGENTS`、记下见过的插入：日志带触发器自己的话、序列的 `last_value` 是 2（`raw` 写过）；普通的笔记本照常 201（3.16） | S1 |
| `shared.ClientOf` | 没有 `Actor` 的 ctx 答 401；会话与 PAT 的 `Actor` 两种各对 | S1 |
| 模板的段落对应 | 块数：少一段、末尾多一段；行内代码：少一项、一处行内代码不同；每种块的形状：两项写在一行、标题降一级、围栏少一行、段落拆成两行（3.9；S1 的偏离 14、审查 A-M1） | S1 |
| 模板里代码之外没有链接与标签 | 正文里的 `[[raw/x]]`、`#ingest`（3.9） | S1 |
| 模板至多 8 KiB、不是 frontmatter | 以写出的 `8 << 10`（不取 `MaxTemplateBytes`）：恰 8 KiB 的照收，8,193 字节的、以 `---` 起头的经同一个函数各答错误（S1 审查 C-N3） | S1 |
| 降级的码的次序 | 写者在普通的笔记本上答 403 不是 409；管理员在有任务的普通笔记本上答 `notebook.not_llm_wiki` 不是 `transfer.busy`（3.10） | S1 |
| 降级在锁下读模式 | 找到时 `llm_wiki`、锁下 `standard`：答 409、没有写（3.10） | S1 |
| 降级写下执行者 | 行先由 bob 改过：alice 降级之后 `updated_by_id` 是 alice，回滚的仍是 bob（`TestADowngradePassesTheModeSubscribers`，3.5；S1 审查 B-M2） | S1 |
| `checkPages` 的两条新规则 | `TestPageViolationsCatchEachLLMWikiGap` 的各个 SQL 的改动（含 `raw`、`AGENTS` 改成附件，`AGENTS` 移到 `raw` 之下：S1 审查 B-M1 = C-M1），与照收的三行（没改的、普通的、最后删掉的 LLM Wiki）（3.14、3.27） | S1、S3 |
| 矩阵的按行的工作区 | 一行 `wiki: true` 而目标在 `lab`，`TestMatrixViolationsCatchesEachGap` 报出（3.14） | S1 |
| 模式的事件的最后一跳 | 组合根不登记它时 `TestTheModeEventReachesTheStreams` 失败；看不到这本笔记本的流的下一帧是标记（`quiet`） | S1 |
| page 的模式的转换 | `TestPageNotebooksConvertTheState`：找不到的答 `ok` 为假与零值；丢掉 `SourceRootID` 时 LLM Wiki 的一行失败（3.12；S1 审查 C-N4） | S1 |
| 守卫放过普通的笔记本 | `TestTheModeGuardLetsAStandardNotebookBy` 交给守卫的是 LLM Wiki 会拒的步，守卫与 `CheckContract` 都不看模式时被拒（3.20） | S2 |
| `CheckContract` 的每一格 | 3.21 的表，普通的笔记本对每一行答零；子页下的 `index`、祖先的改名、同一父节点下排序祖先放行，把祖先移到别的父节点下要管理员 | S2 |
| `Grants` 一个单元问一次 | 一个单元里两步都要 `page.manage_source`（`batch` 两次移动 `raw`），在事务里只问一次（`TestTheModeGuardAsksOnceAUnit`，假的授权者的调用带 `" in tx"`）；两个单元问两次；数据库的错误不记下、下一次再问（`TestAGrantIsAskedOnceAUnit`） | S2 |
| `[mode, lock]` 的次序 | `TestModeComesBeforeTheLock` 的两半：降级之前答 `page.protected`、之后答 `page.locked` | S2 |
| `Refused` 不改答复 | handler 的表以标出的错误答出同样的码；`TestTheRegistrantsRunInTheirOrder` 的 `errors.Is` 照过 | S2 |
| 守卫标出它拒绝的节点 | `TestTheModeGuardMarksTheNodeItRefuses`：被拒的节点（`raw`）不是步的第一个节点（`Notes`）的删除与移动，各以 `raw` 标出（S2 审查 C-M1，第 83、84 条变异） | S2 |
| `batch` 的 `failed` 先认 `Refused` | 一个在 `replans` 里的码：标出的答在它的项上，没标出的照旧重新计划（3.22） | S2 |
| `batch` 的计划的预检 | 写者的 `[move raw, delete AGENTS]` 答第 0 项；第 1 项读的页过了上限时仍答第 0 项；没有单元、没有锁；`AGENTS` 被别人编辑而写者的第 0 项被拒时答第 0 项的 403、不答 `page.locked`（S2 的偏离 14）（3.22） | S2 |
| `Refuse` 的两种行为 | 同一项上批量自己的界线（项数、移动带着的节点）让给拒绝：答那一项的 `page.source_admin_only`；清掉 `final`：更早的一项的正文的失败照旧在先（`TestThePlanChecksTheContract` 的三行，S2 审查 A-M1 = C-M2，第 81、82 条变异） | S2 |
| 只有守卫拒绝的情形 | `TestTheGuardRefusesWhatThePlanLetBy`：计划放过、守卫在锁下拒绝；`guards: []` 的程序里同样的交错让移动成功（3.24） | S2 |
| `mcpToolCodes()` 的新行 | `toolError` 遇到行里没有的码就失败；`TestTheMCPToolsErrorsCarryTheirCodes` 遇到行里有、没有一个调用答出的码就失败 | S2 |
| `x-problem-codes` 的新码 | `apitest.Main`：模块的适配器测试没答出就失败，答出了没声明的也失败 | S2、S3 |
| S3 的三处预检 | 3.28 的模块测试：不加锁的两处（上传的 `Check`、`revertChecks.check`）在没有单元、没有锁的情况下答码；锁下的一遍在第一步之前答码，什么也不拒的替身守卫一步也没看到（`TestARevertsChecksRefuseTheContract` 的锁下的一行，第 53 条变异） | S3 |
| 撤销的事实跟着前面的操作 | `TestARevertsFactsFollowItsSteps`：移动后的删除不再带走已经移出的来源根，恢复的后代进入后面的移动与删除，临时改名后的名字供后步读取；去掉 `sim.place` 被抓到，纯正文的无树对照照过（第 85 条变异） | S3 |
| 预检在服务器的上限之前 | `TestTheServersLimitsComeAfterThePrechecks`：被拒的撤销在预算占着时立刻答 403，放过的同一个撤销答 503；`TestCheckDecidesTheNodeThenTheFreeSpace` 的一种：page 的 `Check` 答 `page.virtual_file` 而磁盘不够时答它；HTTP 适配器的 `checkErr` 一行：不调 `Store` | S3 |
| 预检与守卫同码 | `TestPrechecksAnswerTheGuardsCodes`；同步的撤销的前一步碰到锁、后一步被模式拒绝时答模式的码（S2 审查 A-M3，3.28） | S3 |
| `treeError` 的次序与幂等 | `TestTreeErrorKeepsTheLockFirst` 的四行 | S3 |
| 后台第二批拒绝的组合根最后一跳 | 真正的注册表不登记模式守卫时，`TestTheSourceRootsPutBackIsRefusedInALaterBatch` 用完所有重试仍得不到要求的 refused 而失败（第 88 条变异） | S3 |
| `unitFailure` 的 `refused` 在 `forbidden` 之前 | 一个标出的 `page.source_admin_only`（`KindForbidden`）答 `refused`、不是 `forbidden` | S3 |
| 报告的 `refusal` | 后台与导入各一行；别的失败的报告 `refusal` 为 `null`；存储与 HTTP 的往返收一个自由的码 | S3 |
| 报告的拒绝只属于最终的 refused | `TestAJobsRefusalBelongsToItsFinalFailure` 的普通拒绝与 timeout/interrupted/cancelled 四行；只清掉后三种的 refusal，四行都保留第一批与进度，第 86 条变异抓到漏清理 | S3 |
| ListJobs 保留拒绝的码与节点 | `TestARefusedJobsReportReadsBackAsWritten` 同时读详情与列表；列表投影去掉 refusal 时已知码/未来码的列表断言失败（第 87 条变异） | S3 |
| 加序号只在 LLM Wiki 的根下 | 子页下与普通的笔记本里不加（3.27） | S3 |
| 网页的码的文字 | `problem-messages.test.ts`：契约里多一个码就失败；`Messages` 的类型让少一种语言过不了 tsc | S1–S3 |
| `failures` 的类型表 | 不加 `refused` 过不了 tsc | S3 |
| 报告里网页不认得的码 | `transfer-background.test.tsx` 的 `page.some_later_code` 的一条写出 `problem.other` 带这个码 | S3 |
| `mode` 的处理的最后一跳 | 经应用的表的测试：表里没有 `mode` 时记下读的 `answers` 什么也没记下 | S4 |
| 降级的答复留在列表里 | `notebook.store.test.ts`：降级之前发出、之后答 `llm_wiki` 的读，列表仍是 `standard` | S4 |
| 降级拒绝的重读不被更旧读覆盖 | `a read answered after downgrade reconciliation` 的六个参数化测试实例：`forbidden`、`notebook.not_found`、`notebook.not_llm_wiki` × present/absent，重读先完成、更早的读后到；保留目标字段或缺席、无关条目与原拒绝语义，不标删除。第90条变异抓掉只在拒绝重读时漏计的 changed（S4审查B-I1） | S4 |
| 降级的焦点与持有的对话框 | 403 之前 Ada 已改成编辑者：重读之后按钮没了，对话框仍写着拒绝；成功之后焦点在"Mode"标题上（`general-page.test.tsx`） | S4 |
| 树的菜单与拖放 | 普通的笔记本里叫 `AGENTS`、`raw` 的页照旧全有、照旧可拖；不能拖的行没有 `draggable` 属性；同一棵树的笔记本换成普通的之后 `AGENTS` 的行有了 `draggable`（第 64 条变异） | S4 |
| 树的两行的可读的名字 | jsdom 的 `textContent` 看得到后缀前的空格（第 62 条变异）；LW1 (page)、LW2 (page) 按 Chromium 里算的确切的名字找，后缀被藏出可访问性的树时等不到（第 63 条变异；4.4） | S4 |

### 3.30 S1 的实施对设计的修正

S1 的审查与修复核对见 [S1 的审查记录](reviews/P1-S1-mode-template-review.md)，结果在 10.1。与上文最初的设计不同之处已改在各节里：

- **S1，实施时定下的**（实施者的 15 条偏离，S1 审查 A、B、C 各在自己的范围里判过，A 判第 1–3、14 条，B 判第 5–13 条，C 判第 4、15 条，都对；第 3 条的理由照 A 改正）：没有 `TestCreateNotebookLogs`，建的一行在 `TestCreateNotebookMakesTheCallerItsAdmin` 里核对 `mode`（1，3.10、3.14）；变更集的说明是 `wiki/domain` 的 `TemplateMessage`（2，3.1、3.8）；`TestLLMWikiCreationWritesItsTemplate` 另数 `indexed_pages`，是链接索引的最后一跳（3，3.16）；`TestPageViolationsCatchEachLLMWikiGap` 在 `interleavings_page_test.go`，每一行改回之后再断言为空（4，3.14）；没有 `page/domain/mode_test.go`（5，3.1）；`mode_test.go` 的调用列表照已有的替身、不带 id（6，3.10）；矩阵的变体以它自己的表种进 `seeded` 的 `wiki`（7，3.14）；另两个读每个工作区的测试读 `wlab`（8，3.1、3.14）；`targetViolation` 取行的列所在的工作区的 slug，`workspaceOf(c)` 留给各行的请求（9，3.14）；变体的 `raw` 与 `AGENTS` 是英文的模板的两页（10，3.14）；`TestImportsAndDowngradingAnLLMWiki` 两种次序都持住导入的 worker（11，3.16）；`TestTheModeEventReachesTheStreams` 的私有的 LLM Wiki、读者与看不到它的成员（12，3.16）；假的 `SetMode` 拒绝表拒绝的，修复一轮另加 `CreateNotebook`（13，3.1）；`TestTheTemplatesCorrespond` 加"末尾多一段"，第 4 步改正、第 4–7 步重放（14，3.9、3.29）；LW1 (API) 以 `Promise.all` 建三本（15，第 5 节）。
- **S1，审查与修复核对带来的**（编号是 S1 的审查记录的；主持者的决定见审查记录的第 23–26 条）：`checkPages` 的规则的"根下的 `AGENTS`"有反例（B-M1 = C-M1，3.14、3.29、7.3 第 65 条）；降级写下的执行者有测试（B-M2，3.5、3.29、7.1、7.3 第 66 条）；段落对应的每种块的形状各有反例（A-M1，3.9、3.29、7.3 第 67–70 条）；"在启动时解析一次"在 wiki 与 page 两处钉住（A-M2，3.6、3.8、3.29、7.1、7.3 第 71、72 条）；模板的事实是它的正文的（A-M3，3.6、3.29、7.3 第 73 条）；变更集经 `coalesce` 读（A-N1 = C-N2，3.16、7.3 第 12 条）；假的 `CreateNotebook` 照表拒绝（A-N2，3.1、3.29、7.1、7.3 第 74 条）；`Seed` 交回写下的页的 id，模板交回写下的来源根（A-N3，3.4、3.6、3.8、3.29、7.3 第 75–77 条）；模板的单元在排着的删除之后重入工作区行的子测试，钉住 PostgreSQL（A-N4，3.15、7.2 第 8 条）；降级之后再读 `notebooks`（B-N1，3.16、7.1）；`createNotebook` 的说明不重复（B-N2 = C-N6，3.13）；8 KiB 写在测试里（C-N3，3.9、3.29、7.3 第 78、79 条）；`pageNotebooks.StateOf` 的"找不到"与 `open_notebook` 在门之后被删的答复（C-N4，3.1、3.12、3.29、7.1、7.5、7.3 第 80 条）；第三份的估计算上 `wlab` 的种（C-N5，7.5）。没有交给 v0.1 之后的打磨的。

### 3.31 S2 的实施对设计的修正

S2 的审查与主持者的核对见 [S2 的审查记录](reviews/P1-S2-guard-review.md)，结果在 10.2。与上文最初的设计不同之处已改在各节里：

- **S2，实施时定下的**（实施者的 15 条偏离，S2 审查 A、C 各在自己的范围里判过：A 判全部 15 条，C 判第 4、6、9–15 条，都对）：notebook 的存储的 `ShareNotebook` 答 `(domain.Mode, uuid.UUID, error)`，根由它建 `State`（1，3.18）；事实的位置叫 `NodePlace`（2，3.17、3.21）；`ReservedKeys()` 是函数（3，3.17、3.21）；`page.manage_source` 落在第 2 个提交，第 1 个提交的 `TestAGrantIsAskedOnceAUnit` 问 `node.move`、`node.rename`（4，3.19、6.2）；`pageSessionSubscribers()` 不收连接池（5，第 0 节、2.1 第 6 条、3.17、3.20、第 8 节）；MCP 的 `move` 发 `from`，`wikiCodeCalls` 的写者是经 SQL 加进 acme 的 erin（6，3.22）；`uploadAsset` 与 `revertChangeset` 的说明的新句（7，3.22）；`decidedNotebook`（8，3.17、3.22、6.2）；树的一遍在每一项自己的方法里判（`judge`），一项的拒绝在同一项上批量自己的界线之前（9，3.22）；`Refuse` 取代同一项上的失败、清掉 `final`（10，3.22）；没有钉住整个调用列表的 `batch` 测试，替身记 `"StateOf"`（11，3.17、3.22、6.2）；`TestTheModeGuardAsksOnceAUnit` 的拒绝的一行是计划放行、锁下拒绝，替身授权者默认答 `ErrNotVisible`、加 `failing`（12，3.19）；B-Q1 的一行是同一父节点下的改名与移到原处（13，3.22）；`TestABatchsPlanRefusesBeforeItsLock` 加"写者的 `[move raw, edit AGENTS]`，AGENTS 被持着"（14，3.22、3.29）；第 23、29、32、40 条变异的写法（15，7.3）。报告里写了、没进编号的一处：`TestTheModeGuardReachesEveryMCPWrite` 的 agent 的变更集按 `client LIKE 'mcp%'` 找（3.24）。照代码另写明的一处：`batch` 的计划没有 `Change`，以 `nodePlace` 照 `FactsOf` 的写法从模拟的树建 `NodeFacts`，不经 `FactsOf`（3.21）。
- **S2，审查与主持者带来的**（编号是 S2 的审查记录的；主持者的决定见审查记录的第 27–31 条）：守卫标出的节点有测试（C-M1，3.17、3.20、3.29、7.1、7.3 第 83、84 条）；`Refuse` 的两种行为有测试（A-M1 = C-M2，3.22、3.29、7.1、7.3 第 81、82 条）；两个新的 S2→S3 的窗口（A-M2、A-M3，第 0 节、3.21 的表、3.22 的说明、3.23、3.28、3.29、第 9 节）；MCP 的 `write` 覆盖已有的页（C-N1，3.24）；`ApplyBatch.Execute` 的次序的注释（A-N1，只在代码的注释里，3.22 本就这样写）；第 4 个提交的 sha（C-N2，10.2）；建成的 CI 的红不是 S2 的，grep 的守卫再失败时的改法记作 M10 的收尾的一项（决定 30，10.2，00 号文档"写给后面的 M"）；`ErrSourceAdminOnly` 的文字留给 P2/S1（A 的存疑，第 9 节）；第 8 节另列总体设计 13.1 第 29 条的 `pageRegistrants(pool)`。没有交给 v0.1 之后的打磨的。

### 3.32 S3 的实施对设计的修正

S3 的审查与修复核对见 [S3 的审查记录](reviews/P1-S3-prechecks-refusals-numbering-review.md)，结果在 10.3。与上文最初的设计不同之处照实改在各节，逐项与依据如下：

1. **条件的工作区读取**（任务审查 Q-A1，主持者的决定 32）：每次上传、锁外撤销仍只新增一次 `StateOf`；锁外撤销需要 `page.manage_source` 时还在这一次的 `Grants` 回调里读 `WorkspaceOf`，同一遍至多一次。`Allowed` 不保留工作区，真实授权仍须完整目标；锁下用自己的 `Grants` 重问。3.25 与 7.6 不再把它概括为只有一次新增点查。
2. **事实的共用位置**：纯领域的 `RevertFacts` 复用调度器的模拟，按计划逐步更新树并经 `FactsOf` 建事实；应用的 `checkRevertContract` 给两遍共用，普通模式在建事实前返回。抽出 `orderedSubtree` 保持原调度次序，没有复制调度器。3.25 写明；第 85 条变异去掉 `sim.place` 后，后续删除/移动读旧树的三处断言失败，纯正文对照通过。
3. **仓库的写法**：替身实际记 `"StateOf"`、不带 nb（延续 S1/S2），Go 的零 UUID 是 `uuid.UUID{}`，保留键从 `ReservedKeys()` 取得。3.25、3.27 与 6.3 按实际签名和调用记录改，不改变契约。
4. **持久化与 HTTP 测试的安排**：指定的两个文件里新增相邻的 `TestARefusedJobsReportReadsBackAsWritten`、`TestARefusedJobReadsWithItsReport`，让拒绝元数据与 report 的 failure 相符；原冲突/锁往返保留并重跑。详情、列表、旧报告、显式 null 与未来码分别核对。3.28 照实改写。
5. **最终结果的 refusal 清理**（B-M1）：原实现记录拒绝之后，超时、服务停止或取消覆盖 failure 时漏清旧 refusal。`ba5cded6` 在共用的 `jobRun.finish` 最后选择 failure 之后、持久化之前清掉非 refused 的 refusal，四行回归保留第一批计数和进度，原 B 复核通过，第 86 条变异抓到漏清理。3.26、3.28、3.29 与 7.3 增补；这是兑现原来的 null 契约。
6. **文档的时机与缺项**：第 8 节按现行工作流 3.5–3.6 改为 Step 分支先写、合并后填提交号；00 的头仍写“S1 开工”的滞后改正。总体设计 13.4 第 4 条补 S2 已实现而漏记的交错 5–6，并加 S3 第 7 条，分别写实际的检查，不把 S3 的 `checkPages` 说成还有 `checkNotebooks`。历史审查记录不改；00 第 11 节的独立设计审查编号不追加一个重复的 32。

### 3.33 S4 的实施对设计的修正

S4 的当前代码与证据见 [S4 审查记录](reviews/P1-S4-web-review.md)，结果在 10.4；三份原始完整 Step 独立审查及修复后的范围核对已由主持者接受；B-I1（Important，测试覆盖）已关闭，没有剩余核实的 Important、Minor、Nit 或 Q。原始审查头为a0，最终接受头为13ee；B由原审查者重放核对，A/C由主持者按工作流3.4的测试修复例外核对未变范围，记录分别保存。 已知的实际安排如下，不把测试编写期间的失败算作产品回归或变异成功：

1. 主持者的 R-M1（Minor）：创建测试的 `creationServer` 不能只回显 `mode` 却继续给 LLM Wiki 一个 null 来源根；`a0aa88dd` 的单行修复为 LLM Wiki 给非零根 UUID，普通新建仍给 null。已有 20 项创建/导航测试与 lint-web 通过，主持者复查；真实模板与库由 LW1 核对（4.4）。这是夹具忠实度修复，没有新产品决定、生产改动或新增编号变异。
2. LW1 的两种页面语言在首次导航前仅对该工作区的 notebooks 端点安装 `route.continue()` 的透传路由，使 Chromium 的 `response.request().postDataJSON()` 可观察；请求、正文、答复均不替换，精确 body 断言保留。原来观察到 null 请求体，认证克隆不是已证实的原因；这个窄夹具安排由主持者接受（5.2）。中文 `test.use({ locale })` 只罩住自己的 `describe`。
3. `notebookServer.notebookReads` 独立计读，不改变旧 `sent` 的语义；store 增补无问题码 HTTP 错误及重读失败/看不到的控制。真实 `mode` 的应用测试等初次连接安定后使用记录过的实际列表答复，另外核对工作区/锁日志为空、流身份不变（4.4）。
4. LW12 的导入清理先放开 B 的锁，再 `Promise.allSettled` 等所有已拿到的任务；原错误优先，清理错误在已有错误时注记、否则在 finally 之后抛出。页面版本用另一个账户的独立上下文和流，核对列表重读、文档标记与流计数不变，不靠重载或重新连接证明更新（5.2）。
5. 第 57 条按不 put 与不计 changed 分成 a/b；第 63 条按 aria-hidden 与 hidden 分成 a/b。两条 LW1 页面与 LW2 页面都是 63 的指定 catcher；纯 textContent 的第 62 条先等不带后缀的 Actions 按钮，再读真实空格，不以另一处后缀查询超时代替字面量断言。原始候选 `a0aa88dd` 的 13 个逻辑变体与 20 次指定 catcher 已重放；修复头的完整重放也已完成，见10.4。审查新增两个永久逻辑变体：89（A-X1，根下 AGENTS 限制，由已有测试抓到）与90（B-X3，拒绝 reconciliation 不计 changed，由新增六个参数化测试实例核对）。54–64原编号保持；最终15逻辑变体/27次指定 catcher 的重放结果另记。

6. B-I1（Important，测试覆盖）：生产 refusal reconciliation 的 `put/changed` 本来正确，原永久测试未覆盖三个拒绝码各自 present/absent 的旧读交错。`13ee0bc8` 仅补六个参数化测试实例；先完成重读、再释放更早的列表读，目标模式/来源根/角色或缺席、无关条目、不记删除与原拒绝语义都核对（3.29、4.4，第90条变异）。`13ee0bc8` 只在 `notebook.store.test.ts` 新增51行、六个参数化测试实例，生产代码原本正确且未改。原审查者B在此精确头以同一命令重放原B-X3：59通过（3.327s）→恰六个新实例断言失败、原53个通过（3.375s）→原字节恢复后59通过（3.369s）；全部是已提交的永久测试，没有scratch实例或重试，最终树干净。主持者读完整报告与原始证明后关闭B-I1；`s4-full-review-b-fix1.md`、`s4-review-evidence/b/fix1-prepared/proof.json` 与 `s4-root-final-review-acceptance.json` 保存处置。
7. C-X2 在保持 LW12 原清理结构的外部故事中实际注入主错误与 drain 错误：锁先释放、两个真实任务 drain 后SQL均 succeeded，清理问题注记而原错误仍是失败；原故事未改。此证据不再只限于代码阅读，也不充作新增产品变异。

## 4. 网页

`W/` 是 `web/apps/web/src/`。每个 Step 都要过网页的五步：Lint（含每个包的 `tsc`）、`gen-check-web`、`make knip`、`test-web`、`build-web`（`.github/workflows/ci.yml:151-161`；00-M10 4.16 与第 7 节列的网页的一步漏了 knip，2.1 第 12 条）。所以服务端的每个 Step 带上它改动的契约在网页上要的类型、假的服务端的字段与码的文字，不留到 S4。

### 4.1 S1 随服务端的部分

- **生成的类型**（`make gen-web`）：`Notebook` 多了必填的 `mode`、`source_root_id`，`NotebookCreate` 多了可选的 `mode`、`template_locale`，新的操作 `downgradeNotebook`，`EventMode`。生成的文件经 `export type *` 再导出（`web/packages/api-client/src/index.ts`），knip 不报。`W/services/event.service.ts` 的导出**不在** S1：S1 里没有人导入它，`make knip` 照默认报出未用的导出类型（knip 6.38.0 的默认问题里有 `types`，`node_modules/knip/dist/constants.js:207-225`），所以它与 `mode` 的处理同在 S4（M6 的 `EventLinks` 也是在网页的 Step 里与处理一起导出的，`52ac9870`）。00-M10 第 7 节把它放在 S1，2.1 第 12 条改。
- **两处完整的字面量**（其余 39 处 `...notebookJSON` 随之继承）：`W/test/fakes.ts:91-100` 的 `notebookJSON` 加 `mode: "standard", source_root_id: null`；`W/stores/notebook.store.test.ts:7-17` 的 `notebook()` 同样加。别的 `Notebook` 都来自服务端（列表、建、改、接手），没有生产代码写字面量。
- **码的文字**：`notebook.not_llm_wiki` 进 `W/app/problem-messages.ts:11-66` 的 `problemMessages`（`"notebook.not_llm_wiki": "problem.notebook.not_llm_wiki"`），文字：
  - en："This notebook is not an LLM Wiki."
  - zh-CN："这本笔记本不是 LLM Wiki。"

  `problem-messages.test.ts` 读打包的契约、要每个操作的每个码有一条（`:55-62`），两种语言由 `Messages` 的类型与 `messages.test.ts:14-18` 守着；`downgradeNotebook` 的另外三个码（`notebook.not_found`、`forbidden`、`transfer.busy`）已有文字。
- S1 的网页不发 `mode`：建笔记本的对话框照旧（`notebook-nav.test.tsx:182` 钉住的请求体在 S4 才变）。

### 4.2 S2 随服务端的部分

三个码进 `problemMessages`，文字：

| 码 | en | zh-CN |
|---|---|---|
| `page.protected` | "An LLM Wiki keeps its AGENTS page and its source root: neither can be deleted, and AGENTS cannot be renamed or moved." | "LLM Wiki 保留它的 AGENTS 页与来源根：两者都不能删除，AGENTS 也不能改名或移动。" |
| `page.virtual_file` | "This file is generated by the system: the index updates itself as pages change, and the log is the notebook's changes. No page or attachment at the root may be named index or log." | "这个文件由系统生成，修改页面后目录会自动更新；日志就是变更集，也会自动更新。根下的页与附件不能叫 index 或 log。" |
| `page.source_admin_only` | "Only the notebook's admins can rename or move its source root, or rename, move or delete in its source area." | "来源根的改名、移动，与来源区里的改名、移动、删除，只有笔记本的管理员能做。" |

`page.virtual_file` 的中文照总体设计 5.5 与 00-M10 4.9 的那两句。`page.source_admin_only` 的文字写出来源根：P1 里它唯一的情形是来源根自己，而来源根不在来源区里（00-M10 4.7："来源是来源根的严格后代"，审查 B-N9）。页面树的拒绝经 `errorText`（`W/pages/notebook/page-tree.tsx:140-144`）、改名与移动的对话框经它们的表单显示这些文字，不改组件。

### 4.3 S3 随服务端的部分

- **类型**：`TransferFailure` 多了 `refused`，`TransferReport` 多了必填、可为 `null` 的 `refusal`（`TransferRefusal`）。
- **`W/pages/notebook/transfer-report.tsx`**：`failures`（`:23-44`，`Record<TransferFailure, PlainKey>`，少一个 tsc 就失败）加 `refused: "transfer.failure.refused"`；报告在 `refusal` 不为 `null` 时写出一段 `Refused`（照 `Locked`，`:285-310`）：拒绝的码的文字，再按 `distinctName(pages.tree, notebook, refusal.node_id, t)`：节点在树里时写"被拒的是“{name}”"与打开它的链接（`transfer.openPage`），不在时写一句不带名字的。`refusal.code` 在契约里是自由的字符串（3.26 第 6 条），所以码的文字照 `errorText`（`W/app/problem-messages.ts:115-118`）：`Object.hasOwn(problemMessages, code)` 时取它的文字（P1 的三个码都有、都没有占位符），不认得的码写 `problem.other` 带这个码（之后的 M 加的码在网页更新之前也答得出一句）。
- **文字**：

  | 键 | en | zh-CN |
  |---|---|---|
  | `transfer.failure.refused` | "The notebook's rules as an LLM Wiki refused one of its writes." | "LLM Wiki 的规则拒绝了它的一步写入。" |
  | `transfer.refusedTitled` | "It was refused at “{name}”." | "被拒的是“{name}”。" |
  | `transfer.refusedSome` | "It was refused at a page no longer in the tree." | "被拒的那一页已不在树里。" |

- **假的**：`W/test/jobs-server.ts:35` 的 `reportOf` 加 `refusal: null`（每个网页测试的报告都由它或 `jobJSON(...).report!` 展开）。
- **测试**：`transfer-report.test.ts` 的 `failureText` 表加 `refused`；`transfer-background.test.tsx`（照 `:387-412` 的 `page_locked` 那一条）加"一次撤销以 `refused` 停下，报告写出 `page.source_admin_only` 的文字与 `raw` 的链接"，与节点不在树里的那一种，与码是网页不认得的 `page.some_later_code` 的那一种（写出 `problem.other` 带这个码）。

### 4.4 S4：网页（一）

**建笔记本的对话框**（`W/pages/workspace/create-notebook-dialog.tsx`）：

- 名称之后、开放程度之前加一组 `ModeOptions`（新，`W/pages/workspace/mode-options.tsx`），照 `AccessOptions`（`W/pages/notebook/access-options.tsx:8-55`）：`<fieldset>`、`<legend>`、原生的单选、每项一句由 `aria-describedby` 连上的说明；"普通笔记本"在前、默认选中。`useState<NotebookMode>("standard")`。
- 发出的请求体：`{ name, workspace_access, mode, template_locale: preferences.locale }`，`template_locale` 两种模式都带（00-M10 4.4："网页总是带上"），取 `useStore().preferences.locale`（`"zh-CN" | "en"`，与契约的枚举一一对应，`W/i18n/locale.ts:1`；读法照 `transfer-report.tsx:190`）。新手引导（`W/onboarding/notebook-step.tsx:56-90`）照旧只发名称与开放程度：它建的是普通的个人笔记本，服务端照默认（00-M10 4.4"新手引导不变"读作请求也不变）。
- `fields` 照旧是 `["name"]`（`:80-81`）：模式与语言的 422 由单选给不出（同开放程度），落到横幅。
- 文字：

  | 键 | en | zh-CN |
  |---|---|---|
  | `notebooks.modeLegend` | "Mode" | "模式" |
  | `notebooks.modeStandard` | "Standard notebook" | "普通笔记本" |
  | `notebooks.modeStandardHint` | "Pages you write and organize yourselves." | "你们自己写、自己整理的页。" |
  | `notebooks.modeLLMWiki` | "LLM Wiki" | "LLM Wiki" |
  | `notebooks.modeLLMWikiHint` | "Agents keep a wiki from the sources you add. It starts with two pages in this page's language: raw for the sources, AGENTS for the rules." | "agent 依据你们添加的来源维护一本 wiki。它从两页开始，用这个页面的语言写成：放来源的 raw，写约定的 AGENTS。" |

- 测试（`W/pages/workspace/notebook-nav.test.tsx`）：`:182` 的请求体改为 `{ name: "Field notes", workspace_access: "viewer", mode: "standard", template_locale: "en" }`；新的"选 LLM Wiki 在中文界面里建的请求带 `mode: "llm_wiki"`、`template_locale: "zh-CN"`，建好之后照旧打开它的主页"；`:171` 的"Private 选中"照旧（单选的名字不同）。`creationServer`（`:142-155`）回显 `mode`；LLM Wiki 的新建答复带非零 `source_root_id`，普通新建为 null，与真实创建一致（S4 主持者 R-M1，3.33）。

**设置的"常规"一页加"模式"一节**（`W/pages/notebook/general-page.tsx`，`NotebookGeneralPage` 的两节之间：常规之后、删除之前）：

```tsx
<section className="max-w-md space-y-3">
  <h2 ref={heading} tabIndex={-1} className="text-lg font-semibold outline-none">{t("notebookSettings.mode")}</h2>
  <p>{wiki ? t("notebookSettings.modeLLMWiki") : t("notebookSettings.modeStandard")}</p>
  <p className="text-sm text-muted-foreground">{wiki ? t("notebookSettings.modeLLMWikiHint") : t("notebookSettings.modeStandardHint")}</p>
  {downgrades && <Button ref={button} variant="outline" onClick={() => setOpen(true)}>{t("notebookSettings.downgrade")}</Button>}
  <ConfirmDialog held={{ open, onOpenChange: setOpen, onClosed }} tone="default" … />
</section>
```

持有的对话框随这一节一直挂着（关着时什么也不画），不以 `downgrades || open` 为条件：卸下时少一种情形要想、要测（C 的过度设计，主持者照收）。

- `wiki = notebook.mode === "llm_wiki"`，`downgrades = wiki && notebook.role === "admin"`（`:27` 的 `admin`）。读者、写者与普通笔记本的管理员只看到两行文字，没有按钮（`general-page.test.tsx:138-155` 钉住的"读者没有单选、没有保存"照旧成立）。
- **为什么是持有的对话框**：带触发按钮的 `ConfirmDialog` 随按钮卸下（`confirm-dialog.tsx:23-25`），而 403 之后重读的列表让 `downgrades` 变假，触发它的按钮、连同对话框和对话框里的拒绝一起消失。持有的对话框（`HeldDialog`，`W/app/held-dialog.ts:7`）在 `open` 时留着，拒绝照常写在里面。代价是焦点由这一节自己移：`onClosed(done)` 在 `done` 时把焦点移到这一节的标题（成功之后按钮随模式消失，13.2 第 17 条"触发按钮随行消失，焦点移到这一节的标题"），没成功时回到按钮，按钮已不在时到标题。00-M10 4.5 写的是"`ConfirmDialog` 的 `focusAfter`"，2.1 第 23 条照此改（审查 C-M11）。
- **测试**（`W/pages/notebook/general-page.test.tsx`，审查 C-M4）：
  - LLM Wiki 的管理员看到按钮；它的编辑者、读者与普通笔记本的管理员看到两行文字、没有按钮。
  - 成功：`POST …/downgrade` 恰发一次，这一节写"Standard notebook"，按钮没了，`await waitFor(() => expect(document.activeElement).toBe(heading))`（Radix 在一个计时器里给焦点，`notebook-nav.test.tsx:178-186`）。
  - 403：测试在答 403 之前把 Ada 在 `server.members` 里改成编辑者，如另一个标签页所为（`notebookServer` 只在答复成功时改状态，`W/test/notebook-server.ts:87-95`）；重读之后按钮没了，对话框仍开着、写着"Only the notebook's admins can downgrade it."；取消之后焦点在标题上。这一条抓 7.3 的"带触发按钮的对话框"的变异。
  - 409 `transfer.busy` 写出 `transfer.writingBusy` 的文字，没有重读。
  - 409 `notebook.not_llm_wiki` 当作完成关上，焦点在标题上。
- 对话框：`title` `notebookSettings.downgradeTitle`、`description` `notebookSettings.downgradeBody`、`confirmLabel` `notebookSettings.downgradeConfirm`、`sendingLabel` `notebookSettings.downgrading`、`cancelLabel` `notebookSettings.cancel`；没有 `typedConfirmation`（00-M10 第 11 节主持者的决定：什么也不删除）；`tone="default"`；`confirm={() => notebooks.downgrade(notebook.id)}`；`texts={{ "transfer.busy": "transfer.writingBusy", forbidden: "notebookSettings.downgradeAdminsOnly" }}`。`transfer.writingBusy` 已有（en `:751-752` "An import, a restore or an undo is under way in this notebook: wait for it to end, then try again."，zh `:725`），说的正是降级的情形，不说到导出（`problem.transfer.busy` 说到，`zh-CN.ts:915`），不另写新的。
- 文字：

  | 键 | en | zh-CN |
  |---|---|---|
  | `notebookSettings.mode` | "Mode" | "模式" |
  | `notebookSettings.modeStandard` | "Standard notebook" | "普通笔记本" |
  | `notebookSettings.modeStandardHint` | "Its members write and organize its pages themselves." | "它的页由成员自己写、自己整理。" |
  | `notebookSettings.modeLLMWiki` | "LLM Wiki" | "LLM Wiki" |
  | `notebookSettings.modeLLMWikiHint` | "Agents keep its pages from the sources in its source area, by the rules on its page AGENTS." | "agent 依据来源区里的来源、按 AGENTS 页上的约定维护它的页。" |
  | `notebookSettings.downgrade` | "Downgrade to a standard notebook…" | "降级为普通笔记本…" |
  | `notebookSettings.downgradeTitle` | "Downgrade “{name}” to a standard notebook?" | "把“{name}”降级为普通笔记本？" |
  | `notebookSettings.downgradeBody` | "Nothing is deleted. Its source area becomes ordinary pages, which anyone who can write may change, move or delete; AGENTS can be deleted; the generated index and log leave the page tree; a source the same as one there is no longer refused. Making it an LLM Wiki again is another process, an upgrade." | "什么也不会删除。来源区成为普通的页，能写的人都可以修改、移动、删除；AGENTS 可以删除；生成的目录与日志从页面树里消失；与已有的来源相同的新来源不再被拒绝。再变回 LLM Wiki 是另一个流程：升级。" |
  | `notebookSettings.downgradeConfirm` | "Downgrade" | "降级" |
  | `notebookSettings.downgrading` | "Downgrading…" | "正在降级…" |
  | `notebookSettings.downgradeAdminsOnly` | "Only the notebook's admins can downgrade it." | "只有这本笔记本的管理员能降级它。" |

  `downgradeBody` 写的是 M10 做完之后的后果（来源区、虚拟的两行、去重是 P2 与 P4 的）；它们在 P1 与 P4 之间还不全，句子不改（同 3.9 的模板）。"升级是另一个流程"在 M11 做成之后仍然成立（00-M10 4.5）。

**`NotebookStore.downgrade`**（`W/stores/notebook.store.ts`）：

```ts
/**
 * downgrade makes the LLM Wiki id a standard notebook (M10 design 4.5), in the
 * notebook's turn (v0.1 design 13.2, item 1), the answer put in the list. A
 * refusal that tells the list is behind reads the workspace's notebooks
 * again, as leave does, the notebook put back as seen, or dropped, not as
 * removed, when it is not: notebook.not_llm_wiki (another admin downgraded
 * it first) is then done; forbidden (no longer its admin) and
 * notebook.not_found are thrown, the dialog's or the layout's to tell.
 */
async downgrade(id: string): Promise<void>
```

- 经 `inTurn(id, …)`（`:45`），与这本笔记本的改名、改开放程度、删除、离开排在一队（`:100-158`）。成功：`this.put(answered)`（`changed()` 计数，之后出去的旧读被丢弃，`:166-182`）。
- 重读：`this.service.list(this.slug)`（同 `leave`，`:144-156`）；看得到 → `put`；看不到 → 从列表里拿掉而不记进 `removed`（布局显示"找不到"，而不是像删除那样回到工作区的主页，`notebook-layout.tsx:59-61`）；重读失败 → 照抛出原来的拒绝。`transfer.busy` 不重读，照抛出（排着的任务结束之后再降）。
- 构造的 `Pick<NotebookService, …>`（`:48`）加 `"downgrade"`；`NotebookService.downgrade(id)` 发 `POST /api/v0/notebooks/{notebook_id}/downgrade`、答 `Notebook`（`W/services/notebook.service.ts`，`update` 之后）。
- 测试（`W/stores/notebook.store.test.ts`；`storeOf` 的假的服务（`:22-34`）加默认的 `downgrade`，不加 tsc 就失败）：成功放进答复；排在同一本笔记本的改名之后发出（照 `:186-209`）；"a read answered after a downgrade keeps it" 是它自己的测试，不进 `:43-68` 的表（那张表只比名字，降级不改名字，审查 C-M3）：一次读在降级之前发出、之后答 `mode: "llm_wiki"`，`store.list?.map((n) => [n.name, n.mode])` 仍是 `[["Beta", "standard"]]`（`downgrade` 不 `put` 或 `put` 而不计 `changed()` 时失败）；409 `notebook.not_llm_wiki` 重读、答完成、列表里是重读的那一本；403 重读、抛出、列表里是重读的（角色已变）；404 重读、抛出、不在 `removed` 里；`transfer.busy` 不重读、抛出。 另有 "a read answered after downgrade reconciliation" 的六个参数化交错实例（S4审查B-I1）：三个指定拒绝码各测 present/absent，先挂住旧读，再等拒绝重读完成，最后放开旧读；核对目标的模式/来源根/角色或缺席、无关条目、`!wasRemoved`、恰好三次读，403/404抛原对象而409完成。只在拒绝重读分支直接赋列表、不计 changed 的第90条变异须令六个参数化测试实例全部在运行断言失败。

**`mode` 事件的处理**（`W/events/handlers.ts`）：

```ts
/**
 * modeChanged reads the workspace's notebooks again (M10 design 4.5): a
 * notebook's mode, and with it what its pages show, comes from the list,
 * which is keyed by its workspace (v0.1 design 13.2, item 15). Only that:
 * a downgrade writes no page.
 */
function modeChanged(data: unknown, { mutate }: EventContext): void {
  void mutate(["notebooks", (data as EventMode).workspace_id]);
}
```

- 表（`:276-282`）加 `["mode", modeChanged]`；`:9-14` 的注释改为"M5 has pages and lock, M6 links, M10 mode (M11's upgrade sends it too)"。`W/services/event.service.ts` 的 `:1`、`:5` 导入、再导出 `EventMode`。`refreshedOnConnect`（`W/app/event-stream.tsx:15-35`）不加键：`["notebooks"]` 已在第二级（`:17`），断线期间错过的降级在重连时读到。
- 测试（`W/app/event-stream.test.tsx`）：`:204-218` 的替身测试把类型改成编造的 `later`（不改的话 `new Map([...eventHandlers, ["mode", handler]])` 让它的替身盖过真的处理，什么也测不到）；新的"a mode event reads the workspace's notebooks again, through the app's handlers"：经 `open()` 的默认的表（`:44`），发 `mode` `{workspace_id, notebook_id}`，期待笔记本的列表被读了一次。`pageServer` 的列表的路由什么也不记（`W/test/page-server.ts:299`），所以测试经 `open()` 收的 `answers` 换上一个记下 `"GET notebooks"` 的列表的答复（审查 C-N4），断言 `server.sent` 是 `["GET notebooks"]`（M6 的移交的"最后一跳"写法：表里没有它时失败）。

**页面树里的 `AGENTS` 与来源根**（`W/pages/notebook/page-tree-item.tsx`）：

- 行的种类（`TreeContext.notebook` 给出笔记本，`:53-61`）：`sourceRoot = notebook.mode === "llm_wiki" && node.id === notebook.source_root_id`；`agents = notebook.mode === "llm_wiki" && node.parent_id === null && titleKey(node.name) === titleKey("AGENTS")`（`W/lib/title-key.ts:11`，同服务端的 `shared.TitleKey`）。树里只有页（`page-tree.store.ts:165-174`），所以不必再看种类。
- **菜单**（`PageMenu`，`:215-306`，今天写者都有"新建子页、改名、移动到…、删除"，`:256-261`）：`AGENTS` 的行只有"新建子页"；来源根的行：管理员有"新建子页、改名、移动到…"，没有删除（4.9：谁也删不掉它），写者只有"新建子页"。00-M10 4.16 说来源根"管理员全有"，与 4.9 的"来源根的行没有删除"冲突，照 4.9（2.1 第 13 条）。来源区里的页（来源）的菜单是 P2/S3 的。
- **拖放**（`useDrag`，`:163-198`，今天每行对写者都可拖、可放）：`AGENTS` 谁也不能拖，来源根只有管理员能拖；放到它们上面与旁边照旧（移动的是被拖的那一个）。不能拖的行不登记 `draggable()`，只登记 `dropTargetForElements`（`combine` 里少一项）：pragmatic-drag-and-drop 的 `draggable()` 总是写 `draggable="true"`、只在 `dragstart` 时问 `canDrag`（`node_modules/@atlaskit/pragmatic-drag-and-drop/dist/esm/adapter/element-adapter.js:123-128`、`:279-282`），用 `canDrag` 就分不出两种行（审查 C-M5）。测试照读者的那一条（`page-tree-writes.test.tsx:605-616`）核对属性：不能拖的行没有 `draggable`。一行能不能拖由 `notebook.mode`、`notebook.source_root_id`、`notebook.role` 与节点的父节点、名字算出（`canDragRow`），`useDrag` 的效果的依赖是这个算出的值：今天的依赖 `[row, writer, node.id, notebook.id, pages, open]`（`page-tree-item.tsx:163-198`）在降级之后一个也不变（列表重读了，行没有重新挂上），`AGENTS` 的行就一直不能拖到它重新挂上（审查 C2-N4）。
- **图标与文字等价**：`AGENTS` 的行在名字前一个 `Bot`、来源根的一个 `Library`（`lucide-react` 1.48.0），`aria-hidden`；文字等价在链接的可读的名字里（00-M10 4.16："否则读屏的人得不到它唯一要说的事"），放在名字之后，照 `NewTab`（`W/pages/page/attachment-row.tsx:143-152`）：名字之后一个真的空格在 `sr-only` 的 `<span>` **之外**，`{" "}<span className="sr-only">{t("page.sourceRootKind")}</span>`，后缀的文字以括号起头。为什么要这个空格、后缀以括号起头：Tailwind 的 `sr-only` 是 `position: absolute`，绝对定位的 `span` 算作 `display: block`，Playwright 算可读的名字时给每个不是 `inline` 的子元素两边各补一个空格（`playwright-core/lib/coreBundle.js` 的 `innerAccumulatedElementText`），后缀若以"，"起头、前面没有空格，Chromium 里的名字是 "raw ，来源区"，与 jsdom 里的（jsdom 不加载 Tailwind，`span` 照 `inline` 算）不同（审查 C-I2）；空格在外、文字以括号起头时两边都是同一个名字，空白合成一个。可读的名字因此是：
  - 英文：`raw (source area)`、`AGENTS (instructions for agents)`；
  - 中文：`raw （来源区）`、`AGENTS （给 agent 的约定）`（同 `asset.newTab` 的"（在新标签页打开）"，`zh-CN.ts:421`）。

  00-M10 4.16 的例子把它放在前面（"来源：…"），13.2 第 17 条要可读的名字以看得见的文字开头，所以放在后面（2.1 第 18 条）。图标照 `search-hit.tsx:89-100` 的 `aria-hidden`。今天的行没有表示种类的图标（`page-tree-item.tsx:8` 的 `ChevronRight`、`Ellipsis` 是展开与菜单的按钮的，00-M10 4.16 的"一个图标也没有"指种类的）。读链接的文字的测试（`page-tree.test.tsx:20-28` 读 `link.textContent`）只在 LLM Wiki 的种子上看到后缀（`textContent` 带空格与后缀：`raw (source area)`），普通笔记本的不变。**两种测试各看一半**（审查 C2-M1）：jsdom 的 `textContent` 看得到后缀前的空格，去掉 `{" "}` 时它是 `raw(source area)`（Chromium 的名字照样是 `raw (source area)`：`span` 的补白盖住了缺的空格），由 `page-tree.test.tsx` 抓；后缀被藏出可访问性的树时（`span` 带 `aria-hidden`，或以 Tailwind 的 `hidden` 代 `sr-only`），jsdom 不加载 Tailwind、`textContent` 不看 `aria-hidden`，vitest 照过，Chromium 的名字没有了后缀，只有 LW1 (page) 与 LW2 (page) 以 `getByRole("link", { name, exact: true })` 按上面的确切的名字找这两行时等不到（第 5 节）。空格放进 `sr-only` 里（`<span className="sr-only"> (source area)</span>`）两边的名字都不变，不是要防的写法。e2e 的 `treeRow`（`e2e/fixtures/wiki-pages.ts:29-31`）按确切的名字找链接，P1 的故事以带后缀的名字找；`treeTitles`（`:24-26`）读 `allTextContents()`，LLM Wiki 的树的这两行的标题因此带后缀（审查 C-N13）。
- 文字：

  | 键 | en | zh-CN |
  |---|---|---|
  | `page.agentsKind` | "(instructions for agents)" | "（给 agent 的约定）" |
  | `page.sourceRootKind` | "(source area)" | "（来源区）" |

- 测试（`W/pages/notebook/page-tree.test.tsx`；`pageServer` 的笔记本加一个 LLM Wiki 的，根下有 `raw`、`AGENTS`）：编辑者的 `AGENTS` 的菜单只有"新建子页"、行没有 `draggable`；编辑者的 `raw` 的菜单只有"新建子页"、行没有 `draggable`；管理员的 `raw` 有"改名、移动到…"、没有"删除"、行的 `draggable` 是 `"true"`；普通笔记本里叫 `AGENTS`、`raw` 的页照旧全有、照旧可拖；两行的链接的 `textContent` 是 `raw (source area)`、`AGENTS (instructions for agents)`；同一棵树的 `notebook` 换成 `mode: "standard"`（降级之后重读的列表）再渲染，`AGENTS` 的行有了 `draggable`；重读之前记下这一行的元素，之后断言它是同一个（`expect(after).toBe(before)`：行若重新挂上，挂上时的效果照样登记，第 64 条变异就照过，审查 C3-N1）。

**S4 留给 P2–P4 的**（00-M10 第 7 节）：来源区的行与菜单、拖进来源区之前的确认、"粘贴或上传为来源"、来源页的只读视图与新的失去编辑的原因（P2/S3）；健康度面板（P3/S2）；虚拟的两行、目录视图、历史的 ingest、虚拟页的链接（P4/S3）。`downgradeBody` 与 `ModeOptions` 的说明已写下它们的后果。

## 5. e2e

P1 的故事是 LW1、LW2 与 LW12 的 P1 的几步，在 `e2e/stories/wiki/`（新的一组）。照 10.1 与 M9 的约定：一个文件里依次是 API、MCP、页面的版本，标题 `"LW<n> (<版本>): …"`；MCP 的种子经 REST，每个 agent 一个 PAT，值来自答复。

### 5.1 夹具的改动

每处改动与它的第一个调用者在同一个提交里（knip 也看 e2e 的导出）。

- **`e2e/fixtures/notebooks.ts`**：`createNotebook(api, credential, slug, name, workspaceAccess?)`（`:9-26`）的第五个参数今天有 57 处以字面量传（224 处调用、70 个文件）。00-M10 第 9 节要一个"选项的对象"：不把第五个参数换掉（57 处的改动与这个 M 无关），在它之后加第六个 `options?: { mode?: NotebookMode; templateLocale?: "zh-CN" | "en" }`，给了才发 `mode`、`template_locale`。新的 `downgradeNotebook(api, credential, id)`，照 `updateNotebook`（`:47-53`）交回原样的答复、自己不期待什么：LW12 (API) 要它的 200、409 与 403，状态由故事核对（审查 C-N2）。
- **`e2e/fixtures/assert/notebook.ts`**：`expectNewNotebook`（`:15-41`）与 `expectNotebook`（`:44-60`）的 `SELECT` 加 `mode`、`source_root_id`，与答复的值比较（`source_root_id` 为 `null` 时比较 `IS NULL`）；它们的 8 处调用者都建普通的笔记本，照旧通过。
- **模板的断言**不另开 `assert/wiki.ts`（13.4 第 3 条，wiki 没有表）：每一页用已有的 `expectNewPage(db, created, creatorId, client, content)`（`assert/page.ts:16-60`：节点在 `created_at`、由建的人；正文 revision 1 与大小、SHA-256；以这一页的条目与版本连上的变更集，种类 `edit`、客户端），`created_at` 传**笔记本的**，正文取 `readContent` 的；变更集用 `expectChangesetOf(db, id, {client, byId, items: [raw, agents], versions: [raw, agents], message, kind: "edit"})`（`:203-253`），id 由 `listChangesets`（`history.ts:19-34`）给出。e2e 不能导入 Go 的模板，所以"是哪种语言"以正文里的一个标志核对：中文的 `AGENTS` 以"# 这本 LLM Wiki 的约定"开头，英文的以"# How this LLM Wiki is kept"开头（3.9）。
- **`e2e/fixtures/events.ts`**：`FrameData`（`:15-21`）加 `mode`（`{workspace_id, notebook_id}`），LW12 (API) 读它的帧。
- **`e2e/fixtures/transfer.ts`**：加 `runningJob(api, credential, id)`：照 `endedJob`（`:206-228`）的轮询，以同一个文件里的 `getJob`（`:201-203`）读，等到 `running`（LW12 的"另一本笔记本的导入占着唯一的工人"要知道它已经在跑）。`getJob` 不导出：别处没有人导入它，导出了 `make knip` 报未用的导出（e2e 是 knip 的一个工作区，审查 C-N3）。
- **`e2e/fixtures/notebook-pages.ts`**：`createNotebookWith(page, workspace, name, access?)`（`:37-53`）加 `mode?`，选"LLM Wiki"的单选；新的 `downgradeWith(page)`：在笔记本的设置的"常规"里按"Downgrade to a standard notebook…"、在对话框里按"Downgrade"。中文界面的那一次不用这些帮手（它们的标签是英文的），在故事里直接以中文的名字找。
- **已有的故事随之改**：S1 的 MC1（`e2e/stories/mcp/mc1-connect.spec.ts:89-99` 以 `toEqual` 比较整个 `notebooks` 的列表）的期望每项加 `mode: "standard"`；S3 的 TR1（`tr1-export.spec.ts:66-80`）、TR2（`tr2-import.spec.ts:114-129`）、TR3（`tr3-roundtrip.spec.ts:52-70`）、TR4（`tr4-malicious.spec.ts:75-93`）以 `toEqual` 比较整个报告，加 `refusal: null`（审查 B-I4 = C-I1；`e2e/` 里对报告的别的核对逐个字段，列在 3.26 第 6 条）。`open_notebook` 的 MC1（`toMatchObject`）、PG12、SE2 不受影响。

### 5.2 故事

| 版本 | Step | 文件 | 步骤与断言 |
|---|---|---|---|
| LW1 (API) | S1 | `lw1-create.spec.ts` | `newTeam`；建三本：`{mode: "llm_wiki", templateLocale: "zh-CN"}`、`{mode: "llm_wiki", templateLocale: "en"}`、`{mode: "llm_wiki"}`（得英文），与一本普通的；三本 LLM Wiki 以 `Promise.all` 一起建（oxlint 的 `no-await-in-loop` 不许循环里等；它们只共用 acme 的行的 `FOR SHARE`），422 与 `countNotebooks` 在三本都结束之后（S1 的偏离 15）。每本 LLM Wiki：`expectNewNotebook` 读到 `mode = llm_wiki`、`source_root_id` 是答复的；`listNodes` 的根下恰是 `raw`、`AGENTS`，`raw.id === source_root_id`；两页各 `expectNewPage`（笔记本的时刻、客户端 `api`）；一个变更集 `expectChangesetOf`（说明 "LLM Wiki 模板" 或 "LLM Wiki template"）；`AGENTS` 的标志是那种语言的。普通的那本：没有节点、`source_root_id` 为 `null`。`template_locale: "fr"` 答 422、字段 `template_locale` 码 `invalid_format`，`countNotebooks` 不变（`n1:43-68` 的写法） |
| LW1 (MCP) | S1 | 同上 | 经 REST 建一本英文的 LLM Wiki；`mcpClient(baseURL, pat, {notebook})` 的握手的 `instructions` 等于 `readContent(AGENTS).content.trim()`（`page/domain/instructions.go:35-36`）；账户入口的 `notebooks` 里它的 `mode` 是 `llm_wiki`；`open_notebook` 的 `instructions_from` 是 `AGENTS.md`、`mode` 是 `llm_wiki`。P5 给握手加系统的一段时这一版加一步 |
| LW2 (API) | S2 | `lw2-protected.spec.ts` | 管理员建 LLM Wiki（开放程度 `editor`），`joinAs(..., "member")` 的成员是写者。`AGENTS`：`deleteNode`、`renameNode`、`postMove` 各答 409 `page.protected`，`writeContent` 成功（`expectContentWritten`）。`raw`：`deleteNode` 409 `page.protected`；写者的 `renameNode` 403 `page.source_admin_only`；管理员的 `renameNode` 成功（`expectRenamed`）。`postPage({parent_id: null, title: "index"})` 409 `page.virtual_file`。每个拒绝之后以 `getPage`、`listNodes` 核对什么也没变（`pg2:72-73` 的写法） |
| LW2 (MCP) | S2 | 同上 | 写者的 agent：`delete AGENTS.md`、`move AGENTS.md` 的 `errorOf(...).code` 是 `page.protected`，`write index/x.md` 是 `page.virtual_file`（三个以 `Promise.all` 一起发）；`edit AGENTS.md` 成功，`expectContentWritten(db, writtenOf(...))` 与 `expectChangesetOf` 的客户端 `mcp:e2e-agent` |
| LW1 (page)（两个测试） | S4 | `lw1-create.spec.ts` | `test.describe` 里 `test.use({ locale: "zh-CN" })`（套件里第一处；只罩住这一组，英文的版本在它之外，所以是两个测试）：两种页面版本在首次导航之前仅给本工作区 notebooks 端点加 `route.continue()` 的透传路由，使请求体可观察，原请求/正文/答复不替换、精确断言不变（S4 的实际安排，3.33）。中文界面的对话框选"LLM Wiki"、建；`answerTo(...).request().postDataJSON()` 带 `mode: "llm_wiki"`、`template_locale: "zh-CN"`；到达它的主页，树里的两行以 `getByRole("link", { name, exact: true })` 按确切的可读的名字找：`raw （来源区）`、`AGENTS （给 agent 的约定）`（可读的名字在 Chromium 里的核对，4.4）；打开 `AGENTS` 读到中文的标志；历史的第一个变更集的说明是"LLM Wiki 模板"。英文界面的那一个：建的那本的两行是 `raw (source area)`、`AGENTS (instructions for agents)`，`AGENTS` 是英文的 |
| LW2 (page) | S4 | `lw2-protected.spec.ts` | 写者打开 LLM Wiki，树里的两行以确切的名字 `AGENTS (instructions for agents)`、`raw (source area)` 找（4.4）：`AGENTS` 的"Actions for AGENTS"菜单里只有"New subpage"（菜单打开时其余的项 `toHaveCount(0)`：套件里第一处核对打开的菜单里没有某项，最近的是 PG12 的 `toHaveCount(0)`）、行没有 `draggable` 属性（4.4 的拖放）；`raw` 的菜单同样只有"New subpage"、行没有 `draggable`。管理员：`raw` 的菜单有"Rename"、"Move to…"，没有"Delete"，行的 `draggable` 是 `"true"`；改名成功。把根下的另一页经"Rename"改名为 `index`：改名的对话框里出现 `page.virtual_file` 的文字（4.2），`expectConsole({errors: [failedToLoad(409)]})`，页名不变 |
| LW12 (API) | S4 | `lw12-downgrade.spec.ts` | **排着队时被拒**（00-M10 第 9 节的做法，在 worker 的服务上）：再建一本普通的笔记本 B；`holdNotebookRow(db, B.id)`（`transfer.ts:190-198`，以 `FOR SHARE` 持行）；`startImport` 进 B（非空的 `zipOf`，它的单元等在 B 的行上），`runningJob` 等它 `running`：它占着唯一的导入工人（e2e 的服务照默认 `jobs.import_workers: 1`，`server/configs/config.yaml:121`，`config.test.yaml` 不改它）；`startImport` 进 LLM Wiki A，停在 `queued`；`POST …/downgrade` 答 409 `transfer.busy`，`expectNotebook` 仍是 `llm_wiki`；`finally` 里先放开 B，再以 `Promise.allSettled` 等所有已拿到的任务 `endedJob`（部分准备失败也清理）；原错误保留，清理错误在已有错误时注记、否则在 finally 之后抛出；再降级答 200，`expectNotebook` 读到 `standard` 与 `null`。**事件**：降级之前 `settleEvents`、`openEvents(读者的凭据)`，降级之后 `expectNext("mode")` 等于 `{workspace_id, notebook_id}`。**之后**：`deleteNode` `raw` 与 `AGENTS` 各 204（`expectSubtreeDeleted`）。写者的 `POST …/downgrade` 答 403 `forbidden`。`test.slow()`（两次导入） |
| LW12 (page) | S4 | 同上 | 管理员在设置的"常规"里看到"模式：LLM Wiki"与"Downgrade to a standard notebook…"；另一个标签页（`anotherPage(joinOnboarded)`，它自己的流）开着同一本笔记本的设置页。按下、确认（对话框里没有输入名字的框）：本页的模式一节写"Standard notebook"、按钮消失、焦点在"Mode"标题上；另一个页面不重新加载就显示"Standard notebook"：`holdStream` 与 `countAnswers` 的写法（`l1-links.spec.ts:233-262`）证明它是随事件、不是随重连读到的；降级之后 `AGENTS` 的菜单有"Delete"，删除它成功。写者的设置页在降级**之前**打开（这本笔记本还是 LLM Wiki）：它写"模式：LLM Wiki"，没有降级的按钮（`toHaveCount(0)`）；放在降级之后就分不出"写者没有按钮"与"普通笔记本谁也没有按钮"（审查 C-M6）。有任务排着时的页面那一版在 API 里（同一种安排在页面上只多一个横幅，`expectConsole` 的 409；不重复） |

- **MCP 豁免**：LW12 没有 MCP 版本（00-M10 第 9 节：MCP 没有降级），它在 MCP 上看得到的效果由 `TestTheDowngradeEndsTheContract`（3.24）盖住。
- **CI 的开销**：P1 加 9 个测试（LW1 四个：API、MCP、页面的两个；LW2 三个；LW12 两个；外加 MC1、TR1–TR4 的改动），在一个不分份的 e2e 任务里（时限 20 分钟；M9 收尾的 run 的 E2E 一步 2:56，`M9-closeout-review.md:256`，Codex 评审之后的 run 的 e2e 任务 4:37，`M9-codex-review.md:290`：前者是一步、后者是整个任务，审查 C-N6）；LW12 (API) 的两次导入最重（约几秒）。00-M10 估的整个 M 约 30 个新的测试不分份（第 9 节），P1 照此。

## 6. 实施步骤

每个 Step 一个分支，从前一个 Step 合并之后的 `main` 开始；分支上的每个提交都绿（6.5 的门禁里不要 CI 的那几项在本机跑过）。提交的写法照 00 号文档第 7 节（`:538-546`）与 `docs/README.md` 的"Git 工作方式"：

- 代码 `<范围>: <说明> (M10/P1/S<m>)`，范围是改动所在的部件（`server`、`api`、`web`、`e2e`，多个用逗号分隔，`docs/README.md:100`；00-M10 的例子 `server: add the notebooks' mode (M10/P1/S1)`），下面每个提交的范围列出它碰到的每个部件（审查 C-N8；改契约的提交重新生成 `web/packages/api-client/src/schema.gen.ts`，所以带 `web`）；
- 分支上的文档 `docs: <说明> (M10/P1/S<m>)`；
- 合并之前的审查修复 `(M10/P1/S<m>)`，它们的核对 `(M10/P1/S<m> fix check <k>)`；
- 合并 `Merge branch 'm10-p1-s<m>': <说明> (M10/P1/S<m>)`；
- 合并之后 Phase 一级的修正 `(M10/P1)`，它的核对 `(M10/P1 fix check <k>)`；
- 旁支（修一个偶发的失败、调一个等待）`m10-<slug>`，提交与合并写它所属的 Step 或 Phase（审查 C-N11）。

本文被接受的那个文档提交同时照 2.1 改 00 号文档（第 8 节）。

**每个提交单独是绿的**：下面的次序逐个对着三件事排过（审查 A-M5、C-M1、C-M2）：golangci-lint 的 `unused`（`server/.golangci.yml:5`，测试文件也算）要每个不导出的函数在它落下的提交里有调用者；矩阵的 `accountNotebooksOf` 是手写的，播下 `wlab` 的提交就要改它；e2e 按确切的名字找的链接，要在名字落下之后。

### 6.1 S1（`m10-p1-s1`，从接受本文的文档提交之后的 `main`）

1. `server: the client is the credential's (M10/P1/S1)`：`shared.ClientOf` 与 `TestClientOfIsTheCredential`；page、asset、transfer 的适配器换用它、删掉各自的 `clientOf`（3.7）。适配器的测试不改（输入仍是 `Actor`）。
2. `server, api, web: a notebook's mode and source root (M10/P1/S1)`：00048、`sqlc.yaml`、`schema_test` 的名称与反例、`TestTheNotebooksModeGoesDown`；`Mode`、`Locale`、`Notebook` 的两格；读笔记本的五条查询、两处手写的转换与 `TestTheNotebooksAnswerTheirMode`、`CreateNotebook` 写两列、`notebookOf`；契约的 `Notebook` 的两个必填成员与 `NotebookMode`；根的 `State`、`StateOf`（导出的方法，没有调用者也过 `unused`）；网页的两处字面量（4.1）。这一步所有的笔记本都是 `standard`：`NotebookCreate` 还不收 `mode`。
3. `server: a template's unit in its notebook's creation (M10/P1/S1)`：`Writer.Seed` 与 `run`、`SeedWrites`、`TreeWrites().Seed`、`ParseTemplate`、`page.Facts` 的别名与 3.6 的测试。
4. `server, api, web, e2e: an LLM Wiki is created with its template (M10/P1/S1)`：播种者的扩展点（`NotebookCreation`、`Seeder`、`seed` 与它的缺陷的一支，`TestTheSeedersRunInTheirOrder`）、`CreateNotebook` 的 3.4 的次序、`NewNotebook`、`CheckDraft` 的两个新值、`notebook.New` 的 `Seeders` 的检查与 `TestTheModuleRefusesNoSeeders`、建模块的四处交 `noSeed{}`、`TestACreationPassesTheSeeders`；wiki 模块（契约表、四页、播种者、`Actions()`、`TemplatePages`）与它的测试；组合根的 `wikiSeeds`、`wikiTemplate` 与次序；`NotebookCreate` 的两个可选成员与 `createNotebook` 的说明；建的日志带 `mode`；`pageViolations` 与 3.14 的规则、`TestPageViolationsCatchEachLLMWikiGap`；3.15 的交错与 3.16 的 `TestLLMWikiCreationWritesItsTemplate`、`TestTheTemplateRollsBackWithItsNotebook`；`actions_test` 的并集；e2e 的夹具（`createNotebook` 的选项、两个断言的两列）与 LW1 (API)。模式变化的扩展点不在这一步：`publishMode` 不导出，它的调用者（降级）在第 5 步，放在这一步就过不了 `unused`（审查 A-M5）。
5. `server, api, web: downgradeNotebook (M10/P1/S1)`：模式变化的扩展点（`ModeChange`、`ModeSubscriber`、`publishMode`、`Deps.ModeSubscribers`，`TestTheModeSubscribersRunInTheirOrder`、`TestADowngradePassesTheModeSubscribers`）；`DowngradeNotebook`、`WritingJobs` 与 `transfer.NewWritingJobs`、`notebook.New` 的 `WritingJobs` 的检查与 `TestTheModuleRefusesNoWritingJobs`、建模块的四处加 `noJobs{}`；`notebook.change_mode` 与它的规则、契约的路径、适配器与它的测试、日志的测试；矩阵的按行的工作区、变体的种子与 `downgradeNotebook` 的两行（3.14），`accountNotebooksOf` 手写加上 `wlab` 的笔记本（不然 MCP 的矩阵的每个笔记本的列在这一步失败，审查 C-M1）；`notebook.not_llm_wiki` 的文字（4.1）；`TestTheDowngradeAnswersInItsOrder`、`TestImportsAndDowngradingAnLLMWiki`。组合根这一步交空的模式的订阅者（3.5：可以为空）。
6. `server, api, web: the mode event (M10/P1/S1)`：`notebookEvents.ModeChanged` 与它的登记、`events.yaml` 的 `EventMode` 与帧的一项、`TestTheModeEventReachesTheStreams`。生成的 `EventMode` 经 `export type *` 再导出，knip 看不到（4.1）。
7. `server, api, e2e: MCP answers a notebook's mode (M10/P1/S1)`：page 的 `NotebookState`、端口的 `StateOf` 与组合根的 `pageNotebooks`、`open_notebook` 与 `notebooks` 的 `mode`、`tools.json` 的两个描述与 Go 的常量、钉住答复的测试、`accountNotebook` 的 `mode`、`TestNotebooksAnswerTheirModeOverMCP`；e2e 的 MC1 与 LW1 (MCP)。
8. `docs: … (M10/P1/S1)`：第 10 节的 S1、照实改的各处，第 8 节里归 S1 的总体设计的修订与移交的进度。

### 6.2 S2（`m10-p1-s2`）

1. `server: a unit reads its notebook's state under its lock (M10/P1/S2)`：`ShareNotebook` 的两列、根的 `ShareState`、`LockState`；page 的端口的两个方法交回状态、`pageNotebooks` 转换；`Write.Notebook`、`Write.Grants` 与 `Grants`、`TestAGrantIsAskedOnceAUnit`（照实以 `node.move`、`node.rename` 作问的动作，S2 的偏离 4）；page 的假的 `Notebooks`；`TestTheGuardSeesTheCheckedChange` 的比较（`write_test.go:309-311`，不改就在这一步失败，审查 B-N7）（3.18、3.19）。
2. `server: CheckContract (M10/P1/S2)`：`page/domain` 的事实、规则、`ReservedKeys()`、三个错误、`Refused`；`TestCheckContract` 与 3.21 的表（纯函数，导出的，没有调用者也过 `unused`）。照实另有 `page.manage_source`（动作、`Actions()` 与规则的一行，从第 3 步挪来：`CheckContract` 的 `Needs` 要它，`TestTheRuleTableIsTheModulesActions` 要两边在同一个提交，S2 的偏离 4）。
3. `server, api, web, e2e: the mode guard (M10/P1/S2)`：`ModeGuard`、`NewModeGuard`、`page.Refusal`（`page.manage_source` 与它的规则照实在第 2 步）；`apply` 的标出与 `batch` 的 `failed`；`pageRegistrants(pool, authorizer)`、`pageSessionSubscribers`；六个操作的 `x-problem-codes` 与说明、page 与 asset 的适配器的测试的行；`mcpToolCodes()` 与 `wikiCodeCalls`；矩阵的变体的行；三个码的文字（4.2）；`TestTheModeGuardLetsAStandardNotebookBy`、`TestTheModeGuardAsksOnceAUnit`；3.24 的整个程序的测试（`TestTheGuardRefusesWhatThePlanLetBy` 除外）与 `TestTheProtectionAndADowngrade`；e2e 的 LW2 (API)、LW2 (MCP)。守卫、它的码与它的文字必须在同一个提交：`apitest` 两个方向都核对、`toolError` 遇到行里没有的码就失败、`problem-messages.test.ts` 要每个码有文字。
4. `server: a batch's plan judges the contract (M10/P1/S2)`：`PlanBatch` 的状态的参数、`Needs`、`Refuse`，`planned` 的 `StateOf` 与问（3.22 的"`batch` 的计划"）；`TestThePlanChecksTheContract`、`TestABatchsPlanRefusesBeforeItsLock`（没有钉住调用列表的 `batch` 测试，S2 的偏离 11）；`decidedNotebook`（S2 的偏离 8）；`TestTheGuardRefusesWhatThePlanLetBy` 与收 `wiring` 的 `acmeTeam`（`newAcmeTeamWiring`）。第 3、4 个提交之间 MCP 的 `batch` 先答 `page.locked`：只在分支上。
5. `docs: … (M10/P1/S2)`。

### 6.3 S3（`m10-p1-s3`）

1. `server, api, web: the prechecks judge the contract (M10/P1/S3)`：上传的 `Check`；`revertChecks.check` 与 `Unit.Revert` 的两遍；`startRevert` 的两个码与说明、它的适配器的测试（自己造的问题，审查 B-N4）；钉住调用列表的撤销与上传的测试加 `"StateOf"`（3.25，审查 B-N6）；3.28 的预检的模块测试；`TestPrechecksAnswerTheGuardsCodes`、`TestTheServersLimitsComeAfterThePrechecks`。
2. `server, api, web, e2e: a background job stops as refused (M10/P1/S3)`：`revertDefect` 的那一句与 `TestARevertJobsBatchPassesAGuardsRefusal`；`treeError` 的最后一段与 `TestTreeErrorKeepsTheLockFirst`；transfer 的 `Refused`、`FailureRefused`、`Report.Refusal`、存储与 HTTP、契约（`code` 是自由的字符串）；网页的 `transfer-report.tsx` 与它的文字、`reportOf` 的 `refusal: null`（4.3）；e2e 的 TR1、TR2、TR3、TR4；transfer 的测试的新行；`TestTheSourceRootsPutBackIsRefusedInALaterBatch`。去处（`revertDefect`、`treeError`、`refused`）在一个提交里：分开时中间的提交把被拒的一批报成 `forbidden`（`unitFailure` 认 `KindForbidden`）或 `internal`。任务的计划的预检在第 1 个提交，所以第 1、2 个提交之间它拒绝的任务同样报成 `forbidden` 或 `internal`（审查 B-N10）：只在分支上，没有测试走到它（`startRevert` 的预检在请求里先拒）。
3. `server: reserved titles are numbered at an LLM Wiki's root (M10/P1/S3)`：导入、同步与后台的恢复的加序号；`checkPages` 的"根下没有保留的键"与它的反例；3.27 的模块测试与 `TestTheReservedTitlesAreNumberedAtAnLLMWikisRoot`。
4. `docs: … (M10/P1/S3)`。

### 6.4 S4（`m10-p1-s4`）

树的一步排在最前（审查 C-M2）：LW1 (page) 按带后缀的确切的名字找树的两行，后缀要先落下。

1. `web, e2e: AGENTS and the source root in the page tree (M10/P1/S4)`：行的种类、菜单、不能拖的行不登记 `draggable()`、图标与可读的名字的后缀、文字、`page-tree.test.tsx`；e2e 的 LW2 (page)（LLM Wiki 经 S1 的夹具以 API 建）。
2. `web, e2e: the create dialog chooses the mode (M10/P1/S4)`：`ModeOptions`、请求体、文字、`notebook-nav.test.tsx` 的改动与新测试；e2e 的 `createNotebookWith` 的 `mode` 与 LW1 (page) 的两个测试。
3. `web: the settings' mode and its downgrade (M10/P1/S4)`：`NotebookService.downgrade`、`NotebookStore.downgrade` 与它的测试、`notebookServer` 的路由与 `mode` 的选项、"模式"一节、持有的对话框、文字、`general-page.test.tsx` 的新测试（4.4）。
4. `web: the mode event reads the notebooks again (M10/P1/S4)`：`modeChanged`、表与注释、`EventMode` 的导出、`event-stream.test.tsx` 的替身改成 `later` 与新的测试。
5. `e2e: LW12, a downgrade (M10/P1/S4)`：夹具的 `downgradeNotebook`、`downgradeWith`、`runningJob`、`FrameData` 的 `mode`；LW12 (API)、LW12 (page)。
6. `docs: … (M10/P1/S4)`；合并之后 `docs: M10/P1 done (M10/P1)`：00 号文档的头、第 12 节的 P1 一行与第 13 节的一行、第 10 节的总结、移交的进度与 `M5-events.md` 的 `status: done`（第 8 节）。

### 6.5 合并的门禁

每个 Step 合并之前：

- 本机：`make lint`（含 `lint-web` 的 tsc）、`make gen-check`、`make knip`、`make test`（`test-go` 的竞态一份与组合根三份、成本与守卫的两行、`test-web`）、`make build`、`make e2e`，全绿；
- 分支的 CI 全绿（服务端七份、网页、e2e、镜像），第 10 节记下组合根三份与 e2e 的时间（7.5）；
- 一次审查（三位审查者照 M9 的写法，记录在 `reviews/P1-S<m>-…-review.md`）与它的修复、修复的核对，未结的在合并之前处理完；
- 第 10 节写下这个 Step 照实与设计不同之处（同 M9/P3 的 3.28–3.30），在分支上的文档提交里；
- 合并之后自己推 `main`（负责人 2026-10-14 的约定）；
- 四个 Step 一个接一个合并，中间不合并别的分支、不发布（第 0 节的窗口）。

## 7. 测试与验证

### 7.1 各项与它们的测试

| 项 | 测试（第 3–5 节） | Step |
|---|---|---|
| 迁移 00048 | `TestConstraintAndIndexNames`、`TestChecksRejectCounterexamples` 的新行，`TestTheNotebooksModeGoesDown` | S1 |
| 读笔记本的查询与转换 | `TestTheNotebooksAnswerTheirMode`（五条查询，含两处手写的转换） | S1 |
| `CheckDraft`、码的次序 | `notebook/domain` 的表格；`TestCreateNotebookRefuses` 加 `mode`、`template_locale` 的 422 在 404、403 之后 | S1 |
| 建的事务、播种者、一个时刻 | `notebook/app/write_test.go` 的调用次序（假的播种者与假的可见性的订阅者记进同一个列表：`"Seed … in tx"` 在 `"AddMember …"` 之后、`"VisibilityChanged … in tx"` 之前）、`TestACreationPassesTheSeeders`（含播种者的 `page.*` 错误答 500）、`TestTheSeedersRunInTheirOrder`、`TestTheModuleRefusesNoSeeders`；`TestLLMWikiCreationWritesItsTemplate`、`TestTheTemplateRollsBackWithItsNotebook`；假的 `CreateNotebook` 照表的两个检查拒绝（第 74 条变异） | S1 |
| `Seed` | 3.6 的六个（`TestASeedPassesTheFactsItIsGiven` 是 S1 的修复一轮加的；`TestATemplatesParseIsItsContents` 带 `pageMark()`） | S1 |
| `ClientOf` | `TestClientOfIsTheCredential`；三个适配器的客户端的行；notebook 的 `createNotebook` 交 `Client` | S1 |
| 模板 | 3.9 的三个；`wiki/app` 的 `TestTheTemplateSeedsAnLLMWiki`（创建之后仍是四次解析）、`TestTheTemplateAnswersTheSourceRootWritten`、`TestTheTemplateFails`，模块根的替身；`contract_test` | S1 |
| 降级 | `mode_test.go` 的三个（含"找到时 `llm_wiki`、锁下 `standard`"的一行）、日志的测试、适配器的四行；`TestTheDowngradeAnswersInItsOrder`；矩阵的两行（变体的一行带 `after` 的 `GET`） | S1 |
| 模式的事件 | `TestADowngradePassesTheModeSubscribers`（另读 `updated_by_id`）、`TestTheModeSubscribersRunInTheirOrder`；`TestTheModeEventReachesTheStreams` | S1 |
| MCP 的 `mode` | `open_notebook` 的次序与答复、`notebooks_test.go`、`TestOpenNotebookOfAnEmptyNotebook`、`TestNotebooksListsTheAccountsWorkspaces`、`TestNotebooksAnswerTheirModeOverMCP`（降级之后再读 `notebooks`）、`TestPageNotebooksConvertTheState`、`TestTheToolListsAreTheContract` | S1 |
| `checkPages`、矩阵的变体 | `TestPageViolationsCatchEachLLMWikiGap`、`TestMatrixViolationsCatchesEachGap` 的新行、`TestPermissionMatrix` 的变体的行 | S1、S2、S3 |
| `Write.Notebook`、`Grants` | `notebook/notebooks_test.go` 的新方法；`TestAGrantIsAskedOnceAUnit`、`TestTheModeGuardAsksOnceAUnit` | S2 |
| `CheckContract` | `TestCheckContract`（含 B-Q1 的几行）、`TestTheReservedKeysAreTitleKeys`、`TestFactsOfAStep` | S2 |
| 守卫与它的登记 | `TestTheModeGuardLetsAStandardNotebookBy`、`TestTheModeGuardMarksTheNodeItRefuses`（S2 的修复一轮加）、`TestTheModeGuardReachesEveryPath`、`TestTheModeGuardReachesEveryMCPWrite`、`TestModeComesBeforeTheLock`、`TestTheDowngradeEndsTheContract` | S2 |
| 标出 | `TestAUnitMarksItsGuardsRefusals`（`page/app/unit_test.go`：一个替身守卫的 `*shared.Error` 出来时是以这一步的第一个节点标出的、非问题的错误原样）、`page/refusal_test.go`、`domain/refused_test.go` 的 `TestARefusalIsMarkedOnce`、`TestABatchAnswersARefusalAtItsOperation`、适配器的表的标出的行 | S2 |
| `batch` 的计划 | `TestThePlanChecksTheContract`（`Refuse` 的两种行为的三行是 S2 的修复一轮加的）、`TestABatchsPlanRefusesBeforeItsLock`；`TestTheGuardRefusesWhatThePlanLetBy`（只有守卫拒绝的情形与 `guards: []` 的反向对照） | S2 |
| 码 | `apitest.Main`（page、asset、notebook、transfer 的适配器）、`TestTheMCPToolsErrorsCarryTheirCodes`、`TestEveryMCPToolHasItsCodes`、`problem-messages.test.ts` | S1–S3 |
| 预检 | 3.28 的模块测试；`TestPrechecksAnswerTheGuardsCodes`、`TestTheServersLimitsComeAfterThePrechecks`（解析的预算）；`asset/app/upload_test.go` 的 `TestCheckDecidesTheNodeThenTheFreeSpace` 的一种（`storage_full` 的次序） | S3 |
| 后台的去处 | `TestARevertJobsBatchPassesAGuardsRefusal`、`TestTreeErrorKeepsTheLockFirst`、transfer 的 `TestABackgroundJobStopsWithItsReason`、`TestAnImportsUnitFails`、存储与 HTTP 的往返（含一个自由的码）；`TestTheSourceRootsPutBackIsRefusedInALaterBatch`；`TestABackgroundRevertAndAnEditSession`（已有，钉住 `page_locked` 照旧） | S3 |
| 加序号 | 3.27 的两个模块测试、`TestTheReservedTitlesAreNumberedAtAnLLMWikisRoot` | S3 |
| 网页 | 4.1–4.4 的各个 vitest | S1–S4 |
| e2e | 第 5 节的九个与 MC1、TR1–TR4 的改动；树的两行的可读的名字在 Chromium 里由 LW1 (page)、LW2 (page) 核对，后缀前的空格由 `page-tree.test.tsx` 的 `textContent` 核对（4.4） | S1–S4 |

### 7.2 交错

每一条写明怎样逼出两种次序（13.4 第 4 条）；都以 `checkPages`、`checkNotebooks` 结束。

| # | 交错 | 测试 | 怎样逼出 |
|---|---|---|---|
| 1 | 建 LLM Wiki 与工作区的删除 | `TestWorkspaceChangesAndCreatingAnLLMWiki` 的第 1 个子测试 | `tm.interleave`：测试的事务持着 acme 的行 `FOR NO KEY UPDATE`，先发的那一步先在行上等着（`pgtest.WaitForLockWaitsOn`）再发第二步，放开之后按到达的次序拿到锁（`interleavings_workspace_test.go:158-200`）。建的事务的第一把锁是工作区行 `FOR SHARE`（3.4 第 2 步），删除的是 `FOR NO KEY UPDATE` |
| 2 | 建与建的人的成员身份结束 | 同上，第 2 个 | 同上：移出持工作区行 |
| 3 | 建与建的人的角色改变 | 同上，第 3 个 | 同上：改角色持工作区行 |
| 4 | 降级与开始导入 | `TestImportsAndDowngradingAnLLMWiki` | `tm.interleaveOn(t, notebookRow(nb), …)`：测试持着笔记本行 `FOR NO KEY UPDATE`；开始导入的事务以 `FOR SHARE` 等它（`transfer/app/start.go:103-113`），降级以 `FOR NO KEY UPDATE` 等它 |
| 5 | 降级与删除 `AGENTS` | `TestTheProtectionAndADowngrade` | 同 4：删除的单元以 `FOR NO KEY UPDATE` 锁笔记本行（`unit.go:217-223`） |
| 6 | `batch` 的计划之后、单元之前执行者被降为写者 | `TestTheGuardRefusesWhatThePlanLetBy`（S2） | 同 4：降为写者的请求先到、在行上等；bob 的 MCP 的 `batch [move raw.md → Notes/raw.md]` 的计划在锁外读到管理员、放过，然后单元在行上等；放开之后降级先提交，单元在锁下再问；`guards: []` 的程序里同样的次序让移动成功（反向对照） |
| 7 | 后台撤销的两批之间执行者被降为写者 | `TestTheSourceRootsPutBackIsRefusedInALaterBatch` | `tm.behindFirstBatch`（`background_test.go:196-210`）：测试以 `FOR SHARE` 持着笔记本行，任务的第一批等它；改角色的请求在它之后排队；放开之后第一批先、改角色后；改角色在第二批之前还是之后是竞争，照 `TestARevertJobAndAPageCreatedUnderATopItDeletes`（`interleavings_background_test.go:351-378`）的循环重试 `betweenUnits` 次，每次一本新的笔记本、新的 C 与任务，输的一支核对任务整个成功（3.28）；`transfer.Batch{Nodes: 1}` 让两步分在两批 |
| 8 | 模板的单元在排着的工作区删除之后重入工作区行 | `TestWorkspaceChangesAndCreatingAnLLMWiki` 的第 4 个子测试（S1 的修复一轮加） | 测试库里 `notebook_members` 上的触发器等测试持着的 advisory lock（`pgtest.WaitForAdvisoryLockWaits`）：建停在管理员的插入之后、持着 acme 的行 `FOR SHARE`；删除发出、在 `workspaces` 上等（`WaitForLockWaitsOn`），然后放开。钉住 PostgreSQL 把已持的锁交给唯一的持有者（3.15；主持者的决定 24：我们的代码没有一处改动让它失败） |

两个管理员同时降级不另写交错（3.16：锁下读模式由模块一级的一行核对，一个 `mode` 由 `TestTheModeEventReachesTheStreams` 核对）。00-M10 4.17 列的别的交错（去重、lint 的快照、虚拟文件）是 P2–P4 的。

### 7.3 变异

每个变异是实施时在本机对代码做的一处改动，跑它旁边写的测试，要它失败（不提交）；第 10 节记下每一个的结果。修订把草稿的 47 条重排为 61 条，第一轮复核之后是下面的 64 条：S1 第 1–22 条，S2 第 23–41 条，S3 第 42–53 条，S4 第 54–64 条（复核加了 S3 的第 53 条，S4 的旧的第 53–60 条成为第 54–61 条，旧的第 61 条换成第 62、63 条，另加第 64 条）。S1 的审查与修复核对的变异是第 65–80 条（S2–S4 照旧是第 23–64 条）；第 12 条照建成时分成 a、b 两种。S2 的审查的变异是第 81–84 条；第 23、29、32、40 条写明照实的写法（S2 的偏离 15），第 41 条照建成时分成 a、b 跑。

| # | 变异 | 让它失败的测试 |
|---|---|---|
| 1 | 建的事务先发可见性、后调播种者 | `notebook/app/write_test.go` 的调用次序 |
| 2 | 播种者拿到第二次读的时钟 | `TestACreationPassesTheSeeders`（`At` 等于 `tickingClock` 的第一次读） |
| 3 | 不核对播种者交回的来源根 | `TestACreationPassesTheSeeders` 的 500 的几行 |
| 4 | 核对写成"至少一个" | 同上，两个都交回的一行 |
| 5 | 来源根的 id 在 `Seed` 里另取一个 V7 | `wiki/app/template_test.go`（`raw` 的 id 是值里的）；`TestLLMWikiCreationWritesItsTemplate`（`raw.id == source_root_id`）。模块根的 `TestACreationPassesTheSeeders` 的播种者是替身，看不到它（审查 C-N7） |
| 6 | `CheckDraft` 只在 `llm_wiki` 时核对语言 | `notebook/domain` 的 `standard` 带 `"fr"` 的一行 |
| 7 | `seed` 把播种者的问题照原样交回 | `TestTheSeedersRunInTheirOrder` 的 `*shared.Error` 的一行；`TestACreationPassesTheSeeders`（答 409、不是 500） |
| 8 | `Seed` 照 `Run` 开自己的事务 | `TestASeedJoinsItsCallersTransaction`（外层回滚之后页还在） |
| 9 | `run` 照旧读时钟 | `TestASeedReadsNoClock` |
| 10 | `ClientOf` 对没有 `Actor` 的 ctx 答 `web` | `TestClientOfIsTheCredential` |
| 11 | 模板先写 `AGENTS`、后写 `raw` | `wiki/app/template_test.go`（页的次序）；`TestLLMWikiCreationWritesItsTemplate`（根下的次序） |
| 12 | 模板的变更集不带说明：a，去掉 `Template.NotebookCreated` 的 `Message:`（组合根于是交 `&""`）；b，`wikiSeeds.Seed` 交 `Message: nil`（库照收的一行） | a：`TestLLMWikiCreationWritesItsTemplate`、e2e 的 LW1 (API)，都因建的 500（`changesets_message_check` 拒绝空的说明），所以 a 单独看不出说明被核对；b：两者都在比较说明时失败（Go 的测试经 `coalesce` 读到 `message ""`，主持者的决定 25；LW1 (API) 读到 `"message": null`） |
| 13 | 段落对应的比较只比块数 | `TestTheTemplatesCorrespond` 的"一处行内代码不同"的反例 |
| 14 | `ListOwnerless` 的手写的转换漏了 `Mode` | `TestTheNotebooksAnswerTheirMode` |
| 15 | 降级先看任务、后看模式 | `TestDowngradeNotebookRefuses` 的次序的行 |
| 16 | `SetNotebookMode` 不把 `source_root_id` 写成 `NULL`（检查照收：普通的笔记本可以有它） | `TestTheDowngradeAnswersInItsOrder`（200 的答复与 `getNotebook` 相同）；矩阵的变体的 `after`（审查 A-N9） |
| 17 | 降级在锁之前判定 | `TestDowngradeNotebookMakesItStandard` 的调用次序 |
| 18 | 降级看锁之前（`FindNotebook`）读的模式 | `TestDowngradeNotebookRefuses` 的"找到时 `llm_wiki`、锁下 `standard`"的一行（审查 A-N10） |
| 19 | 不登记模式的订阅者 | `TestTheModeEventReachesTheStreams` |
| 20 | `checkPages` 的新规则不看 `r.deleted_at`（节点的） | `TestPageViolationsCatchEachLLMWikiGap`（`raw` 的 `deleted_at` 设上的一行） |
| 21 | `checkPages` 的新规则不看 `n.deleted_at`（笔记本的） | `TestPageViolationsCatchEachLLMWikiGap` 的删掉的 LLM Wiki 的一行（审查 A-M2） |
| 22 | `targetViolation` 照旧只认 `lab` | `TestThePermissionMatrixCoversEveryOperation`（变体的行被报出） |
| 23 | `ShareNotebook` 不选两列（`ShareState` 总答 `standard`；照实：查询照选两列、存储不读，答 `standard` 与零的来源根，变异之下不重新生成 sqlc） | `notebook/notebooks_test.go` 的 `ShareState` 的一行 |
| 24 | 守卫登记在锁之后 | `TestModeComesBeforeTheLock` |
| 25 | R1 漏了 `OpMove` | `TestCheckContract`；`TestTheModeGuardReachesEveryPath`（排序 `AGENTS`） |
| 26 | R2 只看 `Nodes[0]` | `TestCheckContract`；`TestTheModeGuardReachesEveryPath`（删除 `Notes`） |
| 27 | R3 不只看根下 | `TestCheckContract`；`TestTheModeGuardReachesEveryPath`（子页下的 `index` 201） |
| 28 | R3 也拒导入的单元 | `TestCheckContract` 的导入的一行（S3 起导入在单元写之前就加序号，整个程序的测试抓不到它，3.21，审查 B2-N3） |
| 29 | R4 把祖先的改名也算进来（照实写成"每个改名都要 `page.manage_source`"：改名的一步只有那个节点，P1 不读祖先，契约认不出祖先的改名） | `TestTheModeGuardReachesEveryPath`（bob 改名 `Notes` 200） |
| 30 | R4 对 `OpMove` 只看第一个节点 | `TestCheckContract`；`TestTheModeGuardReachesEveryPath`（bob 把 `Notes` 移到另一页之下 403） |
| 31 | R4 不看第一个节点换没换父节点（同一父节点下排序祖先也要它） | `TestCheckContract` 的排序 `Notes` 的一行；`TestTheModeGuardReachesEveryPath`（bob 排序 `Notes` 200）（主持者对 B-Q1 的决定） |
| 32 | `Write.Grants` 是一个值（照实写它的效果：每一步一份单元的 `Grants` 的副本，不改类型：改类型要改几个测试文件，编译不过不算抓到） | `TestTheModeGuardAsksOnceAUnit`（`page/app`，假的授权者放行 `page.manage_source`、两次移动都写下时记下两次 `"Authorize page.manage_source in tx"`，审查 B3-N1） |
| 33 | `Grants` 记下数据库的错误 | `TestAGrantIsAskedOnceAUnit` 的数据库的错误的一行（下一次再问） |
| 34 | 守卫与 `CheckContract` 都不看模式（把普通的笔记本当 LLM Wiki 判） | `TestTheModeGuardLetsAStandardNotebookBy`；`TestCheckContract` 的普通的笔记本的几行；`TestTheModeGuardReachesEveryPath` 的普通的那一本（审查 B-M6；单去掉守卫的第一行是等价的，不列） |
| 35 | `apply` 不标出别的守卫的错误 | `TestAUnitMarksItsGuardsRefusals` |
| 36 | `failed` 先看 `replans` | `TestABatchAnswersARefusalAtItsOperation` |
| 37 | 计划不问 `Needs`（只靠守卫） | `TestABatchsPlanRefusesBeforeItsLock`（写者的 `[move raw, delete AGENTS]` 答成第 1 项的 409） |
| 38 | 计划的预检排在编辑锁的预检之后 | 同上（`AGENTS` 被会话持着时） |
| 39 | 计划的问在 `a.read` 之后（审查 B-M3） | `TestABatchsPlanRefusesBeforeItsLock` 的 `Big` 过了读的上限的一行 |
| 40 | 计划把它的答案交给单元的 `Grants`（照实：经 ctx 的一个值，单元的 `Grants` 从它开始） | `TestTheGuardRefusesWhatThePlanLetBy`（bob 的移动成功） |
| 41 | 计划把授权者的 `ErrNotVisible` 当拒绝、把数据库的错误标成拒绝 | `TestABatchsPlanRefusesBeforeItsLock` 的 404 与数据库的错误的两行（审查 B-N5）；两处各做一次 |
| 42 | 上传的 `Check` 不判契约 | `TestAnUploadsCheckRefusesAReservedTitle`（整个程序里的磁盘的一半第一轮复核去掉了，3.25） |
| 43 | 撤销的预检排在 `Locks` 之后 | `TestARevertsChecksRefuseTheContract` |
| 44 | 撤销的契约的检查挪到解析之后（`Execute` 的 `DecidedContents` 之后） | `TestTheServersLimitsComeAfterThePrechecks`（bob 的撤销答 503；审查 B-I3） |
| 45 | `revertDefect` 不认标出的拒绝 | `TestARevertJobsBatchPassesAGuardsRefusal`；`TestTheSourceRootsPutBackIsRefusedInALaterBatch`（赢的一支失败成 `internal`） |
| 46 | `treeError` 先认拒绝、后认哨兵 | `TestTreeErrorKeepsTheLockFirst`；`TestABackgroundRevertAndAnEditSession` |
| 47 | `treeError` 不是幂等的 | `TestTreeErrorKeepsTheLockFirst`（再经一次交回同一个指针，审查 C2-N5） |
| 48 | `unitFailure` 先认 `forbidden` | `TestABackgroundJobStopsWithItsReason` 的 `page.source_admin_only` 的一行 |
| 49 | 导入的失败不写 `refusal` | `TestAnImportsUnitFails` 的 "refused" |
| 50 | 加序号写在 `under` 里（每个父节点） | `TestARestoreNumbersTheReservedTitlesAtAnLLMWikisRoot` 的子页下的两行（成员随父节点、根回到活着的子页之下，审查 B2-N2） |
| 51 | 普通的笔记本也加序号 | 3.27 的两个模块测试的普通的一行 |
| 52 | 报告直接以 `problemMessages[code]` 取文字（不认得的码） | `transfer-background.test.tsx` 的 `page.some_later_code` 的一条（审查 B-I2 = C-I3） |
| 53 | `Unit.Revert` 的锁下的一遍不判契约 | `TestARevertsChecksRefuseTheContract` 的锁下的一行（替身守卫什么也不拒：写者的撤销照写下去，审查 B2-N4） |
| 54 | 网页的表里没有 `mode` | `event-stream.test.tsx` 的新测试 |
| 55 | `NotebookStore.downgrade` 不经 `inTurn` | `notebook.store.test.ts` 的排队的一条 |
| 56 | 409 `notebook.not_llm_wiki` 照抛出 | 同上的一条 |
| 57 | a：`downgrade` 不 `put` 答复；b：`put` 而不计 `changed()` | `notebook.store.test.ts` 的 "a read answered after a downgrade keeps it"（审查 C-M3） |
| 58 | 降级的对话框带触发按钮（随 403 卸下） | `general-page.test.tsx` 的 403 的一条（答 403 之前 Ada 已改成编辑者；审查 C-M4） |
| 59 | `template_locale` 取浏览器的语言 | `notebook-nav.test.tsx` 的中文偏好的一条（jsdom 的 `navigator.languages` 是英文的） |
| 60 | `AGENTS` 的菜单留着"删除" | `page-tree.test.tsx` |
| 61 | 不能拖的行照样登记 `draggable()` | `page-tree.test.tsx` 的 `draggable` 属性的几行（审查 C-M5） |
| 62 | 去掉后缀前的 `{" "}` | `page-tree.test.tsx` 的 `textContent` 的两行（`raw(source area)`；Chromium 的名字照样有空格，e2e 抓不到，4.4，审查 C2-M1） |
| 63 | a：后缀的 `span` 带 `aria-hidden`；b：以 `hidden` 代 `sr-only` | e2e 的 LW1 (page) 英文、中文两条与 LW2 (page) 一条，a、b 各核对三条（按确切的名字等不到；jsdom 的 `textContent` 照旧，4.4，审查 C2-M1） |
| 64 | `useDrag` 的依赖不含算出的能不能拖 | `page-tree.test.tsx` 的换成普通的笔记本之后的一条（同一个行元素没有 `draggable`，4.4，审查 C2-N4、C3-N1） |
| 65 | `checkPages` 的新规则不看 `a.parent_id`（`AGENTS` 在哪里都算） | `TestPageViolationsCatchEachLLMWikiGap` 的"AGENTS under raw"一行（S1 审查 B-M1 = C-M1：b1、C1） |
| 66 | `SetNotebookMode` 不写执行者（生成的 SQL 写 `updated_by_id = coalesce(updated_by_id, $3)`） | `TestADowngradePassesTheModeSubscribers` 的成功的一行（行先由 bob 改过；S1 审查 B-M2：b2） |
| 67 | 段落对应不比列表的项数 | `TestTheTemplatesCorrespond` 的"two items on one line"（S1 审查 A-M1：a3） |
| 68 | 段落对应不比标题的级 | 同上的"a heading a level down"（a10） |
| 69 | 段落对应不比围栏的信息串与行数 | 同上的"a fence a line short"（a11） |
| 70 | 段落对应不比段落的行数 | 同上的"a paragraph in two lines"（a12） |
| 71 | `Template.NotebookCreated` 每次创建再解析模板 | `TestTheTemplateSeedsAnLLMWiki`（创建之后仍是四次解析；S1 审查 A-M2：a1） |
| 72 | `SeedWrites.Seed` 在 `CreatePage` 之前再解析正文 | `TestASeedPassesTheFactsItIsGiven`（a13） |
| 73 | `ParseTemplate` 不看正文（答 `Facts("")`） | `TestATemplatesParseIsItsContents`（`pageMark()`；S1 审查 A-M3：a19） |
| 74 | 建 LLM Wiki 不取来源根的 id | `TestCreateNotebookMakesTheCallerItsAdmin`（假的仓库照表的检查答 `notebooks_source_root_check`；S1 审查 A-N2：x7） |
| 75 | 模板答创建的 id，不答写下的 | `TestTheTemplateAnswersTheSourceRootWritten`（S1 审查 A-N3：x8） |
| 76 | `SeedWrites.Seed` 答草稿的 id，不答写下的 | `TestATemplatesParseIsItsContents` 的没有 id 的一页（x9） |
| 77 | 模板不核对 `Seed` 交回的 id 的个数 | `TestTheTemplateAnswersTheSourceRootWritten` 的"one id for two pages"（修复核对 A 的 f2） |
| 78 | `MaxTemplateBytes` 提到 64 KiB | `TestTheTemplatesAreSmall`（写出的 8 KiB；S1 审查 C-N3：C35） |
| 79 | `MaxTemplateBytes` 降到 4 KiB | 同上（恰 8 KiB 的一页照收；x10） |
| 80 | `pageNotebooks.StateOf` 对找不到的笔记本答 `ok` | `TestPageNotebooksConvertTheState`（S1 审查 C-N4：C40） |
| 81 | `Refuse` 保留同一项上的失败（`p.Failed.Op <= at`） | `TestThePlanChecksTheContract` 的"the items' bound"与"the moves' carry"（答成第 1 项的 `page.batch_too_large`；S2 审查 A-M1 = C-M2：RA1 = c06） |
| 82 | `Refuse` 不清 `final` | `TestThePlanChecksTheContract` 的清掉 `final` 的一行（计划什么也不读，答第 2 项的 403、不答第 0 项的正文的失败；RA2 = c05） |
| 83 | 守卫原样交回它的规则的拒绝（`return v.Err`，`Unit.apply` 于是以步的第一个节点标出） | `TestTheModeGuardMarksTheNodeItRefuses` 的删除 `Notes` 的一行（S2 审查 C-M1：c01） |
| 84 | 守卫以步的第一个节点标出 `page.source_admin_only` | 同上，把 `Notes` 移到 `Other` 之下的一行（c02） |
| 85 | `RevertFacts` 去掉 `sim.place(op.Kind, op.NodeID, op.To)`，后面的事实仍看移动/临时改名之前的树 | `TestARevertsFactsFollowItsSteps` 的删除、后续移动与后续删除三处；`TestAContentOnlyRevertsFactsNeedNoTree` 照过（S3 的偏离 2） |
| 86 | `jobRun.finish` 不清掉最终 failure 不是 `FailureRefused` 的 `report.Refusal` | `TestAJobsRefusalBelongsToItsFinalFailure` 的 timeout、interrupted、cancelled 三行，普通拒绝照过、先前的进度仍保留（S3 审查 B-M1） |
| 87 | ListJobs 的报告投影额外去掉 `refusal`（改生成的 `jobs.sql.go`，变异上不重新生成） | `TestARefusedJobsReportReadsBackAsWritten` 的已知码与未来码的 ListJobs 断言，详情仍正确（S3 完整 Step 审查 B） |
| 88 | 组合根的 `pageRegistrants` 不登记模式的守卫，只留编辑锁的守卫 | `TestTheSourceRootsPutBackIsRefusedInALaterBatch`：没有真正的模式守卫时每次都完成第二批，碰不到要求的 refused 而失败（S3 完整 Step 审查 A/C，同一个逻辑变体；正式证明用 C 的实际文件备份与恢复） |
| 89 | AGENTS 分类去掉 `node.parent_id === null`，嵌套同名页也被保护 | `page-tree.test.tsx` 已有的 "only the root AGENTS is protected, by its normalized title"；嵌套页的可读名字被错误加后缀而失败（S4审查A-X1） |
| 90 | 只在拒绝后的 reconciliation 绕过 `put/changed`、直接更新列表，不计 changesAnswered | `notebook.store.test.ts` 的 "a read answered after downgrade reconciliation" 六个参数化测试实例：三个拒绝码 × present/absent，旧读不能盖掉刚重读的结果；六个参数化测试实例都须运行断言失败（S4审查B-X3，B-I1补足） |

### 7.4 门禁

6.5。另：`TestTheBuiltInInstructionsAreShort` 照旧（P1 不改内置的说明）；`TestTheToolListsAreTheContract` 在 S1 的第 7 步改期望之后照过；`make gen-check` 在每个改契约的提交上。

### 7.5 CI 的余量

新的整个程序的测试照 `BOOTSTRAP_FIRST = ^Test([A-D]|E([a-u]|very[A-R]))`、`BOOTSTRAP_THIRD = ^Test(The[F-Z]|T[i-z]|[U-Z])`（`Makefile:217-219`）取名，一个也不落进第一份（它离 7:30 的线最近，`Makefile:204-205`、M9 总设计第 110 条）；第三份的余量比第二份多（下面的量），两个 S3 的测试因此以 `TheS…`、`TheR…` 取名落进第三份：

| 测试 | 份 | Step |
|---|---|---|
| `TestLLMWikiCreationWritesItsTemplate` | 二 | S1 |
| `TestTheTemplateRollsBackWithItsNotebook` | 三 | S1 |
| `TestWorkspaceChangesAndCreatingAnLLMWiki` | 三 | S1 |
| `TestTheDowngradeAnswersInItsOrder` | 二（`TheD` 不配 `The[F-Z]`） | S1 |
| `TestImportsAndDowngradingAnLLMWiki` | 二 | S1 |
| `TestTheModeEventReachesTheStreams` | 三 | S1 |
| `TestNotebooksAnswerTheirModeOverMCP` | 二 | S1 |
| `TestPageViolationsCatchEachLLMWikiGap` | 二 | S1 |
| `TestPageNotebooksConvertTheState`（不要库，S1 的修复一轮加） | 二 | S1 |
| `TestTheModeGuardReachesEveryPath` | 三 | S2 |
| `TestTheModeGuardReachesEveryMCPWrite` | 三 | S2 |
| `TestModeComesBeforeTheLock` | 二 | S2 |
| `TestTheProtectionAndADowngrade` | 三 | S2 |
| `TestTheGuardRefusesWhatThePlanLetBy` | 三 | S2 |
| `TestTheDowngradeEndsTheContract` | 二 | S2 |
| `TestPrechecksAnswerTheGuardsCodes` | 二 | S3 |
| `TestTheServersLimitsComeAfterThePrechecks` | 三 | S3 |
| `TestTheSourceRootsPutBackIsRefusedInALaterBatch` | 三 | S3 |
| `TestTheReservedTitlesAreNumberedAtAnLLMWikisRoot` | 三 | S3 |
| `TestTreeErrorKeepsTheLockFirst`（纯函数） | 三（`Tr` 配 `T[i-z]`） | S3 |

九个在第二份（一个不要库，S1 的修复一轮加）、十一个在第三份。已有的测试加的行：`TestPermissionMatrix`（第二份）的变体的几行（每个写的格子一份库的副本，约 30 格），`TestTheMCPToolsErrorsCarryTheirCodes`（第三份）多建一本 LLM Wiki 与约十次调用，`TestEveryMCPToolHasItsCodes`（第一份）只多几个码、不多请求。

**量**（审查 C-M7：照 CI 的每个测试的平均与竞态检测估，不照本机不带竞态的时间）：组合根的三份在 CI 上跑 `go test -race -count=1 -timeout 15m`（`Makefile:179`、`:207-218`），CI 与本机之比约 1.73（`Makefile:207` 的记录）。M9 收尾的 run 的测试一步：第一份 7:07、第二份 6:12、第三份 5:25（1 节）。Codex 评审之后的 run（P1 开始时的代码）只有作业的时间 5:28、6:33、6:21，不与 7:30 的线比（审查 C-N6）；减去收尾的 run 里作业与测试一步之差（第二份 20 秒、第三份 24 秒），测试一步约是第二份 6:13、第三份 5:57。照 `Makefile:200-203` 的规则取两次的平均作底：第二份约 6:12，第三份约 5:41（审查 C2-N1）。第二份约一百个测试，每个约 3–3.5 秒。所以：

- 第二份：八个新的约 25–30 秒，较重的（`TestImportsAndDowngradingAnLLMWiki` 的一次导入、`TestLLMWikiCreationWritesItsTemplate` 的四本）按 5 秒约多 5 秒，`TestPermissionMatrix` 的约 30 个变体的格子按矩阵今天的每格（一份库的副本）约 15–25 秒：共约 45–60 秒，6:12 之后约 7:00–7:15，较慢的一次可能到线上；
- 第三份：十一个新的约 35–40 秒，后台的两批与三个交错较重，约 45–55 秒；`TestTheServersLimitsComeAfterThePrechecks` 只剩解析的预算的一个程序（磁盘的一半去掉了，少一个库与一个应用，约 3–5 秒），约 40–50 秒，5:41 之后约 6:20–6:30，离线约一分钟。S1 量过之后改正（审查 C-N5，主持者的决定 26）：这一估计没有算 `prepareMatrix` 给它的每个用者多种的 `wlab`（第三份五个用者、第二份三个，3.14），S1 带修复在第三份的 CI 上多约 15–25 秒，比它的三个新测试多；照 S1 的数，加上这里的 S2、S3 的，第三份到 P1 结束约 6:45–7:00，仍在 7:30 之下，离线约 30–45 秒；S2、S3 的估计算上 `wlab` 的种。第二份照这里的估计（S1 的五个新测试 CI 上约 9.4 秒，变体的格在 15–25 秒之内）。S2 量过之后（10.2）：六个新的测试本机竞态下第三份的四个约 6.1 秒、第二份的两个约 1.1 秒（乘 1.73 约 10.6 与 1.9 秒），在这里的估计之内；`TestPermissionMatrix` 的变体的六行是 78 格（每行 13 格，不是约 30 格），并行跑；本机单独的第二份比起点多约 8.4 秒，第三份在波动之内；分支的两次 CI 的测试一步的平均是第一份 6:46.5、第二份 6:11.5、第三份 6:22，都在 7:30 之下，第二、三份在到 P1 结束的估计之下。

第二份离线不远，所以每个 Step 合并之后照 M9/P2 6.5 的规则读两次 CI 的测试一步的平均（第 10 节记下）：一份过了线时只在邻居收下之后两份都在线之下时挪界线（第三份在这里有约一分钟的余量），否则按测试名再分一份，从不提高 `-timeout`。e2e 加 9 个测试，在一个不分份的任务里（第 5 节）。

### 7.6 P1 怎样量

P1 本身不量要数据库的东西（不加成本守卫：建是两页、降级是一条 `UPDATE` 与一次点查、守卫不读库；预检的规则本身不读库，上传、`batch`、锁外撤销各多一次 `StateOf` 的点查，普通的笔记本也是；锁外撤销仅当规则需要 `page.manage_source` 时在本次 `Grants` 的回调里另读一次 `WorkspaceOf`，同一遍至多一次，锁下用已有的 `Write.Notebook` 与单元的 `Grants`（3.25，S3 审查 Q-A1、主持者的决定 32）；13.4 第 8、9 条的成本与计划测试从 P2 起）。实施时记下：四页模板的字节数（`wc -c wiki/domain/template-*.md`，写进第 10 节，测试钉住的是 8 KiB 的上限）；每个 Step 的分支与合并之后的 CI 的各份的时间（作业与测试一步分开写）；e2e 一步的时间；本机 `go test -race -count=1 -run '^(<新的测试名>)$' ./internal/bootstrap` 的时间（竞态检测下，同 CI），乘 1.73 作为 7.5 的估计的核对。

## 8. 完成标准

P1 完成，当且仅当：

1. 四个 Step 各自合并进 `main`，合并之前 6.5 的门禁全绿、审查与修复核对结束；每个 Step 合并之后两次 CI 的组合根三份都在 7:30 之下（7.5），否则照规则处理之后才开下一个 Step。
2. 第 3–5 节的每个测试在，7.3 的每个变异被它旁边的测试抓到（第 10 节记下）；3.29 的每个反向对照在测试里。
3. 第 2 节的移交的落实做完；三份移交写下进度，`M5-events.md` 在 P1 完成时改为 `done`（下面）。
4. 00 号文档（审查 C-M10，照 M9 的写法，`docs/v0.1/M9-mcp/00-M9-design.md:818-840`）：
   - **接受本文的文档提交**：2.1 的二十四处；头的"状态"（`:7`）改为"进行中（P1 设计已审查，S1 开工；[审查记录](reviews/M10-design-review.md)；…）"；第 12 节的 P1 一行（`:812`）写 `[01-P1](01-P1-contract-frame.md)`、"进行中（设计已审查）"、审查一格 `[设计](reviews/P1-design-review.md)`；第 13 节加"开工"一行（`:824` 写着"P1 开工时记"）与"P1 设计"一行（本文、2.1 的二十四处、[P1 设计审查](reviews/P1-design-review.md)）；本文与审查记录进 `docs/v0.1/M10-llm-wiki/` 与它的 `reviews/`。
   - **每个 Step 的文档提交（在 Step 的分支上、合并之前）**：第 12 节的 P1 一行加这个 Step 的审查记录与进度，合并的提交号待合并之后补；第 13 节加一行（实施结果、照实与设计不同之处、随之改的节）。合并之后在 main 的小文档提交里填入确切的合并提交号，Phase 的结果与审查记录同样更新。
   - **`docs: M10/P1 done`**：第 12 节的 P1 一行"完成"；头的"状态"改为下一个 Phase 的；第 13 节加"P1 完成"一行。
5. 下面的 v0.1 总体设计的修订，各在实现它的 Step 的文档提交里改（00 号文档"总体设计的修订"的"随实现它们的 Phase 改"里属于 P1 的部分；P2–P5 的部分留给它们）。

**v0.1 总体设计**（`docs/v0.1/v0.1-design.md`；新的文字照原文的写法，引号里是要写进去的句子）：

- **12.4** 写入守卫一行（`:982`）：S2，"M10（模式不变量：注册者 `page.NewModeGuard` 在 page，排在锁之前）"之后加"；次序在 `pageRegistrants(pool, authorizer)` 里"，同一格末尾的"`pageRegistrants(pool)` 里的登记次序"改为"`pageRegistrants(pool, authorizer)` 里的登记次序"。编辑会话一行的模式的否决者是 P2/S1 的。笔记本的创建与模式的变化两行已在（00 号文档定稿时写下），S1 核对它们的文字与 3.5 的签名一致（`NotebookCreation`、`ModeChange`）。
- **13.1 第 1 条**：
  - "页面的写入单元"一段（`:1031`），S1，"单元拒绝已经在事务里的 ctx"改为"单元拒绝已经在事务里的 ctx（唯一的例外是 `TreeWrites().Seed`：笔记本的模板在建它的事务里写，单元加入调用方的事务、重入同样的锁、时刻取调用方的，M10/P1）"；S2 在同一段末尾加"M10 起单元在笔记本行的锁下读一次它的模式与来源根（`Write.Notebook`），并带着单元自己的授权的备忘（`Write.Grants`，指针：步按值带着 `Write`，答案不能在副本里丢掉）；守卫与预检的模式的拒绝以 `Refused` 标出它拒绝的节点（单元另以一步的第一个节点标出别的守卫的问题），`batch` 答在它的项上，`Refused` 解开到问题，今天认问题的每一处照旧"（后台的任务以 `refused` 停下随 S3 的"后台的恢复与撤销"一段写）。
  - "树写入端口"一段（`:1032`），S3，"上传先不加锁地预检（`Check`，与单元同一套判定、名称与父页的规则）"之后加"（M10 起也判模式的契约，排在 asset 的 `storage_full` 之前）"。
  - "导入的单元"一段（`:1033`），S3，"撞名不答 409，按锁下读到的兄弟加序号"之后加"（LLM Wiki 的根下，四个保留的标题键 `index`、`log`、`index.md`、`log.md` 也算被占的，加序号、报告 `renamed` 问题，M10/P1）"（审查 C-M8：导入的那一句进它自己的一段，不附在恢复的一段末尾）。
  - "从回收站恢复"的那一段（`:1034`），S3，加"LLM Wiki 的根下，恢复把四个保留的标题键也当作被占的，加序号（同步的答 `renamed`，后台的数 `counts.renamed`）"。
  - "整组撤销"一段（`:1035`），S2：临时的名称的"守卫看得到（只凭名称认得，M10）"改为"守卫看得到（M10 的模式守卫照普通的改名、移动判它们，不凭名称认）"（00-M10 的"随实现它们的 Phase 改"里的这一条，审查 C-M8）。
  - "后台的恢复与撤销"那一段（`:1036`），S3，加"开始时的预检（`startRevert` 的 `CheckRevert` 与任务的计划 `PlanRevert` 共用撤销的不加锁的那一遍）判模式的契约，每一批由单元里的守卫判（一批是一个单元，不另检查）；被拒时任务以 `refused` 停下，报告的 `refusal` 写出码与节点（`treeError` 先认已有的哨兵、再认 `page.Refusal`）"。
- **13.1 第 2 条**（`:1040`），S1："页面的 HTTP 适配器按凭证给出（登录会话的访问令牌是 `web`，PAT 是 `api`，`clientOf`）"改为"HTTP 的适配器经 `shared.ClientOf` 按凭证给出（登录会话的访问令牌是 `web`，PAT 是 `api`；它读请求的 `Actor`，在 `CleanClientName` 旁边），page、asset、transfer、notebook 的适配器都用它、各自转换为自己的类型，没有第二份（13.4 第 13 条）"。
- **13.1 第 3 条**：S1 加 `notebook.change_mode`（笔记本管理员；降级，M11 的升级也用）；S2 加 `page.manage_source`（笔记本管理员；LLM Wiki 的来源区与来源根的树操作，模式的守卫经授权者问它，不比较角色，一个单元至多问一次）。
- **13.1 第 4 条**：S1 加降级的次序"`downgradeNotebook`：404 → 403 → 409 `notebook.not_llm_wiki` → 409 `transfer.busy`（同笔记本的管理写的锁序）"；S2 加"守卫的码在领域的 422、409 之后，模式的守卫在编辑锁之前；模式的码之间是 `page.protected` → `page.virtual_file` →（P2 的两个）→ `page.source_admin_only`；`batch` 的计划在编辑锁的预检之前、答第一个会被拒的项（码按项的次序：前一项的 403 在后一项的读的上限与 `differs` 之前）"；S3 加"预检的位置：上传的在 `page.title_taken` 之后、`storage_full` 之前；撤销的在 `page.revert_conflict`、它的 trimmed 与步数的 422 之后，`page.locked` 与 503 之前（被拒的撤销不占解析的预算）"。
- **13.1 第 5 条**（`:1059`），S1："建笔记本只锁工作区行"改为"建普通的笔记本只锁工作区行；建 LLM Wiki 时模板的单元在同一个事务里重入工作区行的 `FOR SHARE` 并以 `FOR NO KEY UPDATE` 锁刚插入的笔记本行（只有这个事务看得到它，不等）"。
- **13.1 第 9 条**，S1：加"建 LLM Wiki 读一次时钟：笔记本、管理员、模板的变更集、节点、正文与版本都是这一刻（`SeedSpec.At`）"。
- **13.1 第 10 条**，S1：加"降级记一行 info "notebook downgraded"（`workspace_id`、`notebook_id`、`user_id`）；建的一行加 `mode`"。
- **13.1 第 11 条**：S1 加 wiki 的模块根（P1 只有 `NewTemplate(seeds)`、`Actions()` 与给组合根的测试读的 `TemplatePages` 与三个常量，没有 `New`）与组合根的次序"page → asset → linking → wiki → transfer → notebook"、`transfer.NewWritingJobs(pool)`；S2 加 `page.NewModeGuard(authorizer)`（P2 加连接池）。
- **13.1 第 19 条**，S1：加"例外：模板的正文在启动时解析一次（`TreeWrites().ParseTemplate`，不经预算），提取结果在几次创建之间只读地共用"。
- **13.1 第 21 条**：
  - "组合在一处"一段（`:1109`），S2：`pageRegistrants(pool)` 改为 `pageRegistrants(pool, authorizer)`；"两处都是 `pageRegistrants(pool).sessionSubscribers`"改为"两处都是 `pageSessionSubscribers()`（命令行没有授权者，`pageRegistrants` 只在 serve；不收连接池，S2 的偏离 5）"。
  - S1 加两个新的扩展点与它们的最后一跳："笔记本的创建（`Seeder`，事务内，交回它写下的来源根；M10 的 wiki 的模板，最后一跳 `TestLLMWikiCreationWritesItsTemplate`、`TestTheTemplateRollsBackWithItsNotebook`）与模式的变化（`ModeSubscriber`；M10 的事件流，最后一跳 `TestTheModeEventReachesTheStreams`）；两个不只凭连接池构造的注册者：播种者要 `(*page.Module).TreeWrites()`（S1），模式的守卫要授权者（S2）"。
  - "行为测试"一段（`:1112`）：S1 加 M10 的两个扩展点的注册者与最后一跳（同上一条）；S2 加"M10 注册的模式的守卫排在锁之前，最后一跳在 `bootstrap/wiki_guard_test.go`（`TestTheModeGuardReachesEveryPath`、`TestModeComesBeforeTheLock`）与 `mcp_wiki_guard_test.go`（`TestTheModeGuardReachesEveryMCPWrite`），组合根交空时失败；有预检的 `batch` 的最后一跳是 `interleavings_wiki_test.go` 的 `TestTheGuardRefusesWhatThePlanLetBy`（`guards: []` 时同样的交错让写成功）"；S3 加"`treeError` 到 `refused` 的最后一跳 `TestTheSourceRootsPutBackIsRefusedInALaterBatch`"（审查 C-M8）；S4：前端"`app/event-stream.test.tsx` 用替身类型 `mode`"改为"用替身类型 `later`（M10 收尾时改名：`mode` 已是真的类型）"，M5 写给 M10 的最后一跳加"`bootstrap/wiki_downgrade_test.go` 的 `TestTheModeEventReachesTheStreams`（帧按契约的 `EventMode` 核对），前端经组合根的表的是 `app/event-stream.test.tsx`（`mode` 的处理）"。
- **13.1 第 29 条**（"否决者与守卫"一段，`:1131`），S2：`pageRegistrants(pool)` 改为 `pageRegistrants(pool, authorizer)`（S2 的文档提交补上：这一节起初没有列它）。
- **13.1 第 30 条**，S1：事件的类型加 `mode`（按笔记本过滤，帧是两个 id，发布的数据为空）。
- **13.2 第 1 条**（`:1160`），S4：加"`NotebookStore.downgrade` 经这本笔记本的 `inTurn`；答 403、404 与 409 `notebook.not_llm_wiki` 时照离开的写法重读列表（不在列表里的拿掉而不记作删除），409 `notebook.not_llm_wiki` 当作完成"。
- **13.2 第 17 条**，S4：加"持有的对话框（`HeldDialog`）由持有它的那一节移焦点：降级成功之后到"模式"一节的标题（`tabIndex={-1}`）；对话框在打开时不随触发按钮卸下，所以 403 之后重读的列表不会带走对话框里的拒绝"；树的行的种类的文字等价在可读的名字里、放在看得见的名字之后，照 `NewTab` 的写法：真的空格在 `sr-only` 之外、后缀以括号起头（`raw (source area)`），可读的名字在 Chromium 里的样子由 e2e 按确切的名字核对（jsdom 没有 Tailwind）。
- **13.2 第 23 条**（`:1192`），S4："M5 的 `pages`、`lock`，M6 的 `links`"改为"M5 的 `pages`、`lock`，M6 的 `links`，M10 的 `mode`（重读笔记本的列表）"。
- **13.3 第 3 条**（`:1216`），S1："写入在事务之前、判定之后、解析预算之内"之后加"（例外：模板的正文在启动时解析一次，13.1 第 19 条）"。
- **13.4 第 1 条**，S1、S3：反例加"模板的两种语言的段落对应（少一段、末尾多一段、少一项、一处行内代码不同与每种块的形状各一个，S1 的偏离 14 与审查 A-M1）与代码之外没有链接与标签；`checkPages` 的规则改为 `pageViolations` 交回违反的规则，每条新规则有一个 SQL 的种子作反例（M10/P1 起）；`schema_test` 的新名称与反例"。
- **13.4 第 3 条**：S1 加"`createNotebook` 的第六个参数是选项（`mode`、`templateLocale`）；模板的页以 `expectNewPage`、`expectChangesetOf` 核对，不另开 `assert/wiki.ts`"；S4 加"`test.use({ locale })` 只在 `test.describe` 里"（套件里第一处是 S4 的 LW1 (page)，审查 C-N5）。
- **13.4 第 4 条**，S1–S3：加 7.2 的八条交错的名字与它们怎样逼出（第 8 条是 S1 的修复一轮加的）。
- **13.4 第 6 条**，S1：加"矩阵的 LLM Wiki 的变体：同样的列在另一个工作区里有同样的角色，行带 `wiki`，`targetViolation` 按行的工作区核对"。
- **13.4 第 13 条**，S2、S3：S2 加"`CheckContract` 是模式的守卫与 `batch` 的计划共用的一组规则，事实照同一种写法建（守卫经 `FactsOf` 从步的改动，计划从它模拟的树，位置都是父节点与名字的标题键），不另写一份（M10/P1 的 S2）"；S3 加上传的 `Check` 与撤销的两遍（以 `FactsOf` 从它们计划的步建事实）。

**`docs/README.md`**：不改（`:103` 之后的 M10 的那一句已在）。README（仓库根的）的 LLM Wiki 一节是 P5 的。

**移交**（`docs/v0.1/M10-llm-wiki/handoffs/`，"处理进展"一节各加一行；00-M10 第 7 节"做完的 Phase 改它们为 `done`，收尾核对"，`M5-events.md:14` 同样，审查 C-M9）：

- `M5-events.md`：S1 合并时"P1/S1：类型 `mode`、发布者 `notebookEvents.ModeChanged` 与契约的 `EventMode`（最后一跳 `TestTheModeEventReachesTheStreams`）"；S4 合并时"P1/S4：前端的 `modeChanged`，替身测试的类型换成 `later`；本移交给 M10 的两项都做完"，`docs: M10/P1 done` 把它的 `status` 改为 `done`。`M8-history.md` 与 `M9-P1-mcp-registry.md` 照旧 `open`（P2–P5 做完它们）。
- `M8-history.md`：S2 合并时"P1/S2：模式的守卫与保护的页，撤销的每一步照它本来的操作判（第 2 项），守卫的码进各操作的 `x-problem-codes`（第 3 项的最后一条）"；S3 合并时"P1/S3：预检与后台的拒绝的去处（`refused`）；第 3 项的来源的四个问题是 P2/S1 的"。
- `M9-P1-mcp-registry.md`：S2 合并时"P1/S2：第 4 项（`AGENTS` 不能删除、改名、移动，正文照常）做完；第 7 项的移动加改名与撤销的临时名称照普通的改名、移动判，合成的正文与先恢复再写回随 P2/S1"。

**写给后面的 M 的项**（审查 C-M12；00-M10"写给后面的 M"：M11 的"发现它的 Phase 写进 `M11-upgrade/handoffs/`"，打磨的"追加到 `M12-release/handoffs/M5-polish.md`（第 33 条起）"）：

- **M11**：`docs: M10/P1 done` 新写 `docs/v0.1/M11-upgrade/handoffs/M10-mode.md`（`status: open`，照已有的三份的写法），收第 9 节"M11"一条里 P1 才发现的几项：`wiki.TemplatePages`、`wiki/domain` 的 `ContractOf` 是升级读的契约表；升级写成新模式后在同一个事务里调用 `ModeSubscriber`，事件流照样发 `mode`；`SetNotebookMode` 一次写模式与来源根，`notebooks_source_root_check` 许普通的笔记本先有来源根（3.2）；`Grants` 与 `Refused` 是冻结的守卫加码的地方（`refusal.code` 是自由的字符串）。第 9 节"M11"一条里别的项（`notebook.change_mode` 给升级、根下的保留的标题与 `AGENTS`）00-M10 已写给 M11，这里只指向它。
- **M12（打磨）**：P1 没有新的项：树的种类的图标再多一组本就在 00-M10 的打磨的列表里（"页面树的图标再多一组"），由 M10 收尾时与 P2、P4 加的图标一起追加为 `M5-polish.md` 的第 33 条起；草稿的"`ModeOptions` 与 `AccessOptions` 合成一个组件"去掉（审查 C 的过度设计：两个约 50 行的单选，P2–P4 不加第三个就不必）。

## 9. 交给后面的 Phase

- **S3**（S2 的审查留下的，主持者的决定 29）：四个 S2→S3 的窗口（第 0 节）的处理与核对见 10.3 和 [S3 的审查记录](reviews/P1-S3-prechecks-refusals-numbering-review.md)，已合并为 `a46c3d1d`（00 号文档第 12 节）。导入与恢复的加序号及 `checkPages` 的根下保留键规则对应 A-M2；撤销的两遍预检及 `TestPrechecksAnswerTheGuardsCodes` 的前一步碰到锁、后一步被模式拒绝的一行对应 A-M3；后台的开始时预检与 `refused` 的去处对应另两处窗口。这些窗口随 `a46c3d1d` 进入 main 并通过首次 main CI；完整 Step 审查、本机门禁与分支 CI 的既有证据保持不变。
- **P2/S1**：`page.NewModeGuard(pool, authorizer)`（加连接池）；模式的会话否决者与它的次序测试（登记在锁的否决者之前，3.20）；`NodeFacts` 加 `InArea`，祖先的一线在已读的三处交出、在改名、删除、上传、同父节点的 `Relocate`、撤销的放回补一次读（3.21）；`CheckContract` 的 `page.source_immutable`、`page.source_duplicate` 在 R3 与 R4 之间（3.21 的码的次序）；`batch` 的计划的正文的一遍与它的 `Needs`（3.22 的 `PlanBatch(…, n)` 已收契约，`Refuse` 已让码按项的次序）；新的码不改 `TransferRefusal`（`code` 是自由的字符串，3.26），只加网页的文字；`Step.Content` 的类型另取名（2.1 第 17 条）；矩阵的变体加来源与它的删除的行（管理员成功）；`TestTheDowngradeEndsTheContract` 加"来源成为可写"；守卫读连接池之后 `TestTheModeGuardLetsAStandardNotebookBy` 加一个计数的存储（普通的笔记本什么也不读，3.20）；后台撤销的一批若要在读写之前的检查，由 P2 的设计加回（2.1 第 19 条）；`ErrSourceAdminOnly` 的文字（"…rename, move or delete in an LLM Wiki's source area, its source root included"）照字面像是管理员能删来源根，`page.protected` 不许，网页的文字（4.2）已为此改写、服务端给 MCP 的 agent 读的没有，随来源区的规则看一次（S2 审查 A 的存疑）。
- **P2/S2**：降级的 `Forget` 插在 3.10 第 5 步之后；`ModeChange` 的订阅者不变；上传、同步的撤销与 `startRevert` 的只有守卫拒绝的情形（重复，3.25）；报告的拒绝可以指一个附件（重复的来源），`transfer.refusedSome` 说的"不再在树里的一页"与 `distinctName` 对附件不答（`W/pages/notebook/distinct-name.ts:11-22`）随之改，00-M10 的 `duplicateText` 已处理附件（审查 C-N9）。

- **P2/S3**："粘贴或上传为来源"的对话框的名字与模板的 `raw` 的那一句一致（3.9），不同时改模板；来源区的树的菜单在 4.4 的行的种类上加"来源"。
- **P3**：`lint.read` 进 `wiki.Actions()`（`actions_test` 的并集已有它）；MCP 的矩阵的 LLM Wiki 的变体（按行的工作区照 3.14 的写法）；模板在两种语言里 lint 干净（3.9 的"代码之外没有链接"是它的前提）。
- **P4**：`TestTheDowngradeEndsTheContract` 加"`list` 没有虚拟的两项"；LW12 加"虚拟的两行消失"。
- **P5**：`open_notebook` 与握手的系统的一段（P1 的 `open_notebook` 已答 `mode`）；LW1 (MCP) 加系统的一段的一步；`TestTheDowngradeEndsTheContract` 加"握手没有系统的一段"；模板讲 prompt 的那一句与系统的那一段同样的意思（00-M10 4.15）。
- **M11**（写进 `M11-upgrade/handoffs/M10-mode.md`，第 8 节）：`notebook.change_mode` 可以给发起与批准升级；升级经 `ModeSubscriber` 发同一个 `mode`；`notebooks_source_root_check` 许普通的笔记本先有来源根、`SetNotebookMode` 一次写模式与来源根（3.2、3.3）；`wiki/domain` 的 `ContractOf`（经 `wiki.TemplatePages`）是它读的契约表；`Grants` 与 `Refused` 是冻结的守卫加码的地方（`refusal.code` 是自由的字符串，3.19、3.22、3.26，审查 C2-N2）；升级要先处理根下的保留的标题与 `AGENTS`（4.9 的"写给后面的 M"）。
- **M12（打磨）**：树的种类的图标的一组（P2、P4 再加来源与虚拟的两行），M10 收尾时追加（第 8 节）。

## 10. 结果

四个 Step 的实施、审查与验证结果见 10.1–10.4；P1 完成的核对与后续移交见 10.5。

### 10.1 S1：模式与模板（服务端）（2026-10-25，合并 `45d086a1`）

- 提交（从 main 的 `f92ecac7` 开始，照 6.1 的次序，第 1–7 步，7 个；第 8 步的文档是主持者的）：
  - `shared.ClientOf`（`TestClientOfIsTheCredential`），page、asset、transfer 的适配器用它，三份 `clientOf` 删去（`46e3d20c`）。
  - 迁移 00048（`mode` 与 `notebooks_mode_check`、`source_root_id` 与 `notebooks_source_root_check`；`schema_test` 的两个名称、三个反例与照收的一行；`TestTheNotebooksModeGoesDown`）；五个读的查询与两处手写的转换带两列（`TestTheNotebooksAnswerTheirMode` 读五个），`CreateNotebook` 写它们；领域的 `Mode`、`Locale`、`CheckDraft`；模块根的 `State`、`StateOf`；契约的 `NotebookMode`、`Notebook.mode`/`source_root_id`，重新生成 Go 与 TS，网页的两个字面量（`d0ce2cf8`）。
  - page 的 `Writer.Seed` 与 `Run` 共用一个 `run`（`Seed` 加入调用方的事务，拒绝没有事务与零时刻，不读时钟），`SeedWrites`、`TreeWrites().Seed`、`ParseTemplate`，在真的库上测（`456fe026`）。
  - 建 LLM Wiki 带它的模板：`NotebookCreation`/`Seeder`/`seed`；建的事务照 3.4 的次序（读一次时钟，来源根的 id 在笔记本的之后取，播种者交回的根必须恰好是这本 LLM Wiki 自己的），`notebook: no seeders`；wiki 模块（`domain` 的契约表与四页模板、`app.Template`、模块根）；组合根的 `wikiSeeds`/`wikiTemplate` 与次序 page → asset → linking → wiki → transfer → notebook；契约的 `NotebookCreate.mode`/`template_locale`；建的一行日志带 `mode`；`checkPages` 分出 `pageViolations` 与 LLM Wiki 的规则（`TestPageViolationsCatchEachLLMWikiGap`）；`TestWorkspaceChangesAndCreatingAnLLMWiki`、`TestLLMWikiCreationWritesItsTemplate`、`TestTheTemplateRollsBackWithItsNotebook`；e2e 的 `createNotebook` 的第六个参数、`expectNewNotebook`/`expectNotebook` 读两个字段、LW1 (API)（`33edc078`）。
  - `downgradeNotebook`：`ActionChangeMode` 与规则 `notebook.change_mode`（笔记本管理员），`ErrNotLLMWiki`、`ErrJobsWriting`（`transfer.busy`），`SetNotebookMode`，`ModeChange`/`ModeSubscriber`/`publishMode`，`Deps.WritingJobs`（`notebook: no writing jobs`）与 `transfer.NewWritingJobs(pool)`（`TestTheWritingJobsAreTheTreesWriters`）；用例在锁下读模式、问一次任务、读一次时钟，记一行 "notebook downgraded"；HTTP 适配器、契约的路径、网页的 `notebook.not_llm_wiki` 的中英两条文字；权限矩阵的变体 `wlab` 与降级的两行；`TestTheDowngradeAnswersInItsOrder`、`TestImportsAndDowngradingAnLLMWiki`（`32bba1f8`）。
  - `mode` 事件：`notebookEvents.ModeChanged`，事件流是唯一的模式订阅者，契约的 `EventMode`；`TestTheModeEventReachesTheStreams`（`e8962f7b`）。
  - MCP 答模式：page 的 `Mode`/`NotebookState` 与端口 `Notebooks.StateOf`，`OpenNotebook` 在门之后读状态（那时已删的答不存在）并答 `Mode`，组合根的 `pageNotebooks`；`openNotebook` 与 `notebooks` 答 `mode`，Go 里的描述与 `tools.json`；`TestNotebooksAnswerTheirModeOverMCP`；MC1 期望 `mode: "standard"`，LW1 (MCP)（`66602404`）。
  - 每个提交单独在一个工作树里 build、vet、碰到的包的静态检查（0 个问题）与 short 测试都通过；碰网页的提交另过 oxfmt、oxlint、tsc、knip 与 vitest（166 个文件、3,344–3,346 个测试）。第 4 步的修正（下面第 14 条）之后第 4–7 步重放、再核对一次。审查 C 另在每个提交上跑整个模块的静态检查、tidy 与整个 short 测试，那个提交加或改的库上的测试，第 2、4、6 步的整个 e2e，都通过。
  - 修复一轮另有 13 个（下面），一共 20 个，合并 `45d086a1`。
- 与设计不同之处（实施时定下的，都是最小的改动；已改在各节，3.30 列出）：
  1. 3.14 说的 `TestCreateNotebookLogs` 不在：建的那一行一直在 `TestCreateNotebookMakesTheCallerItsAdmin` 里核对。`mode` 在那里的两行（普通的、LLM Wiki）核对，同已有的"不带名称"；3.10 的"照 `TestCreateNotebookLogs` 的写法"随之改（审查 A）。
  2. 模板的变更集的说明在 `wiki/domain.TemplateMessage(locale)`，与模板放在一起（设计给了两句话，没说放在哪里）；`contract_test` 钉住每种语言的不为空、`fr` 的为空，整个程序按语言比较它（x5）。
  3. `TestLLMWikiCreationWritesItsTemplate` 另数两页的 `indexed_pages`（2）。实施者的理由（`page_links`/`page_tags` 的零行对一份没被解析过的模板也成立）不对：链接索引的观察者拒绝不是 Markdown 的事实，没解析过的模板让建答 500（审查 A）；这一数加的是链接索引对模板的两页的最后一跳，事实是不是正文的由 `TestATemplatesParseIsItsContents` 核对（审查 A-M3）。
  4. `TestPageViolationsCatchEachLLMWikiGap` 在 `interleavings_page_test.go`，在 `pageViolations` 旁边；每一行撤回之后再断言违反是空的，一行的撤回漏不到下一行。
  5. 没有 `page/domain/mode_test.go`：3.1 列了这个文件、没有说测什么；它只有两个常量与一个结构。notebook 的模式到 page 的转换由 `TestNotebooksAnswerTheirModeOverMCP` 从头到尾钉住（另加的变异 x1 被它抓到），修复一轮另有 `TestPageNotebooksConvertTheState`（审查 C-N4）。
  6. `mode_test.go` 的调用列表照已有的替身的写法（`"LockNotebook in tx"`、`"Authorize notebook.change_mode in tx"`、`"Writing in tx"`、`"SetMode standard by <alice> in tx"`、`"ModeChanged a llm_wiki standard in tx"`）：notebook 的替身不记 id，设计的 `"… nb"` 写作不带 id；次序照设计。要紧的 id 在别处钉住：授权者的目标、`moded` 对 `standard` 的核对、按 id 答的假的任务（审查 B、C）。
  7. 矩阵的变体用它自己的表（`wikiMemberships`、`wikiNotebooks`、`wikiNotebookMembers`，都照 lab 的）种进它自己的 id（`seeded.wiki`），不加进 `matrixWorkspaces()`、`matrixMemberships()`、`matrixNotebooks()`：`prepareMatrix` 的别的用者遍历它们（`TestTheSearchIsWhatEachReadAllows` 搜每个 `matrixWorkspaces()`，`events_lab_test` 给每个 `matrixNotebooks()` 发布），且 lab 与 wlab 的笔记本同名、`seeded.notebooks` 按名称取。`workspaceOfRow` 先认变体的 id。删一本种下的笔记本分成 `prepareMatrix` 里的 `deleteNotebook`，lab 的 gone-nb 与 wlab 的共用。审查 B：比设计的写法好。
  8. 除了 3.14 说的 `accountNotebooksOf`，另两个测试读一列的每个工作区，也要读 wlab，否则在第 5 步失败：`TestTheStreamSeesWhatEachReadAllows` 与 `TestTheAccountsNotebooksAreWhatEachReadAllows`（`notebook_visibility_test.go`）改读 `seededNotebooks(s)`（lab、other 与 wlab 的），事件流那个测试的工作区的循环读 `seededWorkspaces(s)`。
  9. `targetViolation` 取这一行的列所在的工作区的 slug（`workspaceOfIn(c, r)`），不取列；`workspaceOf(c)` 留给各行自己的请求（变体的行从 `s.wiki` 建路径）。第 22 条变异把它换回去。
  10. 变体的 `raw` 与 `AGENTS` 的正文是英文模板的两页（`wiki.TemplatePages`），不是矩阵的标记：lab 的搜索的行找不到它们，页也正是一本 LLM Wiki 里有的。`wikiSeeded.raw`/`agents`（3.14 的 `s.wiki.raw(name)`、`s.wiki.agents(name)`）由种的那一步用：S2 的行用它们之前，lint 的 `unused` 不许它们空着。
  11. `TestImportsAndDowngradingAnLLMWiki` 两种次序都持住导入的唯一的 worker（`holdImportWorker`）：否则"开始在先"里 worker 可能在开始的提交与降级的读之间把任务做完，那是测试没有逼出的次序。两种次序随后放开 worker，等导入以 `succeeded` 结束。审查 B：持住是要的；它的 b7（在锁外读 `Writing`）让"开始在先"答 200，次序是逼出来的。
  12. `TestTheModeEventReachesTheStreams`：LLM Wiki 是私有的，alice 是它的管理员，bob 加作读者，carol 是 acme 的成员、看不到它；标记是 `Eng` 的一页（acme 的成员都看得到），三人都收到（`quiet`）。
  13. 替身的 `SetMode` 拒绝表的检查拒绝的（两种之外的模式、没有来源根的 LLM Wiki）：替身在真的失败处失败。修复一轮让替身的 `CreateNotebook` 经同一个 `checked(n)` 同样拒绝（审查 A-N2）。
  14. 3.29 的"少一段"的反向对照起初不承重：去掉块数的比较，三个反例照样被逐块的比较抓到。`TestTheTemplatesCorrespond` 加第四个反例"末尾多一段"（中文多一段），修正并入第 4 个提交（第 4–7 个提交重放，sha 变了）。修复一轮另加每种块的形状各一个反例（审查 A-M1）。
  15. LW1 (API) 并行建三本 LLM Wiki（`Promise.all`）：oxlint 的 `no-await-in-loop` 不许循环里等；每一本照设计核对，422 与 `countNotebooks` 在三本都结束之后（审查 C：三次建只共用 acme 的行的 `FOR SHARE`）。
- 实测（本机，竞态检测下，经 `heavy.sh`，除非注明）：
  - 模板的字节（`wc -c wiki/domain/template-*.md`，不经 heavy.sh）：英文的 `raw` 648、`AGENTS` 2,209，中文的 `raw` 593、`AGENTS` 2,107，共 5,557；测试钉住的是每页 8 KiB（修复一轮起写出 `8 << 10`）。
  - 八个新的整个程序的测试（`go test -race -count=1 -run '^(…)$' ./internal/bootstrap`，开始时负载 1.8）：包 11.9 秒，测试本身 10.3 秒。第二份的五个共 5.4 秒（`TestPageViolationsCatchEachLLMWikiGap` 1.95、`TestImportsAndDowngradingAnLLMWiki` 1.66（0.93 与 0.73）、`TestTheDowngradeAnswersInItsOrder` 0.69、`TestLLMWikiCreationWritesItsTemplate` 0.59、`TestNotebooksAnswerTheirModeOverMCP` 0.52），乘 1.73 约 9.4 秒；第三份的三个共 4.9 秒（`TestWorkspaceChangesAndCreatingAnLLMWiki` 3.06，三个子测试各 1.02；`TestTheModeEventReachesTheStreams` 1.31；`TestTheTemplateRollsBackWithItsNotebook` 0.49），乘 1.73 约 8.4 秒。都在 7.5 的估计之内（P1 第二份的八个约 25–30 秒）。`TestPermissionMatrix` 带变体的行单独跑，包 21.0 秒。
  - 修复一轮改了的测试：`TestPageViolationsCatchEachLLMWikiGap` 2.09 秒、`TestWorkspaceChangesAndCreatingAnLLMWiki` 3.70 秒（新的子测试 0.53）、`TestNotebooksAnswerTheirModeOverMCP` 0.52、`TestLLMWikiCreationWritesItsTemplate` 0.56、`TestPageNotebooksConvertTheState` 0.00 秒。
  - e2e（不带竞态）：建成时 `make e2e` 284 个 55.4 秒；LW1 (API) 50 毫秒、LW1 (MCP) 29 毫秒、MC1 989 毫秒。修复一轮 284 个 36.0 秒；审查 C 与修复核对 C 284 个 36.4、36.9 秒。
- 反向对照（每一条的结果见 [S1 的审查记录](reviews/P1-S1-mode-template-review.md)"反向对照"）：
  - 7.3 的第 1–22 条都被它点名的测试抓到。第 7 条的第一种写法留下没用的 import、编译不过，改成以 `%w` 包着问题之后两个测试都抓到（原始记录是编译不过的那一次，审查 C-N1 重跑）。第 12 条照字面写（去掉 `Message:`）时组合根交的是空字符串，`changesets_message_check` 拒绝、建答 500，两个测试因此失败，不是因为比较了说明；另写 12b（`wikiSeeds.Seed` 交 `nil`，库照收的一行）：e2e 的 LW1 (API) 比较说明抓到（`"message": null`），`TestLLMWikiCreationWritesItsTemplate` 建成时在读说明时失败，修复一轮之后在比较处（`coalesce`，主持者的决定 25）。7.3 的第 12 条照此分成 a、b。
  - 3.29 的 S1 各行：去掉它所测的检查，都被抓到（rc01–rc21；与 7.3 同一处改动的记在那里：`ClientOf` 第 10 条，播种者的交回第 3、4 条，播种者的问题第 7 条，`Seed` 的时钟第 9 条，降级的次序第 15 条，锁下的模式第 18 条，`checkPages` 的 `deleted_at` 第 20、21 条，五条查询第 14 条）。"少一段"起初不承重（上面第 14 条），加第四个反例之后抓到，在 `66602404` 上再跑一次也抓到。降级的"写者在普通的笔记本上答 403"先对 `TestTheDowngradeAnswersInItsOrder` 跑、照过（那个测试没有写者的一行，本就该照过），对 3.29 点名的 `TestDowngradeNotebookRefuses` 的"an editor, a standard notebook"一行抓到。模板的回滚：创建不管播种者的错误时日志里没有触发器的话，触发器改拦 `raw` 时序列数到 1，都被 `TestTheTemplateRollsBackWithItsNotebook` 抓到。
  - 另加五个变异（x1–x5），都被抓到：`pageNotebooks.StateOf` 总答普通（`TestNotebooksAnswerTheirModeOverMCP`）；MCP 的 `notebooks` 与 `openNotebook` 总答 `standard`（各自的适配器测试）；`OpenNotebook` 不管门之后已删的笔记本（`TestOpenNotebookAnswersInOrder`）；模板的说明总是英文的（`TestLLMWikiCreationWritesItsTemplate` 的中文一行）。
  - 审查者的变异里存活的（A 的 a1、a3、a10–a13、a19，B 的 b1、b2，C 的 C1、C35、C40）各成了一条发现，修复一轮之后都被抓到，连同修复一轮自己的（x7–x10）与修复核对 A 的 f2 写成 7.3 的第 65–80 条。修复一轮在 `86e4c8da` 上重跑 7.3 的第 1–22 条（26 次，含 a/b）、x1–x5、12b 与 rc01–rc20、rc21b（rc11 另照它原来的写法再跑一次），54 行都被抓到（`fix1/rerun.out`）。
- 门禁（本机）：建成时（`66602404`）`make lint gen-check knip build` 通过（约 32 秒）；经 `heavy.sh`：`make test` 928 秒（竞态与组合根三份同时跑，之后成本与守卫，`test-web` 166 个文件、3,346 个测试），`make test-go-short` 62 秒，`make e2e` 284 个 55.4 秒。修复一轮（`86e4c8da`）`make lint gen-check knip build` 通过；经 `heavy.sh`：`make test` 838 秒，`make test-go-short` 54 秒，`make e2e` 284 个 36.0 秒；13 个提交各自 build、vet、碰到的包的静态检查与 short 测试，网页的一个另过网页的门禁与 `make gen-check`。
- 组合根的各份（6.5 的规则）：`go test -race -list` 核对，建成时 419 个测试、三份 168、116、135，第二份的五个、第三份的三个照 7.5 的表，第一份没有新的；修复一轮之后 420 个、168、117、135（`TestPageNotebooksConvertTheState` 在第二份，新的子测试在第三份的测试里）。本机单独两次：建成时第一份 257.5、266.2，平均 261.8 秒；第二份 150.6、159.9，平均 155.2；第三份 198.2、192.1，平均 195.1。S1 的起点 `f92ecac7` 同样两次：第二份 148.3、143.4，平均 145.8（+9.4）；第三份 184.1、179.9，平均 182.0（+13.1）。修复一轮：第二份 155.2、149.8，平均 152.5；第三份 186.6、193.7，平均 190.1。第三份多出的比它的三个新测试多：`prepareMatrix` 给它的每个用者多种一个 wlab，`TestTheStreamSeesWhatEachReadAllows`（第三份）也读 wlab 的笔记本（上面第 8 条）。CI 与本机之比：1.19、2.44、1.91；S1 带修复在 CI 上第二份约多 17–23 秒、第三份约多 15–25 秒，第三份到 P1 结束约 6:45–7:00，7.5 照此改正（审查 C-N5，主持者的决定 26）。
- CI：分支的 run 40098894279（`66602404`）全绿，共 14:24（作业，括号里是测试一步）：竞态 9:51（9:30）、组合根一 5:28（5:11）、二 6:42（6:19）、三 6:36（6:13）、守卫 8:45（8:22）、成本 8:15（6:05）、short 3:20（3:01）、web 4:07（3:36）、e2e 4:29（E2E 一步 3:40）、镜像 1:28。修复一轮的 run 40107114331（`86e4c8da`）全绿，共 16:51：竞态 8:13（7:55）、组合根一 7:33（7:12）、二 5:33（5:12）、三 5:28（5:07）、守卫 12:46（12:27）、成本 8:10（6:03）、short 3:23（3:03）、web 4:08（3:34）、e2e 4:01（E2E 一步 3:09）、镜像 1:14。三份的测试一步都在 7:30 之下；第一份没有 S1 的测试，5:11 → 7:12 是 CI 的波动（第二、三份反过来快了）。照 M9/P2 6.5 的规则的算法，两次的平均是 6:11.5、5:45.5、5:40，都在 7:30 之下；界线没有挪、没有分、没有提高 `-timeout`，CI 的文件与 `Makefile` 没改。
- 设计的文字照建成的与修复一轮改，已改在各节（3.1、3.4、3.5、3.6、3.8、3.9、3.10、3.12、3.13、3.14、3.15、3.16，第 5 节，3.29，7.1、7.2、7.3、7.5，第 8 节的两处；3.30 列出）；7.3 的第 12 条分成 a、b，加第 65–80 条。
- **修复一轮**（2026-10-24，分支 `m10-p1-s1` 的 `86e4c8da`；三份审查与主持者的决定，`fix-brief-1.md`）：从 `66602404` 起 13 个提交，一组一个，每个提交单独 build、vet、碰到的包的静态检查与 short 测试都通过：
  - 次要的：
    - B-M1 = C-M1 `cb9de147`：`TestPageViolationsCatchEachLLMWikiGap` 加"AGENTS under raw"（规则的 `a.parent_id IS NULL`）。
    - B-M2 `9daac068`：`TestADowngradePassesTheModeSubscribers` 读 `updated_by_id`（行先由 bob 改过）。
    - A-M1 `d37f57aa`：`TestTheTemplatesCorrespond` 加每种块的形状各一个反例。
    - A-M2 `ec5e67b4`：`TestTheTemplateSeedsAnLLMWiki` 在创建之后再数解析，page 的新的 `TestASeedPassesTheFactsItIsGiven`。
    - A-M3 `e21332db`：`TestATemplatesParseIsItsContents` 以 `pageMark()` 解析、核对 `page-mark`。
  - 小问题：
    - A-N1 = C-N2 `5008ee7d`：变更集经 `coalesce` 读（主持者的决定 25）。
    - A-N2 `3884692a`：假的 `CreateNotebook` 照表的检查拒绝。
    - A-N3 `75e34bb8`，这一轮唯一的生产的改动：`TreeWrites().Seed` 与 wiki 的 `Seeds` 端口交回写下的页的 id，模板交回写下的来源根（主持者的决定 23）；新的 `TestTheTemplateAnswersTheSourceRootWritten`。
    - A-N4 `ee464e7d`：`TestWorkspaceChangesAndCreatingAnLLMWiki` 的第四个子测试，钉住 PostgreSQL（主持者的决定 24）。
    - B-N1 `670765c6`：`TestNotebooksAnswerTheirModeOverMCP` 降级之后再读 `notebooks`。
    - B-N2 = C-N6 `2e954be5`：`createNotebook` 的说明不重复，重新生成。
    - C-N3 `9c2da5c7`：`TestTheTemplatesAreSmall` 写出 8 KiB。
    - C-N4 `86e4c8da`：`TestPageNotebooksConvertTheState`；`open_notebook` 对门之后被删的笔记本答现有的 `notebook.not_found`（404），`TestTheOpenNotebookAnswer` 核对适配器原样交回它。
    - C-N1：7.3 的变异、另加的与反向对照在修好的头上重跑，记录留在 `fix1/rerun.out`。C-N5：两份重量（上面），7.5 改正（主持者的决定 26）。
  - 实施者在这一轮一次在 `heavy.sh` 之外跑了 `TestPageNotebooksConvertTheState`（不要库，没有起容器）；之后每次都经 `heavy.sh`。
- **修复核对一轮**（2026-10-25，三位各在 `86e4c8da` 的新的工作树里）：三位都说自己的与自己范围里别人的每一条都改了，没有新的发现（Important 0、Minor 0、Nit 0）。A-N3 的生产的改动：建的 `seededItsRoot` 不改，一页以别的 id 写下时建答 500（A 的 h5、B 的 b8、C 的 F6；F6c 证明那就是这一处核对，F6b 显出 A-N3 之前只有整个程序的核对抓得到）。新的子测试是逼出来的（C 的 F7、F7c），但没有我们的代码的变异让它失败（A 的 h7），是 PostgreSQL 的回归测试（主持者的决定 24）。13 个提交各自绿（C）。
- 实施者留给审查与主持者的一项（`TestLLMWikiCreationWritesItsTemplate` 把说明读进 `string`）由修复一轮的 `coalesce` 改正（主持者的决定 25）；文档照偏离改的各处，由这一节与 3.30 记下。

### 10.2 S2：守卫的骨架（2026-10-25，合并 `bad859c0`）

- 提交（从 main 的 `45d086a1` 开始，照 6.2 的次序，第 1–4 步，4 个；第 5 步的文档是主持者的）：
  - 单元在锁下读笔记本的状态：`ShareNotebook` 选出 `mode`、`source_root_id`，notebook 的根的 `ShareState`、`LockState`（`notebooks_test.go` 的四行：LLM Wiki、普通、已删、没有）；page 的端口 `ShareByID`、`LockByID` 交回 `(NotebookState, bool, error)`，组合根的 `pageNotebooks` 经 `toPageState` 转换（`TestPageNotebooksConvertTheState` 读三种读法）；`Write.Notebook`、`Write.Grants` 与 `Grants`（`TestAGrantIsAskedOnceAUnit`）；page 的几个假的 `Notebooks` 交回状态；`TestTheGuardSeesTheCheckedChange` 以 `want.Grants = s.Grants` 比较（`70b0ccdd`）。
  - `CheckContract`：`StepFacts`、`NodeFacts`、`NodePlace`、`FactsOf`、`Verdict`、`ReservedKeys()`、`Reserved`，R1、R2 → R3 → R4；三个错误、`Refused`/`Refuse`；`ActionManageSource` 与它的规则（笔记本管理员）；`TestTheReservedKeysAreTitleKeys`、`TestCheckContract`（3.21 的表，每一行另对来源根的 id 相同的普通笔记本答零）、`TestFactsOfAStep`、`TestARefusalIsMarkedOnce`（`ee64833c`）。
  - 模式的守卫：`app.ModeGuard`、模块根的 `NewModeGuard` 与 `Refusal`；`Unit.apply` 以这一步的第一个节点标出守卫的问题；`batch` 的 `failed` 先认 `Refused`；`pageRegistrants(pool, authorizer)` 的 `[mode, lock]`、`pageSessionSubscribers()`；六个操作的 `x-problem-codes` 与说明，重新生成 bundle 与 TS；page 与 asset 的适配器的表的行；`mcpToolCodes()` 与 `wikiCodeCalls`；矩阵的变体的六行与 `adminsOnlyOr`；三个码的中英文字；`TestTheModeGuardLetsAStandardNotebookBy`、`TestTheModeGuardAsksOnceAUnit`、`TestAUnitMarksItsGuardsRefusals`、`TestABatchAnswersARefusalAtItsOperation`、`TestRefusalReadsTheMark`；3.24 的 `TestTheModeGuardReachesEveryPath`、`TestTheModeGuardReachesEveryMCPWrite`、`TestModeComesBeforeTheLock`、`TestTheDowngradeEndsTheContract` 与 3.23 的 `TestTheProtectionAndADowngrade`；e2e 的 LW2 (API)、LW2 (MCP)（`99ec6abe`）。
  - `batch` 的计划判契约：`PlanBatch` 收笔记本的状态，树的一遍在每一项自己的检查之后判它的步（`judge`），`BatchPlan.Needs`、`Need`、`BatchPlan.Refuse`；`planned` 在 `ListNodes` 之后不加锁读 `StateOf`、在 `a.read` 之前问 `page.manage_source`（`grant`、`decidedNotebook`）；`TestThePlanChecksTheContract`、`TestABatchsPlanRefusesBeforeItsLock`；`TestTheGuardRefusesWhatThePlanLetBy` 与收 `wiring` 的 `newAcmeTeamWiring`（`cfa1c608`）。这个提交在推送之前修订过一次，加了下面第 14 条的一行；修订之前的 sha 是 `7fa387da`，实施者的报告的提交列表起初写的是它（审查 C-N2）。
  - 每个提交单独在一个工作树里 build、vet、碰到的包的静态检查（0 个问题）与 short 测试都通过（要库的包的 `-short` 经 `heavy.sh`）；第 3 个提交另过网页的 tsc、oxlint、oxfmt 与 vitest（problem-messages 与 i18n 的 3 个文件、1,093 个测试）。审查 C 另在每个提交上跑整个模块的静态检查、tidy 与整个 short 测试，第 1、3 个提交上的生成物与 `make gen-go` 相同，第 3 个提交上网页的整个门禁（vitest 166 个文件、3,349 个测试），都通过。
  - 修复一轮另有 4 个（下面），一共 8 个，合并 `bad859c0`。
- 与设计不同之处（实施时定下的，都是最小的改动；已改在各节，3.31 列出）：
  1. notebook 的存储的 `ShareNotebook` 答 `(domain.Mode, uuid.UUID, error)`，不答 `State`：适配器不能导入模块根的 `State`，根由这两个值建它。
  2. 事实的位置叫 `NodePlace`，不叫 `Place`：`page/domain` 已有 `Place`（兄弟间的次序，`order.go:13`）。
  3. `ReservedKeys` 是函数 `ReservedKeys()`，不是变量：lint 的 `gochecknoglobals`（同 `Actions()`）。
  4. `page.manage_source`（动作、`Actions()` 与规则的一行）在第 2 个提交，不在第 3 个：`CheckContract` 的 `Needs` 要它，`TestTheRuleTableIsTheModulesActions` 要两边在同一个提交。第 1 个提交的 `TestAGrantIsAskedOnceAUnit` 因此以 `node.move`、`node.rename` 作问的动作。
  5. `pageSessionSubscribers()` 不收连接池：它唯一的订阅者是事件流的发布者，只要它的 NOTIFY（`events.NewPublisher()`），收池是一个没用的参数。第 0 节、2.1 第 6 条、3.17、3.20、第 8 节、00 号文档 4.1 与第 7 节的 P1 一行、总体设计 13.1 第 21 条随之写作不带池。
  6. MCP 的 `move` 的参数是 `from`、`to`（`api/mcp/tools.json`），3.22 的行写的是 `path`；`wikiCodeCalls`、`TestTheModeGuardReachesEveryMCPWrite`、LW2 (MCP) 发 `from`。`wikiCodeCalls` 的写者是经 SQL 加进 acme 的新账户 erin（读的应用里的 carol 只能看），它的 PAT 发写者的 `move raw.md`。
  7. `uploadAsset` 的说明的新句放在"The checks are made again as the attachment is created."之后：S2 里上传的预检不判契约（S3 的 `Check` 判，3.25），只有单元里的建拒绝它。`revertChangeset` 的一句只说每一步照它的操作判，不说与 S3 的预检的次序（它之后已有的"Then … page.locked"在 S2 对一个形状不成立，即审查 A-M3）。
  8. 批量问 `page.manage_source` 要笔记本的工作区：`decideNotebook` 包着新的 `decidedNotebook`，它另答工作区，`Execute` 把它留在这一次尝试里（不多读一次）。
  9. 树的一遍在每一项自己的方法里判（`judge`），在这一项自己的检查之后、它记下改动之前（3.22 写的是"模拟每一项之后"）：事实相同，被拒的一项不在计划的树里留下改动。一项的拒绝在同一项上批量自己的界线（移动带着的节点、项数）之前：分开的调用没有这些界线。
  10. `Refuse` 也取代同一项上的失败（批量的界线，`final`），并清掉 `final`，让正文的一遍走到被拒的那一项之前（"更早的一项的正文的失败照旧在先"）。修复一轮为这两种行为加了测试（审查 A-M1 = C-M2）。
  11. 没有 `batch` 的测试钉住整个调用列表（计划开始读 `StateOf` 时一个也没有失败），设计的 `"StateOf nb"` 没有地方加；替身记 `"StateOf"`、不记 id（同 S1 的第 6 条）。`TestABatchsPlanRefusesBeforeItsLock` 的最后一行核对 `StateOf` 紧跟 `ListNodes`、不在事务里。
  12. `TestTheModeGuardAsksOnceAUnit` 的拒绝的一行：第 4 个提交起计划在任何单元之前拒绝被拒的 `batch`，这一行改为计划放行、锁下拒绝（记录器在 `"LockNotebook in tx"` 时把放行改成拒绝，等于其间角色变了），仍是守卫的拒绝；核对单元里问一次、计划问一次。替身授权者默认答 `ErrNotVisible`：第 3 个提交里守卫的 `Grants` 把它当拒绝，第 4 个提交起计划把它答成笔记本的 404。替身授权者加 `failing`（数据库的错误的一行）。
  13. 批量的移动没有兄弟间的位置：`TestThePlanChecksTheContract` 的 B-Q1 一行（排序祖先）写成祖先在同一父节点下的改名，与移到原处（不是一步）。
  14. `TestABatchsPlanRefusesBeforeItsLock` 另加一行"写者的 `[move raw, edit AGENTS]`，AGENTS 被别人的会话持着"答第 0 项的 403：设计的那几行里被持着的 AGENTS 的删除在树的一遍就被拒（第 1 项的 409），到不了编辑锁的预检，这一行才钉住计划的问在编辑锁的预检之前。
  15. 第 23 条变异：存储答 `standard` 与零的来源根（查询的两列照选、不读：变异之下不重新生成 sqlc）。第 29 条照字面写不出：改名的一步只有那个节点，P1 不读祖先，契约认不出"祖先的改名"，变异写成"每个改名都要 `page.manage_source`"。第 32 条照它的效果写：步按值带着 `Write`，`Grants` 是值时每一步是单元的一份副本，变异给每一步一份 `Grants` 的副本，不改类型（改类型要改几个测试文件，编译不过不算抓到）。第 40 条：计划把它的放行经 ctx 的一个值交给单元，单元的 `Grants` 从它开始。第 34、38、39、40 条与最后一跳改两三个文件，`mut2.py` 是 `mut.py` 多文件的写法。

  另：`TestTheModeGuardReachesEveryMCPWrite` 的 agent 的变更集按 `client LIKE 'mcp%'` 找：`acmeTeam` 的 REST 请求带登录会话，它们的变更集的客户端是 `web`，不是 `api`（审查 C：之后要的恰是 agent 的那几个，多一个被拒的变更集就失败）。
- 实测（本机，竞态检测下，经 `heavy.sh`，除非注明；机器同时有别的会话，各次开始时负载 2.0–6.1）：
  - 六个新的整个程序的测试（`go test -race -count=1 -run '^(…)$' ./internal/bootstrap`，开始时负载 4.6）：包 9.1 秒。第三份的四个共 6.1 秒（`TestTheProtectionAndADowngrade` 2.42（1.88 与 0.54）、`TestTheGuardRefusesWhatThePlanLetBy` 2.03（0.91、没有守卫的 0.55、0.57）、`TestTheModeGuardReachesEveryPath` 1.13（两本各 0.32、0.28）、`TestTheModeGuardReachesEveryMCPWrite` 0.56），乘 1.73 约 10.6 秒；第二份的两个共 1.1 秒（`TestTheDowngradeEndsTheContract` 0.58、`TestModeComesBeforeTheLock` 0.53），乘 1.73 约 1.9 秒。都在 7.5 的估计之内。审查 C 在负载 8–9 时量得包 9.4 秒，各测试在这些数的几个百分点之内。
  - 改过的：`TestTheMCPToolsErrorsCarryTheirCodes`（第三份）5.59 秒；它与 `TestPermissionMatrix` 一起的包 30.3 秒（S1 记下矩阵单独 21.0 秒）。S2 的矩阵的六行各 13 格、共 78 格（7.5 估的是约 30 格），格的时间之和 43.5 秒，并行跑。修复一轮改的 `TestTheModeGuardReachesEveryMCPWrite` 单独 2.29 秒（包里的第一个测试，带着应用的启动），`TestThePlanChecksTheContract` 多 0.04 秒。
  - e2e（不带竞态）：`make e2e` 286 个 37.0 秒（整个目标 39 秒）；LW2 (API) 80 毫秒、LW2 (MCP) 65 毫秒、LW1 (API) 58 毫秒、LW1 (MCP) 37 毫秒、MC1 205 毫秒。修复一轮 286 个 36.9 秒。
- 反向对照（`mut2.py`：`mut.py` 的多文件的写法；每个文件改完即复原，`git status` 照旧干净；每一条的结果见 [S2 的审查记录](reviews/P1-S2-guard-review.md)"反向对照"）：
  - 7.3 的第 23–41 条（建成时与修复一轮在 `7834bd70` 上各跑一次，各 27 次：25、26、27、30、31 各对 `TestCheckContract` 与 `TestTheModeGuardReachesEveryPath` 跑一次，34 对三个测试，41 分 a、b）都被它点名的测试抓到；没有编译不过、命令没跑或不点名的失败。第 29、32、23、40 条的写法见上面第 15 条。
  - 3.29 的 S2 各行：要改代码才看得到的另跑 13 次，都被抓到：组合根不登记模式的守卫（`{lock}`）时 `TestTheModeGuardReachesEveryPath`（模板的撤销答 200）、`TestTheModeGuardReachesEveryMCPWrite`（`write index/x.md` 写下）与 `TestModeComesBeforeTheLock` 都失败（最后一跳）；计划不看笔记本的状态、移动不列后代、删除只判子树的根、写只判最后一页（不判补建的父页）、`Refuse` 不截 `Reads`、`Refuse` 取代更早的失败，都被 `TestThePlanChecksTheContract` 抓到；契约的 bundle 的 deleteNode 去掉 `page.protected`（答出了没声明）与 handler 的那一行去掉（声明了没答出，`apitest.Main`），都被 page 的 HTTP 适配器的测试抓到；`mcpToolCodes()` 的 write 一行去掉 `page.virtual_file`（`toolError`）与不调 `wikiCodeCalls`（行里的码没有调用答出），都被 `TestTheMCPToolsErrorsCarryTheirCodes` 抓到。其余几行是测试自己的行（普通的笔记本的几行、没标出的 `page.title_taken` 照旧重新计划、`guards: []` 的程序、`TestModeComesBeforeTheLock` 的两半、handler 的表以标出的错误答出同样的码）。修复一轮在 `7834bd70` 上重跑其中的九个（`spec-s2.json`，与第 23–41 条共 36 次），都被抓到。
  - 审查的四个变异建成时都活下来（A 的 RA1、RA2 = C 的 c06、c05；C 的 c01、c02），修复一轮之后都被抓到，作 7.3 的第 81–84 条：`Refuse` 保留同一项上的失败（`Failed.Op <= at`）与不清 `final`，被 `TestThePlanChecksTheContract` 的批量的界线的三行抓到；守卫不标出它的规则的拒绝（`return v.Err`）与以这一步的第一个节点标出 `page.source_admin_only`，被 `TestTheModeGuardMarksTheNodeItRefuses` 抓到。审查者的别的变异（A 的 RA3–RA6，C 的 c03、c04、c07、c08、h1–h7）都被抓到：RA3、RA4（不带工作区问授权者）过模块的测试（替身授权者不看目标），被整个程序的测试抓到。
  - 第 1 个提交改了 `TestPageNotebooksConvertTheState`（三种读法各记下它调的读）：S1 的第 80 条在它上面再跑一次照样抓到；去掉 `SourceRootID`、`LockByID` 读 `ShareState`、`ShareByID` 读不加锁的 `StateOf`，都被抓到。
- 门禁（本机，`cfa1c608`）：`make lint gen-check knip build` 通过；经 `heavy.sh`：`make test` 900 秒（竞态与组合根三份同时跑，之后成本与守卫；`test-web` 166 个文件、3,349 个测试），`make test-go-short` 54 秒，`make e2e` 286 个 37.0 秒。修复一轮（`7834bd70`）：`make lint gen-check knip build` 通过；经 `heavy.sh`：`make test` 第一次 244 秒以 2 退出，不是我们的代码：组合根的一份起不来它的 PostgreSQL 容器（对 Docker 的套接字 `context deadline exceeded`，那时另一个会话的 `postgres:18.6` 容器正在共用的 Docker 上起，负载 8–10），那个二进制的每个测试都在准备时失败，另两份与其余的都过；再跑一次 890 秒通过；`make test-go-short` 53 秒，`make e2e` 286 个 36.9 秒。
- 组合根的各份（6.5 的规则）：`go test -race -list` 核对，426 个测试、三份 168、119、139（S2 的组合根测试之前，`ee64833c`，420 个、168、117、135），第三份四个、第二份两个照 7.5 的表，第一份没有新的。本机单独两次：第一份 271.1、252.9，平均 262.0 秒；第二份 164.6、156.0，平均 160.3；第三份 192.3、191.5，平均 191.9。S2 的起点 `45d086a1` 同样两次：第二份 148.3、155.5，平均 151.9（+8.4）；第三份 189.2、195.8，平均 192.5（−0.6，在波动之内）。CI 与本机之比 1.67、2.48、2.03。
- CI：分支的 run 40123373818（`cfa1c608`）共 12:31，**一个作业红**（作业，括号里是测试一步）：web 3:36（3:03）、short 2:32（2:13）、竞态 8:01（7:41）、组合根一 7:41（7:18）、二 6:56（6:37）、三 6:51（6:30）、守卫 12:28（12:10，失败）、成本 11:48（9:00），e2e 与镜像因此跳过。守卫的失败是 M9 的 grep 的成本守卫 `TestGrepOfAHundredThousandCandidatesTakesItsTime`："GrepCandidates of 100,000, without statistics: [195.8ms 388.3ms 393.1ms]"，中位数过了 270 毫秒；本机 `make test` 里同一守卫照过。S2 不碰 `GrepCandidates`，grep 的路径上只有 `decideNotebook` 改成包着 `decidedNotebook`（读的一样）；这个 run 整个慢：没有 S2 的测试的第一份从 S1 的合并的 run 40116706995（`45d086a1`）的 4:59 到 7:18，竞态 6:00 到 7:41，守卫 7:57 到 12:10。照说明停下，没有再推、也没有重跑；修复一轮的推送重跑了它。修复一轮的 run 40131529634（`7834bd70`）全绿，共 16:53：web 4:04（3:31）、short 3:18（2:56）、竞态 7:58（7:37）、组合根一 6:44（6:15）、二 6:04（5:46）、三 6:32（6:14）、守卫 8:03（7:45，grep 的守卫照过）、成本 12:20（9:20）、e2e 4:28（E2E 一步 3:36）、镜像 1:18。照 M9/P2 6.5 的规则的算法，两次的测试一步的平均是第一份 6:46.5、第二份 6:11.5、第三份 6:22，都在 7:30 之下（第二、三份在 7.5 到 P1 结束的估计 7:00–7:15、6:45–7:00 之下；第一份没有 S2 的测试，是离线最近的一份）；界线没有挪、没有分、没有提高 `-timeout`，CI 的文件与 `Makefile` 没改。
- 建成的 CI 的红不是 S2 的（审查 C 的诊断，主持者的决定 30）：S2 不碰 grep 的守卫跑的任何东西；本机上它在 S2 与它的起点上交替各两次量得相同（三个 `GrepCandidates` 的阶段的中位数 75–87 毫秒）；main 的守卫一份在没改的代码上从 7:57 到 12:35；修复一轮的运行过了它。它再失败时，守卫照 M9 给它的兄弟的界的改法（以量过的 CI 的中位数定界，或计时五次取中位数），不在 M10 的哪个 Step 里改：记作 M10 的收尾的一项（00 号文档第 7 节"写给后面的 M"）。
- 设计的文字照建成的与修复一轮改，已改在各节（第 0 节，2.1 第 6 条，3.17–3.24，3.28，3.29，6.2，7.1，7.3，7.5，第 8、9 节；3.31 列出）；7.3 加第 81–84 条，第 23、29、32、40 条写明照实的写法。
- **修复一轮**（2026-10-25，分支 `m10-p1-s2` 的 `7834bd70`；两份审查（主持者的决定 27；A：守卫、规则、每条写的路径、码与 `batch` 的计划；C：测试、变异、e2e、提交与 CI 的失败），Important 0、Minor 五条（不同的四条，A-M1 = C-M2 同一条）、Nit 3，主持者的决定 `fix-brief-1.md`，决定 28）：从 `cfa1c608` 起 4 个提交，一条一个，每个提交单独 build、vet、碰到的包的静态检查与 short 测试都通过：
  - A-M1 = C-M2 `a5ea0d68`：`TestThePlanChecksTheContract` 加 `Refuse` 在批量自己的界线上的三行（审查者的形状）：项数的界线（`Big` 下 `MaxUnitNodes - 1` 页，删 `Big` 再移 `raw`）与移动带着的节点（`Large` 与带着 `raw` 的 `Notes` 各 5,000 页，都移到别的父节点下）在需要 `page.manage_source` 的那一项上，`Refuse` 取代它们，答那一项的 `page.source_admin_only`；`[edit Notes（不匹配）, delete Big, move raw]` 在 `Refuse` 之后读 `Notes`、`Contents` 答第 0 项的 422（`final` 清掉了）。第 81、82 条变异。
  - C-M1 `8b38a823`：`TestTheModeGuardMarksTheNodeItRefuses`（在 `TestTheModeGuardLetsAStandardNotebookBy` 旁边）：删 `Notes`（`raw` 在它下面）答标出 `raw` 的 `page.protected`，把 `Notes` 移到 `Other` 之下（`page.manage_source` 被拒）答标出 `raw` 的 `page.source_admin_only`。第 83、84 条变异。
  - C-N1 `a5990de5`：`TestTheModeGuardReachesEveryMCPWrite` 在 `edit AGENTS.md` 之后以它答的版本 `write` 覆盖 `AGENTS.md`：写下、什么也没建，agent 的第二个变更集（3.23 的"`write` 覆盖已有的页"）。
  - A-N1 `7834bd70`：`ApplyBatch.Execute` 的次序的注释写出树的一遍判契约（`page.protected`、`page.virtual_file` 在它们的项上）与 `page.manage_source` 在读与编辑锁的预检之前问、`page.source_admin_only` 答在那一项上。
  - A-M2、A-M3（S2 与 S3 之间的另两个窗口：恢复或导入放到根下的 `index` 改名之后的撤销答没声明的 `page.virtual_file`；同步撤销的前一步碰到锁时答 `page.locked`、不答后一步的 `page.source_admin_only`，S3 的两遍预检关上，主持者的决定 29，第 0 节）与 C-N2（这一节里第 4 个提交的 sha 是修订后的 `cfa1c608`）是主持者的文档；代码不为它们改。
- **主持者的核对**（2026-10-25，代替审查者的修复核对，为了额度，主持者的决定 31）：读过 `cfa1c608..7834bd70` 的差异：4 个文件，三个测试与 `apply_batch.go` 的一处注释，没有生产的代码的改动；实施者在修好的头上重跑 7.3 的第 23–41 条与九个要改代码的反向对照（36 次），都被抓到，审查者存活的四个变异各让它的测试失败（第 81–84 条）；CI 全绿。

### 10.3 S3：预检、加序号与后台的拒绝的去处（2026-10-26，合并 `a46c3d1d`）

- **提交**：起点 `8e62b191`，预检 `22ca92af`，后台拒绝 `c3ae49f9`，B-M1 的最终报告清理 `ba5cded6`，三处加序号 `607e761e`。四个提交按依赖顺序；分支文档 `47945074`，合并时的分支头 `47945074`；于 2026-10-26 合并为 `a46c3d1d`（完整 SHA `a46c3d1d076a97e793fafaa6fb2984f02876b2b3`，00 第 12 节）。
- **预检**：上传的 `Check` 在 `assetPlace` 后读状态、经 `FactsOf` 判契约，仍在传文件和查空间前；同步撤销、`CheckRevert` 与 `PlanRevert` 共用锁外的一遍，计划和步数界线在前、锁和解析预算在后。`Unit.Revert` 用锁下的 `Write.Notebook` 与 `Grants` 在第一步前再判。`RevertFacts` 复用调度器的模拟，后一步看前一步留下的树；额外授权只在需要时读取工作区，局部备忘与单元的备忘分开（3.32 第 1–2 项）。`startRevert` 的两个码和约定的完整说明已生成进契约。
- **后台与报告**：标出的拒绝由 `revertDefect` 原样传出，经先认九个哨兵的 `treeError` 成为 transfer 的 `Refused`，先于普通 forbidden 判为 refused；导入仍清理失败批次的文件。报告的码是自由字符串，存储/详情/列表照读，普通报告的 refusal 是 null。共用 finalizer 在 timeout/interrupted/cancelled 覆盖时清掉旧 refusal，普通 refused 保留（B-M1）。网页写已知码、未知码的退路和有/无节点；TR1–TR4 四个完整报告期望增加 `refusal: null`。
- **加序号与不变量**：导入、同步恢复、后台恢复只在 LLM Wiki 实际落到根下时把四个保留键并入已占的名字；子页与普通笔记本照旧。模块测试覆盖四键、大小写、附件、兄弟冲突及后台后续子成员。整个程序导入 `index.md`、`Index/a.md`、`log/b.md`，得到两个 renamed 问题、合并后的子页，恢复再跳过已占的序号。`pageViolations` 的根下保留键规则以根页/子页/普通笔记本三种 SQL 反例核对，在最后删除笔记本之前恢复测试种子。
- **S2 的窗口**：A-M2 对应的根下非法名由加序号和 SQL 不变量覆盖；A-M3 的前一步编辑锁、后一步模式拒绝由 `TestPrechecksAnswerTheGuardsCodes` 明确核对，答 `page.source_admin_only`。开始后台任务的预检和任务计划同码，任务计划之后的权限变化由批内守卫判；`TestTheSourceRootsPutBackIsRefusedInALaterBatch` 保留第一批并报告第二批的原码与节点。这些窗口已随 `a46c3d1d` 进入 main 并通过首次 main CI，见 00 第 12 节。
- **照实的差异与审查**：3.32 记录六项。任务范围独立 A/B/C 的审查结束：A 的 Q-A1 已接受；B-M1 已在 `ba5cded6` 修复并由原 B 复核；C 无发现。任务范围的审查与 B-M1 修复复核、独立的完整 Step A/B/C 审查均已核对。完整 Step 没有经过核实的 Important、Minor、Nit 或 Q，没有剩余产品修复；主持者接受结论。受测头 `607e761e` 的 13 项本机门禁、三份各两次本机测量和分支 CI 十个作业通过；两个契约提交的精确 HEAD 已追溯通过 `gen-check`。合并与合并后的 CI 按决定 33 记录。 详细对象、命令、反例、决策在 [S3 审查记录](reviews/P1-S3-prechecks-refusals-numbering-review.md)，任务范围的审查不代替完整 Step 三方审查。
- **变异**：最终代码头 `607e761e` 上，42–53（51 分 a/b/c）和新增 85、86 共 16 个变体、18 次 catcher 都在指定的运行时断言失败；每组原样通过、字节备份恢复、恢复后通过。45 同时跑模块和最后一跳，46 同时跑纯函数和真实编辑锁；52 跑网页，其余以竞态检测跑 Go。没有把编译、安装、超时或不点名的失败算作抓到。完整 Step 审查追加 ListJobs 丢 refusal 的 87、组合根漏登记模式守卫的 88，各由已有永久测试抓到，合计 18 个逻辑变体。A 的 85/88 overlay 是额外独立确认，不重复算成新变体。逐条表、计数口径与原始证据路径在审查记录。
- **交错与实测**：新增五个 bootstrap 测试以 `-race -count=1` 一起实测，包 7.335s；后台第二批拒绝 2.22s、同码预检 1.44s、解析预算次序 1.28s、根下加序号 1.18s、纯函数 0.00s。第二份新增 1.44s，第三份 4.68s，乘 1.73 只作约 2.49s 与 8.10s 的估计核对；不代替 CI 的测试一步时间。三份各两次本机测量已通过：Go 包时间的平均分别为 304.811s、269.449s、272.732s，命令包装层的平均分别为 305.620s、270.262s、273.512s。这些是本机读数，不充作 CI 的 Test 步；三次 main CI（含失败的第二次）的 bootstrap 样本及所有两两平均已核对，按决定 33–34 转写在下一项及审查记录；最新完整运行全绿后才进入 S4。
- **门禁与 CI**：各任务提交前与 B-M1 修复的常规门禁，以及两处契约提交精确 HEAD 的追溯 `gen-check` 记录见审查记录。受测头 `607e761e` 的本机全套门禁 13/13 通过，最后的状态输出为空；Vitest 166 文件/3,356 测试，e2e 286 通过。 分支 `origin/m10-p1-s3` 的 [CI 40311912868](https://github.com/open-nerve/NerveWiki/actions/runs/40311912868) 在精确头 `607e761e` 上十个作业全部 success；组合根三份 Test 步为 360s、392s、354s，e2e 步 190s。 S3 已于 2026-10-26 合并为 `a46c3d1d`（完整 SHA `a46c3d1d076a97e793fafaa6fb2984f02876b2b3`，父提交 `8e62b191`、`47945074`）。分支文档的 [CI 40318346909](https://github.com/open-nerve/NerveWiki/actions/runs/40318346909) 在精确头 `479450745b11ddabee8e2cc386482f9204d12556` 上十个作业全部 success；组合根第一、二、三份的 Test 步为 6:10、4:19、6:26（作业 6:37、4:36、6:48），E2E 步 3:13（作业 16:00）。合并后的首次 main的 [CI 40322279159](https://github.com/open-nerve/NerveWiki/actions/runs/40322279159) 在精确头 `a46c3d1d076a97e793fafaa6fb2984f02876b2b3` 上十个作业全部 success；组合根第一、二、三份的 Test 步为 7:14、6:33、4:08（作业 7:32、6:51、4:29），E2E 步 3:50（作业 4:39）。第二次 main 的 [CI 40325348498](https://github.com/open-nerve/NerveWiki/actions/runs/40325348498) 在精确头 `ca831b58bdf5c396648daa592b64820d4d93ad0b` 的 attempt 1 失败：仅 costs 的既有 M9 IndexItems 成本守卫超过 30 ms，七个作业通过，e2e/image 跳过。组合根第一、二、三份 Test 步为 306s、393s、402s（作业 326s、412s、421s），与首次 main 的 434s、393s、248s 的平均为 370s、393s、325s；这两次的分片均值低于 7:30，但当时整套 CI 尚未全绿，主持者未据此开 S4。Actions 重跑因 token 缺少权限被拒绝，未产生 attempt 2。决定 34 的新完整 push [CI 40328760108](https://github.com/open-nerve/NerveWiki/actions/runs/40328760108) 在精确头 `f2fe618040945923c5d32fb438d78632645210fd` 上十个作业全部 success（attempt 1；新运行，不是失败运行的 attempt 2）。组合根第一、二、三份 Test 为 300s、390s、396s，作业为 320s、415s、415s；E2E 步 223s、作业 757s。三次 main 的全部 Test 样本分别为第一份 434/306/300s、第二份 393/393/390s、第三份 248/402/396s；按运行 1+2、1+3、2+3 的每对平均依次为第一份 370/367/303s、第二份 393/391.5/391.5s、第三份 325/322/399s，全部低于 450s。三次平均为 346.667/392/348.667s，保留失败运行的成功分片，没有挑最快的一对。主持者在开 S4 前核对并在 `s3-evidence/postmerge-all-samples.json` 接受入口；本次按决定 33–34 转写全部已发生的记录。失败运行 `40325348498`、Actions 重跑的权限拒绝及原样本机诊断仍保留；M10 的 IndexItems 与 grep 稳定性收尾义务未关闭。这些是 S3 的样本，不代替 S4 合并之后的两次测量。
- **后续**：S4 的结果见 10.4，P1 的完成核对见 10.5；M5、M8、M9 移交的状态见各自处理进展。来源、去重、M11 与 M10 收尾由后续阶段处理。

### 10.4 S4：网页（一）（2026-10-27，合并 `f261952f`，见 00 号文档第 12 节）

- **提交**：从 main `f2fe618040945923c5d32fb438d78632645210fd` 开始，依次为树与 LW2 `ab3bd0eb`、创建与 LW1 `35804c04`、降级 store/设置 `d83cd89a`、真实模式事件 `8e4c1899`、LW12 `fa662978`、主持者 R-M1 夹具修复 `a0aa88dd`。完整 Step 首审候选为 `a0aa88dd647e7579022b6c0e5b54ed22c8a0c8ff`；第七个提交 `13ee0bc8` 只补 B-I1 拒绝重读交错的六个参数化测试实例，最终受测头 `13ee0bc8ab4a302d654aa58ad994cbc8cedd6c24`，仍为23个代码/测试文件；分支文档提交 `8556ad07576485e5ed6f902eb6137067ac324a4c`，于 2026-10-27 合并为 `f261952f757bb486908210955f3b679a407b71e7`，父提交 `f2fe6180`、`8556ad07`；合并树与分支相同。
- **网页**：创建的原生模式单选两种模式都带用户偏好的模板语言，新手引导请求不变；树按规范化根下 AGENTS 和来源根 id 限制菜单/拖动，保留放入目标，降级后的同一 DOM 行重建拖动登记；链接的可读名字保留种类后缀与真实空格。降级按笔记本排队，答复覆盖列表并使旧读失效；指定三个拒绝重读，busy/网络或无码错误不重读。持有的对话框不会随管理员按钮消失，成功或无按钮时焦点到 Mode。
- **最后一跳与故事**：实际表登记 `mode`，只重读工作区笔记本列表；替身事件为 `later`。LW1 的两种页面语言、LW2 页面和 LW12 API/页面共五个 S4 故事，使 P1 共九个故事。LW12 在唯一工人被 B 占住时证明 A 的持久队列与 busy；成功降级帧、standard/null 与树删除均核对。独立浏览器上下文在既有流上更新，不靠页面重载。后续来源去重、虚拟两行与 MCP 系统说明仍属 P2/P4/P5。
- **偏离与修复**：3.33 记录测试观察/组织安排。R-M1 为 fixture 的 llm_wiki/null 不忠实，`a0aa88dd` 单行修复；20 项创建/导航与 lint-web 通过并由主持者复查，没有产品行为改动或新增决定。完整 Step 的首次独立审查对象均为 `a0aa88dd`：A/C 没有核实出的发现；B-I1 是 Important 的测试覆盖缺口，生产 reconciliation 的 put/changed 本来正确。 `13ee0bc8` 只在 `notebook.store.test.ts` 新增51行、六个参数化测试实例，生产代码原本正确且未改。原审查者B在此精确头以同一命令重放原B-X3：59通过（3.327s）→恰六个新实例断言失败、原53个通过（3.375s）→原字节恢复后59通过（3.369s）；全部是已提交的永久测试，没有scratch实例或重试，最终树干净。主持者读完整报告与原始证明后关闭B-I1；`s4-full-review-b-fix1.md`、`s4-review-evidence/b/fix1-prepared/proof.json` 与 `s4-root-final-review-acceptance.json` 保存处置。 三份原始完整 Step 独立审查及修复后的范围核对已由主持者接受；B-I1（Important，测试覆盖）已关闭，没有剩余核实的 Important、Minor、Nit 或 Q。原始审查头为a0，最终接受头为13ee；B由原审查者重放核对，A/C由主持者按工作流3.4的测试修复例外核对未变范围，记录分别保存。
- **反向对照**：54–64（57a/b、63a/b）在原始候选 `a0aa88dd` 的完整重放有 13 个逻辑变体、20 次指定永久 catcher，均 pristine 0 → 运行时失败 1 → restored 0、原字节恢复；63a/b 各有 LW1 英文、中文和 LW2 三条。先前一次未改代码的 63 基线遇到 `net::ERR_NETWORK_CHANGED`，在初始模块装载时失败，不算变异；保留原证据，经诊断只做一次原样完整重放，最终证据另列。审查新增两个永久逻辑变体：89（A-X1，根下 AGENTS 限制，由已有测试抓到）与90（B-X3，拒绝 reconciliation 不计 changed，由新增六个参数化测试实例核对）。54–64原编号保持；最终15逻辑变体/27次指定 catcher 的重放结果另记。 精确修复头 `13ee0bc8ab4a302d654aa58ad994cbc8cedd6c24` 已重新执行54–64（57a/b、63a/b）与89、90，共15个逻辑变体、27次指定永久catcher；逐项为原样0→运行断言失败1→恢复0，全部受跟踪文件恢复原字节。第89条由已有根下AGENTS测试抓到，第90条的六个永久实例都失败后恢复通过。原13条证明保留在 `s4-final-mutations/final-13ee0bc8/planned/`，89/90各有独立子proof，15个逻辑变体/27次catcher的完整汇总为 `s4-final-mutations/final-13ee0bc8/proof.json`；原a0重放与审查者外部探针不冒充这次重跑。
- **门禁与 CI**：精确 `a0aa88dd` 的原始候选的本机全 Step 13 项门禁全部通过（含三份组合根各两次）：Vitest 166 文件/3,409 测试，E2E 291 测试；命令时间合计 3127.672s，最终树干净。 原始候选 `a0aa88dd647e7579022b6c0e5b54ed22c8a0c8ff` 的 [分支 CI 40354593712](https://github.com/open-nerve/NerveWiki/actions/runs/40354593712) 十个作业全部成功；bootstrap 1/2/3 的 Test 步为378/370/322s（作业402/396/343s），E2E步232s（作业303s）。`s4-evidence/branch-ci-a0aa88dd/{run,jobs,timings}.json` 保存原始记录。这是修复前的实际历史，不能代替 `13ee0bc8` 的新分支CI或合并后的两个main样本。 修复候选 `13ee0bc8` 的13项门禁范围已覆盖：本头新跑 static-build、web、E2E 三项（合计246.659s），网页166文件/3,415测试、E2E291测试、22条已知TimeoutNaNWarning；十项Go命令保留实际受测头 `a0aa88dd` 的通过记录，以唯一改动为51行前端测试、增加六个参数化实例的完整输入等价证明复用，未声称重跑。 最终修复候选 `13ee0bc8ab4a302d654aa58ad994cbc8cedd6c24` 的 [分支 CI 40362256209](https://github.com/open-nerve/NerveWiki/actions/runs/40362256209) 十个作业全部成功；bootstrap 1/2/3 的Test步为370/391/360s（作业392/415/383s），E2E步190s（作业247s）。其他Test步/作业为web214/250s、short182/201s、race624/653s、costs532/714s、guards770/799s；Image smoke为73s（作业81s）。原始 `s4-evidence/branch-ci-13ee0bc8/{run,jobs,timings}.json` 的run、head、branch、push事件、workflow、十个job与计时逐项核对，B另独立核对同一CI。该分支运行不能代替未来合并后的两个main样本。 原始候选 `a0aa88dd` 的本机各份两次实测如下；修复头只在完整输入等价核对后复用这些读数，不声称新跑了一次Go门禁；Go 包与命令包装层分别记，不充作 GitHub CI 的 Test 步。S4 没有新增 bootstrap 测试，不能拿 S3 新测试的 7.335s 说成 S4 新测。

| 组合根 | Go 包两次（s） | 包平均（s） | 命令两次（s） | 命令平均（s） |
|---|---|---:|---|---:|
| test-go-bootstrap-1 | 300.355 / 307.297 | 303.826 | 306.375 / 308.088 | 307.2315 |
| test-go-bootstrap-2 | 274.975 / 273.525 | 274.25 | 275.769 / 274.306 | 275.0375 |
| test-go-bootstrap-3 | 272.018 / 268.567 | 270.2925 | 272.838 / 269.363 | 271.1005 |

每提交检查、输入等价复用、警告与排除的开发过程失败见 [S4 审查记录](reviews/P1-S4-web-review.md)，不以任务门禁代替完整 Step 门禁。S3 的所有样本已在 10.3 转写。

- **合并与 main CI**：S4 于 2026-10-27 合并为 `f261952f757bb486908210955f3b679a407b71e7`，第一父提交 `f2fe618040945923c5d32fb438d78632645210fd`，第二父提交为分支文档 `8556ad07576485e5ed6f902eb6137067ac324a4c`；合并树与该分支头完全相同。分支文档的 [CI 40367549675](https://github.com/open-nerve/NerveWiki/actions/runs/40367549675) 在精确头 `8556ad07576485e5ed6f902eb6137067ac324a4c` 上十个作业全部成功；bootstrap 1/2/3 的 Test 步为 393/384/247s（作业 415/405/269s），E2E 步 223s（作业 281s）。合并后的首次main的 [CI 40370315130](https://github.com/open-nerve/NerveWiki/actions/runs/40370315130) 在精确头 `f261952f757bb486908210955f3b679a407b71e7` 上十个作业全部成功；bootstrap 1/2/3 的 Test 步为 291/393/397s（作业 309/413/416s），E2E 步 229s（作业 280s）。合并记录文档 `767b69a6aeae4ada095de137bf0996083fae193e` 触发的第二次 main [CI 40374066614](https://github.com/open-nerve/NerveWiki/actions/runs/40374066614)（attempt 1）十个作业全部成功；bootstrap 1/2/3 的 Test 步为 420/387/334s（作业448/431/361s），E2E 步 192s（作业 738s）。 两次 bootstrap Test 步平均依次为 355.5/390/365.5s，三份均严格低于 450s（7:30）。 两次都是精确 main 头的正常 push CI（attempt 1/1），各十个作业全部成功；主持者核对原始 run/jobs/timings 与 postmerge-pair.json 后接受；完整作业时间及原始证据存于树外的 S4 postmerge 记录。 S3 的样本不充作 S4 样本。

| main 样本 | run / 精确 HEAD | bootstrap-1 Test / job | bootstrap-2 Test / job | bootstrap-3 Test / job | E2E step / job |
|---|---|---|---|---|---|
| 第一次 | `40370315130` / `f261952f757bb486908210955f3b679a407b71e7` | 291s / 309s | 393s / 413s | 397s / 416s | 229s / 280s |
| 第二次 | `40374066614` / `767b69a6aeae4ada095de137bf0996083fae193e` | 420s / 448s | 387s / 431s | 334s / 361s | 192s / 738s |
| 两次平均 | 仅比较 bootstrap 的 Test 步 | 355.5s | 390s | 365.5s | E2E 不设此平均门槛 |

- **移交与状态**：P1 已完成，核对见 10.5；M10 继续进行。M5 模式事件移交已改为 done，M8/M9 保持 open；给 M11 的[模式契约移交](../M11-upgrade/handoffs/M10-mode.md)已创建并保持 open，不开展 M11。IndexItems 与 grep 的稳定性义务留给 M10 收尾。

### 10.5 P1 完成（2026-10-27）

P1 的四个 Step 已合并：S1 `45d086a1`、S2 `bad859c0`、S3 `a46c3d1d`、S4 `f261952f`。第 8 节的完成条件已核对；各 Step 的实施修正、审查与核对、门禁及变异证据在 10.1–10.4 和四份审查记录里。S4 最终代码为 `13ee0bc8`，B-I1 的六个永久测试补齐后三方审查的处置与核对见 [S4 审查](reviews/P1-S4-web-review.md)，没有未处理的 Important 或 Minor。

S4 的实际合并为 `f261952f757bb486908210955f3b679a407b71e7`（2026-10-27；父提交 `f2fe618040945923c5d32fb438d78632645210fd`、`8556ad07576485e5ed6f902eb6137067ac324a4c`），合并记录文档为 `767b69a6aeae4ada095de137bf0996083fae193e`。合并后的完整 CI 样本如下；Test 步与作业时间分开：

| main 样本 | run / 精确 HEAD | bootstrap-1 Test / job | bootstrap-2 Test / job | bootstrap-3 Test / job | E2E step / job |
|---|---|---|---|---|---|
| 第一次 | `40370315130` / `f261952f757bb486908210955f3b679a407b71e7` | 291s / 309s | 393s / 413s | 397s / 416s | 229s / 280s |
| 第二次 | `40374066614` / `767b69a6aeae4ada095de137bf0996083fae193e` | 420s / 448s | 387s / 431s | 334s / 361s | 192s / 738s |
| 两次平均 | 仅比较 bootstrap 的 Test 步 | 355.5s | 390s | 365.5s | E2E 不设此平均门槛 |

两次 bootstrap Test 步平均依次为 355.5/390/365.5s，三份均严格低于 450s（7:30）。 两次都是精确 main 头的正常 push CI（attempt 1/1），各十个作业全部成功；主持者核对原始 run/jobs/timings 与 postmerge-pair.json 后接受；完整作业时间及原始证据存于树外的 S4 postmerge 记录。 S3 的三次 main 运行（含失败 `40325348498`）与所有样本的处置已在 10.3 和 S3 审查转写，没有用 S4 的分支运行代替合并后样本。

- [M5 模式事件移交](handoffs/M5-events.md) 已关闭：服务端的 `mode`/`EventMode` 与网页实际处理表的最后一跳均完成。
- 新建给 M11 的[模式契约移交](../M11-upgrade/handoffs/M10-mode.md)，状态为 open，记录模板契约、写行后同事务的订阅者、原子模式/来源根写入、普通模式可预先指定来源根，以及授权备忘与自由字符串拒绝码。这里只移交，不开始 M11。
- M8/M9 移交保持 open，其余来源规则、去重、健康度、虚拟文件与 playbook 按第 9 节交给 P2–P5；P2 尚未开始。M10 仍进行中，IndexItems 与 grep 的稳定性核对留在 M10 收尾。

## 附录：核对过的代码

每个决定在 `d7bb5e8d` 上核对过的地方（读法：模块在 `server/internal/modules/` 之下，`W/` 是 `web/apps/web/src/`）。

| 决定（节） | 核对过的 |
|---|---|
| 迁移的写法与测试（3.2） | `00009_notebook_notebooks.sql` 全文；`00028_linking_page_links_resolved_asset.sql:6-13`；`00034_page_changesets_history.sql:12-17`；`server/migrations/schema_test.go:66-106`、`:163-440`（`notebooks` 的项 `:305-314`）、`:465-468`、`:528-775`（`:582-587`、`:731-740`）；`changesets_moves_test.go:135-140`；`background_jobs_test.go:14-26`；`server/sqlc.yaml:55-58`；`archtest/sqlc_test.go:206-277`；`background_jobs_test.go:15-26`（`downTo` 撤到参数的那一版为止） |
| 笔记本的读（3.3） | `notebook/domain/notebook.go:14-23`、`:97-104`；`notebook/domain/errors.go:10-13`；`notebooks.sql:1-55`；`cascade.sql:37-44`；`ownerless.sql:1-9`；`store.go:34-36`、`:71-100`、`:173-182`；`notebook/notebooks.go:20-85`；`notebook/notebooks_test.go:21`、`:122-131`；`transfer/app/ports.go:30-40`；`linking/app/reindex.go:36-43`、`:87`；`store.go:67`、`:76`、`:126-131`（`ListNotebooks` 手写的 `Notebook`）；`ownerless.go:20-23`；`cascade.go:57` |
| 建的事务（3.4） | `notebook/app/create_notebook.go:42-89`；`notebook/app/manage.go:26-41`；`platform/postgres/tx.go:36-44`、`:117-144`；`page/app/unit.go:167-176`、`:185-251`；`notebook/facts.go:29-31`；`bootstrap/wire.go:144-147`；`events/app/open_stream.go:30-67`、`stream.go:79-96`、`:120-127`；`platform/httpserver/apierrors.go:200-275`（`FaultUnknown` 记 error、`ProblemOf` 经 `errors.As`、取消与期限） |
| 两个扩展点（3.5） | `notebook/app/extension.go:17-43`、`:80-97`；`notebook/module.go:52-59`、`:88-91`；`notebook/tools_test.go:21-33`；`notebook/extension_test.go:36-44`、`:200-246`；`notebook/activity_test.go:84`；`visibility_test.go:110`；`app/visibility_test.go:146`；`notebook/extension_test.go:48-62`（`adminsAuthorizer`）、`:200-246`；`notebook/app/write_test.go:36-112`；`notebook/app/visibility_test.go:140-175` |
| `Seed`（3.6） | `page/app/unit.go:186-251`、`:292-294`、`:299-330`；`page/app/unit_create.go:16-25`、`:31-68`；`page/app/extension.go:162-166`；`page/app/ports.go:737-740`、`:763-766`；`page/app/content.go:56-59`；`page/app/import_writes.go:40-44`；`page/tree_writes.go:12-16`、`:70-121`、`:143-146`、`:155-163`（`steppingClock`）；`page/module.go:256-264`、`:324` |
| 客户端（3.7） | `shared/actor.go:15-53`；`shared/client_name.go:1-46`；`page/domain/client.go:16-17`；`transfer/domain/job.go:53-54`；`page/adapter/http/handler.go:351-362`（与 `:170`、`:205`、`:231`、`:244`、`:258`、`:271`、`:311`、`:323` 的调用，`history.go:89`、`trash.go:57`、`versions.go:75`）；`asset/adapter/http/upload.go:155-171`；`transfer/adapter/http/server.go:103-113`；`archtest/rules_test.go:40`、`:238-244`；`page/adapter/http/handler_test.go:29-40`、`:257-283`；`notebook/adapter/http/handler_test.go:26-33` |
| wiki 与组合根（3.8） | `archtest/composition_test.go:31-117`；`archtest/rules_test.go:155-214`；`identity/domain/password.go:22`；`workspace/domain/reserved.go:13`；`bootstrap/wire.go:136-211`；`bootstrap/deps.go:100-122`、`:141-173`；`bootstrap/registrants.go:34-38`、`:76-78`、`:170-264`；`bootstrap/actions_test.go:20-26`；`page/domain/instructions.go:11`、`:35-36` |
| 模板（3.9） | `tools/md-fixtures/cases/003-code-excluded.md`、`014-tags-basic.md`、`019-frontmatter-not-at-start.md`；`page/adapter/postgres/queries/nodes.sql:84-95` |
| 降级（3.10） | `notebook/app/update_notebook.go`（形状）；`transfer/adapter/postgres/queries/jobs.sql:20-35`；`00039_transfer_background_jobs.sql:26-27`；`transfer/app/start_import.go:82-86`、`:104`、`:112-116`、`:132-140`、`:248-280`；`transfer/app/start_background.go:125-136`、`:158-178`；`transfer/app/start.go:103-113`；`transfer/lifecycle.go:14-26`；`00011_notebook_notebook_audit_events.sql:15-16`；`access/domain/rules.go:60-110` |
| 事件（3.11） | `events/domain/event.go:22-47`；`events/domain/payload.go:10-63`；`events/domain/frame.go:38-57`；`events/app/publisher.go:27-33`；`events/app/hub.go:65-85`；`events/app/publisher_test.go:199-219`；`hub_test.go:103-137`；`bootstrap/linking.go:369-389`；`bootstrap/events_registrants.go:11-55`；`bootstrap/links_test.go:98-131`；`bootstrap/events_stream_test.go:17-173`；`api/modules/events.yaml:21-82`、`:157-215`；`bootstrap/events_pages_test.go:67-76`（`quiet`）；`events_stream_test.go:112-132`（`settle`） |
| page 读模式（3.12） | `page/app/ports.go:54-63`；`page/app/open_notebook.go:25-78`；`page/app/gate.go:17-36`；`page/adapter/mcp/open_notebook.go:18-90`；`notebook/adapter/mcp/notebooks.go:28-75`；`notebook/app/list_account_notebooks.go:33-44`；`page/app/fakes_test.go:112-124`；`page/extension_test.go:67-82`；`page/adapter/postgres/paths_property_test.go:28-33`；`bootstrap/mcp_account_test.go:19-54`；`bootstrap/mcp_instructions_test.go:188`；`bootstrap/permission_matrix_mcp_notebooks_test.go:55-110` |
| 契约（3.13） | `api/modules/notebook.yaml:11-125`、`:229-249`、`:278-299`、`:400-478`；`api/openapi.yaml:17`、`:78-98`；`platform/httpserver/bodyshape/bodyshape.go:9-10`、`:65`、`:146-157`；`apitest/operations.go:185-300`；`apitest/rules_test.go:93-128`、`:206-278`；`api/mcp/tools.json:913-947`；`bootstrap/mcp_tools_test.go:53-160` |
| `checkPages` 与矩阵（3.14） | `bootstrap/interleavings_page_test.go:44-111`；`bootstrap/jobs_test.go:34`；`bootstrap/permission_matrix_test.go:93-177`、`:216-317`；`permission_matrix_coverage_test.go:21-160`；`permission_matrix_notebook_test.go:126-178`；`permission_matrix_page_test.go:559-639`；`permission_matrix_seeded_test.go:68-111`、`:268-282`、`:326-363`、`:474-525`、`:576-750`；`permission_matrix_mcp_test.go:27-48`、`:84-108`、`:382-433` |
| 交错（3.15、7.2） | `bootstrap/interleavings_notebook_test.go:29-130`；`interleavings_workspace_test.go:35-200`；`interleavings_trash_test.go:20-40`；`interleavings_import_test.go:170-204`、`:280-289`；`background_test.go:57-66`、`:158-275`；`interleavings_background_stops_test.go:282-317`、`:386-436` |
| 守卫（3.18–3.20） | `page/app/edit_lock.go:20-85`；`page/edit_lock.go:13-22`；`page/app/extension.go:27-55`、`:141-143`、`:190-204`；`page/app/unit.go:197`、`:213-232`、`:303-307`；`page/app/authorize.go:14-43`；`shared/authorize.go:54-62`；`shared/error.go:111-215`；`page/app/unit_test.go:266-302`；`page/app/session_test.go:224-277`；`bootstrap/trash_test.go:453`、`:472` |
| `CheckContract`（3.21） | `page/domain/change.go:9-88`；`page/domain/node.go:14-67`；`page/domain/order.go:1-40`；`page/app/unit_place.go:40-63`、`:120-153`；`unit_create.go:36`、`:51`、`:75`；`unit_create_asset.go:26-81`；`unit_move.go:21-88`；`unit_relocate.go:37-75`；`unit_rename.go:17-57`；`unit_delete.go:15-62`；`unit_revert.go:69-180`；`unit_restore.go:49-142`、`:177-210`；`unit_import.go:76-194`；`page/adapter/postgres/store.go:27-28`；`page/domain/name.go:15-29`、`:52-119`；`page/domain/trash.go:155-165`；`shared/title_key.go:15-17`；`unit_move.go:15-88`（被移动的节点在步的第一个，排序也列出整棵子树） |
| 标出与码（3.22） | `page/domain/errors.go:13-48`、`:124-151`、`:184-198`；`page/app/apply_batch.go:121-193`、`:261-312`、`:513-600`、`:602-803`；`page/app/path_writes.go:23-42`；`page/adapter/mcp/text.go:22-43`、`write.go:64-75`、`move.go:46-57`、`batch.go:165-199`、`locks.go:47-89`；`platform/httpserver/apierrors.go:256-273`；`mcpserver/errors.go:92-129`；`apitest/problems.go:79-228`；`page/adapter/http/handler_test.go:339-397`；`asset/adapter/http/upload_test.go:344-352`；`api/modules/page.yaml:59`、`:152`、`:199`、`:442`、`:486`、`:578`、`:611`、`:647`、`:767`、`:957`；`api/modules/asset.yaml:45`；`bootstrap/mcp_client_test.go:304-350`；`bootstrap/mcp_reads_test.go:426-473`；`bootstrap/mcp_write_codes_test.go:45-191`；`W/app/problem-messages.ts:11-121`、`problem-messages.test.ts:16-62`；`page/domain/batch_plan.go:179`、`:300-321`、`:700-778`；`page/domain/batch_contents.go:35-58`；`page/app/apply_batch.go:225-300`、`:370-400`；`page/app/authorize.go:14-22`；`page/app/write_test.go:309-311` |
| 没有可拒的路径（3.23） | `page/app/put_page_content.go:64`；`toggle_task.go:86`；`restore_version.go:65`；`edit_page.go:177`；`unit_content.go:48-98`；`open_edit_session.go:42`；`unit_session.go:25-59`；`restore_node.go:65-129`；`write_path.go:105-170` |
| 预检（3.25） | `page/domain/batch_plan.go:301-346`；`page/app/asset_writes.go:38-78`；`asset/app/upload.go:66-127`；`page/app/revert_changeset.go:79-249`；`page/app/jobs.go:148-173`、`:192-239`、`:300-330`、`:410-414`、`:565-730`；`page/domain/revert_part.go:22-100`；`page/app/unit_revert.go:438-546`；`transfer/app/start_background.go:151`；`api/modules/transfer.yaml:255-267`；`transfer/adapter/http/start_background_test.go:105`；`page/app/revert_changeset.go:94-110`、`:230-245`；`page/app/revert_test.go:140-158`；`bootstrap/mcp_write_codes_test.go:155-190`（`busyWrites`）；`bootstrap/page_content_test.go:363`（`whileAWriteWaitsForItsLock`）；`bootstrap/assets_test.go:215-230` |
| 后台的去处（3.26） | `page/app/unit_revert.go:171-186`、`:910-919`；`bootstrap/transfer.go:84-247`；`transfer/app/import_ports.go:124-180`；`transfer/module.go:107-121`；`transfer/app/batches.go:20-96`；`transfer/app/background.go:101-271`；`transfer/app/import_write.go:165-342`；`transfer/app/job_run.go:36-75`、`:205-239`；`transfer/domain/report.go:11-188`；`transfer/adapter/postgres/store.go:347-445`；`jobs.sql:53-65`；`transfer/adapter/http/server.go:163-185`；`api/modules/transfer.yaml:668-1014`；`transfer/app/background_test.go:224-290`；`import_test.go:715-742`；`page/app/jobs_test.go:990-1050`；`bootstrap/background_test.go:190-211`（`behindFirstBatch`）；`bootstrap/interleavings_background_test.go:345-378`；`bootstrap/interleavings_workspace_test.go:60-90`（`newAcmeTeamWired`、`wiring`）；`bootstrap/wire.go:74`（`wiring.page`）；`api/modules/notebook.yaml:182-186` |
| 加序号（3.27） | `page/app/unit_import.go:150-212`；`unit_restore.go:73-83`、`:171-210`、`:252-423`；`page/app/restore_node.go:12-19`、`:125-128`；`api/modules/page.yaml:745-747`、`:1660-1683`；`transfer/domain/import_plan.go:184`；`page/domain/revert_schedule.go:225-359`、`:439-521`；`page/domain/revert.go:268-297` |
| 网页（第 4 节） | `W/stores/notebook.store.ts:40-188`；`notebook.store.test.ts:7-241`；`W/lib/one-at-a-time.ts:9-33`；`W/test/fakes.ts:91-226`；`W/test/notebook-server.ts:57-179`；`W/test/jobs-server.ts:14-36`；`W/pages/workspace/create-notebook-dialog.tsx:39-131`；`notebook-nav.test.tsx:142-192`；`W/pages/notebook/access-options.tsx:8-55`；`W/onboarding/notebook-step.tsx:56-90`；`W/i18n/locale.ts:1-29`；`W/stores/preferences.store.ts:47-86`；`W/pages/notebook/general-page.tsx:22-150`；`general-page.test.tsx:138-194`；`W/app/confirm-dialog.tsx:1-140`；`W/app/held-dialog.ts:7`；`W/pages/notebook/members-page.tsx:36`、`:105-107`；`W/events/handlers.ts:9-29`、`:130-148`、`:249-282`；`W/app/event-stream.tsx:15-92`；`event-stream.test.tsx:44`、`:204-218`；`W/services/event.service.ts:1-5`；`W/pages/notebook/transfer-report.tsx:23-310`；`W/pages/notebook/page-tree.tsx:43-154`；`page-tree-item.tsx:8`、`:53-306`；`W/lib/title-key.ts:11`；`W/pages/search/search-hit.tsx:89-100`；`W/pages/page/attachment-row.tsx:150`；`W/i18n/messages/en.ts:68-80`、`:535-551`、`:751-752`、`:955-956`；`zh-CN.ts:67-79`、`:519-535`、`:725`、`:915`；`.github/workflows/ci.yml:126-214`；`Makefile:159-168`；`node_modules/knip/dist/constants.js:207-225`；`W/pages/page/attachment-row.tsx:143-152`（`NewTab`）；`W/i18n/messages/zh-CN.ts:421`、`en.ts:434`；`W/app/problem-messages.ts:110-118`；`W/test/page-server.ts:290-300`；`W/pages/notebook/page-tree-item.tsx:163-198`；`page-tree-writes.test.tsx:600-616`；`transfer-background.test.tsx:385-412`；`@atlaskit/pragmatic-drag-and-drop` 的 `element-adapter.js:123-128`、`:279-282`（审查 C 核对） |
| e2e（第 5 节） | `e2e/fixtures/notebooks.ts:9-61`；`assert/notebook.ts:15-145`；`assert/page.ts:16-316`；`history.ts:19-34`；`events.ts:15-21`、`:65-113`、`:205-248`；`transfer.ts:42-238`；`notebook-pages.ts:37-80`；`wiki-pages.ts:19-60`；`mcp.ts:86-222`；`test.ts:112-284`；`server.ts:36-152`；`e2e/playwright.config.ts:10-20`；`stories/mcp/mc1-connect.spec.ts:74-110`；`stories/transfer/tr1-export.spec.ts:66-80`；`tr2-import.spec.ts:104-129`；`stories/links/l1-links.spec.ts:233-262`；`server/configs/config.yaml:118-121`；`stories/transfer/tr3-roundtrip.spec.ts:52-70`、`:90-150`；`tr4-malicious.spec.ts:75-93`、`:120-242`；`stories/history/hi5-revert-import.spec.ts:60-120`；`stories/trash/tb2-background-restore.spec.ts:150`；`stories/asset/as3-render.spec.ts:322-350`；`fixtures/assert/transfer.ts:60-80`、`:210-235` |
| CI（7.5） | `Makefile:179-298`（`:204-205`、`:217-237`）；`.github/workflows/ci.yml:33-62`、`:96`；`docs/v0.1/M9-mcp/00-M9-design.md:682`、`:791`；`reviews/M9-closeout-review.md:256`；`reviews/M9-codex-review.md:39-41`、`:268-290`；`Makefile:170-220`（`:179` 的 `-race`、`:207` 的 1.73） |
| 00 号文档与总体设计的行（2.1、第 8 节） | `00-M10-design.md:7`、`:18`、`:57`、`:120`、`:124`、`:143`、`:161`、`:180-191`、`:205`、`:531-537`、`:538-546`、`:558`、`:566-581`、`:654`、`:805-824`；`v0.1-design.md:1031-1036`、`:1112`；`docs/README.md:92-104`；`docs/v0.1/M9-mcp/00-M9-design.md:815-842`；`M11-upgrade/handoffs/`、`M12-release/handoffs/M5-polish.md:217-234` |
