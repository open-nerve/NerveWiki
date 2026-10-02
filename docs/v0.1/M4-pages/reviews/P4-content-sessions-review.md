# M4/P4 正文与编辑会话：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m4-p4-content-sessions`（`main...d03c993`：5 个提交，S1 `c05624a`、S2 `9f7f0cd`、S3 `3e652c5`、S4 `4c00580`、S5 `d03c993`；100 个文件，+6238/−182），对照 [04-P4-content-sessions.md](../04-P4-content-sessions.md) 第 1–6 节、五份 Step 计划、[M4 总设计](../00-M4-design.md)第 3、4、5、8、10、11 节、总体设计 3.8–3.9、13.1、13.3–13.4，以及作者的偏差说明（as-built 笔记，11 条）与五组反向对照（S1 21、S2 26、S3 30、S4 14、S5 3 项） |
| 审查方式 | 独立审查者在 `git archive d03c993` 的快照上实测（快照里 `git init`、`pnpm install --frozen-lockfile --prefer-offline`、`golangci-lint` 取自仓库的 `bin/`），另拷一份快照做探针；仓库、别的进程与别的 Docker 容器都没动过：<br>• 门禁：`GOFLAGS=-p=3 make check` 退出 0——`golangci-lint` 两处 0 issues、`go mod tidy -diff` 干净、样例集自检、oxlint、oxfmt、三个包的 `tsc`、knip 通过；`go test -race -count=1 ./...` 53 个包 ok、无 FAIL（bootstrap 96 秒）；不带竞态检测的三个耗时测试 ok；`server/tools` ok；vitest 63 个文件 1012 个测试通过；`build-web` 通过。`make gen-check` 退出 0、无差异。<br>• 稳定性：交错 38–43 与两个清理任务的整个程序测试 `-race -count=5` 连跑五次全过（72 秒，本机负载 4–7，别的会话在跑测试）。<br>• 资源：带 `//go:build review` 的探针（不进门禁）量了最大正文的代价：`Parse` 在 5 MiB 上逐个跑 `markdowntest.Pathological()` 的输入；经接口量"看不到页面的人写 5 MiB 正文"、阅读视图、并发请求的耗时、分配与堆峰值；限速上传（约 1.1 Mbit/s）对请求期限的影响。数字见 P1–P3。<br>• 并发：一个探针交错（会话里的保存停在读会话之后时，本人结束会话）在原代码与去掉 `LockSession` 的 `FOR UPDATE` 之后各跑一次（T1）。<br>• 反向对照：共 32 次运行（作者的 13 项抽查全部失败；我的 17 项里 15 项失败、2 项存活；另有 2 项是同一变异换一组只连真实数据库的测试重跑，存活；对应 T1–T3），汇总见文末。没有跑 `make image-smoke` 与 `make e2e`（按要求；作者报告 `make e2e` 131 个通过），e2e 只读了故事 |
| 日期 | 2026-10-03 |
| 结论 | **不建议按现状合并；修完 P1、对 P2 定下做法之后合并。** T1、D1 建议合并前一并处理，其余可随手处理。<br>• 正文写入在 404、403 之前解析正文（3.6 的码的次序把 `Parse` 放在找页面之前）：任何已登录的账户——不在任何工作区、页面 id 随便写——一个 5 MiB 的病态正文就让服务端 1.4 秒、分配约 2 GB、堆峰值多出约 1.5 GB，然后答 404；同一个账户 8 个并发请求，堆峰值多出 12 GB（P1）。这样的正文一旦写进页面，每次阅读视图同样 1.5 GB（P2）。P3 接受了"开销线性、分配不超过普通文档 14 倍"，并把并发留给 M12（P3 审查 Q3）；P4 是这些正文第一次写得进来、而且在授权之前就被解析的地方，所以要在这里定。<br>• 共 Critical 0、Major 2、Minor 3、Nit 9；没有发现安全与隐私上的缺陷（S）。<br>• 做得好的：写入单元的加锁次序（工作区 `FOR SHARE` → 笔记本 `FOR SHARE` → 正文行 `FOR NO KEY UPDATE` → 会话行 `FOR UPDATE`）、码的次序、"会话的连续保存一个变更集、夹进别人的写另起一个"、与当前正文相同不写，都与 3.4 一致，用例测试与我的变异都钉得住；心跳"不加锁读 → 按读判定 → 单条语句"与结束"只看本人、活着"的实现与 3.5 一致，别人的会话在授权之前就由 SQL 筛掉，读者问不出别人会话的存在（FINDLIVE-NO-USER 让矩阵失败）；三条删除路径删会话、只告诉活着的，清理 `SKIP LOCKED` 分批，活动来源经 `listOwnerlessNotebooks`，组合根、契约、文案、迁移与授权文件齐全；交错的写法（持笔记本行而不是工作区行，as-built 第 7 条）是对的；作者的控制我抽查的 13 项在最终代码上全部失败 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P1 | Major | **正文在 404、403 之前解析，任何已登录的账户都能让服务端解析 5 MiB**。`app/put_page_content.go:42` 先 `parsed()`（`CheckContent` 加 `Parse`，`app/content.go:9-14`），`:46` 才找页面，判定在单元里；`app/create_page.go:33` 同样在 `notebook.not_found` 之前；`app/content_test.go:52` 把"Parse 在 FindNode 之前"钉成了期望。实测（探针经接口，dana 不在任何工作区，页面 id 不存在，答 404）：普通 5 MiB 正文 289 ms、分配 346 MiB、堆峰值 +204 MiB；病态的 `a*_` 5 MiB 1.43 s、分配 1993 MiB、堆峰值 +1464 MiB；**同一账户 8 个并发：3.3 s，堆峰值 +12183 MiB**。`authenticated` 桶每个凭证 burst 200，一个账户可以开多个 PAT。单独量 `Parse`（`markdowntest.Pathological()` 的输入放大到 5 MiB）：分配最高 1.95 GB、解析结果存活的堆最高 1.05 GB、耗时最高 1.5 s，17 类分配超过 1 GB。失败场景：被移出工作区但账户还在的人、别的工作区的访客、坏掉的 agent，几个并发的写请求就让 4–8 GB 内存的实例 OOM；在 `auth.signup_enabled` 打开的部署里任何注册的人都可以。码的次序本身没有问题（正文的 422 先于 404、403 是 13.1 第 4 条定的），问题在"422 之前"连带了昂贵的 `Parse`。建议：`CheckContent`（便宜，422 仍在最前）→ 不加锁读出节点 → 不加锁的预判定（照 `readable()`，`page.write`、`node.create`；看不到答 404、读者答 403，码与单元的相同）→ `Parse`（仍在事务之外）→ 单元在锁下再判定一次。这样只有能写的人能触发解析，码的次序不变；3.6 与 `TestPutPageContentParsesThenLocksThenDecides` 随之改写，加"看不到的人与读者不触发 Parse"的测试与反向对照 || 已修（`8dd2534`）。`Writer.Allowed` 用单元的 `workspaceOf` 与授权不加锁地预判定，码与单元相同；`app.ContentParser.Parse`：空正文直接解析，否则先 `Allowed`、再取解析预算、再 `Parse`，额度在单元结束之后放回。写正文与带正文的新建都走它（新建是 `notebook.not_found`、403 在解析之前）。码的次序不变，单元在锁下再判定一次。测试：`TestPutPageContentDecidesThenParsesThenLocks`（替换原来的 ParsesThenLocksThenDecides）、`TestOnlyAWriterMakesTheContentParsed`、`TestCreatePageDecidesBeforeItParses`；FIX-NO-ALLOWED 失败 |
| P2 | Major | **最大正文的单次解析、渲染占用约 1.5 GB 堆，没有任何并发上限**。P1 修好之后，能写的人（含 PAT 与 agent）仍可以写进一页 5 MiB 的病态正文；之后每次阅读视图（`GetPageView`：`Parse` + `Render`）实测 1.32 s、分配 2001 MiB、堆峰值 +1475 MiB，4 个并发阅读 +5840 MiB——一个团队打开同一页就够了，阅读只需要读权限。普通 5 MiB 是 +204 MiB。P3 的耗时检查只比较"病态 / 普通"的倍数（分配 ≤ 14 倍），不看绝对值；P3 审查 Q3 把并发交给 M12 的压测，但 P4 之前接口写不进正文，这个代价不可达，从 P4 起可达。建议在本 Phase 定下一种（属于负责人的决定）：① 按字节计的全局并发预算，解析与渲染进入之前取额度（例如同时在途不超过 N MiB 的正文），取不到答 503 `server_busy` 带 `Retry-After`（与 argon2 的并发名额同一先例，13.1 第 19 条）；② 降低 `MaxContentBytes`（解析的存活堆约为正文的 200–290 倍）；③ 部署文档写明内存要求与 `GOMEMLIMIT`。①最直接，也同时兜住 P1 的残余（写的人自己）。若决定留给 M12，写进第 10 节风险与 P4 第 7 节的已知差异，并给出时间点 || 已修，选 ①（负责人可改）：按字节计的全局解析预算 `page.parse_budget_bytes`（默认 8 MiB，至少一页正文的上限 5 MiB，`config.MinParseBudgetBytes`），`adapter/markdown.Budget`（`x/sync/semaphore` 按字节加权）。写入的解析、阅读视图的解析与渲染进入之前取正文字节数的额度，最多等 `page.parse_max_wait`（默认 2 s），等不到答 503 `server_busy`（Retry-After 1 s）并记 "content parsing is saturated"；请求自己被取消或到期答它自己的错误。照 `auth.password.max_concurrent_hashes` 的先例。契约给 `createPage`、`putPageContent`、`getPageView` 加 `server_busy`。默认 8 MiB：最坏同时约 1.6 个最大的病态正文、约 2.4 GB 堆；普通正文约 40 倍。FIX-NO-TAKE-WRITE、FIX-NO-TAKE-VIEW、FIX-NO-RELEASE-PUT/CREATE、FIX-BUDGET-*（5 项）、FIX-VALIDATE-MIN-BUDGET/MAX-WAIT 失败 |
| P3 | Minor | **大正文在慢的上行链路上存不进去，答 500**。两条放宽了请求体的路由仍用 `server.request_timeout`（15 s）与 `read_timeout`（30 s）；请求期限的中间件在读请求体之前就开始计时（`platform/httpserver/api.go:156-157` 的次序：期限 → 上限 → … → 结构检查里 `bodyshape/middleware.go:24` 的 `io.ReadAll`）。实测（基础配置的期限）：2.3 MB 的请求体用 17 秒传完（约 1.1 Mbit/s）→ 500 `internal_error`，页面不变（期限到期后数据库调用失败，平台按设计答 500）。由此：普通 5 MiB 正文要在 15 秒内到达需要约 2.8 Mbit/s 的上行，转义成 30 MiB 的最坏情况需要约 17 Mbit/s；1 MB 的页面在 0.5 Mbit/s 的链路上同样失败。P6 的编辑器只会看到 500 并反复重试。3.8 只按字节定了上限，没有按时间。建议：这两条路由的期限从请求体读完之后起算（或单独给一个更长的期限，`read_timeout` 一并核对），并在 3.8 与给 P6 的移交里写明带宽的前提；至少让"请求体没读完就到期"答一个可辨认的码，而不是 500 || 已修：`APIConfig.BodyReadTimeout`（组合根给 `server.read_timeout`）：`BodyLimits` 的路由期限是 `request_timeout + read_timeout`；配置加交叉规则 `read_timeout + request_timeout < write_timeout`（默认 30 s + 15 s < 60 s）；`config.yaml` 写明 5 MiB 在 30 秒内传完约需 1.4 Mbit/s 的上行，更慢的链路调大 `read_timeout` 与 `write_timeout`。测试配置的 `write_timeout` 随之改为 10 s。`TestARouteBodyLimitLengthensItsDeadline`；FIX-DEADLINE-NO-READ、FIX-API-READTIMEOUT-CHECK、FIX-VALIDATE-CROSS 失败 |
| P4 | Nit | 活动的"最后写入"（`queries/activity.sql:5-10`）对每本笔记本取它全部未删变更集的 `max(updated_at)`，只有 `changesets_notebook_id_idx (notebook_id)` 可用：每次 REST 写都是一个变更集，agent 写了几个月的笔记本有十万级变更集，每次无主清单都要读完。只给工作区管理员、很少调用，可以接受；在意时加 `(notebook_id, updated_at) WHERE deleted_at IS NULL` 的索引。3.2 写的"两条语句"实际是一条带相关子查询的语句，结果相同 || 不改：只给工作区管理员、很少调用；写进 P4 文档第 7 节，数据量大时再加 `(notebook_id, updated_at)` 的部分索引。3.2 随代码改为一条语句 |
| P5 | Nit | 心跳改的是有索引的 `expires_at`（`00017_page_edit_sessions.sql:29`）：每个编辑者每 20 秒一次的心跳都是非 HOT 更新，四个索引各写一项；而这个索引只服务十分钟一次、扫一张只有活着的会话的小表的清理。可以去掉它（顺序扫描足够）让心跳走 HOT；可选 || 已修：迁移 00017（未合并）去掉 `edit_sessions_expires_at_idx`，注释写明原因；`schema_test` 随之 |
| T1 | Minor | **会话行的 `FOR UPDATE` 没有测试守住**：SESSION-NO-FOR-UPDATE（`queries/sessions.sql:7-12` 去掉 `FOR UPDATE`）在 page 模块全部测试与 bootstrap 的正文、会话、交错测试下存活。探针：会话里第二次保存停在读会话之后（测试持会话的变更集行，保存在 `TouchChangeset` 上等），本人此时结束会话。原代码：结束等保存提交之后才删行（"the end waits for the save"），先后是"保存 → 结束"；去掉 `FOR UPDATE`：结束立刻答 204、订阅者被告知"已结束"，随后保存仍以 200 写进这个已结束的会话，`SetSessionWrite`（`:14-16`）更新 0 行也不报错。M4 没有订阅者，后果只是次序；M5 的强制解锁与锁的推送正依赖"结束之后不再有这个会话的写"。建议：加一个交错（结束与会话里的保存，持保存在锁会话之后要碰的行，或 `WaitForLockWaitsOn(…, "edit_sessions", 1)`），或仓储测试"`LockSession` 持有时 `EndSession` 等待"；`SetSessionWrite` 可以核对改到了一行 || 已修：交错 44 `TestEndingASessionASaveInItHolds`（持会话的变更集行，会话里的第二次保存在 `TouchChangeset` 上等，本人的结束在会话行上等；保存 200、结束 204、会话没了、两次保存一个版本行）；新的 `interleaveBehind` 让第二步等另一张表的行。`SetSessionWrite` 改为 `:execrows`，改到的不是一行就报错，仓储测试加"不存在的会话"。FIX-SESSION-NO-FOR-UPDATE（11 秒）、FIX-SETWRITE-ROWS 失败 |
| T2 | Nit | 3.4 第 2、3 步的次序（先查会话、再看是否与当前正文相同）没有测试：SAME-BEFORE-SESSION（把"相同不写"挪到会话检查之前）在全部测试下存活。回归之后，带已结束会话、正文恰好未变的保存答 200 而不是 409 `page.edit_session_ended`，编辑器以为会话还活着。码的次序表里加一格"已结束的会话、与当前相同的正文 → 409" || 已修：码的次序表加"不存在的会话、与当前相同的正文 → 409 `page.edit_session_ended`"；FIX-SAME-BEFORE-SESSION 失败 |
| T3 | Nit | 两条核心规则只由假端口的用例测试钉住：RESUME-ANY（会话的变更集不看 `revision`）与 SUBTREE-SESSIONS-ROOT-ONLY（删子树只删根页的会话）只跑真实数据库的测试（模块根、适配器、bootstrap）时都存活。e2e 与整个程序的测试里没有"会话写过、别人写、会话再写 → 两个变更集"（PG8 里会话在 PAT 写之前没写过，本来就新建变更集），子树删除也只在根页上开过会话。用例测试抓得到，所以不是缺口，只是 3.12 PG7/PG8 与第 5 节"整个程序"可以各补一格，让版本行"前后之间没有别人的改动"在真实 SQL（`RecordRevision` 的 `ON CONFLICT`）上也有证明 || 已修：整个程序的测试 `TestASessionsSavesAroundAnotherWrite`（会话两次保存、alice 一次、会话再一次 → 版本 `-→1`、`1→3`、`3→4`、`4→5`，会话指向最后一个变更集）与 `TestDeletingASubtreeDeletesItsSessions`（子页、孙页的会话删掉，旁边一页的两个留下）；FIX-RESUME-ANY、FIX-SUBTREE-SESSIONS-ROOT-ONLY 只跑这两个测试就失败 |
| T4 | Nit | 交错 43 最后两条断言（清理之后心跳 404、带它的保存 409，`interleavings_content_test.go:282-293`）不依赖清理：会话是用 SQL 推到过期的，`FindLiveSession` 与 `writersSession` 本来就拒绝它。证明清理行为的只有前面的行数。可以删去或改述为"过期会话的码"，避免读者以为它们验证了清理 || 已改：交错 43 的注释写明最后两条是清理删掉的会话的码，不是清理的性质 |
| N1 | Nit | `SessionEnded` 没有 3.7 列出的 `WorkspaceID`（`app/extension.go:109-117`），"edit session ended" 的日志没有 3.5 列出的 `workspace_id`（`app/end_edit_session.go:48` 传零值）；as-built 没有记。结束不读笔记本是有意的（不取工作区行），删除路径手里都有工作区。补上（删除路径直接给，结束不加锁读一次）或改文档；M5 的订阅者要推送时需要知道 || 已修：`SessionEnded.WorkspaceID`；`tellEnded` 带工作区，删除的两条路径直接给；结束在事务里不加锁读一次笔记本的工作区（读不到报错：会话在，笔记本就在），日志带 `workspace_id`。FIX-END-WORKSPACE、FIX-END-LOG-WORKSPACE、FIX-DELETE-WORKSPACE、FIX-NOTEBOOK-DELETE-WORKSPACE 失败 |
| N2 | Nit | `app/ports.go:241-251` 的 `NotebookActivity`、`Activities` 不是 app 的端口（app 里没有用它们的代码，只是模块根把 store 交出去的形状）；`EditSession.Alive`（`:203`）是领域规则。另外 `WriterDeps.Sessions` 的类型是 `SessionWriter`，而 `:224` 另有一个接口就叫 `Sessions`（心跳与结束的），读的时候容易混。建议：活动的两个类型放到模块根（`page/activity.go` 已有别名）或单独的 `app/activity.go`（3.1 原来的写法）；字段改名 `SessionWriter` || 已修：活动的两个类型移到 `app/activity.go`；`WriterDeps.Sessions` 改名 `SessionWriter`。`EditSession.Alive` 留在 app：它是 app 的会话类型的方法，挪进领域要搬整个类型，不划算 |
| N3 | Nit | 哈希在事务里算：`app/unit.go:224-227` 的 `content()` 在正文行的锁下对至多 5 MiB 做 `[]byte` 复制与 SHA-256；3.4 原写 `ContentWrite` 带事务之前算好的 `Hash`（13.1 第 19 条"事务里只有锁、比较与写"）。几毫秒的事，挪进 `parsed()` 一并做即可 || 不改，文档随代码：哈希在单元里算。5 MiB 的 SHA-256 几毫秒，参与者追加的正文写本来就在事务里算；3.4 改写 |
| N4 | Nit | 契约与文案：`api/modules/page.yaml:41`、`:119`、`:582` 写"5 MB"，上限实际是 5 MiB（5,242,880 字节），建议写出字节数；409 的文案 "Someone changed this page since you read it." / "被别人改过了"（`en.ts:326`、`zh-CN.ts:313`），本人在另一个标签页或经 PAT 写过同样触发，建议去掉"别人" || 已修：契约三处与领域的消息写"5 MiB (5,242,880 bytes)"；409 的文案改为 "This page has changed since you read it." / "这个页面在你读取之后有了新的改动。" |
| D1 | Minor | **会话的 `client` 只记不比**：`edit_sessions.client` 只进日志，`writersSession`（`app/unit_content.go:99-110`）不看它。网页开的会话可以被同一账户经 PAT 的写点名（反之亦然），写进会话的变更集，而变更集的 `client` 是它第一次写的那个，之后不同客户端的写都记在它名下，与 13.1 第 2 条"每个写入都能追溯到执行者与客户端"不符；M9 的 `mcp:<名>` 同样。建议定一条：客户端不同时答 409 `page.edit_session_ended`，或不续用会话的变更集（另起一个）；若认为会话只属于账户、客户端无关，写进 3.4 || 已修，选"客户端不同答 409"：`writersSession` 比较 `s.Client != u.write.Client`；契约写明会话里的写要来自开启它的客户端（网页或令牌）；码的次序表加一格；FIX-CLIENT-NOT-COMPARED 失败 |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| D-a | （as-built 1）S2 已实现正文写里会话的部分，S3 补开启、心跳、结束、删除与清理 | 同意，与 3.4 一致 |
| D-b | （2）`Jobs()` 在 module.go；`NotebookActivity`、`Activities` 在 app/ports.go | 同意前一半；后一半见 N2 |
| D-c | （3）矩阵五行：每列在它的目标页上用 SQL 种一个活着一小时的会话，"别人的"属于 acme 的管理员；已删笔记本那一列的会话随笔记本删掉 | 同意：这样每格都由调用时的角色决定，"别人的"对每一列都是别人的。附带：已删笔记本那一列的心跳答 404 是因为会话不在了，心跳里"笔记本已删 → 404"那一支由用例测试（S3-HB-NO-NOTEBOOK）守住 |
| D-d | （4）测试配置加 `Page.EditSessionCleanupInterval: 1h`；为 0 时 River 不停地排它，饿死了 identity 的清理 | 同意。`testConfig` 不经 `validate`，下一个定时任务会再踩一次：可以考虑让 `testConfig` 过一遍校验（不在本 Phase 范围） |
| D-e | （5）加了清理任务接线的整个程序测试与运行时角色测试里的过期会话 | 同意；CLEANUP-NO-RUN-ON-START 由运行时角色测试抓到（测试配置的间隔是一小时，只能靠启动时那一次） |
| D-f | （6）交错 38–43 放在新文件 | 同意 |
| D-g | （7）39、40（笔记本）、41 持笔记本行而不是工作区行 | 同意，理由成立：放开一行时，排队的两个 `FOR SHARE` 先后拿到元组锁、彼此兼容，一起往下走，到笔记本行才分先后，先后不定；持笔记本行（保存 `FOR SHARE`、对方 `FOR NO KEY UPDATE`）才确定。五次连跑全过 |
| D-h | （8）交错 43 用 serve 的任务（间隔 1 秒）而不是用例加假时钟；会话用 SQL 推到过期；等持有期间完成两次运行 | 同意不为测试导出用例。等"两次完成"可靠（S4-CLEANUP-NO-SKIP 11 秒失败）。最后两条断言见 T4 |
| D-i | （9）"`BodyLimits()` 的每个键都是路由"放在模块根 | 同意：整个程序的测试另外证明组合根交给了平台（S4-NO-BODY-LIMITS 失败） |
| D-j | （10）请求体上限的测试用基础配置的期限：竞态检测下解码 30 MB 的转义约 3 秒 | 同意。不带竞态检测我量到 30 MiB 的转义请求体从发出到 404 共 345 ms、分配 205 MiB。同一个期限对真实用户的影响更大的是上传时间（P3） |
| D-k | （11）PG12 另核对笔记本管理员也看不到别人的会话 | 同意 |
| D-l | 作者 `mut_s1.out`、`mut_s2.out`、`mut_s3.out` 里有 5 项不是"失败"：S1-LOCK-NO-LOCK、S1-LOCK-NOTEBOOK 超时，S1-CONTENT-NUL、S2-NO-CHECK 编译失败，S3-CONFIG-VALIDATE 通过 | 在 `d03c993` 上原样重跑，5 项都失败：S1-LOCK-NO-LOCK 要 11 秒（`WaitForLockWaitsOn` 的 10 秒期限，作者的运行器超时更短）；S3-CONFIG-VALIDATE 由之后加的 999 ms 一格抓到。as-built "each mutant failed" 对最终代码成立，但 `.out` 是旧的运行，建议记录换成最终的结果 |
| Q1 | M5 的结束订阅者与加锁次序 | 结束先删会话行（持 `edit_sessions` 的行锁）再调用订阅者；会话里的保存是 `page_contents → edit_sessions`。M5 的订阅者或强制解锁若再去锁正文行或节点行，次序反转，与并发的保存可能死锁。建议写进给 M5 的移交：结束的订阅者不锁 `page_contents`、`nodes`，或强制解锁先取正文行再删会话 |
| Q2 | 与当前正文相同的保存不调用守卫 | 是 3.4 第 3 步有意的；M5 的锁守卫因此不会拒绝一个没持锁、但正文恰好未变的写（答 200、不写）。无害，写进给 M5 的移交即可 |
| Q3 | 被移出工作区、降为阅读者的人的会话不结束，订阅者也不被告知 | 与设计一致（过期不是事件，一个租约之内结束；心跳答 404、403，写在单元里答 404、403）。M5 有锁之后，锁会在移出之后最多再留 60 秒，M5 设计时考虑 |
| Q4 | 会话的时刻全取应用的时钟 | 多实例的时钟偏差或时钟回拨超过一个租约时，心跳写出的 `expires_at` 可能早于 `created_at`，违反 `edit_sessions_expires_at_check` 答 500。很罕见，不作为发现 |
| Q5 | 每次开启都新建一行，同一账户同一页可以有任意多个会话 | M4 不排他，按设计；限流兜住，过期的由清理删除。M5 的否决者会收紧 |

