# M10/P3/S1（lint服务端与协议）：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支m10-p3-s1；BASE b02b4a0至产品HEAD18b922cf、tree852cf970；16个提交，178文件，+32228/-222 |
| 日期 | 2026-10-31 |
| 当前结论 | 实现、18b完整本地门禁/控制和精确分支13jobs已接受；A/B/C2独立初稿与原件附记均已由主持者接受，无未关闭的已核实Important/Minor/Nit；Q-B1审查疑问已关闭，性能边界留M12。文档及Step集成仍按后续实际门禁验收 |
| 后续 | 文档自身minimum/gen/CI；no-ff合并/record和指定两main五份Test均值；S2尚未释放，P3/M10进行中 |

产品门禁与控制实际输入：18b922cfdec5471de622f1bcce9366c497a519e6。

对应 tree 为 `852cf970297933e5635eb1451424c683410b4958`，分支 `m10-p3-s1` 从 `b02b4a01567945919fa6d0705f911ad22c2ae879` 开出。16个实际提交、178个文件、增加32,228行/删除222行；3,328源文件、251 owned embed及原19工具逐项绑定。当前 native/cost/E2E/controls 的真实执行输入为18b；复用七项minimum保留 `0b983268e0e6c1a2b7c37f068b6d898ca1c6bb94` parent加dirty联合源码身份，与18b提交后gen分开记。它们的源字节、native modes、embed和工具实际等价，不称七项在18b提交后重跑。

文档提交自身的minimum/gen/CI另行绑定；本次归档投影不产生这些结果。

本次文档投影时尚未产生合并/record提交；真实记录见00-M10-design.md第12节。

## 对象、方法与独立性

受审输入为BASE→18b最终产品树，原设计49–60、来源修订62及其审查保留。三名fresh席是A、B、C2，均各在独立worktree从设计、diff和源码先形成初稿，初稿冻结后才接收客观历史argv/input/process/raw导航；本文文档起草者不计审查席。旧作者/修复/专项证据与早先审查只是历史或补充，不能代替本轮三席。

| 席位 | 独立主责与本人实际验证 | 当前结论/接受状态 |
|---|---|---|
| A | runtime、snapshot、独立caller、scope、唯一Runs/Stop与组合根；12个真PG/组合根顶层、5个app顶层；单改最终Paths为Background命中旧路径断言，0/1/0恢复；编译495清单 | I/M/N/Q均0；初稿＋原件附记已由主持者接受 |
| B | linking八规则/source slot/迁移/索引/metadata/TOAST；真PG定向、48个fresh A1计划、独立MVCC与slot冲突探针；单改generated First预筛unresolved，在自制真实探针中0/1/0 | I/M/N0；Q-B1作为已知限制关闭审查疑问，性能边界留M12；初稿＋附记已由主持者接受 |
| C2 | REST/schema/generated消费、两scope MCP、Fit/游标、Web fake/双语与LW8；真实SDK/PG跨协议、自制非空sources探针、122个Web测试、495编译清单；单改Fit取未返回末项游标，具名断言0/1/0 | I/M/N0；初稿及原件附记已由主持者接受，Q1–3证据义务已核，Q4为后续集成边界 |

**读界披露。** 原C读到完整§10.1结果段，因此只作补充，另启C2；原C不算三名fresh之一。C2冻结初稿前严格只读P3第1–1197行（§0–9）；冻结后 `collaboration.list_agents` 被动返回其他席摘要，另存post-draft-status-exposure.json，未用作证据，初稿未改。

B在设计关键词检索时意外多见两行；下列为原工具输出的逐字转录，没有重新检索冒充原件：

```text
1215:| OW | empty property key的page/unresolved/asset三个选中用例及命名/缺路径对照 | 1.794 / 1.736 |
1270:- W/routes/NotebookLayout/NotebookPlaces、search-page/load-more、SessionRoot/RootStore/authMiddleware、P2 nodeTarget/openOf/useNodeNavigate、events handlers/refreshedOnConnect；健康页无现成实现。
```

第1270行带实现状态，不能称纯路径。B未继续读相邻结果或以这两行支持SQL/规则/kill/性能结论；主持者按F09限定接受其独立源码/真PG判断，不宣称“零结果段暴露”。若以后发现结论依赖暴露行须重审。

三席重命令都用仓库heavy.sh、共享mkdir锁、GOFLAGS=-p=1与原timeout，单改按字节备份恢复。审查者自己的抽样控制不计入作者248units/262legs总账。原件审计与重新执行分别记录，不称三席各自重跑了全部Step。

## 审查发现、处置与复核

| 编号 | 级别 | 核实情形/位置 | 处置 |
|---|---|---|---|
| A | — | 没有Important/Minor/Nit/Q；snapshot/Runs/Stop有实际运行与原件审计 | 主持者已接受最终稿，无新增修复轮 |
| Q-B1 | Q，已知物理范围边界 | `page/adapter/postgres/queries/lint_metadata.sql:15`、`lint_metadata.go:14`：MVCC后强制generic扫52000仍正确返回2000 | 决定64：关闭审查疑问，保留原失败与新增auto/custom/generic差异，M12真实数据复核；不改当前SQL假称消除边界 |
| B其他 | — | 没有Important/Minor/Nit；最终附记未新增发现 | 主持者限定接受读界后接受最终稿，无新增修复轮 |
| C2 | — | 独立初稿及最终附记Important/Minor/Nit均为0，全部职责原件已核 | 主持者接受最终稿，无新增修复轮；S52标签误拼按下述root勘误处理 |

B既有partial selector的0/0/0样本确实存活：未解析边在后，未改坏After补读。这不说明整个既有套件无catcher；B自制首边未解析、后边resolved且同slot的真实PG探针，同一single fault使J count应1变0，实际0/1/0、恢复字节相同。C2的单fault把Fit续页游标改为未裁剪页末项，两scope的 `MCP-middle-to-REST` 命中last-emitted cursor断言；A的单fault丢最后Paths snapshot，在真实asset_blobs等待/已提交改名后命中旧路径断言。它们均非编译失败、空选择或timeout计kill。

A最终审计S33–S61相关93个控制/97legs/291进程、825选定区间、minimum/gen八份和16提交身份；B独立抽查职责相关23个控制/81阶段，核28份当前costs-lint原raw及1604条测量、8个完整Up、72个写入腿和18个重建阶段。每席仍保留其抽样限制，不把原件数量当kill总数。B原MVCC失败nodes52000/2082blocks、未成功CTE实验52000/4082blocks及fresh成功2000/41heap blocks均原样保留。

各席没有新增过度设计或具体未核实产品怀疑。Q-B1不是默认模式必现错误；C2初稿索取每commit、CI、控制及容量原件属于证据审计义务，已在最终附记逐项核验并关闭；未来main配对仍不预验。当前产品tree未因本轮fresh审查新增修复；此前性能/测试修正的逐提交与复验在后文分别列出。

C2最终逐原件审计16个提交的112项最低门禁，客户端73controls/76legs/228阶段及职责wiring11controls/11legs/33阶段，合84controls/87legs/261进程；五份双轮、60个实际成本recipe进程、336＋7项E2E、825选定heavy区间与精确13jobs分支CI均按原件核。该范围不冒称独立重审全部248units。主持者分别读完三席全文、核其manifest，并亲自读取各自snapshot、slot冲突和continuation单fault的真实0/1/0与字节恢复；C2 continuation各阶段23个实际节点，fault有7个FAIL终态及两scope的last-emitted具名断言。

**文档核对勘误。** 主持者在本文初稿中补正S29/S54逐行计数的联合映射：S29加上S29.4→Q2--S69-f成为4units/4legs，S29.1本身仍无执行；S54加上canonical/length两个内层故障各WA＋LD成为3units/5legs，含原broad控制一腿。原69行、全局222/229及248/262/786均未改变，没有新增控制或补跑。同时把误写的B-Q1统一为原ID Q-B1；C2冻结附记误引 `FAILED_CONTROL_EVIDENCE_PRESERVED`，原report实际为 `FAILED_CONTROL_PRESERVED`，按F12 root erratum在仓库正文写准确值，外部附记/manifest、原report/raw与分类均不改。决定65按root第二版只澄清costs-lint14条双轮、costs17与guards15各一轮，不新增编号、产品修改或测试。

## 实际交付与设计细化

八规则、完整来源区、唯一 sources slot、正文/空属性区分、完整身份及分页、同快照编排、名额/singleflight/取消清理、两REST和notebook/account两scope MCP均已实现。wiki持有自己的DTO，经bootstrap模块根逐字段转换；Rules.PrepareRules与旧guard的Prepare分开。长TitleKeyContext保持有状态NFC→Fold→NFC与分块取消，不改旧写入热路径。每规则只记专属阶段，共享sources另列，失败/expiry只记实际可观察统计。

每caller有独立授权和期限，成功交付前再次授权。名额为请求半池的 `max(1,floor(floor(max_conns/2)/2))`，运行期限为 `min(10s,request_timeout/2)`（含等名额）；singleflight在名额外。唯一Runs owner负责expiry、一次发布、Stop和等待清理；清理仍占slot，不能承诺在计算预算内结束。effective standard的root为nil；stored standard允许explicit LLM合同与拟议root。快照首条应用SQL重新读模式，所有page/linking端口及最后Paths共享ctx；SourceArea.LintData只在该快照内持有metadata bundle，不跨请求缓存，不在已有事务里嵌套调用。

