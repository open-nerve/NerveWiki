# M11/P2 S2（闸门、检查与提交，服务端）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m11-p2-s2`（实施的 10 个提交：组合根的第八份（第 261 条）`9ff43717`，`TestTheMCPCallStopsWhenTheClientGoes` 的两个日志等待 2 秒 → 10 秒（S1 的审查记录"推后"的那一条）`967fdbea`，`Runs` 认升级的三种运行、检查的类型、第二遍另成一项 `5d804808`，page 的四条闸门的读 `8fa3b7c7`，八道闸门、计数与报告的判定 `29b1124a`，检查与条目（`checkUpgrade`、`listUpgradeCheckItems`）`223e7d09`，提交与存下的报告（`submitUpgrade`、`Upgrade.report`、`wiki.upgrade_not_ready`）`74a3057a`，MCP 的 `upgrade_verify`、`upgrade_submit` `d8d2bbaf`，两个成本守卫 `a6e8cb67`，e2e 的 UP5 `c3c35bbc`；从 `main` 的 `ba72cda4` 开始，审查时在 `c3c35bbc` 上，96 个文件、加 10,192 行、减 309 行；推送之前 7–10 号改写过两次，旧的都没推过。修复一轮 9 个：第 272 条 `d7c82c01`，第 275 条 `f962a593`，P2S2C-M1 `701c736d`，B 的 4.7 `e88cf289`，P2S2A-M1–M3、P2S2C-M2、N6 `48bb2e18`，P2S2B-M1 `3893c318`，第 274 条 `968bb586`，P2S2A-N2、P2S2C-N4 `debec8dc`，P2S2C-M3 `b6ef8599`（24 个文件、加 329 行、减 59 行）；修复第二轮 1 个：P2S2C-M4 `9dce4658`（3 个文件、加 42 行、减 8 行）；修复第三轮 1 个：P2S2C-M5 `ef8ac10f`（5 个文件、加 66 行、减 25 行）；修复第四轮 1 个：P2S2C-M6 `d4324ab8`（5 个文件、加 489 行、减 167 行）；修复第五轮 1 个：P2S2C-M7、N9、N10 `43caa32a`（4 个文件、加 58 行、减 12 行）；后四轮都只改测试；一共 23 个；合并 `c61df45a`），对照 [02-P2](../02-P2-guardrails-gate-submit.md) 的 S2（3.7–3.18、4.2、第 5 节的 UP5、6.2、7.1–7.3 的 S2 各项、7.5）与 [00 号文档](../00-M11-design.md) 4.7、4.8、4.13–4.18、第 5、9 节 |
| 审查方式 | 三位审查者（Opus）并行、只读，各在自己的分离的工作树里做实验（前几个 Step 的三个工作树挪到 `c3c35bbc`），重的都经 `heavy.sh`，先读设计与代码、自己形成判断之后才看实施者的报告：检查与提交的正确性（A：`Runs` 的键与合并、第二遍分开问、八道闸门与计数的判定、检查的码序与快照、提交的两个事务、期限、"正在计算"与接管、交付之前的判定、日志；平台的 `outer`、`deadline` 与 `tx.go`，核实实施者第 7 节第 11 条；单元 7 条、组合根 4 条变异），读、成本与份（B：page 的四条读的语义与计划、两个成本守卫与判计划的帮手、八份的归类与第 272 条的挪法；3、6、7、8 号各自 build、vet、`make lint-go`、`make gen-check`、short，8 号另跑竞态的 `^TestToolStrings` 与 `TestUpgradeToolsErrorsCarryTheirCodes`；第七、八份本机各一轮；闸门的读的计划守卫上 6 个改坏生成的 SQL 的变异、检查的守卫上 1 个；run 42882105107 的 15 个 server 作业与 `main` 上六次运行的日志；第二份慢 5% 的原因（`apitest` 包里一个临时测试量读契约的时间）；`TestARestoreJobSkipsAnItemRestoredBetweenItsBatches`），契约、MCP、矩阵与码、网页的文字、e2e 与文档稿（C：三个操作与十几个 schema、报告的编码、两个工具与 `Fit`、注解、说明、"server update"、矩阵与码表、两种语言的文字、UP5；最坏的报告与内置说明的字节实跑；13 条变异与一条组合根的对照；`TestTheMCPCallStopsWhenTheClientGoes` 的根因：`mcpserver` 包里一个临时测试 300 次，组合根的测试二进制在占满 CPU 下 40 次） |
| 日期 | 2026-11-11（实施、审查、五轮修复与修复核对） |
| 结论 | Important 0。Minor 9：提交的"过没过"只被 `frontmatter`、`agents` 两道闸门钉住（P2S2A-M1）；闸门 7 的附件一半没有测试（P2S2A-M2）；提交在快照里的核对没有测试，去掉它，排队时被放弃会记一行假的缺陷（P2S2A-M3）；设计的 MCP 期限说法错了，实施者的纠正对，条件要写对（P2S2A-M4）；判计划的帮手放过"内侧每行按笔记本读整本的 `nodes`"（P2S2B-M1）；costs-3 的 `-timeout 20m` 一行在 EPYC 7763 上贴着作业的 35 分钟（P2S2B-M2）；`TestTheMCPCallStopsWhenTheClientGoes` 的偶发不是等得不够久，根因是 SDK 已知的竞态，测试断言了平台不承诺的一支（P2S2C-M1）；`wanted_now.count_capped` 四层都没有测试（P2S2C-M2）；e2e 的 `expectUpgrade` 断言"提交的 id 恰在审阅中"，不是表的不变式，设计第 5 节本身写错（P2S2C-M3）。Nit 11：提交超时的 `Retry-After: 1` 让重试落进"正在计算"（P2S2A-N1）；`lintQuery` 对摘要带第二遍既不拒也不跑（P2S2A-N2）；设计 3.7 的文件表（P2S2A-N3）；`Makefile` 的计数注释停在开份的提交（P2S2B-N1、P2S2C-N5）；第 7 节第 11 条说过了头（P2S2C-N1）；留给文档提交的清单漏了约十处（P2S2C-N2）；10.2 草稿与总体设计的句子的几处（P2S2C-N3）；`stale_index` 的服务端文字的语气（P2S2C-N4）；测试的两处小事（P2S2C-N6）；MCP 矩阵的提交一格不读它留下的行（P2S2C-N7，不改）。Q 4：`Retry-After` 改代码还是写进 playbook（P2S2A-Q1，决定 274）；第 272 条照字面挪会让 `TestVirtualExport` 跑两次（P2S2B-Q1，决定 272 细化）；契约每长一截组合根各份都慢一截（P2S2B-Q2，交给 P3）；`TestTheMCPCallStopsWhenTheClientGoes` 的改法放在哪里（P2S2C-Q1，修复一轮）。实施者的 26 条偏离三位各在自己的范围里判过，核对过的都对；第 11 条的条件由第 273 条写对，第 26 条的根因由 P2S2C-M1 查清。修复一轮照主持者的修复说明 `fix-1.md`（决定 272–275）九个提交，审查者活下来的变异都死；修复核对一轮三位都判旧发现关上，另有新的 P2S2B-N2（Nit，不改：新测试断言了共用帮手的口子）、P2S2C-M4（Minor：修复一轮让三个夹具的 `wanted_before` 与 `wanted_now` 相同，两个成员弄混测试照过）与 P2S2C-N8（Nit，文档稿的测试名与格数，本提交改）；修复第二轮（`fix-2.md`）一个提交关 P2S2C-M4。此后 C 每轮核对都查出同一类的下一处：修复核对第二轮判 M4 关，新的 P2S2C-M5（Minor：存下的报告的五个计数在夹具里同值、MCP 投影的计数只断言两项，S2 建成时就有）；修复第三轮（`fix-3.md`）一个提交让检查与报告的计数在每一层互不相同、整值断言；修复核对第三轮判 M5 关，新的 P2S2C-M6（Minor：REST 报告的规则的数、`preserved` 的 `nodes`、MCP 投影的闸门）；修复第四轮（`fix-4.md`）不再按成员补，按投影收尾：S2 新写的七个投影的夹具每个标量互不相同、整值断言，555 条对调与常量的变异全死，没有发现投影本身有错；修复核对第四轮判 M6 关、七个投影里这一类收尾（C 整轮重跑 555 条全死），再把范围放宽到 S2 写的所有逐字段转换，新的 P2S2C-M7（Minor：组合根把 page 的 `Preserved` 转成 wiki 的那一行，`NodesKept` 写成 `Nodes` 整个程序照过，删掉的基线附件会过闸门 7）、P2S2C-N9（Nit：REST 报告条目的 `kind` 写成常量）、P2S2C-N10（Nit：提交的日志的 `user_id` 没有断言）；修复第五轮（`fix-5.md`）一个提交三条都改，转换本身都没有错；修复核对第五轮判 M7、N9、N10 都关上，再补查 S2 其余的逐字段转换（ws1、wl1、mv1 死，wb3 是等价变异），这一类收尾；新的 P2S2C-N11（Nit，不改：`TestUpgradeGates` 以 SQL 改库的两格把还原写在 `check` 里，那一格先 `Fatal` 时后面各格跟着红）。修复核对到第五轮为止，没有 Important、Minor 留下 |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| 编号 | — | （编号照审查者的报告：P2S2A-M1–M4、N1–N3、Q1，P2S2B-M1、M2、N1、Q1、Q2，P2S2C-M1–M3、N1–N7、Q1；B 的 4.7 不是发现，照修复说明一并改，单列一行；修复核对的新发现接着编：一轮 P2S2B-N2，P2S2C-M4、N8；二轮 P2S2C-M5；三轮 P2S2C-M6；四轮 P2S2C-M7、N9、N10；五轮 P2S2C-N11。修复一轮照主持者的修复说明（`p2s2/review/fix-1.md`）在 `c3c35bbc` 之上加九个提交（f1–f9），第二至五轮照 `fix-2.md` 至 `fix-5.md` 各加一个，不改写已推的历史。下面的代码位置：审查时的发现是 `c3c35bbc` 的，修复核对一至五轮的是 `b6ef8599`、`9dce4658`、`ef8ac10f`、`d4324ab8`、`43caa32a` 的；路径相对 `server/internal/modules/`，`bootstrap/`、`platform/` 在 `server/internal/` 之下） | — |
| P2S2A-M1 | 中（测试） | 提交的判定 `wiki/app/submit_upgrade.go:232`（`case !check.Passed:`）只被两道闸门钉住。实施者的变异 o1（只看 `frontmatter`）第一轮没抓到，于是单元与组合根各补"只有 `AGENTS` 不过"一格；补完之后，单元里不过的格是 `frontmatter, preserved` 与 `agents`，组合根里是一页缺 `type` 与 `AGENTS` 只有 frontmatter，矩阵的管理员一格两道都不过。A 的变异 a6（`!check.Gates.Frontmatter \|\| !check.Gates.Agents`）在 `TestSubmitUpgradeAnswersInOrder` 与 `TestUpgradeSubmit/the codes in their order` 下都活：这样的提交在 `source_root`、`reserved_titles`、`sources`、`links`、`preserved` 不过时照样存下报告、停在审阅中。检查一侧没有这个缺口：`passed` 由 `JudgeUpgrade` 算，`TestJudgeUpgrade` 每格断言一道单独不过时为假 | 已改（`48bb2e18`，f5）：`TestSubmitUpgradeAnswersInOrder` 末尾加一张表，从通过的检查出发每格只让一道闸门不过：`agents`、`source_root`、`reserved_titles`、`frontmatter`、`sources`、`links`、`preserved` 七格（`index` 由已有的 stale 一格盖住，检查先答 `stale_index`），各答 `wiki.upgrade_not_ready`、`detail` 恰列出那一道、写下退回的三步；`checkFake` 为此加三个字段；只加单元的格。a6 改前活、改后死在 `source_root`、`reserved_titles`、`sources`、`links`、`preserved` 五格。修复核对一轮：A 关，另加的 a6b（只放过 `preserved`）也死 |
| P2S2A-M2 | 中（测试） | 闸门 7（`wiki/domain/upgrade_check.go:243-245`）的四个条件里，`NodesKept == Nodes` 与 `Nodes == BaselineNodes` 只在一个附件（基线节点、不是页）被删或它的基线行没了时才单独起作用；`TestJudgeUpgrade` 的两格都让一页消失（节点与页一起少一），`TestUpgradeGates` 只有"以 SQL 改一页的正文"一格。变异 a1、a2 在 `wiki/domain`、`wiki/app` 与组合根的 `TestUpgradeGates` 下都活：一个基线附件被删（守卫拦着，只有缺陷或绕过单元才到得了，正是这条绊线要抓的）时检查判过。a3（去掉 `Pages == BaselinePages`）也活，但它在 `Nodes == BaselineNodes` 之下冗余 | 已改（`48bb2e18`）：`TestJudgeUpgrade` 加三格，经帮手 `withAttachment` 让基线带一个附件（`BaselineNodes` 比 `BaselinePages` 多一）：附件保留着，过；附件已删（闸门 7 单独不过，`NodeIDs` 是它）；附件的基线行没了（分母取 wiki 行的 5，单独不过）。a1 死在"a baseline attachment deleted"，a2 死在"an attachment's baseline row gone"，两条改前都活。a3 不另加格。本提交把设计 3.10 的表的反例照实写。修复核对一轮：A 关（"行没了"一格的 `Baseline` 仍带着附件，闸门 7 不看它，不算发现） |
| P2S2A-M3 | 中（测试；日志） | 提交的 worker 在快照里核对"这一次升级、`reviewing`、`SubmissionID == submission`、没有报告"（`wiki/app/submit_upgrade.go:111-113`），设计 3.12 第 4 步要它，3.18 要每个新检查有去掉就失败的测试，但只有检查的那一份有。走出的路径：提交的第一个事务提交，flight 排队等名额；管理员放弃，回滚删掉发起补写的 `raw`，行还在 `rolling_back`；flight 拿到名额。有这个核对答 `ErrUpgradeState`、什么也不读；没有它，`checkIn` 照算，读到来源根不是活页，记一行 error "upgrade source root is no live page"，答复一样（第二个事务 `superseded`），但 error 级的日志是假的缺陷报警。变异 a4（只比 `now.ID == up.ID`）在 `wiki/app` 与 `TestUpgradeSubmitInterleavings` 下都活：交错第 7 条的快照取在放弃之前 | 已改（`48bb2e18`）：新的 `TestTheSubmissionsSnapshotIsItsOwn` 三格：第一个事务之后、计算的快照之前经 `submitFake` 已有的 `duringRun` 钩子把行改成 `rolling_back`、别的 `SubmissionID` 或已有报告；各答 `wiki.upgrade_state`，第二个事务什么也不写、info 一行 `outcome = superseded`，trace 里没有 `Roots@`、`prepare@`、`Preserved@`。a4 改前活、改后三格都死；修复核对一轮 A 另加的 a4b（比 id 与状态）死在后两格，所以 `SubmissionID`、`Report` 两项也各被钉住；A 关。A 另判核对里的 `SubmissionID`、`Report` 两项实际上到不了，便宜，留着兜底（"接受"） |
| P2S2A-M4 | 中（设计） | 设计 3.12 第 3 步、7.1 的期限一格与 7.2 第 7 条假定 MCP 的请求期限是开始加 `BodyReadTimeout + RequestTimeout`，`submitted_at + SubmitTimeout` 先到。A 核实实施者第 7 节第 11 条属实：工具调用的期限取 `min(路由的期限, r.read + requestTimeout)`（`platform/httpserver/mcpserver/call.go:46-53`），`r.read` 是读完请求体的一刻（`mcpserver/server.go:201`），路由的期限另加 `BodyReadTimeout`、总晚于它；`at` 在读完请求体之后。所以 `request_timeout ≤ 20 s`（默认 15 s，组合根的测试 4 s 与 20 s）时两个入口都是请求的期限先到；`> 20 s` 时 `at + SubmitTimeout` 先到，生产里到得了，不是死代码。只改说法就够：00 号文档 4.8 的两层意图（脱离请求的取消；第二段在 `submitted_at + SubmitTimeout` 之前结束）照样成立，第二段也没有变短（`Do` 的预算约 7.47 秒，第二个事务留约 7.5 秒，两个入口相同）。没写的后果见 P2S2A-N1。设计审查记录核对 P2A-I1 时算的"MCP 4 s + `ReadTimeout` 5 s = 9 s"同样不对 | 决定 273：只改设计的说法，不改代码。本提交改设计 3.12 第 3 步、7.1 的期限一格与"正在计算"的那一段、7.2 第 7 条，00 号文档 4.8 第 2 步，总体设计 13.1 第 5 条的括号；那份设计审查记录是历史，不改，"9 s"的更正写在设计 3.20。MCP 那一半的期限一格现在与 REST 那一半等价，多钉住的是 MCP 把 503 译成工具的错误，留着 |
| P2S2A-N1 | 细节（行为） | 提交超时的 503 带 `Retry-After: 1`（`wiki/app/submit_upgrade.go:130`，`shared.ServerBusy(time.Second)`）。设计 3.12 第 6 步定 1 秒时以为 REST 早答、MCP 恰在可接管时答；现在两个入口都在请求的期限答，离能接管还有 `at + SubmitTimeout − (开始 + RT) ≈ 2.5 s`（测试配置 2 s）。客户端隔 1 秒重试，第一个事务读到"审阅中、没有报告、未过期"，答 409"正在计算"，可那时什么也没在算 | 决定 274：这一处的 `Retry-After` 取到能接管为止的秒数 `max(1 s, ceil(first.at + SubmitTimeout − now))`；lint 的名额满时的 503 不动。单元钉住秒数，组合根期限一格的 `retry_after_seconds` 照新值断言，REST 与 MCP 两个入口都看得到；一个变异取回 1 秒（`968bb586`，f7：`retryAfter` 取 `now` 为答复时的时钟；`Execute` 的说明与契约 `submitUpgrade` 的说明同改，`make gen`，范围带 `api`、`web`；单元 `TestTheSubmissionsDeadline` 加时钟前进的几格钉住秒数，组合根两格断言下界并照它等过、让另一位管理员接管；r274a、r274b 都死）。修复核对一轮：A 关，另加三条改坏秒数算法的变异都死 |
| P2S2A-N2 | 细节（代码） | `linking/app/lint_rules.go:45-52` 的 `lintQuery`：`AreaWantedByPage && WantedByPage` 而 `Rule == nil` 时，摘要的计划只设 `byPage`、不设 `area`，第二遍被静静丢掉；注释与设计 3.8 写的是"`AreaWantedByPage` 只与 `WantedByPage` 同用"。今天没有调用者这样问，是将来的陷阱 | 已改（`debec8dc`，f8）：条件改成 `q.AreaWantedByPage && (!q.WantedByPage \|\| q.Rule == nil)`，答 `errLintQuery`，`RuleQuery` 的说明同改；`TestTheWantedCountsByPage` 加一格；变异 n9 死。修复核对一轮：A 关。本提交改设计 3.8 |
| P2S2A-N3 | 细节（文档） | 设计 3.7 列的 `bootstrap/upgrade_submit_interleavings_test.go` 不存在，`TestUpgradeSubmitInterleavings` 在 `upgrade_submit_test.go:469` | 本提交：3.7 的文件表照实改（连带 P2S2C-N2 第 2 项）；7.2 只写测试名，不改 |
| P2S2A-Q1 | 问题（给主持者） | P2S2A-N1 取哪一种：改 `Retry-After` 的一行，还是只在 P3/S2 的 playbook 里写"答 `server_busy` 之后若再答'正在计算'，等几秒再交" | 决定 274 取前者。playbook 不必再写"等几秒"，只写照 `retry_after_seconds` 等过再交（设计第 9 节） |
| P2S2B-M1 | 中（成本守卫） | 判计划的帮手对 `nodes` 认任何以 `notebook_id = ` 为条件的索引作"按键"（`page/adapter/postgres/rollback_cost_test.go:421-422`），`rollbackJoined` 只要条件里出现外行的某一列（`:431`）；设计 7.1 写的是闸门的读"`nodes`、`page_contents` 只经主键、在嵌套循环的内侧以外行的键读"。变异 gm3（`UpgradePreserved` 的第一个 `LATERAL` 改成按 `n.notebook_id = b.notebook_id AND n.id::text = b.node_id::text` 读）：从未分析的一格里六个计划都没有报问题，测试进了计时，在 `upgrade_cost_test.go:401` 挂住，到 `-timeout 4m` panic；CI 的这一行没有 `-timeout`，要到 Go 默认的 10 分钟才红，红的是超时的栈，不是计划的诊断。这一类合不进来（组合根的检查守卫在 3 秒的 worker 上几秒就红，cm3），所以是 Minor；但计划守卫说它判的它没判，种子小一点、或内侧只按外行的 `parent_id` 读一个文件夹，它就会过 | 已改（`3893c318`，f6）：`upgrade_cost_test.go` 加 `gateNodesProblems`，`TestTheUpgradeGateReadsPlansAtAnyStatistics` 对三条闸门的读的每个计划同时判 `rollbackReadProblems` 与它：每个 `Relation == "nodes"` 的扫描（Bitmap Heap Scan 的每个位图）都经 `nodes_pkey`、条件含 `id = `，判在计时之前，有问题就 `FailNow`；帮手本身不改。另加不要数据库的 `TestTheGateReadsReadANodeByItsKey`（五格手写的计划：按键、按键的位图过；按外行的笔记本、按笔记本与 `parent_id`、Seq Scan 不过），`make test-go-short` 跑到。gm3 改前计划放过、挂到超时，改后由计划判出（14.2 秒，停在计时之前）；B 的未核实的怀疑第 1 条写成 gm7（另按 `parent_id` 读一个文件夹），同样判出。修复核对一轮：B 关；gm7 在 `c3c35bbc` 上也被抓到（规划器选了只有 `parent_id` 的唯一索引），不能作新判法的证据，"按笔记本与文件夹读"那一种只由 `TestTheGateReadsReadANodeByItsKey` 手写的一格盖住，加上 gm3 足够；B 的 gj1（新判法只要有索引就过）死在那个测试 |
| P2S2B-M2 | 中（CI 的时限） | costs-3 的 `-timeout 20m` 一行（`Makefile:413-414`，`TestARollbackDeletesAHundredThousandScatteredPagesFirst`）在 `main` 上落在 7763 的三次（42834026668、42749131694、42718191474）里在作业的第 12.8、14.4、12.7 分钟开始；S2 在它前面加的闸门计划一行在 9V74 上 0.6 分钟、7763 上约 0.75 分钟。于是它约在第 13.5–15.2 分钟开始，自己的 20 分钟超时落在第 33.5–35.2 分钟，最慢的那一种已在作业的 35 分钟（`ci.yml:99-100`）之后，作业先被杀，看不到卡住的栈（`ci.yml` 的规矩："go test 的超时要先于作业时限到"）。P1/S3 那一行留下的，S2 让它再紧约 0.75 分钟；照第 267、269 条"改到时就提" | 决定 275：costs-3 的作业时限从 35 分钟提到 40 分钟，注释写 run 号、runner 与那一行开始的分钟数，写法同 `80da6b1f`；不挪测试、不改界；范围 `ci`（`f962a593`，f2：注释写 7763 上 `main` 的三次作业 16.7–18.4 分钟、那一行在第 12.7–14.4 分钟开始、S2 之后第 33.5–35.2 分钟超时）。修复核对一轮：B 关，最坏时那一行的超时比 40 分钟早 4.8 分钟，40 分钟约是 7763 上作业的两倍 |
| P2S2B-N1 | 细节（注释） | `Makefile:265`："当前 542 个顶层……静态分配为 145/51/121/124/62/11/27/1"是 1 号提交时的数，那时对；到头是 547 个、145/51/121/124/62/11/27/6，注释仍说"当前" | 随第 272 条改（`d7c82c01`）："当前 547 个顶层（竞态构建，M11/P2/S2 第一轮修复挪 TestVirtualExport 的提交时的 go test -race -list）静态分配为 145/50/121/124/62/11/27/7"。修复核对一轮：B、C 关 |
| P2S2B-Q1 | 问题（给主持者） | 第 272 条照字面"`TestVirtualExport` 从 `BOOTSTRAP_SECOND` 挪到 `BOOTSTRAP_EIGHTH`"会让它跑两次：`BOOTSTRAP_THIRD = ^Test(The[F-Z]\|T[i-z]\|[U-Z])`（`Makefile:250`）的 `[U-Z]` 收它，第三份只 `-skip` `SECOND`、`FIFTH`、`SEVENTH`（`:306`），不跳过 `EIGHTH`；B 照 `Makefile` 模拟：`TestVirtualExport` 在第三、八份各一次。不破坏"第八份是第七份的子集"的写法，三个模式一起改；模拟 547 个，145/50/121/124/62/11/27/7，每个恰在一份；各份的命令不用改。只挪它一个就够：本机 51.7 秒，第二份最大的一个（次大的 25.1 秒）；照分支 CI 与本机之比 1.61 折，挪后第二份在 7763 上约 374 秒，第八份约 190 秒；它不用 `t.Parallel`，排在第八份最后跑 | 决定 272 细化：照 B 写三个模式，`Makefile` 的计数注释、`SEVENTH` 与 `EIGHTH` 前的说明、`ci.yml` 的 bootstrap-8 注释、`m11/tools/shards8.py` 随之改（`d7c82c01`，f1，范围 `build, ci`；三个模式与 B 给的一字不差，各份的命令不改；挪后 547 个，145/50/121/124/62/11/27/7，各在恰好一份；旧的 `shards8.py` 留作 `shards8-c3c35bbc.py`）。修复核对一轮：B 照头的 `Makefile` 另归一次，数相同，关 |
| P2S2B-Q2 | 问题（给主持者） | 契约每长一截，所有建应用的组合根测试都慢一截（第二份慢 5% 的原因，"实测"）：`acmeTeam` 每建一次就 `apitest.Load` 一次（`bootstrap/interleavings_workspace_test.go:117`、`mcp_client_test.go:75` 等），竞态构建下读、`Validate`、建路由一份 30 万字节的契约约 0.63 秒，随契约的大小长；第二份里不到 1 秒的小测试，大半就是这一步。P3 再加契约时各份照比例再慢 | 交给 P3（设计第 9 节 P3/S1 的分份一条）：P3 定分份时把这一项算进去；要不要把契约在进程里只读一次是测试设施的取舍，S2 不动 |
| P2S2C-M1 | 中（测试） | `bootstrap/mcp_test.go:272-274` 的第二个等待（"答复没写出"那一行 debug）断言了平台不承诺的一支。两行日志的来处：工具的 ctx 被取消之后 `toolError` 在 `FaultGone` 一支记"MCP client went away"，总会记；"client went away"只在 SDK 对这个请求什么也没写（`!ns.wrote`，`mcpserver/server.go:208-219`）时记。go-sdk v1.8.0 的 `servePOST` 停在 `hangResponse`，请求的 ctx 结束时去 `release`；工具那边交回错误的结果、`deliverLocked` 在同一把锁下判 `s.done`：`release` 先拿到就什么也不写，后者先拿到就把错误写进已断的连接（`net/http` 的写不报错），`wrote` 为真，第二行不会来。SDK 自己写着"there's a brief race between request cancellation and releasing the stream"。CPU 紧时前者被推后，后者常赢。C 的证据：单元级 `GOMAXPROCS=2` 300 次里 73 次 SDK 写了，这 73 次恰好没有第二行；组合根的二进制在占满 CPU 下 40 次失败 5 次，信息与门禁一字不差（11.52–11.64 秒），恰好那 5 次 SDK 写了。M9/P1 修过同一个竞态（`c87caad7`：单元的 `TestAnUnwrittenAnswerIsAProblem` 改成一直不返回的 `stubborn` 与停在 ctx 上的 `slow` 两个工具，`mcpserver/call_test.go:385-389` 写明"the unwritten line is not its to promise"），组合根的这个测试（早一天写的 `4346eed8`）用的正是 `slow`，没有跟着改。2 号的 10 秒只让每次失败多花 8 秒 | 已改（`701c736d`，f3）：删掉第 272–274 行的断言，照 `call_test.go:385-389` 写注释（两种结局都对，"没写出"那一支由单元的 `stubborn` 确定地钉住）；第一个等待的 10 秒留着（无害）；其余断言（1 秒之内结束、"MCP client went away" 在 debug、没有期限的 warn、没有 ERROR）在 40 次里都成立，留着；只改测试，范围 `server`。设计 3.19 交下的那一条由此关上（3.20）。C 的另两种改法（组合根另加一个睡过客户端的工具；改平台让结局确定）不取。修复核对一轮：C 照审查时的复现条件（`GOMAXPROCS=1`、8 个 `yes` 占满 CPU、`-count=40`）重跑，改前 40 次失败 5 次，改后 40 次全过（1.29–4.24 秒），关 |
| P2S2C-M2 | 中（测试） | `wanted_now.count_capped` 在判定（`wiki/domain/upgrade_check.go:289`）、REST 投影（`wiki/adapter/http/check.go:77`）、存与读（`wiki/adapter/postgres/report.go:119`、`:165`）、MCP 投影（`wiki/adapter/mcp/upgrade_projection.go:119`）四层都没有测试钉住：契约只核它在、是布尔；所有夹具里 `WantedNow` 都不封顶（连"最坏的"一份也是 `{Value: 3}`），封顶的只有 `WantedBefore`；组合根与 UP5 只读 `.count`。变异 c1、c11、c12、c13 各对它那一层的整个包跑，都活：一页的待建到 10,000 时显示"10000"而不是"10000+"，审阅的人看到的现在的待建数是下界却不知道 | 已改（`48bb2e18`）：每层一个封顶的夹具并断言它（`TestJudgeUpgradesCounts` 加"the wanted pages now capped"；REST 的 `aCheck()` 与 `reportedView()`；`fullReport`，最坏的报告因此 35,125 → 35,128 字节；MCP 的 `bigCheck`）；c1、c11、c12、c13 改前活、改后死。修复核对一轮：C 关，但三个夹具的 `wanted_before` 与 `wanted_now` 因此相同，见 P2S2C-M4 |
| P2S2C-M3 | 中（e2e；设计） | `e2e/fixtures/assert/wiki.ts:22` 断言 `(u.submission_id IS NOT NULL) = (u.state = 'reviewing')`（`:47` 期望真，`:14` 的注释"kept exactly while it is reviewing"），照的是设计第 5 节"`submission_id IS NOT NULL` 恰在 `reviewing`"，设计本身就写错：`00057` 的 `submission_state_check` 要 `approved` 有提交，`rejected_check` 要驳回有提交，放弃的 `RollBackUpgrade`（`queries/upgrades.sql:52-57`）不清提交的三列，交错第 7 条正断言"提交的三列还在"。S2 的 e2e 到不了这些状态，今天是绿的；P3 的批准、驳回与审阅中放弃的故事一调它就红 | 已改（`b6ef8599`，f9）：那一列改成 `(u.submission_id IS NOT NULL) = ($3::timestamptz IS NOT NULL) AS submission_answer`（`$3` 是答复的 `submitted_at`），注释写明批准、驳回、放弃之后提交都还在；UP1–UP5 共 19 个照过。修复核对一轮：C 关（错的不变式，没有可跑的变异）。本提交改设计第 5 节 |
| P2S2C-N1 | 细节（文档稿） | 实施者第 7 节第 11 条、第 9 节第 2 条与第 11 节 13.1 第 5 条的建议括号写"MCP 的期限总早于 `at + SubmitTimeout`""只有单元测试走到"：`SubmitTimeout = LintTimeout + 10 s`，`LintTimeout = min(10 s, request_timeout/2)`（`bootstrap/wire.go:250`），`request_timeout` 没有上界（`platform/config/validate.go:48-58`），超过 20 秒时 `SubmitTimeout` 恒为 20 秒，`at + SubmitTimeout` 先到。纠正（MCP 不另加 `BodyReadTimeout`）对，"总早于"说过了头 | 本提交照决定 273 写条件：设计 3.12 第 3 步、7.1；13.1 第 5 条的括号写"（`request_timeout` 不超过 20 秒时，默认配置在内，两个入口都是请求的期限较早）"。代码不改 |
| P2S2C-N2 | 细节（文档稿） | 实施报告第 9 节留给文档提交的清单，照设计第 8 节第 3–5 条与报告第 7 节逐条对过，漏了：1 设计第 3 节的"S2 的实施对设计的修正"（3.20）；2 3.7 的文件表（没有 `upgrade_submit_interleavings_test.go`；`upgrade_fixtures_test.go`、`wiki/module.go`、`page/module.go` 没改，模块的两个方法在 `wiki/lint.go:206-215`，`ReservedKeys` 在 `page/upgrades.go:42`）；3 3.12 的第 7 节第 4、5 条；4 3.14 的第 13 条（`inputSchema` 没有 `properties`）；5 3.16 的第 3 条（`ErrorContext`）；6 第 5 节的第 20、21 条、e2e 的个数（S2 是 520 → 522）与 P2S2C-M3 的那一句；7 7.2 第 5 条的第 9 条；8 7.5 的 S2 的实测与第 272 条的挪动；9 v0.1 第 15 节一行；10 00 号文档第 11 节加第 272 条，第 7 节 P2 一行（`00-M11-design.md:516`）还写"10 万页的成本守卫" | 本提交，十项都落：1 加 3.20；2 3.7 的三行；3 3.12 第 5 步之后的两句；4 3.14 的 `inputSchema` 与它前后的说明；5 3.16 的缺陷一行；6 第 5 节的 `expectUpgrade`、UP5 一行的 `edit`、表后一段的 `Page.name` 与个数；7 7.2 第 5 条；8 7.5 的 S2 一条；9 v0.1 第 15 节一行；10 00 号文档第 11 节加 272–275，第 7 节 P2 一行与第 3 节第 5 条照第 262 条写 5 万页 |
| P2S2C-N3 | 细节（文档稿） | 10.2 草稿与报告第 11 节：10.2 的十七条偏离漏了第 7 节第 9 条（交错第 5 条的逼法）；第二个提交那一行"2 秒 → 10 秒（3.19）"读来像把 3.19 交下的事做完了，门禁一条把它记为"与本 Step 无关的偶发"，而 3.19 明确交给了 S2；13.1 第 4 条的码序漏了计算之后的两处（被接管或已放弃的状态的 409 先于计算自己的错误；交付之前再判 404、403，盖过 200 与内容的 409）；13.4 第 8 条的"中位数另有三倍的界"不准：界是本机三倍与 worker 的八成（2.4 s）中较小的（`checkCostBound`，1.5 s） | 本提交：10.2 的偏离照实施者第 7 节的号写全 26 条（同 3.20）；第二个提交那一行写明没有治好、根因与修复；门禁一条写明它是 3.19 交给 S2 的那一个；13.1 第 4 条补那半句；13.4 第 8 条写"界取本机三倍与 worker 的八成中较小的（1.5 s）" |
| P2S2C-N4 | 细节（文字） | `wiki/domain/upgrade_errors.go:36` 的服务端 `detail`"The link index is out of date: a server admin runs nervewiki reindex, then try again."前半陈述、后半祈使，读不顺，MCP 的 agent 读的就是这句。偏离本身（第 7 节第 2 条：不再说"then the upgrade can start"，提交也答它）对 | 已改（`debec8dc`）："The link index is out of date: ask a server admin to run nervewiki reindex, then try again."；原来没有测试断言这句话，`TestSubmitUpgradeAnswersInOrder` 的 stale 一格改为断言整句；网页的文字不动。修复核对一轮：C 关。本提交改设计 4.2 |
| P2S2C-N5 | 细节（注释） | 同 P2S2B-N1 | 同 P2S2B-N1 |
| P2S2C-N6 | 细节（测试） | `bootstrap/upgrade_tool_codes_test.go:15-16` 的注释一行超过一百五十个字符；`TestUpgradeToolsMatrix`（`upgrade_tools_matrix_test.go:95`）不看 `r.entries`，`TestThePermissionMatrixOverMCP`（`permission_matrix_mcp_test.go:610`）看：两行今天都是 `BothEntries`，以后加一个只在一个入口的升级工具时会误跑 | 已改（`48bb2e18`）：重新折行（最长一行 141 个字符）；加同样的 `if r.entries&entry == 0 { continue }`。修复核对一轮：C 关 |
| P2S2C-N7 | 细节（测试） | `TestUpgradeToolsMatrix` 的 `upgrade_submit` 管理员一格不读它留下的行；REST 的同一行有 `left` | 不改：别处已盖住（`TestUpgradeToolsErrorsCarryTheirCodes` 在两个入口之后读提交的行是否退回 `upgrading`，`TestUpgradeSubmit` 的码依次也读），只是不对称 |
| P2S2C-Q1 | 问题（给主持者） | `TestTheMCPCallStopsWhenTheClientGoes` 的改法放在哪里：建议 S2 的第一轮修复（只改第五份的一个测试，几行），同时关上设计 3.19 的那一条 | 放在修复一轮（P2S2C-M1 的处置） |
| B 的 4.7 | 细节（测试，M8 留下的） | `TestARestoreJobSkipsAnItemRestoredBetweenItsBatches`（第一份）在第一轮 `make test` 的负载下失败一次：请求的恢复答 409 `page.trash_changed`，作业已做完 8/8（`logs/gates-1/test.log:97-99`）。不是产品的竞态，是测试的期望：请求的恢复先在锁外读回收站（`page/app/restore_node.go:65-132`），到 `:133` 才等锁；测试放开之后，PostgreSQL 的行锁不保证排在前面的等待者先拿到，作业提交了这一批、立刻要下一批的锁，可以抢在被唤醒的请求之前；这时第三批恢复了 C6，请求拿到锁之后重读，答 `page.trash_changed`（`unit_restore.go:49-58`，`restore_node.go:59-64` 的注释写明了这一码）。测试为"第三批在先"留的分支（`interleavings_background_test.go:271-275`）只认 `page.not_in_trash`。同一写法还在 `TestARevertJobSkipsAMemberRestoredBetweenItsBatches`（`:591-605`）。S2 没有碰这条路径，最后一次改它是 M8/P4A | 已改（`e88cf289`，f4）：两处"第三批在先"的分支同时认 `page.trash_changed`，作业的断言照旧（恢复：`succeeded`、8 页、`skipped` 0；撤销：`succeeded`、恢复 8、`skipped` 0），照旧 `continue`；只改测试，单独一个提交，说明写明是 M8/P4A 的测试、负载下的次序。修复核对一轮：B 关，只放宽了该放宽的那一支（作业若跳过了 C6 仍答这个码，作业的断言会失败；主路径照旧） |
| 偏离 | — | 实施者的 26 条偏离（[02-P2](../02-P2-guardrails-gate-submit.md) 3.20、10.2）：1 检查的类型在 3 号提交；2 `wiki.upgrade_stale_index` 的服务端 `detail` 不说是哪个动作；3 `Logger` 加 `ErrorContext`；4 第二个事务每种结局都照码序判，锁不到笔记本答 `notebook.not_found`、不再判；5 组合根的 `SetState` 认 `upgrading`；6 提交的答复的 `submitted_by` 照用 `named`（不是偏离）；7 覆盖测试认 `/upgrade/check/{rule}`；8 第二位管理员 dana 是笔记本的管理员；9 交错第 5 条用 `interleaveOn`；10 两个组合根测试的 MCP 一半在 8 号；11 MCP 的期限是读完请求体之后的 `RequestTimeout`；12 第一个事务的期限一格在 8 号；13 `upgrade_submit` 笔记本入口的 `inputSchema` 没有 `properties`；14 流的提交两格、各两帧；15 报告的条目不带 `path`；16 计数拆成 `TestJudgeUpgradesCounts`；17 `TestUpgradeToolsErrorsCarryTheirCodes` 的几种情形；18 检查的守卫的另一份种子改账本；19 检查的守卫在 costs-2 的末尾；20 UP5 审阅中的冻结用 `edit`；21 UP5 的页名用 `Page.name`；22 变异 46 改在 `LintCursor.PositionFor`；23 MCP 矩阵读的行也每格一个库的副本；24 8 号的范围带 `e2e`，改 `validToolArguments` 与 LW8 的工具数；25 交错第 7 条读放弃之后的行不看回滚到了哪一步；26 2 号的 10 秒没有治好 `TestTheMCPCallStopsWhenTheClientGoes` | 三位各在自己的范围里判过：A 核对 4（照 13.1 第 4 条的码序 404、403 在 409 之前；锁不到笔记本答 404 同第一个事务的锁会答的）、5（必要，变异 o5 抓到）、11（属实，即 P2S2A-M4）、12（与设计 7.1 末段一致）、16（变异 33 由它抓到）、22（真的绑定在 `PositionFor`，用例只交根，变异只能改在那里）、25（改后仍抓得到它该抓的：第二个事务写了什么就从 409 变成 500，48、58 在改后的断言上重跑都死）；B 核对 18（改账本与改正文对闸门 7 一样，改正文还会动索引与 lint 的数，反而不是只测闸门 7）、19（末尾对 costs-2 的余量无差别）；C 核对 2（旧的"then the upgrade can start"对提交是错的；语气见 P2S2C-N4）、7（另有反例：不在 `upgrade` 之下的 `/check/{rule}` 照旧报出）、13（账户入口的 `notebooks` 自 M9 就是这样）、15（存下的路径会随改名、移动与回滚变旧，带路径最坏的报告会过 64 KiB）、20（`write` 不带 `base_version` 是创建，先答 `page.title_taken`，到不了冻结）、23（并行的格共用一个库会互相踩）、24（两处都由工具表的长度驱动）；都对。11 的"总早于"只在 `request_timeout` 不超过 20 秒时成立（P2S2C-N1，决定 273）；26 的根因见 P2S2C-M1；其余没有人提出异议 |
| 留下的事 | — | 实施报告第 9 节：1 Step 的文档提交；2 MCP 的提交到不了 `at + SubmitTimeout`，建议只改设计的说法；3 `TestTheMCPCallStopsWhenTheClientGoes` 仍偶发，建议另做一个小调查；4 `TestARestoreJobSkipsAnItemRestoredBetweenItsBatches` 在负载下失败一次；5 第二份过线，倾向合并之前把 `TestVirtualExport` 挪到第八份；6 第七、八份合并之后看两次 `main`；7 costs-2 的余量；8 S3 照设计 | 1 是本提交，清单由 P2S2C-N2 补全；2 即决定 273；3 C 在审查里查清根因（P2S2C-M1），修复一轮改；4 B 查清是 M8 留下的测试的期望（B 的 4.7），修复一轮改；5 即决定 272，照 P2S2B-Q1 写三个模式；6 照设计第 8 节第 1 条，合并之后看；7 B 估 7763 上作业约 25.4 分钟、末行约第 24.5 分钟开始，卡住时 Go 默认的 10 分钟在第 34.5 分钟，先于 45 分钟，不再提；8 — |
| P2S2B-N2 | 细节（测试） | 修复核对一轮的新发现：`page/adapter/postgres/upgrade_cost_test.go` 的 `TestTheGateReadsReadANodeByItsKey` 要求共用的帮手 `rollbackReadProblems` 对"按外行的笔记本、文件夹读 `nodes`"的两格报不出问题（"want none: the gap gateNodesProblems closes"）。以后若照 P2S2B-M1 的另一种改法收紧帮手（`repeated` 的 `nodes` 只认主键），这个测试会因为帮手变好而失败 | 不改（`fix-2.md`）：它正写明新判法补的是哪个口子；收紧帮手时一并改。记在设计 7.1 |
| P2S2C-M4 | 中（测试） | 修复核对一轮的新发现：f5 照 P2S2C-M2 把三个夹具（`wiki/adapter/http/check_test.go` 的 `aCheck()`、`report_test.go` 的 `reportedView()`、`wiki/adapter/postgres/submission_test.go` 的 `fullReport`）的 `WantedNow` 改成 `{10000, true}`，而它们的 `WantedBefore` 本来就是它，两个成员于是完全相同（封顶时值必是 10,000）；改前三处的 `WantedNow` 是 6、6、3，正是它们分得出这两个成员。C 的变异 s1（存报告时 `WantedBefore` 取 `WantedNow`，`report.go:118`）、s2（REST 投影的 `wanted_before` 取 `WantedNow`，`http/check.go:76`）在 `b6ef8599` 上都活，换回 `c3c35bbc` 的三个测试文件都死：f5 带出的回退。s2 仍由组合根的 `TestUpgradeGates/a page moved into raw` 抓到；s1 别处没有测试抓，存下的报告会把"发起时的待建数"写成"现在的"，那是审阅的人判断闸门 6 的依据之一 | 已改（`9dce4658`，修复第二轮，只改测试）：每层让一个成员封顶、另一个不封顶，两个方向各一个夹具：REST 的 `aCheck()` 前 `{10000, true}`、后 `{6, false}`，`reportedView()` 前 `{5, false}`、后 `{10000, true}`，两个测试各断言两个成员的整个值；存与读的 `fullReport(false)` 前 `{10000, true}`、后 `{7, false}`，另一个方向是 `TestTheReportRoundTrips` 末尾的另一格往返（以 SQL 清掉报告，同一次提交再存一份前 `{5, false}`、后 `{10000, true}` 的）；最坏的报告保留两个都封顶（35,128 字节不变）。s1、s2 改前活、改后死，c1、c11、c12、c13 照旧死。修复核对第二轮：C 关，另加的 wanted 两个方向的五条探针（s3 读报告时 `wanted_before` 取 JSON 的 `wanted_now`，s4 存时 `wanted_now` 取 `WantedBefore`，s5 存时 `wanted_before.count_capped` 写假，s6 REST 的 `wanted_now` 取 `WantedBefore`，s7 REST 的 `wanted_before.count_capped` 恒假）都死；清掉再存走的是 `KeepReport` 的正常路径，没有绕过存储的约束；同一类的口子另有两处，见 P2S2C-M5 |
| P2S2C-N8 | 细节（文档稿） | 修复核对一轮的新发现：文档稿对修复一轮的测试几处说法与代码不符：快照里的核对三格是新的 `TestTheSubmissionsSnapshotIsItsOwn`、期限的 503 的秒数在 `TestTheSubmissionsDeadline`，稿子都写在 `TestSubmitUpgradeAnswersInOrder` 名下；闸门 7 的附件是三格、经 `withAttachment`，稿子写"两格""`passingRead` 带一个基线附件"；B 的新测试 `TestTheGateReadsReadANodeByItsKey` 没有出现；最坏的报告 f5 之后是 35,128 字节，待填项没有列 | 本提交：照代码改 3.10、3.12、3.18、7.1、3.20、10.2 与本记录 |
| 修复一轮的说明 | — | 实施者在修复一轮的报告（`fix-1-report.md` 第 3、9 节）：1 组合根期限格的 `retry_after_seconds` 断言下界、不是一个值：秒数随服务端答复的那一刻差一秒（本机 REST 2 秒、MCP 3 秒），所以断言不小于从收到答复到 `submitted_at + 6 s` 的秒数（向上取整、至少 1），再恰等那么多秒让 dana 接管；精确值由单元钉住；2 A-M3 的格用已有的 `duringRun` 钩子改行，不另加只改快照的钩子：只让快照读到别的行，第二个事务读到的仍是这一次提交，会照"计算出错"退回、记 `failed`；3 B-M1 另加不要数据库的 `TestTheGateReadsReadANodeByItsKey` 与变异 gm7；4 A-M2 多一格"附件保留着，过"；5 第 274 条改了契约的说明（原写 `Retry-After: 1`），f7 的范围因此带 `api`、`web`；6 `stale_index` 那句原来没有测试，在单元的 stale 一格断言整句；7 C-M1 留着第一个等待的 10 秒，2 号提交的说明不改写，"等得不够久"在 f3 的说明里纠正。修复说明第 12 条（清掉的变更集）：不是要改的错误。`DropEmptyChangeset` 只删单元自己事务里既没有条目也没有版本、从没提交过的变更集；`PurgeTrimmedChangesets` 只删 `trimmed`、条目与版本都已清掉的，而条目与版本只在节点被删时设 `deleted_at`、在回收站里过了保留期（`jobs.purge_retention`，默认 60 天、至少 24 小时）才清。所以报告的 `changesets` 少数，当且仅当升级拖过了保留期，且某个变更集的全部内容都属于后来删进回收站、又过了保留期的节点；那时它们在历史里已读不到 | 1、2 A 判对：下界在负载下也不会误报，照它等过一定能接管，取回 1 秒的 r274b 两格都死，偏大的写法由单元钉住；"只有快照不同"在整个程序里到不了。3 B 判足够（见 P2S2B-M1、N2）。4–7 C、A 无异议。第 12 条 B 照代码逐步核过，判对；设计 3.9 照它记一句 |
| 修复第二轮的说明 | — | 实施者在修复第二轮的报告（`fix-2-report.md` 第 2 节）：最坏的报告保留两个都封顶（封顶的写法比不封顶的长 3 字节，那才是最大的形状），另一个方向用 `TestTheReportRoundTrips` 末尾的另一格往返承担，免得"最坏的"不再是最坏的 | 照 `fix-2.md` 允许的取法；修复核对第二轮 C 判对：为了让"最坏的"仍是最大的形状，这样取对 |
| P2S2C-M5 | 中（测试） | 修复核对第二轮的新发现，S2 建成时就有、前两轮修复都没碰：同类的口子还有两处，把一个成员写成另一个，单元照过，组合根与 e2e 也看不到。(a) 存与读：`fullReport` 的 `Sources`、`MovedIntoSources`、`WikiPages`、`NewPages`、`Changesets` 都是 100000（`wiki/adapter/postgres/submission_test.go:137`）；p1（存时 `WikiPages` 取 `NewPages`，`report.go:116`）在单元与组合根的 `TestUpgradeSubmit` 上都活，p2（读时 `Sources` 取 `Changesets`，`:162`）在单元上活。组合根与 e2e 看不到存的那一侧：提交的答复是第二个事务之后从库里重读的（`wiki/app/submit_upgrade.go:271`），组合根比"读回的 = 答复的"，e2e 的 `expectUpgrade` 比"库里的 JSON = 答复的"，两边都经过同一个编码。存下的报告是审阅的人看的东西，与 s1 同一类，落在另外五个成员上。(b) MCP 投影：`TestTheCheckFits` 只断言 `new_pages` 与 `wanted_now.count_capped`（`mcp/upgrade_projection_test.go:96-97`），`bigCheck` 的 `MovedIntoSources` 与 `WantedBefore` 是零值；m1（`wanted_before` 取 `WantedNow`）、m2（`wanted_before.count_capped` 恒假）、m3（`wiki_pages` 取 `Sources`）都活；组合根只为码调 `upgrade_verify`，UP5 的 MCP 版本不读 `counts`。C 更正自己第一轮说的"`bigCheck` 的前是 0，本来就分得出，不用改"：两个成员的值不同，但没有测试读 `wanted_before`。对照：REST 的 r1（`wiki_pages` 取 `Sources`）在单元上活，在组合根的 `TestUpgradeGates` 计数两格上死（整值比五个数），REST 不用改 | 已改（`ef8ac10f`，修复第三轮，只改测试；`fix-3.md` 要一次扫清这一类：检查与报告的计数，判定、REST、存与读、MCP 每一层的每个成员都与别的分得开、都被断言，不只改点名的几处）：判定加"each count its own"一格（七个值互不相同，整值比）；REST 的检查与报告以 `fmt.Sprint` 整值断言 `counts`（报告的 `wanted_before` 5 → 8，原来与 `Changesets` 同值）；`fullReport(false)` 的五个数取 11–15，存下的 JSON 逐个断言；MCP 的 `bigCheck` 七个成员互不相同、整值断言 `counts`，另加前封顶、后不封顶一格；最坏的报告照旧（35,128 字节）；`ReportOf` 是整个结构体赋值，不在成员层，这一轮不动。72 条变异（实施者的对调 55 条与 C 的 17 条）改前活 28 条、改后全死。修复核对第三轮：C 关，实施者的 72 条在 `ef8ac10f` 上整轮重跑全死，换回旧夹具抽 8 条与改前的表相同；r1 现在单元也抓 |
| P2S2C-M6 | 中（测试） | 修复核对第三轮的新发现（第三轮的说明只要求计数，不是它漏做）：计数之外同类的三处，把一个成员写成另一个，单元照过，组合根与 e2e 也看不到。(a) REST 报告的规则的 `count` 与 `count_capped`（`wiki/adapter/http/check.go:86` 的 `reportOf`，S2 新写，不经 lint 的投影）：`reportedView()` 每条规则都是 `{5, false}`、恰有 5 项，`TestTheReportIsAnswered` 不读规则的数；hr1（`CountCapped` 恒真）、hr3（恒假）、hr2（`Count` 取条目的个数）都活，hr1 在组合根的 `TestUpgradeSubmit` 上也活：审阅的人看存下的报告时，每条规则的数与实际不符。(b) `preserved` 闸门的 `nodes` 与 `nodes_kept`：REST 的 hg1（`Nodes` 取 `NodesKept`，`check.go:66`）在单元与组合根的 `TestUpgradeGates` 上都活（单元只断言 `nodes_kept`，组合根读这道闸门的两格 `nodes` 都等于 `nodes_kept`），闸门 7 不过时显示"N 个节点保留了 N 个"；MCP 的 xg1（`NodesKept` 与 `Pages` 对调，`upgrade_projection.go:111`）在单元上活，别处没人读。(c) MCP 投影的闸门的布尔值：`bigCheck` 的五道简单闸门都是假、测试不读；xg2（`frontmatter` 取 `sources`）、xg3（`index` 取 `agents`）在单元上活，xg3 到处都活（推断）。对照：REST 的 hg2、hg3、hg5 与 hg4（即 c2）在组合根上死；存与读的 eg1、eg3 死；判定的 dg1、dg2 死 | 已改（`d4324ab8`，修复第四轮，只改测试；`fix-4.md`：M4、M5、M6 是同一类，不再按成员补，按投影收尾）：S2 新写的七个投影（REST 的检查、规则页、报告与带报告的视图，MCP 的检查，存与读，判定的 `ReportOf`）的夹具每个标量互不相同（`count_capped` 有真有假、与值无关，规则的数不等于条目数），八道闸门与 `passed` 排成四行（`gateRows`：每道闸门那一列的四位互不相同、都不全真全假），各投影四行各过一遍；整个答复与 JSON 字面或写死的结构体比（`sameJSON`、`reflect.DeepEqual`）；MCP 另加 `TestTheCheckIsAnsweredWhole`（能整个放下的 `smallCheck`）；存与读第 2–4 行各存一次再读回。555 条对调与常量的变异（含前几轮的与 C 的 hr1–hr3、hg1–hg5、xg1–xg3、eg1–eg3、dg1、dg2）改前活 217 条、改后全死，没有一条是投影本身写错；最坏的报告不变（35,128 字节）。修复核对第四轮：C 关：hr1–hr3、hg1、xg1–xg3、eg2 在旧夹具上活、在 `d4324ab8` 上死，C 整轮重跑 555 条全死；`gateRows` 九列的四位码互不相同、都不全真全假，任意两列对调或一列取常量都看得出；整值断言是真的整值（`sameJSON` 把两边解成 JSON 值再 `DeepEqual`，多一个键、少一个键都失败）；七个投影里这一类收尾，只剩枚举取夹具里唯一的值（P2S2C-N9）；范围放宽之后另有 P2S2C-M7、N10 |
| 修复第四轮的说明 | — | 实施者在修复第四轮的报告（`fix-4-report.md` 第 2–4、6 节）：1 范围在 `fix-4.md` 点名的之外加了两个：REST 的规则页 `ListUpgradeCheckItems` 与 `upgradeOf` 带报告的一支；不算进来的：`CloneUpgradeCheck`（整个结构体赋值再深拷，形状不变）、MCP `upgrade_submit` 的答复（不是检查或报告，`TestTheUpgradeToolsAnswer` 本来整串比它）、判定 `JudgeUpgrade`（是判定，不是投影；C 的 dg1、dg2 照样放进规格）；2 八个布尔只有两种取值，一个夹具做不到任意两道对调都看得出，所以用四行，注释写明；`gateRows` 是函数，lint 的 `gochecknoglobals` 不许测试里有全局变量（第一次 `make lint-go` 报了三处）；3 报告只在检查通过时留下，REST 的报告与存与读把闸门当任意值答，注释写明，这也钉住了 C 说生产里看不到的 eg2；4 REST 的规则页原来用 invalid_frontmatter 那条规则、挂在 `missing_type` 的路径下，改用 `missing_type`；5 改前那一轮 3 条"每次取第一项"的变异因 `for` 的变量没人用而编译不过，一并改掉循环变量后单独重跑，都活；改后那一轮没有编译不过的 | 修复核对第四轮 C 认可范围与不算进来的三项；规则页改用 `missing_type` 对：原来的条目挂在别的规则的路径下，投影按条目自己的 `rule` 分派所以照过，但用例不会这样答；第 3 条属实（eg2 死在 `TestTheReportRoundTrips`） |
| P2S2C-M7 | 中（测试） | 修复核对第四轮的新发现（C 把范围从七个投影放宽到 S2 写的所有逐字段转换：page 读的四个转换，组合根的 `Roots`、`Baseline`、`Preserved`，存储的提交三列，日志字段）：组合根把 page 模块的 `Preserved` 转成 wiki 的那一行（`server/internal/bootstrap/wiki_upgrade.go:188`）是跨模块的逐字段转换，单元碰不到；wp1（`NodesKept` 写成 `Nodes`）在 `TestUpgradeGates` 上活：组合根读这道闸门的两格 `nodes` 都等于 `nodes_kept`（`upgrade_check_test.go:550` 1/1/1/1，`:630` 2/2/2/1）。两边的单元各自钉住（page 的 `TestTheUpgradeGateReads`，wiki 的 `TestJudgeUpgrade` 的 a1、a2），中间这一跳没有。这样写之后闸门 7 的 `NodesKept == Nodes` 恒真，一个基线附件被删（P2S2A-M2 说的那条绊线）在整个程序里照样判过，答复的 `nodes_kept` 也错。C 的其余探针都死（组合根转换的 wr1、wb1、wb2、wp2，存储的 st1、st2，存与读的 k2、k3）；wp3（`Nodes` 取 `NodesKept`）是等价变异，wr2（`RootPage` 恒真）只遮住一条要绕过守卫才到的缺陷日志，C 判都不算口子 | 已改（`43caa32a`，修复第五轮，只改测试）：`TestUpgradeGates` 加一格"a baseline attachment deleted past the units"：开工前上传两个附件，以 SQL 绕过单元把其中一个的 `deleted_at` 设上；不过的闸门恰是 `preserved`、`passed` 为假，`preserved` 整值比 `{nodes 4, nodes_kept 3, pages 2, pages_kept 2, node_ids [那个附件]}`；断言之后把 `deleted_at` 设回（不还原时每格结尾的 `checkPages` 报全库的不变式不成立，后面的格都跟着失败）。`nodes_kept` 3 与另外三个数都不同；只删一个附件时 `pages` 必等于 `pages_kept`，要它们不同就得再改一页的正文，那一页会进 `node_ids`，所以 `PagesKept` 取 `Pages`（wp2）照旧由"a baseline page's body past the units"一格钉住。wp1 改前活、改后死，实施者另加的三条对调与 `NotKept` 取 nil 都死。修复核对第五轮：C 关：wp1 改后只死在新格，wp2 照旧死，实施者另加的两条对调也死；4/3/2/2 的取法与"`pages` 必等于 `pages_kept`"的理由对（新格与原来那格合起来已把四个数两两分开，不必再造一格）；k1、lg1 改后死，k2、k3 照旧死，所以 N9、N10 也关；还原的写法见 P2S2C-N11 |
| P2S2C-N9 | 细节（测试） | 修复核对第四轮的新发现：REST 报告条目的 `kind`（`wiki/adapter/http/check.go:109`，`reportItem` 的 undigested 一支）写成常量 `asset`，单元照过（k1 活）：`reportedView()` 只有一个 undigested 条目、`kind` 是 `asset`，第四轮的规格对枚举只试了"取另一个值"，没试"取夹具里唯一的那个值"；来源区里的页面在审阅时会显示成附件。存与读一侧有空的 `kind` 作对照，k2、k3 死 | 已改（`43caa32a`）：`reportedView()` 的 undigested 再加一项 `kind: page`，`wantReport` 随之改；k1 改前活、改后死。实施者照修复说明把规格里的枚举都过了一遍：REST 规则页的 `mode` 夹具里只有 `llm_wiki`，同一页以 `standard` 再答一次、整值比（`hi-MD=llm` 改前活、改后死；升级的检查在用的时候总是 llm_wiki，看不出区别，但投影本身是把结果的 `mode` 转过去，照样钉住）；规则名与存与读的 `kind` 本来两种都有，MCP 检查的 `mode` 是常量、MCP 条目的 kind 是 lint 的投影，不用补 |
| P2S2C-N10 | 细节（测试；日志） | 修复核对第四轮的新发现：提交的日志行（`wiki/app/submit_upgrade.go:126`）的 `user_id` 没有断言，lg1（取工作区的 id）在 `wiki/app` 上活；单元断言了 `upgrade_id`、`submission_id`、`notebook_id`、`workspace_id`（`submit_upgrade_test.go:590-593`），组合根只看 `upgrade_id`、`submission_id`、`outcome`、`passed`。是日志字段，不是检查或报告的投影 | 已改（`43caa32a`；主持者：Nit 改起来便宜，与 M7 放在同一个提交）：`TestTheSubmitterIsDecidedBeforeTheAnswer` 那一句以 `shared.RequireActor(ctx)` 取回操作人，补上 `user_id`；lg1 改前活、改后死 |
| 修复第五轮的说明 | — | 实施者在修复第五轮的报告（`fix-5-report.md` 第 2–4 节）：1 M7 那一格的四个数不能互不相同（见 P2S2C-M7）；2 C 的 wp3 与同理的 `Pages` 取 `PagesKept` 是等价变异，没放进规格；3 第四轮的 555 条在新头上整轮重跑，全死；4 提交之前直接跑过一次 `go test ./internal/modules/wiki/...`（postgres 包起测试容器几秒），没有经 `heavy.sh`，之后同一套都经 `heavy.sh` 重跑过 | 修复核对第五轮 C 判 1 对；2 C 本来判 wp3 等价，答复的 `nodes`、`pages` 取自升级的基线行；规则页以 `standard` 再答一次合理：投影是把结果的 `mode` 转过去，替换没生效测试就红，不会空过；C 没有再整轮重跑 555 条（这一轮只加了测试） |
| P2S2C-N11 | 细节（测试） | 修复核对第五轮的新发现：`bootstrap/upgrade_check_test.go` 的 `TestUpgradeGates` 以 SQL 改库的两格，修复第五轮的"a baseline attachment deleted past the units"（`:661` 设回 `deleted_at`）与 S2 建成时就有的"a stale index"（`:675` 加回 extractor），还原都写在 `check` 里；`tm.checked` 遇到非 200 或形状不对先 `t.Fatalf`（`:89-101`）时 `check` 不跑、还原被跳过，库里留下"删了却没有回收站条目"的节点或旧的索引，之后每一格的 `checkPages` 或 `checkLinks` 都跟着红，真正的原因被埋在一串失败里。只在那一格已经红时影响诊断，不影响正确性 | 不改（主持者：只在那一格已经红时影响诊断，两格同一写法，为它再加一个提交与一轮 CI 不值）；以后改这个测试时，两格的 `change` 里另用 `t.Cleanup` 注册同一句幂等的还原（`check` 里那句留着：`t.Cleanup` 在同一格的 `checkPages` 之后才跑）。记在设计 7.1 |