## 文档与代码的不一致

作者已知的偏差（D-a…D-k）之外：

- 3.1 的文件清单：没有 `app/activity.go`、`jobs.go`（D-b）；交错在 `interleavings_content_test.go`（D-f）。
- 3.2：活动是一条带相关子查询的语句，不是两条（P4）；迁移另有 `edit_sessions_expires_at_check` 与 `revision >= 1`（比文档严，好）。
- 3.4：`ContentWrite` 没有 `Hash`，哈希在单元里算（N3）。
- 3.5：清理删的是 `expires_at <= now`（文档写 `<`；代码与"`expires_at > now` 为活着"一致，文档随代码改）；"edit session ended" 的日志没有 `workspace_id`（N1）。
- 3.6 与 `TestPutPageContentParsesThenLocksThenDecides`：P1 修好之后"事务之前：取值检查、Parse、不加锁读出节点"的次序要改。
- 3.7：`SessionEnded` 没有 `WorkspaceID`（N1）。
- 3.8：只按字节定了上限，没有带宽与期限的前提（P3）。
- 3.11 第 43 行"清理删掉之后心跳 404、保存 409"不是清理的性质（T4）。
- 第 10 节风险与第 7 节"结果"：P2 的决定。

## 反向对照

做法：在快照里每次改一处（Python 按原文唯一匹配替换，sqlc 的语句改生成的 `*.sql.go`），只跑相关的包（page 模块全部、`internal/bootstrap` 里正文、会话、交错、活动、删除、清理任务的测试，必要时矩阵与运行时角色测试），然后原样写回；运行器与每项的日志在审查目录的 `rv/mut/`。审查者的探针测试带 `//go:build review`，不进这些运行。