`lint.read`允许reader以上。摘要的用例次序是notebook404→503；规则页为cursor400→notebook404→rule404→mode409→limit422→503，已知rule的cursor binding在授权前，未知rule只先验cursor形状。notebook MCP保留外层Gate HTTP404；account和通过Gate后的tool-app按相同用例次序。busy沿用Retry-After。summary每规则最多5项，limit默认50、范围1–100，count在10000处表示下界；显示目标截256B但不裁完整身份。item_key绑定完整身份，cursor绑定version/rule/notebook/implicit或explicit scope、整体≤2048B、内部规范key≤256B。私有Fit共同缩预览、Fit0保留StartCursor，真实SDK答复≤49152B，说明≤3796B。

| 阶段 | 实际提交 | 交付或身份说明 |
|---|---|---|
| s1-0 | `89d3645420c7b2399e06765bd410132c6958047a` | 初始四份容量移界 |
| s1-0a | `e5c16c013faf8b5f1d0efbe2edc73919176d3d22` | 空属性来源/Extractor2与既有读者 |
| s1-1 | `92aaf3eb4f3fdd2d948f3daa2e70fa55c14f61f4` | 有界读取及51/52实测准入 |
| s1-2 | `e20b30d9389d5e6b0e45ff0c6a41f391fd5db818` | 八规则与scoped计算 |
| s1-3 | `4dd2ad95bca99045f08989f9c9a5b300b87763b2` | runtime/名额/唯一Runs；已发布缺Co-Authored-By尾注例外 |
| s1-4 | `0e40c357bf9a1bd787439ab44f886066e38c0f2a` | REST/schema/网页类型与fake |
| s1-5 | `38685948e502ea6f51c2463fa8959e715ef28e2b` | 两scope MCP与说明 |
| s1-6 | `7d3fde72104208fc1c65a3ff04653a7f9e68a36b` | 成本/跨协议E2E |
| s1-7-catchers | `2c4de843406f5e57cf2bfe41400d2308dee3c434` | 使边界控制可观察 |
| s1-7-migration-test-fix | `f64a163db7be52a4333977c402681be3fd14cfdb` | 迁移测试限版本50；按编译闭包复用原受测范围 |
| s1-7-ci-fix | `14fe2068fb1f14fc8e43fc309eb64e77f5616f5d` | Count0/五份；本机实际bf8，未发布尾注桥到14fe |
| s1-7-metadata-fix | `3d96b670a6e8a25cd5bdd4a6fd55e84b6fb37196` | 本次snapshot局部metadata bundle |
| s1-7-frontmatter-fix | `fed62c1383730c92dc6ac8d4c59af20b8017a52d` | 有效frontmatter候选过滤 |
| s1-7-tracer-fix | `045970d1f46ee5ffa7605402eeea30dc18527fae` | 失败样本RunFinished清理；本机实际8ba，同tree subject桥到045 |
| s1-7-ordered-fix | `0b983268e0e6c1a2b7c37f068b6d898ca1c6bb94` | 完整候选CTE/结果双排序 |
| s1-7-sources-fix | `18b922cfdec5471de622f1bcce9366c497a519e6` | Heads/Values有界资格与完整关系presence单探测 |

## 三次成本CI失败、窄修复与历史保留

**三次真实 CI 失败与修正。** f64 的 CI41068871923 attempt1 保持失败：100k summary 三样本在原 worker 2s 期限答 busy，后续必要属性 recipe 未执行。bf8 随后的 Count0 修正仅在四个 page 规则完成 Count 错误、数值和 capped 校验后，对合法 0 省去空 Items 查询并保留 ctx 错误；正 count、After、Items 错误不变。app spy 与真实 BC call-count 原实现均有具名红证，同一早退删除在两层各 0/1/0；ctx 单表达式删除为真实 0/0/0 等价，不能称 kill。专项 8 过程保留 f64+三文件、无 race 的真实身份。[E04–E06]

bf8 完整本机验证通过后，仅补未发布提交尾注而成为同 tree/同父的 14fe；这不把 bf8 日志改称 14fe。14fe 的 CI41102139096 仍实际失败：summary 三样本约 2.001s，wanted 三样本成功，recipe14 未运行。完整 Test 原日志与 PID/snapshot 分组解析保留；旧 trace 可能混入前一取消 worker 的后续查询，不能将 SQL+scan 当 PG 纯执行或把被截断统计补成完整成功耗时。[N01]

这次按既有 §3.3 允许的 bundle，3d96 将显式 `page.SourceArea.LintData` 作为 LLM 路径的唯一 metadata 读取：一次完整 live metadata，复用原 strict source 算法产出 Sources 与 Nodes；wiki 只在本次 snapshot 使用局部值，跨根端口逐字段转换并拥有切片/ParentID，无跨请求缓存。standard 继续 Nodes；旧 SourceNodes 仍存在并共享严格成员算法。不读正文，不以 hash/ProbeSources 过滤成员，不截断 10k，不改变 stale、cap、排序、slot 或必要 JSON 合同。[N02/N05]

fed62 只将 current/outside 页面按 FrontmatterValid 分配候选：有效项进 properties，无效项进 frontmatter；SQL 仍核真实资格，坏 FM 的 root/AGENTS 不被统一豁免。该改动省去有效页面在 frontmatter count 中的宽参数工作，不改变规则结果、公开字段、request4s/worker2s 或固定成本 guard。[N03/N06]

8ba 的 tracer 改动属于测试收尾：每个实际样本在原 Execute/CPU/heap 测量和日志之后，以相同 flight 的 RunFinished 完成清理；Fatal 路径也先清理再允许下一次 reset。缺 Created 会停止后续采样。独立 wiki-cleanup 记录 WaitNS，原测量区间、3+3 样本、固定界及运行期限不变。`TestWikiCostSamplesWaitForFinished` 用真实 child 的取消失败和下一 reset 断言覆盖收尾；child 预期 exit1 与 outer PASS 分开，不把它计作第七个 100k 样本。缺 Created 分支仍仅代码检查，不虚构运行证据。[N04/N07/N15–N16]

8ba 的 subject-only metadata 桥已实际成为045970d1f46ee5ffa7605402eeea30dc18527fae；两者同tree1ea/同父fed62，旧8ba执行日志不改称045。045的CI41148286267仍只有costs-lint失败，image/E2E按依赖跳过。原Test步骤三summary均在2s期限失败、三wanted成功；每样本清理先于下一reset，本次没有foreign snapshot混入。已完成的OrphanItems约334ms、MissingTypeCount约431ms、196次空FirstSourceHeads合约342–352ms只是具体截点观测，未捕获的取消SQL/完整summary成功耗时不能推断。[N17]

正常提交0b983268e0e6c1a2b7c37f068b6d898ca1c6bb94先落ORDER窄修复；随后18b922cfdec5471de622f1bcce9366c497a519e6将A/B来源读取组合并真sqlc生成。0b 的两条 Orphan Items 在完整 candidates CTE 内按 id 排序，外层按等值 c.id 排序，保留全部候选、资格、First/After、零 UUID、Count cap 与返回身份；实际计划守卫限制按 take 产生的 PK/anti 探测，不依赖固定节点形状。18b 的 SourceHeads/SourceValues 将 notebook/current/FM 资格放入 requested 每项的 indexed_pages LATERAL LIMIT1，保留首 ASCII 键、首 position、配对精确值和必要 JSON 两读。新 HasSourceProperties 在 SourceValues 第一次 Next 对完整、已拥有且严格排序唯一的 eligible IDs 只探一次：先 partial-index 属性点探测，命中后仍验原 notebook/current/FM；false 仅停止属性流，正文 D 与 pending 继续，true 继续原 512/8MiB 流。错误/取消不得当 absence，无新 p.notebook 条件、索引、缓存或跨 snapshot 复用。 完整候选数组的校验/编码/物化仍计成本；Has 的晚命中或全密集情形不是零成本。ORDER 的尾/空页在旧诊断中有约3–5ms额外排序样本，不能称所有页更快。fresh100k/noStats/ANALYZE/custom/真实generic与隐藏A/B/A是已测边界，不解决既有page旧版本MVCC generic退化。SQL+Scan墙钟不是PG纯CPU，Go TotalAlloc、采样heap和PG/RSS/WAL分开；本段解释修正的来源；当前18b精确分支CI已通过，仍不证明生产环境或指定两次main。[N18–N22]

**历史本机数表，实际运行头仅 bf8。** 以下是已接受的历史对照，不能填进本轮18b的待验栏；wall 均为锁内进程，不含排队。[E01/E08–E15]

| bf8 实际入口 | 历史终态 | 进程秒 |
|---|---|---|
| 全 nonbootstrap race / 受影响13包第二轮 | 115包、7991 PASS+1既有 skip / 1378 PASS | 1131.394 / 225.095 |
| costs / guards | 17 recipes、89 PASS / 15 recipes、57 PASS | 431.474 / 839.168 |
| costs-lint 两轮 | 各14 recipes、974 PASS | 395.587 / 390.336 |
| Web lint / knip / tests | exit0 / exit0 / 179文件3831 PASS | 2.462 / 0.974 / 104.057 |
| E2E / LW8 第二轮 | 336 / 7 PASS，同实际 bf8 binary 与250 assets | 150.336 / 4.155 |
| bootstrap1 / 2 | 167 / 39 顶层；各两轮 | 296.986/296.121；179.325/177.397 |
| bootstrap3 / 4 / 5 | 114 / 122 / 52 顶层；各两轮 | 236.567/237.218；269.995/267.187；236.016/233.355 |

旧bf8五片覆盖494顶层，当前18b为495，不能换头复用。f64测试迁移修正按明确编译闭包复用当时未变范围；bf8生产修正后全部69实际重跑；本轮ORDER/Sources后当前248units完整重放。原阶段每个pending只代表当时记录，不覆盖当前接受。

## 主持者的决定（负责人可以改判）

