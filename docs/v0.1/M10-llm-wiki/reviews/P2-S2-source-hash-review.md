# P2/S2 来源 hash 与去重：实现、审查与集成记录

> **代码33b4、本机验证及精确11作业CI已接受；文档与main集成待闭环。** 159项控制义务按各轮fresh/reuse范围分列，不称159独特产品bug。三份正式审查及后续产品修复复核已接受，C-M1、C2-M1、B2-N1、NAME-REV-01已关闭；B-N1已在本次真实文档投影后由原B复核并经主持者接受关闭 [F30–F32]。所有旧失败、诊断与来源差异保留。本文为主持者汇编，不是新增独立审查。

| 项目 | 当前状态 |
|---|---|
| S2-1/2/3 原子增量 | root已接受；提交443a8c85 / 05ca33ea / 9bafdaa6 [E02–E04] |
| 正式A/B、独立A/B及第三份三列迁移测量 | root已接受；单次固定语料与各自范围限制保留 [E14–E16、E19–E22、E25/E26] |
| HTTP45三状态基线及五预算校准 | c7之前同3171输入的36+9样本及六成本行双轮已验 [E34–E40]；保留历史，后续SQL与reset断言修复须刷新受影响测量 |
| S2-4代码/增量门禁 | 代码25f88a27已提交22文件；六增量门禁及提交头gen-check已获root接受 [E41–E44]，六份root文档未提交 |
| S2-5 c7控制与本机验证历史 | c7的124控制、97/89清单、89 race各两轮、31成本/guard、完整Web/E2E和静态门禁已验 [E51–E71]；不是后续修复源码的最终绿 |
| 三份正式审查与修复复核 | 原A/B/C已核；C-M1 CLOSED@c7，C2-M1 Important/B2-N1 Nit CLOSED@ca8；B-N1规范文档Nit经实际投影后的原B复核 **CLOSED** [E45–E50/E87/E96/E98；F30–F32] |
| 当前代码与最终验证 | `33b4da5a`及CI40718770843全部11作业成功；旧129＋TrashChildren13＋History8＋Name9，共159义务已按各轮输入/局部重放接受；本机门禁、32成本/guard与当前成本pair见下文[F01–F29]。文档B-N1原审已关闭；doc/main闭环仍待 |

## 范围与实际差异

实现范围：00050/hash完整不变量、有限source-entry、重复及单元起点provenance、SHARE first正文advisory、同snapshot预检/恢复admit、实际后台名义批与最后一跳、import/duplicate字段/API/MCP、真实交错/随机/成本/迁移与定向故事。S3网页与跨context故事及P2done不在本Step完成范围。

审查时必须沿实际实现核对以下差异，不能按旧设计示意推断代码：SELECT-list SRF与has_afters；当前writeback为单条UPDATE self-join，以old.id=n.id返回实际变化目标的旧kind/hash，n的notebook/live/distinct条件不变；内外scalar LIMIT及内层LIMIT后的notebook filter；普通SyncRestoreNode一次StateOf点读；ask(snapshot ctx)授权签名；Probe双tuple真实Index Cond。决定43保留名义计划不搜索其他cuts、缩批可能改变允许结果、前批保留取舍。

S2-1 的 MATERIALIZED previous/changed CTE 方案与原旧值正确性证明仍按当时历史保留；S2-4三状态HTTP暴露末尾CTE join的二次方物理工作，当前实施段与成本证据须按self-join修复更新，不能回写旧S2-1为当时已用新SQL。

原A-I1公开创建/rewrite/undo链被实际历史否定；原设计审查的当时接受/resolved保留，并追加实施更正。真实API回归、合法page模块fixture、真实worker调度加fake端口各自陈述，不拼成未跑的全栈。D-I1真实PG MaxConns=1 SQL与调用者trace分层；增量审查与本稿都不算正式三审。

## 正式三审与处理

三份审查以代码头 `25f88a2730456940bd52efffd899e8ac827a9f27` 为起点，主持者已逐项核原件。C-M1修复头 `c7cec1fab9cc0586f4ba3754876aace2df79c570` 只改两Web文件；六份已核文档字节保持。Go证据按相关Go输入全等继续适用，不称旧新完整快照全等。 原C核最终3171文件工作快照与c7已提交树恰差六份root文档，其余3165一致；不能称3171全已提交。C-M1关闭回执所绑定的原审查报告SHA `e55505ab723d2725ed7df2428de2265ecf13fd9b8733ed6b168b31459a763d43`、结果SHA `b7f5a2bf05a4555de2c2835e4bcd541e9b84d2ab76551ebf35b77304d925cde2`。

| 审查 | 真实结果与反向控制 | 发现、处置与原审查复核 |
|---|---|---|
| A：来源/历史/provenance | 0 Important/0 Minor/0 Nit；13 top-level、78 pass，mutant 67 pass/11 fail，恢复78 pass；报告SHA `8bf67da2181bc8c8364a9fe5d54ebfb1e56021a506796508ece3a18bca72cbd1` [E45] | 无待修发现；最终集成未由此宣布完成 |
| B：数据/迁移/并发/物理计划 | 0 Important/0 Minor/1 Nit；6 top-level、27 pass，mutant 26 pass/1 fail，恢复27 pass；报告SHA `cdfb6c0a4848e6e0be5c5e4df53563861bb3b6a7c720d2656118786915679d29` [E46] | **B-N1 CLOSED**：原发现为P2 §3.7、00及总体设计三处当前规范误写旧CTE；实际应用self-join更正并保留历史后，原B复核无新增发现，主持者核原件接受 [F30–F32] |
| C：REST/MCP/report/Web与E2E | API/MCP/import55 pass；report4 pass→1 pass/3 fail→4 pass；Web268；E2E15，每项一次、0 retry/skip/flaky；observed-results SHA `2ce8fb8bd8c6e19a77d08a408f280a6a19629a889f7501aea4defd871e7b6583` [E47] | **C-M1 Minor CLOSED@c7cec1fa**：Web历史fake先构造自/后代移动环，sourceArea→isUnder可能不终止；正式修复c7cec1fa已提交并核gen绑定 [E49]；原C独立35项通过、输入与恢复已核，主持者接受关闭 [E50] |

C-M1独立观察：真实HTTP移入自身/后代均409且状态不变，正常200。有界观察器计目标projected数组全部find调用，不是仅祖先访问；original/restored normal1/self51/child51，临时修复1/0/0，三套检测退出1/0/1。观察器抛错不当作产品错误响应或全局timeout。临时诊断已恢复，不能以临时绿取代正式修复/复核。

S2-2和S2-3同名增量发现按各自范围保留，不与本轮C-M1或B-N1混淆。A控制映射既有memo-current-key语义义务，不新增独特bug数；C decoder与既有adapter-only字面变异相同，但真实HTTP report的catcher是额外义务。最终124项逐项执行、输入绑定与分类已由E51接受；控制数与不同语义bug数分开，详见下表。

## 已接受证据与历史边界

| 证据组 | 可填入的已验结论 | 仍缺或不能外推 |
|---|---|---|
| E02 S2-1 | 35项最终义务、42组历史triple；7组为替换旧单元轮的最终复演，旧7组保留历史；SQLC/PG/生成核对 | 不是whole-S2 replay |
| E03 S2-2 | 20常规+1外部coverage triple；有限范围已接受 | 外部coverage下Go保持pass；不叫21个Go语义kill |
| E04 S2-3 | 最终53语义变体；历史54triple/162执行单列 | 重复旧空附件replay不加作最终变体 |
| E05/E06 九控制 | 全27阶段0/1/0、sameargv、选中>0、实际assertion非build/panic/Go总timeout、源精确一处改变与恢复；230原件/919检查 | 八产品目标+一测试输入；不代替最终全步变异 |
| E07 Probe | 修复版三状态五形态；真实generic PREPARE；原SQL回退无统计5形状失败、另外10物理访问失败，仅exhausted两叶200001 | 15个不是15个物理失败；第三次work原件不补造前三套 |
| E08/E09 故事/随机 | 15定向E2E；六seed各79成功/9拒绝、17类最低覆盖 | 不是全E2E/最终race；MCP fixture历史失败保留 |
| E10–E13 成本 | 18完整计划、54完整X、暖100k索引及旧16行运行已验 | 旧HTTP15只ANALYZE，新的45基线另见E34；当前受影响六行及暖索引HTTP写入已另由E39/E40接受；旧测量仍保留为历史 |
| E14–E16 迁移A/独立A | 官方A6M、独立三次cold扫描的实际计时/原件核验 | A来源部分索引空，不代表populated暖索引 |
| E17/E18 400诊断 | roundtrip、强锁等待和取消回滚已验 | 非6M停机时间、非公开undo故事、无真实资产文件 |
| E19–E22 迁移B/独立B原DO | 官方6M/300k事务21.580 s、CLI21.610380 s、最终四列索引36,806,656 B；独立DO同事务18.236 s、原样DO18.226946 s、实际更新300k；两组全量oracle/13表已验 | reset非fresh49；cluster LSN非隔离DO WAL；8048流式stdout未存全文，ledger绑定receipt；不替代正式fresh49迁移 |
| E23/E24/E27 确切owned清理 | 官方B、第三份三列与成功400 backfill smoke的容器/PGDATA已清理、对应已接受原件身份不变 | 各自owner范围，不外推其他小库或失败候选；不是新测量 |
| E25/E26 第三份三列 | fresh49 B的6M/300k、9287回执/28ledger/8048批、oracle/13表已验；PG事务21.543 s、driver21.558410 s、最终三列27,312,128 B | driver池4/官方CLI池10；四列多9,494,528 B是固定语料实数，单次时间差不归因纯id开销 |
| E28–E31 writeback定向修复 | root接受单UPDATE self-join；永久回归旧SQL red99,990,000 joinRejected/100,010,000 CTE行，新SQL green0/0与old PK 10k×1；相关6个测试事件通过；真实撤销3次200 | 只定向修复；45基线另有E34接受，HTTP5+索引写入六行双轮另由E39接受，最终replay由E51另行接受，不回写为该定向回执已覆盖全步 |
| E32/E33 HTTP失败与诊断历史 | 首轮42成功+无统计revert首次500到4 s；无统计move三成功2.83–2.86 s超历史1.3 s；同语料10k参数/9999变化诊断末尾join拒绝99,980,001次 | 诊断EXPLAIN6.598 s非HTTP时间；与永久回归100k/10k是不同fixture；不把42+定向3拼作同冻结头45验收 |
| E34–E37 HTTP45基线/校准 | 同3171输入，36+9两命令，分别16/13个Go pass事件；15组45成功；六表统计、四个实际应用连接的设置、初始行/文件指纹及4s/2s均核对；五界500/1400/7600/7600/3100 ms | generic设置非实际PREPARE；原样本用校准前常量；新增3控制见E38；六成本行双轮另由E39接受，完整本机门禁另由E66接受，文档/main集成待验；精确33b4代码CI见F14 |
| E38 新增3控制 | 3×0/1/0、3171源精确单处变更与恢复；两状态检查控制在timer前实际assert失败，baseline/restored各3请求、mutant0；旧SetSQL控制实际物理assert失败 | 两fixture/instrumentation+一production physical，不能合称12全产品语义kill；此处SQL只选1测试，最终6事件选择及124项控制由E51/E55另行接受，保留本次1测试范围 |
| E39/E40 六成本行双轮 | 同3171输入的12全新进程、72 pass事件/0skip/0fail；五HTTP行90成功样本；均值×2.4依序97.949/138.487/182.086/192.154/177.087/71.107 s均<300 s；当前SQL正常写入索引配对原样本已核 | 本机估计非CI；写入时间/WAL差含正负噪声，不归因纯id列开销 |
| E41–E43 六增量门禁 | lint/knip、build、vet、server short、tools short、完整page PG包六项通过；short6875pass/1265显式skip、tools27/27、PG256/256 | 只是S2-4增量门禁；作者报告早于root代码提交，提交头gen-check另由E44接受，不是S2-5全门禁 |
| E44 提交头绑定 | S2-4完整提交25f88a2730456940bd52efffd899e8ac827a9f27，3171源freeze、成本/门禁及gen-check原件绑定，root已接受增量 | whole_step_accepted=false；六份root文档仍dirty；S2-5开始不等于完成 |

