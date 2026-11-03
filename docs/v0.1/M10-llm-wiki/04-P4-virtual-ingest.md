# M10 / P4 虚拟页面与 ingest：架构设计与实施规划

| 项 | 值 |
|---|---|
| 状态 | **正式设计与S1原代码/门禁/原席核修、878/f689测试修复及ac84拆分已接受；2fc文档/97c9 no-ff与e650/a2整CI已接受。e650 record的a1 lint失败保留，a2完整14/40/11已接受但仅本次未复现；指定97/a1＋e650/a2首片Test均值466.5s不满足严格<450s。已按既有M9/P2§6.5实施Make两selector容量维护，代码`af392f641f043e9497ce39abd7734ac3d55007c3`已正常提交/推送，旧字节native17、B-N1注释桥接、新字节minimum02、提交后postgen与原A/B/C第二轮本机/源码补核已接受；CI41556086210/a1因revert12.45394147s>原12s失败保留；预登记的同head完整a2已按14/40/11及原成本/guards/lint接受，原A/B/C03已核代码与完整代码CI、无新增I/M/N；本轮文档`3166bbcde7e23b4eb614b6367ea7e95c801e91cb`已提交并正常推送，精确分支CI41566238435/a1：[完整接受](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-document-ci-final-observation-01.json)；本轮no-ff main `e07cc1964238b3d248c791c124d9a9fc48dd9253`，CI41569049575/a1：[完整接受](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-main-ci-final-observation-01.json)；record ca4688a1的完整CI与预登记e07/a1→ca/a1配对已接受，S1实施与主线集成已闭环；决定76的S2正式设计获绑定放行，本次设计自身minimum10/commit/postgen/CI待验，S2实施未放行。S2/S3未实施，P4尚未完成**；此前各失败与pending按原写入时点保留，最新见[§10.1.2](#1012-容量维护文档与合并补记)及[S1自然转录](reviews/P4-S1-virtual-files-review.md#s1修复record后验随s2正式设计转录) |
| 文档路径 | `docs/v0.1/M10-llm-wiki/04-P4-virtual-ingest.md` |
| 基线 | `6992fd042ff06c110a4e59e67fb89bfe5f43fbd9` / tree `bd74094f1e4b4fbaf04085431e2ba641f7e6c134`；产品源码承接 `78167b424e5911fbd072ab0a047d82a39526a10d` |
| 日期与入口 | 2026-11-01；6992 own main [CI41284728142 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41284728142/attempts/1) 已按13作业／38必需步骤接受，主持者已释放正式设计 |
| 角色与接受状态 | 正式设计三审及原席核对、决定71/72和e5e8入口CI已接受。S1代码审查：77初审A无发现、B有B-I1 Important、C有C-Q1；精确0b0第一次原A/B/C核修均通过，B-I1与C-Q1关闭，新增Important/Minor均0。实际对象、发现/修复和原件见[S1记录](reviews/P4-S1-virtual-files-review.md)；作者与ROOT预读不充fresh席 |
| 迁移与决定 | 固定6992迁移尾号00052；计划 `00053_page_ingest.sql` 尚未实施。实施决定接70，§2.3与00§11正式采用71／72，负责人可以改判 |
| 依赖 | 已批准 00 §4.2/4.9/4.13/4.14/7/8/9、P3 §3.8.1–3.8.2／决定 62、02-workflow §2 |
| 材料组成 | 本文与 `specs/P4-virtual-ingest-ports.md`、`specs/P4-virtual-ingest-queries.sql`、`specs/P4-virtual-ingest-test-completion-matrix.md`、`specs/P4-virtual-ingest-00-revisions.json`、`specs/P4-virtual-ingest-types-boundary.md`、`specs/P4-virtual-ingest-files.json` 是同一正式设计，附件不是以后再填的接口空白 |

§2.1、§2.3、§7中设计commit/CI待产生、当前未实测等准备态原文保留为e5e8正式设计发布前记录；S1实际结果见§10.1及S1记录，S2/S3义务仍待实施。

本文保留已接受的正式设计合同。S1已实现统一虚拟路径、有限目录、RR虚拟read与真实export；代码、完整本地门禁、精确修复CI与三位原席第一次核修已接受。原a1f成功/fd两失败及b4 glob失败及986文档guards超时取消保留，878会话与f689 grep夹具测试修复、本地验证及精确代码CI已接受；修复文档`2fc103dd`及本次no-ff `97c9ad8e`的精确CI已接受，本record自身CI与新指定两main配对待验，S1集成未闭环。十二个代码/测试/CI提交、77正式三席、0b0修复与集成分别见§10.1及[S1记录](reviews/P4-S1-virtual-files-review.md#修复文档与no-ff合并记录)。S2/S3及00053迁移未实施，P4尚未完成。

## 0. Step 划分与独立为绿

| Step | 可独立交付的行为 | 同 Step 的网页与验证 | 不依赖后一步的理由 |
|---|---|---|---|
| S1 虚拟文件与目录服务 | 统一虚拟路径；MCP list/read/open_notebook；REST listIndex 与 MCP 共用用例；两个纯生成器及整本导出贡献者 | API 生成物、双语码、必填字段／fakes；LW10/LW13 API 与两 MCP scope；虚拟读、贡献最后一跳、100k 成本 | log.md 先输出现有历史；无 ingest 字段。网页入口留 S3，但新 API 可由真实客户端直接使用 |
| S2 ingest 与历史 | Before、删前 Capture、完整 coverage、候选 SQL、同事务 marks；迁移／grants／sqlc；历史筛选／来源、清理与不变量；log.md/export 加来源 | 历史 required 字段、types/fakes 同步；LW9 API 与 MCP；真正并发、计划、容量、迁移和最大 batch | S1 文件已有可扩展的日志 DTO；新增字段和真实读者同一提交接通，普通历史一直可用 |
| S3 网页 | 两虚拟行、目录页面与普通本入口、历史 ingest、共享虚拟链接判定 | LW9/LW10/LW13 页面，LW12 只加虚拟行随降级消失；组件、浏览器、焦点／连接证据 | 只调用已经合并的 S1/S2 接口；不再改变 ingest 判定或迁移 |

每 Step 先完成每提交 minimum、**完整本地门禁与全部指定变异／成本测量 → 推分支 → 精确代码 SHA 的绿色分支 CI → 三位独立代码审查**。修复后先补／重跑规定门禁与所有变异、推送并接受精确修复 CI，再交原审查者核对。干净后 Step 文档提交及 CI，no-ff main 及 CI，merge-record 文档及 CI。三 Step 后才 Phase done 及 CI。正式设计本身另先经三位 fresh 设计审查、修订与原审查者复核；前期准备审阅及作者静态检查不能抵任何一轮。审查干净后，正式设计、审查记录与受控00修订作为一个文档提交，接受必需门禁及精确main CI后才开始S1。

## 1. 基线与已核事实

固定 6992 的 `ReadPath.Execute` 明示 unlocked/no transaction；`ListIndex.Execute`、历史读同样没有自带快照。page.Deps 已有 Snapshots；ToolDeps 在 `Module.Tools` 校验，不能把 Virtual 的 nil 检查写在 `page.New`。transfer 的 fill 已拥有读快照，贡献者在其内执行，不能再开。`WithinSnapshot` 拒绝已有事务，Writer 使用 ReadCommitted。这些是快照方案的实际出发点。[F01–F04]

现 Writer 先 workspace SHARE，再 notebook SHARE（普通正文）或 NO KEY UPDATE（改树），再正文／changeset；linking observer 才取 notebook index advisory lock，接着 Replace/Delete、resolve 与发布。source-hash advisory 只在适用的 SHARE 来源新增路径取，在正文后、index 前。不存在“所有普通保存已读完整元数据”；ApplyBatch 的 ListNodes 是 Writer.Run 前的无锁 planned 数据，不能当 unit completeness。[F05–F08]

来源索引当前为 Extractor 2：正文 PropertyKey=NULL，frontmatter 空属性为非 NULL 空串。旧 v1／missing 必须 Unknown；HasSourceProperties=false 只证明那次候选属性流无 sources 属性，既不证明没有正文边，也不证明旧页不存在。P3 已提供 SourceSlots、SourceArea、metadata 与 jsTrim 规则，没有交付 ingest Capture／coverage／marks。[F09–F11]

现 children 游标只有 O/I；虚拟根键守卫已有，尚无路径 lookup 包装、V 游标、虚拟内容、REST 目录或 ingest。固定6992树最大迁移00052，计划下一迁移为00053_page_ingest.sql；采用或实施前若main增加迁移，先核号并同步本设计。已有 purge 先叶后父、历史三种 SQL 路径、export contributed/metaPaths 可增量扩展。[F12–F16]

P3已完成：6992是六文件P3done文档提交，parent为df9b1d49544336f80d768bde4b6cda231403231e；其own CI41284728142 attempt1（push/main/6992）已完成成功，13个作业及38个必需步骤由主持者接受。实际完成／放行回执为 `root-p3done-completion-p4-release-01.json`，SHA256 `b6375d24f2e3b352060d10f9b965356250a063fd84e8f0d18f5d66ff8bbddaad`。P3原§10.3的“自身SHA/run未知”保留其写入时点，本节自然补录实际入口。已接受指定配对仍仅为修复merge4e6e／CI41277758157a1与联合recorddf9／CI41281561763a1，五片Test均值394.5/201.5/308/387/314.5s；原13ed／CI41247708831a1 FAILED保留。6992 own CI不加入该pair，亦不证明P4新增测试已有安全余量。

## 2. 目标与范围

兑现 00 P4 三 Step 全部范围。来源区仍以 notebook mode/source_root_id 和严格 live 后代判定；根自身不算来源，来源数是非空页与附件的总数。虚拟文件只读、无实体节点／revision／search 索引；index 目录不调用 lint、不显示待消化数量、不占 Runs。历史 ingest 是 edit changeset 的追加标记，不是新 changeset kind。

决定76补充MCP历史来源的wire预览，不改变ingest判定或原三代码提交。以下采用的是设计语义；S1集成与S2入口仍以真实main/record接受回执为准，不由本补充放行。API/tools.json、Go/TS及原生生成物在S2-1实施，本次正式设计修订只改文档。

### 2.1 对 00 的受控修订

[00修订数据](specs/P4-virtual-ingest-00-revisions.json)含15个6992唯一old锚和完整new文字：原13锚保持，A04–A07随D/F与current证明修订；新增A12目的地身份、A13导出映射。决定71明确修订old-F紧凑关系的保留政策，决定72采用types方案。三审与原席核对已完成，主持者已接受；15个old锚原字节保持，new仅同步接受措辞，另三处发布状态替换与实际00投影同列于附件。

原13处与新增2处old锚均在6992静态核对。采用时由主持者使用已准备的 `--check`／`--path` 全有或全无脚本，一处不匹配则不写任何文件，先在干净git archive副本试。若main变化先重绑；本次正式文档自身commit/CI仍待产生，接受设计不等于文档门禁或实施通过。

正式采用还须更新00的当前进度／审查／决定／变更记录，不把这15个合同锚当全部发布动作。6992的P4“未开始”整行同时位于当前表907行和历史表922行，不能全局替换；用900行“当前进度表”及906行P3当前行构成唯一上下文，只改当前P4行并补6992 own CI接受和最终P4设计审查状态。915行起的历史快照及P3原“本记录写入时”pending原文保持；§13与主持者决定仅写真实已处置内容，本次正式设计自身commit/CI尚未知，不预写成功。

决定76初始42处合同修订以实际986195a308629131e99fa5ae63e2f8c2ab474c75文档Git对象准备，冻结原件保留。本次正式绑定采用已由[ROOT收口](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-s1-closure-01.json)接受的main ca4688a1e9255d68b9ce54ff1d9c0acb824b3068 / tree 1977eec1626950d51c11d3e6f2fc80ad6be7974c，唯一parent e07cc1964238b3d248c791c124d9a9fc48dd9253；精确record CI41573821884/a1及预登记e07/a1→ca/a1两main配对见[指定配对接受](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-main-pair-01.json)。八份完整preimages从该Git对象重采（七份合同文档及S1自然转录审查记录），原42处old仍各唯一；旧6992的15锚数据与审查原件保留当时范围，不改写为本次输入。应用时仍须全部整文件与唯一锚预检先于写入，一处不符即不写。本次S2正式设计自身minimum10、commit、postgen及完整CI尚待；S2实施未放行，SDK/producer/控制仍未运行。

### 2.2 需要负责人确认

没有需重新请负责人决定的产品事项；主持者已按既有授权采用§2.3实施决定，负责人可以改判。REST父页筛选本期交付，网页父页选择器按00允许留M12打磨；P5 prompt、M11升级和新增公开模式／配置不在本Phase。若正式验证不能满足原预算，提交具体冲突和原始证据修订设计，不自降验收线。

### 2.3 主持者的决定（负责人可以改判）

以下为主持者在三位fresh初审及原席复核后接受的实施决定，接实际序列70使用71／72；负责人可以改判。设计层已核实finding零open，真实实现、成本与三Step门禁仍待执行。

| 编号 | 已接受的实施决定 | 依据、代价与实施验证义务 |
|---|---|---|
| 71 | 主持者的决定（负责人可以改判），已接受设计：改删W以首次Before＋旧current head证明D_W/F_W，未改页依index串行后的最终tree/解析近似；来源page目标Now/Before均要求相应revision与Extractor2头。显式允许必要distinct(S,A)旧F ID关系及资格/不确定性，修订初稿不保W×关系的承诺；不保旧正文/JSON/peers/raw边。完整Unknown上界含known及继承闭包，SQL错回滚。 | 一条最终SQL仅传去重F parent IDs，完整D_U后app投影；已属D_W可折叠、最终资格不可能且旧贡献已全处理可释放。400×300与400×10000/100ops/健康100k均为待执行成本义务；120000改写链接不是旧F总界，512只是读批。原预算、无候选零全扫、非空T、五交错不放宽。原B/C已完成设计复核；设计接受不等于性能或实施接受。 |
| 72 | REST使用有界exact types catalogue：全本可列wiki页排来源后统计，独立type／tag／parent筛选；最大100项、types数组实际JSON≤32KiB。仅完整存储值≤1024UTF-8字节可作为filter；新增required types_complete与unfilterable_type_pages。Catalogue仅REST=true；MCP保留既有精确type SQL，不承担catalogue扫描。 | 00未定exact-key碰撞、超长值、计数范围与输出界，见[types方案](specs/P4-virtual-ingest-types-boundary.md)。新增字段／提示与全量有限输入读取是代价；不改现写入type长度或MCP filter。D01–D11和100k／TOAST／heap／HTTP字节及手写oracle闭合；菜单外URL值保持精确筛选。 |
| 76 | 主持者的决定（负责人可以改判）：MCP log/detail在seq前3/20完整path/null逻辑预览上仅因48KiB保留前缀，required ingest_sources_cut；REST3/20与log.md5不变，来源无游标。 | detail空changes+真实Start/next先Fit来源；log先完整rows、首行退让才固定row-next择来源，再Fit changes；双cut独立。s*/相邻候选独立SDK编码与真实HTTP raw-token双oracle、两adapter过裁控制、旧完整期望同S2-1。三席静态处置与原C核修见设计审查补充；运行验证和S2入口另验。 |

虚拟分支单次RR／export复用、既有RevertTrees保留、00053迁移编号和文件／Step编排为已接受设计方案；实施中与既有契约不一致的新发现仍须交主持者处置，不由本表豁免实际验证。

S1补充决定73以字面&→%26保Markdown实体目的地身份；74是Markdown成本原top按两个完整leaf分进程的单recipe例外；75明确256/500按最终转义显示UTF-8字节计并含省略号，不拆码点/转义对。原算法/样本/整文件与进程预算不变。ROOT已正式采用73、74、75，均为“主持者的决定（负责人可以改判）”；原预算与接口边界保留，技术修复和C-Q1补测不另增产品决定编号 编号及具体边界见00§11，实际依据和失败保留在S1记录；技术lint修复不编产品决定。

## 3. 服务端

### 3.1 文件表与所有权

表中路径均相对仓库根；已存在文件以固定 6992 为准，标“新”的是候选落点。测试与生成物在相应功能提交同入。

| Step | 目录／文件 | 精确职责 |
|---|---|---|
| S1 | `server/internal/modules/page/domain/{path.go,list.go,batch_plan.go,batch_tree.go,errors.go}`；新 `virtual.go` | Lookup 包装／VirtualFileError、V 校验、batch 模拟树与目标保留键、Go fixture |
| S1 | `server/internal/modules/page/app/{node_paths.go,list_by_pattern.go,read_path.go,open_notebook.go,ports.go}`；新 `virtual.go` | 模式读接口与所有构造调用；只在 virtual read 分支开快照；prefix/glob/Fit；真实条目字段不变 |
| S1 | `server/internal/modules/page/{node_paths.go,module.go,tools.go}`；`adapter/mcp/{tools.go,list.go,read.go,open_notebook.go}` | 模块根 DTO／窄端口与 Tools nil 校验；虚拟 wire metadata；PathErr 映射 |
| S1 | `server/internal/modules/linking/app/{list_index.go,ports.go}`，`domain/index.go`，`adapter/postgres/{index.go,queries/index.sql}` | 共用目录用例、parent id/root、type 真正非空、REST有界catalogue及完整性字段（MCP仍精确type SQL）、来源排除；文件读复用此读基础 |
| S1 | `server/internal/modules/linking/module.go`；新 `index_files.go`，`adapter/http/index.go` 与现 `handler.go` | 目录 REST 路由与模块根文件数据端口；严格禁止跨模块 app/adapter 引用 |
| S1 | `server/internal/modules/wiki/{module.go,virtual_files.go}`，新 `app/{virtual_files.go,export.go}`、`domain/virtual_files.go`，现 `app/ports.go` | 只读数据服务、纯生成器、profile；不拿 lint 的 Runs／Stop |
| S1 | `server/internal/bootstrap/{wire.go,deps.go,mcp.go,linking.go}`，新 `wiki_virtual.go`；现 transfer `app/{export.go,contributors.go,import.go}` 按需测试／窄修 | 在 transfer.New 之前构造只读 wiki 服务；逐字段适配并交 contributor，保留既有 export snapshot |
| S2 | `server/internal/modules/page/app/{unit.go,extension.go,unit_content.go,unit_create.go,unit_delete.go,unit_move.go,unit_rename.go,unit_relocate.go,unit_revert.go,ports.go}`；新 `ingest_before.go` | applyStep 前首次 memo，全部真实创建／修改／删除／participant 路径；Event.BeforeIngest 与旧 revision |
| S2 | page 新根 `ingest_marks.go,ingest_metadata.go`，app `ingest_marks.go,ingest_metadata.go`（page只拥有头DTO／读取，coverage编排在linking），postgres `ingest.go,queries/ingest.sql` | page 拥有 raw live／revision／树与写 marks；<=512 头流、显式 EOF；不读 linking 表 |
| S2 | `server/internal/modules/linking/{module.go,admin.go}`；`app/{index.go,ports.go}`，新 `app/{qualified_before.go,ingest.go}`、postgres `ingest.go,queries/ingest.sql` | 索引锁内删前 Capture、index 头校验、旧 union/Unknown、候选 SQL，联接由 bootstrap，store 只读本模块表 |
| S2 | page `domain/changeset.go`，`app/{list_changesets.go,get_changeset.go,history.go,ports.go}`，postgres `history.go,purge.go,queries/{history.sql,purge.sql}`，http `history.go`，mcp `history.go,changeset.go,log.go,tools.go` | S2-1字段、专用筛选、3/20/5来源、MCP两阶段Fit/双cut与工具说明、bounded leaf purge/父防线；tools.go保留S1归属 |
| S2 | 现 `server/internal/bootstrap/mcp_history_test.go`、page `adapter/mcp/history_test.go` | S2-1旧asMCP和完整JSON golden同提交更新required false与非零路径；独立候选/真实SDK双oracle和逐腿控制，Brief/VersionChangeset不添键 |
| S2 | `server/migrations/sql/00053_page_ingest.sql`，`server/sqlc.yaml`，`deploy/runtime-grants.sql`，`server/migrations/schema_test.go` | S2-1同提交迁移／schema／grants／sqlc；采用前确认00053未被其他main迁移占用 |
| S2 | bootstrap `registrants.go,reindex.go,interleavings_page_test.go` 与现 checkPages/pageViolations 落点 | Event Kind/Notebook/ChangesetID/Before 逐字段、reindex 禁回填；三个不变量及两事务 controls |
| S1/S2 | `api/modules/{linking,page}.yaml`，`api/mcp/tools.json`，对应 `api/dist`、server HTTP/Postgres gen、web 生成 schema/client，`web/apps/web/src/test/fakes.ts` | 先契约后生成；每个必填字段／码的中英文本／fakes 同提交可绿 |
| S3 | `web/apps/web/src/app/routes.tsx`、`pages/notebook/{page-tree.tsx,notebook-places.tsx,history-page.tsx,history-filters.tsx,history-row.tsx,changeset-page.tsx}`，新 `index-page.tsx`；`pages/page/unresolved-link.tsx` | 文件名已按固定6992树核对；目录／历史／只读虚拟入口；不复制 page tree store |
| S3 | `web/apps/web/src/services/{linking.service.ts,page.service.ts}`，locale、共享虚拟 fixture、对应 tests；新 `e2e/stories/wiki/{lw9-ingest.spec.ts,lw10-virtual.spec.ts,lw13-index.spec.ts}` 及现 `e2e/stories/wiki/lw12-downgrade.spec.ts` | URL/SWR key、非事件缓存、两语言、readers、已有 LW 故事内增量 |

精确文件／Step见[文件表](specs/P4-virtual-ingest-files.json)：当前190落点（137既存含11生成物、53计划），初稿176项全保留；此前修订2为187（新增11项，含as2），该历史数不改。决定76另补3个原已存在落点：mcp/log.go及bootstrap/mcp_history_test.go、page/adapter/mcp/history_test.go；tools.go保留S1并补S2/S2-1，不新增文件计数。23构造文件／30声明调用、六WriterDeps／TrashReads、两ToolDeps站点分列；15修复依赖保留。S1-3同步24个相关站点及其下游使用；S2增加IngestNow时保留RevertTrees及gen/nodes.sql.go的ListRevertTreeRow，审计不要求无谓改动。

### 3.2 虚拟路径与游标

`domain.VirtualLookup(state, next) Lookup` 包住 FindPath/WalkPath 共用 lookup，只在 LLM、chain 为空和根标题键 index/log/index.md/log.md 拒绝。统一 `VirtualFileError{Name string}` 的 `Unwrap() error { return ErrVirtualFile }`，不把现有哨兵 ErrVirtualFile 写成结构体。page app NodePaths 新收只含 StateOf 的 NotebookStates，Find/Walk 在调用方授权之后读模式一次，纯 lookup 闭包只用该状态；所有构造点与假实现同提交。batch 使用计划里的 NotebookState 包同一个函数。list 自己的 tree.children 路径也必须接；move 最终目标未 lookup，仍由现 CheckContract 守根保留名。初始无锁读与 unit 内重检保持原有分工。

| 形状 | LLM 行为 | standard／嵌套对照 |
|---|---|---|
| canonical 根 `read index.md`／`log.md` | 生成内容，virtual=true，无 version | 原真实路径读 |
| `read index`／`index.md.md`、`list index/` | page.virtual_file，不生成也不建真实节点 | 原真实路径行为 |
| write/edit/delete/versions/backlinks、log(path)、index(parent)、move 来源／父链 | 授权与原字段顺序后经统一 lookup 答 virtual_file | 真页原规则 |
| move 到根保留标题、batch 每种相同路径 | CheckContract 与计划返回同码及准确 failed op | 嵌套不受根保留键影响 |
| grep/search/index 实体条目 | 不枚举两个虚拟文件 | 真实实体照旧 |

`ChildrenCursor` 保留 `O float64 json:"o"`／`I uuid.UUID json:"i"`，增加 `V int json:"v,omitempty"`。外层仍是 shared.EncodeCursor 的 version=1 envelope；payload V 不是 envelope version。V=0 不编码，原 O/I 游标字节不变。V=1 表示 index 之后，V=2 表示 log 之后；仅这两非零值合法且 O=0/I=zero。V<0、V>2、非零 O/I、noncanonical/extra/duplicate JSON 一律现 bad_request。

顺序是 index→log→全部真实 `(sort_order,id)`，不占 -1/-2 哨兵：真实 -3/-2/-1/0/正值全部在虚拟项后。V2 从真实 first 读；非根／standard 收合法 V 也从其真实 first 读。每项 cursor、Next、Fit 共用此语义，limit=1/2 可恰切两虚拟项之间。glob 按现 K 路径键序插入两项，不用 children prefix 次序；`*.md`／`**/*.md`、literal 与分页恢复都不重不漏。open_notebook 同样 prefix/Fit/MoreRoot，真实页面／附件 counts 不加二。list 的虚拟项无 bytes/version/updated；真实零字节页原 bytes=0 仍出现。

### 3.3 ReadPath 的一致性与快照所有权

正常真实页／附件 ReadPath 保持原 unlocked/no transaction。新 Snapshots 依赖来自已存在的 page.Deps.Snapshots；`NewReadPath` 收它，`WithVirtual` 返回不可变副本，`Module.Tools` 才强制 ToolDeps.Virtual 非空。[F01–F04]

Execute 顺序仍 actor→ParsePath→CheckWindow→decideNotebook。只对规范虚拟 read 且无锁 mode 提示为 LLM 的分支调用 **一次** WithinSnapshot；提示不是授权／模式的最终证据。snapshot 回调内重做 notebook 可见性／page.read 和 StateOf，随后所有树／类型／来源／history／path 读与纯生成及 Cut 用同一 ctx。lost visibility 回原 404。若快照内已降级，返回私有 handled=false，结束快照后重新授权并走正常真实路径（真实同名节点或原 404），不在已经结束的 ctx 中读。

在 snapshot 第一读之后发生降级／改名，本次只承诺该 RR snapshot，下一次 read 重生成；跨调用 offset 不稳定。snapshot 不能加入 Writer 的 ReadCommitted，不吞 ErrNestedSnapshot。alias 虚拟拒绝不走内容生成。虚拟 PageRead 无 Version，仍以 domain.Cut 处理 offset 从 1、column 从 0 的字节边界、limit、长行、UTF-8 与 next_offset/next_column，经过真实 SDK 度量保持全答复≤48KiB。

有限备选：把所有 ReadPath 改为 snapshot 会扩大旧读语义与连接持有，否决；让 wiki 自开 snapshot 会在 export 内嵌套，否决。推荐只在规范 virtual 分支拥有 RR，export 复用原 RR，内部 GenerateInSnapshot 明确不授权也不开事务。实测用锁住 export/read 后段需要的独立表配合另一事务提交，让改名前后路径混合才会失败；不能用不影响后段的 mode-only断言冒充 RR 变异 catcher。

### 3.4 目录与文件生成

REST 与 MCP 复用同一 ListIndex、IndexCursor{K,Typed,ID} 与 type-key/id 顺序。IndexQuery 保留 MCP Parent 路径，新加 ParentID、AtRoot、ExcludeArea；Parent 与 ParentID/AtRoot 同给的内部调用为字段 422，公共 REST 只暴露 parent_id/at_root，MCP 仍只 Parent。REST ExcludeArea=true且Catalogue=true：完整首遍头排来源，分批取完整filter值，统计catalogue后筛items；MCP两者false，保留既有IndexPages(typeIs)精确SQL，不承担catalogue扫描。二者共用资格、排序与分页规则。ParentID 需存在、同本、live page；附件／外本／不存在统一 page.not_found；AtRoot=false 等同未给，parent_id + at_root=true 422。

REST按已接受的决定72，以当次首遍完整有限可列wiki页集合排来源严格后代后统计exact types/count，独立于当前页及type/tag/parent筛选；来源根保留，source_area={root_id,sources}或null，另返回当次mode。普通本source_area=null。valid frontmatter的首ASCII-insensitive type必须为string且jsTrim非空；超过257字符且前缀全空才读全值，长空白后x不能漏，首非string不能改取后键。文件次序是typed type-key/path与untyped parent-path/path，REST/MCP仍type-key/id；这一区别是表示次序。S1-2真实PG/HTTP与S1-4成本见§10.1，S3目录UI尚未实现。

文件数据端口见 ports：一次完整紧凑类型头／树／source membership，选定项目后取 facts，不循环公开 limit≤100 的 REST 页反复读全本。index read 初值 MaxIndexFileItems=200，≤40KiB，每项 inbound≤1000；最终取值须正式实测后冻结。log read recent≤50，每 cs 前3变化，message≤500B；S2 每 cs 前5来源。index export≤10000项/2MiB，**不得读 inbound**；log export≤1000cs/1MiB。全部界是代码常量，不加 YAML 配置。

生成器纯函数、固定英文。显示title/type/summary/tag/message/client/name及显示path的控制字符／换行中和为空格，Markdown label转义反斜杠、方括号和反引号。显示上限按最终转义字符串的UTF-8字节计，256B的label与500B的message上限均包含截短时的三字节省略号；恰好到界不省略，截短不拆UTF-8码点或反斜杠转义对。**目的地身份独立**：read用完整原路径，export用完整映射路径；先把字面`%`编码为`%25`，字面`&`编码为`%26`，再处理`<`／`>`为`%3C`／`%3E`并包进`<...>`；字面HTML实体拼写须经过真实Markdown/HTML/URL消费链仍保持原节点身份。destination不做256B显示截断、trim或空格中和；异常控制字符／非法UTF8等非法路径返回无名称内部错误，不造一个指向别处的清理后路径。显示缩短只改label，不能改node身份。每条完整链接按编码／转义后实际UTF8字节试写，放不下则不写该条、保留footer，不能截断目的地／百分号编码或把半条算入M。固定头／来源行与footer也计入界，强制段装不下则内部预算错误。source area只给路径、非空页＋附件数及pending_sources/lint去处，无lint调用或伪pending数。%41/%20独立身份与编码后容量控制见F07/F08。

新增核实的边界：现type filter上限1024 **UTF-8字节**且与索引完整存储值exact相等（沿既有NUL→FFFD归一，不是原Markdown拼写）；type-key/显示前缀会碰撞，不能当filter值。`specs/P4-virtual-ingest-types-boundary.md`规定有限exact catalogue（≤100项、types JSON≤32KiB）、保留项完整计数、必填types_complete与unfilterable_type_pages明确未完整／long页数；长值页面仍可见但不发无效filter。这些新REST字段和00 A11已随决定72接受设计；S1-2服务端行为及S1-4的100k/长值/TOAST校准、fixed targeted guard已有独立收据；0b0 fresh完整6targets/50recipe双轮100过程、1333 recipe-local identities及6case/24固定guards已ROOT接受；各pair原容量均满足，最大占半界65.443%；原首轮100容量失败保留，C-Q1独立补测另账；实际容量与精确CI分账见[S1记录](reviews/P4-S1-virtual-files-review.md)，不以targeted替代全进程。

REST空结果仍交完整响应metadata：合法空父children、TagKey=false、TagPages空、精确type零命中或cursor越过尾部只确定items=[]／next为空，不能跳过mode、source_area、types与完整性字段。当前ListIndex:93–104的非错误空返回在Catalogue=true时改为记录itemsKnownEmpty并继续metadata／catalogue；不再需要的item筛选读取可省，但已遇到的真实字段／权限／parent／读取错误照原优先次序返回，metadata读取失败也返回错误。MCP Catalogue=false保留既有廉价空返回。D04用另一未筛选eligible typed页做非空oracle，逐腿回退每个REST捷径必须失败。

### 3.5 整本导出与 import 边界

组合根在 transfer.New 前构造只有 pool/窄读端口的 wiki VirtualFiles 服务，将同一服务分别逐字段适配给 page.ToolDeps.Virtual 和 transfer.ExportContributor。lint 模块的 Runs/owner Stop 生命周期原样；不因依赖顺序提前启动 worker。wiki app 不 import transfer/page/linking app，公开根 DTO 在 bootstrap 转换。

仅 RootID=nil 且 **现 export snapshot 内** 的 mode=llm_wiki 才 sink.Add index.md/log.md 两文件；没有 nested snapshot、Lint/Runs 或第三文件，subtree 和 standard均无贡献。snapshot 内 source/root/path/history 同一视图；既有 export 授权生命周期不借此重写。index 不用 inbound，两个有界 byte slices 不引入新流式框架。非法旧根 index.md 文件夹与贡献冲突返回 contributor_conflict，不能覆盖或悄悄跳过。

导出路径以transfer已完成的plan为权威。bootstrap把`Scope.Nodes`逐字段复制为wiki自有`ExportPath{ID,Asset,Path}`，由export-only scope交GenerateInSnapshot；不重算plan、不加快照。映射只在当前contributor调用／export snapshot中只读，完成后释放，不跨调用缓存；read profile仍用page域原路径。index条目、来源区行、log变化和S2 log ingest来源按node ID查映射；导出显示path、untyped父目录组键及组内次序用映射路径，index标题仍为原可读标题。folder-only尾斜杠路径作为非链接文字，不伪造`.md`；文件／附件才链接。live tree／来源根／index项缺映射或kind不符为内部错误，历史缺ID保留null／非链接文字。历史Path=nil但ID仍在映射时按ID生成链接；禁止用旧path猜新落点。映射拒绝重复ID（即使值相同）、重复完整Path、空／绝对／dot或dotdot段、反斜杠、非法UTF8／控制字符、asset尾斜杠。校验在生成两文件前完成，错误使导出失败且不含名称，不静默丢live条目。E07/E08用真实zip entries核folder-only及碰撞改名子树，E09与U19核全部path消费者；不只比两份同算法输出。

serve的真实ZIP已由S1-4测试读取两虚拟文件及contributed；metaPaths只是读取该元数据的测试helper。生产链由ReadMeta构造Contributed，再由真实导入按第一个meta中的精确NFC路径跳过；E05的单fault是ReadMeta→Contributed映射，不把破坏helper算生产控制。无meta或首meta超过16MiB时沿既有fallback：LLM根保留名加序号，standard按实体导入。新LLM重新导入仍可产生raw 2/AGENTS 2；存量升级与旧保留根处理留M11。实际whole/subtree/standard、NFC、冲突和fallback证据见[S1记录](reviews/P4-S1-virtual-files-review.md)。行为、控制和完整门禁结果按实际身份记在该记录。

### 3.6 ingest 的时点、锁序与目标 current 证明

决定71保留首次Before、index串行、完整coverage和原预算，并**明确改变初稿一概不保留W×关系的存储承诺**：只为旧F证明保留必要distinct(来源页S,附件A) ID对及每S资格／不确定性，不保旧正文／JSON／slot peers／raw边。主持者已授权这项设计修订，原B/C第1轮已确认相关设计语义；实际成本验收仍待。

| 时点 | 可证明的事实 | 不能替代 |
|---|---|---|
| Ttree：notebook锁、Write.Notebook | 外部稳定tree/mode/source-root；自身树改由首次memo逆推 | 不是所有content同时快照 |
| Tpage：每页内容首写前持自身content lock | 该页旧revision/bytes；删除子树在删前≤512头读齐 | 不是最终revision-1或late SELECT |
| Tindex-before：index锁内、任何Replace/Delete之前 | W全部旧D/F表示、旧target head，与可信BeforeRevision绑定 | 不把新写target head回填Before |
| Tindex-after：同锁、正常Replace/Delete/resolve后 | 未改页最终tree/解析与已提交content/head；Now的当前目标资格 | 不覆写W初始D/F；不是BEGIN快照 |

锁顺序仍workspace→notebook→自身正文／changeset→适用source-hash→index；index下不得再锁别人的content。B持Q未提交content等index时，A plain SELECT读旧committed content与旧head；B先commit则后来的A见新对。禁止为此在index前开RR，也不新增全来源锁。普通wiki SHARE写与source0→非空的SHARE首写仍可交错。

page在每次applyStep守卫后、真正写前保首次node parent/name/kind/live memo；新建Exists=false，删除子树记齐，participant/system rewrite也走同一入口；内容首锁保存Revision/ByteSize，后次修改不覆盖。BeforeIngest逆memo重建unit初始祖先；缺必要祖先／环为内部错误。StepContent增BeforeRevision；IngestNodeBefore增RevisionKnown和PageBytesKnown：W旧页与Tree锁下可信目标可证；SHARE下未持锁目标的late revision/bytes都不可冒称起点事实。

来源page目标的Now资格是live同本、严格来源、非root、非空，**并且**IndexedHeads显式Present、同本、Extractor=2、indexed revision等于该次page metadata revision；≤512点读即可，没实际直接候选不扫全本。asset目标只核live同本严格来源及asset kind，无page head要求。literal-v1／missing／revision mismatch目标不进新候选；不把现在不合格目标记为新消化。P3 current只核Extractor，本P4的revision配对是新增证明，不能声称P3已有。

Before中来源page目标需可信初始revision/bytes、旧Extractor2 head匹配该BeforeRevision；全部目标head须在第一次Replace/Delete前读／缓存，W目标用其旧head，不用之后新head。target-only RevisionKnown=false或bytes不能证时，若旧发出表示和完整目标ID关系已知可给完整可能上界，绝不由late新head升级Known。missing/stale旧head或缺旧表示保守Unknown，不作KnownAbsent；真正SQL／Scan／ctx／DTO错误回滚。F来源页自身也要求初始非空和相同旧current证明。

现行RevertTrees/六字段RevertTreeNode仍独立：ID/ParentID/Kind/Name/存储NameKey/原SortOrder，nil未读与非nil空树区别保留。它无revision/bytes/deleted/audit，不能替代Before、Now或coverage；公共ListNodes、四字段VirtualTreeReads及gen/nodes.sql.go的ListRevertTreeRow保持原边界。

### 3.7 删前 D/F Capture、四态与紧凑关系

D是合格wiki直接引用（真实正文或首sources唯一slot），F(A)是合格非空current来源页以真实正文asset边引用A的集合；不是树parent_id，也不含frontmatter边。旧已消化集合是`D ∪ {A | F(A)∩D≠∅}`。只为本次新直接引用candidate写mark，不自动给随来源页消化的附件写mark；已有跟随消化仍须排除。

W为本unit写正文／删除页。任何Replace/Delete之前Capture全部W，D_W与F_W按初始树／首次Before和旧resolved事实分别判，不重解析旧正文。KnownAbsent只来自unit新建；KnownIneligible仅排除**这一角色**；source对wiki D是Ineligible不能抹掉其F。Known须current／资格／完整流齐备，可贡献空集；Unknown零值缺证不得作空。

先完成D_W的完整known union及uncertainty，再读W的source-role Bodies；两者复用现LintReads批流：SourceValues／SourceSlots用于wiki首属性，Bodies同时覆盖wiki正文和来源F，仍≤512行、普通8MiB／合法单大JSON原界。旧F只接property_key IS NULL、resolved_asset=true且真实同本来源asset；四种正文link kind都支持。每页处理完释放原引用／JSON/peers，保留D ID union与必要F ID对。Bodies输入范围扩大是新增实际读取工作，虽未新增命名SQL，不能隐藏成本。

对每个来源S分别保存F状态：Known给完整distinct资产IDs，未知bytes但完整可信旧表示给PossibleAssetTargets（含所有确定边），missing/stale／不完整表示给AllAssets。D未知有限关系叫PossibleDirectTargets，必须含该页已知与不确定直接目标；缺完整表示则AllDirect。**最终PossibleTargets上界还必须闭合继承关系**：D_possible ∪ F_possible(D_possible)，且含全部已知贡献；只存直接S的U不能声称已排除A。内部可用这些完整关系的符号组合表达上界，最终SQL对完整candidate集逐个判成员；不能截某个关系或把Unknown重命名Known。

保留与释放规则（任何优化都保完整上界）：S已在known D_W且F Known时折到KnownFollowedAssets，S仅可能在D_W或F资格仅可能时折到PossibleFollowedAssets；折叠后pair释放。F AllAssets且S已知／可能属D_W则保InheritedAllAssets，页面候选T不受asset-only不确定性阻挡。未由D_W解决的S，只在仍可能与D_U相交时保distinct(S,A)或UnknownFParents的S ID。最终metadata证明S已删／出来源／空字节且不能成为D_U target，且D_W的known/possible贡献已完整折叠时，可释放剩余pair；missing head／未知资格不能据此当作不相交。system Rewrite可让非空来源S在W中仍live，不能借普通不可变性删掉这类pair。之后与最终D_U相交，投影附件union后释放所有剩余F；不用常驻raw边矩阵。

| wiki Q / source S | D、F依据及必须结果 |
|---|---|
| Q∉W、S∉W | D_U与F_U取Tindex-after最终tree/解析，Q→S且S正文→同级A，则P新直接A不标 |
| Q∈W、S∉W | Q初始D_W含S；S的可信F_U扩展到A，改／删Q后P→A仍不标 |
| Q∈W、S∈W | Q旧D_W与S旧F_W都在删前保存；同批删Q、删S但保同级A后P→A仍不标；绝不能借未改页近似豁免 |
| Q∉W、S∈W | S旧F_W保留到最终D_U可知；Q依既定最终tree／解析资格判。S仍live/current且Q最后指S，旧F的A不标；最终Q不再指S或S不再是可消化当前目标时这一路不贡献。该近似不把Q强化为Before，也不改Q/S皆W的一行 |

独立未消化T（source page或独立asset）必须真mark。若S未被任何D引用，保留其所有F资产然后无条件排除也错误；U16/U17为非空反例。若旧source F缺证，只抑制其可能继承消化的范围：完整F可能集不含T时T仍标，F全未知时只对可能被该parent消化的asset类保守，不把page T全部压成AllDirect。SQL/Scan/rows.Err/ctx错误始终原错回滚。

实际保留pair数R≤Σ_{S∈W_source} distinct_body_assets(S)≤|W_source|×MaxLinks(10000)，512只是批界。优先每来源页局部去重后追加排序ID对，不留双份raw边；实现应量map桶／ID切片／排序重叠及旧D、possible、候选、coverage同时存活的peak。表示采用每S一个排序去重asset-ID切片，逻辑payload约16R字节加每S的ID／slice／状态；切片capacity、每页去重与sort临时区、Go分配器／GC均另计，不能称16R就是heap。最终SQL只传去重parent IDs和其他既有全集，不传R个pair或两uuid数组；接完整D_U命中后app线性投影。SQL的候选F_U join／hash／CTE和parent参数编码仍单列测量。不能把10000 marks cap当证据截断界；不能设使健康100k恒Unknown的低总cap，预算不足须据真实证据修订算法而非默默放宽。

### 3.8 coverage、快速返回与新增成本义务

index流程先Capture W旧D/F，再原Replace/Delete/resolve；新written wiki从SourceValues／SourceSlots／Bodies产生直接resolved目标，经Now＋IndexedHeads≤512点读资格。没有新直接candidate则释放Capture并返回；普通非no-op无候选保存仍零全本Heads／未改heads／NovelSources／SourceArea扫描。Known/possible D_W可以先排除直接已消化候选；asset还须完整F闭包，不能仅做direct减法。

有剩余candidate才扫全部raw nodes（含trash）First无after／After严格id>after，raw LIMIT≤512先于资格筛选，zero UUID合法。LEFT JOIN暴露live page缺content，rawLast来自原始尾行，Done仅短raw批或明确EOF，满批追加一次空读。祖先live同本page、无环、page10/asset11、来源根合法仍需完整核对。外部tree/mode变化被book锁挡住，自身树写由memo/W区分，不把purger当新live成员来源。

coverage分别产出完整untouched wiki emitter IDs、所有当前合格非空source page IDs（含仍live的W供D_U目标资格）及untouched source子集。每页current证明用metadata revision＋present同本Extractor2 head；wiki缺head/stale/mismatch令D覆盖Unknown，不能以candidate reverse hits代替。来源F缺head/stale/mismatch单独记UnknownFParents，不能作空F，也不必让与它无关的page T失败。W的旧角色由Capture负责；untouched来源F按最终树／当前head，同已声明近似。Complete只在raw EOF与全部适用角色检查结束产生私有proof，绑定unit identity、notebook、root、W和Extractor，不收跨事务bool。

保留原健康100k真mark、最大合法batch、普通无候选零全扫、历史200ms／请求和CI原预算。old-F总界是R≤Σ min(该页索引正文asset边数,10000)，还受每页5MiB与该次合法W规模限制。MaxBatchRewriteLinks=120000**只数被rename/move达到、需改写的链接**，不是这些页全部旧F；MaxUnitLinks=20000约束该batch显式内容操作与计划解析，也不能冒充system rewrite旧F总界。400个index rewrite页的这一形状可达上界为4,000,000对；单move/rename的rewrite本身不受400页batch上限，删除W也不受400页上限，所以全体合法请求只能按实际|W_source|×10000及适用节点/字节约束报告，不能宣称全局4M封顶。

| retention成本计划（均未执行） | 静态资格与必须测的非空结果 |
|---|---|
| 400×300=120000对 | 400个current非空source S，每页300条distinct资产body链接；400个未改wiki Q各指S，D_W空。只rename其中一个共享asset，实际被改写链接400，不是120000。A已有跟随消化不标，独立current page T必须1mark。 |
| 400×10000=4,000,000对 | 10000个共享asset用四字符完整名`aa.b`（三个字符位置取小写a–z，26³=17576种，按字典序取10000个不同名，点非首/末、与400个S同处raw目录）；每个S正文先写唯一5B普通文本前缀`S000\n`至`S399\n`（此处\n表示单字节LF），避免source_duplicate，再按同顺序连续拼10000个`[](aa.b)`、链接间无空格换行，80,005B，未写frontmatter，要求真实夹具断言Extractor2索引10000条。其中一个asset改成另一个未占用同样四字符名，改写链接仍400、前后正文80,005B。400页合计32,002,000B≤34,406,400B rewrite字节界；每页≤5MiB。新P不受rename影响，仍A不标/T真标。按真实解析/资格/MaxLinks读取确认，不能仅按字节算式宣称夹具成功。 |
| 上述形状＋100操作＋健康100k | 1个同长度asset rename＋99个独立wiki非no-op小写操作，P占其中一个；这些写的路径/正文均不含被rename资产，故WrittenRewriteWeight不增加400 rewrite计数。显式99页总≤8MiB、链接≤20000、batch字符串≤6MiB、ops=100，少量新/改节点≤10000。首条/中间/末条链接以真实parser验证，所有名称TitleKey唯一且Node.Linkable，实际每页10000条resolved asset/body边；真正Plan/guard仍必须接受并保留实际计数；不能用100个rewrite操作代替。扩至总100000健康节点，真实旧F保留峰值和非空mark都验。 |
| 一页接近5MiB、删除Q/S、更多W | 单大页、按实际字节界混合页、Q/S皆W删除可立即折叠、无D父独立asset正控分别测；单move/rename超400来源页不冒称batch非法，以现有request和parser界验证/记录实际可达规模，不提高任何限制。 |

4M切片逻辑payload约64,000,000B，另有D/possible/candidate/coverage、每页临时区、rewrite解析事实和GC同时存活；不是成本通过。去重采用单页≤10000 IDs排序compact后转移所有权，不复制全关系；可复用同值asset切片仅作为后续有实证的优化，不依赖400页共享assets将夹具伪降为300/10000对。fixture准备在deadline外，计数进入实际请求内：Go TotalAlloc／peak及各阶段同时存活、PG参数／rows/buffers/TOAST、custom/generic、index锁时长与整请求。静态合法数值检查不保证请求预算；如果真实parser/guard/成本不能满足，保留原输入/拒绝/首败并交主持者修订算法，不能降低形状、低cap转Unknown、吞错或放宽预算后称闭合。

### 3.9 完整候选 SQL、同事务标记与错误

SQL附件仍17个命名语句，**NovelSources正文已实质修订**；不是17条未变。新增旧F读取复用已存在LintBodyReferencesFirst/After（通过Bodies），无每S额外查询；旧D用既有SourceValues/Slots/Bodies。头点读从wiki扩到Before/Now source targets和F角色。未执行SQL／sqlc／EXPLAIN。

NovelSources收到完整candidate（含page revision）、可信current source relation、untouched wiki/source IDs、D_W known/possible的完整direct IDs，以及剩余old-F和UnknownF的**去重parent IDs**。R个(S,A)关系与折叠资产union保留在app，不复制为PG参数。SQL先取candidate asset的F_U父（untouched current正文），wanted为candidate加F_U父、old-F父和unknown F父；反向边只找相关wiki页的superset。每相关页首sources JSON一次，完整同path peers先数exactly-one（包括重复target、未resolved、其他target），再筛wanted；property_key NULL才正文。D_U的page目标还须在当前source证明集合，asset候选已有Now证明。D_W保持Before资格，不以删除后的S或新head重判旧D。

一条SQL返回每candidate的直接D_U命中、F_U是否与D_W known/possible或D_U相交，以及每请求parent的D_U命中，外加重复invalid_source_value／invalid_proof。adapter要求candidate和parent各完整恰一行（同ID可不同row_kind），扫完及rows.Err、所有flag后才把结果交app。source candidate的Now revision／Extractor2在SQL defensively核，违反已交证明是内部错误而非改判novel。parent没当前资格时其D_U=false；这不抹掉app的Before D_W证据。

app据D_W known/possible＋完整parent D_U命中投影剩余old-F关系，并将匹配UnknownFParent升级InheritedAllAssets，随后释放关系。最终直接candidate要同时不在known/possible direct、折叠known/possible资产union、F_U继承结果及old-F投影，才能novel；AllDirect阻所有无法证实目标，InheritedAllAssets只阻asset。完整Unknown符号闭包包含所有known贡献，不因SQL只传parent而漏asset。没有parent被D引用时不能把所有F资产都压掉，独立T控制必须真mark。即使novel空，也先消费全部SQL错误；不为每S另开查询。

AddIngestMarks仍单条：绑定owned live edit cs/notebook，FOR NO KEY UPDATE仅自身cs；完整novel去重、排已有、UUID排序取10000-count剩余量，seq接旧count，count按实际INSERT RETURNING更新。零owned／坏seq/count／SQL错误回滚，marks只追加，不能用cap截旧证据。仍同unit/index锁；两个unit争同S仅先提交者标，失败者内容/index/cs/mark全回滚。

C10把真实statement_timeout和ctx取消分成独立腿：57014／原ctx cause在adapter/coverage边界可识别，出错后marks/publish/commit尝试为0；真实rollback和清理后池可执行新查询仍验，不要求复用原backend PID。PG自动abort后COMMIT→ErrTxCommitRollback不能替代及时传播。保留现TxManager WithoutCancel与rollback失败优先基础设施错误的行为，不加新公共错误合同。软预算仅查询之间、事务健康时可Incomplete，真正数据库／取消／内部错误绝不吞成Unknown。

### 3.10 迁移、历史与清理

计划迁移为 `server/migrations/sql/00053_page_ingest.sql`；采用前重新核号，DDL形状按00：changesets.ingest_count int NOT NULL DEFAULT0，命名CHECK>=0；changeset_sources三列 `(changeset_id uuid,seq int,node_id uuid)`均NOT NULL，PK(changeset_id,seq)、UNIQUE(changeset_id,node_id)。没有FK（node或cs均无）、没有deleted_at、新purger或持久ready。保留两个指定partial index名；notebook log index继承00037的真实predicate（无deleted predicate），actor index为deleted_at IS NULL AND ingest_count>0。Down丢marks，Up不补历史；runtime只授DML，无MAINTAIN。

历史count字段必填，REST `ingest_sources`为非null数组、固定seq前N nullable IDs：列表3、单cs20；内部虚拟log.md读取前5，再用其原整条预算。已trashed/purged或路径读取时消失者保留null位置，不压缩后补第N+1个。通过changeset_sources PK `(cs,seq)` 对多个cs做unnest+LATERAL限N；count不因预览/清理变化，≥cap沿既定下界展示。ingest=true专用ListIngestChangesets/ListIngestChangesetsBy/ListIngestChangesetsOfPage三语句，false/省略沿旧三路；原200ms守卫不变。

决定76：MCP两scope的log/detail先把同seq前3/20映射为完整path/null逻辑预览P（m=len(P)），只为原48KiB wire界取P[:s]；`ingest_count`、非null `ingest_sources`及bool `ingest_sources_cut`均required，后者唯一规则s<m，不以count>3/20判cut。零来源0/[]/false；非空P可裁到[]/true；元素null保位、数组整体null无效。不缩短path、不跳长项补短项、不增加来源cursor；REST及虚拟文件DTO不加cut。app在原授权/归属校验后批量解析来源，真实读取错误仍失败，REST不多扫路径。

detail只在首个非nil header做来源阶段：固定完整header其余字段、非nil空changes、原n>0时真实Start（n=0用原next），Fit来源后固定P[:s]再Fit changes；k=0/n>0带header+Start，0<k<n用对应change cursor，k=n用原NextCursor。续页header=null跳过来源阶段；nil header且无change可装保原errNoChangeFits，不造循环空页。log先保原完整rows Fit；仅n>0/k=0处理首行，不跳行。固定next为n>1时首行cursor、n=1时原NextCursor；空changes且`changes_cut=(原changes长度>0)`固定，完整P裸header能装便保全P，否则尾裁；再固定来源Fit changes。最终`changes_cut=(实际changes长度<原长度)`，false沿旧omitempty，来源裁后全部changes装回必须false/省略；与required sources_cut独立。来源空前缀仍装不下则保内部错误，不删其它字段或抬界。详细字段与候选编码见ports§5，独立oracle/四cut组合/控制见矩阵§2.1。

page现purger里增加source叶循环，再做两个父分支。每条leaf语句LIMIT batch，`FOR UPDATE OF s SKIP LOCKED FOR NO KEY UPDATE OF c SKIP LOCKED`；eligibility是deleted_at<before，或trimmed且deleted_at IS NULL，并同时无items/revisions。循环到本语句返回短批；叶删除数计入page purger返回，保证jobs继续同轮处理。两个父DELETE都加NOT EXISTS changeset_sources。已锁住cs不等待，别的cs仍清；短批含SKIP LOCKED表示当前可清项穷尽，锁住的留后run。2500marks/一个cs和500cs×3分别要求同次purge run清完可清叶/父，随后PurgeNotebooks同次成功。

checkPages三个独立反例：mark的cs必存在（允许soft-deleted），ingest_count等于marks数（清理合法eligibility窗口单列，不能普遍豁免），有marks必须kind=edit。源node不要求存活或存在，历史null有意保留。migration schema names+负数/重复seq/重复source反例、Up/Down/Up、runtime-role与sqlc同步必验。ALTER取得ACCESS EXCLUSIVE后保持到事务commit；真实容量从获得锁到整个goose Up commit计时，含CHECK scan/两index建，不能只报index耗时。

### 3.11 契约、错误次序与权限矩阵

准确YAML候选和Go字段见附件。public不加ingest错误码/模式/config；新增 listIndex 声明index.list，reader成功，无503／lint槽。既有page.virtual_file各路径实际可达范围补tools.json与码表，同Step双语；日志现译文沿用，不创第二套错误。

| 操作 | 保持的判断次序 | 真调用矩阵行 |
|---|---|---|
| REST listIndex | 结构binding；cursor400；type/tag/parent422（互斥）；limit422；notebook404/index.list；parent404；读取 | 新operation全角色，standard与LLM、外本/删除/隐藏、parentasset、无权限+坏cursor/limit交叉 |
| MCP index | actor，cursor400，字段422，limit422，notebook404，parent lookup | notebook/account两scope；普通与REST同排序；LLM实体来源保留，reserved parent虚拟码 |
| read/list/open_notebook | 每个原用例的path/window/cursor/limit先后不重排，再notebook授权／lookup | 两scope，规范virtual成功／alias拒绝／无身份／隐藏本；notebook scope外层Gate的HTTP404单列 |
| MCP write/edit/delete/move/versions/backlinks/log(path) | 现字段／授权先后，统一lookup保留，move末段CheckContract | 两scope逐工具真实virtual_file；standard与嵌套真实页成功；虚拟拒绝后DB未写 |
| batch | 计划树／正文两遍逐项判断→完整计划后edit lock预检→unit guard | `[op0 locked,op1 virtual]`准确op1；反序、虚拟父链、目的地、先move再write；原REST可比守卫序保持 |
| REST listChangesets | cursor400→notebook404/page.history→page_id404→client/limit422；ingest非boolean绑定400 | true/false/省略，reader成功、隐藏404，actor/client/page各种组合 |
| MCP log／changeset | AtPath/InNotebook原各自授权/字段次序；log增可选nullable boolean ingest（仅true筛） | 两scope真实count、seq完整path/null及required cut；普通kind、reader/standard、隐藏/外本与Gate规则保持 |

MCP tools描述改list/read/grep/index/open_notebook（S1），log/changeset（S2-1）：明确来源是seq前3/20的非分页预览、required cut仅因wire；“changeset reads one in full”只指全部changes可分页，不承诺追回裁掉的来源。两个scope的api/mcp/tools.json在S2-1手工同步，并由真实tools/list完整对象比对；原生gen不生成该文件。平台只注册InputSchema，不增OutputSchema框架；输入strict/码/工具数量及原描述预算保持。虚拟read每次生成、无version，grep/search忽略，真实index仍列真实页。不是P5系统说明改造，不挤占P3已验余量；账户scope和notebook Gate不得为统一表而重排。

### 3.12 日志与可诊断性

不记录title/path/rawtarget/JSON/body。正常virtual读／目录沿现HTTP/MCP访问日志，不加lint运行info。ingest仅在有候选且证据不足时以受控debug诊断unit cs/notebook ID、Unknown原因枚举、raw/eligible头数、候选与抑制数量；正常无候选不输出一条每保存info。SQL错误沿现error包装回滚，不能重复报敏感内容；日志测试只截取子测试起点之后的行。是否debug默认可见沿现slog配置，不新配置项。诊断是内部辅助，不能成为公开产品状态或Ready标记。

### 3.13 S1实施落点与已核修订

S1按原4提交落地统一Lookup/list、REST目录、虚拟RR read、真实export/LW/成本；Tools负责Virtual nil校验，普通读与虚拟exclusive RR区分。字面&编码保实体身份，FirstChanges相关LATERAL保nullable与物理界；E05控制实际生产ReadMeta映射。门禁期修复L1兼容、CI日志、最终显示字节界、实际scope错误诊断及generic测试pool；Make仅按既有规则分组与移界。

77正式三席为A零发现、B-I1 Important及C-Q1证据问题。0b0增加反引号显示转义与真实domain/两scope/ZIP回归，累计十二个代码/测试/CI提交，minimum/postgen及fresh EW357已接受。其余修复门禁、精确CI、原席核修和独立冷热补测按[S1记录](reviews/P4-S1-virtual-files-review.md)的实际记录收口；不把77的接受外推到0b0。73–75、原4/3/3、fixed guards与原CI循环不变；C-Q1是验证协议处置，不新增产品规则。

## 4. 网页

S1/S2每个required API字段／工具码带中英problem文本、生成TS、现fakes与fixtures；knip不收“以后S3会用”的未消费导出。S3才增加真实入口。

LLM树顶部两普通导航链接index／log（可读中文“目录”“历史”），有文字等价图标，无nodeID、菜单、拖放。只从useNotebook.mode派生，不向PageTreeStore塞伪node。普通本NotebookPlaces的目录仍可用。降级原same event connection更新mode后两行消失，来源内容重新可编行为仍归既有LW12，不重建整个故事。

目录route在NotebookLayout下 `/:slug/notebooks/:id/index`，useDocumentTitle；type/tag在URL，types来自有界catalogue，未完整/long页提示与当前URL保留按A11已接受合同。typed组按type-key、内项按UUIDv7建先后；untyped按已加载父页分组。source area一行到health#undigested_sources（包括规则计数0的异步anchor已有P3行为）；不显示pending数。More分页不抢已交互用户焦点，加载/失败/重试有读屏文字。新打开/筛选/游标为 `(location.key,type,tag,cursor)` SWR key，换键清旧代；迟到响应不得覆盖新代，Back/Forward按新打开处理。非事件驱动，不在page/link事件上自动重算；普通retry.ts策略，无lint的server_busy特殊抑制。响应mode与当前notebook不符时同健康面板提示；重连错过降级要靠既有notebook刷新后标出变化，不伪造目录已实时刷新。

历史URL `ingest=1`，SWR key与请求带boolean，标记附加在edit原kind旁；三条preview与详情20条，附件用openOf落到父/根focus，null显示已删除而无链接。切筛选／分页／迟到处理与原历史规范一致。虚拟unresolved link判定复用一张Go/TSfixture：仅尚未resolved、归一后root-relative单组件、剥一次.md后TitleKey四保留键，在LLM导航目录／历史；reader亦可。已resolved的真实同名对象保持原landing，nested/非根relative不误劫持。

父页选择器建议留M12（服务端parent_id/at_root本期完整）；无新本地存储缓存、无页面内嵌实现解释。所有公开文本中英一致。

## 5. e2e 分工

| 故事 | S1 | S2 | S3 | 非空最后一跳与限制 |
|---|---|---|---|---|
| LW9 | — | API PUT＋MCP batch产生真实mark，历史filter/详情；两scope按原故事约定 | 页面写wiki→历史看到ingest与来源，再filter | oracle确切一个source/一个cs，不以两个空集比较；禁止故事直接造marks当行为证明 |
| LW10 | API／MCP read/list虚拟文件、whole export实际zip/contributed、reimportskip | log文件增加真实source预览 | tree两行／read-only落点、导出对应内容 | 普通/子树无贡献对照；grep不命中；导入数据库断言在现page/transfer断言文件 |
| LW13 | 普通notebook REST/MCP同目录序列，readonly API/MCP验证 | — | NotebookPlaces真实目录、type/tag/More、reader | API/MCP读故事沿原无DB约束；不以DB读取代替用户可见目录 |
| LW12 | — | — | 原两个tab、同一个event stream上加virtual行消失一步 | 不reload，不换连接逃掉事件，现source编辑恢复／去重变化断言保留 |

测试准备在各请求deadline之外，真实浏览器经过实际serve资产；S3至少按原三浏览器套件要求，不把仅Chromium探针说成三浏览器结果。组件W与真实EW独立记录。EW每个baseline/mutant/restore同锁内fresh build server与web，核源SHA、bin/嵌入资产、实际HTTP资产、进程argv/身份，不能跑旧dist证明新源码。

## 6. 实施提交次序（每提交可绿）

以下是候选提交单元。相邻单元若因真实生成物／编译依赖必须合并，在正式设计审查时改表，不允许先提交红中间态。无未使用导出、nil生产依赖或“测试桩替代正式接线”承诺。

| 提交 | 随提交完成的链 | 可绿依据／定向验收 | 建议范围标题 |
|---|---|---|---|
| S1-1 | virtual Lookup/包装、所有NodePaths构造点、batch/list路径、V/Glob/OpenNotebook前缀与MCP编码/描述/fakes | 旧普通路径不变；virtual路径与分页真实使用；V/Path控制、MCP两scope；没有未用接口 | `server,api: add virtual paths and their listing (M10/P4/S1)` |
| S1-2 | 共用ListIndex类型语义、REST parent/types/source/mode、HTTP路由/action矩阵/generated client/双语 | rest入口实际消费新增DTO；旧MCP同usecase；type/排序/错误先后/计划基准 | `server,api,web: expose the notebook index (M10/P4/S1)` |
| S1-3 | 内部file ports＋纯生成器＋ReadPath独占RR＋两处ToolDeps构造（含!race batch_cost_test.go）；PageRead元数据及真实version消费者；normal read原行为；两read文件 | 一次落完整生产consumer，nilcheck实际触发；输出界、RR交错、offset与SDK | `server,api: read virtual files in their snapshot (M10/P4/S1)` |
| S1-4 | 早构造只读wiki服务、传递真实export plan路径、contributor及import核对；LW10/13和新cost/plan recipe | 移除真实contributor必被zip断言抓；100k/容量计划/所有控制；Step全门禁 | `server,e2e: contribute the virtual files to exports (M10/P4/S1)` |
| S2-1 | 迁移/grants/sqlc、Marks store及bounded purge、checkPages、history required字段与专用筛选；REST3/20、MCP两阶段Fit/双cut/两scope SDK、schema/fakes及旧asMCP/完整golden同提交 | 普通历史0/[]/false、真实Add持久样本；独立s*/相邻候选与真实raw-token、H06–H15逐腿控制；不剥键/放宽DeepEqual，Brief/VersionChangeset不添字段；无producer承诺 | `server,api,web: keep ingest marks and read their history (M10/P4/S2)` |
| S2-2 | unit首次memo/BeforeRevision、Event桥、Capture、coverage、candidate SQL、Add完整producer，reindex不回填；合法深树写入到MCP预览 | 真PUT/两scope batch先有缺非空marks的业务red，再接通；核哈希/解析/持久seq后测ASCII20及深& log3，另复验H08 source-Fit退化；原回滚、100k/最大batch/交错/计划保持 | `server: mark proven new source ingestion in the write unit (M10/P4/S2)` |
| S2-3 | log.md/export前5source、LW9、完整容量/迁移/成本/所有控制与新增矩阵 | S1文件实际消费新历史DTO；historyindex守卫及同runpurge非空；Step全门禁 | `server,e2e: show ingestion in virtual logs and its stories (M10/P4/S2)` |
| S3-1 | 目录route/services/SWR/Places与完整组件测试、type/tag/More/mode提示 | 已有REST；每个导出被route消费；tsc/knip/vitest；无需后一步 | `web: browse a notebook index (M10/P4/S3)` |
| S3-2 | virtual树行、Go/TSfixture落点、reader、history标记/filter/detail及两语言 | 已有API；页面焦点与附件/null；同一个事件连接更新 | `web: navigate virtual files and ingestion history (M10/P4/S3)` |
| S3-3 | LW9/10/13页面、LW12增量，W/EW变异与身份/三browser、全门禁 | 真实端到端；不是只加测试名；完整Step验证→CI→三审 | `e2e: exercise the virtual pages and ingestion views (M10/P4/S3)` |

S1-1不能要求Virtual内容依赖先存在：`ToolDeps.Virtual`及其nil要求在S1-3才引入，前缀已可list，规范read在S1-3交付。S2-1新增零值历史字段是完整可用增量，并不声称ingest生产已开；S2-2同commit接上唯一producer。所有阶段代码变化对应的API contract/types/fakes同commit；若迁移与HTTP生成需要同一工具链，一起生成提交。

S2-1明确更新bootstrap/mcp_history_test.go的asMCP及log/detail完整期望、page/adapter/mcp/history_test.go的完整golden和相邻字段断言：零值也required false，非零来源按独立树/seq映射path/null；仅log/detail这两种shape加字段，Brief/VersionChangeset不递归添加，实际wire裁剪腿用独立s*。不得剥新键、投影旧成员或放宽DeepEqual/完整JSON比较。S2-2真实producer与S2-3虚拟5/最终验收仍原分工，不加第四代码提交。

## 7. 测试、交错、变异、容量与 CI 余量

`specs/P4-virtual-ingest-test-completion-matrix.md` 是本节的逐行控制表，含精确候选测试名/子测试与文件，正向非空oracle、单故障、交错barrier和完成标准映射。名字是设计拟定的将建测试，不冒称6992已有。原有守卫保留名称与预算；6992现有Make/CI事实见源码事实附表，新测试仍须在实施时加入实际recipe，不伪造已存在的选择器或结果。

决定76后当前146个控制ID：原136完整保留，H06–H15连续新增10个，34个S2-1基础控制腿，H08四腿在S2-2真实producer上复验，共38个计划阶段腿；这些新腿均NOT_RUN，不混入S1历史执行数。两scope各验log四种独立cut组合（00/01/10/11），完整rows优先、来源优先于changes、首行next/至少一行前进、detail Start/续页header-null与无漏重遍历保持。

每控制必须：正常树同命令过→单故障源码变异→命中指定业务断言首败→从备份恢复→同命令过。不能以编译／fixture失败／超时当成功捕获；改坏时不gen、不用git checkout --。每个病灶独立运行，不能把“删Capture且吞error”绑成一条。未改页v1/missing/晚bad JSON必须确实存在；常量用literal旧版本而非跟被测current常量同步变化。交错用现interleaveOnIndex和可观察barrier证明事务被锁的那一刻，不用sleep。

新增来源oracle只测固定来源阶段：测试手写完整seq/path/null与独立header数据，空changes、真实Start/row-next、其它字段固定，sources_cut=(k<m)，log changes_cut=(原变化数>0)固定。用标准Encoder SetEscapeHTML(false)并仅剥一个encoder末尾LF，外层锁定SDK TextContent编码后读raw text token；不调用生产encode/Fit/TextBytes/MemberBytes/builder，不从返回长度或MCP header反求期望。独立枚举B(0..m)先得s*，核B(s*)≤49152，s*<m时B(s*+1)>49152，再比较真实P[:s*]；另从实际HTTP保存原始text token，Σ(len(token)-2)≤49152且Text/structured完整语义一致。只对来源固定envelope要求相邻不可再加，不给原rows/changes Fit强加全局最大保证。detail/log各做full-source header超界时s→s−1控制，保其它字段与后续Fit，必须被新oracle抓住。

S2-2合法producer用raw根+9层255B父page，下挂255B、不同非空内容/哈希的asset（第11段是asset，不是第11层page），20路径各2563B、仍<4096B；真实wiki PUT/batch先核成功/精确非空marks/seq，再测两scope detail。log用合法含&标题的三深路径与≤4096B message，核真实resolver IDs，不把禁止< >的标题或仅算术当生产证据。独立s*不能预设19；长项后有短项/null时仍只保连续前缀。旧worst helper是合成极大envelope，不等于真实producer；空前缀[]/true须m>0且独立B(0)≤49152<B(1)，记录adapter/store/producer证据层，未实测合法producer空前缀不得宣称可达或通过。S2-1的Add读取证据、S2-2首次缺marks red、接通后Fit退化与S2-3虚拟5各分账。

正式成本必须同时满足：normal非no-op保存无candidate零全扫；100k全健康、完整EOF而真实新T至少一个mark；已有旧S→新T仍成功；最大合法batch许多source、近10000cap仍非空；raw深/宽树、assets、trash/missing/stale、旧metadata revisions、晚首sources和合法最大JSON；read100k最坏inbound、export100k+1000cs且不读inbound；历史页200ms原预算。Go TotalAlloc/peak heap、完整ID参数编码、PG rows/buffers/TOAST/custom/generic、同index锁时长与普通保存耗时分开留原始样本，不能把SQL+Scan墙钟叫PG CPU。

计划比较no statistics、ANALYZE、强制custom、真实generic切换与另一更大notebook干扰；检查实际参数/执行范围/重复JSON读次数，MATERIALIZED不免计划验证。迁移真实容量、锁等待/持有到commit、索引空间/写放大、Up/Down/Up/runtime角色原义务保留；P2的6M空/已有LLM旧迁移证据只供对照，不冒称本P4新ALTER已测。

完整门禁沿02：`make lint gen-check knip build`、`make test`、`make test-go-short`、`make e2e`；每提交minimum含build/vet整个模块lint/短测，Web改动tsc/knip/vitest，提交后实际gen-check。保留原heavy.sh、全局锁、**锁后3GiB与T7预检**、原timeout/request4s/lint2s/capacity；没headroom先解决环境，不盲重跑或降低线。成本/guards的!race不能被race/short替代。每条规定process两轮平均×2.4≤原Go timeout一半；算法guard按两轮各≥3样本中位数的较大者×3候选定界，原已有预算不借此放宽。实际Makefile五bootstrap分片逐一双轮；指定两次main每片Test均值严格<450s，不能换wall/预测数。绿色分支CI在三代码审查前，修复CI在原审查核对前；所有身份/命令/exit/原始首败如实保存。

入口容量实数：已接受6992 own CI41284728142 attempt1的五份bootstrap Test为463/309/362/404/313s；它独立于已接受4e6e/df9指定配对。第一片单次463s不能证明给新增测试留有安全余量。按实际Make分片安排新测试、每进程双轮，并在本Phase规定的实际main pair重测；保持每片均值严格<450s，不沿旧本地数宣布有余量，不自动放宽超时或门槛。

**当前未实测**：完整SQL能否sqlc生成、真实PG计划、RR交错、100k＋maxbatch在原预算内成功、set/参数heap、各迁移上限、全部新测试和门禁。静态证明只到所有权／锁可达性／逻辑全集条件和手写反例，不给性能接受结论。

## 8. 完成标准与文档的新文字

以下是满足相应验证后才可写入仓库的**拟新正文**，不是当前状态。正式文档工具须取真实main原段落作唯一锚点；本节给完整语义文字，不让文档起草者用“完善文档”代替。方括号身份仅在有原件后填，不允许猜日期、SHA、CI或通过数。

### 8.1 S1完成后的文档文字

`v0.1-design.md` §13.1 路径／读事务／扩展点项追加：

> LLM Wiki 根下的 index、log、index.md、log.md 由路径 Lookup 统一识别，包括 batch 的模拟树；只有规范 index.md、log.md 的 read 生成内容。其他路径操作答 page.virtual_file，move 的最终保留目标仍由 CheckContract 判断。children 游标 payload 的 V=1/2 分别在两虚拟项之后，与真实 sort_order/id 分域；V2 从真实首项继续，真实负排序不丢。普通 ReadPath 不开事务；规范虚拟分支独占一次只读 RR 快照，在快照内重新判断 notebook 权限／模式并读取全部生成数据。整本导出贡献者复用 export 已有快照，不嵌套，不调用 lint。

`v0.1-design.md` §13.1 接口校验例外项追加：

> REST listIndex 与 MCP index 使用同一 ListIndex。游标 400、资源无关过滤与 limit 422 在 notebook 404 之前；parent_id 与 at_root=true 互斥。REST 返回 mode、有限exact types、types_complete、unfilterable_type_pages 与 source_area，并排除来源严格后代；MCP 保持真实页目录。二者按 type key/id；index.md 按 type key/path，区别属于表示次序。

`v0.1-design.md` §13.4 扩展点／成本／同规则项追加：

> 虚拟read的RR交错、V分页两侧与负sort、40KiB中和后的界、whole/subtree/standard贡献差异由真实入口验证。清空组合根贡献者时LLM whole zip与contributed断言失败。index.md 100k最坏入链与whole export 100k页/1000变更集的实际样本、计划、内存和预算见本Step审查记录；既有预算不放宽。

M10 `handoffs/M7-transfer.md` 项1/2/6处理进展追加：

> P4/S1在原export snapshot内只给whole LLM贡献有界index.md/log.md；standard/subtree不贡献。生产ReadMeta→Contributed与精确NFC导入、真实ZIP和冲突/fallback已验证，结果见S1记录；metaPaths仅为测试helper。重新导入的raw 2/AGENTS 2限制保留，存量升级留M11，M7整份仍open。

M10 `handoffs/M9-P1-mcp-registry.md` 项3处理进展追加：

> P4/S1接通两scope的虚拟路径、list/read/open_notebook和工具描述；V游标、grep忽略、canonical读与其他virtual_file、真实SDK界已按记录核对。P5 prompt、系统说明与人工清单义务仍open。

### 8.2 S2完成后的文档文字

`v0.1-design.md` §13.1 单元／锁／派生数据项追加：

> ingest 是同一个 page 写入事务中的派生历史。page在首次真实写之前保留node与content旧revision/bytes；linking在现index advisory lock内、任何Replace/Delete前Capture旧v2 JSON/边和起点资格。KnownAbsent只能由可信unit新建证明；missing/stale、未改页coverage不足不能当空历史。未改页在取得index锁后的ReadCommitted串行边界逐raw节点keyset读到明确EOF，完整核page revision与index head；不得锁其他writer的content行。改删页首次Before不由此后移。旧target bytes不能证明时以UnknownAll或完整PossibleTargets上界抑制相关候选，不能用late bytes冒充Known。正常无候选保存不扫全本；SQL、取消与内部错误使整个unit回滚。

`v0.1-design.md` §13.1 清理／历史项追加：

> changeset_sources无外键与deleted_at；同changeset的marks只追加，单cs最多10000，count饱和显示10000+。REST列表/详情按seq给前3/20 nullable IDs，虚拟日志前5及整条预算保持；MCP两scope给同逻辑预览的完整path/null wire前缀，required ingest_sources_cut仅由字节尾裁决定，两个cut独立、来源无cursor，消失节点保留null位置。ingest=true走三条专用历史查询，false或省略沿原查询。page清理器先按批SKIP LOCKED删除符合父清理资格且无items/revisions的source叶，循环到短批并累计返回；两个changeset父DELETE都要求NOT EXISTS sources，使同次job能继续清notebook，不另建purger。

`v0.1-design.md` §13.4 不变量／反例／成本项追加：

> checkPages检验mark有changeset、count与mark数一致（合法清理窗口除外）、有mark的kind为edit，每条都有独立SQL反例。真实PUT/batch验证删前Capture、完整未改页coverage、首sources JSON一次和全peers后target过滤；双事务争同来源仅先提交者标记，错误整unit回滚。健康100k和最大batch必须产生预期非空mark，普通无候选必须零coverage全扫；样本、物理SQL计划、heap/编码、index锁时长、迁移ACCESS EXCLUSIVE全事务持有与剩余负载边界见P4/S2记录。

M10 `handoffs/M8-history.md`（6992 已 done，不重新打开原移交）仅追加相关进展；本 P4 历史交付由 P4 自身 §10／审查记录验收：

> P4/S2已按真实记录落实edit changeset的追加ingest标记、true筛选三SQL、按seq的前3/20/5 nullable来源及10000饱和展示；清理源叶与父防线、同job notebook清理和三不变量各有非空控制。历史原kind、权限、撤销/恢复合同与原200ms页读预算保持；本条完成身份见P4/S2记录，不把P5或M11义务顺带关闭。

M11 `handoffs/M10-mode.md` 追加：

> P4的ingest只相信当前来源表示与充分旧／未改证据。旧v1、missing/stale必须先reindex，之后恢复未来可靠判断，不补此前历史标签；Unknown与PossibleTargets不是升级ready状态。批准升级前仍处理旧根index/log保留名，否则whole export可能contributor_conflict。本移交不启动M11，也不把未改页最终树／解析近似写成全unit旧快照精确。

M12 `handoffs/M4-performance.md` 追加：

> P4新增virtual read/export、ingest coverage与候选判定、历史ingest索引和marks迁移的实测记录在[真实P4记录]。保留具体环境、数据形状、冷/热/custom/generic、heap/PG范围、原预算和锁占有样本；生产并发负载、MVCC老版本积累、大实例迁移停机仍待M12。不得以设计静态SQL或无候选快路证明100k真候选可用。

### 8.3 S3与Phase收尾文字

`v0.1-design.md` §13.2 URL／缓存／焦点／链接项追加：

> 两种模式的目录在NotebookLayout内，type/tag与history ingest=1写入URL；目录SWR键含打开的location.key、筛选和cursor，换键去旧代，迟到结果不覆盖当前页。目录不随事件自动重算，失败沿普通retry策略，mode改变给提示。虚拟树行由notebook.mode派生，为可读链接，无节点动作；reader的尚未解析虚拟链接先导航目录／历史，已解析真实同名链接保持原落点。history来源附件用openOf，null无链接；异步加载与More遵守已交互用户焦点／滚动不被抢的规则。

`v0.1-design.md` §13.4 e2e项追加：

> Go与TypeScript的虚拟目标判定已在S3-2接同一手写样例表并逐项核对。LW9 API/MCP/page验证真实新增ingest，LW10验证虚拟读与实际导出贡献，LW13覆盖普通笔记本目录，LW12在原双tab同event stream增加降级后虚拟行消失。API/MCP只读故事仍按原无DB约束；每个W与EW单故障分别保留真实首败、恢复及运行资产身份，不把组件通过称浏览器通过。

M12 `handoffs/M5-polish.md` 新增“目录父页选择器”项：6992现有1–32项，末行“之后的 M 推迟的体验事项照样追加到这份移交。”为追加约定。S3完成后在该唯一末行前插入、按当时尾号编号并保留末行；没有既有selector项，不预占33，不改open状态或实施M12。

> P4 REST parent_id/at_root合同已完成；网页父页选择器按00允许留作打磨。目录type/tag/More和普通本入口已在P4交付，不把它们一并推后。

每Step Phase §10用以下完整格式写真实结果：

> Sx已实现[本Step最终行为]。实施基线[SHA]，代码[SHA]、Step文档[SHA]；合并见00第12节。三位独立审查[记录链接]的已核实Important/Minor已逐条处置并复核；偏离为[真实偏离或无]。门禁／控制／成本／迁移的实际命令、首败、恢复、身份与CI为[证据链接]。尚未完成[后续Step与剩余边界]，本段不将它们计为本Step接受。

00 §12/13和总体§15只在相应步骤真实完成时记：

> P4/Sx完成，交付为[实际行为]；代码／文档／merge／record及其精确CI见[记录]。P4总状态在三Step与Phase done文档的main CI接受后改为完成；P5/M11不因本记录开始。

Step reviews必须含对象／方法／核实finding与处置commit／原席复核／主持者决定／每条mutation／门禁CI／五片用时／接受与后推。§2.3决定71／72已接受；真实实施数据仍只由事实补入。正式设计、审查记录与受控00修订最终同一个文档提交；Phase没有额外first-draft单独commit要求。所有文档用唯一段落断言脚本先试干净副本并做链接检查；原PREP和失败历史保留。

## 9. 交给后续Phase

P5获得真实virtual行为、tools说明与log字段，继续prompt/握手系统说明与人工清单，P4不提前注册prompt。M11获得保留名冲突、旧表示重建、Unknown／最终树近似边界，只有负责人说开始后进入升级。M12接真实测量，向现open打磨移交新增目录父页selector项；生产负载／停机义务保持。若选定候选被正式三审改判，必须同步00锚点、端口SQL、矩阵与完成文字，不能留下互相矛盾的两套合同。

## 10. 结果

### 10.0 正式设计发布时的记录（历史原文）

2026-11-01：P3完成事实不变。三位fresh初审和两轮原席核对已完成，主持者依据实际root回执SHA256 `c0d9fa1c4c3fd176e306c7cbab7ac644b841b798457c8830c1b05451054810cb` 接受设计及实施决定71／72；当前已核实finding零open。正式设计自身commit/CI尚未知、待文档门禁与精确main CI验收；S1–S3未开始。所有P4产品、SQL、测试、生成、迁移、成本及CI结果仍待实际实施后填写。

### 10.1 S1 虚拟路径、目录、文件与导出

最新S1收口见[S1自然转录](reviews/P4-S1-virtual-files-review.md#s1修复record后验随s2正式设计转录)：record ca4688a1的完整CI与预登记e07/a1→ca/a1配对已获ROOT接受，S1实施和主线集成闭环。以下各测量、CI与pending段保其原写入时点；决定76的S2正式设计自身minimum10/commit/postgen/CI待验，S2实施未放行、未实施。

以下原结果、CI与pending段保其各自写入时点；e650/a1失败、a2完整复核与指定配对容量失败，以及本轮Make调度修复和af392f分支a1失败/预登记完整a2接受，集中见[§10.1.1](#1011-s1-bootstrap-容量维护)和[S1记录](reviews/P4-S1-virtual-files-review.md#e650记录失败与bootstrap容量维护)。本轮未来两次main配对尚未发生，S1未闭环、S2未放行。

S1实现统一Lookup/四根键、V与真实负sort分域、真实SDK Fit、REST/MCP共用ListIndex及决定72 finite exact catalogue、独占RR虚拟read、真实whole LLM export/元数据及NFC reimport。四实施提交及其后门禁/审查修复累计十二提交；167c→15b仅改未发布message、同tree。S1代码、完整本地门禁、精确修复CI与三位原席第一次核修已接受；原a1f成功/fd两失败及b4 glob失败及986文档guards超时取消保留；878/f689独立测试修复及ac84调度拆分、修复文档`2fc103dd`及本次no-ff `97c9ad8e`的精确CI已接受。本record自身CI与新指定两main配对待验，S1集成未闭环、S2未放行；S2/S3未实施。

当前候选0b0df18c2ddd4b6779954458d37a617f66a42c5a/treed6614eb5ef0a03ebe5aa2c2f3091b65f8541d296修复77三审的B-I1反引号显示问题，minimum/postgen及fresh EW357已接受。77原A零发现、B-I1及C-Q1报告保历史身份；C-Q1采用独立PG shared_buffers冷/同操作warm补测，OS缓存保留，不增加原CI循环。最终验证分别为：0b0最终源96条/288实际业务过程已ROOT接受：前86与后10保两控制器身份，各条0/1/0；另保第87条因原3GiB预检拒绝、未启动Go的记录；C-Q1 observer另账；0b0最终源新完整native19已ROOT接受：8项inventory、五bootstrap分片双轮及全nonbootstrap race；五片process均值341.073/271.625/292.684/297.700/250.199s，均严格<450s；指定main CI pair另验；0b0 fresh完整6targets/50recipe双轮100过程、1333 recipe-local identities及6case/24固定guards已ROOT接受；各pair原容量均满足，最大占半界65.443%；原首轮100容量失败保留，C-Q1独立补测另账；0b0 fresh build/EW session14866已ROOT接受357单过、0skip/retry/flaky，实际171.155s；仅Chromium，S3至少三browser仍待执行；精确0b0 CI41454798547 attempt1已13jobs/38必需steps成功；10份完整server日志及6case/24固定guards通过，web182/3926、EW357单过；guards job1707/1800s，branch五片357/312/399/400/321s，指定main pair另验。正式审查：77初审A无发现、B有B-I1 Important、C有C-Q1；精确0b0第一次原A/B/C核修均通过，B-I1与C-Q1关闭，新增Important/Minor均0。各旧失败、来源对应、96控制分账、冷热实测与原席核修详见[S1记录](reviews/P4-S1-virtual-files-review.md)。

原4s请求、2s提交/worker、历史200ms和六case的24固定限值不变；完整成本仍50recipe/100process/1333 recipe-local nodes。

Step文档`2ec19be8793338c59a5f4014fe5812bd98831341`的[CI41469628154 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41469628154/attempts/1)与no-ff合并`a1f21e75804eafd3739558612933ae0c90b44d33`的[CI41476185798 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41476185798/attempts/1)均已由ROOT接受；有序父、同树关系、实际上海时间与CI尝试见[S1合并记录](reviews/P4-S1-virtual-files-review.md#文档与合并记录)，合并登记见[00§12](00-M10-design.md#12-phase-进度表)。

fd record `fd2607ab9fec4cade43fb28622873607891ae58f`的[CI41482903019 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41482903019/attempts/1)有会话fixture与revert成本两项失败；878仅修会话测试，其精确代码CI已接受。原12s/8s guard和15s请求期限不变，本地计划观测不解释旧CI；原件见[S1原补充记录](reviews/P4-S1-virtual-files-review.md#main-record-ci失败与会话fixture修复)。后续b4文档CI41501780779 attempt1的grep-glob成本1.252742361s超过原1.25s；独立结构反证确认旧app fixture只有十万根可达，已作测试父ID副本及计时前全量校验修复，正确图本机单样本/独立BFS分账见[新补充](reviews/P4-S1-virtual-files-review.md#b4文档ci失败与grep-glob-fixture修复)。不改生产或预算、不归因历史CI，正确图与旧成绩不可混标，手工full-load重新标定义务留M12。f689测试修复、提交前minimum8与独立核对已接受；postgen与[精确代码CI41510773390 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41510773390/attempts/1)已接受；986文档CI41517408885 attempt1的guards整job超时取消保持；ac84调度拆分保全原18条recipe及两job各1800s，代码CI和本次修复文档`2fc103dda645c4cced48dd830e445d2b5018d868`与实际no-ff `97c9ad8e96c51a68a2824a893ddb92fd82340ae5`分别由[CI41523895769 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41523895769/attempts/1)、[CI41531309743 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41531309743/attempts/1)、[CI41534799713 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41534799713/attempts/1)接受；同树、有序父及原件见[修复合并记录](reviews/P4-S1-virtual-files-review.md#修复文档与no-ff合并记录)。本record自身CI与指定新main配对仍待；只指定本次实际修复merge（first parent fd、second parent上述文档）＋本record两个完整接受的main attempt的五片实际Test均值仍须严格<450s，不跨attempt拼接。当前仅Chromium；S3仍须至少三browser，S2/S3未实施，P4尚未完成。

当前guards调度拆分：ac84代码、最终修复文档与本次no-ff main的精确CI已按11 server legs / 14 jobs / 40必需steps / 11 artifacts接受，见[S1实际合并记录](reviews/P4-S1-virtual-files-review.md#修复文档与no-ff合并记录)。本地全量入口仍串行core12＋sources6；两guards job各保1800s，原Go recipe/全部limits不变；六P4成本case不替代sources六项完整性。本record自身CI及指定两main五Test均值严格<450s仍待真实接受；旧986取消及13/38/10单guards历史保留。

#### 10.1.1 S1 bootstrap 容量维护

record `e65043fe059c2788338278a2e68a1b80ee7c0a80`的CI41540276246/a1 costs-lint失败，100k summary/sample-1 Execute 2.002009056s返回busy，第14条lint recipe未运行；summary原固定median guard为NOT_EVALUATED，未作新校准。同head完整a2已按14 jobs／40必需steps／11 artifacts及原成本、18 guards/14 lint recipes接受；六个实际样本成功只说明本次未复现，根因仍UNKNOWN，没有性能修复或可靠余量结论。旧失败保留。

指定97c9/CI41534799713/a1与e650/CI41540276246/a2五片实际Test为461/402/266/416/275s与472/317/359/285/258s，均值466.5/359.5/312.5/350.5/266.5s；第一片不满足严格<450s，真实pair verifier拒绝。按既有[M9/P2§6.5](../M9-mcp/02-P2-writes-history.md#65-ci-的余量)只把11个TestE顶层从1移至2；Make非注释改动仅FIRST/SECOND，实际499 race顶层分配168/40/116/123/52→157/51/116/123/52，第三至第五片成员不变。历史27.63s×1.6=44.208s及422.292/403.708s只供选候选，不替代实测。

实际代码`af392f641f043e9497ce39abd7734ac3d55007c3`（tree `d96032187be0079cdc5e807cba37741b7c105c5e`）仅改Make两个selector及说明注释；[旧字节native17](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-native-01.json)、[B-N1实际桥接](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/nit-application-01/receipt.json)、[新字节minimum02](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-minimum-02.json)、[提交后postgen](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-postgen-01.json)及[原三席第二轮补核](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-independent-review-02.json)已接受。[精确分支CI41556086210/a1](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-code-ci-failure-observation-01.json)实际失败，11份原件封存；[一次完整a2预登记](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/code-ci-whole-rerun-decision-01/decision.json)后执行的同head完整a2已由[ROOT完整代码CI及原件接受](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-code-ci-a2-final-observation-01.json)核齐14/40/11和原costs/guards/lint；[ROOT原三席03接受](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-independent-review-03.json)无新增I/M/N，代码与完整代码CI前提已满足，文档自身CI及后续main配对仍待。完整旧ee1327输入native17真实exit0，五片各单独双轮process均值298.377670/276.292745/271.939962/291.183963/242.885033s；两轮各499顶层、4099真实RUN/终态（4095 pass/4 skip）、4097归一身份。B-N1唯一注释差异和17对真实Make展开连接到最终38163B/SHA6f6de4输入，旧native/旧minimum/控制source不重标；新minimum02仍是提交前e650＋最终dirty Make，postgen才绑定真实af392f。六控制为15/15/53/157/6/0、各真实0/1/0；原三席第二轮无新增Important/Minor/Nit、B-N1已解决；原席03随后已核完整代码CI。详见[S1本轮记录](reviews/P4-S1-virtual-files-review.md#e650记录失败与bootstrap容量维护)。

CI41556086210/a1精确af392f分支push最终completed/failure：11 jobs success、guards-core失败、e2e/image跳过。原core第8条的10000 nodes/400000 history revert实测12.45394147s>原12s，同条many-tops2.786200471s PASS；core第9–12条未运行。五bootstrap成功jobs的Test为425/336/274/356/259s，仅是该失败attempt的局部观察，不是整CI或main配对接受。原checker实际exit1拒绝CI，11份原件已封存。此失败与fd的12.299969446s同属原large-revert guard越界风险，性能根因未知；server树/workflow未变不能证明环境原因或排除所有调度影响。ROOT预登记且只请求一次同head完整a2，未取--failed子集；本次已完整成功并接受，a1继续失败。[A后发失败补核](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/code-ci-failure-review-a-01/report.md)及[ROOT接受](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-code-ci-failure-review-a-01.json)当时确认1项Important为a1未通过的CI验收阻塞，非已证实的新Make因果缺陷；HTTP200完整请求超过12s，不是15s请求超时、Go超时或DB错误。此前A/B/C02无新增I/M/N只属于当时源码/本机范围，未覆盖此后发阻塞。[ROOT原三席03接受](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-independent-review-03.json)确认本轮完整代码CI前提已由预登记a2满足；a2的large9.058581014s、many-tops1.917504607s仅说明本次未复现超限，不称性能修复、标定或环境归因。

维护不改五片数、测试名/源码/fixture、race/count/15m、CI五job各25m、原costs/guards/lint recipes或workflow；14/40/11、两guards各1800s、本机每片独立双轮和未来指定两main每片实际Test均值严格<450s均保留。原glob1.25s、revert12s/8s、request4s/summary worker2s与summary原固定median guards、15s请求期限不变。完整代码CI与同席03已接受；本分支文档自身CI及后续no-ff/record两个完整main attempt分别待验；若邻居实际越线，按既有规则撤回移界并处置，不提高超时。合并见[00§12](00-M10-design.md#12-phase-进度表)。S1未闭环、S2/S3未实施且S2未放行，O25 OPEN、M12 open与正确图full-load重标定义务不变；本轮不额外修改M12已open项。

#### 10.1.2 容量维护文档与合并补记

本轮分支文档`3166bbcde7e23b4eb614b6367ea7e95c801e91cb` / tree `c879036e7db93178922d966d199f54f08f0db710`（parent `af392f641f043e9497ce39abd7734ac3d55007c3`）的提交前minimum8与提交后postgen已通过并正常推送；文档自身CI41566238435/a1：[完整接受](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-document-ci-final-observation-01.json)。本轮no-ff为`e07cc1964238b3d248c791c124d9a9fc48dd9253` / tree `c879036e7db93178922d966d199f54f08f0db710`，有序父为`e65043fe059c2788338278a2e68a1b80ee7c0a80`、`3166bbcde7e23b4eb614b6367ea7e95c801e91cb`；精确main CI41569049575/a1：[完整接受](/home/parallels/.local/state/nervewiki/m10/p4-s1/bootstrap-capacity-repair-01/root-main-ci-final-observation-01.json)。合并身份见[00§12](00-M10-design.md#12-phase-进度表)，原件与前置检查见[S1合并补记](reviews/P4-S1-virtual-files-review.md#容量维护文档与no-ff合并记录)。

本record须是该no-ff的三文档直接child；自身完整CI及预登记merge→record两个attempt1的五片实际Test均值严格<450s仍待验。分支CI、本机或旧97/e650失败pair不替代；任一完整attempt失败或容量越线先诊断，不自动重跑或换样本。§10.1.1与此前pending保各自记录时点，旧失败/UNKNOWN、O25 OPEN、M12 open及正确图full-load重标定义务不变；S1未闭环、S2未放行。

## 附录：已核代码与绑定方式

[源码事实](specs/P4-virtual-ingest-source-facts.md)给出6992的关键合同／行与入口证据；[源码pins](specs/P4-virtual-ingest-source-pins.json)去重保存当前Git blob、SHA256、Git mode、字节数与行引用。[端口](specs/P4-virtual-ingest-ports.md)、[SQL](specs/P4-virtual-ingest-queries.sql)、[控制与完成矩阵](specs/P4-virtual-ingest-test-completion-matrix.md)、[types](specs/P4-virtual-ingest-types-boundary.md)、[文件表](specs/P4-virtual-ingest-files.json)与[00修订](specs/P4-virtual-ingest-00-revisions.json)共同构成设计。旧fa／78167比较、组装命令和审计过程保留在冻结PREP，不重复刊入仓库。

实施前按最新main复核迁移序号、实际构造／生成物与测试选择器；本设计的静态核对及旧P3成功都不能计为P4产品验证。若三审改变拟方案，同步00修订、端口／SQL、测试矩阵和完成文字。