编号接原来源修订记录的62；以下为主持者已正式分配的实施决定。

**63．模块与运行细化。** 接受当前wiki模块根与DTO/PrepareRules窄端口、effective standard与explicit LLM范围、TitleKeyContext、专属/共享Timing、expiry通知及唯一Runs/Stop细化。每caller保持独立授权/期限；只在真实snapshot中读取page/linking和最后路径，事务清理仍占运行名额。公开八规则语义、旧写入热路径和平台panic回滚合同不变；不提前实现P4的Capture/ingest。 理由：这是把已审设计的运行与模块边界落实到真实组合根；不增加一套并行调度或写时lint。 边界：实现细化若与将来的M11批准流程冲突，需按真实显式scope重新评估，现有调用不得嵌套已有事务。

**64．索引、实际成本与MVCC限界。** 接受迁移51/52及各自完整Up、必要JSON和5000-links写成本的实际证据。性能说明分别列实际物理工作、计划、generic/DescribeExec与冷暖条件，不以SQL片段代完整迁移。保留旧版本MVCC/generic全本扫描失败与未成功实验；B本轮auto/custom/generic探针作为新增边界证据，Q-B1交M12真实数据复核，不为强制generic实验改写当前查询。summary约358MiB TotalAlloc未达最初250MiB目标，固定guards通过另列；不把目标未达抹去或据此提高阈值。 理由：设计§7.4正证只约束新插入fixture；当前结果正确且已有边界已披露，生产计划/停机和写放大须在M12实际数据验证。 边界：有旧版本或不同统计的生产查询仍可能增加扫描、内存与耗时；当前接受不等于生产物理工作始终有界。

**65．三次CI后的窄修复与容量。** 接受当前18b的Count0/context等价化、snapshot局部metadata bundle、有效frontmatter候选过滤、失败样本RunFinished清理、ORDER双排序、Sources Heads/Values有界ip探测和完整关系HasSourceProperties单探测。保持worker2s/request4s、各fixed guard及Go15m不变。五bootstrap分片和独立costs-lint对应当前495顶层167/39/114/123/52；costs-lint的14条recipe按两轮完整本机运行验收，costs的17条及guards的15条各有一轮完整本机运行，另有当前13作业分支CI验收。guard当前Test1479/job1501秒、30分钟job余299秒仅为本次观察；后续仍盯余量，不调高预算。指定merge/record两次main的各bootstrap Test均值必须各小于450秒后才释放S2。 理由：以真实consumer读量和成本失败为触发修复路径与分份，不以重复重试或单机时间替代CI。 边界：完整候选ID数组仍有分配，当前分支单样本不能保证后续runner；main指定配对或固定guard失败必须先处置。

**66．执行身份、反向对照及审查读界。** 保留每个进程原HEAD/tree及raw；七项minimum实际为0b dirty源码，与18b提交后gen及本轮产品门禁分别记录。当前正式69行和补充控制合计248units/262legs/786阶段，825选定heavy区间不重叠；不把限定等价、singleClone存活、正控、expected/child panic和S52缺Test终态的具名断言混称普通kill。原失败/解析拒绝、8ba至045同树桥、已发布4dd尾注缺失例外永久保留，不强改历史。只native第13全nonbootstrap本机命令用perbinary session exec，第14affected保持原argv。三个fresh席以A/B/C2计：原C完整结果段暴露改为补充并另设C2；B初稿前意外两行历史计时/Web状态，保留逐字披露并限定接受其未依赖这些行的独立SQL审查；C2冻结初稿后被动看到其他席摘要独立记录。所有历史原件仅在各初稿冻结后释放。 理由：证据必须可回溯到实际动作，审查独立性按真实上下文披露；针对上下文完整污染补足新席，不能把补充审查充作fresh。 边界：有限控制只证明其具体路径，不能外推通用无害；B读界保留透明限制，未来若发现相关结论依赖暴露行须重审。

## 变异与反向对照

当前统一18b输入的完整本地控制已由主持者接受；原正式行与补充分账如下。正式行仍为S01–S69；149个设计execution units中148映射到实际控制、1项不可独立观察。149、222和248不是同一种计数。S2的W01–W10尚未执行，原79行设计文字保留。

| 范围 | distinct units | legs | baseline/fault/recovery实际阶段 |
|---|---:|---:|---:|
| 原S01–S69控制 | 222 | 229 | 687 |
| 独立补充控制 | 26 | 33 | 99 |
| 合计 | 248 | 262 | 786 |
| owner A | 117 | 125 | 375 |
| owner B | 58 | 61 | 183 |
| owner C | 73 | 76 | 228 |

| 实际逐腿分类 | 原229腿 | 补充33腿 | 全部262腿 |
|---|---:|---:|---:|
| 具名断言捕获 | 217 | 33 | 250 |
| 具名断言后已披露cleanup panic，仍有Test终态 | 2 | 0 | 2 |
| 具名断言后次生worker cleanup panic、Test终态中断 | 1 | 0 | 1 |
| 预期worker panic | 2 | 0 | 2 |
| 专用child panic | 2 | 0 | 2 |
| 限定等价，不计kill | 2 | 0 | 2 |
| 合法scope等价，不计kill | 1 | 0 | 1 |
| singleClone存活，不计kill | 1 | 0 | 1 |
| 所需正控，不计kill | 1 | 0 | 1 |

补充33腿包括原五个故障新增bundle入口腿、六个metadata控制、FM两腿、tracer一腿，以及本轮ORDER四个、Heads/Values两个、HasSourceProperties十二个units（AP01有app与真实body两腿）。总计26个新units，另五腿属于原控制；未新增正式行。S65/S68的A-OS与B-OB在同18b输入使用逐字相同故障，S68 Web仍为三个真实渲染断言。

S29.1的any→all没有独立可观察中间点；S38单删Close在当前pgx耗尽/错误自动Close路径是限定等价；S60只删Contract字段与删RootID值不同，合法scope等价不计kill。S39只删Execute Clone存活，另一个生产clone仍在；返回后alias隔离不证明并发map安全。S50旧first-flight-only catcher、旧Timings copy catcher、原S39越界panic、旧S04数据替身及旧15m S58命令均保留历史，当前补实的catcher另记，不覆盖旧失败或存活。

S52原report仍为 `FAILED_CONTROL_PRESERVED`：baseline/fault/recovery为0/1/0，fault第6行先有“Stop omitted expired worker still in cleanup”的具名Fatalf，第7行才是重复close的worker cleanup panic；两个RUN没有leaf/top Test终态，仅package FAIL。补充接受分类为 `NAMED_ASSERTION_THEN_SECONDARY_WORKER_CLEANUP_PANIC_INTERRUPTED_TEST_TERMINALS`，`ordinary_kill=false`，不虚造Test FAIL，也不只凭panic计kill。S50/S61各另有具名断言后次生cleanup panic且终态仍在；两类必须分开。已过期冗余cancel仍无独立控制。

原工具控制会话55814的exit1、原第13项report及三errors永久保留；获准会话67432仅续发原14–73，共60units/63legs/189阶段，exit0，没有重跑1–13。55814/67432是工具会话ID；真实续行controller PID为2483239。ordinal65完整恢复后因空间暂停477.813802秒，经精确缓存回收后同PID继续66，未重启、未retry、未中断子测试。当前全部源码/native modes、251embed、工具及driver/timer恢复，工作树干净。

825个选定heavy子进程区间实际无重叠：786控制阶段、25条新native命令、8个复用minimum/gen过程、4个完整成本目标、2个E2E过程；成本recipe嵌套于外层目标，不重复计数。旧锁/重叠、GNU mkdir与Ryuk失败历史不回写。这是选定进程范围，不是主机所有活动的证明。

### 原正式69行逐行账

原设计故障/catcher文字来自P3§7.3并保留；下表units取all_declared_controls、execution_units.actual_units及actual_observable_alternative的联合映射去重；S29为4units/4legs，S29.1本身无执行，S54为3units/5legs。跨行引用可以重合，不能将各行简单相加替代上面的全局去重数。各具名故障的实际baseline/fault/recovery argv、源码备份/恢复、raw行与哈希由F02逐项导航。只有“断言”指该腿的具名断言，其余逐类列出。

