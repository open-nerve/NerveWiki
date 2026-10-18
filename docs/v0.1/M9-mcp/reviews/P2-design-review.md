# M9/P2 写的工具与历史的读设计：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | [02-P2-writes-history.md](../02-P2-writes-history.md)（初稿，2026-10-18 13:13），对照 `main` `2a888a6f` 的代码（与 `0c934bb9` 相同，只多了 P1 的 S3 的文档）、[00 号文档](../00-M9-design.md)、P1 照建成的样子、`v0.1-design.md` 第 13 节、SDK v1.8.0 与 go1.27.1（模块缓存，只读） |
| 审查方式 | 三位审查者（Opus）并行，只读：<br>A：边缘、错误、归属与大的请求体的界线（1；2.1 第 2、4、6、12、14、16、18 条与 2.2；3.2、3.3、3.4、3.10、3.11.5、3.12；3.13 与 3.17 的码的表、矩阵与 `after`；3.14、3.18、3.19、3.21；6.3–6.5；作者的矛盾 2、5、6、8、9、10）。以 httpserver 那样的链（恢复 → 期限 → `MaxBytesReader` → 名额 → bodyshape 那样的 `ReadAll` → 处理函数）做了名额的原型，在 HTTP/1.1 与 HTTP/2、Go 的客户端与裸的 TCP 客户端上跑（`review-A/largebody`）；SDK v1.8.0 的无状态处理在进程里、客户端中途取消（`sdkwait`）；3.10 与 3.11.5 的错误在最坏的取值处照 mcpserver 的量（`sizes`）；`%q` 转义的字符（`quote`）；说明的量（`instr`）；经 `heavy.sh` 一次 postgres:18.6 的容器，五种区域设置，00044 的 Up 与 Down 与逐码位的扫描（`pgcheck-results.txt`）；3.18 的两段 JSON。<br>B：写的语义、单元、锁与链接（1、2.1、2.2、3.5–3.9、3.11、3.13 的交错与锁的格、3.20、linking 的 `Relocation`）。改名的 harness 对照一步与 REST 的两步（`review-B/exp-linking`，`fix.diff`）；M8 的 `PlanRevert` 对一个改名的 MCP `move` 的变更集（`exp-revert`）；对 Go 的 Rabin–Karp 构造的 `old_string`（`rk`）。一次 `go test ./internal/modules/linking/...` 在草稿区的副本里带进了 `linking/adapter/postgres`，在 `heavy.sh` 之外起了 testcontainers，Ryuk 一分钟内收走，别人的东西没有碰到。<br>C：历史的读、测试、e2e、Step 与写法（0、1、2.1、2.2、3.15–3.17、第 4 节的 MC3、MC4、C4（MCP）、MC1 第 5 步、MC8、e2e 的时间、第 5–8 节、作者的矛盾 4、8、11、12）。3.18 的 JSON 并进 `tools.json` 核对形状（`review-C/toolsjson_check.py`）；重跑作者的 `exp/sizes/worst.py`；量 P2 与 REST 真写得出的数据的答复（`realdb_sizes.py`）；CI 与组合根的时间取自 S3 的最后一次运行；抽查 2.1 引的 30 处 00 号文档的行号。不需要 Docker |
| 日期 | 2026-10-18 |
| 结论 | A：Important 1、Minor 8、Nit 11，过度设计 1，另对作者的六个矛盾下了结论。B：Important 2、Minor 7、Nit 7。C：Important 1、Minor 14、Nit 14，过度设计 2，另有与 A、B 重叠的表。<br>全部处理，修订直接写进设计（草稿留作 `02-P2-writes-history.draft1.md`），与这份记录一起提交。处置由主持者定下（下面"主持者的决定"一节），负责人可以改判。<br>四个 Important 都按主持者的方向改了：每个凭证至多一个名额、满了的一行点名持有者、剩下的风险连同数字写进 00 号文档第 10 节与 M12 的移交、P5 推荐缓冲请求体的代理（A-I1）；单个节点的一步的放回从 P3 移进 P2，计划只核对最后的键，加单元测试与经 REST 同步、后台两种撤销的整个程序测试（B-I1）；`edit` 的查找、计数、替换与 CRLF、NFC 的再找都经一个线性的匹配器（KMP），重量了最坏的一次调用，守卫加 B 的构造串，写明期限由输入的上限管住（B-I2）；工具的字符串的检查以每个工具的合法参数到达用例、有反向对照（C-I1 与 A-M1）。<br>6.3 的变异由草稿的 41 条增为 59 条：加了 18 条（第 42–59 条，每条在下面的发现的表里随它的发现点名），C-M10 的四条（#3、#10、#22、#28）照改。修订交回之后，主持者答了作者的三个新问题（"主持者的决定"第 13–15 条），照改。<br>之后三位审查者（Opus）复核两次，设计共修订四轮（审查之后；主持者答了三个问题之后；两次复核之后各一轮）：第一次复核没有 Important，新的 Minor 8、Nit 21；第二次复核没有 Important 与 Minor，新的 Nit 11；全部处理（文末"第二轮复核"与"第三轮复核"）。6.3 的变异最后是 63 条。 |

## 发现与处置

节号是修订后的。

### A：边缘、错误、归属与大的请求体的界线

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| A-I1 | Important | 慢的发送者能无限期地占着整个服务的两个名额：名额在读请求体之前取、在任何权限的检查之前，请求体只受连接的读期限（30 秒）管；任何一个账户（读者、访客，开了注册时任何人）以两个声明 65,537 字节、一秒一个字节的 POST 每 30 秒重来一次，每分钟 4 个请求，按凭证的桶（每分钟 1,200 个）看不到；其间整个服务里超过 64 KiB 的 MCP 调用与超过 1 MiB 的网页保存都等 2 秒后 503，网页三次重试约 11 秒后失败；满了的一行只有路由与等着的数目，持有者的访问日志也不带凭证，所以没人认得出是谁。初稿以"要先有账户""按凭证限流照旧"接受了它，两个前提都不成立 | 照主持者的决定取 A 的四处改动：①一个凭证（限流的键 `pat:<id>`、`session:<id>`，取自认证放进 ctx 的那个，`api.go:277-279`、`:322`）同时至多一个大的请求体，它的第二个在 2 秒之内等它的第一个，别的凭证的不等它；②满了的一行带路由、这个请求的凭证、`own` 与两个持有者的凭证与路由；③剩下的风险连同数字（两个凭证仍占得满两个名额，每 30 秒两个请求，桶看不到）写进 00 号文档第 10 节的新的一行（`apply-00m9.py`）与 M12 的性能移交第 18 项（第 8 节、第 7 节第 7 条）；④P5 的接入文档推荐缓冲请求体的代理（nginx 的 `proxy_request_buffering on`），写明 Caddy 与直接暴露不缓冲（第 8 节）。`server.read_timeout` 不改。测试照 A 的行用两个 PAT（`TestTwoLargeBodiesAreServedAtOnce`、`TestACredentialsSecondLargeBodyWaitsForItsFirst`、`TestTheSaturatedLineNamesTheHolders`；整个程序的 `TestTheLargeBodiesShareTwoSlots` 用三个 PAT，第 5 步是"自己的第二个不拿空着的"）；变异 46、47（3.2.1、3.2.2、3.2.5、3.21、6.3；2.1 第 6 条） |
| A-M1 | Minor | `TestToolStringsDoNotAnswerInternalErrors` 对 P2 的工具是空过的：参数只有被测的那一个，缺了必填的成员是结构表的 `bad_request`，它也算"不是 `internal_error`"；`write`、`edit`、`move` 的每个参数与 `cursor` 的 NUL 都到不了用例 | 同 C-I1（主持者合在一起处理）：见 C-I1 |
| A-M2 | Minor | `%q` 不是逐字节地写路径：`strconv.Quote` 转义 `IsPrint` 为假的字符，标题里能有 U+3000、U+00A0、U+200D、U+00AD 与私用区的字符，文字就不再是路径，agent 照抄得到转义；`page.not_found` 只在文字里有路径 | 路径一律以 `"` 加路径加 `"` 原样写出（`domain.Quoted`），P2 的文字与 P1 的 `Nothing is at %q`（`text.go:22`）、四个提示（`path.go:123-147`）一起改；`text_test.go` 以这四种字符的标题核对。复核了字节的说法：原样引起不比 `%q` 长，`&` 仍是最贵的 6 字节，3.10.3 的界线照旧成立；变异 57（3.10.3、3.10.4、3.11.5、3.21；2.1 第 2、4 条） |
| A-M3 | Minor | REST 的契约的说明（`createPage`、`putPageContent`："先查正文"，`server_busy` 只因解析的预算）与 v0.1 的 13.1 第 4 条（`/mcp` 的边缘、正文写的"422 → 404 → 403 → 503"）留着旧的次序 | 两个说明加一句"A body over server.max_body_bytes waits up to 2 seconds for one of the server's two large-body slots, at most one per credential, then is server_busy, before any other check."（3.2.3）；13.1 第 4 条列进第 7 节第 4 条（`/mcp` 的边缘多两步、REST 的两个正文写的平台的 503 在正文的检查之前，同 C-M12） |
| A-M4 | Minor | 锁的路径的读失败、`edit` 的预检交给了实施者：读失败时用例答它的错误就把 `page.locked` 变成 `internal_error`，答裸的锁的错误就丢了路径、持锁人的字句与 `you`；`edit` 的预检在 `Run` 之外，只包 `Run` 的拒绝，而 C4（MCP）第 1 步要的正是预检的 `lock.path` | 写了两条规则（3.8"锁的拒绝的路径"）：读本身失败时记 warn"lock paths not read"、答 `LockPaths{Err, Paths: nil}`，每个路径为 null，拒绝照旧是 `page.locked`；`edit` 的预检的拒绝照 `Run` 的包上这一页的路径（3.8.2 第 4.6 步）。用例的表各一行（3.8.6），变异 54、55（同 B-M2、C-M9） |
| A-M5 | Minor | 适配器认不出 `NeedsRecursive`、读不到它的计数：它是一个答 422 的函数，而文字要"3 pages and 2 attachments" | 改为类型 `domain.Recursive{Pages, Assets}`，`Unwrap` 是 `recursive` 的 422，适配器以 `errors.As` 读计数（3.8.4、3.8.5、3.10.4） |
| A-M6 | Minor | 上限的测试从不在最坏的取值处量适配器真的文字：mcpserver 的测试造不出 page 的文字，`text_test.go` 只看字句 | 加 page 的 `adapter/mcp` 的 `TestEachErrorFitsAtItsWorst`：3.11.5 的每一行以最坏的取值建出（2,818 字节的 `&` 的路径；25 个锁，显示名各 100 个 `&`，页的路径各是 10 层的 `&`），照 mcpserver 的量（文字的 `TextBytes` 加不带 HTML 转义的 `{"error": 成员}` 的 `TextBytes`，经 mcpserver 导出的同一个函数），都在 48 KiB 之内；A 量的 `Build(0)`（33,818、38,534、37,922、34,224 等）记进 3.10.5 |
| A-M7 | Minor | `after` 不计数，不调它的 runner 也通过；3.21 的反向对照因此不成立 | `after` 照 `check` 计数（`toAfter`、`aftered`，`t.Cleanup` 里不等就失败），REST 的 `after` 同样计数（3.13.2）；变异 51、3.21（同 C-N1） |
| A-M8 | Minor | 两个名额时看不见一个漏掉的名额：每条路走完只查"下一个大的请求不等"，漏了一个另一个照样空着，变异 3（panic 不放回）过；整个程序的第 7、8 步同样；关闭的那一行用的是没有长度的请求体，客户端在读完前缀之前关掉就根本不取名额 | 照主持者的例子：`TestTheSlotComesBackOnEveryPath` 的应用只有一个名额，每条路进行中时另一个凭证的探针请求等满、503（证明它持着名额），结束之后新的请求不等就拿到（凭证的名额也还了）；关闭的一行改为声明过门槛的 `Content-Length`、发了一半就关。整个程序的测试里，客户端取消的一步在另一个 `hold` 持着名额时做（新的第 6 步）；`boom` 的一步去掉（它走不到名额的 panic 的路径，C-M10 #3）。6.3 的第 3 条只列 `TestTheSlotComesBackOnEveryPath`（3.2.5、6.3） |
| A-N1 | Nit | 前缀以 `make([]byte, threshold+1)` 先分配，短的分块请求也占 1 MiB；"等着的请求还没读请求体"对分块的不对 | 以 `io.ReadAll(io.LimitReader(r.Body, 门槛+1))` 读前缀；写明等着的分块请求持着它的前缀（3.2.1） |
| A-N2 | Nit | HTTP/1.1 上等着的请求看不到客户端走了（请求体没读，net/http 没有后台的读），不该期望 499 | 写明只在 HTTP/2 上或期限到时提前结束，测试不期望 499（3.2.1） |
| A-N3 | Nit | "大于它的只有这两个路由与 /mcp"漏了 `API.Stream` 的路由；"S1 带来第一批真的大请求体"不对：P1 的 `list`、`read` 已收 37,814,272 字节的请求体，36 MB 的 `path` 在 `ParsePath` 拒绝之前被检查、解码 | 改为"把请求体读进内存的路由里"（3.2.1）；第 0 节"界线为什么在 S1"改写：P1 的调用今天就进得来，S1 一并管住（P1 的 S1 审查 A 量的 1,381–1,471 MB 正是这一种） |
| A-N4 | Nit | MCP 的删除里 `lock` 与 `locks[0]` 重复路径，至多多占一个锁的位置 | 取 A 的可选的改法：`locks` 在时 `lock` 不带 `path`（3.10.4、3.11.5；2.1 第 2 条） |
| A-N5 | Nit | `Extra` 能盖过文字里也写着的 `fields`、`retry_after_seconds` | `Extra` 只能加问题没有的成员，另外可以盖过 `lock`、`locks`、`locks_count`；`code`、`message`、`fields`、`retry_after_seconds` 总是 mcpserver 的（3.10.1）；变异 48 |
| A-N6 | Nit | 加两个变异："不等，立即 503"、"`Extra` 盖过 `code` 或 `message`" | 加了：6.3 第 45、48 条 |
| A-N7 | Nit | `write` 的说明说带 `base_version` 时"页变了就拒绝"，而内容相同时答 `changed: false`；说明的锁的一行漏了 `move`（改写被编辑的页的链接时拒绝） | `write` 的说明写上"内容已是这样时什么也不写、`changed` 为假，带着这一页过时的 `base_version`、页正被编辑时也是"（3.18，同 B-N6）；说明的锁的一行加"and a move that would rewrite its links"，重量：S1 之后 2,547 与 2,617、S2 之后 2,815 与 2,885（3.12） |
| A-N8 | Nit | 第 7 节第 4 条的"附件只给元数据，图片照样内联"读起来像图片不是附件 | 用 README 的写法"图片照样直接放进答复，别的附件只给元数据"（第 7 节第 4 条、`apply-v01.py`） |
| A-N9 | Nit | `TestParseVersionToken` 的拒绝加 32 位十六进制的写法 | 加了（3.4） |
| A-N10 | Nit | `TestTheLargeBodiesShareTwoSlots` 三次等满 2 秒，约 6 秒在第二份里 | 测试的接线把等候降到 250 ms（`wiring.largeBodyWait`），2 秒由 `TestTheAPIConfigBoundsTheLargeBodies` 钉住（3.2.2、3.2.5）；6.5 记它约 2 秒 |
| A-N11 | Nit | "今天前两种答 internal_error"：`page.locked` 今天根本带不了 `lock.path`，只有第一种今天失败 | 改为"今天第一种答 `internal_error`；`page.locked` 今天根本带不了 `lock.path`"（3.10.5） |
| A 过度设计 | — | `TestTheClientChecksSpellUnshowable` 解析 00044 的 SQL 与 Go 比较拼写；一条查询扫每个码位比的是 PostgreSQL 自己的读法，每个库约 1 秒，也包含了抽样的反例 | 采纳：拼写的测试去掉，`TestChecksRejectCounterexamples` 对两张表各以一条语句（`pg_get_expr(conbin, conrelid)` 读出库里的表达式，套在 1–1,114,111、去掉代理项的 `generate_series` 上）与 `shared.Unshowable` 逐码位比较（3.3.2、3.21、6.3 第 9 条） |

