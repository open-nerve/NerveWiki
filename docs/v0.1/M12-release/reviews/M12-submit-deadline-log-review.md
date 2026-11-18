# M12 旁支 `m12-submit-deadline-log`（M11 的提交期限的测试照生产的先后读钟，断言精确值）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-submit-deadline-log`，从 `main` 的 `5e58e844`（P4/S5 的合并）开始（主持者的决定 564）。实施的 2 个提交，审查时在 `203fc064`：请求先到期的四格在 `WithTimeout` 之后读假时钟 `ad68ca6a`；告警行的 `timeout_ms` 断言精确值 `203fc064`（决定 565）。<br>修复第 1 轮在它之上 2 个提交：两个循环都在 `Execute` 的前一行读假时钟，COMMIT 两格也是 `1f94312b`（SDB-M1）；期望值用测试自己的时刻、行的时刻须等于它、`elapsed_ms` 精确、加回按毫秒的上界 `75c38196`（SDA-N1、N2，SDC-M1、N1）。<br>修复第 2 轮 1 个提交，头 `9845aa7e`：上界改为比较钟自己的读数（核对第 1 轮 A 的 SDA-M1）。<br>本提交之前把当时的 `main` 合进来一次。<br>到 `9845aa7e` 1 个文件（`server/internal/modules/wiki/app/submit_upgrade_test.go`），加 37 行、减 3 行；只改测试，不改产品 |
| 审查方式 | 实施者在工作树 `nerve-wiki-m12-submit-deadline` 里复现、量间隔、试改法，探针都用 `go test -overlay`。三位审查者并行，各在自己的分离的工作树里（A `nerve-wiki-m12-p4-rev-a`，B `nerve-wiki-m12-p4s5-rev-c`，C `nerve-wiki-m12-side-rev`，挪到 `203fc064`），重的都经 `heavy.sh`，不碰实施者的工作树：<br>A 产品一侧与断言：产品怎样算；"生产没有错"沿调用路径核到哪一步；精确的断言是不是照抄实现（变异 A1–A5、B1、B2、C1、C2）；每一格谁早；先后改回之后测试还看不看得出（D0、D2）。<br>B 原因：照本机 Go 1.27.4 标准库的 `uuid.go` 解失败日志里的 UUIDv7（`gh run view 44022740779 --attempt 1 --log-failed`，只读）；复现（2 ms 与 3270 µs 的探针）；COMMIT 两格的 60 ms 探针；整个 `server/` 里 Clock 端口与真实的期限混用的地方。<br>C 测试、提交与文档：两个提交各自为绿，`-race -count=200`；重跑报告的变异，另加 MC1–MC3、T1、T3 与探针 PE、PORD；后缀与说明照不照实；照 00-M12 第 7 节"旁支"与先例列文档的清单，costs-3 那一次记在哪里。<br>修复核对第 1 轮三位挪到 `75c38196`；第 2 轮只请 A，挪到 `9845aa7e`（决定 571） |
| 日期 | 2026-11-17–18（查因、实施、审查、修复两轮、修复核对两轮） |
| 结论 | Important 0。Minor 4：COMMIT 两格 50 ms 的期限也从造替身时算起（SDB-M1）；同样写法的清单漏了 grep（SDB-M2）；告警行的 `elapsed_ms` 不断言、记 0 照过（SDC-M1）；文档的清单漏了第一次分支 CI 的红，草稿停在第二个提交之前（SDC-M2）。Nit 8（SDA-N1、N2，SDB-N1–N3，SDC-N1–N3），Q 1（SDC-Q1）；分开是 A 0I 0M 2N，B 0I 2M 3N，C 0I 2M 3N 1Q。A 确认生产沿代码没有错、精确的断言不是照抄实现；B 确认原因的核实部分成立，并由 UUIDv7 定出停顿的位置与长度；C 确认两个提交各自为绿，精确值抓住了界放过的记 0 与少记 1 ms。代码一侧由修复第 1 轮 2 个提交改，文档一侧在本提交。修复核对第 1 轮：A 0I 1M 1N（SDA-M1：按毫秒的上界看不出读钟的先后被改回），B 0I 0M 1N，C 0I 0M 2N（SDB-N4 = SDC-N5，Nit 去重 3 条）；第一轮的都关上，SDB-N3、SDC-M2 的文档一半交本提交。SDA-M1 由修复第 2 轮 1 个提交改。修复核对第 2 轮（只请 A）：0I 0M 0N，SDA-M1、N3 关上 |

## 起因

- **`main` 的 CI**：P4/S5 合并之后第一次 `main`（`5e58e844`，只改网页与 e2e），run 44022740779 第一次 attempt，`server (short)` 一份红：

  ```
  --- FAIL: TestTheSubmissionsDeadline/the_request_expired (0.05s)
      submit_upgrade_test.go:569: logged [map[elapsed_ms:0 level:WARN msg:upgrade submission timed out … timeout_ms:53 …
  ```

  `:569` 是 `5e58e844` 上 `timeout_ms ≤ 50` 那一条断言（审查 B 核过）。主持者只重跑失败的作业，第二次 attempt 全绿（e2e 8:40）。
- **测试的来历**：`TestTheSubmissionsDeadline`、`≤ 50` 的界与 `newSubmitFake` 的取时都来自 M11/P2/S2 的 `74a3057a`（审查 C 以 `git log -S` 核过；[M11/P2 设计](../../M11-upgrade/02-P2-guardrails-gate-submit.md) 3.12 的测试一段，`:1041`），所以提交写 `(M11 story, found in M12)`。
- **这一次**：主持者读代码推断出原因（假时钟早于请求的期限），开旁支核实、修（决定 564）。
- **本支的第一次分支 CI**：`ad68ca6a` 的 run 44078438315 第一次 attempt 红在 `server (bootstrap-7)`，是 P3/S1 的交错，与本支无关，归等锁帮手的旁支 `m12-lockwait-by-statement`（决定 548）；只重跑失败的作业，为绿（SDC-M2；见"门禁与 CI"）。

