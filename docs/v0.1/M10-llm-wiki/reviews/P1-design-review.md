# M10/P1 契约的框架设计：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | [01-P1-contract-frame.md](../01-P1-contract-frame.md)（初稿，2026-10-24，2,249 行），对照 `main` `d7bb5e8d` 的代码（与 M9 的 Codex 评审之后的 `91ee78d8` 相同，只多了 M10 总设计的文档）、[00 号文档](../00-M10-design.md)、[M10 设计审查](M10-design-review.md)、三份移交（M5-events、M8-history、M9-P1-mcp-registry）、`v0.1-design.md` 13.1–13.4 |
| 审查方式 | 三位审查者（Opus）并行，只读，不跑测试、构建、Docker 与数据库，不下载：<br>A：S1（模式、建、模板、降级、事件；第 0 节的 S1、第 1 节、2.1 第 1–4、9–11、15 条、3.1–3.16、4.1，6.1 与第 7 节的 S1 的行）。`review-A/blocks.py` 把 3.9 的四页从草稿取出（`review-A/tpl/`），照 `TestTheTemplatesCorrespond` 的规则逐块核对两种语言。<br>B：S2、S3（第 0 节的 S2、S3，2.1 第 4–8、14、16 条，3.17–3.29，4.2、4.3，第 6、7 节的 S2、S3 的行）。`review-B/m10-s7.txt` 是 00 号文档第 7 节的摘录。逐条走了每条写入路径到单元的步的种类（审查 B 的表）。<br>C：网页、e2e、测试、Step 与完整性（第 0、2、4–9 节与附录，2.1 逐条对着 `d7bb5e8d` 的 00 号文档）。`review-C/tests.txt`、`tests-where.txt` 是测试名与它们的份的清单；读了 `web/apps/web/node_modules` 里的 `playwright-core`（可读的名字的算法）与 `@atlaskit/pragmatic-drag-and-drop`（`draggable()` 的属性），只读 |
| 日期 | 2026-10-24 |
| 结论 | A：Important 0、Minor 6、Nit 14、Q 2，过度设计的两点（呈现）。B：Important 4、Minor 9、Nit 10、Q 3，过度设计 3。C：Important 4、Minor 12、Nit 14、Q 2，过度设计 4。B-I1 = C-I4、B-I2 = C-I3、B-I4 = C-I1、B-Q2 = C-Q2，所以不同的 Important 是 5 个。<br>全部处理，修订直接写进设计（草稿留作 `01-P1-contract-frame.draft1.md`，各部分留在 `parts-draft1/`）。处置由主持者定下（下面"主持者的决定"一节），负责人可以改判。<br>五个 Important 都照主持者的方向改了：只有守卫拒绝的情形经 MCP 的 `batch`、带 `guards: []` 的反向对照，挪进 S2（B-I1 = C-I4）；`TransferRefusal.code` 是自由的字符串，网页认它知道的码、别的写 `problem.other`（B-I2 = C-I3）；"被拒的撤销不占解析的预算"成为能失败的测试 `TestTheServersLimitsComeAfterThePrechecks`（B-I3）；TR1–TR4 都加 `refusal: null`，`e2e/` 里对报告的其余核对逐个列出（B-I4 = C-I1）；树的文字等价照 `NewTab`，确切的可读的名字由 e2e 核对（C-I2）。<br>另由主持者定下：定客户端的函数是 `shared.ClientOf`（A-Q1）；播种者的有类的错误是 500 的缺陷（A-Q2）；写者可以在同一父节点下排序来源根的祖先（B-Q1）；`batch` 的计划的预检挪进 S2、两个窗口照收并写在第 0 节（B-Q2 = C-Q2）；后台撤销的每批的检查去掉（B-O1）。<br>2.1 由 18 条变为 24 条（第 1、8、11、18 条改写，加第 19–24 条）；2.2 照旧为空。7.3 的变异由 47 条重排为 61 条（S1 第 1–22 条、S2 第 23–41 条、S3 第 42–52 条、S4 第 53–61 条；第一轮复核更正：64 条，S3 第 42–53 条、S4 第 54–64 条）。整个程序的新测试由 19 个（第二份 9、第三份 10）变为 19 个（第二份 8、第三份 11：去掉 `TestTwoAdminsDowngradingAtOnce`，加 `TestTheServersLimitsComeAfterThePrechecks`，两个 S3 的测试改名落进第三份）。作者在修订里另有几处与审查或简报所料不同（"作者的更正"）。<br>对修订的三份核对在后面的"第一轮复核"一节；这一节里被它更正的说法就地标着"第一轮复核更正"。对第二次修订的三份核对在"第二轮复核"一节，被它更正的说法标着"第二轮复核更正" |

## 发现与处置

节号是修订后的。"第 n 条变异"是修订后的 7.3。

### A：S1（模式、建、模板、降级、事件）

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| A-M1 | Minor | `TestTheTemplateRollsBackWithItsNotebook` 分不出失败在 `AGENTS` 还是更早：触发器写错（`NEW.nam`）在第一次插入（`raw`）就抛，键错成 `raw`、组合根的 500 都让测试照过；先例核对触发器自己的话（`cmd/nervewiki/reindex_test.go:144`） | 照改：触发器抛 `'the test refused AGENTS'`，记下的 error 一行（`newAcmeTeamLogging`）带它；另一个序列 `seen_nodes` 数插入，`currval` 是 2（`raw` 写过，回滚收回的是单元已写下的一行）（3.16）（第一轮复核更正：读 `SELECT last_value, is_called FROM seen_nodes`、是 `(2, true)`：`currval` 在测试的连接池上读不了，AC-M1） |
| A-M2 | Minor | `TestPageViolationsCatchEachLLMWikiGap` 以 SQL 软删除 `raw` 同时违反另两条规则（`interleavings_page_test.go:63-66`、`:71-74`）；新规则的 `n.deleted_at IS NULL` 那一半没有反例 | 照改：种子经一次 `POST` 由应用建，再以 SQL 改；断言新规则的一句**在**违反里，照收的三行（普通的、经 `DELETE` 删掉的 LLM Wiki、没改的）**不在**（3.14；第一轮复核更正：删除的那一行放在最后，另加 `raw`、`AGENTS` 改成附件的两行，AC-N5）；第 21 条变异（笔记本的 `deleted_at`）由删掉的那一行抓 |
| A-M3 | Minor | 整个程序的测试"经 wiki 的模块根读模板"，模块根没有导出可读的东西，`bootstrap` 只导入模块根 | 照改：模块根加 `TemplatePages(mode, l) []TemplatePage`（交回副本）与别名 `TemplatePage`，写明是给组合根的测试的（3.8、3.16）（第一轮复核更正：模块根另以常量公开 `ModeLLMWiki`、`LocaleEn`、`LocaleZhCN`，别名不带常量，AC-N1） |
| A-M4 | Minor | 3.1 的表写子目录 `wiki/domain/template/en/…`，3.8、3.9、7.6 是平铺的 `template-en-raw.md`；3.1 缺 `wiki/app/ports.go` | 照改（3.1） |
| A-M5 | Minor | 6.1 的第 4 个提交写"`notebook.New` 的检查"，`WritingJobs` 在第 5 个才有；`publishMode` 不导出、第 5 个才有调用者，第 4 个过不了 `unused`（`server/.golangci.yml:5`） | 照改的目的，做法与 A 的不同（"作者的更正"第 1 条）：第 4 个提交只有播种者的扩展点与 `Seeders` 的检查、`TestTheModuleRefusesNoSeeders`、`noSeed{}`；模式变化的扩展点（`ModeChange`、`ModeSubscriber`、`publishMode`、两个测试）、`WritingJobs` 的检查、`TestTheModuleRefusesNoWritingJobs` 与 `noJobs{}` 都在第 5 个，与它们的调用者（降级）一起（6.1） |
| A-M6 | Minor | `TestTheModeEventReachesTheStreams` 的"第三条什么也没收到（`settle`）"用错了帮手：`settle` 只管开流之前；已开的流要 `quiet`（`events_pages_test.go:67-76`）；`TestTwoAdminsDowngradingAtOnce` 的"只收到一个 `mode`"同样 | 照改：先 `settle` 再开三条流，降级之后在三人都看得到的另一本里写一页作标记，前两条恰一个 `mode` 之后是标记，第三条的下一帧就是标记（3.16）；`TestTwoAdminsDowngradingAtOnce` 照 C 的过度设计去掉（C 的过度设计第 2 条） |
| A-N1 | Nit | `downTo(t, m, 47)` 连 00047 也撤了，应是 `downTo(t, m, 48)` | 照改（3.2） |
| A-N2 | Nit | 四页的大小：英文 `AGENTS` 2,209、中文 2,107、英文 `raw` 648、中文 593 字节，共约 5.4 KiB；草稿写"中文约 2.6 KiB""四页共约 10 KiB" | 照改（3.6、3.9） |
| A-N3 | Nit | `page_search_queue` 没有时刻的列，"搜索的队列的时刻等于 `At`"核对不了什么 | 照改：`TestASeedReadsNoClock` 去掉队列，它留在回滚的那一条里（3.6） |
| A-N4 | Nit | `ListNotebooks`（`store.go:126-131`）与 `ListOwnerless`（`ownerless.go:20-23`）是手写的转换，漏了新列照样编译 | 照改：3.3 写明三种转换与两处手写的，加 `TestTheNotebooksAnswerTheirMode`（五条查询各读一次）；第 14 条变异 |
| A-N5 | Nit | transfer 的假的认证有四个令牌（`session`、`pat`、`bob`、`bobpat`），都要带前缀；`TestClientOfIsTheCredentials` 放在哪个包 | 随 A-Q1 的决定不用了：函数读 `Actor`，假的认证不改；测试在 `shared/client_name_test.go`（3.7） |
| A-N6 | Nit | `git grep "LockByID(ctx context.Context"` 找不到参数没有名字的 `paths_property_test.go:33`、`search_test.go:194` | 照改：写 `git grep "LockByID("`（3.12）（第一轮复核更正：page 的 `Notebooks` 的假的是三个，`search_test.go:194` 是工作区的端口的，AC-N6） |
| A-N7 | Nit | 门经 `WorkspaceOf` 读笔记本的行（`gate.go:35`），只是不读模式 | 照改："都不读模式"（3.12） |
| A-N8 | Nit | `{` 的核对是 `Decode` 的（`payload.go:63`）与 `EventFrame` 的（`frame.go:42-45`），7999 字节对的是整个载荷 | 照改（3.11） |
| A-N9 | Nit | 第 14 条（今第 16 条）"降级不把来源根写成 `NULL`"的两个测试只读用例的 `View`；只改 `SetNotebookMode` 时只有 `TestTheDowngradeAnswersInItsOrder` 抓 | 照改：列出它，矩阵的变体的一行加 `after` 的 `GET`（3.14、第 16 条变异） |
| A-N10 | Nit | 加变异"降级看锁之前读的模式"，`TestTwoAdminsDowngradingAtOnce` 抓它 | 变异加上（第 18 条）；抓它的改为 `TestDowngradeNotebookRefuses` 的一行（找到时 `llm_wiki`、锁下 `standard`），因为整个程序的那个测试照 C 的过度设计去掉（3.10）（第一轮复核更正：假的仓库为此加一张 `locked`，AC-N2） |
| A-N11 | Nit | 模块根的 `adminsAuthorizer`（`extension_test.go:48-62`）对 `notebook.create` 答 `ErrNotVisible`，`TestACreationPassesTheSeeders` 会悄悄断言 404 | 照改：另交 `creatorsAuthorizer`（3.5）（第一轮复核更正：它对 `notebook.create` 一律答 `WorkspaceMember`，读成员关系会拒绝 fixture 里的 alice，AC-N3） |
| A-N12 | Nit | `nb` 用在 `wire.go:205` 与 `:211`；草图漏了 `pd`、`w.page(&pd)` 与 `trDeps.Batch = w.batch` | 照改（3.8 的草图） |
| A-N13 | Nit | S1→S2 的窗口没写：两次合并之间写者可以删除、改名、移动 `AGENTS`，删除 `raw` | 照主持者的决定写在第 0 节"Step 之间的窗口" |
| A-N14 | Nit | 模板的单元的拒绝照原样答会给 `createNotebook` 一个它不声明的 `page.*` 码 | 见 A-Q2 |
| A-Q1 | Q | 定客户端的函数放在平台（读限流的键）还是 `shared`（读 `Actor`，在 `CleanClientName` 旁边） | 主持者的决定第 1 条：`shared`（3.7、2.1 第 1 条、第 8 节 13.1 第 2 条） |
| A-Q2 | Q | 播种者交回的拒绝或 4xx 答 500 还是原样 | 主持者的决定第 2 条：500 的缺陷（3.4 第 9 步、3.5、2.1 第 22 条、第 7 条变异）（第一轮复核更正：`seed` 不另写 ctx 的一支，`TestACreationPassesTheSeeders` 不核对日志，AC-N3、AC-N4） |
| A 过度设计 1 | — | 3.3 把 S2 的 `ShareState`、`LockState`、`ShareNotebook` 写得像 S1 的 | 草稿已在两处标着"S2"（查询的标题与那一段的括号），照旧 |
| A 过度设计 2 | — | `NotebookState` 在 S1 带 `SourceRootID` | A 自己说是对的（端口的返回类型），照旧 |
| A 的保留意见 | — | `TestTwoAdminsDowngradingAtOnce` 超出 00-M10 的清单，但它是"锁下读模式"唯一的测试，留着 | 没有照办（"未采纳"）：主持者照 C 的过度设计去掉它，它要守的那一条由模块一级的一行守着 |

