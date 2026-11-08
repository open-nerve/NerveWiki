# M11 升级流程：总设计

| 项 | 内容 |
|---|---|
| 里程碑 | M11 升级流程（`M11-upgrade`） |
| 日期 | 2026-11-08 |
| 状态 | 设计中（第一轮设计审查之后的修订，待复核；[审查记录](reviews/M11-design-review.md)） |
| 依赖 | M10（模式的契约、守卫的框架、lint 的显式范围、模式变化的订阅者、来源的哈希）、M8（撤销的计划、后台任务与分批）、M9（MCP 的注册表、`batch`、多轮的调度）、M5（事件流）、M7（任务的运行）；总体设计 12.3 只写了 M10 与 M8，第 7 节"总体设计的修订"照此补全 |
| 上级文档 | [v0.1 总体设计](../v0.1-design.md) 第 1.3、3.3、3.8、3.10、5.4–5.9、6.1、6.3、7.2、8.2–8.5、9.2、10、10.1、12.1–12.6、13、14 节 |
| 代码位置的写法 | 同 [M10 总设计](../M10-llm-wiki/00-M10-design.md)：模块写作 `page/app/unit.go:357`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…`、`archtest/…` 在 `server/internal/` 之下；迁移写作 `00048_notebook_notebooks_mode.sql:9`（`server/migrations/sql/`）；契约写 `api/…`；前端写作 `web/…/pages/notebook/general-page.tsx:129`（`web/apps/web/src/` 之下）；e2e 写作 `e2e/playwright.config.ts:3`。行号都是 `main` 的 `dbce5828` 的（代码与 `7ff74a67` 相同，其后只改了文档），由设计者对着它逐条核对过；M10 文档与移交里的行号已有漂移，这里不照抄 |

---

## 1. 目标

1. **发起**：笔记本管理员在网页的设置"模式"一节把一本普通笔记本升级为 LLM Wiki。系统在一个事务里记下升级会话、冻结笔记本、记下基线（升级开始时每个活着的节点），并补写缺的两页：根下没有 `AGENTS` 就照模板建，根下有标题键为 `raw` 的页就用它作提议的来源根，否则照模板建 `raw`（4.4）。
2. **冻结**：升级期间整本笔记本的内容只读：只有发起人（仍是这本笔记本的管理员时）经 MCP 发出的写被放行；审阅中对所有人冻结；回滚中只有回滚自己的写。笔记本的设置、成员、导出与删除照常（4.5）。冻结不设时限（负责人 2026-11-08 第 1 条）。
3. **护栏**：发起人的 agent 写的时候，升级开始时已有的页的正文（frontmatter 之后的字节）不能变，已有的节点不能删除，移动与改名必须让系统改写链接；改名、移动、新建本身不受限；系统的链接改写照常放行（4.6）。
4. **闸门与报告**：检查（REST 的 `checkUpgrade` 与 MCP 的 `upgrade_verify`）在一个快照里按 LLM Wiki 的契约与提议的来源根算出七道闸门、索引是否过时与各条提示，答完整的报告；提交（REST 的 `submitUpgrade` 与 MCP 的 `upgrade_submit`）先进审阅、再在快照里另算一次，全部通过才留在审阅中并存下报告（4.7、4.8）。网页与 agent 走同一套闸门（负责人第 2 条）。
5. **人审**：任何笔记本管理员（发起人也可以）查看存下的报告，批准、驳回或退回继续修改（负责人第 4 条）。批准在一个事务里把笔记本改为 LLM Wiki、定下来源根、重建来源区的哈希、经订阅链发 `mode`（4.9）。批准之后不提供整体撤销，退回普通笔记本用 M10 的降级（负责人第 3 条）。
6. **回滚**：放弃与驳回按基线整体复原：删掉升级中新建的、放回、改回名、写回基线版本的正文；在后台分批，停下之后可以继续，继续就是重新算一次差；做不完时管理员可以结束回滚、保留现在的样子（4.10，主持者第 212 条）。
7. **playbook**：英文的 upgrade prompt，只在笔记本入口、只在升级中给出（4.13）。
8. **网页**：笔记本的常驻横幅、只读收成一处判断、编辑器失去编辑的新原因、发起对话框、升级页（状态、怎样接入 agent、检查与提交、审阅、放弃、回滚的进度与结束）（4.15）。网页不显示 MCP 地址（第 184 条）。

## 2. 范围

**做**：

- notebook：迁移 00055 给 `notebooks` 加升级的四列与检查；`State` 与 `View` 带上升级（四条查询多选这四列，`ListNotebooks` 在内）；`Notebook` 加必填、可为 `null` 的 `upgrade`；模块根导出在调用方事务里执行的端口 `notebook.NewUpgradeStates(pool, subscribers)`（锁、各个写、`Publish` 分开，4.1）；模块根导出 `ActionChangeMode` 给组合根。
- page：迁移 00056 建 `upgrade_nodes`（基线、逐页的待建数、账本、保护行）并给 `changesets` 加 `upgrade_id`；迁移 00058 给变更集加类型 `rollback`；`NotebookState` 带上升级；`Write.UpgradeOwn`；冻结的守卫与否决者 `page.NewFreezeGuard`，登记在模式与锁之前；预检的冻结；护栏（删除、不改写链接的移动、正文的切分与逐步的比较、`batch` 的折进）与账本的维护；基线的读写、发起时的编辑会话与根下的键的读、闸门第 1、2、7 条的读；`Seed` 带升级的标记；回滚（先删新建的子树，再计划、分批）；来源哈希的重建 `page.NewSourceRebuild`；`CheckContract` 对撤销里的恢复也判保留标题（第 181 条）。
- wiki：迁移 00057 建 wiki 的第一张表 `notebook_upgrades`（`server/sqlc.yaml`、`deploy/runtime-grants.sql`、清理器、笔记本删除的订阅者都从零开始加）；升级的编排与十个 REST 操作、两个 MCP 工具、一个 prompt；lint 的计算拆出"在 ctx 带的事务里算"、`Runs` 加只占名额的入口；闸门、报告与它的存储；回滚结束的端口 `wiki.NewRollbackEnd`。
- linking：lint 的 `wanted` 加一个按发出页的数（闸门 6 逐页比较用）；工具说明的措辞。
- transfer：迁移 00059 给任务加种类 `rollback` 与 `upgrade_id`；后台运行认它（先删、计划、分批、按遍重算、结束时在同一事务里调 wiki）；开任务时判冻结（导入的建任务与 `startBackground` 里，不在共用的锁里）；回滚的任务只有笔记本管理员能取消；给 wiki 在调用方事务里开回滚任务的端口。
- 平台、契约：`api/modules/wiki.yaml` 的十个操作与 `api/openapi.yaml` 的目录；`common.yaml` 的 `locks` 的说明；`events.yaml` 的 `mode` 与 `EventMode` 的说明；`api/mcp/tools.json` 的两个新工具与三处措辞；`api/mcp/prompts.json` 的 `upgrade`。
- 网页：4.15 的全部。
- e2e：分组 `e2e/stories/upgrade/`，九个故事（第 9 节）；`fixtures/assert/wiki.ts`。
- 构建与 CI：第七份组合根（`Makefile` 的分份、`test-go` 的列表、CI 的矩阵与注释、`06-environment.md` 的分份一条），P1/S1。
- 文档：README 的 LLM Wiki 一节加"升级"（含怎样把 agent 接到这本笔记本）；M9 的 MCP 人工清单加一节 K。

**不做**：

- 批准之后的整体撤销（负责人第 3 条）；冻结的时限与自动回滚（负责人第 1 条）。
- 退回时附一段说明的文字（第 183 条）；"选一页作来源根"的选择器（第 173 条）。
- 已结束的升级的报告的读（网页只看进行中的那一次）；历史里标出升级的变更集（`upgrade_id` 只记在库里，第 179 条）。
- MCP 的 `notebooks`、`open_notebook` 与握手的说明加升级的状态（第 196 条）；MCP 开后台任务、放弃、批准或结束回滚（MCP 只有检查与提交）。
- 网页上显示 MCP 地址（负责人 M9 的决定照旧，第 184 条）。
- 升级时查来源区里的重复（第 204 条）。

## 3. 完成标准

总体设计 12.5 的通用标准之外：

1. **状态机**：4.3 的每个转移经 REST（与工具）各有测试，错的状态答 `wiki.upgrade_state`；转移都在笔记本行的 `FOR NO KEY UPDATE` 下；每个转移（存下报告在内）经订阅链发 `mode`，另一条流收到（组合根交空的订阅者时失败）。`notebooks` 的升级列与 wiki 的行一致，`checkPages` 守着（P1/S2 起）。
2. **发起**：4.4 的码依次各有测试；模板的两页按缺的补写、挂在升级会话上、不进基线；基线等于发起时活着的节点；闸门 6 的逐页基线与索引是否过时在发起的事务里量，占一个 lint 的名额；发起与正在跑的写、正在开的编辑会话、过期之前读了时钟的心跳、正在上传的导入的交错各有确定的测试。
3. **冻结到达每条路径，也只到那里**：4.5 的规则在第 5 节列出的 14 个 REST 写操作、5 个 MCP 写工具、编辑会话的开启、后台的导入、恢复与撤销的每一批上各有整个程序的测试，组合根不登记冻结的守卫时失败；有预检的路径另有只有预检答的情形；发起人被降为写者之后他的 MCP 写被拒；导出、改名称与开放程度、成员、删除笔记本、强制解锁在三种状态里照常答成功。
4. **护栏**：删除基线节点被拒（P1/S1 起）；不改写链接的移动与改名被拒；正文的切分照 4.6 的每种形状（没有 frontmatter、BOM、`\r\n`、有效的、无效的、只有开头一行的）各有反例；基线页的正文变了、`batch` 折进的改写各答自己的码，`batch` 整个回滚、拒绝标在约定的操作上；frontmatter 的改、改名、移动、新建、系统的链接改写放行；账本随每个正文写更新。
5. **闸门**：4.7 的每道闸门过与不过各有测试，在一个快照里；闸门 6 逐页（抵消、封顶都不再让它通过）；`stale_pages > 0` 时检查不过、提交答 `wiki.upgrade_stale_index`；检查与提交的交错（提交不并入已开始的检查，提交的两次事务之间的退回、放弃）各有确定的测试；10 万页的检查有成本守卫。
6. **批准**：一个事务里模式、来源根、哈希（等于从头重建，`checkPages` 与性质测试）、基线的删除与 `mode`；之后根下的 `AGENTS` 受 M10 的保护；任何一步失败整个回滚。
7. **回滚**：放弃与驳回之后，基线节点的位置、名称、次序与正文都回到发起时，升级中新建的都在回收站里；回滚的任务停下（取消、重启、超时）之后继续能做完；停下之后结束回滚，冻结解除、剩下的数记下；回滚的计划在 10 万个条目上有成本守卫。
8. **回收站里的保留标题**：批准之后，撤销一个把根下 `index` 删掉的旧变更集答 `page.virtual_file`（同步与后台），`revertChangeset`、`startRevert` 声明它；同一项从回收站恢复（`restoreNode` 与 `startTrashRestore`）仍然成功、名为 `index 2`。
9. **MCP**：两个工具进 `api/mcp/tools.json`（两个入口）、矩阵、码表、内置的说明（两个入口在 SDK 的编码下各至少剩 300 字节）、`Fit`；prompt 进 `prompts.json`、只在升级中给出；最后一跳照 M9、M10 的测试。
10. **网页**：第 9 节的页面版本通过；只读的判断收成一处，控件表的每一项有测试；新码与新种类在两种语言里都有文字，由加它们的服务端 Step 一起写。
11. **人工**：M9 的 MCP 人工清单加一节 K（upgrade 的 prompt、两个工具与一次护栏的拒绝）；输入法清单不加步（M11 没有新的文字输入框）。

## 4. 关键决定

### 4.1 模块与归属

| 项 | 决定 | 放弃的方案 |
|---|---|---|
| 编排 | wiki（总体设计 8.2 已写"升级流程（M11）"）：十个 REST 操作、两个工具、prompt、闸门与报告都在 wiki；别的模块只交窄的端口 | 放在 notebook：要闸门、哈希、回滚、wiki 的表的端口，与 M10 4.1 的归属相反 |
| 冻结的状态 | `notebooks` 的四列（升级的 id、状态、发起人、开始时刻）。单元在笔记本行的锁下本来就读一次 `ShareNotebook`/`LockNotebook`（`notebook/adapter/postgres/queries/notebooks.sql:16-34`），多选这四列就进 `Write.Notebook`，冻结的守卫零额外读；状态转移在笔记本行的 `FOR NO KEY UPDATE` 下，单元里不变（同降级，`notebook/app/downgrade_notebook.go:68-81`）。不扩 `notebooks_mode_check`（M10 4.3） | 只在 wiki 的表：每个笔记本的每个写入单元多一次跨模块的读 |
| 会话与报告 | wiki 的 `notebook_upgrades`：发起人、提议的来源根、发起时建了哪几页、基线的数、提交与存下的报告、回滚的任务、结局与回滚是否做完 | — |
| 基线与账本 | page 的 `upgrade_nodes`：每个基线节点一行（种类、发起时的父节点、名称、次序值，页另有版本号、正文哈希、字节数、发出的待建目标数与账本的 `accepted_hash`），另有发起时建的来源根的保护行。守卫与回滚每步都读它们，都是 page 的数据；总体设计 7.2 把"升级前的节点清单与正文哈希"放在 wiki，照改（第 7 节） | 放在 wiki：守卫要经跨模块的端口每步读 |
| 守卫 | page 的 `page.NewFreezeGuard(pool, authorizer, admin)`，既是写入守卫也是会话的否决者，登记在模式与锁之前（`bootstrap/registrants.go:268`、`:270` 的两个列表的最前） | 加一个"单元否决者"让冻结的码排在领域的码之前：新扩展点与 13.1 第 4 条的修订，换来的只是码的次序（4.16） |
| 状态与模式的写 | notebook 模块根的端口 `notebook.NewUpgradeStates(pool, subscribers)`，方法都在调用方的事务里：`Lock`（工作区行 `FOR SHARE`、笔记本行 `FOR NO KEY UPDATE`，交回带升级的状态）、`Start`、`SetState`、`Approve`（模式、来源根并清升级的列）、`Clear`、`Publish`（依次调模式变化的订阅者）。写与通知分开，批准的 `Publish` 在重建哈希之后（同降级的 `SetMode`、`Forget`、`publishMode` 的次序，`notebook/app/downgrade_notebook.go:87-97`）。今天的订阅链只交给降级（`notebook/module.go:139-141`），别的模块调不到；"升级改模式时同样经它"（12.4）要这个端口，是 12.1 第 6 条的例外（第 8 节） | 在 notebook 里写批准等用例：notebook 要闸门、哈希、回滚的端口 |
| 动作的答复 | 十个操作答 wiki 自己的 `Upgrade`，不答 `Notebook`（第 201 条）：`Notebook` 的 schema 在 `notebook.yaml`，wiki 的生成代码不能引用别的模块的生成包（`archtest/rules_test.go:41-42`："only bootstrap imports modules"、"generated code is imported only by its own adapter"），wiki 也不该复制有效角色与成员数的口径。网页在动作之后重读笔记本列表 | 答 `Notebook`、notebook 导出按执行者读视图的端口（审查 C-M13 的建议）：契约要跨模块引用 schema |
| 回滚 | 计划与批在 page 的 `JobWrites`（同 M8/P4 的撤销）；任务的行与运行在 transfer，新种类 `rollback`；回滚做完时 transfer 在写任务结束的同一个事务里调 wiki 的 `RollbackEnd` | wiki 自己的 River 任务：要另写心跳、中断与分批 |
| 来源的哈希 | page 的 `page.NewSourceRebuild(pool, attachments)`，在调用方的事务里清掉这本笔记本的哈希、再为指定的根的严格后代写上。附件的 SHA-256 在 asset 的表里，page 的 SQL 读不到，经附件的端口取（`page/app/ports.go:587-598`）；所以它不只凭连接池，组合根交 `attachmentInfos{asset.NewAttachmentInfos(pool)}`（同 `bootstrap/deps.go:172`），照 `page.NewModeGuard` 写进 13.1 第 21 条的例外 | 在 SQL 里读 `asset_blobs`：只有 00050 的数据迁移经审查例外地这样做过 |

**组合根的次序**（`bootstrap/wire.go:180-254`）：page → asset → linking → wiki 的预先建好的部分（今天是 `VirtualFiles` 与 `Template`，M11 加 `RollbackEnd`，都只凭连接池与别的模块只凭连接池的端口）→ transfer（`Deps.RollbackEnd` 拿它）→ notebook → `wiki.New`（拿 transfer 的 `(*transfer.Module).Rollbacks()` 与 page 的 `TreeWrites()`）。`wiki.New` 今天已经在 transfer 之后（`bootstrap/wire.go:254`），所以没有环：wiki 用 transfer 开任务，transfer 结束时用的是预先建好的 `RollbackEnd`。

**动作**：wiki 与 page 的冻结守卫都判 `notebook.change_mode`（第 195 条），这个值由组合根从 notebook 模块根交给它们（同 transfer 的 `RestoreAction`、`RevertAction`，`bootstrap/deps.go:250`），两个模块都不在自己的 `Actions()` 里列它（`bootstrap/actions_test.go:21-28` 要各模块的动作的并集恰好等于规则表）。

### 4.2 数据：迁移

迁移的号照合并的次序取。每个新的约束与索引都有名字，进 `TestConstraintAndIndexNames`（`server/migrations/schema_test.go:163`）的表，各有 13.4 第 1 条的反例（一个反例只钉住一种形状，05-lessons 第 1 节：下面每个检查的每个析取项各一个）；每条都有 Down；新表进 `deploy/runtime-grants.sql` 的数据授权（`:17-20`）与 `server/sqlc.yaml` 的模块一项。

| 迁移 | 改动 | 为什么 |
|---|---|---|
| 00055 notebook `notebooks`（P1/S1） | `upgrade_id uuid`，`upgrade_state text`，`upgrade_by_id uuid REFERENCES users`，`upgrade_at timestamptz`；`CONSTRAINT notebooks_upgrade_check CHECK ((upgrade_state IS NULL) = (upgrade_id IS NULL) AND (upgrade_id IS NULL) = (upgrade_by_id IS NULL) AND (upgrade_id IS NULL) = (upgrade_at IS NULL) AND (upgrade_state IS NULL OR upgrade_state IN ('upgrading', 'reviewing', 'rolling_back') AND mode = 'standard'))`。每个合取都不会因 NULL 而放过（初稿的写法在"状态为空、另三列有值"时是 NULL，被接受，审查 B-M1 实测）。Down 去掉四列：之前的程序不知道升级，冻结着的笔记本照普通的读（迁移的注释写明） | 4.1。最后一个析取让 LLM Wiki 不会带着升级；`upgrade_id` 不建外键（wiki 的表归 wiki 清理，`notebooks` 指向它会打乱从叶到根的清理，同 `00048_notebook_notebooks_mode.sql:1-6` 的理由） |
| 00056 page `upgrade_nodes` 与 `changesets`（P1/S1） | `upgrade_nodes (upgrade_id uuid NOT NULL, node_id uuid NOT NULL, notebook_id uuid NOT NULL, baseline boolean NOT NULL, kind text NOT NULL, parent_id uuid, name text, sort_order` 与 `nodes.sort_order` 同类型`, revision integer, content_hash bytea, byte_size integer, wanted integer, accepted_hash bytea, CONSTRAINT upgrade_nodes_pkey PRIMARY KEY (upgrade_id, node_id))`；`upgrade_nodes_kind_check CHECK (kind IN ('page', 'asset'))`；`upgrade_nodes_baseline_check CHECK (baseline = (name IS NOT NULL) AND (name IS NULL) = (sort_order IS NULL) AND (baseline OR kind = 'page' AND parent_id IS NULL))`；`upgrade_nodes_page_check CHECK ((baseline AND kind = 'page') = (revision IS NOT NULL) AND (revision IS NULL) = (content_hash IS NULL) AND (revision IS NULL) = (byte_size IS NULL) AND (revision IS NULL) = (wanted IS NULL) AND (revision IS NULL) = (accepted_hash IS NULL))`；`upgrade_nodes_wanted_check CHECK (wanted IS NULL OR wanted BETWEEN 0 AND 10000)`（一页至多 `MaxLinks = 10,000` 条链接，`linking/domain/facts.go:114`）；索引 `upgrade_nodes_notebook_id_idx (notebook_id)`。`changesets.upgrade_id uuid`（不建外键），部分索引 `changesets_upgrade_id_idx ON changesets (upgrade_id) WHERE upgrade_id IS NOT NULL`。不带 `deleted_at`、不建外键（照 `edit_sessions` 与 linking 的派生表）：行在升级结束时删、笔记本删除时删，所以不要清理器。`runtime-grants.sql` 另给 `upgrade_nodes` 的 `MAINTAIN`（第 220 条，照 `:25-30` 的写法）。Down 去掉表与列 | 4.4、4.6、4.7、4.10。`baseline = false` 的一行是发起时建的 `raw` 的保护（第 194 条）：只防删除，不判正文、不进回滚的计划 |
| 00057 wiki `notebook_upgrades`（P1/S2） | `id uuid PRIMARY KEY`，`notebook_id uuid NOT NULL REFERENCES notebooks ON DELETE RESTRICT`，`state text NOT NULL`，`ending text`，`source_root_id uuid NOT NULL`，`seeded_root boolean NOT NULL`，`seeded_agents_id uuid`（发起时建的 `AGENTS`，没建为空），`locale text NOT NULL`，`baseline_nodes integer NOT NULL`，`baseline_pages integer NOT NULL`，`baseline_wanted integer NOT NULL`，`baseline_wanted_capped boolean NOT NULL`（这两列只给报告的前后对比，闸门 6 逐页比，4.7），`submission_id uuid`，`submitted_by_id uuid REFERENCES users`，`submitted_at timestamptz`，`report jsonb`，`rollback_job_id uuid`，`rollback_complete boolean`，`rollback_left_nodes integer`，`rollback_left_pages integer`，`rollback_left_created integer`，审计四列（`created_by_id` 是发起人、`created_at` 是开始的时刻），`deleted_at timestamptz`。检查：`notebook_upgrades_state_check`（`upgrading`、`reviewing`、`rolling_back`、`approved`、`rolled_back`）；`notebook_upgrades_ending_check`（`abandoned`、`rejected`）；`notebook_upgrades_locale_check`（`zh-CN`、`en`）；`notebook_upgrades_baseline_check CHECK (baseline_nodes >= 0 AND baseline_pages BETWEEN 0 AND baseline_nodes AND baseline_wanted BETWEEN 0 AND 10000 AND baseline_wanted_capped = (baseline_wanted = 10000))`（照 `LintCount` 的不变式，`wiki/app/lint_projection.go:39`）；`notebook_upgrades_ending_state_check CHECK ((state IN ('rolling_back', 'rolled_back')) = (ending IS NOT NULL))`；`notebook_upgrades_submission_check CHECK ((submission_id IS NULL) = (submitted_at IS NULL) AND (submitted_at IS NULL) = (submitted_by_id IS NULL))`；`notebook_upgrades_submission_state_check CHECK ((state <> 'upgrading' OR submission_id IS NULL) AND (state NOT IN ('reviewing', 'approved') OR submission_id IS NOT NULL))`；`notebook_upgrades_report_check CHECK (report IS NULL OR submission_id IS NOT NULL AND state <> 'upgrading' AND jsonb_typeof(report) = 'object' AND octet_length(report::text) <= 65536)`；`notebook_upgrades_approved_check CHECK (state <> 'approved' OR report IS NOT NULL)`；`notebook_upgrades_rollback_job_check CHECK ((rollback_job_id IS NOT NULL) = (state IN ('rolling_back', 'rolled_back')))`；`notebook_upgrades_rollback_end_check CHECK ((rollback_complete IS NOT NULL) = (state = 'rolled_back') AND (rollback_complete IS DISTINCT FROM false) = (rollback_left_nodes IS NULL) AND (rollback_left_nodes IS NULL) = (rollback_left_pages IS NULL) AND (rollback_left_nodes IS NULL) = (rollback_left_created IS NULL))`。部分唯一索引 `notebook_upgrades_notebook_id_key ON notebook_upgrades (notebook_id) WHERE state IN ('upgrading', 'reviewing', 'rolling_back') AND deleted_at IS NULL`；索引 `notebook_upgrades_notebook_id_idx (notebook_id)`（清理笔记本时外键的核对）。wiki 第一次有表：`server/sqlc.yaml` 加 wiki 的一项（今天没有，`grep modules/wiki server/sqlc.yaml` 为空）、`wiki/adapter/postgres`、`runtime-grants.sql` 的一个表名、清理器排在 notebook 之前（`bootstrap/registrants.go:311-314`）、笔记本删除的订阅者（`:194-197`）以同一时刻软删除这些笔记本的行 | 一本笔记本至多一次进行中的升级由索引守着；提交、报告、回滚的任务与结局都绑在状态上，状态机到不了的行写不进去（审查 B-M2 实测的五种都被挡住）；驳回与放弃之后提交与报告留着（第 221 条）；结束的行随笔记本软删除、照常清理（13.1 第 6 条） |
| 00058 page `changesets`（P1/S3） | `changesets_kind_check` 加 `rollback`（`00034_page_changesets_history.sql:7`）；`changesets_reverts_check` 不变（`rollback` 没有 `reverts_id`）。Down 把 `rollback` 改成 `edit` 再收紧检查（照 00034 的 Down，`:15-17`） | 4.10、第 192 条 |
| 00059 transfer `transfer_jobs`（P1/S3） | `transfer_jobs_kind_check` 加 `rollback`；`upgrade_id uuid`，`CONSTRAINT transfer_jobs_upgrade_check CHECK ((upgrade_id IS NOT NULL) = (kind = 'rollback'))`；`transfer_jobs_root_check` 的 `CASE` 加 `WHEN 'rollback' THEN root_id IS NULL`；`transfer_jobs_writing_key` 的谓词加 `rollback`（`00039_transfer_background_jobs.sql:26-27`），`transfer/adapter/postgres/queries/jobs.sql:33` 的写入任务的探查同样。Down 删掉 `rollback` 的行、恢复 00039 的定义 | 一本笔记本同时一个写入任务，回滚也算 |

没有新的配置项：上限都是代码里的常量（4.17）。

### 4.3 升级的状态机

模式仍只有 `standard` 与 `llm_wiki`；升级的状态是 `notebooks.upgrade_state` 与 wiki 的行的 `state`，两边在同一个事务里写。

| 从 | 动作（操作） | 到 | 谁 | 写什么 | 状态不对时 |
|---|---|---|---|---|---|
| 没有升级的普通笔记本 | 发起（`startUpgrade`） | `upgrading` | 任何笔记本管理员 | 4.4 | LLM Wiki 答 409 `notebook.already_llm_wiki`；已有升级答 409 `notebook.upgrading` |
| `upgrading`，或 `reviewing` 而没有报告 | 提交（`submitUpgrade`、`upgrade_submit`）的第一个事务 | `reviewing`（还没有报告） | 任何笔记本管理员，或他的 PAT 的 MCP | 新的 `submission_id`、`submitted_by_id`、`submitted_at` | 409 `wiki.upgrade_state` |
| `reviewing`（这次提交、还没有报告） | 提交的第二个事务，闸门全过 | `reviewing`（存下报告） | 同上 | `report` | 这次提交已被退回、放弃或被后一次提交取代：409 `wiki.upgrade_state` |
| `reviewing`（这次提交、还没有报告） | 提交的第二个事务，闸门不过或算不出 | `upgrading` | 同上 | 清掉提交的三列 | 同上 |
| `reviewing` | 退回（`sendBackUpgrade`） | `upgrading` | 任何笔记本管理员 | 清掉提交的三列与 `report` | 409 `wiki.upgrade_state` |
| `reviewing`（有报告） | 批准（`approveUpgrade`） | 没有升级的 LLM Wiki；行 `approved` | 任何笔记本管理员（负责人第 4 条） | 4.9 | 没有报告也答 409 `wiki.upgrade_state` |
| `reviewing` | 驳回（`rejectUpgrade`） | `rolling_back`，`ending = rejected` | 任何笔记本管理员 | 4.10 | 409 `wiki.upgrade_state` |
| `upgrading`、`reviewing` | 放弃（`abandonUpgrade`） | `rolling_back`，`ending = abandoned` | 任何笔记本管理员（负责人第 1 条） | 4.10 | 409 `wiki.upgrade_state` |
| `rolling_back`，回滚的任务已停下 | 放弃（继续回滚） | `rolling_back` | 任何笔记本管理员 | 新开一个回滚的任务 | 任务在排队或运行：409 `transfer.busy` |
| `rolling_back`，回滚的任务已停下 | 结束回滚（`endUpgradeRollback`，主持者第 212 条） | 没有升级的普通笔记本；行 `rolled_back`，`rollback_complete = false` | 任何笔记本管理员 | 4.10 | 任务在排队或运行：409 `transfer.busy`；不在 `rolling_back`：409 `wiki.upgrade_state` |
| `rolling_back` | 回滚的任务做完 | 没有升级的普通笔记本；行 `rolled_back`，`rollback_complete = true` | 系统（任务的执行者） | 4.10 | — |

- **锁**：每个转移是一个事务（提交是两个），次序是工作区行 `FOR SHARE` → 笔记本行 `FOR NO KEY UPDATE` → 判定 → 读与写。wiki 的行只在笔记本行的锁下改，不另加锁；它排在 13.1 第 5 条的全局次序里笔记本行之后。回滚的任务开在同一个事务里时，transfer 在笔记本行之后取它的队列锁（`transfer/app/start_background.go:169-172` 的次序；队列锁是 advisory 锁，`transfer/adapter/postgres/queries/jobs.sql:11-14`）。
- **事件**：每个转移经 `notebook.NewUpgradeStates` 的 `Publish` 调订阅链，发一次 `mode`（4.12）。提交通过时发两次（进审阅、存下报告），不过时也发两次（进审阅、退回）。
- **审阅中的读**：检查（`checkUpgrade`、`upgrade_verify`）在 `upgrading` 与 `reviewing` 都答；`rolling_back` 答 409 `wiki.upgrade_state`。
- **降级**：升级期间笔记本是普通的，`downgradeNotebook` 在锁下就答 409 `notebook.not_llm_wiki`（`notebook/app/downgrade_notebook.go:73-75`），不需新规则（M10 写的"由 M11 定"的答案，第 7 节写明）。
- **删除笔记本**：照常（管理员显式的破坏性操作，第 175 条）；wiki 的行随之软删除，`upgrade_nodes` 的行随 page 的笔记本删除删去，回滚的任务随 transfer 的笔记本删除删去（`bootstrap/registrants.go:194-197` 的订阅者）。
- **发起人离开**：发起人被停用（PAT 失效）、离开、被降为写者或笔记本成了无主的，升级会话留着；任何别的管理员（无主时接管之后的管理员）可以放弃、批准、驳回或结束回滚。横幅写清"任何笔记本管理员都可以放弃"。
- **回滚任务的取消**：只有笔记本管理员能取消回滚的任务（第 218 条）。今天任务的发起人或笔记本管理员都能取消（`transfer/app/cancel.go:45-64`、`views.go:94-103`，规则 `transfer.cancel` 给读者以上，`access/domain/rules.go:151`），开回滚任务的管理员之后被降为写者时还能反复让它停下；`views.sees` 对 `rollback` 的任务要 `admin`。

### 4.4 发起

`POST /api/v0/notebooks/{notebook_id}/upgrade`，请求体 `{template_locale?: "zh-CN" | "en"}`（缺省 `en`，同 `createNotebook` 的 `template_locale`，M10 4.4；网页交界面语言）。wiki 的用例 `StartUpgrade`，次序：

1. 请求体的取值：不认识的语言 422 `validation_failed`（字段 `template_locale`，`invalid_format`）。
2. 不加锁读笔记本的工作区：没有或看不到答 404 `notebook.not_found`。
3. 事务里：`notebook.NewUpgradeStates` 的 `Lock`（工作区行 `FOR SHARE`、笔记本行 `FOR NO KEY UPDATE`），判定 `notebook.change_mode`：看不到 404，不是管理员 403 `forbidden`。读一次时钟（13.1 第 9 条：之后每一步，含会话的判活与 lint 的 `computed_at`，都用这一刻）。
4. 模式是 `llm_wiki`：409 `notebook.already_llm_wiki`。
5. 已有进行中的升级：409 `notebook.upgrading`（并发的两次发起，后一次在锁下看到前一次）。
6. 有排队或运行的写入任务：409 `transfer.busy`（`transfer.NewWritingJobs(pool).Writing`，`transfer/writing_jobs.go:26`，同降级）。
7. 编辑会话：先删这本笔记本在这一刻已过期的会话行（墓碑在内），同开启的第一步（`page/adapter/postgres/queries/sessions.sql:32-36`，`page/app/unit_session.go:32`：这样一个在过期之前读了时钟的心跳找不到行可续，`sessions.sql:66-73`、`page/app/heartbeat_edit_session.go:45-53`）；再读活着的会话：有就 409 `wiki.upgrade_pages_locked`，problem 的 `locks` 按页 id 列出前 100 项（每页与编辑者），`detail` 写总数（一条语句 `LIMIT 100` 加 `count(*) OVER ()`，第 174、203 条）。管理员可以先强制解锁，或等闲置退出。
8. 根下的键：标题键 `raw` 或 `agents` 的根下节点是附件时 409 `wiki.upgrade_title_taken`（第 199 条：先改名再发起）。键 `raw` 是页时用它作提议的来源根（有子节点、正文非空都照用，主持者第 214 条：来源根的正文本来就可以改，5.4；它下面的节点从此按来源算，发起的对话框写明）。键 `agents` 是页时留着它（第 172 条：正文不改）。
9. 占一个 lint 的名额（`Runs.Hold`，第 219 条）：拿不到时 503 `server_busy` 带 `Retry-After: 1`。之后的写与第 13 步的计算都在这个名额里。
10. `notebook.NewUpgradeStates` 的 `Start` 写四列：新的 UUIDv7 作升级的 id，`upgrading`，发起人，这一刻。
11. page 记基线（`UpgradeBaseline.Capture`）：一条 `INSERT … SELECT`，这本笔记本每个活着的节点一行（`baseline = true`，父节点、名称、次序值；页另有 `page_contents` 的版本号、哈希、字节数，`accepted_hash` 等于哈希，`wanted` 先为 0）。答复节点数与页数。
12. 补写缺的页（第 173 条）：缺 `raw` 或 `AGENTS` 时经 `(*page.Module).TreeWrites().Seed` 写一个单元，`SeedSpec` 新加 `Upgrade`（这次升级的 id），单元的 `Write.UpgradeOwn` 由此设上：冻结的守卫放过它（4.5），`ensureChangeset` 给它的变更集记上 `upgrade_id`（4.10 的回滚因此也删掉它们：它们不在基线里）。页的正文是 `wiki.TemplatePages(llm_wiki, locale)` 里的那一页（`wiki/module.go:52-55`），在根下的末尾（`page/app/seed_writes.go:24-27`），变更集的说明是模板的（`wiki/app/template.go:78`）。建了 `raw` 时，page 给它记一行保护（`baseline = false`）。
13. 在这个事务里、笔记本行的锁下调 lint 的 `computeIn`（4.7）一次，种类是"升级的基线"，显式的 LLM Wiki 契约与提议的来源根，期限 `LintTimeout`（超时 503 `server_busy`，整个发起回滚）：`stale_pages > 0` 时 409 `wiki.upgrade_stale_index`（第 198 条：先由服务器管理员运行 `nervewiki reindex`），整个事务回滚；否则得到 `wanted` 的总数（报告用）与每个基线页发出的待建目标数（闸门 6 用，4.7），后者以一条 `UPDATE … FROM unnest(…)` 写进基线行的 `wanted`（第 187 条）。
14. 插入 wiki 的行（提议的来源根、`seeded_root`、`seeded_agents_id`、语言、基线的两个数、`wanted` 的总数与是否封顶）。
15. `Publish`（`From`、`To` 都是 `standard`）；提交；放开名额。
16. 提交之后对 `upgrade_nodes` 做一次 `ANALYZE`（第 220 条；失败只记一行 warn，不改答复）；一行 info 日志（4.18）。答复 200 `Upgrade`。

步 11 在步 12 之前：模板的页不进基线（第 173 条），`AGENTS` 的模板正文可以由 agent 定制。步 12 的 `Seed` 在调用方的事务里重入同样的锁（`page/app/unit.go:198-217` 的注释），它读到的 `Write.Notebook` 已带 `upgrading`。

**空笔记本与整理好的笔记本**：发起之后根下有正文非空的 `AGENTS` 与来源根（模板的或原有的），所以空的普通笔记本在网页上发起、检查、提交、批准就能走完（负责人第 2 条），总体设计 5.7 的"空的普通笔记本过得了每一道闸门"随第 173 条成立（第 7 节照改写法）。

**模板的正文**：语言取请求的 `template_locale`，同建 LLM Wiki（M10 4.4）；`AGENTS` 的模板说它是这本 LLM Wiki 的约定、可以改写，与新建时一样。

### 4.5 冻结：守卫、否决者与预检

**输入**：`page/domain.NotebookState`（`page/domain/mode.go:17-20`）加 `Upgrade NotebookUpgrade{ID uuid.UUID; State UpgradeState; By uuid.UUID}`，只有可比较的字段（`notebook/notebooks_test.go:74` 用 `!=` 比较 `State`）；`notebook.State`（`notebook/notebooks.go:30-33`）同样，组合根的 `toPageState` 逐字段转换（`bootstrap/page_notebooks.go:32-38`）。`FindNotebook`、`LockNotebook`、`ShareNotebook`（`notebooks.sql:9-34`）与 `ListNotebooks`（`:55-68`，它自己列了 `mode, source_root_id`，不经前三条）都多选四列：`listNotebooks` 是网页的横幅与只读判断的来源，漏了它网页上每个编辑控件都放开、每次写都答 409（审查 B-M6）。`Write`（`page/app/extension.go:27-47`）加 `UpgradeOwn uuid.UUID`：这个单元是哪一次升级自己的写（发起时补写的页、回滚的单元）；只有 `SeedSpec.Upgrade` 与回滚的 `JobSpec` 设它，REST 与 MCP 的适配器建不出它（第 177 条）。

**守卫** `FreezeGuard.GuardWrite(ctx, s Step)`：

1. `s.Notebook.Upgrade.ID` 为零：放过，什么也不读。
2. `s.UpgradeOwn == s.Notebook.Upgrade.ID`：放过（发起的补写在 `upgrading`，回滚的单元在 `rolling_back`；不看 `Kind`，第 177 条）。
3. 状态是 `reviewing` 或 `rolling_back`：拒绝，409 `notebook.upgrading`（第 183 条）。
4. 状态是 `upgrading`：`s.Client.IsMCP()`（`page/domain/client.go:44`）且 `s.By == Upgrade.By`，并且经 `s.Grants.Ask` 问一次 `notebook.change_mode`（每个单元至多一次，`page/app/grants.go`），三者都成立才进护栏（4.6），否则 409 `notebook.upgrading`。`Write` 不带 PAT（`page/app/extension.go:27-47`），所以放行的是"发起人的任何一个 MCP 客户端"，写明（第 175 条）。`grants.go:15` 的注释"Only the mode guard and the unit's own locked checks ask"改为也列冻结的守卫。

拒绝经 `refusalOf`（`page/app/unit.go:333-339`）标出节点，于是得到 M10 建好的去处：`batch` 的失败项（`page/app/apply_batch.go:617-625`）、transfer 的 `refused`（`bootstrap/transfer.go:253-257`），`refusal.code` 是自由的字符串，不改契约。

**否决者** `FreezeGuard.VetoEditSession`：任何升级状态都拒绝开启，409 `notebook.upgrading`（MCP 不开会话，发起人的网页同样只读）。接管在否决者之前结束自己的会话（`page/app/unit_session.go:35-44`），否决让整个单元回滚，接管也不生效。

**登记**：`pageRegistrants`（`bootstrap/registrants.go:260-273`）里 `guards: [freeze, mode, lock]`、`sessionVetoers: [freeze, mode, lock]`（总体设计 8.3 的次序）；签名照旧是 `(pool, authorizer, logger)`，冻结的守卫要的动作由组合根取 `notebook.ActionChangeMode`。M10 的移交与 4.6 写的 `pageRegistrants(pool, authorizer)` 是旧的签名，12.4 那一行是对的（第 7 节"文档与代码不符之处"）。排在最前只决定"冻结与锁都拒"时答哪个码；这种情形在实际里到不了（发起时没有活着的会话，冻结期间开不了新的），所以不是用户看得到的效果，`TestUpgradeFreezeAnswersFirst` 以 SQL 造出它、钉住次序（便宜，留着）。

**预检**：上传的 `AssetWrites.Check`（`page/app/asset_writes.go:49`）、后台恢复的 `CheckRestore`（`page/app/jobs.go:118`）、后台撤销的 `CheckRevert`（`:168`）、导入的 `StartImport.Check`（`transfer/app/start_import.go:67`）在判定之后各判一次"这本笔记本有没有升级"（不加锁的状态的 `Upgrade.ID` 不为零就答 `notebook.upgrading`）。它们只经 REST 到达，客户端只会是 `web` 或 `api`，`FreezeAdmits` 对它们不会放过任何写，所以不调它、`CheckRevert` 的签名不改（审查 A-M6；主持者第 175 条写的"用请求真实的客户端"在这里没有可观察的效果，写明）。同步的 `revertChangeset` 不加预检：它在单元之前做不加锁的计划与解析（`page/app/revert_changeset.go:92-95`），冻结期间别人的同步撤销做完这些才被守卫拒绝，只多花一次计划的时间；守卫的码本来就在领域的码之后（下面"码的次序"）。

**transfer 开任务时**：判冻结放在导入建任务的事务（`transfer/app/start_import.go:249` 的 `lock` 之后）与 `startBackground`（`transfer/app/start_background.go:169` 的 `lock` 之后）里，不放进共用的 `StartDeps.lock`（`transfer/app/start.go:105-113`）：导出也调它（`:73`），冻结期间导出照常（第 175 条）。transfer 的 `Notebooks.ShareByID` 改为交回升级的状态，组合根从 `notebook.ShareState` 转换。回滚的任务在 `startBackground` 里放过（种类 `rollback`、`upgrade_id` 等于笔记本当前的升级）。正在上传的导入（进程内的占位，M10 4.5）在建任务行的锁下被拒，文件照导入失败时的处理删掉。

**不冻结**：笔记本的名称、开放程度、成员、导出、删除；强制解锁（`releaseEditLock` 不经守卫，冻结期间也没有会话）；心跳与结束会话（不是单元）。字节不变的正文写在守卫之前返回（`page/app/unit_content.go:66-78`），冻结期间照旧答成功、什么也不写（第 211 条）。

**码的次序**：守卫在单元里领域的检查之后跑（`page/app/unit.go:368`），所以冻结期间别人的写可能先答 422、`page.title_taken`、`page.revision_mismatch` 等，再轮到 `notebook.upgrading`；预检的路径在判定之后先答它。接受（13.1 第 4 条），5.8 的写法改清楚（第 7 节）。

### 4.6 护栏

只在 `upgrading`、只对发起人的 MCP 写（4.5 第 4 步之后）。分两块：删除在 P1/S1（回滚依赖"基线节点删不掉"，要从能发起的那一刻起成立，审查 A-M4），其余在 P2/S1。

**删除**（P1/S1）：`OpDelete` 的 `Changes` 列出整棵子树的每个节点（`page/app/unit_delete.go:34-51`）。其中有 `upgrade_nodes` 里的节点（基线节点或提议的来源根的保护行）时拒绝 409 `page.upgrade_protected`，一条按 `(upgrade_id, node_id = ANY(…))` 的读。拒绝以 `domain.Refuse(err, 第一个受保护的节点)` 标出，不让 `refusalOf` 默认标子树的根（`page/app/unit.go:333-339` 取 `s.Changes[0]`，那可能是 agent 新建的文件夹，审查 A-N3）：`batch` 的失败项与 MCP 的文字指向真正受保护的那个。升级中新建的节点可以删。

**不改写链接的移动与改名**（P2/S1，审查 B-I2）：MCP 的 `move` 与 `batch` 的移动都有 `update_links`（`api/mcp/tools.json` 的说明："leaves every page linking to it as it is: those links may then lead elsewhere, or nowhere"）。单个的 `move` 经 `page/app/relocate_path.go:85-86` 放进 `Write.Options.UpdateLinks`；`batch` 的单元整体是 `UpdateLinks: true`（`page/app/apply_batch.go:255`），每个移动经 `Placement.KeepLinks`（`:769-770`、`page/app/unit_relocate.go:40`）让 `follow` 不跟（`page/app/unit.go:537`），`Step` 里今天没有这个标记。改：单元在搬迁的步上设 `Step.KeepLinks`（来自 `u.keepLinks`）；护栏对 `OpMove`、`OpRename` 的步，`!s.Options.UpdateLinks || s.KeepLinks` 时拒绝 409 `page.upgrade_keep_links`。不改写链接，原有页的链接就断了或指向别处，而它们的字节不变，闸门 6、7 都看不出；agent 可能为了"不碰原有页的正文"关掉改写，playbook 写明系统的改写是放行的。

**正文**（第 189 条）：`body(x)` 是 `x` 去掉 frontmatter 之后的字节，再去掉开头的一个 BOM（U+FEFF）。frontmatter 的结束由 `platform/markdown.FrontmatterEnd`（`platform/markdown/frontmatter.go:102-108`）给出，经 page 已有的端口 `Markdown.FrontmatterEnd`（`page/app/ports.go:781-783`），不另写切分。它是语法上的：开头可以有 BOM，`---` 起、到下一个恰为 `---` 的行止，没有收尾行就是 0（`frontmatterSpan`，`:71-96`）。

**核实主持者说明里"无效的 frontmatter"一条**：`FrontmatterEnd` 只在没有收尾行时答 0。两种形状分开：
- 以 `---` 起、之后没有恰为 `---` 的行：答 0，没有 frontmatter（索引的 `frontmatter_valid` 也是真）。agent 在前面加一块有效的 frontmatter，旧的整页成了不变的正文，护栏放过，闸门 4 也过——同主持者的写法，钉住。
- 以 `---` 起、几行之后又有一行 `---`（中间不是有效的 YAML）：答收尾行之后的位置，这一块是 frontmatter。`Parse` 照样把它空白化（`platform/markdown/parse.go:22-23` 与 `frontmatter.go:112-119`：`frontmatterOf` 不论有效与否都交回结束的位置），阅读视图从不把它当正文显示。护栏照同一个切分：agent 可以就地改这一块（例如改成有效的 YAML，把原文放进一个字符串属性），也可以把页移进来源区（lint 不查来源的 frontmatter，`linking/app/lint_scope.go:221-228` 只收区外的页）；在前面另加一块会让旧的一块成为正文，正文变了，被拒。playbook 写明这两种做法。

**比较**（P2/S1）：每一个正文步，比"这一步之前的当前正文"与"这一步之后的正文"的 `body` 的 SHA-256。之后的那份由单元算：`StepContent`（`page/app/extension.go:76-80`）加 `Body []byte`，只在 `Write.Notebook.Upgrade.State == upgrading` 时由单元从它手里的新正文算（`page/app/unit_content.go:65` 的 `c`），`WriterDeps`（`page/app/unit.go:25`）加 `Markdown` 端口；之前的那份由守卫在单元已持的正文行锁下读（`Nodes.PageContent`，`page/app/ports.go:205`），只对基线页读。不同且 `s.Rewrite == false` 时拒绝 409 `page.upgrade_immutable`。`s.Rewrite == true` 的步（参与者的链接改写与撤销写回的改写，`page/app/unit.go:742-746`）放行（第 7 条闸门的"差异只来自系统的链接改写"）。不比基线的正文：第一次系统改写之后它就不成立了。

**账本**（第 191 条）：单元在每个正文步写了之后（与来源哈希的维护同一处，`page/app/unit.go:385`），在 `Write.Notebook.Upgrade.ID` 不为零时执行 `UPDATE upgrade_nodes SET accepted_hash = $1 WHERE upgrade_id = $2 AND node_id = $3 AND baseline`：不是基线页时改 0 行。今天正文只经单元的 `writeContent` → `applyStep` 写，被拒的单元连账本一起回滚，所以对每个基线页 `content_hash = accepted_hash` 恒成立：它是绊线，只在将来出现一条绕过单元的正文写时才响，杀它的测试只能用 SQL 绕过单元造出来（审查 B-M4 与主持者的处置）。它不是护栏之外的独立证明。

**`batch` 的折进**（第 190 条，取主持者倾向的 B，按审查 A-M1 改判据）：`batch` 的计划把此前的移动对一页的链接改写折进用户的正文（`page/domain/batch_contents.go:94-99`），单元在这一页的第一个正文操作处写最终的正文、不标 `Rewrite`（`page/app/apply_batch.go:713-727`）。这样的页的正文因改写不同，护栏照上一段会拒。B：计划在改写之后记下 `rewritten != content`（这一次改写真的改了这一页），一页的任何正文操作有过这样的改写就是 `Folded`；`ContentWrite`（`page/app/unit_content.go:17-25`）加 `Folded bool`，随 `StepContent.Folded` 到守卫。正文变了而 `Folded` 时答 409 `page.upgrade_batch_rewrite`（"拆开：先写页，再移动，或分两个 batch"），否则答 `page.upgrade_immutable`；整个 `batch` 回滚。不取 `ps.Moved`：它只要两个正文操作之间有任何移动就是真（`batch_contents.go:95-96`，`MovesBetween` 不分页），索引过时、改写什么也没改时也是真（审查 A-M1 的探针），"无关的移动之后改正文"会答错码、叫 agent 去拆。拒绝标在这一页的最后一个正文操作上（计划的页有 `First`、`Last`，`page/domain/batch_plan.go:268`）：单元在 `First` 处写，`failed` 默认标那里（`apply_batch.go:617-625`），护栏的三个码改标 `Last`，playbook 与 5.8 照写。不取 A（计划按项判、单元做后备）：A 要计划读基线、按项判、与单元的后备做差分与性质测试；B 的码明确，agent 拆开就能过。分开调用 `move` 时改写由参与者以 `Rewrite = true` 写，放行；`batch` 里最后一个正文操作之后的移动同样由推后的参与者写（`Rewrite = true`）。

**同一页在一个 `batch` 里改了又改回**：单元只写最终的正文（M10 的 `Merged`），护栏只比最终的，放过；分开调用时第一次就被拒。这是一处与"结果等于按次序分开调用"（M9 4.11）不同的地方，写进 5.8 与 playbook（第 208 条）。

**改名、移动、新建**：除了上面的 `update_links`，不限（5.8）。agent 靠改名腾出根下的保留标题（闸门 3）与定制 `AGENTS`（第 172 条：把原页改名留着，另建一页 `AGENTS`；原页正文非空时也可以原样留用）。新建页的标题会让原有页里按后缀匹配的短链接改指新页（Obsidian 的语义，新建只重新解析、不改写，总体设计 4.4、4.5），接受（主持者第 213 条），playbook 提醒 agent。

**码**：四个护栏的码只在 MCP 的写工具上出现（REST 的写在冻结里就被拒，到不了护栏），进 `mcpOwnCodes()`（`bootstrap/mcp_client_test.go:338-340`）；码的文字写清是"升级开始时已有的"（第 210 条）。

### 4.7 闸门与检查报告

**入口与次序**（审查 C-M14）：检查（`checkUpgrade`、`upgrade_verify` 不带 `rule`）与条目（`listUpgradeCheckItems`、`upgrade_verify` 带 `rule`）是 wiki 的新入口，不是 `Lint.Execute`（它判的是 `lint.read`，对每位读者放行，`wiki/app/lint.go:58`、`:119`）。次序（照 lint 的规则页，`api/modules/wiki.yaml:63-71`）：解游标（条目）→ 判 `notebook.change_mode` → 规则名（条目）→ 不加锁读 wiki 的行（进行中的升级、提议的根；没有或在 `rolling_back` 答 409 `wiki.upgrade_state`）→ `limit`（条目）→ 经 `Runs.Do` 算（摘要或这条规则的一页）→ 交付之前再判一次 `notebook.change_mode`（同 lint 在运行之后再判，`:119`；合并的计算由别人开始，每个调用者交付前各判各的）。

**一个快照**：lint 不能嵌在写事务里（`platform/postgres/tx.go:137-145` 的 `ErrNestedSnapshot`），而 `(*Lint).compute`（`wiki/app/lint.go:198-272`）自己开快照。拆成 `computeIn(ctx, scope, query, kind, at)`：`compute` 的 `WithinSnapshot` 里的那一段，经 ctx 带的事务读（`postgres.TxFrom`，`platform/postgres/tx.go:68`；lint 的各端口只用 ctx 的事务），`computed_at` 用调用方交的时刻（今天 `compute` 在快照里自己读时钟，`:209`）；`compute` 照旧是 `WithinSnapshot(computeIn)`，交它自己读的一刻。检查在一个 `WithinSnapshot` 里依次：读笔记本的升级列与 wiki 的行，不是入口读到的那一次升级、或不在 `upgrading`/`reviewing` 就答 409 `wiki.upgrade_state`（审查 B-M3：排队等名额时这次升级可能已被批准或回滚完，基线行已删，闸门 7 会成了 0/0）；读 page 的几样；调 `computeIn`；读第 7 条：全部在同一个快照里。

**发起时的一致**（审查 B-N4）：发起的事务是 READ COMMITTED（`platform/postgres/tx.go:121` 的 `WithinTx`），`computeIn` 的分页读各取各的快照。它们一致，是因为写 lint 所读的行（`nodes`、`page_contents`、linking 的索引表）的每条路径都先取笔记本行：单元（`page/app/unit.go:244-256`）、`reindex`（`linking/app/reindex.go:94` 取 `FOR NO KEY UPDATE`）、笔记本删除；linking 没有别的后台写。发起持着 `FOR NO KEY UPDATE`，这些都等它。以后加一条不取笔记本行的写，要重看这里。

**名额与期限**：检查经同一个 `Runs`（`wiki/app/lint_runs.go:62`）：同一组名额（`MaxLintRuns`）、`LintTimeout`、超出答 503 `server_busy` 带 `Retry-After: 1`。`RunKey`（`wiki/app/lint_run_types.go:13-23`）加两种 `Kind` 与一个 `Scope uuid.UUID`：检查的键带升级的 id，同一次升级的检查之间合并；提交的键带这次提交的 `submission_id`，从不并入别的计算（审查 B-I1：`Do` 只按键找已有的 flight，不看它的快照是何时取的，`lint_runs.go:89-103`；提交若并入第一个事务之前开始的检查，存下的报告就不是冻结那一刻的，而批准只复核闸门 1–3）。`Runs.Do` 的结果类型由 `domain.LintResult` 泛化为"一次运行的结果"。发起时的那一次不经 `Do`（在写事务里，不能交给另一个 goroutine：`Do` 的 worker 用 `context.WithoutCancel(ctx)`，会把事务带进 goroutine），而经新的 `Runs.Hold(ctx) (release func(), error)`：同样的等待预算、只占名额、不开 goroutine，拿不到答 503；计算包在 `context.WithTimeout(ctx, LintTimeout)` 里（第 219 条，M10 移交的"名额/期限仍适用"）。

**闸门**（总体设计 5.8 的七条，加索引是否过时；照 lint 的口径）：

| 键 | 闸门 | 怎样算 | 从哪里读 |
|---|---|---|---|
| `agents` | 1. 根下有正文非空的 `AGENTS` | 根下标题键 `agents` 的节点是页，它的正文去掉 frontmatter 与 BOM、去掉首尾空白之后非空（同 `page/domain.AgentsBody` 的读法，`page/domain/instructions.go:34-44`） | page 的一次读（节点与正文） |
| `source_root` | 2. 来源根存在 | 提议的来源根是活着的页，且不是根下标题键 `agents` 的那一页（审查 B-Q2：agent 可以把原 `AGENTS` 改名、再把 `raw` 改名为 `AGENTS`，批准之后内置的说明就成了来源区的说明） | 同一次 page 的读。不过时（守卫不许删它，实际到不了）第 4–6 条与提示不算、记为不过：显式 lint 遇到不是活页的根会答错误（`page/adapter/postgres/lint_metadata.go:63-65`） |
| `reserved_titles` | 3. 根下没有占着保留标题的真实节点 | lint 读到的紧凑节点里 `ParentID == nil` 且 `NameKey` 是 `index`、`log`、`index.md`、`log.md`，页与附件都算；列出这些节点（至多 4 个） | `computeIn` 的同一份节点（`wiki/app/lint_ports.go:43-51`）；四个键由组合根从 page 模块根交（`page/domain/contract.go:88`，模块根新导出），不在 wiki 另写一份 |
| `frontmatter` | 4. 区外每页的 frontmatter 能解析，`type`、`summary` 不为空 | 规则 `invalid_frontmatter`、`missing_type`、`missing_summary` 的数都是 0。口径照 lint（`linking/app/lint_scope.go:219-237`）：`invalid_frontmatter` 收区外的每页（含来源根与根下 `AGENTS`）；后两条豁免来源根与根下 `AGENTS`，不豁免文件夹页 | 摘要 |
| `sources` | 5. 每个 `sources` 引用都指向来源区里存在的节点 | 规则 `invalid_sources` 的数是 0 | 摘要 |
| `links` | 6. 原有页的待建链接不多于升级前 | 逐页（主持者对审查 B-I2 的处置，第 180、188 条）：每个现在在来源区以外的基线页，它现在发出的不同的待建目标数不多于发起时记下的 `upgrade_nodes.wanted`。两头同一个数法：linking 的 `prepareWanted`（`linking/app/lint_edges.go:23-87`）按发出页走，同一页的 `seen` 就是这一页的数（`:44-63`），加一个按页的产出；它在 `generalEmitter` 的筛选之前数（来源也数：发起时在 `raw` 里、之后被移出来的基线页也有基线可比），目标的判定照 `wanted`（读作附件的、显式 LLM 契约下虚拟的不算，`linking/domain/lint_target.go:50-63`）。一页至多 10,000 条链接，没有全本的封顶问题。新建的页发出的待建链接不算（LLM Wiki 里待建是提示）。不过的页列出前 5 个与总数 | 发起时的基线行与这次的按页的数（`computeIn` 的同一份） |
| `preserved` | 7. 内容保全 | 每个基线节点（`baseline = true`）都活着；每个基线页现在的 `page_contents.content_hash` 等于账本的 `accepted_hash`；分母取 wiki 行的 `baseline_nodes`、`baseline_pages`，现读的基线行数不等于它们时这一闸不过并记一行 error（缺陷）。数出节点与页的"保全的/全部的"，列出前 5 个不符的节点 | page 的一条 SQL（基线行连 `nodes` 与 `page_contents`，按主键），不读正文 |
| `index` | 索引是新的 | `stale_pages == 0`（`wiki/app/lint.go:253-258`）；不过时提示服务器管理员运行 `nervewiki reindex` | 摘要 |

**第 7 条与护栏的关系**（第 191 条）：护栏逐步保证"基线页的正文只因系统的链接改写而变"，删除的护栏保证基线节点都在。闸门复核的是这两件事在提交那一刻仍然成立，账本是一条绊线（4.6）。它不只凭版本行的 `rewrite`：那一列合并时取"与"（`page/adapter/postgres/queries/changesets.sql:52-64`），`batch` 折进的改写也不标（4.6）。不读正文，所以大笔记本上也便宜。报告与 5.8 写"核对"，不写"证明"。

**提示**（不阻断）：`orphans`、`ambiguous`、`undigested_sources`、`wanted` 的条目，照 lint 的摘要（每条规则至多 5 项）。

**报告**（`UpgradeCheck`，第 5 节）：`computed_at`、`stale_pages`、`passed`、八个闸门（每个 `passed` 与它的数或节点）、八条规则的摘要（与 `getLint` 的每条规则同形，显式契约）、计数：来源区里的节点、移进来源区的基线节点、wiki 页（区外的页，不含来源根与根下 `AGENTS`）、新建的页、本次升级的变更集数（`changesets.upgrade_id`，部分索引）、`wanted` 的前后总数（发起时的与现在的，只作对比）。"移进来源区的基线节点"按基线行的 `parent_id` 在 Go 里重建发起时的树、算发起时提议的根的后代，再与现在区里的基线节点相减（审查 B-N8：已有的 `raw` 下的节点发起时就在区里，不算移进）。"新建的页"是这本笔记本里没有基线行的活页，SQL 用 `NOT EXISTS`。类型分布与每页的类型、摘要在目录视图里看，报告不重复（第 197 条）。

**存与读**（第 182 条，审查 C-I1）：检查不论过不过都答 200 与 `UpgradeCheck`。提交通过时把报告存进 wiki 的行，存的是另一个 schema `UpgradeReport`：与 `UpgradeCheck` 同样的闸门与计数，规则只有 `rule`、`count`、`count_capped` 与至多 5 项的条目，条目与规则都不带 `cursor`、`next_cursor`（契约里八个摘要与八种条目把它们列为必填，`additionalProperties: false`，所以不能拿 `UpgradeCheck` 去掉字段充数；留着游标则最坏 40 项 × 2,048 字节的游标加 8 个 `next_cursor` 约 98 KB，过了 65,536 字节）。显示的字符串截在 256 字节（M10 4.11）。审阅页读存下的那份；某条规则的"更多"从第一页读 `listUpgradeCheckItems`：审阅中对所有人冻结，现算的条目与提交时的一致，代价是每次"更多"占一个 lint 名额。

**条目的分页**：`listUpgradeCheckItems`（REST）与 `upgrade_verify` 带 `rule`（MCP）调 lint 的规则页的那一段（`computeIn` 的规则种类），范围是显式的 LLM Wiki 契约与提议的来源根；游标绑定显式的范围与根（M10 的 lint 游标），所以普通的 `lint` 的游标在这里不通用。

**MCP 的投影**（审查 C-N8）：闸门 3、6、7 的节点在 MCP 的报告里给路径（照 `wiki/adapter/mcp/lint_projection.go`），不在活的树里的（缺陷之外到不了）给 `null`。`Fit` 先截规则的条目（每条规则从后往前），再截闸门的节点列表，计数与 `passed` 不截。

**界与成本**：一次检查约是一次 lint 的摘要加两次 page 的读；M10 量过摘要在 10 万页上约 0.9 秒（M10 4.11）。期限是 `LintTimeout = min(10 s, request_timeout/2)`（`bootstrap/wire.go:249`）。成本守卫见第 9 节。

### 4.8 提交与退回

**提交**（REST `submitUpgrade` 与 MCP `upgrade_submit`，同一个用例；第 182 条与主持者的方向"先置审阅再算"）：

1. 第一个事务：锁（4.3），判定 `notebook.change_mode`，状态是 `upgrading`，或 `reviewing` 而没有报告（上一次提交没有做完，例如进程在两次事务之间停了），否则 409 `wiki.upgrade_state`；写 `reviewing` 与新的 `submission_id`、`submitted_by_id`、`submitted_at`；`Publish`；提交。此后发起人的 agent 也写不进来，提交的计算看到的就是提交的那一刻，不需要另找"写入的水位"。
2. 从这里起脱离请求的上下文（第 215 条）：`context.WithTimeout(context.WithoutCancel(ctx), LintTimeout + 10 s)`，照 transfer 写任务结束的做法（`transfer/app/job_run.go:186`）。MCP 客户端常有自己的工具超时，10 万页的笔记本上它可能在计算中断开；不脱离，第二个事务就开不起来（`platform/postgres/tx.go:153` 的 `begin(ctx, opts)`），行留在"审阅中、没有报告"，agent 自己出不来。
3. 在快照里经 `Runs.Do`、以提交的键算报告（4.7）。
4. 第二个事务：锁；行仍是这次提交（同一个 `submission_id`）、仍是 `reviewing`、还没有报告，否则什么也不写、答 409 `wiki.upgrade_state`（期间被退回、放弃或被后一次提交取代）。
   - 报告通过：存下（`UpgradeReport`），`Publish`（第 216 条），答 200 `Upgrade`（MCP 答 `{state, submitted_at}`）。
   - `stale_pages > 0`：退回 `upgrading`、清掉提交的三列，`Publish`，答 409 `wiki.upgrade_stale_index`。
   - 别的闸门不过：同样退回，答 409 `wiki.upgrade_not_ready`，不带报告（不扩 `Problem` 的成员，第 182 条），`detail` 写不过的闸门的键。
   - 第 3 步答 503 或出错：同样退回，答那个错误。
5. 第二个事务本身失败（进程崩溃、库断开），行留在 `reviewing`、没有报告：批准答 `wiki.upgrade_state`；任何管理员可以退回，发起人的 agent 或任何管理员可以再提交一次（第 1 步认这种状态），都有测试。

**退回**（`sendBackUpgrade`）：`reviewing`（有没有报告都行）→ `upgrading`，清掉提交的三列与报告；v0.1 不附说明（第 183 条）：管理员在自己的客户端里告诉 agent。

**网页上提交不过之后**（审查 C-Q4）：升级页收到 409 `wiki.upgrade_not_ready` 或 `wiki.upgrade_stale_index` 后自动读一次 `checkUpgrade`，显示不过的闸门（等于点了"检查"，再占一个 lint 名额）。

### 4.9 批准

`approveUpgrade`，一个事务：

1. 锁（4.3），判定，状态是 `reviewing` 且有报告，否则 409 `wiki.upgrade_state`。
2. 锁下再核便宜的闸门：提议的来源根是活着的页且不是根下的 `AGENTS`、根下有正文非空的 `AGENTS` 页、根下没有占着保留标题的节点。审阅中对所有人冻结，它们不会变；不过时答 409 `wiki.upgrade_not_ready`（缺陷之外到不了）。
3. `notebook.NewUpgradeStates` 的 `Approve`：一条 `UPDATE` 写 `mode = 'llm_wiki'`、`source_root_id`、清掉四列、审计列（`SetNotebookMode` 加列，`notebooks.sql:42-47`）。
4. `page.NewSourceRebuild` 的 `Rebuild(notebook, root)`：`ForgetSources`（`page/adapter/postgres/queries/sources.sql:59-63`），再为根的严格活后代写上哈希：页取 `page_contents.content_hash`（字节数大于 0 的），附件取 SHA-256，分批经 `SetSourceHashes`（`:46-54`）。附件没有文件行（`AttachmentInfos.Of` 不答它，`page/app/ports.go:592-598`）时同单元的做法报错（`page/app/unit_sources.go:71-72`），批准 500、整个回滚，不跳过（审查 B-N6）。M10 的不变式要模式与哈希在同一个事务里成立（`bootstrap/interleavings_page_test.go:52-55` 的注释）。
5. page 删掉这次升级的 `upgrade_nodes` 的行。
6. wiki 的行改为 `approved`，审计列记批准人与时刻。
7. `Publish`（`From standard`、`To llm_wiki`），在重建之后（审查 B-N3）；提交；日志。答 200 `Upgrade`（`approved`）。

**未消化的来源**（第 202 条）：M10 的"已消化"由当前的边推导（`linking/app/lint_sources.go:24-60`：wiki 页的正文链接或 `sources` 引用到来源，就算消化），不看 ingest 的标记；ingest 的标记只在 `llm_wiki` 下、只对 `edit` 的变更集记（`linking/app/ingest.go:46-48`），升级中的写都不会标。所以批准之后，已被 wiki 页引用的来源算已消化，没有被引用的照实报为未消化，健康度与 `pending_sources` 列出它们，agent 之后照 ingest 的 playbook 处理。不给升级的变更集补 ingest 标记（它们不是一次 ingest）；报告的提示里有未消化的数。

**来源区里的重复**（第 204 条）：升级中移进来源区的来源不查重，批准时也不查：M10 的不变式允许重复（索引不唯一，系统的改写本来就能让两个来源相同，M10 4.8），护栏又不许删除原有节点，查重只会逼出删不掉的东西。

**之后**：笔记本是 LLM Wiki，M10 的契约生效（来源只读、`AGENTS` 与来源根受保护、虚拟页）。不提供整体撤销（负责人第 3 条）。

### 4.10 驳回、放弃与回滚

**开始**（`rejectUpgrade`、`abandonUpgrade`）：一个事务：锁，判定，状态（4.3），写 `rolling_back` 与结局，`Publish`（在开任务之前，缩短持队列锁的时间），再经 transfer 的 `Rollbacks().Start` 在这个事务里建回滚任务的行并投递（`transfer/app/start_background.go:154-198` 的 `startBackground`：嵌套的 `WithinTx` 加入调用方的事务，重入笔记本行的锁；种类 `rollback`、`upgrade_id`、客户端是请求的；`MaxQueued` 满时 503 `server_busy`），记下 `rollback_job_id`。`Rollbacks().Start` 不记日志（`startBackground` 在嵌套的事务返回、外层提交之前就记"queued"，`:192`，外层随后失败时不实，审查 A-N6），由 wiki 在提交之后记。答 200 `Upgrade`。回滚的任务由 REST 开，客户端是 `web` 或 `api`，不必放开 `transfer_jobs.client`（M8 的移交第 5 条不适用）。

**回滚按基线整体复原**（第 178 条，核实可行），每一遍两段：

- **先删新建的子树**（审查 A-I2 的第一条建议）：一棵不含基线节点的新建子树（根是没有基线行的活节点，子树里没有基线节点，取极大的）的删除不需要调度，在计划之外按单元删：每个单元一棵子树、至多 `MaxUnitNodes = 10,000` 个节点（`page/domain/trash.go:57`），更大的从下往上删（先删它下面不超过界线的子树，再删它自己）。它们不计入下面的界，删的数进报告的 `deleted`。发起时补写的 `raw`、`AGENTS` 与 agent 新建的综合页都在这里删掉（进回收站，第 209 条）。
- **再计划剩下的**：`PlanRevert` 是 `page/domain` 的纯函数，输入是 `RevertState{Items, Versions, Nodes, Contents, Hashes, Groups, Tree, Lines, At, Retention}`（`page/domain/revert.go:110-121`、`:373`），`Items` 是 `{NodeID, Before, After *TreeState, Restored, Revision}`（`page/domain/changeset.go:192-198`），`Versions` 是 `{NodeID, Base *int, BaseBytes, Rewrite, Revision, Hash}`（`:204-211`）。合成的"变更集 C"：每个位置、名称或次序值与基线不同的基线节点一条（`Before` 是基线的，`After` 是现在的）；每个收着基线节点的新建节点一条（`Before == nil`，删除）；每个哈希与基线不同的基线页一个版本（`Base` 是基线的版本号，`Revision`、`Hash` 是现在的）；`Hashes` 里基线版本的哈希取自基线的行。合成的条目的 `Revision` 都是 0、`Restored` 为空（不然 `left()` 对没有版本的移动页调 `sameVersion`，`revert.go:463-464`，要 `Hashes` 里有现在的版本号的哈希，没有就记 `missing`、停在 `page.changeset_trimmed`，审查 A-N2）；合成的版本 `Rewrite = false`。基线节点不会被删（护栏，P1/S1 起），所以计划里没有从回收站的恢复，只有放回、删除与写回；`schedule`（`page/domain/revert_schedule.go:304`）照它的轮与三条后备排步（交换名称、移进新建的文件夹、改名之后新建同名，M9/P3 测过的形状），`ErrRevertUnordered` 是缺陷（500），不重试（M9 的移交第 3 条）。合成的 C 比 M9/P3 测过的任何真实变更集都大、形状都杂，第 9 节加大的随机形状的性质测试。升级中新建又删掉的节点不在合成的条目里（净效果为零），留在回收站。
- **基线页的版本不会被清理**：清理只删 `deleted_at` 早于期限的版本与条目（`page/adapter/postgres/queries/purge.sql:23-36`），版本随节点的删除才标 `deleted_at`（`page/adapter/postgres/queries/nodes.sql:221-228`）；基线节点在升级中删不掉，所以发起时的版本行一直在。基线的版本号就是那时的 `page_contents.revision`：发起时没有活着的编辑会话（第 174 条）、没有写入任务，那一行不会再被并入（`changesets.sql:52-64` 的 `ON CONFLICT … WHERE`）。写回经 `History.VersionContents`（`page/app/ports.go:584`）按（页、版本号）读。逐个撤销时"凡碰过新建又删掉的节点的变更集都被截掉"（调查 A 的 D7）的问题因此不存在。
- **app 里换掉的四处**：后台撤销今天以变更集为键读：`batchChangeset`（`page/app/unit_revert.go:634`）读条目、版本与 `trimmed`，`revertContents`（`page/app/jobs.go:872`）按变更集的版本读要写回的正文，`PlanRevert`（`:342`）读变更集，单元的 `UnitSpec` 是 `revertUnit`（`:100`）。回滚加平行的一支：`JobWrites.PlanRollback`（含先删的一段）、`RunBatch` 的回滚分支、`Unit.RollbackBatch`，以基线的行与现在的状态合成同样的 `RevertState`；其余照用：`PlanRevert`、`revertSteps` 的分段（`page/app/jobs.go:378`）、`CheckRevertPart` 与 `CheckRevertSteps`、`place`、`Delete`、`WriteContent`。
- **单元**：`UnitSpec{Kind: rollback, UpgradeOwn: 升级的 id, Changeset: 任务的, Create: true, Tree: true, Background: true, Action: notebook.change_mode, Options: Options{UpdateLinks: false}}`，执行者是任务的发起人（`transfer/app/background.go` 的 `shared.Actor{UserID: job.CreatedBy, JobID}`）。`UpdateLinks` 为假：放回与改回名不让参与者改写链接（`linking/app/rewrite.go:63`），基线页的链接文字由正文的写回复原；设成真时参与者会改写基线页，同一遍的写回以过期的 `Base` 撞上 `ErrRevisionMismatch`，`unit_revert.go:129-133` 把它当缺陷。冻结的守卫按 `UpgradeOwn` 放过它们（4.5）；模式的守卫在普通笔记本里直接放过；没有会话，锁的守卫也放过。发起人之后不再是管理员时，下一批答 `forbidden`，任务停下，另一位管理员继续。
- **变更集**：类型 `rollback`（第 192 条），`upgrade_id` 是这次升级的，说明是固定的英文"Upgrade rollback"，先删的一段与计划的批都进它。它是普通的变更集，之后可以在网页上撤销（等于重做 agent 的整理）。主持者第 179 条写"类型不变"，对 agent 的写成立（照旧是 `edit`，ingest 只认 `edit`）；回滚的写不能用 `revert`（`changesets_reverts_check` 要它有 `reverts_id`，`00034_page_changesets_history.sql:9`），所以另加一种：这是对第 179 条的一处补充，写明。
- **分批**：照 M9/P3 的切法（`transfer/app/background.go:140-236`：段不切开、过大减半、503 重试）。transfer 的 `backgroundRun.run`（`:86`）加 `rollback` 的一支：按"遍"做——先删、计划、跑完这一遍的批、再来，直到差为空，然后结束（下面）；一遍什么也没写而差不空是缺陷（任务以 `internal` 停下）。
- **界**（第 193 条）：先删之后，一遍的树的条目一次计划，至多 `MaxRollbackItems = 300,000`（`page/domain`）；正文的写回超过时分到后面的遍。剩下的树的条目是位置变了的基线节点（不超过基线节点数）与收着基线节点的新建节点（每个都是某个基线节点的祖先，深度至多 `MaxDepth = 10`，`page/domain/node.go:23`，实际是 agent 建的分类文件夹）：它们都是这本笔记本先删之后的活节点，所以条目数不超过笔记本的活节点数。v0.1 的规模是 30 万节点，规模之内的笔记本到不了界；只有超过 v0.1 规模的笔记本才会以 `revert_too_large` 停下，这时有下面的"结束回滚"。M8 的 `MaxJobItems = 60,000`（`page/domain/jobs.go:29`）是一个撤销任务的界，回滚的界另设，计划的内存与时长在 10 万个条目上有成本守卫，30 万的进 M12 的性能移交。
- **停下与继续**：任务停下（取消、重启记作 `interrupted`、River 的任务超时 `transfer.job_timeout` 记作 `timeout`，`platform/config/config.go:252-254`、`transfer/app/job_run.go:233-235`；界、意外的冲突）时，已做的批留着，升级留在 `rolling_back`，网页显示任务的报告、"继续回滚"与"结束回滚"；继续就是 `abandonUpgrade` 在停下的 `rolling_back` 上开一个新任务（第 200 条），它重新算差，超时停下的大回滚因此靠几次继续做完。合成的计划里没有恢复，所以没有 M8 的移交第 2 条的"停在一组中间"。
- **确定性的停下**：`ErrRevertUnordered`（缺陷）、一段放不进一批（`transfer/app/background.go:166-175` 的 `errSegmentTooLarge`）、界，继续会再停在同一处。冻结不设时限、任何管理员随时可以放弃（负责人第 1 条），没有出口笔记本就永久只读；所以有"结束回滚"（主持者第 212 条）。
- **结束回滚**（`endUpgradeRollback`，第 217 条）：一个事务：锁，判定，状态是 `rolling_back` 否则 409 `wiki.upgrade_state`，回滚的任务在排队或运行 409 `transfer.busy`；page 数出剩下的差（基线节点不在原位的数、正文与基线不同的基线页数、没有基线行的活节点数，与 `getUpgrade` 显示的同一条读）；清掉四列、删掉基线的行、wiki 的行改为 `rolled_back`，结局不改，`rollback_complete = false` 与三个数；`Publish`。答 200 `Upgrade`。护栏保证了原有页的正文与节点都在，保留现状不丢内容，只是结构没有全部放回；历史里有升级与回滚的每个变更集，可以逐个撤销。
- **做完**（`wiki.NewRollbackEnd`，transfer 的 `Deps.RollbackEnd`）：任务的最后一遍之后，在 `jobRun.finish` 的 `succeed` 里一个事务（审查 A-N1）：先笔记本行 `FOR NO KEY UPDATE`（笔记本的删除先锁笔记本行再改任务行，`transfer/adapter/postgres/queries/jobs.sql:149-152`；导出的 `succeed` 为此同样先锁笔记本行，`transfer/app/export.go:394-410`）；升级仍是这一次、仍是 `rolling_back`；page 核对差为空；清掉四列、删掉基线的行、wiki 的行改为 `rolled_back`、`rollback_complete = true`、`Publish`；再 `FinishJob`。差不空时答错误，任务以 `internal` 停下，升级留着，可以继续或结束。

M8、M9 两份移交里"逐个倒序撤销、按 `Kind == revert` 放行、停在一组中间的处理"随第 177、178 条不再适用，逐项写在第 7 节"移交的落实"。

### 4.11 回收站里的保留标题（M10 收尾 A-M1，第 181 条）

批准之前删掉的根下 `index`，批准之后撤销那次删除会把它带回 LLM Wiki 的根：撤销里的恢复以计划给的状态 `bringBack`，名称照旧、不加序号（`page/app/unit_revert.go:119-126`），`CheckContract` 只对建、改名、移动判保留的标题（`page/domain/contract.go:137`）。改：`CheckContract` 对 `OpRestore && s.Kind == ChangesetRevert` 同样判。只限撤销：回收站的恢复与后台的恢复在单元里给根下保留的键加序号（`page/app/unit_restore.go:83-90`、`:324-327`），但 LLM Wiki 里它们在单元之前有一道预检，用的是原来的名字（`page/app/restore_node.go:110-117`、`page/app/jobs.go:146-150` 与 `:246-247` 经 `checkRestoreSources`，`page/app/source_restore_precheck.go:51-58` 造的 facts 是 `OpRestore`、`ChangesetTrashRestore`、根下键 `index`）：不分种类地判会让这两条恢复答一个没声明的 `page.virtual_file`（审查 A-I1 的探针）。撤销的 facts 是 `ChangesetRevert`（`page/domain/revert_facts.go:74`；单元里的步经 `contractFacts` 取 `Write.Kind`，`page/app/source_facts.go:48-49`）。撤销里的恢复与撤销里的改名、移动回到根下的保留键整体拒绝，答 `page.virtual_file`。`revertChangeset`（`api/modules/page.yaml:1083-1098`）与 `startRevert`（`api/modules/transfer.yaml:281-298`）今天都没声明它，P3/S1 声明，并各在模块的 `adapter/http` 测试里答出（13.1 第 8 条）。`restoreNode`、`startTrashRestore` 不声明；第 9 节有它们恢复同一项答 200、名为 `index 2` 的正例。不在批准时改回收站里的名字（那是绕过单元的写）。

### 4.12 事件（第 185 条）

- 发起、提交（两个事务）、退回、放弃、驳回、批准、结束回滚、回滚做完，都经 `notebook.NewUpgradeStates` 的 `Publish` 调订阅链（`notebook/app/extension.go:75-101` 的 `ModeChange`、`ModeSubscriber`、`publishMode`），组合根唯一的注册者发 `mode`（`bootstrap/events_registrants.go:59-66`）。状态变了而模式没变时 `From == To`；存下报告时状态不变，也发（第 216 条）。
- `EventMode`（`api/modules/events.yaml:222-233`）不加字段；`mode` 的说明（`:48-51`）改为"一本笔记本的模式或它的升级变了"，`EventMode.description`（`:223`，"a change of a notebook's mode"）一并改（审查 C-N5）。
- 网页：`modeChanged` 今天重读这个工作区的笔记本列表（`web/…/events/handlers.ts:277-279`），横幅与只读随 `Notebook.upgrade` 变；它另外重读这本笔记本的升级（SWR 键 `["upgrade", notebook_id]`，`EventMode` 带 `notebook_id`），升级页的报告、回滚的进度由此更新（第 216 条）。
- [M5 的事件移交](handoffs/M5-events.md)：类型 `mode` 已由 M10 建（`TestTheModeEventReachesTheStreams`，`bootstrap/wiki_downgrade_test.go:88`）；M11 的整个程序测试 `TestUpgradeStatesReachTheStreams` 证明每个转移都到流。

### 4.13 MCP：工具、prompt 与说明

**工具**（第 196 条：两个入口都登记，`mcpserver.BothEntries`，`platform/httpserver/mcpserver/tool.go:25`；同 `lint`、`pending_sources`）：

- `upgrade_verify`：参数 `rule?`、`limit?`、`cursor?`（账户入口另有 `notebook`）。没有 `rule` 时答报告（4.7 的 MCP 投影），有 `rule` 时答这条规则在显式范围里的条目与 `next_cursor`；给了 `limit` 或 `cursor` 而没有 `rule` 答 `validation_failed`（同 `lint`，`wiki/app/lint.go:63-72`）。注解 `readOnlyHint: true`、`idempotentHint: true`。
- `upgrade_submit`：没有参数（账户入口另有 `notebook`）。答 `{state: "reviewing", submitted_at}`；不过时是工具的错误，码同 REST。注解 `readOnlyHint: false`、`destructiveHint: false`、`idempotentHint: false`。

两个工具照 M9 的每个工具：先写 `api/mcp/tools.json`（`TestTheToolListsAreTheContract`，`bootstrap/mcp_tools_test.go:156`）、矩阵有行（`TestThePermissionMatrixCoversEveryTool`，`bootstrap/permission_matrix_mcp_test.go:438`）、码进 `mcpToolCodes()`（`:89-117`，每个笔记本工具的行都有 `bad_request`、`validation_failed`、`notebook.not_found`，第 5 节写全）、答复经 `Fit`（`platform/httpserver/mcpserver/answer.go:127`）至多 48 KiB（`:17`）。`TestTheWikiToolsContract` 写死的 16、18（`bootstrap/mcp_tools_test.go:347-349`）改为 18、20。登记在 wiki 的 `Tools()`（`wiki/tools.go`），组合根的 `mcpTools` 已经拼它（`bootstrap/mcp.go:47-48`）。

**内置的说明**：`pathsAndTools`（`page/adapter/mcp/instructions.go:30-55`，两个入口共用）加两行：

```
- upgrade_verify: an upgrading notebook's LLM Wiki checks, or one rule's items with next_cursor.
- upgrade_submit: submit the upgrade for an admin's review once every check passes.
```

原文 181 字节，SDK 的编码把两个换行各算 2 字节，是 183（审查 C-N2 实测）。今天 SDK 的编码下笔记本入口 3093、账户入口 3163 字节，之后 3276、3346，距 4096 各剩 820、750，过 `TestTheBuiltInInstructionsAreShort` 的 300 字节余量（`page/adapter/mcp/instructions_test.go:68`）。S2 照 SDK 实测写进记录。

**"upgrade"一词**：M10 写给 M11 的"说明里不用 upgrade 一词（它指服务端的升级）"（`docs/v0.1/M10-llm-wiki/00-M10-design.md:624`）与工具名冲突：`TestTheBuiltInInstructionsNameEveryTool`（`bootstrap/mcp_tools_test.go:193`）要说明里有 `- upgrade_verify:` 一行。改为反过来：服务端的升级在工具说明与契约里改称"server update"：`page/adapter/mcp/tools.go:272`、`:282` 的"an upgrade"，`linking/adapter/mcp/tools.go:64` 的"after an upgrade"，`api/modules/events.yaml:35` 的"after an upgrade"；`tools.json` 随之改（三个工具 × 两个入口，六处）。内置的说明今天没有这个词（`page/adapter/mcp/instructions.go` 里找不到 `upgrade`）。`events.yaml:49` 的"a notebook upgraded"指的是 M11 的升级，随 `mode` 的说明一起改（4.12）。

**prompt `upgrade`**：在 wiki 的 `Prompts()`（`wiki/prompts.go:9`），`Entry: NotebookEntry`、静态，先写 `api/mcp/prompts.json` 的笔记本一项；没有参数。处理函数是新的用例 `UpgradePlaybook.Execute`：actor → 判定 `lint.read`（看不到答 `notebook.not_found`，同其余的 prompt，`wiki/app/playbooks.go:28-46`）→ 状态是 `upgrading` 才答 playbook，别的（没有升级、审阅中、回滚中、LLM Wiki）答 `wiki.upgrade_state`。它不经 `Playbooks.Execute`：那一条对不是 `llm_wiki` 的笔记本答 `notebook.not_llm_wiki`，方向相反（M10 收尾的移交）。playbook 是 `wiki/domain/playbook-upgrade.md`，英文（第 176 条），至多 `MaxPlaybookBytes = 16 KiB`（`wiki/domain/playbook.go:21`），内容：只有发起人的 agent 能写，`notebook.upgrading` 在升级中的意思是"你不是发起人，或已不是这本笔记本的管理员"；先 `upgrade_verify`；把原有页归为来源或 wiki 页；来源移进来源根（正文不改）；wiki 页只改 frontmatter（`type`、`summary`、`sources`），文件夹页也要；无效的 frontmatter 就地改（4.6）；`AGENTS` 已有时改名保留、另建；根下的保留标题改名；不删原有节点；移动与改名不要传 `update_links: false`（系统的链接改写是放行的，`page.upgrade_keep_links`）；`batch` 里先写页、后移动（`page.upgrade_batch_rewrite`）；新建页的标题会让原有页的短链接改指它（第 213 条）；反复 `upgrade_verify` 直到全过，再 `upgrade_submit`；每个码的意思。

**prompt 的测试**（审查 C-M3）：`TestThePromptReadsLLMWikiMatrix`（`bootstrap/permission_matrix_mcp_wiki_test.go:153-216`）今天对每个 prompt 只跑两种模式，并断言 `len(prompts) != 3` 时失败（`:205`）；`TestThePromptListsAreTheContract` 写死 `len(want["notebook"]) != 3`（`bootstrap/mcp_prompts_test.go:105`）。改为按"prompt × 状态"的期望表：`llm_wiki`、没有升级的 `standard`、`upgrading`、`reviewing`（`ingest`、`query`、`lint` 在后三种答 `notebook.not_llm_wiki`；`upgrade` 只在 `upgrading` 答文字，其余答 `wiki.upgrade_state`），状态数与 prompt 数的断言随之改；变异"prompt 在审阅中也答"由 `reviewing` 一格抓住。这部分放进第七份的 `TestUpgradePromptMatrix`（第 222 条），原测试只改 prompt 数。

**不加的**（第 196 条）：`notebooks`、`open_notebook` 的每项不加升级的状态；握手的说明不加"正在升级"的系统段（说明的缓存按笔记本、`AGENTS` 节点与版本号，加状态要改缓存的键）。agent 从 README 写的地址、prompt 名、`upgrade_verify` 与写的错误得知升级。

### 4.14 权限与矩阵（第 195 条）

- 十个 REST 操作与两个工具判 `notebook.change_mode`（笔记本管理员，`access/domain/rules.go:80`）：看不到 404 `notebook.not_found`，不是管理员 403 `forbidden`。读升级（`getUpgrade`、检查与条目）也只给管理员：升级页是只给管理员的读（13.2 第 18 条），横幅从 `Notebook.upgrade` 得到状态，所有能读笔记本的人都看得到。
- prompt 判 `lint.read`（同其余的 prompt）：它只是文字，真正的检查在工具里。
- 回滚的任务的取消要笔记本管理员（第 218 条）。
- 不新增动作：`wiki.Actions()` 照旧只有 `lint.read`（`wiki/domain/lint.go:144-147`）。
- 矩阵（审查 C-M4、C-M6）：13 个笔记本列今天落在五本笔记本上（`bootstrap/permission_matrix_notebook_test.go:166-178` 的 `notebookOf`）；不能把它们设成升级中（那几本上已有的每一行写都会改答 `notebook.upgrading`）。照 M10 的 `wikiSeeded`（`bootstrap/permission_matrix_wiki_test.go:30-45`），在单独的工作区建一组"升级的变体"：五本 × 三种状态（`upgrading` 且闸门能过；`reviewing` 且有存下的报告；`rolling_back` 且回滚的任务已停下），以 SQL 种下（升级列、基线行、wiki 的行、停下的任务行），`startUpgrade` 用五本没有升级的普通笔记本。十个 REST 操作、两个工具与 prompt 的行在新的 `TestUpgradePermissionMatrix`、`TestUpgradeToolsMatrix`、`TestUpgradePromptMatrix` 里（第七份，第 222 条）；覆盖的检查 `TestThePermissionMatrixCoversEveryOperation`、`TestThePermissionMatrixCoversEveryTool` 读到它们的行。写操作每格一个库的副本加一个应用，照现有的做法。

### 4.15 网页

**只读收成一处**：`app/effective-role.ts` 加 `writesNotebook(notebook)` = `writesPages(notebook.role) && notebook.upgrade === null`（`writesPages` 在 `:40-42`）。今天直接以 `writesPages(notebook.role)` 判的 14 个文件（`pages/notebook/notebook-home.tsx`、`changeset-page.tsx`、`transfer-page.tsx`、`page-tree-item.tsx`、`page-tree.tsx`、`trash-row.tsx`、`trash-page.tsx`、`trash-preview.tsx`；`pages/page/upload-rows.tsx`、`page-layout.tsx`、`attachments-section.tsx`、`attachment-drop.tsx`、`unresolved-link.tsx`；`app/source-permissions.ts`）都换成它；`writesPages` 之后只由 `writesNotebook` 用。发起人的网页同样只读（只有他的 MCP 能写）。**控件表**（每项一条 vitest，照 M10 4.16 来源页那一格）：笔记本首页的新建；页面树的新建、改名、移动、拖放、排序、删除；页面的编辑与 `Ctrl+E`；阅读视图的任务勾选（`canEditPageBody`）；附件一节的上传、拖入、改名、删除；附件一节与编辑器上传行的重试（含来源标题表单，`pages/page/upload-rows.tsx:48-51` 的 `canRetry`、`:177`；编辑器里同一个组件，`pages/page/editor-uploads.tsx:30`，审查 C-M9）；未建链接的新建（`unresolved-link.tsx` 的 `Why`，`:33`，加 `upgrading` 一种与它的文字）；版本页的还原；历史的撤销；回收站的恢复（行与预览）；导入。

**编辑器失去编辑**：`stores/edit-session.ts:26-32` 的 `EditLost` 加 `{ reason: "upgrading" }`；`lostBy`（`:63-68`）认 409 `notebook.upgrading`（保存与开启都可能答它：会话过了期重开时被否决）；照 `source` 先 `end()`（`:180`）；`EditLostBanner` 的 `switch`（`pages/page/edit-lost-banner.tsx:47`）加它的文字（tsc 强制）。不认它时编辑器每次停顿再存再被拒（同 M10 修好之前的 `source_immutable`）。

**横幅**：`NotebookLayout` 在 `<Outlet>` 之前（`pages/notebook/notebook-layout.tsx:76`）放 `UpgradeBanner`，`role="status"`（不用 `alert`：换页时不被反复读出），不抢焦点（13.2 第 26 条）。按状态：`upgrading`"正在升级为 LLM Wiki，暂时只读"；`reviewing`"已提交，等待管理员审阅，暂时只读"；`rolling_back`"正在撤销这次升级，暂时只读"。管理员另有"查看升级"的链接与"任何笔记本管理员都可以放弃这次升级"；在升级页上不显示这个链接（审查 C-N6）。横幅不显示 MCP 地址与 prompt 名（第 184 条）。横幅出现时在它下面的内容往下移一个横幅的高，靠的是浏览器的滚动锚定，同正文里的公式、图表变高（`pages/workspace/workspace-layout.tsx:107-113` 的注释）；WebKit 上有没有这一跳没有实测（第 224 条）。

**发起**：设置"模式"一节（M10 加的降级，`pages/notebook/general-page.tsx:128-169`）对普通笔记本、管理员、没有升级时加"升级为 LLM Wiki…"按钮，`ConfirmDialog` 的 `held` 写法与 `tone="default"`（同降级，`:156-166`），对话框写后果（冻结到提交或放弃、补写的两页、原有页的正文不改）；根下已有标题为 `raw` 的页而它下面有节点时，对话框另写"根下的 `raw` 将成为来源区（现有 N 个节点），批准之后它下面的都是只读的来源；不是来源的可以由 agent 移出"，N 从页面树已读到的 `listNodes`（`api/modules/page.yaml:12`，整棵树）数出（主持者第 214 条）。模板的语言取界面语言。拒绝的文字：`wiki.upgrade_pages_locked` 列出 `locks` 的页与编辑者，`app/pages-locked.tsx:19` 今天只认 `linking.pages_locked`，放宽到新码（审查 C-N3；`lockedText`，`app/problem-messages.ts:137`，只读单个 `lock`，不合用），并提示可以强制解锁；`transfer.busy` 用 `transfer.writingBusy`；其余照 `problemMessages`。成功之后去升级页、焦点给它的标题。有升级时这一节写状态与"查看升级"。

**升级页**：外壳下自己的一段 `upgrade`（与 `health` 并列，`app/routes.tsx:194-200`），`/:slug/notebooks/:id/upgrade`，调 `useDocumentTitle`。只给管理员的读（13.2 第 18 条）：非管理员不发读、只说明谁能看；导航不加入口，入口是横幅与设置的"模式"一节；403 时重读 `["notebooks", ws]`（`useFollowRole` 重读的是工作区，`app/follow-role.ts:11`，不合用，同 M10 4.5）；404 `wiki.upgrade_not_found`"没有进行中的升级"。SWR 键 `["upgrade", notebook_id]`，`mode` 事件重读它（4.12）。各节：
- 状态（谁、何时发起，提议的来源根与补写的页的链接）。
- 接入 agent（第 184 条，审查 C-M12）：对发起人（仍是管理员）写 prompt 的名字 `upgrade` 与"照 README 的'接入 agent（MCP）'一节把 agent 连到这本笔记本"（README 的升级一节写明笔记本入口 `/mcp?notebook=<id>`，id 在网页地址 `/notebooks/` 之后，`README.md:259`）；对其他管理员写"只有发起人（显示名）的 agent 能写；你可以检查、提交、放弃"。
- 检查（P2/S3）："检查"按钮读 `checkUpgrade`，列八个闸门与规则的卡；卡的"更多"读 `listUpgradeCheckItems`。`HealthItem` 与 `healthRetry`（`pages/notebook/health-request.ts:40`，`server_busy` 不自动重试）照用；`HealthRule`（`pages/notebook/health-rule.tsx:48`）把"读一页"与"模式的核对"做成参数后复用。"提交审阅"按钮（`upgrading`）；提交答 409 之后自动读一次检查（4.8）。
- 审阅（P3/S3）：`reviewing` 读存下的报告（`UpgradeReport`），规则的"更多"从第一页读 `listUpgradeCheckItems`（4.7）；没有报告时写"正在核对，或上一次提交没有做完"并给"退回"。"批准""退回继续修改"（`default`）、"驳回"（`destructive`）。批准之后去笔记本首页、焦点给标题。
- 回滚：`rolling_back` 显示任务的进度与报告（复用 `transfer-report.tsx`）；停下时"继续回滚"与"结束回滚，保留现在的样子"（`destructive`，对话框写剩下的三个数：不在原位的节点、没有写回的页、还在的新建节点，来自 `Upgrade.rollback_left`）。
- "放弃这次升级"（`destructive`，不要求输入名称：回滚本身也能撤销），`upgrading`、`reviewing` 与停下的 `rolling_back` 都有（后者就是继续）。
对话框都用 `held` 与 `onClosed` 的焦点规则（`general-page.tsx:138-140`）。

**回滚任务在网页上的报告**（审查 C-M10）：`pages/notebook/transfer-report.tsx` 的 `shownCounts`（`:122-127`，`Record<string, …>`，tsc 不强制）加 `rollback: ["deleted", "put_back", "written"]`，不然 `:202` 退回导入的计数；`kindFailures`（`:54`）加 `rollback` 的失败文字（`forbidden`"开始回滚的人已不是管理员，另一位管理员可以继续"、`revert_too_large`"这次升级改动太多，回滚停下了：可以结束回滚、保留现在的样子"、`internal`、`interrupted`、`timeout`"可以继续回滚"）；`countText`（`:130-133`）的 `skipped` 不涉及回滚。假服务端 `test/jobs-server.ts:10` 有一份自己的 `writingKinds`，与 `stores/transfer.store.ts:27` 一起加 `rollback`（05-lessons 第 2 节：替身要在真东西会失败的地方失败），P1/S3 带。

**store 与 service**：动作答 `Upgrade`（第 201 条），放在 `NotebookStore`（经 `inTurn`，`stores/notebook.store.ts:45`）：成功之后重读笔记本列表与这本笔记本的升级，403、404、状态不对时照 `downgrade` 重读列表（`:114-143`）；service 是新的 `services/upgrade.service.ts`（十个操作与读），只在网页 Step 里加（knip）。服务端 Step 只带：码的两种语言的文字（`app/problem-messages.ts:11-74`，`problem-messages.test.ts` 读契约核对，`:43-62`）、假服务端的 `upgrade: null`（`test/fakes.ts:91` 的 `notebookJSON` 与各测试里的 `Notebook` 字面量，tsc 逐个报出）、新种类的文字（变更集的 `kindText`，`pages/notebook/changeset-text.ts:14-27`；任务的 `jobTitle` 与 `changesetKinds`，`pages/notebook/transfer-names.ts:9-34`；两份 `writingKinds`）。

**输入法**：没有新的文字输入框（退回不附说明，第 183 条），输入法清单不加步。

### 4.16 码的次序、加锁与交错

- **码的次序**（第 5 节逐个操作写出）：取值的 422 → 看不到 404 → 不是管理员 403 → 模式与状态的 409 → 资源占用的 409（`transfer.busy`、编辑会话）→ 内容的 409 → 503。例外：`listUpgradeCheckItems` 照 `listLintItems`（`api/modules/wiki.yaml:63-71`；lint 在模式之后才判 `PageSize`）把 `limit` 的 422 放在 `wiki.upgrade_state` 之后，游标的 400 在最前（审查 C-N4）；`startUpgrade` 的 503 在内容的 409 之间（名额在写之前占，`wiki.upgrade_stale_index` 要算了才知道）。写入单元里守卫的码在领域的码之后（4.5）。同一操作经 REST 与 MCP 答同样的码与次序（13.4 第 3 条的"三种入口的次序"）。
- **加锁**：所有转移在工作区 `FOR SHARE` → 笔记本 `FOR NO KEY UPDATE` 之下；单元取笔记本行的 `FOR SHARE` 或 `FOR NO KEY UPDATE`（`page/app/unit.go:244-256`），所以转移与写入单元串行，一个单元里状态不变。发起的事务在锁下删过期的会话、读活着的会话、写基线、跑 `Seed` 的单元（重入）与一次 lint 的读；批准的事务在锁下重建哈希。它们在笔记本行的锁下的时长进成本守卫与 M12 的移交。
- **要确定地逼出的交错**（05-lessons 第 3 节：持住双方都要的行，`pgtest.WaitForLockWaits`，再放开；先后在一个共用的日志里断言）：
  1. 发起与正在跑的写入单元：单元先持笔记本行，发起等它，基线含它写的；发起先持，单元等它，之后被冻结拒绝（别人的 REST 写）。
  2. 发起与正在开的编辑会话：开启先持，发起看到会话答 `wiki.upgrade_pages_locked`；发起先持，开启被否决。
  3. 发起与过期之前读了时钟的心跳：会话在发起读会话时刚过期，发起删掉它的行，之后的心跳找不到行可续（审查 A-M3）。
  4. 发起与正在上传的导入（进程内占位）：上传完之后建任务行，被冻结拒绝，文件删掉。
  5. 提交的第一个事务与发起人正在跑的 `batch`：`batch` 先持，提交等它，检查看到它；提交先持，`batch` 被拒。
  6. 检查的快照已取（持住它的 worker）→ 发起人的写提交 → 提交的第一个事务 → 放开：存下的报告看得到那一次写（审查 B-I1，提交不并入检查）。
  7. 提交的两个事务之间的退回、放弃与另一次提交：第二个事务看到别的 `submission_id` 或状态，什么也不写、答 `wiki.upgrade_state`。
  8. 检查排队等名额时升级被批准或回滚完：快照里核对答 `wiki.upgrade_state`（审查 B-M3）。
  9. 批准与驳回同时：后一个答 `wiki.upgrade_state`；批准与删除笔记本同时：串行。
  10. 放弃与发起人正在跑的 `batch`：`batch` 先持，回滚的计划含它的改动。
  11. 回滚的批与取消：在两批之间停下；之后继续做完，或结束回滚。
  12. 回滚做完与删除笔记本同时：两边都先取笔记本行，串行，不死锁。
  13. 发起人被降为写者与他正在跑的写：成员的写取笔记本行，之后的单元经 `Grants` 被拒。
  14. 两次发起同时：后一次答 `notebook.upgrading`。
  每个交错的测试结束时核对 `checkPages`、`checkLinks`、`checkSearch`。

### 4.17 上限

13.1 第 31 条（写者控制的数量都有上限）与 4.18 的照做：

| 上限 | 值 | 在哪里检查 |
|---|---|---|
| 发起时列出的编辑会话 | 至多 100 项，按页 id；`detail` 写总数 | 一条语句 `LIMIT 100` 加 `count(*) OVER ()` |
| 报告里每条规则的条目 | 至多 5 项（同 lint 的摘要） | `computeIn` |
| 条目的分页 | `limit` 1–100，缺省 50（同 `listLintItems`） | 用例，在计算之前 |
| 存下的报告 | `octet_length(report::text) <= 65536` | 表的检查；用例在写之前量 |
| 闸门 3 的节点 | 至多 4 个（四个保留的键） | 天然 |
| 闸门 6、7 列出的节点 | 各至多 5 个与总数 | SQL 与 Go 的 `LIMIT` |
| 一页的待建目标数 | 至多 10,000（一页的链接数，`linking/domain/facts.go:114`） | 表的检查 |
| 回滚先删的一个单元 | 至多 `MaxUnitNodes = 10,000` 个节点 | 删之前数子树 |
| 回滚一遍的树的条目 | `MaxRollbackItems = 300,000` | 计划之前先数 |
| playbook | `MaxPlaybookBytes = 16 KiB`（`wiki/domain/playbook.go:21`） | 单元测试 |
| MCP 的答复 | 48 KiB（`Fit`） | 工具 |

### 4.18 日志

13.1 第 10 条：只记事实（id、数、耗时），不记标题、路径、正文、游标。每个用例一行 info，每条有它自己的测试，只读自己子测试开始之后的行（05-lessons 第 1 节）：

- 发起：`workspace_id`、`notebook_id`、`upgrade_id`、`user_id`、`baseline_nodes`、`baseline_pages`、`seeded_root`、`seeded_agents`、`wanted`、`elapsed_ms`；`ANALYZE` 失败另一行 warn。
- 提交：上面的 id、`passed`、不过的闸门的键（枚举）、`stale_pages`、`elapsed_ms`。
- 退回、驳回、放弃、批准、结束回滚：id、`user_id`；批准另有 `sources_hashed`、`elapsed_ms`；放弃与驳回另有 `job_id`（在提交之后由 wiki 记，4.10）；结束回滚另有剩下的三个数。
- 回滚：transfer 的任务日志照旧（种类、批数、节点数），每遍一行 `pass`、`deleted_first`、`items`、`steps`；做完一行 id 与 `elapsed_ms`；闸门 7 的基线行数不符一行 error。

### 4.19 已有的笔记库与重新导入

M10 收尾的移交：把 LLM Wiki 的导出导回一本新的 LLM Wiki 得到 `AGENTS 2` 与 `raw 2`（`bootstrap/virtual_export_import_test.go:34-36`）。推荐的路：导进一本**普通**笔记本（根下正好是 `AGENTS`、`raw`，没有序号，`:33`），再升级：发起认出根下的 `raw` 作来源根（4.4 第 8 步）、留着原有的 `AGENTS`，来源已在来源区里，agent 只补 frontmatter。README 的"升级"一节写这条路。根下的 `index`、`log`、`index.md`、`log.md`（含一个 `index.md` 的文件夹页）由闸门 3 挡住、agent 改名（导出的 `contributed` 列出的两个文件导入时已跳过）。

### 4.20 真实客户端的人工核对与 M11 的状态

M9 的 MCP 人工清单（`docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md`）在 J 之后加一节"K：升级（M11）"（`:308`、`:323` 写着"M11 的 upgrade prompt 由 M11 登记"，那两句换成指向 K）。准备：在网页上以一位管理员发起升级（设置 → 模式 → 升级为 LLM Wiki），接入用的 PAT 必须是发起人的，否则所有写都被冻结（审查 C-N10）。K1 在升级中的笔记本接笔记本入口，看 `upgrade` 的 prompt 怎样列出；K2 取它；K3 让客户端调 `upgrade_verify`、`upgrade_submit`，记审批的问法，并让它改一页原有页的正文，记 `page.upgrade_immutable` 在客户端里怎样显示；K4 在普通笔记本取 `upgrade`，看 `wiki.upgrade_state` 怎样显示。由 P3/S2 加，与 J 一起在 v0.1 的最后执行。M11 收尾之后状态是"实现完成，待人工验收"（同 M9、M10）。

## 5. 接口

**REST**（先写 `api/modules/wiki.yaml` 与 `api/openapi.yaml` 的目录，再写代码；wiki 标签的说明加"and its upgrade"；十个新操作）：

| 操作 | 方法与路径 | 请求 | 答复 | `x-problem-codes`（次序） |
|---|---|---|---|---|
| `startUpgrade` | `POST /api/v0/notebooks/{notebook_id}/upgrade` | `UpgradeStart {template_locale?}` | 200 `Upgrade` | `validation_failed`、`notebook.not_found`、`forbidden`、`notebook.already_llm_wiki`、`notebook.upgrading`、`transfer.busy`、`wiki.upgrade_pages_locked`、`wiki.upgrade_title_taken`、`server_busy`、`wiki.upgrade_stale_index` |
| `getUpgrade` | `GET …/upgrade` | — | 200 `Upgrade` | `notebook.not_found`、`forbidden`、`wiki.upgrade_not_found` |
| `checkUpgrade` | `GET …/upgrade/check` | — | 200 `UpgradeCheck` | `notebook.not_found`、`forbidden`、`wiki.upgrade_state`、`server_busy`（503，`Retry-After: 1`） |
| `listUpgradeCheckItems` | `GET …/upgrade/check/{rule}?limit=&cursor=` | — | 200 lint 的规则页（`wiki.yaml` 的 `listLintItems` 的同一个 schema） | `bad_request`（游标）、`notebook.not_found`、`forbidden`、`wiki.lint_rule_not_found`、`wiki.upgrade_state`、`validation_failed`（`limit`）、`server_busy` |
| `submitUpgrade` | `POST …/upgrade/submit` | — | 200 `Upgrade` | `notebook.not_found`、`forbidden`、`wiki.upgrade_state`、`server_busy`、`wiki.upgrade_stale_index`、`wiki.upgrade_not_ready` |
| `sendBackUpgrade` | `POST …/upgrade/send-back` | — | 200 `Upgrade` | `notebook.not_found`、`forbidden`、`wiki.upgrade_state` |
| `approveUpgrade` | `POST …/upgrade/approve` | — | 200 `Upgrade` | `notebook.not_found`、`forbidden`、`wiki.upgrade_state`、`wiki.upgrade_not_ready` |
| `rejectUpgrade` | `POST …/upgrade/reject` | — | 200 `Upgrade` | `notebook.not_found`、`forbidden`、`wiki.upgrade_state`、`server_busy` |
| `abandonUpgrade` | `POST …/upgrade/abandon` | — | 200 `Upgrade` | `notebook.not_found`、`forbidden`、`wiki.upgrade_state`、`transfer.busy`、`server_busy` |
| `endUpgradeRollback` | `POST …/upgrade/end-rollback` | — | 200 `Upgrade` | `notebook.not_found`、`forbidden`、`wiki.upgrade_state`、`transfer.busy` |

（`…` 是 `/api/v0/notebooks/{notebook_id}`。没有请求体的 POST 照 `downgradeNotebook` 的写法。顶层的 `bad_request` 等照 `api/openapi.yaml:17`。`listUpgradeCheckItems` 的次序：游标先解，再判权限、认规则名、判状态，最后 `limit`，4.7、4.16。）

**新的 schema**（`wiki.yaml`）：

- `UpgradeStart`：`{template_locale?: "zh-CN" | "en"}`，`additionalProperties: false`。
- `UpgradeState`：`upgrading`、`reviewing`、`rolling_back`、`approved`、`rolled_back`（`getUpgrade` 只答前三种；动作的答复可以是后两种）。
- `Upgrade`：`id`、`notebook_id`、`state`、`started_by`（`UserRef`）、`started_at`、`source_root_id`、`seeded_page_ids`（发起时建的页，0–2 个，来自 `seeded_root` 与 `seeded_agents_id`）、`baseline`（`{nodes, pages, wanted: LintCount}`）、`submitted_by`（`UserRef | null`）、`submitted_at`（`| null`）、`report`（`UpgradeReport | null`，存下的那份）、`rollback_job_id`（`| null`）、`ending`（`abandoned | rejected | null`）、`rollback_complete`（`boolean | null`）、`rollback_left`（`{nodes, pages, created} | null`：`rolling_back` 而任务没有排队或运行时由 page 现数，结束回滚时记下的那份）。全部必填、`additionalProperties: false`。`UserRef` 由 identity 的目录给出显示名（6.1 的例外之一：同锁与历史的 `UserRef`，第 7 节修订第 17 条）。
- `UpgradeCheck`：`computed_at`、`stale_pages`、`passed`、`gates`（`agents`、`source_root`、`reserved_titles`（带 `node_ids`）、`frontmatter`、`sources`、`links`（带 `pages_worse`、`node_ids`）、`preserved`（带 `nodes`、`nodes_kept`、`pages`、`pages_kept`、`node_ids`）、`index`，每个有 `passed`）、`rules`（八条规则的摘要，同 `getLint` 的每一项）、`counts`（`sources`、`moved_into_sources`、`wiki_pages`、`new_pages`、`changesets`、`wanted_before`、`wanted_now`，后两个是 `LintCount`）。
- `UpgradeReport`：`UpgradeCheck` 的闸门与计数，`rules` 的每项只有 `rule`、`count`、`count_capped`、`items`（至多 5 项，每项是那条规则的条目去掉 `cursor`），没有 `next_cursor`（4.7）。契约测试核对一份存下的报告（第 9 节）。

**改的 schema 与操作**：

| 对象 | 改动 | Phase |
|---|---|---|
| `Notebook`（`api/modules/notebook.yaml:462-501`；`listNotebooks`、`getNotebook`、`createNotebook`、`updateNotebook`、`takeOverNotebook`、`downgradeNotebook` 的答复） | 加 `upgrade`：必填，`NotebookUpgrade | null`，`NotebookUpgrade {id, state, started_by, started_at}`（`state` 是进行中的三种，`started_by` 只给 id） | P1/S1 |
| `NotebookMode` 的说明（`:452-460`） | "an upgrade's states are no mode"照旧；加"see the notebook's upgrade" | P1/S1 |
| `Notebook.source_root_id` 的说明（`:483-488`） | 不改："Null for a standard notebook"照旧成立（第 194 条：升级期间不先写它） | — |
| `common.yaml` 的 `locks`（`api/common.yaml:72-80`） | "No other code carries it"改为 `linking.pages_locked` 与 `wiki.upgrade_pages_locked` 带它；后者每项是一页正被编辑的会话，至多 100 项 | P1/S2 |
| `events.yaml` 的 `mode`（`:48-51`）与 `EventMode.description`（`:223`） | "a notebook's mode or its upgrade changed" | P1/S1 |
| `ChangesetKind`（`api/modules/page.yaml:1526-1531`） | 加 `rollback` | P1/S3 |
| transfer 的任务（`TransferJob` 的 `kind`） | 加 `rollback`；`upgrade_id` 不进答复（任务的名称是固定的 `upgrade`） | P1/S3 |
| `revertChangeset`、`startRevert` | 加 `page.virtual_file`（4.11），在 `page.protected` 之后 | P3/S1 |
| `cancelTransferJob` 的说明 | 回滚的任务只有笔记本管理员能取消（码不变：发起人之外的非管理员本来就答 `transfer.not_found`） | P1/S3 |

**加 `notebook.upgrading` 的 14 个写操作**（对着 `api/modules/*.yaml` 的写操作逐个核对过；P1/S1；每个在它的模块的 `adapter/http` 测试里经 `apitest.CheckResponse` 答出，13.1 第 8 条）：page 的 `createPage`、`putPageContent`、`openEditSession`、`restorePageVersion`、`toggleTask`、`renameNode`、`deleteNode`、`moveNode`、`restoreNode`、`revertChangeset`；asset 的 `uploadAsset`；transfer 的 `startImport`、`startTrashRestore`、`startRevert`。不加的写：`releaseEditLock`、`heartbeatEditSession`、`endEditSession`（4.5）、`startExport`、`cancelTransferJob`，以及 notebook 的设置、成员、删除与 `downgradeNotebook`（升级期间答 `notebook.not_llm_wiki`）。

**MCP**（每行的码照 REST 的次序，审查 C-M2）：

| 工具 / prompt | 参数 | 答复 | 入口 | 码（`mcpToolCodes()` 的行） | Phase |
|---|---|---|---|---|---|
| `upgrade_verify`（新） | `rule?`、`limit?`、`cursor?`；账户入口另有 `notebook` | 报告或规则页（路径） | 两个 | `bad_request`、`validation_failed`、`notebook.not_found`、`forbidden`、`wiki.lint_rule_not_found`、`wiki.upgrade_state`、`server_busy` | P2/S2 |
| `upgrade_submit`（新） | 账户入口的 `notebook` | `{state, submitted_at}` | 两个 | `bad_request`、`notebook.not_found`、`forbidden`、`wiki.upgrade_state`、`server_busy`、`wiki.upgrade_stale_index`、`wiki.upgrade_not_ready` | P2/S2 |
| `write`、`edit` | 照旧 | 照旧 | 两个 | 加 `notebook.upgrading`（P1/S1）、`page.upgrade_immutable`（P2/S1） | 见码 |
| `move` | 照旧 | 照旧 | 两个 | 加 `notebook.upgrading`（P1/S1）、`page.upgrade_keep_links`（P2/S1） | 见码 |
| `delete` | 照旧 | 照旧 | 两个 | 加 `notebook.upgrading`、`page.upgrade_protected`（P1/S1） | P1/S1 |
| `batch` | 照旧 | 照旧 | 两个 | 加 `notebook.upgrading`、`page.upgrade_protected`（P1/S1）、`page.upgrade_immutable`、`page.upgrade_batch_rewrite`、`page.upgrade_keep_links`（P2/S1） | 见码 |
| prompt `upgrade`（新） | — | 英文的 playbook | 笔记本 | `notebook.not_found`、`wiki.upgrade_state`（参数过界的 `validation_failed` 照 `TestTheWikiPromptsAreReached` 的做法） | P3/S2 |

`mcpOwnCodes()`（`bootstrap/mcp_client_test.go:338-340`）加四个 `page.upgrade_*`；`wiki.upgrade_state` 等由 REST 声明，不进它。每个码经真实的调用答出过（`bootstrap/mcp_reads_test.go` 的做法），测试的环境用 4.14 的升级的变体。

**新的码**（文字两种语言，由加它的服务端 Step 写）：

| 码 | HTTP | 谁答 | 模块里的定义 | 加的 Step |
|---|---|---|---|---|
| `notebook.upgrading` | 409 | 冻结的守卫与否决者、预检、transfer 开任务、发起时已有升级 | page、transfer、wiki 各自定义同一个码（先例：page 的 `notebook.not_found`，`page/domain/errors.go:21`） | P1/S1（wiki 的 P1/S2） |
| `page.upgrade_protected` | 409 | 护栏：删除基线节点或提议的来源根 | page（只在 MCP） | P1/S1 |
| `notebook.already_llm_wiki` | 409 | 发起一本 LLM Wiki | wiki | P1/S2 |
| `wiki.upgrade_not_found` | 404 | `getUpgrade`，没有进行中的升级 | wiki | P1/S2 |
| `wiki.upgrade_pages_locked` | 409，带 `locks` | 发起时有编辑会话 | wiki | P1/S2 |
| `wiki.upgrade_title_taken` | 409 | 发起时根下的 `raw` 或 `AGENTS` 是附件 | wiki | P1/S2 |
| `wiki.upgrade_stale_index` | 409 | 发起、提交时索引过时 | wiki | P1/S2 |
| `wiki.upgrade_state` | 409 | 状态不对的动作、检查、工具、prompt | wiki | P1/S3（`abandonUpgrade` 是第一个声明它的操作，审查 C-M1） |
| `page.upgrade_immutable` | 409 | 护栏：基线页的正文变了 | page（只在 MCP） | P2/S1 |
| `page.upgrade_batch_rewrite` | 409 | 护栏：`batch` 折进的改写让基线页的正文变了 | page（只在 MCP） | P2/S1 |
| `page.upgrade_keep_links` | 409 | 护栏：不改写链接的移动或改名 | page（只在 MCP） | P2/S1 |
| `wiki.upgrade_not_ready` | 409 | 提交时闸门不过；批准时锁下的再核不过 | wiki | P2/S2 |

**规模**（12.1 第 4 条）：新的 10 个 REST 操作、2 个工具、1 个 prompt，共 13 个接口操作；第 9 节 9 个故事；在"约 20 个接口操作或 15 个故事"之内，M11 是一个 M。改的：14 个写操作加码、答 `Notebook` 的 6 个、`revertChangeset` 与 `startRevert` 的码、`cancelTransferJob` 的说明、历史与任务的两个枚举、`streamEvents` 的说明、5 个 MCP 写工具、3 处工具的措辞。

## 6. 从 Nerve 借鉴

Nerve 没有模式的契约、冻结、闸门或人审的流程（M10 的调查只读核对过 `nerve-project` 没有模式的契约与只增不改的区域，[M10 总设计](../M10-llm-wiki/00-M10-design.md)第 6 节）。M11 不读 Nerve，全部沿用本项目自己的：写入单元、守卫与否决者（M4、M5、M10）、撤销的计划与后台的分批（M8、M9）、lint 的显式范围（M10）、模式变化的订阅链（M10）。

## 7. Phase 划分

| P | 名称 | 交付 | 验证 |
|---|---|---|---|
| P1 | 冻结、发起与放弃（四个 Step） | **S1 服务端：数据、冻结与删除的护栏**：迁移 00055、00056；`State`、`NotebookState`、`View` 带升级（四条查询，`ListNotebooks` 在内），`Notebook.upgrade`；`Write.UpgradeOwn`；`page.NewFreezeGuard`（守卫与否决者的第 1–4 步）登记在最前；删除的护栏与 `page.upgrade_protected`；四处预检、导入建任务与 `startBackground` 里的判断；`ensureChangeset` 记 `upgrade_id`；`notebook.upgrading` 进 14 个操作与 5 个工具，文字；`events.yaml` 的两处说明；第七份（第 186 条：`Makefile` 的分份、`test-go` 的列表、CI、`06-environment.md`）。**S2 服务端：发起与读**：迁移 00057（wiki 的第一张表）；`notebook.NewUpgradeStates` 与订阅链；page 的基线、会话、根下的键、`Seed` 的 `Upgrade`、保护行；wiki 的 lint 拆出 `computeIn`、`Runs.Hold`、按页的待建数；`startUpgrade`、`getUpgrade`；五个码与 `locks` 的说明；`checkPages` 的新规则（补 S1 夹具的 wiki 行）。**S3 服务端：放弃、回滚与结束**：迁移 00058、00059；page 的回滚（先删、计划、批、剩下的差）；transfer 的 `rollback` 与取消的规则；`abandonUpgrade`（含继续）、`endUpgradeRollback`；`wiki.NewRollbackEnd`；`wiki.upgrade_state`；新种类的文字与两份 `writingKinds`。**S4 网页**：横幅、只读、编辑失去、发起、升级页的状态与接入的说明、放弃、回滚的报告、继续与结束 | S1：`TestUpgradeFreezeReachesEveryPath`、`TestUpgradeFreezeLeavesTheRest` 等（第 9 节），最后一跳；S2：发起的码与交错、`TestUpgradeStatesReachTheStreams`、UP1（API、MCP）、UP2（API）；S3：回滚的性质测试与停下继续、结束回滚、UP3（API）；S4：控件表、UP1–UP3 的页面版本 |
| P2 | 护栏、闸门与提交（三个 Step） | **S1 服务端：护栏**：不改写链接的移动与改名、正文的切分、逐步的比较、`Folded` 与拒绝标在最后一个正文操作、账本、三个码。**S2 服务端：闸门、检查与提交**：闸门与报告（逐页的闸门 6、快照里的核对、两种 `RunKey`）、`UpgradeReport`、`checkUpgrade`、`listUpgradeCheckItems`、`submitUpgrade`（脱离请求的第二段）；`upgrade_verify`、`upgrade_submit`（`tools.json`、矩阵、码、说明、`Fit`）；"server update"的措辞；`wiki.upgrade_not_ready`。**S3 网页**：升级页的检查、条目与提交，提交不过之后的再读，审阅中的只读报告 | S1：护栏的形状表、UP4（MCP）；S2：闸门的每一格、提交的交错、10 万页的成本守卫、UP5（API、MCP）；S3：UP5 的页面版本 |
| P3 | 审阅、批准与 playbook（三个 Step） | **S1 服务端：审阅**：`approveUpgrade`（`page.NewSourceRebuild`）、`rejectUpgrade`、`sendBackUpgrade`；第 181 条（撤销里的 `OpRestore`、两个操作的 `page.virtual_file`）。**S2 服务端：playbook 与文档**：prompt `upgrade` 与 playbook、`prompts.json`、prompt 的矩阵改写、README 的"升级"一节、MCP 人工清单的 K。**S3 网页**：审阅的三个动作、报告的"更多"、批准之后的跳转 | S1：批准的一个事务与哈希的重建、驳回的回滚、181 的同步与后台与回收站恢复的正例、UP6、UP7、UP8（API）与 UP6、UP7 的 MCP；S2：prompt 的最后一跳、UP9（MCP）；S3：UP6–UP8 的页面版本 |

**大小与绿**：P1 分四个 Step（冻结的面最宽：14 个操作与 5 个工具的码、每条路径的最后一跳；它不靠发起也能测：整个程序的测试以 SQL 设上升级的列与基线的行，同 M10/P1/S2 先建守卫的骨架），P2、P3 各三个，共 10 个 Step，其中网页三个（P1/S4、P2/S3、P3/S3）。删除的护栏在 P1/S1：回滚依赖"基线节点删不掉"，而 P1/S4 合并之后网页就能发起（审查 A-M4，第 207 条）。`checkPages` 的升级规则从 P1/S2 起（wiki 的表那时才有）；P1/S1 以 SQL 设升级列的夹具到 S2 补上 wiki 的行（审查 C-Q2）。每个 Step 合并时门禁全绿：CI 的网页一份在每次合并时都跑，所以**每个服务端 Step 自己带它加的码与新种类的两种语言的文字、类型表的键与假服务端的字段**（`problem-messages.test.ts` 读契约核对每个操作声明的码，`web/…/app/problem-messages.test.ts:43-62`；`Notebook.upgrade` 必填之后 tsc 要假服务端与覆盖都有它；`kindText`、`changesetKinds` 由 tsc 强制）；网页的 service、store 方法与组件只在网页 Step 里加（knip 报出没人用的导出）。P1/S2 合并之后、P1/S3 之前，发起的升级没有放弃的路，只在开发中的 `main` 上、网页还没有入口（第 207 条）。规模照 12.1 第 4 条在第 5 节末尾。

**提交说明**（照 M10；`docs/README.md:103` 写着"从 M10 起，每个 M 在它的 00 号设计里定一种写法"，`:104` 是 M10 的一条）：
- 代码：`<范围>: <说明> (M11/P<n>/S<m>)`，例如 `server: freeze an upgrading notebook's writes (M11/P1/S1)`；范围是改动所在的部件（`server`、`api`、`web`、`e2e`、`docs`，多个用逗号）。
- 文档：直接提交到 `main` 的写 `docs: <说明> (M11/P<n>)`，M 级的写 `(M11)`，例如初稿 `docs: the M11 design's first draft, as its review reads it (M11)`；Step 的分支上的每个提交，文档的也是，都写 `(M11/P<n>/S<m>)`。
- 审查的修复：Step 的分支上、合并之前的写到 Step；合并之后的 Phase 的写到 Phase；修复的核对写明第几轮，`(M11/P<n>/S<m> fix check <k>)`、`(M11/P<n> fix check <k>)`；收尾 `(M11 closeout)`，收尾的修复同样。
- 合并：一个 Step 一次，`Merge branch 'm11-p<n>-s<m>': <说明> (M11/P<n>/S<m>)`，正文经 `git merge --no-ff -F <文件>`。`Merge main into <分支>: <一句说明>` 不带后缀。
- 旁支：分支 `m11-<slug>`，提交与合并写它所属的 Step 或 Phase；修的是更早的 M 的故事时写 `(M<k> story, found in M11)`。
- 分支名 `m11-p<n>-s<m>`。每个提交以空行加 `Co-Authored-By:` 一行结尾。
- `docs/README.md:104` 之后加一句"M11 同 M10（M11 总设计第 7 节）"，链接写 `v0.1/M11-upgrade/00-M11-design.md`，在记下设计被接受的那个文档提交里改。

### 移交的落实

| 移交 | 项 | 落实 |
|---|---|---|
| [M10 的模式](handoffs/M10-mode.md) | 1 契约表与模板 | 发起时补写的页取 `wiki.TemplatePages`（4.4 第 12 步），P1/S2 |
| | 2 先写行、在同一事务里通知订阅者 | `notebook.NewUpgradeStates` 的每个写之后 `Publish`（4.3、4.12），P1/S2；批准 P3/S1 |
| | 3 沿用 `mode` | 每个转移发 `mode`，`EventMode` 不加字段（4.12），P1/S1 改说明，P1/S2 的最后一跳 |
| | 4 模式与来源根一次写入；普通模式可先指定根 | 不先写根（第 194 条），批准时一条 `UPDATE` 写模式、根并清掉升级的列（4.9），P3/S1 |
| | 5 冻结沿用单元的边界 | 冻结的守卫经 `Grants` 问、经 `refusalOf` 标出、`batch` 与 transfer 的去处照旧（4.5），P1/S1 |
| | P2 补充：批准时重建严格后代的哈希、清区外；保全 `rewrite` | `page.NewSourceRebuild`（4.9），P3/S1；护栏放行 `Rewrite` 的步（4.6），P2/S1 |
| | P3/S1 补充：显式范围 | 闸门与发起都用显式的 LLM Wiki 契约与提议的来源根（4.7），P1/S2、P2/S2 |
| | P3/S1 补充：零 UUID 不是 nil | 提议的根从不为零：来自模板写下的页或根下已有的 `raw`，发起时校验；检查与条目的范围带它 |
| | P3/S1 补充：调用不得处于已有事务内 | 检查与条目照做（`Runs.Do`，在快照里）；发起是偏离：在写事务里调 `computeIn`（第 187 条，理由见那里） |
| | P3/S1 补充：名额与期限仍适用 | 检查与条目经 `Runs.Do`；发起经 `Runs.Hold` 占名额、计算受 `LintTimeout`（第 219 条），P1/S2 |
| | P3/S1 补充：交付前授权仍适用 | 检查与条目在交付之前再判一次 `notebook.change_mode`（4.7），P2/S2 |
| | P3/S1 补充：`stale_pages` | `stale_pages > 0` 时发起与提交拒绝（第 198 条），P1/S2、P2/S2 |
| | P4 补充：旧格式先 reindex；Unknown 不是 ready；批准前处理根下的 `index`、`log` | 同上；闸门 3（4.7），P2/S2 |
| | 收尾补充：已有的笔记库 | 4.19；README 的"升级"一节，P3/S2 |
| | 收尾补充：根下的保留标题 | 闸门 3，P2/S2 |
| | 收尾补充：回收站里的保留标题（A-M1） | 第 181 条（4.11），P3/S1 |
| | 收尾补充：upgrade 的 prompt | 新的用例，不经 `Playbooks.Execute`（4.13），P3/S2 |
| | 收尾补充：闸门、第 6 条的数 | 4.7、第 180、188 条（逐页），P2/S2 |
| | 收尾补充：内容保全按 `rewrite` 判 | 不只凭它：护栏、删除的护栏与账本的绊线（第 191 条），P1/S1、P2/S1、P2/S2 |
| | 收尾补充：守卫的次序、`Write.Notebook` | 冻结排最前（4.5），P1/S1 |
| | 收尾补充：说明与降级、`notebook.change_mode` | "upgrade"一词反过来处理（4.13），P2/S2；降级答 `notebook.not_llm_wiki`（4.3）；所有升级操作判 `notebook.change_mode`（第 195 条） |
| [M5 的事件](handoffs/M5-events.md) | 模式变化的事件类型 | M10 已建 `mode`；M11 的状态变化都发它（4.12），P1/S2 的 `TestUpgradeStatesReachTheStreams` |
| [M8/P4 的后台](handoffs/M8-P4-background.md) | 1 开始之前照 `revertChangeset` 核对、`MaxJobItems`、逐个开任务 | 第 178 条取代：一个回滚任务、先删新建的子树、计划由基线合成、界 `MaxRollbackItems`（4.10），P1/S3 |
| | 2 停下时（停在一组中间） | 合成的计划里没有恢复，没有"一组中间"；继续就是重算，确定性的停下可以结束回滚（4.10），P1/S3 |
| | 3 撤销任务正在写时不在请求里撤销它的变更集 | 不适用：回滚中冻结，回滚的任务是唯一的写者 |
| | 4 冻结按 `Kind == revert` 放行 | 第 177 条取代：`UpgradeOwn`（4.5），P1/S1 |
| | 5 任务的客户端只许 `web`、`api` | 放弃与驳回经 REST，客户端是 `web` 或 `api`，不放开（4.10） |
| [M9 的 MCP 与回滚](handoffs/M9-mcp-upgrade.md) | 1 两个工具 | 4.13，P2/S2 |
| | 2 冻结放行的写 | 发起人的 MCP 写（第 175 条）与 `UpgradeOwn`（第 177 条）；临时名称的改名、移动不看名称、按 `UpgradeOwn` 放行；`batch` 一个单元、整个放行或整个回滚（4.5、4.6），P1/S1、P2/S1 |
| | 3 回滚经多轮的调度 | 合成的条目照用 `PlanRevert` 与 `schedule`，不另写循环，`ErrRevertUnordered` 是缺陷；后台的切批照用（4.10），P1/S3 |
| | 4 MCP 开的后台任务 | 不适用：MCP 不放弃、不开任务 |

四份移交在开始 M11 时各写"处理进展"（02-workflow 第 1 节第 1 步），状态仍是 `open`；做完的 Step 各加一行；M11 收尾时改为 `done`。

### 写给后面的 M

- **M12 的性能**（追加到 [M12 的性能移交](../M12-release/handoffs/M4-performance.md)，第 21 条起）：发起在笔记本行的锁下的时长（删过期会话、基线的 `INSERT … SELECT`、`Seed`、一次 lint 的读与逐页待建数的写）在 30 万节点上；检查（摘要加两次 page 的读）在 30 万页上的时长与它占的 lint 名额（agent 反复检查时别人的 lint 答 `server_busy` 的频率）；批准时重建哈希与删掉基线的行在大的来源区上；回滚先删的单元数与计划在 `MaxRollbackItems` 上的内存与时长，以及一遍跑完的时长；护栏对每个基线页的正文步多读一次正文（LZ4 解压）的代价；`upgrade_nodes` 在 30 万节点上的体积与统计。
- **v0.1 之后的打磨**（追加到 [M12 的打磨移交](../M12-release/handoffs/M5-polish.md)，第 40 条起）：历史里标出升级的变更集（`upgrade_id` 已在库里）；退回时附一段说明；MCP 的 `notebooks`、`open_notebook` 与握手的说明标出升级中；升级页里的类型分布与逐页的清单（今天在目录视图）；已结束的升级的报告（含结束回滚时剩下的数）；网页上显示 MCP 地址（M9 的打磨第 31 条照旧，第 184 条）；横幅出现时 WebKit 上的滚动（第 224 条）。

### 总体设计的修订

随这份设计一起改的（定稿之后用脚本逐条替换，断言每段旧文字在 `v0.1-design.md` 里恰好出现一次；下面写出的每段旧文字都已这样核对过）：

1. **1.3** 不改（"严格保全：正文一字不改、不许删除"照旧，第 172 条不开例外）。
2. **3.8** 表的一行：`| 升级期间 agent 的写入 | 照常产生变更集，并标记所属的升级会话，回滚时按它整体撤销（5.8） |` → `| 升级期间 agent 的写入 | 照常产生变更集（类型照旧是编辑），并以 \`upgrade_id\` 标记所属的升级会话；回滚按基线整体复原，自己记一个类型为回滚的变更集（5.8，M11） |`。
3. **3.8** 正文：`类型（编辑、导入、还原版本、从回收站恢复、撤销、升级等）` → `类型（编辑、导入、还原版本、从回收站恢复、撤销、升级的回滚）、所属的升级会话（M11）`。
4. **5.7**：`空笔记本天然满足 LLM Wiki 的契约，不需要升级流程：新建与升级都不必另写什么（空的普通笔记本过得了 M11 的每一道闸门；M10 不另开直接切换的路，` → `空笔记本天然满足 LLM Wiki 的契约：升级的发起补写缺的 \`AGENTS\` 与 \`raw\`（M11），之后空的普通笔记本过得了每一道闸门，在网页上就能走完（M10 不另开直接切换的路，`。
5. **5.8** 第一句：`升级只能由用户自己的 agent 通过 MCP 执行；产品提供流程、护栏、闸门和人审。` → `升级的整理由用户自己的 agent 通过 MCP 执行；检查与提交 agent 与网页都能做，走同一套闸门（负责人确认，2026-11-08）；产品提供流程、护栏、闸门和人审。`
6. **5.8** 的图：旧的三行
   ```
   standard ──管理员发起──▶ upgrading ──agent 提交，闸门全部通过──▶ reviewing ──管理员批准──▶ llm_wiki
      ▲                         │                                        │
      └──放弃：整体回滚─────────┴──────驳回：整体回滚，或退回继续修改─────┘
   ```
   （第一行与第三行各恰好出现一次，替换的是从第一行到第三行的整段）换成：
   ```
   standard ──管理员发起──▶ upgrading ──提交，闸门全部通过──▶ reviewing ──管理员批准──▶ llm_wiki
      ▲                      │   ▲                              │   │
      │                      │   └─────────退回继续修改─────────┘   │
      │                      └──放弃──▶ rolling_back ◀──驳回、放弃──┘
      └────回滚做完，或停下之后结束回滚────┘
   ```
7. **5.8** 图下的一段：`图里的 \`upgrading\`、\`reviewing\` 是一次升级的状态` → `图里的 \`upgrading\`、\`reviewing\`、\`rolling_back\` 是一次升级的状态`。
8. **5.8** 第 1 步：`提示用户把 agent 连到这个笔记本的 MCP 并运行升级 playbook。` → `记下基线，根下缺 \`AGENTS\`、\`raw\` 时照模板补写（发起人的界面语言；根下已有标题为 \`raw\` 的页就用它作来源根，发起的对话框写明它下面的节点将成为来源），提示用户照 README 把 agent 连到这个笔记本的 MCP 并运行 prompt \`upgrade\`。有编辑会话、写入任务或索引过时时不能发起（M11）。`
9. **5.8** 第 2 步：`其余写入返回 409 \`notebook.upgrading\`。agent 中途断开可以重新连接继续；管理员随时可以放弃。` → `发起人不再是管理员时同样被拒；审阅中对所有人冻结，回滚中只有回滚自己的写；其余写入返回 409 \`notebook.upgrading\`（在请求本身的错误与领域的检查之后）。笔记本的设置、成员、导出与删除不冻结。冻结不设时限；agent 中途断开可以重新连接继续；任何笔记本管理员随时可以放弃（负责人确认，2026-11-08）。`
10. **5.8** 第 3 步：`定制 \`AGENTS\` 页面；按需新建综合页。` → `定制 \`AGENTS\` 页面（已有的 \`AGENTS\` 正文不能改：改名保留、另建一页）；按需新建综合页（新建页的标题会让原有页里按后缀匹配的短链接改指它，这是 4.4、4.5 的解析规则，不算破坏，M11）。`
11. **5.8** 第 4 步：`4. **自检**：agent 随时可以调用 \`upgrade_verify\` 获取检查报告，全部通过后调用 \`upgrade_submit\`。` → `4. **自检与提交**：agent 随时可以调用 \`upgrade_verify\`，管理员可以在升级页"检查"，得到检查报告；全部通过后 agent 调用 \`upgrade_submit\` 或管理员在网页上提交，提交先进入审阅、再另算一次闸门，不过时退回升级中。`
12. **5.8** 第 5 步：`5. **人审**：笔记本管理员查看升级报告，批准、驳回或退回继续修改。` → `5. **人审**：任何笔记本管理员（发起人也可以）查看提交时存下的报告，批准、驳回或退回继续修改（负责人确认，2026-11-08）。批准在一个事务里改模式、定来源根、重建来源区的哈希；之后不提供整体撤销，退回普通笔记本用降级。`
13. **5.8** 第 6 步：`6. **回滚**：放弃或驳回时，按倒序撤销本次升级的全部变更集。因为有冻结，回滚不会与别人的修改冲突。` → `6. **回滚**：放弃或驳回时，按基线整体复原：删掉升级中新建的（进回收站）、放回、改回名、写回升级开始时的正文；在后台分批，停下之后可以继续，继续就是重新算一次差；做不完时管理员可以结束回滚、保留现在的样子，原有页的正文与节点都在，历史里每个变更集都可以逐个撤销（M11 第 178 条、主持者第 212 条）。因为有冻结，回滚不会与别人的修改冲突。`
14. **5.8** 第一道闸：`- 升级开始时已存在的页面：只允许修改 frontmatter 那一块，正文必须逐字节不变；` → `- 升级开始时已存在的页面：只允许修改 frontmatter 那一块（\`platform/markdown\` 的切分，无效的 frontmatter 也是 frontmatter），正文必须逐字节不变（开头的一个 BOM 不算），系统的链接改写除外；\`batch\` 里此前的移动折进这一页的改写同样被拒，拆开即可；`；`- 升级开始时已存在的节点：不允许删除；` → `- 升级开始时已存在的节点与提议的来源根：不允许删除；`；`- 移动和新建不受限制。` → `- 移动、改名和新建不受限制，但移动与改名必须让系统改写链接。`
15. **5.8** 第二道闸：`4. 来源区以外的每篇页面都有能解析的 frontmatter，且 \`type\`、\`summary\` 不为空；` → `4. 来源区以外的每篇页面都有能解析的 frontmatter（含来源根与根下 \`AGENTS\`），且 \`type\`、\`summary\` 不为空（来源根与根下 \`AGENTS\` 除外，文件夹页也要；照 lint 的口径）；`；`6. 解析不到的链接数不多于升级前；` → `6. 每篇原有页面（现在在来源区以外的）发出的、解析不到的不同目标数不多于升级开始时（逐页比较，按 LLM Wiki 的契约与提议的来源根数；新建页面发出的不算，M11 4.7）；`；`差异只来自系统的链接改写。` → `差异只来自系统的链接改写（由护栏逐步保证，闸门在提交时核对基线节点都在、每页的正文是护栏最后认可的那份）。另有：链接的索引是新的（\`stale_pages\` 为 0）；来源根不是根下的 \`AGENTS\`。`
16. **5.8** 报告：`**升级报告**包括：移入来源区的数量、成为 wiki 页的数量、类型分布、每篇页面的类型与摘要清单、链接完整性的前后对比、内容保全的证明（例如"312/312"）。` → `**升级报告**包括：每道闸门的结果、各条 lint 规则的数与前几项、移入来源区的数量、成为 wiki 页的数量、新建的页数、本次升级的变更集数、待建链接的前后对比、内容保全的核对（例如"312/312"）；提交时存下，审阅页读它。类型分布与每篇页面的类型、摘要在目录视图里看。`
17. **5.9**：`M10/P5 实现了这三份，upgrade 一份归 M11。` → `M10/P5 实现了这三份。upgrade 一份（M11，英文）同样只在笔记本入口，只在升级中给出，别的状态答 \`wiki.upgrade_state\`。`
18. **6.1** 的码：`[M10 总设计](M10-llm-wiki/00-M10-design.md)第 5 节）。每个操作用` → `[M10 总设计](M10-llm-wiki/00-M10-design.md)第 5 节；M11 加 \`notebook.already_llm_wiki\`、\`wiki.upgrade_not_found\`、\`wiki.upgrade_state\`、\`wiki.upgrade_pages_locked\`、\`wiki.upgrade_title_taken\`、\`wiki.upgrade_stale_index\`、\`wiki.upgrade_not_ready\` 与只在 MCP 的 \`page.upgrade_immutable\`、\`page.upgrade_protected\`、\`page.upgrade_batch_rewrite\`、\`page.upgrade_keep_links\`，[M11 总设计](M11-upgrade/00-M11-design.md)第 5 节）。每个操作用`；扩展成员一句：`，别的码不带它们（13.1 第 8 条）。` → `；M11 让 \`wiki.upgrade_pages_locked\` 也带 \`locks\`，别的码不带它们（13.1 第 8 条）。`；数据格式一条：`日志带执行者与被撤销者的显示名（共用 \`common.yaml\` 的 \`UserRef\`）` → `日志带执行者与被撤销者的显示名，M11 的升级带发起人与提交人的显示名（共用 \`common.yaml\` 的 \`UserRef\`）`。
19. **6.3** 工具表：`| \`upgrade_verify\` / \`upgrade_submit\` | 升级自检与提交 | M11 |` → `| \`upgrade_verify\` / \`upgrade_submit\` | 升级的检查（报告，或一条规则的条目与游标）与提交；两个入口，管理员的 | M11 |`；prompts 一条：`upgrade（M11）。只在笔记本入口` → `upgrade（M11，英文，只在升级中给出）。只在笔记本入口`，同一条的 `每本笔记本都列出，普通笔记本里 \`prompts/get\` 答 \`notebook.not_llm_wiki\`；` → `每本笔记本都列出，ingest、query、lint 在普通笔记本里 \`prompts/get\` 答 \`notebook.not_llm_wiki\`，upgrade 只在升级中答它的文字、别的时候答 \`wiki.upgrade_state\`；工具说明里服务端的升级称 server update（M11）；`。
20. **7.2**：notebook 一行的 `由 M10 加，升级状态由 M11 加 |` → `由 M10 加；升级的四列 \`upgrade_id\`、\`upgrade_state\`（\`upgrading\`、\`reviewing\`、\`rolling_back\`）、\`upgrade_by_id\`、\`upgrade_at\` 由 M11 加，只在普通笔记本上 |`；在 `| page | \`edit_sessions\` |` 一行之后加一行 `| page | \`upgrade_nodes\` | 升级的基线：每个升级开始时活着的节点一行（种类、父节点、名称、次序值，页另有版本号、正文哈希、字节数、发出的待建目标数与护栏认可的哈希），另有发起时建的来源根的保护行；不软删除，升级结束或笔记本删除时删去（M11） |`；page 的变更集一行末尾的 `（ingest 的来源；没有外键，由清理变更集之前的一步删除，M10） |` → `（ingest 的来源；没有外键，由清理变更集之前的一步删除，M10）；变更集的 \`upgrade_id\`（所属的升级会话，部分索引）与类型 \`rollback\`（M11） |`；transfer 一行末尾 `一本笔记本同时一个写入任务（导入、恢复、撤销，部分唯一索引 \`transfer_jobs_writing_key\`） |` → `一本笔记本同时一个写入任务（导入、恢复、撤销、M11 的回滚，部分唯一索引 \`transfer_jobs_writing_key\`）；M11 加类型 \`rollback\` 与 \`upgrade_id\` |`；wiki 一行 `| wiki | \`notebook_upgrades\` | 升级会话、发起人、升级前的节点清单与正文哈希、状态（M11；wiki 在 M10 没有表） |` → `| wiki | \`notebook_upgrades\` | 升级会话：发起人与时刻、提议的来源根、发起时补写了哪几页、基线的数、提交与存下的报告、回滚的任务、状态、结局与回滚是否做完；一本笔记本至多一次进行中的升级（M11；wiki 在 M10 没有表；节点清单与正文哈希在 page 的 \`upgrade_nodes\`） |`。
21. **8.5** 后台任务表：`| 从回收站恢复、整组撤销（M8/P4） |` → `| 从回收站恢复、整组撤销（M8/P4）、升级的回滚（M11） |`。
22. **9.2** 中间一条：`笔记本首页列出根下的附件（M7）。` → `笔记本首页列出根下的附件（M7）。升级中的笔记本的每个页面上方有一条常驻的提示（M11）。`
23. **10.1**：`M10 的健康度 LW8、playbook LW11 与目录视图 LW13）` → `M10 的健康度 LW8、playbook LW11 与目录视图 LW13，M11 的 playbook UP9）`。
24. **12.2** 的 M11 一行：`升级 playbook | 中 |` → `升级 playbook | 中偏重（三个 Phase、10 个 Step：冻结、发起与放弃（四个）、护栏、闸门与提交（三个）、审阅、批准与 playbook（三个），其中网页三个 Step；冻结横跨每条写的路径） |`。
25. **12.3**：`M11 依赖 M10（模式契约），以及 M8 的整组撤销（用于回滚）。` → `M11 依赖 M10（模式契约、守卫的框架、lint 的显式范围、模式变化的订阅者、来源的哈希）、M8（撤销的计划、后台任务与分批，用于回滚）、M9（MCP 的注册表、\`batch\`、多轮的调度）。`
26. **12.4**：写入守卫一行的 `M11（冻结与护栏）。` → `M11（冻结与护栏：\`page.NewFreezeGuard\`，登记在最前；按 \`Write.Notebook.Upgrade\` 与 \`Write.UpgradeOwn\` 判，不按 \`Kind\`）。`；同一行的 `，M11 在设计里写明冻结排在哪里（M10 建议在最前，[M10 总设计](M10-llm-wiki/00-M10-design.md) 4.6）` → `；冻结排在最前（[M11 总设计](M11-upgrade/00-M11-design.md) 4.5）`；编辑会话一行的 `M11（冻结期间拒绝开启；与锁同为否决者，答哪个码同样由登记次序决定）` → `M11（冻结期间拒绝开启，否决者排在最前）`；权限规则表一行的 `M3–M11 各自加入自己的操作` → `M3–M10 各自加入自己的操作（M11 不加动作，升级用 M10 的 \`notebook.change_mode\`）`；清理任务注册表一行的 `清理器要存储；导入导出的任务） |` → `清理器要存储；导入导出的任务）、M11（wiki 的升级会话） |`；笔记本删除事件一行的 `M6（删掉这些笔记本的链接索引） |` → `M6（删掉这些笔记本的链接索引）、M11（软删除升级会话；page 另删升级的基线行） |`；事件类型一行的 `M10、M11（笔记本模式变化，类型 \`mode\`，M10 加）` → `M10、M11（笔记本模式变化，类型 \`mode\`，M10 加；M11 的升级状态的变化同样发它）`；MCP 注册表一行末尾的 `、M11 |` → `、M11（wiki 的 \`upgrade_verify\`、\`upgrade_submit\` 与 prompt \`upgrade\`） |`；模式变化一行的 `M11（升级改模式时同样经它） |` → `M11（升级的每个状态变化与批准都经它，由 notebook 模块根的 \`notebook.NewUpgradeStates\` 在调用方的事务里调，12.1 第 6 条的例外） |`。
27. **12.6**：M11 一行在设计定稿时 `| M11 | 升级流程 | 未开始 | — |` → `| M11 | 升级流程 | 进行中 | [00-M11-design.md](M11-upgrade/00-M11-design.md) |`；收尾之后照 4.20 是"实现完成，待人工验收"。
28. **14**：`| 大笔记本升级时的冻结时长 | M11 实测；支持断线续做 |` → `| 大笔记本升级时的冻结时长 | 冻结不设时限、断线续做（负责人确认，2026-11-08）；发起、检查、批准与回滚在 10 万页上有成本守卫，30 万页由 M12 实测；回滚一遍的树的条目至多 30 万，做不完时可以结束回滚、保留现状（M11 第 193 条、主持者第 212 条） |`。
29. **15**：加一行，写这些修订与依据。
30. **随实现它们的 Phase 改**（第 13 节）：13.1 第 1 条（`Write.UpgradeOwn`、`Step.KeepLinks`、`StepContent.Body`、`ContentWrite.Folded`、账本的维护、`Seed` 的升级标记、回滚的单元）、第 4 条（升级操作的码的次序）、第 5 条（升级的行排在笔记本行之后；回滚任务的队列锁）、第 8 条（新的码与 `locks`）、第 9 条（发起只读一次时钟）、第 10 条（4.18 的日志）、第 11、21 条（`notebook.NewUpgradeStates`、`page.NewSourceRebuild`、`page.NewFreezeGuard` 不只凭连接池、`wiki.NewRollbackEnd`、`(*transfer.Module).Rollbacks()`）、第 31 条（4.17 的上限）；13.2 第 18 条（升级页）、第 9 条（故事里刻意的 409）；13.4 第 1、3、4、5、6、8 条（第 9 节的反例、故事、交错、最后一跳、矩阵、成本守卫与统计状态）。

**文档与代码不符之处**（M11 里改文档，不改代码，除非另写）：
- 5.7 的"空的普通笔记本过得了每一道闸门"与 5.8 第二道闸第 1、2 条：随第 173 条成立（修订第 4 条）。
- 5.8 第一句与第 4 步与负责人第 2 条：修订第 5、11 条。
- 5.8 第 3 步的定制 `AGENTS` 与第一道闸、第 7 条：第 172 条，修订第 10 条。
- 5.8 第 6 步与 M8、M9 两份移交的"按倒序撤销、按 `Kind` 放行"：第 177、178 条，修订第 13 条与"移交的落实"。
- 5.8 第二道闸第 6 条的全本净数：逐页（修订第 15 条）。
- 3.8 的"升级"类型：代码与库里没有（`page/domain/changeset.go:21-27`），第 179 条用一列，回滚另加 `rollback`；修订第 2、3 条。
- `EventMode` 的两处说明"later, a notebook upgraded"与"a change of a notebook's mode"（`api/modules/events.yaml:48-51`、`:223`）：改为升级的变化也发（4.12）。`source_root_id` 的说明不改（第 194 条）。
- "说明里不用 upgrade 一词"（`00-M10-design.md:624`）与工具名：反过来处理（4.13）。
- 降级在升级期间如何：已答 `notebook.not_llm_wiki`（4.3），只写明。
- 闸门 4、5 照 lint 的口径（文件夹页不豁免、来源根与 `AGENTS` 的豁免）：修订第 15 条。
- M10 的移交与 4.6 写的 `pageRegistrants(pool, authorizer)`：代码是 `(pool, authorizer, logger)`（`bootstrap/registrants.go:260`），12.4 那一行是对的；M11 不改签名。
- M10 文档里的行号已漂移（例如 `stores/notebook.store.ts`、`ci.yml` 的网页一份、`notebook-layout.tsx`），本设计不照抄。

## 8. 本 M 建立与注册的扩展点

M11 不建立新的扩展点（12.4 里 M11 只注册）。注册：

- **写入守卫与开启的否决者**（M4 建立）：`page.NewFreezeGuard(pool, authorizer, admin)`，排在最前（`pageRegistrants`）。
- **笔记本模式的变化：订阅者**（M10 建立）：不加注册者；M11 经它发事件（下面的例外）。
- **实时推送的事件类型**（M5 建立）：沿用 `mode`。
- **MCP 工具与 prompt 注册表**（M9 建立）：wiki 的 `upgrade_verify`、`upgrade_submit` 与 prompt `upgrade`。
- **软删除的清理任务注册表**（M2 建立）：wiki 的清理器。
- **笔记本删除事件**（M3 建立）：wiki 的订阅者；page 的已有订阅者另删 `upgrade_nodes`。
- **权限规则表**（M2 建立）：不加行（第 195 条）。

改前面 M 的扩展点与代码（12.1 第 6 条的例外，逐项写明）：

- notebook（M3、M10）：`State`、`View`、`Notebook` 带升级；四条查询多选四列；模块根新导出 `notebook.NewUpgradeStates(pool, subscribers)`（`Lock`、`Start`、`SetState`、`Approve`、`Clear`、`Publish`，都在调用方事务里；"升级改模式时同样经它"今天没有入口，`notebook/module.go:139-141` 只交给降级）与 `ActionChangeMode`；`SetNotebookMode` 同时清升级的列。
- page（M4–M10）：`NotebookState.Upgrade`；`Write.UpgradeOwn`、`UnitSpec.UpgradeOwn`、`SeedSpec.Upgrade`；`Step.KeepLinks`、`StepContent.Body`、`StepContent.Folded`、`ContentWrite.Folded`；批的计划记下改写是否改了内容、护栏的拒绝标在 `Last`；`WriterDeps.Markdown`；`ensureChangeset` 记 `upgrade_id`；单元维护账本；`CheckContract` 判撤销里的 `OpRestore`；预检判有没有升级；新的只凭连接池的端口（基线、会话与根下的键、闸门与剩下的差的读）与 `page.NewSourceRebuild`（交附件的端口）；回滚（先删、计划、批）；模块根导出 `ReservedKeys`。
- transfer（M7、M8）：种类 `rollback`、`upgrade_id`；`Notebooks.ShareByID` 交回升级的状态、导入建任务与 `startBackground` 里判冻结；`backgroundRun` 的回滚一支（按遍）；`jobRun.finish` 的 `succeed` 里调 `Deps.RollbackEnd`；`views.sees` 对回滚的任务要管理员；模块的 `Rollbacks()`（在调用方事务里开回滚任务，不记日志）。
- wiki（M10）：`Deps` 加连接池、事务、`Names`、`UpgradeAction`、`ReservedKeys`、transfer 的 `Rollbacks`、page 的 `TreeWrites` 与各端口；`computeIn`（收调用方的时刻）；`RunKey` 的两种新种类与 `Scope`、`Runs.Hold`、`Runs.Do` 的结果的泛化。
- linking（M6、M9、M10）：`prepareWanted` 的按页的数；工具说明的措辞（"server update"）。
- 组合根：冻结的守卫的登记、`toPageState` 与 transfer 的转换、wiki 的接线与 `RollbackEnd` 的预先建、清理器与删除订阅者、第七份。
- 网页：M10 的设置"模式"一节（`general-page.tsx:128-169`，审查 C-N9）；M4 的外壳（横幅）；M5 的 `EditLost`；M4–M8 的 14 个文件的只读判断；M6 的未建链接的 `Why`；M6 的 `pages-locked.tsx`；M8 的变更集与任务的种类的文字与报告；M5 的 `modeChanged`。
- 构建与 CI（M8/P3A 起的分份）：第七份（第 9 节）。

## 9. 测试策略

- **规则（单元测试，表格驱动）**：
  - page 的 `domain`：`FreezeAdmits`（状态 × 客户端 × 执行者 × `UpgradeOwn`）；正文的切分的形状表（没有 frontmatter、开头的 BOM、frontmatter 之后的 BOM、`\r\n` 的界线、有效的、无效的带收尾行的、只有开头一行的、空的正文，每种一个反例，05-lessons"一个反例只钉住一种形状"）；`CheckContract` 的撤销里的 `OpRestore`、回收站恢复与导入除外；批的计划的 `Folded`（无关的移动之后改正文不是 `Folded`、索引过时不是、改写真的改了才是）；合成的回滚条目（位置、名称、次序、收着基线的新建、正文、净效果为零的不进、`Revision` 为 0）与先删的子树的划分（含超过 `MaxUnitNodes` 的从下往上）；合成计划在大的随机形状上的性质测试（改名环、跨层的移动、新建的文件夹收着基线节点；`PlanRevert` 不答 `ErrRevertUnordered`，执行之后等于基线）。
  - wiki 的 `domain`：闸门的判定（第 6 条逐页：抵消的场景、封顶的场景、从 `raw` 移出的基线页、新建页不算；第 7 条的数与分母）、状态机的转移表（含"审阅中、没有报告"的再提交）、报告的大小的界、`UpgradeReport` 的投影。
  - wiki 的 `app`：`Runs` 的两种新键不互相并入、`Hold` 的名额与预算（照审查 B 的 `TestReviewBJoinAfterSnapshot` 的写法）。
- **真库的适配器**：`upgrade_nodes` 的记下与核对（`INSERT … SELECT` 等于逐行读）、逐页待建数的写、闸门 7 的 SQL、剩下的差的数、发起时的会话（先删过期的）与根下的键、`SourceRebuild` 等于从头重建（`checkPages`）与缺附件元数据时报错、`notebook_upgrades` 与 `notebooks` 的每个检查与唯一索引（每个析取项一个反例，13.4 第 1 条；含"状态为空、另三列有值"与审查 B 实测的五种）、`transfer_jobs` 的新检查与写入键；`ListNotebooks` 带升级列。
- **契约**：`apitest.CheckResponse` 核对一份存下的报告（`Upgrade.report` 是 `UpgradeReport`）与每个操作的每个声明的码。
- **整个程序的测试与最后一跳**（13.1 第 21 条；把组合根交的注册者换成空，测试必须失败；第 186 条：统一以 `TestUpgrade` 起名）：
  - `TestUpgradeFreezeReachesEveryPath`：第 5 节的 14 个 REST 写操作与 5 个 MCP 写工具，每条一个别人的写与一个发起人的 REST 写，断言 `notebook.upgrading` 与"什么都没写"；编辑会话的开启；后台的导入、恢复、撤销的开始（预检）与只有建任务时才拒的情形（正在上传的导入）；发起人的 MCP 写放过。组合根的守卫列表换回 `[mode, lock]` 时失败。
  - `TestUpgradeFreezeLeavesTheRest`（审查 A-M2）：`startExport`、改名称与开放程度、加成员、删除笔记本、`releaseEditLock` 在 `upgrading`、`reviewing`、`rolling_back` 都答成功。
  - `TestUpgradeFreezeAnswersFirst`（以 SQL 造出冻结与锁都拒的写，答 `notebook.upgrading`；钉住登记次序，4.5）。
  - `TestUpgradeFreezeFollowsTheInitiatorsRole`、`TestUpgradeFreezeLetsOnlyItsOwnWrites`（别人在网页上撤销一个旧变更集被拒：不按 `Kind` 放行）。
  - `TestUpgradeStates`、`TestUpgradeStatesReachTheStreams`：每个转移（存下报告在内）到另一条流的 `mode`；订阅者交空时失败；`listNotebooks` 对升级中的笔记本 `upgrade` 不为空。
  - `TestUpgradeStart`：4.4 的码依次、`Hold` 拿不到时的 503、计算超时整个回滚、`ANALYZE`。
  - `TestUpgradeGuardrails`：P1/S1 起删除；P2/S1 起 MCP 的 `write`、`edit`、`move`（含 `update_links: false`）、`batch` 的护栏，含 `batch` 的折进与拆开之后通过、无关的移动之后改正文答 `page.upgrade_immutable`、拒绝标的操作、系统改写放行、账本（以 SQL 绕过单元改一页的正文，闸门 7 不过）。
  - `TestUpgradeGates`、`TestUpgradeSubmit`（含脱离请求：客户端在计算中断开，报告照样存下；第二个事务失败之后的退回与再提交）、`TestUpgradeSubmitInterleavings`、`TestUpgradeApproval`（模式、根、哈希、事件；`SourceRebuild` 换成空的时 `checkPages` 失败）、`TestUpgradeRollback`（放弃、驳回、先删的一段、停下继续、超时之后继续、结束回滚；`RollbackEnd` 换成空时状态留着、测试失败；写者取消回滚任务答 `transfer.not_found`）、`TestUpgradeTrashReservedTitles`（第 181 条：撤销答 `page.virtual_file`，同步与后台；`restoreNode` 与 `startTrashRestore` 恢复同一项答 200、名为 `index 2`）、`TestUpgradeTools`、`TestUpgradePrompt`、`TestUpgradeDeletion`（删除笔记本：wiki 的订阅者交空时行留着、测试失败；清理器的测试要每张带 `deleted_at` 的表都有清理器）。
  - 矩阵（第 222 条）：`TestUpgradePermissionMatrix`、`TestUpgradeToolsMatrix`、`TestUpgradePromptMatrix`（4.13、4.14）。
  - 交错：4.16 的十四个，各一个确定的测试（持锁、`pgtest.WaitForLockWaits`、放开；先后记在共用的日志里）。
  - `checkPages` 加（P1/S2 起，只看活着的笔记本，审查 A-N4、B-N7）：有升级列的笔记本恰有一行同 id、同状态、同发起人的进行中的 wiki 行；进行中的升级的基线行数等于 wiki 行的 `baseline_nodes`，基线页数等于 `baseline_pages`；基线行只属于进行中的升级；提议的来源根活着且属于这本笔记本；`seeded_agents_id` 不为空时属于这本笔记本；LLM Wiki 没有升级列。
- **e2e**（分组 `e2e/stories/upgrade/`，前缀 `up`；每个故事一个文件，版本写在测试名里，照 10.1；MCP 版本照 M9 的约定在故事自己的文件里、名为"`<故事> (MCP)`"，种下的数据经 REST，每个 agent 一个 PAT，值来自答复；照 10.1 的规则逐个故事判，豁免的写出理由与盖住它的 Go 测试，审查 C-M11；数据库断言照表放：`fixtures/assert/notebook.ts` 的升级列、`page.ts` 的基线与变更集的 `upgrade_id`、`transfer.ts` 的回滚任务，wiki 有了表，另开 `fixtures/assert/wiki.ts`，第 205 条）：

  | 故事 | 内容 | 版本与 Step |
  |---|---|---|
  | UP1 发起与冻结 | 发起；补写的两页、基线；横幅；别人的 REST 与 MCP 写 409；发起人的网页只读、MCP 写放过；另一个标签页随 `mode` 看到横幅 | API、MCP：P1/S2；页面：P1/S4（三种浏览器） |
  | UP2 发起被拒 | 编辑会话（列出页与编辑者）、写入任务、已是 LLM Wiki、已在升级；根下已有 `raw` 时对话框写明它将成为来源区 | API：P1/S2；页面：P1/S4。MCP 豁免：MCP 不能发起，被拒的发起在 MCP 上没有可看的；冻结之外的 MCP 写由 `TestUpgradeFreezeReachesEveryPath` 盖住 |
  | UP3 放弃、回滚与结束 | agent 移动、改名、改 frontmatter、新建之后放弃；基线复原、新建的进回收站、回滚的变更集；取消任务之后继续做完；另一本：取消之后结束回滚，冻结解除、剩下的数记下 | API（MCP 写作准备）：P1/S3；页面：P1/S4。MCP 豁免：MCP 不能放弃或结束回滚；回滚中 MCP 写被拒由 `TestUpgradeFreezeReachesEveryPath` 的 `rolling_back` 一格盖住 |
  | UP4 护栏 | 改正文、删除、`update_links: false` 的移动被拒；改 frontmatter、移动、改名、新建、链接改写放行；`batch` 的折进被拒、拆开通过；无效的 frontmatter 的两种形状 | MCP（只能由 MCP 完成）：P1/S1 起删除一步，P2/S1 全部。API 豁免：REST 的写在冻结里就被拒，由 `TestUpgradeFreezeReachesEveryPath` 盖住 |
  | UP5 检查与提交 | 不过的报告与条目、修好、提交进审阅、存下的报告；提交不过时退回、网页自动再检查 | API、MCP：P2/S2；页面：P2/S3 |
  | UP6 退回与驳回 | 退回之后 agent 的 MCP 写又被放行；驳回之后回滚、MCP 写答 `notebook.upgrading` | API、MCP：P3/S1；页面：P3/S3 |
  | UP7 批准 | 批准；模式、来源根、来源的哈希；另一个标签页随 `mode` 看到 LLM Wiki；根下 `AGENTS` 受保护；撤销删掉根下 `index` 的旧变更集答 `page.virtual_file`（第 181 条）；MCP：`list` 列出虚拟的两项、`delete AGENTS.md` 答 `page.protected` | API、MCP：P3/S1；页面：P3/S3（三种浏览器） |
  | UP8 只在网页上走完 | 空的普通笔记本：发起、检查、提交、批准，不接 agent | API：P3/S1；页面：P3/S3。MCP 豁免：故事本身就是不接 agent；批准之后的 MCP 由 UP7 盖住 |
  | UP9 upgrade 的 playbook | 升级中 `prompts/get upgrade` 的文字；普通笔记本答 `wiki.upgrade_state` | MCP（只能由 MCP 完成，不写，不断言数据库）：P3/S2 |

  `playwright.config.ts:3` 的 `threeBrowsers` 加 `upgrade/up(1|7)-`：横幅、发起与批准的跳转与焦点；其余的对话框与 LW12 是同一个 `ConfirmDialog`、同一套 `held` 与 `onClosed` 的焦点规则，LW12 已在三种浏览器里跑（审查 C-Q3）。页面版本里刻意引起的 409 用 `pageWatch.expectConsole(failedToLoad(409))` 声明（13.2 第 9 条）；对话框与按钮按确切的名字找（05-lessons 第 6 节）。UP9 不写页，照 10.1 的例外"什么也不写的故事……不断言数据库"（修订第 23 条）。
- **网页（vitest）**：控件表的每一项（4.15，含上传的重试）；`lostBy` 的新原因；横幅按状态与身份、在升级页上不带链接；升级页只给管理员的读、接入一节按发起人与其他管理员；对话框的焦点；`NotebookStore` 的动作与交错（照降级的七条）；回滚报告的计数与失败文字；`pages-locked` 认新码。
- **成本守卫**（13.4 第 8 条，`test-go-costs` 各一行、`-short` 跳过，界取本机的三倍并对照 CI 的竞态；审查 B-M9）：

  | 守卫 | 规模 | 统计状态 × 计划 | 预算 |
  |---|---|---|---|
  | 发起 | 10 万节点、10 万页 | `upgrade_nodes` 从未分析、分析过另一次升级的行、本次 `ANALYZE` 之后 × 自定义与通用计划（05-lessons 第 4 节） | 请求 4 s（13.4 第 8 条的默认）；放不下时那个 Step 照决定 160 另立一条决定，不悄悄放宽 |
  | 检查 | 10 万页 | 同上（闸门 7 的连接、"新建的页"的 `NOT EXISTS`） | 照决定 160 的例外：请求 6 s、worker 3 s |
  | 批准 | 10 万个来源 | 同上（基线行的 `DELETE`） | 请求 4 s |
  | 删除护栏的读 | 一次删 1 万个节点的子树 | 同上（`node_id = ANY(…)` 按主键） | 单元 2 s |
  | 回滚的计划 | 10 万个条目 | —（纯函数）与合成读的三种统计状态 | 本机两轮中位数 × 3 的时长与 `TotalAlloc` |

  种子用 SQL（05-lessons 第 1 节：测试准备不超过 4 秒的请求期限）。
- **变异（第 7 节的每个 Phase 设计列全，这里是要列进去的）**：冻结的守卫排在锁之后（`TestUpgradeFreezeAnswersFirst`）；放过发起人的非 MCP 写；去掉 `Grants.Ask`；按 `Kind == revert` 放行；去掉否决者；预检不判冻结（只有预检答的那几个情形）；把冻结的判断放进 `StartDeps.lock`（`TestUpgradeFreezeLeavesTheRest` 的导出）；发起不先删过期的会话；基线在补写之后记；删除的拒绝标在子树的根；不拒不改写链接的移动；正文不去 BOM；比全文的哈希；`Folded` 取 `Moved`；不传 `Folded`；拒绝标在 `First`；账本不在改写时更新（以 SQL 绕过单元的测试杀它，第 191 条）；闸门 6 改回全本的净数（抵消的场景）；闸门 6 算新建页；闸门 7 的分母取现读的行；快照里不核对升级的 id 与状态；提交并入已有的检查（交错第 6 条）；提交的第二个事务不核 `submission_id`；提交的第二段用请求的上下文；存下报告不发 `mode`；闸门 2 不查 `AGENTS`；批准不重建哈希、不发事件；回滚漏删新建的子树；合成的条目带 `Revision`；回滚的单元 `UpdateLinks = true`；`RollbackEnd` 不清列；结束回滚不删基线行；回滚的任务允许写者取消；`CheckContract` 的 `OpRestore` 不分种类（回收站恢复的正例）；`CheckContract` 不判 `OpRestore`；去掉两个操作的 `page.virtual_file` 的声明（`apitest` 的 `unanswered`）；`ListNotebooks` 不选升级的列；说明少一行；prompt 在审阅中也答；`writesNotebook` 不看升级；`lostBy` 不认 `notebook.upgrading`；`modeChanged` 不重读升级。
- **第七份**（第 186 条，P1/S1）：`Makefile` 加 `BOOTSTRAP_SEVENTH = ^TestUpgrade`、`test-go-bootstrap-7`、`test-go-bootstrap` 的依赖（`Makefile:272`）；`test-go` 自己逐个列出六份（`Makefile:190`），加第七份，它上面的注释"竞态检测的这七个最多六个同时跑"（`:184`）随之改为八个（审查 C-M5：不改则 `make test` 不跑 `TestUpgrade…`）；第三份的 `-skip` 加它（`Makefile:283-284`，`BOOTSTRAP_THIRD = ^Test(The[F-Z]|T[i-z]|[U-Z])`，`:250`，会收下 `TestUpgrade…`）；第四份是补集，它的 `-skip` 含第三份的模式（`Makefile:288`），那个模式收得下 `TestUpgrade…`，所以第四份不必改；第一、二、五、六份的模式都不收它（现有的 `TestUploadingAnd…` 也不匹配 `^TestUpgrade`）。CI 的 server 矩阵加 `bootstrap-7`（今天 `bootstrap-1` 到 `-6`，`.github/workflows/ci.yml:55-66`），注释（`:23-25`"十四份……共十七项"）改为十五份、十八项；`docs/v0.1/claude_to_codex/06-environment.md:27`、`:49` 的"六份""17 项"同时改。P1/S1 的记录里写出每份的 `go test -list` 证明 `TestUpgrade…` 只在第七份、各份合起来等于全部（第 223 条：不把"第三份不跳过第七份"列为变异，没有会因它失败的测试）。

## 10. 风险

| 风险 | 应对 |
|---|---|
| 冻结不设时限，发起人走了笔记本一直冻着 | 任何管理员都能放弃（负责人第 1 条）；横幅写清；无主时接管之后的管理员同样（4.3） |
| 回滚做不完，笔记本永久只读 | 先删新建的子树不计入界；规模之内到不了界；继续能越过超时与中断；确定性的停下有"结束回滚"（主持者第 212 条） |
| 合成的回滚计划比测过的真实变更集大而杂 | 大的随机形状的性质测试（第 9 节）；`ErrRevertUnordered` 落到"结束回滚" |
| 冻结的面宽：14 个操作、5 个工具、预检与任务 | 一个整个程序的测试逐条走到，组合根交空时失败；预检与建任务各有只有它答的情形；另有"冻结之外照常"的正例 |
| 护栏偏严：`batch` 的折进与 `update_links: false` 被拒 | 码明确（第 190 条、`page.upgrade_keep_links`），playbook 写拆法与"不要关掉改写"；分开调用不受影响 |
| 无效的 frontmatter 一块可以被 agent 改掉 | 照 `Parse` 的定义它本来就不是正文（4.6）；playbook 要求保留原文；历史里可以逐个撤销 |
| 文件夹页也要 `type`、`summary`，按目录组织的笔记本负担大 | 照 lint 的口径（主持者的方向）；报告列出；README 与 playbook 写明 |
| 根下已有的 `raw` 文件夹放着普通笔记 | 发起的对话框写明它将成为来源区与节点数；agent 可以移出；报告计数（主持者第 214 条） |
| 发起与批准在笔记本行的锁下做较多的事（会话、基线、一次 lint、哈希） | 发起占 lint 名额、受 `LintTimeout`；10 万页的成本守卫；30 万页进 M12；都是一次性的管理员动作 |
| 检查占 lint 的名额，agent 反复检查 | 同一个 `Runs` 的名额与期限、`server_busy` 带 `Retry-After`；同一次升级的检查合并（提交不并入） |
| 提交的两个事务之间进程停了，留在没有报告的审阅中 | 批准拒绝；退回、放弃、再提交都可用（4.8） |
| 横幅出现时 WebKit 上内容跳一下 | 同已有的公式、图表变高；P1/S4 在 WebKit 里人工看一次并记录，留给 M12 的打磨（第 224 条） |
| CI 的余量：六份合计约 2500 秒，平均约 420 秒（[M10 收尾审查](../M10-llm-wiki/reviews/M10-closeout-review.md)"要盯着的"） | 第七份从 P1/S1 起；升级的矩阵与 prompt 的矩阵进第七份（第 222 条），第三、四、五份只改已有测试的几行；e2e 加两个三种浏览器的故事，作业的 20 分钟限额照 M10 的量（6:36–8:12）有余 |
| 内置说明的余量 | 两行 183 字节之后各剩 750 字节以上（4.13） |
| `upgrade_nodes` 在 30 万节点上的体积与统计 | 只在升级中存在，结束即删；发起之后 `ANALYZE`；成本守卫量三种统计状态；M12 量 30 万 |

## 11. 负责人确认的决定

### 2026-11-08：负责人批准开始 M11 与四个产品问题

原话："开 M11 吧，按标准流程推进"。之后在同一次对话里回答了四个产品问题（选项原文）：

1. **冻结时限**：问"升级期间整本笔记本只读（冻结），只有发起人的 agent 能写。如果没人提交也没人放弃，冻结要不要有时限？" 答"不设时限"（推荐项）：一直冻结到有人操作；笔记本上一直挂"升级中"的提示，任何笔记本管理员都能随时放弃，放弃时整体回滚；不会在 agent 干到一半时自动回滚。
2. **网页提交**：问"总体设计写的是'升级只能由 agent 经 MCP 执行'，提交也由 agent 调 upgrade_submit。网页上要不要也给管理员'检查'和'提交审阅'两个按钮？两条路走同一套闸门。" 答"网页也能检查和提交"（推荐项）：闸门一样，所以同样安全；空笔记本或已经整理好的笔记本不必为了提交去接 agent；agent 的整理工作照旧经 MCP 做。
3. **批准后撤销**：问"管理员批准之后笔记本就是 LLM Wiki。之后还要不要能'整体撤销这次升级'？" 答"不提供整体撤销"（推荐项）：批准之后别人照常写，整体回滚会和这些写冲突；要退回普通笔记本就用 M10 已有的降级（立即生效，页面原样留下），个别改动可以在历史里逐个撤销。
4. **批准人**：问"谁可以批准或驳回一次升级？" 答"任何笔记本管理员"（推荐项）：发起人本人也可以批准；只有一位管理员的笔记本也能走完流程。

### 2026-11-08：开始 M11 之前的主持者的决定 172–186（负责人可以改判）

- **172 已有的根下 `AGENTS` 不开例外**。主持者的决定（负责人可以改判）。总体设计 1.3 的"严格保全"照旧：升级开始时已存在的页（含根下的 `AGENTS`）正文不能改。agent 要定制约定时，把原页改名留着（改名不受限），另建一页 `AGENTS`；原页正文非空时也可以原样留用（闸门 1 只要根下有一页正文非空的 `AGENTS`）。理由：给 `AGENTS` 开正文的例外是改负责人的关键决定。5.8 第 3 步照改，playbook 写明（4.6、修订第 10 条）。
- **173 发起时系统写下缺的两页**。主持者的决定（负责人可以改判）。根下没有 `AGENTS` 就照模板建（发起人的界面语言），根下已有标题键为 `raw` 的页就用它作来源根，否则照模板建 `raw`；不给"选一页作来源根"的选择器；这两页的写挂上升级会话、回滚时一并撤销、不进基线。理由：空笔记本与整理好的笔记本只在网页上就能走完（负责人第 2 条），5.7 的说法随之成立。根下的 `raw`、`AGENTS` 是附件时的处理见第 199 条（4.4）。
- **174 发起时有人在编辑：整个拒绝并列出**。主持者的决定（负责人可以改判）。照负责人 M6 的先例（改名碰到被锁的页整个拒绝、列出每页与编辑者）。不选"发起时结束全部会话"：它会丢掉别人正在编辑、还没自动保存的内容。有排队或运行的写入任务时答 `transfer.busy`（同降级）（4.4）。
- **175 冻结只管内容的写**。主持者的决定（负责人可以改判）。冻结挡：页面与附件的每个写、编辑会话的开启、导入与后台的恢复和撤销任务的开始（预检也答 `notebook.upgrading`）。不挡：笔记本的设置、成员、导出、删除笔记本。发起人的写只在他仍是笔记本管理员时放行（每个单元经 `Grants` 问一次）；`Write` 不带 PAT，放行的是"发起人的任何一个 MCP 客户端"。发起人被停用、离开或笔记本成了无主的，升级会话留着，另一位管理员可以放弃；横幅写清谁能放弃。理由：加一位管理员来批准、导出备份都要在冻结中能做（4.5）。原文"预检用请求真实的客户端"：预检只经 REST 到达，客户端只会是 `web` 或 `api`，这一句没有可观察的效果，预检只判有没有升级，`CheckRevert` 的签名不改（设计审查 A-M6）。
- **176 upgrade 的 playbook 用英文**。主持者的决定（负责人可以改判）。理由：同负责人 M10 第 3 条（4.13）。
- **177 回滚的写要有自己的标记，不凭 `Kind`**。主持者的决定（负责人可以改判）。M8、M9 两份移交写的"按 `Kind == revert` 放行"太宽：网页上任何人撤销一个旧变更集也是这个样子。由回滚的用例与任务设一个只有它们会设的字段，守卫要求它等于笔记本当前的升级（4.5 的 `UpgradeOwn`；设计把它同样用于发起时的补写）。
- **178 回滚按基线整体复原**。主持者的决定（负责人可以改判）。理由：冻结不设时限，agent 新建又删掉的节点过了 `purge_retention` 会被清理，碰过它的变更集都被截掉，逐个撤销就做不完。设计已核实可行（4.10：`PlanRevert` 是纯函数、接合成的条目；基线页的版本不会被清理；批与批之间停下之后重算），不退回"逐个倒序撤销"。
- **179 变更集挂升级会话用一列，类型不变**。主持者的决定（负责人可以改判）。`changesets` 加可空的 `upgrade_id`（带部分索引），升级中的写与回滚都填；agent 的写的 `kind` 照旧是 `edit`（ingest 只认 `edit`）。设计的补充：回滚自己的变更集不能用 `revert`（要 `reverts_id`），另加类型 `rollback`（第 192 条）。
- **180 闸门 6 的口径**（设计审查 B-I2 改）。主持者的决定（负责人可以改判）。逐页：发起时给每个基线页记下它发出的、解析不到的不同目标数，检查时对仍在来源区以外的基线页逐页比"不多于"；两头同一个数法（显式的 LLM Wiki 契约与提议的来源根，`wanted` 的目标判定）；新建页发出的待建链接不算（LLM Wiki 里待建是提示）。理由：初稿的全本净数会被别处的抵消（agent 新建一页正是待建的目标）与 10,000 的封顶掩盖，`update_links: false` 断掉的链接照样通过（4.7）。
- **181 回收站里的保留标题**。主持者的决定（负责人可以改判）。`CheckContract` 在 LLM Wiki 的契约下也判撤销里的 `OpRestore`（设计审查 A-I1：只限撤销，回收站的恢复与后台的恢复照旧加序号）；`revertChangeset`、`startRevert` 声明 `page.virtual_file`；不在批准时改回收站里的名字（那是绕过单元的写）（4.11）。
- **182 闸门不过时的答复**。主持者的决定（负责人可以改判）。检查不论过不过都答 200 与完整的报告；提交不过时答 409 的一个码，不带报告（不扩 `Problem` 的成员）；提交通过时存下报告，审阅页读存下的那份（4.7、4.8）。
- **183 审阅中对所有人冻结**。主持者的决定（负责人可以改判）。审阅期间发起人的 agent 也不能写，报告不再变；"退回继续修改"回到 `upgrading`，v0.1 不附说明的文字（4.3、4.8）。
- **184 网页上不显示 MCP 地址**（设计审查 C-I2 改）。主持者的决定（负责人可以改判）。负责人 M9 的决定"网页上显示 MCP 地址的界面等 v0.1 之后的打磨"（`docs/v0.1/M9-mcp/00-M9-design.md:648`）照旧，初稿开的"窄的例外"取消：升级页写 prompt 的名字 `upgrade` 与一句"照 README 的'接入 agent（MCP）'一节把 agent 连到这本笔记本"；README 的升级一节写明笔记本入口 `/mcp?notebook=<id>` 与 id 在页面地址里的位置（`README.md:259` 已写"笔记本的 id 是网页地址里 `/notebooks/` 之后的那一段"）。横幅不重复这些，只给管理员一个"查看升级"的链接。理由：改一条负责人的决定不是主持者能定的；5.8 第 1 步的提示照样有内容（4.15）。
- **185 升级状态在 `Notebook` 上，状态变化都发 `mode`**。主持者的决定（负责人可以改判）。`Notebook` 加必填、可为 `null` 的 `upgrade`；每个状态变化经订阅链发 `mode`（`EventMode` 不加字段，只改说明）；网页的横幅与只读随 `Notebook` 变（4.12）。
- **186 第七份组合根**。主持者的决定（负责人可以改判）。M11 的整个程序测试统一以 `TestUpgrade` 起名，从第一个加整个程序测试的服务端 Step（P1/S1）起加第七份，第三份 `-skip` 它（第 9 节）。理由：M10 收尾已写"再加测试就要分第七份"。

### 2026-11-08：设计审查之后的主持者的决定 212–214（负责人可以改判）

- **212 回滚停下之后可以结束回滚、保留现在的样子**。主持者的决定（负责人可以改判）。回滚停下之后（任何原因），管理员的首选动作是"继续"；另给一个"结束回滚、保留现在的样子"：升级不批准地结束，标出回滚没有做完，解除冻结，记下剩下的条目（4.10 的 `endUpgradeRollback`，细节见第 217 条）。理由：负责人定了冻结不设时限、任何管理员随时可以放弃；确定性的停下（缺陷 `ErrRevertUnordered`、一段放不进一批、界）若没有出口，笔记本会永久只读；护栏保证了原有页的正文与节点都在，保留现状不丢内容，只是结构没有全部放回。它要一个 REST 操作、矩阵的行、文字与确认对话框（`destructive`）。回答设计审查 A-Q1。
- **213 新建页让原有页的短链接改指新页，接受**。主持者的决定（负责人可以改判）。这是 Obsidian 的语义（新建只重新解析，总体设计 4.4、4.5）：agent 在根下新建 `Plan`，`Meetings/` 下一页的 `[[Plan]]` 从 `Projects/Plan` 改指新页，字节与待建数都不变。写进 5.8 的修订（修订第 10 条）与 upgrade 的 playbook（提醒 agent 新建页的标题会改变短链接的指向），报告不另列。回答设计审查 B-Q3。
- **214 根下已有的 `raw` 照用作提议的来源根**。主持者的决定（负责人可以改判）。发起的对话框写明"根下的 `raw` 将成为来源区（现有 N 个节点），批准之后它下面的都是只读的来源"，agent 可以把不是来源的移出来，报告计数（4.4、4.15）。回答设计审查 B-Q1。

### 2026-11-08：00-M11 的设计者的取舍 187–211 与修订中的 215–224（主持者的决定，负责人可以改判）

- **187 闸门 6 的基线在发起的事务里量**。lint 拆出 `computeIn`，发起在笔记本行的锁下调它一次（逐页的待建数、`wanted` 的总数与 `stale_pages`），占名额与期限见第 219 条。理由：在锁下读到的就是提交后立即的状态，又没有"发起提交之后、量之前发起人的 agent 已经写了"的窗口；不另设一个对所有人冻结的过渡状态。代价是发起的事务里多一次 lint 的读（成本守卫与 M12）（4.4、4.7）。
- **188 闸门 6 逐页的数法**（设计审查 B-I2 改；初稿是全本的数与封顶的比较）。按页数在 `generalEmitter` 的筛选之前（来源也数，从 `raw` 移出的基线页有基线可比），目标判定照 `wanted`；只比现在在区外的基线页；一页至多 10,000 条链接，没有封顶的比较；全本的 `wanted` 只作报告的前后对比（4.7）。
- **189 正文的切分**：`body` = 去掉 `FrontmatterEnd` 之后，再去掉开头的一个 BOM（不论有没有 frontmatter）。比主持者的写法多去掉"frontmatter 之后紧跟的 BOM"：agent 在 BOM 之前加 frontmatter 时，BOM 落到 frontmatter 之后，读者看不见它，不该算改了正文。并核实了主持者说明里"无效的 frontmatter"一条的前提：`FrontmatterEnd` 只在没有收尾行时答 0（4.6）。
- **190 `batch` 的折进取 B**（设计审查 A-M1 改判据）：护栏答 `page.upgrade_batch_rewrite`，单元经 `ContentWrite.Folded` 告诉守卫；`Folded` 是"计划里这一页的某次改写真的改了内容"，不是 `Moved`；护栏的拒绝标在这一页的最后一个正文操作上。不取 A。理由见 4.6。
- **191 账本是绊线**（设计审查 B-M4 与主持者的处置改写）：`upgrade_nodes.accepted_hash` 由单元在每个正文写之后维护，闸门 7 = 基线节点都活着 ∧ 每页的 `content_hash` 等于 `accepted_hash`（分母取 wiki 行）。今天正文只经单元写，这一半恒真；它只在将来出现绕过单元的写时响，杀它的测试只能用 SQL 绕过单元造出来；报告写"核对"，不写"证明"（4.6、4.7）。
- **192 回滚总在后台**：transfer 的任务种类 `rollback`，变更集类型 `rollback`；不另开同步的一条路。理由：一条路、一套测试；小的回滚在后台也只要一批（4.10）。
- **193 回滚先删新建的子树，剩下的一遍至多 `MaxRollbackItems = 300,000`**（设计审查 A-I2 改）。不含基线节点的新建子树在计划之外按单元删（每个单元至多 `MaxUnitNodes`），不计入界；剩下的条目是基线节点与收着基线节点的新建节点，都是先删之后的活节点，规模之内的笔记本到不了界；正文的写回超过时分到后面的遍。理由：初稿把新建节点算进界，而冻结不设时限、新建不受限，界会被 agent 的新建撑破、继续又停在同一处（4.10）。
- **194 提议的来源根不先写进 `notebooks.source_root_id`**：记在 wiki 的行；page 的 `upgrade_nodes` 给发起时建的 `raw` 一行保护（已有的 `raw` 是基线节点，本来就删不掉）。理由：`source_root_id` 的契约说明"Null for a standard notebook"照旧成立；page 里读 `SourceRootID` 的地方不止调查抽查的几处（`page/app/asset_writes.go:76`、`page/app/source_restore_precheck.go:60`、`page/app/ingest_before.go:81`、`page/app/unit_source_entry.go:31` 等），先写它要逐处证明都先判模式（4.1、4.6）。
- **195 权限**：升级的十个 REST 操作与两个工具都判 `notebook.change_mode`（组合根交给 wiki 与冻结的守卫），读升级也只给管理员；prompt 判 `lint.read`；不新增动作。理由：发起与批准本来就是"改模式"（`notebook/domain/actions.go:18-19` 的注释已这样预备）；只给管理员的读照 13.2 第 18 条（4.14）。
- **196 两个工具在两个入口都登记**；MCP 的 `notebooks`、`open_notebook` 与握手的说明不加升级的状态。理由：同 `lint`；`pathsAndTools` 是两个入口共用的说明，只在一个入口登记会让另一个入口的说明讲到它没有的工具；说明的缓存按 `AGENTS` 的版本，加状态要改缓存（4.13）。
- **197 报告存摘要，不重复目录**：类型分布与每页的类型、摘要在目录视图里看，审阅页链过去；5.8 的报告一条照改（修订第 16 条）。理由：目录视图已按类型分组、列出每页的类型与摘要（5.5），另存一份清单没有界又会与目录不一致。
- **198 发起时索引过时也拒绝**（409 `wiki.upgrade_stale_index`）。理由：过时的页不计入待建，基线就不可信。
- **199 发起时根下的 `raw` 或 `AGENTS` 是附件：拒绝**（409 `wiki.upgrade_title_taken`），管理员先改名。理由：附件不能作来源根，模板的页也建不出（附件与页共用名字空间）；加序号建出的 `raw 2` 作来源根只会让人困惑。
- **200 放弃在 `upgrading`、`reviewing` 与停下的 `rolling_back` 都可以**，后者就是继续回滚；回滚的任务在排队或运行时答 `transfer.busy`。停下的 `rolling_back` 另可结束回滚（第 212、217 条）。理由："任何管理员都能随时放弃"（负责人第 1 条），继续回滚不另开一个操作。
- **201 动作答 `Upgrade`、读答 `Upgrade`**（设计审查 C-M13 改；初稿的动作答 `Notebook`）；没有进行中的升级时 `getUpgrade` 答 404 `wiki.upgrade_not_found`，动作、检查、工具与 prompt 答 409 `wiki.upgrade_state`。理由：`Notebook` 的 schema 在 `notebook.yaml`，wiki 的生成代码不能引用别的模块的生成包（`archtest/rules_test.go:41-42`），wiki 也不该复制有效角色与成员数的口径；网页在动作之后重读笔记本列表，`mode` 事件也会让它重读（4.1、4.15）。
- **202 批准之后的未消化的来源照 M10 推导**，不补 ingest 标记（4.9）。
- **203 发起时的 `locks` 至多 100 项**，`detail` 写总数（同一条语句的 `count(*) OVER ()`，设计审查 B-N2）；`common.yaml` 的 `locks` 的说明改为两个码都带它。
- **204 批准时不查来源区里的重复**（4.9）。
- **205 e2e**：分组 `upgrade/`、前缀 `UP`、九个故事；wiki 有了表，断言另开 `fixtures/assert/wiki.ts`（第 9 节）。
- **206 发起时的模板的语言取请求的 `template_locale`**（缺省 `en`），网页交界面语言；同建 LLM Wiki（M10 4.4）。
- **207 删除的护栏在 P1/S1；P1/S2 合并之后、P1/S3 之前的 `main` 上，发起的升级没有放弃的路**（设计审查 A-M4 改）：网页还没有入口，只在开发中的 `main` 上，接受。理由：回滚依赖"基线节点删不掉"，要从能发起的那一刻起成立；P1/S3 的回滚是独立的一大块，与发起放在一个 Step 里太重。
- **208 `batch` 里同一页改了又改回的正文放过**：护栏只比最终的正文，与分开调用不同，写进 5.8 与 playbook（4.6）。
- **209 回滚删掉的新建节点留在回收站**，可以再恢复；不另清理。
- **210 护栏的四个码只在 MCP**：进 `mcpOwnCodes()`；REST 的写操作只加 `notebook.upgrading`（REST 的写在冻结里就被拒，到不了护栏）。
- **211 冻结期间字节不变的正文写照旧答成功**：单元在守卫之前返回（`page/app/unit_content.go:66-78`），什么也没写，不为它另加一次判断。
- **215 提交的第二段脱离请求的上下文**（设计审查 A-M5）：第一个事务之后用 `context.WithTimeout(context.WithoutCancel(ctx), LintTimeout + 10 s)` 算与写；"审阅中、没有报告"由任何管理员退回或再一次提交出来（提交的第一个事务认这种状态）。理由：MCP 客户端的工具超时会让升级卡在没有报告的审阅里，agent 自己出不来；照 transfer 写任务结束的做法（4.8）。
- **216 存下报告也发 `mode`，网页的 `modeChanged` 也重读这本笔记本的升级**（设计审查 B-M5）。理由：最常见的路是 agent 提交、管理员开着升级页；不发，审阅页一直停在"没有报告"。不在 `NotebookUpgrade` 加随报告变的字段：报告在 wiki 的表，notebook 模块读不到（4.8、4.12）。
- **217 结束回滚的写法**（实现主持者第 212 条）：只在 `rolling_back` 且回滚的任务没有排队或运行时；结局（放弃或驳回）不改，`rolled_back` 带 `rollback_complete = false` 与剩下的三个数（不在原位的基线节点、正文没写回的基线页、还在的新建节点）。理由：主持者写"记为放弃"；被驳回的升级改记为放弃会丢掉"被驳回"这一事实，所以保留结局、另记没做完（4.10）。
- **218 回滚的任务只有笔记本管理员能取消**（核实设计审查 A 的未核实的怀疑）：今天任务的发起人也能取消自己的任务（`transfer/app/views.go:94-103`），开回滚任务的管理员被降为写者之后能反复让它停下（4.3）。
- **219 发起占 lint 的名额、受 `LintTimeout`**（设计审查 B-M8）：`Runs.Hold` 只占名额、不开 goroutine；`startUpgrade` 的码加 `server_busy`。理由：M10 的移交"名额/期限仍适用"；几本大笔记本同时发起时各跑一次不计名额的 lint 又持着笔记本行（4.4、4.7）。
- **220 发起提交之后对 `upgrade_nodes` 做一次 `ANALYZE`**（设计审查 B-M9），`runtime-grants.sql` 给它 `MAINTAIN`。理由：新表一次写进至多 30 万行之后立刻被读，上一次升级分析过的统计会把这次的 `upgrade_id` 估得偏小（M10 在 `sources.sql:46-49` 记过没有统计时的平方级）；成本守卫量三种统计状态（第 9 节）。
- **221 驳回与放弃之后提交与报告留在行里**（设计审查 B-M2 的"由设计定"）：审计时看得到被驳回的是哪一份；退回才清掉它们（4.2）。
- **222 升级的矩阵写成自己的 `TestUpgrade…` 测试**（设计审查 C-M6）：`TestUpgradePermissionMatrix`、`TestUpgradeToolsMatrix`、`TestUpgradePromptMatrix` 进第七份；覆盖的检查读到它们的行。理由：`TestPermissionMatrix` 在第四份、MCP 与 prompt 的矩阵在第三份，第三、四份的两次均值在 M10/P5 是 387.5、412.5 秒，在 M10 收尾是 281、327.5 秒，界是 450 秒（`docs/v0.1/M10-llm-wiki/reviews/M10-closeout-review.md:132`）（4.14）。
- **223 分份的正确性用记录里的 `go test -list` 证明，不列为变异**（设计审查 C-M7）：去掉第三份 `-skip` 里的第七份，`TestUpgrade…` 在两份各跑一次、都绿，没有测试会失败（第 9 节）。
- **224 横幅照已有的滚动锚定**（核实设计审查 C 的未核实的怀疑）：页面靠窗口的滚动锚定（`pages/workspace/workspace-layout.tsx:107-113`），横幅插在内容之前与正文里的公式、图表变高是同一种情形；WebKit 有没有这一跳没有在三种引擎里实测（设计阶段不跑浏览器），P1/S4 在 WebKit 里人工看一次并记录，有跳动就进 M12 的打磨（4.15）。

### 对主持者说明的核实（初稿的结论，审查之后照旧）

- 第 178 条可行，不退回逐个撤销；但 app 里以变更集为键的四处要加平行的一支，回滚的界另设，新建的子树先删（第 193 条）（4.10）。
- 第 179 条对 agent 的写成立；回滚自己的变更集需要新类型 `rollback`（`changesets_reverts_check`，`00034_page_changesets_history.sql:9`）。
- "无效的 frontmatter，若 `FrontmatterEnd` 答 0……"：只对没有收尾行的形状成立；有收尾行的无效块是 frontmatter，在前面另加一块会被护栏拒，agent 要就地改或移进来源区（4.6、第 189 条）。
- "提交先置审阅再算"照做；发起时闸门 6 的基线不照这个做法，而在发起的事务里量（第 187 条）。
- 第 175 条的"预检用请求真实的客户端"没有可观察的效果（设计审查 A-M6），写明于第 175 条。
- 其余（172–177、180–186 与调查给出的方向）照做，没有发现做不到的地方。

### 需要负责人确认

无。第 184 条已改为不显示 MCP 地址，与负责人 M9 的决定一致；第 212 条（结束回滚）触及负责人第 1 条的"放弃时整体回滚"，是主持者的决定，负责人可以改判。

## 12. Phase 进度表

| P | Phase 文档 | 状态 | 审查 | 合并 |
|---|---|---|---|---|
| P1 冻结、发起与放弃（S1–S4） | — | 未开始 | — | — |
| P2 护栏、闸门与提交（S1–S3） | — | 未开始 | — | — |
| P3 审阅、批准与 playbook（S1–S3） | — | 未开始 | — | — |
| 收尾 | — | 未开始 | — | — |

## 13. 变更记录

| 日期 | 变更 | 依据 |
|---|---|---|
| 2026-11-08 | 初稿：三个 Phase、10 个 Step；九个 REST 操作、两个工具、一个 prompt、九个故事；回滚按基线整体复原；设计者的取舍 187–211 | 负责人 2026-11-08 的批准与四条回答、主持者的决定 172–186、三份只读调查（对着 `7ff74a67` 逐条核对）、四份 `open` 的移交 |
| 2026-11-08 | 设计审查之后的修订：撤销里的恢复才判保留标题（A-I1）；回滚先删新建的子树、可以结束回滚（A-I2、第 212 条）；提交不并入检查（B-I1）；拒绝不改写链接的移动、闸门 6 逐页（B-I2）；存下的报告单立 schema（C-I1）；网页不显示 MCP 地址（C-I2、第 184 条）；删除的护栏挪进 P1/S1；动作答 `Upgrade`；第十个操作 `endUpgradeRollback`；迁移的检查补全；主持者的决定 212–214，设计者的 215–224；其余 Minor、Nit 照审查者的建议 | [设计审查记录](reviews/M11-design-review.md)、主持者的修订说明 |
