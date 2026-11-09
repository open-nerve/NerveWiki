# M11/P1 S2（发起与读（服务端））：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m11-p1-s2`（实施的 7 个提交：wiki 模块的第一张表（迁移 00057、wiki 的 sqlc 一项与 `wiki/adapter/postgres`、授权、清理器、notebook 与 page 两个模块的笔记本删除订阅者）`1423abec`，notebook 的 `UpgradeStates`（`Lock`、`Start`、`Publish`）`26f6762b`，page 的基线、会话与根下的键（`Capture`、`Protect`、`SetWanted`、`Analyze`、按笔记本的两条会话的查询、`RootNodes`、`SeedSpec.Upgrade`）`35c290fb`，lint 的 `computeIn` 与 `prepareIn`、`Runs.TryHold`、按页的待建数与第二遍的读 `LintAreaWantedEdgeHeads` `4488279f`，`startUpgrade` 与 `getUpgrade`（契约、`wiki/adapter/http`、接线、`checkPages` 的七条升级规则、矩阵的三行、组合根的六个 `TestUpgrade…` 与 `TestPageViolationsCatchEachUpgradeGap`、运行时角色的发起、五个码的两种语言的文字）`0778f5e2`，发起的成本守卫与发起自己的期限 `1c4d35c0`，e2e 的 UP1（API、MCP）、UP2（API）与 UP4 的删除一步 `c5a81c7a`；推送之前改写过两次分支的历史：逐个提交的核对发现第 5 个的 `TestLintMatrixTargets` 没读升级的行、第 6 个的 errorlint，第二次 `make test` 发现 `checkPages` 在退到 53 的库上读升级的列，都并回第 5、6 个提交；从 `main` 的 `5795229c` 开始，审查时在 `c5a81c7a` 上，107 个文件、加 7,705 行、减 173 行。修复一轮四个：S2C-I1、S2C-M1、S2A-M1 = S2C-N3、S2A-N1 与 S2C-N2 `219d6f28`，S2A-M2、S2A-N2 与 S2C-N1 `99bf2d54`，S2B-M2 与 S2C-N4 `0da54746`，S2B-M1、S2B-N1–N3 与 S2A-N3 `42529055`（16 个文件、加 656 行、减 207 行）；修复第二轮一个：S2A-N4 `9b3000c1`（只改 `bootstrap/upgrade_start_cost_test.go`）；一共 12 个；合并 `9c7c6821`），对照 [01-P1-freeze-start-rollback.md](../01-P1-freeze-start-rollback.md) 第 0 节的 S2、2.1、3.9–3.15、3.22–3.24、4.2、第 5 节、6.2、6.5、第 7 节的 S2 的各行（7.3 的第 18、22–36、62、63、67、82 条）与第 8 节，[S1 的审查记录](P1-S1-freeze-guardrails-review.md)，[00-M11-design.md](../00-M11-design.md) 第 3 节、4.4、4.7、4.14、4.16、4.17、第 9 节与第 11 节（第 219、227、232、235、236 条），[v0.1 总体设计](../../v0.1-design.md) 第 13 节，实施者的报告（20 条偏离、逐个提交的核对、门禁、变异的表、实测、CI、留下的五项） |
| 审查方式 | 三位审查者（Opus）并行、只读，各在自己的分离的工作树里做实验（S1 时的三个工作树挪到 `c5a81c7a`），用到 Docker 的都经 `heavy.sh`，先读设计与代码、自己判断之后才读实施报告：服务端的正确性（A：沿 `wiki/app/start_upgrade.go` 全文逐步对照设计 3.14 第 1–16 步与 00 号文档 4.4、第 219、227、232 条；`platform/postgres/tx.go` 的 `WithinTx`、请求的期限与 `ProblemOf`、配置的校验；notebook 的 `UpgradeStates`；page 的基线、会话、`UpgradeOwn` 与删除的订阅者；lint 的 `compute`、`computeIn`、`prepareIn` 与改造之前逐行比对，`TryHold` 对照 `Do`、`work`、`finish`、`Stop`；linking 的两遍、`WantedOf` 的段与游标逐个推过；wiki 的读、删除、清理与 00057；接线；一个临时的交错探针 `TestRevAS2StartAndDeletion`：以 `interleaveOn(notebookRow)` 持住笔记本行，发起与删除笔记本两种先后各一次，删除先时发起答 404、不留行，发起先时 wiki 的行与笔记本同一刻软删除、`upgrade_nodes` 删光，都以 `checkPages` 与 `checkNotebooks` 收尾，抓到 B2、B7，审完删去；模块级变异 28 个 W1–W10、L1–L10、N1–N3、P1–P3、WA1、WA2，组合根 8 个 B1–B8）；数据、迁移、契约与矩阵，计划与成本（B：00057 与设计 3.10 逐字比对，14 个约束的每个合取、每个 `=` 的两个方向对着 31 个反例在真库上判"恰好违反一个合取"，部分唯一索引的谓词逐项改坏；成本守卫一轮带 `-v`；一个临时的探针 `TestReviewBCapturePlans`：5 万页的笔记本单独一本与旁边另有 200 本各 2,500 页（`nodes`、`page_contents` 各 55 万行）两种库 × 从未分析与分析过 × 两种计划，以 `PREPARE` 加 `EXPLAIN (ANALYZE, BUFFERS)` 取 `CaptureBaseline`、`RootNodesByKey`、`LiveSessionsOfNotebook`，跑完删去；在 `schema_test.go` 里临时加五个反例验证建议的写法，从备份复原；变异 g1–g10、k1、k2、c1–c3、r2、mx2、cp-all）；测试、变异、e2e、网页、逐个提交、CI 与量的数（C：在临时的分离工作树 `nerve-wiki-rev-c-tmp` 里（用完删去）7 个提交各自 build、vet、`make lint-go`、`make gen-check`、`make test-go-short`，第 5、7 个另 `make lint-web`、`make knip`、`make test-web`；第 1–6 个在竞态下跑各自碰到的包与组合根的测试，第 4 个另跑不带竞态的 `TestLintPlansOfTheAreaPass`，第 5 个另跑不带竞态的 `TestIngestMigrationCostsLongHistory`（`costs-3` 那一行），第 6 个另跑成本守卫；头上 `make e2e` 一次；`go test -race -list` 照 `Makefile` 分份；读分支的 CI 运行 42554470668 与分份调整之后 `main` 的 42530568840，18 项的时间与每项的 CPU；两个临时的探针：期限落在 `States.Lock` 的语句之中（测试持住笔记本行）与领域的拒绝在回滚越过期限时（替身回滚多花 100 ms、期限 50 ms），跑完删去；重跑设计 7.3 的 S2 行 13 行（16 种写法），自己想的 16 种）。修复核对一轮在 `42529055` 上，三位各在自己的工作树里，核对自己的发现与自己范围里别人的，重跑自己存活过的变异、另加新的：A 在模块层重跑 W1、W3、W4、W6、L4、L5、L7、L8、L10、WA2，另把守卫的计划核对抄进暂存目录的独立包、喂合成的计划（S2A-N4 由此而来）；B 重跑 g1–g10、k1、k2、c1–c3、cp-all，新加 rk、ds、ls，探针 `TestReviewBCapturePlans` 比较新的 `LATERAL` 与"普通连接加 `ORDER BY n.id`"（旁边 9 本各 5 万页与 200 本各 2,500 页两种库），守卫一轮带 `-v`；C 在临时的分离工作树里跑 4 个提交各自的门禁与竞态下碰到的包、`0da54746` 上的守卫，重跑 12 次变异（d26b、24a、24b 各两半，o24、o2b、o3、n1、d31、x1），读 CI 运行 42571990070。修复核对第二轮在 `9b3000c1` 上，只由 A 核对（S2A-N4 是 A 的）：读那一个提交，把头上的 `startCostPageScans`、`startCostNode`、`scans`、`bitmaps` 原样抄进暂存目录的独立包、喂六种合成的计划，在自己的工作树里重跑 r1、r2（`-short` 下的单个测试，不经 `heavy.sh`），没有重的运行；守卫本身照说明不重跑，用实施者的数与分支的 CI。三位核对完工作树都干净、没有 `*.mutbak` |
| 日期 | 2026-11-09（实施、审查、修复两轮与修复核对都在这一天） |
| 结论 | Important 一条（S2C-I1）：发起"自己的期限"的两个测试只让期限落在两条语句之间，抓不到第 227 条要防的那一种（在事务里另套一层期限时，期限落在一条语句之中，pgx 关掉连接、`ROLLBACK` 失败，答 500 并记 error）；C 的 d26b 两半都照过，它的探针在 `States.Lock` 的语句之中到期，原样答 503，d26b 下答 500。修复一轮在组合根加 `inside a statement` 一格。Minor 五条（S2A-M1 与 S2C-N3 是同一件事，按 Minor 处置）：名额持在什么时段、503 在码序里的位置没有测试（S2A-M1 = S2C-N3）；第二遍对目标的判断只在第一遍有测试（S2A-M2）；00057 有五个合取的方向没有只违反它的反例（S2B-M1）；成本守卫的库里只有这一本笔记本，`Capture` 丢了笔记本的键也照过，库里有别的笔记本时它分析过的计划读整张 `page_contents`（S2B-M2，决定 240）；"领域的 409 照答"没有测试（S2C-M1）。Nit 十条（S2C-N3 并入 S2A-M1）：九条在代码与测试里改（S2A-N1–N3，S2B-N1–N3，S2C-N1、N2、N4），S2C-N5（第 6 个提交的标题没写生产上期限的改变）记下。代码本身三位都没有查出错，问题都在"测试钉住了什么"与 `CaptureBaseline` 的计划上；修复一轮四个提交，唯一的生产的改动是 `CaptureBaseline` 改为 `LEFT JOIN LATERAL (… LIMIT 1)`。实施者的 20 条偏离由三位在各自的范围里判过，都对；第 16 条（发起的期限改为请求的四分之三、守卫减为 5 万页、10 万页进 M12 的性能移交）照第 232 条与负责人 2026-11-06 的"调整测试用例量级或者时限即可"。建成与修复一轮的两次 CI 都全绿，组合根七份的测试一步都在 450 秒之下。主持者的 3 个决定见下（第 240–242 条，接着 S1 的第 236–239 条）。修复核对一轮：旧发现都关了（只改文档的部分在本提交），新的一条 Nit（S2A-N4：守卫对 `page_contents` 的核对只看索引名，比决定 240 的"逐个经主键读"宽；B、C 把同一处列为未核实的怀疑）。修复第二轮一个提交（`9b3000c1`），本机门禁照决定 242；第二轮修复时本机的根分区满了一次（Go 的构建缓存涨到 59 G），修剪缓存之后重跑通过，`06-environment.md` 第 3 节随之加一句。修复核对第二轮（A）：S2A-N4 关了，没有新的发现。修复核对两轮就结束。三次分支的 CI 都全绿。合并 `9c7c6821` |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| 编号 | — | （编号照审查者的报告：S2A-M1、M2、S2A-N1–N3，S2B-M1、M2、S2B-N1–N3，S2C-I1、S2C-M1、S2C-N1–N5；修复核对一轮的 S2A-N4。修复一轮照主持者的修复说明（`s2-review/fix-1.md`）分四个提交，第二轮照 `fix-2.md` 一个提交。同一件事合为一个修复：S2A-M1 与 S2C-N3。下面的代码位置是 `42529055` 的，路径相对 `server/internal/`，迁移与它的测试相对 `server/`） | — |
| S2C-I1 | 重要（测试） | 发起"自己的期限"的两个测试只让期限落在两条语句之间：组合根的 `TestUpgradeStart/its_own_deadline`（`slowCapture`）与 `wiki/app` 的 `its-own-deadline` 都在 `Capture` 的替身里等 `ctx.Done()`，那时没有语句在跑，连接不关，`ROLLBACK` 成功，事务里另套的期限与外层的答得一样。第 227 条不许在事务里另套期限，理由正是期限落在语句之中时 pgx 关掉连接、`ROLLBACK` 失败、错误只剩文字、答 500 并记 error；生产上会超时的是大的笔记本，期限几乎总落在 `prepareIn` 的一批读、`Capture` 的 `INSERT … SELECT` 或等笔记本行的那一条语句之中，第 232 条说的"运维由此看得出不是一时的忙"靠的就是这一支答 503 加一行 warn。C 的 d26b（`WithinTx` 开在请求的 ctx 上，`fn` 里另套同长的期限）两半都照过；C 的探针（测试持住笔记本行，发起在 `States.Lock` 的 `FOR NO KEY UPDATE` 之中到期）原样答 503、恰一行 warn、没有 error、什么也没写，d26b 下答 500 `internal_error`、一行 ERROR（"…failed to deallocate cached statement(s): conn closed"）。实施的第 26 条变异（`fn` 里另套 `Timeout*9/10`）被抓到，是因为内层先到期时 `ctxStart.Err()` 还是 nil，与连接被关无关 | 已改（`219d6f28`，修复一轮提交 1）：组合根的 `its own deadline` 分两格、共用一个团队（`bootstrap/upgrade_start_test.go:212-277`，`StartTimeout` 300 ms）。`inside a statement`（`:256`）先以 `holdNotebook` 持住笔记本行，发起在 `States.Lock` 的语句之中到期，断言 503 `server_busy` 带 `Retry-After: 1`、`before` 之后恰一行 WARN "upgrade start timed out"（`timeout_ms` 300）、没有 ERROR 级的行、什么也没写（笔记本的升级列、`notebook_upgrades`、`upgrade_nodes`、根下的 raw 与 AGENTS）、`checkPages` 通过；`between statements`（`:267`）是原来的 `slowCapture` 一格（`slowCapture` 加 `armed`，`:280-296`，没武装时照常调 `Capture`）。变异：d26b 被 `inside a statement` 杀死；实施的 m26 被两格一起抓到；d26b 的 `wiki/app` 一半照过，照预料（替身的事务不会在语句之中关连接，这一种只能在组合根钉住）。设计 7.3 第 26 条改为 `inside a statement`。修复核对 C：关（代码），d26b 在头上被 `inside_a_statement` 抓到；修复核对 A：两格的断言齐全，同意只能在组合根钉 |
| S2A-M1 = S2C-N3 | 低（测试） | 发起持着 lint 名额的时段与 503 在码序里的位置都没有测试钉住。代码对：`TryHold` 在根下的键之后（`modules/wiki/app/start_upgrade.go:203-207`），`WithinTx` 返回之后才放开（`:125-128`）。A 的 W3（`TryHold` 之后立刻放开）、W4（挪到 `Writing` 之前）、W6（不在 `WithinTx` 之后放、只靠函数末尾的 `defer`），组合根的 B5、B6，C 的 o2b（挪到 `refuseSessions` 之前）、o3（`fn` 里 `defer` 放开，提交之前）都照过：测试只在事后核对名额已放开；`each-refusal-in-its-turn` 的每一格都在名额空着时跑；组合根的 "the codes in their order" 每个码单独造一次，名字说次序，其实不钉次序（实施偏离 7：组合根不测名额占满）。后果：以后改这一段，"发起不占名额就跑计算"（第 219 条失效）或"本该答 409 的发起在忙时答 503、客户端白白重试"都会悄悄通过 | 已改（`219d6f28`，提交 1）：`wiki/app` 的替身 `slot()`（`modules/wiki/app/start_upgrade_test.go:114`）在 `Capture`、`PrepareRules`、提交之前与 `Analyze` 四处以 `TryHold` 探名额（探到空着就立刻放回），`it-writes-what-the-notebook-lacks` 断言依次是 `capture held true`、`prepare held true`、`commit held true`、`analyze held false`（`:469`）：计算与提交时持着，`ANALYZE` 之前已放开。`each-refusal-in-its-turn` 每一格再在"名额被别人占满"之下跑一次（`-a-full-slot`，`:550-575`）：第 4–8 步的拒绝（含 `transfer.busy`、`wiki.upgrade_pages_locked`、附件名的 `wiki.upgrade_title_taken`）照答自己的 409，`a-stale-index` 答 `server_busy`、调用止于 `roots`；原来那一格的核对抽成 `refusalInItsTurn`（`:383-409`），两种跑法共用。变异：W3、o3 在 `capture`、`commit` 探到空着；W6 在 `analyze` 时仍被持着；W4、o2b 让 `-a-full-slot` 的几格答 503。组合根的 B5、B6 与 W3、W4 是同一处改坏，由 `wiki/app` 抓住，组合根不另加。修复核对 A、C：关（A：503 被夹在"根下的键"之后、"stale"之前，往前或往后挪都有一格失败） |
| S2A-M2 | 低（测试） | 第二遍对目标的判断（资格、去重、相对路径）只有第一遍有测试。第二遍把第一遍的逐页循环复写了一份（`modules/linking/app/lint_edges.go:111-177`），它的数进 `upgrade_nodes.wanted`，是 P2 闸门 6 逐页比较的基线；测试的说明写"a target twice counts once, a target read as an attachment or a virtual file not at all"，可夹具里区里的页只有互不相同的普通 wikilink，这几种情形只在区外的页上、只经第一遍。A 的 L5（第二遍不判资格）、L8（不去重）、L10（不取父页的键）与组合根的 B4 都照过；L9（换页时不清 `seen`）被抓到 | 已改（`99bf2d54`，提交 2）：`modules/linking/app/lint_wanted_by_page_test.go:30-47` 给区里的页 a 加同一目标的第二条 `alpha`（`link` 种类）、`./Alpha`（由父页的键解析成 `raw/alpha`，是另一个目标）与一条已解析的 `AGENTS`，页 b 加图片 `chart.png`（读作附件）与虚拟的 `index`；LLM 契约下 a 记 3、b 记 1（`:65`），标准笔记本里只有一遍，a 记 3、b 记 2（`:105`，`index` 在标准笔记本里是待建的页）；说明改为"两遍判得一样"（`:52-57`）。两遍共用的函数没有抽。变异：L5（b 多算）、L8（a 多算）、L10（`./Alpha` 并成 `alpha`，a 少算）都被杀；组合根的 B4 与 L5 是同一处改坏。修复核对 A：期望值自己推过，对；关 |
| S2B-M1 | 低（测试） | 00057 有五个合取的方向没有只违反它的反例，单向化或去掉都不被发现（`migrations/schema_test.go` 的反例表，总注释写"each conjunct alone broken, an = broken each way"）：`baseline_pages` 与 `baseline_wanted` 的下界没有反例；`rollback_end_check` 的三个 `=`（`rollback_complete` 不是 `false` 就没有剩下的节点数、有页数就有节点数、有新节点数就有节点数）的那一向只由"a short rollback without its nodes left"一条碰到，它同时违反三个合取。B 的 g1–g5 在迁移包与 wiki 适配器的测试下都存活。设计 3.10 的清单也这样写：它对 `submission_check` 专门写了每个方向单独一行，回滚的三个数却照"`false` 缺一个数（三条）"写，所以也是设计的错 | 已改（`42529055`，提交 4）：`schema_test.go` 加 B 在真库上验证过的五条："a baseline of -1 pages"（`:851`）、"-1 wanted at the start"（同时设 `capped = false`，`:853`）、"a short rollback with nothing left"（`:890`）、"a complete rollback with its pages left"（`:891`）、"a complete rollback with its new nodes left"（`:893`）；00057 的反例共 36 个。设计 3.10 的清单在本提交改。变异：g1–g5 各被对应的一条先抓到。修复核对 B：关 |
| S2B-M2 | 低（测试与成本） | 发起的成本守卫的库里只有这一本笔记本（`startCostSeed`），计划又只核对 `upgrade_nodes` 上的 `SetBaselineWanted`（实施偏离 17）：决定 236 要计划为主，正因为只看时长分不出"丢了键、读整张表"（S1B-I1）。B 的 c2（生成的 `CaptureBaseline` 的 `n.notebook_id = $2` 写成 `(…) IS TRUE`，整表读 `nodes`）守卫六格都过，中位数 1.11–1.15 秒、`Capture` 118–135 ms，与没改时（1.16–1.19 秒、122–138 ms）分不出，page 适配器的五个测试也都过；生产上 `nodes` 是全实例的表，这样的退化让每次发起在笔记本行锁下读整个实例。不改代码时，B 的探针在旁边另有 200 本笔记本的库里，分析过之后 `Capture` 的自定义与通用计划都是 Hash Right Join、对整张 `page_contents` 顺序扫描（55 万行，181–186 ms；只有从未分析时逐个经 `page_contents_pkey`），守卫的库里看不到。c1（`SetBaselineWanted` 丢 `upgrade_id`）被守卫的计划杀，c3（第二遍丢 `source_id`）由 `TestLintPlansOfTheAreaPass` 杀，只有 page 这几条新语句落在空当里。B 定为 Minor（一次性的管理员动作，整个事务受发起的期限，退化到超期时每次发起确定地 503、记 warn），并说主持者可以照 S1B-I1 从严改判；主持者按 Minor 处置，照 S1B-I1 的做法修 | 已改（`0da54746`，提交 3，决定 240）：`CaptureBaseline` 改为 `LEFT JOIN LATERAL (SELECT c.revision, c.content_hash, c.byte_size FROM page_contents c WHERE c.node_id = n.id AND c.deleted_at IS NULL LIMIT 1) c ON true`（`modules/page/adapter/postgres/queries/upgrade.sql:12-34`）：`LIMIT 1` 让子查询不被拉平成可以哈希整表的连接，一个节点至多一行正文，不改语义；这是这一轮唯一的生产的改动。守卫的库另种九本笔记本（`startCostOthers = 9`，每本 5 万页及其正文，每十页一个编辑会话、一半已过期）：被测的笔记本占 `nodes`、`page_contents` 各 50 万行的十分之一，`edit_sessions` 4.5 万行一行也不占；`indexed_pages`、`page_links` 不另种（由 lint 的守卫与计划的测试管）。每种统计状态 × 计划模式的第一次发起，在发起的事务里另取 `CaptureBaseline`、`RootNodesByKey`、`DeleteExpiredSessionsOfNotebook`、`LiveSessionsOfNotebook` 的自定义与通用计划（`startCostPageScans`）：`nodes`、`edit_sessions` 只经索引条件含 `notebook_id` 的扫描读，`page_contents` 只经 `page_contents_pkey`，不对这三张表顺序扫描，每条语句都读到它该读的表；"从未分析"另把这三张表的统计也清掉，四张表都关 autovacuum。界照第 236 条重算为 3.6 秒；种子从 1.7 秒到 10.2 秒（这本笔记本自己的 1.4 秒），守卫从约 27 秒到约 36 秒；`ci.yml` 的 costs-2 注释记下。变异：c2 被计划抓到（"nodes read by a Bitmap Heap Scan of nodes_notebook_id_name_key_idx on \"\""：读整个部分索引）；对照 f240a（改回普通的连接）、f240b（`LATERAL` 不带 `LIMIT 1`）都报 "page_contents read by a Seq Scan"；c1 照旧被杀。修复核对 B：代码侧关；六格都是 Nested Loop，外侧经 `nodes_notebook_id_name_key_idx`（从未分析）或 `nodes_notebook_id_parent_id_idx`（分析过）的位图，内侧是 Limit 下经 `page_contents_pkey` 的 Index Scan、`Index Cond: (node_id = n.id)`；探针在两种库、两种统计、两种计划下都是这个形状，比普通连接慢约 20%（208–232 对 176–194 ms），但不随实例变大；新的 rk、ds、ls（另三条语句丢 `notebook_id`）都在第一格被计划抓到。修复核对 C：x1（与按 `node_id` 排序的子查询连接）被抓到。设计 7.1、00 号文档第 9 节与决定 240 的文字在本提交 |
| S2C-M1 | 低（测试） | 设计 7.1 的"领域的 409 照答"没有测试：把 503 的判断里的 `errors.Is(err, context.DeadlineExceeded)` 去掉（C 的 o24），两半的 `TestUpgradeStart` 都照过；替身的 `WithinTx` 只在提交时可以晚，回滚从不晚，造不出"领域的错误、`ctxStart` 已到期、请求还活着"这一格。C 的探针（替身回滚多花 100 ms、期限 50 ms、`fn` 交回 stale）原样答 409 `wiki.upgrade_stale_index`，o24 下答 503。窄：要领域的拒绝恰在期限前后、回滚跨过它才碰得到；碰到时用户看到"忙，请重试"，重试是同一个结局 | 已改（`219d6f28`，提交 1）：替身加 `rollbackLate`（`modules/wiki/app/start_upgrade_test.go:83`、`:135`），新子测试 `a-refusal-rolled-back-past-its-deadline`（`:645`）断言答 409 `wiki.upgrade_stale_index`、没有日志、名额已放开。变异：o24 被杀。7.3 加第 95 条。修复核对 C：关（代码），7.3 在本提交 |
| S2A-N1 | 细节（测试） | 503 的三个条件里"`ctxStart` 已到期"一条没有测试：去掉它（A 的 W1）`wiki/app` 的 `TestUpgradeStart` 仍全过。包着 `DeadlineExceeded`、`ctxStart` 却还活着的错误只来自 `COMMIT` 或 `ROLLBACK` 超过 `commitTimeout`（`platform/postgres/tx.go:199-203`），照 00 号文档 4.4 答 500 并记 error，替身造不出它 | 已改（`219d6f28`，提交 1）：替身加 `commitErr`（`:84`、`:141-143`），新子测试 `a-commit-out-of-its-own-time`（`:658`）：提交交回 `fmt.Errorf("commit transaction: %w", context.DeadlineExceeded)`、`ctxStart` 一分钟，断言答的是提交的错误（不是 `server_busy`）、没有日志、没有 `ANALYZE`、名额已放开。变异：W1 被杀。修复核对 A：关 |
| S2A-N2 | 细节（测试） | 第二遍的两道防御检查没有测试（`modules/linking/app/lint_edges.go:143` 的 `ResolvedID != nil`，`:149-151` 的 `s.source`）：替身与 SQL 本来就先滤掉，A 的 L4、L7 存活。都是纵深防御 | 已改（`99bf2d54`，提交 2）：替身 `lintData` 加 `areaStray`、`areaResolved` 两个开关（`lint_fakes_test.go:41-44`、`:173-174`）；子测试加在 `TestTheWantedCountsByPage`，没有照建议加在 `TestTheAreaPassReadsByChunk`（两道检查在 `Prepare` 的第二遍，后者只测读的流）：`a-page-outside-the-area-answered`（`:116`，断言 `errors.Is(err, app.ErrLintRows)`，`export_test.go:10-11` 导出它）、`a-resolved-link-answered`（`:123`，a 仍是 3）。变异：L4、L7 被杀。修复核对 A：理由成立，关 |
| S2A-N3 | 细节（测试） | `CurrentUpgrade` 里 `rolling_back` 一种状态没有测试：A 的 WA2（状态只剩前两种）`TestTheUpgradesRows` 照过；矩阵也只用 `upgrading`、`reviewing`；S2 自己的路径造不出 `rolling_back`，S3 才造 | 已改（`42529055`，提交 4）：`modules/wiki/adapter/postgres/store_test.go:89-99` 以 SQL 设成 `rolling_back`（带 `ending = 'abandoned'` 与任务 id），断言 `Current` 读得到、与期望整个相等。变异：WA2 被杀。修复核对 A：关 |
| S2B-N1 | 细节（测试） | 部分唯一索引的谓词没有钉住（`migrations/schema_test.go` 对 `00057_wiki_notebook_upgrades.sql` 的 `notebook_upgrades_notebook_id_key`）：只断言第二行 `reviewing` 答 23505、一行 `approved` 放过；B 的 g6（谓词去掉 `'rolling_back'`）、g7（去掉 `deleted_at IS NULL`）存活；`CurrentUpgrade` 的注释说它走这个索引，靠的就是谓词 | 已改（`42529055`，提交 4）：`another` 接受状态与附加的列（`schema_test.go:930-953`），加第二行 `rolling_back`（带结局与任务）答 23505、一行软删除的 `upgrading` 放过。变异：g6、g7 被杀。修复核对 B：关 |
| S2B-N2 | 细节（迁移） | 三个合取被别的约束蕴含，反例只能连带违反别的合取：`baseline_nodes >= 0`（由 `baseline_pages BETWEEN 0 AND baseline_nodes` 推出）、`report_check` 的 `state <> 'upgrading'`（由同一检查的 `submission_id IS NOT NULL` 与 `submission_state_check` 推出）、`submission_state_check` 的 `'approved'`（由 `approved_check` 与 `report_check` 推出）。"a baseline of −1 nodes"同时违反 `pages <= nodes`，B 的 g9 存活；"a report while upgrading"同时违反两个检查，测试期望 `report_check` 只因为 PostgreSQL 按名字的次序查检查约束，g8"被杀"只因报出来的名字换成 `submission_state_check`；g10 存活。都不是错的，只是总注释的"each conjunct alone"对它们不可能成立 | 已改（`42529055`，提交 4，决定 241）：三个合取保留，00057 各加一句注释写明由哪条推出（`migrations/sql/00057_wiki_notebook_upgrades.sql:39`、`:46`、`:49`；迁移还没合并，可以改注释）；反例表的总注释（`schema_test.go:840-844`）与两条反例的注释（`:849`、`:872-873`）照实写连带违反与按名字的次序。g8 照旧被杀，g9、g10 照旧存活，与决定 241 相符。设计 3.10 在本提交。修复核对 B：代码侧关 |
| S2B-N3 | 细节（测试） | `checkPages` 的升级规则有四个合取没有违反来钉（`bootstrap/interleavings_page_test.go` 的规则对 `bootstrap/page_violations_upgrade_test.go` 的表）：`upgradeRowGap` 的同状态与同笔记本、`upgradeBaselineGap` 的基线页数、`upgradeRootGap` 的"是页"；B 的 cp-all 把四个一起去掉，`TestPageViolationsCatchEachUpgradeGap` 照过（去掉合取只会少报，所以每一个单独去掉也照过）。另外注释说"finds each gap … and no other"，代码只查目标的那一句在不在 | 已改（`42529055`，提交 4）：测试的笔记本在页之外上传一个附件 `chart.png`（`page_violations_upgrade_test.go:47`，基线两个节点、一页），加三行："its row another state"（`:75`，wiki 行 `reviewing` 带提交的三列，笔记本仍是 `upgrading`）、"a baseline page more than counted"（`:100`，仍在 `baseline_check` 之内）、"the source root an attachment"（`:112`）；每行断言七条升级规则里只有它自己的报（`:139-149`），说明照实改（`:35-42`）。变异：cp-all 与分开的 cp-state、cp-pages、cp-kind 都被杀；cp-notebook（同笔记本）存活，B 原来就说可以省（主键与部分唯一索引挡着）。实施者在提交说明与报告里写它"没有一行能单独违反"，不对：wiki 行指向一本软删除的笔记本时，`upgradeRowExtra` 与 `upgradeAgentsGap` 只读活着的笔记本、不报，其余四条也不报，只有 `upgradeRowGap` 报（修复核对 B 对着七条规则的 SQL 推的，没有跑）；那一行仍可省（"接受与推后的"）。修复核对 B：关 |
| S2C-N1 | 细节（测试） | `TestComputeInTakesItsMoment` 的"`prepareIn` 不投影"一句永远不会失败：替身的追踪是累加的，标准笔记本的摘要的投影本来就不调 `paths`、`encode`，两者的追踪一样（`state nodes prepare`）。前一半（不读时钟、用给的时刻）是好的，第 31 条变异被它抓到 | 已改（`99bf2d54`，提交 2）：改用会投影的查询（orphans 一条规则、`Limit` 1、带 `PageID` 的条目，`modules/wiki/app/lint_test.go:627-671`），先断言 `computeIn` 的调用恰是 `state, nodes, prepare, encode, paths`，再用新的夹具断言 `prepareIn` 恰是 `state, nodes, prepare`。变异：n1（`prepareIn` 返回之前调一次 `project`）被抓到，C 的 d31 与实施的 m31 照旧被抓到。修复核对 C：关 |
| S2C-N2 | 细节（测试） | 第 24 条变异"抓到"，但持着锁阻塞的写法要等测试二进制的期限（回答主持者的问题；C 判它不算"能过却抓不到"）：不持锁地阻塞（24b）4 秒失败；持着 `r.mu` 阻塞（24a，实施的写法）`TestTryHoldNeverWaits` 2 秒就 `t.Fatal`，但夹具清理里的 `Stop` 要拿 `r.mu`，挂到二进制的期限（本机 60 秒，CI 的竞态一行 15 分钟）；7.2 第 17 条同步地等发起的答复，两种写法都挂到第七份的 15 分钟 | 已改（`219d6f28`，提交 1）：7.2 第 17 条在另一个 goroutine 里发起，限时 2 秒等答复，超时就 `t.Error`、放回名额、再等发起结束（`bootstrap/upgrade_interleavings_test.go:276-330`）。`TestTryHoldNeverWaits` 清理时的挂起不动（要动夹具，不值得）。变异：24a、24b 在第七份里 2.08、2.09 秒失败（"did not answer in 2 s: it waited for the slot"），实施的 m24 的组合根一半 2.06 秒；单元一半 24a 仍挂到 60 秒、24b 4 秒。修复核对 C：关 |
| S2C-N4 | 细节（名字） | 几处过时的名字：`modules/wiki/adapter/http/upgrade_test.go:134` 的注释指不存在的 `TestStartingAnUpgrade`；成本守卫的笔记本叫 "A hundred thousand pages"，规模已是 5 万页；守卫的每行日志借 `lintCostLog`，前缀 "lint index cost upgrade start"，CI 里读起来像 lint 的索引成本 | 已改（`0da54746`，提交 3）：笔记本名 "Fifty thousand pages"；守卫自己的 `startCostLog`，前缀 "upgrade start cost seed / sample / medians"；注释改指 `wiki/app` 的 `TestUpgradeStart`。修复核对 C：关 |
| S2C-N5 | 细节（提交说明） | 第 6 个提交 `1c4d35c0` 改了 `.github/workflows/ci.yml` 的一行注释，范围只写 `server`（同类的 `2beef88c` 写 `server, ci`，S1 的 `a28064e9` 也只写 `server, api, web`）；标题只说成本守卫，没说它同时把发起在生产上的期限从 `LintTimeout` 的 7.5 秒改为请求的四分之三、11.25 秒，这是行为的改变，只写在正文里 | 只记下：已推的历史不改写，写进本记录、P1 设计 10.2 与合并的说明。修复核对 C：同一种情形又出现一次，`0da54746` 也改了 `ci.yml` 的一行注释、范围仍写 `server`，一并记下 |
| S2A-N4 | 细节（测试） | 修复核对一轮的新发现：守卫对 `page_contents` 的计划核对（`bootstrap/upgrade_start_cost_test.go` 的 `startCostPageScans`）只要求扫描经 `page_contents_pkey`，不要求 `Index Cond` 含 `node_id`，也不要求这次扫描在嵌套循环的内侧、逐个节点重复；Merge Join 之下对 `page_contents_pkey` 没有条件的 Index Scan（按键序读整张表，正是决定 240 要排除的）也能过。A 把这段核对原样抄进暂存目录的一个独立包、喂合成的计划核实：Merge Join 的那一种交回 nil，Hash Join 之下的 Seq Scan 被拒。现在的 `LATERAL … LIMIT 1` 带外层的引用，拉不平成连接，生产的计划没有问题；B、C 把同一处列为未核实的怀疑（B 的"普通连接加 `ORDER BY n.id`"、C 的 x1 都逼不出这种计划） | 已改（`9b3000c1`，修复第二轮，`fix-2.md`，门禁照决定 242）：`startCostPageScans` 对 `page_contents` 另要求每个经 `page_contents_pkey` 的扫描的 `Index Cond` 含 `node_id`，且这次扫描是嵌套循环的内侧、逐个节点重复（照 `startCostScan.repeated` 的写法）；加一个不连数据库的小测试，喂 A 的合成计划（Merge Join 之下没有条件的扫描被拒；有条件、不在嵌套循环内侧的被拒；现在六格的那种放过；Hash Join 之下的 Seq Scan 照旧被拒），两项要求各自去掉时它都失败。实际：`startCostPageScans`（`bootstrap/upgrade_start_cost_test.go:522-555`）的两项要求在 `:541-543`（"…with no node_id in …: the whole index"）与 `:545-547`（"…once, not for each node on a Nested Loop's inner side"）；新测试 `TestTheStartGuardJudgesThePageReads`（`:626-667`）五格：现在六格的那种放过，Merge Join 之下没有条件的、Merge Join 内侧有条件只读一次的、Nested Loop 内侧没有条件的、Hash Join 之下的 Seq Scan 都被拒；第三、四格各只违反一项（第四格是实施者加的：没有它，去掉第 1 项时第二格仍因第 2 项被拒）。它与守卫在同一个 `!race` 文件里，不进竞态构建与组合根的七份，跑在 short 一行（`-short` 下不跳过）。变异：r1（去掉第 1 项）、r2（去掉第 2 项）各被一格抓到；B 的 c2、f240a、f240b 照旧被守卫抓到。修复核对第二轮 A：关；合成的计划在 `42529055` 上放过的 Merge Join 一种，在 `9b3000c1` 上被拒，LATERAL 的形状与 Nested Loop 内侧经主键的位图都放过，`nodes` 一侧整读与隔着 Materialize 的扫描被拒；r1、r2 重跑都被抓到 |
| 偏离 | — | 实施者的 20 条偏离（[01-P1](../01-P1-freeze-start-rollback.md) 10.2、3.26）：1 第二遍的待建过滤写在 `LIMIT 1` 的 `LATERAL` 里（没有统计时 `IS NULL` 估 0.005，自定义计划选了位图扫描加 Sort）；2 `WantedByPage` 只与 `wanted` 一条规则同用、别的组合答 `errLintQuery`，第二遍在范围的模式是 LLM Wiki 时跑；3 `wiki.Deps` 加 `Pool`；4 `transfer.busy` 的文字末句是"…start the upgrade once it is done."；5 矩阵的行用 lab 种好的笔记本、每格在库的副本上以 SQL 准备，两处覆盖的检查经 `coveredMatrixRows()` 读它们；6 `checkPages` 的违反的测试名为 `TestPageViolationsCatchEachUpgradeGap`、在第四份，升级的规则只在库里有 `notebook_upgrades` 时加；7 名额占满的 503 只在 7.2 第 17 条与 `wiki/app` 测；8 第 31 条变异的测试是 `TestComputeInTakesItsMoment`（S2 的发起不经 `computeIn`）；9 `wiki/app` 的 `TestUpgradeStart` 在 `app_test` 包里；10 夹具 `upgradeIn` 选根、写一致的 wiki 行，`endUpgrade` 记 `rolled_back`；11 S1 的测试照 `checkPages` 的新规则改 wiki 的行或加 raw；12 7.2 第 3 条的心跳答 `page.edit_session_not_found`；13 流里先是补写的 `pages`、再是 `mode`；14 运行时角色的测试先以 owner 的 SQL 加一个笔记本管理员；15 `GetUpgrade` 的回滚几列直接从行读；16 发起的期限是请求的四分之三（守卫 4.5 秒、生产 11.25 秒），守卫是 5 万页、1.5 万页在 raw 里，10 万页进 M12 的移交；17 守卫在发起的事务里核对 `SetBaselineWanted` 的两种计划，"本次之后"读作分析过这本笔记本的上一次发起，每次发起之后 `VACUUM`，`ci.yml` 加一行注释；18 e2e 的夹具与故事的细节（`upgradesOf`；UP1 的"别人"在 REST 是编辑者、在 MCP 是另一位管理员；UP2 末尾再发起一次答 200；`threeBrowsers` 不改）；19 第 62 条变异写成四种；20 第 18 条变异的 S2 一半由 7.2 第 4 条抓到 | 三位在各自的范围里判过，都对。A 判全部 20 条：1 是等价的写法、有计划的测试；2 今天只有发起传 `WantedByPage`（它交显式的契约），可以接受；3 必要；7 连带出 S2A-M1；8 理由成立；16 符合第 232 条（它预先许可"10 万页放不进时改用 `StartTimeout = request_timeout × 3/4`……记在那个 Step 的记录里，不另开决定"，负责人的"调整测试用例量级或者时限即可"许缩小守卫的规模，守卫断言了 `StartTimeout == 6 s × 3/4`）；19 可以接受；其余与代码相符。B 判 1、2、3、6、10、11、15、16、17、19：1 是 `lint.sql` 已有的写法；6 的 `to_regclass` 条件对（规则读的三张表里 `notebook_upgrades` 最新）；16 的"放不进"有据；17 只核对 `SetBaselineWanted`，就 `upgrade_nodes` 说是对的，别的表见 S2B-M2。C 判 1（反过来写的 62e 被计划的测试抓到）、7（S2C-N3）、8、16（本机 10 万页 2.16–2.41 秒乘 7763 的 2.0–2.3 倍是 4.3–5.5 秒，"4.5 秒仍放不下 10 万页"照实）、18、19（同意 7.3 第 62 条改列三个测试，成本守卫只对组合） |
| 留下的事 | — | 实施报告第 8 节留下的五项：1 Step 的文档提交；2 M12 的性能移交（10 万页的发起在最慢的 CI runner 上约 5.6 秒，过守卫配置下的 4.5 秒，生产的 11.25 秒之内）；3 第 24 条变异靠测试二进制的期限被抓到；4 `make test` 的竞态六份同时跑时第一份的两个交错的计时测试可能超时；5 S3、S4 照设计 | 1 是本提交；2 写进 00 号文档第 232 条与"写给后面的 M"；3 即 S2C-N2；4 C 核对与 S2 无关（"接受与推后的"）；5 照设计 |
| 修复一轮的说明 | — | 实施者在修复一轮的七条说明（`fix-1-report.md` 第 7 节）：1 d26b 的 `wiki/app` 一半存活（替身不在语句之中关连接）；2 c3 不再被守卫的时长抓到（这一轮通用三格 3.41–3.45 秒，界重算为 3.6 秒），由 `TestLintPlansOfTheAreaPass` 抓；守卫若也要以计划抓它，可以把 `LintAreaWantedEdgeHeads` 加进 `startCostPlanned`，说明没有要，没加；3 S2A-N2 的两个开关加在 `TestTheWantedCountsByPage`；4 "从未分析"扩到四张表；5 cp-notebook 存活；6 `ci.yml` 加一行注释；7 提交 3 推送之前重写过一次（`make lint-go` 报两个全局变量，`startCostStatistics`、`startCostPlanned` 改成函数，`--amend` 之后把提交 4 cherry-pick 上去，旧的 `02f77f7e`、`2e5854da` 作废）。另：头上第一次 `make test` 在第一份的 `TestABackgroundJobAndItsNotebooksDeletion/revert` 失败（任务在笔记本被删之前跑完了所有批），单独以 `-race -count=3` 跑三次都通过，第二次完整的 `make test` 通过 | 修复核对判过：三位都认为七条成立（A：1 只能在组合根钉；2 界照第 236 条算对、c3 由计划的测试抓，可以接受；7 两个提交都没推过，不算改写已推的历史。B：4 是必要的，`nodes` 从未分析与分析过时外侧走的索引不同；5 可以省，但"没有一行能单独违反"的说法不对，见 S2B-N3。C：同 B，并把 `LintAreaWantedEdgeHeads` 加进 `startCostPlanned` 列为可选）。`TestABackgroundJobAndItsNotebooksDeletion/revert` 的失败 C 核对与 S2 无关（"接受与推后的"）。A 另提一处文档的口径（不编号）：设计 7.1 写"种子用 SQL，准备不超过 4 秒"，现在守卫的种子共 10.2 秒（CI 22.1 秒），这本笔记本自己的只有 1.4 秒，7.1 那句在本提交照实改 |
| 修复第二轮的说明 | — | 实施者在第二轮的说明（`fix-2-report.md`）：1 小测试的第四格（Nested Loop 内侧、没有条件）是修复说明没有列的，没有它去掉第 1 项抓不到；2 小测试与守卫在同一个 `!race` 文件里，跑在 short 一行，`go test -race -list` 里没有它、不进分份；3 本机门禁照决定 242，没有重跑整个 `make test` 与 `make e2e`；4 `make test-go-costs-2` 第一次失败，不是这个提交的问题：本机根分区到 99%、只剩 1.7 G，其中 `~/.cache/go-build` 占 59 G（几天的变异与竞态构建积下来的），`TestLinksReachedDenseReadPlans` 里 PostgreSQL 报 "could not write to file "pg_wal/xlogtemp.90": No space left on device"、进入 recovery mode；实施者经 `heavy.sh` 删掉构建缓存里 12 小时没用过的条目（`find ~/.cache/go-build -mindepth 2 -type f -mmin +720 -delete`，同 Go 自己按 5 天修剪的做法，阈值更短），缓存降到 18 G、根分区剩 44 G，没有动 Docker 与别的东西，重跑通过；别的工作树下次构建会慢一些；5 这一次守卫的中位数 1.34–1.42 秒，比修复一轮两轮的 1.11–1.17 秒慢、比那次 `make test` 里的 1.31–1.50 秒略快，原因没有查 | 修复核对 A：1、2 成立；守卫的数用实施者的与分支 CI 的，在界内。根分区满的那一次记进"接受与推后的"，`06-environment.md` 第 3 节在本提交加一句：跑重的之前看一眼 `df`，满了先修剪自己的构建缓存，不动 Docker |

