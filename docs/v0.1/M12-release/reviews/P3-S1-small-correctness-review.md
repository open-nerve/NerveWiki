# M12/P3 S1（小的正确性）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-p3-s1`。实施的 8 个提交，从 `main` 的 `48bbfc99`（P3 设计）开始，审查时在 `2200b5d6`：<br>标题键的切罗基文一步、`searchFold` 删掉 `c3eade4e`；标题 id 带上自动链接与原始 HTML `76a159bc`；标签列表不列 `/` `4e813da1`；什么也不显示的名字算空 `0f0d7ea5`；`log` 的 `"/"` 与 `""` `3b7e7db9`；MCP 超时的文字与提交错误的标记 `a89ee212`；日志与变更集的详情各在一个快照里、交错 `81b38143`；事件流每个凭证 4 条、每个账户 64 条 `2200b5d6`。<br>修复第 1 轮 6 个提交，头 `1f1d3d11`：`FaultUnknown` 一支也认提交的标记 `ac30ec3d`；每个 PAT 各是一个凭证、顶掉时减被顶掉那一条的账户 `1d7b471d`；标题 id 的 `www.` 四格 `07da4f76`；linking 的替身拒尾随的 `/` `8f30b563`；`make help` 的第七份 `aab8244b`；网页的注释 `1f1d3d11`。<br>修复第 2 轮 4 个提交（只补核对第 1 轮的 Nit），头 `a7723228`：替身对 `"/"` 答 `required` `1812134f`；`internalFailedSaving` 的注释 `a9f8dd95`；hub 的 `uncount` `7c49a816`；网页注释的 429 两种起因 `a7723228`。<br>两次合并 `main`：`d1e90929`（P2、P4、P5 的设计与 P3/S2，冲突只在 `Makefile`）、`892dd827`（S2 的合并号，没有冲突）。<br>分支共 18 个提交（不计合并），相对 `main` 67 个文件，加 1,679 行、减 205 行：shared、page、linking、identity、events，平台的 postgres、mcpserver 与 markdown，契约（`events.yaml`、`linking.yaml`、`tools.json`）与 `make gen` 的生成物，网页只有生成的类型与两个文件的注释，`Makefile`；迁移不动，`README.md` 与 `docs/` 在本提交 |
| 规格 | [P3 设计](../03-P3-server-correctness.md) 3.1–3.9、3.15–3.20、4.1、6.1、7.1、7.3 的 S1、7.5、第 8 节第 1、2、7–10 条、第 9 节；2.3 的决定 365–371（P3-5 到 P3-11）；[00 号文档](../00-M12-design.md) 4.6.2–4.6.5，决定 313、315 |
| 审查方式 | 三位审查者（Opus）并行，各在自己分离的工作树里（`nerve-wiki-m12-p2-rev-{a,b,c}`，挪到 `2200b5d6`），重的经 `heavy.sh`，不碰实施者的工作树；`TMPDIR`、`GOTMPDIR` 在 `~/.cache` 下各自的目录，用完删了：<br>A 标题键与切罗基文、标题 id、标签、名字为空、`log` 的 `path`（提交 1–5）：切罗基文的探针（`cases.Fold` 的大小写对调、不合法的字节、超过 4096 字节跨块边、`IndexKeyer` 逐码位）；13 种标题写法两边的 id；生产的 `Blank` 与 Node 24.21（ICU 78.3，Unicode 17.0）的 `\p{Default_Ignorable_Code_Point}` ∪ `\p{White_Space}` 逐码位比对；在旧代码上跑新测试；变异 M-a、M-b 与 S1-27 的替身。<br>B MCP 超时的文字、提交错误的标记与快照（提交 6、7）：`go test -overlay` 的探针（提交在调用的期限之前与之后超时）；去掉快照、降成 READ COMMITTED、没有标记三种变异；沿每个写工具核对"changed nothing"；快照里的端口都经事务的查询者。<br>C 事件流、契约与网页、分份、每个提交单独为绿、文档草稿（提交 8 与全部）：16 个 goroutine 的并发探针（竞态 `-count=20`）；`credentialOf` 去掉 PAT 一支的变异；偏离 7 的探针（团队的令牌，9 次）；读配方的分份脚本 `shards_c.py`；8 个提交逐个 lint、short、gen-check，3、8 号另 knip、test-web，7、8 号另分份 |
| 日期 | 2026-11-16（实施、审查、修复第 1 轮、核对第 1 轮、修复第 2 轮与主持者的核对、两次合并 `main`） |
| 结论 | 没有 Important。Minor 5：标题 id 的测试分不出自动链接的 `Label` 与 `URL`（P3S1A-M1）；S1-27 的替身收 `"/"`，设计点名的那一格抓不到（P3S1A-M2）；提交在调用的期限之前超过 `commit_timeout` 的答 "the server failed"，设计 3.7 的"请求的期限也已过"是推断（P3S1B-M1）；`credentialOf` 的 PAT 一支没有测试守（P3S1C-M1）；偏离 7 的理由不成立（P3S1C-M2）。Nit 6。三位都确认：代码照设计写对了；`searchFold` 的调用、不用 `strings.Map`、`render_test` 用 Markdown 链接三条偏离成立；交错在去掉快照与降成 READ COMMITTED 时五格都红；并发下两张表的不变量成立；每个提交单独为绿。修复第 1 轮 6 个提交；核对第 1 轮三位的旧发现都解决，新的 0 Important、0 Minor、4 条 Nit；修复第 2 轮 4 个提交补上，由主持者核对 |

