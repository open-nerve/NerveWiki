# M10/P2 来源区

| 项 | 内容 |
|---|---|
| 里程碑 / Phase | M10 模式契约与 LLM Wiki / P2 来源区 |
| 日期、状态 | 2026-10-29；S1、S2已验；S3最终代码`ae4501aa166771db153dae5dfa710ef16ee26ac4`的实现/原席复核/最终门禁及精确代码、文档、合并CI均已核，no-ff合并`ecd79c77d0cdb2259d50758b26514f61ebb329ca`。本次记录CI及两main分片均值须提交后核对，P2done另作；P2进行中，P3–P5与M10收尾未开始。当前集成见10.3及[S3审查](reviews/P2-S3-source-web-review.md) |
| 基线 | `main` = `origin/main` = `403841fc75d0e5ac6b5f39447158213c0e07b2fe`，树 `b3665215f745d4dcfe7e8d339d4882e6d95b4a63` |
| 入口 | P1 已完成；其后 grep 成本维护也已合并并核对，两份 main CI 全 10 项成功；第 1 节 |
| 权威 | [文档约定](../../README.md)、[总体设计](../v0.1-design.md)、[M10 总设计](00-M10-design.md) §4.6–4.8、§4.16–4.18、§7、§9；[P1](01-P1-contract-frame.md) §8–10；[M8 历史移交](handoffs/M8-history.md)、[M9 MCP 移交](handoffs/M9-P1-mcp-registry.md) |
| 代码位置 | `page/`、`asset/`、`notebook/`、`transfer/` 相对 `server/internal/modules/`；`bootstrap/`、`shared/`、`platform/` 相对 `server/internal/`；迁移相对 `server/migrations/sql/`；`W/` 相对 `web/apps/web/src/`；行号均按上述基线 |
| 审查 | [P2 设计审查](reviews/P2-design-review.md)：三位独立初审、首轮修订、原 A/B/C 复核均完成；2 Important / 2 Minor 与 B-Q1 在设计层 resolved，无新增核实 I/M/N/Q；设计提交 `4755b4c9` 的精确 main [CI 40413727596](https://github.com/open-nerve/NerveWiki/actions/runs/40413727596) 已十项成功；[S1 实施审查](reviews/P2-S1-source-rules-review.md)与结果见10.1；[S2实施审查](reviews/P2-S2-source-hash-review.md)已闭环；[S3审查](reviews/P2-S3-source-web-review.md)记录三份独立初审、三个Minor修复与原席复核，结果见10.3 |

## 0. 三个独立为绿的 Step

| Step | 交付 | 合并时允许存在的窗口、关闭处 |
|---|---|---|
| S1 来源规则 | 00049 rewrite；正文/祖先事实；纯契约与会话否决；按项 batch 正文检查；同步/后台历史的 rewrite 全链；来源预检、refused；API/MCP 的 immutable 与来源树权限、两语言文字；LW4/LW5 API/MCP | 不去重，没有 source_hash；界面仍可显示会被服务端拒绝的来源编辑/管理入口；重复能力由 S2，控件/编辑失去/来源添加对话框由 S3 完成。已有文本/类型/fake 必须当步绿。 |
| S2 去重与派生哈希 | 00050 与回填；单元维护、两条删除、Forget；附件 SHA256；单元/子树/历史/开始任务/导入的查重；新增来源移动的工作界；duplicate_of 完整管道；不变量、性质、交错、计划/成本/600 万迁移；LW3/LW5 duplicate/LW6/LW7 API/MCP 按第 5 节 | S3 前重复错误有中英基本文字，后台报告新增枚举已可读，但未提供全部 UI 的路径链接；不能由此推迟 S2 契约/tsc/fake。 |
| S3 完整网页 | 一处严格来源判断、角色菜单与只读控件、source 编辑失去并释放服务端会话、来源添加/标题/移动确认、duplicateText 与历史说明；LW3–7 页面、LW12 增量 | P2 完整。lint、虚拟文件、ingest、playbook 仍是 P3–P5；不在本 Phase 搭空界面或提前实现。 |

每个 Step 在自己的 worktree；每个提交编译、vet、lint、短测试与受影响网页检查通过。完整门禁、运行时变异、分支 CI、三方审查及修复核对、分支文档齐备才 `--no-ff` 合并。main 与合并记录的精确 SHA CI 均绿，再进入下一 Step。Phase 完成文档也检查其精确 CI；不把 P1 的绿当 P2 的绿。

## 1. 基线与入口证据

P1 的最终文档为 `b112450f`。其第一次精确 CI 的既有 grep 成本失败已单独诊断、测量、审查并在维护分支修正预算；维护代码 `789b4071`、文档 `751719ea`、合并 `2322244a`、合并记录 `403841fc` 的四份 CI 全 10 项成功。本文基线为最后一个，非最初 b112。维护没有改本文引用的生产源码；改的是 grep 成本测试、Makefile 注释和相应设计/记录。

两份实际 main 工作流：[40394818624](https://github.com/open-nerve/NerveWiki/actions/runs/40394818624)、[40397719553](https://github.com/open-nerve/NerveWiki/actions/runs/40397719553)。只用 bootstrap 的 **Test step** 时长判断 450 秒线：

| 份 | 合并 / 记录 Test 秒 | 平均 | 450 秒余量 |
|---|---|---|---|
| bootstrap-1 | 437 / 444 | 440.5 | 9.5 |
| bootstrap-2 | 303 / 332 | 317.5 | 132.5 |
| bootstrap-3 | 263 / 258 | 260.5 | 189.5 |

记录 CI 其余实际 step 秒：race 573、costs 546、guards 751、short 187、web Test 208、E2E 211、Image smoke 78。它们不是 bootstrap 的预算，也不把 job 总时长混进 Test 平均。入口回执是主持者持有的 `p2-entry.json`，原始 run/jobs/timings 与 SHA256、postmerge-pair 均归树外证据，不把本机路径写成仓库长期依赖。

设计基线时的 gap：ModeGuard 只有 P1 三类规则；Step 无正文/来源事实；版本无 rewrite；无 source_hash/duplicate；网页树只认 AGENTS/来源根。已有 P1 的 `Grants`、`Refused`、batch OpFailed、开始撤销预检、后台 refused/finalizer、保留名编号、mode 事件与降级保持。IndexItems 的 30 ms 观察及 grep 的 600 ms 新预算的后续稳定性仍归 M10 收尾，本文不宣称已稳定，不在 Step 中暗改其线。 上述为设计基线差距；S1/S2实际验收见10.1/10.2，S3网页与修复见4.6/10.3，合并/main闭环仍按当步真实回执。

## 2. 目标、范围和明确修订

把 00 §4.7 每一格落实到所有真实写入入口；来源严格指来源根的后代，附件同样属于来源，根自己的说明不是来源。只新增、现有来源树操作管理员、系统 rewrite 例外与其撤销、逐字节去重都用同一组事实/规则。现有普通模式的 no-op、单次大移动、编辑/历史语义保留。

### 2.1 对 00 的精确修订意图

以下正文修订已经三方审查、原审查者复核并由主持者接受，同本设计一并落入 00。实施/Phase 设计决定序列从 **36** 接续（维护为35），与 00 总设计审查的1–40是两套序列；36–43 逐项记在 [P2 设计审查](reviews/P2-design-review.md)“主持者的决定（负责人可以改判）”。

后续实施决定44（四片窄名称再平衡）、45（恢复行内来源hash折叠）、46（单次预检有界facts保留）与47（C4协作场景等待派生读取完成）记于[S2审查](reviews/P2-S2-source-hash-review.md#文档决定与后续)，接续43，不覆盖00总设计审查的1–40。

实施决定48（来源模板改用两个实际入口名）见[S3审查](reviews/P2-S3-source-web-review.md)“主持者的决定（负责人可以改判）”。只修改双语raw模板的一句；新建模板使用现行名称，存量正文不改；P1规范全文/大小同步、S1历史字节保留。

| 决定编号 | 修订位置与新文字要点 | 理由 |
|---|---|---|
| 36 | 00 §4.8“都以 MaxUnitNodes 为界”改成：一次把既存子树从区外移入来源区，至多10,000个节点（空页/附件也计），超过答422 `page.source_too_large`；REST Move、MCP Relocate、batch 单次移动、同步/后台撤销放回均同；普通模式、区内/移出、搬动来源根而成员不变不因此限10k。开始历史任务的整计划与跨批恢复按最多10k一块读元数据/查重；既有全树计划不自称受此界。新增边界的确切次序见3.8。 | 当前单次移动、undo place无此上限，不能虚称已有。来源新增会做额外每节点哈希/去重/派生写，00的工作界需要实际入口；不改普通的10,001正例。主持者已在原三位复核之后接受，实施证据仍待完成。 |
| 37 | 00 §4.7正文/还原行加：来源 version_restore 与已有编辑会话的保存，即使字节相同也拒绝；普通、非来源与普通一次性同字节写仍无写入。no-op 的窄纯契约预检与 changed 写的注册守卫分别验证。 | 现有 `unit_content.go:65` 先返回，单加 guard 条件到不了；明确“从不”与普通 no-op 的交点。 |
| 38 | 00 §4.8 duplicate_of 改成：仅当被指出的节点在本单元开始时已是活来源、kind与本次比较的哈希相同，才给 id；否则不给。新建、旧节点移入/恢复、空来源首次填入、来源被改写成另一个哈希均据此；回滚后仍是同一来源同一字节才是可指的目标。 | “新增来源”不等于“新建节点”；回滚可保留节点却撤回来源身份/被比较字节。 |
| 39 | 00 §4.7撤销说明把“先移回、再写”改为“需要还原正文时先在来源区外还原，再移回；重复时先处理冲突来源，再恢复或移回；重新移回仍须去重，处理冲突不保证之后一定成功”。 | 谁也不能在移入后普通修改来源，旧提示会推荐再次被拒的步骤。不扩大管理员权限。 |
| 40 | 00 §4.6撤销事实与§4.8开始预检明确：content-only无整树时只补相关祖先线；大计划按块读hash/外部匹配，模拟完整顺序；外部候选排除本计划会改变的节点，后者在模拟集合中维护；不取全文。 | 保留现有content-only快路，避免静态初始候选误拒“先删再恢复”或漏判同计划新增。 |
| 41 | 00 §4.2/风险的冷缓存测量明确：实际 goose Up 事务只在开始前冷却一次，全事务时间是停机证据；分别冷却的CHECK/index诊断另列，不相加冒充同一实际事务；现有LLM回填与空首发分别测。 | 冷启动后的CHECK会自然预热后面的index；600万义务本Phase完成，M12另在真实服务器重量。 |

决定 **42**：00 §4.2/4.8 的 source_hash 非唯一部分索引在原三个键后追加 `id`。外部匹配按 key+id 游标分块读取，完整计划的排除集合只在 Go 内保存；每次 SQL 数组与总返回行均≤10k，不发送全计划 excluded_ids，更不把它分块做 NOT IN。该扩展增加索引空间、建索引/回填写入成本，600万迁移和10万来源成本样本同时记录四列与原三列的差异；不是把它称为免费的覆盖索引。准确协议见3.7/3.11；三方设计审查及原审查者复核已完成，设计提交4755的精确main CI40413727596已十项成功；S1结果见10.1，索引/hash/成本仍待S2。

决定43的具体范围：保留M9 compact/Joined/同组批内合并/合法缩批；开始与worker按生产批界算未缩减名义计划，按真实批内动作预检。没有并发时，实际缩批也可能使已通过预检的任务在后批遇来源拒绝；反方向名义拒但更细分法可行也仍拒，不搜索分批。此前提交保留、失败批回滚，同步/后台结果可不同。详细协议与独立证据见3.13，产品说明不暴露内部类型名。

技术落名：`Step.Content` 用 **StepContent**，避免现有 `ContentMeta`；守卫构造为 `page.NewModeGuard(pool, authorizer)`；§3.8增加 source_too_large 的操作表/文字/后台 refusal（code为自由字符串）；§4.16加入来源上传队列中页面与附件的结果区分，普通附件上传API不混入Page。

### 2.2 需要负责人确认

无常规确认项。主持者在原 A/B/C 复核之后接受本设计及决定36–43（负责人可以改判）；当时只是设计接受；S1/S2实施与成本/迁移见10.1/10.2，S3最终实现、三审与修复见10.3；Step集成及Phase完成按实际CI后续收口。若实施证据表明获准方案不能满足现有架构、权限或历史语义，主持者按工作流重新定处置；只有真正重大架构冲突才按用户约定升级。

不做：P3的SourceArea整区lint读、索引规则、P4虚拟文件/ingest、P5prompt、M11升级状态或保全；不接触真实客户端配置。

## 3. 服务端

### 3.1 文件与所有权

| Step | 文件（现有文件扩展，标“新”的新增） | 作用 |
|---|---|---|
| S1 | `00049_page_page_revisions_rewrite.sql`（新）、`server/sqlc.yaml` page项、`page/adapter/postgres/queries/changesets.sql`、`history.sql`、`history.go`、`changesets.go`、生成目录 | rewrite持久化/读取；只列page自己的迁移 |
| S1 | `page/domain/contract.go`、`contract_test.go`、`changeset.go`、`revert.go`、`revert_schedule.go`、`revert_facts.go`、`batch_plan.go`、`batch_contents.go` | 同一纯契约、瞬时来源事实、正文判定和历史标记 |
| S1 | `page/app/extension.go`、`unit*.go`、`ports.go`、`mode_guard.go`、`revert_contract.go`、`revert_changeset.go`、`jobs.go`、`edit_page.go`、`apply_batch.go`、`fakes_test.go` | 元数据/祖先线所有生产者、特殊no-op、锁前MCP预检、单元/后台链 |
| S1 | `page/app/revert_actions.go`、`revert_job_contract.go`、`job_batches.go`、`revert_source_lines.go`（新）、`page/tree_writes.go`、transfer `app/background.go`/`start_background.go`/`import_ports.go`、`module.go`、`bootstrap/transfer.go`及各fake | 共用实际动作遍历、原批界算法和JobBatches闭包；CheckRevert/PlanRevert在S1使用后台事实，不等S2 |
| S1 | `page/mode_guard.go`、`bootstrap/registrants.go`；各模块HTTP测试/API契约、MCP码表、web文字；e2e wiki LW4/LW5 | mode注册为guard与sessionVetoer，均在lock前 |
| S2 | `00050_page_nodes_source_hash.sql`（新）、`page/adapter/postgres/queries/sources.sql`（新）、`source_area.go`（app/adapter/root新）、`nodes.sql`、`migrations/*source_hash*_test.go` | 哈希元数据、探查、批更新、锁、迁移、Forget |
| S2 | `page/app/unit_sources.go`（新）、`source_precheck.go`（新）、`unit_move.go`、`unit_relocate.go`、`unit_revert.go`、`unit_restore.go`、`jobs.go`、`unit_import.go`、`ports.go`、`module.go` | 单元级来源追踪、有限来源移入、开始/历史模拟、import正常重复结果 |
| S2 | `asset/app/ports.go`、`attachment_infos.go`、`adapter/postgres/queries/infos.sql`、`lifecycle.go`、`bootstrap/assets.go`、`page/module.go` | 已有AttachmentInfos批量端口加SHA256、交给WriterDeps |
| S2 | notebook `app/ports.go`、`downgrade_notebook.go`、`module.go`、`bootstrap/deps.go`及适配；shared/httpserver/mcpserver错误、page/refusal、transfer报告/持久/HTTP | Forget与完整duplicate_of管道 |
| S2 | transfer `app/import_ports.go`、`import_write.go`、`domain/import_plan.go`、`report.go`、`bootstrap/transfer.go` | 重复叶/空文件夹与后代保留；已提交后更新计数/映射 |

生成器仍由 Makefile；源码不会手改生成SQLC/Go/OpenAPI/TS。new schema仅page迁移归page；00050数据迁移只读notebook/asset的已批准例外写注释，运行时经端口，不扩大page SQL schema。不臆造 archtest 的跨表读白名单：现有归属检查核DDL目标，数据SQL靠审查。

### 3.2 事实类型与祖先的一处解释（S1）

以下是最终应有的接口形状；实现可在审查后精简私有helper，不能省掉事实或复制判定规则。

```go
// app: already occupied ContentMeta keeps its existing meaning.
type StepContent struct {
    Hash []byte
    Bytes, BeforeBytes, Merged int
}
type StepLines struct {
    Before, After []domain.Ancestor
    AfterParents map[uuid.UUID][]domain.Ancestor
}
// Step gains Content *StepContent, Rewrite bool, Lines *StepLines,
// and Kinds map[uuid.UUID]domain.Kind for every changed node.
// Lines nil = not supplied; non-nil empty line = known notebook root.

// domain: NodePlace gains InArea bool; NodeFacts gains Kind Kind.
// StepFacts gains Content *ContentFacts, Rewrite bool, EditSession bool.
type ContentFacts struct { Hash []byte; Bytes, BeforeBytes, Merged int }
func CheckContract(NotebookState, StepFacts) Verdict // one rule table, retained

// construction at the module root; app takes the minimal source reader.
func NewModeGuard(pool *pgxpool.Pool, auth shared.Authorizer) *ModeGuard
func (g *ModeGuard) GuardWrite(context.Context, Step) error
func (g *ModeGuard) VetoEditSession(context.Context, SessionOpening) error
```

域内增加“给root-first变化列表补Before/After来源事实”的函数；只走 `ParentID` 图与已给的根祖先，不读数据库。严格祖先含source root时才InArea，ID等于source root自己始终false。一个source root被移动/改名、它的祖先被移动时，其后代仍识别为来源；不能将全Changes一律赋成移动根的membership。附件也有Kind与父链。Before/After分别计算，恢复使用计划After，不查询“仍已删除的节点目前是否live”。 恢复跳过已活成员后可能形成多根森林，AfterParents分别提供每个外部父节点的祖先线，不拼成一条；步内Changes的当前父关系优先于外部快照。

Kinds由已读的domain.Node/subtree/group产出；create明确page/asset，delete/move/restore按每个Changes对应节点填，正文单页填page，undo的state/plan也带每节点Kind。不能从有无Content猜附件，更不在守卫按节点补N次FindNode。

已读祖先不重读：CreatePage的`lineOf`、Move目的line、Relocate换父line一直传到insert/move helper；ImportUnit缓存line也传到`insertPage/insertAsset`，因为它没有经过公开CreatePage/CreateAsset。改名、删除、正文、附件创建、同父Relocate、undo place没有line时，LLM模式按需读一次操作根的祖先；同父Before/After共用。改变父节点时Before单独读，After用已读目的line。children内部父关系从Changes/原Node取，无每后代一次递归查询。普通模式在最外层返回，零额外来源祖先读取/管理授权；请求原有响应祖先读取不在这个零额外承诺内。

`ModeGuard`的并发共享实例不缓存单元事实/授权。`Write.Grants`仍是单元指针；第一次需要manage_source才问，后台每批新的memo。S1的Unit.withSourceLines在guards之前准备并复用source facts，GuardWrite消费它；后续S2维护再复用；公开Step副本的Lines仅只读。会话否决者用已锁笔记本状态和页面祖先判严格后代，登记 `[mode, lock]`，根说明/AGENTS可开；会话开启前的过期/接管写都在事务，拒绝后回滚。

### 3.3 内容、no-op、码次序（S1）

CreatePage给before=0，hash/bytes取`Unit.content`；普通改变正文给当前ContentLock的ByteSize、计算后的hash/bytes，Merged=1（非来源也可带1，域只在来源看它）。空创建Merged=0；Appender写设置Rewrite=true、无会话，正常/推后batch参与者都从同一个Appender进入。普通用例不能通过REST/MCP输入Rewrite；undo只有读到持久版本标记才设它。

`unit_content.go:49–80` 的正式顺序：找/锁content与node → 验证给定session（保留外来/结束session码）→算hash/bytes。相同字节：standard立即旧返回；LLM下仅当kind=version_restore、带有效session、或batch交来Merged>1时，构造同一ContentFacts、补必要祖先、调用同一CheckContract并标记Refused，之后仍直接返回，无changeset/Touch/guards/锁检查。因此非来源no-op绝不会因这次改动碰EditLock。普通一次性来源同字节写照旧no-op；不把同字节写计为第二次正文。这个窄检查是纯契约预检，变异证明与guard注册证明分开。

改变字节：当前base mismatch仍先于guard；构造完整Step → `Unit.apply`的guard →写。`version_restore`来源必拒，不能因目标版本rewrite=true例外；有效编辑会话的来源保存必拒（空页也是）；非rewrite、非session、非version_restore的空来源第一次正文允许；非空来源改正文拒；Merged>1拒；系统rewrite及undo对应rewrite允许。无API输入可伪造rewrite。

MCP `EditPage`现在在`:152`读锁。保留actor/path/读权限/baseVersion/Replace领域错误的先后；Replace成功后、编辑锁预检与parser预算前，以已找出的`at.Ancestors`和StateOf运行同一纯契约。standard不补祖先/不问manage_source；来源被改动时immutable早于locked/503。真正unit仍在锁下重判；不因预检通过跳过guard。相同Replace结果按现有one-shot no-op规则，不新制造来源的修改。MCP write无锁外锁预检的入口由unit判；如已有同类预检，按同一链核对，不再添第二份规则。

模式内部顺序：protected → virtual_file → source_too_large（S2仅新增移入界）→ immutable → duplicate（S2）→ source_admin_only。普通领域查找/授权/参数与各操作已有领域错误仍在它前；3.8只为来源移入overflow和深度检查写一个明确例外。batch按项次序，plan整体在编辑锁预检前，故op0锁/op1来源非法答op1的immutable。所有模式拒绝由`domain.Refuse`标出实际节点，authorizer数据库错误原样传播。

### 3.4 rewrite 持久化及撤销的全链（S1）

00049：`ALTER TABLE page_revisions ADD COLUMN rewrite boolean NOT NULL DEFAULT false;` Down drop。旧行false；Down/Up丢标记后撤销只会偏严，升级说明写清。sqlc page schema追加本迁移。

`Revision`加Rewrite；集中`recordRevision`收这一位，新建false、直接用户写false、Appender true、undo取op.Rewrite。真实SQL保持原base与连续性WHERE：

```sql
INSERT INTO page_revisions (..., rewrite)
VALUES (..., sqlc.arg(rewrite))
ON CONFLICT (changeset_id, node_id) DO UPDATE
SET revision = excluded.revision, content = excluded.content,
    content_hash = excluded.content_hash, byte_size = excluded.byte_size,
    updated_at = excluded.updated_at,
    rewrite = page_revisions.rewrite AND excluded.rewrite
WHERE page_revisions.revision = excluded.base_revision;
```

不能用OR或最后一个标记覆盖。真实Store测试 true+true、true+false、false+true、false+false；fakeStore.RecordRevision同AND，不让fake的历史比数据库宽。revision continuity旧测试不变。

两个`ChangesetVersions/ChangesetVersionsOf` SELECT加 **r.rewrite**，不是base的b.rewrite；`ChangesetVersion.Rewrite`经`changedVersionOf`映射。RevertOp加Rewrite与`Bytes`（目标base的BaseBytes），ContentNow加Bytes（ContentsNow读byte_size）。scheduler的Hash仍是目标base hash，而Rewrite取被撤销C的version。`RevertFacts`按op目标hash/Bytes及瞬时before Bytes生成事实。同步`Unit.Revert`与后台`revertStep`压缩、`revertPlan.ops`重建、`RevertBatch`写回都携带Rewrite/Bytes，新的undo版本也记录true，使undo-of-undo同样允许。它们是内存job计划，不新增持久job格式。

content-only undo没有Tree：state读取相关节点已有父id，再在LLM时批量补这些节点的祖先线（有限层，按所需id，标准模式零额外读），供RevertFacts；不为这个检查读整本树。带树计划则复用treeSim，在每一步前后补事实，临时名称/临时移到根仍是普通树操作。模拟的content state按每次write-back/restore更新Bytes/hash；不能永远拿初始ContentsNow。 S1具体由revert_source_lines.go批量补相关父线到RevertState.Lines（字段不是ParentLines）；标准模式和已有Tree的路径不补读。

保持三种已接受偏严shape：祖先先移入、后临时改名后代需要admin；移入后普通写回immutable；先放回S1再删同hash S2 duplicate（S2）。restore后write-back两步分别判。没有净效果豁免、没有按` ~hex`名称放行。来源root/AGENTS的既有历史保护不变。

### 3.5 batch 两遍与合成后备（S1）

PlanBatch保存NotebookState供Contents使用。树一遍按**item当时**Before/After tree判来源树操作；TreeAt(k)是k之后，Before需TreeAt(k-1)或当前模拟状态。正文一遍在Replace/base/path领域检查之后、parser预算之前拿该item的before content（已按此前move重写过）和after content；相同hash+size无写。来源的第一次实际字节改变后即不可再变，root说明不计来源。

`plannedPage`和导出的`PlannedPage`加Merged；只在这项的页面是严格来源且字节实际改变时++，新建从0字节比较。`apply_batch.create`只给叶的PageDraft交最终正文及Merged，补父页0；`apply_batch.content`在已有页第一个正文项交最终正文及Merged，后项不再写。普通调用不接受客户端指定Merged。guard中的>1只是后备，不能替代按项的权威检查；对“先改外部P、移入、再改P”的非法第三项，guard可能只看到外部第一次合成，必须由plan抓住。

必须通过四个正例：admin `[create raw/x=A, move out, edit=B]`、`[empty S first=A, move out, edit=B]`；writer `[create raw/x=A, write same A]`、`[create raw/x="", first=B]`。必须拒三例：createA后editB在后项；empty firstA后再次改变在后项；外部editP、move入raw、再次editP在第三项。MCP的old_string不得空，空页首次正文用合法write，不能造不可达edit测试。 S1落地的无base首正文仅限本batch新建的空来源；既存空页、标准模式与区外页保留原PathTaken，不能扩写为所有空页无base可写。

保留已接受后备偏严例：admin移出、清空、移回、再次写；Merged可能>1。合成body导致move-in查重比最后正文与逐项调用两种不同方向属于S2工具说明和测试。拒绝归属：plan是实际违规项；guard新页归创建项、已有页归首次正文项，move归该move项；Refused先于replans。

### 3.6 S1 预检、完整路径和后台

upload.Check已做的保留名继续；新建来源只是新增，不问admin。同步撤销的不加锁/锁下两遍共享contract并按原RevertPlan.Ops；后台开始/worker同样共享纯规则，但事实来自3.13的compact→名义cuts→本批group合并动作，不能直接复用同步RevertFacts的顺序。该结构和接口S1即落地，S2才增加hash查重/snapshot；content-only补线见3.4。后台每批从当前mode/权限生成事实，实际guard仍最后判，不给worker“已预检”旁路。来源descendant改名在第二批遇角色下降必须refused，第一批留着；P1来源root例继续。

`revertDefect`保留Refused；`bootstrap.treeError`仍九个旧sentinel（包括ErrLocked）优先，再page.Refusal；transfer.unitFailure先Refused后一般403/404；非refused最终失败清整个Refusal，保留P1 timeout/interrupted/cancelled回归。无额外拒绝error日志：正常4xx不记敏感路径/正文，原成功写日志继续只有既定IDs。

### 3.7 S2 schema、读/写端口与实际SQL

00050在一个goose事务：加bytea列与32字节命名CHECK → 建**非唯一**部分索引 → 回填已有live LLM来源。schema准确为：

```sql
ALTER TABLE nodes ADD COLUMN source_hash bytea,
  ADD CONSTRAINT nodes_source_hash_check CHECK (octet_length(source_hash) = 32);
CREATE INDEX nodes_notebook_id_kind_source_hash_idx
  ON nodes (notebook_id, kind, source_hash, id)
  WHERE source_hash IS NOT NULL AND deleted_at IS NULL;
```

回填用goose StatementBegin包DO：遍历`notebooks WHERE mode='llm_wiki' AND deleted_at IS NULL`，每本`WITH RECURSIVE area AS (live root SELECT UNION ALL live children JOIN area)`；排除root自身，页LEFT JOIN page_contents按node_id取byte_size>0的content_hash，附件LEFT JOIN asset_blobs按node_id取sha256，UPDATE的仅page.nodes。live孩子链，不把删除枝下的节点算活来源。只读外模块例外写迁移注释；缺来源根/元数据属已有坏库，不填伪哈希，完整不变量测试会指出。Down去列（索引/检查随列）；再Up要恢复已有LLM，不能假设发布前从无LLM。

具体回填SQL（仅00050迁移使用跨模块只读例外；实际goose包裹/注释同相邻迁移）：

```sql
-- +goose StatementBegin
DO $$
DECLARE wiki record;
BEGIN
  FOR wiki IN SELECT id, source_root_id FROM notebooks
              WHERE mode = 'llm_wiki' AND deleted_at IS NULL LOOP
    WITH RECURSIVE area(id) AS (
      SELECT id FROM nodes WHERE id = wiki.source_root_id
        AND notebook_id = wiki.id AND deleted_at IS NULL
      UNION ALL
      SELECT n.id FROM nodes n JOIN area a ON n.parent_id = a.id
        WHERE n.notebook_id = wiki.id AND n.deleted_at IS NULL
    ), values_to_set AS (
      SELECT n.id, CASE
        WHEN n.kind = 'page' AND c.byte_size > 0 THEN c.content_hash
        WHEN n.kind = 'asset' THEN b.sha256
        ELSE NULL::bytea END AS hash
      FROM area a JOIN nodes n ON n.id = a.id
      LEFT JOIN page_contents c ON c.node_id = n.id
      LEFT JOIN asset_blobs b ON b.node_id = n.id AND b.notebook_id = wiki.id
      WHERE n.id <> wiki.source_root_id
    )
    UPDATE nodes n SET source_hash = v.hash
      FROM values_to_set v WHERE n.id = v.id AND n.notebook_id = wiki.id
        AND v.hash IS NOT NULL;
  END LOOP;
END $$;
-- +goose StatementEnd
```

这个递归以现有树无环不变量为前提，不把坏库迁移静默修成别的树。恢复读heads的SQL不取正文：

```sql
SELECT n.id, n.kind, n.source_hash AS before_source_hash,
       c.content_hash, COALESCE(c.byte_size, 0)::bigint AS byte_size
FROM nodes n LEFT JOIN page_contents c ON c.node_id = n.id
WHERE n.notebook_id = sqlc.arg(notebook_id)
  AND n.id = ANY(sqlc.arg(node_ids)::uuid[]);
```

不加live过滤是为了恢复trash；附件的byte/hash由既有AttachmentInfos批量覆盖，初始source_hash仍取上面nodes列。实际adapter返回同一请求缺少的node要让计划的原missing检查处理，不以空hash伪造它存在。

运行时最小接口：

```go
type SourceValue struct { ID uuid.UUID; Kind domain.Kind; Hash, BeforeSourceHash []byte; Bytes int64 }
type SourceKey struct { Kind domain.Kind; Hash [32]byte }
type SourceProbe struct { Key SourceKey; After *uuid.UUID; Take int }
type SourceMatch struct { Probe int; ID uuid.UUID }
// Store methods, all use postgres.DB(ctx,pool); variable arrays use Planned.
SourceHeads(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]SourceValue, error)
ProbeSources(ctx context.Context, notebook uuid.UUID, probes []SourceProbe) ([]SourceMatch, error)
// BeforeSourceHash is nullable nodes.source_hash, for pages AND assets.
SetSourceHashes(ctx context.Context, notebook uuid.UUID, values []SourceValue) (map[uuid.UUID]SourceValue, error) // changed rows with old BeforeSourceHash; nil Hash clears
SourceBaselines(ctx context.Context, notebook uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]SourceValue, error) // live derived keys before deletion
LockSources(ctx context.Context, notebook uuid.UUID) error // transaction required
ForgetSources(ctx context.Context, notebook uuid.UUID) error
// Existing asset/page AttachmentInfo gains SHA256 []byte.
// WriterDeps.Attachments AttachmentInfos; existing module attachment dep feeds it.
```

`SourceHeads`不读content text，不过滤page_contents.deleted_at（恢复前仍在trash），按给定node ids与notebook限定node，返回kind/page hash/bytes。附件IDs单独一次调用AttachmentInfos.Of（原端口本就读deleted或live），`asset.infos.sql` SELECT增sha256、asset生命周期adapter、bootstrap逐字段转换、page WriterDeps都必须接到。新建附件用Step.Asset，此时blob可能还没执行after callback，不能回查blob取代已有事实。

单元Step S2增`AreaHashes map[uuid.UUID]SourceValue`：一次准备供guard和维护共用，最多10k一块；非空新页用Content、附件用Asset、move/restore仅必要节点批读，0字节页无候选，0字节附件有候选。页/附件不跨kind。

实际探查SQL形状（请求最多10k probes，所有Take之和≤10k；一次新增通常一项、Take=1；实际类型以sqlc生成的既有kind类型为准）：

```sql
SELECT wanted.ord::bigint AS probe, hit.id
FROM (
    SELECT unnest(sqlc.arg(kinds)::text[]) AS kind, unnest(sqlc.arg(hashes)::bytea[]) AS hash,
        unnest(sqlc.arg(after_ids)::uuid[]) AS after_id, unnest(sqlc.arg(takes)::int[]) AS take,
        unnest(sqlc.arg(has_afters)::boolean[]) AS has_after,
        generate_subscripts(sqlc.arg(kinds)::text[], 1) AS ord
) wanted
CROSS JOIN LATERAL (
    SELECT n.id FROM nodes n
    WHERE n.notebook_id = sqlc.arg(notebook_id)
        AND n.source_hash IS NOT NULL AND n.deleted_at IS NULL
        AND (n.kind, n.source_hash, n.id) >= (wanted.kind, wanted.hash, wanted.after_id)
        AND (n.kind, n.source_hash) <= (wanted.kind, wanted.hash)
        AND n.id >= wanted.after_id
        AND (NOT wanted.has_after OR n.id <> wanted.after_id)
    ORDER BY n.kind, n.source_hash, n.id
    LIMIT wanted.take
) hit
ORDER BY wanted.ord, hit.id;
```

`kinds`、`hashes`、`after_ids`、`takes`、`has_afters`五组有类型数组由adapter同一轮构造，长度相等；Take>0、sum(Take)≤10k，空输入不发SQL，变长数组以Planned执行。sqlc不能解析这里的多数组FROM-unnest重载，实际用SELECT-list并列SRF按位置配对，以`generate_subscripts(kinds,1)`给一基序号，adapter转为零基Probe。当前adapter使用的`[]uuid.UUID`参数不表达NULL元素，缺游标用zero UUID加`has_after=false`，续页用末ID加`has_after=true`；两者区分后不漏合法zero UUID，亦不漏最大ID。对非NULL的kind/hash/id，两个tuple上下界固定同一kind/hash前缀并给id下界，`has_after`只排除续页边界。页顺序由四列索引给出；外层至多10k结果的小排序不等于对整个hash组排序。禁止OFFSET。计划断言检查内部Index Cond的notebook及kind/hash/id范围、无全组Sort/SeqScan，不能只看索引名字。

S2-4的首次物理核对发现：原固定kind/hash等式加ORDER BY id在ANALYZE/custom及真实generic PREPARE的exhausted游标上选择主键，检查200,001条索引项，其中另一本200,000条；返回正确不等于工作有界。MATERIALIZED wanted与仅扩展ORDER BY的两个候选仍未修复。当前tuple上下界版本通过无统计、ANALYZE、force_generic_plan三阶段的15个实际计划（定向Go测试39/39）；exhausted末样本只检查来源索引1项，返回10,000的续页末样本检查10,001项。未新增索引、hint或关闭planner选项；每例三次时间均留存，物理计数原件是末样本及另一次EXPLAIN，三次执行中均有断言。阶段回执`p2-s2-4-root-probe-first-green.json` SHA256 `2999d7c6ab336d2a542d0ba069f2903faf2fe05159505ec13443d0c876a43043`只接受首次修复，原SQL回退控制、最终成本与整Step门禁/三审仍按后续实际回执验收。

每个key的完整计划排除集X留在Go map，见3.11。第一次每key只取1个，按10k keys一组；命中不在X立即保存外部见证。若在X，按实际初始source_hash统计该key在X中的个数E，从E减去已经扫到的X候选，下一Take为min(剩余E+1,10k)。贪心装包时再用本包剩余额度截断，所有keys分包总Take≤10k，cursor分别保存，遍历不会从头重查。拿到外部ID停止该key并忽略本页后续行；全是X且返回不足Take则结束为不存在，否则按末ID续页（最后E=0时至多取1个确认）。每key读取至多E+1行，只有全为X的组需要读完E；不会读同hash组的其余无关来源。E可为0，此时一条命中必是外部。未命中/全排除/混合kind、同hash被rewrite大量收敛、恰好在第10,001项出现外部均有测试。不存在按每个被排除节点一轮SQL的N+1：首次批量探1后，余量按上述大页装包。

索引id末键是本协议的必要修订：不靠对三列索引查出整组再排序来模拟游标。单元查重也复用探查端口，X是当前步将加入/更新且需要排除自身的节点；初始E用已读heads，外部ID的duplicate_of仍由单元baseline判。候选中的0字节页不产生key；相同hash的page与asset独立。

更新的两个等长数组同样用SELECT-list并列SRF，不用多数组FROM-unnest；nil bytea表示NULL。当前 `SetSourceHashes` 用单条 UPDATE：`nodes old` 与目标 `n` 按主键连接，在同一语句快照取得旧 kind/hash；保留 n 的 notebook/live/distinct 条件，RETURNING 只交回实际改变的目标，供单元 provenance 合并。S2-1 当时的 MATERIALIZED previous/changed 方案保留历史；S2-4 无统计 HTTP 暴露末尾 CTE join 的二次方比较后改为以下 self-join，物理反例与真实请求见 §10.2。删除前另用 `SourceBaselines` 只读 live 节点派生 key，不取正文/asset 元数据。当前 SQL：

```sql
UPDATE nodes n SET source_hash = v.hash
FROM nodes old,
     (SELECT unnest(sqlc.arg(node_ids)::uuid[]) AS id,
             unnest(sqlc.arg(hashes)::bytea[]) AS hash) v
WHERE n.id = v.id AND n.notebook_id = sqlc.arg(notebook_id)
    AND old.id = n.id
    AND n.deleted_at IS NULL AND n.source_hash IS DISTINCT FROM v.hash
RETURNING n.id, old.kind, old.source_hash AS before_source_hash;
```

每块≤10k，空块不发SQL；没有源区membership变化的rename/reorder不更新。Forget精确为`UPDATE nodes SET source_hash=NULL WHERE notebook_id=$1 AND source_hash IS NOT NULL AND deleted_at IS NULL`，在同一事务内走同一部分索引。

### 3.8 来源移入的工作界与已有大移动（S2，决定36）

新增一个**只供source-entry**用的`Nodes.SubtreeWithin(ctx,nb,id,max int) (domain.Subtree, complete bool,error)`，max为内部1…10,000，原Subtree不改。按层读，但不照搬旧ChildrenOfAll的sort_order/id后加LIMIT——旧索引不支持该排序，可能先扫完整宽层。新SQL复用00042的`nodes_parent_id_name_key_key(parent_id,name_key) WHERE parent_id IS NOT NULL AND deleted_at IS NULL`，不新建reader索引：

```sql
-- name: SourceChildrenWithin :many
SELECT child.id, child.notebook_id, child.parent_id, child.kind,
       child.name, child.name_key, child.sort_order,
       child.created_by_id, child.updated_by_id, child.created_at, child.updated_at
FROM unnest(sqlc.arg(parents)::uuid[]) AS p(id)
CROSS JOIN LATERAL (
  SELECT n.id, n.notebook_id, n.parent_id, n.kind, n.name, n.name_key,
         n.sort_order, n.created_by_id, n.updated_by_id, n.created_at, n.updated_at
  FROM nodes n
  WHERE n.parent_id = p.id AND n.parent_id IS NOT NULL
    AND n.deleted_at IS NULL
  ORDER BY n.name_key
  LIMIT (SELECT sqlc.arg(max_rows)::integer)
) child
WHERE notebook_id = sqlc.arg(notebook_id)
LIMIT (SELECT sqlc.arg(max_rows)::integer);
```

parents只来自本reader已验证本笔记本的根/上一层，不重复，≤max；内外相同take=`max-len(out)+1`，根计数；两处都用`LIMIT (SELECT sqlc.arg(max_rows)::integer)`，使custom plan以运行时值估价，避免低估行数时按整层取数选计划。父数组有限的Function Scan与读取整个孩子层不是一回事。查询经Planned/现有ctx事务，不开snapshot、不关seqscan/bitmapscan。name_key为C collation，同父live唯一，不需id次序；notebook过滤明确放在每父内层LIMIT之后；复合父FK保证正常同父孩子同本，但显式过滤仍拒绝外本输入，不得删除或移回内层改变实际计划。只选Node元数据，不取source_hash/正文或资产端口；堆页可能同时含相邻hash字段，不能说物理页绝无hash。

adapter先FindNodeIn放root，逐层读：返回len(rows)==take时已证明总数>max，立即`return nil,false,nil`，不转换/排序当前超额层、不再读下一层/正文hash/asset。返回不足take说明本层全部父的孩子已穷尽（任何父若被内层截断到take，总输出必到take）；追加这层并继续。恰好max仍以take=1探下一层，空才完整。64层缺陷防护若未穷尽返回内部错误，不标complete；合法最大深度+附件层行为不变。完整后才对≤10k节点按`(Level, SortOrder, UUID字节序)`整层全局排序，逐字段/Level等于同一视图下旧Subtree的结果；不能按父分组、不能改DFS。root层只有root，Height沿原page-only规则。

协议能证明累计返回/独立节点元数据≤max+1=10,001、临时sqlc行slice/domain slice/父数组总O(max)、overflow后无新来源工作；**不能仅由LIMIT证明物理IO/候选访问也是10,001**。期望计划outer Limit→Nested Loop(parent Function Scan, inner Limit→现有partial parent index参数化scan)，无作用于无界孩子集合的Sort/Incremental Sort/bitmap/Seq Scan/Materialize；其是否成立必须真实EXPLAIN/访问范围验证，未达不能以功能返回界冒充工作界通过。完整叶层可能有近10k次空父索引探查，成功10k成本必须单独量。source root/ancestor移动而成员不变不使用该reader。

先FindNode/正常action授权、目的parent与name/position合法、目的祖先判cycle、同层title冲突；复用已知祖先做root/AGENTS保护；确认Before非source、After是source，才用此入口。overflow标`Refused(page.source_too_large, movedRoot)`，不读content hash、不问asset端口、不写/解析。完整时再按subtree height核深度及常规模式规则。**唯一新增次序差异**：一个既超10k又有尚未读到的过深后代的source-entry先答422 source_too_large，非overflow仍按原depth409；把此句写进REST move/MCP move/batch说明及00§4.17。不能声称所有领域409仍先于这道提前工作界。protected与已知cycle/title仍先于此码。

batch已有整本tree物化和模拟subtree不是新有限reader；plan在上述同一检查位置按瞬时tree数source-entry nodes，给正确op，再unit以当前state走有限reader重判；普通`TestPlanBatchBoundsWhatItsMovesCarry`的单move10,001成功保留。两个普通move累计界也保留。锁外计划来源状态与锁下变化仍以unit为准，不绕过domain.Refused/OpFailed。

同步undo和后台开始计划在每个place/aside模拟点数**当前携带的source-entry成员**，同码拒绝，before进入来源时也会触发；锁下`Unit.place`走相同有限reader。后台later batch遇树增加导致越界，以refused/source_too_large保留前批。已有undo全树规划、批首读取/CheckRevertSteps的旧工作仍可能大于10k，本修改只限制新增来源的逐节点工作及单元此处的subtree物化，不宣称给整个旧算法设了内存界。

source-out/区内及source root/ancestor移动沿用原Subtree；membership不变不读hash，清除大source-out的hash按已有Changes切成≤10k写块，不再读取正文/资产。跨批restore整体可大于10k，单批实际恢复成员≤生产100或existing joined segment10k；开始预检按块处理完整计划。归档import总60k、每批100保持，无新的10k整归档限制。没有自动把大的move拆成非原子多次写的服务端行为；错误文字建议用户分几棵子树移入。

有限reader验收的四组为无统计/ANALYZE × custom/generic：绑定实参Planned的实际EXPLAIN ANALYZE BUFFERS，generic另用同连接PREPARE+force_generic_plan+EXPLAIN EXECUTE；GENERIC_PLAN只有形状，不称执行。使用默认planner设置，不走会强关seqscan的辅助函数。保存实际SQL、内外take、plan JSON、Actual Rows/Loops/Removed Filter、heap/index buffers和同连接事务索引计数差；多loop平均rows不可乘完冒称精确全部访问数。

巨大单层以10k、100k、一次串行专项1M活孩子测拒绝，另有前层已9999而剩take=2；目标笔记本固定，另一本更大A→B→A改变并核结果/索引范围不随其候选线性增长，正常parent限定之外另用直接SQL传foreign parent证显式nb过滤。测root-only、max=1、9999/10000/10001、删除孩子、末层附件、跨父sort_order交错/同序id tie/name_key与order相反、最终leaf空探查；完整结果逐字段等旧Subtree。记录MVCC/死项/缓存与B-tree高度影响，不承诺固定物理块差。退化计划或sqlc生成不符带原始证据修订，不自动添持久索引。

三个独立真实反向对照：仅把inner ORDER改sort_order,id（返回界仍对但巨大层访问/计划应失败）；只去outer LIMIT（多父合计越额应失败，单父不足）；parent等式转text（同结果但不可用uuid点范围，计划/范围应失败）。另测complete前Go排序/按父排序/len(out)==max便结束/overflow仍hash读取，各自catcher，不能只fake store。以上都是实施义务，本轮无SQL执行结果。

### 3.9 单元维护、删除、降级与 provenance（S2）

`Unit.apply`经`applyStep`保留同一顺序：准备共享facts/hash → guards → **rememberSourceBefore记住首次改变前的来源key** → ensureChangeset → write → **通用source_hash维护与unit provenance合并** → recordItems → participants。恢复写接收已准备并经守卫检查的`Step.AreaHashes`，按本次恢复IDs把可选值交给`Restoration.SourceHashes`，在`RestoreNodes`恢复live的原UPDATE内同时写hash；缺项保持原列，present nil写NULL，现有ID与删除时刻条件不变。通用`writeSourceHashes`仍执行；恢复已写成相同值时，既有`SetSourceHashes`的distinct条件可以使它不产生物理更新/RETURNING行。provenance不能依赖这次RETURNING才记baseline：单元开始已在trash的恢复者记nil，单元内先删后恢复的原来源保留第一次改变前的key，不被覆盖。同单元后步仍能查到前步hash；来源rewrite只更新hash、不查重，两个既有来源改成同字节合法，索引绝不能unique。此为实施决定45的内部写入折叠；事务、守卫、历史及既有请求/提交/成本界保持。

**预检的纯facts保留（实施决定46，与写入折叠分开）。** §3.11的RR快照、完整X与locked重新检查保持；SourcePrecheck单次check第一遍metadata收集中可私有保留最多10000个NodeFacts出现次数、1500个Step（空Step计数）及1MiB Key/Hash载荷。复制当前Step前判界，任一超限即丢弃已有副本，继续完整第一遍并由后续遍重放原seq；不限制大计划接受、不截断X。Nodes/Place/ParentID/Content/Hash/Key私有复制，不缓存DB读、权限或结论、不跨snapshot/锁；纯契约→duplicate→授权次序不变。1MiB是变量载荷界，结构/切片另受数量界约束，不是总heap/RSS上限。旧仅保留metadata的实现记录保留为历史，当前实现增加这个有界纯facts例外。


完整不变式：node的source_hash非NULL当且仅当live、notebook为llm_wiki、strict source且非空page或asset；page等当前content_hash，asset等blob.sha256。root、空页、区外、trash、standard全NULL。`DeleteNodes`与`DeleteNotebooksPages`各在其现有UPDATE同句清NULL（后者覆盖notebook/workspace deletion）；不能只在Unit里维护。普通模式不重新设旧来源的值。降级`SetMode`后、ModeSubscribers前调用新增notebook端口`SourceHashes.Forget(ctx,id)`（只依pool构造page.NewSourceArea，由bootstrap适配），同事务/同notebook锁；缺接线模块New拒绝，失败回滚mode与hash，不发成功mode事件。

`Write`给guard可读的`Tree bool`与单元私有来源状态指针，和Grants一样不把memo按值复制。来源状态只记本单元已改变/即将改变的id及**第一次改变前**的baseline key，非全库快照。一个key包含eligible live source、kind、hash。查重发现的既存ID：未在memo中改变过时当前就是baseline；在memo时只有baseline key与被比较key完全相同才给duplicate_of。若source被改写再改回baseline hash，ID恢复有效；若先移出再移回且baseline同key，仍可给；否则不给。例：普通旧P移入raw后同hash新增Q，rollback后P在区外，省ID；trash恢复P后Q重复，rollback后P仍trash，省；空来源P本次写A后Q=A，rollback后P空，省；既存来源P由rewrite A→B后Q=B，rollback后P=A，省；既存P=A未变后Q=A，给P。每个例都有真实HTTP/MCP/report或相邻单元反例。

同一步子树内部按Changes原顺序比key，第一个重复的加入者与前项比较；内存命中与数据库命中调用同一个baselineKey==comparedKey判断，不能仅因前项刚进入本步集合就省duplicate_of。同unit新建、移入、恢复用相同算法，不以“是否新UUID”代替来源来源追踪。不同unit（导入前批）已提交的source可指。错误必须在回滚前构造**新**shared.Error，不能修改全局sentinel或把路径放进HTTP problem。

同一步正例（B-M1）：单元起点raw/F/A已是字节H的来源，F为空页。管理员batch把F移出、在区外F下建B(H)、把F移回（A在Changes中先于B）。末步内部B与A重复，A满足起点来源key，错误必须带A的id；回滚后A仍在raw/F/A且H，B不存在。相同动作但起点F/A就在区外时，A不满足来源baseline，错误不带id。两入口MCP同时断言structuredContent.error.duplicate_of与可见路径/无id文字，HTTP和持久report仍遵循同谓词；不能以“内部重复一律无id”作快捷分支。

### 3.10 去重锁与确定性交错（S2）

树单元已持notebook `FOR NO KEY UPDATE`，不取来源advisory。只有实际`Write.Tree=false`的空来源第一次非会话正文，guard在pure rules放过后、索引探查前取`pg_advisory_xact_lock(int4,int4)`；space=`0x73726365`，第二键沿linkIndex的FNV-1a UUID派生，导出`page.SourceLockKey(id)`给测试。仅guard此处取得，不以“OpContent”猜Tree；batch、sync/background revert的content都Tree=true。

次序：workspace SHARE → notebook SHARE → 当前page content row →（如有session验证，其session row）→ source advisory → 后续搜索入队/观察者link index，继承现有queue/index次序。当前生产SHARE来源新增只写一页，证明无第二个content-row等待环；不能把“未来多页按ID排序”说成充分证明。以后多页SHARE调用者必须在advisory前锁齐所需content rows，或使用Tree单元，并重做交错证明。

测试不用sleep：直接持导出的source advisory，`WaitForAdvisoryLockWaits`确认两个empty writers到达；放开后一个成功一个duplicate。两个上传都过Check且已流完，再用notebook行控制先后；后者拒绝且存储文件清理，checkAssets。tree新增与SHAREfirst的两个次序由notebook锁确定。降级与被source lock挡住的first写、来源会话开两个次序，最终按取得的mode拒绝或标准写/开成功。各例结束checkPages/checkLinks/source rebuild；不能把放开一把锁后调度概率当“先A后B”。

### 3.11 历史开始预检与导入（S2）

实际接线复用已有`shared.Snapshots`，不新造事务平台：`page.Deps.Snapshots`目前供Search，仍由`bootstrap/deps.go`的同一个`postgres.TxManager`提供。`app/source_precheck.go`增加`SourcePrecheckDeps{Snapshots shared.Snapshots, Sources SourceReads, Attachments AttachmentInfos}`与`NewSourcePrecheck(deps)`；同步的`Check(ctx, notebookID, notebook, facts, ask)`与后台的`CheckJob(ctx, notebookID, notebook, checked, jobPlan, cuts, ask)`专供**锁外**调用，normal直接返回，否则`Snapshots.WithinSnapshot(ctx, func(sctx){...})`把metadata/E/所有游标读用同一个sctx完成。公开`ask`的类型是`func(context.Context, shared.Action) error`；snapshot内传给它实际sctx，同步、开始与worker调用者必须使用回调收到的ctx，不得捕获外层ctx。page.module构造一次并注入同步restore/revert及Jobs的CheckRestore/CheckRevert/PlanRestore/PlanRevert，不让transfer模块直接连page表。纯模拟函数独立于事务壳，锁内guard只用现有`postgres.DB(ctx,pool)`和同一模拟/探查helper，不调用Check，不嵌套开snapshot。误在已有事务调用Check由既有ErrNestedSnapshot立即报基础设施错误，测试守调用边界。

S2-3的D-I1修复守住这个上下文约束：真实PostgreSQL、MaxConns=1与真实权限SQL证明work/no-work两条路径复用snapshot连接，避免自己占满连接再等自己；实际revertChecks同步/开始/worker三入口另用marker/fake调用记录核对传递。已有OnConn case的权限回调是noop，这两层证据不能合称“OnConn加真实授权加整个server”的一个组合见证。D-I1已在S2-3增量复核关闭（`9bafdaa6`），整S2三审仍待后续。

WithinSnapshot已有REPEATABLE READ READ ONLY、ctx取消、panic/error rollback、WithoutCancel限时结束及连接归还机制，复用而不私写BeginTx。锁外正常请求从pool独立取得一条连接；后台若已有OnConn且无事务，按平台既有规则在该专用连接串行读，不额外占第二条连接。snapshot只覆盖这次来源事实/候选读取和纯模拟，不能包parser、等待notebook锁、enqueue或写单元。测试同时记录source所有查询的backend/transaction snapshot一致、并发rewrite后仍按原snapshot分页、取消后连接可复用、错误会rollback、不发写SQL、guard路径不开第二事务。锁内E稳定性分开证明：Tree单元的notebook NO KEY UPDATE排除别的内容/树写；生产SHARE新增只有本页且已锁content row并已取得source advisory，X的初始key不变。未来多页SHARE必须重审，不能外推此证明。

**CheckRestore admission接线（B-Q1）**：page/app.JobWrites、page.TreeWrites及treeWrites wrapper、transfer/app.Tree、bootstrap.transferTree的签名统一为`CheckRestore(ctx, notebookID, nodeID, admit func(context.Context) error) (string,error)`。既有权限→FindAnyNode/本笔记本→not_in_trash/not_trash_root/expiry检查先完成；随后恰一次调用admit，成功后才读完整TrashShape/group、祖先/heads/SHA/snapshot及source检查。admit成功后通过现有Notebooks.StateOf作一次必要的模式点读（单独计数，不宣称该新入口连模式也无需知道），普通模式随后保持旧名称返回，不加完整group/祖先/hash/资产/source计划或snapshot。LLM模式完整检查采用与PlanRestore共享的只读计划helper（复用已读root，snapshot内复查必要根时间/成员一致性），不调用会重新admit的公开入口。

`StartTrashRestore.Execute`改成现有StartRevert同一release变量/defer模式：admit闭包调用s.d.claim，失败把release复位no-op；去掉CheckRestore后第二次claim。之后startBackground的CountActive/queue room/事务内动作复判、writing/queue/enqueue次序保留；任何后续来源拒绝、读取/取消/排队失败都执行release，成功提交后也释放进程内claim。PlanRestore是worker独立入口，无admit，照当前模式重新做source预检。module/root/wrappers/fakes、start_background与jobs测试同提交更新，不让普通模式busy发生完整来源读。`TestSourceRestoreAdmissionPrecedesFullReads`断基本错误先于admit、busy→完整group/hash/snapshot读0次、duplicate→未enqueue且claim释放、普通→无完整group/祖先/hash/资产/source计划或snapshot读（模式点读单列）、成功claim只一次；真实HTTP/start流程与fake录调用序各自守一层。

共用`sourcePrecheck`在恢复计划/同步RevertFacts或后台3.13的实际名义动作生成之后，开始入口在admit的busy检查之后，但在真正排队之前且在parser/锁预检之前运行（撤销原有trimmed/conflict/step bounds在前）。normal state立即返回。同步`SyncRestoreNode`也做unlocked duplicate探查：在原权限、root/group与历史上限检查之后、正文读取/解析之前新增一次`Notebooks.StateOf`点读，普通模式也会读这一次；仅LLM进入额外source snapshot/元数据检查，普通模式不增加group/祖先/hash/asset/source扫描。它与CheckRestore的admit后模式点读是两条独立入口，不应合称“普通模式零额外读”。锁下guard重判；startTrashRestore/PlanRestore、startRevert/PlanRevert都走同一元数据算法，不给worker遗漏。

预检先得到整个计划可能改变source资格/hash的节点集合X（不仅显式item，也含place/delete携带的后代），保留原全树/group计划的规模：MaxJobItems=60k只限changeset items，不限制一个place的后代，后台trash group也可大于60k。每个source-entry place受3.8的10k界，移出/删除及已有全树读仍不受这一总量界；这是明确保留的旧行为，不引入隐藏的整任务新上限。X自身可大于10k，内存O(|X|)；不把全X发送为一个SQL数组。

按X及计划需要的节点分块≤10k读取heads（含初始source_hash）、attachment SHA256，目标version hash/Bytes用已有RevertOp，不取全文。在一致的只读snapshot内完成这组metadata与外部探查，避免跨分页读到同一组不同时间状态；生产单元已有写事务，预检独立用只读repeatable-read短事务，结束后不保留连接/锁。原无锁计划仍可能被其他写改变，之后单元锁下重查是最终权威。snapshot期间只读，不为排除集建临时表或写数据库。

为计划出现的每个key记录E=初始活来源且在X、kind/hash等此key的节点数；用3.7的游标协议取得第一个不在X的外部见证。总返回行≤K+ΣE（K为探查的不同key数），每次≤10k；全部被影响的来源最多在其**初始key**被排除一次，所以ΣE≤|X|。外部与Go模拟相互分离：X按初始membership/hash建多值集合，每步先pure contract、再查重、再ask manage_source、最后模拟membership/hash变化。先删/移出的不再重复、先新增的算、rewrite改变key、restore先用trash正文、write-back用target hash；内部同一步也依顺序模拟。多个相同key来源必须保留各ID，删一个不能清别人。外部每key一见证始终足够，因为这些ID不被计划修改；不把新出现的sim key误当snapshot外部。每页错误/取消立即停止，无解析、无新全来源扫描。

反向对照：X跨两个10k块，块2的P是某key的首个候选，外部Q排在P之后；计划先移出P再新增相同key，必须指Q而不是P；无Q时必须允许。另测E=0、全X且恰好满页、外部在页边界、混合kind、rewrite收敛100k候选而X仅3个（最多4行）与X=60k（最多60,001行），用端口记录器核对Take总和、无重复cursor/未查询全文，再用真实计划和成本验证。


导入不是异常回滚重试：扩展两个模块的导入值与结果，`ImportedPage`增加`HasChildren bool`（按整ImportPlan的Parent关系算，非仅本批）；`CreatedNode`增加`Duplicate *DuplicateSource`（带已有目标的路径/内部id）与`Skipped bool`。page结果可为“重复叶跳过，没有Node”“重复非叶建空页，有Node且Skipped”“普通建Node”。transfer端不能把重复叶当ErrTooDeep递归drop；只有真过深继续旧drop后代。重复非叶将空facts入库（使用提前解析好的empty facts，无原正文parser残留），子页/附件按该ID继续，跨批父映射保留。重复附件不Attach，已Put的file归dropFiles清理。查重在每个node插入之前，原有insert helpers仍经guards。

报告问题新`duplicate_source`，path为archive项、to为重复目标当时笔记本路径，三种重复都counts.skipped++；空folder不计imported page但实际创建ID与changeset正确。报告/ID map只在事务提交成功后发布，503重试不累计两次；不泄露回滚的同unitUUID作为公共duplicate_of。import可指出同一已准备批中前面实际创建且将一并提交的来源的路径（报告不是被回滚的拒绝problem）；失败/取消以现有最终报告语义处理。现有archive `duplicate`问题是另一项，不能替代新enum。

### 3.12 error、API、MCP、矩阵与文本（各服务端 Step 同步）

| Step | REST操作新增码（已有码只补行为测试） |
|---|---|
| S1 | putPageContent、toggleTask、openEditSession、restorePageVersion、revertChangeset、startRevert：`page.source_immutable` 409；deleteNode增`page.source_admin_only`403；rename/move/revert已有admin_only扩到来源descendant |
| S2 | createPage、putPageContent、moveNode、restoreNode、revertChangeset、uploadAsset、startRevert、startTrashRestore：`page.source_duplicate`409；moveNode、revertChangeset、startRevert增`page.source_too_large`422；startImport无新增4xx（重复skip） |

toggle/session/version_restore不声明duplicate，它们先immutable。S2 too_large不加restoreNode/startTrashRestore，后者是可分批恢复，不是一次move。每个新增operation×code在本模块HTTP适配器实际答出且apitest反向对照通过；不是只加shared枚举。

```yaml
# api/common.yaml Problem optional property, only duplicate code emits it:
duplicate_of: { type: string, format: uuid }
# api/modules/transfer.yaml TransferRefusal, required becomes:
required: [code, node_id, duplicate_of]
# duplicate_of: { type: [string, "null"], format: uuid }
# TransferProblemCode adds duplicate_source; existing duplicate remains.
```

`shared.Error.DuplicateOf *uuid.UUID`及新构造`SourceDuplicate(id *uuid.UUID)`、`ProblemDuplicateOf`可选accessor；httpserver.Problem与problemOf对应，平台不导入shared；`TestPlatformProblemsMatchTheContract`覆盖有/无。`page.Refusal`扩成返回有Code/NodeID/DuplicateOf的值（同步所有调用与测试），不吞unwrap。transfer.Refused、domain.Refusal、background/import worker写报告、postgres refusalJSON双向、HTTP gen转换均增加可空id；旧JSON缺字段读nil；finalizer非refused清整个对象。

MCP `membersOf`增加duplicate_of并放reservedMembers，不能被Extra覆盖；page适配器查找duplicate目标路径写**错误文字**，只有目标仍可见/存在才写路径，读失败保留原duplicate码/ID与可懂基本文字，不升级500、不跨notebook。write/move/batch三个错误路径用同一helper；batch保留op及tool。MCP edit无duplicate（不能first-write）；S1 write/edit/batch有immutable、delete有admin_only；S2 write/move/batch加duplicate、move/batch加too_large；`mcpToolCodes()`与真实答码表对应。`tools.json`只S2改batch描述（最终合成body差异和来源单次移入10k），move描述补10k；不抢P4虚拟路径说明。

MCP增量闭合表（每格在notebook与account两个入口实际调用答出；保留原protected/virtual_file/admin_only等已有码，不覆盖旧集合）：

| Step | 工具 | 新增码 | 可达答码例与证据 |
|---|---|---|---|
| S1 | write | page.source_immutable | 覆盖已有非空来源，断正文/版本不变 |
| S1 | edit | page.source_immutable | 有效非空old_string替换来源，mode先于lock/预算 |
| S1 | batch | page.source_immutable | 一项覆盖已有来源或create后再改变，断实际违规op/整组无写 |
| S1 | delete | page.source_admin_only | writer删来源后代页/附件，admin成功对照 |
| S2 | write | page.source_duplicate | 新建来源或空来源一次性first与已有同kind/hash来源重复 |
| S2 | move | page.source_duplicate | 区外页或附件移入，同kind/hash已有来源 |
| S2 | batch | page.source_duplicate | 移入/新建重复，含B-M1内部baseline正负例 |
| S2 | move | page.source_too_large | 10,001节点区外子树移入来源，普通10001成功对照 |
| S2 | batch | page.source_too_large | 同一source-entry放batch，断正确op/整组无写 |

S1的move/batch已有admin_only扩到来源descendant，write/edit/batch的immutable全部加进各自mcpToolCodes行；S2的write/move/batch全部声明duplicate。`TestSourceMCPToolsAnswerEveryAddedCode`逐格驱动真实客户端并让既有mcpToolCodes断言核答码，adapter/mcp相应测试与权限矩阵独立守scope/action；删表中任一code/一个scope的实际答码/其中一条权限矩阵行必须各自失败。edit不加duplicate，delete不加too_large，不新造历史/后台MCP工具。

权限矩阵沿P1独立wlab：每本播一个非空来源页及source附件，source_hash在S2 seed按真实不变量写；editor对来源rename/move/reorder/delete403，admin成功；create/restore-in writer成功；immutable不因admin放宽。root说明/AGENTS正文与standard对照。矩阵不是源区动作单独API，仍由实际rename/move/delete行证明`page.manage_source`授权。新增失败只用IDs，不加拒绝日志；日志反例检查新增error detail/path不进服务端普通日志。

S1 同时修正 `ErrSourceAdminOnly` 的 detail：管理员可管理来源、改名/移动来源根；不得暗示管理员可删除来源根。S2 `transfer.refusedSome` 中“已不在树里的页”改为节点中性表述，附件拒绝不能误叫页面；富链接在S3补齐。

### 3.13 同步、后台compact与实际批界（S1接结构/契约，S2加hash；决定43）

**三种表示不能混用**：同步RevertPlan.Ops的一个restore表示整个group，调度轮先restores再writes；revertSteps将每个恢复页的write-back放到该member后，并记录aside/content-delete等Joined；Unit.RevertBatch又在本批第一次遇group时一次bringBack本批该group的所有选中members，之后跳过该group其余restore marker。不同group不会合并，哪怕同批。CheckRevertSteps对打开group的root模拟整个组来判前提，其输出也不是实际写入trace。Rewrite/Bytes传播只是输入正确，不能证明执行顺序相同。

**接口与生产算法复用**：transfer/app.background抽出原贪心代码为`batchEnd(steps []JobStep, from int, bounds Batch) int`，backgroundRun.batch调用它确定初始to；已有segmentEnd/weigh/halved不改语义。新增`nominalBatches(steps []JobStep, bounds Batch) [][2]int`从0反复调用batchEnd，得到未缩减名义cuts。只用Nodes/Bytes（默认100/8MiB），第一个完整segment即使超过配置也纳入；Links/历史/单元页数等仍由RunBatch检查，503仍重试同cut，ErrBatchTooLarge才按**step下标距离**halve到合法segment边界。成功prefix之后从新from重新贪心，不把第一次cuts冻结成运行时调度。

```go
// transfer/app type; page/app has the same shape with domain.JobStep.
type JobBatches func([]JobStep) [][2]int
// page.TreeWrites exposes page.JobBatches (alias of app), and existing page.JobStep alias.
CheckRevert(ctx context.Context, notebookID, changesetID uuid.UUID,
    message *string, admit func(context.Context) error, batches JobBatches) (string, error)
PlanRevert(ctx context.Context, spec JobSpec, changesetID uuid.UUID,
    batches JobBatches) (JobPlan, error)
```

transfer StartRevert用BackgroundDeps.Batch.bounds()生成闭包，worker用r.bounds生成；module构造以同一个有效Batch配置接两者，测试同样显式非nil。bootstrap.transferTree把page.JobStep转换为transfer.JobStep再调用传入闭包，返回原cuts；模块不互相import。回调不来自客户端，不执行IO。缺callback或cut缺口/重叠/空片/越界/切开Joined是内部错误，不隐式成功，不新增产品码。普通模式检查callback合法存在但不调用，不为来源构造名义计划；worker原本必需的compact构造照旧。

`revertBounds.BackgroundBatches`为后台可选内部参数，同步为nil；后台入口必给。checks.check保持message/权限/changeset/trimmed/CountItems→admit→items/versions/nodes/groups→domain计划/conflict/step bounds的先后。取得notebook state后，LLM后台构造一次revertSteps(c.ID,checked)，使用名义cuts运行契约，随后保留原locked预检。同步用原facts；不让后台再先执行一份同步来源检查。checked可带构造好的JobPlan供PlanRevert复用；普通CheckRevert不额外compact，普通PlanRevert仍在原位置构造它原来需要的plan。S1用已有state/Bytes/相关祖先完成纯来源契约；S2在同点加CheckJob的hash/snapshot，不在S1提前查询尚未存在的source_hash。

**共用实际结构顺序**：page/app新私有`revertActions(steps []revertStep) []revertAction`，action含原step index；restore action再带此批同group的restore step indices。遍历时非restore按序产出，第一次group收集本批该group全部恢复成员，后来该group marker不再产出。用成员索引保持原contiguous preorder范围，不能把跨批的组尾拉进来。Unit.RevertBatch用该遍历器驱动原place/Delete/restoreGroup/WriteContent调用；保留原batchChangeset/CheckRevertPart/continuation/CheckRevertSteps/parsedAs、当前Group/States/retimed/frontier与skips处理，不拿预检初始state当锁内payload。helper只是结构枚举；实际member的状态、跳过、revision、trash时刻仍由现有锁下读与检查决定。共享遍历器不暴露私有compact plan给transfer，也不让只有Nodes/Bytes/Joined的公开JobStep伪装成完整动作。

`checkJobContract(checked, jobPlan, cuts, notebook, ask)`（S1）与`SourcePrecheck.CheckJob`（S2）都按cuts逐片运行同一结构顺序，事实从当前**模拟**state按选中members与当前操作生成：restore用trash hash/bytes/当前模拟位置，content用目标base hash/bytes与被撤销r.Rewrite，place/aside/delete依实际步。每片开始重置group-begun，跨片保留sim树/hash，不把group未选中的未来成员提前恢复。S2整个metadata/E/外部探查及模拟处于同一readonly RR；X含完整计划会影响的节点。开始/worker拒绝的duplicate_of只指出本次snapshot中已存在且匹配的来源，不把模拟前批当已提交；实际RunBatch每单元重新捕获真实baseline，后批可指先前已commit的来源。

**承诺范围与取舍**：名义预检只验证当前未缩减批计划，不证明所有合法cuts。一旦真实RunBatch因历史/页/链接等既有上限返回ErrBatchTooLarge，保持原halving；随后实际guard仍可能拒绝来源，哪怕没有别人修改。前批保留、本批回滚；反方向名义计划拒但更细可行也仍拒，不搜索分法。同步可能成功而后台被拒。保持已验证M9原子segments/资源界比枚举所有不可预知分批更可控；主持者已接受该取舍43，负责人可改判；原三位设计复核已完成，S1结构/契约与trace已落地，实际记录见3.14/10.1；S2真实duplicate与缩批结果验证仍pending。

**必要例和证据层次**：

1. **A-I1原公开历史例的实施更正**：原“同批创建S1/S2再deferred rewrite，undo得到C且S1保留rewrite=true/base B”的具体链不成立；创建与rewrite合并后的base=nil、rewrite=false，同步undo也不启用link rewrite。S2-3没有改变生产撤销来凑该例。真实回归`TestTheSourceCreationRewriteUndoKeepsItsRecordedBase`核对实际版本链及duplicate、`duplicate_of=nil`、零入队；旧失败fixture保留在树外记录。下面合法模块历史fixture证明compact/cuts可得相反结果，是不同证据层，不称原公开链已复现，也不推出所有其他公开链都不可达。原设计审查当时的A-I1接受与设计层resolved记录保留，并追加[实施更正](reviews/P2-design-review.md)。
2. 合法模块历史fixture同group `[restore F, restore A(A), content A→B, restore B(B)]`，A恢复/content Joined：大批[0,4)先恢复F/A/B再rewrite可过；配置Nodes=3的合法cuts[0,3),[3,4)则后批duplicate。真实halving**另按源码**：[0,4)失败先缩[0,1)提交F；余下[1,4)失败缩[1,3)提交A+rewrite；[3,4)的B拒。把已有history上限设为使相应prefix可过而大批超限的合法fixture，验证实际ErrBatchTooLarge/halved输出，不能只stub错误然后宣称真实历史界已触发。
3. 同group反方向fixtureA(H)写回J，B(H)：大批先恢复A/B重复所以开始拒；细分先A→J再B可过。它证明“最细segment最严”为假，不作为绕过开始拒绝的API。模块fixture的groups/revisions/rewrite/bytes必须合法；未用公开入口构造的形状标模块证据，不称真实API已复现。
4. 两独立group同/跨批、同group同批/合法cuts、首segment超配置仍纳入、连续halving/新from再贪心、content-delete/aside joined不可拆、continuation skips/retimed/frontier继续核对。独立oracle手写动作/拒绝层/计数/持久状态，不以revertActions或sim输出验证其自身。

### 3.14 S1 实际落地、差异与初审修复

S1的实现头与最终验收状态见10.1；以下是已提交事实及其边界，不把S2/S3计划提前写成完成。

1. 后台私有helper实际分为`revert_actions.go`、`revert_job_contract.go`、`job_batches.go`与`revert_source_lines.go`；事实在app/domain各自的`source_facts.go`。类型为`StepContent`、`Step.Kinds`、`NodePlace.InArea`、`NodeFacts.Kind`。附件保持`Step.Asset{MIME, Bytes, SHA256}`，其Step.Content为nil。
2. `StepLines.AfterParents`补恢复跳过活父节点后的森林；每个外父独立，Changes中的当前父图优先。`TestSourceRestorePartKeepsEachRootLineAfterASkippedParent`与`TestSourceRevertContinuationKeepsEachRootLineAfterASkippedParent`是合法模块执行见证；`TestSourceJobSimulatorUsesCurrentLinesForEachRestoredParent`只是纯动作模拟，不冒称其任意交错来自名义cuts。没有据未获实证的缓存疑点增加失效逻辑。
3. 写单元先补/复用来源事实再进guard；会话否决者在锁下经SourceAncestors读当前祖先。普通模式零额外来源读/管理授权，保留原响应读。content-only历史只批读相关线到RevertState.Lines，不读整树。后台实际动作与同步Ops分开；active-background→sync在真实S1场景存活，事实trace能区分结构，S2duplicate结果差异仍待完成。
4. P1§9的真实降级覆盖由08fa补充：同一非空来源先由writer/admin changed写拒绝，真实降级后原父位置writer REST与notebook MCP各实际写一次并核SQL版本。首次补测即绿；no-op请求负控单列test-input，不虚构生产功能红→绿。6fac修复矩阵独立trash与整本删除共时的test-fixture缺陷，增加严格时间序/垃圾组/非空集与checkPages catcher，保留08fa门禁失败。
5. API/MCP与双语/fake随S1交付，完整来源UI仍S3。LW4/LW5分别6与5项API/MCP；没有对应MCP工具的task/session/version/undo/reorder使用API+Go，rewrite的undo明确API bridge。C初审发现历史fake写回使用请求前位置（C-M1，Minor），另补充确认删除新建祖先前未反映root已移出的管理位置（C-M2，Minor）；真实服务对应对照通过，fake业务断言失败。第一组C-M1/M2已在b750同组提交并增量核验，该增量阶段未作整步接受。原C随后在b750诊断确认C-M3 Minor：简单现活祖先先逆移入来源后，fake仍按原树放行后代逆改名；真实editor拒绝。child UUID先时，真实editor同步/worker先区外逆改名再移入且成功，是必须保留的正对照。第二组C-M3修复须反映每个管理动作之前的位置，不能按全局最终树或current/final OR判断；不改后端产品规则。各ID分别验收，不把方向/数组顺序拆新发现；修复及原C复核见[实施审查](reviews/P2-S1-source-rules-review.md)。第二组C-M3仅修改Web历史测试设施：现活简单placement按node UUID排序，每步在当时node/树上先管理检查后移动，保留本动作Before权限；不按历史数组、不用原树OR最终树。第一遍不提前移除created/restored，placed避免第二遍重做原树检查；删除/正文保留第一组已证阶段位置，blocked残余保留原管理后备，不模拟完整key/depth/aside/conflict scheduler。第一组C-M1/M2提交b750保持历史，三个ID已由原C正式复核分别关闭

## 4. 网页（S1/S2 跟随契约，S3 完整界面）

### 4.1 类型、假服务端、单一来源判断

S1即生成新增problem codes，补en/zh-CN与problemMessages映射、假服务端实际拒绝；S2同样补source_duplicate/source_too_large、Problem.duplicate_of、TransferRefusal的可空字段与duplicate_source报告枚举。`messages.test`的键和占位符、problem code覆盖、服务端类型生成、假报告fixture同步，不把文本和生成链欠到S3。API仍返回普通Page/Asset；source是按当前notebook/tree派生的身份。

S3在`PageTreeStore`建立**全节点**索引（直接由既有nodes建，含附件），保留页面专用tree/byId供页面树渲染；新增`nodeById`、`nodeAncestors`及`isSource(id)`或同等一个公共判断。后者只判当前mode=llm_wiki、root存在、严格祖先含root，root自己false，祖先缺失/循环时false且现有刷新流程处理未知节点。不按标题raw、不只看直接parent、不让每个组件重写公式。MobX computed依赖当前notebook mode/root与nodes；改名/移动root、嵌套来源、附件、降级都即时重算。

来源行/附件图标有可读的“来源”文字等价（title/aria标签遵循现有组件）；AGENTS只有笔记本根的那个有身份。现有S4 `nestedAGENTS`在raw下的fixture仍证明嵌套同名不是特殊AGENTS；因它是来源，菜单/拖放按来源writer权限收紧，不能为了保留旧“full menu/draggable”断言而错误放行。另加普通区同名页保留原完整菜单的对照。

### 4.2 管理、正文和入口

沿现有角色能力判定，不增加客户端授权来源：writer可新建/上传/移入来源，但来源现有节点rename/move/reorder/delete菜单与拖放关闭；admin可管理，来源根删除与祖先删除仍protected，根改名/移动沿P1。来源root说明、AGENTS说明和普通页面继续编辑。页/附件的权限使用同一个isSource，附件区rename/move/delete不遗漏。 writer携带来源根的普通祖先仅限制换父，不封同父排序或改名；admin可换父。实际`canMoveNode(pages,id,parent)`供目的地选项、DnD命中与队列末跳共用（A-M1修复），后端仍最终裁决。

正文来源页显示只读横幅“来源保留原文，可作为整理材料；需要修改时由管理员移出来源区后修改”。隐藏Edit和Mod+E、Markdown任务checkbox写操作、历史“还原此版本”、锁提示的“在这里编辑”。不仅隐藏按钮：快捷键和handler也检查同一条件；task渲染传入只读，不能靠点击后409当正常产品流程。历史仍可查看/比较/复制。来源页里的未解决链接点击若只是创建**别的页面**而不改当前正文，保留现有创建流程；落在来源父下的新页仍走标题/创建规则，不给此链接入口修改来源的例外。附件上传是新增，来源页下仍可添加。

writer将区外页/附件移入来源时，树DnD、移动到对话框、附件移动都给同一确认：“移入后你将不能编辑正文或管理这些来源；管理员可以移出。”确认前不发写；取消不变。判断用待移动根的before与目的strict source区域（目的为root也算将进入），整棵只有一次确认。admin免确认；已经来源内的writer没有移动入口；普通区对照无确认。pending/mounted/location防护沿原移动流程，重复提交受原busy状态约束。过10k由server答source_too_large，文案建议分几棵子树，前端不假设自己当前树数量是服务器权威。

### 4.3 编辑会话失去来源权限时真正结束

`edit-session.ts`的EditLost加`source`，lostBy识别page.source_immutable；`page-editing.ts`保存收到该码进入已有settle/lost路径，停止autosave与heartbeat，不把它当可重试普通failure。保留本地尚未保存正文供复制和“返回阅读”。`edit-lost-banner`中英分支解释页面已经成为来源。

修复具体生命周期：来源loss在清id之前捕获当前session id并交给一个幂等结束过程，标ended并存ending promise；lose只负责lost与unfollow，不再使结束请求失去id。若open仍在途中，opened见ended后按原late-end模式释放新返回id，不把它设成活session；end/组件卸载和source loss同时发生只发一次有效end。其他lost reason保留原既有语义，不能对别人接管后的会话发无关写。

UI立即进入readonly，即使结束网络请求失败也不继续写；正常连接下真实end请求完成且服务端session lock消失。沿用现有end失败吞掉/租约过期的网络故障处理，不声称断网时可保证立即释放，不引入后台无限重试。使用实际editor registry→lockReadOnly→controls.setReadOnly链的测试验证readonly/editable改变；只断言session.lost字段不够。覆盖保存response与route卸载并发、open迟到、重复source loss、unsaved copy、heartbeat/autosave停止、server get-lock已无session。

### 4.4 添加来源：粘贴、文件与持续队列

来源根/来源页的添加入口给“粘贴Markdown”和“上传文件”。粘贴对话框先填写合法非空标题与正文，一次`createPage{parent_id,title,content}`提交，不先建Untitled再PUT；409 title_taken留在当前表单让人改，不自动编号重投。textarea会把输入换行规范化，帮助文字明确这一点；需要保留原始字节用上传.md。与原表单一致校验标题上限/空白/保留名，最终服务端仍裁决。成功只在mounted且location仍为起点时导航；否则仅刷新树，不抢用户位置。modal关闭中止尚未发送的粘贴表单操作，已提交后的反馈按既有mounted防护，不虚构撤销创建。

文件路径严格保字节：仅对Markdown正文文件按`File.size <= 5*1024*1024`检查（0字节合法），`.md`大小写不敏感；普通附件仍各自原有上限；`arrayBuffer`后`new TextDecoder('utf-8', {fatal:true, ignoreBOM:true})`解码，保留BOM为U+FEFF，保留CRLF/CR/LF与结尾换行，不trim、不parse/stringify Markdown。拒非法UTF-8，不用替换字符悄悄改字节；重编码UTF-8字节数须等于读取buffer长度，作为实现断言/测试而非用户额外请求。文件名去最后.md扩展作建议标题；为空/非法先在该上传行的标题表单处理再发送，所有Markdown新建均先确定标题；服务端title_taken保留该行待修改，**不会**走附件已有最多3次自动编号重试。普通文件保持附件API、各自资产size上限与原重名规则；同一次多选.md+附件分别走对应端点。

上传归现有每本唯一`AssetStore`，modal只展示/启动，关闭不取消；RootStore generation变化/logout卸载统一abort，upload rows沿原beforeunload提示。扩展值用可判别结果而不破坏编辑器附件调用：

```ts
type UploadResult = { kind: 'asset'; asset: Asset } | { kind: 'page'; page: Page };
// Upload has target: 'asset' | 'source-page', result?: UploadResult;
// AssetStore.upload(...) remains attachment-only for editor/drop callers;
// new uploadSources(parent, files, terms) owns mixed-source rows;
// AssetStore.uploaded(upload): Promise<Asset> rejects/nonaccepts source-page rows.
// PageService.createPage(nb,parent,title, options?: {content?:string; signal?:AbortSignal})
// PageTreeStore.createWithContent(...) uses its existing queued write/re-read path.
```

Source-page行通过PageTreeStore排队创建，队列任务发送前再检查abort（modal关闭不abort，generation变更会abort），signal透传PageService。成功page结果调用树已有重读并dismiss/导航条件，不送进asset列表分页“找uploaded asset”循环；附件仍用原read/refresh/paging。不能把Page塞到`Upload.uploaded: Asset`、让编辑器的`uploaded(upload)`返回联合类型再到处强转。上传行取消、重试、进度、失败与名称编辑按target分支，读取文件阶段可显示准备中，不伪造网络百分比。MD只在来源上传入口新增支持；编辑器附件拖放原来拒.md的行为保持。S3明确测试关闭对话框仍完成、两种generation取消、混选互不阻塞、409只改一行、来源移动后server最终裁决。 失败Markdown行的新显式retry按当前notebook角色禁用/恢复控件，submit及caller-provided beforeSend在排队真正发出前再查当前角色；同一行/File/草稿保留。已发上传与父移动的服务端终裁不变（B-M1）。

### 4.5 重复说明、附件路径和历史提示

新增统一`duplicateText(problem, pages, navigate)`（返回ReactNode或可供组件观察的模型）用于source_duplicate；读取optional duplicate_of，没有id用“本次操作包含字节相同的来源”的专门文案，按调用上下文另说同步“本次操作未写入”或后台“本批未写入，前面已完成的批保留”，不能展示回滚UUID。先从全节点索引生成**完整可读路径**；有id但当前树不能解析为有效来源目标时（不存在，或缓存中尚在区外）触发一次去重的pages.read，重读后仍无则显示“重复来源目前不可用”，不显示裸UUID、不无限重读。点击前复核当前mode/tree/目标；page用page route，asset用现有`openOf`的parent+attachment focus，不硬拼`/pages/<asset>`。各异步回调保留mounted/location约束。只读到目标的当前可见信息，不用Problem带路径跨越权限。 C-M1补旧树中已存在区外page/asset后来移入的真实场景；不通过取消来源复核把区外目标当可导航来源。

接入位置逐个：添加粘贴表单；source upload rows（附件区与拖放）；树DnD Alert；MovePageDialog；Asset move；trash页与preview批恢复；RevertDialog；changeset-page的carried outcome；TransferReport.Refused及duplicate_source问题的to。`form.explain`、Alert已可收ReactNode；`upload-rows.failureText`、trash refusal helpers和report当前string返回要在调用处换成可渲染内容，普通errorText保留string给现有非UI用途，不能全局改签名造成无关连锁。被拒node_id与duplicate_of分开，前者可为子树内附件；两者都用全节点路径。

同步撤销、后台报告保留偏严解释：临时步骤也守来源规则，管理员不例外修改已有正文。可执行提示是“需要还原正文时先移到来源区外还原，再移回；重复时先处理冲突来源后再恢复/移回”。重新移入仍校验重复，不能承诺必成功；也不把这些动作拼成仍会拒绝的一次batch。后台撤销另解释“后台撤销分步处理，处理顺序可能与一次撤销不同。本批操作未保存，已经完成的部分仍然保留；请按来源规则手动处理冲突后重试。” 不把nominalBatches/halved/Joined写进产品界面，也不声称只有并发变化才会拒绝。页面测试按真实操作顺序验证区外还原可行、移回被尚存重复拒绝，消除冲突后成功，且无管理员正文豁免。

### 4.6 S3 实际落地与设计差异

S3以`8904a8769646c3b8919ff6a4c503eeae3ac70727`为入口，四个增量c877/4246/98bc/1da和审查修复`8c17b5cceb65b2a0f70976bbe4752daeaabaee05`分别记于10.3及审查记录。全节点nodeById/nodeAncestors/isSource保留页面专用树；source身份是当前mode/root/祖先的派生值，不按标题raw。writer来源管理、根与根AGENTS正文例外、同父排序和携根祖先换父的区别由共享能力判定，最终服务端仍授权。

PageService.createPage的content/signal、PageTreeStore.createWithContent的排队/重读、AssetStore.uploadSources/retrySource的持久行共用既有生命周期；页面/附件结果分型而编辑器仍只收Promise<Asset>。树队列前重验signal和调用方beforeSend，失败MD新retry另验当前role；普通Untitled排队后父变来源时取消该隐式创建。父移动后的已开始文件请求仍按服务端终裁，未增加客户端来源位置否决。

duplicateText使用全节点完整路径、当前openOf与一次去重补读；source无效不仅是UUID不存在。树Alert在导航前消耗以完成焦点交接，保持共享读者动作保护。changeset carried renderer虽接线，自然source409仍留RevertDialog；不伪造route state作为consumer实测。

决定48按P2两入口改双语raw模板一句，当前659/605 B，AGENTS2209/2107 B，总5580 B；存量正文不变，P1旧测量保留。A/B/C三Minor和C的预算/证据Q由原席核对；C-location单守卫0/0/0存活按防御冗余保留，不声称所有候选均被kill。完整证据与集成边界见[S3审查](reviews/P2-S3-source-web-review.md)。

## 5. 端到端故事与数据库断言

现有`e2e`故事命名/fixtures沿用，不另造wiki断言服务。notebook/page/transfer的数据库助手分别加模式/来源hash、revision.rewrite、refusal.duplicate_of断言；标准笔记本原故事全部保留。每个真实拒绝核对响应+持久状态，无假服务端代替。

| Step | 故事与入口 | 必须发生的行为/证据 |
|---|---|---|
| S1 | LW4 API、notebook/account MCP对应写路径 | writer建来源正文后PUT/edit/task/version_restore/session均按可达入口拒；管理员也不可修改；系统链接重写真实触发，版本rewrite=true，undo及undo-of-undo保持true并成功；同字节version_restore/有效旧session也拒；root说明/普通页对照可写。MCP无task/version/session工具的部分以API+Go补足，不造工具。 |
| S1 | LW5 API+MCP | writer来源改名/移动/排序/删除拒，admin成功；普通节点移入允许并变只读，移出须admin；move+rename同次都判；来源内嵌套AGENTS不冒充根身份。 |
| S2 | LW3 API+MCP（附件为API） | 同笔记本同kind同bytes重复409，空页允许多个、0字节附件重复，换标题/路径不能绕；不同kind/不同笔记本/区外/已删允许。two-in-one回滚无duplicate_of，已提交来源带id；移入/restore/首正文/rewrite四类旧节点provenance用Go全链补齐。 |
| S2 | LW5重复步骤、LW6/LW7 API | 移入含附件子树重复整组无写；同步restore/undo source新增拒并回滚，偏严顺序有解释；start拒不排队；任务批间新重复当前批refused且前批保留，report实际id正确；import叶skip、重复非叶空folder后代继续。LW6/LW7无MCP历史/后台工具，保留API+Go豁免。 |
| S3 | LW3–LW7 页面 | 粘贴/文件精确字节、混选/关modal、只读所有入口、角色菜单和writer移入确认、重复页与附件链接/旧树重读、会话loss真实release、历史/任务报告提示。沿用S1/S2相同的服务端断言，页面不靠静态快照。 |
| S3 | LW12扩展既有跨标签故事 | 保留第二context原settings页的stream identity、server不重新连的证据；该context进入真实来源页面，在第一context降级后不手动reload，观察原body可编辑并成功保存；再在原来源位置创建相同字节成功且hash全NULL。现有排队import/另一笔记本阻塞对照保持。 |

`settleEvents`会写sentinel，P2必须把写入目标固定为AGENTS说明或可写普通root页，不可再拿已有来源正文当探针。降级前等待mode/tree事件用现有可观察条件，不用固定sleep；降级后确认同一连接序列/页面实例。测试账号writer/admin分开，不用全管理员假装权限覆盖。每个故事的缺口与Go替代证据在结果§10列出，不能泛称“E2E通过”掩掉入口豁免。

## 6. 按设计顺序提交，各提交可独立为绿

每Step独立`m10-p2-sN`分支/worktree（遵循用户指定workflow），开始基线为上一Step**合并及记录CI全绿**后的main；本文完整Phase先审查，代码期间不另写下一Phase设计。每个提交包括其直接测试，红测先在worktree验证失败再实现、完成后一并提交绿色状态；不提交临时绕过或只有声明而服务端无答码的接口。父agent唯一提交/推送/合并，作者不得提前实施。

| Step/顺序 | 提交的完整增量 | 本提交不能留下的半链 |
|---|---|---|
| S1-1 | 00049、SQLC、真实/fake AND、读取/compact undo rewrite+Bytes、历史反例与旧标记说明 | 所有版本生产者明确false/true，undo-of-undo实际持久通过；暂不激活来源规则 |
| S1-2 | StepContent/Kind/Lines、producer与直接import、batch逐项metadata、RevertFacts无Tree事实；共用实际后台revertActions/原batchEnd、JobBatches参数与所有wrapper/fake | 普通模式零附加读/授权；此时旧guard仍可忽略新字段，不能导出无人用helper触发knip/lint |
| S1-3 | 完整source纯规则、no-op窄检查、MCP锁前检查、guard/sessionveto注册、同步及后台名义动作预检、API/闭合MCP码表/双语/生成/fakes | 每操作每新增码已真实答出；全程序最后一跳与角色矩阵同提交；不能只开REST不管MCP |
| S1-4 | LW4/5 API+MCP、必要mutation catcher完善，重门禁/审查修复；完成文档 | 功能证据补足后才Step done，审查发现修复单独提交并复核 |
| S2-1 | 00050最终四列索引/回填、source heads/probe/asset SHA/写回端口、单元hash维护、两delete及Forget接线 | 同提交建立全不变量与重建测试，不允许合并“有列但部分写不维护”的中间状态；尚不激活重复拒绝 |
| S2-2 | 有限source-entry reader的真实LATERAL/双LIMIT与完整后整层排序、计划/范围反向对照、plan/单元/history全入口10k界、too_large完整API/MCP/文字/报告 | 必须原普通10001正例、SQL输出界与无阻塞范围访问分别验收、source越界precheck+last-hop同时完成 |
| S2-3 | 去重、provenance、advisory、CheckRestore admit与完整预检、后台名义动作上的hash/snapshot、开始预检游标模拟、import跳过、全部duplicate字段管道、API/MCP/文字/fakes | 这组不可拆成对外字段已声明但无实现的提交；每种可达码及report读回链均完成 |
| S2-4 | 真实交错/最后一跳/随机重建/计划与成本、600万迁移实测、LW3/5/6/7、升级文档 | 测量先出原始样本再定新预算，不改旧timeout；发现问题回修对应实现并重测相关范围 |
| S2-5 | 门禁、变异、独立代码审查/核对，精确文档投影 | 全证据合并后M8尚待S3网页验收才能done |
| S3-1 | 全节点来源判断、文字等价图标、菜单/control/shortcut/task/version只读、writer移入确认、nestedAGENTS修订 | helper与全部消费点一起；普通/root/AGENTS对照仍通过 |
| S3-2 | source loss结束session、实际editor只读与unsaved行为 | 不只更新banner、不遗漏迟到open与卸载 |
| S3-3 | 添加来源标题/粘贴、AssetStore source-page队列、精确.md与混选 | 结果类型、signal、列表刷新/关闭继续/代际取消一次完整落地 |
| S3-4 | duplicateText全表面、附件/stale-tree导航、历史提示、LW3–7/12 | report的两种node指向明确，page与asset都真实验证 |
| S3-5 | 门禁、变异、独立审查/核对、Step和Phase最终文档 | P2done/main CI全绿，M10继续P3；不提前开始M11 |

每Step在代码完成后执行§7全部门禁；三份独立代码审查覆盖接口/规则与历史、数据/并发/成本、Web/API/E2E，主持者逐条定处置、作者修复、审查者复核修复，不能用本设计审查代替代码审查。按workflow完成文档→`--no-ff`合并→精确merge SHA的实际workflow全部CI作业绿（四分片配置为11项）→合并记录提交及其精确CI绿→用这两个实际main Test样本核算bootstrap均值，才进入下一Step。main只推完成核对状态，从不强推；失败先修，不留下“CI以后再看”的进度。

## 7. 测试、变异、测量与门禁

S1以下列真实测试名为准，已核的6fac执行与修复后最终执行分别记在10.1及[审查记录](reviews/P2-S1-source-rules-review.md)；S2/S3仍为将新增的验收契约。模块app/domain完成组合矩阵，bootstrap证明接线、跨模块持久行为与真交错，不以源码中存在名字代替实际选择数。

### 7.1 功能证据与反例

| 层 | 主要测试/观察 | 必须覆盖的正负对照 |
|---|---|---|
| domain/app S1 | `domain.TestSourceContractUsesEachFact`、`TestSourceFactsKeepsMoveSidesSeparate`、`TestSourceFactsRootAndAncestorMovesKeepStrictDescendants`、`TestSourceFactsRestoredForestUsesEachExternalParent`；batch的`TestBatchSourceContractJudgesEachActualChange`/`TestBatchSourceContractKeepsMergedGuardFallback`；app的`TestSourceStepMetadataAddsNoReadsInStandardMode` | strict root/深度/附件、三拒四准与Merged后备、零额外读/授权；全部producer含direct import；两条真实模块forest/continuation另列3.14 |
| postgres/app history S1 | 两包`TestRevisionsKeepOnlyAllRewriteWrites`；postgres的`TestSourceHistoryReadersKeepTheRevertedVersionsRewrite`；app的`TestSourceRevertsCarryRewriteAcrossBatches`、`TestSourceContentOnlyRevertReadsOnlyRelevantLines` | SQL/fake AND四格、full/Of分别读被撤销r、compact/写回/undo-of-undo、无Tree相关祖先与瞬时content |
| bootstrap S1第二份与app预检 | bootstrap的`TestSourceRESTWritesReachTheRegisteredGuard`、`TestSourceSessionsAreVetoedBeforeLocks`、`TestSourceMCPPrecheckBeforeEditLocks`、`TestSourceMCPPrecheckBeforeParseBudget`、`TestSourceMCPBatchPlanBeforeEarlierEditLock`、`TestSourceDescendantRefusalKeepsAnEarlierCommittedBatch`、`TestSourceMatrixSeedKeepsItsTrashGroups`；app的`TestSourceNoOpsKeepTheirDifferentContracts`、`TestSourceMCPChecksBeforeLocksAndBudgets` | REST/MCP答码、旧session、no-op与changed末跳分开、锁优先序、真实后台层次/前批保留、矩阵fixture时间/垃圾组 |
| bootstrap S1第三份/既有降级测试 | `TestTheSourceRewriteSurvivesUndoAndUndoAgain`、扩展既有`TestTheDowngradeEndsTheContract` | 真实版本链/两次undo与根说明/AGENTS对照；真实降级后同一来源REST/MCP写与SQL历史，S2重复释放/S3页面仍待 |
| postgres/migrations S2 | `TestSourceHashSchemaAndBackfill`、`TestSourceProbesFollowBoundedCursors`、`TestSourcePrecheckKeepsOneSnapshot` | 31/33 CHECK拒、nil/32允许、索引非唯一/谓词、existing LLM页+asset/空/trash/普通预根；跨10k X/E=0/满页/全X/后外部/取消/只读 |
| app/domain S2 | `TestSourceHashTransitionsMatchRebuild`、`TestSourceDuplicateProvenanceUsesTheUnitStart`、`TestSourcePlansSimulateEveryHashChange` | 所有hash变化、两delete/Forget、四类非新UUID回滚provenance、完整顺序、source_too_large与普通10001 |
| bootstrap S2第二份 | `TestSourceDuplicatesReachEveryLastHop`、`TestSourceDuplicatesCarryTheirTargetEverywhere`、`TestSourceEntryStopsBeforeReadingTheWholeSubtree`、`TestSourceDowngradeClearsHashesAtomically` | 真交错与HTTP/MCP/report全链、附件、overflow真实LIMIT、Forget失败回滚与事件 |
| bootstrap S2第三份 | `TestTheSourceWritersSerializeInBothLockModes`、`TestTheSourceImportKeepsDuplicateParentsChildren`、`TestTheSourceRebuildMatchesSuccessfulRandomSteps` | 上传败者文件、两first/树写/降级次序、跨批import空folder、独立oracle与成功次数 |
| S2恢复折叠追加 | bootstrap的`TestSourceRestoresWriteEachNodeOnce`、`TestSourceRestoresRollBackWhenTheBlobObserverFails`；postgres的`TestRestoreNodesMaintainsOptionalSourceHashes`、`TestRestoreNodesRejectsMalformedSourceHashesWithoutChangingRows`、`TestRestoreNodesRollsBackHashesWithTheGroupAndHistory`；app的`TestSourceRestorePreservesTheUnitsOriginalDuplicateTarget` | 真同步/后台restore/revert四入口×source/outside/standard的物理trigger；可选hash缺项/显式NULL/乱序/删除时刻范围；节点、正文、版本、items及末端asset observer失败的事务回滚；起点trash与单元内先删后恢复的provenance。最终hash正确与物理一次写分开证明，非法长度只声明拒绝且最终无行变化 |
| S2有界facts保留追加 | app的`TestSourcePrecheckBoundsItsFactReplay`、`TestSourcePrecheckFreezesYieldedFacts`、`TestSourcePrecheckFreezesParentValues`、`TestSourcePrecheckDoesNotReuseFactsBetweenChecks`、`TestSourcePrecheckCountsKeysAndHashesAtTheReplayBoundary`、`TestSourcePrecheckReplaysTheDecisiveTailAfterOverflow` | 出现次数/空Step/payload精确边界、全部引用私有所有权、同实例新check不复用、1MiB/+1含合法32-byte Hash、超界尾部protected与duplicate仍拒绝；两次race与非race变异分别记账 |

| Web S1/S2 | 既有problem-messages/messages、API生成与fakes测试 | 每码双语、占位符、report nullable字段、too_large文字，不延到S3 |
| Web S3 | `source-area.test.ts`、`source-actions.test.tsx`、`edit-session.test.ts`、`page-editing.test.ts`、`source-uploads.test.tsx`、`duplicate-text.test.tsx` | 全节点动态来源、每control、真editor只读/end id、MD exact bytes/UTF8/BOM/size、409/导航/持续队列、所有rich surfaces与asset/stale id |

权限矩阵新增source行、每REST operation×code的adapter实际答码、MCP两个入口实际答码各有独立失败对照：删矩阵行/删code声明/处理器吞通用码/Extra覆盖duplicate_of分别失败；shared.Error构造测试不能代替完整HTTP/报告证据。

随机重建以Go独立模型依据最终原始node树+mode+真实page_contents bytes/asset_blobs.sha256算期望，不调用被测InArea/source SQL/membership helper。固定6个seed，每seed至少60个成功步、至少3次各类：非空来源create、空页first、asset、source-entry/out、区内rename/reorder、delete、restore、system rewrite、rewrite undo、undo-of-undo、import、后台restore、后台revert；降级每seed至少1次并继续普通写/恢复，再新建LLM继续。确定性预备epoch保证稀有形状可达，再随机交织；次数不足直接失败，不让随机拒绝冒充覆盖。每步后比较所有node哈希与完整不变量，拒绝步核历史/资产/树无写。真正SQL造每个`pageViolations`反例：另一笔记本、来源外hash、空页hash、deleted hash、page当前hash不同、asset SHA不同、root hash；不能两个空集合通过。

### 7.2 预检之后守卫拒绝的独立见证

最后一跳测试在**锁外预检已成功**之后改变条件，用既有锁/可注入端口barrier记录到达点，再让真实单元继续。删除mode guard注册必须错误成功/写坏才算；纯预检拒绝不是guard证据。

| 路径 | 如何逼出 | 观察 |
|---|---|---|
| batch | unlocked plan/content通过，unit前barrier；另一事务加重复或移目标入source | duplicate/immutable及正确op，原子回滚；与plan拒绝分开 |
| 上传 | Check/流完/Put已过，TreeWrites前barrier；另一上传commit | duplicate带已提交目标、败者file清理、checkAssets |
| 同步restore | unlocked来源预检过，unit前barrier；提交相同source | 最终guard拒，trash未部分恢复 |
| 同步revert | 无锁plan/snapshot过，unit前barrier；新增冲突source | 若锁下重复预检会先拒，独立guard-only测试装配只绕该预检而保留真实writer/guard，默认路径另证锁下预检存在；不能把默认预检拒当末跳 |
| startTrashRestore/startRevert请求 | source已经重复，调用开始端点 | 纯开始预检拒、queue调用0、无job/变更集行；不是guard mutation |
| worker重新规划 | 开始成功入队后，阻住Tree.PlanRestore/PlanRevert调用前，另一事务加入重复再放行 | Plan预检拒、RunBatch调用0、报告refused但无任何前批，证明worker预检；去guard仍拒不算杀死 |
| worker首批最后一跳 | 测试Tree decorator让真实PlanRestore/PlanRevert成功返回后停住；在第一次RunBatch调用前加入重复，再放行真实RunBatch | 所有生产prechecks保留；实际写guard拒，先前批数=0，去guard会写坏/错误成功，证明guard注册 |
| 后台later batch | Tree decorator观察真实首批RunBatch成功返回（已commit）后，下一RunBatch调用前barrier；新重复或降权 | earlier batch数>0且SQL核实际保留节点/版本；当前批无写、以后不续，code/node/duplicate及报告精确，去guard会错误继续；非refused终态仍清对象 |
| session | 找到页后锁前barrier，节点先移入source | 锁下mode veto在lock前，删veto错误开会话；写guard不能替代 |
| import | 模块假guard记录直接insert页/asset/空folder；bootstrap真结果 | 按规则skip，不虚构整个import应返回duplicate4xx |

barrier优先既有可注入端口，只用于测试装配，不在生产暴露接口、不用sleep。后台测试以包装真实transfer.Tree的decorator在Plan返回与RunBatch前后阻塞，默认生产预检保持；decorator只加barrier/计数，不返回伪造plan或吞拒绝。首批与后批的RunBatch走现有locked前提/解析/真实guard，若未来增加会截获相同duplicate的锁内预检，必须重新给出guard-only装配证据，不能沿用默认路径的拒绝声明。default路径和guard-only装配明确分开，记录每次mutation杀在预检、guard或veto；不可混计。

### 7.3 运行时变异表

每条先在未改坏代码跑同命令为绿，再只改一处语义、见明确断言失败、从备份恢复并核SHA；不生成代码，不用`git checkout --`覆盖在手修改。编译错/无关panic不是有效失败。以下G/W是唯一机械缩写，不是待选择命令：

* `G(pkg,test)` = `docs/v0.1/claude_to_codex/tools/heavy.sh bash -lc 'cd server && go test -count=1 -run "^TEST$" ./PKG'`，TEST/PKG按表文字替换。
* `W(file,name)` = `docs/v0.1/claude_to_codex/tools/heavy.sh pnpm --dir web/apps/web exec vitest run FILE -t 'NAME'`。

记录保存展开的完整命令、exit与失败断言；一行中的“分别”表示独立轮，各轮都要证据，不能一次多处改坏。

| #/Step | 改坏一处 | catcher/命令 |
|---|---|---|
| 1/S1 | 去mode guard注册（changed写） | G(internal/bootstrap,TestSourceRESTWritesReachTheRegisteredGuard) |
| 2/S1 | 去mode session veto；另轮交换mode/lock | G(internal/bootstrap,TestSourceSessionsAreVetoedBeforeLocks) |
| 3/S1 | 去same-byte窄检查；另轮所有no-op过EditLock | G(internal/modules/page/app,TestSourceNoOpsKeepTheirDifferentContracts) |
| 4/S1 | MCP edit锁/预算先于contract | G(internal/modules/page/app,TestSourceMCPChecksBeforeLocksAndBudgets)；真实入口G(internal/bootstrap,TestSourceMCPPrecheckBeforeEditLocks)、G(internal/bootstrap,TestSourceMCPPrecheckBeforeParseBudget) |
| 5/S1 | root算source/只看直接parent/漏asset（分别） | G(internal/modules/page/domain,TestSourceFactsRootAndAncestorMovesKeepStrictDescendants)；G(internal/modules/page/domain,TestSourceContractUsesEachFact) |
| 6/S1 | standard补祖先或问manage_source | G(internal/modules/page/app,TestSourceStepMetadataAddsNoReadsInStandardMode) |
| 7/S1 | SQL AND改OR/覆盖；fake同样独立轮 | G(internal/modules/page/adapter/postgres,TestRevisionsKeepOnlyAllRewriteWrites)；fake轮G(internal/modules/page/app,TestRevisionsKeepOnlyAllRewriteWrites) |
| 8-full/S1 | full reader误读base.rewrite | `docs/v0.1/claude_to_codex/tools/heavy.sh bash -lc 'cd server && go test -count=1 -run "^TestSourceHistoryReadersKeepTheRevertedVersionsRewrite$/^full$" ./internal/modules/page/adapter/postgres'`；真实bootstrap链另有捕获 |
| 8-Of/S1 | Of reader误读base.rewrite | `docs/v0.1/claude_to_codex/tools/heavy.sh bash -lc 'cd server && go test -count=1 -run "^TestSourceHistoryReadersKeepTheRevertedVersionsRewrite$/^(of\|of_subset)$" ./internal/modules/page/adapter/postgres'`；原bootstrap存活，不能记其捕获Of |
| 8-chain/S1 | compact/ops去Rewrite、sync/bg写回不存true（各独立轮） | G(internal/modules/page/app,TestSourceRevertsCarryRewriteAcrossBatches)；G(internal/bootstrap,TestTheSourceRewriteSurvivesUndoAndUndoAgain)。bg marker的bootstrap仅barrier失败；直接app断实际Rewrite值才是该轮语义catcher |
| 9/S1 | content-only缺线当区外/改读全Tree（分别） | G(internal/modules/page/app,TestSourceContentOnlyRevertReadsOnlyRelevantLines) |
| 10/S1 | Merged数全部项/区外项；另轮去逐项检查 | G(internal/modules/page/domain,TestBatchSourceFactsKeepEachContentStep)；G(internal/modules/page/domain,TestBatchSourceMergedIgnoresStandardAndRootBodies)；G(internal/modules/page/domain,TestBatchSourceContractJudgesEachActualChange) |
| 11/S1 | 仅去guard的Merged后备（保留plan） | G(internal/modules/page/app,TestSourceMergedBatchGuardKeepsItsActualOwner)；G(internal/modules/page/app,TestSourceMergedCreationIsCheckedWithoutAPlanner) |
| 12/S1 | direct import缺line/kind | G(internal/modules/page/app,TestSourceStepProducersKeepContentKindsAndBothLines)的direct_import子例 |
| 13/S1 | 后台复用旧授权或吞Refused | G(internal/bootstrap,TestSourceDescendantRefusalKeepsAnEarlierCommittedBatch)；请求/worker/实际unit层次另由TestSourceBackgroundRequestRefusesBeforeEnqueue、TestSourceBackgroundRechecksAtTheWorkerAndActualUnit区分 |
| 14/S2 | 00050漏LLM/asset回填；另轮检查31byte | G(migrations,TestSourceHashSchemaAndBackfill) |
| 15/S2 | 两条delete SQL分别不清hash | G(internal/bootstrap,TestSourceDeletesClearEveryHash) |
| 16/S2 | Forget省/事务外/subscriber后（分别） | G(internal/bootstrap,TestSourceDowngradeClearsHashesAtomically) |
| 17/S2 | rewrite不改hash/移动只根/restore漏asset（分别） | G(internal/modules/page/app,TestSourceHashTransitionsMatchRebuild) |
| 18/S2 | duplicate_of只凭新UUID/当前key；另轮内部匹配一律无id（B-M1） | G(internal/bootstrap,TestSourceDuplicatesCarryTheirTargetEverywhere) |
| 19/S2 | 漏kind/nb、空页参与、空asset不参与（分别） | G(internal/modules/page/adapter/postgres,TestSourceProbesFollowBoundedCursors)；G(internal/bootstrap,TestSourceDuplicatesCarryTheirTargetEverywhere) |
| 20/S2 | 首candidate即止/分块NOT IN/游标不排等于/满页提前止（分别） | G(internal/modules/page/adapter/postgres,TestSourceProbesFollowBoundedCursors) |
| 21/S2 | snapshot用ReadCommitted或E在snapshot外 | G(internal/modules/page/adapter/postgres,TestSourcePrecheckKeepsOneSnapshot) |
| 22/S2 | 外部包含X/sim删一个清整key/rewrite不更新sim（分别） | G(internal/modules/page/app,TestSourcePlansSimulateEveryHashChange) |
| 23/S2 | 去source advisory；树写也取；探查后才取（分别） | G(internal/bootstrap,TestTheSourceWritersSerializeInBothLockModes) |
| 24/S2 | 去mode guard，生产预检保留；分别batch/upload/restore、明确装配的sync revert、worker Plan成功后首批、前批已commit后批 | G(internal/bootstrap,TestSourceDuplicatesReachEveryLastHop)（逐层子例记录；请求拒/worker规划拒不能充当这项结果） |
| 25/S2 | reader先全Subtree再len/普通单move也限/undo漏限（分别） | G(internal/bootstrap,TestSourceEntryStopsBeforeReadingTheWholeSubtree)；G(internal/modules/page/domain,TestPlanBatchBoundsWhatItsMovesCarry) |
| 26/S2 | duplicate parent drop后代/漏Skipped/503累加（分别） | G(internal/bootstrap,TestTheSourceImportKeepsDuplicateParentsChildren) |
| 27/S2 | HTTP/MCP reserved/report JSON/报告HTTP任一层drop字段（分别） | G(internal/bootstrap,TestSourceDuplicatesCarryTheirTargetEverywhere)（含asset与nil） |
| 28/S2 | 新查询不用Planned/省nb/索引无id全组Sort（分别） | G(internal/modules/page/adapter/postgres,TestSourceQueriesKeepTheirPlans) |
| 29/S2 | pageViolations漏新规则；另轮允许成功计数0 | G(internal/bootstrap,TestSourceHashViolationsCatchEachGap)；G(internal/bootstrap,TestTheSourceRebuildMatchesSuccessfulRandomSteps) |
| 30/S3 | isSource算root/page-only/不观察mode（分别） | W(src/stores/source-area.test.ts,strict source membership follows all nodes and mode) |
| 31/S3 | ModE/task/restore/edit-here留一入口（分别） | W(src/pages/page/source-actions.test.tsx,source controls stay read only at every entry) |
| 32/S3 | loss清id后end；late-open不end | W(src/stores/edit-session.test.ts,source loss ends the server session exactly once) |
| 33/S3 | registry缺readonly或autosave不停 | W(src/stores/page-editing.test.ts,source loss locks the mounted editor and preserves unsaved text) |
| 34/S3 | decoder丢BOM/规整CRLF/宽容UTF8/只数字符（分别） | W(src/pages/notebook/source-uploads.test.tsx,Markdown uploads preserve the exact UTF-8 bytes) |
| 35/S3 | MD409自动编号/modal关即abort/generation不abort/Page走asset分页（分别） | W(src/pages/notebook/source-uploads.test.tsx,source uploads keep their queue and explicit title) |
| 36/S3 | asset拼page链接/缺id显UUID/旧树不刷新（分别） | W(src/pages/notebook/duplicate-text.test.tsx,duplicate targets include attachments and stale trees) |
| 37/S3 | source不观察downgrade/标准仍去重 | `docs/v0.1/claude_to_codex/tools/heavy.sh pnpm --dir e2e exec playwright test stories/wiki/lw12-downgrade.spec.ts`；G(internal/bootstrap,TestSourceDowngradeClearsHashesAtomically) |

本轮追加变异（仍逐轮先绿/改坏红/恢复，使用上面的G/W）：

| #/Step | 改坏一处 | catcher/命令 |
|---|---|---|
| 38/S1、S2 | 后台source facts退回同步RevertFacts；S1结构与S2真实duplicate分开 | S1：G(internal/modules/page/app,TestSourceJobFactsUseNominalCutsAndActualActions)、G(internal/modules/page/app,TestSourceJobAsideFactsMatchActualBatches)；active-background→sync在三个真实S1场景存活，不记产品结果已区分。S2-3实际catcher为G(internal/modules/page/app,TestSourceJobPrechecksKeepOppositeCutOutcomes)、G(internal/modules/page/app,TestSourceJobIndependentGroupsDoNotCoalesceForDuplicates)，来自合法模块历史fixture；实际控制`38-active-job-snapshot-uses-sync-facts`已按0→1→0核对。公开链改由TestTheSourceCreationRewriteUndoKeepsItsRecordedBase守真实记录语义，不再声称无效旧例已复现；整Step最终重放仍待 |
| 39/S1、S2 | revertActions不合并同group/跨group误合并、只用最细cuts、忽略Batch配置（分别） | S1：G(internal/modules/page/app,TestSourceRevertActionsFollowTheActualBatchGroups)、G(internal/modules/page/app,TestSourceJobFactsUseNominalCutsAndActualActions)；transfer/app的TestARevertStartReceivesNominalBatches、TestARevertWorkerReceivesNominalBatches、TestABackgroundBatchesMatchHandwrittenCuts、TestABackgroundRevertRegreediesAfterJoinedHalving，均按G展开；S2 duplicate结果仍pending |
| 40/S2 | CheckRestore完整读在admit之前；去后续error release；重复仍enqueue（分别） | G(internal/modules/page/app,TestSourceRestoreAdmissionPrecedesFullReads)；G(internal/modules/transfer/app,TestSourceRestoreReleasesEveryClaim) |
| 41/S2 | worker Plan预检移除（保留guard）；另轮只去guard | G(internal/bootstrap,TestSourceBackgroundRefusalsNameTheirActualLayer)（Plan前重复应0 RunBatch；Plan成功后首批和真实非零前批后的拒绝分别杀guard轮） |
| 42/S1、S2 | MCP码表漏immutable的write/batch或duplicate的move；另轮漏另一scope | G(internal/bootstrap,TestSourceMCPToolsAnswerEveryAddedCode) |
| 43/S2 | 实际halving误拆Joined/按nodes而非index一半，或缩后来源拒被当并发专用错误 | G(internal/modules/transfer/app,TestSourceJobPreviewUsesTheWorkersBatchEnds)；G(internal/modules/page/app,TestSourceJobCutsKeepTheirDifferentOutcomes) |

有限reader追加变异44/S2：inner ORDER改sort_order,id、去outer LIMIT、parent转text、完整前排序/错按父分组、恰好max不探下一层（分别）；命令G(internal/modules/page/adapter/postgres,TestSourceSubtreeReadsWithinItsWorkAndOrder)。采用真实SQL的宽层/多父/范围与完整顺序子例，不只检查LIMIT文本；overflow后hash/asset仍由G(internal/bootstrap,TestSourceEntryStopsBeforeReadingTheWholeSubtree)核对。

恢复折叠新增控制RF1–RF7（与实施决定44/45不同编号体系，旧159义务另列）：

| ID | 改坏一处 | catcher/命令与证明边界 |
|---|---|---|
| RF1 | 不转交prepared AreaHashes | G(internal/bootstrap,TestSourceRestoresWriteEachNodeOnce)；物理写次数/恢复转live同时带hash，通用维护仍可能使最终hash正确 |
| RF2 | 生成RestoreNodes SQL省略折叠 | 同RF1；物理工作控制，不冒称最终hash语义失败 |
| RF3 | 缺项也覆盖旧hash | G(internal/modules/page/adapter/postgres,TestRestoreNodesMaintainsOptionalSourceHashes)；nil/empty/mixed map与显式NULL分开 |
| RF4 | hash/presence错配到下一个ID | 同RF3；乱序且不同值的非空oracle |
| RF5 | 省略恢复删除时刻条件 | 同RF3；wrong-time/already-live不应被改，节点数非空断言 |
| RF6 | 忘记先前trash恢复者的nil baseline | G(internal/modules/page/app,TestSourceRestorePreservesTheUnitsOriginalDuplicateTarget)；后继duplicate_of错误给id |
| RF7 | 覆盖同单元原来源的首次baseline | 同RF6；先删再恢复后错误省duplicate_of |

本轮新控制的真实退出/失败断言/恢复及独审接受见[S2审查](reviews/P2-S2-source-hash-review.md#最终验证与集成)；原159依赖重放单独按fresh/reuse表核定，不将七项加入旧159后称作全部重新执行或独特产品bug。

有界facts保留追加11项，独立于上述RF7和旧159：绕过缓存；分别不复制Nodes、Place、ParentID、Content、Hash；不累加NodeFacts出现次数；去Step界；扩大载荷界；超界错误返回截断副本；独立漏计Hash载荷。缓存命中catcher为`BoundsItsFactReplay`，引用catcher为`FreezesYieldedFacts/FreezesParentValues`，边界catcher为对应Bounds与`CountsKeysAndHashesAtTheReplayBoundary/extra_byte_1`；超界控制将原selector联合`ReplaysTheDecisiveTailAfterOverflow`及已有whole-X测试，三阶段argv保持完全一致。精确名称/命令、所有scalar断言及全源恢复见本轮11项封存原件，不将11个非race控制充作两次race或性能证据。精确1MiB+1的Hash独立控制应实际得到1048577、1遍而应3遍；尾部protected/duplicate两叶应拒绝，截断变异错误放行必须被捕获。


另有契约/矩阵/双语/SQL不变量每项反例，跟随已有对应测试记录，不以这些变异替代§3.12逐操作答码检查。Stage最终修复按workflow重跑该Step全表与审查者追加变异；恢复后门禁针对最终树，不沿用改坏前的结果。

S1补充反例单列类别：`TestTheDowngradeEndsTheContract`的合法no-op请求是test-input（200仍成功、readback正文/revision失败），`TestSourceMatrixSeedKeepsItsTrashGroups`的同刻删除是test-fixture；均不混入生产语义变异killed数。AfterParents用`TestSourceFactsRestoredForestUsesEachExternalParent`与两条RestorePart/continuation测试；cross-group全成员literal oracle替代旧首成员弱观察，旧存活不删。审查者A的旧有效session changed末跳、B的SQL/fake/双reader/compact/forest六轮、C的严格v2公开入口guard轮及C-M1正文双向、C-M2删除管理位置、C-M3祖先/child两种实际管理次序真实/fake对照，分别记录在[S1审查](reviews/P2-S1-source-rules-review.md)。最终修复头全部重放：fd00新目录whole-step-final-v4：109 variants /112 checks /336同argv执行，112 baseline与112 restore均0，mutant110次exit1、2次已知exit0。108项至少有一个预期catcher；test-input、test-fixture及cycle结构观察单列，不全部称产品语义kill。Of/bootstrap存活而direct of/of_subset捕获；active-background→sync继续存活；marker barrier、trace与用户结果边界保持。原93控制288份失败测试名集合与6fac轮一致；A/B/C八控制、fix1五控制、fix2三控制均重放。另11真实/fake对照exit0，原C-M3 Folder先拒绝与child先允许由真实HTTP/worker/SQL次序验证，主持者root-replay-integrity及root-counterexample-integrity均接受

S3追加控制独立列名，不与实施决定48混号：

| ID | 改坏一处/观察 | catcher与边界 |
|---|---|---|
| S33-queued-create-ignores-abort | 树队列轮到时不查signal | W(src/stores/page-tree.store.test.ts, source creation)；队列取消后未调用send observer/service |
| S33-create-request-drops-signal | PageService不转signal | W(src/services/page.service.test.ts, PageService.createPage with source content)；真实Request对象在HTTP fake中的signal和AbortError，不宣称真服务或数据库回滚 |
| S33-queued-ordinary-auto-names-source | 普通Untitled排队后父成为来源仍自动创建 | W(src/pages/notebook/source-add-dialog.test.tsx, queued ordinary creation)；真实组件/树队列的请求与no-navigation回归 |
| S34-tree-alert-retains-focus | duplicate导航前不消耗持久树Alert | W(src/pages/notebook/source-moves.test.tsx, duplicate tree drop link)；真实组件附件定位/focus，不放宽共享读者活动保护 |

审查修复追加控制及最终运行清单：

W 的文件/选择器按 P2 §7.3 展开；本轮共同 heavy 编排下各 Vitest 命令另包 `timeout 10m`。PW 是 e2e 工作目录的 `timeout 15m pnpm exec playwright test FILE --workers=1`，每阶段先 `timeout 15m make -j1 build`。完整 argv（含固定输出路径）和实际失败断言在原始回执，不省略在证据中。

| 控制组 | 实际 catcher | 原版→改坏→恢复 exit | 结果 |
|---|---|---|---|
| `30-root` | `W(src/stores/source-area.test.ts, strict source membership follows all nodes and mode)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `30-page-only` | `W(src/stores/source-area.test.ts, strict source membership follows all nodes and mode)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `30-unobserved-mode` | `W(src/stores/source-area.test.ts, strict source membership follows all nodes and mode)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `31-mod-e` | `W(src/pages/page/source-actions.test.tsx, source controls stay read only at every entry)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `31-task` | `W(src/pages/page/source-actions.test.tsx, source controls stay read only at every entry)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `31-restore` | `W(src/pages/page/source-actions.test.tsx, source controls stay read only at every entry)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `31-edit-here` | `W(src/pages/page/source-actions.test.tsx, source controls stay read only at every entry)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `32-clear-id-before-end` | `W(src/stores/edit-session.test.ts, source loss ends the server session exactly once)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `32-late-open-not-ended` | `W(src/stores/edit-session.test.ts, source loss ends the server session exactly once)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `33-registry-without-readonly` | `W(src/stores/page-editing.test.ts, source loss locks the mounted editor and preserves unsaved text)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `33-autosave-keeps-running` | `W(src/stores/page-editing.test.ts, source loss locks the mounted editor and preserves unsaved text)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `34-bom-stripped` | `W(src/pages/notebook/source-uploads.test.tsx, Markdown uploads preserve the exact UTF-8 bytes)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `34-newlines-normalized` | `W(src/pages/notebook/source-uploads.test.tsx, Markdown uploads preserve the exact UTF-8 bytes)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `34-lenient-utf8` | `W(src/pages/notebook/source-uploads.test.tsx, Markdown uploads preserve the exact UTF-8 bytes)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `34-characters-not-bytes` | `W(src/pages/notebook/source-uploads.test.tsx, Markdown uploads preserve the exact UTF-8 bytes)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `35-md-title-numbered` | `W(src/pages/notebook/source-uploads.test.tsx, source uploads keep their queue and explicit title)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `35-modal-cancels-files` | `W(src/pages/notebook/source-uploads.test.tsx, source uploads keep their queue and explicit title)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `35-generation-keeps-uploads` | `W(src/pages/notebook/source-uploads.test.tsx, source uploads keep their queue and explicit title)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `35-page-enters-asset-pagination` | `W(src/pages/notebook/source-uploads.test.tsx, source uploads keep their queue and explicit title)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `S33-queued-create-ignores-abort` | `W(src/stores/page-tree.store.test.ts, source creation)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `S33-create-request-drops-signal` | `W(src/services/page.service.test.ts, PageService.createPage with source content)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `S33-queued-ordinary-auto-names-source` | `W(src/pages/notebook/source-add-dialog.test.tsx, queued ordinary creation)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `36-attachment-as-page` | `W(src/pages/notebook/duplicate-text.test.tsx, duplicate targets include attachments and stale trees)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `36-missing-target-leaks-refused-id` | `W(src/pages/notebook/duplicate-text.test.tsx, duplicate targets include attachments and stale trees)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `36-stale-tree-never-refreshed` | `W(src/pages/notebook/duplicate-text.test.tsx, duplicate targets include attachments and stale trees)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `S34-tree-alert-retains-focus` | `W(src/pages/notebook/source-moves.test.tsx, duplicate tree drop link)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `37-stale-notebook-mode` | `PW(stories/wiki/lw12-downgrade.spec.ts)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `37-standard-still-rejects-same-byte-page` | `PW(stories/wiki/lw12-downgrade.spec.ts)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `A-move-before-send` | `W(src/pages/notebook/source-moves.test.tsx, a confirmed move still queued behind another write rechecks\|a queued ordinary move stops)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `B-carriage-return-only` | `W(src/stores/asset.store.test.ts, Markdown keeps its UTF-8 bytes while only the suggested title is trimmed and composed)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `B-queue-signal` | `W(src/stores/page-tree.store.test.ts, source creation cancelled in the tree queue never calls its send observer or service)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `C-current-source` | `W(src/pages/notebook/duplicate-text.test.tsx, clicking a retained link after)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `C-current-parent` | `W(src/pages/notebook/duplicate-text.test.tsx, a retained attachment link uses its new parent)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `C-refused-target` | `W(src/pages/notebook/transfer-background.test.tsx, a duplicate report distinguishes its refused attachment)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `C-location-survivor` | `W(src/pages/notebook/duplicate-text.test.tsx, "")` | 0 → 0 → 0 | 冗余位置守卫存活 |
| `fix-A-ancestor-reparent` | `W(src/pages/notebook/source-moves.test.tsx, source-root ancestor\|subtree gains the source root)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `fix-B-current-role` | `W(src/pages/notebook/source-uploads.test.tsx, a failed Markdown upload cannot be retried after becoming reader)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `fix-B-retry-queued-role` | `W(src/pages/notebook/source-uploads.test.tsx, a Markdown retry queued before becoming reader)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `fix-C-existing-outside-target` | `W(src/pages/notebook/duplicate-text.test.tsx, an existing outside)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |
| `A-drag-hint-binding` | `W(src/pages/notebook/source-drag-hints.test.tsx, source-root ancestor drag hints)` | 0 → 1 → 0 | 语义断言失败；恢复绿 |

最终ae4501上共40控制组、120次测试和6次真实build，126份原日志；39组0→1→0，C-location为已核0→0→0，不算kill或独立产品bug。控制组不等于独特缺陷数；B队列取消与既有S33同语义但选择器来源分别保留。三阶段测试argv逐字一致；#37固定执行输出目录后逐阶段归档，不再沿用1da时输出目录各异的说法。所有source/input SHA与最终Git一致，恢复后工作树干净。125份producer日志校验值直接匹配raw；B-carriage-return-only的mutant断言输出含2个真实CR，producer read_text().encode()规范化换行后算hash，首轮raw核对因此拒绝。根保留原日志/原报告，另记录raw SHA与准确规范化SHA桥，不把它当成重新运行，也不改写历史。C原席在8c另作组件94→3红→94和实际每phase build的LW3三例3→2红→3；test-only桥核3198既有文件全不变，浏览器证据与根40组执行数分列。 原C席只读独立复算最终桥、40组、126日志及唯一规范化关系：final-evidence-audit.json SHA-256 461c10c32b0bd4a419c13ae7dcba084fe8ebea0cfebf8f90f1dec361e3f78139。该轮未重跑测试/build/container，不冒称新的产品验证。

### 7.4 计划、性能与600万迁移

计划三阶段：无统计、ANALYZE后、force_generic_plan；每阶段结果/范围与EXPLAIN JSON形状都断言，真实应用仍Planned。一本10万活来源、另一本20万并含同key，混普通区/trash/空页；另一本不得匹配或影响范围。单页新增、10k来源移入（页/asset混合）、同步恢复100页+9900附件、上限撤销、后台60k计划、10万来源Forget各单独Makefile成本行（-v输出）。rewrite使100k同hash时量X=3与60k、外部在首/末/不存在；记录返回数/查询数/heap fetch/排序。四列索引与原三列用同数据记录relation/index bytes、建索引/回填、正常写维护差值。普通10001移动与原大Subtree守卫继续；不把早拒当成功操作很快。

新时间线先按总体13.4第8条实测：同构建/CPU/Go/PG、无并行重活、固定节点/链接/字节/统计，列全部样本。请求级短守卫每形状三次取最快，DB探查三阶段各三次中位数；预算为所采本机基线三倍，毫秒级读采用总体已接受的最小宽松底线并在结果写出实际数与理由。几秒级整单元守卫三次逐次记录，不能用最快一次隐藏重复超线；大操作按实际最慢合理样本三倍定新线。先实测再写常量，附计划与退化反例；不凭一次快样本猜数。性能不达既有请求/单元界先改算法或审查范围，不增旧timeout/放宽旧门禁。race setup与被测请求分离，不把准备放4秒请求期限里。

**600万冷启动迁移是S2完成项**。共享heavy锁内建立专属ext4目录/PG实例/DB，写唯一名称和拥有资源清单；只停/删自己启动的。先预算基表/索引/WAL/clone峰值，空间不足只清自己确认可丢的产物或分次测，不能碰未知数据。已接受16MiB普通文件preflight只证明scoped fsync+DONTNEED可用（4096→0→0pages），25.36GiB只是当时余量；不是PG/600万证据，不重做普通文件实验。

依次量两份从00049起的600万实际schema：A无LLM（首发空回填）；B已有LLM（明确至少100k eligible sources，分非空页/空页/asset，另含trash/普通区/另一笔记本），实际分布/行宽/深度/哈希有效性独立SQL核对并记清单。B不得用几行回填代替主测。运行项目实际嵌入goose runner执行完整00050事务，不用手写近似SQL/并发建索引替代。

冷启动协议：停**自有**PG，flush/fsync自有目录文件，对确认属于它的relation/index/WAL作POSIX_FADV_DONTNEED，记录manifest/fincore驻留；启动后、goose前再记录关键6M nodes heap/index驻留（承认启动必需页会热，列具体数量）。只在路径/权限/覆盖和即时驻留可证时称OS page-cache cold-start，不声称硬盘控制器缓存冷。禁止全局drop_caches、重启Docker、停用户PG；VACUUM/CHECKPOINT不等于冷。实际事务只在起点冷一次，记begin/commit、SQL阶段、总wall、IO/WAL/索引大小/锁等待/检查点条件。

独立可重置fixture分别冷却后量CHECK验证扫描与CREATE INDEX扫描；诊断时序不是goose实际时序，不把两段相加冒充停机。B的backfill时间/写行数/WAL及完整不变量单列。锁影响reader/writer probe在独立诊断跑，不混主计时。A/B全事务都有证据后写升级窗口：节点表强锁、期间相关读写阻塞，停旧实例升级、备份/恢复与失败事务回滚；Down/Up只在自有库核对rewrite丢标记偏严、source hash重建。原始日志/JSON/sha外存，仓库保留命令/机器/数值摘要。M12仍在真实服务器负载重测，本机结果不是生产时间承诺。

### 7.5 串行门禁、CI分份与余量

所有Docker/PG/构建/E2E/性能/变异只经同一heavy.sh mkdir锁，不另设NW_HEAVY_LOCK并行。工作树根分次执行：

```sh
docs/v0.1/claude_to_codex/tools/heavy.sh make -j1 lint gen-check knip build
docs/v0.1/claude_to_codex/tools/heavy.sh make -j1 test-go-race
docs/v0.1/claude_to_codex/tools/heavy.sh make -j1 test-go-bootstrap-1
docs/v0.1/claude_to_codex/tools/heavy.sh make -j1 test-go-bootstrap-2
docs/v0.1/claude_to_codex/tools/heavy.sh make -j1 test-go-bootstrap-3
docs/v0.1/claude_to_codex/tools/heavy.sh make -j1 test-go-bootstrap-4
docs/v0.1/claude_to_codex/tools/heavy.sh make -j1 test-go-costs
docs/v0.1/claude_to_codex/tools/heavy.sh make -j1 test-go-guards
docs/v0.1/claude_to_codex/tools/heavy.sh make -j1 test-go-short test-web
docs/v0.1/claude_to_codex/tools/heavy.sh make -j1 e2e
```

本机不跑plain make test/test-go（四分片配置的race段内部-j5），以上显式成分构成等价门禁；最终分支CI要求实际11项含image/smoke全成功。设计与S1当时为三片/十作业，它们的真实结果仍在原段保留，不追改为四片。设计合并前的文档链接/guarded替换与CI不代产品门禁；S1历史执行与最终记录见10.1。

新增`TestSource...`在第二份、`TestTheSource...`第三份，其余名称按FIRST、SECOND、THIRD减SECOND及三者并的补集分到四份。每Step新增race测试各两次、四份完整bootstrap race各两次；门禁样本可复用但必须绑定相关最终输入且确有两次。c7旧两分支的三片Test均值398/555.5/446.5秒只用于触发拆分，不是四片实测或两个main验收；即使FIRST选择不变，SQL变更后它的旧时间也不能自动沿用。实际新清单须从最终源码重建，468顶层是c7历史数；本机四份各两次、package/wall与CI Test分别记录。每Step合并与记录提交的精确CI都绿后，用这两个main Test样本核各份均值<450秒。成本行按原本机两次均值×2.4及M9/P2§6.5半个进程超时线分行，现有timeout不提高。S1入口440.5/317.5/260.5及其后328/313.5/394均为各自历史，见10.1；最终S2新头的实际结果集中见10.2与审查表。

**S2后续边界再平衡（实施决定44）。** 33b4/4550两分支第一片Test444/457s均值450.5s触发M9/P2§6.5；相邻第二片419/418s均值418.5s。重排时仅将当时清单的`TestEveryResolutionCaseImportsFromAVault`移到第二片：两次选中EveryR命令中该顶层run→终止跨度11.087505/10.921745s，均值乘1.6后迁移估计17.6074s，预测第一/第二432.8926/436.1074s；全R预测第二451.5228688s而被拒绝。这些是估计，非CI实测。重排的四片完整双轮、原件与独立核对已接受；fb精确CI随后因Revert10k失败，最新有界facts保留输入须重建清单并绑定完整双轮、新代码/文档两分支及两main样本。集中见[S2审查](reviews/P2-S2-source-hash-review.md#4550后的容量再平衡)，不以本机量或旧分片样本替代。

IndexItems 30ms与grep600ms现有义务不改，后续接近/超线查根因/样本，M10收尾核稳定性。仅认P2入口已有的两个样本，不承诺新增工作必然装得下。

## 8. 完成标准与逐项文档投影

此节中的引文是实施验收后应写入对应章节的新文字；现在是**计划**，不预先改成已完成。每Step使用带标签、旧段恰好一次断言、`--check/--path`与全部成功后才写的脚本，在干净archive副本演练；主持者审核，不能全文件格式化。每条须用最终真实接口/测试名/数字替代本文设计名；没有证据则Step不完成。第15节分别记该Step真实改动与审查链接；不改历史失败记录为成功。

### 8.1 S1 的精确完成投影

| 目标 | 应加入或替换的新文字 |
|---|---|
| v0.1-design §12.4 page guard/session行 | “来源正文事实由StepContent（Hash/Bytes/BeforeBytes/Merged）、Rewrite、每节点Kind与严格祖先事实交给同一CheckContract；mode同时是写守卫与SessionVetoer，在lock之前注册。来源会话开启在笔记本锁下否决，已有会话的保存也受正文规则约束。” |
| §13.1 第1项单元/历史 | “page_revisions.rewrite只由系统追加写及相应撤销带true，同一行累积用AND；用户混入即false。撤销/后台压缩计划/撤销的撤销保留被撤销版本的标记，旧行false。content-only计划复用相关祖先事实，不为来源判断读取整树。后台事实在S1即按compact/名义批界/本批group合并的实际动作构造；同步仍按原Ops，不能混用。” |
| §13.1 第4项拒绝/次序 | “来源是来源根的严格后代，含附件；普通模式不补来源事实/授权。来源version_restore与有效已有session保存即使字节相同也拒绝；该窄no-op纯预检不改变普通及非来源no-op的锁行为。changed写始终由已注册guard最终判定；MCP edit在已有领域错误之后、锁与解析预算之前共享模式预检。” |
| §13.1 第8项契约码 | “P2/S1为put/task/session/version_restore/revert/startRevert声明并真实答出source_immutable，为delete补source_admin_only；REST与两入口MCP码表、逐码答复和双语同时完成。根说明不算来源，管理员无正文修改豁免。” |
| §13.1 第11项端口 | “ModeGuard以pool与authorizer构造，按需取根祖先，复用已读line；直接导入insert helper也携带line/Kind。单元Grants只在需要时问manage_source，普通模式零额外调用。” |
| §13.1 第21项扩展点 | “组合根mode guard与mode session veto分别以最后一跳反向对照守注册；任务各批重取模式/权限，预检通过不能绕过单元。” |
| §13.4 第1/3/4/6/13项 | 1：“rewrite真SQL/fake AND、no-op例外、来源facts/每路径码均有反向对照。” 3：“LW4/LW5 API+MCP在P2/S1完成，尚缺页面见P2/S3。” 4：“来源session模式与lock优先序、后台descendant降权以受控交错验证。” 6：“权限矩阵覆盖writer来源页/附件管理拒绝和admin成功，根说明普通对照。” 13：“来源guard、batch逐项、upload/历史预检同一CheckContract；Merged只计项当时来源的实际字节改变，三拒四准与偏严后备有对应例。” |
| M8-history 处理进展 | “P2/S1已补rewrite持久/全撤销链、version_restore/session不可改来源、来源管理与临时步骤规则、同步回滚/后台refused；source_duplicate与网页解释仍待P2/S2、S3，状态open。” |
| M9-P1-mcp-registry §7进展 | “P2/S1落实四种写的来源事实、移动加改名、合成正文与临时步骤/恢复后写回；码与实际调用覆盖两入口。查重与batch最终正文差异待S2，整份仍open。” |
| P1设计§9、00 §12/13、P2 §10.1与审查记录 | “P1交P2的source事实/rewrite/session/Merged/管理矩阵项在P2/S1验收；P2/S1完成，S2下一步。” 附真实提交、审查处置/复核、变异与门禁、分支CI、合并及记录CI位置；merge号提交前写“见00§12”，之后据实填。 |

S1不把source_hash/duplicate UI/SourceArea lint读写成已实现；决定36–43已随本设计接受，仍不等于任何Step的实现验收。新的src_admin_only detail跟真实保护规则一致。

### 8.2 S2 的精确完成投影

| 目标 | 应加入或替换的新文字 |
|---|---|
| §13.1 第1项派生数据 | “nodes.source_hash由单元每步写后、记条目前维护；仅活LLM严格来源的非空页/附件有当前hash。DeleteNodes与DeleteNotebooksPages同句清空，降级在SetMode后/ModeSubscribers前同事务Forget。系统改写允许收敛同hash，索引非唯一。” |
| §13.1 第4项错误/范围 | “source_duplicate的duplicate_of只指本单元开始已具相同来源资格/kind/hash的节点，回滚撤去资格或字节时省略。source-entry既存子树一次最多10k节点，有限读取最多返回/物化10,001后拒source_too_large，真实索引访问/排序范围另外验收；普通单次大移动/区内或移出行为保留。已知cycle/title/protection先判，overflow优先于未读到的深度错误。” |
| §13.1 第5项锁序 | “只有FOR SHARE空来源首次正文在content行之后、探查之前取source advisory；树单元靠notebook互斥。当前生产仅单页SHARE，未来多页须先锁齐全部content行再取advisory，单按ID升序不足以证明无环。” |
| §13.1 第8项错误成员 | “Problem可选duplicate_of；MCP错误文字有可见路径且structuredContent保留成员不能被Extra覆盖；TransferRefusal的duplicate_of为必填可null，node_id是被拒节点，不是重复目标。每操作每码由本模块HTTP与整个程序实际答出。” |
| §13.1 第11项端口/迁移 | “AttachmentInfos批量给SHA256；page.SourceArea提供Forget与元数据探查，不跨模块运行时读表。00050只读notebook/asset回填的迁移例外有明确注释；只改page表。锁外来源预检复用既有Snapshots，heads/E/游标在同一readonly repeatable-read，guard不嵌套事务。” |
| §13.1 第21项最后一跳 | “新增重复在batch/上传/同步恢复撤销/后台批写的预检后出现，实际guard仍拒；guard-only装配与默认预检证据分别列明。import重复是skip或空folder继续后代，模块guard观察与整个程序结果共同证明路径。” |
| §13.4 第1项 | “source_hash完整不变量每条由SQL反例捕获，schema31/33-byte反例、非唯一索引与已有LLM迁移回填均核对；两delete/Forget/asset/rewrite不遗漏。” |
| §13.4 第3/4/6项 | 3：“LW3 API+MCP、LW5重复、LW6/LW7 API完成；历史与后台无MCP工具的豁免附Go证据。” 4：“双上传、两SHAREfirst、tree/first两种顺序、downgrade/first/session、批间重复由真实锁等待逼出。” 6：“来源矩阵fixture按真实hash不变量播种，并核每条码与实际授权。” |
| §13.4 第8项 | “来源新增/10k移入/上限恢复撤销/60k计划/10万Forget各有独立成本行，三统计状态输出原样本、预算与余量；四列来源索引与三列空间/写入差异及600万冷迁移见P2§10.2。” 在取得样本后追加真实常量、命令、CI余量，不预填数字。 |
| §13.4 第9项 | “source元数据和写回每块≤10k，经Planned；候选四列索引以nb/kind/hash/id游标读，sum(Take)≤10k；排除全集X在Go，按key最多E+1候选、总≤K+|X|。完整历史计划可更大，不以分块NOT IN伪造完整排除，不读全文或全来源。” |
| §13.4 第10项 | “来源hash随机成功步后等于独立从树/正文/资产重建，上传、空页first、rewrite、import、后台restore/revert、降级后写/恢复均有最低成功次数，失败步比较原状态。” |
| §13.4 第13项 | “开始restore/revert的source检查模拟各自实际执行表示的每个临时membership/hash变化；后台按未缩减名义批计划，缩批可在无并发时改变允许结果，决定43保留该取舍，稳定外部候选与计划受影响节点分离；恢复body与之后write-back两步各判。writer/admin及逐步偏严形状保持，MCP batch说明明确最终合成body查重差异。” |
| README 部署/升级段与P2测量记录 | “00049增加rewrite默认false，旧/Down-Up丢标记的撤销偏严；00050验证节点表、建立非唯一来源索引并回填已有LLM，会持表锁。升级先停旧实例并备份，600万本机冷启动A/B总时间与诊断分别为[实测]，硬件/数据分布见P2记录，生产窗口需实机复测。” 只加入必要运维文字，不在P2重写P5的MCP接入说明。 |
| M7-transfer §3进展 | “P2/S2逐字节SHA256去重覆盖上传/移入/恢复，页与附件不跨kind；导入重复页/附件报告与后代保留验收。该项完成，整份open，导出/虚拟文件与lint分别留P4/P3。” |
| M8/M9进展 | M8：“source duplicate、起始预检与后台refused/rollback完整，网页解释待S3，仍open。” M9§7：“来源四写完整，batch最后合成正文差异已写入两份工具描述，整份open，其余tools/virtual/prompts属后续Phase。” |
| 00/P1/P2结果 | 00 §12/13记S2真实完成与审查；P1§9对应hash/Forget/last-hop/asset字段义务指到P2/S2；P2§10.2填全部真实测量与迁移/测试结果。 |

本轮补入S1/S2的完成投影：

- S1总体§12.4/§13.1第21项补“后台来源事实按compact、生产名义批界和同group批内动作合并生成；实际写入保留M9当前状态检查/retimed/frontier，普通模式不额外构造来源预检。” M8进展记同步/后台事实差异已在S1落地，S2去重待做。
- S2总体§13.1第4项/§13.4第13项补“内存同一步重复与SQL命中按同一真实起点key判duplicate_of；raw/F/A移出、新增B再移回的内部重复带A，起点在区外对照不带。开始/worker模拟前批不是已提交来源。”
- S2总体§13.1第11/21项补“CheckRestore基本错误后admit claim、完整来源读前可busy；所有后续错误释放，duplicate不enqueue。请求/worker预检/首批guard/非零前批后guard四层证据分开，保留生产预检。”
- S2总体§13.4第8/9项补“有限source-entry reader双LIMIT+已有parent/name_key索引顺序、完成后Go整层排序；返回/内存界与实际索引范围/扫描/缓冲证据分别验证，四组统计/计划含真实prepared generic执行，巨大单层和多父反例。”
- S1/S2总体§13.1第8项及M9§7进展写闭合两scope增量码表；S2 M8进展/00取舍明确“名义计划预检不搜索其他分法，缩批可无并发地改变来源结果，先前提交保留”，指向主持者决定43及相应真实/模块证据。

### 8.3 S3 及 P2 完成投影

| 目标 | 新文字/完成动作 |
|---|---|
| §13.2 第7项标题/异步 | “来源新建先问标题，粘贴正文一次create；409留表单不自动编号；成功导航遵守mounted/location。” |
| §13.2 第12项上传归属 | “来源.md与附件的持续队列归每本AssetStore；关闭对话框继续，generation取消。source-page与asset结果分型，编辑器附件Promise仍只返Asset；.md fatal UTF-8保BOM/换行，正文5MiB按原字节发前校验，附件界不变。” |
| §13.2 第17项可读名称 | “来源重复用全节点完整路径和openOf，附件链接去父页定位；树缺目标先重读，仍缺给节点中性说明，无裸UUID；被拒node与duplicate target分别显示。” |
| §13.2 第23项增强/编辑状态 | “来源正文不给task写能力，Edit/快捷键/version restore/edit-here均关闭；source_immutable使session失去、真editor只读/停止autosave且结束server session，unsaved可复制。mode事件让同一标签立即恢复普通行为，无需reload。” |
| §13.4 第3项故事 | “P2完成LW3–LW7页面与LW12另一context真实来源页降级可写、不再去重；保留原事件连接/排队任务证明；API/MCP共用数据库断言，settleEvents只写可编辑说明页。” |
| M8-history 状态/处理进展 | “P2已完成来源历史所有义务：rewrite全链、不可还原版本、来源管理/去重、同步原子与后台部分完成、所有码/报告/网页解释。状态done，证据P2/S1–S3审查与故事。” 不因P1笼统‘P2–P5’而误留open。 |
| M7/M9/M5/M6 | M7§3与M9§7记页面最终验收；两份整体仍open。M5done保持，M6lint仍open，不提前关闭后续工具/虚拟/prompt义务。 |
| P1§9/00头与§12、§13/P2头与§10 | P1每条交P2项有实证去向；00头显示P2完成/P3下一Phase、M10进行中；P2行全完成并链接三Step审查与合并，§13真实记录。P2done独立文档提交及其精确main CI绿才算Phase结束。 |

最终人工核对单：每Step所有新增码真实答出、每条guard/veto接线有杀死mutation、source_hash/rewrite真实SQL与fakes一致、每个声明API字段全链、全部原有、本轮追加与审查追加变异、600万A/B与成本真实数、每main实际workflow全部CI作业绿（当前四分片为11项）及分份均值、所有handoff状态精确。未完成任一项不能用“实现已完成，后续补测试/文档”宣布P2done。

## 9. 给后面 Phase 与 M 的移交

P3：在已实现page.SourceArea基础上增加整区lint/pending来源读取，不复用匹配探查当整区枚举；继续root严格排除、asset SHA/当下字节、0字节page规则。P2未做lint、健康度、两工具或budget说明余量。P4：ingest/虚拟index/log/贡献者使用现有write guard与rewrite、derived hash规则；系统rewrite只维护当前hash不拒收敛，不能用新增普通输入伪造rewrite。P5：prompts与系统说明按最终源区规则（不许admin改正文、too_large、按事务provenance的duplicate文字）表述，仍自己完成prompt契约/人工清单/根README；P2不抢写prompt实现。

`M11-upgrade/handoffs/M10-mode.md`在P2验收后加入：“批准变为LLM时，在同事务重建指定根严格后代source_hash，清来源外；普通模式允许预指定SourceRootID但hash仍NULL。保全的revision.rewrite需保留，旧无标记/Down-Up偏严。SourceHashes/ModeSubscriber顺序及free refusal.code沿P1；实现与例见P2。” 状态open；不编写升级流程。SourceRootID旧注释过强的Nit仍在M10收尾，不在这里顺手改变SQL约束。

`M12-release/handoffs/M4-performance.md`追加真实600万A/B、三/四列空间差/回填/锁等待、10万Forget挡写、source advisory等待与失败率的本机证据，并列“真实服务器/真实数据重测、升级停机窗口、并发来源首写负载”未验事项。`M5-polish.md`仅记实际新增图标/可读名需收尾汇总之处，不造未发现问题。IndexItems30ms及grep600ms持续CI稳定性与M10收尾复发审查保留，不能以P2新cost通过抵消。用户明确不开始M11；本Phase最终交给M10/P3继续，M10收尾才中文汇报与停下。

## 10. 结果

| 项目 | 当前状态 | 证据与剩余 |
|---|---|---|
| P2设计 | 已审查并接受 | 原四项发现及B-Q1在设计层resolved；4755精确main CI40413727596十项成功；[设计审查](reviews/P2-design-review.md) |
| 10.1 S1 | S1已合并；记录CI与两main Test均值已核，S2继续实施 | [实施审查](reviews/P2-S1-source-rules-review.md)；合并见 00 号文档第 12 节 |
| 10.2 S2 | 已验；66d823合并与8904记录精确main CI均已接受、四片Test均值433/403/299/319.5s；旧失败和fresh/reuse历史保留 | [S2审查](reviews/P2-S2-source-hash-review.md#合并与后续验收)与本节入口补记 |
| 10.3 S3 | 实现、原席复核、最终门禁与精确代码/文档/合并CI已验；本次记录CI及两main均值待核 | no-ff `ecd79c77d0cdb2259d50758b26514f61ebb329ca`；[S3审查](reviews/P2-S3-source-web-review.md)；00第12节 |
| 10.4 P2done | pending；本次合并记录精确main CI与两main四片均值闭环后独立完成 | 核§8.3与handoff；P2done自身精确main CI获接受后放行P3，P3仍未开始 |

### 10.1 S1 来源规则

S1后续核对已完成：合并`bf8d6625`的[CI40501936360](https://github.com/open-nerve/NerveWiki/actions/runs/40501936360)与记录`6a4f33b8`的[CI40506120884](https://github.com/open-nerve/NerveWiki/actions/runs/40506120884)均十作业成功。三份bootstrap Test为370/286、322/305、389/399秒，均值328/313.5/394秒，各<450秒，余122/136.5/56秒；root树外回执`p2-s1-main-test-pair.json`已接受并放行S2。以下S1旧轮次/本机与分支样本保留各自范围，不拿它们代替S2当前验收。

六个初审提交依次为08dfaf5f、a0dfe443、93395bd5、3be31d58、08fa1d16、6facccde；第一组C-M1/M2修复提交`b750631362a468a01a18dfc4faa065b7d88af4db`并增量核验，第二组C-M3提交`fd00ee670c24db3ad4d9478419acd8340f1e6d91`，最终代码头`fd00ee670c24db3ad4d9478419acd8340f1e6d91`。00049/rewrite全链、全部producer事实、严格来源正文与管理/session规则、按项Merged与后备、实际后台动作/名义批界、REST/两scope MCP、双语与fake、LW4/LW5 API/MCP已落地；具体差异见3.14。A/B初审0 I/M/N/Q，C初审1 Minor C-M1、补充1 Minor C-M2，b750准备诊断另1 Minor C-M3；主持者分别接受，共三个ID、两组修复，第一组增量通过不代表最终验收；第二组C-M3仅修改Web历史测试设施：现活简单placement按node UUID排序，每步在当时node/树上先管理检查后移动，保留本动作Before权限；不按历史数组、不用原树OR最终树。第一遍不提前移除created/restored，placed避免第二遍重做原树检查；删除/正文保留第一组已证阶段位置，blocked残余保留原管理后备，不模拟完整key/depth/aside/conflict scheduler。第一组C-M1/M2提交b750保持历史，三个ID已由原C正式复核分别关闭；原C复核：原C在fd00正式复核分别关闭C-M1/C-M2/C-M3，未关闭Important=0、Minor=0，无新增已核实I/M/N/Q；主持者核96件/740,998字节、17独立执行及五个0→1→0。报告review-c/final-fix-recheck/C-final-fix-recheck.md，SHA256 82f06e259ce3da1e3a370cc8385d9f74e7c4a402fd861d28d34097334b167aab；完整证据索引见下文。root-I1降级覆盖补充与root-I2矩阵fixture修复属于实施核对，未冒称三审或生产规则缺陷。

6fac的历史证据：93variants/96checks/288commands；91生产/观察变体中90有语义catcher、1active-sync存活，另test-input/test-fixture各1。Of/bootstrap存活、direct reader捕获；background marker的bootstrap只见barrier失败，direct app断真实marker值；旧cross-group/Merged弱oracle存活保留。08fa门禁第五项bootstrap-3因fixture不变量失败、后八项未执行，修为6fac后全新目录重跑。三轮各自绑定真实头，不覆盖或合计成最终头的一轮。

6fac两次focused race每次74顶层/286含子测试、8实际包，wall37.803/37.871s；完整13门禁全0、3146.177s，Web169文件/3436测试、E2E302。三片本机Go包均值299.336/307.5335/267.733s。exact branch [CI40468800650](https://github.com/open-nerve/NerveWiki/actions/runs/40468800650)十项成功，bootstrap Test441/309/371s，仅分支单样本。20项成本均在原界，保留IndexItems/grep旧失败与M10收尾义务，不推断长期稳定。原始证据身份/报告哈希与详细命令、各轮秒数见[S1审查](reviews/P2-S1-source-rules-review.md)。

最终修复头replay：fd00新目录whole-step-final-v4：109 variants /112 checks /336同argv执行，112 baseline与112 restore均0，mutant110次exit1、2次已知exit0。108项至少有一个预期catcher；test-input、test-fixture及cycle结构观察单列，不全部称产品语义kill。Of/bootstrap存活而direct of/of_subset捕获；active-background→sync继续存活；marker barrier、trace与用户结果边界保持。原93控制288份失败测试名集合与6fac轮一致；A/B/C八控制、fix1五控制、fix2三控制均重放。另11真实/fake对照exit0，原C-M3 Folder先拒绝与child先允许由真实HTTP/worker/SQL次序验证，主持者root-replay-integrity及root-counterexample-integrity均接受。focused race：fd00两轮同argv -race -count=1 -json -timeout 15m -p 1，均exit0、输入稳定、无fail/skip；wall36.692/37.655s，各74顶层/286含子测试，8实际包/9请求包，page/adapter/http无匹配且不计focused覆盖。rewrite1.40/1.53s、downgrade0.86/0.86s、seed0.79/1.87s；主持者root-focused-races已核。完整门禁：fd00完整13阶段串行、全exit0，总wall3155.474s；static-build7.951、race300.399、bootstrap1/2/3为288.549/308.360/262.781、costs331.525、guards469.401、web103.541、short54.698、e2e139.401，三片第二次308.944/312.807/267.028s。完整Web169文件/3492测试通过，完整E2E302测试通过；root-gate-acceptance核13条命令/日志与121份提交文件未变，未提高阈值。本机分片：fd00本机三片各两轮：make wall 1=288.549/308.944（均298.7465）、2=308.360/312.807（均310.5835）、3=262.781/267.028（均264.9045）秒；Go包1=287.873/308.122（均297.9975）、2=307.708/311.993（均309.8505）、3=261.978/266.243（均264.1105）秒。两种计时分开；最终两main Test均值已另按真实CI核验：328/313.5/394秒，不用本机值替代。代码exact CI：[CI40492563209](https://github.com/open-nerve/NerveWiki/actions/runs/40492563209)，exact fd00ee670c24db3ad4d9478419acd8340f1e6d91、attempt1，十作业completed/success，主持者与原C各核实时API原件。Test步骤：web219、race625、bootstrap1/2/3为365/441/396、costs379、guards745、short180秒；E2E182、image smoke66秒。仅一个代码分支样本，不能替代两次main Test均值；成本：本次fd00代码CI20项均在原界内：IndexItems每样本为20读总量，三阶段中位12.092768/12.053564/13.592351ms，对30ms；GrepCandidates三中位197.394567/184.531791/196.978013ms，对600ms；完整grep调用中位402.851941ms，对1000ms。新累计25个observed workflows/24个modern，IndexItems与Candidates各自72组/216原始样本，app call24组/72样本；三项历史失败保留，不用新界追改旧分类。6fac各自69/207继续作为旧历史，文档/main后续增加按届时实际回执更新；不从单轮绿色推稳定性。fd00本机21原始样本另列：IndexItems三中位10.201291/9.178542/10.357667ms、Candidates99.131791/93.979667/105.028250ms、call214.983917ms，均在原界；不混作CI样本。最终代码成本补充SHA256 abc63563b93aa5410aead1e037d479e28286a207e275b146c3ee9f3301970241。文档exact CI：已核通过，见[S1合并记录](reviews/P2-S1-source-rules-review.md#合并与后续验收)。主持者验收：主持者已接受fd00代码、完整本机验证、精确分支CI/成本及原C正式复核，三个Minor分别关闭。A/B初审无发现；6fac后仅两TS测试设施变化，server、Makefile与CI workflow对象未变，原范围结论保持；C独立执行三项修复与相邻回归并审计v4/CI。v4交付1594件/8,132,996字节、350外层执行与24份冻结程序/fixture已核。文档及合并main CI已核；记录CI与两main均值已由root核对，S2继续实施，P2/M10继续进行中。

合并见00号文档第12节与[S1合并记录](reviews/P2-S1-source-rules-review.md#合并与后续验收)。S1 main merge `bf8d6625daba554ee467aef2003346459657a060` 的CI `40501936360`、记录 `6a4f33b8fb621c66d97d38b3fea4fea0adbfc565` 的CI `40506120884` 均十项成功；三份Test两样本370/286、322/305、389/399 s，均值328/313.5/394 s，各<450 s，S2已释放。这是S1验收，不能替作S2 CI。S1不去重且来源UI仍待S3；M8仍open、S2/S3验收后才done，M9/M7/M6open，M5done。P2/M10仍进行中。

### 10.2 S2 来源 hash、去重与验证

**S2已经闭环，S3入口自然补记：精确 `66d8236c12aaa8923616f6f42f2ee33b772884f9` 的 [CI 40800844731](https://github.com/open-nerve/NerveWiki/actions/runs/40800844731)（main push，attempt 1）实际 11 个作业及列明必需步骤均成功；四片 Test 步为 431.0/358.0/308.0/322.0 s；精确 `8904a8769646c3b8919ff6a4c503eeae3ac70727` 的 [CI 40804121398](https://github.com/open-nerve/NerveWiki/actions/runs/40804121398)（main push，attempt 1）实际 11 个作业及列明必需步骤均成功；四片 Test 步为 435.0/448.0/290.0/317.0 s。两次 main 四片 Test 均值为 433/403/299/319.5 s，均<450 s；主持者已接受并放行 S3。旧失败与阶段原件保留。** 以下33b4实现/本机/CI及测试声明数为其历史接受范围：`33b4da5a38bed24db05411e83209d30ef1e17db9`，CI40718770843全部11作业成功；旧159义务=129＋TrashChildren13＋History8＋Name9，按具体输入fresh/reuse接受，不称159独特bug或全部提交后重跑；当时相对main新增107个Go声明=98race＋9!race，总2947。4550文档绿色、fb容量重排与失败、288恢复折叠及其失败CI40768488401、有界facts保留`f18180667a8fa47e44efd2b0d33e4623443eb389`的最新清单/门禁/CI集中在[S2验证表](reviews/P2-S2-source-hash-review.md#最终验证与集成)。旧失败、候选拒绝、metadata survivor、测试观察器失败及6M冻结证据限制均保留；B-N1在4550前规范文档投影后已关闭。S3实现与后续集成见10.3；P3–P5及M10收尾仍待，不开始M11。

| 已接受增量 | 提交 | 已验范围 |
|---|---|---|
| S2-1 | `443a8c8561cdc0fdb7f01c376bcf1136997a4ce3` | 00050、hash 维护/端口/两 delete/Forget；35项最终义务、42组历史triple；7组为替换旧单元轮的最终复演，旧7组保留历史，属于原子增量 [E02] |
| S2-2 | `05ca33eabf13aa8109efc3f5be2a2fa636072e08` | 有限父读者、10k 界及相关接口；20 组常规控制与 1 组树外 runtime coverage，后者 Go 仍通过、外部检查失败 [E03] |
| S2-3 | `9bafdaa67513de135c19f51b5ba9678cac59fe82` | 去重/provenance/snapshot/admit/import/字段全链；53 个最终语义变体、54 个历史 triple/162 次执行分别保留；D-I1、C-M1、C-N1 已复核关闭 [E04] |
| S2-4代码增量 | `25f88a2730456940bd52efffd899e8ac827a9f27` | 22代码文件已提交；六成本行双轮/六增量门禁及提交头gen-check已获root接受 [E39–E44]；六份root文档待提交，不是整步S2验收 |
| S2-5 | `c7cec1fa`→`ca8a27db`→`53488fee`→`056677a4`→`2536175e`→`33b4da5a` | 正式A/B/C与产品修复复核、159控制分层/依赖复用、实际本机门禁及CI40718770843已接受；B-N1实际文档复核已关闭，doc/main闭环待完成 |
| S2-5容量再平衡 | `fb92974b9928f334d2a1d6007fbd169cd6dfacf3`（parent45509391） | 两配置文件、窄名称边界及注释；完整本机双轮/独立核对已接受。CI40740783718 attempt1失败，三状态Revert sample2 HTTP500；保留失败成本报告，不能接受新双分支容量或main集成 |
| S2-5恢复行内hash | `288ecb2b6ccb9a1209e674074d685329be38f37d` | guarded prepared值合入原RestoreNodes UPDATE，通用维护及起点provenance保留；原本机与有限控制接受不变。CI40768488401三状态Revert sample2仍HTTP500，不能称CI修复成功 |
| S2-5有界facts保留 | `f18180667a8fa47e44efd2b0d33e4623443eb389` | 单次check只保留边界内纯facts，超界丢弃并重放原seq；最终代码f1818066的本机门禁、独立审查与提交后gen-check均已核定，精确代码CI40793294472全11绿。文档002a5638的精确CI40797519100全部11作业及必需步骤、code/docs分支四片Test配对，以及main合并`66d8236c12aaa8923616f6f42f2ee33b772884f9`的精确CI40800844731全部11作业及必需步骤均获主持者接受；本记录提交自身CI与双main Test配对仍由后续真实运行收口，完成前不开始S3。前33b4/4550绿、fb/288失败和03不精确同步的历史均保留。旧159（16fresh/143scope）、折叠7、缓存11分别核验；文档002a5638与main合并CI已验，记录自身CI及双main配对由外部回执收口 |

实现按实际 SQL 投影：probe/writeback 使用 SELECT-list SRF；当前 adapter 的 `[]uuid.UUID` 不表达 NULL 元素，缺游标用 zero UUID 加 `has_afters=false`，合法 zero UUID 不被排除。当前 writeback 改为单条 UPDATE self-join：`FROM nodes old` 与 zipped unnest 的 `v` 配合，以 `old.id=n.id` 取得旧值，保留 n 的 notebook/live/distinct 条件，`RETURNING n.id, old.kind, old.source_hash AS before_source_hash`。S2-1 当时的 MATERIALIZED previous/changed CTE 方案及旧值正确性验证保留为历史；本轮因无统计时的二次方末尾 join 修订，定向接受见 [E28–E31]。有限父读者内外均为 `LIMIT (SELECT sqlc.arg(max_rows)::integer)`，内层 parent/live 范围按已有 parent/name_key 索引顺序，notebook filter 在内层 LIMIT 之后，完整层读完再由 Go 排序；返回/物化界与真实物理访问分开验收。

公开 `Check/CheckJob` 的授权回调接收 snapshot ctx，D-I1 的真实 PG MaxConns=1 权限 SQL 与实际调用者 trace 为两层证据；不合称一个完整 OnConn/auth/server 场景。普通 `SyncRestoreNode` 例外增加一次 `StateOf` 点读，仍无额外来源 snapshot/扫描。原 A-I1“创建与 deferred rewrite 后 undo 留有 rewrite=true/base B”的公开链不成立，实际记录为 base=nil/rewrite=false；保留原设计审查历史并追加实施更正，合法模块历史/cuts 与真实 worker 分层证据不冒充该公开链。决定43的名义预检不搜索其他 cuts、前批保留取舍继续适用。

Probe 的最终双 tuple 上下界已获局部物理证据，失败候选历史保留。原 SQL 回退的 15 个叶子 IDs 仍正确：无统计 5 叶仅 tuple Index Cond 形状失败，ANALYZE 与 actual generic 各 5 叶另有真实 PK 访问；只有两种 exhausted 叶达到 200,001 次，修复版同例来源索引 1 条、PK 0。generic 查询确实 PREPARE/EXECUTE，计数 4 generic/0 custom；完整计划与 X 测量的 `force_generic_plan` 只是生产 Planned 路径设置，不称为同一实际 PREPARE 证据。[E05–E07、E11]

**局部控制与故事。** 九组控制同 argv、退出码 0/1/0；root 接受独立核过的 230 原件/919 项。八组改产品目标，#29 仅归零 seed7 测试输入；包含父事件的 baseline/restored 各 52 pass、mutant 43 fail/9 pass 不当作独立语义变体数。#24 捕获 7 个 last-hop 叶和 4 个 worker 后置叶，2 个 before-plan 叶仍通过。advisory 缺失只捕获真实等待缺失，tree 额外 advisory 捕获 5 s HTTP 客户端进展超时；session 201 使断言短路，未据此查得错误 session 行数。#43 的真实 Background.Run + fake Tree/history 与真实 page JobWrites + fake store/手写切分分列，不合成 HTTP/DB/worker 全栈。[E05、E06]

另有新增3组实际0/1/0控制，3171输入精确单处变更并恢复：两组是fixture/instrumentation（无统计状态被ANALYZE至nodes pg_stats=17；仅inspection设generic而实际四连接仍auto），其baseline/restored各3请求，mutant在计时前失败且0请求；一组是回退旧Set SQL的真实物理工作量控制，本次仅选1个physical测试。它们与原9组分列，不能称12个全产品语义kill；这次增量SQL控制只选1测试；最终124项控制中的SQL项已按old-key/set/planned等6个事件核验，详见E51/E55，不把两次范围混述。[E38]

六 seed 各 79 成功/9 拒绝，共 474/54，17 类成功各至少 3 次，降级后普通写/恢复另验；成功计数位于实际请求和独立树/正文/资产不变量之后。15 个定向 E2E（新10+既有 LW5 5）通过，覆盖 LW3 API/两 scope MCP、LW5 重复、LW6 历史/后台、LW7 import；保留初轮两例 MCP 缺必填 message 的夹具失败。LW6 的首批 100/201 保留、后101拒绝与持久报告已验。无 MCP 历史/import 工具处由各自 Go/API 补足；不是完整 E2E，也不是 S3 网页验收。[E08、E09]

**成本：c7以前已验样本与后续刷新边界。** 以下保留原输入上的测量/定界及当时接受状态；后续受影响行已按各修复重新实测与核对，当前pair见本文末表；本段数值仍只代表原输入。 环境 Go 1.27.1、Linux arm64、4 logical CPU、PG18.6；累计 TotalAlloc 不是 RSS。原请求4 s/提交2 s及既有成本界未提高。完整计划、X、暖索引和旧成本行的原始值在 E10–E13；本机容量估计不称真实 CI。

| 已验部分 | 实数与口径 | 最终状态 |
|---|---|---|
| 60k CheckRevert/PlanRevert | 54k页/6k附件、2048-byte/20-link 页、600名义批；三状态各路径各三次共18样本，最慢1661.868 ms，新界5000 ms逐样本；TotalAlloc约1.57–1.58 GB | 原样本保留；c7的31条成本/guard覆盖已由E64接受 |
| 完整 X 预检 | X=3/60k × 首部/末部/不存在 × 三设置 × 三次=54；最大中位2.906/327.599 ms，对界40/1000 ms；末部2/7 probe、4/60001返回，缺失2/7 probe、3/60000返回 | 已验本机增量；时间与实际索引范围分开 |
| 暖100k索引 | 历史三/四列7,774,208/9,551,872 B，差1,777,664 B；三配对建索引及真实HTTP写入均保留原样本/有符号WAL差 | 原样本保留；self-join修复后的HTTP写入对照与成本双轮已验 [E39/E40]；暖100k不替代冷6M |
| 五个 HTTP 形状 | 仅 ANALYZE 的15成功样本：新增154.862/127.791/134.229、移入429.596/417.878/427.675、恢复1134.623/1314.203/1545.791、撤销1661.835/1816.710/1988.313、Forget1070.729/1073.819/1039.994 ms | **历史首基线，不是最终三状态结果** |
| 五 HTTP 三状态 | 无统计 / ANALYZE / generic设置，每形状每状态3次，共15组45成功样本；36+9两个实际Go命令、同3171源快照 | root已接受基线及五预算校准 [E34–E37]；新增3控制已验 [E38]；六成本行双轮已验 [E39/E40]；c7本机门禁由E66接受，随后两次CI失败及修复刷新范围见下文 |
| 独立成本行 | 旧8行×2全新进程=16次通过；两轮均值×2.4估计31.813/48.811/57.324/60.969/123.075/97.954/121.566/68.909 s，均<300 s | 原HTTP5与索引写入对照保留历史；当前6行×2已验见下表；未受影响plan/precheck保留原范围 |

受影响六行每行两个全新Go进程均通过，同3171冻结输入，72个Go pass事件且无skip/fail；五HTTP行贡献90成功请求。原10m单行timeout不变，容量估计仍用两轮进程wall均值×2.4，并要求低于300 s；这不是实际CI Test时长。[E39/E40]

| 成本行 | 两轮进程wall s | 均值×2.4估计 s |
|---|---|---|
| create | 39.206 / 42.419 | 97.949 |
| move | 55.488 / 59.918 | 138.487 |
| restore | 72.544 / 79.195 | 182.086 |
| revert | 78.459 / 81.669 | 192.154 |
| forget | 73.068 / 74.505 | 177.087 |
| index正常HTTP写入对照 | 29.660 / 29.596 | 71.107 |

当前SQL的两轮暖100k索引对照仍各用相同语料配对，三/四列索引字节差均为1,777,664 B。四列减三列的正常HTTP写入时间差依次为−9.842250/+3.340875/+4.743083 ms与+0.926667/+3.824333/−7.362166 ms，写入WAL差也有正负，完整原样本见E39/E40；不删负值、不将噪声解读为纯id列因果开销。

HTTP45逐状态原值由E34/E35保留：root独立核六表统计、四条实际应用池连接的设置、原请求4 s/提交2 s、初始行/文件指纹与成功响应。generic设置不冒称实际PREPARE；36+9为两命令，分别16/13个Go pass事件，不能写成一次选择五形状。每形状各状态从相同初始数据开始；同一状态的连续样本会积累历史，Forget另用新clone。

| HTTP形状 | 接受的校准基线 ms | 新时间断言 ms | 取样口径 |
|---|---|---|---|
| create | 150.123667 | 500 | 每状态最快样本，再取最慢状态 |
| move | 465.672375 | 1400 | 全部有效样本最大值 |
| restore | 2531.533460 | 7600 | 全部有效样本最大值 |
| revert | 2513.224876 | 7600 | 全部有效样本最大值 |
| forget | 1013.237209 | 3100 | 全部有效样本最大值 |

五界均为基线×3后向上取整到100 ms；这是成本测试余量，未提高4 s请求/2 s提交限制。45原样本使用校准前的新来源预算常量；新增3控制已另获E38接受；六成本行双轮已由E39接受，c7相关Go输入绑定与31条成本/guard覆盖已由E64/E68接受。[E34–E37]

**三状态 HTTP 的实际失败与定向修复。** 首轮留下42个成功请求，以及revert/no_statistics首次请求500/internal_error（4.000720335 s，后两样本未执行）；move/no_statistics三次成功为2.831278001/2.857559001/2.861921752 s，超过历史ANALYZE-only的1.3 s界，不写成完整45样本通过 [E32]。同语料回滚诊断采用10k参数/9999变化，旧previous CTE×changed CTE末尾join拒绝99,980,001次比较；EXPLAIN 6.597701 s带仪表，不是HTTP请求计时，也不是捕获原请求字面参数 [E33]。

永久回归另用100k语料/10k变化：旧SQL为99,990,000 joinRejected与100,010,000 CTE行，实际失败；新self-join为0/0，old别名PK点读10k次、各1行。新生成SQL及原有old-key/no-op/rollback相关6个选中事件均通过；定向撤销无统计请求三次200为1.710994501/1.835675543/2.029426501 s。原请求4 s/提交2 s未提高；该处只接受定向修复 [E28–E31]；其后的完整45基线及定界另由E34接受，当前6成本行双轮已由E39接受，c7本机门禁已由E66接受；后来CI的ChildrenOfAll/reset失败另列，后续修复头已按范围刷新验证，见S2审查当前表。已冻结00050迁移SQL/二进制不受此查询变更影响，已接受6M结果保留，不重跑。

**迁移与运维。** 原49 CLI来自 `fd00ee67…`，50 CLI来自 `9bafdaa6…`，00050 SQL SHA `89a5cb6e57f3f9fd13032f3ed3a0b754fee58ccb1f4964fdd7371a5a580ce2e7`。正式 A/B 均为真实49起点、20本×300k=6M；A全普通/0 eligible，B前两本 LLM 来源100k/200k。两份数据构成相同，前两本 mode/source_root_id 不同。页5,040,980/附件959,020/垃圾100,000；页0/64/256/1024字节，附件元数据0/64/256字节；未物化资产文件。每轮 fsync 自有目录、逐文件 FADV_DONTNEED/fincore，驱逐后全部0页、启动后 nodes heap/index仍0页；只证明OS页缓存，不证明设备缓存。PG设置与分布细节承接已冻结迁移正文 [D02]。

00050 的实际次序是 ADD source_hash（初始全 NULL）→ CREATE 部分索引 → DO 回填。因此 A、B、第三份在 CREATE INDEX 时都还没有非 NULL 来源键，该阶段主要是扫描，不能拿来代表 populated100k 暖态重建。B/third 的最终300k条目索引大小、DO/整事务差与暖100k构建差须分别记录；一次6M四/三列差不能归因成纯 id 列的因果开销。B 最终 `after50` collector 的 storage section、`nodes_notebook_id_kind_source_hash_idx` 行的 `main_bytes` / `total_including_indexes_toast_bytes` 均为 36,806,656 B；CREATE 时的 reltuples=0/relpages=1 是未刷新的估计，不能当作最终大小 [E21]。第三份三列的两值均为 27,312,128 B，本固定语料四列多 9,494,528 B [E26]。

| 已验/待验运行 | PG事务 / 命令wall | server ALTER / index / DO | WAL/空间与边界 |
|---|---|---|---|
| 官方A，6M/0 | 1.678 / 1.711060 s | 493.706 / 1178.889 / 1.850 ms | CLI bracket1,053,648,248 B；PGDATA/WAL峰15,769,739,264/1,174,417,408 B；9286回执、28ledger/8048批、前后6M oracle/13表通过 [E14、E15] |
| 独立A CHECK/三列/四列 | 命令0.479914 / 0.360773 / 0.372451 s | 464.186 / 345.773 / 357.044 ms | bracket55,984/60,264/62,920 B；三次各自cold、4096回执、oracle/13表/最终4-key与CHECK通过；A全NULL，两索引均8192 B，只是空索引扫描 [E16] |
| 官方B，6M/300k | 21.580 / 21.610380 s | 405.030 / 1111.227 / 20061.088 ms | CLI bracket3,734,735,832 B；PGDATA/WAL峰16,078,131,200/1,375,744,000 B；9286回执、28ledger/8048批、前后6M oracle/13表通过 [E19、E20] |
| 独立B原DO | 同事务18.236 / psql18.253609 s | DO18226.946 ms，实际更新300,000行 | DO前后cluster LSN bracket482,303,008 B；reset bracket405,899,704 B另计；前后6M oracle/13表通过 [E22] |
| 第三份fresh49 B三列诊断 | 21.543 / driver21.558410 s | 463.042 / 1147.079 / 19930.782 ms | CLI bracket3,709,954,784 B；PGDATA/WAL峰16,085,606,400/1,996,500,992 B；9287回执、28ledger/8048批，前后6M oracle/13表与官方B初始数据一致；driver池4/官方CLI池10 [E25] |

A完整建库/装载/前后验证2232.683 s不是迁移停机时间；2230资源样本最小Available15,384,055,808 B>reserve6,242,815,181 B，startup596热页但nodes0。独立A886样本最小Available14,819,930,112 B，三次startup nodes均0。统计wal_bytes与紧贴CLI的LSN bracket不相互替代，累计统计可能滞后。[E14–E16]

B完整建库/装载/前后验证2227.942 s不是迁移停机时间；2225资源样本最小Available14,707,286,016 B>原reserve6,165,193,524 B，startup596热页但nodes0。nodes heap由1,054,203,904变为1,117,224,960 B，含索引/TOAST总大小由2,849,783,808变为2,951,602,176 B；累计wal_bytes差4,420,446,965 B与CLI bracket分别保留。[E19–E21]

独立B只在已迁移的schema50中清空并恢复前两本100k/200k hash，四列索引和CHECK仍在；旧tuple/index entries使其不等于fresh49。原样DO的同连接marker核PID/session/xid与本事务300k更新，5条SQL日志绑定实际DO和时长；紧贴DO的cluster LSN排除reset和之后COMMIT，仍可能含后台/检查点WAL，不称隔离DO WAL。root已核8083命令回执、28ledger与16,355实际输入SHA；8048个oracle/row流式stdout按原设计未保留大原文，账本逐项绑定receipt的query/output SHA。1208严格资源边界（含两组各601个oracle点）与1749连续样本均未越原B reserve；最小Available14,564,749,312 B，PGDATA/WAL峰16,108,802,048/1,073,754,112 B。startup596热页但nodes0；1750.547 s完整流程不当作DO时间，官方B结果SHA保持不变。[E22]

第三份三列完整流程2158.503 s含建库/全量验证，不是迁移停机时间；2155资源样本最小Available14,436,462,592 B>reserve6,123,985,306 B，startup596热页但nodes0。最终nodes heap同为1,117,224,960 B，含索引/TOAST总大小三列2,942,107,648 B、四列2,951,602,176 B。四列减三列的PG事务差0.037 s、DO差130.306 ms、命令wall差0.051970 s分列；单次运行、缓存/检查点及不同driver启动/连接池，不能推出纯id列的因果时间开销。三份6M与独立诊断的测量义务已获root接受，仍不等于S2验收。[E25、E26]

本轮补入官方B、第三份三列及成功400 backfill smoke各自确切owned容器/PGDATA清理记录；root验收确认目录消失、各清理回执指定的已接受原件身份不变，其余小库/失败候选不由此宣称清理。证据保留，不将清理当新测量或额外验收层。[E23、E24、E27]

两个400节点库分别验证 nodes强锁下reader/writer等待，以及取消原迁移、schema49/13表回滚后再Up成功；asset_blobs锁是人为屏障，不写成生产业务锁。另400库六次官方CLI roundtrip证明Down50保留rewrite、Down49丢标记，再Up49全false；排除nodes.source_hash/page_revisions.rewrite两列后13表摘要不变，186 hash重新构建。合成rewrite标记不是公开undo故事。升级先停旧实例、备份DB与附件、迁移并补运行角色权限后启动；00050持表强锁，Down/Up不代替备份恢复。本机数字不承诺生产窗口，M12须实机复测。[E17、E18]

**已验增量门禁。** 六项命令在同3171输入上通过：lint/knip 15.429 s，build 14.116 s，server/tools vet 12.555 s；server short 151.269 s、8140 run=6875 pass+1265显式short skip；tools short 1.093 s、27/27；未过滤page PostgreSQL包215.108 s、256/256无skip。22个server零选中包为无测试/生成模块，skip原因原件保留；这六项不冒充S2-5完整串行门禁。[E41–E43]

**正式审查与首轮 race。** A 在上述代码头无 Important/Minor/Nit；其独立控制为 baseline 78 pass、mutant 67 pass/11 fail、restored 78 pass。B 为 0 Important/0 Minor/1 Nit，独立控制为 27 pass → 26 pass/1 fail → 27 pass；B-N1 指本设计 §3.7、00及总体设计规范把当前 SQL 误写为 previous/changed CTE。本文已实际应用 self-join 更正，原 B 核三处规范及对应代码无新增发现，主持者核原件后接受 B-N1 **CLOSED**；原始发现与审查计数保留，闭环回执见[S2审查](reviews/P2-S2-source-hash-review.md) [F30–F32]。C 原件已核：真实 API/MCP/import 55 个 pass 事件、report 4 pass → 1 pass/3 fail → 4 pass、Web 268、E2E 15（每项一次、无 retry/skip/flaky）。C-M1 为 Web 历史 fake 将节点移到自身/后代时，先构造 projected 环再由 isUnder 遍历；真实服务端两种情况均 409 且状态不变，正常移动 200。有界观察器统计目标 projected 数组的所有 find 调用，旧/恢复 normal1、self51、child51，临时修复1/0/0；观察器异常不是产品响应或全局超时。正式修复已提交 `c7cec1fab9cc0586f4ba3754876aace2df79c570`，仅两Web文件变化，提交头gen及修复绑定已核；原C另核最终3171文件工作快照相对该提交有且仅六份文档差异，其余3165一致，不称3171全已提交；原C已在该提交上复核35项并由主持者核原件关闭，C-M1 **CLOSED** [E49、E50]。[E45–E47]

**ca8普通子树修复与原审复核。** 当前RD只用于普通`ChildrenOfAll`：去重父UUID，notebook/parent闭区间LATERAL、OFFSET0及外层live过滤，原11列与全层sort_order/id排序不变，无LIMIT或新索引。生成$1=Parents、$2=NotebookID；实验相反顺序经类型保持映射核同。有限来源入口是另一reader，仍有内外scalar LIMIT与完成层后Go排序，不把10k界施加给普通完整Subtree。初轮adapter267pass/223.795s，真实三状态各3move+3reset、439.521–500.438ms/60.831s，不变1.4s/4s/2s；只是初轮，最终全门禁另验。普通读者仍保留50001/20001节点各8读、64层、全序/cache/删除语义；数组含NULL元素只在外部SQL实验覆盖，Go []uuid.UUID不表达该输入。[E88/E91/E93/E94/E96]

作者四条old-ANY/global-order/skip-reset/inner-live-HTTP控制各0/1/0，另metadata inner-live实际0/0/0存活保留，不算kill。原B B2-N1要求backend track_counts=on及fresh fixture非空entries正证，sameargv7pass→7fail→7pass关闭；原C C2-M1在实际9move/9reset→3move/0reset→9move/9reset三阶段、root parent/hash准备断言精准失败后关闭。不同selector的旧B窗口与C首次分块parser STOP均保留原史。root已逐原件核两份修复复核，旧B-N1仍OPEN。[E87/E95–E99/E103–E105] 后续状态：以上OPEN为ca8修复复核时的原记录；本次实际文档投影后，原B已复核并由主持者接受关闭B-N1，见[S2审查](reviews/P2-S2-source-hash-review.md) [F30–F32]。

六快门禁在3172固定输入通过（server short8142run=6875pass+1267skip，tools27pass），root在ca8另跑gen-check0/3.448s且输入无变化。Makefile/CI与已审四片after逐字一致，实际8server+3=11作业；ca8当时99新增Go、91race=71nonbootstrap+20bootstrap、8!race是当时清单。最终129账本为原124fresh+4作者post-fix reuse+1B sameargv reuse，124Go/1coverage/4Web；该账本/四片计时/门禁后来已接受，后续修复及最新33b4代码CI见当前表。旧c7的121fresh+3reuse只属下面历史表，不套用于ca8。[E88–E90/E100–E102]

**c7控制与Go清单（已接受的修复前历史）。** 主持者已接受c7冻结输入上的124项义务：35/21/53/15四组，121项全新执行共363阶段，3项复用共9阶段绑定；119项Go、1项coverage、4项Web。coverage为Go 0/0/0而外部checker 0/1/0，其他123项0/1/0。catalog、观察器/计划策略、测试输入、仪器与AST/coverage控制单列，不称124个独特产品bug。5条原STOP来自动态地址/ISO时间/UUID的matcher差异，原状态保留，最终断言由完整error/error-continue重组和独立/root核对接受；不会把首片摘要改成完整断言。逐ID、分类、复用范围与全部receipt SHA见[S2最终控制表](reviews/P2-S2-source-hash-review.md#最终控制复演)。root另核执行后3171路径、mode、size和SHA全部恢复。[E51–E55]

新增Go清单从immutable `6a4f33b8fb621c66d97d38b3fea4fea0adbfc565` 到 `c7cec1fab9cc0586f4ba3754876aace2df79c570` 逐函数重建，仍为97新增：89项race eligible=69非bootstrap+20bootstrap，8项!race。25f→c7仅两Web文件，全部Go输入不变；正式审查追加的是控制/HTTP catcher义务，未新增committed Go测试。未入仓的review临时测试与诊断不计入97。[E56]

**c7全部本机门禁（已接受的修复前历史）。** 全非bootstrap首轮7470 pass+1固定Markdown成本skip；第二轮focused为13命令248 run/pass，69新增tops各两次的Elapsed已由双raw重算。bootstrap三片各两轮，每轮468 tops+3020纯descendant=3488 run/3484 pass/4固定Markdown race skip（含2top），20新增tops每轮各一次；仅3个源码固定UUID叶标题不同，原名/loop位置保留。25 fresh成本/guard命令105 run/pass、0skip/fail，另6条按相关Go输入一致复用双轮，共31成本/guard recipe无漏重（含tools，short另1条）。4个bootstrap race skip及首轮非bootstrap Markdown成本skip均已在无race下实际补齐。完整Web3548通过并仅延续已核标题改动；完整E2E108文件312测试/312次成功，无skip/retry/flaky，wall139.573777 s。静态/构建/生成/短测、完整命令原件与reuse范围见[S2验证表](reviews/P2-S2-source-hash-review.md#最终验证与集成)。[E63–E70]

| bootstrap分片 | 两轮Go package Elapsed s → 均值 | 两轮命令wall s → 均值 |
|---|---|---|
| 1（168 tops） | 289.720 / 297.166 → 293.443 | 296.025730 / 297.981815 → 297.003772517 |
| 2（151 tops） | 400.897 / 400.504 → 400.7005 | 401.670684 / 401.308331 → 401.489507546 |
| 3（149 tops） | 327.654 / 327.567 → 327.6105 | 328.510077 / 328.360416 → 328.435246511 |

每轮三片run为743/1883/862、pass为743/1879/862，4个固定skip均在第二片。上表是本机真实量，package与wall不混用，不是CI Test均值；SHA前后图不证明进程中的瞬时修改，结束后3171路径/mode/size/SHA全恢复已另核。[E63/E66/E71]

本轮IndexItems30ms守卫三状态中位9.633750/12.240333/10.054916ms，GrepCandidates600ms为90.496750/89.577833/95.540458ms，均未复发；原阈值与请求/提交期限未调。长期CI稳定性仍归M10收尾，不由本轮绿关闭。[E64]

**后续CI修复与当前结果。** c7两次reset失败后RD实际整合，5348及0566两次Restore/4s失败继而修TrashChildren和TrashHistory；2536在Restore九次200后Revert三状态sample2仍500，33b4只减少planner.before无关名字候选分配，保留完整冲突语义。该次精确CI40718770843全部11项成功。没有提高4s请求/2s提交、7.6s Restore/Revert成本界或既有1s History界；一个绿分支不证明统一根因或长期稳定。[F01–F16]

当前32条成本/guard recipe（17costs/15guards，含tools；short另1）逐条有实际pass及依赖证明；新History400k每轮四次真实计数，core16最多20101的计划/语义结果与400k永久守卫分开。TrashChildren13与History8、Name9反向控制不揉进旧129全新执行；Name后旧129为14直接＋1保守fresh、114复用。各阶段声明数/原件、!race补测、159分类及当前门禁计数见[S2审查](reviews/P2-S2-source-hash-review.md#33b4最终代码控制与本机验证)。NAME-REV-01旧0/0/0覆盖缺口及后续精确HolderID失败修复保留，原review02已关闭。[F03–F13]

| 当前正式成本行 | package两轮s | 均值×2.4 s | 复用边界 |
|---|---|---:|---|
| create | 44.754/41.323 | 103.2924 | 既有具体不变路径 |
| move | 59.174/52.601 | 134.1300 | 既有具体不变路径 |
| restore | 76.629/75.649 | 182.7336 | History后fresh |
| revert | 92.676/92.311 | 221.9844 | Name后fresh |
| Background check/plan | 68.300/69.535 | 165.4020 | Name后fresh，18check＋18plan |
| forget | 66.868/66.867 | 160.4820 | helper后正式pair，旧fixture失败不入pair |
| History400k | 9.911/9.977 | 23.8656 | 独立recipe，每轮4×Count400000 |

当前pair以GoJSON package Elapsed计算，旧S2-4 wall估算保留原单位，不混为同一值；本机<300s不是CI保证。Revert18HTTP200最大3.063165043s，Background check/plan最大2.388390585/2.409432001s；!race helper只排除autovacuum worker，普通client仍拒绝，旧失败PID身份未知。原173份受保护的是外部CI/预算证据，源码另由固定Git/来源图证明。[F01/F11/F13]

四片本机双轮package均值319.5785/284.784/240.4775/262.5225s（ca8受测来源，后续相关caller另fresh）；33b4单分支Test444/419/302/271s。33b4 CI Restore/Revert各9次200，最大2.467645682/3.130127760s，后续Background18及Forget9完整通过；History四个实际400k Count为171.483340/198.331556/191.590538/158.465408ms，named非执行EXPLAIN计划不当作Store实际执行计划。guards Test1127s/job1146s分开，11作业success已由原件接受；main双样本仍待。[F14–F16/F22]

以上均为33b4阶段历史。4550文档CI40730123058全11项成功，旧第一片与33b4配对均值450.5s触发决定44；重排fb CI40740783718虽四片Test356/308/235/371s，整体因Revert三状态sample2 HTTP500失败。恢复折叠的已封存本机旧/候选交替观察为18/18请求均成功、均值2.283366749→1.983884524s（-13.1158%）；它不证明CI根因、物理写次数或新头CI成功。最终输入的完整门禁/两轮四片与精确新代码CI见[S2审查](reviews/P2-S2-source-hash-review.md#最终验证与集成)；失败历史、成本报告与旧容量预测保留。

恢复折叠代码288的精确CI40768488401仍因Revert三状态sample2请求4s失败；四片Test297/454/340/359s只是失败workflow的一份分支样本。随后有界facts保留的本机AB结果混合；组件内check、进程总分配、HTTP、package与wrapper是不同量纲，不能由组件改善推断CI根因或普遍端到端收益。正式Restore/Revert package两次均值×2.4、其余成本与当前32 recipes按最新原件另核，不能拿失败CI或旧fold AB补最终样本。

**S2入口闭环补记（S3既定文档自然转录）。** 精确 `66d8236c12aaa8923616f6f42f2ee33b772884f9` 的 [CI 40800844731](https://github.com/open-nerve/NerveWiki/actions/runs/40800844731)（main push，attempt 1）实际 11 个作业及列明必需步骤均成功；四片 Test 步为 431.0/358.0/308.0/322.0 s；精确 `8904a8769646c3b8919ff6a4c503eeae3ac70727` 的 [CI 40804121398](https://github.com/open-nerve/NerveWiki/actions/runs/40804121398)（main push，attempt 1）实际 11 个作业及列明必需步骤均成功；四片 Test 步为 435.0/448.0/290.0/317.0 s。两次 main 四片 Test 均值为 433/403/299/319.5 s，均<450 s；主持者已接受并放行 S3。旧失败与阶段原件保留。 本次依据树外`root-s2-completion-and-s3-release-8904.json`（SHA256 `795a5508863326df0f51ae8ccb40867642a16e60190149b26de2d188b53bb235`）；旧33b4/4550、fb/288失败、03屏障误匹配、折叠7/缓存11与旧159的各自范围完整保留，不全局改成成功。以上S2原段的pending均保留当时阶段语境，当前S2已闭环；S3见10.3，P2/M10仍进行中。

### 10.3 S3 完整来源网页

入口为S2记录`8904a8769646c3b8919ff6a4c503eeae3ac70727`，Phase原设计基线仍403841。实际提交c8772640（来源能力/移入确认）、424601ec（会话结束/真readonly）、98bc8f79（显式标题/原字节mixed队列）、1da53366（duplicate/真实故事/模板48），审查修复`8c17b5cceb65b2a0f70976bbe4752daeaabaee05`；最终受测代码`ae4501aa166771db153dae5dfa710ef16ee26ac4`，tree `8e31387857571a2db6baf48c092382b66adc77d4`。合并见00第12节。

三份独立初审在1da核0Important/3Minor/0Nit，另C-Q1新增slow预算与C-Q2输出路径精度。A-M1携来源根祖先换父、B-M1失败MD新retry当前reader、C-M1旧缓存区外目标移入来源后不补读分别修复；原席复核见[S3审查](reviews/P2-S3-source-web-review.md#初审发现处置与原席复核)。C-location的0/0/0存活及卸载防御解释保留，不冒充kill。

Web证据使用实际UI→stores→service/fakeHTTP、独立字节oracle、非空分页、队列与abort；LW3–7/LW12浏览器使用真实产物/HTTP/PG/文件，保留API/MCP和持久断言，含当前已存在区外目标的stale补读、source loss释放锁、mode事件同context恢复可写及普通同字节新增。后台前批保留/失败批回滚与同步整单元回滚分开，报告被拒node与duplicate target分开。

决定48的现行双语raw入口和大小已同步P1规范；只影响今后新建模板。初轮42/46、类型/旧按钮/console期望和lazy-route夹具纠正保留；最终状态依据下列新输入，不套1da旧28控制。

Web 178 文件/3812 通过；short 8208 run=6915 pass+1293 明确 skip；完整 E2E 108 文件/329 通过，0 skip、0 retry、0 flaky。

| 门禁 | 本次范围 | wrapper 秒 | 结果 |
|---|---|---:|---|
| `race` | 最终代码输入新跑 | 1100.969 | pass |
| `bootstrap-1-round1` | 最终代码输入新跑 | 314.974 | pass |
| `bootstrap-2-round1` | 最终代码输入新跑 | 317.963 | pass |
| `bootstrap-3-round1` | 最终代码输入新跑 | 255.320 | pass |
| `bootstrap-4-round1` | 最终代码输入新跑 | 262.871 | pass |
| `costs` | 最终代码输入新跑 | 444.634 | pass |
| `guards` | 最终代码输入新跑 | 877.449 | pass |
| `e2e-all` | 最终代码输入新跑 | 295.788 | pass |
| `bootstrap-1-round2` | 最终代码输入新跑 | 309.390 | pass |
| `bootstrap-2-round2` | 最终代码输入新跑 | 319.021 | pass |
| `bootstrap-3-round2` | 最终代码输入新跑 | 265.440 | pass |
| `bootstrap-4-round2` | 最终代码输入新跑 | 260.254 | pass |
| `lint-gen-knip-build` | 已验原件按逐文件未变输入复用 | 9.044 | pass |
| `web-all` | 已验原件按逐文件未变输入复用 | 113.498 | pass |
| `vet-server` | 已验原件按逐文件未变输入复用 | 1.409 | pass |
| `vet-tools` | 已验原件按逐文件未变输入复用 | 0.103 | pass |
| `short-server` | 已验原件按逐文件未变输入复用 | 159.727 | pass |

非组合根race实际7579 run = 7578 pass + 1 skip；package 90 pass/22 skip。既有成本测试的race skip由非race成本/guard原recipe补跑；不把skip当作pass。完整命令、每轮原始日志SHA及最终3199份输入恢复均在验收原件。

明确复用五项：ae4501测试增量的static/gen/knip/build与全Web；8c17产品修复的server/tools vet与short，经3198个原Git条目/文件字节不变桥加新增测试绑定最终3199输入。Web178文件3812测试、short8208=6915pass+1293skip取各自实际原件。完整Go race、全部cost/guard、双轮bootstrap与完整E2E均新跑；旧47例Wiki子集没有冒充本次完整329例E2E。

新增Go race测试情况：最终S3相对8904没有新增或修改Go测试/Go源码（实际差异为两份嵌入raw模板）；race编译清单实际470个组合根顶层，分片167/74/114/115，新增Go race测试为0。因此没有冒用S2旧新增测试清单；模板嵌入输入改变后的全Go门禁和四份双轮另按最终头执行。

编译后的实际清单为470个顶层测试；四片167/74/114/115个，每轮并集完全相等、交集为空、每个顶层恰好执行一次。以下为Go package秒，另列进程wrapper秒，不能当CI的Test步骤秒。

| 片 | 顶层数 | 第一轮package | 第二轮package | package均值 | 两轮wrapper |
|---|---:|---:|---:|---:|---|
| 1 | 167 | 314.009 | 308.527 | 311.268 | 314.974/309.390 |
| 2 | 74 | 317.072 | 318.075 | 317.573 | 317.963/319.021 |
| 3 | 114 | 254.386 | 264.488 | 259.437 | 255.320/265.440 |
| 4 | 115 | 261.947 | 259.285 | 260.616 | 262.871/260.254 |

既有race跳过项（按测试名去重）：`TestTheAppsLinksAreWithinTheirBound`, `TestTheAppsMarkdownCostsAboutItsSize/to_a_page`, `TestTheAppsMarkdownCostsAboutItsSize/to_an_image`, `TestTheAppsMarkdownRendersCheckedHTML`。这些非race专属测试在完整guards原recipe实际执行；未提高测试预算、未改变分片边界。两次main的Test均值留待合并及记录CI真实验证。

最终输入完整执行Makefile原17个costs和15个guards recipe，命令清单逐行吻合且无空选择。包含IndexItems、grep、来源新增/移入/恢复/撤销/十万Forget和计划测试，所有原界通过；原始样本保存在costs.log/guards.log。这是最终输入的一轮完整门禁，不是重做600万迁移或新建重复成本基线。旧IndexItems30ms、grep600ms持续观察及M12真实服务器/数据/并发负载义务保持，代码CI样本已按下列原件独立核定。

精确代码CI 40843325190的20项成本指标和45个真实HTTP样本（5种操作×3统计状态×3次）已核原始日志、预算与状态。各操作HTTP最大秒数：create 0.359200391，move 1.047166084，restore 3.228866466，revert 3.970788992，forget 2.227704099。revert离原4秒请求期限仅29.211ms；保留为后续CI/M10收尾的窄余量观察，不由单次通过宣称稳定。guards的Test 1553s，整个job 1571s（原job上限1800s）；Test与job分别记录。没有抬高任何预算，也不以较快的样本替换这次最慢请求。

原席产品结论绑定 `8c17b5cceb65b2a0f70976bbe4752daeaabaee05`；最终 `ae4501aa166771db153dae5dfa710ef16ee26ac4` 只有测试差异 `web/apps/web/src/pages/notebook/source-drag-hints.test.tsx`。根核全部 3198 份既有Git entries/blob与文件字节未变；产品未改、测试增量另按根回执接受，原席未被记为重跑后续全Step。原件 `/home/parallels/.local/state/nervewiki/m10/p2-s3/s35-test-only-final-code-bridge.json`，SHA-256 `3c6d20185ec954adfb532ea6014f2f3eac16301710eff9090c3e1830ac797a03`。

精确 `ae4501aa166771db153dae5dfa710ef16ee26ac4` 的 [CI 40843325190](https://github.com/open-nerve/NerveWiki/actions/runs/40843325190)（m10-p2-s3 push，attempt 1）实际 11 个作业及列明必需步骤均成功；四片 Test 步为 453.0/311.0/289.0/371.0 s。全部控制及每轮argv、失败断言、恢复/输入hash见S3审查与树外原件；S3新增/原审追加和旧S2库存分开。S3集成记录（2026-10-29）：代码`ae4501aa166771db153dae5dfa710ef16ee26ac4`/[CI40843325190](https://github.com/open-nerve/NerveWiki/actions/runs/40843325190)、文档`f6b397b7234f2495ac092be8214b8bf266a95c91`/[CI40848348596](https://github.com/open-nerve/NerveWiki/actions/runs/40848348596)和no-ff合并`ecd79c77d0cdb2259d50758b26514f61ebb329ca`/[main CI40852524505](https://github.com/open-nerve/NerveWiki/actions/runs/40852524505)均已按精确SHA/attempt接受；两父与同树证明见[S3审查](reviews/P2-S3-source-web-review.md#合并phase边界与移交)。本次合并记录写作时尚未产生自身提交号/CI；两main分片均值及P2完成另在10.4自然补记。

### 10.4 P2done 的待闭环事项

S1/S2已验；S3实现、三审复核、最终门禁和精确代码/文档/合并CI按10.3记录。本次合并记录的精确main CI和合并/记录两main四片Test均值<450s尚待提交后核对；接受后独立作`docs: M10/P2 done (M10/P2)`，核§8.3及handoff并更新00头/§12/§13和本节。取得P2done自身精确main CI全绿后，主持者才放行P3。

根逐项核P2 §8.3/P1 §9与移交：M8的S1/S2服务端和S3历史页面义务已有对应证据，分支阶段保持open，P2done最迟done；M7去重页面完成、lint/贡献者等P3/P4仍open；M9写契约与页面消费完成，工具/虚拟/prompt属P3–P5仍open；M5 mode消费已验且done保持；M6六行与注意项仍P3/open。M11只交接既有来源/hash/rewrite契约且升级未开始；M12保留既有6M与真实生产主机/负载/停机窗口的未验边界。无新打磨事项，因此M5-polish不改。最终门禁/CI与S3合并、记录、P2done是后续实际验收条件，不预填完成。

M8来源历史实施义务已逐项覆盖，当前等待P2done核对后将状态改done；M7/M9整份仍open，M5done保持，M6lint仍open。M11只接既有契约，M12保留实机测量；P3是下一Phase但未开始，不在本次写P3设计，M10继续进行中。

## 附录：核对过的代码与证据

路径均相对仓库根，按`403841fc75d0e5ac6b5f39447158213c0e07b2fe`核对；外部manifest列blob OID与本次阅读用途。长文件只声称核过所列相关段，不声称通读每行。

| 代码/文档 | 已核对事实 |
|---|---|
| docs/README.md、claude_to_codex/README/02-workflow/04-owner-decisions-and-preferences/06-environment、M10/00与P1§8–10 | 权威/工作流/独立审查/文档/当前P1完成边界 |
| page/app/extension.go、unit.go、unit_content.go、domain/contract.go、app/mode_guard.go、bootstrap/registrants.go | Step现状、guard/participant顺序、same-byte早返回、仅lock session veto、普通绕过 |
| page/domain/batch_plan.go、batch_contents.go；app/apply_batch.go、edit_page.go、unit_import.go | 每项瞬时tree/合成body、MCP锁前缝隙、导入直接insert路径 |
| page/domain/revert*.go、app/revert_contract.go、revert_changeset.go、jobs.go、unit_revert.go；transfer/app/background.go/batches.go与实际job测试 | content-only无Tree、原Ops/compact/本批group合并区别、sync/bg Tree=true；Nodes/Bytes贪心、真实step下标halving及Joined、现有合法fixture |
| page/app/unit_move.go、unit_relocate.go、adapter/postgres/store.go及nodes.sql、domain/batch_plan_test.go、00042索引 | 原Subtree BFS无节点LIMIT且每层sort_order/id、普通单move10001成功、restore与place界不同；现有partial parent/name_key序可供新reader提议，实际计划尚未执行 |
| page/adapter/postgres/queries/changesets.sql、history.sql；app/fakes_test.go；migrations/sql/00048* | RecordRevision continuity、rewrite尚不存在、迁移编号最后48 |
| asset/attachment_infos.go、adapter/postgres/queries/infos.sql、bootstrap/assets.go、notebook/app/downgrade_notebook.go | SHA端口缺口、Forget接线时序、ModeSubscriber原事务 |
| platform/postgres/tx.go、shared/tx.go、page/module.go/deps.go | 既有WithinSnapshot只读RR/禁止嵌套/取消释放、page已有Snapshots，不需新平台事务 |
| shared/error.go、platform/httpserver/apierrors.go/problem.go/mcpserver/errors.go；page/refusal.go；bootstrap/transfer.go；transfer报告链；api YAML | duplicate_of各层缺口、Refused既有链与日志边界 |
| web stores/page-tree/asset/edit-session/page-editing、source editor/lock-read-only registry、页面tree/move/assets/trash/revert/changeset/transfer-report | 全节点/页面树区别，upload Asset返回约束，lose清id，富文本/附件导航现状 |
| e2e/stories/wiki/lw12-downgrade、lw2-protected、fixtures/wiki-pages | 现有LW12stream、nestedAGENTS raw fixture、settleEvents会写sentinel |
| Makefile、总体§13.4、maintenance两main receipts | 分份正则、serial constituent命令、cost独立行、真实Test均值与既有欠项 |

只读recon（含move-bounds memo、UI helper回报、completion checklist）是线索，不是权威或实施结果。入口p2-entry.json与缓存preflight仅支持其各自有限结论。冻结设计/拟00修订/manifest及三份原审查、复核的SHA见[P2设计审查](reviews/P2-design-review.md)；正式正文只更新接受状态与审查链接，实施结果仍待填写。
