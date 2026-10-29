# M10 模式契约与 LLM Wiki：总设计

| 项 | 内容 |
|---|---|
| 里程碑 | M10 模式契约与 LLM Wiki（`M10-llm-wiki`） |
| 日期 | 2026-10-24 |
| 状态 | 进行中；P1与P2/S1已验。S2的33b4产品/159控制、本机门禁及精确11项CI为已接受历史，B-N1在4550前的规范文档投影后已关闭；4550绿色后第一片双分支均值450.5s触发再平衡，fb92974b的精确CI40740783718因Revert10k第三样本HTTP500失败。恢复折叠已提交288但CI40768488401仍因Revert10k第三样本失败；有界facts保留：最终代码f1818066的本机门禁、独立审查与提交后gen-check均已核定，精确代码CI40793294472全11绿。文档002a5638的精确CI40797519100全部11作业及必需步骤、code/docs分支四片Test配对，以及main合并`66d8236c12aaa8923616f6f42f2ee33b772884f9`的精确CI40800844731全部11作业及必需步骤均获主持者接受；本记录提交自身CI与双main Test配对仍由后续真实运行收口，完成前不开始S3。前33b4/4550绿、fb/288失败和03不精确同步的历史均保留。最新验证与剩余集成见[P2](02-P2-source-area.md)及[S2验证表](reviews/P2-S2-source-hash-review.md#最终验证与集成)。S3、P3–P5和M10收尾仍待，不开始M11。 |
| 依赖 | M9（MCP 工具与 prompt 注册表）、M7（导出贡献者、附件进树与链接）、M6（链接索引）、M8（变更集的类型、撤销与后台任务）；总体设计 12.3 照此修订（第 7 节"总体设计的修订"） |
| 上级文档 | [v0.1 总体设计](../v0.1-design.md) 第 3.3–3.10、4.2、4.4、4.5、5、6.1–6.3、7.1、7.2、8.2、8.3、8.5、9.2、10、10.1、12.1–12.6、13、14、15 节 |
| 代码位置的写法 | 同 [M9 总设计](../M9-mcp/00-M9-design.md)：模块写作 `page/app/unit.go:187`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…`、`archtest/…` 在 `server/internal/` 之下；迁移写作 `00009_notebook_notebooks.sql:5`（`server/migrations/sql/`）；SDK 写作 `sdk/mcp/protocol.go:966`（`~/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.8.0/`）；前端写作 `web/…/pages/page/page-layout.tsx:153`（`web/apps/web/src/` 之下）；e2e 写作 `e2e/fixtures/mcp.ts:41`。行号都是 `91ee78d8` 的（`ebed0e07` 只加了这份文档） |

---

## 1. 目标

1. **模式是笔记本的一项属性**：`standard` 或 `llm_wiki`，记在笔记本行上。建笔记本时在对话框里选，默认普通；`llm_wiki` 在 M10 只能在建的时候选（普通笔记本升级是 M11）；管理员可以把 LLM Wiki 降级为普通（4.3、4.5）。
2. **模板与建笔记本是一件事**：新建 LLM Wiki 笔记本时，笔记本行一开始就以 `llm_wiki` 与先定下的来源根 id 写入，根下的来源根页面 `raw`（带说明）与 `AGENTS` 页面在同一个事务、同一个变更集与时刻里写下，用建的人的界面语言（中文或英文，没给时英文）；建成的 `llm_wiki` 笔记本一定有这两页（4.4）。
3. **来源区只写一次**：来源根之下的整棵子树是来源，写进去之后正文只读；来源区里的节点的改名、移动、排序、删除只有笔记本管理员能做（规则表里的动作 `page.manage_source`）；与已有来源逐字节相同的新来源被拒绝，并指出已有的那一个；系统的链接改写不算修改。每一条写入路径（网页、REST、MCP、`batch`、上传、导入、还原、恢复、撤销，同步的与后台的）经同一个守卫；后台任务被拒时，不论是开始时的预检还是守卫拒绝的，以码停下，不报内部错误（4.6–4.8）。
4. **`AGENTS` 与保留的标题**：LLM Wiki 下根下的 `AGENTS` 页面不能删除、改名、移动，来源根不能删除；根下的 `index`、`log`、`index.md`、`log.md` 是保留的标题（页与附件都是），`index.md`、`log.md` 是系统的虚拟页面（4.9）。
5. **必填属性只检查**：来源区以外的页面缺 `type` 或 `summary` 时，健康度面板与 MCP 的 `lint` 报出来，任何写入都不因此被拒（负责人 2026-10-24，第 11 节负责人的决定第 2 条；4.10）。
6. **lint 与健康度面板**：两种模式都有的通用规则、LLM Wiki 专属的规则，读的时候在一个快照里算，同时运行的次数有界；lint 不看来源里写的内容，来源只由"未消化"一条判定；REST、网页的健康度面板与 MCP 的 `lint`、`pending_sources` 给出同样的结果，每条规则一个数目与分页的条目；能读笔记本的人都看得到（4.11）。
7. **虚拟的 `index`、`log`**：MCP 的 `list` 列出、`read` 读到实时生成的 `index.md`、`log.md`，写它们得到"这个文件由系统生成"的错误；`index.md` 只列 wiki 的页，来源区写成一行；网页的页面树里两行虚拟的行，一行打开目录视图（目录视图两种模式都有），一行去历史；整本导出时写成真实的文件（4.13）。
8. **ingest 标记**：一个变更集新引用了原本未消化的来源时，记下这些来源；对这个变更集写了或删除了的页，"原本"按它们旧的行确切判定；历史里每个读者看得到标记、能按它筛选，MCP 的 `log`、`changeset` 同样给出（4.14）。
9. **playbook**：ingest、query、lint 三份英文的操作手册，以 MCP prompt 在笔记本入口下发（每本笔记本都列出，普通笔记本里答 `notebook.not_llm_wiki`）；握手的说明按笔记本的模式与入口拼成；真实客户端上的人工核对补进 M9 的清单（4.15、4.19）。
10. **接到 M11**：模式的列与它的检查、守卫的次序、`Write` 上的笔记本状态、事件类型 `mode`、说明的余量（也给 M11 的两个工具留够）、lint 的显式范围的入口与降级都留好了 M11 的位置，M11 只加自己的（第 7 节"写给后面的 M"）。

## 2. 范围

**做**：

- notebook：迁移 00048 加 `notebooks.mode` 与 `source_root_id`；`createNotebook` 收 `mode` 与 `template_locale`（没给时 `en`）；LLM Wiki 的笔记本行一开始就以 `llm_wiki` 与先定下的来源根 id 插入，在事务里调用新的创建播种者（新的扩展点，4.4），播种者交回它写下的来源根，没有写下时整个创建作为缺陷回滚；HTTP 适配器由凭据定出客户端交给用例（同 page 的，4.4）；专门的降级动作 `POST /api/v0/notebooks/{notebook_id}/downgrade` 与新操作 `notebook.change_mode`；模式变化的订阅者（新的扩展点）与事件类型 `mode`（4.5）；`Notebook` 带上 `mode`、`source_root_id`。
- access：规则表加 `page.manage_source`（笔记本管理员），来源区里的树操作由它决定（4.7）。
- page：写入单元的 `Write` 带上笔记本的模式与来源根（读一次，在笔记本行的锁下）；`Step` 带上正文的哈希与大小、合并进这一步的正文操作数、"系统的链接改写"的标记；模式的守卫 `page.NewModeGuard` 与它的否决者（来源页不开编辑会话），登记在编辑锁之前；没有数据的规则是 `page/domain` 的纯函数 `CheckContract`，由 `batch` 的计划、上传的 `Check`、撤销与恢复的核对与导入的单元先用一遍，去重是守卫与导入的单元在它之后的一次探查（4.6、4.8）；守卫与预检的拒绝都标出，`batch` 的 `replans` 不再吞掉它们，后台的恢复与撤销以新的失败 `refused` 停下；来源的哈希 `nodes.source_hash` 由单元维护，删除时清掉；附件的 SHA-256 经 `WriterDeps` 的新端口读（asset 的）；版本行的 `rewrite`；根下保留的标题与 `AGENTS`、来源根的保护；导入与恢复在根下给保留的标题加序号；页面的单元在建笔记本的事务里写模板（`TreeWrites().Seed`，一处声明的例外，4.4）；路径的一处把关认出虚拟的 `index.md`、`log.md`（`list`、`read`、写的拒绝、`batch` 的计划）；变更集的 ingest 标记（`changesets.ingest_count` 与 `changeset_sources`），历史的读与筛选；内置的说明缩短，给 M10 与 M11 的工具留出余量（4.15）。
- linking：lint 的查询（各规则，量过之后才加的部分索引）；索引的观察者拿到 `ChangesetID`、写的种类与笔记本的模式，以这个变更集写了、删除了的页的旧行判定 ingest，经 page 的端口记下（4.14）；REST 的目录 `GET /api/v0/notebooks/{notebook_id}/index`（M9 的 `ListIndex` 加按父节点 id 的筛选）。
- wiki（新模块）：模式的契约表（每种模式有哪些 lint 规则、虚拟文件与 playbook）；模板的两种语言的正文；lint 的编排（一个快照里调 linking 与 page 的端口，进程内的名额与同一本笔记本的合并）与它的 REST、MCP 的 `lint`、`pending_sources`，另有给 M11 的显式范围的入口；虚拟文件的生成；导出贡献者；ingest、query、lint 三个 prompt。M10 里它没有自己的表（4.1）。
- transfer：报告的失败加 `refused`（带拒绝的码与节点），导入的问题加 `duplicate_source`；`ExportContributor` 登记 wiki 的一个。
- 平台：mcpserver 的 `Prompt` 加 `Entry`（同 `Tool` 的位掩码），`prompts` 的能力按每个入口的 prompt 声明；参数超过 4,096 字节在处理函数之前答 -32602；错误经 `ProblemOf` 分类、答复有界；prompt 的契约 `api/mcp/prompts.json`。
- 网页：建笔记本的对话框选模式；设置里的模式与降级；`mode` 事件的处理；页面树的图标（来源、`AGENTS` 各一个，带文字的等价）、虚拟的两行、来源区与 `AGENTS` 的行按角色与行的种类给出菜单；来源区里的"新建页面"先问标题、拖进来源区之前确认；"粘贴或上传为来源"的对话框；来源页的只读视图（`isSource` 一处判定，收起编辑、勾选、还原版本、附件的改名与移动、"在这里编辑"）与编辑中的页被移进来源区时失去编辑；重复的来源的文字与链接（`duplicateText`）；健康度面板；目录视图；历史的 ingest 标记与筛选。
- 测试与文档：守卫在每条触发路径上的最后一跳，prompt、贡献者、事件、两个订阅者的最后一跳；交错；lint、`index.md`、去重、ingest、恢复与撤销进来源区、降级的成本守卫与计划测试；本 M 的故事（第 9 节）；README 的 LLM Wiki 一节；M9 的人工清单加 prompt 的一节（4.19）；总体设计的修订。

**不做**：

- 普通笔记本切换到 LLM Wiki（空的也不行）、升级的状态、冻结、护栏与闸门：M11（第 11 节设计的取舍第 5 条）。
- 写入时检查必填属性（负责人 2026-10-24）；语义的检查（矛盾、过时）：agent 做，lint 的 playbook 给范围（5.6）。
- 来源根改指另一页：M10 里来源根由模板定下、降级清掉，M11 的升级再定（4.3）。
- 健康度面板的定时刷新与推送、写入时维护的计数（负责人 2026-10-24）。
- 中文的 playbook（负责人的第 3 条决定）、账户入口的 prompt、prompt 的补全（`CompletionHandler`）、按笔记本过滤 `prompts/list` 与 `tools/list`（4.15）。
- 来源的差异对比、来源区的配额、跨笔记本的去重。
- 把一个 LLM Wiki 的导出重新导入到新的 LLM Wiki 时，把档案的 `raw/` 并进来源根、合并 `AGENTS.md`：v0.1 的一处写明的限制（得到 `raw 2` 与 `AGENTS 2`），已有的笔记库走 M11 的升级（4.9、第 7 节"写给后面的 M"）。
- 网页上显示 MCP 的地址：仍是 v0.1 之后的打磨（M9）。

## 3. 完成标准

总体设计 12.5 的通用标准之外：

1. **模式与模板**：两种模式经网页与 PAT 都建得出；`llm_wiki` 的笔记本建成时根下恰有 `raw`（来源根，`source_root_id` 指向它）与 `AGENTS`，正文是所选语言的模板，与笔记本同一个变更集、同一个时刻（时钟每读一次就走的测试）、客户端是建的请求的；三者在一个事务里：模板的写失败时笔记本也不存在（整个程序的测试以测试库上的一个触发器在插入 `AGENTS` 时报错，之后没有笔记本、成员、节点与变更集）。降级照 4.5：之后 `raw` 是普通的页（可以删除）、`AGENTS` 可以删除、虚拟的两行消失，另一个标签页经 `mode` 事件看到；有写入任务时答 `transfer.busy`。
2. **守卫到达每条路径**：4.7 的每条规则在 REST 的新建、改名、移动、排序、删除、写正文、勾选任务、开启编辑会话、上传、还原版本、从回收站恢复（同步与后台）、整组撤销（同步与后台），MCP 的 `write`、`edit`、`move`、`delete`、`batch` 上各有整个程序的测试，组合根不登记守卫时失败：有预检的路径（`batch` 的计划、上传的 `Check`、撤销与恢复的核对）另各有一个只有守卫拒绝的情形（单元里才发现的重复）。导入在 M10 的规则下没有可拒的（保留的标题加序号、重复的跳过、每一步都是写者能做的新增），它到达守卫在模块一级以假的守卫测（4.6）。拒绝的码在每个操作的 `x-problem-codes`（各自模块的 `adapter/http` 测试里都答出过）与 `mcpToolCodes()` 里；`batch` 的答复指出第几项；后台任务以 `refused` 停下、报告带码与节点，已写的留着：两批之间执行者被降为写者与两批之间出现的重复各一例，都由那一批的守卫拒绝。
3. **来源区**：写一次、之后只读（系统的链接改写与撤销它的写回除外）；编辑会话的保存从不算来源的第一次正文；来源区里的改名、移动、排序、删除要 `page.manage_source`；移入、上传、导入、恢复进来源区算新增；去重照 4.8（页按字节、附件按 SHA-256、不跨种类、只看活着的来源、比较现在的字节），并发的两个相同的新增只成一个（两种锁的路径各一个交错）；`nodes.source_hash` 等于从头重建（性质测试与 `checkPages`），删除（节点的与笔记本的）时清掉，降级之后与降级之后的恢复也成立。
4. **`AGENTS`、来源根与保留的标题**：4.9 的每一格有测试；导入与恢复（同步与后台）在根下给 `index`、`log`、`index.md`、`log.md` 加序号、报告 `renamed`（后台的恢复数 `counts.renamed`）；导入里重复的来源页在库里下面有东西时建成空的文件夹、它下面的照常落下，没有时跳过。
5. **lint**：4.11 的每条规则在两种模式下的表格测试（含豁免的页、附件、读作附件的目标、带点的页名、来源里写的链接、过时的行）；一次运行在一个快照里；同时的运行有名额、超出时答 `server_busy`；REST、MCP 与网页的数目与条目相同；MCP 的答复在 48 KiB 之内；10 万页的成本守卫各在 Makefile 的一行上，计划测试在小的、分析过的种子上照三个阶段跑（4.11）。
6. **虚拟文件**：`list`（文件夹与 glob，含游标在虚拟项两边的分页）、`read`（分行、`offset`、`column`）、写与别的工具的拒绝、`batch` 的计划各有测试；`index.md`、`log.md` 的内容与界线照 4.13（以字节为界，写者的字符串中和过）；整本导出带两个文件、`meta.json` 的 `contributed` 列出它们，子树的导出不带，重新导入跳过它们（贡献者的最后一跳在 `llm_wiki` 的笔记本上，普通笔记本的档案照旧作对照）。
7. **ingest**：4.14 的判定在真的库上测（新引用、早已消化、这个变更集改走或删除了原来的引用、编辑会话、导入与撤销不算、两本笔记本互不影响）；历史的标记、筛选与来源的列表经 REST 与 MCP。
8. **playbook 与说明**：笔记本入口的 `prompts/list` 等于先写的 `api/mcp/prompts.json`（反向对照），账户入口不声明 `prompts`；三个 prompt 经组合根到达它们的处理函数（组合根交空时失败）；普通笔记本里答 `notebook.not_llm_wiki`；握手与 `open_notebook` 的说明照 4.15 按入口拼成；内置的说明在 4 KiB 之内、讲到每个工具，并给 M11 的两行留着余量；Claude Code 与 Codex 上的人工核对写进 M9 的清单（4.19）。
9. **网页**：第 9 节的页面版本的故事通过；新码与新失败在两种语言里都有文字（`problem-messages.test.ts`，报告的失败与问题由 `transfer-report.tsx` 的类型表强制），由加它们的服务端 Step 一起写。

## 4. 关键决定

### 4.1 模块与归属

| 项 | 决定 | 放弃的方案 |
|---|---|---|
| 模式的列 | `notebooks.mode` 与 `source_root_id` 是 notebook 的迁移（00048）：迁移只能改自己模块的表（`archtest/sqlc_test.go:223-236`），总体设计 7.2 也把这两列给了笔记本行。别的模块经只凭连接池的端口读它们（4.6） | 一张 wiki 的表：要端口、笔记本删除的订阅者与清理器，而每个写入单元都要读它 |
| 写入的不变量 | 在 page：守卫 `page.NewModeGuard(pool, authorizer)`（P2/S1已接连接池与SourceAncestors最小端口，app构造为NewModeGuard(auth, sources)）是 page 模块的注册者（同 M5 的 `page.NewEditLock`），没有数据的规则是 `page/domain` 的纯函数 `CheckContract`，去重是守卫在它之后的一次探查（4.6、4.8）。理由：守卫要的每一样都是 page 的数据（树、祖先、正文的哈希、节点的种类、单元的标记），而同一组规则还要在 page 自己的计划里先用一遍（`batch` 的计划、上传的 `Check`、撤销与恢复的核对、导入的单元）。放在 wiki 时，wiki 不能导入 page，`Step` 与 `Change` 要在 wiki 里另抄一份、由组合根逐步转换（`bootstrap/linking.go:25-38` 的做法），page 的计划又要经端口回到 wiki 判每一步：两个方向的端口，换来的只是名义上的归属。来源区的管理员权限是规则表的动作 `page.manage_source`（access，笔记本管理员），守卫经 `shared.Authorizer` 问它（4.7） | 守卫在 wiki：上面的两套转换与端口；守卫在 linking：linking 管链接，不管树；守卫自己比较角色：权限散到规则表之外，矩阵的覆盖检查看不到 |
| 附件的哈希 | 移入、恢复进来源区的附件的 SHA-256 在 asset 的 `asset_blobs` 里，page 的 SQL 读不到（`server/sqlc.yaml:3`、`:78-101`）。M8 的附件端口 `AttachmentInfo`（`page/app/ports.go:558-569`，今天只给 MIME 与字节数）加 `SHA256`，asset 的适配器一并读出；`WriterDeps` 加这个端口，单元在需要时一次读出一棵子树的附件的哈希、放在步上，守卫与 `source_hash` 的维护都从步上取（4.8） | 守卫自己经端口读：单元维护 `source_hash` 时还要同样的数据，读两次 |
| 编排与内容 | 新模块 wiki（总体设计 8.2）：模式的契约表、模板的正文、三个 playbook、lint 的编排（一个快照里调 linking 与 page 的端口，同导出由 transfer 编排 page、linking、asset 的端口）、lint 的 REST 与 MCP 工具、虚拟文件的生成、导出贡献者。M10 里 wiki 没有表（不进 `server/sqlc.yaml`、`deploy/runtime-grants.sql` 与清理器）；要动的清单是操作的并集（`bootstrap/actions_test.go:21`）、组合检查（`archtest/composition_test.go` 的命令行的路径上不许建模块的 `New`，`:100-117`；`:50-83` 不按模块名列、不必改）、`api/modules/wiki.yaml` 与根的 `openapi.yaml`、`wiki/adapter/http` 的 `TestMain`、矩阵与 wiki 自己的一个码 `wiki.lint_rule_not_found`（"不是 LLM Wiki"一律答 notebook 的 `notebook.not_llm_wiki`，同 notebook 答 `workspace.not_found` 的跨模块的码）。M11 的升级（`notebook_upgrades`、升级的工具与 playbook）也落在这里 | 不建 wiki、把 lint 与工具放进 linking、模板与 prompt 放进 page：今天最省，但一种模式的契约散在三个模块里，以后的模式（总体设计 5.1"再写一份契约"）每次改三处，M11 照样要建 wiki |
| lint 的数据 | linking 写它自己的表的查询（`page_links`、`indexed_pages`、`page_properties`），来源区与 `AGENTS` 的 id、页的路径、种类、建的时刻与正文的字节数经 page 的端口（`page.NewSourceArea(pool)` 的读，linking 的 SQL 不连 `nodes`，`server/sqlc.yaml:119-130`；M6 移交注意第 3 条）。wiki 用的只凭连接池的端口都在第 8 节列出 | wiki 读 linking 的表：越过模块的边界 |
| ingest 的判定 | 在 linking 的索引观察者里：只有它同时有一页写之前的链接与写之后的解析。`ReplacePage` 与 `DeletePages` 交回每页旧的（解析到的节点、属性键）与旧的第一个折成 `sources` 的键（`linking/app/index.go:46-69`），判定由此对这个变更集写了、删除了的页是确切的（4.14）。结果经 page 只凭连接池的端口记在变更集上（page 的表、page 的 SQL） | wiki 的观察者排在 `links` 之后：那时旧的行已经没了，只能再读上一版的正文重新解析；排在 `links` 之前：新的链接还没有解析 |
| 虚拟文件 | 路径的认出在 page 的 domain（`domain.FindPath`、`WalkPath` 的查找，MCP 的工具与 `batch` 的计划共用它，`page/domain/path.go:195`、`page/domain/batch_plan.go:361-493`）；内容由 wiki 生成，经 page 的 `ToolDeps.Virtual` 交给 `read` 与 `list`（同 M9 的 `ToolDeps.Attachments`，在 wiki 建好之后由组合根调 `Tools`；`page.New` 在它为空时 panic，同 `TestTheModuleRefusesNoAttachmentInfos`） | 每个工具各自认：漏掉 `batch` 的模拟的树 |
| 模板的写 | 经 page 的写入单元（13.1 第 1 条：每个写入入口都经写入管线），由 wiki 的创建播种者经 `(*page.Module).TreeWrites().Seed` 调用（4.4） | notebook 直接插入 `nodes`：越过模块，也绕过变更集与索引 |
| 降级 | notebook 的用例（模式是它的列），经 transfer 只凭连接池的 `transfer.NewWritingJobs(pool)` 判断有没有写入任务、经 page 的 `page.NewSourceArea(pool).Forget` 清掉这本笔记本的 `source_hash`（4.5；一处声明的不经单元的写，同删除的级联：派生的数据、没有变更集、在笔记本行的锁下） | 放在 wiki：要 notebook 的写端口，多一层 |

组合根的次序：page → asset → wiki（拿 page 的 `TreeWrites()` 与 linking、page、notebook 只凭连接池的端口）→ transfer（`Deps.Contributors` 拿 wiki 的贡献者）→ notebook（`Deps.Seeders` 拿 wiki 的模板）；page 的 `Tools(ToolDeps{Attachments, Virtual})` 在 asset 与 wiki 之后调用。今天 notebook 在 page 之前建（`bootstrap/wire.go:149`、`:163`），而 page 只用 notebook 只凭连接池的端口（`notebook.NewNotebooks(pool)` 等），所以把 notebook 挪到 wiki 之后不改别的；wiki 用 linking 与 page 只凭连接池的端口，不要 linking 的模块实例；13.1 第 11 条的 page → asset → transfer 不变。

两个注册者不只凭连接池构造，照 M5、M6 的例外写进 13.1 第 21 条：wiki 的创建播种者要 `(*page.Module).TreeWrites()`；`page.NewModeGuard` 要组合根的 `shared.Authorizer`（access 的，与写入单元判定用的是同一个）。授权者只有 serve 有，所以 `pageRegistrants(pool)` 改为 `pageRegistrants(pool, authorizer)`，命令行经 `notebookRegistrants` 要的会话的订阅者抽成 `pageSessionSubscribers()`（命令行的路径上不许建 `access.New`，`archtest/composition_test.go:100-117`；它不收连接池：唯一的订阅者是事件流的发布者，只要它的 `NOTIFY`，P1/S2）。

### 4.2 数据：迁移

迁移的号照合并的次序取（goose 不收比已执行的更早的号）：下表按 Phase 的次序，号是预计的。每个新的约束与索引都有名字，进 `TestConstraintAndIndexNames`（`server/migrations/schema_test.go:163`）的表，各有 13.4 第 1 条的反例（第 9 节）。

| 迁移 | 改动 | 为什么 |
|---|---|---|
| 00048 notebook `notebooks`（P1 的 S1） | `mode text NOT NULL DEFAULT 'standard'`，`CONSTRAINT notebooks_mode_check CHECK (mode IN ('standard', 'llm_wiki'))`；`source_root_id uuid`（可空，不建外键，照 `transfer_jobs.root_id`）；`CONSTRAINT notebooks_source_root_check CHECK (mode <> 'llm_wiki' OR source_root_id IS NOT NULL)`。Down 去掉两列：之前的程序把每本 LLM Wiki 看作普通的（同 00028、00034 的写法，迁移的注释写明）；notebook 的 sqlc 一项列出它（`server/sqlc.yaml:55-58`；`archtest/sqlc_test.go:267-271` 要每一项列出它的模块的每个迁移） | 4.3。`DEFAULT` 让已有的行与测试里 38 处只写明列的 `INSERT` 照旧；外键会让 notebook 与 page 互相引用、打乱清理的次序（`bootstrap/registrants.go:293-296`）；检查只要求 LLM Wiki 有来源根，不要求普通的没有，M11 升级期间可以先定来源根。`notebooks` 小，检查扫一遍可以不计 |
| 00049 page `page_revisions`（P2 的 S1） | `rewrite boolean NOT NULL DEFAULT false`：这一行覆盖的每一次写都是系统的链接改写（参与者追加的写），或撤销里写回一个这样的版本。同一版本行连续写以 AND 合并，旧行 false；撤销取被撤销版本的标记，写回与撤销的撤销沿用。Down 去掉列：再 Up 时旧的标记都成了 `false`，撤销这些改写的写回在来源上被拒（只会更严） | 4.7 的"撤销的写回"；只改元数据（审查 B 在 200 万行上量过 0.5 ms）。M11 的"内容保全"也用得上 |
| 00050 page `nodes`（P2 的 S2） | `source_hash bytea`，`CONSTRAINT nodes_source_hash_check CHECK (octet_length(source_hash) = 32)`；部分索引 `nodes_notebook_id_kind_source_hash_idx ON nodes (notebook_id, kind, source_hash, id) WHERE source_hash IS NOT NULL AND deleted_at IS NULL`。Up 最后为已有的 `llm_wiki` 笔记本补上哈希：每本一条递归的语句，从 `source_root_id` 走活着的子孙，页取 `page_contents.content_hash`（字节数大于 0 的），附件取 `asset_blobs.sha256`（按 `node_id`）。它读两张别的模块的表，都只读、不改：notebook 的 `notebooks`（`mode` 与 `source_root_id`）与 asset 的 `asset_blobs`。归属的检查只看改表的语句（`archtest/sqlc_test.go:123-129`、`:235-245`），它的注释写明不看数据的语句、由审查把关（`:113-122`）；sqlc 也不理会 schema 文件里的数据的语句（审查 A 以仓库钉住的 sqlc v1.31.1 试过）。所以这是迁移读别的模块的表的第一处，到 00047 为止的迁移里没有先例，作为 7.1"只写自己的查询"的一处经过审查的例外，写进迁移的注释与"总体设计的修订"的 7.1。索引非唯一，系统改写可以使已有来源同字节；末键 id 供完整历史计划排除受影响节点时有界分页，不再另建三列索引。Down 去掉列与索引 | 去重（4.8）：只有 LLM Wiki 活着的、有正文的来源有值。补哈希是为了发布之前已有 LLM Wiki 的库（P1 合并之后、P2/S2 之前的开发库）与 `goose down` 再 `up`；发布的第一次 Up 时没有 `llm_wiki` 的笔记本，这条语句什么也不做。**锁**：一个 goose 的事务里，`ALTER` 取 ACCESS EXCLUSIVE，一直持到建完部分索引，读与写都等；检查让 `ALTER` 扫一遍表（审查 A、B 在 200–300 万行上量过 72–94 ms，不带检查 0.15–0.7 ms），建索引再扫一遍（82 ms）。升级说明本就先停服务，所以写成停机的时长：P2/S2 在 600 万节点分别测无 LLM 与已有 LLM（含真实页/附件回填）的完整 goose Up 冷启动事务；只在事务开始前冷却一次自有 PG 文件的 OS page cache，以文件 manifest/fincore 核验，承认设备缓存与启动预热。CHECK/index 分别冷却的诊断另列，不相加冒充完整事务/停机时间；不动全局缓存或用户服务。600 万迁移与 10 万来源另量四列相对原三列的空间、建索引/回填及写维护代价；实际结果写入升级说明，M12 仍重测真实服务器。不用 `NOT VALID` 再 `VALIDATE`：同一个事务里 ACCESS EXCLUSIVE 照样持到提交，省不下什么 |
| linking（P3） | 一个一定有的部分索引 `page_properties_sources_idx ON page_properties (source_id, position) WHERE translate(key, 'SOURCE', 'source') = 'sources'`，同 00047 给 `type`、`summary` 的那两个：每页的第一个 `sources` 键由它一次探到（没有它时一个横向的读要读一页的每个属性，00047 的注释），也找得到只有不加引号的 `[[x]]` 或普通字符串、因而一条属性链接也没有的页（4.11 的"`sources` 指向无效"）。按量的结果另加的至多几个：有歧义的链接（`ambiguous`，插入时为假）、属性链接（`property_key IS NOT NULL`）、解析失败的页（`NOT frontmatter_valid`）。不建 `WHERE resolved_id IS NULL` 的索引：`InsertLinks` 把每条链接先写成没有解析（`linking/adapter/postgres/queries/links.sql:2-9`），这个索引就要收每次保存的每条链接（5,000 条链接的一页一次 5,000 项，在索引锁里，M6 量过这样的保存约 70 ms），想要的页改用别的形状（4.11） | 4.11：`sources` 的那一个是规则要的；别的只给量过、确实慢的规则加，并量它在 M6 的 5,000 条链接的保存上的代价。只建索引的迁移取 SHARE：写等、读不等（读与写都等只在同一个事务里有 `ALTER` 持着 ACCESS EXCLUSIVE 时，如 00050 与 ingest 的迁移）；每个各自量扫描的用时、写进升级说明 |
| 下一个号 page `changesets` 与新表 `changeset_sources`（P4 的 S2） | `changesets.ingest_count integer NOT NULL DEFAULT 0`，`CONSTRAINT changesets_ingest_count_check CHECK (ingest_count >= 0)`；部分索引 `changesets_notebook_id_created_at_ingest_idx ON changesets (notebook_id, created_at DESC, id DESC)` 与 `changesets_notebook_id_created_by_id_created_at_ingest_idx ON changesets (notebook_id, created_by_id, created_at DESC, id DESC)`，谓词都是 `ingest_count > 0` 加 00037 的日志索引的谓词；`changeset_sources (changeset_id uuid NOT NULL, seq integer NOT NULL, node_id uuid NOT NULL, CONSTRAINT changeset_sources_pkey PRIMARY KEY (changeset_id, seq), CONSTRAINT changeset_sources_changeset_id_node_id_key UNIQUE (changeset_id, node_id))`，两列都不建外键。新表进 `server/sqlc.yaml` 的 page 的 schema 与 `deploy/runtime-grants.sql`（只要数据的授权，同 `changeset_items` 的那一行，`runtime-grants.sql:19`；不给 MAINTAIN：它只给任务要 ANALYZE 的表，`:24-29`，而没有任务写标记，标记只在 `edit` 的变更集上；13.1 第 16 条），同一个提交。Down 去掉列、索引与表：再 Up 时旧的标记没了 | 4.14。`changeset_sources` 照派生的表（`page_search`、linking 的索引表）：没有 `deleted_at`、没有清理器，也不引用 `changesets`。清理的规则（13.1 第 6 条）要求指向被清理的表的外键有排在前面的清理器，而没有 `deleted_at` 的表又不许有清理器（`bootstrap/purge_test.go:56-94`），所以它的行由清理变更集之前的一个叶子的步删除：同一个清理器里的一条语句，`LIMIT` 一批（13.1 第 6 条：先叶子、按批的行，`purge.sql:1-3`；一个变更集至多 10,000 行，跟着变更集一起删会让一条语句删到 1,000 × 10,000 行，`platform/jobs/purge.go:42`），删的是两条清理变更集的语句（`PurgeChangesets`，`page/adapter/postgres/queries/purge.sql:57-64`；`PurgeTrimmedChangesets`，`:66-75`）就要取的变更集的行，谓词同它们，以同样的行锁取那些变更集（`FOR … OF c SKIP LOCKED`）；两条语句再加 `NOT EXISTS (SELECT 1 FROM changeset_sources …)`，同它们对条目与版本的做法。这一步照 `PurgeNodes` 对它的叶子的做法循环到一批不满为止，删的行数算进清理器交回的数（`purgeTable` 只在交回满批时再调它，`platform/jobs/purge.go:94-103`）：否则一个有 2,500 个来源的变更集、或 500 个各有 3 个来源的变更集会让一轮停在中途，删除了的笔记本的变更集留过这次运行，而 `changesets.notebook_id` 是 `ON DELETE RESTRICT`（`00014_page_changesets.sql:8`），同一次运行里 `PurgeNotebooks` 就失败、之后的清理器也不跑。每条语句仍至多一批（审查 B3-M1）；清理的测试：一本删除了的 LLM Wiki 笔记本带着这两种形状，在一次运行里删完，同一次运行里 `PurgeNotebooks` 成功。它不是登记的清理器（表没有 `deleted_at`，`TestEverySoftDeletedTableHasAPurger` 不许它有）。`checkPages` 守着"只有存在的变更集有行"（笔记本删除之后、清理之前，变更集的 `deleted_at` 已设、行还在，`page/adapter/postgres/queries/nodes.sql:290`，所以不是"活着的"）与"`ingest_count` 等于行数"，后一条只对清理的谓词不取的变更集（叶子的步删到一半的、锁着而被跳过的不算）。来源可能在变更集之前被清理掉，所以 `node_id` 不引用 `nodes`；清理来源不删这些行，`ingest_count` 照旧数着清理掉的来源（读作 `null`，4.14）。主键按 `seq`：读"前 N 个"按它走索引；`(changeset_id, node_id)` 唯一，同一来源只记一次。两个部分索引给"只看 ingest"的历史与"某人的 ingest"（日志的约定：每种筛选一条语句，`page/adapter/postgres/queries/history.sql:24-26`）。锁与扫描同 00050：检查扫一遍 `changesets`，P4 量、写进升级说明 |

下文以内容称这些迁移（"`rewrite` 的迁移""`source_hash` 的迁移""ingest 的迁移"）。wiki 没有迁移。没有新的配置项：上限都是代码里的常量（4.18）。

### 4.3 模式、来源根与 M11 的状态

- **取值**：`standard`、`llm_wiki` 两种。总体设计 5.8 的图里 `upgrading`、`reviewing` 画在两种模式之间，它们是一次升级的状态，不是模式：升级期间笔记本仍按普通的契约（守卫、lint、虚拟文件都照普通），冻结与护栏是 M11 另加的守卫；7.2 也把"`mode`、升级状态、来源根页面 id"分开列。M11 加它自己的列（或 `notebook_upgrades` 的状态）与检查，不扩 `notebooks_mode_check`；以后真有新的模式时，按名字替换这个检查（第 11 节设计的取舍第 2 条）。
- **来源根**：`source_root_id` 是一页的 id，只在模板里定下：M10 不提供改指另一页（第 2 节"不做"）。来源根在 LLM Wiki 下不能删除（4.9），所以活着的 `llm_wiki` 笔记本的来源根总是活着的页（`checkPages` 守着：每本活着的 `llm_wiki` 笔记本有活着的来源根与根下的 `AGENTS`）；笔记本删除时连同它一起删除（不经单元，`page/app/extension.go:259-277`），没有悬空的 id。管理员可以改名、移动来源根（4.7），所以它的路径是一次祖先的读（`page/adapter/postgres/queries/nodes.sql:129-141`），不是一次点查。
- **降级时**清成 `NULL`：普通笔记本里没有谁维护它，那一页之后可能被删除、清理；留着的 id 只会让 M11 拿到一个过时的提议。M11 的升级重新定来源根（"写给后面的 M"）。
- **谁读它**：写入单元在笔记本行的锁下读一次（4.6）；不开事务的读（MCP 的 `list`、`read`、说明、lint、虚拟文件、导出）经 notebook 的端口不加锁地读，与降级并发时读到哪一边都是一致的一份。
- **`Notebook`** 的答复带 `mode`（必填）与 `source_root_id`（必填、可为 `null`，普通笔记本为 `null`），网页的每个笔记本页面经 `useNotebook()` 拿到（`web/…/pages/notebook/notebook-layout.tsx:25-64`）；MCP 的 `notebooks` 与 `open_notebook` 的每项另带 `mode`（P1/S1 写进 `api/mcp/tools.json` 的两个工具的说明）。

### 4.4 新建与模板

**原子性**：一本提交了的 `llm_wiki` 笔记本没有 `AGENTS` 或来源根，就是一本坏的笔记本（守卫保护的两页从一开始就不在）。今天的 `Writer.Run` 拒绝已经在事务里的 ctx（`page/app/unit.go:186-188`），笔记本的创建也没有扩展点（`notebook/module.go:52-59`）。三种做法：

| 做法 | 代价 | 结论 |
|---|---|---|
| 页面的单元加入建笔记本的事务（声明的例外） | `Writer.Run` 多一种只给模板用的入口；单元再取工作区行与笔记本行的锁是同一事务里的重入（`platform/postgres/tx.go:121-129` 的 `WithinTx` 本就加入外层的事务），判定读到同一事务里刚插入的管理员成员（`notebook.NewFacts(pool)` 经 `postgres.DB(ctx, pool)`） | 选这个，与下一行合起来 |
| 笔记本创建的播种者（新的扩展点） | notebook 的 `CreateNotebook` 在插入笔记本与管理员之后、发可见性之前调用 `Deps.Seeders`；组合根交 wiki 的模板 | 选：notebook 不知道模板，wiki 不写 notebook 的表 |
| 第二个事务加补偿（失败时删掉笔记本） | 提交与补偿之间别人看得到一本空的 `llm_wiki` 笔记本；补偿自己也会失败；删除走软删除与 60 天的清理 | 不选 |

做法：`CreateNotebook` 在要建 `llm_wiki` 时先定下来源根的 id（UUIDv7），以 `llm_wiki` 与这个来源根直接插入笔记本（检查 `notebooks_source_root_check` 因此成立）与管理员，再调用每个播种者（值里带笔记本、模式、来源根的 id、模板的语言、执行者、客户端与时刻：13.1 第 21 条"事件的值带时刻与执行者，订阅者用这个时刻写自己的行"）。wiki 的模板在 `mode` 是 `llm_wiki` 时经 `TreeWrites().Seed` 跑**一个**改树的单元，以这个 id 写 `raw`、再写 `AGENTS`，交回它写下的来源根 id（`Seed` 交回它写下的页的 id，模板取其中来源根的那一页的；P1/S1 审查 A-N3）。`CreateNotebook` 核对**恰有一个**播种者交回来源根的 id、且等于先定下的那一个，交回的是写下的，所以这一核对说的是来源根写下了；没有、两个、或一个不同的 id 都是缺陷（500），整个回滚：一个没有播种者的组合、一个什么也没写的播种者或两个都写了的组合都不会提交一本坏的笔记本（别的播种者交回空）；`notebook.New` 在 `Seeders` 为空时 panic（同 `AccountWorkspaces`，`notebook/module.go:89-91`）。然后发可见性的事件、提交。播种者交回它写下的东西，与 13.1 第 21 条"订阅者只交回错误"不同，作为一个新的扩展点写进 12.4 与 13.1 第 21 条。

模式在建的事务里从不改变：模板的单元读到的 `Write.Notebook` 就是 `llm_wiki` 与这个来源根，守卫照常检查模板（第 11 节主持者的决定）。它对今天的模板没有可拒的：`raw` 在根下、id 是来源根，是来源根而不是来源；`AGENTS` 与 `raw` 都不是保留的标题。以后的模板若在根下写 `index`，守卫照样拒绝。单元的观察者（页面事件的 `NOTIFY`、链接索引、附件）与搜索的入队照常，都在这个事务里；页面事件的 `NOTIFY` 照样发出，但没有一个流会转给它：流在打开时定下它转发的笔记本（`events/app/stream.go:89`、`:122-124`），新的笔记本不在任何一个里面，所以没有故事等它（第 9 节）。任何一步失败，整个创建回滚；播种者的有类的错误答 500 并记下它的码，ctx 的取消与期限照原样。建 LLM Wiki 因此在笔记本的管理事务里锁笔记本行、走页的分支：13.1 第 5 条"建笔记本只锁工作区行"改写（第 7 节"总体设计的修订"）。

`Seed(ctx, SeedSpec{NotebookID, At, Client, Message, Pages}) ([]uuid.UUID, error)` 是 page 的 `TreeWrites()` 上的新方法，只给这一处，交回它写下的页的 id（照 `Pages` 的次序；交回的个数与页数不同是缺陷）：它要求 ctx 已在事务里（与 `Run` 相反）；`Pages` 是 `PageDraft` 的列表（`PageDraft.ID` 本就有，`batch` 的计划用它，`page/app/unit_create.go:16-19`），各带 id、父节点、标题与正文，一次调用一个单元、一个变更集（`NotebookID` 是单元锁的、判定所在的笔记本，`Message` 是变更集的说明，所选语言的"LLM Wiki 模板"）；时刻与客户端取值里的，不读 `w.d.Clock.Now()`（`page/app/unit.go:230`），所以笔记本与模板是同一个时刻（13.1 第 9 条：一个操作读一次时钟）；执行者取 ctx 的（同 `Run`，`page/app/unit.go:197`；守卫问 `page.manage_source` 也用它），`SeedSpec` 不另带一个，免得两者不一致（审查 A2-N3）；其余同 `Run`（锁、判定 `page.create`、单元、观察者）。这是 13.1 第 1 条"单元拒绝已经在事务里的 ctx"的例外，写进 13.1；它不是给别的 M 的扩展点，12.4 只列播种者。模板的正文在启动时经 `TreeWrites` 的另一个新方法 `ParseTemplate(content) (Facts, error)` 解析一次（两种语言各两页，几 KiB）：事务里只做锁、比较与写（13.1 第 19 条），提取结果是静态的；13.3 第 3 条"判定之后、解析预算之内"为这一处写明例外。

**客户端**：模板的变更集的客户端是建的请求的（`web` 或 `api`）。今天三个 HTTP 适配器各有一份（page、asset、transfer：`page/adapter/http/handler.go:351-362`、`asset/adapter/http/upload.go:161-171`、`transfer/adapter/http/server.go:103-113`），notebook 的没有；为了不出现 13.1 第 2 条的第二份实现（13.4 第 13 条），这个函数挪到 `shared`（`ClientOf`，在 `CleanClientName` 旁边，读请求的 `Actor`），四个适配器都调它、各自转换为自己的类型，三份都删，13.1 第 2 条照此修订。

**模板的内容**（wiki 的 `domain`，中文与英文各一份，入库、有测试）：

- `raw`：来源根。正文是一段说明：这里放原始资料（文章、论文、记录、PDF 与图片），写进来之后不改；在网页上用"粘贴或上传为来源"，agent 用一次 `write`；来源区里的改名、移动、删除只有管理员能做。
- `AGENTS`：LLM Wiki 的约定模板：页面的类型与 frontmatter（`type`、`summary`、`sources` 的写法，`sources: ["[[raw/…]]"]`，并写明不加引号的 `sources: [[raw/…]]` 是嵌套的列表、不算引用）、来源区的规则、`index.md` 与 `log.md` 是生成的、ingest / query / lint 三个 prompt 在笔记本的入口 `/mcp?notebook=<id>` 上怎么用；结尾一段"按你的领域改写这一页"。至多 8 KiB。测试钉住两种语言的段落一一对应（反例：块数的少一段、末尾多一段，行内代码的少一项、一处不同，每种块的形状各一个：两项写在一行、标题降一级、围栏少一行、段落拆成两行；13.4 第 1 条），并钉住模板里没有代码之外的 `[[raw/…]]`：新建的 LLM Wiki 在两种语言里都 lint 干净（4.11）。
- 两页不带 frontmatter（lint 对 `AGENTS` 与来源根豁免缺类型、缺摘要，4.11）。

**语言**：服务端不知道界面语言（`web/…/session/auth-middleware.ts:36`），所以客户端在建的请求里带 `template_locale`（`zh-CN` 或 `en`，契约的枚举）；没给时是 `en`，网页总是带上；服务端持有两种语言的模板；之后由用户改写（负责人 2026-10-24，第 11 节负责人的决定第 3 条）。只在 `llm_wiki` 时有用，但两种模式都核对它：不认识的 `mode` 或 `template_locale`（如 `"fr"`）是 `CheckDraft` 的 422 `invalid_format`，排在 `workspace.not_found` 与 `forbidden` 之后，同 `workspace_access`（`notebook/domain/notebook.go:97-104`、`notebook/app/create_notebook.go:57-66`）。

**变更集**：类型 `edit`，客户端是建的请求的，执行者是建的人，说明是所选语言的"LLM Wiki 模板"/"LLM Wiki template"；网页的历史里它是笔记本的第一个变更集。撤销它要删掉 `AGENTS` 与来源根，撤销的预检在开始时拒绝（4.9）。

**界面**：建笔记本的对话框加模式的选择（普通在先、默认），带一句说明；`template_locale` 取界面语言。新手引导照旧建普通的个人笔记本（`web/…/notebook-step.tsx:56-90`）。

### 4.5 降级与模式的事件

- **动作**：`POST /api/v0/notebooks/{notebook_id}/downgrade`（`downgradeNotebook`，操作 `notebook.change_mode`，笔记本管理员；M11 的发起与批准升级可以用同一个操作）。次序同笔记本的管理写（13.1 第 5 条）：工作区行 `FOR SHARE` → 笔记本行 `FOR NO KEY UPDATE` → 判定（404 → 403）→ 不是 `llm_wiki` 答 409 `notebook.not_llm_wiki` → 有排队或运行的写入任务（导入、后台的恢复与撤销）答 409 `transfer.busy`（经 `transfer.NewWritingJobs(pool)`，同 `transfer/app/start_import.go:132-140` 的探查；跨前缀的码有先例：notebook 答 `workspace.not_found`）→ 写：`mode = 'standard'`、`source_root_id = NULL`、`updated_by`、`updated_at`，经 `page.NewSourceArea(pool).Forget` 清掉这本笔记本的 `nodes.source_hash`（一条语句，`WHERE notebook_id = $1 AND source_hash IS NOT NULL AND deleted_at IS NULL`，走 `source_hash` 的部分索引：两种删除都清掉哈希，`DeleteNodes` 与笔记本删除的 `DeleteNotebooksPages`，4.8，所以删除了的节点从不带它，这一句就是确切的，读的只是来源；它在笔记本行的锁下挡住这本笔记本的写，P2 的成本守卫在 10 万个来源上量它；P2 的 S2 随这一列加上，P1 的降级没有这一步）→ 调用模式变化的订阅者 → 提交。答复是改完的 `Notebook`。
- **为什么有任务时拒绝**：任务的批之间放开笔记本行（总体设计 8.5），降级落在两批之间时，一次导入的前半按 LLM Wiki 的契约（去重、来源区）、后半按普通的。拒绝是一次读，任务结束之后再降。开始任务的事务以 `FOR SHARE` 锁笔记本行（13.1 第 5 条），与降级串行。任务的行之外，一次正在上传的导入与一次正在开始的任务只是进程里的占位（`transfer/app/start_import.go:82-86`、`start_background.go:125-136`），降级看不到它们、照样成功；它们写下任务的行时以 `FOR SHARE` 锁笔记本行，排在降级之后，按普通的契约走。
- **降级之后**：`raw` 是普通的页（谁都能改、移动、删除），`AGENTS` 可以删除、改名，虚拟的 `index`、`log` 消失（真实的同名页从此可以建），frontmatter 原样（总体设计 5.8）；不写变更集（没有页面的写），日志记一行 info，只带 id（`workspace_id`、`notebook_id`、`user_id`），不带名称，有日志的测试（13.1 第 10 条"每个写用例的日志测试"）。M10 没有审计行：`notebook_audit_events` 只记归属的事（`00011_notebook_notebook_audit_events.sql:15-16`）。
- **网页**：设置的"常规"一页加"模式"一节：读者看到模式；管理员在 LLM Wiki 里看到"降级为普通笔记本"。确认对话框是普通的确认（不要输入笔记本的名称，第 11 节主持者的决定：什么也不删除），写明后果：来源区成为普通的页、`AGENTS` 可以删除、虚拟的两行消失、去重停止，以及"升级是另一个流程"（M11 在 v0.1 之内做成之后这句仍然成立）。`NotebookStore.downgrade` 经这本笔记本的 `inTurn` 队列发出、放进答复的 `Notebook`（`web/…/stores/notebook.store.ts:43-45`、`:100-106`；13.2 第 1 条）；403、404 与 409 `notebook.not_llm_wiki`（别的管理员先降了）重读笔记本的列表；对话框是持有的 `ConfirmDialog`（`held`），成功之后 `onClosed` 把焦点移到这一节的标题：带触发按钮的对话框会随 403 之后重读的列表卸下（13.2 第 17 条）。`transfer.busy` 的现有文字说到"你对它的导出还在进行"（`web/…/i18n/messages/zh-CN.ts:915`），降级不看导出，所以经 `texts` 给它自己的文字。`useFollowRole` 不合用（它是管理员的读的 `onError`，`web/…/app/follow-role.ts:11-17`）。
- **空的普通笔记本直接切到 LLM Wiki**：M10 不做。M11 的升级覆盖它（空的笔记本过得了每一道闸门；总体设计 5.7"空笔记本天然满足"读作新建与升级都不必另写什么，"总体设计的修订"照此写明）；在 M10 加，是在 M11 的状态机之外开一条改模式的路（第 11 节设计的取舍第 5 条，负责人可以改判）。
- **事件**（[M5 的移交](handoffs/M5-events.md)，照 [给 M6 的事件类型移交](../M6-links/handoffs/M5-events.md) 的做法）：新类型 `mode`，按笔记本过滤，帧是 `{workspace_id, notebook_id}`，发布的数据是 `{}`（两个 id 由流写进帧，`events/domain/frame.go:38-57`；只带 id，13.1 第 30 条）；在写模式的事务里经组合根的适配以 `events.Publisher.Publish` 发出（照 `linkEvents`，`bootstrap/linking.go`），是 notebook 的新扩展点"模式变化的订阅者"的第一个注册者（模块根的替身测试与两个注册者的次序测试照 13.1 第 21 条，同创建的播种者）。契约加 `EventMode`，进 `streamEvents` 的 `oneOf` 与帧的列表（`api/modules/events.yaml:21-86`），`services/event.service.ts` 导出这个类型（M6 的移交第 1 条）。前端的 `eventHandlers` 加 `mode`（`web/…/events/handlers.ts:276-282` 是那张表，预告 M10、M11 的注释在 `:12`）：重读 `["notebooks", 工作区]`（模式与角色从这里来），只此一行。健康度面板与目录视图不在处理的表里（它们不是由事件驱动的视图，13.2 第 23 条）：`getLint` 与 `listIndex` 答出它们算时的模式（`mode`），视图拿它比 `useNotebook().mode`，不同时写"模式已改变，请重新检查"、不重算（4.12、4.16）。所以断线期间错过的降级在重连之后也看得出：`refreshedOnConnect` 重读笔记本的列表（审查 C2-M1、B2-N7）；不重读 `["pages", 笔记本]`（降级不写页，虚拟的两行由 `Notebook` 定）；`refreshedOnConnect`（`web/…/app/event-stream.tsx:15-35`）不加新的键。前端已有的替身类型就叫 `mode`（`web/…/app/event-stream.test.tsx:204-217`），换成真的；那条一般的测试换一个编造的类型留着。建笔记本不发 `mode`（新笔记本经可见性的事件让别人的流重连；它的页面事件没有流转发，4.4）。M11 的状态变化发同一个类型（[M11 的移交](../M11-upgrade/handoffs/M5-events.md)）。工作区级的类型那一处缺口（访客接受邀请等不发 `access`）与按笔记本过滤的类型无关。
- **降级之后的页面**：目录视图两种模式都有，照旧；树里的虚拟两行消失；停在一页来源的只读视图上的人随 `mode` 事件看到它成为普通的页（可以编辑，`isSource` 由 `Notebook` 的模式与树算出，4.16），去重从此不再拒绝。

### 4.6 守卫：输入、位置、次序与拒绝的去处

**今天守卫看不到的**（`page/app/extension.go:27-55`）：执行者的权限（单元判定之后丢掉 `Grant`，`page/app/unit.go:225`）、笔记本的模式、页面正文的哈希与写之前的大小（单元算了哈希，`page/app/unit.go:556-560`，不交给守卫）、一次写是不是系统的链接改写（参与者追加的写与用户的写同样，`page/app/unit.go:658-661`）、`batch` 把几个正文操作合成了一步。

**加的输入**（都是 M4、M9 代码的声明的改动，第 8 节）：

| 加在哪里 | 什么 | 从哪里来 |
|---|---|---|
| `Write.Notebook` | `NotebookState{Mode, SourceRootID}`（M11 在这里加升级的状态） | page 的 `Notebooks` 端口的 `ShareByID`、`LockByID` 改为交回它（方法名不变，交回 page 的 `domain.NotebookState`，`page/app/ports.go:54-63`）；notebook 的根另加 `StateOf`（P1/S1，不加锁，`open_notebook` 与预检用）、`ShareState`、`LockState`（P1/S2），它的 `ShareByID`、`LockByID` 不改（transfer 与 linking 的端口用它们），page 经组合根的 `pageNotebooks` 逐字段转换；notebook 的 `ShareNotebook` 今天只选 `id`（`notebook/adapter/postgres/queries/notebooks.sql:22-28`），加两列，在笔记本行的锁下读：降级要 `FOR NO KEY UPDATE`，与每个单元的锁互斥，所以一个单元里模式不变（建笔记本的单元也是，4.4） |
| `Step.Content` | `*StepContent{Hash, Bytes, BeforeBytes, Merged}`（类型名定为 StepContent，避免已有名称：`page/app` 已有一个 `ContentMeta`，`ports.go:104-109`，版本号、字节数与最后写的人与时刻）：这一步写下的正文的 SHA-256 与字节数（此Content只放页面正文；附件仍用Step.Asset，S1附件的Step.Content为nil），写之前的字节数（新建时为 0），`Merged` 是合进这一步的正文操作之中、在那一项时这一页是来源（计划的树里来源根的严格后代，4.7；来源根自己的说明不算）而且改了它的字节的个数（`batch` 新建的页从 0 字节算起；字节不变的写不算，同 `writeContent`；`batch` 之外是 0 或 1） | 单元已经算出（`Unit.content`）、已经读出当前的正文（`page/app/unit_content.go:62-66`）；附件的取自上传的 `Step.Asset`（`page/app/extension.go:128-135`）；`Merged` 由 `batch` 的单元从计划交出（计划知道每一项时的树，`TreeAt`；`page/app/apply_batch.go:602-690` 合并一页的正文操作的那两处） |
| `Step.AreaHashes` | 移入、恢复进来源区的这一步的每个节点的哈希（页的正文的、附件的文件的），按节点 id | 单元一条语句读出页的（`page_contents`；恢复时这些页还在回收站里，所以不论 `deleted_at`：`DeleteNodes` 把它们的正文行标了删除，`nodes.sql:200`，`RestoreNodes` 只在写里清掉，`:239`），经 `WriterDeps` 的附件端口一次读出附件的（它本就读标了删除的行，`page/app/ports.go:558-569`）（4.1、4.8；审查 A3-N3）；守卫的查重与 `source_hash` 的维护都从这里取 |
| `Step.Rewrite` | 这一步是系统的链接改写：参与者经 `Appender` 追加的写（含 `batch` 推后的参与者），或撤销里写回一个 `rewrite` 的版本（4.7） | `appender.WriteContent` 与撤销的 `RevertContent`（计划带上版本行的 `rewrite`） |
| `Step.Lines`、`Step.Kinds` | 可选 `StepLines{Before, After, AfterParents}`；每个改动节点的 Kind | nil 祖先线表示未提供，已知空线表示笔记本根；Create/Move/Relocate 与直接 import insert 复用已读的线，其余只在 LLM 模式按需补操作根的祖先。Before/After 分别算严格后代，根自身不算来源，附件也算；不按后代逐个查询、不从 Content 猜种类。AfterParents保留恢复跳过已活成员后每个外父的独立祖先线，Changes当前父图优先，不能将森林拼成一条线。 |

**权限**：来源区里要管理员的步，守卫经组合根交的 `shared.Authorizer` 问 `page.manage_source`（规则表里给笔记本管理员，4.7）；一个单元至多问一次（第一次碰到这样的步时问）：守卫是所有并发的单元共用的一个注册者，记不住答案，所以结果记在单元自己的一格里：`Run` 与 `Seed` 在 `Write` 上放这个单元的一个备忘 `Write.Grants`：单元拥有的一个指针（问过的动作与答案）。不能是一个值：`Step` 按值带着 `Write`，每个守卫按值拿到 `Step`（`page/app/extension.go:41-42`、`page/app/unit.go:304`），值会在副本里填了又丢掉，守卫就每一步都问。模式的守卫与单元自己的锁下的检查写它，单元之前的预检另问、不写它，别的扩展（参与者、观察者）看得到、不写（审查 A2-N4、A3-N2）。普通笔记本与不碰来源区的单元一次也不问。它在单元的事务里读，所以与单元的 `page.*` 判定看到同一份成员；后台任务每批的单元重新判定，所以是那一批时的权限。不再要 `Write.Role`。

**守卫要的祖先**（P2/S1，随来源区的第一条规则；P1 的规则不读祖先：`AGENTS` 在根下，来源根按 id 认）：凡是步上没有带着父节点的一线的，Unit.withSourceLines在guards之前于 `llm_wiki` 下按需读一次并复用，GuardWrite只消费事实（一条递归的语句，至多 10 层，`page/adapter/postgres/queries/nodes.sql:129-141`）：写正文、改名、删除，附件的新建（`unit_create_asset.go:26-55`）、同一个父节点下的 `Relocate`（`unit_relocate.go:48-50`）与撤销的放回（`unit_revert.go:129-168`）都是这样。新建页面、移动与移动加改名时单元已经读出这一线（`page/app/unit_create.go:36`、`unit_move.go:26`、`unit_relocate.go:51`），交给守卫，不再丢掉。普通笔记本里模式的守卫没有规则（来源区、保护的页与保留的标题都只在 LLM Wiki），看到 `Write.Notebook.Mode` 是 `standard` 就返回，零额外来源读，不包括请求原有响应读取。 撤销的 content-only 计划没有整树时只补相关节点的祖先线、当前/目标 hash 与 bytes，不读整本树。compact 后台计划保存 Rewrite/Bytes，标记取被撤销行而非 base；S1 就接后台实际动作与批界，S2 再加 hash/snapshot（P2 §3.6、§3.13）。 P2/S2的同步`SyncRestoreNode`是普通模式读数的一处明确增量：原权限、root/group与历史界之后、正文解析之前新增一次StateOf模式点读；普通模式仍不增加来源snapshot或来源元数据扫描。这个点读不归入S1写guard本身的零额外来源读结论。

**位置与次序**：`pageRegistrants(pool, authorizer)` 里 `guards: [mode, lock]`、`sessionVetoers: [mode, lock]`（`bootstrap/registrants.go:253-264`；模式的否决者随 P2/S1 的来源的会话规则）。几个守卫都拒绝时答登记在前的码（12.4）：模式的拒绝是长久的（权限、来源只读），锁是一时的；先答锁，用户等锁放开、再试，才得知其实不许。写进 12.4 的那一行；M11 的冻结建议排在最前（冻结期间整本只读，比哪一条都先），由 M11 定。整个程序的测试钉住这个次序：别的账户的编辑会话持着 `AGENTS` 时删除它，答 `page.protected`（第 9 节）。

守卫的码排在领域的码之后（13.1 第 4 条）：写者把一页来源改名成 `a/b` 先答 422，改成兄弟的名称先答 409 `page.title_taken`，然后才是 403 `page.source_admin_only`。这不泄露什么，13.1 第 4 条的修订写明守卫的 403 在领域的 422、409 之后。模式的守卫自己的码依次：`page.protected` → `page.virtual_file`（保留的标题）→ `page.source_too_large`（来源移入的提前工作界）→ `page.source_immutable` → `page.source_duplicate` → `page.source_admin_only`。所以写者删除 `raw` 的一个祖先答 409 `page.protected`（管理员也删不了，不把写者支去找管理员）；写者移进来源区的重复答 `page.source_duplicate`，在任何要管理员的码之前。 新工作界保留已知参数/parent/title/cycle/protected/virtual_file 在前，overflow 可先于尚未读到的子树深度错误；这是 4.8 明示的错误次序例外。

**规则**：分成两部分（审查 A-M9）。

- **没有数据的规则是纯函数**：`page/domain` 的 `CheckContract(n NotebookState, s StepFacts) Verdict`（契约取笔记本的状态：模式与来源根，P1/S2）：输入是这一步的操作、每个改动的节点在这一步之前与之后在不在来源区、是不是来源根或 `AGENTS`、根下的名称键、`Step.Content`、`Step.Rewrite`、单元的类型、是不是编辑会话的保存；输出是 4.7、4.9 的码，或"这一步要 `page.manage_source`"。调用方（守卫或预检）在后一种情形问一次权限（见上），没有时答 `page.source_admin_only`。函数不读库，所以守卫与预检不会走散。
- **去重是一次探查**：守卫在 `CheckContract` 放过之后、按这一步新增进来源区的每个节点的哈希探 `source_hash` 的部分索引（4.8）。导入的单元同样。`batch` 的计划不去重：去重是每一步的，守卫拒在那一项上。

**预检**：

| 预检 | 时机与判什么 | 拒绝在什么之前 |
|---|---|---|
| `batch` 的计划（`PlanBatch`） | 对每项的正文规则（写一次之后不改、第一次正文只从 0 字节开始），**计划是权威**：守卫只看到合成之后的一步（`apply_batch.go:616-619`、`:666-668`、`:676`：`[write raw/x "A", edit raw/x → "B"]` 在单元里只是一次新建一页来源、正文 "B"），所以这两条只能由计划按项判，"预检漏判的，单元里照样拒绝"对它们不成立。计划的两遍各判各的：树的一遍在模拟的树上按项的次序判来源区、保护的页、保留的标题与权限，所以 `[改 P，把 P 移进来源区，再改 P]` 在第三项被拒，与分开调用相同；正文的一遍（解析每项的正文时，那一刻的模拟的正文与大小都在手上）判写一次与第一次正文；字节不变的写当作没有写，同 `writeContent`（字节数与哈希都相等时在 `apply` 之前就返回，`page/app/unit_content.go:65`、`:80`），所以计划自己也不拒 `[write raw/x "A", write raw/x "A"]`。计划不去重（见上）。另有一道后备：`batch` 的单元交出 `Step.Content.Merged`，即合进这一步的正文操作之中、那一项时这一页是来源（来源根的严格后代）而且改了它的字节的个数（新建的页从 0 字节算起）；它大于 1 就是一页来源的第二次改动，分开调用总会拒绝（第一次改动之后它就不是空的了），守卫拒绝（`page.source_immutable`），计划的检查退化时前两种形状也拒得住。只数页是来源时的那几项，所以管理员把页移出之后再写的形状（`[write raw/x "A", 把 raw/x 移到 wiki/x, edit wiki/x "B"]`、`[edit 空的来源 S "A", 把 S 移出, edit S "B"]`，分开调用都成功）不被后备误拒（审查 A2-M1）；只数改了字节的，所以写者的 `[write raw/x "A", write raw/x "A"]` 与 `[write raw/x "", write raw/x "B"]`（分开调用都成功）也通过（审查 A3-M1）。这四种都有测试、都通过，A-I1 的两种照样拒绝。剩下的一种误拒要管理员：把页移出、清空、移回、再写；写进第 10 节。`batch` 答守卫的拒绝时指出这一步的那一项：新建的页是新建它的那一项，已有的页是它的第一个正文操作（单元在那里写下合成的正文） | 单元与任何正文的解析之前（`page/app/apply_batch.go:261-312`）；编辑锁的预检照 M9/P3 在整个计划之后（`apply_batch.go:294`） |
| 上传的 `Check` | 不加锁、读文件之前（`page/app/asset_writes.go:47-54`）：根下的保留标题（上传进来源区是新增，写者能做，不问权限）；排在 `Check` 已有的码（领域的 422、409）之后、asset 的 `storage_full` 之前（`storage_full` 是 asset 模块在 page 的整个 `Check` 之后查的，两者都不写）。去重要文件的 SHA-256，只能在流式写完之后的单元里判（守卫） | 读文件之前 |
| 撤销的核对（请求里不加锁的一遍、锁下的一遍） | 计划做成之后（`page.revert_conflict`、它的 trimmed 与步数的 422 之后；不加锁的一遍在 `revertChecks.check` 里，所以在 `page.locked` 与 503 之前，被拒的撤销不占解析的预算），同步按原 Ops 的每一步判权限、保护的页、保留的标题与写一次；后台使用下面的实际动作/名义批协议，不先再跑一遍同步来源事实 | 单元之前 |
| 后台任务的开始（`startTrashRestore`、`startRevert`）与 worker 规划 | 基本错误之后、完整来源读之前 admit：CheckRestore 增加与 CheckRevert 一致的回调，恰一次 claim；后续读取/来源/取消/排队错误或成功提交均释放，普通模式只多一次必要模式点读、不读完整来源计划。LLM 在只读 repeatable-read snapshot 内读相关元数据、按实际动作跑 CheckContract 与 hash 模拟；不取正文。恢复按恢复计划；撤销按 compact→未缩减名义 cuts→本批同组一次恢复后的动作顺序，不能拿同步 RevertFacts 代替。S1 共用生产 batchEnd/JobBatches 与结构动作遍历，start/worker 使用同一有效 Batch 配置，nil/非法 cuts 是内部错误；S2 加 hash/snapshot。名义预检拒绝不入队、不搜索更细分法。保留 M9 Joined、同组批内合并、ErrBatchTooLarge 的 step 下标 halving、503 同批重试与成功后重新贪心；预检不是执行调度。即使没有并发，运行时缩批也可能使原来允许的任务来源拒绝，前批保留、失败批回滚；同步/后台结果可不同（主持者的决定43，P2 §3.13）。模拟前批不算已提交来源，实际 RunBatch 每单元捕获真实 baseline | 请求预检在入队前；worker 重新规划和每批 guard 再判，开始通过不保证运行时所有批通过 |
| 导入的单元 | 每个节点建之前判保留的标题（加序号，4.9）与去重（重复的页建成空的文件夹、附件跳过，都写进报告，4.8） | 那个节点的写之前，不让整个单元回滚 |

守卫仍是最后的权威，`batch` 的正文规则除外（上面）。

**拒绝的去处**：

- **标出拒绝**：模式的每一个拒绝都包一层（`Refused`，带 `Unwrap`），不论出自守卫还是预检：`CheckContract` 的调用方与去重的探查都经同一个构造函数出错，`Unit.apply` 再给别的守卫的错误包上同一层。`errors.As` 照样拿得到 `*shared.Error`，答复照旧。`page.Refusal(err)` 认出它。
- **`batch`**：`failed` 先认出它、答 `OpFailed{Op: i}`，不再交给 `replans`（`page/app/apply_batch.go:576-597`：守卫答 `page.not_found`、`validation_failed`、`page.title_taken`、`page.cycle`、`page.too_deep` 时今天会被当成树变了而重新计划，守卫的错误就丢了）。M10 的码本就不在那个集合里；标出之后 M11 的冻结、以后任何守卫都不受它约束。
- **后台撤销与恢复**：`revertDefect`（`page/app/unit_revert.go:913-918`）先认出它，原样交回，不当作缺陷；两批之间执行者的角色被降低时，那一批的守卫拒绝，同样标出。
- **transfer 的任务**：组合根的 `treeError` 先认它已有的九个哨兵（都在认拒绝之前，`ErrLocked` 是最后一个，`bootstrap/transfer.go:243-244`；碰到锁的后台撤销照旧以 `page_locked` 与 `report.Lock` 停下，有测试），再经 `page.Refusal(err)` 认出模式的拒绝，换成 transfer 的 `Refused{Code, NodeID, Err}`（`Err` 是 page 的拒绝，`Unwrap` 到它）；`unitFailure`（`transfer/app/batches.go:87-96`）在 403/404 那一支之前认它，任务以新的失败 `refused` 停下，报告带 `refusal: {code, node_id}`，已写的批留着（同别的停下）。今天那一批的守卫的拒绝会成为 `internal`。网页的报告照 `refusal.code` 取那个码的文字（`problem-messages.ts` 里已有）。后台的这条路在 P1/S3 做，同开始时的预检：P1 的规则已经让得出一个后台的拒绝（一次后台撤销要改回 `raw` 的名称或位置，两批之间执行者从管理员被降为写者，下一批的守卫答 `page.source_admin_only`），它就是 P1/S3 的整个程序的测试；P2/S1 只加来源的情形（审查 C2-M4）。
- **码**：每个操作的 `x-problem-codes` 加上它可能答的模式的码，逐个操作列在第 5 节，按 Phase 分批加（`apitest` 按操作双向核对，13.1 第 8 条；守卫的码要写进每个被扩展的操作，13.1 第 21 条），每个码在那个操作所在模块的 `adapter/http` 测试里答出过（13.1 第 8、21 条：page、asset、transfer、notebook、wiki、linking），另有整个程序的测试；`mcpToolCodes()`（`bootstrap/permission_matrix_mcp_test.go:84-108`）的行也列在第 5 节。

### 4.7 来源区的规则

"来源"是来源根的严格后代。来源根自己不是来源：它的正文是说明，写者可以改（总体设计 5.4），下表的规则不碰它的正文。下表的"新增"写者以上可以做；"管理员"指执行者有 `page.manage_source`（规则表里给笔记本管理员，守卫经授权者问，4.6），没有时答 403 `page.source_admin_only`；正文的修改答 409 `page.source_immutable`。负责人的规则（第 11 节负责人的决定第 1 条）照字面：来源区里的节点的每一种树操作（改名、移动、排序、删除）都要管理员；往来源区里加东西（新建、上传、移入、导入、恢复）是新增。

| 操作 | 落到来源上时 | 码 |
|---|---|---|
| 新建页面（REST、MCP 的 `write`、`batch`、导入，带正文） | 新增；带正文的要过去重（4.8）；空的页（MCP 与 `batch` 补建的父页、网页的"新建页面"）照样新增，是文件夹 | `page.source_duplicate` |
| 上传附件 | 新增，按 SHA-256 去重（流式写完之后，单元里） | `page.source_duplicate` |
| 写正文（PUT、自动保存、勾选任务、MCP 的 `write` 覆盖与 `edit`、`batch`） | 只许两种：系统的链接改写（`Step.Rewrite`）；空的来源（写之前 0 字节）的第一次正文，一次，只给一次性的写（不在编辑会话里：`Step.EditSessionID` 为零，即不带会话的 PUT 与 MCP 的 `write`），过去重。编辑会话的保存在任何来源上都拒绝，即使有效旧会话保存的字节相同也拒绝（审查 A-M1：一页空的页被人编辑时被别人拖进来源区，第一次自动保存不能成为它的正文）。`batch` 里这两条由计划按项判（4.6）。别的一律拒绝 | `page.source_immutable`、`page.source_duplicate` |
| 开启编辑会话 | 来源上拒绝（否决者，同锁的否决者的位置）；来源根可以 | `page.source_immutable` |
| 改名 | 来源、来源根：管理员 | `page.source_admin_only` |
| 移动、排序（同一父节点下的移动也是 `OpMove`，`page/domain/change.go:75-88`）、MCP 的移动加改名 | 移入来源区（之前不在、之后在）：新增，子树里每个带正文的页与每个附件过去重；移出、在来源区里移动或排序：管理员；移动或排序来源根自己，或把它的祖先移到别的父节点下（来源跟着换位置）：管理员，来源区是空的也一样；同一父节点下排序它的祖先不要（来源区没有换位置） | `page.source_admin_only`、`page.source_duplicate` |
| 删除 | 子树里有来源：管理员；子树里有来源根：一律拒绝（4.9） | `page.source_admin_only`、`page.protected` |
| 还原版本（`version_restore`） | 来源上一律拒绝（即使目标字节相同）（还原旧的字节就是改正文，也会撤回链接改写，留下断链）；来源根与 `AGENTS` 照常 | `page.source_immutable` |
| 从回收站恢复（`trash_restore`，同步与后台的每一批） | 落进来源区：新增，过去重（比的是活着的来源，4.8）；父节点不在而落到根下：不再是来源 | `page.source_duplicate` |
| 整组撤销（`revert`，同步与后台的每一批） | 每一步照它本来的操作判（下面） | 同各操作 |

来源 version_restore 与有效旧会话的 same-byte 入口通过窄的同一纯契约预检拒绝；changed 写仍经注册 guard，两类证据分开。普通模式、非来源和普通一次性同字节写保留原 no-op/锁行为。MCP edit 在原领域检查之后、编辑锁/解析预算之前共享来源预检（P2 §3.3）。

**撤销与历史**（[M8 的移交](handoffs/M8-history.md)第 3 项，[M9/P1 的移交](handoffs/M9-P1-mcp-registry.md)第 7 项）：

- **写回 C 改写过的来源页**：撤销一次改名或移动 C 时，C 的链接改写碰过的来源被逐字节写回。C 的那一版是参与者追加的写，版本行的 `rewrite` 为真（`rewrite` 的迁移，`RecordRevision` 的 `ON CONFLICT` 以"与"合并：同一变更集里用户的写与改写落在一行时为假，`page/adapter/postgres/queries/changesets.sql:52-63`）；撤销的计划把它带到写回的一步，这一步的 `Step.Rewrite` 为真，守卫放行：撤销系统的改写同样不算修改。撤销写下的这一版也记 `rewrite`，所以撤销的撤销同样放行。C 的那一版不是改写（来源的第一次正文；M11 升级之前的历史里用户的写）时，写回是修改，拒绝。M10 之前已有的版本行都是 `false`：LLM Wiki 的笔记本从 M10 才有，没有这样的旧行；M11 升级的笔记本里旧的改写会被当作修改（偏严）。
- **还原来源页的旧版本**：一律拒绝（上表）。
- **从回收站恢复来源、撤销一次删除来源**：新增（写者可以），过去重。删除来源只有管理员能做，恢复它不要：恢复是把资料加回来，不改任何已有的来源。
- **撤销一次新建来源（删掉它）、一次来源的移动或改名**：照删除、移动、改名，要管理员。所以撤销一次带着新来源的 ingest 要管理员；只写 wiki 页的 ingest（常见的）写者照常撤销。写者把一页移进来源区之后，移出它（包括撤销那次移动）也要管理员（第 11 节设计的取舍第 8 条，主持者照准；网页在拖进来源区之前确认，4.16）。
- **临时名称的步**（`AsideName`，三条后备）：守卫逐步判，临时的改名与移到根下照普通的改名、移动判，**不加标记、不凭名称认**。理由：临时的步只发生在撤销要放回的节点上（`page/domain/revert_schedule.go:439-521` 的三条后备都只取待放回的，`:460` 的 `s.places[h]`）；一个此刻是来源的节点被临时改名或移走，它最后的放回本身就是来源的改名或移动，要管理员；来源换位置必经一个落在来源区里的节点或来源根的步，所以逐步判从不放过一个净效果要管理员或要改来源的撤销。`AGENTS` 从不被临时改名（它在 LLM Wiki 下不改名、不移动，没有变更集碰它的树）；临时名称带 ` ~<hex>`，从不等于保留的标题。
- **逐步判比净效果严的形状**（只多拒、不多放；写进第 10 节，P2 的测试各钉住结果）：每一轮先放回、再删除、再恢复、再写回（`revert_schedule.go:291-292`、`:311-339`），所以至少有三种：
  1. 撤销先把某个祖先放回来源区、再把它之下一个待放回的节点临时改名（那一刻它已在来源区里）：净效果是"移入来源区"（写者可以），逐步判要管理员。
  2. 移入来源区之后的写回：C（管理员）是 `[把 raw/F 移出到 W/F，编辑 W/F/P]`；撤销先把 F 放回 `raw`（按 P 编辑过的字节去重），再写回 P，这是来源上 `rewrite` 为假的 `OpContent`，`page.source_immutable`。谁也撤销不了 C，虽然净的状态正是 C 之前的。
  3. 放回与删除的次序里的去重：C（管理员）是 `[把 raw/S1 移出，以 S1 的字节写 raw/S2]`；撤销在同一轮里先把 S1 放回来源区、后删除 S2，`page.source_duplicate`。

  撤销的对话框与任务的报告在这些码上给出可执行的分步说明：需要还原正文时，先在来源区外还原，再移回；重复时先处理冲突来源，再恢复或移回。重新移回仍须去重，不能保证一定成功；这是分开的操作，不暗示合在一个 batch 也一定允许。管理员没有修改已有来源正文的例外。
- **先恢复、再写回**：同一单元里恢复又写回的来源页，恢复的一步是新增（照回收站里的正文的哈希去重），写回的一步照上面的写回判。
- **移动加改名的一步**（`Unit.Relocate`、撤销的放回）：守卫同时看这一步的父节点与名称，按移动判来源区，按改名判 `AGENTS` 与保留的标题，不从步的种类推只变了一样。
- **拒绝时**：同步的撤销与恢复整组拒绝（`page/app/revert_changeset.go:117-121`、`restore_node.go:117-124` 照答守卫的码）；后台的在开始时预检（4.6），之后在那一批以 `refused` 停下；网页的撤销对话框与任务的报告照码说明。

**`batch` 的合成**（审查 A-I1）：一页的几个正文操作在它第一个正文操作的位置以最后的正文写一次（`page/app/apply_batch.go:602-628`、`:651-690`），守卫看到的是合成之后的一步。所以写一次与第一次正文这两条由计划按项判（4.6，计划是权威，`Merged` 是后备），结果与按次序分开调用相同；只有一种要管理员的形状被后备多拒（移出、清空、移回、再写，第 10 节）。去重不在计划里：单元里一页移进来源区时比的是它在这个 `batch` 里最后的正文，不是那一项时的正文。这只在管理员在同一个 `batch` 里又把它移出、再写它时不同于分开调用（写者移不出来，来源区里也谁都写不了）：`[编辑 P 为 "X"（在来源区外），把 P 移进 raw，移出，编辑 P 为 "Y"]` 在 "Y" 与一个活着的来源相同时在第二项被拒，分开调用成功；反过来 "X" 重复、"Y" 不重复时 `batch` 放过、分开调用在第二项被拒，而结束时来源区里没有重复。两种都写进第 10 节与 MCP 的 `batch` 的工具说明。

**来源页的编辑会话**：否决者在开启时拒绝（`page.source_immutable`），所以网页从不对来源开编辑器。一页被编辑时被移进来源区，之后的每次保存被守卫拒绝（编辑会话的保存在来源上一律拒绝）；网页把这个码当作一种新的失去编辑的原因：编辑器只读、给出它的横幅，并结束服务端的会话（4.16）。

### 4.8 去重

- **范围**：一个新增的来源与这本笔记本里**活着的**来源（不含回收站、不含来源根）比较：页面按正文的 SHA-256（即逐字节相同：同一个 BOM、换行、frontmatter），附件按测定的 SHA-256（`asset_blobs.sha256`）；页面与附件不互相比较（第 11 节设计的取舍第 10 条）。空的页（0 字节）不比较、不算重复。同一个单元里先后加的两个也算（`batch`、导入的一批、移入的一棵子树里两个相同的）。
- **比的是现在的字节**（第 11 节主持者的决定）：`source_hash` 跟着来源的每一次正文的写，系统的链接改写与撤销写回改写都在内，所以来源被改写过之后，它的哈希是改写之后的（负责人的决定说"与已有来源的字节完全相同"）。链接改写可能让两个活着的来源逐字节相同：这是允许的（谁也没有新增）。之后一个同时含有这两个的回收站的组恢复不了：后一个与同一步里前一个重复。这可以接受，写明。
- **拒绝**：409 `page.source_duplicate`，problem 的新成员 `duplicate_of` 是已有那一个的节点 id（6.1：答复只给 id；网页按树写出它的路径并给链接，树里还没有它时先重读，4.16）。只有该节点在本单元开始时已是活来源、kind 与被比较的当前 hash 相同，才带 `duplicate_of`；回滚会撤去来源身份或这份字节时省略，不仅限新 UUID：旧节点本单元移入、从 trash 恢复、空来源首次正文、既有来源 rewrite 成新 hash 都适用，改回起点同 key 可再指出。内存同一步内部命中与 SQL 命中共用 baseline 谓词；起点 raw/F/A(H) 随 F 移出、区外新增 B(H)、再移回的内部重复仍可指 A，起点 A 在区外则省 id。缺 id 文字只说“与本次写入里的另一来源相同”；同步整单元未写与后台本批未写/前批保留分别解释。MCP 的错误文字带它的路径（page 的 MCP 适配器写路径），`structuredContent.error` 另带 `duplicate_of`（`membersOf` 今天只抄 `fields`、`lock`、`locks`，`mcpserver/errors.go:184-210`）；后台任务的报告的 `refusal` 也带 `duplicate_of`（可为 `null`，与被拒节点 `node_id` 独立，二者均可为附件）。这个成员的管道在 P2/S2 列全：`common.yaml` 的成员、`httpserver.Problem`、`problemOf` 读的取值函数、`shared.Error` 的字段与构造函数、`TestPlatformProblemsMatchTheContract`。
- **导入进来源区**：不让整个任务停下，重复的在导入的单元里先判（4.6 的预检）：重复的附件跳过；重复的页在库里下面有页或附件时建成一页空的页（文件夹），它们照常落在它之下（审查 A-M3：今天唯一的按节点跳过是过深的页，`transfer/app/import_write.go:259-263`、`:289-292`，连它下面的一起丢，这里不能照用）；下面什么也没有时跳过，不留下一页空的、带着重复的名称的来源（审查 A2-N6）。两种都写进报告的问题 `duplicate_source`：`path` 是这一项在库里的路径，`to` 是已有的来源在笔记本里的路径；`counts.skipped` 数它们（空的文件夹页不算导入的页）。LW7 有一步是带着子页的重复。
- **哈希记在哪里**：`nodes.source_hash`（`source_hash` 的迁移）。部分索引 `(notebook_id, kind, source_hash, id) WHERE source_hash IS NOT NULL AND deleted_at IS NULL` 非唯一；按这本笔记本的 kind/hash 探查，id 末键支持完整计划的有界候选续读（下面的代价协议）。
  - **不变式**（按模式与生死写）：一个节点的 `source_hash` 不是 `NULL`，当且仅当它活着、在一本 `llm_wiki` 笔记本的来源区里、是正文大于 0 字节的页（值是正文现在的哈希）或附件（值是文件的 SHA-256）；回收站里的、来源区以外的、普通笔记本里的与空的页一律是 `NULL`。`checkPages` 加这一条（带 SQL 播下的反例，13.4 第 1 条）。
  - **由单元维护**：`Unit.apply` 在每一步写之后、记条目之前，对 `llm_wiki` 的单元按这一步之后的来源区设上或清掉这一步改动的节点的 `source_hash`：新建、对来源的每一次正文的写（第一次正文、链接改写、撤销写回改写）、移入与移出（子树的每个节点都在 `OpMove` 的改动里，`page/app/unit_move.go:73-88`）、恢复。移入、恢复进来源区时，子树的页的哈希一条语句读出（`page_contents`，恢复时不论 `deleted_at`，4.6），附件的 SHA-256 经 `WriterDeps` 的附件端口一次读出（4.1），都放在步的 `AreaHashes` 上交给守卫（4.6）。与搜索的入队一样是 page 自己的派生数据，不是扩展点。
  - **删除时清掉**：`DeleteNodes` 本就更新子树的每个节点（`page/adapter/postgres/queries/nodes.sql:191-197`），同一条语句把 `source_hash` 设为 `NULL`，不多一条语句（审查 A-M2）；笔记本（与工作区）的删除经 `DeleteNotebooksPages` 软删除它的节点（`nodes.sql:270-282`，`gone` 的 UPDATE 在 `:275-279`），不经 `DeleteNodes`，那一条也同样设为 `NULL`（审查 A2-M2：否则删掉一本 LLM Wiki 之后 `checkPages` 就失败）；恢复时单元再设。降级时 `Forget` 一条语句清掉这本笔记本的（4.5，走部分索引）。所以降级之后在普通笔记本里恢复一个当年的来源，得到的是 `NULL`，不变式成立。
  - 同一单元里后面的步的守卫看得到前面的步设下的值，所以单元里的重复也找得到。
  - **性质测试**："随机的写之后等于从头重建"（13.4 第 10 条）。步的种类含上传进 `raw`、空的来源的第一次正文、来源的链接改写、导入进 `raw`、后台的恢复与撤销、降级之后的写与降级之后的恢复；随机的写在来源上多半被拒，所以每一种照 `TestTheIndexIsItsRebuild` 定一个最少的次数（`bootstrap/links_rebuild_test.go:45-76`）。
- **为什么不给 `page_contents`、`asset_blobs` 建哈希索引**（第 11 节设计的取舍第 10 条，与"每次比较整个笔记本的页"一起比较）：

  | 做法 | 代价 |
  |---|---|
  | `page_contents (content_hash)` 的部分索引 | `page_contents` 没有 `notebook_id`（`00013_page_page_contents.sql:5-21`）：索引是整个实例的，同一份字节在别的笔记本里有多少就扫多少（空的、模板一样的小页成千上万），一次写的工作随看不到的内容变（13.1 第 4 条的规则）；内容的每次写（自动保存约 2 秒一次）都改这个索引的键，失去 HOT 的更新；在 600 万行的表上建索引挡住写 |
  | 给 `page_contents` 加 `notebook_id` 再建索引 | 回填要改写整张表（600 万行估计约一分钟、上 GB 的 WAL），每次写正文照样多改一个索引 |
  | 每次新增读出整个来源区再比 | 一步 O(来源数)：10 万个来源时约 0.3 秒一步，`batch` 加 100 个来源过了 15 秒的期限 |
  | `nodes.source_hash`（选） | 加列只改元数据（检查要扫一遍，4.2）；索引只含来源；只有 LLM Wiki 的新增读它，按笔记本分键；代价是单元里多一段维护与一条不变式 |

- **并发**：新增来源的写多数是改树的单元（新建、上传、移动、导入、恢复、撤销），在笔记本行的 `FOR NO KEY UPDATE` 下串行，后一个的守卫看得到前一个已提交的 `source_hash`：两次上传同一个文件进 `raw`，两者都过了 `Check`、流完，在笔记本行上排队，后一个答 `page.source_duplicate`，它的文件被删掉（`checkAssets` 守着）。只有"空的来源的第一次正文"是 `FOR SHARE` 的单元，两个可以并行：守卫在判这种新增之前取这本笔记本的事务级 advisory lock，**只有 `FOR SHARE` 的单元取它**（改树的单元已经排除了每个 `FOR SHARE` 的单元，不必取），两个这样的写一个接一个，后一个在前一个提交之后再查。键是两个 int 的键空间：第一个是常量 `0x73726365`（"srce"，不同于 linking 的索引锁的 `0x6c696e6b`，`linking/adapter/postgres/store.go:24`，也不同于 transfer 与搜索的单个的键），第二个由笔记本 id 派生，派生的函数导出给 `bootstrap` 的测试（同 `interleaveOnIndex`，`bootstrap/interleavings_links_test.go:21-31`）。锁的次序：工作区 → 笔记本（`FOR SHARE`）→ 正文行 → 来源区的 advisory lock → 观察者里的索引锁，当前生产 FOR SHARE 来源新增只写一页，取它的只有守卫里这一处；同步/后台 revert 即使 content-only 也用 Tree 单元，不取 source advisory。未来多页 FOR SHARE 调用须在 advisory 前锁齐全部将用的 content rows，或改用 Tree 单元；只按 ID 顺序取正文锁不足以排除 content→advisory→content 环。写进 13.1 第 5 条。
- **新增来源的工作界**：普通单次 Move/Relocate、batch 仅一次 move 与 undo place 目前可携带超过 10k 后代，不把它们误称已有统一上限。P2 仅对一次把既存子树从来源区外移入来源区新增 `MaxUnitNodes=10,000` 界（空页/附件计数）：REST move、MCP move、batch 单 move、同步/后台 undo place 一致，超出答 422 `page.source_too_large`。source-entry 使用有限 `SubtreeWithin`，父数组 LATERAL、内部按 name_key 的既有 partial parent 索引序、内外同take且均为`LIMIT (SELECT sqlc.arg(max_rows)::integer)`，notebook过滤在每父内层LIMIT之后，最多返回/物化 10,001 节点就停止，不能先完整 Subtree 再 len；确认完整后 Go 按 Level/sort_order/UUID 整层排序，保持旧 Subtree 顺序，overflow 不排序/读后层或 hash/asset。不新增 reader 索引。返回界不等于物理 IO 界，无/有统计 × custom/generic 实际计划、范围/缓冲及巨大宽层反例另验。普通单 move 10,001 成功、区内/移出、搬来源根而成员不变不受新界；既有多 move 累计界保留。已知参数/parent/title/cycle/protected/virtual_file 在前；overflow 先于未读子树的深度错误，未 overflow 按原深度规则。完整历史计划/跨批恢复仍可大于 10k，单个 source-entry place 受新界；批间长大越界则 refused，前批保留。source-out 从已有 Changes 分块清 hash，不再读正文/资产。
- **代价与全计划排除**：一次普通新增通常一个 key 的索引探查；元数据/批写数组每次≤10k，经 `postgres.Planned`；Probe实际用SELECT-list并列SRF和generate_subscripts配对五组数组，has_afters区分缺游标与zero UUID；固定notebook后以(kind,hash,id)下界和(kind,hash)上界约束同key游标范围，SetSourceHashes用单条UPDATE、nodes old主键self-join与RETURNING交回实际改变目标的旧kind/hash，n的notebook/live/distinct条件不变；无统计末尾CTE join二次方反例与修复见P2 §10.2；附件 SHA256 经已有端口批量读取。历史模拟受影响全集 X 留在 Go，不分块 NOT IN。外部候选用 `(kind,hash,id)` 游标，初始每 key 取1，跳过 X 后按该 key 初始受影响数 E 的余量大页续读，各次 sum(Take)≤10k，直至首外部候选或组末；每 key 至多 E+1 行，总≤K+|X|，不按每个排除节点一条 SQL、不读全部来源。锁外 heads/E/分页与权限SQL共用已有只读 repeatable-read snapshot；Check/CheckJob的ask为func(context.Context, shared.Action) error，调用者用实际snapshot回调ctx，避免MaxConns=1时重新等外层连接。guard 复用写事务而不嵌套；树锁及当前单页 SHARE 的 content/source advisory 分别保证锁下 E 稳定，最终仍由 guard 裁决。完整签名、SQL、满页/空组停止条件见 P2 §3.7、§3.11。
- **测量**：P2/S2 在 10 万来源上分别量单次新增、10k 移入、MaxUnitNodes/MaxUnitPages 恢复与撤销、60k 任务计划和 Forget，各一成本行；覆盖大量 rewrite 收敛同 hash、X 跨10k且外部候选在后、成功末层的空父探查，不以早拒/空结果冒充成功代价。四列索引额外空间/写入与 600 万两类实际冷迁移按4.2验收，阈值按13.4实测，不预填结果。 已接受三状态HTTP45、六成本行双轮、正式A/B及独立扫描/DO/第三份三列原数见[P2 §10.2](02-P2-source-area.md#102-s2-来源-hash去重与验证)。完整本机门禁已另由主持者接受；本机实数不替代CI或生产停机承诺，精确33b4代码CI11项成功；文档B-N1已由原B复核关闭，最新文档/分支配对与main合并CI已验，记录CI与双main样本见[S2合并记录](reviews/P2-S2-source-hash-review.md#合并与后续验收)；上述测量仍保留原阶段范围。

### 4.9 `AGENTS`、来源根与保留的标题

只在 `llm_wiki` 下：

| 对象 | 规则 | 码 |
|---|---|---|
| `AGENTS` 页（根下、标题键是 `TitleKey("AGENTS")`、类型是页，同说明的找法 `nodes.sql:84-95`） | 不能删除（含删除它的祖先：它在根下，没有祖先）、改名、移动（含排序）；正文与 frontmatter 照常改（写者） | 409 `page.protected` |
| 来源根（`source_root_id`） | 不能删除（含删除它的祖先）；改名、移动要管理员（4.7）；正文写者可以改 | 409 `page.protected`、403 `page.source_admin_only` |
| 根下的标题键 `index`、`log`、`index.md`、`log.md` | 任何节点（页与附件）不能以它们为名放在根下：新建、上传、改名、移动（拒绝）；恢复、导入（加序号，下面）；`list`、`read` 的路径 `index.md`、`log.md` 是虚拟的（4.13） | 409 `page.virtual_file` |

- 保留的标题对附件也保留：一个根下叫 `index` 的附件与虚拟的 `index.md` 的路径不同，但同一个标题键会让链接 `[[index]]` 与目录的名字在人看来指向两样东西；一并保留最简单（第 11 节设计的取舍第 13 条）。`index.md`、`log.md` 也保留（审查 B-M11）：页可以叫 `index.md`（只有附件不许以 `.md` 结尾，`page/domain/name.go:15-29`），agent 的 `write index.md/x.md` 会建出这样一页作父页，于是 MCP 的路径里虚拟的 `index.md`、真实的 `index.md.md` 与文件夹 `index.md/` 同时在，整本导出时贡献者的 `index.md` 碰上这个文件夹答 `contributor_conflict`（`transfer/domain/plan.go:290-299`）。保留之后这种页在 LLM Wiki 的根下建不出来。
- **导入与恢复加序号**：导入（`page/app/unit_import.go:172-194` 的 `free`）、同步的恢复（`unit_restore.go:177-210` 的 `restoredNames`）与后台的恢复（落到根下的一组经 `placeRoot` 与 `partPlaces.under(-1, siblings)`，`unit_restore.go:297-306`、`:388-395`）按活着的兄弟给重名的加序号；LLM Wiki 的根下把这四个键当作已被占（加进根的 `keys`），所以一个库里根下的 `index.md` 导入成 `index 2`、报告 `renamed`，不让守卫拒绝整个单元。导入的 `Reserved` 钩子（`transfer/app/import_ports.go:124-127`）只决定序号跳过哪些候选，不让原来的名称加序号，所以不用它。
- **重新导入一个 LLM Wiki 的导出**：导进一本新的 LLM Wiki 的根下，`AGENTS.md` 因为已有 `AGENTS` 成为 `AGENTS 2`，档案的 `raw/` 因为已有 `raw` 成为 `raw 2`，它里面的来源落在来源区以外。这是 v0.1 写明的一处限制（README 的 LLM Wiki 一节写明；把档案的 `raw/` 并进来源根、合并 `AGENTS.md` 不做，第 2 节），已有的笔记库走 M11 的升级（"写给后面的 M"）。
- **虚拟页的链接**：一条链接指虚拟页，当且仅当它解析不到（有别的同名的页时 `[[index]]` 解析到那一页，4.4 的单段的目标照标题找），而它的目标照 lint 的待建页面的做法改成从根起（`./`、`../` 按链接它的页的文件夹，开头的 `/` 去掉）之后只有一段，去掉 `.md` 之后的标题键是四个保留的键之一（`index`、`log`，与 `index.md`、`log.md`：后两个是写作 `[[index.md.md]]` 的根下的页，守卫照样不许建，网页把它们当作 `index`、`log`，审查 B2-N6）。所以 `[[index]]`、`[[/index]]`、`[[index.md]]` 与根下的页里的 `[[./index]]` 是，`[[sub/index]]` 与 `sub/p` 里的 `[[./index]]` 不是。Go（linking 的 `domain`，lint 与落点用）与 TypeScript（`web/…/pages/page/unresolved-link.tsx`，读者的点击从不问服务端，`:115-118`）各实现一份，共用一张样例表（同 `tools/md-fixtures` 的做法）。落点（`linking/domain/landing.go:57-60`）也用它：根下的页里的 `[[index]]`（与 `[[index.md.md]]`）不再提议建一页守卫会拒的根下的页，待建页面也不数它们。
- `page.virtual_file` 的文字（中英）照总体设计 5.5："这个文件由系统生成，修改页面后目录会自动更新"（`log` 的写"日志就是变更集，会自动更新"）。
- 网页：`AGENTS` 的行没有改名、移动、删除；来源根的行没有删除，改名、移动只对管理员；虚拟页的链接（上面的定义）在 LLM Wiki 里打开目录视图与历史（4.16），不给"新建这一页"，读者也一样。

### 4.10 必填属性

- **只检查**（负责人 2026-10-24，第 11 节负责人的决定第 2 条）：来源区以外的页缺 `type` 或 `summary` 时，健康度面板显示为"需要注意"，MCP 的 `lint` 报出来；任何写入都不因此被拒：网页的新建页面、agent 补建的父页、导入、还原与恢复都照今天。总体设计 5.1"写入时的不变量……违反就拒绝"只说来源区、`AGENTS` 与保留的标题，5.2 的"必填"改为"lint 检查"（"总体设计的修订"）。M11 的提交闸门以它为闸（"写给后面的 M"）。
- **"缺"的定义**：frontmatter 能解析（`indexed_pages.frontmatter_valid`），照 M9 取 `type`、`summary` 的规则找键（第一个以 ASCII 不分大小写等于它的键，`linking/adapter/postgres/queries/index.sql:12-25`），然后：没有这个键，或值不是字符串，或字符串去掉首尾空白之后为空。空白的集合是 `jsTrim` 的（`linking/adapter/markdown/facts.go:184-191`：Unicode 的空白与 U+FEFF，不含 U+0085），SQL 里写同一个集合，测试拿 SQL 的结果对着 `jsTrim`（同 4.2 读 `aliases` 的空白）。frontmatter 写坏了的页只算进"frontmatter 解析失败"，不在这两条里重复。`index.md` 与目录视图的"有类型"照这个"缺"的反面：`ListIndex` 今天把 `type: ""` 当作有类型（`linking/app/list_index.go:118-121`），一页 lint 说它缺类型，却被归在一个空白的类型下；M10 照 4.10 改（4.13）。
- **豁免**：来源、来源根、`AGENTS`、虚拟的两页、附件。来源的 frontmatter 一概不查（第 11 节主持者的决定：lint 不看来源里写的内容）。
- **存储**：用 `page_properties` 与 00047 的 `value_start`、`value_chars` 与两个部分索引（`00047_linking_page_properties_keys.sql:27-40`），不提升 `domain.Extractor`、不要 `nervewiki reindex`（提升让每本笔记本在重建期间不能写，30 万页估计约 330 秒，`linking/app/reindex.go:75-131`）。只有空白的判断：`value_start` 是前 257 个字符，值更长而前 257 个都是空白时读整个的值。这样的行少，但数目要看每一行（含结果并不缺的），所以它们的读以这本笔记本的内容为界，不是以条目数为界；成本守卫里有一组这样的行。

### 4.11 lint

**lint 不看来源里写的内容**（第 11 节主持者的决定）：来源只写一次，它们里面的断链、歧义与坏的 frontmatter 谁也修不了（粘贴的文章与导入的文档常带相对链接与 `---` 的块），报出来只是永远的噪声，lint 的 playbook 的"能机械修的就修"还会去修、被拒。所以在 LLM Wiki 里：来源发出的链接不产生待建页面与歧义，也不让 wiki 的页不算孤儿；来源的 frontmatter 不查；来源只由"未消化"一条判定。普通笔记本里没有来源区，每页照常。

**规则**（总体设计 5.6；"适用"一列：通用的两种模式都有；"链接它的页"在 LLM Wiki 里都不含来源）：

| 规则（码） | 适用 | 定义 | 条目 |
|---|---|---|---|
| 待建页面（`wanted`） | 通用 | 解析不到的链接，按**写的目标**分组，数链接它的不同的页。分组：目标的每一段取标题键、照 4.4 去掉 `.md` 的写法；`./`、`../` 的按链接它的页的文件夹改成从根起，开头的 `/` 去掉（所以 `[[/a/x]]` 与 `[[a/x]]` 是一组：在根下建 `a/x` 两者都解析到它）。不算的：`target_key` 为 `NULL` 的（`[[a//b]]`、最后一段的键过了 `MaxKey`，无论树怎样都解析不到，`00021_linking_page_links.sql:18-19`、`linking/domain/facts.go:56-72`）；`../` 越过根的（改不成从根起）；**读作附件的**：照代码的定义，`Target.ReadsAsAsset`（`linking/domain/target.go:123`）对着候选判，候选是 page 的 `ByKeys` 按这些目标的最后一段的键读出的节点（一条语句，只对带扩展名的解析不到的目标的不同的最后一段），即这本笔记本里某处有这个名字的附件；**解析不到的图片与嵌入**（`kind` 是 `image` 或 `embed`）而最后一段照 4.4 去掉 `.md` 之后（`Target.Name`）仍带扩展名的：它们是缺的文件，不是待建的页（嵌入一页的 `![[note.md]]` 去掉 `.md` 之后没有扩展名，照常是待建的页，审查 B2-N3）；虚拟页的链接（4.9 的定义）。带点的页名照常算：`[[Node.js]]`、`[[v1.2 release]]`、`[[Python 3.12]]` 是待建的页（"带扩展名"是 `hasExtension`，`linking/domain/resolve.go:41-44`，不是读作附件）。次序就是游标的那一组：页数降序、组的键的前 256 字节、组的键的 SHA-256（前 256 字节相同的两组按哈希排，审查 B2-N1） | 目标（写下的第一种写法：按 `(source_id, range_start)` 最小的那一条，截到 256 字节并标出 `cut`）、页数 |
| 孤儿页（`orphans`） | 通用 | 没有别的页的链接解析到它的页（自己的不算，`resolved_asset` 的不算，LLM Wiki 里来源发出的不算）；豁免来源、来源根、`AGENTS`（两种模式都豁免：普通笔记本里根下的 `AGENTS` 也是 MCP 的说明，5.2）、虚拟页面、附件，以及文件夹页（有子节点而正文为空的页，第 11 节主持者的决定：agent 补建的父页总落在这里） | 页 |
| 链接有歧义（`ambiguous`） | 通用 | `page_links.ambiguous`：几页在每一项偏好上都相同、按 id 选了一页（`linking/domain/resolve.go:335`，同 4.4）；5.6 的说法"匹配到多个节点"改正 | 链接的页、目标（截到 256 字节） |
| frontmatter 解析失败（`invalid_frontmatter`） | 通用 | `indexed_pages.frontmatter_valid = false`；LLM Wiki 里不含来源 | 页 |
| 缺类型（`missing_type`）、缺摘要（`missing_summary`） | LLM Wiki | 4.10 | 页 |
| 未消化的来源（`undigested_sources`） | LLM Wiki | 来源区里的页与附件，没有一页**来源区以外的 wiki 页**以正文链接（`property_key IS NULL`）或 `sources` 属性链接（属性路径的第一段以 ASCII 不分大小写等于 `sources`，取这一页第一个这样的键，同 M9 的第一个键的规则）解析到它。来源根与 `AGENTS` 的链接不算（来源根的正文描述来源区，`AGENTS` 是给 agent 的约定，都不是 wiki 的内容；`raw` 的说明里一个目录不能让每个来源都算消化了），虚拟页从不算（第 11 节主持者的决定；总体设计 5.4 照此修订）。来源区里的附件，有一页来源页链接或嵌入它时，跟着那一页：那一页消化了，它就消化了；那一页待消化时它不另列，`undigested_sources` 与 `pending_sources` 只列那一页（一篇文章的图片不独自留在待消化里，ingest 的 playbook 也不会把它们当作另外的来源，审查 B2-N5）。**0 字节的来源页不算**（审查 B-I2：MCP 与 `batch` 补建的父页、来源区里的"新建页面"都是空的文件夹页，谁也不链接它，会永远待消化，ingest 的 playbook 先取 `pending_sources`，会一直拿到它们）；同 4.8 的空的页不去重。一页空的来源之后有了第一次正文，从那时起算。按新建的时刻 | 来源 |
| `sources` 指向无效（`invalid_sources`） | LLM Wiki | 来源区以外的页：`sources` 属性链接解析不到，或解析到来源区以外（含来源根）；或第一个折成 `sources` 的键（由 4.2 的 `page_properties_sources_idx` 每页一次探到）的值里有一项不是属性链接：数组或对象（不加引号的 `sources: [[raw/x]]` 是 YAML 的嵌套列表），或一个没有属性链接对上的字符串（`sources: [raw/x.md]`、`sources: raw/x`、`sources: ["raw/x"]`）；它们既不消化也不出现在别的规则里。`page_properties` 存着每个属性的整个值（`linking/adapter/markdown/facts.go:76-81`，`00023_linking_page_properties.sql:11`），所以读这些行、把字符串的叶子与这个键下的属性链接对一对就认得出，不提升提取器（审查 B2-M3） | 页、目标（截到 256 字节；不是链接的一项写出它存着的 JSON 的文字，截到 `MaxIndexString` 并标出 `target_cut`，再标出"不是链接"；一页里同样的文字只算一项，同歧义按页与目标算） |

- 属性路径不记深度（`platform/markdown/frontmatter.go:27-29`）：叫 `sources.0` 的顶层键与 `sources` 的第 0 项在索引里相同；块标量与别名的值不是属性链接（`tools/md-fixtures/README.md:107-109`）。都照今天的索引读，要改得提升提取器，M10 不改；`AGENTS` 模板写明带引号的写法（4.4）。
- **算法**：读的时候算，没有写的时候维护的计数（负责人 2026-10-24）。一次运行一个 `REPEATABLE READ READ ONLY` 的快照（`postgres.WithinSnapshot`，同导出）：各条规则、来源区的节点、路径看到的是同一个时刻；快照占一个连接到它结束。REST 与 MCP 的范围在快照的第一条语句里读这本笔记本的模式与来源根（不是在快照之前由请求定下），所以答出的 `mode` 与规则用的契约都是快照的；M11 的显式的范围照它传的。一次运行的语句依次是：模式与来源根、来源区的读、各规则、答出的附件的信息、答出的项的路径（一次 `NodePaths.Paths`）（审查 B3-M2）。来源区的节点由 page 的端口读一次（`page.NewSourceArea(pool)` 的 `Nodes`：来源根的子树的每个节点的 id、种类、建的时刻与页的正文的字节数，只读 page 的表；附件的字节数在答出时经附件端口读（`asset_blobs`），4.17 的交错靠这个次序；`ListNodes` 量过约 51–100 ms 每 10 万个节点），以数组交给 linking 的语句。
- **待建页面是整个的聚合**：它按页数排序，所以摘要的前 5 项与每一页条目都要这本笔记本的解析不到的链接的整个聚合，数到 10,000 就停也省不下什么；"更多"再聚合一次。为了少做：SQL 里先按写下的 `target` 分组、数不同的页（相对的目标按 `(target, source_id)` 分组：linking 的 SQL 不连 `nodes`，没有文件夹），Go 里把每个来源页对到它的文件夹、再按算出的组的键合并（审查 B2-N2）（`target_key` 只是最后一段的键，`00021_linking_page_links.sql:18-21`，组要每一段的键）。一次运行的内存因此是 O(这本笔记本的解析不到的目标的字节数)，成本守卫带一个内存的界（同 `ListIndex` 的 250 MB，`linking/adapter/postgres/index_cost_test.go:31-51`）。
- **过时的行**：快照里正文与索引在同一个事务里写（观察者），版本不会对不上；对不上的只有提取器不是当前的（升级改了规则、还没 `nervewiki reindex`），linking 自己就判得出。这样的页跳过、计数，答复带 `stale_pages` 与一句"请服务器的管理员运行 `nervewiki reindex`"；不即时解析（30 万页约 2 分钟的 CPU，M6 移交注意第 1 条许可"跳过"）。v0.1 带着 M6 发布，`checkLinks` 守着每页活着的页都有索引行（`bootstrap/links_test.go:24-31`），所以没有"没有索引行的页"要找（那要每次读整棵树）。
- **数目**：每条规则的数目数到 `MaxLintCount` = 10,000 就停、答"10,000+"（`count_capped`）：停下的数是**下界**。不到上限的数在快照里是确切的：SQL 数出的规则都在 SQL 里过滤（过时的页、来源、0 字节的来源都在语句里排除），待建页面的数在 Go 的合并之后数。
- **条目与游标**：条目照规则的次序游标分页（`EncodeCursor`），`limit` 1–100、缺省 50。游标的载荷带规则的名称（`DecodeCursor` 只拒载荷的形状不同，`shared/cursor.go:41-62`，而孤儿与 `invalid_frontmatter` 的形状都是 `{id}`），另一条规则的游标答 400。待建页面的游标是（页数、组的键截到 256 字节、组的键的 SHA-256），有界；别的规则是排序键与 id。每一项带它自己的游标（同 `IndexItem.Cursor`），MCP 的 `Fit` 可以停在一页的中间。写者控制的字符串（目标）都截到 `MaxIndexString`（256 字节，`linking/domain/index.go:20`）并标出 `cut`，同 `index`：一个 100 KB 的目标不会让摘要放不下一项。路径只为答出的一页读，一次 `NodePaths.Paths`，在附件的信息之后。每一页条目是它自己的一次运行、自己的快照：两页之间条目可能移动，答复与网页写明（13.2 第 19 条的按键去重：页的 id；待建页面是组的键；歧义与 `sources` 是页的 id 加目标）。
- **摘要的 `Fit`**：每条规则的数目、`count_capped`、`stale_pages` 与算的时刻总在；每条规则的前几项一起减，从 5 项减到 0 项，直到放得进 48 KiB。
- **同时运行的界**（审查 B-M5、C-M10）：一次运行占一个连接、一个快照与 O(笔记本) 的内存，秒级；读者经面板、"重新检查"、"更多"，agent 经 `lint`、`pending_sources`（ingest 的 playbook 每次 ingest 之后都调）都能开始一次。所以：进程内的名额 `MaxLintRuns`，同 `GrepParses`（`page/app/grep.go:47-60`）。**名额至少 1，取自请求的那一半**：今天的配置的核对已经把快照的那一半用满（`2 × (export_workers + import_workers + SearchJobConns) ≤ max_conns`，`platform/config/validate.go:251-271`，最小的合法配置 6 正好填满），所以 lint 的名额从请求的那一半里取（`max(1, …)`，P3 定式子，写进 README 的配置说明），最小的配置也有 1 个，13.1 第 23 条照此修订。**lint 自己的期限** `LintTimeout`（低于请求的期限，P3 定数）：一个从合成的那一次开始算的期限，盖住等名额与运行两段，到了就停、答 503 `server_busy`，不会碰上请求的期限而答 500 `internal_error`（`platform/httpserver/apierrors.go:272`）。同时开始的相同的请求（同一个范围：笔记本、`Contract` 与 `AreaRootID`；摘要，或同一条规则、游标与 `limit` 的一页）**在名额之外**合成一次（singleflight，结果对这本笔记本的每个读者都一样），仍是"打开时算"：合成的那一次在一个不随第一个请求取消的 ctx 上（`context.WithoutCancel`，以 `LintTimeout` 为界）先等名额、再运行，所以合并的请求只占一个名额，最小的配置下两个请求也合得上；跟随的请求在它自己的 ctx 到期时停，第一个请求走了或到期不连累别的（审查 B2-M4、B3-N1）。键里带着范围，所以 M11 的闸门与同一本笔记本的 REST 的 lint 不会合在一起、拿到对方的规则（审查 B3-N2）。四个 REST 操作与两个 MCP 工具声明 `server_busy`。M12 的性能移交加一条。
- **成本**（调查估计，P3 量）：30 万页时每条规则 0.03–3 秒，加上来源区的读。来源区以数组进 linking 的语句：孤儿、缺类型与缺摘要、未消化、`sources` 无效。这些语句经 `postgres.Planned` 以参数规划（缓存的通用计划对这样的数组逐行比较，M6 收尾 FA4-M1：5 万个父节点 3.5 秒；13.4 第 9 条），清单：各规则的计数与条目的语句、读作附件的候选、待建页面的分组。P3 先量，**只给量过、超出预算的规则加部分索引**（4.2 的 linking 一行，含它在 M6 的 5,000 条链接的保存上的代价）。预算：10 万页时一次摘要至多 2 秒，以 13.4 的规则判（本机量的最快的几次的三倍，CI 再留余量），2 秒是 P3 量之前的目标。成本守卫在 10 万页上，各在 Makefile 的一行（`!race`，照 M9/P2 6.5 定行；今天的计划测试不带标记、在竞态的一份里跑，这几行是新的写法，写明）；计划测试不要 30 万页：在小的、分析过的种子上照 M9 的三个阶段（没有统计、分析过、通用的计划，`index_cost_test.go:189-227`）跑，种子里另有一本更大的笔记本（一个整个实例扫 `page_links` 或 `page_properties` 的计划违反 13.1 第 4 条，而在只有一本笔记本的种子上照样过）。一次运行的读至多是这本笔记本的索引，与读者看不到的内容无关（13.1 第 4 条）。
- **语义检查的范围**（5.6）：lint 的 playbook 让 agent 以 `log` 取最近 N 个变更集碰过的页，系统不另设规则。
- **阻断与提示**（5.1）：M10 里每条都是提示（"需要注意"），没有阻断：必填属性只检查（负责人 2026-10-24）；M11 的闸门在它自己的提交里以其中几条为闸。

**接口**：

- **wiki 的 `Lint`**：内部的入口收显式的范围 `LintScope{NotebookID, Contract, AreaRootID}`：REST 与 MCP 传"取这本笔记本的"（`Contract` 与 `AreaRootID` 为空），运行在快照的第一条语句里读出它们；M11 的闸门在升级期间（笔记本仍是 `standard`）传 LLM Wiki 的契约与它提议的来源根（审查 B-M16；"写给后面的 M"）。
- **REST（wiki）**：`GET /api/v0/notebooks/{notebook_id}/lint`（`getLint`：适用的每条规则一个对象 `{rule, count, count_capped, items, next_cursor}`，`items` 是前 5 项，`next_cursor` 接着它们；另有 `stale_pages`、算的时刻 `computed_at` 与算时的模式 `mode`，4.12）与 `GET /api/v0/notebooks/{notebook_id}/lint/{rule}`（`listLintItems`：一条规则的 `{data, next_cursor}`）；操作 `lint.read`（读者以上，规则表一行）。条目照 6.1 给 id：`wanted` 是 `{target, target_cut, pages}`；孤儿、`invalid_frontmatter`、缺类型与缺摘要是 `{page_id}`；歧义与 `sources` 无效是 `{page_id, target, target_cut}`，`sources` 无效另带 `not_link`（不是链接的一项，`target` 是它的值的文字）；未消化的来源是 `{node_id, kind, bytes, created_at}`。不认识的规则答 404 `wiki.lint_rule_not_found`；这个模式没有的规则（普通笔记本问 LLM Wiki 的）答 409 `notebook.not_llm_wiki`（第 11 节主持者的决定：一个码）。次序：`getLint` 是 404 → 503；`listLintItems` 是游标 400 → 404 笔记本 → 404 规则 → 409 模式 → 422 `limit`（13.1 第 27 条）→ 503。模式另在运行之内再核对一次：运行在等到名额之后、在快照的第一条语句里读模式与来源根，其间（至多一个 `LintTimeout`）提交的降级让快照的模式没有这条规则，这时运行答同一个 409 `notebook.not_llm_wiki`，不拿空的来源根去跑规则；这第二次核对出自运行之内、在等到名额之后（等不到名额时答的是 503；名额被占着时降级、排着的那一次答 409，P3/S1 有测试；审查 B4-N3）。权限矩阵的 `targetViolation` 认 `{rule}` 一段，照 `{tag}` 的写法加一种情形，不进 `notTargets`（那样也会跳过 `{notebook_id}` 的检查，`bootstrap/permission_matrix_coverage_test.go:125-150`）。
- **MCP（wiki，两个入口）**：`lint`（`rule?`、`limit?`、`cursor?`：没有 `rule` 是摘要，带上是那条规则的条目；条目带路径；`rule` 的码同 REST）；`pending_sources`（`limit?`、`cursor?`：未消化的来源，每项路径、类型、字节数、新建的时刻；就是 `undigested_sources` 那一条，给 ingest 的 playbook 一个直接的名字），普通笔记本答 409 `notebook.not_llm_wiki`。次序同 REST（笔记本由入口定），运行之内的第二次模式的核对也一样（`lint {rule}` 与 `pending_sources`）。两者的答复经 `Fit` 至多 48 KiB。

### 4.12 健康度面板

- **谁看得到**：能读这本笔记本的人（负责人 2026-10-24，第 11 节负责人的决定第 4 条）。两种模式都有：通用规则两种模式都算，LLM Wiki 加它自己的。
- **在哪里**：左栏 `NotebookPlaces`（`web/…/pages/notebook/notebook-places.tsx:15-29`）在历史之前加"健康"，路由 `/:slug/notebooks/:id/health`（在 `NotebookLayout` 之下，经 `useNotebook()` 拿到笔记本，不另读，13.2 第 16 条）；不是设置的一节（读者也要看），也不挤进页面的右栏（它是整本的）。页调 `useDocumentTitle`（13.2 第 22 条）。
- **算的时机**：打开时算一次（`getLint`），页上写"算于 HH:MM"（`formatDateTime`，13.2 第 14 条）与"重新检查"；没有定时刷新、不随事件重算（一次运行在大笔记本上要秒级，负责人：打开时算、不维护计数）。所以它不是由事件驱动的视图：不进 `eventHandlers` 与 `refreshedOnConnect`（13.2 第 23 条），SWR 照搜索页关掉 `revalidateIfStale`、`revalidateOnFocus`、`revalidateOnReconnect`（`web/…/pages/search/search-page.tsx:36`；应用只设了 `provider` 与 `onErrorRetry`，`web/…/app/providers.tsx:17-20`，不关就会在回到标签页、重连时重算）；键里带一次打开的标记（`location.key`，同搜索页），所以每次打开是一次新的运行；并照搜索页的缓存规则只留显示着的那一个：换键时以 `mutate(旧的键, undefined)` 去掉别的（13.2 第 7 条；不这样时退回到一次还在读的打开会一直停在"加载中"，M8/P5B A-I1，每次打开也会留下一份摘要）；"重新检查"调 `mutate`。模式的改变不靠事件：`getLint` 答出它算时的 `mode`，面板拿它比 `useNotebook().mode`，不同时写"模式已改变，请重新检查"、不自己重算（审查 B-N6：一次降级让每个打开的面板同时重算，正是名额要防的；C2-M1：比模式，所以断线时错过的降级在重连重读笔记本的列表之后也标得出，网页的测试钉住这一种）。
- **加载与重试**（13.2 第 7 条）：面板与每张卡的"更多"各是一块 SWR 的读，读不到时显示 `NotLoaded` 与"重试"，各有两个测试（读不到时说明原因、重试之后加载；再读一次时显示别处的变化，在这里是"重新检查"或新的一次打开）。lint 的 503 `server_busy` 照 `shared.ServerBusy(d)` 带 `Retry-After`（同别的每个 `server_busy`，`shared/error.go:188-191`，各模块的 `Problem` 也这样声明，如 `api/modules/linking.yaml:235-244`）。`retry.ts` 对 503 会自动重试五次（`web/…/app/retry.ts:18-30`），每次都要一个名额，所以面板的 `useSWR`（摘要与每张卡的"更多"）给自己的 `onErrorRetry`（目录视图的 `listIndex` 不占 lint 的名额、不答 503，不要它，4.16；审查 C4-N2），遇到 `ApiError.code === "server_busy"` 就不再重试，面板照 `server_busy` 的文字给"重试"，由人决定；网页的测试：一个 503 `server_busy` 只发一次请求，然后是 `NotLoaded` 与"重试"（审查 C3-M1）。
- **数据**（13.2 第 15 条）：摘要按一次打开、条目按规则与游标，都只在 SWR 里，没有新的按笔记本的 store。
- **样子**：每条适用的规则一张卡：名称与一句说明、数目（到顶时"10,000+"，经 `Intl.NumberFormat`）、前 5 项（页的链接、目标的文字与页数；截掉的目标标出），"更多"从 `getLint` 给这条规则的游标接着读 `listLintItems`，经 `pages/load-more.tsx`，焦点移到新加的第一项（13.2 第 19、26 条；每页是它自己的快照，按 4.11 的键去重）；数目为 0 的规则收在"没有问题"的一行里。每条规则的列表有 `aria-label`（13.2 第 17 条）；"重新检查"运行时 `aria-busy`，结果在一个总在的 `<output>` 里读出（13.2 第 12 条）。过时的页多于 0 时顶上一条提示，说请服务器的管理员运行 `nervewiki reindex`（读者自己做不了）。`server_busy` 有它的文字与重试。

### 4.13 虚拟的 `index` 与 `log`

**认出它们的一处**：`domain.FindPath` 与 `WalkPath` 的查找（`Lookup`）由一个按契约的包装给出：LLM Wiki 下根下（祖先为空）查四个保留的标题键之一（`index`、`log`、`index.md`、`log.md`，4.9）时，不查库，答 `ErrVirtualFile{Name}`（`page.virtual_file`，带 4.9 的文字）。MCP 的每个按路径的工具经 `NodePaths.Find`/`Walk`，`batch` 的计划经它模拟的树上的同一个查找（`page/domain/batch_plan.go:361`、`:403`、`:427-431`、`:493`），所以写、`edit`、`move` 的来源与目的地的父节点一线、`delete`、`versions`、`backlinks`、`log {path}`、`index {parent}` 与 `batch` 的每一项都答这个码；`write` 新建 `index.md` 不再建一页真的（今天会，`page/app/write_path.go:105-170`），`edit`、`delete` 不再答 `page.not_found`。`read` 与 `list` 认出它、改为给出内容与虚拟的项；`read index`（附件的路径，键是 `["index"]`）也碰到这个包装，答 `page.virtual_file`，不给 `index.md` 的内容；`read index.md.md`（键是 `["index.md"]`）同样答 `page.virtual_file`：内容只给键是 `index`、`log` 的两个路径 `index.md`、`log.md`；`list {pattern: "index/"}`（把 `index` 当文件夹）同样答 `page.virtual_file`。`move` 的目的地的最后一段从不经过这个查找（`batch` 与 `relocate` 只走 `to.Keys[:len-1]`，`page/domain/batch_plan.go:431`、`page/app/relocate_path.go:128`）：移到根下的 `index.md` 由 4.9 的保留标题在守卫与 `CheckContract` 里以同一个码拒绝，`batch` 的计划里照样在预检里拒（审查 B-N4）。

**MCP**：

- `list`：根的子节点（`pattern` 为空或 `/`）在最前面列出 `index.md`、`log.md`，项带 `virtual: true`，没有版本令牌、字节数与最后改动。它们在子节点的次序里的位置不用 `sort_order` 的值：排序可以是负的（`Place` 把放在第一的节点放在 `siblings[0] - 1`，`page/domain/order.go:13-23`；`nodes_sort_order_check` 只排除无穷，`00012_page_nodes.sql:18-19`；-Inf 在 JSON 里写不出），`{O: -2}` 一类的哨兵会让排在 -3 的页永远列不出（审查 B2-I1、C2-I1）。所以子节点的游标 `ChildrenCursor{O: sort_order, I: id}`（`page/app/list_by_pattern.go:84-85`、`:141-152`）加一个它自己的成员 `V`（`json:"v,omitempty"`）：`V: 1` 是 `index.md` 之后，`V: 2` 是 `log.md` 之后，真实的子节点没有它；比较时先比 `V`：带 `V` 的游标排在每个真实的子节点之前，从 `V: 2` 接着走就从第一个真实的子节点开始。`omitempty` 让 M9 的游标照旧是原来的写法（13.1 第 27 条：一个位置一种写法）。为了同一条规则：带 `V` 的游标的 `O` 与 `I` 必须为零（`DecodeCursor` 收下任何能逐字节重新编码的载荷，`ChildrenCursor` 总写 `o` 与 `i`，所以 `{"v":1,"o":3.5,…}` 否则就是"`index.md` 之后"的第二种拼法），`V` 不是 1 或 2 答 400；在不是根的列表上（或普通笔记本里），一个 `V` 游标从第一个子节点开始（审查 C3-N1）。每项带它自己的游标，所以 `limit: 1` 的第一页是 `index.md`、它的 `next_cursor` 接着是 `log.md`、再是第一个真实的子节点，`Fit` 也可以停在它们之间。glob 照路径匹配它们（`*.md`、`**/*.md`、`index.md` 都配得上），在路径键的次序里按它们的键走到（`GlobCursor{K}` 接着走，`:202-204`、`:309`；根下没有真实的节点有这几个键），不是放在最前面，所以 `TestTheGlobResumesFromItsCursor` 的性质（游标总向前、接着走的调用合起来至多一次遍历）照旧。分页的测试在它们的两边各有一个页界：根下的子节点的排序有负的（如 -3、-2、-1、0），以 `limit` 1 与 2 跨过虚拟项翻页，一项不重、不漏（`TestListByPatternPagesOnFromItsCursor` 的性质，`page/domain/list.go:36-39`）。`open_notebook` 的根下的节点同样先列它们（照答复的余量截，`MoreRoot`）；`overview.pages` 与 `attachments` 不数它们。`list` 的工具说明的"`**/*.md` 是每一页"改为"每一页，另加虚拟的两页"。
- `read`：给生成的 Markdown，按 `read` 的行切（`offset`、`column`、`limit`，`page/domain/read.go:67` 对任何字符串都成立）；元数据带 `virtual: true`，没有版本令牌（元数据的那一行说明的十个成员改写）。每次调用重新生成：两次之间别人写了，行号会挪；`tools.json` 的说明里写明。
- `grep`、`search` 不搜它们（候选来自搜索的索引，它们不在树里）；`grep` 与 `index` 在 `api/mcp/tools.json` 的工具说明里写明"虚拟的两页不在其中，读 `index.md` 或用 `index`"（这些字在 `tools.json`，不在内置的说明里，4.15）。

**内容**（wiki 生成，`domain` 里的纯函数，有表格测试）：

- `index.md`：只列 wiki 的页（第 11 节主持者的决定）。`ListIndex` 加一个选项 `ExcludeArea`：带着来源根时去掉来源区里的页（来源区的 id 由 page 的 `SourceArea.Nodes` 每次读一次，约 51–100 ms 每 10 万个节点，在 Go 里按集合去掉；`types` 的页数也不数它们）；`index.md`、目录视图与 REST 的 `listIndex` 用它，MCP 的 `index` 工具照 M9 建成的列每一页（它是筛选的工具，审查 B2-M2）。来源区以外的页，按总体设计 5.5 分组：有类型的（照 4.10 的"缺"的反面）按类型分组（类型按标题键排，组里按路径），没有类型的按父页分组（父页按路径排）；每项一行：`- [标题](<路径>) — 摘要`，再带标签、更新时间、入链数。来源区写成一行：来源区的路径与来源的数目（同一次来源区的读数出：来源根的严格后代里的页与附件，不数 0 字节的页即文件夹，同 4.11 的待消化；`index.md` 的这一行、`listIndex` 的 `source_area.sources` 与目录视图数的都是这个，审查 B3-N3），与"用 `pending_sources` 列出待消化的来源、`lint` 看别的"；不写待消化的数目：它是"未消化"一条规则，每次 `read index.md` 与每次打开目录都要跑一遍，又在 lint 的名额之外（审查 B2-M2）。数据是 M9 的 `ListIndex` 的那一份（路径、类型、摘要、标签、时刻、入链数到 1,000，字符串截到 256 字节、标出截断），承袭它的界线（索引里没有行的页不在其中）；它的次序（类型的键、再按 id，`linking/domain/index.go:27-61`）不是 5.5 的分组，所以文件先读全部索引的页的类型键（`ListIndex` 每次本就读它们）与树，在 Go 里排出 5.5 的次序，再为前面的项读条目。**字节是起作用的界**：一行带一句摘要、标签、时刻与入链数约 200–250 字节，40 KiB 约放 170–200 项，项数的上限很少起作用，所以写到 40 KiB 为止（一次 `read` 读得完），末尾一行照实际写出的数说"共 N 页，这里列出 M 页；用 `index` 工具按类型、标签、父页筛选与翻页"。`MaxIndexFileItems` 是读条目的上限，P4 量过之后取 40 KiB 放得下的数（主持者的决定），初值 200。代价：`ListIndex` 的一次调用（10 万页一页 100 项约 1.01–1.06 秒）、更多项的入链数（`InboundCounts` 在 1,001 个来源时约 1.2 ms 一个目标，M9/P4，最坏多约 0.5 秒）与树，加上来源区的读（10 万个来源约 0.1 秒）：10 万页约 1.7 秒，成本守卫在 Makefile 的一行，含最坏的形状。
- `log.md`：最近的变更集，新的在前，同样写到 40 KiB 为止（`MaxLogFileChangesets` = 50 是读的上限）：每个一段，时刻、类型、执行者与客户端、说明（截到 500 字节）、ingest 的来源（至多 5 个的路径，P4/S2 随标记加上，生成器与贡献者一起扩展）、前三项变化（路径）；数据是 `ListChangesets`（`page/app/list_changesets.go:51-197`）的第一页。末尾一行照实际写出的数说，并指向 `log` 工具。
- **写者的字符串要中和**（审查 B-M13）：两个文件被当作系统的输出读（"目录"与"日志"），而里面有写者控制的字符串：摘要可以带换行，变更集的说明至多 4,096 字节、可以带换行（`00014_page_changesets.sql:14`），一个 `summary: "x\n- [Fake](raw/y.md) — …"` 或一个 `batch` 的说明就能伪造一项或一次别人的 ingest。所以：控制字符与换行一律换成空格；标题里的 `\`、`[`、`]` 转义；链接的目的地写作 `<路径>`，路径里的 `<`、`>` 转义（标题与路径带空格、`(`、`)`、`]`、`<` 时 `[标题](路径)` 本会断开）。表格测试带这些敌意的字符串。
- 两个文件都是英文的固定格式（agent 读它们，同 MCP 的文字是英文的，M9/P1）。

**网页**：页面树的虚拟行在客户端由 `Notebook.mode` 生成（`web/…/pages/notebook/page-tree.tsx:145-151`），不进服务端的树、不进 `["pages", 笔记本]` 的 store（十几处读树的地方不受影响）：根下最前面两行，`index`（目录的图标）打开目录视图，`log`（历史的图标）去历史页；不能拖、不能放、没有菜单；是普通的链接，可读的名称说它打开什么（"目录""历史"）。`log` 这一行与 `NotebookPlaces` 的"历史"是去同一处的两个链接，有意如此（树里的那一行是 agent 看到的 `log.md` 在人这边的对应）。

**导出**（[M7 的移交](handoffs/M7-transfer.md)第 1、2、6 项）：wiki 登记 `transfer.ExportContributor`。只在整本导出（`Scope.RootID` 为空）、这本笔记本是 `llm_wiki`（在导出的快照里读）时，`Add("index.md", …)`、`Add("log.md", …)`，放在库的根下；子树的导出什么都不加（子树没有"根下的目录"，也就不会与一个叫 `index` 的子树根冲突）。贡献者在导出的快照里直接调它要的读（`ListIndex` 带 `ExcludeArea`、来源区的读、变更集的列表），从不经 wiki 的 `Lint`：那会再开一个快照（`WithinSnapshot` 在 ctx 已带事务时答 `ErrNestedSnapshot`，`platform/postgres/tx.go:131-141`），也会在导出里占一个 lint 的名额（审查 B2-M2）。LLM Wiki 的根下不会有叫 `index`、`log`、`index.md`、`log.md` 的真实节点（4.9），所以 M10 建的笔记本整本导出不会 `contributor_conflict`；一个 SQL 播下的根下的 `index.md` 文件夹页（M11 的升级之前可能有）让导出答 `contributor_conflict`，测试钉住这个结果，M11 的升级要先处理保留的标题（"写给后面的 M"）。导出用的上限比 `read` 大：`index.md` 至多 10,000 项、2 MiB，**不带入链数**（审查 B-M12：1,000 页各链接全部 10,000 项在 `MaxLinks` 之内，入链数在导出的快照里最坏约 12.5 秒），`log.md` 至多 1,000 个变更集、1 MiB，各自末尾说明截断；至多两个文件，写明上限、由贡献者守住、测试钉住（13.1 第 31 条；M7 移交第 6 项：不加流式写入），成本守卫在 10 万页与 1,000 个变更集上（P4 量，含最坏的形状）。贡献者的错误不带名称（13.1 第 10 条）。
- **导入**：读到 `meta.json` 的 `contributed` 时跳过这些路径（`transfer/app/import.go:221`，精确的 NFC 路径，只认第一个 `meta.json`），重新导入一个 LLM Wiki 的导出不会多出 `index`、`log` 两页；没有 `meta.json` 的库（或它超过 16 MiB）把根下的 `index.md`、`log.md` 当作页导入：进 LLM Wiki 的根下时加序号（4.9），进普通笔记本照常。
- **最后一跳**：今天钉住档案的测试（`bootstrap/transfer_test.go:244-250`）导出的是普通笔记本，M10 对它什么都不加，照旧作对照；新的测试导出一本 `llm_wiki` 的笔记本，经 `metaPaths`（`:180-204`，今天不读 `contributed`，加上）读出 `meta.json` 的 `contributed`，再重新导入核对跳过。

### 4.14 ingest 标记

- **定义**：一个 `edit` 类型的变更集 C 是一次 ingest，当且仅当它新引用了原本未消化的来源 S：C 写了正文的某一页 P 是来源区以外的 wiki 页（不含来源根与 `AGENTS`，同 4.11 的"消化"），C 之后 P 以正文链接或 `sources` 属性链接解析到 S，C 之前 P 没有；并且 S 在 C 之前未消化。0 字节的来源页（文件夹）不算：同 4.11，它从不待消化，新引用它的页（如 `[[raw/papers]]`）不是它的 ingest（审查 B2-N4）。导入、还原版本、从回收站恢复与撤销的变更集不是 ingest（不是 agent 或人消化来源的动作）。C 记下它直接新引用的这些来源（跟着来源页消化的附件不另记，4.11）。
- **"C 之前未消化"的判定**（审查 B-M1）：C 写了或删除了的页，它们 C 之前的状态就在手上（旧的行），所以确切地判：这些页的旧行里没有一条消化 S 的链接；**并且**来源区以外别的页（C 没有写也没有删的）此刻都不引用 S。后一半按 C 之后的树与解析判，是标出的近似：C 里的改名让别的页的一条链接改为解析到 S、或不再解析到 S，C 里的移动让一页越过来源区的边界，都按之后的算（调查 §8：重新解析看不到页越过边界）。这几种都要 C 同时改树与写正文，按之后算只会少标或多标一个历史的标签，不影响任何规则；写进第 10 节。确切的一半的例子：一个整理的 `batch` 把引用 `[[raw/s]]` 的 Q 改掉、再新建引用它的 P，或删掉 Q、再新建 P：S 在 C 之前由 Q 消化了，C 不是它的 ingest。两种都有测试。
- **在哪里算**：linking 的索引观察者里（4.1）。它在锁下替换写了正文的页的行、删除删掉的页的行时拿到旧行：`ReplacePage` 与 `DeletePages` 改为按页交回旧的（`resolved_id`、`property_key`）对与旧的第一个折成 `sources` 的键（今天 `ReplacePage` 只交回目标、不交回属性，`DeletePropertiesOf` 什么也不交回，`linking/adapter/postgres/store.go:61-71`、`:151-172`；`linking/app/index.go:57-69` 的删除在判定之前），解析之后读新行；只对 `llm_wiki` 的 `edit` 单元做：每页 P 新得到的、解析到来源区的消化的链接 → 候选；去掉旧行里已消化的；剩下的候选**一条语句**（`unnest` 的数组，经 `postgres.Planned`）查来源区以外还有没有别的页引用它们（按 `resolved_id` 的部分索引），来源区的判断一次祖先的读（linking 已有的 page 端口）。为此组合根交给 linking 的值带上 `ChangesetID`（今天在组合根丢掉，`bootstrap/linking.go:25-38`）、`Kind` 与 `Notebook`。结果经 page 只凭连接池的 `page.NewIngestMarks(pool)` 写进 `changeset_sources`（一条以数组插入的语句，`seq` 是第一次记下的次序）并设 `changesets.ingest_count`，都在单元的事务里、索引的锁下。
- **界**：判定在每次保存都要等的索引锁里（`index.go:41`），而一页可以有 10,000 条链接（`MaxLinks`，`linking/domain/facts.go:101-109`）、一个 `batch` 写很多页，所以候选的检查与插入各是一条语句，不是每个候选一条。一个变更集至多记 `MaxIngestSources` = 10,000 个来源（编辑会话把几次保存并进一个变更集，每次都可以引用新的来源）：到了就不再记，`ingest_count` 等于记下的行数，到顶时历史写"10,000+"。成本守卫在 `batch` 的界上（很多页 × 每页很多新的来源链接），在 `TestABatchAtItsRewriteBoundTakesItsTime` 旁边；M12 的性能移交加一条。
- **并发**：同一本笔记本的两个单元在索引的锁上串行（13.1 第 5 条）：后一个在前一个提交之后才读，看得到前一个新写的引用，所以同一个来源不会被两个变更集都记成"新引用"。
- **编辑会话**：一次会话的几次保存并入同一个变更集（`page/app/unit_content.go:70-75`），每次保存各自判，标记只增不减（`(changeset_id, node_id)` 唯一，同 `renames`、`moves` 的"一旦为真就留着"）：会话里先加了引用又删掉，标记留着。
- **不变式**（`checkPages`，各有 SQL 播下的反例）：`changeset_sources` 只有存在的变更集的行（笔记本删除之后、清理之前，变更集已标删除而行还在，所以不写"活着的"）；`ingest_count` 等于这个变更集的行数（清理的谓词要取的变更集除外，4.2）；有标记的变更集都是 `edit` 类型。
- **读**：`listChangesets` 加筛选 `ingest`（布尔；`true` 只列 ingest，没有或 `false` 不筛），每条带 `ingest_count` 与前 3 个来源 `ingest_sources`；`getChangeset` 带 `ingest_count` 与前 20 个来源（id；网页按树写路径），是"N 个之中的前 20 个"，没有游标，REST 与 MCP 都写明。"前 N 个"按 `seq` 走主键：`unnest(ids) CROSS JOIN LATERAL (… ORDER BY seq LIMIT k)`，同 `ChangesetFacts`；`TestTheLogReadsUseTheirIndexes` 加这一种。筛选照日志的约定每种一条语句（`page/adapter/postgres/queries/history.sql:24-26`）：日志、按人的、一页的，各有带 ingest 的一条；前两种走 4.2 的两个部分索引，一页的日志带 ingest 时在这一页的变更集里走、按 `ingest_count` 过滤，界在 `TestTheLogReadsInTimeAsLongAsAPage`（200 ms）之内。MCP 的 `log`（`inputSchema` 加 `ingest`）与 `changeset` 同样，来源带路径。回收站里的与已清理的来源读作 `null`（`node_id` 不建外键）；清理不删 `changeset_sources` 的行，`ingest_count` 照旧数着清理掉的来源，写明。每个读者看得到（`page.history` 是读者的，负责人 2026-10-24 的"可见性"照 M8 的历史）。
- **网页**：历史的每一行在标记那一行（`web/…/pages/notebook/history-row.tsx:96-117`）加"ingest · N 个来源"，筛选加"只看 ingest"（`history-filters.tsx:94-155` 今天按人、客户端、页；URL 的参数是 `ingest=1`，13.2 第 10 条），变更集页列出来源的链接（回收站里的来源是文字，不是链接）；`kindText` 不变（ingest 是标记，不是新的类型：一次 ingest 同时是编辑或 batch）。

### 4.15 MCP：工具、prompt 与说明

**工具**：`lint` 与 `pending_sources`（4.11），由 wiki 的模块根 `Tools()` 交出、组合根的 `mcpTools` 拼接（`bootstrap/mcp.go:43-48`），照 M9 的每个工具：先写进 `api/mcp/tools.json`、权限矩阵有行、码进 `mcpToolCodes()`、答复经 `Fit`、注解只读、内置的说明有它们的一行（[M9/P1 的移交](handoffs/M9-P1-mcp-registry.md)第 8 条）。注册表是静态的（`listChanged: false`）：两个工具在两种模式的笔记本里都列出，`pending_sources` 在普通笔记本里答 409 `notebook.not_llm_wiki`，文字说清"只有 LLM Wiki 有来源区"，工具的说明写明"LLM Wiki only"（第 11 节设计的取舍第 20 条，主持者照准）。M10 改动的别的工具对象（`tools.json` 比较整个对象，`bootstrap/mcp_tools_test.go:82-121`、`:161-168`）与 `mcpToolCodes()` 的行逐个列在第 5 节，各在改它的 Phase 先写。

**prompt**（总体设计 5.9）：

- **三个**，英文（负责人 2026-10-24，第 11 节负责人的决定第 3 条，与工具的说明一致）：
  - `ingest`（`source?`：来源的路径；没有时先 `pending_sources`）：读来源 → 照 `AGENTS` 的约定定要写与要改的页 → 一个带说明（`ingest: <来源>`）的 `batch` 写完、每页的 frontmatter 带 `type`、`summary` 与带引号的 `sources` → `lint` 核对；
  - `query`（`question`，必填）：`index.md` 或 `index`、`search`、`grep`、`backlinks`、`read` 找答案，答复引用页与来源，值得留下的答案另写一页；
  - `lint`（无参数）：`lint` 的摘要 → 能机械修的（缺类型、缺摘要、无效的 `sources`）以 `batch` 修，来源一概不改（lint 本就不报来源里写的内容，4.11）→ 语义检查取最近的变更集（`log`）碰过的页 → 报告。
  每份至多 16 KiB，写法照 M9 的人工清单记下的 agent 的毛病（小的改动用 `edit` 不用整页的 `write`，`move` 的一步，`partial_lines`、`cut` 与 `read(offset, column)`，[M9/P1 的移交](handoffs/M9-P1-mcp-registry.md)第 6 条）；清单在 v0.1 的最后才跑，跑过之后照记下的改（4.19）。
- **只在笔记本入口**：`mcpserver.Prompt` 加 `Entry`（与 `Tool` 的同名同类型的位掩码，`platform/httpserver/mcpserver/tool.go:19-25` 的 `Spec.Entry`），今天的 `addPrompts` 两个入口都登记（`platform/httpserver/mcpserver/prompt.go:47`）。账户入口没有笔记本，prompt 要另带 `notebook` 才有意义，M10 不做。**能力按每个入口的 prompt 声明**：今天 `newEntry` 只要有任何 prompt 就给两个入口都声明 `prompts`（`mcpserver/server.go:144-149`），改为 `newEntry` 拿到这个入口的 prompt、有才声明；所以笔记本入口有 `prompts`、`listChanged: false`，账户入口没有。`TestTheMCPNegotiatesItsVersion` 今天钉住笔记本入口的能力恰是 `{"tools":{}}`（`bootstrap/mcp_test.go:314-327`），改为按入口核对。账户入口的 `prompts/list` 答空的列表，`prompts/get` 答 SDK 的"不认识的 prompt"（-32602，`sdk/mcp/server.go:899-916`）。`mcpserver.Prompt` 已有 `Title`；`PromptArgument` 不加 `Title`。
- **普通笔记本**：`prompts/list` 照样列出（静态），`prompts/get` 答 JSON-RPC 的错误（`GetPromptResult` 没有 `isError`，`sdk/mcp/protocol.go:966-985`）：码 -32602，`message` 是 `notebook.not_llm_wiki: <detail>`（同工具的错误文字），`data` 是 `{"code": …}`。
- **错误的分类**（[M9/P1 的移交](handoffs/M9-P1-mcp-registry.md)第 5 条；今天 `promptHandler` 一律答 -32603，`prompt.go:59-73`）：照 `httpserver.ProblemOf` 的类：一个 problem 在 `message` 与 `data` 里答它的码，4xx 是 -32602、5xx（如 `server_busy`）是 -32603，日志照 `toolError` 的级别；一个 fault 是 -32603，日志按它的类（`FaultGone` 记 debug、`FaultDeadline` 记 warn，`platform/httpserver/apierrors.go:270-273`、`mcpserver/errors.go:95-101`），客户端走了不会记成 error。
- **参数与答复的界**：参数照旧是字符串（`GetPromptParams.Arguments` 是 `map[string]string`），`Required` 由 mcpserver 核对（SDK 不核对）；`source`、`question` 至多 4,096 字节，超过的在处理函数之前答 -32602（`validation_failed`，写出参数的名字）：一个 60 KiB 的 `question` 放得进 64 KiB 的请求体（`mcpserver/edge.go:79-86`），回显进消息就会让答复过界。答复的消息合计至多 48 KiB，按 `TextBytes` 数编码之后的字节（`<`、`>`、`&` 与 C0 的控制字符各 6 字节，`mcpserver/answer.go:19-25`，所以 4,096 字节的这些字符是 24 KiB）；三份是静态的文字加至多两份回显的参数，表格测试在上限处用这些字符（`description` 与消息都回显时两处都放）；超过时答 -32603，是缺陷。请求体同别的方法至多 64 KiB，不另设例外。不加 `CompletionHandler`。
- **日志**（13.1 第 10 条）：`prompts/get` 同工具的调用记一行（prompt 的名字、入口、笔记本的 id、时长、结果），从不记 `source` 与 `question`。
- **模块根**：wiki 的 `Prompts()`（12.4 的"以后 `Prompts()`"），组合根拼接、交给 `mcpserver.New` 的 `Options.Prompts`（今天组合根不传，`bootstrap/mcp.go:85-93`）。
- **契约**：先写的 `api/mcp/prompts.json`（`{"notebook": [...], "account": []}`，每个数组是那个入口的 `prompts/list` 的 `prompts`），`TestThePromptListsAreTheContract` 比较整个对象，有反向对照（少一个、多一个、说明不同）。不加进 `tools.json`：今天有五处假定它恰好两个键（`bootstrap/mcp_tools_test.go:79-117` 的 `toolListDiff`、`:175-187` 的 `unnamedTools`，`bootstrap/permission_matrix_mcp_test.go:113-135`、`:382-388`，`e2e/stories/mcp/mc1-connect.spec.ts:22-25`）。
- **最后一跳**：经组合根拼起的三个 prompt 经笔记本入口的 `prompts/list`、`prompts/get` 到达处理函数（文字里有 playbook 的第一行），组合根不交 `Prompts()` 时失败；账户入口照上面答空的列表与"不认识"。e2e 的 MCP 客户端加 `listPrompts`、`getPrompt`（`e2e/fixtures/mcp.ts:115-156` 今天只有工具；`send` 遇到任何 JSON-RPC 的错误就抛出，`:86-108`，所以 `getPrompt` 交回 `{result} | {error}`，LW11 的普通笔记本要它），`PageRead.meta` 的 `version` 改为可选（`mcp.ts:183-196`，虚拟的读没有它）。

**说明**（M9 4.6）：

- **按模式与入口拼，每次拼**：笔记本入口的握手、账户入口的 `open_notebook` 经同一个函数给说明（今天两处各自选内置的文字，`page/mcp_entry.go:36-45`、`page/adapter/mcp/open_notebook.go:59-90`）；函数收"放得下多少"作参数：握手把 `AGENTS` 截到 `MaxInstructionsBytes`，`open_notebook` 截到 48 KiB 的答复减去它的骨架与 `openNotebookRootReserve` 之后剩下的（`open_notebook.go:66-80`），两种预算都先给系统的那一段留出位置。LLM Wiki 下是 `AGENTS` 的正文（照 M9 去掉 frontmatter、截断）加一段系统的说明（英文，至多 512 字节），每个入口一种：笔记本入口的是 "This notebook is an LLM Wiki: pages under `<来源根的路径>/` are sources you add once and never change; index.md and log.md are generated; run the prompts ingest, query and lint."；账户入口的 `open_notebook` 的最后一句改为 "its prompts ingest, query and lint are served at `/mcp?notebook=<id>`"（账户入口没有 prompt）。`AGENTS` 模板讲 prompt 的那一句照同样的意思写（4.4）。来源根可以被管理员改名、移动，它的路径可以过 2.5 KiB（标题至多 255 字节，`shared/title.go:13`；层级至多 10，`page/domain/node.go:23`）：这一段放不进 512 字节时，路径换成来源根的标题，仍放不进就截断并标出 `…`；测试在 10 层、每层 255 字节的路径上核对。路径每次拼时读一次（一次祖先的读，4.3）。缓存照旧只存 `AGENTS` 的正文、键是（笔记本、节点、版本号）（`page/app/instructions.go:36-87`）：系统的那一段每次拼，降级之后的下一次握手就没有它，不用让缓存失效。`AGENTS` 的正文被写成空的时，照 M9 给内置的说明，再加这一段。
- **内置的说明的余量**：账户入口 4,027 字节、笔记本入口 3,957 字节（`page/adapter/mcp/instructions.go:31-74` 的 `pathsAndTools` 共用 3,645 字节；审查 C 照 `TextBytes` 重数过），离 4 KiB 剩 69 与 139 字节。哪些字放在哪里：`grep`、`index`、`read` 讲虚拟页的那几句、`batch` 讲合成的那一句都放进 `tools.json` 的工具说明（4.13），不进内置的说明；内置的说明只加 `lint`、`pending_sources` 的两行（共用的部分）。P3 先缩短已有的行，缩到加上这两行之后每个入口还剩至少 300 字节，给 M11 的 `upgrade_verify`、`upgrade_submit` 两行；剩下的字节数写进 [M11 的移交](../M11-upgrade/handoffs/M9-mcp-upgrade.md)第 1 条（今天写着"只剩 69 字节"，P3 的文档提交改）。`TestTheBuiltInInstructionsAreShort`（`page/adapter/mcp/instructions_test.go:15-27`）照旧守 4 KiB，`TestTheBuiltInInstructionsNameEveryTool` 守每个工具都有一行。"upgrade" 在说明里今天指服务端的升级（`instructions.go:52-53`），M10 不加这个词。
- **`AGENTS` 必须有**（总体设计 5.2）：模板建它（4.4），守卫不许删除、改名、移动它（4.9）。

### 4.16 网页

| 部分 | 做法 |
|---|---|
| 建笔记本 | 对话框加模式（普通、LLM Wiki，默认普通，一句说明）；请求带 `mode` 与界面语言的 `template_locale`（网页总是带上）；新手引导不变 |
| 设置 | "常规"加"模式"一节：读者看到模式；管理员在 LLM Wiki 下有降级与普通的确认对话框（4.5：`NotebookStore.downgrade`、403、404 与 409 时重读列表、持有的对话框与焦点、`transfer.busy` 自己的文字） |
| 来源的判断 | 一处：`isSource(node)` 由 `Notebook` 的模式与 `source_root_id`、已加载的整棵树（附件也在，不分页）算出；由全节点索引（含附件）的严格祖先关系计算，mode/root/tree 变化可观察；根自身不是来源，nested AGENTS 按真实区域权限，下面每一处都用它。页面树若以后改为分层加载（总体设计 14 的页面树一行"不达标时改为分层加载"），这个判断要改为问服务端（"总体设计的修订"写明） |
| 页面树 | 图标的最小一组（lucide，`web/apps/web/package.json:31` 已有）：普通的页不加；来源根、来源、`AGENTS`、虚拟的 `index` 与 `log` 各一个（今天一个图标也没有，`web/…/pages/notebook/page-tree-item.tsx:8`），图标有文字的等价，进行的可读的名称（如 `raw (source area)`：后缀在名字之后、空格在 `sr-only` 之外），否则读屏的人得不到它唯一要说的事（13.2）；虚拟的两行（4.13）。菜单按角色与行的种类给：写者在来源根与来源上有"添加来源"与"新建页面"，没有改名、移动、删除与拖动；管理员在来源上全有，在来源根上有改名、移动，没有删除（来源根不能删除，4.9）；`AGENTS` 对谁都没有改名、移动、删除。**来源区里的"新建页面"先问标题**（同粘贴的对话框，一个标题的输入），再建页，不建 Untitled；对话框沿用 `useNewPage` 的两道防护（`useMounted` 与 `location.key` 的核对），409 `page.title_taken` 时在对话框里显示错误，不自动换下一个标题（13.2 第 7 条：表单从不自动重新提交）。理由：来源区里的树操作都要管理员，写者建出的"Untitled""Untitled 2"自己改不了名（`web/…/pages/notebook/new-page.ts:41-63`；第 11 节主持者的决定）。**写者拖进来源区之前确认**：拖放与"移动到…"的目的地在来源区里时（今天它们立刻移动，`page-tree.tsx:76-102`），先问一次，写明"移进来源区之后，只有管理员能移出或改名"；管理员不问 |
| 添加来源 | "粘贴或上传为来源"的对话框，在来源根与来源区的页上打开（写者以上）：粘贴（标题与 Markdown）经 `createPage` 带 `content` 一次写成（`api/modules/page.yaml:1998-2017` 已收 `content`，留着 BOM，`:2110-2115`）；`<textarea>` 把 CRLF 与 CR 都换成 LF，所以粘贴从不与一个 CRLF 的原文逐字节相同，对话框写明"要逐字节的副本请上传文件"。上传照附件的上传（同一个父页），归 `AssetStore`，关掉对话框之后照样传完（13.2 第 12 条）。选了 `.md` 文件时（后缀不分大小写）在客户端以 `new TextDecoder("utf-8", { fatal: true, ignoreBOM: true })` 读出它（留着 BOM、非 UTF-8 拒绝），标题取文件名去掉 `.md`、照标题的规则，照粘贴写成页（`.md` 不能作附件，总体设计 3.7）；混着选时 `.md` 成页、别的成附件；5 MiB 的发前上限只用于 Markdown 正文文件，普通附件沿自身上限；保留 BOM/原换行。AssetStore 用页面/附件结果分型处理混选队列，保持编辑器附件 Promise<Asset> 不变，关闭对话框继续、代际取消；409 不改名重投。`page.source_duplicate` 显示已有那一个的路径与链接（下面的 `duplicateText`） |
| 来源页 | 阅读视图，顶上一行"来源只能新增，不能修改"。凡是会写它的控件都收起（`isSource` 一处判）：编辑与 `Mod+E`（`web/…/pages/page/page-layout.tsx:153`、`:218-235`、`:263-273`）、任务的勾选框（`pages/page/reading-view.tsx:151`）、"还原这个版本"（`pages/page/version-page.tsx:134`，来源上总被拒）、锁的提示里的"在这里编辑"（`page-layout.tsx:301-306`）；写者看不到附件的行的改名、移动、删除（`pages/page/attachments-section.tsx:80`，要管理员）。附件照今天在正文之下列出（100 个一页、"更多附件"），总体设计 3.7 的例外本就成立，不另做（第 11 节主持者的决定）。**编辑中的页被移进来源区**：保存答 `page.source_immutable` 时，编辑会话失去编辑，原因是新的 `source`（今天 `lostBy` 只认接管、解锁、带 `lock` 的 `page.locked`、404 与 403，`web/…/stores/edit-session.ts:62-79`，这个 409 只是一次普通的保存失败，`stores/page-editing.ts:332-349`，编辑器照样可编辑、每次停顿都再保存再被拒）：先捕获会话 id 再真实结束服务端会话，编辑器实际只读、停止自动保存/心跳、保留未存正文供复制，late open/卸载/重复 end 幂等；正常连接断言 server lock 已释放，网络失败沿原租约收尾（今天 `lose` 与失去之后的 `end()` 都不放开它，`edit-session.ts:179-186`、`:270-274`，新的来源会一直被锁到租约结束，其间管理员删除它答 `page.locked`、别的标签页还提议"在这里编辑"） |
| 重复的来源 | `duplicateText` 与链接的辅助，挨着 `lockedText`，每一处收到 `page.source_duplicate` 的地方都用它：添加来源的对话框、上传的行（`web/…/stores/asset.store.ts:309-331`，含拖到页上与附件一节的）、树的拖放与"移动到…"的提示（`page-tree.tsx:140-144` 今天只显示 `errorText`）、回收站的恢复、撤销、任务的报告。已有的那一个可能是附件（树的 store 的 `byId`、`tree` 只有页，`stores/page-tree.store.ts:171-189`，`distinctName` 对附件什么也不答，`pages/notebook/distinct-name.ts:11-22`），所以按已加载的节点（附件在内）写路径；比这个标签页的树新时先重读，同 `pages/page/unresolved-link.tsx:96-103` 的 `go`；用 openOf 导航附件（父页/根页加 focus，不拼 page URL）；没有 `duplicate_of` 时照4.8区分同步未写与后台本批未写/前批保留 |
| 健康度 | 4.12 |
| 目录视图 | `/:slug/notebooks/:id/index`（在 `NotebookLayout` 之下），两种模式都有（总体设计 5.2"目录和日志在界面与工具中查看"），普通笔记本从 `NotebookPlaces` 的"目录"进（这个入口留着，LW13 的页面版本也从这里进），LLM Wiki 另有树里的虚拟行。只列 wiki 的页（`ListIndex` 的 `ExcludeArea`），来源区写成一行（来源区的路径与来源的数目，链接到健康度面板的"未消化的来源"；不写待消化的数目），同 `index.md`（第 11 节主持者的决定）。分组照 5.5：有类型的（4.10 的"缺"的反面）按类型（类型按标题键排），组里按建的先后（`ListIndex` 在一个类型里按 id 排，id 是 UUIDv7，写明是"按建的先后"；与 `index.md` 不同，它是一个文件、在 Go 里按路径排，两者的不同写明，审查 B2-N8）；没有类型的按父页分组（已加载的项）。筛选按类型、标签，都在 URL 里（`?type=`、`?tag=`）；类型从答复带出的类型与页数里选（`ListIndex` 每次本就读每页的类型键）；按父页的筛选要一个选择器（`parent-options.tsx` 有），可以进 M12 的打磨（第 11 节主持者的决定）。"更多"翻页。页调 `useDocumentTitle`。不是由事件驱动的视图，选项、加载与重试、缓存的规则同健康度面板（4.12），只是没有遇到 `server_busy` 不自动重试的 `onErrorRetry`：`listIndex` 不占 lint 的名额、不声明 503（第 5 节），它的读不到照 `retry.ts` 的规则（审查 C4-N2）；`listIndex` 答出算时的 `mode`，视图同样比 `useNotebook().mode` 标出模式的改变；键是一次打开的 `location.key` 加上类型、标签与游标，只在 SWR 里，换键时去掉旧的（13.2 第 7、15 条；审查 C3-N3）。数据是新的 REST 目录（第 5 节：linking 的 `ListIndex` 加按父节点 id 的筛选与以 id 写的项，`index.list`）；代价：一个 REST 操作与一页，`ListIndex` 每次读整本的索引，加上 `ExcludeArea` 与 `source_area` 的来源区的读（10 万页约 1 秒，10 万个来源再约 0.1 秒；审查 B3-N3） |
| 历史 | ingest 的标记、筛选（`ingest=1`）与来源（4.14） |
| 链接 | 虚拟页的链接（4.9 的定义，与服务端共用样例表）在 LLM Wiki 里打开目录视图与历史，读者也一样（`web/…/pages/page/unresolved-link.tsx:115-157` 今天会提议新建一页，读者在任何落点之前就看到"reader"） |
| 事件 | `mode`（4.5） |
| 文字 | 新码（`page.source_immutable`、`page.source_admin_only`、`page.source_duplicate`、`page.source_too_large`、`page.virtual_file`、`page.protected`、`notebook.not_llm_wiki`、`wiki.lint_rule_not_found`、lint 的 `server_busy` 的新的地方）、失败 `refused`、问题 `duplicate_source` 的中英文字：`problem-messages.test.ts` 要每个契约的码都有两种语言的文字（13.2 第 11 条），报告的失败与问题由 `pages/notebook/transfer-report.tsx:24`、`:79` 的类型表强制（tsc）。**由加它们的服务端 Step 一起写**，连同类型表的键与假服务端的字段（`web/apps/web/src/test/fakes.ts:91-100`、`stores/notebook.store.test.ts:7-17` 的 `Notebook` 加 `mode` 与 `source_root_id`），所以每次合并都是绿的（CI 的网页一份在每次合并时跑 `gen-check-web`、lint、`make knip`、测试与构建，`.github/workflows/ci.yml:126-161`；第 7 节） |

### 4.17 权限、码的次序、加锁与交错

- **操作与规则表**：`notebook.change_mode`（笔记本级，管理员）、`lint.read`（笔记本级，读者以上，wiki 的 `Actions()`）、`page.manage_source`（笔记本级，管理员，page 的；第 11 节主持者的决定）；目录的 REST 用已有的 `index.list`；`pending_sources` 是 `lint.read`。来源区的管理员规则是 `page.manage_source` 这一行：写者能做的操作（`node.move`、`node.delete`、`node.rename`）落在来源区里时，守卫经授权者问它（4.6），所以每一项权限都在 `access/domain/rules.go` 里（同 `page.release_edit_lock` 给笔记本管理员；`TestTheRuleTableIsTheModulesActions` 要 page 的 `Actions()` 列出它，`bootstrap/actions_test.go:20-27`）。`page.manage_source` 不是一个接口操作，矩阵的覆盖检查是按操作的（`matrixViolations`，`bootstrap/permission_matrix_coverage_test.go:21`），不认它；盖住它的是 LLM Wiki 的变体里来源上的改名、移动、删除的行（写者答 `page.source_admin_only`、管理员成功），没有另一道点名守卫问的动作的检查（审查 A2-N9）。
- **权限矩阵的 LLM Wiki 的变体**：矩阵的每一行今天以 SQL 播下的 id 为目标，每个笔记本的列从普通的笔记本 `priv`、`team`、`wiki`、`gone-nb`、`orphan` 取角色（`bootstrap/permission_matrix_notebook_test.go:154-166`）。变体在一个它自己的工作区里播下同样的几本 LLM Wiki 笔记本，同样的列有同样的角色，每本带 `mode`、`source_root_id`、`raw`、`AGENTS` 与一页带 `source_hash` 的来源（`checkPages` 在种子上跑）；放在自己的工作区里，`listedNotebooks` 的期望不变；矩阵因此按行的工作区核对目标（P1/S1 加上；`targetViolation` 今天要求每个目标在 `lab` 里）；MCP 的 `notebooks` 的期望（`accountNotebooksOf`）是手写的，播下变体的那个提交给它手写加上变体的笔记本。跑在变体上的行：`downgradeNotebook`（管理员成功）、`listLintItems` 的 LLM Wiki 的规则与 `pending_sources`（读者成功）、来源根的改名、移动（写者答 `page.source_admin_only`、管理员成功，P1；来源根谁也删不掉）、来源上的改名、移动、删除（写者答 `page.source_admin_only`、管理员成功，P2）；MCP 的矩阵每个工具恰一行，变体的行随 P3 的 `lint`、`pending_sources`；只在普通的目标上，这些成功的格子一个也出不来。
- **码的次序**：新操作照 13.1 第 4 条（404 → 403 → 422 → 409）；`downgradeNotebook`：404 → 403 → 409 `notebook.not_llm_wiki` → 409 `transfer.busy`；`getLint`、`listLintItems`、MCP 的 `lint` 与 `pending_sources` 照 4.11；`createNotebook` 的 `mode`、`template_locale` 照 `workspace_access`（4.4）；守卫的码在领域的码之后（守卫的 403 在领域的 422、409 之后），模式在锁之前，模式的码之间照 4.6；预检的位置照 4.6 的表。**`batch`** 照 M9/P3 的实际：计划的两遍按项的次序判领域与模式的规则（树的一遍、正文的一遍，4.6），编辑锁的预检在整个计划之后（`apply_batch.go:294`），所以 `[op0 写一页别人正在编辑的页，op1 编辑一页来源]` 答 op1 的 `page.source_immutable`，在 op0 的 `page.locked` 之前；单元里守卫的拒绝指出那一步的那一项（4.6）。13.1 第 4 条照它每个操作的写法把这些都写进去（第 7 节"总体设计的修订"）。
- **加锁**：降级同笔记本的管理写；建 LLM Wiki 在笔记本的管理事务里锁笔记本行、走页的分支，模板的单元重入它已持的锁（4.4）；来源区的 advisory lock 只在 `FOR SHARE` 的单元里取，在正文行之后、索引锁之前（4.8）；lint 与虚拟文件不加锁（快照）。13.1 第 5 条的修订写全这一支的次序，`edit_sessions` 与 `page_search_queue` 的位置照它今天写下的，P2/S2 核对。
- **交错**（13.4 第 4 条，确定性的测试，结束时 `checkPages`、`checkLinks`，碰到来源的另核对 `source_hash` 的不变式）：
  - 建 LLM Wiki 与工作区的删除、建的人的工作区成员身份的结束、他的工作区角色的改变（这些持工作区行 `FOR NO KEY UPDATE`，测到 `Seed` 的重入与事务里的判定），两种次序，同交错 15（`bootstrap/interleavings_notebook_test.go:58-95`）。初稿的"与同一工作区的另一次建"去掉：两次建只共有工作区行的 `FOR SHARE`，写的行互不相交，测试排不出任何次序。
  - 降级与 `AGENTS` 的删除（P1，两种次序：删除看到的模式就是它判的）；降级与一次来源的写（P2：持来源区的锁的第一次正文、在来源上开编辑会话，否决者在 `FOR SHARE` 下读模式）。
  - 降级与开始导入（拒绝或之后是普通的，在笔记本行上串行）。
  - 两次上传同一个文件进 `raw`（笔记本行的路径：两者都过了 `Check`、流完，后一个答 `page.source_duplicate`、它的文件被删掉）；一次改树的新增（带正文的新建、上传、移入）与一次 `FOR SHARE` 的、字节相同的第一次正文，两种次序（后一个答 `page.source_duplicate`：由笔记本行排出次序，不是 advisory lock）；两个空的来源同时写入相同的第一次正文（测试持着来源区的锁，键由导出的函数派生，4.8）。
  - 后台的恢复或撤销在两批之间，与一个字节相同的新来源：下一批以 `refused` 与码、节点停下，之前的批留着（P2）。
  - 两个单元同时新引用同一个未消化的来源（在索引锁上，`interleaveOnIndex`；只有先提交的是 ingest）。
  - lint 与并发的写、lint 与降级：lint 不持任何行，`interleaveOn` 排不了它；`WithinSnapshot` 在 ctx 已带事务时答 `ErrNestedSnapshot`（`platform/postgres/tx.go:131-141`），测试不能替它开快照。所以照导出的 `holdAssetBlobs`（`bootstrap/interleavings_transfer_test.go:51-63`）：种下一个未消化的附件来源，测试持着 `asset_blobs`，一次运行在来源区的读与各规则之后、读这个附件的信息时停下（来源区的读只读 page 的表，4.11）；另一个连接提交一次改名或一次降级。改的是来源区以外一页被答出的孤儿 wiki 页，它走 M8 的改名、不读任何附件的表；降级只写 `notebooks` 与 `nodes`。放开之后：运行在附件的信息之后才读路径（4.11 的次序），所以答出的是改名之前的路径；模式与来源根是快照的第一条语句读的，所以降级的那一半答出的 `mode` 是 `llm_wiki`、规则是 LLM Wiki 的（审查 B2-M1、B3-M2）。改名的那一半经 MCP 的 `lint`（它的条目带路径；REST 的条目是 id，改名不改 REST 的答复），降级的那一半经 `getLint`（它带 `mode`；审查 B4-N2）。变异的核对：在 `WithinSnapshot` 之外跑 `Lint` 时改名的那一半失败（READ COMMITTED 下答出改名之后的路径）；降级的那一半核对答出的 `mode` 与规则出自同一次读，在 READ COMMITTED 下也通过（模式与来源根在停下之前就读了，停下之后只读附件的信息与路径，降级改不了它们），它不是这个变异的核对（审查 B4-N1）。`nodes`、`notebooks` 与 linking 的表都不行：写或降级要读或写它们，持着会挂住测试。
- **日志**（13.1 第 10 条）：lint 每次运行一行 info（`notebook_id`、规则、每条的耗时与数目、过时的页数、等名额的时长），不记标题与目标；降级一行 info（只有 id，有日志的测试）；`prompts/get` 一行（4.15）；模板失败的错误不带名称；守卫的拒绝是普通的 4xx，不另记。

### 4.18 上限

| 量 | 上限 | 处 |
|---|---|---|
| lint 的数目 | 每条数到 10,000（`MaxLintCount`），之后"10,000+"（下界） | 4.11 |
| lint 的条目 | `limit` 1–100、缺省 50；摘要每条至多 5 项（放不下时一起减）；目标截到 256 字节；MCP 答复 48 KiB | 4.11 |
| lint 的运行 | 进程内的名额 `MaxLintRuns`（至少 1，取自请求的那一半，P3 定式子）；自己的期限 `LintTimeout`；同一个范围相同的请求在名额之外合并（不随第一个请求取消）；一个 `LintTimeout` 盖住等与跑，到期或没有名额答 503 `server_busy` | 4.11 |
| `index.md` | 40 KiB（`read`），写到它为止；读条目的上限 `MaxIndexFileItems`（P4 量过之后取 40 KiB 放得下的数，初值 200）；导出 10,000 项、2 MiB、不带入链数 | 4.13 |
| `log.md` | 40 KiB，写到它为止；读的上限 50 个变更集；导出 1,000 个、1 MiB；说明截到 500 字节 | 4.13 |
| 导出贡献者 | 至多 2 个文件，字节同上 | 4.13 |
| ingest 的来源 | 一个变更集至多记 10,000 个（`MaxIngestSources`），之后"10,000+"；历史的一行 3 个、变更集 20 个（"N 个之中的前 20 个"）、`log.md` 5 个 | 4.14 |
| playbook | 每份 16 KiB；`prompts/get` 的答复 48 KiB（`TextBytes`）；参数 4,096 字节，超过答 -32602 | 4.15 |
| 说明的系统段 | 512 字节；放不下时来源根的路径换成标题、再截断 | 4.15 |
| 内置的说明 | 4 KiB；加上 M10 的两行之后每个入口至少剩 300 字节给 M11 | 4.15 |
| 模板 | `AGENTS` 8 KiB | 4.4 |
| 一次既存子树移入来源区 | 10,000节点（空页/附件计数）；有限 reader 最多返回10,001便拒绝 | 4.8；普通大移动不变 |

新加的、由写者的内容决定数量的东西各在上限处有测试（13.1 第 31 条）。

### 4.19 真实客户端的人工核对与 M10 的状态

M9 的人工清单把 prompt 留给了 M10（`docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md:301`："prompts（ingest、query、lint）：M10 登记"）。只有真实的客户端看得出的：Claude Code 与 Codex 把 prompt 列成斜杠命令（总体设计 5.9）；它们怎样传 `source` 与多个词的 `question`；在普通笔记本里对 -32602 显示什么；说明里系统的那一段与 `AGENTS` 一起时 agent 照不照做（一次 ingest 写出带引号的 `sources`、不改来源）。所以 P5 在 M9 的清单里加一节"prompts 与 LLM Wiki"（不另开一份清单：同一次人工核对、同一个临时的环境），照 M9 的写法一步一步写、每步写明看什么；清单的范围之外那一行"prompts（ingest、query、lint）：M10 登记"（`:301`）换成指向这一节，不然清单自相矛盾（审查 C2-N4）。

M10 的状态照 M9 4.16：P1–P5 与收尾完成之后是"实现完成，待人工验收"；清单由负责人在 v0.1 的最后执行（同 M9），通过之后改为已完成，playbook 照记下的改。

## 5. 接口

**REST**（先写 `api/`，再写代码；新的 4 个操作，加上改的）：

| 操作 | 方法与路径 | 模块 | 说明 |
|---|---|---|---|
| `getLint`（新） | `GET /api/v0/notebooks/{notebook_id}/lint` | wiki | 适用的每条规则 `{rule, count, count_capped, items, next_cursor}`（前 5 项与接着它们的游标），`stale_pages`、`computed_at`、`mode`（算时的模式）；码 404、503 `server_busy`（4.11） |
| `listLintItems`（新） | `GET /api/v0/notebooks/{notebook_id}/lint/{rule}?limit=&cursor=` | wiki | 一条规则的 `{data, next_cursor}`；各规则的条目的形状照 4.11；`undigested_sources` 就是未消化的来源；码 400 游标、404、404 `wiki.lint_rule_not_found`、409 `notebook.not_llm_wiki`、422 `limit`、503 `server_busy` |
| `listIndex`（新） | `GET /api/v0/notebooks/{notebook_id}/index?type=&tag=&parent_id=&at_root=&limit=&cursor=` | linking | M9 的 `ListIndex`（MCP `index` 的同一份），`IndexQuery` 加按父节点 id 的筛选：`parent_id` 是一页的 id，`at_root=true` 是根下的子节点，两者同给答 422；项以 id 写（`{page_id, parent_id, title, type, summary, summary_cut, tags, updated_at, inbound, inbound_capped}`，6.1），去掉来源区的页（`ExcludeArea`；MCP 的 `index` 不用它）；答复 `{data, next_cursor, types, source_area, mode}`：`types` 是出现过的类型与各自的页数（不数来源区的页，目录视图的类型筛选从它选），`source_area` 是 `{root_id, sources}`（来源根与来源的数目，普通笔记本为 `null`），`mode` 是算时的模式。次序照 `ListIndex` 今天的（`linking/app/list_index.go:66-90`：游标 400，类型、标签、父节点的 422，含 `parent_id` 与 `at_root` 同给，都在 `l.check` 里，再 `limit` 的 422，然后笔记本的 404），声明为 13.1 第 27 条的一处例外：它是一个 MCP 工具的 REST 的一面，它的值的核对都与资源无关（13.1 第 4 条的那种例外），这样 M9 的 `TestListIndexJudgesInTurn`（`linking/app/list_index_test.go:149-175`）与文档都不改（审查 C2-M3） |
| `downgradeNotebook`（新） | `POST /api/v0/notebooks/{notebook_id}/downgrade` | notebook | 4.5；码 404、403、409 `notebook.not_llm_wiki`、409 `transfer.busy` |
| `createNotebook` | `NotebookCreate` 加 `mode`（`standard`、`llm_wiki`，缺省 `standard`）、`template_locale`（`zh-CN`、`en`，缺省 `en`）；不认识的值 422 `invalid_format`（`CheckDraft`，两种模式都核对） | notebook | 4.4 |
| `Notebook`（`listNotebooks`、`getNotebook`、`createNotebook`、`updateNotebook`、`takeOverNotebook`、`downgradeNotebook` 的答复） | 加 `mode`（必填）、`source_root_id`（必填，`[string, "null"]`）；`takeOverNotebook` 也答 `Notebook`（`api/modules/notebook.yaml:297`）。`NotebookUpdate` 照旧是关着的：带 `mode` 的 `PATCH` 答 400 | notebook | 4.3 |
| `listChangesets`、`getChangeset` | 筛选 `ingest`（布尔，`true` 只列 ingest）；每条 `ingest_count` 与 `ingest_sources`（前 3 个 / 前 20 个的 id） | page | 4.14 |
| 页面的写操作 | `x-problem-codes` 加下表的码 | page、asset、transfer | 4.6–4.9 |
| `common.yaml` 的 `Problem` | 加 `duplicate_of`（只有 `page.source_duplicate` 带它，能否带 id 由4.8的本单元起点来源 key 决定） | — | 4.8 |
| transfer 的 `Report`（导入、导出、恢复、撤销共用的 `TransferReport`，`additionalProperties: false`） | 失败 `refused`；`refusal: {code, node_id, duplicate_of}` 同 `lock` 必填、可为 `null`：`code` 是自由的字符串（守卫的码，M11 加冻结的），`node_id` 是被拒的那一步的节点（移进一棵子树时是子树里重复的那一个）；导入的问题 `duplicate_source`：`path` 是这一项在库里的路径，`to` 是已有的来源在笔记本里的路径，`counts.skipped` 数它。`unitFailure` 的测试钉住 `refusal` | transfer | 4.6、4.8 |
| `events.yaml` | `EventMode`，进 `streamEvents` 的 `oneOf` 与帧的列表 | events | 4.5 |

**写操作新加的码**（按操作，13.1 第 8、21 条；每个码在那个操作所在模块的 `adapter/http` 测试里答出过）：

| 操作 | 新加的码 | 在哪个 Phase 加 |
|---|---|---|
| `createPage` | `page.source_duplicate`、`page.virtual_file` | `virtual_file` P1/S2，`source_duplicate` P2/S2 |
| `putPageContent` | `page.source_immutable`、`page.source_duplicate` | P2/S1、P2/S2 |
| `toggleTask`、`openEditSession`、`restorePageVersion` | `page.source_immutable` | P2/S1 |
| `renameNode` | `page.source_admin_only`、`page.protected`、`page.virtual_file` | P1/S2（来源根的 `source_admin_only`），P2/S1（来源的） |
| `moveNode` | `page.source_admin_only`、`page.source_duplicate`、`page.source_too_large`、`page.protected`、`page.virtual_file` | P1/S2、P2/S1、P2/S2 |
| `deleteNode` | `page.source_admin_only`、`page.protected` | P1/S2（`protected`），P2/S1 |
| `restoreNode` | `page.source_duplicate`（保留的标题加序号，不答 `virtual_file`） | P2/S2 |
| `revertChangeset` | `page.source_admin_only`、`page.protected`、`page.source_immutable`、`page.source_duplicate`、`page.source_too_large`（撤销碰不到根下的 `index`；预检在 `page.revert_conflict`、它的 trimmed 与步数的 422 之后，`page.locked` 与 503 之前） | P1/S2（`protected`、来源根的 `source_admin_only`），P2/S1、P2/S2 |
| `uploadAsset`（asset） | `page.source_duplicate`、`page.virtual_file`（`virtual_file` 是 `Check` 的预检，在 `page.title_taken` 之后、`storage_full` 之前；`source_duplicate` 在流完之后的单元里） | P1/S2、P2/S2 |
| `startRevert`（transfer） | 开始时的预检（撤销的不加锁的那一遍，在 503 之前）：`page.source_admin_only`、`page.protected`、`page.source_immutable`、`page.source_duplicate`、`page.source_too_large` | P1/S3（前两个），P2/S1、P2/S2 |
| `startTrashRestore`（transfer） | 基本错误/admit 后、入队前的去重探查：`page.source_duplicate` | P2/S2 |
| `startImport` | 不加（重复的跳过、保留的标题加序号） | — |

`page.source_too_large` 只由 source-entry move/place 产生；restoreNode/startTrashRestore 不声明此码（恢复可分批），startImport 不声明 duplicate 4xx（报告 skip）。后台 refusal.code 仍是自由字符串，无新枚举。

**MCP**：

| 工具 / prompt | 参数 | 答复或内容 | 入口 | 模块 |
|---|---|---|---|---|
| `lint`（新） | `rule?`、`limit?`、`cursor?` | 摘要，或一条规则的条目（带路径）与游标；`stale_pages` | 两个 | wiki |
| `pending_sources`（新） | `limit?`、`cursor?` | 未消化的来源（路径、类型、字节数、时刻）与游标；普通笔记本 `notebook.not_llm_wiki` | 两个 | wiki |
| `list`、`read` | 照旧 | 加虚拟的 `index.md`、`log.md`（`virtual: true`） | 两个 | page（内容由 wiki） |
| 写的工具与按路径的读 | 照旧 | 虚拟路径答 `page.virtual_file`；模式的码 | 两个 | page |
| `log`、`changeset` | `log` 加 `ingest?` | 加 `ingest_count` 与 `ingest_sources`（路径） | 两个 | page |
| `notebooks`、`open_notebook` | 照旧 | 加 `mode`；`open_notebook` 的说明照 4.15 拼、根下先列虚拟的两项 | 账户 | notebook、page |
| prompt `ingest` | `source?` | 英文的 playbook | 笔记本 | wiki |
| prompt `query` | `question` | 同上 | 笔记本 | wiki |
| prompt `lint` | — | 同上 | 笔记本 | wiki |

**`api/mcp/tools.json` 里改的工具对象**（两个入口；`TestTheToolListsAreTheContract` 比较整个对象，描述与 `inputSchema` 在内），各在改它的 Phase 先写：

| 工具 | 改什么 | Phase |
|---|---|---|
| `notebooks`、`open_notebook` | 描述加 `mode`；`open_notebook` 的描述加系统的那一段与根下的虚拟项 | `mode` P1/S1，虚拟项 P4/S1，说明 P5 |
| `lint`、`pending_sources` | 新的两个对象 | P3/S1 |
| `list` | 描述加虚拟的两项，"`**/*.md` 是每一页"改写 | P4/S1 |
| `read` | 元数据的一行（十个成员）改写：虚拟的读加 `virtual`、没有 `version`；"每次重新生成" | P4/S1 |
| `grep`、`index` | 描述写明虚拟的两页不在其中 | P4/S1 |
| `log` | `inputSchema` 加 `ingest`，描述加 `ingest_count` 与来源 | P4/S2 |
| `changeset` | 描述加 `ingest_count` 与来源 | P4/S2 |
| `batch` | 描述加 4.7 的那一处不同（管理员在同一个 `batch` 里移出再写，去重比最后的正文）；两入口描述明示 source-entry 10k 界 | P2/S2 |
| `move` | 两入口描述明示既存子树从区外移入来源区的 10k 界 | P2/S2 |

**`mcpToolCodes()` 的行**（`bootstrap/permission_matrix_mcp_test.go:84-108`；每行的每个码都要是声明了的：REST 的 `x-problem-codes` 加 `mcpOwnCodes()`，`bootstrap/mcp_client_test.go:325-340`；`notebook.not_llm_wiki` 由 REST 的 `downgradeNotebook` 与 `listLintItems` 声明，不进 `mcpOwnCodes()`）：

| 工具 | 加的码 |
|---|---|
| `write` | `page.source_immutable`、`page.source_duplicate`、`page.virtual_file` |
| `edit` | `page.source_immutable`、`page.virtual_file`（`old_string` 从不为空，写不了空的来源的第一次正文） |
| `move` | `page.source_admin_only`、`page.source_duplicate`、`page.source_too_large`、`page.protected`、`page.virtual_file` |
| `delete` | `page.source_admin_only`、`page.protected`、`page.virtual_file` |
| `batch` | `page.protected`、`page.virtual_file`、`page.source_admin_only`、`page.source_immutable`、`page.source_duplicate`、`page.source_too_large` |
| `read`、`list`、`versions`、`backlinks`、`log`、`index` | `page.virtual_file`（`read index`、`list index/` 与按路径的读，4.13） |
| `lint` | `wiki.lint_rule_not_found`、`notebook.not_llm_wiki`、`server_busy` |
| `pending_sources` | `notebook.not_llm_wiki`、`server_busy` |

P2 增量逐格闭合：S1 write/edit/batch→source_immutable、delete→source_admin_only；S2 write/move/batch→source_duplicate、move/batch→source_too_large。notebook/account 两入口每工具每码以真实可达调用答出；move/batch 原有 admin_only 补来源后代行为，既有 protected/virtual_file 与矩阵保留（P2 §3.12）。

每个码经调用答出过（`bootstrap/mcp_reads_test.go:426-474` 的做法）。测试的环境今天只有一本普通笔记本，加一本 LLM Wiki 的。

契约：`api/mcp/tools.json` 与新的 `api/mcp/prompts.json`。新的码：`page.source_too_large`（422）、`page.source_immutable`（409）、`page.source_admin_only`（403）、`page.source_duplicate`（409）、`page.virtual_file`（409）、`page.protected`（409）、`notebook.not_llm_wiki`（409）、`wiki.lint_rule_not_found`（404）。

规模（第 11 节主持者的决定：新的操作算进 12.1 第 4 条的"约 20 个接口操作"，改的不算、但照它们定 Phase 的大小）：新的是 4 个 REST 操作、2 个工具、3 个 prompt，第 9 节 13 个故事，在"约 20 个接口操作或 15 个故事"之内，M10 仍是一个 M。改的约 22 个 REST 操作（上表的 12 个写、答 `Notebook` 的 5 个、历史的 2 个读、transfer 的 2 个读、`streamEvents`）、`Problem` 的 schema、每个入口约 14 个 MCP 工具：多是机械的，但每个码要一个适配器的测试与一次整个程序的答出，第 7 节照它们分 Step。

## 6. 从 Nerve 借鉴

Nerve 没有 MCP、prompt、数据的 lint、模式的契约或只增不改的区域（调查只读核对 `nerve-project`）。可以照搬的一样：**建的时候在同一个事务里写种子**：Nerve 建项目时在建项目的事务里写默认的状态（`nerve-project/server/internal/modules/project/app/create_project.go:113-139`），e2e 断言它们"随项目一起"写下（`nerve-project/e2e/fixtures/assert/project.ts:28-39`）。M10 的模板照这个做（4.4），e2e 的 LW1 照它断言模板的两页与笔记本同一个时刻、同一个变更集。其余沿用本项目自己的：写入单元、守卫、参与者与观察者（M4–M6）；派生数据由单元维护（M8/P5 的搜索入队）与"等于重建"的测试（M6）；导出的快照与贡献者（M7）；撤销的计划与临时名称（M8/P3、M9/P3）；MCP 的注册表与它的契约文件（M9）。

## 7. Phase 划分

| P | 名称 | 交付 | 验证 |
|---|---|---|---|
| P1 | 契约的框架（四个 Step） | **S1 模式与模板（服务端）**：00048；`Notebook` 的两列（契约里必填）；`createNotebook` 的 `mode` 与 `template_locale` 与它们的核对；notebook 的创建播种者与模式变化的订阅者（两个新的扩展点，模块根的替身测试与次序测试）；`notebook.New` 在没有播种者时 panic；page 的 `TreeWrites().Seed(SeedSpec)`（声明的例外）与 `ParseTemplate`；定客户端的函数挪到 `shared`（`ClientOf`，page、asset、transfer 的三份都删）；wiki 模块的骨架（模块根、契约表、两种语言的模板、播种者、`Actions()`）与组合根的次序；`downgradeNotebook`、`notebook.change_mode`、`transfer.NewWritingJobs`（降级的 `Forget` 在 P2/S2 加）；事件 `mode`、它的发布者与契约的 `EventMode`；MCP 的 `notebooks`、`open_notebook` 带 `mode`（`tools.json` 的两个描述；`open_notebook` 经 notebook 的根的 `StateOf` 读它）；`checkPages` 的"活着的 `llm_wiki` 有来源根与 `AGENTS`"；随这个 Step 的网页的文字、类型与假服务端的字段（`notebook.not_llm_wiki`、`Notebook` 的两列）。**S2 守卫的骨架**：`Write.Notebook`（page 的端口的 `ShareByID`、`LockByID` 交回状态，notebook 的根加 `ShareState`、`LockState`）；规则表的 `page.manage_source`，守卫经授权者问它；`page.NewModeGuard` 登记在锁之前（`pageRegistrants(pool, authorizer)`；命令行的会话的订阅者是 `pageSessionSubscribers()`，不收连接池）；`CheckContract` 与 P1 的规则：`AGENTS`、来源根的保护与它的管理员规则、根下的四个保留标题；拒绝的标出（`Refused`）与 `batch` 的 `failed`；`batch` 的计划的树的一遍调 `CheckContract`（本在 S3，挪进 S2：S2 与 S3 之间 `batch` 的码的次序与 REST、MCP 的 `delete` 相同）；P1 的码进各操作的 `x-problem-codes`（第 5 节的表）与 `mcpToolCodes()`，连同它们的文字。**S3 预检与加序号**：上传的 `Check`、撤销的核对（请求里的两遍；后台撤销的每一批不另检查，由那一批的守卫判）与 `startRevert` 开始时的预检，都调 `CheckContract` 并标出拒绝；**后台的拒绝的去处**：`revertDefect` 认出它，`treeError` 先认已有的哨兵（`ErrLocked` 是其中最后一个）、再认 `page.Refusal`，`unitFailure` 的 `refused`，`TransferReport` 的 `refusal` 与 `transfer-report.tsx` 的类型表与文字（同开始时的预检在一个 Step：P1 的规则已经让得出一个后台的拒绝，审查 C2-M4）；导入、同步与后台的恢复给四个保留标题加序号。**S4 网页（一）**：建笔记本的模式、设置的模式与降级（普通的确认对话框）、`mode` 的处理（重读笔记本的列表）与 `event.service.ts` 导出的 `EventMode`（与处理同一个提交：网页的 CI 的 `make knip` 不收未用的导出类型）、树里 `AGENTS` 与来源根的菜单与带文字等价的图标 | S1：两种模式、两种语言的建与模板的原子性（测试库的触发器注入失败）、同一时刻（时钟每读一次就走）、客户端；`schema_test` 的名称与反例；降级的码的次序、日志的测试、交错（降级与开始导入）；创建与工作区的删除、成员身份的结束、角色的改变的交错；e2e：LW1（API、MCP）。S2：P1 的规则在每条同步的路径上的整个程序的测试（守卫的最后一跳，P1 的子集写明：`putPageContent`、`toggleTask`、`restorePageVersion`、`openEditSession`、`restoreNode`、导入与 MCP 的 `edit` 在 P1 没有可拒的，P2 补全）；`[mode, lock]` 的次序（别人的会话持着 `AGENTS` 时删除它答 `page.protected`）；`batch` 指出被拒的那一项，计划的预检与守卫同码，计划与单元之间的降级由守卫拒绝（`guards: []` 时成功）；模块一级以假的守卫测 `replans`；降级与 `AGENTS` 的删除的交错；矩阵的新行；e2e：LW2（API、MCP）。S3：预检的拒绝与单元的拒绝同码（有预检的路径上只有守卫拒绝的情形：P1 有 `batch` 的计划与单元之间、后台撤销的两批之间的降级；上传、同步的撤销与 `startRevert` 的到 P2/S2 的重复才有）；四个键在导入、同步与后台的恢复里加序号、报告 `renamed`（后台的恢复数 `counts.renamed`）；后台：模块一级以假的守卫测 `revertDefect`、`treeError` 的标出与 `ErrLocked` 在前，整个程序里一次后台撤销要改回 `raw` 的名称：变更集也把根下的另一页改成了 `raw` 原来的名字，`raw` 的改回要等那一页的步，排过了第一批（每一轮按节点 id 的次序先放回，`page/domain/revert_schedule.go:292`、`:313`，而 `raw` 的 id 是这本笔记本最早的，名字空着时它就是第一批的第 0 步，审查 A3-N5）；两批之间执行者从管理员被降为写者，下一批的守卫拒绝、以 `refused`/`page.source_admin_only` 停下、报告带码与节点、之前的批留着。S4：e2e：LW1、LW2 的页面版本，LW12（页面、API；P1 的几步：另一个标签页随事件更新、`raw` 可以删除、`AGENTS` 可以删除、有导入排着队时 `transfer.busy`、写者看不到降级） |
| P2 | 来源区（三个 Step） | **S1 来源的规则**：`rewrite` 的迁移、`RecordRevision` 以"与"合并、撤销的计划带上它；`Step.Content`（StepContent，含 `Merged`）、`Step.Rewrite`、逐节点 Kind/祖先线；同步/后台实际动作事实与批界接口，compact/同组恢复与名义预检按P2 §3.13；4.7 的全部规则（正文只写一次、编辑会话的保存从不算第一次正文、来源区里的树操作要 `page.manage_source`、新增的各种），来源的会话否决者；`batch` 的计划的正文的一遍与 `Merged` 的后备；预检里补上来源区；后台的拒绝加来源的情形（去处在 P1/S3）；P2/S1 的码与文字。**S2 去重**：`source_hash` 的迁移（含补哈希）、单元维护它、`DeleteNodes` 与 `DeleteNotebooksPages` 清掉它、降级的 `Forget`（走部分索引）；附件端口加 `SHA256`、`WriterDeps` 拿到它；守卫的查重（页、附件、单元里、移入的子树）；来源区的 advisory lock 与导出的键的函数；`startTrashRestore` 的 CheckRestore admit；开始/worker 沿 S1 实际结构的 hash/snapshot；source-entry 真实10k工作界、全计划排除的有界游标、四列非唯一索引与空间/写入测量，600万空/已有LLM两种实际冷迁移本Step完成；导入里重复的页建成文件夹、`duplicate_source`；`duplicate_of` 的管道（problem、MCP 的 `structuredContent.error`、`refusal`）；`checkPages` 与"等于重建"；`batch` 的描述的那一处不同；P2/S2 的码与文字。**S3 网页（二）**：添加来源的对话框；`isSource` 与来源页的只读（每一处控件）；新的失去编辑的原因 `source`；来源区的菜单按角色与行的种类、"新建页面"先问标题、写者拖进来源区之前确认；`duplicateText`；撤销对话框/报告给出可执行的区外还原/再移回说明，后台非并发缩批仍可能拒绝且保留前批 | S1：4.7 的表的每一格经 REST 与 MCP 的整个程序测试（含历史与后台任务）；A-I1 的三个 `batch`（`[write raw/x, edit raw/x]`、`[edit 空的来源, 再 edit]`、`[edit P, 把 P 移进 raw, edit P]`）各在对的一项被拒；管理员移出之后再写的两个 `batch` 与写者的 `[write raw/x "A", write raw/x "A"]`、`[write raw/x "", write raw/x "B"]` 通过（`Merged` 只数页是来源时改了字节的项，审查 A2-M1、A3-M1），A-I1 的两种照样拒绝；编辑会话的保存在空的来源上被拒；撤销里临时的步、移动加改名、先恢复再写回、4.7 的三种偏严的形状各有测试；后台：整个程序里一次后台撤销在两批之间执行者被降为写者、下一批碰到来源的改名以 `refused` 停下（那一批的守卫拒绝，来源的情形）；e2e：LW4、LW5（API、MCP；LW5 的重复一步在 S2）。S2：P2 44组表中归属S2的原/追加变异与实际计划/范围证据，普通10001成功/来源有限物化、同snapshot全计划X跨10k、内部与SQL的baseline目标；有预检的每条路径各有一个只有守卫拒绝的重复（`batch`、上传、撤销、恢复，守卫的最后一跳由此完整）；后台的恢复在两批之间碰到一个字节相同的新来源，以 `refused`/`page.source_duplicate` 停下（只有守卫拒绝的那一种）；去重的交错（4.17）；`source_hash` 的性质测试与不变式的反例；成本守卫（一次新增、大的子树移进 `raw`、恢复与撤销在单元的上限而每个节点都在来源区里、`Forget` 在 10 万个来源）；e2e：LW3（API、MCP）、LW5 的重复一步、LW6（API）、LW7（API）。S3：e2e：LW3–LW7 的页面版本，LW12 加一步（另一个标签页里一页来源随 `mode` 成为可编辑的，之后重复不再被拒） |
| P3 | lint 与健康度（两个 Step） | **S1 服务端**：linking 的各规则的查询与 `page_properties_sources_idx` 的迁移（别的部分索引先量、按量的结果加，量它们在保存上的代价）；wiki 的 `Lint(LintScope)` 在一个快照里，名额（至少 1，取自请求的一半）、`LintTimeout` 与不随第一个请求取消的合并；`getLint` 与 `listLintItems`；MCP 的 `lint` 与 `pending_sources`（`tools.json` 的两个对象、`mcpToolCodes()` 的行）；矩阵的 `{rule}` 与 LLM Wiki 的变体的行；内置的说明缩短（每个入口至少剩 300 字节）并加两行，M11 的移交改写剩下的字节；`wiki.lint_rule_not_found` 与 `server_busy` 的文字。**S2 网页**：健康度面板 | 4.11 的每条规则在两种模式下的表格测试（含 `[[Node.js]]`、`![](x.png)`、来源发出的链接、文件夹页、0 字节的来源、嵌套的来源区、不加引号的与普通字符串的 `sources`、新建的 LLM Wiki 在两种语言里 lint 干净）；REST、MCP 结果相同；名额满时与到期时 `server_busy`、合并（第一个请求走了不连累别的）；lint 的快照与写、与降级的交错（`holdAssetBlobs` 的做法，改名一页答出的孤儿页经 MCP 的 `lint`、降级经 `getLint`，以 `WithinSnapshot` 之外的 `Lint` 作改名的那一半的变异的核对）；名额被占着时降级、排着的 `listLintItems` 答 409（B4-N3）；合并在名额之外（最小的配置下两个请求合得上）；10 万页的成本守卫（含内存的界）、小的分析过的种子上三个阶段的计划测试（13.4 第 8、9 条），各在 Makefile 的一行；内置的说明在 4 KiB 之内；e2e：LW8（API、MCP）。S2：LW8 的页面版本；面板的加载与重试的两个测试、`server_busy` 只发一次请求（C3-M1）；错过的降级在重连之后标出模式的改变 |
| P4 | 虚拟页面与 ingest（三个 Step） | **S1 虚拟的 `index`、`log`**：路径的一处把关（MCP 的工具与 `batch` 的计划）；`list`（游标的 `V`、glob 按键）与 `read`；`ToolDeps.Virtual`（为空时 panic）；wiki 的两个生成器（字节为界、中和写者的字符串、只列 wiki 的页与来源区的一行）；导出贡献者（上限、不带入链数、在 `llm_wiki` 的笔记本上的最后一跳、`metaPaths` 读 `contributed`）；导入的跳过与加序号的核对；linking 的 REST `listIndex`（按父节点 id 的筛选、以 id 写的项、`types`、`source_area` 与 `mode`，次序照 `ListIndex` 今天的，是 13.1 第 27 条的例外）与 `ListIndex` 的 `ExcludeArea`、"有类型"照 4.10；`tools.json` 的 `list`、`read`、`grep`、`index`、`open_notebook` 的描述。**S2 ingest**：组合根交给 linking 的值带 `ChangesetID`、`Kind`、`Notebook`；`ReplacePage`、`DeletePages` 交回旧行；一条语句的判定与 `page.NewIngestMarks`；ingest 的迁移（授权、sqlc）与清理变更集之前按批删除它的行的一步（循环到一批不满、行数算进清理器的返回、`SKIP LOCKED`，两条清理加 `NOT EXISTS`；清理的测试：删除了的笔记本在一次运行里删完、`PurgeNotebooks` 同一次成功）；`checkPages` 的三条；历史的读与筛选（每种筛选一条语句）；MCP 的 `log`、`changeset`（`tools.json`）；`log.md` 的生成器与贡献者加 ingest 的来源。**S3 网页（三）**：虚拟的两行；目录视图（两种模式，普通笔记本的"目录"入口）；历史的 ingest；虚拟页的链接（与服务端共用样例表） | S1：虚拟路径在每个工具与 `batch` 上的测试（`read index`、`read index.md.md`、`list index/`、移到根下的 `index.md`）；分页在虚拟项两边的页界，根下的子节点有负的排序；两个文件的内容、界线与敌意的字符串；`index.md` 在 10 万页上的成本守卫（含最坏的形状）；贡献者在 10 万页与 1,000 个变更集上；贡献者的最后一跳；e2e：LW10（API、MCP）、LW13（API、MCP）。S2：ingest 在真的库上的判定（4.14 的每一条，含改走与删除原来的引用）与交错；判定在 `batch` 的界上的成本守卫；`TestTheLogReadsUseTheirIndexes` 的新的情形；e2e：LW9（API、MCP）。S3：LW9、LW10、LW13 的页面版本，LW12 加虚拟行消失的一步；目录视图的加载与重试的两个测试、错过的降级在重连之后标出模式的改变（审查 C3-N3；`server_busy` 的测试只在面板，C4-N2） |
| P5 | playbook、说明与收尾的故事（一个 Step） | mcpserver 的 `Prompt.Entry`、按入口的能力、错误的分类、参数的界与日志、答复的上限；wiki 的 `Prompts()` 与三份 playbook；`prompts.json` 与它的检查；按模式与入口拼的说明（一个函数给握手与 `open_notebook`，收"放得下多少"，系统的那一段的路径规则），`open_notebook` 的 `tools.json` 的描述；e2e 的 MCP 客户端加 `listPrompts`、`getPrompt`；LW11；M9 的人工清单加"prompts 与 LLM Wiki"一节、范围之外的"M10 登记"一行换成指向它（4.19）；README 的 LLM Wiki 一节（模式、来源区、虚拟文件、playbook、降级、重新导入的限制）；总体设计随之的修订 | prompt 的最后一跳与反向对照（账户入口没有能力、答"不认识"）；说明的测试（G16 的换法加降级，路径在界上）；e2e：LW11（MCP），LW1 加握手的系统段的一步；全部 e2e 通过（本 M 与之前所有 M） |

**大小与绿**：P1 分四个 Step（初稿的 S2 是最重的一个：守卫的输入、否决者、保护与保留的标题与加序号、三处的预检、三处的拒绝的去处与每个写操作的码；预检、加序号与后台的拒绝的去处另成 S3：去处与开始时的预检在同一个 Step，因为 P1 的规则已经让得出一个后台的拒绝，审查 C2-M4），P2、P4 各三个，P3 两个，P5 一个，共 13 个 Step。每个 Step 合并时门禁全绿：CI 的网页一份在每次合并时都跑（`.github/workflows/ci.yml:126-161`），所以**每个服务端的 Step 自己带它加的码的两种语言的文字、类型表的键与假服务端的字段**（`problem-messages.test.ts:55-62` 在一个操作的 `x-problem-codes` 多了没有文字的码时失败；`refused` 进失败的枚举时 tsc 在 `transfer-report.tsx:24` 失败，`duplicate_source` 进问题时在 `:79`；`Notebook`、`Changeset` 加必填的字段时在假服务端失败），同 `feat(M8/P2A)`、`feat(M8/P3A)` 与 `page: the lock's contract, HTTP, matrix and texts (M5/P1/S3)` 的先例；网页的 Step 只加界面。网页的 Step 在它的服务端之后（12.1 第 5 条：前端不出现调用尚不存在接口的入口：虚拟的两行与目录视图都在 P4，P1 的树只给 `AGENTS` 与来源根的菜单）。每个故事与它的每个版本在上表里写明落在哪个 Step；没有 e2e 故事的 Step（P1/S3）以它的 Go 测试为验证。照 12.2 是"中偏重"：新的接口不多（第 5 节的规模），改的多，所以 Step 照改的分。不挪给 M11：M11 依赖的正是这里的契约（守卫、lint、来源区）。可以挪进 v0.1 之后的打磨的，在 P4 超出时由负责人定：目录视图按父页的筛选（选择器，第 11 节主持者的决定）、页面树的图标再多一组（只在"写给后面的 M"的打磨一条里列）。普通笔记本的"目录"入口留着（总体设计 5.2，主持者的决定）。收尾：三位 Opus 审查者并行（服务端、网页与 e2e、完成标准与文档），然后 Opus 核对修复。

**提交说明**（M9 总设计第 11 节第 111 条：从 M10 起每个 M 在 00 号设计里定一种；[文档约定](../../README.md)"Git 工作方式"）：M10 照文档约定成文的写法，只是 Step 的分支上的每个提交与它的合并都写到 Step，文档约定在三处写的是 Phase：Step 的分支上的文档提交（`docs/README.md:101`）、合并之前的审查修复与它的核对（`:100`"审查修复只写到 Phase"）、合并（`:102`）（审查 C4-N1）：
- 代码：`<范围>: <说明> (M10/P<n>/S<m>)`，例如 `server: add the notebooks' mode (M10/P1/S1)`；范围是改动所在的部件。
- 文档：直接提交到 `main` 的写 `docs: <说明> (M10/P<n>)`，M 级的 `(M10)`；Step 的分支上的每个提交，文档的也是（如 P5/S1 的 README 一节与清单的一节），都写 `(M10/P<n>/S<m>)`，`docs` 是约定的范围之一（M9 同样写过 `docs(M9/P5 S1): …`；审查 C2-N2）。
- 审查的修复：一个 Step 的分支上、合并之前的修复也写到 Step，`(M10/P<n>/S<m>)`（M9 同样写过 `fix(M9/P2 S1): …`；审查 C3-N2）；合并之后的 Phase 的审查的修复只写到 Phase，`(M10/P<n>)`；修复的核对写明第几轮：在 Step 的分支上、合并之前的写到 Step，`(M10/P<n>/S<m> fix check <k>)`，合并之后的 Phase 的写到 Phase，`(M10/P<n> fix check <k>)`（审查 C4-N1）；收尾 `(M10 closeout)`，收尾的修复同样；Codex 评审的修复 `(M10 Codex review)`。
- 合并：一个 Step 一次合并，`Merge branch 'm10-p<n>-s<m>': <说明> (M10/P<n>/S<m>)`（M9 的合并是 `(M9/P1 S1)`，同样写到 Step）；`docs/README.md:103` 已经写着"从 M10 起，每个 M 在它的 00 号设计里定一种写法，照它写"，所以这份设计被接受之后就许这样写，不改 README 的合并一行（`:102`；改了就成了每个 M 的规则，与 `:103` 相反）；只在记下设计被接受的那个文档提交里，在 `:103` 之后加一句"M10 照前三条（代码、文档、合并）的写法，只是在 Step 的分支上的提交（文档、合并之前的审查修复与它们的核对在内）与合并写到 Step"（审查 C2-N1、C4-N1）。审查修复、收尾与 Codex 评审的分支合并照它们的提交写（`(M10/P<n>)`、`(M10 closeout)`、`(M10 Codex review)`）。
- 旁支（修一个偶发的失败、调一个等待，M9 的 `m9-job-waits`、`m9-p1-s1-flake`）：分支名 `m10-<slug>`，提交与合并写它所属的 Step 或 Phase；修的是更早的 M 的故事时照 `a4-flake` 写 `(M<k> story, found in M10)`。
- `Merge main into <分支>`：照 git 的写法加一句说明，不带后缀（M9 同样）。
- 分支名 `m10-p<n>-s<m>`，旁支 `m10-<slug>`。

理由：约定的写法是成文的那一种，范围说出改动在哪个部件，后缀在末尾便于按 M、P、S 找；M9 的 `<类型>(M9/P<n> S<m>)` 是没有照约定（收尾审查 C-N6），不是另定的规则。

### 移交的落实

| 移交 | 项 | 落实 |
|---|---|---|
| [M5 实时推送的事件类型](handoffs/M5-events.md) | 笔记本模式的变化 | 类型 `mode`，按笔记本过滤，只带 id；在写模式的事务里经组合根的适配发出；契约 `EventMode`，进 `streamEvents` 的 `oneOf` 与帧的列表（`api/modules/events.yaml:21-86`），`services/event.service.ts` 导出这个类型（M6 的事件类型移交第 1 条的做法）；前端的处理表（重读笔记本的列表；健康度面板与目录视图不在表里，比答复里的 `mode` 与 `useNotebook().mode` 标出模式的改变，重连之后也成立），`refreshedOnConnect` 不加键；替身类型 `mode` 换成真的，一般的那条测试换一个编造的类型；最后一跳：降级经 REST 之后另一条流收到帧（帧按契约核对）、前端的处理经组合根的表（`app/event-stream.test.tsx`），组合根交空时失败；工作区级的缺口不涉及（4.5，P1 的 S1、S4）。M11 的同一份照它 |
| [M6 lint 的数据在链接索引里](handoffs/M6-links.md) | 表的六行 | 待建：改为按写的目标分组、数页（只按 `target_key` 会把 `[[a/x]]` 与 `[[b/x]]` 合在一起），读作附件照代码的 `ReadsAsAsset` 对着 page 的 `ByKeys` 的候选，解析不到的带扩展名的图片与嵌入不算，来源发出的不算；孤儿：反链的查询加豁免（含文件夹页），来源发出的链接不算入链；歧义：`ambiguous`，保留代码与 4.4 的含义、改正 5.6 的说法，来源发出的不算；解析失败：`frontmatter_valid`，不含来源；`sources`：`property_key`，第一个键的规则，深度不记照旧，另认出不加引号的嵌套列表；缺类型、缺摘要：`page_properties` 加 00047（4.10、4.11，P3） |
| 同上 | 注意 1–4 | 1：快照里只剩提取器的过时，跳过、计数、说请服务器的管理员运行 `nervewiki reindex`（没有索引行的页在发布的实例里没有，不找）；2：M10 不加提取的东西、不提升 `domain.Extractor`；3：linking 的 SQL 不连 `nodes`，来源区（含种类、时刻、字节数）与路径经 page 的端口；4：来源区由模式与 `source_root_id` 定（4.3、4.11，P3） |
| [M7 导出贡献者、附件与来源区](handoffs/M7-transfer.md) | 1–6 | 1：wiki 登记贡献者；新的整个程序的测试整本导出一本 LLM Wiki 的笔记本，zip 里有 `index.md`、`log.md`，经 `metaPaths` 读出 `meta.json` 的 `contributed` 列出它们，组合根交空时失败；`bootstrap/transfer_test.go:244-250` 钉住的是普通笔记本的档案，M10 对它不加什么，照旧作对照；2：核对导入按 `contributed` 跳过（库的根下的路径、只认第一个 `meta.json`），没有 `meta.json` 的库加序号；3：去重：附件按 SHA-256（上传的 `Step.Asset`，移入与恢复经加了 `SHA256` 的附件端口）、页按正文的哈希，记在 `nodes.source_hash`（4.8）；4：来源区的页的附件照今天在正文之下列出（100 个一页、"更多附件"），总体设计 3.7 的例外本就成立，不另做（第 11 节主持者的决定）；5：lint 不把附件当页（孤儿、缺类型豁免附件，`resolved_asset` 的链接不算入链），读作附件的目标照代码的定义不进待建（4.11）；6：贡献者的输出有界：至多 2 个文件、`index.md` 2 MiB 且不带入链数、`log.md` 1 MiB，测试钉住，不加流式写入（4.13）。P2、P3、P4 |
| [M8 来源区的守卫与撤销、还原、恢复](handoffs/M8-history.md) | 1–3 | 1、2：照旧（`Write.Kind`、`RevertsID`，撤销的每一步经守卫），守卫另看 `Step.Rewrite`；3 的五条：写回 C 改写过的来源页按版本行的 `rewrite` 放行；还原来源的版本一律拒绝；恢复来源、撤销一次删除算新增（写者），去重适用（比活着的来源）；撤销新建来源或来源的移动照删除、移动要管理员；同步的整组拒绝，后台的在开始时预检（含去重的探查），之后在那一批以 `refused` 停下（由那一批的守卫拒绝），码进各操作的 `x-problem-codes`、网页有文字；逐步判比净效果严的三种形状写明，对话框说管理员可以分几步撤销（4.6、4.7，P1 的 S2、S3，P2 的 S1、S2；完整网页解释在S3，P2/S3验收后本移交才done） |
| [M9/P1 MCP 的 prompt、虚拟文件与 `AGENTS`](handoffs/M9-P1-mcp-registry.md) | 1 注册表、2 最后一跳与契约 | 1：`Prompt` 加 `Entry`（同 `Tool` 的位掩码）、能力按入口声明、错误分类、参数与答复的上限，不加 `PromptArgument.Title` 与补全；2：最后一跳（笔记本入口到达处理函数，账户入口没有能力）与 `prompts.json`，不改 `tools.json` 的两个键（4.15，P5） |
| 同上 | 3 虚拟文件 | 路径的一处把关（`FindPath`、`WalkPath` 的查找，含 `batch` 的计划），四个保留的键；`list` 的子节点以游标的 `V` 一员列在最前（不用 `sort_order` 的值：排序可以是负的）、glob 按键走到（`**/*.md` 配得上），`grep` 不搜；写由 `page.virtual_file` 拒绝；`index.md` 与 `log.md` 都写到 40 KiB 为止、按实际写出的数说截断；`ListIndex` 的代价写在 4.13（约 1.7 秒，P4 的 S1） |
| 同上 | 4 `AGENTS` 的保护 | 模式的守卫（4.9，P1 的 S2）；说明照 M9 4.6 取，加按入口的系统的一段（4.15） |
| 同上 | 5 `prompts/get` | 64 KiB 放得下参数，不另设例外；参数过 4,096 字节在处理函数之前答 -32602；错误经 `ProblemOf`，4xx 的 problem -32602、5xx 的 problem 与 fault -32603，日志按类；答复至多 48 KiB（`TextBytes`）（4.15，P5） |
| 同上 | 6 余量、故事的约定、prompt 的说法 | 余量：先缩短共用的行，缩到每个入口加上 M10 的两行之后还剩至少 300 字节给 M11，M11 的移交改写剩下的字节（P3）；讲虚拟页的字放进 `tools.json` 的描述；故事的 MCP 版本照 M9 的约定、逐个故事判（第 9 节）；playbook 的写法照人工清单记下的毛病（4.15，P5） |
| 同上 | 7 守卫要认得的四种步 | 移动加改名：两样都看；`batch` 的合成：正文的两条规则由计划按项判（计划是权威），守卫看合成之后的一步，`Merged` 是后备，去重比最后的正文（与分开调用的一处不同写进第 10 节）；临时名称：逐步判、不加标记，理由与三种偏严的形状写在 4.7；先恢复再写回：恢复是新增、写回照 `rewrite` 判（来源事实/按项正文/管理为P2/S1；查重及最终合成正文差异为S2） |
| 同上 | 8 `lint` 与 `pending_sources` | 照 M9 的每个工具（4.11、4.15，P3 的 S1） |

开工时（P1 之前）五份移交照上表写进各自的"处理进展"，仍是 `open`；做完的 Phase 改它们为 `done`，收尾核对。

### 写给后面的 M

- **M11**（发现它的 Phase 写进 `M11-upgrade/handoffs/`，收尾核对）：
  - **状态**：升级的状态是 M11 自己的列或 `notebook_upgrades` 的状态，不扩 `notebooks_mode_check`（4.3）；升级期间按普通的契约；`source_root_id` 在普通笔记本里可以先定（检查只要求 LLM Wiki 有它）；批准时把 `mode` 改为 `llm_wiki`，并为来源区算出 `source_hash`、同时清掉别处的（不变式按模式与生死写，4.8：M10 只在单元里维护它，升级的批准不经单元时要另算，等于重建）。
  - **保留的标题**：升级之前的笔记本根下可能已有 `index`、`log`、`index.md`、`log.md` 的真实节点（含一个 `index.md` 的文件夹页，它会让整本导出答 `contributor_conflict`，4.13），批准之前要改名或拒绝。
  - **守卫**：冻结与护栏登记在 `pageRegistrants` 的守卫里，建议排在模式与锁之前；`Write.Notebook` 是加升级状态的地方；模式的拒绝不论出自守卫还是预检都已标出（`Refused`：`replans`、后台撤销、transfer 的 `refused`），冻结的码照样得到这些去处；`refusal.code` 是自由的字符串，加冻结的码不改契约。
  - **闸门**：经 wiki 的 `Lint` 的内部入口，传显式的范围 `LintScope{Contract: LLM Wiki 的, AreaRootID: 提议的来源根}`（升级期间笔记本仍是 `standard`，按笔记本的模式取会算不出 LLM Wiki 的规则、答 `notebook.not_llm_wiki`；4.11），在同一个快照里取，不另写查询。第 4 条（来源区以外的每篇页面的 frontmatter 能解析）是 `invalid_frontmatter`（LLM Wiki 的契约里本就不含来源）；第 5 条（`type`、`summary` 不为空，`sources` 指向来源区里的节点）是 `missing_type`、`missing_summary`、`invalid_sources`（负责人 2026-10-24：必填属性在 M10 只检查，M11 的提交以它为闸）；第 6 条"解析不到的链接数不多于升级前"要它自己的数：`wanted` 数的是不同的写下的目标，不含读作附件的、虚拟的与来源发出的，到 10,000 就停。**过时的页**：lint 跳过并计数过时的页（`stale_pages`），第 4、5 条在这些页上会放过；`stale_pages > 0` 时拒绝提交，或先 reindex。
  - **内容保全**：第 7 条"差异只来自系统的链接改写"可按版本行的 `rewrite` 判。
  - **事件、说明、降级**：状态变化发 `mode`；说明里不用"upgrade"一词（它指服务端的升级）；内置的说明在 P3 之后每个入口至少剩 300 字节，够 `upgrade_verify`、`upgrade_submit` 的两行（[M11 的移交](../M11-upgrade/handoffs/M9-mcp-upgrade.md)第 1 条随 P3 改写剩下的字节数）；降级在升级期间如何（拒绝或先放弃升级）由 M11 定；`notebook.change_mode` 可以给发起与批准用；降级的确认对话框写的是"升级是另一个流程"，M11 做成之后仍然成立，不必改。
  - **已有的笔记库**：把一个 LLM Wiki 的导出重新导入一本新的 LLM Wiki，得到 `raw 2` 与 `AGENTS 2`、来源落在来源区以外（4.9，v0.1 写明的限制）；已有的笔记库（含这样导回来的）走 M11 的升级定来源根。
  - **playbook**：upgrade 的 prompt 登记在 wiki 的 `Prompts()`，同样只在笔记本入口、静态、在别的模式里答错误。
- **M12**：进 [M12 的性能移交](../M12-release/handoffs/M4-performance.md)：lint 每条规则在 30 万页上的耗时与内存（快照占一个连接的时长）与名额 `MaxLintRuns`（取自请求的那一半）与 `LintTimeout` 在真实的负载下够不够；ingest 的判定在索引锁里的时长；`index.md` 每次 `read` 读整本的索引、树与来源区（10 万页约 1.7 秒），目录视图同 `ListIndex` 加来源区的读；`source_hash`、ingest 与 linking 的迁移在大的实例上的停机时长（ACCESS EXCLUSIVE，读与写都等），写进升级说明；去重的 advisory lock 在大量并发新增时的等待；降级的 `Forget` 在大的来源区上挡住写的时长。
- **v0.1 之后的打磨**：追加到 [M12 的打磨移交](../M12-release/handoffs/M5-polish.md)（第 33 条起，同 M9 写进它的第 31 条）：健康度面板的刷新与推送；页面树的图标再多一组；目录视图按父页的筛选（选择器）。来源的差异对比与中文的 playbook 不在这里：前者是第 2 节的"不做"，后者是负责人的第 3 条决定（只有英文），要改由负责人提。
- **M10 的收尾**（P1/S2 记下，[P1 S2 审查](reviews/P1-S2-guard-review.md)的主持者的决定第 30 条）：M9 的 grep 的成本守卫 `TestGrepOfAHundredThousandCandidatesTakesItsTime` 在 P1/S2 的建成的 CI 运行上红过一次（没有统计时 `GrepCandidates` 的中位数 388 毫秒，它的界 270 毫秒）：S2 不碰它，本机上 S2 与它的起点量得相同，修复一轮的运行过了它。它在别的分支或 main 上再失败时，照 M9 给它的兄弟的界的改法（`0f0ccbf9`，调用的界提到 1 秒）：以量过的 CI 的中位数定界（写进 `grepCandidatesBound` 的注释，放宽到比最慢的 CI 的中位数还有余量），或计时五次取中位数；不在 M10 的哪个 Step 里改。收尾核对它之后有没有再失败。 后续 P1 完成文档 `b112450f` 的 main run `40378004872` 再次失败：generic 三次为 195.1798、389.483274、415.468253 ms，中位数 389.483274 ms 超过旧界，完整调用中位数 420.822970 ms 仍在 1 s 内。按本项和工作流第 5 节，在 P1/P2 之间另开 `m10-grep-budget` 维护旁支；主持者依据已测最慢 CI 阶段中位数 389.483274 ms，接受候选界 600 ms、保留各阶段三次中位数；三个独立审查均无核实发现，真实点查、计划和范围反向对照已完成，六项本机门禁通过，代码为 `789b4071`。完整调用 1 s 与其他守卫不变。依据、原始样本、探针、反向对照、审查及实际完成的门禁见[候选预算维护审查](reviews/grep-candidates-budget-review.md)决定 35（P1 实施与维护的编号，与本设计第 11 节独立）。代码分支 CI `40388475778`（精确 `789b4071`，attempt 1）十个作业全部成功并已核对；维护文档 `751719ea` 的分支 CI `40392243355` 已通过；旁支以 `--no-ff` 合并为 `2322244a`，首次 main CI `40394818624` 十个作业全部成功并经主持者核对。本次合并记录提交及其 CI 尚待进行，P2 入口仍关闭，须该提交的 main CI 全绿并经主持者验收后才开；M10 真正收尾时仍须继续核对维护后的复发，不能把这次维护写成 M10 收尾已完成。
- **M10 的收尾追加**（P1/S3，[S3 审查](reviews/P1-S3-prechecks-refusals-numbering-review.md)主持者的决定第 34 条）：核对 `TestIndexOfAHundredThousandPagesTakesItsTime` 的 IndexItems 成本守卫在 CI 上的稳定性，与上一项 grep 分别记录。S3 第二次 main CI 的 analyzed 三个批次都超过 30 ms；保留失败与后续运行的全部样本，即使后续全绿也不把它写成已稳定。本次不改代码或阈值；同一失败再现时先做窄范围的计划及服务端/客户端耗时归因，不再用文档推送换绿。若收尾需要校准界线，须依据实测分布、保留并验证整值读取的反例与计划守卫，同时更新现行预算约定。

### 总体设计的修订

随这份设计一起改的（已落到 `v0.1-design.md`，与这份设计的定稿同一个提交；最后"随实现它们的 Phase 改"的几条除外）：

- **3.3**：笔记本管理员的"修改模式"在 M10 是降级、M11 加升级；"删除或移动来源"改为"来源区里的改名、移动、排序、删除"（规则表的 `page.manage_source`），来源根的改名、移动也是。
- **3.8**：变更集另记 ingest 的来源（`changeset_sources`，`ingest_count`），只对 `edit` 的变更集；版本行记下是不是系统的链接改写（`rewrite`）；模板是笔记本的第一个变更集。
- **3.10**：从回收站恢复到 LLM Wiki 的根下时给保留的标题加序号；恢复进来源区是新增，可能因重复被拒（同步的整组拒绝，后台的在开始时或那一批以 `refused` 停下）。
- **4.2**：`type`、`summary` 的"LLM Wiki 模式下必填"改为"LLM Wiki 模式下由 lint 检查，不拒绝写入"，"缺"的定义（4.10）。
- **4.4**：末尾加：LLM Wiki 下指虚拟页的链接（没有别的同名的页时解析不到的 `[[index]]`、`[[log]]` 等，4.9 的定义）不算待建，网页打开目录视图与历史。
- **4.5** 第 5 条：系统的链接改写在单元里标出（`Step.Rewrite`）；撤销写回一个改写的版本同样不算修改。
- **5.1**：写入时的不变量是来源区、`AGENTS`、来源根与保留的标题；必填属性是 lint（负责人 2026-10-24）。
- **5.2**：frontmatter 一行改为"识别；LLM Wiki 由 lint 检查 `type`、`summary`"；"通用 lint"一行加"健康度面板"；`AGENTS` 一行的"必须有"写明由模板建、守卫保护；"目录和日志在界面与工具中查看"照旧，普通笔记本的入口是 `NotebookPlaces` 的"目录"。
- **5.4**：照 4.7、4.8 改写：来源根是笔记本行上记下的一页（`source_root_id`，由模板定下，M10 不能改指），不是"笔记本的一项设置"；写一次、之后只读（两种例外：系统的链接改写与它的撤销写回，空的来源的一次性的第一次正文）；来源区里的树操作要 `page.manage_source`；新增的各种（新建、上传、移入、导入、恢复）；去重的范围（活着的来源，页按字节、附件按 SHA-256，不跨种类，比现在的字节；改写让两个来源相同是允许的）与位置的指出；"已消化"的定义：来源区以外的 wiki 页（不含来源根与 `AGENTS`，虚拟页不算）以正文链接或 `sources` 的第一个键引用它，区里的附件跟着链接或嵌入它的来源页，0 字节的来源页不算待消化；ingest 的定义与近似（4.14）。
- **5.5**：虚拟文件的内容（只列 wiki 的页，来源区一行）、以字节为界与截断的说法、写者的字符串中和；`list` 列出、`grep` 不搜；保留的标题含 `index.md`、`log.md`，对附件也保留；"导出时写成真实文件"限整本导出；目录视图两种模式都有，分组照 5.5（有类型的按类型，没有的按父页）；来源区的一行只写路径与来源的数目。
- **5.6**："链接有歧义"改为"几页在每一项偏好上都相同、按 id 选了一页"（同 4.4）；待建页面按写的目标、按页数，读作附件的目标（照代码的定义）与解析不到的带扩展名的图片、嵌入不算；lint 不看来源里写的内容（来源发出的链接不产生待建与歧义、不让页不算孤儿，来源的 frontmatter 不查）；"未消化的来源"一行改为"来源区以外的 wiki 页都没有引用的来源"；文件夹页不算孤儿；豁免写全；健康度面板能读笔记本的人都看得到，打开时算；每条数到 10,000（停下的是下界）；同时的运行有界。
- **5.7**：模板与建笔记本在一个事务里；模板的项只有 `raw` 与 `AGENTS`（虚拟的 `index`、`log` 不是模板写的，是模式的契约给的）；模板用建的人的界面语言（中文或英文，没给时英文），之后由用户改写；来源根记在笔记本行上（不是"由笔记本设置按 id 记录"）；建的对话框选模式，新手引导照旧是普通的；来源根的改名要管理员；"空笔记本天然满足"读作新建与升级都不必另写什么（4.5）。
- **5.8**：图里的 `upgrading`、`reviewing` 是升级的状态，不是模式；降级是专门的动作（`downgradeNotebook`），有写入任务时拒绝，清掉来源根与来源的哈希。
- **5.9**：ingest、query、lint 三份是英文的，只在笔记本入口，每本笔记本都列出，普通笔记本里答 `notebook.not_llm_wiki`；真实客户端的核对在 M9 的人工清单里（4.19）。
- **6.1**：problem 的扩展成员加 `duplicate_of`；新的码。
- **6.3**：工具表的 `lint`、`pending_sources` 写明两个入口、`pending_sources` 只在 LLM Wiki 有结果；prompts 一条写明只在笔记本入口、按入口声明能力、静态、契约 `prompts.json`；路径一条加虚拟的两个路径与四个保留的键；说明按模式与入口拼。
- **7.1**：迁移读别的模块的表（`source_hash` 的迁移的补哈希读 notebook 的 `notebooks` 与 asset 的 `asset_blobs`）是"只写自己的查询"的第一处经过审查的例外：只读、只在数据的迁移里，归属的检查不看数据的语句、由审查把关（`archtest/sqlc_test.go:113-122`），迁移的注释写明。
- **7.2**：notebook 的行写明 `mode`（`standard`、`llm_wiki`）与 `source_root_id`（不建外键）；page 的 `nodes` 加 `source_hash`，`page_revisions` 加 `rewrite`，`changesets` 加 `ingest_count` 与派生的表 `changeset_sources`；wiki 在 M10 没有表。
- **8.2**：wiki 的说明改为"模式契约的编排、lint、虚拟文件的内容、模板、playbook、升级流程（M11）"；写入的不变量（守卫）在 page。
- **8.3**：写入守卫的次序写成"（升级的冻结）→ 模式不变量 → 锁"。
- **9.2**：左栏加"健康"与"目录"。
- **10**：后端单元测试的"lint 规则、模式契约"写到 page 与 wiki 的 domain。
- **10.1**：对等验收一段的"哪些故事有 MCP 版本、哪些豁免与理由列在 M9 总设计第 9 节"改为"列在各 M 的第 9 节（M9 起）"（M10 的在它自己的第 9 节，主持者的决定第 16 条）；"什么也不写的故事（M8 的搜索 SE1、SE2）不断言数据库"加上 M10 的 LW8、LW11、LW13（审查 C4-M1）。
- **12.2** 的 M10 一行：五个 Phase、13 个 Step（P1 四个，P2、P4 各三个，P3 两个，P5 一个，网页四个 Step）；"必填属性"改为"必填属性的检查"。
- **12.3**："M10 依赖 M9（工具与 prompt 注册表）、M7（导出贡献者）"改为带着理由的四个：M6（lint 的数据与虚拟页的链接在链接索引里）、M7（导出贡献者、附件进树与链接、附件的 SHA-256）、M8（守卫照撤销与恢复判、后台的恢复与撤销以 `refused` 停下、历史的 ingest 标记与筛选）、M9（工具与 prompt 注册表、按路径的查找、`batch`）（同这份设计的头上一行，M9 也为它自己的依赖改过 12.3；审查 C4-N3）。
- **12.4**：写入守卫一行写明 M10 的注册者在 page、排在锁之前、M11 的冻结建议在最前；"编辑会话"一行写明模式的否决者与它的位置（在锁的否决者之前）；新加两行"笔记本的创建（事务内的播种者，交回它写下的）"与"笔记本模式的变化（订阅者）"（建立于 M10，注册者 M10 的模板与事件、M11）；`Seed` 不是扩展点，不进 12.4（只在 13.1 第 1 条写成例外）；"页面领域事件"一行写明链接索引的观察者判定 ingest；"阅读视图的交互增强"一行写明虚拟页的链接打开目录视图与历史；"实时推送的事件类型"的 M10 写明类型 `mode`；"导出贡献者"写明 M10 的贡献者与它的上限；"MCP 工具与 prompt 注册表"写明 prompt 的入口（`Entry`）与 `prompts.json`。
- **12.6**：M10 一行在这份设计定稿时是"进行中"、带这份设计的链接（同 M9 的设计）；收尾之后照 4.19 是"实现完成，待人工验收"（同 M9），完成标准照第 3 节。
- **14**：新的风险见第 10 节（守卫逐步判的偏严、`batch` 的去重比最后的正文、去重的派生列、lint 与 `index.md` 的代价）；页面树一行的"不达标时改为分层加载"加一句：来源区的判断（`isSource`）今天读整棵已加载的树，改为分层加载时要改为问服务端（4.16）。
- 随实现它们的 Phase 改（第 13 节）：
  - **13.1** 第 1 条：`TreeWrites().Seed` 是"单元拒绝已在事务里的 ctx"的唯一例外；`Write.Notebook`、`Write.Grants`（单元的权限的备忘）、`Step.Content`（含 `Merged`）、`Step.AreaHashes`、`Step.Rewrite`；模式的拒绝不论出自守卫还是预检都标出（`Refused`）；单元维护 `nodes.source_hash`，`DeleteNodes` 与 `DeleteNotebooksPages` 清掉它，降级的 `Forget` 是一处声明的不经单元的写（同删除的级联：派生的数据、没有变更集、在笔记本行的锁下）；导入的单元一段加保留的键与重复的页建成文件夹；"从回收站恢复"一段加加序号；"树写入端口"一段加 `Check` 的预检与流完之后的去重；"后台的恢复与撤销"一段加开始时的预检与 `refused`；整组撤销一段里临时名称的"守卫看得到（只凭名称认得，M10）"改为"守卫看得到（M10 的模式守卫照普通的改名、移动判它们，不凭名称认）"（审查 C4-N3）。第 2 条：定客户端的函数在 `shared`（`ClientOf`），page、asset、transfer、notebook 的适配器都用它；ingest 的标记。第 3 条：`notebook.change_mode`、`lint.read`、`page.manage_source`（守卫经授权者问它，不比较角色）。第 4 条：照每个操作的写法写出新操作与守卫的码的次序（守卫的 403 在领域的 422、409 之后；模式在锁之前；模式的码之间的次序；预检的位置；`batch` 的计划在编辑锁的预检之前；lint 的次序；REST 的 `listIndex` 照 `ListIndex` 今天的次序）。第 5 条：来源区的 advisory lock 的位置（只在 `FOR SHARE` 的单元里取，键空间）；"建笔记本只锁工作区行"改写：建 LLM Wiki 在笔记本的管理事务里锁笔记本行、走页的分支，模板的单元重入它们。第 6 条：`changeset_sources` 是派生的表，没有外键与登记的清理器，它的行由清理变更集之前的一个叶子的步按批删除、循环到一批不满、行数算进清理器的返回，两条清理变更集的语句加 `NOT EXISTS`。第 8 条：`duplicate_of`；prompt 的错误。第 9 条：`Seed` 取值里的时刻。第 10 条：lint、降级与 `prompts/get` 的日志。第 11 条：`page.NewModeGuard`、`page.NewSourceArea`、`page.NewIngestMarks`、`transfer.NewWritingJobs`、附件端口的 `SHA256`、`linking.NewIndex` 的签名（reindex 的组合也建它）、wiki 读的只凭连接池的端口、wiki 的模块根与组合根的次序。第 13 条：MCP 的契约加 `prompts.json`。第 16 条：`changeset_sources` 的授权。第 19 条：模板的正文在启动时解析。第 21 条：M10 注册的与最后一跳；两个新的扩展点；播种者交回它写下的；两个不只凭连接池构造的注册者（播种者要 `TreeWrites()`，模式的守卫要授权者）。第 23 条：lint 的快照的名额至少 1，取自请求的那一半（快照的那一半已由导出、导入与搜索用满，`platform/config/validate.go:251-271`），有自己的期限 `LintTimeout`。第 27 条：REST 的 `listIndex` 是一处例外（它是 MCP 的 `index` 的 REST 的一面，值的核对与资源无关，次序照 `ListIndex`）；`list` 的游标"子节点的次序值与 id、glob 的路径键"加"虚拟项的 `V`"（带 `V` 时 `O`、`I` 为零）。第 30 条：`mode`。第 31 条：4.18。第 32 条不改。
  - **13.2**：第 23 条（健康度与目录不是由事件驱动的视图，不进处理表；它们比答复里的 `mode` 与 `useNotebook().mode` 标出模式的改变，不要例外）；第 19 条（lint 的"更多"每页一次运行、按键去重）；第 12 条（"重新检查"的结果读出；添加来源的上传归 `AssetStore`）；第 7 条（面板、每张卡的"更多"与目录视图的加载与重试与它们的两个测试；按一次打开的键照搜索页只留显示着的那一个，`mutate(旧的键, undefined)`；lint 的 `server_busy` 照旧带 `Retry-After`，面板的 `onErrorRetry` 遇到它不自动重试（目录视图不答 503，没有它，审查 C4-N2）；来源区的标题对话框 409 时不自动重新提交）；第 15 条（M10 的键：lint 的摘要按一次打开、条目按规则与游标、目录按一次打开、类型、标签与游标、历史的 `ingest` 筛选在 `["changesets", …]` 里，都只在 SWR，没有新的按笔记本的 store）；第 16 条（`/health` 与 `/index` 在 `NotebookLayout` 之下、经 `useNotebook()`，不另读；来源区的标题对话框沿用 `useNewPage` 的两道防护；`refreshedOnConnect` 不加键）；第 1、10、14、17、22、26 条照 4.5、4.12、4.14、4.16 的用法写明（审查 C2-M2）。
  - **13.3**：第 6 条的"M10 的 lint 跳过它或即时解析"改为"跳过并计数"；第 3 条"判定之后、解析预算之内"为模板在启动时的解析写明例外。
  - **13.4**：第 1 条（新的检查的反例：`prompts.json`、模板的段落、`checkPages` 的新规则、`schema_test` 的新名称）；第 3 条（MCP 的客户端加 prompt 的方法；故事的 MCP 版本逐个判；数据库的断言按表放进 `notebook.ts`、`page.ts`、`transfer.ts`，不另开 `wiki.ts`）；第 4 条（4.17 的交错）；第 5 条（prompt 的列表的整个程序的测试）；第 6 条（矩阵的 LLM Wiki 的变体）；第 8、9 条（lint、`index.md`、去重、ingest、降级的守卫与计划测试，小的分析过的种子与另一本更大的笔记本，Makefile 的 `!race` 的行是新的写法）；第 10 条（`source_hash` 等于重建）；第 13 条（M10 的三处：虚拟页的链接的判定在 Go 与 TypeScript 里、对同一张样例表（4.9）；`jsTrim` 的空白在 SQL 与 Go 里，测试拿 SQL 的结果对着 `jsTrim`（4.10）；`CheckContract` 由计划、上传、撤销与恢复的预检与守卫共用，不另写一份（4.6）；审查 C4-N3）。
- **文档约定**（`docs/README.md` 的"Git 工作方式"）：合并的那一行（`:102`）不改，`:103` 已许每个 M 在 00 号设计里定它的写法；只在 `:103` 之后加一句："M10 照前三条（代码、文档、合并）的写法，只是在 Step 的分支上的提交（文档、合并之前的审查修复与它们的核对在内）与合并写到 Step（`[M10 总设计](v0.1/M10-llm-wiki/00-M10-design.md)`第 7 节）。"在记下设计被接受的那个文档提交里改（审查 C2-N1、C4-N1）。

## 8. 本 M 建立与注册的扩展点

建立（总体设计 12.4）：

- **笔记本的创建（事务内的播种者）**：notebook 的 `app/extension.go` 的值 `NotebookCreation`（笔记本、工作区、模式、来源根的 id、模板的语言、执行者、客户端、时刻）与播种者接口 `Seeder`（方法 `NotebookCreated`）：在插入笔记本与管理员之后、可见性之前同步调用，交回它写下的（来源根的 id，或空：模板交回的是 `Seed` 写下的那一页的 id）与错误，任何错误整个回滚；`CreateNotebook` 核对恰有一个播种者交回来源根、且等于先定下的 id，没有、两个或不同都是缺陷（500）；`notebook.New` 在没有播种者时 panic。模块根上以替身证明到达、次序、交回与回滚（`notebook/extension_test.go`）。注册者：wiki 的模板。
- **笔记本模式的变化（订阅者）**：值 `ModeChange`（笔记本、工作区、之前与之后的模式、执行者、时刻）与订阅者接口 `ModeSubscriber`（方法 `ModeChanged`），降级（与 M11 的状态变化）在写模式的事务里调用。模块根上同样有替身与两个注册者的次序的测试（13.1 第 21 条）。注册者：事件流的 `mode`。
- **页面的单元加入调用方的事务**：`(*page.Module).TreeWrites().Seed(ctx, SeedSpec{NotebookID, At, Client, Message, Pages})`，交回它写下的页的 id（执行者取 ctx 的；模板的正文在启动时经 `TreeWrites().ParseTemplate` 解析一次），只给模板；是 13.1 第 1 条的例外，不是给别的 M 的扩展点。

注册：

- **写入守卫与开启的否决者**（M4 建立）：`page.NewModeGuard(pool, authorizer)`（P2/S1已接来源事实与模式会话否决者；写guard与sessionVeto分别核对），排在锁之前（`pageRegistrants(pool, authorizer)`）。
- **导出贡献者**（M7 建立）：wiki 的虚拟文件。
- **实时推送的事件类型**（M5 建立）：`mode`。
- **MCP 工具与 prompt 注册表**（M9 建立）：wiki 的 `lint`、`pending_sources` 与第一批 prompt。

改前面 M 的扩展点与代码（12.1 第 6 条的例外，逐项写明）：

- access（M2）：规则表加 `page.manage_source`（笔记本管理员）。
- notebook（M3）：`CreateNotebook` 收模式与语言（`CheckDraft` 核对）、以 `llm_wiki` 直接插入、调用播种者并核对来源根；`Deps.Seeders` 与模式变化的订阅者，`notebook.New` 的三个新的 `Deps`（另有 `WritingJobs`：`transfer.NewWritingJobs`，没有它时 `New` panic）；HTTP 适配器经 `shared.ClientOf` 定客户端；`View` 与查询带两列；`LockNotebook`、`ShareNotebook` 选出两列；`notebook.NewNotebooks` 加 `StateOf`、`ShareState`、`LockState`（`ShareByID`、`LockByID` 不改）；MCP 的 `notebooks` 带 `mode`；新用例 `DowngradeNotebook`。
- asset（M7）：附件端口的 `AttachmentInfo`（`page/app/ports.go:558-569`）加 `SHA256`，asset 的适配器一并读出。
- page（M4–M9）：`Notebooks` 端口交回状态；`Write.Notebook`、`Write.Grants`；`Step.Content`（含 `Merged`）、`Step.AreaHashes`、`Step.Rewrite`；`WriterDeps` 加附件端口；`Unit.apply` 标出拒绝、维护 `source_hash`；`DeleteNodes` 与 `DeleteNotebooksPages` 清掉 `source_hash`；`Writer.Run` 之外的 `Seed`；单元交出父节点的一线；`batch` 的 `failed`、计划的两遍的预检与 `Merged`；撤销的 `revertDefect`、计划带 `rewrite`、核对的预检；`startRevert`、`startTrashRestore` 的开始的预检（经 transfer 调 page 的端口）；导入与恢复（同步与后台）在根下给保留的标题加序号、导入的单元把重复的页建成文件夹；上传的 `Check` 的预检；`RecordRevision` 合并 `rewrite`；路径的查找认出虚拟的两页（`FindPath`、`WalkPath` 与 `batch` 的模拟的树）；`list`（`ChildrenCursor.V`）、`read`、`open_notebook` 的虚拟项；`ToolDeps.Virtual`；说明的拼法（两处合成一处，收"放得下多少"）与缩短的内置的说明；喂 `log.md` 的变更集的端口；历史的读与筛选带 ingest（每种筛选一条语句）；清理变更集之前按批删除 `changeset_sources` 的行的一步与两条清理的 `NOT EXISTS`；新的端口 `page.NewSourceArea`（`Nodes`、`Forget`）、`page.NewIngestMarks`、`page.Refusal`。
- linking（M6、M9）：`linking.NewIndex` 的签名（13.1 第 11、21 条引了它；reindex 的组合也建它）；索引的观察者拿到 `ChangesetID`、`Kind`、`Notebook` 并判定 ingest；`ReplacePage`、`DeletePages` 交回旧行；lint 的查询（与 `page_properties_sources_idx` 的迁移）与它们给 wiki 的只凭连接池的端口；`ListIndex` 的按父节点 id 的筛选、`ExcludeArea`、"有类型"照 4.10；REST 的 `listIndex`；虚拟页的链接的判断与落点。
- transfer（M7、M8）：`unitFailure` 认出模式的拒绝、失败 `refused` 与报告的 `refusal`、问题 `duplicate_source`；`Deps.Contributors` 由组合根交 wiki 的；`transfer.NewWritingJobs`；`startRevert`、`startTrashRestore` 开始时的预检。
- 平台：mcpserver 的 `Prompt.Entry`、`newEntry` 按入口声明能力、`promptHandler` 的错误、参数的界与日志、答复的上限、`membersOf` 抄 `duplicate_of`；httpserver 的 `Problem` 加 `duplicate_of`。
- 契约：`api/mcp/tools.json` 里改的工具对象（第 5 节的表）与新的 `api/mcp/prompts.json`；`events.yaml` 的 `EventMode`。
- 组合根：wiki 的接线与构建次序；`linkIndex` 的转换带上三样；`treeError` 的 `refused`（`ErrLocked` 在前）；守卫的次序；`mcpTools` 拼 wiki 的工具，`Options.Prompts`。
- 网页：M3 的建笔记本的对话框与设置；M4 的树的行（图标、菜单、来源区的"新建页面"、拖进来源区的确认）；M5 的 `eventHandlers`（`refreshedOnConnect` 不变）；M5 的编辑会话的失去编辑的原因；M6 的解析不到的链接；M7 的附件一节（写者在来源上收起改名、移动、删除）与上传的行的重复的文字；M8 的历史的行、筛选与变更集页，回收站与撤销的对话框；`NotebookPlaces` 的"健康"与"目录"。

## 9. 测试策略

- **规则**：`page/domain` 的 `CheckContract` 的表格测试，覆盖 4.7–4.9 的每一格（操作 × 之前与之后在不在来源区 × 要不要 `page.manage_source` × 类型 × 标记 × 是不是编辑会话 × `Merged`，含管理员移出之后再写、不改字节与从空开始的形状）；wiki 的 `domain`（契约表、两个生成器含敌意的字符串、模板两种语言的对应与它的反例（少一段、末尾多一段与每种块的形状各一个）、虚拟页的链接的样例表）的表格测试；linking 的 `domain` 的待建页面的分组（4.11 的每一种边，含 `[[Node.js]]`、`![](x.png)`、`[[/a/x]]` 与 `[[a/x]]`、`target_key` 为 `NULL`、越过根）。 P2 另钉住来源 version/session 同字节拒绝与普通 no-op 对照；无整树历史、真实 compact/同组两方向/真实缩批与独立 oracle，来源管理员仍无普通正文例外。
- **整个程序的最后一跳**（13.1 第 21 条，组合根交空时失败）：
  - 守卫：`TestTheModeGuardReachesEveryPath`（照 `bootstrap/page_lock_test.go:8-20`）：第 3 节第 2 条的每条路径，每条一个会被拒的写，断言码与"什么都没写"；MCP 的写另有一个照 `TestTheEditLockReachesEveryMCPWrite`（`bootstrap/mcp_lock_test.go:11-39`），用 `AGENTS` 与来源的情形（P4 之后 `write index.md` 由路径的一层在单元之前答，证明不了守卫）。**有预检的路径**（`batch` 的计划、上传的 `Check`、撤销与恢复的核对）：预检是 page 自己调同一个 `CheckContract` 的代码，`pageRegistrants` 里 `guards: []` 时它们照样以同样的码拒绝，所以这些路径上一个预检拒得住的情形在空的组合里也会通过；每条这样的路径另有一个只有守卫拒绝的情形（单元里才发现的重复：`batch` 不去重、上传在流完之后、撤销与恢复的开始的探查不加锁，交错让重复在探查之后出现），P2/S2 补上（P1 已有 `batch` 的计划与单元之间的降级，P1 设计 3.24），守卫的最后一跳由此完整。P1 的子集写明（第 7 节）。**导入**在 M10 的规则下没有可拒的，它到达守卫在模块一级以假的守卫测（`TestAnImportsUnitsCreatePagesAndAttachments` 的做法，`page/app/import_test.go:52-105`），整个程序的测试只核对加序号与重复的文件夹。**次序**：`[mode, lock]` 由一个两个守卫都拒绝的情形钉住：别的账户的编辑会话持着 `AGENTS` 时删除它答 `page.protected`（`TestTheRegistrantsRunInTheirOrder` 用的是替身）。**后台**：两批之间执行者被降为写者（P1）与两批之间出现的重复（P2/S2），都由那一批的守卫拒绝，以 `refused` 停下、报告带码与节点、之前的批留着；碰到锁的后台撤销照旧是 `page_locked`。 P2 分清四层：请求拒且零 enqueue；worker Plan 拒且零 RunBatch；真实 Plan 成功后首 RunBatch guard 拒；真实前批非零 commit 后后批 guard 拒。保留生产预检，以只插 barrier/计数的 decorator 证明后两层；不能用前两层杀 guard 变异。同步锁下预检与 guard-only 装配亦分别标识。
  - 创建的播种者：经 REST 建 LLM Wiki 的笔记本，两页、变更集（同一时刻、请求的客户端）、索引都在（`pages` 的 `NOTIFY` 照样发出、流不转发它，4.4，没有测试等它）；测试库上一个在插入 `AGENTS` 时报错的触发器（13.4 用过测试的触发器）让创建失败，之后没有笔记本、成员、节点与变更集。
  - 模式变化与事件：经 REST 降级，另一条流收到 `mode` 的帧（按契约核对）；模块根上两个注册者的次序。
  - 贡献者：经 serve 整本导出一本 `llm_wiki` 的笔记本，zip 与 `meta.json` 的 `contributed`（`metaPaths` 读它），重新导入跳过；普通笔记本的档案的测试照旧作对照。
  - `ToolDeps.Virtual`：`page.New` 在它为空时 panic（同 `TestTheModuleRefusesNoAttachmentInfos`），再经 serve 的 `read index.md`。
  - prompt：笔记本入口的 `prompts/list`、`prompts/get` 到达处理函数；账户入口没有 `prompts` 的能力、列表为空、`get` 答"不认识"。
  - ingest：经 REST 与 MCP 的写之后 `changeset_sources` 的行。
  - wiki 的工具：照 M9 的工具清单、矩阵与码的表。
- **派生数据**：`source_hash` 等于重建（4.8 的步的种类各有最少的次数；降级之后的写与恢复在内）；`checkPages` 加 `source_hash` 的不变式、"活着的 `llm_wiki` 有来源根与 `AGENTS`"、`changeset_sources` 的三条（4.14），每条一个 SQL 播下的反例（13.4 第 1 条）；`batch` 进这个测试。
- **交错**：4.17 的各个，结束时核对不变式。
- **清理**：一本删除了的 LLM Wiki 笔记本，它的变更集带着很多来源（一个变更集 2,500 个，与 500 个变更集各 3 个），在一次清理的运行里删完，同一次运行里 `PurgeNotebooks` 成功（4.2）。
- **成本与计划**（13.4 第 8、9 条，`!race`，界线照 13.4：本机实测的最快的几次的三倍，与目标的数（如 4.11 的 2 秒）不同时以 13.4 的为准；各在 `test-go-costs` 或 `test-go-guards` 的一行，行照 M9/P2 6.5 的规则定。今天的计划测试不带标记、在竞态的一份里跑，这些行是新的写法）：
  - lint 每条规则与摘要在 10 万页上（含内存的界，含 4.10 的长空白的行）；
  - `index.md` 的一次 `read` 在 10 万页（含入链数最坏的形状）；贡献者在 10 万页与 1,000 个变更集；
  - 去重：一次新增在 10 万个来源；一棵上限子树移进 `raw`（另有普通10001成功与超界有限物化、成功末层空父探查）；恢复与撤销在 `MaxUnitNodes`、`MaxUnitPages` 而每个节点都在来源区里；60k任务完整计划、X跨10k与大量同hash收敛、降级 Forget 在10万来源；按4.8独立成本行；600万空/已有LLM实际冷事务与三/四列成本差按4.2；
  - ingest 的判定在 `batch` 的界上（很多页 × 每页很多新的来源链接），在 `TestABatchAtItsRewriteBoundTakesItsTime` 旁边；
  - 计划测试在小的、分析过的种子上，照 M9 的三个阶段（没有统计、分析过、通用的计划），种子里另有一本更大的笔记本：lint 的经 `postgres.Planned` 的语句（4.11 的清单，来源区的数组以真实的大小、定制与通用的计划）；`nodes_notebook_id_kind_source_hash_idx` 的点查/有界游标；有限 reader 在无/有统计×custom/generic 的真实计划/范围/缓冲，巨大宽层、多父、另一笔记本 A→B→A 与反向对照（P2 §3.8）；`TestTheLogReadsUseTheirIndexes` 加 ingest 的筛选与 `changeset_sources` 的"前 N 个"；一页的日志带 ingest 在 `TestTheLogReadsInTimeAsLongAsAPage` 之内。
- **上限**：4.18 的每一处有测试。
- **反向对照**（13.4 第 1 条）：`prompts.json` 的检查（少一个、多一个、说明不同）；`TestTheBuiltInInstructionsNameEveryTool` 对新的两个工具；`TestConstraintAndIndexNames`（`server/migrations/schema_test.go:163`）的表加 `notebooks_mode_check`、`notebooks_source_root_check`、`nodes_source_hash_check`、`changesets_ingest_count_check`、两个迁移的部分索引与 `changeset_sources_pkey`、`changeset_sources_changeset_id_node_id_key`；反例：模式 `'upgrading'` 与 `'LLM_WIKI'`、`llm_wiki` 而来源根为 `NULL`、31 与 33 字节的哈希、`ingest_count = -1`；模板的段落少一段、末尾多一段与每种块的形状；`checkPages` 的每条新规则。
- **权限矩阵**：新操作的行；`{rule}` 的一段；LLM Wiki 的变体（4.17：它自己的工作区里的种子与跑在它上面的行）；MCP 的矩阵加两个工具与第 5 节的码的行。
- **日志**：降级、lint、`prompts/get` 各有日志的测试（只有 id，不带名称、`source`、`question`）。
- **测试的名字与分份**：M10/P2/S2按M9/P2§6.5两次CI与邻居容量规则，从三份增为四份；首次四片保留FIRST、另取SECOND的历史结果不改；其后33b4/4550两分支第一片Test均值450.5s再次触发M9/P2§6.5，按实施决定44选择窄名称移界。当前FIRST为`^Test([A-D]|E([a-u]|very([A-Q]|ResolutionCase[A-H])))`，SECOND为`^Test(Every(ResolutionCase[I-Z]|[S-Z])|Source|The(MCP|Contents))`；第三份仍为THIRD减SECOND，第四份为三者并的补集。重排测量时的468顶层仅Imports由第一片移第二片，分配167/72/114/115；八次完整本机运行、candidate02仅注释桥接及独立复核已接受，提交`fb92974b`。其后fb精确CI因Revert10k失败；最新有界facts保留输入的清单、完整双轮及新代码/文档两分支CI与main双样本按S2审查实际回执核验，预测不当CI实测。`TestSource…`在第二份，`TestTheSource…`在第三份，其余新名按Makefile实际表达式落位，不再把所有未匹配FIRST/THIRD的名称称第二份。每份独立两轮与两次main Test按实际新头核验，见[P2结果](02-P2-source-area.md#102-s2-来源-hash去重与验证)及[S2审查](reviews/P2-S2-source-hash-review.md#最终验证与集成)；历史M9名字建议和旧三片时间保留原阶段。e2e仍不分份，不以增加bootstrap份数替代产品失败修复。
- **e2e**（13.4 第 3 条）：`e2e/fixtures/notebooks.ts` 的 `createNotebook` 今天是按位置的参数（`:9-26`），加一个选项的对象（`mode`、`templateLocale`）；`fixtures/mcp.ts` 加 `listPrompts`、`getPrompt`（交回 `{result} | {error}`），`PageRead.meta` 的 `version` 可选；数据库的断言按表（13.4 第 3 条）：`assert/notebook.ts` 的 `expectNewNotebook`、`expectNotebook` 读 `mode` 与 `source_root_id`，`assert/page.ts` 加 `source_hash`、`changeset_sources`，`expectVersions` 加 `rewrite`，`assert/transfer.ts` 的 `expectBackgroundJob` 收停下的任务与 `refusal`；不另开 `assert/wiki.ts`（wiki 没有表）。`playwright.config.ts` 设 `locale: "en-US"`，所以 LW1 的中文界面用 `test.use({ locale: "zh-CN" })`。LW12 的"有导入在跑时被拒"不用 `holdNotebookRow`（`e2e/fixtures/transfer.ts:191-198` 以 `FOR SHARE` 持行，降级的 `FOR NO KEY UPDATE` 排在它后面，放开之后谁先都可能，13.4 第 4 条的"被放开的行不排队"，故事会时好时坏）：持另一本笔记本的行、在那里开始一次导入，`jobs.import_workers = 1` 时它占着唯一的工人，LLM Wiki 的导入停在 `queued`，降级立刻答 `transfer.busy`。

**本 M 的故事**（`e2e/stories/wiki/lw<n>-<slug>.spec.ts`；"页面"有网页一侧的动作；MCP 版本照 M9 的约定在故事自己的文件里、名为"`<故事> (MCP)`"，种下的数据经 REST，每个 agent 一个 PAT，值来自答复。照 10.1 的规则逐个故事判：读或写页的故事有 MCP 版本，豁免的写出它的 Go 测试，同 M9；每个版本写明落在哪个 Step）：

| 故事 | 内容 | 版本与 Step |
|---|---|---|
| LW1 新建 LLM Wiki 笔记本 | 对话框选 LLM Wiki（中文界面）：根下 `raw` 与 `AGENTS` 是中文的模板，与笔记本同一个变更集与时刻，`source_root_id` 是 `raw`；英文界面建的是英文的；普通笔记本没有它们；MCP：笔记本入口握手的说明是 `AGENTS` 的正文（P1），加系统的一段（P5） | API、MCP（握手的 `AGENTS` 正文）P1/S1；页面 P1/S4；P5 给 MCP 版本加系统的一段 |
| LW2 保护与保留的标题 | `AGENTS` 删除、改名、移动被拒（`page.protected`），正文可改；`raw` 不能删除，写者不能改名、管理员能（来源根的管理员规则在 P1/S2）；根下建 `index` 页被拒（`page.virtual_file`）；MCP：`delete AGENTS.md`、`move AGENTS.md`、根下的 `write index/x.md` | API、MCP P1/S2；页面 P1/S4 |
| LW3 添加来源 | 粘贴为来源一次写成；上传一个 PDF 为来源；同样的字节再加一次被拒并指出已有的路径；MCP：`write raw/x.md`，重复被拒 | API、MCP P2/S2；页面 P2/S3 |
| LW4 来源只读 | 来源页没有编辑与勾选、阅读视图有说明；PUT、MCP 的 `edit` 与 `write` 覆盖、还原版本都被拒；来源根的说明可以改；改名一页 wiki 页时来源里指向它的链接被系统改写 | API、MCP P2/S1；页面 P2/S3 |
| LW5 管理员管理来源 | 写者改名、移动、删除来源被拒（`page.source_admin_only`），管理员可以；写者把一页 wiki 页移进 `raw`（页面上先确认）成为来源，与已有来源相同时被拒（S2）；来源区里"新建页面"先问标题 | API、MCP P2/S1（重复一步 P2/S2）；页面 P2/S3 |
| LW6 历史与来源 | 写者撤销管理员删除来源的变更集（恢复，新增）；写者撤销管理员给来源改名的变更集被拒，管理员撤销成功、写回被改写过的页；回收站恢复一个与活着的来源相同的来源被拒；后台撤销以 `refused` 停下、报告说明 | API P2/S2；页面 P2/S3。MCP 豁免：agent 只读历史（M9 第 11 节第 1 条，MCP 没有撤销与恢复）；它在 MCP 上看得到的效果由 `TestTheModeGuardReachesEveryPath` 的同步与后台的撤销、恢复的情形与 M9 的 `log`、`read` 的测试盖住 |
| LW7 导入进 LLM Wiki | 导入一个库到根下：库里的 `index.md` 成为 `index 2`；导入到 `raw` 下：重复的文件跳过、重复的页成为空的文件夹而它下面的页照常落下，报告 `duplicate_source` | API P2/S2；页面 P2/S3。MCP 豁免：v0.1 的 MCP 不开后台任务（M9 第 9 节）；导入的单元的加序号与重复由 transfer 与 page 的导入的 Go 测试盖住 |
| LW8 健康度 | 两种模式的面板：待建、孤儿、解析失败；LLM Wiki 另有缺类型、缺摘要、未消化、无效的 `sources`；读者看得到；"更多"翻页；MCP：`lint` 的摘要与一条规则的条目 | API、MCP P3/S1；页面 P3/S2 |
| LW9 未消化的来源与 ingest | `pending_sources` 列出新来源（空的文件夹页不在其中）；一个引用它的 `batch` 被标为 ingest、记下它，历史里有标记与筛选，之后它不再未消化；再引用它的编辑不是 ingest。API 版本只能以 PUT 引用它 | API、MCP P4/S2；页面 P4/S3 |
| LW10 虚拟的 index 与 log | 树里两行：`index` 打开目录视图、`log` 去历史；MCP：`list` 列出、`read index.md` 与 `log.md`、写被拒；整本导出带两个文件，再导入不多出页 | API、MCP P4/S1；页面 P4/S3 |
| LW11 playbook | 笔记本入口的 `prompts/list` 列出三个；`prompts/get ingest` 带来源的路径；普通笔记本里答 `notebook.not_llm_wiki` | 只有 MCP（只能经 MCP），P5 |
| LW12 降级 | 管理员确认降级：另一个标签页随事件更新、`raw` 可以删除、`AGENTS` 可以删除（P1）；实际打开的一页来源在另一个标签页不 reload 成为可编辑的、之后重复不再被拒，保留同连接证明（P2 加）；虚拟的两行消失（P4 加）；有导入排着队时被拒；写者看不到降级 | 页面、API P1/S4，P2/S3、P4/S3 各加一步。MCP 豁免：MCP 没有降级（网页上管理员的动作）；它在 MCP 上看得到的效果（`list` 没有虚拟的两项、`delete AGENTS.md` 成功、握手没有系统的一段）由 `TestTheDowngradeEndsTheContract` 盖住 |
| LW13 目录视图 | 普通笔记本的目录（从 `NotebookPlaces` 进）按类型分组、按标签筛选、翻页；MCP：`index` 与 REST 的 `listIndex` 是同一份（项与次序相同），证明两者共用 `ListIndex` | API、MCP P4/S1；页面 P4/S3 |

- LW8、LW11、LW13 不写页，照 10.1 的例外"什么也不写的故事……不断言数据库"（同 M8 的 SE1、SE2；10.1 照第 7 节"总体设计的修订"加上这三个，审查 C4-M1）。
- 前面 M 的故事都在普通笔记本里，照旧通过；它们的 MCP 版本不变（M9/P5）。MC1 的 `tools/list` 照 `tools.json` 读，不写死，加两个工具不改它。

## 10. 风险

| 风险 | 缓解 |
|---|---|
| 守卫逐步判比净效果严，至少三种形状（4.7）：先放回祖先再临时改名它之下的节点；移入来源区之后的写回（谁也撤销不了）；同一轮里先放回、后删除时的去重 | 只多拒、不多放（4.7 写明理由）；这些撤销要同一个变更集既移出来源区又在里面写或交换；P2 的测试钉住每一种的结论；对话框/报告按4.7给区外还原再移回的步骤，重新移回仍须去重 |
| `batch` 里去重比的是一页在这个 `batch` 里最后的正文：管理员在同一个 `batch` 里移进、移出再写一页时与分开调用不同（两个方向都有，4.7） | 只在管理员移出时发生（写者移不出来）；反方向放过的那一种结束时来源区里没有重复；写进 MCP 的 `batch` 的工具说明；正文的两条规则由计划按项判（字节不变的写不算），`Merged` 是后备（只数页是来源时改了字节的项），所以写一次的规则与分开调用相同；只有一种要管理员的形状被后备多拒：把页移出、清空、移回、再写 |
| 后台 compact、同组批内合并与运行时缩批使来源结果依赖实际顺序；同步与后台可不同，即使没有并发也可能预检后拒绝 | 决定43：开始/worker按生产未缩减名义批预检，名义拒不搜索更细分法；保留原调度/Joined/halving，前批保留、失败批回滚。真实 API 两组反例、同组两方向、真实资源界缩批及四层拒绝分别验收；产品提示不称只因并发（P2 §3.13）。 |
| `source_hash` 是派生的列，维护分散在单元的每种步 | 维护在 `Unit.apply` 的一处，按这一步之后的来源区算；`DeleteNodes` 与 `DeleteNotebooksPages` 各在同一条语句里清掉；降级的 `Forget` 清掉整本；迁移补上已有的；不变式按模式与生死写，`checkPages` 与"等于重建"的性质测试（含撤销、恢复、降级之后）；只有 LLM Wiki 的单元碰它 |
| 迁移在大的实例上停住服务：`nodes` 的 `source_hash` 与 `changesets` 的 ingest 持 ACCESS EXCLUSIVE，读与写都等；linking 只建索引的取 SHARE，写等、读不等 | 升级说明先停服务再 `migrate up`（同 M9），所以是停机的时长；P2/S2 的 source_hash 按4.2测600万无LLM/已有LLM完整冷启动 goose 事务；分别冷却的扫描诊断单列，不能相加冒充停机；P3/P4自己的扫描义务保留，真实结果写进升级说明；每个迁移的 Down 写明之前的程序看到什么 |
| lint 在 30 万页上一次运行秒级，快照占一个连接，谁都能开始 | 每条规则先量、只给超出预算的加部分索引；数目数到 10,000；不看来源里写的内容；面板打开时算一次、不随事件重算；进程内的名额（至少 1）、自己的期限与同一本笔记本的合并，超出或到期时 `server_busy`；成本守卫与计划测试 |
| `index.md` 每次 `read` 读整本的索引、树与来源区（10 万页约 1.7 秒） | 写到 40 KiB 为止，末尾指向 `index` 工具；成本守卫含最坏的形状；导出不带入链数；M12 的性能移交 |
| 写者把一页移进来源区之后要管理员才能移出或撤销 | 负责人的决定 1 的直接结果，主持者照准；网页在写者拖进来源区之前确认一次（"之后只有管理员能移出"，4.16）；负责人可以改判（第 11 节设计的取舍第 8 条） |
| ingest 的判定对这个变更集没写的页按单元之后的树与解析看 | 写了与删除了的页按旧行确切判（4.14）；剩下的只在一个变更集同时改树与写正文时差一个标签；不影响 lint 与任何规则 |
| 模板的单元加入建笔记本的事务，是写入单元的第一个例外 | 只经 `Seed`、只给模板；同一事务里的重入；播种者交回写下的来源根，没有时回滚；测试库的触发器注入失败的整个程序测试 |
| 去重比现在的字节：来源被链接改写之后，再加原来的那份不算重复；改写让两个来源相同时，同时含有它们的回收站的组恢复不了 | 链接改写碰到来源很少（来源少有指向 wiki 页的链接）；与负责人"字节完全相同"的说法一致（主持者照准）；写明；改判为"比加入时的字节"要另存一份哈希 |
| 静态的注册表：普通笔记本里也列出 `pending_sources` 与三个 prompt | 说明写"LLM Wiki only"，调用答 `notebook.not_llm_wiki` 与清楚的文字（主持者照准）；按笔记本过滤要在 `listChanged: false` 下让同一个入口的列表随笔记本变，留给以后 |
| 内置的说明离 4 KiB 只剩 69 字节，M11 还要两行 | P3 先缩短共用的行，加上两行之后每个入口至少剩 300 字节；讲虚拟页的字放进 `tools.json`；测试守 4 KiB；M11 的移交写下剩下的字节；LLM Wiki 的系统段不在 4 KiB 的内置文字里 |
| 网页的部分多（四个网页的 Step），改的接口多（约 22 个 REST 操作、每个入口约 14 个工具） | 每个服务端的 Step 带它加的文字、类型表与假服务端的字段，合并时全绿；每个故事落在写明的 Step；可挪进打磨的几项在第 7 节列出，由负责人在 P4 超出时定 |
| 10 万页一次摘要的 2 秒是量之前的目标 | 守卫的界照 13.4（本机最快的几次的三倍），两者不同时以 13.4 的为准；P3 量过之后写下 |
| agent 不照 playbook 做 | playbook 只是建议，来源区、保护与保留的标题由守卫强制；真实客户端的人工核对在 v0.1 的最后（4.19） |

## 11. 负责人确认的决定

负责人 2026-10-24 定下四件事：

1. **来源区：一个来源写一次，之后只读**。网页上新的来源来自"粘贴或上传为来源"的对话框，一次写成；打开已有的来源是只读的视图。agent 以一次 `write` 写一个来源。只有笔记本的管理员能改名、移动、删除来源。与来源区里已有的一个逐字节相同的新来源被拒绝，并指出已有的在哪里（4.7、4.8）。
2. **必填属性：检查，从不强制**。缺 `type` 或 `summary` 的页在健康度面板里显示为需要注意，MCP 的 `lint` 报出来；没有写入因此被拒：网页新建的页、agent 自动补建的父页、导入与恢复都照今天。M11 的升级提交以它为闸，那是 M11 的事（4.10，"写给后面的 M"）。
3. **语言**：模板（根下的 `AGENTS` 页与 `raw` 的说明）用建的人的界面语言（zh-CN 或 en）写，之后由用户改写；三份 playbook（MCP 的 prompt：ingest、query、lint）只有英文，同工具的说明（4.4、4.15）。
4. **健康度面板**：能读这本笔记本的人都看得到；普通与 LLM Wiki 的笔记本都有：通用规则两种模式都有，LLM Wiki 加它自己的；打开时算，每条规则显示数目与前几项、可以翻页；写入时不维护计数（4.11、4.12）。

**设计里的取舍**（负责人可以改判；设计审查之后照主持者的决定改过的标出）：

1. 分工：wiki 是新模块，管契约表、模板、playbook、lint 的编排与工具、虚拟文件的内容与导出贡献者，M10 里没有表；写入的不变量（守卫、来源的哈希、保留的标题）在 page；来源区的管理员权限是 access 的规则表的 `page.manage_source`（主持者）；ingest 的判定在 linking 的观察者、记在 page 的变更集上（4.1）。
2. 模式只有 `standard`、`llm_wiki` 两个值；升级的状态是 M11 自己的，不是模式（4.3）。
3. 来源根是笔记本行上不建外键的一列，M10 不能改指；降级时清掉它与来源的哈希（4.3、4.5）。
4. 模板与建笔记本在一个事务里：笔记本以 `llm_wiki` 与先定下的来源根直接插入（主持者），恰有一个创建的播种者交回它写下的来源根，页面的单元在这一处加入调用方的事务（`Seed`，声明的例外）（4.4）。
5. M10 里空的普通笔记本不能直接切到 LLM Wiki，留给 M11 的升级（4.5）。
6. 降级是专门的动作，有写入任务时答 `transfer.busy`，不写审计行；确认是普通的对话框，不要输入名称（主持者）（4.5）。
7. 模式的守卫排在编辑锁之前：长久的拒绝先答（4.6）。
8. 来源区的规则：新建、上传、移入、导入、恢复进来源区算新增（写者）；来源区里的每一种树操作（改名、移动、排序、删除）与来源根的改名、移动要管理员；写者移进来源区的页要管理员才能移出或撤销那次移动，网页在写者拖进去之前确认（主持者照准）（4.7、4.16）。
9. 撤销写回来源页只在那一版是系统的链接改写时放行（`page_revisions.rewrite`）；还原来源的版本一律拒绝；空的来源可以一次性地写一次第一次正文，编辑会话的保存不行（4.7）。
10. 去重：活着的来源，页按字节、附件按 SHA-256，不跨种类，比现在的字节（主持者照准）；哈希记在来源的节点上（`nodes.source_hash`），删除时清掉，不给 `page_contents`、`asset_blobs` 建全表的哈希索引；并发的第一次正文以按笔记本的 advisory lock 串行，只有 `FOR SHARE` 的单元取它（4.8）。
11. 导入进来源区时重复的附件跳过、重复的页在库里下面有东西时建成空的文件夹、没有时跳过，都写进报告，不让任务停下（4.8）。
12. 守卫逐步判，撤销里临时名称的步照普通的改名、移动判，不加标记；比净效果严的三种形状写明（4.7）。
13. 根下的 `index`、`log`、`index.md`、`log.md` 对页与附件都保留；导入与恢复给它们加序号（4.9）。
14. "缺"是没有这个键、值不是字符串、空或只有 `jsTrim` 的空白；frontmatter 写坏的页只算解析失败（4.10）。
15. lint 读时算、一个快照；每条规则数到 10,000；条目分页、游标带规则；过时的页跳过并计数；同时的运行有名额（至少 1，取自请求的那一半）与自己的期限（4.11）。
16. 待建页面按写的目标分组、按链接它的页数；读作附件的（照代码的定义）、解析不到的带扩展名的图片与嵌入、虚拟页的链接不算；"有歧义"保留按 id 选了一页的含义（4.11）。
17. lint 不看来源里写的内容；消化只算来源区以外的 wiki 页（不含来源根与 `AGENTS`，虚拟页不算），区里的附件跟着引用它的来源页，0 字节的来源页不待消化；文件夹页不算孤儿；不加引号的 `sources: [[x]]` 报在"`sources` 指向无效"里（主持者）（4.11）。
18. 虚拟文件：`list` 与 glob 列出，`grep` 不搜；两个文件写到 40 KiB 为止；`index.md` 只列 wiki 的页、来源区一行（主持者）；只在整本导出时写成文件（4.13）。
19. ingest：只看 `edit` 的变更集里写了正文的、来源区以外的 wiki 页；"原本未消化"对这个变更集写了或删除了的页按旧行确切判（主持者），别的页按单元之后的树与解析；编辑会话的标记只增不减；一个变更集至多记 10,000 个；导入、还原、恢复与撤销不是 ingest；每个读者看得到（4.14）。
20. 注册表是静态的：`pending_sources` 与三个 prompt 在每本笔记本里列出、普通笔记本里调用答 `notebook.not_llm_wiki`（一个码，主持者）；prompt 只在笔记本入口；prompt 的契约是另一个文件 `prompts.json`（4.15）。
21. 目录视图两种模式都有，普通笔记本的"目录"入口留着（主持者）；按 5.5 分组（4.16）。
22. 来源页的编辑会话由否决者拒绝；来源区里"新建页面"先问标题（主持者）（4.7、4.16）。
23. `batch` 里正文的两条规则由计划按项判（计划是权威），`Merged`（只数这一页是来源时改了它的字节的正文操作）是守卫的后备，计划不去重（4.6、4.7）。
24. 提交说明照文档约定的写法，只是 Step 的分支上的每个提交与它的合并写到 Step（第 7 节；修订核对第三轮之后照 C4-N1 写全三处）。
25. 第 9 节逐个故事判 MCP 版本，豁免的写出它的 Go 测试（主持者）。

**主持者在设计审查之后定下的**（[审查记录](reviews/M10-design-review.md)"主持者的决定"；负责人可以改判）：

1. A-Q1：笔记本以 `llm_wiki` 与先定下的来源根直接插入，守卫检查模板的单元，建的事务里模式不改变（4.4）。
2. B-Q1：lint 不看来源里写的内容（来源发出的链接不产生待建与歧义、不让 wiki 的页不算孤儿，来源的 frontmatter 不查），来源只由"未消化"一条判定（4.11）。
3. B-Q2 与 `AGENTS`：只有来源区以外的页消化来源，来源根与 `AGENTS` 都不算，虚拟页从不算（4.11）。
4. B-Q3：区里的附件，有一页来源页链接或嵌入它时，跟着那一页消化（4.11）。
5. B-Q4：`index.md` 与目录视图只列 wiki 的页，来源区写成一行（路径、来源的数目、指向 `pending_sources`）；修订核对之后照 B2-M2 不写待消化的数目（4.13、4.16）。
6. B-Q5：有子节点而正文为空的页（文件夹页）从不是孤儿（4.11）。
7. B-Q6：00023 与 00047 的行里认得出不加引号的 `sources: [[x]]`（`value` 是数组里的数组），不提升提取器，所以报在"`sources` 指向无效"里；修订核对之后照 B2-M3 加 `page_properties_sources_idx`，`sources` 键下每一项不是属性链接的（数组、对象、普通字符串）都报（4.2、4.11）。
8. B-Q7：把一个 LLM Wiki 的导出重新导入新的 LLM Wiki 得到 `raw 2` 与 `AGENTS 2`，是 v0.1 写明的限制，写给 M11（4.9、第 7 节）。
9. C-Q1：一个码 `notebook.not_llm_wiki`：这个模式没有的规则或工具答 409，不认识的规则答 404（`wiki.lint_rule_not_found`）（4.11、4.15、第 5 节）。
10. C-Q2：新的操作算进 12.1 的"约 20 个"，改的不算、但照它们定 Phase 的大小；M10 仍是一个 M（第 5、7 节）。
11. C-Q3：来源区的管理员权限是规则表的动作 `page.manage_source`（笔记本管理员），守卫问授权者，权限矩阵盖住它（4.1、4.6、4.17）。
12. C-Q4：降级用普通的确认对话框，写明改变什么，不用 `typedConfirmation`：什么也不删除（4.5）。
13. C-Q5：P1 分出 S3（预检与加序号）；后台的拒绝的去处初定挪到 P2/S1，修订核对之后照 C2-M4 放在 P1/S3；目录视图保留 5.5 的分组，按父页的筛选可以进 M12 的打磨（第 7 节）。
14. C-Q6：附件本就在正文之下列出，去掉 4.16 的那一项（4.16）。
15. C-Q7 = A-M10：负责人的规则照字面，来源区里的每一种树操作都要管理员；所以来源区里的"新建页面"先问标题、不建 Untitled；写者拖进来源区之前确认，写明只有管理员能移出（4.7、4.16）。
16. C-Q8：照 10.1 的规则逐个故事判：读或写页的故事有 MCP 版本，豁免的写出它的 Go 测试（第 9 节）。
17. 作者的疑问：管理员才能撤回写者移进来源区的页，照准，加上 15 的确认（4.7）。
18. 作者的疑问：去重比来源现在的字节，`source_hash` 跟着来源的每一次正文的写（链接改写在内）；改写让两个来源相同是允许的（4.8）。
19. 作者的疑问：ingest 的近似照 B-M1 改为对写了与删除了的页确切判（4.14）。
20. 作者的疑问：lint 的代价：待建页面不含来源发出的链接；保留 10,000 的上限；`index.md` 的项数在 P4 量、取 40 KiB 放得下的（4.11、4.13）。
21. 作者的疑问：没给 `template_locale` 时是 `en`，网页总是带上（4.4）。
22. 作者的疑问：挪进打磨的：图标再多一组可以；普通笔记本的"目录"入口留着（第 7 节）。
23. 作者的疑问：静态的注册表照准，`pending_sources` 与 prompt 在每本笔记本里列出、以清楚的文字答 `notebook.not_llm_wiki`（4.15）。

**主持者在修订核对第一轮之后定下的**（[审查记录](reviews/M10-design-review.md)"修订核对第一轮"；负责人可以改判）：

24. B2-I1 = C2-I1：虚拟项的位置是游标自己的成员 `V`（`omitempty`，M9 的游标写法不变），先比它；根下有负的排序的子节点跨虚拟项翻页的测试（4.13）。
25. A2-M1：`Merged` 只数计划的树在那一项时把这一页放在来源区里的正文操作，大于 1 才拒绝；管理员移出之后再写的两种形状有测试、都通过；修订核对第二轮之后照 A3-M1、A3-N1 再收窄：只数这一页是来源（来源根的严格后代）时改了它的字节的操作，新建的页从 0 字节算起，计划的正文的一遍把字节不变的写当作没有写（4.6）。
26. A2-M2：`DeleteNotebooksPages` 的 UPDATE 也清掉 `source_hash`；`Forget` 加 `AND deleted_at IS NULL`、走部分索引（4.5、4.8）。
27. B2-M1：lint 的交错照导出的 `holdAssetBlobs`，种下一个未消化的附件来源（4.17）。
28. B2-M2：`ListIndex` 加一个去掉来源区的页的选项，`index.md`、目录视图与 REST 的 `listIndex` 用它，MCP 的 `index` 照 M9 列每一页；来源区的一行只写路径与来源的数目，指向 `pending_sources` 与 `lint`；导出贡献者在导出的快照里直接调它要的读，从不经 `Lint`（4.13、4.16、第 5 节）。
29. B2-M3：`page_properties` 存着每个属性的整个值，所以加 00047 样式的 `page_properties_sources_idx`，`sources` 键下每一项不是属性链接的都报（4.2、4.11）。
30. B2-M4：lint 至少有 1 个名额，取自请求的那一半；它自己的期限，到期答 `server_busy`；合成的那一次不随第一个请求取消（4.11、4.18）。
31. B2-M5：清理变更集之前的一个叶子的步，`LIMIT` 一批，两条清理加 `NOT EXISTS`；引的行号改正；不变式写"存在的变更集"（4.2、4.14）。
32. C2-M1 = B2-N7：`getLint` 与 `listIndex` 答出算时的模式，视图拿它比 `useNotebook().mode`（4.5、4.12、4.16）。
33. C2-M3：保留 `ListIndex` 的次序，REST 的 `listIndex` 声明为 13.1 第 27 条的例外（第 5 节）。
34. C2-M4：后台的拒绝的去处放在 P1/S3，以 C 的被降级的管理员为它的测试（4.6、第 7 节）。

**主持者在修订核对第二轮之后定下的**（[审查记录](reviews/M10-design-review.md)"修订核对第二轮"；负责人可以改判）：

35. A3-M1、A3-N1：`Merged` 只数这一页是来源（来源根的严格后代）时改了它的字节的正文操作，新建的页从 0 字节算起；计划的正文的一遍把字节不变的写当作没有写；剩下的一种要管理员的误拒写进第 10 节；A 的两种通过、A-I1 的两种照样拒绝（4.6、4.7、第 10 节；第 25 条随之改）。
36. B3-M1：清理 `changeset_sources` 的叶子的步照 `PurgeNodes` 循环到一批不满，行数算进清理器的返回，每条语句一批；清理变更集的语句以 `SKIP LOCKED` 取行；`ingest_count` 的不变式只对清理的谓词不取的变更集；删除了的笔记本在一次运行里删完、`PurgeNotebooks` 同一次成功的测试（4.2、4.14、第 9 节）。
37. B3-M2：REST 与 MCP 的 lint 在快照的第一条语句里读模式与来源根，所以答出的 `mode` 是快照的；路径一次读、在附件的信息之后；交错改名一页来源区以外的孤儿 wiki 页；`WithinSnapshot` 之外的 `Lint` 让测试失败的变异的核对；降级的那一半照此改写（4.11、4.17）；修订核对第三轮之后照 B4-N1、B4-N2 写明只有改名的那一半是变异的核对、两半各经哪个入口，照 B4-N3 运行之内的模式的核对答同一个 409。
38. C3-M1：`server_busy` 照旧是带 `Retry-After` 的 `shared.ServerBusy(d)`；健康度面板与目录视图的 `useSWR` 有自己的 `onErrorRetry`，遇到 `server_busy` 就停，由人经"重试"决定；网页的测试（4.12、4.16）；修订核对第三轮之后照 C4-N2 只留在面板上：`listIndex` 不答 503，目录视图不带它。
39. Nit 里的几处选择：`Write.Grants` 是单元拥有、模式的守卫（与单元自己的锁下的检查，M10/P1 设计 2.1 第 24 条）写的指针；恢复的 `AreaHashes` 的读含标了删除的行；开始的探查照计划写回的哈希算；P1/S3 的测试怎样把改回推过第一批写明；lint 的请求在名额之外合并、一个 `LintTimeout` 从合成的那一次开始；合并的键带 `Contract` 与 `AreaRootID`；`V` 游标的规则；Step 的分支上的审查修复写到 Step；第 11 节的每一处引用写明是哪一个列表（4.6、4.11、4.13、第 7 节）。

**主持者在修订核对第三轮之后定下的**（[审查记录](reviews/M10-design-review.md)"修订核对第三轮"；负责人可以改判）：

40. 八处都照审查者的建议改（4.6、4.11、4.12、4.16、4.17、第 7、9 节与"总体设计的修订"）。C4-N2 取第一种：`server_busy` 的 `onErrorRetry` 与它的测试只留在健康度面板上，不为 `listIndex` 声明 503。C4-N1 的修复的核对：Step 的分支上的写到 Step（`(M10/P<n>/S<m> fix check <k>)`），同那个分支上的每个提交。总体设计的修订随这份设计的定稿落到 `v0.1-design.md`，`docs/README.md` 加 M10 的一句（第 7 节）。

**P2 实施/Phase 设计决定（另一个编号系列）**：36–43 见 [P2 设计审查](reviews/P2-design-review.md)“主持者的决定（负责人可以改判）”，接 P1 与维护的实施序列35；不覆盖、重编号或替换上面的总设计审查决定1–40。本次只是设计接受，P2/M8 尚未完成。

后续实施决定44（四片窄名称再平衡）、45（恢复行内来源hash折叠）、46（单次预检有界facts保留）与47（C4协作场景等待派生读取完成）见[S2审查](reviews/P2-S2-source-hash-review.md#文档决定与后续)；仍沿实施序列接续，不覆盖总设计审查1–40。

## 12. Phase 进度表

| P | Phase 文档 | 状态 | 审查 | 合并 |
|---|---|---|---|---|
| P1 契约的框架（S1–S4） | [01-P1](01-P1-contract-frame.md) | 已完成 | [设计](reviews/P1-design-review.md)、[S1](reviews/P1-S1-mode-template-review.md)、[S2](reviews/P1-S2-guard-review.md)、[S3](reviews/P1-S3-prechecks-refusals-numbering-review.md)、[S4](reviews/P1-S4-web-review.md) | S1 `45d086a1`、S2 `bad859c0`；S3 `a46c3d1d`；S4 `f261952f` |
| P2 来源区（S1–S3） | [02-P2](02-P2-source-area.md) | 进行中；S1已验；S2旧33b4/4550已验、fb与288 CI失败、恢复折叠及有界facts保留分列，最终代码f1818066的本机门禁、独立审查与提交后gen-check均已核定，精确代码CI40793294472全11绿。文档002a5638的精确CI40797519100全部11作业及必需步骤、code/docs分支四片Test配对，以及main合并`66d8236c12aaa8923616f6f42f2ee33b772884f9`的精确CI40800844731全部11作业及必需步骤均获主持者接受；本记录提交自身CI与双main Test配对仍由后续真实运行收口，完成前不开始S3。前33b4/4550绿、fb/288失败和03不精确同步的历史均保留；记录CI与双main配对待闭环，S3待做 | [设计](reviews/P2-design-review.md)、[S1](reviews/P2-S1-source-rules-review.md)、[S2](reviews/P2-S2-source-hash-review.md) | S1 `bf8d6625`；S2历史代码链 `25f88a27`→`c7cec1fa`→`ca8a27db`→`53488fee`→`056677a4`→`2536175e`→`33b4da5a`，文档`45509391`、重排`fb92974b`、恢复折叠`288ecb2b`，有界facts保留`f18180667a8fa47e44efd2b0d33e4623443eb389`；最终文档`002a5638`，main合并`66d8236c12aaa8923616f6f42f2ee33b772884f9`；记录号不写入自身 |
| P3 lint 与健康度（S1、S2） | — | 未开始 | — | — |
| P4 虚拟页面与 ingest（S1–S3） | — | 未开始 | — | — |
| P5 playbook、说明与收尾的故事（S1） | — | 未开始 | — | — |
| 收尾 | — | 未开始 | — | — |

## 13. 变更记录

- 2026-10-29：补S2实际实施、6M/成本、159控制分类与局部重放、正式审查/修复复核及精确33b4 CI11项成功；当前SetSourceHashes规范为UPDATE self-join，CTE保留历史。B-N1实际文档原审已关闭；doc/main集成与双main样本待闭环，详见[P2 §10.2](02-P2-source-area.md#102-s2-来源-hash去重与验证)。

- 2026-10-29：S2按实施决定44将重排时清单的Imports一项移至邻接第二片；4550与fb的精确CI、容量触发、预测及失败均保留历史。按决定45把守卫已准备的来源hash合入RestoreNodes原UPDATE，保留通用维护与单元起点provenance；288的CI仍失败。决定46在同一次预检内有界保留纯facts，超界回原全量重放。决定47只同步C4 story的派生读取，原E2E 311通过/1失败保留，后端不变证据按scope复用；最终代码f1818066的本机门禁、独立审查与提交后gen-check均已核定，精确代码CI40793294472全11绿。本文档提交自身CI、code/docs分支四片Test配对、main合并CI及记录提交CI/双main配对由后续真实运行收口；完成前不开始S3。前33b4/4550绿、fb/288失败和03不精确同步的历史均保留。旧159、折叠7和缓存11项控制分别记账，文档自身CI、分支配对及main闭环不预填，详见[S2审查](reviews/P2-S2-source-hash-review.md#最终验证与集成)。

- 2026-10-29：S2最终文档`002a5638ee80283823e5b4032a9cce9427ad7365`的[CI40797519100](https://github.com/open-nerve/NerveWiki/actions/runs/40797519100)（attempt1）全部11作业及必需步骤成功，code/docs四片Test配对已在树外接受；随后`--no-ff`合并为`66d8236c12aaa8923616f6f42f2ee33b772884f9`，精确main [CI40800844731](https://github.com/open-nerve/NerveWiki/actions/runs/40800844731)（attempt1）全部11作业及必需步骤成功。以上旧记录保留当时状态；本记录自身CI及合并/记录两main四片Test配对尚待真实发生，由外部回执收口，后续既定S3/Phase自然转录。详见[S2合并记录](reviews/P2-S2-source-hash-review.md#合并与后续验收)。

| 日期 | 变更 | 依据 |
|---|---|---|
| 2026-10-24 | 初稿（`ebed0e07`） | 移交 5 份、代码的调查（`91ee78d8`）、负责人 2026-10-24 的四个决定（第 11 节） |
| 2026-10-24 | 设计审查之后的第一次修订（`9b22c8e5`）：五个 Important 照审查者的建议修（`batch` 的正文规则由计划判、`Merged` 是后备；待建页面照代码的读作附件；0 字节的来源不待消化；`changeset_sources` 没有外键、在清理变更集时删；预检的拒绝在后台也是 `refused`）；Minor 与 Nit 都处理（不取的两处写明理由）；主持者的 16 个决定与对作者的 7 个疑问的回答（第 11 节）；P1 分成四个 Step、后台的去处挪到 P2/S1，共 13 个 Step；每个故事与版本写明 Step；新加 4.19（真实客户端的核对与 M10 的状态）。还没有开工，没有记"开工"一条：负责人确认之后、P1 开工时记 | [审查记录](reviews/M10-design-review.md) |
| 2026-10-24 | 修订核对第一轮之后的第二次修订：一个 Important（虚拟项的游标是它自己的成员 `V`，排序可以是负的）、11 个 Minor 与 23 个 Nit 都照审查者的建议或主持者的选择修（`Merged` 只数来源区里的项；笔记本删除也清掉 `source_hash`、`Forget` 走部分索引；lint 的交错照 `holdAssetBlobs`；`ListIndex` 去掉来源区的选项、来源区一行不写待消化的数目、贡献者不经 `Lint`；`sources` 的部分索引；lint 的名额至少 1、自己的期限、不随第一个请求取消的合并；`changeset_sources` 的叶子的清理；按答复的 `mode` 标出模式的改变；13.2 第 7、15、16 条写全；`listIndex` 照 `ListIndex` 的次序、声明例外；后台的去处在 P1/S3）；主持者的 11 个决定（第 11 节主持者的决定第 24–34 条）。没有不取的 | [审查记录](reviews/M10-design-review.md)"修订核对第一轮" |
| 2026-10-24 | 修订核对第二轮之后的第三次修订（与第二次修订同在 `1f871e26`）：4 个 Minor 与 13 个 Nit 都照审查者的建议或主持者的选择修（`Merged` 只数页是来源时改了字节的项，剩下的一种要管理员的误拒进第 10 节；清理 `changeset_sources` 的叶子的步循环到一批不满、`SKIP LOCKED`、一次运行删完删除了的笔记本；lint 在快照的第一条语句里读模式与来源根，路径在附件的信息之后读，交错改名一页孤儿页，加变异的核对；`server_busy` 照旧带 `Retry-After`、两个视图的 `onErrorRetry` 不重试）；主持者的决定第 35–39 条；第 11 节的每一处引用写明是哪一个列表。没有不取的 | [审查记录](reviews/M10-design-review.md)"修订核对第二轮" |
| 2026-10-24 | 修订核对第三轮之后的第四次修订：1 个 Minor 与 7 个 Nit 都照审查者的建议修（"总体设计的修订"加 10.1，第 9 节引 10.1 的原话；Step 的分支上的每个提交与合并写到 Step，修复的核对也是，README 的一句随之；`server_busy` 的 `onErrorRetry` 与它的测试只在健康度面板；12.3 写出四个依赖的理由，13.1 第 1 条照原文引，13.4 第 13 条加 M10 的三处；开始的探查用计划带着的 `RevertOp.Hash`；lint 的变异的核对只是改名的那一半，两半各经哪个入口，运行之内的模式的核对答同一个 409）；主持者的决定第 40 条。总体设计照第 7 节"总体设计的修订"改了（第 13 节的长期约定随实现它们的 Phase 改），`docs/README.md` 的"Git 工作方式"加 M10 的一句。没有不取的 | [审查记录](reviews/M10-design-review.md)"修订核对第三轮" |
| 2026-10-24 | 开工：P1 的设计审查结束、设计被接受，P1 从 S1 开工；五份移交的"处理进展"已照第 7 节"移交的落实"写下（`d7bb5e8d`），仍是 `open` | 负责人 2026-10-24 的确认；[P1 设计审查](reviews/P1-design-review.md) |
| 2026-10-24 | P1 设计：定客户端的函数是 `shared.ClientOf`（今天的三份都删，四个适配器都用它）；`SeedSpec` 带 `NotebookID` 与 `Message`，`TreeWrites` 加 `ParseTemplate`；两个扩展点的值叫 `NotebookCreation`、`ModeChange`；播种者的有类的错误答 500；`Write.Notebook` 的来路（page 的端口交回状态，notebook 的根加 `StateOf`、`ShareState`、`LockState`）；P1 的守卫是 `page.NewModeGuard(authorizer)`，登记在 `pageRegistrants(pool, authorizer)`，命令行的会话的订阅者是 `pageSessionSubscribers(pool)`，祖先的一线与会话的否决者随 P2/S1；`Write.Grants` 由守卫与单元自己的锁下的检查写（第 11 节主持者的决定第 39 条加注）；上传的预检在 `storage_full` 之前，撤销的在 `page.locked` 与 503 之前；后台撤销的每一批不另检查，由那一批的守卫判；`batch` 的计划的树的一遍在 S2；写者可以在同一父节点下排序来源根的祖先（R4），`AGENTS` 也不能排序；降级的对话框是持有的、404 也重读，降级看不到进程里的占位，`mode` 的两个 id 在帧里；`treeError` 已有的哨兵都在认拒绝之前，transfer 的 `Refused` 带着 page 的拒绝，后台的恢复数 `counts.renamed`，有预检的路径上只有守卫拒绝的情形 P1 已有两种；矩阵按行的工作区，MCP 的 `notebooks` 的期望手写加上变体的笔记本；`EventMode` 的导出在 S4，网页的一步带 `make knip`；来源根的菜单没有删除，树的种类的文字等价在名字之后；00048 进 notebook 的 sqlc 一项；导入的模块测试的名字与 P2 的 `Step.Content` 的类型名；第 1、3 节，4.1、4.2、4.4–4.7、4.9、4.16、4.17，第 5、7、8、9、11 节与"移交的落实"、"总体设计的修订"随之 | [01-P1-contract-frame.md](01-P1-contract-frame.md) 第 2.1 节的二十四处、[P1 设计审查](reviews/P1-design-review.md) |
| 2026-10-25 | P1 的 S1 合并（`45d086a1`）：照设计建成笔记本的两种模式（00048 的 `mode`、`source_root_id` 与两个检查，`Notebook` 的两个必填成员）；`shared.ClientOf`，三份 `clientOf` 删去；建 LLM Wiki 的播种者（`NotebookCreation`、`Seeder`）与 wiki 的模板（`TreeWrites().Seed`、`ParseTemplate`，两种语言的 `raw` 与 `AGENTS`，与笔记本同一个事务、时刻与变更集）；`downgradeNotebook`（`notebook.change_mode`、`transfer.NewWritingJobs`）与模式变化的订阅者（`ModeChange`、`ModeSubscriber`）；事件 `mode`；MCP 的 `notebooks` 与 `open_notebook` 答 `mode`；`checkPages` 的 LLM Wiki 的规则；权限矩阵的变体 `wlab`。与设计不同之处：`Seed` 交回它写下的页的 id，模板交回写下的来源根，建的"恰有一个、相等"因此说的是写下了（审查 A-N3，修复一轮唯一的生产的改动）；`open_notebook` 对门之后被删的笔记本答已有的 `notebook.not_found`；模板的段落对应的反例加末尾多一段与每种块的形状；实施时的 15 条偏离都在测试与它们的写法上；组合根的第三份的增长比它的新测试多（每个 `prepareMatrix` 的用者多种一个 `wlab`），到 P1 结束约 6:45–7:00，仍在 7:30 之下；4.4、4.5（降级的日志另带 `workspace_id`）、第 8、9 节随之 | [P1 S1 审查](reviews/P1-S1-mode-template-review.md)、[01-P1](01-P1-contract-frame.md) 3.30 与 10.1 |
| 2026-10-25 | P1 的 S2 合并（`bad859c0`）：照设计建成守卫的骨架：单元在笔记本行的锁下读一次模式与来源根（`ShareNotebook` 的两列，notebook 的根的 `ShareState`、`LockState`，page 的端口 `ShareByID`、`LockByID` 交回状态，`Write.Notebook`）与单元自己的授权的备忘（`Write.Grants`，指针）；`page.manage_source`（笔记本管理员）；`CheckContract` 与 P1 的四条规则（`AGENTS` 的保护、来源根的删除、根下的四个保留的标题、来源根的管理员规则）；模式的守卫 `page.NewModeGuard(authorizer)` 登记在锁之前（`pageRegistrants(pool, authorizer)` 的 `[mode, lock]`）；拒绝的标出（`Refused`、`page.Refusal`）与 `batch` 的 `failed`；`batch` 的计划的树的一遍判契约、在读之前问 `page.manage_source`；P1 的三个码进六个操作的 `x-problem-codes`、`mcpToolCodes()` 与两种语言的文字；矩阵的变体的六行；e2e 的 LW2 (API)、LW2 (MCP)。与设计不同之处：命令行的会话的订阅者是 `pageSessionSubscribers()`，不收连接池（它唯一的订阅者是事件流的发布者）；事实的位置叫 `NodePlace`（`Place` 已是兄弟间的次序），`ReservedKeys()` 是函数；notebook 的存储答模式与来源根、根由它们建 `State`；`batch` 问 `page.manage_source` 用判定 `page.batch` 时读到的工作区（`decidedNotebook`）；`Refuse` 也取代同一项上批量自己的界线、清掉 `final`，修复一轮为它与守卫标出的节点加了测试（审查 A-M1 = C-M2、C-M1）；实施时的 15 条偏离别的都在测试与它们的写法上；审查另找到两个 S2→S3 的窗口（恢复或导入放到根下的 `index` 改名之后的撤销答没有声明的 `page.virtual_file`，同步的撤销的前一步碰到锁时答 `page.locked`：A-M2、A-M3），都由 S3 关上；两位审查者与主持者自己的核对（额度）；建成的 CI 运行红在 M9 的 grep 的成本守卫，不是 S2 的，修复一轮的运行全绿，组合根三份的两次的平均 6:46.5、6:11.5、6:22，都在 7:30 之下；4.1、4.6（`CheckContract` 的签名）、第 7 节的 P1 一行与"写给后面的 M"（M10 的收尾的一项）随之 | [P1 S2 审查](reviews/P1-S2-guard-review.md)、[01-P1](01-P1-contract-frame.md) 3.31 与 10.2 |
| 2026-10-26 | P1 的 S3 合并（`a46c3d1d`）：预检、后台的拒绝与保留标题的加序号，实施结果见 01-P1 的 10.3；任务范围的 A/B/C 审查已核对，Q-A1 的条件工作区读取由主持者接受（S3 审查决定 32），B-M1 在 `ba5cded6` 修复并由原 B 复核；纯事实模拟及最终结果的 refusal 清理追加变异 85、86；完整 Step A/B/C 独立审查无经过核实的 Important、Minor、Nit、Q 或剩余修复；本机 13 项门禁、各份两次测量、分支 CI 十个作业和两处契约提交的追溯 `gen-check` 已通过；追加审查反向对照 87、88，首次 main CI 已通过；第二次由填合并 SHA 的文档提交触发，按决定 33 核对后才开 S4，数值见 01-P1 的 10.3。头的状态从滞后的“S1 开工”改为 P1 的实际进度。00 第 11 节的设计审查决定不改；本次实现差异与处置记在 S3 审查记录、01-P1 3.32 与 10.3。 | [P1 S3 审查](reviews/P1-S3-prechecks-refusals-numbering-review.md)、[01-P1](01-P1-contract-frame.md) 3.32 与 10.3 |
| 2026-10-26 | S3 的合并记录文档 `ca831b58` 触发的第二次 main CI `40325348498` attempt 1 仅 costs 的既有 M9 IndexItems 守卫失败，七个作业通过，e2e/image 跳过；本机原样两次诊断通过。Actions 重跑被 token 权限拒绝，未产生 attempt 2。决定 34 允许一次实质诊断记录文档提交触发新的完整 push CI，当前待运行；保留失败样本，S4 等全绿及样本核对，M10 收尾追加该守卫的稳定性义务。 | [P1 S3 审查](reviews/P1-S3-prechecks-refusals-numbering-review.md)决定 34 与“第二次 main CI 的失败与诊断”、[01-P1](01-P1-contract-frame.md) 10.3 |
| 2026-10-27 | P1/S4 在分支上建成：创建模式与模板语言、树的保护与可读名称、串行降级与持有的对话框、真实 `mode` 的网页最后一跳，以及 LW1/LW2/LW12 页面与 API 故事；主持者的 R-M1 夹具修复在 `a0aa88dd` 核对。三份原始完整 Step 独立审查及修复后的范围核对已由主持者接受；B-I1（Important，测试覆盖）已关闭，没有剩余核实的 Important、Minor、Nit 或 Q。原始审查头为a0，最终接受头为13ee；B由原审查者重放核对，A/C由主持者按工作流3.4的测试修复例外核对未变范围，记录分别保存。 修复候选 `13ee0bc8` 的13项门禁范围已覆盖：本头新跑 static-build、web、E2E 三项（合计246.659s），网页166文件/3,415测试、E2E291测试、22条已知TimeoutNaNWarning；十项Go命令保留实际受测头 `a0aa88dd` 的通过记录，以唯一改动为51行前端测试、增加六个参数化实例的完整输入等价证明复用，未声称重跑。 合并见第 12 节；P1 仍进行中。按 S3 决定 33–34 转写三次 main CI（含失败 `40325348498` 和新全绿 `40328760108`）、全部分片与每对平均，保留 IndexItems 与 grep 的收尾义务。 | [P1 S4 审查](reviews/P1-S4-web-review.md)、[01-P1](01-P1-contract-frame.md) 3.33、10.3–10.4 |
| 2026-10-27 | P1/S4 合并为 `f261952f`：分支文档 `8556ad07`（含总体设计历史表的触发栏补齐），两父提交为 `f2fe6180`、`8556ad07`，合并树与分支相同。分支文档及首次 main CI 十个作业全部通过；当前仅有一个合并后样本，第二次由本合并记录文档提交触发，实际数据及两次平均在 P1 完成文档前核对转写。P1 仍进行中，M5/M8/M9 仍开放，IndexItems 与 grep 收尾义务不变。 | [P1 S4 审查](reviews/P1-S4-web-review.md)“合并与 main CI”、[01-P1](01-P1-contract-frame.md) 10.4 |
| 2026-10-27 | P1 完成：四个 Step 已合并，S4 的两个 main CI 样本及组合根 Test 步平均已核对；M5 模式事件移交改为 done；给 M11 新建开放的模式契约移交。P2–P5 与 M10 收尾继续按原划分；IndexItems、grep 成本守卫的稳定性义务保留。 | [01-P1](01-P1-contract-frame.md) 10.5、[S4 审查](reviews/P1-S4-web-review.md)、[M11 模式移交](../M11-upgrade/handoffs/M10-mode.md) |
| 2026-10-27 | P1 完成文档的 main CI `40378004872` 在既有 M9 `GrepCandidates` generic 阶段再次超过旧 270 ms 界；按第 7 节的复发入口，在 P1/P2 之间的维护旁支 `m10-grep-budget` 处置。代码 `789b4071` 采用 600 ms、三次中位数，余量 54.05%；三个独立审查无核实发现，真实点查、计划/范围反向对照和六项本机门禁均完成；代码分支 CI `40388475778`（精确 `789b4071`，attempt 1）十个作业全部成功并已核对。本次文档的 CI、合并与 main CI 待进行。P2 的入口以维护后 main 全绿及主持者验收为准；IndexItems 独立记录，M10 收尾义务仍在。 | [候选预算维护审查](reviews/grep-candidates-budget-review.md)决定 35 |
| 2026-10-27 | M9 候选预算维护合并（`2322244a`）：维护文档 `751719ea` 的分支 CI `40392243355` 与合并后的首次 main CI `40394818624` 均十个作业成功；合并父提交为 `b112450f`、`751719ea`，合并树与分支文档相同。历史分支阶段的记录保留；本合并记录的提交及其 main CI 尚待进行，P2 等该 CI 全绿与主持者验收。IndexItems 独立义务及 M10 收尾核对不变，无新增产品决定。 | [候选预算维护审查](reviews/grep-candidates-budget-review.md)“维护文档、合并与 main CI” |
| 2026-10-27 | P2 设计已由三位独立审查者初审、修订并由原 A/B/C 复核；2 Important/2 Minor 及 B-Q1 在设计层 resolved，主持者接受决定36–43（另属实施/Phase设计序列）。4.2/4.6–4.8/4.16–4.18、接口、P2范围/测试/风险按02-P2最小修订；无来源正文管理员例外，保留普通大移动、明确新来源移动界、baseline重复目标、真实后台名义动作与非并发缩批取舍、有限读取与实际迁移/成本义务。入口维护记录提交403841fc的main CI40397719553十项成功已核对，与40394818624的bootstrap Test均值为440.5/317.5/260.5秒；这只是P2入口。当前待本设计文档精确main CI绿后开S1，P2与M8未完成；IndexItems/grep收尾义务不变。 | [P2设计](02-P2-source-area.md)第1/2/10节、[P2设计审查](reviews/P2-design-review.md) |
| 2026-10-27 | P2/S1来源规则已实施：rewrite持久与全撤销链、来源事实/恢复森林、不可变与管理/session、按项Merged、实际后台动作/批界、API/MCP/双语/fake及LW4/LW5 API/MCP。初审头6fac，A/B无发现，C初审1 Minor C-M1与补充1 Minor C-M2由主持者接受、第一组b750已提交并增量核验；b750准备诊断另确认C-M3 Minor，第二组修复`fd00ee670c24db3ad4d9478419acd8340f1e6d91`；三个ID的正式复核状态见S1审查，最终代码`fd00ee670c24db3ad4d9478419acd8340f1e6d91`，该分支文档阶段尚未合并；后续合并及验收见S1合并记录。08fa降级覆盖与6fac矩阵fixture修复及旧失败/存活边界保留；正式结果见P2 10.1。P2/M10继续，S2/S3 pending，M8open；合并见第12节。 | [P2设计](02-P2-source-area.md)3.14/10.1、[S1审查](reviews/P2-S1-source-rules-review.md) |
| 2026-10-27 | P2/S1合并（`bf8d6625`）：文档78f0与首次main精确CI已核；本记录CI/两main均值待root树外验收，S2未释放，P2/M10继续。 | [S1合并记录](reviews/P2-S1-source-rules-review.md#合并与后续验收) |
| 2026-10-28 | 转录S1两main十项绿与bootstrap均值328/313.5/394秒，S2已获准继续；按已接受增量纠正SourcePrecheck授权ctx、同步restore模式点读、实际SRF/写前像/有限reader与Probe tuple查询。S2-4/整Step验收仍进行中。 | [P2设计](02-P2-source-area.md)、[S1后续核对](reviews/P2-S1-source-rules-review.md#合并与后续验收) |