## 定因

### 核实的

- **机制**（实施者与审查 A、B 读代码）：
  - 产品：`deadline` 取 `first.at + SubmitTimeout` 与请求的期限里早的那个（`submit_upgrade.go:104-107`）；告警行的 `timeout_ms` 是 `deadline.Sub(first.at).Milliseconds()`，向下取整（`:131`）。`first.at` 是第一个事务里读的钟，截到微秒；`UTC()` 与 `Truncate` 都去掉单调读数，所以这个差按墙上时钟算（审查 A）。
  - 测试：`first.at` 来自假时钟。`newSubmitFake` 造替身时就取 `time.Now().UTC()`（`5e58e844` 的 `submit_upgrade_test.go:93`、`:105`），之后才 `newSubmit` 与 `context.WithTimeout(ctx, 50ms)`（`:556`）。
  - 所以请求先到期的格记下的是 ⌊50 ms + 两次取时的间隔⌋，间隔到 1 ms 就过界。主持者的推断成立。
- **停顿的位置与长度**（审查 B 由失败日志里的 UUIDv7 定出；SDB-N1、N2）：
  - Go 标准库的 `uuid.NewV7`：前 48 位是毫秒，`rand_a` 的 12 位是 1/4096 毫秒（Go 1.27.4 的 `src/uuid/uuid.go:233-237`）。失败日志里五个 id 的时刻（05:17:21 之后）：

    | id | 在哪里造 | 时刻 |
    |---|---|---|
    | notebook | `newCheckFake` 的六个 id 里第一个（`check_upgrade_test.go:56-58`） | .287359375 |
    | workspace | 同上 | .287361328 |
    | upgrade | 同上，紧挨着之后读假时钟 | .287361572 |
    | user | `newSubmit` 的 actor，紧挨着之后是 `WithTimeout` | .290629639 |
    | submission | 第一个事务（`submit_upgrade.go:194`） | .290679932 |

  - notebook 与 user 两个 id 相隔 3.27 ms。两次取时相隔 **3.00–3.32 ms**：下界来自记下的 53（53 ≤ 50 + 间隔 < 54）；上界来自 id 圈出的窗口：假时钟读在 upgrade 的 id 之后，`WithTimeout` 在 user 与 submission 的 id 之间。
  - 停在 `newCheckFake` 造完 id 之后、`newSubmit` 造 actor 之前。窗口里只有几个一行的构造函数（`NewLint`、`NewCheckUpgrade`、`NewSubmitUpgrade`：`lint.go:19`、`check_upgrade.go:34`、`submit_upgrade.go:27`），停顿来自外面。
  - 期限落在 .340630–.340680，告警行记在 .340821，晚 0.14–0.19 ms，前后对得上。
- **四格同因**：请求先到期的四格（"the request expired"，`SubmitTimeout` 1 小时；"7.5 s in""as it may be taken over""past that"，`SubmitTimeout` 10 秒）请求的期限都是 50 ms，断言相同；CI 只碰上了第一格。
- **复现**（经 `heavy.sh`，探针都用 `go test -overlay`，工作树没动）：

  | 运行 | 谁 | 结果 |
  |---|---|---|
  | 改前，不加探针，`-count=200` | 实施者 | 200 次全绿 |
  | 改前，量间隔的探针（`ctx.Deadline() − f.at() − 50ms`），`-count=200` | 实施者 | 800 个样本：中位 5 µs，p99 39 µs，最大 216 µs；`-race` 下中位 10 µs，p99 65 µs，最大 104 µs；没有红的 |
  | 改前，`WithTimeout` 之前睡 2 ms，`-count=20` | 实施者，审查 B、C | 四格 20/20 红：实施者那一次 80 次都记 52；B、C 各一次，52 有 79 次、53 有 1 次（睡过了头）；"alive"与 COMMIT 两格 20/20 绿 |
  | 改前，`WithTimeout` 之前睡 3270 µs，`-count=20` | 审查 B | 四格 20/20 红：53 有 17 次，54 有 58 次，55 有 5 次；CI 的 53 复现得出，`Sleep` 常睡过头 |

  本机空闲时，间隔离 1 ms 差三个数量级以上，要靠探针才确定地红。
- **COMMIT 两格**（审查 B 的 SDB-M1）：它们没有请求的期限，`SubmitTimeout` 是 50 ms；产品拿 `first.at + SubmitTimeout` 做 `detached` 的真实的 ctx 期限（`submit_upgrade.go:108`），起点同样是早读的假时钟。测试要的是检查与第二个事务里的授权都在期限之前做完、只有 COMMIT 等过期限，假的授权在 ctx 到期后答错误。`203fc064` 上在 `Execute` 之前睡 60 ms，两格 5/5 红（答 `server_busy`，期望 `""` 与 `wiki.upgrade_not_ready`）；睡后照生产的先后再读一次钟，5/5 绿。文档里没有它们红过的记录（审查 B）。
- **生产没有错**（审查 A 沿代码核到"读钟之前期限已定"；没有跑整个程序，组合根也不断言 `timeout_ms`）：
  - 生产的钟是 `clock.System`（`bootstrap/wire.go:262` → `wiki/lint.go:186`），`Now` 是 `time.Now().UTC().Truncate(µs)`（`platform/clock/clock.go:13-15`）；第一个事务在两把锁与授权之后才读它（`submit_upgrade.go:97-99` → `:165` → `start_upgrade.go:78`）。
  - API：`httpserver/api.go:297-311` 的 `deadline` 中间件在处理器之前 `WithTimeout`（`:306`），处理器原样传 ctx（`wiki/adapter/http/upgrade.go:69-70` → `wiki/lint.go:226-227`）。
  - MCP：路由经 `api.Wrap`（`bootstrap/mcp.go:144`）先过上面的 `deadline`；`mcpserver/call.go:47-53` 的 `outer` 取 `read + requestTimeout`（`read` 是 `server.go:201` 的 `time.Now()`）与请求自己的期限里早的那个，`WithDeadline`；工具原样传 ctx（`wiki/adapter/mcp/upgrade.go:35-36`）。
  - 所以生产里 `timeout_ms` 不超过请求的超时：`first.at` 截到微秒，最多早不到 1 µs，向下取整吃掉。墙上时钟往回跳时不成立，见"接受与推后的"。

