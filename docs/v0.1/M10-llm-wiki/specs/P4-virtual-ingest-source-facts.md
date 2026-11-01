# 固定源码事实索引

以下事实固定在 `6992fd042ff06c110a4e59e67fb89bfe5f43fbd9`。当前源码身份与引用集中在[P4源码pins](P4-virtual-ingest-source-pins.json)；新增端口／SQL／字段由设计规定，未声称已实现。历史比较与静态审计原件保留树外冻结PREP，本附件只保留实施者需要的当前事实。

| ID | 固定源码锚点 | 已核事实／候选界线 |
|---|---|---|
| F01 | `server/internal/modules/page/app/read_path.go:52` | 普通ReadPath明示unlocked/no transaction及校验顺序/Cut；virtual独占快照为新增候选。 |
| F02 | `server/internal/platform/postgres/tx.go:31` | 现Tx/Snapshots的RR只读、ErrNestedSnapshot与RC边界，不能假定普通read caller开tx。 |
| F03 | `server/internal/modules/transfer/app/export.go:137` | export已有snapshot包住fill/贡献者；复用ctx，不嵌套。 |
| F04 | `server/internal/modules/page/tools.go:31` | ToolDeps在Module.Tools检查；Virtual新检查归这里，page.Deps.Snapshots已存在。 |
| F05 | `server/internal/modules/page/app/unit.go:355` | 共享applyStep在guard后真正write前可捕获memo；tell在711行交Event。现行尚无BeforeIngest。 |
| F06 | `server/internal/modules/page/adapter/postgres/source_area.go:16` | source-hash namespace与SHARE来源路径锁，与index advisory分开。 |
| F07 | `server/internal/modules/page/app/apply_batch.go:273` | batch计划ListNodes在Writer.Run前，不能当unit completeness。 |
| F08 | `server/internal/modules/page/app/extension.go:76` | 现StepContent有BeforeBytes但无BeforeRevision，Event无BeforeIngest。 |
| F09 | `server/internal/modules/linking/domain/facts.go:9` | Extractor2与body/property表示；literal-v1不能冒称current。 |
| F10 | `server/internal/modules/linking/domain/lint_sources.go:93` | 首JSON/direct slot、全matching peers、空target/asset无id错误。 |
| F11 | `docs/v0.1/M10-llm-wiki/03-P3-lint-health.md:544` | 批准的旧事实、首次Before、Unknown、untouched coverage和最终树近似边界。 |
| F12 | `server/internal/modules/page/domain/list.go:31` | 现游标O/I；V字段与校验是新候选。 |
| F13 | `server/migrations/sql/00052_linking_lint_ranges.sql:5` | 固定6992最后迁移52；P4计划00053_page_ingest.sql，采用前核号。 |
| F14 | `server/internal/modules/page/adapter/postgres/queries/history.sql:11` | 原三种历史路径和FirstChanges；true ingest需专用分支。 |
| F15 | `server/internal/modules/page/adapter/postgres/queries/purge.sql:57` | 父deleted/trimmed资格与SKIP LOCKED；source叶接现purger。 |
| F16 | `server/internal/modules/transfer/app/contributors.go:11` | Contribute/Scope/Sink、RootID whole/subtree及snapshot约定。 |
| F17 | `api/modules/linking.yaml:3` | 真实OpenAPI3.1/api/v0、security/x-problem-codes；无x-action新键。 |
| F18 | `deploy/runtime-grants.sql:19` | runtime grants真实路径；新表只加数据权限，不加MAINTAIN。 |
| F19 | `Makefile:278` | 固定6992成本/guards/五分片入口；实际行与命令见后表，本轮不执行。 |
| F20 | `server/internal/modules/linking/app/list_index.go:132–149` | MaxIndexType实际用UTF8字节len；校验在notebook之前；目录当前无tx。 |
| F21 | `server/internal/modules/linking/adapter/postgres/queries/index.sql:1` | 首ASCII type/position；257字符存储头、128字符排序头；exact filter。 |
| F22 | `server/internal/modules/linking/domain/index.go:25` | 1024B参数、256B显示、64B key/128字符；key碰撞不代表exact同类型。 |
| F23 | `server/internal/modules/linking/admin.go:57` | reindex直接app.Index构造，与生产NewIndex不同；不装伪marks、不回填。 |
| F24 | `web/apps/web/src/app/routes.tsx:146` | 实际网页路由文件与NotebookLayout；S3在此新增index route。 |

