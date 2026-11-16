# M12/P4 性能与内存实测：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M12/P4 性能与内存实测（三个 Step：S1 设施（Go）；S2 浏览器侧的设施与实测，页面树先，不等 S1；S3 服务端实测；另有 S4 起的条件修复） |
| 日期 | 2026-11-16（初稿；第一至第三次修订；S2 的实施、审查与结果） |
| 状态 | 进行中（设计已审查；S2 已合并 `54a89127`，S1、S3 未开工；条件的 S4（页面树的渲染）设计已审查，[S4 设计](04-P4-S4-page-tree.md)；条件的 S5（编辑器的解析）设计在审查；设计的[审查记录](reviews/P4-design-review.md)（三份设计审查、三次修订与三轮修订核对）；S2 的[审查记录](reviews/P4-S2-browser-measures-review.md)；S2 实施与审查之后与本文不同之处照实改在各节（4.11 列出），结果在 10.2） |
| 上级文档 | [M12 总设计](00-M12-design.md) 4.1、4.7 全节、第 3 节第 6、11 条、第 7 节 P4 一行与"次序与并行""移交的落实""总体设计的修订"第 9 条、9.4、9.6、第 10 节、第 11 节决定 305、317、318、319、329、330、352、357、359、378、424、452、需要负责人确认第 7 项；[M12 设计审查](reviews/M12-design-review.md)的 A-M5、A-M6、A-M7、A-I4、A-N6、A-N11、A-N13、A-N15、A-Q1、C-I2、C 的 U4 与过度设计 3、4；[P1 设计](01-P1-foundation.md)第 9 节写给 P4/S2 的三条；[P2 设计](02-P2-web-polish.md)第 9 节写给 P4/S2 的四条；[P3 设计](03-P3-server-correctness.md) 3.10（第 302 条改撤销的规划，B5 硬期限的窗口）；[P5 设计](05-P5-deploy-release.md) 3.18（部署文档要的数）与 2.1 第 20、21 条；[移交](handoffs/)：M4-performance 全部、M5-performance 第 3 条、M7-transfer 第 2、4、5、6 条、M8-history 第 2 条；[v0.1 总体设计](../v0.1-design.md) 13.4 第 8、9 条、第 14 节；M8 的预算与定界线的写法（[M8/P5](../M8-history-search/05-P5-search.md) 3.9、[M8/P2](../M8-history-search/02-P2-trash.md) 3.11）；M10 决定 150、153、160 |
| 代码位置的写法 | 同 00 号文档：模块写作 `page/domain/trash.go:57`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…` 在 `server/internal/` 之下；前端写作 `W/stores/page-tree.store.ts:276`（`W/` 是 `web/apps/web/src/`）；e2e 写作 `e2e/fixtures/navigation.ts:97`；其余写仓库根起的路径。行号都是 `main` 的 `86565dd6` 的（P1 完成的提交；之后到 `1ba6c7f6`，`main` 上只有 P3、P2、P5 设计的文档提交，代码与总体设计的行号不变）。00 号文档引总体设计的行号照它的头是 `5e8af5b5` 的，本文引 `86565dd6` 的，相差一行的几处不是错 |
| 工作笔记 | 设计不做整批的实测。经 `heavy.sh` 做了三个小探针（临时 worktree 在 `86565dd6`，用完删了；`TMPDIR`、`GOTMPDIR` 在 `~/.cache/m12-p4-design`）：(1) 一个 PostgreSQL 18.7 的容器，本机用户读得到它 cgroup v2 的 `memory.current`、`memory.peak`、`memory.stat`，写不了（清零 `memory.peak` 要 root）；(2) 生产配置的 `serve`（`NWIKI_ENV=prod`、JWT 的私钥、先 `migrate up`、`users create`）在一个这样的容器上起来，`GODEBUG=gctrace=1` 的行与 JSON 日志都在 stderr，空闲约 120 秒时运行时自己的强制 GC 打出一行（存活堆 3 MB），`/proc/<pid>/status` 的 `VmRSS`、`VmHWM` 读得到；(3) 本机单跑 B5 的三个守卫各一次：界线处的恢复 5.46 秒、撤销一次删除 5.50 秒、撤销 1,500 个最上层 1.54 秒（请求的用时，含解析）。另经 `gh api` 只读地取了 P1 期间 11 次 `main` 的 `guards-core` 作业日志，读出各自的 CPU 与这三个守卫的数（3.5.4）。探针的输出在 scratchpad 的 `m12/p4-work/probes/`。第二次修订另做一个探针（临时 worktree 在 `1ba6c7f6`，经 `heavy.sh`，用完删了）：照 4.4 的写法（两个 devDependencies、增量解析经 `state.update`、`knip.jsonc` 忽略 `vite`）搭 `e2e/perf/` 的解析页，`tsc`、knip、oxlint 都是 0，构建与 Chromium 里的解析通过（`m12/p4-rev2/b9/`）。第三次修订没有另做探针，B9 在上限处与上限减一的数取自修订核对 C 的 `m12/p4-check2-c/b9/`。三份设计审查另做了探针（审查记录"审查方式"一行）：审查 C 在 test 配置下量过一次 1 万页平的形状的 T1，作预告（第 9 节） |

---

## 0. 分三个 Step，另有条件的修复

照 00 号文档第 7 节，P4 分三个 Step，各自一个分支（`m12-p4-s<m>`）、一次审查与修复核对、一次合并。S1 与 S2 在 P1 之后就能开，彼此不等（2.1 第 12 条）；S3 在 P3 全部合并、S1 与 S2 合并之后。S4 起只在 S2、S3 的结论超出 4.7.1 或 4.7.2 的判据时开（3.9）。

- **S1 设施（Go）**（`m12-p4-s1`）：
  - 合成的搜索语料：组合根里一个 `//go:build !race` 的 `_test.go`，写成 `BenchmarkSearch` 读的三个 TSV，200 本与 5,000 本两种分布；另能写出 `308eafc6`（B1 的对照）那时的笔记本列；词表进仓库的 `testdata`，去掉含查询词的词，查询的词按 M8 的频率类确切地植入（3.2）；
  - 现有守卫的规模开关：升级的发起、检查、批准与 lint 摘要四个守卫读 `NW_SCALE_PAGES`，按页的日志的成本读 `NW_SCALE_CHANGESETS`；规模、期限、客户端与判不判界由一个纯函数给，开关没设时它给的就是守卫今天的常量；开关打开时只记、不判界，客户端与写超时等得比期限久（3.3）；
  - lint 守卫的 worker 期限提到界之上（worker 6 秒、请求 12 秒，照决定 359 的做法；M10 决定 160 留下的，取舍 P4-23）；总体设计 13.4 第 8 条写着的旧期限随之改（第 8 节第 5 条）；
  - 回收站两个基准的档位开关，只在手跑时读（3.3 末）；
  - `server/tools/perf/`：起一个生产配置的 `serve` 的脚本、内存与 CPU 的采样、gctrace 的解读、导入用的 vault 的生成器、五个驱动（3.4）；
  - **单独为绿**：组合根的新测试没有环境变量就跳过，只有两个不连库的测试与一个 1,000 页的载入测试真的跑；它们在 `!race` 的文件里，不进竞态的八份，Makefile 给 `test-go-costs-3` 加一行带 `-v` 的（2.1 第 8 条）；`server/tools` 模块已在 `make lint-go` 与 `test-go-costs-3` 里（`Makefile:151-156`、`:498`），新包随之被 lint 与测试；CI 的工作流不改。
- **S2 浏览器侧的设施与实测**（`m12-p4-s2`）：
  - `e2e/perf/`：自己的 Playwright 配置与一次生产配置的 `serve`、三种形状的笔记本经导入建起、页内的计时脚本；`e2e/package.json` 加一个 `perf` 脚本与解析页要的两个 devDependencies，`knip.jsonc` 给 `e2e` 忽略解析页构建用的 web 的 `vite`，`e2e/fixtures/auth.ts` 导出引导的步骤表（4.1、4.2、4.4、第 5 节）；
  - **先量页面树**：照 4.3 的界线（00 号文档 4.7.2 照抄，加量法），两轮，结论先报主持者（P2/S6 等它）；审查 C 的探针预告平的形状多半过不了（第 9 节）；
  - 再量 M4-performance 第 7、9、11、12 条与 M7-transfer 第 4 条（4.4–4.8）；第 11 条在 P2/S4 合并之后量（[P2 设计](02-P2-web-polish.md)第 9 节、决定 424），S2 量到它时 P2/S4 还没合并，就挪到 S3（4.6）；
  - 记录写进本文第 10 节与 S2 的审查记录；
  - **单独为绿**：脚本不进 `make e2e`；`tsc`、oxlint、oxfmt、knip 与换文档的门禁照常看 `e2e/perf/`。
- **S3 服务端实测**（`m12-p4-s3`）：
  - 批次 B–G（H 看负责人对 00 号文档"需要负责人确认"第 7 项的回答）；4.7.1 有预算的每一行照 3.6 量、判；只量的照 3.7 量或推算；
  - 部署文档要的数照 [P5 设计](05-P5-deploy-release.md) 3.18 的表交给 P5/S3（第 9 节的对照表）；
  - 已知限制与 v0.2 的清单；总体设计第 14 节写着 M12 的十行与 13.4 第 8、9 条照结论改（第 8 节；`NOTIFY` 与 SSE 两行由 P5/S3 改，决定 452）；
  - **单独为绿**：只改文档。
- **S4 起：条件的修复**（`m12-p4-s<m>`，m ≥ 4）：每个先写设计，带缩小规模的守卫（3.9）。

**为什么 S1 与 S2、S3 分开**：设施是代码，要审查它会不会量错；实测是文档，要审查方法、种子与数（00 号文档决定 330）。S2 的设施（TypeScript）与它的实测放在同一个 Step，是因为页面树的结论挡着 P2/S6。

**S2 不等 S1**（审查 C 的 Q1，核对过）：S2 只用 TypeScript 与 e2e 已有的帮手（`startPostgres`、`runNervewiki`、`startNervewiki` 的 `env`、`zipOf`、`startImport`、`login`、`signInContext`、`createNotebook`），起服务、建账户、建笔记本、导入都在它自己的 `setup.ts` 里（4.2）；不用 S1 的 `stack.sh`、`server/tools/perf/` 与组合根的任何代码。两个 Step 改的代码不重叠：S1 改 `server/` 与 `Makefile`，S2 改 `e2e/`、`pnpm-lock.yaml`（e2e 的两个 devDependencies，与 web 同版本，锁文件里已有）与 `knip.jsonc`（4.4）。文档有重叠（审查 C 的 N11）：两个 Step 的文档提交都改本文第 10 节与 00 号文档第 12 节 P4 一行、第 13 节，S1 另改总体设计 13.4 第 8 条与第 15 节；后合并的一个先 `git merge main`（02-workflow 3.6），在合并之后的文字上写自己的一行。所以 S2 在 P1 之后就能开，与 S1 并行，占"同时在实施的 Step 不多于两个"的一个名额（00 号文档第 7 节"次序与并行"，2.1 第 12 条）。S3 用 S1 的设施，排在两者之后。

## 1. 基线

从 `main` 的 `86565dd6` 开始（P1 完成）。P1 留下的、与 P4 有关的：

- **工具链与数据库**：Go 1.27.4、PostgreSQL 18.7（P1/S2）。实测与守卫都在它们上面量，不再量 S2 之前的版本。
- **e2e 的帮手**：`e2e/fixtures/navigation.ts:97-115` 的 `goto`、`reload`；门禁 `tools/e2e-navigation/check.mjs` 走整个 `e2e/`（`:133-143`），只认帮手模块的确切路径（`:19`、`:45`），`.goto(`、`.reload(` 在注释与字符串里也算（`:24`）。`e2e/perf/` 在它的范围里（P1 设计第 9 节）。
- **来源的恢复与撤销的守卫**（决定 359）：改在生产的 15 秒期限下跑，客户端等得比期限久（`bootstrap/source_cost_test.go:81-87`），以后在 EPYC 7763 上记下的是不被截断的实数。这也是 P4 的规模开关与 lint 守卫照的做法（3.3）。
- **CI 的组合根**：八份，P1/S2 合并之后各两次 `main` 的平均都在 450 秒之下（00 号文档第 13 节）。P4 的 S1 不改这八份（7.5）。

**已有的量的设施**（只列 P4 要用的；行号是 `86565dd6` 的）：

- **搜索**：`bootstrap/search_bench_test.go:66` 的 `BenchmarkSearch`（`//go:build !race`，第 1 行）。读 `NERVE_SEARCH_CORPUS` 指向的目录里的 `notebooks.tsv`、`nodes.tsv`、`contents.tsv`（`corpusTables`，`:282-289`；笔记本一行取表的全部列，另两个有列名），以 `COPY … FROM STDIN` 载入（`loadCorpus`，`:293`，是 `trashBench` 的方法，`trashBench` 在 `trash_cost_test.go:55`，也是 `!race`）；语料属于固定的用户 `00000000-0000-4000-8000-000000000001` 与工作区 `…0002`（slug `corpus`，`:295`）。全部入队、以生产的 `Drain` 建完（`buildSearch`，`:365-382`），记体积（`:387`）；然后 3 个统计阶段（无统计、ANALYZE 之后、为任意的参数规划，`:76`）× 3 种可见（200/200、10/200、一本，`:77`）× 45 个查询（`:271-278`）× 第一、第三页 × 积压的档（`NERVE_SEARCH_BACKLOG`，`:75`、`:158`；M9/P3 加的，`308eafc6` 上没有），每格热身一次之后 `NERVE_SEARCH_REPS`（默认 20，`:71-72`）次，`b.Logf` 写每格的 p50、p99（最近秩，`nearestRank` `:469`）。没有 `ReportMetric`，没有按格过滤的开关。注释写的跑法是 `-benchtime 1x -timeout 4h`，四档积压要 `-timeout 6h`、要 `-v`（`:31`、`:47`、`:65`）。
- **B5 的基准与守卫**（都是 `//go:build !race`）：
  - `bootstrap/trash_cost_test.go`：`BenchmarkTrashRestore`（`:307`，轻与满两种负载 × 附件 0、1,000、2,000、5,000、10,000，每档先建一个库、建单元、删一次，`:308-316`；无统计与 ANALYZE 两格；每次迭代计时，报 `request-ms`、`parse-ms`、`unit-ms` 等，是 `b.N` 次的均值，`:334-338`）、`BenchmarkTrashDelete`（`:352`，`ImportMaxEntries` 个节点）、`BenchmarkTrashRestoreHistory`（`:449`，每页 250–2,000 个版本，版本一路累加，`:455-466`）；守卫 `TestARestoreAtTheUnitsBoundTakesItsTime`（`:522`，无统计、不 `VACUUM`，记在 `:538`，界 14 秒 `:539`）。满负载是 `benchLoads()` 的 full：单元的三个上限分给 100 页，合计 8 MiB、20,000 条链接（`:196-199`）。基准的应用在 `newTrashBenchOf`（`:111`）：请求 15 秒、读 30 秒、写 60 秒（`:117`），`MaxConns` 10（`:118`），关掉十几张表的 autovacuum（`:146-151`），经 `httptest` 进程内调用。
  - `bootstrap/revert_cost_test.go`：`BenchmarkRevert`（`:237`，删除、恢复、新建三种 × 另有 0 或 10 万个节点 × 三个统计阶段）、`BenchmarkRevertOfManyTops`（`:418`，`NERVE_BENCH_TOPS`，默认 1,000、1,500、2,000、3,000，`manyTops` `:483-485`）；守卫 `TestARevertAtTheUnitsBoundTakesItsTime`（`:580`，记 `:622`，界 14 秒 `:623`）、`TestARevertOfManyTopsAtItsBoundTakesItsTime`（`:649`，记 `:669`，界 8 秒 `:670`）。
  - `bootstrap/background_cost_test.go`：`BenchmarkBackgroundJobs`（`:466`）、`BenchmarkBackgroundSteps`（`:566`，`top-ms`、`group-ms`）、`BenchmarkStartRevert`（`:414`），不按 `b.N` 循环，M8 以 `-benchtime 1x` 跑（`:33`）；`NW_JOB_LINKS`（默认 20，`:67-69`）、`NW_JOB_SIZES`（默认 5、6、7 万，`:78-80`）；满负载是 2 KiB 正文、每页 20 条链接、十分之一是附件（`:56-60`）；五个守卫的界在 `:935-941`。
- **lint、`index` 与升级的守卫**：
  - `bootstrap/wiki_lint_cost_test.go:40` 的 `TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory`：请求 6 秒、worker 3 秒（`wikiCostRequest`、`wikiCostWorker`，`:31-34`；worker 经夹具 `wiki_lint_cost_fixture_test.go:53` 设），摘要与 `wanted` 各 3 个样本，记 wall、CPU、`TotalAlloc`、`HeapAlloc` 的峰值增量，界在 `:116-123`（摘要的墙钟 5.2 秒，在 3 秒的 worker 之上，轮不到它判，审查 B 的 Q3）；日志是 `wiki_lint_cost_test.go:65: wiki cost summary median wall-ns=… bound=…`（`t.Helper` 让行号是调用处）。夹具 `newWikiCostFixtureOf(t, 名字, 页数)`（`:49`）也给升级的检查用（5 万页）；`wikiCostRequest` 另是检查的期限（`upgrade_check_cost_test.go:142`）与发起的期限的来历（`startCostDeadline = wikiCostRequest*3/4`，`upgrade_start_cost_test.go:54`）。
  - `linking/adapter/postgres/index_cost_test.go:121` 的 `TestIndexOfAHundredThousandPagesTakesItsTime`，`indexCallBound` 3.5 秒（`:37`），在 `test-go-costs-1` 带 `-v`（`Makefile:421`），日志是 `index_cost_test.go:326: ListIndex of a page of 100: [...]`。
  - 升级：`bootstrap/upgrade_start_cost_test.go`（5 万页 `:42`，期限 4.5 秒 `:54`，界 3.6 秒 `:68`，写超时钉在 15 秒 `:111`，计时的请求经 `ask`，`:155`）、`upgrade_check_cost_test.go`（5 万页 `:21`，界 1.5 秒 `:36`，进程内调用）、`upgrade_approve_cost_test.go`（47,500 个来源 `:46`，请求 15 秒 `:66`，写超时 45 秒 `:357`，界 3.9 秒 `:84`，计时的请求经 `ask`，`:412`）。`ask`（`permission_matrix_test.go:311-330`）最后经 `sendRequestBy(t, client(), req)`（`contract_test.go:381`、`:385`），`client()` 5 秒就放弃（`app_test.go:48`）。批准的守卫在播种里也经 `ask` 发起升级、再提交（`upgrade_approve_cost_test.go:239`、`:247`），两者都随来源数变大（修订核对 A 的 M12）。
  - 按页的日志：`page/adapter/postgres/history_cost_test.go:42` 的 `newLongLog`（`!race`），10 万个变更集写死在 SQL 里，几个数随它定（`revision = 20000`、`c.k <> 99950`、条目数 `50_000+20_000+59_999`，`:42-108`）；200 ms 的守卫 `TestTheLogReadsInTimeAsLongAsAPage`（`:161`、`:281`）与 `TestTheRevertsArrayReadsInTimeAtTheUnitsBound`（`:296`）用它。
- **CI 上的实数**：守卫在 `test-go-guards-core`（`Makefile:530-570`）、`test-go-guards-sources`（`:571-590`）、`test-go-costs-*`、`test-go-costs-lint` 里各自一行带 `-v`，作业的 "Runner CPU" 一步打出 CPU（`ci.yml:167-168`）。同一次运行的各作业落在不同的 CPU 上，按作业挑。读作业的日志要认证（`gh api --allow-escape-sequences …/actions/jobs/<id>/logs`，06-environment 第 4 节）。
- **生产配置的预算**（`server/configs/config.yaml`）：`request_timeout` 15 秒（`:17`）、`read_timeout` 30 秒、`write_timeout` 60 秒（`:11-12`；校验要求 `read + request < write`，`platform/config/validate.go:50-58`）、`max_body_bytes` 1 MiB（`:23`）、`max_conns` 10（`:37`）；`authenticated` 每凭证每分钟 1,200、突发 200（`:76`）；`access_token_ttl` 15 分钟（`:46`）；`parse_budget_bytes` 8 MiB、`parse_max_wait` 2 秒（`:97-98`）；`job_timeout` 6 小时（`:144`）；导入 512 MiB 的 zip、5 万个条目、解压 4 GiB（`:151-157`），节点至多 6 万（`page.MaxJobItems`，`bootstrap/deps.go:253`）；日志是 JSON（`log.format`，`:159-161`；测试的配置是文本，`config.test.yaml:26-28`）。大请求体：整个服务 2 个名额、每凭证 1 个、等 2 秒（`bootstrap/mcp.go:40-41`，`platform/httpserver/largebody.go`），REST 两个正文路由过 1 MiB 的也取它（`page/adapter/http/limits.go:8-17`）。lint 的期限 `min(10 秒, request_timeout/2)`（`bootstrap/wire.go:253`），名额 `max(1, floor(floor(max_conns/2)/2))`（`:263`）；升级的发起与批准 `request_timeout*3/4`（`:281`、`:297`）。
- **没有的**：合成语料的生成器、并发的驱动、进程与容器的内存采样、导入用的大 vault、浏览器一侧的计时脚本（00 号文档 4.7.4；I2 第 4.2 节）。服务端没有 pprof、expvar 或 metrics 端点（M12 也不加，决定 319）。`server/tools` 的 `go.mod` 没有 PostgreSQL 的客户端。

## 2. 目标与范围

**做**（00 号文档 4.7、第 7 节 P4 一行）：

- S1、S2 的设施进仓库，可复现、可审查；
- 4.7.1 有预算的 B1–B10 每一行：量、写明判在哪台机器上、对照预算给"过"或"超"；
- 页面树照 4.7.2 的界线（本文 4.3 照抄）量、判，结论先报主持者；
- 部署文档要的数（[P5 设计](05-P5-deploy-release.md) 3.18 的表，本文第 9 节逐项对照）；
- 只量的各项：量，或用已有的数推算并写明"推算"与依据；推不出又不判预算的（事件流同时重连、按凭证的并发上传）写明不量与理由（00 号文档 4.7.1 末段）；
- 超出的在 v0.1 修，各开一个条件的 Step，带缩小规模的守卫；B5 的规则一层没过、或硬期限一层的推断过了 15 秒时，照 4.7.5 列给主持者与负责人定。

**不做**：

- CI 不加 10 万、30 万页的守卫（决定 319；I2-D5）。只有修了东西的 Step 加缩小规模的守卫。
- 产品里不加 pprof、metrics 端点与新的配置项（决定 319；00 号文档 5.2）。
- 已经知道的超出项照 4.7.1 的处置，不重开（00 号文档 4.7.5 末条）：搜索 300 ms 的尾巴与约 5 倍的体积、按页的日志在 13–25 万个变更集过 200 ms、`DeleteNode` 与单个 `move` 的病态形状、lint 的 `TotalAlloc`。
- 不为守卫优化：守卫的规模与时限照负责人 2026-11-06 的话调（决定 317；M10 决定 150、153、160 的先例）。
- 不量：CPU 降速、持续改树（00 号文档 4.7.2"只记的"）、事件流同时重连、按凭证的并发上传（4.7.1 末段）、导入导出占用的连接与磁盘写满时的 `storage_full`（3.7）、迁移在大库上的停机（批次 H，看负责人的回答）。

### 2.1 对 00 号文档的修订

接受本文的文档提交同时照下面改 00 号文档（第 8 节第 4 条写出每段新文字，修订脚本断言旧文字各恰好出现一次）。

1. **B5 的"满负载"是 M8 的口径：数据的形状，不是并发的编辑者**。M8 定界线时的满负载有两种，各带出处（审查 B 的 M3）：
   - `MaxUnitNodes`、`MaxUnitHistory`、`MaxRevertSteps`：单元的三个上限分给 100 页，合计 8 MiB 正文、20,000 条链接，每页约 82 KiB、200 条（`benchLoads()` 的 full，`bootstrap/trash_cost_test.go:196-199`；[M8/P2](../M8-history-search/02-P2-trash.md) 3.11 第 470、486、496 行）；
   - `MaxJobItems`：2 KiB 正文、每页 20 条解析的链接、十分之一是附件（`bootstrap/background_cost_test.go:56-60`；移交 `M4-performance.md:392`）。

   它们由 `BenchmarkTrash*`、`BenchmarkRevert*`、`BenchmarkBackground*` 在进程内量。00 号文档 4.7.4 写的"N 个编辑者自动保存加树写（B4、B5 的满负载与部署文档的内存）"把两件事并在了一起：本文里 B5 只用现成的基准量（3.6 的 B5），编辑者的驱动只服务 M4-performance 第 1 条（只量）与部署文档的内存（3.7）。4.7.4 那一句照第 8 节改。
2. **B1 怎样判**（审查 B 的 I1、I2，C 的 M5；取舍 P4-3）：
   - **吞吐、1 小时与保存照原预算直接判**：任务 ≥ 100 页/秒、30 万页从空建 ≤ 1 小时、一页的保存 p99 多 ≤ 2 ms（保存可以推算，写"推算"）。超了就是超，不经对照。
   - **两步只给 M8 已接受的两项**：300 ms 的尾巴与约 5 倍的体积。第 1 步比 M8 记下的最大：200 本 719 ms（P5A 真语料"为任意的参数规划"那一阶段；无统计是 348 ms，[M8/P5](../M8-history-search/05-P5-search.md) 3.9 的表，`:894`；R1 之后的合成语料上 474.6 ms），5,000 本 827.2 ms（R1 之后，无统计；同节末的表，`:1102-1107`），体积 5.25 倍（R1 之后按节点的 id 建）。第 2 步超了的，在同一台机器、同一份语料上对照 `308eafc6` 两轮，也慢 20% 以上才算回归。
   - **"建索引时"与"导入 5 万页的清空期间"沿用 M8 的数、不重量**（908 ms、1,149 ms，M8 第 1 步的工具量的，`BenchmarkSearch` 没有这两个阶段）；**积压的格只记一轮**：1,191 ms 是 M8 第 1 步的工具在另一种负载下量的，与 `BenchmarkSearch` 的积压不同口径，`308eafc6` 上也没有积压的开关（审查 B 的 I2）。00 号文档的"建索引与积压时约 1.2 秒"照第 8 节改写。
3. **"推断"用本机对 EPYC 7763 的实测之比，不用 2.33 与 4.6**（取舍 P4-2，主持者采纳；审查 B 的 Q1、M1、M2，C 的 M4）。00 号文档 4.7.1 自己写了：2.33、4.6 的分母是更快的 M5 Max，拿它乘更慢的本机会多算一层（本机约比 M5 Max 慢 1.4 倍，7763 约慢 2.8 倍，"本机 × 4.6"约等于"M5 Max × 6.4"）。比的两边同口径：同一组守卫在 7763 上各次的中位，除以本机在同一批提交上的中位（3.5.4）；只留回收站与撤销、升级两族。设计时的探针给了第一族的预告：约 2.0–2.1（撤销很多最上层 1.7–1.8）。照旧的倍数的结论并列写出，不同时列给主持者。00 号文档里写着倍数的各处一并改（第 8 节）：4.7.1 表里 B4、B8 的"本机 × 4.6"，B6、B7、B10 的"本机 × 2.33"；"判在哪台机器上"的第二、第三条；4.7.3"机器"一条；第 11 节决定 318 的那一半（写明由 P4 的决定改）；第 3 节第 6 条与 9.4 的"或乘了哪个倍数"改为"或乘了哪个比"（修订核对 B 的 N13；4.7.1 的动机例子"慢 4.6 倍的主机"说的是一台假想的慢主机，不改）。
4. **页面树的形状与终点写清**（4.3）：
   - 节点树的次序是 `sort_order` 再按 id（`page/domain/tree.go:9-36`），不是名字。T2 的"名字按次序排在最前"改为"以 `after_id: null` 新建，排在同级的最前"（`api/dist/openapi.yaml:3937-3938`）。
   - 深的形状照 4.7.2 的字面是 10 + 100 + 10,000 + 1 + 1,000 = 11,111 页，不是整 1 万；那个 1,000 个孩子的文件夹放在根下、排在最前（名字以 `000` 开头，导入按名字排）。带附件的形状另有 1,000 个附件，分在 100 个第二层的文件夹下，各 10 个。
   - T3 只在深的两种形状上判：平的形状没有文件夹。
   - 左栏没有自己的滚动容器，整页滚动（`W/pages/workspace/workspace-layout.tsx:111-113`），"视口"就是窗口的视口；树没有 ARIA 的 tree，行是 `nav[aria-label="Pages of …"]` 里的 `li`（`W/pages/notebook/page-tree.tsx:29-34`、`:126`）。
5. **对外量的大笔记本经导入生成的 vault 建起，不另写 SQL 的播种**（取舍 P4-5）。导入是生产的路，写入的节点、正文、版本、链接、搜索队列与事件都是一致的；一次至多 5 万个条目、6 万个节点（`config.yaml:151-157`），10 万页由两次导入建起，30 万页只在进程内由现成守卫的规模开关量。00 号文档 4.7.4"合成的大笔记本：1 万、5 万、10 万、30 万个节点；链接、附件、历史可调"照第 8 节改。GB 级的 vault 也是这一个生成器，不在组合根的 `_test.go` 里（它不用应用，驱动要读它）。
6. **浏览器的线上字节离线算压缩，不起两种代理**（取舍 P4-12）：4.7.2"线上的字节（代理开压缩与不开各一次）"改为"树的答复的 JSON 字节，与它在 gzip 1 级（nginx 的默认）、6 级与 zstd 3 级（Caddy `encode` 的默认）下的字节"。
7. **knip 读到 `e2e/perf/` 的配置**（取舍 P4-20）：经 `e2e/package.json` 的 `perf` 脚本，knip 的 Playwright 插件从脚本里解析 `-c`；但它只收字符串的 `testMatch`，`globalSetup` 按 workspace 的目录解析，所以配置写字符串的 glob 与绝对路径（审查 C 的 M1 实跑过）。`knip.jsonc` 只多一处：`e2e` 一节的 `ignoreDependencies: ["vite"]`，B9 的解析页按路径载入 web 自己的 Vite，e2e 不依赖它（4.4；修订核对 C 的 M6 实跑过）。读不到配置时才在这一节另加入口。00 号文档 4.7.4 与第 7 节 P4/S2 一行的"`knip.jsonc` 给 `e2e/perf/` 加入口"照第 8 节改。
8. **S1 的 Makefile 只加一行，不改竞态的八份**（审查 A 的 I1；取舍 P4-25）：语料的载入测试调 `loadCorpus`、`buildSearch`，它们是 `!race` 的文件里 `trashBench` 的方法，放进不带标签的文件会让竞态的八份都编不过。所以语料的生成器与它的三个测试都在 `//go:build !race` 的文件里，`test-go-costs-3` 在 `server/tools` 那一行之前加一行 `go test -count=1 -v -run '^TestPerfCorpus…$$'`（7763 上推断多约 15 秒，7.5）；第七、第八份的模式不改（决定 328 是给竞态的整个程序的测试的）。`server/tools` 模块的新包已由 `make lint-go`（`Makefile:151-156`）与 `test-go-costs-3` 的 `go -C server/tools test -race -count=1 ./...`（`:498`）看着，不加目标。00 号文档第 7 节 P4/S1 一行的"内容"与"组合根与 CI 的用时"两格照第 8 节改（另写 lint 守卫的 worker 与回收站基准的档位开关）。
9. **B2 的"堆峰值"、B3 的"约 1 GB"写成可判的数**（取舍 P4-7、P4-21）：堆峰值是 gctrace 每行"GC 开始与结束时的堆"两者的最大值在负载期间的最大，减这一轮的基线（服务起来空闲 130 秒之后、运行时强制 GC 那一行的存活堆）；B2 超出是它大于 2,400 MiB（8 MiB × 300，`config.yaml:97` 的注释；README 的"约 2.4 GB"是同一个乘积的约数，增量落在 2,289–2,400 MiB 之间时两样都写、列给主持者，审查 B 的 N9），B3 超出是两轮六次里的最大大于 1,024 MiB。M4 的 1.5 GB 与 M9 的 +961 MiB 是进程内每 5 ms 采样的 `HeapAlloc` 与 `HeapInuse`（[M4/P4 审查](../M4-pages/reviews/P4-content-sessions-review.md)第 92 行；[M9/P2](../M9-mcp/02-P2-writes-history.md)第 353 行），口径相近、不相同，记录写明。
10. **4.7.3"建库参数同测试（builtin `C.UTF-8`、LZ4）"**：LZ4 不是建库参数，是迁移 00054 给列设的压缩（`server/migrations/sql/00054_page_content_compression.sql:3-4`）。改为"建库参数同测试（`--locale-provider=builtin --locale=C.UTF-8`，`pgtest/pgtest.go:36`）"。另写明：进程内的基准与守卫经 `pgtest`，testcontainers 的 PostgreSQL 模块给它加了 `fsync=off`（`postgres@v0.44.0/postgres.go:163`），M8 定界线时也是这样；对外的 `serve` 用的容器不加，同生产（取舍 P4-19）。
11. **00 号文档引总体设计的行号不改**：它的头写明行号是 `5e8af5b5` 的，B2 的 `:1300`、B5 的 `:1171`、B9 的 `:1227`、B10 与 B11 的 `:1275`、页面树的 `:1316` 在那个提交上都对；`86565dd6` 上各下移一行，本文与 P4 的记录引新号（初稿要改其中两处，是设计者看错了，审查 B 的 M10、C 的 N1）。
12. **P4/S2 不等 P4/S1**（审查 C 的 Q1，主持者定；取舍 P4-24）：核对见第 0 节末段。00 号文档第 7 节"次序与并行"的"P4/S2 在 P4/S1 之后"改为"P4/S2 不等 P4/S1"，P1/S2 合并之后可以开的 Step 加 P4/S2，P4/S3 写明在 P4/S1 与 P4/S2 之后；4.1 的"P2、P3、P4/S1、P5/S1 可以并行"加 P4/S2。
13. **B5 的统计量与硬期限的推断**（审查 B 的 I4、M4；取舍 P4-1、P4-16）：
    - 规则一层的"两轮的较大中位数"是 M8 的同一量法：回收站与撤销的基准每轮是 `-benchtime 3x` 报的三次均值（`ReportMetric` 取 `b.N` 次的均值，`trash_cost_test.go:334-338`），后台的基准不按 `b.N` 循环、每轮一个样本（`1x`，`background_cost_test.go:33`），两轮取较大；
    - 硬期限一层的判据照旧是 7763 上的实数加 2 秒，只取被量的代码没再改过的那一段里的各次（取舍 P4-16；修订核对 B 的 M13）；另并列两行推断（3.6 的 B5），任一行过 15 秒就同规则一层列给主持者与负责人，部署文档的硬件下限照他们定的写（修订核对 B 的 N18）；00 号文档第 3 节第 6 条的"B5 的规则一层没过的"随之加"或硬期限一层的推断过了 15 秒"（修订核对 B 的 N17）。

    4.7.1 B5 一行"超出的判据"一格照第 8 节改。