A 对作者的矛盾的结论（都照作者的处理，记下）：2（`Extra` 被盖过）是真的，P2 的改法是干净的；5（`linking.pages_locked` 没有上限）在 P2 有了 `move` 之后是真的（70 个锁 51,348 字节，约 65 个就过），上限 20、`locks_count` 与 `Fit` 对；6（MC1 第 5 步写死）是真的；8（`public_url` 的文字）是真的，照 A-N8 的写法改（`apply-v01.py`）；9（P1 的 `page.not_found`）是 `main` 上今天的 bug：11 段的路径上带 `AttachmentHint` 67,590 字节、10 层的页上带 `PageHint` 61,448 字节，答 `internal_error`，P2 的 `Fit{N: 1}` 去掉提示后 33,818，也管住 `list` 的 `ChildrenHint`；修正随 S1，写进 2.1 第 4 条与第 9 节要记的数；10（`mcpRow` 没有 `after`）是真的，并照 A-M7 计数。

### B：写的语义、单元、锁与链接

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B-I1 | Important | 撤销一个也改名的 MCP `move` 可能答 500：`Drafts` 下有 `Plan` 与 `Plan v2`，`move Drafts/Plan v2.md → Final/Plan.md` 写出一个条目同带新的父节点与名称；撤销时 S\* 合法，计划却先"带着现在的名称移回 `Drafts`"，撞上那里的 `Plan`，`ErrRevertUnordered` 原样传出，网页 500，后台的撤销同样；初稿把修正留在 P3，P2 合并到 P3 合并之间 `main` 上一直可达 | 照主持者的决定：00 号文档 4.12 第 2 条里单个节点的部分移进 P2（3.5.4）。计划对既移动又改名的放回只核对最后的键 `holder(to.ParentID, TitleKey(to.Name))`（连同父节点活着、不在自己的子树里）；单元以 `moveSubtree` 加 `NodeWriter.RelocateNode` 一步写（父节点、次序值、名称一起改）；P3 只留多轮的调度、三条后备、临时名称、写回的 `Base` 与后台的切批。测试：B 的复现改成 `page/domain` 的 `TestARevertOfAMoveThatRenamesIsOnePlace`；`unit_revert_test.go` 看到一步；整个程序的 `TestARevertPutsBackAnMCPMoveThatRenames` 在两本笔记本里各做一遍，经 REST 同步地与后台地撤销，`Final` 下另有一页叫旧的名称；撤销的性质测试加"经 MCP 的移动加改名"的操作。00 号文档 4.12、第 7 节的 P2 与 P3 两行、"移交的落实"的 M8/P3 一行经 `apply-00m9.py` 改（2.1 第 20 条，3.20、第 8 节、6.3 第 52 条、3.21） |
| B-I2 | Important | `edit` 的匹配对构造的 `old_string` 是 O(n·m)：模式长过 `bytealg.MaxLen` 时 `strings.Index` 在几次误中之后走 Rabin–Karp，哈希是固定的 32 位多项式，三个字节就能让每个窗都撞、每次都比整个窗；5 MiB 的页上 120 KB 9.85 秒，按斜率 1 MiB 约 88 秒，`Replace` 还要找好几遍，重做再来一遍；没有东西看 ctx，处理函数跑过 45 秒的期限，同时占着大的请求体的名额；初稿的表与守卫都没有撞的形状 | 照主持者的决定（3.9）：查找、第二处、计数、`replace_all`、CRLF 与 NFC 的再找都经一个 KMP 的匹配器（`int32` 的回退表，一个模式一份），从不以 `strings.Index`、`Count`、`ReplaceAll` 拿 agent 的串作模式（`TestReplaceSearchesOnlyWithItsMatcher` 以 `go/parser` 核对，有合成源码的反向对照）；对照 `strings` 的差分 `TestTheMatcherIsStrings`（草稿区 257,204 例相同）。重量（`exp/kmp`）：最坏的一次调用是 NFC 的提示放大到 15 MiB 的那一种，本机 149–162 ms；B 的构造串 12 万字节 7.1–7.4 ms、1 MiB 9.0–9.1 ms；构造串在两端各一次 11.5–16.2 ms（`strings.Index` 与 `Count` 在 3 万字节就 3.01 秒）；每字节 1.05–1.30 ns。期限：输入的上限让匹配不必看 ctx（最坏约 0.16 秒，重做约 0.33 秒，CI 上约 1 秒，调用的期限 15 秒，留着 15 倍以上），下一步的 `parser.Decided` 与 `Run` 带 ctx。守卫 `TestReplaceStaysWithinItsTime` 加 B 的构造串（测试里对 `primeRK` 解出三个字节，一次与两端各一次），`strings.Index` 让它失败；6.2、6.3 第 42、58、59 条、6.5（守卫的一行）（2.1 第 5 条） |
| B-M1 | Minor | linking 的修正对，但"与 REST 的两步相同"有一种写法不对：带路径、以旧的末段作显示的 `[[A/x\|x]]`，一步得 `[[X\|X]]`，两步得 `[[X\|x]]`；harness 自己算 `recased`，不调 `domain.Relocation`，样例 047 抓不住变异 24，只去掉父节点的比较还会让 harness 在同名的纯移动上也设 `recased` | 3.5.3 与 2.1 第 21 条写明一步只用一次 M6 的显示规则、与两步在这种写法上不同；047 加 `[[a/x\|x]]`，一步的结果手写；改名的用例的 `recased` 取自 `domain.Relocation`（于是 047 抓得住变异 24）；整个程序里与 REST 两步的比较避开这种写法（3.13.5、3.13.6） |
| B-M2 | Minor | `edit` 的锁的预检的拒绝不带路径：调用之前就开着的编辑器是最常见的一种，C4 第 1 步与锁的第 2 格照写会失败 | 同 A-M4：预检的拒绝同样包上这一页的路径，没有单元要回滚 |
| B-M3 | Minor | `TestAWriteNeverLandsOnAnotherPageAtItsPath` 在变异 13 之下照样通过：M 写到版本号 2、令牌是 N:1，只比版本号的写法也答 409；危险的是 M 在令牌的版本号上 | M 以一次带正文的 REST 调用建成（M:1），之后不再写；断言"另一页"的文字；`TestAVersionOfAnotherNotebookWritesNothing` 写明两页都是版本号 1（3.13.4、6.3 第 13 条，同 C-M2） |
| B-M4 | Minor | `move` 的 `server_busy` 一格会挂住：持预算的 REST 写持着它那一本的笔记本行 `FOR SHARE`，同一本里的 `move` 要 `FOR NO KEY UPDATE`，等到调用的期限 | 那一格在另一本笔记本里做，写明理由（预算是整个进程的）（3.13.1） |
| B-M5 | Minor | 3.13.5 的三步排不成这个次序：第 1 步移走 A 之后，第 2、3 步还以为 A 在 `A.md`；移回也不行，指不到页的链接从不改写 | 次序照改：先做被拒（C 被编辑）与之后改写，`update_links: false` 在另一组页 D、E 上做（3.13.5，同 C-M8） |
| B-M6 | Minor | 附件改名可以换扩展名（`CheckAssetRename` 只要有一个扩展名，样例 035），MC4 第 3 步的 `flow.png → flow.jpg` 会成 | MC4 第 3 步改为 `→ Guides/Install/flow`（没有扩展名，422）；3.5.1、3.11.3、3.18 写"保留一个扩展名（可以换）"；换扩展名的一步放进 3.13.6 的整个程序测试（4.3；2.1 第 15 条） |
| B-M7 | Minor | (a) 每一步的规则（缺的祖先以 `.md` 结尾、路过附件、太深、叶子的类型与内容、`from` 与 `to` 的类型、有子节点而没有 `recursive`）写在用例的 `do` 里，P3 的 `PlanBatch` 要再写一份；(b) `Unit.Relocate` 总把节点放在最后，放不回 S\* 的次序，不能给撤销用 | (a) 写成 `page/domain` 的函数 `CreationOf`、`MoveKinds`、`DeletionOf`（路径加找到的节点），用例在 `do` 里调，`PlanBatch` 在模拟的树上调，`steps_test.go` 每条规则一行（3.8、3.20）；读正文的哈希的那一步留在用例；(b) 3.20 写明撤销的放回走 `moveSubtree` 加 `RelocateNode`（3.5.4），不是 `Relocate` |
| B-N1 | Nit | "每个 write 至多约 40 条语句"不对：每个 `CreatePage` 约七条，十层都缺约 70 条 | 改为约 70 条，列出语句（2.1 第 1 条） |
| B-N2 | Nit | CRLF 的规则之下大小要以换过的两个串算 | 照改（3.9；2.1 第 5 条），`edit_test.go` 加 CRLF 的大小一行，变异 59 |
| B-N3 | Nit | `changed` 为假不只因 CRLF 的规则：读与单元之间别人的写恰好写成替换的结果时也是 | 照改（3.8.2 第 6 步、3.11.2），用例的表加这一行 |
| B-N4 | Nit | `ErrTitleTaken` 之后以 `RootByKey`、`ChildByKey` 分辨页与附件，只在 `titleFree` 的 409 之后安全；`RelocateNode` 的索引违反（缺陷）之后事务已中止，那次查找本身出错 | `Relocate` 由它读过的兄弟点名占着的节点（`*domain.PathTaken{Asset}`），不另查；用例只填上 `Path: to`（3.5.1、3.8.3） |
| B-N5 | Nit | 锁的第 9 格只持着一页，"只答第一个"与"全部"分不出；只有 `TestTheEditLockGuardsByOperation` 抓得住变异 28 | 同 C-M10 #28：新的第 11 格持着三页（3.13.3、6.3 第 28 条） |
| B-N6 | Nit | `write` 的说明的"页变了就拒绝"在内容已是这样时不成立，打开的编辑器也不拒同样的写 | 同 A-N7（3.18） |
| B-N7 | Nit | page/app 新的 `Relocation` 与 linking 的 `Relocation(changes)` 同名，同一段里都讲到 | 改名为 `Placement`（3.5.1、3.8.3、3.20） |

B 没有过度设计的发现。

