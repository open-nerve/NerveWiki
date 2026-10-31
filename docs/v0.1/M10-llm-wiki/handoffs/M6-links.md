```yaml
status: done
from: M6 收尾
to: M10
created: 2026-10-06
```

# lint 的数据在链接索引里

总体设计 5.6 的 lint 规则大多由 DB 查询算出。它们要的数据 M6 已经写进链接索引（[M6 总设计](../../M6-links/00-M6-design.md) 4.3；表在总体设计 7.2 的 `linking`），只记在 M6 的文档里（[M6 收尾审查](../../M6-links/reviews/M6-closeout-review.md) C-Q1）：

| lint 规则 | 索引里的数据 |
|---|---|
| 待建页面 | `page_links.resolved_id IS NULL` 的链接，按 `target_key`（目标最后一段的标题键）分组计数；`target_key` 为 `NULL` 的目标无论树怎样都解析不到 |
| 孤儿页 | 没有别的页的 `page_links.resolved_id` 指向它（反链的查询，`linking/app/backlinks.go`） |
| 链接有歧义 | `page_links.ambiguous`（几页在每一项偏好上都相同，按 id 选了一页） |
| frontmatter 解析失败 | `indexed_pages.frontmatter_valid = false`（这时没有属性、frontmatter 的标签与别名；正文里的 `#标签` 照记） |
| `sources` 指向无效、未消化的来源 | 属性链接带属性路径：`page_links.property_key` 是 `sources.0` 这样的路径（只有整个值恰好是一个链接的字符串才算，总体设计 4.4 末） |
| 缺类型、缺摘要 | `page_properties` 的顶层键（`position` 是书写的次序，值是 jsonb） |

注意：

1. **索引不总是当前的**：`indexed_pages.revision` 与正文的不同（读正文与读索引之间有写），或 `extractor` 不是当前的 `domain.Extractor`（升级改了提取规则、还没 `nervewiki reindex`）时，这一页的行不可信。lint 跳过这样的页，或者像阅读视图那样即时解析（M6 总设计 4.7）。
2. **提取规则变了就提升 `domain.Extractor`**，并在发布说明里要求执行 `nervewiki reindex`。M10 若加提取的东西（例如 `type`、`summary` 单独成列），照此办。
3. **索引表不带外键、没有 `deleted_at`**：被删页的行由删除路径删掉，`checkLinks`（`bootstrap/links_test.go`）守住不变式。linking 的 SQL 不 JOIN page 的表，要节点的状态经 page 的读端口（M6 总设计 4.3）。
4. **来源区与模式**：哪些页在来源区，由 M10 的笔记本模式决定；索引只给出链接解析到哪一页。

## 处理进展

### P3/S1完成接收（2026-10-31）

