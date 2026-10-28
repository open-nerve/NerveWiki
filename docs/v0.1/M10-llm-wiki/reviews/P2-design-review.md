# M10/P2 来源区：设计审查记录

日期：2026-10-27。基线：`403841fc75d0e5ac6b5f39447158213c0e07b2fe`，树 `b3665215f745d4dcfe7e8d339d4882e6d95b4a63`。对象为 [P2 完整 Phase 设计](../02-P2-source-area.md)及其对 [00 总设计](../00-M10-design.md)的修订。

**结论：设计已审查并由主持者接受，待 S1 实施。** 三位独立初审共核实 2 Important、2 Minor，另有 B-Q1 接线问题及一项未核实的有限 reader 工作量怀疑。作者修订为冻结 v2，原 A/B/C 全文复核后，原四项发现与 B-Q1 均在设计层 resolved；有限 reader 的协议缺口已补齐，实际计划/工作量仍待实施验证。复核无新增已核实 Important、Minor、Nit 或 Q。主持者全文读原报告、复核报告、完整修订差异与相关上下文，核对核心源码后接受设计与决定36–43。

本轮是**静态设计审查**。没有 P2 生产实现、产品测试、运行时变异、数据库/容器、迁移、600万测量、性能或新 CI 结果。文档的精确 main CI 仍须核对，绿后才开始 S1；各 Step 的独立代码审查、门禁和合并后 CI 另行完成。P2 与 M8 均未 done。

## 1. 输入、独立性与实际方法

初审 A/B/C 是三个新鲜独立上下文，均阅读同一冻结 v1 的完整 Phase 与拟00文本、相关权威及当前真实源码；不读取作者/主持者其他推理笔记或另两位报告。重点分别是 A 来源契约/历史/顺序，B 哈希/去重/事务/代价，C 网页/API/MCP/E2E/完成度。使用与主持者相同的模型，无模型覆盖。复核由原三位进行，读取唯一冻结 v2 的完整设计及拟00、自己原报告和必要冻结配套，不读 mutable 修订稿或另两位报告。

实际方法为文件阅读、`rg`、Git只读/哈希、逐段源码跟踪和具体输入的手工推演。初审 A/B 核设计与拟00哈希并深入其负责调用链；C 另核四份产物及76个基线blob。复核 A/C 各核六份产物和94个基线blob；B 核六份产物、HEAD/干净状态与重点源码，**未重新核验全部94个blob**。主持者核六份产物与94个blob、完整差异及报告。哈希全核不等于逐行通读94份源码，三位均明确其深入/抽查范围。

一位只读协作者调查后台分组/批界，另一位给出有限reader SQL/排序与证据方案；这些是修订的事实/方案输入，不充当正式独立审查。作者整合，主持者先对决定43与reader协议定处置，随后冻结v2交原三位复核。作者、主持者、审查者均未把静态推演当成运行复现。

冻结输入及原报告作为树外审查证据保留，以以下摘要定位；不把机器路径写成仓库依赖。正式 Phase 相对冻结v2只更新当前接受状态及本审查链接，协议、SQL、接口、测试/变异、完成投影未变。

| 冻结输入 | SHA256 |
|---|---|
| v1 Phase | `2b80f09e747e5bcf5c41bd7e9dae25c83be64dd0a221c837f98cac03123d84da` |
| v1 拟00 | `ae3c393ab032b21d3aa93bc21f381e0630ce8c59c1edee2b58f7ae4b47d73eb3` |
| v1 outline | `026a34f847041309206bda5134e41acc7242274decce5d270bea64a13e17c725` |
| v1 manifest（76 blobs） | `414e81fd017bae770c7a7b65d11ca0c3aac4db8f779c0da75cd45ff41c07103c` |
| v2 Phase | `210b0e60fb467a851d4fb7e31016b02e12bb41d185bd4488295aa32dc6382040` |
| v2 拟00 | `72908098389fcd3c86eeaedcc6748748fe11c15e025d7cbcd5295f0a7d9aaac1` |
| v2 outline | `80bfd4cfdc0e6c169c73e6cc97bce8ae195883b5c2185684bb0210eb7ec041a1` |
| v2 manifest（94 blobs） | `4426419b161403198bd835d9a0ed7238b41bdb3e2a4fcbc1fbfd00765994181b` |
| v2 review-record | `0270cb3fa591cbc8f4755a36858269913ac53b2f6c779e1fa33021c0d251cc0b` |
| v2 semantic-changes | `8387ef508dda7f20051d5f575db30e8457a1d86db47ee8e9a7ddb3e3e5f02ce2` |

