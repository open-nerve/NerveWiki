# M12 旁支 `m12-lintbusy-await-expiry`（M10 的 lint 忙的测试等到旁证就当作对方已做完）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-lintbusy-await-expiry`，从 `main` 的 `5154e0de`（提交期限的旁支记合并号的提交）开始（主持者的决定 586）。只改测试，不改产品，不加长任何等待。<br>实施 1 个提交 `7abfd6ef`：新的同键调用者发起之前，等旧 flight 的 `RunExpired`。<br>修复第 1 轮 2 个提交：新 flight 的跟随者发起之前，等旧 worker 的 `RunFinished` `d8e664b3`（LBA-M1）；末尾的连接数改成限时轮询 `9079f9af`（LBB-M2）。<br>到 `9079f9af` 1 个文件（`server/internal/bootstrap/interleavings_lint_test.go`），加 19 行、减 2 行。本提交之前把当时的 `main` 合进来一次 |
| 审查方式 | 实施者在工作树 `nerve-wiki-m12-lintbusy` 里实施、跑反向对照与变异。三位审查者并行，各在自己的分离的工作树里（A `nerve-wiki-m12-side-rev`，B `nerve-wiki-m12-p4-rev-a`，C `nerve-wiki-m12-p4-rev-c`），挪到 `7abfd6ef`，重的都经 `heavy.sh`，不碰实施者的工作树：<br>A 改动本身：新的一行等到的只能是 `expire` 的事件；之后各断言；S61 三种变异；计时探针 8 次。<br>B 定因与同类：在 `main` 与本支上复现；连接池的销毁在运行时看（`-race -count=10` 的探针）；交错测试之外的组合根测试与 transfer 心跳的取消（拖慢 `stop` 的探针）。<br>C 门禁、提交与文档：build、vet、`make lint-go` 三项与本测试 `-race -count=10`；时间线探针 20 次；提交说明逐句核对；文档的去处。<br>修复核对第 1 轮三位挪到 `9079f9af` |
| 日期 | 2026-11-18（查因在 P2/S2 的审查 C；实施、审查、修复一轮、修复核对一轮） |
| 结论 | Important 0。Minor 6（去重 5 条）：S61 第三种变异的抓法靠"新 flight 到了 Prepare"这个旁证（LBA-M1）；"亚毫秒""平常两者都在微秒级"不实（LBA-M2 = LBC-M1）；同类的清单漏了 transfer 心跳的取消（LBB-M1）；末尾的连接数依赖池的异步销毁（LBB-M2）；提交说明把推断写在 Verified 下（LBC-M2）。Nit 6（LBB-N1、N2，LBC-N1–N4），Q 2（LBC-Q1、Q2）；分开是 A 0I 2M，B 0I 2M 2N，C 0I 2M 4N 2Q。三位都确认改动本身成立。LBA-M1、LBB-M2 由修复第 1 轮两个提交改；LBB-M1 查明、不改（决定 596）；其余在本提交的文档与"更正"。修复核对第 1 轮：A 0I 0M 1N，B 0I 0M 0N，C 0I 0M 5N（LBA-C1-N1 = LBC-C1-N1，Nit 去重 5 条），第一轮的都关上，新 Nit 都在本提交写对 |

## 起因

- **P2/S2 推送之前的本机门禁**：实施者在头 `4c9f1212` 上经 `heavy.sh` 跑 `make test`（竞态检测，组合根九个目标 `-j6`），第一次 rc=2：第 4 份的 `TestLintBusyReturnsBeforeCleanup/postgres` 红在 `interleavings_lint_test.go:427`：

  ```
  new same-key caller joined an expired worker: {Kind:2 FlightID:1 …}
  ```

  同一个头上单跑 `-race -count=10` 十次过，第二次整个 `make test` 过。S2 在服务端只改 `wiki/domain` 的两份模板与一个测试（审查 C 以 `git diff --stat f08dca69 4c9f1212 -- server/` 核过）。
- **测试的来历**：测试与 `Runs` 都来自 M10/P3/S1 的 `4dd2ad95`（`git log -S`）；[M10/P3 设计](../../M10-llm-wiki/03-P3-lint-health.md) `:966` 与 `:1040` 的 S61 写"过期 flight 不再 join"，没写要等过期的事件。所以提交写 `(M10 story, found in M12)`。
- **定因与开旁支**：P2/S2 的审查 C（P2S2C-Q1）沿代码走出原因、以探针确定地复现；主持者定为测试的先后假设，开本支（决定 586）。