### B：S2、S3（守卫、预检、去处、加序号）

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B-I1 | Important | `TestTheGuardRefusesWhatThePlanLetBy` 走 REST 的 `moveNode`，那条路径没有预检：去掉任何预检它都照过，证明不了 2.1 第 8 条加给 00-M10 的"P1 里已有一种只有守卫拒绝的情形" | 同 C-I4（主持者的决定第 4 条）：第二步是 bob 的 MCP 的 `batch [move raw.md → Notes/raw.md]`，降级落在它的计划与单元之间；`guards: []`（`wiring{page: …Guards = nil}`）的程序里同样的交错让移动成功；随 `batch` 的计划挪进 S2（3.24、6.2 第 4 个提交）；2.1 第 8 条改写：P1 有 `batch` 的与后台撤销两批之间的两种，上传、同步的撤销与 `startRevert` 的到 P2/S2；第 40 条变异（计划的答案交给单元） |
| B-I2 | Important | `TransferRefusal.code` 写成枚举，违背 00-M10 第 5 节与它的审查 C-M23 的"自由的字符串" | 同 C-I3（主持者的决定第 5 条）：`type: string`、说明写出今天的码（3.26 第 6 条）；网页以 `Object.hasOwn` 认得的写它的文字，不认得的写 `problem.other` 带码（4.3），vitest 一行 `page.some_later_code`；存储与 HTTP 的往返收一个自由的码；第 9 节不再让 P2 改枚举；第 52 条变异 |
| B-I3 | Important | "被拒的撤销不占解析的预算"没有能失败的测试：P1 被拒的撤销都不解析什么（模板的变更集只建页，改名没有正文）；没有"`server_busy` 的计数" | 主持者的决定第 6 条取能失败的测试：`TestTheServersLimitsComeAfterThePrechecks`（预算 64 KiB、300 ms，由另一本里等着页锁的大写入占着）：alice 的 `batch [edit Notes.md "x", move raw.md → sources.md]` 写下 C（`Notes` 先有约 1 KiB 的正文），bob 的撤销立刻 403，alice 的撤销 503（3.28、2.1 第 7 条）；第 44 条变异"契约的检查挪到解析之后" |
| B-I4 | Important | TR3、TR4 也以 `toEqual` 比较整个报告，S3 合并不了 | 同 C-I1（主持者的决定第 7 条）：`git grep` 了 `e2e/`，四处 `toEqual` 整个报告（TR1–TR4）都加 `refusal: null`，其余逐个字段的核对列在 3.26 第 6 条（3.26、5.1、6.3） |
| B-M1 | Minor | `TestTheModeGuardReachesEveryPath` 照草稿的次序，两次撤销都答 `page.revert_conflict`（撤销的计划先核对每个碰过的节点，`revert.go:450-457`） | 照改：模板的撤销放在最前，bob 撤销 alice 的改名紧跟那次改名，alice 撤销它作反向对照（200）（3.24；另见"作者的更正"第 3 条） |
| B-M2 | Minor | S3 的计划再问一次授权者，`TestTheModeGuardAsksOnceAUnit` 数的变了 | 主持者的决定第 8 条取第一种："一个单元问一次"数单元里的问，计划另问、答案不交给单元；测试只数事务里的问，计划的问由 `TestABatchsPlanRefusesBeforeItsLock` 数（3.19、3.22）；2.1 第 24 条（第一轮复核更正：`TestTheModeGuardAsksOnceAUnit` 挪到 `page/app`，以假的存储与授权者数 `" in tx"` 的问，B 的轻的意见；第二轮复核更正：假的授权者放行 `page.manage_source`，两次移动都写下，B3-N1） |
| B-M3 | Minor | `batch` 的 `Needs` 在 `a.read` 之后才问：后一项的 `CheckReads` 的上限或 `differs` 可以先于前一项的 403 | 照改：拒绝在 `PlanBatch` 之后、`a.read` 之前记下，`BatchPlan.Refuse` 让 `Reads`、`CheckReads` 停在它之前（3.22；"作者的更正"第 2 条）；`TestABatchsPlanRefusesBeforeItsLock` 加两行（`Big` 过了读的上限、`Big` 不见了）；第 39 条变异；13.1 第 4 条的新文字写明码按项的次序 |
| B-M4 | Minor | "被拒的上传不写文件"不会失败（守卫拒绝之后上传也删文件）；`storage_full` 之前的次序在整个程序里没有核对 | 照改：`TestTheServersLimitsComeAfterThePrechecks` 的另一半（`MinFreeBytes = 1 << 60`：`index` 409、`index.png` 507）；那一句从 `TestPrechecksAnswerTheGuardsCodes` 去掉（3.28）；第 42 条变异由它抓（第一轮复核更正：照 C 的意见去掉这一半，次序是结构的，由 `asset/app/upload_test.go` 的一行核对，第 42 条变异由 `TestAnUploadsCheckRefusesAReservedTitle` 抓；第二轮复核更正：那一行是 `TestCheckDecidesTheNodeThenTheFreeSpace` 的一种，"不调 `Store`"挪到 HTTP 适配器的 `checkErr` 一行，B3-N2） |
| B-M5 | Minor | 加序号的测试的期望与导入的计划（文件夹并进同键的页，`import_plan.go:191-197`）、后台的恢复（一个任务只有一个根落到根下）不合 | 照改：库是 `index.md`、`Index/a.md`、`log/b.md`，得 `index 2`（`a` 在下）、`log 2`（`b` 在下）、两个 `renamed` 问题；后台的恢复一项，`counts.renamed` 是 1（3.27、3.28）（第一轮复核更正：两次恢复在导入之后的同一本里，得 `log 3`、`index 3`，B2-M1） |
| B-M6 | Minor | 第 28 条（今第 34 条）变异与 `TestTheModeGuardLetsAStandardNotebookBy` 不会失败：`CheckContract` 自己对普通的笔记本答零 | 同 B-O2，主持者的决定第 9 条取"能失败"：测试交 LLM Wiki 会拒的步，变异改为"守卫与 `CheckContract` 都不看模式"；单去掉守卫的第一行是等价的变异，不列（3.20、第 34 条变异）（第一轮复核更正：不核对"授权者一次也没被问"，它在 P1 与 P2 都失败不了，B 的轻的意见） |
| B-M7 | Minor | 照机理，排序来源根的祖先也让来源根出现在 `OpMove` 里、要管理员；R4 的文字说不要 | 主持者的决定第 3 条（B-Q1）：允许。R4 改为"来源根是第一个节点，或在其余节点里而第一个节点换了父节点"；表分出"移动到别的父节点""同一父节点下排序"两行；`TestCheckContract` 加四行；`TestTheModeGuardReachesEveryPath` 加"bob 排序 `Notes` 200、把它移到别处 403"（3.21、3.24、2.1 第 21 条、第 31 条变异） |
| B-M8 | Minor | S2→S3 另有两个窗口没写：MCP 的 `batch` 先答 `page.locked` 后答 `page.protected`；后台撤销的 `internal` | 主持者的决定第 4 条：`batch` 的计划的预检挪进 S2，前一个窗口由此没有；别的窗口照收，与 A-N13 的一起写在第 0 节（2.1 第 20 条） |
| B-M9 | Minor | 两批之间的测试没写输的那一支的断言、每次重试要新的笔记本、C 与 bob；`behindFirstBatch` 的行号与降级的路由写错 | 照改：照 `TestARevertJobAndAPageCreatedUnderATopItDeletes` 的循环，输的那一支核对任务整个成功（`put_back` 2、`raw` 叫 `raw`），`betweenUnits` 次都输时 `t.Fatalf`；`background_test.go:195-211`（第一轮复核更正：`:196-210`，B2-N5）；`PATCH /api/v0/notebook-members/{成员关系的 id}`（3.28、7.2） |
| B-N1 | Nit | `TitleKey` 是 Unicode 的折叠，不是小写；全角的 `ｉｎｄｅｘ` 不保留 | 照改（3.21） |
| B-N2 | Nit | `revertChecks.check` 的那一行漏了步数的 422 `page.revert_too_large` | 照改（3.25 的表，`TestARevertsChecksRefuseTheContract` 加它的次序） |
| B-N3 | Nit | `revertChangeset` 的新码列在 `server_busy` 之后，与 2.1 第 7 条不一致 | 照改：放在 `server_busy` 之前（3.22） |
| B-N4 | Nit | asset 与 transfer 不导入 page，适配器测试的行要自己造问题 | 照改（3.22、3.25） |
| B-N5 | Nit | 计划的预检把 `ErrNotVisible` 答成 403、把数据库的错误标成拒绝 | 照改：`ErrNotVisible` 答这个操作的 404（`notebook.not_found`），数据库的错误原样（3.22 的代码）；两行测试；第 41 条变异 |
| B-N6 | Nit | 每次上传、`batch`、撤销的检查多一次 `StateOf`，普通的笔记本也是；7.6 的"预检的规则不读库"要写明；钉住调用列表的测试要改 | 照改（3.22、3.25 的"读的调用"、6.2、6.3、7.6） |
| B-N7 | Nit | `TestTheGuardSeesTheCheckedChange`（`write_test.go:309-311`）以 `s.Write != want` 比较，`Grants` 的新指针让 S2 的第一个提交失败 | 照改：先 `want.Grants = s.Grants` 再比（3.17、3.18、6.2 第 1 个提交） |
| B-N8 | Nit | "S2 只碰 page（与 access 的一行）"不对 | 照改（第 0 节，同 C-N1） |
| B-N9 | Nit | `page.source_admin_only` 的网页的文字只说来源区，P1 唯一的情形是来源根自己（不在来源区里） | 照改：两种语言的文字写出来源根（4.2） |
| B-N10 | Nit | S3 的第 1、2 个提交之间，任务的计划拒绝的任务报成 `forbidden` 或 `internal`，要说"开始的计划与每一批的" | 照改：每批的检查去掉之后，写明是任务的计划的那一种，只在分支上（6.3） |
| B-Q1 | Q | 写者排序子树里有来源根的页要不要 `page.manage_source` | 主持者的决定第 3 条，见 B-M7 |
| B-Q2 | Q | S2→S3 的窗口照收，还是把 `batch` 的计划的检查挪进 S2 | 主持者的决定第 4 条，见 B-M8 |
| B-Q3 | Q | 若要枚举，主持者是否推翻 C-M23 | 主持者：不必回答（B-I2 定为自由的字符串） |
| B-O1 | 过度设计 | `RevertBatch` 的每批的检查：一批是一个单元，守卫以同样的码与节点拒绝、整个回滚 | 主持者的决定第 10 条：去掉（3.25、3.26、3.28，2.1 第 19 条改 00-M10 的十处；第一轮复核更正：十一处，加 `:181` 与 `:191` 的括号里的 `CheckRevertPart`，B2-N1 = C2-M2） |
| B-O2 | 过度设计 | `TestTheModeGuardLetsAStandardNotebookBy` 照草稿写的 | 见 B-M6 |
| B-O3 | 过度设计 | `Grants.Ask` 记下数据库的错误买不到什么 | 主持者的决定第 11 条：只记允许与拒绝（3.19，`TestAGrantIsAskedOnceAUnit` 一行，第 33 条变异） |