## 审查的发现与处置

编号照审查者的报告。审查时的代码位置是 `2200b5d6` 的。修复照主持者的说明（`p3s1/fix-1.md`、`fix-2.md`）在分支上加提交，普通推送、不改写已推的历史，每个单独为绿；文档的部分在实施者的草稿里改，落在本提交。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P3S1A-M1 | 中（测试） | 标题 id 的新格只用 `https://a.example`，这时 goldmark 的 `URL` 与 `Label` 相同：`PlainText` 写 `URL` 的变异（M-b）在 `platform/markdown` 全绿。用户碰得到：linkify 的 `www.` 链接 `URL` 前面加 `http://`，`# Go www.example.com now` 的 id 会是 `nw-go-httpwwwexamplecom-now`，锚点算出 `nw-go-wwwexamplecom-now` | 照 A（`07da4f76`）：`# Go www.example.com now` → `nw-go-wwwexamplecom-now` 写死，进 `TestHeadingsGetPrefixedUniqueIDs`、`TestHeadingIDIsAHeadingsID`，`render_test` 与 `obsidian/view_test` 各一格（主持者另要；这两格同一文档里有标题也有链接）；变异 a-m1-url 让四个测试红。核对 A：解决 |
| P3S1A-M2 | 中（测试） | `linking/app` 的替身 `paths.Check`（`backlinks_test.go:60-66`）只拒 `""`、`//`、NUL，收下真的 `Check`（`ParsePath`）会拒的 `"/"` 与尾随的 `/`：设计 7.3 说 S1-27 由 "the root" 一格抓到，实际只由 `TestListIndexJudgesInTurn` 的调用次序抓到；整个程序里也没有 `index {parent:"/"}` 成功的格 | 照 A（`8f30b563`，只改测试）：替身拒尾随的 `/`；改之前 `linking/app` 全绿，没有测试靠它收。S1-27 由 "the root"、"type and parent"、"a notebook hidden" 三格抓到。核对 A：解决，另提 `"/"` 的码（A-C1-N1，"核对"） |
| P3S1A-N1 | 细节（报告） | 实施报告说 `TestBlankIsUnicodesDerivation` 的总数"版本变了跳过"，代码是失败 | 照实改：有意的绊线，Unicode 换版本时照新版本的文件核对总数。核对 A：解决 |
| P3S1A-N2 | 细节（文档稿） | "标题 id 的新格在没改的代码上红"只对一半：`render_test`、`view_test` 两格守锚点一侧，没改时也绿 | 照实写（P3 设计 3.21 第 3 条、10.1）。核对 A：解决 |
| P3S1B-M1 | 中（行为） | 带标记的"可能已保存"只在提交跨过调用的期限时出现。提交的时限从提交开始算（`tx.go:219-223`），默认 2 秒短于请求的 15 秒：提交先超时、调用的期限还没到时 `ProblemOf` 归为 `FaultUnknown`（`apierrors.go:278` 要调用的 ctx 过期），答 "the server failed"，结局一样未知；不带 `base_version` 的 `edit` 照它重试可能重复追加。overlay 探针核实了两种情形（`review/b/probe/commit_probe.log`）。不是回归；设计 3.7"这时请求的期限也已过"是推断 | 决定 483（主持者取甲，`ac30ec3d`）：`toolError` 与 `methodError` 的 `FaultUnknown` 一支也认标记（`committing`、`failedText`），带标记的答 `the server failed as its changes were being saved, so they may or may not have been; read the notebook to see whether they were before retrying it`；码照旧 `internal_error`，日志照旧 error，REST 不变；`answer_test.go` 与 `prompt_bounds_test.go` 各一格整句比对，修之前都红（`fix1/logs/b-m1-before.txt`）。设计 3.7 照实改（3.21 第 11 条）。核对 B：解决，另提注释（B-C1-N1） |
| P3S1B-N1 | 细节（注释） | `tx.go` 的注释把每个带标记的提交失败都说成结局未知；标记加在读写事务的每一种 `COMMIT` 失败上，连接在送出 `COMMIT` 之前就断了的（(a) 格）结局是回滚 | 照 B 改（`ac30ec3d`）。核对 B：解决 |
| P3S1C-M1 | 中（测试） | 去掉 `credentialOf` 的 PAT 一支（全实例的 PAT 落到零值的会话凭证、共用一个）在事件流的三个单元包与九个整个程序的测试下都活下来。这样回归之后第 5 条 PAT 的流顶掉别的账户的，`hub.go:157` 减的是开者的账户，被顶掉的账户的数永远多一，累到 64 再开就 429 | 照 C（`1d7b471d`）：`TestEachTokenIsACredentialOfItsOwn`（alice 两个 PAT、bob 一个、alice 一个会话，开在第一个 PAT 之前，"按别的东西分凭证"会先顶掉它们）；照 C 的可选改法，顶掉时减被顶掉那一条自己的账户（`oldest.userID`，同 `close`），`TestASupersededStreamLeavesItsOwnAccount` 守它（决定 484）。变异 c-m1-pat、c-m1-opener 都红；去掉 `job` 一支等价（"反向对照"）。核对 C：解决，另提 0 项（C-C1-N2） |
| P3S1C-M2 | 中（文档稿） | 偏离 7 说"同一会话里还没关掉的一条会先被顶掉，第 1 条就不是最老的"：沿代码走，`settle` 留下 k 条（0–4）还算着的旧流时，第 5 次打开仍顶掉五条里的第 1 条；照设计用团队的令牌的探针 `-race` 下 9 次绿 | 主持者定：测试不动（新的登录更隔离），理由照 C 的探针改写（3.21 第 7 条）。核对 C：解决 |
| P3S1C-N1 | 细节 | `make help` 里第七份的说明没有两个新测试 | 加上（`aab8244b`）。核对 C：解决 |
| P3S1C-N2 | 细节（文档稿） | 第 10 节草稿的变异条数把 x1–x11 算了两次；第 4 节的表少 x11 | 写成"S1-1 到 S1-34 的 45 条与 x1–x11，共 56 条"（10.1）；表补 x11。核对 C：解决 |
| P3S1C-N3 | 细节（注释） | 网页两处注释说 `Retry-After` 只来自 503 `not_ready`；S1 之后打开流也会答 429，网页照样等它（`api.ts:61`、`:70`） | 决定 485：只改注释（`1f1d3d11`），行为不改，设计 4.1 的"不改"指行为。核对 C：解决，另提括号窄了（C-C1-N1） |