## 定因

### 核实的

- **机制**（P2S2C-Q1；实施者与审查 A、B 读代码）：
  - 一次运行的过期由 `context.AfterFunc(worker, func() { r.expire(f) })` 在另一个 goroutine 里做（`wiki/app/lint_runs.go:98`）。`expire` 在 `r.mu` 下 `forget`、发布 `server_busy`，放开锁之后才发 `RunExpired`（`:186-197`）；`Do` 在 `r.mu` 下查 `r.flights`（`:85-99`），`forget` 之前同键的调用者照样加入。
  - `pg_sleep` 被取消、回调关 `unwinding`，是 pgx 看同一个 worker ctx 做的（pgx v5.11.0 默认的 `DeadlineContextWatcherHandler`，项目没有覆盖）。两件事都由 worker ctx 结束触发，各在自己的 goroutine 里，没有先后：审查 C 的探针里，不带竞态的 10 次 `expire` 晚于 `pg_sleep` 返回 8 次、早 2 次。
  - alice 的 `server_busy` 也不一定经过 `expire`：她的 `wait` 是另一个计时，在 worker 之前造、同是 2 秒（`:78`、`:92`；内部调用的 ctx 是 `t.Context()`，没有期限）。
  - 改前的测试（`:414-427`）收到 alice 的 503、等到 `unwinding`，就让 bob 发起，并断言他新建一次运行。
- **两边各要多久**（审查 A 的计时探针 8 次、审查 C 的时间线探针 20 次，本机没有别的负载）：
  - `expire` 平常在 `pg_sleep` 返回前后 0.1 ms 之内就忘掉 flight（C：−0.06 到 +0.10 ms）；测试过了 `unwinding` 之后 10–42 µs 就取到 `RunExpired`（A）。
  - 测试从 `unwinding` 走到 bob 被收下要 1.3–5.1 ms（C 1.34–3.07 ms，A 2.07–5.14 ms）：`f.actor` 查一次库，`Execute` 在 `Runs.Do` 之前 `authorize` 再读成员（`wiki_lint_runtime_test.go:89-91`，`wiki/app/lint.go:58`）。
- **复现**（经 `heavy.sh`，`-run '^TestLintBusyReturnsBeforeCleanup$'`）：

  | 情形 | 谁 | 结果 |
  |---|---|---|
  | 改前，不动 | P2S2C；实施者；审查 B（`main` `b88428b0`，3 次） | 过 |
  | 改前，`expire` 开头 `time.Sleep(300 * time.Millisecond)` | P2S2C（不带竞态）；实施者；审查 B（3/3） | 红在 `:427`，信息一字不差 |
  | 改后（`7abfd6ef`），同样的拖慢 | 实施者；审查 B（3/3） | 过 |
  | 改后，同样的拖慢，去掉等事件的一行 | 实施者 | 红在 `:427` |

  300 ms 是测试那一段的 100 倍上下，所以每次都红。
- **对产品**：deadline 与 `expire` 之间来的调用者，加入将要过期的运行、随后收到 `expire` 发布的 `server_busy`：多一次 503。窗口平常在 0.1 ms 之内。这与 `Runs` 的注释"A flight's expiry removes only its permission to be joined"相符：资格在 `expire` 运行时才去掉。测试断言得比代码保证的严，不改产品。

### 推断

- 那一次就是 `expire` 排晚：本机没有复现原来的那一次，是由信息一字不差与 300 ms 的探针推出的。
- 照这个推断，那一次 `expire` 比测试从 `unwinding` 到 bob 拿到 `r.mu` 那一段（平常 1.3–5 ms）还晚，即负载下窗口可以到毫秒级；原因是竞态检测、组合根六份同时跑，机器忙，停了多久定不出。

## 同样写法的清单

