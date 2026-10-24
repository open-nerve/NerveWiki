# M10 模式契约与 LLM Wiki：总设计

| 项 | 内容 |
|---|---|
| 里程碑 | M10 模式契约与 LLM Wiki（`M10-llm-wiki`） |
| 日期 | 2026-10-24 |
| 状态 | 设计中（初稿，待设计审查） |
| 依赖 | M9（MCP 工具与 prompt 注册表）、M7（导出贡献者、附件进树与链接）、M6（链接索引）、M8（变更集的类型、撤销与后台任务） |
| 上级文档 | [v0.1 总体设计](../v0.1-design.md) 第 3.3–3.10、4.2、4.4、4.5、5、6.1–6.3、7.2、8.2、8.3、8.5、10.1、12.1–12.6、13、14 节 |
| 代码位置的写法 | 同 [M9 总设计](../M9-mcp/00-M9-design.md)：模块写作 `page/app/unit.go:187`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…`、`archtest/…` 在 `server/internal/` 之下；迁移写作 `00009_notebook_notebooks.sql:5`（`server/migrations/sql/`）；SDK 写作 `sdk/mcp/protocol.go:966`（`~/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.8.0/`）；前端写作 `web/…/page-tree-item.tsx:8`（`web/apps/web/src/` 之下）；e2e 写作 `e2e/fixtures/mcp.ts:41`。行号都是 `91ee78d8` 的 |

---

## 1. 目标

1. **模式是笔记本的一项属性**：`standard` 或 `llm_wiki`，记在笔记本行上。建笔记本时在对话框里选，默认普通；`llm_wiki` 在 M10 只能在建的时候选（普通笔记本升级是 M11）；管理员可以把 LLM Wiki 降级为普通（4.3、4.5）。
2. **模板与建笔记本是一件事**：新建 LLM Wiki 笔记本时，根下的来源根页面 `raw`（带说明）与 `AGENTS` 页面在建笔记本的同一个事务里写下，用建的人的界面语言（中文或英文）；建成的 `llm_wiki` 笔记本一定有这两页（4.4）。
3. **来源区只写一次**：来源根之下的整棵子树是来源，写进去之后正文只读，改名、移动、删除只有笔记本管理员能做；与已有来源逐字节相同的新来源被拒绝，并指出已有的那一个；系统的链接改写不算修改。每一条写入路径（网页、REST、MCP、`batch`、上传、导入、还原、恢复、撤销，同步的与后台的）经同一个守卫；后台任务被拒时以码停下，不报内部错误（4.6–4.8）。
4. **`AGENTS` 与保留的标题**：LLM Wiki 下根下的 `AGENTS` 页面不能删除、改名、移动，来源根不能删除；根下的 `index`、`log` 是系统的虚拟页面，不能有同名的真实节点（4.9）。
5. **必填属性只检查**：来源区以外的页面缺 `type` 或 `summary` 时，健康度面板与 MCP 的 `lint` 报出来，任何写入都不因此被拒（负责人 2026-10-24，第 11 节第 2 条；4.10）。
6. **lint 与健康度面板**：两种模式都有的通用规则、LLM Wiki 专属的规则，读的时候在一个快照里算；REST、网页的健康度面板与 MCP 的 `lint`、`pending_sources` 给出同样的结果，每条规则一个数目与分页的条目；能读笔记本的人都看得到（4.11）。
7. **虚拟的 `index`、`log`**：MCP 的 `list` 列出、`read` 读到实时生成的 `index.md`、`log.md`，写它们得到"这个文件由系统生成"的错误；网页的页面树里两行虚拟的行，一行打开目录视图（两种模式都有），一行去历史；整本导出时写成真实的文件（4.13）。
8. **ingest 标记**：一个变更集新引用了原本未消化的来源时，记下这些来源；历史里每个读者看得到标记、能按它筛选，MCP 的 `log`、`changeset` 同样给出（4.14）。
9. **playbook**：ingest、query、lint 三份英文的操作手册，以 MCP prompt 在笔记本入口下发；握手的说明按笔记本的模式拼成（4.15）。
10. **接到 M11**：模式的列与它的检查、守卫的次序、`Write` 上的笔记本状态、事件类型 `mode`、说明的余量、lint 的规则与降级都留好了 M11 的位置，M11 只加自己的（第 7 节"写给后面的 M"）。

## 2. 范围

**做**：

- notebook：迁移 00048 加 `notebooks.mode` 与 `source_root_id`；`createNotebook` 收 `mode` 与 `template_locale`；笔记本的创建在事务里调用新的创建订阅者（新的扩展点，4.4）；专门的降级动作 `POST /api/v0/notebooks/{notebook_id}/downgrade` 与新操作 `notebook.change_mode`；模式变化的订阅者（新的扩展点）与事件类型 `mode`（4.5）；`Notebook` 带上 `mode`、`source_root_id`。
- page：写入单元的 `Write` 带上笔记本的模式、来源根与执行者在笔记本里的角色（读一次，在笔记本行的锁下）；`Step` 带上正文的哈希与大小、"系统的链接改写"的标记；模式的守卫 `page.NewModeGuard` 与它的否决者（来源页不开编辑会话），登记在编辑锁之前；规则是 `page/domain` 的纯函数，由 `batch` 的计划、上传的 `Check`、撤销的核对与导入的单元先用一遍；守卫的拒绝在单元里标出，`batch` 的 `replans` 不再吞掉它，后台的恢复、撤销与导入以新的失败 `refused` 停下；来源的哈希 `nodes.source_hash` 由单元维护；版本行的 `rewrite`；根下保留的标题与 `AGENTS`、来源根的保护；导入与恢复在根下给保留的标题加序号；页面的单元在建笔记本的事务里写模板（一处声明的例外，4.4）；路径的一处把关认出虚拟的 `index.md`、`log.md`（`list`、`read`、写的拒绝、`batch` 的计划）；变更集的 ingest 标记（`changesets.ingest_count` 与 `changeset_sources`），历史的读与筛选。
- linking：lint 的查询（各规则，量过之后才加的部分索引）；索引的观察者拿到 `ChangesetID`、属性路径与笔记本的模式，判定 ingest 并经 page 的端口记下（4.14）；REST 的目录 `GET /api/v0/notebooks/{notebook_id}/index`（M9 的 `ListIndex`）。
- wiki（新模块）：模式的契约表（每种模式有哪些 lint 规则、虚拟文件与 playbook）；模板的两种语言的正文；lint 的编排（一个快照里调 linking 与 page 的端口）与它的 REST、MCP 的 `lint`、`pending_sources`；虚拟文件的生成；导出贡献者；ingest、query、lint 三个 prompt。M10 里它没有自己的表（4.1）。
- transfer：报告的失败加 `refused`（带拒绝的码与节点），导入的问题加 `duplicate_source`；`ExportContributor` 登记 wiki 的一个。
- 平台：mcpserver 的 `Prompt` 加入口（只登记在笔记本入口）、错误经 `ProblemOf` 分类、答复有界；prompt 的契约 `api/mcp/prompts.json`。
- 网页：建笔记本的对话框选模式；设置里的模式与降级；`mode` 事件的处理；页面树的图标（最小的一组）、虚拟的两行、来源与 `AGENTS` 的行按角色收起操作；"粘贴或上传为来源"的对话框；来源页的只读视图与附件的完整列出；健康度面板；目录视图；历史的 ingest 标记与筛选；新码与新失败的中英文字。
- 测试与文档：守卫在每条触发路径上的最后一跳，prompt、贡献者、事件、创建订阅者的最后一跳；交错；lint、`index.md`、去重的成本守卫与计划测试；本 M 的故事（第 9 节）；README 的 LLM Wiki 一节；总体设计的修订。

**不做**：

- 普通笔记本切换到 LLM Wiki（空的也不行）、升级的状态、冻结、护栏与闸门：M11（第 11 节设计的取舍第 5 条）。
- 写入时检查必填属性（负责人 2026-10-24）；语义的检查（矛盾、过时）：agent 做，lint 的 playbook 给范围（5.6）。
- 来源根改指另一页：M10 里来源根由模板定下、降级清掉，M11 的升级再定（4.3）。
- 健康度面板的定时刷新与推送、写入时维护的计数（负责人 2026-10-24）。
- 中文的 playbook、账户入口的 prompt、prompt 的补全（`CompletionHandler`）、按笔记本过滤 `prompts/list` 与 `tools/list`（4.15）。
- 来源的差异对比、来源区的配额、跨笔记本的去重。
- 网页上显示 MCP 的地址：仍是 v0.1 之后的打磨（M9）。

## 3. 完成标准

总体设计 12.5 的通用标准之外：

1. **模式与模板**：两种模式经网页与 PAT 都建得出；`llm_wiki` 的笔记本建成时根下恰有 `raw`（来源根，`source_root_id` 指向它）与 `AGENTS`，正文是所选语言的模板，三者在一个事务里：模板的写失败时笔记本也不存在（整个程序的测试注入失败）。降级照 4.5：之后 `raw` 是普通的页、`AGENTS` 可以删除、虚拟的两行消失，另一个标签页经 `mode` 事件看到；有写入任务时答 `transfer.busy`。
2. **守卫到达每条路径**：4.7 的每条规则在 REST 的新建、改名、移动、排序、删除、写正文、勾选任务、开启编辑会话、上传、还原版本、从回收站恢复（同步与后台）、整组撤销（同步与后台）、导入，MCP 的 `write`、`edit`、`move`、`delete`、`batch` 上各有整个程序的测试，组合根不登记守卫时失败。拒绝的码在每个操作的 `x-problem-codes` 与 `mcpToolCodes()` 里、都答出过；`batch` 的答复指出第几项；后台任务以 `refused` 停下、报告带码与节点，已写的留着。
3. **来源区**：写一次、之后只读（系统的链接改写与撤销它的写回除外）；改名、移动、排序、删除来源要管理员；移入、上传、导入、恢复进来源区算新增；去重照 4.8（页按字节、附件按 SHA-256、不跨种类、只看活着的来源），并发的两个相同的新增只成一个（交错）；`nodes.source_hash` 等于从头重建（性质测试与 `checkPages`）。
4. **`AGENTS`、来源根与保留的标题**：4.9 的每一格有测试；导入与恢复在根下给 `index`、`log` 加序号、报告 `renamed`。
5. **lint**：4.11 的每条规则在两种模式下的表格测试（含豁免的页、附件、读作附件的目标、过时的行）；一次运行在一个快照里；REST、MCP 与网页的数目与条目相同；MCP 的答复在 48 KiB 之内；10 万页的成本守卫与 30 万页的计划测试，各在 Makefile 的一行上。
6. **虚拟文件**：`list`（文件夹与 glob）、`read`（分行、`offset`、`column`）、写与别的工具的拒绝、`batch` 的计划各有测试；`index.md`、`log.md` 的内容与界线照 4.13；整本导出带两个文件、`meta.json` 的 `contributed` 列出它们，子树的导出不带，重新导入跳过它们（贡献者的最后一跳）。
7. **ingest**：4.14 的判定在真的库上测（新引用、早已消化、编辑会话、导入与撤销不算、两本笔记本互不影响）；历史的标记、筛选与来源的列表经 REST 与 MCP。
8. **playbook 与说明**：两个入口的 `prompts/list` 等于先写的 `api/mcp/prompts.json`（反向对照）；三个 prompt 经组合根到达它们的处理函数（组合根交空时失败）；普通笔记本里答 `wiki.not_llm_wiki`；握手与 `open_notebook` 的说明照 4.15 拼成；内置的说明在 4 KiB 之内并讲到每个工具。
9. **网页**：第 9 节的页面版本的故事通过；新码与新失败在两种语言里都有文字（`problem-messages.test.ts`）。

## 4. 关键决定

### 4.1 模块与归属

| 项 | 决定 | 放弃的方案 |
|---|---|---|
| 模式的列 | `notebooks.mode` 与 `source_root_id` 是 notebook 的迁移（00048）：迁移只能改自己模块的表（`archtest/sqlc_test.go:223-236`），总体设计 7.2 也把这两列给了笔记本行。别的模块经只凭连接池的端口读它们（4.6） | 一张 wiki 的表：要端口、笔记本删除的订阅者与清理器，而每个写入单元都要读它 |
| 写入的不变量 | 在 page：守卫 `page.NewModeGuard(pool, attachments)` 是 page 模块的注册者（同 M5 的 `page.NewEditLock`），规则是 `page/domain` 的纯函数。理由：守卫要的每一样都是 page 的数据（树、祖先、正文的哈希、节点的种类、单元的标记），而同一组规则还要在 page 自己的计划里先用一遍（`batch` 的计划、上传的 `Check`、撤销的核对、导入的单元）。放在 wiki 时，wiki 不能导入 page，`Step` 与 `Change` 要在 wiki 里另抄一份、由组合根逐步转换（`bootstrap/linking.go:25-38` 的做法），page 的计划又要经端口回到 wiki 判每一步：两个方向的端口，换来的只是名义上的归属 | 守卫在 wiki：上面的两套转换与端口；守卫在 linking：linking 管链接，不管树 |
| 编排与内容 | 新模块 wiki（总体设计 8.2）：模式的契约表、模板的正文、三个 playbook、lint 的编排（一个快照里调 linking 与 page 的端口，同导出由 transfer 编排 page、linking、asset 的端口）、lint 的 REST 与 MCP 工具、虚拟文件的生成、导出贡献者。M10 里 wiki 没有表（不进 `server/sqlc.yaml`、`deploy/runtime-grants.sql` 与清理器）；要动的清单是操作的并集（`bootstrap/actions_test.go:21`）、组合检查（`bootstrap/composition_test.go:50-83`）、`api/modules/wiki.yaml` 与根的 `openapi.yaml`、`wiki/adapter/http` 的 `TestMain`、矩阵与 `wiki.*` 的码。M11 的升级（`notebook_upgrades`、升级的工具与 playbook）也落在这里 | 不建 wiki、把 lint 与工具放进 linking、模板与 prompt 放进 page：今天最省，但一种模式的契约散在三个模块里，以后的模式（总体设计 5.1"再写一份契约"）每次改三处，M11 照样要建 wiki |
| lint 的数据 | linking 写它自己的表的查询（`page_links`、`indexed_pages`、`page_properties`），来源区与 `AGENTS` 的 id、页的路径经 page 的端口（linking 的 SQL 不连 `nodes`，`server/sqlc.yaml:119-130`；M6 移交注意第 3 条） | wiki 读 linking 的表：越过模块的边界 |
| ingest 的判定 | 在 linking 的索引观察者里：只有它同时有一页写之前的链接（`ReplacePage` 交回旧行，`linking/app/index.go:46-51`）与写之后的解析（4.14）。判定的结果经 page 只凭连接池的端口记在变更集上（page 的表、page 的 SQL） | wiki 的观察者排在 `links` 之后：那时旧的行已经没了，只能再读上一版的正文重新解析；排在 `links` 之前：新的链接还没有解析 |
| 虚拟文件 | 路径的认出在 page 的 domain（`domain.FindPath`、`WalkPath` 的查找，MCP 的工具与 `batch` 的计划共用它，`page/domain/path.go:195`、`page/domain/batch_plan.go:361-493`）；内容由 wiki 生成，经 page 的 `ToolDeps.Virtual` 交给 `read` 与 `list`（同 M9 的 `ToolDeps.Attachments`，在 wiki 建好之后由组合根调 `Tools`） | 每个工具各自认：漏掉 `batch` 的模拟的树 |
| 模板的写 | 经 page 的写入单元（13.1 第 1 条：每个写入入口都经写入管线），由 wiki 的创建订阅者经 `(*page.Module).TreeWrites().Seed` 调用（4.4） | notebook 直接插入 `nodes`：越过模块，也绕过变更集与索引 |
| 降级 | notebook 的用例（模式是它的列），经 transfer 只凭连接池的 `transfer.NewWritingJobs(pool)` 判断有没有写入任务、经 page 的 `page.NewSourceArea(pool)` 清掉来源的哈希（4.5） | 放在 wiki：要 notebook 的写端口，多一层 |

组合根的次序：page → asset → wiki（拿 page 的 `TreeWrites()` 与 linking、page 只凭连接池的端口）→ transfer（`Deps.Contributors` 拿 wiki 的贡献者）→ notebook（`Deps.Created` 拿 wiki 的模板）；page 的 `Tools(ToolDeps{Attachments, Virtual})` 在 asset 与 wiki 之后调用。今天 notebook 在 page 之前建（`bootstrap/wire.go:149`、`:163`），而 page 只用 notebook 只凭连接池的端口（`notebook.NewNotebooks(pool)` 等），所以把 notebook 挪到 wiki 之后不改别的；wiki 用 linking 与 page 只凭连接池的端口，不要 linking 的模块实例；13.1 第 11 条的 page → asset → transfer 不变。

### 4.2 数据：迁移

迁移的号照合并的次序取（goose 不收比已执行的更早的号）：下表按 Phase 的次序，号是预计的。

| 迁移 | 改动 | 为什么 |
|---|---|---|
| 00048 notebook `notebooks`（P1 的 S1） | `mode text NOT NULL DEFAULT 'standard'`，`CONSTRAINT notebooks_mode_check CHECK (mode IN ('standard', 'llm_wiki'))`；`source_root_id uuid`（可空，不建外键，照 `transfer_jobs.root_id`）；`CONSTRAINT notebooks_source_root_check CHECK (mode <> 'llm_wiki' OR source_root_id IS NOT NULL)` | 4.3。`DEFAULT` 让已有的行与测试里 38 处只写明列的 `INSERT` 照旧；外键会让 notebook 与 page 互相引用、打乱清理的次序（`bootstrap/registrants.go:293-296`）；检查只要求 LLM Wiki 有来源根，不要求普通的没有，M11 升级期间可以先定来源根 |
| 00049 page `page_revisions`（P2 的 S1） | `rewrite boolean NOT NULL DEFAULT false`：这一行覆盖的每一次写都是系统的链接改写（参与者追加的写），或撤销里写回一个这样的版本 | 4.7 的"撤销的写回"；只改元数据，不重写表。M11 的"内容保全"也用得上 |
| 00050 page `nodes`（P2 的 S2） | `source_hash bytea`（`CHECK (octet_length(source_hash) = 32)`），部分索引 `nodes_notebook_id_kind_source_hash_idx ON nodes (notebook_id, kind, source_hash) WHERE source_hash IS NOT NULL AND deleted_at IS NULL` | 去重（4.8）：只有 LLM Wiki 的来源有值。加可空的列只改元数据；建部分索引要扫一遍 `nodes`（一行也不匹配），在 goose 的事务里挡住 `nodes` 的写，600 万行估计几秒，P2 量、写进升级说明 |
| linking（P3，按量的结果，可能没有） | 至多几个部分索引：未解析的链接、有歧义的链接、解析失败的页、按笔记本的属性链接 | 4.11：只给量过、确实慢的规则加；每个建的时候挡住那张表的写，各自量、写进升级说明 |
| 下一个号 page `changesets` 与新表 `changeset_sources`（P4 的 S2） | `changesets.ingest_count integer NOT NULL DEFAULT 0`（`CHECK (ingest_count >= 0)`），部分索引 `changesets_notebook_id_created_at_ingest_idx ON changesets (notebook_id, created_at DESC, id DESC) WHERE ingest_count > 0`；`changeset_sources (changeset_id uuid NOT NULL REFERENCES changesets ON DELETE CASCADE, node_id uuid NOT NULL, seq integer NOT NULL, PRIMARY KEY (changeset_id, node_id))`，`node_id` 不建外键 | 4.14。来源可能在变更集之前被清理掉，所以 `node_id` 不引用 `nodes`；变更集删除时它的行随之删除（同 `changeset_items`）。部分索引给"只看 ingest"的历史 |

下文以内容称这些迁移（"`rewrite` 的迁移""`source_hash` 的迁移""ingest 的迁移"）。wiki 没有迁移。没有新的配置项：上限都是代码里的常量（4.18）。

### 4.3 模式、来源根与 M11 的状态

- **取值**：`standard`、`llm_wiki` 两种。总体设计 5.8 的图里 `upgrading`、`reviewing` 画在两种模式之间，它们是一次升级的状态，不是模式：升级期间笔记本仍按普通的契约（守卫、lint、虚拟文件都照普通），冻结与护栏是 M11 另加的守卫；7.2 也把"`mode`、升级状态、来源根页面 id"分开列。M11 加它自己的列（或 `notebook_upgrades` 的状态）与检查，不扩 `notebooks_mode_check`；以后真有新的模式时，按名字替换这个检查（第 11 节设计的取舍第 2 条）。
- **来源根**：`source_root_id` 是一页的 id，只在模板里定下：M10 不提供改指另一页（第 2 节"不做"）。来源根在 LLM Wiki 下不能删除（4.9），所以活着的 `llm_wiki` 笔记本的来源根总是活着的页；笔记本删除时连同它一起删除（不经单元，`page/app/extension.go:259-277`），没有悬空的 id。
- **降级时**清成 `NULL`：普通笔记本里没有谁维护它，那一页之后可能被删除、清理；留着的 id 只会让 M11 拿到一个过时的提议。M11 的升级重新定来源根（"写给后面的 M"）。
- **谁读它**：写入单元在笔记本行的锁下读一次（4.6）；不开事务的读（MCP 的 `list`、`read`、说明、lint、虚拟文件、导出）经 notebook 的端口不加锁地读，与降级并发时读到哪一边都是一致的一份。
- **`Notebook`** 的答复带 `mode` 与 `source_root_id`（普通笔记本为 `null`），网页的每个笔记本页面经 `useNotebook()` 拿到（`web/…/notebook-layout.tsx:25-64`）；MCP 的 `notebooks` 与 `open_notebook` 的每项另带 `mode`。

### 4.4 新建与模板

**原子性**：一本提交了的 `llm_wiki` 笔记本没有 `AGENTS` 或来源根，就是一本坏的笔记本（守卫保护的两页从一开始就不在）。今天的 `Writer.Run` 拒绝已经在事务里的 ctx（`page/app/unit.go:186-188`），笔记本的创建也没有扩展点（`notebook/module.go:52-59`）。三种做法：

| 做法 | 代价 | 结论 |
|---|---|---|
| 页面的单元加入建笔记本的事务（声明的例外） | `Writer.Run` 多一种只给模板用的入口；锁的次序不变：建笔记本的事务第一条语句已是工作区行的 `FOR SHARE`，插入的笔记本行本就只有它看得见，单元再取工作区行与笔记本行的锁是同一事务里的重入（`platform/postgres/tx.go:121-129` 的 `WithinTx` 本就加入外层的事务），判定读到同一事务里刚插入的管理员成员（`notebook.NewFacts(pool)` 经 `postgres.DB(ctx, pool)`） | 选这个，与下一行合起来 |
| 笔记本创建的订阅者（新的扩展点） | notebook 的 `CreateNotebook` 在插入笔记本与管理员之后、发可见性之前调用 `Deps.Created`；组合根交 wiki 的模板 | 选：notebook 不知道模板，wiki 不写 notebook 的表 |
| 第二个事务加补偿（失败时删掉笔记本） | 提交与补偿之间别人看得到一本空的 `llm_wiki` 笔记本；补偿自己也会失败；删除走软删除与 60 天的清理 | 不选 |

做法：`CreateNotebook` 在要建 `llm_wiki` 时先定下来源根的 id（UUIDv7），以 `standard` 插入笔记本与管理员，调用创建订阅者（值里带模式、来源根的 id、模板的语言、执行者、客户端与时刻；订阅者照 13.1 第 21 条只交回错误）；wiki 的模板在 `mode` 是 `llm_wiki` 时经 `TreeWrites().Seed(ctx, …)` 跑一个改树的单元，以这个 id 写 `raw`、再写 `AGENTS`；`CreateNotebook` 在同一个事务里把这一行改成 `llm_wiki` 与这个来源根，再发可见性的事件、提交。模板的单元在 `standard` 下运行，模式的守卫对它没有可拒的；单元的观察者（页面事件的 `NOTIFY`、链接索引、附件）与搜索的入队照常，都在这个事务里。任何一步失败，整个创建回滚，答它的错误。

`Seed` 是 page 的 `TreeWrites()` 上的新方法，只给这一处：它要求 ctx 已在事务里（与 `Run` 相反），收下页的 id、父节点、标题与正文（只有这里由调用方给页的 id），其余同 `Run`（锁、判定 `page.create`、单元、观察者）。这是 13.1 第 1 条"单元拒绝已经在事务里的 ctx"的例外，写进 13.1 与 12.4。模板的正文在启动时解析一次（两种语言各两页，几 KiB）：事务里只做锁、比较与写（13.1 第 19 条），提取结果是静态的。

**模板的内容**（wiki 的 `domain`，中文与英文各一份，入库、有测试）：

- `raw`：来源根。正文是一段说明：这里放原始资料（文章、论文、记录、PDF 与图片），写进来之后不改；在网页上用"粘贴或上传为来源"，agent 用一次 `write`；谁能改名、移动、删除。
- `AGENTS`：LLM Wiki 的约定模板：页面的类型与 frontmatter（`type`、`summary`、`sources` 的写法，`sources: ["[[raw/…]]"]`）、来源区的规则、`index.md` 与 `log.md` 是生成的、ingest / query / lint 三个 prompt 怎么用；结尾一段"按你的领域改写这一页"。至多 8 KiB，测试钉住两种语言的段落一一对应。
- 两页不带 frontmatter（lint 对 `AGENTS` 与来源根豁免缺类型、缺摘要，4.11）。

**语言**：服务端不知道界面语言（`web/…/auth-middleware.ts:36`），所以客户端在建的请求里带 `template_locale`（`zh-CN` 或 `en`，契约的枚举；缺省是 `en`），服务端持有两种语言的模板；之后由用户改写（负责人 2026-10-24，第 11 节第 3 条）。只在 `llm_wiki` 时有用，普通笔记本收下、不用。

**变更集**：类型 `edit`，客户端是建的请求的（`web` 或 `api`），执行者是建的人，说明是所选语言的"LLM Wiki 模板"/"LLM Wiki template"；网页的历史里它是笔记本的第一个变更集。撤销它要删掉 `AGENTS` 与来源根，守卫拒绝（4.9）。

**界面**：建笔记本的对话框加模式的选择（普通在先、默认），带一句说明；`template_locale` 取界面语言。新手引导照旧建普通的个人笔记本（`web/…/notebook-step.tsx:56-90`）。

### 4.5 降级与模式的事件

- **动作**：`POST /api/v0/notebooks/{notebook_id}/downgrade`（`downgradeNotebook`，操作 `notebook.change_mode`，笔记本管理员；M11 的发起与批准升级可以用同一个操作）。次序同笔记本的管理写（13.1 第 5 条）：工作区行 `FOR SHARE` → 笔记本行 `FOR NO KEY UPDATE` → 判定（404 → 403）→ 不是 `llm_wiki` 答 409 `notebook.not_llm_wiki` → 有排队或运行的写入任务（导入、后台的恢复与撤销）答 409 `transfer.busy`（经 `transfer.NewWritingJobs(pool)`，同 `transfer/app/start_import.go:132-140` 的探查；跨前缀的码有先例：notebook 答 `workspace.not_found`）→ 写：`mode = 'standard'`、`source_root_id = NULL`、`updated_by`、`updated_at`，经 `page.NewSourceArea(pool).Forget` 清掉这本笔记本的 `nodes.source_hash`（一条语句，走 `source_hash` 的部分索引；P2 的 S2 随这一列加上），调用模式变化的订阅者 → 提交。答复是改完的 `Notebook`。
- **为什么有任务时拒绝**：任务的批之间放开笔记本行（总体设计 8.5），降级落在两批之间时，一次导入的前半按 LLM Wiki 的契约（去重、来源区）、后半按普通的。拒绝是一次读，任务结束之后再降。开始任务的事务以 `FOR SHARE` 锁笔记本行（13.1 第 5 条），与降级串行。
- **降级之后**：`raw` 是普通的页（谁都能改、移动、删除），`AGENTS` 可以删除、改名，虚拟的 `index`、`log` 消失（真实的同名页从此可以建），frontmatter 原样（总体设计 5.8）；不写变更集（没有页面的写），日志记一行 info（`notebook_id`、`user_id`）。M10 没有审计行：`notebook_audit_events` 只记归属的事（`00011_notebook_notebook_audit_events.sql:15-16`）。
- **网页**：设置的"常规"一页加"模式"一节：读者看到模式；管理员在 LLM Wiki 里看到"降级为普通笔记本"，确认对话框写明上面的后果与"不能在这里再升回来（升级流程在之后的版本）"；`transfer.busy` 照它的文字。
- **空的普通笔记本直接切到 LLM Wiki**：M10 不做。M11 的升级覆盖它（空的笔记本过得了每一道闸门，总体设计 5.7"空笔记本天然满足"说的是新建）；在 M10 加，是在 M11 的状态机之外开一条改模式的路（第 11 节设计的取舍第 5 条，负责人可以改判）。
- **事件**（[M5 的移交](handoffs/M5-events.md)，照 [给 M6 的事件类型移交](../M6-links/handoffs/M5-events.md) 的做法）：新类型 `mode`，按笔记本过滤，载荷 `{workspace_id, notebook_id}`（只带 id，13.1 第 30 条）；在写模式的事务里经组合根的适配以 `events.Publisher.Publish` 发出（照 `linkEvents`，`bootstrap/linking.go`），是 notebook 的新扩展点"模式变化的订阅者"的第一个注册者；契约加 `EventMode`（`api/modules/events.yaml`）；前端的 `eventHandlers` 加 `mode`（`web/…/events/handlers.ts:276-282`，注释里已预告 M10、M11），重读 `["notebooks", 工作区]`、`["pages", 笔记本]` 与健康度、目录的键，并加进 `refreshedOnConnect`（`web/…/app/event-stream.tsx:15-35`）；前端已有的替身类型就叫 `mode`（`web/…/app/event-stream.test.tsx:204-216`），换成真的。建笔记本不发 `mode`（新笔记本经可见性的事件让别人的流重连）。M11 的状态变化发同一个类型（[M11 的移交](../M11-upgrade/handoffs/M5-events.md)）。工作区级的类型那一处缺口（访客接受邀请等不发 `access`）与按笔记本过滤的类型无关。
- **降级之后的页面**：目录视图两种模式都有，照旧；树里的虚拟两行消失，停在一页来源的只读视图上的人随 `mode` 事件看到它成为普通的页（可以编辑）。

### 4.6 守卫：输入、位置、次序与拒绝的去处

**今天守卫看不到的**（`page/app/extension.go:27-55`）：执行者的角色（单元判定之后丢掉 `Grant`，`page/app/unit.go:225`）、笔记本的模式、页面正文的哈希与写之前的大小（单元算了哈希，`page/app/unit.go:556-560`，不交给守卫）、一次写是不是系统的链接改写（参与者追加的写与用户的写同样，`page/app/unit.go:658-661`）。

**加的输入**（都是 M4、M9 代码的声明的改动，第 8 节）：

| 加在哪里 | 什么 | 从哪里来 |
|---|---|---|
| `Write.Notebook` | `NotebookState{Mode, SourceRootID}`（M11 在这里加升级的状态） | page 的 `Notebooks` 端口的 `ShareByID`、`LockByID` 改为交回它（`page/app/ports.go:54-63`；notebook 的 `ShareNotebook` 今天只选 `id`，`notebook/adapter/postgres/queries/notebooks.sql:22-28`，加两列），在笔记本行的锁下读：降级要 `FOR NO KEY UPDATE`，与每个单元的锁互斥，所以一个单元里模式不变 |
| `Write.Role` | 执行者在这本笔记本里的有效角色 | 单元的判定交回的 `Grant.NotebookRole`（不再丢掉）；后台任务每批的单元重新判定，所以是那一批时的角色 |
| `Step.Content` | `*ContentMeta{Hash, Bytes, BeforeBytes}`：新建页面与写正文的这一步写下的正文的 SHA-256 与字节数，写之前的字节数（新建时为 0） | 单元已经算出（`Unit.content`）、已经读出当前的正文（`page/app/unit_content.go:62-66`） |
| `Step.Rewrite` | 这一步是系统的链接改写：参与者经 `Appender` 追加的写（含 `batch` 推后的参与者），或撤销里写回一个 `rewrite` 的版本（4.7） | `appender.WriteContent` 与撤销的 `RevertContent`（计划带上版本行的 `rewrite`） |

守卫要的祖先：新建、移动、移动加改名、恢复时单元已经读出父节点的一线（`page/app/unit_create.go:36`、`unit_move.go:26`、`unit_relocate.go:51`），交给守卫，不再丢掉；写正文、改名、删除时由守卫在 `llm_wiki` 下按需读一次（一条递归的语句，至多 10 层，`page/adapter/postgres/queries/nodes.sql:129-141`）。普通笔记本里模式的守卫没有规则（来源区、保护的页与保留的标题都只在 LLM Wiki），看到 `Write.Notebook.Mode` 是 `standard` 就返回，什么都不读。

**位置与次序**：`pageRegistrants(pool)` 里 `guards: [mode, lock]`、`sessionVetoers: [mode, lock]`（`bootstrap/registrants.go:253-264`）。几个守卫都拒绝时答登记在前的码（12.4）：模式的拒绝是长久的（角色、来源只读），锁是一时的；先答锁，用户等锁放开、再试，才得知其实不许。写进 12.4 的那一行；M11 的冻结建议排在最前（冻结期间整本只读，比哪一条都先），由 M11 定。守卫的码照旧排在领域的码之后（13.1 第 4 条）。

**规则是纯函数**：`page/domain` 的 `CheckContract(c Contract, s StepFacts) error`：输入是这一步的操作、每个改动的节点在这一步之前与之后在不在来源区、是不是来源根或 `AGENTS`、根下的名称键、角色、`Step.Content`、`Step.Rewrite`、单元的类型；输出是 4.7–4.9 的码。守卫从库与单元读出这些事实再调它；下面的预检从各自的计划算出同样的事实再调它：

| 预检 | 时机 | 拒绝在什么之前 |
|---|---|---|
| `batch` 的计划（`PlanBatch`） | 在模拟的树上按项的次序，每项照"那一刻"的树判：所以 `[改 P，把 P 移进来源区，再改 P]` 在第三项被拒，与分开调用相同（不是看合成之后的一次写） | 单元与任何正文的解析之前（`page/app/apply_batch.go:261-312`） |
| 上传的 `Check` | 不加锁、读文件之前（`page/app/asset_writes.go:47-54`）：根下的保留标题、角色无关（上传进来源区是新增）；去重要文件的 SHA-256，只能在流式写完之后的单元里判 | 读文件之前 |
| 撤销的核对（请求里不加锁的一遍、锁下的一遍，后台每批的 `CheckRevertPart`） | 照计划的每一步判角色、保护的页与保留的标题 | 单元之前；后台的在每批之前 |
| 导入的单元 | 每个节点建之前判保留的标题（加序号，4.9）与去重（跳过、写进报告，4.8） | 那个节点的写之前，不让整个单元回滚 |

守卫仍是最后的权威：预检与守卫用同一个函数，所以两者不会走散；预检漏判的，单元里照样拒绝。

**拒绝的去处**：

- **标出守卫的拒绝**：`Unit.apply` 把守卫的错误包一层（`guardRefusal`，带 `Unwrap`），`errors.As` 照样拿得到 `*shared.Error`，答复照旧。`batch` 的 `failed` 先认出它、答 `OpFailed{Op: i}`，不再交给 `replans`（`page/app/apply_batch.go:576-597`：守卫答 `page.not_found`、`validation_failed`、`page.title_taken`、`page.cycle`、`page.too_deep` 时今天会被当成树变了而重新计划，守卫的错误就丢了）。M10 的码本就不在那个集合里；标出之后 M11 的冻结、以后任何守卫都不受它约束。
- **后台撤销**：`revertDefect`（`page/app/unit_revert.go:913-918`）先认出守卫的拒绝，原样交回，不当作缺陷。
- **transfer 的任务**：组合根的 `treeError` 经 page 导出的 `page.Refusal(err)` 认出守卫的拒绝，换成 transfer 的 `ErrRefused{Code, NodeID}`；`unitFailure`（`transfer/app/batches.go:87-96`）在 403/404 那一支之前认它，任务以新的失败 `refused` 停下，报告带 `refusal: {code, node_id}`，已写的批留着（同别的停下）。今天它会成为 `internal`，403 的码会报成 `forbidden`（"你不能再写这本笔记本"，不对）。网页的报告照 `refusal.code` 取那个码的文字（`problem-messages.ts` 里已有）。
- **码**：每个页面的写操作的 `x-problem-codes` 加上它可能答的模式的码（`apitest` 按操作双向核对，13.1 第 8 条；守卫的码要写进每个被扩展的操作，13.1 第 21 条），`mcpToolCodes()`（`bootstrap/permission_matrix_mcp_test.go:84-108`）的写的工具各加一行，整个程序的测试每个码都要答出过。

### 4.7 来源区的规则

"来源"是来源根的严格后代（来源根自己不是来源：它的正文是说明，写者可以改，总体设计 5.4）。下表的"新增"写者以上可以做；"管理员"指笔记本的有效角色是管理员；角色不够答 403 `page.source_admin_only`，正文的修改答 409 `page.source_immutable`。

| 操作 | 落到来源上时 | 码 |
|---|---|---|
| 新建页面（REST、MCP 的 `write`、`batch`、导入，带正文） | 新增；带正文的要过去重（4.8）；空的页（MCP 与 `batch` 补建的父页、网页的"新建页面"）照样新增，是文件夹 | `page.source_duplicate` |
| 上传附件 | 新增，按 SHA-256 去重（流式写完之后，单元里） | `page.source_duplicate` |
| 写正文（PUT、自动保存、勾选任务、MCP 的 `write` 覆盖与 `edit`、`batch`） | 只许三种：系统的链接改写（`Step.Rewrite`）；空的来源（写之前 0 字节）的第一次正文，一次，过去重；来源根自己的正文。别的一律拒绝 | `page.source_immutable`、`page.source_duplicate` |
| 开启编辑会话 | 来源上拒绝（否决者，同锁的否决者的位置）；来源根可以 | `page.source_immutable` |
| 改名 | 来源、来源根：管理员 | `page.source_admin_only` |
| 移动、排序（同一父节点下的移动也是 `OpMove`，`page/domain/change.go:75-88`）、MCP 的移动加改名 | 移入来源区（之前不在、之后在）：新增，子树里每个带正文的页与每个附件过去重；移出、在来源区里移动或排序：管理员；移动的子树里有来源根（移动来源根或它的祖先，来源跟着换位置）：管理员，来源区是空的也一样 | `page.source_admin_only`、`page.source_duplicate` |
| 删除 | 子树里有来源：管理员；子树里有来源根：一律拒绝（4.9） | `page.source_admin_only`、`page.protected` |
| 还原版本（`version_restore`） | 来源上一律拒绝（还原旧的字节就是改正文，也会撤回链接改写，留下断链）；来源根与 `AGENTS` 照常 | `page.source_immutable` |
| 从回收站恢复（`trash_restore`，同步与后台的每一批） | 落进来源区：新增，过去重（比的是活着的来源，4.8）；父节点不在而落到根下：不再是来源 | `page.source_duplicate` |
| 整组撤销（`revert`，同步与后台的每一批） | 每一步照它本来的操作判（下面） | 同各操作 |

**撤销与历史**（[M8 的移交](handoffs/M8-history.md)第 3 项，[M9/P1 的移交](handoffs/M9-P1-mcp-registry.md)第 7 项）：

- **写回 C 改写过的来源页**：撤销一次改名或移动 C 时，C 的链接改写碰过的来源被逐字节写回。C 的那一版是参与者追加的写，版本行的 `rewrite` 为真（`rewrite` 的迁移，`RecordRevision` 的 `ON CONFLICT` 以"与"合并：同一变更集里用户的写与改写落在一行时为假，`page/adapter/postgres/queries/changesets.sql:52-63`）；撤销的计划把它带到写回的一步，这一步的 `Step.Rewrite` 为真，守卫放行：撤销系统的改写同样不算修改。撤销写下的这一版也记 `rewrite`，所以撤销的撤销同样放行。C 的那一版不是改写（来源的第一次正文；M11 升级之前的历史里用户的写）时，写回是修改，拒绝。M10 之前已有的版本行都是 `false`：LLM Wiki 的笔记本从 M10 才有，没有这样的旧行；M11 升级的笔记本里旧的改写会被当作修改（偏严）。
- **还原来源页的旧版本**：一律拒绝（上表）。
- **从回收站恢复来源、撤销一次删除来源**：新增（写者可以），过去重。删除来源只有管理员能做，恢复它不要：恢复是把资料加回来，不改任何已有的来源。
- **撤销一次新建来源（删掉它）、一次来源的移动或改名**：照删除、移动、改名，要管理员。所以撤销一次带着新来源的 ingest 要管理员；只写 wiki 页的 ingest（常见的）写者照常撤销。写者把一页移进来源区之后，移出它（包括撤销那次移动）也要管理员（第 11 节设计的取舍第 8 条）。
- **临时名称的步**（`AsideName`，三条后备）：守卫逐步判，临时的改名与移到根下照普通的改名、移动判，**不加标记、不凭名称认**。理由：临时的步只发生在撤销要放回的节点上（`page/domain/revert_schedule.go:439-521` 的三条后备都只取待放回的）；一个此刻是来源的节点被临时改名或移走，它最后的放回本身就是来源的改名或移动（它现在在来源区里，而它在 C 之前的状态不同），要管理员；所以凡是会碰到来源的临时步的撤销，净效果本来就要管理员，逐步判与按净效果判的结论相同。只有一种情形逐步判比净效果严：撤销先把某个祖先放回来源区、再把它之下一个待放回的节点临时改名（那一刻它已在来源区里），净效果是"移入来源区"（写者可以），逐步判要管理员。这只会多拒、不会多放：逐步判不会放过任何一个净效果要管理员的撤销，因为来源换位置必经一个落在来源区里的节点或来源根的步。写进第 10 节的风险。`AGENTS` 从不被临时改名（它在 LLM Wiki 下不改名、不移动，没有变更集碰它的树）；临时名称带 ` ~<hex>`，从不等于保留的标题。
- **先恢复、再写回**：同一单元里恢复又写回的来源页，恢复的一步是新增（照回收站里的正文的哈希去重），写回的一步照上面的写回判。
- **移动加改名的一步**（`Unit.Relocate`、撤销的放回）：守卫同时看这一步的父节点与名称，按移动判来源区，按改名判 `AGENTS` 与保留的标题，不从步的种类推只变了一样。
- **拒绝时**：同步的撤销与恢复整组拒绝（`page/app/revert_changeset.go:117-121`、`restore_node.go:117-124` 照答守卫的码）；后台的在那一批以 `refused` 停下（4.6）；网页的撤销对话框与任务的报告照码说明。

**`batch` 的合成**：一页的几个正文操作在它第一个正文操作的位置以最后的正文写一次（`page/app/apply_batch.go:665-676`），守卫看到的是合成之后的一步；计划按项的次序先判（4.6），所以 `batch` 的结果与按次序分开调用相同，不是 M9 4.11 的那一条规则之外的新的不同。

**来源页的编辑会话**：否决者在开启时拒绝（`page.source_immutable`），所以网页从不对来源开编辑器；一页被编辑时被移进来源区，之后的自动保存被守卫拒绝，编辑器照"失锁"的样子只读并给出文字。

### 4.8 去重

- **范围**：一个新增的来源与这本笔记本里**活着的**来源（不含回收站、不含来源根）比较：页面按正文的 SHA-256（即逐字节相同：同一个 BOM、换行、frontmatter），附件按测定的 SHA-256（`asset_blobs.sha256`）；页面与附件不互相比较（第 11 节设计的取舍第 10 条）。空的页（0 字节）不比较、不算重复。同一个单元里先后加的两个也算（`batch`、导入的一批、移入的一棵子树里两个相同的）。比的是现在的字节：来源被系统的链接改写过之后，它的哈希是改写之后的（负责人的决定说"与已有来源的字节完全相同"）。
- **拒绝**：409 `page.source_duplicate`，problem 的新成员 `duplicate_of` 是已有那一个的节点 id（6.1：答复只给 id；网页按树写出它的路径并给链接）；MCP 的错误文字带它的路径（page 的 MCP 适配器写路径）。导入进来源区时不让整个任务停下：重复的文件在导入的单元里先判、跳过，报告的问题 `duplicate_source`（带它在库里的路径），其余照常（4.6 的预检）。
- **哈希记在哪里**：`nodes.source_hash`（`source_hash` 的迁移）：活着的、在 LLM Wiki 的来源区里、带正文的页的正文哈希，或附件的 SHA-256；别的节点为 `NULL`。部分索引 `(notebook_id, kind, source_hash) WHERE source_hash IS NOT NULL AND deleted_at IS NULL` 让一次查找是一次索引的探查，只扫这本笔记本的来源。
  - **由单元维护**：`Unit.apply` 在每一步写之后、记条目之前，对 `llm_wiki` 的单元按这一步之后的来源区设上或清掉这一步改动的节点的 `source_hash`（新建、第一次正文、链接改写、移入与移出（子树的每个节点都在 `OpMove` 的改动里，`page/app/unit_move.go:73-88`）、恢复），与搜索的入队一样是 page 自己的派生数据，不是扩展点。删除不用动：部分索引排除 `deleted_at`，恢复时再设。降级时一条语句清掉整本的（4.5）。同一单元里后面的步的守卫看得到前面的步设下的值，所以单元里的重复也找得到。
  - **不变式**：`source_hash` 恰是活着的来源的当前哈希：`checkPages` 加这一条，性质测试"随机的写之后等于从头重建"（13.4 第 10 条）。
- **为什么不给 `page_contents`、`asset_blobs` 建哈希索引**（第 11 节设计的取舍第 10 条，与"每次比较整个笔记本的页"一起比较）：

  | 做法 | 代价 |
  |---|---|
  | `page_contents (content_hash)` 的部分索引 | `page_contents` 没有 `notebook_id`（`00013_page_page_contents.sql:5-21`）：索引是整个实例的，同一份字节在别的笔记本里有多少就扫多少（空的、模板一样的小页成千上万），一次写的工作随看不到的内容变（13.1 第 4 条的规则）；内容的每次写（自动保存约 2 秒一次）都改这个索引的键，失去 HOT 的更新；在 600 万行的表上建索引挡住写 |
  | 给 `page_contents` 加 `notebook_id` 再建索引 | 回填要改写整张表（600 万行估计约一分钟、上 GB 的 WAL），每次写正文照样多改一个索引 |
  | 每次新增读出整个来源区再比 | 一步 O(来源数)：10 万个来源时约 0.3 秒一步，`batch` 加 100 个来源过了 15 秒的期限 |
  | `nodes.source_hash`（选） | 加列只改元数据；索引只含来源；只有 LLM Wiki 的新增读它，按笔记本分键；代价是单元里多一段维护与一条不变式 |

- **并发**：新增来源的写多数是改树的单元（新建、上传、移动、导入、恢复、撤销），在笔记本行的 `FOR NO KEY UPDATE` 下串行，后一个的守卫看得到前一个已提交的 `source_hash`。只有"空的来源的第一次正文"是 `FOR SHARE` 的单元，两个可以并行：守卫在判这种新增之前取这本笔记本的事务级 advisory lock（键由笔记本 id 与"来源区"派生，照 linking 的索引锁 `pg_advisory_xact_lock`，13.1 第 5 条），两个这样的写一个接一个，后一个在前一个提交之后再查。锁的次序：工作区 → 笔记本 → 正文行 → 来源区的 advisory lock → 观察者里的索引锁，所有取它的路径都在守卫里、同一个位置取，不成环。写进 13.1 第 5 条。
- **代价**：一次新增至多一次索引的探查（移入一棵子树时一条语句读出它的页的哈希与附件的 SHA-256，再一条语句查重）；P2 在 10 万个来源上量，成本守卫在 Makefile 的一行。

### 4.9 `AGENTS`、来源根与保留的标题

只在 `llm_wiki` 下：

| 对象 | 规则 | 码 |
|---|---|---|
| `AGENTS` 页（根下、标题键是 `TitleKey("AGENTS")`、类型是页，同说明的找法 `nodes.sql:84-95`） | 不能删除（含删除它的祖先：它在根下，没有祖先）、改名、移动；正文与 frontmatter 照常改（写者） | 409 `page.protected` |
| 来源根（`source_root_id`） | 不能删除（含删除它的祖先）；改名、移动要管理员（4.7）；正文写者可以改 | 409 `page.protected`、403 `page.source_admin_only` |
| 根下的标题键 `index`、`log` | 任何节点（页与附件）不能以它们为名放在根下：新建、改名、移动、恢复、导入；`list`、`read` 的路径 `index.md`、`log.md` 是虚拟的（4.13） | 409 `page.virtual_file` |

- 保留的标题对附件也保留：一个根下叫 `index` 的附件与虚拟的 `index.md` 的路径不同，但同一个标题键会让链接 `[[index]]` 与目录的名字在人看来指向两样东西；一并保留最简单（第 11 节设计的取舍第 13 条）。
- **导入与恢复加序号**：导入（`page/app/unit_import.go:172-194` 的 `free`）与恢复（`unit_restore.go:177-210` 的 `restoredNames`）按活着的兄弟给重名的加序号；LLM Wiki 的根下把 `index`、`log` 的键当作已被占（加进根的 `keys`），所以一个库里根下的 `index.md` 导入成 `index 2`、报告 `renamed`，不让守卫拒绝整个单元。导入的 `Reserved` 钩子（`transfer/app/import_ports.go:124-127`）只决定序号跳过哪些候选，不让原来的名称加序号，所以不用它。`AGENTS.md` 在根下照旧因为已有 `AGENTS` 而成为 `AGENTS 2`。
- `page.virtual_file` 的文字（中英）照总体设计 5.5："这个文件由系统生成，修改页面后目录会自动更新"（`log` 的写"日志就是变更集，会自动更新"）。
- 网页：`AGENTS` 的行没有改名、移动、删除；来源根的行没有删除，改名、移动只对管理员；`[[index]]`、`[[log]]` 的未建链接在 LLM Wiki 里打开目录视图与历史（4.16），不给"新建这一页"。

### 4.10 必填属性

- **只检查**（负责人 2026-10-24，第 11 节第 2 条）：来源区以外的页缺 `type` 或 `summary` 时，健康度面板显示为"需要注意"，MCP 的 `lint` 报出来；任何写入都不因此被拒：网页的新建页面、agent 补建的父页、导入、还原与恢复都照今天。总体设计 5.1"写入时的不变量……违反就拒绝"只说来源区、`AGENTS` 与保留的标题，5.2 的"必填"改为"lint 检查"（"总体设计的修订"）。M11 的提交闸门以它为闸（"写给后面的 M"）。
- **"缺"的定义**：frontmatter 能解析（`indexed_pages.frontmatter_valid`），照 M9 取 `type`、`summary` 的规则找键（第一个以 ASCII 不分大小写等于它的键，`linking/adapter/postgres/queries/index.sql:12-25`），然后：没有这个键，或值不是字符串，或字符串按 JavaScript 的 `trim` 去掉首尾空白之后为空（同 4.2 读 `aliases` 的空白）。frontmatter 写坏了的页只算进"frontmatter 解析失败"，不在这两条里重复。
- **豁免**：来源、来源根、`AGENTS`、虚拟的两页、附件。
- **存储**：用 `page_properties` 与 00047 的 `value_start`、`value_chars` 与两个部分索引（`00047_linking_page_properties_keys.sql:27-40`），不提升 `domain.Extractor`、不要 `nervewiki reindex`（提升让每本笔记本在重建期间不能写，30 万页估计约 330 秒，`linking/app/reindex.go:75-131`）。只有空白的判断：`value_start` 是前 257 个字符，值更长而前 257 个都是空白时读整个的值（这样的行极少，界在一次 lint 里的条目数之内）。

### 4.11 lint

**规则**（总体设计 5.6；"适用"一列：通用的两种模式都有）：

| 规则（码） | 适用 | 定义 | 条目 |
|---|---|---|---|
| 待建页面（`wanted`） | 通用 | 解析不到的链接，按**写的目标**分组（目标的每一段取标题键、照 4.4 去掉 `.md` 的写法；`./`、`../` 的按链接它的页的文件夹改成从根起），数链接它的不同的页；读作附件的目标（最后一段带不是 `.md` 的扩展名，`linking/domain/target.go:105-129`）不算：它们是缺的文件，不是待建的页；LLM Wiki 下根下的 `index`、`log` 不算（虚拟页面）；按页数降序、再按目标 | 目标（写下的第一种写法）、页数 |
| 孤儿页（`orphans`） | 通用 | 没有别的页的链接解析到它的页（自己的不算，`resolved_asset` 的不算）；豁免来源、来源根、`AGENTS`、虚拟页面、附件 | 页 |
| 链接有歧义（`ambiguous`） | 通用 | `page_links.ambiguous`：几页在每一项偏好上都相同、按 id 选了一页（`linking/domain/resolve.go:335`，同 4.4）；5.6 的说法"匹配到多个节点"改正 | 链接的页、目标 |
| frontmatter 解析失败（`invalid_frontmatter`） | 通用 | `indexed_pages.frontmatter_valid = false` | 页 |
| 缺类型（`missing_type`）、缺摘要（`missing_summary`） | LLM Wiki | 4.10 | 页 |
| 未消化的来源（`undigested_sources`） | LLM Wiki | 来源区以外没有任何页以正文链接（`property_key IS NULL`）或 `sources` 属性链接（属性路径的第一段以 ASCII 不分大小写等于 `sources`，取这一页第一个这样的键，同 M9 的第一个键的规则）解析到它的来源（页与附件）；`AGENTS` 与来源根在来源区以外，它们的链接也算（5.4"来源区以外只要有页面引用了它"）；按新建的时刻 | 来源 |
| `sources` 指向无效（`invalid_sources`） | LLM Wiki | 来源区以外的页的 `sources` 属性链接解析不到，或解析到来源区以外（含来源根） | 页、目标 |

- 属性路径不记深度（`platform/markdown/frontmatter.go:27-29`）：叫 `sources.0` 的顶层键与 `sources` 的第 0 项在索引里相同；不加引号的 `[[x]]`（YAML 的嵌套列表）、块标量与别名的值不是属性链接（`tools/md-fixtures/README.md:107-109`）。都照今天的索引读，要改得提升提取器，M10 不改；写进 `AGENTS` 模板的 frontmatter 写法（带引号的字符串）。
- **算法**：读的时候算，没有写的时候维护的计数（负责人 2026-10-24）。一次运行一个 `REPEATABLE READ READ ONLY` 的快照（`postgres.WithinSnapshot`，同导出）：各条规则、来源区的节点、路径看到的是同一个时刻；快照占一个连接到它结束。来源区的节点由 page 的端口读一次（来源根的子树，10 万个节点约 70 ms 量级），以数组交给 linking 的语句。
- **过时的行**：快照里正文与索引在同一个事务里写（观察者），版本不会对不上；对不上的只有两种：提取器不是当前的（升级改了规则、还没 `nervewiki reindex`）与没有索引行的页（M6 之前写的）。这样的页跳过、计数，答复带 `stale_pages` 与一句"运行 `nervewiki reindex`"；不即时解析（30 万页约 2 分钟的 CPU，M6 移交注意第 1 条许可"跳过"）。
- **数目与条目**：每条规则的数目数到 `MaxLintCount` = 10,000 就停、答"10,000+"（`count_capped`），这样的数目照 M8 的搜索是近似的上界；条目照规则的次序游标分页（`EncodeCursor`，载荷是规则的排序键与 id），`limit` 1–100、缺省 50。摘要（全部规则）每条给前 5 项。路径只为答出的一页读（page 的 `NodePaths.Paths`）。
- **成本**（调查估计，P3 量）：30 万页时每条规则 0.03–3 秒，加上来源区 150–300 ms；没有索引的规则（未解析、歧义、按笔记本的属性链接）读整本的链接。P3 先量，**只给量过、超出预算的规则加部分索引**（4.2 末行）；预算：10 万页时一次摘要至多 2 秒，30 万页时每条规则的计划测试走索引。一次运行的读至多是这本笔记本的索引，与读者看不到的内容无关（13.1 第 4 条）。
- **语义检查的范围**（5.6）：lint 的 playbook 让 agent 以 `log` 取最近 N 个变更集碰过的页，系统不另设规则。
- **阻断与提示**（5.1）：M10 里每条都是提示（"需要注意"），没有阻断：必填属性只检查（负责人 2026-10-24）；M11 的闸门在它自己的提交里以其中几条为闸。

**接口**：

- REST（wiki）：`GET /api/v0/notebooks/{notebook_id}/lint`（`getLint`：适用的每条规则的数目、是否到顶、前 5 项，`stale_pages`，算的时刻）与 `GET /api/v0/notebooks/{notebook_id}/lint/{rule}`（`listLintItems`：一条规则的一页条目与游标）；操作 `lint.read`（读者以上，规则表一行）。普通笔记本问 LLM Wiki 的规则答 404 `wiki.lint_rule_not_found`（不在这个模式的契约里）。
- MCP（wiki，两个入口）：`lint`（`rule?`、`limit?`、`cursor?`：没有 `rule` 是摘要，带上是那条规则的条目；条目带路径）；`pending_sources`（`limit?`、`cursor?`：未消化的来源，每项路径、类型、字节数、新建的时刻；就是 `undigested_sources` 那一条，给 ingest 的 playbook 一个直接的名字），普通笔记本答 409 `wiki.not_llm_wiki`。两者的答复经 `Fit` 至多 48 KiB。

### 4.12 健康度面板

- **谁看得到**：能读这本笔记本的人（负责人 2026-10-24，第 11 节第 4 条）。两种模式都有：通用规则两种模式都算，LLM Wiki 加它自己的。
- **在哪里**：左栏 `NotebookPlaces`（`web/…/notebook-places.tsx:15-29`）在历史之前加"健康"，路由 `/:slug/notebooks/:id/health`；不是设置的一节（读者也要看），也不挤进页面的右栏（它是整本的）。
- **算的时机**：打开时算一次（`getLint`），页上写"算于 HH:MM"与"重新检查"；没有定时刷新、不随事件重算（一次运行在大笔记本上要秒级，负责人：打开时算、不维护计数）。
- **样子**：每条适用的规则一张卡：名称与一句说明、数目（到顶时"10,000+"）、前 5 项（页的链接、目标的文字与页数），"更多"按 `listLintItems` 的游标接着读（13.2 第 19 条的游标列表）；数目为 0 的规则收在"没有问题"的一行里。过时的页多于 0 时顶上一条提示。

### 4.13 虚拟的 `index` 与 `log`

**认出它们的一处**：`domain.FindPath` 与 `WalkPath` 的查找（`Lookup`）由一个按契约的包装给出：LLM Wiki 下根下（祖先为空）查标题键 `index` 或 `log` 时，不查库，答 `ErrVirtualFile{Name}`（`page.virtual_file`，带 4.9 的文字）。MCP 的每个按路径的工具经 `NodePaths.Find`/`Walk`，`batch` 的计划经它模拟的树上的同一个查找（`page/domain/batch_plan.go:361`、`:403`、`:427-431`、`:493`），所以写、`edit`、`move` 的两端、`delete`、`versions`、`backlinks`、`log {path}`、`index {parent}` 与 `batch` 的每一项都答这个码；`write` 新建 `index.md` 不再建一页真的（今天会，`page/app/write_path.go:105-170`），`edit`、`delete` 不再答 `page.not_found`。`read` 与 `list` 认出它、改为给出内容与虚拟的项。

**MCP**：

- `list`：根的子节点（`pattern` 为空或 `/`）在最前面列出 `index.md`、`log.md`，项带 `virtual: true`，没有版本令牌、字节数与最后改动；glob 照路径匹配它们（`*.md`、`**/*.md`、`index.md` 都配得上）；`open_notebook` 的根下的节点同样。
- `read`：给生成的 Markdown，按 `read` 的行切（`offset`、`column`、`limit`，`page/domain/read.go:67` 对任何字符串都成立）；元数据带 `virtual: true`，没有版本令牌。每次调用重新生成：两次之间别人写了，行号会挪；说明里写明。
- `grep`、`search` 不搜它们（候选来自搜索的索引，它们不在树里）；`grep` 与 `index` 的工具说明写明"虚拟的两页不在其中，读 `index.md` 或用 `index`"。

**内容**（wiki 生成，`domain` 里的纯函数，有表格测试）：

- `index.md`：总体设计 5.5 的分组：有 `type` 的按类型分组（类型按标题键排，组里按路径），没有 `type` 的按父页分组（父页按路径排）；每项一行：`- [标题](路径) — 摘要`，再带标签、更新时间、入链数。数据是 M9 的 `ListIndex` 的那一份（路径、类型、摘要、标签、时刻、入链数到 1,000，字符串截到 256 字节、标出截断），承袭它的界线（索引里没有行的页不在其中）；它的次序（类型的键、再按 id，`linking/domain/index.go:27-61`）不是 5.5 的分组，所以文件先读全部索引的页的类型键（`ListIndex` 每次本就读它们）与树，在 Go 里排出 5.5 的次序，再为前 `MaxIndexFileItems` = 500 项读条目。超过时末尾一行说"共 N 页，这里列出前 500 页；用 `index` 工具按类型、标签、父页筛选与翻页"。文件至多 40 KiB（一次 `read` 读得完）。代价同 `ListIndex` 的一次调用加树：10 万页约 1.1 秒，成本守卫在 Makefile 的一行。
- `log.md`：最近 `MaxLogFileChangesets` = 50 个变更集，新的在前：每个一段，时刻、类型、执行者与客户端、说明（截到 500 字节）、ingest 的来源（至多 5 个的路径）、前三项变化（路径）；数据是 `ListChangesets`（`page/app/list_changesets.go:51-197`）的第一页。末尾一行指向 `log` 工具。至多 40 KiB。
- 两个文件都是英文的固定格式（agent 读它们，同 MCP 的文字是英文的，M9/P1）；标题与说明是写者的文字，原样。

**网页**：页面树的虚拟行在客户端由 `Notebook.mode` 生成（`web/…/page-tree.tsx:145-151`），不进服务端的树、不进 `["pages", 笔记本]` 的 store（十几处读树的地方不受影响）：根下最前面两行，`index`（目录的图标）打开目录视图，`log`（历史的图标）去历史页；不能拖、不能放、没有菜单。

**导出**（[M7 的移交](handoffs/M7-transfer.md)第 1、2、6 项）：wiki 登记 `transfer.ExportContributor`。只在整本导出（`Scope.RootID` 为空）、这本笔记本是 `llm_wiki`（在导出的快照里读）时，`Add("index.md", …)`、`Add("log.md", …)`，放在库的根下；子树的导出什么都不加（子树没有"根下的目录"，也就不会与一个叫 `index` 的子树根冲突）。LLM Wiki 的根下不会有叫 `index`、`log` 的真实节点（4.9），整本导出不会 `contributor_conflict`。导出用的上限比 `read` 大：`index.md` 至多 10,000 项、2 MiB，`log.md` 至多 1,000 个变更集、1 MiB，各自末尾说明截断；至多两个文件，整个贡献者在 10 万页上约 1–2 秒（P4 量），写明上限、由贡献者守住、测试钉住（13.1 第 31 条；M7 移交第 6 项：不加流式写入）。贡献者的错误不带名称（13.1 第 10 条）。
- **导入**：读到 `meta.json` 的 `contributed` 时跳过这些路径（`transfer/app/import.go:221`，精确的 NFC 路径，只认第一个 `meta.json`），重新导入一个 LLM Wiki 的导出不会多出 `index`、`log` 两页；没有 `meta.json` 的库（或它超过 16 MiB）把根下的 `index.md`、`log.md` 当作页导入：进 LLM Wiki 的根下时加序号（4.9），进普通笔记本照常。P4 的最后一跳核对这两条。

### 4.14 ingest 标记

- **定义**：一个 `edit` 类型的变更集 C 是一次 ingest，当且仅当它新引用了原本未消化的来源 S：C 写了正文的某一页 P 在来源区以外，C 之后 P 以正文链接或 `sources` 属性链接（4.11 的"消化"）解析到 S，C 之前 P 没有；并且除了 C 里这样新引用 S 的页，来源区以外没有别的页引用 S。导入、还原版本、从回收站恢复与撤销的变更集不是 ingest（不是 agent 或人消化来源的动作）。C 记下这些来源。
- **近似**：别的页（C 没写正文的）按 C 之后的树与解析判断：C 里的改名让别的页的一条链接改为解析到 S、或不再解析到 S，C 里的移动让一页越过来源区的边界，都按之后的算（调查 §8：重新解析看不到页越过边界）。这几种都要 C 同时改树与写正文，按之后算只会少标或多标一个历史的标签，不影响任何规则；写进第 10 节。
- **在哪里算**：linking 的索引观察者里（4.1）。它在锁下替换写了正文的页的行时拿到旧行（`ReplacePage` 交回旧行的解析与属性路径，今天只交回目标、没有属性路径，`linking/adapter/postgres/store.go:61-71`、`:151-172`，加上），解析之后读新行；只对 `llm_wiki` 的 `edit` 单元做：每页 P 新得到的、解析到来源区的消化的链接 → 候选；每个候选查来源区以外还有没有别的页引用它（一条语句，按 `resolved_id` 的部分索引）。为此组合根交给 linking 的值带上 `ChangesetID`（今天在组合根丢掉，`bootstrap/linking.go:25-38`）、`Kind` 与 `Notebook`；来源区的判断经 linking 已有的 page 端口（祖先）。结果经 page 只凭连接池的 `page.NewIngestMarks(pool)` 写进 `changeset_sources`（新的行，`seq` 是第一次记下的次序）并加 `changesets.ingest_count`，都在单元的事务里、索引的锁下。
- **并发**：同一本笔记本的两个单元在索引的锁上串行（13.1 第 5 条）：后一个在前一个提交之后才读，看得到前一个新写的引用，所以同一个来源不会被两个变更集都记成"新引用"。
- **编辑会话**：一次会话的几次保存并入同一个变更集（`page/app/unit_content.go:70-75`），每次保存各自判，标记只增不减（`changeset_sources` 的主键去重，同 `renames`、`moves` 的"一旦为真就留着"）：会话里先加了引用又删掉，标记留着。
- **读**：`listChangesets` 加筛选 `ingest=true`（走 ingest 的迁移的部分索引），每条带 `ingest_count` 与前 3 个来源；`getChangeset` 带 `ingest_count` 与前 20 个来源（id；网页按树写路径）；MCP 的 `log` 与 `changeset` 同样，来源带路径（回收站里的为 `null`）。每个读者看得到（`page.history` 是读者的，负责人 2026-10-24 的"可见性"照 M8 的历史）。
- **网页**：历史的每一行在标记那一行（`web/…/history-row.tsx:96-117`）加"ingest · N 个来源"，筛选加"只看 ingest"（`history-filters.tsx:94-155` 今天按人、客户端、页），变更集页列出来源的链接；`kindText` 不变（ingest 是标记，不是新的类型：一次 ingest 同时是编辑或 batch）。

### 4.15 MCP：工具、prompt 与说明

**工具**：`lint` 与 `pending_sources`（4.11），由 wiki 的模块根 `Tools()` 交出、组合根的 `mcpTools` 拼接（`bootstrap/mcp.go:43-48`），照 M9 的每个工具：先写进 `api/mcp/tools.json`、权限矩阵有行、码进 `mcpToolCodes()`、答复经 `Fit`、注解只读、内置的说明有它们的一行（[M9/P1 的移交](handoffs/M9-P1-mcp-registry.md)第 8 条）。注册表是静态的（`listChanged: false`）：两个工具在两种模式的笔记本里都列出，`pending_sources` 在普通笔记本里答 409 `wiki.not_llm_wiki`，说明写明"LLM Wiki only"（第 11 节设计的取舍第 20 条）。

**prompt**（总体设计 5.9）：

- **三个**，英文（负责人 2026-10-24，第 11 节第 3 条，与工具的说明一致）：
  - `ingest`（`source?`：来源的路径；没有时先 `pending_sources`）：读来源 → 照 `AGENTS` 的约定定要写与要改的页 → 一个带说明（`ingest: <来源>`）的 `batch` 写完、每页的 frontmatter 带 `type`、`summary` 与 `sources` → `lint` 核对；
  - `query`（`question`，必填）：`index.md` 或 `index`、`search`、`grep`、`backlinks`、`read` 找答案，答复引用页与来源，值得留下的答案另写一页；
  - `lint`（无参数）：`lint` 的摘要 → 能机械修的（缺类型、缺摘要、无效的 `sources`）以 `batch` 修 → 语义检查取最近的变更集（`log`）碰过的页 → 报告。
  每份至多 16 KiB，写法照 M9 的人工清单记下的 agent 的毛病（小的改动用 `edit` 不用整页的 `write`，`move` 的一步，`partial_lines`、`cut` 与 `read(offset, column)`，[M9/P1 的移交](handoffs/M9-P1-mcp-registry.md)第 6 条）；清单在 v0.1 的最后才跑，跑过之后照记下的改。
- **只在笔记本入口**：`mcpserver.Prompt` 加 `Entries`（同 `Tool` 的入口），今天的 `addPrompts` 两个入口都登记（`platform/httpserver/mcpserver/prompt.go:47`）。账户入口没有笔记本，prompt 要另带 `notebook` 才有意义，M10 不做。能力声明照旧：笔记本入口有 `prompts`、`listChanged: false`，账户入口没有。
- **普通笔记本**：`prompts/list` 照样列出（静态），`prompts/get` 答 JSON-RPC 的错误（`GetPromptResult` 没有 `isError`，`sdk/mcp/protocol.go:966-985`）：码 -32602，`message` 是 `wiki.not_llm_wiki: <detail>`（同工具的错误文字），`data` 是 `{"code": …}`。
- **错误与上限**（[M9/P1 的移交](handoffs/M9-P1-mcp-registry.md)第 5 条）：`promptHandler` 的错误经 `httpserver.ProblemOf` 分类：可预期的（4xx）答 -32602 与上面的形状，别的答 -32603 并记 error（今天一律 -32603，`prompt.go:59` 起）；答复的消息合计至多 48 KiB（三份是静态的文字加至多 4,096 字节的参数，表格测试在上限处核对；超过时答 -32603，是缺陷）。请求体同别的方法至多 64 KiB（`mcpserver/edge.go:79-128`）：`source` 是路径（至多 4,096 字节）、`question` 至多 4,096 字节，放得下，不另设例外。参数照旧是字符串、`Required` 由 mcpserver 核对（SDK 不核对，`sdk/mcp/server.go:900-916`）；不加 `CompletionHandler`。
- **模块根**：wiki 的 `Prompts()`（12.4 的"以后 `Prompts()`"），组合根拼接、交给 `mcpserver.New` 的 `Options.Prompts`（今天组合根不传，`bootstrap/mcp.go:85-93`）。
- **契约**：先写的 `api/mcp/prompts.json`（`{"notebook": [...], "account": []}`，每个数组是那个入口的 `prompts/list` 的 `prompts`），`TestThePromptListsAreTheContract` 比较整个对象，有反向对照（少一个、多一个、说明不同）。不加进 `tools.json`：今天有五处假定它恰好两个键（`bootstrap/mcp_tools_test.go:79-117` 的 `toolListDiff`、`:175-187` 的 `unnamedTools`，`bootstrap/permission_matrix_mcp_test.go:113-135`、`:382-388`，`e2e/stories/mcp/mc1-connect.spec.ts:22-25`）。
- **最后一跳**：经组合根拼起的三个 prompt 经两个入口的 `prompts/list`、`prompts/get` 到达处理函数（文字里有 playbook 的第一行），组合根不交 `Prompts()` 时失败；e2e 的 MCP 客户端加 `prompts/list`、`prompts/get`（`e2e/fixtures/mcp.ts:115-156` 今天只有工具）。

**说明**（M9 4.6）：

- **按模式拼，每次拼**：笔记本入口的握手、账户入口的 `open_notebook` 经同一个函数给说明（今天两处各自选内置的文字，`page/mcp_entry.go:36-45`、`page/adapter/mcp/open_notebook.go:59-90`）。LLM Wiki 下是 `AGENTS` 的正文（照 M9 去掉 frontmatter、截断）加一段系统的说明（英文，至多 512 字节）："This notebook is an LLM Wiki: pages under `<来源根的路径>/` are sources you add once and never change; index.md and log.md are generated; run the prompts ingest, query and lint."；截断时给这一段留出位置（`AGENTS` 截到 32 KiB 减去它）。来源根的路径每次握手读一次（一次点查）。缓存照旧只存 `AGENTS` 的正文、键是（笔记本、节点、版本号）（`page/app/instructions.go:36-87`）：系统的那一段每次拼，降级之后的下一次握手就没有它，不用让缓存失效。`AGENTS` 的正文被写成空的时，照 M9 给内置的说明，再加这一段。
- **内置的说明的余量**：账户入口 4,027 字节、笔记本入口 3,957 字节（`page/adapter/mcp/instructions.go:31-74` 的 `pathsAndTools` 共用 3,645 字节），离 4 KiB 剩 69 与 139 字节。`lint`、`pending_sources` 的两行加在共用的部分，先缩短已有的行（P3）；`TestTheBuiltInInstructionsAreShort`（`page/adapter/mcp/instructions_test.go:15-27`）照旧守 4 KiB，`TestTheBuiltInInstructionsNameEveryTool` 守每个工具都有一行。"upgrade" 在说明里今天指服务端的升级（`instructions.go:52-53`），M10 不加这个词。
- **`AGENTS` 必须有**（总体设计 5.2）：模板建它（4.4），守卫不许删除、改名、移动它（4.9）。

### 4.16 网页

| 部分 | 做法 |
|---|---|
| 建笔记本 | 对话框加模式（普通、LLM Wiki，默认普通，一句说明）；请求带 `mode` 与界面语言的 `template_locale`；新手引导不变 |
| 设置 | "常规"加"模式"一节：读者看到模式；管理员在 LLM Wiki 下有降级与确认对话框（4.5）；`useFollowRole` 照管理员的读的做法收起 |
| 页面树 | 图标的最小一组（lucide，`web/apps/web/package.json:31` 已有）：普通的页不加；来源根、来源、`AGENTS`、虚拟的 `index` 与 `log` 各一个，所以树里一眼认得出特殊的行（今天一个图标也没有，`web/…/page-tree-item.tsx:8`）；虚拟的两行（4.13）；来源与来源根的行按角色收起拖动与菜单（写者在来源上只读；来源根的菜单有"添加来源"），`AGENTS` 没有改名、移动、删除；判断在客户端，来源区由 `source_root_id` 与已加载的树算 |
| 添加来源 | "粘贴或上传为来源"的对话框，写者在来源根与来源区的页上打开：粘贴（标题与 Markdown）经 `createPage` 带 `content` 一次写成（`api/modules/page.yaml:1998-2017` 已收 `content`）；上传照附件的上传（同一个父页）；选了 `.md` 文件时在客户端读出它的字节（`TextDecoder` 留着 BOM、非 UTF-8 拒绝）、照粘贴写成页（`.md` 不能作附件，总体设计 3.7）；`page.source_duplicate` 显示已有那一个的路径与链接 |
| 来源页 | 阅读视图，没有"编辑"、`Ctrl+E` 不开会话，顶上一行"来源只能新增，不能修改"；来源区的页把它的附件在正文之下完整列出（分页"更多"，总体设计 3.7 的例外，M7 移交第 4 项），不只放在右栏的附件一节；在来源区里"新建页面"建空的页、停在阅读视图（它是文件夹） |
| 健康度 | 4.12 |
| 目录视图 | `/:slug/notebooks/:id/index`，两种模式都有（总体设计 5.2"目录和日志在界面与工具中查看"）：按 5.5 分组（有类型的按类型、没有的在"未分类"里），可按类型、标签、父页筛选，"更多"翻页；数据是新的 REST 目录（4.1，linking 的 `ListIndex` 原样，`index.list`）。普通笔记本从 `NotebookPlaces` 的"目录"进，LLM Wiki 另有树里的虚拟行。代价：一个 REST 操作与一页，`ListIndex` 每次读整本的索引（10 万页约 1 秒，同 MCP 的 `index`） |
| 历史 | ingest 的标记、筛选与来源（4.14） |
| 链接 | 未建的 `[[index]]`、`[[log]]` 在 LLM Wiki 里打开目录视图与历史（`web/…/reading/unresolved-link.tsx:115-157` 今天会提议新建一页） |
| 事件 | `mode`（4.5） |
| 文字 | 新码（`page.source_immutable`、`page.source_admin_only`、`page.source_duplicate`、`page.virtual_file`、`page.protected`、`notebook.not_llm_wiki`、`wiki.not_llm_wiki`、`wiki.lint_rule_not_found`）、失败 `refused`、问题 `duplicate_source` 的中英文字（`problem-messages.test.ts` 要每个契约的码都有两种语言的文字，13.2 第 11 条）；假服务端与 `web/test/fakes.ts:95` 的 `Notebook` 加 `mode` |

### 4.17 权限、码的次序、加锁与交错

- **操作与规则表**：`notebook.change_mode`（笔记本级，管理员）、`lint.read`（笔记本级，读者以上，wiki 的 `Actions()`）；目录的 REST 用已有的 `index.list`；`pending_sources` 是 `lint.read`。来源区的管理员规则不是规则表的行：写者能做的操作（`node.move`、`node.delete`、`node.rename`）落在来源上时由守卫按 `Write.Role` 判（守卫看得到角色之后，13.1 第 3 条"判定返回的 `Grant` 带锁下读到的角色，用例拿它做比较"的做法）。权限矩阵加新操作的行，矩阵另有 LLM Wiki 笔记本的变体：来源上的改名、移动、删除对写者答 `page.source_admin_only`、对管理员答成功。
- **码的次序**：新操作照 13.1 第 4 条（404 → 403 → 422 → 409）；`downgradeNotebook`：404 → 403 → 409 `notebook.not_llm_wiki` → 409 `transfer.busy`；`listLintItems`：游标 400 → 404 → 404 `wiki.lint_rule_not_found` → 422 `limit`；守卫的码在领域的码之后，模式在锁之前（4.6）；`batch` 里每一项照"领域 → 模式 → 锁"，按项的次序答第一个被拒的。
- **加锁**：降级同笔记本的管理写；模板的单元在建笔记本的事务里重入它已持的锁（4.4）；来源区的 advisory lock 在正文行之后、索引锁之前（4.8）；lint 与虚拟文件不加锁（快照）。
- **交错**（13.4 第 4 条，确定性的测试，结束时 `checkPages`、`checkLinks`）：降级与一个正文写（两种次序：写看到的模式就是它判的）；降级与开始导入（拒绝或之后是普通的）；两个空的来源同时写入相同的第一次正文（后一个答 `page.source_duplicate`）；建 LLM Wiki 笔记本与同一工作区的另一次建；两个单元同时新引用同一个未消化的来源（只有先提交的是 ingest）；lint 的快照与并发的写（结果是快照那一刻的）。
- **日志**（13.1 第 10 条）：lint 每次运行一行 info（`notebook_id`、规则、每条的耗时与数目、过时的页数），不记标题与目标；降级一行 info；模板失败的错误不带名称；守卫的拒绝是普通的 4xx，不另记。

### 4.18 上限

| 量 | 上限 | 处 |
|---|---|---|
| lint 的数目 | 每条数到 10,000（`MaxLintCount`），之后"10,000+" | 4.11 |
| lint 的条目 | `limit` 1–100、缺省 50；摘要每条 5 项；MCP 答复 48 KiB | 4.11 |
| `index.md` | 500 项、40 KiB（`read`）；导出 10,000 项、2 MiB | 4.13 |
| `log.md` | 50 个变更集、40 KiB；导出 1,000 个、1 MiB；说明截到 500 字节 | 4.13 |
| 导出贡献者 | 至多 2 个文件，字节同上 | 4.13 |
| ingest 的来源 | 全部记下（`changeset_sources`）；历史的一行 3 个、变更集 20 个、`log.md` 5 个 | 4.14 |
| playbook | 每份 16 KiB；`prompts/get` 的答复 48 KiB；参数 4,096 字节 | 4.15 |
| 说明的系统段 | 512 字节 | 4.15 |
| 模板 | `AGENTS` 8 KiB | 4.4 |

新加的、由写者的内容决定数量的东西各在上限处有测试（13.1 第 31 条）。

## 5. 接口

**REST**（先写 `api/`，再写代码；新的 4 个操作，加上改的）：

| 操作 | 方法与路径 | 模块 | 说明 |
|---|---|---|---|
| `getLint`（新） | `GET /api/v0/notebooks/{notebook_id}/lint` | wiki | 适用的规则的数目、是否到顶、前 5 项，`stale_pages`、算的时刻 |
| `listLintItems`（新） | `GET /api/v0/notebooks/{notebook_id}/lint/{rule}?limit=&cursor=` | wiki | 一条规则的一页条目；`undigested_sources` 就是未消化的来源 |
| `listIndex`（新） | `GET /api/v0/notebooks/{notebook_id}/index?type=&tag=&parent_id=&limit=&cursor=` | linking | M9 的 `ListIndex`（MCP `index` 的同一份），目录视图用 |
| `downgradeNotebook`（新） | `POST /api/v0/notebooks/{notebook_id}/downgrade` | notebook | 4.5 |
| `createNotebook` | `NotebookCreate` 加 `mode`（`standard`、`llm_wiki`，缺省 `standard`）、`template_locale`（`zh-CN`、`en`，缺省 `en`） | notebook | 4.4 |
| `Notebook`（列表、读、建、改、降级的答复） | 加 `mode`、`source_root_id` | notebook | 4.3 |
| `listChangesets`、`getChangeset` | 筛选 `ingest`；每条 `ingest_count` 与来源 | page | 4.14 |
| 页面的写操作（新建、改名、移动、删除、写正文、勾选任务、开启会话、上传、还原版本、恢复、撤销、`startRestore`、`startRevert`） | `x-problem-codes` 加它们可能答的模式的码 | page、asset、transfer | 4.6–4.9 |
| `common.yaml` 的 `Problem` | 加 `duplicate_of`（只有 `page.source_duplicate` 带它） | — | 4.8 |
| transfer 的 `Report` | 失败 `refused` 与 `refusal: {code, node_id}`；导入的问题 `duplicate_source` | transfer | 4.6、4.8 |
| `events.yaml` | `EventMode` | events | 4.5 |

**MCP**：

| 工具 / prompt | 参数 | 答复或内容 | 入口 | 模块 |
|---|---|---|---|---|
| `lint` | `rule?`、`limit?`、`cursor?` | 摘要，或一条规则的条目（带路径）与游标；`stale_pages` | 两个 | wiki |
| `pending_sources` | `limit?`、`cursor?` | 未消化的来源（路径、类型、字节数、时刻）与游标；普通笔记本 `wiki.not_llm_wiki` | 两个 | wiki |
| `list`、`read` | 照旧 | 加虚拟的 `index.md`、`log.md`（`virtual: true`） | 两个 | page（内容由 wiki） |
| 写的工具与按路径的读 | 照旧 | 虚拟路径答 `page.virtual_file`；模式的码 | 两个 | page |
| `log`、`changeset` | `log` 加 `ingest?` | 加 `ingest_count` 与来源 | 两个 | page |
| `notebooks`、`open_notebook` | 照旧 | 加 `mode`；`open_notebook` 的说明照 4.15 拼 | 账户 | notebook、page |
| prompt `ingest` | `source?` | 英文的 playbook | 笔记本 | wiki |
| prompt `query` | `question` | 同上 | 笔记本 | wiki |
| prompt `lint` | — | 同上 | 笔记本 | wiki |

契约：`api/mcp/tools.json`（加两个工具）与新的 `api/mcp/prompts.json`。新的码：`page.source_immutable`（409）、`page.source_admin_only`（403）、`page.source_duplicate`（409）、`page.virtual_file`（409）、`page.protected`（409）、`notebook.not_llm_wiki`（409）、`wiki.not_llm_wiki`（409）、`wiki.lint_rule_not_found`（404）。

规模：4 个新的 REST 操作、2 个工具、3 个 prompt，第 9 节 13 个故事，在 12.1 第 4 条的"约 20 个接口操作或 15 个故事"之内。

## 6. 从 Nerve 借鉴

Nerve 没有 MCP、prompt、数据的 lint、模式的契约或只增不改的区域（调查只读核对 `nerve-project`）。可以照搬的一样：**建的时候在同一个事务里写种子**：Nerve 建项目时在建项目的事务里写默认的状态（`nerve-project/server/internal/modules/project/app/create_project.go:113-139`），e2e 断言它们"随项目一起"写下（`nerve-project/e2e/fixtures/assert/project.ts:28-39`）。M10 的模板照这个做（4.4），e2e 的 LW1 照它断言模板的两页与笔记本同一个时刻、同一个变更集。其余沿用本项目自己的：写入单元、守卫、参与者与观察者（M4–M6）；派生数据由单元维护（M8/P5 的搜索入队）与"等于重建"的测试（M6）；导出的快照与贡献者（M7）；撤销的计划与临时名称（M8/P3、M9/P3）；MCP 的注册表与它的契约文件（M9）。

## 7. Phase 划分

| P | 名称 | 交付 | 验证 |
|---|---|---|---|
| P1 | 契约的框架（三个 Step） | **S1 模式与模板（服务端）**：00048；`Notebook` 的两列；`createNotebook` 的 `mode` 与 `template_locale`；notebook 的创建订阅者与模式变化的订阅者（两个新的扩展点）；page 的 `TreeWrites().Seed`（声明的例外）；wiki 模块的骨架（模块根、契约表、两种语言的模板、创建的注册者、`Actions()`）与组合根的次序；`downgradeNotebook`、`notebook.change_mode`、`transfer.NewWritingJobs`；事件 `mode` 与它的发布者。**S2 守卫的骨架**：`Write.Notebook`、`Write.Role`（`ShareByID`、`LockByID` 交回状态）、`page.NewModeGuard` 与它的否决者登记在锁之前；`AGENTS`、来源根的保护与根下的保留标题（导入、恢复加序号）；`page/domain` 的规则函数与预检的框架（`batch` 的计划、上传的 `Check`、撤销的核对）；守卫的拒绝标出（`replans`、`revertDefect`、transfer 的 `refused` 与报告）；每个页面写操作的 `x-problem-codes` 与 `mcpToolCodes()`。**S3 网页（一）**：建笔记本的模式、设置的模式与降级、`mode` 的处理、树里 `AGENTS` 与来源根的收起与图标、新码的文字 | 两种模式、两种语言的建与模板的原子性（注入失败）；`schema_test` 的两个检查的反例；降级的码的次序与交错；守卫在每条触发路径上的保护与保留标题的最后一跳；`batch` 指出守卫拒绝的那一项；后台的恢复、撤销、导入以 `refused` 停下；矩阵的新行；e2e：LW1、LW2、LW12（不含虚拟行的一步） |
| P2 | 来源区（三个 Step） | **S1 来源的规则**：4.7 的全部规则（正文只写一次、管理员的树操作、新增的各种），`Step.Content`、`Step.Rewrite`，`rewrite` 的迁移与撤销的计划带上它，来源的会话否决者，预检里补上来源区；**S2 去重**：`source_hash` 的迁移、单元维护它、降级的 `Forget`、守卫的查重（页、附件、单元里、移入的子树）、来源区的 advisory lock、导入跳过重复（`duplicate_source`）、problem 的 `duplicate_of`、`checkPages` 与"等于重建"；**S3 网页（二）**：添加来源的对话框、来源页的只读与附件的完整列出、来源行按角色收起、撤销与报告的文字 | 4.7 的表的每一格经 REST 与 MCP 的整个程序测试（含历史与后台任务）；撤销里临时的步、移动加改名、先恢复再写回、`batch` 的合成各有测试；去重的交错；`source_hash` 的性质测试；去重在 10 万个来源上的成本守卫；e2e：LW3–LW7 |
| P3 | lint 与健康度（两个 Step） | **S1 服务端**：linking 的各规则的查询（先量，按量的结果加部分索引）、wiki 的 `Lint` 在一个快照里、`getLint` 与 `listLintItems`、MCP 的 `lint` 与 `pending_sources`、内置的说明缩短并加两行；**S2 网页**：健康度面板 | 4.11 的每条规则在两种模式下的表格测试；REST、MCP 结果相同；10 万页的成本守卫、30 万页的计划测试（13.4 第 8、9 条），各在 Makefile 的一行；内置的说明在 4 KiB 之内；e2e：LW8 |
| P4 | 虚拟页面与 ingest（三个 Step） | **S1 虚拟的 `index`、`log`**：路径的一处把关（MCP 的工具与 `batch` 的计划）、`list` 与 `read`、wiki 的两个生成器、导出贡献者（上限与最后一跳，`bootstrap/transfer_test.go:244-250` 钉住的清单随之改、`metaPaths` 读 `contributed`）、导入的跳过与加序号的核对、linking 的 `listIndex`；**S2 ingest**：组合根交给 linking 的值带 `ChangesetID`、`Kind`、`Notebook`，`ReplacePage` 交回旧行的属性路径，判定与 `page.NewIngestMarks`，ingest 的迁移，历史的读与筛选，MCP 的 `log`、`changeset`；**S3 网页（三）**：虚拟的两行、目录视图（两种模式）、历史的 ingest、`[[index]]` 与 `[[log]]` 的去处 | 虚拟路径在每个工具与 `batch` 上的测试；两个文件的内容与界线；`index.md` 在 10 万页上的成本守卫；贡献者的最后一跳；ingest 在真的库上的判定（4.14 的每一条）与交错；e2e：LW9、LW10、LW13，LW12 加虚拟行消失的一步 |
| P5 | playbook、说明与收尾的故事（一个 Step） | mcpserver 的 `Prompt.Entries`、错误的分类与答复的上限；wiki 的 `Prompts()` 与三份 playbook；`prompts.json` 与它的检查；按模式拼的说明（一个函数给握手与 `open_notebook`）；e2e 的 MCP 客户端加 prompt 的方法；LW11；README 的 LLM Wiki 一节（模式、来源区、虚拟文件、playbook、降级）；总体设计随之的修订 | prompt 的最后一跳与反向对照；说明的测试（G16 的换法加降级）；全部 e2e 通过（本 M 与之前所有 M） |

**大小与绿**：P1、P2、P4 各分三个 Step，可以分 Step 合并，每个 Step 合并时门禁全绿、带它自己的故事（网页的 Step 在它的服务端之后，12.1 第 5 条：前端不出现调用尚不存在接口的入口：虚拟的两行与目录视图都在 P4，P1 的树只收起 `AGENTS` 与来源根的操作）。P3 中等，P5 较轻。共 12 个 Step，比 M9（11 个，只有一条网页的文字）多了网页的四个 Step，与 12.2 的"中偏重"相符。不挪给 M11：M11 依赖的正是这里的契约（守卫、lint、来源区）。可以挪进 v0.1 之后的打磨的，在 P4 超出时由负责人定：普通笔记本的"目录"导航入口（视图本身 LLM Wiki 要）、来源区页面完整列出附件的分页（先在正文下列出前 100 个）、图标再多一组。收尾：三位 Opus 审查者并行（服务端、网页与 e2e、完成标准与文档），然后 Opus 核对修复。

**提交说明**（M9 总设计第 11 节第 111 条：从 M10 起每个 M 在 00 号设计里定一种）：M10 回到[文档约定](../../README.md)的写法：代码 `<范围>: <说明> (M10/P<n>/S<m>)`（例如 `server: add the notebooks' mode (M10/P1/S1)`），审查修复 `(M10/P<n>)`，修复的核对 `(M10/P<n> fix check <k>)`，收尾 `(M10 closeout)`；文档 `docs: <说明> (M10/P<n>)`，M 级的 `(M10)`；合并 `Merge branch '<分支>': <说明> (M10/P<n>/S<m>)`，分支名 `m10-p<n>-s<m>-<slug>`。理由：约定的写法是成文的那一种，范围说出改动在哪个部件，`(M10/P1/S1)` 在末尾便于按 M、P、S 找；M9 的写法是没有照约定（收尾审查 C-N6），不是另定的规则。

### 移交的落实

| 移交 | 项 | 落实 |
|---|---|---|
| [M5 实时推送的事件类型](handoffs/M5-events.md) | 笔记本模式的变化 | 类型 `mode`，按笔记本过滤，只带 id；在写模式的事务里经组合根的适配发出；契约 `EventMode`；前端的处理表与 `refreshedOnConnect`；替身类型 `mode` 换成真的；最后一跳：降级经 REST 之后另一条流收到帧（帧按契约核对）、前端的处理经组合根的表（`app/event-stream.test.tsx`），组合根交空时失败；工作区级的缺口不涉及（4.5，P1 的 S1、S3）。M11 的同一份照它 |
| [M6 lint 的数据在链接索引里](handoffs/M6-links.md) | 表的六行 | 待建：改为按写的目标分组、数页（只按 `target_key` 会把 `[[a/x]]` 与 `[[b/x]]` 合在一起）；孤儿：反链的查询加豁免；歧义：`ambiguous`，保留代码与 4.4 的含义、改正 5.6 的说法；解析失败：`frontmatter_valid`；`sources`：`property_key`，第一个键的规则，深度不记照旧；缺类型、缺摘要：`page_properties` 加 00047（4.10、4.11，P3） |
| 同上 | 注意 1–4 | 1：快照里只剩两种过时（提取器、没有行），跳过、计数、说出 `nervewiki reindex`；2：M10 不加提取的东西、不提升 `domain.Extractor`；3：linking 的 SQL 不连 `nodes`，来源区与路径经 page 的端口；4：来源区由模式与 `source_root_id` 定（4.3、4.11，P3） |
| [M7 导出贡献者、附件与来源区](handoffs/M7-transfer.md) | 1–6 | 1：wiki 登记贡献者，`bootstrap` 上整本导出一本 LLM Wiki 的笔记本，zip 里有 `index.md`、`log.md`、`meta.json` 的 `contributed` 列出它们，组合根交空时失败（`boot/transfer_test.go:244-250` 钉住的清单随之改）；2：核对导入按 `contributed` 跳过（库的根下的路径、只认第一个 `meta.json`），没有 `meta.json` 的库加序号；3：去重：附件按 `Step.Asset` 的 SHA-256、页按正文的哈希，记在 `nodes.source_hash`（4.8）；4：来源区的页在正文之下完整列出附件（4.16）；5：lint 不把附件当页（孤儿、缺类型豁免附件，`resolved_asset` 的链接不算入链），未解析而读作附件的目标不进待建（4.11）；6：贡献者的输出有界：至多 2 个文件、`index.md` 2 MiB、`log.md` 1 MiB，测试钉住，不加流式写入（4.13）。P2、P3、P4 |
| [M8 来源区的守卫与撤销、还原、恢复](handoffs/M8-history.md) | 1–3 | 1、2：照旧（`Write.Kind`、`RevertsID`，撤销的每一步经守卫），守卫另看 `Step.Rewrite`；3 的五条：写回 C 改写过的来源页按版本行的 `rewrite` 放行；还原来源的版本一律拒绝；恢复来源、撤销一次删除算新增（写者），去重适用（比活着的来源）；撤销新建来源或来源的移动照删除、移动要管理员；同步的整组拒绝、后台的在那一批以 `refused` 停下，码进各操作的 `x-problem-codes`、网页有文字（4.6、4.7，P1 的 S2、P2 的 S1） |
| [M9/P1 MCP 的 prompt、虚拟文件与 `AGENTS`](handoffs/M9-P1-mcp-registry.md) | 1 注册表、2 最后一跳与契约 | 1：`Prompt` 加入口、错误分类、答复上限，不加 `Title` 与补全；2：最后一跳与 `prompts.json`，不改 `tools.json` 的两个键（4.15，P5） |
| 同上 | 3 虚拟文件 | 路径的一处把关（`FindPath`、`WalkPath` 的查找，含 `batch` 的计划）；`list` 与 glob 列出（`**/*.md` 配得上），`grep` 不搜；写由 `page.virtual_file` 拒绝；`index.md` 至多 500 项、`log.md` 50 个变更集，各有截断的说法；`ListIndex` 的代价写在 4.13（P4 的 S1） |
| 同上 | 4 `AGENTS` 的保护 | 模式的守卫（4.9，P1 的 S2）；说明照 M9 4.6 取，加系统的一段（4.15） |
| 同上 | 5 `prompts/get` | 64 KiB 放得下参数，不另设例外；错误经 `ProblemOf`，可预期的 -32602、别的 -32603；答复至多 48 KiB（4.15，P5） |
| 同上 | 6 余量、故事的约定、prompt 的说法 | 余量：先缩短共用的行（P3）；故事的 MCP 版本照 M9 的约定（第 9 节）；playbook 的写法照人工清单记下的毛病（4.15，P5） |
| 同上 | 7 守卫要认得的四种步 | 移动加改名：两样都看；`batch` 的合成：计划按项先判，守卫看合成之后的一步；临时名称：逐步判、不加标记，理由与唯一的偏严写在 4.7；先恢复再写回：恢复是新增、写回照 `rewrite` 判（P2 的 S1） |
| 同上 | 8 `lint` 与 `pending_sources` | 照 M9 的每个工具（4.11、4.15，P3 的 S1） |

开工时（P1 之前）五份移交照上表写进各自的"处理进展"，仍是 `open`；做完的 Phase 改它们为 `done`，收尾核对。

### 写给后面的 M

- **M11**（发现它的 Phase 写进 `M11-upgrade/handoffs/`，收尾核对）：
  - **状态**：升级的状态是 M11 自己的列或 `notebook_upgrades` 的状态，不扩 `notebooks_mode_check`（4.3）；升级期间按普通的契约；`source_root_id` 在普通笔记本里可以先定（检查只要求 LLM Wiki 有它）；批准时把 `mode` 改为 `llm_wiki`，并经 `page.NewSourceArea` 为来源区算出 `source_hash`（M10 只在单元里维护它，升级的批准不经单元时要另算，等于重建）。
  - **守卫**：冻结与护栏登记在 `pageRegistrants` 的守卫里，建议排在模式与锁之前；`Write.Notebook` 是加升级状态的地方；守卫的拒绝已在单元里标出（`replans`、后台撤销、transfer 的 `refused`），冻结的码照样得到这些去处。
  - **闸门**：提交闸门第 4、5 条（frontmatter 能解析、`type`、`summary` 不为空，`sources` 指向来源区里的节点）就是 lint 的 `invalid_frontmatter`、`missing_type`、`missing_summary`、`invalid_sources`（负责人 2026-10-24：必填属性在 M10 只检查，M11 的提交以它为闸）：经 wiki 的 `Lint` 在同一个快照里取，不另写查询；"解析不到的链接数不多于升级前"可用 `wanted` 的数目，但它数到 10,000 就停，闸门要准确的数时另数。
  - **内容保全**：第 7 条"差异只来自系统的链接改写"可按版本行的 `rewrite` 判。
  - **事件、说明、降级**：状态变化发 `mode`；说明里不用"upgrade"一词（它指服务端的升级）；降级在升级期间如何（拒绝或先放弃升级）由 M11 定；`notebook.change_mode` 可以给发起与批准用。
  - **playbook**：upgrade 的 prompt 登记在 wiki 的 `Prompts()`，同样只在笔记本入口、静态、在别的模式里答错误。
- **M12**：lint 每条规则在 30 万页上的耗时与内存（快照占一个连接的时长），`index.md` 每次 `read` 读整本的索引与树（10 万页约 1.1 秒），目录视图同 `ListIndex`；`source_hash`、ingest 与 linking 的部分索引在大的实例上建的用时（挡住写），写进升级说明；去重的 advisory lock 在大量并发新增时的等待。进 [M12 的性能移交](../M12-release/handoffs/M4-performance.md)。
- **v0.1 之后的打磨**：健康度面板的刷新与推送；页面树的图标再多一组；来源的差异对比；中文的 playbook。

### 总体设计的修订

随这份设计一起改的（审查之后由主持者落到 `v0.1-design.md`）：

- **3.3**：笔记本管理员的"修改模式"在 v0.1 是降级（升级在 M11）；"删除或移动来源"加改名与排序，来源根的改名、移动也是。
- **3.7**：来源区的页把它的附件在正文之下完整列出（4.16）。
- **3.8**：变更集另记 ingest 的来源（`changeset_sources`，`ingest_count`），只对 `edit` 的变更集；版本行记下是不是系统的链接改写（`rewrite`）；模板是笔记本的第一个变更集。
- **4.2**：`type`、`summary` 的"LLM Wiki 模式下必填"改为"LLM Wiki 模式下由 lint 检查，不拒绝写入"，"缺"的定义（4.10）。
- **4.4**：末尾加：LLM Wiki 下根下的 `index`、`log` 是虚拟页面，链接 `[[index]]` 在索引里照旧解析不到、不算待建，网页打开目录视图。
- **4.5** 第 5 条：系统的链接改写在单元里标出（`Step.Rewrite`）；撤销写回一个改写的版本同样不算修改。
- **5.1**：写入时的不变量是来源区、`AGENTS`、来源根与保留的标题；必填属性是 lint（负责人 2026-10-24）。
- **5.2**：frontmatter 一行改为"识别；LLM Wiki 由 lint 检查 `type`、`summary`"；"通用 lint"一行加"健康度面板"；`AGENTS` 一行的"必须有"写明由模板建、守卫保护。
- **5.4**：照 4.7、4.8 改写：写一次、之后只读（三种例外）；改名、移动、排序、删除来源要管理员；新增的各种（移入、上传、导入、恢复）；去重的范围（活着的来源，页按字节、附件按 SHA-256，不跨种类，比现在的字节）与位置的指出；"已消化"的定义（正文链接或 `sources` 的第一个键，区外任何页）；ingest 的定义与近似（4.14）。
- **5.5**：虚拟文件的内容、上限与截断的说法；`list` 列出、`grep` 不搜；"导出时写成真实文件"限整本导出；目录视图两种模式都有。
- **5.6**："链接有歧义"改为"几页在每一项偏好上都相同、按 id 选了一页"（同 4.4）；待建页面按写的目标、按页数，读作附件的目标不算；豁免写全；健康度面板能读笔记本的人都看得到，打开时算；每条数到 10,000。
- **5.7**：模板与建笔记本在一个事务里；模板用建的人的界面语言（中文或英文），之后由用户改写；建的对话框选模式，新手引导照旧是普通的；来源根的改名要管理员。
- **5.8**：图里的 `upgrading`、`reviewing` 是升级的状态，不是模式；降级是专门的动作（`downgradeNotebook`），有写入任务时拒绝，清掉来源根。
- **5.9**：ingest、query、lint 三份是英文的，只在笔记本入口，普通笔记本里答错误。
- **6.1**：problem 的扩展成员加 `duplicate_of`；新的码。
- **6.3**：工具表的 `lint`、`pending_sources` 写明两个入口、`pending_sources` 只在 LLM Wiki；prompts 一条写明只在笔记本入口、静态、契约 `prompts.json`；路径一条加虚拟的两个路径；说明按模式拼。
- **7.2**：notebook 的行写明 `mode`（`standard`、`llm_wiki`）与 `source_root_id`（不建外键）；page 的 `nodes` 加 `source_hash`，`page_revisions` 加 `rewrite`，`changesets` 加 `ingest_count` 与 `changeset_sources`；wiki 在 M10 没有表。
- **8.2**：wiki 的说明改为"模式契约的编排、lint、虚拟文件的内容、模板、playbook、升级流程（M11）"；写入的不变量（守卫）在 page。
- **8.3**：写入守卫的次序写成"（升级的冻结）→ 模式不变量 → 锁"。
- **10**：后端单元测试的"lint 规则、模式契约"写到 page 与 wiki 的 domain。
- **12.2** 的 M10 一行（五个 Phase，P1、P2、P4 各三个 Step，P3 两个，网页四个 Step）；**12.4**：写入守卫一行写明 M10 的注册者在 page、排在锁之前、M11 的冻结建议在最前；新加两行"笔记本的创建（事务内的订阅者）"与"笔记本模式的变化（订阅者）"（建立于 M10，注册者 M10 的模板与事件、M11）；"实时推送的事件类型"的 M10 写明类型 `mode`；"导出贡献者"写明 M10 的贡献者与它的上限；"MCP 工具与 prompt 注册表"写明 prompt 的入口与 `prompts.json`；**12.6**。
- 随实现它们的 Phase 改（第 13 节）：
  - **13.1** 第 1 条（`TreeWrites().Seed` 是"单元拒绝已在事务里的 ctx"的唯一例外；`Write.Notebook`、`Write.Role`、`Step.Content`、`Step.Rewrite`；守卫的拒绝在单元里标出；单元维护 `nodes.source_hash`；整组撤销一段里"守卫只凭名称认得临时的步"改为"守卫照普通的改名、移动判临时的步"）、第 2 条（ingest 的标记）、第 3 条（`notebook.change_mode`、`lint.read`；守卫按 `Write.Role` 判来源区的管理员）、第 4 条（新操作与守卫的码的次序；模式在锁之前）、第 5 条（来源区的 advisory lock 的位置；模板的单元重入建笔记本的锁）、第 8 条（`duplicate_of`；prompt 的错误）、第 10 条（lint 的日志）、第 11 条（`page.NewModeGuard`、`page.NewSourceArea`、`page.NewIngestMarks`、`transfer.NewWritingJobs`、wiki 的模块根与组合根的次序）、第 21 条（M10 注册的与最后一跳；两个新的扩展点）、第 30 条（`mode`）、第 31 条（4.18）、第 32 条不改。
  - **13.4** 第 3 条（MCP 的客户端加 prompt 的方法；故事的 MCP 版本）、第 4 条（4.17 的交错）、第 8、9 条（lint、`index.md`、去重的守卫与计划测试）、第 10 条（`source_hash` 等于重建）。
- **14**：新的风险见第 10 节（守卫逐步判的偏严、去重的派生列、lint 与 `index.md` 的代价）。
- **文档约定**（`docs/README.md` 的"Git 工作方式"）：M9 那一条之后加一句："M10 照上面的写法（[M10 总设计](v0.1/M10-llm-wiki/00-M10-design.md)第 7 节）。"

## 8. 本 M 建立与注册的扩展点

建立（总体设计 12.4）：

- **笔记本的创建（事务内的订阅者）**：notebook 的 `app/extension.go` 的 `NotebookCreated`（笔记本、工作区、模式、来源根的 id、模板的语言、执行者、客户端、时刻）与订阅者接口；在插入笔记本与管理员之后、可见性之前同步调用，任何错误整个回滚。模块根上以替身证明到达、次序与回滚（`notebook/extension_test.go`）。注册者：wiki 的模板。
- **笔记本模式的变化（订阅者）**：`ModeChanged`（笔记本、工作区、之前与之后的模式、执行者、时刻），降级（与 M11 的状态变化）在写模式的事务里调用。注册者：事件流的 `mode`。
- **页面的单元加入调用方的事务**：`(*page.Module).TreeWrites().Seed`，只给模板；是 13.1 第 1 条的例外，不是给别的 M 的扩展点。

注册：

- **写入守卫与开启的否决者**（M4 建立）：`page.NewModeGuard`，排在锁之前。
- **导出贡献者**（M7 建立）：wiki 的虚拟文件。
- **实时推送的事件类型**（M5 建立）：`mode`。
- **MCP 工具与 prompt 注册表**（M9 建立）：wiki 的 `lint`、`pending_sources` 与第一批 prompt。

改前面 M 的扩展点与代码（12.1 第 6 条的例外，逐项写明）：

- notebook（M3）：`CreateNotebook` 收模式与语言、调用创建订阅者、在同一事务里改成 `llm_wiki`；`View` 与查询带两列；`LockNotebook`、`ShareNotebook` 选出两列；`notebook.NewNotebooks` 的 `ShareByID`、`LockByID` 交回状态；新用例 `DowngradeNotebook`。
- page（M4–M9）：`Notebooks` 端口交回状态；`Write.Notebook`、`Write.Role`（不再丢掉 `Grant`）；`Step.Content`、`Step.Rewrite`；`Unit.apply` 标出守卫的拒绝、维护 `source_hash`；`Writer.Run` 之外的 `Seed`；单元交出父节点的一线；`batch` 的 `failed` 与计划的预检；撤销的 `revertDefect`、计划带 `rewrite`、核对的预检；导入与恢复在根下给保留的标题加序号、导入的单元跳过重复；上传的 `Check` 的预检；`RecordRevision` 合并 `rewrite`；路径的查找认出虚拟的两页（`FindPath`、`WalkPath` 与 `batch` 的模拟的树）；`list`、`read`、`open_notebook` 的虚拟项；`ToolDeps.Virtual`；说明的拼法（两处合成一处）；历史的读与筛选带 ingest；新的端口 `page.NewSourceArea`、`page.NewIngestMarks`、`page.Refusal`。
- linking（M6、M9）：索引的观察者拿到 `ChangesetID`、`Kind`、`Notebook` 并判定 ingest；`ReplacePage` 交回旧行的属性路径；lint 的查询；REST 的 `listIndex`。
- transfer（M7、M8）：`unitFailure` 认出守卫的拒绝、失败 `refused` 与报告的 `refusal`、问题 `duplicate_source`；`Deps.Contributors` 由组合根交 wiki 的；`transfer.NewWritingJobs`。
- 平台：mcpserver 的 `Prompt.Entries`、`promptHandler` 的错误与上限。
- 组合根：wiki 的接线与构建次序；`linkIndex` 的转换带上三样；`treeError` 的 `refused`；守卫的次序；`mcpTools` 拼 wiki 的工具，`Options.Prompts`。

## 9. 测试策略

- **规则**：`page/domain` 的 `CheckContract` 的表格测试，覆盖 4.7–4.9 的每一格（操作 × 之前与之后在不在来源区 × 角色 × 类型 × 标记）；wiki 的 `domain`（契约表、两个生成器、模板两种语言的对应）的表格测试。
- **整个程序的最后一跳**（13.1 第 21 条，组合根交空时失败）：
  - 守卫：`TestTheModeGuardReachesEveryPath`（照 `bootstrap/page_lock_test.go:8-20`）：第 3 节第 2 条的每条路径，每条一个会被拒的写，断言码与"什么都没写"；后台任务断言 `refused` 与报告；MCP 的写另有一个照 `TestTheEditLockReachesEveryMCPWrite`（`bootstrap/mcp_lock_test.go:11-39`）。守卫的效果（一个模式的码）在空的组合里答不出，所以这些测试本身就是反向对照，不另加"空的组合"的套具（调查第二部分第 40 条的那个问题：沿用 M5–M9 的做法）。
  - 创建订阅者：经 REST 建 LLM Wiki 的笔记本，两页、变更集、索引与 `pages` 事件都在；订阅者失败时笔记本不在。
  - 模式变化与事件：经 REST 降级，另一条流收到 `mode` 的帧（按契约核对）。
  - 贡献者：经 serve 整本导出，zip 与 `meta.json`。
  - prompt：两个入口的 `prompts/list`、`prompts/get`。
  - ingest：经 REST 与 MCP 的写之后 `changeset_sources` 的行。
  - wiki 的工具：照 M9 的工具清单、矩阵与码的表。
- **派生数据**：`source_hash` 等于重建（随机的写、移动、删除、恢复、撤销、降级之后）；`checkPages` 加 `source_hash` 的不变式；`batch` 进这个测试。
- **交错**：4.17 的六个，结束时核对不变式。
- **成本与计划**（13.4 第 8、9 条，`!race`，界线约本机实测的 3 倍，各在 `test-go-costs` 或 `test-go-guards` 的一行）：lint 每条规则与摘要在 10 万页上、计划测试在 30 万页上（量过之后加的部分索引的计划用上它）；`index.md` 的一次 `read` 在 10 万页；去重的一次新增在 10 万个来源；贡献者在 10 万页。
- **上限**：4.18 的每一处有测试。
- **反向对照**（13.4 第 1 条）：`prompts.json` 的检查（少一个、多一个、说明不同）；`TestTheBuiltInInstructionsNameEveryTool` 对新的两个工具；`schema_test` 的 `notebooks_mode_check`、`notebooks_source_root_check`、`source_hash` 与 `ingest_count` 的检查的反例。
- **权限矩阵**：新操作的行；LLM Wiki 的变体（来源上的树操作按角色）；MCP 的矩阵加两个工具。
- **e2e**（13.4 第 3 条）：`e2e/fixtures/notebooks.ts` 的 `createNotebook` 加 `mode` 与 `templateLocale`；`fixtures/mcp.ts` 加 `listPrompts`、`getPrompt`；数据库的断言加 `assert/wiki.ts`（模板的两页、`source_hash`、`changeset_sources`、报告的 `refusal`）。

**本 M 的故事**（`e2e/stories/wiki/lw<n>-<slug>.spec.ts`；"页面"有网页一侧的动作；MCP 版本照 M9 的约定在故事自己的文件里、名为"`<故事> (MCP)`"，种下的数据经 REST，每个 agent 一个 PAT，值来自答复）：

| 故事 | 内容 | 版本 | Phase |
|---|---|---|---|
| LW1 新建 LLM Wiki 笔记本 | 对话框选 LLM Wiki（中文界面）：根下 `raw` 与 `AGENTS` 是中文的模板，与笔记本同一个变更集与时刻，`source_root_id` 是 `raw`；英文界面建的是英文的；普通笔记本没有它们；MCP：笔记本入口握手的说明是 `AGENTS` 的正文加系统的一段 | 页面、API、MCP | P1 |
| LW2 保护与保留的标题 | `AGENTS` 删除、改名、移动被拒（`page.protected`），正文可改；`raw` 不能删除，写者不能改名、管理员能；根下建 `index` 页被拒（`page.virtual_file`）；MCP：`delete AGENTS.md`、`write index.md` | 页面、API、MCP | P1 |
| LW3 添加来源 | 粘贴为来源一次写成；上传一个 PDF 为来源；同样的字节再加一次被拒并指出已有的路径；MCP：`write raw/x.md`，重复被拒 | 页面、API、MCP | P2 |
| LW4 来源只读 | 来源页没有编辑、阅读视图有说明；PUT、MCP 的 `edit` 与 `write` 覆盖、还原版本都被拒；来源根的说明可以改；改名一页 wiki 页时来源里指向它的链接被系统改写 | 页面、API、MCP | P2 |
| LW5 管理员管理来源 | 写者改名、移动、删除来源被拒（`page.source_admin_only`），管理员可以；写者把一页 wiki 页移进 `raw` 成为来源，与已有来源相同时被拒 | 页面、API、MCP | P2 |
| LW6 历史与来源 | 写者撤销管理员删除来源的变更集（恢复，新增）；写者撤销管理员给来源改名的变更集被拒，管理员撤销成功、写回被改写过的页；回收站恢复一个与活着的来源相同的来源被拒；后台撤销以 `refused` 停下、报告说明 | 页面、API | P2 |
| LW7 导入进 LLM Wiki | 导入一个库到根下：库里的 `index.md` 成为 `index 2`；导入到 `raw` 下：重复的文件跳过、报告 `duplicate_source` | 页面、API | P2 |
| LW8 健康度 | 两种模式的面板：待建、孤儿、解析失败；LLM Wiki 另有缺类型、缺摘要、未消化、无效的 `sources`；读者看得到；"更多"翻页；MCP：`lint` 的摘要与一条规则的条目 | 页面、API、MCP | P3 |
| LW9 未消化的来源与 ingest | `pending_sources` 列出新来源；一个引用它的 `batch` 被标为 ingest、记下它，历史里有标记与筛选，之后它不再未消化；再引用它的编辑不是 ingest | 页面、API、MCP | P4 |
| LW10 虚拟的 index 与 log | 树里两行：`index` 打开目录视图、`log` 去历史；MCP：`list` 列出、`read index.md` 与 `log.md`、写被拒；整本导出带两个文件，再导入不多出页 | 页面、API、MCP | P4 |
| LW11 playbook | `prompts/list` 列出三个；`prompts/get ingest` 带来源的路径；普通笔记本里答 `wiki.not_llm_wiki` | MCP | P5 |
| LW12 降级 | 管理员确认降级：另一个标签页随事件更新、虚拟的两行消失（P4 加）、`raw` 可以编辑、`AGENTS` 可以删除；有导入在跑时被拒；写者看不到降级 | 页面、API | P1、P4 |
| LW13 目录视图 | 普通笔记本的目录按类型分组、按标签筛选、翻页 | 页面、API | P4 |

- 没有 MCP 版本的：LW6、LW7、LW12、LW13（历史的写、导入、降级、网页的视图：MCP 没有这些动作，M9 第 11 节第 1 条）；LW11 只有 MCP 版本（只能经 MCP）。
- 前面 M 的故事都在普通笔记本里，照旧通过；它们的 MCP 版本不变（M9/P5）。MC1 的 `tools/list` 照 `tools.json` 读，不写死，加两个工具不改它。

## 10. 风险

| 风险 | 缓解 |
|---|---|
| 守卫逐步判：撤销中途先把祖先放回来源区、再临时改名它之下的节点时，比净效果严 | 只多拒、不多放（4.7 写明理由）；这种撤销要同一个变更集既把文件夹移出来源区又在其中交换名称；被拒时管理员照样撤销得了；测试钉住这一种的结论 |
| `source_hash` 是派生的列，维护分散在单元的每种步 | 维护在 `Unit.apply` 的一处，按这一步之后的来源区算；`checkPages` 与"等于重建"的性质测试（含撤销、恢复、降级）；只有 LLM Wiki 的单元碰它 |
| 部分索引的迁移（`nodes` 的 `source_hash`、`changesets` 的 ingest、linking 的）在大的实例上挡住写 | 加列只改元数据；建索引的用时 P2、P3、P4 各量，写进升级说明（先停旧的服务再 `migrate up`，同 M9） |
| lint 在 30 万页上一次运行秒级，快照占一个连接 | 每条规则先量、只给超出预算的加部分索引；数目数到 10,000；面板打开时算一次、不随事件重算；成本守卫与计划测试 |
| `index.md` 每次 `read` 读整本的索引与树（10 万页约 1.1 秒） | 至多 500 项、40 KiB，末尾指向 `index` 工具；成本守卫；M12 的性能移交 |
| 写者把一页移进来源区之后要管理员才能移出或撤销 | 负责人的决定 1 的直接结果；网页在移进来源区之前确认一次（"之后只有管理员能移出"）；负责人可以改判（第 11 节设计的取舍第 8 条） |
| ingest 的判定按单元之后的树与解析看别的页 | 只在一个变更集同时改树与写正文时差一个标签；不影响 lint 与任何规则（4.14） |
| 模板的单元加入建笔记本的事务，是写入单元的第一个例外 | 只经 `Seed`、只给模板；锁的次序不变（同一事务里的重入）；注入失败的整个程序测试 |
| 去重比现在的字节：来源被链接改写之后，再加原来的那份不算重复 | 链接改写碰到来源很少（来源少有指向 wiki 页的链接）；与负责人"字节完全相同"的说法一致；改判为"比加入时的字节"要另存一份哈希 |
| 静态的注册表：普通笔记本里也列出 `pending_sources` 与三个 prompt | 说明写"LLM Wiki only"，调用答 `wiki.not_llm_wiki`；按笔记本过滤要在 `listChanged: false` 下让同一个入口的列表随笔记本变，留给以后 |
| 内置的说明离 4 KiB 只剩 69 字节 | 先缩短共用的行再加两行；测试守 4 KiB；LLM Wiki 的系统段不在 4 KiB 的内置文字里 |
| 网页的部分多（四个网页的 Step） | 每个 Step 带它的故事、合并时全绿；可挪进打磨的几项在第 7 节列出，由负责人在 P4 超出时定 |
| agent 不照 playbook 做 | playbook 只是建议，来源区、保护与保留的标题由守卫强制；人工清单在 v0.1 的最后核对 |

## 11. 负责人确认的决定

负责人 2026-10-24 定下四件事：

1. **来源区：一个来源写一次，之后只读**。网页上新的来源来自"粘贴或上传为来源"的对话框，一次写成；打开已有的来源是只读的视图。agent 以一次 `write` 写一个来源。只有笔记本的管理员能改名、移动、删除来源。与来源区里已有的一个逐字节相同的新来源被拒绝，并指出已有的在哪里（4.7、4.8）。
2. **必填属性：检查，从不强制**。缺 `type` 或 `summary` 的页在健康度面板里显示为需要注意，MCP 的 `lint` 报出来；没有写入因此被拒：网页新建的页、agent 自动补建的父页、导入与恢复都照今天。M11 的升级提交以它为闸，那是 M11 的事（4.10，"写给后面的 M"）。
3. **语言**：模板（根下的 `AGENTS` 页与 `raw` 的说明）用建的人的界面语言（zh-CN 或 en）写，之后由用户改写；三份 playbook（MCP 的 prompt：ingest、query、lint）只有英文，同工具的说明（4.4、4.15）。
4. **健康度面板**：能读这本笔记本的人都看得到；普通与 LLM Wiki 的笔记本都有：通用规则两种模式都有，LLM Wiki 加它自己的；打开时算，每条规则显示数目与前几项、可以翻页；写入时不维护计数（4.11、4.12）。

下面是设计里的取舍，负责人可以改判：

1. 分工：wiki 是新模块，管契约表、模板、playbook、lint 的编排与工具、虚拟文件的内容与导出贡献者，M10 里没有表；写入的不变量（守卫、来源的哈希、保留的标题）在 page；ingest 的判定在 linking 的观察者、记在 page 的变更集上（4.1）。
2. 模式只有 `standard`、`llm_wiki` 两个值；升级的状态是 M11 自己的，不是模式（4.3）。
3. 来源根是笔记本行上不建外键的一列，M10 不能改指；降级时清掉它与来源的哈希（4.3、4.5）。
4. 模板与建笔记本在一个事务里：笔记本的创建订阅者，加上页面的单元在这一处加入调用方的事务（`Seed`，声明的例外）（4.4）。
5. M10 里空的普通笔记本不能直接切到 LLM Wiki，留给 M11 的升级（4.5）。
6. 降级是专门的动作，有写入任务时答 `transfer.busy`，不写审计行（4.5）。
7. 模式的守卫排在编辑锁之前：长久的拒绝先答（4.6）。
8. 来源区的规则：移入、上传、导入、恢复进来源区算新增（写者）；改名、移动、排序、删除来源与来源根的改名、移动要管理员；写者移进来源区的页要管理员才能移出或撤销那次移动（4.7）。
9. 撤销写回来源页只在那一版是系统的链接改写时放行（`page_revisions.rewrite`）；还原来源的版本一律拒绝；空的来源可以写一次第一次正文（4.7）。
10. 去重：活着的来源，页按字节、附件按 SHA-256，不跨种类，比现在的字节；哈希记在来源的节点上（`nodes.source_hash`），不给 `page_contents`、`asset_blobs` 建全表的哈希索引；并发的第一次正文以按笔记本的 advisory lock 串行（4.8）。
11. 导入进来源区时重复的文件跳过、写进报告，不让任务停下（4.8）。
12. 守卫逐步判，撤销里临时名称的步照普通的改名、移动判，不加标记（4.7）。
13. 根下的 `index`、`log` 对页与附件都保留；导入与恢复给它们加序号（4.9）。
14. "缺"是没有这个键、值不是字符串、空或只有空白；frontmatter 写坏的页只算解析失败（4.10）。
15. lint 读时算、一个快照；每条规则数到 10,000；条目分页；过时的页跳过并计数（4.11）。
16. 待建页面按写的目标分组、按链接它的页数；读作附件的目标与 LLM Wiki 的 `[[index]]`、`[[log]]` 不算；"有歧义"保留按 id 选了一页的含义（4.11）。
17. 消化：来源区以外任何页（含 `AGENTS` 与来源根）的正文链接或 `sources` 的第一个键的属性链接（4.11）。
18. 虚拟文件：`list` 与 glob 列出，`grep` 不搜；`index.md` 至多 500 项、`log.md` 至多 50 个变更集；只在整本导出时写成文件（4.13）。
19. ingest：只看 `edit` 的变更集里写了正文的、来源区以外的页；别的页按单元之后的树与解析；编辑会话的标记只增不减；导入、还原、恢复与撤销不是 ingest；每个读者看得到（4.14）。
20. 注册表是静态的：`pending_sources` 与三个 prompt 在普通笔记本里列出、调用答 `wiki.not_llm_wiki`；prompt 只在笔记本入口；prompt 的契约是另一个文件 `prompts.json`（4.15）。
21. 目录视图两种模式都有（4.16）。
22. 来源页的编辑会话由否决者拒绝；在来源区里"新建页面"只建空的页（4.7、4.16）。
23. 提交说明回到文档约定的写法（第 7 节）。
24. 第 9 节里没有 MCP 版本的故事与理由。

## 12. Phase 进度表

| P | Phase 文档 | 状态 | 审查 | 合并 |
|---|---|---|---|---|
| P1 契约的框架（S1–S3） | — | 未开始 | — | — |
| P2 来源区（S1–S3） | — | 未开始 | — | — |
| P3 lint 与健康度（S1、S2） | — | 未开始 | — | — |
| P4 虚拟页面与 ingest（S1–S3） | — | 未开始 | — | — |
| P5 playbook、说明与收尾的故事（S1） | — | 未开始 | — | — |
| 收尾 | — | 未开始 | — | — |

## 13. 变更记录

| 日期 | 变更 | 依据 |
|---|---|---|
| 2026-10-24 | 初稿（未提交，待设计审查） | 移交 5 份、代码的调查（`91ee78d8`）、负责人 2026-10-24 的四个决定（第 11 节） |