### C：历史的读、测试、e2e、Step 与写法

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C-I1 | Important | `TestToolStringsDoNotAnswerInternalErrors` 对 P2 的多数字符串参数是空过的，对 P3 的全部也会是：参数只有被测的那一个，缺了必填的成员就是结构表的 `bad_request`，断言只要"不是 `internal_error`"；18 个新的字符串参数里 11 个的 NUL 到不了用例（`write` 的三个、`edit` 的四个、`move` 的两个、`versions` 与 `changeset` 的 `cursor`）；P1 第 8 节说"结构表与 NUL 的推导已覆盖嵌套"，而 `ops[]` 的每一项有必填的成员 | 照主持者的决定（同 A-M1）：测试有一张每个工具的合法参数的表（`validToolArguments`；`tools/list` 里有而表里没有的工具让它失败，P3、P4 必须加上自己的），每个字符串参数只在合法参数的一份拷贝里换成 NUL（`withArgumentAt`，嵌套的换第一个 `op` 里的），断言答复不是 `internal_error`、也不是结构表的拒绝（以 `mcpserver/errors.go:126` 的那段文字起头）。反向对照 `TestToolStringsCatchASchemaRefusal`：带接缝工具 `strict` 的应用，表里它的一项缺必填的 `must` 时检查报出、表里没有它时也报出；结构表的拒绝的文字若改了，反向对照失败。覆盖 P2 的每个工具（S1 加四项、S2 加三项），P3 的 `batch` 继承它，写进第 8 节。3.13.1 的"不用改代码"去掉，3.21 加一行，6.3 加变异 49（`EditPage` 不查 `new_string` 的 NUL）与 50 |
| C-M1 | Minor | CI 的余量：组合根两份真正卡住的是 go test 的 `-timeout 15m`，第二份已过一半（S3 的一次 7:54）；初稿的 6.5 拿 8:16 比 25 分钟，规则从不触发；P1 第 8 节交给 P2 的这一项初稿的表里漏了 | 照主持者的规则（6.5、5.1 第 9 步）：S1 以每份单独跑两次的平均与 CI 的测试一步判断，较慢的一份的 CI 测试一步过了约 7.5 分钟就按测试名分出第三份（`BOOTSTRAP_THIRD`、`test-go-bootstrap-3`、`ci.yml` 的 `bootstrap-3`），不提高 `-timeout`；按名字算了 P2 的测试落在哪一份、各加多少（第一份约 7:05–7:15，第二份约 7:25–8:10，多半要分），写了 Makefile 与 `ci.yml` 的形状与界线怎样选（候选 `^Test(The[F-Z]\|T[i-z]\|[U-Z])`，两份约 124 与 120 秒）；守卫那一份做同样的检查（测试一步约 42% 的 25 分钟，P2 不加守卫）；第 2 节的 P1 第 8 节的表加这一行。修订之后主持者另定了第一份过线时的挪法与成本、守卫的行的拆法（主持者的决定第 13、14 条） |
| C-M2 | Minor | `TestAWriteNeverLandsOnAnotherPageAtItsPath` 在变异 13 之下照样通过 | 同 B-M3 |
| C-M3 | Minor | `TestTheMCPHistoryAnswersGoOn` 以 P2 与 REST 写得出的数据到不了 `log` 的退回（要三项各带五个长名称的移动或改名，P2 的 `move` 只记移动的那一个节点），路径也太短，到不了 `changeset` 的退回（要至少 8 段长的） | 照主持者的决定以种下的库做（3.17.3 的 `seedLongHistory`）：`log` 的一种以 SQL 加两个变更集，各三个 `changeset_items`（X1–X3 从 P 移到 Q，节点与父页是经 MCP `write` 建的长名称的真节点），说明 4,096 个 `\x01`；`changeset` 的一种照 C 的建议是真的写：经 MCP 建的 10 层长路径上的一个改名，由 bob 经 REST 撤销，撤销的说明是请求的 4,096 个 `\x01`（`api/modules/page.yaml:899`）。量（`exp/sizes/seeded.py`）：53,299 退到 45,321；52,656 退到 29,332 加 23,500。6.2 改正：最坏的取值由替身上的适配器测试核对，真的库核对的是能写到的大小上的前进与不跳行；加变异 43 |
| C-M4 | Minor | `log` 的退回以"`changes` 少于 `min(3, node_count)`"表示省去，不对：`node_count` 只数条目，变化是条目与版本合起来的；13.1 第 31 条要标出截断 | 被截的一行加 `changes_cut: true`（只有退回写它，`omitempty`），`log` 的说明写它（3.16.2、3.18）；2.1 第 19 条、第 7 节第 4 条的 13.1 第 31 条随之；变异 44 |
| C-M5 | Minor | 节点的类型：REST 写 `asset`，P1 的工具都说 `attachment`；照抄就互相矛盾，照换则与 REST 的比较在第一个附件上失败，而备好的历史里也许没有附件 | MCP 处处写 `attachment`，历史也是，在适配器里换（3.16.3）；比较时把 REST 的 `asset` 当作 `attachment`；备好的历史里上传一个附件并经 MCP 移动它（3.17.3）；2.1 第 8 条、变异 56 |
| C-M6 | Minor | MC3 第 1 步拿 `getPage` 读出的 `Page` 去比库，是库与它自己比 | 每个 `Page` 由答复的 `created[i]` 建出（`id`、末段的名称、上一项的 `node_id` 或 null、笔记本、答复的 `at`），`expectNewPage` 的参数放宽为 `Pick<Page, …>`（4.1、4.2） |
| C-M7 | Minor | `expectRelocated` 没有目的地，拿条目的"后"比节点现在的行，又是库与它自己比 | `expectRelocated(db, id, from, to: { parentId, name }, moverId, at?)`，目的地由测试给出（Guides 的 id 取自 REST 的建），可另比变更集的时刻与答复的 `at`（4.1、4.3） |
| C-M8 | Minor | 3.13.5 的三步排不成这个次序 | 同 B-M5 |
| C-M9 | Minor | `edit` 的预检的拒绝没有路径 | 同 A-M4、B-M2 |
| C-M10 | Minor | 6.3 有四条列了不会失败的整个程序的一步，整个程序也从没看到两个锁：#3 的 `boom` 一步（mcpserver 在调用里接住 panic，走不到名额的 panic 的路径）；#10 的最后一跳第 6 步（`Call.Client` 到达时已清理过）；#22 的交错的带令牌的一步（重做读到 r + 1 之后令牌仍不符，答同样的 409）；#28 的锁的第 9 格（只持着一页）；`LockHolders.Of` 按页 id 排，替身以给的次序答，`locks` 的次序没人测 | 四条照改，只列能失败的：#3 只列 `TestTheSlotComesBackOnEveryPath`，#10 只列 `client_test.go` 的性质，#22 列 `EditPage` 的表（有 `Base` 时 `PageContent` 只读一次），#28 列新的第 11 格与 `TestTheEditLockGuardsByOperation`；3.13.3 与 3.13.4 写明为什么不列那几步。锁的第 11 格照 C 的设计：Beside 下 Notes2（bob）、移动之后建的 Late（bob）、`Notes2/Child.md`（alice 自己的 REST 会话），`locks` 是子树的次序 Notes2、Late、Child（页 id 的次序是 Notes2、Child、Late），`locks_count` 3，只有 Child 的 `you` 为真；之后结束三个会话，删除答 `pages` 4（3.13.3、6.3 第 53 条）；`move` 的 `linking.pages_locked` 的两个锁（本人的与 bob 的，页 id 的次序）在 3.13.5 |
| C-M11 | Minor | 没有测试抓得住跳行的 `log` 的退回：写成 `NextCursor` 就跳过这一页的第 1 到 n−1 行；适配器的测试只给一行，两种游标相同 | 两个测试都把最坏的一行放在至少两行的一页的最前；真的库上以 `limit: 2` 从头读完、与 REST 的全部比较；加变异 43（3.17.3、3.17.4、6.3） |
| C-M12 | Minor | 第 7 节第 4 条漏了 v0.1 的 13.1 第 4 条（七个工具的次序、`move` 的 503 在 409 之后、`/mcp` 的边缘的 413 与 503、`changeset` 只认这本笔记本的、`versions` 用错了模式的 422 在游标的 400 之前），也漏了第 8、21 条与 13.4 第 3 条 | 都列进第 7 节第 4 条；`versions` 的例外另写进 2.1 第 9 条，00 号文档 4.15 加这一句（`apply-00m9.py`） |
| C-M13 | Minor | 2.1 第 19 条留着 00 号文档 4.7 的"单项不会超过它"，而 `log` 的一行与 `changeset` 的第一页都会超过 | 同一条改 :164：改为"一项可能超过它：`log` 的一行与 `changeset` 的第一页各有退回（4.9）；别的工具的单项都不会"（2.1 第 19 条，3.16.4） |
| C-M14 | Minor | 内存的测量只量一个请求体，而要记下的界线是两个同时的 | 量满载：两个同时在上限处加一个等着的，两种组合（两个 `/mcp` 的；一个 `/mcp` 的与一个 REST 的），峰值写进第 9 节（6.2） |
| C-N1 | Nit | `after` 不计数 | 同 A-M7 |
| C-N2 | Nit | "四个新的测试"应是五个（MC3 有两个） | 改为五个（4.7） |
| C-N3 | Nit | "三个入口"应是四个 | 改为四个（3.15、3.17.4、5.2 第 2 步） |
| C-N4 | Nit | 2.1 第 16 条的"(4.7)"应是"(4.5)" | 改为"本文 4.5" |
| C-N5 | Nit | MC8 从没建 B 要改名的 Guides，也没说 B 在 Handbook 的角色 | A 以 REST 建 Guides；B 以 `joinAs(…, "member")` 加入，Handbook 以 `workspace_access: "editor"` 建，所以 B 是编辑者（4.6） |
| C-N6 | Nit | MC3 第 5 步的新正文没写，而第 7、8 步要恰好一个 `ship` 与两个 `- [ ]` | 写定：`# Plan\n\n- [ ] ship\n- [ ] tell\n\nUpdated by the agent.\n`（4.2） |
| C-N7 | Nit | 锁的格漏了 00 号文档 4.10 的表的两格：`edit` 被调用者本人的会话拒绝；`move` 的改写碰上调用者本人的会话 | 锁的第 5 格（bob `edit` 自己开着的 Notes）；3.13.5 的第 1 步里 B 是 alice 自己的会话（`you: true`）（3.13.3、3.13.5） |
| C-N8 | Nit | 6.4 的 `make test-go` 的括号里漏了 `test-go-guards` | 补上（6.4） |
| C-N9 | Nit | `tools.json` 在第 6 个提交变长，MC1 第 5 步的修正在第 8 个，其间 MC1 是红的 | 修正并进第 6 个提交，它在加工具之前与之后都通过（4.5、5.1 第 6 步） |
| C-N10 | Nit | 写明 S2 不加 SQL，以及哪些计划测试覆盖它的读 | 写了（3.16） |
| C-N11 | Nit | 第 1 节与 2.1 的"S3 的文档还没进来""S3 合并以来没改过"在 `2a888a6f` 已过时 | 基线改为 `2a888a6f`，2.1 写明 S3 的文档改了 00 号文档的九处、引的行没有挪（第 1 节、2.1） |
| C-N12 | Nit | `configs/config.yaml` 应是 `server/configs/config.yaml` | 改正（3.19、第 7 节第 6 条） |
| C-N13 | Nit | 2.1 第 12、18 条都没给要改的 00 号文档的字句 | 两条写明"00 号文档不改"，第 12 条更正的是 P1 第 8 节（2.1） |
| C-N14 | Nit | 第 7 节没列把第 8 节的 M12 各项写进 `M12-release/handoffs/M4-performance.md` | 列进第 7 节第 7 条（写进它的第 18 项） |
| C 过度设计 1 | — | `TestTheLargeBodiesShareTwoSlots` 的第 8 步（`boom`）显示不了它要显示的，第 5 步（分块）重复 httpserver 的测试、多等 2 秒；第 7 步（客户端取消）值得留 | 采纳：两步去掉，`/mcp` 的门槛走的是同一个中间件；取消的一步留着（新的第 6 步，A-M8 让它在另一个名额被持着时做）（3.2.5） |
| C 过度设计 2 | — | 3.13.5 第 3 步在笔记本的副本上以 REST 的两步再做一遍、比较正文：047 与 3.13.6 的跨父节点一步已与 REST 比较，B-M1 又说明它本来就不完全相同 | 采纳：去掉，只断言链接改写为指向 A2 与 `checkLinks`（3.13.5） |

C 对作者的矛盾的结论（都照作者的处理）：4（说明的 `batch` 一行留给 P3）对；8（`public_url`）照 README 的语序改（A-N8）；11（一次调用 6 MiB 的字符串）：37,814,272 = 6 × (5 MiB + 1 MiB) + 64 KiB，`old_string` 至多 1 MiB 之后 4.14 的"合计 6 MiB"成立；12（"到 48 KiB 就停"不前进）重跑证实，退回前进，另见 C-M3、C-M4、C-M13。

## 重叠

| C | A、B | 合在一处的处置 |
|---|---|---|
| C-I1 | A-M1（A 评为 Minor） | 以 Important 处理：合法参数的表、到达用例、反向对照（3.13.1） |
| C-M2 | B-M3 | M 以一次 REST 调用建在版本号 1（3.13.4） |
| C-M8 | B-M5 | 3.13.5 改了次序 |
| C-M9 | A-M4 的第二种情形、B-M2 | 预检的拒绝同样包上路径；A-M4 的第一种情形（读失败）另一条规则（3.8） |
| C-M10 #28 | B-N5 | 锁的第 11 格持着三页、看次序（3.13.3） |
| C-M10 #3 | A-M8 | 一个名额的应用；整个程序去掉 `boom` 的一步（3.2.5、6.3 第 3 条） |
| C-M12 | A-M3（边缘与 REST 的正文写） | 契约的两个说明加一句；13.1 第 4 条列进第 7 节 |
| C-N1 | A-M7 | `after` 计数（3.13.2） |
| C 的"固定的等候的 CI 代价" | A-N10 | 测试的接线等 250 ms（3.2.5） |
| — | A-N7 与 B-N6（`write` 的说明） | 同一句（3.18） |
| C 的矛盾 8 | A 的矛盾 8、A-N8 | `apply-v01.py` |

## 主持者的决定

主持者在修订的简报里定下的处置（作者照做；负责人可以改判）：