## 主持者的决定

负责人可以改判（272 在 S2 的实施之后、审查之前定下，审查之后照 P2S2B-Q1 细化；273–275 在第一轮的修复说明 `fix-1.md` 里定下；负责人 2026-11-06 的话照旧："调测试用例量级或者时限即可"）：

272. **第二份在合并之前挪 `TestVirtualExport` 到第八份**（S2 的分支 CI；P2S2B-Q1，2026-11-11）：分支 CI（42882105107）第二份在 EPYC 7763 上 Test 一步 457 秒，过了 450 秒的线（`main` 上 438、430 秒）；本机 S2 让第二份慢约 5%（`ba72cda4` 上 250.8、252.7 秒，`c3c35bbc` 上 263.2、264.4 秒），B 查出是契约变大、组合根每次 `apitest.Load` 多约 62 ms，不是产品的热路径（P2S2B-Q2）；第八份在 CI 上 97 秒。照 B 写三个模式，第八份仍是第七份的子集：
     - `BOOTSTRAP_SECOND = ^Test(E|Source|The(MCP[C-Z]|Contents))`
     - `BOOTSTRAP_SEVENTH = ^Test(Upgrade|IngestBefore$$|ABackground|VirtualExport$$)`
     - `BOOTSTRAP_EIGHTH = ^Test(Upgrade(Check|Gates|Submit|Tools)|VirtualExport$$)`

     只改 `SECOND` 与 `EIGHTH` 时它在第三份与第八份各跑一次。B 估挪后第二份在 7763 上约 374 秒，第八份约 190 秒。理由：一次分支的 457 秒加本机 5% 的差，合并之后两次 `main` 的平均几乎一定过线，第八份正空着；06-environment 第 50 行与设计第 8 节第 1 条看的是合并之后两次 `main` 的平均，提前挪是主持者的取舍，挪了要在头上重跑门禁与 CI。