14. **机器时间**（3.8、4.10）：00 号文档 4.7.3 写"缩小之后的机器时间少于初稿估的 10–16 小时"，P4 重估为 S3 约 16–20 小时、有条件的另约 4.5–6 小时，S2 约 3–3.5 小时，多出的原因写在 3.8。4.7.3"批次"一条与第 7 节 P4/S3 一行"组合根与 CI 的用时"一格照第 8 节改。
15. **"总体设计的修订"加第 25 条：13.4 第 8 条随 P4/S1 改**（修订核对 B 的 M11；取舍 P4-23）：那里写着十万页摘要的成本用例"请求 6 s、worker 3 s（决定 160）"，S1 改成 12 秒与 6 秒之后它就过时；S1 的文档提交改这一句、第 15 节加一行，同决定 359 的先例。新旧文字在第 8 节第 5 条。

### 2.2 需要负责人确认

1. **有没有 v0.1.0 之前就在跑、要升级到 v0.1.0 的实例**（00 号文档"需要负责人确认"第 7 项）。没有就不量批次 H、不写开发版的附录。什么时候要：P4/S3 开始之前，主持者与 P5 的几问一起问。推荐的答复：没有（只有负责人自己的开发库）。

另有几件预告，不要负责人现在答（第 9 节；结论出来之后照 00 号文档 4.7.5 列给主持者与负责人）：

- **B5 的规则一层在本机多半过不了**。探针 (3) 里界线处的恢复与撤销一次删除的请求在本机是 5.46、5.50 秒，减去约 0.2 秒的解析，单元约 5.2–5.3 秒，过了 M8 的 5 秒；M8 在 ANALYZE 状态下 419,900 行的单元在 M5 Max 上 4,288 ms，乘约 1.4 倍约 6 秒（审查 B 核过）。
- **B5 的硬期限一层的推断照今天的数已过 15 秒**（修订核对 B 的 M15）。第一行推断（7763 各次的中位 × 1.25 加 2 秒）只要中位过 10.4 秒就过 15 秒；今天的两次 7763 是恢复 11.0 秒、撤销一次删除 11.35–11.48 秒（3.5.4 的表），算出来 15.75–16.4 秒。第二行：守卫量的是无统计、不 `VACUUM` 的状态，不是最慢的；M5 Max 上 ANALYZE 状态的单元 4,288 ms 加解析约 4.5 秒，对守卫的请求 3.62–3.94 秒（[M8/P2](../M8-history-search/02-P2-trash.md) `MaxUnitHistory` 一段），慢约 1.14–1.24 倍；推到 7763（界线处的恢复约 11.0 秒）加 2 秒约 14.5–15.7 秒，平常的 7763 就可能过 15 秒（审查 B 的 I4、修订核对 B 的 N15，推断）。只要 P3 之后这几个守卫在 7763 上没变快，S3 量定之后，规则一层与硬期限一层多半一起照 00 号文档 4.7.5 列给主持者与负责人：降常量，或写进部署要求（要求比 7763 快多少的 CPU）；部署文档的硬件下限照他们定的写。主持者可以在 S3 之前，与上面第 1 条、P5 的几问一起，先告诉负责人。
- **P4-2 多半决定 B4 删除的结论**。B4 删除 `vault`（约 6 万个节点、每页 30 条链接）照 M8 的数推，本机约 6–7.5 秒；乘实测之比（约 2.0）是 12–15 秒，贴着期限；乘 4.6 是 28–35 秒，必"超"（审查 B 的 Q1，推断）。
- **页面树的平的形状多半过不了**。审查 C 的探针（不是正式的量）里 T1 约 2.4 秒，界线 1 秒（第 9 节）。
- **B9 多半超**（修订核对 C 的 Q2 探针，不是正式的量：没持 heavy 的锁、机器上有别的 Step、每种热身两次加五次、两次运行）：一串空格的一段正好在上限处，整篇解析 170–440 ms，过 80 ms；上限减一再插一个字 44–110 ms，随负载与形状在 80 ms 上下（照 4.4 加上头之后中位约 48 ms，修订核对 C 的 N13）；`*a** ` 与 `[a](b (x ` 也各有一次 84–93 ms 的整篇。"多半超"靠整篇那一格。正式的量在 S2（4.4）；没过时照 00 号文档 4.7.5 另起改法的设计，不在本文预先设计（主持者定，同页面树；第 9 节）。

### 2.3 本文的取舍（主持者的决定，负责人可以改判）

正式编号 453–478，主持者在设计审查之后给的：P4-1 到 P4-25 是 452 + k，P4-26 已是决定 452（P5-23），P4-27 是 478；括号里是临时号，审查记录用它。设计审查之后由主持者定的标"主持者定"。

- **453（P4-1）B5 的满负载照 M8 的口径，用现成的基准量；一轮是一次 `go test -bench` 的运行；每轮的值照 M8 的量法**（2.1 第 1、13 条）：回收站与撤销的基准取 `-benchtime 3x` 报的三次均值，后台的取 `1x` 的一个样本，两轮取较大。理由：B5 的规则是 M8 写下的，判它要用 M8 定界线时的同一种量法（`unit-ms`，请求减解析），否则不是同一个预算。一次运行起一个新的 testcontainers 容器，满足 4.7.3"各自重启 PostgreSQL"。
- **454（P4-2）推断用本机对 7763 的实测之比**（主持者定，采纳；2.1 第 3 条，3.5.4）：
  - 比的两边同口径：7763 上各次的中位，除以本机在同一批提交上的中位；
  - 只留两族：回收站与撤销（B4 用），升级（B7 用；B6、B10 的 `index` 没有 7763 的实数时也用它，旧的倍数里它们与升级同属 2.33 一组）；B8 取两族的较大；
  - 本机只在每个作业最近的至多三次 7763 的提交上跑那一族，另在被量的提交上跑两族（3.5.4；修订核对 B 的 M14）；
  - 对硬期限的推断（B4、B7）另并列一行"× 1.25"（决定 359 记的 7763 的尾巴：慢一截的那几台慢 9–25%），它过期限而主行没过时，照 B5 的推断行一样列给主持者，不自动判"超"（设计者的取舍）；
  - 旧倍数的结论并列写出，不同时列给主持者；
  - 理由：同一组守卫两边的数今天就有；照旧乘 4.6 会把本机 4 秒的操作推成 18 秒、判"超"，而推断 7763 上约 8–9 秒（审查 B 的 Q1：旧倍数对 7763 高估约 2.3 倍）。
  - **P4-2 多半决定 B4 删除的结论**（12–15 秒对 28–35 秒，2.2），写进第 9 节与 M12 的汇报，负责人可以改判。
- **455（P4-3）B1 照原预算判吞吐、1 小时与保存；尾巴与体积先比 M8 记的最大，超了才在同机同语料上对照，对照也慢了才叫回归**（主持者定；2.1 第 2 条，3.6 的 B1）。对照取 M8 的 Codex 评审合并 `308eafc6`（按笔记本分键之后，今天的设计）：同样两轮、整轮地跑（`BenchmarkSearch` 没有按格过滤的开关），两边都取两轮里较大的 p99；生成器另写出那时的笔记本列，S1 的试跑真的把 1,000 页喂给 `308eafc6` 一次。积压只记一轮，不对照。理由：本机、合成语料都与 M8 的不同，比 M8 的数大可能是机器与语料，不一定是代码；吞吐与 1 小时是写下的预算，从没被接受为超出，照它判（审查 B 的 I1）。
- **456（P4-4）搜索语料的词表进 `testdata`；查询的词确切地植入**（3.2；审查 A 的 M1、M2）。词表取仓库自己的文字，生成一次进仓库，之后同一个种子出同一份语料；词表里去掉含任一非重类查询词（折叠之后）的词，段落拼出了它们就重抽；非重类的词按目标的页数确切地植入，测试断言确切的数。不改成种子生成的合成词表（审查 A 的过度设计第 2 条，未采纳）：00 号文档 4.7.4 写了取仓库的 Markdown，中文二元组的分布决定每页的键数与倒排的长短，进的是体积与 p99。
- **457（P4-5）对外量的大笔记本经导入建起**（2.1 第 5 条）。理由：导入是生产的路，什么都一致；SQL 的播种要逐张表抄应用的不变式，审查要核对它。代价是一次 6 万个节点，10 万页导两次。
- **458（P4-6）驱动、采样与 vault 的生成器放在 `server/tools/perf/`；Go 只用标准库，读库经 `os/exec` 调 `docker exec <容器> psql`；MCP 手写 JSON-RPC；起服务的是一个 shell 脚本**（3.4；审查 A 的 M6）。理由：`server/.golangci.yml:36-45` 不许组合根与 mcpserver 之外导入 MCP 的 Go SDK；`server/tools/go.mod` 没有 PostgreSQL 的客户端，经容器里的 `psql` 读几个系统视图就够，`go mod tidy -diff` 不变；起容器与进程照 `deploy/image-smoke.sh` 的先例写 shell，逐步判返回码。
- **459（P4-7）内存的读数**（2.1 第 9 条，3.4.2；审查 A 的 M4、M5）：服务进程的 Go 堆取 gctrace，峰值是每行 GC 开始、结束两个堆的最大；每轮一个基线，取服务起来空闲 130 秒之后运行时强制 GC 那一行的存活堆，各组合都对它算增量（偏保守）；窗口里最后一次 GC 之后的那一段取上界 `max(峰值, 窗口之后第一行的目标)`，上界跨过界线时那一组合之后空闲 130 秒重量；被访问日志插断的 gctrace 行拼回，`gc N` 缺号就报出、重量；进程的开始时刻用 `/proc/uptime` 换算；另记 `VmHWM`、`VmRSS`；PostgreSQL 每轮一个新容器，读 `memory.peak`，每 100 ms 读 `memory.current`，轮末读 `memory.stat` 的 `anon`、`file`、`shmem`。理由：不改产品就能读到；gctrace 的每行正是那一个周期里 `HeapAlloc` 的最大处，与 M4 的采样口径最近。
- **460（P4-8）只为准备而设的限流放宽，量的限流照默认**（3.4.4）：`password_user`、`login_ip`、`login_ip_email` 三个桶在实测时调到每分钟 600；`authenticated`、`anonymous`、`asset_content` 照默认。每个编辑者、读者、写者一个 PAT；浏览器的测试各自以邮箱与密码登录（令牌 15 分钟就过期，审查 C 的 N4）。理由：一个账户 50 个 PAT 照默认要建 10 分钟；这三个桶不在任何被量的路上。
- **461（P4-9）B4 的正常形状与病态形状**（主持者定，照初稿；3.6 的 B4）：正常形状是导入上限的 vault（约 6 万个节点、每页 30 条链接，链接写不带路径的名字、九成指向同一个文件夹附近的页）；被很多页链着的页的改名（M6：每页 10 KB、改写 2,500 页 10.3 秒，5,000 页到期限）与 M6 的 400 页 × 2,000 条链接的 `move` 算病态，用已有的数记成已知限制，并照 3.5.4 的比换算到 7763 的规模、标"推断"（审查 B 的 M9：例如被约 1,300 页链着的页的改名在 7763 上就到期限）。理由：00 号文档只给了删除与 `move` 的病态形状；改名在 M6 已量过、写进第 14 节；"一页被几千页链着"算不算正常要负责人看，所以写成可改判。
- **462（P4-10）B4 在空载上判，在编辑者的负载下只记；每次删除之前等后台安静**（3.6 的 B4；审查 B 的 M8）。理由：B4 问的是操作本身的大小到没到期限；并发编辑者的影响是 M4-performance 第 1 条的事；恢复把约 4.5 万页排回搜索的队列，不等它清空，下一次删除就与搜索的任务抢 CPU。
- **463（P4-11）页面树的量法**（4.3）：页内的计时脚本以新文档的 `timeOrigin` 为零点；"已画出"是条件第一次成立之后的第二个动画帧；每种形状每轮一个浏览器上下文、登录一次，每次 T1 开一个新的标签页、之前经 CDP 清 HTTP 缓存；T2 以 `after_id: null` 新建，只看根列表的第一行；T3 只在深的两种形状上判。理由：见 4.3 的各条。
- **464（P4-12）树的压缩离线算**（2.1 第 6 条）。理由：只记的一项；起 nginx 与 Caddy 要拉镜像，量出的也只是同一份 JSON 的压缩比。
- **465（P4-13）B9 在 Chromium 里量解析本身**（4.4；审查 C 的 M2）：`e2e/perf/parse/` 把 `W/editor/markdown.ts` 的 `markdownEditing()` 与测试用的那几个导出，以 Vite 的库构建（`configFile: false`，`root` 是 web，IIFE）打成一页小的包，在 Chromium 里对每种最坏的形状计时；e2e 加 `@codemirror/state`、`@codemirror/language` 两个 devDependencies（与 web 同版本，同一份存储，构建不用别名，包里只有一份 CodeMirror）；增量解析经 `state.update` 与 `ensureSyntaxTree`，同编辑器自己的做法，不另要 `@lezer/common`；`build.ts` 按路径载入 web 自己的 Vite，`knip.jsonc` 给 e2e 忽略 `vite`（修订核对 C 的 M6 实跑过）。另在真的编辑器里量按一个键到下一帧，只记，也作上界。理由：B9 的预算是"每次解析"，按键到下一帧还含 CodeMirror 的视图，比它大；V8 对长字符串的哈希只在 Chromium 里那样（05-lessons 第 5 节），所以不拿 Node 的 vitest 计时代替；web 的 Vite 配置分块，打不出 IIFE（审查 C 实跑过）。
- **466（P4-14）10 万页的树的重读用 Go 的驱动模拟标签页**（3.7）。理由：量的是服务进程的 CPU 与出口字节，同一份树的读与 refresher 的节流在驱动里照做就够；八个真的 Chromium 标签页自己的 CPU 会抢服务进程的。
- **467（P4-15）记录与原始数据**：每一行的数与结论写进本文第 10 节（10.2 S2、10.3 S3）与各自的审查记录；原始的采样、日志与 JSON 放本机 `~/.local/state/nervewiki/m12-p4/<批次>/<轮>/`，gzip，记录写每个文件的 sha256。理由：同 docs/README"其他"的约定（本机证据不进仓库）；脚本与种子在仓库，审查者能从它们重跑。
- **468（P4-16）B5 的硬期限**（主持者定；3.6 的 B5；审查 B 的 I4，修订核对 B 的 M13）：
  - 判据照 00 号文档：落在 7763 上的 `main` 的 `guards-core` 的三行，最大值加 2 秒 > 15 秒为超；
  - **窗口**：从这三条守卫与它们量的文件（3.6 的 B5 列出）最后一次被改到的那个提交起，到被量的提交为止，窗口里落在 7763 上的各次都取；窗口里的运行是 `main` 上提交包含那个改动的各次（`git merge-base --is-ancestor`；修订核对 B 的 N16）。只改别的测试函数（例如 P4/S1 给 `BenchmarkTrashRestore` 加档位开关）或只改期限与客户端的提交不算改到；算不算，S3 读那几个提交的 diff 定，写进记录；
  - 不到三次就用已有的，写明次数；
  - 一次也没有时（例如 P3/S2 改了撤销的规划之后，`main` 上还没有一次落在 7763 上）：取改动之前最近的至多三次 7763，各乘本机在被量的提交上与在那一次的提交上的守卫之比（本机新 ÷ 本机旧，同 3.5.4 的同提交做法），取最大加 2 秒，写"推断"；这几个提交就是 3.5.4 里 `guards-core` 那一组，不另跑；
  - 另并列两行推断：7763 各次的中位 × 1.25 加 2 秒；本机基准里最慢的一阶段（多半是 ANALYZE）对本机守卫之比 × 7763 的守卫加 2 秒。任一行过 15 秒，就与规则一层一起列给主持者与负责人，部署文档的硬件下限照他们定的写（修订核对 B 的 N18）。照今天的数第一行已过 15 秒（2.2，修订核对 B 的 M15）；
  - B6、B10 的窗口同样取，各列自己的文件；
  - 理由：判据按原始数判，被量的代码改了之前的数量的不是同一个东西（初稿与第一次修订的"往前补到 `86565dd6`、再补到上一次改动"在 P3 改了撤销之后说不通，修订核对 B 的 M13）；取最大只取得到碰巧采到的 7763，慢一截的那种在决定 359 的 38 次里只出现一次；守卫量的状态（无统计、不 `VACUUM`）不是最慢的。
- **469（P4-17）基准先只跑界线那一档，规则没过时再给降常量的候选**（3.6 的 B5；审查 B 的 M5）：候选先照 M8 的做法在两档之间线性插值，再以 S1 加的档位开关在插值点两边各补一档、跑两轮，给出"5 秒之内的最大一档"。理由：两轮整条曲线要 4 小时以上；写死的档太粗（附件 5,000 与 10,000 之间、版本 23.99 万行与 41.99 万行之间），会让降常量显得代价过大。
- **470（P4-18）规模开关只在手跑时用**（3.3；审查 A 的 I2、I3）：规模、请求期限、worker、写超时、客户端与判不判界由一个纯函数 `guardScaleOf` 给；开关没设时它给守卫今天的常量，守卫与它的短测试断言这些常量本身（不拿推出的值验它自己）；设了时用生产的期限（给了 `NW_SCALE_REQUEST` 时 worker 取请求的一半、不截 10 秒，这不是生产的公式，只为看到真的用时），客户端等请求期限的两倍，写超时在读超时加请求期限之上，每次计时之前断言这两条；只记、不判界。四个守卫的默认值是 `perf_scale_test.go`（不带构建标签）里返回字面的四个函数，守卫从那里取，并在计时之前断言自己的依赖（期限、客户端的 `Timeout`）等于它；计时的请求与随规模变大的请求（批准的守卫播种时的发起与提交在内）都经同一个帮手 `timedAsk`（修订核对 A 的 M11、M12）。理由：守卫的界是对着默认的规模定的，10 万、30 万页上一判就红，红了也说明不了什么；5 秒的测试客户端、15 秒的写超时与截到 10 秒的 worker 会先于被量的东西判（决定 359、05-lessons 第 1 节）。
- **471（P4-19）进程内的经 `pgtest`（`fsync=off`），对外的用不加 `fsync=off` 的容器；B4 的底数两种都量**（主持者定，审查 B 的 Q2；2.1 第 10 条，3.6 的 B4）：比来自 `fsync=off` 的进程内守卫，B4 的底数来自对外的 `serve`；批次 F 里同一个删除在一个加了 `fsync=off` 的容器上另量一次，推断写成"`fsync=off` 的本机数 × 比，加两种容器之差（不乘）"，标推断，两个数都记。理由：B5 与 B1 要与 M8 用同一种量法；对外的数进部署文档，要像生产；`fsync` 的那一份不随 CPU 变，不该被乘。
- **472（P4-20）knip 经 `e2e/package.json` 的脚本，配置写字符串的 `testMatch` 与绝对路径的 `globalSetup`**（2.1 第 7 条；审查 C 的 M1）。理由：一条脚本同时是跑法的说明；knip 6 的 Playwright 插件只收字符串的 glob、`globalSetup` 按 workspace 解析，这样写 knip 就读得到配置的入口。`knip.jsonc` 只为 B9 的构建多一节 `e2e` 的 `ignoreDependencies: ["vite"]`（4.4；修订核对 C 的 M6）。
- **473（P4-21）B2 与 B3 的负载**（主持者定 B3 取峰值；3.6；审查 A 的 M7、B 的 I3）：B2 每个并发是一个凭证，一半读 5 MiB 页的阅读视图、一半写 5 MiB 的正文；"没等满 `parse_max_wait` 就 503"是客户端从发出到答复头不到 2.0 秒的 503，不分是解析预算还是大请求体的名额（两个都等 2 秒）。B3 照 M9/P2 的形状：一个长字符串；`tools/call` 的 `_meta` 里 `valuesShapes()` 的三种值，在上限处与过一个；`/mcp` 加一个过 1 MiB 的 REST 写；两个名额加一个等着的。B3 取两轮六次里的最大，同 00 号文档 4.7.3 的"内存取两轮的较大峰值"。
- **474（P4-22）S3 的批次与时段**（3.8）：B（搜索）一夜；对照 B′ 有候选时另一夜；C 与 D 一夜；E 一个傍晚或一夜；F 一夜；每批整段持 heavy 的锁。S2 在一个傍晚，页面树先。
- **475（P4-23）lint 守卫的 worker 期限提到界之上：worker 6 秒、请求 12 秒，断言界低于期限**（主持者定，审查 B 的 Q3、A 的 I3；3.3）。只改这一个用例：升级的检查与发起照旧用 6 秒与 3 秒（`wikiCostRequest`、`wikiCostWorker`）。这是 M10 决定 160 留下的（摘要的墙钟界 5.2 秒在 3 秒的 worker 之上，轮不到它判），P4/S1 本来就改这个文件，照决定 359 的做法顺带修；守卫的牙齿从 3 秒的期限回到它自己写的 5.2 秒的界，7763 上摘要 1.72–2.01 秒，两者都远。总体设计 13.4 第 8 条的"请求 6 s、worker 3 s（决定 160）"由 S1 的文档提交改，第 15 节加一行（第 8 节第 5 条；修订核对 B 的 M11）。
- **476（P4-24）P4/S2 与 P4/S1 并行**（主持者定，审查 C 的 Q1；2.1 第 12 条，第 0 节末段）。
- **477（P4-25）语料的载入测试在 `!race` 的文件里，`test-go-costs-3` 单独一行带 `-v`**（2.1 第 8 条；审查 A 的 I1）。理由：它调的帮手只在 `!race` 下编译；`test-go-costs-3` 已经编译不带竞态的组合根（`Makefile:465`、`:481`、`:483`）。
- **452（P4-26）总体设计第 14 节的 `NOTIFY` 与 SSE 两行由 P5/S3 改**（主持者定，P5 审查 C 的 M7；第 8 节第 5 条）。P5 的设计先提交，这一条已是决定 452（P5-23），本文不另编号，不进 00 号文档第 11 节的 P4 一节。理由：两行都不是实测的事：`NOTIFY` 一行在升级 PostgreSQL 大版本时复核，SSE 一行写的是部署文档沿用的配置。
- **478（P4-27）批次 E 另量一组小机器档**（第 9 节，[P5 设计](05-P5-deploy-release.md) 3.18 与 2.1 第 21 条）：`page.parse_budget_bytes` 5 MiB 与 `GOMEMLIMIT=1536MiB` 下 B2 的 k = 8、两种正文各一次，只记。理由：部署文档要写小机器的内存；总体设计第 14 节写着"下限 5 MiB，约 1.5 GB"，量一组比推算可靠，约 10 分钟。

## 3. 服务端：设施与方法

### 3.1 S1 的文件

| 文件 | 内容 |
|---|---|
| `server/internal/bootstrap/perf_corpus_test.go`（新，`//go:build !race`） | 合成的搜索语料：`TestPerfSearchCorpus`（没有 `NW_PERF_CORPUS_OUT` 就跳过）写三个 TSV；`TestPerfVocabulary`（没有 `NW_PERF_VOCAB_FROM` 就跳过）生成词表；两个不连库的测试与一个 1,000 页的载入测试（3.2）。整个文件 `!race`：载入测试调的 `loadCorpus`、`buildSearch` 只在 `!race` 下编译（审查 A 的 I1） |
| `server/internal/bootstrap/testdata/perf-vocab-zh.txt`、`perf-vocab-en.txt`（新） | 词表：每行一个词与它在仓库的文字里出现的次数，按次数排；文件头写生成它的提交（3.2） |
| `server/internal/bootstrap/perf_scale_test.go`（新，不带构建标签） | 规模开关的纯函数 `guardScaleOf` 与判界的 `judge`、四个守卫的默认值（返回字面的四个函数）、计时的帮手 `timedAsk`、回收站基准的档位 `benchLevels`，与它们不连库的短测试（3.3） |
| `bootstrap/upgrade_start_cost_test.go`、`upgrade_check_cost_test.go`、`upgrade_approve_cost_test.go`、`wiki_lint_cost_test.go`、`wiki_lint_cost_fixture_test.go` | 从 `perf_scale_test.go` 的默认值经 `guardScaleOf` 取规模与期限，原来的规模与期限的常量不再用；计时之前断言自己的依赖等于它；计时的请求与随规模变大的请求经 `timedAsk`；判界经 `judge`；lint 这一个用例的 worker 6 秒、请求 12 秒（3.3；取舍 P4-23） |
| `bootstrap/trash_cost_test.go` | `BenchmarkTrashRestore` 读 `NERVE_BENCH_LOADS`、`NERVE_BENCH_ATTACHMENTS`，`BenchmarkTrashRestoreHistory` 读 `NERVE_BENCH_VERSIONS`，只在手跑时设（3.3 末） |
| `page/adapter/postgres/history_cost_test.go`、`page/adapter/postgres/perf_scale_test.go`（新，不带构建标签） | `newLongLog(t, n)`：变更集数与随它的几个数由 n 推出；n 由 `changesetsOf` 读 `NW_SCALE_CHANGESETS`（3.3） |
| `Makefile` | `test-go-costs-3` 在 `server/tools` 那一行之前加一行，跑语料的三个测试，带 `-v`（3.2；2.1 第 8 条） |
| `server/tools/perf/…`（新） | 起服务的脚本、采样、vault 的生成器、驱动、分位的计算（3.4） |

### 3.2 合成的搜索语料（B1）

**写什么**：`BenchmarkSearch` 读的三个 TSV，列与次序照 `corpusTables`（`bootstrap/search_bench_test.go:282-289`），`COPY` 的文本格式（反斜杠、制表符、换行照它转义）。

- **`notebooks.tsv`**：`N` 本笔记本，都在工作区 `…0002` 里，`created_by` 是 `…0001`（`:295`）。`corpusTables` 对它不写列名，`COPY` 取表的全部列，所以一行要恰好是被量的提交上 `notebooks` 的全部列；以后的迁移给它加列时，`test-go-costs-3` 里的载入测试先红。`NW_PERF_CORPUS_SCHEMA=308eafc6` 时只写那个提交上的列：少 00048 的 `mode`、`source_root_id` 与 00055 的 `upgrade_id`、`upgrade_state`、`upgrade_by_id`、`upgrade_at`（审查 A 的 M10、B 的 I2；`nodes`、`page_contents` 有列名，`308eafc6` 之后加的列都可空或有默认）。
- **`nodes.tsv`**：30 万页，2% 已删（同 M8 的语料活着 294,055 页），没有附件。每本笔记本里的页排成三层：根下的文件夹页、每个下面至多 100 页。守住应用的不变式（审查 A 的 M9）：
  - 名字带大写的拉丁字母与一个折叠不同于小写的字母（例如 `Σς`），`name_key` 由 `shared.TitleKey` 算（生成器直接调它），所以每页的 `name` 与 `name_key` 都不同；同级的 `name_key` 以序号保证唯一（两个唯一索引，`migrations/sql/00042_page_nodes_path_keys.sql:14-16`）；`sort_order` 在同级里递增；
  - 删除只删整个第二层的文件夹连同它的页、同一个时刻，或删叶子页（"A subtree's deletion sets one time on all of it"，`00012_page_nodes.sql` 的注释）；
  - id 由种子与序号导出（版本位照 v7 的写法），时间戳由一个固定的起点加序号导出，不调 `uuid.NewV7` 与 `time.Now`。
- **`contents.tsv`**：每页一行，`revision` 1，`content_hash` 与 `byte_size` 由应用自己的函数算（生成器调它），删了的页 `deleted_at` 同它的节点。正文的长度按对数正态抽，中位约 6 KiB、平均约 8 KiB（M8 的真语料平均 8.1 KB），千分之一是 200 KiB 的长页；六成的段落是中文、四成是英文。合计约 2.4 GB。
- **两种分布**：`NW_PERF_CORPUS_NOTEBOOKS=200` 与 `=5000`，页在笔记本之间照 Zipf（s = 1）分：Go 的 `NewZipf` 要 s > 1，生成器自己建累积表、二分查（审查 A 的 N1）；最大的一本占 `1/H_N`，5,000 本约 11%，200 本约 17%。同一个种子、只差笔记本数。

**词从哪来**（取舍 P4-4）：

- 不下载语料（`04-owner-decisions-and-preferences.md` 第 4 节）。词表取仓库自己的文字：中文取 `docs/` 下 Markdown 里的汉字串，按二字、三字切开计数；英文取 `docs/` 与 `server/`、`web/` 代码注释里的英文词，小写计数。
- 词表在 S1 里由 `TestPerfVocabulary` 生成一次，写进 `testdata`（每种至多 2 万个词），文件头写生成它的提交。之后的语料只读这两个文件，所以仓库的文档怎样改，同一个种子都出同一份语料。
- 段落里的词按词表的次数排名照 Zipf（s = 1）抽，标点与换行照固定的比例插。

**查询的词按 M8 的频率类确切地植入**（审查 A 的 M1）：`searchBenchQueries`（`:271-278`）的 45 个查询是对着维基的语料挑的：最常见的键（`的`、`the`、`of`、`a`、`ing`、`中国`）、走查用完的罕见词（`veress`、`ranters`、`latere`）、名称多而通过率低的（`A股`）、正文层没有键的（`锅 #`）。仓库的文字正好记着这些查询（M8 的设计、审查与注释），照词表随机抽，罕见类的 `veress` 在 30 万页里会落进约 1,540 页（审查 A 的探针）；搜索又按子串匹配（汉字的二元、词的三元加复核，`migrations/sql/00040_page_search.sql:18-21`），两个词相接也会拼出查询的词。所以：

- 生成器在源码里有一张表，每个查询的词一行：它的类、在 30 万页里的目标文档数、出现在标题里的比例。能对上 M8 记下的真实数的，照它：`A股` 406 页、`latency` 225、`Windows系统` 201、`Go语言` 12（[M8/P5](../M8-history-search/05-P5-search.md) 3.9"计数"一行）。其余分四类：重（至少 1.5 万页，即 5%：上面六个最常见的键，加 `数据库`、`索引`、`history`、`api`、`go`、`c`、`ai`、`linux`、`kernel`）；中（300–3,000 页）；轻（50–300 页）；罕见（1–20 页：`veress`、`ranters`、`latere`、`geass`、`犇`、`囧`）。每个词归哪一类写在表里，审查者对着 M8 的查询说明（同节"查询"一段）核对。
- **非重类的词不让它自然出现**：词表去掉折叠之后含任一非重类查询词为子串的词；每生成一段（与一个名字），折叠之后查它含不含非重类的词，含就以同一个随机源重抽这一段。
- **非重类的词确切地植入**：每个词以种子选定恰好目标数的活着的页，各放一次（中文的词两边是全角逗号，英文的两边是空格，不与邻字拼出新的键）；表里标题的比例那一份放进名字。生成完逐页再数一遍，与表不符就失败、什么也不写。
- **重类的词自然出现**，不够目标的页再补放，至少到目标。
- 多个词的查询（`中国 历史`、八个词的那一个）靠随机的共现，不另植入。

**测试**（都在 `perf_corpus_test.go`，跑在 `test-go-costs-3` 的那一行；前两个不连库，也在 `test-go-short` 里跑）：

- `TestPerfCorpusIsItsSeed`：同一个种子、同样的参数，1,000 页的三份 TSV 的 sha256 相同；换种子不同；`NW_PERF_CORPUS_SCHEMA=308eafc6` 的 `notebooks.tsv` 每行恰好少上面那六列，其余两份相同。
- `TestPerfCorpusPlantsTheQueries`：3,000 页（缩小一百倍，目标照比例、至少 1）上，每个非重类的词在正文与名字里的文档数各等于表的目标；重类的至少到目标；两份词表里没有一个词含非重类的查询词。
- `TestPerfCorpusLoadsIntoTheBenchmark`：1,000 页的语料经 `loadCorpus`（`:293`）载入 `pgtest` 的库、`buildSearch`（`:365`）建完，`中国` 一个查询有结果；每页的 `name_key` 等于 `shared.TitleKey(name)` 而不等于 `name`；每个删了的节点，它下面的都删了、`deleted_at` 相同（一句 SQL）；`-short` 下经 `pgtest.NewDatabase` 跳过（`pgtest.go:88`）。
- 用时：S1 量本机与分支 CI 的数写进记录（审查 A 的 M2：照初稿的 3 万页加 ±20%，竞态下本机约 11 秒；改成确切的数之后几千页够了，推断本机三个合计 10 秒以内，不带竞态）。

**跑法**（S3 的批次 B）：`NW_PERF_CORPUS_OUT=<目录> NW_PERF_CORPUS_NOTEBOOKS=200 NW_PERF_SEED=20261116 go test -count=1 -run '^TestPerfSearchCorpus$' -timeout 30m ./internal/bootstrap/`，5,000 本同样；对照要的另加 `NW_PERF_CORPUS_SCHEMA=308eafc6` 写到另一个目录（只有 `notebooks.tsv` 不同）。目录在 `~/.cache/m12-p4/`。估 10–20 分钟一份（推断：写 2.4 GB 的文字）。