## 主持者的决定

负责人可以改判（接着 S1 的第 236–239 条编号；240、241 在第一轮的修复说明 `fix-1.md` 里定下，242 在第二轮的 `fix-2.md` 里）：

240. **发起的成本守卫照第 236 条"计划为主"扩到 page 的读**（S2B-M2，2026-11-09）：守卫的库里另有别的笔记本的行（九本各 5 万页），`CaptureBaseline`、`RootNodesByKey` 与两条会话的语句在每种统计状态 × 计划模式的第一次发起里、在发起的事务里取自定义与通用两种计划，都只读这本笔记本的行：`nodes`、`edit_sessions` 只经索引条件含 `notebook_id` 的扫描，`page_contents` 只经 `page_contents_pkey`，不对这几张表顺序扫描。`CaptureBaseline` 改为 `LEFT JOIN LATERAL (… LIMIT 1)`，正文由这本笔记本的节点逐个经主键读，发起的成本随这本笔记本、不随实例。时长为辅，界照第 236 条重算为 3.6 秒。第二轮把"逐个经主键读"落到核对上：`page_contents` 的扫描的索引条件含 `node_id`、在嵌套循环的内侧逐个节点重复（S2A-N4）（[01-P1](../01-P1-freeze-start-rollback.md) 3.12、7.1，7.3 第 103–105、110 条；00 号文档第 9 节的同一行）。
241. **被别的约束蕴含的合取可以为读得清楚而保留**（S2B-N2）：在迁移里注明由哪条推出；13.4 第 1 条"每个合取单独一条反例"对它们不适用，反例照实写连带违反的检查。00057 的三处：`baseline_nodes >= 0`、`report_check` 的 `state <> 'upgrading'`、`submission_state_check` 的 `'approved'`（01-P1 3.10）。
242. **只改成本或守卫测试本身的修复提交，本机门禁取那一行加 lint 与 short**（`fix-2.md`，2026-11-09）：只改成本或守卫测试本身、且该测试只由一行跑的修复提交，本机门禁取提交单独为绿（build、vet、`make lint-go`、`make gen-check`、short）、`make lint gen-check knip build` 与经 `heavy.sh` 的那一行与 `make test-go-short`，其余由分支 CI 的全部各项担保；动生产代码、契约、网页或 e2e 的照旧跑完整的 6.5。S2A-N4 的修复是第一次照它（只改 `test-go-costs-2` 跑的 `!race` 守卫文件与一个不连数据库的计划测试）。