### 推断

- runner 在 `newCheckFake` 与 `newSubmit` 之间停了 3 ms 多的原因：GC、调度或虚拟机被抢，从日志里定不了。旁证：`ci.yml:136` 写着 short 一份的各包同时跑，一个 2 秒的界到过 2.6 秒；窗口里只有几个一行的构造函数（审查 B 的 SDB-N2）。
- 只红过一次，与本机的间隔分布相符：要一次少见的停顿，才到得了 1 ms。

## 同样写法的清单

查的是"一个值里混着 Clock 端口与真实的期限，又断言这个值"。实施报告第 4 节原写"只有 submit 一处混用"、COMMIT 两格"没有"，由审查 B 改正（SDB-M2、M1）。

| 位置 | 有没有同样的问题 | 依据 |
|---|---|---|
| `submit_upgrade_test.go` `TestTheSubmissionsDeadline` 请求先到期的四格 | **有**，`ad68ca6a` 改 | 见"定因" |
| 同上，COMMIT 两格 | **有**（SDB-M1），`1f94312b` 改 | 50 ms 的 `SubmitTimeout` 是真实的 ctx 期限，起点是早读的假时钟 |
| 同上，"the request alive" | 没有 | 没有请求的期限，`deadline` 全由假时钟算，恰 50；`Execute` 之前睡 60 ms 照绿（审查 B） |
| `submit_upgrade_test.go` `TestTheSubmissionDetaches` | 没有，先后相同而不受影响 | 两格都比 `Equal`：一格比请求的期限，一格比时刻加一小时；间隔要到 30 分钟才会改变哪个早 |
| `submit_upgrade_test.go` 其余调 `newSubmit` 的测试 | 没有 | `SubmitTimeout` 都是 1 分钟或 1 小时（审查 B） |
| `start_upgrade_test.go:619-652`（`its-own-deadline`、`the-requests-own-end`） | 没有 | 产品记常量 `s.d.Timeout.Milliseconds()`（`start_upgrade.go:177`），断言 `== 50`；假时钟是固定的日期；请求的期限先到时不记这一行 |
| `review_upgrade_test.go:157-193`（批准；仓库里没有 `approve_upgrade_test.go`） | 没有 | 产品记常量 `a.d.ApproveTimeout.Milliseconds()`（`approve_upgrade.go:86`），断言 `== 50`；它的 `elapsed_ms ≥ 50` 是真实对真实，起点在 `WithTimeout` 之前，恒成立（审查 B） |
| 产品里用 `ctx.Deadline()` 的地方 | submit 与 grep 两处混用 Clock 端口与真实的期限；只有 submit 的测试偶发 | grep 由 ctx 的真实期限算出软期限，与 Clock 端口的读数比（`page/app/grep.go:329`、`:479`、`:565`）；它的测试用 `WithDeadline` 传一个确定的时刻，假时钟从同一个时刻推出，二者之差在构造上精确（`grep_test.go:714-718`、`:787-790`、`:869-871`、`:890-892`、`:910-913`），默认的 `tickingClock` 只配不带期限的 ctx。`ingest_coverage.go:179`、`lint_runs.go:68`、`mcpserver/call.go:50` 是真实对真实；`index_search.go:82` 是 Clock 对 Clock；产品里的 `WithDeadline` 只有 `call.go:53` 与 `submit_upgrade.go:108`（审查 B 核实） |
| 组合根 `upgrade_start_test.go:239`、`upgrade_approve_test.go:321` | 没有 | 断言常量 300、2000；真实时钟 |
| `lint_runs_test.go:448`、`call_test.go:310`、`transfer/app/export_test.go:337` | 没有 | 真实对真实，界都宽（审查 B） |

假时钟与数据库的 `now()` 混用的测试（适配器一层）不是这种写法，没有查（审查 B）。

## 改法

只改 `submit_upgrade_test.go`，产品没动，不加长任何超时。

1. **实施**：
   - `ad68ca6a`：请求先到期的四格在 `WithTimeout` 之后再读一次假时钟，照生产的先后；断言仍是 `timeout_ms ≤ 50`。这之后 `Execute` 之前的停顿只让值变小。
   - `203fc064`：`timeout_ms` 断言精确值，不留余量：请求活着的格等于 `tc.timeout`，请求先到期的格等于请求的期限减行里的 `SubmittedAt`；行的断言先查 `SubmittedAt == nil`，免得解引用 panic（决定 565：实施者自己报的 M5，界放过了记 0）。
2. **修复第 1 轮**（主持者的修复说明 `submit-deadline/fix-1.md`）：
   - `1f94312b`：两个循环都在 `Execute` 的前一行读假时钟，`ad68ca6a` 那一行也挪到这里，COMMIT 两格在 `newSubmit` 之后加同样的一行；读钟那一行的注释改为说明用意（SDB-M1；SDA-N2、SDC-N1 的注释一半）。
   - `75c38196`：读钟之后取测试自己的时刻 `at := f.at()`；行的断言要 `SubmittedAt` 等于它，期望值改为 `d.Sub(at)`（SDA-N1）；请求有期限的格另断言 `want.Milliseconds() <= tc.request.Milliseconds()`（SDA-N2、SDC-N1，按毫秒比照 C 的提醒）；告警行要 `elapsed_ms` 等于 `tc.elapsed`（SDC-M1）。
