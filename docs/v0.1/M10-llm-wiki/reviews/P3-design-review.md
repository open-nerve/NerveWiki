# M10/P3 lint与健康度：设计审查记录

日期：2026-10-29。基线：`f6bd0cdee2b7570252a43581cf71cd1e71592f76`。对象为[P3完整设计](../03-P3-lint-health.md)、[M10总设计](../00-M10-design.md)、[总体设计](../../v0.1-design.md)及P1/P2的设计承接链接。

**结论：设计已审查并由主持者接受，S1/S2待实施。** 原三位独立审查者先完整复核第一轮修订，再核对第二轮的窄差异；所有Important/Minor已在设计层关闭，Nit和Q亦已处置。该设计审查没有新产品实现、构建、数据库、运行时变异、性能或SDK实测。设计提交 `9e38a38f648ffa19130afb8a8b468b792dad1381` 的 [CI 40897835721](https://github.com/open-nerve/NerveWiki/actions/runs/40897835721) attempt1随后因既有M8搜索守卫失败，原失败及未运行项保留。按后续实施决定61的[搜索守卫维护](search-guard-calibration-review.md)，S1入口须由集成该维护后的精确main自身完整CI成功、主持者验收并重新绑定P3放行回执与S1-0候选来满足；不要求或宣称原9e38失败attempt被改写为绿。此处只更新入口条件，以下设计发现、原始哈希、处置及决定49–60不变。

## 输入、独立性与方法

初审A/B/C分别侧重数据与SQL/代价、运行与授权/MCP、网页与测试/提交/文档；使用与主持者相同的模型。各席读取同一冻结输入及实际源码，复核只读自己的旧报告和主持者处置，不读另两席本轮报告或作者私有推理。Round2完整复核Phase与55条old/new；Round3只核本轮窄差异及相邻合同。主持者全文读原件、修订差异和所有拟上位新文字，并核实际MaxKey、解析/保存、事务清理与取消链；静态推演不当作运行复现。

正式Phase相对最终冻结稿只更新当前接受状态和审查链接；SQL、接口、变异、预算及完成标准保持冻结语义。55条上位替换只有三处状态句在正式落库时更新。原始输入、报告及其哈希作为树外证据保留，不把机器路径当仓库依赖。

| 冻结轮次 | Phase SHA256 | manifest SHA256 |
|---|---|---|
| Round1 | `4e870a7b49d757ccc27559c3d01c92d64652000357ee2d47d029c11842f71d69` | `4c5bb1f4e51eb1c9d14a5d4340e4fd7b11df53cb05def03d6868724c290ee627` |
| Round2 | `b9e7758f034cdc239cd96fc26e1f04bb108dbb6f3bc19bbdb723c2bf899ea092` | `424b33eb38c6bef6da03a0d22a50a715c199fe527c33d3ae0075c698e2b014d0` |
| Round3 | `ea546be31147ea9fa62e1b644aa07ef92949687c7c179c957761e267b4d434ac` | `980f019dff452b93197d5db53b1aa657362d6361f3dfc4b7bf61a7514fb73623` |

## 初审发现、主持者处置与原席核对

原席A：3 Important/1 Minor；B：4 Important/2 Minor/1 Q；C：4 Important/1 Minor。原始合计11I/4M/1Q；重复组A-I3/B-I1/C-I3、B-I4/C-I4、B-M1/C-I1保留ID，按较高等级归并为8个独立Important组、3个Minor组、1Q。root增量R-I1另计Important；其设计与双路径测试合同在Round3闭合，不修改原席报告或原始计数。

Round1依据为三份完整原报告及root-revision-round1.md的11组处置+Q、追加R-I1；决定58/59/60分别是Runs收尾、P4旧合格事实、compactNodes presence。root-upper-document-sync-inventory.json仅作漏项清单，实际old从精确源码重新校验；没有把清单当已接受patch。Round2完整读各原席最终报告与root-revision-round2.md：A原4项、B原4I/2M/1Q、C原4I/1M均由原席设计层closed，当时新增A2-I1 Important、A2-N1 Nit、C2-Q1，R-I1整体等待A2-I1补齐；这些增量随后在Round3闭合。关闭轮次如下，不把原席关闭冒称代码、测试、SDK或成本通过。

| 原席ID/等级 | 已核发现 | 作者修订处置与具体章节 | 指定负控/真实catcher（仍待实施） | 当前状态 |
|---|---|---|---|---|
| A-I1 Important | direct SQL漏slot恰一基数，可把dotted冲突两个来源都消化 | §3.8删除相关direct EXISTS；首JSON每页解析一次（长度/值两次DB工作已计）、源/全property_path边先计基数；短heads/512行/8MiB/12B合法slot path，JSON批区间单调不重扫；D/invalid共用SourceSlots，根array与嵌套容器分明 | S47手写Sources数组+dotted键，NULL/区外/asset冲突仍不消化；S55长非slot key/跨行字节批；LQ+LD+LP。不以两套相同错误matcher互比 | Round1修订待复核；Round2 A原席设计层closed，见a-round2.md |
| A-I2 Important | 旧resolved/property_key+首键名丢JSON形状，P4无法判旧合格 | §3.8.1/2.1/8/9；00§4.14/§7/§8 exact补Capture OldQualifiedFacts，Body/FirstSourcesSlot与旧resolved事实；BeforeIngest由unit起点memo逆向重建，最终树不充旧树；未改页近似保留，P4另行批量SQL待兑现 | S48本期拟交付SourceSlots的旧对象→数组与旧数组→数组、dotted冲突手写oracle；P3只验纯资格，P4实际捕获/改树/ingest/成本单列交接，不冒充已测 | Round1修订待复核；Round2 A原席设计层closed，见a-round2.md |
| A-I3 Important | S1-3真授权先于S1-4规则，无法独立绿 | §6.1把ActionLintRead/wiki.Actions/access rule+测试一并移S1-3；HTTP operation/矩阵留S1-4 | S58；AC实际TestTheRuleTableIsTheModulesActions+BL TestLintReadsBothModesWithRealAuthority。各删除一项要真失败，不用allow-all fake | Round1修订待复核；Round2 A原席设计层closed，见a-round2.md |
| A-M1 Minor | wanted ByKeys实际cached，且物化候选祖先 | 决定60；§3.2/3.3/3.6删除WantedKeys/ByKeys，从Planned compactNodes建name_key/kind存在位；原name算Linkable；不把发出者过滤套候选，不改保存热路 | S49的PP核page metadata路由；Round2澄清BC复用摘要成本核生产wanted链/SQL trace、ByKeys回退；S12/13手写/Target.form差分；S32只承诺linking层 | Round1修订待复核；Round2 A原席设计层closed，见a-round2.md |
| B-I1 Important | 同A-I3提交依赖 | 同§6.1及00P3/access/接线片段；内部action不是公开operation | 同S58/AC+BL，每提交真内部授权/全集绿 | Round1修订待复核；Round2 B原席设计层closed，见b-round2.md |
| B-I2 Important | notebook MCP外层Gate先HTTP404，不能全scope cursor优先 | §3.10/3.11/3.13/4.1/5及00接口/MCP表；保留平台Gate，account与gate后用例再cursor→book→rule→mode→limit→busy | S56/BL+MX；不可见本+badcursor在两scope分别HTTP404/tool400；gate后按逐项修复；REST fake不假扮MCP入口 | Round1修订待复核；Round2 B原席设计层closed，见b-round2.md |
| B-I3 Important | 独立worker panic无recover会越过请求边界 | §3.10最外worker recover，snapshot清理后slot归还、身份删除、一次脱敏failure发布；expiry/Stop并发终结受同状态机保护 | S50/WA+BL；controlled panic/followers/cleanup error/expiry，pool归还、同key下一次成功、无原panic日志。不得靠HTTP recover | Round1修订待复核；Round2 B原席设计层closed，见b-round2.md |
| B-I4 Important | S36没有主动取消follower触发 | §7.2新leader/F/G确认join，只cancel F，持worker barrier先见F返回，再放行另两者；§3.10独立caller等待 | S51/WA+BL；S36仅授权复核，leader cancel仍S35；不拿撤权/expiry代主动cancel | Round1修订待复核；Round2 B原席设计层closed，见b-round2.md |
| B-M1 Minor（与C-I1合并按Important） | MX不运行AreShort，放宽测试阈值不是负控 | §7.1 MI精确page/adapter/mcp命令；MX只工具行。S45/S57拆开 | S45删工具行由MX抓；S57真实生产说明垫到SDK3797–4096B、工具仍齐，MI的3796阈值不动；记录非空测试事件 | Round1修订待复核；Round2 B原席设计层closed，见b-round2.md |
| B-M2 Minor | batch文案误成忽略aliases本身 | §3.13改“ignoring intermediate changes, including alias changes”；保留batch起点alias语义，不靠删语义省字 | 原batch行为例保持；真实说明MI≤3796、MX每工具行；静态估计不替代真实SDK实测 | Round1修订待复核；Round2 B原席设计层closed，见b-round2.md |
| B-Q1 Q | shutdown owner/信号/接线未落定 | 决定58，§3.10具体Runs Stop、closed/active/drained、workers包括expired；wireApp同owner；app.run最前defer全出口，app.close在pool前兜底，commit_timeout+5s独立界 | S52 WA Stop并发/排队/重复/超时；S53 BL真run退出、上游newApp ctx alive、pool前清理；删除真正run与close调用分别控制 | Round1修订待复核；Round2 B原席设计层closed，见b-round2.md |
| C-I1 Important | 同B-M1，指定命令漏真实SDK字节守卫 | §7.1/7.3分别MI/MX、生产文本垫长；00阶段表同步 | 同S45/S57，两命令分别真目标非空 | Round1修订待复核；Round2 C原席设计层closed，见c-round2.md |
| C-I2 Important | EW启动旧bin不能观察Web源码变异 | §7.1 EW完整同锁build→sha→定向Playwright，NWIKI_E2E_VERSION与VERSION一致；三轮各重build，不gen/不嵌套heavy | W01/04/05/07；源manifest/Web dist/嵌入产物/bin hash关联，WB组件与EW浏览器证据分账 | Round1修订待复核；Round2 C原席设计层closed，见c-round2.md |
| C-I3 Important | 同A-I3/B-I1独立绿 | §6.1/2.1/8.0及exact阶段/access行 | S58/AC+BL真实授权、不是内部action提前公开 | Round1修订待复核；Round2 C原席设计层closed，见c-round2.md |
| C-I4 Important | 同B-I4缺follower取消 | §3.10/7.2/7.3主动F取消单独场景 | S51；S36授权、S35leader、S37期限、S61cleanup分别计 | Round1修订待复核；Round2 C原席设计层closed，见c-round2.md |
| C-M1 Minor | 文档投影漏总体5.4/sources附件/4.18/P1P2交接 | §2.1/8.0设计语义与8.1/8.2实际完成分层；exact补总体5.4/5.6、00表/接口/扩展/阶段/头/13/P4、P1P2仅合同待实现链接 | 静态逐old恰1/顺序可替；完成须真实S1/S2证据，M6未done，M7/M9仍open，不改历史失败/待验 | Round1修订待复核；Round2 C原席设计层closed，见c-round2.md |
| R-I1 root增量Important | 合法YAML转义可使target>固定5MiB，不能当非法 | §3.6/3.8共同MaxLintPayloadBytes宽界；固定常量由bootstrap传，JSONB指数展开512/node；单大项独占前进；新接口/成本/步骤/exact同步，Round2按A2-I1修读取见证 | S62/LP真实parser→PageFacts→生产保存/读取；手写a+U+2028重复+z/x且源<C、target>C、target_key=x；wanted/source-slot各非空并各自只收紧target检查，JSON宽界不动；未跑真实复现 | Round2 A认可宽界、B确认边界；Round3 A核双路径并关闭R-I1整体设计，C核catcher |

## 第二轮增量与第三轮修订核对

主持者接受以下窄修，原报告和冻结输入保持原样。原A在Round3关闭A2-I1/A2-N1及R-I1整体合同，原C关闭C2-Q1并核对catcher；B核相关运行边界未回归。三席均无新增未关闭Important/Minor，主持者核原件后接受。产品决定仍49–60、变异仍72行、正式投影仍六docs。

| 新ID/等级 | 已核发现或问题 | 本轮修订与具体章节 | 负控/证据义务 | 当前状态 |
|---|---|---|---|---|
| A2-I1 Important | R-I1原单长末段被MaxKey丢Keys，wanted headers排target_key=NULL，不能抓只收紧wanted target界 | §3.8真实parser见证改长前段加短末尾z/x；完整手写target、实际key=x/resolved=NULL，current区外发出者，排除条件明确；§7.3 S62拆两路径独立单改，JSON宽界不动 | LP的legal-escaped-target/wanted及source-slot，各证heads/完整target读非空、wanted pages=1与未解析L项；分别基线/变异/恢复并记录失败阶段，不能更早JSON失败或空集合代证；旧单长段仅MaxKey过滤对照 | Round3原A设计层closed，C核catcher，主持者接受 |
| A2-N1 Nit | §3.6空间汇总仍写无例外一批≤8MiB，漏合法单大项及规范化临时空间 | §3.6汇总同步正常≤8MiB目标批，或一个≤MaxLintPayloadBytes合法大target，并计规范化临时空间 | 与既有§3.6/3.8单大项前进合同静态一致；不新增机制或测试种类，实际峰值仍待原成本测量 | Round3原A确认修订，主持者关闭Nit |
| C2-Q1 Q | 已deadline取消的expired worker省Stop重复cancel且仍正确drain是等价 | §3.10/§7.2/S52明记重复cancel等价，不改独立cleanup；未到期运行/排队取消与expired-cleanup active/drain分开 | WA在原deadline前确认运行/排队ctx取消；另持expired-cleanup barrier，负控只等accepting flights会提前drain/pool close；省重复cancel但保workers/active者不强报kill | Round3原C确认等价及drain合同，主持者关闭Q |

S49是本轮catcher澄清，不虚构新发现ID：PP只证page metadata Planned/自身读取范围；新增BC精确命令选择**原已规划**TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory，!race/非short，复用其一末键×深路径形状。经真实bootstrap wiki Execute摘要与单wanted页，生产compactNodes→presence消费链、非空规则/手写结果、LintNodeMetadata与wanted读trace一起证明；回退LinkTargetsByKeys须让BC失败。种子保存不混进读取trace，最终答出ids的Paths合法读独立识别；不能直调DTO再测试自建presence冒充。§7.1/S49/§7.5与两条必要上位new同步，不另增大suite；仍待实施/测量。

## 新变异与历史映射

保留S01–S46/W01–W10原ID。S35的slot与scope义务移S59/S60，S36主动follower取消移S51，S37cleanup移S61，S45真实字节义务移S57；旧报告仍按原ID阅读，不能删掉未覆盖历史。Round1修订新增S47–S62，现共72行（每行内各单改须分别留证，不能把一半失败算全行通过）。S16的UI由S2 WB承接，S1 fake不算健康页验收。W01若nonce单删在实际生命周期等价，记录等价原因并用真实重进/inflight违约负控，不强报kill。

S32/LP只cover linking Planned；S49的PP只核page metadata，BC另核真实wanted consumer，复用既定成本行。新增S54内层K严格decode→reencode与≤256B，原外层唯一编码不能替代；pgtest.WaitForLockWaits只计全库等待，§7.2补tracer PID和pg_locks/pg_stat_activity目标关联。

## 尚未核实的成本与准入

本轮未做任何新测量。10万页2s/250MiB仅目标；S3历史值、P3入口第二片450s和B/C候选估算保留原来源，未称候选已绿。新增panic/Stop/follower/Gate/slot/presence/合法escape用例使旧20s/35s预算不再足够作未更新计划，暂重估第三≤25s/第四≤45s（只排程包络），对应保守估算438.693/432s；必须全新代码四片各双轮及指定merge/record两mainTest严格<450，原timeout不提高。

必测形状：10万source与大量区外未解析边同时出现的每批参数序列化/分配、MATERIALIZED关系/DBjoin/往返总量；一末键大量深目录候选无祖先读取；SourceArea/compactNodes两份metadata；sources首JSON长度/值两次必要DB工作、大array与同path冲突跨批、长非slot key短头；合法大escape target、JSONB指数及别名最大值；Rows.Close/取消、slotcleanup/Stop drain时间；5000links保存开关、每个索引迁移扫描/大小/SHARE挡写不挡读。generic须真实PREPARE、无统计/ANALYZE及隐本A-B-A；Planned不等于force custom。任何未达都阻断对应实施验收，不伪造数字。

## 本轮静态核验边界

Round1已读完整三份报告、root处置与实际bootstrap app.run/close/wire、page unit/Event/Before事实、linking.Index/store、Target.form/Node.Linkable、Markdown YAML/slots/PageFacts调用链、Makefile build/e2e、现有actions与说明测试包。Round2完整读三份最终报告与root处置，窄核实际Link.Keys/MaxKey→保存key、Target分段、TxManager.end及Go CancelFunc/CancelCause语义、LinkTargetsByKeys真实查询名。本轮仍只做文稿文本/哈希/old唯一性与内存顺序应用核验；实际SQL生成/执行、Go语义/race、SDK字节、浏览器构建与运行均是未来S1/S2义务。

原A/B/C完成Round3窄差异复核，主持者全文读原报告及完整差异后接受设计。这里只关闭静态设计缺口；真实实施、变异、测量与代码三审仍待S1/S2。以下原件保留各轮当时结论，不改写历史。

原报告SHA（冻结报告不改）：

- a-round1.md: `6f6f142e12425f10e161f15546a00b18b0bb0c3e83a06840a76532cd807a504b`
- b-round1.md: `f8a7f239d528a9090d2270f264194533cdf5a18f5f6b56072ecbb78075ed59b9`
- c-round1.md: `6a99a22c81e052579ec911573e5164488abdea60077999937f2de927a1118d4d`

Round2最终原报告SHA（只读、未改）：

- a-round2.md: `9642851d72d748948b6b4b39b99b1a3173f91222d11f6d9c9b8ceebfcadb646e`
- b-round2.md: `50a85f4288ad949db80f8c855160a10fafffe798fe3f384745547754072d54d6`
- c-round2.md: `89f16dba42d24782c8a751de97ae31657ab748e7163d5e8dba88702ddc0dd94e`
- root-revision-round2.md: `dbeed93628d831190250296ef7176b860c8a2271f80194d62cf6f63b6abcbd14`

Round3原席最终报告（主持者已读）：

- a-round3.md: `d95a72440c5265b01860aa2d28e1bd2ea9aa50210a39c6c3d3aed8452f239e30`；设计层增量闭合、无新增未关闭Important/Minor。
- b-round3.md: `994cf763225a18089916976d72b79c1c7507359ded48920adf7a506cfc27cec7`；设计层增量闭合、无新增未关闭Important/Minor。
- c-round3.md: `586b771db2fd421f8e719a5c95ba4c1dd58ea872d96f449444214d29d2ddb803`；设计层增量闭合、无新增未关闭Important/Minor。

## 主持者的决定（负责人可以改判）

连续实施编号49–60接P2的48，不覆盖00总设计的1–40。决定是实施合同，索引/容量候选仍须实测准入。表中主文位置相对[P3设计](../03-P3-lint-health.md)。

| 编号 | 决定与依据 | 主文 |
|---|---|---|
| 49 | wanted有界sqlc短heads+字节前缀取完整target，当前source的canonical seen跨批保留，完整组计数；不相加raw计数，不保存全本source_ids关联。明确修订00原算法/空间表述 | §3.6 |
| 50 | sources部分索引必做；普通(notebook,source,range)复合索引及其他候选只在真实退化、修正、5000links保存代价及迁移扫描测量后批准。不以Planned冒称强制custom | §3.5/7.4 |
| 51 | item_key与每项cursor、列表mode/time/stale、完整身份SHA（内部仍完整键）、invalid_sources L/J标签、2048B游标绑定scope/rule/notebook、Fit0 StartCursor；UUID可选字段用指针 | §3.9/3.12 |
| 52 | 附件任一已消化来源parent即消化；否则由current非空未消化来源parent代表；stale/empty父不吞附件，直接wiki引用优先 | §3.8 |
| 53 | max(1,floor(floor(maxconns/2)/2))名额；LintTimeout=min(10s,R/2)，独立caller期限/交付前再授权；cleanup仍占slot，过期flight脱离可合并map | §3.10 |
| 54 | 第二片实际均值450已触线，选MCP B/C窄移界候选：SECOND的MCP→MCP[C-Z]，旧470分167/70/118/115。完整新布局双轮与指定两main真Test重验；不提高timeout | §7.6 |
| 55 | Web九项：每次含Back/Forward、entry+generation、刷新成功重置/失败显错、局部首新项焦点、busy手动、mode禁More不自动算、当前树/missing保项、abort与迟答guard分证、零规则顺序/锚点 | §4.2 |
| 56 | MCP lint无rule却给limit/cursor不默默忽略；cursor纯格式400，notebook404之后组合参数422；同文案/fake/真实工具测试 | §3.11 |
| 57 | 各规则按自己明确范围豁免；invalid_sources的root/根下AGENTS仍查，真实坏sources报，fenced示例不报；不把真实unresolved sources边从wanted全排 | §3.4/3.8 |
| 58 | wiki唯一Runs owner，独立worker recover；幂等Stop拒新flight/cancel所有worker/有界drain；app.run全出口defer，close在migrator/pool前兜底 | §3.10/7.2；S50/S52/S53 |
| 59 | P4删旧索引前逐页Capture OldQualifiedFacts，旧JSON/解析与unit起点BeforeIngest资格齐全；不靠最终树；未改页一条批量SQL留P4兑现同资格及成本 | §3.8.1；00§4.14/§7/§8；S48及P4交接 |
| 60 | 候选kind presence来自已读且经Planned的compactNodes；原name计算Linkable布尔；删除WantedKeys/ByKeys读取，不改保存热路 | §3.3/3.6；S12/13/S49 |

## 文档落实与下一入口

本次仅六份docs：00、P1、P2、P3、设计审查和总体。唯一旧段、基线哈希、目标不存在与完整diff核对全部通过后才允许应用；先在精确main的干净git archive副本演练并检查相对链接，再到main。P1/P2只补设计链接，M6/M7/M9尚待后续实际承接，不提前标done。

文档自己的精确main CI全11作业及必需步骤接受后，才从main开m10-p3-s1。S1先按决定54测量分片候选，再按P3§6逐提交实现；各Step仍须门禁、变异、真实测量、三位代码审查、原席修复核对、文档、no-ff合并及指定两main容量核验。P4/P5/M10收尾继续原分工，不开始M11。
