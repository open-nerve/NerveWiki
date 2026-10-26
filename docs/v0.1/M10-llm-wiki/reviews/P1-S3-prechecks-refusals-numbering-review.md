# M10/P1 S3（预检、加序号与后台的拒绝的去处）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 从 `8e62b191` 到 `607e761e2e3e20b59baa7e7bb4a54895a963728c`；四个实施与修复提交依次为 `22ca92af`、`c3ae49f9`、`ba5cded6`、`607e761e`。S3 于 2026-10-26 合并为 `a46c3d1d`（完整 SHA `a46c3d1d076a97e793fafaa6fb2984f02876b2b3`），分支文档 `47945074`；[00 号文档](../00-M10-design.md)第 12 节。依据 [01-P1](../01-P1-contract-frame.md) 第 0–2 节、3.25–3.29 的 S3 部分、4.3、第 5 节的四个报告字面量、6.3/6.5、第 7 节的 S3 测试与变异、第 8 节、10.2 与第 9 节的移交，[00-M10](../00-M10-design.md) 4.6–4.9 与第 7–10 节，[总体设计](../../v0.1-design.md) 13.1/13.4 及 [工作流](../../claude_to_codex/02-workflow.md) 3.3–3.6 |
| 审查方式 | 任务范围的 A/B/C 各在独立工作树读代码、调用者和设计，先不读作者报告；完整 Step 的三方审查单列在下，不由任务范围的审查代替。任务 A：预检与事实；任务 B：后台拒绝、报告、网页及最后一跳；任务 C：三处加序号与 SQL 不变量。B 对发现另做外部 Go overlay 短测试，修复之后由原 B 用同一个 overlay 复核 |
| 日期 | 2026-10-26 |
| 当前结论 | 任务范围的审查与 B-M1 修复复核、独立的完整 Step A/B/C 审查均已核对。完整 Step 没有经过核实的 Important、Minor、Nit 或 Q，没有剩余产品修复；主持者接受结论。受测头 `607e761e` 的 13 项本机门禁、三份各两次本机测量和分支 CI 十个作业通过；两个契约提交的精确 HEAD 已追溯通过 `gen-check`。S3 已合并为 `a46c3d1d`，首次 main CI 已通过；第二次 main CI 的 costs 失败、e2e/image 跳过的证据保留；决定 34 的新完整 push CI `40328760108` 全绿，三次全部样本及每对平均核对后接受 S4 入口，数值在末节。IndexItems 与 grep 的 M10 稳定性收尾义务仍在。 |

## 任务范围的审查与处置

| 编号 | 级别 | 具体情形与依据 | 处置 |
|---|---|---|---|
| Q-A1 | Q／实施的读路径 | `page/app/revert_changeset.go:254–268` 的局部 `Grants` 回调在需要 `page.manage_source` 时另读一次 `WorkspaceOf`；现有 `Writer.Allowed`（`unit.go:142`）丢弃工作区，而真实授权者需要工作区与笔记本两个目标。原来 3.25/7.6 的“只多一次 StateOf”简写没有写出这次条件读取 | 主持者接受，决定 32；在 3.25、7.6、3.32 与结果 10.3 明写。锁外的答复只属于这一遍，锁下的单元用自己的 `Grants` 重问 |
| B-M1 | Minor | `backgroundRun.stopped` 或 `importRun.unit` 已记下拒绝，`jobRun.finish` 随后选择 timeout、interrupted 或 cancelled，却保留旧 `Report.Refusal`。这与非 refused 的终态为 null 的契约冲突。B 的四行外部短测试中普通拒绝通过、三个覆盖情形在运行时失败，先前已提交的计数仍为 1 | 主持者接受；`ba5cded6` 在 `transfer/app/job_run.go:207` 选定最终 failure 后、`FinishJob` 前清掉非 `FailureRefused` 的 refusal。新增 `TestAJobsRefusalBelongsToItsFinalFailure`；原 B 在 `ba5cded6` 以相同 overlay 复核四行均通过，正式变异 86 在最终代码头又抓到漏清理 |

任务 A 在 `22ca92af` 没有 Important、Minor 或 Nit；Q-A1 是读路径的设计表述问题。任务 B 初审 `c3ae49f9` 有上表的一条 Minor，没有 Important、Nit 或别的 Q；修复复核没有剩余发现。任务 C 在 `607e761e` 的 Critical、Important、Minor 均为 0，没有新增发现。三份都是各自任务范围的审查结论。

核对过的要点：预检保留领域错误、界线、契约、锁与预算的次序；锁外额外授权的失去可见性答原操作 404，数据库错误原样；锁下没有复用锁外的放行。`RevertFacts` 复用调度器的模拟，按前面的操作改变之后的树建事实。`revertDefect` 不吞标出的拒绝；`treeError` 的九个哨兵仍在前，已转换的错误及其 wrapper 原样交回；四处单行 `nolint:errorlint` 只为钉住对象身份。`Refused.Unwrap` 保留导入文件的清理判定。存储与 HTTP 收自由字符串的码，普通报告显式 `null`，网页未知码含原码。三处加序号只在 LLM Wiki 实际落到根下时启用，通用 `under` 不变。没有另列的未核实怀疑或过度设计发现；完整 Step 审查另记。