3. **修复第 2 轮**（`fix-2.md`）：
   - `9845aa7e`：读钟拆成 `read := time.Now()` 与 `f.checkFake.at = read.UTC()`；上界改为 `d.Sub(read) > tc.request` 就红，按毫秒的那一条去掉，注释跟着改（SDA-M1）。`read` 留着单调读数（实施中定）。

## 审查的发现与处置

编号照审查者的报告。第一轮的代码位置是 `203fc064` 的，核对第 1 轮是 `75c38196` 的。修复照主持者的说明（`submit-deadline/fix-1.md`、`fix-2.md`）在分支上加提交，普通推送、不改写已推的历史，每个单独为绿；文档的发现在本提交。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| SDA-N1 | 细节（测试） | 期望值由产品写的 `SubmittedAt` 算：A4（时刻截到毫秒）、A5（再减 1 ms）在本测试里活，行与日志一起挪；`TestTheSubmissionDetaches` 抓得到 | `75c38196`：期望值用测试自己的 `f.at()`，断言 `SubmittedAt` 等于它。核对 A：A4、A5 五格都红在行的断言，关上 |
| SDA-N2 | 细节（注释、提交说明） | 断言成了精确值之后，读钟的先后已不影响过不过（D0、D2 都 20/20 绿）；`:558-564` 的注释读来像那一行挡住了失败；`203fc064` 说明的"the value does not depend on how long the test takes"不对 | 注释由 `1f94312b` 改；说明见"更正"；先后另由上界看见（决定 565）。核对 A：关上 |
| SDB-M1 | 中（测试） | COMMIT 两格的 50 ms 也从造替身时算起，实施报告第 4 节说它们"没有"问题；`Execute` 之前睡 60 ms，两格 5/5 红 | `1f94312b`：两个循环都在 `Execute` 的前一行读钟（决定 566）。核对 B：挪之前红，挪之后读钟之前睡 60 ms 绿，关上 |
| SDB-M2 | 中（文档稿） | 实施报告第 4 节说只有 submit 混用 Clock 端口与真实的期限：grep（`grep.go:329`、`:479`、`:565`）也混用，它的测试不偶发 | 本记录"同样写法的清单"一行；05-lessons 的第一条补 grep 的写法。核对 B：关上 |
| SDB-N1 | 细节（文档稿、提交说明） | 3.27 ms 是两个 id 之间的；两次取时相隔 3.00–3.32 ms | 本记录"定因"；`ad68ca6a` 说明的那一句见"更正"。核对 B：关上 |
| SDB-N2 | 细节（文档稿） | 停顿的位置与长度是核实的，只有原因是推断 | 本记录"定因"照此分开写。核对 B：关上 |
| SDB-N3 | 细节（文档稿） | UUIDv7 那一条 lesson 要补前提：同一进程、标准库、墙上时钟；id 圈出的是窗口 | 05-lessons 第 7 节那一条。核对第 1 轮 B：部分关上，剩"各推后 1/4096 ms"不准、缺"`timeout_ms` 也按墙上时钟算"，本提交照改 |
| SDC-M1 | 中（测试） | 告警行的 `elapsed_ms` 不断言，记 0 照过（C 的 MC3；M11/P2/S2 起就有） | `75c38196`：断言它等于 `tc.elapsed`（决定 565）。核对 C：EL0 三格红，关上 |
| SDC-M2 | 中（文档稿） | 文档的清单漏了第一次分支 CI 的红，实施报告的草稿停在第二个提交之前 | 本提交写全：本记录、第 11 节决定 564–571、第 12 节 P4 一行、第 13 节一行、05-lessons 两条。核对第 1 轮 C：部分关上，缺第 12 节一行与放在哪一行的决定、记录的"审查的发现与处置""主持者的决定""核对"三节、合并之后的文档提交，都在本提交补；合并之后的文档提交照先例 `0918ad73`、`db009edd`，补合并号、本提交的分支 CI 与合并之后第一次 `main` |
| SDC-N1 | 细节（注释） | 精确值之后，`ad68ca6a` 那一行的先后已不影响任何断言，注释读来像它还在挡偶发 | `1f94312b` 改注释；`75c38196` 加回按毫秒的上界，`9845aa7e` 改为改回就红。核对 C：关上 |
| SDC-N2 | 细节（提交说明） | `ad68ca6a` 说明里 `TestTheSubmissionDetaches` 的比较对象只对一格 | 见"更正"。核对 C：关上 |
| SDC-N3 | 细节（说明、注释、记录） | `WithTimeout` 之后、第一个事务的授权之前停 50 ms 以上，四格仍红（`context deadline exceeded`），改前就有 | 不改，写进"接受与推后的"（决定 567）。核对 C：关上 |
| SDC-Q1 | 问 | `203fc064` 的"Logging SubmitTimeout fails"算不算不准 | 记成细化（决定 571），见"更正"。核对 C：关上 |
| SDA-M1 | 中（测试，核对第 1 轮） | 按毫秒的上界看不出先后被改回：改回、中间不停，5 格 × 20 次全绿（差 50.0002–50.0015 ms，按整毫秒仍是 50），要停 1 ms 以上才红，又是原来的偶发 | `9845aa7e`：上界比较钟自己的读数（决定 565）。核对第 2 轮 A：D0 四格 20/20 红，关上 |
| SDA-N3 | 细节（文档稿，核对第 1 轮） | 草稿的变异表少了取整的 C1、C2 与 D0 | 本记录"反向对照与变异"的表。核对第 2 轮 A：关上 |
| SDB-N4 = SDC-N5 | 细节（文档稿，核对第 1 轮） | 读钟挪到 `Execute` 前一行之后，COMMIT 两格读钟之后停 50 ms 以上也红（答 `server_busy`），草稿只写了四格；变异表的 MC1、ORD 两格没填 | 不改代码，写进"接受与推后的"（决定 567）；表照填 |
| SDC-N4 | 细节（提交说明，核对第 1 轮） | `1f94312b` 的说明误述了被换掉的注释 | 见"更正" |