## 主持者的决定

负责人可以改判；写进 [00 号文档](../00-M12-design.md)第 11 节"2026-11-16：P3/S1 的决定 483–486"：

- **483 MCP 的 `FaultUnknown` 一支也认 `TxCommitFailed` 的标记**（P3S1B-M1，取甲；P3 设计 3.21 第 11 条）：带标记的答 `the server failed as its changes were being saved, so they may or may not have been; read the notebook to see whether they were before retrying it`，不带的照旧 `the server failed`；码照旧 `internal_error`，日志照旧，REST 不变。理由：默认 `commit_timeout` 2 秒短于 `request_timeout` 15 秒，提交先超时、调用的期限未到是更常见的一种，旧的答复让 agent 重试，不带 `base_version` 的 `edit` 可能重复追加。设计 3.7"这时请求的期限也已过"一句照实改。没有取乙（只改设计的文字、在已知限制里写一句）：那样更常见的一种仍答 "the server failed"。
- **484 事件流的 hub 顶掉一条流时减它自己账户的数（`oldest.userID`），减到 0 就删掉这一项，与关闭共用 `uncount`**（P3S1C-M1 的可选改法，核对 C 的 C1-N2；3.21 第 12 条）：凭证跨了账户的回归下数仍对、表不留 0 项。今天凭证与账户一一对应，正常时两种写法结果相同；顶掉的 `debug` 一行仍记开者的 `user_id`。
- **485 网页的事件流只改注释**（P3S1C-N3，核对 C 的 C1-N1；3.21 第 14 条）：503 `not_ready` 与 429 `rate_limited`（请求的限流，或账户到了流数的上限）都带 `Retry-After`、照它等；行为不改。P3 设计 4.1"网页不改"指的是行为。
- **486 只补核对的 Nit、碰一处很小的生产代码的修复轮，由主持者核对**（修复第 2 轮）：主持者读 diff、看变异与分支 CI，本地不跑 `make test` 与 `make e2e`，分支 CI 跑全套（02-workflow 3.4 的小轮；`main` 上合并之后照常两次 CI）。

修复说明里另两处（不编号）：A-M2 的替身照真的 `Check` 拒尾随的 `/`（只改测试）；C-M2 测试不动、只改理由。

## 实施中定的

实施报告第 7、8 节与 P3 设计 3.21 第 1–10 条（三位审查者各在自己的范围里判过；实施者没有要编号的产品决定）：