任务范围报告保存在树外 `/home/parallels/.local/state/nervewiki/m10/s3-task1-review-a.md`、`s3-task2-review-b.md`、`s3-task2-review-b-fix1.md`、`s3-task3-review-c.md`。实施记录与逐条命令在 `s3-evidence/task1`、`task2`、`task3`。这里记录实际结果，不把实施者报告里当时的“尚未提交”“未跑变异”等历史状态当作最终状态。

## 完整 Step 的独立审查

A、B、C 各在独立工作树审查 `8e62b191..607e761e` 的四个提交、全部 51 个改动文件。三位先读设计、完整 diff 与调用者，之后才核对原始测试/变异/门禁数据；不以作者报告、此前任务审查结论或这份文档作理由。本轮与任务范围 A/B/C 分开：三位均无经过核实的 Important、Minor、Nit、Q，没有保留的产品缺陷怀疑或请求的代码/测试修改，主持者接受。原报告在树外 `s3-full-review-a.md`、`s3-full-review-b.md`、`s3-full-review-c.md`，原始命令和日志分别在 `s3-evidence/full-review-a/`、`full-review-b/`、`full-review-c/`。

- **A**：独立的六个 bootstrap 测试在竞态下通过，包 8.166s；另跑 page 的五个预检/授权/批拒绝测试、transfer 的三个拒绝/清理/最终结果测试、真实 PostgreSQL 往返。其临时 overlay 用测试自己的变化模型逐步建预期事实，不调用生产 `FactsOf` 或模拟器；种子 100000–102999、至多四次嵌套撤销，11,628 个计划一致（232 个纯正文计划、1,534 个带后代的移动；操作含 aside 2,008、content 10,220、delete 12,012、place 15,996、restore 12,012），并有最低覆盖数量断言。移除模拟的 place 在 seed 100002、level 1、op 1 失败，恢复通过。另用 Go overlay 移除真正的模式守卫登记，永久的第二批拒绝测试失败，原样/恢复通过；两次都是额外独立确认，分别归 85 与 88。
- **B**：独立执行真实 PostgreSQL 的详情/列表、整个 transfer HTTP 包及 page/app、page/domain、transfer/app 的定向竞态测试。SQL 探针对四个字面保留键逐个核对 live root 违规，子页、普通笔记本、已删节点与已删笔记本不违规，恢复后 `checkPages` 通过，临时探针随后移除。真正装配的加序号、预检同码、第二批拒绝、编辑锁与矩阵覆盖/畸形行反例通过，包 8.499s、命令 14.224s；另跑五条实际权限矩阵行各 13 个调用者，共 65 格通过，包 12.584s。ListJobs 去掉 refusal 的真实 SQL 变异被已有往返测试抓到，作为 87。
- **C**：十个模块顶层测试及七个真实 bootstrap 测试在竞态下通过，后者包 9.220s、命令 18.090s；真实守卫登记的文件变异由永久第二批拒绝测试抓到，按字节备份恢复，作为 88 的正式证明。真实 PostgreSQL 往返和两个网页文件的 78 项通过；Chromium 的 TR1–TR4 九个既有故事加一个临时报告探针共 10 项通过，Playwright 17.6s、包装层 18.018s。探针先经真实 API 导入/撤销，再以 SQL 存入报告分支，核对真实 HTTP 到网页的三种 P1 码、链接、未知码与缺失节点；它不冒充在浏览器自然制造每一种后台拒绝。实际拒绝产生由 bootstrap 的交错覆盖。431 个顶层测试按现行正则分成 168/120/143，无遗漏或交集。C 另补两次精确契约提交的 `gen-check`，见下。

每位的重命令都经 `heavy.sh` 的共享 mkdir 锁串行。报告区分自己执行的定向验证与读取的完整门禁证据；没有声称每人重复所有门禁、所有变异或每个历史提交的全套测试。A 没有跑浏览器，B 的临时 SQL 探针没有暴露缺陷，C 的 UI 探针由 SQL 造报告分支；均未把 S4、P2 之后的规则当成本轮已验证。三棵树最后都回到精确 `607e761e` 且干净，实际文件变异恢复，临时探针移除。没有另列的过度设计或需要扩大的产品改动。

## 主持者的决定（负责人可以改判）

编号接着 [S2 审查记录](P1-S2-guard-review.md)的第 27–31 条；与 00-M10 第 11 节的设计审查编号分开。

32. 接受 Q-A1 的条件读取：上传和撤销每遍各多一次不加锁的 `StateOf`；锁外撤销只有规则需要 `page.manage_source` 时才在这一遍自己的 `Grants` 回调里再读 `WorkspaceOf`，同一遍至多一次。理由是当前 `Allowed` 不保留工作区，额外判定仍须给真实授权者完整的目标。锁下用 `Write.Notebook` 与单元的 `Write.Grants`，不把锁外放行带进去。改正文而不把已采用的局部实现扩大成跨调用缓存。