1. **A-I1**：取 A 的四处改动：每个凭证（限流的键 `pat:<id>`、`session:<id>`，取自 ctx）至多一个名额，同一个凭证的第二个大的请求体在 2 秒之内等它的第一个，别的凭证的不等它；满了的一行点名持有者的凭证与路由；剩下的风险连同数字（两个凭证仍以每 30 秒两个请求占满两个名额，桶看不到）写进 00 号文档第 10 节与 M12 的移交；P5 的接入文档推荐缓冲请求体的代理（nginx 的 `proxy_request_buffering on`），写明 Caddy 与直接暴露不缓冲。测试照 A 的行，用两个 PAT。A-M8：名额的测试要看得见一个漏掉的名额（例如测试的应用只给一个名额，或每条路之后断言两个名额都空），让变异 3 失败。`server.read_timeout` 不改。
2. **B-I1**：00 号文档 4.12 第 2 条里单个节点的部分移进 P2：既移动又改名的放回经 `RelocateNode` 一步，计划只核对最后的键；B 的 `review-B/exp-revert` 作单元测试；一个整个程序的测试经 REST 撤销一个 MCP 的移动加改名，同步与后台各一次，旁边有一个叫旧的名称的兄弟；00 号文档 4.12 与第 7 节的 P2、P3 两行经 `apply-00m9.py` 改，P3 只留多轮的部分。
3. **B-I2**：以线性的匹配器（例如 KMP）查找，计数、替换与 CRLF、NFC 的再找都用它；从不以 `strings.Index` 或 `strings.Count` 拿 agent 给的模式；时间的守卫加 B 的构造串（`review-B/rk`）；重量最坏的一次调用，写进 6.2；写明调用怎样守它的期限：匹配器按有界的间隔看 ctx，或输入的上限让它不必看，二者说明是哪一个。
4. **C-I1 与 A-M1**：`TestToolStringsDoNotAnswerInternalErrors` 有每个工具的合法参数的表，断言每次调用到达了它的工具的用例；反向对照：一个先被 schema 拒绝的工具必须让测试失败；覆盖 P2 的每个工具，P3 的 `batch` 继承它，写进第 8 节。
5. **C 的 CI 的余量**：组合根两份受 go test 的 `-timeout 15m`（`Makefile:178`）管，不是作业的 25 分钟。规则：S1 量两份（两次运行的平均）并读 CI 的测试一步；较慢的一份的 CI 测试一步过了约 7.5 分钟（go test 的 15 分钟的一半），S1 按测试名分出第三份（Makefile 与 `ci.yml` 的 `bootstrap-3`，照已有的两份的写法），而不是提高 `-timeout`；规则、Makefile 与 `ci.yml` 的形状、界线怎样选写进 6.5 与第 5 节；守卫那一份对它自己的时限做同样的检查。另：P2 加的每个守卫要说它并进已有的一行（那一行的和要远在 Go 默认的 10 分钟之下）还是单独一行，带数字；`edit` 的匹配器的守卫是纯 domain、不带竞态检测的，属于成本那一份，同 `TestTheWorstGlobsStopWithinTheirWork`。规划用的数是 `main` 最近两次运行（`39098913463`、`39096175708`）的：`bootstrap-1` 6:46 / 7:09、6:41 / 7:03，`bootstrap-2` 5:34 / 5:55、7:52 / 8:08，`guards` 10:33 / 10:51、9:47 / 10:09，`race` 8:04 / 8:24、7:04 / 7:24，`costs` 测试 3:58、6:22、作业 5:51、9:08，`e2e` 3:28、3:31。
6. **C-M4**：`log` 的截断加明确的标记（13.1 第 31 条），不用 `changes < min(3, node_count)`。
7. **C 的到不了退回的历史测试**：以种下的库（SQL 或建数据的帮助函数）造出大到两种退回都走到的变更集：三项移动或改名的变化，与八段长的路径；那个测试不必走真的写的路径；加 C 的跳行的变异。
8. **节点的类型**：MCP 处处说 `attachment`，历史也是；REST 的 `asset` 在适配器里换。
9. **A-M2**：路径原样引起，不用 `%q`；复核加的字节的说法。
10. **作者的矛盾 9** 是 `main` 上真的 P1 的 bug（`page.not_found` 带提示 67,590 字节、答 `internal_error`）；P2 的修法（放不下就去掉提示）对，随 S1，写进 2.1 与第 9 节要记的数。
11. **别的**：每个 Step 单独合并仍是绿的（lint、knip、工具清单、矩阵的覆盖含 `write` 与计数的 `after`、码的表、archtest、depguard；S1 里没有空过的检查）；6.3 的每个变异由一个能失败的、点名的测试抓住，改正 C 的四条（#3、#10、#22、#28），加一个看得到两个锁的测试，让 `locks` 的次序被测到；2.1、2.2 随之更新，2.2 保持为空，除非碰到 00 号文档第 11 节负责人的决定。
12. 其余的 Minor、Nit 与过度设计照审查者的建议改，作者认为建议不对时写明理由、另想办法（"未采纳"一节）。

修订交回之后，主持者答了作者的三个新问题（第二份简报；负责人同样可以改判）：

13. **第一份也过线时**：00 号文档第 11 节第 28 条的"不再动"防的是凭一次运行的摆动去挪界线，不是冻结；经 `apply-00m9.py` 改写它，留着它的来历（S3 审查 C-N4）。任何一份的界线（`BOOTSTRAP_FIRST`、`BOOTSTRAP_THIRD`）只凭两次运行的平均移动，而且只在两种时候：一份的 CI 测试一步过了约 7.5 分钟（go test 的 15 分钟的一半）；分出一份之后紧接着重新平衡。第一份过了线，那个 Step 把 `BOOTSTRAP_FIRST` 的界线往前挪、让它的末尾进第二份，再照同样的规则核对第二、第三份。界线、每份本机的两次运行与它们的平均、CI 的测试一步写进那个 Step 的审查记录。写进 6.5、5.1 第 9 步、5.2 第 6 步、2.1 第 22 条（新加的，改写第 28 条）与 2.2 的说明，"由主持者定"的两句换成这条规则。
14. **守卫的 markdown 一行过了 Go 默认的一半**：`^(TestTheAppsMarkdownCostsAboutItsSize|TestTheAppsMarkdownRendersCheckedHTML|TestTheAppsLinksAreWithinTheirBound)$` 本机约 151 秒。S1 把它的三个测试各单独跑两次、取平均，以 6.5 的 CI 与本机之比估计 CI 上的时间，说明用的是哪个比（6.5：守卫那一份的 2.4）；这一行的估计过了 Go 默认的每个测试二进制 10 分钟的一半，就在 `test-go-guards` 里照已有的写法拆成一个测试一行；单独一行仍过一半的那个测试，那一行加 `-timeout 15m`（同 `GO_TEST_RACE`），Makefile 的注释写明为什么与数。同一条"时限的一半"管 P2 碰到的每一行成本与守卫的行，含成本那一份 `edit` 的守卫所在的 page/domain 一行。写进 6.5 与 5.1 第 9 步，随 S1 的 CI 的提交。
15. **按凭证还是按账户**：同意按凭证。按账户、一个账户的两个 PAT 占满两个名额，照旧是记下的剩下的风险，交 M12 的压测（00 号文档第 10 节、M12 的移交）；不另改。

## 作者的更正

修订时作者自己发现的：

| # | 发现 | 处置 |
|---|---|---|
| 作者-1 | 初稿 3.6 的守卫的测试行写的页不在 `newLockFixture` 里（`edit_lock_test.go:37-55` 的树是 Top 下 Mid、Mid 下 Low 与 Side，alice 的会话在 Top、bob 的在 Low 与 Side） | 行照这棵树重写：`mcp:x` 的 alice 删除 Top 答 `Locks` Top、Low、Side（子树的次序），`web` 的同样的删除照旧被 bob 的 Low 拒（3.6） |
| 作者-2 | 两种锁的列表次序不同：`linking.pages_locked` 的 `locks` 来自 `LockHolders.Of`，按页 id 排（`lock_holders.go:49`）；MCP 的删除的是子树的次序（`lockedAll` 按改动的次序） | 3.11.5 的表写明各自的次序；锁的第 11 格测子树的次序，3.13.5 的第 1 步测页 id 的次序 |
| 作者-3 | 页 id 的次序靠 UUIDv7 在一个进程里单调：Go 1.27 标准库的 `uuid.NewV7`（`$GOROOT/src/uuid/uuid.go:205-262`，`unit_create.go:63`），所以"后建的"就是"id 大的" | 第 1 节写明；锁的第 11 格的 Late 在移动之后建，子树的次序与页 id 的次序才不同（3.13.3） |
| 作者-4 | Go 的父测试的耗时不含它并行的子测试（`exp/partime`：四个各 0.4 秒的并行子测试，go1.27.1 给父测试记 0.10 秒），按 `-json` 的 `Elapsed` 选界线会把矩阵算得太轻 | 6.5 写明矩阵另以它的格的第一个开始到最后一个结束计 |
| 作者-5 | B 的构造串只测了"没有"；放在正文的两端各一次时，草稿的写法（`Index`、从 i+1 再找、`Count`）在 3 万字节就 3.01 秒 | 守卫加"两端各一次、中间每个窗都撞"的形状，不带与带 `replace_all`，匹配器 11.5–16.2 ms（3.9 的表，`exp/kmp/twice_test.go`） |
| 作者-6 | 匹配器的"每个字节至多比两次"不精确：一个字节可以在回退里被比几次，界是摊还的 | 改为"扫过的字节合起来至多两次比较"；计数的每次从上一处的末尾接着扫、范围互不重叠，唯一的检查从 i+1 再扫至多多扫 `len(old)` 个字节（3.9） |
| 作者-7 | 说明的锁的一行加了 `move`（A-N7），初稿的量不再对 | 以 `exp/instr/s1-r1.go.txt`、`s2-r1.go.txt` 重量：S1 之后 2,547 与 2,617，S2 之后 2,815 与 2,885，4 KiB 之下还剩 1,281 与 1,211（3.12） |
| 作者-8 | `tools.json` 的四个说明改了（A-N7、B-M6、C-M4、B-N3） | 以 `exp/toolsjson/gen2.py` 重新生成、解析，核对两个入口除 `notebook` 之外相同（3.18） |
| 作者-9 | "到达了用例"要分得出结构表的 400 与用例自己的 400（坏的游标） | 以结构表的拒绝的文字（`mcpserver/errors.go:126` 的 `lead`）分辨；反向对照的 `strict` 管着这段文字不悄悄失效（3.13.1） |
| 作者-10 | 带了 `changes_cut` 之后的大小 | `exp/sizes/seeded.py` 与 `worst.py` 重算：不带变化的被截的一行最坏 35,602，带一项 43,650，两项 51,698 放不下；种下的历史 45,321（3.16.4、3.17.3） |
| 作者-11 | 初稿 6.3 的第 9 条列的拼写的测试随 A 的过度设计去掉了；第 5 条与第 3 条列的整个程序的第 5、8 步也去掉了 | 第 9 条改列逐码位的查询，第 3、5 条只列 httpserver 的测试（6.3） |
| 作者-12 | 第三份的界线：`T[h-z]` 起的一段太重 | 以 M9/P1 S3 的一次 `-v` 运行（第二份的 201 个测试）加 P2 的估计算：`The[F-Z]` 起约 124 秒、其余约 120 秒；`T[h-z]` 是 156 对 88（6.5）；`-run`、`-skip` 的三个表达式在 `exp/shards` 上核对过互不重叠、合起来是全部 |

## 审查者核对过、没有问题的

- **A**：名额的位置（认证与限流之后、bodyshape 之前，`BodyLimits` 里没有公开的路由，`LongLived` 与 `Stream` 的路由不经它）；分块的请求体（`ContentLength` 为 -1 时读前缀、前缀逐字节接回、`MaxBytesReader` 数前缀，限加一个字节答 413 并还名额）；声明的长度过上限立刻 413，Go 的客户端与先写的裸客户端都读得到；比声明的长的请求体躲不过名额；每条路都还名额（panic、客户端中途走、读超时）；SDK 的无状态处理在工具返回之后才返回，mcpserver 的 `AfterFunc` 让客户端走了时立刻结束（`call.go:55`）；空出的名额先给等着的；与解析的预算没有死锁；REST 的共用（网页对带 `Retry-After` 的 503 重试、两个正文路由声明了 `server_busy`、上传与导入不受影响、网页的 `createPage` 只发标题）；边缘的次序与中间件的次序一致。归属：清理过的名称经 `MCPClient` 总是 `Valid`；00044 在五种区域设置下拒绝的恰是 Go 的集合（78 个码位），今天的 `[:cntrl:]` 与 `unicode.IsControl` 一致；Up 与 Down 在 20 万行上安全（Up 9–16 ms）；最后一跳的第 3 步是真的重启。`ParseVersionToken` 的往返拒绝 Go 1.27 的 `uuid.Parse` 的每种非规范写法与 `Atoi` 的 `+1`、`01`。错误：`Extra` 盖不过 `code` 与 `message`；P = 16,843 是最贵的路径；每个错误的 `Build(0)` 量过都放得下（30,962–38,534）；锁的成员不漏任何东西。`tools.json` 的 14 个条目合 Go 的字段与 SDK 的注解。说明的量照作者写的。码的表的每个码都声明过。日志与配置。
- **B**：路径在单元的锁下说了算（`PathStore` 的语句在单元的事务里，每个改树的写锁笔记本行，与覆盖、编辑的 `FOR SHARE` 冲突，READ COMMITTED 之下等过之后读到提交了的树），文中问到的每种情形都成立；锁的次序与死锁（`move` 带 `update_links` 的五步，别的路径与它成不了环；解析的预算在单元之前排队，改写在单元里 `TakeNow`）；`write`（不经导入的缓存的 `Walk`、撞名的 409 分得清页与附件、内容相同答 `changed: false` 不掩盖冲突、NUL 与 5 MiB）；`edit`（重做安全，替换只从读到的字节算，不丢别人的写；"重叠也算"的唯一对 UTF-8 安全；CRLF 的规则与次序）；`move`（`to` 是完整的路径、改名与移动的动作按父段的键、只改大小写的两种、成环、一条语句、`update_links` 交到 `Moved`、503 在 409 之后）；`delete`（`recursive` 在单元里看得到新加的子节点、MCP 的规则、不结束会话、回收站与附件、子树只读一次）；历史的行与 REST 的同一套单元代码；2.1 的第 1、3、7、10、13、15、17 条；交错、锁的格与 C4 可以照写的行做出来（B-M2 之外）。
- **C**：每个工具对应到 M8 的用例（`versions`、`log`、`changeset` 的读，REST 的 `Execute` 次序不变）；筛选（`client: "mcp"`、`actor: "me"` 是作者 `created_by_id`、`path` 找活的节点再以 `page_id` 筛、回收站里的行照收）；别的笔记本的变更集答 `page.changeset_not_found`、看不到的笔记本先答 `notebook.not_found`，没有预言机；读历史对每个角色都是 `page.history`，没有 403；游标（`Fit` 之后的 `Cursors[k-1]`、`log` 的退回从 `Cursors[0]` 接着、`changeset` 的 `Start` 从头列、`versions` 的 `Below`；除 C-M11 的未测的一种之外不重复、不跳过）；`versions` 的模式与范围；每次调用都前进，输入合代码的界；计划（S2 不加 SQL）；`tools.json` 的两段与输入；码的表（B-M4 之外每个调用都到得了它的码）；MC1 第 5 步的改法；e2e（MC8 的筛选各有一个不匹配的变更集、活动是最后的变更集的时刻、C4（MCP）第 4 步是唯一的不同、MC4 第 4 步、五个测试在 e2e 的作业里余量够、e2e 的 REST 请求体到不了 1 MiB 的门槛、knip）；Step（四个拴在一起的检查让一个工具的五部分在一个提交里；S2 单独合并是绿的；archtest 与 depguard）；成本那一份得 `TestReplaceStaysWithinItsTime`、守卫那一份没有新的；REST 与 MCP 的时刻都是 UTC。

## 未采纳

或采纳的方式与审查者的建议不同：