迁移UNIQUE控制的限制：catalog非唯一断言可捕获错误；当前四列键包含全局唯一UUID id，同hash不同UUID的twins即使UNIQUE也不冲突，不能把不含id的三列形状与当前四列混述，也不能靠twins本身辨别当前索引是否UNIQUE。因此35项最终义务不全部称为产品语义kill；此限制见E02所链初始控制审计。

九控制的catcher分栏：#24为7 last-hop+4 worker后置叶失败，2 before-plan仍pass；advisory后置为真实重复/持久化，advisory移除为真实等待观察；tree额外advisory为5s HTTP进展超时；session为响应201且行数断言短路；#29为seed7测试输入成功覆盖；#43为真实worker+fake底层，另有真实page JobWrites+fake store的不同层；query为形状/实际访问分层。52/43/9是含父测试事件数。

失败历史只记触发与后续同范围重跑：两个probe失败候选、MCP batch缺message夹具、初期random/fixture问题、backfill旧suffix提取失败、HTTP三状态首轮500/移入预算失败都保留原件，不改写成绿色，也不把纯工具错误算产品语义kill。`http-revert-fixed-01`因slash正则未选中目标，其25个其他pass不作为撤销HTTP证据；定向3请求来自E31的fixed-02。全部受影响输入的最终replay另由下节E51收口；旧失败与诊断原件不删除。

## 最终控制复演

本表保留c7冻结输入的124项历史接受。当前159义务及后来SQL/fixture/domain变化的fresh/reuse桥见下文“33b4最终代码、控制与本机验证”，不把旧表冒称新头全部重跑。

以下是c7当时最终124项**验证义务**的实际接受表，输入为c7代码及冻结工作快照 `2c7778103947014c17c2c85289f832ef88d3496922adf14310617efd449e7786`；rich snapshot `0af84148def2361eedd9fcf0de248236e1cf9e42e434889e0c1ddf5e3a2c4836`。历史增量42/54组等各留原义，不与本表相加。[E51]

| 最终组 | 义务类型与边界 | 实际执行 | 接受依据 |
|---|---|---|---|
| S2-1：35 | 35 Go；hash维护、端口、迁移/schema、catalog非唯一、observer与计划策略。unit hash控制不独立证明S2-3 duplicate_of provenance | 35 fresh，105阶段，均0/1/0 | E52：105原Go阶段、123完整断言，1050证据文件SHA；catalog UNIQUE只验flag，含id的twins不能辨UNIQUE |
| S2-2：21 | 20 Go + 1 coverage；有限读者、entry边界与物理计划，另有运行时覆盖义务 | 21 fresh，63阶段；20项0/1/0；coverage Go0/0/0、checker0/1/0 | E53：193完整断言、886证据文件SHA；144计划与273 rich snapshots由独立审核对，原指针STOP保留 |
| S2-3：53 | 50 Go + 3 Web；guard/provenance/snapshot、字段/报告、真实入口与fake契约，层次不合并 | 53 fresh，159阶段：150 Go+9 Web；全部0/1/0 | E54：265完整断言、1599证据文件SHA、689 rich snapshots；操作期限与Web fake异常不冒称全局timeout或生产响应 |
| 后续/审查追加：15 | 14 Go + 1 Web；原九控制、两HTTP仪器控制、旧SetSQL物理控制、B旧值返回、C decoder真实HTTP report、cycle | 12 fresh=36阶段；3 reuse=9阶段绑定；15项均0/1/0 | E55：42 Go+3 Web阶段、226完整断言、789证据文件SHA；复用不是新执行，session错201后DB行数断言未执行 |
| 合计：124 | 119 Go / 1 coverage / 4 Web；并非124独特bug或全为产品语义kill | 121 fresh=363阶段 + 3 reuse=9阶段绑定；123项0/1/0，另1 coverage双口径 | E51绑定四组root接受、全部124 ID/receipt SHA及执行后3171路径/mode/size/SHA恢复 |

三项复用明确为 `http--wrong-analysis`、`http--inspection-only-mode` 和 `s2-5--web-cycle-precheck-omitted`。两HTTP控制只允许两Web文件差异，其余3169输入相同；cycle完整3171快照相同。HTTP仪器mutant在timer前失败、0请求，不能作产品响应反例。A映射既有memo-current-key语义义务；C decoder字面补丁与adapter-only相同但真实HTTP catcher新增，故义务可分别验，不增加独特bug计数。

五条原STOP原样保留：`s2-2--exact-max-premature-complete`、`s2-3--43-worker-duplicate-misclassified-as-conflict`、`s2-3--report-decode-target-drop`、`s2-3--report-encode-target-drop`、`s2-4--session-mode-veto-removed-downgrade-order`。这些是动态地址/ISO时间/UUID匹配差异，独立审与root逐原件接受；实际失败断言采用所有Go `error`/`error-continue`续片重组，Web用实际JSON/failureMessages。首片旧摘要及STOP状态不改为“从未失败”。没有通过重跑或删除历史使其变绿。

全部ID、kind、fresh/reuse、原状态、receipt路径标识和SHA在E51逐项列出，四组完整断言/报告由E52–E55继续绑定。此处是原件接受汇总，不是追加一层产品审查。工作快照只与c7相差六份已接受dirty文档，不能称3171文件全已提交。

## 最终验证与集成

新增Go清单已按immutable 6a4→c7逐函数核为97：89 race eligible（69非bootstrap、20bootstrap）与8 !race；25f→c7所有Go输入相同。C-M1新增3个Web case、修改2个Web文件；B/C后审增加控制义务，不使committed Go测试数变多。未入仓review临时测试单列，不计入97。[E56]

**c7全部本机门禁已接受，保留为修复前历史。** 主持者在c7核完最终124项控制、89项新增race测试各两次、完整非bootstrap/三片bootstrap、31成本/guard、完整Web/E2E及静态/构建/生成/短测，接受范围见E66。复用的旧命令保持原身份，不将其改称在最终工作快照重新执行。执行后全部3171路径、mode、size和SHA及`git diff --check`已核；仅六份已接受文档与c7提交不同。[E66–E71]

| 本机门禁类别 | 实际选择/结果与复用范围 | 原件 |
|---|---|---|
| lint/knip/types、build、vet、gen | Go lint/vet等按所需输入绑定复用，Web修复后有实际lint/knip与build；c7提交头gen-check实际通过，wall2.923 s。Web之后仅既有case标题插值改动，反向字节证明与最终35项/完整cycle triple覆盖该窄差异 | E49/E68/E70；不称所有旧新快照全同 |
| server/tools short与tools race | server8140 run=6875 pass+1265既定short skip，8m限时；tools short27 pass，实际tools race27 pass | E68/E70 |
| 全非bootstrap及新增69第二轮race | 首轮全包7470 pass+1固定Markdown成本skip；第二轮13个focused命令248 run/pass，69新增tops各第二次通过 | E48/E57/E59；第二轮不是第二次全包 |
| bootstrap三片各两轮 | 每轮468 tops+3020纯descendant=3488 run=3484 pass+4固定race skip（其中2个top）；20新增tops每轮各一次。仅既有一测试的3个UUID叶名跨轮变化，6个原名与固定loop位置保留，其余所选名相同 | E63；6 raw/24SHA/12×3171来源图 |
| 全31成本/guard recipes | 25 fresh命令105 run/pass、0skip/fail；6条明确复用此前各两轮共12回执，相关Go输入相同。31个c7成本/guard recipe唯一覆盖、无漏重（含tools，short另1条） | E64/E67 |
| 全Web | 3548测试通过；最后仅既有显示标题改变，完整回执与最终35项及cycle triple按root接受的窄范围延续 | E68/E70，不称重新跑完整Web |
| 全E2E | 108文件312测试/312成功attempt，每项一次，0skip/retry/flaky/unexpected/global error；保留原build/VERSION/全选，只加JSON reporter | E65，wall139.573777 s |