修复一轮的处置照 `fix-1.md`：三份审查全部接受，一组一个提交、各自绿、接在 `c5a81c7a` 之后、不改写已推的历史；只改文档的（S2C-N5，设计 3.10、3.13–3.15、7.1、7.3、7.6，00 号文档第 232 条与第 9 节，M12 的移交）留给 Step 的文档提交。第二轮照 `fix-2.md`：一个提交，B 指出的 cp-notebook 的说法、C 指出的 costs-2 的作业时长与第一份负载敏感的测试只记进本记录。

## 接受与推后的

- **接受**：
  - 照预料存活的变异：A 的 N2（`StartUpgrade` 的 SQL 去掉 `mode = 'standard'`：00055 的检查照样拒）、P3（`Capture` 不判 `c.deleted_at`：活的节点没有已删的正文）、B8（`LockNotebook` 不排除已删的笔记本：之后的 `Authorize` 看不见，照样答 404），B 的 g9、g10（被蕴含，决定 241），C 的 d26b 的 `wiki/app` 一半（替身不在语句之中关连接，由组合根钉住），cp-notebook（可以造，见 S2B-N3；有主键与部分唯一索引挡着，省了），`TestTryHoldNeverWaits` 在 24a 下挂到二进制的期限（要动夹具，第 17 条交错已在 2 秒内抓到）。
  - 成本守卫对单独的第二遍退化不敏感：实施的 62a（整区一次传入的新写法）在 5 万页上六格中位数 2.09–3.18 秒、照过（最慢一次 3.57 秒），62b 照过，c3 在这一轮界重算之后照过（3.41–3.45 秒对 3.6 秒，B 的那一轮 3.91–3.96 秒被抓到，看机器）；它们各由 `TestTheAreaPassReadsByChunk`、`TestTheAreaWantedHeadsGoByBatch/refused` 与 `TestLintPlansOfTheAreaPass` 确定地抓到（第 235 条的守卫）。B、C 建议可以把 `LintAreaWantedEdgeHeads` 加进守卫的 `startCostPlanned`，不加：同一个计划已由 `costs-lint` 一行的计划的测试钉住。
  - 守卫减为 5 万页之后对超线性的写法更迟钝（B 的未核实的怀疑第 2 条，C 的第一条）：62a 一类 O(区内页数 × 批数) 的写法在 5 万页上只到界的约 90%，按它的阶推到 10 万页约四倍、会过期限（没有实测）。已知的超线性写法靠计划与分段的测试抓，写进设计 7.1 与 M12 的移交。
  - A 的未核实的怀疑：`COMMIT` 超过 `commitTimeout` 时答 500 而升级可能已经开了，00 号文档 4.4 已写明，网页读一次就看得出；10 万页在最慢的 runner 上约 5.6 秒是推算（本机 2.41 秒乘 lint 十万页守卫在 CI 上量到的最坏倍率 2.33），只影响移交的措辞。
  - A 的探针 `TestRevAS2StartAndDeletion` 不进仓库：删除订阅者的登记由 `TestUpgradeDeletion`（实施的第 33 条）钉住，B7 与它是同一处改坏；B8 是等价的变异。
  - 第一份负载敏感的交错计时测试：实施者第一次 `make test` 的 `TestARestoreJobAndThePurge/between_two_batches`（10 秒内等不到行锁的等待者）与 `TestARevertJobSkipsAMemberRestoredBetweenItsBatches`（测试只容许两种次序之外的第三种），修复一轮第一次 `make test` 的 `TestABackgroundJobAndItsNotebooksDeletion/revert`（后到的第二批先拿到行锁）。三个都是 `make test` 让六个竞态进程同时跑时的负载问题：S2 没有碰恢复、撤销与 River；这条路上 S2 的代码只有删除事务里新加的两个订阅者语句，它们在删除已持有笔记本行之后才跑，改变不了谁先拿到锁；单独跑、之后完整的 `make test` 与 CI 都通过。C 建议列为已知的负载敏感，不挡合并。
  - costs-2 的作业时长（修复核对 C）：`main` 的 run 42530568840 里 costs-2 在 EPYC 7763 上已是 22.2 分钟（那时还没有发起的守卫），加上守卫约 23.5 分钟，对 35 分钟的时限已不到 `ci.yml` 注释说的"约两倍"（约 1.5 倍）；S2 只多约 1.2 分钟，不是 S2 的问题。修复一轮的分支运行里 costs-2 在 8370C 上 20.6 分钟。
  - 流程（照实写下）：
    - 实施者推送之前改写过两次分支的历史（都还没推过）：逐个提交的核对发现第 5 个的 `TestLintMatrixTargets`（`-short` 下也以 `matrixViolations` 核对整个矩阵，只读 `matrixRows()`，改为两处共用 `coveredMatrixRows()`）与第 6、7 个的 errorlint（`%v` 包错误；6、7 号当时只跑了 `go vet` 就提交，实施者记为自己的疏漏）；第二次 `make test` 在 `costs-3` 发现 M10 的 `TestIngestMigrationCostsLongHistory` 在退到 53 的库上调 `checkPages`（S1 偏离 24 的同类：只在不带竞态的 `costs-3` 里跑）。改写之前的头 `59d0e6b2` 与中间的头 `b2c65a37` 与 `c5a81c7a` 只差 5 个文件（加 26 行、减 11 行），门禁的大部分、变异与七份的量在 `b2c65a37` 上跑，第 34 条在头上重跑。
    - 修复一轮的提交 3 推送之前重写过一次（上表"修复一轮的说明"）。
    - 修复第二轮时本机的根分区满了：几天的变异与竞态构建让 `~/.cache/go-build` 涨到 59 G，PostgreSQL 写不进 WAL（"No space left on device"），`costs-2` 第一次失败；实施者经 `heavy.sh` 删掉 12 小时没用过的缓存条目（缓存降到 18 G，根分区剩 44 G），没有动 Docker，重跑通过。`06-environment.md` 第 3 节在本提交加一句：长时间的变异与竞态构建会让 Go 的构建缓存涨到几十 G，跑重的之前看一眼 `df`，满了先修剪自己的缓存（可以重建），不动 Docker。