33. 合并之后的两次 main CI 必须在 S4 开始之前测完并核对：一次是 S3 合并触发的，一次是填合并 SHA 的小文档提交触发的。主持者在树外控制记录保存两次的 run ID、精确 HEAD、各作业和 Test/E2E 步时间、三份 Test 步平均，满足 7.5 与第 8 节后才开 S4。第二次运行的结果不能写进触发它自己的提交；在下一次原已安排的 S4 文档或 P1 完成文档提交里把完整的两次样本转写到 P1 的 10.3，必须在 P1 完成之前落实。这不豁免任何门禁，不拿分支样本充数，也不为转写制造额外 CI 轮次。C 复核现行文字没有另一个要求立即转录的期限，未列为 Q。

34. 第二次 main CI `40325348498` attempt 1 在既有 M9 IndexItems 成本守卫失败；只改三份文档的 `ca831b58` 没有改变该读路径。原样本机两次诊断通过，具体的计划或运行环境原因仍未归因。主持者原拟一次失败作业重跑，但 `gh run rerun 40325348498 --failed` 被 `Resource not accessible by personal access token` 拒绝（token 缺少 Actions 写权限），没有产生 attempt 2。现授权一次实质诊断记录文档提交，正常推送并在新 SHA 上触发新的完整 CI；这是对实际失败与权限限制的有界处置，不是 attempt 2，也不修改代码、30 ms 阈值或门禁。保留失败运行中通过的 bootstrap 样本及后来全部样本，不挑选较快的一对；新运行全绿且样本分析满足 7.5/第 8 节之前不开 S4。若同一失败再现，不再推文档追绿，转入窄范围的 PostgreSQL 实际计划、服务端执行与客户端/运行环境耗时归因。即使新运行全绿，M10 收尾也须核对 IndexItems 守卫稳定性，与既有 grep 义务并列。决定 33 继续保留：树外记录全部 run ID、精确 HEAD、作业与 Test/E2E 时间和平均，下一次已安排的 S4/P1 完成文档转写，P1 完成前落实；此次一次性处置不形成循环。

## 接受与推后的

接受的实施组织差异列在 [01-P1](../01-P1-contract-frame.md) 3.32：纯领域的 `RevertFacts` 复用 `revertSim`，没有另一份调度器；替身记 `StateOf`（不带 nb），零 UUID 写 `uuid.UUID{}`，保留键来自 `ReservedKeys()`；持久化与 HTTP 的拒绝往返放在指定文件的相邻测试函数中，原来的冲突与锁往返保留。B-M1 是修复后的报告语义，不是放宽 null 的契约。文档的时机按工作流 3.5–3.6，分支先写结果、合并后填确切的合并提交。

S2→S3 的实现窗口由本次代码覆盖：根下导入/恢复加序号及 `checkPages` 的规则使 A-M2 的非法根名不再产生；同步撤销的两遍预检与混合编辑锁/模式拒绝的一行覆盖 A-M3；开始后台撤销与任务计划共用锁外预检；两批之间再失去管理员权限时由单元守卫拒绝、报告 `refused` 并保留第一批。上述窗口随 `a46c3d1d` 进入 main 并通过首次 main CI，合并记录见 00 第 12 节。

S4、P2 之后的来源区规则/去重、M11 移交与 M10 收尾均未在本 Step 宣告完成。M5、M8、M9 三份移交仍为 open；总体设计补回此前漏写的 S2 交错 5–6，并写入 S3 的交错 7。没有改历史 S1/S2 审查记录。

## 反向对照

原计划与初次修复的变异在受测代码头 `607e761e2e3e20b59baa7e7bb4a54895a963728c`，42–53（51 分 a/b/c）及新增 85/86 共 16 个变体、18 次指定 catcher 均被抓到。每次先以原样跑同一命令退出 0，改坏后指定运行时断言退出 1，按备份逐字节恢复后同一命令退出 0；所有 final-state 都是同一 HEAD、干净状态且来源 SHA-256 一致。没有编译/安装失败或 timeout 充数。Go 的每条都用 `-v -race -count=1 -run` 指定下表名字和相应包，并经 `heavy.sh`；网页 52 的实际命令是 `pnpm --filter @nervewiki/web exec vitest run src/pages/notebook/transfer-background.test.tsx --reporter=verbose`。