### 3.3 现有守卫的规模开关

照 00 号文档 4.7.4 与取舍 P4-18、P4-23。帮手在 `bootstrap/perf_scale_test.go`（不带构建标签：守卫是 `!race`，帮手的测试要在竞态的第四份里也跑）：

```go
// guardScale is what a cost guard runs at (M12/P4): its size, its
// deadlines and its client's, and whether it judges its bounds.
type guardScale struct {
	Size            int // pages, sources or changesets
	ByHand          bool
	Request, Own    time.Duration // the app's request deadline; the guarded call's (lint's worker, the start's, the approval's)
	Read, Write     time.Duration
	Client          time.Duration // the timed request's client's
}

// guardScaleOf is def, a guard's own, unless sizeVar (NW_SCALE_PAGES)
// gives a size by hand: then production's request deadline, or
// NW_SCALE_REQUEST's, the call's own deadline by production's formula
// (lint's worker request/2, uncapped under NW_SCALE_REQUEST), a client
// that outwaits it and a write timeout past the read and the request
// (00-M12 decision 359).
func guardScaleOf(getenv func(string) string, sizeVar string, def guardScale) (guardScale, error)

// judge reports value over bound through errorf, unless s runs by hand.
func (s guardScale) judge(errorf func(string, ...any), name string, value, bound uint64)
```

- **纯函数**：读环境变量经传进来的 `getenv`，坏值返回错误、守卫 `t.Fatal`；不需要假的 `testing.TB`（审查 A 的 N2）。
- **开关没设时，它给的就是守卫今天的常量**（审查 A 的 I3；P4-23 改了 lint 的两个）。这张表写在 `perf_scale_test.go` 里，是返回字面的四个函数 `startCostScale()`、`checkCostScale()`、`approveCostScale()`、`lintCostScale()`（不用包级的 `var`：测试文件也受 `gochecknoglobals` 约束，组合根的测试今天没有包级的 `var`）。今天这些数是 `!race` 文件里的常量（`wikiCostRequest` 等，`wiki_lint_cost_test.go:31-34`），不带标签的测试一引用它们，竞态的编译就报 `undefined: wikiCostRequest`（修订核对 A 的 M11，探针）。所以反过来：守卫从这四个函数取默认值，原来那几个规模与期限的常量（`startCostPages`、`startCostDeadline`、`checkCostPages`、`approveCostFolders`、`approveCostRequest`、`approveCostDeadline`、`wikiCostRequest`、`wikiCostWorker`）改为从 `guardScale` 取，别处不用的删掉，不留两份数：

  | 守卫 | 规模 | 请求 | 被守的调用的期限 | 读、写超时 | 客户端 |
  |---|---|---|---|---|---|
  | 发起（`upgrade_start_cost_test.go`） | 50,000 页，来源区 15,000，另九本各 50,000 页 | 6 秒 | 4.5 秒（`d.StartTimeout`） | 5、15 秒 | 12 秒 |
  | 检查（`upgrade_check_cost_test.go`） | 50,000 页 | 6 秒 | 3 秒（worker） | 进程内调用 | — |
  | 批准（`upgrade_approve_cost_test.go`） | 47,500 个来源（500 个文件夹 × 95） | 15 秒 | 11.25 秒 | 5、45 秒 | 30 秒 |
  | lint（`wiki_lint_cost_test.go`） | 100,000 页 | 12 秒 | 6 秒（worker） | 进程内调用 | — |

  - 短测试 `TestGuardScaleKeepsEachGuardsOwnUnlessGiven` 以空的环境逐格断言这四个函数的字面的数，不由别的常量推（05-lessons 第 1 节"期限要由界推出，再与起应用的配置对证，不要拿配置去验它自己"）；
  - **守卫一侧接对了没有，由守卫自己在计时之前断言**（开关设不设都断言；修订核对 A 的 M11）：实际的依赖等于 `guardScale` 的值：发起的 `d.StartTimeout`（今天 `upgrade_start_cost_test.go:115` 对 `startCostDeadline` 断言）、批准的 `d.ApproveTimeout`（`upgrade_approve_cost_test.go:359`）、lint 与检查的夹具设下的 `d.LintTimeout`（`wiki_lint_cost_fixture_test.go:53`；夹具设了之后读回来断言）、进程内调用的 `context.WithTimeout` 用的请求期限（`upgrade_check_cost_test.go:142`、`wiki_lint_cost_test.go:183`），与 `timedAsk` 的客户端的 `Timeout`。这样守卫一侧接错时守卫自己红（例如 lint 的守卫不用表里的 worker 而用 `min(10 秒, …)`：短测试照绿，"界低于期限"5.2 < 7.5 < 15 也照过，只有这条断言抓得到）；
  - **客户端一格是新的**：计时的请求不再经 `client()`（5 秒），改经帮手 `timedAsk(t, s, contract, method, url, token, body)`（在 `perf_scale_test.go`，不带标签）：以 `&http.Client{Timeout: s.Client}` 经 `sendRequestBy`（`contract_test.go:385`）发、照契约核对答复；发之前断言客户端的 `Timeout` 等于 `s.Client`、大于 `s.Own`；返回状态、答复与用时。同 `source_cost_test.go:87` 的 `2 * 请求`。开关没设时界在期限之下、先判，不改通过与否；批准今天以 5 秒的客户端等 11.25 秒的期限，界 3.9 秒在 5 秒之下所以没事（审查 A 的 I2）；
  - **随规模变大的请求也经 `timedAsk`**（修订核对 A 的 M12）：批准的守卫播种时经 `ask` 发起升级、再提交（`upgrade_approve_cost_test.go:239`、`:247`），两者都随来源数变大（本机 5 万页时发起 1.11–1.17 秒，`upgrade_start_cost_test.go:56-67` 的注释），B7 的 30 万推断会过 5 秒，守卫会在量之前以 `Client.Timeout exceeded` 失败。它们改经 `timedAsk`，用时记进日志、不判。工作量不随 n 变的请求（建工作区、建笔记本、建 `raw` 页，`upgrade_start_cost_test.go:220-230`、`upgrade_approve_cost_test.go:217-223`）照旧经 `ask`。
- **lint 这一个用例的期限**（取舍 P4-23）：检查照旧是请求 6 秒、worker 3 秒，发起照旧是请求 6 秒、期限 4.5 秒（`checkCostScale()`、`startCostScale()`）；lint 的守卫的 12 秒与 6 秒只写在 `lintCostScale()` 里，夹具 `newWikiCostFixtureOf` 多一个 worker 的参数（`wiki_lint_cost_fixture_test.go:49-53`），量的那一处的 `context.WithTimeout` 用 `s.Request`（`wiki_lint_cost_test.go:183`）。守卫在量之前断言摘要与 `wanted` 的墙钟界（5.2、1.4 秒，`:122-123`）都低于 worker、worker 低于请求。总体设计 13.4 第 8 条写着这个用例"请求 6 s、worker 3 s（决定 160）"，S1 的文档提交照改、第 15 节加一行（第 8 节第 5 条；修订核对 B 的 M11）。
- **设了 `NW_SCALE_PAGES` 时**：
  - 规模是它；请求是 `NW_SCALE_REQUEST`（Go 的时长）或生产的 15 秒；被守的调用的期限照生产的公式：发起与批准 `请求*3/4`（`bootstrap/wire.go:281`、`:297`），lint 与检查的 worker `min(10 秒, 请求/2)`（`:253`），给了 `NW_SCALE_REQUEST` 时取 `请求/2`、不截 10 秒（这不是生产的公式，只为看到 10 秒之外真的用时，记录写明）；读超时照测试的配置（5 秒），写超时取读加请求再加 15 秒（生产的校验要 `read + request < write`，`platform/config/validate.go:50-58`）；客户端等请求的两倍；
  - 每次计时之前断言"客户端 > 被守的调用的期限"与"写超时 > 读 + 请求"（开关没设时也断言）；
  - `judge` 只记、不判时间与内存的界；正确性的断言照旧，由规模推出（发起的 `:285-300` 照 n 与来源区的比例推）；
  - 日志的行照旧，另加 `scale=<n>`。
- **四个守卫怎样按规模缩放**：
  - 发起：被量的笔记本 `startCostPages`（`:42`）换成 n，来源区照原比例（`startCostArea`/`startCostPages` = 0.3，`:43`）；另九本仍各 50,000 页：今天它们用同一个常量（`:262`），另起一个常量 `startCostOtherPages`（审查 A 的 N4）；
  - 检查：被量的那一本 `checkCostPages`（`upgrade_check_cost_test.go:21`）换成 n，另几本同样另起常量（`:76`）；
  - 批准：来源数 `approveCostSources = approveCostFolders * (1 + approveCostPages + approveCostFiles)`（`:46`），文件夹数换成 `round(n/95)`，每个文件夹的页与文件不变；
  - lint：`newWikiCostFixtureOf(t, …, n, worker)`。
- **按页的日志**：`page/adapter/postgres/history_cost_test.go:42` 的 `newLongLog(t, n)`：n 由 `changesetsOf`（`page/adapter/postgres/perf_scale_test.go`，同样的写法十几行，不跨包导入测试代码）读 `NW_SCALE_CHANGESETS`，默认 100,000，要是 5 的倍数、至少 1,000。随 n 推的（审查 A 的 N3）：`generate_series(1, n)`、长历史那一页的 `revision = n/5`、大变更集是第 `n − 50` 个（今天的 99,950）、条目数 `50_000 + n/5 + (3n/5 − 1)`；用它的两个测试（`TestTheLogReadsInTimeAsLongAsAPage` `:161`、`TestTheRevertsArrayReadsInTimeAtTheUnitsBound` `:296`）在设了时只记。帮手的文件不带标签，`history_cost_test.go` 仍是 `!race`；帮手的测试在 `test-go-race` 一份与 `test-go-short` 里跑。
- **回收站两个基准的档位开关**（审查 B 的 M4 第 3 点、M5；取舍 P4-17）：`benchLevels(getenv, name, def)` 读逗号分开的列表，同 `manyTops`（`revert_cost_test.go:483-485`）的写法，坏值返回错误。`BenchmarkTrashRestore` 读 `NERVE_BENCH_LOADS`（`light`、`full`）与 `NERVE_BENCH_ATTACHMENTS`，`BenchmarkTrashRestoreHistory` 读 `NERVE_BENCH_VERSIONS`（递增；版本照旧一路累加）。只设一档时版本一次写到那一档：例如只设 `1000`，版本从 1 一次写到 1,000；整条曲线是 250、500、1,000 累加着写，`versions`（`trash_cost_test.go:383`）的最后一步把页当时约 82 KiB 的正文写进每一档的最新版本，所以第 250、500 两个版本留着大的正文，两种写法差每页两行、共约 200 行，量级多半小；记录写明用的是哪一种（修订核对 B 的 N12）。设了时每一档之前的建库与建单元只做选中的那几档：今天只选 `b.Run` 的子项，`b.Run` 之外的准备十档都做（`trash_cost_test.go:308-316`），省不掉。
- **测试**：`TestGuardScaleKeepsEachGuardsOwnUnlessGiven`（没设时是上表；设了是推出的值、`ByHand` 为真；坏值报错；`judge` 没设时对过界的值调 `errorf`、设了不调）、`TestLevelsOfTheTrashBenchmarksAreTheirOwnUnlessGiven`，在组合根的第四份与 `test-go-short`（G、L 起头，第一、二、三、五至八份的模式都不收；初稿的 `TestBenchLevels…` 以 B 起头，落在第一份的 `^Test[A-D]`，修订核对 A 的 N7）；`changesetsOf` 的一个在 page 的适配器。都不连库，毫秒级。
- **手工的反向对照**（S1 的记录，审查 A 的 I3）：开关没设时把发起的守卫的界临时压到 1 ms，守卫红；设了 `NW_SCALE_PAGES=20000` 时同样的改动守卫绿；改动复原。

### 3.4 `server/tools/perf/`

Go 只用标准库（取舍 P4-6）；`server/.golangci.yml` 的规则照常（`gochecknoglobals`、`depguard` 不许 `log` 与 MCP 的 SDK，`:19-45`）。读库（`pg_stat_*`、队列、WAL 的位置、表的大小）经 `os/exec` 调 `docker exec <容器> psql -XAt -v ON_ERROR_STOP=1 -c '<SQL>'`，执行器可注入（审查 A 的 M6）。每个包有不连网络、不起 Docker 的单元测试，`test-go-costs-3` 的 `go -C server/tools test -race -count=1 ./...` 跑它们（`Makefile:498`）。

```
server/tools/perf/
  doc.go          包的说明：各子命令、跑法、与本文的对应
  stack.sh        起与停一个生产配置的 serve 与它的 PostgreSQL 容器
  main.go         子命令：sample、gctrace、vault、bodies、mcp、editors、tabs、ops、transfer
  sample/         /proc 与 cgroup 的采样；gctrace 的解读
  stats/          最近秩的分位、中位、两轮的合并
  vault/          导入用的 vault 的生成器
  client/         最小的 REST 客户端、MCP 的 JSON-RPC、经 docker exec 的 psql
  drive/          五个驱动
```

#### 3.4.1 `stack.sh`：一个生产配置的 serve

照 `deploy/image-smoke.sh` 的写法，`set -u`，每一步判返回码（"有副作用的命令链要看返回码"）；只停、只删自己起的进程与容器（按 PID 与容器 id）。

- `stack.sh up <目录> [--fsync-off] [--env KEY=VALUE …]`：
  1. `docker run -d` 一个 `postgres:18.7-trixie`，`POSTGRES_INITDB_ARGS='--locale-provider=builtin --locale=C.UTF-8'`，端口只绑 `127.0.0.1`；默认**不加** `fsync=off`（取舍 P4-19），`--fsync-off` 时加 `-c fsync=off`（只给 B4 的对照，3.6）；等 `pg_isready`。
  2. `openssl genpkey -algorithm ed25519` 生成 JWT 的私钥（生产必需，`platform/config/validate.go:178-181`）。
  3. `NWIKI_ENV=prod nervewiki migrate up`（生产不自动迁移，`server/configs/config.prod.yaml:1-4`）。
  4. `nervewiki users create --email perf@example.com`，密码一行经标准输入（`deploy/image-smoke.sh:142-144`）。
  5. 起 `nervewiki serve`：`NWIKI_ENV=prod`、`NWIKI_SERVER__ADDR=127.0.0.1:0` 与 `NWIKI_SERVER__ADDR_FILE`、`NWIKI_STORAGE__DIR=<目录>/data`、`GODEBUG=gctrace=1`、取舍 P4-8 的三个桶（`NWIKI_RATELIMIT__PASSWORD_USER__PER_MINUTE=600` 等）、`--env` 给的（例如取舍 P4-27 的 `NWIKI_PAGE__PARSE_BUDGET_BYTES` 与 `GOMEMLIMIT`）；标准输出与标准错误进 `<目录>/serve.log`（探针 (2)：JSON 日志与 gctrace 都在标准错误）。
  6. 等 `/readyz` 答 200，再等日志里 `"msg":"notification listener listening"` 与 `"msg":"jobs started"` 两行：生产的日志是 JSON（`config.yaml:159-161`），e2e 等的是文本的 `msg="…"`（`e2e/fixtures/server.ts:170-171`），照抄等不到（审查 A 的 M8）。
  7. 写 `<目录>/stack.env`：地址、serve 的 PID、容器 id、库的地址、`nervewiki version` 的一行、`SELECT version()`、有没有 `fsync=off` 与 `--env`。
- `stack.sh restart <目录>`：停 serve（SIGTERM，等它退出）、`docker restart <id>`、等就绪、再起 serve。数据留着。用于保留数据的第二轮（批次 E）。
- `stack.sh down <目录>`：停 serve、`docker stop` 并 `docker rm` 这个 id。
- 二进制是被量的提交上 `make build` 出的 `bin/nervewiki`（在那个提交的 worktree 里）。

#### 3.4.2 `sample` 与 `gctrace`：内存与 CPU

- `perf sample -pid <serve> -container <id> -every 100ms -out <文件>`：每 100 ms 写一行 CSV：时刻（Unix 纳秒）、`VmRSS`、`VmHWM`（`/proc/<pid>/status`，kB 乘 1024）、`utime`、`stime`（`/proc/<pid>/stat` 第 14、15 个字段，除以起来时经 `getconf CLK_TCK` 读一次的值，读不到就失败）、容器的 `memory.current` 与 `cpu.stat` 的 `usage_usec`（`/sys/fs/cgroup/system.slice/docker-<id>.scope/`：systemd 的 cgroup 驱动下的路径，探针 (1) 核过本机用户读得到；找不到就失败，不读成零，审查 A 的 N5）。起来时记下进程开始的墙上时刻：当下的墙钟减去（`/proc/uptime` 减 `/proc/<pid>/stat` 第 22 个字段除以 `CLK_TCK`），精到 0.01 秒（`/proc/stat` 的 `btime` 只到整秒，审查 A 的 M5）。收到 SIGINT 时另写一行终值：`memory.peak`、`memory.stat` 的 `anon`、`file`、`shmem`。
- `perf gctrace -log <serve.log> -start <进程开始的时刻> -base <时刻> -from <时刻> -to <时刻>`：读 `gc N @Ts …, A->B->C MB, D MB goal, …` 的行（探针 (2) 的格式；单位是 MiB，运行时按 `1<<20` 除；`@Ts` 是这次 GC 开始距进程开始的秒数），换成墙上的时刻。
  - **被插断的行**（审查 A 的 M4）：运行时把一行 gctrace 分三十来次 `write(2)` 写出（`runtime/mgc.go:1592-1631`、`runtime/print.go:89-107`），serve 每个请求的 JSON 访问日志会插进行中间（审查 A 的探针：每秒 200 行日志时 0.5% 的行、2,000 行时 4.9%）。解读把一行里嵌着的整个 `{…}`（以 `{"time":` 起、到配对的 `}` 与换行）去掉，再把断开的两半拼回；从基线到窗口末尾，`gc N` 的序号连续，缺号就报出（这一组合重量），不悄悄跳过。
  - **基线**：每轮一个，取 `-base` 那一行的 `C`（存活堆）：服务起来之后空闲至少 130 秒，运行时两分钟没有 GC 就强制一次，打出的那一行（探针 (2)：约 120 秒时存活堆 3 MB）。各组合都对它算增量：组合之间的那次 GC 含着在途的请求，拿它作基线会偏高、增量偏低，最坏是负数（审查 A 的 M5）；用一轮开头的这一个偏保守，记录写明。
  - **峰值**：`[from, to]` 里每行 `max(A, B)` 的最大。
  - **末尾的上界**：窗口里最后一次 GC 之后，堆至多涨到下一次的目标就会触发 GC；上界取 `max(峰值, 窗口之后第一行的 D)`。上界跨过界线时，这一组合单独重量一次，之后空闲 130 秒，让强制 GC 那一行的 `A` 记下末尾（审查 A 的 M5）。
  - **输出**：基线、峰值、增量 = 峰值 − 基线、上界与它的增量、窗口里 `D` 的最大与 GC 的次数、缺号。
- **测试**：`gctrace` 的解读对一份写死的日志：结束时的堆大于开始时的那一行取到结束的；一行被一条 JSON 日志插断，拼回之后读到它的 `A->B->C`；中间缺一个序号时报出；基线取 `-base` 那一行；上界取窗口之后第一行的 `D`；只有 JSON 的行跳过；`@Ts` 的换算。`sample` 对一个临时目录里写的假 `/proc` 与假 cgroup 文件（路径可注入）：kB 的换算、`CLK_TCK`、由 `uptime` 换算的开始时刻、终值读 `memory.peak` 与 `memory.stat`、cgroup 的目录不在时报错。

#### 3.4.3 `vault`：导入用的 vault

- `perf vault -out <zip> -pages P -attachments A -fanout 10,10,98 -links L -body 2KiB -attachment-bytes B -random R -seed S`：写一个 zip（`archive/zip`，deflate 的默认级别），每页一个 `.md`，每个附件一个文件，文件夹是路径里的目录（导入把它们建成有孩子的页，`config.yaml:153` 的注释"条目加路径隐含的文件夹"）。页名唯一（`p<编号> <两个词>`），链接写不带路径的 `[[页名]]`：九成指向同一个第二层文件夹之下的页，一成在整本里均匀抽；每页 L 条。附件的内容 1/R 是随机的、其余是同一块 4 KiB 的重复。
- **上限**（`config.yaml:151-157`；`bootstrap/deps.go:253`）：写之前核对条目 ≤ 5 万、条目加文件夹 ≤ 6 万、解压 ≤ 4 GiB，超了就拒绝、什么也不写；zip 的大小写完才知道，所以先写进同目录的临时文件，写完核对 ≤ 512 MiB，超了删掉、失败（审查 A 的 M3）。写完打出条目数、文件夹数、解压与压缩后的字节、sha256。
- **三种 vault**：
  - **导入上限**（批次 F 与 B4）：45,000 页、5,000 个附件（十分之一），`fanout 10,10,98`（10 + 100 + 9,800 = 9,910 个文件夹，节点 59,910），每页 30 条链接、2 KiB 正文，附件平均约 800 KiB、`-random 12`。推算：解压约 3.90 GiB（附件约 3.81 GiB、页约 88 MiB，修订核对 A 的 N8）；deflate 对重复的部分也要约 1.3%（匹配至多 258 字节），附件压到约 9.5%、约 371 MiB，页压到约 61%、约 54 MiB，加头约 8 MiB，zip 约 433 MiB（照初稿的 `compressible 8` 与 820 KiB 是约 607 MiB，过了上限，审查 A 的 M3 算过）。真的图片不可压缩：同样 512 MiB 的包解压之后只有约 512 MiB；这一形状为了让条目、节点与解压三个上限同时接近。
  - **5 万页**（批次 E 的 10 万页树，导两次）：50,000 页、没有附件、`fanout 10,50`（10 + 500 = 510 个文件夹，节点 50,510），每页 10 条链接、1 KiB 正文。
  - S2 的三种形状不用它（4.2：TypeScript 的 `zipOf` 够用）。
- **测试**：同一个种子同样的参数 sha256 相同；缩小一百倍的导入上限形状核对条目、文件夹、解压与 zip 的字节都在缩小一百倍的上限之内、与表里的数相符；超了上限时拒绝（条目的写之前，zip 的写之后删掉临时文件）；每条链接的目标都是 vault 里的页，链接里没有 `/`，九成在同一个第二层文件夹之下（审查 A 的 N6）。

#### 3.4.4 驱动的公共部分：账户、凭证与记录

- **准备**（每轮、`stack.sh up` 之后）：`perf` 以 `users create` 的账户登录，建工作区 `perf`（`POST /api/v0/workspaces`，`openapi.yaml:348`）与要的笔记本，建要的 PAT（`POST /api/v0/me/api-tokens`，`:261`，要当前密码）。每个读者、写者、编辑者、标签页、agent 一个 PAT（取舍 P4-8）。PAT 写进 `<目录>/pats.json`，下一轮重建。
- **一次请求的记录**：一行 JSON，`op`、凭证的序号、发出与答复头、答复读完的时刻（Unix 纳秒）、状态、`code`（problem+json 的）、答复的字节数。
- **分位**：`stats` 的最近秩，与 `nearestRank`（`bootstrap/search_bench_test.go:469`）同一种定义，测试对一张写死的表核对 p50、p99 与只有一个、两个样本时的取值。

#### 3.4.5 五个驱动

| 子命令 | 量的行 | 做什么 |
|---|---|---|
| `bodies` | B2 | k 个凭证各一页 5 MiB（普通或病态）的正文，一半读阅读视图（`GET /api/v0/pages/{id}/view`，`openapi.yaml:1237`）、一半写（开编辑会话 `:1167`，`PUT …/content` 带 `base_revision` 与 `edit_session_id`，`:1128`），各自循环 |
| `mcp` | B3 | 3.6 的 B3 的各种形状，三个凭证同时发、两个名额加一个等着的；JSON-RPC 的 `tools/call` 手写（`/mcp?notebook=<id>`），请求头照 `bootstrap/mcp_bodies_test.go:122-125`：`Content-Type: application/json`、`Accept: application/json, text/event-stream`、`Authorization`、`Mcp-Protocol-Version` |
| `editors` | M4-performance 第 1、6 条，部署文档的内存，批次 G | N 个编辑者各一页 10 KiB、20 条链接，开编辑会话、每 2 秒改一个字写一次（同网页的自动保存，`W/editor/autosave.ts:4`）、每 20 秒续一次（`W/stores/edit-session.ts:18`）；每个编辑者每 60 秒关掉会话、再开一个（一个版本是一次编辑会话，`page/adapter/postgres/queries/changesets.sql:53-66` 以 `(changeset_id, node_id)` 就地更新）；一个树写者每秒新建、改名、移动轮着做一次；监视经 `psql` 每秒读 `pg_stat_slru`、`pg_stat_activity` 的等待；开始之前与结束之后各 `VACUUM FULL page_revisions` 一次，读 `pg_total_relation_size('page_revisions')`（表、TOAST 与索引）与行数（修订核对 B 的 M12） |
| `tabs` | 4.7.3 的 10 万页的树重读 | k 个标签页各开一条事件流（`GET /api/v0/events`，`:2340`），收到这本笔记本 `tree: true` 的 `pages` 事件就照 refresher 读整棵树：第一次立即读，500 ms 之内再来的合成末尾的一次（`W/events/handlers.ts:36`、`W/events/refresher.ts:59-77`），读完整个答复、记字节；一个 agent 每秒新建一页，60 秒。refresher 在读开始时记时刻（`W/events/refresher.ts:79-88`），上一次读还没完、节流已到期时照样发下一次，驱动同样不等上一次读完；网页的 SWR 是否真的并发发出，S2 在浏览器里核对一次（审查 A 的 U1） |
| `ops` | B4 | 在导入的 vault 上删、移、改名；删之后经后台的恢复（`POST …/trash-restores`，`:2670`）等它成功，再经 `psql` 等 `page_search_queue` 为空、River 没有 `running` 与 `available` 的任务，再空闲 30 秒，才下一次（审查 B 的 M8） |
| `transfer` | B8 | 上传 vault 开始导入（`POST …/imports`，`:2623`，multipart），每 2 秒问任务（`:2778`）到成功；导出（`:2591`）同样，再下载（`:2824`）；前后经 `psql` 读 WAL 的位置（`pg_current_wal_lsn()`）、`pg_database_size`，读存储目录与它的 `imports/`、`exports/` 的大小 |

- **病态的正文**：`platform/markdown/markdowntest` 的 `Pathological()` 的写法放大到 5 MiB（M4/P4 审查 P1 的 `a*_`），驱动照抄这个写法（不跨模块导入），S3 的记录用一次单独的阅读核对它是病态的：堆的增量应在 1.2 GiB 以上（M4 量过 +1,464 MiB）。
- **测试**：每个驱动对一个 `httptest` 的假服务端：发的路径、头与体对；`mcp` 的每种形状按 bodyshape 的数法（请求体本身、每个对象、数组、字符串、数字、字面量各算一个值）数出的值的个数等于设计的数（审查 A 的 M7）；`tabs` 的节流用一个假的时钟核对"第一次立即、500 ms 之内合成一次"；`bodies` 的"早答的 503"按客户端的用时 < 2.0 秒判，假服务端立刻答 503 时数到、2.1 秒答时不数；`ops` 在假的 `psql` 答队列不空时不发下一次删除；`psql` 的执行器对一个假的命令核对参数与解析。

#### 3.4.6 为什么驱动不进 e2e 或组合根

- 组合根的 `_test.go` 只能在进程里量，量不到一个真的 `serve` 进程的内存与 CPU，也不经回环。
- e2e（TypeScript）能打 HTTP，但 B2–B4、B8 要的是长时间的并发与采样，与浏览器无关；Go 的驱动与采样在同一个模块里，单元测试也简单。
- S2 的页面树必须在浏览器里，所以只有它在 `e2e/perf/`（第 4 节）。

### 3.5 量的公共规则

#### 3.5.1 量之前

- 记 `nproc`、`free -h`、`df -h /`、`uptime`、`docker info` 的 CPU 与内存、`go version`、`nervewiki version`、PostgreSQL 的 `version()`、被量的提交。交换已满时照记（I2 第 1.1 节），不清。
- 整个批次经一次 `docs/v0.1/claude_to_codex/tools/heavy.sh` 持锁，其间不跑别的构建与测试（00 号文档 4.7.3）。
- `TMPDIR`、`GOTMPDIR` 放 `~/.cache/m12-p4/tmp`；大的输出写完 gzip；只删自己的东西（06-environment 第 3 节）。
- 批次 B 之前 `df -h /` 可用要至少 35 GB：不够先经 `heavy.sh` 修剪 12 小时没用过的 Go 构建缓存（06-environment 第 3 节的命令）；仍不够就停下报主持者。

#### 3.5.2 轮、重复与统计量

- **两轮**（00 号文档 4.7.3）：
  - 进程内的基准与守卫：一轮是一次 `go test` 的运行，起一个新的 testcontainers 容器（取舍 P4-1）；
  - 对外的：一轮是一次 `stack.sh up … down`（批次 F，新的库）或一次 `stack.sh restart`（批次 E，保留数据）；
  - 浏览器：一轮是一次 `pnpm perf`，起新的库与服务（4.2）。
- **统计量**：时延报每轮的 p50、p99，对照预算取较差的一轮；时长取每轮重复的中位数，两轮的较大中位数；内存取两轮的较大峰值（增量），B3 是两轮六次里的最大。B5 的规则一层照 M8 的量法（取舍 P4-1）：回收站与撤销的基准每轮是三次的均值，后台的每轮一个样本，两轮取较大。
- **种子**：每批一个固定的种子，写在 3.6 的表与记录里；生成器同一个种子出同样的数据（3.2、3.4.3 的测试）。

#### 3.5.3 判在哪台机器上

照 00 号文档 4.7.1 的"判在"一列与决定 318（P4-2 改了它的倍数一半）：

- **本机的数**：内存（B2、B3）、浏览器（B9、页面树）、B1、B5 的规则一层。
- **有 CI 上同一规模的实数时由实数定**：B5 的硬期限（`guards-core` 的三个守卫）、B6 的 10 万页（`costs-lint`）、B10 的 `index`（`costs-1`）；实数只取被量的代码没再改过的那一段里的（取舍 P4-16）。
- **没有实数时推断**，写明"推断"：B4、B7 的 10 万页、B8。推断用 3.5.4 的比（取舍 P4-2），对硬期限的另并列 × 1.25 的一行；00 号文档原来的 2.33、4.6 的结论也写出，与实测之比的结论不同时列给主持者。

#### 3.5.4 本机对 EPYC 7763 的比

- **两族**（取舍 P4-2；审查 B 的过度设计：来源、lint、`index` 三族只喂 B8，B8 离 6 小时远，不量）：
  - 回收站与撤销族：`TestARestoreAtTheUnitsBoundTakesItsTime`、`TestARevertAtTheUnitsBoundTakesItsTime`、`TestARevertOfManyTopsAtItsBoundTakesItsTime`（`guards-core`，`Makefile:551`、`:553`），各取它日志里的那一个数；
  - 升级族：发起、检查、批准三个守卫（`costs-2` 的 `:455`、`:460`，`costs-1` 的 `:437`），各取它日志里各状态与计划模式的中位数里最大的那一格。
- **7763 的数**：从 GitHub 只读地取被量的提交及之前 `main` 的这几个作业的日志，挑出 CPU 是 EPYC 7763 的（作业的 "Runner CPU" 一步打出 `model name`，`ci.yml:167-168`；同一次运行的各作业落在不同的 CPU 上，按作业挑）；**每个作业取最近的至多三次**（修订核对 B 的 M14）。比的两边是同一个提交上的数，其间代码改了什么不碍事，所以这里不像 P4-16 那样从最后一次改动算起。
- **本机的数**（审查 B 的 M1 第 4 点）：在上面每一次 7763 的运行的提交上，本机只跑那个作业的那一族一次（`Makefile` 的那几行，经 `heavy.sh`；同一个提交上两个作业属同一族时只跑一次）；另在被量的提交上两族各跑一次（B5 的推断行与 P4-16 的"一次也没有"用它）。至多九个提交加被量的那一个，每个约 10–15 分钟（3.8 的 C）。这样比的两边是同一份代码，不管其间改了什么（P3 改撤销的规划）。
- **怎样算**：每个守卫，7763 上各次的中位，除以本机在同一批提交上各次的中位；一族取各守卫的比的最大。两边都是中位（审查 B 的 M1 第 1、2 点：初稿来源族的"约 2.3"是 7763 上最慢样本的中位除以本机全部样本的平均，同口径是 1.77–1.95，删掉）。
- **今天的数**（设计时的探针，只作预告；S3 照上面重算，取的是那时最近的三次）：

  | 运行 | CPU | 恢复 | 撤销一次删除 | 撤销 1,500 个最上层 |
  |---|---|---|---|---|
  | 本机（探针 (3)，一次，`86565dd6`） | aarch64，8 vCPU | 5.46 秒 | 5.50 秒 | 1.54 秒 |
  | 43628067241（`179a7aed`） | EPYC 7763 | 10.99 秒 | 11.48 秒 | 2.68 秒 |
  | 43732360029（`acb2948e`） | EPYC 7763 | 11.02 秒 | 11.35 秒 | 2.78 秒 |
  | 其余九次 | Xeon 8573C、6975P-C，EPYC 9V74、9V45 | 6.94–10.59 秒 | 7.07–10.38 秒 | 1.50–2.55 秒 |

  回收站与撤销族的比约 2.0–2.1（撤销很多最上层 1.7–1.8）。
- **用到哪**：B4 用回收站与撤销族的比（同是持着笔记本行的树写的单元）；B7 用升级族的比；B6、B10 的 `index` 没有 7763 的实数时也用升级族的；B8 用两族的较大（6 小时之下乘多少都不改结论，00 号文档 A-N11）。

#### 3.5.5 每一行的记录