| 行 | 原设计单改/故障 | 设计catcher | 当前关联units/legs | 当前分类（逐腿） |
|---|---|---|---:|---|
| S01 | SourceArea包含root | TestLintSourceAreaIsComplete/root；PD | 1/1 | 断言×1 |
| S02 | 来源改用source_hash非NULL/ProbeSources枚举 | 同测empty-page、explicit-standard-root、>10k；PD | 2/2 | 断言×2 |
| S03 | 整区截10000或使用SourceHeads含trash | 同测large/trash；PD | 3/3 | 断言×3 |
| S04 | asset以page_bytes=0答/created_at改updated_at | TestLintRules/pending-metadata与TestTheWikiToolsAreRESTs；LQ+BL | 4/5 | 断言×5 |
| S05 | metadata查content列/对每页取正文 | TestLintMetadataReadsNoBody（TOAST实际读取正负控制）；PD | 2/2 | 断言×2 |
| S06 | source/root/AGENTS/folder/stale在count cap之后过滤 | TestLintCountsAndPages：10000个应排项在前、至少7个合格项在后；LQ | 1/1 | 断言×1 |
| S07 | wanted按lastkey合组 | TestLintWantedCanonical/paths-a-x-b-x；LD | 1/1 | 断言×1 |
| S08 | Unicode用lower/漏NFC/full-fold | 同测Straße/STRASSE、组合字、多段；LD | 3/3 | 断言×3 |
| S09 | relative用页自身folder/越根夹到根/保rooted斜线 | 同测nested-folder、above-root、rooted-pair；LD | 3/3 | 断言×3 |
| S10 | raw COUNT页数相加 | TestLintWantedUnion/raw-spellings-on-same-page；LQ | 1/1 | 断言×1 |
| S11 | 同source每批清seen | TestLintEdgeBatches/same-page-across-byte-and-row-cuts；LQ | 1/1 | 断言×1 |
| S12 | 先计Node.js后按asset组减计数 | TestLintWantedUnion/page-and-asset-spellings；LQ | 1/1 | 断言×1 |
| S13 | image/embed带扩展名也当wanted，或所有带点目标都排掉 | TestLintWantedCanonical/kinds-and-extensions、presence原name无扩展名/.hidden/尾点/多点/.md/casefold/全本source-stale候选；LD+LQ | 2/2 | 断言×2 |
| S14 | virtual谓词在standard生效/LLM根规则漏relative绝对化 | TestLintWantedCanonical/virtual-scope；LD | 2/2 | 断言×2 |
| S15 | 代表取过滤前最早边/最短raw | TestLintWantedUnion/representative-after-kind-filter；LQ | 2/2 | 断言×2 |
| S16 | 用显示256B作map/item_key/cursorhash | TestLintCountsAndPages/long-prefix-distinct-groups；S1为LD+LQ，S2再以WB核两个同显示条目，S1 fake不冒称UI验收 | 5/5 | 断言×5 |
| S17 | `:many`全本edges/忽略8MiB前缀/不消费cursor | TestLintEdgeBatches/max-materialized-bytes-and-progress；LQ；成本分配为补充 | 4/5 | 断言×5 |
| S18 | keyset去notebook或tuple界，或每批从首扫 | TestLintPhysicalRanges/first-middle-tail-hidden-A-B-A；LP | 2/2 | 断言×2 |
| S19 | 精确positions数组交叉配对/漏notebook复核 | TestLintEdgeBatches/paired-positions；LQ | 6/6 | 断言×6 |
| S20 | orphans计自己/asset/来源或stale发出边 | TestLintRules/orphans独立四反例；LQ | 4/4 | 断言×4 |
| S21 | 所有规则误用统一root/AGENTS豁免 | TestLintRules/root-agents-scopes；LQ（坏sources报、fenced不报） | 1/1 | 断言×1 |
| S22 | 首type/summary键跳过非string找下一个 | TestLintMissingProperties/first-key-wins；LQ | 2/2 | 断言×2 |
| S23 | jsTrim含0085或漏FEFF | TestLintMissingProperties/trim-codepoints；LQ | 2/2 | 断言×2 |
| S24 | 只看257空白前缀 | TestLintMissingProperties/long-whitespace-then-text；LQ+LP | 2/4 | 断言×4 |
| S25 | sources不用首ASCII键/把非链接strings或nested array/object当link | TestLintSources/first-key-and-json-items；LQ+LD | 5/5 | 断言×5 |
| S26 | actual unresolved sources边一概从wanted删除 | TestLintSources/real-link-and-not-link-boundary；LQ | 1/1 | 断言×1 |
| S27 | L/J同文字合并，或按截断文字去重 | TestLintSources/identity-tags-and-long-values；LQ | 5/5 | 断言×5 |
| S28 | 来源发出边全部丢弃使附件不能跟随 | TestLintSources/attachment-follows-source；LQ | 1/1 | 断言×1 |
| S29 | 多父须全已消化/直接wiki引用不能覆盖/空或stale父吞附件 | TestLintSources/multi-parent-direct-empty-stale；LQ | 4/4 | 断言×4；S29.1本身无执行，仅列可观察替代 |
| S30 | 0字节只豁免有子来源页，或区外空叶当folder | TestLintRules/empty-page-versus-folder；LQ | 2/2 | 断言×2 |
| S31 | 去sources部分索引/first probe不LIMIT1 | TestLintPlans/first-sources-probe（真实SQL计划+读工作）及5000属性无sources反例；LP | 2/2 | 断言×2 |
| S32 | Planned改成默认cached且不覆盖真实generic | TestLintPlans/cache-route和actual PREPARE counters；LP（仅linking，page独立S49/PP；观察实际mode） | 1/1 | 断言×1 |
| S33 | snapshot外运行或Paths端口丢ctx | TestTheLintSnapshotAndRename；BL | 2/2 | 断言×2 |
| S34 | 快照首读不StateOf/排队后不复核mode | TestTheLintQueuedModeIsReadAgain与snapshot trace；BL | 3/3 | 断言×3 |
| S35 | 继承leaderctx（原slot/scope义务拆S59/S60） | TestTheLintFollowersSurviveTheirLeader；BL+WA | 2/2 | 断言×2 |
| S36 | 只授权leader/没有交付前再授权（各单改；原follower取消义务移S51） | TestLintCallersKeepTheirAuthority；BL+WA | 4/4 | 断言×4 |
| S37 | deadline固定大于短合法request或等待不计时（各单改；cleanup移S61） | TestLintTimeoutFollowsTheRequest；WA+BL | 5/5 | 断言×5 |
| S38 | worker error/Scan cancel漏Close/slot归还 | TestLintEdgeBatches/cancel-and-query-error、TestLintBusyReturnsBeforeCleanup；LQ+BL | 3/3 | 限定等价×1；断言×2 |
| S39 | shared结果被Fit原地截/并发写map | TestLintFollowersOwnTheirProjection；WA+WM，race | 3/3 | 断言×2；singleClone存活×1 |
| S40 | REST/MCP wiki注册删掉 | TestTheWikiToolsReachServe、TestTheLintReadsBothModesThroughServe；BL | 2/2 | 断言×2 |
| S41 | rule route加入notTargets跳整条、漏notebook_id列归属 | TestLintMatrixTargets/foreign-notebook-column；MX | 2/2 | 断言×2 |
| S42 | list先limit再notebook/mode/cursor | TestLintJudgesInTurn+HTTP多重无效输入逐次修复；WA+WH | 2/2 | 断言×2 |
| S43 | Fit用rawJSON长度/单规则吃预算/少发最后item cursor | TestLintFitsTheSDK、TestLintSummaryShrinksTogether、TestLintPageResumesMidway；WM | 3/3 | 断言×3 |
| S44 | preview0非空规则next=null或默认零位置 | TestLintSummaryAtZeroPreviewsStartsAtTheFirstItem；WM+WH | 2/2 | 断言×2 |
| S45 | 删除生产内置说明的lint或pending_sources工具行（各单改；原字节义务移S57） | TestTheBuiltInInstructionsNameEveryTool；MX | 2/2 | 断言×2 |
| S46 | 日志带target/key/path或只读前一子测日志 | TestLintLogsItsRunNotItsInputs；WA+BL | 5/5 | 断言×5 |
| S47 | 同path≥2仍合法，或先过滤NULL/区外/asset目标再数唯一性（各单改） | TestLintSources/slot-collision-unfiltered-cardinality；LQ+LD，手写dotted-key oracle与跨批 | 7/7 | 断言×7 |
| S48 | 共用SourceSlots把旧对象{0:string}错误当数组slot（仅改本期已交付纯matcher） | TestLintSources/old-qualified-json-oracle；LD+LQ；P4实际Capture/旧树负控明确留P4 | 2/2 | 断言×2 |
| S49 | page metadata的Planned改cached，或生产wanted consumer重走ByKeys全祖先读（分别单改） | 路由由TestLintMetadataPlans/cache-route的PP抓；consumer由TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory的BC抓，§7.5第2行一末键×深路径、非空wanted及真实query trace；不得用测试自造presence代替 | 2/2 | 断言×2 |
| S50 | worker去recover/重新panic/先发结果再清理/原panic入日志（各单改） | TestLintWorkerRecovers、TestTheLintWorkerRecoversThroughModule；WA+BL，清理错误及expiry独立交错 | 8/8 | child panic×2；worker panic×2；断言×3；断言后cleanup panic/终态在×1 |
| S51 | 已join的follower不监听自己的Done | TestTheLintFollowerCancelsAlone、TestLintFollowerCancelsAlone；BL+WA；F先取消而leader/G仍等 | 2/2 | 断言×2 |
| S52 | Stop不拒新flight/漏cancel尚未到期运行或排队worker/仅按flights等而漏expired-cleanup导致active未清便drain（各单改） | TestLintRunsStop；WA，deadline前取消与expired清理barrier分例；重复Stop及超时正例；已deadline取消者省重复cancel且仍正确drain是等价，不强报kill | 4/4 | 断言×3；断言后cleanup panic/终态缺×1 |
| S53 | 删除真实app.run的Stop defer、或删除close前Stop兜底（各单改） | TestTheLintStopsWhenAppRunEnds/run-exits与construct-close；BL，newApp ctx始终alive，pool前drain | 2/2 | 断言×2 |
| S54 | 只校外层cursor，不校内层K规范编码/256B（各单改） | TestLintCursor/inner-key-canonical；LD+WA，YQ/YR及超长反例 | 3/5 | 断言×5；两内层故障WA/LD共4腿，另broad控制1腿 |
| S55 | slot heads投影任意长property_key/先物化后挑8MiB，或换页前清slot计数 | TestLintSources/short-heads-long-nonslot-key及cross-byte-row-batch、TestLintPhysicalRanges/source-slot-disjoint-batch-ranges；LQ+LP，单JSON合法大项进度 | 4/5 | 断言×5 |
| S56 | 绕过notebook Gate或把account码次序套成入口HTTP404 | TestTheWikiToolsJudgeTheirEntryBeforeArguments；BL+MX，真实两scope多重无效逐次修复 | 2/2 | 断言×2 |
| S57 | 生产内置文字真实SDK垫到3797–4096B，保留全部工具行，测试3796阈值不动 | TestTheBuiltInInstructionsAreShort；MI。MX工具行应仍通过；只放宽测试阈值不是变异证据 | 1/2 | 断言×1；正控×1 |
| S58 | S1-3把Execute授权action改为未登记字面量，或从wiki.Actions/access规则表删除相应行（各单改、保留声明可编译） | TestTheRuleTableIsTheModulesActions及TestLintReadsBothModesWithRealAuthority；AC+BL；真Authorizer到达snapshot | 3/3 | 断言×3 |
| S59 | singleflight移入slot之后 | TestTheLintFollowersSurviveTheirLeader/join-before-slot；BL+WA，单slot下真实3caller加入 | 2/2 | 断言×2 |
| S60 | key遗漏Contract或AreaRootID（各单改） | TestLintScopesNeverShare；BL+WA，implicit与两个不同proposedRoot的显式scope | 5/5 | 限定等价×1；断言×3；scope等价×1 |
| S61 | cleanup前release/过期flight仍可join/旧worker删新flight（各单改） | TestLintBusyReturnsBeforeCleanup；WA+BL，deadline→cleanup→新同key有序 | 6/6 | 断言×5；断言后cleanup panic/终态在×1 |
| S62 | 分别只将wanted或source-slot路径的target读取上界收紧为C=5MiB；JSON宽界不动 | TestLintJSONToast/legal-escaped-target/wanted与source-slot；LP各自基线/单改/恢复。真实parser→PageFacts→保存、手写a+U+2028重复+z/x、target_key=x；两生产heads/完整target均非空且规则确有项，非SQL伪种子或更早JSON失败 | 2/2 | 断言×2 |
| S63 | linksParams仅用Property非空判origin，丢掉Quote（只改writer，不改期待） | TestLintOriginRows/current-format；OS。OF补事实见证，实际写入断言须失败 | 1/1 | 断言×1 |
| S64 | 实际执行的generated InsertLinks只把property CASE改回NULLIF；bool参数保持可编译，不运行gen | TestLintOriginRows/current-format；OS，实际SQL的非NULL空串断言失败 | 1/1 | 断言×1 |
| S65 | domain.Extractor回退1、writer保持新表示 | TestLintOriginRows/current-format及TestTheLintOriginCompatibility/literal-v1；OS+OB。literal 2/current、literal 1/legacy独立，不用current-1 | 2/2 | 断言×2 |
| S66 | D First、D After、F的真正body谓词各自改为COALESCE(property_key,'')=''（三次独立单改） | TestLintSources/origin/d-first、d-after、f分别独立执行下方OQ命令；每次空属性目标会错入D或F，断言exact pending IDs | 3/3 | 断言×3 |
| S67 | F的kind条件排除wikilink；另将link/embed/image各单独排除，共四次；无原kind条件时添加此真实过滤 | TestLintSources/origin/f-kind/wikilink、link、embed、image；OQ-K分别执行。普通body [[asset.pdf]]必须改变非空pending结果 | 4/4 | 断言×4 |
| S68 | PageProperties三个平行SQL投影一致加property_key<>''（一次逻辑故障，数组仍等长）；另独立把Web taking的超长key排除改为同时排除key.length===0 | SQL故障：TestLintOriginProperties/empty-key与TestTheLintOriginProperties/empty-key，OS+OB；Web故障：OW的三个empty property key渲染用例。分别baseline/单改/断言失败/恢复，不用假服务端OW冒充SQL kill | 3/3 | 断言×3 |
| S69 | D First、D After的SQL extractor复核分别移除；F生成父资格关系的有效current门移除，三次单改 | TestLintSources/origin-stale/d-first、d-after、f；OQ-S分别执行。D语句边界负控显式给含literal-v1的候选id关系，固定extractor=2，真实生成SQL须拒旧行；另有真实scope端到端排除，不能把只删冗余门的等价变异伪称kill | 3/3 | 断言×3 |