- 切罗基文的一步保留不合法的字节，不用 `strings.Map`（3.21 第 2 条）。决定 365 的"用标准库的 `strings.Map`"随之加括注。审查 A 的 M-a 证实：换成 `strings.Map`，`TestTitleKeysKeepASCIIAndUnicodeBoundaries` 红。
- `searchFold` 的调用多一处，`grep_tables.go` 重新生成（第 1 条）。
- `render_test.go` 那一格用 Markdown 链接，维基链接的一格在 `obsidian/view_test.go`（第 3 条）；`IndexKeyer` 的两格写成新测试（第 4 条）。
- 快照碰到已有的单元测试：写死的调用表加后缀，否定断言改用 `calledAtAll`；替身的快照拒绝嵌套（第 5 条）。审查 B 逐个找过剩下的用法，没有一处变得恒真。
- `TestVirtualRead/no_nested` 改数"这次读多出的一个"（第 6 条），审查 B 判没有削弱。
- 流数的整个程序测试用一次新的登录（第 7 条，理由照 P3S1C-M2 改写）；hub 的测试另起文件、`export_test.go` 导出两张表的长度（第 8 条）。
- `deadlineText` 按结构认标记（`interface{ TxCommitFailed() bool }`），不引 `postgres` 包，同设计 3.7。
- 分份脚本"去掉 `$$`"不会失败，改用"只加进 `EIGHTH`""从 `SEVENTH` 去掉"两个对照（第 9 条）；处理器的 429 一格先看状态（第 10 条）。
- 自己加的格（实施报告第 8 节）：`TestBlankIsUnicodesDerivation` 另断言 Unicode 17.0.0 下的总数（默认可忽略的码位 4174 个，其中 `White_Space` 25 个），版本变了就红，是有意的绊线；`TestSearchFoldIsTheTitleKeyNow` 与 `TestTitleKeyIsStable` 另逐码位跑；名字的格比设计多；`TestTheHubForgetsWhatClosed`；`TestTheRefusalSaysItsBound`；被顶掉的那一条留着顶掉之前送到的事件；变异 x1–x11。

## 接受与推后的

- **接受**：
  - S1-24、S1-29、S1-30 的整个程序那一格照绿：快照在它的第一条语句开始时冻结，第一条就是被锁挡住的读，晚开始的快照仍在 R 之前（3.8.2）；单元的那一格抓到。
  - b-m1-mark、c-m1-job 等价（证据在"反向对照"）。b-m1-mark 是"以后有实现者答 `false` 时不再等价"的一种。
  - S1-16 红在"全关之后两张表空"，不在设计写的"关掉之后能开"：这个写法账户照减，所以能开（实施报告第 4 节）。
  - `pgtest.WaitForTableLockWaits` 数这个库里任何在等 `page_revisions` 的后端：有后台读也在等时，测试会在读开始之前写 R，结果只会是假红、不会是假绿；四次运行都稳定，没有复现（审查 B 的未核实的怀疑）。
  - "提交落下"的那一半结局（过了提交点、或取消送不到）没有造过，同设计审查。
  - 顶掉一个已因别的原因重置、处理器还没关的流：它照旧以原来的原因结束，计数立刻去掉；打开在 `seenBy` 失败时那一次已顶掉的旧流不恢复，只多一次重连（审查 C）。
  - 第一次分支 CI 的 e2e 8.6 分钟（`main` 6.1 分钟），最慢的故事都慢 1.4–2 倍：之后三次 6.1–7.4 分钟，没有再现；审查 C 没有找到 S1 让它变慢的路径（事件流的上限在 e2e 里碰不到）。
  - 修复第 2 轮本地没跑 `make test` 与 `make e2e`（决定 486）。
- **推后、交出去的**：
  - P5/S3 发布说明的已知限制（P3 设计第 9 节，照 S1 的探针改写）：切罗基文的旧键在升级之后、`reindex` 之前，大写的标题找不到，同名或只差大小写的兄弟页都建得成；标题 id 变了；U+2800 单独作名字仍通过；一个凭证至多 4 条、一个账户至多 64 条事件流。
  - P4/S3：快照每次多一个事务与一条连接的占用，hub 两张表的常数时间维护（P3 设计第 9 节，实施报告第 13 节核对过）。
  - 合并之后两次 `main`：第七份与竞态一份的 Test 一步照 06-environment 第 4 节记进 00 号文档第 13 节；第七份这几次都没落在 EPYC 7763 上。
  - S3 也改 `Makefile` 的 `SEVENTH` 与计数：后合并的一方 `git merge main` 时合 `Makefile`，照 `go test -race -list` 重数。

## 反向对照

工具：`docs/v0.1/claude_to_codex/tools/mut.py` 的拷贝 `p3s1/tools/mutx.py`（只多打出每一行 `--- FAIL` 与每条变异的完整输出），改一处、跑命令、从 `.mutbak` 复原。每张表在头上干净的临时 worktree 里、经 `heavy.sh` 跑，先在没改的树上把它的每个命令跑一遍为绿；之后 0 处改动、0 个 `*.mutbak`。

**实施时**（`2200b5d6`；轻的 40 条，单元的包；重的 16 条，要库的包与整个程序）：