照 00 号文档 9.4：机器与版本与提交（3.5.1）；规模、形状、种子、命令；两轮的数与取哪一轮；判在哪台机器上（本机、CI 的实数并列出运行号与作业号，或乘了哪个比并写"推断"）；对照的预算与结论（过、超）；超出的去处。原始的数据照取舍 P4-15。

### 3.6 每一行怎样量

下面每行写：规模与形状、种子、驱动与命令、采样、重复、统计量、判在、判据、超出时。

#### B1 搜索

- **规模与形状**：30 万页（2% 已删），200 本与 5,000 本两种分布（3.2）。
- **种子**：语料 `NW_PERF_SEED=20261116`。
- **驱动与命令**：`NERVE_SEARCH_CORPUS=<目录> NERVE_SEARCH_BACKLOG=<档> go test -count=1 -v -run '^$' -bench '^BenchmarkSearch$' -benchtime 1x -timeout 6h ./internal/bootstrap/`。
  - 200 本：第一轮 `NERVE_SEARCH_BACKLOG=0,1600`（M8 的积压最大值记在约 1,000–1,600 页），第二轮 `0`；每格 20 次（默认）。两轮的积压 0 不是同一条件（修订核对 B 的 N14）：`BenchmarkSearch` 逐格交替积压，第一轮每格的 1,600 页把 `nodes`、`page_contents` 的 `updated_at` 各更新两次，而 `newTrashBench` 关了这两张表的 autovacuum；`updated_at` 没有索引、是 HOT 更新，影响多半小。记录写明；候选只出现在第一轮、第二轮没有时，照 3.9 第 2 条复核那一格（积压 `0`）。
  - 5,000 本：两轮 `0`，`NERVE_SEARCH_REPS=10`：R1 记的 827.2 ms 是每格 10 次的 p99，即 10 次里的最大，20 次的会系统地偏大（审查 B 的 N4）。
  - 另跑 `BenchmarkSave`（`-benchtime 300x`，`save_bench_test.go:19`）两轮。
  - 建索引前后在同一个容器上读 `pg_current_wal_lsn()`（3.7 第 17 条）。
- **采样**：基准自己的日志：每格的 p50、p99，建索引的行数、轮数、行/秒、MB/秒（`:382`），体积与对正文的倍数（`:387`）。
- **重复**：每种分布两轮。
- **统计量**：每格两轮里较大的 p99；体积取较大的倍数；吞吐取较小的；从空建的用时取较长的。
- **判在**：本机。
- **判据**（取舍 P4-3；2.1 第 2 条）：
  - **照原预算直接判**（审查 B 的 I1）：
    - 任务的吞吐 < 100 页/秒为超；
    - 30 万页从空建 > 1 小时为超（`buildSearch` 从全部入队到队列清空，`:365-382`）；
    - 一页的保存 p99 多 > 2 ms 为超：`BenchmarkSave` 的 p50、p99 记下；S3 用 `git log` 核对入队（`page_search_queue` 的写）自 M8 以来改了没有，没改就写"入队的开销沿用 M8 实测的 p99 0.69 ms（推算）"、照它判，改了就照 M8 的做法量入队本身再判。
    - 参照（只记，不改结论）：M8 的产品任务在真语料上从空建 285–329 页/秒（`05-P5-search.md:900`），合成语料上 R1 之后按节点的 id 174 行/秒、修复二轮按笔记本 314 行/秒（`:1123`），M9/P3 的 `BenchmarkSearch` 249 行/秒（`M9-mcp/03-P3-batch-revert.md:1189`）。初稿用的 189 行/秒是满的索引上重建 2 万行的下限（`:1125`），不是从空建，删掉。
  - **两步：只给 p99 的尾巴与体积**：
    1. **候选**：200 本、积压 0，任一格的 p99 > 719 ms（M8 在 200 本上记下的最大，P5A 真语料"为任意的参数规划"那一阶段）；5,000 本任一格 > 827.2 ms（Codex 评审 R1 之后，无统计）；体积 > 正文的 5.25 倍（R1 之后按节点的 id 建）。
    2. **对照**：有候选的那一种分布，在同一台机器、同一份语料上跑 `308eafc6`（M8 的 Codex 评审合并，按笔记本分键之后）的 `BenchmarkSearch` 两轮，整轮地跑（它没有按格过滤的开关，也没有 `NERVE_SEARCH_BACKLOG`，审查 B 的 I2）；`NERVE_SEARCH_REPS` 与今天的那一种相同。候选的格，今天两轮里较大的 p99 比 `308eafc6` 两轮里较大的慢 20% 以上，而且仍超第 1 步的界，才算回归；体积同样比。
       - 语料用 `NW_PERF_CORPUS_SCHEMA=308eafc6` 写的那一份（只有 `notebooks.tsv` 不同）；开跑之前在两个提交的库上各读一次 `notebooks`、`nodes`、`page_contents` 的列（`information_schema.columns`），记录写出差别（审查 A 的 M10）。
       - 记录写明混进对照的变量：`308eafc6` 的 `pgtest` 用 PostgreSQL 18.6；它的 `page_contents.content` 是默认的 pglz，今天是 LZ4（00054）。
  - **只记**：积压 1,600 的格（第一轮）：1,191 ms 是 M8 第 1 步的工具在"准确的积压 1,000 页、1 万个附件、负载到 26"时量的，与它不同口径，`308eafc6` 上也没有对照（审查 B 的 I2 与过度设计）。
- **不重量的**："建索引时"（30 万个节点以批量排队、任务在跑时的读，M8 记过至多 908 ms）与"导入 5 万页的清空期间"（1,149 ms）是 M8 第 1 步的工具量的，`BenchmarkSearch` 没有这两个阶段。B1 的尾巴只看回归，为它们另写驱动不值（00 号文档 4.7.1 的"量的范围缩小"）；记录写明沿用 M8 的数、没有重量（2.1 第 2 条，审查 C 的 M5）。
- **超出时**：照原预算判的三项超了，在 v0.1 修（S4 起，带缩小规模的守卫）；尾巴与体积回归了同样修，没回归维持已知限制（00 号文档 4.7.1；I2-D3）。

#### B2 解析预算与内存

- **规模与形状**：k = 2、4、8、16 个并发，各一个凭证；⌈k/2⌉ 个读阅读视图、⌊k/2⌋ 个写正文，各自一页；正文 5 MiB（`MaxContentBytes`，`page/domain/content.go:12`），普通与病态两种（3.4.5）。普通的是带标题、列表与链接的散文。
- **种子**：正文的生成 `-seed 2`。
- **驱动与命令**：`stack.sh up`，空闲 130 秒（这一轮的基线）；`perf bodies -k <k> -kind <normal|pathological> -warmup 10s -for 60s`，八个组合依次跑，之间空闲 30 秒；全程 `perf sample`。
- **采样**：gctrace（3.4.2）、`/proc` 与 cgroup 每 100 ms、每次请求的记录。
- **重复**：两轮，`stack.sh restart` 隔开；每轮开头重取基线。
- **统计量**：每个组合，堆的增量取两轮的较大；末尾的上界一并写；"早答的 503"两轮合计。
- **判在**：本机，不乘（内存不随 CPU 变）。
- **判据**：任一组合的增量 > 2,400 MiB（8 MiB × 300，`config.yaml:97` 的注释；README:140 的"约 2.4 GB"是同一个乘积的约数，增量落在 2,289–2,400 MiB 之间时两样都写、列给主持者，审查 B 的 N9）；上界跨过界线的照 3.4.2 重量那一组合；或任一个 503 在客户端从发出到答复头不到 2.0 秒（取舍 P4-21：解析预算与大请求体的名额都等 2 秒，`platform/markdown/budget.go:86-106`、`bootstrap/mcp.go:41`）。
- **写者在哪本笔记本**：判的组合里每个写者各在自己的一本笔记本，k 个解析同时进行，堆最大；另记一个组合：k = 8、普通正文、写者都在同一本笔记本里，看等链接索引的锁时占着的解析预算让读者早答多少（M4-performance 第 3 条，总体设计第 14 节"先到先得"一行；审查 A 的 U2）。
- **另记**（部署文档）：`VmHWM`、PostgreSQL 的 `memory.peak`、503 的比例、各操作的 p50、p99。小机器档（取舍 P4-27）：`stack.sh up --env NWIKI_PAGE__PARSE_BUDGET_BYTES=5242880 --env GOMEMLIMIT=1536MiB`，k = 8 的两种正文各一次，记同样的数。
- **超出时**：修（S4 起）。

#### B3 大请求体

- **规模与形状**：照 M9/P2 量过的（[M9/P2](../M9-mcp/02-P2-writes-history.md) 3.2.6、第 353 行），各由三个凭证同时发（两个名额加一个等着的，`bootstrap/mcp.go:40-41`）：
  1. **长字符串**：`tools/call` 的 `write`，`content` 是 5 MiB 的普通正文；
  2. **上限处的值**：`tools/call` 的 `_meta` 里放 `valuesShapes()` 的三种（许多成员、嵌套的空数组、长的平的数组，`platform/httpserver/mcpserver/values_test.go:14-34`），每种恰在 `MaxBodyValues` 上（`MaxBodyValues − 9` 个成员，同那个测试），体过 64 KiB；M9 量出贵的正是 SDK 把 `_meta` 泛型地解码成 map 与 slice（审查 A 的 M7；初稿放进 `batch` 的 `ops`，`batch` 至多 100 项，`page/domain/batch.go:49`，量不到这一种）；
  3. **过了上限的值**：同 2.，三种各多一个值，应答 400；
  4. **混合**：两个凭证发 1.，第三个发一个 5 MiB 的 REST 正文写（`PUT …/content`，过 1 MiB 也取名额，`page/adapter/http/limits.go:8-17`）。
- **种子**：`-seed 3`。
- **驱动与命令**：`perf mcp -shape <1|2a|2b|2c|3|4> -reps 3 -gap 20s`；全程 `perf sample`。
- **重复**：每种形状每轮 3 次，两轮。
- **统计量**：每种形状，两轮六次的增量里的最大（取舍 P4-21；审查 B 的 I3：初稿取中位，与 4.7.3 的"峰值"不合，这一格离界很近，M9 记过最高 +961 MiB，同一形状的几次能差约 190 MiB）；末尾的上界一并写。
- **判在**：本机，不乘。
- **判据**：任一种形状 > 1,024 MiB（"两个合起来约 1 GB"，`README.md:628`）。第三个凭证等满 2 秒答 503 是设计的行为，只记它的用时。
- **超出时**：修（S4 起）。

#### B4 请求期限下的树写

- **规模与形状**（取舍 P4-9）：导入上限的 vault（3.4.3）导进一本标准的笔记本、放在根下一页 `vault` 之下：这一页是一个约 6 万个节点的最上层。
- **种子**：vault `-seed 1201`。
- **驱动与命令**：批次 F 的导入、等搜索的队列清空之后（同一轮）：`perf ops`，四种操作各 3 次，每次之前等后台安静（3.4.5 的 `ops`，取舍 P4-10）：
  1. 删除 `vault`（约 59,911 个节点），之后经后台的恢复等它成功；
  2. 把 `vault` 移到根下另一页之下；
  3. 改 `vault` 的名；
  4. 改被链得最多的那一页的名（记下它被几页链着）；
  另删一个第一层文件夹（约 6,000 个节点）3 次，只记，看曲线。
- **`fsync=off` 的对照**（取舍 P4-19，审查 B 的 Q2）：批次 F 另起一个 `stack.sh up --fsync-off` 的库，导入同一个 vault、等队列清空，删除 `vault` 3 次（各经恢复、等安静）。
- **采样**：每次请求的用时与状态；`perf sample`。
- **重复**：两轮（批次 F 的两轮，各是新的库）；`fsync=off` 的一轮。
- **统计量**：每种操作两轮的较大中位数；`fsync=off` 的删除取三次的中位数。
- **判在**：没有 CI 上的实数，推断（写"推断"）：
  - 删除：`fsync=off` 的本机数 × 回收站与撤销族的比（3.5.4），加两种容器之差（不带 `fsync=off` 的减带的，不乘）；`fsync` 的那一份不随 CPU 变；
  - 移动与改名：本机 × 同一个比（它们写的 WAL 少，差别记下、不另量）；
  - 另并列：删除是 `fsync=off` 的本机数 × 比 × 1.25，再加差（差不乘）；移动与改名是本机 × 比 × 1.25（取舍 P4-2；修订核对 B 的 N11）；本机 × 4.6 的旧结论。
- **判据**：推断的 > 15 秒，或本机就有一次答 500、503。× 1.25 那一行过 15 秒而主行没过时，列给主持者，不自动判"超"。预告：P4-2 多半决定删除的结论（2.2）。
- **只记**（取舍 P4-10）：同样的四种操作在 `perf editors` N = 20 的负载下各一次。
- **病态形状**（已知限制，用已有的数，不量；取舍 P4-9）：20 万个节点的最上层的删除（10.6–13.0 秒，到期限，移交第 16 条）；单个 `move` 400 页 × 2,000 条链接（14.3–14.6 秒，移交第 18 条）；被 2,500 页链着的页的改名（每页 10 KB 改写 2,500 页 10.3 秒，5,000 页到期限，`v0.1-design.md` 第 14 节"同一笔记本的保存串行"一行）。这些是当时的开发机（M5 Max）上的数；记录另照 7763 对 M5 Max 约 2.8 倍（界线处的恢复 11.0 秒对 3.8–4.0 秒，推断）换算到最低配置上到期限的规模，标"推断"（审查 B 的 M9：例如被约 1,300 页链着的页的改名）。
- **超出时**：修，照用户碰到的错误（I2-D6）。

#### B5 一个单元 ≤ 5 秒

- **规则一层**（取舍 P4-1、P4-17；审查 B 的 M4）：
  - **档**：
    - `MaxUnitNodes`：`BenchmarkTrashRestore` 的 `full/10000 attachments`（10,100 个节点），无统计与 ANALYZE 之后两格，`unit-ms`；
    - `MaxUnitHistory`：`BenchmarkTrashRestoreHistory` 的每页 1,000 个版本（419,900 行，[M8/P2](../M8-history-search/02-P2-trash.md) `MaxUnitHistory` 的表），无统计与 ANALYZE 之后两格，`unit-ms`；
    - 撤销的界线：`BenchmarkRevert` 的删除、恢复、新建 × 另有 0 与 10 万个节点 × 三个统计阶段，`unit-ms`；
    - `MaxRevertSteps`：`NERVE_BENCH_TOPS=1500` 的 `BenchmarkRevertOfManyTops`，`request-ms`（这个基准没有 `unit-ms`；M8 定界线时用的也是它，移交第 15 条）；
    - `MaxJobItems`：`NW_JOB_LINKS=20 NW_JOB_SIZES=60000` 的 `BenchmarkBackgroundSteps` 的 `top-ms`、`group-ms` 与 `BenchmarkBackgroundJobs` 的 `revert-max-batch-ms`、`restore-max-batch-ms`。
  - **命令**（这些文件是 `!race`；正则锚住，`BenchmarkTrashRestore` 不带锚也匹配 `BenchmarkTrashRestoreHistory`）：
    - 回收站与撤销：`NERVE_BENCH_LOADS=full NERVE_BENCH_ATTACHMENTS=10000 NERVE_BENCH_VERSIONS=1000 NERVE_BENCH_TOPS=1500 go test -count=1 -v -run '^$' -bench '^(BenchmarkTrashRestore|BenchmarkTrashRestoreHistory|BenchmarkRevert|BenchmarkRevertOfManyTops)$' -benchtime 3x -timeout 3h ./internal/bootstrap/`；
    - 后台：`NW_JOB_LINKS=20 NW_JOB_SIZES=60000 go test -count=1 -v -run '^$' -bench '^(BenchmarkBackgroundSteps|BenchmarkBackgroundJobs)$' -benchtime 1x -timeout 3h ./internal/bootstrap/`：后台的基准不按 `b.N` 循环，`3x` 会把每格的主体（一次 6 万个节点的导入）跑两遍、只报最后一遍（审查 B 的探针）。
  - **重复**：两轮。
  - **统计量**：回收站与撤销的每格每轮是 `3x` 报的三次的均值（`ReportMetric` 取 `b.N` 次的均值，M8 定界线时一样）；后台的每格每轮一个样本（M8 的"每次运行都不超过 5 秒"）；两轮取较大（2.1 第 13 条）。
  - **判在**：本机，不乘（M8 的规则，00 号文档决定 318）。
  - **判据**：任一格 > 5,000 ms。
  - M8 第一次跑 `BenchmarkTrashRestoreHistory` 的 ANALYZE 那一库时，从 1,000 个版本起建数据的语句被取消（[M8/P2](../M8-history-search/02-P2-trash.md) `MaxUnitHistory` 一段）；重现时记下、那一库单独重跑一次，仍失败就报主持者（审查 B 的未核实的怀疑）。
  - **没过时**（00 号文档 4.7.5）：不自动降常量。对没过的那个常量给降常量的候选（取舍 P4-17）：先照 M8 的做法在写死的两档之间线性插值（M8 对 ANALYZE 那一列就这样推出"约 52 万行处过 5 秒"），再以 3.3 末的档位开关（`NERVE_BENCH_ATTACHMENTS`、`NERVE_BENCH_VERSIONS`，`NERVE_BENCH_TOPS`、`NW_JOB_SIZES` 已有）在插值点两边各补一档、跑两轮，给出"5 秒之内的最大一档"。连同"写进部署要求"（要求比本机快多少的 CPU）两种做法、各自的影响（例如降 `MaxUnitNodes` 时更多的恢复走后台、导入的单元随之），列给主持者与负责人定。
- **硬期限一层**（取舍 P4-16；审查 B 的 I4）：
  - 只对请求一路的三个守卫：界线处的恢复、撤销一次删除、撤销很多最上层（`MaxJobItems` 是后台的单元，没有请求期限，00 号文档 A-N15）。
  - **判据**：窗口里每一次落在 7763 上的 `main` 的 `guards-core` 的这三行（`trash_cost_test.go:538`、`revert_cost_test.go:622`、`:669`），最大值加 `parse_max_wait`（2 秒）> 15 秒。
  - **窗口**（取舍 P4-16；修订核对 B 的 M13）：从下面这些文件最后一次被改到的提交起（`git log -1 --format=%H <被量的提交> -- <文件>`，再读 diff：只改别的测试函数或只改期限与客户端的不算，往前找下一个），到被量的提交为止。窗口里的运行是 `main` 上提交包含那个改动的各次（`git merge-base --is-ancestor <改动> <运行的提交>`）：改动多半先在旁支上，按时间排会把合并之前 `main` 上的运行算进来（修订核对 B 的 N16）。列的文件：
    - 守卫：`bootstrap/trash_cost_test.go`、`bootstrap/revert_cost_test.go`，只算改到这三个守卫与它们调的帮手（`newTrashBench`、`item`、`versions`、`toppedUp`、`timed` 等）的提交；P4/S1 给 `BenchmarkTrashRestore` 加档位开关不算；
    - 产品：`page/domain/trash.go`、`page/domain/revert*.go`；`page/app/` 的 `trash.go`、`restore_node.go`、`unit.go`、`unit_restore.go`、`unit_revert.go`、`revert_changeset.go`、`revert_actions.go`；`page/adapter/postgres/` 的 `trash.go`、`history.go`、`revert_tree.go`、`changesets.go` 与 `queries/` 的 `trash.sql`、`history.sql`、`changesets.sql`；`linking/adapter/postgres/store.go` 与 `queries/links.sql`（恢复时重解析链接，M8 改过的 `SetResolutions` 在这里）。
    - P3/S2 改撤销的规划，改到 `page/domain/revert.go`、`revert_part.go`、`revert_facts.go` 与 `page/app/unit_revert.go`（[P3 设计](03-P3-server-correctness.md) 3.10.3）：S3 的窗口多半从 P3/S2 的合并起。
  - 窗口里不到三次就用已有的，写明次数。一次也没有时：取改动之前最近的至多三次 7763（就是 3.5.4 里 `guards-core` 那一组：窗口里没有，最近的三次都在改动之前），各乘本机在被量的提交上与在那一次的提交上的守卫之比（本机新 ÷ 本机旧），取最大加 2 秒，写"推断"。
  - **并列两行推断**（标"推断"）：
    1. 7763 各次的中位 × 1.25（决定 359 记的 7763 的尾巴）加 2 秒；窗口里一次也没有时，用上一条推断的各次；
    2. 每个守卫，它量的那个形状在基准里最慢的一阶段（多半是 ANALYZE；`BenchmarkRevert` 另有"为任意的参数规划"一阶段，修订核对 B 的 N10）的那一格（本机，`request-ms`），除以本机在被量的提交上的守卫的数（3.5.4），乘 7763 上的最大（窗口里一次也没有时用推断的），加 2 秒。守卫量的是无统计、不 `VACUUM` 的状态（`trash_cost_test.go:511-539`）；M5 Max 上 ANALYZE 状态的单元 4,288 ms 加解析约 4.5 秒，对守卫的请求 3.62–3.94 秒，慢约 1.14–1.24 倍（修订核对 B 的 N15：第一次修订拿单元比请求，写成 1.10–1.15），推到 7763（恢复约 11.0 秒）加 2 秒约 14.5–15.7 秒，平常的 7763 就可能过 15 秒（审查 B 的 I4，推断）。
  - 任一行过 15 秒，就同规则一层一起列给主持者与负责人；部署文档的硬件下限照他们定的写（第 9 节；修订核对 B 的 N18）。照今天的数，第一行已是 15.75–16.4 秒（2.2，修订核对 B 的 M15）。
  - 初稿以"今天的两次 7763"作余量的依据，删掉：它们在 P3 改撤销之前，照本条的窗口不算（窗口里一次也没有时，才经上面的推断用改动之前最近的几次）。
  - **没过时**：同规则一层，两种做法列给主持者与负责人。

#### B6 lint

- **规模与形状**：10 万页（守卫的夹具，`wiki_lint_cost_fixture_test.go:43`）判；30 万页只记。
- **种子**：夹具自己的（确定的）。
- **驱动与命令**：
  - 10 万页：CI 的 `costs-lint` 的日志：`wiki_lint_cost_test.go:65: wiki cost summary median wall-ns=… bound=…`（`wanted` 同样；每条规则的 `TestLintRulesOfAHundredThousandPagesTakeTheirTime`，`Makefile:520-521`）。本机不跑：lint 一族的比不量（3.5.4）。
  - 30 万页：`NW_SCALE_PAGES=300000 go test -count=1 -p 1 -v -timeout 2h -run '^TestLintOfAHundredThousandPagesKeepsItsTimeAndMemory$' ./internal/bootstrap/`，生产的期限（worker 7.5 秒）；摘要或 `wanted` 答 `server_busy` 时再以 `NW_SCALE_REQUEST=60s` 跑一次（worker 30 秒，不截 10 秒，3.3），得真的用时。
- **重复**：30 万页一轮（只记）。
- **统计量**：守卫自己的中位数（3 个样本）。
- **判在**：10 万页由 7763 上的实数定：窗口里各次的最大，不到三次就用已有的、写明次数（窗口同取舍 P4-16；文件是 `bootstrap/wiki_lint_cost_test.go` 的这个用例与 `wiki_lint_cost_fixture_test.go`、`linking/adapter/postgres/lint_cost_test.go` 的规则的用例，产品是 `wiki/app/lint*.go`、`linking/app/lint_*.go`、`linking/adapter/postgres/lint*.go` 与 `queries/lint*.sql`、`page/adapter/postgres/lint_metadata.go` 与 `queries/lint_metadata.sql`；P4/S1 只改这个用例的期限，不算）；同规模，不乘（00 号文档 A-N13）。窗口里一次 7763 也没有时，本机 10 万页一轮 × 升级族的比，写"推断"（lint 一族的本机守卫不在旧提交上跑，3.5.4）。
- **判据**：摘要或任一条规则过 7.5 秒（生产的 lint 期限，`bootstrap/wire.go:253`）。今天 7763 上摘要 1.72–2.01 秒（M10 决定 160），每条规则的数 S3 从日志读。升级的检查经 lint 的 worker（M11 第 262 条），它的 7.5 秒也属这一行的预算（B7）。
- **另记**：`TotalAlloc`（B11：被 1,571 MiB 的守卫取代，不是预算，I2-D8）。
- **超出时**：修。

#### B7 升级的发起与批准

- **规模与形状**：10 万页（发起、检查）与 10 万个来源（批准）判；30 万页只记。
- **驱动与命令**：`NW_SCALE_PAGES=100000 go test -count=1 -v -timeout 1h -run '^TestUpgrade(Start|Check|Approval)TakesItsTimeAtAnyStatistics$' ./internal/bootstrap/`，生产的期限（发起与批准 11.25 秒，检查的 worker 7.5 秒）；答 `server_busy` 或 500 时以 `NW_SCALE_REQUEST=60s` 再跑一次，得真的用时（审查 B 的 N5）。30 万页同样一轮，必要时 `NW_SCALE_REQUEST=120s`。
- **重复**：10 万两轮。
- **统计量**：守卫的中位数（3 个样本），各统计状态与计划模式里最大的那一格，两轮的较大。
- **判在**：CI 上只有 5 万页的实数（`upgrade_start_cost_test.go:41-44`），规模不同，推断：本机 10 万页 × 升级族的比（3.5.4），写"推断"；另并列 × 1.25 的一行与 × 2.33 的旧结论（00 号文档 A-N13）。
- **判据**：发起或批准推断的 > 11.25 秒为 B7 超。检查推断的 > 7.5 秒记在 B6 的预算下（检查经 lint 的 worker；只有 4.7.1 的预算判超出，决定 317，审查 B 的 N6）。
- **超出时**：修。

#### B8 导入与导出

- **规模与形状**：导入上限的 vault（3.4.3）；导出这本笔记本的 `vault` 那一页之下（约 3.8 GiB 的附件）。
- **种子**：vault `-seed 1201`。
- **驱动与命令**：`perf vault …`（一次，两轮共用）；每轮 `stack.sh up`，空闲 130 秒，`perf transfer -import <zip>`，等搜索的队列清空（`psql` 看 `page_search_queue` 为空、River 没有在跑的任务），再 B4 的 `perf ops`，再 `perf transfer -export`，`stack.sh down`。
- **采样**：任务从 202 到成功的用时、上传与下载的用时；`perf sample`；WAL 的字节、库与存储目录（与它的 `imports/`、`exports/`）的大小。
- **重复**：两轮。
- **统计量**：两轮的较大。
- **判在**：推断：本机 × 两族的比的较大；另写 × 4.6。
- **判据**：推断的 > 6 小时（`config.yaml:144`）。
- **另记**（部署文档）：内存的峰值、磁盘（附件、库、导出的 zip、过程中的临时文件）、WAL、搜索的队列清空用的时间。
- **不量**（审查 B 的 N8）：M7-transfer 第 2 条另问的"连接的占用"与"与同时的编辑、上传争用磁盘时 `storage_full` 的表现"。前者是一个任务至多一条连接、照 `max_conns` 的写法推（写"推算"）；后者要把磁盘写满，在本机做有伤到别的东西的风险，`storage_full` 的行为已由 M7 的测试守住（写进部署文档的"由管理员监控磁盘"，P5/S3）。都不判预算（00 号文档 4.7.1 末段）。
- **超出时**：修。

#### B9 编辑器的解析

在 Chromium 里，第 4.4 节。

#### B10 `index` 与日志

- **`index`**：
  - 10 万页由 7763 上的实数定：`costs-1` 的 `TestIndexOfAHundredThousandPagesTakesItsTime` 的日志行 `index_cost_test.go:326: ListIndex of a page of 100: [...]`，窗口里各次的最大，不到三次就用已有的、写明次数（窗口同 P4-16；文件是这个测试与 `linking/app/list_index.go`、`index_directory.go`、`linking/adapter/postgres/index.go`、`queries/index.sql`）；窗口里没有 7763 时本机一轮 × 升级族的比，写"推断"。本机不为比跑（3.5.4）。
  - **判据**：> 3.5 秒（`indexCallBound`，`:37`）照 13.4 第 8 条调守卫的界（一个小的条件 Step，3.9）；> 15 秒照 B4 修。今天 7763 上 3.01–3.03 秒（移交第 20 条）。
- **按页的日志**：只记，10⁵ 与 10⁶ 个变更集：`NW_SCALE_CHANGESETS=1000000 go test -count=1 -p 1 -v -timeout 2h -run '^(TestTheLogReadsInTimeAsLongAsAPage|TestTheRevertsArrayReadsInTimeAtTheUnitsBound)$' ./internal/modules/page/adapter/postgres/`（3.3；10⁵ 是默认，同一条命令不设变量）。200 ms 的守卫在约 13–25 万个时过，记成已知限制，迁移进 v0.2（I2-D7）。

### 3.7 只量的各项

照 00 号文档 4.7.1 末段的范围；能用已有的数推的推，写"推算"与依据。

| 出处 | 怎样得到 | 批次 |
|---|---|---|
| 移交 M4-performance 头一段、13.4 第 9、12 条：旧版本下的通用计划 | 在 F 导入之后的库上（有死行、活着时 ANALYZE），同一条连接跑 B4 的读 6 次以上，看 `pg_prepared_statements` 的 `generic_plans` 与用时；13.4 第 9 条的"返回 2,000 却扫 52,000"在 `lint` 的计划测试上照原样复测一次 | F、C |
| 第 1 条 共享锁与树写；第 6 条 链接多的页的自动保存 | `perf editors` N = 5、20、50，各 120 秒：树写的 p50、p99，MultiXact 的 `pg_stat_slru`；其中一个编辑者的页有 10,000 条链接（上限，`linking/domain/facts.go:114`），记它保存的 p99 | E |
| 第 2、3 条 | B2 | E |
| 第 4 条 | B4 | F |
| 第 5 条 活动的最后写入 | 推算：`overview_cost_test.go` 的 10 万个变更集 5.7–7.9 ms（I2 第 1.3 节），按变更集线性推到 10⁶ | — |
| 第 7 条 补全读链接目标 | 4.5 | S2 |
| 第 8 条 很多同名的页 | 照 `costs-1` 里已有的几个测试在本机跑一次，记 | C |
| 第 9 条 | B9 | S2 |
| 第 10 条 ANALYZE、通用计划、连接池代理 | 同第一行；"恢复之后先 `ANALYZE`、只能会话模式"写进部署文档（P5/S3） | F |
| 第 11、13 条 大图与每小时的整篇替换 | 4.6 | S2 |
| 第 12 条 附件列表的重读 | 4.7（I2-D10 的判据在那里） | S2 |
| 第 14 条 回收站 | B5；另记 `BenchmarkBackgroundSteps` 的 `history`（100 万行历史的一步，`background_cost_test.go:610`）一次 | D |
| 第 15 条 整组撤销与日志 | B5、B10；另记 `BenchmarkPurgeOfTrimmedChangesets`（`revert_cost_test.go:504`）一次 | D |
| 第 16 条 后台的恢复与撤销 | B5；整个任务的时长取 `BenchmarkBackgroundJobs` 的日志（`jobRun.String`） | D |
| 第 17 条 搜索 | B1；WAL 取批次 B 建索引前后的 `pg_current_wal_lsn()`（在 `BenchmarkSearch` 的日志之外，S3 另在同一个容器上读一次；读不到时用 F 里导入之后搜索的队列清空的 WAL）；`nervewiki reindex` 不另量（P5 设计 3.18 的"升级"沿用 `README.md:535` 的数或以 B 的从空建作下界，写"推算"） | B、F |
| 第 18 条 MCP | 分项照 I2 第 5.1.1 节：大请求体 B3；`index` B10；按页的日志 B10；`edit` 最坏、glob、`batch` 的改写用 CI 上已有守卫的数（`guards-core` 的五个 batch 测试与三个 grep 守卫，`Makefile:562`、`:570`）；10 万个节点的 `list` 在 E 的 10 万页上一次；`BenchmarkGrepAtFullLoad`（`page/app/grep_load_test.go:144`）一次；慢的发送者与 Caddy 早答只写进部署文档（I2-D12、D13） | B、C、E |
| 第 19 条 LLM Wiki | B6；30 万页的 lint 只记；`index.md` 的读用 `TestVirtualIndexOfAHundredThousandPagesTakesItsTime` 的 CI 数；迁移的停机只在批次 H | C |
| 第 20 条 两个守卫的 CI 记录 | 照 3.5.4 读到的 CI 数写余量 | C |
| 第 21 条 升级 | B7；回滚的数用 `costs-3` 已有的两个守卫的 CI 数（`Makefile:485`、`:497`） | C |
| 第 22 条 分份与 `apitest.Load` | 不做（I2-D11） | — |
| 补记 `:480–:488`、`:494` 来源的首写负载、真实的附件、MVCC 的旧版本 | 旧版本同第一行；来源的持续并发首写不写驱动，写明不量与理由（不判预算，00 号文档 4.7.1 末段）；附件用 F 的 vault | F |
| 补记 `:490`、`:492` | 硬件下限进部署文档（P5/S3）：数取 B5 硬期限一层的实数与两行推断、B4 与 B7 的推断（含 × 1.25 的一行）、3.5.4 的比；`batch` 的期限进已知限制 | — |
| M5-performance 第 3 条 重连与整体刷新 | 推算：N 个标签页 × 一次树的读（E 的 `tabs` 量的每次读的用时与字节）加阅读视图的读（B2 的普通页）；不写驱动（00 号文档 4.7.4） | E |
| M7-transfer 第 2、5 条 | B8；第 2 条的连接占用推算、`storage_full` 不量（B8 的"不量"） | F |
| M7-transfer 第 4 条 轮询 | 4.8 | S2 |
| M7-transfer 第 6 条 附件 | 节点树的大小在页面树的带附件的形状（4.3）；按凭证的并发上传不量（00 号文档 4.7.1） | S2 |
| M8-history 第 2 条 版本的增长 | 推算（批次 G）：一个版本是一次编辑会话（`RecordRevision` 以 `(changeset_id, node_id)` 就地更新，`page/adapter/postgres/queries/changesets.sql:53-66`；移交 `M8-history.md` 第 2 条），不是一次保存（审查 B 的 M6）。E 的 `editors` 开始之前与结束之后各 `VACUUM FULL page_revisions`，每个版本的字节 =（两次的表、TOAST 与索引之和的差）÷（行数的差），正文是 LZ4 压缩的真实正文。不用普通的 `VACUUM`：它只把死元组的空间标成可用、不缩小文件，而每次保存就地更新同一行、写一份新的 TOAST（每个版本约 30 份），大小之差会是运行中的最高水位（修订核对 B 的 M12；审查 B 的 M6 与第一次修订都写错了）；部署文档写"每人每天 S 次编辑会话 × 每个版本的字节"的公式与两三组代入的数 | G |
| 4.7.3 的 10 万页的树重读 | `perf tabs` k = 1、4、8，各 60 秒：服务进程的 CPU（`utime + stime` 的差除以 60 秒）、PostgreSQL 容器的 CPU、出口的字节、每次读的用时 | E |