273. **MCP 的提交的期限只改设计的说法，不改代码**（P2S2A-M4、P2S2C-N1，2026-11-11）：`request_timeout ≤ 20 s`（默认配置在内）时两个入口都是请求的期限先到；`> 20 s` 时是 `submitted_at + SubmitTimeout`（`SubmitTimeout` 那时恒为 20 秒）。00 号文档 4.8 的两层意图照样成立，第二段没有变短；设计的改法在本提交。
274. **提交的期限答 503 时，`Retry-After` 取到能接管为止的秒数**（P2S2A-N1、P2S2A-Q1，2026-11-11）：`max(1 s, ceil(first.at + SubmitTimeout − now))`，只改 `wiki/app/submit_upgrade.go:130` 那一处，lint 的名额满时的 503 不动。依据：默认配置下 503 比能接管早约 2.5 秒，隔 1 秒重试会落进"正在计算"的 409。测试：单元钉住秒数；组合根期限一格里 `retry_after_seconds` 照新值断言；一个变异取回 1 秒；REST 与 MCP 两个入口都要看到。
275. **costs-3 的作业时限从 35 分钟提到 40 分钟**（P2S2B-M2，2026-11-11）：注释写 run 号、runner 与 `-timeout 20m` 那一行开始的分钟数，写法同 `80da6b1f`（第 269 条）；不挪测试、不改界；范围 `ci`。