| # | 改坏 | 指定测试 | 实际失败 |
|---|---|---|---|
| 42 | 上传略过契约 | `TestAnUploadsCheckRefusesAReservedTitle` | 根下 index 答 nil、应为 page.virtual_file |
| 43 | 锁外契约挪到编辑锁之后 | `TestARevertsChecksRefuseTheContract` | start/held_template 与 plan/held_template 答 page.locked、应为 page.protected |
| 44 | 撤销解析预算在契约之前 | `TestTheServersLimitsComeAfterThePrechecks` | 写者答 503 server_busy、应为 403 page.source_admin_only |
| 45 | revertDefect 不认标出的拒绝 | `TestARevertJobsBatchPassesAGuardsRefusal`；`TestTheSourceRootsPutBackIsRefusedInALaterBatch` | 模块读不到拒绝；真实第二批 failure=internal、put_back=1 |
| 46 | treeError 在哨兵之前认拒绝 | `TestTreeErrorKeepsTheLockFirst`；`TestABackgroundRevertAndAnEditSession` | 锁成为 refusal；真实任务 failure=refused、lock=null，应为 page_locked |
| 47 | treeError 重新包装已有 transfer 拒绝 | `TestTreeErrorKeepsTheLockFirst` | 原错误与 wrapper 的对象身份不相同 |
| 48 | unitFailure 先认 forbidden | `TestABackgroundJobStopsWithItsReason` | 计划/批次的标出拒绝变成 forbidden |
| 49 | 导入不记报告的拒绝 | `TestAnImportsUnitFails` | failure=refused 而 Refusal=nil |
| 50 | 保留键加进通用 under | `TestARestoreNumbersTheReservedTitlesAtAnLLMWikisRoot` | LLM Wiki 的活页下根、同批随父恢复、后批随父恢复三个子页对照被加序号 |
| 51a | 导入给普通模式也加保留键 | `TestAnImportNumbersTheReservedTitlesAtAnLLMWikisRoot` | 普通根下四键、附件、兄弟冲突六行错误加序号；子页对照通过 |
| 51b | 同步恢复不看模式 | `TestARestoreNumbersTheReservedTitlesAtAnLLMWikisRoot` | 普通模式同步恢复的六行错误加序号；子页及 LLM Wiki 对照通过 |
| 51c | 后台恢复不看模式 | `TestARestoreNumbersTheReservedTitlesAtAnLLMWikisRoot` | 普通模式后台恢复的六行错误加序号；子页及 LLM Wiki 对照通过 |
| 52 | 网页把未知码当已知码取文字 | `transfer-background.test.tsx` | page.some_later_code 的报告不能写出带码的退路 |
| 53 | 略过锁下的撤销预检 | `TestARevertsChecksRefuseTheContract` | sync/locked_denial 答 nil，有步/写入、没有应有的锁下额外授权 |
| 85 | RevertFacts 不执行 sim.place | `TestARevertsFactsFollowItsSteps`；纯正文对照 | 删除仍含已移出的 raw，后续移动/删除用旧名字；纯正文对照通过 |
| 86 | finalizer 不清非 refused 的 refusal | `TestAJobsRefusalBelongsToItsFinalFailure` | timeout、interrupted、cancelled 三行保留旧拒绝；普通拒绝通过 |

原始目录在树外 `/home/parallels/.local/state/nervewiki/m10/s3-evidence`：`mutations-task1/final-607e761e`（42/43/44/53）、`mutations-task1/run-final-85`、`mutations-task2/final-607e761e`（45/46/47/48/49/52/86）、`mutations-task3/run-607e761e`（50/51a/b/c）。各有 metadata、results、proof、原样/变异/恢复的命令 JSON 与日志、差异和备份，以及 final-state；结果由逐条的 named test、assertions、invalid_failure、restored_exactly 与 restored_green 读取，不凭 helper 总退出码推断。

**完整 Step 审查增加的反向对照**：保留 42–86 编号，追加 87、88 两个逻辑变体，各由一个永久测试抓到。与前面的 16 个变体/18 次 catcher 合起来是 **18 个逻辑变体、20 次指定永久测试的 catcher 执行**。A 的 85/88 overlay 另外提供两次独立确认；不另造变体编号，不把其临时事实探针称作新增的永久 catcher。

| # | 实际改坏与永久 catcher | 观察到的失败与恢复 |
|---|---|---|
| 87 | B 在生成的 `transfer/adapter/postgres/gen/jobs.sql.go` 的 ListJobs 投影追加 `- 'refusal'`，不改详情、不在变异上重新生成；`TestARefusedJobsReportReadsBackAsWritten` | 已知 `page.source_admin_only` 与未来 `page.some_later_code` 两行在 `store_test.go:793` 的 ListJobs 报告读到 `Refusal:nil`，应有的拒绝缺失；普通无拒绝对照通过。原样/改坏/恢复退出码 0/1/0，命令秒数 2.866/2.033/2.903；原始与恢复 SHA-256 均为 `7595dc134b3572d3303cd5c19dbdd6a7c2435af48227050efe0c611209869cfc` |
| 88 | C 在真正的 `bootstrap/registrants.go` 把 `guards: []page.WriteGuard{mode, lock}` 改为只登记 lock（另用 `_ = mode` 维持编译）；`TestTheSourceRootsPutBackIsRefusedInALaterBatch` | 五次任务都成功走过第二批，测试在 `wiki_background_test.go:54` 以 `the demotion came after the second batch, 5 times of 5` 失败；原样/改坏/恢复退出码 0/1/0，命令秒数 4.157/8.931/4.248。原始与恢复 SHA-256 均为 `ce5562981ef495ba9709c3fe800332761dd1adeb66b7aeeabd7ccda140df5a3d` |