补充：P3§3.8.2/00§4.14决定62禁止把HasSourceProperties(false)当OldFactsKnownAbsent。`linking/app/lint_reads.go:271–337`现SourceValues/SourceSlots/Bodies可复用无tx所有权的读取，不代表Capture已实现；`page/app/unit_delete.go`已把全子树Changes/Kinds送shared applyStep。`server/migrations/sql/00037_page_changesets_log.sql`原notebook历史索引无deleted predicate而actor索引有，新ingest索引承接此差异。

新增types计数范围、有界catalogue字段与数值已随决定72接受设计，但不是6992已有源码事实；完整约束/必要00修订见P4-virtual-ingest-types-boundary.md。普通无锁目录不因types_complete新增RR承诺。

F25：`server/internal/modules/linking/adapter/markdown/facts.go:23–26,76–89` 与 `facts_test.go:166–198` 固定源码确认NUL→FFFD存储归一；types精确身份绑定索引保存值，不能绑定原Markdown/YAML拼写。两文件当前身份见source-pins；本Phase没有据此声称新产品测试结果。

## 撤销树修复的继承事实（固定6992）

| ID | 实际锚点 | 必须保留的边界 |
|---|---|---|
| F26 | `server/internal/modules/page/app/ports.go:168`、`unit.go:32`、`trash.go:20`、`module.go:285,295` | RevertTrees独立接线；新增IngestNow不替代它，NewWriter/NewRevertChangeset参数形式未改，无新增全局nil验证。 |
| F27 | `server/internal/modules/page/domain/revert_tree.go:7`、`revert.go:117` | 六字段ID/ParentID/Kind/Name/NameKey/SortOrder；stored NameKey与原SortOrder保留，nil未读与非nil空树区分；没有revision/bytes/deleted/audit，不能作Before/coverage。 |
| F28 | `server/internal/modules/page/app/revert_changeset.go:209`、`unit_revert.go:71,682`；`adapter/postgres/gen/nodes.sql.go:807,817` | 保留预读／锁下重读和ListRevertTreeRow；公共ListNodes及四字段VirtualTreeReads候选不变。 |
| F29 | `server/internal/modules/page/adapter/postgres/revert_tree_test.go:18,68,76,104`、`app/revert_tree_test.go:51,84`、`app/revert_test.go:572`、`domain/revert_facts_test.go:14`、`domain/revert_name_test.go:81` | 完整活树含asset、空树、ctx事务/取消、锁下重读、来源与未触及节点见证为现行回归基线；不计P4控制已执行。 |

23路径构造consumer共30处声明／调用，另外六处WriterDeps／TrashReads字面构造在五文件，见[文件表](P4-virtual-ingest-files.json)。186个实现／生成／审计路径加计划00053迁移共187个落点；15项撤销修复依赖要求保留，审计不等于要求逐项改写。

## 6992 的实际 Make／CI 入口与余量边界（仅静态读取）