查的是"测试等到一件事（取消、关闭、答复、事件、数据库里的一行），就认定另一个 goroutine 的副作用已经发生"。26 份 `interleavings_*_test.go`：`interleavings_lint_test.go` 整份读完，其余 25 份按 grep（等通道、`cancel()`、`close(`、`sleep`、轮询、对连接数、日志、运行事件的断言）逐处读，没有逐行读完；另看了用同一个 `Runs` 的 `wiki_lint_runtime_test.go`、`mcp_lint_interleavings_test.go`、`wiki_lint_lifecycle_test.go`，与 transfer 心跳的 `cancelSeen`（审查 B）。实施报告原写"只此一处要改"，由审查 A、B 改正（LBA-M1、LBB-M1、LBB-M2）。

| 位置（`server/internal/bootstrap/`） | 等到的 → 认定的 | 判断 |
|---|---|---|
| `interleavings_lint_test.go` 改前 `:416-427` | `unwinding`、alice 的 503 → `expire` 已忘掉 flight 1 | **是**，`7abfd6ef` 改：等 `RunExpired` |
| 同一测试改前 `:441-447` | 新 flight 到了 Prepare（`nextPrepared`）→ 旧 worker 的 `finish` 已 `forget`。对的代码不受影响（`forget` 只删自己），受影响的是 S61 第三种变异的抓法：`finish` 在 `stopExpiry` 与 `r.mu` 之间慢 300 ms，变异活下来（LBA-M1） | **是**，`d8e664b3` 改：等旧 worker 的 `RunFinished` |
| 同一测试改前 `:464` | 新 flight 的 `RunFinished` 与 `SELECT 1` → 旧连接已出池。取消之后 pgconn `asyncClose`，`Release` 走 puddle 的 `Destroy`（`go destroyAcquiredResource`），析构还要等 `CleanupDone`；审查 B 在运行时看到旧 `Release` 返回时 10/10 仍算借出，销毁 28 µs–1.16 ms，余量是新 flight 整整一次运行 23–30 ms（LBB-M2） | **是**，`9079f9af` 改：限时轮询到 0 |
| `background_test.go:332-343` 的 `cancelSeen`（7 处调用：`interleavings_background_stops_test.go:346`、`:371`，交错 11；`:703`、`:709`，交错 19；`upgrade_rollback_interleavings_test.go:153`；`upgrade_rollback_test.go:402`；`upgrade_start_test.go:440`）与 `interleavings_import_test.go:263-268` | 心跳写下 `heartbeat_at > cancel_requested_at` 的一行 → 作业已 `stop`。心跳先 `BeatJob`（自动提交的 `UPDATE … RETURNING cancel_requested`，`transfer/app/job_run.go:156`），之后才 `stop(errCancelRequested)`（`:175-176`）；两批之间看的是 `running.Err()`（`background.go:130`、`:167`，`import_write.go:51`、`:181`），写一批用的是 `ctx`。审查 B 的探针：心跳读到取消之后 `stop` 晚 300 ms，恢复、撤销、导入三个测试全红（`interleavings_background_stops_test.go:349`、`:374`，`interleavings_import_test.go:273`、`:276`），作业都成了 `succeeded`（LBB-M1） | **是，查明、不改**（决定 596）：`stop` 之后心跳随 `running` 结束就返回、不再写什么（`job_run.go:145-147`），测试一侧没有可等的事件，要给产品加观察点 |
| 同一测试 `:421`、`:438-440` | 旧回调挡在 `<-release` 上；新 flight 排在槽上 | 有先后 |
| `interleavings_lint_test.go:82-84`、`:161-163`，`wiki_lint_runtime_test.go:598-599` | `RunFinished` → 连接已还：运行成功，连接空闲，`Release` 同步，早于 `finish` | 有先后 |
| `mcp_lint_interleavings_test.go:97-98`、`:143-144` | `RunFinished` → 追踪已全（`assertSnapshot` 看追踪到的 SQL 与快照，在 worker 的 goroutine 里同步记下，早于 `finish`；LBB-N2） | 有先后 |
| `interleavings_lint_test.go:353-355`（worker panic） | `RunFinished` → 连接已还：回调里没有语句在跑，`TxManager.run` 用脱离取消的 ctx 回滚（`platform/postgres/tx.go:160-163`、`:222-226`），成功之后同步回池 | 有先后（回滚失败时会走异步的 `Destroy`，没造） |
| `wiki_lint_lifecycle_test.go:102`、`:154`、`:326`（LBB-N1） | worker 挡着时 `== 1`；`l.cleaned`、`Stop` 之后 `== 0`。回调在 `<-ctx.Done()` 上等，ctx 结束时没有语句在跑，pgx 不关连接，回滚成功，`Release` 同步（池已关时 puddle 也在锁下同步移出，`puddle/v2/pool.go:675-677`）。实施者的探针在内层 `WithinSnapshot` 返回时读连接数，九个子测试都是 0 | 不同形，不改 |
| `interleavings_lint_test.go:343`，`wiki_lint_runtime_test.go:586`、`:618` | 半预算的 503 → 调用者自己的 ctx 未到期 | 计时的余量，不是本类 |
| 其余 | 等锁的帮手（被挡的后端自己的状态）、钩子的通道、HTTP 答复；`stopUpload` 的 `.tmp` 文件（同一处理器里先 `Check` 后 `Store`，`transfer/adapter/http/start_import.go:60-66`）；交错 18 在 `stop()` 之后轮询到恢复的开始答 202；编辑会话的清理轮询 River 完成的次数到 `held+2` | 没有本类 |