## 主持者的决定

负责人可以改判；正文在 [00-M12](../00-M12-design.md) 第 11 节"2026-11-18：提交期限的旁支的决定"。

- **564 测试的问题，开旁支修，改测试、不改产品，替身照生产的先后读钟**；照常审查、合并（同决定 356、359、541、548）。不把间隔算进上界。
- **565 随旁支补精确值的断言、`elapsed_ms` 与按读数的上界**：实施者自己的 M5（界放过记 0）与 C 的 SDC-M1 是同一行、同一种缺口；A 的 SDA-N1 让期望值用测试自己的时刻；SDA-N2、SDC-N1 加回上界，核对 A 的 SDA-M1 改为比较钟自己的读数，改回就红。
- **566 两个循环都在 `Execute` 的前一行读假时钟**（SDB-M1，取 B 的第 1 种改法）。
- **567 读钟之后停 50 ms 以上照样红，不改**（SDC-N3、SDB-N4 = SDC-N5），写进"接受与推后的"。
- **568 分支 CI 的两次红只重跑失败的项**：bootstrap-7 那一次由等锁帮手的旁支查清、修（决定 548）；costs-3 那一次记成观察的偶发，M12 收尾之前再出现一次就开旁支查（见"costs-3 那一次"）。
- **569 第 12 节的链接与合并号放在 P4 一行**（同决定 546）。
- **570 05-lessons 第 2、7 节各加一条；M11 的文档与总体设计不改**。
- **571 核对第 2 轮只请 A，之后不开第三轮；已推的提交说明不改写，不准与细化写在"更正"**（SDC-Q1 记成细化；同决定 547、554）。

## 实施中定的

1. 改法取"照生产的先后"，不取"把间隔算进上界"：后者要在测试里量间隔、把界放成 50 加间隔，是在生产不会有的先后下认可 53 这样的值（实施报告第 5 节；决定 564）。
2. 修复第 1 轮分成两个提交：一个改替身读钟的位置（`1f94312b`），一个改断言（`75c38196`）。"alive"一格也在 `Execute` 的前一行重读一次，结果不变，五格写法相同。
3. 上界先按整毫秒比（`75c38196`）：时刻截到微秒，最多早于读钟不到 1 µs，按 `Duration` 比会偶发（审查 C 的提醒，推断，没造出这种情形）。之后改为比较钟自己的读数（`9845aa7e`，SDA-M1）。
4. `read` 留着单调读数。A 的建议写的是 `time.Now().UTC()`，那样会去掉它。`WithTimeout` 的期限带单调读数，两边都带时 `Sub` 只用单调时钟，墙上时钟在两次读之间往回跳也不误红；假时钟仍取 `read.UTC()`，与产品看到的值相同。核对第 2 轮 A 读 `context`、`time` 的源码，并用探针核实（见"核对"）。
5. 后缀写 `(M11 story, found in M12)`：被修的测试、界与取时都来自 M11/P2/S2 的 `74a3057a`，照 00-M12 第 7 节"旁支"（审查 C 核过）。

## 接受与推后的

- **接受**：
  - **50 ms 期限本身的余量**（决定 567；SDC-N3、SDB-N4 = SDC-N5）：读钟之后、`Execute` 走到相应的一步之前停 50 ms 以上，六格照样红。请求先到期的四格在答复那一条断言（`9845aa7e` 的 `:572-573`）答 `context deadline exceeded`：替身的授权先看 `ctx.Err()`，第一个事务里就授权（`start_upgrade.go:71`）。COMMIT 两格在 `:625-626` 答 `server_busy`：它们 50 ms 的 `SubmitTimeout` 要盖住第一个事务与检查。"alive"一格不受影响。这个窗口改前就有（读钟挪动之前 COMMIT 两格的更大，即 B60），要停的是 CI 那次停顿的 15 倍；以后在这两处见到这两种答复，不是这个偶发回来了。
  - 先后改回时，两次单调读数恰好相等就看不出：本机两次读之间最小 125 ns，80 次里没有一次相等（核对第 1、2 轮 A）。
  - MC2（请求有期限就总取它，不比哪个早）在本测试里活：没有一格的请求晚于 `SubmitTimeout`，是表的形状；`TestTheSubmissionDetaches/the_request's_later` 抓到，包里有人守（审查 C）。
  - M1b、M1c：产品总取 `first.at + SubmitTimeout` 时，1 小时那一格只靠 `go test -timeout` 才红；同一个变异在 10 秒的三格上红在断言上（M1a）。
- **未核实**：
  - 墙上时钟（审查 A）：生产里若墙上时钟在定期限与读钟之间往回跳（NTP 一步调回），`timeout_ms` 可以超过请求的超时。只影响日志的一个字段，与本旁支无关，没有造这种情形。测试一侧的上界比的是单调读数，不受它影响（`9845aa7e`）；核对第 1 轮 A 对按读数的上界提的同一种怀疑，由此消掉。
  - runner 停顿的原因；COMMIT 两格与 50 ms 的余量在 CI 上多久碰上一次；SQL 的 `uuidv7()` 的亚毫秒位怎样排（审查 B）。
- **不改的文档**：
  - M11：[P2 设计](../../M11-upgrade/02-P2-guardrails-gate-submit.md) `:1041` 写"假的时钟以 `time.Now()` 起算"，改后仍成立，只是在 `Execute` 的前一行读；它列的各格与 P2/S2 的审查记录（变异表）是当时的记录。
  - `v0.1-design.md`：第 13 节的长期约定没变，跨 M 的契约与扩展点也没变；这次没有新的帮手，写法的经验进 05-lessons（先例 357、541 也没改）。
  - Phase 设计：这条旁支不属哪个 Phase，没有"结果"可写。
  - 00-M12 第 10 节的风险：只红过一次、已经修了，同决定 360 不加。
  - 移交：没有与它有关的 `open` 事项。

