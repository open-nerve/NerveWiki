# M9/P2 写的工具与历史的读：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M9/P2 写的工具与历史的读（两个 Step：S1 写的工具；S2 历史的读） |
| 日期 | 2026-10-18 |
| 状态 | 设计稿，按设计审查与它的两次复核修订（[P2 设计审查](reviews/P2-design-review.md)，含"第二轮复核"与"第三轮复核"）；没有遗留的 Important 与 Minor |
| 上级文档 | [M9 总设计](00-M9-design.md) 4.1、4.2、4.5、4.7、4.8、4.10、4.12、4.14、4.15、第 5、7、9、10、11 节与"移交的落实"；[P1](01-P1-platform-reads.md) 3.3、3.4、3.9、3.13、3.16、3.19 与第 8、9 节；[v0.1 总体设计](../v0.1-design.md) 6.1–6.4、7、8.4、10.1、12、13 |
| 代码位置的写法 | 同 00 号文档：模块写作 `page/app/unit.go:168`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…`、`archtest/…` 在 `server/internal/` 之下；迁移写作 `00014_page_changesets.sql:13`（`server/migrations/sql/`）；SDK 写作 `sdk/mcp/streamable.go:440`（`~/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.8.0/`）；Go 的标准库写作 `$GOROOT/src/net/http/request.go:1234`（go1.27.1）；前端写作 `web/…/history-filters.tsx:17`；e2e 写作 `e2e/fixtures/mcp.ts:108` |

---

## 0. 分两个 Step

照 00 号文档第 7 节，P2 中等大小。分两个 Step，各自一个分支、一次审查与修复核对、一次合并；后一个从前一个合并之后的 `main` 开始。两个都在服务端与 e2e，没有网页的改动：网页的历史已能按 MCP 筛选、显示"MCP（名）"（`web/…/history-filters.tsx:17-18`、`:114-115`，`web/…/changeset-text.ts:34-46`）。

- **S1 写的工具**（分支 `m9-p2-s1`）：
  - 大的请求体的界线：整个服务两个名额、每个凭证至多一个，httpserver 的逐路由中间件，REST 的两个正文路由共用（3.2）；
  - 归属：`domain.MCPClient`、`Client.IsMCP`、`Client.Valid` 改用 `shared.Unshowable`、迁移 00044 的显式范围（3.3）；
  - 版本令牌的解析 `ParseVersionToken`（3.4）；
  - 单元的操作：`Unit.Relocate` 与一条语句的 `RelocateNode`、`Unit.Subtree` 与 `Unit.DeleteSubtree`；linking 的 `Relocation` 的大小写改名不看父节点；撤销的一步的放回（移动加改名是一步，3.5）；
  - 编辑锁对 MCP 删除的规则（3.6）；`NodePaths.Walk` 与 `NodePaths.Paths`（3.7）；
  - 用例 `WritePath`、`EditPage`、`RelocatePath`、`DeletePath`，每一步的规则在 page/domain（P3 的 `PlanBatch` 照用），`edit` 的线性的匹配（3.8、3.9）；
  - mcpserver 的错误：`Extra` 只补问题没有的成员、`locks` 的上限、`ToolError.Fit`；P1 的 `page.not_found` 过界的修正（3.10）；
  - 工具 `write`、`edit`、`move`、`delete`，`api/mcp/tools.json`、内置的说明、码的表与矩阵的四行（3.11–3.13）；
  - 整个程序的测试：最后一跳、锁的每一格、交错、`update_links=false`、撤销 MCP 的移动加改名、大的请求体、NUL 的检查到达用例（3.13）；
  - e2e：MC3、MC4、C4（MCP），MC1 第 5 步改为照 `tools.json`（第 4 节）；
  - CI：组合根的每一份照 6.5 的规则：过了线（CI 的测试一步约 7.5 分钟）的一份，只在一个邻居挪过之后仍在线之下时把测试给它，否则按测试名再分一份（第二份预计要分出第三份），从不提高 `-timeout`；本机的 `make test-go` 照同样的份跑组合根，竞态检测的几个二进制同时跑；`edit` 的时间的守卫并进成本那一份的 page/domain 一行；成本与守卫的行在 CI 上的估计过了 Go 默认的 10 分钟的一半就一个测试一行（守卫的 markdown 一行多半要拆）。
- **S2 历史的读**（分支 `m9-p2-s2`）：
  - 用例的按路径与按笔记本的入口：`ListVersions.AtPath`、`GetVersion.AtPath`、`ListChangesets.AtPath`、`GetChangeset.InNotebook`（3.16）；
  - 工具 `versions`、`log`、`changeset`，`tools.json`、内置的说明、码的表与矩阵的三行（3.16、3.17）；
  - e2e：MC8（4.6）。

**界线为什么在 S1**：S1 带来第一批真的大的写（`write`、`edit` 的 5 MiB 正文），界线与它们一起进 `main`，`main` 在每次合并时都有它。大的请求体今天就进得来：P1 的 `list`、`read` 的调用收至多 37,814,272 字节的请求体（`bootstrap/mcp.go:24-28`、`mcpserver/edge.go:100-104`），一个 36 MB 的 `path` 在 `ParsePath` 拒绝之前被检查、解码（`mcpserver/tool.go:178`、`:194-197`，schema 没有 `maxLength`），P1 的 S1 审查 A 量的 1,381–1,471 MB 正是这一种（[P1](01-P1-platform-reads.md) 第 9 节）：S1 把它们一起管住。P1 留下的接缝工具只在测试里登记（`bootstrap/wire.go:58-67`）。

**历史的读为什么另成一步**：它们只读、不碰写的单元，用例是 M8 的加上按路径与按笔记本的入口；MC8 要 MCP 写出的变更集（按客户端 `mcp` 筛选），所以排在 S1 之后。S2 单独合并是绿的：它只加工具与它们的行，S1 的检查（清单、矩阵、码的表、说明、工具的字符串的合法参数的表）照样覆盖它们，表里缺了它们的项就失败（5.2）。

## 1. 基线

- 从 P1 完成之后的 `main` 开始：`2a888a6f`（P1 的 S3 在 `c9468e5b` 合并；`0c934bb9` 合并了 River 的任务的等待与竞态那一份的 20 分钟，只改测试与 CI；`2a888a6f` 是 S3 的文档，P1 完成，代码与 `0c934bb9` 相同）。本文的行号是 `2a888a6f` 的。
- **写入单元**（M4–M8）：
  - `Writer.Run(ctx, spec UnitSpec, do)`（`page/app/unit.go:168-230`）：事务之前核对客户端（`!spec.Client.Valid()` 是不带码的错误，`:172-174`）、执行者与工作区；事务里锁工作区行 `FOR SHARE`、笔记本行 `FOR SHARE` 或（`Tree`）`FOR NO KEY UPDATE`（`:197-206`），锁下判定（`:207-210`），读一次时钟（`:211-214`），然后 `do`、搜索的入队、观察者、提交。
  - 每个操作经 `apply`：守卫 → 第一次写入时建变更集 → 写 → 条目 → 参与者（`unit.go:261-291`）；条目只记"前""后"不同的改动（`recordItems`，`:370-392`），一个节点的几次改动以 `Change.Then` 合并（`page/domain/change.go:63-73`）。
  - 树的操作：`CreatePage`（`unit_create.go:28-53`，检查依次是标题 422、父页 422、`after_id` 422、名称 409、层级 409）、`Rename`（`unit_rename.go:17-50`，只改大小写的改名不查兄弟、照写）、`Move`（`unit_move.go:21-65`，环 409、名称 409（新兄弟里看现在的名称）、层级 409，子孙随它、只有它记条目；写经 `moveSubtree`，`:71-84`）、`Delete`（`unit_delete.go:20-45`，读子树、守卫、`DeleteNodes`、`DeleteNodeSessions` 删这些页的全部会话行、告诉订阅者那时还活着的）。三个调用 `Delete` 的：`delete_node.go:38`、`unit_revert.go:95`、`:454`。
  - 正文的写：`WriteContent`（`unit_content.go:42-96`）锁正文行 `FOR NO KEY UPDATE`、找节点、内容与当前完全相同时什么也不写（不经守卫，`:62-64`）、`Base` 不等于当前版本号答 `page.revision_mismatch`（`:65-66`），然后经守卫写。
  - `Options{UpdateLinks}` 只交给参与者（`page/app/extension.go:17-21`）；REST 的十个用例写死为真（`create_page.go:37` 等）。
  - 存储：`MoveNode` 只改父节点与次序值、`RenameNode` 只改名称与键（`page/adapter/postgres/queries/nodes.sql:143-157`）；兄弟的名称由 00042 的两个部分唯一索引兜底，`nameTaken` 把它们的违反译成 `domain.ErrTitleTaken`（`page/adapter/postgres/store.go:24-64`、`:330-357`）。
  - 没有一个用例在同一单元里先建一页、再写它的正文；建页时正文直接进 `PageDraft.Content`（`unit_create.go:67`，版本行 `base_revision` 为空、版本号 1，`:79`）。
- **撤销**（M8/P3）：计划的 `planner.place` 把一个放回先模拟成"带着现在的名称移到 S\* 的父节点下"、再改名（`page/domain/revert.go:865-891`，`:874` 查现在的键），单元的 `place` 同样写两步（`page/app/unit_revert.go:124-150`）；中间态撞上兄弟是 `ErrRevertUnordered`，`planned` 原样传出，网页得到 500（`unit_revert.go:171-182`、`revert_changeset.go:240-243`）。M8 的前提是"每个变更集只有一种操作"（`revert.go:89-95`）：P2 的单个 `move` 是第一个写出"新的父节点与新的名称同在一个条目里"的操作（3.5.4）。
- **编辑锁**（M5）：`EditLock.GuardWrite`（`page/app/edit_lock.go:44-74`）：`OpContent` 被任何不是这次写所带的活着的会话拒绝（`:47-48`；不带会话的写，本人的编辑器也拒），`OpDelete` 只被别的账户的拒绝（`:49-50`），别的操作放行（`:51-52`）；被拒时 `domain.Locked(page, user, name)` 只带一个 `Lock`（`page/domain/errors.go:125-131`），选的是子树次序里第一个被持着的页（`edit_lock.go:68-72`；子树逐层读，每层按次序值与 id，`page/adapter/postgres/store.go:243-263`）。`LockHolders.Of(ctx, ids, now)` 读这些页活着的锁，带显示名、按页 id 排（`page/app/lock_holders.go:27-51`），不加锁。节点的 id 是 UUIDv7（Go 1.27 的 `uuid.NewV7`，`page/app/unit_create.go:63`），同一个进程里按生成的次序递增（`$GOROOT/src/uuid/uuid.go:205-262`）：后建的页 id 大。
- **链接改写**（M6）：唯一的参与者 `linking.Rewrite.Participate`（`linking/app/rewrite.go:47-89`）：`!m.UpdateLinks` 就什么也不做（`:48-51`）；要改写的页里有被编辑的就以 `linking.pages_locked` 拒绝（预检 `refuseLocked`，`:72-74`、`:134-145`，`Locks` 列出全部、没有上限，`linking/domain/errors.go:22-28`）；之后逐页 `ParseNow`（取不到预算立即答 503，`rewrite.go:236`），写回时守卫的 `page.locked` 换成 `linking.pages_locked`（`:147-161`）。观察者不看 `UpdateLinks`、照样重新解析（`bootstrap/linking.go:25-38`）。`Relocation` 只在同一父节点下设上 `recased`（`linking/domain/change.go:73-77`）；改写的样例的 harness 自己算 `recased`、不调 `Relocation`（`linking/domain/rename_cases_test.go:88-91`）。
- **历史的读**（M8）：`ListChangesets.Execute(ctx, notebookID, LogQuery)`（`page/app/list_changesets.go:51-108`，次序：游标 400 → 笔记本 404 → `page_id` 404（回收站里的照收）→ `client` 422 → `limit` 422）；`GetChangeset.Execute(ctx, id)` 与 `ListChanges.Execute(ctx, id, limit, cursor)` 在变更集自己的笔记本上判定 `page.history`（`page/app/history.go:39-60`），不收笔记本；`ListVersions.Execute(ctx, id, limit, cursor)`、`GetVersion.Execute(ctx, id, revision)` 按节点 id，活着或在回收站里的页都行（`readableAny`，`page/app/readable.go:45-52`）。游标都经 `shared.EncodeCursor`，`limit` 1–100、默认 50（`shared/cursor.go:21-92`）。日志的前三项变化由条目与版本行一起给出、按节点 id（`FirstChanges`，`page/adapter/postgres/queries/history.sql:110-130`），`node_count` 只数条目（`:63-71`）；REST 的节点的类型是 `page` 或 `asset`（`page/adapter/http/history.go:144`）。
- **客户端**：`domain.Client.Valid()` 收 `web`、`api`、`cli` 与 `mcp:` 加 1–128 个不是 `unicode.IsControl` 的字符（`page/domain/client.go:20-30`）；两张表的检查是 `client ~ '^mcp:[^[:cntrl:]]{1,128}$'`（`00014_page_changesets.sql:12-13`、`00017_page_edit_sessions.sql:14-15`）。`shared` 的 `unshowable` 是 Cc、Zl、Zp 与 Bidi_Control（`shared/name.go:34-36`），S3 只为包外的测试导出了它（`shared/export_test.go:3-4`）。`mcpserver.Call.Client` 是清理过的名称，没有时是 `unknown`（`platform/httpserver/mcpserver/tool.go:66-77`）；没有 `domain.MCPClient`（`shared/client_name.go:13-18` 只说 P2 会写它）。
- **MCP 的平台与 P1 的工具**：
  - `ToolError{Err, Text, Extra}`（`mcpserver/errors.go:21-25`）；错误的成员先抄 `Extra`，再写 `code`、`message`、`fields`、`lock`、`locks`、`retry_after_seconds`，同名的盖过 `Extra`（`errors.go:83-99`）；整个错误的答复过了 `MaxAnswerBytes` 就答 `internal_error`（`errors.go:166-177`、`:197-201`，`answer.go:101-112`）。
  - `Fit(n, build)` 倍增再二分地找放得下的项数（`answer.go:127-175`）；`TextBytes` 与 `MemberBytes` 是两种量（`answer.go:25-96`）。
  - page 的 `adapter/mcp`：`list`、`read`、`open_notebook`（`page/adapter/mcp/tools.go:35-45`）；`toolErr` 只把 `*domain.PathNotFound` 写成带路径与提示的文字，以 `%q` 引路径（`page/adapter/mcp/text.go:17-24`），四个提示同样（`page/domain/path.go:123-147`）。这个文字今天就可能过 48 KiB：`AttachmentHint` 在 11 段的路径上 67,590 字节、`PageHint` 在 10 层的页上 61,448 字节，答 `internal_error`（P1 的 bug，设计审查 A 实测，3.10.3）。
  - 路径：`ParsePath`（`page/domain/path.go:37-81`）、`NodePaths.Find`（每段一次点查，不加锁，`page/app/node_paths.go:35-70`），它的注释说按 id 给路径随 P4（`:10-12`）。版本令牌只有写法 `VersionToken.String()`（`page/domain/version_token.go:11-18`），没有解析。
  - 内置的说明：两个入口共用 `pathsAndTools`（`page/adapter/mcp/instructions.go:31-48`），每个不超过 4 KiB 的 `TextBytes`（`page/adapter/mcp/instructions_test.go:15-27`）；`TestTheBuiltInInstructionsNameEveryTool` 要清单里每个工具在它的入口的说明里有一行 `- <名>:`（`bootstrap/mcp_tools_test.go:189-205`）。
  - 码的表 `mcpToolCodes()`（`bootstrap/permission_matrix_mcp_test.go:71-78`），`TestEveryMCPToolHasItsCodes`（`:83-105`）与 `TestTheMCPToolsErrorsCarryTheirCodes`（`bootstrap/mcp_reads_test.go:401-451`，每行的每个码都要答出过）；矩阵的 `mcpRow` 有 `write`、没有 `after`（`permission_matrix_mcp_test.go:27-36`），写的格各在库的副本上跑、至多 8 个同时（`:389-393`），`check` 数了、REST 的 `after` 也没数（`permission_matrix_test.go:279-283`）；`TestThePermissionMatrixCoversEveryTool` 要 `readOnlyHint` 为假的工具的行有 `write`（`:187-238`）。
  - `TestToolStringsDoNotAnswerInternalErrors`（`bootstrap/mcp_reads_test.go:562-606`）的参数只有被测的那一个（`argumentsAt`，`:528-542`），schema 先以缺了必填的成员答 `bad_request`，而断言只要"不是 `internal_error`"：P2 的 18 个新的字符串参数里 11 个的 NUL 到不了用例（设计审查 C-I1）。
  - 名称：握手的中间件给每个方法它的客户端（`mcpserver/log.go:48-71`、`names.go:47-99`）；`TestTheClientNameIsTheTokens` 备好了三个应用（同一个库上的第二个、重启的第三个，`bootstrap/mcp_names_test.go:47-88`）。
- **HTTP 的请求体**：
  - 逐路由的中间件：请求信息 → 期限 → 请求体上限（`MaxBytesReader`，不看 `Content-Length`）→ 认证（把凭证的限流键 `pat:<id>`、`session:<id>` 放进 ctx，`platform/httpserver/api.go:277-279`、`:322`）→ 限流 → 结构检查（bodyshape 的 `io.ReadAll`）（`api.go:190-214`、`:270-275`；`limit.go:24-36`；`bodyshape/middleware.go:9-37`）。
  - 放宽的路由：page 的两个（`PUT …/content`、`POST …/pages`，31,522,816 字节，`page/adapter/http/limits.go:5-17`）与 `/mcp`（`bootstrap/mcp.go:24-28`，在 `bootstrap/wire.go:170-171` 并进同一张表）；这些路由的期限另加 `server.read_timeout`（`api.go:246-265`），连接的读期限也是它（30 秒，`platform/httpserver/server.go:44`、`server/configs/config.yaml:11`）。
  - 没有一个逐路由的中间件持着计数的资源；httpserver 不能导入 `shared`（`archtest/rules_test.go:40`），它的平台码里没有 `server_busy`（`platform/httpserver/problem.go:16-28`）。契约的顶层码是 `bad_request`、`payload_too_large`、`rate_limited`、`internal_error`（`api/openapi.yaml:17`）；`createPage`、`putPageContent` 自己声明了 `server_busy`（`api/modules/page.yaml:56`、`:156`），说明里写的次序是"先查正文"（`:39-55`、`:121-145`）；`/mcp` 不在契约里。
  - SDK 的无状态处理在答完之后 `defer session.Close()`（`sdk/mcp/streamable.go:440`），`ServerSession.Close` 等进行中的方法结束（`sdk/mcp/server.go:2136-2165`；`sdk/internal/jsonrpc2/conn.go:497-509`），所以 `mcpserver.ServeHTTP` 在工具的处理函数结束之后才返回，客户端走了也一样。
  - 网页的正文保存遇到带 `Retry-After` 的 503 重试至多 3 次（`web/…/stores/page-editing.ts:13-14`、`:336-342`）。
- **字符串的查找**：Go 的 `strings.Index` 对长过 `bytealg.MaxLen`（arm64 上 32、amd64 上 64）的模式在几次误中之后改用 Rabin–Karp（`$GOROOT/src/internal/stringslite/strings.go:28-79`，`:68-75`），哈希是固定的 32 位多项式、每次撞上都比整个窗（`$GOROOT/src/internal/bytealg/bytealg.go:71-92`）：一个构造的 `old_string` 让每个窗都撞，O(n·m)；`strings.Count`、`ReplaceAll` 循环用它（设计审查 B-I2 实测：5 MiB 的页上 120 KB 的构造串 9.85 秒；1 MiB 70.2 秒，它的复核实测，`review-B/rk/rk-1mib.txt`）。
- **CI**：组合根的两份在 `go test -race -count=1 -timeout 15m` 之下（`Makefile:178`、`:207-213`），作业的 25 分钟不先到；守卫与成本两份的 `go test` 不带 `-timeout`，每个测试二进制在 Go 默认的 10 分钟之下（`Makefile:215-242`）。
- **e2e**：脚本化的客户端 `mcpClient(baseURL, credential, {notebook?, client?})` 答 `{isError, structured, text, content}`（`e2e/fixtures/mcp.ts:108-144`）；`expectNewPage` 断言正文为空、版本号 1（`e2e/fixtures/assert/page.ts:15`）；`expectMoved` 要名称不变（`:128`）；MC1 第 5 步把笔记本入口的工具写死为 `["list", "read"]`（`e2e/stories/mcp/mc1-connect.spec.ts:164`）；C4 的 API 版本在 `e2e/stories/collab/c4-guard.spec.ts:39-77`。

## 2. 目标与范围

做的事对应 00 号文档第 7 节 P2 一行与第 3 节第 4（`versions`、`log`、`changeset`）、5、6（`batch` 之外）、10、11、12（`mcp:<名>`）条的 P2 部分：

- **S1**：
  - 写的工具 `write`（补建父页、未改动、`base_version` 覆盖）、`edit`（唯一、`replace_all`、CRLF、重做，线性的匹配）、`move`（一步的移动加改名、大小写的改名、`update_links`）、`delete`（`recursive`），四个新用例与它们的码的次序；
  - 版本令牌的解析与比较（写的对象是锁下按路径找到的节点，令牌只比较）；
  - 撤销的一步的放回：MCP 的移动加改名撤销得了（00 号文档 4.12 第 2 条里单个节点的部分，从 P3 移来）；
  - 编辑锁对 MCP 删除的规则；
  - 客户端检查的显式范围与 `domain.MCPClient`，最后一跳；
  - 错误的文字、`lock` 与 `locks` 的形状与上限，每个错误的答复在 48 KiB 之内（含修正 P1 的 `page.not_found`）；
  - 大的请求体的并发界线（整个服务两个、每个凭证一个，REST 的正文路由共用）；
  - 测试：锁的每一格（4.10 的表）、交错、`update_links=false`、`schema_test` 的六类字符、矩阵与码的表、NUL 的检查到达用例；e2e 的 MC3、MC4、C4 的 MCP 版本。
- **S2**：`versions`、`log`（`actor: "me"`、按客户端与路径）、`changeset`（按笔记本的入口），它们的测试与 MC8。

不做：`batch`（P3；本文第 3.20 节写 P3 复用什么）；撤销的多轮调度、后备与临时名称（P3，00 号文档 4.12）；`grep`、`search`、`backlinks`、`index` 与模块根的 `page.NewNodePaths(pool)`（P4）；还原、恢复、撤销的工具（第 11 节第 1 条：agent 只读历史）；编辑会话与接管（第 11 节第 2 条）；其余故事的 MCP 版本与接入文档（P5）。

**P1 第 8 节交给 P2 的各项**，逐项落实：

| P1 的项 | 落实 |
|---|---|
| `ParseVersionToken` 与"令牌的节点是路径的节点"；写的工具在单元的锁下按路径找（同一个 `PathStore`，在单元的事务里） | 3.4；3.8 的每个写的用例在 `do` 里以 `NodePaths` 找（`PathStore` 的语句经 `queries(ctx)` 进入单元的事务，`page/adapter/postgres/store.go:42-44`）；新建经 `NodePaths.Walk`，不另建缓存（2.1 第 1 条） |
| `domain.MCPClient(Call.Client)`、`mcp:` 的前缀与 `Client.Valid`、两张表的检查；最后一跳（两种来源与重启之后记下的名称） | 3.3；最后一跳在 `TestTheClientNameIsTheTokens` 的三个应用上各写一页（3.13.3） |
| 工具的错误的文字：`page.locked` 带路径与持锁人、`page.revision_mismatch` 带当前的令牌；`lock`、`locks` 与 `Extra` 有上限，错误的答复在 48 KiB 之内 | 3.10（`Extra` 只补问题没有的成员与锁的三个、`locks` 至多 20 项加 `locks_count`、`ToolError.Fit`）与 3.11 的每个错误的文字；P1 的 `page.not_found` 带提示时今天就过 48 KiB、答 `internal_error`，S1 一并修好（2.1 第 4 条） |
| 写的工具在矩阵里的行带 `write` | 3.13.2：四行，`mcpRow` 加 `after`（在这一格的库的副本上读回），`after` 与 `check` 一样计数 |
| 内置的说明的"Tools"加写的工具与 `batch`、锁、版本令牌的用法 | 3.12：加 P2 的七个工具、版本令牌与锁；`batch` 由登记它的 P3 加（说明不提没有的工具，2.1 第 12 条） |
| 写的格用库的副本；`read` 与 `write` 之间同一路径换了一页的交错 | 矩阵的 runner 已经这样做（`permission_matrix_mcp_test.go:389-393`）；交错见 3.13.4 |
| 超过 64 KiB 的请求体的并发界线 | 3.2：整个服务两个名额、每个凭证至多一个；REST 的正文路由共用，门槛是 `server.max_body_bytes`（2.1 第 6 条） |
| 按工具的码的表 | 3.13.1、3.17：七行，每行的每个码在 `TestTheMCPToolsErrorsCarryTheirCodes` 里答出 |
| 组合根两份在 P2 再量：第二份的 go test 一步到过 7:54（15 分钟的约 53%），P2 的写的矩阵与整个程序的测试多半落在第二份 | 6.5：按名字算出 P2 的测试落在哪一份、各加多少；规则是较慢的一份的 CI 测试一步过了约 7.5 分钟（`-timeout 15m` 的一半）就在 S1 分出第三份，不提高 `-timeout`；预计第二份多半过线（约 7:25–8:10），所以 S1 照计划分（5.1 第 9 步） |

**00 号文档"移交的落实"里交给 P2 的**：

| 移交 | 项 | 落实 |
|---|---|---|
| [M4/P1 客户端的检查](handoffs/M4-P1-client-check.md) | 1 先检查名称；2 区域设置；3 最后一跳；4 写入选项 | 1：`domain.MCPClient` 以 `shared.CleanClientName` 再清理一次、空了是 `unknown`，从不因名称拒绝（3.3）；2：两张表的检查改为显式的范围、与 `shared.Unshowable` 同集合，`schema_test` 以一条语句核对每个码位，移交写的六个字符（U+0085、U+009F、U+007F 拒，U+2028 现在也拒，U+200B、U+00AD 收）都在其中（3.3.2）；3：3.13.3；4：`move` 的 `update_links` 经新用例 `RelocatePath` 的 `UnitSpec.Options` 交给参与者，REST 的十处不改（3.8.3） |
| [M5 编辑锁与 MCP 的写](handoffs/M5-locks.md) | 1 告诉 agent；4 测试 | 1：`page.locked` 的文字带路径、持锁人与"是你"，`lock` 带 `path` 与 `you`，不说自动重试、不能接管（3.11.5）；4：锁的每一格经 MCP 的整个程序测试、交错、C4 的 MCP 版本（3.13.3、3.13.4、4.4）。2（batch）是 P3 的，3（接管）不给，5（闲置上限）不涉及 |
| [M6/P4 链接改写](handoffs/M6-P4-rewrite.md) | 1 一个单元里几次改名；2 `update_links=false` | 1：单个 `move` 是一步（`Unit.Relocate`，3.5），batch 的是 P3 的；2：整个程序的测试：链接与正文不改、被锁的页不拒、索引照样维护、`checkLinks`（3.13.5） |
| [M7 附件与 MCP](handoffs/M7-assets.md) | 4 写 | 不上传；`write`、`edit` 写的正文里指向附件的链接经同一个观察者进索引，`move` 附件经同一个参与者改写（MC4 的一步，4.3） |
| [M8/P3 batch 的整组撤销](handoffs/M8-P3-revert.md) | 2 一条语句的放回（单个节点）；"另"的 `log` | 2：一个既移动又改名的放回是一步，经 `RelocateNode`，计划只核对最后的键（3.5.4；P2 的单个 `move` 是第一个写出这种条目的操作，多轮的调度、后备与临时名称仍是 P3 的）；`log` 经 `ListChangesets.AtPath` 调同一个用例（3.16.2） |
| [M8/P4 MCP 开的后台任务](handoffs/M8-P4-background.md) | 3 `log` 看得到任务的变更集 | 不用改：任务的变更集是普通的变更集，`log`、`changeset` 照常读到（3.17.3 的测试里有一个导入的变更集）；后台的撤销也撤销得了 MCP 的移动加改名（3.5.4 的整个程序测试） |

### 2.1 对 00 号文档的修订

P2 的文档提交随这份文档一起改 [`00-M9-design.md`](00-M9-design.md)（照 P1 的做法，以脚本落下，每段原文恰好出现一次才改，`scratchpad/m9/p2/apply-00m9.py`）。行号指 `2a888a6f` 的 00 号文档（P1 的 S3 的文档改了它的九处，都在原地，这里引的行没有挪；第 11 节在 :660 之后多了七条，第 12、13 节随之后移七行）。

1. **新建补建父页不经缓存**（4.10 的 `write` 一条 :228）："照 `ImportUnit` 在单元里缓存兄弟与祖先"改为：单元里以 `NodePaths.Walk` 逐段点查（同 `Find` 的 `RootByKey`、`ChildByKey`，经单元的事务），缺的祖先与叶子依次以 `Unit.CreatePage` 建。一条链至多 10 段，十段都缺时约 70 条语句（每个 `CreatePage` 约七条：`FindNodeIn`、`Ancestors`、`Children`、`CreateNode`、`CreateContent`、`RecordRevision`、`RecordItems`，`unit_create.go:28-82`、`unit_place.go:40-63`、`unit.go:264-291`；另有 `Walk`、变更集、搜索的入队与观察者）；`ImportUnit` 的缓存为一个单元里上百页的导入而设（`page/app/unit_import.go:12-18`），它的序号与"太深就跳过"的规则 `write` 都不要（撞名拒绝、太深 409）。缓存与它在移动、改名、删除之后的作废是 P3 的 batch 的事（3.20）。P1 第 8 节写的"同一个 `PathStore`、在单元的事务里"照做。
2. **锁的成员**（4.7 的"锁的拒绝" :178；4.10 的锁表之后 :254）：`lock` 是 `{page_id, path, user_id, display_name, you}`；MCP 的删除被拒时另有 `locks`（子树里每个被持着的页，子树的次序：逐层、每层按次序值）与 `locks_count`，这时 `lock` 是 `locks` 的第一项而不带 `path`（路径在 `locks` 里，不重复）；`linking.pages_locked` 的 `locks` 同形。`structuredContent.error` 的 `locks` 至多 20 项（`MaxErrorLocks`），多的只数在 `locks_count` 里；带路径的项由 `ToolError.Fit` 放到 48 KiB 之内（3.10）。文字只引这次调用自己的路径，以 `"` 原样引起（不用 `%q`：它把 U+3000、U+00A0、U+200D 一类写成转义，文字就不是路径，标题里没有 `"` 与 `\`，原样引起不会有歧义，2.1 第 4 条），别的页的路径只在结构里（3.10.3 的计算：一个最贵的路径在文字与 `message` 里各一份已是约 33.7 KB）。
3. **`move` 的码的次序**（4.15 的"通用" :365）：通用的次序把 503 排在单元里的 422、409 之前；`move` 的 503 来自参与者在单元里不排队地取预算（`linking/app/rewrite.go:236`；13.1 第 19 条的"持着锁时不排队"），在 409 之后。`move` 的完整次序：`bad_request` → 取值 422（两个路径、两者的类型）→ 笔记本 → 节点（`from`，不加锁）→ 403 → 单元里：节点（`from`）→ `to` 的父页链（第一个不成的段决定：缺的 `page.not_found`、路过附件的 422；设计审查的复核 B 的 BC-N6）→ 422（名称的规则）→ 409（成环、名称、太深）→ `linking.pages_locked`（预检）→ 503（改写的解析）→ `linking.pages_locked`（守卫）（3.8.3）。`write`、`edit` 的 503 照旧在单元之前；`write` 新建的 409 `page.too_deep` 排在要补建的祖先的 422 之后（3.8.1）。
4. **错误的答复的上限与引路径的写法**（4.7 的"要读资源的文字由模块写" :177）：加一句"列出或重复路径的错误经 `ToolError.Fit` 填，路径以 `"` 原样引起"：mcpserver 按与 `Fit` 同样的找法（倍增、二分）取放得下的 k，`build(0)` 也放不下才是 `internal_error`（3.10.2）。P1 的 `page.not_found` 的文字带写下的路径与提示里的另一个路径，今天在 `main` 上就过 48 KiB、答 `internal_error`（设计审查 A 实测：11 段的路径上的 `AttachmentHint` 67,590 字节，10 层的页上的 `PageHint` 61,448 字节；`page/adapter/mcp/text.go:22`、`page/domain/path.go:123-147`）：S1 改为经 `ToolError.Fit`，放不下时去掉提示（33,818 字节，3.10.4），并以 `"` 原样引路径（P1 的文字与四个提示都改，3.11.5）；第 9 节记下修好之前与之后的量。
5. **`edit` 的上限、唯一与匹配**（4.10 的 `edit` :232-237；4.14 的"一次调用的字符串参数"一行 :341）：`old_string` 1 字节到 1 MiB（`MaxEditOldBytes`，`/mcp` 的请求体上限给"正文之外的 1 MiB"留的就是它，`bootstrap/mcp.go:24-28`），`new_string` 至多 5 MiB，替换之后的正文至多 5 MiB（`new_string` `too_long`，在替换之前算出、不先拼出来；CRLF 的规则之下以换过的两个串算）；两者都是 UTF-8、不含 NUL。唯一按"第一处之后还有没有一处"判断，重叠的也算（`aa` 在 `aaa` 里是两处；Claude Code 按不重叠的计数，这里更严，写进说明）；"found N times"的 N 是不重叠的计数与 2 中较大的。**匹配是线性的**：查找、第二处、计数、`replace_all` 与 CRLF、NFC 的再找都经同一个 KMP 的匹配器，从不以 `strings`、`bytes` 的查找、切分与替换（`Index`、`Count`、`ReplaceAll` 这一族）拿 agent 给的串作模式（它们对长的模式走 Rabin–Karp，一个构造的 1 MiB 的 `old_string` 在 5 MiB 的页上约 70 秒，设计审查 B-I2 与它的复核实测 70.2 秒），也不拿它编 `regexp`（模式就是程序，k 个字节的 `old` 让匹配是 O(n·k)：1 MiB 的 `a` 的页上 1,000 个 `a` 加 `b` 4.8 秒，第二次复核 B 实测）；最坏的一次调用本机约 0.16 秒（3.9），所以不在匹配里看期限（输入的上限管着它，3.9）。`write` 的正文至多 5 MiB、路径至多 4,096 字节，所以一次调用的字符串参数合计在 6 MiB 加一个路径之内。
6. **大的请求体的界线**（4.14 的请求体一行 :340；4.15 的"边缘" :364；第 10 节的内存一行 :607；第 11 节主持者的决定第 16 条 :652）：整个服务同时至多两个大的请求体，**每个凭证（限流的键：`pat:<id>`、`session:<id>`）至多一个**；按路由的门槛：`/mcp` 是 64 KiB（`mcpserver.MaxOtherBytes`），REST 的两个正文路由是 `server.max_body_bytes`（1 MiB）：任何路由都收 1 MiB 的请求体，在这之下计数挡不住什么，而 `/mcp` 的放大约 40 倍（37.8 MB 的请求体分配约 1.4 GB），64 KiB 起就要算。同一个凭证的第二个大的请求体在 2 秒之内等它的第一个，别的凭证的不等它；取不到名额的至多等 2 秒（同 `page.parse_max_wait` 的默认），然后 503 `server_busy`、`Retry-After: 1`，info 的一行点名持着名额的凭证与路由；`Content-Length` 超过路由的上限的立刻 413、不取名额。`/mcp` 的边缘次序在 413（bodyshape）之前多两步：413（声明的长度超过路由的上限）→ 503（没有名额）（3.2）。第 11 节第 16 条与第 10 节的那一行随之写细：REST 的两个正文路由的门槛是 1 MiB 而不是 64 KiB（第 10 节说它们"可以共用这份预算"）；取不到的先等 2 秒再答 503（不等的话，一个 agent 并行的第二个大的写立刻就是传输的失败，3.2.1）；每个凭证至多一个。第 10 节另加一行**剩下的风险**：两个凭证（一个账户的两个 PAT 也算）仍占得满两个名额，每个慢的请求体占着它至多 `read_timeout` 30 秒，每 30 秒两个请求、每分钟 4 个，按凭证的桶（每分钟 1,200 个）看不到；其间超过门槛的 MCP 调用与超过 1 MiB 的网页保存等 2 秒后 503（网页重试三次，约 11 秒后失败）；info 的一行点名凭证，管理员撤销那个 PAT 或结束会话；缓冲请求体的反向代理把慢的发送者挡在应用之外，P5 的接入文档按路由写：`/mcp` 与 page 的两个正文路由缓冲（nginx 的 `proxy_request_buffering on`，它的默认），上传与导入照 README 的部署一节不缓冲（`README.md:357`）；Caddy 默认不缓冲请求体，直接暴露也不，用它们时这个风险照旧（设计审查的复核 A 的 MN-2）；写给 M12（`read_timeout` 不改）。**每个凭证一个名额的代价**（复核 A 的 MN-1，主持者接受）：名额在读请求体之前取，一个大的写占着它的是上传加处理，10 Mbit/s 上 5 MiB 约 4.2 秒、1 Mbit/s 上过约 250 KB 就过 2 秒，所以同一个凭证并行的第二个大的写在慢的上行上等满 2 秒、答 503（修订之前它会拿空着的第二个名额）；一般的页（100 KiB 在 10 Mbit/s 上约 80 ms）不受影响。照旧每个凭证一个、等 2 秒、503，不加长等候、不加按凭证的队列；第 10 节的那一行与 M12 的移交写上它，说明加一行叫 agent 一个接一个地发大的写（3.12），P5 的人工清单加一次限速的上行的试用。
7. **按 id 给路径随 P2**（第 7 节 P1 一行 :427、P4 一行 :430；[P1](01-P1-platform-reads.md) 2.1 第 19 条与 `page/app/node_paths.go:10-12` 的注释）：`NodePaths.Paths`（按 id 给活着的节点的路径，一条语句：已有的 `LinkTargetsByIDs`，`page/adapter/postgres/queries/links.sql:25-42`）随 P2 进 page：`move` 的 `linking.pages_locked` 与 `changeset` 的各项要它。模块根的 `page.NewNodePaths(pool)` 与 linking 的 `Deps.NodePaths` 仍随 P4。
8. **`changeset` 的各项带路径，节点的类型是 `attachment`**（4.9 的 `changeset` :220；第 5 节的工具表 :410）：每项的节点另带它现在的路径（活着的；回收站里的与清理掉的为 null），由 `NodePaths.Paths` 为这一页的节点读一次：agent 只能按路径调别的工具，没有路径的 id 走不下去。节点的类型同别的工具写 `page` 或 `attachment`（REST 的 `asset` 在适配器里换，同 `list`、`read`）。`log` 的前三项变化照 M8 4.7 的形状，不带路径（它是概要）。
9. **`versions` 读一个版本的元数据与它的次序**（4.9 的 `versions` :218；4.15 的"通用" :365）：第一块是 `{path, kind, revision, current_version, offset, column, lines, total_lines, byte_size, next_offset, next_column}`：`revision` 是答出的版本自己的号，可能大于问的（一个版本行覆盖它的变更集里连续写的几个版本号，`page/app/get_version.go:27-33`）；`current_version` 是这一页现在的令牌。`limit` 在列表里是版本数（1–100，默认 50），读一个版本时是行数（默认 2,000）；`cursor` 只用于列表、`offset` 与 `column` 只用于读，用错了答 422 `not_allowed`；`revision` 不在 1–2,147,483,647 之间是与资源无关的 422（P1 2.1 第 6 条的规则；REST 的 `getPageVersion` 对它们答 `page.version_not_found`，不改）。用错了模式的 422 排在一切之前，带着它们的调用不读游标：所以读一个版本时坏的 `cursor` 答 422 而不是游标的 400，是 4.15 通用次序（`bad_request` 在先）的例外，4.15 加这一句。
10. **`write` 覆盖时内容相同**（4.10 的 `write` 第二条 :229）："与当前相同不写、答 `changed: false`"写细：先比令牌的节点（不是路径的节点就 409），节点相同而内容相同时，即使令牌的版本号过时也答 `changed: false` 与当前的令牌（同 REST 的 `unit_content.go:62-64`：内容已经是调用者要的样子，答 409 只会让它重读、再写同样的内容）；同样的写也不被打开的编辑器拒绝（不经守卫，13.1 第 29 条）。
11. **`write` 的答复**（4.10 的 `write` 的答复 :231）：`{path, node_id, version, changed, created, created_count, changeset_id, at}`：`path` 是这一页现在的路径（找到的祖先的名称照库里的大小写）；`created` 是这次建的节点，从根往下、这一页在最后（覆盖与未改动时为空），经 `Fit` 填、`created_count` 是总数；未改动时 `changeset_id`、`at` 为 null。
12. **说明不提 `batch`**（[P1](01-P1-platform-reads.md) 第 8 节的"内置的说明的 Tools 加写的工具与 `batch`"）：P2 加它的七个工具、版本令牌与锁的用法；`batch` 由登记它的 P3 加：说明提一个不存在的工具，agent 会去调它，`TestTheBuiltInInstructionsNameEveryTool` 也只要求清单里有的（3.12）。00 号文档不改：它的"移交的落实"已写"`batch` 写说明（P3）"，这一条更正的是 P1 第 8 节。
13. **`delete` 的 `recursive`**（4.10 的 `delete` :242）："这个检查在单元里、笔记本行的锁下"照做，写细：单元先读子树（`Unit.Subtree`），子树多于一个节点而没有 `recursive` 时答 422，文字给出其下的页数与附件数（字段的消息里不写路径，3.10.3），否则以同一个子树删除（`Unit.DeleteSubtree`）：子树只读一次。
14. **客户端的检查的写法**（4.2 的 `changesets`、`edit_sessions` 一行 :95；"移交的落实"的 M4/P1 一行 :440）：`shared` 的 `unshowable` 改名导出为 `Unshowable`（S3 为测试导出的那一行随之去掉），`Client.Valid` 用它；两张表的检查以 PostgreSQL 的正则的 `\uXXXX` 写出范围，从 U+0001 起（`text` 收不下 NUL）（3.3.2）。
15. **`move` 的 `to` 与附件的扩展名**（4.10 的 `move` :238）：`to` 总是节点的完整新路径，从不是"移进去"的文件夹；`to` 的父页与 `from` 的父页按键相同时判定 `node.rename`，否则 `node.move`（两者都是写者，`access/domain/rules.go`）；答复的路径照找到的父页的名称。附件改名照 `CheckAssetRename` 要有一个扩展名，扩展名可以换（`page/domain/name.go:29-41`，样例 `035-attachment-new-extension`），:238 的"保留扩展名"随之改写。
16. **MC3 的 PG14 断言与 MC1 的第 5 步**（第 9 节的故事表 MC3 一行 :546 与 MC1 一行 :544）：MC3 照 PG14 的 API 版本断言活动：这本笔记本的所有者以自己的 PAT 经 MCP 写，管理员移出所有者之后，无主的列表里它的大小与最后的活动是 MCP 的写（`e2e/stories/page/pg14-notebook-activity.spec.ts:16-43`）。MC1 第 5 步（官方 TypeScript SDK 的客户端）列出的工具改为照 `tools.json` 的笔记本入口，不写死（本文 4.5）。
17. **交错的写法**（4.15 的"交错" :371）：P2 的几个照 3.13.4 写定：MCP 的正文写、`edit` 与开会话在正文行上相遇；MCP 的删除与本人的开会话、MCP 的删除与在它下面的新建（`recursive` 的检查在单元里）在笔记本行上相遇；`write` 新建与 REST 新建同名在笔记本行上相遇；`write` 覆盖与 REST 的改名在笔记本行上相遇（路径在单元里找）；`edit` 的重做与 REST 的正文写在正文行上相遇。"读与写之间同一路径换了一页"与"别的笔记本的令牌"没有并发的窗口，是顺序的测试。
18. **配置**（4.2 的"配置" :102）：P2 不加配置项；名额 2、每个凭证 1 个、等 2 秒、门槛是组合根与平台的常量（3.19）。00 号文档不改：4.2 已写"MCP 的其余量都是代码里的常量（4.14），不进配置"。
19. **历史的答复的上限**（4.7 的"答复的上限" :164；4.9 的 `log` :219 与 `changeset` :220；第 5 节的 `log` 一行 :409）：说明 4,096 字节、标题与显示名都是最贵的字符时，`log` 的一行带三项变化约 59.7 KB，`changeset` 的变更集加一项带路径的变化约 60.5 KB，都过 48 KiB（3.16.4 的计算），"到 48 KiB 就停"会一行、一项都给不出、游标不前进。改为：`log` 连一行都放不下时答第一行与放得下的前几项变化，这一行带 `changes_cut: true`（只有这条退回设它，13.1 第 31 条的"标出截断"；`node_count` 只数条目、变化由条目与版本行一起给出，靠数目推不出省没省，设计审查 C-M4）；`changeset` 的变更集只在没有 `cursor` 的第一页，第一页连一项变化都放不下时答 `changes: []` 与从头列的游标，带游标的页只有变化（一项总放得下）。4.7 的"单项不会超过它：每个写者控制的字符串都有上限（4.14）"随之改为"一项可能超过它：`log` 的一行与 `changeset` 的第一页各有退回（4.9）；别的工具的单项都不会"。
20. **撤销的一步的放回在 P2**（4.12 第 2 条 :296 与测试一条 :307；第 7 节 P2、P3 两行 :428–429；"移交的落实"的 M8/P3 一行 :445）：单个 `move` 是第一个写出"新的父节点与新的名称同在一个条目里"的操作，撤销它时计划先模拟"带着现在的名称移到旧的父节点下"，撞上那里叫这个名字的兄弟就是 `ErrRevertUnordered`，网页答 500（设计审查 B-I1 复现：`Drafts` 下有 `Plan` 与 `Plan v2`，`move Drafts/Plan v2.md → Final/Plan.md` 之后撤销它）。所以 4.12 第 2 条里单个节点的部分随 P2：一个既移动又改名的放回是一步，经 `moveSubtree` 与 `RelocateNode`（父节点、次序与名称一起改），计划只核对最后的键（`holder(to.ParentID, TitleKey(to.Name))`），连同父节点活着、不在自己的子树里（3.5.4）；P3 只留多轮的调度、三条后备、临时名称、写回的 `Base` 与后台的切批。:307 的"移动加改名的中间态"一项由 P2 测（单个 `move` 的），P3 测 batch 里的。第 7 节的 P2 一行加"撤销的一步的放回"（连同大的请求体的界线、线性的匹配与"两个 Step"），P3 一行的 S2 去掉"一条语句的放回"；"移交的落实"的第 2 项写明 P2；"写给后面的 M"里来源区的守卫的一条（:461）在 4.10 的 `Unit.Relocate` 之外加"与撤销的一步的放回，4.12"（设计审查的复核 B 的 BC-N2）。
21. **一步的移动加改名与 REST 的两步**（4.10 的 `move` 的"一步" :239）："只改大小写的名称……与 REST 的两步（移动、再改大小写）结果相同"写细：只有一种写法不同：名称在笔记本里唯一时，带路径的 wikilink 以旧的末段作显示的文字（`[[A/x|x]]`），一步的移动加改名得 `[[X|X]]`，REST 的两步得 `[[X|x]]`（M6 的显示规则只在目标带 `/` 时跟着改，两步的第一步已把目标改成不带 `/` 的名称，`linking/domain/rewrite.go:387`）；另有同名的页时两者都写带路径的 `[[b/X|X]]`（设计审查的复核 B 的 BC-M2）。一步只用一次 M6 的规则，结果更好，样例 047 照一步在名称唯一的树上写出、整个程序的比较避开这种写法（设计审查 B-M1，3.5.3）。同一条的 `Unit.Relocate(id, parent, position, name)` 改为 P2 的 `Unit.Relocate(id, Placement{parent, name, field})`（没有次序：在新的兄弟里排在最后，3.5.1；复核 B 的 BC-N2）。
22. **组合根各份的界线与分份**（第 11 节主持者的决定第 28 条 :664）："组合根两份的界线 S3 移一次之后不再动"没有说一份过线时怎样动：P2 的第二份多半要分出第三份（6.5），别的份也可能过线。"不再动"防的是凭一次运行的摆动去挪界线（单独跑一次的时间前后可差一成以上），不是冻结（P2 的设计审查之后主持者写明）。改写为一条管组合根每一份的规则（设计审查的复核 C 的 C-NM2，主持者的决定）：一份**过了线**，是两次运行的平均里它在 CI 的测试一步过了约 7.5 分钟（go test 的 15 分钟的一半）；过了线的一份，只在一个邻居挪过之后两次运行的平均仍在线之下时，才挪它们之间的界线、把测试给那个邻居，否则那个 Step 按测试名把它再分一份；从不提高 `-timeout`。界线只在这时与分出一份之后紧接着重新平衡时移动，都凭两次运行的平均（本机每份单独跑两次、CI 的两次运行）；界线、每份本机的两次运行与它们的平均、CI 的测试一步写进那个 Step 的审查记录。本机的 `make test-go` 以同样的份跑组合根（P2 的 S1 起，复核 C 的 C-NM3），同一条规则管两处。条目的来历（S3 审查 C-N4、S3 移到的 `^Test([A-D]|E([a-u]|very[A-R]))`、[P1](01-P1-platform-reads.md) 6.1）留着（6.5）。

### 2.2 需要负责人确认

无。

- 第 11 节负责人的四个决定照做：agent 只读历史（`versions`、`log`、`changeset`，没有还原、恢复、撤销，3.16；3.5.4 修的是网页上的撤销，人做的）；MCP 的写不开会话，被编辑的页（本人的编辑器也算）拒绝写、编辑与删除，答 `page.locked` 并说出持锁人，不能接管（3.6、3.11.5）；`AGENTS` 与说明不变；MCP 不开后台任务。
- 2.1 的各条是实现的写定、P1 实现之后的更正或 00 号文档内部的不一致，不碰第 11 节负责人的决定，由主持者定（负责人可以改判）。第 6 条是主持者的决定第 16 条的实现，写细了它的三处（REST 的门槛 1 MiB、等 2 秒、每个凭证一个）并写下剩下的风险，都是主持者一级的；第 20 条把 4.12 的一部分从 P3 挪到 P2，Phase 的划分是主持者的；组合根分出第三份（6.5）与第 22 条对第 28 条的改写（过了线的一份只在邻居挪过之后仍在线之下时把测试给它，否则再分一份，从不提高 `-timeout`；本机照同样的份跑）都是主持者为 P2 定的规则，第 28 条本身是主持者的决定，不是负责人的。第 6 条剩下的风险是任何一个账户都能造成的可用性的风险（设计审查的复核 A），主持者接受、交 M12；负责人在 00 号文档第 10 节与 M12 的移交里看得到它。第 5 条的 `old_string` 1 MiB 是 4.14 的"其中正文与 `new_string` 合计 5 MiB"之外那 1 MiB 的写定。

## 3. 服务端

### 3.1 S1 的文件

| 文件 | 包 | 新/改 | 内容 |
|---|---|---|---|
| `platform/httpserver/api.go` | httpserver | 改 | `APIConfig` 加 `LargeBodies`、`LargeBodyAbove`、`LargeBodyWait`；`NewAPI` 的检查；`Middlewares` 在限流与结构检查之间加 `largeBody`（3.2） |
| `platform/httpserver/largebody.go` | httpserver | 新 | 整个服务的名额与每个凭证的一个、`largeBody`、`takeLarge`、`serverBusy`、满了时的 info 一行、读过门槛的前缀之后接回请求体 |
| `platform/httpserver/problem.go` | httpserver | 改 | 平台码加 `CodeServerBusy` |
| `platform/httpserver/largebody_test.go`、`api_test.go` | httpserver | 新、改 | 3.2.5 的测试；`TestNewAPIRequiresItsSettings` 照旧 |
| `platform/httpserver/mcpserver/edge.go` | mcpserver | 改 | `maxOtherBytes` 导出为 `MaxOtherBytes`（组合根拿它作 `/mcp` 的门槛） |
| `platform/httpserver/mcpserver/errors.go`、`answer.go` | mcpserver | 改 | `ToolError.Fit` 与 `ErrorFit`；`ErrorAnswerBytes`（模块的测试以它量错误的答复，3.10.2）；`Extra` 只补问题没有的成员与 `lock`、`locks`、`locks_count`；`locks` 至多 `MaxErrorLocks` 项加 `locks_count`；`Fit` 与错误共用找 k 的函数（3.10） |
| `platform/httpserver/mcpserver/errors_test.go` 等 | mcpserver | 改 | 3.10.5 的测试 |
| `bootstrap/deps.go`、`bootstrap/wire.go`、`bootstrap/mcp.go` | bootstrap | 改 | `apiConfig` 的名额 2、等 2 秒，`/mcp` 并进 `BodyLimits` 的那一处加它的门槛；测试的接缝 `wiring.largeBodyWait` |
| `bootstrap/api_config_test.go` | bootstrap | 新 | `TestTheAPIConfigBoundsTheLargeBodies`：`apiConfig` 给的名额与等候（不起应用、不连库） |
| `shared/name.go`、`shared/client_name.go`、`shared/export_test.go` | shared | 改、删 | `unshowable` 改名导出为 `Unshowable`，导出它的测试文件删去（3.3.1） |
| `page/domain/client.go`、`client_test.go` | domain | 改、新 | `Valid` 用 `shared.Unshowable`；`IsMCP`、`MCPClient`、`UnknownClientName`（3.3.1） |
| `server/migrations/sql/00044_page_client_checks.sql` | 迁移 | 新 | 两张表的客户端检查（3.3.2） |
| `server/sqlc.yaml`、`server/migrations/schema_test.go` | — | 改 | page 的一条加 00044；`TestChecksRejectCounterexamples` 以一条语句核对每个码位（3.3.2） |
| `page/domain/version_token.go`、`path_test.go` | domain | 改 | `ParseVersionToken`、`NotAVersion`（3.4） |
| `page/domain/edit.go`、`edit_test.go`、`edit_cost_test.go` | domain | 新 | `MaxEditOldBytes`、`CheckEdit`、线性的匹配器、`Replace`、`Replacement` 与它们的错误；与 `strings` 的差分测试；时间的守卫；只许匹配器拿 agent 的模式的静态检查（读 `old_string` 经过的三个包的非测试文件，3.9） |
| `page/domain/steps.go`、`steps_test.go` | domain | 新 | 每一步的规则：`CreationOf`、`MoveKinds`、`DeletionOf`（3.8，P3 的 `PlanBatch` 照用） |
| `page/domain/errors.go` | domain | 改 | `PathTaken`、`VersionMismatch`、`LockPaths`、`LockedAll`、`Recursive`、`AttachmentOnPath`（3.8、3.11） |
| `page/domain/path.go` | domain | 改 | `PathOfNames`（`PathOf` 改用它）、`MoveParentHint`、`Quoted`；四个提示以 `"` 原样引路径（3.7、3.10.4、3.11.5） |
| `page/domain/revert.go`、`revert_test.go` | domain | 改 | 计划的 `place`：移动加改名是一个放回，只核对最后的键；`ErrRevertUnordered` 的注释（3.5.4） |
| `page/app/ports.go` | app | 改 | `PathStore.PathsOf`、`NodeWriter.RelocateNode` |
| `page/app/node_paths.go` | app | 改 | `Walk`、`Paths`（3.7） |
| `page/app/unit_relocate.go` | app | 新 | `Placement`、`Unit.Relocate`（3.5.1） |
| `page/app/unit_move.go`、`unit_rename.go` | app | 改 | `moveSubtree` 收写入的语句；`nameOf` 收字段名 |
| `page/app/unit_revert.go` | app | 改 | 单元的 `place`：移动加改名是一步，经 `moveSubtree` 与 `RelocateNode`（3.5.4） |
| `page/app/unit_delete.go` | app | 改 | `Unit.Subtree`、`Unit.DeleteSubtree`，`Delete` 是两者（3.5.2） |
| `page/app/edit_lock.go` | app | 改 | MCP 的删除被任何活着的会话拒绝，答出全部（3.6） |
| `page/app/write_path.go`、`edit_page.go`、`relocate_path.go`、`delete_path.go` | app | 新 | 四个用例（3.8） |
| `page/app/*_test.go` | app | 新、改 | 用例的表格测试（码的次序）、`Relocate`、`Subtree`、守卫、`Walk`、撤销的一步的放回（`revert_test.go`） |
| `page/adapter/postgres/queries/nodes.sql`、`store.go`、`links.go`、`gen/` | postgres | 改 | `RelocateNode`；`PathsOf` 经已有的 `LinkTargetsByIDs`；`make gen` |
| `page/adapter/postgres/*_test.go` | postgres | 改 | `RelocateNode` 的唯一索引与 `nameTaken`；`PathsOf` |
| `page/adapter/mcp/tools.go`、`write.go`、`edit.go`、`move.go`、`delete.go` | mcpadapter | 改、新 | 四个工具（3.11） |
| `page/adapter/mcp/text.go`、`locks.go` 与它们的测试 | mcpadapter | 改、新 | 错误的文字与结构，路径以 `"` 原样引起；每个错误在最坏的取值处量（3.10.4、3.10.5、3.11.5） |
| `page/adapter/mcp/instructions.go` | mcpadapter | 改 | "Tools"一段（3.12） |
| `page/module.go`、`page/tools.go` | page | 改 | 建四个用例，交给 `mcpadapter.UseCases` |
| `linking/domain/change.go`、`change_test.go`、`rename_cases_test.go` | domain | 改 | `Relocation` 的大小写改名不看父节点；样例的 harness 的 `recased` 取自 `Relocation`（3.5.3） |
| `tools/md-fixtures/rename/047-move-and-recase.{json,md,out.md}` | 样例 | 新 | 跨父节点的大小写改名，含 `[[a/x\|x]]` |
| `api/mcp/tools.json` | 契约 | 改 | 四个写的工具，两个入口（3.18） |
| `api/modules/page.yaml`（与 `make gen` 的生成物） | 契约 | 改 | `createPage`、`putPageContent` 的说明加大的请求体的一句（3.2.3） |
| `bootstrap/mcp_writes_test.go`、`mcp_lock_test.go`、`interleavings_mcp_test.go`、`mcp_bodies_test.go`、`mcp_revert_test.go` | bootstrap | 新 | 3.13 的整个程序测试与交错，3.2.5 与 3.5.4 的整个程序测试 |
| `bootstrap/mcp_names_test.go`、`mcp_reads_test.go`、`mcp_client_test.go`、`mcp_test.go`、`permission_matrix_mcp_test.go`、`permission_matrix_test.go`、`revert_property_test.go` | bootstrap | 改 | 最后一跳、码的表的调用、工具的字符串的合法参数的表与它的反向对照、接缝工具 `hold`、`strict` 与调用的帮助函数、边缘的次序、矩阵的四行与 `after` 的计数（REST 的 `after` 同样计数）、撤销的性质测试的 MCP 移动加改名 |
| `server/configs/config.yaml` | 配置 | 改 | `server.max_body_bytes` 的注释写上大的请求体的名额（没有新键） |
| `Makefile`、`.github/workflows/ci.yml` | — | 改 | `test-go-costs` 的 page/domain 一行加 `TestReplaceStaysWithinItsTime`；组合根的每一份照 6.5 的规则（过了线的一份把测试给挪过之后仍在线下的邻居，否则再分一份，多半分出第三份）；`test-go` 照 CI 的份跑组合根、竞态检测的几个二进制同时跑；成本与守卫的行照 6.5 的"时限的一半"，`test-go-guards` 的 markdown 一行多半拆成一个测试一行 |
| `e2e/…` | e2e | 新、改 | 第 4 节 |

### 3.2 S1：大的请求体的界线

#### 3.2.1 规则

整个服务同时至多 `LargeBodies` = 2 个**大的请求体**（00 号文档第 11 节主持者的决定第 16 条），**每个凭证至多一个**。一个请求体是大的，当它的路由在 `BodyLimits` 里、而它长过这个路由的**门槛**：

| 路由 | 上限（`BodyLimits`） | 门槛 | 理由 |
|---|---|---|---|
| `/mcp` | 37,814,272（`bootstrap/mcp.go:24-28`） | 65,536（`mcpserver.MaxOtherBytes`） | 超过它的只能是工具的调用（`mcpserver/edge.go:58-105`）；实测上限处一个调用分配约 1.4 GB、峰值约 +0.5 GB（00 号文档第 10 节），放大约 40 倍，64 KiB 起就要算 |
| `PUT /api/v0/pages/{page_id}/content`、`POST /api/v0/notebooks/{notebook_id}/pages` | 31,522,816（`page/adapter/http/limits.go:5-17`） | `server.max_body_bytes`（默认 1,048,576） | 任何一个路由都收 1 MiB 的请求体（`api.go:270-275`），在这之下计数挡不住什么；把请求体读进内存的路由里，大于它的只有这两个与 `/mcp`（上传与导入是 `API.Stream` 的路由，流式地读，`stream.go:84-96`，不在其中） |

所以 REST 的正文路由与 MCP **共用**这两个名额（00 号文档第 10 节写的是"可以共用这份预算"；共用了，峰值才是整个服务的：两个各约 +0.5 GB 时约 1 GB）。网页的保存大于 1 MiB 的页少见；遇到 503 时网页照 `Retry-After` 重试至多 3 次（`web/…/stores/page-editing.ts:13-14`、`:336-342`），用户看不到。

- **取在哪里**：逐路由的中间件，认证与限流之后、结构检查（bodyshape 读请求体）之前（`api.go:190-198` 的 `inOrder` 加一项）。没有令牌、令牌坏了、桶用完的请求从不取名额：取名额的只有认得出的账户（`BodyLimits` 的路由都不公开）。
- **按凭证**：凭证是认证放进 ctx 的限流的键（`pat:<id>`、`session:<id>`，`api.go:277-279`、`:322`，`limit.go:27` 同样读它）；没有凭证的请求（`BodyLimits` 里的公开的路由，今天没有）以它的 `RequestMeta.IPKey` 作凭证，同限流的匿名的桶。一个凭证同时至多一个大的请求体：它的第二个在 `LargeBodyWait` 之内等它的第一个，别的凭证的不等它。一个慢的发送者只占得住一个名额。**代价**（设计审查的复核 A 的 MN-1，主持者接受）：名额在读请求体之前取，一个大的写占着它的是上传加处理。契约说 5 MiB 在 30 秒里要约 1.4 Mbit/s（`api/modules/page.yaml:51-54`）；10 Mbit/s 上一个 5 MiB 的 `write` 约 4.2 秒，1 Mbit/s 上过约 250 KB 的就过 2 秒；一般的页（100 KiB 在 10 Mbit/s 上约 80 ms）不受影响。所以 agent 并行的两个大的写里，第二个在慢的上行上等满 2 秒、答 503，模型看到的是传输的失败（修订之前它会拿空着的第二个名额）；网页在两个标签页里（一个会话）保存两个大的页同样，三次重试多半盖住，除非第一个的上传很长。照旧每个凭证一个、等 2 秒、503：不加长等候、不加按凭证的队列；说明加一行叫 agent 一个接一个地发大的写（3.12），P5 的人工清单加一次限速的上行的试用（第 8 节）。平台只知道凭证、不知道账户，所以一个账户的两个 PAT 仍是两个凭证（剩下的风险，下面）。
- **怎样知道大小**：有 `Content-Length` 时按它：长过路由的上限的立刻答 413 `payload_too_large`（不取名额、不读请求体；今天它要读到上限加一个字节才答，`$GOROOT/src/net/http/request.go:1234-1273`）；长过门槛的取名额；不长的照常往下。没有 `Content-Length`（分块）时先读前缀 `io.ReadAll(io.LimitReader(r.Body, 门槛+1))`（不先分配门槛那么大的缓冲：短的分块请求不该占 1 MiB）：读完了（短）就把读到的接回去、照常往下；还有就取名额，再把读到的接回请求体的前面（`io.MultiReader`），后面的照旧经 `MaxBytesReader`（它已数过读走的字节）。读前缀出错（客户端走了、读超时）照 bodyshape 的答法（`a.Errors.BodyError`，`apierrors.go:155-182`）。
- **取不到**：先取凭证的一个、再取整个服务的一个，两者合起来至多等 `LargeBodyWait` = 2 秒（一个计时器；与 `page.parse_max_wait` 的默认相同）。还取不到答 503 `server_busy`、`Retry-After: 1`，同解析的预算记 info（`platform/markdown/budget.go:86-89`）"large request bodies are saturated"，带路由、这个请求的凭证、`own`（是不是它自己的第一个还持着凭证的名额）、等着的请求数与 `holders`（持着整个服务的名额的每个请求的凭证与路由）：管理员由它找到占着名额的是谁、撤销那个 PAT 或结束会话（凭证的键是 id，13.1 第 10 条许记）。不等就答，并行的大的写只要多过空着的名额就立刻 503；它是 HTTP 的错误，不是工具的错误，模型看到的是传输失败：Go 的 SDK v1.8.0 的客户端把 503 当作暂时的错误，答"Service Unavailable"、不读答复的正文（`sdk/mcp/streamable.go:2583-2587`、`:2821-2833`），`server_busy` 与 `Retry-After` 都到不了模型；Claude Code、Codex 怎样表现由 P5 的人工清单看。等 2 秒，前一个写完（快的链路上几十到几百毫秒）就轮到它；慢的上行见上一条的代价。等着的有 `Content-Length` 的请求还没读请求体，只占一个连接；分块的已读了前缀（至多门槛加一个字节：`/mcp` 64 KiB、REST 1 MiB）。请求的 ctx 先结束时答它的错误（期限是 500，同 `Write`）；HTTP/1.1 上等着的请求看不到客户端走了（请求体没读，net/http 没有后台的读），只在 HTTP/2 上或期限到时提前结束，测试不期望 499（设计审查 A-N2 实测）。
- **还**：`defer` 在 `next.ServeHTTP` 返回之后还（先还整个服务的、再还凭证的），所以每条路都还：
  - 正常的答复与各种错误的答复（bodyshape 的 400、413，mcpserver 的 403、404、413）都经 `next` 正常返回；
  - panic：沿途的 `defer` 在 `withRecover` 写 500 之前执行（Go 的语义；`withRecover` 在平台链的外层，`middleware.go:42-46`、`:111-156`），`http.ErrAbortHandler` 重新 panic 时同样先经它们；
  - 客户端在请求体中途走了：bodyshape 的 `ReadAll` 出错、答 400、返回；
  - 客户端在请求体之后走了：REST 的处理函数看到 ctx 取消、返回；`/mcp` 的 SDK 在答完或 ctx 结束之后 `session.Close()`，它等进行中的方法结束（`sdk/mcp/streamable.go:440`、`sdk/mcp/server.go:2136-2165`、`sdk/internal/jsonrpc2/conn.go:497-509`），mcpserver 的 `outer` 以 `AfterFunc` 取消调用的 ctx（`mcpserver/call.go:55`），所以名额在工具的处理函数结束之后才还：处理函数里的请求体的几份拷贝那时才没人引用；
  - 期限：路由的期限（`request_timeout` 加 `read_timeout`，默认 45 秒，`api.go:246-265`）取消 ctx，同上。
  - 不还的只有一个不看 ctx、永不返回的处理函数，那本身是缺陷；page 的每条语句都带 ctx。
- **慢的发送者与剩下的风险**：取了名额的请求在它读完请求体之前一直占着，请求体只受连接的读期限管（`ReadTimeout` = `server.read_timeout` 30 秒，`server.go:44`），再加处理；名额在任何权限的检查之前取（3.2.3）。所以一个凭证以一个滴着发的请求体（声明 `Content-Length: 65537`、一秒一个字节）占得住一个名额 30 秒，之后再发一个：每 30 秒一个请求，按凭证的桶（每分钟 1,200 个，`server/configs/config.yaml:74`）看不到。每个凭证一个名额之后，占满两个名额要两个凭证（一个账户的两个 PAT 也算），每 30 秒两个请求、每分钟 4 个；`auth.signup_enabled` 默认关着（`server/configs/config.yaml:43`），账户只来自管理员与邀请。其间整个服务里超过门槛的 MCP 调用与超过 1 MiB 的网页保存等 2 秒后 503（网页重试三次，约 11 秒后失败）。这是剩下的风险，写进 00 号文档第 10 节与 M12 的性能移交（2.1 第 6 条、第 8 节）：info 的一行点名占着名额的凭证；缓冲请求体的反向代理先把整个请求体收下再转给应用，慢的发送者到不了名额。P5 的接入文档按路由推荐（设计审查的复核 A 的 MN-2，主持者的决定）：`/mcp` 与 page 的两个正文路由（`PUT /api/v0/pages/{page_id}/content`、`POST /api/v0/notebooks/{notebook_id}/pages`）缓冲（nginx 的 `proxy_request_buffering on`，它的默认）；`API.Stream` 的上传与导入照 README 的部署一节不缓冲（`README.md:357`：否则提前的拒绝也要等整个文件传完）；Caddy（README 与总体设计的默认，`v0.1-design.md:127`）默认不缓冲请求体，直接暴露的服务也不，用它们时这个风险照旧（第 8 节）。`server.read_timeout` 不改（它也是上传与导入的读期限）。

#### 3.2.2 httpserver 的改动

```go
// APIConfig gains:

	// LargeBodies is how many large request bodies the server reads and
	// serves at once (M9 design 4.14, 10; M9/P2 design 3.2): a route of
	// BodyLimits whose body is longer than its threshold takes one before
	// its body is read, and gives it back when it has been served; each
	// credential holds at most one of them. Zero bounds none.
	LargeBodies int
	// LargeBodyAbove is a route's threshold when it is not MaxBodyBytes:
	// /mcp's is the 64 KiB past which only a tool's call may go.
	LargeBodyAbove map[string]int64
	// LargeBodyWait is how long a large body waits for its credential's
	// slot and the server's, both together: then it is 503 server_busy,
	// retry after a second.
	LargeBodyWait time.Duration
```

`NewAPI` 另查：`LargeBodies` 不小于 0；大于 0 时 `LargeBodyWait` 为正；`LargeBodyAbove` 的每个键是 `BodyLimits` 的键、值在（0，那个路由的上限）之间。错误照今天的写法拼起（`api.go:153-155`）。为零时没有名额、中间件直接往下：四处建 `APIConfig` 的里只有组合根设它（`bootstrap/deps.go:270-293`），`httpservertest`、`api_test.go`、mcpserver 的测试套件照旧（第 2.1 节第 18 条：没有配置项）。

```go
// largeBody takes a slot of the large bodies for a request of a route of
// BodyLimits whose body is longer than its threshold, from before its
// body is read to when it has been served (M9/P2 design 3.2.1): a declared
// length past the route's limit is 413 at once, with no slot; one past the
// threshold takes its credential's slot and one of the server's; a body
// of no declared length is read up to the threshold first, and what was
// read goes back in front of the rest.
func (a *API) largeBody(next http.Handler) http.Handler

// takeLarge takes the credential's slot, then one of the server's, for a
// request of route, waiting LargeBodyWait at most for both: serverBusy
// past it, the request's own error if it ends first, as the password
// hasher's acquire waits (identity/adapter/argon2/hasher.go:151-170). The
// func it answers gives both back, the server's first.
func (a *API) takeLarge(ctx context.Context, credential, route string) (func(), error)

// serverBusy is 503 server_busy, the large bodies' slots all taken: the
// platform's own problem, as rateLimited is (limit.go:49-56).
type serverBusy time.Duration

func (serverBusy) Error() string                { return "The server is busy; retry shortly." }
func (serverBusy) ProblemStatus() int           { return http.StatusServiceUnavailable }
func (serverBusy) ProblemCode() string          { return CodeServerBusy }
func (b serverBusy) RetryAfter() time.Duration  { return time.Duration(b) }
```

整个服务的名额是 `chan struct{}`（容量 `LargeBodies`）；每个凭证的是一个容量为 1 的 `chan struct{}`，放在一个由互斥锁守着的表里，按凭证带引用的计数，持着与等着的都没了就从表里去掉（表的大小至多是同时在等或在持的请求数）。两个都先不阻塞地送，送不进再与计时器、ctx 一起 `select`（同 argon2 的 `acquire`），计时器是一个，两个的等候合计 `LargeBodyWait`；持着整个服务的名额的请求另记（凭证、路由）在一个小的表里，只为 info 的一行。不用 `x/sync/semaphore`：名额不按字节分。`problem.go` 的平台码加 `CodeServerBusy = "server_busy"`（注释：大的请求体的名额，M9/P2）。httpserver 不导入 `shared`（`archtest/rules_test.go:40`），所以是自己的 `ProblemError`；文字与 `shared.ServerBusy` 的相同（`shared/error.go:188-192`），两者都经 `Write` 写出 `Retry-After`（`apierrors.go:225-227`）。

组合根：`apiConfig`（`bootstrap/deps.go:270-293`）设 `LargeBodies: largeBodies`（2）与 `LargeBodyWait: largeBodyWait`（2 秒），`bootstrap/wire.go` 并 `bodies` 的那一处（`:170-172`）设 `LargeBodyAbove: {mcpserver.Pattern: mcpserver.MaxOtherBytes}`；两个常量在 `bootstrap/mcp.go`，挨着 `mcpBodyLimit`，注释写 00 号文档第 10 节的量与第 11 节第 16 条。`wiring` 加 `largeBodyWait time.Duration`（零是常量；整个程序的大请求体的测试设 250 ms，3.2.5），`TestTheAPIConfigBoundsTheLargeBodies` 以 `apiConfig` 的返回值核对 2 与 2 秒（纯函数，不起应用）。

#### 3.2.3 契约与边缘的次序

- **REST**：`createPage` 与 `putPageContent` 已声明 `server_busy`（`api/modules/page.yaml:56`、`:156`），`Retry-After` 是每个模块的 `Problem` 答复的头（`page.yaml:1122-1131`），`apitest` 的 `checkProblemCode` 收操作自己声明的码（`platform/httpserver/apitest/problems.go:76-104`）：码不改。两个操作的说明写的次序是"先查正文，不论笔记本或页"、`server_busy` 只因为解析的预算（`page.yaml:39-55`、`:121-145`），S1 起不对了，所以各加一句，放在"先查正文"那一句之前（S1 的实施，`make gen` 随之；句子照设计审查的复核 A 的 NN-6，不让它读成每个这样的请求体都是 `server_busy`）："Before any check, a body over server.max_body_bytes waits up to 2 seconds for one of the server's two large-body slots, at most one per credential, when they are taken, and is server_busy if none frees." 不加进顶层的 `x-problem-codes`：只有这两个操作会答它，都已声明。`Problem` 的 `code` 的说明已列着 `server_busy`（`api/common.yaml:47-54`）。v0.1 总体设计 13.1 第 4 条的这两个操作的次序随 P2 的合并改（第 7 节）。
- **MCP**：`/mcp` 不在 OpenAPI 的契约里（`bootstrap/contract_test.go:24-49`），503 是 HTTP 一层的拒绝、不是工具的错误（00 号文档 4.7），不进 `mcpToolCodes()`。边缘的次序（00 号文档 4.15，`TestTheMCPEdgeAnswersInOrder`，`bootstrap/mcp_test.go:29-136`）改为：401 → 429 → 401 → 429 → **413（声明的长度超过上限）→ 503（没有名额）** → 413、400（bodyshape）→ 413（超过 64 KiB 而不是这个入口的工具的调用）→ 403（不是 PAT）→ 400、405 → 404（2.1 第 6 条）。
- 答复照平台的出口：problem+json、`no-store`（`/mcp` 与 `/api/` 都由 `setFixedHeaders` 设，`middleware.go:56-72`）。

#### 3.2.4 内存与测量

两个名额时，大的请求体同时至多两个：上限处的 MCP 调用各约 +0.5 GB 的峰值，合计约 1 GB（00 号文档第 10 节）。这是估计：P1 的 S1 审查 A 量的是一个。实施者照它的做法（同一进程里 `runtime.ReadMemStats` 的峰值、`GOGC` 默认，一次性的测试、不入库）量界线本身：两个同时在上限处的请求体、第三个在等，两种组合各一次：两个 `/mcp` 的 37,814,272 字节的 `write`（5 MiB 的正文、全是最贵的转义）；一个 `/mcp` 加一个 REST 的 `PUT …/content` 的 31,522,816 字节。峰值写进第 9 节；超过 1 GB 的，第 10 节的那一行随之改写。一个 PAT 的桶仍允许 200 个同时的请求（小的请求体不受这个界线管），压测交 M12。

#### 3.2.5 测试

httpserver（`largebody_test.go`，`testAPIConfig` 加一个 `BodyLimits` 的路由 `POST /big`、门槛 1 KiB、等 50 ms；处理函数从一个通道读到信号才答，借它持住名额；测试的认证给每个令牌一个凭证 `session:<令牌>`，`api_test.go:55`，所以两个令牌是两个凭证）。名额默认 2；`TestTheSlotComesBackOnEveryPath` 的应用只有 1 个名额：一个漏掉的名额让下一个大的请求等满、答 503，看得见（设计审查 A-M8）。

| 测试 | 断言 |
|---|---|
| `TestTwoLargeBodiesAreServedAtOnce` | 两个凭证的 2 KiB 的请求持着两个名额；第三个凭证的在 50 ms 之后答 503 `server_busy`、`Retry-After: 1`、problem+json；其间一个 1 KiB 的（恰在门槛）照常答；放开一个之后第三个凭证的下一个拿到名额 |
| `TestACredentialsSecondLargeBodyWaitsForItsFirst` | 凭证 A 的一个持着名额、整个服务还空着一个：A 的第二个大的请求在 50 ms 之后答 503（`own`），没有拿那个空着的名额；其间凭证 B 的大的请求拿到空着的那个、照常答（不是 503）；A 的第二个再发一次，测试见它在等（包内的测试读凭证的表：A 的计数是 2）就放开 A 的第一个：A 的第二个拿到它、照常答（不靠睡 20 ms，同第二次复核 A 的 NN3-2） |
| `TestTheSaturatedLineNamesTheHolders` | 满了时答 503 的那一次记一行 info：消息、路由、这个请求的凭证、`own`、等着的数目与两个持有者的凭证与路由；不记请求体 |
| `TestABodyOfNoLengthTakesASlotPastItsThreshold` | 分块的请求体（客户端的 `io.MultiReader`，没有 `Content-Length`）2 KiB 占名额、1 KiB 不占；处理函数读到的请求体与发出的逐字节相同（前缀接回去了），一个 bodyshape 的路由同样 |
| `TestADeclaredLengthPastTheLimitIs413WithoutASlot` | 名额满时，`Content-Length` 超过路由上限的请求答 413（不是 503）、处理函数没被调用、名额数不变；`Content-Length` 等于上限的照常排队 |
| `TestTheSlotComesBackOnEveryPath` | 一个名额的应用：处理函数正常返回、返回错误的答复、panic（经平台的 `middleware` 链，答 500）、客户端在请求体中途关掉（声明的 `Content-Length` 过门槛，发了一半就 `CloseWithError`，答 400：取名额在读请求体之前）、请求的期限到了（处理函数等 ctx）：每一种在进行中时，另一个凭证的探针请求等满、答 503（证明这一种持着名额）；结束之后，另一个凭证的新请求拿到名额、照常答，同一个凭证的新请求也是（凭证的名额也还了）：漏了的名额让它等满、答 503，所以只看答复、不计时（`-j4` 之下竞态检测的 httpserver 与组合根的几份同时跑，几十毫秒的上限会偶尔失败；名额在 `ServeHTTP` 返回之后才还，客户端先读到答复也是正常的；设计审查的第二次复核 A 的 NN3-2） |
| `TestALargeBodyWaitsForItsTurn` | 两个别的凭证的大的请求持着两个名额；第三个凭证的请求在等时（包内的测试读凭证的表里有它）放开其中一个：等着的请求拿到它、照常答，不是 503（不靠睡 20 ms，同第二次复核 A 的 NN3-2；只持着一个名额时，等着的不等就拿到空着的那个，变异 45 就活下来；设计审查的复核 A 的 NN-3） |
| `TestTheSlotsAreTakenAfterTheBuckets` | 名额满时，没有令牌的大请求体答 401、坏的令牌 401、桶用完的 429，不是 503（先取名额的话它们会等满、答 503：只看答复）；名额只被认得出的账户取（`Wrap` 的次序，照 `TestWrapRunsTheMiddlewaresInTheirOrder`，`api_test.go:489-528`） |
| `TestARouteOutsideBodyLimitsTakesNoSlot` | 名额满时，一个普通路由的 1 MiB 请求体照常答 |
| `TestNewAPIChecksTheLargeBodies` | 负的名额、有名额而没有等候、门槛的路由不在 `BodyLimits` 里、门槛不在（0，上限）之间：各一行，错误的文字照 `TestNewAPIRequiresItsSettings` 的写法 |

整个程序（`bootstrap/mcp_bodies_test.go` 的 `TestTheLargeBodiesShareTwoSlots`，接缝工具 `hold` 经 `wiring.mcpTools` 登记：收一个字符串 `s`，进来时告诉测试（测试由此知道它持着名额），等测试的通道或它的 ctx；`wiring.largeBodyWait` = 250 ms，2 秒本身由 `TestTheAPIConfigBoundsTheLargeBodies` 钉住；alice 的三个 PAT P1、P2、P3 是三个凭证）：

1. P1、P2 的 `hold`，`s` 各 70 KiB：持着两个名额。
2. P3 的 70 KiB 的 `hold`：等候之后 HTTP 503，problem+json `server_busy`、`Retry-After: 1`、`Cache-Control: no-store`；info 的一行点名 P1、P2 的凭证与 `/mcp`。
3. P3 的 REST 的 `PUT …/content`，正文 1.1 MiB：503 `server_busy`（契约核对过）；正文 100 KiB：200（在门槛之下）。
4. P3 的 `echo`，`s` 60 KiB：答结果（在门槛之下）。
5. 放开 P1 的：P2 的第二个 70 KiB 的调用等候之后 503（`own`：整个服务空着一个，它不拿），P3 的 70 KiB 的 `hold` 拿到空着的那个：进了工具，放开之后答结果，不是 503（只看答复，不计时，设计审查的第二次复核 A 的 NN3-2）。
6. P2 的持着的 `hold` 的客户端取消它的请求（P3 的仍持着另一个），名额回来：P1 的一个 70 KiB 的调用答结果，不是 503（它要的正是回来的那个；名额在 P2 的工具的处理函数结束之后才还，P1 的调用在它 250 ms 的等候里拿到）。

边缘的次序（`TestTheMCPEdgeAnswersInOrder`）加两行：名额满时声明的长度超过 `mcpBodyBytes` 答 413；名额满时一个 70 KiB 而不合 JSON 的请求体答 503（不是 bodyshape 的 400）。

### 3.3 S1：客户端与 `mcp:<名>`

#### 3.3.1 `shared.Unshowable`、`Client.Valid` 与 `domain.MCPClient`

- `shared/name.go`：`unshowable` 改名为导出的 `Unshowable`，注释写"Cc、Zl、Zp 与 Bidi_Control；page 的两张表的客户端检查写出同一个集合（00044）"；`CheckName` 与 `CleanClientName` 改用它；`shared/export_test.go`（S3 为包外的测试导出它的唯一一行）删去，`client_name_test.go` 的 `shared.Unshowable` 照旧编译。
- `page/domain/client.go`：

```go
// mcpPrefix starts an agent's client; UnknownClientName is the name of one
// that told none (mcpserver.UnknownClient, which page's adapter/mcp holds
// it to).
const (
	mcpPrefix         = "mcp:"
	UnknownClientName = "unknown"
)

// Valid reports whether the changesets and edit_sessions tables take c
// (M9 design 4.2): one of the three, or mcp: and a name of 1 to
// shared.MaxClientNameChars characters of UTF-8, none of them unshowable
// (shared.Unshowable, the set the tables' checks spell out).
func (c Client) Valid() bool {
	switch c {
	case ClientWeb, ClientAPI, ClientCLI:
		return true
	}
	name, ok := strings.CutPrefix(string(c), mcpPrefix)
	n := utf8.RuneCountInString(name)
	return ok && utf8.ValidString(name) && n >= 1 && n <= shared.MaxClientNameChars && !strings.ContainsFunc(name, shared.Unshowable)
}

// IsMCP reports whether c is an agent's: mcp: and its name.
func (c Client) IsMCP() bool { return strings.HasPrefix(string(c), mcpPrefix) }

// MCPClient is the client of the agent named name (M9 design 4.5): mcp:
// and the name cleaned again, unknown when nothing is left. A name a token
// recorded, cleaned before, stays itself: CleanClientName is idempotent.
func MCPClient(name string) Client {
	if name = shared.CleanClientName(name); name == "" {
		name = UnknownClientName
	}
	return Client(mcpPrefix + name)
}
```

- 测试（`page/domain/client_test.go`）：`MCPClient` 的表（`Claude Code`、前后的空白、`""` 与 `unknown` 都是 `mcp:unknown`、U+202E 起头、200 个 `n` 截成 128 个、不是 UTF-8 的字节）；性质：S3 的 10 万个随机串（`shared/client_name_test.go:54-81` 的生成器）经 `MCPClient` 都 `Valid()`，所以单元不会因为 MCP 的客户端答 500（今天的 `unit.go:172-174`）。`ParseClientFilter` 的表（`page/domain/changeset_test.go:42-72`）加 U+2028、U+202E、U+061C、U+200E 被拒，U+200B、U+00AD、U+FEFF 被收。`page/adapter/mcp` 的测试核对 `domain.UnknownClientName == mcpserver.UnknownClient`（page 的 domain 不能导入 mcpserver）。`MCPClient` 的再清理只由这个性质测试守着：到达适配器的 `Call.Client` 已清理过（`mcpserver/names.go:51`、`tool.go:72`），整个程序看不出它省没省（6.3 第 10 条）。

#### 3.3.2 迁移 `00044_page_client_checks.sql`

迁移归 page（两张表都是它的，`archtest/sqlc_test.go:189-194`），编号接 S3 的 00043；`server/sqlc.yaml` 的 page 一条加上它（`TestSQLCSchemaScope`）。检查只改写法，不加表、列、序列，`deploy/runtime-grants.sql` 不动。

```sql
-- The clients the changesets and the edit sessions take, written without the locale (M9 design 4.2; the M4/P1
-- hand-off's items 1-2): web, api, cli, or mcp: and 1 to 128 characters none of which is a control character (Cc:
-- U+0000-U+001F, U+007F-U+009F; text holds no NUL), a line or paragraph separator (Zl, Zp: U+2028, U+2029) or a
-- bidirectional control (Bidi_Control: U+061C, U+200E, U+200F, U+202A-U+202E, U+2066-U+2069): shared.Unshowable,
-- which page's Client.Valid checks. [:cntrl:] followed the database's LC_CTYPE. The ranges are the regular
-- expression's own escapes; U+2028-U+202E is Zl, Zp and the five embeddings and overrides at once.
-- Each table is scanned once under the ALTER's lock: a changeset is a unit's write, an edit session a page open.

-- +goose Up
ALTER TABLE changesets DROP CONSTRAINT changesets_client_check,
    ADD CONSTRAINT changesets_client_check CHECK (client IN ('web', 'api', 'cli')
        OR client ~ '^mcp:[^\u0001-\u001f\u007f-\u009f\u061c\u200e\u200f\u2028-\u202e\u2066-\u2069]{1,128}$');
ALTER TABLE edit_sessions DROP CONSTRAINT edit_sessions_client_check,
    ADD CONSTRAINT edit_sessions_client_check CHECK (client IN ('web', 'api', 'cli')
        OR client ~ '^mcp:[^\u0001-\u001f\u007f-\u009f\u061c\u200e\u200f\u2028-\u202e\u2066-\u2069]{1,128}$');

-- +goose Down
ALTER TABLE edit_sessions DROP CONSTRAINT edit_sessions_client_check,
    ADD CONSTRAINT edit_sessions_client_check CHECK (client IN ('web', 'api', 'cli') OR client ~ '^mcp:[^[:cntrl:]]{1,128}$');
ALTER TABLE changesets DROP CONSTRAINT changesets_client_check,
    ADD CONSTRAINT changesets_client_check CHECK (client IN ('web', 'api', 'cli') OR client ~ '^mcp:[^[:cntrl:]]{1,128}$');
```

- `standard_conforming_strings` 默认开着，字面量里的反斜杠原样交给正则；PostgreSQL 的 ARE 在方括号里收 `\uXXXX`，`{1,128}` 数的是字符（库是 UTF8）。设计审查 A 在 postgres:18.6 上、五种区域设置（builtin C.UTF-8、libc C、C.UTF-8、en_US.utf8、icu）里照本节的 SQL 跑过：新的检查恰好拒绝 78 个码位（NUL 之外的 64 个 Cc 与 Zl、Zp、Bidi_Control 的 14 个），等于 Go 的集合；今天的 `[:cntrl:]` 在五种里都拒 64 个、收那 14 个（`review-A/pgcheck-results.txt`）。
- 迁移在 `ALTER` 的锁下扫一遍两张表。`changesets` 一个单元一行，`edit_sessions` 一个打开的编辑器一行（过期的由任务删）；P2 之前没有 `mcp:` 的行，`IN` 那一支对每行都成立（A 实测 20 万行 9–16 ms；Down 照样成立：新的检查拒绝旧的拒绝的一切）。实施者在一个 100 万行的 `changesets` 上量一次迁移的时间，写进第 9 节。
- `schema_test`：
  - `TestChecksRejectCounterexamples`（`server/migrations/schema_test.go:436`）对两张表各加**一条语句**核对每个码位（取代草稿的拼写测试，设计审查 A 的过度设计）：以 `pg_get_expr(conbin, conrelid)` 读出这个检查在库里的表达式，套在 `SELECT n FROM (SELECT n, 'mcp:a' || chr(n) AS client FROM generate_series(1, 1114111) n WHERE n < 55296 OR n > 57343) s WHERE NOT (<表达式>)` 上：拒绝的码位恰是 `shared.Unshowable` 为真的那些（NUL 除外），由 PostgreSQL 照它自己的读法算出（约 1 秒一张表），Go 升级 Unicode 的版本而集合变了时它失败、提醒再写一个迁移（13.4 第 13 条：两处实现同一条规则）。另照 `:604-607` 的写法留几个可读的反例（U+0085、U+009F、U+007F、U+2028，M4/P1 的移交写的四个）；今天 `edit_sessions` 只有 `'mobile'` 一个反例（`:625`），补上 `mcp:`、换行与 129 个字符。
  - 新的 `TestTheClientChecksTakeWhatValidTakes`：两张表各以 `chr(n)` 插入应当收下的：32、160、173、8203、8204、8205、8231、8239、8293、8298、65279 与 128 个 `名`，都成功（`Client.Valid` 只是在 `shared.Unshowable` 之上加前缀与长度，由 `page/domain` 的测试核对；迁移的测试不导入模块的内部，13.1 第 11 条）。
  - `TestConstraintAndIndexNames` 的名字不变（`:233`、`:246`）。

#### 3.3.3 最后一跳

page 的 `adapter/mcp` 的四个写的处理函数都以 `domain.MCPClient(c.Client)` 交客户端（3.11）。测试见 3.13.3：`mcp:<名>` 进 `changesets.client`，两种来源与重启之后记下的名称。

### 3.4 S1：版本令牌的解析

```go
// maxVersionTokenBytes is the longest token String writes: an id's 36
// characters, ":" and MaxRevision's ten digits.
const maxVersionTokenBytes = 36 + 1 + 10

// ParseVersionToken is the version token s as String writes it (M9 design
// 4.10): a node's id in its canonical form, ":", and a revision of 1 to
// MaxRevision in decimal, without a sign or a leading zero. Any other
// spelling is 422 on field, invalid_format, the value never repeated: an
// agent's argument may be long.
func ParseVersionToken(field, s string) (VersionToken, error) {
	if len(s) <= maxVersionTokenBytes {
		if id, rev, ok := strings.Cut(s, ":"); ok {
			node, err := uuid.Parse(id)
			r, rerr := strconv.Atoi(rev)
			if t := (VersionToken{Node: node, Revision: r}); err == nil && rerr == nil && r >= 1 && r <= MaxRevision && t.String() == s {
				return t, nil
			}
		}
	}
	return VersionToken{}, NotAVersion(field)
}

// NotAVersion is 422 on field: not a version token as read, list and write
// give them.
func NotAVersion(field string) error {
	return shared.Invalid(shared.FieldError{Field: field, Code: shared.FieldInvalidFormat,
		Message: `is not a version, "<node id>:<revision>" as read, list and write give it`})
}
```

- 往返：`t.String() == s` 挡住 `uuid.Parse` 收的别的写法（大写、花括号、`urn:uuid:`、32 个十六进制字符，`$GOROOT/src/uuid/uuid.go:35-45`、`:134-140`）与 `strconv.Atoi` 收的 `+1`、`01`。零值的 id 写得出、解析得了：它从不是一个节点，比较时是"别的节点"，答 409（3.8.1）。
- 测试（`page/domain/path_test.go` 旁的 `TestParseVersionToken`）：随机的令牌往返；被拒的各一行：空、`:`、`x:1`、只有 id、`id:`、`id:0`、`id:-1`、`id:+1`、`id:01`、大写的 id、`{id}:1`、`urn:uuid:id:1`、`<32 个十六进制字符>:1`、`id:2147483648`、`id: 1`、`id:1 `、48 字节、带 NUL；`MaxRevision` 恰好收下。

### 3.5 S1：单元的操作

#### 3.5.1 `Unit.Relocate` 与 `RelocateNode`

```go
// Placement is where Relocate puts a node: under ParentID (nil: the
// notebook's root), named Name; Field is the field a 422 on the name
// names (move's "to"; P3's ops[i].to).
type Placement struct {
	ParentID *uuid.UUID
	Name     string
	Field    string
}

// Relocate moves the node id under p's parent and names it p's name, in
// one step and one statement (M9 design 4.10). Under another parent it is
// an OpMove: the node goes last among its new siblings, its descendants
// with it, and only its change is an item. Under its parent it is an
// OpRename and keeps its place. 404 for a node not in the notebook; 422
// for a name against its kind's rules (p.Field) or a parent that is no
// page of the notebook (parent_id); then 409 for a parent in the node's
// own subtree, a name a sibling holds there (*domain.PathTaken's holder
// named from the siblings it read), or a subtree deeper than
// domain.MaxDepth there. A node there and so named already writes
// nothing; a name that differs in case alone is written, as a rename's
// is.
func (u *Unit) Relocate(ctx context.Context, id uuid.UUID, p Placement) (domain.Node, error)
```

（名字取 `Placement`，不叫 `Relocation`：linking 有 `Relocation(changes)`，本节与 3.5.3 都要说它，设计审查 B-N7。）

次序：

1. `FindNodeIn` → 404（`found(err, domain.ErrNotFound)`）。
2. `nameOf(n, p.Name, p.Field)` → 422：页照 `CheckTitle`，附件照 `CheckAssetRename`（旧的名称有扩展名时新的也要有一个，扩展名可以换，`page/domain/name.go:29-41`，样例 `035-attachment-new-extension`）。`nameOf` 多收一个字段名，`rename` 交 `"name"`（今天写死的）。
3. 同一个父节点（`domain.SameParent(n.ParentID, p.ParentID)`）：名称没变 → 返回 `n`，什么也不写；键变了 → `Children(n.ParentID)` 与 `titleFree(siblings, title, n.ID)` → 409；键没变（只改大小写）不查兄弟。改名之后的节点只换 `Name`、`NameKey`、`UpdatedBy`、`UpdatedAt`；一步 `OpRename`，`apply(…, true, …)` 写 `RelocateNode`（父节点与次序值照旧）。
4. 别的父节点：`lineOf(p.ParentID)` → 422 `parent_id`；`Children(p.ParentID)`；`Subtree(n.ID)`；`p.ParentID` 在子树里 → `ErrCycle`；`titleFree(siblings, title, n.ID)`（新的名称，在新的兄弟里）→ `ErrTitleTaken`；`Depth(line)+sub.Height()-1 > MaxDepth` → `ErrTooDeep`；`placeAmong(siblings, len(siblings)-1)`（最后）；`moved` 换父节点、次序值、名称、键与更新者、时刻；`moveSubtree(ctx, sub, moved, renumber, NodeWriter.RelocateNode)`：一步 `OpMove`，节点的改动同时带新的父节点与名称，子孙的"前""后"相同（不记条目），写先重排次序（如果要）再一条 `RelocateNode`。
5. 409 的次序同 `Move`（环、名称、层级，`unit_move.go:49-60`）。撞名时 `titleFree` 答出占着的兄弟（它就在读过的 `siblings` 里），`Relocate` 以它的类型写 `domain.PathTaken{Asset}`，不另查（`RelocateNode` 撞上唯一索引是缺陷：那时事务已中止，再查也不成，设计审查 B-N4）。

`moveSubtree` 多收写入的语句：`Move` 照旧交 `MoveNode`，`Relocate` 与撤销的一步的放回（3.5.4）交 `RelocateNode`。

```sql
-- name: RelocateNode :exec
-- A node's parent, order and name at once (M9 design 4.10): one statement, the siblings' partial unique indexes
-- (00042) checking it under its last parent with its last key.
UPDATE nodes
SET parent_id = sqlc.narg(parent_id), sort_order = sqlc.arg(sort_order), name = sqlc.arg(name),
    name_key = sqlc.arg(name_key), updated_by_id = sqlc.arg(by), updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);
```

`NodeWriter` 加 `RelocateNode(ctx, n domain.Node) error`；存储同 `RenameNode`、`MoveNode` 以 `nameTaken` 译成 `domain.ErrTitleTaken`（`page/adapter/postgres/store.go:330-357`）。按主键改一行，没有计划的问题。存储的测试：改父节点与名称一次成功；撞根下与子节点的两个索引各答 `ErrTitleTaken`（照 `TestEverySiblingsIndexTellsANameTaken`）；只改大小写不撞自己。

单元的测试（`page/app/unit_relocate_test.go`，替身的存储）：同一父节点的改名、只改大小写、名称不变（不写、没有步）、移动加改名一步（守卫与参与者各看到一步，`OpMove`，改动带新的父节点与名称；`NodeWriter` 只收到一次 `RelocateNode`）、子孙随它而只有它记条目、排在新兄弟的最后、只改名时次序不变、附件改名换扩展名（成）与去掉扩展名（422 落在 `p.Field`）、环、新兄弟里的名称（409，占着的是页与是附件各一行，替身的 `RootByKey`/`ChildByKey` 没被调用）、新兄弟里只改大小写的同键（409：别的节点）、层级（附件不算一层）。

#### 3.5.2 `Unit.Subtree` 与 `Unit.DeleteSubtree`

```go
// Subtree is the node id and every node under it, in the unit's notebook
// and transaction, the root first and then level by level: page.not_found
// for a node that is not in it.
func (u *Unit) Subtree(ctx context.Context, id uuid.UUID) (domain.Subtree, error)

// DeleteSubtree deletes sub, a subtree Subtree read in this unit, into the
// trash: one step of OpDelete, an item a node, its edit sessions' rows
// deleted, their subscribers told of those alive (M8/P2 design 3.3).
func (u *Unit) DeleteSubtree(ctx context.Context, sub domain.Subtree) error

// Delete is Subtree, then DeleteSubtree.
func (u *Unit) Delete(ctx context.Context, id uuid.UUID) (domain.Subtree, error)
```

`unit_delete.go:20-45` 的读与写分开，行为不变；`Delete` 的三个调用者照旧。`DeleteSubtree` 的子树要是这个单元在锁下读的（笔记本行 `FOR NO KEY UPDATE`，树不会变）；交一个别处读的是调用者的缺陷，`DeleteNodes` 的 `stray` 与 `checkPages` 会抓到其中一种（`page/adapter/postgres/store.go:357-369`）。

#### 3.5.3 linking 的 `Relocation`

`recased` 的条件去掉 `sameParent`（`linking/domain/change.go:73-77`）：一步里节点换了父节点、名称只差大小写，也是大小写的改名，参与者照它改写链接里那一段的大小写。注释改为"a rename or a move changed the case of its title alone"。

- **与 REST 的两步不同的一处**（设计审查 B-M1）：名称在笔记本里唯一、带路径的 wikilink 以旧的末段作显示的文字时（`[[A/x|x]]`），一步的移动加改名得 `[[X|X]]`，REST 的两步（移动、再改大小写）得 `[[X|x]]`（另有同名的页时两者都写带路径的 `[[b/X|X]]`，设计审查的复核 B 的 BC-M2）；改成别的键也一样（`[[A/x|x]]` 一步得 `[[y|y]]`、两步得 `[[y|x]]`）。`relink` 只对带 `/` 的目标让显示的文字跟着改（`linking/domain/rewrite.go:387`），REST 的第一步已把目标改成不带 `/` 的名称，第二步就不再跟。一步只用一次 M6 的规则，结果更好；别的写法两者相同（设计审查 B 在 `review-B/exp-linking` 上照本节的改法跑过 46 个改名的样例与 linking 的两层测试，都通过）。00 号文档 4.10 的"与 REST 的两步结果相同"随之写细（2.1 第 21 条）。
- `TestRelocationIsWhatARewriteFollows`（`linking/domain/change_test.go:90-122`）加一行："a move that changes the case alone"，`relocates` 真、`recased` 是它；同一个名称的纯移动 `recased` 仍为假（`change.go:74` 要名称不同）。
- 改写的样例加 `tools/md-fixtures/rename/047-move-and-recase`：页是 `a`、`a/x`、`b` 与来源页 `src`（`x` 这个名称在笔记本里唯一），`from` `a/x`、`to` `b/X`；来源页是 `[[a/x]] [[x]] [[a/x|alias]] [[a/x|x]] ![[a/x]] [t](a/x.md)`，`.out.md` 是一步在这棵树上的结果 `[[X]] [[X]] [[X|alias]] [[X|X]] ![[X]] [t](X.md)`：`x` 唯一，所以 M6 写名称的形式（同样例 009 的 `[[A/x]]` → `[[x]]`）；再有一个 `c/x` 时一步写 `[[b/X]] [[b/X]] [[b/X|alias]] [[b/X|X]] ![[b/X]] [t](b/X.md)`。草稿手写的是两棵树混起来的，哪一棵都不这样写（设计审查的复核 B 的 BC-M2，在两棵树上以 linking 的改名的 harness 跑过，`review-B/check/tests/linking_domain_zz_check_b_test.go`）。`.out.md` 不由 REST 的两步得出；变异 24 在这棵树上让 `[[x]]` 留作 `[[x]]`，样例失败。
- 样例的 harness 的 `recased` 改为取自 `domain.Relocation` 对这个样例的改动（今天 `rename_cases_test.go:88-91` 按样例的路径自己算、不调生产的代码，所以 047 抓不住"`recased` 仍要同一父节点"的变异，只去掉父节点的比较又会让 harness 对同名的纯移动也设上它）：样例于是经生产的规则。
- 别的调用者：`Relocation` 只在 `Rewrite.Participate`（`linking/app/rewrite.go:48`）。REST 没有一步同时换父节点与名称的操作（改名与移动是两个单元，撤销的单元交 `Options{}`、参与者在 `:49` 返回），M6 的单元测试与 `TestTheIndexIsItsRebuild` 照常通过即是证明。整个程序里与 REST 的两步比较的（3.13.6 的跨父节点只改大小写）不用 `[[A/x|x]]` 这种写法。

#### 3.5.4 撤销的一步的放回

M8 的撤销假定一个变更集只有一种操作（`page/domain/revert.go:89-95`）：一个放回在计划里先模拟"带着现在的名称移到 S\* 的父节点下"、再改名（`planner.place`，`:865-891`，`:874` 查现在的键），单元同样写两步（`page/app/unit_revert.go:124-150`）。P2 的单个 `move` 是第一个写出"新的父节点与新的名称同在一个条目里"的操作：`Drafts` 下有 `Plan` 与 `Plan v2`，agent 调 `move {from: "Drafts/Plan v2.md", to: "Final/Plan.md"}`，之后有人在网页上撤销这个变更集 C：S\*（X 回到 `Drafts`、叫 `Plan v2`）合法，但计划的第一步把 X 带着 `Plan` 移回 `Drafts`，撞上那里的 `Plan`，`ErrRevertUnordered` 原样传出、网页答 500（`unit_revert.go:171-182`、`revert_changeset.go:240-243`），后台的撤销用同一个计划，同样失败（设计审查 B-I1，`review-B/exp-revert` 在纯的计划上复现）。第 11 节第 1 条说撤销是人在网页上做的，所以这个修正随 P2，不等 P3（00 号文档 4.12 第 2 条里单个节点的部分，2.1 第 20 条）：

- **计划**（`planner.place`）：放回既移动又改名时（`op.Move && op.Rename`）只核对最后的键：`m.holder(to.ParentID, shared.TitleKey(to.Name))` 被别的节点占着、`to.ParentID` 不活着、或在节点自己的子树里，是 `ErrRevertUnordered`（S\* 合法时不会发生：此刻占着那个键的别的节点是 L 与 S\* 的核对里的 `title_taken`，答 `page.revert_conflict`，在这之前）；然后一次设上父节点与键。只移动或只改名的照旧。
- **单元**（`Unit.place`）：`op.Move && op.Rename` 时读子树，节点换上 S\* 的父节点、次序值、名称与键、更新者与时刻，`moveSubtree(ctx, sub, n, noop, NodeWriter.RelocateNode)`：一步 `OpMove`，改动同时带父节点与名称（同 `Relocate` 的一步，3.5.1），撞名照旧经 `taken` 成为不分类的缺陷。只移动的照旧 `MoveNode`、只改名的照旧 `RenameNode`。守卫看到一步：M10、M11 的守卫要认得撤销里 `OpMove` 一步的改名（`Write.Kind` 是 `revert`）；00 号文档"写给后面的 M"的来源区的守卫一条今天只说 4.10 的 `Unit.Relocate`，加"与撤销的一步的放回"（2.1 第 20 条，设计审查的复核 B 的 BC-N2）。撤销不经链接的参与者（`Options{}`），所以链接照 M8 留在新的地方（网页的撤销提示它）。
- **P3 留着的**：多轮的调度、三条后备、临时名称、写回的 `Base`、撤销的撤销与后台的切批（00 号文档 4.12），它们的放回都经这同一步。
- **测试**：
  - `page/domain/revert_test.go` 的 `TestARevertOfAMoveThatRenamesIsOnePlace`（B 的复现改成单元测试）：`Drafts` 下另有一个与 X 现在的名称同名的 `Plan` 时，计划是一个放回、`Move` 与 `Rename` 都真、没有错误（今天是 `ErrRevertUnordered`）；没有它时同样；`Final` 下另有一个叫 `Plan v2` 的（先改名的中间态会撞上它）时同样；此刻 `Drafts` 下另有人新建了 `Plan v2` 时是冲突 `title_taken`，不是 `ErrRevertUnordered`。
  - `page/app/revert_test.go`（M8 的撤销的单元测试在这里，`TestRevertChangesetPutsEachChangeBack` 一类，`:199`；没有 `unit_revert_test.go`，设计审查的复核 B 的 BC-N3）：这样的放回是一步（替身的守卫看到一个 `OpMove`，改动的"前""后"各带父节点与名称），`NodeWriter` 只收到一次 `RelocateNode`；只移动的、只改名的照旧各一次 `MoveNode`、`RenameNode`；`TestRevertChangesetsDefectsAreUnclassified`（`:586`）照 `RenameNode`、`MoveNode` 的两行加一行 `RelocateNode` 答 `ErrTitleTaken`：撞名照旧是不分类的缺陷。`ErrRevertUnordered` 的注释（`page/domain/revert.go:89-95`，"Every changeset of M8 is one kind of operation"）改为说一个既移动又改名的条目计划成一个放回。
  - 整个程序（`bootstrap/mcp_revert_test.go` 的 `TestARevertPutsBackAnMCPMoveThatRenames`）：两本笔记本，每本里 `Drafts` 下有 `Plan` 与 `Plan v2`（X）、`Final` 下有一页 `Plan v2`；alice 经 MCP `move` `Drafts/Plan v2.md` → `Final/Plan.md`；第一本经 REST 同步地撤销（`changesetRevert`，`bootstrap/history_test.go:138`），第二本经后台的撤销（`POST /api/v0/notebooks/{id}/reverts`：`revertStart`，`bootstrap/background_test.go:35-40`，经 `tm.background` 等到任务结束）：都成，X 回到 `Drafts`、叫 `Plan v2`、次序值是原来的，`Drafts/Plan` 与 `Final/Plan v2` 不变，撤销的变更集一个条目；`checkPages`、`checkLinks`，后台的另 `checkBackground`。
  - 撤销的性质测试（`bootstrap/revert_property_test.go:76-170`，操作在 `:94-162`）的操作加一种"经 MCP 的移动加改名"：alice 的 PAT 调 `move`，`from` 是一页的路径（不取附件：附件的 `to` 要以它的扩展名结尾），`to` 是一个随机的父页的路径加另一个标题（与 REST 的改名同取 `revertTitles()`）与 `.md`；`change` 只把 HTTP 409 当作跳过（`:163`），而 MCP 的拒绝是带码的 HTTP 200，所以 `page.title_taken`、`page.cycle`、`page.too_deep` 记作跳过，`changed: false`（同一个父页、同一个标题）记作没有改动（设计审查的复核 B 的 BC-N4）；从新到旧连续撤销之后照旧等于之前。

### 3.6 S1：编辑锁对 MCP 删除的规则

00 号文档 4.10 的锁表：MCP 的删除被任何活着的会话拒绝，本人的也算，不结束任何会话。守卫按 `Write.Client` 认出 MCP（`Step` 嵌着 `Write`，`page/app/extension.go:38-52`）：

```go
// GuardWrite refuses a content write when the page's alive session is not
// the one it is made in; a deletion when another account's alive session
// holds a page of it, naming the first in the subtree's order, the
// deleter's own sessions ending with their pages; and an agent's deletion
// when any alive session holds one, its caller's own too, naming each in
// the subtree's order: an agent ends no edit (M9 design 4.10). Any other
// operation passes.
func (l *EditLock) GuardWrite(ctx context.Context, s Step) error {
	var holds func(EditSession) bool
	switch {
	case s.Operation == domain.OpContent:
		holds = func(a EditSession) bool { return a.ID != s.EditSessionID }
	case s.Operation == domain.OpDelete && s.Client.IsMCP():
		holds = func(EditSession) bool { return true }
	case s.Operation == domain.OpDelete:
		holds = func(a EditSession) bool { return a.UserID != s.By }
	default:
		return nil
	}
	// … ids and held as today (edit_lock.go:54-67) …
	if s.Operation == domain.OpDelete && s.Client.IsMCP() {
		return l.lockedAll(ctx, ids, held) // every held page, in ids' order; nil when none
	}
	// … the first held, as today (:68-73) …
}

// lockedAll is page.locked naming every page of ids that held holds, in
// ids' order (the subtree's: level by level), their owners named in one
// read: the first is its Lock, all its Locks.
func (l *EditLock) lockedAll(ctx context.Context, ids []uuid.UUID, held map[uuid.UUID]EditSession) error
```

`domain.LockedAll(holders []shared.LockHolder) error` 复制 `ErrLocked`、设 `Lock = &holders[0]`、`Locks = holders`（`page/domain/errors.go` 挨着 `Locked`）。次序是 `ids` 的，即删除的那一步的改动的次序：子树逐层、每层按次序值与 id（`Store.Subtree`，`page/adapter/postgres/store.go:243-263`），不是 `LockHolders.Of` 与 `AliveSessionsOf` 的页 id 的次序。被持着的页至多是子树的页数，一页至多一个活着的会话（`checkPages` 守着），名字一次 `DisplayNames`；答复里的上限在 mcpserver（3.10）。REST 的删除从不带 `mcp:`，它的 `page.locked` 照旧只有 `lock`（`deleteNode` 的契约不变）。

守卫通过之后，`DeleteNodeSessions` 删这些页的全部会话行（`page/adapter/postgres/queries/sessions.sql:84-89`），这时活着的一个也没有（有就被拒了），所以删的只是过期的与墓碑；`tellEnded` 只告诉活着的（`unit_session.go:101-111`），没有人被告诉：MCP 的删除不结束任何会话。守卫的读不加锁，与开会话的串行靠笔记本行的 `FOR NO KEY UPDATE`（`edit_lock.go:10-16`），交错见 3.13.4。

测试（`page/app/edit_lock_test.go`，`TestTheEditLockGuardsByOperation` 的表加行；`newLockFixture` 的树是 Top 下 Mid、Mid 下 Low 与 Side，alice 的会话在 Top、bob 的在 Low 与 Side，Side 的早建一秒，`edit_lock_test.go:37-55`）：`mcp:x` 的 alice 删除 Top，这一步的改动给成 Top、Mid、Side、Low（不是页 id 的次序：五页依次以 `uuid.NewV7` 建，id 的次序是 Top、Mid、Low、Side，`fakes_test.go:1276-1288`）→ `page.locked`，`Lock` 是 Top（她自己的）、`Locks` 是 Top、Side、Low（`ids` 的次序，不论替身以什么次序答会话；按页 id 排就成了 Top、Low、Side，变异 53 在这里失败，设计审查的复核 B 的 BC-M3），名字都在；同样的删除（同样的改动的次序）以 `web` 的客户端 → 被 bob 的 Side 拒（这一步的次序里第一个别人的：`GuardWrite` 照改动的次序找，`page/app/edit_lock.go:44-74`，所以这一行也核对了 `web` 的路照这一步的次序；设计审查的第二次复核 B 的 CN-2）、`Locks` 为空；`mcp:x` 删除没有会话的 Free → 放行；`mcp:x` 的正文写 → 照旧被任何会话拒；`mcp:x` 的新建、改名、移动 → 放行。整个程序里两个以上的锁的次序与"不是页 id 的次序"由锁的第 11 格核对（3.13.3）。

### 3.7 S1：`NodePaths.Walk` 与 `NodePaths.Paths`

```go
// Walk looks keys up from the notebook's root, one segment after another
// as Find does, in the transaction ctx carries when it carries one: the
// nodes found, from the root, up to the first key that names nothing (all
// of them when each names one). A key before the last that names an
// attachment is an *AttachmentOnPath of its segment: an attachment holds
// no nodes.
func (p NodePaths) Walk(ctx context.Context, notebookID uuid.UUID, keys []string) ([]domain.Node, error)

// Paths is the paths of ids' nodes alive in notebookID, by id, as PathOf
// writes them: one statement whatever their number (PathStore.PathsOf). A
// node deleted, purged or of another notebook has none.
func (p NodePaths) Paths(ctx context.Context, notebookID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]string, error)
```

- `Walk` 与 `Find` 共用每段一次的点查（`RootByKey`、`ChildByKey`，`page/app/node_paths.go:35-70`），不做 `Find` 的提示与 `titled` 的再查：它给写的用例在单元里用，单元的笔记本行的锁让找到的链在单元里不变（00 号文档 4.8）。`domain.AttachmentOnPath{Segment int}` 是一个不带码的值，用例照自己的字段换成 422（`path` 或 `to`，3.8）。
- `Paths` 经新的端口方法 `PathStore.PathsOf(ctx, notebookID, ids) (map[uuid.UUID]string, error)`。存储以已有的 `LinkTargetsByIDs` 实现（递归的一条语句，按参数做计划，`page/adapter/postgres/queries/links.sql:25-42`、`links.go:38-52`；没有统计时的耗时已由 M6 收尾的测试守着，13.4 第 9 条），每条路径以 `domain.PathOfNames(names, kind)` 写出：它是从 `PathOf`（`page/domain/path.go:100-111`）抽出的同一段代码，所以给出路径的仍只有一处。不加新的 SQL。
- 调用它的：`move` 的 `linking.pages_locked` 与四个写的工具的锁的拒绝（单元回滚之后、不加锁，至多 `domain.MaxNamedLocks` 个，3.8.5）、`changeset` 的一页（3.16.3）。模块根的 `page.NewNodePaths(pool)` 与 linking 的端口仍是 P4 的（2.1 第 7 条），`node_paths.go:10-12` 的注释随之改写。
- 测试：存储的 `TestThePathsOfNodesAreTheirPaths`（随机的树，含标题以 `.md`、`.MD` 结尾的页、叫 `x.png` 的页、附件；回收站里的、别的笔记本的没有）；P1 的性质测试 `TestEveryNodesPathFindsIt` 加一条：每个活着的节点的 `Paths` 等于 `PathOf`，以它 `Find` 找回的就是它。`Walk` 的表格测试：全找到、中途缺、中途是附件（第几段）、最后一段是附件（找到，由调用者判断）。

### 3.8 S1：四个用例

四个用例都在 `page/app`，由 `page.New` 建一次，交给 `mcpadapter.UseCases`；REST 的十个用例不改（00 号文档 4.10）。共同的做法：

- **判定**：先以 `decideNotebook(…, domain.ActionRead, notebookID)` 判定笔记本看不看得到（`notebook.not_found`，零值的 id 不查库，`page/app/authorize.go:28-43`），再找节点（`page.not_found`），再以 `decideNotebook(…, <写的动作>, notebookID)` 判定这个写（403）：所以次序是笔记本 → 节点 → 403（00 号文档 4.15）。新建（`write` 不带 `base_version`）没有节点，以 `ActionCreate` 判定一次，403 紧跟笔记本。`Run` 在锁下再判定一次（`unit.go:207-210`），`UnitSpec.NotFound` 是 `domain.ErrNotebookNotFound`：寻址的是（笔记本，路径），笔记本在其间被删时答它。
- **单元里按路径找**：`do` 里以 `NodePaths`（同一个 `PathStore`，语句经 `queries(ctx)` 进入单元的事务，`page/adapter/postgres/store.go:42-44`）在锁下再找一次；写的对象总是这次找到的节点，版本令牌只与它比较（00 号文档 4.8、4.10）。
- **每一步的规则在 page/domain**（设计审查 B-M7）：不要存储的规则写成（路径，找到的节点）的函数，用例在 `do` 里调它们，P3 的 `PlanBatch` 在它模拟的树上调同样的函数（3.20），一条规则只有一份（13.4 第 13 条）：

```go
// CreationOf is what writing a page at path creates, given the nodes Walk
// found from the root (M9 design 4.10): the leaf when it is there, else
// the segments to create, from the first missing one. 422 on field: a
// found node before the last segment that is an attachment, or a missing
// ancestor ending with .md; 409 page.too_deep for a path of more than
// MaxDepth pages.
func CreationOf(field string, path Path, found []Node) (Creation, error)

// MoveKinds is 422 on to when from and to are not of one kind: a page's
// path to a page's, an attachment's to an attachment's.
func MoveKinds(from, to Path) error

// DeletionOf is 422 on recursive (*Recursive) when sub has nodes under its
// root and recursive is false.
func DeletionOf(sub Subtree, recursive bool) error
```

  `write` 的叶子是页还是附件、内容同不同（3.8.1 第 3.4 步）要读正文的哈希，在用例里；`to` 的父页链的附件由 `Walk` 的 `AttachmentOnPath` 给出，`RelocatePath` 照 `to` 的字段换成 422。`steps_test.go` 的表格测试每条规则一行，P3 的计划测试照用。
- **客户端**：适配器交 `domain.MCPClient(Call.Client)`；`Options{UpdateLinks: true}`，只有 `move` 由参数定。
- **锁的拒绝的路径**：`Run` 以 `page.locked` 或 `linking.pages_locked` 失败时，以及 `edit` 的锁的预检拒绝时（它在 `Run` 之前，3.8.2 第 4.6 步：调用之前就开着的编辑器是最常见的一种，设计审查 A-M4、B-M2、C-M9），用例在单元之外、不加锁地以 `NodePaths.Paths` 读这些页现在的路径（至多 `domain.MaxNamedLocks` = 20 个，按错误里的次序），以 `domain.LockPaths{Err, Paths}` 包着交给适配器（3.8.5）；读不到的（其间被删了）没有路径。这次读本身失败时（ctx 结束、连接池出错）记 warn"lock paths not read"（带错误与页数），答 `LockPaths{Err, Paths: nil}`：每个路径为 null，拒绝照旧是 `page.locked`、带持锁人与"是你"，不变成 `internal_error`，也不丢掉文字。
- **日志**：照 REST 的 `logged(o, n, client)`（`page/app/create_page.go:61-69`：工作区、笔记本、节点、变更集、执行者与客户端的 id），什么都没写时不记，从不记路径与标题（3.14）。

#### 3.8.1 `WritePath`

```go
// PathWrite is write's arguments (M9 design 4.10): Base is base_version as
// given, nil without one.
type PathWrite struct {
	Path    string
	Content string
	Base    *string
}

// Written is a write's outcome: the page, its path by the names it has,
// its version, and the nodes the write created, from the root down, the
// page last (none when the page was there). Outcome's ChangesetID is zero
// when nothing changed.
type Written struct {
	Path    string
	Node    domain.Node
	Version domain.VersionToken
	Created []Placed
	Outcome Outcome
}

// Placed is a node and its path.
type Placed struct {
	Path string
	Node domain.Node
}

// WritePath writes a page's whole content by its path: without Base it
// creates the page with its missing parents, empty, in one unit; with Base
// it replaces the content of the page the path names, which Base's node
// must be.
type WritePath struct { /* writer, parser, paths, nodes, states, notebooks, auth, logger */ }

func NewWritePath(writer *Writer, parser *ContentParser, paths PathStore, nodes Nodes, states PageStates,
	notebooks Notebooks, auth shared.Authorizer, logger *slog.Logger) *WritePath

func (w *WritePath) Execute(ctx context.Context, notebookID uuid.UUID, p PathWrite, client domain.Client) (Written, error)
```

**码的次序**（新建与覆盖的前几步相同）：

1. `shared.RequireActor`（MCP 总有执行者）。
2. 与资源无关的 422：`domain.ParsePath("path", p.Path, false)`（4.8 的写法与段的检查）；不是页的路径（不以 `.md` 结尾）答 422 `path` `not_allowed`："is an attachment's path: write writes pages, whose paths end with .md; attachments are uploaded in the web"；`domain.CheckContent("content", p.Content)`（5 MiB、UTF-8、NUL，`page/domain/content.go:14-29`）；有 `Base` 时 `domain.ParseVersionToken("base_version", *p.Base)`。
3. **新建**（没有 `Base`）：
   1. `decideNotebook(ActionCreate)`：`notebook.not_found`、403。
   2. `parser.Decided(ctx, p.Content)`：503（解析的预算，单元之前取，13.1 第 19 条）；`defer release()`。空的父页的提取结果是 `parser.Decided(ctx, "")`（空的正文不取预算，`page/app/content.go:55-58`）。
   3. `Run(UnitSpec{NotebookID, Action: ActionCreate, Tree: true, Client, Options{UpdateLinks: true}, NotFound: ErrNotebookNotFound}, do)`，`do` 里：
      1. `found := paths.Walk(ctx, notebookID, path.Keys)`。
      2. `domain.CreationOf("path", path, found)`：路过的附件答 422 `path` `not_allowed`："segment N is an attachment, which holds no pages"；缺的祖先（`found` 之后、最后一段之前的每一段）以 `.md` 结尾（ASCII 不区分大小写）时答 422 `path` `not_allowed`："segment N ends with .md: an ancestor is written by its title, without .md"（00 号文档 4.10；有一页就叫 `A.md` 时它在 `found` 里，不补建）；`len(path.Keys) > domain.MaxDepth`：409 `page.too_deep`（一页的层数就是它的段数，`page/domain/node.go:22-23`；先于写任何东西）。
      3. （同上）。
      4. 叶子已在（`Creation` 给出它）：是页时以 `Nodes.LockContent` 读它的哈希与字节数（改树的单元里锁正文行无害），与 `p.Content` 的 SHA-256 与长度都相同就什么也不写，答它的版本令牌；不同答 409 `page.title_taken`（`domain.PathTaken{Path, Asset: false}`）；是附件答 409 `PathTaken{Asset: true}`。撞名不加序号（与导入不同）。
      5. 从第一个缺的段起，依次 `u.CreatePage(ctx, PageDraft{ParentID: 上一个, Title: path.Titles[i], Content: "", Facts: 空的})`（最后，`Position{}`），叶子带 `p.Content` 与它的提取结果；每个照常经守卫（新建放行）、条目、版本（`base_revision` 为空、版本号 1）、参与者（新建不改写，`linking/domain/change.go:68-72`）。一个变更集、一个时刻、一次 `pages` 事件。`CreatePage` 自己的检查（标题、父页、名称、层级，`unit_create.go:28-53`）在这里都不会答：段已查过、父页刚找到或刚建、名称在锁下是空的、层级已比过。
4. **覆盖**（有 `Base`）：
   1. `decideNotebook(ActionRead)`：`notebook.not_found`。
   2. `paths.Find`（不加锁）：`page.not_found`（`*domain.PathNotFound`，带 P1 的提示）。
   3. `decideNotebook(ActionWrite)`：403。
   4. `parser.Decided`：503；`defer release()`。
   5. `Run(UnitSpec{NotebookID, Action: ActionWrite, Client, Options{UpdateLinks: true}, NotFound: ErrNotebookNotFound}, do)`（不改树：笔记本行 `FOR SHARE`，挡住改树的写），`do` 里：
      1. `paths.Find`（锁下）：`page.not_found`。
      2. 令牌的节点不是找到的节点：409 `domain.VersionMismatch{Path, Current: (节点, PageStates 读的版本号), Why: OtherNode}`。别的笔记本的令牌、已删除的、从没有过的节点都在这里：令牌的 id 从不拿来读、锁或写（00 号文档 4.10）。
      3. `u.WriteContent(ctx, ContentWrite{NodeID: 找到的, Content, Facts, Base: 令牌的版本号})`：内容与当前相同时什么也不写，即使版本号过时（2.1 第 10 条，`unit_content.go:62-64`），打开的编辑器也不拒它（不经守卫）；版本号不符 → 409 `VersionMismatch{Why: Changed, Current: 锁下的版本号}`（锁着的正文行上以 `PageStates` 读一次）；守卫的 `page.locked`（任何活着的会话，本人的也拒，`edit_lock.go:47-48`）。
5. 日志：新建是"page created"，另记 `pages`（建了几页）；覆盖是"page content written"，另记 `revision`；未改动不记。

#### 3.8.2 `EditPage`

```go
// PageEdit is edit's arguments (M9 design 4.10).
type PageEdit struct {
	Path     string
	Old, New string
	All      bool    // replace_all
	Base     *string // base_version as given, nil without one
}

// Edited is an edit's outcome: the page, its version, how many times
// old_string was replaced, and whether the edit was made again once
// (Redone) after the page changed between its read and its write.
type Edited struct {
	Path         string
	Node         domain.Node
	Version      domain.VersionToken
	Replacements int
	Redone       bool
	Outcome      Outcome
}

// EditPage replaces a string in a page's content on the server, atomically
// (M9 design 4.10): read unlocked, replaced, parsed, then written with the
// revision it read as the unit's Base, the path found again under the
// lock; once more from the read when the page or the path moved on in
// between, without Base.
type EditPage struct { /* writer, parser, paths, nodes, states, locks, notebooks, auth, clock, logger */ }

func NewEditPage(writer *Writer, parser *ContentParser, paths PathStore, nodes Nodes, states PageStates,
	locks *LockHolders, notebooks Notebooks, auth shared.Authorizer, clock Clock, logger *slog.Logger) *EditPage

func (e *EditPage) Execute(ctx context.Context, notebookID uuid.UUID, p PageEdit, client domain.Client) (Edited, error)
```

**码的次序**（00 号文档 4.15 的 `edit` 一条，逐步写定）：

1. `RequireActor`。
2. 与资源无关的 422：`ParsePath("path")`、是页的路径（同 `write`）；`domain.CheckEdit(p.Old, p.New)`（3.9）；有 `Base` 时 `ParseVersionToken("base_version")`。
3. `decideNotebook(ActionRead)`：`notebook.not_found`。
4. 一次尝试（至多两次；有 `Base` 时一次），每次在它自己的函数里，`defer release()` 在那一次结束时放回预算：
   1. `paths.Find`（不加锁）：`page.not_found`。
   2. 第一次：`decideNotebook(ActionWrite)`：403（读者先得 403，不得 422）。
   3. `Nodes.PageContent(节点)`（不加锁，正文、版本号 r 与哈希，`page/app/ports.go:76-81`）：没有了答 `page.not_found`。
   4. 有 `Base` 而令牌不是（N, r）：409（节点不同 `OtherNode`，版本号不同 `Changed`），在匹配之前（在过时的正文上匹配没有意义，同 `toggleTask`）。
   5. `domain.Replace(正文, p.Old, p.New, p.All)`（3.9）：422 `old_string`（没有、多处）、422 `new_string`（替换之后长过 5 MiB）。
   6. 锁的预检：`locks.Of(ctx, [节点], clock.Now())`（不加锁，`page/app/lock_holders.go:29-51`）有活着的会话：`page.locked`，不占解析的预算；拒绝照"锁的拒绝的路径"包上这一页的路径（`LockPaths`，与 `Run` 的拒绝同样的文字与 `lock`）。
   7. `parser.Decided(替换之后的正文)`：503。
   8. `Run(UnitSpec{NotebookID, Action: ActionWrite, Client, Options{UpdateLinks: true}, NotFound: ErrNotebookNotFound}, do)`，`do` 里：`paths.Find`（锁下）：`page.not_found`；找到的不是 N：**重做**的信号；`u.WriteContent(ContentWrite{NodeID: N, Content: 替换之后的, Facts, Base: r})`：`ErrRevisionMismatch` 是重做的信号；守卫的 `page.locked`（预检与单元之间开的会话）。
5. **重做**（00 号文档第 7 节 P2 一行的"重做"、4.10 的"锁下版本号不再是 r 或路径换了节点时……从读起重做一次"、4.14 的"`edit` 的重试 1 次"）：单元回滚之后，没有 `Base` 而这是第一次：从第 4.1 步再来一次（路径重新找、正文重新读、替换在新的正文上重新成立，否则 422；预检、解析也重新做），答复的 `Redone` 为真；有 `Base`：409 `VersionMismatch{Why: Changed 或 OtherNode}`，不再读正文；第二次也不成：409 `VersionMismatch{Why: KeptChanging}`。当前的令牌在单元之外以 `paths.Find` 与 `PageStates` 读。替换只作用于它读到的那一版的字节，所以不会丢别人的写；不在正文行的锁下替换：那样要在事务里解析（13.1 第 19 条）。
6. **未改动**：替换之后的正文与锁下的当前正文相同时 `WriteContent` 什么也不写，答复的 `changed` 为假、`changeset_id` 为 null：CRLF 的规则把两个串变成同一个（3.9），或读与单元之间别人的写恰好把这一页写成了替换的结果（`unit_content.go:62-64`，设计审查 B-N3）。
7. 日志："page edited"，另记 `revision`、`replacements`、`redone`。

#### 3.8.3 `RelocatePath`

```go
// PathMove is move's arguments (M9 design 4.10): UpdateLinks is
// update_links, true when the agent left it out.
type PathMove struct {
	From, To    string
	UpdateLinks bool
}

// Relocated is a move's outcome: the node at its path after.
type Relocated struct {
	Path    string
	Node    domain.Node
	Outcome Outcome
}

// RelocatePath moves, renames or both a page or an attachment from its
// path to its full new path in one step (Unit.Relocate), its links
// rewritten unless UpdateLinks is false.
type RelocatePath struct { /* writer, paths, notebooks, auth, logger */ }

func NewRelocatePath(writer *Writer, paths PathStore, notebooks Notebooks, auth shared.Authorizer, logger *slog.Logger) *RelocatePath

func (m *RelocatePath) Execute(ctx context.Context, notebookID uuid.UUID, p PathMove, client domain.Client) (Relocated, error)
```

**`to` 的意思**：总是节点的完整新路径（`A/x.md` 到 `B/y.md` 是移到 `B.md` 之下并改名为 `y`），从不是"移进去"的文件夹（2.1 第 15 条）。理由：MCP 的路径是寻址（4.8），一个路径只指一个节点；"移进 `B`"要在 `B` 已是页时与"改名为 `B`"有歧义，`mv` 的两种意思正是 agent 常写错的。agent 写 `to: "B.md"` 想移进 `B.md` 而根下已有 `B.md` 时答 409，文字说 `to` 是完整的新路径。

**码的次序**（2.1 第 3 条）：

1. `RequireActor`。
2. 与资源无关的 422：`ParsePath("from")`、`ParsePath("to")`；`domain.MoveKinds(from, to)`：两者的类型不同（一个以 `.md` 结尾、一个不）答 422 `to` `not_allowed`："is not of from's kind: a page moves to a page's path, ending with .md, an attachment to an attachment's"。
3. `decideNotebook(ActionRead)`：`notebook.not_found`。
4. `paths.Find(from)`（不加锁）：`page.not_found`。
5. 动作：`to` 的父段的键与 `from` 的父段的键相同（同一个父节点）是 `ActionRename`，否则 `ActionMove`（两者都是写者，`access/domain/rules.go`）；`decideNotebook(动作)`：403。键相同就是同一个父节点：锁下按路径找到的父节点唯一。
6. `Run(UnitSpec{NotebookID, Action: 动作, Tree: true, Client, Options{UpdateLinks: p.UpdateLinks}, NotFound: ErrNotebookNotFound}, do)`，`do` 里：
   1. `paths.Find(from)`（锁下）：`page.not_found`。
   2. `to` 的父页链：一段的 `to` 是根；否则 `paths.Walk(to.Keys[:n-1])`，第一个不成的段决定（`Walk` 停在那里，3.7；设计审查的复核 B 的 BC-N6）：缺的答 `page.not_found`（`PathNotFound{Path: to 的父页的路径, Hint: domain.MoveParentHint}`："move creates no parent pages: write the parent first, or move to an existing page's path"）；路过的附件、或父段本身是附件，答 422 `to` `not_allowed`："segment N is an attachment, which holds no nodes"。所以 `A/img.png/missing/y.md` 是 422（`AttachmentOnPath{2}`），`A/missing/img.png/y.md` 是 404。
   3. `u.Relocate(ctx, from 的节点, Placement{ParentID: 父页, Name: to 的最后一段的标题或名称, Field: "to"})`（3.5.1）：422 `to`（附件改名去掉了扩展名等）→ 409 `page.cycle`、`page.title_taken`、`page.too_deep`。`page.title_taken` 是 `Relocate` 答的 `*domain.PathTaken{Asset}`（占着的兄弟的类型，取自它读过的兄弟），用例填上 `Path: to`：文字说 `to` 是完整的新路径。只改大小写的 `a/x.md → a/X.md` 在 `to` 按键找到的就是这个节点，不算撞名；`a/x.md → b/X.md` 换了父节点，在新的兄弟里看（`b` 下有 `x` 就撞）。
   4. 参与者（改写链接，只在 `UpdateLinks`）：预检的 `linking.pages_locked`（要改写的页被编辑，本人的也算）→ 解析的 503（单元里不排队地取预算，`linking/app/rewrite.go:236`）→ 写回时守卫的 `page.locked` 换成的 `linking.pages_locked`（`:147-161`）。`UpdateLinks` 为假时参与者什么都不做、也不预检锁（`rewrite.go:48-51`），所以被编辑的页不拒；观察者照样重新解析（`bootstrap/linking.go:25-38`），链接可能断开或指向别处（3.13.5）。
7. 答复的路径：`to` 的父页链（找到的名称）加移动之后的节点，`domain.PathOf`。移到它已在的地方、名称不变：什么也不写，答原来的路径，`changeset_id` 为 null。
8. 日志：换了父节点是"node moved"，否则"node renamed"，另记 `update_links`；没写不记。

#### 3.8.4 `DeletePath`

```go
// PathDelete is delete's arguments (M9 design 4.10).
type PathDelete struct {
	Path      string
	Recursive bool
}

// Deleted is a deletion's outcome: the node deleted and how many pages and
// attachments went into the trash with it, itself included.
type Deleted struct {
	Path          string
	Node          domain.Node
	Pages, Assets int
	Outcome       Outcome
}

// DeletePath puts a page or an attachment in the trash by its path; a page
// with children only with Recursive.
type DeletePath struct { /* writer, paths, notebooks, auth, logger */ }

func NewDeletePath(writer *Writer, paths PathStore, notebooks Notebooks, auth shared.Authorizer, logger *slog.Logger) *DeletePath

func (d *DeletePath) Execute(ctx context.Context, notebookID uuid.UUID, p PathDelete, client domain.Client) (Deleted, error)
```

**码的次序**：

1. `RequireActor`。
2. `ParsePath("path")`：422。
3. `decideNotebook(ActionRead)`：`notebook.not_found`。
4. `paths.Find`（不加锁）：`page.not_found`。
5. `decideNotebook(ActionDelete)`：403。
6. `Run(UnitSpec{NotebookID, Action: ActionDelete, Tree: true, Client, Options{UpdateLinks: true}, NotFound: ErrNotebookNotFound}, do)`，`do` 里：
   1. `paths.Find`（锁下）：`page.not_found`。
   2. `sub := u.Subtree(ctx, 节点)`。
   3. `domain.DeletionOf(sub, p.Recursive)`：子树多于一个节点而没有 `recursive` 时 422 `*domain.Recursive{Pages, Assets}`（其下的页数与附件数，即 `sub[1:]` 的 `Counts()`，`page/domain/tree.go:66-77`）。在单元里、笔记本行的锁下：不加锁的预检看不到其间新加的子节点（00 号文档 4.10）。
   4. `u.DeleteSubtree(ctx, sub)`：守卫的 `page.locked`（MCP 的规则：任何活着的会话，3.6）。
7. 答复：路径、节点、`sub.Counts()`（自己也算）、变更集与时刻。没有大小的界线（同 REST；M12 的性能移交第 14 项）。
8. 日志："node deleted"，另记 `nodes`、`pages`、`assets`。

#### 3.8.5 用例的错误

`page/domain/errors.go` 加几个带信息的错误，它们 `Unwrap` 到已有的码，所以 REST 的分类照旧、mcpserver 照样分类（`ProblemOf` 经 `errors.As` 找 `ProblemError`）：

```go
// PathTaken is page.title_taken at a path: a node holds the name the path
// asks for, a page's or an attachment's (Asset).
type PathTaken struct {
	Path  string
	Asset bool
}

// VersionMismatch is page.revision_mismatch at a path: why, and the
// page's version there now.
type VersionMismatch struct {
	Path    string
	Current VersionToken
	Why     Mismatch // OtherNode, Changed, KeptChanging
}

// LockPaths is a lock's refusal, page.locked or linking.pages_locked, with
// the paths of the first MaxNamedLocks pages it names, by page id: a page
// gone since has none, and none has one when they could not be read.
type LockPaths struct {
	Err   error
	Paths map[uuid.UUID]string
}

// MaxNamedLocks is how many of a refusal's locks have their paths read:
// mcpserver.MaxErrorLocks, which page's adapter/mcp holds it to.
const MaxNamedLocks = 20

// AttachmentOnPath is a path's segment, from 1, that names an attachment
// where the path goes on under it.
type AttachmentOnPath struct{ Segment int }

// Recursive is 422 on recursive: a page with Pages pages and Assets
// assets under it is deleted only with recursive.
type Recursive struct{ Pages, Assets int }

// LockedAll is page.locked naming each of holders, the first as Lock too.
func LockedAll(holders []shared.LockHolder) error
```

`PathTaken`、`VersionMismatch`、`Recursive` 的 `Error()` 是被包着的码的说明（同 `PathNotFound`，`page/domain/path.go:115-121`），`Unwrap()` 是 `ErrTitleTaken`、`ErrRevisionMismatch` 与 `recursive` 的 422（`shared.Invalid(FieldError{Field: "recursive", Code: required, Message: "must be true to delete a page with children: 3 pages and 2 attachments"})`，字段的消息不写路径，3.10.3）；`LockPaths` 的 `Unwrap()` 是 `Err`。适配器以 `errors.As` 认出它们、读出计数（设计审查 A-M5：一个答错误的函数给不出计数）。`MaxNamedLocks` 与 `mcpserver.MaxErrorLocks` 相等由 page 的 `adapter/mcp` 的测试核对（page 的 domain 不导入 mcpserver）。

#### 3.8.6 用例的测试

`page/app` 的表格测试（替身的端口，照 P1 的 `ReadPath` 的测试）：每个用例的每一步各一行，按次序：前一步的错误挡住后一步（例如没有笔记本时坏的令牌仍是 422、看不到的笔记本里不存在的路径是 `notebook.not_found`、读者对不存在的路径是 `page.not_found` 而对存在的是 403、`edit` 的读者对不匹配的 `old_string` 是 403）。另有：

- `write`：建三层（两个缺的祖先）是一个单元、一个变更集，`Created` 从根往下；已有的父页按库里的大小写给出路径；内容相同答未改动、不建变更集；令牌的节点不同、版本号过时、两者都对而内容相同（未改动）；祖先以 `.md` 结尾而缺（422）与它已在（找到）；路过附件；11 段（409）；叶子是附件（409，`Asset`）。
- `edit`：唯一、多处、`replace_all`、CRLF（3.9）；预检的锁（不占解析的预算，拒绝带这一页的路径）；第一次单元里版本号变了而第二次成（`Redone`，替身的 `PageContent` 读了两次）；两次都变（`KeptChanging`）；有 `Base` 时单元里版本号变了：409 `Changed`、`PageContent` 只读了一次（不重做，6.3 第 22 条）；单元里路径换了节点（重做，第二次在新的节点上，`old_string` 不在里面答 422）；读与单元之间别人的写恰好写成替换的结果（`changed` 为假）。
- `move`：同一父节点的改名（`node.rename`）、换父节点（`node.move`）、两者、只改大小写、跨父节点只改大小写、附件换扩展名（成）与去掉扩展名（422）、`to` 的父页缺（提示）、穿过附件、第一个不成的段决定（`A/img.png/missing/y.md` 是 422、`A/missing/img.png/y.md` 是 404）、类型不同、`to` 被页占与被附件占（`PathTaken` 的 `Asset` 与 `Path`）、环、太深；`UpdateLinks` 交到 `UnitSpec.Options`（替身的参与者看到它）。
- `delete`：单个页、单个附件、有子节点而没有 `recursive`（`*domain.Recursive` 的计数）、带它；`MCP` 的客户端交到守卫（替身的守卫看到 `Write.Client`）。
- 锁的拒绝的路径：`Run` 的拒绝、`edit` 的预检的拒绝都带路径；`PathsOf` 失败时拒绝照旧是 `page.locked`、`Paths` 为 nil、记一行 warn（替身的 `PathStore` 答错误）。

### 3.9 S1：`edit` 的匹配

```go
// MaxEditOldBytes bounds an edit's old_string: the 1 MiB besides a page's
// content that a call's body has room for (bootstrap's mcpBodyLimit; M9/P2
// design 2.1 item 5).
const MaxEditOldBytes = 1 << 20

// CheckEdit checks an edit's two strings, given (M9 design 4.10):
// old_string of 1 to MaxEditOldBytes bytes, new_string of at most
// MaxContentBytes, both UTF-8 without NUL, and different; 422 on the one
// that is not.
func CheckEdit(old, new string) error

// Replacement is an edit's content: the page's with old_string replaced,
// how many times, and whether the two were matched as CRLF.
type Replacement struct {
	Content string
	Count   int
	CRLF    bool
}

// Replace replaces old with new in content, byte for byte (M9 design
// 4.10): once, when old occurs once (a second occurrence, overlapping or
// not, makes it ambiguous), or each occurrence with all, left to right as
// strings.ReplaceAll does. When old does not occur, content's line breaks
// are all CRLF and old's are bare LF, both are matched and written with
// each bare LF made CRLF. 422 on old_string when old does not occur
// (hinting when it would under NFC) or occurs more than once without all;
// on new_string when the content would pass MaxContentBytes, which it
// knows before it builds it. Every search is the matcher's: linear in the
// content and old, whatever their bytes.
func Replace(content, old, new string, all bool) (Replacement, error)

// matcher finds a pattern in a text in time linear in both, whatever their
// bytes (Knuth–Morris–Pratt): the one search Replace makes, for finding,
// counting and replacing, and for the CRLF and NFC retries.
// strings.Index is never given an agent's pattern: past bytealg.MaxLen it
// is Rabin–Karp, quadratic for a crafted one (M9/P2 design review B-I2).
type matcher struct {
	p    string
	fail []int32 // fail[i]: the longest proper border of p[:i+1]
}

func newMatcher(p string) matcher

// index is the start of the first occurrence of m.p in s at or after from,
// -1 for none: one scan of s[from:] up to the occurrence's end, at most two
// comparisons a byte scanned in all; while no prefix of p is matched, the
// bytes that cannot start p are skipped with IndexByte.
func (m matcher) index(s string, from int) int
```

- **`CheckEdit` 的码**：`old_string` 空：`too_short` "must not be empty"；过 1 MiB：`too_long` "must be at most 1 MiB (1,048,576 bytes)"；`new_string` 过 5 MiB：`too_long`（`CheckContent` 的文字）；不是 UTF-8 或带 NUL：`invalid_format`（同 `CheckContent`）；两者相同：`new_string` `not_allowed` "must differ from old_string"。都不复述取值。
- **匹配**：`m := newMatcher(old)`，`i := m.index(content, 0)`。没有：页的换行全是 `\r\n`（至少一个 `\r\n`，每个 `\n` 前面都是 `\r`）、`old` 里有 `\n` 而没有 `\r` 时，把 `old` 与 `new` 里单独的 `\n`（前面不是 `\r` 的）换成 `\r\n`，以换过的 `old` 建一个新的匹配器再找一次（`CRLF` 为真；混用两种换行的页不这样做，00 号文档 4.10）。还没有：`old_string` `not_allowed` "is not in the page: copy it from what read gave, byte for byte"；页或 `old` 不是 NFC（`norm.NFC.IsNormalString`）而两者的 NFC 以同一种匹配器配得上时另加 "; the page's text and old_string differ in Unicode normalization (NFC)"。BOM 是正文的字节，照原样。
- **唯一**：不带 `all` 时 `m.index(content, i+1)`（重叠的也找得到：`aa` 在 `aaa` 里是两处）；有就是 `old_string` `not_allowed` "is found N times; add context to make it unique, or set replace_all"，N 是不重叠的计数与 2 中较大的。**计数**是 `for j := m.index(s, 0); j >= 0; j = m.index(s, j+len(old))`：从左到右、不重叠，同 `strings.Count` 与 Claude Code 的 `replace_all`；每次从上一处的末尾接着扫，扫过的范围互不重叠，合起来至多 2n 次比较（唯一的检查从 i+1 再扫，至多多扫 `len(old)` 个字节）；带 `all` 时次数是它，替换以同样的循环拼出（`strings.Builder` 先 `Grow` 到算好的大小，不存每一处的位置）。
- **大小**：`len(content) + count*(len(new)-len(old))` 过 `MaxContentBytes` 时答 `new_string` `too_long` "would make the page longer than 5 MiB (5,242,880 bytes)"，不先拼出来；CRLF 的规则之下以换过的两个串算（设计审查 B-N2）。
- **只有匹配器拿 agent 的串作模式**：找、切与换的一族都不收 `old` 或它的 CRLF、NFC 的形式。一族是 `strings` 与 `bytes` 里名字以 `Index`、`LastIndex`、`Contains`、`Count`、`Cut`、`Split`、`Replace` 起头的、`NewReplacer` 与 `Trim`、`TrimLeft`、`TrimRight`（cutset 不是 ASCII 时每个字节查一遍 cutset），加 `regexp` 的 `Compile`、`MustCompile`、`CompilePOSIX`、`MustCompilePOSIX`、`Match`、`MatchString`、`MatchReader`：模式就是程序，`regexp.MustCompile(regexp.QuoteMeta(old))` 是 O(n·k)，1 MiB 的 `a` 的页上 1,000 个 `a` 加 `b` 的 `old` 4.8 秒（设计审查的第二次复核 B 实测，`review-B/check/kmp/zz_b4_test.go`）。`TestReplaceSearchesOnlyWithItsMatcher` 以 `go/parser` 读 `old_string` 经过的三个包（page 的 `adapter/mcp`、`page/app`、`page/domain`）的每个非测试文件，不只是 `edit.go`：放在别的文件里的再找的帮助函数也读得到。包按每个文件的导入路径认：别名（`import s "strings"` 之后的 `s.Index`）与点导入（不带前缀的 `Index`）都算。这一族的调用，模式的实参（`Index` 的第二个、`Replace` 的 `old`、`NewReplacer` 的每个旧串、`Trim` 的 cutset、`regexp` 的模式……）都是至多 32 字节的字面量（字符串、字符、数，或字面量的 `[]byte(...)`；任何找法每扫一个字节至多比模式那么多次，32 次之内是线性的）的照用；别的都要在一张明确的表里（`allowedSearches`：文件、所在的函数、调用的源码与理由），否则测试失败；这一族的函数不作调用的函数而被引用的（`f := strings.Index`、当作值传出去）同样要在表里；表里的一项对不上任何调用也失败，不留过时的项（设计审查的复核 B 的 BC-M1，主持者的决定；第二次复核 B 的 CN-1）。S1 时表里八项，照 S1 改过之后的源码写（照今天的源码写的项到 S1 就对不上，测试失败；第二次复核 B 的 CN-4），理由照括号里的写：匹配器在没有配上前缀时跳过不可能起头的字节的 `strings.IndexByte(s[i:], m.p[0])`（模式的一个字节，线性的；照实际的源码写）；`Client.Valid` 的 `strings.CutPrefix(string(c), mcpPrefix)`（4 字节的常量）与 `strings.ContainsFunc(name, shared.Unshowable)`（一个函数），都是 3.3.1 写的；`ParseGlob` 的 `strings.ContainsFunc(seg, func…)`（`glob.go:114`）与 `lexemes.scan` 的 `strings.IndexFunc(s[i:], func…)`（`search.go:304`），模式是一个函数；`inDataURL` 的 `strings.ContainsRune(…, r)`（在一个七个字节的常量里找一个字符，`search.go:345`）；`parseTerm` 与 `SearchQuery.InContents` 的 `regexp.MustCompile(t.pattern(syntaxRE2, …))`（`search.go:547`、`:580`，M8 的搜索的词，查询至多 `MaxSearchQueryBytes` = 256 字节）。三个包今天别的这一族的调用都以短的字面量作模式（`glob.go`、`path.go` 的 `"/"`，`read.go` 的换行，`content.go`、`changeset.go` 的 0……，最长的是 `client.go` 的 4 字节的 `"mcp:"`，S1 起是 `mcpPrefix`）；page 的 `adapter/mcp` 与 `page/app` 没有这一族的调用，三个包都没有别名、点导入或当作值的引用（第二次复核 B 扫过，`review-B/check2/scan/`）。它的反向对照：合成的源码里，另一个文件的帮助函数的 `strings.LastIndex(content, old)`、以参数作分隔的 `bytes.SplitN(b, sep, 2)`、`strings.NewReplacer(old, new)`、`strings.CutPrefix(s, p)`、`regexp.MustCompile(regexp.QuoteMeta(old))`、`import s "strings"` 之后的 `s.Index(content, old)`、点导入的 `bytes` 的 `Index(b, sep)`、`f := strings.Index` 与一个 33 字节的字面量的模式各被报出；表里有的照用；表里多一项而源码里没有的被报出。时间的守卫另以构造的形状核对每一处的找法，含 CRLF 与 NFC 的再找（下面）。
- **与 Claude Code 的 Edit 不同的**（写进工具的说明，人工清单核对，00 号文档 4.16）：它把弯引号与直引号互相匹配、`new_string` 为空时连 `old_string` 之后的换行一起删；这里只按字节。唯一按"有没有第二处"判断，比它按不重叠的计数严。
- **CRLF 的规则把两个串变成同一个**（`old` 是 `a\n`、`new` 是 `a\r\n`，页全是 CRLF）：替换之后的正文等于原来的，`WriteContent` 什么也不写，答复 `changed: false`、`changeset_id` 为 null。
- **正确**：`TestTheMatcherIsStrings`（`edit_test.go`）：10 万个随机的（正文至多 40 字节、模式至多 6 字节，字母表 `ab`、`abc`、`a\r\n`、`aé`）：`index`（从 0 与随机的起点）、计数、全部替换等于 `strings.Index`、`Count`、`ReplaceAll`（只在测试里作预言）；`Replace` 的唯一、多处（含重叠）、CRLF 的再找与 `replace_all` 等于逐个位置比较的预言。草稿区的实验上 257,204 例都相同（`scratchpad/m9/p2/exp/kmp`；复核 B 以更宽的字母表与起点另跑 376,287 例，也都相同）。`edit_test.go` 另有（设计审查的复核 B 的 BC-N5）：**CRLF 的大小一行**：全是 CRLF 的页里有 `a\r\nb`，`old_string` 是 `a\nb`（LF 的两边有字：单独一个 `\n` 在页的每个 CRLF 里都找得到，第一次就找到、走不到 CRLF 的规则，设计审查的第二次复核 B 的 CN-3），`new_string` 带几个单独的 LF，页的长短取得让 `new_string` 换成 CRLF 之后页过 5 MiB、不换时恰好不过：答 `new_string` `too_long`（以换过的两个串算；变异 59 以没换的两个串算就不答）；**NFC 的提示两行**：页与 `old_string` 的 NFC 配得上时文字带提示，配不上时不带。

**代价与期限**（13.4 第 8 条；时间的守卫 `TestReplaceStaysWithinItsTime`，`page/domain/edit_cost_test.go`，`//go:build !race`，进 `make test-go-costs` 的 page/domain 一行，6.5）：每种形状跑一次、2 秒的时限，`-v` 打出每种的用时。本机实测（Apple M5 Max，负载 1.5–3.9，离线的 Go 实验 `scratchpad/m9/p2/exp/kmp`，KMP 的匹配器与 `Replace` 照本节写，不连库；每种取三次里最快的）：

| 形状（正文 5 MiB） | 答复 | 实测 |
|---|---|---|
| `a` 重复，`old` 是 k 个 `a` 加 `b`（k = 1、100、1 万、1 Mi − 1） | 没有 | 5.9–9.0 ms |
| `ab` 重复，`old` 是 512 Ki 个 `ab` 加 `ac` | 没有 | 7.6–7.7 ms |
| 随机的 `acgt`，`old` 是 64 字节（没有）或自己的 1 MiB 的一段（唯一） | 没有；替换 1 处 | 12.4–13.1 ms |
| **B 的构造串**（`review-B/rk`：k 个 `a` 改三个字节，对 `primeRK` 让每个窗的 Rabin–Karp 的哈希都撞），120,000 字节、1 MiB | 没有 | 7.1–7.4 ms、9.0–9.1 ms（`strings.Index`：120 KB 9.85 秒，1 MiB 70.2 秒，B 与它的复核实测） |
| B 的构造串在正文的两端各一次、中间每个窗都撞，3 万、12 万、1 MiB 字节，不带与带 `replace_all` | 找到 2 处；替换 2 处 | 11.5–16.2 ms（`strings.Index`、从 i+1 再找与 `strings.Count`：3 万字节就 3.01 秒） |
| **NFC 的再找**：正文是 5 MiB − 3 字节的 `a` 加 `e` 与 U+0301（不是 NFC），`old` 是 B 的构造串（12 万字节） | 没有 | 16.6–17.9 ms（NFC 的再找改用 `strings.Contains`：3 万字节 1.64 秒、12 万字节 8.90 秒；复核 B 实测） |
| **CRLF 的再找**：正文是一个 CRLF 加 5 MiB − 2 字节的 `a`，`old` 带一个单独的 LF，它的 CRLF 形式的哈希对 `primeRK` 解成与 m + 1 个 `a` 的相同（目标 84·b⁵ + 87·b⁴：CR 与 LF 在末尾往前第 5、4 个字节，`a` 减 CR 是 84、减 LF 是 87），12 万字节 | 没有 | 13.4–16.1 ms（CRLF 的再找改用 `strings.Index`：3 万字节 1.62 秒、12 万字节 9.00 秒；复核 B 实测） |
| 深的回退链（`a` 重复，`old` 是 1,024 段 1,023 个 `a` 加 `b`；每种形状每扫一个字节至多比两次，一处最深的回退链来自 Fibonacci 串，设计审查的复核 B 的 BC-N5） | 没有 | 7.7–8.2 ms |
| `a` 重复，`old` 是 `a`、`replace_all`（5,242,880 处） | 替换 | 73–74 ms |
| `a` 重复，`old` 是 `a`、`new` 是 1 MiB、`replace_all`（拼出来约 5 TiB） | 422，大小先算出 | 35.7 ms（计数的一遍） |
| `x\r\n` 重复，`old` 带单独的 `\n`：换成 CRLF 再找到（多处）；1 MiB 的单独 `\n` 的 `old`（没有） | 422；没有 | 30.2–30.5 ms；36.5–37.6 ms |
| `e` 加 U+0301 重复（不是 NFC），`old` 没有：NFC 的提示 | 没有 | 113–118 ms |
| U+1D160 重复（NFC 是它的三个字符，15 MiB），`old` 没有 | 没有 | 149–162 ms |
| 韩文的组合字母（NFC 合成）；顺序不对的组合符号（NFC 重排） | 没有 | 75–85 ms |
| 每一种再找都走一遍：CRLF 而不是 NFC 的页，1 MiB 的单独 `\n` 的 `old` 没有 | 没有 | 107 ms |

- **线性**：同一个匹配器在 1、2.5、5、20 MiB 的正文上、对 12 万与 1 MiB 的构造串，每字节 1.05–1.30 ns，随长度成正比。一次调用至多八遍扫描：查找、第二处、计数、拼出；CRLF 的检查与再找；NFC 的检查、规整与再找。每遍对至多 15 MiB 的文字（NFC 至多放大 3 倍）线性；模式的表是每字节一个 `int32`：`old` 的 4 MiB、CRLF 的至多 8 MiB、NFC 的至多 12 MiB，连同规整过的正文，NFC 的提示那一路暂时多约 35 MiB（另外两条路至多 8 MiB），在请求体之外。
- **期限：输入的上限让匹配不必看 ctx**。最坏的一次调用本机约 0.16 秒（复核 B 另量了表外的三种：U+1D160 的 CRLF 页上 1 MiB 的单独 LF 的 `old`，两种再找与 NFC 的三倍都走，158.5 ms；5 MiB 的一个组合符号 69 ms；30 个一段、类别递减的组合符号 86 ms），重做一次（3.8.2 第 5 步）约 0.33 秒；CI 的 runner 慢 2.5–3 倍时约 1 秒。调用的期限是读完请求体之后的 `server.request_timeout` 15 秒（00 号文档 4.3），留着 15 倍以上，所以匹配器里不查 ctx（每 N 个字节查一次反而让最常见的情形变慢）；`Replace` 之后的下一步 `parser.Decided` 与 `Run` 都带 ctx，期限到了在那里停下。匹配在单元之前，不持任何锁、不占连接；大于 64 KiB 的调用另受大的请求体的名额管（3.2）。
- 守卫的形状就是上表（B 的构造串在测试里以三个字节对 `primeRK` 解出，同 `review-B/rk`；CRLF 的那一种同样在测试里解出，照 `review-B/check/kmp/zz_b_test.go` 的 `craftedCRLF`），在 CI 上打出的值由实施者写进第 9 节。守卫先以 `go/parser` 读 `go env GOROOT` 之下的 `src/internal/bytealg/bytealg.go`，断言 `primeRK` 仍是 16777619（go1.27.1 的 `:33`）：换了它的工具链会让构造串悄悄变成普通的串，那时守卫失败、要重新解（复核 B 的可选的建议，采纳）。`Replace` 的任何一处（查找、计数、CRLF 与 NFC 的再找）若改用 `strings` 的查找，构造串的几种在守卫里就是几秒到几十秒，守卫失败（6.3 第 42、61、62 条）。解析在单元之前、有预算（13.1 第 19 条），不在这里算。

### 3.10 S1：mcpserver 的错误

#### 3.10.1 `Extra` 只补问题没有的成员

今天的 `toolError` 先抄 `Extra`，再写 `code`、`message`、`fields`、`lock`、`locks`、`retry_after_seconds`，同名的盖过 `Extra`（`mcpserver/errors.go:80-99`）。00 号文档 4.7 要 `lock` 带页的路径与 `you`（mcpserver 只看得到页 id、用户 id 与显示名），所以改为：先写问题的成员，再写 `Extra`：`Extra` 只能加问题没有的成员，另外可以盖过 `lock`、`locks` 与 `locks_count`（要模块写路径的只有锁的这三个）；`code`、`message`、`fields`、`retry_after_seconds` 总是 mcpserver 的，文字里也写着它们，结构与文字不能各说各的（设计审查 A-N5）。没有 `Extra` 的工具照旧。

#### 3.10.2 `locks` 的上限与 `ToolError.Fit`

```go
// MaxErrorLocks is how many locks an error's structuredContent lists at
// most (M9/P2 design 3.10): the rest are counted in locks_count.
const MaxErrorLocks = 20

// ToolError gains:

	// Fit, when set, builds the error with k of its N items: mcpserver
	// answers the largest k it finds whose whole answer is within
	// MaxAnswerBytes; Text and Extra are not read then.
	Fit *ErrorFit

// ErrorFit is an error of a list, or of parts that may not all fit (M9/P2
// design 3.10): Build(k) gives the text and the extra members with k of
// the N items. mcpserver searches k as Fit does; Build(0) over the bound is
// internal_error.
type ErrorFit struct {
	N     int
	Build func(k int) (text string, extra map[string]any)
}

// ErrorAnswerBytes is the code mcpserver answers err with, a tool's
// error, and the size of that answer (M9/P2 design 3.10.5): err takes
// the path of a tool's error, Fit's search and the members' writing
// included, so a module's test measures its errors as they are answered.
// When even k = 0 is over the bound, code is internal_error and bytes is
// the size of the answer that went over, which the over-bound line logs:
// the internal_error answer itself carries the call's request id, which
// err does not have.
func ErrorAnswerBytes(err error) (code string, bytes int)
```

- 问题的 `locks`（`linking.pages_locked`、MCP 的删除的 `page.locked`）由 mcpserver 只写前 `MaxErrorLocks` 项，并写 `locks_count`（总数）；有 `Extra` 盖过 `locks` 的，`locks_count` 也由它给。今天 `linking.pages_locked` 列出全部被编辑的页（`linking/domain/errors.go:22-28`），一项至多约 750 字节（显示名至多 100 个字符，`identity/domain/user.go:33`，最贵的字符 6 字节；两个 id 与键名约 150），约 65 项就过了 48 KiB、答 `internal_error`（设计审查 A 实测：70 项 51,348 字节；P1 没有 `move`，所以 P1 没有这个 bug）。
- `Fit` 与错误共用一个找 k 的函数（`answer.go:127-175` 的倍增与二分抽出来）：量是整个错误的答复，同 `errorResult`（文字块的 `TextBytes` 加 `structuredContent` 的 JSON 的 `TextBytes`，`answer.go:101-112`）。k 不一定是最大的（同 `Fit`）。
- `ErrorAnswerBytes` 是错误的答复那一条路导出的入口（设计审查的复核 A 的 NN-5）：模块的测试把适配器答的错误（`toolErr(err, call)`）交给它，得到的就是 mcpserver 真的答出的码与字节，不在适配器里另抄一份量法；mcpserver 的 `TestErrorAnswerBytesIsTheAnswer` 核对它与真的调用答出的相同。过了界的错误答 `internal_error`，字节是过了界的那个答复的大小（`overBound` 记的 `bytes`，`mcpserver/errors.go:195-201`），不是 `internal_error` 的答复的：那个带着调用的请求 id（在文字里，`message` 是文字的一份，另有 `request_id`，`errors.go:150-160`；请求 id 可以由调用者给，1 到 128 个字符，`httpserver/middleware.go:81-95`），`err` 里没有（设计审查的第二次复核 A 的 NN3-1）。
- P3 的 `batch` 的错误（出错的 `op`、它的锁）与 P4 用得上同一个。

#### 3.10.3 每个错误的答复在 48 KiB 之内：计算

一个路径至多 2,818 字节（11 段、每段 255 字节、10 个 `/`、`.md`）；全是 `&`（标题里能有，`shared/title.go:64`）时文字块里 6 字节一个，`MemberBytes` 也是 6（`answer.go:25-96`）：P = 2,805 × 6 + 13 = 16,843 字节。路径以 `"` 原样引起（3.11.5），不比 `%q` 长（`%q` 只会把字符写成更长的转义），`&` 仍是最贵的字节。一个显示名至多 600 字节。错误的答复是文字块加 `structuredContent`，`message` 是同一段文字，所以文字里的每个字节在答复里至少数两次：

| 答复 | 最坏 | 结论 |
|---|---|---|
| 文字引一个路径、一个名字，结构里没有路径 | 2 × (P + 600 + 300) ≈ 35.5 KB | 放得下 |
| 再加 `lock.path` | 约 52.4 KB | 过了：`lock.path` 经 `Fit`（N = 1），放不下时省去 |
| 文字引两个路径（P1 的 `page.not_found` 加提示里的路径） | 2 × 2P ≈ 67.4 KB | 过了：今天在 `main` 上答 `internal_error`（A 实测 `AttachmentHint` 67,590、`PageHint` 61,448）；提示经 `Fit`（N = 1），放不下时省去，33,818（2.1 第 4 条） |
| 文字引一个路径与三个名字，`locks` 里 20 项各带路径 | 远过 | `locks` 经 `Fit`，k 项各是完整的一项；`Build(0)` 是空的 `locks` 与 `locks_count`，约 2 × (P + 1,800 + 300) ≈ 38 KB |

所以规则是：**文字只引这次调用自己的一个路径**（`write`、`edit`、`delete` 的 `path`，`move` 按错误引 `from` 或 `to`）与至多三个持锁人的名字、计数与版本令牌；**别的页的路径只在结构里**，经 `Fit`；**字段的消息不写路径与取值**（字段的消息在文字与 `fields` 里各一份）。各个错误的文字见 3.11.5。

#### 3.10.4 page 的 `adapter/mcp`：错误的写法

`toolErr(err error, c mcpserver.Call) error`（`page/adapter/mcp/text.go`，今天只认 `PathNotFound`、以 `%q` 引路径）按错误的类型写，路径一律以 `"` 加路径加 `"` 写出（`domain.Quoted(path)`，在 `page/domain/path.go` 挨着提示；P1 的 `Nothing is at %q`（`text.go:22`）与四个提示（`PageHint`、`AttachmentHint`、`TitleHint`、`ChildrenHint`，`path.go:123-147`）同样改，设计审查 A-M2：`%q` 把标题里能有的 U+3000、U+00A0、U+200D、U+00AD 与私用区的字符写成 `\`、`u` 加四位的转义，文字就不再是路径）：

- `*domain.PathNotFound`：`Fit{N: 1}`：k = 1 带提示，k = 0 不带（P1 的工具同样经它，修正 2.1 第 4 条的那个超过；`list` 的 `ChildrenHint` 在 `PathNotFound` 里时同样）。
- `*domain.PathTaken`、`*domain.VersionMismatch`、`*domain.Recursive`：`Text`（3.11.5 的文字）；`VersionMismatch` 另有 `Extra` 的 `current_version`，`Recursive` 的计数写进文字。
- `*domain.LockPaths` 包着的 `page.locked`：只有一个锁（`write`、`edit`）时 `Fit{N: 1}`，`lock` 是 `{page_id, path, user_id, display_name, you}`，k = 0 时没有 `path`；MCP 的删除（`Locks` 多于零）时 `Fit{N: min(len(Locks), MaxErrorLocks)}`，`locks` 是前 k 项、各带 `path`（读不到的为 null），`lock` 是第一项而不带 `path`（它的路径在 `locks` 里，设计审查 A-N4），`locks_count` 是 `len(Locks)`。
- `*domain.LockPaths` 包着的 `linking.pages_locked`：同删除的列表，没有 `lock`。
- `you` 是 `holder.UserID == c.UserID`。
- 别的错误照旧交给 mcpserver（`code: detail (fields)`）。

#### 3.10.5 测试

mcpserver（`errors_test.go`）：

| 测试 | 断言 |
|---|---|
| `TestExtraOverridesTheProblemsMembers` | 带 `Lock` 的问题加 `Extra{"lock": 带 path 的, "extra": 1}`：答复的 `lock` 是 `Extra` 的、多了 `extra`；`Extra` 里的 `code`、`message`、`fields`、`retry_after_seconds` 不生效 |
| `TestTheLocksOfAnErrorAreBounded` | 25 个 `Locks`：`locks` 20 项、`locks_count` 25；3 个：3 项、`locks_count` 3 |
| `TestAnErrorFitsItsItems` | `Fit{N: 30}`、每项 4 KB：答复在 48 KiB 之内、k 项都在、k + 1 项放不下（或 k = N）；`Build(0)` 就过了：`internal_error`、error 日志 "MCP answer over its bound"；`Build` panic：`internal_error`（照 `recoverTool`） |
| `TestErrorAnswerBytesIsTheAnswer` | 带 `Fit` 的、带 `Extra` 的几种错误：`ErrorAnswerBytes` 的码与字节等于经 `New` 起的服务的一次真的工具调用答出的 `result` 的；过界的一种只比码（`internal_error`），字节等于那次调用的 error 一行"MCP answer over its bound"记的 `bytes`（真的答复带着调用的请求 id，比不了字节；第二次复核 A 的 NN3-1） |
| `TestAnErrorOfTheLongestPathFits` | 全是 `&` 的 2,818 字节的路径：带提示的 `page.not_found` 形状、带 `lock.path` 的 `page.locked`、20 项的 `locks`：都答出各自的码、都在 48 KiB 之内（今天第一种答 `internal_error`；`page.locked` 今天根本带不了 `lock.path`） |

page 的 `adapter/mcp`（`text_test.go`）：每种错误的文字与结构（3.11.5 的表），"是你"与不是，路径读不到时为 null，引号之内恰是路径（U+3000、U+00A0、U+200D、U+00AD 的标题原样）；`MaxNamedLocks == mcpserver.MaxErrorLocks`、`UnknownClientName == mcpserver.UnknownClient`。**每个错误在最坏的取值处量**（`TestEachErrorFitsAtItsWorst`，设计审查 A-M6）：3.11.5 的表的每一行以最坏的取值建出（2,818 字节的 `&` 的路径；25 个锁，显示名各 100 个 `&`，页的路径各是 10 层的 `&`，2,562 字节、`TextBytes` 15,312），经 `mcpserver.ErrorAnswerBytes` 量（3.10.2：真的答复那一条路，`Fit` 的找法与成员的写法都在里面）：码是表里的，都在 48 KiB 之内。设计审查 A 照本节的文字量过（`review-A/sizes-output.txt`）：`page.not_found` 的 `Build(0)` 33,818，三个名字的删除的 `page.locked` 38,534，`linking.pages_locked` 37,922，`page.revision_mismatch` 34,224，`move` 的 `page.title_taken` 34,022，`validation_failed`（`recursive`）34,161。

### 3.11 S1：四个写的工具

四个都在两个入口（`BothEntries`）、`Notebook: true`（账户入口另带 `notebook`，`mcpserver/tool.go:33-41`），处理函数调各自的用例、以 `domain.MCPClient(c.Client)` 交客户端、错误经 `toolErr(err, c)`（3.10.4）。输入的 Go 类型照 P1 的写法（必填的字段没有 `omitempty`；可选的是指针，`null` 当作没有，`mcpserver/schema.go:21-145`）；`jsonschema` 的标签就是 `tools.json` 的参数说明（3.18）。说明与标题都是英文（13.5 第 4 条）。

| 工具 | 标题 | 注解 | 用例 |
|---|---|---|---|
| `write` | Write a page | `{Destructive: true, Idempotent: true}` | `WritePath` |
| `edit` | Edit a page | `{Destructive: true}` | `EditPage` |
| `move` | Move or rename a page or an attachment | `{Destructive: true, Idempotent: true}` | `RelocatePath` |
| `delete` | Delete a page or an attachment | `{Destructive: true, Idempotent: true}` | `DeletePath` |

注解照 00 号文档第 5 节：写的都是 `destructiveHint: true`；`write` 以同样的参数再调一次答"未改动"（新建时也是：路径已是那一页、内容相同），`move`、`delete` 第二次找不到源、答 404，没有别的改动，所以 `idempotentHint: true`；`edit` 第二次在新的正文上可能再替换一处（`replace_all`）或答 422，所以不是。`readOnlyHint` 由 SDK 照样写出为假，`openWorldHint` 都是假（`mcpserver/tool.go:150-156`）。矩阵的覆盖检查要这四个的行带 `write`（`permission_matrix_mcp_test.go:187-232`），它们的 `after` 照 `check` 计数（3.13.2）。

#### 3.11.1 `write`

```go
type writeIn struct {
	Path        string  `json:"path" jsonschema:"…"`
	Content     string  `json:"content" jsonschema:"…"`
	BaseVersion *string `json:"base_version,omitempty" jsonschema:"…"`
}

// writeAnswer is write's answer (M9/P2 design 2.1 item 11): created, the
// nodes the write created from the root down, its page last, fitted;
// created_count all of them; changeset_id and at null when nothing changed.
type writeAnswer struct {
	Path         string        `json:"path"`
	NodeID       string        `json:"node_id"`
	Version      string        `json:"version"`
	Changed      bool          `json:"changed"`
	Created      []createdNode `json:"created"`
	CreatedCount int           `json:"created_count"`
	ChangesetID  *string       `json:"changeset_id"`
	At           *time.Time    `json:"at"`
}

type createdNode struct {
	Path   string `json:"path"`
	NodeID string `json:"node_id"`
}
```

答复经 `mcpserver.Fit(len(Created), build)`：一项至多一个路径（16,843 字节），十项放不下时少放、`created_count` 照旧是总数；`build(0)` 只有这一页的路径，放得下。`at` 是单元的时刻（UTC，微秒，`platform/clock/clock.go:14`）：e2e 拿它与库里的时刻在 SQL 里比较（13.1 第 9 条），MC3 拿它作新页的 `created_at`（4.2）。

#### 3.11.2 `edit`

```go
type editIn struct {
	Path        string  `json:"path" jsonschema:"…"`
	OldString   string  `json:"old_string" jsonschema:"…"`
	NewString   string  `json:"new_string" jsonschema:"…"`
	ReplaceAll  *bool   `json:"replace_all,omitempty" jsonschema:"…"`
	BaseVersion *string `json:"base_version,omitempty" jsonschema:"…"`
}

type editAnswer struct {
	Path         string     `json:"path"`
	NodeID       string     `json:"node_id"`
	Version      string     `json:"version"`
	Replacements int        `json:"replacements"`
	Changed      bool       `json:"changed"`
	ChangesetID  *string    `json:"changeset_id"`
	At           *time.Time `json:"at"`
}
```

`changed` 为假有两种：CRLF 的规则把两个串变成同一个（3.9），或读与单元之间别人的写恰好把这一页写成了替换的结果（3.8.2 第 6 步，设计审查 B-N3）；两种都没有新的变更集。00 号文档 4.10 的答复照列，多这一个成员。

#### 3.11.3 `move`

```go
type moveIn struct {
	From        string `json:"from" jsonschema:"…"`
	To          string `json:"to" jsonschema:"…"`
	UpdateLinks *bool  `json:"update_links,omitempty" jsonschema:"…"`
}

type moveAnswer struct {
	Path        string     `json:"path"`
	NodeID      string     `json:"node_id"`
	Changed     bool       `json:"changed"`
	ChangesetID *string    `json:"changeset_id"`
	At          *time.Time `json:"at"`
}
```

`update_links` 缺省（或 `null`）为真。附件改名要带一个扩展名，扩展名可以换（`flow.png → flow.jpg` 成，`flow.png → flow` 答 422，`CheckAssetRename`，`page/domain/name.go:29-41`）。

#### 3.11.4 `delete`

```go
type deleteIn struct {
	Path      string `json:"path" jsonschema:"…"`
	Recursive *bool  `json:"recursive,omitempty" jsonschema:"…"`
}

type deleteAnswer struct {
	Path        string    `json:"path"`
	NodeID      string    `json:"node_id"`
	Pages       int       `json:"pages"`
	Attachments int       `json:"attachments"`
	ChangesetID string    `json:"changeset_id"`
	At          time.Time `json:"at"`
}
```

`pages`、`attachments` 是进了回收站的，被删的节点自己也算（删一个附件是 0 与 1）。删除总写了东西，`changeset_id` 不为空。

#### 3.11.5 错误的文字与结构

文字只引这次调用自己的一个路径（3.10.3），以 `"` 加路径加 `"` 原样写出（`domain.Quoted`，3.10.4）：标题里没有 `"` 与 `\`（`shared/title.go:64`），所以引号之内恰是路径的字节，agent 照抄就是路径（设计审查 A-M2；P1 的 `Nothing is at %q` 与提示一起改）。没有写在这里的码照 mcpserver 的 `code: detail (fields)`，例如 `page.too_deep: Pages nest at most 10 levels deep.`、`page.cycle: A page cannot move under itself or one of its subpages.`（`page/domain/errors.go:27-30`）、`server_busy: The server is busy; retry shortly. Try again in 1 s.`。

| 码 | 什么时候 | 文字（`content` 与 `message`） | 结构的另外的成员 |
|---|---|---|---|
| `page.not_found` | 路径不在（四个工具） | P1 的：`page.not_found: Nothing is at "A/B.md".` 加提示，提示放不下时省去（`Fit{N: 1}`） | — |
| `page.not_found` | `move` 的 `to` 的父页不在 | `page.not_found: Nothing is at "B.md", to's parent. move creates no parent pages: write the parent first, or move to an existing page's path.` | — |
| `validation_failed` | `delete` 有子节点而没有 `recursive` | `validation_failed: "A.md" has 3 pages and 2 attachments under it: set recursive to true to put them in the trash with it.` | `fields`：`recursive` `required` "must be true to delete a page with children: 3 pages and 2 attachments" |
| `page.title_taken` | `write` 新建而路径已是一页（内容不同） | `page.title_taken: "A/B.md" is a page already. To replace its content, write it with base_version, the version read gave.` | — |
| `page.title_taken` | `write` 新建而一个附件占着这个名称 | `page.title_taken: An attachment holds the name of "A/x.png.md" there: a page there needs another title.` | — |
| `page.title_taken` | `move` 的 `to` 被别的节点占着 | `page.title_taken: "B/x.md" is taken: a page there has that title. to is the full new path of what moves, never a page to move it into.`（附件：`an attachment there has that name`） | — |
| `page.revision_mismatch` | 令牌的节点不是路径的节点 | `page.revision_mismatch: "A/B.md" is another page now than the one base_version was read from: read it, and write from what you read. Its version is "<当前的令牌>".` | `current_version` |
| `page.revision_mismatch` | 令牌的版本号过时 | `page.revision_mismatch: "A/B.md" changed since base_version: read it again, and write from what you read. Its version is "<当前的令牌>".` | `current_version` |
| `page.revision_mismatch` | `edit` 的重做之后仍不符 | `page.revision_mismatch: "A/B.md" changed twice while the edit was made: read it again, and edit what you read. Its version is "<当前的令牌>".` | `current_version` |
| `page.locked` | `write`、`edit` 碰上别人的编辑器（`edit` 的预检与单元里的守卫同样） | `page.locked: "A/B.md" is being edited by Alice in the web editor; try again after they leave edit mode. The agent cannot take the lock over.` | `lock`：`{page_id, path, user_id, display_name, you: false}`（`Fit{N: 1}`：`path` 放不下时省去；路径读不到时为 null） |
| `page.locked` | 同上，持锁人是调用者本人 | `page.locked: "A/B.md" is being edited by you, in a browser; try again after you leave edit mode. The agent cannot take the lock over.` | `lock` 同上，`you: true` |
| `page.locked` | MCP 的删除碰上子树里的编辑器 | `page.locked: "A.md" has 2 pages being edited in the web editor, by Alice and by you, in a browser: deleting it would end their edits. Try again after they leave edit mode; the agent cannot take the locks over.` | `lock`（第一项，不带 `path`：它的路径在 `locks` 里，设计审查 A-N4）、`locks`（至多 20 项，各带 `path`，读不到的为 null，子树的次序）、`locks_count` |
| `linking.pages_locked` | `move` 要改写的页被编辑（本人的也算） | `linking.pages_locked: Moving "A.md" would rewrite links in 3 pages being edited in the web editor, by Alice, Bob and you, in a browser. Try again after they leave edit mode, or move with update_links false to leave their links as they are; the agent cannot take the locks over.` | `locks`、`locks_count`（同上；次序是页 id 的，`LockHolders.Of`，`page/app/lock_holders.go:27-51`） |

- 名字：同一个人只说一次，按锁的次序，至多三个，多的写"and N others"；本人写"you, in a browser"（00 号文档 4.7）。不说"自动重试"：锁跟着编辑器开着，关掉浏览器之后至多 120 秒才过期（`page/domain/session.go:14`）。
- 一页时写"page"、一个时写"attachment"，单复数照数目；数目从 1 起。
- `page.locked` 的 `locks` 与 `locks_count` 只在 MCP 的删除里出现；`write`、`edit` 的只有 `lock`（`Locks` 为空，3.6）。
- 结构里的 `code`、`message`、`fields`、`retry_after_seconds` 总是 mcpserver 写的，与文字一致（3.10.1）。

### 3.12 S1、S2：内置的说明

两个入口共用的 `pathsAndTools` 的"Tools"一段（`page/adapter/mcp/instructions.go:41-48`）改为下面的样子（`list`、`read` 两行不变）；S1 加 `write`、`edit`、`move`、`delete` 与最后三行，S2 加 `versions`、`log`、`changeset`。每个工具一行，以 `- <名>:` 起头（`TestTheBuiltInInstructionsNameEveryTool`，`bootstrap/mcp_tools_test.go:189-205`）；不提 `batch`（2.1 第 12 条）。

```text
Tools
- list: …（不变）
- read: …（不变）
- write: a page's whole content. Without base_version it creates the page and its missing parent pages; with the version you read, it replaces the content.
- edit: replaces old_string, which must occur exactly once unless replace_all is set, with new_string, byte for byte.
- move: moves or renames a page or an attachment to its full new path ("B/x.md", never a page to move it into); links to it follow unless update_links is false.
- delete: puts a page or an attachment in the trash; a page with children needs recursive.
- versions: a page's versions, or with revision one version's lines.
- log: the notebook's changes, newest first; path, actor ("me") and client ("mcp") narrow it.
- changeset: one change in full. History is read-only here: people restore and revert in the web.
- A page's version, "<node id>:<revision>", tells which content you read. A write or edit with base_version is refused when the page changed since: read it again.
- A page being edited in the web editor, by you too, refuses writes, edits, deletes and a move that would rewrite its links, saying by whom: try again after they leave edit mode. The lock cannot be taken over.
- Calls over 64 KiB are taken one at a time per token, the next waiting 2 seconds at most: send large writes one after another, not in parallel.
```

锁的一行写上 `move`：要改写的页被编辑时它答 `linking.pages_locked`，除非 `update_links` 为假（设计审查 A-N7）。量（`TextBytes`，`page/adapter/mcp/instructions_test.go:15-27` 的量；以 `scratchpad/m9/p2/notes/work/measure.py` 照 `instructions.go` 的拼接算，算法同 `mcpserver/answer.go:25-96`；文本是 `exp/instr/s1-r2.go.txt`、`s2-r2.go.txt`）：

| 入口 | P1 照建成的 | S1 之后 | S2 之后 | 4 KiB 之下还剩 |
|---|---|---|---|---|
| 笔记本 | 1,715 | 2,693 | 2,961 | 1,135 |
| 账户 | 1,785 | 2,763 | 3,031 | 1,065 |

最后一行是设计审查的复核 A 的 MN-1（主持者让它可选：放得下、而且与客户端给 agent 看到的相符才加）。放得下：它加 146 字节（`TextBytes`），上表是加了它之后的量，P3、P4 的五行（下一段）在账户入口剩下的 1,065 字节里照旧放得下。相符：它只说服务的行为与做法，不说 agent 会看到什么。`/mcp` 的 503 是 HTTP 的错误，未必以文字到达模型：Go 的 SDK v1.8.0 的客户端把 503 当作暂时的错误，只答"Service Unavailable"、不读答复的正文（`sdk/mcp/streamable.go:2583-2587`、`:2821-2833`），`server_busy` 与 `Retry-After` 都到不了模型；Claude Code 与 Codex 怎样表现离线核对不了，由 P5 的人工清单看（第 8 节）。所以在说明里事先说：说明在握手时就交给客户端，不靠 503 的文字。

P3 的 `batch` 一行（00 号文档"移交的落实"的 M8/P3 一行）与 P4 的 `grep`、`search`、`backlinks`、`index` 四行，每行约 100–200 字节，放得下；超过时 `TestTheBuiltInInstructionsAreShort` 失败，由加行的 Phase 收短。实施时以测试的 `TextBytes` 复核这几个数（照实际的字句，差几个字节不改结论）。

### 3.13 S1：码的表、矩阵与整个程序的测试

#### 3.13.1 码的表与工具的字符串

`mcpToolCodes()`（`bootstrap/permission_matrix_mcp_test.go:71-78`）加四行；每个码契约都声明过（`createPage`、`putPageContent`、`renameNode`、`deleteNode`、`moveNode` 的 `x-problem-codes`，`api/modules/page.yaml:56`、`:146-157`、`:572-573`、`:605`、`:641-651`），`TestEveryMCPToolHasItsCodes` 照常核对：

```go
"write":  {"bad_request", "validation_failed", "notebook.not_found", "page.not_found", "forbidden", "server_busy",
	"page.title_taken", "page.too_deep", "page.revision_mismatch", "page.locked"},
"edit":   {"bad_request", "validation_failed", "notebook.not_found", "page.not_found", "forbidden", "server_busy",
	"page.revision_mismatch", "page.locked"},
"move":   {"bad_request", "validation_failed", "notebook.not_found", "page.not_found", "forbidden", "server_busy",
	"page.title_taken", "page.cycle", "page.too_deep", "linking.pages_locked"},
"delete": {"bad_request", "validation_failed", "notebook.not_found", "page.not_found", "forbidden", "page.locked"},
```

`TestTheMCPToolsErrorsCarryTheirCodes`（`bootstrap/mcp_reads_test.go:401-451`）的表加每个码的调用（`answered` 要每行的每个码都答出过）。读的应用（`startReadsApp`）另注册 carol、以 SQL 加进 alice 的工作区（同 `acmeTeam.join`，`interleavings_workspace_test.go:108-118`）、alice 建一本 `workspace_access` 为 reader 的笔记本，carol 的 PAT 答 `forbidden` 的格；`server_busy` 的格在同一个测试里另起一个应用（`ParseBudgetBytes` 64 KiB、`ParseMaxWait` 300 ms，同 `TestAWriteWaitingForItsLockKeepsItsFactsShare`，`page_content_test.go:302-340`），预算由一个等在正文行锁上的 REST 写持着（`writeWaitingForItsLock`，`:342-379`）：

| 工具 | 码 | 调用 |
|---|---|---|
| `write` | `bad_request` | `path: 7` |
| | `validation_failed` | `path: "Guide"`（附件的路径）；`a/./b.md`；带 NUL 的正文；`base_version: "nope"` |
| | `notebook.not_found` | 账户入口里 bob 的笔记本 |
| | `page.not_found` | 带 `base_version` 而 `Missing.md` |
| | `forbidden` | carol 写 reader 的笔记本 |
| | `server_busy` | 预算被持着时 100 KiB 的正文 |
| | `page.title_taken` | 不带 `base_version` 写已有的 `Guide.md`、内容不同 |
| | `page.too_deep` | 11 段的页路径 |
| | `page.revision_mismatch` | `base_version` 的版本号是 99 |
| | `page.locked` | alice 以 REST 开了 `Guide` 的会话之后，带 `base_version` 写它 |
| `edit` | `bad_request`、`validation_failed` | `old_string: 7`；`old_string: ""` |
| | `notebook.not_found`、`page.not_found`、`forbidden`、`server_busy` | 同 `write` |
| | `page.revision_mismatch`、`page.locked` | 过时的 `base_version`；会话 |
| `move` | `bad_request`、`validation_failed` | 没有 `to`；`to` 的类型不同 |
| | `notebook.not_found`、`page.not_found`、`forbidden` | 同上 |
| | `server_busy` | 预算被持着时移动一个有反链的页（改写的 `TakeNow`），**在另一本笔记本里**：持预算的 REST 写持着它那一本的笔记本行 `FOR SHARE`（`page/app/unit.go:200-206`），同一本里的 `move` 要 `FOR NO KEY UPDATE`，会一直等到调用的期限；预算是整个进程的（设计审查 B-M4） |
| | `page.title_taken`、`page.cycle`、`page.too_deep` | `to` 已有页；`Guide.md` 移到 `Guide/Sub/Guide.md`；两层的子树移到第 9 层之下 |
| | `linking.pages_locked` | 链到它的页开着会话 |
| `delete` | `bad_request`、`validation_failed` | `recursive: "yes"`；有子节点而没有 `recursive` |
| | `notebook.not_found`、`page.not_found`、`forbidden`、`page.locked` | 同上；会话 |

`mcpAnswer.toolError` 照旧拿每个工具错误对照它的工具的行（`mcp_client_test.go:293-301`）。

**工具的字符串**（设计审查 C-I1、A-M1）：今天的 `TestToolStringsDoNotAnswerInternalErrors`（`mcp_reads_test.go:562-606`）以 `argumentsAt(param, "a\x00b")` 造只有这一个参数的参数（`:530-542`），只断言不是 `internal_error`；缺了必填的成员是结构表的 `bad_request`（`mcpserver/tool.go:177-181`、`errors.go:117-131`），它也过。所以 P2 的 18 个新的字符串参数里有 11 个（`write` 的三个、`edit` 的四个、`move` 的两个、`versions` 与 `changeset` 的 `cursor`）的 NUL 从没到过用例：一个漏了 `CheckEdit` 的 NUL 规则的 `EditPage` 把 `new_string` 的 NUL 交给 PostgreSQL（22021，`internal_error`），测试照样过。S1 把它改成：

- **合法的参数的表**：`validToolArguments(r readsApp) map[string]map[string]any`，每个工具一项，是能到达它的用例的一组参数（测试先以 REST 建 `Strings.md`，正文 `x`）：`notebooks {}`、`open_notebook {}`（只在账户入口，P1 的两个：没有它们，S1 的第 6 个提交里检查就是红的，设计审查的复核 C 的 C-NM1）、`list {}`、`read {path: "Strings.md"}`、`write {path: "Strings.md", content: "x"}`、`edit {path: "Strings.md", old_string: "x", new_string: "y"}`、`move {from: "Strings.md", to: "Moved.md"}`、`delete {path: "Gone.md"}`；S2 加 `versions {path: "Strings.md"}`、`log {}`、`changeset {id: <建 Strings.md 的变更集，以 SQL 读>}`。`tools/list` 里有而表里没有的工具让测试失败：P3、P4 的工具必须加上自己的一项。
- **只换一个参数**：对 `tools/list` 推导出的每个字符串参数（`stringParameters`），在这个工具的合法参数的一份深拷贝里只把这一个换成 `a\x00b`（`withArgumentAt(valid, param, value)`：`ops[].path` 换的是第一个 `op` 的 `path`，没有就加一个）。账户入口的 `notebook` 在换之前就放进合法的参数：换的若正是 `notebook`（`open_notebook` 与账户入口的每个工具都有它），它就是那个 NUL；在换之后才加会盖掉它，那一项就什么也不测了（今天的测试在 `:581-583` 只在没有时才加）。每一项发出之前断言最后的参数在点名的参数处恰是 `a\x00b`（`ops[].path` 看第一个 `op`），所以一项不会悄悄地不测（复核 C 的 C-NM1，主持者的决定）。
- **到达了用例**：答复不是 `internal_error`，也不是结构表的拒绝（文字以 `bad_request: The arguments do not match the tool's input schema.` 起头，`mcpserver/errors.go:126`）。用例自己答的 400（坏的游标）与 422 都算到达。不成对的代理项照旧是边缘的 400（整个请求体，不论别的成员）。
- **反向对照**：`TestToolStringsCatchASchemaRefusal` 起一个带接缝工具 `strict` 的应用（`wireMCPApp(t, cfg, logs, tools)`，`mcp_client_test.go:84`；它的参数要 `must` 与 `s` 两个字符串），给检查的函数一张 `strict` 的一项缺 `must` 的表：检查报出"`strict` 的 `s` 没有到达用例"；给一张没有 `strict` 的表：检查报出"没有 `strict` 的合法参数"。检查的函数返回它找到的问题（`toolStringProblems(...) []string`），主测试逐条 `t.Error`，反向对照断言整张问题的表恰是期望的那几条、不多不少：只看期望的一条在不在的话，变异 50（改回只放这一个参数）之下它还过，因为缺 `must` 的那一项另多报一条（设计审查的复核 A 的 NN-1、C 的 N1）。结构表的拒绝的文字若改了，反向对照就失败，所以测试里的这段文字不会悄悄失效。
- 每个入口、每个参数一次调用：P2 之后约 2 × 25 次，加反向对照的一个应用。6.3 的变异"`EditPage` 不查 `new_string` 就写"由它抓住。

#### 3.13.2 矩阵的四行

`mcpRow` 加一个字段（REST 的 `matrixRow` 有同样的，`permission_matrix_test.go:158-176`）：

```go
	// after, when set, runs on each answer that is a success, on the copy
	// of the database the cell wrote: call is a call as the cell's caller,
	// through the cell's entry, to read back what it wrote.
	after func(t *testing.T, c caller, s seeded, call func(tool string, args map[string]any) mcpAnswer)
```

`after` 照 `check` 计数（设计审查 A-M7、C-N1）：runner 在调用之前按 `want` 计，格应答成功（`want` 是零值）而行有 `after` 时 `toAfter.Add(1)`，同今天的 `toCheck`（`permission_matrix_mcp_test.go:385-387`）；调过 `after` 之后 `aftered.Add(1)`（同 `checked`，`:413-416`）；`t.Cleanup` 里两者不等就失败（`:367-372`）。按 `want` 而不按调用的结果计：去掉调用之后整块的 runner 才计不到、失败（设计审查的复核 A 的 NN-2）；不调 `after` 的 runner 同样失败。REST 的 `after` 同样计数（`permission_matrix_test.go:279-283` 今天不计）。

`writers` 是 `editorsOnly` 的 MCP 版本（`permission_matrix_page_test.go:609-622`）：管理员与编辑者答结果、看得到的读者答 `forbidden`、看不到的 `notebook.not_found`（笔记本入口由 runner 换成 HTTP 404，`permission_matrix_mcp_test.go:382-384`）。四行都是 `BothEntries`、`write: true`：

| 工具 | 参数 | `check` | `after` |
|---|---|---|---|
| `write` | `{path: "mcp-written.md", content: "quillwort, written\n"}` | `changed`、`created_count` 1、路径 | `read` 它：正文相同、版本号 1 |
| `edit` | `{path: pagePathOf(c), old_string: "quillwort", new_string: "quillwort, edited"}` | `replacements` 1、版本是这一页的 `:2` | `read`：`quillwort, edited\n` |
| `move` | `{from: pagePathOf(c), to: 同一个父页下的 "<pageOf(c)> moved.md"}` | 新的路径、`node_id` 是这一页的 | `read` 新的路径、旧的路径 `page.not_found` |
| `delete` | `{path: pagePathOf(c), recursive: true}` | `pages` 1、`node_id` | `read` 旧的路径 `page.not_found` |

列的页（`pageOf(c)`）没有种下的会话与链接（种下的会话在各自的草稿页上，`permission_matrix_seeded_test.go:249-258`；正文只有 `quillwort`，`:191-200`），所以写者的格都成功。写的格各在库的副本上跑、各起一个应用（`permission_matrix_mcp_test.go:389-393`，至多 `matrixApps` = 8 个同时）：4 行 × 2 个入口 × 13 列 = 104 格。

#### 3.13.3 最后一跳与锁的每一格

**最后一跳**（M4/P1 的移交第 3 项，13.1 第 21 条）：`TestTheClientNameIsTheTokens`（`bootstrap/mcp_names_test.go:47-88`）的每一步在 `echo` 之后各以 `write` 建一页，断言 `SELECT client FROM changesets WHERE id = <答复的 changeset_id>`：

| 步 | 情形 | 期望的 `client` |
|---|---|---|
| 2 | 握手报 `  Claude Code\t` 之后的调用（不带 `_meta`） | `mcp:Claude Code` |
| 3 | 同一个库上的第二个应用，没有握手（记下的名称） | `mcp:Claude Code` |
| 4 | 重启的第三个应用，握手报同一个名称（`Record` 不写） | `mcp:Claude Code` |
| 5 | 带 `_meta` 的 `Codex`，然后不带 | `mcp:Codex`、`mcp:Codex` |
| 6 | 握手报 U+202E 加 200 个 `n` | `mcp:` 加 128 个 `n` |

第 1 步（什么都没报）另写一页：`mcp:unknown`。页的适配器不交 `MCPClient`（例如交 `ClientAPI`）、名称不记在 PAT 上时这些断言失败（6.3 的变异表）。`Call.Client` 到达时已经清理过（`names.go:51`、`mcpserver/tool.go:72`），所以 `MCPClient` 自己的再清理只由 `client_test.go` 的性质测试守着，第 6 步不守它（6.3 第 10 条，设计审查 C-M10）。`clientOfEcho` 推广为 `callAs(t, tool, args, told)`，带 `_meta` 的写同它。MCP 不开会话，`edit_sessions.client` 不涉及（00 号文档 4.5）。

**锁的每一格**（00 号文档 4.10 的表、第 9 节"锁：4.10 的表的每一格经 MCP"；照 `TestTheEditLockReachesEveryPath`，`bootstrap/page_lock_test.go:20-64`）：`bootstrap/mcp_lock_test.go` 的 `TestTheEditLockReachesEveryMCPWrite`，`newAcmeTeam(t, "member", "")`，alice 与 bob 各以 `createToken` 建 PAT（`contract_test.go:311-333`），笔记本 Eng、页 Beside 与 Notes，bob 以 REST 开 Notes 的会话：

| # | 谁 | 调用 | 期望 |
|---|---|---|---|
| 1 | alice | `write` Notes（带 `base_version`） | `page.locked`，`lock` 是 Notes、`path: "Notes.md"`、bob、`you: false`；文字说"by bob in the web editor" |
| 2 | alice | `edit` Notes | `page.locked`，同上（锁的预检的拒绝，带路径，3.8.2 第 4.6 步） |
| 3 | alice | `delete` Notes | `page.locked`，`locks` 是 Notes（带路径）、`locks_count` 1，`lock` 不带 `path` |
| 4 | bob | `write` Notes | `page.locked`，`you: true`，文字说"by you, in a browser" |
| 5 | bob | `edit` Notes | `page.locked`，`you: true`（本人的会话也拒 `edit`，设计审查 C-N7） |
| 6 | bob | `delete` Notes | `page.locked`，`you: true`；bob 的会话还活着（REST 的删除对本人放行、结束会话：这一格是 MCP 与 REST 不同的那一处） |
| 7 | bob、alice | bob 在他的会话里以 REST 保存一次（版本号 2）；alice 带第 1 格的令牌（版本号 1）`write` Notes，内容与现在的相同 | `changed: false`、`version` 是 `:2`（与当前相同的写不经守卫、也不比版本号，2.1 第 10 条） |
| 8 | alice | `write` `Notes/Child.md` | 建了（新建放行） |
| 9 | alice | `move` Notes 到 `Notes2.md`，再到 `Beside/Notes2.md` | 都成（改名、移动放行；没有链接） |
| 10 | bob、alice | bob `write` `Beside/Late.md`（在移动之后，所以它在 Beside 下排在 Notes2 之后）并以 REST 开它的会话；alice 以 REST 开 `Beside/Notes2/Child.md` 的会话 | 都成 |
| 11 | alice | `delete` `Beside.md`（`recursive`） | `page.locked`，`locks` 依次是 Notes2（bob，`Beside/Notes2.md`）、Late（bob，`Beside/Late.md`）、Child（alice，`Beside/Notes2/Child.md`，`you: true`）：子树的次序（逐层、每层按次序值），不是页 id 的次序（Notes2、Child、Late）；`locks_count` 3，`lock` 是 Notes2、不带 `path`；文字说"3 pages being edited in the web editor, by bob and by you, in a browser" |
| 12 | bob、alice | 以 REST 结束三个会话 | 各 204 |
| 13 | alice | `delete` `Beside.md`（`recursive`） | 成；`pages` 4 |
| — | — | 整个测试里 | `edit_sessions` 里从没有 `mcp:` 的行（MCP 不开会话）；最后 `checkPages` |

第 11 格是整个程序里唯一看到两个以上的锁的一处：`lockedAll` 只答第一个、或按页 id 排的，在这里失败（6.3 第 28 条；设计审查 C-M10、B-N5）。`move` 的 `linking.pages_locked` 里本人的锁与两个锁的次序（页 id 的）在 3.13.5。

#### 3.13.4 交错

`bootstrap/interleavings_mcp_test.go`，照 13.4 第 4 条的写法（测试持着双方都要的行，`pgtest.WaitForLockWaitsOn` 等到对方在等它，`interleavingWait`，`interleavings_workspace_test.go:33`、`:151-220`）；每个结束时核对 `checkPages`、`checkLinks`、`checkSearch`（`interleavings_page_test.go:44-111`）。MCP 的一步是 `step` 的 `command`（在进程里经 HTTP 调 `/mcp`，`interleavings_workspace_test.go:123-127`）：

| 测试 | 持的行 | 两步 | 期望 |
|---|---|---|---|
| `TestAnMCPWriteMeetsAnOpening`（两种次序） | `contentRow(Notes)` | bob 开会话；alice 的 `write`（带 `base_version`） | 开会话在先：`write` 答 `page.locked`（bob），正文不变、会话活着；`write` 在先：写成、之后开会话成（照交错 51，`interleavings_lock_test.go:223-252`） |
| `TestAnMCPEditMeetsAnOpeningAfterItsRead` | `contentRow(Notes)`，`interleaveBehind` | bob 开会话（等着行）；alice 的 `edit`（不加锁的读与预检都过了，等在 `LockContent` 上） | 放开之后会话先拿到行；`edit` 的单元里守卫答 `page.locked`（"读与写之间锁被取"），带 `lock.path`，正文不变、会话活着 |
| `TestAnMCPEditRedoesAfterAConcurrentWrite` | `contentRow(Notes)`，`interleaveBehind` | REST 的正文写（等着行）；alice 的 `edit`（读到 r，等在 `LockContent` 上） | 放开之后 REST 写出 r + 1；`edit` 的 `Base: r` 不符，从读起重做一次，成：`redone`（日志）、版本号 r + 2、正文是 REST 的加替换；带 `base_version` 的同样的一步答 409、`current_version` 是 r + 1 |
| `TestAnMCPDeleteMeetsItsCallersOpening`（两种次序） | `notebookRow(Eng)` | bob 开 Notes 的会话；bob 的 `delete` Notes | 开会话在先：`delete` 答 `page.locked`、`you: true`、会话活着；`delete` 在先：页进回收站，开会话答 `page.not_found`（照交错 50，`interleavings_lock_test.go:192-221`） |
| `TestAnMCPCreateMeetsACreationOfItsName`（两种次序） | `notebookRow(Eng)` | alice 的 `write` `Plan.md`（新建）；bob 的 REST 新建 Plan | 后到的答 `page.title_taken`（MCP 的文字说"is a page already"，REST 的 409）；只有一个 Plan |
| `TestAnMCPDeleteSeesAChildAddedBeforeItsUnit`（两种次序） | `notebookRow(Eng)` | alice 的 `delete` `Notes.md`（不带 `recursive`，它没有子节点：不加锁的几步都过了）；bob 的 REST 在 Notes 下新建 | 新建在先：`delete` 的单元里读的子树有新的子页，答 422 `recursive`（"1 page"），什么都没删；`delete` 在先：Notes 进回收站，新建答 422 `validation_failed`（父页已不是这本笔记本的页，`page/app/unit_create.go:24-27`）。在单元之外数子节点的写法在第一种次序里把新的子页一起删了（2.1 第 13 条） |
| `TestAnMCPWriteFindsItsPathUnderTheLock`（两种次序） | `notebookRow(Eng)` | bob 的 REST 改名 Notes 为 Old；alice 的 `write` `Notes.md`（带 Notes 的令牌） | 改名在先：`write` 在锁下找不到，答 `page.not_found`，Old 的正文不变；`write` 在先：写成，之后改名 |

`TestAnMCPEditRedoesAfterAConcurrentWrite` 的最后一步（带 `base_version`）守的是"有 `Base` 时答 409、不写"；"有 `Base` 时也重做"的变异在这里照样答同样的 409（重做读到 r + 1，令牌仍不符），所以由 `EditPage` 的表格测试数 `PageContent` 的读抓住（3.8.6，6.3 第 22 条，设计审查 C-M10）。

两种没有并发的窗口、顺序的测试（在 `mcp_writes_test.go`）：

- `TestAWriteNeverLandsOnAnotherPageAtItsPath`（00 号文档 4.10 的例子；设计审查 B-M3、C-M2）：`read` `Inbox/todo.md`（N:1）；REST 把 N 改名为 done，再以**一次** REST 调用在 Inbox 下新建带正文的 todo（M:1，与令牌的版本号相同，之后不再写它）；带 N:1 `write` `Inbox/todo.md`：409，文字是"另一页"的那一种（"is another page now than the one base_version was read from"），`current_version` 是 M:1；M 与 N 的正文与版本号都不变。只比版本号的写法（6.3 第 13 条）在这里静静地覆盖了 M，测试失败。
- `TestAVersionOfAnotherNotebookWritesNothing`：两本笔记本 A、B 各有 `Doc.md`，**都以一次带正文的 REST 调用建成，都是版本号 1**；以 B 的 `Doc` 的令牌写 A 的 `Doc.md`：409（"另一页"）、`current_version` 是 A 的；两页都不变，B 里没有新的变更集。

#### 3.13.5 `update_links`

M6/P4 的移交第 2 项（`bootstrap/mcp_writes_test.go` 的 `TestAMoveWithoutItsLinksLeavesThemAndTheIndex`；次序照设计审查 B-M5、C-M8 改过：移走 A 之后 `[[A]]` 就不再指向任何页，之后的步骤无从做起）：笔记本里有页 X、A、B（链 `[[A]]`、`[[A|别名]]`）、C（链 `[[A]]`），先建 C 后建 B（页 id 的次序 C、B，与名称的、持锁人的次序 B、C 不同，所以第 1 步看得出次序是页 id 的；设计审查的复核 C 的 N7）；另一组 D、E（E 链 `[[D]]`、`[[D|别名]]`）。alice 以 REST 开 B 的会话（她自己的），bob 开 C 与 E 的：

1. alice `move` A 到 `X/A2.md`（`update_links` 缺省）：`linking.pages_locked`，`locks` 是 C（bob）与 B（alice，`path: "B.md"`，`you: true`：本人的也算，设计审查 C-N7），次序是页 id 的（`LockHolders.Of`），`locks_count` 2；文字里的名字按锁的次序（bob、你，照 3.11.5 的规则）；什么都没写，A 仍在 `A.md`。
2. alice 与 bob 结束 B、C 的会话之后，同样的移动：成；B、C 的链接改写为指向 A2 的写法（M6 的规则），`page_links` 指向 A2，`checkLinks`。
3. alice `move` D 到 `X/D2.md`，`update_links: false`：成（E 被编辑也不拒）；E 的正文逐字节不变，它的索引照样重新解析（`[[D]]` 找不到 D2，`page_links` 的这几条现在不指向任何页），`checkLinks`（增量等于重建）。

（草稿里第 3 步在笔记本的副本上以 REST 的两步再做一遍、比较正文，不做了：与 REST 比较过的只有 3.13.6 的跨父节点一步；047 号用例是一步的结果、不由 REST 的两步得出，一步与两步在名称唯一时的 `[[a/x|x]]` 上本来就不同，3.5.3，设计审查 C 的过度设计；草稿这里说 047 与 REST 比较过，不对，复核 B 的 BC-M2。）

#### 3.13.6 写的工具到达用例

`bootstrap/mcp_writes_test.go` 的 `TestTheMCPWritesReachTheUseCases`（两个入口，组合根不交 page 的写的工具时失败），每步之后照数据库核对：

- `write` 新建 `Projects/Q3/Plan.md`（两个缺的父页）：一个变更集（类型 `edit`、客户端 `mcp:whole-program`、执行者 alice），三个条目、三个版本行（父页的空正文、叶子的正文，`base_revision` 为空、版本号 1）；`created` 是三项、从根往下；再写一次同样的：`changed: false`、没有新的变更集；内容不同：409。
- `write` 覆盖：带 `read` 给的令牌写成、版本号 2；以旧的令牌再写：409，`current_version` 是 `:2`。CRLF、BOM 与结尾的空白原样（`read` 读回逐字节相同）。
- `edit`：唯一的一处；多处不带 `replace_all`（422，"found 3 times"）；带它（3 处）；CRLF 的页上以 LF 的 `old_string` 编辑（写回的是 CRLF）；版本令牌在答复里。
- `move`：同一父节点的改名、移动加改名（一个变更集、一个条目，条目带新的父节点与名称）、跨父节点只改大小写（`a/x.md → b/X.md`：链到它的页里的 `[[x]]`、`[[x|别名]]`、`[[x#h]]` 与 REST 两步的结果相同；不用 `[[a/x|x]]`，一步与两步在它上面不同，3.5.3，设计审查 B-M1）、附件改名换扩展名（`flow.png → flow.jpg`）。
- `delete`：有子节点而没有 `recursive`（422，计数）；带它：进回收站（`nodes.trash_root`、`trash_pages`、`trash_assets`，网页的回收站列得出）。
- 每步之后 `checkPages`、`checkLinks`，最后 `Drain` 再 `checkSearch`（MCP 写的立即在搜索的队列里，13.1 第 1 条的入队）。

### 3.14 S1：日志

照 13.1 第 10 条：mcpserver 每次调用照旧一行"MCP tool call"（工具名、`notebook_id`、`user_id`、`token_id`、`request_id`、`client`、`outcome`、`duration_ms`，`mcpserver/log.go:85-90`）；用例记 id 与数目，从不记路径、标题、正文与 `old_string`：

| 用例 | 消息 | 另记 |
|---|---|---|
| `WritePath` 新建 | `page created` | `pages`（建了几页） |
| `WritePath` 覆盖 | `page content written` | `revision` |
| `EditPage` | `page edited` | `revision`、`replacements`、`redone` |
| `RelocatePath` | `node moved` 或 `node renamed` | `update_links` |
| `DeletePath` | `node deleted` | `nodes`、`pages`、`assets` |

公共的属性是 `logged(o, n, client)`（`page/app/create_page.go:61-69`），`client` 是 `mcp:<名>`。什么都没写时不记（同 REST）。另有两行：锁的拒绝的路径读不到时的 warn"lock paths not read"（错误与页数，3.8），大的请求体的名额满时的 info（路由、这个请求的凭证、`own`、等着的数目、两个持有者的凭证与路由，3.2.1）：凭证是限流的键（`pat:<id>`、`session:<id>`），13.1 第 10 条允许记的 id。日志的测试照 P1 的 `callLogged`（`bootstrap/mcp_client_test.go:385`）：一次 `write` 的两行都在、都不带路径与正文（以全是标记字符的路径与正文核对）。

### 3.15 S2 的文件

| 文件 | 包 | 新/改 | 内容 |
|---|---|---|---|
| `page/app/history.go` | app | 改 | `HistoryReads` 加 `Paths PathStore`；按笔记本读一个变更集的 `changesetIn` |
| `page/app/list_changesets.go` | app | 改 | `PathLogQuery`、`PathLog`、`ListChangesets.AtPath`；`Execute` 的后半（读、截断、游标、`logged`）抽成两者共用的 `page` |
| `page/app/get_changeset.go` | app | 改 | `ChangesetPage`、`GetChangeset.InNotebook` |
| `page/app/list_versions.go`、`get_version.go` | app | 改 | `VersionReads` 加 `Paths PathStore`、`States PageStates`；`PathVersionList`、`ListVersions.AtPath`；`PathVersion`、`GetVersion.AtPath`；`Execute` 的读抽成共用的函数 |
| `page/app/*_test.go` | app | 改 | 四个入口（两个 `AtPath`、`ListChangesets.AtPath`、`InNotebook`）的码的次序（3.17.4） |
| `page/domain/changeset.go`、`changeset_test.go` | domain | 改 | `ParseActor` |
| `page/domain/version.go`、`version_test.go` | domain | 改 | `CheckRevision`、`CheckVersionsArgs` |
| `page/adapter/mcp/versions.go`、`log.go`、`changeset.go`、`history.go` 与各自的测试 | mcpadapter | 新 | 三个工具；`history.go` 是 REST 的形状在 MCP 里的写法（`changesetRow`、`changeRow`、`versionRow`；节点的类型写作 `attachment`，3.16.3） |
| `page/adapter/mcp/tools.go`、`instructions.go` | mcpadapter | 改 | `UseCases` 加四个入口；说明的三行（3.12） |
| `page/module.go`、`page/tools.go` | page | 改 | `VersionReads`、`HistoryReads` 带上 `store`；REST 与 MCP 共用同一个用例的值 |
| `api/mcp/tools.json` | 契约 | 改 | 三个工具，两个入口（3.18） |
| `bootstrap/mcp_history_test.go` | bootstrap | 新 | 3.17.3：`TestTheMCPHistoryIsRESTs`、`TestTheMCPHistoryAnswersGoOn` 与它种下长的历史的 `seedLongHistory` |
| `bootstrap/mcp_reads_test.go`、`permission_matrix_mcp_test.go` | bootstrap | 改 | 码的表的三行与它们的调用、矩阵的三行、工具的字符串的合法参数的三项（3.13.1） |
| `e2e/stories/mcp/mc8-history.spec.ts` | e2e | 新 | 4.6 |

### 3.16 S2：三个工具与它们的用例

三个都是只读的（`Annotations{ReadOnly: true}`，`tools.json` 里是 `readOnlyHint: true`、`idempotentHint: false`，同 P1 的 `list`、`read`），在两个入口（`BothEntries`）、`Notebook: true`。判定都是这本笔记本上的 `page.history`（`decideNotebook(…, domain.ActionHistory, notebookID)`，任何角色）：看不到的是 `notebook.not_found`，没有 403。都不加锁、不开事务（同 M8 的读）。都只认活着的节点：回收站里的页没有路径（00 号文档 4.9），网页读得到它的版本。

入口是 M8 的用例的四个新方法：判定与资源的读照 MCP 的次序（00 号文档 4.15：参数的形状 400 → 与资源无关的 422 → 笔记本 → 资源；`versions` 用错了模式的 422 是例外，在游标的 400 之前，3.16.1），之后调与 REST 相同的读。REST 的 `Execute` 不改行为，只把后半抽成共用的函数（M8 的表格测试照旧通过）。

**S2 不加 SQL**（设计审查 C-N10）：三个工具的读都是已有的语句，它们的计划都已有测试守着：P1 的 `RootByKey`、`ChildByKey` 与 `PageStates`，M8 的日志与版本的读，`LinkTargetsByIDs`（按参数做计划、没有统计时，`page/adapter/postgres/store_test.go:459-494`、`links_test.go:225-260`）。后者在 `changeset` 的一页里至多 100 个 id（`shared.PageSize` 的上限），在锁的拒绝里至多 20 个（`MaxNamedLocks`）。

#### 3.16.1 `versions`

```go
type versionsIn struct {
	Path     string  `json:"path" jsonschema:"…"`
	Revision *int    `json:"revision,omitempty" jsonschema:"…"`
	Offset   *int    `json:"offset,omitempty" jsonschema:"…"`
	Column   *int    `json:"column,omitempty" jsonschema:"…"`
	Limit    *int    `json:"limit,omitempty" jsonschema:"…"`
	Cursor   *string `json:"cursor,omitempty" jsonschema:"…"`
}
```

```go
// PathVersionList is versions' list (M9 design 4.9): the page at its path,
// its version now, and a page of its versions, each with the cursor that
// goes on after it.
type PathVersionList struct {
	Path       string
	Node       domain.Node
	Current    domain.VersionToken
	Versions   []domain.Version
	Cursors    []string
	NextCursor string
}

// AtPath lists the versions of the page at path in notebookID as Execute
// lists a page's: the page alive, found by its path. Its order: the cursor,
// 400; the path, 422, an attachment's refused; the limit, 422; the
// notebook, notebook.not_found (page.history); the page, page.not_found;
// then Execute's reads, and the page's version now.
func (l *ListVersions) AtPath(ctx context.Context, notebookID uuid.UUID, path string, limit *int, cursor *string) (PathVersionList, error)

// PathVersion is a version as versions reads it, as ReadPath reads a page:
// the window asked for and the slice of its content in it.
type PathVersion struct {
	Path     string
	Node     domain.Node
	Current  domain.VersionToken
	Revision int // the version's own, maybe above the one asked for
	Window   domain.Window
	Slice    domain.Slice
	Bytes    int
}

// AtPath reads the version of the page at path in notebookID holding
// revision. Its order: the path, 422; the revision, 422 outside
// 1–MaxRevision; the window, 422; the notebook, notebook.not_found; the
// page, page.not_found; the version, page.version_not_found; the column on
// its line, 422; the slice, within m's room.
func (g *GetVersion) AtPath(ctx context.Context, notebookID uuid.UUID, path string, revision int, offset, column, limit *int, m TextMeasure) (PathVersion, error)
```

- **两种调用**：有 `revision` 是读一个版本，没有是列表（00 号文档 4.9）。用错了模式的参数最先答 422，由适配器调 `domain.CheckVersionsArgs(revision, offset, column, cursor)`：读一个版本时的 `cursor` 是 `not_allowed` "is for the list of versions, without revision"，列表时的 `offset`、`column` 是 `not_allowed` "is for reading a version, with revision"，每个用错的参数一项（2.1 第 9 条）。它们不属于这一次调用，所以排在一切之前，带着它们的调用不读游标：这是 4.15 的"400 在先"的一个例外，00 号文档 4.15 与 13.1 第 4 条照写（2.1 第 9 条、第 7 节）。`limit` 两种都收：列表的版本数（1–100，默认 50，`shared.PageSize`）、读的行数（≥ 1，默认 2,000，`domain.CheckWindow`）。
- **`revision` 的范围**：小于 1 或大于 `domain.MaxRevision`（`page/domain/version.go:11`）是 422 `revision` `out_of_range` "must be from 1 to 2147483647"（`domain.CheckRevision`）：与资源无关（P1 2.1 第 6 条的同一条规则）。REST 的 `getPageVersion` 对它们答 `page.version_not_found`（`api/modules/page.yaml:370`），不改。
- **路径**：`domain.ParsePath("path", path, false)`；不以 `.md` 结尾的是 422 `path` `not_allowed` "is an attachment's path: attachments have no versions"（附件的名称从不以 `.md` 结尾，所以 `.md` 的路径只找得到页）。
- **读**：列表照 `Execute` 的读（`Versions.ListVersions`、第一页的 `Editing`、`named`，`page/app/list_versions.go:48-92`），游标是每项之后的 `VersionCursor{Below: v.Revision}`；读一个版本照 `findVersion`（`list_versions.go:121-133`），切行照 `ReadPath`（`domain.Cut`，`page/app/read_path.go:88-92`），不读 `Editing` 与显示名（读的元数据里没有它们）。`Current` 由 `States.PageStates(ctx, [节点])` 读（`page/app/ports.go:203-221`）；其间被删了答 `page.not_found`。
- **列表的答复**（结构）：

```go
type versionsAnswer struct {
	Path           string       `json:"path"`
	NodeID         string       `json:"node_id"`
	CurrentVersion string       `json:"current_version"`
	Versions       []versionRow `json:"versions"`
	NextCursor     *string      `json:"next_cursor"`
}
```

  `versionRow` 是 REST 的 `PageVersion` 的成员（`id`、`revision`、`base_revision`、`byte_size`、`created_at`、`updated_at`、`editing`、`changeset{id, kind, client, created_at, created_by{user_id, display_name}}`，`page/adapter/http/versions.go:90-103`）。项经 `mcpserver.Fit`，放不下的从 `Cursors[k-1]` 接着读（同 P1 的 `list`，`page/adapter/mcp/list.go:44-80`）。
- **读一个版本的答复**（文字，两块，同 `read`）：第一块是一行 JSON `{path, kind, revision, current_version, offset, column, lines, total_lines, byte_size, next_offset, next_column}`，第二块是这些行。`TextMeasure.Room` 量的是这一行在最宽的取值时（`revision` 与令牌的版本号取 `math.MinInt64`，同 `page/adapter/mcp/read.go:70`）。

#### 3.16.2 `log`

```go
type logIn struct {
	Path   *string `json:"path,omitempty" jsonschema:"…"`
	Actor  *string `json:"actor,omitempty" jsonschema:"…"`
	Client *string `json:"client,omitempty" jsonschema:"…"`
	Limit  *int    `json:"limit,omitempty" jsonschema:"…"`
	Cursor *string `json:"cursor,omitempty" jsonschema:"…"`
}
```

```go
// PathLogQuery is log's arguments (M9 design 4.9): Path a node's path for
// LogQuery's page id, Actor an account's id or "me".
type PathLogQuery struct {
	Path, Actor, Client *string
	Limit               *int
	Cursor              *string
}

// PathLog is a page of the log as log answers it: the changesets, each with
// the cursor that goes on after it, and the next page's.
type PathLog struct {
	Changesets []LoggedChangeset
	Cursors    []string
	NextCursor string
}

// AtPath lists the notebook's log as Execute does, its node by its path,
// alive, and "me" the caller. Its order: the cursor, 400; the path, the
// actor, the client and the limit, 422; the notebook, notebook.not_found
// (page.history); the node, page.not_found; then Execute's reads.
func (l *ListChangesets) AtPath(ctx context.Context, notebookID uuid.UUID, q PathLogQuery) (PathLog, error)

// ParseActor is the log's actor s (M9 design 4.9): "me", the caller me, or
// an account's id as uuid.Parse reads one; 422 on actor otherwise, "is an
// account's id or \"me\"". The account is not looked up, as REST's.
func ParseActor(s string, me uuid.UUID) (uuid.UUID, error)
```

- **次序**：`RequireActor` → 游标（`domain.ChangesetFrom()` 起，解不开 400，`page/app/list_changesets.go:52-57`）→ `path`（`ParsePath`，422，末尾的 `/` 拒绝）→ `actor`（`ParseActor`，422）→ `client`（`domain.ParseClientFilter`，422，`page/domain/changeset.go:124-138`）→ `limit`（`shared.PageSize`，422）→ `decideNotebook(ActionHistory)`（`notebook.not_found`）→ `paths.Find`（不加锁，活着的页或附件，`page.not_found` 带 P1 的提示）→ 共用的 `page`：`History.ListChangesets(notebookID, LogFilter{Actor, Client, AllMCP, PageID}, below, size+1)`、截断、每项之后的游标 `ChangesetCursor{At, ID}`、`logged`（`list_changesets.go:92-107`）。REST 的 `Execute` 的次序（404 在 422 之前）不变。
- **`actor: "me"`**：调用者自己（`shared.RequireActor` 的账户）：agent 没有别的办法知道自己的 id（00 号文档 4.9）。按执行者筛的是变更集的作者 `created_by_id`（`page/adapter/postgres/queries/history.sql:30`、`:57`）：PAT 的写的作者是 PAT 的账户，所以"我的"含这个账户在网页里与经 MCP 的写；只要 agent 的写，再加 `client: "mcp"`。
- **`path`**：照 `page_id` 的意思（这个节点的条目与版本所在的变更集，回收站里的行照收，`history.sql:37-61`），只是节点按路径找、只找活着的。
- **答复**（结构）：

```go
type logAnswer struct {
	Changesets []changesetRow `json:"changesets"`
	NextCursor *string        `json:"next_cursor"`
}
```

  `changesetRow` 是 REST 的 `Changeset` 的成员（`id`、`notebook_id`、`kind`、`client`、`message`、`created_by`、`created_at`、`updated_at`、`reverts`、`reverted_by`、`trimmed`、`node_count`、`content_count`、`changes`，`page/adapter/http/history.go:108-156`），`changes` 是前三项变化（`loggedChanges`，`page/app/history.go:33`），不带路径（2.1 第 8 条），节点的类型照 3.16.3 写作 `attachment`；另加一个成员 `changes_cut`（`bool`，`omitempty`）。行经 `Fit`；**连一行都放不下时**（3.16.4：一行带三项变化最坏约 59.7 KB；2.1 第 19 条），答第一行，它的 `changes` 再经一次 `Fit` 取放得下的前几项，并写上 `changes_cut: true`；`next_cursor` 是 `Cursors[0]`（只有这一行时是 `NextCursor`）。所以每次调用至少前进一行，省去了变化的一行明说省去了（13.1 第 31 条的"标出截断"），`changeset` 读得到全部。草稿以"`changes` 少于 `min(3, node_count)`"让 agent 自己推断，不对：`node_count` 只数条目，变化却是条目与版本合起来的（`history.sql:63-71`、`:110-130`），一个移动加两个正文写的变更集 `node_count` 是 1、变化有三项（设计审查 C-M4）。`changes_cut` 只由退回写，别的行没有这个成员。
- **任务的变更集**（M8/P4 的移交第 3 项）：导入、后台的恢复与撤销写的是普通的变更集，照常列出（3.17.3 里有一个导入的）。

#### 3.16.3 `changeset`

```go
type changesetIn struct {
	ID     string  `json:"id" jsonschema:"…"`
	Limit  *int    `json:"limit,omitempty" jsonschema:"…"`
	Cursor *string `json:"cursor,omitempty" jsonschema:"…"`
}
```

```go
// ChangesetPage is changeset's answer (M9 design 4.9): the changeset as
// GetChangeset reads it, on the first page only (nil with a cursor); a
// page of its changes by node id, each with the cursor that goes on after
// it; the paths of their nodes alive now, by node id; the next page's
// cursor; and Start, the cursor of the first page's changes, for a first
// page whose changeset leaves its first change no room.
type ChangesetPage struct {
	Detail     *ChangesetDetail
	Changes    []domain.ChangeRow
	Cursors    []string
	Paths      map[uuid.UUID]string
	NextCursor string
	Start      string
}

// InNotebook reads the changeset id of the notebook notebookID: a
// changeset of another notebook, one the caller could read there among
// them, is page.changeset_not_found (the entry is the notebook's). Its
// order: the cursor, 400; the limit, 422; the notebook, notebook.not_found
// (page.history); the changeset, page.changeset_not_found; then
// GetChangeset's and ListChanges' reads, and the nodes' paths.
func (g *GetChangeset) InNotebook(ctx context.Context, notebookID, id uuid.UUID, limit *int, cursor *string) (ChangesetPage, error)
```

- **`id`** 在输入里是字符串（mcpserver 不收 `uuid.UUID` 的字段，`mcpserver/schema.go:21-34`），适配器以 `uuid.Parse` 读：不是 UUID 的是零值的 id，答 `page.changeset_not_found`（同 P1 的 `notebook`，`mcpserver/tool.go:212-234`，零值的 id 不查库）。
- **判定**：`decideNotebook(ActionHistory, notebookID)` 之后 `History.FindChangeset(id)`（活着的笔记本里没删的，`queries/history.sql:3-9`），`c.NotebookID != notebookID` 也是 `page.changeset_not_found`（00 号文档 4.9："入口是 cd 进去"）。`readableChangeset` 在变更集自己的笔记本上判定（`page/app/history.go:39-60`），这里判定的是调用的笔记本、再要求变更集在它里面，两者对这本笔记本里的变更集一致。
- **读**：没有游标时 `logged` 与 `ChangesetMoves`（同 `GetChangeset.Execute`，`get_changeset.go:34-48`）；`History.ListChanges(c.ID, after, size+1)`，游标是每项之后的 `ChangeCursor{After: NodeID}`，`Start` 是 `ChangeCursor{After: uuid.Nil}`（从头列，同没有游标的那一页的变化，`list_changes.go:35-63`）；这一页里活着的节点经 `NodePaths.Paths` 读一次路径（3.7）。
- **答复**（结构）：

```go
type changesetAnswer struct {
	Changeset  *changesetHeader `json:"changeset"` // null with a cursor
	Changes    []changeRow      `json:"changes"`
	NextCursor *string          `json:"next_cursor"`
}
```

  `changesetHeader` 是 REST 的 `getChangeset` 的成员去掉 `changes`（前三项在 `changes` 里本来就有），加 `renames`、`moves`；`changeRow` 是 REST 的 `ChangesetChange`（`node{id, kind, name, alive}`、`kind`、`before`、`after`、`content`，`page/adapter/http/history.go:129-156`），`node` 另带 `path`：活着的节点现在的路径，回收站里的与清理掉的为 null（2.1 第 8 条）。变化经 `Fit`；第一页连一项都放不下时（变更集最坏约 35.6 KB，一项带路径最坏约 25.1 KB，3.16.4；2.1 第 19 条）答 `changes: []` 与 `next_cursor: Start`：带游标的页没有变更集，一项总放得下，所以每次调用至少前进一项（第一页的那一次前进的是变更集本身）。
- **节点的类型**（设计审查 C-M5）：`node.kind` 是 `page` 或 `attachment`。REST 的 `gen.NodeKind` 写 `asset`（`page/adapter/http/history.go:144`），P1 的工具都说 attachment 并说了为什么（`page/adapter/mcp/list.go:83-86`、`read.go:126`、`open_notebook.go:99-105`），所以 `history.go` 在这里换，`log` 的 `changes` 同样。变化本身的 `kind`（`created`、`moved` 等）照 REST。

#### 3.16.4 答复的上限：计算

照 3.10.3，结构的答复只数文字块（`MaxAnswerBytes` 是文字块的 `TextBytes`，`mcpserver/answer.go:11-17`、`:101-112`），字符串的成员按 `MemberBytes` 算（`answer.go:62-96`）：说明 4,096 字节、全是 `\x01`（`CheckMessage` 只拒 NUL，`page/domain/changeset.go:144-160`）时 7 字节一个，28,672；显示名 100 个 `&`（`identity/domain/user.go:33`），600；标题 255 个 `&`，1,530；客户端 `mcp:` 加 128 个 `&`，772；路径 P = 16,843；`reverted_by` 100 个 id（`MaxRevertedBy`）。以 `scratchpad/m9/p2/exp/sizes/worst.py` 照 Go 的编码量得（`MaxAnswerBytes` = 49,152）：

| 答复 | 最坏 | 结论 |
|---|---|---|
| `log` 的一行，没有变化 | 35,581（带 `changes_cut` 35,602） | 放得下 |
| `log` 的一行，三项变化（每项的五个名称各 1,530） | 59,727 | 过了：退回到这一行、放得下的变化与 `changes_cut`（每项约 8,048：最坏时带一项 43,650，两项 51,698 放不下） |
| `changeset` 的变更集，没有变化 | 35,610 | 放得下 |
| 变更集加一项带路径的变化 | 60,515 | 过了：第一页答变更集与 `Start` |
| 带游标的一页，一项带路径的变化 | 25,081 | 放得下 |
| `versions` 的列表，路径加一项 | 18,995（每多一项约 1,856） | 放得下 |
| `versions` 读一个版本 | 同 P1 的 `read`（元数据一行之外至少约 31 KiB） | 放得下 |

所以三个工具每次调用都前进；没有 `internal_error` 的情形（`Fit` 的 `build(0)` 过界只在形状写错时，`mcpserver/answer.go:117-175`）。`log` 的一行与 `changeset` 的第一页是 00 号文档 4.7 的"单项不会超过它"的例外，2.1 第 19 条把那一句改为"一项可能超过它：`log` 的一行与 `changeset` 的第一页各有退回（4.9）；别的工具的单项都不会"（设计审查 C-M13）。

### 3.17 S2：码的表、矩阵与测试

#### 3.17.1 码的表

```go
"versions":  {"bad_request", "validation_failed", "notebook.not_found", "page.not_found", "page.version_not_found"},
"log":       {"bad_request", "validation_failed", "notebook.not_found", "page.not_found"},
"changeset": {"bad_request", "validation_failed", "notebook.not_found", "page.changeset_not_found"},
```

每个码契约都声明过（`listPageVersions`、`getPageVersion`、`listChangesets`、`getChangeset`、`listChangesetChanges` 的 `x-problem-codes`，`api/modules/page.yaml:338`、`:370`、`:803`、`:850`、`:877`）。`TestTheMCPToolsErrorsCarryTheirCodes` 的调用：

| 工具 | 码 | 调用 |
|---|---|---|
| `versions` | `bad_request` | `path: 7`；列表的 `cursor: "x"` |
| | `validation_failed` | `revision: 0`；`revision` 与 `cursor` 一起；没有 `revision` 而有 `offset`；`limit: 101`；附件的路径 |
| | `notebook.not_found` | 账户入口里 bob 的笔记本 |
| | `page.not_found` | `Missing.md` |
| | `page.version_not_found` | `revision: 99` |
| `log` | `bad_request` | `cursor: "x"`；`limit: "x"` |
| | `validation_failed` | `actor: "you"`；`client: "mobile"`；`limit: 0` |
| | `notebook.not_found` | 同上 |
| | `page.not_found` | `path: "Missing.md"` |
| `changeset` | `bad_request` | `cursor: "x"` |
| | `validation_failed` | `limit: 0` |
| | `notebook.not_found` | 同上 |
| | `page.changeset_not_found` | 随机的 UUID；`"not-a-uuid"`；alice 的另一本笔记本里的变更集（她在那本里读得到） |

工具的字符串的合法参数的表（3.13.1）加三项：`versions {path: "Strings.md"}`、`log {}`、`changeset {id: …}`。

#### 3.17.2 矩阵的三行

读者的格（P1 的 `readers`：看得到这本笔记本的答结果，`permission_matrix_mcp_test.go:112-117`），`BothEntries`，没有 `write`：

| 工具 | 参数 | `check` |
|---|---|---|
| `versions` | `{path: pagePathOf(c)}` | 一项、版本号 1、`current_version` 是这一页的 `:1`、`path` |
| `log` | `{}` | 第一行是种下的改名（`s.changeset(notebookOf(c))`，同 REST 的行，`permission_matrix_page_test.go:314`） |
| `changeset` | `{id: s.changeset(notebookOf(c))}` | 变更集的 id、`renames` 为真、`moves` 为假；一项变化，节点是 `renamedIn(notebookOf(c))`，`node.path` 是它的路径 |

#### 3.17.3 整个程序的测试

`bootstrap/mcp_history_test.go`，`newAcmeTeam(t, "member", "")`、alice 与 bob 的 PAT，笔记本 Eng：

- `TestTheMCPHistoryIsRESTs`：备好的历史：alice 经 MCP 写（`write` 新建两层、覆盖、`edit`、`move`、`delete`），alice 以 REST 上传一个附件 `Guide/flow.png` 再经 MCP `move` 它为 `Guide/flow-1.png`（历史里有附件，设计审查 C-M5），bob 经 REST 写与改名，bob 撤销 alice 的一个变更集（REST，它带说明），一个导入（`tm.imported`，`bootstrap/transfer_import_test.go:97`）。然后逐个比较（REST 的一方以 `tm.history`，`bootstrap/history_test.go:89-106`；JSON 照成员比较，节点的类型把 REST 的 `asset` 当作 `attachment`，这是唯一换的成员）：
  - `log` 不带筛选、`path`（对 `page_id`）、`actor: "me"`（对 `actor=<alice 的 id>`）、`actor: <bob 的 id>`、`client: "mcp"`、`client: "mcp:whole-program"`、`limit: 2` 一页一页地读完：行与 REST 的 `listChangesets` 相同、次序相同，`next_cursor` 读完的集合与 REST 的相同；导入的变更集在里面（`kind: import`）。
  - `changeset` 每个变更集：变更集等于 REST 的 `getChangeset` 减去 `changes`；变化逐项等于 `listChangesetChanges` 加 `node.path`，`path` 等于 `NodePaths.Paths` 给的、删了的节点为 null；附件的移动的变化 `node.kind` 是 `attachment`；`limit: 1` 读完。别的笔记本的变更集、不是 UUID 的 id：`page.changeset_not_found`。
  - `versions` 列表等于 REST 的 `listPageVersions`，`current_version` 是 `read` 给的令牌；读每个版本（`limit` 小，经 `next_offset` 读完）连起来等于 REST 的 `getPageVersion` 的 `content`；问一个被会话合并的中间版本号，答出的 `revision` 是那个版本自己的（同 REST）。
  - 删了的页：`versions` 与 `log` 的 `path` 答 `page.not_found`；它的变化在 `changeset` 里 `path` 为 null。
- `TestTheMCPHistoryAnswersGoOn`（两种退回在真的库上都走到，设计审查 C-M3、C-M11）。P2 与 REST 能写的变更集到不了 `log` 的退回（一行要三项各带五个长名称的移动或改名，P2 的 `move` 只记移动的那一个节点；创建、恢复、删除只有一边），`changeset` 的退回要变更集带说明（MCP 的写没有，REST 的撤销有：它的说明是请求的，`api/modules/page.yaml:899`）而节点的路径至少 8 段长的。所以这个测试以 `seedLongHistory` 备下历史，`log` 的那一种以 SQL 种下（写的路径不必是真的）：
  1. **`log` 的退回**：经 MCP `write` 建 `P`（根下，标题 255 个 `&`）、`P/Q`（同样）与 Q 下的三页 X1–X3（标题 254 个 `&` 加一位数字，兄弟的键不同）；以 SQL 加两个变更集 W1、W2（`kind` `edit`、`client` `mcp:whole-program`、作者 alice、`message` 是 4,096 个 `\x01`、时刻比别的都新），各三个 `changeset_items`：X1–X3 从（P，255 个 `&`）移到（Q，255 个 `&`）。`FirstChanges` 以连接读节点与父页的名称（`history.sql:110-130`），所以它们是真的节点、名称是长的；条目不必与树一致，这个测试不跑 `checkPages`。照 `exp/sizes/seeded.py` 量：W1 的一行 53,299 字节，过界；退回的一行带两项变化与 `changes_cut` 45,321。
  2. **`changeset` 的退回**（真的写，设计审查 C-M3 的建议）：经 MCP `write` 建一条 10 层的链，每段 254 个 `&` 加一位数字（路径约 2,562 字节，JSON 里约 15,316），再经 MCP `move` 把叶子在原地改名，然后 bob 经 REST 撤销这个变更集，说明是 4,096 个 `\x01`：撤销的变更集 C1 有一个条目（叶子改回原来的名称），节点的路径是那条长路径。第一页带这一项约 52,850，过界；退回的 `changes: []` 与 `Start` 约 29,526；带游标的一页约 23,500（以 bob 的 REST 撤销的真样子量：`kind` `revert`、`client` `api`、带 `reverts`；草稿照一个没有 `reverts` 的 MCP 的 `edit` 量，少约 194 字节，设计审查的复核 C 的 N3，`review-C/check/c1_sizes.py`，`exp/sizes/seeded.py` 随之）。
  3. 断言：`log` 以 `limit: 2` 从头读完：W1、W2 各自单独一页、`changes_cut: true`、两项变化；被截的两行（W1、W2）的 `changes` 是 REST 的前三项里的前两项、带 `changes_cut: true`，别的成员与 REST 的相同；其余的行与 REST 的 `listChangesets` 的全部相同；次序相同（REST 每个变更集列前三项变化，`api/modules/page.yaml:785-800`，所以被截的行不能整行相同，复核 C 的 N5）。最坏的两行在一页里的最前：`limit: 2` 的第一页本该有 W1、W2，退回只答 W1；退回的 `next_cursor` 若写成 `NextCursor`（存储的，在 W2 之后），W2 被跳过（它后面的一行照读），测试失败（6.3 第 43 条；复核 C 的 N4）。`changeset` C1 以 `limit: 1` 读完：第一页 `changes: []`、`next_cursor` 是 `Start`，第二页是那一项、`node.path` 是那条长路径，与 REST 的相同。每个答复在 48 KiB 之内。
  最坏的取值（显示名 100 个 `&`、路径 10 段都长而再带说明）只由适配器的测试在替身上量（3.17.4）；这个测试在真的库上核对两种退回都前进、都不跳行（6.2 照此改写）。

#### 3.17.4 用例与适配器的测试

- `page/app`：四个入口的表格测试，每步一行、前一步的错误挡住后一步（例如坏的游标与看不到的笔记本：400；`actor` 坏而笔记本看不到：422；看得到的笔记本里不存在的路径：`page.not_found`；别的笔记本的变更集：`page.changeset_not_found`；`versions` 用错了模式的 `offset` 与坏的游标：422）；`AtPath` 与 `Execute` 对同一页、同一个筛选给同样的行；`InNotebook` 带游标时没有 `Detail`、`Start` 从头列。
- `page/domain`：`ParseActor`（`me`、大写的 `ME` 拒绝、各种 UUID 的写法照 `uuid.Parse`、空串、`me ` 拒绝）；`CheckRevision`（0、1、`MaxRevision`、`MaxRevision+1`）；`CheckVersionsArgs`（每种用错的组合一项）。
- `page/adapter/mcp`：三个答复的形状；`Fit` 截断之后的 `next_cursor` 是第 k 项的游标；节点的类型 `asset` 写作 `attachment`（`log` 的 `changes` 与 `changeset` 的变化）；`log` 的退回：替身给一页两行、最坏的一行在前（显示名 100 个 `&`、说明 4,096 个 `\x01`、三项变化各五个 255 个 `&` 的名称）：答复只有它、`changes_cut: true`、`next_cursor` 是第一行的游标（不是 `NextCursor`）；没有退回的行没有 `changes_cut`；`changeset` 的第一页（同上的变更集加一项 10 层长路径的变化）答 `changes: []` 与 `Start`；各自的答复在 48 KiB 之内且前进；`id` 不是 UUID 时交给用例的是零值。

### 3.18 `api/mcp/tools.json` 加的条目

照 P1 的写法：每个入口的工具按名称排（文件今天的次序），工具的成员是 `name`、`title`、`description`、`inputSchema`、`annotations`，参数照 Go 的字段的次序，必填的不收 null、可选的是 `["null", T]`，`additionalProperties: false`，账户入口的 `notebook` 排在参数与 `required` 的最前（`mcpserver/schema.go:21-60`）；注解照 `annotationsOf`（`mcpserver/tool.go:150-156`）。说明的字句就是 Go 里的常量与 `jsonschema` 标签：`TestTheToolListsAreTheContract` 逐个成员比较应用答的与文件里的（`bootstrap/mcp_tools_test.go:152-160`）。格式照 oxfmt（`.oxfmtrc.json` 的 120 列，`make fmt`）。S1 加 `write`、`edit`、`move`、`delete`，S2 加 `versions`、`log`、`changeset`；两个入口的条目除 `notebook` 之外相同，全文如下（以 `scratchpad/m9/p2/exp/toolsjson/gen2.py` 生成、解析核对过）。

按设计审查改过的字句：`write` 说内容相同时什么也不写、`changed` 为假，带着这一页过时的 `base_version`、页正被编辑时也是（A-N7、B-N6）；`edit` 说 `changed` 为假的时候；`move` 说附件保留一个扩展名、扩展名可以换（B-M6），被编辑的页本人的也算；`log` 说一个放不下的变更集单独来、`changes_cut` 为真（C-M4）。

**笔记本入口**（`"notebook"` 的数组里，与 `list`、`read` 一起按名称排：`changeset`、`delete`、`edit`、`list`、`log`、`move`、`read`、`versions`、`write`）：

```json
    {
      "name": "changeset",
      "title": "Read a changeset",
      "description": "Read one of the notebook's changesets by the id log gives: its kind, client, author, times, message, counts, what it reverts and what reverted it, and whether it renames or moves nodes; then what it did to each node it changed, a page at a time by node id: where the node was before and after in the tree, the revision of the content it wrote, and the node's current path (null when it is in the trash or gone). The changeset itself comes on the first page, without cursor; go on with next_cursor while it is set. A changeset of another notebook is not found here. History is read-only here: people revert changesets in the web.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "id": {
            "type": "string",
            "description": "The changeset's id, as log gives it."
          },
          "limit": {
            "type": ["null", "integer"],
            "description": "At most this many changes, 1 to 100. Default 50."
          },
          "cursor": {
            "type": ["null", "string"],
            "description": "next_cursor of the previous answer, to go on from there."
          }
        },
        "required": ["id"],
        "additionalProperties": false
      },
      "annotations": {
        "idempotentHint": false,
        "openWorldHint": false,
        "readOnlyHint": true
      }
    },
    {
      "name": "delete",
      "title": "Delete a page or an attachment",
      "description": "Put a page or an attachment in the notebook's trash, by its path from the notebook's root: people restore it from the trash in the web. A page with pages or attachments under it needs recursive set to true, which puts them in the trash with it. Any of its pages being edited in the web editor, by you too, refuses the deletion, saying by whom; the locks cannot be taken over. The answer has how many pages and attachments went to the trash, itself counted, and the changeset's id and time.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "path": {
            "type": "string",
            "description": "The path of the page or attachment to delete, from the notebook's root."
          },
          "recursive": {
            "type": ["null", "boolean"],
            "description": "Delete a page with its children, its pages and attachments. Default false: a page with children is refused."
          }
        },
        "required": ["path"],
        "additionalProperties": false
      },
      "annotations": {
        "destructiveHint": true,
        "idempotentHint": true,
        "openWorldHint": false,
        "readOnlyHint": false
      }
    },
    {
      "name": "edit",
      "title": "Edit a page",
      "description": "Replace old_string with new_string in a page's content on the server, byte for byte, as a find and replace: old_string must occur exactly once (occurrences that overlap count), unless replace_all is set, which replaces each, left to right. Copy old_string from what read gave, with enough of the text around it to be unique: quotes and spaces are matched as they are. On a page whose line breaks are all CRLF, old_string and new_string with bare LF are matched and written as CRLF. Without base_version, the edit applies to the content as it is now, made once more if the page changed while it was made; with base_version, it is refused when the page changed since. A page being edited in the web editor, by you too, refuses the edit, saying by whom; the lock cannot be taken over. The answer has the page's version, how many times old_string was replaced, changed (false when the content stayed as it was), and the changeset's id and time.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "path": {
            "type": "string",
            "description": "The page's path from the notebook's root, as \"A/B.md\": its ancestors' titles, then its own title and \".md\"."
          },
          "old_string": {
            "type": "string",
            "description": "The text to replace, copied from what read gave, byte for byte: 1 byte to 1 MiB."
          },
          "new_string": {
            "type": "string",
            "description": "The text to put in its place, which differs from old_string."
          },
          "replace_all": {
            "type": ["null", "boolean"],
            "description": "Replace every occurrence of old_string, left to right. Default false: old_string must occur exactly once."
          },
          "base_version": {
            "type": ["null", "string"],
            "description": "The page's version, as read, list or write gave it: the edit is refused when the page changed since. Without it, the edit applies to the content as it is now."
          }
        },
        "required": ["path", "old_string", "new_string"],
        "additionalProperties": false
      },
      "annotations": {
        "destructiveHint": true,
        "idempotentHint": false,
        "openWorldHint": false,
        "readOnlyHint": false
      }
    },
    {
      "name": "log",
      "title": "List the notebook's changes",
      "description": "List the notebook's changesets, newest first: what people and agents changed, each with its kind, client, author (id and display name), times, message, how many nodes and contents it changed, what it reverts and what reverted it, and its first three changes; a changeset too large for the answer comes alone, its changes cut to fit and changes_cut true. path keeps the changesets that changed that page or attachment; actor those of an account, \"me\" for yours; client those of a client, \"mcp\" for every agent. Go on with next_cursor while it is set. changeset reads one in full. History is read-only here: people restore versions and revert changesets in the web.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "path": {
            "type": ["null", "string"],
            "description": "Only the changesets that changed the page or attachment at this path, from the notebook's root."
          },
          "actor": {
            "type": ["null", "string"],
            "description": "Only the changesets by this account: its id, or \"me\" for yours."
          },
          "client": {
            "type": ["null", "string"],
            "description": "Only the changesets of this client: \"web\", \"api\", \"cli\", \"mcp:<name>\" for an agent, or \"mcp\" for every agent."
          },
          "limit": {
            "type": ["null", "integer"],
            "description": "At most this many changesets, 1 to 100. Default 50."
          },
          "cursor": {
            "type": ["null", "string"],
            "description": "next_cursor of the previous answer, to go on from there."
          }
        },
        "additionalProperties": false
      },
      "annotations": {
        "idempotentHint": false,
        "openWorldHint": false,
        "readOnlyHint": true
      }
    },
    {
      "name": "move",
      "title": "Move or rename a page or an attachment",
      "description": "Move, rename, or both at once, a page or an attachment: from is its path, to the full path it will have, both from the notebook's root (\"Archive/Plan 2026.md\"), never a page to move it into. A page's path ends with .md, and an attachment's keeps an extension, which may change. A page moves with its children. The page above to must exist: move creates no pages. The links to what moves, and to its pages, are rewritten to follow it, unless update_links is false, which leaves every page linking to it as it is: those links may then lead elsewhere, or nowhere. Rewriting the links of a page being edited in the web editor, by you too, is refused, saying by whom; the lock cannot be taken over. The answer has the new path, node_id, changed, and the changeset's id and time (null when nothing moved).",
      "inputSchema": {
        "type": "object",
        "properties": {
          "from": {
            "type": "string",
            "description": "The path of the page or attachment to move, from the notebook's root."
          },
          "to": {
            "type": "string",
            "description": "The full path it will have, its new name last: never a page to move it into. The page above it must exist."
          },
          "update_links": {
            "type": ["null", "boolean"],
            "description": "Rewrite the links to what moves, and to its pages, so that they follow it. Default true."
          }
        },
        "required": ["from", "to"],
        "additionalProperties": false
      },
      "annotations": {
        "destructiveHint": true,
        "idempotentHint": true,
        "openWorldHint": false,
        "readOnlyHint": false
      }
    },
    {
      "name": "versions",
      "title": "List or read a page's versions",
      "description": "Without revision, list a page's versions, newest first: each with its revision, size, times, and the changeset that wrote it (its kind, client and author); the answer also has the page's current version. Go on with next_cursor while it is set. With revision, read that version's content as read reads a page: two text blocks, a line of JSON metadata (path, kind, revision, current_version, offset, column, lines, total_lines, byte_size, next_offset, next_column), then its lines exactly as stored; go on from next_offset, and next_column. The version answered has its own revision, which may be above the one asked for: an edit session's saves share one version. Revisions are for reading: write and edit take the page's version, from read. A page in the trash has no path: its versions are read in the web.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "path": {
            "type": "string",
            "description": "The page's path from the notebook's root, as \"A/B.md\"."
          },
          "revision": {
            "type": ["null", "integer"],
            "description": "The revision to read, from 1. Without it, the list of the page's versions."
          },
          "offset": {
            "type": ["null", "integer"],
            "description": "With revision: the first line, from 1. Default 1."
          },
          "column": {
            "type": ["null", "integer"],
            "description": "With revision: the byte of the first line to start from, from 0: next_column of the previous answer. Default 0."
          },
          "limit": {
            "type": ["null", "integer"],
            "description": "Without revision: at most this many versions, 1 to 100, default 50. With revision: at most this many lines, from 1, default 2000."
          },
          "cursor": {
            "type": ["null", "string"],
            "description": "Without revision: next_cursor of the previous answer, to go on from there."
          }
        },
        "required": ["path"],
        "additionalProperties": false
      },
      "annotations": {
        "idempotentHint": false,
        "openWorldHint": false,
        "readOnlyHint": true
      }
    },
    {
      "name": "write",
      "title": "Write a page",
      "description": "Write a page's whole content by its path from the notebook's root; the path ends with .md. Without base_version, it creates the page, and the parent pages it lacks, empty, in one change; a path that is a page already is refused. With base_version, the version read, list or write gave, it replaces the content of the page at the path: refused when the page changed since, or when the path names another page now; read it again. When the page holds that content already, nothing is written and changed is false, with an older base_version of that page too, and while the page is being edited. The content is stored byte for byte: at most 5 MiB of UTF-8, without NUL. A page being edited in the web editor, by you too, refuses the write, saying by whom; the lock cannot be taken over. The answer has the page's path, node_id and version, changed (false when nothing was written), the nodes it created, and the changeset's id and time.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "path": {
            "type": "string",
            "description": "The page's path from the notebook's root, as \"A/B.md\": its ancestors' titles, then its own title and \".md\"."
          },
          "content": {
            "type": "string",
            "description": "The page's whole content, as it will be stored: frontmatter, byte order mark and line breaks included."
          },
          "base_version": {
            "type": ["null", "string"],
            "description": "The page's version, as read, list or write gave it, to replace its content. Without it, the page is created."
          }
        },
        "required": ["path", "content"],
        "additionalProperties": false
      },
      "annotations": {
        "destructiveHint": true,
        "idempotentHint": true,
        "openWorldHint": false,
        "readOnlyHint": false
      }
    }
```

**账户入口**（`"account"` 的数组里：`changeset`、`delete`、`edit`、`list`、`log`、`move`、`notebooks`、`open_notebook`、`read`、`versions`、`write`）：

```json
    {
      "name": "changeset",
      "title": "Read a changeset",
      "description": "Read one of the notebook's changesets by the id log gives: its kind, client, author, times, message, counts, what it reverts and what reverted it, and whether it renames or moves nodes; then what it did to each node it changed, a page at a time by node id: where the node was before and after in the tree, the revision of the content it wrote, and the node's current path (null when it is in the trash or gone). The changeset itself comes on the first page, without cursor; go on with next_cursor while it is set. A changeset of another notebook is not found here. History is read-only here: people revert changesets in the web.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "notebook": {
            "type": "string",
            "description": "The notebook's id, from notebooks."
          },
          "id": {
            "type": "string",
            "description": "The changeset's id, as log gives it."
          },
          "limit": {
            "type": ["null", "integer"],
            "description": "At most this many changes, 1 to 100. Default 50."
          },
          "cursor": {
            "type": ["null", "string"],
            "description": "next_cursor of the previous answer, to go on from there."
          }
        },
        "required": ["notebook", "id"],
        "additionalProperties": false
      },
      "annotations": {
        "idempotentHint": false,
        "openWorldHint": false,
        "readOnlyHint": true
      }
    },
    {
      "name": "delete",
      "title": "Delete a page or an attachment",
      "description": "Put a page or an attachment in the notebook's trash, by its path from the notebook's root: people restore it from the trash in the web. A page with pages or attachments under it needs recursive set to true, which puts them in the trash with it. Any of its pages being edited in the web editor, by you too, refuses the deletion, saying by whom; the locks cannot be taken over. The answer has how many pages and attachments went to the trash, itself counted, and the changeset's id and time.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "notebook": {
            "type": "string",
            "description": "The notebook's id, from notebooks."
          },
          "path": {
            "type": "string",
            "description": "The path of the page or attachment to delete, from the notebook's root."
          },
          "recursive": {
            "type": ["null", "boolean"],
            "description": "Delete a page with its children, its pages and attachments. Default false: a page with children is refused."
          }
        },
        "required": ["notebook", "path"],
        "additionalProperties": false
      },
      "annotations": {
        "destructiveHint": true,
        "idempotentHint": true,
        "openWorldHint": false,
        "readOnlyHint": false
      }
    },
    {
      "name": "edit",
      "title": "Edit a page",
      "description": "Replace old_string with new_string in a page's content on the server, byte for byte, as a find and replace: old_string must occur exactly once (occurrences that overlap count), unless replace_all is set, which replaces each, left to right. Copy old_string from what read gave, with enough of the text around it to be unique: quotes and spaces are matched as they are. On a page whose line breaks are all CRLF, old_string and new_string with bare LF are matched and written as CRLF. Without base_version, the edit applies to the content as it is now, made once more if the page changed while it was made; with base_version, it is refused when the page changed since. A page being edited in the web editor, by you too, refuses the edit, saying by whom; the lock cannot be taken over. The answer has the page's version, how many times old_string was replaced, changed (false when the content stayed as it was), and the changeset's id and time.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "notebook": {
            "type": "string",
            "description": "The notebook's id, from notebooks."
          },
          "path": {
            "type": "string",
            "description": "The page's path from the notebook's root, as \"A/B.md\": its ancestors' titles, then its own title and \".md\"."
          },
          "old_string": {
            "type": "string",
            "description": "The text to replace, copied from what read gave, byte for byte: 1 byte to 1 MiB."
          },
          "new_string": {
            "type": "string",
            "description": "The text to put in its place, which differs from old_string."
          },
          "replace_all": {
            "type": ["null", "boolean"],
            "description": "Replace every occurrence of old_string, left to right. Default false: old_string must occur exactly once."
          },
          "base_version": {
            "type": ["null", "string"],
            "description": "The page's version, as read, list or write gave it: the edit is refused when the page changed since. Without it, the edit applies to the content as it is now."
          }
        },
        "required": ["notebook", "path", "old_string", "new_string"],
        "additionalProperties": false
      },
      "annotations": {
        "destructiveHint": true,
        "idempotentHint": false,
        "openWorldHint": false,
        "readOnlyHint": false
      }
    },
    {
      "name": "log",
      "title": "List the notebook's changes",
      "description": "List the notebook's changesets, newest first: what people and agents changed, each with its kind, client, author (id and display name), times, message, how many nodes and contents it changed, what it reverts and what reverted it, and its first three changes; a changeset too large for the answer comes alone, its changes cut to fit and changes_cut true. path keeps the changesets that changed that page or attachment; actor those of an account, \"me\" for yours; client those of a client, \"mcp\" for every agent. Go on with next_cursor while it is set. changeset reads one in full. History is read-only here: people restore versions and revert changesets in the web.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "notebook": {
            "type": "string",
            "description": "The notebook's id, from notebooks."
          },
          "path": {
            "type": ["null", "string"],
            "description": "Only the changesets that changed the page or attachment at this path, from the notebook's root."
          },
          "actor": {
            "type": ["null", "string"],
            "description": "Only the changesets by this account: its id, or \"me\" for yours."
          },
          "client": {
            "type": ["null", "string"],
            "description": "Only the changesets of this client: \"web\", \"api\", \"cli\", \"mcp:<name>\" for an agent, or \"mcp\" for every agent."
          },
          "limit": {
            "type": ["null", "integer"],
            "description": "At most this many changesets, 1 to 100. Default 50."
          },
          "cursor": {
            "type": ["null", "string"],
            "description": "next_cursor of the previous answer, to go on from there."
          }
        },
        "required": ["notebook"],
        "additionalProperties": false
      },
      "annotations": {
        "idempotentHint": false,
        "openWorldHint": false,
        "readOnlyHint": true
      }
    },
    {
      "name": "move",
      "title": "Move or rename a page or an attachment",
      "description": "Move, rename, or both at once, a page or an attachment: from is its path, to the full path it will have, both from the notebook's root (\"Archive/Plan 2026.md\"), never a page to move it into. A page's path ends with .md, and an attachment's keeps an extension, which may change. A page moves with its children. The page above to must exist: move creates no pages. The links to what moves, and to its pages, are rewritten to follow it, unless update_links is false, which leaves every page linking to it as it is: those links may then lead elsewhere, or nowhere. Rewriting the links of a page being edited in the web editor, by you too, is refused, saying by whom; the lock cannot be taken over. The answer has the new path, node_id, changed, and the changeset's id and time (null when nothing moved).",
      "inputSchema": {
        "type": "object",
        "properties": {
          "notebook": {
            "type": "string",
            "description": "The notebook's id, from notebooks."
          },
          "from": {
            "type": "string",
            "description": "The path of the page or attachment to move, from the notebook's root."
          },
          "to": {
            "type": "string",
            "description": "The full path it will have, its new name last: never a page to move it into. The page above it must exist."
          },
          "update_links": {
            "type": ["null", "boolean"],
            "description": "Rewrite the links to what moves, and to its pages, so that they follow it. Default true."
          }
        },
        "required": ["notebook", "from", "to"],
        "additionalProperties": false
      },
      "annotations": {
        "destructiveHint": true,
        "idempotentHint": true,
        "openWorldHint": false,
        "readOnlyHint": false
      }
    },
    {
      "name": "versions",
      "title": "List or read a page's versions",
      "description": "Without revision, list a page's versions, newest first: each with its revision, size, times, and the changeset that wrote it (its kind, client and author); the answer also has the page's current version. Go on with next_cursor while it is set. With revision, read that version's content as read reads a page: two text blocks, a line of JSON metadata (path, kind, revision, current_version, offset, column, lines, total_lines, byte_size, next_offset, next_column), then its lines exactly as stored; go on from next_offset, and next_column. The version answered has its own revision, which may be above the one asked for: an edit session's saves share one version. Revisions are for reading: write and edit take the page's version, from read. A page in the trash has no path: its versions are read in the web.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "notebook": {
            "type": "string",
            "description": "The notebook's id, from notebooks."
          },
          "path": {
            "type": "string",
            "description": "The page's path from the notebook's root, as \"A/B.md\"."
          },
          "revision": {
            "type": ["null", "integer"],
            "description": "The revision to read, from 1. Without it, the list of the page's versions."
          },
          "offset": {
            "type": ["null", "integer"],
            "description": "With revision: the first line, from 1. Default 1."
          },
          "column": {
            "type": ["null", "integer"],
            "description": "With revision: the byte of the first line to start from, from 0: next_column of the previous answer. Default 0."
          },
          "limit": {
            "type": ["null", "integer"],
            "description": "Without revision: at most this many versions, 1 to 100, default 50. With revision: at most this many lines, from 1, default 2000."
          },
          "cursor": {
            "type": ["null", "string"],
            "description": "Without revision: next_cursor of the previous answer, to go on from there."
          }
        },
        "required": ["notebook", "path"],
        "additionalProperties": false
      },
      "annotations": {
        "idempotentHint": false,
        "openWorldHint": false,
        "readOnlyHint": true
      }
    },
    {
      "name": "write",
      "title": "Write a page",
      "description": "Write a page's whole content by its path from the notebook's root; the path ends with .md. Without base_version, it creates the page, and the parent pages it lacks, empty, in one change; a path that is a page already is refused. With base_version, the version read, list or write gave, it replaces the content of the page at the path: refused when the page changed since, or when the path names another page now; read it again. When the page holds that content already, nothing is written and changed is false, with an older base_version of that page too, and while the page is being edited. The content is stored byte for byte: at most 5 MiB of UTF-8, without NUL. A page being edited in the web editor, by you too, refuses the write, saying by whom; the lock cannot be taken over. The answer has the page's path, node_id and version, changed (false when nothing was written), the nodes it created, and the changeset's id and time.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "notebook": {
            "type": "string",
            "description": "The notebook's id, from notebooks."
          },
          "path": {
            "type": "string",
            "description": "The page's path from the notebook's root, as \"A/B.md\": its ancestors' titles, then its own title and \".md\"."
          },
          "content": {
            "type": "string",
            "description": "The page's whole content, as it will be stored: frontmatter, byte order mark and line breaks included."
          },
          "base_version": {
            "type": ["null", "string"],
            "description": "The page's version, as read, list or write gave it, to replace its content. Without it, the page is created."
          }
        },
        "required": ["notebook", "path", "content"],
        "additionalProperties": false
      },
      "annotations": {
        "destructiveHint": true,
        "idempotentHint": true,
        "openWorldHint": false,
        "readOnlyHint": false
      }
    }
```

### 3.19 配置

P2 不加配置项（2.1 第 18 条）。大的请求体的名额 2、等 2 秒与 `/mcp` 的门槛 64 KiB 是组合根的常量（3.2.2），REST 的门槛取已有的 `server.max_body_bytes`；`edit` 的 `MaxEditOldBytes`、错误的 `MaxErrorLocks`、`MaxNamedLocks` 是代码里的常量。`server/configs/config.yaml` 只在 `server.max_body_bytes` 的注释里写上它也是 REST 的大请求体的门槛。`server.read_timeout` 不改（3.2.1）。改它们是 M12 的负载测试之后的事（第 8 节）。

### 3.20 给 P3 复用的

P3 的 `batch` 照 00 号文档 4.11 在一个单元里做多步，复用 P2 的这些，不另写一份：

| P2 的 | P3 怎样用 |
|---|---|
| `domain.ParsePath`、`ParseVersionToken`、`CheckEdit`、`Replace`（线性的匹配器）、`CheckContent` | 每一步的参数检查与 `edit` 的替换（batch 的 `edit` 在合成的正文上做，4.11） |
| `domain.CreationOf`、`MoveKinds`、`DeletionOf`（每一步的规则，3.8，设计审查 B-M7） | `PlanBatch` 在它模拟的树上调同样的函数：缺的祖先以 `.md` 结尾、路过附件、太深、叶子的类型、`from` 与 `to` 的类型、有子节点而没有 `recursive`，一条规则一份 |
| `NodePaths.Walk`（单元里逐段点查） | batch 的每一步在单元里按路径找；P3 在它之上加单元里的缓存与移动、改名、删除之后的作废（2.1 第 1 条留给 P3 的） |
| `Unit.Relocate`、`RelocateNode`（一条语句、`Placement.Field`） | batch 的 `move` |
| 撤销的一步的放回（`moveSubtree` 加 `NodeWriter.RelocateNode`、计划只核对最后的键，3.5.4） | P3 的多轮调度、临时名称与后备的每一个放回都走它；不是 `Unit.Relocate`：它总把节点放在最后，放不回 S\* 的次序（设计审查 B-M7） |
| `Unit.Subtree`、`Unit.DeleteSubtree` | batch 的 `delete` 与它的 `recursive` 的检查 |
| 编辑锁对 MCP 删除的规则（`OpDelete` 与 MCP 的客户端，3.6） | batch 里的删除同样被任何活着的会话拒绝 |
| `mcpserver.ToolError.Fit` 与 `MaxErrorLocks` | batch 的错误带出错的那一步与它的锁（4.11），放到 48 KiB 之内 |
| `domain.PathTaken`、`VersionMismatch`、`Recursive`、`LockPaths`、`Quoted` 与 page 的 `adapter/mcp` 的文字 | batch 的每一步的错误照同样的文字，另说第几步 |
| 四个用例的码的次序 | batch 的每一步照它的工具的次序（4.15），加 `page.batch_too_large` |
| 工具的字符串的合法参数的表（3.13.1） | batch 加自己的一项（`ops` 里一个合法的 `write`）：没有它测试失败；`ops[].path` 等嵌套的字符串只换第一个 `op` 里的 |
| `TestTheMCPWritesReachTheUseCases`、锁的每一格、交错的帮助函数 | batch 的整个程序测试照样写 |

P3 加 `batch` 的那一行说明（2.1 第 12 条）。P2 不预留 batch 的接口：没有用的接口进不了 knip 与 lint 的检查。

### 3.21 每个新检查的反向对照

每个新的检查都要在去掉它（或把它改错）时有测试失败（v0.1 总体设计 12 的"反向对照"，00 号文档第 9 节）。下表是检查、去掉它时失败的测试；变异的完整清单在 6.3。

| 检查 | 改错的样子 | 失败的测试 |
|---|---|---|
| 大的请求体的名额 | 不取名额；用完不放回 | `TestTwoLargeBodiesAreServedAtOnce`、`TestTheSlotComesBackOnEveryPath`（一个名额的应用，漏一个就看得见，3.2.5）；整个程序的 `TestTheLargeBodiesShareTwoSlots` |
| 一个凭证至多一个名额 | 不看凭证 | `TestACredentialsSecondLargeBodyWaitsForItsFirst`；整个程序的第 5 步 |
| 等整个服务的名额等满时还凭证的名额 | 答 503 时凭证的名额不还 | `TestTwoLargeBodiesAreServedAtOnce` 的最后一句（第三个凭证的下一个拿到名额）；整个程序的第 5 步（P3 答过 503 之后它的 `hold` 拿到空着的那个，不是 503） |
| 满了的一行点名持有者 | 只记等着的数目 | `TestTheSaturatedLineNamesTheHolders`；整个程序的第 2 步 |
| 等候 | 不等，立即 503 | `TestALargeBodyWaitsForItsTurn`（两个名额都被别的凭证持着）；`TestACredentialsSecondLargeBodyWaitsForItsFirst` 的最后一句 |
| 门槛与声明的长度 | 门槛之下也取；分块的不取；不看 `Content-Length` | `TestTwoLargeBodiesAreServedAtOnce` 恰在门槛的一行、`TestABodyOfNoLengthTakesASlotPastItsThreshold`、`TestADeclaredLengthPastTheLimitIs413WithoutASlot` |
| 名额在认证与桶之后 | 在认证之前取 | `TestTheSlotsAreTakenAfterTheBuckets` |
| `Client.Valid` 用 `Unshowable` | 改回 `unicode.IsControl` | `page/domain/client_test.go` 的 `Valid` 一表（U+2028、U+202E 拒绝） |
| 迁移 00044 的范围 | 少一个范围（例如 U+2066–U+2069） | `TestChecksRejectCounterexamples` 的逐码点的一条查询（3.3.2） |
| `MCPClient` 的前缀与清理 | 不加 `mcp:`；不再清理 | `client_test.go` 的 `MCPClient` 一表与 10 万个随机串的性质（3.3.1）；不加 `mcp:` 另由最后一跳的 `SELECT client` 抓住 |
| `ParseVersionToken` 的严格 | 收大写的 UUID、前导的 0、`+1`、32 位十六进制的 UUID | `TestParseVersionToken` |
| 令牌的节点是路径的节点 | 只比版本号 | `TestAWriteNeverLandsOnAnotherPageAtItsPath`（M 与令牌同是版本号 1）；`TestAVersionOfAnotherNotebookWritesNothing`（两页都是版本号 1） |
| `RelocateNode` 撞名 | 不经 `nameTaken` | 存储的测试（照 `TestEverySiblingsIndexTellsANameTaken`，3.5.1） |
| `Relocation` 的大小写改名不看父节点 | 改回只在同一父节点 | `TestRelocationIsWhatARewriteFollows` 加的一行；样例 `047-move-and-recase`（改名的用例的 `recased` 取自 `domain.Relocation`，3.5.3）；`TestTheMCPWritesReachTheUseCases` 的跨父节点的一步 |
| 撤销的一步的放回 | 改回先移动、后改名 | `TestARevertOfAMoveThatRenamesIsOnePlace`；`revert_test.go` 的一步与 `RelocateNode` 的不分类的一行；`TestARevertPutsBackAnMCPMoveThatRenames`（同步与后台）；撤销的性质测试 |
| 每一步的规则在 domain | 缺的祖先以 `.md` 结尾也补建；类型不同也移动 | `steps_test.go` 的表；四个用例的表格测试 |
| MCP 的删除被本人的会话拒绝 | MCP 也只看别的账户 | `TestTheEditLockGuardsByOperation` 的 `mcp:x` 一行；锁的每一格的第 6 格；`TestAnMCPDeleteMeetsItsCallersOpening` |
| MCP 的删除答出全部的锁、子树的次序 | 只答第一个；按页 id 排 | `TestTheEditLockGuardsByOperation`（改动的次序 Top、Mid、Side、Low，`Locks` 是 Top、Side、Low）；锁的每一格的第 11 格（Notes2、Late、Child） |
| 锁的拒绝带路径 | `edit` 的预检的拒绝不包；路径读不到时答 `internal_error` | `EditPage` 的表格测试的两行（3.8.6）；锁的第 2 格；C4（MCP）第 1 步 |
| `delete` 的 `recursive` 在单元里 | 在单元之前、不加锁地数 | `TestAnMCPDeleteSeesAChildAddedBeforeItsUnit` 的第一种次序 |
| `write` 新建时内容相同不写 | 总答 `title_taken` | `TestTheMCPWritesReachTheUseCases` 的第二次写；MC3 |
| `write` 覆盖时内容相同不写（令牌过时也是） | 先比版本号 | 锁的每一格的第 7 格；`WritePath` 的表格测试（3.8.6） |
| `edit` 只以线性的匹配器找 | 任何一处（查找、计数、CRLF 与 NFC 的再找）用 `strings` 的查找；帮助函数放在别的文件；用 `LastIndex`、`SplitN`、`NewReplacer` 这一族的别的；以 `regexp` 编 `old`；经别名、点导入或当作值的引用 | `TestReplaceStaysWithinItsTime` 的 B 的构造串与两种再找的形状（几秒到几十秒）；`TestReplaceSearchesOnlyWithItsMatcher`（三个包的每个非测试文件、按导入路径认包、明确的表；它的反向对照） |
| `edit` 的匹配器是对的 | 回退表错一位；计数重叠 | `TestTheMatcherIsStrings`（10 万例对照 `strings`） |
| `edit` 的唯一（重叠也算） | 用不重叠的计数 | `page/domain/edit_test.go` 的 `aa` 在 `aaa` 里一行 |
| `edit` 的 CRLF | 不转换；混用的页也转换 | `edit_test.go` 的两行；`TestTheMCPWritesReachTheUseCases` 的 CRLF 一步 |
| `edit` 的 NFC 的提示 | 总给；从不给 | `edit_test.go` 的两行（3.9） |
| `edit` 的大小在拼之前 | 先拼再比；CRLF 之下以没换的串算 | `TestReplaceStaysWithinItsTime` 的 `replace_all` 一种（拼出来约 5 TiB）；`edit_test.go` 的 CRLF 的大小一行 |
| `edit` 的重做一次 | 不重做；重做多次；有 `Base` 也重做 | `TestAnMCPEditRedoesAfterAConcurrentWrite`；`EditPage` 的表格测试的 `KeptChanging` 与"有 `Base` 只读一次" |
| `Extra` 只补问题没有的成员 | 改回先抄 `Extra`；`Extra` 盖过 `code`、`message` | `TestExtraOverridesTheProblemsMembers` |
| `locks` 至多 20 项 | 不截 | `TestTheLocksOfAnErrorAreBounded` |
| 错误经 `Fit` | 不经；`Build(0)` 过界不答 `internal_error` | `TestAnErrorFitsItsItems`、`TestAnErrorOfTheLongestPathFits`；适配器的 `TestEachErrorFitsAtItsWorst`（经 `ErrorAnswerBytes`）；`TestErrorAnswerBytesIsTheAnswer` |
| 路径原样引起 | 改回 `%q` | `text_test.go` 的 U+3000 等四种标题 |
| 码的表的七行 | 少一个码、多一个码 | `TestEveryMCPToolHasItsCodes`；`TestTheMCPToolsErrorsCarryTheirCodes`（每个码都答出过；答了表外的码的调用失败） |
| 工具的字符串到达用例 | 只放这一个参数；表里缺一个工具；账户入口的 `notebook` 在换之后才加 | `TestToolStringsCatchASchemaRefusal`（`strict` 的反向对照，比整张问题的表）；`TestToolStringsDoNotAnswerInternalErrors`（每一项断言最后的参数里有那个 NUL；变异 50 之下 `write`、`edit`、`move` 得到结构表的拒绝） |
| 矩阵的七行与 `after` | 写的行没有 `write`；runner 不调 `after`（计数按 `want`，在调用之前） | `TestThePermissionMatrixCoversEveryTool`；`after` 的计数（`toAfter` 与 `aftered` 不等） |
| 说明的每个工具一行 | 少一行 | `TestTheBuiltInInstructionsNameEveryTool`（它的反向对照 `TestUnnamedToolsCatchesEachGap` 已在） |
| 最后一跳 | 页的适配器交 `ClientAPI` | `TestTheClientNameIsTheTokens` 的 `SELECT client` |
| MC1 第 5 步照 `tools.json` | 写死的名单 | 写死的在 S1 加了工具之后失败；照文件的照旧通过 |
| `log` 的 `actor: "me"` | 当作字面的 id | `TestTheMCPHistoryIsRESTs`；`ParseActor` 的表 |
| `changeset` 只认这本笔记本的 | 照 `readableChangeset` 在变更集自己的笔记本上判定 | `TestTheMCPHistoryIsRESTs` 的别的笔记本；`InNotebook` 的表格测试 |
| 历史的答复的退回 | 不退回；退回的 `next_cursor` 是 `NextCursor`；不写 `changes_cut` | `TestTheMCPHistoryAnswersGoOn`（种下的历史，3.17.3）；适配器的测试（3.17.4） |
| 历史里节点的类型 | 照抄 REST 的 `asset` | `TestTheMCPHistoryIsRESTs` 的附件；适配器的测试 |
| `versions` 的模式 | 读一个版本时收 `cursor` | `CheckVersionsArgs` 的表；码的表的调用 |

## 4. e2e

故事照 00 号文档第 9 节的表（MC3 :546、MC4 :547、MC8 :548，C4 的 MCP 版本 :572）。脚本化的客户端是 P1 的 `mcpClient`（`e2e/fixtures/mcp.ts:108-144`，名称默认 `e2e-agent`，所以变更集的客户端是 `mcp:e2e-agent`）。MC3、MC4 照 10.1 调 `page.ts` 的数据库断言；MC8 不做数据库的断言（00 号文档 :594 的例外），它对照的是 REST 的答复。断言照 00 号文档第 9 节"断言的适配"：比较的是"库里的与答复的"，不是库与它自己（设计审查 C-M6、C-M7）。

### 4.1 断言与 fixture 的改动（S1）

| 文件 | 改动 |
|---|---|
| `e2e/fixtures/mcp.ts` | `mcpClient` 的 `options` 加 `handshake?: boolean`（默认真；假时不发 `initialize` 与 `notifications/initialized`，之后的请求照样带 `MCP-Protocol-Version: 2025-11-25`）：MC3 的"没有重新握手的一次调用"。`McpAnswer` 在故事的帮助函数的签名要它时导出（`mcp.ts:10-12` 的规则，knip 只认别的文件的导入） |
| `e2e/fixtures/assert/page.ts` | `expectNewPage(db, created, creatorId, client, content = "")`：`created` 的类型改为 `Pick<Page, "id" \| "notebook_id" \| "parent_id" \| "name" \| "created_at">`（已有的调用照样传 `Page`），正文、字节数、哈希与版本行的正文照 `content`（`page.ts:15-53` 今天写死空串）。新的 `expectRelocated(db, id, from: Place & { name: string }, to: { parentId: string \| null; name: string }, moverId, at?: string)`：节点最后一个变更集的条目从 `from`（父节点、次序值、名称）一步到 `to`（父节点与名称由测试给出，不从节点的行读），节点的行现在也是 `to`，正文不动；给了 `at` 时变更集的时刻在 SQL 里等于它（`expectMoved` 要名称不变、`expectRenamed` 要位置不变，`page.ts:128-147`、`:59-80`，都不合一步的移动加改名）。新的 `expectChangesetOf(db, id, { client, byId, items, versions })`：变更集的客户端与作者，它的条目恰是 `items` 这些节点的、版本行恰是 `versions` 这些页的（一次 `write` 补建的三个节点、一次 `move` 与它改写的页都在一个变更集里） |
| `e2e/fixtures/assert/links.ts`、`fixtures/pages.ts`、`fixtures/history.ts`、`fixtures/trash.ts` | 不改：`expectIndexedLinks`（`assert/links.ts:34`）、REST 的帮助函数（`pages.ts`、`history.ts:10-52`）、`trashPath`、`trashListed`（`trash.ts:37-50`）照用 |

### 4.2 MC3 写与编辑（`e2e/stories/mcp/mc3-write-edit.spec.ts`，S1）

两个测试。

**"MC3 (MCP)"**：`newOnboardedTeam` 的管理员（PAT 与浏览器的 `tokens`），笔记本 Handbook，`agent = mcpClient(baseURL, pat, { notebook })`：

| # | 动作 | 断言 |
|---|---|---|
| 1 | `write` `Projects/Q3/Plan.md`，正文 `# Plan\n\n- [ ] ship\n- [ ] tell\n`，不带 `base_version` | `changed`、`created` 是三项（`Projects.md`、`Projects/Q3.md`、`Projects/Q3/Plan.md`）、`created_count` 3、`version` 是 Plan 的 `:1`；`expectNewPage` 三次，每次的 `created` 由答复建出（设计审查 C-M6）：`id` 是 `created[i].node_id`，`name` 是路径的末段去掉 `.md`，`parent_id` 是 `created[i-1].node_id`（根下为 null），`notebook_id` 是这本笔记本，`created_at` 是答复的 `at`（13.1 第 9 条的一次读钟）；两个父页的正文为空，Plan 的是这段正文，客户端都是 `mcp:e2e-agent`；`expectChangesetOf`：一个变更集，三个条目、三个版本行 |
| 2 | 同一个路径、别的正文，不带 `base_version` | `isError`，`page.title_taken`，文字含 `is a page already` |
| 3 | 同一个路径、同样的正文，不带 `base_version` | `changed: false`、`changeset_id: null`；`expectVersions` 仍是一个版本 |
| 4 | 浏览器打开 Plan 的页（`wikiPagePath`），阅读视图里有 `ship` | — |
| 5 | `write` 带第 1 步的版本、正文 `# Plan\n\n- [ ] ship\n- [ ] tell\n\nUpdated by the agent.\n`（一个 `ship`、两个 `- [ ]`，第 7、8 步要它们，设计审查 C-N6） | 版本 `:2`；浏览器里的阅读视图不重新加载就显示 `Updated by the agent.`（事件流，13.4 第 3 条） |
| 6 | `write` 再带第 1 步的版本 | `page.revision_mismatch`，`structuredContent.error.current_version` 是 `:2`；`readContent` 不变 |
| 7 | `edit` `ship` → `ship it` | `replacements` 1、版本 `:3` |
| 8 | `edit` `- [ ]` → `- [x]`，不带 `replace_all` | `validation_failed`，文字含 `found 2 times`；带 `replace_all`：`replacements` 2 |
| 9 | REST 建 Notes，正文 `a\r\nb\r\n`；`edit` `a\nb` → `a\nc` | `readContent` 是 `a\r\nc\r\n` |
| 10 | `later = mcpClient(…, { notebook, handshake: false })`，`write` `Later.md` | `expectChangesetOf` 的客户端是 `mcp:e2e-agent`（PAT 记下的名称，00 号文档 4.5） |
| 11 | 浏览器打开历史（`historyPath(slug, notebook, { client: "mcp" })`） | `historyListed` 的每一行以 `MCP (e2e-agent) · ` 起头，行数是 MCP 写的变更集数；Notes 的 REST 的写不在其中 |
| — | 整个测试里 | `expectVersions(db, plan, …)` 的每个版本的客户端是 `mcp:e2e-agent` |

**"MC3 (activity)"**（PG14 的 API 版本的形状，`e2e/stories/page/pg14-notebook-activity.spec.ts:16-43`，2.1 第 16 条）：成员以 `joinAs` 的 PAT 建笔记本 Plans，经 MCP `write` Intro（新建）与 Notes（新建再覆盖）；管理员移出她（`removeMember`）；`ownerlessNotebooks` 列出 Plans，大小是两页正文的字节数，`last_activity_at` 在 SQL 里等于最后那次 `write` 答的 `at`（`SELECT $1::timestamptz = $2::timestamptz`）。PG14 的页面版本照旧（P5 另写 MCP 的一遍，00 号文档 :570）。

### 4.3 MC4 移动、改名与删除（`e2e/stories/mcp/mc4-move-delete.spec.ts`，S1）

管理员的 PAT，笔记本 Handbook；REST 建 Guides、Setup（根下）、Index（正文 `[[Setup]] and [[Setup|the setup]] ![[diagram.png]]\n`），在 Setup 下上传 `diagram.png`（`uploadAsset`，`e2e/fixtures/assets.ts:33`）；Guides、Setup 与附件的 id 取自 REST 的答复：

| # | 动作 | 断言 |
|---|---|---|
| 1 | `move` `Setup.md` → `Guides/Install.md` | 答 `Guides/Install.md`；`expectRelocated(db, setup, {根、次序值、Setup}, { parentId: guides, name: "Install" }, admin, 答复的 at)`（目的地由测试给出：改了名而父节点仍是根的写法在这里失败，设计审查 C-M7）；Index 的正文改写为 `[[Install]] and [[Install\|the setup]] ![[diagram.png]]\n`；`expectIndexedLinks`：两条 wikilink 解析到它；`expectChangesetOf`：一个变更集，条目只有 Setup，版本行只有 Index |
| 2 | `move` `Guides/Install/diagram.png` → `Guides/Install/flow.png` | Index 的嵌入改写为 `![[flow.png]]`；`expectRelocated(…, { parentId: setup, name: "flow.png" }, …)` |
| 3 | `move` `Guides/Install/flow.png` → `Guides/Install/flow`；→ `Guides/Install/flow.md` | 都是 `validation_failed`、字段 `to`：附件改名要带一个扩展名（`CheckAssetRename`）；`to` 与 `from` 的类型不同（3.8.3）。换扩展名（`flow.png → flow.jpg`）是成的（设计审查 B-M6，样例 `035-attachment-new-extension`），由 3.13.6 的整个程序测试做，这里不做：后面的步骤还要 `flow.png` |
| 4 | `move` `Guides/Install.md` → `Guides/Setup guide.md`，`update_links: false` | `readContent(Index)` 逐字节不变；`expectIndexedLinks`：两条 wikilink 的 `resolved` 为 null、嵌入照旧解析到附件（索引照样重新解析，00 号文档 4.10） |
| 5 | `move` `Guides/Setup guide.md` → `Guides.md` | `page.title_taken`，文字含 `to is the full new path` |
| 6 | `delete` `Guides.md` | `validation_failed`，文字含 `1 page and 1 attachment` |
| 7 | `delete` `Guides.md`，`recursive: true` | `pages` 2、`attachments` 1；`expectSubtreeDeleted`（Guides 与 Setup guide）、`expectTrashRoot`（2 页、1 个附件、两层）；浏览器的回收站（`trashPath`）列出 Guides |

### 4.4 C4 的 MCP 版本（`e2e/stories/collab/c4-guard.spec.ts` 加 "C4 (MCP)"，S1）

照 "C4 (API)"（`c4-guard.spec.ts:39-77`）：A、B 的 PAT，各一个 MCP 客户端（`a-agent`、`b-agent`），Team 下的 Notes，A 以 REST 开 Notes 的会话：

| # | 动作 | 断言 |
|---|---|---|
| 1 | B `write` Notes（带 `read` 给的版本）、`edit` | 都是 `page.locked`，`structuredContent.error.lock` 是 `{page_id, path: "Team/Notes.md", user_id: A, display_name: A 的名字, you: false}`；文字说 `by <A 的名字> in the web editor` 与 `cannot take the lock over`。`edit` 的拒绝来自调用之前的锁的预检，同样带路径（3.8.2 第 4.6 步，设计审查 A-M4、B-M2、C-M9） |
| 2 | A `write` Notes | `page.locked`，`you: true`，文字说 `by you, in a browser` |
| 3 | B `delete` `Team/Notes.md`、`delete` `Team.md`（`recursive`） | 都是 `page.locked`；后者的 `locks` 一项（带 Notes 的路径）、`locks_count` 1，`lock` 不带 `path` |
| 4 | A `delete` `Team/Notes.md` | `page.locked`，`you: true`；`expectAliveSessions` 照旧是 A 的会话（与 API 版本不同的一处：MCP 的删除不结束会话，00 号文档 4.10） |
| 5 | — | `readContent` 不变 |
| 6 | B `move` 改名为 `Team/Notes 2.md`、再移到根下；B `write` 同样的正文（带 `read` 给的版本） | 都成；最后一个 `changed: false`（与当前相同的写不经守卫），版本号仍是 1 |
| 7 | A 以 REST 结束会话；A `delete` `Notes 2.md` | 删了；`sessionsOf` 为空 |

### 4.5 MC1 的第 5 步（S1，与 `tools.json` 同一个提交）

`e2e/stories/mcp/mc1-connect.spec.ts:164` 写死的 `["list", "read"]` 改为文件的笔记本入口的名单（`toolLists.notebook` 的名称排序，同第 1 步的 `byName`，`:23-30`）：S1、S2 加工具之后它照旧通过，以后的 Phase 也不用再改（2.1 第 16 条）。这一处随 S1 的第 6 个提交（`tools.json` 变长的那一个），不等 e2e 的提交：否则 MC1 在两个提交之间是红的（设计审查 C-N9）；改过的写法在加工具之前与之后都通过。

### 4.6 MC8 历史的读（`e2e/stories/mcp/mc8-history.spec.ts`，S2）

管理员 A（PAT、`e2e-agent`）与成员 B（`joinAs(…, "member")`，`e2e/fixtures/invitations.ts:106-117`）；笔记本 Handbook（以 `workspace_access: "editor"` 建，`fixtures/notebooks.ts:18`，所以 B 是它的编辑者，设计审查 C-N5）与另一本 Other。A 以 REST 建 Guides；A 经 MCP `write` 建 Plan 与 `Archive/Index.md`（补建 Archive）、覆盖 Plan、`edit`、`move` 到 `Archive/Plan.md`；B 以 REST 把 Guides 改名为 Guide；A 在 Other 里以 REST 写一页（那个变更集给第 5 步）：

| # | 动作 | 断言（对照 REST 的 `listChangesets`、`getChangeset`、`listChanges`、`getVersions`、`getVersion`，`e2e/fixtures/history.ts:19-50`、`pages.ts:202-215`） |
|---|---|---|
| 1 | `log` `{client: "mcp"}` | 行的 id 与次序等于 REST 的 `client=mcp`，每行的 `client` 是 `mcp:e2e-agent`；B 的改名不在其中 |
| 2 | `log` `{actor: "me"}`；`log` `{actor: <B 的 id>}` | 等于 REST 的 `actor=<A 的 id>`、`actor=<B 的 id>`（后者只有改名的那一个） |
| 3 | `log` `{path: "Archive/Plan.md", limit: 1}`，接着 `next_cursor` | 读完的行等于 REST 的 `page_id=<Plan>` |
| 4 | `changeset` 移动的那一个 | `changeset.moves` 为真；`changes` 的一项 `node.path` 是 `Archive/Plan.md`、`node.kind` 是 `page`；其余成员等于 REST 的 `getChangeset` 与 `listChanges` |
| 5 | `changeset` Other 的变更集的 id | `isError`，`page.changeset_not_found`（00 号文档 :548："别的笔记本的变更集答 404"） |
| 6 | `versions` `{path: "Archive/Plan.md"}` | 等于 REST 的版本列表；`current_version` 等于 `read` 给的 `version` |
| 7 | `versions` `{path, revision: 1}` | 两个文字块；元数据的 `revision` 1、`current_version` 同上；第二块是第一次 `write` 的正文 |

### 4.7 e2e 的时间

五个新的测试（MC3 的两个、MC4、C4（MCP）、MC8，设计审查 C-N2）与 MC1 的一行在 e2e 的作业里。`main` 最近两次运行（`39098913463`，`2a888a6f`；`39096175708`，`0c934bb9`）的 E2E 一步是 3:28 与 3:31（时限 20 分钟）；P2 的五个故事都只有几十次调用，预计加起来约一分钟，到约 4:30。实施者把合并时的作业时间写进第 9 节。

## 5. 实施步骤

每个 Step 在自己的分支上，按下面的次序提交；**每个提交之后都是绿的**，不只是 Step 的末尾：`make lint-go test-go`（含 archtest 与 depguard），动了 `web/`、`e2e/` 的提交另加 `make lint-web knip`。几处检查把一个工具的几个部分拴在一个提交里：工具、它在 `tools.json` 里的条目、码的表的行、矩阵的行（写的带 `write`，`after` 计数）、说明的一行与工具的字符串的合法参数的一项（`TestTheToolListsAreTheContract`、`TestEveryMCPToolHasItsCodes`、`TestThePermissionMatrixCoversEveryTool`、`TestTheBuiltInInstructionsNameEveryTool`、`TestToolStringsDoNotAnswerInternalErrors` 让它们分不开）。依赖的规则：page 的 domain 不导入 mcpserver（`MaxNamedLocks` 与 `MaxErrorLocks` 相等由 page 的 `adapter/mcp` 的测试核对）；page 的 `adapter/mcp` 只导入 `page/app`、`page/domain` 与 mcpserver（同 P1）；接缝工具 `hold`、`strict` 只在测试里。

S1 里容易空过的检查（设计审查各自指出的），每个都有能失败的一处：工具的字符串的检查以合法的参数到达用例、有反向对照（3.13.1）；`after` 计数（3.13.2）；名额的测试用一个名额的应用，漏一个看得见（3.2.5）；锁的第 11 格看得到三个锁的次序（3.13.3）；样例 047 的 `recased` 取自 `domain.Relocation`（3.5.3）；`TestAWriteNeverLandsOnAnotherPageAtItsPath` 的 M 与令牌同是版本号 1（3.13.4）；时间的守卫有 B 的构造串，`strings.Index` 让它失败（3.9）；`TestReplaceSearchesOnlyWithItsMatcher` 有合成的源码作反向对照（3.9）。

### 5.1 S1（`m9-p2-s1`）

1. **平台**：httpserver 的大请求体的名额（`largebody.go`：整个服务两个、每个凭证一个、满了时点名持有者的一行；`APIConfig` 的字段、`CodeServerBusy`）与它的测试（3.2.5 的 httpserver 一表）；mcpserver 的 `MaxOtherBytes` 导出、`Extra` 只补问题没有的成员、`MaxErrorLocks` 与 `locks_count`、`ToolError.Fit`（3.10.5 的 mcpserver 一表）。组合根先交零值的名额（`LargeBodies: 0` 不起作用），所以这一步不改行为。
2. **客户端**：`shared.Unshowable`、`Client.Valid`、`IsMCP`、`MCPClient`、迁移 00044、`sqlc.yaml`、`schema_test` 的逐码点的一条查询与两个一致性测试（3.3）。`make gen` 之后生成物没有差异（迁移只改 CHECK）。
3. **domain**：`ParseVersionToken`、`CheckEdit`、`Replace` 与线性的匹配器、`TestTheMatcherIsStrings`、`TestReplaceSearchesOnlyWithItsMatcher` 与它的反向对照、时间的守卫（`Makefile` 的 `test-go-costs` 的 page/domain 一行加它，6.5）；每一步的规则 `CreationOf`、`MoveKinds`、`DeletionOf` 与 `steps_test.go`；`PathTaken`、`VersionMismatch`、`Recursive`、`LockPaths`、`LockedAll`、`AttachmentOnPath`、`PathOfNames`、`MoveParentHint`、`Quoted`，P1 的 `text.go:22` 与四个提示改为原样引起（P1 的文字的测试随之改）（3.4、3.8、3.8.5、3.9、3.10.4）。
4. **单元与存储**：`RelocateNode`（SQL 与 `make gen`）、`Unit.Relocate`（`Placement`）、`Unit.Subtree`、`Unit.DeleteSubtree`、编辑锁对 MCP 删除的规则（子树的次序）、`PathStore.PathsOf`、`NodePaths.Walk`、`NodePaths.Paths`；linking 的 `Relocation`、改名的用例的 `recased` 取自 `domain.Relocation`、样例 `047-move-and-recase`（一步的结果，含手写的 `[[a/x|x]]`）；撤销的一步的放回（`planner.place` 与 `Unit.place`、`moveSubtree` 的写的参数）与 `TestARevertOfAMoveThatRenamesIsOnePlace`、`revert_test.go` 的几行与 `ErrRevertUnordered` 的注释（3.5–3.7）。REST 的用例照旧，M4–M8 的测试照常通过即是"行为不变"的证明。
5. **用例**：`WritePath`、`EditPage`、`RelocatePath`、`DeletePath`（锁的拒绝的路径、预检的包装、路径读不到时的规则）与它们的表格测试（3.8.6）。
6. **工具**：page 的 `adapter/mcp` 的四个工具、错误的文字（`text.go`、`locks.go`）、说明的"Tools"一段、`tools.json` 的八个条目、`page.New` 与 `page.Tools` 的接线、码的表的四行与它们的调用、矩阵的四行与 `after`（计数）、工具的字符串的合法参数的表（`notebooks`、`open_notebook`、`list`、`read` 与四个写的工具；账户入口的 `notebook` 在换之前放进去，每一项断言 NUL 在点名的参数里）与它的反向对照 `TestToolStringsCatchASchemaRefusal`、MC1 的第 5 步（4.5）（3.11–3.13.2）；组合根交上名额 2、等 2 秒与两个门槛（3.2.2），`TestTheLargeBodiesShareTwoSlots` 与边缘的两行一起进来。
7. **整个程序的测试**：最后一跳、锁的每一格、交错、`update_links`、写的工具到达用例、`TestARevertPutsBackAnMCPMoveThatRenames` 与撤销的性质测试的新操作、日志（3.13.3–3.14、3.5.4）。
8. **e2e**：4.1–4.4。
9. **测量、CI 的份与报告**（6.5 的规则；S1 的 CI 的提交）：
   1. 组合根的两份各单独跑两次、取平均，并读 S1 的分支上 CI 的两次运行里两份的测试一步（00 号文档第 11 节第 28 条，2.1 第 22 条照 6.5 改写它）。
   2. 一份过了线（两次运行的平均里 CI 的测试一步过了约 7:30，go test 的 `-timeout 15m` 的一半），照 6.5 的规则：只在一个邻居挪过之后仍在线之下时挪它们之间的界线、把测试给那个邻居，否则按测试名再分一份（`BOOTSTRAP_THIRD`、`test-go-bootstrap-3`、`ci.yml` 的 `bootstrap-3`，界线照 6.5 选、紧接着重新平衡，各份单独跑两次核对）；不提高 `-timeout`。照 6.5 的估计，第二份多半过线；它要给出的是过线的部分（照估计至多约 0:40），而它的邻居第一份 P2 之后约 7:05–7:15、离线只有约 15–25 秒，所以多半分出第三份；第二份只是刚过线（约 7:45–7:55 之下）时挪 `BOOTSTRAP_FIRST`（6.5 的"每一份的邻居"）。界线、每份本机的两次运行与它们的平均、CI 的测试一步写进 `Makefile` 与 `ci.yml` 的注释（同 `m9-job-waits`）与 S1 的审查记录。
   3. 本机的门禁（6.5 的"本机的门禁"）：`test-go` 改为照 CI 的份跑组合根，组合根之外的包与组合根的各份以 `$(MAKE) -j4` 同时跑，之后成本与守卫两份依次；`Makefile:187`、`:198` 的注释随之改。在这个提交上同时的与依次的 `make test-go` 各跑两次、记下每次的用时与平均，写进审查记录；同时的平均不比依次的快 5 分钟以上就改回依次（更简单；单独一次的摆动约 ±1 分钟，比估计的余量大，设计审查的第二次复核 C 的 N11）。
   4. 成本与守卫那两份的行（6.5 的"时限的一半"）：守卫的 markdown 一行的三个测试各单独跑两次、取平均，乘 CI 与本机之比 2.4（守卫那一份的，S3 的五次本机运行的中位数，各次 1.9–2.7，偏高）；这一行的估计过了 5 分钟（Go 默认的每个测试二进制 10 分钟的一半），就照 `test-go-guards` 已有的写法拆成一个测试一行，单独一行仍过 5 分钟的那一行加 `-timeout 15m`，Makefile 的注释写明为什么与数（照 6.5 的估计约 6 分钟，多半要拆）。`test-go-costs` 的 page/domain 一行照同样的规则，并读 CI 日志里两个守卫的 `-v` 输出与这一行的用时（作业的日志要认证，读不到时只记每一步的时间）。数写进 S1 的审查记录。
   5. 大的请求体的内存（6.2）；6.3 的变异逐个跑；结果写进第 9 节。

### 5.2 S2（`m9-p2-s2`）

1. **domain**：`ParseActor`、`CheckRevision`、`CheckVersionsArgs` 与测试。
2. **用例**：`VersionReads`、`HistoryReads` 的新字段，四个入口 `ListVersions.AtPath`、`GetVersion.AtPath`、`ListChangesets.AtPath`、`GetChangeset.InNotebook`（设计审查 C-N3），`Execute` 的后半抽出（M8 的表格测试照常通过），四个入口的表格测试（3.17.4）。
3. **工具**：`versions`、`log`、`changeset`、`history.go` 的形状（节点的类型 `attachment`、`changes_cut`）与适配器的测试（含两种退回），`tools.json` 的六个条目、说明的三行、接线、码的表与矩阵的三行、工具的字符串的合法参数的三项（3.16、3.17.1、3.17.2）。
4. **整个程序的测试**：`TestTheMCPHistoryIsRESTs`（带附件）、`TestTheMCPHistoryAnswersGoOn`（`seedLongHistory`，3.17.3）。
5. **e2e**：MC8（4.6）。
6. **测量与报告**：同 S1 的第 9 步：组合根的各份（S1 分了就是三份）各两次的平均与 CI 的测试一步；一份过了线，照 6.5 的同一条规则（00 号文档第 11 节第 28 条，2.1 第 22 条）：只在一个邻居挪过之后仍在线之下时挪它们之间的界线（`BOOTSTRAP_FIRST` 在第一、第二份之间，`BOOTSTRAP_THIRD` 在第二、第三份之间），否则在 S2 里按测试名再分一份（照第三份的写法，本机的 `-j` 随份数），不提高 `-timeout`。界线、每份本机的两次运行与它们的平均、CI 的测试一步写进 S2 的审查记录。

## 6. 测试与验证

### 6.1 各项与它们的测试

| 项 | 单元与包的测试 | 整个程序 | e2e |
|---|---|---|---|
| 大的请求体的界线（3.2） | 3.2.5 的 httpserver 一表（含两个凭证、一个名额的应用、满了的一行） | `TestTheLargeBodiesShareTwoSlots`（三个 PAT）、`TestTheMCPEdgeAnswersInOrder` 的两行 | — |
| 客户端（3.3） | `client_test.go`、`changeset_test.go` 的表、`schema_test` 的逐码点的查询与两个一致性测试 | 最后一跳（3.13.3） | MC3 的第 10 步 |
| 版本令牌（3.4） | `TestParseVersionToken` | 两个顺序的测试（3.13.4） | MC3 的第 6 步 |
| 单元的操作（3.5） | `unit_relocate_test.go`、`RelocateNode` 的存储测试、`TestRelocationIsWhatARewriteFollows`、样例 047 | `TestTheMCPWritesReachTheUseCases` | MC4 |
| 撤销的一步的放回（3.5.4） | `TestARevertOfAMoveThatRenamesIsOnePlace`、`revert_test.go` | `TestARevertPutsBackAnMCPMoveThatRenames`（同步与后台）、撤销的性质测试 | — |
| 编辑锁（3.6） | `TestTheEditLockGuardsByOperation` 的行 | 锁的每一格（第 11 格的三个锁）、两个交错 | C4（MCP） |
| `Walk`、`Paths`（3.7） | `TestThePathsOfNodesAreTheirPaths`、`TestEveryNodesPathFindsIt` 的一条、`Walk` 的表 | — | — |
| 四个用例与每一步的规则（3.8） | 3.8.6 的表格测试、`steps_test.go` | 3.13.3–3.13.6 | MC3、MC4 |
| `edit` 的匹配（3.9） | `edit_test.go`、`TestTheMatcherIsStrings`、`TestReplaceSearchesOnlyWithItsMatcher`、`TestReplaceStaysWithinItsTime` | `TestTheMCPWritesReachTheUseCases`、`TestAnMCPEditRedoesAfterAConcurrentWrite` | MC3 的第 7–9 步 |
| mcpserver 的错误（3.10） | 3.10.5 的两表、`TestEachErrorFitsAtItsWorst` | 锁的每一格的 `lock` 与 `locks` | C4（MCP） |
| 四个工具（3.11）与说明（3.12） | page 的 `adapter/mcp` 的测试 | 码的表、矩阵（`after` 计数）、清单、说明、工具的字符串的检查与它的反向对照 | MC1、MC3、MC4 |
| 日志（3.14） | 用例的日志测试 | `callLogged` 的一次 `write`；满了的一行在 `TestTheLargeBodiesShareTwoSlots` | — |
| 历史的读（3.16） | 3.17.4 | 3.17.1–3.17.3（种下的历史） | MC8 |

### 6.2 P2 怎样量它的界线

本文没有跑需要数据库的东西（草稿的规则），所以：

- **`edit` 的代价**：离线的 Go 实验量了（`scratchpad/m9/p2/exp/kmp`：KMP 的匹配器、`Replace`、对照 `strings` 的差分 257,204 例、3.9 的表；`exp/nfc`；B 的 `review-B/rk`）。最坏的一次调用是 NFC 的提示把 U+1D160 的页放大三倍到 15 MiB 的那一种，本机 149–162 ms；B 的构造串 1 MiB 在 5 MiB 的页上 9.0–9.1 ms，同一个输入 `strings.Index` 70.2 秒（设计审查 B 的复核实测；12 万字节处 9.85 秒）；CRLF 与 NFC 的再找在构造的形状上 13–18 ms（`strings` 在 12 万字节处约 9 秒，复核 B 实测）；每字节 1.05–1.30 ns，随长度成正比（设计审查 B-I2 要的重量）。CI 上由 `TestReplaceStaysWithinItsTime`（`!race`，`test-go-costs` 的 page/domain 一行，带 `-v`）守着每种形状 2 秒，实施者把 CI 上打出的值写进第 9 节。
- **答复的上限**：照编码算出的最坏（3.10.3、3.16.4，`exp/sizes/worst.py`；错误的文字照设计审查 A 量的 `review-A/sizes-output.txt`），由适配器的测试在替身上以最坏的取值核对（`TestEachErrorFitsAtItsWorst`、3.17.4 的退回）与 mcpserver 的 `TestAnErrorOfTheLongestPathFits`，不是抽样。真的库上的 `TestTheMCPHistoryAnswersGoOn` 核对的是两种退回在种下的历史上都前进、不跳行（53,299 退到 45,321；52,850 退到 29,526 加 23,500，`exp/sizes/seeded.py`；C1 照 REST 的撤销的真样子量，复核 C 的 N3），不是最坏（草稿说它在最坏的取值处核对，不对：P2 与 REST 能写的数据到不了退回，设计审查 C-M3）。
- **内置的说明**：量了（3.12），`TestTheBuiltInInstructionsAreShort` 守着 4 KiB。
- **大的请求体的内存**：3.2.4 是估计；名额的数目是主持者的决定（第 11 节第 16 条），不是量出来的界线。实施者在本机以一次性的测试（不入库）量满载的情形（设计审查 C-M14）：两个同时在上限处的请求体加一个等着的，两种组合：两个 `/mcp` 的 37,814,272 字节的调用；一个 `/mcp` 的与一个 REST 的正文写（5 MiB 的正文，JSON 最贵的转义）。量 `runtime.ReadMemStats` 的峰值与分配，写进第 9 节；负载下的测量是 M12 的（第 8 节）。
- **组合根的各份与 CI 的各份**：6.5 是估计与规则，实施者量（5.1 的第 9 步）。

### 6.3 变异

实施者逐个做下面的改动（每个之后还原），确认右边的测试失败，结果写进第 9 节（照 P1 的 6.3）。每个变异列出的测试都能因它失败（设计审查 C-M10 指出草稿里有四处列了不会失败的一步，已改）。草稿是第 1–41 条，设计审查之后加了第 42–59 条（18 条），复核之后加了第 60–62 条（3 条），第二次复核之后加了第 63 条：

| # | 变异 | 应失败的测试 |
|---|---|---|
| 1 | `largeBody` 不取名额 | `TestTwoLargeBodiesAreServedAtOnce`、`TestTheLargeBodiesShareTwoSlots` 的第 2 步 |
| 2 | 名额在调用处理函数之前就放回（不 `defer`） | `TestTwoLargeBodiesAreServedAtOnce`（处理函数持着时第三个不再 503） |
| 3 | panic 时不放回 | `TestTheSlotComesBackOnEveryPath` 的 panic 一行（一个名额的应用：之后另一个凭证的新请求等满、503）。整个程序里工具的 panic 由 mcpserver 在调用里接住（`mcpserver/call.go:88-94`），走不到名额的 panic 的路径，所以不列它 |
| 4 | 门槛用 `>=` | `TestTwoLargeBodiesAreServedAtOnce` 恰在门槛的一行 |
| 5 | 只看 `Content-Length`（分块的不数） | `TestABodyOfNoLengthTakesASlotPastItsThreshold` |
| 6 | 声明的长度过上限时也排队 | `TestADeclaredLengthPastTheLimitIs413WithoutASlot`；`TestTheMCPEdgeAnswersInOrder` 的 413 一行 |
| 7 | `largeBody` 排在认证之前 | `TestTheSlotsAreTakenAfterTheBuckets` |
| 8 | `Client.Valid` 改回 `unicode.IsControl` | `client_test.go` 的 U+2028、U+202E 与 `MCPClient` 的性质（清理过的名称都 `Valid`，而含 U+2028 的 `mcp:` 不该 `Valid`） |
| 9 | 00044 的范围少 `U+2066-U+2069` | `TestChecksRejectCounterexamples` 的逐码点的查询（8294–8297 被收） |
| 10 | `MCPClient` 不再清理 | `client_test.go` 的性质（随机串经 `MCPClient` 都 `Valid`）。最后一跳的第 6 步不列：`Call.Client` 到达时已清理过（`names.go:51`），再清理一次与否它看不出 |
| 11 | 页的适配器交 `domain.ClientAPI` | `TestTheClientNameIsTheTokens` 的 `SELECT client`；MC3 |
| 12 | `ParseVersionToken` 去掉往返的比较 | `TestParseVersionToken` 的大写、`+1`、`01`、32 位十六进制 |
| 13 | `write` 覆盖时不比令牌的节点 | `TestAWriteNeverLandsOnAnotherPageAtItsPath`（M 静静地被覆盖）、`TestAVersionOfAnotherNotebookWritesNothing`（两页都是版本号 1） |
| 14 | `write` 覆盖时先比版本号再看内容 | 锁的每一格的第 7 格；`WritePath` 的表 |
| 15 | `write` 新建时内容相同也答 `title_taken` | `TestTheMCPWritesReachTheUseCases`；MC3 的第 3 步 |
| 16 | 缺的祖先以 `.md` 结尾时照建 | `steps_test.go` 的 `CreationOf`；`WritePath` 的表 |
| 17 | `write` 在单元之外找路径（不在锁下再找） | `TestAnMCPWriteFindsItsPathUnderTheLock` 的第一种次序 |
| 18 | `Replace` 的唯一用不重叠的计数 | `edit_test.go` 的 `aa`/`aaa` |
| 19 | CRLF 的规则在混用的页上也转换 | `edit_test.go` |
| 20 | 大小在拼出之后才比 | `TestReplaceStaysWithinItsTime` 的 `replace_all` 一种 |
| 21 | `edit` 不重做 | `TestAnMCPEditRedoesAfterAConcurrentWrite`；`EditPage` 的表 |
| 22 | `edit` 带 `base_version` 时也重做 | `EditPage` 的表（有 `Base` 时单元里版本号变了：409 `Changed`，`PageContent` 只读了一次）。交错的带令牌的一步不列：重做读到 r + 1 之后令牌仍不符，答同样的 409 |
| 23 | `edit` 不做锁的预检 | `EditPage` 的表（被锁时不占解析的预算） |
| 24 | `Relocation` 的 `recased` 仍要同一父节点 | 样例 047（改名的用例的 `recased` 取自 `domain.Relocation`，3.5.3）；`TestRelocationIsWhatARewriteFollows` |
| 25 | `Unit.Relocate` 分两步写（先移后改名） | `unit_relocate_test.go`（守卫与参与者各看到一步） |
| 26 | `RelocatePath` 不分改名与移动的动作 | `RelocatePath` 的表（`node.rename` 与 `node.move`） |
| 27 | MCP 的删除照 `OpDelete` 只看别的账户 | `TestTheEditLockGuardsByOperation`；锁的每一格的第 6 格；C4（MCP）的第 4 步 |
| 28 | `lockedAll` 只答第一个 | 锁的每一格的第 11 格（三个锁）；`TestTheEditLockGuardsByOperation`。草稿列的第 9 步只持着一页，看不出 |
| 29 | `recursive` 在单元之前数 | `TestAnMCPDeleteSeesAChildAddedBeforeItsUnit` |
| 30 | `update_links: false` 也交真 | `TestAMoveWithoutItsLinksLeavesThemAndTheIndex` 的第 3 步；MC4 的第 4 步 |
| 31 | `toolError` 改回先抄 `Extra` | `TestExtraOverridesTheProblemsMembers`；锁的每一格的 `lock.path` |
| 32 | `locks` 不截 | `TestTheLocksOfAnErrorAreBounded` |
| 33 | `PathNotFound` 不经 `Fit` | `TestAnErrorOfTheLongestPathFits`；`TestEachErrorFitsAtItsWorst` |
| 34 | 码的表少 `page.locked`（`write` 一行） | `TestTheMCPToolsErrorsCarryTheirCodes`（答了表外的码） |
| 35 | 矩阵的 `write` 一行去掉 `write: true` | `TestThePermissionMatrixCoversEveryTool` |
| 36 | 说明少 `delete` 一行 | `TestTheBuiltInInstructionsNameEveryTool` |
| 37 | `ParseActor` 不认 `me` | `TestTheMCPHistoryIsRESTs`；MC8 的第 2 步 |
| 38 | `InNotebook` 不比笔记本 | `TestTheMCPHistoryIsRESTs`；MC8 的第 5 步 |
| 39 | `log` 不退回 | `TestTheMCPHistoryAnswersGoOn`（种下的 W1）；适配器的测试 |
| 40 | `changeset` 带游标的页也带变更集 | `TestTheMCPHistoryAnswersGoOn`（C1 的第二页过界）；适配器的测试 |
| 41 | `versions` 的列表收 `offset` | `CheckVersionsArgs` 的表；码的表的调用 |
| 42 | `Replace` 以 `strings.Index`、`strings.Count` 找 | `TestReplaceStaysWithinItsTime` 的 B 的构造串（几秒到几十秒，过 2 秒）；`TestReplaceSearchesOnlyWithItsMatcher` |
| 43 | `log` 的退回的 `next_cursor` 是 `NextCursor` | `TestTheMCPHistoryAnswersGoOn`（W2 被跳过，与 REST 的全部不等）；适配器的测试（一页两行） |
| 44 | 退回不写 `changes_cut` | 适配器的测试；`TestTheMCPHistoryAnswersGoOn` |
| 45 | 取不到名额时不等，立即 503 | `TestALargeBodyWaitsForItsTurn`（两个名额都被别的凭证持着，设计审查 A-N6、复核 A 的 NN-3）；`TestACredentialsSecondLargeBodyWaitsForItsFirst`（A 的第二个拿到 A 的第一个还回的名额） |
| 46 | 名额不按凭证（同一个凭证的第二个拿空着的） | `TestACredentialsSecondLargeBodyWaitsForItsFirst`；`TestTheLargeBodiesShareTwoSlots` 的第 5 步 |
| 47 | 满了的一行不点名持有者 | `TestTheSaturatedLineNamesTheHolders` |
| 48 | `Extra` 盖过 `code`、`message` 或 `fields` | `TestExtraOverridesTheProblemsMembers`（设计审查 A-N6） |
| 49 | `EditPage` 不查 `new_string` 里的 NUL（`CheckEdit` 只查 `old_string`） | `TestToolStringsDoNotAnswerInternalErrors`（`new_string` 的 NUL 到了 PostgreSQL，`internal_error`，设计审查 C-I1） |
| 50 | 工具的字符串的检查只放这一个参数（改回 `argumentsAt`） | `TestToolStringsCatchASchemaRefusal`（比整张问题的表：缺 `must` 的那一项另多报一条）；`TestToolStringsDoNotAnswerInternalErrors`（`write`、`edit`、`move` 得到结构表的拒绝；设计审查的复核 A 的 NN-1、C 的 N1） |
| 51 | 矩阵的 runner 不调 `after` | `TestThePermissionMatrixOverMCP` 的计数（`toAfter` 与 `aftered` 不等，设计审查 A-M7） |
| 52 | 撤销的放回改回先移动、后改名（计划查现在的键） | `TestARevertOfAMoveThatRenamesIsOnePlace`；`revert_test.go` 的一步；`TestARevertPutsBackAnMCPMoveThatRenames`（同步与后台都答 500） |
| 53 | `lockedAll` 按页 id 排 | 锁的每一格的第 11 格（Notes2、Child、Late 而不是 Notes2、Late、Child）；`TestTheEditLockGuardsByOperation`（改动的次序 Top、Mid、Side、Low：`Locks` 成了 Top、Low、Side 而不是 Top、Side、Low；设计审查的复核 B 的 BC-M3） |
| 54 | `edit` 的预检的拒绝不包 `LockPaths` | `EditPage` 的表；锁的第 2 格；C4（MCP）的第 1 步 |
| 55 | 锁的路径读不到时答它的错误 | 用例的表（`PathsOf` 失败的一行：照旧 `page.locked`、`Paths` 为 nil、一行 warn） |
| 56 | 历史里节点的类型照抄 REST 的 `asset` | `TestTheMCPHistoryIsRESTs` 的附件；适配器的测试 |
| 57 | 路径改回以 `%q` 引起 | `text_test.go` 的 U+3000、U+00A0、U+200D、U+00AD 的标题 |
| 58 | 匹配器的回退表错一位 | `TestTheMatcherIsStrings` |
| 59 | CRLF 的规则之下以没换的两个串算大小 | `edit_test.go` 的 CRLF 的大小一行 |
| 60 | 等整个服务的名额等满而 503 时不还凭证的名额 | `TestTwoLargeBodiesAreServedAtOnce` 的最后一句（第三个凭证答过 503 之后，它的下一个拿到名额）；`TestTheLargeBodiesShareTwoSlots` 的第 5 步（P3 在第 2、3 步答过 503，它的 `hold` 拿到空着的那个、不是 503；设计审查的复核 A 的 NN-4） |
| 61 | NFC 的再找以 `strings.Contains` 找 | `TestReplaceStaysWithinItsTime` 的 NFC 的再找一种（12 万字节 8.90 秒，过 2 秒；第二轮的守卫没有这种形状、抓不住它，复核 B 的 BC-M1）；`TestReplaceSearchesOnlyWithItsMatcher` |
| 62 | CRLF 的再找以 `strings.Index` 找，写在 page/domain 的另一个文件的帮助函数里 | `TestReplaceStaysWithinItsTime` 的 CRLF 的再找一种（12 万字节 9.00 秒）；`TestReplaceSearchesOnlyWithItsMatcher`（读那个文件；第二轮的检查只读 `edit.go`，复核 B 的 BC-M1） |
| 63 | 账户入口的 `notebook` 在换那一个参数之后才加 | `TestToolStringsDoNotAnswerInternalErrors`（每一项发出之前的断言：换 `notebook` 的那几项最后的参数里没有 NUL；设计审查的第二次复核 C 的 N9） |

### 6.4 门禁

每个 Step 合并之前，本机：`make lint`（`lint-go`、`lint-web`）、`make knip`、`make gen-check`、`make test-go`（竞态检测下的全部、`test-go-costs` 与 `test-go-guards`，`Makefile:180-185`，设计审查 C-N8；S1 起组合根照 CI 的份、各一个二进制，6.5）、`make test-web`、`make build`、`make e2e`，都通过；CI 的各份都是绿的。之后照 M9 的做法：审查（Opus）、修复、核对、合并、推送 `main`。

### 6.5 CI 的余量

`main` 最近两次运行（`39098913463`，`2a888a6f`；`39096175708`，`0c934bb9`）的数（测试一步 / 作业），与各份真正卡住的时限：

| 份 | 两次运行 | 卡住的时限 | P2 加的（按名字） | P2 之后的估计 |
|---|---|---|---|---|
| `bootstrap-1` | 6:46 / 7:09、6:41 / 7:03；测试一步平均约 6:44 | go test 的 `-timeout 15m`（`Makefile:178`；作业的 25 分钟到不了） | `TestAn…` 的七个交错（两种次序的共约 12 次）、`TestAWriteNeverLands…`、`TestAVersionOfAnother…`、`TestAMoveWithoutItsLinks…`、`TestARevertPutsBack…`（同步与后台），`TestEveryMCPToolHasItsCodes` 的行：本机约 +13–18 秒 | CI 约 +20–30 秒，测试一步约 7:05–7:15，在 7:30 之下 |
| `bootstrap-2` | 5:34 / 5:55、7:52 / 8:08；平均约 6:43 | 同上 | `TestThePermissionMatrixOverMCP` 的 104 个写的格（各起一个应用、复制一个库，至多 8 个同时）与 S2 的 78 个读的格；`TestTheLargeBodiesShareTwoSlots`（等候 250 ms，约 2 秒，设计审查 A-N10）；`TestTheEditLockReachesEveryMCPWrite`、`TestTheMCPWritesReachTheUseCases`、`TestTheMCPToolsErrorsCarryTheirCodes`（另一个应用）、`TestToolStrings…` 的两个、`TestTheClientNameIsTheTokens` 的写、边缘的两行、撤销的性质测试的新操作；S2 的 `TestTheMCPHistory…` 两个：本机约 +30–50 秒 | CI 约 +0:45–1:30，测试一步约 7:25–8:10：多半过 7:30，S1 分出第三份 |
| `guards` | 10:33 / 10:51、9:47 / 10:09 | 作业的 25 分钟；每一行另受 Go 默认的每个测试二进制 10 分钟（这一份的行不带 `-timeout`） | 无（markdown 的一行照下面的规则量） | 作业不变：测试一步约 42% 的 25 分钟；markdown 的一行估计约 6 分钟，过了 10 分钟的一半，S1 多半拆成三行 |
| `race` | 8:04 / 8:24、7:04 / 7:24 | 作业的 20 分钟；每个包 `-timeout 15m` | httpserver（名额的测试，各等几十毫秒）、mcpserver、page 的 domain（匹配器的差分 10 万例）与 app、linking 的样例：约 +10–20 秒 | 作业约 7:40–8:45，在 10 分钟之下 |
| `costs` | 测试 3:58、6:22；作业 5:51、9:08（含 lint 与生成物的检查） | 作业的 20 分钟；每一行另受 Go 默认的 10 分钟 | `TestReplaceStaysWithinItsTime`：本机约 3–4 秒，CI 约 8–10 秒 | 作业约 6:00–9:20 |
| `e2e` | E2E 一步 3:28、3:31 | 作业的 20 分钟 | 五个故事（4.7） | 约 4:30 |
| `web` | — | 15 | 无 | 不变 |

**组合根的规则**（主持者为 P2 定的，设计审查 C-M1 与它的复核 C-NM2；00 号文档第 11 节第 28 条照它改写、管组合根的每一份，2.1 第 22 条）：一份**过了线**，是两次运行的平均里它在 CI 的测试一步过了约 7.5 分钟（`-timeout 15m` 的一半）。测试一步含竞态检测的构建，go test 自己跑的比它短，所以按测试一步判断是有意偏严的。**过了线的一份，只在一个邻居挪过之后两次运行的平均仍在线之下时，才挪它们之间的界线、把测试给那个邻居（`BOOTSTRAP_FIRST` 在第一、第二份之间，`BOOTSTRAP_THIRD` 在第二、第三份之间）；否则那个 Step 按测试名把它再分一份，S1、S2 都一样；从不提高 `-timeout`。** 挪之前以本机的数估计邻居挪过之后的 CI 测试一步（移过去的测试在本机的用时乘组合根的 CI 与本机之比，两次运行的平均约 1.6），挪之后以 CI 的两次运行核对，核对出邻居过了线就撤回这次挪动、改为分。界线只在这时与分出一份之后紧接着重新平衡时移动，都凭两次运行的平均（本机每份单独跑两次、CI 的两次运行）。第 28 条原先的"S3 移一次之后不再动"防的是凭一次运行的摆动去挪（单独跑一次的时间前后可差一成以上），不是冻结。界线、每份本机的两次运行与它们的平均、CI 的测试一步写进那个 Step 的审查记录，Makefile 与 `ci.yml` 的注释照旧写量出的数。

- **会落在哪里**：组合根的测试是一个接一个跑的（只有矩阵的格 `t.Parallel()`），所以一份的时间约是它的测试之和。P2 的整个程序测试按名字：`TestA…`、`TestAn…` 落在第一份；`TestThe…`、`TestTo…` 与 `TestReverting…` 落在第二份（`BOOTSTRAP_FIRST` 的 `-skip`）。第二份因此多出约 +30–50 秒（本机），按 CI 与本机之比（两次运行的平均约 1.6，单次 1.35–1.91，设计审查的复核 C 算的），CI 上约 +0:45–1:30。
- **第三份的形状**（照已有的两份的写法；`exp/shards` 核对过：`-run`、`-skip` 的表达式顶层的 `|` 照正则的意思，三份互不重叠、合起来是整个包）：

```make
.PHONY: test-go
test-go: ## 运行 Go 测试，含集成测试与 server/tools（开启竞态检测，不用测试缓存；需要 Docker）
	@# 本机与 CI 同样分份（M9/P2 设计 6.5）：组合根之外的包一个 go test，组合根按测试名几份、各一个 go test 二进制，各受 15 分钟；
	@# 竞态检测的这几个同时跑（-j4，随份数；macOS 的 make 3.81 也有 -j），之后不带竞态检测的两份依次跑，不与竞态检测下的构建抢 CPU。各份之和就是 CI 的各份
	@$(MAKE) --no-print-directory -j4 test-go-race test-go-bootstrap-1 test-go-bootstrap-2 test-go-bootstrap-3
	@$(MAKE) --no-print-directory test-go-costs
	@$(MAKE) --no-print-directory test-go-guards

# CI 的六份：……（:187 起的注释照旧，只把"五份"改为"六份"）
# -run 与 -skip 用同一组表达式：第一、第三份 -run 各自的表达式，第二份 -skip 它们的并，三份之和总是整个包，新加的测试
# 自动落在其中一份；一份过了线时照 M9/P2 设计 6.5 的规则改这些表达式
BOOTSTRAP_FIRST = ^Test([A-D]|E([a-u]|very[A-R]))
# M9/P2 S1：第二份的测试一步过了 go test 的 15 分钟的一半（<量出的数>），照规则按测试名再分一份，不提高 -timeout（M9/P2 设计 6.5）
BOOTSTRAP_THIRD = ^Test(The[F-Z]|T[i-z]|[U-Z])

.PHONY: test-go-bootstrap
test-go-bootstrap: test-go-bootstrap-1 test-go-bootstrap-2 test-go-bootstrap-3 ## 竞态检测下运行组合根的整个程序测试，三份依次（需要 Docker）

# test-go-bootstrap-1 照旧：-run '$(BOOTSTRAP_FIRST)'

.PHONY: test-go-bootstrap-2
test-go-bootstrap-2: ## 竞态检测下运行组合根的整个程序测试中名字既不匹配 BOOTSTRAP_FIRST 也不匹配 BOOTSTRAP_THIRD 的（CI 的一份；需要 Docker）
	cd server && $(GO_TEST_RACE) -skip '$(BOOTSTRAP_FIRST)|$(BOOTSTRAP_THIRD)' ./internal/bootstrap

.PHONY: test-go-bootstrap-3
test-go-bootstrap-3: ## 竞态检测下运行组合根的整个程序测试中名字匹配 BOOTSTRAP_THIRD 的（CI 的一份；需要 Docker）
	cd server && $(GO_TEST_RACE) -run '$(BOOTSTRAP_THIRD)' ./internal/bootstrap
```

```yaml
          - leg: bootstrap-2
            timeout: 25
          # M9/P2 S1：第二份的测试一步过了 go test 的 15 分钟的一半，按测试名再分一份（Makefile 的 BOOTSTRAP_THIRD）
          - leg: bootstrap-3
            timeout: 25
```

  `test-go` 照上面改写（下面的"本机的门禁"）；`Makefile:187` 的"CI 的五份"改为六份，`:198` 的"-run 与 -skip 用同一个表达式：两份之和总是整个包"改为上面的两行（设计审查的复核 C 的 N8）；`ci.yml` 的矩阵在 `bootstrap-2` 之后加 `- leg: bootstrap-3`、`timeout: 25`，组合根那一段的注释写"按测试名分成三份"与量出的数；作业的注释里列的 `make` 目标加 `-3`。
- **界线怎样选**：S1 以一次 `go test -race -json` 的整个组合根（`Makefile:186-199` 的做法）取每个顶层测试的 `Elapsed`；矩阵的父测试的 `Elapsed` 不含它并行的格（Go 的父测试在它并行的子测试跑之前就记下了时间：`exp/partime` 里四个各 0.4 秒的并行子测试，go1.27.1 给父测试记 0.10 秒），所以矩阵另以它的格的第一个开始到最后一个结束计。在今天的第二份上，按 M9/P1 S3 的一次 `-v` 运行（本机，`TestEveryS…` 之后的 201 个）加上 P2 的估计，`The[F-Z]` 起的一段约 124 秒、其余约 120 秒（不含包的启动），两份在一成之内；之前的候选 `T[h-z]` 是 156 对 88。S1 照量出的数再挪，三份各单独跑两次核对，两份新的之差在一成之内为止。以后分出的一份照同样的写法：一个按测试名的表达式、`-run` 它的一个目标与 `ci.yml` 的一份，第二份（不以表达式选的那一份）的 `-skip` 是所有表达式的并；从第一、第三份里分时，它的表达式减去分出的一段。S1 照 `exp/shards` 的做法核对各份互不重叠、合起来是整个包：复核 C 照 go1.27.1 的匹配（`$GOROOT/src/testing/match.go`）把今天的三个表达式用在组合根的 361 个测试函数上，竞态下编译的 356 个各落在一份里（156、102、98），另 5 个在 `//go:build !race` 的文件里、在守卫那一份；第二份的 `-skip` 在顶层的 `|` 处分成两段，各凭顶层的名字决定，跳过的测试的子测试随它。
- **每一份的邻居**（同一条规则）：第一份的邻居是第二份（`BOOTSTRAP_FIRST` 往前挪：第一份按名字排在最后的测试，今天是 `TestEvery…` 的一段，进第二份）；第二份的邻居是第一份（`BOOTSTRAP_FIRST` 往后挪）与第三份（`BOOTSTRAP_THIRD` 往前挪）；第三份的邻居是第二份（`BOOTSTRAP_THIRD` 往后挪）。挪动要让两份在挪过之后都在线之下：收的一份不过线，给的一份也回到线下，否则那个 Step 分。第二份要给出的是它过线的部分，照估计（7:25–8:10）至多约 0:40；第一份 P2 之后约 7:05–7:15，离线约 15–25 秒，只收得下第二份刚过线的那几种情形（第二份约 7:45–7:55 之下）。所以 S1 的第二份过线时多半分出第三份；第二份只是刚过线时，规则把 `BOOTSTRAP_FIRST` 往后挪、不分，那时 S2 的两个 `TestTheMCPHistory…` 落进第二份（没有第三份），S2 照同一条规则再看（设计审查的第二次复核 C 的 N10）。
- **本机的门禁**（设计审查的复核 C 的 C-NM3，主持者的决定）：今天的 `make test-go` 以一个 `$(GO_TEST_RACE) ./...` 跑全部的包（`Makefile:183`），组合根是一个二进制、受同样的 15 分钟：S3 的六次本机运行里它 479–573 秒（7:58–9:33，每次都过了 7:30，`scratchpad/m9/p1s3/test-go-*.log`），P2 再加约 45–70 秒，约 8:45–10:45；第三份对它没有用，之后每个 Phase 的测试也都整个落在这里，它比 CI 的任何一份先到 15 分钟。所以 S1 把 `test-go` 改为与 CI 同样分份（上面的 `make` 一段）：组合根之外的包一个 `go test`（`test-go-race`），组合根的各份各一个 `go test` 二进制、各受同样的 15 分钟，同一条规则管两处，`make test-go` 照旧是 CI 各份之和。耗时的守卫（成本与守卫两份）照旧在竞态检测的二进制之后跑、不与它们同时（CI 的注释的理由：不与竞态检测下的构建抢 CPU，`ci.yml:23`）。
  - **同时还是依次**（本机 18 核、128 GB；以 S3 的日志估计，没有实测）：组合根之外的包最长的一个约 101–103 秒（`page/adapter/http`，四次）；第一份单独跑 236–260 秒（S3 的修复一轮，两次，`halves-f1.log`），P2 之后约 250–280 秒；第二、第三份照 6.5 的界线各约 120–124 秒加包的启动。依次跑约 100 + 265 + 2 × 128 ≈ 620 秒，约 10.5 分钟；以 `$(MAKE) -j4` 同时跑（macOS 的 make 3.81 也有 `-j`），约是最长的第一份 250–280 秒加一点争用（今天组合根的一个二进制与别的 86 个包同时跑，也几乎是两份单独跑之和：479–492 秒对 486–498 秒），约 4.5–5 分钟。依次比同时每次门禁多约 5.5–6 分钟，过了主持者的约 5 分钟：取同时跑。整个 `make test-go` 约 11–14 分钟（今天约 15 分钟：`test-go-f1b` 从 8:42 到 8:57）。S1 两种各实测两次、取平均（5.1 第 9 步）：单独跑一次的时间前后可差一成以上，约是依次的 ±1 分钟，比估计的余量（0.5–1 分钟）大，一次定不了（设计审查的第二次复核 C 的 N11）；同时的平均不比依次的快 5 分钟以上就改回依次。
  - **失败找得到**：make 3.81 没有 `--output-sync`，四个 `go test` 的输出交错；但不带 `-v` 的 `go test` 在包列表的模式里先把一个包的输出存着（`$GOROOT/src/cmd/go/internal/test/test.go:1467-1507`），跑完时以一次 `os.Stdout.Write` 写出（`builderPrintTest`，`:2257-2268`），失败的一段有 `--- FAIL: <测试名>` 与 `FAIL <包>`；make 另打出 `*** [test-go-bootstrap-2] Error 1` 点名那一份，并等别的跑完（`Waiting for unfinished jobs`）。组合根的三份打出同样的 `FAIL …/internal/bootstrap`，是哪一份由 make 的那一行与测试名看出。所以找 `--- FAIL`、`FAIL` 与 `Error` 三种行就知道是哪一份、哪个测试。经管道时（例如 `make test-go | tee`），长过 `PIPE_BUF`（macOS 是 512 字节，`getconf PIPE_BUF /`）的一次写不是原子的，别的份的一行短的 `ok` 可能落进一段长的失败里，这三种行照样找得到（设计审查的第二次复核 C 的 N12）。
- **守卫那一份**：作业的时限够：测试一步 10:33、9:47，作业的 25 分钟的约 42%，P2 不加守卫。它的行另受下一条的规则：最长的一行（P2 之前就有的）是 markdown 的三个测试 `^(TestTheAppsMarkdownCostsAboutItsSize|TestTheAppsMarkdownRendersCheckedHTML|TestTheAppsLinksAreWithinTheirBound)$`，本机 151.2 秒（M9/P1 S3 的一次；五次是 139–200 秒），照下一条的比在 CI 上约 6 分钟，已过 Go 默认的 10 分钟的一半，S1 照规则量、多半拆开。
- **成本与守卫那两份的每一行**（主持者为 P2 定的规则，修订之后的回答）：它们的 `go test` 不带 `-timeout`，每一行另受 Go 默认的每个测试二进制 10 分钟。**P2 碰到的每一行，CI 上的估计过了 5 分钟（这 10 分钟的一半），S1 就照 `test-go-guards` 已有的写法拆成一个测试一行；单独一行仍过 5 分钟的那个测试，那一行加 `-timeout 15m`（同 `GO_TEST_RACE`），Makefile 的注释写明为什么与量出的数。** 组合根的份是再分而不提高 `-timeout`；一个测试的一行分不下去，所以给它竞态检测的那几份同样的 15 分钟。
  - **估计**：这一行的测试各单独跑两次、取平均（本机），乘 CI 与本机之比 **2.4**。这个比是守卫那一份的：`main` 的两次运行里测试一步的平均 10:10（10:33、9:47），比它的五行在本机之和。S3 的五次本机运行是 228.7–317.4 秒（`costs-guards-1.log` 与 `test-go-r1`–`r3`、`test-go-f1b`），2.4 是中位数 254.4 秒的比，各次是 1.9–2.7（设计审查的复核 C 的 N2）。它偏高：分子含每一行的编译与起库，分母只是测试二进制自己的时间。CI 的作业日志里每一行打着 `ok …/internal/bootstrap <秒>`，是这一行在 CI 上自己的时间，但作业的日志要认证才读得到，这里读不到，读得到的只是每一步的时间，所以不用它。结论不随这个摆动变：各次自己的比给 markdown 的一行的估计是 6.0–6.4 分钟，固定的 2.4 乘它在本机的 139–200 秒是 5.6–8.0 分钟，都过 5 分钟。成本那一份自己的比是 1.4–2.3（测试一步 3:58、6:22 比本机之和 169 秒），它的行也用 2.4，偏严。
  - **守卫的 markdown 一行**：S1 把三个测试各单独跑两次（`cd server && go test -count=1 -run '^<名>$' ./internal/bootstrap`），三个的平均之和乘 2.4。照今天的 151 秒约 6 分钟，多半过线：`test-go-guards` 的这一行换成三行，一行一个测试名，注释写三个的数与拆的理由；拆开之后一个测试单独仍过 5 分钟（本机两次的平均约 125 秒以上），它的那一行加 `-timeout 15m`，注释写"本机两次 <a>、<b> 秒，乘 2.4 约 <c> 分钟，过了 Go 默认的每个测试二进制 10 分钟的一半"。
  - **成本的 page/domain 一行**：P2 加的时间的守卫只有一个，`edit` 的 `TestReplaceStaysWithinItsTime`（page/domain，不连库、不带竞态检测）：**并进 `test-go-costs` 已有的 page/domain 一行**，与 glob 的守卫同一个二进制、依次跑：`go test -count=1 -v -run '^(TestReplaceStaysWithinItsTime|TestTheWorstGlobsStopWithinTheirWork)$$' ./internal/modules/page/domain`。这一行今天本机 20.1 秒（glob 的守卫 19.7 秒，M9/P1 S3 的一次），加 `edit` 的约 3–4 秒，乘 2.4 连编译约 1–1.5 分钟，远在 5 分钟之下。照同样的规则，它若过了 5 分钟，`edit` 的守卫单独一行，单独仍过的那一行加 `-timeout 15m`。
  - 拆行与 `-timeout` 随 S1 的 CI 的提交（5.1 第 9 步），量的数写进 S1 的审查记录。P2 加的别的检查（匹配器的差分、`TestReplaceSearchesOnlyWithItsMatcher`、`TestEachErrorFitsAtItsWorst`）是普通的单元测试，在竞态那一份里。
- **竞态那一份**：P2 加的包的测试都是秒级的，离每个包的 15 分钟与作业的 20 分钟都远。

## 7. 完成标准

1. S1、S2 各自合并进 `main`，各自的 CI 是绿的；第 3 节的每一项照写定的做了，偏离写进第 9 节。
2. 第 6 节的测试都在、都通过；6.3 的变异逐个确认过；第 4 节的故事都通过，之前所有 M 的故事照旧通过。
3. **00 号文档**：2.1 的二十二条照写定的改，设计审查之后以 `scratchpad/m9/p2/apply-00m9.py` 落下（每段原文恰好出现一次才改，任何一段不合就什么都不写）；第 12 节的进度表 P2 一行（设计已审查；合并时改为完成）；第 13 节的变更记录一行。
4. **v0.1 总体设计**（`docs/v0.1/v0.1-design.md`）：
   - 6.4（:687）与 8.4（:790）的"为空时只给元数据"改为"为空时图片照样直接放进答复，别的附件只给元数据"（README 的写法，`README.md:148`、`:213`；00 号文档 4.9 与第 11 节主持者的决定第 19 条、P1 的 S2 照此实现，设计审查 A-N8）：这一处与 P1 的实现已经不符，不等 P2 合并，由 `scratchpad/m9/p2/apply-v01.py` 先落下（连同第 15 节的一行）。下面各条随 P2 的合并：
   - 13.1 第 2 条：`domain.MCPClient`（`mcp:` 加再清理一次的名称，空了是 `unknown`）；`Client.Valid` 与两张表的 CHECK 是 `shared.Unshowable` 的同一个集合，CHECK 以显式的范围写出（00044）。
   - 13.1 第 4 条（设计审查 C-M12、A-M3）：七个工具的码的次序（`move` 的 503 在它的 409 之后，2.1 第 3 条）；`/mcp` 的边缘在 bodyshape 之前多两步：413（声明的长度）→ 503（没有名额）；REST 的两个正文写的平台的 503 在正文的检查之前；`changeset` 只认这本笔记本的；`versions` 用错了模式的 422 在游标的 400 之前（4.15 的例外）。
   - 13.1 第 8 条：`Extra` 只补问题没有的成员（另可盖过 `lock`、`locks`、`locks_count`）；`locks` 至多 20 项加 `locks_count`；`ToolError.Fit`。
   - 13.1 第 14 条：逐路由的中间件在限流与结构检查之间多一步"大的请求体的名额"（门槛、整个服务 2 个、每个凭证 1 个、等 2 秒、503 `server_busy` 与 `Retry-After: 1`、声明的长度过上限立刻 413、满了时点名持有者的一行）。
   - 13.1 第 21 条：最后一跳现在写（MCP 的写的 `changesets.client` 是 `mcp:<名>`）。
   - 13.1 第 29 条：删除按账户之外，"MCP 的删除被任何活着的会话拒绝（本人的也算），按子树的次序答出全部，不结束会话"。
   - 13.1 第 31 条的 MCP 一项：`edit` 的 `old_string` 至多 1 MiB、替换之后至多 5 MiB，匹配是线性的；错误的 `locks` 至多 20 项加 `locks_count`，带路径的经 `ToolError.Fit`；大的请求体同时至多 2 个、每个凭证 1 个；`log` 的退回带 `changes_cut`（标出截断）。
   - 13.4 第 1 条（P2 的反向对照，3.21）、第 3 条（C4 的 MCP 版本与 P2 的故事）、第 4 条（`interleavings_mcp_test.go` 的七个）、第 5 条（MCP 的历史等于 REST 的）、第 6 条（七个工具的行，写的四行带计数的 `after`）、第 8 条（`TestReplaceStaysWithinItsTime`；组合根的份与它的规则，本机照同样的份跑；成本与守卫的行的"时限的一半"）、第 13 条（Go 的 `Client.Valid` 与 PostgreSQL 的 CHECK；每一步的规则在 page 的 domain、P3 共用）。
   - 第 15 节的变更记录一行。
5. **README**（MCP 一节与"页面"一节）：工具一条写上七个新工具（"写的工具随 M9 之后的 Phase 加上"一句去掉，改为 P3、P4 的工具随它们的 Phase）；新的"写"一条（`base_version`、补建父页、未改动、`edit` 的唯一与 CRLF、`move` 的 `to` 是完整的新路径、附件可以换扩展名、`delete` 的 `recursive`、被编辑的页拒绝并说出持锁人、不能接管）与"历史"一条（`versions`、`log`、`changeset` 只读，还原与撤销在网页上；MCP 的移动加改名在网页上撤销得了）；"传输"一条加大的请求体的名额与 503；"编辑会话与编辑锁"的"锁"（:131）加 MCP 的删除的规则；"改名与移动改写链接"（:157）加 `update_links: false`；"日志"（:172）加 MCP 的客户端 `mcp:<名>` 与 `mcp` 的筛选；内存一条（:359）加大的请求体的估计（3.2.4）。
6. `server/configs/config.yaml` 的 `server.max_body_bytes` 的注释（3.19，设计审查 C-N12）。
7. **移交**（00 号文档 :451-453：发现它的 Phase 写进目标 M 的 `handoffs/`，收尾时核对；设计审查 C-N14）：第 8 节的 M12 各项写进 `docs/v0.1/M12-release/handoffs/M4-performance.md` 的第 18 项（MCP）；P5 的各项写进 `manual/P5-mcp-clients-checklist.md` 与 P5 的设计输入。
8. 本文第 9 节写上实施的结果（各项的偏离、测量的数、变异、CI 的时间），设计审查与各 Step 的审查记录在 `reviews/`。

## 8. 交给后面的 Phase

- **P3（batch 与多轮撤销）**：
  - 单元里的路径缓存与它在移动、改名、删除之后的作废（P2 的 `Walk` 每段一次点查，2.1 第 1 条）；
  - `PlanBatch` 在模拟的树上调 P2 的每一步的规则（`CreationOf`、`MoveKinds`、`DeletionOf`），不另写一份（3.20）；
  - `batch` 的错误经 `ToolError.Fit` 带出错的一步与它的锁，`page.batch_too_large`；每一步的码的次序与文字照 P2 的四个用例（3.20）；
  - 撤销：一个既移动又改名的放回已在 P2 是一步（`moveSubtree` 加 `RelocateNode`，3.5.4），P3 只做多轮的调度、三条后备、临时名称、写回的 `Base` 与后台的切批；
  - **工具的字符串的检查**（3.13.1）：`batch` 加自己的一项合法的参数（`ops` 里一个合法的 `write`），没有它测试失败；`ops[].path` 等嵌套的参数只换第一个 `op` 里的。P1 第 8 节说"结构表与 NUL 的推导已覆盖嵌套"，不对：每个 `op` 有必填的成员，只放一个参数的调用都是结构表的 `bad_request`，到不了用例（设计审查 C-I1）；
  - 说明的 `batch` 一行（4 KiB 之下约 1 KiB 的余地，账户入口 1,065 字节，与 P4 的四行共用，3.12）；码的表、矩阵（带 `write` 与计数的 `after`）、`tools.json` 的条目照 P2 的写法。
- **P4（grep、search、index、backlinks）**：模块根的 `page.NewNodePaths(pool)` 与 linking 的 `Deps.NodePaths` 复用 P2 的 `NodePaths.Paths`（`PathStore.PathsOf` 经 `LinkTargetsByIDs`），不另写给路径的语句（2.1 第 7 条）；四个工具的说明的行与工具的字符串的合法参数的项；`backlinks` 的来源页带路径同 `changeset` 的 `node.path`；它们的错误照 3.10.3 的规则（文字只引这次调用自己的路径，原样引起）。
- **P5（其余的 MCP 版本与接入文档）**：
  - 人工清单（`manual/P5-mcp-clients-checklist.md`）加：Claude Code、Codex 遇到 `/mcp` 的 HTTP 503 `server_busy`（`Retry-After: 1`）怎样表现（重试、报错还是断开），一个 agent 并行几个大的写时（每个凭证一个名额，第二个等 2 秒）是否照样走得通，其中一次在限速的上行上（例如 1 Mbit/s，两个各约 1 MiB 的并行的 `write`：第二个预期等满 2 秒后 503），记下 agent 看到什么、是否照说明一个接一个地重发（设计审查的复核 A 的 MN-1）；`edit` 的唯一比 Claude Code 的 Edit 严（重叠也算）、不折叠引号，真实的 agent 是否照说明重试；`move` 的 `to` 是完整的新路径，agent 是否写成"移进去"；破坏性的工具在各审批选项下的表现。
  - 接入文档写上：大的请求体的名额（整个服务两个、每个凭证一个，超过时等 2 秒后 503）；**部署建议按路由缓冲请求体**（设计审查 A-I1、复核 A 的 MN-2，2.1 第 6 条）：`/mcp` 与 page 的两个正文路由（`PUT /api/v0/pages/{page_id}/content`、`POST /api/v0/notebooks/{notebook_id}/pages`）缓冲（nginx 的 `proxy_request_buffering on`，它的默认），把慢的发送者挡在应用之外；上传与导入照 README 的部署一节不缓冲（`README.md:357`）；Caddy 默认不缓冲请求体、直接暴露也不，用它们时剩下的风险照旧（复核 A 提到 Caddy 的 `request_buffers` 也许能为一个路由缓冲，离线核对不了：P5 核对过才写）；`base_version` 的用法；锁的拒绝不能接管；历史只读。
  - 其余故事的 MCP 版本照 P2 的断言（`expectRelocated` 带目的地、`expectChangesetOf`、`expectNewPage` 由答复建出的 `created`）。
- **M12（发布）**，写进 `M12-release/handoffs/M4-performance.md` 的第 18 项：
  - **慢的发送者**（剩下的风险，2.1 第 6 条，设计审查 A-I1）：两个凭证（一个账户的两个 PAT 也算）仍占得满两个名额，每个慢的请求体占着它至多 `server.read_timeout` 30 秒，每 30 秒两个请求、每分钟 4 个，按凭证的桶（每分钟 1,200 个）看不到；其间超过门槛的 MCP 调用与超过 1 MiB 的网页保存等 2 秒后 503（网页重试三次，约 11 秒后失败）。info 的一行点名持着名额的凭证，管理员能撤销那个 PAT 或结束会话；`read_timeout` 不改。压测时量，并定要不要按账户而不是按凭证、要不要配置项。
  - **每个凭证一个名额的代价**（2.1 第 6 条，设计审查的复核 A 的 MN-1，主持者接受）：名额在读请求体之前取，上传也占着它：10 Mbit/s 上 5 MiB 约 4.2 秒、1 Mbit/s 上过约 250 KB 就过 2 秒，同一个凭证并行的第二个大的写在慢的上行上等满 2 秒后 503（一般的页不受影响）；压测时与 P5 的限速的一次一起看，再定要不要按凭证排队或放宽等候。
  - 大的请求体的内存在负载下的测量（6.2 的一次性测量之后）；名额与等候的配置项（今天是常量，3.19）。
  - `edit` 的匹配：最坏的一次调用本机约 0.16 秒，守卫在 CI 上的值（第 9 节）；单个 `delete` 没有大小的界线（M12 的性能移交第 14 项，同 REST）。

## 9. 结果

（实施之后填写：每个 Step 的提交、偏离与理由、测量的数、变异的结果、CI 各份的时间、审查与修复。）

要记下的数（设计里先写了预计）：

- P1 的 `page.not_found` 修好之前与之后：11 段的路径上带 `AttachmentHint` 67,590 字节、10 层的页上带 `PageHint` 61,448 字节（今天答 `internal_error`），修好之后去掉提示 33,818 字节（设计审查 A 实测）。
- 组合根的各份：每份单独跑两次的平均、CI 的测试一步；分没分第三份、界线与量出的数（6.5 估计第二份约 7:25–8:10）；本机的 `make test-go` 同时与依次各两次的用时与平均（6.5 估计竞态检测的部分约 4.5–5 分钟对约 10.5 分钟）；守卫的 markdown 一行拆开之后三个测试各自在本机的两次。
- `edit` 的守卫在 CI 上每种形状的用时，与 page/domain 那一行的用时。
- 大的请求体的内存：两种组合的峰值与分配（6.2）。
- 迁移 00044 在已有的行上的用时（设计审查 A 在 20 万行上 Up 9–16 ms）。
- 种下的历史的两种退回的答复的大小（预计 45,321 与 29,526、23,500，3.17.3）。

## 附录：核对过的代码与 SDK

每个决定依据的代码，行号是 `2a888a6f` 的（它的代码与 `0c934bb9` 相同，只多了 S3 的文档；模块的路径在 `server/internal/modules/` 之下，同文首的写法）（2.1 与第 1 节另有的不重复）：

| 决定 | 核对的代码 |
|---|---|
| 写的对象是锁下按路径找到的节点；`PathStore` 的语句在单元的事务里 | `page/adapter/postgres/store.go:42-48`（`queries(ctx)` 取事务）；`page/app/node_paths.go:35-70`（`Find` 每段一次点查）；`page/app/unit.go:197-214`（锁、判定、时钟） |
| 新建不经 `ImportUnit` 的缓存；十段都缺时的语句数 | `page/app/unit_import.go:12-18`；`unit_create.go:28-82`、`unit_place.go:40-63`、`unit.go:264-291` |
| 内容相同不写、不经守卫 | `page/app/unit_content.go:62-66` |
| MCP 的删除的规则与锁的次序 | `page/app/edit_lock.go:44-74`；`page/app/extension.go:38-52`（`Step` 嵌着 `Write`，有 `Client`）；`page/app/unit_delete.go:20-45`；`page/adapter/postgres/store.go:243-263`（子树逐层、每层按次序值与 id）；`page/adapter/postgres/queries/sessions.sql:84-89`；`page/app/unit_session.go:101-111` |
| 锁的拒绝带路径与"是你"；`linking.pages_locked` 的次序 | `page/app/lock_holders.go:27-51`（按页 id 排）；`linking/app/rewrite.go:134-161`；`page/domain/errors.go:125-131`；`mcpserver/errors.go:58-99`（今天 `Extra` 先抄） |
| `locks` 的上限 | `linking/domain/errors.go:22-28`（列出全部）；`identity/domain/user.go:33`（显示名 100 个字符） |
| 错误的答复的量与 `Fit` | `mcpserver/answer.go:11-17`、`:25-96`、`:101-112`、`:117-175`；`mcpserver/errors.go:117-131`（结构表的拒绝的文字，`:126`）、`:166-201`；`mcpserver/tool.go:177-181`、`:298-306`（编码不转义 HTML） |
| P1 的 `page.not_found` 过界；`%q` 的转义 | `page/adapter/mcp/text.go:17-24`；`page/domain/path.go:113-147`；`shared/title.go:13`、`:35`、`:64`；`strconv.Quote` 转义 `IsPrint` 为假的字符 |
| 一步的移动加改名 | `page/app/unit_move.go:21-65`、`:71-84`（`moveSubtree`）、`unit_rename.go:17-50`；`page/adapter/postgres/queries/nodes.sql:143-157`；`page/adapter/postgres/store.go:24-64`、`:330-357`（`nameTaken`）；00042 的两个部分唯一索引；`page/domain/name.go:29-41`（附件的扩展名可以换） |
| 撤销的一步的放回 | `page/domain/revert.go:89-95`（一个变更集一种操作的假定）、`:865-891`（`planner.place`，`:874` 查现在的键）；`page/app/unit_revert.go:124-150`、`:171-182`；`page/app/revert_changeset.go:240-243`；`bootstrap/history_test.go:138`、`bootstrap/background_test.go:35-40`、`:128-132`；`bootstrap/revert_property_test.go:76-170`（`:163` 只把 409 当作跳过）；`page/app/revert_test.go:199`、`:586`（M8 的撤销的单元测试） |
| 大小写的改名跨父节点；一步与两步的显示 | `linking/domain/change.go:61-80`（`recased` 要 `sameParent`）；`linking/app/rewrite.go:47-89`；`linking/domain/rewrite.go:387`；`linking/domain/rename_cases_test.go:88-91` |
| `move` 的 503 在单元里 | `linking/app/rewrite.go:236`（`ParseNow`）、`:134-161`（预检与守卫的 `pages_locked`） |
| `update_links` 只给参与者 | `page/app/extension.go:17-21`；`linking/app/rewrite.go:48-51`；`bootstrap/linking.go:25-38`（观察者照样重新解析） |
| `recursive` 的计数 | `page/domain/tree.go:66-77`（`Counts`） |
| 客户端的检查 | `page/domain/client.go:20-30`；`shared/name.go:34-36`；`shared/client_name.go:10-45`；`00014_page_changesets.sql:12-13`、`00017_page_edit_sessions.sql:14-15`；`page/app/unit.go:172-174`；`mcpserver/tool.go:66-77`；`names.go:51` |
| 版本令牌的解析 | `page/domain/version_token.go:11-18`；`page/domain/version.go:11`；`$GOROOT/src/uuid/uuid.go:35-45`、`:134-140`（32 位十六进制）、`:205-262`（`NewV7` 在进程里单调） |
| `edit` 的上限与匹配 | `bootstrap/mcp.go:24-28`（`/mcp` 的请求体给正文之外的 1 MiB）；`page/domain/content.go:14-29`；`$GOROOT/src/internal/stringslite/strings.go:28-79`（`Index` 过 `bytealg.MaxLen` 走 Rabin–Karp，`:68-75`）；`$GOROOT/src/internal/bytealg/bytealg.go:33`（`primeRK`）、`:71-92` |
| 大的请求体的界线 | `platform/httpserver/api.go:190-214`、`:246-279`、`:322`（凭证的键在 ctx 里）；`platform/httpserver/limit.go:27`；`platform/httpserver/server.go:44`（`ReadTimeout`）；`bodyshape/middleware.go:9-37`；`$GOROOT/net/http/request.go` 的 `MaxBytesReader`（不看 `Content-Length`）；`sdk/mcp/streamable.go:440`、`sdk/mcp/server.go:2136-2165`（处理函数结束之后 `ServeHTTP` 才返回）；`mcpserver/call.go:55`、`:88-94`；`platform/httpserver/problem.go:16-28`（没有 `server_busy`）；`page/adapter/http/limits.go:5-17`；`web/…/stores/page-editing.ts:13-14`、`:336-342`（网页遇到 503 重试） |
| 一个凭证的代价、客户端看到的 503、代理 | `api/modules/page.yaml:51-54`（5 MiB 在 30 秒里要约 1.4 Mbit/s）；`sdk/mcp/streamable.go:2583-2587`、`:2821-2833`（Go 的客户端把 503 当作暂时的错误、不读正文）；`README.md:357`（上传与导入不缓冲）；`v0.1-design.md:127`（Caddy 是默认） |
| 只读的注解 | `mcpserver/tool.go:150-156`；`sdk/mcp/protocol.go:1951-2010` |
| 输入的 schema | `mcpserver/schema.go:21-60`（`uuid.UUID` 的字段不收、可选的 `["null", T]`、`notebook` 在最前） |
| 历史的读 | `page/app/list_versions.go:19-26`、`:48-92`、`:121-133`；`get_version.go:34-59`；`list_changesets.go:14-27`、`:51-108`；`get_changeset.go:12-48`；`list_changes.go:13-63`；`page/app/history.go:16-107`；`page/domain/changeset.go:95-160`；`page/adapter/postgres/queries/history.sql:3-81`、`:63-71`（`node_count` 只数条目）、`:110-130`（`FirstChanges`）；`page/adapter/http/history.go:108-156`（`:144` 的 `asset`）、`page/adapter/http/versions.go:90-103`；`page/adapter/mcp/list.go:83-86` |
| `changeset` 的 id 不是 UUID | `mcpserver/tool.go:212-234`（`takeNotebook` 的零值） |
| 矩阵的写的格与计数 | `bootstrap/permission_matrix_mcp_test.go:27-36`、`:343-400`、`:367-372`、`:385-387`、`:413-416`；`permission_matrix_test.go:158-176`、`:226`、`:279-283`；`permission_matrix_page_test.go:609-622`、`:643-648`；`permission_matrix_seeded_test.go:186-262`、`:630-664`（页的正文、会话只在草稿页上、种下的改名） |
| 码的表与工具的字符串 | `bootstrap/permission_matrix_mcp_test.go:61-105`；`bootstrap/mcp_client_test.go:84`、`:131-152`、`:293-327`；`bootstrap/mcp_reads_test.go:401-451`、`:528-606`；`api/modules/page.yaml:56`、`:146-157`、`:338`、`:370`、`:572-573`、`:605`、`:641-651`、`:803`、`:850`、`:877` |
| 说明 | `page/adapter/mcp/instructions.go:1-48`；`page/adapter/mcp/instructions_test.go:15-27`；`bootstrap/mcp_tools_test.go:171-205` |
| 最后一跳与名称 | `bootstrap/mcp_names_test.go:47-88`；`mcpserver/log.go:48-90`；`names.go:47-99` |
| 锁的每一格与交错的写法 | `bootstrap/page_lock_test.go:20-64`；`interleavings_lock_test.go:192-252`；`interleavings_workspace_test.go:33`、`:108-127`、`:151-220`；`interleavings_page_test.go:44-111`；`page_content_test.go:302-379`；`page/app/unit.go:200-206`（正文写持笔记本行 `FOR SHARE`） |
| e2e | `e2e/fixtures/mcp.ts:8-144`；`e2e/fixtures/assert/page.ts:15-160`、`:176-194`、`:351-369`；`e2e/fixtures/invitations.ts:106-117`；`e2e/fixtures/notebooks.ts:18`；`e2e/stories/mcp/mc1-connect.spec.ts:20-30`、`:164`；`e2e/stories/collab/c4-guard.spec.ts:39-77`；`e2e/stories/page/pg14-notebook-activity.spec.ts:16-43` |
| CI 与各份 | `.github/workflows/ci.yml:20-48`；`Makefile:176-244`；`$GOROOT/src/cmd/go/internal/test/test.go:1467-1507`、`:2257-2268`（不带 `-v` 时一个包的输出先存着，跑完时 `builderPrintTest` 一次写出）；本机的用时 `scratchpad/m9/p1s3/test-go-*.log`、`halves-f1.log`、`costs-guards-1.log` |