- **A-I1 的"每个账户一个名额"**：照主持者的决定取"每个凭证一个"：平台只知道认证放进 ctx 的限流的键，不知道账户，按账户要改 `Authenticator` 的答复；代价是一个账户的两个 PAT 仍占得满两个名额，写进剩下的风险（00 号文档第 10 节、M12 的移交），要不要按账户由 M12 的压测之后定。满了的一行点名的是持有者的凭证（`pat:<id>` 里就是令牌的 id，由它找得到账户），不另记 `user_id`。
- **A-M8 的"每一行另以一个阻塞的请求持着另一个名额"**：取主持者的另一个例子：测试的应用只有一个名额，效果相同而每行少一个请求；A 的另两点照做（进行中时断言它持着名额、关闭的请求体声明过门槛的长度）。
- **B-I2 的"不超过 `bytealg.MaxLen` 的模式照用 `strings.Index`"**：没有采纳：主持者的决定是从不拿 agent 的模式给 `strings.Index` 与 `Count`。一个匹配器管所有长度，代码只有一条路、守卫只守一条路；短的模式在状态为 0 时以 `IndexByte` 跳过不可能起头的字节，与 `strings.Index` 同一个量级（3.9 的表）。KMP 而不是 Two-Way：回退表 `int32`，一个 1 MiB 的模式 4 MiB，写法与证明都短。
- **B-I2 的"按间隔看 ctx"**：取另一种：输入的上限（正文 5 MiB、`old_string` 1 MiB、NFC 至多放大 3 倍）让最坏的一次调用约 0.16 秒（CI 上约 1 秒），离调用的 15 秒远；在匹配里看 ctx 只会让常见的情形变慢（3.9）。
- **C-M1 的"`-timeout 20m`、作业 30 分钟"**：没有采纳：主持者的规则是分出第三份而不提高 `-timeout`。
- **C-M3 的"把 `log` 的一种移到 P3 的 batch 的测试"**：没有采纳：主持者定了以种下的库在 P2 做；batch 来了之后它的测试另有真的写的路径。
- **C-M3 的"说明由一个 REST 的撤销写出"**：`changeset` 的那一种照做（撤销的说明是请求的）；`log` 的那一种的两个变更集本来就由 SQL 种下（P2 写不出一个变更集里三项移动），说明在同一条语句里写上。
- **C-M4 的标记总在**：`changes_cut` 只在被截的一行出现（`omitempty`）："只由退回设"是 C 的说法，总在的 `false` 每行多约 20 字节，对 agent 也不多说什么。
- **C 过度设计 1 的"第 5 步只在 `/mcp` 的门槛的路不同时留"**：去掉了：`/mcp` 与 REST 走同一个中间件，门槛只是一个参数。
- **A-N3 的第二点**：采纳，另把它写成"界线为什么在 S1"的理由（第 0 节）：P1 的调用今天就进得来，S1 的界线一并管住它们。

## 新的问题

修订时作者提出的三个问题，主持者都答了（上面"主持者的决定"第 13–15 条），已照改：

- **`BOOTSTRAP_FIRST` 那一份若也过线**（P2 之后估计约 7:05–7:15，在 7.5 分钟之下）：第 13 条；00 号文档第 11 节第 28 条照 2.1 第 22 条改写，6.5、5.1 第 9 步、5.2 第 6 步写了挪法。
- **守卫那一份的最长的一行**（不是 P2 加的；本机 151 秒，CI 上估计约 6 分钟，过了 Go 默认的 10 分钟的一半）：第 14 条；6.5 与 5.1 第 9 步写了量法、比与拆法。
- **按凭证还是按账户**：第 15 条；不改。

## 第二轮复核

| 项 | 内容 |
|---|---|
| 复核对象 | 修订后的 [02-P2-writes-history.md](../02-P2-writes-history.md)（2026-10-18 17:42，306,549 字节，含主持者的决定第 13–15 条）与上面的处置，对照 `main` `2a888a6f` 的代码、SDK v1.8.0 与 go1.27.1；`apply-00m9.py`（39 处）与 `apply-v01.py`（3 处）只以 `--check` 跑 |
| 复核方式 | 三位审查者（Opus）各复核自己的一份，只读，不用 Docker：<br>A：逐节对照初稿（`review-A/check/secdiff.py`）；两个脚本的副本指到草稿区的两份文档上跑、比较结果（`review-A/check/docs/`）；重量修订的说明（`instr-s1`、`instr-s2`）；3.18 的两段 JSON 对照 `out2.json` 与 Go 的输入标签。<br>B：照 3.5.4 把一步的放回做进 `server/` 的草稿区的副本（`review-B/check/one-step-place.diff`；`page/domain`、`page/app` 的测试都过，副本已删）；linking 的改名的 harness 在两棵树上跑 047 的来源页；重跑作者的 `exp/kmp`，加 CRLF、NFC 的再找的构造串、更宽的差分（376,287 例）与表外的最坏的形状（`review-B/check/kmp`）；`strings.Index` 在 120 KB 与 1 MiB 上实测（`review-B/rk/rk-1mib.txt`）。<br>C：三个 `-run`、`-skip` 的表达式照 go1.27.1 的 `testing/match.go` 用在组合根的 361 个测试函数上（`review-C/check/shards.py`）；重跑 `exp/sizes/seeded.py`，另照 REST 的撤销的真样子量 C1（`review-C/check/c1_sizes.py`）；`tools.json` 的形状；时间取自 `main` 的两次运行与 S3 在 `scratchpad/m9/p1s3/` 的日志 |
| 日期 | 2026-10-18 |
| 结论 | 没有 Important。<br>A：自己的 21 条都落下了，5 条带一个小问题（M-1、M-3、M-6、M-7、N-6，各成下面的一条 Nit）；新的 Minor 2、Nit 7。B：16 条里 14 条落下，B-I2 与 B-M1 带问题（成 BC-M1、BC-N1 与 BC-M2）；新的 Minor 3、Nit 6。C：31 条都落下，C-I1、C-M1 带 Minor 的问题，C-M3、C-M4、C-M8、C-M11 带 Nit；新的 Minor 3、Nit 8。<br>全部处理（下面三张表），修订直接写进设计（复核时的样子留在 `scratchpad/m9/p2/notes/rev2-snapshot/`）。几条较大的处置由主持者定下（"主持者的决定（第二轮）"第 16–25 条），负责人可以改判。<br>6.3 的变异由 59 条增为 62 条（第 60–62 条）。`apply-00m9.py` 改了 6 处编辑的新文字（3、5a、6c、21、22 与第 13 节的一行），加了 2 处（21b、20g），`--check` 41 处都对得上；`apply-v01.py` 不变，`--check` 3 处都对得上；都没有写 |

### 发现与处置

第一轮的条目照原样留着；被这一轮改掉的处置以下面的表为准：A-I1 的④（P5 的代理改为按路由，MN-2）、A-M3 的那一句（NN-6）、A-M6 的"经 mcpserver 导出的同一个函数"（`ErrorAnswerBytes`，NN-5）、B-I2 的"1 MiB 约 88 秒"（70.2 秒，BC-N1）与它的守卫、静态的检查（BC-M1）、B-M1 的 047（BC-M2）、主持者的决定第 5、13 条里界线怎样动（一条规则，C-NM2）、作者-7 的说明的数（作者-16）。C 的复核的 Nit 记作 C N1–N8，与第一轮的 C-N1–N14 不是同一组。节号是这一轮修订后的。

#### A

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| MN-1 | Minor | 为每个凭证一个名额算的代价没算上传：名额在读请求体之前取，上传也占着它（10 Mbit/s 上 5 MiB 约 4.2 秒，1 Mbit/s 上过约 250 KB 就过 2 秒，契约自己说 5 MiB 在 30 秒里要约 1.4 Mbit/s），一个 agent 并行的第二个大的写在一般的上行上等满 2 秒、503，模型看到的是传输的失败；修订之前它会拿空着的第二个名额。一般的页（100 KiB 在 10 Mbit/s 上约 80 ms）不受影响；网页在两个标签页里保存两个大的页同样 | 照主持者的决定第 16 条留着每个凭证一个、等 2 秒、503，不加长等候、不加按凭证的队列；代价连同 A 的数写进 3.2.1（"按凭证"的"代价"）、2.1 第 6 条、00 号文档第 10 节的那一行（`apply-00m9.py` 的 6c）与 M12 的移交（第 8 节）；P5 的人工清单加一次限速的上行（1 Mbit/s，两个各约 1 MiB 的并行的 `write`，记下 agent 看到什么、是否一个接一个地重发，第 8 节）。说明加了一行（3.12）："Calls over 64 KiB are taken one at a time per token, the next waiting 2 seconds at most: send large writes one after another, not in parallel."。核对过的：放得下（146 字节；S1 之后 2,693、2,763，S2 之后 2,961、3,031，账户入口还剩 1,065，P3、P4 的五行照旧放得下）；相符（只说服务的行为与做法，不说 agent 会看到什么）：Go 的 SDK v1.8.0 的客户端把 503 当作暂时的错误，只答"Service Unavailable"、不读正文（`sdk/mcp/streamable.go:2583-2587`、`:2821-2833`），`server_busy` 与 `Retry-After` 到不了模型，所以这一行在握手时事先交给客户端；Claude Code 与 Codex 离线核对不了，交 P5 的清单。3.2.1 原先说 Claude Code 并行的第三个大的 `write` 拿到 503，没有核对过，改为一般的说法（作者-15） |
| MN-2 | Minor | P5 的缓冲的建议与 README 的上传的建议相反（`README.md:357`：上传与导入不要缓冲，提前的拒绝才不必等整个文件传完），也避开了项目默认的 Caddy（`v0.1-design.md:127`）；照 P5 在 nginx 的 `location /` 开缓冲就把上传与导入也缓冲了，照 README 则 `/mcp` 与正文路由不缓冲 | 照主持者的决定第 17 条按路由写：`/mcp` 与 page 的两个正文路由（`PUT /api/v0/pages/{page_id}/content`、`POST /api/v0/notebooks/{notebook_id}/pages`）缓冲（nginx 的 `proxy_request_buffering on`），`API.Stream` 的上传与导入照 README 不缓冲；Caddy 默认不缓冲请求体，直接暴露也不，用它们时记下的风险照旧（3.2.1、2.1 第 6 条、第 8 节；00 号文档第 10 节的新的一行写"`/mcp` 与 page 的两个正文路由"，`apply-00m9.py` 的 6c）。A 说的 Caddy 的 `request_buffers` 离线核对不了，交 P5 核对过再写（"第二轮未采纳"） |
| NN-1 | Nit | M-1 的问题：变异 50（改回 `argumentsAt`）之下反向对照的第一张表照样报出"`strict` 的 `s` 没有到达用例"，只多报一条 `must` 的，只看期望的一条在不在的反向对照照样过 | 反向对照断言整张问题的表恰是期望的那几条（3.13.1）；6.3 第 50 条另列 `TestToolStringsDoNotAnswerInternalErrors`（变异之下 `write`、`edit`、`move` 得到结构表的拒绝）；3.21（同 C N1） |
| NN-2 | Nit | M-7 的问题："格答成功而行有 `after` 时 `toAfter.Add(1)`"读得成在调用旁边按结果计，去掉调用的整块的 runner 就什么也不计、照样过 | `toAfter` 在调用之前按 `want` 计，同 `toCheck`（`permission_matrix_mcp_test.go:385-387`）；`aftered` 在调过 `after` 之后计，同 `checked`（`:414-417`）；`t.Cleanup` 里比（`:367-372`）（3.13.2、3.21、附录） |
| NN-3 | Nit | N-6 的问题：`TestALargeBodyWaitsForItsTurn` 在两个名额时只持着一个，等着的不等就拿到空着的那个，变异 45（不等、立即 503）活下来 | 两个别的凭证的请求持着两个名额，其中一个 20 ms 之后放开（3.2.5）；6.3 第 45 条另列 `TestACredentialsSecondLargeBodyWaitsForItsFirst`（它的最后一句在变异 45 之下失败）；3.21 |
| NN-4 | Nit | 没有变异是"等整个服务的名额等满、503 时不还凭证的名额"：`takeLarge` 的错误的一支，错了的话一个碰上过满的服务的凭证此后每个大的请求都 503，直到重启 | 加第 60 条，列 `TestTwoLargeBodiesAreServedAtOnce` 的最后一句（第三个凭证答过 503 之后，它的下一个拿到名额）与整个程序的第 5 步（P3 在第 2、3 步答过 503，它的 `hold` 不等就持着空着的那个）（6.3、3.21） |
| NN-5 | Nit | M-6 的问题：`TestEachErrorFitsAtItsWorst` 经"mcpserver 导出的同一个函数"量，而 3.10.2 没有声明这样的函数（导出的是 `TextBytes`、`MemberBytes` 与 `Fit`）；适配器里另抄一份量法会与 mcpserver 的走开 | 3.10.2 声明 `func ErrorAnswerBytes(err error) (code string, bytes int)`：拿错误本身，走工具的错误那一条路（`Fit` 的找法与成员的写法都在里面），答出码与字节；mcpserver 的 `TestErrorAnswerBytesIsTheAnswer` 核对它与经 `New` 起的服务的一次真的调用相同（3.10.5 的新的一行）；`TestEachErrorFitsAtItsWorst` 经它量（3.10.5、3.1、3.21）。签名与 A 的例子不同（"第二轮未采纳"） |
| NN-6 | Nit | M-3 的问题：契约的两个说明留着"先查正文"，新的一句"…then is server_busy, before any other check"读起来像每个这样的请求体都是 `server_busy` | 照 A 的写法，放在"先查正文"那一句之前："Before any check, a body over server.max_body_bytes waits up to 2 seconds for one of the server's two large-body slots, at most one per credential, when they are taken, and is server_busy if none frees."（3.2.3） |
| NN-7 | Nit | 行号：`apiConfig` 两处写成 `deps.go:271-291`、`:273-293`，是 `:270-293`；NUL 的测试在第 1 节与 3.13.1 不一（`:562-606`、`:566-606`），说明从 562 起 | 改为 `deps.go:270-293`（3.2.2 两处）与 `mcp_reads_test.go:562-606`（3.13.1） |
| A 对 I-1 | — | I-1 的剩下的风险仍是任何一个账户都能造成的可用性的风险；2.2 说没有要负责人决定的是主持者的事，A 会让负责人看到这一条 | 2.2 加一句：第 6 条剩下的风险是任何一个账户都能造成的可用性的风险，主持者接受、交 M12，负责人在 00 号文档第 10 节与 M12 的移交里看得到它；"第二轮的新问题"同样列出 |
| A 对记录 | — | 作者-2 的 `linking.pages_locked` 的排序在 `page/app/lock_holders.go:49`，不是 `:51` | 改了（作者-2）；作者-3 的 `uuid.go:210-262` 照审查者引的改为 `:205-262`（含说 NewV7 递增的注释），设计的第 1 节与附录随之 |