### 3.8 S3 的批次与机器时间

估计都是**推断**（I2 第 6 节的估计、M8 与 M9 记的用时、本机比 M5 Max 约慢 1.4 倍、探针 (3) 的用时、审查 B 的 M7）。每批整段持一次 heavy 的锁；00 号文档要求 P4/S3 独占机器（第 7 节"次序与并行"），主持者把它排在没有别的 Step 在实施与审查的时段。

| 批次 | 内容 | 本机的时间 | 磁盘 | 时段 |
|---|---|---|---|---|
| B | 两份语料各生成一次（10–20 分钟一份）；200 本两轮（每轮载入与从空建约 30 分钟、读约 25 分钟，第一轮另有积压 1,600 约 15 分钟）；5,000 本两轮（每轮从空建约 30 分钟、`NERVE_SEARCH_REPS=10` 的读约 12 分钟）；`BenchmarkSave` 两轮（几分钟） | 约 4–4.7 小时 | 语料 2 × 2.4 GB；一次运行的库约 15–20 GB（`page_search` 约正文的 5 倍，加 WAL） | 第一夜 |
| B′ | 有候选时：`308eafc6` 的 worktree 与构建；有候选的那一种分布两轮整轮（200 本约 1.8–2 小时，5,000 本约 1.5–1.7 小时） | 至多约 3.4–3.8 小时（条件） | 同 B | 另一夜（条件） |
| C | 3.5.4 的本机守卫：每个作业最近的至多三次 7763 的提交上各跑那一族一次，被量的提交上两族一次（提交数 × 10–15 分钟：worktree、编译、那一族的守卫；推断 6–10 个提交，1–2.5 小时）；lint 30 万一轮；升级三个守卫 10 万两轮、30 万一轮；按页的日志 10⁶ 一次；第 8 条的几个测试一次（这几样约 1.75–2 小时） | 约 3–4.5 小时 | 小 | 第二夜 |
| D | B5 的界线那一档两轮（档位开关只建选中的档）；第 14–16 条的另记各一次；没过时的候选（插值、两边各补一档、两轮） | 约 2.3–2.8 小时；候选另加 1–2 小时（条件） | 小 | 第二夜，C 之后 |
| E | 两份 5 万页的 vault 生成、导入（10 万页的树）；B2、B3 各两轮；小机器档一次；`editors` 三档两轮；`tabs` 三档两轮；空闲与稳态的内存；版本的字节 | 约 2.5–3 小时 | 小 | 第三夜，或一个傍晚 |
| F | 导入上限的 vault 生成一次（约 10 分钟）；两轮：导入（推断 20–40 分钟）、等搜索的队列、B4（每次删除之后经恢复、等队列清空，约 12 分钟一次）、导出与下载；`fsync=off` 的库上导入与删除三次 | 约 4–5 小时（导入与导出的用时没量过，推断） | 每轮约 10 GB（附件约 3.8 GiB、库、导出的 zip、上传的 zip）；`fsync=off` 的库另约 8 GB，先 `down` 前一个再起 | 第四夜 |
| G | 推算，不跑 | — | — | — |
| H | 只在负责人说有 v0.1.0 之前的实例时（2.2 第 1 条） | 1–2 小时 | — | — |
| **合计** | | **约 16–20 小时**；条件的另约 4.5–6 小时（B′、D 的候选），H 另算 | | 四夜，条件的另一夜（C 与 D 一夜约 5.5–7.5 小时，D 的候选放不下时另一夜） |

- **比 00 号文档的估计多多少、为什么**：00 号文档 4.7.3 写"缩小之后的机器时间少于初稿估的 10–16 小时"。S3 无条件的部分约 16–20 小时，加 S2 的约 3–3.5 小时（4.10），P4 约 19–23.5 小时，比 10–16 小时多约 3–7.5 小时；条件的全开还要多 4.5–6 小时。多在：
  - B1 两种分布各两轮就占约 3.5–4 小时（从空建 30 万页本机约 30 分钟一次，审查 B 的 M7：初稿的"每轮约 40 分钟"偏少）；对照照整轮算（审查 B 的 I2：`BenchmarkSearch` 不能只跑几格），有候选时另一夜；
  - F 的每次删除之后要等搜索的队列（审查 B 的 M8），另有 `fsync=off` 的一个库（审查 B 的 Q2），约多 1.5–2 小时；
  - 3.5.4 的本机守卫要在 7763 的提交上跑（审查 B 的 M1），每个作业至多三次（修订核对 B 的 M14），约 1–2.5 小时。
  - 省下的：来源、lint、`index` 三族的本机两轮（约半小时）、积压的第二轮与对照（约 0.5 小时）、5,000 本每格 10 次（约 0.4 小时）、B5 的档位开关省下不要的档的准备。
- **主持者可以取的省法**（每条省约 0.4–1 小时）：5,000 本只跑一轮（它的尾巴只看回归，有候选时还要对照）；F 的第二轮只做 B4、不再导出（B8 离 6 小时远）；`fsync=off` 的库只删一次；3.5.4 每个作业只取最近的一次 7763（比的样本少、摆动大；B5 的窗口里一次也没有时仍要改动之前的那几次）。
- 每批之间释放锁，别的 Step 可以跑短的东西；批内不释放。
- 一批红在中途（例如容器起不来、磁盘满），停下、清理自己的容器与目录，报主持者，不换个办法再跑。

### 3.9 S4 起：条件的修复

**开一个修复 Step 的判据**：

1. 某一行照 3.6 判"超"（页面树照 4.3）。
2. 先复核：在没有别的负载的时段把超出的那一格（只那一格）再跑两轮。再一次超，才开 Step；不超就写"第一次超、复核过"，带三次的数，交主持者定（多半是机器的摆动）。页面树同样复核（审查 B 的 N7）：没过的那一格在同一种形状上再量两轮，约 10 分钟；它会开分层加载这样大的 Step，不凭一次。
3. B5 两层、B10 的 `index` 过 3.5 秒、B1 的尾巴与体积不走这条：B5 照 4.7.5 交主持者与负责人；`index` 是调守卫的界的小 Step；B1 的尾巴与体积先对照（3.6 的 B1）。B4、B7 的 × 1.25 那一行过期限而主行没过的，同样只列给主持者。
4. 负责人可以改判为已知限制（00 号文档 4.7.5）。

**修复 Step 的设计要写的**：

- **原因**：带证据（计划、剖面、读数），不靠推理；05-lessons 第 4 节的"量多种形状"照做。
- **改法与取舍**：几种做法、各自的代价，选哪一种；改到前面 M 的代码时照 00 号文档第 8 节逐项写。
- **缩小规模的守卫**（负责人 2026-11-06 的话"调整测试用例量级或者时限"）：
  - 规模：CI 一份里放得下的（例如 1 万页，而不是 10 万）；
  - 界线：本机的三倍，并对照 7763 上的实数（05-lessons 第 1 节"限时的测试"）；界高于测试配置的请求期限时，只给这个用例提高期限，并断言界低于期限、客户端等得比期限久（05-lessons、M12 决定 359）；
  - 进哪一份：照 13.4 第 8 条，`!race` 加成本或守卫的一份带 `-v` 的一行（`test-go-costs-*` 或 `test-go-guards-*`），写出那一份在 7763 上加多少秒；
  - 反向对照：把修复撤回，守卫红。
- **在原规模上再量**：同样的方法、种子与两轮，写出改前改后。
- **变异表**：每条写出抓住它的测试。
- **页面树**的改法照 00 号文档 4.7.2 的"结论"：数据一侧多半是分层加载，只有平的形状在渲染上没过多半是只渲染看得见的行；改完照同样的终点再量；P2/S6 排在它之后。两种候选的范围见第 9 节，改法的设计在结论之后另起（00 号文档 4.7.5），不在本文。

## 4. 浏览器一侧：`e2e/perf/`（S2）

### 4.1 文件

| 文件 | 内容 |
|---|---|
| `e2e/perf/playwright.config.ts` | `testDir: "."`、`testMatch: "**/*.perf.ts"`（字符串的 glob：knip 的 Playwright 插件丢掉正则，审查 C 的 M1）、`workers: 1`、`fullyParallel: false`、每个测试的时限 30 分钟、`globalSetup: path.join(import.meta.dirname, "setup.ts")`（绝对路径：knip 按 workspace 的目录解析相对的）、`outputDir: "../test-results/perf"`（在已忽略的 `e2e/test-results/` 之下，`.gitignore:27-28`）、`trace` 与 `screenshot` 关；一个 project：Chromium（Desktop Chrome，默认视口 1280 × 720），`locale: "en-US"`。主配置的 `testDir` 是 `stories`（`e2e/playwright.config.ts:7`），这些文件不进 `make e2e` |
| `e2e/perf/setup.ts` | 一轮的准备（4.2） |
| `e2e/perf/shapes.ts` | 三种形状与 5 万页、3,000 页的 vault，用 `e2e/fixtures/zip-write.ts:34` 的 `zipOf`；名字与正文由带种子的伪随机数生成 |
| `e2e/perf/probe.ts` | 经 `addInitScript` 进每个文档的计时脚本（4.3）：长任务的观察者、"视口里的行已画出"的条件、两帧之后取时刻 |
| `e2e/perf/record.ts` | 结果追加到 `NW_PERF_OUT`（默认 `e2e/test-results/perf/results.jsonl`）；中位数（最近秩的分位 S2 用不上，没写，4.11 第 1 条） |
| `e2e/perf/tree.perf.ts` | 页面树的 T1、T2、T3（4.3） |
| `e2e/perf/tree-sizes.perf.ts` | 5 万、10 万页只记的两样（4.9） |
| `e2e/perf/editor.perf.ts`、`e2e/perf/parse/entry.ts`、`e2e/perf/parse/build.ts` | B9（4.4）：解析页的入口与它的库构建，`editor.perf.ts` 在测试里调 `build.ts`（4.11 第 1 条），以 `import type` 引 `entry.ts` 导出的 `NwParse` |
| `e2e/perf/links.perf.ts`、`images.perf.ts`、`assets.perf.ts`、`jobs.perf.ts` | M4-performance 第 7、11、12 条，M7-transfer 第 4 条（4.5–4.8） |
| `e2e/package.json` | 加脚本 `"perf": "playwright test -c perf/playwright.config.ts"`（取舍 P4-20）；加 devDependencies `@codemirror/state`、`@codemirror/language`，版本与 `web/apps/web/package.json` 的相同（6.7.6、6.12.4，锁文件里已有，取舍 P4-13） |
| `e2e/fixtures/auth.ts` | `onboardingSteps`（`:149`）导出，`setup.ts` 用它记下引导的三步（4.2；审查 C 的 M3） |
| `knip.jsonc` | `workspaces` 加 `"e2e": { "ignoreDependencies": ["vite"] }`，中文注释写明解析页按路径载入 web 自己的 Vite、e2e 不依赖它（4.4；修订核对 C 的 M6） |

- `e2e/tsconfig.json` 的 `include` 是 `**/*.ts`，`pnpm -r run check:types` 看得到它们（`Makefile:164`）；oxlint 看 `e2e`（`package.json:5-10` 的 `check:lint`），oxfmt 也看。
- **换文档的门禁**走整个 `e2e/`（`tools/e2e-navigation/check.mjs:133-143`），连注释里的 `.goto(`、`.reload(` 也算（`:24`）。所以：
  - 冷打开经帮手的 `goto`（`e2e/fixtures/navigation.ts:97`）；每个页面在第一次导航之前 `watchPage`（`e2e/fixtures/browser.ts:133`，帮手对没被监视的页面直接失败，决定 341）；
  - B9 的解析页用 `page.setContent` 与 `addScriptTag`，不导航（审查 C 核过不触发门禁）；
  - 这些文件的注释里不写那两个方法的调用形式。
- **knip**：`e2e` 是 workspace 的包（`pnpm-workspace.yaml:2-5`）；knip 的 Playwright 插件从 `perf` 脚本的 `-c` 读到这份配置，入口是它的 `testMatch` 与 `globalSetup`；两个 devDependencies 由 `parse/entry.ts` 的导入用到，`entry.ts` 经 `editor.perf.ts` 的 `import type` 进图，`build.ts` 载入的 `vite` 由 `knip.jsonc` 的 `e2e` 一节忽略（4.4）。S2 的第一个提交以 `make knip` 核对，三步的反向对照（审查 C 实跑过同样的几种）：去掉脚本，knip 报出 `e2e/perf/` 的文件；`testMatch` 换回正则 `/\.perf\.ts$/`，knip 报出 `.perf.ts`（入口来自配置）；都复原，干净。读不到配置时才在 `knip.jsonc` 的 `e2e` 一节另加入口（2.1 第 7 条）。

### 4.2 一轮的准备（`setup.ts`）

一轮是一次 `cd e2e && pnpm perf`（经 `heavy.sh`）。

1. `startPostgres()`（`e2e/fixtures/db.ts:45`；`postgres:18.7-trixie`、builtin `C.UTF-8`、`max_connections=300`，不加 `fsync=off`）；建库、`runNervewiki(["migrate", "up"], …)`（`e2e/fixtures/server.ts:68`）。
2. `startNervewiki(url, log, { env })`（`server.ts:90`），`env` 盖过测试的配置：`NWIKI_ENV=prod`、JWT 的私钥（Node 的 `crypto.generateKeyPairSync("ed25519")` 导出 PKCS#8 PEM，写进本轮的目录）、取舍 P4-8 的三个桶、`NWIKI_LOG__FORMAT=text`。`nervewikiEnv`（`:139-155`）在前、`env` 在后，所以盖得过（`:101-106`）。日志格式要盖：生产是 JSON，帮手等的是文本的 `msg="notification listener listening"` 与 `msg="jobs started"`（`:170-171`），不盖就等到启动的时限失败（审查 A 的 M8）；S2 不读 gctrace，文本的日志不碍事。
3. 两个账户：`runNervewiki(["users", "create", …], url, { input })` 建两个；各自 `login`（`e2e/fixtures/auth.ts:35`），再以 `POST /api/v0/me/onboarding-steps` 记下 `onboardingSteps` 的三步（同 `registerOnboarded`，`:152-160`）：`users create` 不记引导，没走完引导的账户每个页面都被转到 `/onboarding`（`W/app/guards.tsx:77-85`；审查 C 的探针第一次就落在那里）。第一个建工作区与笔记本，笔记本以 `createNotebook(…, "editor")`（`e2e/fixtures/notebooks.ts:24-35`）建：新笔记本的 `workspace_access` 默认是 none，第二个账户只进工作区就新建不了页（审查 C 的 M3）；经邀请把第二个加进工作区（`e2e/fixtures/invitations.ts` 的帮手）。第一个是笔记本的管理员（左栏每行注册拖放与菜单，比读者重；团队的笔记本里多数人能写），量的标签页用它；第二个是 T2 里"另一个账户"。
4. 每种形状一本笔记本，`startImport`（`e2e/fixtures/transfer.ts:64`）导入 `shapes.ts` 的 zip；每 2 秒问任务到成功（`endedJob` 默认 15 秒，`:239`，不够）。一轮导入哪几种形状由 `NW_PERF_SHAPES` 定，账户、PAT 与生产配置的细处见 4.11 第 2 条。
5. 等后台安静：经 `openDatabase`（`db.ts:100`）问 `page_search_queue` 为空、River 没有 `running` 与 `available` 的任务，再空闲 30 秒。导入之后搜索的任务会占 CPU（M8：1 万页约 2 分钟清空），不等就量进去了。
6. 把地址、两个账户的邮箱与密码、笔记本的 id 写进本轮目录的 `perf-env.json`，测试读它、各自登录：令牌 15 分钟就过期（`config.yaml:46`），一轮远长于它（审查 C 的 N4；取舍 P4-8 放宽了登录的桶）。结束时 `stop`。

### 4.3 页面树（先量，结论先报主持者）

**界线**（00 号文档 4.7.2 照抄，决定 329；量之前不再改）：

> - **规模与形状**：1 万页，三种形状：
>   - 平：1 万页都在根下；
>   - 深：10 个根、各 10 个文件夹、各 100 页，另有一个 1,000 个孩子的文件夹；
>   - 带附件：深的形状另加一成附件（1,000 个）。
> - **环境**：本机，Playwright 自带的 Chromium，生产构建（`make build`），默认视口。不降 CPU，不加 CDP 的网络降速：走回环。
> - **三条界线**：
>   - **T1 冷打开**：从 `goto` 笔记本首页（已登录、HTTP 缓存清空）到树的答复已到、并且左栏视口里的行已画出，≤ 1 秒。终点用 `PerformanceObserver` 的 element timing，或视口内行数的 `waitForFunction`。
>   - **T2 别人改一次树**：另一个账户经 API 新建一页，名字按次序排在最前，落在视口里。从答复到这个标签页画出新的一行 ≤ 1.5 秒（含 refresher 的 500 ms，`web/…/events/handlers.ts:36`）；其间主线程最长的一个长任务 ≤ 200 ms。新页放不进视口时，量"store 已更新加视口重画"。
>   - **T3 展开**：展开那个 1,000 个孩子的文件夹，到视口里的孩子都画出 ≤ 500 ms。
> - **怎样判**：每条界线在每种形状上判。两轮，每轮 5 次取中位数，两轮的较大中位数对照界线。

**量法**（取舍 P4-11；2.1 第 4 条的更正）：

- **形状的确切的数**：
  - 平：10,000 页，都在根下，名字 `p00001 …` 起，按名字排；
  - 深：10 个根（`r01`–`r10`）、各 10 个文件夹页、各 100 页，加根下排在最前的 `000 big` 与它的 1,000 个孩子，共 11,111 页；
  - 带附件：深的形状，100 个第二层的文件夹页下各 10 个 1 KiB 的 PNG，共 1,000 个附件。附件在树的答复里、不在左栏里（`W/stores/page-tree.store.ts:174-176`）。
  - 导入按名字排同级（`.nerve/meta.json` 不写；`transfer/domain/import_plan.go:201-213`），`000 big` 因此排在根下最前。
- **页面与账户**：笔记本首页 `/{slug}/notebooks/{id}`（`W/app/routes.tsx:144`、`:151`；`e2e/fixtures/notebook-pages.ts:62-64`）。它除了左栏的树，还在正文里列一遍根下的页（`W/pages/notebook/notebook-home.tsx:66`，`SubpageList`）：平的形状要画两份一万行，这是产品今天的冷打开，照量。
- **"视口里的行已画出"的条件**（`probe.ts`）：
  - 行是 `nav[aria-label="Pages of <笔记本名>"]` 里每个 `li` 的第一个子元素（行的 `div`，`W/pages/notebook/page-tree-item.tsx:97-105`）。没有 ARIA 的 tree（`page-tree.tsx:29-34`）；左栏不自己滚动，视口是窗口的（`W/pages/workspace/workspace-layout.tsx:111-113`）。
  - T1：树里至少一行，并且（最后一行的 `top` ≥ `innerHeight`，即视口已铺满到底边；或行数 ≥ 这种形状根下的页数：同一轮里 T2 先跑过时根下多了几页，审查 C 的 N5）。
  - T2：根列表的第一行（新页以 `after_id: null` 排在最前）的链接文字等于新页的名字，并且与视口相交。只看第一行，不逐帧扫一万个链接（审查 C 的探针：扫一遍约 2.9 ms，会算进长任务的窗口）。
  - T3：展开的列表（展开按钮的 `aria-controls` 以 `document.getElementById` 取到的 `ul`，`page-tree-item.tsx:107-116`、`:167`；值来自 `useId`，不拼成 CSS 选择器）里（最后一行的 `top` ≥ `innerHeight`，或行数 ≥ 1,000）。
  - 计时脚本用 `MutationObserver` 加每帧检查；条件第一次成立时请两次 `requestAnimationFrame`，第二次的回调的 `performance.now()` 是"已画出"（那一帧的绘制已经过去）。另记条件第一次成立的时刻，两者都写进记录。每帧的检查约 0.46 ms（审查 C 量过）。
  - **不用 element timing**：它要元素带 `elementtiming` 属性，产品的行没有，为量去改产品不值。
  - **实际**：先数行数、画出之后才取位置；T2 名字对上就成立，与视口相交与否画出之后记下（4.11 第 3 条）。
- **T1**：
  - 零点是新文档的 `timeOrigin`（导航开始；P1 设计第 9 节：帮手在旧文档上的等待在零点之前，不算）；终点是上面的"已画出"，就是那个 `performance.now()`。
  - 每种形状每轮一个浏览器上下文，以第一个账户登录一次（`login`，再 `signInContext`，`e2e/fixtures/auth.ts:103-116`）。每次 T1 在这个上下文里开一个新的标签页：`watchPage`，经 CDP `Network.clearBrowserCache` 清 HTTP 缓存，再 `goto` 笔记本首页；量完经帮手的 `goto` 到 `about:blank`（帮手先等它自己的条件；它的 `settle` 没有导出，这样不改夹具，换文档的门禁也照过；修订核对 C 的 N9）再关。新标签页是新的渲染进程，渲染进程里 V8 按源码的编译缓存不跟着过来（审查 C 的 U1）。不每次新开上下文：每次登录要过 `login_ip_email` 的桶（默认每分钟 10、突发 5）与 argon2，一轮三十几次登录量不完。
  - 另记：树的答复（`/api/v0/notebooks/{id}/nodes` 的 resource timing 的 `responseEnd`）、`DOMContentLoaded`、主 JS 的 `transferSize`。
- **T2**：
  - 量的标签页停在笔记本首页；先把窗口滚到树的第一行在视口里（左栏随整页滚动；1280 × 720 下树之前有搜索框、工作区切换、几个分区与笔记本列表，第一行离底边多远实施时先核对，审查 C 的 U3），再等 1 秒没有长任务、没有在途的接口请求。滚过之后新页那一行仍不与视口相交的，这一次作废、记下；作废过半时改用 4.7.2 的退路"store 已更新加视口重画"，记录写明。
  - Node 一侧以第二个账户 `POST /api/v0/notebooks/{id}/pages`，体 `{"parent_id": null, "title": "000 T2 <轮>-<次>", "after_id": null}`（`after_id: null` 排在同级的最前，`openapi.yaml:3937-3938`；`e2e/fixtures/pages.ts:19-25` 的 `postPage` 原样发体）。记下答复到达时 Node 的 `performance.timeOrigin + performance.now()`。
  - 页内"已画出"取 `performance.timeOrigin + performance.now()`；两者之差是 T2（同一台机器的同一个时钟，审查 C 量过两边差 0.37–0.62 ms）。
  - 长任务：页里一直开着 `PerformanceObserver({ type: "longtask", buffered: true })`，取开始在 [答复, 已画出] 之间的长任务的最大时长；没有长任务时记 0。
  - 两次之间隔 3 秒（refresher 末尾的那次读要落完，`W/events/refresher.ts:59-77`）。新建的页不删（删又是一次改树）；一轮多 5 页，相对一万页不计。
- **T3**（深与带附件两种形状；平的没有文件夹）：
  - 起点是展开按钮的 `click` 事件的 `timeStamp`（页内一个捕获阶段的监听者记下；与 `performance.now()` 同一个零点）。`000 big` 在根下最前，展开之后它的孩子从视口上部往下排。
  - 终点是上面 T3 的"已画出"。两次之间收起、等 1 秒。
- **每轮**：每种形状 T1 五次、T3 五次（深的两种；在标签页打开之后、T2 之前量，事后断言视口里的孩子 ≥ 1，决定 515，4.11 第 4 条）、T2 五次。
- **只记的**（4.7.2"只记的"，取舍 P4-12）：
  - 树的答复的 JSON 字节，与 Node 的 `zlib` 在 gzip 1 级、6 级与 zstd 3 级下的字节（同一份答复，各压一次）；
  - 每种形状第五次 T1 之后，CDP `HeapProfiler.collectGarbage`，再 `Performance.getMetrics` 的 `JSHeapUsedSize`；
  - T1 之内最长的一个长任务（审查 C 的探针里约 2.1–2.3 秒），与首页正文的 `li` 数；
  - 深的形状里，对 `000 big`（1,000 个孩子）打开"移到……"对话框到列表画出的时间，五次；T1 的一次 CDP `Profiler` 剖面里 `subtreeOf` 的自身时间。[P2 设计](02-P2-web-polish.md) 4.6.6 末的附带观察：对话框对树里每一页调 `canHold`（`W/pages/notebook/move-page-dialog.tsx:120`），每一行渲染算一次 `subtreeOf`（`page-tree-item.tsx:274`）；推断，没有界线，只记。
- **怎样判**：每条界线每种形状：每轮 5 次的中位数，两轮的较大中位数对照界线；T2 的长任务取每次的最大值，同样每轮中位、两轮较大，对照 200 ms。没过的那一格照 3.9 第 2 条再量两轮复核。
- **结论**（00 号文档 4.7.2）：
  - 三种形状都过：保留整棵树；5 万、10 万页的数（4.9）记成已知限制，分层加载进 v0.2。
  - 有一条没过：S2 的实施者先停下其余各项，把表报主持者；主持者照 4.7.2 定改法、开条件的 Step（3.9），P2/S6 排在它之后。两种候选的范围见第 9 节。
- **报主持者的表**：三种形状 × T1、T2、T2 的长任务、T3，每格两轮的中位数与五次的数、对照的界线、过或超；另附三种形状的 JSON 字节与堆、T1 之内最长的长任务。

**量法的反向对照**（S2 实施时各做一次，写进记录；审查者重跑，7.3）：

1. **零点**（审查 C 的 I1）：一次 T1 经路由把主 JS（`/assets/index-*.js`）的答复压后 1,000 ms，T1 多 1.0 ± 0.15 秒。零点错取在 `goto` 返回之后时，压后的那一段落在零点之前（`goto` 等 `load`），T1 不变，这条对照失败。初稿的"T1 ≥ `domContentLoadedEventEnd`"抓不到这种错：审查 C 的探针里错的零点下五次都照过。
2. **终点看得见树的答复**：一次 T1 把 `/nodes` 的答复经路由压后 1,000 ms，T1 多 1.0 ± 0.15 秒。
3. **长任务的观察者**：一次 T2 在答复之后往页里放一个 300 ms 的忙循环，长任务的最大值 ≥ 300 ms。
4. **T2 的时钟**：一次 T2 把量的标签页对 `/nodes` 的读压后 500 ms，T2 多 0.5 ± 0.1 秒。容差实施时先核对：一万行上的重画本身可能摆动近 0.1 秒（审查 C 的 U4），先不压后量五次 T2 看极差，极差过 0.1 秒时容差取极差加 0.05 秒、记录写明。
5. **缓存清了**：每次 T1 断言主 JS 的 `transferSize` > 0；不清缓存时它是 0，断言失败（审查 C 量过：清了 225,583 字节）。
6. **T3 的条件**：对一个收起的文件夹等"视口里的孩子已画出"，5 秒之内不成立（没有误报）。

### 4.4 B9：编辑器的解析（取舍 P4-13）

- **预算**：段落、标题每次解析至多约 80 ms（`v0.1-design.md:1228`）；编辑器的上限是一段之内的行内内容 10,000 个 UTF-16 码元（`W/editor/markdown.ts:46`）、嵌套 100 层（`:55`），注释写"在上限处最贵的约 80 ms"（`:28-45`）。
- **形状**：`W/editor/markdown.test.ts` 量过的那些，各放大到上限：一段之内 10,000 码元的 `*a** `、`***a*** `、配不上的 `]`、`[a](b (x `、长度各异的反引号串、一串空格、`<?`、自动链接；100 层的列表与引用；长的引用式定义；有 `|` 的一行之后一行空白（移交第 9 条的十几种，`M4-performance.md:358`）。实际另加四种多行的引用式定义，列表与引用量到 49、98 层，共 23 种（4.11 第 5 条，10.2）。
- **解析页**（审查 C 的 M2 实跑过这条路，606 KB 的 IIFE）：
  - `e2e/perf/parse/entry.ts` 导入 `markdownEditing()` 与 `inlineLimit`（`W/editor/markdown.ts:46`、`:127`；它另导出 `blockDepth`，第二次修订写的"只导出它"不对）与测试用的同一组导出：`@codemirror/state` 的 `EditorState`、`@codemirror/language` 的 `ensureSyntaxTree`（`markdown.test.ts:1-6`）；把"整篇解析"与"插一个字之后的增量解析"两个函数挂在 `window.nwParse` 上，并导出它的类型 `NwParse`。增量解析是先整篇解析，再 `state.update({ changes }).state` 加 `ensureSyntaxTree`，CodeMirror 自己按变更复用片段，同编辑器里的做法，不另要 `@lezer/common`（修订核对 C 的 M6）。不为 P4 改产品。
  - `editor.perf.ts` 以 `import type { NwParse } from "./parse/entry"` 引它：knip 只顺着导入走，`entry.ts` 只被 `build.ts` 按路径交给 Vite 时，knip 报 `Unused files: e2e/perf/parse/entry.ts` 与 `Unused devDependencies`（修订核对 C 的 M6，探针）。
  - `e2e/perf/parse/build.ts` 从 `web/apps/web/package.json` 解析出 web 的 `vite`（`createRequire(…).resolve("vite")` 再动态 `import`），所载模块的类型手写（`{ build: (config: object) => Promise<unknown> }`：写 `typeof import("vite")` 时 e2e 的 `tsc` 报 `TS2307`，e2e 不依赖 `vite`），以库构建打 IIFE：`configFile: false`、`root` 是 `web/apps/web`、`build.lib` 的入口是 `entry.ts`、`formats: ["iife"]`、不压缩、`copyPublicDir: false`，输出到 `e2e/test-results/perf/parse/parse.js`。不用 `resolve.alias`：e2e 的两个包与 web 的 CodeMirror 是同一份存储，包里只有一份 CodeMirror；第一次修订写的别名会失败（`@lezer/common` 不是 web 的直接依赖，`web/apps/web/node_modules/@lezer/` 下没有它，构建报 `UNLOADABLE_DEPENDENCY`）。web 自己的 `vite.config.ts` 分块（`:22-39` 的 `codeSplitting.groups`），打不出 IIFE；它的 `outOfMain()` 插件拦的正是入口静态导入编辑器的形状。
  - e2e 加两个 devDependencies（4.1）给 `tsc` 与 knip；不加时 `tsc` 报 `TS2307`、Vite 解析不了（审查 C 的探针）。`knip.jsonc` 的 `e2e` 一节 `ignoreDependencies: ["vite"]`，注释写明构建用的是 web 自己的 Vite（不忽略时 knip 报 `Unlisted dependencies: vite`）。
  - 实跑过（修订核对 C 的 `out-fix2/`：三个包加 `TreeFragment` 的写法；第二次修订的探针：上面的两个包加 `state.update` 的写法，在 `1ba6c7f6` 的临时 worktree 里）：`tsc`、knip、oxlint 都是 0，构建与 Chromium 里的解析通过；20 万个空格整篇解析 4.7–5.1 ms。第二次修订的探针记下的"插一个字之后 1.3–1.5 ms"是在过了上限的段上插的，量到的是纯文本的快路，不作数（修订核对 C 的 M8；修订核对 C 的探针照字面重搭，结果相同）。改动是 `e2e/package.json` 5 行、`knip.jsonc` 4 行、锁文件 6 行。
  - 测试在一个空页里 `page.setContent`、`addScriptTag` 它。
- **量**：每种形状两格，一页里只有这一段加前后各一段普通的；这一段以 `markdown.test.ts:49-50` 的 `inline()` 用的头（一个链接、一处强调、一段行内代码）起，后接这种形状：
  - **整篇解析**：这一段正好 `inlineLimit`（10,000 个码元）。量一次解析，走粘贴的路子（4.11 第 5 条）。
  - **插一个字之后的增量解析**：这一段取上限减一个码元，先整篇解析，再在段中间插一个字（实际另有段尾一格，两格都判，4.11 第 5 条），插后正好到上限（`state.update` 加 `ensureSyntaxTree`，上一条）。不取上限处再插（修订核对 C 的 M8）：插了就是 10,001 个码元，`LongInlineAsText` 判 `cx.end - cx.offset > inlineLimit`（`W/editor/markdown.ts:99`），整段变成纯文本，量到的只是过了上限的那条快路；C 的探针里上限处插一个字是 0.0–6.4 ms，上限减一是 12.7–110 ms。
  - **每次量完都断言**这一段量的是上限之内的解析：段长等于 `inlineLimit`（`entry.ts` 从 `markdown.ts` 导入它），解析树里这一段之下有 `Link`、`Emphasis`、`InlineCode` 三种节点（来自那个头；同 `markdown.test.ts:39-58` 的 `inlineNodes`：变成纯文本时一个也没有）。`window.nwParse` 的两个函数交回用时、段长与这三种节点的名字。
  - 各热身两次、量 5 次，每轮一个新的浏览器；两轮。实际：每种形状的每一格一个新的页、先收一次垃圾，热身两次、量 20 次（决定 512，4.11 第 5 条）。