### C：网页、e2e、测试、Step 与完整性

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C-I1 | Important | 同 B-I4 | 同 B-I4 |
| C-I2 | Important | 树的文字等价在 Playwright 里的可读的名字不同：`sr-only` 是 `position: absolute`，算作 `display: block`，Playwright 给不是 `inline` 的子元素两边补空格（`innerAccumulatedElementText`），名字成了 "raw ，来源区"；jsdom 没有 Tailwind，vitest 照过、e2e 等不到 | 主持者的决定第 12 条：照 `NewTab`（`attachment-row.tsx:143-152`），真的空格在 `sr-only` 之外、后缀以括号起头；确切的名字 `raw (source area)`、`AGENTS (instructions for agents)`、`raw （来源区）`、`AGENTS （给 agent 的约定）`；LW1 (page)、LW2 (page) 按它们找，这就是 Chromium 一侧的核对，4.4 写明 jsdom 核对不了（4.4、5.2、2.1 第 18 条、第 61 条变异）（第一轮复核更正：空格放进 `sr-only` 里两边的名字都不变，第 61 条抓不到；jsdom 的 `textContent` 看得到去掉的 `{" "}`（今第 62 条变异），只有 e2e 看得到被藏出可访问性的树的后缀（今第 63 条），C2-M1；理由只在 4.4 写一次） |
| C-I3 | Important | 同 B-I2；另：存下的报告由下一个版本读时可能带枚举之外的码 | 同 B-I2 |
| C-I4 | Important | 同 B-I1；另：`Unit.Revert` 的锁下的一遍在守卫之前，同步的撤销代替不了 | 同 B-I1；3.24 写明 S3 之后上传与撤销的那几行由预检答出，它们上的守卫的最后一跳等 P2/S2 |
| C-M1 | Minor | S1 的第 5 个提交播下 `wlab`，而手写的 `accountNotebooksOf` 要第 7 个才改，第 5 个是红的；"由种子算出"不对 | 照改：第 5 个提交手写加上 `wlab` 的笔记本，第 7 个只加 `mode`（6.1、3.12、3.14、2.1 第 11 条） |
| C-M2 | Minor | S4 的第 1 个提交的 LW1 (page) 找第 4 个提交才有的名字 | 照改：树的提交排到最前（6.4） |
| C-M3 | Minor | `notebook.store.test.ts` 的那一行只比名字，降级不改名字，抓不到 | 照改：自己的测试，比 `[name, mode]`（4.4）；第 56 条（第一轮复核之后第 57 条）变异 |
| C-M4 | Minor | 设置的那一节的 vitest 没有列出；第 45 条（今第 57 条，第一轮复核之后第 58 条）的测试在 `notebookServer` 只在成功时改状态时照过 | 照改：4.4 列出五条，403 的那一条在答 403 之前把 Ada 改成编辑者（4.4） |
| C-M5 | Minor | `draggable()` 总是写 `draggable="true"`、只在 `dragstart` 问 `canDrag`，属性分不出两种行 | 照改：不能拖的行不登记 `draggable()`，测试核对属性没有（4.4、5.2）；第 60 条（第一轮复核之后第 61 条）变异 |
| C-M6 | Minor | LW12 (page) 的"写者没有降级的按钮"在降级之后核对，不会失败 | 照改：在降级之前核对（5.2） |
| C-M7 | Minor | 7.5 的估计没算竞态检测与 CI 与本机之比（1.73），7.6 的量法不带 `-race`；照 CI 的每个测试约 3–3.5 秒，第二份会到约 7:00 | 照改：7.5 照 CI 的平均重估（第二份约 7:00–7:15、第三份约 6:10–6:20），两个 S3 的测试改名落进第三份；7.6 带 `-race`、乘 1.73（第一轮复核更正：第三份以两次 run 的平均约 5:41 作底、去掉磁盘的一半，约 6:20–6:30，C2-N1） |
| C-M8 | Minor | 第 8 节少了 00-M10 列的三处 13.1 的修订（整组撤销的临时名称；导入的单元；第 21 条的行为测试的守卫与去处的最后一跳） | 照改（第 8 节） |
| C-M9 | Minor | M5-events 在 P1 做完之后应改 `done`（00-M10 第 7 节，移交自己的第 14 行） | 照改：`docs: M10/P1 done` 改它（第 8 节、6.4） |
| C-M10 | Minor | 第 8 节没说接受的提交与每次合并写 00-M10 的头、进度表与变更记录的哪些 | 照改，照 M9 的写法（第 8 节第 4 条） |
| C-M11 | Minor | 持有的对话框代替 4.5 的 `focusAfter`，2.1 没有一条 | 照改：2.1 第 23 条（含 404 的重读） |
| C-M12 | Minor | 第 9 节的 M11、M12 的项没有放进移交 | 照改：M11 的在 `docs: M10/P1 done` 写进新的 `M11-upgrade/handoffs/M10-mode.md`；M12 的图标本就在 00-M10 的打磨列表里、收尾时追加，`ModeOptions` 的那一项去掉（第 8、9 节） |
| C-N1 | Nit | 同 B-N8 | 同 B-N8 |
| C-N2 | Nit | `downgradeNotebook` 照 `updateNotebook` 应交回原样的答复，LW12 (API) 要 409 与 403 | 照改（5.1） |
| C-N3 | Nit | 导出 `getJob` 会被 knip 报出 | 照改：不导出（5.1，C 的过度设计第 1 条） |
| C-N4 | Nit | `pageServer` 的列表的路由什么也不记，`server.sent` 是 `[]` | 照改：测试经 `answers` 换上一个记下读的答复（4.4） |
| C-N5 | Nit | `test.use({ locale })` 的那一句是 S4 的 | 照改：放进 S4 的文档提交（第 8 节 13.4 第 3 条） |
| C-N6 | Nit | 时间混了作业与测试一步 | 照改：分开写，7:30 的线是测试一步的（第 1 节、5.2、7.5） |
| C-N7 | Nit | 第 5 条变异 `TestACreationPassesTheSeeders` 看不到；第 28 条（今第 34 条）"建事实"那一半看不到 | 照改（第 5、34 条变异） |
| C-N8 | Nit | 提交的范围漏了改到的部件 | 照改，取 README 的部件为范围（`server`、`api`、`web`、`e2e`，"作者的更正"第 5 条）（第 6 节） |
| C-N9 | Nit | P2 的拒绝可以指附件，`transfer.refusedSome` 与 `distinctName` 不认附件 | 照改：交给 P2/S2（第 9 节） |
| C-N10 | Nit | 上级文档漏了 4.18（`AGENTS` 至多 8 KiB） | 照改（头、3.9） |
| C-N11 | Nit | 第 6 节的写法漏了合并之后的修复的核对与旁支 | 照改（第 6 节） |
| C-N12 | Nit | `behindFirstBatch` 在 `background_test.go:195-211` | 照改（3.28、7.2，同 B-M9）（第一轮复核更正：`:196-210`，B2-N5） |
| C-N13 | Nit | `treeTitles` 读 `allTextContents()`，按确切名字找链接的是 `treeRow` | 照改（4.4） |
| C-N14 | Nit | LW1 (page) 是两个测试 | 照改（5.2） |
| C-Q1 | Q | 同 B-Q3 / B-I2 | 同 B-I2 |
| C-Q2 | Q | 同 B-Q2 | 主持者的决定第 4 条 |
| C 过度设计 1 | — | `getJob` 的导出 | 照收（C-N3） |
| C 过度设计 2 | — | `TestTwoAdminsDowngradingAtOnce`：两半都已有别的测试 | 照收，主持者的决定第 13 条：去掉；"锁下读模式"那一半由 `TestDowngradeNotebookRefuses` 的一行守着（C 说的调用次序看不出用例取了哪一次读的模式，所以另加这一行），"一个 `mode`"由 `TestTheModeEventReachesTheStreams` 守着（3.10、3.16、7.2） |
| C 过度设计 3 | — | 持有的对话框外面的 `(downgrades \|\| open) &&` | 照收：一直挂着（4.4） |
| C 过度设计 4 | — | M12 的"`ModeOptions` 与 `AccessOptions` 合成一个组件" | 照收：去掉（第 9 节） |

**2.1 逐条**（C 对着 `d7bb5e8d` 的 00 号文档核对 18 条）：没有一条与第 11 节的决定相悖；两处与审查过的 00-M10 不同而没有一条（C-I3、C-M11）：前者改回自由的字符串，后者加第 23 条。第 11 条（C-M1）与第 18 条（C-I2）改写。

## 重叠

- B-I1 = C-I4（只有守卫拒绝的情形走了没有预检的路径）；
- B-I2 = C-I3（`TransferRefusal.code` 的枚举）；
- B-I4 = C-I1（TR3、TR4 的整个报告）；
- B-Q2 = C-Q2（Step 之间的窗口），B-M8 是它的 Minor 的那一面；
- B-N8 = C-N1（"S2 只碰 page"）；
- B-M9 的行号 = C-N12；
- B-M6 = B-O2；B-Q1 是 B-M7 的问题，B-Q3 随 B-I2 消失；
- A-N14 是 A-Q2 的 Nit 的那一面；A-N13 与 B-M8 一起写进第 0 节；
- A-N10 与 C 的过度设计第 2 条说的是同一个测试，处置一并定（主持者的决定第 13 条）。

## 主持者的决定

主持者在修订的简报里定下的处置（作者照做；负责人可以改判）：

1. **A-Q1：定客户端的函数是 `shared` 里的一个帮手**，在 `shared.CleanClientName` 旁边，读 `Actor`：输入与今天的三份相同，没有 `Actor` 照旧 401，"PAT 是 `api`"不进平台；一份，四个适配器调它、各自转换为自己的类型。2.1 第 1 条与第 8 节 13.1 第 2 条的新文字随之。
2. **A-Q2：播种者的有类的错误是缺陷**：创建答 500，日志记下它的码。模板是服务端的，守卫不能给 `createNotebook` 一个它不声明的码（M11 的冻结的也不）。ctx 的取消与期限照原样，同别处。以一个交回 `page.*` 错误的假的播种者测它。
3. **B-Q1：写者可以排序来源根的祖先**。排序不让来源根换位置，来源区不换位置（00-M10 4.7 的理由）。R4 只在来源根自己是被移动的节点（移动、排序、改名）或它的父节点的一线变了（一个祖先移到别的父节点下）时要 `page.manage_source`；R4 的文字与步 × 节点的表照此写；`TestCheckContract` 加行，`TestTheModeGuardReachesEveryPath` 加一步：写者排序 `Notes`（`raw` 在下）200，再把 `Notes` 移到别的父节点下 403。
4. **B-Q2 = C-Q2：`batch` 的计划的树的一遍的 `CheckContract` 挪进 S2**：它小，没有它 MCP 的 `batch` 先答 `page.locked`、后答 `page.protected`（B-M8a），与 REST 与 MCP 的 `delete` 不同。别的窗口照收：S1→S2（`AGENTS` 与 `raw` 没有保护，A-N13）、S2→S3（加序号、后台撤销报成 `internal`）。在第 0 节一起写明；每个窗口由下一个 Step 关上；Step 一个接一个合并，中间不合并别的、不发布。
5. **B-I2 = C-I3：`TransferRefusal.code` 是自由的字符串**，照 00-M10 第 5 节与它的审查 C-M23（M11 的冻结的码不改契约）；网页认它知道的码、别的有退路的文字；第 9 节不再让 P2 扩一个枚举。
6. **B-I3：解析的预算**：把"被拒的撤销不占解析的预算"做成能失败的测试（例如 B 的形状：变更集也写回另一页的正文、预算设得让解析答 503、被拒的撤销答它自己的码），或去掉这个说法、只留码的次序与它的测试；写明取哪一种。作者取前一种。
7. **B-I4 = C-I1**：TR3、TR4 在 S3 加 `refusal: null`，`e2e/` 里每个对报告的 `toEqual` 同样；grep 出来、列出。
8. **B-M2**：在"一个单元问一次数单元里的问、计划的另算"与"计划把答案交给单元"之间选一种、写明，让 `TestTheModeGuardAsksOnceAUnit` 与之相合。作者取前一种（理由在 3.19：交给单元就让计划与单元之间被降级的执行者照计划时的权限写下去）。
9. **B-O2 = B-M6：普通的笔记本的测试**：做成能失败的（例如数守卫在普通的笔记本上什么也不读、什么也不问），或连同它的变异去掉；写明取哪一种。作者取前一种，形状见"作者的更正"第 9 条。
10. **B-O1：去掉 `RevertBatch` 的每批的契约的检查**：一批是一个单元，守卫以同样的码与节点拒绝，单元回滚，草稿自己的反向对照显出结果相同；后台的路径要的是 `revertDefect` 认出 `Refused`。两批之间的降级的测试由此显出后面一批的守卫的拒绝。在 2.1 记作对 00-M10 的 P1 一行（"后台每批的 `CheckRevertPart`"）的改动，写明这个理由。
11. **B-O3**：`Grants` 只记允许与拒绝，不记数据库的错误。
12. **C-I2**：照 `NewTab` 的写法（`attachment-row.tsx:143-152`）：真的空格在 `sr-only` 的 `span` 之外，文字如"（来源区）"/"(source area)"；写出 LW1、LW2 (page) 找的确切的可读的名字；加 Playwright 一侧的核对（e2e 的一步，或写明名字只在 e2e 里核对：jsdom 没有 Tailwind）。
13. **C 的过度设计**：照 C 的建议收下（`getJob` 的导出、`TestTwoAdminsDowngradingAtOnce` 与其余），除非去掉的是 00-M10 4.17 或第 9 节要的检查，那时留着、写明。四条都不是（`TestTwoAdminsDowngradingAtOnce` 不在 4.17 的清单里，第 9 节也没有要它），都收下。
14. **其余的 Minor 与 Nit 照审查者的建议**，除非审查者错了，那时写明。A-M5 与 C 的提交次序（S1 的第 4、5、7 个提交，S4 的第 1 个）：6.1–6.4 的每个提交单独核对绿（linter 的 `unused`、矩阵手写的 `accountNotebooksOf`、LW1 的页面的名字），需要时重排。C 的第 8 节的缺口：三处 13.1 的修订、M5-events 在 P1 完成时 `done`、00-M10 的变更记录、进度表与头、持有的对话框代替 `focusAfter` 作 2.1 的一条、M11 与 M12 的项放进移交或写明留给收尾。
15. **另**：第 7 节的变异对着每处改动重核（需要时重新编号，每条有抓它的测试）；份的落点与 7.5 的估计照竞态检测与 C 的 CI 之比重核；2.1（加的、改的）、2.2（应当照旧为空）与附录随之。