会失败的（28 项）：

- 作者的控制，抽查重跑（13 项，都成立）：S1-LOCK-NO-LOCK（11 秒）、S1-LOCK-NOTEBOOK、S1-CONTENT-NUL、S2-NO-BASE、S2-NO-CHECK、S2-SESSION-USER、S2-RESUME-ANY、S3-TELL-EXPIRED、S3-HB-NO-AUTH-MATRIX、S3-CONFIG-VALIDATE、S4-NB-DELETION-NO-SESSIONS、S4-CLEANUP-NO-SKIP（交错 43，11 秒）、S4-NO-BODY-LIMITS。
- 正文的写：SAME-SIZE-ONLY（"相同"只比字节数，`TestAnEditSessionsWritesAreOneChangeset` 的 "one"/"two" 同长）、SET-SESSION-WRITE-OLD-REVISION（会话记下的是写之前的版本）、LOCK-FOR-SHARE（闸门降为 `FOR SHARE`：`TestLockContent` 与交错 38 都失败）、HTTP-NO-SESSION（处理函数丢掉 `edit_session_id`）、CREATE-ROUTE-NO-LIMIT（带正文的新建不放宽请求体）。
- 会话：FINDLIVE-NO-USER（不加锁的读不看 `user_id`：矩阵"别人的会话"一行里阅读者的列答 403 而不是 404，即泄露会话的存在）、CLEANUP-STRICT（清理只删 `expires_at < now`）、CLEANUP-NO-RUN-ON-START（运行时角色测试）、OPEN-TREE-LOCK（开启锁树）、DELETE-NO-TELL（删子树不告诉订阅者）、SUBTREE-SESSIONS-ROOT-ONLY（用例测试）、END-NO-LOG。
- 活动与授权：ACTIVITY-CREATED-AT（取变更集的 `created_at`：会话的第二次保存不再推后最后活动）、ACTIVITY-NO-LASTWRITE（组合根不转换最后写入）、RULE-WRITE-READERS（规则表把 `page.write` 给阅读者，矩阵失败）。