四个bootstrap Markdown race skip在无race下逐叶补齐：page91.48 s、image91.80 s、checked HTML4.11 s、link-bound33.94 s；非bootstrap的`TestTheCostsAreAboutTheSize`也由recipe245实际通过。它们不计作race中已执行。旧目录`whole-test-go-guards-line-303`保留：实际选的是c7行304的Background recipe，当前行303的Revert由03/04复用对覆盖，不能按目录名误算漏测或重测。[E64/E67/E69]

| bootstrap分片 | 两轮Go package Elapsed s → 均值 | 两轮命令wall s → 均值 |
|---|---|---|
| 1（168 tops） | 289.720 / 297.166 → 293.443 | 296.025730 / 297.981815 → 297.003772517 |
| 2（151 tops） | 400.897 / 400.504 → 400.7005 | 401.670684 / 401.308331 → 401.489507546 |
| 3（149 tops） | 327.654 / 327.567 → 327.6105 | 328.510077 / 328.360416 → 328.435246511 |

上表均由真实本机两轮计算，package与wall分列，不是CI Test秒数或CI预测。每轮三片run为743/1883/862、pass为743/1879/862，4个固定skip在第二片。首轮与第二轮名称只对源码规定的3个UUID位置作对齐，不改原日志。[E63]

本轮旧IndexItems 30 ms守卫三状态中位9.633750/12.240333/10.054916 ms，GrepCandidates 600 ms为90.496750/89.577833/95.540458 ms，均未复发；完整grep调用中位214.575292 ms仍在原1 s界。阈值与请求/提交期限不变；这只结束本轮复发核对，长期CI稳定性仍归M10收尾。[E64/E70]


### 两次c7 CI失败、候选诊断与四分片整合

