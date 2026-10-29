# M10/P3来源表示修订：独立设计审查记录（事实性出版稿）

日期：2026-10-30。状态：方向由主持者决定62核定；首轮三席对同一reindex事务/失败边界提出的发现，经第二稿修订和原A/B/C实际复核均已关闭，各席新/未决I/M/N/Q全0。主持者已接受第二稿设计语义。本事实性出版稿仍待root逐字审读/最终冻结及投影，实际设计提交自身CI与P3放行仍待；本来源修订没有新实现、产品测试或测量结论。

## 对象与来源

审查对象为[P3设计](../03-P3-lint-health.md)新增来源合同及其[00号文档](../00-M10-design.md)/[总体设计](../../v0.1-design.md)同步、M6/M11/M12 handoff进展。原[设计审查](P3-design-review.md)的49–60接受和[搜索维护](search-guard-calibration-review.md)的61事实各保留自己的对象，不扩展其接受范围。

最终提案源为维护记录提交601c5832882d8df10486ea335e691b7f11efc259、树ff1a8b2e761a6091910c90e136695398096eb806，父b957fdbe0367a9a6ff0f65d2afc303db49a755c2。它只改四份维护文档，产品源码与aa860/b957相同；当前domain.Extractor=1。已逐字读取601c的维护差异，保留其aa860文档CI和b957首次mainCI接受事实、旧失败/校准，以及该记录写入时自身CI待验的历史原文。冻结第二稿之前，主持者交来601c自身CI40932595248 attempt1的实际接受回执：11作业、32必需步骤成功及23条件跳过；20指标、45 HTTP、60搜索格核验通过，210旧文件未变，累计51次已观察运行、无库存缺口。搜索测试63.82秒；go首页三统计状态中位数22.483/22.572/23.364 ms，亦低于旧40 ms，不能据此归因或宣称未来稳定。来源为树外calibration/ci-record-601c5832/root-ci-acceptance.json SHA256 bd303df1c3b899a4743e7ed6b6251a5090246928b808f083aca29e964121f597，以及search-calibration-record-601c5832-ci-cost-supplement.json SHA256 184ab9ae3faa3eaf97f49f399418ae05e3a1d6fd84dc9c6fec9c1f29f63f7042；作者只核回执/报告绑定与哈希，没有另跑CI。它只接受维护记录，不是本来源修订的独审/CI/P3放行。

树外方向原件p3-preparation/origin-amendment/root-decision-62.md，SHA256 245f24b9052d5f890b2bbcc326fd0e867c6dd5e8cb88424af16462618e172829；前序来源调查empty-property-origin-design-recommendation.md，SHA256 2714629cdf01064f7deb6bd7ad563c9c53e08443c397f5e6e49c3a7856a5cb2b。它们以名/哈希标证，不把机器绝对路径变为仓库依赖。具体提案的replacements.json、rendered文档、proposal.diff及artifact-receipt.json在作者外部包保存；冻结哈希由主持者给三位审查者，不在本文件内形成自引用哈希。

## 已核源码事实与修订理由

下表linking/与page/在server/internal/modules/下，bootstrap/在server/internal/下，迁移在server/migrations/sql/下；API/Web从仓库根计。行号绑定上述601c源的未变产品代码。

| 事实 | 精确源路径/边界 | 本提案如何处理 |
|---|---|---|
| Extract已保留空属性来源；持久化丢失 | linking/domain/facts.go:48–53的InFrontmatter看Property或Quote；linking/adapter/markdown/facts.go:49起保留两者；linking/adapter/postgres/store.go:98起只写Property，linking/adapter/postgres/queries/links.sql:7的NULLIF合并空串与NULL | 只补等长bool与CASE；不改parser/语法/depth/MaxLinks |
| 旧NULL无法逆推出原来源 | 00020_linking_indexed_pages.sql要求extractor>=1；00021的property_key可NULL/空串，00026已有IS NOT NULL部分索引 | 同提交Extractor 2；不加列、不UPDATE旧行、不设0，管理员重建 |
| 版本敏感读者已有明确降级 | linking/app下views.go:42、rewrite.go:265、batch_links.go:65/90、backlinks.go:165 | Views重resolve不写索引；rewrite/batch跳过非current；backlinks无旧context，分别验证 |
| Properties读者不丢空字符串 | linking/adapter/postgres/queries/reads.sql:58起三组IS NOT NULL；同adapter的reads.go、linking/app/properties.go与adapter/http/handler.go逐层；api/modules/linking.yaml的PropertyLink.key；web/apps/web/src/pages/page/page-properties.tsx:125 taking | v2空key进入原API/asset签名/面板，实际正反证，body排除；v1旧行为持续至重建 |
| 新版reindex是现成管理路径 | linking/app/reindex.go:75–130及linking/admin.go的Rebuild仅管linking/rekey事务；bootstrap/reindex.go:55–103随后入队/决定退出；page/search_index.go:45的Requeue及QueueSearch每批独立提交 | 明确linking事务持锁/阶段失败不留部分改动；queue在其后，即使linking失败仍可运行，queue错误不撤销v2/前批，单本命令及全实例均非整体原子；分阶段实测留S1/M12 |
| P4旧union目前仅为未来合同 | P3§3.8.1/00§4.14；Index.PagesChanged是最终树观察者、BeforeIngest仍待P4实现 | 显式KnownAbsent/Ineligible/Known/Unknown；旧与未改coverage足够才允许新标，未知不误作空 |