| 原始报告 | SHA256 | 结论 |
|---|---|---|
| A-report.md | `40417b6357a4f2a42fc4dd4355b8427bf77cf20155754bc72fa798ec147e6b50` | 1 Important |
| B-report.md | `692fe5991eccb98a1ef83a3616689c75a472918d780c6c3406385498998e4c32` | 1 Minor、1 Q、1未核实SQL工作量怀疑 |
| C-report.md | `19511fae9ff2568d0c25e073d6e7952d40b643ea249ceadcbd19eb6b93a9228b` | 1 Important、1 Minor |
| A-recheck-v2.md | `62b9c3f444618bb7cf86da29485d7a45d17c5cb3a3dd3d01b01a338e93b1ed5b` | A-I1设计层resolved；无新增I/M/N/Q |
| B-recheck-v2.md | `9497090aa4ac59e9ed11aed56bed1af7f916d83015ddc4d3fe4588d0b126ad22` | B-M1/B-Q1设计层resolved；reader协议闭合，运行pending；无新增I/M/N/Q |
| C-recheck-v2.md | `9e82ed7a5e9b3ee23f92856702507e189d2934e9dbfe6374bb71c43adda8e539` | C-I1/C-M1设计层resolved；无新增I/M/N/Q |

以下引文摘自对应原始报告，保留原文核心；其余为主持者处置与修订核对汇总。源码路径中 `page/`、`transfer/` 等相对 `server/internal/modules/`；`bootstrap/` 相对 `server/internal/`，行号按本记录基线。

## 2. 初审发现、主持者处置与修订

### A-I1 Important：后台预检与实际压缩/同组执行顺序不一致

原 A 的核心：

> 新去重预检对现有后台撤销调度的设计缺口，不是把尚未实施的 P2 功能当作基线回归。
>
> 不要仅新增 Rewrite/Bytes 字段后就继续把原 RevertFacts 的结果当作后台结果。

原 v1 将共同 `RevertFacts` 输入用于同步及后台预检，但真实三层不同：`page/domain/revert_schedule.go:327` 先恢复各组再写回；`page/app/jobs.go:346–385` 的 compact 把每恢复成员的 write-back 紧随该成员；`page/app/unit_revert.go:521–545` 实际执行在本批第一次遇 group 时恢复本批该组全部选中成员，之后跳过该组 marker，不会把不同组合并。

可达历史链：先有空的 raw/F1、raw/F2 及 Old/New 目标；D 以一次真实 MCP batch 新建 S1(A=`[x](./Old.md)`)、S2(B=`[x](./New.md)`)，再把 F1/Old 改名 New，真实 deferred rewrite 使 S1 收敛为 B。同步 undo D 得 C，先改回目标令 S1 成 A，再分别删除 S1/S2，留下独立组 A/B，C 的 S1 版本 rewrite=true、base B。同步 undo C 是 restore S1(A)、restore S2(B)、rewrite S1(B)，可通过；后台 compact 却 restore S1(A)、rewrite S1(B)、restore S2(B)，最后新增必重复。同批也存在，跨批会留下前批；原预检会漏掉无并发的必拒任务。

**主持者接受 Important。** 保留M9调度/原子片段，不为证明所有合法缩批重造调度器；要求先提交准确协议，另以决定43定承诺范围。修订 P2 §3.6/3.13：S1 就抽取生产 `batchEnd`/`nominalBatches`、非nil `JobBatches` 配置桥及实际结构 `revertActions`，后台按 compact→未缩减名义cuts→本批同组动作预检；同步保留Ops。S2 在同一结构加入hash/snapshot。实际writer的当前Group/States、continuation、retimed/frontier/skips与前提检查保留，不以plan初始payload代替。

名义批不是执行调度；不搜索所有分割。没有并发时，真实资源界缩批仍可能使来源拒绝；前批提交保留，失败批回滚；名义拒而更细可行也仍拒。这一取舍和用户说明明记，不再声称最细分法总更严格。真实API两独立组、同组两方向合法模块fixture、真实halving与独立oracle分别验收，变异38/39/43覆盖退回同步facts、group/cuts和实际缩批错误。**原A复核：resolved（设计层）**，无运行结果。