### 补充控制与新增消费腿

以下31组为26个独立补充units＋五个原故障bundle新腿，共33legs/99阶段，没有新增正式行。FM同fault两腿与AP01两腿不折成单腿；metadata两项原控制重锚仍归原行，不冒称新增。

| owner / unit | 补充类型 | 实际legs | 实际catcher及三阶段退出 |
|---|---|---:|---|
| A / `P1--s01-root` | 原故障新增bundle腿 | 1 | BUNDLE01: TestLintMetadataBundle, TestLintMetadataBundle/complete-live-notebook-and-strict-area（0/1/0） |
| A / `P1--s03-cap` | 原故障新增bundle腿 | 1 | BUNDLE01: TestLintMetadataBundle, TestLintMetadataBundle/complete-hashless-area-past-ten-thousand（0/1/0） |
| A / `P1--s49-page-cached` | 原故障新增bundle腿 | 1 | BUNDLE01: TestLintMetadataBundle, TestLintMetadataBundle/one-planned-read-without-body-toast（0/1/0） |
| A / `GAP--S02-probe-membership` | 原故障新增bundle腿 | 1 | BUNDLE01: TestLintMetadataBundle, TestLintMetadataBundle/complete-hashless-area-past-ten-thousand（0/1/0） |
| A / `GAP--S05-per-page-body` | 原故障新增bundle腿 | 1 | BUNDLE01: TestLintMetadataBundle, TestLintMetadataBundle/one-planned-read-without-body-toast（0/1/0） |
| A / `HEADER--heads-qualified-ip` | 独立补充unit | 1 | A01: TestLintPlans, TestLintPlans/source-heads-bounded-indexed-pages, TestLintPlans/source-heads-bounded-indexed-pages/fresh-100k（0/1/0） |
| A / `HEADER--values-qualified-ip` | 独立补充unit | 1 | A01: TestLintPlans, TestLintPlans/source-heads-bounded-indexed-pages, TestLintPlans/source-heads-bounded-indexed-pages/fresh-100k（0/1/0） |
| A / `ORDER--after-cte` | 独立补充unit | 1 | A01: TestLintPlansRules, TestLintPlansRules/A1, TestLintPlansRules/A1/no-statistics（0/1/0） |
| A / `ORDER--after-result-order` | 独立补充unit | 1 | A01: TestLintPlansRules, TestLintPlansRules/A1, TestLintPlansRules/A1/no-statistics（0/1/0） |
| A / `ORDER--first-cte` | 独立补充unit | 1 | A01: TestLintPlansRules, TestLintPlansRules/A1, TestLintPlansRules/A1/no-statistics（0/1/0） |
| A / `ORDER--first-result-order` | 独立补充unit | 1 | A01: TestLintPlansRules, TestLintPlansRules/A1, TestLintPlansRules/A1/no-statistics（0/1/0） |
| B / `AP01-absence-shortcut` | 独立补充unit | 2 | app: TestLintSourcePresence, TestLintSourcePresence/absence-skips-property-reads（0/1/0）；body-consumer: TestLintSources, TestLintSources/origin, TestLintSources/origin/d-after（0/1/0） |
| B / `AP02-complete-relation` | 独立补充unit | 1 | app: TestLintSourcePresence, TestLintSourcePresence/last-only-is-not-absence（0/1/0） |
| B / `AP03-owned-relation` | 独立补充unit | 1 | app: TestLintSourcePresence, TestLintSourcePresence/complete-owned-relation-once-and-zero（0/1/0） |
| B / `AP04-one-probe` | 独立补充unit | 1 | app: TestLintSourcePresence, TestLintSourcePresence/complete-owned-relation-once-and-zero（0/1/0） |
| B / `AP05-post-probe-cancel` | 独立补充unit | 1 | app: TestLintSourcePresence, TestLintSourcePresence/probe-error-and-post-call-cancellation（0/1/0） |
| B / `AP06-error-not-absence` | 独立补充unit | 1 | app: TestLintSourcePresence, TestLintSourcePresence/probe-error-and-post-call-cancellation（0/1/0） |
| B / `AP07-original-no-FK-semantics` | 独立补充unit | 1 | wrong-property-notebook: TestLintSources, TestLintSources/presence, TestLintSources/presence/exact-qualification-and-no-FK（0/1/0） |
| B / `AP08-current` | 独立补充unit | 1 | stale: TestLintSources, TestLintSources/presence, TestLintSources/presence/exact-qualification-and-no-FK（0/1/0） |
| B / `AP09-frontmatter` | 独立补充unit | 1 | invalid: TestLintSources, TestLintSources/presence, TestLintSources/presence/exact-qualification-and-no-FK（0/1/0） |
| B / `AP10-notebook` | 独立补充unit | 1 | foreign: TestLintSources, TestLintSources/presence, TestLintSources/presence/exact-qualification-and-no-FK（0/1/0） |
| B / `AP11-planned` | 独立补充unit | 1 | planned: TestLintSources, TestLintSources/presence, TestLintSources/presence/complete-1025-last-source-and-planned（0/1/0） |
| B / `AP12-positive-stream` | 独立补充unit | 1 | json-buffer: TestLintSources, TestLintSources/json-buffer-and-complete-eligible-set（0/1/0） |
| B / `FM-known-valid-prefilter` | 独立补充unit | 2 | pg: TestLintFrontmatterEligibility/mixed-standard, TestLintFrontmatterEligibility/all-valid-standard, TestLintFrontmatterEligibility/mixed-llm（0/1/0）；bc: TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory/summary/sample-1, TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory/summary/sample-2, TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory/summary/sample-3（0/1/0） |
| B / `Tracer-finished-before-reset` | 独立补充unit | 1 | child: TestWikiCostSamplesWaitForFinished（0/1/0） |
| C / `META:01-duplicate-metadata-read` | 独立补充unit | 1 | BC: TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory, TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory/summary, TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory/summary/sample-1（0/1/0） |
| C / `META:02-clear-source-last-hop` | 独立补充unit | 1 | BL: TestLintMetadataBundle, TestLintMetadataBundle/real-nonempty-and-next-snapshot（0/1/0） |
| C / `META:03-clear-nodes-last-hop` | 独立补充unit | 1 | BL: TestLintMetadataBundle, TestLintMetadataBundle/real-nonempty-and-next-snapshot（0/1/0） |
| C / `META:04-bootstrap-parent-alias` | 独立补充unit | 1 | BL: TestLintMetadataBundle, TestLintMetadataBundle/compact-projection-owns-parent（0/1/0） |
| C / `META:06-wiki-node-slice-alias` | 独立补充unit | 1 | WA: TestLintSnapshotOrder, TestLintSnapshotOrder/complete-source-and-final-paths（0/1/0） |
| C / `META:07-wiki-parent-alias` | 独立补充unit | 1 | WA: TestLintSnapshotOrder, TestLintSnapshotOrder/complete-source-and-final-paths（0/1/0） |