改了却没有失败的（2 项，各对应一个发现）：

- SESSION-NO-FOR-UPDATE（T1）；SAME-BEFORE-SESSION（T2）。

换一组测试重跑时存活的（2 项，用例测试抓得到，见 T3）：

- RESUME-ANY-REALDB、SUBTREE-SESSIONS-ROOT-ONLY-REALDB：只跑模块根、适配器与 bootstrap 的测试。

探针（不是变异，作为 P1–P3、T1 的依据）：

- `Parse` 在 5 MiB 上的代价（普通文档 258 ms、分配 296 MB、存活 141 MB；病态输入最高 1.5 s、1951 MB、存活 1050 MB）。
- 经接口：看不到页面的人写 5 MiB（普通、病态、30 MiB 转义）、8 个并发；阅读视图 1 个与 4 个并发；限速上传。
- 会话里的保存与本人结束的交错，原代码"结束等保存"，去掉 `FOR UPDATE` 之后"结束先答 204、保存随后写进已结束的会话"。

## 没能验证的风险

- 数字来自本机（Apple Silicon，Docker Desktop 给容器 7.9 GB，期间别的会话在跑测试，负载 4–7）；堆峰值是测试进程里每 5 ms 采样的 `HeapAlloc`，含测试客户端一侧的少量分配。8 个并发以上没有跑（本机内存）。
- 没有跑 `make e2e`、`make image-smoke`（按要求）；PG5–PG10、PG12、PG14 只读了故事与断言，判断它们与 3.12 一致，没有在浏览器与构建产物上复现。
- P3 只量了一种带宽（约 1.1 Mbit/s）与一个大小（2.3 MB）；更慢的链路上 `read_timeout` 先到时的答复没有量。
- 多实例部署下的时钟偏差（Q4）与 River 在多实例下只由 leader 跑定时任务，都只按代码推理。