- **推后**：
  - M12 的性能：10 万页的发起（守卫的 6 秒请求下放不下，生产的 15 秒请求下放得下；默认 4 秒的请求下发起只有 3 秒，慢的机器上 10 万页每次都答 503）；守卫减半之后超线性的写法靠计划与分段的测试抓。写在 00 号文档第 232 条与"写给后面的 M"，M11 收尾时追加到 M12 的性能移交。
  - 合并之后看：第一次落在 EPYC 7763 上的 costs-2 里新守卫的数（C 照 lint 守卫的比推算约 2.3–2.5 秒，对 3.6 秒的界约 1.45 倍）与 costs-2 的作业时长；建成时分支运行的第二、四份落在 7763 上，测试一步都是 431 秒，修复第二轮的运行里第一份 444 秒，合并之后两次 `main` 的平均是否仍在 450 秒之下（设计第 8 节第 1 条）。
  - 没有交给 v0.1 之后的打磨的。

## 反向对照

建成时以 `mut.py` 跑（`NW_SERVER` 指实施者的工作树，库的经 `heavy.sh`，每个之后由工具从备份复原，改坏时不跑 `make gen*`，改生成的 SQL 时改生成的常量），在 `b2c65a37` 上，先在没改的树上跑同样的 16 条命令与第 62 条的两条（`m11/s2/mut/run-all.out` 的 baseline 段）；第 62 条要同时改两到三个文件，另写 `m11/s2/mut/m62.py`，同样先备份、跑完复原。设计列两个测试的各跑一次。修复一轮在 `42529055` 上整批经 `heavy.sh` 重跑（`s2-review/fix1-mut/`）。审查者的在 `s2-review/` 下（A 的 `a/`、B 的 `b/`、C 的 `c/`；修复核对一轮 A 的 `a/check1/`、B 的 `b/check1/`、C 的 `c1/`）。这些都在 `scratchpad/m11/` 下。