| 固定锚 | 现行事实 |
|---|---|
| `Makefile:179` | `GO_TEST_RACE = go test -race -count=1 -timeout 15m`；不提高原期限。 |
| `Makefile:190` | `test-go`先以-j6运行race及bootstrap-1到5，再依次costs、costs-lint、guards。 |
| `Makefile:239` | FIRST=`^Test([A-D]或E([a-u]或very([A-Q]或ResolutionCase[A-H])))`；精确正则以source pin原行字节为准。 |
| `Makefile:240`、`:242`、`:249` | SECOND、THIRD、FIFTH分别按原正则；FIFTH=`^TestThe(MCP或Lint或WikiTools)`。下表给实际命令，避免把表格中的“或”写成shell。 |
| `Makefile:258`、`:262`、`:266`、`:270`、`:274` | 五片实参依次为run FIRST；run SECOND skip FIFTH；run THIRD skip SECOND或FIFTH；skip FIRST或SECOND或THIRD或FIFTH；run FIFTH。 |
| `Makefile:278`、`:309`、`:334` | !race成本三个独立targets是test-go-costs、test-go-costs-lint、test-go-guards；P4新cost recipe尚不存在。 |
| `Makefile:289`、`:354` | 原历史页成本组与TestABatchAtItsRewriteBoundTakesItsTime组保留；新history/ingest成本按原界添加，不替掉旧组。 |
| `.github/workflows/ci.yml:46`、`:55`、`:68`、`:71`、`:76`、`:84` | server matrix为race20m，五bootstrap各25m，costs25m，costs-lint25m，guards30m，short20m；Go期限先到。 |

五个现行bootstrap recipe的精确命令（未执行）：

```make
cd server && $(GO_TEST_RACE) -run '$(BOOTSTRAP_FIRST)' ./internal/bootstrap
cd server && $(GO_TEST_RACE) -run '$(BOOTSTRAP_SECOND)' -skip '$(BOOTSTRAP_FIFTH)' ./internal/bootstrap
cd server && $(GO_TEST_RACE) -run '$(BOOTSTRAP_THIRD)' -skip '$(BOOTSTRAP_SECOND)|$(BOOTSTRAP_FIFTH)' ./internal/bootstrap
cd server && $(GO_TEST_RACE) -skip '$(BOOTSTRAP_FIRST)|$(BOOTSTRAP_SECOND)|$(BOOTSTRAP_THIRD)|$(BOOTSTRAP_FIFTH)' ./internal/bootstrap
cd server && $(GO_TEST_RACE) -run '$(BOOTSTRAP_FIFTH)' ./internal/bootstrap
```

按这组现行表达式，候选bootstrap TestVirtualPaths/TestVirtualRead/TestVirtualExport落第三片，TestIngestUnit落第四片；这是名字的静态归类，未编译运行，不能代实际完整清单唯一归属核验。新!race成本选择器须显式加入相应recipe，不以普通race/short替代。

6992 own main CI41284728142 attempt1已按最终sample-10接受：13作业、38必需步骤全部成功。五片Test仍为463/309/362/404/313s；不纳入已接受4e6e/df9指定pair。单次第一片463s不能保证新测试余量；每进程双轮与本Phase指定main配对各片均值严格<450s仍必需，不沿旧本地数宣布有余量。


## P3完成与正式设计入口