## 门禁、分份与精确CI

当前18b的完整本地门禁、原生E2E和控制总账已获接受，数值来自实际进程和原始测试事件；下表process均为锁内子进程wall，不含排队，也不是CI Test步骤时间。

| 当前门禁 | 真实终态/覆盖 | 锁内process秒 |
|---|---|---|
| 全nonbootstrap race | 115包，8038 PASS＋1既有Markdown成本SKIP | 1118.231 |
| 受影响13包第二轮 | 1425 PASS，0skip | 222.509 |
| costs | 17 recipes，89 PASS，0skip | 411.941 |
| costs-lint第一/二轮 | 各14 recipes、1047 PASS，0skip | 378.950 / 377.538 |
| guards | 15 recipes，57 PASS，0skip | 829.613 |
| lint-web / knip / test-web | exit0 / exit0 / 179文件3831 PASS，0失败/0skip | 2.619 / 1.053 / 111.497 |
| E2E / LW8第二轮 | 336项E2E测试通过 / 7项通过 | 155.134 / 4.487 |

仅native第13条全nonbootstrap命令使用已接受的per-binary session `-exec`；第14条affected保持原go test argv，没有`-exec`。92条binary trace关联21个Testcontainers包，末次资源观察为空且PID不存在；不据此称上游已修复。E2E实际 `make e2e` 带已接受的只读build捕获prerequisite，一次build后复用同binary/assets/browser；版本明确18b/modified=false、250个Web资产及含.gitkeep的251owned embed，默认2workers、无retry/skip。336是测试项数，不是不同故事数；这不验收S2健康页。

race/nonrace真实编译清单为495/525顶层。当前五份各完整两轮共10条命令，顶层167/39/114/123/52恰覆盖495，没有重复或遗漏；两轮节点/动作按既定UUID子名归一化后匹配，片4每轮保留4个既有条件SKIP。新增!race tracer只归costs-lint第13条。

| bootstrap片 | 顶层 | 两轮process秒 | process均值秒 | Go package均值秒 | 每轮SKIP |
|---|---:|---|---:|---:|---:|
| 1 | 167 | 311.710 / 309.126 | 310.418 | 309.544 | 0 |
| 2 | 39 | 195.656 / 187.509 | 191.582 | 190.697 | 0 |
| 3 | 114 | 248.732 / 241.450 | 245.091 | 244.199 | 0 |
| 4 | 123 | 280.015 / 277.559 | 278.787 | 277.890 | 4 |
| 5 | 52 | 227.324 / 227.886 | 227.605 | 226.728 | 0 |

`make test-go`组合为nonbootstrap加五bootstrap竞态入口 `-j6`，之后costs→costs-lint→guards；short独立。CI为server10加web/e2e/image共13jobs，costs17/costs-lint14/guards15条recipe；静态/gen-check仍只在原costs一次，server/tools保留。Go与job timeout未提高。