| # | 变异 | 由什么抓到 |
|---|---|---|
| S1-1 | `TitleKey` 不经 `cherokeeCapitals` | `TestTitleKeyIsStable`、`TestTitleKeysMatchAcrossCaseAndNormalization`、`TestSearchFoldIsTheTitleKeyNow`，另 grep 的四个 |
| S1-2 | `TitleKeyContext` 长的一支不换 | `TestLintTitleKeyContext`（`cherokee`） |
| S1-3 | `IndexKeyer.Key` 不换 | `TestAnIndexKeyIsCherokeesCapitals`、`TestAnIndexKeyerTakesEachKeyWithItsBuffers`（差分） |
| S1-4 | 只换 U+AB70–ABBF | `TestTitleKeyIsStable`（`ᏸ`）等 |
| S1-5、S1-6 | `PlainText` 不写自动链接、不写原始 HTML | `TestHeadingIDIsAHeadingsID`、`TestHeadingsGetPrefixedUniqueIDs` |
| S1-7 | `Tags` 照列 `/`（改生成的常量） | `TestTheTagsOfANotebook` |
| S1-8–S1-11、S1-25、S1-31 | `Blank` 恒假、漏 `Other_Default_Ignorable_Code_Point`、U+2800 也算、先判长度；`CleanClientName` 不判空；不减 `Prepended_Concatenation_Mark` | `TestCheckName`、`TestCleanClientName`、`TestCheckUserPatch`、逐码位的比对，各在对应的格 |
| S1-12 | 凭证只看 `UserID` | `TestACredentialsFifthStreamSupersedesItsOldest`、`TestClientsThatReconnectAtOnceComeToRest`（1017 次打开）等 |
| S1-13 | 顶掉的算到它关掉 | `TestASupersededStreamLeavesTheCountsAtOnce` 等 |
| S1-14 | 凭证满时先看账户 | `TestAFullAccountsCredentialSupersedesItsOwn` |
| S1-15 | 顶掉最新的 | hub 的三个；整个程序 `TestASignInsFifthStreamSupersedesItsOldest` |
| S1-16 | `close` 不从 `held` 去掉（账户照减） | `TestAnAccountsSixtyFifthStreamIsRefused`、`TestTheHubForgetsWhatClosed`：红在"全关之后两张表空" |
| S1-17、S1-26 | 不带 `RetryDelay`；账户的上限乘 1000 | 处理器的 429 一格、`TestTheRefusalSaysItsBound`；`TestAnAccountsSixtyFifthStreamIsRefused` 等 |
| S1-18、S1-19 | `"/"` 经 `ParsePath`；`""` 当作整本 | `path_history_test.go` 的两格；整个程序 `TestTheMCPHistoryIsRESTs`、`TestTheMCPToolsErrorsCarryTheirCodes` |
| S1-20–S1-23 | 四个用例各去掉快照 | 单元 `TestTheLogAndAChangesetReadInOneSnapshot` 的对应格与调用表；整个程序的对应格 |
| S1-24、S1-29、S1-30 | 快照只包 `detail`；`AtPath` 的快照从 `page` 才开始；`alivePaths` 在快照之后 | 单元的格；整个程序的照绿（3.8.2，"接受"） |
| S1-27 | `index` 的 `parent:"/"` 也经 `Check` | 当时只有 `TestListIndexJudgesInTurn` 的调用次序（替身收 `"/"`，P3S1A-M2） |
| S1-28、S1-34 | 超时的文字照旧；`deadlineText` 不看标记 | `TestTheToolErrorsAreTheTables`、`TestPromptErrorsAndLogs`，整个程序 `TestTheMCPCallRunsOutOfTime`；`the deadline at a commit` |
| S1-32、S1-33 | 读写事务的提交错误不带标记；只读的也带 | `TestACommitThatFailsIsMarked` 的 (a)、(b)；(c) |
| x1–x11 | `Blank` 不减 U+FFF9–FFFB、U+13430–13440（只有逐码位的比对抓到）；不算空白；先判控制字符；`close` 不删空项（两种）；原因写成 `overflow`（单元、整个程序）；`MaxAccountStreams = 65`；新的一条加在切片头上；`cherokeeCapitals` 只换第一个；`commitError` 没有 `Unwrap` | 都抓到（实施报告第 4 节） |

写代码时的一条发现：8 号提交之前在拷贝上跑事件流的 12 条，S1-12 与 x7 两条把 `events/adapter/http` 挂到 10 分钟：处理器的 429 一格答 200 时 `io.ReadAll` 一条开着的流。改为先看状态、读第一帧就失败（3.21 第 10 条），之后两条都在 1 秒内红。

**修复第 1 轮**（`1f1d3d11`，23 条：审查要的 8 条与修复碰到的 S1 原有的 15 条）：