## 作者的更正

1. **A-M5 的做法**：A 建议 `TestTheModeSubscribersRunInTheirOrder` 随 `publishMode` 落在第 4 个提交；修订的 3.5 让它经 `DowngradeNotebook` 跑（同 `TestTwoVisibilitySubscribers` 经用例），用例在第 5 个提交。所以整个模式变化的扩展点挪到第 5 个提交、与它的调用者一起，第 4 个只有播种者的；`unused` 照样安静。
2. **B-M3 要一个 domain 的方法**：只在应用里设 `Failed` 不够：`Contents` 照 `Failed.Op` 停（`batch_contents.go:40-44`），`Reads` 与 `CheckReads` 却走树的一遍记下的 `p.order`，`Needs` 不结束那一遍，所以后面的项的页还在里面。`BatchPlan.Refuse(at, err)` 设 `Failed` 并去掉第一项在 `at` 或之后的页。
3. **`TestTheModeGuardReachesEveryPath` 的普通的那一本**：照 B-M1 重排时发现草稿的"普通的笔记本里同样的请求全部成功"对一个序列不成立：草稿先删 `Notes`（`raw` 在下）、再删 `raw`，普通的那一本里后一个是 404；bob 的撤销在普通的那一本里成功，alice 再撤销就冲突。修订把删除放在最后、依次 `AGENTS`、`raw`、`Notes`，普通的那一本不做 alice 的撤销与模板的撤销（3.24）。
4. **计划的 404 是 `notebook.not_found`**：B-N5 说照 `authorize` 答这个操作的 404；`batch` 的单元的 `NotFound` 是 `domain.ErrNotebookNotFound`，所以是它，不是 `page.not_found`。
5. **C-N8 的范围**：C 要求把漏了的部件加进范围；草稿的范围是模块名（`notebook`、`page`），而 `docs/README.md:100` 与 00-M10 自己的例子（`server: add the notebooks' mode (M10/P1/S1)`）的范围是部件。修订改用部件（`server`、`api`、`web`、`e2e`）。
6. **A-N5 不用做**：随 A-Q1 的决定，函数读 `Actor`，假的认证的键不再要紧，各模块的适配器的测试不改。
7. **中文的后缀**：C 举的是英文的 "(source area)"；中文照 `asset.newTab` 的"（在新标签页打开）"写全角的括号，可读的名字是 `raw （来源区）`（外面的空格照样在）。
8. **份**：照 C-M7 的读法重估，第二份到约 7:00–7:15，离线太近；修订把两个 S3 的测试改名为 `TestTheServersLimitsComeAfterThePrechecks`、`TestTheReservedTitlesAreNumberedAtAnLLMWikisRoot`，落进余量多的第三份（第二份 8 个、第三份 11 个）。
9. **普通的笔记本的守卫的测试**：单去掉守卫的第一行在 P1 是等价的变异（`CheckContract` 自己看模式，普通的笔记本没有来源根），数"什么也不读、什么也不问"抓不住它；能失败的是交给守卫 LLM Wiki 会拒的步，变异改为两处的模式的核对都去掉。P2/S1 的守卫读连接池之后，计数的存储才有意义，交给 P2/S1（第 9 节）。
10. **不认得的码的文字**：B 建议只写 `transfer.failure.refused` 的那一句，C 建议照 `errorText` 写 `problem.other`；取 C 的，它把码说出来。
11. **2.1 第 24 条**：B 在"核对过、没有问题的"里指出，草稿让单元自己的锁下的检查也写 `Grants`，扩了 00-M10 4.6 的"只有模式的守卫写它"；修订把它记成 2.1 的一条。
12. **2.1 第 19 条的范围**：去掉每批的检查不只改 00-M10 的 P1 一行：第 1 节第 3 条、第 3 节第 2 条、4.6 的表与两段、第 7 节的 P1、P2 两行与"大小与绿"、移交的落实的 M8-history 一行、第 9 节的"后台"都写着"每批的预检"，十处一起列出（第一轮复核更正：漏了 4.6 的表的 `:181`"每批的检查照留"与"transfer 的任务"的括号里的 `CheckRevertPart`，是十一处，B2-N1 = C2-M2）。P2 要一批在读写之前的检查时由 P2 的设计加回。
13. **B-I3 的种子**：B 的 `batch [edit Notes "x", move raw.md → sources.md]` 要 `Notes` 在 C 之前有正文，C 的撤销才写回、才解析；修订写明约 1 KiB。
14. **播种者的缺陷不以 `%w` 包**：包了，`ProblemOf` 经 `errors.As` 照样找到问题、答它的码（`apierrors.go:260-262`），所以码只进文字，不带 `Detail`（守卫的说明里有页的名称）。
15. **修订自己的核对**（一位 Opus 的核对者对着修订稿与这份记录读了一遍，只读）：找到十三处修订没有改到的地方，都改了：7.2 的交错的表与 7.3 的标题在重排变异的时候被一并换掉（恢复，七行）；3.28 的两个改名的测试的份写成"二"（改为"三"）；3.26 第 7 条的退路的文字是草稿的（改为 `problem.other`）；3.19 的 `answers`（改为 `allowed`）；3.22 的 `moveNode` 的说明没随 R4 改（"under another parent"）；3.5 引的 2.1 的条号（第 3 条，草稿里就错了）；四个变异点名的测试在第 3–5 节没有写（`TestAUnitMarksItsGuardsRefusals` 写进 3.22 与 3.17，`TestThePermissionMatrixCoversEveryOperation` 写进 3.14，`TestABackgroundRevertAndAnEditSession` 写进 3.26，建的调用次序写进 3.5）；不认得的码的变异是 S3 的（挪到第 52 条，S4 是第 53–61 条；第一轮复核之后 S3 到第 53 条、S4 是第 54–64 条）；三个改契约的提交的范围漏了 `web`（它们重新生成 `schema.gen.ts`）；3.29 说锁下的一遍"没有锁"（改写，并给它一行模块测试：`TestARevertsChecksRefuseTheContract` 的锁下的一行）；两处表里的空行；第 0 节的 S3 漏了 asset；2.2 的条号的分组与 7.5 的"两个交错"（是三个）。核对者为了比较测试名在 `scratchpad/m10/` 写过两个临时的清单、随即删掉，此外没有写什么。

## 审查者核对过、没有问题的

**A**（S1）：00048 的 SQL 在一条 `ALTER TABLE` 里成立（同 00034 的写法），Down 的次序、不对称的检查、两个名字在 `want` 里的位置、三个反例，"38 处"的数目；sqlc 的一项（`sqlc_test.go:267-271`），wiki 不要一项；五条查询是仅有的建 `domain.Notebook` 的读，`gen.FindNotebookRow(r)` 的两处转换；`SourceRootID` 是值；建的事务的每一处重入（`WithinTx` 加入、`postgres.DB` 经外层的事务、工作区行的 `FOR SHARE` 与刚插入的笔记本行的锁都不等、授权者没有缓存），没有升级的锁、没有死锁；提交得了的 `llm_wiki` 都有 `raw` 与 `AGENTS`（回滚或启动时 panic）；一个时钟（`unit.go:230` 是唯一的读）；`Seed`；两个扩展点；`ClientOf` 的三份（`api.go:350-367` 的键与 identity 的写法一致）；wiki 的骨架（`gochecknoglobals` 豁免 `embed`）；模板的两种语言逐块对应（`review-A/blocks.py`）、代码之外没有链接；降级的锁序与 `ImportWorkers 1`；事件；MCP（`mcp_instructions_test.go:188-195`）；契约（`operations.go:200-260`）；矩阵的变体（除 C-M1）；份；提交的次序（除 A-M5）。2.1 第 1–4、9–11、15 条对着代码都对。

**B**（S2、S3）：每条写入路径到单元的步的种类与 3.21 的表一致（`writer.Run` 的 21 处调用，每一步至少一个改动，`Unit.Participate` 只给参与者）；一步里同时移动与改名、改名为 `index.md`、排序、兄弟的重新编号、撤销放回已删的 `AGENTS`、恢复进根、叫 `log.md` 的附件、大小写与 Unicode 的折叠；2.1 第 4 条（page 的端口的唯一调用者是 `Writer.Run`）；`Write.Notebook` 在每种单元里都在排斥降级的锁下读；`Grants` 是指针的理由；登记与命令行（`pageSessionSubscribers`）、`[mode, lock]` 由 `TestModeComesBeforeTheLock` 的两半钉住；`Refused` 的解开（仓库里没有对 `*shared.Error` 的类型断言或 `==` 比较）；`failed` 先认 `Refused`；六个操作的码与它们的适配器测试、MCP 的码；2.1 第 7 条的两个位置；`ErrLocked` 是 `treeError` 的最后一个、导入的错误经它两次所以要幂等；`unitFailure` 的次序；撤销的计划把 `raw` 排过第一批（`revert_schedule.go`，`Batch{Nodes: 1}`）；交错 6（今 5）；加序号的位置（`free`、`restoredNames`、`under`）；S2→S3 的加序号的窗口可以接受；份；第 19–27、29–41 条变异（旧号）。

**C**（网页、e2e、测试、Step、完整性）：S1 的 `Notebook` 的两处完整的字面量是仅有的（其余的展开 `notebookJSON`，e2e 没有），`notebook.not_llm_wiki` 的文字随 `downgradeNotebook`，MC1 的 `toEqual` 随 MCP 的 `mode`；S2 的三个码的文字在守卫的提交；S3 的 `refused` 的类型表与 `reportOf`；生成的类型经 `export type *` 对 knip 不可见；建的对话框的请求体（`preferences.locale`，jsdom 的语言是英文，中文偏好的测试抓第 58 条，第一轮复核之后第 59 条）、新手引导不变；设置的"常规"页（`transfer.writingBusy` 是合适的文字，`texts` 有类型，持有的对话框的理由属实，卸下时 Radix 照样调 `onCloseAutoFocus`，那一页今天没有可聚焦的标题）；`NotebookStore.downgrade`（`inTurn`、照 `leave` 重读，404 不进 `removed` 时布局显示找不到）；`mode` 的处理（键与 `refreshedOnConnect`，替身要改成 `later`）；树（菜单照 4.9，`titleKey`，图标在 lucide-react 1.48.0 里）；e2e 的夹具与模板的核对，LW12 的忙的做法是确定的（`StartJob` 在它的单元等锁之前把 B 标成 running，`import_workers: 1`，每个 worker 一个服务）；`settleEvents` 在 `openEvents` 之前；`holdStream`/`countAnswers` 的证明；10.1 的每个故事的 MCP 规则与 LW12 的豁免；19 个名字的份（第二份 9、第三份 10，第一份没有）；交错 1–6 与 8 逼得出（7 是 C-I4）；抽查的第 2、7、8、10、13、16、18、19、20、21、24、27、31、36、38、40、42–47 条变异（旧号）；第 8 节引的 v0.1 的行与旧文字都在，`docs/README.md:104` 已有 M10 的一句；第 9 节交出的各项；附录抽查的行（除 C-N12）。2.1 的 18 条逐条对着 `d7bb5e8d` 的 00 号文档都是真的，没有一条与第 11 节相悖。