- **统计量与判据**：每种形状每种解析，每轮 5 次（实际 20 次）的中位数，两轮的较大 > 80 ms 为超。本机的 Chromium，不乘（在用户的机器上跑，00 号文档 4.7.1）。
- **只记**：在真的编辑器里，三种最贵的形状各按一个键：`keydown` 的 `timeStamp` 到下一帧（两帧）的时间与其间的长任务。
- **反向对照**：解析页对 20 万个空格的一段整篇解析要在 1 秒之内；换成没有上限的配置（默认的 `@lezer/markdown`），这条断言失败（一串空格没有上限时按长度的平方涨，96 KB 进入编辑要 5.6 秒，`markdown.ts:36`；审查 C 的 N6：`markdown.test.ts` 的十几种不都在没有上限时过 1 秒，所以写定这一种）。
- **反向对照二**（修订核对 C 的 M8）：增量的那一格改成上限处再插（形状取上限，插后 10,001 个码元），段长与行内节点的断言都失败；复原，照过。S2 实施时做一次，写进记录。
- **解析页打不出来时**仍给结论：先换一条路，入口放到 web 的 `build/` 下（已在 web 的 `tsconfig` 里，knip 给它一个入口），以同样的库构建打；两条路都打不出来，用真的编辑器里按一个键到下一帧作解析的上界：≤ 80 ms 判"过"；> 80 ms 时以 Node 跑同一个入口的数判，写"推断"（V8 的版本与 Chromium 的不同，对长字符串的哈希可能不同，05-lessons 第 5 节），≤ 80 ms 判"过（推断）"、> 80 ms 判"超（推断）"，都列给主持者。审查 C 已打出过，后两种多半用不上。

### 4.5 M4-performance 第 7 条：补全读链接目标（只记）

- 在 1 万页（平的形状）与 10 万页（4.9）的笔记本上：Node 一侧 `GET /api/v0/notebooks/{id}/link-targets`（`W/services/linking.service.ts:69-76`）的字节与用时，各 5 次；浏览器里在一页的编辑器里打 `[[`，`keydown` 到补全的列表画出（两帧）的时间，各 5 次（10 秒之内复用，`W/editor/loaded/link-completion.ts:73`，每次换一处 `from`）。两轮，记中位数。10 万页浏览器一侧量不到（4.11 第 6 条）。

### 4.6 M4-performance 第 11、13 条：阅读视图的大图（只记）

- **在 P2/S4 合并之后量**（[P2 设计](02-P2-web-polish.md)第 9 节、4.4.2；决定 424）：P2 的 U3 之后，每小时的重读不再换节点，但图片仍改 `src`、重新下载，量的是这一种。S2 量到这一节时 P2/S4 还没合并，就把它挪到 S3：S3 开始时在被量的提交上照这一节跑 `images.perf.ts` 两轮（经 `heavy.sh`，约 10 分钟），记录写在 10.3。S2 时 P2/S4 没合并，照此挪了（决定 518，4.11 第 7 条）。
- 一页 20 张 4,000 × 4,000 的 PNG（简单的图样，文件小、解码之后每张约 64 MB），经附件上传。阅读视图里图片 `loading="lazy"`、带宽高（`platform/markdown/obsidian/asset.go:115`、`:136-145`）。
- 量：打开这一页、以 200 px 的步子滚到底，经 CDP `SystemInfo.getProcessInfo` 找到渲染进程的 id，每 100 ms 读它的 `/proc/<id>/status` 的 `VmHWM`，记峰值；另记 `JSHeapUsedSize`。两轮。
- **实施时先核对**（审查 C 的 U2）：Chromium 解码的图片可能在 GPU 进程或可丢弃的共享内存里，渲染进程的高水位未必涨。先开一张图看渲染进程与 GPU 进程的 `VmHWM` 谁涨；都不涨就只记 `JSHeapUsedSize` 与图片的字节，不追（审查 C 的过度设计一段）。
- 第 13 条（每小时整篇替换的带宽）推算：这一页的图片字节 × 开着它的标签页数，每小时一次。

### 4.7 M4-performance 第 12 条：附件列表的重读（I2-D10 的判据）

- **复测移交的那一次**：根下 3,000 页（一次导入）；标签页 A 停在笔记本首页（左栏根下 3,000 行，首页的附件一节读根的附件，`W/pages/page/attachments-section.tsx:76`）；同一登录的标签页 B 在另一页的编辑器里打字；上传方三个 PAT（单个 PAT 的桶 20 秒至多约 600 次）20 秒里往根下传 1,200 个 1 KiB 的 PNG。
- **实际**（决定 513，4.11 第 8 条）：判据取 A 持全部页的情形，上面的情形也量；B 跟上传窗口一起停手；每次复测一本新的笔记本。
- **B 怎样打字**（审查 C 的 I2）：自动保存是去抖的，正文 2 秒不变才存（`W/editor/autosave.ts:4`、`:18-23`），每秒一个字在 20 秒里一次也不存。所以 B 每打一个字停 3 秒，20 秒的上传窗口里约 6–7 次保存；失败的保存不自己重试，等下一次停顿（`autosave.ts:5-14`），B 一直在打，下一次停顿就是重试。
- **量**：A 发出的附件列表的读（`GET …/assets` 的请求数）与其中的 429；A 的长任务之和占 20 秒的比例；B 的每次保存：发出的时刻、状态、在上传窗口里的位置；同一凭证的桶上 A、B 各自的 429。
- **一次复测有效**：至少 5 次保存落在上传窗口里；不到就作废、重来，作废的次数也记。
- **"保存失败"**：B 有一次保存答 429 之后 10 秒之内没有一次成功的保存（之后的保存带着全部正文，成功一次就补上了）；或结束时这一页的正文（`GET …/content`）少了打的字；或复测结束、B 最后一次停顿之后的那次保存答完时，编辑器仍显示保存失败。中途答一次 429、下一次就成功的不算：编辑器显示的 `problem.rate_limitedFor`（`W/app/problem-messages.ts:128-129`）要到下一次保存成功才消失，窗口里有 6–7 次保存，凡显示过就算会让量法偏向 A 类；移交第 12 条记的"拒一次、之后成功、没丢字"不算失败（I2-D10；修订核对 C 的 M7）。
- **重复与判据**：每轮 5 次有效的复测，两轮。任一轮 5 次里至少 3 次有"保存失败"，算"能稳定让用户的保存失败"（I2-D10）：照 A 类开条件的 Step（3.9，先照它的第 2 条复核），改法是移交写的"先比较 id 与名称再读"。否则记成已知限制，带每次复测的保存数与 429 数。

### 4.8 M7-transfer 第 4 条：任务列表的轮询（只记）

- 设置页的任务列表在有运行中的任务时每秒读一次（`W/pages/notebook/transfer-page.tsx:46-56`、`:180`）。准备 60 个结束了的小导出（让列表有两页），再开一个长的导出：导出 4.9 的 5 万页笔记本（推断几分钟，60 秒的窗口里一直是进行中；不够长时记下、照它的用时缩短窗口，审查 C 的 N7）；N = 1、3 个标签页停在这一页、各点一次"加载更多"。
- 量：每个标签页每分钟的请求数与 429，60 秒。两轮。推算到每凭证 1,200 次每分钟的桶要几个标签页、几页。
- **实际**：长任务用导入，不用导出（4.11 第 9 条）。

### 4.9 5 万、10 万页（只记）

- 平的形状：5 万页一次导入；10 万页两次导入（每次 5 万，至多 6 万个节点）。
- 记树的答复的字节（不压缩与三种压缩）与冷打开之后的 `JSHeapUsedSize`；T1 也记一次（只记，至多等 120 秒，超了记"120 秒之内没画出"）。一轮。10 万页的那一格渲染进程崩溃，记下崩溃的时刻（4.11 第 10 条）。

### 4.10 S2 的机器时间

估计是**推断**。每轮的准备约 12 分钟（三次 1 万页上下的导入、等搜索的队列），页面树的量约 8 分钟（每次 T1 开新的标签页）；两轮约 40 分钟，没过的那一格复核约 10 分钟，之后先报主持者。其余：5 万、10 万页的导入与只记的量约 40 分钟，B9 的构建与量约 20 分钟，第 7 条约 10 分钟，第 11 条约 10 分钟（挪到 S3 时随之），第 12 条约 35 分钟（有效的复测，作废的重来），轮询约 15 分钟。合计约 3–3.5 小时，经 `heavy.sh` 持锁，一个傍晚。实际见 4.11 第 11 条。

### 4.11 S2 实施与审查之后与本文不同之处

S2 照实做的与上面写的不同之处（实施中定的经主持者接受，决定 516；审查改的各有决定号）。上面各节在相应处注了"4.11 第 n 条"，数在 10.2，理由与出处在 [S2 的审查记录](reviews/P4-S2-browser-measures-review.md)。

1. **提交**（4.1、6.2）：`probe.ts` 与 `record.ts` 的 `median` 在第 2 个提交（第 1 个提交里没人导入，knip 报，它单独不绿）；`record.ts` 没有最近秩的分位（S2 用不上）；B9 的解析页在测试里构建，不在 `beforeAll`；`tree-sizes.perf.ts` 在第 4 个提交改成记下渲染进程的崩溃（第 10 条）。实施 5 个提交（第 5 个是主持者看过第一次报告之后要的：附件列表持全部页的测试），修复三轮 7 个。第一次推送之前跑 6.4 的四项门禁；之后只改 `e2e/perf/` 的提交只跑 `make lint gen-check knip build` 与 `make test-go-short`，分支 CI 跑全套（决定 517）。
2. **一轮的准备**（4.2）：一轮导入哪几种形状由 `NW_PERF_SHAPES` 定（默认平、深、带附件，空的不导入；5 万、10 万页也经它），形状不在本轮的测试跳过并写明变量；`users create` 与 `migrate up` 也用生产的配置（测试配置的 argon2 是最低的）；准备与 `assets`、`jobs` 两个测试经自己的 PAT（导入、等安静与上传可能长过访问令牌的 15 分钟），其余测试用登录的访问令牌。
3. **页面树的探针与判法**（4.3）：先数行数、画出之后才取位置（照条件的字面次序，会在 React 提交之后的微任务里强迫排版，"成立"记在排版之后；第一遍的两轮与复核作废重量，平的 T1 没变）；T2 名字对上就成立，与视口相交与否画出之后记下、不相交的作废（40 次没有一次作废）；T1 每次事后断言行数；反向对照是 `tree.perf.ts` 里的一个测试，`NW_PERF_CONTROLS=1` 才跑；"移到……"对话框与 `subtreeOf` 的剖面只在深的形状上量。
4. **T3 在标签页打开之后、T2 之前量**，事后断言视口里的孩子 ≥ 1（4.3"每轮"；审查 B 的 M1，决定 515）：原来在五次 T2 之后量，`000 big` 已被新页挤到根下第 6 行、孩子全在视口之下，终点一画出列表就平凡地成立。
5. **B9**（4.4；审查 B 的 I1、I2、M2、N3，核对 B 的 C1-M1、C2-I1，决定 512）：
   - "整篇"量一次解析：空文档的 state 以一个事务换成整篇，`ensureSyntaxTree` 到文末（粘贴的路子），计时只包这一步。`EditorState.create` 解析到 3,000 个码元就停，跨过那里的一段下一次要整段再解析，原来量到的是两次之和。另只记解析器在 CodeMirror 之外一次解析作对照。
   - 插一个字分段中、段尾两格，都照 80 ms 判：一串空格类的代价随连串长度的平方涨，段中插只量到约一半。两行的定义取第一行的行尾，闭合的定义取收尾引号之前。
   - 形状加 `markdown.test.ts` 计时量过的三种多行、不闭合的引用式定义（括号没闭、标题没闭、JSON 数组）与"标题在最后一行闭合"一种，都放大到段长恰上限、行尽量短。"上限之内"另写：没闭合的三种段长恰上限、没有 `LinkReference`（由 `W/editor/markdown.ts` 的 `LongReferenceAsText` 保证读到最后一行）；闭合的一种 `LinkReference` 恰 1 个、长恰上限。定义的几种以标签开头，没有头、不断言三种行内节点；有 `|` 的一行那一种加了头。
   - 列表与引用量到 49 层、98 层：50 层列表、99 层引用里那一段已在 `blockDepth` 处，是纯文本。
   - 每种形状的每一格一个新的页（`setContent` 加 `addScriptTag`，再经 CDP `HeapProfiler.collectGarbage`），热身 2 次、量 20 次取中位；每轮一个浏览器上下文。定义的几种一次解析造几十 MB 的垃圾：同一页里先做过的解析抬高读数，新页里头几次又便宜，5 次的中位偏低。
   - 真编辑器的按键另记光标的位置（第几行、行内偏移与行长）。
6. **补全**（4.5）：10 万页的笔记本里打开一页，渲染进程崩溃，浏览器一侧量不到，记"not measured"、不失败；只记 Node 一侧。
7. **大图**（4.6）：S2 时 P2/S4 没合并，挪到 S3（决定 518）；`images.perf.ts` 数 `complete` 且 `naturalWidth > 0` 的图（字段 `loaded`；加载失败的图也是 `complete`，审查 C 的 M5），S3 的记录写"加载并画过"。
8. **附件列表的重读**（4.7；审查 C 的 I1，决定 513）：移交那一次的"根下展开到 3,000 行"是附件列表展开到 3,000 项（30 页），重读时读回已持的全部页；判据取 A 持全部页的情形（`assets.perf.ts` 的 "every page held"：每次复测先往根下传 3,000 个附件，A 的附件一节点"More attachments"到底），本节写的情形（只持第一页）也量。B 跟上传窗口一起停手，最后一次停顿在窗口里（原来打到窗口之后 3 秒，多出的一次保存替被拒的那次补上）；每次复测另记最后一次保存的状态与时刻。每次复测一本新的笔记本（上一次传的 1,200 个附件留在列表里，起点就不同）。
9. **任务列表的轮询**（4.8）：长任务用导入，不用导出 5 万页：导出在 1 秒之内做完，而且那本笔记本的五万行树在每个标签页的左栏里，打开时主线程被占约 19 秒、堆约 3 GB。每个 N 一本新的笔记本：60 个小导出之后导入 5 万页都在一个文件夹之下的包（`shapes.ts` 的 `folder50k`），左栏的根下只有两行。
10. **5 万、10 万页**（4.9）：10 万页的那一格渲染进程崩溃，记下树的字节与崩溃在请求地址之后多少毫秒。
11. **机器时间**（4.10）：审查之前量的持锁约 1 小时 55 分，修复三轮另约 70 分钟（10.2 末）。

## 5. e2e

- 不加故事，不改故事（00 号文档 9.3）。
- `e2e/perf/` 照第 4 节守门禁、帮手与 `watchPage` 的规矩；门禁的表不加逃生口（perf 没有有意在请求途中离开的）。
- `e2e/fixtures/` 只改一处：`auth.ts` 的 `onboardingSteps` 导出（4.1；审查 C 的 M3），`registerOnboarded` 照旧用它，故事不受影响。S2 另用 `startNervewiki` 的 `env`、`runNervewiki` 的 `input`、`zipOf`、`startImport`、`login`、`signInContext`、`createNotebook` 与邀请的帮手，都已有。
- `e2e/package.json` 加 `perf` 脚本与两个 devDependencies（4.1），锁文件随之；版本同 web 的，不引入新的解析。`knip.jsonc` 加 `e2e` 一节，只忽略 `vite`（4.4）。

## 6. 实施步骤

每个 Step 一个分支，从开工时的 `main` 开始；分支上的每个提交都绿。提交的写法照 00 号文档第 7 节"提交说明"。

### 6.1 S1（`m12-p4-s1`，P1 之后可开）

1. `server, build: a synthetic search corpus for BenchmarkSearch, its words from the repository's own text, its queries' words planted exactly (M12/P4/S1)`：`perf_corpus_test.go`（`!race`）、两份词表（这个提交里由 `TestPerfVocabulary` 生成，文件头写这个提交的父）、三个测试、`NW_PERF_CORPUS_SCHEMA=308eafc6` 的开关、`Makefile` 的 `test-go-costs-3` 加一行（3.2）。
2. `server: lint's cost guard's worker deadline above its bounds, as decision 359 has it (M12/P4/S1, M10 story)`：lint 的守卫另有 12 秒与 6 秒，夹具多一个 worker 的参数，断言界低于期限（3.3；取舍 P4-23）。
3. `server: the cost guards and the long log take their size and deadlines from one function, by hand, and then only log (M12/P4/S1)`：`perf_scale_test.go` 的 `guardScaleOf`、`judge`、四个守卫的默认值、`timedAsk` 与短测试；四个守卫改从那里取、断言自己的依赖，计时与随规模变大的请求经 `timedAsk`，第 2 个提交的 lint 的两个常量并进 `lintCostScale()`；`history_cost_test.go` 与 page 的 `perf_scale_test.go`（3.3）。
4. `server: the trash benchmarks' levels by hand (M12/P4/S1)`：`benchLevels` 与它的测试，`BenchmarkTrashRestore`、`BenchmarkTrashRestoreHistory` 读三个档位开关（3.3 末）。
5. `tools: perf's quantiles and samplers: a process's and a container's memory, the runtime's gc trace (M12/P4/S1)`：`stats/`、`sample/`、`main.go` 的 `sample`、`gctrace`（3.4.2）。
6. `tools: perf's vault, an import at its limits from a seed (M12/P4/S1)`：`vault/` 与子命令（3.4.3）。
7. `tools: perf's client and drivers: large bodies, MCP's large calls, editors, tabs, tree operations, imports and exports (M12/P4/S1)`：`client/`（含经 `docker exec` 的 `psql`）、`drive/`（3.4.4、3.4.5）。
8. `tools: perf's stack, a production serve over its own PostgreSQL container (M12/P4/S1)`：`stack.sh`、`doc.go`（3.4.1）。
9. **小规模的试跑**（不提交，结果写进 S1 的记录）：
   - `stack.sh up`，每个驱动以最小的参数跑一次（`bodies -k 2 -for 5s`、`mcp -reps 1` 的每种形状、`editors -n 2 -for 70s`（含一次换会话）、`tabs -k 1 -for 10s`、缩小一百倍的 vault 的 `transfer` 与 `ops`），`gctrace` 读这一次的日志，`stack.sh down`；`stack.sh up --fsync-off` 起停一次；
   - 语料 1,000 页喂给今天的 `BenchmarkSearch` 一次；`NW_PERF_CORPUS_SCHEMA=308eafc6` 的 1,000 页在 `308eafc6` 的临时 worktree 里喂给那时的 `BenchmarkSearch` 一次（审查 B 的 I2），用完删；
   - 四个守卫各以 `NW_SCALE_PAGES=20000` 跑一次，日志带 `scale=`、不判界；批准的守卫的日志另有播种的发起与提交的用时，记进记录（修订核对 A 的 M12）；3.3 末的手工反向对照；
   - `NERVE_BENCH_LOADS=full NERVE_BENCH_ATTACHMENTS=1000` 的 `BenchmarkTrashRestore` 以 `-benchtime 1x` 跑一次，日志里只建了一档。
10. `docs: … (M12/P4/S1)`：审查记录、10.1、00 号文档、总体设计 13.4 第 8 条与第 15 节、移交（第 8 节）。

**每个提交单独为绿**：`cd server && go build ./... && go vet ./...`、`make lint-go`（含 `server/tools` 与 `go mod tidy -diff`）、`make test-go-short`、`go -C server/tools test -race -count=1 ./...`；第 1 个提交另经 `heavy.sh` 跑新加的那一行 `test-go-costs-3` 与 7.4 的分份核对，第 2、3 个提交另经 `heavy.sh` 跑改到的守卫各自在 `Makefile` 里的那一行（`costs-1`、`costs-2`、`costs-lint`、`page` 的两个日志测试）。

### 6.2 S2（`m12-p4-s2`，P1 之后可开，与 S1 并行）

1. `e2e: perf's own Playwright run on a production serve, with notebooks imported in three shapes (M12/P4/S2)`：配置、`setup.ts`、`shapes.ts`、`probe.ts`、`record.ts`、`e2e/package.json` 的脚本、`auth.ts` 导出 `onboardingSteps`（4.1、4.2）。knip 的三步反向对照在这个提交之前做（4.1）。
2. `e2e: the page tree's three bounds and its sizes measured in Chromium (M12/P4/S2)`：`tree.perf.ts`、`tree-sizes.perf.ts`（4.3、4.9）。
   - 之后：4.3 的反向对照一次（第 4 条先核对容差）；页面树两轮；没过的格复核（3.9 第 2 条）；**报主持者**（4.3"报主持者的表"）。主持者转给 P2 的 S6。
3. `e2e: the editor's parse at its limits measured in Chromium (M12/P4/S2)`：`editor.perf.ts`、`parse/entry.ts`、`parse/build.ts`，`e2e/package.json` 的两个 devDependencies 与锁文件，`knip.jsonc` 的 `e2e` 一节（4.4）。
4. `e2e: link targets, large images, the attachment list's rereads and the jobs' polling measured (M12/P4/S2)`（4.5–4.8）。大图的量在 P2/S4 合并之后，没合并就挪到 S3（4.6）。
5. 其余的量（4.4–4.9），各两轮（4.9 一轮）。
6. `docs: … (M12/P4/S2)`：审查记录、10.2、00 号文档、移交（第 8 节）。

**每个提交单独为绿**：`make lint-web`（门禁、oxlint、oxfmt、`tsc`）、`make knip`；第 1 个提交另跑 `make test-web`（`auth.ts` 的导出）。S1 与 S2 都开着时，后合并的先 `git merge main`。实际的提交与门禁见 4.11 第 1 条。

### 6.3 S3（`m12-p4-s3`，P3 全部合并、S1 与 S2 合并之后）

1. 2.2 第 1 条的回答在手；被量的提交是开分支时的 `main`，在它的 worktree 里 `make build`。
2. 3.5.4 的 CI 日志与本机守卫的比；批次 B、C、D、E、F（3.8 的次序与时段），B′ 与 D 的候选看条件，G 推算，H 照回答；S2 挪来的大图（4.6）在开始时跑。
3. 有超出的照 3.9 先复核那一格。
4. `docs: … (M12/P4/S3)`：审查记录、10.3、00 号文档、总体设计第 14 节的十行与 13.4 第 8、9 条、移交（第 8 节）。

### 6.4 合并的门禁

02-workflow 第 3.1 节：`make lint gen-check knip build`、`make test`、`make test-go-short`、`make e2e`（经 `heavy.sh`；`TMPDIR` 放 `~/.cache` 下自己的目录）；分支的 CI 全绿（`ci-wait.sh <sha>`）；合并之前看一眼 `vuln.yml` 在 `main` 上最近一次的结论（00 号文档第 7 节 P1 的说明）。S3 只改文档：链接检查（`check_links.py`）与分支的 CI。

## 7. 测试与验证

### 7.1 各项与它们的测试

| 项 | 测试 | 在哪跑 |
|---|---|---|
| 语料是它的种子的；`308eafc6` 的列 | `TestPerfCorpusIsItsSeed` | `test-go-costs-3` 的新一行；`test-go-short`（不连库） |
| 查询的词确切地植入、词表不含它们 | `TestPerfCorpusPlantsTheQueries` | 同上 |
| 语料载得进基准、守住不变式 | `TestPerfCorpusLoadsIntoTheBenchmark` | `test-go-costs-3` 的新一行（`-short` 下跳过） |
| 规模开关、判界、四个守卫的默认值、lint 的期限、档位开关 | `TestGuardScaleKeepsEachGuardsOwnUnlessGiven`、`TestLevelsOfTheTrashBenchmarksAreTheirOwnUnlessGiven` | 组合根第四份（竞态）与 `test-go-short` |
| 按页的日志的规模 | `changesetsOf` 的测试 | `test-go-race` 一份与 `test-go-short` |
| 守卫照旧判界、lint 的界低于期限；守卫的期限与客户端接的是表里的数 | 四个守卫自己（判界在开关没设时；依赖的断言设不设都做） | `costs-1`、`costs-2`、`costs-lint` 原来的行 |
| 分位、采样、gctrace、vault、驱动、`psql` 的执行器 | `server/tools/perf/...` 的单元测试 | `costs-3` |
| 守卫的界真的被判 | 3.3 末的手工反向对照 | S1 实施时一次 |
| 页面树的量法 | 4.3 的六个反向对照 | S2 实施时一次，审查者重跑 |
| B9 的解析页 | 4.4 的反向对照 | 同上 |
| knip 读到 `e2e/perf/` | 4.1 的三步反向对照 | S2 的第一个提交之前 |
| 驱动对真的 serve | 6.1 第 9 步的小规模试跑 | S1 实施时一次 |

### 7.2 变异

每条在本机改坏代码、跑旁边写的测试、看到失败，再从备份复原（02-workflow 第 3.1 节；`mut.py`，`server/tools` 里的设 `NW_SERVER` 指向 `server/tools` 之上的 `server`、命令带 `-C tools`）。先在没改过的树上跑一遍同样的命令。

| # | 变异 | 抓它的 |
|---|---|---|
| 1 | 语料的生成器用时间作种子 | `TestPerfCorpusIsItsSeed` |
| 2 | `308eafc6` 的笔记本列多写一列 | 同上（逐列核对） |
| 3 | 一个非重类的词少植入一页 | `TestPerfCorpusPlantsTheQueries`（确切的数） |
| 4 | 词表不去掉含查询词的词 | 同上（核对两份词表） |
| 5 | 段落拼出了非重类的词也不重抽 | 同上（另以一张相接就拼出查询词的两个词的小词表生成 100 页，数仍等于目标） |
| 6 | `nodes.tsv` 的 `name` 与 `name_key` 两列对调 | `TestPerfCorpusLoadsIntoTheBenchmark`（名字带大写与 `ς`，两列处处不同） |
| 7 | `name_key` 用小写代替 `shared.TitleKey` | 同上（`ς` 折叠成 `σ`，小写不变） |
| 8 | 删一个文件夹时不删它下面的页 | 同上（删除的不变式的 SQL） |
| 9 | `guardScaleOf` 设了也返回默认 | `TestGuardScaleKeepsEachGuardsOwnUnlessGiven` |
| 10 | 设了不返回 `ByHand` | 同上 |
| 11 | 坏的 `NW_SCALE_PAGES` 当作默认 | 同上（返回错误） |
| 12 | 没设时用生产的期限（审查 A 的 I3） | 同上（四个函数的字面的数） |
| 13 | 没设时规模不是默认（批准的文件夹数照 `round(n/95)`、n 取错） | 同上 |
| 14 | `judge` 恒不报（判界恒关） | 同上（没设时过界的值要调 `errorf`） |
| 15 | 给了 `NW_SCALE_REQUEST` 仍截到 10 秒 | 同上 |
| 16 | 开关打开时客户端仍是 5 秒（审查 A 的 I2） | 同上（客户端是请求的两倍） |
| 17 | 写超时不在读 + 请求之上 | 同上 |
| 18 | lint 的 worker 仍是 3 秒 | 同上（表里 lint 一行），lint 守卫的"界低于期限"在 `costs-lint` 也抓 |
| 19 | `benchLevels` 设了也用默认 | `TestLevelsOfTheTrashBenchmarksAreTheirOwnUnlessGiven` |
| 20 | `changesetsOf` 不读 `NW_SCALE_CHANGESETS` | 它的测试 |
| 21 | gctrace 的峰值只取 GC 开始时的堆 | `gctrace` 的测试（结束时更大的那一行） |
| 22 | 基线不取 `-base` 那一行、取 `from` 之前最近的 | 同上 |
| 23 | 被插断的行跳过（审查 A 的 M4） | 同上（拼回之后读到它） |
| 24 | 缺号不报 | 同上 |
| 25 | 末尾的上界不看窗口之后第一行的 `D` | 同上 |
| 26 | `@Ts` 不加进程的开始时刻 | 同上 |
| 27 | 进程的开始时刻用 `btime` 的整秒 | `sample` 的测试（假的 `uptime`） |
| 28 | `VmHWM` 不乘 1024 | 同上 |
| 29 | 终值读 `memory.current` 当作峰值 | 同上 |
| 30 | cgroup 的目录不在时读成零 | 同上 |
| 31 | 最近秩差一位 | `stats` 的测试 |
| 32 | vault 不核对条目与节点的上限 | `vault` 的测试（超了要拒绝） |
| 33 | zip 过了 512 MiB 不删临时文件、不失败 | 同上（缩小的上限） |
| 34 | vault 用时间作种子 | 同上 |
| 35 | 链接写带路径的，或目标不在 vault 里 | 同上 |
| 36 | `bodies` 不看用时、凡 503 都算早答 | `drive` 的测试（2.1 秒答的 503 不数） |
| 37 | `tabs` 每个事件都读、不节流 | 同上（假的时钟） |
| 38 | `ops` 不等后台的恢复成功、不等队列清空就下一次删除 | 同上（假服务端先答运行中，假的 `psql` 先答队列不空） |
| 39 | `mcp` 的上限处的形状少一个值 | 同上（按 bodyshape 的数法数值） |
| 40 | `timedAsk` 用 `client()`（5 秒）而不是 `s.Client`（修订核对 A 的 M11） | 发起与批准的守卫（`costs-2`、`costs-1` 原来的行）：`timedAsk` 发之前断言客户端的 `Timeout` 等于 `s.Client`；批准的在播种的发起处就红 |
| 41 | lint 的夹具把 `d.LintTimeout` 设成生产的 `min(10 秒, 15 秒/2)` = 7.5 秒而不是 `s.Own`（"请求"取守卫自己的 12 秒时 `min(10 秒, 6 秒)` 仍是 6 秒，是等价的，修订核对 A 的 N9） | lint 的守卫（`costs-lint`）：读回的 `d.LintTimeout` 不等于 `s.Own`（短测试照绿，"界低于期限"也照过） |
| 42 | 发起的守卫以 15 秒的请求期限起应用（生产的） | 发起的守卫（`costs-2`）：`d.StartTimeout` 不等于 `s.Own` |

四个守卫里任何经 `timedAsk` 的请求改回经 `ask`，在默认的规模上都是等价的变异，抓不到：批准的守卫播种时的发起与提交在 5 秒之内答完；计时的那一个界先判，`timedAsk` 根本没被调，第 40 条也抓不到（修订核对 A 的 M12、N10）。审查者读代码核对四个守卫里剩下的 `ask` 只有工作量不随 n 变的那几处（3.3），6.1 第 9 步的日志里有播种的两个请求的用时。

浏览器一侧没有单元测试的跑法，量法的错由 4.3、4.4 的反向对照核对（7.3）。其中两条写明为什么不另设：

- "已画出"只等一帧而不是两帧：差一帧（约 17 ms），对 500 ms 以上的界线不改结论；审查者读代码核对。
- T1 的条件只看"有一行"：React 一次提交画出整个列表（没有虚拟列表，`page-tree-item.tsx:68-76`），与"视口铺满"在同一帧成立，是等价的变异（审查 C 的探针里条件成立时 10,000 行都在）；由 4.3 的条件之外的事后断言（终点时视口里的行数等于这种形状该有的）核对。

### 7.3 量法的反向对照

- S1：3.3 末的守卫的界的手工反向对照；6.1 第 9 步的 `308eafc6` 载入。
- S2：4.3 的六条、4.4 的两条、4.1 的 knip 三步，实施时各做一次，结果进记录；审查者在自己的副本里重跑其中的 1、3、5 与 4.4 的两条。
- S3：
  - B2 的阳性对照：一次单独的病态 5 MiB 阅读，堆的增量 ≥ 1.2 GiB（3.4.5）；
  - 采样的阳性对照：E 的第一轮开头，一次 5 MiB 普通正文的阅读，`VmHWM` 与 gctrace 的峰值都上升；
  - 3.5.4 的日志读取：随机挑一次运行，用作业的网页核对 CPU 与守卫的数。

### 7.4 门禁

6.4。另：`make gen-check` 照过（P4 不改契约与 SQL）；S1 的第 1、3、4 个提交核对组合根每个顶层的竞态测试恰在一份（`go test -race -list` 对八个模式，同 `Makefile` 第八份上面的注释的做法；`86565dd6` 上是 554 个）：语料的三个测试是 `!race`，不在列表里；新的 `TestGuardScale…`、`TestLevelsOf…` 落在第四份，名字不碰第一、二、三、五至八份的模式（修订核对 A 的 N7：初稿的 `TestBenchLevels…` 落在第一份）。第七、第八份的模式不改。

### 7.5 CI 的余量

- **S1**：竞态的八份只多编译两个不带标签的小文件与两个毫秒级的测试（第四份）；`test-go-costs-3` 多一行：编译不带竞态的组合根已在这一份里（`Makefile:465` 等），多的是一个新的 testcontainers 库、1,000 页的生成、载入与建索引，本机推断 5–10 秒，7763 上约 15 秒（推断，S1 量）；`server/tools/perf` 的单元测试几秒。`costs-1` 的 `lint-go` 多几个包，十几秒以内（推断）。改到的守卫在默认的规模上用时不变（只改期限与客户端）。S1 的记录写出分支 CI 与合并之后两次 `main` 的 `costs-3` 作业的用时与新那一行的秒数。
- **S2**：`web` 一项多 `tsc`、oxlint、oxfmt、knip 读的十来个文件，几秒；`e2e` 不变（`auth.ts` 只多一个导出）。
- **S3**：只改文档，不变。
- 每个 Step 合并之后两次 `main` 的组合根每份平均在 450 秒之下（00 号文档第 3 节第 11 条），否则照 06-environment 第 4 节挪份。

### 7.6 给审查者的抽查清单

实测的 Step 照常审查：核对方法、种子与脚本，抽一两项在自己的副本上重跑，不重跑全部（00 号文档第 7 节 P4）。