## 改法

只改 `interleavings_lint_test.go`，产品没动，不加长任何等待。

1. **实施** `7abfd6ef`：在 `AcquiredConns() != 1` 那一判之后、bob 发起之前 `f.event(t, wiki.RunExpired, created.FlightID)`。`f.event` 一路丢掉 flight 1 的 `RunAcquired`，之后没有谁读它。单元一层的同一格 `old-worker-keeps-new-flight`（`wiki/app/lint_runs_test.go:386-420`）从一开始就先等 `RunExpired`（`:398`）。
2. **修复第 1 轮**（主持者的修复说明 `lintbusy/fix-1.md`）：
   - `d8e664b3`：等到 `nextPrepared` 之后、carol 的 `last` 发起之前 `f.event(t, wiki.RunFinished, created.FlightID)`。`finish` 先放槽、后 `forget`，`RunFinished` 在 `forget` 之后（`lint_runs.go:200-207`、`:234`）（LBA-M1）。
   - `9079f9af`：`SELECT 1` 的检查留着；连接数先读一次，再每 5 ms 读一次、到 `interleavingWait`（10 秒）为止，失败时报最后读到的数（LBB-M2；时限照同包的轮询，例如 `interleavings_background_stops_test.go:588`、`:614`）。

## 审查的发现与处置