## 更正

已推送的说明不改写（先例 `642f9196`，决定 547、554），在这里写对：

- `ad68ca6a`：
  - "it compares the run's deadline with the request's exactly"：`TestTheSubmissionDetaches` 两格里只有"the request's earlier"比请求的期限；"the request's later"比的是时刻加一小时（`submit_upgrade_test.go:480-483`）。"不受影响"的结论不变（SDC-N2）。
  - "the notebook's, made in newCheckFake just before the clock was read … 3.27 ms between"：3.27 ms 是两个 id 之间的；紧挨着读钟的是 upgrade 的 id，比 notebook 的晚 2 µs；两次取时相隔 3.00–3.32 ms（SDB-N1）。
  - 细化："Logging SubmitTimeout as timeout_ms fails it too"在测试一层成立：红的是请求先到期的四格，"alive"一格的 `SubmitTimeout` 就是它的期限，等价（SDC-Q1）。
  - 细化："A pause anywhere before Execute now only lowers timeout_ms"对 `timeout_ms` 成立；读钟之后停 50 ms 以上，测试换一个样子红，见"接受与推后的"（SDC-N3）。
- `203fc064`：
  - "so the value does not depend on how long the test takes"：不依赖用时的是判断，值本身依赖：读钟挪回 `WithTimeout` 之前再停 2 ms，记下的与期望的都是 52（SDA-N2 的 D2）。
  - 细化："Logging SubmitTimeout fails"同 `ad68ca6a` 那一句（SDC-Q1）。实施者原想写成"四格红"，那次改动没生效，提交用的是原稿。
- `1f94312b`："The comment no longer says the value does not depend on how long the test takes"：被换掉的注释（`ad68ca6a` 的）说的是"不论用时多久，都不超过 50"（"is then at most the request's 50 ms however long the test takes to get here"）；"不依赖用时"只在 `203fc064` 的说明里（SDC-N4）。注释本身的改动照实。

## 反向对照与变异

探针都用 `go test -overlay`；变异由 `mut.py` 改文件、从 `.mutbak` 复原。命令先在没改的树上跑过：整个测试与 10 秒的三格都 rc=0，`-run` 的过滤只选中要的格。各列是那个头上的结果，"没跑"是那个头上没跑过的；"四格"是请求先到期的四格。

| 变异或探针 | 改了什么 | `ad68ca6a`（界 ≤ 50） | `203fc064`（精确值） | `75c38196`（按毫秒的上界） | `9845aa7e`（读数的上界） |
|---|---|---|---|---|---|
| ORD（撤掉修复加探针 A） | 测试：读钟挪回 `WithTimeout` 之前，中间睡 2 ms | 红，四格 20/20，52 ×79、53 ×1（C 的 T1） | 活，160 PASS（C） | 红，四格 20/20 在上界，52 ×75、53 ×5 | 红，四格 5/5 在上界，52.2–53.1 ms |
| D0 | 测试：读钟挪回 `WithTimeout` 之前，不停 | 活（就是改前的测试，200 次全绿） | 活，20/20（A） | **活**，5 格 × 20，记下与期望都是 50（A，SDA-M1） | **红**，四格 20/20 在上界，期限比读数晚 50.00008–50.00113 ms（实施者）、50.000125–50.006625 ms（A） |
| 探针 A、B | 测试：`WithTimeout` 之前、或它与读钟之间睡 2 ms | 绿，各 20 次 | 绿，各 20 次 | 绿，各 20 次 | 绿，各 20 次 |
| B60 | 测试：COMMIT 两格在读钟（挪之前是 `Execute`）之前睡 60 ms | 没跑（这两格的代码同 `203fc064`） | 红，两格 5/5，答 `server_busy`（B） | 绿，5/5 | 绿，5/5 |
| T3、T3c | 测试：读钟之后、`Execute` 之前睡 60 ms | 没跑 | 红，四格 3/3，`context deadline exceeded`（C 的 T3） | 红：四格 3/3（C 的 T3），COMMIT 两格 3/3（C 的 T3c）、5/5（B），答 `server_busy`；余量，接受 | 没重跑（读钟之后的代码同 `75c38196`） |
| M5、MC1 | 产品：告警行的 `timeout_ms` 记 0、少记 1 ms | 活，8 PASS（实施者、C） | 红，五格 | 红，五格（MC1："alive"记 49，四格记 48） | 红，五格 |
| C1、C2 | 产品：`timeout_ms` 向上取整、四舍五入 | 活，10/10（A，用这一版的断言） | 红，四格 10/10 | 红，四格 10/10 | 红，四格 10/10；"alive"恰 50，等价 |
| A1–A3 | 产品：告警行取错时刻（`time.Now()`；`Clock.Now()`；第一个事务之前的 `time.Now()`） | 没跑 | 红 | 红 | 红（核对第 2 轮 A） |
| B1、B2 | 产品：期限取晚的那个；告警行取 `SubmitTimeout` | 没跑 | 红 | 红 | 红（核对第 2 轮 A） |
| A4、A5 | 产品：时刻截到毫秒；再减 1 ms | 没跑 | 本测试里活（`TestTheSubmissionDetaches` 两格红） | 红，五格在行的断言 | 红，五格在行的断言 |
| EL0（C 的 MC3） | 产品：告警行的 `elapsed_ms` 记 0 | 没跑（不断言 `elapsed_ms`） | 活（C，整个包 `-short`） | 红，钟挪了的三格；`elapsed` 为 0 的两格等价 | 红，三格 |
| M1a | 产品：`deadline` 总取 `first.at + SubmitTimeout`，只跑 10 秒的三格 | 红，三格各等 10 秒，记 10000 | 红 | 红 | 红 |
| M3 | 产品：告警行记 `SubmitTimeout` | 红，四格 | 红，四格 | 红，四格 | 红，四格；"alive"等价 |