87 的证明是 `full-review-b/list-proof.json`、`independent-results.json` 与 list-pristine/mutant/restored.log；88 的正式证明用 C 的真实文件备份/恢复，是 `full-review-c/registration-hashes.json`、`independent-results.json` 与 registration-pristine/mutant/restored.log。路径都在上面的 `s3-evidence/` 下。两者用指定永久测试的 `go test -race -count=1 -v`，运行时失败而非编译/安装/超时。

A 的独立确认在 `full-review-a/`：同一守卫登记的 Go overlay 让永久的第二批拒绝测试失败，原样/改坏/恢复退出 0/1/0（命令 3.806/8.673/3.943s），归 88；`sim.place` overlay 让其独立事实重放在 seed 100002、level 1、op 1 失败，原样 11,628 计划通过、恢复通过，归 85。原始文件从未写入，前后字节哈希由 finally 核对，两个 hash JSON、overlay 与日志留存；没有在 overlay 上跑生成器。


3.29 的 S3 行由测试里的反例与这些变异一起核对：上传非法名、已占名先答原码，子页下照收；撤销计划的 trimmed/步数界线先于契约，契约先于编辑锁与解析预算；锁下用什么也不拒的替身守卫证明预检在第一步之前；授权失去可见性与数据库错误不变成模式拒绝。锁的哨兵、未标出的错误、重复转换的对象身份、普通授权失败、导入文件清理、普通/未来码报告与 HTTP 原始成员的显式 null 都有反例。SQL 的根下保留键反例在删笔记本之前分别改根页、子页和普通笔记本，再恢复名字；只有第一种违规。加序号的模块测试覆盖四键、大小写、附件、已加序号的兄弟、子页和普通模式。网页覆盖已知码的有/无节点与未知码。

## 修复核对

B-M1 的生产修复只有 shared finalizer 的三行。新增四行回归在修复前实际运行失败于 timeout/interrupted/cancelled 的旧 refusal；普通拒绝保留原码与节点。修复后四行均通过，每行核对第二批确实返回拒绝、只尝试两批、只提交第一批、Done=1/Total=3、计数=1；取消等待真实 heartbeat 取消运行上下文，不靠睡眠猜交错。原 B 的外部 overlay 复核命令为：

```sh
go -C server test -short -count=1 -v \
  -run '^TestReviewBRefusalClearedWhenFinalOutcomeChanges$' \
  -overlay /home/parallels/.local/state/nervewiki/m10/s3-task2-review-b-evidence/overlay.json \
  ./internal/modules/transfer/app
```

初审退出 1，包 0.021s；`ba5cded6` 上复核退出 0，包 0.019s。修复后的聚焦竞态测试还覆盖后台此前批次、导入拒绝的文件清理、超时/停止/取消；真正装配的后台第二批拒绝与既有编辑锁测试也通过。最终代码头的变异 86 另有原样、改坏、恢复三次的证据。

## 实测与交错

最终代码头 `607e761e` 的新 bootstrap 测试通过 `heavy.sh` 串行实测：`go test -race -count=1 -v -run '^(TestPrechecksAnswerTheGuardsCodes|TestTheServersLimitsComeAfterThePrechecks|TestTheSourceRootsPutBackIsRefusedInALaterBatch|TestTheReservedTitlesAreNumberedAtAnLLMWikisRoot|TestTreeErrorKeepsTheLockFirst)$' ./internal/bootstrap`。`s3-evidence/new-bootstrap-timings-607e761e.log` 的包时间 7.335s；各项依次为同码预检 1.44s、预算次序 1.28s、第二批拒绝 2.22s、加序号 1.18s、纯函数 0.00s，全部 PASS。第二份的新增合计 1.44s、第三份 4.68s；乘设计的 1.73 仅作约 2.49s、8.10s 的估计。当前记录没有采集机器负载指标，不据此声称机器空闲；三份的本机两次测量见下一节，CI 的 Test 步另记。

`TestTheSourceRootsPutBackIsRefusedInALaterBatch` 用真正的 API/MCP 装配和 `transfer.Batch{Nodes: 1}`；`tm.behindFirstBatch` 持有笔记本行并等两个等待者，使真实成员 PATCH 排在第一批之后。第二批与改角色仍竞争：输的一支必须成功、put_back=2、来源根已复原，再以新笔记本重试；赢的一支必须 refused、put_back=1、Notes 已复原、来源根仍叫 sources，带 source root 的 id。用完 `betweenUnits` 仍没撞到拒绝即失败。两支都跑 `checkPages`；此测试没有额外调用 `checkNotebooks`。加序号的整个程序测试最后跑 `checkPages` 和 `checkBackground`。总体设计对 S2 两个交错另外记下它们实际有的 `checkNotebooks`。

## 组合根的各份