#### B

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| BC-M1 | Minor | B-I2 的问题：时间的守卫碰不到 CRLF 与 NFC 的再找（守卫的形状是不带 LF、已是 NFC 的 `a` 的页，那两处只在全是 CRLF 的页或不是 NFC 的串上走），NFC 的再找改用 `strings.Contains`、CRLF 的再找改用 `strings.Index` 的两个变异过得了每种形状；B 为两处造了串（12 万字节上 `strings` 8.90 秒、9.00 秒，1 MiB 约 70 秒）。静态的检查只读 `edit.go`，别的文件里的帮助函数读不到；禁的表漏了 `LastIndex`、`SplitN`、`SplitAfter`、`SplitAfterN` 与 `NewReplacer` | 照主持者的决定第 18 条：①守卫加 B 的两种形状（3.9 的表；NFC：5 MiB − 3 字节的 `a` 加 `e` 与 U+0301，`old` 是 B 的构造串，匹配器 16.6–17.9 ms；CRLF：一个 CRLF 加 5 MiB − 2 字节的 `a`，`old` 带一个单独的 LF、它的 CRLF 形式的哈希在测试里对 `primeRK` 解出（`craftedCRLF`，目标 84·b⁵ + 87·b⁴），13.4–16.1 ms），并照 B 的可选的建议先以 `go/parser` 读 `src/internal/bytealg/bytealg.go`、断言 `primeRK` 仍是 16777619；②`TestReplaceSearchesOnlyWithItsMatcher` 读 `old_string` 经过的三个包（page 的 `adapter/mcp`、`page/app`、`page/domain`）的每个非测试文件，`strings` 与 `bytes` 的查找、切分与替换的一族（名字以 `Index`、`LastIndex`、`Contains`、`Count`、`Cut`、`Split`、`Replace` 起头的与 `NewReplacer`）的模式不是字面量的，要在明确的表 `allowedSearches` 里（文件、函数、调用的源码、理由），表里过时的一项也失败；S1 时表里五项（作者-13）；反向对照：另一个文件的帮助函数的 `strings.LastIndex(content, old)`、以参数作分隔的 `bytes.SplitN`、`strings.NewReplacer(old, new)`、`strings.CutPrefix(s, p)` 各被报出，表里有的照用，过时的一项被报出；③变异第 61、62 条（3.9、3.1、3.21、6.3） |
| BC-M2 | Minor | B-M1 的问题：3.5.3 手写的 047 的结果是两棵树混起来的，哪一棵都不这样写：`x` 唯一时一步写 `[[X]] [[X]] [[X\|alias]] [[X\|X]] ![[X]] [t](X.md)`，另有 `c/x` 时都写带路径的 `[[b/X]]` 一类；B-M1 的不同（一步 `[[X\|X]]`、两步 `[[X\|x]]`）只在名称唯一的树上有。3.13.5 的说明说 047 与 REST 比较过，与 3.5.3 相悖 | 照主持者的决定第 19 条用 B 的构造：3.5.3 写明 047 的树（`a`、`a/x`、`b` 与来源页 `src`，`x` 在笔记本里唯一）、来源页与一步在这棵树上的结果（B 跑过的），M6 写名称的形式是因为 `x` 唯一，再有一个 `c/x` 时的结果另写一句；变异 24 在这棵树上仍让样例失败。3.13.5 的说明改为只有 3.13.6 与 REST 比较过，草稿那一句不对。2.1 第 21 条与 `apply-00m9.py` 的 21 写"名称在笔记本里唯一时"，另有同名的页时两者都写带路径的 `[[b/X\|X]]` |
| BC-M3 | Minor | 变异 53（`lockedAll` 按页 id 排）列的 `TestTheEditLockGuardsByOperation` 抓不住它：`newLockFixture` 依次以 `uuid.NewV7` 建 Top、Mid、Low、Side，按 id 排正是 Top、Low、Side，也就是那一行期望的子树的次序 | 取 B 的第一种：3.6 的那一行把这一步的改动给成 Top、Mid、Side、Low（不是 id 的次序），期望 `Locks` 是 Top、Side、Low（`ids` 的次序）；按页 id 排成了 Top、Low、Side，变异 53 在这里失败（3.6、3.21、6.3 第 53 条） |
| BC-N1 | Nit | "1 MiB 约 88 秒，B 实测"是 B 的外推；同一个程序实测 120 KB 8.75 秒、1 MiB 70.2 秒 | 照主持者的决定第 24 条处处改为 70.2 秒（B 的复核实测，`review-B/rk/rk-1mib.txt`）：第 1 节、2.1 第 5 条、3.9 的表、6.2 与 `apply-00m9.py` 的 5a（"约 70 秒……实测 70.2 秒"） |
| BC-N2 | Nit | 00 号文档 4.10（:239）留着 `Unit.Relocate(id, parent, position, name)`，而 P2 的 `Relocate` 没有次序（在新的兄弟里排在最后，3.5.1）；"写给后面的 M"（:461）只说 4.10 的 `Unit.Relocate`，没说撤销的一步的放回 | `apply-00m9.py` 加两处：21b 把签名改为 `Unit.Relocate(id, Placement{parent, name, field})`（没有次序：在新的兄弟里排在最后）；20g 在 :461 加"与撤销的一步的放回，4.12"。2.1 第 20、21 条与 3.5.4 随之 |
| BC-N3 | Nit | 没有 `page/app/unit_revert_test.go`，M8 的撤销的单元测试在 `revert_test.go`（`:199`、`:586`）；没有测试走到新的写 `RelocateNode` 撞名；`ErrRevertUnordered` 的注释（`revert.go:89-95`）说每个变更集是一种操作 | 改为 `revert_test.go`；`TestRevertChangesetsDefectsAreUnclassified` 照 `RenameNode`、`MoveNode` 的两行加 `RelocateNode` 答 `ErrTitleTaken` 的一行；注释改为一个既移动又改名的条目计划成一个放回（3.5.4、3.1、5.1 第 4 步、6.1、6.3 第 52 条、3.21、附录） |
| BC-N4 | Nit | 撤销的性质测试的新操作没写它的拒绝：`from` 是附件时 `to` 以 `.md` 结尾，`MoveKinds` 答 422；MCP 的拒绝是带码的 HTTP 200，不是 409，`change` 会让测试失败 | 取 B 的第一种：`from` 只取页的路径；`page.title_taken`、`page.cycle`、`page.too_deep` 记作跳过，`changed: false` 记作没有改动；行号改为 `revert_property_test.go:76-170`（`:163` 只把 409 当作跳过）（3.5.4、附录） |
| BC-N5 | Nit | 变异 59 与 3.21 引的"CRLF 的大小一行"在 3.9 里没写；没有一行核对 NFC 的提示；表里"KMP 自己最坏的回退"不是 KMP 最坏的（每种形状每扫一个字节至多比两次，一处最深的回退链来自 Fibonacci 串） | 3.9 写明 CRLF 的大小一行（全是 CRLF 的页、单独的 LF 的 `old`、换成 CRLF 之后才过 5 MiB 的 `new`，答 `new_string` `too_long`）与 NFC 的提示两行（配得上时带、配不上时不带），3.21 加 NFC 的提示一行；表的那一行改称"深的回退链"，写上 B 的说明 |
| BC-N6 | Nit | `to` 的父页链停在第一个不成的段：`A/img.png/missing/y.md` 答 422，`A/missing/img.png/y.md` 答 404，不是总是 404 在 422 之前 | 写成一步"`to` 的父页链：第一个不成的段决定，缺的 `page.not_found`、路过附件的 422"（2.1 第 3 条、`apply-00m9.py` 的 3、3.8.3 第 6.2 步、3.8.6 的两个例子）；`RelocatePath` 的表各留一行 |
| B 核对过的 | — | 表外的三种最坏的形状：U+1D160 的 CRLF 页上 1 MiB 的单独 LF 的 `old`（两种再找与 NFC 的三倍都走）158.5 ms，5 MiB 的一个组合符号 69 ms，30 个一段、类别递减的组合符号 86 ms；"最坏约 0.16 秒"成立 | 写进 3.9 的"代价与期限" |

#### C

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C-NM1 | Minor | C-I1 的问题：合法参数的表没有账户入口的 `notebooks` 与 `open_notebook`，而"`tools/list` 里有而表里没有的工具让测试失败"，照写的检查在 S1 的第 6 个提交是红的；也没说账户入口的 `notebook` 何时加：换了那一个参数之后才加会盖掉 NUL，那一项悄悄地什么也不测 | 照主持者的决定第 20 条：表加 `notebooks {}` 与 `open_notebook {}`；账户入口的 `notebook` 在换之前放进合法的参数；每一项发出之前断言最后的参数在点名的参数处恰是那个 NUL（`ops[].path` 看第一个 `op`）（3.13.1、5.1 第 6 步、3.21） |
| C-NM2 | Minor | C-M1 的问题：第二、第三份过线时有三种不同的做法（6.5 的规则、6.5 的"`BOOTSTRAP_FIRST` 过线时"、5.2 第 6 步只挪 `BOOTSTRAP_THIRD`），没有选的准则；2.1 第 22 条与第 28 条的改写没说第二、第三份怎样做、不提高 `-timeout`；只挪 `THIRD` 在两份都近线时把另一份推过线 | 照主持者的决定第 21 条取 C 的一条规则，处处一样（6.5、5.1 第 9.2 步、5.2 第 6 步、2.1 第 22 条、2.2、第 0 节、3.1；00 号文档第 11 节第 28 条经 `apply-00m9.py` 的 22，管组合根的每一份）：一份过了线（两次运行的平均里 CI 的测试一步过了约 7.5 分钟），只在一个邻居挪过之后仍在线之下时挪它们之间的界线，否则那个 Step 按测试名再分一份，S1、S2 都一样；从不提高 `-timeout`。6.5 另写：每一份的邻居是谁；挪之前以本机的数（乘约 1.6）估计邻居挪过之后的 CI 测试一步，挪之后以 CI 的两次运行核对，核对出邻居过了线就撤回、改为分；以后分出的一份怎样写（每份一个表达式，第二份 `-skip` 它们的并）。照估计第一份 P2 之后离线只有约 15–25 秒，收不下第二份要给出去的，所以 S1 多半分出第三份 |
| C-NM3 | Minor | C-M1 的问题：本机的 `make test-go` 以一个二进制跑整个组合根，受同样的 15 分钟，S3 的六次 479–573 秒（每次都过 7:30），P2 之后约 8:45–10:45，第三份对它没用，没有规则看着它 | 照主持者的决定第 22 条：S1 把 `test-go` 改为照 CI 的份跑组合根，每份一个 `go test` 二进制、各受 15 分钟，同一条规则管两处，`make test-go` 照旧是 CI 各份之和；成本与守卫两份照旧在竞态检测的二进制之后跑。同时还是依次：以 S3 的日志估计（没有实测），依次约 100 + 265 + 2 × 128 ≈ 620 秒（约 10.5 分钟），`$(MAKE) -j4` 同时约 4.5–5 分钟，依次每次多约 5.5–6 分钟、过了约 5 分钟：取同时；整个门禁约 11–14 分钟（今天约 15 分钟）。失败找得到：不带 `-v` 的 `go test` 在包列表的模式里把一个包的输出在它跑完时成段写出（`cmd/go/internal/test/test.go:1467-1507`），make 的 `*** [test-go-bootstrap-2] Error 1` 点名那一份。S1 各跑一次两种、写进审查记录，同时的不比依次快 5 分钟以上就改回依次。选择、数与 Makefile 的形状写进 6.5（"本机的门禁"与 `make` 一段）与 5.1 第 9.3 步；6.4、第 0 节、3.1、第 9 节与第 7 节第 4 条的 13.4 第 8 条随之 |
| C N1 | Nit | 同 NN-1 | 同 NN-1 |
| C N2 | Nit | 2.4 的比是五次本机运行的中位数，各次 1.9–2.7，偏高；守卫的作业日志里每一行打着 `ok …/internal/bootstrap <秒>`，S1 可以直接读 | 照主持者的决定第 23 条：6.5 与 5.1 第 9.4 步写明 2.4 是五次的中位数、各次 1.9–2.7、偏高（分子含编译与起库），结论不变（各次自己的比给 6.0–6.4 分钟，2.4 乘 139–200 秒给 5.6–8.0 分钟，都过 5 分钟）；每一行的 CI 时间不用：作业的日志要认证，这里读不到，读得到的只有每一步的时间（"第二轮未采纳"） |
| C N3 | Nit | `seeded.py` 把 C1 的头当作不带 `reverts` 的 MCP 的 `edit`，而 C1 是 bob 的 REST 撤销（`kind` `revert`、`client` `api`、带 `reverts`）：52,850、29,526、23,500 | `seeded.py` 照真样子量，得 C 的数（3.17.3、6.2、第 9 节）；结论不变 |
| C N4 | Nit | "W2 与它后面的一行被跳过"说多了：变异之下 `next_cursor` 是存储的、在 W2 之后，只跳过 W2 | 改为只跳过 W2（3.17.3、6.3 第 43 条） |
| C N5 | Nit | REST 每个变更集列前三项变化，"读完的行与 REST 的 `listChangesets` 的全部相同"对 W1、W2 不成立 | 被截的行的 `changes` 是 REST 的前两项、带 `changes_cut: true`，别的成员相同；其余的行全部相同（3.17.3） |
| C N6 | Nit | `log` 的说明前后两句说了两遍 `changeset` 读全 | `exp/toolsjson/gen2.py` 只留一句，重新生成 `out2.json` 与 3.18 的两段 |
| C N7 | Nit | 3.13.5 第 1 步的"次序是页 id 的"分辨不出：B 先于 C 建，页 id、兄弟、名称、持锁人（你在 bob 之前）四种次序都是 B、C | 取 C 的第一种：先建 C 后建 B，页 id 的次序 C、B 与名称、持锁人的次序不同；第 1 步期望 `locks` 是 C（bob）、B（alice，`you: true`），文字里的名字是 bob、你（3.13.5） |
| C N8 | Nit | 第三份让 `Makefile:187` 的"CI 的五份"与 `:198` 的"-run 与 -skip 用同一个表达式：两份之和总是整个包"不再对 | 6.5 写明两处都改（`make` 一段写出新的两行） |
| C 核对过的 | — | 组合根的 CI 与本机之比：两次运行的平均约 1.6，单次 1.35–1.91，比 6.5 的"约 1.4–1.7"宽 | 6.5 两处改为"两次运行的平均约 1.6，单次 1.35–1.91" |