精确18b的[分支CI41171083998](https://github.com/open-nerve/NerveWiki/actions/runs/41171083998) attempt1已由主持者核验：全部13jobs及必需步骤success；下表为该一次真实运行。条件性失败注解/报告上传跳过不冒称必需步骤缺失。

| job | Test或主测试步骤秒 | job秒 |
|---|---:|---:|
| server (bootstrap-1) | 336 | 356 |
| server (bootstrap-2) | 198 | 216 |
| server (bootstrap-3) | 372 | 395 |
| server (bootstrap-4) | 386 | 416 |
| server (bootstrap-5) | 288 | 307 |
| server (race) | 745 | 763 |
| server (short) | 213 | 232 |
| server (costs) | 650 | 811 |
| server (costs-lint) | 617 | 641 |
| server (guards) | 1479 | 1501 |
| web | 246 | 278 |
| e2e | 264 | 312 |
| image | 79 | 86 |

guards job在原30分钟预算余299秒，15条实际recipe均以package ok结束；非verbose配方不推断完整Test节点数。该单次分支观察不作硬件或根因归因，也不是指定merge/record两main配对。后两次各bootstrap Test均值必须分别严格<450秒，才释放S2；文档提交与其分支CI也须独立取得真实结果。

三次旧分支失败永久保留：[f64/41068871923](https://github.com/open-nerve/NerveWiki/actions/runs/41068871923)、[14fe/41102139096](https://github.com/open-nerve/NerveWiki/actions/runs/41102139096)、[045/41148286267](https://github.com/open-nerve/NerveWiki/actions/runs/41148286267)，均attempt1。summary撞原2s worker预算；未执行的后续recipe、按依赖跳过的E2E/image和旧trace局限照原件保存，18b通过不将旧run改绿。

## 成本、迁移与物理范围

18b两轮完整costs-lint每条recipe的process均值×2.4最大156.087704524秒≤300秒；这是排程容量换算，不是算法guard的×3校准。每轮100k恰3summary＋3wanted，12次RunFinished清理各独立记账；专用cleanup child的预期exit1与outer PASS不计为额外100k样本。

| 当前18b两轮指标 | 第一轮 | 第二轮 |
|---|---:|---:|
| 100k Module summary wall中位，ms | 895.245 | 873.887 |
| 同summary TotalAlloc中位，MiB | 357.802 | 357.849 |
| 同summary采样heap增量中位，MiB | 125.745 | 123.666 |
| 100k Module wanted wall中位，ms | 341.591 | 334.144 |
| 最慢recipe11 process，s | 64.961 | 65.113 |

原8项Module固定guard全部PASS，worker2s/request4s及门限未变；summary约358MiB TotalAlloc仍未达最初250MiB目标，采样heap不是TotalAlloc或连续RSS。直接linking wanted最坏形状的秒数也不能替代真实Module的2s生命周期证明。

完整仓库内成本表见[M12性能移交](../../M12-release/handoffs/M4-performance.md)：保留314行附录的全部数表及原件导航，包括12个100k样本/12次cleanup、224个component shapes/672样本、8个51/52完整Up、72个单索引5000-links写入腿、18个reindex阶段、必要JSON/TOAST及大target。普通SHARE构建挡写允许读；分别完整Up，不能拿索引片段替代。Go CPU/TotalAlloc/采样heap不含PG CPU/RSS，SQL+Scan是墙钟；warm fixture、cluster WAL、受控barrier不承诺生产停机。

旧版本MVCC/generic物理范围未解决。已保存原失败 `TestLintMetadataPhysicalRanges/ANALYZE/A-restored/generic`：返回2000行但Seq Scan nodes扫52000行、过滤50000行；未成功CTE实验和其他原失败一并保留。B本轮探针auto/custom有界、强制generic仍扫52000，结果始终正确。设计§7.4正证限新插入无旧版本fixture；决定64把生产数据、统计/计划与停机复核留M12，不为强制generic实验改写当前查询。

bf8历史成本身份另存：14条lint recipe双轮均值×2.4最大185.022s；summary wall中位1373.078/1389.919ms、TotalAlloc479.282/478.216MiB、采样heap增量128.682/119.193MiB；直接linking wanted最坏约3.082/3.334s。S1-0a原10,004页代表reindex与本轮相同形状重跑分别记；单本linking事务与独立search≤10000行批次分账，后批失败保留已提交前批，不称整条命令或全实例原子。

| costs-lint recipe | 真实selector | 两轮process秒 | 均值×2.4秒 | 每轮PASS节点 |
|---|---|---|---:|---:|
| 01 | `^TestLintIndexesCostTheirScanAndWrites$` | 47.656 / 47.296 | 113.943 | 6 |
| 02 | `^TestLintSourcesIndexCostsItsNormalWrites$` | 10.990 / 10.844 | 26.201 | 22 |
| 03 | `^TestLintSourcesMigrationCostsEmptySources$` | 14.826 / 15.042 | 35.842 | 1 |
| 04 | `^TestLintSourcesMigrationCostsDenseSources$` | 14.172 / 14.079 | 33.901 | 1 |
| 05 | `^TestLintMigrationFixturesKeepVersionsAndLinkFacts$` | 2.715 / 2.744 | 6.551 | 3 |
| 06 | `^TestLintRangeMigrationCostsEmptySources$` | 32.401 / 32.220 | 77.545 | 1 |
| 07 | `^TestLintRangeMigrationCostsDenseSources$` | 32.922 / 33.009 | 79.117 | 1 |
| 08 | `^TestLintRangeIndexCostsItsNormalWrites$` | 12.654 / 10.776 | 28.115 | 22 |
| 09 | `^TestLint(Plans\|PhysicalRanges\|JSONToast)` | 54.201 / 54.746 | 130.736 | 809 |
| 10 | `^TestLintMetadata(Plans\|PhysicalRanges\|PresenceDoesNotReadAncestors)$` | 10.324 / 10.118 | 24.530 | 18 |
| 11 | `^TestLintRulesOfAHundredThousandPagesTakeTheirTime$/^eight-rules-and-100k-sources$` | 64.961 / 65.113 | 156.088 | 84 |
| 12 | `^TestLintRulesOfAHundredThousandPagesTakeTheirTime$/^(hot-many-spellings-kinds\|unique-long-targets\|relative-deep-folder\|same-last-key-presence\|one-page-10000-edges\|100k-excluded-and-outside)$` | 58.582 / 59.072 | 141.185 | 11 |
| 13 | `^(TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory\|TestWikiCostSamplesWaitForFinished)$` | 13.898 / 13.759 | 33.189 | 10 |
| 14 | `^TestLintPropertiesReadOnlyNecessaryValues$` | 7.545 / 7.600 | 18.174 | 58 |

上述14个完整独立进程每轮run集合非空且相同，每条均值×2.4≤原默认10m一半300s；recipe11/12互斥且合起来覆盖全部七种100k形状，recipe13含真实编排和独立cleanup child，未删样本。8项Module及84个linking固定常量未调整。

## 接受、移交与后续验收

实现和当前本地/分支证据以F01–F04为准；三席最终接受由F08/F11/F12与汇总F13收口，A/B/C2均无未关闭的已核实Important/Minor/Nit，读界与Q-B1限定处置保留。M6六行及注意1–4完成，本次分支文档交付将status改done；M7第5项来源附件分型、M9第8项两工具及内置说明余量完成，但各自P4/P5事项使整份open。M11只交explicit scope/旧版本stale闸门，尚未开始；M12保留MVCC/generic、初始250MiB未达、生产负载/停机及CI余量。P4 KnownAbsent/Ineligible/Known/Unknown、Capture和未改页coverage未实施，不能把旧事实缺失当空union。

文档起草只写树外文件，由主持者按工作流§3.5审全文、在真实git archive干净副本投影后应用。旧56锚全保留；原准备工具漏M6 status，主持者仅加H08成为57锚，13既有目标、四份M6历史只核不改，没有新产品决定。本文自身minimum/gen/CI不借产品18b结果，合并/record与指定两main配对均尚未发生。只有两main各片Test均值严格<450且全部必需作业/步骤通过才释放S2；本记录不启动S2/P4/M11。

## 原件导航与身份

树外原件共同根为 `/home/parallels/.local/state/nervewiki/m10/p3-s1/s1-7/`（记作S17）；下面列相对S17的原件位置与完整SHA256。S1-0a、S1-1等上级路径保留真实位置。正文关键数据与分类在仓库内自包含；这些导航用于追原raw/input/argv，不以哈希存在替代语义审查。E17是旧queued快照，不能替代N01终态失败；各H/N历史接受的适用头不变。

| 键 | 原件（相对S17） | SHA256 |
|---|---|---|
| E01 | `root-post-ci-final-local-gates-acceptance-01.json` | `e6f2cfa19f55e930480f575e859c8d9ff507b8c661737f8c3312c1ca93bc4613` |
| E02 | `unpublished-trailer-amend-01/result.json` | `89c68bf24b775b99f59f6ecf1e19841e186f8b019217f5cc9c480778071c53bc` |
| E03 | `branch-push-14fe-01/result.json` | `e26b29373b285ed3a61e5f8c3431db4fd21159579d1206fb7743d392539b1b7e` |
| E04 | `branch-ci-f64/final-failure-summary-01.json` | `d2283e7ad82460671ef5ddea854f1e26e2c93f125a8341274cc15b90b4ec7fa1` |
| E04 | `ci-cost-investigation-01/analysis-01/manifest.json` | `e06c2309693f2bb4ecb9db1569f5ff8d3b22f23245845df44abd4233353847d0` |
| E05 | `ci-cost-investigation-01/final-01/manifest.json` | `71f722cba9f5b275c234c0bd56e603b7b83fa83e8dfaaabb5ff6d83f01477703` |
| E06 | `root-ci-fix-minimum-gates-acceptance-01.json` | `d0dff3c87b623d4c58a24ca58f4102e34c3459d38795510559aa3faef7ee1085` |
| E07 | `root-post-ci-formal69-final-controls-acceptance-01.json` | `1e12e90dd2595fa45c956faa4af6f926ebe9df1b0ad2f6f9df8a7c3b06e92408` |
| E08 | `post-ci-13-nonbootstrap-race-first-acceptance.json` | `7e67448674f909037f9e1b02b7f20ff54a06595fcb8510354b64a636b62c7802` |
| E08 | `post-ci-14-affected-race-packages-second-acceptance.json` | `07041bd1d7056c403abddfeae926d02a9dc53460b4f9676c3f62f1b62a46ea46` |
| E09 | `post-ci-19-lint-web-acceptance.json` | `9a092cd0a164222410ab786d624c20790d2804b6e8859832e9b9d198683cc2cf` |
| E09 | `post-ci-20-knip-acceptance.json` | `e598dc275fe0fb765e1834e3fbc2fa6fa1431f6a45010e602930a388732c87f6` |
| E09 | `post-ci-21-web-tests-acceptance.json` | `ce692246e8a8660e756a518638161b8232608dd282173755efb7e5c473a935ee` |
| E10 | `root-post-ci-e2e-acceptance-01.json` | `61ab45db24cbbc09d3f28786c59c4799cae62f850c7603f3bcd0233ca5ad1560` |
| E11 | `post-ci-fix-checks-01/01-native-gen-check/result.json` | `75d93c8a0e44729a84dcac98fae305a5d198795e54ab47b0cdfe64529c7d9898` |
| E12 | `root-five-bootstrap-pairs-acceptance-01.json` | `d03a6b81a6b4157d729773fe80f3af72224cbecadd20e7d3d9254ef73e87eb80` |
| E13 | `post-ci-cost-facts-preparation-01/manifest.json` | `5bd4028ac5ecf5fd6789f4cb23b442b0d3e57c9454ca1d0827b5c788d6e059b3` |
| E13 | `post-ci-cost-facts-preparation-01/facts-02.zh.md` | `cbff8f70230c3509c4f5712141728a02ec0714605307d52afe59043689d406b2` |
| E13 | `post-ci-cost-facts-preparation-01/capacity-02.zh.md` | `b31aa4834618b9909ed5fb18eeb9afbf8568bd9ca0b49335356cc0a84b3189fa` |
| E13 | `post-ci-cost-facts-preparation-01/document-values.json` | `7ea3d1659b577bf1b1e67dbeafbf2dff50c24365d7873a3d1103567874792307` |
| E14 | `root-post-ci-lint-cost-capacity-acceptance-01.json` | `a717d84de1a2429c85d40d5fad800e86b61b477af47ddb7ead6b2b3b849f436f` |
| E15 | `/home/parallels/.local/state/nervewiki/m10/p3-s1/s1-6/root-rules-guard-proposal-acceptance.json` | `da474def7a4ed72c6e55d21e77671a5c748b1555518f336d9db3c0101eee58ed` |
| E16 | `/home/parallels/.local/state/nervewiki/m10/p3-s1/s1-1/page-author/known-mvcc-boundary.json` | `3385e8edaf592d8782c42bc43fbc2d352628a49cfbc3061ed3132d657474c631` |
| E17 | `branch-ci-14fe/01/run.json` | `8730fa1b124c282eafa66aa8c835a38746d191ac1c9900a8ebb599a034560f0f` |
| E18 | `m6-closure-preparation-01/manifest.json` | `0a0e1a7f44abad8da4ee3f3a5d3eae545b8443e2446e22f8518e09657888ecc1` |
| E19 | `root-m6-semantic-preparation-review-01.json` | `a42bd467e7bc4fd9504771bdccd2389552fe820c1d3b5318b0b0aa1c755c9414` |
| E20 | `root-m7-m9-handoff-preparation-review-01.json` | `da6244d5dec282ba885a22a02b08e1814ef2e2d2dd8a03a28abb45e362f46901` |
| H01 | `document-preparation-01/manifest.json` | `236f9919b279f4862cb22858fc71c757e7b74d5344056d649a3f87159b130b17` |
| H02 | `doc-preparation-02/manifest.json` | `183c5d62e5ac968ba20dc7f3dd47956d5bf3227d20a7f8a147cd1644d4388eb4` |
| H03 | `/home/parallels/.local/state/nervewiki/m10/p3-s1/s1-0a/root-s1-0a-completion.json` | `daf46bbfc8ecb85b31a02147efe58d8d5fcca2540879af58afb594a8f8ac5de2` |
| H04 | `root-formal69-final-controls-acceptance-01.json` | `ebe1d497b4d740e95ae36d0ea4ecdfbd480662cfde21476dfe16aa00227078f2` |
| H05 | `root-final-local-gates-acceptance-01.json` | `be5b9a6766a544f3f28b1c63ba83d6629cd9fcd812e30e4836feb56e1d3e43d1` |
| H06 | `root-unaffected-evidence-reuse-acceptance-01.json` | `09821db86f5a161f810708bff59285c688cce383699fd74c02ad52fe9f3b943b` |
| N01 | `branch-ci-14fe/costs-lint-failure-analysis-01/analysis.json` | `1a584727704a02c9a45fb570b62bc565982886748d1a061a12b9d17f82a41230` |
| N02 | `metadata-commit-receipt-01.json` | `8ab2113fa6393d563177cf8fdce39fb0e367795eb51d1bbbed1cf2550f2153c8` |
| N03 | `frontmatter-commit-receipt-01.json` | `d33061f30964f647929fd267759802db7c700e82ab226ec3152bac3b031d5ea9` |
| N04 | `tracer-commit-receipt-01.json` | `4ad3d189e772dd6bb7af68f93eca7d6ac6728eaab4129d91bfcb67e4d7889d33` |
| N05 | `root-metadata-author-controls-acceptance-01.json` | `1113cc5a8eafb538254debf2d8ffb36e3b225fade685aed36972297f72e2c7ba` |
| N06 | `root-frontmatter-author-acceptance-01.json` | `6438802642a8e5c383f614f6482621c9d727dcfe060a7c3a0d780c7800907b47` |
| N07 | `root-tracer-author-acceptance-01.json` | `827bf9c189b5beab848e5250af65dde82adc96ffe27919ec71e4cfa2ed734c39` |
| N08 | `ci-cost-investigation-02/metadata-implementation-01/control-mapping-clarification.json` | `d6bd8b8458f9e2ae860c62f45d5787b8e0ea994473a2e213e60bae09ceaf677b` |
| N09 | `post-metadata-native-gates-01/00-native-gen-check/result.json` | `1a8b64ed8439770f5648e186df1491e92757d6f585460f56484be55acf346a02` |
| N10 | `post-metadata-native-gates-01/00-native-gen-check/inputs-after.json` | `a575f9e385b7a7fceccf4fe749736a84a910a094b349e9fc35a7a0259515e87d` |
| N11 | `post-metadata-final-gates-binding-01/projection.json` | `6add76d1ceade0e944c89b926aebc92f5daef721b38b23b2eb5ba9ec73f59f1b` |
| N12 | `post-metadata-controls-preparation-03/core/manifest.json` | `5e9ccc110fb33b6a98dcabd850be6d8125ee003e4dfd85529452c448a5c1c020` |
| N13 | `post-metadata-controls-01/core-binding/binding.json` | `a96236b73f9bea14057f34e67ad451d52d4c82c1f51b00d5c0515fd329a54a49` |
| N14 | `post-metadata-controls-01/root-preparation/worktrees.json` | `3be1f2118286a5c80305bece02accc07e35a151f4f0748e00cad3eda6639e2bd` |
| N15 | `ci-cost-investigation-02/tracer-implementation-01/checkpoint-01/log-contract.json` | `6918a0f3926c4d9f9d83ac4a8c442bfbeab49b0a58c8e6eb179c26d3bf59ecde` |
| N16 | `ci-cost-investigation-02/tracer-implementation-01/checkpoint-01/static-preservation.json` | `218d02f843b69e37bd12178416416ca120df488dee6a4424a895bf3e2a7c8abf` |
| N17 | `ci-failure-045970d1-41148286267-01/linking-analysis-01/analysis.json` | `64b1dd90b82d21d53376b3e55d7c2d82a112d6b82bc942a99402d719b12ac569` |
| N18 | `root-ordered-commit-acceptance-01.json` | `7dde9189be43104ff2cf52517f1fc6e74982676523074b304f4ea1fb149f9486` |
| N19 | `ci-failure-045970d1-41148286267-01/ordered-candidate-implementation-01/manifest.json` | `8b1ff118735219962b6f370e391d6db0aa66cef6e5b4790b365620618aad9d16` |
| N20 | `ci-cost-investigation-03/header-implementation-01/checkpoint-01/manifest.json` | `16494242dc603e9bec390ff11df28be0cc396fa82a99a6c133bcaadceeffd3bf` |
| N21 | `ci-cost-investigation-03/absence-implementation-01/final-01/manifest.json` | `f4d3f047f9502c4ab3fe815cfb7fbfee64516f443024e5062a7f77732b110e23` |
| N22 | `root-sources-commit-acceptance-01.json` | `8ad7e47986c95bbb0790095d88fe19a280bb3f3967dba09fed580f1362f094e9` |
| N23 | `root-post-sources-cost-pair-review-01.json` | `34298af3553a67b44324cc99e0f9057e9518d3c646afaf93a772506420790d4a` |
| N24 | `post-sources-controls-01/root-release-01/acceptance.json` | `b224e2657fde8f66edc2a52cde1414b61edcb2a0fb5baf6563d19e2ab62c5a89` |
| N25 | `root-post-sources-all-cost-targets-review-01.json` | `4f187063ee86d0125616b6a609236371825b7ff3061e43b59601f569c1bfb701` |
| N26 | `root-current-cost-supplement-02-acceptance.json` | `00b90c723ea7d72ef6d22e554bc9e9a3740b49c436a4a387926d715cca2f6992` |
| N27 | `post-sources-native-gates-01/inventory-verification.json` | `be3f3a8251d753ec092a863012624f24e34c4290ad9a509bf91da1247a223a88` |
| N28 | `post-sources-native-gates-01/race-verification.json` | `e5ea2cf90ab9904a0311094cd9c16de7136666c6a7de520196d745ad66bc65c2` |
| N29 | `post-sources-native-gates-01/session-isolation-verification.json` | `b9c8ea36c4aa44d2610bb7dfac8af25d1784e9f53f8533189546336770948be5` |
| N30 | `post-sources-native-gates-01/five-pairs-verification.json` | `76bf4118a62f40210b4a767bfd511e8a81a103381993cb8875d751c213a0d282` |
| N31 | `post-sources-native-gates-01/web-verification.json` | `822a06e887c4a2c71f9a7b26d4afeeece5994c32dce60ee869c7b8904cc6e53c` |
| N32 | `root-post-sources-e2e-acceptance-01.json` | `c23f78360e3405d626061b7e01891e23a1ad0ce7deacc5a42dd080dc23ce4ffd` |
| N33 | `root-client-s52-supplement-acceptance-01.json` | `ba0025b0d2a124b2146888f38c9d1a702ed79863443a7cb724f6fc495c644607` |
| N34 | `root-client-s52-supplement-verification-01.json` | `b2617a242333dbd5075086aab047ee67c1ae07effd45f9505603c83ad1cad977` |
| N35 | `root-client-s52-extended-cycle-acceptance-01.json` | `75eacb160ba44b7c5942d94304d1dda1786ec6ecfc23175d5cf3ff9aa9fcdcc9` |
| N36 | `post-sources-controls-01/client-dispatch-01/result.json` | `3e8616360a8c2d22a5a9c1549be58405cdcc4092f9c4c389a5c1763d190bf6db` |
| N37 | `post-sources-controls-01/client/R3--S52-expired-active/report.json` | `ac40ad8d29a5b9932dbd2036884ee24ce6a945202686652c71faa29a8da8407c` |
| F01 | `root-post-sources-final-local-gates-acceptance-01.json` | `c70186f92204d2dfed6966a4a6efbfa6d037d5414c07c1b3ccce0dd05901ab0d` |
| F02 | `root-post-sources-formal69-final-controls-acceptance-01.json` | `c2c252924bba9304693143967cfcb8f8170d00471ab13b07d065985d911d4d10` |
| F03 | `root-post-sources-selected-heavy-intervals-acceptance-01.json` | `28533d42c283fc43505cd0c8db678b2e083ec95f1dde239fe66e92230156ad25` |
| F04 | `branch-ci-18b922cf-01/root-acceptance-01.json` | `8fed344317105140ac39cc0de4b4145dd24e52f85507335f399632eea715f2b2` |
| F05 | `branch-ci-18b922cf-01/guards-timing-observation-02/timing.json` | `4cae5a56ccf4412d434f03a101d68cc875fd12d8050f40d2ff7665c6b6d259ba` |
| F06 | `fresh-review-evidence-post-sources-01/per-commit-index.json` | `badc363b4e443fc22bfcbe39e946d336c09197b79fab0b8adb4e514fc0886fe3` |
| F07 | `root-final-s1-decisions-02.json` | `ba47665eed196fd0fde34bcd4cb6b7dcb526dc889beb62517b87d73229a38f25` |
| F08 | `fresh-independent-reviews-18b-01/root-a-acceptance-01.json` | `9b1dbd2cde21a07b0891472ec110d91e12503a29f2e450d4819b264d5cf92c66` |
| F09 | `fresh-independent-reviews-18b-01/root-b-read-boundary-disposition-01.json` | `2ac1481ad50cb9d01d397a72a831f95a7d5e788edd3f42a9bc7dff09141d64da` |
| F10 | `root-document-archive-18b922cf-01.json` | `160ef24c45305beb285ca1285371e7d03cecf63dbad91f4bc1bfbf9b663859ff` |
| F11 | `fresh-independent-reviews-18b-01/root-b-acceptance-01.json` | `44a58c5aa69bf9040a50b776f2d895a8b4dab3363238387bb3eb2db8625318a8` |
| F12 | `fresh-independent-reviews-18b-01/root-c2-acceptance-01.json` | `93ecd97df37d3f84316327907466b1a36a25a29eae005618b2ec5a3e0ba25d73` |
| F13 | `fresh-independent-reviews-18b-01/root-final-three-acceptance-01.json` | `50c1535989964c3c9ecc623a138e01a142593fc0a25436869c50bf612e3f5476` |

每commit原件索引F06将16个parent/tree及各minimum/gen/projection原件逐一列出。S1-0a/dd6e、e5、7d3＋catchers/同字节2c4、candidate02/03与386dirty、f64测试迁移修正的编译闭包复用均保留；未把它们写成本轮18b重跑。bf8→14fe尾注桥与8ba→045 subject桥都是未发布同tree/同父的元数据修正，0b/18b是其后真实源码变化；已发布4dd缺尾注不强改历史。GNU mkdir锁、Ryuk失败/重放和旧重叠事实、原S52 strict parser拒绝均未删除。