按现行 Makefile 的命名边界，同码预检在第二份；解析预算、第二批拒绝、根下加序号及纯函数在第三份，没有新增第一份测试。C 在竞态构建下实际列出 431 个顶层测试，照两条 Makefile 正则分成 168/120/143，无交集或遗漏（`full-review-c/shard-proof.json`）。受测头 `607e761e` 的三份各两次本机命令都退出 0，未提高 `-timeout`：

| 份 | 包装层第 1/2 次（秒） | 包装层平均（秒） | Go 包第 1/2 次（秒） | Go 包平均（秒） |
|---|---|---|---|---|
| 1 | 303.264 / 307.975 | 305.6195 | 302.452 / 307.169 | 304.8105 |
| 2 | 270.525 / 269.998 | 270.2615 | 269.704 / 269.194 | 269.4490 |
| 3 | 276.803 / 270.220 | 273.5115 | 276.015 / 269.449 | 272.7320 |

包装层时间来自 `full-gates-607e761e/results.jsonl` 的 `seconds`，包括 `make`/进程启动等；Go 包时间来自各日志最后的 `ok .../internal/bootstrap ...s`，不把两者混写。日志中的命令均为 `go test -race -count=1 -timeout 15m` 加各份原有的 `-run` 名称边界。这是本机两次测量；CI 的作业时间与 Test 步时间另列，不用本机平均证明 CI 的 7:30 条件。合并后的两次 CI 由合并记录补记，满足 7.5 后开 S4。

## 门禁与 CI

以下各任务的常规门禁在对应提交之前执行，当时没有后来的提交 SHA；不把这些记录倒填成在未来精确 HEAD 上执行，也不要求每个小提交重复完整 Step 门禁。

任务 1 的聚焦竞态、build/vet、完整 lint-go、short、types、knip、Vitest 和 build 均通过；Vitest 166 文件/3,349 测试。任务 2 的 `make gen`、Go build/vet、lint-go、short、lint-web/types、knip、Vitest 与 build 均通过；Vitest 166 文件/3,356 测试，聚焦竞态包涵 page/app、transfer/app、HTTP/PostgreSQL 与 bootstrap。B-M1 修复的聚焦竞态、真实第二批/编辑锁测试、Go build/vet、完整 lint-go 与 short 通过；未改前端、契约或生成文件，未重复那些门禁。任务 3 的 Go build/vet、完整 lint-go、short 与 build 通过，聚焦竞态包含加序号与 SQL 不变量。每条命令、退出码和时间见 `s3-evidence/task1/logs/task1-gates.json`、`task2/logs/task2-commands.jsonl`、`task3/logs/task3-commands.jsonl`。

如实保留的过程问题：任务 2 的初次 lint 因对象身份比较失败，四处行级说明后最终全量 lint 通过；修复的第一次聚焦命令误写一个既有测试名，最终命令改为真实的 `TestACancelledImportStopsBetweenTwoUnits`，日志确认六个目标函数均跑。任务 1 与任务 2 的完整 Vitest 都有 22 条 TimeoutNaNWarning，聚焦网页测试没有；build 的大 chunk 提示与既有基线相同，命令均退出 0。没有据此提高 timeout，也没有把失败的初次命令删出记录。

审查时没有找到两个改契约提交各自保存的 `gen-check` 记录。主持者授权 C 在精确 HEAD 追溯核对：`22ca92af5a97c4121d44d7d3e4c89ce56dd5e21c` 的 `make gen-check` 退出 0、3.196s；`c3ae49f94a9f2436507a98cdb30bb5facac06a1e` 同命令退出 0、2.836s。每次先备份生成物，之后逐字节相同、status/diff 干净，最终回到干净的 `607e761e`。这是流程证据的补齐，不是产品代码修复，也不声称最初提交时已经保存这些运行。原始命令、备份与核对在 `s3-evidence/full-review-c/gen-check-22ca92af/`、`gen-check-c3ae49f9/`、`percommit-gen-results.json`、`percommit-final-state.json`。

受测头 `607e761e2e3e20b59baa7e7bb4a54895a963728c` 的本机全套门禁 13/13 均退出 0，`full-gates-607e761e/final-status.txt` 为空。阶段与结果分别在同目录的 `metadata.json`、`results.jsonl`，每一行带受测 HEAD 和原始日志路径；本次 13 条记录没有失败或重跑替换。`make test` 的目标按流水线拆开执行，连同静态门禁、short、e2e 和第二遍的三份测量如下。表里的用时是命令包装层的 elapsed seconds，Go 包时间在上一节分开列。

| 阶段 | 命令 | 退出码 | 包装层秒数 |
|---|---|---|---|
| static-build | `make lint gen-check knip build` | 0 | 8.613 |
| race | `make test-go-race` | 0 | 320.908 |
| test-go-bootstrap-1 | `make test-go-bootstrap-1` | 0 | 303.264 |
| test-go-bootstrap-2 | `make test-go-bootstrap-2` | 0 | 270.525 |
| test-go-bootstrap-3 | `make test-go-bootstrap-3` | 0 | 276.803 |
| costs | `make test-go-costs` | 0 | 326.496 |
| guards | `make test-go-guards` | 0 | 453.055 |
| web | `make test-web` | 0 | 101.849 |
| short | `make test-go-short` | 0 | 55.763 |
| e2e | `make e2e` | 0 | 138.264 |
| test-go-bootstrap-1-second | `make test-go-bootstrap-1` | 0 | 307.975 |
| test-go-bootstrap-2-second | `make test-go-bootstrap-2` | 0 | 269.998 |
| test-go-bootstrap-3-second | `make test-go-bootstrap-3` | 0 | 270.220 |