## 修复的核对

修复 `8dd2534` 另经 Opus 在快照上核对（报告原文在审查目录之外的草稿里，要点如下）：15 条处置全部属实，P1 的探针从 1.43 s、约 2 GB 分配降到 26 ms、36 MiB（看不到页面的人写 5 MiB 病态正文）；预算在 `-race` 下 400 个 goroutine 压测，同时占用的峰值等于预算；P3 的慢上传探针 4.5 s 传完答 200（修之前 500）；交错 44 带 `-race` 连跑 15 次全过；N1 的结束没有走得到 500 的路径。新发现 5 项，合并前处理：

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| C1 | Minor | `Parse` panic 时预算永久泄漏：`ContentParser.Parse` 取了额度之后直接返回解析结果，释放要等调用方 `defer`；平台的 recover 答 500、进程照常运行，那份额度再不还，累计之后大页面的写与阅读视图一律 503，直到重启 | 已修（`209ed11`）：解析没有返回时由 `Parse` 自己放回；`TestAParseThatPanicsGivesTheBudgetBack`；FC-PANIC-LEAK 失败 |
| C2 | Minor | 预算的接线没有测试守住：去掉组合根的 `ParseBudgetBytes`，整个 bootstrap 照样通过；大小为 0 的预算静默不设限 | 已修：`NewBudget` 遇到大小 < 1 或等待 ≤ 0 时 panic（配置的校验本来就排除它们，`TestABudgetOfNothingIsRefused`）；整个程序的测试 `TestTheParseBudgetBoundsWritesAndViews`（预算 64 KiB：A 页的写在正文行上等锁、占着额度，B 页的阅读视图答 503 `server_busy`、`Retry-After: 1`，放开之后 A 200、B 200）；FC-NEWBUDGET-ZERO、FC-DEPS-BUDGET、FC-DEPS-WAIT、FC-TWO-BUDGETS（阅读视图另用一个预算）失败 |
| C3 | Minor | 写入在单元里等锁的整段时间都占着预算（解析结果要活到单元结束），FIFO 让小请求排在大请求之后：一页的锁争用可以让全实例的大页面阅读视图 503 | 设计取舍，写进 P4 文档第 10 节风险与第 7 节；缓解（正文写入的单元设 `lock_timeout`）与 M12 的压测一并考虑 |
| C4 | Nit | 错误路径上的释放只有 `PutPageContent` 有测试：`GetPageView` 只在渲染成功后释放、`CreatePage` 只在成功后释放，两个变异都存活 | 已修：渲染出错的测试给页面一段正文并核对额度放回；新建的码的次序表加"标题已被占用、带正文"一格并核对额度；FC-VIEW-RELEASE-ON-SUCCESS、FC-CREATE-RELEASE-ON-SUCCESS 失败 |
| C5 | Nit | 一行注释没折行；`Allowed` 让每次非空写多读一次工作区与授权 | 注释已折行；多出的一次读不改（开销小，预判定与单元各读一次才各自成立） |

修复的反向对照：`8dd2534` 的 24 项（FIX-*）全部失败，其中 FIX-BUDGET-CANCEL-IS-BUSY 先存活，补了"请求自己的期限先到"一格；核对之后的 7 项（FC-*）全部失败，其中两项先存活或编译失败，补了测试、改写了变异。S1–S5 的 94 项在 `8dd2534` 上重跑，全部失败（6 项随代码改写了原文）。

疑问的去向：Q1–Q4 写进 M4 总设计第 8 节"编辑会话"与 P4 文档第 7 节，M4 收尾时进给 M5 的移交；Q5 按设计，M5 的否决者收紧。D-d（`testConfig` 不经校验）不在本 Phase 处理，测试配置的 `write_timeout` 已改得满足新的交叉规则。D-l：S1–S5 的反向对照在 `8dd2534` 上重跑，见上。