- A2 只在钟挪了的三格红、A3 只在"alive"一格红（20/20，49 对 50），另几格等价；B1 在 10 秒的三格各等 10 秒之后红，1 小时那一格没跑（审查 A）。
- M1b、M1c（1 小时那一格与整个测试）只在实施时跑过，靠 `go test -timeout` 才红。MC2 见"接受与推后的"。
- 定改法时的探针：C 的 PE（只加精确的 `elapsed_ms`）`-count=50` 400 PASS，与 MC3 合跑三格红；PORD（按毫秒的上界）`-count=200` 1600 PASS，加 T1 四格 20/20 红在它上面。核对第 1 轮 A 的 S_ok、S_rev（比较读数的上界）：先后对时 `-count=100` 与 `-race -count=100` 全绿，改回不停四格 20/20 红，多出 125 ns–1.5 µs。
- 不误红的探针（`75c38196`）：A 的 D3（读钟之后、`Execute` 之前睡 2 ms）四格记 49、D4（`WithTimeout` 与读钟之间睡 2 ms）记 47 或 46，都绿；B 的 `WithTimeout` 之前睡 60 ms 7 格 × 5 全绿（读钟在后，停多久都不越界），"alive"一格读钟之后睡 60 ms 也全绿。

## 核对

**修复核对第 1 轮**（2026-11-18，三位挪到 `75c38196`，看 `203fc064..75c38196` 与实施报告的文档草稿）：

- **A**：SDA-N1、N2 关上。重跑 A1–A5、B1、B2、C1、C2 都红；探针 P 每格打印期望与记下的值，"alive"50/50，四格 49/49，四格都是请求的期限早；D3、D4 不误红。新的 SDA-M1（D0 照绿）与 SDA-N3（变异表），并用 overlay 核过 SDA-M1 的改法。
- **B**：SDB-M1、M2、N1、N2 关上，SDB-N3 部分关上（剩两处，本提交改）。9 个探针：挪之前 B60 红、挪之后绿；`WithTimeout` 之前睡 60 ms 也绿；`WithTimeout` 与读钟之间睡 60 ms，四格红在答复那一条（SDC-N3）；COMMIT 两格读钟之后睡 60 ms，两格红（新的 SDB-N4）。两个提交的说明把"读钟之前"说准了。
- **C**：SDC-M1、N1、N2、N3、Q1 关上，SDC-M2 部分关上（本提交补齐）。`1f94312b`、`75c38196` 各自为绿，`-race -count=200` 1600 PASS；变异与探针 13 项照预期；`75c38196` 说明里变异与探针的六条逐条复现。新的 SDC-N4、N5。

**修复核对第 2 轮**（2026-11-18，只请 A，挪到 `9845aa7e`；决定 571）：

- **A**：SDA-M1、N3 关上，没有新的发现。D0 四格 20/20 红在上界，D2 四格 5/5 红；先后对时 `-count=100` 与 `-race -count=100` 全绿；A1–A5、B1、B2、C1、C2 重跑都红。"单调部分"一说成立：`WithTimeout` 就是 `WithDeadline(parent, time.Now().Add(timeout))`，`Add` 连单调读数一起加，`WithDeadline` 原样存、`Deadline()` 原样返回（`context/context.go:704-706`、`:642`、`:670-672`），两边都带单调读数时 `Sub` 只用单调时钟（`time/time.go:1198-1200`）；探针里四格的 `ctx.Deadline()` 与 `read` 都带单调读数、`at` 不带，`d.Sub(read)` 与只看墙上时间的差相差 0–41 ns。期望值与产品仍按墙上时钟同算，仍精确。

## 门禁与 CI

- **门禁**（本机 arm64，经 `heavy.sh`，连着的几项写成一个脚本经一次 `heavy.sh` 跑完）：

  | 头 | 谁 | 结果 |
  |---|---|---|
  | `ad68ca6a` | 实施者 | `go build ./...`、`go vet ./...` rc=0；`make lint-go` 两处 `0 issues`，`go mod tidy -diff` 两处干净；`make test-go-short` 94 个包 `ok`（46 秒）；`TestTheSubmissionsDeadline` `-race -count=200` 过（75 秒） |
  | `203fc064` | 实施者 | 同上四项绿（`make test-go-short` 94 个包 `ok`）；`-count=200`（75 秒）与 `-race -count=200`（80 秒）都过 |
  | `ad68ca6a`、`203fc064` | 审查 C | 各自 build、vet、`make lint-go`、`make test-go-short`（41、43 秒）都绿；`203fc064` 上 `-race -count=200` 1600 PASS（78 秒） |
  | `1f94312b`、`75c38196` | 实施者 | 各自四项绿（`make test-go-short` 41、40 秒）；`75c38196` 上 `-count=200`（74 秒）与 `-race -count=200`（75 秒），7 格各 200 次都过 |
  | `1f94312b`、`75c38196` | 核对第 1 轮 C | 各自四项绿；`75c38196` 上 `-race -count=200` 1600 PASS（76 秒） |
  | `9845aa7e` | 实施者 | 四项绿（`make test-go-short` 40 秒）；`-count=200`（73 秒）与 `-race -count=200`（75 秒）都过 |
  | `9845aa7e` | 核对第 2 轮 A | `-count=100` 与 `-race -count=100` 都过 |

  build、vet、lint 有几次只用 0–1 秒：Go 的构建缓存与 lint 的缓存命中，日志里是真跑过的输出；`make test-go-short` 带 `-count=1`。golangci-lint 是主克隆 `bin/` 里的 2.14.0，没有下载。