**S2-3实施更正（2026-10-28）**：上面的“可达历史链”保留为当时设计审查原文，不再视为已证的公开入口事实。真实执行显示同批创建与deferred rewrite合并后base=nil、rewrite=false，同步undo也不运行link rewrite，故原例所需的C版本不存在。主持者没有改生产历史语义迁就它；`TestTheSourceCreationRewriteUndoKeepsItsRecordedBase`改核真实记录链及duplicate/nil-target/no-enqueue。合法模块历史fixture另外核对同/跨组、compact与cuts的相反结果，实际`38-active-job-snapshot-uses-sync-facts`控制的catcher为`TestSourceJobPrechecksKeepOppositeCutOutcomes`、`TestSourceJobIndependentGroupsDoNotCoalesceForDuplicates`。详细更正见[P2 §3.13、§7.3](../02-P2-source-area.md)；`p2-s2-3-root-acceptance.json`接受提交`9bafdaa67513de135c19f51b5ba9678cac59fe82`的增量，不代替整S2三审。决定43的名义预检/原调度取舍保持；不声称不存在其他可达公开历史链。

### B-M1 Minor：同一步内部命中也要依起点key给 duplicate_of

原 B 的核心：

> 内部匹配与 SQL 外部匹配共用同一个 `baselineKey == comparedKey` 判断；“前项在当前步刚加入模拟集合”不是省略 ID 的充分条件。

反例是单元起点已有 raw/F/A(H)，管理员把F移出，在区外F下新建B(H)，再把F移入。内部A先于B，重复回滚后A仍是原来源/H、B不存在；v1内部前项一律无id与自己的baseline定义矛盾。路径依据为 `page/app/apply_batch.go:576/666`、`unit_move.go:73` 与 `unit.go:342`。

**主持者接受 Minor。** 修订 P2 §3.9/§8/变异18：SQL与内存共用起点来源资格+kind+hash；真实两scope MCP验证A的id/路径与回滚状态，起点F/A在区外的对照不带id。旧节点移入、restore、empty-first-body、rewrite新hash均不能因UUID已存在而被误指；真实前批已提交可指，预检模拟前批不算提交。**原B复核：resolved（设计层）**，真实字段传播与变异待执行。

### C-I1 Important：worker规划前的重复不是guard最后一跳

原 C 的核心：

> 删掉 mode write guard 注册，仍会在这同一预检退出。
>
> 不移除生产预检来迁就本行。

v1在enqueue后、worker开始前加重复，却同时承诺worker Plan有来源预检；实际 `transfer/app/background.go:93–101` 先Plan，`:111`才首batch，故此例零RunBatch，由预检拒，不能证明guard或不存在的前批保留。

**主持者接受 Important。** 修订 P2 §7.2/变异24、41与00投影为四层：请求已重复→零enqueue；enqueue后、worker Plan前加重复→零RunBatch、无前批；真实Plan成功返回后、首RunBatch前加重复→guard拒、零前批；真实前批RunBatch返回且commit后、下一批前加重复/降权→guard拒、非零持久前批保留。decorator只barrier/计数，保留生产Plan、不伪造计划。同步锁内预检与guard-only测试装配也分别标识；若实现新增上游拦截，必须重给实际守卫证据。**原C复核：resolved（设计层）**；实际调用与mutation杀死位置待验证。

### C-M1 Minor：MCP逐工具增量码不闭合

原 C 的核心：

> 上述合法拒绝的全程序测试就会报该工具返回未列码，不能达到 S1-3/S2-3 的独立全绿目标。

v1明细漏write/batch的immutable与move的duplicate，虽规则及旧00要求这些行为。`bootstrap/permission_matrix_mcp_test.go:93/97/102` 是实际码表；`bootstrap/mcp_client_test.go:305–310` 按调用工具自己的表核码。

**主持者接受 Minor。** P2 §3.12闭合表逐格列notebook/account两入口的可达答码：S1 write/edit/batch→source_immutable、delete→source_admin_only；S2 write/move/batch→source_duplicate、move/batch→source_too_large。保留既有protected/virtual_file/admin_only及后代矩阵，不新增工具、不给edit不可达duplicate。`TestSourceMCPToolsAnswerEveryAddedCode`、变异42及契约/同提交文字/fake同步。**原C复核：resolved（设计层）**，每格实际答码待执行。

### B-Q1及有限reader：精确化，不冒称已实现缺陷

**B-Q1原核心**：只给worker PlanRestore接入会让请求先enqueue；直接把完整检查放旧CheckRestore内又会在busy之前全量读。基线 `transfer/app/start_background.go:66` 先CheckRestore、`:70`才claim，旧签名没有admit。

主持者明确选 `CheckRestore(..., admit)`，不留二选一：基本权限/root/trash/expiry之后恰一次claim，再必要模式点读；LLM完整group/hash/snapshot在后，普通无完整来源工作；page/app/root、transfer、bootstrap、fakes同提交。沿现有StartRevert的release/defer，删第二次claim，所有后续失败/取消/排队与成功均释放；busy零完整读、duplicate零enqueue、单claim/释放有变异40。**原B复核B-Q1设计层resolved**，A/C亦核相邻接线。