**S1（代码）**：
- 7.2 的变异，另加审查者自己的；
- 语料：自己生成 1,000 页与 3,000 页，核对 sha256 与查询的词的确切数；读生成器的频率表，对照 M8 的查询说明；`308eafc6` 的那一份载进那个提交一次；
- vault：生成缩小一百倍的导入上限形状，核对条目、文件夹、字节；导进一个 `stack.sh up` 的服务，导入成功、节点数对；照参数算一遍全尺寸的 zip 是否在 512 MiB 之下；
- 采样：对一个跑着的 `serve` 读 `/proc` 与 cgroup，与 `ps`、`docker stats` 对得上；gctrace 的解读对一份真的、有访问日志的日志手算一遍；
- 规模开关：以 `NW_SCALE_PAGES=20000` 跑一个升级的守卫，日志带 `scale=`、不判界、客户端与写超时照 3.3；不设时期限与客户端照 3.3 的表；手工的反向对照重做一次；
- 驱动：`bodies` 对真的服务答出 503 时的分类；`tabs` 的节流读代码与 `W/events/refresher.ts` 对照。

**S2（页面树先）**：
- 读 `probe.ts` 的条件与两帧，对照 4.3；
- 重跑：平的形状的 T1 一轮（5 次），深的形状的 T3 一轮；中位数与实施者的同一轮相差在 25% 之内、结论（过、超）相同；结论不同时这一格照 3.9 第 2 条复核；
- 4.3 的反向对照 1、3、5；
- 从 `results.jsonl` 重算报给主持者的表；
- 门禁与 knip 看得到 `e2e/perf/`（各自的反向对照）。

**S3（服务端）**：
- 每一行的记录有 3.5.5 要的各项；"判在"照 3.5.3；推断写了"推断"、写了用的比、× 1.25 的一行与 00 号文档的倍数各自的结论；
- 重跑：B2 的 k = 8、病态正文一轮；B5 的 `BenchmarkTrashRestore` 界线那一档一轮；与实施者的同一格相差在 25% 之内、结论相同；
- 从基准的日志重算 B1 的表（照原预算判的三项；尾巴与体积每格两轮较大的 p99、对照的界）；B1 有对照时核对对照的提交、语料的 sha256 与列的差别；
- 3.5.4：随机挑两次运行，用 `gh api` 重取作业的日志，核对 CPU 与守卫的数、本机在同一提交上的数、比的算法；
- B5 的硬期限：窗口里的运行、补进来的与其间的改动、两行推断的算式；
- 只量的各项：推算的写了依据，不量的写了理由（3.7）；
- 第 8 节要改的总体设计各句的锚点恰好出现一次（脚本断言）。

## 8. 完成标准

P4 完成，当且仅当：

1. S1、S2、S3 各自合并进 `main`，合并之前 6.4 的门禁全绿、审查与修复核对结束；每个 Step 合并之后两次 `main` 的组合根每份平均在 450 秒之下（7.5）。S4 起开了的条件 Step 同样合并，或经负责人改判为已知限制。
2. 7.1 的每个测试在，7.2 的每个变异被它旁边的测试抓到，7.3 的反向对照做过（记在第 10 节与各自的审查记录）。
3. **00 号文档第 3 节第 6 条有证据**：4.7.1 有预算的每一行在 10.3（B9 在 10.2）有 3.5.5 的各项与结论；B5 的规则一层没过、或硬期限一层的推断过了 15 秒时，记录写明列给主持者与负责人的两种做法与他们的决定；只量的各项有数或"推算"与依据；页面树的三条界线在本文 4.3、量之后逐条对照（10.2）；每一个超出已修（带缩小规模的守卫）或经负责人改判，其余在已知限制与 v0.2 的清单里。
4. **00 号文档**（`docs/v0.1/M12-release/00-M12-design.md`）：
   - **接受本文的文档提交**：2.1 的各条照下面的新文字改，由脚本 `apply_p4_design.py` 做（一处一个带标签的替换，旧文字各恰好出现一次，一处不匹配就什么也不写；02-workflow 第 2 节第 3 步）。下表由脚本的 `--list` 打出，与脚本是同一份：

   - **4.7.4 第四条"外部的驱动与采样"**（`474-editors`）：
     - 旧：N 个编辑者自动保存加树写（B4、B5 的满负载与部署文档的内存）
     - 新：N 个编辑者自动保存加树写（M4-performance 第 1 条与部署文档的内存；B5 的满负载是 M8 的口径，即数据的形状，用现成的基准量，P4 设计 2.1 第 1 条）
   - **4.7.1 B1 一行"超出的判据"**（`471-b1-criterion`）：
     - 旧：只有超出 M8 已记的最大值才算回归：无统计 719 ms，建索引与积压时约 1.2 秒，5,000 本 827 ms（I2-D3）；其余几项照原预算
     - 新：只有超出 M8 已记的最大值才算回归：200 本 719 ms（P5A 真语料"为任意的参数规划"那一阶段；无统计是 348 ms，R1 之后的合成语料上 474.6 ms），5,000 本 827.2 ms（R1 之后，无统计），体积 5.25 倍（R1 之后）；超了先在同机同语料上对照 `308eafc6` 两轮，也慢 20% 以上才算回归；建索引时与导入的清空期间沿用 M8 的数（908、1,149 ms）、不重量，积压只记（I2-D3；P4 设计 2.1 第 2 条、3.6 的 B1）；其余几项照原预算直接判（任务 ≥ 100 页/秒、30 万页从空建 ≤ 1 小时、保存 p99 多 ≤ 2 ms，保存可以推算），不经对照
   - **4.7.1 B1 一行"判在"**（`471-b1-where`）：
     - 旧：| 本机的数，对照 M8 记的数看回归 |
     - 新：| 本机的数；尾巴与体积对照 M8 记的数看回归，其余对原预算 |
   - **4.7.1 B1 一行"超出时"**（`471-b1-over`）：
     - 旧：| 回归要修；没回归，维持已知限制 |
     - 新：| 回归与超了原预算的要修；尾巴与体积没回归，维持已知限制 |
   - **4.7.1 B2 一行"超出的判据"**（`471-b2`）：
     - 旧：Go 的堆峰值超过 2.4 GB 加空闲时的基线；或没等满 `parse_max_wait` 就 503
     - 新：Go 的堆峰值超过 2.4 GB 加空闲时的基线；或没等满 `parse_max_wait` 就 503（堆峰值是 gctrace 每行 GC 开始与结束时的堆的最大，减每轮空闲时强制 GC 那一行的存活堆；超 2,400 MiB，即 8 MiB × 300，为超出，增量落在 2,289–2,400 MiB 之间时（README 的"约 2.4 GB"是同一个乘积的约数）两样都写、列给主持者；早答是客户端不到 2.0 秒的 503；P4 设计 2.1 第 9 条、3.6 的 B2）
   - **4.7.1 B3 一行"超出的判据"**（`471-b3`）：
     - 旧：| 内存超过约 1 GB 加基线 |
     - 新：| 内存超过约 1 GB 加基线（同 B2 的读法，两轮六次里的最大超 1,024 MiB；P4 设计 3.6 的 B3） |
   - **4.7.1 B4 一行"判在"**（`471-b4`）：
     - 旧：| 有 CI 上的实数时由它定；没有时本机 × 4.6（推断） |
     - 新：| 有 CI 上的实数时由它定；没有时本机 × 回收站与撤销一族的实测之比（推断；另写 × 4.6 的结论，P4 设计 3.5.4） |
   - **4.7.1 B6 一行"判在"**（`471-b6`）：
     - 旧：| 10 万页上每条规则与摘要过 7.5 秒 | 有 CI 上的实数时由它定；没有时本机 × 2.33（推断） |
     - 新：| 10 万页上每条规则与摘要过 7.5 秒 | 有 CI 上的实数时由它定；没有时本机 × 升级一族的实测之比（推断；另写 × 2.33 的结论，P4 设计 3.5.4） |
   - **4.7.1 B7 一行"判在"**（`471-b7`）：
     - 旧：有 CI 上的实数时由它定（守卫在 7763 上记下的）；没有时本机 × 2.33（推断）
     - 新：有 CI 上的实数时由它定（守卫在 7763 上记下的）；没有时本机 × 升级一族的实测之比（推断；另写 × 2.33 的结论，P4 设计 3.5.4）
   - **4.7.1 B8 一行"判在"**（`471-b8`）：
     - 旧：本机 × 4.6（推断：移交与 I2 里没有导入导出的比；6 小时之下乘多少都不改结论）
     - 新：本机 × 两族的实测之比的较大（推断；另写 × 4.6 的结论；6 小时之下乘多少都不改结论，P4 设计 3.5.4）
   - **4.7.1 B10 一行"判在"**（`471-b10`）：
     - 旧：`index`：有 CI 上的实数时由它定，没有时本机 × 2.33（推断），对 15 秒
     - 新：`index`：有 CI 上的实数时由它定，没有时本机 × 升级一族的实测之比（推断；另写 × 2.33 的结论，P4 设计 3.5.4），对 15 秒
   - **4.7.1"判在哪台机器上"第一条**（`471-where-1`）：
     - 旧：只看回归的 B1，与 B5 的规则
     - 新：B1（尾巴与体积只看回归，其余照原预算），与 B5 的规则
   - **4.7.1"判在哪台机器上"第二条**（`471-where-2a`）：
     - 旧：没有时才乘倍数**，写明"推断"
     - 新：没有时才乘本机对 7763 的实测之比**，写明"推断"
   - **4.7.1"判在哪台机器上"第二条**（`471-where-2b`）：
     - 旧：所以 B7 在 10 万页上只能乘 2.33、写"推断"
     - 新：所以 B7 在 10 万页上只能推断（本机 × 升级一族的实测之比）
   - **4.7.1"判在哪台机器上"第三条（整条）**（`471-where-3`）：
     - 旧：  - 倍数是移交里记下的 CI 对当时开发机之比的最大值：lint、`index` 与升级记的是 1.16–2.33 倍，取 2.33；回收站、撤销与后台记的是 2.1–4.6 倍，取 4.6（I2 第 1.1 节）。它的分母是当时更快的开发机（M5 Max），其中还有 CI 的摆动；拿它乘现在更慢的本机，会多算一层，所以只在没有实数时用。
     - 新：  - 推断用本机对 7763 的实测之比（P4 的设计审查之后的决定 454 改了这一条与决定 318 的倍数一半；P4 设计 2.1 第 3 条、3.5.4）：同一组守卫在 7763 上各次的中位（每个作业取最近的至多三次），除以本机在同一批提交上的中位，按族取最大；回收站与撤销一族给 B4，升级一族给 B7（B6、B10 的 `index` 没有实数时也用它），B8 取两族的较大；对硬期限的推断另并列 × 1.25 的一行（决定 359 记的 7763 的尾巴）。移交里记下的倍数（lint、`index` 与升级 1.16–2.33 倍，取 2.33；回收站、撤销与后台 2.1–4.6 倍，取 4.6，I2 第 1.1 节）的分母是当时更快的开发机（M5 Max），其中还有 CI 的摆动，拿它乘现在更慢的本机会多算一层；照它的结论并列写出，与实测之比的结论不同时列给主持者。
   - **4.7.3"机器"一条**（`473-machine`）：
     - 旧：没有时按移交里记的两者之比推算（4.7.1 的"判在"一列）
     - 新：没有时按本机对 7763 的实测之比推算（4.7.1 的"判在"一列；P4 设计 3.5.4）
   - **4.7.1 B5 一行"超出的判据"（规则）**（`471-b5-rule`）：
     - 旧：参照机（本机）上、满负载、两轮的较大中位数超过 5 秒
     - 新：参照机（本机）上、满负载、两轮的较大值超过 5 秒（每轮的值是 M8 的量法：回收站与撤销的基准三次的均值，后台的一个样本；P4 设计 2.1 第 13 条）
   - **4.7.1 B5 一行"超出的判据"（硬期限）**（`471-b5-hard`）：
     - 旧：`MaxJobItems` 是后台单元、没有请求期限，只用规则一层（修订核对第 2 轮 A 的 N15） |
     - 新：`MaxJobItems` 是后台单元、没有请求期限，只用规则一层（修订核对第 2 轮 A 的 N15）。实数只取这几条守卫与它们量的文件没再改过的那一段里的 7763 各次：不到三次就用已有的、写明次数；一次也没有时，取改动之前最近的几次，乘本机在两个提交上之比，写"推断"。实数之外并列两行推断：7763 各次的中位 × 1.25 加 2 秒；本机基准里最慢的一阶段（多半是 ANALYZE）对本机守卫之比 × 7763 的守卫加 2 秒；任一行过 15 秒，就同规则一层列给主持者与负责人，部署文档的硬件下限照他们定的写（P4 设计 3.6 的 B5） |
   - **4.7.2 T2**（`472-t2`）：
     - 旧：名字按次序排在最前
     - 新：以 `after_id: null` 新建、排在同级的最前（树按 `sort_order` 排，不按名字）
   - **4.7.2"规模与形状"的"深"**（`472-deep`）：
     - 旧：  - 深：10 个根、各 10 个文件夹、各 100 页，另有一个 1,000 个孩子的文件夹；
     - 新：  - 深：10 个根、各 10 个文件夹、各 100 页，另有一个 1,000 个孩子的文件夹（共 11,111 页；那个文件夹在根下排在最前；T3 只在深的两种形状上判，P4 设计 4.3）；
   - **4.7.2"只记的"**（`472-bytes`）：
     - 旧：线上的字节（代理开压缩与不开各一次）
     - 新：树的答复的 JSON 字节与它在 gzip 1 级、6 级与 zstd 3 级下的字节（离线算，P4 设计 2.1 第 6 条）
   - **4.7.3"被量的"**（`473-db`）：
     - 旧：建库参数同测试（builtin `C.UTF-8`、LZ4）
     - 新：建库参数同测试（`--locale-provider=builtin --locale=C.UTF-8`）；进程内的基准经 `pgtest`（testcontainers 加了 `fsync=off`，同 M8），对外的容器不加（P4 设计 2.1 第 10 条）
   - **4.7.3"批次"**（`473-batches`）：
     - 旧：缩小之后的机器时间少于初稿估的 10–16 小时，P4/S3 的设计重估。
     - 新：P4 的设计重估：S3 约 16–20 小时、有条件的另约 4.5–6 小时，S2 约 3–3.5 小时，比初稿估的 10–16 小时多（搜索两种分布各两轮、对照照整轮算，导入之后每次删除都等搜索的队列，本机对 7763 之比要在每个作业最近的至多三次 7763 的提交上量；P4 设计 3.8）。
   - **4.7.4"Go 的生成器与基准"第一条**（`474-notebooks`）：
     - 旧：合成的大笔记本：1 万、5 万、10 万、30 万个节点；链接、附件、历史可调
     - 新：对外量的大笔记本经导入生成的 vault 建起（`server/tools/perf/vault`，10 万页导两次）；30 万页只经现有守卫的规模开关在进程内量（P4 设计 2.1 第 5 条）
   - **4.7.4"Go 的生成器与基准"第三条**（`474-vault`）：
     - 旧：GB 级的 vault（批次 F 要）
     - 新：GB 级的 vault 也是这一个生成器（批次 F）
   - **4.7.4"浏览器的计时脚本"**（`474-knip`）：
     - 旧：`knip.jsonc` 今天没有 e2e 的入口配置，P4/S2 给 `e2e/perf/` 加入口
     - 新：knip 要读到 `e2e/perf/` 的配置：P4/S2 经 `e2e/package.json` 的 `perf` 脚本（knip 从脚本解析 `-c`；配置写字符串的 `testMatch` 与绝对路径的 `globalSetup`）；`knip.jsonc` 只给 e2e 加一节，忽略 B9 的解析页按路径载入的 web 的 `vite`（P4 设计 2.1 第 7 条、4.4）
   - **第 7 节 P4/S2 一行"内容"**（`7-p4s2-content`）：
     - 旧：`e2e/perf/` 的脚本与配置，`knip.jsonc` 给它加入口；
     - 新：`e2e/perf/` 的脚本与配置，knip 读到它的配置；
   - **第 7 节 P4/S2 一行"改的部件"**（`7-p4s2-parts`）：
     - 旧：| e2e（perf）、knip、docs |
     - 新：| e2e（perf；`package.json` 的脚本与两个 devDependencies；`fixtures/auth.ts` 导出引导的步骤表）、knip（`e2e` 一节忽略 `vite`）、docs |
   - **第 7 节 P4/S1 一行"内容"**（`7-p4s1-content`）：
     - 旧：| S1 设施（Go） | 4.7.4：合成的大笔记本、搜索语料（三个 TSV）、GB 级的 vault；现有守卫的规模开关；`server/tools/perf/` 的驱动（有预算的几项要的）与采样 |
     - 新：| S1 设施（Go） | 4.7.4：搜索语料（三个 TSV）、导入用的 vault 的生成器（含 GB 级）；现有守卫的规模开关；lint 守卫的 worker 期限提到界之上（决定 359 的做法，M10 决定 160 留下的）；回收站两个基准的档位开关；`server/tools/perf/` 的驱动（有预算的几项要的）与采样 |
   - **第 7 节 P4/S1 一行"单独合并为绿"与"组合根与 CI 的用时"**（`7-p4s1-green-time`）：
     - 旧：| 能：没有环境变量就跳过，CI 只多编译；lint 要过 | 组合根多编译，不多跑；推断各份不变 |
     - 新：| 能：没有环境变量就跳过；语料的三个测试在 `!race` 的文件里，`test-go-costs-3` 单独一行；lint 要过 | 竞态的八份不变（只多两个毫秒级的短测试，在第四份）；`test-go-costs-3` 多一行，7763 上推断约 15 秒（P4 设计 2.1 第 8 条、7.5） |
   - **第 7 节 P4/S3 一行"组合根与 CI 的用时"**（`7-p4s3-time`）：
     - 旧：| 不变；本机的机器时间少于初稿估的 10–16 小时（推断，P4/S3 的设计重估），独占 heavy 的锁 |
     - 新：| 不变；本机的机器时间约 16–20 小时、有条件的另约 4.5–6 小时（推断，P4 设计 3.8；比初稿估的 10–16 小时多），分几夜独占 heavy 的锁 |
   - **第 7 节"次序与并行"第二条**（`7-order-open`）：
     - 旧：- P1/S2 合并之后，可以开 P2/S1–S5、P3/S1、P4/S1、P5/S1。
     - 新：- P1/S2 合并之后，可以开 P2/S1–S5、P3/S1、P4/S1、P4/S2、P5/S1。
   - **第 7 节"次序与并行"P4 一条**（`7-order-p4`）：
     - 旧：- P4/S2 在 P4/S1 之后；P4/S3 在 P3 全部合并、P4/S2 之后；
     - 新：- P4/S2 不等 P4/S1（只用 TypeScript 与 e2e 的帮手，P4 设计第 0 节、2.1 第 12 条）；P4/S3 在 P3 全部合并、P4/S1 与 P4/S2 之后；
   - **4.1 的"可以并行"**（`41-parallel`）：
     - 旧：**P2、P3、P4/S1、P5/S1 可以并行**
     - 新：**P2、P3、P4/S1、P4/S2、P5/S1 可以并行**
   - **第 3 节第 6 条**（`3-6-ratio`）：
     - 旧：在哪台机器上判（本机、CI 的实数，或乘了哪个倍数）
     - 新：在哪台机器上判（本机、CI 的实数，或乘了哪个比）
   - **第 3 节第 6 条（B5 的两层，修订核对 B 的 N17）**（`3-6-b5`）：
     - 旧：B5 的规则一层没过的，记录写明列给主持者与负责人定的两种做法与他们的决定。
     - 新：B5 的规则一层没过、或硬期限一层的推断过了 15 秒的，记录写明列给主持者与负责人定的两种做法与他们的决定。
   - **9.4**（`94-ratio`）：
     - 旧：- 判在哪台机器上（本机、CI 的实数，或乘了哪个倍数并写"推断"）
     - 新：- 判在哪台机器上（本机、CI 的实数，或乘了哪个比并写"推断"）
   - **第 7 节"总体设计的修订"，第 24 条之后**（`7-revision-25`）：
     - 旧：**文档与代码不符之处**（M12 里改文档
     - 新：25. **13.4 第 8 条**，随 P4/S1 改（P4 设计 2.1 第 15 条）：`例外是十万页摘要的成本用例，请求 6 s、worker 3 s（决定 160）` → `例外是十万页摘要的成本用例，请求 12 s、worker 6 s（决定 160；M12/P4/S1 起，期限由界推出、断言界低于期限，M12 决定 475）`；第 15 节随之加一行，同决定 359 的先例。

   **文档与代码不符之处**（M12 里改文档
   - **第 11 节决定 318**（`11-318`）：
     - 旧：没有时才乘记下的最大倍数（2.33 或 4.6）、写"推断"
     - 新：没有时才乘本机对 7763 的实测之比、写"推断"（这一半由 P4 的设计审查之后的决定 454 改，原来是乘记下的最大倍数 2.33 或 4.6）

   - 2.3 的取舍写进第 11 节，在"对主持者说明的核实"之前另起一节"P4 的设计审查之后的取舍（主持者的决定，负责人可以改判）"，每条一句、带它在本文与审查记录里的位置，正式编号 453–478（决定 318、"判在哪台机器上"第三条里指向 P4-2 的那一处是 454，"总体设计的修订"第 25 条的是 475）；P4-26 已是决定 452，不写进去；
   - 第 12 节的 P4 一行 `| P4 性能与内存实测（S1–S3，条件的 S4 起） | — | 未开始 | — | — |` → `| P4 性能与内存实测（S1–S3，条件的 S4 起） | [04-P4](04-P4-performance.md) | 设计已审查（2026-11-16） | [设计](reviews/P4-design-review.md) | — |`；第 13 节在表的最后一行之后加一行；头的"状态"与 P2、P3、P5 共用一行，由主持者在提交时改；
   - 第 11 节、第 13 节的表尾与第 12 节，P2、P3、P5 的设计提交也改：脚本以"对主持者说明的核实"与表尾之后的"附录 A"为锚点，在当时的位置插入，先后不论（在 `86565dd6`、P3 设计提交之后的 `48bbfc99` 与 P2、P3、P5 设计都已提交的 `1ba6c7f6` 上都试过：第三次修订之后 `1ba6c7f6` 上 42 处都匹配，写入之后 1,617 行变成 1,650 行，再跑 38 处报出现 0 次、什么也不写）。
   - **每个 Step 的文档提交**：第 12 节的 P4 一行写到这个 Step，"审查"一格加它的记录 `reviews/P4-S<m>-<slug>-review.md`，"合并"写占位、合并之后填；第 13 节加一行（S2 那一行写页面树的结论，S3 那一行写 B1–B10 的结论一句）；"需要负责人确认"第 7 项写上负责人的回答与日期（S3）。
   - **`docs: M12/P4 done (M12/P4)`**：第 12 节"完成"、头的状态、第 13 节一行。
   - 00 号文档第 7 节"总体设计的修订"第 9 条不改：它写"随 P4/S3、P5/S3 改"，P5 的设计提交已在它末尾写明 `NOTIFY` 与 SSE 两行归 P5/S3（P5 设计 2.1 第 20 条，决定 452）；P4 的脚本不碰这一条。第 25 条（13.4 第 8 条）由 P4 的脚本加。
5. **v0.1 总体设计**（`docs/v0.1/v0.1-design.md`；00 号文档"总体设计的修订"第 9 条，脚本断言锚点恰好出现一次）：
   - **第 14 节**写着 M12 的十二行里，P4 改十行：搜索、渲染的内存、`FOR SHARE`、解析预算先到先得、lint 与 `index.md`、保存串行与改名、附件与导入导出、页面树、版本的增长、升级的冻结时长。把"M12 实测""M12 压测""留给 M12"改为量得的数、结论与已知限制，每行末尾写 `（M12/P4 实测，[P4 设计](M12-release/04-P4-performance.md) 10.3）`，页面树一行写 `10.2`（审查 C 的 N2）。
     - **`NOTIFY` 一行由 P5/S3 改**（决定 452，即取舍 P4-26）：它不是实测的事（升级 PostgreSQL 大版本时复核），P5/S3 把"M12"的指向改为部署文档；SSE 一行（`v0.1-design.md:1311`，"M12 的部署文档沿用"）同样归 P5/S3（审查 B 的 N3）。
     - **页面树一行在 S2 的文档提交里改**（它的结论在 S2；S2 的文档提交落了），锚点 `M12 实测；不达标时改为分层加载` 在 S2 的提交里断言；S3 的提交断言 00 号文档点名的其余四个锚点（`M12 压测`、`v0.1 全部保留；M12 实测后在 v0.2 设计保留策略`、`配额与负载留给 M12（M7 写的移交）`、`10 万页与 30 万页由 M12 实测`）各恰好出现一次，另五行在 S3 的提交里先找出锚点、同样断言（审查 C 的 N2）。
   - **13.4 第 8 条**的 `进 M12 的性能移交（00-M11 第 232 条）`、`检查的十万页在 M12`，**第 9 条**的 `旧generic返回2000却扫52000的失败和未成功实验仍移交M12`：改成量得的数或已知限制（3.6 的 B7、3.7 第一行），在 S3 的提交里。
   - **13.4 第 8 条的 lint 那一句在 S1 的文档提交里改**（00 号文档"总体设计的修订"第 25 条；取舍 P4-23；修订核对 B 的 M11）：
     - 旧：`例外是十万页摘要的成本用例，请求 6 s、worker 3 s（决定 160）`（`v0.1-design.md:1276`，在 `1ba6c7f6` 上恰好出现一次，S1 的提交断言它）
     - 新：`例外是十万页摘要的成本用例，请求 12 s、worker 6 s（决定 160；M12/P4/S1 起，期限由界推出、断言界低于期限，M12 决定 475）`
     - 第 15 节加一行，同决定 359 那一行的写法：`- <日期>：M12/P4/S1：13.4 第 8 条十万页摘要的成本用例的期限改为请求 12 s、worker 6 s（摘要的墙钟界 5.2 秒原在 3 秒的 worker 之上，轮不到它判；期限由界推出、断言界低于期限）。依据 [00-M12](M12-release/00-M12-design.md) 决定 475 与 [P4/S1 的审查记录](M12-release/reviews/P4-S1-<slug>-review.md)。`
   - **第 15 节**：S1、S2、S3 的文档提交各加一行（S1 的是上一条）。
6. **移交的"处理进展"**各加一行（状态仍是 `open`，收尾时改 `done`）：
   - S1：`M4-performance.md`："M12/P4/S1（日期，合并见 00-M12 第 12 节）：量的设施进仓库：搜索语料的生成器、四个守卫与按页的日志的规模开关、lint 守卫的 worker 提到界之上、回收站基准的档位开关、`server/tools/perf/` 的驱动、采样与 vault（P4 设计 3.2–3.4）。"
   - S2：`M4-performance.md` 第 7、9、11–13 条，`M7-transfer.md` 第 4、6 条：各自的数与结论（10.2）。第 11、13 条随 4.6 挪到 S3 时，写在 S3 的那一行（10.3；修订核对 C 的 N12）。S2 的文档提交落了（第 11、13 条挪到了 S3）。
   - S3：`M4-performance.md` 其余各条与补记、`M5-performance.md` 第 3 条、`M7-transfer.md` 第 2、5 条、`M8-history.md` 第 2 条：各自的数、结论、已知限制或 v0.2 的去处（10.3）。
7. **其余的文档**：S3 照实测的经验在 `05-lessons.md` 加条目（例如本机对 7763 之比的读法、gctrace 被访问日志插断）；`06-environment.md` 第 3 节加一句"P4 的实测怎样跑：`server/tools/perf/stack.sh` 与 `cd e2e && pnpm perf`，经 `heavy.sh`，独占机器"。

## 9. 交给后面的 Phase

- **P2/S6**：页面树的结论（S2 先报主持者，10.2；决定 378，P2 设计第 9 节）。三种形状都过：S6 照它的设计做 12.12、27.2、22.14、23.10；有一条没过：S6 排在改法的 Step 之后（00 号文档第 7 节"次序与并行"）。S2 的结论：平的形状三格超，S6 排在 P4/S4 之后（决定 510，10.2）。
  - **预告，不是正式的量**（审查 C 的 Q1 探针：test 配置、一次、只量了平的形状，`m12/p4-review-c/t1/`）：1 万页平的形状清了缓存的 T1 2,412–2,585 ms，界线 1 秒；其间最长的一个长任务 2,144–2,296 ms；首页的 `SubpageList` 又列一遍根下的页，DOM 里 20,001 个 `li`（左栏 10,000、正文 10,000，`notebook-home.tsx:66`）。平的形状的 T1 多半过不了，P2/S6 多半要等一个改法的 Step；S2 先量页面树、可与 S1 并行（取舍 P4-24），就是为了早一点给出正式的结论。
  - **两种候选改法的范围**（正式的量没过时，改法的设计照 00 号文档 4.7.5 另起，不在本文）：
    1. **左栏只渲染看得见的行**（虚拟列表或分批）：改 `W/pages/notebook/page-tree.tsx` 与 `page-tree-item.tsx` 的列表，拖放、键盘、展开与滚到当前页随之改；4.7.2 的终点都在视口里，改前改后量得了；它与 P2/S6 的 12.12、27.2 在同一块（树的 store 与组件：12.12、27.2 改的是 `W/stores/page-tree.ts:116`、`:124` 的纯函数，候选 1 改的是 `page-tree.tsx` 与 `page-tree-item.tsx`，修订核对 C 的 N8），改法的设计照 [P2 设计](02-P2-web-polish.md) 4.6.6 写明会碰到的 S6 的项。
    2. **笔记本首页不再列全部根下的页**：`notebook-home.tsx` 的 `SubpageList` 只列前若干个加"更多"，或不列；平的形状的 DOM 约减半，但左栏仍有一万行，单靠它多半不够；首页列什么是产品的事，可能要负责人看。
- **M12 的汇报与负责人**（取舍 P4-2，主持者的决定）：**P4-2 多半决定 B4 删除的结论**：乘本机对 7763 的实测之比约 12–15 秒，乘 00 号文档原来的 4.6 约 28–35 秒（审查 B 的 Q1，推断）。S3 写出两种结论，汇报列出，负责人可以改判。同列：B5 的两层（2.2，修订核对 B 的 M15）：规则一层在本机多半过不了，硬期限一层的第一行推断照今天的数已是 15.75–16.4 秒；S3 量定之后两层一起照 00 号文档 4.7.5 列给主持者与负责人（降常量，或写进部署要求），部署文档的硬件下限照他们定的写，主持者可以在 S3 之前先告诉负责人；B9 的预告（下一条）；P4-9 的病态形状、P4-17 的曲线只在没过时跑、P4-23 的 lint 守卫的期限。
- **B9 的预告，不是正式的量**（修订核对 C 的 Q2 探针：没持 heavy 的锁、机器上有别的 Step、每种热身两次加五次、两次运行，`m12/p4-check2-c/b9/out-shapes*/`）：一串空格的一段正好在上限处，整篇解析 170–440 ms；上限减一再插一个字 44–110 ms，随负载与形状在 80 ms 上下（照 4.4 加上头之后中位约 48 ms，修订核对 C 的 N13）；`*a** ` 与 `[a](b (x ` 也各有一次 84–93 ms 的整篇；界线 80 ms。B9 多半"超"，靠的是整篇那一格。主持者定，同页面树：这里只记作预告；正式的量在 S2（4.4），没过时照 00 号文档 4.7.5 另起改法的设计，不在本文预先设计（C 提过的降 `inlineLimit`、另拦一串空格只记在这里，不是设计）。
- **P5/S3**（部署文档与发布说明）要的数，S3 在 10.3 列成一节"给部署文档"。照 [P5 设计](05-P5-deploy-release.md) 3.18"数从哪里来"一表逐项对照（已提交的表，十三行，它的头一句说与本节对过；表里没有的，P4 写"推算"与依据，或在 S3 开始之前与主持者对齐）：

  | P5 设计 3.18 的一行 | P4 的出处 | 量还是推算 |
  |---|---|---|
  | 默认配置的内存：服务进程空闲与 N = 20 个编辑者稳态的 RSS；B2、B3 的峰值；PostgreSQL 容器的 `memory.peak` | 批次 E：空闲 130 秒之后与 `editors` N = 20 稳态的 `VmRSS`、`VmHWM`；B2、B3 的堆峰值与上界；每轮容器的 `memory.peak` | 量 |
  | `GOMEMLIMIT` 的建议值 | 由上一行的数给：B2 解析预算满时的峰值与 `editors` 稳态之间，S3 写依据 | 推算（依据是量的数） |
  | 小机器档的内存 | 批次 E 的小机器档（取舍 P4-27：`page.parse_budget_bytes` 5 MiB 加 `GOMEMLIMIT=1536MiB`，B2 的 k = 8、两种正文各一次） | 量（只记） |
  | 最低 CPU（EPYC 7763 4 核）的期限余量 | B5 硬期限一层的实数（窗口里的次数写明）与两行推断；B4、B7 的推断（含 × 1.25 的一行）；3.5.4 的比 | 实数与推断 |
  | 磁盘：每 1,000 页（2 KiB 正文、30 条链接）的库的大小；`page_search` 对正文的倍数 | 批次 F 导入、队列清空之后的 `pg_database_size` ÷ 45（导入上限的 vault 正是 2 KiB 正文、30 条链接，另有一成附件，附件在存储目录、不在库里）；B1 的体积对正文的倍数（两种分布） | 量 |
  | 磁盘：附件与导出的 zip | B8：存储目录与它的 `imports/`、`exports/`，导出的 zip 的大小 | 量 |
  | WAL：导入上限的一次导入、30 万页从空建搜索 | B8 前后的 `pg_current_wal_lsn()`；批次 B 建索引前后的同一个读数（3.7 第 17 条；读不到时用 F 里导入之后搜索的队列清空的 WAL） | 量 |
  | WAL 与时长：`nervewiki reindex` | 不量，沿用 M8 的数（P5 设计 2.1 第 21 条） | — |
  | 版本的增长 | 批次 G：每个版本的字节（3.7，`VACUUM FULL` 两头量）× 每人每天的编辑会话 | 推算 |
  | 时长：导入、导出、搜索的队列清空 | B8：任务从 202 到成功、上传与下载；F 里队列清空的用时 | 量 |
  | 10 万页的树被 k 个标签页重读时的 CPU 与出口字节；树的 JSON 的压缩比 | 批次 E 的 `tabs`（k = 1、4、8）；压缩比在 4.3（1 万页）与 4.9（5 万、10 万页） | 量 |
  | 迁移的停机 | 批次 H，只在负责人说有 v0.1.0 之前的实例时 | 看回答 |
  | 已知限制的清单 | 10.3 的清单（每项写出处、数与 v0.2 的去处） | — |

  `README.md:500`、`:533`、`:540`（写的是"M12真实环境复核""交M12复核"这样的话；00 号文档转述为"M12 实机复核"，原文里没有这几个字）由 P5/S3 照这一节改（00 号文档"文档与代码不符之处"末条；审查 C 的 N3）。总体设计第 14 节的 `NOTIFY` 与 SSE 两行也归 P5/S3（决定 452；第 8 节第 5 条）。