**7.3 的 S2 各行**（建成时 20 条、25 种改坏：第 18 条的 S2 一半，第 22–36、62、63、67、82 条，第 82 条两个方向各一次，第 62 条四种写法；每一种都有点名的测试抓到；修复一轮在 `42529055` 上重跑，结果相同）：

| # | 变异（实际的改法） | 跑的测试 | 结果 |
|---|---|---|---|
| 18 | 导入建任务的事务不判冻结（`start_import.go` 的 `if false && …`） | `TestUpgradeInterleavings/4_…`（7.2 第 4 条） | 抓到 |
| 22 | 发起不先删过期的会话 | `TestUpgradeInterleavings/3_…` | 抓到 |
| 23 | 基线在补写与保护之后记 | 组合根 `TestUpgradeStart/it_writes_what_the_notebook_lacks` | 抓到（保护行与基线行的主键冲突；C 的 23b 在"已有 raw、只补 AGENTS"时没有冲突，由基线的节点数抓到） |
| 24 | `TryHold` 等名额（`default` 换成阻塞的发送） | `TestTryHoldNeverWaits`；`TestUpgradeInterleavings/17_…` | 两个都抓到；建成时都等到测试二进制的期限（60、180 秒）才失败（S2C-N2），修复一轮之后第 17 条 2.06 秒 |
| 25 | 持有者不计入 `active` | `TestStopWaitsForTheHolders` | 抓到 |
| 26 | 发起的事务里另套一层期限（`WithTimeout(ctx, Timeout*9/10)`，先于 `ctxStart` 到期） | 组合根 `TestUpgradeStart/its_own_deadline` | 抓到（不是 503）；修复一轮之后 `inside a statement` 与 `between statements` 两格都抓到。同长的那种（C 的 d26b）建成时照过，见 S2C-I1 |
| 27 | `WithinTx` 交回 nil 时也看 `ctxStart` | `wiki/app` 的 `TestUpgradeStart/a-commit-past-its-deadline` | 抓到 |
| 28 | `ANALYZE` 用 `ctxStart` | 同上 | 抓到 |
| 29 | 第二遍不跑（`Prepare` 里 `if false && …`） | `TestTheWantedCountsByPage`；组合根 `TestUpgradeStart/a_raw_page_is_the_source_root` | 两个都抓到 |
| 30 | 第一遍换页时不清 `seen` | `TestTheWantedCountsByPage` | 抓到 |
| 31 | `computeIn` 自己读时钟 | `TestComputeInTakesItsMoment` | 抓到（设计写的 `TestUpgradeStart` 抓不到：S2 的发起不经 `computeIn`，偏离 8） |
| 32 | 发起不 `Publish` | `TestUpgradeStatesReachTheStreams` | 抓到 |
| 33 | 组合根不登记 wiki 的删除订阅者 | `TestUpgradeDeletion` | 抓到 |
| 34 | `checkPages` 的"有升级列就有 wiki 行"恒为 0（`WHERE false AND …`） | `TestPageViolationsCatchEachUpgradeGap` | 抓到（`b2c65a37` 与头各一次） |
| 35 | `stale_pages > 0` 不拒 | 组合根 `TestUpgradeStart/the_codes_in_their_order` | 抓到 |
| 36 | 根下的附件不拒（去掉 `Kind != page` 的判断） | 同上（附件名为 `AGENTS` 的那一步） | 抓到 |
| 62a | 第二遍整区一次传入：应用不分段、不按游标收窄，适配器不限 512 个 id | 成本守卫；`TestTheAreaWantedHeadsGoByBatch/refused` | 守卫没抓到：第二遍从 0.17–0.22 秒到 2.16–2.48 秒（293 次读），六个中位数 3.18、3.03、2.78、2.13、2.13、2.19 秒，在 3.5 秒的界之内；`refused` 抓到 |
| 62b | 语句改为与 `unnest` 的 id 连接、连接之后排序（生成的常量），应用照旧分段 | `TestLintPlansOfTheAreaPass`；成本守卫 | 计划的测试抓到（没有统计时出现 Sort）；守卫没抓到（每段 512 页，排序很小） |
| 62c | 62a 与 62b 同时（审查者 B 在设计审查里量的形状） | 成本守卫；`TestLintPlansOfTheAreaPass` | 两个都抓到：自定义计划三格中位数 1.90、1.87、1.90 秒，通用计划第一次发起就过了 4.5 秒的期限、答 503 |
| 62d | 只有 62a 的应用一半 | `TestTheAreaPassReadsByChunk` | 抓到（替身拒收 512 个以上的 id） |
| 63 | 发起不调 `Protect` | 组合根 `TestUpgradeStart/it_writes_what_the_notebook_lacks` | 抓到 |
| 67 | 发起准备整份摘要（`RuleQuery` 去掉 `Rule: &wanted`） | `wiki/app` 的 `TestUpgradeStart` | 抓到 |
| 82a | `notebook_upgrades_submission_check` 第一个合取改成单向 | `TestChecksRejectCounterexamples` | 抓到（"a submission's id alone"） |
| 82b | 第二个合取改成单向 | 同上 | 抓到（"a submission's submitter alone"） |