**有限reader原怀疑**：v1只给查询名，照搬旧 `ORDER BY sort_order,id LIMIT` 可能仍读/排序巨大整层；B明确没有新SQL，不能定为已核实缺陷。修订 P2 §3.8给 exact SQL：parent数组 `unnest + LATERAL`，内层按name_key、复用00042的partial parent/name_key索引，内外同take LIMIT；不足take才证明整层完整，恰好max仍下一层take1探测；overflow丢弃、不排序/读后层/hash/asset；只有完整后Go按Level/sort_order/UUID整层排序，保留旧Subtree顺序。最多返回/物化10,001只是逻辑界，不保证物理IO。

A/B/C接受该协议足够具体，未保留设计疑问。无/有统计×custom/generic四组真实计划、prepared generic实际执行、巨大宽层/多父/另本A→B→A、实际范围/缓冲与访问计数、成功末层空父探查、变异44仍是S2义务；本轮未执行SQL或EXPLAIN，不能宣称已用预期索引或达标。

## 3. 原审查者复核与保留的验收责任

| 原审查者 | 复核原文核心 | 主持者核对与处置 |
|---|---|---|
| A | “原 A-I1：resolved（设计层面）。本轮没有新增核实的 Important、Minor、Nit 或 Q。” | 全文报告及源码核对；三种表示、same-group合并、生产批界接口、保留runtime payload、两方向合法fixtures/真实halving与普通模式均闭合。按决定43接受，无运行成功外推。 |
| B | “B-M1 与 B-Q1 在设计层 resolved；原有限 reader 怀疑所指的协议缺口已补齐，实际计划/工作量仍待实施验证。” | 全文报告及相关上下文核对；baseline、admission与snapshot、cursor/reader、非唯一索引/测量相容。无新增I/M/N/Q；不把未重核94blob写成已核。 |
| C | “C-I1 resolved（设计层）；C-M1 resolved（设计层）。本轮无新增已核实 Important / Minor / Nit / Q。” | 全文报告/94blob及关键实际链核对；四层证据、MCP全矩阵、UI/session/上传、每提交可绿和完成投影闭合。无新增I/M/N/Q。 |

A另给出真实资源界fixture的**静态可实施参考**：四compact步骤 `[restore F, restore A, write A, restore B]`，A恢复/写回Joined；真实halving先 `[0,4)→[0,1)`，F提交后新from1贪心得 `[1,4)`，再缩 `[1,3)`，最后 `[3,4)`。合法测试配置 `maxHistory=1`，真实准备A/B历史行，`page/app/jobs.go:633–645/699` 的多节点TrashHistory超界，单节点F及同一A两step按原豁免通过；SQL `trash.sql:102–112`与现有`revert_job_test.go:662–685`支持其构造方向。这是实现参考，**不是新fixture已经建立或跑绿**；必须真实构造连续版本/组、retimed/frontier及实际ErrBatchTooLarge，不用stub冒充。

仍待实施的证据包括：S1 rewrite真实SQL AND/undo-of-undo与所有事实生产者、特殊no-op/changed守卫分离、batch逐项与Merged；S2完整字段/两删除/Forget、内部与SQL provenance、admission失败释放、四层guard/veto注册变异、真实公开历史链与同组两方向、source_hash随机重建/确定性交错；所有REST/MCP每码实答；S3真实server lock结束/readonly/不再autosave、文件精确bytes/上传取消、附件富导航/缺树重读与LW12无reload。计划44组和后续审查追加变异都要实际记录，静态resolved不代替任一组。

600万无LLM/已有LLM实际goose冷启动事务、独立冷扫描诊断、四列对三列的空间/写成本、10万来源/60k计划与成功末叶成本、两份精确main Test均值也仍待完成。入口均值440.5/317.5/260.5秒仅说明首份余量9.5秒；IndexItems30ms与grep600ms稳定性继续留M10收尾，不因设计通过而关闭。

三位均未核实应删除的过度设计：共用结构遍历/生产批界针对已证实的表示差异，CheckRestore admit复用已有模式，有限reader复用已有索引，四列游标应对大计划排除与rewrite允许收敛；没有调度搜索、临时排除表、通用新事务平台、全局缓存清理或新来源总配额。实际实现仍可精简私有helper，不能删掉必要事实和验证。

## 4. 主持者的决定（负责人可以改判）

这是**实施/Phase设计**序列，接P1决定1–34及维护35。它与00 §11原总设计审查决定1–40是两套编号，不覆盖旧决定。36–42在初审时是提议，主持者先有条件接受核心方向，修订并经原三位复核后全部接受；43在修订协议阶段获明确范围接受，亦经过三位复核。每项实现结果仍待验收。