修复一轮的处置照 `fix-1.md`：在 `c3c35bbc` 之上加新提交，不改写已推的提交，一类事一个提交、各自绿，第 272 条的挪份放第一个；逐个提交照 S2 的 `verify.sh`，改了工具表或契约的另跑会读它们的测试（S2 第一轮门禁漏过 `TestToolStrings*` 与 LW8）；审查者活下来的 a1、a2、a4、a6、c1、c11、c12、c13、gm3 各重跑一次，要都死，第 274 条与 P2S2A-N2 各一个新变异；头上跑全部门禁、第二份与第八份本机各两轮、`go test -race -list` 的八份归类，每一项重的之前判 df；推分支之后看第二份、第八份与 costs-3 的作业时间。只改文档的（P2S2A-M4 与 P2S2C-N1 的设计说法，P2S2A-N3，P2S2C-N2、N3，第 5 节里 P2S2C-M3 的那一句）在本提交；P2S2C-N7 不改；P2S2B-Q2 交给 P3。第二轮照 `fix-2.md`：修复核对一轮的 P2S2C-M4 接受，一个只改测试的提交、接在 `b6ef8599` 之后，s1、s2 要死、c1、c11–c13 照旧死；只改了单元测试的夹具，门禁只跑 `make test-go-short` 与 wiki 模块全部竞态单元；P2S2B-N2 不改，收紧帮手时一并改；P2S2C-N8 是文档的，在本提交。第三轮照 `fix-3.md`：修复核对第二轮的 P2S2C-M5 接受，并要一次扫清这一类（检查与报告的计数，判定、REST、存与读、MCP 每一层的每个成员都分得开、都被断言），不只改 C 点名的几处；自己写对调与常量的变异，先在 `9dce4658` 上跑。第四轮照 `fix-4.md`：修复核对第三轮的 P2S2C-M6 接受；M4、M5、M6 是同一类（夹具里有重复的值或只断言部分成员，投影或编码把一个成员写成另一个、测试照过），前两轮按点名的成员补、每轮核对又找到下一处，这一轮按投影收尾：S2 新写的把检查或报告变成另一种形状的函数，每一个都要夹具里每个标量互不相同、整值断言、对调与常量的变异全死；只改测试，发现投影本身有错就停下（没有发现）。这一条写进 05-lessons。修复核对第五轮只余一条 Nit（P2S2C-N11），不改，记下以后改这个测试时的写法。第二至四轮都只改单元测试，门禁只跑 build、vet、`make lint-go`、`make gen-check`、`make test-go-short` 与 wiki 模块全部竞态单元。第五轮照 `fix-5.md`：修复核对第四轮的 P2S2C-M7、N9、N10 都在这一轮改，Nit 改起来便宜，与 M7 放在同一个提交；N9 顺手查规格里其余的枚举是否夹具里只有一个值；只改测试，发现转换本身有错就停下（没有发现）；M7 那一格在第八份，另经 `heavy.sh` 跑 `TestUpgradeGates` 与第八份一轮。05-lessons 那一条补一句：投影之外，跨模块的逐字段转换（组合根）同样要有一格让每个成员取不同的值。

主持者另定（不编号）：修复一轮的 CI 第一次 attempt 红在 e2e 的 L1（带修饰键点链接、等新标签页 30 秒超时，已知的旧偶发，这是第四次记下），只重跑 e2e 之后全绿；L1 交给 P2/S3（网页）的实施者另查，S2 不查。

## 接受与推后的