编号照审查者的报告，第一轮的位置是 `7abfd6ef` 的。修复照主持者的说明（`lintbusy/fix-1.md`）在分支上加提交，普通推送、不改写已推的历史，每个单独为绿；文档的发现在本提交。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| LBA-M1 | Minor | S61 第三种变异靠"新 flight 到了 Prepare"认定旧 worker 已 `forget`；`finish` 在 `stopExpiry` 与 `r.mu` 之间慢 300 ms，变异活下来 | `d8e664b3`：carol 发起之前等旧 worker 的 `RunFinished`。同样的拖慢下变异红在 `:454`。核对 A：关上 |
| LBA-M2 = LBC-M1 | Minor | "亚毫秒""平常两者都在微秒级"不实：测试那一段 1.3–5 ms，`expire` 平常 0.1 ms 之内 | 本记录"定因"照两位量的数写；`7abfd6ef` 的说明与 P2/S2 记录的那一句见"更正"。核对 A、C：关上 |
| LBB-M1 | Minor | 同类漏了 transfer 心跳的取消：`cancelSeen` 等到心跳的行就当作 `stop` 已做；拖慢 `stop` 三个测试全红 | 查明、不改（决定 596）：清单一行，证据附上。核对 B：关上 |
| LBB-M2 | Minor | 末尾 `AcquiredConns()==0` 依赖 puddle 的异步销毁，运行时看到了 | `9079f9af`：限时轮询。改前把销毁推后 300 ms 红在那一判，改后过。核对 B：关上 |
| LBB-N1 | Nit | `wiki_lint_lifecycle_test.go` 三处连接数断言没列 | 不同形（没有被取消的语句，`Release` 同步），探针核过；清单一行。核对 B：关上 |
| LBB-N2 | Nit | `assertSnapshot` 写成"连接已还" | 清单改成"追踪已全"。核对 B：关上 |
| LBC-M2 | Minor | `7abfd6ef` 的说明把那一次的原因写在 Verified 下，没有推断一段 | 见"更正"；`d8e664b3` 的说明也写了一遍。核对 C：关上 |
| LBC-N1 | Nit | 05-lessons 草稿把"调用者自己的等待"算作同一个 ctx 结束触发的 | 改成"另一个同长的计时，与它们也没有先后"。核对 C：关上 |
| LBC-N2 | Nit | 连接池那一子条开了药方，本支同一个测试没照做 | `9079f9af` 照做（随 LBB-M2 关上）。核对 C：关上 |
| LBC-N3 | Nit | 第 11 节草稿重说了决定 586，号也没接在 588 之后 | 决定只定本支自己的；"同决定"补 580；去处与收口并成一条。核对 C：关上 |
| LBC-N4 | Nit | `7abfd6ef` 说明第 19 行 121 个字符 | 已推，不改写；记在"更正" |
| LBC-Q1 | Q | 05-lessons 放第 3 节"交错与锁"，不放第 1 节？ | 采纳（决定 597） |
| LBC-Q2 | Q | 文档提交之前先合 `main`？ | 采纳：先合 `main`，再提交文档（决定 597） |
| LBA-C1-N1 = LBC-C1-N1 | Nit（核对第 1 轮） | "负载下到过毫秒级"是由"那一次就是 `expire` 排晚"这个推断推出的，却写在核实的一侧 | 本记录挪进"推断"，决定 595 与第 13 节一行照改；`d8e664b3` 说明的那一句见"更正" |
| LBC-C1-N2 | Nit（核对第 1 轮） | 草稿的反向对照表里 M2 在 `9079f9af` 一格写"红"，其实没跑 | 本记录的表照实写"没跑" |
| LBC-C1-N3 | Nit（核对第 1 轮） | 第 12 节草稿说 `main` 的 P2 一行 S2 还是待填，已过时 | 照 `main` 写：放在 S2 `6505c3bf` 之后 |
| LBC-C1-N4 | Nit（核对第 1 轮） | 结论没写去重；审查 C 的门禁是三项加本测试，不是"四项"；门禁表缺核对第 1 轮 C 那一行 | 本记录照改 |
| LBC-C1-N5 | Nit（核对第 1 轮） | `d8e664b3` 说明"f.event drops the new flight's RunAcquired on the way"说得过满 | 见"更正" |

## 主持者的决定

负责人可以改判；正文在 [00-M12](../00-M12-design.md) 第 11 节"2026-11-18：lint 忙的测试的旁支的决定"。开旁支、不记成已知偶发，已由决定 586 定。

- **595 改测试、不改产品：等被认定的那件事自己的事件，不等旁证；不加长等待**：`RunExpired`（实施）、旧 worker 的 `RunFinished`（LBA-M1），池的连接数限时轮询（LBB-M2）；照常审查、合并（同决定 356、359、541、548、564、580）。
- **596 transfer 心跳的取消查明、不改，记成潜在的先后问题，不是已知偶发**（LBB-M1，取 B 的 (b)）：M12 收尾之前它在 CI 或本机门禁上红一次，就开旁支给 transfer 的作业加一个只供测试的观察点（仿 lint 的 `RunObserver`），`cancelSeen` 改等它（同决定 568 的写法）。
- **597 文档的去处与收口**：第 12 节放 P2 一行（同决定 546、569：放在见到它的那一行）；05-lessons 第 3 节首条之下加子条（LBC-Q1）；记录用本名；文档提交之前先合 `main`（LBC-Q2）；修复核对第 1 轮之后不开第二轮；已推的提交说明不改写，不准处写在"更正"（同决定 547、554、571、579、583、588）。

## 实施中定的

各条主持者接受，不另编号。