### 主持者的决定（第二轮）

主持者在第三份简报里定下的处置（作者照做；负责人可以改判）：

16. **A 的 MN-1**：留着每个凭证一个名额、等 2 秒、503 `server_busy`；在 3.2、00 号文档第 10 节的那一行与 M12 的移交写明这个代价与 A 的数；P5 的人工清单加 A 的限速的上行的一次；说明的一行可选：放得下、与客户端给 agent 看到的相符才加（传输上的 503 未必以文字到达模型），写明核对了什么；不加长等候、不加按凭证的队列。
17. **A 的 MN-2**：P5 的建议按路由：`/mcp` 与 page 的两个正文路由缓冲，上传与导入照 `README.md:357` 不缓冲；写明 Caddy 不缓冲请求体，用 Caddy 时记下的剩下的风险照旧。
18. **B 的 BC-M1**：时间的守卫加 B 的两种形状，`strings` 的查找用在 CRLF 或 NFC 的再找里就让它失败；静态的检查读每个处理 agent 的模式的非测试文件，不只是 `edit.go`，禁 `strings` 与 `bytes` 的查找、切分与替换的一族（含 `LastIndex*`、`Split*`、`Cut*`、`Contains*` 与 `NewReplacer`）拿 agent 的模式；非留不可的调用（例如在常量上的）由一张明确的表带理由放过；检查留着它的反向对照。
19. **B 的 BC-M2**：047 的期望的结果必须是一棵真的树的答复，用 B 的构造；改正 3.13.5 说 047 与 REST 比较过的那一句。
20. **C 的 C-NM1**：表加 `notebooks` 与 `open_notebook`，S1 的第 6 个提交才是绿的；账户入口的 `notebook` 在换那一个参数之前合进去；测试断言每一项最后的参数在点名的参数里带着 NUL，一项不会悄悄地不测。
21. **C 的 C-NM2**：处处取 C 的一条规则（6.5、5.1 第 9 步、5.2 第 6 步、2.1 第 22 条、`apply-00m9.py` 的第 28 条与这份记录）：一份过了线（两次运行的平均；CI 的测试一步过了约 7.5 分钟），只在邻居挪过之后仍在线之下时挪界线，否则分，S2 也一样；从不提高 `-timeout`；第 28 条管每一份。
22. **C 的 C-NM3**：S1 让本机的门禁与 CI 一样分份，每份一个 go test 二进制、受同样的 15 分钟，一条规则管两处，`make test-go` 照旧是 CI 各份之和；耗时的守卫（成本、守卫）照旧在竞态检测的二进制之后跑、不与它们同时；量或以数估计同时（`$(MAKE) -j`，macOS 的 make 3.81 有）与依次的本机用时，说明交错的输出里失败怎样找得到；取简单的一种，除非它每次门禁多花约 5 分钟以上；选择、数与 Makefile 的形状写进 6.5 与 5.1 第 9 步。
23. **C 的 N2**：CI 的作业日志在这里要认证，每一行的 CI 时间读不到，只读得到每一步的；2.4 照用，写明它偏高、为什么不用每一行的时间，给出 1.9–2.7 的摆动。
24. **B 的 BC-N1**：1 MiB 的 `strings.Index` 是 B 实测的 70.2 秒，不是 88 秒，引它的每一处都改。
25. 其余照审查者的建议改；作者认为建议不对时写明理由、另想办法（下面"第二轮未采纳"）。

### 第二轮的作者的更正

修订时作者自己发现的：

| # | 发现 | 处置 |
|---|---|---|
| 作者-13 | 静态的检查的表在 S1 不止一项：page/domain 今天有四处这一族的调用，模式是一个函数或一个字符、不是字面量：`Client.Valid` 的 `strings.ContainsFunc(name, unicode.IsControl)`（`client.go:29`）、`ParseGlob` 的 `strings.ContainsFunc`（`glob.go:114`）、`lexemes.scan` 的 `strings.IndexFunc`（`search.go:304`）、`inDataURL` 的 `strings.ContainsRune`（在一个七个字节的常量里找一个字符，`search.go:345`）；另有以数作模式的（`ContainsRune(s, 0)`、`IndexByte(content, 0)`） | 字面量含数；S1 时表里五项：匹配器的 `IndexByte` 与这四处，理由照写（3.9）。page 的 `adapter/mcp` 与 `page/app` 没有这一族的调用 |
| 作者-14 | 2.1 第 5 条与 `apply-00m9.py` 的 5a 说"从不以 `strings.Index`、`strings.Count`、`ReplaceAll`"，读得成只有这三个 | 改为"从不以 `strings`、`bytes` 的查找、切分与替换（`Index`、`Count`、`ReplaceAll` 这一族）" |
| 作者-15 | 3.2.1 说"Claude Code 一次并行的几个大的 `write` 里第三个就拿到 503"，没有核对过；2.1 第 6 条同样，而每个凭证一个名额之后是第二个 | 3.2.1 改为一般的说法加 Go 的 SDK 的做法（MN-1）；2.1 第 6 条写"一个 agent 并行的第二个大的写"；Claude Code 与 Codex 交 P5 的清单 |
| 作者-16 | 说明加了 MN-1 的一行，3.12 的数与第 8 节的余地不再对 | 以 `exp/instr/s1-r2.go.txt`、`s2-r2.go.txt` 重量：S1 之后 2,693 与 2,763，S2 之后 2,961 与 3,031，4 KiB 之下还剩 1,135 与 1,065（3.12）；第 8 节给 P3 的余地由"约 1.2 KiB"改为"约 1 KiB"（账户入口 1,065 字节）；3.12 的"最后两行"改为"最后三行" |

### 第二轮审查者核对过、没有问题的

- **A**：每个凭证一个名额的做法（先取凭证的、再取整个服务的，一个计时器；还时先还整个服务的；凭证的表按进行中的请求计数、有界；名额在处理函数之前取、两种名额的次序固定，没有死锁）；`/mcp` 与 REST 共用 `pat:<id>`、`session:<id>` 的键；满了的一行只记凭证的键（13.1 第 10 条许）；变异 1、2、4、6、7、46、47 与一个名额的应用上的 3 都由列出的测试抓住；`Build(0)` 是空的 `locks` 加 `locks_count`；`linking.pages_locked` 按页 id、MCP 的删除按子树，第 11 格两者不同；原样引起（标题里没有引号与反斜杠，引的路径都过了 `ParsePath`，P1 的 `text.go:22` 与四个提示是用户看得到的全部 `%q`，3.10.3 的界照旧）；NUL 的测试的 18 个参数、11 个空过的，重数过；说明的量与文字块相同；3.18 的两段与 `out2.json` 相同；两个脚本的写法与 A 的区域的新文字。
- **B**：3.5.4 照写的做进副本之后四种情形都对，`page/domain`、`page/app` 的测试都过；后台的撤销走同样的两块；搜索的队列、事件、linking 的观察者与编辑锁都按改动走，一个也改名的 `OpMove` 照样跟得上；整个程序的测试抓得住两种两步的次序。匹配器是标准的 KMP，差分重跑 257,204 例、B 的更宽的 376,287 例都相同；计数、唯一、N、CRLF 的再找与大小的算法照写；线性（1.05–1.19 ns 一个字节），作者的测量重现，"最坏约 0.16 秒"成立；期限（`/mcp` 的 15 + 30 秒，请求体读满 30 秒仍剩 15 秒），不看 ctx 成立；变异 42（主要的几处）、58、18–20、59 能失败。3.6、3.11.5、3.13.3 一致；3.13.1 的 `server_busy` 的格不会挂住；3.13.5 的预检连本人的锁也拒；3.5.1、3.8.3 的 409 的次序。两个脚本，B 的区域的编辑与修订相符（88 秒与签名之外）。
- **C**：三个分份的表达式互不重叠、合起来是整个包（竞态下编译的 356 个函数 156、102、98，另 5 个在 `!race` 的文件里），第二份的 `-skip` 在顶层的 `|` 分段，跳过的测试的子测试随它；`ci.yml` 照 `matrix.leg` 跑 make 的目标；第三份的平衡（S3 的 `-v` 运行里 100.4 对 104.3 秒，另加矩阵的墙钟时间）合作者的 124 对 120 秒；规则的逻辑（两次运行的平均、按测试一步判断偏严）；Q2 的"时限的一半"与成本的数（本机之和 169.2 秒，1.41、2.26）；种下的历史建得出来（组合根的测试里用 SQL 是已有的做法，00014 只限字节长，10 层的链在 `MaxDepth` 与 4,096 字节之内）；每个 Step 逐个提交是绿的（C-NM1 之外）；格的重新编号一致；6.5 的数加得起来；两个脚本的新文字在 C 的区域与修订相符。

### 第二轮未采纳

或采纳的方式与审查者的建议不同：

- **NN-5 的签名**：A 的例子是 `ErrorAnswerBytes(text string, member map[string]any) int`，取了 `ErrorAnswerBytes(err error) (code string, bytes int)`。照 A 的签名，模块的测试要自己建成员（`code`、重复文字的 `message`、`Fit` 在 `Extra` 上的找法），正是 A 说的会走开的那一份；拿错误本身就走 mcpserver 自己的那条路，码也一并答出，测试连码一起核对。A 的另一种（每个错误经一个真的 `mcpserver.New` 与替身的工具）由 mcpserver 的 `TestErrorAnswerBytesIsTheAnswer` 做一次，核对这个函数与真的答复相同。
- **MN-1 的说明的字句**：A 建议 "Writes over 64 KiB with one token are served one at a time: send large writes one after another."，写成 "Calls over 64 KiB are taken one at a time per token, the next waiting 2 seconds at most: send large writes one after another, not in parallel."：门槛看的是调用的请求体，一个大的 `edit` 也算；2 秒告诉 agent 第二个为什么失败。146 字节，A 估计约 100。
- **MN-2 的 Caddy**：主持者的"Caddy 不缓冲请求体"写成"Caddy 默认不缓冲请求体"：A 记得新的 v2 的 `reverse_proxy` 有 `request_buffers <size>`，能为一个路由缓冲，离线核对不了；"默认"两种情形都对。P5 核对过 `request_buffers` 再决定写不写进接入文档（第 8 节）。
- **BC-M1 的明确的表**：模式是字面量的调用不进表，只有模式不是字面量的才要一项。字面量拿不到 agent 的串；三个包今天约二十处以 `"/"`、换行、`"mcp:"`、0 作模式，一一列进表什么也不多防，以后每加一处字面量都要改表。主持者的例子"在常量上的"指具名的常量，它不是字面量，要一项。这与 B 自己的写法（"every other pattern is a literal"）相同，表是主持者加的。
- **C N2 的"直接读 CI 上每一行的时间"**：照主持者的决定没有用：作业的日志要认证，这里读不到。S1 读得到时（5.1 第 9.4 步）连同每一步的时间记下。
- **C-NM3 的同时跑**：主持者让作者在同时与依次之间选，取了同时（`-j4`）：依次每次门禁多约 5.5–6 分钟，过了约 5 分钟。数是以 S3 的日志估计的，没有实测；S1 实测一次两种，不够快就改回依次。

### 第二轮的新问题

- 无要负责人决定的。
- 负责人看得到的（主持者接受、交 M12）：每个凭证一个名额的代价（一个凭证并行的第二个大的写在慢的上行上 503），与任何一个账户以两个凭证占满两个名额的风险（2.1 第 6 条、2.2、00 号文档第 10 节、M12 的移交）。
- 实施时要量的：本机的 `make test-go` 同时与依次各一次（5.1 第 9.3 步）；守卫的 markdown 一行的三个测试与成本的 page/domain 一行（5.1 第 9.4 步）；两种再找的形状在 CI 上的值（第 9 节）。
- P5 要核对的：Claude Code 与 Codex 遇到 `/mcp` 的 503 时 agent 看到什么，限速的上行上并行的两个大的写；Caddy 的 `request_buffers` 能不能为 `/mcp` 与正文路由缓冲请求体（第 8 节）。

## 第三轮复核

| 项 | 内容 |
|---|---|
| 复核对象 | 第一次复核之后修订的 [02-P2-writes-history.md](../02-P2-writes-history.md)（2026-10-18 21:24，332,701 字节）与上面"第二轮复核"的处置，对照 `main` `2a888a6f` 的代码、SDK v1.8.0 与 go1.27.1；`apply-00m9.py`（41 处）与 `apply-v01.py`（3 处）只以 `--check` 跑 |
| 复核方式 | 三位审查者（Opus）第二次复核，各看自己的一份：第一次复核的条目落下了没有、主持者在它那一份的处置、这一轮有没有弄坏别的；只读，不用 Docker：<br>A：逐节对照第一次复核时的样子（`review-A/check/secdiff3.py`）；脚本的副本落在 00 号文档的草稿区的副本上、读写出的行（`review-A/check/docs3/`）；重量说明（`instr2-s1`、`instr2-s2`）；读设计引的 SDK 的行。<br>B：以 `go/ast` 扫三个包（`review-B/check2/scan/`）；41 处编辑落在 00 号文档的副本上读（`review-B/check2/00m9-after-r3.md`）；kmp 的副本加 `regexp` 与 CRLF 的大小一行的实验（`zz_b4_test.go`、`zz_b5_test.go`）。<br>C：两版逐字地对照（`review-C/check2/cdiff.py`）；在 macOS 的 GNU Make 3.81 上跑 6.5 的 `-j4` 的形状（替身的 Makefile，打上 6.5 的一段的真的 Makefile 的 `make -n test-go`，两个包的 Go 模块在 `make -j2` 下看失败怎样写出：`review-C/check2/mk`、`mkreal`、`gobuf`）；读 go test 的输出缓冲、testcontainers 的会话与 `pgtest.go:44`；重跑 `seeded.py` 与 `tools.json` 的形状 |
| 日期 | 2026-10-18 |
| 结论 | 没有 Important，也没有 Minor。<br>A：第一次复核的 14 条都落下了，NN-5 换了做法，A 认为理由成立；新的 Nit 2。B：11 条（BC-M1–BC-N6，与 B-I2、B-M1 照落下的）里 9 条落下，BC-M3 与 BC-N5 各带一个 Nit 的问题（成 CN-2、CN-3）；新的 Nit 5（含那两个）。C：17 条都落下，主持者的 C-NM1–C-NM3 照落下的成立；新的 Nit 4。<br>11 个 Nit 全部处理（下面三张表），修订直接写进设计（第二次复核时的样子留在 `scratchpad/m9/p2/notes/rev3-snapshot/`）。6.3 的变异由 62 条增为 63 条。`apply-00m9.py` 改了 1 处编辑的新文字（5a：也不拿 agent 的串编 `regexp`），`--check` 41 处都对得上；`apply-v01.py` 不变，`--check` 3 处都对得上；都没有写 |