- **接受**：
  - A 的过度设计两点，不建议删：提交在快照里的核对里 `SubmissionID == submission` 与 `Report == nil` 两项实际上到不了（接管只在期限之后，flight 的快照取在它自己的 `LintTimeout` 之内；报告只由这一次的第二个事务写），起作用的只有状态那一项，两项便宜，留着兜底；闸门 7 的 `Pages == BaselinePages` 在 `Nodes == BaselineNodes` 之下冗余（a3），照设计写出来更好读。
  - A 的 a5（检查在第二次判定之后不看 `ctx.Err()`）活下来：无害，客户端已走，答给没有人，同 `Lint.Execute`。
  - C 的 c2（REST 投影把两道闸门对调）在单元里活、在组合根的 `TestUpgradeGates` 里被抓到（c2b）：整个程序按键断言不过的闸门，够了。
  - A 的未核实的怀疑：多实例时，`submitted_at` 用写它的那台的墙钟、接管用另一台的墙钟比，差几秒时"正在计算"的边界挪了，安全仍由笔记本行的锁与 `submission_id` 的核对守着，没有查 v0.1 是否支持多实例；`COMMIT` 在 `commitTimeout` 内没有返回时答 503 而行可能已存下报告，设计 3.12 第 6 步已写明接受。
  - B 的未核实的怀疑第 2 条（`PurgeTrimmedChangesets` 或 `DropEmptyChangeset` 会不会删掉一次进行中的升级的变更集，使报告的 `changesets` 少数；它只是报告里的一个数、不是闸门）：修复说明第 12 条交实施者查代码，结论是不改（"修复一轮的说明"），B 判对；设计 3.9 记一句"数的是这次升级写下、历史里还留着的变更集"。
  - C 的 eg2（存报告时 `frontmatter` 取 `sources`，修复核对第三轮）：只有通过的检查才存报告（`KeepReport` 只在 `Passed` 时调），存下的报告每道闸门都是真，这种对调在生产里看不到；修复第四轮的往返把闸门当任意值存，仍钉住了它。
  - C 的 wb3（修复核对第五轮：组合根 `Baseline` 的 `kind` 恒为 page）是等价变异：基线的 `kind` 只在闸门 6 用（`upgrade_check.go:236`），`WantedByPage` 只含页，附件的 id 进不去。
  - C 的 wp3、wr2（修复核对第四轮）：wp3（组合根转换的 `Nodes` 取 `NodesKept`）是等价变异，闸门答出的 `nodes` 取自升级的基线，`passed` 照样为假；wr2（组合根转换的 `RootPage` 恒真）只遮住一条"来源根不是活页"的缺陷日志，那条路径要绕过守卫才到得了，wiki 的单元以假端口钉住了它。都不算口子。
  - C 的未核实的怀疑第 2 条：`upgrade_submit` 笔记本入口的 `inputSchema` 没有 `properties`，个别 MCP 客户端可能不接受；账户入口的 `notebooks` 已是同样的写法，M9 的人工清单没有报过，留给 P3/S2 的 K3（设计第 9 节）。
  - 流程（照实写下）：实施者推送之前改写过两次 7–10 号，都是在 7 号或 8 号上 `git commit --amend` 之后逐个 `git cherry-pick` 其后的提交，不用交互的 rebase：第一次补变异 o1 抓不到的两格（7 号 `e3803c20` → `8d013ae5`，头 `624bb617` → `c3bfdd54`）；第二次修第一轮门禁的三处失败（7 号 → `74a3057a`，8 号 → `d8d2bbaf`，头 → `c3c35bbc`）。旧的都没推过，分支的推送是普通的 push。实施者自己记下的疏漏：逐个提交的核对只跑每个提交"自己的"测试，8 号改了 MCP 的工具表，却没跑第三份的 `TestToolStrings*` 与 e2e 的 LW8，到门禁才红。
- **推后**：
  - 契约每长一截组合根各份都慢一截（P2S2B-Q2）：交给 P3 定分份时算进去（设计第 9 节）。
  - P3/S2 的 playbook：提交答 `server_busy` 时照 `retry_after_seconds` 等过再交（第 274 条之后不必另写"等几秒"）；K3 记 `upgrade_submit` 的 `inputSchema` 在客户端里是否被接受（设计第 9 节）。
  - guards-sources 的时限：S2 没有改到这一份（`Makefile` 与 `ci.yml` 都没动它），照旧等下次改到时照第 269 条提到 40 分钟（B：这次在 9V74 上 16:38，`main` 上 7763 一次 998 秒）。
  - 检查的十万页照第 262 条在 M12 的性能移交，不变。
  - P2S2C-N11：以后改 `TestUpgradeGates` 时，以 SQL 改库的两格的 `change` 里另用 `t.Cleanup` 注册同一句幂等的还原（设计 7.1）。
  - P2S2B-N2：收紧共用的帮手 `rollbackReadProblems` 时一并改 `TestTheGateReadsReadANodeByItsKey`（设计 7.1）。
  - e2e 的 L1 的修饰键新标签页偶发（修复一轮的 CI 第一次 attempt；M10/P3/S1、M10/P4/S1、M11/P1/S3 的记录里同一行、同一症状）：交给 P2/S3 另查。

## 偶发

- **`TestTheMCPCallStopsWhenTheClientGoes`（第五份）**：S1 的审查记录把它推后到 S2 的一个测试提交，理由是本机负载下第三次失败、像是等得不够久。2 号提交（`967fdbea`）把两个等待推到 10 秒；第一轮 `make test` 里它又失败一次（11.64 秒，"the log has map[], want the answer never written at debug"），10 秒里一行也没有。C 查清根因（P2S2C-M1）：失败的每一次都恰是 SDK 在客户端走后把答复写进了已断的连接，"答复没写出"那一行根本不会来；平台两种结局都处理得对，测试只认其中一种，负载越重越常失败，等多久也没有用。M9/P1 在单元里已照两种结局改过，组合根的这一个没有跟着改。修复一轮删掉那一条断言（`701c736d`），设计 3.19 交下的那一条关上；修复核对一轮 C 在同样的负载下 40 次全过。C 排除了的：调用没被取消（没有期限的 warn、1 秒之内结束）、`ServeHTTP` 当它是没写的缺陷（会记 ERROR）、`wrote` 的数据竞争（写与读都在 `s.mu` 之下，有先后）。
- **`TestARestoreJobSkipsAnItemRestoredBetweenItsBatches`（第一份）**：第一轮 `make test` 的负载下失败一次，单独重跑与第二轮都过。B 查清是 M8 留下的测试的期望（B 的 4.7）：负载下作业抢先拿到笔记本行的锁，请求的恢复答的是它说明里的另一个 409（`page.trash_changed`），测试只认 `page.not_in_trash`；产品没有写错东西。同一写法的 `TestARevertJobSkipsAMemberRestoredBetweenItsBatches` 一并改（`e88cf289`）；两轮修复的门禁里它都没有再出。
- **e2e 的 L1**（page，chromium，`l1-links.spec.ts:178`）：修复一轮的 CI（42913206700）第一次 attempt 里，带修饰键点链接、等新标签页 `browserContext.waitForEvent("page")` 30 秒超时，其余 521 个通过；这一轮没有碰网页的代码，本机门禁的 `make e2e` 522 个都过。这是已知的旧偶发，同一行、同一症状见 M10/P3/S1、M10/P4/S1 与 M11/P1/S3 的记录，底层原因仍没查明。照 S1 修复第二轮的先例只重跑红的那一项（`gh run rerun --failed`），第二次 attempt e2e 绿；主持者定交给 P2/S3 另查。

## 反向对照

建成时以 `mut.py` 跑（`NW_SERVER` 指实施者的工作树的 `server`；规格由 `m11/p2s2/mut/mkspec.py` 生成 74 条：7.3 的 S2 各行（25–66）42 行 59 条，设计列几个测试的每个测试各一条，记作"a/b/c"；加实施者自己的 10 种 o1、o3–o11，15 条（o2"两处的同一判断"与 45、56 重了，没有另写）；`mut/run.sh` 先在没改的树上把 43 条不同的命令各跑一次，都 rc=0、都跑到了测试，没有"no tests to run"；整轮经 `heavy.sh`；改坏 sqlc 生成的 Go 常量（35、36、37、o6）时不跑 `make gen*`；组合根的格以 `-run '^TestX$/^<格>'` 单独跑，输出里点名到格；每轮之后 `git status` 干净，没有 `*.mutbak`）。第一轮在 `624bb617` 上 69 条抓到，5 条没抓到：25b、42b 是设计 7.3 把别的条的测试归到这两行（第 25 行的"7.2 第 6 条"是第 26 行的：提交的键的 `Kind` 与检查的不同，去掉 `Scope` 也不并入；第 42 行的提交答 `stale_index` 看的是检查的 `StalePages`，不经闸门 8），由 25a、42a 抓到；33 对 `TestJudgeUpgrade` 没抓到，因为计数的格拆进了 `TestJudgeUpgradesCounts`，另跑抓到；o1a、o1b（提交只看 `frontmatter` 一道闸门）是真的缺口，7 号补"只有 `AGENTS` 不过"的单元一格与组合根一格之后都抓到。第一轮门禁之后改了交错第 7 条的断言，48、58 在它上面重跑（头 `c3c35bbc`），都抓到；另两处（`validToolArguments`、LW8 的工具数）不在任何变异点名的测试里，没有重跑。

**7.3 的 S2 各行与实施者自己的**：

| # | 变异（怎样改坏） | 跑的测试 | 结果 |
|---|---|---|---|
| 25a | `Do` 里 `key.Scope = uuid.UUID{}` | `TestTheUpgradeKeysJoinOnlyTheirOwn` | 抓到 |
| 25b | 同上 | `TestUpgradeSubmitInterleavings/6` | 没抓到（见上） |
| 26 | 提交的键改成 `upgradeKey(up, LintUpgradeCheck, up.ID)` | `TestUpgradeSubmitInterleavings/6` | 抓到 |
| 27 | `cloneRunResult` 交同一个 `Check` | `TestTheRunsCopyACheck` | 抓到 |
| 28 | `logFinished` 删掉三种升级的 `case` | `TestTheUpgradeKindsAreNamed`（三格） | 抓到 |
| 29a | 摘要的计划 `area: q.WantedByPage` | `TestTheWantedCountsByPage/the-summary-without-the-second-pass` | 抓到 |
| 29b | 同上 | `TestUpgradeGates/a page moved into raw` | 抓到 |
| 30 | 发起的 `RuleQuery` 去掉 `AreaWantedByPage` | `TestUpgradeStart/a raw page is the source root` | 抓到 |
| 31a | 闸门 6 先算全本的净数，净数不增就不算变坏 | `TestJudgeUpgrade`（"one page worse, another better"等两格） | 抓到 |
| 31b | 同上 | `TestUpgradeGates/two pages offset` | 抓到 |
| 32 | 闸门 6 算没有基线行的新页 | `TestJudgeUpgrade` | 抓到 |
| 33 | `MovedIntoSources` 数现在在区里的每个基线节点 | `TestJudgeUpgrade`；`TestJudgeUpgradesCounts` | 前者没抓到，后者抓到（见上） |
| 34 | 闸门 7 的分母与 `passed` 取现读的行数 | `TestJudgeUpgrade/a baseline row fewer than the upgrade's counts` | 抓到 |
| 35a | `UpgradePreserved` 不比账本（`pages_kept` 与 `not_kept` 只看正文在不在） | `TestUpgradeGates/a baseline page's body past the units` | 抓到 |
| 35b | 同上 | `TestTheUpgradeGateReads` | 抓到 |
| 36a | `not_kept` 只收不在的节点 | `TestTheUpgradeGateReads` | 抓到 |
| 36b | 同上 | `TestUpgradeGates/a baseline page's body past the units` | 抓到 |
| 37 | `UpgradeChangesets` 带回 `AND deleted_at IS NULL` | `TestTheUpgradeGateReadsPlansAtAnyStatistics`（从未分析一格的计划） | 抓到 |
| 38 | `rollbackReadProblems` 不判 `changesets` | `TestTheRollbackGuardJudgesThePlans`（BitmapAnd 与经笔记本的索引两格） | 抓到 |
| 39 | 闸门 2 不查来源根是不是 `AGENTS` | `TestJudgeUpgrade/the source root renamed AGENTS` | 抓到 |
| 40 | 闸门 3 不限根下 | `TestJudgeUpgrade`（两格） | 抓到 |
| 41 | `Roots` 不切 frontmatter | `TestTheUpgradeRoots`（只有 frontmatter；只有 BOM 与空白） | 抓到 |
| 42a | `g.Index = true` | `TestJudgeUpgrade/a stale page` | 抓到 |
| 42b | 同上 | `TestUpgradeSubmit/the codes in their order` | 没抓到（见上） |
| 43a | 快照里只看有没有升级，不核对 id 与状态 | `TestUpgradeCheck/8` | 抓到 |
| 43b | 同上 | `TestTheCheckRefusesAnotherUpgradesSnapshot` | 抓到 |
| 44 | `Preserved` 在快照之外读 | `TestTheCheckReadsInOneSnapshot` | 抓到 |
| 45 | 检查交付之前不再判 | `TestTheCheckAuthorizesBeforeItsAnswer` | 抓到 |
| 46 | lint 游标的绑定不比范围（`scope != c.scope` → 恒假）（第 7 节第 22 条） | `TestUpgradeCheck`（码依次与游标两格） | 抓到 |
| 47 | 第二个事务不核 `submission_id` | `TestTheSecondTransactionWritesOnlyItsOwn/another submission` | 抓到 |
| 48 | 第二个事务不核状态 | `TestUpgradeSubmitInterleavings/7` | 抓到（第 7 条的断言改过之后重跑，仍抓到） |
| 49a | 第二段用请求的 ctx | `TestUpgradeSubmit/the client gone` | 抓到 |
| 49b | 同上 | `TestTheSubmissionDetaches` | 抓到 |
| 50 | 期限从第一个事务之后的时钟算 | `TestTheSubmissionDetaches/the request's later` | 抓到 |
| 51a | 期限不取 min | `TestTheSubmissionDetaches/the request's earlier` | 抓到 |
| 51b | 同上 | `TestUpgradeSubmit/the deadline, REST` | 抓到 |
| 52a | `at` 不截到微秒 | `TestTheSubmissionDetaches` | 抓到 |
| 52b | 同上 | `TestSubmitUpgradeAnswersInOrder` | 抓到 |
| 53a | 第 6 步还要求请求的 ctx 活着 | `TestTheSubmissionsDeadline/the request expired` | 抓到 |
| 53b | 同上 | `TestUpgradeSubmit/the deadline, REST` | 抓到 |
| 54a | 第二个事务的期限错误原样交出（500） | `TestTheSubmissionsDeadline` | 抓到 |
| 54b | 同上 | `TestUpgradeSubmit/the deadline`（REST 与 MCP 两格） | 抓到 |
| 55 | 判定挪到 `COMMIT` 之后、以 `detached` 判（只对存下的） | `TestTheSubmissionsDeadline/a COMMIT past the deadline, a report kept` | 抓到 |
| 56a | 提交的交付之前不判 | `TestTheSubmitterIsDecidedBeforeTheAnswer` | 抓到 |
| 56b | 同上 | `TestUpgradeSubmitInterleavings/the submitter demoted` | 抓到 |
| 57a | 判定拒了存下的那一次时不记 info | `TestTheSubmitterIsDecidedBeforeTheAnswer` | 抓到 |
| 57b | 同上 | `TestUpgradeSubmitInterleavings/the submitter demoted` | 抓到 |
| 58 | info 记在第二个事务之前（`outcome` 写 `stored`），原来那一行去掉 | `TestUpgradeSubmitInterleavings/7` | 抓到（同 48） |
| 59a | 去掉"正在计算"（期限之内的再提交顶掉） | `TestUpgradeSubmitInterleavings/15` | 抓到 |
| 59b | 同上 | `TestUpgradeSubmit/the deadline, REST` | 抓到 |
| 59c | 同上 | `TestSubmitUpgradeAnswersInOrder`（两格） | 抓到 |
| 60 | `not_ready` 不退回（不 `SetState`、不 `Unsubmit`） | `TestUpgradeSubmit/the codes in their order` | 抓到 |
| 61 | 第二个事务存下报告时不发 `mode` | `TestUpgradeStatesReachTheStreams` | 抓到 |
| 62 | `ReportOf` 不去游标 | `TestReportOf` | 抓到 |
| 63a | `destructiveHint` 照旧只在真时写 | `TestTheAnnotationsAreTheHints` | 抓到 |
| 63b | 同上 | `TestTheToolListsAreTheContract` | 抓到 |
| 64 | 内置说明少 `upgrade_submit` 一行 | `TestTheBuiltInInstructionsNameEveryTool` | 抓到 |
| 65 | 第二次 `Fit` 不截闸门的路径 | `TestTheCheckFits` | 抓到 |
| 66 | `coveredMCPRows()` 不加升级的两行 | `TestThePermissionMatrixCoversEveryTool` | 抓到 |
| o1a | 提交只看 `frontmatter` | `TestSubmitUpgradeAnswersInOrder` | 第一轮没抓到；补一格之后抓到 |
| o1b | 同上 | `TestUpgradeSubmit/the codes in their order` | 第一轮没抓到；补一格之后抓到 |
| o3a | `upgrade_verify` 不传游标 | `TestUpgradeCheck/the codes in their order`（MCP 一半） | 抓到 |
| o3b | 同上 | `TestUpgradeToolsErrorsCarryTheirCodes` | 抓到 |
| o4 | 读报告丢掉 `links.pages_worse` | `TestTheReportRoundTrips` | 抓到 |
| o5 | 组合根的 `SetState` 不认 `upgrading` | `TestUpgradeSubmit/the codes in their order` | 抓到 |
| o6a | `UnsubmitUpgrade` 不清 `submitted_at` | `TestTheSubmissionsRows` | 抓到 |
| o6b | 同上 | `TestUpgradeSubmit/the codes in their order` | 抓到 |
| o7 | 接管的界从 `>=` 改成 `>` | `TestSubmitUpgradeAnswersInOrder/… SubmitTimeout ago: taken over` | 抓到 |
| o8 | MCP 的闸门路径不给活树之外的节点占位 `null` | `TestTheCheckFits` | 抓到 |
| o9 | `upgradeToolCodes()` 不收两行 | 第五份的 `TestTheMCPToolsErrorsCarryTheirCodes` | 抓到 |
| o10a | 不带 `rule` 的 `limit` 不拒 | `TestCheckUpgradeAnswersInOrder/a limit without a rule` | 抓到 |
| o10b | 同上 | `TestUpgradeCheck/the codes in their order`（MCP 一半） | 抓到 |
| o10c | 同上 | `TestUpgradeToolsErrorsCarryTheirCodes` | 抓到 |
| o11 | `TestUpgradeToolsMatrix` 不 `prepare` | `TestUpgradeToolsMatrix`（管理员的格） | 抓到 |