## 未采纳

- **A 的"留着 `TestTwoAdminsDowngradingAtOnce`"**：主持者照 C 的过度设计去掉它（00-M10 4.17 与第 9 节都没有要它）。A 要它守的"锁下读模式"由模块一级的一行守着（找到时 `llm_wiki`、锁下 `standard`），比交错便宜、确定；第 18 条变异由它抓。
- **A-M5 的原样的做法**：见"作者的更正"第 1 条，目的照做、做法不同。
- **B-M2 的第二种**（计划把答案交给单元）：没有取，理由在主持者的决定第 8 条与 3.19。
- **B-I3 的第二种**（去掉"不占预算"的说法）：没有取，做成了能失败的测试。
- **B-M7 的"拒绝"的那一支**：主持者定为允许（主持者的决定第 3 条）。
- **B-M6 与 B-O2 的"去掉测试与变异"**：没有取，做成能失败的。
- **B-M8 的"三个窗口写在 3.21"**：照主持者的决定写在第 0 节，`batch` 的那一个由挪进 S2 消掉。
- **B 的退路的文字**（只写 `transfer.failure.refused`）：取了 C 的 `problem.other`（"作者的更正"第 10 条）。
- **C-M2 的第二种**（第 1 个提交写不带后缀的名字、第 4 个再改）、**C-M3 的第二种**（表的每行一个投影）、**C-N2 的第二种**（另加 `postDowngrade`）：取了各自的第一种，改得少。
- **C-M12 的"留给收尾"**：M11 的项不留，`docs: M10/P1 done` 写进新的移交；M12 的图标的那一项本就在 00-M10 的打磨列表里，照它留给收尾一起追加。
- 没有一条审查者的建议被判为错的。

## 新的问题

没有要负责人现在定的。两件实施时要看的事：

1. **第二份的余量**：照 C-M7 的读法，改名之后第二份估计在 7:00 上下（6:12 加约 45–60 秒，含 `TestPermissionMatrix` 的约 30 个变体的格子）。每个 Step 合并之后照 M9/P2 6.5 的规则读两次 CI；过线时第三份有约一分钟的余量可以收下一段。
2. **C-I2 的依据是读代码**：Playwright 给非 `inline` 的子元素补空格是 C 读 `playwright-core` 的 `innerAccumulatedElementText` 得的，没有跑。修订的写法（空格在外、后缀以括号起头）在两种算法下给同一个名字，并有 `NewTab` 与 AS3 的 e2e（`as3-render.spec.ts:322`）作先例，所以不依赖这一读法是否完全准确；S4 的 LW1 (page) 是第一次实地的核对（第一轮复核更正：C 照这一算法算了两种写法，空格放在 `sr-only` 里外名字相同；只有 e2e 看得到的是被藏出可访问性的树的后缀，C2-M1）。

## 第一轮复核

| 项 | 内容 |
|---|---|
| 复核对象 | 修订后的 [01-P1-contract-frame.md](../01-P1-contract-frame.md)（2026-10-24，312,264 字节、2,423 行；复核时的样子留作 `01-P1-contract-frame.rev1.md` 与 `parts-rev1/`，这份记录的样子留作 `P1-design-review.rev1.md`）与上面的处置，对照 `main` `d7bb5e8d` 的代码与 00 号文档；修订照的简报 `revise-brief.md` |
| 复核方式 | 三位审查者（Opus）各复核自己的一份，对着草稿逐部分比（`parts/` 对 `parts-draft1/`），只读，不跑测试、构建、Docker 与数据库，不下载：<br>A：S1（第 0 节的 S1、第 1 节、2.1 第 1–4、9–11、15、22 条，3.1–3.16、4.1、6.1，第 7 节的 S1 的行与第 1–22 条变异）。<br>B：S2、S3（2.1 第 4–8、14、16、19–21、24 条，3.17–3.29，4.2、4.3，第 6、7 节的 S2、S3 的行）；2.1 第 19 条的十处对着 `d7bb5e8d` 的 00 号文档逐处核对。<br>C：网页、e2e、测试、Step 与完整性；读了 `web/apps/web/node_modules` 的 `playwright-core@1.63.0/lib/coreBundle.js`（`innerAccumulatedElementText`，算了两种写法的可读的名字）与 `@atlaskit/pragmatic-drag-and-drop`，只读；第 7.3 节每条变异点名的测试以程序对着第 3–5 节核对（`review-C/mut-tests.txt`、`defined.txt`） |
| 日期 | 2026-10-24 |
| 结论 | A：自己的 22 条，落下 18、带问题 3（A-M1 成 AC-M1；A-M3、A-N11 是 Nit 级的，成 AC-N1、AC-N3）、随 A-Q1 不用做 1。新的 Important 0、Minor 1、Nit 6。B：自己的 29 条（含 3 个 Q 与 3 条过度设计）都落下，B-M5 与 B-O1 带问题（成 B2-M1、B2-N1）；新的 Important 0、Minor 2、Nit 5，轻的过度设计 3。C：自己的 36 条（含 2 个 Q 与 4 条过度设计）都落下，C-I2 带问题（成 C2-M1）；新的 Important 0、Minor 2、Nit 5，过度设计与大小 2。B2-N1 = C2-M2。<br>全部处理（下面三张表）；处置由主持者定下（"主持者的决定（第一轮复核）"第 16–21 条），负责人可以改判。修订直接写进设计（325,393 字节、2,439 行），复核时的样子留在 `01-P1-contract-frame.rev1.md` 与 `parts-rev1/`；这份记录前面被复核更正的说法就地标着"第一轮复核更正"。<br>7.3 的变异由 61 条变为 64 条：S3 加第 53 条（`Unit.Revert` 的锁下的一遍不判契约，B2-N4），S4 的旧的第 53–60 条成为第 54–61 条，旧的第 61 条换成第 62 条（去掉后缀前的 `{" "}`）与第 63 条（后缀被藏出可访问性的树），另加第 64 条（`useDrag` 的依赖，C2-N4）。2.1 仍是 24 条：第 19 条加 00 号文档的 `:181` 与 `:191` 的括号里的 `CheckRevertPart`、`:191` 的那一句整句改写；第 8 条加 transfer 的 `Refused` 的名字（C2-N3）；第 7、18 条的说法改正；第 1、2、4–6、8、11、12、19–21、23 条写全 `apply-00m10.py` 碰到的每一处、引的新文字与脚本逐字相同，第 24 条写明它细化第 11 节主持者的决定第 39 条、2.2 随之（"第一轮复核的作者的更正"作者-2、作者-6）。`apply-00m10.py` 由 75 处变为 78 处（加 transfer 的 `Refused`、4.16 的 `make knip`、第 39 条的注），改了五处的新文字；`--check` 对着仓库的 00 号文档全部对上，在一份副本上照样应用之后再 `--check`，73 处的旧文字都不在了，另 5 处（6d、9、12b、20a、A3）是在原文之后加字、旧文字留在新文字里（只核对，没有写仓库）。整个程序的新测试仍是 19 个（第二份 8、第三份 11） |

### 发现与处置

第一轮的条目照原样留着，复核更正的说法已就地标出；被这一轮改掉的处置以下面的表为准。节号与变异的号是这一轮修订后的。

#### A

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| AC-M1 | Minor | `TestTheTemplateRollsBackWithItsNotebook` 以 `currval('seen_nodes')` 读序列：`currval` 只认本会话调过的 `nextval`，触发器在应用的连接上调、测试读在 `tm.pool` 上，每次都报 55000；这一读正是分出"触发器键错成 `raw`"的那一半 | 照主持者的决定第 16 条：读 `SELECT last_value, is_called FROM seen_nodes`、期待 `(2, true)`（`nextval` 不回滚，`CACHE 1` 下每个会话读到的 `last_value` 相同），理由照旧（3.16、3.29） |
| AC-N1 | Nit | 测试调 `wiki.TemplatePages(wiki.ModeLLMWiki, l)`，模块根只以类型别名公开 `Mode`、`Locale`，别名不带常量，组合根又不许导入 `wiki/domain` | 照改：模块根加 `const (ModeLLMWiki = domain.ModeLLMWiki; LocaleEn = …; LocaleZhCN = …)`，写明 `module.go` 导入 `slices`（3.8） |
| AC-N2 | Nit | `TestDowngradeNotebookRefuses` 要 `FindNotebook` 答 `llm_wiki`、`LockNotebook` 答 `standard`，今天的 `fakeStore` 两者读同一张表（`notebook/app/fakes_test.go:98-114`） | 照改：假的仓库加一张 `locked`，`LockNotebook` 先读它（3.1 的 `app/fakes_test.go` 一行、3.10） |
| AC-N3 | Nit | `TestACreationPassesTheSeeders`：（a）500 的 error 一行是 API 的 `APIErrors` 以 `httpservertest.APIOptions.Logger` 写的，不经模块的 `Deps.Logger`；（b）`creatorsAuthorizer` 照 `sqlWorkspaces` 读成员关系会拒绝 alice（`newFixture` 不写 `workspace_members`） | （a）取 A 的第二种：不在这里核对日志（码进错误的文字由 `TestTheSeedersRunInTheirOrder` 核对，记下那一行是平台自己的、测过的事，整个程序的回滚的测试读它；A 的过度设计一条也这样说）；（b）取第一种：`notebook.create` 一律答 `WorkspaceMember`（3.5、3.29） |
| AC-N4 | Nit | `seed` 的 ctx 的一支走不到：`shared.Error` 没有 `Unwrap`，这条路上唯一的 `errors.Join`（`postgres/tx.go:170`）只合 ctx 与回滚的错误；去掉它是等价的变异 | 照改：去掉这一支与注释的半句，写明理由；`TestTheSeedersRunInTheirOrder` 的两行 ctx 照留（`default` 也当缺陷时它们失败）（3.5） |
| AC-N5 | Nit | `TestPageViolationsCatchEachLLMWikiGap`：（a）`DELETE` 的一行改不回，要放在最后，"没改的那一行"就是第一行；（b）草稿的"`AGENTS` 改成附件"一行没了，规则的两处 `kind = 'page'` 都没有反例 | 照改：没改的一行在最前、作反向对照，删除的一行在最后（S3 加的反例都在它之前，3.27）；（b）加 `raw` 改成附件、`AGENTS` 改成附件两行（`UPDATE nodes SET kind = 'asset'`），两处 `kind` 各有一行（3.14、3.29） |
| AC-N6 | Nit | page 的 `Notebooks` 的假的是三个：`search_test.go:194` 的 `searchWorkspace` 是工作区的端口的；`git grep 'LockByID('` 恰找到三个（A 更正自己的 A-N6） | 照改（3.1 的 page 一行、3.12：三个，`fakes_test.go:103-124`） |