### 发现与处置

"第二轮复核"的条目照原样留着；被这一轮改掉的处置以下面的表为准：3.2.5 与整个程序的测试里"不等就"的计时（NN3-2）、`ErrorAnswerBytes` 过界时的字节（NN3-1）、3.9 的静态的检查的一族与它的表（CN-1、CN-4）、3.6 的 `web` 的一行（CN-2）、CRLF 的大小一行（CN-3）、6.5 的邻居与本机门禁的量法（C N10、N11）。节号是这一轮修订后的。

#### A

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| NN3-1 | Nit | `TestErrorAnswerBytesIsTheAnswer` 的过界的一行比不了字节：真的过界的答复是 mcpserver 的 `internal_error`，文字（`message` 是它的一份）与 `request_id` 带着调用的请求 id（`mcpserver/errors.go:150-160`），请求 id 来自请求的 ctx、可以由调用者给，1 到 128 个字符（`httpserver/middleware.go:81-95`）；`ErrorAnswerBytes(err)` 没有 ctx | 照 A 的改法：3.10.2 写明过界时码是 `internal_error`、字节是过了界的那个答复的大小（`overBound` 记的 `bytes`，`errors.go:195-201`），不是 `internal_error` 的答复的；3.10.5 的那一行对过界的一种只比码，字节另与那次调用的 error 一行记的 `bytes` 比；放得下的几种不带请求 id，照旧码与字节都比 |
| NN3-2 | Nit | `TestTheSlotComesBackOnEveryPath` 的"新请求不等就拿到名额（计时小于等候的一半）"约 25 ms：`-j4` 之下竞态检测的 httpserver 与组合根同时跑时会偶尔失败，也看不出答复看不出的（一个名额的应用里，漏了的名额让下一个等满 50 ms、答 503，还了的答 200）；名额在 `ServeHTTP` 返回之后才还，客户端先读到答复也是正常的。整个程序的第 5、6 步的"不等就"同样 | 照主持者的决定第 28 条只看答复、不计时：3.2.5 的那一行改为新请求拿到名额、照常答（不是 503）；整个程序的第 5 步改为 P3 的 `hold` 拿到空着的那个（进了工具，放开之后答结果，不是 503），第 6 步改为 P1 的调用答结果、不是 503；`hold` 进来时告诉测试，整个程序的测试由此知道它持着名额；3.21 与 6.3 第 60 条随之。同一类的另三处也改了（作者-17） |
| A 另记 | — | NN-2 引的 `checked` 今天在 `permission_matrix_mcp_test.go:413-416`，不是 `:414-417` | 改了（3.13.2、附录） |

#### B

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| CN-1 | Nit | 静态的检查的一族停在 `strings`、`bytes`：`regexp.MustCompile(regexp.QuoteMeta(old))`（Go 找字面量、全部替换的常见写法；模式就是程序，O(n·k)：1 MiB 的 `a` 的页上 1,000 个 `a` 加 `b` 4.8 秒，B 的 30 KB 的构造串在 5 MiB 的页上 10 分钟没跑完）、经别名、点导入或函数值（`f := strings.Index`）的写法、长的字面量的模式都过得了检查。今天的六处由时间的守卫抓住，新加的一处没有 | 照主持者的决定第 27 条取 B 的收紧：一族加 `regexp` 的 `Compile`、`MustCompile`、`CompilePOSIX`、`MustCompilePOSIX`、`Match`、`MatchString`、`MatchReader`；包按每个文件的导入路径认（别名、点导入都算）；一族的函数不作调用的函数而被引用的也要在表里。B 的两条可选的也取了：字面量的模式至多 32 字节，一族加 `Trim`、`TrimLeft`、`TrimRight`（cutset 不是 ASCII 时每个字节查一遍它）。今天的两处 `regexp`（`parseTerm` 与 `SearchQuery.InContents` 的 `regexp.MustCompile`，`search.go:547`、`:580`）进表，理由是查询至多 `MaxSearchQueryBytes` = 256 字节。反向对照加 `regexp.MustCompile(regexp.QuoteMeta(old))`、`import s "strings"` 之后的 `s.Index(content, old)`、点导入的 `bytes` 的 `Index`、`f := strings.Index` 与 33 字节的字面量（3.9、3.21）；2.1 第 5 条与 `apply-00m9.py` 的 5a 写上"也不拿它编 `regexp`" |
| CN-2 | Nit | BC-M3 的问题：3.6 的那一行把改动给成 Top、Mid、Side、Low 之后，旁边的 `web` 的一行仍期望"被 bob 的 Low 拒"；`GuardWrite` 的非 MCP 的删除照这一步的次序点名第一个别人的页（`page/app/edit_lock.go:44-74`），那是 Side | 取 B 的第二种：`web` 的一行用同样的改动的次序、期望被 bob 的 Side 拒，这一行也就核对了 `web` 的路照这一步的次序（3.6） |
| CN-3 | Nit | BC-N5 的问题：CRLF 的大小一行的 `old_string` 写成"单独的 LF"，而单独一个 LF 在页的每个 CRLF 里都找得到，第一次就找到，走不到 CRLF 的规则（B 实测：十行的 CRLF 页上把 LF 换成 `y` 加 LF 答 10 处、`CRLF` 为假；`x` LF `x` 才走 CRLF 的规则） | 照 B 的改法：页里有 `a` CRLF `b`，`old_string` 是 `a` LF `b`（LF 的两边有字），`new_string` 带几个单独的 LF，页的长短取得让它换成 CRLF 之后过 5 MiB、不换时恰好不过（3.9） |
| CN-4 | Nit | `allowedSearches` 的 S1 的项写的是今天的 `Client.Valid` 的源码，而 S1 的 3.3.1 把它改成 `strings.ContainsFunc(name, shared.Unshowable)`：项按文件、函数与调用的源码对，照今天的写，到 S1 那一项过时、新的调用不在表里，测试失败 | 照主持者的决定第 26 条，表照 S1 改过之后的源码写；3.3.1 另把 `strings.CutPrefix(string(c), "mcp:")` 改成以常量 `mcpPrefix` 作模式，常量不是字面量，也要一项（作者-18）：S1 时表里八项（3.9） |
| CN-5 | Nit | 6.3 第 61 条的括号"修订的守卫没有这种形状，抓不住它"跟在 `TestReplaceSearchesOnlyWithItsMatcher` 后面，读起来像静态的检查抓不住；它说的是第二轮的守卫 | 括号移到 `TestReplaceStaysWithinItsTime` 的那一种后面，写"第二轮的守卫"；第 62 条的"修订的检查"同样改为"第二轮的检查"（6.3） |

#### C

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C N9 | Nit | 6.3 没有"账户入口的 `notebook` 在换那一个参数之后才加"的变异，而 3.21 列着它、说完整的清单在 6.3 | 加第 63 条，列 `TestToolStringsDoNotAnswerInternalErrors`（每一项发出之前的断言：换 `notebook` 的那几项最后的参数里没有 NUL）（6.3） |
| C N10 | Nit | 6.5 的"每一份的邻居"比错了量：第二份要给出的是它过线的部分（照 6.5 的估计至多约 0:40），不是 P2 加的全部（0:45–1:30）；第二份约 7:45–7:55 之下时规则挪 `BOOTSTRAP_FIRST` 而不分，那时 S2 的两个 `TestTheMCPHistory…` 落进第二份，S2 再分。6.5 说得肯定，5.1 说"多半" | 照 C 的改法：以过线的部分算、说"多半"；写明挪动要让两份在挪过之后都在线之下（给的一份也回到线下），否则分；第二份只是刚过线时挪 `BOOTSTRAP_FIRST`，S2 照同一条规则再看（6.5、5.1 第 9.2 步） |
| C N11 | Nit | 改回依次只凭各一次的运行：估计的余量只有 0.5–1 分钟，而单独一次前后可差一成以上（依次的约 ±1 分钟） | 各跑两次、按平均定（6.5、5.1 第 9.3 步、第 9 节） |
| C N12 | Nit | "成段写出"该引 `builderPrintTest`（`test.go:2257-2268`，一次写出），:1467-1507 是要不要先存着的选择；经管道（`make test-go \| tee`）时长过 `PIPE_BUF`（macOS 512 字节）的一次写不是原子的，别的份的一行 `ok` 可能落进一段失败里；结论不变 | 照 C 的改法写进 6.5 的"失败找得到"与附录；另写组合根的三份打出同样的 `FAIL …/internal/bootstrap`，是哪一份由 make 的那一行与测试名看出 |

### 主持者的决定（第三轮）

主持者在第二次复核之后的消息里定下的（作者照做；负责人可以改判）：

26. **B 的 CN-4**：`allowedSearches` 里 `Client.Valid` 的一项要合 S1 改过之后的源码，不是今天的，否则测试在 S1 失败。
27. **B 的 CN-1**：取收紧：一族加 `regexp` 的构造函数，按导入路径认包（别名与点导入），报出不作调用的引用；留着反向对照。
28. **A 的 NN3-2**：断言 200 或 503 的答复，不断言时间。
29. 其余照审查者的建议改；作者认为建议不对时写明理由（下面"第三轮未采纳"）。

### 第三轮的作者的更正

修订时作者自己发现的：

| # | 发现 | 处置 |
|---|---|---|
| 作者-17 | 同 NN3-2 的一类：`TestACredentialsSecondLargeBodyWaitsForItsFirst` 与 `TestALargeBodyWaitsForItsTurn` 的"20 ms 之后放开"在 50 ms 的等候里也是一个时间，`-j4` 之下睡过了头，等着的就 503；`TestTheSlotsAreTakenAfterTheBuckets` 的"都不等"要计时才看得出 | 放开在等着的请求已在等之后（httpserver 的包内测试读凭证的表：A 的计数是 2、第三个凭证在表里），不靠睡；"都不等"改为看答复：401、429，不是 503（先取名额的话它们会等满、答 503）（3.2.5） |
| 作者-18 | S1 的 3.3.1 把 `Client.Valid` 的 `strings.CutPrefix(string(c), "mcp:")` 改成以常量 `mcpPrefix` 作模式，常量不是字面量 | `allowedSearches` 加这一项（4 字节的常量）；S1 时表里八项（3.9） |

### 第三轮审查者核对过、没有问题的

- **A**：MN-1、MN-2 在 3.2.1、2.1 第 6 条、第 8 节与 00 号文档第 10 节的那一行（A 读了脚本写出的行）说的相同；说明的一行重量过（S1 之后 2,693、2,763，S2 之后 2,961、3,031，加 146 字节），只说服务的规矩与做法，Go 的 SDK 把 503 只答"Service Unavailable"、不读正文，握手时说是唯一靠得住的路；"Calls"比 A 的"Writes"好（大的 `edit` 也过门槛）；"Caddy 默认不缓冲"成立；NN-5 的签名的理由成立；变异 45、50、60（与没改的 3、46、47）都由列出的测试抓住；`tools.json` 的两段与 `out2.json` 相同；第 8 节交 M12 的"再定要不要按凭证排队或放宽等候"与主持者的"不在 P2"不冲突。
- **B**：BC-M1 的两种新的形状与 `primeRK` 的断言是 B 的构造与数；五项（这一轮之前）都合今天的代码，字面量的例外放不过 agent 的串；静态的检查的范围对 P2 是对的（`old_string` 与它的 CRLF、NFC 形式只到这三个包）；047 的结果正是 B 的 harness 写出的；00 号文档 :239 与 :461 在 41 处编辑之后读得通，21 与 21b 不重叠；变异 42、52、53（第 11 格与改过的一行）、59（CN-3 之后）、61、62 都能失败；3.9 引的 158.5 ms、69 ms、86 ms 与 376,287 例照 B 的。
- **C**：C-NM1 的断言抓得住换的次序错了，与变异 50 不冲突；C-NM2 的一条规则在各处相同，挪之前估计、挪之后核对让"邻居仍在线下"核对得了；`-j4` 的形状在 make 3.81 上照写的跑（四个目标同一秒起，一个失败时等别的跑完、成本与守卫不跑；`make -n test-go` 的输出合起来正是 CI 的六份）；组合根的几份同时跑是安全的（每个测试二进制起自己的容器，testcontainers 的会话按父进程分，测试只写 `t.TempDir()`，没有固定的端口）；估计合 S3 的日志，依次的 100 秒是低的一端，取同时只更有理由；S1 的第 6、7 个提交到第 9 个提交之间一个二进制的门禁约 8:45–10:45，仍在 15 分钟之下，每个提交照旧是绿的。

### 第三轮未采纳

或采纳的方式与审查者的建议不同：

- **CN-1 的 32 字节**：B 说的是 arm64 的 `bytealg.MaxLen`；amd64 不带 AVX2 时它是 31，`LastIndex` 对两个字节以上的模式总走 Rabin–Karp，所以 32 不是"从不走 Rabin–Karp"的界。取 32 的理由写成另一个：任何找法（Rabin–Karp、`strings.Index` 的暴力的一段、`regexp` 的程序）每扫一个字节至多比模式那么多次，32 次之内是线性的。
- **CN-2**：B 的两种改法取了第二种，它多核对一件事：`web` 的路照这一步的次序找。
- 别的都照审查者的建议。

### 第三轮的新问题

- 无要负责人决定的；实施时要量的与 P5 要核对的照"第二轮的新问题"，本机门禁的同时与依次改为各两次（5.1 第 9.3 步）。