**审查者的**（审查时在 `c3c35bbc` 上；每条之前在没改的树上跑过同样的命令；中间输出在 `p2s2/review/a/`、`b/`、`c/`）：

| 审查者 | 变异 | 测试 | 结果 |
|---|---|---|---|
| A | a1 闸门 7 去掉 `p.NodesKept == p.Nodes &&` | `wiki/domain`、`wiki/app` 全部；`TestUpgradeGates` | 都活（P2S2A-M2） |
| A | a2 闸门 7 去掉 `p.Nodes == r.Upgrade.BaselineNodes &&` | 同上 | 都活（P2S2A-M2） |
| A | a3 闸门 7 去掉 `p.Pages == r.Upgrade.BaselinePages &&` | `wiki/domain`、`wiki/app` 全部 | 活（冗余，"接受"） |
| A | a4 提交的快照核对只比 `now.ID == up.ID` | `wiki/app` 全部；`TestUpgradeSubmitInterleavings` | 都活（P2S2A-M3） |
| A | a5 检查在第二次判定之后不看 `ctx.Err()` | `wiki/app` 全部 | 活（无害，"接受"） |
| A | a6 `case !check.Passed` → `!check.Gates.Frontmatter \|\| !check.Gates.Agents` | `wiki/app` 全部；`TestUpgradeSubmit/the codes in their order` | 都活（P2S2A-M1） |
| A | o1（实施者的，复跑） | `wiki/app` 全部 | 抓到（`TestSubmitUpgradeAnswersInOrder/AGENTS_alone_not_passed`） |
| B | gm1 `UpgradeGateBaseline` 按 `notebook_id` 读 | `TestTheUpgradeGateReadsPlansAtAnyStatistics` | 抓到（从未分析：`upgrade_nodes read by upgrade_nodes_notebook_id_idx`） |
| B | gm2 `UpgradePreserved` 的两个 `LATERAL` 去掉 `LIMIT 1` | 同上 | 抓到（拉成连接、`nodes` 整读） |
| B | gm3 `UpgradePreserved` 的 `nodes` 按 `n.notebook_id = b.notebook_id AND n.id::text = b.node_id::text` 读 | 同上 | 失败，但不是计划判出：六个计划都过，进计时后在 4 分钟超时 panic（P2S2B-M1） |
| B | gm4 `UpgradeChangesets` 加 `AND deleted_at IS NULL` | 同上 | 抓到（从未分析：`BitmapAnd` 里没有条件的部分索引） |
| B | gm5 `UpgradeChangesets` 按笔记本读、`upgrade_id::text` 比 | 同上 | 抓到 |
| B | gm6 `page_contents` 按 `c.node_id::text = b.node_id::text` 读 | 同上 | 抓到（顺序扫描） |
| B | cm3 同 gm3，对组合根的检查守卫 | `TestUpgradeCheckTakesItsTimeAtAnyStatistics` | 抓到：第一份种子第一次，worker 3 s 到期答 `server_busy` |
| C | c1 REST 投影 `wanted_now.count_capped` 恒为 `false` | `wiki/adapter/http` 全包 | 活（P2S2C-M2） |
| C | c2 REST 投影把 `frontmatter` 与 `sources` 两道闸门对调 | `wiki/adapter/http` 全包 | 活（单元只核 `agents`） |
| C | c2b 同上 | `TestUpgradeGates/a page without a summary` | 抓到 |
| C | c3 读报告丢掉规则的 `count_capped` | `TestTheReportRoundTrips`、`TestTheWorstReportFits` | 抓到 |
| C | c4 存报告丢掉条目的 `target` | 同上 | 抓到 |
| C | c5 读报告把 `nodes_kept` 读成 `nodes` | 同上 | 抓到 |
| C | c6 MCP 的 `preserved` 用保留标题的节点的路径 | `wiki/adapter/mcp` 全包 | 抓到（`TestTheCheckFits`） |
| C | c7 `upgrade_submit` 标成 `Destructive: true` | 同上 | 抓到（`TestLintToolObjects` 两个入口） |
| C | c8 `ReportOf` 每条规则留六项 | `TestReportOf` | 抓到 |
| C | c9 `upgrade_submit` 的答复丢掉 `submitted_at` | `wiki/adapter/mcp` 全包 | 抓到（`TestTheUpgradeToolsAnswer`） |
| C | c10 条目的答复的 `mode` 恒为 `standard` | `wiki/adapter/http` 全包 | 抓到（`TestTheCheckIsAnswered`） |
| C | c11 判定的 `WantedNow` 去掉封顶 | `wiki/domain`、`wiki/app` | 活（P2S2C-M2） |
| C | c12 存报告时 `wanted_now.count_capped` 写 `false` | `wiki/adapter/postgres` 全包 | 活（P2S2C-M2） |
| C | c13 MCP 投影 `wanted_now.count_capped` 恒为 `false` | `wiki/adapter/mcp` 全包 | 活（P2S2C-M2） |

**修复一轮**（规格 `fix1/mut/spec.json` 13 条，从三位审查者的规格取出改动、命令照这一轮的测试写；整轮经 `heavy.sh`，先在没改的树 `b6ef8599` 上把 9 条不同的命令各跑一次，都 rc=0、都跑到了测试；改坏生成的 SQL 常量时不跑 `make gen*`；"改前"是审查者在 `c3c35bbc` 上的结果）：13 条都死。

| # | 变异 | 跑的测试 | 改前 | 改后（`b6ef8599`） |
|---|---|---|---|---|
| a1 | 闸门 7 去掉 `NodesKept == Nodes` | `wiki/domain`、`wiki/app` | 活（A） | 死：`TestJudgeUpgrade/a baseline attachment deleted` |
| a2 | 闸门 7 去掉 `Nodes == BaselineNodes` | 同上 | 活（A） | 死：`…/an attachment's baseline row gone` |
| a4 | 提交的快照只比升级的 id | `wiki/app` | 活（A） | 死：`TestTheSubmissionsSnapshotIsItsOwn` 三格 |
| a6 | 提交只看 `frontmatter` 与 `agents` | `wiki/app` | 活（A） | 死：`TestSubmitUpgradeAnswersInOrder` 的 `source_root`、`reserved_titles`、`sources`、`links`、`preserved` 各格 |
| c1 | REST 投影 `wanted_now.count_capped` 恒假 | `wiki/adapter/http` | 活（C） | 死：`TestTheCheckIsAnswered`、`TestTheReportIsAnswered` |
| c11 | 判定的 `WantedNow` 去掉封顶 | `wiki/domain`、`wiki/app` | 活（C） | 死：`TestJudgeUpgradesCounts/the wanted pages now capped` |
| c12 | 存报告时 `wanted_now.count_capped` 写假 | `wiki/adapter/postgres` | 活（C） | 死：`TestTheReportRoundTrips`、`TestTheWorstReportFits` |
| c13 | MCP 投影 `wanted_now.count_capped` 恒假 | `wiki/adapter/mcp` | 活（C） | 死：`TestTheCheckFits` |
| gm3 | `UpgradePreserved` 的 `nodes` 按 `n.notebook_id = b.notebook_id` 读 | `TestTheUpgradeGateReadsPlansAtAnyStatistics` | 计划放过、计时超时（B） | 死：14.2 秒，由计划判出 |
| gm7 | 同上，另按 `n.parent_id = b.parent_id`（读一个文件夹） | 同上 | （B 没跑） | 死：14.3 秒，由计划判出 |
| r274a | 提交的期限一支取回 `Retry-After: 1` | `TestTheSubmissionsDeadline` | — | 死：`the request expired`、`…7.5 s in` |
| r274b | 同上 | 组合根 `TestUpgradeSubmit/^the_deadline`（竞态） | — | 死：REST、MCP 两格 |
| n9 | `lintQuery` 去掉 `\|\| q.Rule == nil` | `TestTheWantedCountsByPage` | — | 死：`the-second-pass-alone` |

**修复核对一轮的**（各在自己的工作树、头 `b6ef8599` 上）：

| 审查者 | 变异 | 测试 | 结果 |
|---|---|---|---|
| A | a1、a2、a4、a6、n9、r274-one-second（同实施者的） | `wiki/domain`、`wiki/app`、`linking/app` | 都死 |
| A | a4b 提交的快照只比 id 与状态 | `wiki/app` | 死：`another_submission`、`a_report_kept` |
| A | a6b 提交放过只有 `preserved` 不过的 | `wiki/app` | 死 |
| A | r274-floor（`ceil` 改 `floor`）、r274-now-at-submit（秒数不看答复时的时钟，恒为 `SubmitTimeout`）、r274-no-floor-of-one（去掉"至少 1 秒"） | `wiki/app` | 都死在 `TestTheSubmissionsDeadline` |
| A | r274b | 组合根 `TestUpgradeSubmit/^the_deadline`；没改的树两轮 | 死在两格；没改的树上两个入口第一轮都是 2 秒、第二轮都是 3 秒，可接管的那一刻在收到答复之后 1.997–1.999 秒 |
| B | gm3 | 计划守卫 | 死，14.4 秒，只有新判法报出（`rollbackReadProblems` 照旧一句不报） |
| B | gm7（在 `b6ef8599` 与 `c3c35bbc` 上各一次） | 同上 | 都死：规划器选了只有 `parent_id` 的唯一索引，改前的帮手已判出 |
| B | gj1 新判法只要有索引就过 | `TestTheGateReadsReadANodeByItsKey` | 死在"the row's notebook's nodes""the row's folder's nodes" |
| C | c1、c11、c12、c13 | 各层的包 | 都死 |
| C | s1 存报告时 `WantedBefore` 取 `WantedNow` | `wiki/adapter/postgres` | 活（P2S2C-M4）；换回 `c3c35bbc` 的夹具死 |
| C | s2 REST 的 `wanted_before` 取 `WantedNow` | `wiki/adapter/http` | 活（P2S2C-M4）；换回 `c3c35bbc` 的夹具死 |
| C | `TestTheMCPCallStopsWhenTheClientGoes` 的负载复现（不是变异） | 组合根的测试二进制，`GOMAXPROCS=1`、8 个 `yes`、40 次 | 40 次全过（改前 5 次失败） |

**修复第二轮**（S2 的 `mut/run.sh`，经 `heavy.sh`，先在没改的树上跑同样的命令）：

| 变异 | 包 | `b6ef8599` | `9dce4658` |
|---|---|---|---|
| s1 | `wiki/adapter/postgres` | 活 | 死：`TestTheReportRoundTrips` |
| s2 | `wiki/adapter/http` | 活 | 死：`TestTheCheckIsAnswered`、`TestTheReportIsAnswered` |
| c1 | `wiki/adapter/http` | 死 | 死：`TestTheReportIsAnswered` |
| c11 | `wiki/domain`、`wiki/app` | 死 | 死 |
| c12 | `wiki/adapter/postgres` | 死 | 死：`TestTheReportRoundTrips`（另一个方向）、`TestTheWorstReportFits` |
| c13 | `wiki/adapter/mcp` | 死 | 死：`TestTheCheckFits` |

**修复核对第二轮**（C，`9dce4658`；规格 `p2s2/review/c/check2/`，组合根的格另经 `heavy.sh`，改前先在没改的树上跑过）：

| 变异 | 测试 | 结果 |
|---|---|---|
| s1、s2、c1、c11–c13 | 各层的包 | 都死 |
| s3–s7（wanted 两个方向的探针，见 P2S2C-M4） | `wiki/adapter/postgres`、`wiki/adapter/http` | 都死 |
| p1 存报告时 `WikiPages` 取 `NewPages` | `wiki/adapter/postgres`；组合根 `TestUpgradeSubmit` | 都活（P2S2C-M5） |
| p2 读报告时 `Sources` 取 `Changesets` | `wiki/adapter/postgres` | 活（P2S2C-M5） |
| m1 MCP 的 `wanted_before` 取 `WantedNow`，m2 `wanted_before.count_capped` 恒假，m3 `wiki_pages` 取 `Sources` | `wiki/adapter/mcp` | 都活（P2S2C-M5） |
| r1 REST 的 `wiki_pages` 取 `Sources` | `wiki/adapter/http`；组合根 `TestUpgradeGates` | 单元活，组合根死（计数两格整值比五个数） |

**修复第三轮**（规格 `fix3/mut/spec.json` 72 条：实施者的对调 55 条，每层五个数各取另一个成员的值、两个 wanted 互换、四个 `count_capped` 各恒假恒真，判定层在 `return c` 之前插一句赋值；加 C 的 17 条 c1、c11–c13、s1–s7、m1–m3、p1、p2、r1；S2 的 `mut/run.sh`，经 `heavy.sh`，每轮先在没改的树上把四条命令各跑一次，都 rc=0；没有一条因编译失败而死）：改前 28 条活、改后 72 条全死。

| 层 | `9dce4658` 上活的 | `ef8ac10f` 上 |
|---|---|---|
| 判定 | j-NewPages | 全死：`TestJudgeUpgradesCounts` |
| REST | h-Sources、h-WikiPages、h-Changesets、r1 | 全死：`TestTheCheckIsAnswered`、`TestTheReportIsAnswered` |
| 存 | e- 的五个数、p1 | 全死：`TestTheReportRoundTrips`（封顶恒假的另加 `TestTheWorstReportFits`） |
| 读 | d- 的五个数、p2 | 全死：`TestTheReportRoundTrips` |
| MCP | x-Sources、x-MovedIntoSources、x-WikiPages、x-Changesets、x-WantedBefore 与它的封顶恒假、恒真，x-WantedNow 的封顶恒真，m1–m3 | 全死：`TestTheCheckFits` |

**修复核对第三轮**（C，`ef8ac10f`；规格与结果在 `p2s2/review/c/check3/`；探针只改一处、只跑它那一层的包，注明组合根的另经 `heavy.sh` 跑）：

| 变异 | 测试 | 结果 |
|---|---|---|
| C 的 17 条 | 各层的包 | 都死（r1 在修复核对第二轮只在组合根死，现在单元也抓） |
| 实施者的 72 条，整轮重跑 | 同上 | 都死，与实施者的结果相同 |
| 抽 8 条，五个测试文件临时换回 `9dce4658` 的 | 同上 | 与实施者的改前相同：6 条活、2 条死 |
| hr1 REST 报告的规则 `CountCapped` 恒真，hr3 恒假，hr2 `Count` 取条目的个数 | `wiki/adapter/http`；hr1 另跑组合根 `TestUpgradeSubmit` | 都活（P2S2C-M6） |
| hg1 REST 的 `preserved` 的 `nodes` 取 `nodes_kept` | `wiki/adapter/http`；组合根 `TestUpgradeGates` | 都活（P2S2C-M6） |
| hg2 `pages` 取 `pages_kept`、hg3 `pages_worse` 取 `pages_kept`、hg5 `index` 取 `source_root`，hg4（即 c2） | `wiki/adapter/http`；组合根 `TestUpgradeGates` | 单元活，组合根死 |
| xg1 MCP 的 `nodes_kept` 与 `pages` 对调，xg2 `frontmatter` 取 `sources`，xg3 `index` 取 `agents` | `wiki/adapter/mcp` | 都活（P2S2C-M6） |
| eg1 存与读的 `preserved` 的数对调，eg3 条目的 `created_at` 取报告的 `computed_at` | `wiki/adapter/postgres` | 都死 |
| eg2 存报告时 `frontmatter` 取 `sources` | 同上 | 活（生产里看不到，"接受"） |
| dg1、dg2 判定的 `preserved` 的 `Nodes`、`Pages` 取保留数 | `wiki/domain` | 都死 |

xg4（MCP 的 `stale_pages` 取 `Changesets`）写错了类型、编译不过，不算数。

**修复第四轮**（规格 `fix4/mut/spec.json` 555 条，由 `fix4/mut/mkspec.py` 从每个函数的模板生成，生成器先核对模板填上原值在文件里恰好出现一次：八道闸门两两对调与恒真恒假，`passed` 与 `count_capped` 恒真恒假与互取，整数两两对调、取 0、取节点数或条目数，列表、字符串、指针、时间与 kind 的对调与常量，循环里每次取第一项，闸门或计数整个取零值，视图不带报告；加第三轮的 72 条与 C 的 hg1–hg5、hr1–hr3、xg1–xg3、eg1–eg3、dg1、dg2，去掉编译不过的 xg4；改前那一轮 3 条因 `for` 的变量没人用编译不过，一并改掉循环变量后单独重跑，都活；改后那一轮没有编译不过的）：