#### B

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B2-M1 | Minor | `TestTheReservedTitlesAreNumberedAtAnLLMWikisRoot` 的两次恢复在导入之后的同一本里：`log 2`、`index 2` 已被导入占着，同步的恢复得 `log 3`、后台的得 `index 3`（`restoredNames`、`free`、`under` 都把兄弟的键当作已占）；草稿里就是这样 | 照主持者的决定第 19 条取 B 的第二种：同一本笔记本，期待 `log 3`、`index 3`，这样保留的键与兄弟的键一起被钉住（3.28） |
| B2-M2 | Minor | `TestTheModeGuardReachesEveryPath` 的普通的那一本到第 7 步时根下已有四个保留的键（第 3 步建 `index`、`Log`、`index.md`，第 4 步改名 `log.md`），第 7 步传 `index` 答 409 `page.title_taken`，与"普通的笔记本里同样的请求全部成功"相反 | 照主持者的决定第 19 条：每个保留的键只用一次：第 3 步建 `index`、`index.md`，第 4 步改名为 `log.md`，第 7 步传 `LOG`（LLM Wiki 里 409，留着大小写折叠的核对；普通的 201）与 `log.png`（3.24） |
| B2-N1 | Nit | = C2-M2；另：第 19 条对 `:191` 的改法留下"今天那一批的守卫的 403 会成为 `internal`（"你不能再写这本笔记本"，不对），守卫的会成为 `internal`" | 同 C2-M2；`:191` 的那一句整句改为"今天那一批的守卫的拒绝会成为 `internal`"（2.1 第 19 条、`apply-00m10.py` 的 19g） |
| B2-N2 | Nit | 第 50 条变异点名 `TestARestoreNumbersTheReservedTitlesAtAnLLMWikisRoot` 的"子页下的一行"，3.27 没有给它这样的行（草稿就是这样） | 照改：加两行：后台的一部分里叫 `index` 的成员随恢复了的父节点回去、一部分的根回到活着的子页之下，都留着 `index`、不数 `Renamed`（3.27、第 50 条变异） |
| B2-N3 | Nit | 第 28 条变异的第二个抓手 `TestTheReservedTitlesAreNumberedAtAnLLMWikisRoot` 抓不到：S3 起导入在单元写之前就加序号，R3 没有可拒的 | 照改：只留 `TestCheckContract` 的导入的一行；3.21 写明 R3 的导入的豁免在 S3 之后走不到、留作规则的文字（B 的第二条轻的意见） |
| B2-N4 | Nit | `TestARevertsChecksRefuseTheContract` 的锁下的一行在登记了真的 `ModeGuard` 时分不出锁下的一遍与守卫：守卫在第一步以同样的码拒绝、也只问一次；7.3 没有"锁下的一遍不判契约"的变异 | 照改：这一行交一个记下步、什么也不拒的替身守卫，断言它一步也没看到；加第 53 条变异（3.28、3.29、7.3） |
| B2-N5 | Nit | 3.17 漏了 `app/grants_test.go` 与 `TestTheModeGuardAsksOnceAUnit` 的文件；3.20 的 `TestTheModeGuardLetsAStandardNotebookBy` 两处文件名不同；守卫的 `errors.Is(err, shared.ErrNotVisible) \|\|` 是死的（`Grants.Ask` 已把它换成 `Forbidden()`）；`behindFirstBatch` 是 `:196-210` | 照改：3.17 的两行写全（`TestTheModeGuardAsksOnceAUnit` 随 B 的第三条轻的意见挪进 `page/app/mode_guard_test.go`）；3.20 写 `app/mode_guard_test.go`、去掉那一个条件、写明为什么；3.28 与 7.2 第 7 行写 `:196-210` |
| B 轻的意见 1 | — | `TestTheModeGuardLetsAStandardNotebookBy` 的"计数的授权者一次也没被问"在 P1 失败不了（普通的笔记本没有来源根，R4 从不问） | 照主持者的决定第 21 条：去掉这一句，写明它在 P2 也失败不了（普通的笔记本从来没有来源区）；P2/S1 的计数的存储照旧（3.20、第 9 节） |
| B 轻的意见 2 | — | R3 的"单元不是导入"只在 S2→S3 的窗口里用得上 | 照主持者的决定第 21 条：3.21 一句写明，留作规则的文字（见 B2-N3） |
| B 轻的意见 3 | — | `TestTheModeGuardAsksOnceAUnit` 不必真库：守卫在 `page/app`，那里的假的给事务里的调用记下 `" in tx"` | 照主持者的决定第 21 条：挪到 `page/app/mode_guard_test.go`，假的存储、事务与授权者，真的 `ModeGuard`；`Write.Grants` 是值时（第 32 条变异）记下两次，仍能失败（3.19、3.29、7.3；第二轮复核更正：只在假的授权者放行 `page.manage_source`、两次移动都写下时，B3-N1） |

#### C

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C2-M1 | Minor | 第 61 条变异的第一种（空格放进 `sr-only` 里）在 Chromium 与 jsdom 里给同一个名字，是等价的变异；第二种（以"，"起头）每个测试都抓；2.1 第 18 条、3.29、7.3 的理由（"空格放进 `sr-only` 里时……名字与 jsdom 里的不同"）不对。真正要防的：去掉 `{" "}`（只有 jsdom 的 `textContent` 看得到）；后缀被藏出可访问性的树（`aria-hidden`、`hidden`：只有 e2e 看得到） | 照主持者的决定第 17 条：第 61 条换成第 62 条"去掉后缀前的 `{" "}`"（`page-tree.test.tsx` 的 `textContent`）与第 63 条"后缀的 `span` 带 `aria-hidden`（或以 `hidden` 代 `sr-only`）"（LW1、LW2 (page)）；4.4 写明两种测试各看哪一半；2.1 第 18 条、3.29、7.1 的理由改正，照第 21 条只在 4.4 写一次、别处指向它（5.2 的 LW1 (page) 同样） |
| C2-M2 | Minor | 2.1 第 19 条漏了 00 号文档的两处每批的检查：`:191` 的一句（C 说第三句，是第四句的括号里）"下一批的 `CheckRevertPart` 答 `page.source_admin_only`"、`:181` 的"每批的检查照留"；"作者的更正"第 12 条的"十处"随之 | 照主持者的决定第 18 条：两处加进第 19 条（`:191`"下一批的守卫答 `page.source_admin_only`"，`:181`"每一批由它的单元里的守卫判"，取 C 的写法），共十一处；`:191` 的那一句整句改写（B2-N1）；`apply-00m10.py` 本就改这两处，新文字与 2.1 对齐（19e、19g、19i）；第 12 条就地更正 |
| C2-N1 | Nit | 7.5 以收尾的 run 的 5:25 作第三份的底；Codex 评审之后的 run（P1 开始时的代码）的第三份作业 6:21，约是 5:57 的测试一步；`Makefile:200-203` 的规则取两次的平均 | 照改：第二份约 6:12、第三份约 5:41 作底；第三份去掉磁盘的一半之后约 6:20–6:30，离线约一分钟（7.5） |
| C2-N2 | Nit | 第 8 节说新的 `M10-mode.md` 收第 9 节"M11"里的"`Grants` 与 `Refused` 是冻结的守卫加码的地方"，第 9 节的那一条没有它 | 照改：第 9 节的 M11 一条加上 |
| C2-N3 | Nit | 00 号文档 `:191` 写 `ErrRefused{Code, NodeID}`，3.26 第 3 条定的是 `transfer.Refused{Code, NodeID, Err}`，2.1 没有一条 | 照改：加进 2.1 第 8 条（它本就改 `:191` 的 `treeError` 一句；C 给了第 19 条或第 8 条两种）；3.26 第 3 条写明名字与多的 `Err`；`apply-00m10.py` 加 8f |
| C2-N4 | Nit | 一行能不能拖由模式、来源根、角色与节点算出，`useDrag` 的依赖（`page-tree-item.tsx:163-198`）一个也不随降级变，`AGENTS` 的行一直不能拖到重新挂上 | 照改：效果依赖算出的能不能拖；`page-tree.test.tsx` 加一条：同一棵树的笔记本换成普通的再渲染，`AGENTS` 的行有了 `draggable`（第二轮复核更正：并断言重读前后是同一个行元素，C3-N1）；加第 64 条变异（4.4、3.29、7.3） |
| C2-N5 | Nit | `TestTreeErrorKeepsTheLockFirst` 的"再经一次 → 不变"只在比指针时抓得到第 47 条：再包一层的码与节点相同 | 照改："交回同一个 `*transfer.Refused`"（`got == in`）（3.26、3.28、7.3） |
| C 大小 1 | — | 可读的名字的理由写了七遍（2.1 第 18 条、4.4、5.2、7.1、7.3、3.29、第 8 节 13.2 第 17 条） | 照主持者的决定第 21 条：只在 4.4 写；2.1 第 18 条、5.2、7.1、3.29、7.3 指向 4.4；第 8 节的 13.2 第 17 条是要写进总体设计的规则，留着（C 也说它可以留） |
| C 大小 2 | — | `TestTheServersLimitsComeAfterThePrechecks` 的磁盘的一半重复 `asset/app/upload_test.go` 的那一行：次序是结构的（asset 调完 page 的整个 `Check` 才查空间），第 42 条变异由 `TestAnUploadsCheckRefusesAReservedTitle` 抓；它在余量较少的第三份里多一个库与一个应用 | 照主持者的决定第 21 条：去掉（3.25、3.28、3.29、2.1 第 7 条、7.1、7.3 的第 42 条）；份重核（7.5）（第二轮复核更正：留下的那一行是 `TestCheckDecidesTheNodeThenTheFreeSpace` 的一种，"不调 `Store`"在 HTTP 适配器的 `checkErr` 一行，B3-N2） |

A 的过度设计的两处（`seed` 的 ctx 一支、日志的核对）随 AC-N3、AC-N4 处理。

### 重叠（第一轮复核）

| 审查 | 审查 | 处置 |
|---|---|---|
| B2-N1 | C2-M2 | 第 19 条的十一处与 `:191` 的整句（第 18 条） |
| B2-N3 | B 轻的意见 2 | 3.21 写明，第 28 条变异只留一个抓手 |
| B2-N5 的文件 | B 轻的意见 3 | `TestTheModeGuardAsksOnceAUnit` 挪进 `page/app`，3.17 写全 |
| AC-N3（a）、AC-N4 | A 的过度设计 | 去掉日志的核对与 ctx 的一支 |
| C2-M1 | C 大小 1 | 理由改正、只在 4.4 写一次 |

### 主持者的决定（第一轮复核）

主持者在第二份修订的简报（`revise2-brief.md`）里定下的处置（作者照做；负责人可以改判）：

16. **AC-M1**：读 `SELECT last_value, is_called FROM seen_nodes`、期待 `(2, true)`。
17. **C2-M1**：第 61 条换成"去掉 `{" "}`"，由 `page-tree.test.tsx` 的 `textContent` 的行抓；加一条"后缀的 `span` 带 `aria-hidden`（或 `hidden`）"，只由 LW1、LW2 (page) 抓；2.1 第 18 条、3.29、7.1 的理由改正。
18. **C2-M2 = B2-N1**：第 19 条也列 `:181` 与 `:191`，条数随之改，"作者的更正"第 12 条同样；第 19 条留在 `:191` 的那一句不通的句子改好；`apply-00m10.py` 本就改这两处，新文字与之对齐。
19. **B2-M1**：恢复在一本新的 LLM Wiki 里跑，或期待 `3`，写明取哪一种（作者取后一种：同一本里钉住保留的键与兄弟的键一起被当作已占）。**B2-M2**：普通的那一本里每个 R3 的情形各用自己的键，每个请求都成功。
20. **Nit 都照收**：A 的模块根的模式与语言的常量（AC-N1）、假的仓库分开答找到与锁下（AC-N2）、`TestACreationPassesTheSeeders` 的日志与授权者（AC-N3）、死的 ctx 的 `case`（AC-N4）、反例的表的次序与 `AGENTS` 作附件的一行（AC-N5）、假的的个数（AC-N6）；C 的第三份的底取对的 run（C2-N1）、第 9 节的 M11 一条（C2-N2）、`transfer.Refused` 的名字作 2.1 的一处（C2-N3）、`useDrag` 的依赖（C2-N4）、第 47 条的同一性（C2-N5）；B 的第 50、28 条变异、锁下的一遍的接法与它的变异（B2-N2–N4）、小的文字（B2-N5）。
21. **大小与过度设计**：可读的名字的理由只在 4.4 写一次、别处指向它；去掉 `TestTheServersLimitsComeAfterThePrechecks` 的磁盘的一半（`asset/app/upload_test.go` 的一行盖住它），之后重核份；B 的三条：普通的笔记本的"授权者没被问"在 P1 失败不了就去掉，或写明 P2 里什么让它失败（去掉：P2 也不会）；一句写明 R3 的导入的豁免在 S3 之后走不到、留作规则的文字；`TestTheModeGuardAsksOnceAUnit` 挪到假的上，只要它仍能失败（仍能：第 32 条）。

### 第一轮复核的作者的更正