| 原移交行 | 当前 S1 实现与已存在证据 |
|---|---|
| 待建页面 | current unresolved 边先过来源/kind/presence/虚拟目标资格，再按完整 canonical 分组；pages 为不同 source 的 union，代表取过滤后按 source/range 排序的首个 raw。旧 target_key 只作可解析末段资格，不能称现行分组键。TestLintWantedCanonical、TestLintWantedUnion 的 same-source/later-source summary/list、S06–S15 及实际跨批控制对应。 |
| 孤儿页 | 候选资格先于 cap，入边只计本 notebook 的 current 其他 page；排 self、asset、被排来源发出者，root/AGENTS/source/folder 按本规则豁免。TestLintRules/page-qualification、TestLintCountsAndPages 及 S20 四个独立故障对应。 |
| 链接有歧义 | 读取真实 ambiguous 标记，按 page_id+完整 raw 去重；current emitter，resolved page 也须 current，asset 不伪造 extractor。不改变原 resolver 的 ID tie-break。八规则真实消费者的非空 summary/list、批次及物理范围证据分层保存。 |
| frontmatter 解析失败 | current 行的 FrontmatterValid=false；fed62 的 current/outside 候选先排有效 FM，SQL 仍核资格；LLM 排来源但不统一豁免 root/AGENTS。保存提取仍不保留坏 FM 的属性/FM 标签/别名，正文标签照记；lint 不重 parse。TestAFrontmatterNotValidHasNothing、真实坏 FM 消费者与 S21 对应。 |
| sources 无效/未消化 | 首 ASCII sources 键与完整值链接 slot；先数全部匹配边基数，再判目标，L/J 身份分开。D 来自合格 current 区外发出者的正文或首 sources 唯一 slot；F(A) 是 current 非空来源页正文四 kind 指向附件 A 的父集。A∈D 或 F(A)∩D 非空时已消化；**否则** F(A)非空由待消化来源页代表；F(A)空则独立待消化。生产 pending 附件查询只需 A∉D 且 F(A)=空，不新增无消费者的父消化查询。TestLintSources、S25–S30、S47/48/55、S66–S69 分项对应；S29 无独立 any→all 中间点照实记录。 |
| 缺类型/缺摘要 | position 首个 ASCII 键，非字符串即 missing，不跳到后续键；JS trim 含 FEFF、不含0085，257 空白前缀必要时查完整值。资格先于 cap，Count/Items 同关系。TestLintMissingProperties、S22/23/24 的两层腿与必要 JSON 物理证据对应。 |

**本轮读取实现补记，六行语义不变。** 0b 的两条 Orphan Items 在完整 candidates CTE 内按 id 排序，外层按等值 c.id 排序，保留全部候选、资格、First/After、零 UUID、Count cap 与返回身份；实际计划守卫限制按 take 产生的 PK/anti 探测，不依赖固定节点形状。18b 的 SourceHeads/SourceValues 将 notebook/current/FM 资格放入 requested 每项的 indexed_pages LATERAL LIMIT1，保留首 ASCII 键、首 position、配对精确值和必要 JSON 两读。新 HasSourceProperties 在 SourceValues 第一次 Next 对完整、已拥有且严格排序唯一的 eligible IDs 只探一次：先 partial-index 属性点探测，命中后仍验原 notebook/current/FM；false 仅停止属性流，正文 D 与 pending 继续，true 继续原 512/8MiB 流。错误/取消不得当 absence，无新 p.notebook 条件、索引、缓存或跨 snapshot 复用。 完整候选数组的校验/编码/物化仍计成本；Has 的晚命中或全密集情形不是零成本。ORDER 的尾/空页在旧诊断中有约3–5ms额外排序样本，不能称所有页更快。fresh100k/noStats/ANALYZE/custom/真实generic与隐藏A/B/A是已测边界，不解决既有page旧版本MVCC generic退化。SQL+Scan墙钟不是PG纯CPU，Go TotalAlloc、采样heap和PG/RSS/WAL分开；当前18b精确分支CI已通过；本机/分支证据仍不证明生产环境或指定两main配对。[N18–N22]

### 六行之外的注意1–4