| 函数 | 条数 | `ef8ac10f` 上活 | `d4324ab8` 上活 |
|---|---|---|---|
| REST 的检查 `CheckUpgrade` | 9 | 3 | 0 |
| REST 的规则页 `ListUpgradeCheckItems` | 5 | 3 | 0 |
| REST 的闸门 `upgradeGatesOf`（含 hg1–hg5） | 75 | 60 | 0 |
| REST 的计数 `upgradeCountsOf`（含第三轮与 c1、s2、s6、s7、r1） | 23 | 0 | 0 |
| REST 的报告 `reportOf`（含 hr1–hr3） | 19 | 16 | 0 |
| REST 的报告的条目 `reportItem` | 36 | 26 | 0 |
| REST 的视图的报告 `upgradeOf` | 1 | 0 | 0 |
| MCP 的检查：顶层与规则 | 7 | 6 | 0 |
| MCP 的闸门（含 xg1–xg3） | 73 | 53 | 0 |
| MCP 的计数（含第三轮与 c13、m1–m3） | 22 | 0 | 0 |
| 存：`encodeReport`（含第三轮、c12、s1、s4、s5、p1、eg1–eg3） | 131 | 24 | 0 |
| 读：`decodeReport`（含第三轮、s3、p2） | 125 | 23 | 0 |
| 判定：`ReportOf` | 15 | 3 | 0 |
| 判定：计数（第三轮的 j-*、c11） | 12 | 0 | 0 |
| 判定：`preserved` 闸门（dg1、dg2） | 2 | 0 | 0 |
| **合计** | **555** | **217** | **0** |

改前活的里有代表性的：REST 的闸门 28 对布尔对调活 23 对、16 条常量活 15 条；REST 的报告的规则的数恒真、恒假、取条目数、取 stale，`passed` 恒真、恒假、取 agents；REST 的条目原来只断言个数与 rule；MCP 的八道闸门的 44 条全活；存与读活的都是 `fullReport` 里同值的那几对与只有一个方向的常量；`ReportOf` 的 stale 取 0、`passed` 恒假、规则名取第一条。逐条的名单与杀死它的测试在 `fix4/mut/after.out`。

**修复核对第四轮**（C，`d4324ab8`；规格与结果在 `p2s2/review/c/check4/`；组合根的探针经 `heavy.sh`，每条之前在没改的树上跑过同样的命令）：

| 变异 | 测试 | 结果 |
|---|---|---|
| hr1–hr3、hg1、xg1–xg3、eg2 | 各层的包 | `ef8ac10f` 的夹具上活，`d4324ab8` 上都死 |
| 实施者的 555 条，整轮重跑 | 各层的包 | 都死 |
| k2、k3 存或读时条目的 `kind` 恒为 `asset`；st1 `Current` 的 `SubmissionID` 取 `SubmittedByID`，st2 `Submit` 的提交 id 与提交人对调 | `wiki/adapter/postgres` | 都死 |
| wr1 组合根转换的 `AgentsBody` 取 `RootPage`，wb1 `Wanted` 取 0，wb2 `ParentID` 取 nil，wp2 `PagesKept` 取 `Pages` | 组合根 `TestUpgradeGates` | 都死 |
| wp1 组合根转换的 `NodesKept` 取 `Nodes` | 同上 | 活（P2S2C-M7） |
| wp3 `Nodes` 取 `NodesKept`，wr2 `RootPage` 恒真 | 同上 | 活（等价，或只遮住一条缺陷日志，"接受"） |
| k1 REST 报告条目的 `kind` 恒为 `asset` | `wiki/adapter/http` | 活（P2S2C-N9） |
| lg1 提交的日志的 `user_id` 取工作区的 id | `wiki/domain`、`wiki/app` | 活（P2S2C-N10） |

lg2 编译不过，不算数。

**修复第五轮**（规格 `fix5/mut/spec.json` 11 条：C 的 wp1、wp2、k1、k2、k3、lg1，实施者加的组合根 `Preserved` 转换的其余成员（`NodesKept` 取 `Pages`、取 `PagesKept`，`PagesKept` 取 `NodesKept`，`NotKept` 取 nil）与规则页的 `mode` 取夹具里那个值；S2 的 `mut/run.sh`，经 `heavy.sh`，每轮先在没改的树上把各条命令跑一次，都 rc=0）：改前活 4 条，改后 11 条全死；第四轮的 555 条在 `43caa32a` 上整轮重跑，全死，没有一条编译失败。

| 变异 | `d4324ab8` | `43caa32a` |
|---|---|---|
| wp1 组合根 `NodesKept` 取 `Nodes` | 活 | 死：`TestUpgradeGates/a_baseline_attachment_deleted_past_the_units` |
| wp2 组合根 `PagesKept` 取 `Pages` | 死 | 死：`…/a_baseline_page's_body_past_the_units` |
| 组合根 `NodesKept` 取 `Pages`、取 `PagesKept`，`PagesKept` 取 `NodesKept` | 死 | 死 |
| 组合根 `NotKept` 取 nil | 死 | 死：`…/a_baseline_page's_body_past_the_units` 等 |
| k1 REST 报告条目的 `kind` 恒为 `asset` | 活 | 死：`TestTheReportIsAnswered` |
| k2、k3 存或读时 `kind` 恒为 `asset` | 死 | 死：`TestTheReportRoundTrips`、`TestTheWorstReportFits` |
| lg1 日志的 `user_id` 取工作区的 id | 活 | 死：`TestTheSubmitterIsDecidedBeforeTheAnswer` |
| hi-MD=llm 规则页的 `mode` 取 `llm_wiki` | 活 | 死：`TestTheCheckIsAnswered` |

**修复核对第五轮**（C，`43caa32a`；规格与结果在 `p2s2/review/c/check5/`；每轮先在没改的树上跑过同样的命令，组合根的经 `heavy.sh`）：

| 变异 | 测试 | 结果 |
|---|---|---|
| wp1 | 组合根 `TestUpgradeGates` | 死，只死在新格（两处断言都用 `t.Errorf`，照样走到还原，没有连累后面的格） |
| wp2 | 同上 | 死 |
| k1、k2、k3 | `wiki/adapter/http`、`wiki/adapter/postgres` | 都死 |
| lg1 | `wiki/app` | 死 |
| hi-MD=llm | `wiki/adapter/http` | 死 |
| ws1 组合根 `SetState` 把 upgrading 译成 reviewing（`wiki_upgrade.go:62`） | 组合根 | 死：`TestUpgradeSubmit` 三格 |
| wl1 组合根丢掉 `AreaWantedByPage`（`wiki_lint.go:142`） | 组合根 | 死：`TestUpgradeStart/a_raw_page_is_the_source_root` |
| mv1 MCP `upgrade_verify` 的 `rule` 与 `cursor` 对调 | `wiki/adapter/mcp`；组合根 | 单元活，组合根 `TestUpgradeCheck/the_codes_in_their_order` 死 |
| wb3 组合根 `Baseline` 的 `kind` 恒为 page | 组合根 `TestUpgrade(Gates\|Check\|Submit)` | 活，等价（"接受"） |

## 核对

| 轮 | 对象 | 发现 | 处置 |
|---|---|---|---|
| 1 | `b6ef8599`（修复一轮 9 个提交），三位各在自己的工作树里 | A：P2S2A-M1–M3、N1、N2 关，M4、N3 交给文档；第 274 条的秒数、契约与两层测试对，下界的断言不会偶发；没有新发现。B：P2S2B-M1、M2、N1、Q1 关，B 的 4.7 关；挪后两份折到 7763 第二份约 360 秒、第八份约 193 秒；清掉的变更集的结论对；新的 P2S2B-N2。C：P2S2C-M1–M3、N4–N6 关，N7、Q1 的处置妥当；文档稿里 N1–N3 与 P2S2A-M4、N3 都落了、落对了；新的 P2S2C-M4、N8 | P2S2B-N2 不改；P2S2C-M4 由修复第二轮改；P2S2C-N8 在本提交 |
| 2 | `9dce4658`（修复第二轮 1 个提交），C | P2S2C-M4 关：s1、s2 与另加的 s3–s7 都死，最坏的报告的取法对；新的 P2S2C-M5：存下的报告的五个计数与 MCP 投影的计数，S2 建成时就有 | P2S2C-M5 由修复第三轮改 |
| 3 | `ef8ac10f`（修复第三轮 1 个提交），C | P2S2C-M5 关，实施者的变异表核过属实，计数这一类扫清；新的 P2S2C-M6：REST 报告的规则的数、`preserved` 的 `nodes`、MCP 投影的闸门 | P2S2C-M6 由修复第四轮按投影收尾 |
| 4 | `d4324ab8`（修复第四轮 1 个提交），C | P2S2C-M6 关，555 条整轮重跑全死，七个投影里这一类收尾；范围放宽到 S2 写的所有逐字段转换，新的 P2S2C-M7（组合根的 `Preserved` 转换）、N9（REST 报告条目的 `kind`）、N10（提交的日志的 `user_id`） | 三条都由修复第五轮改 |
| 5 | `43caa32a`（修复第五轮 1 个提交），C | P2S2C-M7、N9、N10 关；补查 S2 其余的逐字段转换（ws1、wl1、mv1 死，wb3 等价），这一类收尾；新的 P2S2C-N11；没有 Important、Minor 留下 | P2S2C-N11 不改，记在"推后"与设计 7.1 |

## 实测

- **组合根的各份**（竞态构建的口径；`go test -race -list '.*' ./internal/bootstrap`，照 `Makefile` 的八个模式以 `m11/tools/shards8.py` 归类）：头 `c3c35bbc` 上 547 个顶层，145、51、121、124、62、11、27、6，各在恰好一份（第一轮的 `c3bfdd54` 上同样；B 照 `Makefile` 另归一次，数相同）。比 S1 末的 542 个多 5 个：`TestUpgradeCheck`、`TestUpgradeGates`、`TestUpgradeSubmit`、`TestUpgradeSubmitInterleavings`、`TestUpgradeToolsMatrix`；第七份少的一个是 P1 的 `TestUpgradeToolsErrorsCarryTheirCodes`，挪到第八份。两个新守卫带 `//go:build !race`，不在这里。本机单独两轮（`heavy.sh make -j1 test-go-bootstrap-<n>`，包的时间）：第七份 185.1、179.5 秒（头 `c3bfdd54`，负载 2.0、1.0；之后第七份的测试没有改；S1 建成时 172.4、174.6 秒，修复第二轮之后 183.9、181.4 秒；B 一轮 175.4 秒），照设计 7.5 的 7763 与本机之比 1.68 折约 302–311 秒；第八份 46.7、46.3 秒（头 `c3c35bbc`；B 一轮 46.1 秒），折 7763 约 78 秒。第 272 条挪后（f1 起）：547 个顶层，145、50、121、124、62、11、27、7，各在恰好一份（实施者与 B 各归一次）；第二份本机 204.4、202.0 秒（f1）、204.1、202.6 秒（头 `b6ef8599`，门禁里），少约 61 秒；第八份 98.6、99.8、99.3、100.3 秒，多约 53 秒；修复第五轮在第八份加一格之后本机一轮 97.9 秒（开始时负载 2.7，新的一格 0.06 秒）。照 1.61 加 33 秒折到 7763：第二份约 360 秒、第八份约 193 秒（B；B 另从修复一轮的 CI 交叉核对：第二份约 357 秒、第八份约 185–205 秒），都在 450 秒之下。修复第三、四轮的 CI 恰好各有一份落在 7763 上：第二份 Test 一步 400 秒、第八份 187 秒（"门禁与 CI"）。
- **第二份慢约 5% 的原因**：实施者在分支 CI 上看到第二份过线之后补量（`m11/p2s2/shard2.sh`、`shard2v.sh`，两个头交替，负载约 1）：`c3c35bbc` 263.2、264.4 秒，`ba72cda4` 250.8、252.7 秒；各再跑一轮 `-v`，243.3 → 253.1 秒，51 个测试的和多 9.7 秒，摊在许多测试上（`TestVirtualExport` 48.2 → 51.7 秒，其余各零点几秒），S2 碰到第二份的只有逐个操作或工具的几个测试，没有找到哪条热路径变了。B 查清（B 的 4.4）：S2 给每个应用的启动只多了闸门的读的端口、`ReservedKeys` 与 `SubmitTimeout`，第二份的 51 个测试一个也不碰升级；增量来自测试的夹具每建一个应用就 `apitest.Load` 一次（读 `api/dist/openapi.yaml`、`doc.Validate`、建路由），S2 让这个文件从 286,572 字节长到 306,033 字节（+6.8%），竞态构建下一次从约 571 ms 到约 633 ms（各五次、三轮的中位，C 另量 567 → 625 ms）；第二份里不到 1 秒的小测试各多 0.06–0.09 秒，合计 +9.7 秒约合 150–160 次 `Load`。CI 的 7763 上第二份的包从 `main` 的 398.6–405.3 秒到 423.8 秒（+4.6–6.3%），与本机的 5% 一致，不是起伏。不是缺陷，是契约变大的比例代价（P2S2B-Q2）。
- **5 万页的检查**（`TestUpgradeCheckTakesItsTimeAtAnyStatistics`，组合根，costs-2 的末尾，`!race`、`-short` 跳过）：lint 的成本夹具 5 万页做成标准笔记本、以 SQL 设成升级中，它的升级的每个节点一行基线（55,001 行）、这次升级 1,000 个变更集、之前的 10 万个；另九本各 5 万页的笔记本与它们的升级的基线与各 1,000 个变更集；分析过。lint 的守卫的配置（请求 6 s、worker 3 s），整个 `Module.CheckUpgrade`（经 lint 的 `Runs`），两份种子（每一页如账本所收；以 SQL 把一页的账本记成别的哈希，闸门 7 不过）各三次取中位数。界取本机两轮较大的中位数 489 ms × 3、向上取整到 100 ms（1.5 s），与 worker 的 3 s 的八成（2.4 s）取小：1.5 s（`checkCostBound`）。

  | 一次 | 每页都保留 / 一页没保留（ms） | 界 |
  |---|---|---|
  | 定界的第一轮（`logs/c9-check-cost-1.log`） | 487 / 487 | （2.4 s，定界之前） |
  | 定界的第二轮（`c9-check-cost-2.log`） | 489 / 485 | （同上） |
  | 逐个提交，第一次（`verify/c9-checkcost.log`） | 491 / 487 | 1.5 s |
  | 逐个提交，第二次（`verify2/`） | 498 / 494 | 1.5 s |
  | `make test` 里（`gates/test.log`） | 501 / 492 | 1.5 s |
  | 审查 B 一轮 | 480 / 475 | 1.5 s |

  一次检查里：规则 285–312 ms（`missing_type`、`missing_summary` 各 82–91 ms），来源 76–92 ms，`UpgradePreserved` 86–96 ms，`UpgradeGateBaseline` 13–19 ms，`UpgradeChangesets` 1–2 ms；升级的种子 11.9–12.1 秒，整个测试 19.7–21 秒；余量约 3 倍。CI 上它落在 9V45：726 / 720 ms，与本机之比约 1.48；B 照同一作业里发起的守卫的比（9V45 / 本机 1.55，7763 / 本机 1.92–2.16）折，检查在 7763 上约 0.92–1.04 s，对 1.5 s 约 1.45–1.6 倍；设计估的 7763 之比 2.2–2.6 偏保守。
- **闸门的三条读的计划**（`TestTheUpgradeGateReadsPlansAtAnyStatistics`，`page/adapter/postgres`，costs-3，紧接回滚的读）：一本 10 万页（1,000 个文件夹各 100 页）笔记本的升级的基线 10 万行、它写的 1,000 个变更集与它之前笔记本的 10 万个；另九本各 5 万页的笔记本，各有它们的升级的基线与 1,000 个变更集。三种统计 × 自定义与通用的计划，每格先判三条语句的计划（照 `rollbackReadProblems`；`UpgradeChangesets` 只经 `changesets_upgrade_id_idx`），再各读三次取中位数；只判计划，时间不设界。

  | 状态 × 计划 | 基线（ms，第一轮 / `make test`） | 保留 | 变更集 |
  |---|---|---|---|
  | 从未分析，自定义 | 39.7 / 39.3 | 314 / 313 | 0.38 / 0.40 |
  | 从未分析，通用 | 32.5 / 34.8 | 301 / 298 | 0.38 / 0.28 |
  | 分析过，自定义 | 32.8 / 37.9 | 310 / 311 | 0.48 / 0.39 |
  | 分析过，通用 | 35.3 / 38.5 | 297 / 295 | 0.31 / 0.28 |
  | 只有别的升级的统计，自定义 | 62.3 / 63.0 | 244 / 241 | 0.47 / 0.36 |
  | 只有别的升级的统计，通用 | 33.1 / 34.2 | 293 / 302 | 0.30 / 0.30 |

  六格的计划在四次运行里都合规；种子 13.8–14.2 秒，连同 `TestTheRollbackGuardJudgesThePlans`（`changesets` 进它的判法、四格）约 22 秒。B 在没改的树上一轮：基线 32.8–55.5 ms、保留 227–282 ms、变更集 0.26–0.41 ms，种子 13.1 秒。修复一轮另判 `nodes` 只经主键（P2S2B-M1）：f6 的逐个核对里这一行 20.5 秒、六格合规；`make test` 里连同新判法六格合规、种子 13.1 秒；B 在头上重跑也过。
- **别的**：最坏的报告（八条规则各五项最长的条目，`TestTheWorstReportFits`）35,125 字节，界 65,536（C 实跑相同）；修复一轮把它的 `wanted_now` 也改成封顶，35,128 字节，第二至五轮不变；内置说明经 SDK 的字节（`instructions_test.go`）：笔记本入口 3,093 → 3,276，账户入口 3,163 → 3,346，界 3,796（C 实跑相同）；UP5 单独跑（`make build` 之后 `stories/upgrade/`，6–7 秒，UP1–UP5 共 19 个都过），`make e2e` 里 API 版本 213 ms、MCP 版本 242 ms；`make e2e` 522 个（S1 末 520 个）；vitest 192 个文件、4,345 个测试。修复一轮之后检查的守卫在 `make test` 里中位数 474、473 ms（界 1.5 s）；`make e2e` 522 个，2.2 分钟。