| # | 发现 | 处置 |
|---|---|---|
| 作者-1 | 第 53 条加在 S3 的末尾，S4 的变异由此整体后移一位（旧的第 53–60 条成为第 54–61 条）；这份记录前面点名 S4 的变异号的五处（C-I2、C-M3、C-M4、C-M5 的处置与 C 的"核对过"一段）就地标出新号 | 不另留空号：S3 的变异仍在一段里 |
| 作者-2 | 写 `apply-00m10.py` 时发现 2.1 的几条没有写全它们在 00 号文档里要改的每一处（第 1 条的 `:616`、`:640`、`:645`，第 2 条的第 7 节 S1，第 4 条的 4.6 的表、第 7 节与 `:640`，第 5 条的 `:163`、`:165`，第 6 条的 `:165`、`:531`，第 8 条的 `:59` 与第 7 节 S3，第 11 条的变体的行，第 19 条的 S3 的验证，第 20 条的 S2 的验证，第 23 条的 `:389`）：同样的说法不改，00 号文档就自相矛盾 | 2.1 各条写出这些地方与它们的新文字；`apply-00m10.py` 的每一处改动都是 2.1（或第 8 节第 4 条）点名的，新文字照 2.1 |
| 作者-3 | AC-N5（b）A 只要回 `AGENTS` 改成附件的一行；规则的两处 `kind = 'page'` 是 `raw` 的与 `AGENTS` 的 | 两行都加 |
| 作者-4 | C2-N4 C 说树的 vitest"也可以"核对降级之后重新登记 | 照收为一条测试，并给它第 64 条变异（依赖不含算出的值时它失败），守 13.4 第 1 条"每个新的检查有一个让它失败的反例" |
| 作者-5 | 第 19 条的 `:531` 那一处，复核时的修订稿写"去掉"；`apply-00m10.py` 在原处写"后台撤销的每一批不另检查，由那一批的守卫判"（与 `:181` 的"每一批由它的单元里的守卫判"同义） | 2.1 写成"改为"那一句，脚本与之一致 |
| 作者-6 | 修订自己的核对（一位 Opus 的核对者对着这一轮的修订、三份复核与这份记录读了一遍，只读；它以 `--check` 跑了脚本，并在 `scratchpad/m10/p1/rev2/check/` 的一份副本上应用）：所有的发现都照表落下，没有 Important 与 Minor；11 处 Nit：上面结论里"旧文字都不在了"对在原文之后加字的 5 处不对；2.1 第 1、21、23 条引的新文字与脚本不逐字相同；第 12 条点名 4.16 而没有一处改它（`:400` 的网页的一步漏了 `make knip`）；未采纳里 AC-N5 的指向；作者-5 的"同 `:181`"；第 9 节的 M11 一条缺第 8 节列的 `SetNotebookMode`；3.1、3.8 与 13.1 第 11 条没有列 AC-N1 的三个常量；第 24 条细化第 11 节主持者的决定第 39 条而 2.2 说不碰；3a 留下"的 值"的空格；`:191` 的 `CheckRevertPart` 在第四句的括号里，不是"第三句"；4.4 的"为什么空格在外"的引语。核对者写的只在 `rev2/check/` 里 | 都照改：2.1 第 1、21、23 条引脚本的文字；第 12 条加 `:400`，脚本加 12c；第 24 条写明细化第 39 条，`:802` 加注、脚本加 24b，2.2 改；3a 改为"的值"；4.4 的引语改为"为什么要这个空格、后缀以括号起头"；其余照改 |

### 第一轮复核审查者核对过、没有问题的

- **A**：`shared.ClientOf` 的签名与三份同一条规则（`APITokenID` 设着是 `api`），401 来自 `RequireActor`、`errors.Is` 经 `(*Error).Is`，四个适配器的转换与它们的测试（asset、transfer 的假的已有会话与 PAT 的 `Actor`，notebook 的加一个 PAT 的），13.1 第 2 条的新文字；播种者的 500：只有 `*shared.Error` 实现 `ProblemError`，不以 `%w` 包的理由（`ProblemOf` 先 `errors.As`），码进日志（`FaultUnknown`），`errorlint` 安静，没有可重试的错误丢掉，第 7 条变异两个测试抓；`TestTwoAdminsDowngradingAtOnce` 的替代逐环钉住（`LockNotebook` 选模式、`manager.lock` 交回锁下读的行、用例判那一行、一个 `mode`），00-M10 4.17 的清单里没有它；A-M2 的表的每个条件都有失败的一行；A-M5 逐个提交（`State`/`StateOf` 导出，`noSeed{}` 在第 4 个、`noJobs{}` 在第 5 个，`accountNotebooksOf` 第 5 个加 `wlab`、第 7 个加 `mode`，改契约的提交带 `web`）；`TemplatePages` 是完整的副本；S1 的八个名字的份；2.1 的条号、"二十四处"、"七条交错"、"建模块的四处"；它范围里新引的每个行号；第 1–22 条变异。
- **B**：B-I1 的交错怎样逼出（计划的调用不取行锁，降级以 `FOR NO KEY UPDATE` 排第一）与 `d.Guards = nil` 的反向对照；`TestTheServersLimitsComeAfterThePrechecks` 的两半各能失败（`check` 在 `contents` 与 `DecidedContents` 之前，`busyWrites` 的写法，`Notes` 的 1 KiB 让 C 的撤销要解析）；R4 的新文字、步 × 节点的表与新的行、步（`moveSubtree` 被移动的在先，撤销的放回用同一个函数，计划的树的一遍同样）；`batch` 的计划的树的一遍在 S2、第 3 个提交不靠它也绿；`BatchPlan.Refuse`（去掉在 `at` 或之后的项的页，`Contents` 停在 `Failed.Op`，404 与数据库的错误两行，第 41 条）；B-M2 的选择与问的次数；每批的检查去掉之后后台的唯一的路是 `revertDefect`，2.1 第 19 条的十处的原话都在 `d7bb5e8d` 的 00 号文档里；两批之间的测试显出守卫的拒绝；B-M5 的加序号本身；份；第 1–61 条连续，抽查的 S2、S3 的变异能失败（除 B2-N2、B2-N3、B2-N4）；4.2 的文字、第 8 节 13.1 第 4 条的 S2、S3 文字、第 9 节 P2/S1。
- **C**：TR1–TR4 是 `e2e/` 里仅有的对整个报告的 `toEqual`，其余逐列读；四个可读的名字在 Chromium 与 jsdom 里都照写的出来（`innerAccumulatedElementText`）；自由的字符串的 `code`、`problem.other` 的退路与它的测试、第 52 条；C-M1–C-M12、C-N1–C-N14 都落下；设置的五个测试（`notebookServer` 只在答 ok 时改状态、角色取自 `members`）、`[name, mode]` 的测试、`answers` 的覆盖与 SWR 的键、替身改名 `later`、树的菜单与属性；6.1–6.4 的每个提交单独绿（`unused`、`accountNotebooksOf`、knip 经 `export type *`、tsc 的两处字面量与 `failures`、e2e 没有整个比较 REST 的笔记本、LW1 (page) 的名字在 S4 的第 1 个提交）；`TestTheGuardRefusesWhatThePlanLetBy` 的交错与反向对照；预算的一半；第 1–61 条每条点名的测试都在第 3–5 节（以程序核对），抽查的都能失败（除第 61 条与第 47 条）；19 个名字的份（第二份 8、第三份 11），已有的测试加的行的份；第 8 节（13.1–13.4 的文字与 v0.1 的每一行、M5-events `done`、00-M10 的头、进度表与变更记录、新的 M11 的移交）、第 9 节；2.1 的 24 条对着 `HEAD` 的 00 号文档（除第 18 条的理由与第 19 条漏的两处）。

### 第一轮复核未采纳

- **AC-N1 的另一种**（调用写字面量 `"llm_wiki"`、`"zh-CN"`）：取了常量，组合根的测试不写模块的取值。
- **AC-N3（a）的另一种**（把写进缓冲的 handler 交给 `NewAPI`）：取了去掉这一核对（A 的过度设计一条），日志由整个程序的回滚的测试读。
- **AC-N5（b）的另一种**（写明那一行去掉了）：两行都加回（"第一轮复核的作者的更正"作者-3）。
- **B2-M1 的另一种**（恢复在一本新的笔记本里）、**B2-M2 的另一种**（把普通的那一本的 409 写成第三处不同）：没有取，理由在主持者的决定第 19 条与上表。
- **B2-N1 的写法**（`:181` 写"每一批仍由它的守卫判"）：取了 C 的"每一批由它的单元里的守卫判"，说出守卫在单元里。
- **B2-N4 的另一种**（这一行不交守卫）：取了记下步的替身，断言它一步也没看到，比"没有守卫"多说一件事。
- **C2-N3 的"加进第 19 条"**：加进了第 8 条，它管 `:191` 的 `treeError` 的那一句。
- 没有一条审查者的建议被判为错的。

### 第一轮复核的新问题

没有要负责人现在定的。第三份的估计换了底（两次 run 的平均）之后约 6:20–6:30，离线约一分钟，"新的问题"第 1 条的读法不变；第二份照旧离线最近。

## 第二轮复核

| 项 | 内容 |
|---|---|
| 复核对象 | 第二次修订后的 [01-P1-contract-frame.md](../01-P1-contract-frame.md)（2026-10-24，325,393 字节、2,439 行；复核时的样子留作 `01-P1-contract-frame.rev2.md` 与 `parts-rev2/`，这份记录的样子留作 `P1-design-review.rev2.md`，`apply-00m10.py` 的样子（78 处）留作 `apply-00m10.rev2.py`）与"第一轮复核"一节的处置，对照 `main` `d7bb5e8d` 的代码与 00 号文档；核对照的简报 `check-brief-2.md` |
| 复核方式 | 三位审查者（Opus）各复核自己的一份，对着第一轮复核时的样子逐部分比（`parts/` 对 `parts-rev1/`），只读，不跑测试、构建、Docker 与数据库，不下载。三位都以 `--check` 对着仓库跑了 `apply-00m10.rev2.py`（78 处都对上），在自己的副本上应用（`review-A/c2/`、`review-B/c2/`、`review-C/apply/`），读了改过的地方的上下文；仓库一直是干净的：<br>A：S1（第 0 节的 S1、第 1 节、2.1 第 1–4、9–11、15、22 条，3.1–3.16、3.29 的 S1 的行、4.1、6.1，第 7 节的 S1 的行）与脚本的这几条的改动。<br>B：S2、S3（2.1 第 4–8、14、19–21、24 条，3.17–3.29，第 6、7 节的 S2、S3 的行）与脚本的这几条的改动。<br>C：网页、e2e、测试、Step 与完整性，第 1–64 条变异与 7.5；另核对整个脚本：每一条 2.1 都有改动，每一处改动都属于 2.1 的一条或第 8 节第 4 条，副本上的 00 号文档的 4.4–4.6、4.9、4.16、5、7、8、9、11–13 节对着修订后的设计读，头、进度表与变更记录照 M9 的样子。C 报了一件没有影响的事：在 `parts/` 里跑 `python3 assemble.py --help` 执行了这个脚本（它不认选项），以当时的各部分重新拼出 `01-P1-contract-frame.md`，`cmp` 与 `.rev2.md` 逐字节相同，只变了修改时间 |
| 日期 | 2026-10-24 |
| 结论 | A：自己的 7 条都落下；新的 Important 0、Minor 0、Nit 1（AC2-N1）。B：自己的 2 条 Minor、5 条 Nit 与 3 条轻的意见都落下，第三条轻的意见带问题（成 B3-N1）；第一次审查的 B-M4 随主持者的决定第 21 条撤回，B 认为对；新的 Important 0、Minor 0、Nit 2（B3-N1、B3-N2）。C：自己的 2 条 Minor、5 条 Nit 与大小的 2 条都落下；新的 Important 0、Minor 0、Nit 3（C3-N1–N3）、Q 0，没有新的过度设计；脚本对。没有 Important 与 Minor 留下。<br>六条 Nit 都照审查者的建议收下（"主持者的决定（第二轮复核）"第 22 条），负责人可以改判。修订直接写进设计（327,486 字节、2,439 行），复核时的样子留在 `01-P1-contract-frame.rev2.md` 与 `parts-rev2/`；这份记录前面被这一轮更正的说法就地标着"第二轮复核更正"。7.3 仍是 64 条变异，2.1 仍是 24 条（第 3、7、8、20 条加字）。`apply-00m10.py` 由 78 处变为 81 处：加 3c（`:640` 的第三个 `Deps`）、8g（`:654`）、20d（`:531` 的 S3 的验证去掉 `replans`），20b 的新文字加"模块一级以假的守卫测 `replans`"；脚本改以 `argparse` 读选项，不认的选项在读文件之前就报错（作者-10）；`--check` 对着 `d7bb5e8d` 的 00 号文档 81 处全部对上，在一份新的副本上应用 81 处之后再 `--check`，76 处的旧文字都不在了，另 5 处（6d、9、12b、20a、A3）是在原文之后加字、旧文字留在新文字里（只核对，没有写仓库）。整个程序的新测试仍是 19 个（第二份 8、第三份 11） |

### 发现与处置

前两轮的条目照原样留着，被这一轮更正的说法就地标着"第二轮复核更正"；节号与变异的号与第一轮复核时相同。