| # | 变异 | 由什么抓到 |
|---|---|---|
| b-m1-text | `failedText` 不看标记 | `TestTheToolErrorsAreTheTables/a fault at a commit`、`TestPromptErrorsAndLogs/unknown_commit` |
| b-m1-tool、b-m1-method | `toolError`、`methodError` 的 `FaultUnknown` 一支照旧 `internalFailed` | 上面两格各一 |
| b-m1-mark | `committing` 只看类型、不看 `TxCommitFailed()` 的值 | **活下来，等价**。证据：仓库里实现 `TxCommitFailed() bool` 的只有 `postgres.commitError`（`tx.go:203`）与测试的替身 `failedCommit`（`answer_test.go:361`），都答 `true`；另两处是接口字面量（`errors.go:98`、`tx_test.go:378`）。核对 A、B 各自查过，B 以 overlay 跑过、两格照绿。带 bool 是 `shared.TxNotBegun` 那几个标记的写法 |
| c-m1-pat | 去掉 `credentialOf` 的 PAT 一支 | `TestEachTokenIsACredentialOfItsOwn`（修之前活下来，P3S1C-M1） |
| c-m1-opener | 顶掉时照旧减开者的账户 | `TestASupersededStreamLeavesItsOwnAccount` |
| c-m1-job | 去掉 `credentialOf` 的 `job` 一支 | **活下来，等价**。证据：HTTP 的认证只造会话（`identity/app/authenticate.go:86`）与 PAT（`:112`）的 actor，任务的 actor 只在 transfer 的后台里造、从不经 `OpenStream`（审查 C、核对 C） |
| a-m1-url | `PlainText` 写自动链接的 `URL` | `TestHeadingIDIsAHeadingsID`、`TestHeadingsGetPrefixedUniqueIDs`、`TestALinkToAnAnchorAloneLeadsToItsHeading`、`TestLinksLeadWhereTheyResolve` 的 `www.` 一格 |
| S1 原有的 15 条 | hub 的 S1-12–S1-17、S1-26、x4–x8，`mcpserver` 的 S1-28、S1-34，S1-27 | 全抓到；S1-12、S1-15、x6、x8 另有两个新测试红；S1-27 现在由 "the root"、"type and parent"、"a notebook hidden" 三格抓到 |

**修复第 2 轮**（`a7723228`，16 条：hub 的 S1-12–S1-17、S1-26、x4–x8、c-m1-pat、c-m1-opener，S1-27，与新的一条）全抓到：新的 c-c1-n2（顶掉时不经 `uncount`，不删 0 项）让 `TestASupersededStreamLeavesItsOwnAccount` 红（`2 accounts counted`、全关之后 `1 accounts`）；S1-13、x5、c-m1-opener 的文字照新代码改成 `h.uncount(…)`。

**审查者另加的**：A 的 M-a（`cherokeeCapitals` 换成 `strings.Map`：`TestTitleKeysKeepASCIIAndUnicodeBoundaries` 红，偏离的理由成立）、M-b（写 `URL`：活下来，P3S1A-M1）、替身拒尾随的 `/` 之后的 S1-27（三格红）；B 的 B1（四处快照都去掉：五格红）、B2（快照降成 READ COMMITTED：五格也都红，测试要的是快照、不只是一个事务）、B3（没有 `commitError`：(a)、(b) 红，(c) 绿）、快照里再开快照（overlay，四格红）；C 的 mut-pat（活下来，P3S1C-M1），核对第 1 轮的四种（去掉 PAT 一支、PAT 按 `UserID` 当 id、顶掉时减开者、顶掉时不减：都红），并发探针的对照（去掉顶掉时的减一：立刻红）。

## 核对

**核对第 1 轮**（2026-11-16）：三位各把自己的工作树挪到 `1f1d3d11`，之后干净、没有 `*.mutbak` 与探针。三位的旧发现都解决，都没有 Important、Minor；新的 4 条 Nit，主持者定照审查者改，在修复第 2 轮：

- **A**：M1、M2、N1、N2 解决。自己跑 a-m1-url 与 S1-27：没改时绿，变异之后四个测试、三格红；替身拒的集合与真的 `ParsePath(…, false)` 一致（`"/"`、`a/`、`a.md/`、`//` 拒，`/a.md` 收）。新的：
  - **P3S1A-C1-N1**：替身对 `"/"` 答 `invalid_format`，真的去掉开头的 `/` 剩空串、答 `required`；今天没有测试断言这个码。修复第 2 轮（`1812134f`）：`"/"` 单独一个答 `required` `is required`，别的以 `/` 结尾的照旧；S1-27 重跑仍由那三格抓到。
- **B**：M1、N1 解决。第一轮的探针在新头上重跑：期限未到答新的一句，已过答 `internalDeadlineSaving`；三条变异自己跑过都红；b-m1-mark 的等价理由成立。新的：
  - **P3S1B-C1-N1**：`internalFailedSaving` 的注释说这一支是"超过 `database.commit_timeout`、短于 `server.request_timeout`"：比代码窄（`failedText` 认任何带标记的提交失败），也把默认值写成了不变式（`config/validate.go:92-104` 不要求它）。修复第 2 轮（`a9f8dd95`）照 B 改写，只改注释。
- **C**：M1、M2、N1–N3 解决。四种变异都红；竞态的并发探针 `-count=20` 照过；事件流三个包竞态 `-count=3` 绿。新的：
  - **P3S1C-C1-N1**：网页注释把打开流的 429 只说成账户的上限；打开流在限流中算一次请求，桶空了同样答 429 带 `Retry-After`。修复第 2 轮（`a7723228`）写成两种起因，只改注释（决定 485）。
  - **P3S1C-C1-N2**：顶掉时减 `oldest.userID` 不删为 0 的项；在 `TestASupersededStreamLeavesItsOwnAccount` 代表的回归下（凭证跨了账户），全关之后账户表留一个 0（探针 `accounts map[<bob>:0]`）。修复第 2 轮（`7c49a816`）：顶掉与 `close` 共用 `uncount`（决定 484）；测试另把 bob 顶到 0、再全关，断言 `HubCounts` 是 (0, 0)。照说明的字面只在原来那一段末尾断言，没改的代码也绿（bob 没被顶到 0），所以实施者多加了把 bob 顶到 0 的一段，新断言在没改的代码上红（`fix2/logs/c-c1-n2-before.txt`）。