1. 等事件的一行放在 `AcquiredConns() != 1` 那一判之后、bob 发起之前，紧挨着它保护的那一步；P2S2C 的探针放在 `select { case <-unwinding }` 之后，两者之间只有那一判，等价。
2. 不改断言与信息：bob 新建（改后 `:431`）、carol 加入（`:454`）两处照旧。
3. 连接数的轮询每 5 ms 一次（同文件 `waitSleep` 的 5 ms）、到 `interleavingWait`；先读一次再进循环，失败时报最后读到的数。
4. 把池的销毁推后，用的是 `go.mod` 里临时的 `replace`，指向一份在 `destroyAcquiredResource` 开头睡 300 ms 的 puddle v2.2.2；跑完从工作树外的备份复原 `go.mod`。`go test -overlay` 不能替换 GOMODCACHE 下的文件（go 命令拒绝），先试过、作废。同样 `replace` 到一份没改的拷贝，测试照过：红是睡眠造成的，不是 `replace` 本身。
5. 修复第 1 轮拆成两个提交：一个管事件的先后，一个管池的计数，各自单独为绿。三个提交的后缀都写 `(M10 story, found in M12)`。

## 接受与推后的

- **接受**：
  - 新 flight 的 worker 有自己的 2 秒，从 bob 发起算，要盖住 carol 的探针 150 ms、旧 worker 的回滚与还连接、新 flight 的取槽与 Prepare：是计时的余量，不是先后（审查 A、B 核过）。本机每次整个测试 3.1–4.5 秒。
  - `interleavings_lint_test.go:353-355` 在回滚失败时会走异步的 `Destroy`；回调 panic 时没有语句在跑，回滚照常成功，没造回滚失败。
  - pgxpool 的析构等 `CleanupDone` 最长 15 秒，轮询的时限是 10 秒：只有 PostgreSQL 十秒以上不理 `CancelRequest` 或 `Terminate` 时，轮询才会先到时限；本测试里异步关闭从 `unwinding` 就开始，3.8–7.0 ms 做完（审查 B 第一轮的探针；核对第 1 轮 B 提的边界）。
- **推后**：transfer 心跳的取消（决定 596）。
- **未核实**：
  - transfer 心跳那一处在 CI 上多久碰上一次；`cancelSeen` 的三处 upgrade 调用只核到用了它，没有逐个读断言（审查 B）。
  - 负载下 `expire` 到底晚了多少，puddle 的 goroutine 能晚到多少；交错测试之外的组合根测试没有逐份扫"等到旁证"。
- **不改的文档**：
  - M10 的 P3 设计 `:966`、`:1040`："过期 flight 不再 join"仍成立，改的是测试怎样看到"已过期"。
  - `v0.1-design.md`：第 13 节的长期约定没变。13.4 第 4 条写的是"确定地等到对方……不靠 sleep"，本支的改法是它的一个落实，没有新的帮手；审查 C grep 过"交错""事件"，没有关于等事件的约定要改。写法的经验进 05-lessons（同决定 570）。
  - P2 设计：这条旁支不属哪个 Phase，没有"结果"可写；P2/S2 记录只加一处后注（见"更正"末条）。
  - 00-M12 第 10 节的风险：只红过一次、已经修了，同决定 360、570 不加。
  - 移交：没有与它有关的 `open` 事项。
- **备忘**：项目若以后给池设 `AfterRelease`，每次 `Release` 都变成异步（`pgxpool/conn.go:57-66`），清单里"`Release` 同步"的几行都会落进本类（审查 B）。

## 更正

已推送的说明不改写（决定 597），在这里写对：

- `7abfd6ef`：
  - 第 2 段末句"When expire's goroutine ran late, bob joined flight 1 …"放在 Verified 之下，是推断：本机没有复现原来的那一次，是由信息一字不差与 300 ms 的探针推出的（LBC-M2）。
  - "well under a millisecond"没有量过，也不是界：`expire` 平常在 0.1 ms 之内，测试从 `unwinding` 到 bob 拿到 `r.mu` 要 1.3–5 ms；照推断，那一次 `expire` 比这还晚（LBA-M2 = LBC-M1）。
  - 第 19 行 121 个字符，重排版时两行接成了一行（LBC-N4）。
- `d8e664b3`：
  - "Corrections to 7abfd6ef's message"第 2 条末句"In the failure expire came later than that"是由上面的推断推出的，也是推断（LBA-C1-N1 = LBC-C1-N1）。
  - "f.event drops the new flight's RunAcquired on the way"：放开 `release` 之后，flight 2 的 `RunAcquired` 与 flight 1 的 `RunFinished` 谁先不定（审查 C 的时间线两种都见到）；`RunAcquired` 先到时才在这里丢，否则由之后的 `nextScopeEvent` 丢。两种都没有谁读它（LBC-C1-N5）。