| 编号 | 决定 | 边界与理由 |
|---|---|---|
| 36 | 一次既存子树从区外移入来源区至多10,000节点，超过422 source_too_large；Move/Relocate、batch单move、同步/后台undo place一致 | 现有普通单次移动/undo携带无统一10k界；新增来源的逐节点工作要有真实界。保留普通10001成功、区内/移出及搬来源根成员不变；有限reader先停，不完整Subtree后len；已知领域错误在前，overflow可先于未读深度，完整旧历史规划不假称有总10k内存界。 |
| 37 | 来源version_restore和有效旧session的same-byte保存也拒绝；普通/非来源/普通一次性同字节no-op保留 | 现有早返回在guard之前，窄纯预检与changed注册guard的证据分开，不让普通no-op新增EditLock影响。 |
| 38 | duplicate_of以本单元起点的活来源资格+kind+比较hash为准，SQL与内部命中一致 | 不把新来源等同新UUID；移入/恢复/empty-first-body/rewrite都可能因回滚失去被比较来源，模拟前批不是commit；可稳定指向的既有A仍给id。 |
| 39 | 历史提示改为区外还原正文再移回，重复先处理冲突，移回仍须去重且不保证成功 | 管理员没有已有来源正文例外；保留临时改名、先放回后删除、移入再写回偏严规则，不推荐必被拒的“先移回再写”。 |
| 40 | content-only历史补相关祖先/元数据，不读整本树；完整计划X在Go模拟、外部候选排除整个X | 保留现有快路，大计划每次读/写数组≤10k，不用分块NOT IN漏排除；真实执行表示按43补全。 |
| 41 | S2完成600万空/已有LLM两种实际goose Up冷启动事务；独立冷扫描诊断另列 | 实际事务起点只冷一次，不能把分别冷却的CHECK/index相加冒充停机；只自有PG文件、manifest/fincore，承认启动/设备缓存；M12真实服务器仍重测。 |
| 42 | 非唯一source_hash部分索引在原三键后加id，以key/id有界续读；同快照E保证排除候选行界 | rewrite可收敛，不能unique或取首项即停；每次总Take≤10k、总≤K+\|X\|，全X留Go。额外空间、建索引/回填与写维护成本纳入600万/10万测量，不称免费。 |
| 43 | 保留M9compact/Joined/同组批内一次恢复/合法缩批；开始和worker按生产未缩减名义批计划的实际动作预检 | 不枚举所有分法、不假设细分更严；名义拒不搜索细分，无并发真实缩批也可拒，已提交前批保留、失败批回滚，同步/后台可不同。S1就接同配置非nil JobBatches和结构遍历，S2加hash/snapshot；实际payload/continuation检查与调度不被预检替代；产品说明用分步/顺序/本批未保存/已完成保留，不露内部类型。 |

## 5. 文档落实与下一入口

本次仅新增正式Phase与本审查记录，并对00的迁移/事实/正文规则/后台预检/duplicate_of/工作界/并发证明/网页/API/MCP/Phase分工/测试/风险及状态作对应最小替换，保留原总设计决定和P1/维护历史。00不并存两套相反的后台保证或duplicate规则。头和§12只标“P2设计已审查、待S1”，§13记本次设计与真实维护入口；不提前写Step、P2或M8完成。

设计文档在干净归档副本用唯一旧段/基线哈希/目标不存在检查演练，主持者审完整diff后才在main应用、提交；文档精确main CI绿后才能开 `m10-p2-s1` worktree。后续三个Step分别照Phase §6–8实现、门禁、变异、三审、修复/核对、文档、no-ff合并及两份精确main CI。M8只在整个P2验收后done，M7/M9仍open；P3–P5及M10收尾原分工保留，不开始M11。


**2026-10-28实施记录（不改写原设计审查历史）**：S2-1–4增量与成本/迁移已核；当前SetSourceHashes为UPDATE self-join，保留当时CTE旧值证明及后来无统计二次方修复。A-I1实施更正仍按原附记分层。正式A/B/C、尚未关闭的B-N1及当前159项义务/本机验证/33b4代码CI已验、剩余文档复核/main集成见[S2审查](P2-S2-source-hash-review.md)，S2尚未验收。

**2026-10-29 B-N1后续闭环**：保留以上实施记录及原发现；实际应用三处self-join规范后，原B复核无新增发现，主持者核原件接受关闭。回执及文档/main集成剩余项见[S2审查](P2-S2-source-hash-review.md) [F30–F32]。