**修复核对第 2 轮（主持者）**：这一轮只改测试、注释与一个小函数（02-workflow 3.4 的小轮，决定 486）。主持者读了 `1f1d3d11..a7723228` 的 diff，与说明的处置一致；新断言在没改的代码上红；16 条变异全红；本地没跑 `make test` 与 `make e2e`，分支 CI（run 43801996056）跑全套、19 项全绿。没有新的发现。

## 实测

本机 8 核 arm64，经 `heavy.sh`（`p3s1/tools/timing.sh`，头 `2200b5d6`，开始时 1 分钟的负载 9.8）：

| 测试 | `go test -run '^Name$' -count=1 -v` | 另加 `-race` | 设计推断 |
|---|---|---|---|
| `TestTheLogReadsOneMomentAcrossARevert` | 1.46 秒（五格各 0.04–0.05 秒，其余是起应用与建团队） | 2.39 秒（各 0.06–0.08） | 1–2 秒 |
| `TestASignInsFifthStreamSupersedesItsOldest` | 1.53 秒 | 2.19 秒（8 号提交之前一次 0.87 秒；审查 C 0.89 秒） | 约 1 秒 |

第七份因此竞态下约多 4.6 秒，在 7.5 的"约 4–6 秒"之内。合并 `main` 之后（`d1e90929`）竞态下各一次：2.51、1.23 秒。单元：`TestSearchFoldIsTheTitleKeyNow` 0.29 秒（逐码位）；`TestBlankIsUnicodesDerivation` 与 hub 的测试都在 0.1 秒以内；`TestAStreamThatCannotOpenIsAProblem` 竞态下 2.2 秒（多出的 64 条流）。

## 门禁与 CI

- **逐个提交**（实施者在干净的临时 worktree 里逐个 `checkout --detach`，整个经 `heavy.sh`，先 `pnpm install --frozen-lockfile --prefer-offline`）：实施的 8 个、修复第 1 轮的 6 个、修复第 2 轮的 4 个，每个 build、vet、`make lint`（两处 `0 issues`）、`make test-go-short`（94 个包）、`gen-check`、knip 都是 0；改了 `web/` 的 `4e813da1`、`2200b5d6`、`1f1d3d11`、`a7723228` 另 `make test-web`（4537 个）为 0；之后已跟踪的改动都是 0。审查 C 在自己的工作树里把实施的 8 个逐个再跑（lint、short、gen-check 都 0；3、8 号 knip、test-web 0；7、8 号分份 555、556 个，每个恰在一份）。
- **合并之前的门禁**（每一步单独经 `heavy.sh`，之前 `df -h /`）：

  | 步 | `2200b5d6` | 修复第 1 轮 `1f1d3d11` | 修复第 2 轮 `a7723228` | 合并 `main` 的 `d1e90929` |
  |---|---|---|---|---|
  | `make lint gen-check knip build` | 0（10 秒） | 0（10 秒） | 0（11 秒） | 0（800 秒，多是排在别人的运行后面） |
  | `make test`（含组合根八份与竞态一份） | 0（3538 秒，含等锁；185 个包 `ok`，没有 `FAIL`；web 4537 个） | 0（3536 秒，同上） | 不跑（决定 486） | 不跑 |
  | `make test-go-short` | 0（40 秒） | 0（40 秒） | 0（48 秒） | 0（51 秒） |
  | `make e2e` | 0（154 秒；577 个，576 个通过、1 个照旧跳过：WebKit 的 A15 第 3 格） | 0（155 秒，同上） | 不跑（决定 486） | 不跑 |
  | 3.19 的分份脚本 | 556 个，每个恰在一份 | 同 | — | 556 个（S2 改名的只在第八份） |

  `make test` 的日志里缩进的 `--- FAIL: TestWikiCostSamplesWaitForFinished/sample-failure` 是那个测试自己起的子进程里预期的失败，顶层 `--- PASS`。修复第 2 轮另跑事件流三个包竞态 `-count=3`、`linking/app` 与 `mcpserver`（53 秒，都 `ok`）。