- `9079f9af` 的细化："the test failed before this commit at that check"跑在 `7abfd6ef` 上（红在 `:465`），不是父提交 `d8e664b3`；两者这一段相同（核对第 1 轮 C）。
- [P2/S2 的记录](P2-S2-text-review.md)决定 586 的"对产品是窗口在亚毫秒级的一次多余的 503"：亚毫秒是平常的情形（`expire` 平常在 0.1 ms 之内）；照推断，那一次在负载下到了毫秒级。那一行加了后注指向这里（同决定 481 的写法）。

## 反向对照与变异

都经 `heavy.sh`，竞态检测，`-run '^TestLintBusyReturnsBeforeCleanup$'`；改文件的由实施者的小脚本改、跑、从工作树外的备份复原并核对 sha256。"慢 finish"是 `finish` 在 `f.stopExpiry()` 与 `r.mu.Lock()` 之间睡 300 ms；"慢销毁"见"实施中定的"第 4 条。

| # | 改了什么 | `5154e0de`（改前） | `7abfd6ef` | `d8e664b3` | `9079f9af` |
|---|---|---|---|---|---|
| 0 | 不改 | 过 | 过 | 过 | 过（实施者；B 3/3） |
| a | 产品：`expire` 开头睡 300 ms | **红** `:427`（实施者；B 在 `main` 上 3/3） | 过（实施者；B 3/3） | 没跑 | 没跑 |
| c | a，再去掉等 `RunExpired` 的一行 | — | **红** `:427` | 没跑 | 没跑 |
| M1 | 产品：`expire` 不 `forget`（S61 第二种） | 没跑 | **红** `:431` | 没跑 | 没跑 |
| S61-1 | 产品：取到槽后过期就还槽，`finish` 不还（S61 第一种） | 没跑 | **红** `:439`（A） | 没跑 | 没跑 |
| M2 | 产品：`forget` 不看是不是自己（S61 第三种） | 没跑 | **红** `:451`（实施者、A） | **红** `:454` | 没跑 |
| M2s | M2 加慢 finish | 没跑 | **活**（A；实施者 1/1） | **红** `:454` | **红** `:454`（实施者；核对 A） |
| Fs | 只慢 finish（安慰剂） | 没跑 | 过（A） | 过 | 没跑 |
| Dp | `replace` 到没改的 puddle（安慰剂） | 没跑 | 过 | 没跑 | 没跑 |
| Ds | 慢销毁 | 没跑 | **红** `:465`（retained a connection） | 没跑 | 过（实施者；核对 B 3/3） |
| M3 | 测试的快照包装：已关的连接 15 秒之后才 `Release` | 没跑 | 没跑 | 没跑 | **红** `:476`（`acquired=1`，10 秒之后） |

- lifecycle 的探针（`wiki_lint_lifecycle_test.go` 的 `after` 里读连接数，`-run '^TestTheLintStopsWhenAppRunEnds$'`）：九个子测试都读到 0，测试过。
- transfer 心跳的探针（审查 B，`job_run.go:156` 之后读到取消就睡 300 ms，`-run '^(TestCancellingABackgroundJob|TestCancellingARunningImport)$'`）：不改全过；拖慢之后恢复、撤销、导入全红，作业都成了 `succeeded`。

## 核对

**修复核对第 1 轮**（2026-11-18，三位挪到 `9079f9af`，看 `7abfd6ef..9079f9af` 与文档草稿）：