1. 当前 P3 只以 Extractor 判 current/stale，不逐页对照正文 revision、不查缺失索引行，旧 extractor 每页计一次 stale_pages。真实 read-only REPEATABLE READ 快照贯穿模式、来源、节点、规则与末端投影。**正常写事务同时维护内容和索引不变式，checkLinks 是测试核对，不是运行时 revision 扫描。** 保留原 M6 revision 警告，并追加 P3 的这项选择；snapshot rename、downgrade、S05/S33/S69 不互相冒充。
2. Extractor 2 和既有 InFrontmatter 事实/等长 bool/CASE 已落地，正文为 NULL，合法空属性为非 NULL 空串。管理员以新 binary reindex，启动不自动重建；旧 v1 保留旧读者行为直到重建，lint 计 stale。linking/rekey 每本事务与后续 search ≤10000 节点独立批次分开；后批失败保留前批，不称整条命令原子。S63–S69的SQL/HTTP/Web及同故障OS/OB在当前18b总账已逐腿核验，bf8原总账保留历史身份。
3. DeletePages/DeleteNotebooks 显式清五类索引行；page 写事务内以同 ctx/revision 调用 linking，失败回滚，checkLinks 测试核对 live/revision/notebook/resolution 不变式。lint SQL 只查 linking 自有关系，page metadata 经根端口、逐字段 DTO 和同 snapshot ctx 进入；不把测试 helper 写成生产守卫。
4. **effective standard 的 root=nil；stored standard 仍允许 explicit LLM 合同+拟议 root。** 模式由 wiki/notebook 决定；SourceArea 提供不依赖 hash/ProbeSources、无10k成员截断的严格完整 live 后代，含空页/asset且排 root。3d96 的 LintData 将同一次完整 metadata 与原 strictSources 结果作为本次 snapshot 的局部 bundle 交给 wiki；standard 仍用 Nodes，旧 SourceNodes 与 bundle 共享严格成员 helper。page bytes/created_at 来自真实元数据，最终唯一 asset IDs 再从 asset 根端口取 bytes。S01–S05、S60 及跨 REST/两 MCP 非空字段证据对应；M11 仅交接。

以上六行及注意1–4的实际源码、具名测试和控制全部在18b核验，正式69行与补充分账见[S1审查](../reviews/P3-S1-lint-protocol-review.md)。本次分支文档交付将本移交改为done；Step文档自身与merge/record两main验收仍是独立集成门禁。原M6六行表的target_key/即时parse说法、revision警告、P3A L6与原阶段open句均保留为当时事实，不用它们覆盖当前状态。

### 既往处理进展（历史）

以下open/pending为各次记录当时状态，不再作为本移交当前状态。


- S1-0a（2026-10-30）：已实现writer/generated与Extractor 2，正文NULL、空属性非NULL空串，现有读者/API/asset/Web与管理员重建说明同步；前置测试、S63/S64/S65/S68及代表成本见[P3 §10.1](../03-P3-lint-health.md#101-s1)。注意2的版本边界已含持久化表示；注意1的lint跳过/计数、六行规则和S66/67/69仍待后续S1，注意1–4及表六行全部验收前保持open。M6/P3A的L6及以下各阶段原记录保留。

- P3来源修订提案（2026-10-30；[新记录](../reviews/P3-origin-design-review.md)）：主持者的决定（负责人可以改判）62拟在S1-0a保留Link.InFrontmatter已有来源事实、writer/generated与Extractor 1→2同提交，管理员用新版reindex；当前实现仍是1，M6原空键限制仍为当前事实。注意1在P3选择跳过/计数并提示管理员，不在lint parse；注意2的版本边界含持久化表示。PageProperties保持IS NOT NULL，v2空key可见，View/rewrite/batch/backlink及搜索队列边界按P3§3.0；S1-2再交真实D/F四kind与原六行。原M6说明/README待实施时更新，不把计划当完成；六行及注意1–4全部验收前仍open。

- M10 开工（2026-10-24）：照[M10 总设计](../00-M10-design.md)第 7 节"移交的落实"的"M6 lint 的数据在链接索引里"两行（表的六行、注意 1–4）做，在 P3 的 S1（4.11）。仍是 `open`，做完的 Phase 改为 `done`，收尾核对。
- P2/S1（2026-10-27，合并见[00号文档](../00-M10-design.md)第12节）：S1仅提供写入契约的来源事实/rewrite，未实现本移交的整区lint读取/索引规则；六行及注意项仍归P3，状态open。S1范围见[P2结果](../02-P2-source-area.md)10.1。

- P2/S3（2026-10-29）：本轮只完成来源网页身份、系统rewrite既有契约消费和真实mode变化；未实现整区lint读取、链接索引规则或健康度。六行和注意1–4仍归P3，open保持；[P2结果](../02-P2-source-area.md#103-s3-完整来源网页)不替代此移交验收。