- **收尾**：v0.2 的移交写 P4 的已知限制（5 万、10 万页的树与分层加载、按页的日志的迁移、搜索的尾巴、病态的删除、改名与 `move` 换算到 7763 的规模、B5 若写进部署要求的那一条等）；M12 的汇报列出上面那几条负责人可能想改判的。
- **`server/tools/perf/` 与 `e2e/perf/` 留在仓库里**：v0.2 的实测照用；它们不进 CI 的跑，只进编译与 lint（语料的三个测试在 `test-go-costs-3` 里跑）。

## 10. 结果

（实施后补。）

### 10.1 S1：设施

### 10.2 S2：浏览器一侧

被量的产品是 `main` 的 `99def961`（只改 `e2e/`；修复三轮的各轮用分支上门禁构建的 `bin/nervewiki`，web 与 server 与 `99def961` 相同）。本机 aarch64、8 vCPU、30 GiB；Playwright 1.63 的 Chromium 153.0.8010.12（headless shell），1280 × 720；PostgreSQL 18.7；生产配置的 `serve`（4.2）。每轮一次 `cd e2e && pnpm perf`（新的服务、库与浏览器），经 `heavy.sh`；形状的种子 20261116。各轮的提交、持锁与负载在 [S2 的审查记录](reviews/P4-S2-browser-measures-review.md)，结果文件（每轮的 `results.jsonl` 与 `run.log`）在设计工作目录的 `m12/p4s2/results/`。与本文写法不同之处见 4.11。

**页面树**（4.3"报主持者的表"；决定 510）。每格：两轮各五次的中位数，两轮的较大加粗、对照界线；括号里是五次。单位 ms。

| 形状 | T1 冷打开 ≤ 1,000 | T2 别人改树 ≤ 1,500 | T2 的最长长任务 ≤ 200 | T3 展开 ≤ 500 |
|---|---|---|---|---|
| 平（10,000 页） | 3,002（3,014、2,999、2,964、3,002、3,011）；**3,015**（3,015、2,999、2,998、3,030、3,033）；**超**。复核 3,062（3,036、3,083、3,096、3,062、3,060）、3,097（3,097、3,106、3,088、3,193、3,091），超 | 1,532（1,824、1,510、1,532、1,473、1,540）；**1,609**（1,792、1,549、1,618、1,533、1,609）；**超**。复核 1,637（1,798、1,554、1,652、1,571、1,637）、1,546（1,879、1,532、1,588、1,481、1,546），超 | 1,361（1,647、1,338、1,368、1,296、1,361）；**1,429**（1,621、1,376、1,439、1,351、1,429）；**超**。复核 1,462、1,365，超 | —（没有文件夹） |
| 深（11,111 页） | 196（195、194、196、208、206）；**198**（193、198、205、193、198）；过 | **61**（61、61、67、56、67）；58（58、62、58、68、58）；过 | 两轮都是 0（没有长任务）；过 | **212.4**（253、214、212、196、203）；206.4（270、220、202、204、206）；过 |
| 带附件（11,111 页、1,000 个附件） | 204（198、208、204、204、198）；**210**（210、202、204、211、213）；过 | 65（69、65、62、67、60）；**66**（62、70、66、65、67）；过 | 两轮都是 0；过 | **211.3**（284、214、211、202、203）；209.6（266、219、210、200、204）；过 |

- **结论**：平的形状三格超，3.9 第 2 条的复核两轮照样超；照 00 号文档 4.7.2，改法在条件的 Step P4/S4（[S4 设计](04-P4-S4-page-tree.md)，决定 510），P2/S6 排在它之后。深、带附件两种形状全过。S4 改完用 `tree.perf.ts`（`5a76031a` 起未改）照同样的终点、方法与两轮再量。
- **慢在渲染**（诊断，不是正式的量）：树的答复约 150 ms 读完，之后一个约 2.25 秒的长任务，画出在 3.0 秒；一次 T1 的 CPU 剖面里 react-dom 约 830 ms、每行的 `NavLink` 约 220 ms、每行的 `DropdownMenu` 约 360 ms、GC 约 320 ms、DOM 的原生调用约 230 ms、排版约 390 ms，store 与树的索引不在前列；首页正文的 `SubpageList` 又列一遍一万页（文档里 20,003 个 `li`）；T2 里树重读之后整列重渲染，一个 1.3–1.5 秒的长任务。
- **T2**：正式 30 次、复核 10 次，没有一次作废（新行都在视口里，树的第一行 top 524 px，不必滚动）。复核两轮只导入平的形状（`NW_PERF_SHAPES=flat`），左栏上方少两行（第一行 top 444 px，T1 终点时视口里 9 行对 6 行），三格照样超。
- **T3** 是挪到 T2 之前重量的两轮（决定 515，`5a76031a`，`NW_PERF_SHAPES=deep,attachments`；第一行 top 484 px，S4 再量照同一个设定），每次视口里 6 个孩子。原来在五次 T2 之后量（深 227 / 231、带附件 236 / 234），那时 `000 big` 已在根下第 6 行、视口里没有孩子，不用。深 212、带附件 211 是 P4/S4 的"改前"。
- **量法的反向对照**（4.3 的六条，深的形状）：主 JS 压后 1 秒，T1 多 1,007；`/nodes` 压后 1 秒，多 1,007；答复之后 300 ms 的任务，最长长任务 300；五次 T2 极差 8.7、容差取 100，`/nodes` 压后 0.5 秒多 498；不清缓存时主 JS 的 `transferSize` 0；收起的 `r01` 5 秒没成立。都照期望。第一遍的探针在条件成立的微任务里取位置、强迫排版，作废重量（4.11 第 3 条）：平的 T1 没变。
- **只记的**（每种形状每轮一次；两轮之差在千分之一以内，列轮 2）：

  | 形状 | 树的 JSON 字节 | gzip 1 / 6 | zstd 3 | 第五次 T1 之后的 JS 堆（GC 之后） | T1 之内最长的长任务 | 首页正文的 `li` / 文档的 `li` |
  |---|---|---|---|---|---|---|
  | 平 | 2,449,229 | 251,677 / 235,893 | 252,362 | 604 MB | 2,254 ms | 10,000 / 20,003 |
  | 深 | 3,156,470 | 291,975 / 273,339 | 290,085 | 17.4 MB | 0 | 11 / 25 |
  | 带附件 | 3,423,296 | 319,140 / 296,042 | 315,090 | 18.3 MB | 0 | 11 / 25 |

  深的形状里"移到……"对话框（`000 big`，1,000 个孩子）从点菜单项到选择框画出：两轮的较大中位 1,258 ms（1,283、1,288、1,258、1,231、1,230），没有界线；一次 T1 的剖面里 `subtreeOf` 的自身时间 0.52 / 0.21 ms。

**B9：编辑器的解析**（4.4；决定 511、512）。修复第 3 轮的写法（`869b9f3c`）：每种形状的每一格在一个新的页里、先收一次垃圾，热身 2 次、量 20 次、取中位；四轮，1、2 正式，3、4 是超的格的复核。每次量完都断言"上限之内"，四轮都过。单位 ms，界 80（≤ 80 过）。每格是轮 1 / 轮 2 的中位，粗体是两轮的较大；超的格后面是复核两轮。判的是前三列，第四列只记。

| 形状 | 整篇一次解析（段长 10,000） | 段中插一个字（9,999 → 10,000） | 段尾插一个字 | 只记：编辑器的解析器一次解析 |
|---|---|---|---|---|
| links | **3.2** / 3.0 | 3.0 / **3.1** | 3.1 / **3.2** | 3.2 / 3.0 |
| links and brackets that close none | **3.5** / **3.5** | **3.5** / 3.4 | **3.6** / 3.6 | 3.5 / 3.5 |
| emphasis that closes none | **13.4** / 13.2 | **13.4** / 12.1 | 13.2 / **13.3** | 13.3 / 13.2 |
| emphasis closed in part | 13.8 / **13.9** | 13.9 / **14.1** | **14.0** / 13.9 | 14.0 / 14.1 |
| strong emphasis in emphasis | **1.1** / **1.1** | **1.0** / 0.9 | 1.0 / **1.1** | 1.0 / 1.0 |
| a run of spaces | **81.3** / 81.2，**超**；复核 80.9 / 81.8，照样超 | **41.2** / 40.9 | **81.2** / 81.0，**超**；复核 81.9 / 81.4，照样超 | 81.4 / 80.6 |
| an escape, then a run of spaces | **81.2** / 81.0，**超**；复核 81.4 / 81.3，照样超 | **41.3** / 41.1 | **81.6** / 81.4，**超**；复核 81.7 / 81.3，照样超 | 80.9 / 81.3 |
| processing instructions | **75.6** / 75.4 | 74.7 / **75.6** | **75.6** / 74.8 | 75.4 / 74.6 |
| images not closed | 22.0 / **22.2** | 22.1 / **22.3** | 22.1 / **22.3** | 22.1 / 22.4 |
| autolinks not closed | 8.0 / **8.1** | **8.1** / 8.1 | **8.4** / 8.3 | 8.3 / 8.3 |
| link titles not closed | **11.5** / 11.3 | 10.6 / **10.6** | **10.6** / 10.5 | 11.4 / 12.0 |
| code spans of distinct lengths | 2.7 / **2.7** | **1.2** / **1.2** | 2.6 / **2.6** | 2.6 / 2.7 |
| nested emphasis | **11.1** / 11.0 | 11.2 / **11.5** | **11.3** / **11.3** | 11.0 / 11.1 |
| nested brackets | 15.2 / **15.3** | 15.3 / **15.4** | 15.3 / **15.4** | 15.3 / 15.3 |
| marks of emphasis and code only | **6.8** / 6.6 | **6.5** / **6.5** | **6.7** / 6.5 | 6.6 / 6.6 |
| lists to blockDepth（49 层） | **0.9** / **0.9** | **1.0** / 0.9 | **1.0** / **1.0** | 1.0 / 0.9 |
| quotes to blockDepth（98 层） | **1.0** / **1.0** | **1.0** / 0.9 | **1.0** / **1.0** | 0.9 / 1.0 |
| a link reference definition over two lines | **0.1** / **0.1** | 0.1 / **0.2** | **0.2** / **0.2** | 0.1 / 0.1 |
| a definition's bracket never closed, over lines | 45.2 / **45.6** | 55.3 / **55.6** | 54.6 / **55.9** | 43.8 / 43.7 |
| a definition's title never closed, over lines | 73.7 / **75.2** | **85.8** / 85.7，**超**；复核 85.9 / 84.6，照样超 | 85.4 / **88.2**，**超**；复核 85.7 / 89.9，照样超 | 72.9 / 73.0 |
| a JSON array, a definition's bracket never closed | 2.0 / **2.2** | **2.0** / **2.0** | **2.0** / **2.0** | 2.0 / 2.0 |
| a definition's title closed on its last line | **75.4** / 73.6 | 89.6 / **89.8**，**超**；复核 90.0 / 88.9，照样超 | **89.6** / 86.2，**超**；复核 88.6 / 86.7，照样超 | 74.2 / 72.7 |
| a line with a pipe, then spaces | **80.8** / 80.7，**超**；复核 80.2 / 81.0，照样超 | 40.6 / **40.9** | **81.7** / 81.1，**超**；复核 81.2 / 81.2，照样超 | 80.6 / 80.8 |

- **结论：超**（3.9 第 2 条：超的格复核两轮的较大仍过界）。超的五种：一串空格、转义之后一串空格、有 `|` 的一行之后一行空白三种每次解析约 81 ms（超出 1–2%），段尾按键同价，段中插一个字把一串劈成两半、约一半；标题没闭、标题在最后一行闭合两种多行的引用式定义，段中、段尾插一个字 85.8–89.8 ms。改法在条件的 Step P4/S5（决定 511），它的再量照这里的量法（决定 512）。
- **两种标题的定义**每次解析在一页的头十几次约 67–77 ms，之后 80–124 ms（推断是 GC：它们每来一行就拼出一份新的全段文字，一次解析造几十 MB 的垃圾）。插一个字每次量之前先不计时地整篇解析一遍，页里的解析数涨得快一倍，所以插一个字的两格超、整篇一格的中位 75 不超：这个差别来自量法，不来自形状（核对 B 的 C3-M1）；整篇从第 14 次量起过 80，最后 7 次的中位 83–85。打开、粘贴这样一段同样会贵。
- **没超的**：`<?` 连串三格 74.7–75.6（20 次里至多 1 次过 80）；括号没闭的定义 45.6 / 55.6 / 55.9；JSON 数组 2.0–2.2；两行的定义 0.1–0.2；其余十二种至多 22.3。二十万个空格整篇一次解析 1.6–2.0 ms。
- **20 次与 5 次**：同一份数里只取头 5 次，一串空格类与 `<?` 几乎不变（81.4 对 81.3，76.8 对 75.6）；两种标题的定义在段中、段尾是 73–77 对 85–90，括号没闭的 45–47 对 55–56。5 次的中位落在新页里便宜的一段（核对 B 的 C2-I1）。
- **一次解析**：解析器在 CodeMirror 之外一次解析（第四列）与"整篇一次解析"相近，粘贴的路子确是一次解析。
- **旧的量**，留在结果里、不再用来判：`cf95dd9f` 的四轮，"整篇"是两次解析之和（一串空格 171.4、转义之后一串空格 172.9、`<?` 155.4、`|` 172.3；审查 B 的 I1）；`fcaaadba` 的四轮，所有形状在一个页里依次量，定义的几种受先前的解析影响（130–220；核对 B 的 C1-M1）；`4616e2da` 的四轮，每格新页、只量 5 次，定义的几种只取到头几次（72–82）。
- **只记的**：真的编辑器里，整篇的中位最贵的三种上按一个键，`keydown` 到两帧之后（四轮，各五次的中位）：有 `|` 的一行之后一行空白（光标在段首）88.3、85.3、85.4、86.2，其间最长的长任务 87–93；一串空格（第 1,523 个码元处，把连串劈成 1,507 与 8,475）65.2、67.2、64.7、65.3；转义之后一串空格 67.4、66.3、69.4、71.2。修复第 1 轮的各轮在两种标题的定义第一行的行尾按键：92.3–115.1 ms，长任务 92–147 ms。按整篇的中位挑挑不到定义，S5 再量时宜按三格的较大中位挑，或固定加一种定义（核对 B 的 C3-M1 的建议）。
- **反向对照**（4.4 的两条，三种写法各做一次，`cf95dd9f`、`fcaaadba`、`4616e2da`）：去掉上限（`configure([])`），二十万个空格整篇解析 < 1 秒的断言失败（22,322、22,528、22,550 ms）；插一个字改到上限处（插后 10,001 个码元），"上限之内"的断言在第一种形状上失败（段长 10,001、行内节点为空），修复之后段中、段尾两格各做一次。两条碰不到"一次还是两次解析"与"漏了哪种形状"。修复第 3 轮只改次数，没有再做。

**M4-performance 第 12 条：附件列表的重读**（4.7；决定 513）。移交那一次的标签页是附件列表展开到底（30 页，`AssetStore.load` 重读时读回已持的全部页，`W/stores/asset.store.ts:201-221`），4.7 原读成左栏的 3,000 行；两种情形都量，判据取 A 持全部页的那一种（`assets.perf.ts` 的 "every page held"：先往根下传 3,000 个附件，A 的附件一节点"More attachments"到底，其余照 4.7）。

| 轮 | 保存失败的次数 | B 的保存答 429（各次复测） | B 的最后一次保存（发出的时刻，秒；状态） | A 读附件列表（20 秒） | A 的 429 | B 的 429 | A 的长任务占窗口 |
|---|---|---|---|---|---|---|---|
| 1 | 记下 0；B 跟窗口一起停手时 0（复算） | 0、1、0、0、0 | 20.46、20.43、20.48、20.50、20.75，都 200 | 470、471、466、464、479 | 11、8、6、5、14 | 2、2、1、0、8 | 0.79–0.80 |
| 2 | 记下 0；**B 跟窗口一起停手时 3**（复算：第 1、2、3 次） | 2、2、1、0、0 | 20.50 **429**、20.71 **429**、20.55 **429**、20.45、20.53 | 481、488、480、477、483 | 17、21、19、15、16 | 4、6、4、2、2 | 0.81–0.82 |
| 3（复核） | **1**（第 3 次） | 0、0、1、0、0 | 20.45、20.47、20.47 **429**、20.78、20.50 | 467、467、473、469、470 | 9、8、11、9、8 | 3、4、2、1、2 | 0.78–0.80 |
| 4（复核） | **0** | 0、0、0、0、0 | 20.66、20.59、20.47、20.82、20.62，都 200 | 461、472、484、484、472 | 9、5、21、17、14 | 2、2、4、3、2 | 0.78–0.82 |

- 每轮 5 次有效、0 次作废；窗口里的保存每次 6 次（2.2–17.4 秒）；上传每次 1,200 个都 201；每次复测一本新的笔记本（根下 3,000 页的 `flat3k`）。
- 正式两轮（`4b1b4439`）的 B 打到上传窗口之后 3 秒，在约 23.5 秒多一次保存，那时桶已回满，十次都答 200，替被拒的那次补上（审查 C 的 I1）；B 跟窗口一起停手时记录里每个答复都不变，被拒的就是最后一次停顿的保存，复算是 0 / 5、3 / 5，判"超"。复核两轮用 `e28f220e`（B 跟窗口一起停手，每次复测记最后一次保存）：1 / 5、0 / 5。**结论：第一次超、复核过（3.9 第 2 条），记成已知限制**（决定 513，负责人可以改判）。复核的第 3 次三条判据都成立：429 之后 10 秒之内没有成功的保存，页的正文少了最后打的字，编辑器停在保存失败。
- **机制**：附件一节重读时读回已持的全部页，每个带 `tree` 的 `pages` 事件（至多半秒一次）之后都重读；持着 30 页的标签页在别人大批上传时每秒约 24 次列表读、主线程约八成被占，与同一登录的保存共用一个凭证的桶，桶在上传窗口的后段见底（推断）。用户最后一次停顿的那次保存答 429；编辑器不自己重试 429（`W/stores/page-editing.ts` 只重试 503），显示"保存失败"，直到下一次保存。被拒的与答 200 的最后一次保存时刻重叠（20.43–20.82 秒），记录里没有桶的状态，看不出分界。
- **读者的退路**：字还在编辑器里，再打一个字或按 Ctrl/⌘+S 就补上；离开这一页或关、重载标签页时，没存的编辑会先问（`W/pages/page/unsaved-guard.tsx`）。
- **v0.2 的候选**：429 之后照 `Retry-After` 自己重试一次；或照移交说的，附件一节重读之前"先比较 id 与名称再读"。v0.2 改完用 "every page held" 照两轮再量。
- **4.7 的情形**（A 只持附件列表的第一页，每次重读一个请求）：两轮 0 / 5，A 每 500 ms 读一次树、随后读一次附件列表，20 秒恰 40 次，没有 429，A 的长任务占窗口 0.21–0.24；B 的保存都答 200（中位 11–16 ms）。这两轮是 `e28f220e` 之前的脚本，保存全部答 200，B 跟窗口一起停手也是 0 / 5（核对 C 的 C1-N2）。轮 1 重量过（`assets-r1b`）：原来的轮 1 与别人不经锁的 vitest 重叠，作废。

**M7-transfer 第 4 条：任务列表的轮询**（4.8，只记）。长任务用导入（4.11 第 9 条）：每个 N 一本新的笔记本，60 个小导出之后导入 `folder50k`（5 万页都在一个文件夹之下；12,079,206 字节，sha256 `60b2fbb640371dc210bb153b80c8ba36b3c68da0fd4bc8909bbd78f77528cccd`，正式两轮之后复算），窗口 60 秒里导入一直在运行。

| 轮 | N | 每个标签页 60 秒里读任务列表 | 读树 | 429 |
|---|---|---|---|---|
| 1 | 1 | 116 | 120 | 0 |
| 1 | 3 | 118、118、116 | 各 120 | 0 |
| 2 | 1 | 116 | 120 | 0 |
| 2 | 3 | 118、118、116 | 各 120 | 0 |

- 有任务在跑时每个标签页每分钟读任务列表 60 × 已加载的页数（两页 116–118 次），写树的任务另每分钟读树 120 次（`TREE_INTERVAL_MS` 500 ms）。三个标签页、同一个凭证每分钟约 712 次，在 1,200 的桶之内。
- **推算**：只有导出在跑时，所有标签页合计持着 20 页就到每凭证每分钟 1,200 次；导入时每个标签页两页的话约 5 个标签页（5 × 240）。导出 5 万页在本机 1 秒之内做完（审查 C 的探针 0.57、0.62 秒）。

**M4-performance 第 7 条：补全读链接目标**（4.5，只记）：

| 形状 | 链接目标的字节 | Node 一侧读一次（五次的中位，轮 1 / 轮 2） | 浏览器里 `[[` 的第二个 `[` 到补全列表画出（五次的中位，轮 1 / 轮 2） |
|---|---|---|---|
| 平 1 万页 | 1,311,247 | 23.5 / 24.1 ms | 760 / 507 ms（列表各 100 项） |
| 平 10 万页 | 13,316,241 | 211.5 / 211.2 ms | 量不到：打开这一页时渲染进程崩溃（两轮都是） |

- 轮 1 的补全比轮 2 慢一半，原因未明：轮 1 在 `cf95dd9f` 加未提交的 `links.perf.ts` 等上跑（14:09 随 `b9c45dd6` 提交），轮 2 在 `b9c45dd6` 上；两轮记下的字段相同。1 万页上打开编辑器（地址到编辑器得到焦点）5.2–6.3 秒，多半是左栏一万行。

**M4-performance 第 11、13 条：阅读视图的大图**（4.6）：挪到 S3（决定 518）。`images.perf.ts` 在 S2 试跑过一次（U3 之前的代码，不是正式的量）：20 张 4,000 × 4,000 的 PNG 共 5.27 MB，滚到底之后渲染进程的 `VmHWM` 97.0 → 646.2 MiB，GPU 进程 107.4 → 140.6 MiB，JS 堆 7.6 MiB；4.6 的"先核对"由此有了答案：涨的是渲染进程。一轮约 50 秒。

**5 万、10 万页**（4.9，只记；决定 514）：

| 形状 | 导入 | 树的 JSON 字节 | gzip 1 / 6 | zstd 3 | 冷打开 | JS 堆 |
|---|---|---|---|---|---|---|
| 平 5 万页 | 一个包 11.4 MB，130 秒 | 12,294,679 | 1,264,490 / 1,187,897 | 1,282,976 | T1 19,318 ms（5 万行都画出） | 2,984 MB |
| 平 10 万页 | 两个包，136 + 184 秒（轮 2：128 + 172 秒） | 24,587,759 | 2,529,915 / 2,382,012 | 2,570,187 | **渲染进程崩溃**，请求地址之后 31.3 秒 | — |

- 平的形状的堆与行数大致成正比（1 万页 604 MB）；10 万页推算约 6 GB，过了 Chromium 每个渲染进程的 V8 堆上限（64 位上约 4 GB，推断）。树的 JSON 约每页 245 字节，gzip 1 级之后约每页 25 字节（三种规模都是 9.7 倍）。
- 记成已知限制，分层加载进 v0.2；已知限制与 v0.2 的清单由 S3 写（10.3）。

**M7-transfer 第 6 条：附件很多时节点树的大小**：页面树带附件的形状里，1,000 个附件让树的 JSON 从 3,156,470 增加到 3,423,296 字节（每个附件节点约 267 字节），T1 204 / 210 ms 对深的 196 / 198 ms，堆 18.3 MB 对 17.4 MB；左栏不画附件。

**机器时间**（4.10 推断 3–3.5 小时）：审查之前量的持锁约 1 小时 55 分（页面树 13 分、B9 的试跑与对照 5 分、其余五段 97 分；另有逐个提交的核对与门禁），导入与搜索的队列都比推断快；修复三轮另约 70 分钟（附件列表的复核 30 分、B9 三次重量与 T3 约 40 分）。

### 10.3 S3：服务端

| 行 | 规模 | 种子 | 两轮的数 | 判在 | 预算 | 结论 | 旧倍数的结论 | 去处 |
|---|---|---|---|---|---|---|---|---|
| B1 | | | | | | | — | |
| … | | | | | | | | |

其后：B5 硬期限的两行推断、只量的各项（3.7 的表）、"给部署文档"一节（第 9 节的对照表）、已知限制与 v0.2 的清单。

---

## 附录：核对过的代码

本文引的每个行号都在 `86565dd6` 上读过（第一、第二次修订新引的在最后两条）：

- 搜索：`bootstrap/search_bench_test.go:1`、`:31`、`:47`、`:65-77`、`:158`、`:271-278`、`:282-289`、`:293-300`、`:332`、`:365-387`、`:469`；`bootstrap/save_bench_test.go:19-59`。
- B5：`bootstrap/trash_cost_test.go:42`、`:55`、`:86-151`、`:196-199`、`:307-338`、`:352-369`、`:449-483`、`:511-539`；`bootstrap/revert_cost_test.go:29`、`:39`、`:237-299`、`:418-485`、`:504`、`:567-623`、`:649-670`；`bootstrap/background_cost_test.go:28-80`、`:414-429`、`:466`、`:566-650`、`:788-798`、`:910-941`；`page/domain/trash.go:47-88`、`revert.go:58`、`jobs.go:29`、`:37`。
- lint、`index`、升级：`bootstrap/wiki_lint_cost_test.go:25-70`、`:110-141`、`:183`；`bootstrap/wiki_lint_cost_fixture_test.go:40-81`；`linking/adapter/postgres/index_cost_test.go:31-51`、`:121`、`:232`、`:320-330`；`bootstrap/upgrade_start_cost_test.go:31-70`、`:89`、`:100-125`、`:150-160`；`bootstrap/upgrade_check_cost_test.go:20-36`、`:58`、`:142`；`bootstrap/upgrade_approve_cost_test.go:36-84`、`:350-360`、`:405-415`；`page/adapter/postgres/history_cost_test.go:1`、`:30-110`、`:161`、`:281`、`:296`；`page/adapter/postgres/overview_cost_test.go:39-81`。
- 配置与组合根：`server/configs/config.yaml:5`、`:11-23`、`:36-39`、`:46`、`:72-85`、`:97-98`、`:144-161`；`server/configs/config.prod.yaml:1-4`；`server/configs/config.test.yaml:26-28`；`platform/config/validate.go:50-58`、`:178-181`；`bootstrap/app_test.go:44-66`；`bootstrap/permission_matrix_test.go:311-330`；`bootstrap/source_cost_test.go:78-95`；`bootstrap/mcp.go:30`、`:40-41`；`bootstrap/mcp_bodies_test.go:118-128`；`bootstrap/wire.go:253`、`:263-264`、`:281`、`:297`；`bootstrap/deps.go:253`；`page/adapter/http/limits.go:8-17`；`platform/markdown/budget.go:86-106`；`platform/httpserver/mcpserver/edge.go:77`、`:86`；`platform/httpserver/mcpserver/values_test.go:10-40`；`page/domain/content.go:12`；`linking/domain/facts.go:114`；`page/domain/tree.go:9-36`；`pgtest/pgtest.go:35-36`、`:88`；`server/migrations/sql/00042_page_nodes_path_keys.sql:1-18`、`00054_page_content_compression.sql:3-4`、`00048`、`00050`、`00055` 加的列（`git diff --diff-filter=A 308eafc6 86565dd6 -- server/migrations/sql`）。
- `server/tools`：`server/tools/go.mod`；`server/.golangci.yml:3-54`；`Makefile:145-156`、`:164-169`、`:204`、`:272-288`、`:321`、`:365`、`:375-403`、`:405-600`；`.github/workflows/ci.yml:23`、`:88-117`、`:160-171`。
- 起服务的先例：`deploy/image-smoke.sh:34`、`:83-95`、`:116-144`；`server/cmd/nervewiki/commands.go:31-97`。
- e2e：`e2e/playwright.config.ts:3-28`；`e2e/global-setup.ts:15-22`；`e2e/fixtures/db.ts:13-14`、`:45-60`、`:88-100`；`e2e/fixtures/server.ts:26-173`；`e2e/fixtures/navigation.ts:21-29`、`:97-153`；`e2e/fixtures/browser.ts:133`；`e2e/fixtures/auth.ts:7-160`；`e2e/fixtures/notebooks.ts:20-45`；`e2e/fixtures/pages.ts:19-47`；`e2e/fixtures/transfer.ts:64`、`:239`；`e2e/fixtures/zip-write.ts:34`；`e2e/fixtures/notebook-pages.ts:62-64`；`tools/e2e-navigation/check.mjs:19-56`、`:133-148`；`knip.jsonc`；`pnpm-workspace.yaml:2-5`；`.gitignore:27-28`；`node_modules/.pnpm/@testcontainers+postgresql@12.2.0…/postgresql-container.js:87-105`（不加 `fsync=off`）。
- 网页：`W/stores/page-tree.store.ts:78`、`:160-192`、`:276-308`、`:655-663`；`W/stores/page-tree.ts:19-35`、`:116`、`:124`；`W/pages/notebook/page-tree.tsx:29-34`、`:63`、`:126-174`；`W/pages/notebook/page-tree-item.tsx:68-170`；`W/pages/workspace/workspace-layout.tsx:111-113`；`W/pages/notebook/notebook-home.tsx:38-74`；`W/events/handlers.ts:36`、`:62-115`；`W/events/refresher.ts:11`、`:59-77`；`W/app/routes.tsx:144-158`；`W/editor/markdown.ts:28-102`；`W/editor/markdown.test.ts:1-6`、`:54-226`；`W/editor/autosave.ts:1-25`；`W/editor/loaded/link-completion.ts:73`、`:205`、`:321-339`；`W/services/linking.service.ts:69-76`；`W/pages/page/attachments-section.tsx:76`；`W/stores/asset.store.ts:201-238`、`:494-530`；`W/pages/notebook/transfer-page.tsx:37-56`、`:180-182`；`W/stores/edit-session.ts:18`；`platform/markdown/obsidian/asset.go:115`、`:136-161`；`web/apps/web/vite.config.ts:22-40`。
- 契约：`api/dist/openapi.yaml:261`、`:348`、`:1029-1051`、`:1128`、`:1167`、`:1237`、`:2340`、`:2591`、`:2623`、`:2670`、`:2778`、`:2824`、`:3540-3546`、`:3937-3938`。
- 文档：`docs/v0.1/v0.1-design.md:1172`、`:1228`、`:1297-1319`、`:1317`；`README.md:500`、`:533`、`:540`；`docs/v0.1/M8-history-search/05-P5-search.md:894`、`:900`、`:1093`、`:1102-1107`、`:1123-1127`；`docs/v0.1/M8-history-search/02-P2-trash.md:470`、`:486`、`:496`；`docs/v0.1/M9-mcp/02-P2-writes-history.md:353`；`docs/v0.1/M9-mcp/03-P3-batch-revert.md:1189`；`docs/v0.1/M4-pages/reviews/P4-content-sessions-review.md:92`；`docs/v0.1/M12-release/handoffs/M4-performance.md:378`、`:392`。
- 第一次修订另读的（审查给的出处，设计者在 `86565dd6` 上核过）：`308eafc6` 的 `search_bench_test.go`（没有 `NERVE_SEARCH_BACKLOG`，`corpusTables` 与今天同字）；审查 A 的 `p4-review-a/`（竞态下编不过、gctrace 被插断、vault 的大小）、B 的 `p4-review-b/`（作业的日志、`-benchtime` 的探针）、C 的 `p4-review-c/`（knip、Vite、T1）。
- 第二次修订另读的（在 `1ba6c7f6` 上核过；代码与 `86565dd6` 相同）：`bootstrap/contract_test.go:381`、`:385`；`bootstrap/permission_matrix_test.go:311-330`；`bootstrap/upgrade_approve_cost_test.go:37-67`、`:217-247`、`:318`、`:357-360`；`bootstrap/upgrade_start_cost_test.go:42-54`、`:111-116`、`:220-230`；`bootstrap/upgrade_check_cost_test.go:21`、`:142`；`bootstrap/wiki_lint_cost_test.go:31-34`、`:183`；`bootstrap/wiki_lint_cost_fixture_test.go:53`；`bootstrap/trash_cost_test.go:376-415`、`:449-487`；`Makefile:272-365` 的八份的模式；`knip.jsonc`；`web/apps/web/package.json:19`、`:22`、`:53`；`W/app/problem-messages.ts:128-129`；B5、B6、B10 的窗口列的 `page`、`linking`、`wiki` 模块的文件名；`docs/v0.1/v0.1-design.md:1276`、`:1327`；`docs/v0.1/M8-history-search/02-P2-trash.md:497-506`；`00-M12-design.md` 的第 112、1076 行、"总体设计的修订"第 9、24 条与决定 378、424、451、452；[P2 设计](02-P2-web-polish.md)第 9 节与 4.6.6；[P3 设计](03-P3-server-correctness.md) 3.10.3；[P5 设计](05-P5-deploy-release.md) 3.18 与 2.1 第 20、21 条；修订核对的探针：A 的 `p4-review-a/probe2.*`（竞态下引用 `!race` 的常量编不过），C 的 `p4-check1-c/b9/`（解析页的三种写法），第二次修订自己的 `p4-rev2/b9/`（两个包的写法）。