- **A**：LBA-M1、M2 关上。M2s 在 `9079f9af` 上红在 `:454`；沿代码走，新的一行等到的只能是旧 worker 在 `forget` 之后发的 `RunFinished`，不加长等待，`finish` 不发时以 "no actual run event" 具名地红。新 Nit 1 条（LBA-C1-N1）。
- **B**：LBB-M1、M2、N1、N2 关上，没有新的发现。在 `9079f9af` 上基线 3/3 过、慢销毁 3/3 过（同一份 puddle 拷贝，`diff -r` 只多一行睡眠；`go.mod` 从备份复原，核过 sha256）。
- **C**：第一轮 8 条里 7 条关上，LBC-Q2 采纳、在本提交之前做。`d8e664b3`、`9079f9af` 各自 build、vet、`make lint-go`、`make test-go-short` 与本测试 `-race -count=10` 全绿。新 Nit 5 条（LBC-C1-N1 = LBA-C1-N1，C1-N2–N5）。另更正 C 自己第一轮的一处：00-M12 的决定 586 里没有"亚毫秒"，只有 P2/S2 记录的那一行有。

没有 Important、Minor，不开第二轮（决定 597）。

## 门禁与 CI

- **门禁**（本机 arm64，经 `heavy.sh`，连着的几项写成一个脚本经一次 `heavy.sh` 跑完，每项之前判 `df`）：

  | 头 | 谁 | 结果 |
  |---|---|---|
  | `7abfd6ef` | 实施者 | `go build ./...`、`go vet ./...` rc=0；`make lint-go` 两处 `0 issues`、`go mod tidy -diff` 两处干净；`make test-go-short` 94 个包 `ok`（54 秒）；本测试 `-race -count=20` 20/20；`make test-go-bootstrap-4` rc=0（203 秒） |
  | `7abfd6ef` | 审查 C | build、vet、`make lint-go` 三项绿；本测试 `-race -count=10` 10/10 |
  | `d8e664b3` | 实施者 | build、vet、`make lint-go`、`make test-go-short`（51 秒）绿；本测试 `-race -count=20` 20/20（每次 3.10–4.54 秒） |
  | `9079f9af` | 实施者 | 同上（`make test-go-short` 52 秒；`-race -count=20` 20/20，每次 3.10–4.48 秒）；`make test-go-bootstrap-4` rc=0（208 秒） |
  | `d8e664b3`、`9079f9af` | 核对第 1 轮 C | 各自 build、vet、`make lint-go`、`make test-go-short`（54、53 秒）绿；本测试 `-race -count=10` 10/10 |

  golangci-lint 是主克隆 `bin/` 里的 2.14.0，没有下载。`wiki_lint_lifecycle_test.go` 没改，它那几份组合根没有另跑。
- **分支的 CI**（普通推送，没有改写已推的历史）：

  | 推送 | 头 | `ci.yml` | 结果 |
  |---|---|---|---|
  | 1 | `7abfd6ef` | 44279808743 | 第一次 attempt 18 项绿（`bootstrap-4` Test 5:22）；`e2e` 卡在"Install the Playwright browsers"（apt 125 MB 用了 19 分 16 秒，108 kB/s），到 20 分钟的作业时限被取消。主持者只重跑 `e2e`，第二次 attempt 绿（9:21，E2E 一步 7:42） |
  | 2 | `9079f9af`（`d8e664b3` 一起推送） | 44292935807 | 19 项第一次 attempt 全绿：`short` 3:08（Test 2:50），`race` 9:31，八份组合根的 Test 4:07–5:08（`bootstrap-4` Test 4:50），costs-3 12:45，`web` 3:56，`e2e` 9:46（E2E 一步 8:20），`image` 1:19 |

- **本提交自己的分支 CI**：（待填：本提交的分支 CI）。旁支合并为（待填：合并号）。

## 环境

实施在工作树 `nerve-wiki-m12-lintbusy`；审查与核对各在分离的工作树里，不碰实施者的工作树。重的都经 `heavy.sh`，每项之前判 `df`（实施者 30–34G，审查者 29–33G）；对照与变异从工作树外的备份复原并核对 sha256，跑完各工作树干净，没有 `*.mutbak`；`go.mod` 的临时 `replace` 也由备份复原。`TMPDIR`、`GOTMPDIR` 与日志在 `~/.cache/m12-lintbusy*`，用完删掉；没手起容器。本机 arm64（Parallels 虚机）。审查 C 与主持者经 `gh` 只读 CI；`e2e` 的重跑由主持者做。

要留的在设计工作目录的 `m12/lintbusy/`（`impl-report.md`、`review-*.md`、`fix-1.md`、`fix-1-report.md`、`check1-*.md`，与日志摘要 `logs/`）。