## 主持者的决定（负责人可以改判）62

采用现有nullable property_key：NULL只为真实body，非NULL空串为合法空路径属性；writer/generated与Extractor 1→2同提交；现有Properties API接纳空key。管理员新版reindex，现有版本敏感读者行为与搜索副作用公开；F含wikilink/link/embed/image四kind。P4区分旧事实未知并保守不新增无法证明的ingest，旧标记保留、正常编辑继续，未改页coverage也必须可信；它与可信未改页的最终树近似分列。

原“不升Extractor”承诺明确修订，其他49–60合同及61维护不变。本轮将决定62及前序推荐中的“逐本事务/失败原样”准确限定为linking/rekey阶段；随后search各批独立提交的既有行为不变。原方向原件/哈希保留，这项文字校正不新增产品决定或编号。采用是有升级/API成本的局部索引表示修正，不构成未解重大架构冲突；本决定不是绕过设计审查、版本边界或实测。新增列也不能恢复旧NULL、却增加DDL与重复不变量；保留近似会错误消化，不采用。

S1-0容量处置先行，S1-0a来源表示/版本及兼容独立绿，随后S1-1..7。原72行文字不改，S63–S69追加为79行：writer、generated SQL、literal版本、D First/After/F来源、F四kind、Properties SQL/Web、D/F有效current门各自有真实catcher。D/F控制待S1-2，旧/新/API/reindex控制可在S1-0a交付；P4 Unknown变异留P4，不假称本期运行。

## 第一轮独立审查与主持者处置（历史）

首轮同模型A/B/C各自读取冻结提案及实际源码，独立静态核实后交报告；没有产品测试/SQL执行/运行时变异/性能或CI核验。受审round1-manifest.json SHA256为9143833e8684fc90e479a068b98f2bb659d8e02be28813a69d611cc1602dbab2。主持者完整阅读全部66项old/new、新记录全文和三份报告后定处置；以下保留各席原级别，不把三份同义报告改写成三件独立缺陷。

| 原席/发现 | 首轮结论与核实的问题 | 主持者处置/第二稿修订 | 第二稿冻结时的原席复核状态（历史） |
|---|---|---|---|
| A finding1 — Minor | 核心来源/版本符合现有合同；命令的“每本一个事务/失败原样”泛化了linking保证，README与OB oracle需区别queue结果。其余0 Important/Minor/Nit/Q | 接受，同组按Important处理；Phase§3.0、§7.1/7.4/8.1a及00/总体/M12明确linking事务和后续queue，保留既有CLI | 待进行，不预填关闭 |
| B-M1 — Minor | D/F与P4已排除未知当空的反例；reindex同本的queue独立提交，被写成整体回滚。其余0 Important/Minor/Nit/Q | 接受，同一处置；linking失败queue成功、linking成功queue失败、后批失败保留前批分别写独立状态oracle及成本 | 待进行，不预填关闭 |
| C-I1 — Important | 整条命令可以部分成功，原OB/升级投影会承诺错误回滚；Requeue源码指针也需纠正。其余0 Important/Minor/Nit/Q | 接受，作为本组最高级别：准确限定锁/事务；OB实际子名及单本/全本结果、已有CLI控制；源码改page/search_index.go:45。指针改正属本发现，不另造新发现 | 待进行，不预填关闭 |

原报告在树外origin-amendment/reviews/：a-round1.md SHA256 abb5c0b6b3922ecf482dada84999384f4bd6ae20a262e589db2f92d1d83e0ee4；b-round1.md SHA256 478dfbf730fce471d25c90d75d93a6c4b8f2f5acbc6329afdaaed03651c6a181；c-round1.md SHA256 0ae43a1addad23058ea3071f0eab8738df4fd21369762b47731d721ad336af76。三份原文/原级别完整保留，合计为同一问题的1 Important与2 Minor报告，主持者按1组Important修订。root-round1-revision.md SHA256 9ba48f3ea7e5de351f721222391488ec03bedfff4e83f2de15654e7e732b54e2记统一处置；没有新决定编号。

第二稿当时保持66处对601c原Git的精确替换、同7修改＋1新文件及79行变异逐字不变。该轮语义修订只收紧linking/queue的事务、锁、失败/部分提交、成本/README未来投影；另按主持者新交真实回执更新维护record当前CI状态，历史pending原文保留并标明时点。列出TestTheLintOriginReindex的真实拟子名：literal-v1、linking-fails-queue-succeeds、clash-queues、linking-succeeds-queue-fails、queue-partial-commit、tokenizer-eligibility；故障/clash各有single/all。既有OB总命令不变，另明确保留并复核现有cmd/nervewiki两条CLI测试。没有改变命令实现，也没有执行这些测试。