第 62 条设计写"发起的成本守卫；`LintAreaWantedEdgeHeads` 的计划的测试"：在 PostgreSQL 18 上，`= ANY` 的主键扫描带着行比较的游标，整区一次传入只让第二遍慢约 10 倍，整个发起仍在界内；守卫只抓得到三者合在一起的 62c，拆开的各由分段、适配器与计划的测试抓到（偏离 19，7.3 第 62 条改为列这三个测试）。修复一轮重跑时 62a 的守卫中位数 2.09–2.89 秒（第二遍 2.17 秒），62c 的通用计划第一次发起到 4.5 秒的期限、自定义三格 1.91–1.92 秒。

**修复一轮的变异**（`s2-review/fix1-mut/`，在 `42529055` 上；连同审查者的写成 [01-P1](../01-P1-freeze-start-rollback.md) 7.3 的第 95–109 条，第 24、26、31、62 条照实改）：

| 来源 | 变异 | 跑的测试 | 结果 |
|---|---|---|---|
| A 的 W1（96） | 503 的条件去掉 `ctxStart.Err() != nil` | `wiki/app` `TestUpgradeStart` | 抓到（之前存活）：`a-commit-out-of-its-own-time` |
| A 的 W3、C 的 o3（97） | `TryHold` 之后立刻放开；`fn` 里 `defer` 放开 | 同上 | 抓到（之前存活）：`it-writes-what-the-notebook-lacks` 的名额探针（`capture`、`commit` 时名额空着） |
| A 的 W6（97） | `WithinTx` 之后不放开，只靠 `defer` | 同上 | 抓到（之前存活）：`analyze` 时名额仍被持着 |
| A 的 W4、C 的 o2b（98） | `TryHold` 挪到 `Writing` 之前；挪到 `refuseSessions` 之前 | 同上 | 抓到（之前存活）：`a-writing-job-a-full-slot`、`edit-sessions-a-full-slot`、`an-attachment-raw-a-full-slot` 等答 503 |
| C 的 o24（95） | 503 的判断去掉 `errors.Is(err, DeadlineExceeded)` | 同上 | 抓到（之前存活）：`a-refusal-rolled-back-past-its-deadline` |
| C 的 d26b（26） | `WithinTx` 开在请求的 ctx 上，`fn` 里另套同长的期限 | 组合根 `its own deadline` | 抓到（之前存活）：`inside a statement` 答 500；`wiki/app` 一半照过，照预料 |
| C 的 24a、24b（24） | `TryHold` 持着 `r.mu` 阻塞；放开 `r.mu` 再阻塞 | 第七份的第 17 条交错 | 抓到：2.08、2.09 秒（之前挂到第七份的期限） |
| A 的 L5、L8、L10（99） | 第二遍不判资格；不去重；不取父页的键 | `TestTheWantedCountsByPage` | 抓到（之前存活）：`an-llm-contract` 等 |
| A 的 L4、L7（100） | 第二遍不核对 `s.source`；不跳过已解析的链接 | 同上 | 抓到（之前存活）：`a-page-outside-the-area-answered`；`a-resolved-link-answered` |
| A 的 WA2（101） | `CurrentUpgrade` 去掉 `rolling_back` | `TestTheUpgradesRows` | 抓到（之前存活） |
| n1（102） | `prepareIn` 返回之前调一次 `project` | `TestComputeInTakesItsMoment` | 抓到 |
| B 的 c2（103） | 生成的 `CaptureBaseline` 丢 `notebook_id` | 成本守卫 | 抓到（之前存活）：计划（读整个部分索引） |
| f240a、f240b（104） | `CaptureBaseline` 改回普通的连接；`LATERAL` 不带 `LIMIT 1` | 同上 | 抓到："page_contents read by a Seq Scan"（在提交之前跑） |
| B 的 c1（105） | 生成的 `SetBaselineWanted` 丢 `upgrade_id` | 同上 | 抓到：计划（通用计划、从未分析） |
| B 的 c3（106） | 生成的 `LintAreaWantedEdgeHeads` 丢 `source_id` | `TestLintPlansOfTheAreaPass`；成本守卫 | 计划的测试抓到；守卫照过（通用三格 3.41–3.45 秒，界 3.6 秒） |
| B 的 g1–g5（107） | 00057 五个合取去下界或改成单向 | 迁移包、wiki 适配器 | 抓到（之前存活）：各被对应的新反例抓到 |
| B 的 g6、g7（108） | 谓词去掉 `rolling_back`；去掉 `deleted_at IS NULL` | 同上 | 抓到（之前存活） |
| B 的 cp-all；cp-state、cp-pages、cp-kind（109） | `checkPages` 去掉四个合取；去掉其中一个 | `TestPageViolationsCatchEachUpgradeGap` | 抓到（之前存活） |
| cp-notebook | `checkPages` 去掉"同笔记本" | 同上 | 存活，照预料（S2B-N3） |
| A 的 N2、P3、B8；B 的 g9、g10 | 等价或被蕴含的 | 各自的测试（B8 换成组合根的 `TestUpgradeStart`、`TestUpgradeDeletion`、`TestUpgradeInterleavings`：A 的探针没有入树） | 存活，照预料 |

实施的 25 种在这一轮全部重跑（`final-mine.out`、`final-m62.out`），结果与建成时相同；第 24 条的单元一半仍挂到 60 秒（不动夹具），第 26 条两格都抓到。

**审查者的变异**（照各自的命名；"存活"的都成了一条发现，等价与被蕴含的除外）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | W1 | 存活：S2A-N1 |
| A | W3、W4、W6；B5、B6（组合根的 W3、W4） | 存活：S2A-M1 |
| A | L5、L8、L10；B4（组合根的 L5） | 存活：S2A-M2 |
| A | L4、L7 | 存活：S2A-N2 |
| A | WA2 | 存活：S2A-N3 |
| A | N2、P3、B8 | 存活：等价的变异 |
| A | W2、W5、W7–W10，L1–L3、L6、L9，N1、N3，P1、P2，WA1；B1、B2、B3、B7 | 抓到（B2、B7 另被探针抓到） |
| B | g1–g5 | 存活：S2B-M1；加建议的五个反例之后各被杀 |
| B | g6、g7 | 存活：S2B-N1 |
| B | g8 | 被杀，只因报出的名字换了（S2B-N2） |
| B | g9、g10 | 存活：被蕴含（S2B-N2） |
| B | k1、k2（对照） | 被杀 |
| B | c1 | 被守卫的计划杀（"分析过、通用计划"一格；"从未分析"一格两种计划都顺序扫描 `upgrade_nodes`、照过，表小的缘故） |
| B | c2 | 存活：S2B-M2 |
| B | c3 | 计划的测试杀；守卫只在通用计划三格以 3.58–3.82 秒刚过 3.5 秒的界 |
| B | r2（`MAINTAIN` 去掉 `upgrade_nodes`）、mx2（组合根的 `UpgradeAction` 换成 `notebook.read`） | 被杀 |
| B | cp-all | 存活：S2B-N3 |
| C | 22、23、23b、24a、24b、25、29、30、31、32、35、62b、62d、63（7.3 的行重跑）；36b（只放过名为 raw 的附件：组合根照过、`wiki/app` 的 `an-attachment-raw` 抓到） | 抓到 |
| C | d26b | 存活：S2C-I1（探针下答 500） |
| C | o24 | 存活：S2C-M1（探针下答 503） |
| C | o2b、o3 | 存活：S2C-N3（并入 S2A-M1）；o2（挪到锁工作区之前）只被组合根第 14 条抓到 |
| C | o4、o6、o7、o8、o9、o13、o14、o20、o21、o22、o23、62e | 抓到（62e：待建的过滤挪出 `LATERAL`，被 `TestLintPlansOfTheAreaPass` 抓到） |

A 36 个（20 个抓到，13 个存活成了发现，3 个等价），B 18 个与读实施者 62a 的日志，C 重跑 7.3 的 13 行（16 种写法）、自己想的 16 种。

**修复核对一轮**（在 `42529055` 上）：

| 审查者 | 变异 | 结果 |
|---|---|---|
| A | W1、W3、W4、W6、L4、L5、L7、L8、L10、WA2（存活过的） | 都抓到 |
| B | g1–g7（存活过的）；g8；k1、k2 | 都被杀（g1–g5 各由对应的新反例先抓到） |
| B | g9、g10 | 存活：被蕴含（决定 241） |
| B | c1、c2（存活过的）；c3 | c1、c2 被守卫的计划杀；c3 被计划的测试杀，守卫这一轮以时长杀（通用 3.91–3.96 秒对 3.6 秒），实施者那一轮照过 |
| B | rk、ds、ls（新：`RootNodesByKey`、两条会话的语句丢 `notebook_id`） | 都在第一格被计划杀（rk 是 PG18 的跳跃扫描，条件只有 `name_key`；ds、ls 是 Seq Scan） |
| B | cp-all（存活过的） | 被杀（三行各答 `[]`） |
| C | d26b | 组合根被 `inside_a_statement` 抓到；`wiki/app` 照过，照预料 |
| C | o24、o2b、o3 | 都抓到 |
| C | 24a、24b | 第七份的第 17 条 2.08、2.09 秒；单元一半 24a 挂到 60 秒（照处置不动）、24b 4 秒 |
| C | n1、d31 | 都抓到 |
| C | x1（新：`CaptureBaseline` 改为与按 `node_id` 排序的子查询连接） | 抓到（"page_contents read by a Seq Scan"） |

A 10 个都抓到；B 19 个，17 个被杀、g9、g10 被蕴含；C 12 次，d26b 的 `wiki/app` 一半照预料照过，24a 的单元一半照处置挂到期限。A 另以合成的计划核对守卫的 `startCostPageScans`，得 S2A-N4。

**修复第二轮**（在 `9b3000c1` 上，`s2-review/fix2-mut/`，整批经 `heavy.sh`；写成 7.3 第 110 条）：

| 来源 | 变异 | 跑的测试 | 结果 |
|---|---|---|---|
| r1（110） | `startCostPageScans` 去掉第 1 项（`page_contents` 的 `Index Cond` 含 `node_id`） | `TestTheStartGuardJudgesThePageReads`（`-short`） | 抓到：`the_whole_key_for_each_node` |
| r2（110） | 去掉第 2 项（`repeated`：在嵌套循环的内侧逐个节点读） | 同上 | 抓到：`a_key_read_once` |
| B 的 c2（103） | 生成的 `CaptureBaseline` 丢 `notebook_id` | 成本守卫 | 抓到：计划（"nodes read by a Bitmap Heap Scan of nodes_notebook_id_name_key_idx on \"\""） |
| f240a、f240b（104） | `CaptureBaseline` 改回普通的连接；`LATERAL` 不带 `LIMIT 1` | 同上 | 抓到："page_contents read by a Seq Scan" |

**修复核对第二轮**（A，在 `9b3000c1` 上；`a/check2/`）：先在没改的树上跑同一条命令，五格都通过；r1、r2 各被恰好违反那一项的一格抓到。合成的计划（把头上的核对原样抄进独立包）：check1 的 Merge Join、`page_contents_pkey` 没有条件，在 `42529055` 上放过、在 `9b3000c1` 上被拒；Hash Join 之下的 Seq Scan 两处都被拒；LATERAL 的形状（Nested Loop 内侧、Limit 下的 Index Scan、`node_id = n.id`）与 Nested Loop 内侧经主键的位图都放过；`page_contents` 在 Nested Loop 外侧整读、内侧有条件但隔着一层 Materialize 都被拒（后者真实的计划里不会有，只为确认 `repeated` 遇到 Materialize 会复位）。