#### A

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| AC2-N1 | Nit | 00 号文档第 8 节的 notebook 一行（`:640`）写"`notebook.New` 的两个新的 `Deps`"：P1 给 notebook 的 `Deps` 加三个（`Seeders`、`ModeSubscribers`、`WritingJobs`），没有第一个或最后一个时 `New` panic（3.5 的 `TestTheModuleRefusesNoWritingJobs`）；`WritingJobs` 在 00 号文档里只以 `transfer.NewWritingJobs(pool)` 出现（`:80`、`:644`）。A 前两次都漏了 | 照主持者的决定第 22 条取 A 的第一种：脚本加 3c，同一行的第三处改动（另两处是 1d、4d），改为"`notebook.New` 的三个新的 `Deps`（另有 `WritingJobs`：`transfer.NewWritingJobs`，没有它时 `New` panic）"；写进 2.1 第 3 条（它管第 8 节的扩展点的那几行），不另立一条 |

#### B

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B3-N1 | Nit | `TestTheModeGuardAsksOnceAUnit` 只在假的授权者放行 `page.manage_source` 时抓得到第 32 条：page/app 的 `fakeAuthorizer` 对不放行的动作答 `ErrNotVisible`（`page/app/fakes_test.go:1150-1170`），`Grants.Ask` 把它换成 `Forbidden()`，守卫在 `batch` 的第一次移动就拒绝、单元停下，`Write.Grants` 是指针还是值都只问一次 | 照主持者的决定第 22 条：3.19 写明假的授权者放行 `batch` 自己的动作与 `page.manage_source`（`f.grant(domain.ActionBatch)` 之外再 `grant` 它），两次移动都写下，并写明不放行时为什么分不出指针与值；7.3 第 32 条同样 |
| B3-N2 | Nit | 3.28 的 `asset/app/upload_test.go` 一行（与 3.29 的"预检在服务器的上限之前"）的"不调用 `Store`"在那一层失败不了：`asset/app` 的 `Upload.Check` 从不调 `Store`，是 HTTP 的处理函数先 `Check`、再 `Store`（`asset/adapter/http/upload.go:63-68`） | 照主持者的决定第 22 条：那一行写成已有的 `TestCheckDecidesTheNodeThenTheFreeSpace`（`:72-101`）的一种：page 的 `Check` 答 `page.virtual_file` 而磁盘不够时答 `page.virtual_file`；"不调 `Store`"挪到 3.22 给 S3 加的 HTTP 适配器的 `checkErr` 一行（照 `TestUploadAnswersEachFailure` 的 `checkErr` 的几行，`asset/adapter/http/upload_test.go:337-346`；`stored`、`kept` 都是 `false`）；2.1 第 7 条、3.25、3.28、3.29、7.1 点名这个测试 |

#### C

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C3-N1 | Nit | 4.4 为 C2-N4 加的 `page-tree.test.tsx` 的一条写"（行不重新挂上）"而不断言它：重读若让行重新挂上（加载的状态、变了的键），挂上时的效果照样登记，第 64 条变异也照过 | 照主持者的决定第 22 条：重读之前记下这一行的元素，之后断言是同一个（`expect(after).toBe(before)`）；7.3 第 64 条写"同一个行元素" |
| C3-N2 | Nit | 00 号文档 `:531` 的 S3 的验证仍写"模块一级以假的守卫测 `replans`、`revertDefect`、`treeError` 的标出……"，设计把 `replans` 的测试放在 S2（3.28；6.2 第 3 个提交的 `TestABatchAnswersARefusalAtItsOperation`）。`HEAD` 就是这样，但 2.1 第 20 条本就改这一格的 S2 与 S3 的验证（20b、20c）而留下了它 | 照主持者的决定第 22 条：第 20 条把"模块一级以假的守卫测 `replans`"放进 S2 的验证、S3 的那一句去掉 `replans`；脚本的 20b 的新文字加这一句，另加 20d；3.28 的"模块一级"一段改引 S3 的新文字 |
| C3-N3 | Nit | 第 9 节的"有预检的路径"（`:654`）仍把 P2/S2 的重复说成让守卫的最后一跳"完整"的只有守卫拒绝的情形；第 8 条在 `:531` 改正了同样的说法：P1 已有 `batch` 的计划与单元之间的降级（`TestTheGuardRefusesWhatThePlanLetBy`，第 8 节 13.1 第 21 条的文字称它为 `batch` 的最后一跳）。不是矛盾，但读来像 P2/S2 之前 `batch` 没有只有守卫拒绝的情形 | 照主持者的决定第 22 条：第 8 条加 `:654`，"P2/S2 补上"之后加"（P1 已有 `batch` 的计划与单元之间的降级，P1 设计 3.24）"；脚本加 8g |

### 重叠（第二轮复核）

没有两位审查者指同一处的。三条接着前一轮的处置：B3-N1 是第一轮复核的 B 的第三条轻的意见带的问题，C3-N1 是 C2-N4 的，C3-N3 与 2.1 第 8 条在 `:531` 的改动是同一个说法；B3-N2 接着第一轮复核的"C 大小 2"（B-M4 的撤回）。

### 主持者的决定（第二轮复核）

主持者在给作者的话里定下的处置（作者照做；负责人可以改判）：

22. **六条 Nit 都照审查者的建议收下**：`:640` 写三个新的 `Deps`，脚本第三处改那一行（AC2-N1）；假的授权者放行 `page.manage_source`，两次移动都写下（B3-N1）；asset 的那一行作 `TestCheckDecidesTheNodeThenTheFreeSpace` 的一种，"不调 `Store`"写在 HTTP 适配器的一行（B3-N2）；记下行元素、重读之后断言是同一个（C3-N1）；在第 20 条里把 `replans` 的测试挪进 `:531` 的 S2 的一格（C3-N2）；`:654` 照第 8 条改 `:531` 那样改（C3-N3）。

### 第二轮复核的作者的更正

| # | 发现 | 处置 |
|---|---|---|
| 作者-7 | B3-N1 B 说放行 `page.manage_source`（与 `batch` 自己的动作）；`apply_batch_test.go` 的写法里 `batch` 自己的动作是 `f.grant(domain.ActionBatch)` | 3.19 两个都写出，并写明不放行时为什么指针与值分不出 |
| 作者-8 | B3-N2 "不调 `Store`"在 HTTP 适配器的表里看的是 `TestUploadAnswersEachFailure` 的 `stored`（存了又删）与 `kept`（存了留着）两列：都为 `false` 就是没有文件写下 | 3.22 那一行写出这两列与测试名，3.28 指向它 |
| 作者-9 | C3-N2 改了 `:531` 的 S3 的验证之后，3.28 的"模块一级"一段引的 00 号文档的 S3 的旧文字（含 `replans`）与之不合 | 改为引 S3 的新文字，并写明第 20 条把 `replans` 挪进 S2 |
| 作者-10 | 作者在这一轮以 `--help` 跑了 `apply-00m10.py`：脚本只认 `--check`、`--path`、`--date`，别的选项不报错，照默认的路径写了仓库的 00 号文档（写下的正是 81 处改动，与在 `d7bb5e8d` 的一份副本上应用的结果逐字节相同，没有别的改动）。作者无权还原，已交主持者还原 | 脚本改以 `argparse` 读选项：不认的选项报错、退出码 2，`--help` 只打印用法，都在读文件之前；在副本上核对过两者都不写文件。之后的 `--check` 都对着 `d7bb5e8d` 的 00 号文档的一份副本跑 |

### 第二轮复核审查者核对过、没有问题的

- **A**：自己的 7 条的落下：`last_value`、`is_called` 的读能失败（触发器键错成 `raw` 时是 1，没有插入时 `is_called` 为假）；模块根的常量经类型别名可以编译；假的仓库的 `locked`，第 18 条变异仍以 200 让那一行失败；`TestACreationPassesTheSeeders` 的第 7 条变异；去掉 ctx 的 `case` 之后的 `switch` 写法完整、两行 ctx 仍能失败；反例的表的次序与两行附件（`nodes_kind_check` 允许 `asset`）；三个假的。S1 的第 1–22 条变异不变，3.5、3.14 引的第 1、22 条对；文档里没有留下 `currval`、日志的核对、读成员关系的授权者、ctx 的 `case` 与第四个假的。脚本的 1a–1e、2a–2e、3a–3b、4a–4d、9、10、11a–11b、15a–15b、22 在副本里的上下文，2.1 第 1、2、4、11 条引的文字与脚本相同。6.1、4.1、7.5 的 S1 的份（第二份 5、第三份 3）与 7.1 的 S1 的行不变。新引的行号（`notebook/app/fakes_test.go:98-114`、`page/app/fakes_test.go:103-124`、`shared/error.go:111-133`、`:208-214`、`httpservertest/api.go:25`、`:44-53`、`notebook/extension_test.go:113-127`、`00012_page_nodes.sql:12`）；没有别的约束或触发器挡着 `UPDATE nodes SET kind = 'asset'`。
- **B**：B2-M1 的 `log 3`、`index 3` 与代码相合（`restoredNames`、`under`，`free` 跳过每个已占的键，`unit_import.go:189`），两种缺陷各让它失败；B2-M2 的普通的那一本走完第 1–9 步没有别的冲突，`LOG` 留着大小写折叠的核对；第 19 条的十一处在副本里没有留下每批的检查，留下的两处"每批"是对的；B2-N2 的两行在第 50 条下成为 `index 2`；B2-N3、B2-N4（第 53 条下三处断言失败）、B2-N5；B-M4 的撤回（`TestCheckDecidesTheNodeThenTheFreeSpace` 已以 `free = 0` 钉住次序，`asset/app/upload.go:66-77`）。脚本的这几条在副本里的上下文（`:18`、`:57`、`:59`、`:156-157`、`:161`、`:163`、`:165`、`:179-181`、`:190-191`、`:205`、`:262`、`:471-473`、`:531-532`、`:537`、`:558`、`:654`、`:802`、`:829`）与 3.19–3.27 相合。第 1–64 条变异连续，第 3–5 节引的号都是新号；去掉磁盘的一半之后解析的一半两半仍能失败，份仍是 8 与 11，第三份 5:41 加 40–50 秒；锁下的一遍的替身守卫（`f.guards`，`fakes_test.go:1337`）与三者问的次序；没有 `ErrNotVisible` 一支的守卫与 `Grants.Ask` 的注释相合；6.2、6.3 不变、与挪了的测试相合。这份记录的 B 的行、主持者的决定第 19、21 条、B-M2、B-M4、B-O1 的更正与未采纳的 B 的几条与设计相合。
- **C**：C2-M1 的第 62、63 条都能失败（`innerAccumulatedElementText`），4.4 说两种测试各看哪一半；C2-M2 的十一处与 19e、19g、19i，副本里没有 `每批的预检`、`每批的检查`、`CheckRevertPart`、`ErrRefused`；C2-N1 的底的算术（6:12、5:41，第三份 6:20–6:30、第二份 7:00–7:15）；C2-N2–N5 与大小的两条。脚本：先对着原文、再在每一处替换之前核对旧文字，都对上才写，`--check` 不写，只写 `path` 的那一个文件；每个标签属于 2.1 的一条（1–24，每条至少一处）或第 8 节第 4 条（A1–A3）；2.1 引的每一处新文字都在脚本的新文字里（唯一没找到的引的旧文字"后台任务的开始"是行的名字，不是改的文字），描述而不引的改动与描述相合；副本上的 00 号文档的那几节与修订后的设计相合，旧的说法都不在了，`:191` 一段读得通；进度表的一行、变更记录的"P1 设计"一行照 M9 的 P1 设计的提交（`7f03403c`），头与"开工"一行照 00-M10 自己要的（`:824`、第 8 节第 4 条），五份移交在 `d7bb5e8d` 都有"M10 开工"一行、仍是 `open`。4.4 的改动对着代码（`page-tree-item.tsx:196` 今天的依赖）；第 1–64 条的号与引用，改了的几条都能失败；7.5；挪了的测试没有别的矛盾（`TestTheModeGuardAsksOnceAUnit`、`TestTheModeGuardLetsAStandardNotebookBy` 在 `page/app`、6.2 第 3 个提交，`TestAGrantIsAskedOnceAUnit` 在 `grants_test.go`）；第 8、9 节；这份记录的 C 的行、作者-1 到作者-6。

### 第二轮复核未采纳

- **AC2-N1 的另一种**（另立 2.1 的一条）：放进了第 3 条，它已管第 8 节的扩展点的那几行。
- 没有一条审查者的建议被判为错的。

### 第二轮复核的新问题

没有要负责人现在定的。