作者当时提供完整第二稿及round1→round2的revision.diff；静态hash/唯一替换/archive投影不等于独立复核。第二稿冻结时原A/B/C复核尚待，上表因此没有预填关闭；之后三席实际报告和主持者处置见下节。首轮的1 Important/2 Minor同组发现及原级别仍保留，不把首轮写成无发现。

## 第二轮原席复核与主持者设计语义接受

三席共同受审对象为round2-manifest.json，SHA256 bb039137fa41b722b52196ab6ff1f13c982a926ea83cf7794288efaa952d9f19，绑定601c5832882d8df10486ea335e691b7f11efc259；该轮revision.diff SHA256 a344bb7ec70a75583072d73e237ea35d1aab6e9af5a9ff4cb7c5b186e8a8bfff。这是已接受的第二稿语义对象，不是本事实性出版稿或未来Git提交的自引用哈希。

| 原席/原级别 | 实际第二轮报告（树外origin-amendment/reviews/）与SHA256 | 原席复核结论 | 新发现I/M/N/Q | 最终未决I/M/N/Q |
|---|---|---|---|---|
| A finding 1 — Minor | a-round2.md；4f665edace04be9d549b29c5d03c9f401f692ce9234fe93b196aab1eeb3c98c7 | 原1条Minor已解决；没有其他A遗留 | 0/0/0/0 | 0/0/0/0 |
| B-M1 — Minor | b-round2.md；a6a05a951c19770fb0b0d9f3d2f320c780b922e4fae7ca62bcfe1d3aff5530fa | 原1条Minor已解决；没有其他B遗留 | 0/0/0/0 | 0/0/0/0 |
| C-I1 — Important | c-round2.md；51cbc35bf20c99d77c3cd5188c9009bd85d6ee6337d7f4d4450b9ed8be3b1d13 | 原1条Important已关闭；没有其他C遗留 | 0/0/0/0 | 0/0/0/0 |

三席各自核原发现与修订引入的问题，完整读第二稿250行窄差异并静态追踪相关真实源码、OB状态oracle/CLI选择器、锁/失败与成本/README投影；未重审所有未变合同，也未运行产品测试、Go/SQL、变异、队列故障实验、锁观察、性能或CI。报告各自只关闭本席发现；主持者完整读三报告后，按原处置关闭同一组Important并接受第二稿设计语义，没有新增产品决定。实际统一处置为root-round2-disposition.md，SHA256 2289684061e78807b55153dd199aa15d8aefba0dc69deed622e67389071101d4；决定62仍为主持者的决定（负责人可以改判）。

本次出版只补上述实际复核/处置与当前状态；全部79行、API/D/F/P4、SQL/兼容/queue边界、预算/timeout、测试oracle/选择器及实施次序保持第二稿。root仍须审读事实性publication.diff、冻结最终稿并完成archive/main投影；实际设计提交及其完整CI、原release verifier与P3入口重绑仍待。三席静态结论及维护record CI不替代这些条件，更不证明Extractor 2或S1-0a已经实施。

## 尚待完成的边界

- 首轮报告在各自范围未提出S69的新缺陷，C明确接受其作为低层防御负控的声明范围；真实scope与未来F门位置/selector/运行时效力仍由实施核验，不把静态评价当kill。上述唯一共同设计发现组现经原席实际复核并由主持者关闭；真实实施核验仍待。
- P4高效coverage/Unknown传播与诊断、保存成本由P4具体设计；本期只定兼容义务，不引入ready字段、每保存全树扫描或运行时正文修复。
- S1-0a与S1-2实际测试/变异、全量每提交门禁、来源保存/重建测量与容量仍未执行；原阈值不改。M12真实数据停机/负载仍open。
- 当前M6/README的v1说明保持；实施后的更新逐条写在P3§8.1a。原M6 review的“不改”处置保留历史，不能改写成早已支持v2。
- 维护record自身CI接受及此修订原席复核已有实际回执/报告，主持者已接受第二稿设计语义；最终事实稿冻结/投影、实际设计提交自身CI、主持者重新绑定放行回执与S1-0候选仍待，仍是产品入口条件。旧maintenance-release wrapper即使通过也不能释放本修订。

## 结果

主持者已接受第二稿设计语义，第一轮共同发现经实际原席复核后关闭，三席新/最终未决I/M/N/Q均为0。本事实性出版稿仍待最终冻结/投影、实际设计提交自身CI与P3放行。本来源修订未实施；当前Extractor仍为1，Extractor 2/S1-0a、D/F/P4实际行为、运行时测试/变异kill、迁移/reindex耗时及成本均待各自实施验收，无未来PASS结论。上文维护record的实际CI接受不覆盖本来源修订。