- **分支的 CI**（`ci.yml` 每次 19 项第一次 attempt 全绿，没有重跑，没有碰到已知的偶发；Test 与 E2E 是那一步的用时）：

  | 作业 | `2200b5d6`，run 43778049280 | `1f1d3d11`，run 43793058599 | `a7723228`，run 43801996056 | `d1e90929`，run 43814516991 |
  |---|---|---|---|---|
  | bootstrap-1 | 4:47，Xeon 6975P-C | 6:11，Xeon 8573C | 4:25，EPYC 9V45 | 4:55 |
  | bootstrap-2 | 4:13，Xeon 6975P-C | 4:46，Xeon 8573C | 4:07，Xeon 6975P-C | 4:13 |
  | bootstrap-3 | 6:16，EPYC 7763 | 4:26，EPYC 9V45 | 5:21，Xeon 6973P-C | 5:07 |
  | bootstrap-4 | 5:34，EPYC 9V74 | 5:38，EPYC 9V74 | 4:57，Xeon 6975P-C | 6:32 |
  | bootstrap-5 | 5:18，EPYC 9V74 | 4:07，Xeon 6973P-C | 4:55，Xeon 8573C | 4:06 |
  | bootstrap-6 | 4:34，Xeon 6975P-C | 4:33，Xeon 6975P-C | 6:01，EPYC 7763 | 4:26 |
  | bootstrap-7（S1 的两个） | 4:03，EPYC 9V45 | 4:29，EPYC 9V74 | 4:00，Xeon 6975P-C | 3:54，Xeon 6975P-C |
  | bootstrap-8 | 4:25，Xeon 6975P-C | 4:14，Xeon 6975P-C | 4:15，Xeon 6975P-C | 5:24，Xeon 8573C |
  | race | 8:50，Xeon 6975P-C | 10:57，EPYC 9V74 | 8:23，EPYC 9V45 | 12:33，Xeon 8573C |
  | e2e | E2E 一步 8:52（Playwright 8.6 分钟） | 6:50（6.7 分钟） | 7:34（7.4 分钟） | 6:17（6.1 分钟） |

  各份都在 450 秒之下。第七份对照 `main` 的 `99def961` 263 秒、`1ba6c7f6` 303 秒（7.5 的平均 279.5 秒），S1 的两个新测试看不出来；包的用时随 runner 的 CPU 差得多，不拿来与 `main` 比。第一次的 e2e 比 `main` 整体慢，之后没有再现（"接受"）。审查 C 读过 43778049280、核对 C 读过 43793058599，19 项都 `success`。
- **本提交自己的分支 CI**（与 `892dd827` 一起推）：`79e3b884` 的 run 43819949305，第 1 次 attempt 全绿；第七份 4:26、竞态一份 11:12，E2E 一步 7:34。合并 `f3e5a6de` 之后第一次 `main`：run 43823936850，第 1 次 attempt 全绿；第七份 4:09、竞态一份 9:10，E2E 一步 6:56。
- 合并之后两次 `main` 的各份照 06-environment 第 4 节记进 00 号文档第 13 节。

## 分份

3.19 的脚本（实施者的 `shards.py`，`go test -race -list . ./internal/bootstrap` 的列表，照八份的 `-run`/`-skip` 判）：7 号提交之后 555 个顶层，145/50/119/120/60/15/31/15；8 号之后与头上 556 个，145/50/119/120/60/15/32/15，每个恰在一份，`TestTheLogReadsOneMomentAcrossARevert`（原在第三份）与 `TestASignInsFifthStreamSupersedesItsOldest`（原在第一份）只在第七份。审查 C 的 `shards_c.py` 从 `make -s -n test-go-bootstrap-N` 取 make 展开后的 `-run`/`-skip`，结果相同（头上另有它自己的探针一个，落在第四份）。反向对照：只加进 `EIGHTH` 报在第一、第八两份，从 `SEVENTH` 去掉报在第一份，都 rc=1；去掉 `$$` 照过（3.21 第 9 条）。合并 `main` 之后（`d1e90929`）仍是 556 个、八份的数不变，S2 改名的 `TestRevertingARollbackPutsBackWhatTheAgentLeft` 只在第八份。`Makefile` 的计数注释照实改（"M12/P3/S1 之后 556 个 145/50/119/120/60/15/32/15"），`make help` 的第七份写出两个新测试。

## 环境

实施、审查与核对各在自己的工作树里（实施者 `nerve-wiki-m12-p3-s1`，审查者 `nerve-wiki-m12-p2-rev-{a,b,c}`），`TMPDIR`、`GOTMPDIR` 与原始日志在 `~/.cache` 下各自的目录，锁固定在 `/tmp`，用完删掉；探针（`zz_*_test.go`）从没提交。

- 实施者在 8 号提交之前想取消自己排在 `heavy.sh` 锁上的一次核对、在锁外直接跑，权限分类器拒了；照规矩没有换办法，让它排队跑完，多等了约一个小时（S2 的门禁在前）。之前一次结束自己挂住的变异测试进程没有被拒。8 号提交之前在拷贝上跑的那 12 条事件流的变异（只有 events 的单元测试）没有经 `heavy.sh`，之后的整张表都经它。
- 审查 A 有一次在含要库的包的模式上直接跑了 `go test`（`page/adapter/postgres` 与 `page`），没有经 `heavy.sh`；想结束它时权限分类器拒了，照规矩让它跑完（341 秒、112 秒），之后没有残留的容器。
- 根分区在修复第 1 轮的门禁时只有 15–17G 可用（`make test` 中间一度 14G，是别人的运行），`gates.sh` 每一项之前判了 15G 的线；共用的 Go 构建缓存没有动。
- P3/S2 与 P4 的审查、门禁同时在跑，`make test` 的用时含在锁上排队的时间。要留的汇总在设计工作目录的 `m12/p3s1/`（`logs/`、`tools/`、`fix1/`、`fix2/`、`mergemain/`、`review/a/`、`review/b/`、`review/c/`）。