## 核对

| 轮 | 对象 | 发现 | 处置 |
|---|---|---|---|
| 1 | `42529055`（修复一轮 `219d6f28`–`42529055`，4 个提交），三位各在自己的工作树里 | A：自己的 S2A-M1、M2、N1–N3 都关了，留下的 10 个变异在头上都被抓到；直接跑 `TestTheBaselineIsTheNotebookAsItStarted` 核对 `CaptureBaseline` 改写之后的语义；第 7 节的七条说明都成立；新的一条 Nit（S2A-N4）；另提设计 7.1 "准备不超过 4 秒"的口径。B：S2B-M1、N1、N3 关了，M2、N2 代码侧关、文档待改；g1–g7、c2、cp-all 都被杀，新的 rk、ds、ls 被杀；`CaptureBaseline` 在两种库、两种统计、两种计划下都逐个经主键读；界 3.6 秒的口径对；实施者"cp-notebook 没有一行能单独违反"的说法不对；没有新的发现，列一条未核实的怀疑（同 S2A-N4 那一处）。C：S2C-I1、M1 代码侧关、设计 7.3 待改，N1–N4 关，N5 照处置记下（`0da54746` 同样只写 `server`）；4 个提交各自 build、vet、`make lint-go`、`make gen-check`、short（44–46 秒）都通过，竞态下各自碰到的包与组合根的测试都通过，`0da54746` 上的守卫中位数 1.26–1.31 秒；12 种变异的结果见上；CI 运行 42571990070 的数与说法相符；第一份的 `TestABackgroundJobAndItsNotebooksDeletion/revert` 与 S2 无关；没有新的发现，列一条未核实的怀疑（同 S2A-N4 那一处）。三位：Important 0、Minor 0、Nit 1 | 修复第二轮（`fix-2.md`，S2A-N4，决定 242） |
| 2 | `9b3000c1`（修复第二轮 1 个提交，只改 `bootstrap/upgrade_start_cost_test.go`），A 在自己的工作树里 | A：S2A-N4 关了。两项新要求合起来正是决定 240 的"逐个经主键读"，`nodes` 与 `edit_sessions` 的核对没有改；`TestTheStartGuardJudgesThePageReads` 的五格里第三、四格各只违反一项，去掉任何一项都有一格失败，第一格与实施报告记的真实计划一致；它跑在 short 一行，不进竞态的七份；r1、r2 都被抓到；合成的计划的结果见上。守卫本身照说明不重跑：分支的 CI 运行 42579807531 全绿，costs-2 在 9V74 上中位数 1.69–1.74 秒，界 3.6 秒。Important 0、Minor 0、Nit 0 | 合并（修复核对两轮就结束） |

## 实测

- **新的测试在竞态检测下**（建成时，`go test -race -count=1 -v`，逐包，经 `heavy.sh`，共 59 秒）：第七份的 S2 的六个共 9.5 秒（`TestUpgradeStart` 2.27、`TestUpgradeStatesReachTheStreams` 2.13、`TestUpgradeInterleavings` 1.91、`TestUpgradePermissionMatrix` 1.48、`TestUpgradeStates` 0.94、`TestUpgradeDeletion` 0.78），组合根的包 21.4 秒；第四份的 `TestPageViolationsCatchEachUpgradeGap` 2.34 秒；改过的 `TestTheRuntimeRoleServesWithTheGrantsFile` 4.73、`TestThePermissionMatrixCoversEveryOperation` 0.61 秒。`migrations`：`TestChecksRejectCounterexamples` 5.69、`TestConstraintAndIndexNames` 1.35 秒。`wiki/adapter/http`：`TestTheUpgradesCodesAreAnswered` 7.87 秒（13 个码各一格）、`TestTheUpgradeIsAnswered` 0.62 秒。其余（notebook 的两个，page 的适配器三个与 `TestASeedForAnUpgradeIsItsOwn`，linking 的适配器与应用三个，wiki 的适配器一个与应用六个）各在 1.7 秒之内，含起容器。修复一轮新加的 `inside a statement` 约 0.3 秒，第 17 条交错挪进 goroutine 之后用时不变。
- **发起的成本守卫**（`TestUpgradeStartTakesItsTimeAtAnyStatistics`，`bootstrap/upgrade_start_cost_test.go`，`//go:build !race`，`-short` 跳过，`test-go-costs-2` 的末一行；真的应用，请求 6 秒，照 lint 的成本守卫的配置；一个查询追踪器按 sqlc 的名字记各段；每种统计状态 3 次发起取中位数；六格的次序是自定义计划的从未分析、分析过这本笔记本的上一次发起、只有另一次进行中的升级的 10 万行的统计，再是通用计划的同样三种）：
  - 10 万页（设计的规模；3 万页在 raw 里、各 10 条待建，区外 6,999 页各 1 条，1,000 个目标；那时的期限是 lint 的 3 秒）：种子 3.26、3.08 秒；两轮六个中位数 2.35、2.33、2.36、2.38、2.33、2.41 秒与 2.23、2.24、2.22、2.21、2.16、2.23 秒。各段：准备 1.47–1.68 秒（wanted 一条规则 1.34–1.52 秒，其中第二遍 0.34–0.40 秒、644 次读）、`Capture` 0.24–0.30 秒、补写 0.21–0.26 秒、`SetWanted` 0.16–0.21 秒、提交之后的 `ANALYZE` 32–57 ms。离 3 秒的期限只余约 20%；lint 十万页守卫的摘要在 `main` 的 12 次 CI 里是 0.95–1.91 秒、本机 0.82 秒，CI 的 runner 是本机的 1.16–2.33 倍，折到发起 2.5–5.6 秒。所以照第 232 条改用 `StartTimeout = request_timeout × 3/4`（守卫 4.5 秒）；4.5 秒仍放不下最慢的 runner 上的 10 万页（约 5.6 秒，推算），守卫的规模减半，10 万页进 M12 的性能移交。
  - 5 万页（建成时；1.5 万页在 raw 里、各 10 条，区外 3,499 页各 1 条，1,000 个目标，库里只有这一本）：种子 1.66、1.67 秒；两轮六个中位数 1.14、1.12、1.15、1.13、1.14、1.11 秒与 1.11、1.06、1.07、1.04、1.04、1.05 秒；各段：准备 0.69–0.80 秒（第二遍 0.17–0.22 秒、322 次读）、`Capture` 0.11–0.14 秒、补写 0.10–0.13 秒、`SetWanted` 78–95 ms、`ANALYZE` 28–53 ms。界取较大的中位数 1.15 秒的三倍、向上取整到 100 ms，即 3.5 秒。`make test` 里两次是中位数 1.02–1.05 秒与 1.19–1.25 秒（负载约 8）；审查 B 一轮 1.16–1.19 秒；审查 C 在第 6 个提交上 1.12–1.15 秒。CI 的 costs-2（EPYC 9V74，run 42554470668）：守卫 38.6 秒，六个中位数 1.57–1.62 秒，准备 1.02–1.04 秒，第二遍 0.24–0.27 秒。
  - 5 万页、另九本笔记本（修复一轮之后，决定 240）：本机两轮六个中位数 1.153、1.153、1.167、1.146、1.127、1.162 秒与 1.123、1.135、1.155、1.148、1.112、1.152 秒；种子 10.23、10.19 秒（这本笔记本的 1.46、1.43 秒），守卫全程 35.9、35.7 秒。界取较大的 1.167 秒乘 3 是 3.501 秒，向上取整到 100 ms，即 3.6 秒，在 4.5 秒的期限之内。各段：准备 0.69–0.76 秒（第二遍 0.17–0.19 秒）、`Capture` 0.16–0.20 秒（原来 0.11–0.14 秒：现在在 50 万行的表里按笔记本的键与主键逐个读）、补写 0.12–0.15 秒、`SetWanted` 76–92 ms、`ANALYZE` 28–50 ms、EXPLAIN 至多 6.4 ms（从时间里减去）。`make test` 里种子 12.0 秒、中位数 1.31–1.50 秒（与别的成本份同时跑）。修复核对 B 一轮 1.32–1.37 秒（`Capture` 194–248 ms，种子 11.6 秒，负载 1.5–2.5）；修复核对 C 一轮 1.24–1.41 秒（种子 12.0 秒）。CI 的 costs-2（Intel Xeon 8370C，run 42571990070）：种子 22.1 秒（这本笔记本的 3.1 秒），六个中位数 2.13、2.16、2.22、2.14、2.13、2.16 秒（单次最长 2.27 秒），约本机的 1.9 倍，对界 3.6 秒至少 1.62 倍、对 4.5 秒的期限 1.98 倍，守卫全程 70.1 秒。
  - 修复第二轮之后（`9b3000c1`，计划另核对 `page_contents` 的扫描含 `node_id`、在嵌套循环内侧逐个读）：本机 `make test-go-costs-2`（第二次，608 秒，11 个包都通过）里种子 11.9 秒（这本笔记本的 1.6 秒），六个中位数 1.34、1.42、1.41、1.42、1.38、1.40 秒，界 3.6 秒，守卫全程 43.1 秒，`CaptureBaseline` 照旧是内侧经主键的 Nested Loop，新的两项要求在六格都过；比修复一轮两轮的 1.11–1.17 秒慢，原因没有查。CI 的 costs-2（EPYC 9V74，run 42579807531）：种子 16.6 秒（这本笔记本的 2.4 秒），六个中位数 1.69、1.71、1.72、1.74、1.74、1.72 秒，守卫全程 55.7 秒；同型号的机器上建成时（库里只有这一本，界 3.5 秒）是 1.57–1.62 秒。
  - 计划：建成时 `SetBaselineWanted` 从未分析时自定义是 Hash Join（内侧经 `upgrade_nodes_pkey` 的位图），通用是 Nested Loop + Memoize + 经主键的扫描，分析过之后两种都经主键；没有顺序扫描。修复一轮之后 `CaptureBaseline` 六格都是 Nested Loop，外侧经 `nodes_notebook_id_name_key_idx`（从未分析）或 `nodes_notebook_id_parent_id_idx`（分析过）的位图，内侧是 Limit 下经 `page_contents_pkey` 的 Index Scan；`RootNodesByKey` 经 `nodes_notebook_id_name_key_root_key` 或 `nodes_notebook_id_parent_id_idx` 的位图；两条会话的语句经 `edit_sessions_notebook_id_idx`。
  - 与审查者 B 在设计审查里量的对照（10 万页里 3 万页在区里、各 10 条、30 万条边，PostgreSQL 18.6）：整条第二遍 0.34–0.40 秒（644 次读，每次至多 512 行），六种状态与模式都一样；B 量的整组一次传入：强制自定义计划 18.1 秒、通用计划约 150 秒、自动选择 66.2 秒，一段 512 页一次读 0.17 秒。`SetBaselineWanted`（约 3.7 万个 id）10 万页时 162–211 毫秒、5 万页时 78–95 毫秒；B 量的自定义与通用：从未分析 432、671 ms，分析过已结束的另一次 386、444 ms，分析过进行中的另一次 400、445 ms。`TestLintPlansOfTheAreaPass` 在 28 格里都只经 `page_links_pkey` 读一次、没有 Sort、每批读的行与取的块不过 1,024，每批 0.25–1.13 ms（审查 B）；CI 的 costs-lint（8370C）2.12 秒。
- **e2e**（不带竞态）：建成时 `make e2e` 507 个（Playwright 2.2–2.4 分钟，133、148 秒；S1 时 503 个，多的是 UP1 的两个、UP2、UP4）；审查 C 在头上 507 个（2.3 分钟，142 秒）；修复一轮 507 个，130 秒。

## 组合根的各份