最终 `web.log` 是 166 个文件、3,356 个测试通过，Vitest 自报 101.54s（包装层 101.849s）；逐行数到 22 条 `TimeoutNaNWarning:`，这是此次完整门禁日志的实数，不取任务门禁的数量代替。`e2e.log` 是 286 passed，Playwright 自报 2.3m（包装层 138.264s）。构建仍输出大 chunk 提示；e2e 还有 NO_COLOR/FORCE_COLOR 的提示，各阶段退出码仍为 0。这里记录本机结果，CI 结论另列。

分支 `m10-p1-s3` 的 [run 40311912868](https://github.com/open-nerve/NerveWiki/actions/runs/40311912868) 对象是精确头 `607e761e2e3e20b59baa7e7bb4a54895a963728c`，2026-10-26 11:01:27 UTC 开始、11:18:30 UTC 完成，十个作业都是 success。独立保存的 `s3-evidence/branch-ci-607e761e/run.json`、`jobs.json`、`timings.json` 相互核对了 HEAD、结论和各起止时刻；作业与测试一步分开列：

| 作业 | 结果 | 作业时长 | Test/E2E（或注明的一步） |
|---|---|---|---|
| web | success | 4:16 | Test 3:44 |
| server (costs) | success | 12:04 | Test 9:06 |
| server (bootstrap-2) | success | 6:55 | Test 6:32 |
| server (race) | success | 6:26 | Test 6:07 |
| server (bootstrap-1) | success | 6:26 | Test 6:00 |
| server (short) | success | 3:18 | Test 3:01 |
| server (bootstrap-3) | success | 6:15 | Test 5:54 |
| server (guards) | success | 12:55 | Test 12:36 |
| e2e | success | 4:04 | E2E 3:10 |
| image | success | 1:41 | Image smoke 1:33 |

三份 Test 步为 6:00、6:32、5:54，距 7:30 分别 90、58、96 秒；这是分支的一个样本，与合并后的两次 main CI 分开。等待脚本曾收到一次空响应，`wait.log` 留下一条 JSONDecodeError，随后恢复并记录 completed success；另存的 API 结果独立确认十个作业成功。这是观察脚本的瞬时问题，没有产品 CI 作业因此失败，也没有为此重跑 CI。

S3 已于 2026-10-26 合并为 `a46c3d1d`（完整 SHA `a46c3d1d076a97e793fafaa6fb2984f02876b2b3`，父提交 `8e62b191`、`47945074`）。分支文档的 [CI 40318346909](https://github.com/open-nerve/NerveWiki/actions/runs/40318346909) 在精确头 `479450745b11ddabee8e2cc386482f9204d12556` 上十个作业全部 success；组合根第一、二、三份的 Test 步为 6:10、4:19、6:26（作业 6:37、4:36、6:48），E2E 步 3:13（作业 16:00）。合并后的首次 main的 [CI 40322279159](https://github.com/open-nerve/NerveWiki/actions/runs/40322279159) 在精确头 `a46c3d1d076a97e793fafaa6fb2984f02876b2b3` 上十个作业全部 success；组合根第一、二、三份的 Test 步为 7:14、6:33、4:08（作业 7:32、6:51、4:29），E2E 步 3:50（作业 4:39）。第二次 main 的 [CI 40325348498](https://github.com/open-nerve/NerveWiki/actions/runs/40325348498) 在精确头 `ca831b58bdf5c396648daa592b64820d4d93ad0b` 的 attempt 1 失败：仅 costs 的既有 M9 IndexItems 成本守卫超过 30 ms，七个作业通过，e2e/image 跳过。组合根第一、二、三份 Test 步为 306s、393s、402s（作业 326s、412s、421s），与首次 main 的 434s、393s、248s 的平均为 370s、393s、325s；这两次的分片均值低于 7:30，但当时整套 CI 尚未全绿，主持者未据此开 S4。Actions 重跑因 token 缺少权限被拒绝，未产生 attempt 2。决定 34 的新完整 push [CI 40328760108](https://github.com/open-nerve/NerveWiki/actions/runs/40328760108) 在精确头 `f2fe618040945923c5d32fb438d78632645210fd` 上十个作业全部 success（attempt 1；新运行，不是失败运行的 attempt 2）。组合根第一、二、三份 Test 为 300s、390s、396s，作业为 320s、415s、415s；E2E 步 223s、作业 757s。三次 main 的全部 Test 样本分别为第一份 434/306/300s、第二份 393/393/390s、第三份 248/402/396s；按运行 1+2、1+3、2+3 的每对平均依次为第一份 370/367/303s、第二份 393/391.5/391.5s、第三份 325/322/399s，全部低于 450s。三次平均为 346.667/392/348.667s，保留失败运行的成功分片，没有挑最快的一对。主持者在开 S4 前核对并在 `s3-evidence/postmerge-all-samples.json` 接受入口；本次按决定 33–34 转写全部已发生的记录。失败运行 `40325348498`、Actions 重跑的权限拒绝及原样本机诊断仍保留；M10 的 IndexItems 与 grep 稳定性收尾义务未关闭。这些是 S3 的样本，不代替 S4 合并之后的两次测量。

### 第二次 main CI 的失败与诊断

[CI 40325348498](https://github.com/open-nerve/NerveWiki/actions/runs/40325348498) 的 attempt 1（`ca831b58bdf5c396648daa592b64820d4d93ad0b`）仅 [costs 作业 121580501180](https://github.com/open-nerve/NerveWiki/actions/runs/40325348498/job/121580501180) 失败，七个其他作业 success，e2e/image skipped。`TestIndexOfAHundredThousandPagesTakesItsTime` 的 analyzed 阶段三个 IndexItems 批次为 32.443486、34.342778、35.733972 ms，中位数 34.342778 ms 超过 30 ms；每个批次是读取 100 页的 20 次调用，三个批次都超线，不把它说成单个异常读数。这是 `test-go-costs` 的既有 M9 守卫，与先前 `test-go-guards` 的 grep 失败分别记录。

从首次全绿 main `a46c3d1d` 到 `ca831b58` 只有三份 Markdown 文档改变；从 S3 起点 `8e62b191` 到当前，相关 linking 读路径、SQL、迁移、依赖、Makefile 与 CI 工作流均未改变。已有三次全绿 CI 的 analyzed 中位数依次为 25.812651、11.902565、25.824862 ms。主持者原样运行同一测试 `-count=2`，两次通过，analyzed 中位数 9.546083、9.826583 ms（测试 65.55s、63.85s，包 129.424s）。这些证据没有定位 S3 回归；对 runner 波动的余量不足是当前解释，但失败时的计划、CPU 调度、I/O 或 Go GC 尚未量出，不能据此宣称原因已解决或守卫已稳定。本次不改阈值和代码。

| 合并后的样本 | 精确 HEAD / run / attempt | bootstrap-1 Test / job | bootstrap-2 Test / job | bootstrap-3 Test / job | E2E 步 / job | 整体 |
|---|---|---:|---:|---:|---:|---|
| 首次 main | `a46c3d1d076a97e793fafaa6fb2984f02876b2b3` / `40322279159` / 1 | 434s / 452s | 393s / 411s | 248s / 269s | 230s / 279s | 十个作业 success |
| 第二次 main | `ca831b58bdf5c396648daa592b64820d4d93ad0b` / `40325348498` / 1 | 306s / 326s | 393s / 412s | 402s / 421s | skipped / 无有效时长 | costs failure；七个 success；e2e/image skipped |
| 第三次 main（决定 34 的新完整 push） | `f2fe618040945923c5d32fb438d78632645210fd` / `40328760108` / 1 | 300s / 320s | 390s / 415s | 396s / 415s | 223s / 757s | 十个作业 success |
| Test 平均：1+2 | 保留失败运行的成功分片 | 370s | 393s | 325s | skipped 不充作样本 | 三份均低于 450s |
| Test 平均：1+3 | 第一次与新完整运行 | 367s | 391.5s | 322s | — | 三份均低于 450s |
| Test 平均：2+3 | 失败运行与新完整运行 | 303s | 391.5s | 399s | — | 三份均低于 450s |
| 全部三次 Test 平均 | 不舍弃任何一个分片读数 | 346.667s | 392s | 348.667s | — | 全部每对平均核对后接受 S4 入口 |

上述第三次运行是决定 34 允许的新完整 push CI，保存于 `s3-evidence/main-ci-f2fe6180/`；所有三次及每对平均在 `s3-evidence/postmerge-all-samples.json`，主持者在 S4 开始前已核对 latest main 全绿与所有每对平均低于 450s。本次是决定 33–34 要求的既定 S4 文档转写，保留原失败及权限拒绝，没有为转写制造额外运行；M10 的两项成本守卫稳定性义务仍未关闭。

原始证据保存在树外 `s3-evidence/main-ci-ca831b58/attempt-1/{run,jobs,timings,costs-annotations}.json`、`costs-failed.log`，两次本机诊断为该运行目录的 `index-cost-local-diagnostic.log`；首次 main 原始文件在 `s3-evidence/main-ci-a46c3d1d/`。独立只读诊断为 `ci-index-cost-recon.md`，主持者的有界处置及 Actions 权限拒绝记录为 `s3-evidence/main-ci-ca831b58/diagnosed-retry-decision.json`（均相对 `/home/parallels/.local/state/nervewiki/m10/`）。保留这些失败证据；下一次全绿不会自动取消 M10 收尾的稳定性义务。