## 门禁与 CI

本机（PostgreSQL 18 在 Docker 里，要 Docker 的与计时的都经 `heavy.sh`）：

- **逐个提交**（`m11/p2s2/verify.sh`）：在工作树里逐个 detached（每个之前与之后核对 `git status` 干净），每个跑 `cd server && go build ./... && go vet ./...`、`make lint-go`、`make gen-check`、`make test-go-short`，再跑它自己的：1 号第八份（这时只有 `TestUpgradeToolsErrorsCarryTheirCodes`，13 秒）；2 号 `^TestTheMCPCallStopsWhenTheClientGoes$`；3 号 wiki 模块与 linking 的 app 的竞态单元、`TestUpgradeStart`、`TestUpgradeStatesReachTheStreams`；4 号 page 模块的两个闸门的读的测试；5 号 wiki 模块的竞态单元；6 号 wiki 模块的单元、`TestUpgradeCheck`、`TestUpgradeGates`、两个矩阵与 `TestLintMatrixTargets`、网页三项；7 号 wiki 模块的单元、`TestUpgradeSubmit`、`TestUpgradeSubmitInterleavings`、`TestUpgradeStatesReachTheStreams`、两个矩阵、网页三项；8 号 wiki 模块、MCP 平台、page 与 linking 的 MCP 适配器的单元，组合根的工具矩阵、码、工具表、契约、说明的九个测试与 `TestUpgradeCheck`、`TestUpgradeSubmit`（83–92 秒），网页三项；9 号两个守卫与 lint 的十万页守卫；10 号网页三项、`make build` 与升级的故事。第一次（十个，头 `c3bfdd54`）只有 7 号的组合根一项 rc=1（交错第 7 条，同门禁第一轮）；照门禁改过 7、8 号之后第二次核对 7–10 号（`74a3057a`、`d8d2bbaf`、`a6e8cb67`、`c3c35bbc`），每一项 rc=0。1–6 号两次改写都没有动。审查 B 另在 3、6、7、8 号上各自跑 build、vet、`make lint-go`（0 issues）、`make gen-check`（之后工作树仍干净）、`make test-go-short`（94 个包 ok），8 号另经 `heavy.sh` 跑竞态的 `^TestToolStrings`（3.08、1.28 秒）与 `^TestUpgradeToolsErrorsCarryTheirCodes$`（11.25 秒），都过；静态核对跨提交的依赖（矩阵的行、`TestLintMatrixTargets` 的格与覆盖测试的 `{rule}` 在 6 号；网页的两种语言的文字与 `report: null` 在 7 号；`validToolArguments`、两张码表、LW8 的工具数在 8 号），没有哪个提交要靠后面的提交才绿。
- **第一轮门禁**（头 `c3bfdd54`，`m11/p2s2/gates.sh`，每一项之前判 df，整轮 40–41G 可用）：

  | 门禁 | 结果 | 用时 |
  |---|---|---|
  | `make lint gen-check knip build` | rc=0 | 8 秒 |
  | `heavy.sh make test` | rc=2，`test-go` 停在三份上 | 582 秒 |
  | `heavy.sh make test-go-short` | rc=0 | 41 秒 |
  | `heavy.sh make e2e` | rc=2，9 个失败（LW8 三个测试 × 三个浏览器），513 个通过 | 140 秒 |
  | 第八份两轮 | 都 rc=2，`TestUpgradeSubmitInterleavings/7` | 46、46 秒 |

  失败与处理：第三份的 `TestToolStringsDoNotAnswerInternalErrors`、`TestToolStringsCatchASchemaRefusal`（每个工具要在 `validToolArguments` 里有一组能到用例的参数，8 号漏了两个新工具）与 LW8 的三个（两个入口的工具数 16、18 现在是 18、20）在 8 号补上，范围加 `e2e`；交错第 7 条（回滚的 worker 在本机常在测试读之前做完，读"进行中的升级"读到空）在 7 号改成不看回滚到了哪一步；另两次偶发见"两个偶发"。
- **第二轮门禁**（头 `c3c35bbc`，开始前与结束时 df 40G，工作树干净）：`make lint gen-check knip build` 8 秒；`make test` 3,501 秒通过（竞态、组合根八份、成本三份、lint 的成本、守卫两份、vitest 192 个文件 4,345 个测试；检查的中位数 501、492 ms，闸门的读六格的计划都合规）；`make test-go-short` 41 秒；`make e2e` 522 个 138 秒（Playwright 2.3 分钟）；第八份两轮 48、47 秒。第一轮那两次偶发这一轮都没有再出。
- **修复一轮**（`fix1/verify.sh`、`gates.sh`，整个经 `heavy.sh`，每一项之前判 df，整轮 37G，没有修剪）：逐个提交每个跑 build、vet、`make lint-go`、`make gen-check`、`make test-go-short`，再跑它碰到的（f1 的 `-list` 归类，f3 那个测试，f4 两个交错，f5 wiki 模块全部竞态单元与两个工具的组合根测试，f6 两个计划的测试，f7 `TestUpgradeSubmit` 与读契约的两个测试、网页三项，f8 `TestUpgradeStart`、`TestUpgradeCheck`，f9 网页三项、`make build` 与 UP1–UP5），九个提交的每一项 rc=0。头 `b6ef8599`：`make lint gen-check knip build` 7 秒；`make test` 3,342 秒通过；`make test-go-short` 41 秒；`make e2e` 522 个 135 秒；第二份、第八份各两轮（作业 205、203 秒与 100、101 秒）；`go test -race -list` 547 个、八份各在恰好一份。S2 第一轮门禁里那两次偶发都没有再出。
- **修复第二轮**（`fix2/verify.sh`，经 `heavy.sh`，df 37G）：头 `9dce4658` 上 build、vet、`make lint-go`、`make gen-check`、`make test-go-short`（39 秒）、wiki 模块的 http 与 postgres 适配器的单元（最坏的报告 35,128 字节）、wiki 模块全部竞态单元（48 秒），都 rc=0；只改了单元测试的夹具，照 `fix-2.md` 不跑 `make test` 与 `make e2e`。
- **修复第三轮**（`fix3/`，经 `heavy.sh`，df 36–37G）：头 `ef8ac10f` 上 `go build ./...`、`go vet` wiki 模块、`make lint-go`（0 issues）、`make gen-check`、`make test-go-short`（94 个包）、wiki 模块全部竞态单元（6 个包，http 46.9 秒）、postgres 的两个报告测试（35,128 字节），都 rc=0；只改单元测试，照 `fix-3.md` 不跑 `make test` 与 `make e2e`。
- **修复第四轮**（`fix4/`，经 `heavy.sh`，df 34–35G）：头 `d4324ab8` 上 `go build ./...`、`go vet ./...`、`make lint-go`（第一次报 `gochecknoglobals` 三处，三个包里的 `gateRows` 改成函数之后 0 issues）、`make gen-check`、`make test-go-short`（94 个包，40 秒）、wiki 模块全部竞态单元（52 秒）、postgres 包（6 个测试，35,128 字节），都 rc=0；同上不跑 `make test` 与 `make e2e`。
- **修复第五轮**（`fix5/`，经 `heavy.sh`，df 31–32G）：头 `43caa32a` 上 `go build ./...`、`go vet ./...`、`make lint-go`（0 issues）、`make gen-check`、`make test-go-short`（94 个包，40 秒）、wiki 模块全部竞态单元（53 秒）、`TestUpgradeGates`（14 格，新格 0.06 秒，整个测试 2.04 秒）、`make -j1 test-go-bootstrap-8`（包 97.9 秒），都 rc=0；照 `fix-5.md` 的核对清单，没有跑 `make test` 与 `make e2e`。提交之前有一次 wiki 模块的测试没有经 `heavy.sh`（"修复第五轮的说明"）。

CI（作业，括号里是测试一步；e2e 是 E2E 一步；runner 由审查 B 或实施者从作业日志取）：

| 作业 | 42882105107（建成，`c3c35bbc`） | 42913206700（修复一轮，`b6ef8599`） | 42921758062（修复第二轮，`9dce4658`） | 42927783261（修复第三轮，`ef8ac10f`） | 42940795835（修复第四轮，`d4324ab8`） | 42951453163（修复第五轮，`43caa32a`） |
|---|---|---|---|---|---|---|
| server (race) | 11:08（10:47），9V74 | 14:45（14:24） | 11:22 | 14:38 | 11:08 | 9:02 |
| server (bootstrap-1) | 6:45（6:23），7763 | 4:34（4:16），9V45 | 5:39 | 4:38 | 5:43 | 6:47 |
| server (bootstrap-2) | 7:57（7:37），7763 | 5:08（4:48），Xeon 8573C | 5:14（4:52），9V74 | 7:00（6:40），7763 | 5:12（4:52），9V74 | 4:08（3:49），9V45 |
| server (bootstrap-3) | 7:30（7:11），9V74 | 6:18（5:54），9V74 | 7:31 | 7:30 | 5:20 | 6:10 |
| server (bootstrap-4) | 6:38（6:16），9V74 | 6:37（6:17），9V74 | 7:36 | 6:37 | 6:01 | 7:49 |
| server (bootstrap-5) | 6:07（5:49），9V74 | 8:11（7:53），7763 | 6:19 | 6:25 | 7:39 | 6:23 |
| server (bootstrap-6) | 5:47（5:26），7763 | 5:29（5:11），Xeon 8573C | 5:54 | 5:47 | 5:06 | 5:10 |
| server (bootstrap-7) | 4:43（4:20），9V74 | 5:18（4:58），9V74 | 5:36 | 5:29 | 5:32 | 4:36 |
| server (bootstrap-8) | 1:56（1:37），Xeon 8573C | 2:45（2:27），9V45 | 3:11（2:50），9V74 | 2:36（2:18），9V45 | 3:27（3:07），7763 | 2:30（2:12），9V45 |
| server (costs-1) | 12:51（9:57），9V74 | 18:57（14:46） | 15:32 | 13:00 | 15:13 | 17:17 |
| server (costs-2) | 14:48（14:28），9V45 | 13:42（13:23），9V45 | 13:19 | 24:54，7763 | 22:27，9V74 | 24:58 |
| server (costs-3) | 14:46（14:25），9V74 | 15:11（14:47），Xeon 8573C | 16:33（16:10），Xeon 8573C | 13:42（13:21），9V74 | 13:38（13:15），9V74 | 13:55（13:34），9V74 |
| server (costs-lint) | 11:58（11:40），Xeon 8573C | 10:17（9:54） | 8:13 | 12:51 | 12:19 | 13:04 |
| server (guards-core) | 15:43（15:14），9V74 | 18:27（18:09） | 16:11 | 15:11 | 19:14 | 18:31 |
| server (guards-sources) | 16:38（16:18），9V74 | 13:35（13:17） | 13:54 | 13:35 | 13:43 | 14:38 |
| server (short) | 3:26（3:09） | 4:32（4:07） | 4:22 | 4:22 | 3:31 | 4:22 |
| web | 3:16（2:44） | 3:40（3:10） | 4:59 | 3:36 | 3:25 | 4:53 |
| e2e | 8:37（7:17） | 第一次 attempt 红（L1），重跑 8:08（6:05） | 7:22（6:03） | 8:06（6:27） | 7:51（6:17） | 6:37（5:28） |
| image | 1:20 | 1:27 | 1:26 | 1:27 | 1:27 | 1:16 |

- 建成时一次全绿，19 项（多的一项是 `bootstrap-8`），普通推送（新分支，`git ls-remote` 是 `c3c35bbc8fe8effc4e8d45be333fdd12802d2595`）。细看的几份：第七份在 9V74 上包 234.0 秒、Test 一步 260 秒；第八份在 Xeon 8573C 上包 66.9 秒、Test 一步 97 秒；照 7763 与本机之比 1.61 加约 33 秒的编译，B 折第七份约 315 秒、第八份约 108 秒。**第二份**在 7763 上包 423.8 秒、Test 一步 457 秒，过了 450 秒的线（`main` 上最近两次 438、430 秒，S1 的修复第二轮在分支上 409 秒），即第 272 条。costs-2 在 9V45 上 14:48（时限 45 分钟）：检查的守卫 30.7 秒（种子 16.8 秒），中位数 726、720 ms；末行在第 13.9 分钟开始。costs-3 在 9V74 上 14:46：闸门的读的计划 34.3 秒（种子 21.6 秒），六格的计划都合规，中位数基线 60–83 ms、保留 333–436 ms、变更集 0.33–0.45 ms；`-timeout 20m` 一行在第 11.8 分钟开始，即第 275 条的依据（P2S2B-M2）。guards-core（40 分钟）与 guards-sources（30 分钟）S2 没有改到。
- 修复一轮（普通推送 `c3c35bbc..b6ef8599`，`git ls-remote` 是 `b6ef85993fccb610bb906c616a0d02efaa03d69a`）：run 42913206700 第一次 attempt 18 项绿、e2e 红在 L1（"偶发"），`gh run rerun --failed` 之后 e2e 绿，run 的结论 success，19 项全绿。第二份这次落在 Xeon 8573C 上，Test 一步 288 秒（包 263.3 秒），没有落在 7763 上，不能直接与 457 秒比；第八份在 9V45 上 147 秒（包 123.4 秒）；costs-3 在 Xeon 8573C 上 15:11（时限 40 分钟），`-timeout 20m` 那一行在第 11.7 分钟开始，闸门的读的计划 37.7 秒；costs-2 在 9V45 上，检查的中位数 656、647 ms。
- 修复第二轮（普通推送 `b6ef8599..9dce4658`，`git ls-remote` 是 `9dce465850750c22cc1d75415b7c6334068319bb`）：run 42921758062 19 项一次全绿；第二份在 9V74 上 Test 一步 292 秒（包 265.5 秒），第八份在 9V74 上 170 秒（包 142.9 秒），costs-3 在 Xeon 8573C 上 16:33（`-timeout 20m` 那一行在第 12.7 分钟开始）。第二份、第八份这两次都没有落在 7763 上。
- 修复第三轮（普通推送 `9dce4658..ef8ac10f`，`git ls-remote` 是 `ef8ac10fff25d4c1832bd3e764f729a48765486c`）：run 42927783261 19 项一次全绿。第二份这次落在 7763 上：Test 一步 400 秒（包 365.6 秒），第 272 条挪后第一次落在 7763，在 450 秒的线之下；比 B 估的约 374 秒与修复一轮折的约 360 秒都高，这一轮只改了单元测试、第二份的测试没有动，是机器之间的起伏（S2 建成时同一种机器上 457 秒）。第八份在 9V45 上 138 秒（包 115.6 秒）；costs-3 在 9V74 上 13:42；costs-2 落在 7763 上 24:54（`main` 在 7763 上 22.2–24.4 分钟，S2 在末尾加了一行，时限 45 分钟；B 估的约 25.4 分钟相符）。
- 修复第四轮（普通推送 `ef8ac10f..d4324ab8`，`git ls-remote` 是 `d4324ab8c04a137cadd8c71b9f3c1cb1b96d7ffb`）：run 42940795835 19 项一次全绿。第八份这次落在 7763 上：Test 一步 187 秒（包 156.0 秒），与 B 估的约 190 秒相符；第二份在 9V74 上 292 秒（包 265.7 秒），同修复第二轮；costs-3 在 9V74 上 13:38。
- 修复第五轮（普通推送 `d4324ab8..43caa32a`，`git ls-remote` 是 `43caa32adba3a4c7559fed09a6a4cc471e2f5742`）：run 42951453163 19 项一次全绿；第二份在 9V45 上 Test 一步 229 秒（包 208.2 秒），第八份在 9V45 上 132 秒（包 112.1 秒，含新的一格），costs-3 在 9V74 上 13:55；e2e 比别的作业晚约 25 分钟才开始，是在等运行器，不是重跑。合并之后照设计第 8 节第 1 条看两次 `main`。

S2 开工之前 `main` 的两次 CI：S1 合并那次（run 42834026668，`8cf7af36`）18 项一次全绿，七份的测试一步 254、438、405、396、403、318、197 秒；记合并号的 `ba72cda4`（run 42839913448）18 项全绿，七份 355、346、342、304、287、320、382 秒（第七份换了 runner，上一次 197 秒），guards-core 19:44、guards-sources 16:38、costs-2 24:21。两次的平均 304.5、392、373.5、350、345、319、289.5 秒，都在 450 秒之下。

## 环境

整个 Step 磁盘都够：实施者的门禁每一项之前判 df，整轮 40–41G 可用，没有修剪；三位审查者每次重的运行之前看过 `df -h /`，可用 39G。审查者的临时代码都已复原（C 的 `server.go` 从备份复制回去，临时测试与测试二进制删掉，占 CPU 的 `yes` 由 `timeout` 结束、核过没有残留；B 的临时测试移出了工作树），三个工作树最后都回到 `c3c35bbc`、`git status` 干净、没有 `*.mutbak`。五轮修复与各轮修复核对每一项重的之前判 df，31–37G，没有修剪；审查者的临时改动都已复原，工作树回到各自的头、`git status` 干净。

合并之后 `main` 的第一次 CI（run 42959287160，`c61df45a`）19 项一次全绿，数见 [P2 设计](../02-P2-guardrails-gate-submit.md) 10.2 的"合并之后"；第三、第五份在 EPYC 7763 上到了 450 秒的线，开 S3 之前先挪份（00 号文档第 276 条）。