c7精确头的 [40616009992](https://github.com/open-nerve/NerveWiki/actions/runs/40616009992) 与 [40619625333](https://github.com/open-nerve/NerveWiki/actions/runs/40619625333)（各attempt1）均为7 success、guards failure、e2e/image skipped。失败在generic设置下的真实move-out reset：首轮测量move-in200/729.774329ms后reset500/4001.073863ms；第二轮move-in200/797.598895ms后reset500/4000.879315ms，错误均定位到Subtree期限。被计时的move-in成功不能让整行成本通过；后续guard/E2E/image未执行范围保持原记录。CI现场没有查询计划，不能把本机计划写成CI原plan。[E72–E74]

本机run02的真实reset为200/2102.319918ms，未复现CI的500。捕获相同语料的实际DescribeExec SQL与UUID后，用真实PREPARE比较generic/custom：9999父ID的空末层，generic扫110002行、1835.588ms；custom父索引1.305ms。诊断SELECT/EXPLAIN单独30s的上下文不改变真实HTTP4s/commit2s；不得用计划带仪表时长代请求计时。[E74]

候选历史按实际范围保留：裸semijoin虽generic空层17.357ms，但custom引入全表扫描，未采纳[E75]；R闭parent范围run04四核心对通过[E76]，run05却在无统计时选notebook-name部分索引Bitmap，只有notebook在IndexCond，parent范围留Filter；读140118索引项、110006heap行，后续9999叶诊断30s超时，整44对未完成，root已拒绝[E77]。RD仅将deleted_at过滤移到OFFSET0外，保留notebook与parent闭区间，run06四个无统计核心对通过[E78]；run07实际exit0、44对通过，root已核346份artifact SHA/size及44实际plan，原B独立逐字核88份完整gzip结果，正式选择RD并授权产品整合。[E79/E80]

44对是14个无统计core/growth及30个ANALYZE（含语义边界）配对，不是全部语义×所有统计状态的全笛卡尔积。88计划/88有序输出、92实际DescribeExec traces（44RD）保留；44RD访问都把notebook与parent上下界放进IndexCond。空末层9999探查、0index entries/heap rows、29997block访问（非唯一页），外本增长100k后仍然如此。RD不是普遍提速：analyzed/custom空层1.343→8.528ms；analyzed增长后generic原1.267→RD8.107ms。100001行多数父仍全返，最终排序会用15080kB临时盘；非partial索引会读取同父已删条目/旧版本，不能称live-only物理读者。全部这些是树外固定语料实验，不是已接受产品HTTP修复、新预算或CI绿。[E79/E80]

当时新增 **C2-M1**：reset仅检查200，省略reset可能让后续计时变no-op。原C建议Minor，主持者按Important OPEN跟踪的原记录[E86]保留；该项现已在ca8正式修复并经原C真实三状态0/1/0关闭，见下节[E87/E98/E99]。它与Web fake循环C-M1是两个发现，也不证明原产品reset伪成功。

| c7旧三片 | 分支1/分支2 Test s | 均值s |
|---|---|---|
| bootstrap-1 | 367 / 429 | 398 |
| bootstrap-2 | 513 / 598 | 555.5 |
| bootstrap-3 | 476 / 417 | 446.5 |

第二份超450s达105.5s，邻居余量52+3.5=55.5s不足接收；三片理想均分1400/3=466.667s仍过线。按M9§6.5双样本/邻居规则选择第四份，FIRST不变，SECOND取`Every[S-Z]`、`Source`、`TheMCP`、`TheContents`；第三为旧THIRD减SECOND，第四为三者并的补集。c7静态分配168/71/114/115 tops；每轮468顶层+3020纯descendant=3488含顶层事件，4skip包含2top，不能称468race全pass。原C静态核两配置文件补丁通过，现已随ca8逐字节应用并经原C复核；旧身份静态重分和模型不作新四片实测，当时两轮尚待；后来实际八条命令已接受，见F22。配置为8server+web/e2e/image=11项，make race段-j5，新增第四片沿用25m job限时、race15m不变。该阶段31条costs+guards含tools，short另1条，合计32；后来2536另加History recipe，现32条costs+guards、short另1。原C初稿漏tools且含short的31计数由追加更正保留原史。[E81–E85]

SQL/config在ca8实际整合时，后续全部控制、门禁、四片各两轮、受影响成本与精确代码CI尚须刷新；这些后来按下文接受，FIRST名字不变也没有自动复用旧SQL下时间。c7已验124/门禁和6M原件继续保留各自输入与范围，不追改原FAIL，也不将它们合成ca8整步最终绿。

### ca8产品修复与原B/C复核

修复提交`ca8a27dbcd8ad365da2bfac8ef36ed0c50ff8c07`包含9个源码/测试/配置文件，六份root文档未变。实际`ChildrenOfAll`先去重非NULL父UUID，再逐父LATERAL读取notebook相等、parent闭区间，`OFFSET 0`保留边界，live过滤在外；返回原11列并按全层`sort_order,id`排序。内部deleted_at不外泄，无新LIMIT/index/migration、全局planner开关或超时变更。sqlc实际先遇parents，生成`$1=Parents []uuid.UUID`、`$2=NotebookID uuid.UUID`，Query与Store keyed参数映射一致；外部实验参数次序相反，经类型保持的参数置换、空白/注释正规化逐字核相同。[E88/E91/E96]

**普通RD与有限来源读者分开。** RD保护完整普通Subtree，仍有既有深度、缓存和跨父排序语义，可完整返回100001行，不套用来源入口10k合同。另一套有限source-entry reader仍保留内外scalar LIMIT、parent/name_key索引范围、LIMIT之后的notebook filter，以及完整一层才Go排序；其返回/物化界不是RD的普通遍历上限。产品`[]uuid.UUID`不能表达数组内NULL元素；NULL-containing是外部run07 SQL边界证据，不能冒称永久Go接口测试覆盖。[E80/E91/E96]

Tests-first旧ANY实跑red：3个物理工作leaf及父事件失败，无build/setup失败；整合后未过滤page/postgres adapter共267pass，223.795222s，包含原无统计50001节点与多数父20001节点各同连接8读、最长1s界，以及order/depth/cache/有限来源读者。初轮真实HTTP三状态各3次，9move/9reset均成功，60.831449s；移入439.520917–500.438459ms，原1.4s成本界、request4s/commit2s未变。每次reset后独立SQL核live root的parent NULL、原组10000个ID全部live/hash NULL、原100000来源ID/hash摘要不变；观察在计时请求和真实reset之后。初轮HTTP是B2-N1前提增强前的已验输入，后续ca8正式C正向另列，不把它称最终全门禁。[E92–E94/E98]

| 修复控制 | 作者初轮实际结果 | 真实catcher与范围 |
|---|---|---|
| 恢复旧ANY | 0/1/0；7pass→1pass/6fail→7pass | notebook/parent访问范围与物理工作；3个leaf实际失败，父事件另计 |
| 去掉global order | 0/1/0；4pass→top1fail→4pass | 完整Subtree的全层顺序/字段比较；先top失败故mutant子case未执行 |
| 跳过真实reset | 0/1/0；4pass→4fail→4pass | 三状态sample0真实移入后，root live/parent NULL后置断言；测试准备条件控制 |
| inner-live：真实HTTP/no_statistics | 0/1/0；2pass→2fail→2pass | 正向各3move/reset；mutant先成功移入，再reset500并报Subtree期限；是产品4s期限，不是Go或harness总timeout |
| inner-live：原insert-only metadata | **0/0/0真实survivor保留**；各7pass | 8个mutant执行仍用parent范围；不改fixture硬造kill，不推断MVCC/page数量是唯一原因 |

四条控制义务与五组历史triple分列；root绑定111原件/30,384,205bytes、15阶段及各3172输入快照，无skip/build/Go或harness timeout。前三条作者triple与ca8仅children_plan_test.go四行覆盖注释有差异；由旧owned.diff反向重建旧字节，SHA匹配原baseline manifest，这是重建证明，不冒称原时间戳副本。inner-live HTTP与最终源码完整输入一致；survivor不计作kill或129额外通过项。[E95/E102–E104]

**B2-N1 Nit CLOSED。** 原B发现计数上界若track_counts关闭可全0空过；ca8实际backend guard要求on，fresh insert-only且唯一结果fixture另要求entries>=want，原上界不放松，不要求heap fetched>0以保留合法IndexOnly可能。原B在clean ca8以同一单top argv真实0/1/0：7pass→7fail→7pass；四统计/mode叶均实际报track_counts=off wanton，3.798207/3.453546/3.824713s。唯一变异给测试pool加track_counts=off，保留guard，是计数前提控制，不是产品语义kill。此前六top正向各15pass/中间off仅单top的不同selector窗口保持原样，不能称那一轮sameargv闭环；追加最小同argv闭环没有重跑六top成本。B两正向窗口共32份plan实际核父复合IndexCond、mode计数及positive工作，少量其他索引planning buffer访问不抹掉，不称全库只访问父索引。[E87/E96/E97]

**C2-M1 Important CLOSED。** 原C在ca8同一完整move selector实际0/1/0：baseline/restored各4pass、9move+9reset，分别63.155/61.265s；mutant4fail、3move/0reset，44.820s，三状态sample0均在root准备状态断言失败，未进入后续sample。RootLiveOutside=false、GroupNullHashes=1是实际观察；SourceNodes等0为第二条查询尚未执行，不解读为来源被删。每阶段独立随机seed只与自身baseline摘要比较，不要求跨建库ID相等。原C首轮真实baseline全pass后因Go Output分块解析STOP，未执行mutant；旧目录保留，修正外部parser后另目录完成triple，不把STOP变成产品失败或kill。[E87/E98/E99/E105]

原B/C本轮无新增Important/Minor；C2-M1、B2-N1关闭不改变旧B-N1仍OPEN。root独立逐Go输出重组核六同argv阶段与真实失败原因，已接受关闭。Makefile/CI逐字等于已审after：8server+3其他=11项、四片与-j5实际应用；配置运行规则不变；注释已由root更新为本机四片双轮与33b4单分支实测，并独立绑定实际SHA，运行规则与timeout保持。[E87/E96/E98] 后续状态：以上OPEN为ca8修复复核时的原记录；本次实际文档投影后，原B已复核并由主持者接受关闭B-N1，见[F30–F32]。

**已验快门禁与最终执行入口。** candidate01固定3172输入六条：lint/knip11.078s、build3.485s、vet1.776s、server short154.136s（8142run=6875pass+1267既定skip）、tools short0.371s/27pass、全量生成3.135s且rich输入相同；root提交后精确ca8 gen-check另实跑0/3.448434876s、无输入差异。旧c7的97/89清单保留历史；ca8当时静态新增99Go=91race eligible（71nonbootstrap+20bootstrap）+8!race，新增两项是`TestSubtreeChildrenKeepTheirParentRanges`和`TestSubtreeOrdersChildrenAcrossParents`；这是清单更新，不声称91项已各两轮。[E88–E90/E100]

### 33b4最终代码、控制与本机验证

当前代码为 `33b4da5a38bed24db05411e83209d30ef1e17db9`，精确 [CI40718770843](https://github.com/open-nerve/NerveWiki/actions/runs/40718770843) attempt1、push/m10-p2-s2，11作业和必需步骤均成功。正式审查及后续修复复核已完成其产品范围；文档B-N1已在本次实际投影后由原B复核并经主持者接受关闭 [F30–F32]，文档/合并/记录的CI及两个main样本尚待真实发生。以下汇编既有独立审计与主持者接受，不追加一层产品审查。

**ca8→5348：最终本机验证与测试连接观察。** ca8原129义务已实际接受：原124 fresh共372阶段，另5条有身份绑定的reuse，不是129个不同产品缺陷。原九条STOP由完整error/error-continue重组和具体动态字段分类接受，旧记录/matcher保留，不以重跑删除失败。先前成本队列52成功/53尝试，唯一Forget fixture失败；其历史PID身份未知。诊断01未重现，02实见同seed autovacuum，03原guard真实RED、04只排除 `backend_type=autovacuum worker` 后GREEN、05普通client仍拒绝。03/04两容器PID均134不代表同一进程。最小!race helper不等待/kill，只尊重PG18.6 CREATE DATABASE原生锁等待及取消auto worker；无生产SQL/预算/fixture数据改动。[F01]

5348前冻结代码实际完成12条补门禁、6正式pair与四条post-helper新triple；每项0/1/0，旧129中其余125按依赖复用。inner-live-HTTP mutant先计时move-IN200，再reset move-OUT500/internal_error/app deadline，不写成计时移入500或Go总timeout。skip-reset先Fatal，未查询的SourceNodes/Live/Hashes默认0不写成真实清空。31成本/guard义务各有实际pass，主要证明7条post-helper新执行＋24条原接受，141run/pass；旧failedForget与C旧Move baseline不入最终pair。测试在提交前freeze运行，以字节桥接到 `53488fee`，不是提交后再跑。[F01]

**两次Restore CI失败及两条不同SQL修复。** 5348/CI40655454504在Restore generic sample0约4s真实500，前6次成功；056677a4/CI40682540338在sample2为500/4.002043903s，generic0/1为3.039175854/3.805950695s，其他两状态6次成功。两轮后续guard及E2E/image未达；7.6s成本界检查未到，不是放宽预算或Go/job timeout可解决。末尾linking/queue-search标签不证明整个4s耗在哪条SQL，也不声称失败回滚成功。[F02–F05]

0566只改TrashChildrenOfAll源/生成SQL及两测试文件（三个新声明）。按父UUID逐键LATERAL/闭区间、时刻检查置边界外，完整13列/全层顺序/无LIMIT保持。初次RED的pkey规划端点访问假阳性保留；最小oracle保留parent真实entries/scans、heap无Seq/额外fetch与唯一实际父IndexCond，other-index规划读完整报告而不误设0。REDv2为17run/13pass/4fail（3真实物理叶＋父），GREEN17/17。nonexec EXPLAIN的constant/custom计划不校准named generic噪声，不subtract或加魔数。永久fixture analyzed/custom空层1.072ms/1probe→10.854ms/9999probes是代价；另core8全HTTPfixture1.278→7.864ms不可混算。13新控制和旧129中的7项新重放分别接受；后者6语义＋1测试输入覆盖、122复用。[F03/F04/F17/F24]

2536随后只改TrashHistory源/生成literal、两新测试和Make新增recipe。旧simple-IN候选在真实prescreen被拒：21文件、2完整pair＋1仅original完成、13未起；不放宽matcher、不进产品。采纳的node-range候选core16最多20101条，仅证明其当时规模/语义/计划；400k由永久真实Store测试另证。旧SQL40run/38pass/2fail，唯一实际慢叶为analyzed generic 400000计数3.957469127s及其父，计数和32语义case本来正确；新SQL40pass，四种统计/mode均实际Count400000且小于原1s界。custom大数组4.782→54.237、5.074→61.577ms及generic小limit .068→4.453ms的代价保留，不称普遍提速；90197 shared hit/read是访问次数非唯一页。[F05–F08/F25/F26]

History的17条fresh集成＋独立第18条downgrade race已接受：完整adapter非race324、race两轮各284、九bootstrap top共58pass；short8147/6875pass/1272skip；E2E108文件312单次成功、无重试。新增8控制为6 SQL语义＋cached route＋track_counts前提，24进程0/1/0，98完整断言/1584续片/1,493,410 UTF8字节；105份3176 rich图同一。cached route Count400k/参数/PID正确，但ModeArguments/Mode为0仅证缺显式DescribeExec，不冒称捕获实际generic cached plan；track_counts off在Store前失败，Count/Elapsed0、Reads空。原B代码/8控制与G18审已独立接受，173份历史外部CI/预算原件未改，不是173源码。[F06–F08/F18/F19]

**2536 CI失败与命名规划修复。** CI40698884331的Restore9次全200后，Revert三状态均sample0/1成功、sample2 HTTP500约4s；8其他初始job成功，E2E/image跳过。完整本机trace九次200，CPU诊断2.961367210s未复现CI500；whole-process inclusive planner.before约230ms仅为优化线索，不把重叠样本相加。写计划v1因观察器membership-ID/user-ID错误在EXPLAIN前停止，v2是一次analyzed rollback plan，非第三次实际Revert或CI计划。[F09]

33b4仅改 `planner.before` 与两个新增测试文件：只收集moved/restored目标(parent,key)的holder，保留全star ChildrenAdded检查、untouched/content-only候选、同父同key、自身排除及UUID排序/优先级。真实RED完整计划正确但allocation102143超过新25000次数界；GREEN1612次。不是字节或HTTP时长。原审NAME-REV-01 Minor真实发现versions-only sole-holder fixture覆盖缺口；同wrong-version mutant旧0/0/0存活保留，补真实content-only竞争page后0/1/0，精确HolderID错及父事件失败。原review02已关闭，I/M/Nit/Q均0。[F10–F13]

| 当前控制义务 | 类型与实际接受范围 | 当前代码的执行/复用边界 |
|---|---|---|
| 旧129（35/21/53/15原组＋5）[F28/F29] | 124 Go、1coverage、4Web；coverage仍Go0/0/0、checker0/1/0，其他实际0/1/0；完整ID/SHA见原账本 | ca8为124fresh＋5reuse；helper后4fresh/125reuse；TrashChildren后7fresh/122reuse；Name后14直接＋1保守=15fresh/114reuse；逐轮各保留输入绑定 |
| TrashChildren新增13 | 11 query/route＋2 instrumentation，39进程0/1/0；51完整断言、286续片 | 相对0566真实执行，后续未改该SQL/相关oracle；不并入旧129的fresh数 |
| History新增8 | 6 SQL语义＋1 route＋1前提仪器，24进程0/1/0；完整98断言 | 相对2536真实执行，Name无SQL改动；保留原B独审与105输入图 |
| Name新增9 | 完整冲突计划/holder语义与allocation工作界；27进程0/1/0，每进程3top＋8子测试 | final candidate02真实执行；旧版本优先级控制0/0/0是历史，不计新通过项 |
| 合计159义务 | 154 Go、1coverage、4Web；产品语义、物理工作、catalog、fixture/仪器、coverage各有边界 | 不是159独特bug，亦不是159项在33b4提交后全部新跑；未入仓临时review测试另列 |

Name后的旧15组共45进程；跨15项汇总baseline/restored各132run/pass，mutant132run=80fail＋52pass，不是每进程132。195 rich图复原。`internal-target-always-nil`真实kill在unit-baseline，REST Revert叶仍pass，因此归保守正交重放；此前六个正交控制只有fresh正常caller补证，未冒称重跑mutant。[F12]

**最新清单与race。** 以可信main基线2840及逐修复明确声明delta，当前2947、分支新增107=98race＋9!race。ca8的71非bootstrap/20bootstrap两轮由各实际raw绑定，后续TrashChildren三个、History一个、Name三个race声明各有两次真实pass，并按相关输入/调用路径差异刷新门禁；不声称全部98在33b4提交后重新各跑两遍。!race九项有各自实际nonrace成本证明。codegen/文档/测试工具不计新测试声明。[F01/F06/F13/F27]

| 本机验证阶段 | 实际选择/结果；边界 |
|---|---|
| ca8全非bootstrap及focused第二轮 | 7482run/7481pass/1固定Markdown成本skip；13focused命令259run/pass覆盖71新top第二次；其后SQL/domain相关包和caller另按下列阶段fresh |
| 四片各两轮 | 每轮468top/3488事件=3484pass＋4固定Markdown skip；分片168/71/114/115，20新bootstrap top各两次。5个全局Markdown race skip由实际nonrace recipes补齐，不算race中通过 |
| helper后补测 | 12新进程，short8142/6875/1267；312 E2E单次成功及6正式pair；与旧31成本具体义务/4fresh控制组合接受 |
| TrashChildren修复 | 10 fresh集成门禁、完整adapter284非race/249 race两轮、5 caller top/42事件、Restore两轮与legacy bound、312 E2E；原13和旧7控制分别接受 |
| History修复 | 上述17＋独立downgrade第18条；324非race/284 race各两轮、9 caller top/58事件、新400k recipe两轮、Restore/Revert双轮、312 E2E |
| Name最终来源16命令 | lint/knip/build、vet；short8158=6886pass＋1272skip，package91pass＋22无测试skip；app996非race/994 race；16 bootstrap top/92pass；3个legacy recipe为2/2/3pass；Name最终3top两轮各11事件；2 Revert＋2 Background；312 E2E |
| 早期domain桥与Web | Name candidate01 domain563正常/561race；candidate02仅新增priority case，最终short与两轮11事件race补证，不能把563/561改称最终源码重跑。951 Web只是受影响子集；完整Web169文件/3548仍按原输入/窄标题修复范围延续。E2E则每次声明fresh均有全312原件 |
| 最新提交头 | 33b4仅3文件，freeze字节等已提交；提交后gen-check实际exit0/3.240790502s/无漂移，普通push成功；测试是在提交前冻结源码执行 |

Name最终16门禁及两条早期domain由独审核36份内容图3178路径无漂移；rich模式/size恢复另由控制freeze证明，内容图不冒称rich。全部成本义务当前为32条recipe（17costs/15guards，含tools），short另1；History增加1条，原31逐字不变。按每条实际pass与相关输入桥核定，非32个测试或全部在新头重跑。[F01/F08/F13/F20/F21/F22/F23]

| 当前正式成本pair | GoJSON package first/second s | 均值×2.4 s | 接受范围 |
|---|---|---:|---|
| Create | 44.754 / 41.323 | 103.2924 | 保留具体不变路径pair |
| Move | 59.174 / 52.601 | 134.1300 | 保留具体不变路径pair；C旧baseline不入pair |
| Restore | 76.629 / 75.649 | 182.7336 | History后fresh，18HTTP200/max2.011885335s；Name不改该路径 |
| Revert | 92.676 / 92.311 | 221.9844 | Name后fresh，18HTTP200/max3.063165043s |
| Background check/plan | 68.300 / 69.535 | 165.4020 | Name后fresh，18check＋18plan，max2.388390585/2.409432001s |
| Forget | 66.868 / 66.867 | 160.4820 | helper后正式pair，原fixture失败排除 |
| History400k独立recipe | 9.911 / 9.977 | 23.8656 | History后两轮，每轮4次真实Count400000，沿用1s界 |

上表是本机package均值，不是wrapper wall/HTTP/CI；与S2-4早期wall估算分列，不混为同一单位。所有×2.4<300只作容量检查。Revert4s请求/2s提交/7.6s预算、Background5s不变。Background 60k步骤/600名义cut/6000asset元数据只证DB成本，不含资产文件。warm100k索引写对照和6M A/B/独立扫描/DO/third继续按原冻结原件/调用路径范围复用，不声称后来整server二进制逐位同一，也不重跑迁移。[F11/F13]

| 四片本机两轮（ca8受测来源） | package s → 均值 | wrapper wall s → 均值 | 33b4单分支CI Test s |
|---|---|---|---:|
| 1 | 324.647/314.510 →319.5785 | 331.608539/315.442641 →323.525590467 | 444 |
| 2 | 291.092/278.476 →284.784 | 292.003378/279.388048 →285.695712678 | 419 |
| 3 | 241.769/239.186 →240.4775 | 242.759446/240.038256 →241.398850761 | 302 |
| 4 | 263.563/261.482 →262.5225 | 264.458520/262.354546 →263.406532730 | 271 |

本机四片与后续caller补测的输入边界如上；一轮分支Test不替代两次main均值。本机`ok`显示与GoJSON package Elapsed最多4ms差均保留，表按package事件值，不挑小数。四片策略来自c7两次分支CI触发，不把那些失败workflow抹掉。

### 精确代码CI与尚待集成

33b4 CI40718770843全部11作业success：web Test201s，server short187/race692/costs537/guards1127s，bootstrap1–4为444/419/302/271s；E2E步骤231s，image smoke78s。job总时长与步骤时长分开，guards job1146s不是Test1127s。只有这一精确分支样本，不宣称长期稳定性、每个优化的CI因果贡献或main验收。[F14]


实际guards原日志中Restore、Revert各三状态各3请求全200，最大分别2.467645682/3.130127760s；后续Background18调用最大2.418723397s、Forget9请求最大1.432085934s，四个top均有PASS。4s减HTTP实测只是算术余量，不是trace剩余deadline或未来保证。独审只重组所存costs/guards raw，其他job/必需步骤据终态GitHub记录核，不冒称逐叶重新审全CI的Web/race/E2E。[F14]

History专门CI成本四实际Count均400000、Limit400001：无统计generic/custom171.483340/198.331556ms，ANALYZE generic/custom191.590538/158.465408ms，均<原1s；原Store SQL/全部参数/PID/显式DescribeExec已核。独立named PREPARE/非执行EXPLAIN的0→1 generic/custom计数及plan不冒称实际unnamed Store执行计划；大JSON的GitHub时间戳间隔不是请求时长。[F16]

成本collector实际接续2536失败报告，20项旧指标全部在原界内、已查询清单0gap、173历史外部原件未改；新绿不追改旧失败分类。H1 IndexItems30ms/GrepCandidates600ms本轮未复发，长期稳定性仍M10收尾。[F15]

| 尚待事项 | 当前真实状态 |
|---|---|
| B-N1规范文档 | **CLOSED**：root实际应用三处规范后，原B核文档及对应查询源/生成SQL/Store，无新增Important/Minor/Nit/Q；root核原件接受。绑定代码33b4及实际应用字节，未声称文档已提交或main集成完成 [F30–F32] |
| 文档提交/精确CI | 提交号与CI尚未发生，不预填 |
| --no-ff main合并/记录 | merge、record号及各精确CI尚未发生；依workflow记录 |
| 两次main四片Test | 待两份真实main样本逐片配对/均值与余量；不以本机、旧S1或分支样本代替 |
| 后续产品范围 | P2/S3、P3–P5、M10收尾；M7/M9整份open、M8待S3、M6 lint留P3、M5done；不开始M11 |

闭环顺序是实际文档应用→原B复核→文档提交/CI→no-ff main/记录的精确CI与双样本。记录提交自身CI及双main均值可由外部接受回执收口，下次既定S3/Phase文档自然转录，避免为了自写SHA/CI无限追补提交。当前实现/本机验证/代码CI已完成，不再写“124/129复演未做”；S2最终集成尚待上述事项。

## 文档、决定与后续

P2§8.2/§9对应总体12.4/13.1/13.4、00/P1/P2、README升级、M7/M8/M9/M12及本文。M11移交按§9留到P2整体验收；本轮不启动M11。S1旧pending按两真实main与328/313.5/394 s纠正，不作为S2样本。M8在S2/S3验收前open；M7/M9整份open，M6 lint后续，M5done保持，M10进行中。

可以由负责人改判的决定保留“主持者的决定（负责人可以改判）”标签：决定43的名义批/cuts取舍；CPU/数据/统计口径受限的本机成本新界；新增普通SyncRestoreNode一点读的已记录差异。没有用设计豁免替代HTTP三状态义务，也没有把诊断driver池4与官方CLI池10差异隐藏。

## 证据索引

下面是主持者接受记录与原件归档的标识和SHA，不是仓库外文件的永久链接。原件需随项目交接保全；完整命令、叶子断言、采样与失败历史由对应原件和下游SHA绑定。E45–E50记录正式审查与关闭，E51–E62记录最终控制/清单与初期race范围，E63–E71补c7本机门禁的接受；E72–E86记录随后CI失败、候选边界与问题初开；E87–E105记录ca8产品整合、B/C实际关闭与最终执行入口，原旧pending按各自时间范围保留。后续F01–F29接续helper、TrashChildren、History、Name及精确33b4 CI接受；仍待文档/main集成统一见最终验证表。正文实数与环境见[P2 §10.2](../02-P2-source-area.md#102-s2-来源-hash去重与验证)。

| ID | 归档标识 | SHA256 |
|---|---|---|
| E01 | `p2-s1-main-test-pair.json` | `a2fb7008a38d2849908f3a3f7dd42453055efa9d67b62f7d7931b169d05878b3` |
| E02 | `p2-s2-1-root-acceptance.json` | `d8713df99dfc9058203bed52ae65b292f20283fd8f9dbc00692d9febad0a219c` |
| E03 | `p2-s2-2-root-acceptance.json` | `d8c1def6f6402e0ba07fd5f9b3e7b59bbf7f8284627799d245246884b2500a69` |
| E04 | `p2-s2-3-root-acceptance.json` | `3b14a808737dbe51396941292e70ab35b82b2e1c4038788c9ae28b24a74d2f4b` |
| E05 | `p2-s2-4-root-controls-acceptance.json` | `8f1a37e155f5f1e9c052530571a3165ef549bb584e7c24771655e33ae51427cd` |
| E06 | `p2-s2-4-control-independent-audit/report.md` | `99670b9b7e7d9b70b3b2c35c105f48a2edd1ff3e46bcd538de072dc28ffb59bc` |
| E07 | `p2-s2-4-root-probe-first-green.json` | `2999d7c6ab336d2a542d0ba069f2903faf2fe05159505ec13443d0c876a43043` |
| E08 | `p2-s2-4-root-e2e-first-green.json` | `3be9876364c2d694f95d58038e1d1c93d16c3d9f533914a8e54bfb5d41572b43` |
| E09 | `p2-s2-4-root-bootstrap-first-green.json` | `77ef74fa388e507fb206eaf2483665cd0288923ed42b7b7be05ac7f8635cb64f` |
| E10 | `p2-s2-4-root-cost-budget-review.json` | `ee9066d6b16f55ccd0119fc591de5212494dd27ac0743a4e9ef6c0b46cbcc6d5` |
| E11 | `root-x-cost-audit/audit.json` | `3b36b5050eb0a348d74529fdf798d8e2217964cdfc43df8f867a40fd5848da2b` |
| E12 | `p2-s2-4-root-index-first-green.json` | `249438aef19aac95c53d2bd6628721a116e40b031d339a69d9fe25d3774e3170` |
| E13 | `p2-s2-4-root-cost-rows-two-rounds-audit.json` | `fdc78f89a8e253c5fbd0a80668b56ee8716e72fe38128045492e2de55518702b` |
| E14 | `p2-s2-4-migration/runtime-preparation/root-main-A-audit.json` | `889a525ad71f2b4643bfa464782b56589502270a82db721743692cff92d06aea` |
| E15 | `p2-s2-4-migration/runtime-preparation/root-main-A-execution-bindings.json` | `06500544e5d0fb0e0ca9cbf1e40a4130a4054b3f574be6a99b0c79fd27362533` |
| E16 | `p2-s2-4-migration/runtime-preparation/root-independent-scans-A-audit.json` | `8c56a5312845f79bc32e09295543f9f7276f8e902904e77105c95564f7d7f66b` |
| E17 | `p2-s2-4-migration/runtime-preparation/root-roundtrip-audit.json` | `49f7c29cf5c5e9c36f81c3876e5e9d45fea3cd1f2236abb5515619a0136cba57` |
| E18 | `p2-s2-4-migration/runtime-preparation/root-lock-cancel-audit.json` | `ab5ebd4dd501ea1032319a718668930512fe27ea7a6a00f9e06a2e697bba6aea` |
| E19 | `p2-s2-4-migration/runtime-preparation/root-main-B-audit.json` | `f9a0f43a3ccb96f71e0d27080481381009aba58c511a93c094ccd2862591d61d` |
| E20 | `p2-s2-4-migration/runtime-preparation/root-main-B-execution-bindings.json` | `9b6bde77f9acb104711ed971db329d182002eb504f2b623725ef773f1936aa79` |
| E21 | `p2-s2-4-migration/runtime-preparation/root-main-B-storage-summary.json` | `cfe021ac435916b984e7bb494c8def89c2e4c3be8a68c63c132debff855da30f` |
| E22 | `p2-s2-4-migration/runtime-preparation/root-independent-backfill-B-audit.json` | `4c4223a8f85a3c13736c93e7027f0d69d911de6973099f233da61c8009410841` |
| E23 | `p2-s2-4-migration/runtime-preparation/root-owned-B-cleanup-audit.json` | `cac7922b2b36d02a67c7d5aeb9127b4740baafc340c149e19e1cf72b02a43492` |
| E24 | `p2-s2-4-migration/runtime-preparation/root-owned-backfill400-cleanup-audit.json` | `a3f374a303b0f3ae9ffe641429fa62b2497070c77a58b3a9c73d00acb8a4e9a3` |
| E25 | `p2-s2-4-migration/runtime-preparation/root-three-main-B-audit.json` | `74ae7e481dd989abdae8c1d8e9246f90e4ddd7ac371987bc1ec59fca11d3190e` |
| E26 | `p2-s2-4-migration/runtime-preparation/root-three-main-B-storage-summary.json` | `41fbf526803a291feb110966f7a6a9e9ee8b663a5f50ccef45a09bfa80e9cba0` |
| E27 | `p2-s2-4-migration/runtime-preparation/root-owned-third-B-cleanup-audit.json` | `3958eb3515cca24eb163837c6e2dab29284e8efdfbd6c6b49798c6992fd1f646` |
| E28 | `p2-s2-4-root-set-hash-fix-first-green.json` | `a56ce6b2fa778f6c25a05e3562bd0f56a91647b4b31b7dfddc7a8431fdf9957c` |
| E29 | `p2-s2-4-author/http-set-regression-red-01/receipt.json` | `ac3cc484a0a7e7d9c2905837246179ff6a0627e8b8c6a6f3085985bda4125743` |
| E30 | `p2-s2-4-author/http-set-regression-green-01/receipt.json` | `46292490ad20d876db5053ba8964a7854558d17295447c4c6310e7068b1d2706` |
| E31 | `p2-s2-4-author/http-revert-fixed-02/receipt.json` | `3e6d669ef49b7f2b5cdf9605a20be11284f69ac9553e20ac97630daea91eeee6` |
| E32 | `p2-s2-4-author/http-states-samples-01/receipt.json` | `6a6435aea1d169b8d0264e1b6b0f366d08b7fd21e9da75bc87ab29cefc44c45f` |
| E33 | `p2-s2-4-author/http-set-plan-01/receipt.json` | `d27044feb09086a6b2bc6e54f7158281cafa70e07058bf4c3ee09231bebb6f15` |
| E34 | `p2-s2-4-root-http-45-baseline.json` | `67804ae43615664ad861a99811e756cbbd3347e52f7144a8d54288ebc92e111e` |
| E35 | `p2-s2-4-author/http-final-45-samples/summary.json` | `2908cab05a6f686cc39dda1cbdd7914c6143b1d6f593afd4827df51d1a127c85` |
| E36 | `p2-s2-4-author/http-final-45-samples/budget-calibration.json` | `394f5cf74fdc1ee0b1f33e77243879f94f4c2b6d5f39abf2e2dbbd8e55993920` |
| E37 | `p2-s2-4-author/http-final-45-samples/identity.json` | `011fd04038d4984d6525bb6d8f152156f3be1d105223e1e983b788585c0726d1` |
| E38 | `p2-s2-4-root-http-final-controls.json` | `5a069484aecff5854f32878eb7d10a582b5a98549dcdd45e461bc168c05508b6` |
| E39 | `p2-s2-4-root-refreshed-cost-rows.json` | `4e22fe677f05c9e99a2f30b2303b4fae7c738ae82dd5b512786e5f18e465cb37` |
| E40 | `p2-s2-4-author/refreshed-cost-rows-summary.json` | `d629691e3601cdee99b0b3c2147947845d683e7837bcd04cb02470ec4b0d2465` |
| E41 | `p2-s2-4-root-increment-gates.json` | `814eb1b2d89649a16f8f572e581ac57dea99a820b134f6f6d464298012f3c250` |
| E42 | `p2-s2-4-author/final-gates.json` | `fb68fc5e2279170e9392813b4610e43b5ad8b19df556ad337eb0eac956381aee` |
| E43 | `p2-s2-4-author/REPORT.md` | `b76a86e404a3a2fd31d8720901ff335844264e5a9f19a4ac5eb837ad2390dc5f` |
| E44 | `p2-s2-4-root-commit-binding.json` | `be0e1bfca709841c66d1f1d393f491ff0fdb1f95e20205f8e26b000712203116` |
| D01 | `p2-s2-4-doc-projection-preparation.md` | `03f31ced10ab7b4de269a0606f88bf6fe629f19bf6c38d1fe361cde22aa4659c` |
| D02 | `p2-s2-4-migration-prose-draft.md` | `8f8bb51b7b59cc0e938013a9ce5f3a7f79cfb972941e4456e76adca978dd8c1f` |
| D03 | `p2-s2-4-measurement-prose-draft.md` | `e41df37839bf02f3e00f3e774bc18c0a3e4769491e4f33804fa3760159245317` |
| D04 | `p2-s2-4-measurement-prose/source-index.json` | `298161b3bdd2e963904014f74dc2aac854a236d8942c2737afe88962ece5117f` |
| D05 | `p2-s2-4-doc-known-corrections/v2/replacements.json` | `495e123d37ba43a02257148a6b7f5fa1c4dc2213961502c5397305b5671c85e2` |
| D06 | `p2-s2-4-doc-known-corrections/v2/root-apply-receipt.json` | `300b4c368ac951b56ed8d04953fc853a2fbb27c390222180554629faceec88ce` |
| E45 | `p2-s2-root-review-a-acceptance.json` | `1d2171409612855b534adbf33b7b5f44ff038eb0fe65ecae2bb8072e3200bf5f` |
| E46 | `p2-s2-root-review-b-acceptance.json` | `a43e3689bfdcec09fbda0274b7aa9391744b6072e5cc9a75ef89a4fb5dae4a90` |
| E47 | `p2-s2-root-review-c-acceptance.json` | `74bc1ce4d6ffe04f7c59207d81ed32055766030f3ac0658339c22d812271b94c` |
| E48 | `p2-s2-5-root-first-full-race.json` | `2a1e0b62214b76f2d97874d1fd80ecd918d2275887d5cf05ec9791313b694290` |
| E49 | `p2-s2-5-root-c-m1-commit-binding.json` | `6301fb12439a18019b80a8f4b880db0eb6bce1227864e2e6c457eee10a537b9a` |
| E50 | `p2-s2-root-c-m1-closure.json` | `61d43c3093f303b6248eb169008066452e06025e2d176447b46fdf204e2e5667` |
| E51 | `p2-s2-5-root-final-124-controls-acceptance.json` | `359b6cc5096d9fa222315285b2152b03a25eb6fdb1782180dc3ec8cee4baaee8` |
| E52 | `p2-s2-5-root-final-s21-acceptance.json` | `7d92198b7f8c6c24f76749f8916346bdf93aaf4491bb0cfef1adf2150219f7ae` |
| E53 | `p2-s2-5-root-final-s22-acceptance.json` | `a0e3a35e680b1c06788ac5258cb25400d4388d1ae59ae8f4f75550282da3318c` |
| E54 | `p2-s2-5-root-final-s23-acceptance.json` | `d6fe48942a0698c4e6cfa3458da275dd80c772d045f853a1b117389e7f7fede8` |
| E55 | `p2-s2-5-root-final-tail15-acceptance.json` | `d95d824e9522267d2db6046a028a48deb312a906666caa00ac0864fdc966bd7a` |
| E56 | `p2-s2-5-author/go-inventory-c7-binding.json` | `800e9931661bdfb4c51f441098e288eaac0eef5b86fd0195434d73495d748aa0` |
| E57 | `p2-s2-5-root-second69-race-acceptance.json` | `a4f9716bd2dd25ea6511cd5f03a146f1a87647a5d67caf48d7ab23a80b713bda` |
| E58 | `p2-s2-final-gates-independent/bootstrap-first-round-independent.json` | `3537e49ba2fcbcd8280dedbc9c7206a53c4cd29d7ad007472a8b884e37bfd8fe` |
| E59 | `p2-s2-final-gates-independent/new69-independent.json` | `9b6c19d73218235e2d7c6abe60b4de58d1724bf1abe5c42f3b34f7f21ac3b6e3` |
| E60 | `p2-s2-final-gates-independent/bootstrap-1-round-1-independent.json` | `b0b32e4de7f5352620bb70d6972cb2ac6cde688d735364a98cafd9d3becc4a9e` |
| E61 | `p2-s2-final-gates-independent/bootstrap-2-round-1-independent.json` | `db28692da98113635d47ea1d5e5da4f3b98f465349cb81fcba97789b8e4f4fce` |
| E62 | `p2-s2-final-gates-independent/bootstrap-3-round-1-independent.json` | `1db320b6517f5a371c8a174c70b4cdeef94dd0ab4fd9f9e129c4f23a52cbb3fb` |
| E63 | `p2-s2-5-root-bootstrap-two-rounds-acceptance.json` | `6b632a6fdb98653fdecac0ab3b0c9345340c16d391a7294269ccb3a4b738ff21` |
| E64 | `p2-s2-5-root-cost-guards-acceptance.json` | `d55d898a1e4f68f268d898dcead76e4c22d62b2cb6159a0fd8c833253a0c642f` |
| E65 | `p2-s2-5-root-whole-e2e-acceptance.json` | `cdabff770da46e543011ed109c462e6296c470c1f035aa71017510bc7d4454da` |
| E66 | `p2-s2-5-root-local-gates-acceptance.json` | `09b71afb8313ca4681191e4521d641e9704f1b6cf6bb781dadbaa1d7107da945` |
| E67 | `p2-s2-final-gates-independent/final-gates-audit-index.json` | `a418d8ff7e353e46191832487a73572b7f2a11a6649f76b3cf25865b2221074e` |
| E68 | `p2-s2-5-root-gate-reuse-02.json` | `a848d8344fd04939298fc8615401146cdfb9ee336eb87610109cdbca16e9573a` |
| E69 | `p2-s2-5-author/REPORT.md` | `27ea5307a75e1fb0adbfc86cb6d0a0ce98ede5a74d24f0e9aa37e1f7eeb1b544` |
| E70 | `p2-s2-5-author/final-gates.json` | `a1f33f867c5b73cd39207b705b38c0c61c513539072248d17ebf7c573ad0e207` |
| E71 | `p2-s2-5-author/final-source-process-binding.json` | `13f71b3103409365330592460b37a9fae0741b91f65e24b92387548c58388be6` |
| E72 | `p2-s2-root-code-ci-c7-failure.json` | `c78bdefc2b96257616b6aec35d10a2e81c99934d9bddda95fff846da7ce54e7e` |
| E73 | `p2-s2-root-code-ci-c7-baseline2-failure.json` | `cbe98af1352d93b9d83d5995d85d8638ab5dcd69dcc86535e1d9cb7b92b1240c` |
| E74 | `p2-s2-ci-diagnosis/REPORT.md` | `abbf4ecb058d7f869b7fcc0e0227956481e8c4112ca4f471bdf49c5ac1dc1d36` |
| E75 | `p2-s2-ci-diagnosis/CANDIDATE-03.md` | `ef3ef87dc64a9c35d41206f731b834a5139b1130bf8cc4b9f6154e4e0e0a609b` |
| E76 | `p2-s2-ci-diagnosis/CANDIDATE-R-04.md` | `97eaa98ad5dbe157f2b20af2af2969d11df10a1c643a1c1dd47334eafe0754b6` |
| E77 | `p2-s2-ci-diagnosis/CANDIDATE-R-05.md` | `88157a62bfea58614183d58c2943208daffa6aaeb52cf006355ff411a632e01e` |
| E78 | `p2-s2-ci-diagnosis/CANDIDATE-RD-06.md` | `e01515a96317526df9d3e7c4a79d325a7753a7ec81379866705ad1fc8751b991` |
| E79 | `p2-s2-root-rd-experiment-integration-basis.json` | `3e0009cc431c1a251ef69b193ea77a03d29c28b01074749a29e1332b7c2ea5c4` |
| E80 | `p2-s2-ci-diagnosis/CANDIDATE-RD-07.md` | `798f5f7766edca809a9e7e98f1cd79ccdcad88bec3a5347361c6602483253310` |
| E81 | `p2-s2-shard-study-c7/four-shard-proposal-two-branch-samples-v2.json` | `7f6d9f54d4e2512fc326b0569be7a1a1a0ab88ab908a6e8ad3e65c446c24768b` |
| E82 | `p2-s2-shard-study-c7/four-shard-two-file-draft/draft-manifest.json` | `1609fadca5f604cad568323973c0aecbda0fdc6c5e65f83975d61b7a3e76a834` |
| E83 | `p2-s2-shard-study-c7/final-two-CI-reconciliation.json` | `9328a43559e559fa3b98eaf5b2f326919654f3e552ff6af078ef12cf580d18ea` |
| E84 | `p2-s2-review-c/config-draft-review/report.md` | `67f37b04bf409bf16a3eba83712c9b7d9c444b9e7ddb185c2554e37778069386` |
| E85 | `p2-s2-review-c/config-draft-review/recipe-count-clarification.md` | `59c07312b4c5464071d5ae21f542d1755e6fb9c3127742acaab942b90a6d59f6` |
| E86 | `p2-s2-review-c/ci-fix-preparation/C2-M1.md` | `e7f85aca7a0537a0545e82dc8bfcaebe2b7017fdfca350532ee5c6b2b9fb7447` |
| E87 | `p2-s2-root-ca8-bc-fix-check.json` | `621b7e05880647b36284327ee9e716bbfd70ed873d0d87172b99af55dae81195` |
| E88 | `p2-s2-ci-diagnosis/integration-candidate01/REPORT.md` | `bcae7403b045c01de16e4500a3a130a1810726688e54391cb803327926144e57` |
| E89 | `p2-s2-ci-diagnosis/integration-candidate01/quick-gates.json` | `cda3f5e2fa0cf287b6de7d3d9dc7a0814992fcc26bb88193eaf054ab48034be8` |
| E90 | `p2-s2-ci-diagnosis/root-ca8-gencheck01/receipt.json` | `957e360cd728b03973b36b5de8fd65caab845d7540563d4b33d877f49048a9ca` |
| E91 | `p2-s2-ci-diagnosis/generated-rd-mapping.json` | `58e585eb8c781d07c7a47698e4e4fda5951b0083d677e7b944e27d1db82b1ba5` |
| E92 | `p2-s2-ci-diagnosis/integration-red01/receipt.json` | `7982f0fe56d95b1d3b508d27be937911884b9bb3a9cf448adbd544119e6834fe` |
| E93 | `p2-s2-ci-diagnosis/integration-adapter-green01/receipt.json` | `7ca3ef2c05014720f2a09efac8a09fc8a882eb417fc1d2675f8361c321e1e772` |
| E94 | `p2-s2-root-initial-rd-http-check.json` | `83b5b7b5cdd5279eeb2194e3f77fe372dde337a99662f32770c499c9c19c92de` |
| E95 | `p2-s2-root-initial-controls-audit-check.json` | `b1711646a277ff90c4cd82a2edeb104a7566dc5b6d34ff54c0a2feafffcf2da3` |
| E96 | `p2-s2-review-b/product-adapter-final-review/review.txt` | `d86bd45df6ba4f248f63699620c2a9fe8c1562d3e7b14a72380bdce7542bb147` |
| E97 | `p2-s2-review-b/product-adapter-track-counts-same-argv/receipt.json` | `9c5189438d4fec33cec5c37ffe8bfb22203ec54bf1a23a6818767327ffb0abe7` |
| E98 | `p2-s2-review-c/c2-recheck-2/report.md` | `3d451a7ed4b16449eef8d068d062cfdd3ab73aacc9dc7b537212f40bdcf46f73` |
| E99 | `p2-s2-review-c/c2-recheck-2/triple-result.json` | `60b5f9d86ba4cfdb180450cd9f8e281f90a68a059e49e79c8223812f25df5971` |
| E100 | `p2-s2-ci-diagnosis/integration-candidate01/go-inventory.json` | `5e0bead832e3f27a9aa0c2fc1ddd3fb893f73c08faac272f64d4f5863fafc600` |
| E101 | `p2-s2-ci-diagnosis/final-validation/entry.json` | `7c16b892482a67cb225d04f3ee8d134820827cb21719551cd173777ea54b72c2` |
| E102 | `p2-s2-ci-diagnosis/final-validation/reused-controls.json` | `3fbd9e8e8ef1a529f0bd6414a378773f3303f21e642f39b5ea338df2c54622b0` |
| E103 | `p2-s2-ci-diagnosis/integration-test-source-snapshots/receipt.json` | `466ed5522aad30575589c851f6007b76b9a238b302fa202844d4deb5ca4ff609` |
| E104 | `p2-s2-ci-diagnosis/integration-control-inner-live/SURVIVOR-CLASSIFICATION.json` | `b2522227d240088465420eafb8e5db17fc1964030e50f80a779c0a52ddc78833` |
| E105 | `p2-s2-review-c/c2-recheck-1/parser-rejection.json` | `3802832f8abababf0883d5df7193c3846e0f628325c9fad4b7c69c32909066bf` |


第一轮非bootstrap race输入freeze `2532f6c088850e72e54f0cf7d98faf1904d39b6c0b86defee5e441417ad3b519`；receipt `44a1aa61c385c2821d3e18ca98fa43bab036b9246d483126260e3b85c47ae95c`；raw `a50b66fc6a816b3330d087a4766b37539ed30161e2f8ee09a9b4d35e955c490f`，1156.168105 s。只有该已完成范围可标PASS。

| ID | 后续归档标识 | SHA256 |
|---|---|---|
| F01 | `p2-s2-final-ca8-gates-independent/final-post-helper-acceptance/audit.json` | `ab9fc6c195d71416c6e6997314c1b7e6de955eafa25e36c3fccbd068d5a07792` |
| F02 | `p2-s2-5348-failed-ci-cost-supplement.json` | `aa2e996f0046cf5111ec2c9c02b7a5baadb7ec72fe1e9027cb9ecab7bfe7782e` |
| F03 | `p2-s2-restore-query-diagnosis/root-fix-and-new13-independent-acceptance.json` | `dbad8d5605d750cc32e70596d322dd8f4b79acbb50bad5218db935442fb5560f` |
| F04 | `p2-s2-restore-query-diagnosis/root-old-seven-independent-acceptance.json` | `8ca703456102b595eee16ecf16245a8e8226223400b13ead182dbd87ef63228e` |
| F05 | `p2-s2-0566-failed-ci-cost-supplement.json` | `4b4a71c56a93663f3fbe2ad925a561c2fcca617cfaed130ab01fd64e452f35e6` |
| F06 | `p2-s2-restore-query-diagnosis/root-history-original-B-final-acceptance.json` | `7ed73dbc0ece08627c49e14f2c52a695f3f7de5c78231812cad777341de2902a` |
| F07 | `p2-s2-restore-query-diagnosis/root-history-G18-final-integration-acceptance.json` | `7493e01fb0bba813f5323e456ae162a5265599f120e8e6f2528628181224d1f1` |
| F08 | `p2-s2-restore-query-diagnosis/root-history-cost-obligation-inventory-acceptance.json` | `6ea5bb116591ffb7e1a12d1fa8b08538a3abae89dcf312c65cfcf0e6e4f808f1` |
| F09 | `p2-s2-restore-query-diagnosis/root-ci2536-independent-evidence-acceptance.json` | `cf81ae92aaf9fa38e6cc072441e1f84d418c8d25be64ba3e3734d37edef4eb92` |
| F10 | `p2-s2-restore-query-diagnosis/root-name-review-closure-acceptance.json` | `cde1eedb316094aa020d999908734b91830c4bfc88ee1e9ac03d9e477d0b457b` |
| F11 | `p2-s2-restore-query-diagnosis/root-name-performance-acceptance.json` | `de536f777fd1d5572264511d4ccd7d1ed850d69b7efdec959c11081035f4b940` |
| F12 | `p2-s2-restore-query-diagnosis/root-name-old-fifteen-acceptance.json` | `7f43acda2c2f28a28a67f8640019ca95bc3c2e3f1323c2339fa5c8cc0b923ae9` |
| F13 | `p2-s2-restore-query-diagnosis/root-name-final-integration-acceptance.json` | `e179084471c4a9aee1de87de0a30d753d1ceee67d217139491d7d2d5b6e019f3` |
| F14 | `p2-s2-restore-query-diagnosis/root-ci33b4-independent-acceptance.json` | `40c89aa00e304dd4c845d738a98e2d0bedb6522e0c59a61afa67e45bfaee5d0c` |
| F15 | `p2-s2-33b4-code-ci-cost-supplement.json` | `6006461a9d84c71b1d98aef5d3c85707e984b994352fc34ba9923c3c0bfa57a3` |
| F16 | `p2-s2-33b4-history-cost-ci-bounded/manifest.json` | `de7f245a1031d24778ff38a8bf4c97c8265dcd39083d8ea4748e5087a207e743` |
| F17 | `p2-s2-restore-query-diagnosis/root-local-consolidation-acceptance.json` | `3f17d42a4047ac1f7e0c0ff0bc34a566f6142e584a81ce89786cf730e1556210` |
| F18 | `p2-s2-restore-query-diagnosis/root-history-new8-actual-classification.json` | `dda7734c569c4a223d46716683141efe2164ef773fb3b918674e01d131ce07ab` |
| F19 | `p2-s2-restore-query-diagnosis/root-history-final-source-and-protected-evidence-check.json` | `40b9b0f1c8a0c200b9284ca2f214589615a0a84f69d41290a597785c431a9754` |
| F20 | `p2-s2-restore-query-diagnosis/revert-name-gates/commit-receipt.json` | `4a46670f5c5fd5aeac484775e496b20c49e5affef1281d1e527bcb34ff6517e1` |
| F21 | `p2-s2-restore-query-diagnosis/revert-name-gates/postcommit-gen-check/receipt.json` | `f6f2567b933b28bbad642350aa8555c7c3f5b2107701e72e8f4bf28e498671ab` |
| F22 | `p2-s2-final-ca8-gates-independent/bootstrap-eight-two-rounds/audit.json` | `eccf3f320cf37ac36b5c49c162b21c37d6be734461de0902425a32db26b18844` |
| F23 | `p2-s2-final-ca8-gates-independent/new71-two-race-passes/audit.json` | `9696d2cd6b65f381e23e3a2667c98ecccb58e9adcc4c56289acb88818decc129` |
| F24 | `p2-s2-restore-query-diagnosis/root-original-B-code-fix-acceptance.json` | `405c863c31c0ee97cae13211c8f493c8988d06a7a10592ec7df9e991130abf79` |
| F25 | `p2-s2-restore-query-diagnosis/root-history-red-green-independent-acceptance.json` | `77ac153963959296f356ef35a073d98bf140ee58108d39af599b9fa6dca73c59` |
| F26 | `p2-s2-restore-query-diagnosis/root-history-integration-impact-acceptance.json` | `080c1c70c683d86d64e551c207f03480573a0cbf0eee73443a4cb41f29331054` |
| F27 | `p2-s2-restore-query-diagnosis/revert-name-gates/new-name-declarations.json` | `659c426e74d6cefbddb9f349808670ad93645df285e46e8668dab9de1b0e95d8` |
| F28 | `p2-s2-final-ca8-gates-independent/controls129-final-index/audit.json` | `11e8d8a9027074e0f2e3f1eb91ad6bb4534508194c01e769f317020d64782047` |
| F29 | `p2-s2-final-ca8-gates-independent/controls129-final-index/control-ledger.json` | `8b7bac166374b592a056aa01fd2dfc755129d50febcdbd2a9e545015cb073300` |
| F30 | `p2-s2-review-b/b-n1-final07-doc-review/final-review.txt`（原B实际文档复核，CLOSED，无新增发现） | `42f54464de9b02e03100012dd133039bbc8e76ce9486bcf5c96066abea0ff653` |
| F31 | `p2-s2-review-b/b-n1-final07-doc-review/final-review.json`（实际三处文档/代码SHA与历史边界） | `6c5a5b94db25cecfb6b1ea7ae54b93de7737bce8b3b111a1092e897f058f25ba` |
| F32 | `p2-s2-integration/root-B-N1-acceptance.json`（主持者全文核报告及实际3文档/3代码/4原件后接受关闭） | `1995bc47914c8c0c2957c81cb8b5a2acb0513dda9b51a8cfd011209b7ded34f5` |
