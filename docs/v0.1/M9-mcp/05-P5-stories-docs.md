# M9/P5 其余的故事的 MCP 版本与接入文档：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M9/P5 其余的故事的 MCP 版本与接入文档（一个 Step：前面 M 的 21 个故事的 MCP 版本与 fixture 的四处改动、README 的"接入 agent（MCP）"与部署一节的反向代理、人工清单 `manual/P5-mcp-clients-checklist.md`、总体设计与 00 号文档的修订） |
| 日期 | 2026-10-23 |
| 状态 | S1 已合并（`053d3ba1`），P5 完成；设计已审查（三位审查者），照主持者的处置修订四次，复核三轮（前两轮由三位，第三轮由 C 与主持者）；审查记录 [P5-design-review.md](reviews/P5-design-review.md)；S1 的审查记录 [P5-S1-stories-docs-review.md](reviews/P5-S1-stories-docs-review.md)，实施与审查之后与代码不同之处照实改在各节（3.9 列出），4.1 与 6.1 是 README 与清单建成的样子，结果在第 12 节 |
| 上级文档 | [M9 总设计](00-M9-design.md) 4.4、4.5、4.6、4.10、4.14、4.16、第 7、9、10、11 节与"移交的落实""写给后面的 M""总体设计的修订"；[P1](01-P1-platform-reads.md)、[P2](02-P2-writes-history.md)、[P3](03-P3-batch-revert.md)、[P4](04-P4-grep-search.md) 各自的第 8 节；[v0.1 总体设计](../v0.1-design.md) 6.1–6.4、10.1、11、13.1、13.2、13.4；移交 [M0/P1 MCP 会话](handoffs/M0-P1-mcp-sessions.md)第 1、3 项；[M0/P1 的实验](../M0-foundation/01-P1-spikes.md)第 7 节；[M4/P6 的输入法清单](../M4-pages/manual/P6-ime-checklist.md)（清单的格式）；[M5/P2](../M5-collab-editing/02-P2-event-stream.md)第 7 节（反向代理的容器） |
| 代码位置的写法 | 同 P1–P4：模块写作 `page/adapter/mcp/read.go:94`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…` 在 `server/internal/` 之下；e2e 写作 `e2e/fixtures/mcp.ts:114`、`e2e/stories/page/pg1-create-page.spec.ts:25`（一节里反复引同一个文件时只写 `pg1:25` 一类）；README 写作 `README.md:219`。代码与 README 的行号是 `7dc4a686` 的（P4 的文档提交），文档的行号同样 |
| 研究与实验 | 全部离线，在 `scratchpad/m9/p5/` 之下：`research/` 是四份逐故事的核对（页面的写、读与推送与搜索、链接与附件、豁免；各自对着 `7dc4a686` 的代码，`file:line` 照它们在附录里重列）；`exp/insts/` 数清单的 C12 步的模式编译之后的指令（`\pL{30}完成` 34 条，4 MiB 的一行只匹配得到开头约 2.2 MB，进 `partial_lines`）；`exp/caddy/` 是 Caddy 的按路由缓冲的重跑（设计审查 B 的 `Caddyfile` 与 `run.sh`，经 `heavy.sh` 在本机已有的 `caddy:2.10-alpine` 里，`--rm --pull=never --network none`、没有映射端口）、缓冲在内存里还是磁盘上的一次量（`mem.sh`）与 README 的 Caddy 样例的 `caddy validate`（`readme/`）；审查 B 的第二轮的两次（代理与上游分开进程时的内存 `mem-separate/`，经 Caddy 时服务端早答的拒绝 `early/`）与第三轮的一次（nginx 每个大小三次、替身答 401 与 429 时的 Caddy，`early2/`）复制在同一处。没有运行任何 MCP 客户端，没有下载，没有改仓库 |

---

## 0. 一个 Step

00 号文档第 7 节说 P5 轻。它没有服务端的代码（2.3：P1–P4 第 8 节给 P5 的没有一条要改服务端），只有三样东西：e2e 的改动、README 的两处文字、一份清单。e2e 的改动是：21 个新的测试；MC3 (activity) 挪走，成为 PG14 (MCP)（MC3 去掉它与只有它用的导入）；SE1 的 `expectFound` 加一个调用处的选项（MCP 的调用处不比命中的范围）；PG2、PG9、HI1 的几个常量提到文件的顶层，两个测试共用（HI1 的四个；HI1 (page) 的三个同名的纯文本常量随之改名为 `plainCreated`、`plainSecond`、`plainThird`，3.9）；fixture 的三处类型的放宽与两个帮助函数（新的 `writtenOf`，与从 MC2 挪来的 `readPage` 及其类型 `PageRead`）；L6 (API) 与 C7 (API) 的事件流的检查各补一两行（3.3、3.4）。三样互不依赖，却都要同一次审查读懂同一件事（MCP 的行为在故事里、在文档里怎样说），分成两个 Step 只多一次审查与一次合并、少不了什么风险：e2e 的改动每个故事在它自己的文件里，改已有的测试的只有上面列的几处；文档不碰代码。所以一个 Step **S1**（分支 `m9-p5-s1`，从 `main` 起），一次审查与修复核对，一次合并。

S1 的每个提交都是绿的（第 8 节）：先是 fixture 的一个提交（放宽的类型让今天的每个调用者照样编过，`writtenOf` 与 `readPage`、`PageRead` 在同一个提交里有调用者，过 knip；`make lint` 的 tsc 证明编得过），之后按故事的分组各一个提交（页面、协作与链接、附件、历史与搜索），每个提交的新测试都通过；README 的两处与清单各一个提交。e2e 的 CI 那一份只在合并时多跑一次（9.4）。

为什么不是两个 Step（故事一个、文档一个）：文档不依赖故事，故事也不依赖文档，分开时两次审查各读一半，却都要核对同一组事实（锁的拒绝的形状、名额、`base_version`）在测试与 README 里说得一样；合在一起，审查者看到 README 的一句时就能在 3.x 的测试里找到它被断言的那一步（例如"锁不能接管"是 C4 (MCP) 断言的文字，`c4-guard.spec.ts:108`；"本人的编辑器也拒"是 PG4 (MCP) 的第 3 步，"过期的会话不再挡 agent 的删除"是它的第 4 步）。

## 1. 基线

`main` 在 `7dc4a686`（P4 的文档提交；P4 的 S2 合并是 `ed9e149f`）。与 P5 有关的现状：

**MCP**：两个入口，笔记本入口 14 个工具、账户入口 16 个（`api/mcp/tools.json`）；读的九个与 `notebooks`、`open_notebook` 是 `readOnlyHint`，写的五个是 `destructiveHint`，其中 `delete`、`move`、`write` 另是 `idempotentHint`。内置的说明：账户入口 4,027 字节、笔记本入口 3,957 字节，4 KiB 由 `TestTheBuiltInInstructionsAreShort` 守（`page/adapter/mcp/instructions_test.go:15-27`；说明在 `instructions.go:9-74`）。边缘的答复：没有令牌的 401 的说明 `NoOAuth`（`platform/httpserver/mcpserver/edge.go:147`），坏的令牌的 401（`platform/httpserver/api.go:378-383`），403（`edge.go:193-195`），`/mcp/…` 的 404（`edge.go:132-140`），`POST /register` 的 400（`edge.go:175-181`）；`/mcp` 的请求体至多 37,814,272 字节（`bootstrap/mcp.go:29`），两个正文路由 31,522,816 字节（`page/adapter/http/limits.go:8`）；大的请求体的名额从读请求体之前占到答完（`platform/httpserver/largebody.go:123-171`），满了时 info 的一行 `large request bodies are saturated`（`:244-250`）；回环保护已关（`mcpserver/server.go:132`）。

**e2e**：

- 客户端 `e2e/fixtures/mcp.ts`：`mcpURL`、`postMcp`（不经握手的一个 POST，`:67-78`）、`mcpClient(baseURL, credential, {notebook?, client?, handshake?})`（`:114`；`send` 要 200 与 `application/json`，否则抛出，`:99-101`）、`resultOf`、`errorOf`、`lineAt`；答复的类型不导出，直到有故事导入（`:12-14`）。
- 断言：P2 放宽了 `expectNewPage`（`e2e/fixtures/assert/page.ts:16-22` 收 `Pick<Page, …>`），加了 `expectRelocated`（`:157`）、`expectChangesetOf`（`:213`）与 `expectVersions` 的客户端（`:509`）；`expectContentWritten` 仍收 `Page`（`:429-431`），`expectMoved` 仍收 `TreeNode`（`:135`），`Embedded.written` 仍是 `Page`（`assert/asset.ts:162-166`）；`expectToggleRevision` 的客户端是 `"web" | "api"`（`assert/collab.ts:44-50`）；identity 有 `expectTokenClient`。
- 故事：MC1–MC8 在 `e2e/stories/mcp/`；C4 (MCP) 在 `e2e/stories/collab/c4-guard.spec.ts:81-146`（第 9 节的表里唯一做完的前面 M 的故事）；MC3 (activity)（`mc3-write-edit.spec.ts:170-201`）是 PG14 (API) 的形状。MC2 有本地的 `PageRead`（`mc2-read-list.spec.ts:64-73`）与 `readPage`（`:76-83`），`readAll` 的签名写着 `PageRead`（`:86`）。
- 服务端：`server.public_url` 是 `http://wiki.example`（`e2e/fixtures/server.ts:150`）；请求期限是默认的 15 秒（`server.ts:139-152` 与 `server/configs/config.test.yaml` 都不设，`server/configs/config.yaml:17`）；Playwright 的 `expect` 等 10 秒（`e2e/playwright.config.ts:10`），worker 数是默认的。
- `make e2e` 本机 262 个测试约 36 秒（[P4](04-P4-grep-search.md) 9.2）。

**README**："MCP"一节在 `README.md:205-219`（P1–P4 写的：地址、令牌、传输、工具、写、`batch`、历史、找、索引与反链、说明、客户端的名称、`server.public_url`），最后一行 `:219` 是"Claude Code、Codex 的接入配置与反向代理的要求在 M9 的接入文档里写（M9/P5）。"。"部署"一节在 `:297-374`：`trusted_proxies`（`:342-343`）、事件流与 Caddy、nginx 的样例（一个 `location /`，`:344-363`）、上传（`proxy_request_buffering off`、`client_max_body_size`，`:365-367`）、内存（`:368`）；升级的说明是 `:324`（M6）、`:330`（M8）、`:332`、`:334`（M9/P4），之后 `:336` 是两个数据库角色。管理员命令（`:82-92`）按邮箱，没有按令牌或会话的。

**清单**：`docs/v0.1/M9-mcp/manual/` 还没有；格式照 `docs/v0.1/M4-pages/manual/P6-ime-checklist.md`（开头一张表、准备、分部分的步骤表、不在这份清单里的）。M0/P1 的实验记下了 Claude Code 2.1.285 经 `.mcp.json` 的 `headers` 连上、`claude -p` 调用工具与拿到说明，Codex 0.157.1 的 `bearer_token_env_var`、`default_tools_approval_mode` 的四个取值与"非交互模式默认拒绝要审批的工具"（`docs/v0.1/M0-foundation/01-P1-spikes.md:306-331`），没有留下配置的样例。

**总体设计**：6.3 已是 M9 修订过的（`v0.1-design.md:641-681`，`:654` 是接入文档与 Codex 的审批一句）；10.1 已有 MCP 版本的一句（`:887`）；第 11 节的反向代理一条已有 `/mcp` 的一句（`:896`），没有按路由缓冲；13.4 第 3 条（`:1224`）写到 P2 的 MC3、MC4、C4 与 MC8，没有 MC5–MC7 与前面 M 的故事的 MCP 版本；第 15 节的最后一行是 M9/P4 的 S2（`:1360`）。

**CI**（主持者给的，`main` 在 `7dc4a686` 的 run 39884100596，绿）：组合根三份的测试一步 7:15、5:59、3:58；e2e 的 E2E 一步 3:41；web 2:52、守卫 10:03、成本 9:10、竞态 10:07。`ed9e149f` 的 run 39878983820：第一份 4:45、第三份 4:07；它的 web 作业在 Test 一步卡住一次、15 分钟时被取消，没有再出现。第一份的单次运行在 7:30 的线附近（7:12、7:07、7:15），最近两次的平均约 6:00。分份的表达式 `BOOTSTRAP_FIRST = ^Test([A-D]|E([a-u]|very[A-R]))`、`BOOTSTRAP_THIRD = ^Test(The[F-Z]|T[i-z]|[U-Z])`（`Makefile:217-237`）。e2e 的作业时限 20 分钟（`.github/workflows/ci.yml:164-205`）。变异编号到 222（[P4](04-P4-grep-search.md) 6.3），P5 的从 223 起。迁移到 00047。

## 2. 目标与范围

P5 做 00 号文档第 7 节 P5 一行的全部：

1. **其余故事的 MCP 版本**（第 9 节的表）：21 个故事各一个"`<故事> (MCP)`"测试，在它自己的文件里，调它的 API 版本调的同一组断言，值来自 MCP 的答复（第 3 节）；fixture 的四处改动（3.1）；两个已有的测试的同一个洞一起补（L6 (API) 与 C7 (API) 的事件流的检查，3.3、3.4）。
2. **README 的接入一节**"接入 agent（MCP）"（4.16 的"接入文档"）：两个客户端的样例（每个客户端一个令牌的变量，`.mcp.json` 里写 `${NERVEWIKI_CLAUDE_TOKEN}`）、审批、上限、HTTP 的答复、名额、写的规矩；人工清单走过之前，客户端的每一句在样例里、标着"照文档"或"未核对"，或由仓库里的记录核对过，4.2 逐句列出（第 4 节）。
3. **README 的部署一节的反向代理**（4.16 的"反向代理"）：按路由的 nginx 与 Caddy 的样例、MCP 的一条与升级的一条（第 5 节）。
4. **人工清单** `docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md`（4.16 的"人工的"与 P1–P4 第 8 节加的各步，第 6 节）。
5. **总体设计与 00 号文档随之的修订**（2.1、第 7 节），与给 M10、M12 的移交（第 11 节）。

验证照第 7 节的 P5 一行："全部 e2e 通过（本 M 与之前所有 M）"（第 9 节）；"接入文档的样例与清单一致"：清单不另写样例，照抄 README 的、只换地址、`<笔记本 id>` 与变量（清单的"准备"第 5、6 步），所以一致由构造成立（4.2）。

不做：

- **服务端的代码**：P1–P4 第 8 节给 P5 的没有一条要改服务端（2.3）；内置的说明不加字（账户入口 4,027 字节，剩 69，2.3）。实施中若发现服务端的缺陷，照 M9 的做法报告主持者，不在 P5 里顺手改。
- **跑人工清单**：负责人在 v0.1 的最后与输入法清单一起跑（第 11 节的取舍第 12 条；不先跑一遍的理由见 2.1 第 10 条）；P5 写它、不跑它，也不在任何客户端上试 README 的样例。README 里客户端的每一句标出来处（4.2）。Caddy 不等清单：它的写法 P5 在本机已有的镜像里核对过（第 5 节）。
- 网页的改动；3.8 列出的可选的增加。
- 组合根的测试：P5 不加。实施中若要加，名字取 `TestThe` 加 F–Z 起头，落在第三份（9.4）。

### 2.1 对 00 号文档的修订

P5 照实改 00 号文档的这些处（行号是 `7dc4a686` 的 00 号文档的；`apply-00m9.py` 随这份设计的提交落下它们，按原文匹配，第 7 节）。

1. **断言的适配**（第 9 节 `:539`）：行号过时了，`expectNewPage` 在 `e2e/fixtures/assert/page.ts:16`、`expectContentWritten` 在 `:429`、`expectMoved` 在 `:135`（不是 `:15`、`:291`、`:128`）；`expectNewPage` 的放宽 P2 已做（`Pick<Page, "id" | "notebook_id" | "parent_id" | "name" | "created_at">`）。P5 做其余两个：`expectContentWritten` 收 `Pick<Page, "id" | "revision" | "content_updated_at">`（连同 `assert/asset.ts:162-166` 的 `Embedded.written`，`expectEmbedded` 把它交给 `expectContentWritten`），`expectMoved` 收 `Pick<TreeNode, "id" | "parent_id">`：它们只读这几个字段（`page.ts:440`、`:443`；`:144`、`:147`）；`fixtures/mcp.ts` 加 `writtenOf`（由 `write`、`edit` 的答复造出前者，`version` 与 `at` 不对时失败）与由 MC2 挪来的 `readPage`，连同它的类型 `PageRead`，两者都导出（MC2 的 `readAll` 的签名写着 `PageRead`，`mc2-read-list.spec.ts:86`；3.1）。
2. **前面 M 的故事的 MCP 版本放在哪里**（第 9 节 `:557` 的一段，写细）：每个是它的故事自己的文件里的"`<故事> (MCP)`"测试（C4 (MCP) 的做法），种下的数据照 API 版本经 REST，故事的动作经 MCP；每个 agent 一个 PAT，要分开两个 agent 的变更集时各有自己的客户端名称；都不开浏览器（3.1）。
3. **"调同一组断言"的两个例外**（同一段）：PG7 的 `expectOneSessionRevision`、`expectSessionGone` 与 PG8 的 `expectOneSessionRevision` 收编辑会话的 id，MCP 不开会话：它们属编辑会话的豁免（PG10 一类），这两个 MCP 版本以 `expectVersions` 代之（每次 MCP 的写是 agent 的客户端的一个变更集里的一个版本，逐行比全部的版本），PG7 另断言 agent 的写之后这一页没有编辑会话。API 版本不调断言函数的（PG11、PG12、PG14、C7、SE1、SE2），MCP 版本做同样的检查（REST 的答复、帧、内联的 SQL、故事自己的帮助函数），值来自 MCP 的答复（3.1）。第 9 节末尾的一句（`:598`）随之加上这两点；"总体设计的修订"一节的 13.4 第 3 条（`:493` 的"MCP 版本调同一组断言"）同样加"（会话的断言的两个例外见第 9 节）"。
4. **`collab.ts:50` 不放宽**（`:539` 的"`collab.ts:50` 的客户端类型 `"web" | "api"` 放宽到 `mcp:${string}`"）：那是 `expectToggleRevision` 的参数（`e2e/fixtures/assert/collab.ts:44-50`），只有 C10 调它，C10 豁免，没有一个 MCP 版本用它。这一句删掉。
5. **MC3 的活动挪到 PG14**（MC3 一行 `:550` 的"这本笔记本的活动计入 MCP 的写（照 PG14 的 API 版本……）"；PG14 一行 `:574` 的"MC3 里断言，P5 照 PG14 的形状另写一遍"）：MC3 (activity)（`e2e/stories/mcp/mc3-write-edit.spec.ts:170-201`）与 PG14 (API) 逐行是同样的检查，它就是 PG14 的 MCP 版本；P5 把它挪进 `pg14-notebook-activity.spec.ts`、改名为 PG14 (MCP)，照 PG14 的形状把 Notes 的第二次写改成 `edit`，不另写一个重复的（3.2）。MC3 一行去掉活动的一句，PG14 一行改为"`write` 与 `edit` 计入活动（MC3 的活动挪来，P5）"。
6. **C7 的 MCP 版本以事件流的帧断言**（C7 一行 `:577` 的"MCP 的写经事件到达网页（页面一侧的断言照旧）"）：改为"MCP 的写经事件流到达，帧照 C7 的 API 版本（id 与版本号来自答复），一个 `batch` 一个帧，最后 B 的流是安静的；网页对同样的帧的反应由 C7 的页面版本证明"：浏览器的版本做不了 C7 (page) 的排序与"正在编辑"两步，帧的断言正是 API 版本调的那一组（3.3）。
7. **AS4 的 MCP 版本只有删除**（AS4 一行 `:589`）：加"删除笔记本（PG13 的豁免）与 61 天之后的清理（平台的定时任务）没有 MCP 的形式；无主的笔记本的大小（AS4 的第二个测试）没有 MCP 版本：它读没删的附件，删除让行离开已由删除的一步看到"（3.5）。
8. **豁免的说法**（`:557` 与 TB1、TB2 一行 `:592`）：两类的第一类加上删除笔记本、释放锁、自动保存与勾选任务（PG13、C3、C5、C10 的理由），改成"如：……"之外照列；TB 一行的"MCP 的删除由 PG4 覆盖"改为"MCP 的删除进回收站由 PG4 的 MCP 版本以 TB 的 `expectTrashRoot(…, {alone: true})` 覆盖"：今天覆盖它的是 MC4（`mc4-move-delete.spec.ts:123-134`），但它的 `expectTrashRoot` 不带 `alone`（`:129`），TB1、TB2 都带（`tb1-trash.spec.ts:58`、`:146-147`、`:252`，`tb2-background-restore.spec.ts:131`、`:196`）；PG4 (MCP) 的第 4 步带着它（3.2、3.8）。
9. **慢的发送者占着名额多久，与谁能撤销**（第 10 节 `:612`）：
   - "取了名额的请求在读完请求体之前一直占着它"：代码是从读请求体之前占到这次请求答完（`platform/httpserver/largebody.go:123-171`：`defer release()` 在 `next.ServeHTTP` 之后；P2 的设计 3.2.1 本来也这样写）。所以一个慢的请求体占着名额至多 `server.read_timeout` 30 秒，之后这次调用的工作至多 `server.request_timeout` 15 秒，合起来至多约 45 秒（`mcpserver/call.go:40-56`、`platform/httpserver/api.go:297-310`）。改成这样；结论（两个凭证占得满两个名额）不变。
   - "管理员撤销那个 PAT 或结束会话"：没有这样的管理员的路。撤销 PAT 的接口只撤调用者自己的（`identity/app/revoke_api_token.go:12-15`），管理员命令按邮箱（`README.md:82-92`）。改为"由 `api_tokens` 或 `auth_sessions` 那一行的 `user_id` 找到账户，请他撤销那个令牌，或以 `nervewiki users reset-password` 撤销这个账户的全部会话与 PAT"。
   - "Caddy 默认不缓冲请求体，直接暴露也不，用它们时这个风险照旧"：改为"Caddy 默认不缓冲请求体，README 的样例以 `request_buffers` 为这三个路由缓冲（M9/P5 在 Caddy 2.10.2 上核对过；经 Caddy 时服务端早答的拒绝对几 MiB 的请求体常成 502，交 M12）；直接暴露时这个风险照旧"（第 5 节、5.2）。
   - "P5 的接入文档按路由写"：改为"README 的部署一节按路由写（nginx 与 Caddy 的样例，M9/P5）"（同 7.3 对 M12 的那一条的改法）。

   [M12 的性能移交](../M12-release/handoffs/M4-performance.md)第 18 项的"慢的发送者"一条（`:95`）同样有前两处："取了名额的请求在读完请求体之前一直占着它，请求体只受 `server.read_timeout`（30 秒）管"与"管理员能撤销那个 PAT 或结束会话"，随文档的提交同样改（7.3）。
10. **样例与人工清单**（4.16 的"接入文档"一条的末句"样例出自人工清单的那一遍"；第 3 节完成的标准第 13 条 `:67` 的"接入文档里的配置样例出自它"）：清单由负责人在 v0.1 的最后跑，P5 合并时它还没跑过。两处都改为："样例照两个客户端的文档与 M0/P1 的实验写成，人工清单照原样用它们；在清单跑过之前，README 里客户端的每一句在样例里、标着'照文档'或'未核对'，或由仓库的记录核对过，逐句列在 [P5](05-P5-stories-docs.md) 4.2，清单跑过之后照记下的改"（4.2）。**不先跑一遍是主持者的决定**：4.16（`:383`）许可实施者在负责人同意时先跑，P5 不请求，因为负责人的规则是 v0.1 的最后一次验收（取舍第 12 条），先跑一遍要的正是那条规则要省下的负责人的时间与他自己的客户端；代价是 README 带着标记直到 v0.1 的最后。负责人可以改判：那样实施者在合并之前照清单跑一遍，README 照 4.2 去掉标记、写上核对过的版本。
11. **反向代理**（4.16 的"反向代理"一条）：`README.md:347` 过时了（今天是 `:365-367`）；"`client_max_body_size` 不小于 `/mcp` 的上限与……中较大的"改为按路由：`/mcp` 不小于 37,814,272 字节、两个正文路由不小于 31,522,816 字节，都缓冲请求体；上传与导入不缓冲、不小于导入包的上限加 64 KiB（导入的路由收 zip 与表单，`transfer/adapter/http/handler.go:80`）；其余路由 nginx 的 1 MB 等于默认的 `server.max_body_bytes`（改了它时一起改）。一个 `location /` 同时容不下"上传不缓冲"与"`/mcp` 缓冲"（第 5 节）。
12. **客户端显示 429 的样子**（4.4 `:122` 的"（Claude Code 显示"HTTP 429 …"）"）：没有核对过（P1 的 TypeScript 客户端的试用只看了 401、403、`/register`），改为"（客户端显示什么由人工清单的第 E 部分记下）"；README 不照抄它（4.2）。
13. **接入文档一条的两处**（4.16 的"接入文档"，`:384`）：`headers` 里的"`${NERVEWIKI_TOKEN}`"改为"每个客户端一个变量（如 `${NERVEWIKI_CLAUDE_TOKEN}`）：同一个变量就是同一个 PAT，历史里分不清是哪个 agent"（4.3）；Caddy 的整个分句"Caddy 默认不缓冲请求体、直接暴露也不，用它们时剩下的风险照旧（Caddy 的 `request_buffers` 能不能为一个路由缓冲，P5 核对过才写）"（在 `7dc4a686` 恰好出现一次）改为"Caddy 默认不缓冲请求体，README 的部署一节的样例以 `request_buffers` 只为这三个路由缓冲（P5 在 Caddy 2.10.2 上核对过；经 Caddy 时服务端早答的拒绝对几 MiB 的请求体常成 502，P5 5.2）；直接暴露时剩下的风险照旧"，与 README 的 Caddy 样例和 2.1 第 9 条对第 10 节的改法一致（第 5 节）。只换括号里的一句会让这一条前半说风险照旧、后半说样例去掉了它。

### 2.2 需要负责人确认

无。负责人在第 11 节的决定都照办：MCP 不开会话、不开任务、不恢复、不撤销（决定 1；3.8 的豁免与 PG7、PG8 的例外都由它来）；人工清单在 v0.1 的最后由负责人跑，M9 在它通过之后才算完成（取舍第 12 条；P5 写清单、不跑，README 里客户端的每一句标出来处，4.2）。不先跑一遍清单是主持者照那条规则定的，写在 2.1 第 10 条，负责人可以改判。2.1 的各条是写细或照代码改正。主持者在第一轮审查之后维持四处取舍：MC3 的活动挪到 PG14 而不另写一遍（2.1 第 5 条）、C7 以事件流的帧而不以浏览器（第 6 条）、PG7 与 PG8 以 `expectVersions` 代会话的两个断言（第 3 条）、反向代理的样例改成按路由（第 5 节）。

### 2.3 前面的 Phase 交给 P5 的

清单的步按来源列在这里（第 6 节不再按步另列一遍）。

| 来源 | 项 | 落实 |
|---|---|---|
| 00 号文档第 7 节 P5 一行 | 第 9 节表里尚未做的 MCP 版本 | 第 3 节：21 个（C4 随 P2 做完） |
| 同上 | README 的接入与反向代理 | 第 4、5 节 |
| 同上 | 人工清单 | 第 6 节 |
| 同上 | 总体设计随之的修订 | 第 7 节 |
| 00 号文档 4.16"人工的" | 隔离 | 清单的"不碰你自己的客户端配置"与"做完"（Codex 的登录按三种的次序，`auth.json` 只在指纹允许时复制回去） |
| 同上 | 两种入口连上、收到说明（"复述 `AGENTS` 的第一行"）；样例出自清单 | A1、A2、A5、B1–B3；A3 另核对项目的范围的命令不把 PAT 写进 `.mcp.json`（4.3） |
| 同上 | 读、写、`edit`、`batch`、`grep`；Codex 看得到 `read` 的正文（4.7 的形状） | C1–C6、C11、C14；C1 的 Codex 一栏 |
| 同上 | 重启之后的下一次调用 | G1、G2（交互的会话，6.2） |
| 同上 | 撤销 PAT 之后的提示；被锁的页的提示 | E3；H1–H3 |
| 同上 | `readOnlyHint` 的工具在各审批选项下（Claude Code 的许可模式，Codex 的四个取值） | D1–D8：每类注解一两个工具（D 的开头），Claude Code 只试默认的模式（6.2） |
| 同上 | 48 KiB 的 `read` 放得下（4.9 的"人工清单核对"）；读图片看得到图 | C7；C8 |
| 同上 | `edit` 与 Claude Code 的 Edit 不同的两处（弯引号、空的 `new_string`） | C3、C4 |
| 同上 | 没有令牌时两种客户端各显示什么 | E1、E2 |
| 同上 | 20 张以上的图片、3 MiB 的内联图片 | C10、C9 |
| 00 号文档 4.16"接入文档" | 一个 agent 一个 PAT、两种地址、`claude mcp add` 与 `.mcp.json`（环境变量的展开、绝不写 PAT）、Codex 的 `config.toml`、审批选项不一律 `approve`、`server.public_url`、上限、401/403/404/429、没有令牌时的那一句、名额与 JSON 值、按路由缓冲、Caddy、`base_version`、锁、历史只读 | 4.1 的各段；变量每个客户端一个（2.1 第 13 条）；`server.public_url` 在"MCP"一节已有（`README.md:218`），接入一节的"先准备"与上限表的 `read` 一行另提它；Caddy 的样例在 5.1 |
| 00 号文档 4.16"反向代理" | `/mcp` 不是 SSE、`X-Accel-Buffering` 只关事件流、`client_max_body_size`、读超时 45 秒、`Host` | 5.1；`client_max_body_size` 改为按路由（2.1 第 11 条） |
| [M0/P1 的移交](handoffs/M0-P1-mcp-sessions.md)第 1、3 项 | 真实客户端的重启之后的调用；审批选项 | G1、G2 与 D1–D8。第 1 项的后一半（"会话因 `SessionTimeout` 被回收后"）不会发生：服务端无状态、没有会话（00 号文档 4.3）。这一行与那份移交留到清单跑过才关（第 11 节） |
| [P1](01-P1-platform-reads.md) 第 8 节 | 接入文档、反向代理（不是 SSE、约 36 MiB、45 秒）、人工清单；第 9 节的 TypeScript 客户端的试用供"401、403、404、429 各说明什么" | 4.1 的"HTTP 的答复"一表（文字出自代码，4.3）、5.1；429 的显示等清单的 E6（2.1 第 12 条） |
| P1 第 8 节 | 清单：没有令牌时两个客户端显示什么（`/register`、Codex 退到 `/authorize`、`/token`）；20 张以上的图片；3 MiB 的内联图片与约 25,000 个 token | E1、E2、C10、C9 |
| P1 第 8 节 | 没有令牌时的那一句，照清单核对、改写 | 4.1 的"没有配置令牌时"照 P1 的写法（"例如 Claude Code"换成试用实际跑过的 SDK 的客户端）；4.2 的一行写明清单之后怎样改 |
| [P2](02-P2-writes-history.md) 第 8 节 | 清单：503 的表现、并行的大的写与限速的上行、`edit` 的唯一与引号、`move` 的 `to`、破坏性的工具的审批 | F1–F3（在账户入口；F1 的占位是一个循环；F2 的大小、限速的做法与按日志分的两种预期，6.2）、C3、C5、C6、D1、D4–D7 |
| P2 第 8 节 | 接入文档：名额、按路由缓冲（三个路由）、上传与导入不缓冲、Caddy（`request_buffers` 核对过才写）、`base_version`、锁不能接管、历史只读 | 4.1 的"大的请求体的名额""写的规矩"；5.1（Caddy 的样例 P5 核对过，5.2） |
| P2 第 8 节 | 其余故事的 MCP 版本照 P2 的断言（`expectRelocated` 带目的地、`expectChangesetOf`、由答复建出的 `created`） | 3.1；PG1 照 `expectNewPage` 的那种适配，PG2、PG3、PG4、L1、L3、AS2、AS3 加 `expectChangesetOf`，PG3 的目的地由测试给出 |
| P2 的设计审查第 16 条 | 说明里 503 的一行可选 | P2 已加（"Calls over 64 KiB are taken one at a time per token…"，`page/adapter/mcp/instructions.go:73-74`），P5 不再加 |
| [P3](03-P3-batch-revert.md) 第 8 节 | 接入文档写 `batch`、与分开调用的不同（那一条规则）与说明的别名 | README 的"MCP"一节的"batch"已写全（P3 写的：那一条规则、"batch 改的别名都看不到"）；接入一节的最后一条指过去 |
| P3 第 8 节 | 清单在真实的客户端里试 `batch`（破坏性、非幂等的审批提示） | C14、D1、D4–D7 |
| P3 第 8 节 | P4 之后说明离 4 KiB 只剩约 50 字节，P5 加字的先缩短 | P5 不加字（下面一行） |
| [P4](04-P4-grep-search.md) 第 8 节 | 接入文档写 `grep` 与 `search` 的分工、`incomplete` 与 `partial_lines`、`building`、一个汉字只找标题、被截掉词的页总被走查 | README 的"MCP"一节的"找"已写全（P4 写的）；接入一节的最后一条指过去 |
| P4 第 8 节 | 清单：`grep` 的 48 KiB 与 `partial_lines` 是否完整交给模型、`read(offset, column)` 读截取的行与部分匹配的行、`readOnlyHint` 的四个工具不要确认 | C11、C12、C15（截取的行，`命中.md` 的第 601 行，6.2）、D1、D4（`grep` 代表读的工具，D 的开头） |
| P4 第 8 节 | MCP 版本：L4 的标签由 `index`、L6 由 `backlinks`、SE1 由 `search`，照 MC6、MC7 的比较 REST | 3.4 的 L4、L6，3.7 的 SE1 |
| P3、P4 第 8 节 | 说明剩 69 字节，加字的先缩短 | 不加：接入一节写的都是给接上它的人的，agent 要的规则已在说明与工具的描述里；`TestTheBuiltInInstructionsAreShort`（`page/adapter/mcp/instructions_test.go:15-27`）照旧，两个入口仍是 4,027 与 3,957 字节。M10 加说明时的余量写进 M10 的移交（第 11 节） |
| 00 号文档"写给后面的 M" | M12 打磨：网页显示 MCP 地址、`clientInfo.title`；部署文档沿用 4.16 的反向代理 | 不在 P5（M12）；部署一节的改正就是 4.16 的那一条（第 5 节），M12 照它；P5 的文档提交写进 M12 的移交（第 11 节） |

## 3. 前面 M 的故事的 MCP 版本

00 号文档第 9 节的表里 22 个故事有 MCP 版本：C4 随 P2 做完（`e2e/stories/collab/c4-guard.spec.ts:81-146` 的"C4 (MCP)"），其余 21 个由 P5 做；27 个豁免（表里 14 行），3.8 逐行对着代码核对。

### 3.1 共同的做法

- **放在哪里**：每个 MCP 版本是它的故事自己的文件里的一个测试，名为"`<故事> (MCP): …`"，同 C4 (MCP)：故事文件里的本地帮助函数（PG3 的 `chain`、PG4 的 `expire`、PG9 的 `bom` 与 `acute`、L4 的 `expectTagged`、L6 的 `expectLinked`、AS3 的 `upload`、SE1 的 `expectFound` 与 `finds`）原样共用，不导出（knip 只数导出了而没人用的，`e2e/fixtures/mcp.ts:12-14`）。唯一的例外是 PG14：MC3 的"MC3 (activity)"（`e2e/stories/mcp/mc3-write-edit.spec.ts:170-201`）挪进 PG14 的文件，成为 PG14 (MCP)（3.2 的 PG14）。
- **准备照 API 版本**：种下的页、会话、上传、成员都照 API 版本经 REST 做（MC2、MC4、C4 (MCP) 的做法）；只有故事的动作经 MCP。会话与上传本来就只有 REST（3.8）。agent 要的版本令牌由它自己的 `read` 或 `list` 读到。
- **一个 PAT 一个客户端名称**（00 号文档第 9 节、4.5）：每个 agent 是 `mcpClient(nervewiki.baseURL, <它自己的 PAT>, {notebook})`，各用自己的 PAT；测试要分开两个 agent 的变更集时，另给各自的名称（`client: "a-agent"` 一类，同 `c4-guard.spec.ts:92-93`，L3），断言里的客户端是 `mcp:<名称>`；不分开时都用默认的 `e2e-agent`（PG2、PG12、SE2）。
- **期望的值来自 MCP 的答复**（00 号文档第 9 节"断言的适配"）：`node_id` 是 id，`version`（`"<node id>:<revision>"`，`page/domain/version_token.go:21`）给版本号，`at` 是变更集的时刻，即正文的 `content_updated_at` 与新建的 `created_at`（一个操作只读一次时钟：`page/adapter/mcp/tools.go:148-153`、`page/app/unit.go:250`、`:556-559`；MC3 已这样用，`mc3-write-edit.spec.ts:79`）；目的地与标题由测试给出（MC4 的做法，`mc4-move-delete.spec.ts:63`）。从不从库里读出期望的值再与库比。
- **同一组断言**：每个 MCP 版本调它的 API 版本调的每一个数据库断言函数（`e2e/fixtures/assert/*.ts`），值换成由答复造出的；API 版本不调断言函数的（PG11、PG12、PG14、C7、SE1、SE2，只有 REST 的答复、事件的帧、内联的 SQL 或故事自己的帮助函数），MCP 版本做同样的检查，值来自答复。例外只有两个、都属编辑会话的豁免（PG10 一类）：PG7 的 `expectOneSessionRevision`、`expectSessionGone` 与 PG8 的 `expectOneSessionRevision` 都收一个编辑会话的 id，MCP 不开会话（`page/adapter/mcp/write.go:14-18`、`edit.go:13-19` 的参数里没有会话），它们的 MCP 版本以 `expectVersions`（`e2e/fixtures/assert/page.ts:509-522`）代之：每个 MCP 的写是 agent 的客户端的一个变更集里的一个版本，`expectVersions` 逐行比全部的版本（基、版本号、正文与哈希、种类、作者、客户端、每个变更集的版本数），写被错合成一个变更集时少一行、失败，正是 `expectOneSessionRevision` 断言的"一个变更集一个版本"在 MCP 下的样子；PG7 另断言 agent 的写之后这一页没有编辑会话（`sessionsOf` 为空，`page.ts:474-480`），是 `expectSessionGone` 在 MCP 下的对应（2.1 第 3 条）。
- **REST 的读照旧**：API 版本里的 REST 的读（`readContent` 的哈希、`getView`、`getPageProperties`、`listNodes`、`getPage`）在 MCP 版本里照旧：它们读的是同一个结果，不是故事的动作。各故事的"不同"一条列出它丢掉的检查。
- **加给 fixture 的**（都在 `e2e/fixtures/` 下；第 8 节第 1、2 步）：
  1. `expectContentWritten(db, written, content, writerId)`（`assert/page.ts:429-445`）的 `written` 由 `Page` 放宽到 `Pick<Page, "id" | "revision" | "content_updated_at">`：它只读这三个（`:440`、`:443`）；`Embedded.written`（`assert/asset.ts:162-166`）同样放宽，`expectEmbedded` 把它原样交给 `expectContentWritten`。P2 这样放宽过 `expectNewPage`（`page.ts:16-22`）。今天的调用者（PG5、PG7、PG8、PG9、PG10、C10、AS2）都传整个 `Page`，照样编过。
  2. `expectMoved(db, moved, from, moverId)`（`page.ts:135-149`）的 `moved` 由 `TreeNode` 放宽到 `Pick<TreeNode, "id" | "parent_id">`：它只读这两个（`:144`、`:147`）。
  3. `fixtures/mcp.ts` 加 `writtenOf(answer, what)`：由 `write` 或 `edit` 的答复（结构上的 `{node_id, version, at}`，不导出类型）造出上面的 `Pick`；`version` 不是 `` `${node_id}:` `` 加数字、或 `at` 为 null（没有写）时失败，不悄悄地给 `NaN` 或 `"null"`。四个故事导入它（PG7、PG8、PG9、AS2），过 knip。它自己的检查由一次伪造的答复证明会失败（第 8 节第 8 步，只记进审查记录）。
  4. MC2 的本地 `readPage` 与 `PageRead`（`mc2-read-list.spec.ts:64-83`：`read` 两块文字、没有 `structuredContent`，答 `{meta, text}`）挪进 `fixtures/mcp.ts`，**两个都导出**：MC2 的 `readAll` 在自己的签名里写 `PageRead`（`mc2:86`），只挪函数而类型不导出时 MC2 编不过（`TS2304`）；MC2 导入两者，正是 `mcp.ts:12-14` 的规则许可导出的时候（故事导入了才导出）。PG7、PG8、PG9、PG11、PG12、HI1、L1、AS2、AS3 用 `readPage`。C4 (MCP) 手写的一处（`c4-guard.spec.ts:95-96`）不动。建成的 `PageRead.meta` 写出 `read` 一页的第一块的十个成员（`path`、`kind`、`version`、`offset`、`column`、`lines`、`total_lines`、`byte_size`、`next_offset`、`next_column`，`page/adapter/mcp/read.go:28-37`），不再是 MC2 的四个加一个索引签名：新的故事读它的 `version` 与 `path`（3.9）。

  00 号文档第 9 节说的 `collab.ts:50` 的客户端类型放宽**不做**：那是 `expectToggleRevision` 的参数（`e2e/fixtures/assert/collab.ts:44-50`），只有 C10 调它，C10 豁免（2.1 第 4 条）。
- **两个已有的测试的同一个洞一起补**：L6 (API) 与 C7 (API) 各自的事件流的检查有 3.3、3.4 说的洞（L6 (API) 开流之前不 settle，C7 (API) 删除之后不查流是否安静），各一两行的改法，与它们的 MCP 版本在同一个提交里补上。
- **循环**：PG11 的按文件夹走、L4 与 L6 的翻页里的 `await` 在循环里，照 MC2 的写法加 `// oxlint-disable-next-line no-await-in-loop`（`mc2-read-list.spec.ts:92`；oxlint 零警告）。
- **不要浏览器**：21 个 MCP 版本都只经 Node（`mcpClient`、`openEvents`、REST 的 fixture 与库），没有一个开页面（C7 的取舍在 3.3）。每个约 0.3–1 秒（本机），请求都是一页、一次调用的大小；e2e 的服务端的请求期限是默认的 15 秒（`e2e/fixtures/server.ts:139-152` 与 `server/configs/config.test.yaml` 都不设 `request_timeout`，`server/configs/config.yaml:17`），4 秒的期限是 Go 的整个程序测试的（9.4）。

### 3.2 页面的故事

**PG1 新建页面**（`e2e/stories/page/pg1-create-page.spec.ts`，API 版本 `:25-79`）

- 断言：API 版本只调 `expectNewPage`（`:49`、`:52`），MCP 版本调它两次，`created` 由答复与测试造：`{id: node_id, notebook_id, parent_id: null 或 Roadmap 的 node_id（前一个答复的）, name: 测试给的标题 "Roadmap"、"Q4", created_at: String(at)}`，客户端 `"mcp:e2e-agent"`（同 `mc3-write-edit.spec.ts:70-86`；`created[].path` 是从根起的整个路径，`mc3:62-66`，名称照字面写，不由路径拼）。不加 fixture。
- 步骤（`PG1 (MCP): an editor's agent writes a page at the root and one under it, each new in a changeset of its client; a reader's agent cannot; a path segment a file could not take, or with blanks at its ends, is refused on path`）：
  1. 准备照 API 版本（`:29-36`：编辑者经成员、读者经笔记本的 `viewer`）；编辑者与读者各一个 agent。
  2. `write {path: "Roadmap.md", content: ""}` 答 `[path, created 的路径, created_count, version, changed]` = `["Roadmap.md", ["Roadmap.md"], 1, "<id>:1", true]`；`expectNewPage`。e2e 里第一次以空的正文 `write`（`content` 没有下限，`api/mcp/tools.json` 的 `write`）。
  3. `write {path: "Roadmap/Q4.md", content: ""}` 只建 Q4（`created` 是 `["Roadmap/Q4.md"]`）；`expectNewPage`，父页是第 2 步的 `node_id`。
  4. 读者的 agent `write {path: "Reader's.md", content: ""}` 答 `forbidden`（`page/app/authorize.go:41`）。
  5. 四个路径各被拒，`[code, fields 的 [field, code, message]]` 逐个 `toEqual`（文字分得开两个 `invalid_format`）：`"Roadmap/  .md"` 是 `path` 的 `required`、`"Roadmap/a:b.md"` 是 `invalid_format`、`"Roadmap/con.txt.md"` 是 `not_allowed`、`"  Roadmap  .md"` 是 `invalid_format`（"segment 1 must not start or end with white space"，`page/domain/path.go:70-77`）。
  6. 库里这本笔记本恰好两个节点（API 版本的计数，`:75-78`），在全部的拒绝之后读。
- 与 API 版本的不同：MCP **不去掉两端的空白**（REST 的标题去掉，`shared/title.go:29`；MCP 的路径段照字节检查），所以 API 版本的 `"  Roadmap  "` 在这里是一个拒绝；`"Plans/2026"` 不是拒绝（`/` 是段的分隔，它建两页），`invalid_format` 换成 `:`；"父页不是页"没有 MCP 的形式（路径缺的父页由 `write` 补建，MC3 断言过）。

**PG2 改名**（`pg2-rename-page.spec.ts`，API 版本 `:24-63`）

- 断言：`expectRenamed` 三次（`page.ts:66-87`，收 id 与名称），id 取 `move` 的答复的 `node_id`（并断言它等于种下的 id）；另加一次 `expectChangesetOf(db, changeset_id, {client: "mcp:e2e-agent", byId: editorId, items: [notes.id], versions: []})`：`expectRenamed` 不看客户端，这是改名是 agent 的写的唯一证据（MC4 的做法）。不加 fixture；NFD 的标题用 API 版本的字面（`pg2:49`），提到文件的顶层，两个测试共用。
- 步骤（`PG2 (MCP): an editor's agent renames a page by its path; a title a sibling holds by its key is refused on to, in any case or normalization, as is a write there of other bytes; a change of case alone is written`）：
  1. 准备照 API 版本（`:28-36`：Notes、Straße、Café）；编辑者与管理员各一个 agent（各自的 PAT）。
  2. 编辑者 `move {from: "Notes.md", to: "Journal.md"}` 答 `{path: "Journal.md", node_id: notes.id, changed: true}`；`expectRenamed(db, node_id, "Notes", "Journal", editorId)` 与 `expectChangesetOf`。同一个父页下是改名、留在原处（`page/app/unit_relocate.go:25-29`、`:48-49`），`expectRenamed` 的"次序不变"成立。
  3. 管理员 `move {from: "Journal.md", to: "a:b.md"}` 答 `validation_failed`，字段 `to` 的 `invalid_format`。
  4. 管理员把 Journal 改成 `"STRASSE.md"`、`"strasse.md"`、NFD 的 Café 加 `".md"`、`"CAFÉ.md"`，`Promise.all`，每个 `page.title_taken`；映射成 `[to, code]` 整个 `toEqual`。
  5. 管理员 `write {path: "STRASSE.md", content: "Other\n"}`（没有 `base_version`）答 `page.title_taken`（"… is a page already"，`page/adapter/mcp/write.go:73-74`）：API 版本的同名的新建（`pg2:54-55`）在 MCP 下的形式。MCP 按键找页，`STRASSE.md` 就是 Straße 的路径；正文必须与 Straße 的（空）不同，否则它是答未改动的成功、不是拒绝。
  6. 管理员 `move {from: "Straße.md", to: "STRASSE.md"}` 写下（只改大小写）；`expectRenamed(db, street.id, "Straße", "STRASSE", adminId)`；`expectRenamed(db, notes.id, "Notes", "Journal", editorId)` 再一次：第 3–5 步什么都没写。
- 不同：拒绝落在 `to` 而不是 `name`；不去掉空白（`" Journal "` 照 API 版本会被拒，所以写 `"Journal.md"`）。

**PG3 移动**（`pg3-move-page.spec.ts`，API 版本 `:32-85`）

- 断言：`placeOf`（移动之前读的"从"，`page.ts:119-128`）与 `expectMoved`。API 版本调 `expectMoved` 四次，MCP 版本三次：第一次（C 排到根的最前）是兄弟之间的排序，MCP 的 `move` 没有位置（`tools.json` 的 `move` 只有 `from`、`to`、`update_links`；同一父页下是留在原处的改名，`unit_relocate.go:25-29`；换父页时排在新的兄弟的最后，`:70`），这一次没有 MCP 的形式。`moved` 是 `{id: node_id, parent_id: 测试给的目的地（a.id、null、levels[6].id）}`，**不从移动之后的库读**；要 3.1 的第 2 条放宽。另加 `expectChangesetOf`：`expectMoved` 不看客户端（`page.ts:135-149`）。移动的 `at` 归 MC4 查（`mc4-move-delete.spec.ts:63`），这里不再查。
- 步骤（`PG3 (MCP): an editor's agent moves a page under another and back to the root, each time last among its new siblings; a move under itself is page.cycle, one past ten levels page.too_deep`，标题不说排序）：
  1. 准备照 API 版本（A > A1 > A2、B、C；后面的 `chain(8)` 照旧经 REST）。
  2. `placeOf(b)`；`move {from: "B.md", to: "A/B.md"}` 答 `{path: "A/B.md", node_id: b.id, changed: true}`；`expectMoved`；`expectChangesetOf(db, changeset_id, {client: "mcp:e2e-agent", byId: editorId, items: [b.id], versions: []})`；REST `getPage(b).ancestors` 是 `[A]`，A 的子页（REST `listNodes`）是 `["A1", "B"]`：在最后。
  3. `placeOf(b)`；`move {from: "A/B.md", to: "B.md"}`；`expectMoved`；根下的次序（REST `listNodes`，API 版本的读）是 `["A", "C", "B"]`：**最后**，不是 API 版本的最前，这正是 MCP 的规则，要断言。
  4. `move {from: "A.md", to: "A/A.md"}` 与 `move {from: "A.md", to: "A/A1/A2/A.md"}`，各 `page.cycle`（`page/domain/steps.go:143-146`），映射成 `[code]` 整个 `toEqual`；之后 REST `getPage(a).parent_id` 为 null。
  5. `chain(8)`；`move` A 到 Level 1 之下（9 段，`MaxPathSegments` 11 之内，`domain/path.go:14-17`）答 `page.too_deep`（`steps.go:151-153`）；`getPage(a).parent_id` 仍为 null。
  6. `placeOf(a)`；`move` A 到 Level 2 之下；`getPage(a2).ancestors.length` 是 9；`expectMoved`。
- 不同：没有排序的三步；两次换父页都落在最后（第 2、3 步各断言一次）。

**PG4 删除子树**（`pg4-delete-subtree.spec.ts`，API 版本 `:25-66`）

- 断言：`expectSubtreeDeleted(db, rootId, ids, deleterId)`（`page.ts:287-311`，收 id），`rootId` 取答复的 `node_id`，答复的 `pages` 等于 `ids.length`（3）。另加三个，API 版本没有：
  - `expectChangesetOf(db, changeset_id, {client: "mcp:e2e-agent", byId: editorId, items: [doomed, child, grandchild], versions: []})`：`expectSubtreeDeleted` 不看客户端，e2e 里还没有一个测试查 MCP 的删除的客户端（MC4 也不查，`mc4:123-129`）。
  - `expectDeletionRevisions(db, changeset_id, [doomed, child, grandchild])`（`page.ts:567`）：网页撤销一次删除靠"每个条目带着它的页的版本号"，今天只有 HI4 断言它（REST 与网页的删除、撤销的再删除之后，`hi4-revert-delete.spec.ts:96`、`:111`、`:158`），MCP 的删除走同一个 `Unit.DeleteSubtree`（`page/app/delete_path.go:86`），一行就钉住（3.8 的 HI 一行）。
  - `expectTrashRoot(db, doomed.id, {pages: 3, assets: 0, height: 3}, {alone: true})`（`page.ts:327-363`）：TB1、TB2 的断言，带 `alone`（`tb1-trash.spec.ts:58`、`:146-147`、`:252`，`tb2-background-restore.spec.ts:131`、`:196`）。MC4 删进回收站时不带 `alone`（`mc4:129`）；有了这一行，"MCP 的删除进回收站"由 PG4 (MCP) 以 TB 自己的断言覆盖（3.8 的 TB 一行）。

  不加 fixture（`expire` 是这个文件的本地函数，`:18-23`）。
- 步骤（`PG4 (MCP): an editor's agent is refused a page with subpages without recursive, saying how many; with recursive, refused while any of their pages is being edited, the editor's own too; once those sessions expire, it deletes the subtree at one time, the sessions with it, into the trash as one item; no one reads them after, and its sibling stays, its session too`）：
  1. 准备照 API 版本（`:29-41`：Doomed > Child > Grandchild、Sibling；管理员在 Child 与 Sibling 上、编辑者自己在 Grandchild 上各一个会话）。
  2. `delete {path: "Doomed.md"}`（没有 `recursive`）答 `validation_failed`，整个文字（`errorOf` 的 `message`，也就是答复的文字块，`e2e/fixtures/mcp.ts:171-176`）连同码的前缀是 `validation_failed: "Doomed.md" has 2 pages under it: set recursive to true to put them in the trash with it.`、`fields` 映射成 `[[field, code]]` 是 `[["recursive", "required"]]`（文字由 `page/adapter/mcp/text.go:35-38` 造、带着码，mcpserver 把它原样作 `message`，`platform/httpserver/mcpserver/errors.go:155-156`、`:188-190`；子页的检查在锁之前，`delete_path.go:42-50`）。第 3 节别的对文字的期望都是"含"某几个字（PG2、L3、AS4）或字段的说明（PG1），不是整个文字。
  3. `delete {path: "Doomed.md", recursive: true}` 答 `page.locked`，`locks` 恰是两项、**按子树的次序**（一层一层，`page/app/edit_lock.go:87-113` 的 `lockedAll`；不是按页的 id，那是 linking 的 `pages_locked` 的，`lock_holders.go:49`），期望照字面写 Child 在前、Grandchild 在后：Child 的 `{page_id, path: "Doomed/Child.md", user_id: adminId, display_name, you: false}` 与 Grandchild 的 `you: true`，`locks_count: 2`，`lock` 是第一项去掉路径（`page/adapter/mcp/locks.go:14-32`）。**这是与 API 版本的不同**：agent 的删除连调用者自己的编辑器也拒（`page/app/edit_lock.go:46-55`，"agent 不结束编辑"，00 号文档 4.10），API 版本里编辑者自己的会话随删除一起结束。
  4. 删除之前两页的会话各一行（`sessionsOf`，`page.ts:474-480`），之后 `expire` Child 的与 Grandchild 的两个会话；`delete {path: "Doomed.md", recursive: true}` 答 `{node_id: doomed.id, pages: 3, attachments: 0}`；`expectSubtreeDeleted`（含"没有编辑会话留下"：过期的两行随子树删掉，`page/app/unit_delete.go:56-60`）、`expectChangesetOf`、`expectDeletionRevisions` 与 `expectTrashRoot`。**过期的会话不再拒绝 agent 的删除**由此第一次在 e2e 里断言（C4 (MCP) 只有结束了的会话，`c4-guard.spec.ts:143-145`；Go 的假的存储的测试有这条规则，`page/app/edit_lock_test.go:105`；3.8 的 C6）。
  5. REST `heartbeat(siblings)` 200；三个路径的 `read` 各答 `page.not_found`；`list {}` 答 `["Sibling.md"]`。
  6. 再 `delete {path: "Doomed.md", recursive: true}` 答 `page.not_found`。
- 不同：API 版本的"自己的活着的会话随删除结束"在 MCP 下是拒绝（第 3 步）；"在被删的 Child 下新建"没有 MCP 的形式（`write "Doomed/Child/Late.md"` 会重新建出 Doomed 与 Child：被删的节点不在路径上）。

**PG7 编辑与保存**（`pg7-edit-save.spec.ts`，API 版本 `:27-66`）

- 断言：`expectContentWritten(db, writtenOf(edited), content, editorId)`（要 3.1 的第 1、3 条）；`expectOneSessionRevision` 与 `expectSessionGone` 收会话的 id，换成 `expectVersions(db, page.id, [{base: null, revision: 1, content: "", kind: "edit", byId: adminId, client: "api"}, {base: 1, revision: 2, content: "# Notes\n", …, byId: editorId, client: "mcp:e2e-agent"}, {base: 2, revision: 3, content, …, client: "mcp:e2e-agent"}])`（3.1；MC3 这样用，`mc3-write-edit.spec.ts:155-167`；REST 的建是 `kind: "edit"`、客户端 `api`，同 `expectNewPage` 的期望，`page.ts:57-59`）与 `expect(await sessionsOf(db, page.id)).toEqual([])`：agent 的写不开、不留编辑会话（一个开了或留下会话的 MCP 的写会挡住网页，今天没有测试抓得到：C4 (MCP) 只查经 REST 开的会话还活着）。
- 步骤（`PG7 (MCP): an editor's agent writes a page on its version and edits it on the next: a changeset of its client and a version each, and no edit session; the revision, hash and size are the content's, which read gives back and the reading view shows`）：
  1. 准备照 API 版本（`:31-36`，Notes 空）；agent `readPage` 得 `"<id>:1"`。
  2. `write {path: "Notes.md", content: "# Notes\n", base_version: "<id>:1"}` 答 `version "<id>:2"`。
  3. `edit {path: "Notes.md", old_string: "# Notes\n", new_string: "# Notes\n\nSecond save.\n", base_version: 第 2 步的 version}` 答 `{version: "<id>:3", replacements: 1, changed: true}`（`edit` 不能从空的正文开始：`old_string` 至少 1 字节，`edit.go:15`，所以先 `write`）。
  4. `expectContentWritten`；`expectVersions`；`sessionsOf` 为空。
  5. `readPage {path: "Notes.md"}`：`meta.version` 是 `"<id>:3"`、`byte_size` 是正文的字节数、`text` 逐字节相等；REST `readContent` 的 `content_hash` 是 SHA-256、`getView` 的 `<p>Second save.</p>`（API 版本的读，`:55-62`）。
- 不同：没有会话，两次写各是一个变更集；会话的开与结束没有 MCP 的形式。

**PG8 保存冲突**（`pg8-save-conflict.spec.ts`，API 版本 `:39-77`）

- 断言：`expectContentWritten(db, writtenOf(kept), "Mine\n", editorId)`；`expectOneSessionRevision` 换成 `expectVersions`（三行：管理员 REST 的建与 "Theirs\n"、agent 的 "Mine\n"），逐行比全部的版本，所以两次被拒的写什么都没写下（同 API 版本的 `expectOneSessionRevision(…, 2, 3)` 证明的）。
- 步骤（`PG8 (MCP): another credential writes first, so an agent's write and edit on the version it read are page.revision_mismatch, saying the version now; it reads the current content and writes its own on that version, in a changeset of its client`）：
  1. 准备照 API 版本（`:43-48`）；编辑者的 agent `readPage {path: "Notes.md"}` 得 `version "<id>:1"`（空的页照样是两块文字：SDK 总写出 `text`）。
  2. 管理员 REST `writeContent("Theirs\n", base 1)`（另一个凭证先写）。
  3. `write {path: "Notes.md", content: "Mine\n", base_version: "<id>:1"}` 与 `edit {path: "Notes.md", old_string: "Theirs", new_string: "Mine", base_version: "<id>:1"}`：各 `[code, current_version]` = `["page.revision_mismatch", "<id>:2"]`（`page/adapter/mcp/text.go:32-34`、`:82-94`；`edit` 先查令牌再替换，`page/app/edit_page.go:141-148`；`old_string` 取当前正文里有的 `Theirs`，被拒的是版本，不是"找不到"）。过时的 `edit` 是 e2e 里第一次（MC3 只有过时的 `write`，`mc3:108-113`）。
  4. `readPage`：`meta.version` 等于错误的 `current_version`，`text` 是 `"Theirs\n"`（`current_version` 不带正文，agent 照样要读）。
  5. `write {path: "Notes.md", content: "Mine\n", base_version: 读到的 version}` 答 `"<id>:3"`；`expectContentWritten`；`expectVersions`。
- 不同：`page.edit_session_ended` 是会话的码，没有 MCP 的形式（API 版本的第 2、4 步）；MCP 的错误带当前的版本（REST 的 409 只有码与文字，`platform/httpserver/problem.go:33-52`）。

**PG9 字节保真**（`pg9-byte-fidelity.spec.ts`，API 版本 `:19-45`）

- 断言：`expectContentWritten(db, writtenOf(w), content, adminId)`，每种正文一次；六种正文的数组从 API 版本里提到文件的顶层，两个测试共用（`bom`、`acute` 已在顶层，`:16-17`）。
- 步骤（`PG9 (MCP): CRLF, CR alone, mixed line breaks, a byte order mark, trailing blanks and NFD written by an agent come back from read byte for byte, its lines ended by LF alone, stored with the SHA-256 of those bytes`），六种正文 `Promise.all`：
  1. REST `createPage("Page i")`（API 版本的准备）；`write {path: "Page i.md", content, base_version: "<id>:1"}` 答 `changed: true`、`version "<id>:2"`。
  2. `expectContentWritten`。
  3. `readPage {path: "Page i.md"}`：`Buffer.from(text).equals(Buffer.from(content))`（失败的消息带 `JSON.stringify`，同 `:39`），`meta.byte_size` 等于 `Buffer.byteLength(content)`，`meta.version` 等于写的；六页的 `meta.total_lines` 依次是 `[4, 1, 3, 1, 3, 1]`（只有 `\n` 结束一行，`page/domain/read.go:61-69`：单独的 `\r` 留在行里，单独 CR 的正文是一行）；REST `readContent` 的 `content_hash` 是那些字节的 SHA-256（`read` 不给哈希）。
- 不同：没有。

**PG11 导航**（`pg11-navigation.spec.ts`，API 版本 `:19-46`）

- 断言：API 版本不调断言函数（不取 `db`）：`getPage(C).ancestors` 与 `listNodes` 的次序比字面的值。MCP 版本同样比字面的值，并与 REST 的同一个读比（MC6、MC7 的做法）。不加 fixture（一个本地的、由 `parent_id` 拼路径的小函数，不导出）。
- 步骤（`PG11 (MCP): an agent reads a page by its ancestors' path, in any case, and not by another; each folder lists its children in their order, the tree's order walked folder by folder`）：
  1. 准备照 API 版本（`:22-35`；First、Between 的位置要 REST 的 `after_id`，`write` 不能定位置）。
  2. `readPage {path: "a/b/c.md"}`：`meta.path` 是存下的名称拼成的 `"A/B/C.md"`（路径按键找，答复给存下的名称，`page/app/read_path.go:80`），`meta.version` 是 `"<c.id>:1"`；`meta.path` 等于 REST `getPage(c).ancestors` 的名称加 `"C.md"`。`read {path: "B/C.md"}`（祖先不对）答 `page.not_found`：读真的是沿着祖先找的。
  3. `list {}`、`list {pattern: "A/"}`、`list {pattern: "A/B/"}` 按 `has_children` 先序地走（循环有上界，同 `mc2-read-list.spec.ts:90-91`）：合起来是字面的 `["A.md", "A/First.md", "A/Between.md", "A/B.md", "A/B/C.md", "Last.md"]`，也等于 REST `listNodes` 由 `parent_id` 拼成的路径（文件夹的子节点按树的次序，`page/app/list_by_pattern.go:296-298`）。
- 不同：网页的面包屑、子页、快速切换只在页面版本里；`**/*.md` 是路径的次序（MC2 断言过），所以按文件夹走。

**PG12 页面权限**（`pg12-page-permissions.spec.ts`，API 版本 `:32-117`）

- 断言：API 版本不调断言函数，最后以内联的语句比 `nodes` 的行与字面的值（`:109-116`）；MCP 版本原样重复这一句。读者的六个 `forbidden` 是 API 版本的那六个（`pg12:59-69`）在 MCP 下的形式。不加 fixture（`postMcp`、`mcpClient`、`errorOf` 已有）。
- 步骤（`PG12 (MCP): a member's agent writes, renames, moves and deletes by the workspace access editor; a guest reader's reads and is refused each write; one who does not see the notebook is the notebook entry's 404 and the account entry's notebook.not_found`）：
  1. 准备照 API 版本（`:36-45`：成员、以笔记本的 `reader` 加入的访客、没有角色的访客；"Team" 的工作区访问 `editor`；Notes）。三个 PAT 各一个 agent。
  2. 成员：`write {path: "Notes/Mine.md", content: ""}` 答 `created` 一项；`move {from: "Notes.md", to: "Shared.md"}` 答 `{path: "Shared.md", node_id: page.id, changed: true}`（Mine 随它成了 `Shared/Mine.md`）；`readPage {path: "Shared.md"}` 得 `"<id>:1"`，`write` 带它写 `"# Shared\n"`，答 `"<id>:2"`。
  3. 读者：`readPage {path: "Shared.md"}` 得 `["# Shared\n", "<id>:2"]`；`list {}` 答 `["Shared.md"]`。六个写，每个都是写者会成功的调用，`base_version` 取读者自己读到的 `"<id>:2"`：`write {path: "Reader's.md", content: ""}`、`move {from: "Shared.md", to: "Reader's rename.md"}`、`delete {path: "Shared/Mine.md"}`、`write {path: "Shared.md", content: "# Reader\n", base_version}`、`edit {path: "Shared.md", old_string: "# Shared", new_string: "# Reader", base_version}`、`batch {message: "reader", ops: [{op: "write", path: "Batch.md", content: ""}]}`，各答 `forbidden`，映射成码与六个字面的 `"forbidden"` 比（每个工具里 `forbidden` 在别的检查之前，`server/internal/bootstrap/permission_matrix_mcp_writes_test.go:14-24` 的 `writers` 一行）。
  4. 看不到的访客：笔记本入口的 `postMcp(initialize)` 答 `[404, "application/problem+json", "no-store"]`、码 `notebook.not_found`（MC1 的形状，`mc1-connect.spec.ts:133-145`；`mcpClient` 在这里会抛出，`mcp.ts:99-101`，所以用 `postMcp`）；账户入口的 `notebooks` 答 `[]`，`list`、`read`、`write`、`move`、`delete`、`edit`、`batch` 带 `notebook: notebook.id` 各答 `notebook.not_found`（先判定笔记本：`page/app/authorize.go:28` 的 `decideNotebook`，所以从不是 `page.not_found`）。
  5. 成员：`move {from: "Shared/Mine.md", to: "Mine.md"}` 答 `{path: "Mine.md", node_id: Mine 的（第 2 步的答复）, changed: true}`、`delete {path: "Mine.md"}` 答 `[node_id, pages, attachments]` = `[Mine 的, 1, 0]`；内联的 `nodes` 的语句答 `[{Mine, deleted: true}, {Shared, deleted: false}]`（没有读者与访客写下的节点：行的集合是确切的）。
- 不同：MCP 按笔记本加路径找，看不到的笔记本里每个路径都是 `notebook.not_found`，从不是 `page.not_found`（总体设计 13.1 第 4 条，`v0.1-design.md:1039`）；会话的两步（开、心跳与结束别人的）没有 MCP 的形式。Go 的矩阵已有这三列的每一格（`permission_matrix_notebook_test.go:154-176`），PG12 (MCP) 加的是经真的二进制、PAT 与 SDK 的整条路与故事的形状。

**PG14 笔记本的活动**（`pg14-notebook-activity.spec.ts`，API 版本 `:16-44`）

- 今天：MC3 (activity)（`mc3-write-edit.spec.ts:170-201`）与 PG14 (API) 逐行是同样的检查（`ownerlessNotebooks` 的大小、`last_activity_at` 与最后一次写的时刻在 SQL 里相等），不同只在动作经 MCP、时刻取答复的 `at`；它就是 PG14 的 MCP 版本，只是放在 MC3 的文件里、名字是 MC3 的。00 号文档第 9 节的 PG14 一行写"MC3 里断言，P5 照 PG14 的形状另写一遍"。
- 取舍：**挪过去，不另写一个重复的**：MC3 (activity) 挪进 `pg14-notebook-activity.spec.ts`，改名为 PG14 (MCP)，照 PG14 的形状把 Notes 的第二次写改成 `edit`（PG14 是一次建、会话里两次保存；MCP 没有会话，它的"再保存一次"是 `edit`，带 `write` 给的版本）；MC3 去掉这个测试、只有它用的导入（`emailFor`、`joinAs`、`memberOf`、`removeMember`、`ownerlessNotebooks`、`newTeam`：MC3 的主测试用 `newOnboardedTeam`，`mc3:50-168` 不用它们）与文件开头注释的末句（"an ownerless notebook's activity of an agent's writes"，`mc3:18-19`）。00 号文档的 MC3 一行与 PG14 一行随之改（2.1 第 5 条）。另写一遍只多约 0.3 秒，但是同一组动作与检查的重复，审查会问它多证明了什么。
- 断言：同 API 版本的两条，值来自答复：大小是 Intro 与 `edit` 之后的 Notes 的字节（测试自己的字面的值，`edit` 答 `replacements: 1, changed: true`），最后的活动等于 `edit` 的 `at`。活动是这本笔记本的每个变更集的 `max(updated_at)`（`page/adapter/postgres/queries/activity.sql:5-10`），`edit` 与 `write` 同一种变更集，所以以 `edit` 收尾证明的是同一条规则；移出所有者不写变更集。不加 fixture：两个本地的接口 `Written` 与 `Edited`，后者带 `at`（MC3 的 `Edited` 没有，`mc3:34-39`）。
- 步骤（`PG14 (MCP): an ownerless notebook's size is the bytes of the pages an agent wrote, and its last activity the agent's last write, an edit`）：所有者经自己的 PAT 的 agent `write` Intro（带正文）与 Notes（`"# Notes\n"`），`edit` Notes 带 `write` 给的版本；管理员移出所有者（204）；两条检查。

### 3.3 协作：C7 推送

**C7 推送**（`e2e/stories/collab/c7-push.spec.ts`，API 版本 `:27-73`）

- 断言：API 版本不调断言函数，它的检查是 B 的事件流里的帧整个 `toEqual`，与 C 什么都收不到（C 的第一帧是它自己的写）。MCP 版本调**同样的帧的检查**，帧里的 id 与版本号来自 MCP 的答复，最后另查 B 的流是否安静（下面第 5 步）。不加 fixture（`openEvents` 是 `e2e/fixtures/test.ts:52`、`:197-207` 的测试 fixture）。
- 取舍：**以 `openEvents` 收帧，不开浏览器**。每个页面的写，REST 的与 MCP 的，都经同一个写的单元，单元在它的事务里告诉观察者一次（`page/app/unit.go:635-647` 的 `tell`），事件的观察者把单元的改动交给发布者（`bootstrap/events_registrants.go:23-32`），发布者建 `pages` 帧（`events/app/publisher.go:52-63`）；MCP 的 `write`、`move`、`delete`、`edit`、`batch` 都经这个单元（`page/app/write_path.go:128`、`:214`，`relocate_path.go:87`，`delete_path.go:72`，`edit_page.go:169`，`apply_batch.go:235`）。帧到了事件流，网页对同样的帧怎样反应由 C7 (page) 证明（`c7-push.spec.ts:87-114`，它先持住事件流、数到两次读，之后的更新只能经事件）。浏览器的 MCP 版本做不了 C7 (page) 的两步（排到最前、"正在编辑"），又要约 2–4 秒。Go 里已有一个 MCP 的写到事件流的测试：`TestThePagesEventListsNoPageInTheTrash`（`bootstrap/events_pages_test.go:141-161`）经笔记本入口发一个 `batch`、在 bob 的流上读它的帧；C7 (MCP) 是第一个逐个工具（`write`、`move`、`edit`、`delete`）看帧、并看一个 batch 两页一帧的测试。00 号文档的 C7 一行"页面一侧的断言照旧"改为这样（2.1 第 6 条）。（MC3 (page) 看到一次 MCP 的写更新打开的页，`mc3-write-edit.spec.ts:99-107`，但它不持住事件流，更新可能来自流连上时的重读，不算这里的证据。）
- 步骤（`C7 (MCP): B, who sees Eng, receives a pages frame of each of A's agent's writes, its creations, renames, moves, edits, a batch's one frame, and deletion, and nothing more; C, who does not, receives nothing of them`）：
  1. 准备照 API 版本（`:33-43`：B 以读者在 Eng 里、C 在自己的 Mine 里；`settleEvents` 之后开 B、C 的事件流）；A 的 agent 在 Eng 的入口（握手只写 PAT 的名称，没有页的写、没有帧；所有准备都在 `openEvents` 之前，`events.ts:95-100`）。
  2. `write {path: "Notes.md", content: "Draft\n"}` → `{tree: true, pages: [{id: node_id, revision: 1}]}`（新建的页不论有没有正文都在版本号 1 列出）；`write {path: "Parent.md", content: "# Parent\n"}` → 同样的形状；`move {from: "Notes.md", to: "Minutes.md"}` → `{tree: true, pages: []}`；`move {from: "Minutes.md", to: "Parent/Minutes.md"}` → 同；`edit {path: "Parent/Minutes.md", old_string: "Draft\n", new_string: "# Minutes\n", base_version: "<id>:1"}`（`edit` 要至少 1 字节的 `old_string`，`page/adapter/mcp/edit.go:15`，所以第一次的 `write` 写进 `Draft\n`；两次移动不改版本号，令牌仍是第一次 `write` 答的 `"<id>:1"`）→ `{tree: false, pages: [{id, revision: 2}]}`。每个帧带 `{workspace_id, notebook_id: eng.id}`，同 API 版本的 `ofEng`。
  3. **新的**：一个 `batch`（Minutes 的正文与新建 `Parent/Late.md` 两步）是**一个**帧 `{tree: true, pages: 两页}`：比较时 `pages` 按 id 排、长度是 2（一个单元一次 `tell`，按改动的次序列出）。
  4. `delete {path: "Parent.md", recursive: true}` → `{tree: true, pages: []}`（删除不列页：只列 `c.After != nil` 的，`events_registrants.go:23-32`）。
  5. **安静**（Go 的 `tm.quiet` 的做法，`events_pages_test.go:67-77`）：A 经 REST 写一次 Eng 的 Marker（准备时建、`settleEvents` 用的那一页，`c7-push.spec.ts:41`；settle 写过它，版本号先以 `readContent` 读出），B 的下一帧恰是它 `{tree: false, pages: [{marker.id, revision}]}`：删除之后单元若另发一个不同的帧，在这里失败（对照 236q：删除的单元另发 `{tree: false, pages: []}`，C7 (MCP) 在这一步、C7 (API) 在它新加的检查失败，`171b739d` 的 C7 (API) 通过）。它抓不到别的两种：batch 的第二个帧在第 3 步（236 的形状）或第 4 步（它等的是删除的帧）就失败，只在与删除的帧一模一样时才到这里；删除的"一页一帧"在流上看不出来，删除的每个改动都是 `{workspace_id, notebook_id, tree: true, pages: []}`、彼此逐字节相同，由单元的事务里的 `pg_notify` 发出，同一个事务里同一个通道上相同的载荷 PostgreSQL 只送一次（M5 记下过，`docs/v0.1/M5-collab-editing/00-M5-design.md:241`），抓到它的是 Go 的 `TestAPagesEventSaysWhatTheUnitWrote`（`events/app/publisher_test.go:57`，假的通知者不合并）。（S1 的实施者的发现，审查 A 与 C-I1；原写"第 3 步的 batch 若发了第二个帧、第 4 步的删除若一页一帧，都在这里失败"，3.9。）
  6. C 的第一帧是它自己在 Mine 里的写（REST，照 API 版本），所以 Eng 的帧一个都没到 C。
- 不同：会话的开与结束（锁的帧，API 版本的 `:57-60`）没有 MCP 的形式。
- **C7 (API) 一起改**：它在删除之后只读一帧、之后再不读 B 的流（`c7:62-63`），一个删除发两帧它照样通过；在同一个提交里给它加同样的第 5 步：先把 Marker 提成一个变量（今天它在 `settleEvents` 的调用里就地建，`c7-push.spec.ts:41`），删除之后读出它的版本号、写它一次、`expectNext` 下一个版本号的帧，三行。版本号照 C7 (MCP) 先以 `readContent` 读：`settleEvents` 在它的流先结束时会再写 Marker（`e2e/fixtures/events.ts:95-113`），它不总是 2（S1 的审查 A-M1 = C-M1，修复一轮；原写"它的版本号不必读：建时 1，settle 写成 2"，3.9）。

### 3.4 链接的故事

**L1 链接**（`e2e/stories/links/l1-links.spec.ts`，API 版本 `:17-74`）

- 断言：`expectIndexedLinks` 三次（`e2e/fixtures/assert/links.ts:34-48`，收来源与目标的 id），id 取 `write` 的 `node_id`；移动之后断言 `move` 的 `node_id` 等于 Target 的。另加 `expectChangesetOf(db, move.changeset_id, {client: "mcp:e2e-agent", byId: adminId, items: [target], versions: [source]})`：这里的改写含锚点与 Markdown 链接，MC4 的只有 wikilink 与嵌入（`mc4-move-delete.spec.ts:41`、`:72-77`）。不加 fixture。
- 步骤（`L1 (MCP): an agent writes a page whose wikilink, anchor link and Markdown link lead to pages, one to a page not there; the index has them, the missing one resolving once the agent writes it; a move renaming the page writes the links again, each still leading to it`）：
  1. `write` Target（`"# Part Two\n"`）与 Source（`"[[Target#Part Two|see]] [[Missing]] [md](Target.md)\n"`）；`expectIndexedLinks`（Missing 为 null）。
  2. `write {path: "Missing.md", content: "# Missing\n"}`；`expectIndexedLinks`（Missing 指向它的 `node_id`）。
  3. `move {from: "Target.md", to: "Renamed.md"}`（`update_links` 默认 true，`page/adapter/mcp/move.go:33-34`）；`readPage {path: "Source.md"}` 的文字是 `"[[Renamed#Part Two|see]] [[Missing]] [md](Renamed.md)\n"`（锚点与别名留着），REST `readContent` 同（API 版本的读）；`expectIndexedLinks`；`expectChangesetOf`。
- 不同：阅读视图的三处（`getView` 的锚点与未解析的链接）是渲染，豁免（PG5 一类）；L1 (MCP) 的页由 agent 写（L1 的动作就是写带链接的页），API 版本的 `getView` 不在。

**L3 改写**（`l3-rewrite.spec.ts`，API 版本 `:30-60`）

- 断言：`expectIndexedLinks`（`:56-59`），加 API 版本读的 `content_updated_by`（REST `getPage` 与 `accountIdOf`，`:53-55`）；另加 `expectAliveSessions(db, source.id, [session.id])`（`e2e/fixtures/assert/collab.ts:18`）：被拒的移动没有结束会话（C4 (MCP) 的做法，`c4-guard.spec.ts:131`），与 `expectChangesetOf`：改写是移动者的、在它的客户端下。锁的 `user_id` 取 `accountIdOf(db, aEmail)`（API 版本已为 `content_updated_by` 读它，是 id 与答复比，不是库与库比）。不加 fixture。
- 步骤（`L3 (MCP): an agent's move whose rewrite reaches a page another edits is linking.pages_locked naming the page and its editor, the editor's own agent's too, and moves nothing; once the editor is done, the editor's agent renames it and the links follow, as the mover; a move without its links passes a page being edited and leaves it as it is`）：
  1. 准备照 API 版本（`:34-41`：管理员、成员 A、Target 与 Source、A 在 Source 上的会话）；两个 agent：管理员的 `admin-agent`、A 的 `a-agent`，各自的 PAT。
  2. 管理员的 agent `move {from: "Target.md", to: "Renamed.md"}` 答 `linking.pages_locked`，`locks` 恰是 `[{page_id: source.id, path: "Source.md", user_id: A 的 id, display_name: A, you: false}]`、`locks_count: 1`，文字含 `by <A 的显示名>` 与 `update_links false`（`page/adapter/mcp/locks.go:86-104`）；A 自己的 agent 同样的移动答同样的码、`you: true`，文字含 `by you, in a browser`（`locks.go:110-143`）。e2e 里第一次经 MCP 断言 `linking.pages_locked`（今天只有 Go，`server/internal/bootstrap/mcp_writes_test.go:388-405`）。
  3. 什么都没动：REST `listNodes` 的名称是 `["Source", "Target"]`、`readContent` 的版本号是 1（API 版本的读，`:48-49`）；`expectAliveSessions`。
  4. REST `endSession`（204）；A 的 agent `move {from: "Target.md", to: "Renamed.md"}` 答 `{path: "Renamed.md", node_id: target.id, changed: true}`。
  5. `readContent` 是 `["[[Renamed]] and [md](Renamed.md)\n", 2]`，`getPage(source).content_updated_by` 是 A 的 id；`expectIndexedLinks`；`expectChangesetOf(db, changeset_id, {client: "mcp:a-agent", byId: aId, items: [target.id], versions: [source.id]})`。
  6. **`update_links` 的另一种**（00 号文档表里"`update_links` 两种"）：A 再开 Source 的会话；管理员的 agent `move {from: "Renamed.md", to: "Renamed later.md", update_links: false}` 照样写下（不改写就不碰锁，锁的文字自己这样说，`locks.go:87`），答 `{path: "Renamed later.md", node_id: target.id, changed: true}`；`expectChangesetOf(db, changeset_id, {client: "mcp:admin-agent", byId: adminId, items: [target.id], versions: []})`：没有 Source 的版本；Source 的 `readContent` 与之前相同（正文与版本号 2）；`expectIndexedLinks` 的两个链接都为 null（改了名，都不再指向它，同 `mc4-move-delete.spec.ts:103-112`）；`expectAliveSessions`（第二个会话）。用改名而不是移进一页：纯的移动之后相对的 `Renamed.md` 解析不解析取决于解析器的后缀匹配，页面版本以带根的链接避开它（`l3-rewrite.spec.ts:74`）。
- 不同：MCP 的拒绝的 `locks` 带 `path`、`you` 与 `locks_count`（REST 的没有，`:46`），所以期望的是 MCP 的形状。

**L4 标签与公式**（`l4-tags-math.spec.ts`，API 版本 `:44-75`）

- 断言：`expectTagged`（本地，`:38-42`：`expectIndexedTags` 三次，`links.ts:51-59`），id 取 `write` 的 `node_id`。标签的读照 P4 第 8 节的移交"由 `index` 读，照 MC6、MC7 的比较 REST 的写法"，但先比字面：`index {tag}` 与 REST `getTag` 读的是同一条 `TagPages` 的语句（`linking/app/list_index.go:102`、`linking/app/tags.go:55`、`reads.sql:96-105`），两边比只证明它们一样，标签的选法错了两边一起错。所以每个 `index {tag}` 的答复先与字面的路径的列表比，次序是这些页由 `write` 答的 id 的次序（没有类型的页按 id，同 MC7 的 `byId`，`mc7-index-backlinks.spec.ts:58-61`），之后再与 `getTag` 映射成路径的比（不认识的 id 映射成它自己、于是比较失败，同 `mc7:179`）。不加 fixture（本地的 `IndexAnswer` 接口与 `next_cursor` 为 null 的检查）。
- 步骤（`L4 (MCP): an agent writes pages with tags in their text, a tag under another's; the index has them; index by a tag gives its pages and those under it, and the pages' tags fold to the notebook's list of tags, as REST reads them`）：
  1. `write` Source（`tagged`，正文里的 `#proj/alpha`、公式与 mermaid，`:22-35`）、Deeper（`#proj/alpha/beta\n`）、Other（`#proj\n`）；`expectTagged`。
  2. `index {tag: "proj/alpha"}` 答 Source 与 Deeper，`index {tag: "#proj"}` 答三页，`index {tag: "proj/alpha/beta"}` 答 Deeper：各先与字面的（按 id 排的）路径比，再与 REST `getTag` 映射之后比；`#proj` 对 `getTag("proj")`（`index` 去掉开头的 `#`，REST 的 `getTag` 不去，`linking/adapter/markdown/tag.go:19-31`）。
  3. `index {}` 每页的 `tags` 折成 `[{tag, count}]`（每个标签有它的页数，按标签排），等于 API 版本的字面 `[{proj, 1}, {proj/alpha, 1}, {proj/alpha/beta, 1}]`（`l4:70-74`），也等于 REST `listTags`（`listTags` 的数是每个标签的页数，`linking/adapter/postgres/gen/reads.sql.go:242-248`），每个 `cut` 为 `[]`。
- 不同：阅读视图的标签、公式与图（`:54-62`）是渲染（表里"公式的渲染不在 MCP 版本里"）。MC7 的标签都在 frontmatter 里；正文里的标签与三层的嵌套是新的。

**L6 右栏**（`l6-panel.spec.ts`，API 版本 `:32-88`）

- 断言：`expectLinked`（本地，`:24-30`：`expectIndexedLinks` 两次），id 取 `write` 的 `node_id`；反链经 `backlinks` 与 REST `listBacklinks` 比（路径换成 id、`links` 对 `count`，同 `mc7:177-198`），先断言字面的 `[path, links]`，次序照 `write` 的 id（`byId`），不靠 UUIDv7 的 id 随请求递增；MCP 自己的 `next_cursor` 也断言；事件的帧照 API 版本（`:60-71`）。不加 fixture（本地的 `Backlink` 接口）。
- 步骤（`L6 (MCP): an agent reads a page's backlinks by its path, a property's link with its line, as REST's; one it writes shows at once, the links event naming the page linked, a page of them at a time`）：
  1. `write` Hub、Other（各一行短的正文，不依赖空的正文；Other 的不链到 Hub）、Doc（frontmatter 的 `up: "[[Hub]]"`、`see: "[[Nowhere]]"` 与 `# Intro`）。
  2. `backlinks {path: "Hub.md"}` 答 `[{path: "Doc.md", links: 1, contexts: ['up: "[[Hub]]"']}]`、`next_cursor: null`，等于 REST 的。
  3. **先 settle 再开流**：Doc 自己的写把它的属性链到 Hub，它的单元发出的 `links {pages: [], targets: [hub], aliases: false}`（`linking/app/index.go:36-92`、`:113-123`）正是这一步要等的帧；它晚到时，这一步在一个丢掉 MCP 的正文写的 `links` 事件的变异下照样通过。所以 `settleEvents(api, nervewiki.baseURL, pat, <在第 1 步之前建的 Marker>)`（`e2e/fixtures/events.ts:95-113`；C7 (API) 的做法，`c7-push.spec.ts:41`；Marker 先建，Hub 的版本号不动），然后 `openEvents(pat)`；`write {path: "Other.md", content: "Meet at [[Hub]] today\n", base_version: Other 的版本}`；第一个不是 `pages` 的帧是 `links {workspace_id, notebook_id, pages: [], targets: [hub], aliases: false}`：e2e 里第一个在 MCP 的写之后看的 `links` 帧。
  4. `backlinks` 全部（Other、Doc，按 id）、`limit: 1` 与它的游标，各等于 REST 的，各自的 `next_cursor` 照断言。
  5. `expectLinked`。
- 不同：页的属性的读（`getPageProperties`，`:43-54`）没有 MCP 的工具，`index` 只给类型、摘要与标签；属性的链接的解析由 `backlinks` 的上下文与 `expectLinked` 看到。大纲、右栏的布局、键盘与大页是浏览器的。
- **L6 (API) 一起改**：它开流之前不 settle（`l6-panel.spec.ts:58`），有同样的洞；在同一个提交里加同样的 Marker 与 `settleEvents` 一行。

### 3.5 附件的故事

**AS2 粘贴**（`e2e/stories/asset/as2-paste.spec.ts`，API 版本 `:30-69`）

- 断言：`expectEmbedded(db, storageDir, {written: writtenOf(w), content, attachments: [dropped, pasted]}, pngBytes, adminId, "api")`（`assert/asset.ts:173-192`：`expectContentWritten`、每个附件的 `expectUploaded`、`parent_id`、嵌入的 `expectIndexedLinks`），要 3.1 的第 1、3 条；`client` 照旧是 `"api"`：它是**上传**的客户端，上传只有 REST。正文的写是 agent 的，由另加的 `expectChangesetOf(db, w.changeset_id, {client: "mcp:e2e-agent", byId: adminId, items: [], versions: [guide]})` 证明（只写正文的写没有条目：条目只记移动的节点，`page/app/unit.go:419-431`）。
- 步骤（`AS2 (MCP): an agent reads the page's uploaded images, each as itself with how to link it and its address, and writes the page with their embeds; the index leads them to the attachments`）：
  1. 准备照 API 版本经 REST：Guide（`"Notes of the day.\n"`）；**另在别处（根下的 Other 页下）先上传一个同名的 `chart.png`**，之后上传 Guide 下的两张 PNG（`uploadAsset`，`e2e/fixtures/assets.ts:33-46`）。名称唯一的附件的 `link` 就是它的名称（`linking/app/asset_links.go:36-40`），分不出"链接"与"名称"；有了同名的那一个，Guide 下的 `chart.png` 的 `link` 是带路径的。先传别处的那一个：`expectEmbedded` 比的是上传的答复的 `link`（`assert/asset.ts:190`），Guide 下的两个的答复要在第二个同名出现之后给出，才带着带路径的 `link`。agent `readPage {path: "Guide.md"}` 得 `"<id>:1"`。
  2. 两个 `read`（`Guide/chart.png`、`Guide/Pasted image 20261009120000.png`）：第一块的 `link` 等于上传的答复的 `link`（`chart.png` 的是带路径的那种），`url` 是 `http://wiki.example` 加 `/api/v0/assets/<id>/content` 的签名的路径（`page/adapter/mcp/read.go:40-56`、`:162-165`），换成 `baseURL` 下载 200、同样的字节；第二块是 `image`，base64 的字节等于 `pngBytes`（`read.go:136-152`）。
  3. `write` Guide 带它的版本，正文的两个嵌入以 `read` 给的 `link` 写（agent 写嵌入的方法："how to link it"）；`expectEmbedded`；`expectChangesetOf`；REST `getView` 的两个 `<img class="nw-asset">` 的 `src` 是 `/api/v0/assets/<id>/content`、dropped 在前（API 版本的读，`:64-68`）。
- 不同：粘贴与拖放是编辑器的。

**AS3 渲染**（`as3-render.spec.ts`，API 版本 `:62-150`）

- 断言：`expectIndexedLinks` 两次（`:125-134`、`:141-150` 的八个链接，原样），页的 id 取 `write` 的 `node_id`，附件的 id 取 REST 上传的答复（API 版本同样）；另加 `expectChangesetOf(db, move.changeset_id, {client: "mcp:e2e-agent", byId: adminId, items: [cover.id], versions: [guide]})`。REST 的读照旧：`getView`（`:79-110`）、`getPageProperties`（`:113-123`）与改名之后的 `readContent`、`getView`（`:137-140`）。不加 fixture（`upload` 是本地的，`:34-43`）。
- 步骤（`AS3 (MCP): an agent writes a page embedding attachments, in its properties too; the index has their ids; a move renaming an attachment writes its embeds and its property's link again, caption and width kept, each still leading to it`）：
  1. REST 上传五个文件到根下（`upload`）；`write {path: "Guide.md", content}`（`:26-28` 的 frontmatter 与嵌入）；REST 的 `getView` 与 `getPageProperties` 照 API 版本；`expectIndexedLinks`。
  2. `move {from: "cover.png", to: "front.png"}`（根下的改名，附件的扩展名留着）答 `node_id` 是 cover 的；`readPage {path: "Guide.md"}` 与 REST `readContent` 的文字都是原来的把每个 `[[cover.png` 换成 `[[front.png`（属性与嵌入，标题与宽度留着）；`getView` 的图片仍在 cover 的 id；`expectIndexedLinks`（八个，`data.zip` 的两个不动、`missing.png` 仍为 null）；`expectChangesetOf`。
- 不同：页由 agent 写（AS3 的动作之一）；媒体的播放、PDF 的标签页、下载是浏览器的。上传的答复的类型（AS3 (API) 的 `[cover.mime, …]`，`as3-render.spec.ts:81-87`）不再查：两个测试是同一个本地的 `upload`，AS3 (API) 查过（S1 的审查 A-N1）。MC4 改名过页下的一个附件（`mc4-move-delete.spec.ts:79-88`），属性里的链接、带标题与宽度的嵌入与改名之后整个的索引是新的。

**AS4 生命周期**（`as4-lifecycle.spec.ts`，API 版本 `:43-102`）

- 断言：`expectAssetsDeletedWithNodes`（`assert/asset.ts:108-116`，收附件的 id，取 REST 上传的答复），两次、id 的列表都不空（空的列表会空过，`:110-115`）。API 版本另外两处在 MCP 版本里没有：删除笔记本之后的 `expectAssetsDeletedWithNotebook` 没有 MCP 的工具（PG13 的豁免）；61 天之后的清理 `expectAssetsPurged` 是平台的定时任务，不是 agent 的动作，清理的行与 MCP 的删除留下的同样（`deletedDaysAgo` 改写工作区的每个 `deleted_at`，`e2e/fixtures/purge.ts:14-34`），再等一次 15 秒的轮询不证明 MCP 的任何事。AS4 的第二个 API 测试（`:104-130`，无主的笔记本的大小与活动算附件）同样没有 MCP 版本：大小读的是没删的 `asset_blobs`，第 4 步已经证明 MCP 的删除让行离开。这两处写进 00 号文档的 AS4 一行（2.1 第 7 条）。另加一个，API 版本没有：删掉的附件的文件仍在存储里（`blobPath`，`assert/asset.ts:21`；恢复要它们，文件只由清理删）。不加 fixture。
- 步骤（`AS4 (MCP): an agent's deletion of a page with attachments under it counts them when refused without recursive, and with recursive takes their rows at its time, their addresses no file, their files kept; its deletion of an attachment alone takes its row`）：
  1. REST 种下 Root > Child，`deep.png` 在 Child 下、`shallow.txt` 在 Root 下、`kept.txt` 与 `single.txt` 在根下。
  2. `delete {path: "Root.md"}` 答 `validation_failed`，文字说出"1 page and 2 attachments"（同 `mc4-move-delete.spec.ts:120-122`）。
  3. `delete {path: "Root.md", recursive: true}` 答 `[node_id, pages, attachments]` = `[root.id, 2, 2]`；`expectAssetsDeletedWithNodes(db, [deep.id, shallow.id])`；两个文件仍在存储里；REST `getAsset(deep)` 答 `asset.not_found`、它的签名的地址 404、`kept.txt` 的 200（API 版本的读）；`read {path: "Root/Child/deep.png"}` 答 `page.not_found`。
  4. `delete {path: "single.txt"}` 答 `[single.id, 0, 1]`；`expectAssetsDeletedWithNodes(db, [single.id])`；它的签名的地址 404：内容的路由在读行之前就因节点已删答 404（`asset/app/content.go:59-64`），行由 `expectAssetsDeletedWithNodes` 看到。
- 不同：上面三处没有；"单独删一个附件，行随之删除"（表里的说明）是 API 版本没有的一步。

### 3.6 历史：HI1 版本

**HI1 版本**（`e2e/stories/history/hi1-versions.spec.ts`，API 版本 `:22-113`）

- 断言：`expectVersions`（`:108-112`）原样：版本是 REST 的会话写下的，比的是测试自己的字面的值；agent 再写一次之后多一行，`{base: 4, revision: 5, content, kind: "edit", byId: adminId, client: "mcp:e2e-agent"}`，版本号由答复的 `version` 给。`versions` 的列表与 REST `getVersions` 整行比（MC8 的做法，`mc8-history.spec.ts:153-162`），另断言三行的投影是 API 版本的字面的值、长度 3（两边都空时整行比较会空过）。BOM 与 NFD 的正文用 API 版本的常量（提到文件的顶层、不导出），不重打（`:28-32` 里是原样的 U+FEFF 与分解的 é）。REST 的 `readContent === third`（`:106`）照旧，`readPage` 的文字另比一次。不加 fixture。
- 步骤（`HI1 (MCP): an agent lists a page's versions as REST does, each edit session one version, the newest marked while its session is alive; reads a version by any revision it held, answered with its own, byte for byte; its own write is a version of its own`）：
  1. 准备照 API 版本（`:26-39`：两个会话、四个版本号、第二个会话开着）。
  2. `versions {path: "Tasks.md"}` 的 `[revision, base_revision, editing]` 是 `[[4, 3, true], [3, 1, false], [1, null, false]]`。
  3. REST `endSession`；`versions` 的列表整行等于 REST 的、投影等于字面的值、`next_cursor` 为 null、没有一行带 `content`，`current_version` 是 `"<id>:4"`，等于 `readPage` 的 `meta.version`；`readPage` 的文字与 REST `readContent` 都是 `third`。
  4. `versions {path, revision: 1}` 答 `[1, created]`（逐字节：BOM、CRLF 与分解的 é 都在）；`{revision: 2}` 答它的版本自己的 3 与 `second`（持有 2 的版本答它自己的版本号）；`{revision: 5}` 答 `page.version_not_found`。
  5. agent `write` 带 `"<id>:4"` 答 `"<id>:5"`；`versions` 四行，最新的客户端是 `mcp:e2e-agent`、`editing` 为假；`expectVersions` 四行。
- 不同：会话本身（开、存、结束）只有 REST，HI1 要证明的"一个编辑会话一个版本"只有它们能造；版本页、比较与别名的链接是浏览器的。API 版本的 `getVersion(2)` 的两项（版本的 `id` 等于列表那一行的、`base_revision` 是 1，`hi1:97-103`）在 MCP 下没有：`versions {revision}` 的元数据只有它自己的 `revision` 与 `current_version`（`page/adapter/mcp/versions.go:100-118`），那一行的 `id` 与 `base_revision` 由第 3 步的整行比较看到。

### 3.7 搜索的故事

**SE1 搜索**（`e2e/stories/search/se1-search.spec.ts`，API 版本 `:119-168`）

- 断言：SE1 不断言库（文件自己这样说，`:21-23`）；它的两个版本对同一张查找表 `finds`（`:63-76`）调同一个本地的 `expectFound`（`:79-82`，总体设计 13.4 第 3 条）。MCP 的 `search` 不给命中的范围与 id（项是路径、种类、摘要与时刻，`page/adapter/mcp/search.go:32-37`），`Found.marked` 造不出。`Found.marked` **照旧是必须的**：MCP 的调用处明说不比它，`expectFound` 多一个重载，收 `Omit<Found, "marked">` 与 `{marked: false}`（`expectFound(q, found, {marked: false})`），`found` 是本地的 `mcpFoundOf` 造的 `{names, count}`，类型就是 `Omit<Found, "marked">`，从不造一个假的 `marked: []`；API 与页面版本照旧整个比（由找到的对象决定比不比，会让以后不再设 `marked` 的 `foundOf` 或 `shownOf` 悄悄地少比，`se1:90-117`）。缺的范围由两处补上：每个查询的 MCP 的 `[path, kind]` 等于 REST 按这本笔记本的搜索的每个命中由 `ancestors` 与 `node.name` 拼成的路径与它的种类，`count` 相等（MC6 的做法，`mc6-grep-search.spec.ts:125-143`；`written()` 只答出四个节点，`se1:37-53`，别的页没有 id，所以比路径不比 id；每个查询只有一个命中，比不出次序）；Storage 的项的 `excerpt` 等于 REST 的 `excerpt.text`，`cut_start`、`cut_end` 为真时两头各加 `…`（范围标的就是这段文字）。不加 fixture（本地的 `mcpFoundOf`）。
- 步骤（`SE1 (MCP): an agent finds a notebook's pages by their titles and contents and an attachment by its name, as REST's search of the notebook, counted exactly; CJK from two characters and letters from three as substrings, two letters as a whole word, one CJK character in titles alone; a page deleted is found no more, and again once restored`）：
  1. `written()` 照旧（REST，含上传的附件）；agent 在这本笔记本的入口。
  2. 表里的每个 `q`：`search {q}` 的名称（路径的最后一段，页去掉 `.md`）与数（`count.kind` 是 `exact` 时）经 `expectFound(…, {marked: false})`；`next_cursor` 为 null、`building` 为假；与 REST 比路径、种类与数。
  3. Storage 的项 `{path: "Storage.md", kind: "page", excerpt: REST 的 excerpt.text（照 cut_start、cut_end 加 …）}`；附件的 `{path: "Storage/Reports/季度报告.pdf", kind: "attachment", excerpt: null}`。
  4. `delete {path: "Storage.md", recursive: true}`（Storage 下有 Reports 与附件）答 `[node_id, pages, attachments]` = `[storage.id, 2, 1]`；紧接着 `search {q: "检索"}` 答名称 `[]`、数 0（在 5 秒的搜索任务删掉回收站里的节点的行之前，`page/app/index_search.go:159-168`：这时排除它的是搜索的语句自己）；REST `restoreNode`（恢复只有网页与 REST，TB 的豁免）之后照表又找到。MCP 的删除立即离开搜索是新的（API 版本以 REST 删除）。
- 不同：SE1 (API) 是工作区的搜索；MCP 只在一本笔记本里搜（`search.go:55` 的 `InNotebook`），而 SE1 的工作区只有这一本，所以结果相同。

**SE2 搜索的可见性**（`se2-search-visibility.spec.ts`，API 版本 `:58-88`）

- 断言：SE2 不调 `expectFound`，也不断言库，它的检查是本地的 `namesFound`（`:52-56`）与 404。MCP 版本做能映射的那些：缩小到一本笔记本的查找与三个拒绝；工作区范围的三个查找没有 MCP 的形式（没有一个工具搜多本笔记本；逐本搜再按时刻合并测的是测试自己的合并）。`notebooks` 是 notebook 模块的列表，不是搜索的可见性（搜索以 `Visible.VisibleIn` 定范围，`page/app/search.go:171-175`）：所以每个账户在它列出的每一本里都搜一次，管理员 4 次、成员 2 次、访客 1 次，各恰好一个命中（只比列表时，一个在笔记本的范围里拒绝"经 `viewer` 读的笔记本"（成员的 Shared）的变异两个版本都通过）。名称是路径的最后一段去掉 `.md`。不加 fixture（本地的 `mcpNamesFound`）。
- 步骤（`SE2 (MCP): an agent searches each notebook it sees, and finds its page there; the notebooks it may search are those it sees; one it does not see is the notebook entry's 404 and the account entry's notebook.not_found`）：
  1. 准备照 API 版本（`:61-64`，四本笔记本各一页带 `zephyr`）。
  2. 管理员、成员、访客的账户入口的 `notebooks`：`[id, role]` 是 `[[Guests, admin], [Open, admin], [Private, admin], [Shared, admin]]`、`[[Open, editor], [Shared, reader]]`、`[[Guests, reader]]`（id 来自 `notebooksOf`，`:38-49`），`truncated: false`。
  3. 每个账户对它列出的每一本 `search {notebook, q: "zephyr"}`：答那一本的那一页（如成员在 Open 里 `["Open plan"]`、在 Shared 里 `["Shared plan"]`），数都是确切的 1。
  4. 三对（成员与 Private、成员与 Guests、访客与 Open）：笔记本入口的 `postMcp(initialize)` 各 `[404, "application/problem+json", "no-store"]`、`notebook.not_found`；账户入口的 `search` 各答 `notebook.not_found`。列表是字面的三对。
- 不同：工作区范围的搜索与它的跨笔记本的次序没有；页面版本的提示与"整个工作区"是网页的。

### 3.8 豁免的核对

MCP 的工具面：`tools.json` 两个入口 14 与 16 个工具；`mcpTools` 只拼 page、linking 与 notebook 的（`server/internal/bootstrap/mcp.go:45-47`）；asset、transfer、events 没有 `adapter/mcp`；page 的工具只调 `UseCases` 里的 14 个用例（`page/tools.go:31-37`，类型在 `page/module.go:233-246`），只有 REST 的用例——阅读视图 `GetPageView`（`:299`）、会话的 `OpenSession`、`Heartbeat`、`EndSession`、`GetEditLock`、`ReleaseEditLock`（`:303-307`）、`ToggleTask`（`:308`）、`GetVersionView`、`RestoreVersion`、回收站的三个、`RevertChangeset`（`:311-319`）、上传、导入与后台的写（`:324-325`）——一个都不在其中；linking 的工具只是 `ListBacklinks` 与 `ListIndex`（`linking/module.go:91-94`），落点与补全的 `GetLinkLanding`、`ListLinkTargets` 不是（`:89-90`）；notebook 的只有 `ListAccountNotebooks`（`notebook/module.go:140`、`:144-145`）；MCP 的传输无状态、JSON 的答复（`platform/httpserver/mcpserver/server.go:127-128`），POST 之外的方法是 SDK 的 405，没有事件流。

| 故事 | 表的理由 | 对着代码 |
|---|---|---|
| PG5 阅读视图 | MCP 不渲染 | 成立。`read` 答元数据一行与原样的行（`page/adapter/mcp/read.go:94-115`），没有一个工具答 HTML；能经 MCP 的建页是 PG1 (MCP) 的动作 |
| PG6 渲染安全 | 同上 | 成立。两个测试都不断言库；`read` 原样给出不安全的 HTML，本来就该这样，没有净化过的输出可查 |
| PG10 编辑会话 | MCP 不开会话 | 成立。工具的参数里没有会话（`write.go:14-18`、`edit.go:13-19`；`app.PathWrite`、`app.PageEdit`，`page/app/write_path.go:17-21`、`edit_page.go:14-19`）；负责人的决定"MCP 的写是一次性的，没有编辑会话"（00 号文档 `:640`）；MCP 碰到会话的是 C4 (MCP)，agent 的写不留会话由 PG7 (MCP) 断言 |
| PG13 删除笔记本 | 动作是删除笔记本；之后入口 404 由 MC1 覆盖 | 成立。没有删除笔记本、工作区或无主的笔记本的工具；MC1 删掉一本之后它的入口答 404 `notebook.not_found`、`application/problem+json`、`no-store`（`mc1-connect.spec.ts:125-126`、`:136`、`:140-144`） |
| C1–C3、C5、C6、C8、C9 | 锁、接管、释放、自动保存、会话过期、事件流：MCP 没有这些动作 | 成立。会话的五个用例只有 REST（`page/module.go:303-307`，`page/adapter/http/handler.go:268-320`）；agent 碰到的锁是写的守卫（`page/app/edit_lock.go:42-66`），由 C4 (MCP) 与 `server/internal/bootstrap/mcp_lock_test.go:39` 覆盖；C5、C9 只有页面版本（C9 是一个 `for` 循环声明的两个测试，`c9-one-stream.spec.ts:28-95`）；没有事件流（C8）。C6 的一个后果能经 MCP 看到："过期的会话不再拒绝 agent 的删除"，由 PG4 (MCP) 的第 4 步断言（3.2；没有一个 MCP 版本去写一页会话已过期的页），C6 照旧豁免 |
| C10 任务 | 勾选是网页的瞬时写；`edit` 能改但不是这个动作 | 成立。`ToggleTask` 只有 REST（`page/module.go:308`），按阅读视图的复选框的位置、校验它是任务项（`page/app/toggle_task.go:11-13`、`:27-47`）；`edit` 是字节的查找替换，没有这些（MC3 以它把 `- [ ]` 换成 `- [x]`，`mc3-write-edit.spec.ts:119-129`）。所以 `expectToggleRevision` 的客户端类型不必放宽（3.1） |
| L2 新建链接的页 | 动作是链接的落点（网页） | 成立。`GetLinkLanding` 不是工具；agent 以路径写，`[[Hub/Next]]` 的页由 `write "Hub/Next.md"` 放在它该在的地方，没有落点的问题；"悬空的链接在页建成之后指向它"是 L1 (MCP) 的第 2 步 |
| L5 补全 | 编辑器 | 成立。`ListLinkTargets` 不是工具，`index` 不列别名；带链接与标签的写是 L1、L4 的 MCP 版本的 |
| L7 软换行 | 渲染 | 成立。不断言库；`read` 给原样的行 |
| AS1 上传 | MCP 不上传 | 成立。没有写字节的工具（`write` 只收 UTF-8 的页的正文）；能经 MCP 的一半（附件的读、同样的字节、签名的地址的下载）是 MC2 的第 5 步（`mc2-read-list.spec.ts:190-227`） |
| AS5 内容安全 | 只有浏览器能证明 | 成立。MCP 的答复里只带图片的字节；别的附件只给内容路由的下载的地址（`asset/attachment_reads.go:58-69`），即 AS5 (API) 已查的那个路由（`as5-content-safety.spec.ts:93-95`） |
| TR1–TR4 | 导入导出是任务 | 成立。transfer 没有 `adapter/mcp`；`transfer_jobs.client` 只收 `web`、`api`（`00029_transfer_transfer_jobs.sql:23`，之后的迁移没有改它）；负责人的决定"v0.1 里 MCP 不开后台任务"（`:639`） |
| TB1、TB2 | 恢复是网页的动作；MCP 的删除由 PG4 覆盖 | 成立，P5 之后照字面成立：PG4 (MCP) 的第 4 步以 TB 自己的断言 `expectTrashRoot(…, {alone: true})` 覆盖 MCP 的删除进回收站（3.2）；今天覆盖它的是 MC4（`mc4-move-delete.spec.ts:123-134`），不带 `alone`。2.1 第 8 条把"PG4"写成"PG4 的 MCP 版本" |
| HI2–HI5 | 还原与撤销是网页的动作；MCP 的历史的读由 MC8 覆盖 | 成立。`RestorePageVersion`、`RevertChangeset` 与后台的撤销只有 REST（`page/module.go:311-319`；`transfer/adapter/http/server.go:55`）；工具的说明"History is read-only here"（`page/adapter/mcp/tools.go:231-232`、`:238`）；负责人的决定 1（`:639`）。MC5 (page) 在网页上撤销 agent 的 batch（`mc5-batch-revert.spec.ts:160-193`）。HI4 的撤销删除靠"每个条目带着它的页的版本号"（`expectDeletionRevisions`，`assert/page.ts:567`；今天只有 HI4 调它：撤销的再删除、REST 的与网页的删除之后，`hi4-revert-delete.spec.ts:96`、`:111`、`:158`），MCP 的删除之后由 PG4 (MCP) 的第 4 步断言（3.2） |

49 个故事（page 14、collab 10、links 7、asset 5、history 5、search 2、trash 2、transfer 4）每个恰在一行里：22 个有、27 个豁免，00 号文档 `:598` 的数对。表上面那一句的两类（`:557`）漏了删除笔记本、释放锁、自动保存与勾选任务，2.1 第 8 条补上。

**不采纳的可选的增加**（研究里找到的，每个都只是已有 MCP 版本的故事的一两行；不做的理由）：

- MC5 (page) 末尾以 MCP 的 `changeset` 读撤销与被撤销的 batch、与 REST 比（MC8 的数据里没有撤销，`reverts`、`reverted_by` 在比较里只是空的）：映射有单元测试（`page/adapter/mcp/history_test.go:314-366`），MC5 (page) 以库的 `revertsOf` 核对撤销（`:190`）；不碰 P3 的故事。
- L5 的别名（只有 L5 断言 `expectIndexedAliases`）：别名的解析是共用的解析器的（`page/module.go:328`），MCP 写不改变它。
- L1 (MCP) 的悬空的链接写成带路径的 `[[Hub/Next]]` 以带上 L2 的结果：那样 L1 (MCP) 的正文就与 L1 (API) 的不同了。
- MC1 加一本随工作区删掉的笔记本、MC2 的下载加 CSP 的头：与已测的是同一段代码。

### 3.9 S1 的实施对设计的修正

S1 的实施见第 12 节，审查与修复核对见 [S1 的审查记录](reviews/P5-S1-stories-docs-review.md)。与上文最初的设计不同之处都是最小的改动，不改服务端、不改答复与码；各处已照实改在上面的节里，这里列出。主持者的决定记在 00 号文档第 11 节第 96–104 条。

- **实施时定下的**：
  1. **第 236 条变异的两处说法不对**（3.3 第 5 步、9.2 的第 236 条；实施者的发现，审查 A 与 C-I1 复现并补全；00 号文档第 11 节第 103 条）：删除的"每个改动一帧"在事件流上看不出来。删除的帧都是 `{workspace_id, notebook_id, tree: true, pages: []}`，彼此相同；帧由单元的事务里的 `pg_notify` 发出，同一个事务里同一个通道上相同的载荷 PostgreSQL 只送一次（M5 记下过，`docs/v0.1/M5-collab-editing/00-M5-design.md:241`）。所以在 e2e 里 236 只由 C7 (MCP) 第 3 步抓到（batch 的两页成两帧，`c7-push.spec.ts:170`）；加了安静的检查的 C7 (API) 在 236 之下**通过**（9.2 原说它失败）；C7 (MCP) 第 5 步也抓不到删除的那一半（236-del：第 3 步只收下 batch 的帧的临时副本在 236 之下通过）。删除的那一半由 Go 的 `TestAPagesEventSaysWhatTheUnitWrote`（`events/app/publisher_test.go:57`，假的通知者不合并）抓到："sent 3 payloads, want one"（审查 C；实施者的草稿没说这一点）。安静的检查本身是对的：对照 **236q**（删除的单元另发一个不同的 `{tree: false, pages: []}` 帧，`bootstrap/events_registrants.go:31`）让 C7 (MCP) 第 5 步与 C7 (API) 新加的检查失败，`171b739d` 的 C7 (API) 通过，这正是新加的两行补上的洞。测试不改，改的是设计说它们为什么在。
  2. 第 229 条变异写成 `AND $2::timestamptz IS NOT NULL`：直接去掉 `AND expires_at > $2` 时语句只剩一个参数，生成的 `AliveSessionsOf` 照旧传两个（`gen/sessions.sql.go:29`），pgx 每次调用都报"expected 1 arguments, got 2"，不是要的变异（审查 C-N2 改正了草稿里"`$2` 没有类型"的理由）；这样写同去掉这个条件（`Now` 从不为 null）。
  3. 第 230 条分两半做：230w 在 `write_path.go:224` 的 `Changed` 的调用处（`:250` 是 `mismatch` 帮助函数，那里没有 `base`），230e 在 `edit_page.go:146`；各自让 PG8 (MCP) 第 3 步失败，哪一半都不靠另一半。
  4. `PageRead.meta` 写出 `read` 一页的第一块的每个成员（`path`、`kind`、`version`、`offset`、`column`、`lines`、`total_lines`、`byte_size`、`next_offset`、`next_column`），不再是 MC2 的四个加一个索引签名：新的故事读它的 `version` 与 `path`（3.1 第 4 条）。
  5. HI1 提到顶层的是四个正文常量（`created`、`first`、`second`、`third`），不是第 8 节第 7 步说的两个：MCP 版本的准备照 API 版本（3.6 第 1 步），四个都要。HI1 (page) 自己的三个同名的纯文本常量随之遮住它们（oxlint 的 `no-shadow`，`make lint` 要零警告），改名为 `plainCreated`、`plainSecond`、`plainThird`，行为不变（第 0 节、第 8 节第 7 步）。
  6. README `:219` 的一行用 ASCII 的双引号：`接上 Claude Code、Codex 见下一节"接入 agent（MCP）"；反向代理的要求见"部署"。`。第 4 节原来的单引号是这一行嵌在设计自己的引号里的写法；README 的正文里指一节都用双引号。
  7. README 的两处照 `oxfmt` 加了列表项之间的空行、对齐了两张表；去掉空白与表格的 `|`、`-` 之后与 4.1、5.1 的文字逐字相同（审查 B、C 比过；4.1 现在是建成的样子，5.1 与 README 逐字节相同）。清单在 `docs/**` 之下，`oxfmt` 不管，建成时与 6.1 逐字节相同。
  8. Caddy 的重跑的脚本的副本只在末尾加了停掉它们在后台起的 Caddy（`kill` 与 `wait`），量的部分不变。
  9. 提交的标题照实施的说明写作 `test(M9/P5 S1)`、`docs(M9/P5 S1)`（第 8 节写的是 `test(e2e)`、`docs(readme)`、`docs(M9)`）；第 8 步只做变异、不提交，所以是十个提交，另有合并 `main` 的一个。README 到清单的链接在第 9 步的提交里写下、第 11 步的提交之后解析得到（`make lint` 不查链接）。
  10. 实施的写法，不是与设计的不同（审查 C-N5）：PG8 (MCP) 的过时的 `write` 与过时的 `edit` 依次调用，不用 `Promise.all`（3.2 的 PG8 没要它）：第 247 条之下并发时失败落在了 `write` 上，依次调用时每个失败指名它自己的调用；第 235b 条改两个文件（9.2 自己这样写）；AS4 (MCP) 放在 AS4 (API) 的第一个测试之后。
- **审查与修复定下的**（S1 的审查记录；修复一轮十个提交、二轮三个）：
  1. **C7 (API) 读出 Marker 的版本号再写**（审查 A-M1 = C-M1，`5cb7e824`；第 104 条）：`settleEvents` 在它的流先结束时会再写 Marker，版本号不总是 2；照 C7 (MCP) 先以 `readContent` 读，等下一个版本号的帧（3.3 的"C7 (API) 一起改"、9.1 表下的一句）。
  2. **README 里 MCP 的拒绝写码，不写 HTTP 的状态**（审查 C-N7，`17f0292e`；第 101 条）：接入一节的"写的规矩"一处（4.1、4.3），与 P1–P4 写的"MCP"一节八处，这份设计里没有那一节的原文，记在这里：
     - "写"：`write` 的路径已是一页而内容不同时"答 409 `page.title_taken`"成了"答 `page.title_taken`"；带 `base_version` 而那一页之后被写过时"答 409 `page.revision_mismatch`（带现在的版本）"成了"答 `page.revision_mismatch`（带现在的版本）"；带了 `base_version` 的 `edit`"在那之后被写过答 409"成了"答 `page.revision_mismatch`"。
     - "batch"：别人占着解析的预算时"照旧答 503 `server_busy`"成了"照旧答 `server_busy`"；"过了这些答 422 `page.batch_too_large`"成了"答 `page.batch_too_large`"。
     - "找"：太复杂的模式"答 422"成了"答 `validation_failed`"；等 1 秒取不到名额"答 503 `server_busy`"成了"答 `server_busy`"。
     - "索引与反链"：传 `""`"答 422"成了"答 `validation_failed`"。

     修复报告说八句、其中四句是 P1–P4 的；照审查 C 对着 diff 的核对，是九处，八处在"MCP"一节。HTTP 一层的状态（401、403、404、413、429、名额的 503、请求体的 400）照旧。
  3. **`MCP-Protocol-Version` 按版本写**（审查 B-N4，`0f747e13`；第 101 条）："HTTP 的答复"的 400/415 一行的"什么时候"改写，后面加 JSON-RPC 的 400 一行（4.1、4.3）。
  4. **清单**（6.1 是建成的文件，6.2 写理由；第 98–100 条）：
     - D1、D2、D3 各在只给这一步的新的临时目录里做、各有自己的一份 `mcp.json`，D 做完以 `${VAR:?}` 守着删掉它们、回到 `$CC`（审查 B-M1，修复核对一轮 B2-N1、B2-N2；`6de4fb9c`、`f60c4c80`）；
     - C11：种子的 TODO 行约 520 字节，第一次答复停在 48 KiB（85 个命中），不带 `limit` 时 8 次调用（B-M2，`dcefa8a8`）；
     - 图片与 PDF 要是名称说的类型，`sips` 转、`file` 核对（B-M3，`00a11a88`）；
     - I1 写它自己的 `经 nginx.md`，I3 以 `200k-caddy.bin` 上传，I 部分不靠前面的部分（B-M4、B-N9，`8b6267f6`）；I1 的浏览器经 `:8090` 是另一个源、在那里再登录（B2-N3，`380b253e`）；
     - A3 在命令之前与会话之后各数一次 `~/.claude.json` 里的 `"nervewiki"`（B 的存疑 1，`5ce22531`）；
     - 做完不给删除 Claude Code 的会话的命令（C 的存疑 1、B2-M1 = C2-M1；`bc009520`、`8067afd9`）；
     - 准备第 2 步设 `BASE`、`NB_CC`、`NB_CODEX`，第 I 部分设 `CADDY`；F1 的 `TOKEN` 以 `read -rs` 设；E3 也改三份 `*-literal.json`；准备第 1 步要 OpenSSL 3；E2 的 `/.well-known/...`；F1 的日志写作 `own=true`；E4 在登录后 15 分钟之内；`host.docker.internal` 未核对、不通时跳过容器的路，服务从不绑到 127.0.0.1 之外（B-N1–N3、B-N5–N8、B 的存疑 2；`e519297c`）。
  5. **设计的文字的更正**：第 8 节第 11 步不列"M5/P2"（C-N3）；9.1 的"不空过"里看流是安静的只有 C7（C-N4）；3.5 的 AS3 与 9.1 的 AS3 一行记下 AS3 (MCP) 不重查上传的类型（A-N1）；第 11 节给 M12 的 E2E 一步写量得的（审查 C），第 12 节的 Caddy 只有一次校验（C-N1）。

## 4. 接入文档：README 新的一节"接入 agent（MCP）"

00 号文档 4.16 的"接入文档"一条与 P1–P4 第 8 节给接入文档的各点（2.3 的处置表）落在 README 的一个新的二级标题里，放在"运行后端"之后、"接口与代码生成"之前（今天"运行后端"的最后一个三级标题就是"MCP"，`README.md:205-219`）。它不重复"MCP"一节的规则（工具、路径、`batch`、历史、找、索引与反链都已在那里，P1–P4 写的），只讲怎样接上、审批、上限、HTTP 的答复与几条写的规矩，并指回那一节。"MCP"一节的最后一行（`README.md:219`："Claude Code、Codex 的接入配置与反向代理的要求在 M9 的接入文档里写（M9/P5）。"）改为：`接上 Claude Code、Codex 见下一节"接入 agent（MCP）"；反向代理的要求见"部署"。`（建成的：README 指一节都用 ASCII 的双引号，3.9）

### 4.1 全文（README 建成的样子）

M9 收尾照代码改了这一节的几行（`741519d0`：`README.md:273`、`:327`、`:341`、`:346`、`:347`、`:368`、`:369`，`:339-351` 的表随之重新对齐；[收尾审查](reviews/M9-closeout-review.md)），下面的全文照它改过，与 `f214cb77` 的 `README.md:221-371` 逐字节相同；下一段说的是 P5 建成时的样子（`053d3ba1`）。

下面四个反引号之间的是 README 里这一节建成的样子（`053d3ba1`）：S1 照这一节原来的文字提交，oxfmt 在带续行的列表项之间加了空行、对齐了两张表，修复一轮改了"写的规矩"的拒绝与"HTTP 的答复"的 `MCP-Protocol-Version` 的两行（3.9）；之后的 4.2 逐句列出客户端的行为：每一句在样例里、标着"照文档"或"未核对"，或由仓库里的记录核对过，以及人工清单的哪一步核对它、核对之后怎样改。

````markdown
## 接入 agent（MCP）

Claude Code、Codex 一类的 agent 经 `/mcp` 读写笔记本。工具、路径与它们的规则见上文"MCP"一节；这一节讲怎样接上、审批怎样设、有哪些上限，以及出错时看到什么。下面的样例照两个客户端的文档与 M0/P1 的实验（Claude Code 2.1.285、Codex 0.157.1）写成，还没有在真实的客户端上照[人工清单](docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md)走过；清单在 v0.1 的最后走一遍，之后这里写明核对过的客户端的版本，并照它改正标出的几句：标着"照文档"的照客户端自己的文档写，标着"未核对"的是推断。

### 先准备

- **一个 agent 一个 PAT**：在"设置"→"访问令牌"（`/settings/tokens`）创建，名称写明给哪个 agent、在哪台机器上（如 `claude-code-laptop`），最好设一个期限；也可以 `POST /api/v0/me/api-tokens`（上文"账户与认证"）。令牌只显示这一次。PAT 能做这个账户能做的一切：笔记本的地址只是"进到这本笔记本"，不是权限的边界。一个 agent 一个令牌，撤销它只切断这一个 agent；agent 的变更集记为 `mcp:<客户端报的名称>`（网页的历史显示"MCP（名称）"），名称记在令牌上，两个客户端共用一个令牌时记的是最后报名称的那个。
- **令牌放在环境变量里**，一个 agent 一个变量：下面的样例里 Claude Code 用 `NERVEWIKI_CLAUDE_TOKEN`、Codex 用 `NERVEWIKI_CODEX_TOKEN`。同一个变量就是同一个 PAT：两个 agent 的写在历史里分不清，撤销它两个一起断。在终端里设时用 `read -rs NERVEWIKI_CLAUDE_TOKEN && export NERVEWIKI_CLAUDE_TOKEN`（粘贴令牌、回车：不回显，也不进 shell 的历史）；长期用的写在只有自己读得到的 shell 配置里，或由密钥管理器给出。不要把令牌写进会提交的文件。
- **只经 HTTPS 连 `/mcp`**（本机试用除外）：每个请求的头里都是 PAT。
- **实例设好 `server.public_url`**（`NWIKI_SERVER__PUBLIC_URL`，只到源，如 `https://wiki.example.com`）：否则 `read` 读附件时只给图片，别的附件没有下载的地址。
- **两种地址**（`wiki.example.com` 换成实例对外的地址）：
  - 笔记本入口 `https://wiki.example.com/mcp?notebook=<笔记本 id>`：笔记本的 id 是网页地址里 `/notebooks/` 之后的那一段。握手时下发这本笔记本的 `AGENTS` 页（上文"MCP"的"说明"）。一个 agent 在一本笔记本里工作时用它。
  - 账户入口 `https://wiki.example.com/mcp`：agent 先以 `notebooks` 看有哪些笔记本，再以 `open_notebook` 读一本的说明与概况，之后每个工具都带 `notebook`。
  - 地址里有 `?`：写在 shell 里时加引号（zsh 把 `?` 当作通配）。

### Claude Code

- **只给自己**（默认的本地范围：服务与令牌存在 `~/.claude.json`，只在当前的目录里、只有自己用；照 Claude Code 的文档）：

  ```sh
  claude mcp add --transport http nervewiki "https://wiki.example.com/mcp?notebook=<笔记本 id>" \
    --header "Authorization: Bearer $NERVEWIKI_CLAUDE_TOKEN"
  ```

  shell 先展开 `$NERVEWIKI_CLAUDE_TOKEN`，存下的是令牌本身；加 `--scope user` 则在每个目录里都有它（照 Claude Code 的文档）。

- **随项目共享**（项目的范围：项目根目录的 `.mcp.json`，它通常会提交）：`.mcp.json` 里**绝不要写令牌**，写环境变量的展开（照 Claude Code 的文档），每个人在自己的环境里设 `NERVEWIKI_CLAUDE_TOKEN`：

  ```json
  {
    "mcpServers": {
      "nervewiki": {
        "type": "http",
        "url": "https://wiki.example.com/mcp?notebook=<笔记本 id>",
        "headers": {
          "Authorization": "Bearer ${NERVEWIKI_CLAUDE_TOKEN}"
        }
      }
    }
  }
  ```

  用命令写它时加 `--scope project`，令牌那一段用**单引号**，让 `${NERVEWIKI_CLAUDE_TOKEN}` 原样进文件：

  ```sh
  claude mcp add --scope project --transport http nervewiki "https://wiki.example.com/mcp?notebook=<笔记本 id>" \
    --header 'Authorization: Bearer ${NERVEWIKI_CLAUDE_TOKEN}'
  ```

  写好之后 `grep nwk_pat_ .mcp.json` 应当什么也找不到。项目里的服务第一次用时，Claude Code 先问要不要启用它（照 Claude Code 的文档）。

- **看连上没有**：`claude mcp list`，或在会话里输入 `/mcp`：`nervewiki` 已连上，笔记本入口有 14 个工具（账户入口 16 个）。工具在 Claude Code 里叫 `mcp__nervewiki__read` 一类（照 Claude Code 的文档）。
- Claude Code 把一次 MCP 工具的结果限在约 25,000 个 token，环境变量 `MAX_MCP_OUTPUT_TOKENS` 可以调（照 Claude Code 的文档）；服务端的每个答复的文字至多 48 KiB（`read` 直接放进答复的图片另算，至多 3 MiB），`read` 一次至多约 45 KiB 的正文，中文约 1.5 万字。

### Codex

- 写进 `~/.codex/config.toml`（设了 `CODEX_HOME` 时是 `$CODEX_HOME/config.toml`；照 Codex 的文档）：

  ```toml
  [mcp_servers.nervewiki]
  url = "https://wiki.example.com/mcp?notebook=<笔记本 id>"
  bearer_token_env_var = "NERVEWIKI_CODEX_TOKEN"
  default_tools_approval_mode = "prompt"
  ```

  文件里没有令牌：Codex 启动时从 `NERVEWIKI_CODEX_TOKEN` 读它，所以启动 Codex 的 shell 里要有这个变量。`default_tools_approval_mode` 见下面的"审批"。

- **看连上没有**：在会话里输入 `/mcp`，列出 `nervewiki` 与它的工具（照 Codex 的文档）。

### 审批

工具带着 MCP 的注解：读的九个（`list`、`read`、`grep`、`search`、`index`、`backlinks`、`versions`、`log`、`changeset`；账户入口另有 `notebooks`、`open_notebook`）是 `readOnlyHint`；写的五个（`write`、`edit`、`move`、`delete`、`batch`）是 `destructiveHint`，其中 `write`、`move`、`delete` 另是 `idempotentHint`（同样的参数再调一次不再改变什么）。调用之前问不问人由客户端与它的设置定；两个客户端是否照注解区别对待未核对。

- **不要一律放行**：放行了写的工具，agent 不经人确认就能以这个账户的权限改写、删除、移动它看得到的每一本笔记本里的页。读的工具可以免问；写的工具照常问，除非这个 agent 要无人值守地工作，而你接受事后在网页的历史里检查、撤销它做的（下面的"历史只读"）。审批只管 MCP 的工具：agent 能执行命令时，它启动的命令照常继承启动它的环境（这是 shell 的），里面就有令牌的变量，可以直接调 REST；客户端是否把自己的环境交给它执行的命令（或滤掉一些变量）未核对，当作交了。
- **Claude Code**：一个工具在调用之前问；回答"不再问"之后这个工具不再问（照 Claude Code 的文档）。要事先放行读的工具，写进设置（项目的 `.claude/settings.json` 或自己的 `~/.claude/settings.json`）的 `permissions.allow`：

  ```json
  {
    "permissions": {
      "allow": [
        "mcp__nervewiki__list",
        "mcp__nervewiki__read",
        "mcp__nervewiki__grep",
        "mcp__nervewiki__search",
        "mcp__nervewiki__index",
        "mcp__nervewiki__backlinks",
        "mcp__nervewiki__versions",
        "mcp__nervewiki__log",
        "mcp__nervewiki__changeset"
      ]
    }
  }
  ```

  账户入口另加 `mcp__nervewiki__notebooks` 与 `mcp__nervewiki__open_notebook`。`claude -p`（不交互）问不了人，没有放行的工具不调用：以 `--allowedTools` 给它这次要的工具，写的工具只给确实要写的任务（照 Claude Code 的文档）。

- **Codex**：一个服务的 `default_tools_approval_mode` 决定调用它的工具之前问不问人，取值是 `auto`、`prompt`、`writes` 与 `approve`；`approve` 是都不问。`codex exec`（不交互）问不了人，要审批的调用都被拒。交互的会话用 `prompt`；只读的无人值守的任务不必放行写的工具；`approve` 只给确实要无人值守地写的任务，并照上面的第一条接受它的代价。

### 上限

| 什么            | 上限                                                                                                                                                                                                   |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 一次调用        | 15 秒（`server.request_timeout`，从读完请求体算起）；请求体至多 37,814,272 字节（约 36 MiB），超过 64 KiB 的只能是这个入口的工具的调用；请求体里至多 10,000 个 JSON 值                                 |
| 每个工具的答复  | 48 KiB：到了就停，给出 `next_cursor`（或 `next_offset`、`next_column`）接着调，没有游标的标出截断                                                                                                      |
| `read`          | 一次至多约 45 KiB 的正文（中文约 1.5 万字）；长的一行经 `column` 分几次读完。图片（PNG、JPEG、GIF、WebP）至多 3 MiB、每边 8,000 像素时直接放进答复，别的附件给下载的地址（要配置 `server.public_url`） |
| `write`、`edit` | 正文至多 5 MiB；`edit` 的 `old_string` 至多 1 MiB                                                                                                                                                      |
| `batch`         | 至多 100 项，字符串合计至多 6 MiB、正文与 `new_string` 合计至多 5 MiB，别的界线见上文"MCP"的"batch"                                                                                                    |
| 说明            | 见上文"MCP"的"说明"                                                                                                                                                                                    |
| 限流            | 一个 POST 算一次请求，与这个令牌的 REST 请求共用一个桶（`ratelimit.authenticated`）                                                                                                                    |

### 大的请求体的名额

超过 64 KiB 的调用（大的 `write`、`edit`、`batch`）在整个服务里同时至多两个、每个令牌至多一个；取不到名额的等 2 秒，然后答 503 `server_busy`（`Retry-After: 1`），稍后重发即可。所以一个 agent 的大的写一个接一个地发（内置的说明也这样告诉 agent）。名额从读请求体之前占到这次调用答完：慢的上行占得久（1 Mbit/s 上过约 250 KB 就过 2 秒），这时同一个令牌同时发出的两个大的写，第二个多半拿到 503。反向代理照"部署"一节为 `/mcp` 缓冲请求体时，请求在代理收完之后才到服务，名额只占服务端读请求体与做这次调用的时间，不再随上行的快慢。

### HTTP 的答复

这些是 HTTP 一层的答复，在任何工具运行之前：它们不是工具的结果，客户端把它们当作连接或传输的错误（官方 TypeScript SDK 的客户端是这样）；Claude Code、Codex 怎样显示、模型看不看得到未核对。工具自己的错误不在其中：它们是带 `isError` 的结果，模型读得到码与说明（`page.locked`、`page.revision_mismatch` 一类），照说明改了再调。

| 答复                                                                                                                  | 什么时候                                                                                                                                                                                                                                                                                                                                                                                                                                                               | 怎么办                                                                                                                                                                             |
| --------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 401 `unauthorized`，"This server has no OAuth: MCP takes a personal access token in the Authorization header."        | 没有带令牌，或带的 `Authorization` 不是 `Bearer <令牌>`（`Bearer` 之后是空的、别的方案、令牌里有空白）                                                                                                                                                                                                                                                                                                                                                                 | 在客户端的配置里加 `Authorization: Bearer <PAT>`（下一段）；已经加了的，看令牌的变量在启动客户端（或执行 `claude mcp add`）的 shell 里设了没有（变量没有设时客户端发出什么未核对） |
| 401 `unauthorized`，"The bearer token is invalid or has expired."（`WWW-Authenticate: Bearer error="invalid_token"`） | 令牌写错了、过期了或被撤销了                                                                                                                                                                                                                                                                                                                                                                                                                                           | 新建一个 PAT，换上                                                                                                                                                                 |
| 403 `forbidden`，"MCP takes a personal access token, not a session's access token."                                   | 带的是网页登录的访问令牌                                                                                                                                                                                                                                                                                                                                                                                                                                               | 换成 PAT                                                                                                                                                                           |
| 404 `notebook.not_found`                                                                                              | 地址里的笔记本 id 不对、为空、出现两次，或你看不到这本笔记本、它已被删除                                                                                                                                                                                                                                                                                                                                                                                               | 改正 id；或用账户入口的 `notebooks` 看有哪些                                                                                                                                       |
| 404 `not_found`                                                                                                       | 地址是 `/mcp/` 之下的路径                                                                                                                                                                                                                                                                                                                                                                                                                                              | 地址是 `/mcp` 本身                                                                                                                                                                 |
| 400 `bad_request`                                                                                                     | 一次发了 JSON-RPC 的批、请求体不是一个合法的 JSON 对象（空的请求体见下一行），或 JSON 值过了 10,000 个                                                                                                                                                                                                                                                                                                                                                                 | 一条消息一个请求                                                                                                                                                                   |
| 400、415（纯文字，不是上面的 JSON）                                                                                   | 请求头不对：`Accept` 不是同时认 `application/json` 与 `text/event-stream`（`application/*`、`text/*` 与 `*/*` 也算，所以 `curl` 默认的 `*/*` 可以）、`Content-Type` 不是 `application/json`，或 `MCP-Protocol-Version` 是服务端不说的、排在 `2026-07-28` 之前的版本（服务端说 `2025-11-25`、`2025-06-18`、`2025-03-26`、`2024-11-05`；不带这个头也行）；或请求体不是 SDK 认得的请求（空的、方法名写错、少了 `id`、`jsonrpc` 不是 `"2.0"`、`tools/call` 没有 `params`） | 照下面的 `curl` 写这几个头与请求体                                                                                                                                                 |
| 400（JSON-RPC 的错误，`application/json`）                                                                            | `MCP-Protocol-Version` 是 `2026-07-28` 或更新的版本：它们过得了上一行的检查，SDK 再以 JSON-RPC 的错误拒绝（缺那个版本要的 `_meta` 时 `-32602`，带了时 `-32022` `unsupported protocol version`，列出服务端说的版本）                                                                                                                                                                                                                                                    | 用服务端说的版本，如下面的 `curl` 的 `2025-11-25`                                                                                                                                  |
| 413 `payload_too_large`                                                                                               | 请求体过了约 36 MiB，或超过 64 KiB 而不是工具的调用                                                                                                                                                                                                                                                                                                                                                                                                                    | 拆小：一页的正文至多 5 MiB                                                                                                                                                         |
| 429 `rate_limited`                                                                                                    | 这个令牌的请求过了限流；或同一个客户端 IP 上认证失败的令牌太多（失败闸门 `auth_failure`，按 IP：反向代理之后要设 `server.trusted_proxies`，否则所有人共用代理的 IP）                                                                                                                                                                                                                                                                                                   | 等 `Retry-After` 的秒数；后一种先换掉失败的令牌（上面 401 的第二行）                                                                                                               |
| 503 `server_busy`（`Retry-After: 1`）                                                                                 | 大的请求体没有名额（上一段）                                                                                                                                                                                                                                                                                                                                                                                                                                           | 稍后重发，大的写一个接一个地发                                                                                                                                                     |

**没有配置令牌时**：客户端看到的都是同一句："This server has no OAuth: MCP takes a personal access token in the Authorization header."。不走 OAuth 的客户端从 401 的说明里读到它；会走 OAuth 的客户端（例如官方 TypeScript SDK 的 OAuth 客户端）先找 OAuth 的元数据（`/.well-known/` 之下都答 404）、再试注册（`POST /register` 答 OAuth 的错误 `invalid_client_metadata`），从注册的错误里读到它。Claude Code、Codex 各走哪一条未核对。照它在配置里加上 `Authorization: Bearer <PAT>` 即可。

**自己试一下**（不握手，不改这个令牌记下的客户端名称）：

```sh
curl -sS "https://wiki.example.com/mcp?notebook=<笔记本 id>" \
  -H "Authorization: Bearer $NERVEWIKI_CLAUDE_TOKEN" -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2025-11-25' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
```

（只给 Codex 设了令牌的，换成 `$NERVEWIKI_CODEX_TOKEN`。）答 200 与工具的清单就是通的；别的答复照上表。

### 写的规矩

- **`base_version`**：`list`、`read`、`write`、`edit` 的答复里有页的 `version`（`"<节点 id>:<版本号>"`），它说出读到的是哪一版。覆盖一页用 `write` 带读到的 `version` 作 `base_version`；`edit` 不带它时在现在的正文上做（做的期间被写过就在新的正文上再做一次，再做的期间又被写过就拒绝），带了的在那之后被写过就拒绝。拒绝是工具的错误 `page.revision_mismatch`（带 `isError` 的结果，不是 HTTP 的答复），`current_version` 是现在的版本：重读再写。同一个路径换成了另一页（原来的被改名、移走之后有人新建了同名的页）时同样拒绝，不会写到新的页上。与这一页现在的内容相同的 `write` 什么也不写、答未改动（`changed: false`），带的 `base_version` 过时了也一样。
- **锁**：正在网页上编辑的页（包括你自己在别的浏览器里编辑的）拒绝 agent 的写（与页现在的内容相同的 `write` 除外：它什么也不写）、编辑与删除，答 `page.locked`，说出谁在编辑；`move` 要改写链接的页正被编辑时答 `linking.pages_locked`。agent 不能接管锁：等编辑的人离开编辑（关掉浏览器之后锁至多约 2 分钟过期），或请笔记本的管理员强制解锁。agent 自己不开编辑会话，不挡网页上的人。
- **历史只读**：agent 读得到历史（`log`、`changeset`、`versions`），改不了它：还原版本、从回收站恢复、撤销变更集是人在网页上做的。agent 的每次写是一个变更集，一次 `batch` 也是一个（带它的说明）；网页的历史可以按"MCP"筛选，一次移动加改名在网页上一步撤销，一个 `batch` 整组撤销。
- `batch` 与分开调用只有一条规则不同、`grep` 与 `search` 怎样分工与怎样读 `incomplete`、`partial_lines`、`building`：见上文"MCP"的"batch"与"找"。
````

### 4.2 客户端的每一句与核对它的清单的步

4.16 说"样例出自人工清单的那一遍"，而清单由负责人在 v0.1 的最后执行（第 11 节的取舍第 12 条；不先跑一遍的理由见 2.1 第 10 条）：P5 合并时 README 里是上面的文字（M9 收尾改的几行除外，4.1 的开头），样例照两个客户端的文档与 M0/P1 的实验（Claude Code 2.1.285、Codex 0.157.1，[01-P1-spikes](../M0-foundation/01-P1-spikes.md)第 7 节⑤，`01-P1-spikes.md:318-319`）写。规则只有一条：README 与清单里每一句客户端的行为，在样例里、标着"照文档"或"未核对"，或由仓库里的记录核对过（M0/P1 的实验、P1 的 S1 的 TypeScript 客户端的试用），下表每一句一行。清单的每一步在"记下的"一栏里有它要核对的那一句（第 6 节）；那一遍之后，由负责人或主持者照记下的改 README，改的提交写明照清单的哪一步。

| README 的那一句 | 现在怎样写（P5 合并时） | 清单 | 核对之后 |
|---|---|---|---|
| 开头的"还没有在真实的客户端上照人工清单走过""照文档""未核对"的说明 | 写"还没有走过"（B-I1） | 整份清单 | 写明走过的日期与 `claude --version`、`codex --version`；没有标记留下时去掉说明标记的那半句 |
| Claude Code 与 Codex 的样例（两个 `claude mcp add`、`.mcp.json`、`config.toml`） | 样例；`${NERVEWIKI_CLAUDE_TOKEN}` 的展开照 Claude Code 的文档；`bearer_token_env_var` 是 M0/P1 记下的 | 第 A1、A3、A4、B1、B4 步 | 照实改；`--scope project` 的命令若把展开后的令牌写进文件，删掉它的代码块，只给手写的 `.mcp.json` |
| "默认的本地范围：服务与令牌存在 `~/.claude.json`，只在当前的目录里"与"加 `--scope user` 则在每个目录里都有它" | 照 Claude Code 的文档 | 清单不核对：会写负责人自己的 `~/.claude.json` | 留作文档上的说法 |
| "项目里的服务第一次用时，Claude Code 先问要不要启用它" | 照 Claude Code 的文档 | 清单测不到：A3 以 `--mcp-config … --strict-mcp-config` 载入，不走项目的服务的路；以项目的服务载入会连带起负责人别的服务 | 留作文档上的说法，或删掉 |
| "看连上没有"（`claude mcp list`、`/mcp`；Codex 的 `/mcp`）与"工具在 Claude Code 里叫 `mcp__nervewiki__read` 一类" | 照两个客户端的文档 | 第 A1、B1 步（只核对 `/mcp`：清单不执行 `claude mcp list`，它会连负责人配置过的每个服务）；工具名由第 D2 步的 `permissions.allow` 核对 | 照记下的命令、显示与工具名改 |
| Claude Code 的约 25,000 个 token、`MAX_MCP_OUTPUT_TOKENS` 与 48 KiB 的 `read`（中文约 1.5 万字） | 照 Claude Code 的文档（25,000 也是 00 号文档 4.9、4.16 的数）；只写数与中文的字数，不写"放得下"（00 号文档 4.9 说由清单核对） | 第 C7 步（`grep` 的 48 KiB 另见 C11） | 记下的 48 KiB 的 `read`（中文）放得下时加"放得下"；放不下时写明在哪里截、`MAX_MCP_OUTPUT_TOKENS` 要调到多少，并把 48 KiB 交 M12（"写给后面的 M"的上限一条） |
| Codex 的配置文件在 `~/.codex/config.toml` 或 `$CODEX_HOME/config.toml` | 照 Codex 的文档 | 第 B1 步（清单以临时的 `CODEX_HOME` 起 Codex，它读到那里的 `config.toml` 就核对了后一半） | 照实改 |
| 审批："调用之前问不问人由客户端与它的设置定；两个客户端是否照注解区别对待未核对" | 未核对；不说它们是否看 `readOnlyHint` | 第 D1–D8 步 | 照记下的，写每个客户端对读的与写的工具问不问 |
| 审批：Claude Code 的提问与"不再问"、`permissions.allow` 与两个设置文件、`claude -p` 不调没放行的工具与 `--allowedTools` | 照 Claude Code 的文档 | 第 D1–D3 步 | 记下的是 Claude Code 对 `readOnlyHint` 的工具不问时，加一句"读的工具不问"，`permissions.allow` 留作非交互的写法；`-p` 与 `--allowedTools` 照 D3 记下的改 |
| 审批：Codex 的四个取值、`approve` 都不问、`codex exec` 拒绝要审批的调用 | M0/P1 记下的（`01-P1-spikes.md:319`）；不写 `auto`、`prompt`、`writes` 各问什么 | 第 D4–D8 步 | 照记下的，每个取值写一句它对读的与写的工具问不问，推荐交互的会话用哪一个；样例的 `default_tools_approval_mode = "prompt"` 照推荐的改 |
| HTTP 的答复："客户端把它们当作连接或传输的错误（官方 TypeScript SDK 的客户端是这样）；Claude Code、Codex 怎样显示、模型看不看得到未核对" | 前半由 P1 的试用核对（[01-P1](01-P1-platform-reads.md) 9.1 第 4a、5 步：`connect` 抛出 `StreamableHTTPError`）；后半未核对。00 号文档 4.4 的"Claude Code 显示'HTTP 429 …'"没有核对过，不照抄 | 第 E1–E5 步 | 加一列"Claude Code 显示""Codex 显示"，照记下的原文（截到一行）；去掉"未核对"的半句 |
| "HTTP 的答复"一表没有令牌的 401 的"怎么办"："看令牌的变量在启动客户端（或执行 `claude mcp add`）的 shell 里设了没有（变量没有设时客户端发出什么未核对）"（`f214cb77` 的 `README.md:341`，M9 收尾时改的，`741519d0`） | 收尾时照代码改（[收尾审查](reviews/M9-closeout-review.md) B-M4）：`Authorization` 不是 `Bearer <令牌>` 的也答这个 401（`platform/httpserver/api.go:396-403` 的 `bearerToken`）；看变量是 shell 的事（样例之后已说 shell 先展开它，`f214cb77` 的 `README.md:245`）；变量没有设时客户端怎样（Claude Code 对 `.mcp.json` 里的 `${…}` 报错、留空还是照原样发出，Codex 对 `bearer_token_env_var` 不起这个服务还是不带令牌连接）未核对 | 第 A4、B4 步，E1 在旁（不带令牌时客户端显示什么） | 照 A4、B4 记下的写每个客户端在变量没有设时怎样（报错、不起这个服务，或不带令牌连接、得到 401），去掉"未核对"的半句；E1 记下的显示照"没有配置令牌时"一行改 |
| "没有配置令牌时"一段 | P1 的写法（[01-P1](01-P1-platform-reads.md)第 8 节 P5 的那一段），"例如 Claude Code"换成 P1 的试用实际跑过的"官方 TypeScript SDK 的 OAuth 客户端"（9.1 第 4a、4b 步）；"Claude Code、Codex 各走哪一条未核对" | 第 E1、E2 步 | 写明两个客户端各走哪一条、显示什么（Codex 若退到源站的 `/authorize`、`/token`，写明它显示什么）；两种客户端都显示了那一句时去掉"未核对"的一句 |
| 大的请求体的名额："同一个令牌同时发出的两个大的写，第二个多半拿到 503""内置的说明也这样告诉 agent" | 照服务端的规则（P2 的 3.2）与内置的说明（`page/adapter/mcp/instructions.go:73-74`）；客户端会不会同时发两个写不写 | 第 F1–F3 步 | 照记下的写客户端遇到 503 时怎样（重试、报错、断开）、它是否同时发两个大的写、agent 是否照说明一个接一个地重发 |
| 审批："agent 能执行命令时，它启动的命令照常继承启动它的环境……客户端是否把自己的环境交给它执行的命令未核对" | 继承环境是 shell 的事；客户端的命令工具交不交、滤不滤未核对，照最坏的写（当作交了） | 清单不核对：核对要让 agent 读出令牌 | 留作提醒，不改 |
| "MCP"一节的"服务重启之后客户端照常调用，不用重新连接"（`README.md:209`，P1 写的） | 照官方的 Go 与 TypeScript 客户端（00 号文档 4.3 的整个程序的测试、P1 的 9.1 第 3 步） | 第 G1、G2 步 | 某个客户端要重新连接或报错时，在那一句之后写明是哪一个、怎样恢复 |
| "MCP"一节"传输"的 503："这是 HTTP 的答复，不是工具的结果（官方 TypeScript SDK 的客户端把它当作传输的错误；Claude Code、Codex 怎样显示未核对，见下文'HTTP 的答复'）"（`f214cb77` 的 `README.md:209`，M9 收尾时改的，`741519d0`） | 前半与"HTTP 的答复"一行的前半同一个依据（P1 的试用）；后半未核对（[收尾审查](reviews/M9-closeout-review.md) B-I1，表下一段） | 第 F1 步 | 照 F1 记下的写 Claude Code、Codex 遇到 503 时怎样（重试、把错误交给模型、断开），去掉"未核对"的半句；与"大的请求体的名额"一行一起改 |
| "部署"一节 Caddy 样例之后的"agent 看不到 `Retry-After`（客户端怎样对待 502 未核对）" | 经 Caddy 的 502 没有 `Retry-After` 是服务端一侧的，替身量过（5.2）；客户端怎样对待 502 未核对，不写它重试、报错还是断开。同一句的网页的一半是我们自己的代码（`web/apps/web/src/stores/page-editing.ts:336`） | 第 I3 步记下经 Caddy 的被拒的写显示什么（状态码与头，不是客户端）；客户端对 502 的做法清单不核对 | 留作提醒；I3 记下的状态码与头与 5.2 不同时照它改那一句 |

这样 P5 合并时，README 里客户端的行为只出现在样例与标明"照文档""未核对"的句子里，或是仓库的记录核对过的，每一句在上表有它的一行；样例在清单的第一部分就被照原样用一遍（第 6 节的"准备"），所以"接入文档的样例与清单一致"（00 号文档第 7 节的 P5 一行的验证）在 P5 里由构造成立：清单不另写一份样例，只引 README 的、把 `<笔记本 id>` 与变量换成清单里的。同一条规则也管清单：清单的"预期"里客户端的行为同样标"照文档"或"未核对"（第 6 节的开头）。M9 收尾找到漏了的一句（[收尾审查](reviews/M9-closeout-review.md) B-I1）：`README.md:209` 原说客户端把 503 显示成传输的错误，没有标记；收尾时它改为标出"未核对"，与收尾改 `:341`（`f214cb77` 的行号）时新标的一句各在上表有一行。

### 4.3 为什么这样写

- **放在哪里**：不并进"MCP"一节：那一节是给写 agent、读规则的人的（每个工具的语义与界线），这一节是给接上它的人的（配置、审批、出错），读的人不同；"MCP"一节已有约 3,600 个汉字，再加就难找。放在"运行后端"之后、紧接"MCP"一节，两节互相指。
- **在清单走过之前怎样写**：开头一句照实说还没有走过（不写"走过一遍"：P5 合并到 v0.1 的最后之间，读的人会把样例当作测过的，其中就有写错了会把 PAT 写进提交的文件的 `--scope project` 的命令）；客户端的每一句带它的来处，清单走过之后照 4.2 逐行改。标记只有"照文档"与"未核对"两种：仓库里的记录核对过的（M0/P1 的实验、P1 的试用）不标，它们在 4.2 里有出处。
- **令牌只出现在本地的范围**：4.16 要"`.mcp.json` 通常会提交，绝不要把 PAT 写进去"。`claude mcp add` 在 shell 里先展开 `$NERVEWIKI_CLAUDE_TOKEN`，项目的范围下这样写就把令牌写进了 `.mcp.json`：所以项目的范围的命令是它自己的代码块、单引号，并给一个 `grep` 自查（照着本地范围的那一块改、漏了 `--scope project` 的人，会把带着令牌的服务写进自己的 `~/.claude.json`）。Codex 的样例本来就只写变量名（`bearer_token_env_var`）。
- **一个客户端一个变量**：一个 `NERVEWIKI_TOKEN` 照着样例写进 `~/.zshrc`，两个 agent 就共用一个 PAT；无状态的调用不带 `clientInfo`（00 号文档 4.3），记下的名称是最后握手的那个（4.5），Claude Code 的写会在历史里显示成"MCP（codex-mcp-client）"。审批一节靠的正是"在网页的历史里检查"。
- **不握手的 `curl`**：握手的 `initialize` 会把 `clientInfo.name` 记在令牌上（00 号文档 4.5），以 agent 的令牌试一次会把它的名称改成 `curl`，直到 agent 下一次握手。`tools/list` 不带 `clientInfo`，无状态的服务端不要先握手（P2 的 `handshake: false` 的客户端就是这样调用的，`e2e/fixtures/mcp.ts:110-137`），所以它只看令牌与地址。
- **审批的推荐**：4.16 与总体设计 6.3 要"不一律推荐 `approve`"。这一节给出代价（写的工具以账户的权限）与缓解（每次写是一个变更集，人在网页上撤销），读的工具免问、写的工具照常问；无人值守的写是负责人自己的选择。审批只管 MCP 的工具一句：能执行命令的 agent 启动的命令继承它的环境（shell 的规则），客户端交不交给命令工具未核对，所以照最坏的写；它在 4.2 有一行。
- **上限一表**只列 agent 会碰到的；`batch`、`grep` 与说明的细的界线已在"MCP"一节，这里指过去。数都出自代码：15 秒是 `server/configs/config.yaml:17`，37,814,272 是 `bootstrap/mcp.go:29` 的 `6*(page.MaxContentBytes+1<<20) + 64<<10`，10,000 是 `mcpserver.MaxBodyValues`，48 KiB 是 `MaxAnswerBytes`（00 号文档 4.14）。`read` 的"约 45 KiB"是内置的说明自己的数（`page/adapter/mcp/instructions.go:45`）；按代码，正文的地方是 48 KiB 减去最宽的第一块（`page/adapter/mcp/read.go:60-92`），约 48.7 KB，一个汉字 3 字节、一个换行在 JSON 里 2 字节（`mcpserver/answer.go:25-60`），所以中文的散文约 1.5 万字；清单的 C7 用同一个数。
- **HTTP 的答复一表**的文字出自代码：没有令牌的 401 的说明是 `mcpserver.NoOAuth`（`platform/httpserver/mcpserver/edge.go:147`），坏的令牌的是平台的 `unauthorized`（`platform/httpserver/api.go:378-383`），403 的是 `forbidden`（`edge.go:193-195`），`/mcp/` 之下的 404 是 `Subtree`（`edge.go:132-140`），`/register` 的 400 是 `Register`（`edge.go:175-181`）；纯文字的 400、415 是 SDK 的（`modelcontextprotocol/go-sdk` v1.8.0 的 `mcp/streamable.go:347-398`，00 号文档 4.4）；`MCP-Protocol-Version` 按版本分两行（S1 的修复一轮，审查 B-N4）：不带这个头或服务端说的四个版本照常，别的、排在 `2026-07-28` 之前的由 `streamable.go:354` 答纯文字的 400，`2026-07-28` 起的过得了那一检查、再由 SDK 以 JSON-RPC 的错误答 400（缺那个版本的 `_meta` 时 `-32602`，`mcp/shared.go:577` 的检查；带了时服务端答 `-32022` `unsupported protocol version`），实施者以一次不提交的探针试过十一种版本；429 的失败闸门在认证之前按 IP 取（`platform/httpserver/api.go:346-350`，`server/configs/config.yaml:74`，README 的"限流"）。405 不列：操作的人只在日志里见到它，也不必做什么。
- **工具的拒绝写码，不写 HTTP 的状态**（S1 的修复一轮，审查 C-N7）：工具的拒绝是 HTTP 的 200 与一个带 `isError` 的结果，文字是"<码>: <说明>"，`structuredContent.error` 带码（`page/adapter/mcp/text.go:22-43`、`mcpserver/errors.go:84-143`），没有状态；所以"写的规矩"写"工具的错误 `page.revision_mismatch`"，P1–P4 写的"MCP"一节的八处同样去掉了 409、422、503（3.9）。写状态的只有 HTTP 一层的答复（上一条的表）。

## 5. README 的部署一节：反向代理的改正

00 号文档 4.16 的"反向代理"一条（D10）与 P2 第 8 节的"部署建议按路由缓冲请求体"（2.3 的处置表）落在 README 的"部署"一节。今天那里关于反向代理的是四条（`README.md:342-367`）：`trusted_proxies`（`:342-343`，不改）、事件流与 Caddy、nginx 的样例（`:344-363`）、上传（`:365-367`）；内存一条（`:368`）不改。要改正的是：

1. **nginx 的样例只有一个 `location /`**（`:353-363`），上传一条又说"反向代理不要缓冲上传的请求体：nginx 的 `proxy_request_buffering off`"（`:366`）：照它在 `location /` 里关掉缓冲，`/mcp` 与页面正文的两个写也就不缓冲了，慢的发送者占得住大的请求体的名额（00 号文档第 10 节的那一行；P2 的设计审查 MN-2）；照样例不关，上传又被缓冲了。所以样例改为一个按路由的 `server`：上传与导入不缓冲，`/mcp` 与两个正文路由缓冲，各有自己的 `client_max_body_size`。
2. **`client_max_body_size`**（`:367`）只说"不小于导入包的上限"：`/mcp` 的上限（37,814,272 字节，`bootstrap/mcp.go:29`）与两个正文路由的（31,522,816 字节，`page/adapter/http/limits.go:8`）都比默认的 1 MB 大；导入的路由收的是 zip 加 64 KiB 的表单（`transfer/adapter/http/start_import.go:24` 的 `Envelope`，`handler.go:80`），512m 差 64 KiB。按路由写，缓冲的路由的上限就是服务端的上限：一个更大的上限只会让代理先把超过服务端上限的请求体收下，再由服务端答 413。
3. **`/mcp` 不是 SSE**：`X-Accel-Buffering: no` 只在事件流的答复上（`:344`），`/mcp` 是普通的 JSON 答复，不持住连接（00 号文档 4.3 的"不持住答复"）；读超时不短于 45 秒（`server.request_timeout` 15 秒加 `server.read_timeout` 30 秒，00 号文档 4.4 的"请求体与期限"），nginx 默认的 60 秒够；`Host` 照常转发（回环保护已关，`platform/httpserver/mcpserver/server.go:132`）。
4. **Caddy**：默认不缓冲请求体（P2 的设计审查 MN-2 的处置）；它的 `request_buffers` 能否只为一些路由缓冲，P2 交 P5"核对过才写"（P2 第 8 节）。这一条现在就定了，不等人工清单：`caddy:2.10-alpine`（Caddy 2.10.2）在本机的镜像里（M5/P2 用过），不必下载；P5 的设计审查 B 在隔离的容器里（`--rm --pull=never --network none`，没有映射端口）以下面的写法跑过，P5 修订时照它的脚本重跑一遍（`exp/caddy/Caddyfile`、`run.sh`、`run-out.txt`），另量了缓冲在内存里还是磁盘上（`exp/caddy/mem.sh`、`mem-out.txt`），并核对了 README 的样例本身（`exp/caddy/readme/`）。审查 B 的第二轮另量了代理与上游分开时的内存（`exp/caddy/mem-separate/`）与经 Caddy 时服务端早答的拒绝（`exp/caddy/early/`），第三轮又量了 nginx 每个大小三次、替身答 401 与 429 时的 Caddy（`exp/caddy/early2/`）。结果在 5.2 的 Caddy 几条。人工清单的 I3 由"核对写法"改为在负责人的部署上确认（第 6 节）。

### 5.1 全文（替换 `README.md:344-367`，并加一条升级的说明）

M9 收尾照代码改了其中两行（`741519d0`：`README.md:500` 的"`server` 里的三行 `proxy_set_header`"，`:571` 的日志只在有请求被拒时记；[收尾审查](reviews/M9-closeout-review.md)），下面的四条照它改过，与 `f214cb77` 的 `README.md:498-571` 逐字节相同。

`:342-343` 的 `trusted_proxies` 与搜索的词在访问日志里两条之后，换成下面的四条（事件流、样例、上传、MCP），`:368` 的内存一条照旧接在后面：

````markdown
- 反向代理不能缓冲事件流，读超时要长于心跳间隔（`events.heartbeat_interval`，默认 20 秒）。服务端在事件流的答复上设了 `X-Accel-Buffering: no`；只有事件流要它，`/mcp` 是普通的 JSON 答复（下面"反向代理与 MCP"）。下一条的两份样例里，事件流经过的那一部分（nginx 的 `location /` 与 `server` 里共用的几行、Caddy 的最后一个 `handle`）与验证过的配置是同一组指令：帧在写入答复的同一毫秒到达，心跳让连接一直活着（M5/P2）。Caddy 开了 `encode`（`zstd gzip`）也一样：它不压缩 `text/event-stream`，不必排除。nginx 用 HTTP/1.1 连上游；`proxy_read_timeout` 默认 60 秒，长于默认的心跳，调大心跳时一起调大。
- 样例（整个站点）：请求体按路由处理，`/mcp` 与页面正文的两个写缓冲，附件的上传与导入不缓冲（下面两条）。
  - nginx：共用的几行写在 `server` 里，每个 `location` 都继承它们。在某个 `location` 里加 `proxy_set_header` 时，它不再继承 `server` 里的三行 `proxy_set_header`，要在那里一起写上（少了 `X-Forwarded-For`，每个客户端都成了代理的 IP）：

    ```nginx
    server {
        server_name wiki.example.com;
        # listen 443 ssl; ssl_certificate …; ssl_certificate_key …; 照你的部署

        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 60s;

        # 页面、接口与事件流；1m 等于 server.max_body_bytes（默认 1 MiB），改了它时一起改
        location / {
            proxy_pass http://127.0.0.1:8080;
            client_max_body_size 1m;
        }

        # MCP：缓冲请求体，上限是 /mcp 的 37,814,272 字节
        location = /mcp {
            proxy_pass http://127.0.0.1:8080;
            proxy_request_buffering on;  # 显式写：server 或 http 一层的 off 会被继承
            client_max_body_size 37m;
        }

        # 页面正文的两个写：缓冲请求体，上限是它们的 31,522,816 字节
        location ~ ^/api/v0/(pages/[^/]+/content|notebooks/[^/]+/pages)$ {
            proxy_pass http://127.0.0.1:8080;
            proxy_request_buffering on;  # 同上
            client_max_body_size 31m;
        }

        # 附件的上传与导入的 zip：不缓冲；上限是导入包的上限（默认 512 MiB）加 64 KiB 的表单
        location ~ ^/api/v0/notebooks/[^/]+/(assets|imports)$ {
            proxy_pass http://127.0.0.1:8080;
            proxy_request_buffering off;
            client_max_body_size 513m;
        }
    }
    ```

  - Caddy（默认不缓冲请求体，`request_buffers` 只给要缓冲的三个路由；自动的 HTTPS 照 Caddy 的默认）：

    ```caddyfile
    wiki.example.com {
        # /mcp 与页面正文的两个写：缓冲请求体（37MiB 不小于 /mcp 的 37,814,272 字节）
        @buffered path_regexp ^/(mcp|api/v0/pages/[^/]+/content|api/v0/notebooks/[^/]+/pages)$
        handle @buffered {
            reverse_proxy 127.0.0.1:8080 {
                request_buffers 37MiB
            }
        }

        # 别的路由（页面、接口、事件流、上传与导入）：不缓冲，Caddy 的默认
        handle {
            reverse_proxy 127.0.0.1:8080
        }
    }
    ```

    经 Caddy 时，服务端在读请求体之前就答的拒绝（503 `server_busy`、401、429）对几 MiB 以上的请求体常常到客户端成了 502、没有 `Retry-After`（我们在 Caddy 2.10.2 上量过这三种，用的是在读请求体之前就答它们的替身，不是 nervewiki 本身；nginx 照原样转发了替身的 503 与它的 `Retry-After`）：网页的大的保存看到的是一次失败、不会等一等再重发；agent 看不到 `Retry-After`（客户端怎样对待 502 未核对）。

- 反向代理与上传（M7 起）：附件的上传（`POST /api/v0/notebooks/{notebook_id}/assets`）与导入的 zip（`POST /api/v0/notebooks/{notebook_id}/imports`）走流式的路由，服务端按字节放宽它们的读写期限（不低于每秒 64 KiB）。
  - 反向代理不要缓冲这两个路由的请求体（nginx 的 `proxy_request_buffering off` 只写在它们的 `location` 里；Caddy 默认不缓冲），否则整个文件传到代理之后服务端才开始读，提前的拒绝（无权、重名、超过上限）也要等传完。别的路由不要关掉缓冲（下一条）。
  - 它们的 `client_max_body_size`（nginx 默认只有 1 MB）不小于导入包的上限（`transfer.import_max_bytes`，默认 512 MiB）加 64 KiB：请求体是 zip 加表单的几个部分。附件的上限（`asset.max_bytes`，默认 50 MiB）加 64 KiB 在它之内（`import_max_bytes` 不小于 `asset.max_bytes`）。改了 `transfer.import_max_bytes` 的，这一行照它改。nginx 的 `client_body_timeout`、`proxy_send_timeout`、`proxy_read_timeout` 是两次读写之间的间隔，不是整个传输的时长，默认的 60 秒就够；按整个请求计时的代理或负载均衡，要容得下最慢的传输（50 MiB 的附件在每秒 64 KiB 下约 13 分钟，512 MiB 的导入包约 2 小时 17 分钟；`transfer.import_max_bytes` 按 `asset.upload_min_rate` 传完不能超过 3 小时）。
- 反向代理与 MCP（M9 起）：`/mcp` 是普通的请求与答复，每个 POST 一个 JSON 答复，不是 SSE，也不持住连接。
  - 请求体的上限不小于 `/mcp` 的 37,814,272 字节（约 36 MiB；nginx 写 `client_max_body_size 37m`，Caddy 的 `request_buffers 37MiB` 同样不小于它）：一次调用至多 5 MiB 的正文加 1 MiB 的别的参数，每个字节按 JSON 最长的转义算 6 个。
  - 读超时不短于 45 秒：一次调用的工作至多 15 秒（`server.request_timeout`），之前读请求体至多 30 秒（`server.read_timeout`）；改了这两个的，不短于它们的和。nginx 默认的 `proxy_read_timeout 60s` 够；Caddy 默认不限（照 Caddy 的文档）。
  - `Host` 照常转发（nginx 的 `proxy_set_header Host $host`；Caddy 默认就转发）：服务端关掉了 MCP SDK 对不是回环地址的 `Host` 的拒绝（同一台机器上的代理转来的 `Host` 是对外的名称），由 Bearer 令牌把关。
  - **`/mcp` 与页面正文的两个写（`PUT /api/v0/pages/{page_id}/content`、`POST /api/v0/notebooks/{notebook_id}/pages`）要缓冲请求体**：大的请求体（MCP 超过 64 KiB 的调用、超过 `server.max_body_bytes` 的正文写）整个服务同时只有两个名额、每个凭证一个（上文"MCP"的"传输"与"接入 agent（MCP）"），名额从读请求体之前占到答完；代理收完请求体再一次转给服务，慢的发送者就占不住名额。上面的两份样例都这样做。缓冲的请求体在服务端认证之前就收下了，放在代理那里：nginx 把过了 `client_body_buffer_size` 的写到 `client_body_temp_path` 的磁盘上（照 nginx 的文档），Caddy 放在内存里（我们在 Caddy 2.10.2 上量过：收着一个 30 MiB 的请求体时代理的内存涨了约 66 MB，一个接近 `/mcp` 上限的约 116 MB，没有写临时文件），所以几个慢的发送者不必有有效的令牌，就能让代理各占至多约 37 MiB 的磁盘（nginx）或一百多 MB 的内存（Caddy）：给代理留出这些地方。
  - 不缓冲时（代理关掉了缓冲，或直接暴露服务）：两个慢的发送者（两个令牌，或两个登录的会话）各占一个名额至多约 45 秒（请求体至多 `server.read_timeout` 30 秒，之后这次调用至多 `server.request_timeout` 15 秒），其间别人的大的写等 2 秒后答 503 `server_busy`。这样被拒的每个请求，服务端记一行 info 的日志 `large request bodies are saturated`（名额只是满着、没有人被拒时不记），`holders` 是这时持着名额的凭证（`pat:<令牌 id>` 或 `session:<会话 id>`）与路由：由 `api_tokens`（PAT）或 `auth_sessions`（会话）那一行的 `user_id` 找到账户，请他在"访问令牌"里撤销那个令牌，或以 `nervewiki users reset-password` 撤销这个账户的全部会话与 PAT（上文"管理员命令"）。
````

升级的说明加在 M9/P4 的两条（`README.md:332`、`:334`）之后、两个数据库角色一条（`:336`）之前：

````markdown
- 升级到带 MCP 的版本（M9）时，照下面部署一节的"样例（整个站点）"改代理的配置：`/mcp` 与页面正文的两个写缓冲请求体，附件的上传与导入不缓冲。照之前的样例在 `location /`、`server` 或 `http` 里写的 `proxy_request_buffering off` 只留在上传与导入的 `location` 里，`client_max_body_size 512m` 换成按路由的几个（导入的是 `513m`）。照之前的 README 设了 `client_max_body_size 512m` 的，不改时 `/mcp` 照常能用，但它与正文的写一直不缓冲；留着 nginx 默认的 1m 的，过 1 MiB 的 MCP 调用由 nginx 答 413。
````

### 5.2 与今天的不同、为什么

- **事件流一条**：加一句"只有事件流要它，`/mcp` 是普通的 JSON 答复"；"下面两份配置都验证过"改为事件流经过的那一部分与验证过的配置是同一组指令：M5/P2 验证的是一个 `location /` 与 Caddy 的默认（`docs/v0.1/M5-collab-editing/02-P2-event-stream.md:261`，`caddy:2.10-alpine` 与 `nginx:1.29-alpine` 的容器），不是新的整个 `server`。nginx 的 `location /` 继承 `server` 的四个 `proxy_set_header`（审查 B 的隔离的一次运行看到每个 `location` 都转发了 `Host`，`review-B/nginx-route/`），Caddy 的最后一个 `handle` 就是今天的 `reverse_proxy`。nginx 用 HTTP/1.1 与读超时的一句从样例的引子挪进这一条。
- **样例是它自己的一条**：两份样例都是整个站点的配置，上传与 MCP 两条都指着它们，放在事件流一条里难找。
- **nginx 的样例**：共用的五行从 `location /` 挪到 `server` 里。nginx 的 `proxy_http_version`、`proxy_read_timeout` 由 `location` 继承上一层的；`proxy_set_header` 只在这一层自己一个也不写时继承（四个 `location` 都不写），所以样例后面那一句提醒加的人。`location = /mcp` 是精确匹配（查询串不参与匹配），优先于正则；两个正则只匹配各自的路由（`[^/]+` 不跨段）。审查 B 以 `nginx -t` 核对了语法、以隔离的一次运行把 2 MiB 的请求体发到每一类路由（`review-B/nginx/`、`review-B/nginx-route/`）：每个路由落在它的 `location`，`/mcp/`、`/MCP`、`/.well-known/…`、`/register` 与事件流落在 `location /`。修订加的只是显式的 `proxy_request_buffering on`、`location /` 的 `1m`、`513m` 与注释，不改匹配。
- **显式的 `proxy_request_buffering on`**：照今天的 README 在 `server` 或 `http` 一层写了 `off` 的部署，新的 `location` 会继承它，`/mcp` 又不缓冲了，正是这一节要去掉的风险，而且没有任何报错；显式写在两个缓冲的 `location` 里就不继承。
- **`location /` 的 `1m`**：等于服务端的 `server.max_body_bytes` 的默认（1,048,576 字节，`server/configs/config.yaml:23`），别的路由的请求体在它之内（契约里比它大的只有两个正文路由、`/mcp` 与两个流式的上传：`bootstrap/wire.go:183-184`，`asset/adapter/http/handler.go:67`，`transfer/adapter/http/handler.go:71`）。`server.max_body_bytes` 能配置（它也是正文路由算"大的请求体"的门槛，`platform/httpserver/api.go:315-320`），调大了它而不调这一行，1 到新的上限之间的请求体就被 nginx 自己的 413 拒掉，服务端什么也不记：所以写出来、注释说一起改。
- **两个缓冲的 `client_max_body_size` 取服务端的上限**：37m 是 38,797,312 字节、31m 是 32,505,856 字节，都不小于服务端的 37,814,272 与 31,522,816（`bootstrap/mcp.go:29` 的 `6*(page.MaxContentBytes+1<<20) + 64<<10`；`page/adapter/http/limits.go:8` 的 `6*domain.MaxContentBytes + 64<<10`）。服务端的上限是常量（4.14），改了时这两行随之：样例的注释写出字节数，审查对得上。
- **513m**：导入的路由收至多 `import_max_bytes + Envelope` 字节（`transfer/adapter/http/handler.go:80`，`start_import.go:24` 的 `Envelope = 64 << 10`；`:45` 按 `Content-Length` 先拒），默认 536,870,912 + 65,536 = 536,936,448（`server/configs/config.yaml:151`）；上传收至多 `asset.max_bytes + Envelope`（`asset/adapter/http/handler.go:76`，`upload.go:30`），默认 52,428,800 + 65,536 = 52,494,336（`config.yaml:133`）。两者共用一个 `location`，取较大的，管它的是导入的：`import_max_bytes` 不小于 `asset.max_bytes`（`config.yaml:149` 的注释；配置的校验 `platform/config/validate.go:291-292` 拒绝更小的）。513m 是 537,919,488 字节，不小于 536,936,448。512m（536,870,912）差 65,536 字节：一个接近上限的导入包由 nginx 自己的 HTML 413 拒掉，网页显示一个笼统的失败，服务端的日志里什么也没有。
- **事件流以外的读不必缓冲**：附件与导出的下载经 `location /`；导入的列表（`GET /api/v0/notebooks/{notebook_id}/imports`）与导入同一个路径，落在不缓冲的 `location`：GET 没有请求体，缓冲与否无关。
- **Caddy 的样例**：审查 B 与修订的重跑（同一份 `Caddyfile` 与 `run.sh`，Caddy 2.10.2，上游是同一个容器里读请求体并记 `duration` 的第二个站点）都是 `Valid configuration`；200,000 字节的请求体以 `curl --limit-rate 50k`（约 4 秒）发到每个路由，上游的 `duration`：

  | 经 `:8091` 的路由 | 审查 B | 修订的重跑 |
  |---|---|---|
  | `POST /mcp?notebook=x` | 0.0014 秒（缓冲） | 0.000539 秒（缓冲） |
  | `PUT /api/v0/pages/x/content` | 0.0011 秒（缓冲） | 0.000599 秒（缓冲） |
  | `POST /api/v0/notebooks/x/pages` | 0.0006 秒（缓冲） | 0.00197 秒（缓冲） |
  | `POST /api/v0/notebooks/x/assets` | 3.85 秒（流式） | 3.8466 秒（流式） |
  | `POST /api/v0/notebooks/x/imports` | 3.85 秒（流式） | 3.8396 秒（流式） |

  所以 `request_buffers` 在这个写法下只让这三个路由的请求体被缓冲，`path_regexp` 只看路径（`/mcp?notebook=x` 照样匹配）。分块（`Transfer-Encoding: chunked`）的请求到上游时仍是分块的、没有 `Content-Length`（`run-out.txt` 的最后两行），服务端的 `largeBody` 先读 64 KiB 再取名额（`platform/httpserver/largebody.go:150-160`）：代理已收下了请求体，这一读很快，保护照样成立。README 的样例本身（`wiki.example.com` 与注释，`exp/caddy/readme/Caddyfile`）`caddy validate` 是 `Valid configuration`，`caddy adapt` 出的 `request_buffers` 是 38,797,312、`path_regexp` 是这三个路由（`exp/caddy/readme/validate-out.txt`；唯一的一行警告是缩进不是 Tab，`caddy fmt` 的格式，今天的 README 的样例同样用空格）。
- **Caddy 的缓冲在内存里**：一个 30 MB 的请求体以每秒 3 MB 发到 `/mcp`，收着的时候每秒读一次 Caddy 进程的 `VmRSS` 并找 `/tmp`、`/var/tmp` 下新的文件（`exp/caddy/mem.sh`）：43,684 kB 涨到第 6 秒的 109,252 kB，临时文件一直是 0 个（`mem-out.txt`；那一次上游与代理在同一个进程里，之后的 220,236 kB 也算着上游与它回显的 30 MB）。审查 B 的第二轮把代理与上游分在两个进程里、量到上传的最后（`exp/caddy/mem-separate/run.sh`、`out.txt`）：30 MiB 的请求体 42,368 → 108,472 kB（约 +66 MB）；36 MiB（37,748,736 字节，刚在 `/mcp` 的 37,814,272 之下）108,544 → 224,268 kB（约 +116 MB），Go 的缓冲过了 32 MiB 再翻一倍；都没有临时文件。所以 README 写"放在内存里"，并给出这两个数；它在认证之前（代理不认令牌），几个慢的、不带有效令牌的发送者就能让代理各占一百多 MB，README 照实说。nginx 写临时文件是 nginx 的文档说的，标出来。
- **经 Caddy 时早答的拒绝成了 502**（P5 的设计审查 B 的第二轮、第三轮，`exp/caddy/early/`、`exp/caddy/early2/`）：服务端有几种拒绝在读请求体之前就答出：取不到名额的 503 `server_busy`（声明的长度过了门槛、等满 2 秒，`platform/httpserver/largebody.go:146`、`:166-170`），认证失败的 401 与限流的 429（`platform/httpserver/api.go:335-350`、`limit.go:24-36`）。审查 B 以一个不读请求体就答 503 与 `Retry-After: 1` 的上游代替服务端（另一个 Caddy 进程，Go 的 `net/http`，同服务端；不是 nervewiki 本身），经上面的 Caddy 样例与 nginx 样例各发一遍；每格是三次的状态码，标着"一次"的只发了一次（Caddy 的是 `early/out-caddy.txt`、`early/out-caddy2.txt`；nginx 的 100 KiB 是 `early/out-nginx.txt`，1 MiB 起是第三轮的 `early2/out-nginx-3x.txt`，三类路由每个大小各三次）：

  | 请求体 | Caddy，缓冲的路由（`/mcp` / 正文的 PUT） | Caddy，不缓冲的路由（附件） | nginx（修订后的样例，三类路由） |
  |---|---|---|---|
  | 100 KiB | 503（只有 `/mcp`，一次） | 503（一次） | 503（一次） |
  | 1 MiB | 503（只有 `/mcp`，一次） | 503（一次） | 503 503 503 |
  | 2 MiB | 503 502 502 / 502 502 502 | 503 502 502 | 503 503 503 |
  | 4 MiB | 503 502 502 / 502 502 503 | 503 503 503 | 503 503 503 |
  | 8 MiB | 502 502 502 / 502 502 502 | 502 503 503 | 503 503 503 |
  | 30 MiB | 502 502 502 / 502 502 503 | 503 503 503 | 503 503 503 |

  第三轮又以同样的替身答 401（带 `WWW-Authenticate`）或 429，经 Caddy 样例各发三次（`early2/out-caddy-401-429.txt`，只记了状态码；nginx 没有以这两个替身量）：

  | 替身答的 | 请求体 | Caddy，`/mcp` | Caddy，正文的 PUT |
  |---|---|---|---|
  | 401 | 1 MiB | 401 401 401 | 401 401 401 |
  | 401 | 8 MiB | 401 502 502 | 502 502 502 |
  | 429 | 1 MiB | 429 429 429 | 429 429 429 |
  | 429 | 8 MiB | 502 502 502 | 502 502 502 |

  1 MiB 时照原样转发；8 MiB 时 12 次里 11 次是 502，同 503 的一样。

  Caddy 的日志里那些 502 是 `write tcp 127.0.0.1:…->127.0.0.1:8080: write: connection reset by peer`：上游答了、关了连接（Go 的 `net/http` 在处理函数留下太多没读的请求体时关掉连接），Caddy 还在写缓冲着的请求体，报的是写的错误，不是答复。nginx 在量过的每个大小（100 KiB 到 30 MiB）、每类路由都把 503 与它的 `Retry-After: 1` 照原样转发，第三轮的 45 次都是（`early/out-nginx.txt`、`early2/out-nginx-3x.txt`，修订后的样例 `nginx -t` 通过）。

  后果：两个大的调用占着两个名额时，经 Caddy 的第三个大的写或过 `server.max_body_bytes` 的网页保存拿到的是一个光秃的 502。网页的保存只在 503 带着 `Retry-After` 时等一等再发（`web/apps/web/src/stores/page-editing.ts:336`），于是立刻失败；README 的"503……稍后重发"也到不了 agent。过期的访问令牌碰上一次大的保存（401）、限流（429）同样（替身量过，上面第二张表）：网页只在 401 时续期再发（`web/apps/web/src/session/auth-middleware.ts:45`），一个 502 不续期。缓冲让"慢的发送者占着名额"少见，所以这是窄的情形，却正是名额自己的拒绝；Caddy 的默认（不缓冲）在 2 MiB 时也有，样例让它在三个缓冲的路由上几乎每次都是。

  仍然保留 Caddy 的样例：它去掉了慢的发送者的风险，502 是一个变差了的错误信号，不丢数据。README 在 Caddy 的样例之后一句说明；服务端怎样改（例如早答之前先把一个快的请求体短短地读掉）交 M12（第 11 节），只是一种可能，不是决定，P5 不加服务端的代码。合并之前的重跑带上这个探测（9.3），清单的 I3 记下经 Caddy 的一次被拒的大的写显示什么。
- **剩下的风险的说法**出自代码：名额从读请求体之前占到答完（`platform/httpserver/largebody.go:123-171` 的 `largeBody`：`defer release()` 在 `next.ServeHTTP` 之后），所以至多约 45 秒：请求体至多 `server.read_timeout` 30 秒（`config.yaml:11`，`platform/httpserver/server.go:44` 的 `ReadTimeout` 管整个请求），之后这次调用至多 15 秒（`mcpserver/call.go:40-56`：调用的期限是读完请求体加 15 秒），都在路由的 45 秒之内（`platform/httpserver/api.go:297-310`：`BodyLimits` 的路由的期限是 `request_timeout + read_timeout`）。满了时的 info 的一行是 `large request bodies are saturated`，带路由、发起的凭证与 `holders`（`largebody.go:244-250`）；凭证的形状是 `pat:<id>` 或 `session:<id>`（`identity/adapter/authn/authenticator.go:48-50`），两张表都有 `user_id`（`server/migrations/sql/00003_identity_auth_sessions.sql:7`、`00004_identity_api_tokens.sql:7`）。
- **没有按令牌撤销的管理员命令**：草稿照 P2 与 00 号文档写的"管理员可以撤销那个 PAT 或结束那个会话"不成立：撤销 PAT 的接口只撤调用者自己的（`identity/app/revoke_api_token.go:12-15`，`DELETE /api/v0/api-tokens/{token_id}`），管理员命令按邮箱、没有按令牌或会话的（`README.md:82-92`：`reset-password` 撤销账户的全部会话与 PAT，`deactivate` 停用）。所以 README 写能做的：由 `user_id` 找到账户，请他撤销，或 `reset-password`。00 号文档第 10 节（`:612`）与 M12 的移交（`M12-release/handoffs/M4-performance.md:95`）里同样的说法，P5 随第 7 节的修订一起改（2.1 第 9 条、7.3）。
- **升级的一条**：README 的部署一节对每个有运维改动的 M 写一条升级的说明（`README.md:324`、`:330`、`:332`、`:334`）。P5 改了推荐的代理配置，照今天的样例部署的 M8 实例升级之后 `/mcp` 照常能用（512m 大于 37m），没有什么让操作的人回来看这一节，`/mcp` 与正文的写就一直不缓冲：所以加一条。
- **HTTPS**：样例的 `server` 不写 `listen` 时听 80 端口，每个 `/mcp` 的请求都带着 PAT；一行注释指出 `listen 443 ssl` 与证书照部署写（总体设计第 11 节"公网部署加 HTTPS"），接入一节的"先准备"另有一句。Caddy 对站点的名称自动用 HTTPS。
- 总体设计第 11 节的反向代理一条随之加按路由缓冲的一句（第 7 节）。

## 6. 人工清单 `manual/P5-mcp-clients-checklist.md`

文件是 `docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md`（同输入法清单在 `docs/v0.1/M4-pages/manual/P6-ime-checklist.md`）。格式照输入法清单：开头一张表（来源、执行人、客户端、结果），然后"准备"、分部分的"步骤"（每步一行：操作、预期，另有记下实际的一栏），最后"不在这份清单里的"。与输入法清单不同的两处：每一步多一栏"记下的"（Claude Code 与 Codex 各一栏；只对一个客户端的部分一栏），因为这份清单的一半是"记下客户端显示的原文"，P5 的 4.2 照它改 README；步骤分 A–I 九部分、各自编号（`A3`、`D5`），4.2 与 README 的改法引它们。每一步出自哪里只在 2.3 的处置表里列（按来源、带步号），这里不再按步列一遍：两份同样的对照会各改各的。

### 6.1 全文（仓库里建成的样子）

下面是 `docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md` 建成的样子（`053d3ba1`）：S1 照这一节原来的文字逐字节提交，修复两轮照审查改了它（3.9 的"审查与修复定下的"第 4 条；为什么这样写见 6.2）。

````markdown
# M9/P5 人工验收：真实的 MCP 客户端

| 项 | 内容 |
|---|---|
| 来源 | [M9 总设计](../00-M9-design.md) 4.16"人工的"；[M0/P1 MCP 会话的移交](../handoffs/M0-P1-mcp-sessions.md)第 1、3 项；[P1](../01-P1-platform-reads.md)、[P2](../02-P2-writes-history.md)、[P3](../03-P3-batch-revert.md)、[P4](../04-P4-grep-search.md) 各自第 8 节给 P5 的清单的几步；[P5](../05-P5-stories-docs.md) 4.2（README 里客户端的每一句与核对它的步）、第 5 节（反向代理）；每一步出自哪里见 P5 设计 2.3 |
| 执行人 | 负责人，在 v0.1 的最后与[输入法清单](../../M4-pages/manual/P6-ime-checklist.md)一起（M9 总设计第 11 节的取舍第 12 条）；负责人同意时，P5 的实施者可以先跑一遍 |
| 客户端 | Claude Code 与 Codex 的当前正式版，各一遍。开始之前记下 `claude --version`、`codex --version` 与日期 |
| 结果 | 每一步每个客户端：通过、不通过或"记下"（只要原文的步），写在"记下的"一栏，原文照抄（截到一两行）。整份写进 M9 收尾审查记录的"人工验收"一节。**通过**：每一步不是"记下"的预期都成立，或者每个不成立的旁边写着负责人的处置（照 P5 设计 4.2 改 README、交 M12，或改服务端）；M9 在这份清单通过之后才改为已完成。README 里客户端的句子（P5 设计 4.2 的表）照记下的改，改的提交写明照哪一步 |

这份清单补上自动化测试做不到的：e2e 的 MC1 经 TypeScript 官方 SDK 的客户端连接，整个程序的测试用官方 Go SDK 的客户端（M9 总设计 4.16"自动的"），它们不是 Claude Code 与 Codex，也没有模型、审批与界面。

**怎样读"预期"**：服务端的答复、日志与库里的结果由代码定；客户端与 agent 的做法都未核对（这份清单就是核对它们的），出自客户端的文档的标"照文档"，M0/P1 的实验记下过的标"M0/P1"。不符时照实记下：多半是那一句写错了，不是客户端坏了。

## 不碰你自己的客户端配置

整份清单不改 `~/.claude.json` 里的 MCP 服务、`~/.claude/settings.json`、`~/.codex/` 与任何项目里已有的 `.mcp.json`：

- **Claude Code**：一律在 `$CC`（"准备"第 5 步的临时目录）里以 `claude --mcp-config "$CC/<文件>" --strict-mcp-config` 启动：只用这个文件里的服务（照文档；插件带来的服务与 claude.ai 的连接器是否也不起未核对，A1 看 `/mcp` 的列表）。不在别的目录里执行 `claude mcp add`（本地与用户的范围写 `~/.claude.json`）；第 A3 步只在 `$CC` 里以 `--scope project` 写那个目录自己的 `.mcp.json`，前后各数一次 `~/.claude.json` 里的 `"nervewiki"` 核对它。不执行 `claude mcp list`：它会连一遍你配置过的每个服务。Claude Code 照常在 `~/.claude.json` 里记下它进过的目录（信任与历史），在 `~/.claude/projects/` 下记每个会话（那些目录怎样起名未核对）：那是它自己的状态，不是 MCP 的配置，可以留着（"做完"的第 4 步）。审批的几步各在只给这一步的临时目录里做（第 D 部分的开头），D2 的放行写在它那个目录的 `.claude/settings.json`（项目的设置），不改你自己的设置。**开始之前记下**你自己的 `~/.claude/settings.json` 里的 `permissions.defaultMode` 与提到 `mcp__` 的放行或拒绝的规则：它们会改变第 D 部分的结果。
- **Codex**：一律在一个临时的 `CODEX_HOME` 里、从一个空的临时目录 `$CX` 启动（"准备"第 6 步）：Codex 只读 `$CODEX_HOME/config.toml`，你的 `~/.codex/config.toml` 里的别的 MCP 服务不会起（照文档；`codex -c mcp_servers.x.url=…` 是叠在 `~/.codex/config.toml` 之上的，那样它们照样起）；从空的目录启动，它不会拿到某个项目自己的配置，也不会在你的代码里读写文件。登录，按这个次序选：
  1. **用 API 密钥**（推荐）：`printenv OPENAI_API_KEY | codex login --with-api-key`（照文档），凭证写在临时的 `CODEX_HOME` 里，与你自己的登录无关。
  2. **在临时的 `CODEX_HOME` 里另登录一次**：`codex login`（ChatGPT 的账户）。做完之后在一个没有设 `CODEX_HOME` 的终端里启动你自己的 Codex，记下它是否照常可用；不能用时在那个终端里（你自己的 `~/.codex`）`codex login` 再登录一次。第二次登录会不会让第一次的失效未核对。
  3. **最后才用：复制 `auth.json`**。复制时记下原来的那一份的指纹：`shasum ~/.codex/auth.json`，写进"记下的"。清单进行的期间**不要在别处用 Codex**（别的终端、编辑器的插件）：任何一边的一次续期都会轮换刷新令牌，另一边的那一份随之作废。做完时（"做完"的第一步，在删掉临时目录之前）：原来的那一份的指纹没变、临时的那一份变了，就把临时的复制回 `~/.codex/auth.json`；原来的那一份变了，不复制；两份都变了，其中一份已经作废，在你自己的环境里 `codex login` 再登录一次。
- **令牌**：给每个客户端建它自己的 PAT（"准备"第 3 步），各放在自己的变量里：Claude Code 的终端里 `NERVEWIKI_CLAUDE_TOKEN`，Codex 的终端里 `NERVEWIKI_CODEX_TOKEN`，都以 `read -rs <变量> && export <变量>` 设（粘贴、回车：不回显，不进 shell 的历史）。做完在网页上撤销它们。

## 准备

1. 起一个实例：`make dev-db`、`make build`。dev 不配置签名私钥时每次启动换一把，重启之后浏览器都要重新登录，所以在服务的终端里先生成一把、整份清单都用它（README"签名私钥"；要 OpenSSL 3，例如 Homebrew 的 `openssl@3`：macOS 自带的 `/usr/bin/openssl` 是 LibreSSL，没有 ed25519，先 `openssl version` 看它以 `OpenSSL 3` 起头）：

   ```sh
   export KEYS=$(mktemp -d)
   openssl genpkey -algorithm ed25519 -out "$KEYS/jwt.pem"
   export NWIKI_AUTH__JWT__PRIVATE_KEY_FILE="$KEYS/jwt.pem" NWIKI_SERVER__PUBLIC_URL=http://127.0.0.1:8080
   make run
   ```

   （`make run` 提供上一次 `make build` 的前端；环境变量照常传下去。）之后的重启（E6、G1、G2）都在这个终端里 `make run`。下面 `$BASE` 是 `http://127.0.0.1:8080`（第 2 步说怎样设它）。这个终端里看服务的日志：每个请求一行 `http request`（方法、路径、状态、用时 `duration`），每次 MCP 调用一行 `MCP tool call`，握手一行 `MCP initialize`（带客户端名称），大的请求体没有名额时一行 `large request bodies are saturated`。
2. 两个账户：A（你，工作区的管理员）与 B（同一工作区的成员）。B 的加入：A 在工作区的成员页邀请 B 的邮箱（README"邀请"），B 在另一个浏览器或无痕窗口里注册（dev 开放注册）并接受邀请。A 建两本笔记本，**一个客户端一本**：`MCP 清单 CC`（Claude Code 用）与 `MCP 清单 Codex`（Codex 用），B 是两本的编辑者。第 C、H 部分改写种下的页：若两个客户端在同一本里做，后一个会看到前一个改过的页，所以一个客户端一本。笔记本的 id 是网页地址里 `/notebooks/` 之后的一段，下面写 `$NB_CC` 与 `$NB_CODEX`；只说 `$NB` 的步，用这个客户端自己的那本。这些是 shell 的变量：在用到它们的每个终端里（种下页的、`$CC` 的、E4 的、F1 的占位的与第 I 部分的）各设一次：

   ```sh
   export BASE=http://127.0.0.1:8080
   read -r NB_CC && export NB_CC
   read -r NB_CODEX && export NB_CODEX
   ```

   （两次 `read` 各粘贴一本笔记本的 id、回车。）第 I 部分的终端另设 `export CADDY=http://127.0.0.1:8091`（经 Caddy 的地址，I3 用）。
3. A 在"设置"→"访问令牌"（`$BASE/settings/tokens`）建三个 PAT：`seed`（下一步用）、`claude-checklist`、`codex-checklist`。
4. 种下页与附件，两本笔记本各一次：
   - 准备三个文件，每个的字节都要是它的名字说的类型：服务端按开头的字节判断，名字是 `.png` 而字节是 JPEG 的附件存成 `application/octet-stream`，不直接放进答复（`asset/domain/types.go` 的 `TypeOf`）。`small.png` 是 PNG、每边不过 1,000 像素、几十 KB（例如一张截图以 `sips -Z 800 -s format png 截图.png --out small.png` 缩小）；`wide.png` 是 PNG、宽 2,500–7,000 像素、2.0–2.8 MiB（JPEG 或别的格式先转成 PNG：`sips -s format png 照片.jpg --out wide.png`，太大时加 `-Z` 与一个像素数缩小再转；以 `sips -g pixelWidth -g pixelHeight wide.png` 与 `ls -l` 核对：3 MiB 以上的不直接放进答复）；`doc.pdf` 是一个小的 PDF。做好之后 `file small.png wide.png doc.pdf`：前两个是 "PNG image data"，第三个是 "PDF document"。
   - 把下面的脚本存成临时目录里的 `seed.mjs`，`read -rs SEED_TOKEN && export SEED_TOKEN`（`seed` 的令牌），然后 `node seed.mjs "$BASE" "$NB_CC" small.png wide.png doc.pdf` 与 `node seed.mjs "$BASE" "$NB_CODEX" small.png wide.png doc.pdf`（Node 22 起；它只经 REST 建页、上传附件）：

   ```js
   // seed.mjs：在一本笔记本里种下这份清单要的页与附件（经 REST，令牌取自 SEED_TOKEN）
   import { openAsBlob } from "node:fs";
   const [base, notebook, small, wide, pdf] = process.argv.slice(2);
   const auth = { Authorization: `Bearer ${process.env.SEED_TOKEN}` };
   async function page(title, content, parent = null) {
     const r = await fetch(`${base}/api/v0/notebooks/${notebook}/pages`, {
       method: "POST",
       headers: { ...auth, "Content-Type": "application/json" },
       body: JSON.stringify({ parent_id: parent, title, content }),
     });
     if (!r.ok) throw new Error(`${title}: ${r.status} ${await r.text()}`);
     return (await r.json()).id;
   }
   async function asset(file, name, parent) {
     const form = new FormData();
     form.set("parent_id", parent);
     form.set("file", await openAsBlob(file), name);
     const r = await fetch(`${base}/api/v0/notebooks/${notebook}/assets`, { method: "POST", headers: auth, body: form });
     if (!r.ok) throw new Error(`${name}: ${r.status} ${await r.text()}`);
   }
   const han = "天地玄黄宇宙洪荒日月盈昃辰宿列张寒来暑往秋收冬藏";
   const cut = "甲".repeat(500) + "FIXME 在这里" + "乙".repeat(500) + "行尾在此";
   await page("AGENTS", "---\ntype: agents\n---\n这份清单的暗号是青石板。\n\n回答用中文。\n");
   const guide = await page("Guide", "# Guide\n\n他说“先读再写”，然后离开。\n重叠：aaa\n要删的一行\n- [ ] 一件事\n");
   await page("长文", Array.from({ length: 800 }, (_, i) => `第 ${i + 1} 行：${han}`).join("\n") + "\n");
   await page("命中", Array.from({ length: 600 }, (_, i) => `TODO 第 ${i + 1} 项：${han.repeat(7)}`).join("\n") + "\n" + cut + "\n");
   await page("长行", "x".repeat(4 << 20) + "完成\n");
   await page("锁", "被编辑的页。\n", guide);
   const pictures = await page("图库", "");
   await asset(small, "图一.png", guide);
   await asset(wide, "宽图.png", guide);
   await asset(pdf, "说明.pdf", guide);
   for (let i = 1; i <= 21; i++) await asset(small, `p${String(i).padStart(2, "0")}.png`, pictures);
   console.log("seeded");
   ```

   - `长文.md` 约 69 KB（约 2.2 万个汉字与全角的标点）：一次 `read` 给不完。
   - `命中.md` 前 600 行都有 `TODO`，每行约 520 字节：`grep` 的一次答复放 85 行就到 48 KiB，在 `limit` 的默认 100 之前；第 601 行约 3,000 字节，`FIXME` 在它的中间，`grep` 只给 `FIXME` 前后共约 1,000 字节（`cut`），行尾的"行尾在此"不在其中。
   - `长行.md` 是 4 MiB 的一行，`\pL{30}完成` 这样的模式只匹配得到它的开头（约 2.2 MB），这一行进 `partial_lines`。
5. Claude Code 的配置，在一个新的临时目录 `$CC` 里（`export CC=$(mktemp -d); cd "$CC"`）。每一份都是 README"接入 agent（MCP）"的 `.mcp.json` 样例原样照抄，只把 `https://wiki.example.com` 换成 `http://127.0.0.1:8080`（或下面说的地址）、`<笔记本 id>` 换成 `$NB_CC` 的值（文件里写字面的 id）：
   - `$CC/mcp.json`：笔记本入口；
   - `$CC/account.json`：账户入口 `http://127.0.0.1:8080/mcp`；
   - `$CC/throttled.json`：账户入口，经第 F2 步的限速转发 `http://127.0.0.1:8081/mcp`；
   - `$CC/notoken.json`：同 `mcp.json`，去掉整个 `headers`（第 E1 步）。

   在这个终端里设 `NERVEWIKI_CLAUDE_TOKEN`（`claude-checklist` 的令牌，`read -rs`）。
6. Codex：`export CODEX_HOME=$(mktemp -d) CX=$(mktemp -d); cd "$CX"`，照上面的"不碰你自己的客户端配置"登录，把 README 的 `config.toml` 样例原样写进 `$CODEX_HOME/config.toml`，同样只换地址与 `<笔记本 id>`（`$NB_CODEX` 的值）；在这个终端里设 `NERVEWIKI_CODEX_TOKEN`（`codex-checklist` 的令牌，`read -rs`）。每次改了 `config.toml`，退出 Codex、再在 `$CX` 里启动。

## 步骤

下面"让 agent……"是在客户端的会话里用自然的话说；不要告诉它工具的参数，看它照工具的说明与服务端的说明自己怎样做。两个客户端都是能读写本地文件的 agent：每个会话的第一句话先说"页与附件都在 Nerve Wiki 的这本笔记本里，只经 nervewiki 的工具读写，不读写本地的文件"（只说是哪个服务，不说参数）。

在笔记本入口，种下的 `AGENTS` 页整个取代内置的说明（README"MCP"的"说明"）。C 部分里照说明做的几步（C3 的引号、C5 的唯一、C6 的完整路径、C12 与 C15 的 `partial_lines`、`cut` 与 `column`）靠的是工具自己的说明（`api/mcp/tools.json`），不受影响；F 部分要内置的说明里"大的写一个接一个地发"那一句（只在内置的说明里），所以 F 在账户入口做：它的握手照常给内置的说明，`open_notebook` 另给 `AGENTS` 页的正文。

### A：Claude Code 接上

| # | 操作 | 预期 | 记下的 |
|---|---|---|---|
| A1 | 在 `$CC` 里 `claude --mcp-config "$CC/mcp.json" --strict-mcp-config`，输入 `/mcp` | `/mcp` 只列出 `nervewiki`、已连上、14 个工具（照文档；列出别的服务时记下：它来自插件或 claude.ai 的连接器）；服务的日志里一行 `MCP initialize`，记下它的客户端名称。日志里若是 401：这个文件里的 `${NERVEWIKI_CLAUDE_TOKEN}` 没有展开（照文档它展开）：记下，之后改用 `$CC/mcp-literal.json`（同一个文件，令牌照字面写进去），`account.json` 与 `throttled.json` 也同样写成字面的（`account-literal.json`、`throttled-literal.json`，A5 与 F 用它们），三份都 `chmod 600`，"做完"删掉 | |
| A2 | 让 agent 说出"这本笔记本的说明里的暗号" | 它答"青石板"（握手时的说明是 `AGENTS` 页去掉 frontmatter 的正文；M0/P1 记下 Claude Code 收到了说明），不调用工具 | |
| A3 | 先数一次你的 `~/.claude.json` 里的 `"nervewiki"`：`grep -o '"nervewiki"' ~/.claude.json` 每处列一行，记下几行。在 `$CC` 里执行 README 的项目范围的命令，地址照第 5 步换：`claude mcp add --scope project --transport http nervewiki "http://127.0.0.1:8080/mcp?notebook=$NB_CC" --header 'Authorization: Bearer ${NERVEWIKI_CLAUDE_TOKEN}'`（双引号里的 `$NB_CC` 由 shell 展开，单引号里的令牌一段原样）；然后 `grep nwk_pat_ .mcp.json`，再 `claude --mcp-config "$CC/.mcp.json" --strict-mcp-config`，输入 `/mcp`；退出之后再数一次 `"nervewiki"` | `$CC/.mcp.json` 里是 `${NERVEWIKI_CLAUDE_TOKEN}` 原样、`grep` 什么也找不到（照文档）；连上，14 个工具（`${…}` 由 Claude Code 展开，照文档）。记下 `ls -a "$CC"` 里这条命令另外写了什么。两次数的相同：命令没有把服务写进你的 `~/.claude.json`（它的结构仓库里没有记录，数的只是出现的次数，未核对）。多了时记下，在 `$CC` 里执行 `claude mcp remove nervewiki --scope local`（未核对）去掉它，再数一次，仍然多时也记下；不要动你原有的 `nervewiki`（第一次数的不是 0 时） | |
| A4 | 只把 `$CC/mcp.json` 里的 `${NERVEWIKI_CLAUDE_TOKEN}` 换成一个不存在的变量 `${NO_SUCH_TOKEN}`，启动，`/mcp`；之后改回 | 记下 Claude Code 怎样对待没有设的变量（报错、留空、照原样发出）与服务端的日志（401？） | |
| A5 | `claude --mcp-config "$CC/account.json" --strict-mcp-config`，让 agent 列出它能用的笔记本，再打开 `MCP 清单 CC` 并说出暗号 | 16 个工具；它先调 `notebooks`、再调 `open_notebook`（说明在它的答复里），答"青石板"；之后的调用都带 `notebook` | |
| A6 | 第 C 部分做过之后，网页上打开两本笔记本的历史，按"MCP"筛选 | `MCP 清单 CC` 里 agent 的变更集显示"MCP（<A1 记下的名称>）"，`MCP 清单 Codex` 里的显示"MCP（<B1 记下的名称>）" | |

### B：Codex 接上

| # | 操作 | 预期 | 记下的 |
|---|---|---|---|
| B1 | 在设了临时 `CODEX_HOME` 与 `NERVEWIKI_CODEX_TOKEN` 的终端里，在 `$CX` 启动 `codex`，输入 `/mcp` | 列出 `nervewiki` 与它的 14 个工具（照文档）；服务的日志里一行 `MCP initialize`，客户端名称 `codex-mcp-client`（M9 总设计 4.5） | |
| B2 | 让 agent 说出"这本笔记本的说明里的暗号" | 它答"青石板"；记下它有没有调用工具（读 `AGENTS.md` 得到的不算握手的说明：Codex 是否把服务端的说明交给模型未核对） | |
| B3 | 把 `config.toml` 的 `url` 换成账户入口 `http://127.0.0.1:8080/mcp`，重启 Codex，做 A5（打开 `MCP 清单 Codex`）；之后换回笔记本入口 | 同 A5 | |
| B4 | 退出 Codex，`unset NERVEWIKI_CODEX_TOKEN`，再启动 | 记下 Codex 怎样对待没有设的 `bearer_token_env_var`（不起这个服务并报错，或不带令牌连接、得到 401）；之后设回 | |

### C：读与写（两个客户端各做一遍，各在自己的笔记本里，笔记本入口）

| # | 操作 | 预期 | Claude Code 记下的 | Codex 记下的 |
|---|---|---|---|---|
| C1 | 让 agent 读 `Guide.md`，原样告诉你第 3 行 | 它答`他说“先读再写”，然后离开。`：`read` 的正文是文字块，没有 `structuredContent`（M9 总设计 4.7），客户端交给了模型 | | |
| C2 | 让 agent 在 `计划/本周/周一.md` 写一页（几行字） | 一次 `write`，缺的 `计划`、`计划/本周` 一并建成；网页的树里有三页，历史里一个变更集 | | |
| C3 | 让 agent 把 `Guide.md` 里"先读再写"改成"读过再写"（引号是弯引号） | 它照 `read` 给的字节写 `old_string`，一次成功；若它写成直引号得到 `not found`，记下它之后怎样做（重读、照提示改） | | |
| C4 | 让 agent 删掉 `Guide.md` 里"要删的一行"那一行 | 记下它的 `old_string` 与 `new_string`：只删文字、留下一个空行，还是连换行一起删（服务端只按字节；Claude Code 自己的 Edit 在 `new_string` 为空时连后面的换行一起删，M9 总设计 4.10）；结果的正文 | | |
| C5 | 让 agent 把 `Guide.md` 的 `aa` 换成 `bb` | `aa` 在 `aaa` 里出现两处（重叠也算），答 `old_string` 的 `found 2 times`；记下它是加上下文、带 `replace_all`，还是放弃 | | |
| C6 | 让 agent 把 `计划/本周/周一.md` 移到 `计划` 之下 | 它给 `move` 的 `to` 是完整的新路径 `计划/周一.md`；若它先写成 `计划`（"移进去"）得到错误，记下它之后怎样做 | | |
| C7 | 让 agent 从头读完 `长文.md`，说出最后一行 | 第一次 `read` 约 45 KiB（中文约 1.5 万字）、带 `next_offset`，它接着读完，答"第 800 行：…"；记下第一次的答复有没有被客户端截断或拒绝（Claude Code 的约 25,000 个 token，照文档） | | |
| C8 | 让 agent 读 `Guide/图一.png`，说出图里有什么 | 答复带图片，它说得出图里的内容；读 `Guide/说明.pdf` 时答复带 `http://127.0.0.1:8080/` 起头的下载地址 | | |
| C9 | 让 agent 读 `Guide/宽图.png`（2.0–2.8 MiB） | 图片直接在答复里（3 MiB、每边 8,000 像素之内）；记下客户端是否把它算进自己的输出上限、截断或拒绝 | | |
| C10 | 让 agent 依次读完 `图库/` 下的 21 张图，再读 `Guide/宽图.png`，然后接着聊一句 | 记下读过 20 张以上、其中一张宽过 2,000 像素之后，之后的请求会不会被模型的接口拒绝（一个请求里 20 张以上的图片时每张至多 2,000 像素，未核对） | | |
| C11 | 让 agent 用 `grep` 找 `TODO`，告诉你一共几处 | 第一次答复到 48 KiB 停下：85 处、带 `next_cursor`（不带 `limit`、或带的 `limit` 不小于 85 时都是这样；小于 85 时先到 `limit`，记下它带的 `limit`）；它接着调用（不带 `limit` 时共 8 次，最后一次 5 处），答 600 处；记下第一次的答复是否整个交给了模型 | | |
| C12 | 让 agent 用 `grep` 找 `\pL{30}完成` | 答复没有命中、`partial_lines` 列出 `长行.md` 第 1 行；记下它是否照说明以 `read`（`offset`、`column`）去读那一行的后面 | | |
| C13 | 让 agent 搜索"宇宙"，再搜索"宙" | 第一次以 `search` 找到 `长文.md`、`命中.md`；单个汉字只找标题，第二次什么也找不到或只有标题里有它的；记下它分不分得清 `search` 与 `grep` | | |
| C14 | 让 agent 做一次"整理"：在 `收件箱` 下建两页、改 `Guide.md` 的一行、把 `Guide/锁.md` 改名为 `Guide/已锁.md`，作为一次带说明的 `batch` | 一个变更集，历史里显示它的说明；网页上整组撤销它之后树与正文回到之前（`Guide/锁.md` 又是这个名字） | | |
| C15 | 让 agent 用 `grep` 找 `FIXME`，告诉你那一行最后的四个字 | 一处命中（`命中.md` 第 601 行），`cut: true`、带 `column`：给出的只是 `FIXME` 前后约 1,000 字节；它要再读那一行才答得出"行尾在此"：记下它是否照说明以 `read`（`offset` 601，必要时 `column`）读，还是猜 | | |

### D：审批

每一步记下：调用之前问没问、问的文字、选了"不再问"之后是否不再问。D1–D3 只对 Claude Code，D4–D8 只对 Codex，都在笔记本入口。

Claude Code 的 D1、D2、D3 各在一个只给这一步的新的临时目录里做，每个有它自己的一份 `mcp.json`：A、C 与前一步里答过的"不再问"和 D2 的放行都留在别的目录里，不论 Claude Code 把它们记在哪里（未核对）。D1 之前一次建好三个：

```sh
export CCD1=$(mktemp -d) CCD2=$(mktemp -d) CCD3=$(mktemp -d)
for d in "$CCD1" "$CCD2" "$CCD3"; do cp "$CC/mcp.json" "$d/mcp.json"; done
```

（A1 改用了 `mcp-literal.json` 的，复制它：`for d in "$CCD1" "$CCD2" "$CCD3"; do cp "$CC/mcp-literal.json" "$d/mcp.json" && chmod 600 "$d/mcp.json"; done`。）每一步在它自己的目录里启动 Claude Code（下表写着命令）。每一步若有该问而没问的工具（像是从前面带过来的放行），记下它，并记下这一步的目录里 `ls -a .claude` 与其中各个文件的内容（没有这个目录时也记下）。

每类注解只试一个或两个工具：读的工具的注解都一样（`readOnlyHint`，`api/mcp/tools.json`），所以 `read` 与 `grep`（P4 问的四个读的工具之一）；写的分两类，`write`、`move`、`delete` 是 `destructiveHint` 加 `idempotentHint`，`edit`、`batch` 只有 `destructiveHint`，所以 `write` 与 `batch`。Claude Code 只试默认的许可模式（"不碰你自己的客户端配置"记下的 `defaultMode` 不是 `default` 时，D1–D3 的结果照它读）。

| # | 操作 | 预期 | 记下的 |
|---|---|---|---|
| D1 | `cd "$CCD1"`，以 `claude --mcp-config "$CCD1/mcp.json" --strict-mcp-config` 启动，不放行任何工具，依次让它 `read`、`grep`、`write`、`batch` 一次：`read` 问时选"不再问"，别的问时只答这一次；之后再让它 `read` 一次 | 记下四个工具各问不问，与选了"不再问"之后的第二次 `read` 问不问 | |
| D2 | `cd "$CCD2"`；`mkdir "$CCD2/.claude"`，在 `$CCD2/.claude/settings.json` 写 README 的 `permissions.allow` 样例；以 `claude --mcp-config "$CCD2/mcp.json" --strict-mcp-config` 启动，同 D1（这一次每个问的都只答这一次） | 读的工具不问，`write`、`batch` 照常问（照文档） | |
| D3 | `cd "$CCD3"`（这里没有 D2 的放行）；`claude -p "读 Guide.md 的第 3 行" --mcp-config "$CCD3/mcp.json" --strict-mcp-config`；再加 `--allowedTools mcp__nervewiki__read`；再以 `-p "在 Guide 下写一页 X"`（不放行 `write`） | 不放行时记下它怎样（拒绝、报错）；放行 `read` 时答出那一行；不放行的 `write` 不调用（照文档） | |
| D4 | Codex 的 `default_tools_approval_mode = "prompt"`（README 的样例），依次 `read`、`grep`、`write`、`batch` | 记下各问不问 | |
| D5 | 退出 Codex，把 `$CODEX_HOME/config.toml` 的 `default_tools_approval_mode` 改成 `"writes"`，再启动，同 D4 | 记下 | |
| D6 | 同样改成 `"auto"`，同 D4 | 记下 | |
| D7 | 同样改成 `"approve"`，同 D4 | 都不问（M0/P1） | |
| D8 | 改回 `"prompt"`，`codex exec "读 Guide.md 的第 3 行"`；再以 `"approve"` 执行一次 | `prompt` 时要审批的调用被拒（M0/P1），记下它的输出；`approve` 时答出那一行 | |

D3 做完之后，在 Claude Code 的终端里回到 `$CC`（之后的 Claude Code 的步都在那里启动），删掉 D 的三个目录：

```sh
cd "$CC"
rm -rf "${CCD1:?}" "${CCD2:?}" "${CCD3:?}"
```

### E：HTTP 的答复与提示

| # | 操作 | 预期 | Claude Code 记下的 | Codex 记下的 |
|---|---|---|---|---|
| E1 | 不带令牌：Claude Code 用 `$CC/notoken.json`；Codex 退出，去掉 `config.toml` 的 `bearer_token_env_var`，再启动。`/mcp` | 服务端答 401，说明是"This server has no OAuth: MCP takes a personal access token in the Authorization header."；记下客户端显示的原文 | | |
| E2 | 接着 E1，看服务的日志 | 记下客户端在 401 之后请求了哪些路径：`/.well-known/` 之下（答 404；服务的日志把这里的每个请求都记作 `path=/.well-known/...`，分不出是哪一个：记下次数与状态）、`POST /register`（答 400 `invalid_client_metadata`），以及是否退到源站的 `/authorize`、`/token`（GET 是网页的 200 `index.html`，POST 是 405 `Allow: GET, HEAD`）。之后改回 E1 之前的配置 | | |
| E3 | 会话进行中在网页上撤销这个客户端的 PAT，再让 agent 读一页 | 服务端答 401 `Bearer error="invalid_token"`、"The bearer token is invalid or has expired."；记下客户端显示什么、是否转去 OAuth。之后建一个新的 PAT，以 `read -rs` 设进这个客户端的变量，重启客户端；A1 改用了 `*-literal.json` 的，也把新的令牌写进那三份（`mcp-literal.json`、`account-literal.json`、`throttled-literal.json`） | | |
| E4 | 取 A 的一个会话的访问令牌（表下的命令：密码不进 shell 的历史；没有 `jq` 时从浏览器的开发者工具里取）。把这个客户端的变量换成它，重启客户端，让 agent 读一页；之后换回 PAT、再重启。登录后 15 分钟之内做完：访问令牌过期之后答的是 401，不是 403 | 403，"MCP takes a personal access token, not a session's access token."；记下显示的原文 | | |
| E5 | 地址里的笔记本 id 换成一个随机的 UUID（`uuidgen`），重启客户端；之后换回 | 404 `notebook.not_found`；记下显示的原文 | | |
| E6 | （可选）在服务的终端里以小的桶重启：`NWIKI_RATELIMIT__AUTHENTICATED__PER_MINUTE=10 NWIKI_RATELIMIT__AUTHENTICATED__BURST=10 make run`，让 agent 连续读十几页；做完照常 `make run` | 429 `rate_limited` 带 `Retry-After`；记下显示的原文、客户端是否自己等着重试 | | |

E4 的访问令牌：

```sh
read -r EMAIL; read -rs PW
jq -n --arg e "$EMAIL" --arg p "$PW" '{email:$e,password:$p}' |
  curl -sS -X POST "$BASE/api/v0/auth/login" -H 'Content-Type: application/json' --data @- |
  jq -r .access_token
```

### F：大的请求体（账户入口）

F 部分在账户入口做：Claude Code 以 `$CC/account.json` 启动（F2 以 `$CC/throttled.json`），Codex 把 `config.toml` 的 `url` 换成 `http://127.0.0.1:8080/mcp`（F2 换成 `http://127.0.0.1:8081/mcp`），每次换了都重启客户端。每一步先让 agent 打开它的笔记本（`open_notebook`）。agent 有的说明：账户入口握手时的内置说明，其中有"超过 64 KiB 的调用每个令牌一次一个，大的写一个接一个地发"（`page/adapter/mcp/instructions.go:73-74`），与 `open_notebook` 给的 `AGENTS` 页的正文。下面 `$TOKEN` 是这个客户端的令牌（`$NERVEWIKI_CLAUDE_TOKEN` 或 `$NERVEWIKI_CODEX_TOKEN` 的值），`$NB` 是它的笔记本：在 F1 的另一个终端里以 `read -rs TOKEN && export TOKEN` 设 `TOKEN`（粘贴这个客户端的令牌，E3 之后是新的那一个），以 `export NB="$NB_CC"`（Codex 的是 `"$NB_CODEX"`）设 `NB`。

| # | 操作 | 预期 | Claude Code 记下的 | Codex 记下的 |
|---|---|---|---|---|
| F1 | 在另一个终端里（设好 `$TOKEN`、`$NB`）以这个客户端的令牌**一直**占住它的名额：照表下的"F1 的占位"，先写 `body.json`（1 MiB 的写），再起循环：每一轮约 26 秒（短于 `server.read_timeout` 的 30 秒），一轮接一轮；答了 HTTP 的错误（`--fail`）的一轮之后先停 2 秒，不空转。第二轮起 `占位.md` 已是同样的正文，`write` 答 `changed: false`（不带 `base_version` 写同样的正文不算冲突，`page/app/write_path.go:176-190`），不要紧：名额在读请求体时就占着。服务的日志里出现它的第一行 `http request`（`POST /mcp`，约 26 秒）之后，回到会话，让 agent 写一页 `大.md`，内容是"lorem ipsum dolor sit amet "重复 2,500 遍（67,500 字节）。这一步做完在那个终端里 `kill %1`（`jobs` 看它的号） | **先看日志**：应当是约 26 秒一行的 200。约 2 秒一行的 401 是错的令牌，约 26 秒一行的 404 是错的 `$NB`（它照样占着名额，却不写页）：先 `kill %1`，改正 `$TOKEN` 或 `$NB` 再起：错的令牌每一轮都耗认证失败的闸门（按客户端 IP、在认证之前取，每分钟 60 个，`platform/httpserver/api.go:346-350`），耗完了这台机器上每个带令牌的请求（agent 的与浏览器的）都答 429。agent 的调用发出时占位的那一轮必须正在传：服务的日志里，agent 的那一行 `POST /mcp` 之前有一行 `large request bodies are saturated`，`own=true`，`holders` 里是这个令牌（`Credential:pat:<id>`；dev 的日志是文字的格式）。然后它多半（约九成：占位的一轮在那 2 秒里答完时它拿得到名额）等 2 秒后答 HTTP 503 `server_busy`（`Retry-After: 1`）；记下客户端怎样（重试、把错误交给模型、断开）、agent 之后是否等一等再发。agent 的调用答 200 时记下它落在占位的两轮之间 | | |
| F2 | 起一个限速的转发（下面的脚本，`node throttle.mjs 16384`：客户端到服务的方向每秒 16 KiB），客户端换到经它的账户入口（上面），让 agent **同时**写两页，各是 F1 那样的 67,500 字节 | 每个约 4 秒才传完。看服务的日志里这两个 `POST /mcp` 的时刻与 `duration`：重叠的（第二个在第一个答完之前开始），第二个等 2 秒后 503，记下 agent 看到什么、是否照说明一个接一个地重发；一个接一个的，记下这个客户端是一个接一个地发写的（那它碰不到每个令牌一个名额的代价）。两个参数合起来约 3 万个 token 的输出，客户端若写不出（输出的上限，未核对）也记下 | | |
| F3 | F2 之后看历史；停掉转发（Ctrl-C），客户端换回笔记本入口 | 两页都写成了（重发的那一次成功），各一个变更集 | | |

F1 的占位（另一个终端，在放 `seed.mjs` 的临时目录里，照上面设好 `BASE`、`TOKEN`、`NB`；表格里不写命令，免得从原文复制时带上转义）：

```sh
node -e 'process.stdout.write(JSON.stringify({jsonrpc:"2.0",id:1,method:"tools/call",params:{name:"write",arguments:{path:"占位.md",content:"y".repeat(1<<20)}}}))' > body.json
while :; do
  curl --fail -sS -o /dev/null --limit-rate 40k "$BASE/mcp?notebook=$NB" \
    -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2025-11-25' \
    --data-binary @body.json || sleep 2
done &
```

F2 的转发（存成 `throttle.mjs`，`node throttle.mjs <每秒字节数>`；Ctrl-C 停）：

```js
// throttle.mjs：127.0.0.1:8081 转到 127.0.0.1:8080，客户端到服务的方向限速
import net from "node:net";
const rate = Number(process.argv[2] ?? 16384);
net
  .createServer((client) => {
    const server = net.connect(8080, "127.0.0.1");
    server.pipe(client);
    client.on("data", (chunk) => {
      client.pause();
      server.write(chunk, () => setTimeout(() => client.resume(), (chunk.length / rate) * 1000));
    });
    client.on("end", () => server.end());
    const close = () => {
      client.destroy();
      server.destroy();
    };
    client.on("error", close);
    server.on("error", close);
  })
  .listen(8081, "127.0.0.1", () => console.log(`127.0.0.1:8081 → :8080, ${rate} bytes/s up`));
```

### G：服务重启

| # | 操作 | 预期 | Claude Code 记下的 | Codex 记下的 |
|---|---|---|---|---|
| G1 | 交互的会话里让 agent 读一页；在服务的终端 Ctrl-C 停掉 `make run`、再 `make run`（同一把签名私钥：浏览器不必重新登录）；回到会话，让 agent 再读一页、再写一页 | 不重新连接就成功（服务端没有会话：日志里没有新的 `MCP initialize`，或客户端自己又握了一次手，记下是哪一种）；写的变更集的客户端仍是 A1、B1 记下的名称 | | |
| G2 | 同 G1，但服务停着时就让 agent 读一页，再启动服务，再让它读 | 记下停着时显示什么、之后能不能不重启客户端就恢复 | | |

`claude -p` 与 `codex exec` 每次都是新的进程、新的握手，测不到重启：G 部分要交互的会话。

### H：编辑锁

| # | 操作 | 预期 | Claude Code 记下的 | Codex 记下的 |
|---|---|---|---|---|
| H1 | B 在网页上打开这个客户端的笔记本里的 `Guide/锁.md`（C14 的撤销之后它又是这个名字）并进入编辑；让 agent 改它的一行 | `page.locked`：文字说出 B 的显示名、"try again after they leave edit mode"、"cannot take the lock over"；记下 agent 怎样转述、是否反复重试 | | |
| H2 | B 离开编辑；A 自己在网页上编辑同一页；再让 agent 改它 | `page.locked`，文字说"by you, in a browser" | | |
| H3 | A 仍在编辑；让 agent 删除 `Guide.md`（带子页） | 不带 `recursive` 的第一次先答 `validation_failed`，说出它下面有几页、几个附件（数在锁之前）；带了 `recursive` 答 `page.locked`，列出子树里被编辑的那一页；Guide 与它的子页都还在 | | |

### I：反向代理

上游是本机的服务：容器里是 `host.docker.internal:8080`（Docker Desktop 把它转到本机的回环地址，未核对）。连不上时跳过容器的路（I1、I2，与 I3 的容器）：P5 已在隔离的容器里核对过 README 的两份样例（P5 设计 5.2）。**不要**让服务听 `127.0.0.1` 之外的地址。

- nginx：把 README"部署"一节的 nginx 样例原样存成 `nginx.conf`，只在 `server_name` 之后加一行 `listen 8090;`，四个 `location` 的 `proxy_pass` 都换成 `http://host.docker.internal:8080;`；然后 `docker run --rm -d --name nw-nginx -p 127.0.0.1:8090:8090 -v "$PWD/nginx.conf:/etc/nginx/conf.d/default.conf:ro" nginx:1.29-alpine`（样例是一个 `server`，放在 `conf.d` 里）。
- I2、I3 的写：`node -e 'process.stdout.write(JSON.stringify({jsonrpc:"2.0",id:1,method:"tools/call",params:{name:"write",arguments:{path:"代理.md",content:"z".repeat(200000)}}}))' > body200k.json`（200 KB）与一个约 200 KB 的文件 `200k.bin`（`head -c 200000 /dev/urandom > 200k.bin`）。

| # | 操作 | 预期 | 记下的 |
|---|---|---|---|
| I1 | Claude Code 经 nginx 连（一份 `mcp.json` 的地址换成 `http://127.0.0.1:8090/mcp?notebook=…`），让 agent 读 `Guide.md` 的第 3 行（C3 改过它：答的是现在的那一行），再写一页 `经 nginx.md`（几行字；不靠 C 部分留下的页）；浏览器经 nginx（`http://127.0.0.1:8090/`）打开这一页的阅读视图（这是另一个源：网页的登录不带过来，在那里再登录一次），再让 agent 改它 | 都成；阅读视图不重新加载就显示 agent 的改动（事件流经 nginx 照常） | |
| I2 | 缓冲：`curl -sS -o /dev/null --limit-rate 20k "http://127.0.0.1:8090/mcp?notebook=$NB_CC" -H "Authorization: Bearer $NERVEWIKI_CLAUDE_TOKEN" -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2025-11-25' --data-binary @body200k.json`，再把地址换成 `$BASE` 直接发一次；经 nginx 以 `--limit-rate 20k` 上传 `200k.bin`（`curl -sS -o /dev/null --limit-rate 20k "http://127.0.0.1:8090/api/v0/notebooks/$NB_CC/assets" -H "Authorization: Bearer $NERVEWIKI_CLAUDE_TOKEN" -F file=@200k.bin`，传到笔记本的根下） | 服务的日志里：经 nginx 的 `/mcp` 那一行的 `duration` 很短（nginx 收完才转来），直接的约 10 秒；上传那一行的 `duration` 约 10 秒（不缓冲，服务端边收边写） | |
| I3 | 你的部署前面是 Caddy 时，在那里确认：照 README 的 Caddy 样例配置，同 I2（经 Caddy 的地址；上传另起一个名字：`-F 'file=@200k.bin;filename=200k-caddy.bin'`，免得撞上 I2 传到根下的 `200k.bin`，同名的上传在读文件之前就答 `page.title_taken`），看服务的日志。没有 Caddy 的部署时可以以容器代替：README 的 Caddy 样例存成 `Caddyfile`，`wiki.example.com` 换成 `:8091`、两处上游换成 `host.docker.internal:8080`，`docker run --rm -d --name nw-caddy -p 127.0.0.1:8091:8091 -v "$PWD/Caddyfile:/etc/caddy/Caddyfile:ro" caddy:2.10-alpine`；都没有时跳过（P5 已在 Caddy 2.10.2 上核对过这个写法，P5 设计 5.2）。然后照表下的"I3 的被拒的写"，以一个错的令牌把一个 8 MiB 的写经 Caddy 发到 `/mcp`，再直接发一次 | `/mcp` 那一行的 `duration` 很短、上传的约 10 秒，同 I2 经 nginx 的。被拒的大的写：直接的答 401（服务端在读请求体之前就答）；经 Caddy 的记下状态码与头。替身量过（P5 设计 5.2）：在读请求体之前答的 503、401、429，8 MiB 时经 Caddy 多半成了 502；503 的那些没有 `Retry-After`，401 的头替身没有记。这一步是在你的部署上、以真的服务端核对 | |

I3 的被拒的写（`CADDY` 是经 Caddy 的地址，如 `http://127.0.0.1:8091`）：

```sh
head -c 8388608 /dev/zero | tr '\0' 'a' > big.bin
for addr in "$CADDY" "$BASE"; do
  echo "== $addr"
  curl -sS -o /dev/null -D - -w '%{http_code}\n' "$addr/mcp?notebook=$NB_CC" \
    -H 'Authorization: Bearer nwk_pat_wrong' -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' --data-binary @big.bin
done
```

## 做完

1. **Codex 的登录**：照"不碰你自己的客户端配置"登录的第 3 种复制过 `auth.json` 的，先照那里的规则决定复制不复制回去（看 `shasum` 的两个指纹），**再**删临时目录；用第 2 种的，在一个没有设 `CODEX_HOME` 的终端里启动你自己的 Codex，记下它是否照常可用，不能用时在那里 `codex login` 再登录一次。
2. 在网页上撤销 `seed`、`claude-checklist`、`codex-checklist` 与 E3 新建的 PAT（服务还开着）。
3. 停掉 `make run`、`throttle.mjs`（若还在）与代理的容器：`docker stop nw-nginx nw-caddy`。
4. **Claude Code 的会话**：Claude Code 在 `~/.claude/projects/` 下为这份清单的临时目录（`$CC` 与 D 部分的几个）记了会话。它们可以留着，不碍事。这份清单不给删除它们的命令：想删掉的，先核对目录的名字确是这份清单的临时目录的，再自己手动删掉那一个目录。Claude Code 怎样给这些目录起名未核对。
5. 删掉临时目录：`$CC`（含几份 `*-literal.json`，若有）、`$CX`、`$CODEX_HOME`、`$KEYS` 与放 `seed.mjs`、`body.json`、`big.bin` 的那一个；关掉设过令牌的终端（或 `unset NERVEWIKI_CLAUDE_TOKEN NERVEWIKI_CODEX_TOKEN SEED_TOKEN TOKEN EMAIL PW`）。

## 不在这份清单里的

- 本地进程的（stdio）MCP 服务、OAuth：服务端没有它们（M9 总设计第 2 节"不做"）。
- 2026-07-28 的协议（`server/discover`、`subscriptions/listen`）：等真实的客户端发它再开（M9 总设计第 11 节主持者的决定第 2 条）；客户端若以它连接而被拒，记在"记下的"里，交负责人。
- Claude Code 的别的许可模式（`acceptEdits`、`bypassPermissions` 一类）：README 不推荐它们，是负责人自己的选择。
- prompts（ingest、query、lint）：M10 登记。
- 多个实例：v0.1 只支持单实例。
````

### 6.2 与 4.16 的写法不同的几处

- **一个客户端一本笔记本**：第 C 部分改写种下的页（C3 改了"先读再写"、C4 删了一行、C2 与 C6 建了又移了 `计划/周一.md`），同一本里做第二遍时它的预期一条也对不上，记下的会被读成客户端的失败。两本各种一次，`seed.mjs` 不变；H 部分照样各在自己的那本里。
- **F 在账户入口**：种下的 `AGENTS` 页在笔记本入口整个取代内置的说明，而"大的写一个接一个地发"只在内置的说明里（`page/adapter/mcp/instructions.go:73-74`；`api/mcp/tools.json` 的 14 个工具的说明里没有 64 KiB、503 或"一个接一个"）。账户入口的握手照常给内置的说明（`instructions.go:19-27` 的 `AccountInstructions` 含 `pathsAndTools`）。C 部分的"照说明"靠的是工具的说明（`edit` 的引号与唯一、`move` 的完整路径、`grep` 的 `partial_lines`、`cut` 与 `column`），在笔记本入口照样成立。
- **F1 的占位是一个循环**：一次 1 MiB 的慢的写只占名额约 20 秒，而 agent 要先写出约 1.5 万个 token 的参数才发出调用，往往要几分钟，那时占位的早已答完、名额空着，F1 就什么也没测到。一轮接一轮、每轮约 26 秒（短于 30 秒的读期限，不会因超时被断开）的循环几乎一直占着；日志里的 `large request bodies are saturated`（dev 的文字格式写作 `own=true`，`holders` 里是 `Credential:pat:<id>`，S1 的审查 B-N7）证明 agent 的调用来时它正占着（`own` 由 `takeLarge` 定，`platform/httpserver/largebody.go:188-226`；满了时记 info 的是 `saturated`，`:243-252`）。循环里的 `--fail … || sleep 2` 是刹车：一个打错的 `$TOKEN` 或 `$NB` 让每一轮几毫秒就答完，没有它会空转、耗光认证失败的闸门或这个令牌的桶。也可以借客户端的审批（客户端问要不要调 `write` 时才起占位、15 秒内同意），但那要审批没有放行，与 D 部分的状态纠缠，所以用循环。
- **F2 的大小与并行**：P2 第 8 节写"1 Mbit/s，两个各约 1 MiB 的并行的 `write`"。模型逐个 token 写出工具的参数，1 MiB 的正文约 25 万个 token，真实的 agent 写不出两个。要的是形状：两个并行的、超过 64 KiB 的写，上行慢到一个要传 2 秒以上。所以用 67,500 字节的正文（加上 JSON 的外壳仍过 65,536 字节，模型写得出：约 1.5 万个 token）与每秒 16 KiB 的转发（一个约 4 秒）。两个客户端会不会同时发两个写的调用没有记录（P2 的设计审查已把"Claude Code 并行地发大的写"划为未核对，`reviews/P2-design-review.md:274`），所以 F2 的预期按日志分两种，一个接一个也是一个结果。
- **限速的做法**：清单不叫负责人改系统的网络（`pfctl`、`dnctl` 一类要 sudo、改的是整台机器）；一个 20 行的 Node 转发只限这一个端口，Ctrl-C 就没了。
- **重启**：4.16 写"交互的会话，或让 agent 先执行 `sleep 30` 再调用工具"。清单用交互的会话（G1）：`sleep` 的写法要 agent 有执行命令的权限，Codex 的沙箱与 Claude Code 的审批都会挡，结果混进审批的变量。dev 不配置签名私钥时每次启动换一把（README"签名私钥"），所以第 1 步生成一把，重启（E6、G1、G2）之后两个浏览器不必重新登录；PAT 只存哈希，本来就跨过重启（`identity/domain/api_token.go:16-31`）。
- **只试默认的许可模式**：4.16 写"Claude Code 的许可模式"。README 只推荐默认的（读的工具放行、写的工具照常问），别的模式（`acceptEdits`、`bypassPermissions`）是负责人自己的选择，README 不写它们对 MCP 工具的作用；所以 D 只试默认的模式、一份 `permissions.allow` 与 `-p`，并先记下负责人自己的 `defaultMode` 与 `mcp__` 的规则（它们会让 D1 的意思变了）。每类注解一两个工具的理由在 D 的开头。
- **截取的行**：P4 第 8 节问 agent 是否以 `read(offset, column)` 读"截取的行与部分匹配的行"。部分匹配的行是 `长行.md`（C12）；截取的行要一行过 `MaxGrepLineText`（1,000 字节，`page/domain/grep.go:31`）的命中，原来种下的行都短（`命中.md` 的 TODO 行约 523 字节（S1 的修复一轮为 C11 加长，下一条），`长文.md` 的约 87 字节），所以 `命中.md` 加第 601 行（约 3,000 字节、`FIXME` 在中间、行尾是别处没有的"行尾在此"）与 C15。
- **I3 是确认**：Caddy 的写法 P5 已在 Caddy 2.10.2 上核对过（第 5 节、5.2），清单只在负责人的部署上确认它（或以本机的容器代替）。
- **加的几步**：A4、B4（变量没设时）与 I1–I3（反向代理）是 P5 加的：README 的样例都靠环境变量，第 5 节的样例改了结构；它们的结果都直接改 README 的某一句（4.2、5.2）。
- **E6 是可选的**：要以小的桶重启实例；429 的提示与 401、403 同是"HTTP 的答复"一类（4.4），E1–E5 已记下客户端怎样显示这一类。
- **C11 的种子的行长**（S1 的修复一轮，审查 B-M2）：`grep` 的 `limit` 默认 100（`page/domain/grep.go:29`），一次调用到它就停；原来每行约 164 字节时第一次答复停在 100 个命中、约 21 KB，48 KiB 的那一半从没走到，只有 agent 带 230 以上的 `limit` 才走到。每行加长到约 520 字节（仍在 `MaxGrepLineText` 之下，不截）：不带 `limit` 时第一次答复停在 48 KiB、85 个命中，600 个 8 次调用（实施者以一次不提交的探针经真的服务端量）；C11 记下 agent 带的 `limit`。
- **图片是名称说的类型**（审查 B-M3）：服务端按开头的字节判类型，`.png` 里是 JPEG 的字节时存作 `application/octet-stream`、不内联（`asset/domain/types.go` 的 `TypeOf`），C9、C10 就因服务端的原因失败；所以清单要 PNG 与 PDF，给 `sips` 的转法与 `file` 的核对。
- **D 的每一步一个新的临时目录**（审查 B-M1，修复核对一轮 B2-N1、B2-N2）：C 与 D1 答的"不再问"、D2 写的放行，不论 Claude Code 记在项目的设置里还是 `~/.claude.json` 里（未核对），都会带进下一步，让"不放行"的一次与放行了的分不出，4.2 的 D3 一行就照一次从没不放行过的运行改 README。D1、D2、D3 各在一个新的目录里、各有自己的一份 `mcp.json`，就不靠它记在哪里；D 做完以 `${VAR:?}` 守着删掉三个目录、回到 `$CC`。
- **不给删除会话的命令，A3 数一数**（审查 C 的存疑 1 与 B 的存疑 1，修复核对一轮 B2-M1 = C2-M1）：Claude Code 为临时目录在 `~/.claude/projects/` 下记的会话可以留着；一条在 `~/.claude/` 之下的 `rm -rf`，一次失手（多一个空格、一个没设的变量）就删掉负责人每个项目的会话与记忆，所以清单不带它，要删的人自己核对名字、手动删那一个目录。A3 的项目范围的命令照文档不写 `~/.claude.json`，没有一步看过：前后各数一次 `"nervewiki"`，多了时记下、照未核对的 `claude mcp remove` 去掉。
- **I 部分不靠前面的部分**（审查 B-M4、B-N9，修复核对一轮 B2-N3，B 的存疑 2）：I1 写它自己的页，I3 以另一个文件名上传（同名在读文件之前就答 `page.title_taken`），I1 的浏览器在 `:8090` 上另登录一次（另一个源）；`host.docker.internal` 到不到只听 127.0.0.1 的服务未核对，不通时跳过容器的那几路（P5 已核对过样例），服务从不绑到 127.0.0.1 之外。

## 7. 总体设计与 00 号文档的修订

照 P1–P4 的做法：每处以脚本落下，每段原文在文件里恰好出现一次才改，否则停下报告。设计的提交只带 `scratchpad/m9/p5/apply-00m9.py`，它只落 2.1 的 13 条与 00 号文档第 12、13 节的两行（7.1 的第一段）；第 7 节其余的各处（7.1 的 S1 合并之后的各处、7.2 的 `apply-v01.py`、7.3）与第 11 节的四份移交都在 S1 合并之后的文档提交里（第 8 节第 13 步），那时照实际合并的样子写。

### 7.1 00 号文档

**随这份设计的提交**（审查之后）：

- 2.1 的 13 条（第 3 节 `:67`；"总体设计的修订"的 13.4 第 3 条 `:493`；第 9 节 `:539`、`:550`、`:557`、`:574`、`:577`、`:589`、`:592`、`:598`；第 10 节 `:612`；4.16 的"接入文档"（`:384` 与它的末句）与"反向代理"（`:385`）两条；4.4 `:122`）。
- 第 12 节的 P5 一行：Phase 文档 [05-P5](05-P5-stories-docs.md)，状态"进行中（设计已审查）"，审查 [设计](reviews/P5-design-review.md)；第 13 节加一行（日期、"P5 设计：……"、依据）。

**S1 合并之后的文档提交**：

- 第 12 节的 P5 一行：状态"完成"，审查加 [S1](reviews/P5-S1-stories-docs-review.md)，合并写 S1 的提交；"收尾"一行照旧"未开始"。第 13 节加合并的一行。
- "移交的落实"的 M0/P1 一行（`:443`）：第 1 项的"真实客户端进人工清单，重启要一个跨过重启还活着的客户端进程（4.16，P1、P5）"后加"（P5：清单的 G1、G2）"；第 3 项的"各审批选项的效果进人工清单、写进接入文档（P5）"后加"（P5：清单的 D 部分，README 的'审批'；每个选项问什么等清单跑过之后写）"。**这一行与 [M0/P1 的移交](handoffs/M0-P1-mcp-sessions.md)都不改为 `done`，M9 收尾时也不**：它们要的是在 Claude Code 与 Codex 上量过，落实是清单的 G、D 部分，清单在 v0.1 的最后才跑；跑过之后照记下的关，同 M9 自己的状态"实现完成，待人工验收"（第 11 节）。
- "写给后面的 M"的 M12（`:469-470`）：上限一条"48 KiB 与 3 MiB 两个上限按真实客户端的反馈再看"后加"（人工清单的 C7、C9、C10 记下的）"；打磨一条的"部署文档沿用 4.16 的反向代理说明"改为"部署文档沿用 README 的部署一节（按路由的 nginx 与 Caddy 的样例，M9/P5）；README 的'接入 agent（MCP）'是网页上接入的帮助的底稿"。两条都由 P5 的文档提交写进 M12 的移交（第 11 节）。
- "总体设计的修订"的 10.1 与 11 两条照 7.2 写成已落下的样子。
- 实施中若有与这份设计不同之处，照实改 2.1 或第 3 节，并在 3.9 列出（同 P4 的 3.18、3.19）。

### 7.2 v0.1 总体设计（S1 合并之后的文档提交，`apply-v01.py`）

- **6.3**（`v0.1-design.md:654`）整行改为："- Claude Code、Codex 的接入配置写进 README 的'接入 agent（MCP）'一节（M9/P5）：一个 agent 一个 PAT，令牌放在环境变量里（每个客户端一个变量），会提交的 `.mcp.json` 只写环境变量的展开。Codex 的非交互模式默认拒绝需要审批的工具；接入文档说明各审批选项的含义，不一律推荐 `approve`：读的工具可以免问，写的工具照常问，无人值守地写的 agent 由人在网页的历史里检查、撤销；每个选项对读与写的工具问不问，由人工清单（`M9-mcp/manual/P5-mcp-clients-checklist.md`）记下之后写，在那之前接入文档里客户端的每一句标出它的来处（照文档、未核对）。"
- **10.1**（`:887`）在"……列在 `[M9 总设计](M9-mcp/00-M9-design.md)`第 9 节。"之后加："前面 M 的故事的 MCP 版本（M9/P5）在故事自己的文件里，种下的数据照接口版本经 REST；收编辑会话的断言（MCP 不开会话）以版本的断言代之，接口版本不调断言函数的，MCP 版本做同样的检查。真实的客户端（Claude Code、Codex）由人工清单验收，与 M4 的输入法清单一起在 v0.1 的最后执行。"
- **11**（`:896`）的最后一句"`/mcp` 是普通的 JSON 答复，不是 SSE：`client_max_body_size` 不小于它的上限（约 36 MiB）与附件、导入包的上限中较大的，`proxy_read_timeout` 不短于 45 秒，`Host` 照常转发（M9）。"改为："`/mcp` 是普通的 JSON 答复，不是 SSE（`X-Accel-Buffering: no` 只在事件流上）。请求体按路由处理：`/mcp`（上限 37,814,272 字节）与页面正文的两个写（31,522,816 字节）缓冲请求体，慢的发送者就占不住大的请求体的名额；上传与导入不缓冲，上限是导入包的上限加 64 KiB；读超时不短于 45 秒，`Host` 照常转发。nginx 默认缓冲，样例显式写出；Caddy 默认不缓冲，样例以 `request_buffers` 只为这三个路由缓冲（M9/P5 在 Caddy 2.10.2 上核对过；经 Caddy 时服务端在读请求体之前的拒绝对几 MiB 的请求体常成 502、没有 `Retry-After`）；缓冲的请求体在认证之前收在代理那里（nginx 过了缓冲区写磁盘，Caddy 在内存里），给代理留出地方（M9/P5，README 的部署一节）。"
- **13.4 第 3 条**（`:1224`）在"（`[M9/P2](M9-mcp/02-P2-writes-history.md)` 第 4 节）。"之后加（MC5–MC7 是 P3、P4 没写进这里的，一并补上）："M9/P3 加 MC5（agent 的 batch 与网页的整组撤销）；M9/P4 加 MC6、MC7（与 REST 的同一个读比较，MC6 的一次写照 MC3 核对库）。M9/P5 给前面 M 的 21 个故事各加一个 MCP 版本，名为'<故事> (MCP)'、在故事自己的文件里（C4 的做法），种下的数据照接口版本经 REST：调接口版本调的每一个断言函数，值由 MCP 的答复造出（`fixtures/mcp.ts` 的 `writtenOf` 与 `readPage`；`expectContentWritten`、`expectMoved` 收 `Pick`），从不由库造期望的值；收编辑会话的 id 的断言（PG7、PG8）以 `expectVersions` 代之；接口版本不调断言函数的（PG11、PG12、PG14、C7、SE1、SE2）做同样的检查，SE1 的 MCP 调用处以 `{marked: false}` 不比命中的范围（MCP 的搜索不给它），另与 REST 的同一个搜索比路径与数；都不开浏览器，C7 的 MCP 版本以 `openEvents` 收帧，最后以一次写的帧证明流是安静的（`[M9/P5](M9-mcp/05-P5-stories-docs.md)` 第 3 节；MC3 的活动由 P5 挪进 PG14 (MCP)）。"
- **第 15 节**加一行：`| <合并的日期> | M9/P5：6.3 的接入配置与审批、10.1 的前面 M 的故事的 MCP 版本与真实客户端的验收、11 的反向代理按路由、13.4 第 3 条（MC5–MC7 与前面 M 的故事的 MCP 版本） | P5 的设计、实施与审查：[M9/P5](M9-mcp/05-P5-stories-docs.md)、[S1 审查](M9-mcp/reviews/P5-S1-stories-docs-review.md) | M9/P5 |`
- 13.1、13.2 不改：P5 没有服务端的代码，e2e 的约定（安静的页、`openEvents`、按表的断言）照旧；6.1、6.2、6.4 不改。

### 7.3 别的文件

- **README**（S1 的提交，第 8 节）：4.1 的整节、`:219` 一行（第 4 节）；5.1 替换 `:344-367`，升级的一条加在 `:334` 之后（第 5 节）。
- **人工清单**（S1 的提交）：`docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md`，6.1 的全文。
- **M12 的性能移交**（`docs/v0.1/M12-release/handoffs/M4-performance.md` 第 18 项，S1 合并之后的文档提交）："慢的发送者"一条（`:95`）的"取了名额的请求在读完请求体之前一直占着它，请求体只受 `server.read_timeout`（30 秒）管"改为"取了名额的请求从读请求体之前一直占着它到答完：请求体至多 `server.read_timeout`（30 秒），之后这次调用的工作至多 `server.request_timeout`（15 秒），合起来至多约 45 秒"；"管理员能撤销那个 PAT 或结束会话"改为"由 `api_tokens` 或 `auth_sessions` 那一行的 `user_id` 找到账户，请他撤销那个令牌，或以 `nervewiki users reset-password` 撤销这个账户的全部会话与 PAT（没有按令牌或会话的管理员命令）"；"（M9/P5 的接入文档按路由写）"改为"（README 的部署一节按路由写，nginx 与 Caddy 的样例，M9/P5）"（2.1 第 9 条）。同一项再加两条（第 11 节）。
- **M12 的另两份与 M10 的移交**：第 11 节逐份写出文件与文字，P5 的文档提交写进去。
- **这份设计**：S1 合并之后，状态、3.9（实施与设计的不同）与第 12 节的结果。

## 8. 实施步骤

S1（分支 `m9-p5-s1`，从 `main` 起）。每个提交都是绿的：`make lint`（含 `pnpm -r run check:types`，e2e 的是 `tsc --noEmit`，`e2e/package.json`；oxlint 零警告）、`make knip` 通过，提交里新的与改过的测试通过（`pnpm exec playwright test <文件>`，在提交的说明里写明跑过哪些）；改一个签名的提交同时改它的每个调用者，加一个导出的提交同时有它的使用者（knip 也数没人用的导出的类型）。循环里的 `await`（PG11 的按文件夹走、L4 与 L6 的翻页）照 MC2 加 `// oxlint-disable-next-line no-await-in-loop` 与理由（`mc2-read-list.spec.ts:92`）。

1. `test(e2e)`：fixture 的放宽与 `readPage`（3.1 的第 1、2、4 条）：`expectContentWritten` 与 `Embedded.written` 收 `Pick<Page, "id" | "revision" | "content_updated_at">`、`expectMoved` 收 `Pick<TreeNode, "id" | "parent_id">`（各自的注释写"as the API answered its write, or as a story builds it from an MCP answer"）；MC2 的 `readPage` 与 `PageRead` 挪进 `fixtures/mcp.ts`、都导出，MC2 改为导入两者（它的 `readAll` 的签名写着 `PageRead`）。今天的调用者不改；`make lint` 的 tsc 证明全部编得过；MC2 通过。
2. `test(e2e)`：页面的写的 MCP 版本：`writtenOf`（3.1 的第 3 条）与 PG1、PG2、PG3、PG4、PG7、PG8、PG9 (MCP)；PG2 的 NFD 的标题、PG9 的六种正文提到文件的顶层，两个测试共用。
3. `test(e2e)`：PG11、PG12 (MCP)。
4. `test(e2e)`：PG14 (MCP)：MC3 (activity) 挪进 `pg14-notebook-activity.spec.ts`、改名、第二次写改为 `edit`、本地的 `Edited` 带 `at`；MC3 去掉它、只有它用的导入与文件开头注释的末句（`mc3:18-19`，3.2 的 PG14）。
5. `test(e2e)`：C7、L1、L3、L4、L6 (MCP)；同一个提交里 C7 (API) 加删除之后的安静的检查、L6 (API) 在开流之前 `settleEvents`（3.3、3.4：它们的洞与 MCP 版本的同一个，各一两行）。
6. `test(e2e)`：AS2、AS3、AS4 (MCP)。
7. `test(e2e)`：HI1、SE1、SE2 (MCP)；HI1 的四个正文常量（`created`、`first`、`second`、`third`）提到顶层，HI1 (page) 的三个同名的纯文本常量改名为 `plainCreated`、`plainSecond`、`plainThird`（oxlint 的 `no-shadow`；建成的，原写"两个"，3.9）；SE1 的 `expectFound` 加一个重载 `expectFound(q, found: Omit<Found, "marked">, options: {marked: false})`，`Found.marked` 照旧是必须的，原来的签名不变；MCP 的调用处传 `mcpFoundOf` 造的 `Omit<Found, "marked">` 与 `{marked: false}`，不造假的 `marked: []`（3.7）。
8. **反向对照**：照 9.2 逐个做变异（每个之后还原，不提交），确认右边的测试失败，结果写进审查记录的"反向对照"与第 12 节。
   - 服务端的变异：改了之后先 `make build`，再跑它的测试：故事跑的是 `bin/nervewiki`，`make e2e` 先构建（`Makefile:80-81`），直接的 `pnpm exec playwright test <文件>` 不构建，跑的是上一次的二进制，变异就读作"没抓到"。
   - 语句的变异改在 sqlc 生成的常量里（`gen/*.sql.go`，9.2 写明位置）：只改 `queries/*.sql` 不改二进制；也不在改着变异时执行 `make gen*`，那会把变异写进生成的文件、混进之后的提交。还原之后 `make gen-check` 是干净的。
   - `writtenOf` 自己的检查（令牌的节点与版本号、`at` 不为 null）以一次伪造的答复证明：临时在一个测试里交给它一个 `version` 以别的节点的 id 起头的答复与一个 `at: null` 的答复，看它各抛出一次（照 P3 对 `expectNoAsideNames` 的做法，`e2e/fixtures/assert/page.ts:261-270`）；只记进审查记录，不提交。
9. `docs(readme)`：4.1 的"接入 agent（MCP）"一节与 `:219` 的一行。
10. `docs(readme)`：5.1 的部署一节的反向代理与升级的一条。第 9、10 步的 README 要过 `pnpm run check:format`（`oxfmt --check .`；`.oxfmtrc.json` 只忽略 `docs/**`，根下的 README 照常检查）：格式化只动空白与表格的对齐时照它，动了文字的意思时报告主持者。
11. `docs(M9)`：人工清单 `docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md`（6.1）。它的链接（00 号文档、各 Phase、M4 的清单、M0/P1 的移交；清单不链到 M5/P2，S1 的审查 C-N3）与 README 到它的链接都要解析得到（`make lint` 的 Markdown 检查若不查相对链接，实施者逐个打开核对，写进审查记录）。
12. 门禁（9.3），含合并之前的 Caddy 的重跑；推送分支，CI 的各份都是绿的；e2e 那一份的用时写进审查记录（9.4）。
13. 审查（Opus：e2e 与文档各一位，第三位核对完成标准与 2.3 的处置表）、修复、核对、合并、推送 `main`（负责人的规则：完成并核对过的单元由主持者推送）；之后的文档提交：第 7 节的 S1 之后的各处、第 11 节的移交、这份设计的状态与 3.9、第 12 节、审查记录。设计的提交的 `apply-00m9.py` 只落 2.1 的 13 条与 00 号文档第 12、13 节的两行；第 7 节其余的各处（7.1 的 S1 合并之后的各处、7.2 的 `apply-v01.py`、7.3）与第 11 节的四份移交都在这个文档提交里，不在设计的提交里。

不在 S1 里的：人工清单的那一遍（第 11 节）；它之后 README 照 4.2 的改动，由负责人或主持者在那一遍之后另一个提交里做，写明照清单的哪一步。

## 9. 测试与验证

### 9.1 故事与它们的断言

每行的"共用的断言"是 API 版本调的、MCP 版本照样调的（3.1）；"另加的"是 API 版本没有、MCP 版本多做的；时间是本机的估计（REST 的准备占大头），都不开浏览器。

| 故事 | 共用的断言 | 不共用的（为什么） | 另加的 | 约 |
|---|---|---|---|---|
| PG1 (MCP) | `expectNewPage` ×2（`created` 由答复与字面的标题造） | — | 四个路径的拒绝落在 `path`（含两端的空白），比 `[field, code, message]` | 0.3–0.6 秒 |
| PG2 (MCP) | `expectRenamed` ×3；`STRASSE.md` 的同名的写被拒（API 版本的 `twin`） | — | `expectChangesetOf`（客户端） | 0.3–0.5 秒 |
| PG3 (MCP) | `placeOf`、`expectMoved` ×3（放宽的 `Pick`，目的地由测试给） | 第 1 次 `expectMoved`（排序：`move` 没有位置） | `expectChangesetOf`；两次换父页都在最后（A 的子页、根下的次序） | 0.4–0.8 秒 |
| PG4 (MCP) | `expectSubtreeDeleted` | — | 没有 `recursive` 的拒绝的整个文字与 `fields`；两个锁按子树的次序、本人的 `you: true`；过期的会话不再挡 agent 的删除；`expectChangesetOf`、`expectDeletionRevisions`、`expectTrashRoot(…, {alone: true})` | 0.4–0.7 秒 |
| PG7 (MCP) | `expectContentWritten`（`writtenOf`） | `expectOneSessionRevision`、`expectSessionGone`（会话），以 `expectVersions` 代之 | 没有编辑会话（`sessionsOf` 为空）；`read` 的版本与字节 | 0.3–0.5 秒 |
| PG8 (MCP) | `expectContentWritten` | `expectOneSessionRevision`（会话），以 `expectVersions` 代之 | 过时的 `edit` 也被拒；`current_version` 等于重读的版本 | 0.3–0.5 秒 |
| PG9 (MCP) | `expectContentWritten` ×6 | — | `read` 逐字节、`byte_size`、`total_lines` 是 `[4, 1, 3, 1, 3, 1]` | 0.3–0.6 秒 |
| PG11 (MCP) | （API 版本没有断言函数）祖先与树的次序的字面的值 | — | 按键读、答存下的名称；祖先不对时 `page.not_found`；与 REST 的 `getPage`、`listNodes` 比 | 约 0.3 秒 |
| PG12 (MCP) | （没有）内联的 `nodes` 的语句，原样；读者的六个 `forbidden` | 会话的两步（MCP 没有） | 六个写的参数照写者的；读者的 `list`；成员的 `move`、`delete` 的答复；访客的 `notebooks` 为空；两种入口的 404 与 `notebook.not_found`（含 `batch`） | 0.5–1 秒 |
| PG14 (MCP) | （没有）大小与最后的活动，SQL 里比时刻 | — | 最后的写是 `edit` | 0.3–0.5 秒 |
| C7 (MCP) | （没有）帧的 `toEqual`、C 收不到 | 锁的帧（MCP 没有会话） | 一个 `batch` 一个帧；`edit` 的帧；最后流是安静的（Marker 的帧是下一帧） | 0.3–0.6 秒 |
| L1 (MCP) | `expectIndexedLinks` ×3；`readContent` | 阅读视图（渲染） | `expectChangesetOf`（锚点与 Markdown 链接的改写） | 0.3–0.5 秒 |
| L3 (MCP) | `expectIndexedLinks`、`content_updated_by` 的读、`listNodes` 与 `readContent` | — | `linking.pages_locked` 的形状、本人的 `you: true`、`expectAliveSessions` ×2、`expectChangesetOf` ×2、`update_links: false` 穿过被编辑的页 | 0.4–0.8 秒 |
| L4 (MCP) | `expectTagged`（`expectIndexedTags` ×3）；折叠等于 API 版本的字面的 `listTags` | 公式、图（渲染） | `index {tag}` 先比字面的路径（按 id）、再比 REST `getTag` ×3；`#proj` 对 `getTag("proj")` | 0.3–0.5 秒 |
| L6 (MCP) | `expectLinked`（`expectIndexedLinks` ×2）、`links` 的帧 | 属性的读（没有工具）、大纲与右栏（浏览器） | 开流之前 settle；`backlinks` 先比字面的、再比 REST ×3（含游标与 `next_cursor`） | 0.3–0.6 秒 |
| AS2 (MCP) | `expectEmbedded`（`expectContentWritten`、`expectUploaded` ×2、`expectIndexedLinks`）；`getView` 的 `<img>` | 粘贴与拖放（编辑器） | 别处的同名附件让 `link` 带路径；`read` 的 `link`、`url` 与图片；`expectChangesetOf` | 0.4–0.6 秒 |
| AS3 (MCP) | `expectIndexedLinks` ×2；`getView`、`getPageProperties`、改名之后的 `readContent` 与 `getView` | 媒体、PDF、下载（浏览器）；上传的答复的类型（同一个 `upload`，AS3 (API) 查过；S1 的审查 A-N1） | `expectChangesetOf` | 0.4–0.7 秒 |
| AS4 (MCP) | `expectAssetsDeletedWithNodes` ×2；`getAsset` 与签名的地址 | `expectAssetsDeletedWithNotebook`（PG13 的豁免）、`expectAssetsPurged`（平台的定时任务）；第二个 API 测试（大小读没删的附件） | 没有 `recursive` 的数目；单独删一个附件；删掉的附件的文件仍在存储里 | 0.4–0.6 秒 |
| HI1 (MCP) | `expectVersions`（再加 agent 的一行）；`readContent` | 会话本身（只有 REST）；`getVersion` 的 `id` 与 `base_revision`（由整行的比较看到） | `versions` 对 REST 整行与字面的投影；`editing` | 0.3–0.5 秒 |
| SE1 (MCP) | `expectFound` 对同一张 `finds`（重载收 `Omit<Found, "marked">` 与 `{marked: false}`） | `marked`（MCP 不给范围） | 对 REST 的路径、种类与数，摘要照 `cut_start`、`cut_end`；删除的答复；MCP 的删除立即离开搜索 | 约 0.5 秒 |
| SE2 (MCP) | （没有）缩小的查找与三个拒绝 | 工作区范围的三个（MCP 只搜一本） | 三个账户的 `notebooks` 与 `truncated: false`；在列出的每一本里各搜一次 | 约 0.5 秒 |

两个已有的测试各补一两行（与它们的 MCP 版本在同一个提交里）：C7 (API) 把 Marker 提成变量，删除之后读出它的版本号、写它一次、`expectNext` 下一个版本号的帧（原写"`base_revision` 2、版本号 3"；settle 会再写 Marker，S1 的审查 A-M1 = C-M1，3.3）；L6 (API) 先建 Marker，开流之前 `settleEvents`。

**不空过**：每个"恰是这些"的比较都 `toEqual` 整个列表，不 `toContain`；拒绝的列表都是字面写下的几项，映射之后与字面的码比（空的列表比不出东西）；"什么都没写"的检查都在全部的拒绝之后（PG1 的节点数、PG2 的 `expectRenamed` 再一次、PG8 的 `expectVersions`、PG12 的 `nodes`、L3 的 `listNodes` 与版本号）；与 REST 比的都先断言字面的值（L4 的路径与折叠、L6 的 `[path, links]`、HI1 的三行、SE1 的表、SE2 的每一本一页）：两边读同一条语句时（L4 的 `index {tag}` 与 `getTag`）只比两边证明不了选法；次序都写成字面的（PG4 的锁按子树、L4 与 L6 按 `write` 的 id），不靠 UUIDv7 的 id 随请求递增；`sessions: 0` 之前读过两页各有一个会话（PG4）；`expectAssetsDeletedWithNodes` 的 id 的列表都不空（AS4）；事件流的检查开流之前都 settle（C7、L6），C7 最后看流是安静的（L6 在 `links` 的帧之后不再读流，S1 的审查 C-N4）。

**期限**：e2e 的服务端的请求期限是默认的 15 秒（第 1 节）；每次调用是一页、一次小的写或一个查找。最慢的准备是 PG3 的 `chain(8)`（8 次顺序的 REST 的建页）与 AS3 的五个上传（`Promise.all`），都是 API 版本已有的、通过的。4 秒的请求期限是 Go 的整个程序测试的（`docs/v0.1/M9-mcp/01-P1-platform-reads.md:861`），P5 不加 Go 的测试。等待只有 `expectNext` 的 10 秒（`e2e/fixtures/events.ts:29`，C7、L6）与 Playwright 的 `expect` 的 10 秒，都是失败的界线，不是等候。

### 9.2 变异与反向对照

实施者逐个做下面的改动（每个之后还原），确认"应失败"的一栏失败，结果写进第 12 节与审查记录（照 P4 的 6.3；做法见第 8 节第 8 步：服务端的先 `make build`，语句的改在生成的常量里）。编号接着 [P4](04-P4-grep-search.md) 6.3 的 222。位置是 `7dc4a686` 的（S1 施加时的位置与每条失败的行在 [S1 的审查记录](reviews/P5-S1-stories-docs-review.md)的"反向对照"里）。服务端与 fixture 的变异只在本机的工作树里做，不提交。最后一栏是 API 版本是否也抓到它：抓到的是共用的覆盖，抓不到的是只有 MCP 版本守着的。

| # | 变异（位置） | 应失败（故事与步） | API 版本也抓到？ |
|---|---|---|---|
| 223 | `writtenOf` 给出 `revision + 1`（fixture，`e2e/fixtures/mcp.ts` 新加的函数） | PG7 (MCP) 第 4 步、PG8 第 5 步、PG9 第 2 步、AS2 第 3 步（`expectContentWritten` 读的版本号来自答复） | 不适用（MCP 版本的 fixture） |
| 224 | `edit` 的答复的 `version` 以别的节点的 id 起头（`page/adapter/mcp/edit.go:41`） | PG7 (MCP) 第 3 步（与字面的 `"<id>:3"` 比）；MC3 同样（`mc3:119`）。它不是 `writtenOf` 的检查的反向对照：那些由第 8 节第 8 步的伪造的答复证明 | 不（REST 没有 `edit`） |
| 225 | MCP 的路径段在检查之前去掉两端的空白（`page/domain/path.go:70-77` 之前 `TrimSpace`） | PG1 (MCP) 第 5 步 | 不（REST 的标题本来就去掉，`shared/title.go:29`） |
| 226 | 同一父页下的改名也排到兄弟的最后（`page/app/unit_relocate.go:48-49` 照换父页的一支） | PG2 (MCP) 第 2 步（`expectRenamed` 的次序不变） | 不（REST 的改名经 `unit_rename.go`，不经 `Relocate`） |
| 227 | 去掉 `RelocationOf` 的成环的检查（`page/domain/steps.go:143-146`） | PG3 (MCP) 第 4 步 | 不（REST 的移动自己查环，`page/app/unit_move.go:49-51`；只有 MCP 的 `move` 与 `batch` 经 `RelocationOf`） |
| 228 | agent 的删除只看别人的会话（`page/app/edit_lock.go:55-56` 的 `agent` 一支改成 `a.UserID != s.By`） | PG4 (MCP) 第 3 步（锁只剩 Child 一项）、C4 (MCP) | 不（REST 的删除走 `OpDelete` 的一支） |
| 229 | 活着的会话不看 `expires_at`（`page/adapter/postgres/gen/sessions.sql.go:19` 的 `AliveSessionsOf` 的 `AND expires_at > $2` 写成 `AND $2::timestamptz IS NOT NULL`，同去掉它：直接去掉时语句只剩一个参数、生成的调用照旧传两个，pgx 每次都报"expected 1 arguments, got 2"（S1 的审查 C-N2）；来源 `queries/sessions.sql:29`） | PG4 (MCP) 第 4 步（过期的会话仍挡） | PG4 (API) 也失败（它的过期的会话同样挡住删除）；C6 (API) 不（开会话先删掉过期的行，`page/app/unit_session.go:32`） |
| 230 | `page.revision_mismatch` 的当前的版本给成调用的 `base_version`（分两半做，各自让 PG8 (MCP) 第 3 步失败：230w 在 `page/app/write_path.go:224`，`Changed` 的调用处（`:250` 是 `mismatch` 帮助函数，那里没有 `base`），230e 在 `edit_page.go:146`；适配器的 `text.go:32-34` 做不了：`VersionMismatch` 只有 `Path`、`Current`、`Why`，`page/domain/errors.go:222-226`） | PG8 (MCP) 第 3 步（两个 `current_version` 都成了 `"<id>:1"`）；MC3 的过时的 `write` 同样（`mc3:112`） | 不（REST 的 409 不带当前的版本） |
| 231 | MCP 的 `write` 把正文的 CRLF 规整成 LF（`page/adapter/mcp/write.go:44` 交给用例之前） | PG9 (MCP) 第 2、3 步（CRLF、混合的两种的字节与哈希） | 不（REST 的写不经它） |
| 232 | 文件夹的子节点按名称排而不是树的次序（`page/app/list_by_pattern.go:296-298` 的 `SortOrder`） | PG11 (MCP) 第 3 步（First、Between 的位置）；MC2 同样 | 不（REST 的 `listNodes` 不经它） |
| 233 | 笔记本入口不判定可见性（`platform/httpserver/mcpserver/edge.go:232-250` 放行看不到的） | PG12 (MCP) 第 4 步、SE2 (MCP) 第 4 步；MC1 与矩阵同样 | 不 |
| 234 | 笔记本的活动只算 `web`、`api` 的变更集（`page/adapter/postgres/gen/activity.sql.go:15-21` 的 `NotebookActivities` 的 `WHERE` 加 `AND cs.client NOT LIKE 'mcp:%'`；来源 `queries/activity.sql:1-10`。不是 `notebook/app/list_ownerless.go:76-80`：那里只合计各来源，`notebook/app/extension.go:128-154`） | PG14 (MCP)（这本笔记本只有 agent 的变更集：大小与活动都不在答复里） | 不（REST 的写的客户端是 `api`） |
| 235a | 事件流对 MCP 的单元不发帧（`bootstrap/events_registrants.go:23-32` 的 `PagesChanged` 遇到 MCP 的客户端就返回 nil）。不放在 `page/app/unit.go:635-647` 的 `tell`：那会连链接的索引（`bootstrap/linking.go:25`）与附件（`bootstrap/assets.go:78`）一起关掉，MC4 的 `expectIndexedLinks` 今天就会失败 | C7 (MCP) 第 2 步；Go 的 `TestThePagesEventListsNoPageInTheTrash`（`bootstrap/events_pages_test.go:141-161`，一个 MCP 的 `batch`）同样 | 不 |
| 235b | 链接的索引对 MCP 的单元不发 `links` 的帧，行照写。两处的改动：`linking.PagesChanged`（`linking/app/index.go:13-17`）加一个字段，`bootstrap/linking.go:37` 把 `e.Write.Client.IsMCP()` 填进去（页面的观察者转换事件时丢掉了客户端，`bootstrap/linking.go:24-37`），`linking/app/index.go:85` 在它为真时跳过 `x.publish`。只在 `index.go:85` 跳过、不分客户端，L6 (API) 也会失败，就不再是只有 MCP 版本守着的了。这是临时的、两个文件的改动，只为这一条反向对照，不提交 | L6 (MCP) 第 3 步（开流之前 settle 之后，Doc 自己的晚到的帧不再冒充 Other 的） | 不 |
| 236 | `PagesWritten` 每个改动发一个帧（`events/app/publisher.go:52-63`） | C7 (MCP) 第 3 步（batch 的两页成两帧）；删除的一半（Parent、Minutes、Late 的删除成三帧）在流上看不出来：三帧彼此相同，PostgreSQL 在一个事务里只送一次（3.3 第 5 步），抓到它的是 Go 的 `TestAPagesEventSaysWhatTheUnitWrote`（`events/app/publisher_test.go:57`）。对照 236q（`bootstrap/events_registrants.go:31`：删除的单元另发一个不同的 `{tree: false, pages: []}` 帧）让 C7 (MCP) 第 5 步失败 | 不（C7 (API) 通过；它抓到 236q，`171b739d` 的 C7 (API) 抓不到）。原写"与第 5 步"与"C7 (API) 加了同样的安静的检查之后也失败"（S1 的实施者的发现，审查 C-I1） |
| 237 | `move` 的 `update_links` 默认为假（`page/adapter/mcp/move.go:33-34`） | L1 (MCP) 第 3 步（Source 不改写）、L3 (MCP) 第 2 步（管理员的移动跳过改写，不被拒）；MC4 同样 | 不（REST 的移动没有这个参数） |
| 238 | 锁的拒绝的项都不带路径（`page/app/path_writes.go:36-40` 总把 `read` 设为 nil，如同读路径失败） | L3 (MCP) 第 2 步；PG4 (MCP) 第 3 步（`page.locked` 也经它，`delete_path.go:89`） | 不 |
| 239 | 锁的项的 `you` 总为假（`page/adapter/mcp/locks.go:60`、`:67`） | L3 (MCP) 第 2 步（A 自己的 agent）、PG4 (MCP) 第 3 步（Grandchild）；C4 (MCP) 同样 | 不 |
| 240 | 不改写链接的移动也改写、也查锁（`linking/app/rewrite.go:62-64` 去掉 `if !m.UpdateLinks { return nil }`） | L3 (MCP) 第 6 步（被 `linking.pages_locked` 拒绝）；Go 的 `mcp_writes_test.go:415-423` 同样 | 不（REST 的移动没有 `update_links`） |
| 241 | `TagPages` 只取相等的标签、不含其下的（`linking/adapter/postgres/gen/reads.sql.go:205-213`，去掉 `:210` 的一支） | L4 (MCP) 第 2 步：与字面的列表比时失败；与 `getTag` 比照样相等（两边同一条语句），所以先比字面的 | L4 (API) 也失败（`l4-tags-math.spec.ts:65-69` 的 `getTag("proj/alpha")` 要 Source 与 Deeper）；MC7 不（它的两页都直接带 `plan`） |
| 242 | 附件的 `read` 不给 `link`（`page/adapter/mcp/read.go:133-134`） | AS2 (MCP) 第 2 步 | 不 |
| 243 | 附件的行不随 MCP 的删除而删（`bootstrap/assets.go:78-91` 的 `assetObserver` 对 MCP 的单元跳过 `NodesDeleted`：附件的行经页面的观察者跟着它们的节点，asset 不是页面的写的参与者） | AS4 (MCP) 第 3、4 步 | 不（只跳过 MCP 的；不分客户端地跳过时 AS4 (API) 同样失败） |
| 244 | `versions` 的列表的 `editing` 总为假（`page/adapter/mcp/history.go:70`） | HI1 (MCP) 第 2 步；Go 的 `page/adapter/mcp/history_test.go:110-150` 同样（MC8 的数据里没有活着的会话，抓不到） | 不 |
| 245 | 搜索不排除回收站里的节点（`page/adapter/postgres/gen/search.sql.go` 的 `SearchBodies` `:255`、`SearchBodyCount` `:374`、`SearchTitleCount` `:562`、`SearchTitles` `:610` 去掉 `deleted_at` 的条件） | SE1 (MCP) 第 4 步，删除之后立刻搜：5 秒一次的搜索任务删掉回收站里的节点的行之后（`page/app/index_search.go:159-168`）它就抓不到，所以那一次的任务先跑了时重跑，至多三次，写进记录 | SE1 (API) 也失败（同样的语句） |
| 246 | SE1 (MCP) 的 `mcpFoundOf` 丢掉最后一个名称（fixture，反向对照） | SE1 (MCP) 第 2 步（`{marked: false}` 之后名称与数照样比） | 不适用 |
| 247 | `edit` 不看 `base_version`（`page/app/edit_page.go:141-146` 去掉两个 `VersionMismatch` 的分支） | PG8 (MCP) 第 3 步（过时的 `edit` 写下了 "Mine"）：e2e 里只有它发过时的 `edit` | 不（REST 没有 `edit`） |
| 248 | 换了父页的节点排在最前（`page/app/unit_relocate.go:70` 的 `placeAmong(siblings, len(siblings)-1)` 改成 `-1`） | PG3 (MCP) 第 2 步（A 的子页成了 `["B", "A1"]`）与第 3 步 | 不（REST 的移动经 `unit_move.go`，PG3 (API) 照样通过） |
| 249 | 附件改名时属性里的链接不改写（`linking/domain/rewrite.go:76-78` 的 `Rewrite` 遇到 `r.Link.InFrontmatter()` 就 `continue`） | AS3 (MCP) 第 2 步（`readPage` 的文字里属性的 `[[cover.png` 没换；`expectIndexedLinks` 的那一个不再指向 cover） | AS3 (API) 也失败（同一个改写）：共用的覆盖 |
| 250 | 按路径读一个版本时丢掉 BOM 与 `\r`（`page/app/get_version.go:86` 的 `GetVersion.AtPath` 给的正文，S1 改在 `:115`、`findVersion` 之后；它只有 MCP 的 `versions {path, revision}` 调，`page/adapter/mcp/versions.go:101`） | HI1 (MCP) 第 4 步（`revision: 1` 的 `created` 逐字节） | 不（REST 的 `getVersion` 按 id，不经 `AtPath`） |
| 251 | SE1 (API) 的 `foundOf` 不再给 `marked`（fixture，`se1-search.spec.ts:90-104`；`marked` 照旧必须时它编不过，所以改成给 `[]`） | SE1 (API) 第 2 步（命中的范围不对）：`marked` 留作必须、MCP 在调用处不比，API 与页面版本照旧比它 | 本身就是 API 版本 |

### 9.3 门禁

合并之前，本机：`make lint`（含 README 的格式与 e2e 的 tsc）、`make knip`、`make gen-check`、`make test-web`、`make build`、`make e2e`（本 M 与之前所有 M 的故事），都通过；`make test-go` 与 `make test-go-short` 照常跑（P5 不改 Go 的代码，它们照样要绿）；CI 的各份都是绿的。之后照 M9 的做法：审查（Opus）、修复、核对、合并、推送 `main`。

**Caddy 的重跑**（合并之前，S1 的实施者）：README 的 Caddy 样例由 P5 在 Caddy 2.10.2 上核对过（5.2）；合并之前在本机已有的镜像里照同一个脚本再跑一次，把五个路由的上游的 `duration` 写进审查记录：缓冲的三个在毫秒级、上传与导入约 4 秒；再跑一次下面的早答的探测，记下经 Caddy 的状态码。不下载（`--pull=never`），不联网（`--network none`），不映射端口；`Caddyfile` 的第二个站点是同一个容器里的上游，读请求体、记 `duration`：

```caddyfile
{
    admin off
    auto_https off
    persist_config off
}

:8091 {
    @buffered path_regexp ^/(mcp|api/v0/pages/[^/]+/content|api/v0/notebooks/[^/]+/pages)$
    handle @buffered {
        reverse_proxy 127.0.0.1:8080 {
            request_buffers 37MiB
        }
    }
    handle {
        reverse_proxy 127.0.0.1:8080
    }
}

:8080 {
    log {
        output stderr
        format json
    }
    respond "uri={http.request.uri} cl={http.request.header.Content-Length} bodylen={http.request.body}" 200
}
```

`run.sh`（审查 B 的原样；与 `Caddyfile` 放在同一个目录，经主持者的共用锁 `heavy.sh` 执行 `docker run --rm --pull=never --network none -v "$PWD:/etc/caddy:ro" caddy:2.10-alpine sh /etc/caddy/run.sh`）：

```sh
#!/bin/sh
caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile 2>&1 | tail -2
caddy run --config /etc/caddy/Caddyfile --adapter caddyfile 2>/tmp/caddy.log &
sleep 2
head -c 200000 /dev/zero | tr '\0' 'a' > /tmp/b200k
for spec in "POST /mcp?notebook=x" "POST /api/v0/notebooks/x/assets" "PUT /api/v0/pages/x/content" "POST /api/v0/notebooks/x/pages" "POST /api/v0/notebooks/x/imports"; do
  m=${spec%% *}; u=${spec#* }
  t0=$(date +%s)
  curl -s -o /dev/null -X $m --limit-rate 50k --data-binary @/tmp/b200k "http://127.0.0.1:8091$u"
  t1=$(date +%s)
  echo "$m $u client_secs=$((t1-t0))"
done
# chunked to /mcp and to assets
curl -s -o /tmp/o1 -X POST -H 'Transfer-Encoding: chunked' --data-binary @/tmp/b200k "http://127.0.0.1:8091/mcp" ; echo
curl -s -o /tmp/o2 -X POST -H 'Transfer-Encoding: chunked' --data-binary @/tmp/b200k "http://127.0.0.1:8091/api/v0/notebooks/x/assets"; echo
sleep 1
grep '"logger":"http.log.access' /tmp/caddy.log | sed -E 's/.*"method":"([A-Z]+)".*"uri":"([^"]*)".*"Content-Length":\[?"?([0-9]*)"?\]?.*"bytes_read":([0-9]+).*"duration":([0-9.]+).*/\1 \2 cl=\3 bytes_read=\4 duration=\5/' 
grep -c access /tmp/caddy.log
```

最后几行是上游的访问日志里每个请求的 `Content-Length`、读到的字节与 `duration`；末尾两个分块的请求到上游时仍是分块的（5.2）。P5 的修订跑的就是这一份（`scratchpad/m9/p5/exp/caddy/run.sh`、`run-out.txt`）。

**早答的探测**（同一次重跑；审查 B 的第二轮的 `review-B/early/`，复制在 `scratchpad/m9/p5/exp/caddy/early/`）：上游是另一个 Caddy 进程，Go 的 `net/http`，同服务端，不读请求体就答 503 `server_busy` 与 `Retry-After: 1`，同 `largeBody` 等不到名额时（`platform/httpserver/largebody.go:146`、`:166-170`）；代理是上面那份的 `:8091`。照 5.2 的表，几 MiB 以上的请求体经缓冲的路由多半答 502、没有 `Retry-After`；把每个大小、每个路由三次的状态码写进审查记录。三个文件放在同一个目录，经 `heavy.sh` 执行 `docker run --rm --pull=never --network none -v "$PWD:/cfg:ro" caddy:2.10-alpine sh /cfg/run-caddy2.sh`：

`backend.Caddyfile`：

```caddyfile
{
    admin off
    auto_https off
    persist_config off
}
:8080 {
    # like largeBody's 503 server_busy: answered before the body is read
    header Retry-After 1
    respond "{\"status\":503,\"code\":\"server_busy\"}" 503
}
```

`proxy.Caddyfile`：

```caddyfile
{
    admin off
    auto_https off
    persist_config off
}
:8091 {
    @buffered path_regexp ^/(mcp|api/v0/pages/[^/]+/content|api/v0/notebooks/[^/]+/pages)$
    handle @buffered {
        reverse_proxy 127.0.0.1:8080 {
            request_buffers 37MiB
        }
    }
    handle {
        reverse_proxy 127.0.0.1:8080
    }
}
```

`run-caddy2.sh`：

```sh
#!/bin/sh
caddy run --config /cfg/backend.Caddyfile --adapter caddyfile 2>/tmp/backend.log &
sleep 1
caddy run --config /cfg/proxy.Caddyfile --adapter caddyfile 2>/tmp/proxy.log &
sleep 2
for size in 2097152 4194304 8388608 31457280; do
  head -c $size /dev/zero | tr '\0' 'a' > /tmp/body
  for u in "/mcp?notebook=x" "/api/v0/pages/x/content" "/api/v0/notebooks/x/assets"; do
    res=""
    for i in 1 2 3; do
      res="$res $(curl -s -o /dev/null -w '%{http_code}' -X POST --data-binary @/tmp/body "http://127.0.0.1:8091$u")"
    done
    echo "size=$size via=caddy $u ->$res"
  done
done
```

审查 B 量 nginx 的那两次（`exp/caddy/early/run-nginx.sh`、`site.conf`、`out-nginx.txt`；第三轮每个大小三次的 `exp/caddy/early2/run-nginx.sh`、`out-nginx-3x.txt`）要把 `caddy` 的二进制从镜像里拷进挂载的目录当上游，不在合并之前的重跑里；替身答 401、429 的那一次（`exp/caddy/early2/run-caddy.sh`）也不在。它们的结果在 5.2。

### 9.4 CI 的余量

**现在**（第 1 节的主持者的数）：组合根三份 7:15、5:59、3:58（`ed9e149f` 时第一份 4:45、第三份 4:07），第一份的单次运行在 7:30 的线附近、最近两次的平均约 6:00；e2e 的 E2E 一步 3:41（P4 之前 3:28–3:37，P4 的两个 MCP 故事之后 3:31–3:41）；web 2:52（`ed9e149f` 的一次卡住在 P5 之外，没有再出现）、守卫 10:03、成本 9:10、竞态 10:07。

**P5 加的**：

| 份 | P5 加的 | P5 之后的估计 |
|---|---|---|
| `bootstrap-1`、`bootstrap-2`、`bootstrap-3` | 无（没有 Go 的测试） | 不变 |
| `guards`、`costs`、`race`、`short` | 无 | 不变 |
| `web` | 无（`make test-web` 不含 e2e） | 不变 |
| `e2e` | 21 个测试、净加 20 个（PG14 (MCP) 取代 MC3 (activity)），本机各约 0.3–1 秒、合计约 9–14 秒（单个 worker 算），都不开浏览器；C7 (API)、L6 (API) 各多一次写或 settle；fixture 的改动只是类型 | E2E 一步约 3:45–4:15；作业的时限 20 分钟 |

估计的依据：P4 的 MC6、MC7（本机估计 11–15 秒）让 E2E 一步多了约 4–10 秒；P5 的 21 个各自更轻（一两次握手、几次调用，REST 的准备与 API 版本相同），各 worker 有自己的服务（`e2e/fixtures/test.ts:111-134`），在 CI 的默认的 worker 数下约 +5–30 秒（审查 A 估 +5–20 秒）。合并之前以分支的一次运行核对，写进审查记录。

**组合根的规则照旧**（00 号文档第 11 节第 28 条；[P4](04-P4-grep-search.md) 6.5）：P5 不加组合根的测试。实施中若要加一个，它的名字取 `TestThe` 加 F–Z 起头，落在第三份（`BOOTSTRAP_THIRD = ^Test(The[F-Z]|T[i-z]|[U-Z])`，3:58–4:07，余地最多），同 P4；以 `cd server && go test -race -list '<表达式>' ./internal/bootstrap` 对两个表达式各列一次核对，写进审查记录。第一份在线附近，P5 不往它加任何东西。从不提高 `-timeout`。

## 10. 完成标准

**S1**：

- 21 个 MCP 版本都在各自的故事的文件里、名为"`<故事> (MCP)`"，都通过；每个调它的 API 版本调的每一个断言函数（9.1 的"共用的断言"一栏），值来自 MCP 的答复；不共用的只有 9.1 写明的那几个，各有理由（会话、渲染、MCP 没有的动作）；都不开浏览器。
- MC3 (activity) 不在 MC3 里了，PG14 (MCP) 在 `pg14-notebook-activity.spec.ts`，它的最后一次写是 `edit`；MC3 没有多余的导入，开头的注释不再说活动。
- C7 (API) 在删除之后查流是安静的，L6 (API) 在开流之前 settle，都通过。
- fixture：`expectContentWritten`、`Embedded.written`、`expectMoved` 收 `Pick`，今天的调用者没改；`writtenOf`、`readPage` 与 `PageRead` 在 `fixtures/mcp.ts`、都导出、各有导入者；`make lint`（含 tsc）与 `make knip` 通过。`expectToggleRevision` 没改；SE1 的 `Found.marked` 仍是必须的。
- 9.2 的第 223–251 条（含 235a、235b）都被抓到，各自写明哪个测试、哪一步失败；`writtenOf` 的伪造的答复的检查与 SE1 (API) 的反向对照写进审查记录。
- README：4.1 的"接入 agent（MCP）"一节在"运行后端"之后、"接口与代码生成"之前；`:219` 改为指向它与"部署"；部署一节的反向代理是 5.1 的四条（含 nginx 与 Caddy 的样例），升级的说明多了 M9 的一条；`check:format` 通过；4.2 的表之外没有客户端的行为（4.2 的表逐行对过）。
- 人工清单在 `docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md`，是 6.1 的全文：A–I 九部分，每步有预期与"记下的"一栏，"结果"一行写明怎样算通过；一个客户端一本笔记本；样例照抄 README 的（只换地址、`<笔记本 id>` 与变量）；它的链接都解析得到。
- 服务端没有改动；内置的说明仍是 4,027 与 3,957 字节。
- 9.3 的门禁，含合并之前的 Caddy 的重跑（五个路由的 `duration` 与早答的探测的状态码写进审查记录）；`make e2e` 通过（本 M 与之前所有 M）；CI 的各份都是绿的，e2e 的 E2E 一步的用时写进审查记录。

**文档**：设计的提交以 `apply-00m9.py` 只落 00 号文档的 2.1 各条与第 12 节（"进行中（设计已审查）"）、第 13 节的两行。其余的都在 S1 合并之后的文档提交里（照第 7 节）：00 号文档第 12 节的 P5 一行"完成"、第 13 节合并的一行、"移交的落实"的 M0/P1 一行（加清单的步，仍是 open）、"写给后面的 M"的 M12；v0.1 总体设计的 6.3、10.1、11、13.4 第 3 条与第 15 节；M12 的性能移交第 18 项的三处与两条新的、M12 的打磨移交第 31 项与它开头的一句、M12 的转移移交的部署一条、M10 的 MCP 移交第 6 项（第 11 节）；这份设计的状态、3.9 与第 12 节；审查记录。

**M9 不因 P5 合并而完成**：P5 之后是 M9 的收尾（第 11 节）；人工清单由负责人在 v0.1 的最后跑，通过之后 M9 才改为已完成（00 号文档第 11 节的取舍第 12 条）。

## 11. 交给后面的

- **M9 的收尾**（00 号文档第 7 节：三位 Opus 审查者并行，服务端、e2e 与文档、完成标准，然后核对修复）：
  - "移交的落实"的九行对照代码逐项核对；M0/P1 一行**不改为 `done`**，[M0/P1 的移交](handoffs/M0-P1-mcp-sessions.md)也照旧 `open`：它的第 1、3 项要的是在 Claude Code 与 Codex 上量过，落实写"清单的 G、D 部分；待 v0.1 最后的人工验收"，清单跑过之后照记下的关（同 M9 自己的状态）。
  - 第 9 节的表：22 个有 MCP 版本的故事各指到它的测试（C4 (MCP) 与 P5 的 21 个），27 个豁免各有 3.8 的核对；9.2 的变异与 P1–P4 的一起列进收尾的反向对照。
  - M9 的状态写"实现完成，待人工验收"：清单与输入法清单在 v0.1 的最后一起跑，通过之后才改为已完成；收尾审查记录留一节"人工验收"，清单的结果整份写进去。
- **v0.1 的最后，两份人工清单**（负责人；M4 的[输入法清单](../M4-pages/manual/P6-ime-checklist.md)与 P5 的 MCP 客户端的清单，同一次验收）：
  - MCP 的那份约两到三小时：九部分、两个客户端、各自的一本笔记本；第 I 部分要 Docker（容器的命令在清单里），可以与别的部分分开做；I3 在负责人用 Caddy 的部署上确认。开始之前记下两个客户端的版本与日期。
  - 隔离照清单的"不碰你自己的客户端配置"：Claude Code 只以 `--mcp-config … --strict-mcp-config` 在临时目录里启动、不执行 `claude mcp add`（除了临时目录里的项目范围）与 `claude mcp list`；Codex 只在临时的 `CODEX_HOME` 与空的临时目录里，登录先用 API 密钥，其次在临时的 `CODEX_HOME` 里另登录一次，最后才复制 `auth.json`（记下指纹，指纹允许时才复制回去，复制回去在删临时目录之前，其间不在别处用 Codex）。
  - 跑过之后：README 照 4.2 的表改（每一句的"核对之后"一栏），改的提交写明照清单的哪一步；记下的与预期不同之处（例如某个客户端在服务重启之后要重新连接、Codex 的某个审批取值对读的工具也问）由负责人定是改 README、交 M12，还是要改服务端（清单的"结果"一行的通过的规则）。

**移交的文件与文字**（00 号文档：每一条"由发现它的 Phase 写进目标 M 的 `handoffs/`"，`00-M9-design.md:457`）。P5 的文档提交（S1 合并之后）写进下面四份：

1. **M12 的打磨移交**（`docs/v0.1/M12-release/handoffs/M5-polish.md`，新的第 31 项，加在第 30 项之后、末行"之后的 M 推迟的体验事项照样追加到这份移交。"之前；文件开头的"第 30 条 M9"（`:10`，恰好一次）同时改为"第 30、31 条 M9"）：

   ```markdown
   31. **MCP 的接入在网页上**（[M9 总设计](../../M9-mcp/00-M9-design.md)"写给后面的 M"的 M12 打磨一条；[M9/P5](../../M9-mcp/05-P5-stories-docs.md) 第 4 节）：
       - 网页上显示 MCP 的两个地址（笔记本入口与账户入口）与接入的帮助，以 README 的"接入 agent（MCP）"为底稿：一个 agent 一个 PAT、令牌放在环境变量里（每个客户端一个变量）、审批不一律放行。那一节里标着"照文档""未核对"的几句，等人工清单在 v0.1 的最后跑过、README 照它改过之后再照抄。
       - agent 的变更集在历史里只显示 `clientInfo.name`（"MCP（名称）"，M9 总设计 4.5）；客户端报了 `clientInfo.title` 时显示它。
   ```

2. **M12 的转移移交**（`docs/v0.1/M12-release/handoffs/M7-transfer.md:29`，"部署文档"一条的末尾加一句）：

   ```markdown
   M9/P5 起反向代理按路由处理请求体：`/mcp` 与页面正文的两个写缓冲，上传与导入不缓冲、上限是导入包的上限加 64 KiB（它不小于附件的上限加 64 KiB；nginx 写 `513m`），README 的部署一节有 nginx 与 Caddy 的样例与升级的一条（[M9/P5](../../M9-mcp/05-P5-stories-docs.md) 第 5 节）；部署文档照它整理，缓冲的请求体在认证之前收在代理那里（nginx 的磁盘、Caddy 的内存），一并写明；经 Caddy 时服务端早答的拒绝成 502 的事见性能移交第 18 项。
   ```

3. **M12 的性能移交**（`docs/v0.1/M12-release/handoffs/M4-performance.md` 第 18 项）："慢的发送者"一条（`:95`）的三处照 7.3 改；第 18 项的最后加两条；这一项说 10 万个候选的守卫的时限的两处照 00 号文档第 11 节第 102 条改为 1 s（S1 的文档的提交）。经 Caddy 的 502 放在这里而不放在转移移交的部署一条：它是名额与认证的早答在代理上的样子，改法（若改）在服务端，与这一项的名额、慢的发送者是同一件事：

   ```markdown
      - **真实客户端与 e2e**（[M9/P5](../../M9-mcp/05-P5-stories-docs.md) 第 11 节）：48 KiB 的答复与 3 MiB 的内联图片两个上限，照人工清单的 C7、C9、C10 记下的再看（Claude Code 的约 25,000 个 token 的结果上限；一个请求里 20 张以上的图片时每张至多 2,000 像素）。压测时也量慢的发送者经 Caddy（缓冲在内存里）与直接暴露时占满名额的样子。e2e 的 E2E 一步在 M9/P5 之后量得 3:37、3:40、3:35（分支的三次运行；P5 之前 3:41，P5 的设计估 3:45–4:15），合并之后 `main` 的一次 2:56，21 个新的测试本机合计约 3.8 秒；作业的时限 20 分钟，之后的 M 继续往里加故事时看要不要把 e2e 分份。
      - **经 Caddy 时早答的拒绝成了 502**（[M9/P5](../../M9-mcp/05-P5-stories-docs.md) 5.2，Caddy 2.10.2 上量过，上游是一个同样不读请求体就答的 Go `net/http` 的替身）：服务端在读请求体之前的拒绝，取不到名额的 503 `server_busy`、认证失败的 401、限流的 429，对几 MiB 以上的请求体经 Caddy 常常到客户端成了 502、没有 `Retry-After`（M9/P5 的 S1 合并之前的重跑：缓冲的两个路由 2–30 MiB 的 24 次都是 502）；nginx 照原样转发（量的是 503）。网页的保存只在 503 带着 `Retry-After` 时等一等再发（`web/apps/web/src/stores/page-editing.ts:336`），在 401 时续期再发（`web/apps/web/src/session/auth-middleware.ts:45`），拿到 502 两样都不做：两个大的调用占着名额时的一次大的保存、访问令牌过期时的一次大的保存，都立刻失败；agent 也看不到"稍后重发"。服务端可以怎样改由 M12 定，例如早答之前把一个很快地到来的请求体短短地读掉，让代理写完再拿到答复；这只是一种可能，不是决定。部署文档至少写明用 Caddy 时的这个代价，或推荐 nginx。
   ```

4. **M10 的 MCP 移交**（`docs/v0.1/M10-llm-wiki/handoffs/M9-P1-mcp-registry.md`，新的第 6 项，加在第 5 项之后）。选这一份而不另开 `M9-P5` 的一份：它就是 M9 给 M10 的 MCP 的那一份（状态 `open`），开头说 M9 总设计给 M10 的别的"由之后的 Phase 写进 M10 的移交，M9 收尾时核对"；下面三条都关于 M10 在 MCP 上加东西，说明的余量挨着它的第 4 条（说明的找法与缓存），另开一份会把 M9 给 M10 的 MCP 的事分在两处，M9 收尾时要核对两份。

   ```markdown
   6. **之后的 Phase 留下的三条**（[M9/P3](../../M9-mcp/03-P3-batch-revert.md)、[M9/P4](../../M9-mcp/04-P4-grep-search.md) 第 8 节，[M9/P5](../../M9-mcp/05-P5-stories-docs.md) 第 11 节）：
      - **内置的说明的余量**：账户入口 4,027 字节、笔记本入口 3,957 字节，离 4 KiB 只剩 69 字节。M10 往说明里加字（prompt、虚拟的两页、`llm_wiki` 的规则）的先缩短已有的行，`TestTheBuiltInInstructionsAreShort`（`page/adapter/mcp/instructions_test.go:15-27`）照旧守 4 KiB。
      - **故事的 MCP 版本的约定**（[M9 总设计](../../M9-mcp/00-M9-design.md)第 9 节，M9/P5 3.1）：在故事自己的文件里、名为"`<故事> (MCP)`"，种下的数据照 API 版本经 REST，每个 agent 一个 PAT，值来自答复（`e2e/fixtures/mcp.ts` 的 `writtenOf`、`readPage`），收编辑会话的断言以 `expectVersions` 代之，不开浏览器。
      - **prompt 的说法**：人工清单（`M9-mcp/manual/P5-mcp-clients-checklist.md`）记下 agent 不照说明做的地方（C3–C6 的 `edit` 与 `move`，C12、C15 的 `partial_lines`、`cut` 与 `read(offset, column)`）；M10 写 ingest、query、lint 的 prompt 时照它们改说法。清单在 v0.1 的最后才跑，M10 在那之前写的照工具的说明。
   ```

## 12. 结果

**S1：其余的故事的 MCP 版本、接入文档与人工清单**（2026-10-23，分支 `m9-p5-s1`，合并 `053d3ba1`；审查记录 [P5-S1-stories-docs-review.md](reviews/P5-S1-stories-docs-review.md)）

- 提交（从 `main` 的 `171b739d` 开始，照第 8 节的次序，第 1–7、9–11 步各一个，共 10 个；第 8 步的变异不提交）：
  - fixture 的放宽（`expectContentWritten`、`Embedded.written` 收 `Pick<Page, …>`，`expectMoved` 收 `Pick<TreeNode, …>`）与 MC2 的 `readPage`、`PageRead` 挪进 `fixtures/mcp.ts`（`14287a43`）。
  - `writtenOf` 与 PG1、PG2、PG3、PG4、PG7、PG8、PG9 (MCP)；PG2 的 NFD 的标题、PG9 的六种正文提到顶层（`a3bc759c`）。
  - PG11、PG12 (MCP)（`615e6434`）。
  - PG14 (MCP)：MC3 (activity) 挪来、改名，第二次写改为 `edit`；MC3 去掉它、只有它用的六个导入与文件开头注释的末句（`4bce4e8f`）。
  - C7、L1、L3、L4、L6 (MCP)；C7 (API) 删除之后的安静的检查、L6 (API) 开流之前 settle（`f2575d0b`）。
  - AS2、AS3、AS4 (MCP)（`3ca6ee57`）。
  - HI1、SE1、SE2 (MCP)；HI1 的四个正文常量提到顶层；`expectFound` 的重载（`d4a8adf8`）。
  - README 的"接入 agent（MCP）"与 `:219` 的一行（`00c19120`）；部署一节的反向代理与升级的一条（`c63b4bb2`）；人工清单 `manual/P5-mcp-clients-checklist.md`（`f29b9853`）。
  - 合并 `main` 的 `eb371c17`，带进 `9d991781`（10 万个候选的 grep 守卫的调用的时限 1 s，只改 `grep_cost_test.go`，00 号文档第 11 节第 102 条），使分支的 CI 带着它。
  - 修复一轮十个（`5cb7e824`–`e519297c`：C7 (API) 一个，README 两个，清单七个），修复二轮三个（`8067afd9`、`f60c4c80`、`380b253e`，只改清单），各项见审查记录。
  - 每个提交单独在一个工作树里 `make lint`、`make knip` 与它碰到的故事的文件（经 `heavy.sh`；第 1–7 个各 1、27、6、4、22、12、11 个通过）都通过；文档的提交只 lint 与 knip；修复两轮的每个提交 `make lint`、`make knip` 通过（修复一轮的十个由修复核对 C 各自重跑过）。
- 与设计不同之处：见 3.9。
- **e2e**：21 个新的测试，MC3 (activity) 成为 PG14 (MCP)，262 → 282。本机 `make e2e`（含 `make build`）282 个 37.4 秒（之前 262 个 36.5 秒；负载 5.6–6.3），修复一轮之后 37.2 秒（负载 10.3）。新的 21 个在整个套件里的用时（ms，本机）：PG1 107、PG2 111、PG3 266、PG4 174、PG7 116、PG8 94、PG9 134、PG11 105、PG12 191、PG14 87、C7 245、L1 82、L3 174、L4 90、L6 170、AS2 412、AS3 385、AS4 193、HI1 147、SE1 255、SE2 227，合计约 3.8 秒（单个 worker 算；9.4 估 9–14 秒）；修复一轮之后 C7 (API) 273 ms、C7 (MCP) 303 ms。每个测试在 CI 上的用时读不到（作业的日志要认证），只有 E2E 一步的（下面）。审查 A 把这 21 个与 C7 (API)、L6 (API) 跑了 276 次（三种 worker 数），没有失败。
- **变异**（9.2 的第 223–251 条，各单独施加、之后还原；服务端的先 `make build`，语句的改在 `gen/*.sql.go`；在最终的测试上整个重跑一遍，两遍结果相同）：每一条都让点名的测试在点名的步失败：223 PG7 第 4 步、PG8 第 5 步、PG9 第 2 步、AS2 第 3 步（`expectContentWritten`）；224 PG7 第 3 步；225 PG1 第 5 步；226 PG2 第 2 步（`expectRenamed`）；227 PG3 第 4 步；228 PG4 第 3 步；229 PG4 第 4 步；230 的两半各在 PG8 第 3 步；231 PG9 第 2 步；232 PG11 第 3 步；233 PG12 第 4 步与 SE2 第 4 步；234 PG14；235a C7 第 2 步；235b L6 第 3 步；236 C7 第 3 步；237 L1 第 3 步与 L3 第 2 步；238、239 L3 第 2 步与 PG4 第 3 步；240 L3 第 6 步；241 L4 第 2 步；242 AS2 第 2 步；243 AS4 第 3 步；244 HI1 第 2 步；245 SE1 第 4 步（第一次就抓到：搜索任务还没删掉回收站里的行）；247 PG8 第 3 步；248 PG3 第 2 步；249 AS3 第 2 步；250 HI1 第 4 步。API 版本也抓到的是 229（PG4 (API)）、241（L4 (API)）、245（SE1 (API)）、249（AS3 (API)），同 9.2；只有 236 不同：C7 (API) 加了安静的检查之后照样通过，删除的一半由 Go 的发布者的测试抓到（3.9 第 1 条）。每条施加时的位置与失败的行在审查记录的"反向对照"。
- **反向对照**：`writtenOf` 的两个伪造的答复（别的节点的 id 起头的 `version`、`at: null`）各在 `fixtures/mcp.ts:193` 抛出一次；SE1 (API) 在 251（`foundOf` 的 `marked` 给 `[]`）之下在第 2 步失败；246（`mcpFoundOf` 丢掉最后一个名称）让 SE1 (MCP) 第 2 步失败；236q 让 C7 (MCP) 第 5 步与 C7 (API) 新加的检查失败、`171b739d` 的 C7 (API) 通过；236-del（236 之下绕过 C7 (MCP) 第 3 步）通过。审查 A 以自己的改法重跑 12 个变异与两个伪造的答复，C 重跑 236、236q 与 Go 的测试，修复核对一轮 A、C 在 `e519297c` 上重跑 236 与 236q，结果都同。
- **门禁**（本机，头 `eb371c17`，都经 `heavy.sh`）：`make lint`、`knip`、`gen-check`、`test-web`（166 个文件、3,342 个测试）、`build` 都通过；`make e2e` 282 个 37.4 秒；`make test-go` 810 秒（组合根三份 183.5、214.0、295.0 秒；10 万个候选的 grep 守卫的调用 134.6–149.2 ms，对 1 s）；`make test-go-short` 53 秒。服务端的代码没有改，内置的说明照旧 4,027 与 3,957 字节（审查 C 以副本重算）。清单的链接与 README 到它的链接逐个打开核对；清单的 `sh` 块与行内命令过 `sh -n`、`bash -n`、`zsh -n`（审查 B 与修复的两轮）。
- **Caddy 的重跑**（合并之前，`caddy:2.10-alpine`，`--rm --pull=never --network none`，没有映射端口）：`Caddyfile` 是 `Valid configuration`（`run.sh` 里唯一的一次 `caddy validate`；早答的探测的脚本不校验，审查 C-N1）；200,000 字节以 `--limit-rate 50k` 发到五个路由，上游的 `duration`：`/mcp` 0.0010 秒、正文的 PUT 0.00069 秒、新建页的 POST 0.0022 秒（缓冲，毫秒级），上传 3.84 秒、导入 3.85 秒（不缓冲，约 4 秒），同 5.2；分块的请求到上游时仍是分块的。早答的探测（替身不读请求体就答 503 与 `Retry-After: 1`）经 Caddy、每格三次：缓冲的两个路由在 2、4、8、30 MiB 都是 502 502 502；附件 2 MiB 503 503 503、4 MiB 503 503 503、8 MiB 503 502 502、30 MiB 503 502 503，同 5.2 的表（几 MiB 以上经缓冲的路由多半 502、没有 `Retry-After`；这一次 24 次都是 502）。审查 B 以 README 自己的 Caddy 样例重跑，结果相同。之后 `heavy.sh docker ps -a` 里没有留下 Caddy 的容器。
- **CI**：建成 run 39908901709（`eb371c17`）全绿，整个 17:21，E2E 一步 3:37（作业 4:26；9.4 估 3:45–4:15、P5 之前 3:41）；修复一轮 run 39916610094（`e519297c`）全绿，E2E 一步 3:40；修复二轮 run 39920143015（`380b253e`）全绿，E2E 一步 3:35；合并之后 `main` 在 `053d3ba1` 上的 run 39922678794 全绿，E2E 一步 2:56（作业 3:43）。组合根一的测试一步 7:13、5:53、4:37，合并之后 7:17，都在 7:30 之下；P5 没有加组合根的测试，各份的分界不动。各份的用时见审查记录的"门禁与 CI"。`main` 在 `171b739d`（这份设计的提交）上的运行红在守卫那一份（P4 的 S1 的 10 万个候选的那次调用，00 号文档第 11 节第 102 条），修复合并之后 `main` 的 run 39905348090 绿。
- **审查与修复**：三位审查者（Opus：e2e；README 与清单；完成标准、2.3、偏离与提交），Important 1（文档，第 236 条的说法）、Minor 8、细节 20、存疑 5；修复两轮（一轮十个提交、二轮三个），修复核对一轮由 A、B、C，二轮由 B 与主持者；主持者的决定 9 条（00 号文档第 11 节第 96–104 条）。M9 不因 P5 合并而完成：之后是 M9 的收尾，人工清单在 v0.1 的最后由负责人跑（第 10、11 节）。

## 附录：在代码里核对过的（`7dc4a686`）

| 决定 | 核对的代码 |
|---|---|
| 没有服务端的改动（2、2.3） | P1–P4 第 8 节的 P5 各项（`01-P1-platform-reads.md:2552-2556`、`02-P2-writes-history.md:2676-2680`、`03-P3-batch-revert.md:2565`、`04-P4-grep-search.md` 第 8 节）；说明的 503 一行已在（`page/adapter/mcp/instructions.go:73-74`）；说明的 4 KiB（`instructions_test.go:15-27`） |
| 期望的值来自答复（3.1） | `write` 的答复 `page/adapter/mcp/write.go:23-32`、`:54-57`；`edit` 的 `edit.go:23-31`、`:41`；`move` 的 `move.go:23-29`；`delete` 的 `delete.go:20-27`；`read` 的两块 `read.go:27-38`、`:102-122`；版本令牌 `page/domain/version_token.go:21`；`at` 是单元的时刻 `page/adapter/mcp/tools.go:148-153`、`page/app/unit.go:250`、`:556-559`；MC3 以它作 `created_at`（`e2e/stories/mcp/mc3-write-edit.spec.ts:70-86`），`created` 是从根起的路径、名称取最后一段（`mc3:62-66`、`:78`） |
| `expectContentWritten`、`expectMoved` 的放宽（3.1、2.1 第 1 条） | `e2e/fixtures/assert/page.ts:429-445`（只读 `id`、`revision`、`content_updated_at`，`:440`、`:443`）、`:135-149`（只读 `id`、`parent_id`，`:144`、`:147`）；`expectNewPage` 已收 `Pick`（`:16-22`）；`Embedded.written`（`assert/asset.ts:162-166`）；调用者 PG5、PG7、PG8、PG9、PG10、C10、AS2 都传 `Page` |
| `writtenOf`、`readPage` 与 `PageRead` 进 `fixtures/mcp.ts`，都导出（3.1、第 8 节第 1 步） | knip 的类型规则 `e2e/fixtures/mcp.ts:12-14`；MC2 的 `PageRead` `mc2-read-list.spec.ts:64-73`、`readPage` `:76-83`、`readAll` 的签名 `:86`、循环的 oxlint 注释 `:92`；C4 (MCP) 手写的解析 `c4-guard.spec.ts:95-96`；`make lint` 的 tsc `Makefile:163`、`e2e/package.json:8`；伪造的答复的做法 `assert/page.ts:261-270` |
| `collab.ts:50` 不放宽（2.1 第 4 条） | `e2e/fixtures/assert/collab.ts:44-61`（`expectToggleRevision`，只有 C10 调） |
| 只写正文的写没有条目（AS2、L1、AS3 的 `expectChangesetOf`） | `page/app/unit.go:419-431`（条目只记 `c.Moves()` 的改动）；MC4 的 `items: [setup.id]`、`versions: [index]`（`mc4-move-delete.spec.ts:72-77`） |
| PG1：MCP 不去掉空白、`/` 是分隔 | `page/domain/path.go:70-77`；REST 去掉 `shared/title.go:29`；读者的 `forbidden` `page/app/authorize.go:41` |
| PG2：按键找页；改名留在原处；同名的写是 API 版本的 `twin` | `page/adapter/mcp/write.go:73-74`；`page/app/write_path.go:176-190`（内容相同时不写）；`page/app/unit_relocate.go:25-29`、`:35-36`、`:48-49`；`pg2-rename-page.spec.ts:49`（NFD 的字面）、`:54-55` |
| PG3：`move` 没有位置，换父页时在最后；成环与太深；REST 走别的路 | `api/mcp/tools.json` 的 `move`（`from`、`to`、`update_links`）；`unit_relocate.go:70-74`；`page/domain/steps.go:141-154`；`MaxPathSegments` `page/domain/path.go:14-17`；REST 的移动自己查环 `page/app/unit_move.go:49-51`，改名经 `unit_rename.go:17-21`；移动的 `at` 由 MC4 查（`mc4-move-delete.spec.ts:63`） |
| PG4：agent 的删除连本人的也拒；锁按子树的次序；数目在锁之前；进回收站 | `page/app/edit_lock.go:42-66`（`agent` 一支 `holds` 恒真，`:55-56`）、`:74-113`（`lockedAll`：一层一层）；按 id 排的是 linking 的 `pages_locked`（`page/app/lock_holders.go:49`）；`page/adapter/mcp/locks.go:14-32`；`page/adapter/mcp/text.go:35-38`；`page/app/delete_path.go:42-50`、`:86`；过期的会话 `page/adapter/postgres/gen/sessions.sql.go:15-21`（来源 `queries/sessions.sql:29`）；开会话先删过期的行 `page/app/unit_session.go:32`；随子树删掉 `page/app/unit_delete.go:56-60`；本地的 `expire` `pg4-delete-subtree.spec.ts:18-23`；`expectDeletionRevisions` `assert/page.ts:567-575`；`expectTrashRoot` `:327-363`；TB 的 `alone` `tb1-trash.spec.ts:58`、`:146-147`、`:252`，`tb2-background-restore.spec.ts:131`、`:196`；MC4 不带 `mc4:129`；Go 的过期的规则 `page/app/edit_lock_test.go:105`；整个文字带着码：`page/adapter/mcp/text.go:35-38` 造，mcpserver 原样作 `message`（`platform/httpserver/mcpserver/errors.go:155-156`、`:188-190`），`errorOf` 比它与文字块（`e2e/fixtures/mcp.ts:171-176`） |
| PG7、PG8：没有会话；当前的版本；过时的 `edit` | 工具的参数 `write.go:14-18`、`edit.go:13-19`；`app.PathWrite`、`app.PageEdit` `page/app/write_path.go:17-21`、`edit_page.go:14-19`；`edit` 先查令牌 `edit_page.go:140-148`（两个 `VersionMismatch` `:141-146`）；`write` 的 `write_path.go:250`；`VersionMismatch` 的字段 `page/domain/errors.go:222-226`；`current_version` `page/adapter/mcp/text.go:32-34`；REST 的 409 `platform/httpserver/problem.go:33-52`；`expectVersions` `assert/page.ts:509-522`；`sessionsOf` `:474-480`；MC3 的过时的 `write` `mc3:108-113` |
| PG9：只按 `\n` 分行；正文照存 | `page/domain/read.go:61-69`；`page/domain/content.go:19-29`；常量 `pg9-byte-fidelity.spec.ts:16-17` |
| PG11：按键读、答存下的名称；文件夹是树的次序，glob 是路径的次序 | `page/app/read_path.go:80`；`page/app/list_by_pattern.go:296-298`；MC2 `mc2-read-list.spec.ts:90-91`、`:131-171` |
| PG12：两种入口的拒绝；先判定笔记本；矩阵已有三列 | `platform/httpserver/mcpserver/edge.go:232-250`；`page/app/authorize.go:28`（`decideNotebook`）；`mcpClient` 抛出 `e2e/fixtures/mcp.ts:99-101`；`server/internal/bootstrap/permission_matrix_mcp_test.go:150-155`、`:582-585`；`permission_matrix_mcp_writes_test.go:14-24`；`permission_matrix_notebook_test.go:154-176`；API 版本的六个 `forbidden` `pg12-page-permissions.spec.ts:59-69`、内联的语句 `:109-116` |
| PG14：MC3 (activity) 就是它；活动的来源 | `mc3-write-edit.spec.ts:170-201` 对 `pg14-notebook-activity.spec.ts:16-44`；MC3 的主测试不用那几个导入（`mc3:1-11`、`:50-168`），开头的注释 `:18-19`，本地的 `Edited` `:34-39`；`page/adapter/postgres/queries/activity.sql:1-10` 与 `gen/activity.sql.go:15-21`；`notebook/app/list_ownerless.go:76-80` 只合计 `notebook/app/extension.go:128-154`；`changesets.client` 的约束（`client ~ '^mcp:…'`） |
| C7：MCP 的写经同一个单元告诉事件；一个单元一个帧；安静的检查 | `page/app/unit.go:635-647`；`bootstrap/events_registrants.go:23-32`（删除不列页）；`events/app/publisher.go:43-63`；`write_path.go:128`、`:214`，`relocate_path.go:87`，`delete_path.go:72`，`edit_page.go:169`，`apply_batch.go:235`；Go 已有的 MCP 的 `batch` 到流 `bootstrap/events_pages_test.go:141-161`（`tm.agent(...).batchDone`，`mcp_writes_test.go:21-24`）、`tm.quiet` `events_pages_test.go:67-77`；C7 (API) 的 settle `c7-push.spec.ts:41`、删除之后只读一帧 `:62-63`；`settleEvents` `e2e/fixtures/events.ts:95-113`；`openEvents` `e2e/fixtures/test.ts:52`、`:197-207`；`events.ts:29`；`edit` 的 `old_string` 至少 1 字节 `page/adapter/mcp/edit.go:15` |
| L1：改写在同一个单元；`update_links` 默认真 | `page/app/relocate_path.go:28-31`、`:86`；`page/adapter/mcp/move.go:33-34`；`expectIndexedLinks` `e2e/fixtures/assert/links.ts:21-48` |
| L3：`linking.pages_locked` 的形状、`you`、不改写不查锁 | `page/adapter/mcp/locks.go:38-143`（文字 `:86-88`，`you` `:60`、`:67`）；`page/app/path_writes.go:16-40`（`read = nil` `:36-40`）；`linking/app/rewrite.go:62-64`；Go 的 `server/internal/bootstrap/mcp_writes_test.go:368-425`；`expectAliveSessions` `assert/collab.ts:18` |
| L4：标签由 `index` 读，先比字面的，次序按 id | `linking/adapter/mcp/index.go:25-89`；同一条语句 `linking/app/list_index.go:102`、`linking/app/tags.go:55`、`linking/adapter/postgres/queries/reads.sql:96-105`、生成的 `gen/reads.sql.go:205-213`；`index` 去掉 `#`、REST 不去 `linking/adapter/markdown/tag.go:19-31`；`listTags` 的数 `gen/reads.sql.go:242-248`；API 版本的 `getTag` 与折叠 `l4-tags-math.spec.ts:65-69`、`:70-74`；MC7 的 `byId` `mc7-index-backlinks.spec.ts:58-61`、`:149-198`，它的标签 `:90-92` |
| L6：反链带路径；`links` 的帧；开流之前 settle | `linking/adapter/mcp/backlinks.go:20-62`；`linking/app/index.go:36-92`、`:85`、`:113-123`；L6 (API) 开流之前不 settle `l6-panel.spec.ts:58`；235b 要两处：`linking.PagesChanged` 没有客户端 `linking/app/index.go:13-17`，页面的观察者转换时丢掉它 `bootstrap/linking.go:24-37` |
| AS2：附件的 `link`、`url` 与图片；同名的附件 | `page/adapter/mcp/read.go:40-56`、`:133-134`、`:136-152`、`:162-165`；名称唯一时 `link` 就是名称 `linking/app/asset_links.go:36-40`；`page/app/ports.go:562-575`；`expectEmbedded` `assert/asset.ts:162-192`（比上传的答复的 `link`，`:190`）；上传 `e2e/fixtures/assets.ts:33-46`；API 版本的 `getView` `as2-paste.spec.ts:64-68` |
| AS3：属性里的链接随附件改名 | `linking/domain/rewrite.go:74-97`（`InFrontmatter` 的编辑）；API 版本的 REST 的读 `as3-render.spec.ts:79-140` |
| AS4：删除笔记本、清理没有 MCP 的形式；404 的理由；附件的行跟着节点 | `notebook/module.go:140`、`:144-145`；`e2e/fixtures/purge.ts:14-34`；内容的路由先看节点 `asset/app/content.go:59-64`；`bootstrap/assets.go:78-91`（`assetObserver`）；`expectAssetsDeletedWithNodes` `assert/asset.ts:108-116`；`blobPath` `:21` |
| HI1：`versions` 的行是 REST 的；按路径读一个版本 | `page/adapter/mcp/versions.go:28-34`、`:100-118`（`AtPath` `:101`）；`page/adapter/mcp/history.go:70`（`Editing`）；`page/app/get_version.go:86`；Go 的映射测试 `page/adapter/mcp/history_test.go:110-150`；REST 的行 `page/adapter/http/history.go:46-65`；API 版本的两项 `hi1-versions.spec.ts:97-103`；MC8 `mc8-history.spec.ts:153-170` |
| SE1、SE2：MCP 的搜索只在一本里、没有范围；搜索的可见性 | `page/adapter/mcp/search.go:32-37`、`:55`；`Visible.VisibleIn` `page/app/search.go:168-178`；`expectFound` 与 `finds` `se1-search.spec.ts:56-82`，`written()` `:37-53`，`foundOf` `:90-104`；回收站的行由任务删 `page/app/index_search.go:159-168`；回收站的条件 `page/adapter/postgres/gen/search.sql.go:255`、`:374`、`:562`、`:610`；`namesFound` `se2-search-visibility.spec.ts:52-56`；MC6 的比较 `mc6-grep-search.spec.ts:125-143`；`Found` 与 `expectFound` `se1-search.spec.ts:55-82`（`marked` 必须） |
| 豁免（3.8） | `server/internal/bootstrap/mcp.go:45-47`；`page/tools.go:31-37`；`page/module.go:233-246`、`:299-325`；`linking/module.go:89-94`；`notebook/module.go:140-145`；`platform/httpserver/mcpserver/server.go:127-128`；`page/app/toggle_task.go:11-47`；`asset/attachment_reads.go:58-69`；`server/migrations/sql/00029_transfer_transfer_jobs.sql:23`、`00044_page_client_checks.sql:10-15`；MC1 `mc1-connect.spec.ts:124-145`；MC4 `mc4-move-delete.spec.ts:120-134`；MC5 `mc5-batch-revert.spec.ts:160-193`；C9 的循环 `c9-one-stream.spec.ts:28-95`；HI4 的三处 `hi4-revert-delete.spec.ts:96`、`:111`、`:158` |
| 请求期限（9.1、9.4） | `e2e/fixtures/server.ts:139-152`（不设 `request_timeout`）、`:150`（`public_url`）；`server/configs/config.yaml:17`；`e2e/playwright.config.ts:10`；Go 的 4 秒 `docs/v0.1/M9-mcp/01-P1-platform-reads.md:861`；各 worker 的服务 `e2e/fixtures/test.ts:111-134` |
| 接入一节的数与文字（4.1、4.3） | `server/configs/config.yaml:11`、`:17`、`:23`、`:74`、`:76`；`bootstrap/mcp.go:29`；`page/adapter/http/limits.go:8`；`platform/httpserver/mcpserver/edge.go:132-140`、`:147`、`:175-181`、`:193-195`；`platform/httpserver/api.go:297-310`（`BodyLimits` 的路由的期限另加 `server.read_timeout`）、`:346-350`（失败闸门按 IP，在认证之前）、`:378-383`；调用的工作从读完请求体起 15 秒 `platform/httpserver/mcpserver/call.go:40-56`；`read` 的地方 `page/adapter/mcp/read.go:60-92`、说明的"约 45 KiB" `instructions.go:45`、答复的量法 `mcpserver/answer.go:25-60`；没有 `server.public_url` 时没有地址 `read.go:40-60`；SDK 的纯文字的 400、415（`modelcontextprotocol/go-sdk` v1.8.0 `mcp/streamable.go:347-398`）；`api/mcp/tools.json` 的注解；PAT 的接口 `api/modules/identity.yaml`（`ApiTokenCreate`）；P1 的试用 `01-P1-platform-reads.md:2578-2592`、没有令牌的那一句 `:2554-2556`；M0/P1 的实验 `01-P1-spikes.md:318-319` |
| 名额占到答完；谁持着；谁能撤销（2.1 第 9 条、4.1、5.1、清单的 F1） | `platform/httpserver/largebody.go:123-171`（`defer release()` 在 `next.ServeHTTP` 之后）、`:150-160`（分块的先读 64 KiB）、`:188-226`（`takeLarge`：自己的凭证的名额、`own`）、`:244-250`（`large request bodies are saturated`，带 `holders`）；`platform/httpserver/server.go:44`；凭证的形状 `identity/adapter/authn/authenticator.go:48-50`；`user_id` `server/migrations/sql/00003_identity_auth_sessions.sql:7`、`00004_identity_api_tokens.sql:7`；撤销只撤自己的 `identity/app/revoke_api_token.go:12-15`；管理员命令 `README.md:82-92` |
| 按路由的反向代理（第 5 节） | `README.md:324-336`、`:342-368`；上传 `api/modules/asset.yaml:44-66`、导入 `api/modules/transfer.yaml:104`；`Envelope` `transfer/adapter/http/start_import.go:24`、`:45`，`handler.go:80`，`asset/adapter/http/upload.go:30`、`handler.go:76`；`import_max_bytes` 不小于 `asset.max_bytes` `platform/config/validate.go:291-292`；`server/configs/config.yaml:23`、`:133`、`:149`、`:151`；`server.max_body_bytes` 也是正文路由的门槛 `platform/httpserver/api.go:315-320`；比它大的请求体只在 `bootstrap/wire.go:183-184` 与两个流式的路由 `asset/adapter/http/handler.go:67`、`transfer/adapter/http/handler.go:71`；回环保护 `mcpserver/server.go:132`；M5/P2 的容器 `docs/v0.1/M5-collab-editing/02-P2-event-stream.md:261`；审查 B 的 `nginx -t` 与路由的一次运行 `scratchpad/m9/p5/review-B/nginx/`、`nginx-route/`；Caddy 的重跑、内存与样例 `scratchpad/m9/p5/exp/caddy/`（`run-out.txt`、`mem-out.txt`、`readme/validate-out.txt`）；早答的拒绝：`platform/httpserver/largebody.go:140-172`（`:146`、`:166-170`）、`api.go:335-350`、`limit.go:24-36`；网页的保存的重试 `web/apps/web/src/stores/page-editing.ts:336`、401 的续期 `web/apps/web/src/session/auth-middleware.ts:45`；审查 B 第二轮的探测 `scratchpad/m9/p5/exp/caddy/early/`（`out-caddy.txt`、`out-caddy2.txt`、`out-nginx.txt`）、第三轮的 `exp/caddy/early2/`（nginx 每个大小三次的 `out-nginx-3x.txt`，替身答 401、429 的 `out-caddy-401-429.txt`）与分开进程的内存 `exp/caddy/mem-separate/out.txt` |
| 清单（第 6 节） | 格式 `docs/v0.1/M4-pages/manual/P6-ime-checklist.md`；M0/P1 的实验 `docs/v0.1/M0-foundation/01-P1-spikes.md:306-331`；`make run` `Makefile:54-55`、`config.dev.yaml:4`；签名私钥 `README.md:103`；PAT 只存哈希 `identity/domain/api_token.go:16-31`；建页 `api/modules/page.yaml:1998-2017`；上传的 `parent_id` 可省 `asset/adapter/http/upload.go:143-149`；请求的日志 `platform/httpserver/middleware.go:249-255`；说明：账户入口的 `instructions.go:19-27`、"一个接一个" `:73-74`、`AGENTS` 取代内置的 `:9-17`；工具的说明里的引号、唯一、完整路径与 `partial_lines`（`api/mcp/tools.json`）；`MaxGrepLineText` `page/domain/grep.go:31`、命中的 `cut` 与 `column` `page/adapter/mcp/grep.go:31-37`；网页对 POST 的 405 `platform/webui/handler.go:82-83`；图片内联的上限 `page/adapter/mcp/read.go:183-184`；`exp/insts/` 的指令数；同样的正文的 `write` 答没有改动 `page/app/write_path.go:176-190`；认证失败的闸门 `platform/httpserver/api.go:346-350`、`server/configs/config.yaml:74`；M12 的打磨移交的开头 `docs/v0.1/M12-release/handoffs/M5-polish.md:10` |
| 变异的做法（第 8 节第 8 步、9.2） | `make e2e` 先构建 `Makefile:80-81`；`make gen-check`；各条的位置在 9.2 的表里 |
| CI（9.4） | `Makefile:217-237`；`.github/workflows/ci.yml:164-205`；主持者给的 run 39884100596 与 39878983820 的用时 |