- **分支的 CI**（普通推送，没有改写已推的历史；`1f94312b` 与 `75c38196` 一起推送）：

  | 推送 | 头 | `ci.yml` | 结果 |
  |---|---|---|---|
  | 1 | `ad68ca6a` | 44078438315 | 第一次 attempt：16 份 server 里 15 份绿，`web` 绿；`short` 3:31（Test 3:17），本支改的测试在这一份里，过了；`race` 11:09。`server (bootstrap-7)` 红（job 134022409608，Test 4:22）在 P3/S1 的 `TestTheLogReadsOneMomentAcrossARevert/MCP's_log`（`history_snapshot_test.go:112`，读看到了 R），`e2e`、`image` 被跳过；本支只改 `wiki/app` 的测试，`bootstrap` 包编译不到它。只重跑失败的作业，第二次 attempt 全绿。原因由旁支 `m12-lockwait-by-statement` 查清、修（决定 548、568） |
  | 2 | `203fc064` | 44163179005 | 第一次 attempt：16 份 server 里 15 份绿，`web` 绿；`short` 4:30，`race` 9:32。`server (costs-3)` 红（job 134322148664，Test 2:11），见下"costs-3 那一次"；`e2e`、`image` 被跳过。只重跑 costs-3 与被跳过的两项，第二次 attempt 19 项全绿 |
  | 3 | `75c38196` | 44175820202 | 19 项第一次 attempt 全绿：`short` 3:29（Test 3:14），`race` 14:55，八份组合根的 Test 3:44–6:24，costs-3 11:51，`web` 4:13，`e2e` 9:38（E2E 一步 8:13），`image` 1:17 |
  | 4 | `9845aa7e` | 44191830725 | 19 项第一次 attempt 全绿：`short` 3:00（Test 2:44），`race` 13:19，八份组合根的 Test 4:14–6:31，costs-3 14:04，`web` 4:14，`e2e` 10:04（E2E 一步 8:28），`image` 1:15 |

- **本提交自己的分支 CI**：`dd8ccd9c`（合并 `main` 的 `9a61b990` 一起推送）的 run 44214674002：第一次 attempt 18 项绿，`e2e` 在"Install the Playwright browsers"一步用了 18.5 分钟（`--with-deps` 的 apt 下载慢），撞上作业 20 分钟的时限被取消；只重跑 `e2e`，第二次 attempt 绿（安装 2:08，E2E 8:32）。旁支合并为 `4e309997`（00-M12 第 12 节的表在合并时与 P5/S1 的记号相邻冲突，两边都留），之后第一次 `main` 的 run 44221271650 同样只有 `e2e` 被取消在浏览器的安装上，第二、三次 attempt 也是（apt 镜像 120 kB/s），第四次 attempt 绿（安装 1:03，E2E 7:03）；同一时段别的运行的安装不到一分钟，慢的是个别 runner 的镜像。其余 18 项第一次 attempt 都绿。

## costs-3 那一次

`203fc064` 的 run 44163179005 第一次 attempt（决定 568）：

- 红的是 M8 的代价守卫 `TestTrashHistoryAtItsBoundCountsInTimeWithForcedPlans/no_pg_stats/force_generic_plan`（`page/adapter/postgres`，`trash_history_forced_cost_test.go:200`）：40 万行的计数 2.720 秒，界 1 秒；同一测试的另三格 0.25–0.30 秒。
- 那一次跑在 AMD EPYC 9V74 上，读的是 `gh run view 44163179005 --attempt 1 --log` 的 Runner CPU 一步：`gh run view --job 134322148664 --log` 给的是第二次 attempt 的日志（05-lessons 第 7 节，审查 C）。第二次 attempt 只重跑它与被跳过的 `e2e`、`image`，在 Intel Xeon 6973P-C 上 19 项全绿。
- 与本支无关（核实）：两个提交只改 `wiki/app` 的 `_test.go`，`page/adapter/postgres` 编译不到它们；`ad68ca6a` 的运行里 costs-3 是绿的。
- 主持者查了最近 100 次失败的 `ci.yml`：costs 的份红过的有 M10 的 `TestGrepOverTheGlobGuardTreeTakesItsTime`（42342868114）等，没有这个测试；文档里也没有它偶发的记录（审查 C 以 `git grep` 核过）。原因没查：只一次，读数是界的 2.7 倍，同一作业的另三格正常，像是计划或停顿，是推断。
- 照决定 360 的先例只重跑，记成观察的偶发；M12 收尾之前再出现一次就开旁支查；收尾时一并查这个测试在 `main` 与各分支 CI 上有没有再红。只一次、原因没查，不进 05-lessons。

## 环境

实施在工作树 `nerve-wiki-m12-submit-deadline`；审查与核对各在分离的工作树里（A `nerve-wiki-m12-p4-rev-a`，B `nerve-wiki-m12-p4s5-rev-c`，C `nerve-wiki-m12-side-rev`），不碰实施者的工作树。重的都经 `heavy.sh`，每项之前判 `df`；探针都用 `go test -overlay`，变异由 `mut.py` 改文件、从 `.mutbak` 复原；审完各工作树干净，没有 `*.mutbak`。`TMPDIR`、`GOTMPDIR` 与日志在 `~/.cache/m12-submit-*`，用完删掉；没起容器。本机 arm64（Parallels 虚机）。审查 B、C 与主持者经 `gh` 只读 CI 的日志与历史。

**磁盘**：规矩是每项重的之前判 `df -h /`，可用少于 15G 就停。实施第一个提交时，有两项重的运行起在只剩 14G、13G 的时候：一项是带输出重跑的 M1 变异，它的 `df` 那一行印出 14G，实施者当时没读；紧接着的门禁起之前没判。实施者看到 13G 时没杀正在跑的那一项（它拿着 heavy 的锁，怕留下锁目录），之后不再起重的运行。空间是别处占的：根分区用量从 101G 涨到 107G，共用的 `~/.cache/go-build` 36G。第二个提交的脚本在 14G 时照规矩停下。两次都由主持者经负责人确认腾出空间（Docker 的构建缓存与卷，Go 的构建缓存）之后续做；之后每项之前可用 41–52G。

要留的在设计工作目录的 `m12/submit-deadline/`（`impl-report.md`、`review-*.md`、`check1-*.md`、`check2-a.md`）。