| 当前事实 | 精确身份 |
|---|---|
| P3done main | 6992fd042ff06c110a4e59e67fb89bfe5f43fbd9；tree bd74094f1e4b4fbaf04085431e2ba641f7e6c134；parent df9b1d49544336f80d768bde4b6cda231403231e |
| own CI | [41284728142 attempt1](https://github.com/open-nerve/NerveWiki/actions/runs/41284728142/attempts/1)，push／main／精确6992；13作业、38必需步骤成功；接受回执root-p3done-ci-acceptance-01.json SHA256 `a83df64a0aa5146ef15d5cdd744edd8e26a82148200ac356faa712ffaee17105` |
| 实际完成／放行 | root-p3done-completion-p4-release-01.json，2026-11-01 07:50:28 UTC，SHA256 `b6375d24f2e3b352060d10f9b965356250a063fd84e8f0d18f5d66ff8bbddaad`；仅释放P4正式设计 |
| 原指定pair | 4e6e／41277758157a1及df9／41281561763a1；五片Test均值394.5/201.5/308/387/314.5s；6992 own run不加入 |
| 历史与后续 | 原13ed失败／重跑限界及M12分配量、生产负载／停机债务保留；P4三审、处置／原席复核、受控00与设计文档main CI先于实施 |

## 修订1事实及修订2校正（固定6992，只读；不是产品执行）

- **R01 目的地与导出**：`linking/domain/rewrite.go:473`已对literal percent先编码；`transfer/app/contributors.go:15`的ScopeNode实际有ID/Asset/Path，`transfer/app/export.go:207`从实际plan建立贡献scope；`transfer/domain/plan.go:158`含folder-only与collision路径。本设计改传此映射；不能由原Notebook路径重造归档名。
- **R02 遗漏消费者**：`bootstrap/batch_cost_test.go:64`与`bootstrap/mcp.go:48`是两个ToolDeps字面构造。e2e/fixtures/mcp.ts:199–221的PageRead.meta现version必填；当前审计为21个直接.meta.version访问＋3个PageRead.meta解构读取，共24站点/9故事文件。新增as2:113→base_version138，补pg12:153→156（expect154）及pg8:101→108/109（expect102）；取值站点与下游使用分列，S1-3先真实页/version收窄，保留原断言。c4:97／mc8:152的本地JSON.parse casts不算PageRead消费者，不纳入本次修改；call args为Record<string,unknown>不构成已观察tsc失败。writtenOf必填version与P5边界保持。
- **R03 D/F与current**：P3 lint_scope的current只核Extractor；P4的matching revision为新增证明。来源不可变性有system Rewrite例外（page/domain/contract.go:171；page/app/unit_content.go:24,122）；S∈W仍live可达。来源F与wiki D资格必须分开，真实body边不是filesystem parent。
- **R04 实际容量边界**：linking/domain/facts.go:102–110的MaxLinks=10000是每页索引全边上限，frontmatter在前。page/domain/batch_plan.go:49/78/109的400页/120000links/34,406,400B分别约束batch rewrite达到的页、达到的链接、这些页字节；linking/app/rewrite.go:47–54,79,214,371确认120000不是全部旧F。page/domain/batch.go:49/53为100ops/6MiB字符串；trash.go:47–49为显式多页解析100页/8MiB/20000links，apply_batch.go:340实际执行。单rename/move不受batch rewrite400界；R必须按实际W来源页×10000再计各适用约束。附件有扩展名才Linkable（linking/domain/resolve.go:31–44）；4M夹具用不同完整名aa.b及连续8B链接，真实解析/guard/预算全待执行。
- **R05 错误与retry**：普通目录错误不满足lint healthRetry的server_busy条件，因此旧W06 mutation等价；修订仅令目录retry no-op。TxManager的fn错误走rollback并保原错（rollback失败优先infra），成功才Commit且结束用WithoutCancel；固定pgx v5.11.0在COMMIT得到ROLLBACK时返回ErrTxCommitRollback，不能拿最终泛错证明取消及时传播。依赖pin留外部inputs，不发布Go module cache路径。
- **R06 移交尾部**：M12/handoffs/M5-polish.md固定现1–32，未有目录parent-selector项，实际最后一句是“之后的 M 推迟的体验事项照样追加到这份移交。”；S3完成后按实际尾号在该句前新增，当前不写handoff。

本轮未执行任何新增SQL、SQL生成、产品测试、成本或迁移。旧D/F读取扩大Bodies输入范围并保紧凑old-F关系，是实际设计变化；命名SQL仍17不表示SQL工作未变。所列代码行以source-pins为准；全部原source pin、原控制ID和P3修复依赖保留。

- **R07 新常量命名与取消池恢复（修订2）**：固定linking/app/lint_reads.go:116已有BodyReference DTO，:337的Bodies返回它；只将本设计新增origin常量改名ReferenceBody，保iota值/FirstSourcesSlot语义与既有DTO不变。固定linking/app没有ReferenceBody声明，未执行编译。TxManager:97–100允许语句取消时关闭物理连接；C10要求清理后池可服务新查询，不要求同backend PID复用，原错/禁止后续写与commit/真实回滚/WithoutCancel及rollback失败语义不变。