照 3.8 以竞态构建列出（`go test -race -list`，各份照 `Makefile` 的 `-run`、`-skip`）：541 个顶层测试，七份 158、51、121、124、62、11、14，每个恰在一份；比 S1 之后的 534 个多 7 个：第七份的六个（`TestUpgradeStart`、`TestUpgradeStates`、`TestUpgradeStatesReachTheStreams`、`TestUpgradeDeletion`、`TestUpgradeInterleavings`、`TestUpgradePermissionMatrix`），第四份的 `TestPageViolationsCatchEachUpgradeGap`（名字里有 Upgrade 但不以 `TestUpgrade` 起头，落在补集）。第六份少的一个就是第七份的 `TestIngestBefore`，是分份调整（`2beef88c`）挪的。成本守卫 `TestUpgradeStartTakesItsTimeAtAnyStatistics` 带 `//go:build !race`，不进竞态构建、不进任何一份，只在 `test-go-costs-2` 的末一行跑。`Makefile` 的分份注释在第 5 个提交改写这些数，审查 C 在头上数的一样。修复一轮不加顶层测试（`inside a statement` 是子测试）。修复第二轮的 `TestTheStartGuardJudgesThePageReads` 与守卫在同一个 `!race` 文件里，不进竞态构建、不进任何一份，跑在 short 一行（`-short` 下不跳过）；costs-2 那一行只以 `-run '^TestUpgradeStartTakesItsTimeAtAnyStatistics$'` 跑守卫。

本机各份单独、竞态下，经 `heavy.sh`（建成时在 `b2c65a37` 上，七份的测试与头相同；`make -j1 test-go-bootstrap-N` 依次两轮，包的时间，秒）：

| 份 | 测试数（竞态构建） | 第一次 | 第二次 | 平均 | S2 之前的两轮 |
|---|---|---|---|---|---|
| 第一 | 158 | 288.3 | 286.4 | 287.4 | 298.8、290.8 |
| 第二 | 51 | 262.7 | 243.2 | 253.0 | 279.5、283.2 |
| 第三 | 121 | 267.3 | 249.8 | 258.6 | 266.2、278.9 |
| 第四 | 124 | 223.9 | 208.5 | 216.2 | 220.2、227.4 |
| 第五 | 62 | 263.0 | 245.8 | 254.4 | 259.1、259.9 |
| 第六 | 11 | 184.5 | 187.0 | 185.8 | 212.6、222.4 |
| 第七 | 14 | 95.6 | 89.6 | 92.6 | 82.1、84.1 |

- 14 次都通过。"S2 之前的两轮"第一至五份是 S1 的数，第六、七份是分份调整之后的数（`shards-report.md`）。第七份多了约 10 秒；照 7763 与本机之比 1.77（`Makefile` 的注释），折到 7763 约 164 秒，远在 450 秒之下（7.5 估 P1 末 274–374 秒）。
- 修复一轮之后第七份本机两轮 90.3、93.4 秒（墙钟 91.1、94.2 秒）。
- CI 上七份的测试一步：建成（run 42554470668）357、431、382、431、395、245、149 秒（第一份 Xeon 6973P-C，第二、四份 EPYC 7763，第三份 8370C，第五、七份 9V74，第六份 6973P-C）；修复一轮（run 42571990070）288、350、342、410、393、298、151 秒（第一份 9V45，第二、三、五、七份 9V74，第四份 8573C，第六份 8370C）；修复第二轮（run 42579807531）444、429、262、422、393、304、119 秒；都在 450 秒（7:30）之下，第二轮的第一份 444 秒离线只有 6 秒（第二轮没有碰任何一份的测试）。最紧的是建成时落在 7763 上的第二、四份（431 秒）：S2 只给第四份加了 `TestPageViolationsCatchEachUpgradeGap`（本机 2.3 秒，折到 7763 约 4 秒）。分份调整合并之后 `main` 的第一次运行 42530568840（`5795229c`）七份是 288、399、348、284、392、228、137 秒（审查 C 读的）；设计第 8 节第 1 条要的是 S2 合并之后两次 `main` 的平均：合并那次（run 42583342275，`9c7c6821`）七份是 363、436、351、361、393、320、124 秒（第六份第一次 284 秒时因 M10 的 `TestIngestCoverage` 的数据竞争失败，重跑 320 秒，见"合并之后"），修两个 M10 测试的合并那次（run 42593177104，`58abac7a`）是 442、344、384、388、269、281、142 秒；两次平均 402.5、390、367.5、374.5、331、300.5、133 秒，都在 450 之下。分份调整之后的两次（42530568840 与 42583342275）平均 325.5、417.5、349.5、322.5、392.5、274、130.5 秒，第六份挪份之后的核对到此完成。没有挪份，没有提高 `-timeout`。

## 门禁与 CI

本机（PostgreSQL 18 在 Docker 里，要 Docker 的与计时的都经 `heavy.sh`，机器同时有别的会话在跑测试）：

- **逐个提交**（建成时，实施者的 `m11/tools/percommit-s2.sh`，一个干净的临时工作树、`pnpm install --frozen-lockfile`，经 `heavy.sh`）：7 个提交各自 `go build ./...`、`go vet ./...`、`make lint-go`、`make gen-check`、`make test-go-short`；碰 `api/`、`web/` 或 `e2e/` 的第 5、7 个另跑 `make lint-web`、`make knip`、`make test-web`。第一次：1–4 号每一项都通过；5 号 short 失败（`TestLintMatrixTargets`）；6、7 号 `make lint-go` 失败（errorlint）、short 失败（同一个测试）。改写之后对 5–7 号重跑两次（第二次是 `0778f5e2`、`1c4d35c0`、`c5a81c7a`），每一项都通过，vitest 189 个文件、4,095 个测试；每一步之后工作树干净。审查 C 另在临时的工作树里对 7 个提交跑同样的五步（short 每个 43–45 秒）与第 5、7 个的网页三步，第 1–6 个在竞态下跑各自碰到的包与组合根的测试，第 4 个另跑 `TestLintPlansOfTheAreaPass`、第 5 个另跑 `TestIngestMigrationCostsLongHistory`（12.96 秒，库退到 53，`checkPages` 照 `to_regclass` 不加升级的规则）、第 6 个另跑守卫（中位数 1.12–1.15 秒），都通过。
- **建成时的头**（表里注明头：`b2c65a37` 与 `c5a81c7a` 只差第二次改写的 5 个文件）：`make lint gen-check knip build`（`b2c65a37`）9 秒通过；`make test` 第一次（`b2c65a37`，571 秒）在第一份的两个交错的计时测试失败（"接受与推后的"），make 随之停下；`make test-go-short` 41 秒；`make e2e` 507 个 133 秒；`make test` 第二次（单独跑，`b2c65a37`，1,467 秒）竞态、组合根七份、`costs-1`、`costs-2`（新的守卫中位数 1.02–1.05 秒）通过，`costs-3` 的 `TestIngestMigrationCostsLongHistory` 失败（`checkPages` 的升级规则读不到 53 的库里的列，改为库里有这张表时才加，并回第 5 个提交）。头 `c5a81c7a` 上：`make lint gen-check knip build` 13 秒；`make test` 第三次 3,473 秒通过（负载约 8；竞态、七份、成本三份、lint 的成本、守卫两份、vitest 189 个文件 4,095 个测试）；`make test-go-short` 47 秒；`make e2e` 507 个 148 秒。
- **修复一轮**（`42529055`）：4 个提交各自在干净的工作树里（`m11/tools/percommit-fix1.sh`、`percommit-fix1b.sh`，经 `heavy.sh`）build、vet、`make lint-go`、`make gen-check` 与 short 都通过，每个之后工作树干净（提交 3 重写之前的 `02f77f7e` 在 `make lint-go` 失败，上表"修复一轮的说明"）。头上 `make lint gen-check knip build` 通过（9 秒）；经 `heavy.sh`：`make test` 第一次 556 秒在第一份的 `TestABackgroundJobAndItsNotebooksDeletion/revert` 失败（单独以 `-race -count=3` 三次通过），第二次完整的 3,324 秒通过；`make test-go-short` 40 秒；`make e2e` 507 个 130 秒。审查 C 在修复核对里另跑了 4 个提交各自的门禁与竞态下的包，都通过。
- **修复第二轮**（`9b3000c1`，决定 242）：提交单独为绿（干净的工作树里 build、vet、`make lint-go`、`make gen-check`、`make test-go-short`，经 `heavy.sh`，之后干净）；`make lint gen-check knip build` 通过；经 `heavy.sh`：`make test-go-costs-2` 第一次失败（本机根分区满，上表"修复第二轮的说明"），修剪 Go 的构建缓存之后第二次 608 秒通过、11 个包都 ok；`make test-go-short`（新测试所在的那一行）44 秒通过。照决定 242 没有重跑整个 `make test` 与 `make e2e`，由分支 CI 的全部各项担保。

CI（作业，括号里是测试一步；e2e 是 E2E 一步）：

| 作业 | 42554470668（建成，`c5a81c7a`） | 42571990070（修复一轮，`42529055`） | 42579807531（修复第二轮，`9b3000c1`） |
|---|---|---|---|
| server (race) | 13:39（13:20） | 10:36（10:04） | 14:01（13:28） |
| server (bootstrap-1) | 6:18（5:57） | 5:18（4:48） | 7:56（7:24） |
| server (bootstrap-2) | 7:30（7:11） | 6:20（5:50） | 7:44（7:09） |
| server (bootstrap-3) | 6:40（6:22） | 6:15（5:42） | 4:51（4:22） |
| server (bootstrap-4) | 7:30（7:11） | 7:41（6:50） | 7:31（7:02） |
| server (bootstrap-5) | 6:57（6:35） | 7:34（6:33） | 7:13（6:33） |
| server (bootstrap-6) | 4:25（4:05） | 5:28（4:58） | 5:36（5:04） |
| server (bootstrap-7) | 2:49（2:29） | 3:07（2:31） | 2:26（1:59） |
| server (costs-1) | 17:14（13:40） | 17:22（13:31） | 13:39（10:04） |
| server (costs-2) | 16:13（15:49） | 20:39（20:06） | 16:39（16:12） |
| server (costs-3) | 11:32（11:14） | 7:32（7:03） | 7:49（7:20） |
| server (costs-lint) | 11:41（11:21） | 12:51（12:21） | 9:12（7:49） |
| server (guards-core) | 18:49（18:29） | 19:38（19:03） | 12:59（12:28） |
| server (guards-sources) | 10:59（10:39） | 16:58（16:28） | 14:55（14:18） |
| server (short) | 4:06（3:47） | 3:21（2:52） | 5:04（3:41） |
| web | 4:53（4:18） | 3:49（3:18） | 4:48（4:16） |
| e2e | 6:06（4:53） | 6:54（5:48） | 8:29（7:02） |
| image | 1:29 | 1:30 | 1:23 |

三次都全绿，18 项，都是普通的推送（第二次前先核对 `c5a81c7a` 是新头的祖先；第三次是 `42529055..9b3000c1`）。组合根七份的测试一步都在 7:30 之下（上面"组合根的各份"）。建成时 costs-2 在 EPYC 9V74 上，新守卫 38.6 秒、中位数 1.57–1.62 秒；修复一轮 costs-2 在 Intel Xeon 8370C 上 20.6 分钟，守卫 70.1 秒（种子 22.1 秒）、中位数 2.13–2.22 秒，比上一次多约 5 分钟，其中约 30 秒是守卫变长，其余是 runner 不同（审查 C）。第七份在 9V74 上，包 124.1 秒（建成）、126 秒（修复一轮）。修复第二轮的 costs-2 在 EPYC 9V74 上 16.6 分钟，守卫 55.7 秒（种子 16.6 秒）、中位数 1.69–1.74 秒；新的小测试在 short 一行（5:04）。

合并之后 `main` 的第一次 CI 红在两个 M10 的测试，与 S2 无关，修在分支 `m11-p1-flakes`（合并 `58abac7a`），经过见 [P1 设计](../01-P1-freeze-start-rollback.md) 10.2 的"合并之后"；审查者 C 的核对另记一条留给以后碰那道守卫的 Step：M10 这道界扣的舍入余量按整数设（位图每次循环 0.5、堆每次 1.5），PostgreSQL 18 的行数带两位小数，堆的界在 relevant 一段（404 次循环）有 606 条余量、形同虚设，精确的是 `checkWork` 以 `pg_stat` 计数的聚合界；P1 不动它。
