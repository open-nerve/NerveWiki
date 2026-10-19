# M9/P1 平台与读的工具：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M9/P1 平台与读的工具（三个 Step：S1 平台、传输与边缘；S2 路径、`list` 与 `read`；S3 说明与账户） |
| 日期 | 2026-10-17 |
| 状态 | 已完成（S1 合并 `384d9f96`、S2 合并 `a36e92df`、S3 合并 `c9468e5b`）；设计稿已按设计审查与它的复核修订两轮（[P1 设计审查](reviews/P1-design-review.md)，第二轮在它的末节），实施与审查之后与代码不同之处照实改在各节（3.20 列出），结果在第 9 节 |
| 上级文档 | [M9 总设计](00-M9-design.md) 4.1、4.3–4.9、4.14–4.16、第 5、8、9、10、11 节与"移交的落实"；[v0.1 总体设计](../v0.1-design.md) 6.1–6.4、7、8.4、10.1、12、13 |
| 代码位置的写法 | 同 00 号文档：模块写作 `page/app/unit.go:168`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…`、`archtest/…` 在 `server/internal/` 之下；迁移写作 `00012_page_nodes.sql:32`；SDK 写作 `sdk/mcp/streamable.go:400`（`~/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.8.0/`）；jsonschema-go 写作 `jsonschema-go/infer.go:219`（`…/github.com/google/jsonschema-go@v0.4.3/jsonschema/`）；e2e 写作 `e2e/fixtures/server.ts:96` |

---

## 0. 分三个 Step

照 00 号文档第 7 节，P1 分三个 Step，各自一个分支（`m9-p1-s1`、`m9-p1-s2`、`m9-p1-s3`）、一次审查与修复核对、一次合并；后一个从前一个合并之后的 `main` 开始。三个都在服务端与 e2e，没有网页的改动。

- **S1 平台、传输与边缘**：SDK v1.8.0 与 archtest、depguard 的规则；`platform/httpserver/mcpserver`（无状态的 JSON 答复、协议版本的上限、能力与缓存、两个入口与 `notebook` 的解析、`Gate`、只收 PAT、按调用的 ctx、恢复 panic、结果与错误、答复的上限、日志与 SDK 的 logger）；httpserver 的 `ProblemOf` 与 `API.Wrap`；`/mcp`、`/mcp/`、`/.well-known/` 的挂载，`setFixedHeaders` 的前缀，只许对象的请求体，按路由的上限，`no-store`；保留名单与 `reserved_test` 的例外；`api/mcp/tools.json` 与它的检查（S1 时两个入口都是空的清单，3.5）；按工具的权限矩阵的框架与"矩阵覆盖每个工具"；答复的量与填答复的 `Fit`；仓库之外的 TypeScript 客户端试用的传输的几步（S1 的末尾）。S1 没有生产的工具：整个程序的测试经组合根的测试接缝登记测试工具（3.4.9）。
- **S2 路径、`list` 与 `read`**：段的检查与路径的解析、给出（`NodePaths.Find`、`PathOf`），线性的 glob 与它的工作上限，`PageStates`，版本令牌的写法，`list`，`read`（页与附件，asset 的 `AttachmentReads`；上传与读附件时读 WebP 的宽高），`server.public_url`；e2e 的脚本化 MCP 客户端 `e2e/fixtures/mcp.ts` 与 MC2；仓库之外的 TypeScript 客户端试用的读的一步。
- **S3 说明与账户**：`AGENTS` 与说明的缓存、`FrontmatterEnd`、内置的说明；客户端名称（`shared.CleanClientName`、`api_tokens.client_name` 与 identity 的 `NewClientNames`、按 PAT 的缓存）；`notebooks`（`AccountWorkspaces`）与 `open_notebook`；e2e 的 TypeScript SDK 开发依赖与 MC1。

e2e 的客户端与 TypeScript SDK 的依赖不随 S1 进来：S1 没有一个故事用得到它们，knip 会把没人用的导出与开发依赖报成错，S1 就合并不了（2.1 第 9 条）。

## 1. 基线

- 从 `main` 的 `e16a98f7` 开始：M9 的总设计已两次修订、总体设计已为 M9 改过（1.3–12.6），代码与 M8 收尾（`fff7c560`）相同。本文引用的行号都是这一版的。
- **没有 MCP 的代码**：`server/go.mod` 没有 SDK（`go 1.27`，`toolchain go1.27.1`；`github.com/google/uuid` 只作为测试依赖的间接依赖在列，`archtest/binary_test.go` 禁止它进二进制）。标准库的 `uuid` 包是项目的 id 类型。
- **平台的 HTTP**：
  - 根路由器 `httpserver.Router`（`platform/httpserver/routes.go:22-96`），平台链"请求号 → 恢复 → 访问日志 → 固定头"（`middleware.go:29-31`），固定头只给 `/api/` 设 `no-store`（`middleware.go:45-52`）。
  - 逐路由中间件 `API.Middlewares(bodies)` 为生成的代码倒序返回（`api.go:176-187`）：请求信息 → 期限（`BodyLimits` 里的路由另加 `read_timeout`，`api.go:225-238`）→ 请求体上限（`:243-248`）→ 失败闸门与认证（`:263-297`）→ 限流（`limit.go:24-36`）→ 结构检查（`bodyshape/middleware.go:17-37`：读完整个请求体、放回，`Check` 对空的请求体不说话，`bodyshape.go:170-191`）。
  - 错误的出口 `APIErrors.Write`（`apierrors.go:202-242`）：`ProblemError` 照 `problemOf`（未导出，`:254-299`），`MaxBytesError` 413，客户端走了 499 与 debug，请求的期限到了 warn 与 500，别的 error 与 500。
  - 事件流以 `api.LongLived` 挂在根路由器上（`events/module.go:86-90`），是挂在契约之外的先例。
- **认证**：PAT 以 `nwk_pat_` 开头，`Authenticate.personal` 读行、判定、尽力写 `last_used_at`（失败只记 warn，`identity/app/authenticate.go:93-113`），执行者是 `shared.Actor{UserID, APITokenID}`（`shared/actor.go:15-22`）；`api_tokens` 没有记客户端的列（`00004_identity_api_tokens.sql`）。
- **树与正文**：
  - `nodes`：`name` 1–255 字节，`name_key` 是 `shared.TitleKey(name)`（`COLLATE "C"`），兄弟的唯一索引 `(notebook_id, parent_id, name_key) NULLS NOT DISTINCT WHERE deleted_at IS NULL`（`00012_page_nodes.sql:5-38`），另有 `(notebook_id, name_key) WHERE deleted_at IS NULL`（`00019_page_nodes_name_key.sql:5`）。
  - `page_contents`：一页一行，主键 `node_id`，`revision`、`byte_size`、`updated_at`、`deleted_at`（`00013_page_page_contents.sql:5-21`）；`ContentSizes` 只给字节数与时刻、是导出用的存储方法（`page/adapter/postgres/queries/contents.sql:34-38`）。
  - `ListNodes` 读整本笔记本的活的节点（`queries/nodes.sql:30-35`），`domain.PreOrder` 按（`sort_order`，id 的字符串）排兄弟（`page/domain/tree.go:12`）；`Children` 按 `sort_order, id`（`nodes.sql:37-44`）。
  - 读的用例不开事务、不加锁：`ListNodes` 先 `WorkspaceOf`、再判定 `node.list`、再读树（`page/app/list_nodes.go:24-47`）；`GetPageContent` 经 `readable`（先找节点、再判定 `page.read`，`readable.go:13-37`）读正文、版本号与哈希（`get_page_content.go:24-38`）。
- **附件**：`asset_blobs` 一个附件一行，`mime`、`byte_size`、`sha256`、可空的 `width`/`height`（两者都有或都没有，`00027_asset_asset_blobs.sql:8-25`）；服务端只读 PNG、JPEG、GIF 的宽高（`asset/domain/types.go:107-111` 的 `HasSize`，`asset/adapter/sniff/sniff.go` 只导入这三个解码器），WebP 的行没有宽高，`golang.org/x/image` 不在依赖里；`Reads.Get` 找节点、判定 `asset.read`、读行、读链接写法、签名（`asset/app/reads.go:57-91`）；签名一到两小时（`asset/adapter/mac/sign.go:34-37`），地址是相对的、由 HTTP 适配器拼（`asset/adapter/http/upload.go:186-194`）；字节经 `Blobs.Open`（`asset/blobs.go:124-134`），文件丢了的下载记 warn、答 404（`asset/app/content.go:67-76`）。
- **笔记本与工作区**：`ListNotebooks` 按工作区判定 `notebook.list`、以 `ReachedByAccess` 筛、以 `EffectiveNotebookRole` 定角色（`notebook/app/list_notebooks.go:27-49`）；`Memberships.WorkspacesOf` 的语句 `ListWorkspacesOf` 已经读出 `slug` 与 `name`（`workspace/adapter/postgres/queries/workspaces.sql:12-20`），`Membership` 只留下 id 与角色（`workspace/memberships.go:29-32`、`:49-59`）。
- **保留名单**：`mcp` 在 `[reserved]`（`workspace/domain/reserved_slugs.txt:29`），`[server]` 是 `api`、`assets`、`healthz`、`readyz`（`:19-23`）；`bootstrap/reserved_test.go:18-36` 要根路由器每个模式的第一段与 `[server]` 相同。
- **整个程序的测试**：`TestFreeTextParametersDoNotAnswer5xx` 由契约推导（`bootstrap/contract_test.go:247-272`）；权限矩阵以会话的访问令牌按列调用（`permission_matrix_test.go`，笔记本级 13 列 `:112-118`），`TestThePermissionMatrixCoversEveryOperation` 与它的反向对照 `TestMatrixViolationsCatchesEachGap`（`permission_matrix_coverage_test.go:152-160`）。组合根的 `wireApp` 最后一个参数是给测试的 `transfer.Batch`（`bootstrap/wire.go:46-52`），唯一交它的测试是 `interleavings_workspace_test.go:78`。
- **SDK v1.8.0**（设计审查 A 已实测，本文 `scratchpad/m9/p1/exp/` 的实验补了几处，见附录）：无状态时 GET、DELETE 答 405 `Allow: POST`（`sdk/mcp/streamable.go:381-386`），在那之前先查 `Mcp-Protocol-Version`（不认识的答纯文字 400，`:347-355`），空的请求体答纯文字 400；`getServer` 每个 POST 调两次（`:347-359`、`:400-403`）；每个方法的 ctx 是这个 POST 的 ctx 包上 `notDone`（值看得到，期限与取消没有，`sdk/internal/jsonrpc2/conn.go:813-821`）；接收的中间件包住每个方法，含 `initialize`（`sdk/mcp/shared.go:206-225`），可以改 `*InitializeResult` 的 `Instructions`；每个方法（含 `tools/call`）都在 jsonrpc2 的 `handleAsync` 起的 goroutine 里（`sdk/internal/jsonrpc2/conn.go:684-688`；`Async` 只放开队列，`sdk/mcp/server.go:2003-2005`），没有 `recover`；无状态时通知到不了中间件：SDK 答 202 就关了临时的会话，之后的读循环什么也不收（P1 的审查 A-M12 实测，3.4.1），所以取消只来自客户端结束它的 HTTP 请求；`initialize` 的 `ClientInfo` 与任何请求的 `_meta` 都经 `ClientInfo()` 读到，握手之后的调用读到的是 nil（合成的参数没有它）；工具按名称排序列出（`sdk/mcp/features.go:76-81`），同名的后登记的悄悄替换前一个（`:40-44`）；结果的 JSON 在线上把 `<`、`>`、`&` 写成六个字节的转义（客户端解码之后照原样）。

## 2. 目标与范围

做的事对应 00 号文档第 7 节 P1 一行（三个 Step）与第 3 节第 1–4、10、12、13 条的 P1 部分：

- **S1**：
  - SDK 固定 v1.8.0；archtest：SDK 与 jsonschema-go 只由 `platform/httpserver/mcpserver` 导入；depguard 连测试文件也管（只许 mcpserver 与 bootstrap）。
  - httpserver 导出 `ProblemOf`（REST 与 MCP 共用的一个分类）与 `API.Wrap`；`setFixedHeaders` 给 `/mcp` 与 `/.well-known/` 设 `no-store`。
  - mcpserver：注册表的类型（含最小的 `Prompt`）、输入 schema 的推导与整理、参数的结构检查与解码、两个 `*mcp.Server`、无状态的 handler、SDK 之前的 403 与 `notebook` 的判定、按调用的 ctx、恢复 panic、结果与错误、答复的量（`TextBytes`、`MemberBytes`）与填答复的 `Fit`、答复的上限、答复没写时的 500、`no-store` 的写者、日志与 SDK 的 logger。
  - 组合根：`/mcp`（`API.Wrap`、只许对象的结构表、按路由的上限）、`/mcp/` 与 `/.well-known/` 的 404；`mcp` 移到 `[server]`、`reserved_test` 对 `.well-known` 的例外；`mcpTools` 与 `api/mcp/tools.json` 的检查；`wireApp` 的测试接缝。
  - 整个程序的测试：PAT、访问令牌、`notebook` 的 404、边缘的次序、JSON-RPC 的批、panic、期限与断开、`no-store`、协议版本的协商、旧协议的 `subscriptions/listen`、`/.well-known/`、服务重启之后继续调用、接线、按工具的权限矩阵的框架；TypeScript 客户端在仓库之外试传输。
- **S2**：page 的路径（段的检查、解析、给出、`NodePaths.Find`）、线性的 glob 与 `MaxGlobWork`、`PageStates`、版本令牌、新用例 `ListByPattern` 与 `ReadPath`、page 的 `adapter/mcp`（`list`、`read`）；asset 的 `AttachmentReads`、`Deps.PublicURL` 与 WebP 的宽高（`sniff` 读 RIFF 头、`HasSize`）；`server.public_url`；性质测试、上限处的测试、时间的守卫与计划测试；e2e 的 `mcp.ts` 与 MC2；矩阵的 `list`、`read` 两行；TypeScript 客户端在仓库之外试 `list` 与 `read`。
- **S3**：`platform/markdown.FrontmatterEnd`；page 的 `Instructions`、说明的缓存、内置的说明、`MCPEntry()`、`OpenNotebook`（计数与最后的变更两条语句、只在根下的 `FirstChildren`）与按剩下的预算填的 `open_notebook`；`shared.CleanClientName`；迁移 `00043`（`api_tokens.client_name`；S2 的修复一轮占了 00042）与 `identity.NewClientNames(pool)`；mcpserver 的名称（来源、按 PAT 的缓存、记下）；workspace 的 `Membership` 带上 slug 与名称、notebook 的 `AccountWorkspaces` 与 `ListAccountNotebooks`、`notebooks`；G16 的五种换法、"列表等于逐项判定"；e2e 的 `@modelcontextprotocol/sdk` 1.32.1 与 MC1；矩阵的 `notebooks`、`open_notebook` 两行。

不做（P2 及以后）：写的工具与版本令牌的解析（P2 写 `ParseVersion`，本文只定写法）；`domain.MCPClient` 与 `changesets.client` 的 `mcp:<名>`（P2，名称的来源与记下在 P1）；`Client.Valid` 与两张表的检查（P2）；`versions`、`log`、`changeset`（P2）；`batch`（P3）；`grep`、`search`、`backlinks`、`index`（P4；`NodePaths.Paths`、`page.NewNodePaths(pool)` 与 linking 经它的最后一跳也在 P4）；真的 prompt（M10；P1 S1 只有最小的 `mcpserver.Prompt` 与它的替身，第 8 节）；接入文档与人工清单（P5）。

### 2.1 对 00 号文档的修订

P1 的文档提交随这份文档一起改 [`00-M9-design.md`](00-M9-design.md)（主持者以 `scratchpad/m9/p1/apply-00m9.py` 落下，每段原文恰好出现一次才改）。行号指 `e16a98f7` 的，理由在本文各节。第 1–13 条是草稿的，按设计审查改过的在条末注明；第 14–22 条是设计审查之后加的；第 23–31 条是 S1、S2 的实施与审查之后加的，第 32、33 条是 S3 的，随它们的合并改进 00 号文档（主持者以 `scratchpad/m9/p1docs/m9_doc.py`、`m9_s3.py` 落下，同样每段原文恰好出现一次才改），与第 13、15 条不同之处以它们为准。

1. **`notebooks` 的工作区不要 `Summaries`**（4.1"`notebooks` 的读"一行 :81；第 2 节 :35；第 7 节 P1 一行 :426；第 8 节 :506）：`ListWorkspacesOf` 今天就读出 `w.slug`、`w.name`（`workspace/adapter/postgres/queries/workspaces.sql:12-20`），只是 `memberships.WorkspacesOf` 把它们丢了（`workspace/memberships.go:49-59`）。改为 `workspace.Membership` 加 `Slug`、`Name` 两个字段，`AccountWorkspaces` 经组合根只接 `Memberships.WorkspacesOf`：一条语句、不加新的端口与查询。别的调用者（access 的事实、事件流的工作区）只读 id 与角色，加字段不影响它们（3.17.1）。第 2 节改为"workspace：`Membership` 带上 slug 与名称"，第 8 节同。
2. **`CleanClientName` 的次序与截法**（4.5"清理与检查" :142）：今天写的"按 NFC、先去 `unshowable`、再去空白……"不幂等，答回的也不一定是 NFC：`e`、U+200E、U+0301 按 NFC 不变（U+200E 是起始字符，挡住组合），去掉 U+200E 之后剩下 `e` 加 U+0301，第二次清理把它组合成 `é`。P2 的 `domain.MCPClient` 会对记下的名称再清理一次，两次不同就让 PAT 上的名称与变更集的客户端各说各的。改为：去掉不是 UTF-8 的字节 → 去掉 `unshowable` 的字符 → NFC → 去首尾的空白 → 截到 128 个字符（第 129 个字符之前是规范化的边界时，即它是起始字符、不与前面的组合（`norm.NFC.PropertiesString(string(r[128])).BoundaryBefore()`），截在第 128 个之后；否则截在前 128 个字符的 `norm.NFC.LastBoundary` 处：截下的仍是 NFC、至多 128 个字符，也不多丢一个。设计审查 A-M5 量出草稿在前 128 个字符上取 `LastBoundary` 时 129 个 ASCII 只留 127 个；第一轮修订改为在前 129 个字符上取它，复核 A N-1 量出第 129 个字符之后也是边界时（汉字、数字、`-`、emoji）留下 129 个，`client_name` 的检查就拒绝每次的 `Record`）→ 再去一次首尾的空白；空了答 `""`，由调用者换成 `unknown`。性质测试：幂等、至多 128 个字符、没有 `unshowable`、是 NFC，全是起始字符的长名称恰好留下 128 个（起始字符取自字母、汉字、谚文、数字、标点与 emoji，不只 ASCII 的字母）；表格里有 128 个 `x` 加 `中`、129 个 `中` 两行，都留下 128 个（3.16.1）。
3. **参数不合 schema 的文字用字段的路径**（4.7 的表"参数不合 schema"一行 :170）：`bad_request: <JSON 指针>` 改为与 REST 相同的字段路径（`ops[3].path`）。参数由 bodyshape 的同一个检查核对（3.3.3）：问题至多 16 个、路径至多 256 字节、从不复述取值；jsonschema-go 的校验错误把实例的值写进消息（`jsonschema-go/validate.go` 的 "type: %v has type %q"），而工具的参数里有正文。文字是 `bad_request: The arguments do not match the tool's input schema. (path: required: is required; …)`，`fields` 同 REST 的 `FieldError`；路径为空（`arguments` 本身不是对象）写作 `arguments`（设计审查 A-N5）。
4. **账户入口里的"先过 `Gate`"就是用例自己的判定**（4.6"调用里的笔记本" :149）："两个入口里，每次调用都先过一次 `Gate`"与 4.15 的通用次序（"取值 422 → 笔记本 `notebook.not_found`"）在账户入口上相悖：mcpserver 在工具之前判定，就把 404 排到了路径的 422 之前。改为：笔记本入口在 SDK 之前经 `Gate` 判定（HTTP 的 404）；账户入口的 mcpserver 只把 `notebook` 解析成 id（不是 UUID 时是零值 id，用例不查库就答 `notebook.not_found`），判定由用例自己做，同一条规则（`node.list`、`page.read`，都是读者以上），排在与资源无关的 422 之后。两个入口里用例都照常判定，笔记本入口的判定因此做两次（第二次是用例的），接受：一次点查。
5. **`ProblemOf` 带 ctx**（4.1"平台包" :73）：`ProblemOf(err)` 改为 `ProblemOf(ctx, err) Answer`：分辨"客户端走了"（`context.Canceled` 而请求的 ctx 已结束）与"期限到了"要看 ctx（今天的 `Write` 看 `r.Context()`，`apierrors.go:232-237`）。MCP 交调用的 ctx（3.4.2）。
6. **`read` 的几处写定**（4.9 :216；4.7"文字的工具" :166）：元数据另有 `kind`；`lines` 是这次给出的行数（被切开的一行算一行）；第二块文字是正文里的一段原样的字节（从 `offset` 行的 `column` 起，到最后一个整行的 `\n` 为止，切开的一行没有 `\n`），所以依次读完的各段连起来就是正文；`offset` 超过末尾时 `column` 只能是 0（否则 422 `column` `out_of_range`，没有那一行）；`limit` 没有上限（字节数管着）；`offset`、`limit` 小于 1、`column` 小于 0 是与资源无关的 422（3.11.1）。
7. **一个量：每块文字在答复里由 `encoding/json` 写出的字节**（4.6"文字" :153、"`open_notebook`" :156；4.7"答复的上限" :164；4.14 :343、:351；设计审查 B-I1、A-M4 改写了这一条）：
   - **量**：答复的上限数每块文字在答复里由 `encoding/json` 写出的字节：`"`、`\`、`\n`、`\r`、`\t`、`\b`、`\f` 两个字节；别的 U+0000–U+001F 与 U+2028、U+2029 六个；U+007F 一个；C1 控制字符两个（原样的 UTF-8）；文字块由 SDK 以 `json.Marshal` 写，`<`、`>`、`&` 也是六个（3.4.4 的表，go1.27.1 实测）。4.6 :156 的"别的控制字符六个"把 `\r`、`\t` 也算成六个，CRLF 的页每行少四个字节的预算：更正为这张表（A-M4）。
   - **结构的工具**的文字块就是结果的 JSON 文字，所以它的字符串转义两次：成员里的引号与反斜杠四个字节、换行三个、别的控制字符七个。这对线上是诚实的。
   - **一个帮助函数**：mcpserver 的 `Fit` 按这个量填 `list`、`notebooks` 与 `open_notebook` 的根下节点，适配器从不自己数（3.4.4）。
   - **说明的 32 KiB**（握手）用同一个量、含末尾的说明行；那行说的是这一页现在的标题（`AGENTS` 按标题键找，可以叫 `agents`）："(The rest of agents.md is cut here: read it whole with read, path "agents.md".)"（3.15.3）。
   - **`open_notebook`** 按剩下的预算填，说明在先：说明截到"48 KiB 减去骨架、再留 8 KiB 给根下的节点"之内（按成员的量，通常约 40 KiB，带说明行），再放根下的节点直到放满（至多 100 个）。4.6 :156 的"说明至多 32 KiB，概况至多 16 KiB……合计在答复的 48 KiB 之内"不成立（32 + 16 已是 48，键名在其上，引号还要转义两次），4.14 :351 的"概况 16 KiB"去掉（3.17.3）。
8. **`api/mcp/tools.json` 的形状**（4.1"工具的契约" :83）：`{"notebook": [...], "account": [...]}`，每个数组就是那个入口的 `tools/list` 答复的 `tools`（名称、标题、说明、输入 schema、注解），按名称比较；比较的是答复里整个工具对象（多一个成员也失败），文件恰好有这两个键；三个 Step 各写到它那时的工具：S1 两个都是空的数组（3.5）。先写的全文在本文 3.18，审查读它。
9. **e2e 的客户端与 TypeScript SDK 随第一个用它的故事，试用分两次**（第 7 节 P1 一行 :426；4.16；第 10 节）：`e2e/fixtures/mcp.ts` 随 S2 的 MC2，`@modelcontextprotocol/sdk` 随 S3 的 MC1；每个导出随第一个导入它的故事（knip 只认别的文件的导入，4.1）。仓库之外的 TypeScript 客户端试用的传输的几步（GET 与 405、重启、401 与 `/.well-known/`、403）在 S1 的末尾、合并之前：它们只要 S1 的握手与空的 `tools/list`；调 `list` 与 `read` 的一步在 S2 的末尾（设计审查 A-M14、B-M14：草稿整个放在 S2 的末尾，S1 的传输与边缘就先进了 `main`、S2 建在它们之上）。
10. **矩阵的 PAT 经接口建**（第 9 节"权限矩阵" :528）："种子加 `api_tokens` 的行"改为笔记本级的 13 列的账户在 MCP 矩阵测试自己的准备里各经 `POST /api/v0/me/api-tokens` 建一个 PAT（`bootstrap/contract_test.go` 已有的 `createToken`）：令牌的格式与哈希只在 identity，测试不另抄一份；多 13 次带当前密码的请求，只在这个测试里（共用的 `prepareMatrix` 有六个使用者，不进它；设计审查 A-N8、B 的 Nit 4）。
11. **固定头的前缀写定**（4.4"答复头" :127）：`setFixedHeaders` 对 `r.URL.Path == "/mcp"`、以 `/mcp/` 开头或以 `/.well-known/` 开头的设 `no-store`；不写 `HasPrefix(path, "/mcp")`：它也盖住 `mcp-team` 一类工作区的页面（slug 只保留 `mcp` 本身）。
12. **段的检查的码**（4.8"段的检查" :186）：照 `shared.CheckTitle` 的码，多数是 `invalid_format`，Windows 保留的名称是 `not_allowed`、长于 255 字节是 `too_long`、空的标题（`.md` 一段）是 `required`；`CheckTitle` 收下而首尾有空白的段（它先去掉空白）是 `invalid_format`，"segment N must not start or end with white space"（设计审查 B-M7）；消息前加第几段（3.9.1）。
13. **路径的点查是两条语句**（4.8"每段按 `shared.TitleKey` 比较" :188）：根下 `parent_id IS NULL`、别处 `parent_id = $`，都走兄弟的唯一索引；不写 `IS NOT DISTINCT FROM`：它不能用索引（今天的 `Children` 那样写，`nodes.sql:42`，靠的是 `notebook_id` 的前缀）。不用一条递归的语句：递归的语句整体做计划，没有统计时退成每层扫表（`nodes.sql:74-80` 的注释，M6 收尾 FA-M1、FA4-M1）；至多 11 次点查（3.9.2）。`open_notebook` 的根下节点同理写 `parent_id IS NULL`（设计审查 A-M16、B-M3，3.17.3）。
14. **上传与 `read` 读 WebP 的宽高**（4.9"`read`"的附件 :217；第 2 节 asset :32；4.14 :348）：今天服务端只读 PNG、JPEG、GIF 的宽高（`asset/domain/types.go:107-111`），WebP 总是 0，`read` 就从不内联它，与 4.9、第 11 节的取舍第 7 条相悖（设计审查 B-I3）。不加依赖（`golang.org/x/image` 不在 `go.mod` 与模块的缓存里）：`asset/adapter/sniff` 在 `image.DecodeConfig` 旁边自己读 RIFF 头（VP8、VP8L、VP8X 三种块），不登记 `image` 的格式；`HasSize` 加 `image/webp`。**这改了 M7 的上传的行为**：新上传的 WebP 的行有了宽高，REST 的答复与网页随之看得到它们；同一条路也是 Obsidian 导入的（`transfer/app/import_write.go:229` 经 `asset/blobs.go:79` 到 `asset/app/blobs.go:82-84`），导入的 WebP 也有了宽高；阅读视图对嵌入的 WebP 写出 `width`、`height`（`platform/markdown/obsidian/asset.go:157-164`，经 `asset/embeds.go:56`），加载时页不再跳动。都是想要的（第 7 节的文档改动，复核 B N-3）。之前上传的、没有宽高的行不迁移：`read` 从它为内联读的字节里读出宽高，已知而在上限之内才内联（3.11.2）。第 11 节的那一条不变。
15. **glob 是线性的，一次 `list` 的匹配有工作的上限**（4.8"glob"的代价 :209；4.9"`list`" :214；4.14 :345；第 9 节 :516；第 11 节的取舍第 6 条不变）：4.8 的"两指针算法……不是线性的，有界……4–8 秒……每匹配一个节点查一次 ctx"改为：每段一个位并行的自动机（Shift-And）、段之间一个自动机、按树走时每个节点从父节点接过状态，代价是键的字节数乘有界的字数，线性；RE2 也是线性的，但它每个节点从头跑整条路径、不共享祖先（每条 1.5–1.8 ms，3.10.1）。一次 `list` 的匹配至多 `MaxGlobWork` = 4 亿个单位（一个字乘键的一个字节；每项另 256，在进入父节点时为它的全部项算上），到了就停、答 `next_cursor`（同答复到 48 KiB 时的意思，这一页可能是空的）；接着走时跳过游标之前的项、只重走游标的祖先，所以每次调用的工作有界、`next_cursor` 总是往后。本机（主机另有负载）到上限时约 0.3–0.8 秒，慢 2.5 倍的主机约 2 秒。守卫用设计审查 B 的最坏形状（`**/*P*/**`，P 是长的、带 `?` 的片段）与它的守卫树（10 万条路径、各 11 段、每段 255 字节），键各由 1、2、3 个字节的字符（`a`、`é`、`漢`）组成，另从末尾附近的游标接着走一次（3.10.1，设计审查 B-I5，复核 B N-1、N-2、N-8）。
16. **无状态时通知什么也不做，取消只来自客户端结束请求**（4.3"每次调用的 ctx" :111）：SDK v1.8.0 的无状态模式答了一个通知的 202 就关了临时的会话，通知到不了中间件（设计审查 A-M12 实测：40 个里只到了一个）；名称、日志、取消都不能依赖通知，`notifications/cancelled` 取消不了在另一个 POST 里的调用。一次调用只在客户端结束它自己的 HTTP 请求（或停机）时取消，经调用的 ctx 上的 `AfterFunc(请求的 ctx, cancel)`（3.4.1）。4.3 写的"请求的 ctx 结束（客户端断开、停机）时取消"不变，加这一句。
17. **`open_notebook` 的"最后改动"是树的最后一次写**（4.6"`open_notebook`" :156）："最后一个变更集的时刻"改为"最后改动的时刻：这本笔记本的树的最后一次写，即变更集的最新 `updated_at`（网页的笔记本活动里页的那一部分，`activity.sql`；编辑会话之后的写会把它往后挪）；没有变更集时为 null"，单独一条语句（设计审查 B-I2、B-M12，3.17.3）。网页的活动还从笔记本这一行自己的 `updated_at` 起（`notebook/app/extension.go:125-152`），所以空的笔记本在网页上有时刻而这里是 null，给笔记本改名挪动网页的时刻而不挪这里的：这里说的是内容（复核 B N-4）。
18. **入口的 `notebook` 照 REST 解析，查询串解析不了答 404**（4.6"入口" :148；第 9 节 :521）：`r.URL.Query()["notebook"]` 改为 `url.ParseQuery(r.URL.RawQuery)`：`Query()` 丢掉解析不了的那一对（`notebook=%zz`、`;` 分隔），客户端就悄悄落到账户入口，正是这一条要防的（设计审查 A-M1）；解析出错答 404，不查库。"是 UUID"照 REST 的 `uuid.Parse`（大写、花括号、`urn:uuid:` 都收），日志记规范的小写（设计审查 A 的过度设计第 2 条，3.2.4）。
19. **`NodePaths` 的按 id 给路径随 P4**（第 7 节 P1 一行 :426、P4 一行 :429）：P1 只要按路径找节点（`read`）；按 id 给路径与模块根的 `page.NewNodePaths(pool)` 的唯一使用者是 P4 的 linking，随 P4 来（设计审查 B 的过度设计第 1 条，3.9.2）。
20. **`mcpserver.Prompt` 是最小的**（4.1"注册表的类型" :77）："`mcpserver.Prompt` 同形"改为"`mcpserver.Prompt` 是最小的：名称、标题、说明、字符串的参数（MCP 的 `PromptArgument`）与返回说明和文字消息的处理函数，M10 按需要在注册表上加"（设计审查 A-M7 与 A 的过度设计第 1 条，3.4.7）。
21. **SDK 的导入规则也管测试文件**（4.1"SDK 的版本" :84）：archtest 的图不含测试文件，另加一条 depguard 的规则，SDK 与 jsonschema-go 只许 mcpserver 与 bootstrap 之下的文件导入（设计审查 A-N10，3.7）；`segmentio/asm` 是 v1.2.1（goose 要它，MVS 取较新的；A-N6）。
22. **边缘的次序写细**（4.4"边缘的码的次序" :122；4.15 :364）：认证与限流照 REST 的四步（没有令牌 401 → 失败闸门 429 → 坏的令牌 401 → 桶 429，`api.go:269-277`）；不是 POST 的先过 SDK 的协议版本的检查（不认识的版本答纯文字 400）再 405；POST 的协议版本在 404 之后由 SDK 查（设计审查 A-N1，3.2.2）。
23. **`POST /register` 答 OAuth 的错误**（4.4 的 `/.well-known/` 一条、"SDK 之前的拒绝是 problem+json"、答复头、保留名单；4.16；第 5 节；第 11 节主持者的决定第 13 条；S1 的试用的第 4b 步）：会走 OAuth 的客户端找完 `/.well-known/` 的元数据还去源站 `POST /register` 注册，拿到网页的 405，看不到提示；`/register` 答 400 `application/json` 的 `invalid_client_metadata`，说明是 PAT 的提示，只服务 POST，公开、契约之外，`register` 进 `[server]`（3.2.2）。
24. **没有令牌的 401 带同一句提示**（4.4"SDK 之前的拒绝"；4.16）：httpserver 的 `APIConfig.NoTokenDetails`，只给没有令牌的 401，`/mcp` 的 detail 是 `/.well-known/` 的 404 的那句；失败的令牌与 REST 的 401 不变（3.2.3）。
25. **超过 64 KiB 的只能是工具的调用**（4.4"请求体与期限"与边缘的码的次序；4.14 的请求体一行；4.15 的边缘）：这个入口登记了的工具的 `tools/call`、`jsonrpc` 是 `"2.0"`、`id` 至多 256 字节、`_meta` 的数 float64 放得下时才过，别的 413，在 403 之前；`params` 的形状在 SDK 之前查；`prompts/get` 因此至多 64 KiB（3.2.2）。
26. **大的请求体的内存**（第 10 节的一行；"写给后面的 M"的 M12）："36 MiB 时约 150 MiB"改为实测的约 1.4 GB 的分配、约 +0.3–0.5 GB 的峰值，P2 限为同时两个（第 8 节）。
27. **路径的点查用 00042 的两个部分唯一索引**（4.6 的说明的缓存；4.8 的"同一个路径至多一个节点"与"按段一次点查"；第 10 节）：取代第 13 条的"都走兄弟的唯一索引"；去掉 00012 的兄弟的索引；`ChildByKey` 没有统计时的计划接受（3.9.2）。
28. **`MaxGlobWork` = 3 亿个单位**（4.8 的 glob 的代价；4.14 的 glob 一行；第 10 节）：取代第 15 条的 4 亿；守卫加键是名字两倍长的 `և` 的树，续走的重进占 17%、约 58.5 万个子节点的文件夹吃掉整个预算（3.10.1）。
29. **没有 `server.public_url` 时图片照样内联**（4.2 的配置；4.9 的附件；4.14 的内联的图片一行；第 11 节取舍第 7 条的写法）：内联不要地址，只有下载的链接要；别的附件只给元数据并说明（3.11.2）。
30. **WebP 的宽高只在头合得上文件时给、被拒的 `public_url` 带 `@` 的不照录**（4.2、4.9；第 13 节的配置一条）：RIFF 的大小、第一个块的大小、VP8L 的版本、VP8X 的块的大小（3.11.2）；没有源站带 `@`（3.12）。
31. **超过 1,024 字节的路径由父节点列出**（4.8 的"以它为模式的 `list` 恰好列出它"）：性质限定为 `MaxGlobBytes` 之内的路径，`too_long` 的消息指向 `"A/B/"` 与 `read`（3.10.1）。
32. **说明的缓存存副本，`AgentsPage` 的版本号是子查询**（4.6 的说明的缓存）：缓存的是 `strings.Clone` 之后的正文：`AgentsBody` 答的是 pgx 为整个正文分配的一个字符串（至多 5 MiB）的一段，原样缓存时每项钉住整页，"256 项、每项至多 32 KiB"不成立（S3 审查 A-I1，3.15.1）；`AgentsPage` 的版本号是对找到的根的子查询，不是联接：ANALYZE 之后一个常见的键的联接读每本笔记本的正文（实施时的偏离 22，3.15.1）。
33. **说出的名称立刻记住，到知道在行里为止每次调用再记下**（4.5 第 2 步与两个客户端的一段；第 10 节的一行；S3 审查 B-M1、B-M2、C-M2，修复核对第一轮 B 的 N-1）：取代第一轮的"写了一行才缓存"：说出的名称（清理之后不空）立刻为它的 PAT 记住，标出还不知道在行里；写了一行，或 0 行之后一次点读看到行里是它，才去掉标；标没设时这个令牌的每次调用都再记下；已知在行里的同一个名称不写也不读。两个客户端在同一刻报不同的名称时，进程与行可以停在不同的名称上，直到报出缓存之外的名称、这一项被挤出或进程重启；不加按令牌的锁（3.16.4）。

### 2.2 需要负责人确认

无。

- 第 11 节负责人的四个决定照做：`AGENTS` 按标题键找、去掉 frontmatter、约 32 KB 截断、没有时给简短的内置说明，缓存键经得起替换、改名、恢复（3.15）。
- 内置的说明只写英文（3.15.4）：照 13.5 第 4 条（Go 的字符串用英文）；MCP 的请求不带区域设置（`initialize` 的 `clientInfo` 只有名称与版本），没有选语言的依据；工具的说明与 schema 也是英文；网页的中英文是按浏览器给人看的，agent 读英文最稳。团队要中文的约定就写一个 `AGENTS` 页，它整个取代内置的说明。
- 设计审查之后的几条也不碰第 11 节：WebP 的宽高（2.1 第 14 条）让取舍第 7 条（"PNG、JPEG、GIF、WebP 在 3 MiB、每边 8,000 像素之内内联"）真的成立，那一条不改；答复的量（第 7 条）只写定 48 KiB 怎么数，48 KiB 不变；glob 的工作上限（第 15 条）不改取舍第 6 条的方言。
- 2.1 的各条是实现的写定或 00 号文档内部的不一致，都不碰第 11 节的决定，由主持者定（主持者在 [P1 设计审查](reviews/P1-design-review.md) 里的决定，负责人可以改判）。
- S1、S2 的实施与审查里主持者定下的，同样负责人可以改判，写进 00 号文档第 11 节（主持者的决定第 13–23 条）与两份审查记录的"主持者的决定"：[S1](reviews/P1-S1-platform-review.md) 的 `POST /register`、没有令牌的 401 的提示、超过 64 KiB 的只能是这个入口的工具的调用、内存的界线给 P2、`id` 的回显也限住；[S2](reviews/P1-S2-reads-review.md) 的两个部分唯一索引与去掉兄弟的索引、`ChildByKey` 没有统计时的计划、不论有没有 `public_url` 都内联图片、WebP 的检查、`MaxGlobWork` = 3 亿、守卫那一份的时限 25 分钟、`BOOTSTRAP_FIRST` 到 S3 再动。第 11 节取舍第 7 条只改写法（下载的地址要 `public_url`，内联不要），取舍本身不变。
- S3 的实施与审查里主持者定下的，同样负责人可以改判，写进 00 号文档第 11 节（主持者的决定第 24–30 条）与 [S3 的审查记录](reviews/P1-S3-account-review.md)的"主持者的决定"：说明的缓存存副本；客户端名称照 B 的方向（说出的立刻记住、到知道在行里为止再记下，热行的规则保住）；两个客户端的镜像的交错只改设计的那一句、不加按令牌的锁；截在最后一个换行、只由看不见的字符组成的名称、客户端走了的 warn 留给 M12 的打磨；组合根两份的界线不再动，以后判断挪不挪取两次的平均；MC1 不加非管理员的角色；任务的等待与竞态那一份的时限在只改测试的分支里改。

## 3. 服务端

### 3.1 S1 的文件

| 文件 | 包 | 新/改 | 内容 |
|---|---|---|---|
| `server/go.mod`、`go.sum` | — | 改 | 直接依赖 `github.com/modelcontextprotocol/go-sdk v1.8.0` 与 `github.com/google/jsonschema-go v0.4.3`（mcpserver 直接导入它）；新加的间接依赖 `github.com/segmentio/encoding v0.5.4`、`github.com/segmentio/asm v1.2.1`、`github.com/yosida95/uritemplate/v3 v3.0.2`、`golang.org/x/oauth2 v0.35.0`、`golang.org/x/time v0.15.0`。这是 `go mod tidy` 真选的（离线，go1.27.1，在 `exp/tidy/` 的仓库副本上加一个导入 `mcp`、`jsonrpc` 与 `jsonschema` 的包；与设计审查 A 的结果相同）：SDK 的 `go.mod` 要 `asm` v1.1.3，而 goose v3.28.0 要 v1.2.1，MVS 取后者（A-N6）；`golang-jwt/jwt/v5 v5.3.1` 已在，`x/sync`、`x/sys`、`x/tools` 仍是仓库的版本；`go.sum` 只多这七个模块 |
| `platform/httpserver/api.go` | httpserver | 改 | `API.Wrap(h, bodies)`（3.2.1）；`APIConfig.NoTokenDetails`：按路由的模式，没有令牌的 401 的说明（3.2.3） |
| `platform/httpserver/bodyshape/middleware.go` | bodyshape | 改 | `Bytes(r)`：中间件读过、放回的请求体，不复制（`sizeError` 读它，3.2.2） |
| `platform/httpserver/apitest/problems.go` | apitest | 改 | `(*Contract).ProblemCodes(t)`：契约声明的每个码（顶层与每个操作的 `x-problem-codes`，要令牌的操作加 `unauthorized`），整个程序以它核对工具错误的码（3.6） |
| `platform/httpserver/apierrors.go` | httpserver | 改 | `ProblemOf(ctx, err) Answer`、`Fault`；`Write` 改用它，行为不变（3.3.6） |
| `platform/httpserver/middleware.go` | httpserver | 改 | 导出的 `MCPPath` 与 `RegisterPath`；`setFixedHeaders` 的 `no-store` 加 `/mcp`、`/mcp/…`、`/.well-known/…` 与对 `/register` 的 POST（3.2.3） |
| `platform/httpserver/mcpserver/server.go` | mcpserver | 新 | `Options`、`New`、`Server.ServeHTTP`、两个 `*mcp.Server`、`getServer`、SDK 的选项（3.3.1、3.2.2） |
| `…/mcpserver/tool.go` | mcpserver | 新 | `Entry`、`Annotations`、`Spec`、`Tool`、`NewTool`、`Call`、`Result`、`Block`、`Image`、`Link`；登记与包装（3.3.2–3.3.4） |
| `…/mcpserver/schema.go` | mcpserver | 新 | 输入 schema 的推导与整理、整理之后的 schema 转成 `bodyshape.Table`（3.3.3） |
| `…/mcpserver/errors.go` | mcpserver | 新 | `ToolError`、工具的错误答复（3.3.6） |
| `…/mcpserver/call.go` | mcpserver | 新 | 请求的值、按调用的 ctx、恢复 panic 的接收中间件（3.4.1–3.4.2） |
| `…/mcpserver/nostore.go` | mcpserver | 新 | `noStore` 写者（3.4.3） |
| `…/mcpserver/answer.go` | mcpserver | 新 | `MaxAnswerBytes`、`TextBytes`、`MemberBytes`、`Fit`、答复的上限（3.4.4） |
| `…/mcpserver/prompt.go` | mcpserver | 新 | 最小的 `Prompt`、`PromptArgument` 与它们的登记（3.4.7） |
| `…/mcpserver/log.go` | mcpserver | 新 | 调用与握手的日志、SDK 的 logger（warn 以上，3.4.5） |
| `…/mcpserver/edge.go` | mcpserver | 新 | `const Pattern = httpserver.MCPPath`，函数 `Bodies()`、`Subtree()`、`WellKnown()`（两个 404）、`notPAT()`、`notebook` 的解析（3.2）；`NoOAuth`、`RegisterPattern` 与 `Register()`（OAuth 的注册的错误）；超过 64 KiB 的请求体的 `sizeError`（`maxOtherBytes`、`maxIDBytes`，以 `encoding/json/v2` 读）与 `askedEntry`（3.2.2） |
| `…/mcpserver/*_test.go` | mcpserver | 新 | 单元测试：schema 的整理与结构表、参数的 `null`、结果与错误的形状、从不同时返回结果与错误、量与 `Fit`、答复的上限、ctx、答复没写、恢复、`no-store`、`notebook` 的解析、SDK 的 logger、`Prompt` 的替身（3.4.8） |
| `page/mcp_entry.go`、`page/app/gate.go` | page、page/app | 新 | `(*page.Module).MCPEntry()`：`Gate`（S3 加 `Instructions`），用例 `NotebookGate`（3.3.5） |
| `bootstrap/mcp.go` | bootstrap | 新 | `mcpTools(pg, ln, nb)`（S1 返回空；S2 加 asset，是 `mcpTools(pg, as, ln, nb)`）、`mcpServer(cfg, …)`、凭证的适配 `mcpCredentials`、`/mcp`、`/mcp/`、`/.well-known/` 与 `POST /register` 的挂载（`mountMCP`）、`mcpPublicOperations()`（`/register` 是公开的）与 `mcpNoTokenDetails()`（3.2） |
| `bootstrap/wire.go`、`app.go` | bootstrap | 改 | `wireApp` 的最后一个参数改为测试的接缝 `wiring{batch, mcpTools}`；`/mcp` 的按路由上限在 `wire.go` 里（调 `apiConfig` 的地方）并进 `BodyLimits`，公开的操作加 `mcpPublicOperations()`，`NoTokenDetails` 交 `mcpNoTokenDetails()`；`app.mcpTools` 留着 `wireApp` 建的模块的工具，接线的测试读它；挂载在模块注册之后、`/` 之前 |
| `bootstrap/interleavings_workspace_test.go` | bootstrap | 改 | `wireApp` 的调用改交 `wiring{batch: …}` |
| `bootstrap/reserved_test.go`、`workspace/domain/reserved_slugs.txt` | bootstrap、workspace/domain | 改 | `mcp` 从 `[reserved]` 移到 `[server]`，`register` 加进 `[server]`（OAuth 的注册，3.2.2）；以点开头的第一段（`.well-known`）不比较，理由写在测试里；名单头的注释（`:9`"mcp 随 M9"）改为说它已在 `[server]` |
| `api/mcp/tools.json` | — | 新 | `{"notebook": [], "account": []}`（S1；3.5） |
| `bootstrap/mcp_tools_test.go` | bootstrap | 新 | `TestTheToolListsAreTheContract`、`TestTheMCPToolsBuildBothServers` 与反向对照（3.5） |
| `server/.golangci.yml` | — | 改 | depguard 的一条规则：SDK 与 jsonschema-go 只许 mcpserver 与 bootstrap 之下的文件（含测试）导入（3.7） |
| `bootstrap/mcp_test.go`、`mcp_client_test.go` | bootstrap | 新 | 整个程序的边缘、传输与恢复（3.4.9）；官方 Go 客户端的帮助函数（令牌经 `HTTPClient.Transport`） |
| `bootstrap/ratelimit_test.go`、`contract_test.go` | bootstrap | 改 | `TestTheMCPRoutesTakeTheBuckets`（`/mcp` 与 `/register` 的桶，3.4.9）；`createTokenWithID`（撤销的 PAT 的一例） |
| `bootstrap/permission_matrix_mcp_test.go` | bootstrap | 新 | 按工具的矩阵的框架、`TestThePermissionMatrixCoversEveryTool` 与反向对照（3.6） |
| `archtest/rules_test.go`、`rules_cases_test.go` | archtest | 改 | `mcpSDKStaysInMCPServer` 与它的合成的边（3.7） |
| `configs/config.yaml` | — | 改 | 只改注释：`server` 一节"长连接路由（SSE、MCP 流）自己解除写上限"改为"长连接路由（SSE）"，`/mcp` 是普通的请求与答复（`configs/config.yaml:7`；`platform/config/config.go` 没有这条注释） |
| `Makefile` | — | 改 | `BOOTSTRAP_FIRST` = `^Test([A-D]\|E([a-u]\|very[A-Q]))`：组合根两份的界线移到 `TestEveryR` 之前（6.1） |

### 3.2 S1：挂载与边缘

#### 3.2.1 `API.Wrap`

```go
// Wrap wraps h, a route registered on the root router outside the
// generated code, in the per-route middlewares in their order (M1/P2
// design 3.3): request meta → request deadline → body limit → failure gate
// and authentication → rate limit → body structure, the last innermost.
// bodies holds the route's body schema under its pattern; it must not be
// nil (bodyshape.Middleware reads its Roots).
func (a *API) Wrap(h http.Handler, bodies *bodyshape.Table) http.Handler {
	for _, m := range a.Middlewares(bodies) { // reversed for the generated code: the last wraps outermost
		h = m(h)
	}
	return h
}
```

`Middlewares` 的倒序与生成的代码套的方式（列表的最后一个在最外层）原样不动，`Wrap` 只是同一个套法写在一处：手拼时容易把认证与结构检查套反。测试：`Wrap` 之后没有令牌又坏的请求体答 401（不是 400），带令牌而超限的请求体答 413，各中间件的次序与 `Middlewares` 给生成的代码的相同（同一张表、同一个请求，两种挂法的答复相同）。`bodies` 不能为 nil：`bodyshape.Middleware` 读 `t.Roots`（`bodyshape/middleware.go:20`），`Wrap(h, nil)` 会 panic（设计审查 A-N2 实测）；表里没有这个模式的根时它照今天直接放过（`:20-22`）。`/mcp` 总交 `mcpserver.Bodies()`。

#### 3.2.2 挂载与边缘的次序

组合根（`bootstrap/mcp.go`），在模块的 `Register` 之后、`router.Handle("/", …)` 之前：

```go
router.Handle(mcpserver.Pattern, api.Wrap(mcp, mcpserver.Bodies())) // "/mcp", no method: GET and DELETE reach the SDK's 405
router.Handle(mcpserver.Pattern+"/", mcpserver.Subtree())            // "/mcp/…": 404 problem+json, not the frontend
router.Handle("/.well-known/", mcpserver.WellKnown())                // 404 problem+json: no OAuth here
router.Handle(mcpserver.RegisterPattern, api.Wrap(mcpserver.Register(), &bodyshape.Table{})) // "POST /register": OAuth's error, the PAT hint
```

```go
// Pattern is the route of both entries on the root router; httpserver
// owns the string, since its fixed headers key on it (3.2.3).
const Pattern = httpserver.MCPPath

// Bodies is Pattern's body structure: one object, whose params, when
// there are any, are an object: its name a string, its arguments and its
// _meta objects, and no inputResponses or requestState, which answer an
// input-required result that no protocol version of this server's has.
// An array (a JSON-RPC batch), a member named twice and an unpaired
// surrogate are 400 before the SDK (M9 design 4.3, 4.4); so are params
// the SDK could not read into a request's, whose error repeats them,
// escaped to four times their size (M9/P1 S1 review A-M1). Null is the
// member left out, as the SDK reads it. A function, not a package
// variable: each caller gets its own table (gochecknoglobals).
func Bodies() *bodyshape.Table {
	const (
		message = iota
		params
		name
		object
		null
	)
	open := func(types bodyshape.Type, props map[string]int) bodyshape.Node {
		return bodyshape.Node{Types: types, Props: props, Extra: bodyshape.Open, Items: bodyshape.Open}
	}
	return &bodyshape.Table{
		Nodes: []bodyshape.Node{
			message: open(bodyshape.Object, map[string]int{"params": params}),
			params: open(bodyshape.Object|bodyshape.Null, map[string]int{
				"name": name, "arguments": object, "_meta": object, "inputResponses": null, "requestState": null,
			}),
			name:   open(bodyshape.String, nil),
			object: open(bodyshape.Object|bodyshape.Null, nil),
			null:   open(bodyshape.Null, nil),
		},
		Roots: map[string]int{Pattern: message},
	}
}

// Subtree answers every path below Pattern; WellKnown every path under
// /.well-known/. Both are 404 problem+json.
func Subtree() http.Handler
func WellKnown() http.Handler

// NoOAuth is what a client looking for OAuth is told, under /.well-known/
// and at RegisterPattern alike, and the detail of Pattern's 401 without a
// token (httpserver.APIConfig's NoTokenDetails).
const NoOAuth = "This server has no OAuth: MCP takes a personal access token in the Authorization header."

// RegisterPattern is OAuth's dynamic client registration at the origin,
// where a client turns when no metadata names an authorization server
// (the default endpoints of MCP 2025-03-26). Register answers it with
// OAuth's error, 400 invalid_client_metadata (RFC 7591 3.2.2), whose
// error_description is NoOAuth. It reads no body.
const RegisterPattern = http.MethodPost + " " + httpserver.RegisterPath
func Register() http.Handler
```

`Bodies()` 的形状是 S1 审查 A-M1 之后的：`params` 是对象或 null，`name` 是字符串，`arguments` 与 `_meta` 是对象或 null，`inputResponses`、`requestState` 只能是 null（它们回答之后的协议版本的 input-required 结果），所以 SDK 读一个 `tools/call` 的参数不会失败；不是对象的 `arguments` 是边缘的 400，不再是工具在 `arguments` 处的 `bad_request`。`Items: Open` 不起作用（根是对象，bodyshape 不读它），无害。

`bodyshape.Middleware` 按 `r.Pattern` 找根（`bodyshape/middleware.go:20`），`router.Handle("/mcp", …)` 的 `r.Pattern` 就是 `"/mcp"`，所以表的键是它。`/mcp?notebook=…` 是同一个模式。表与两个处理器都是函数，不是包级的变量：`server/.golangci.yml:8-12` 开着 `gochecknoglobals`，它只放过以 `err` 开头的错误变量、`_` 与 `version`（设计审查 A-M13；代码库的做法同 `page.BodyLimits()`）。同理，mcpserver 的哨兵错误只有 `errors.New` 的（`errNoCredential`）；要答一个 problem 的写成函数：`notPAT()` 返回 mcpserver 自己的一个未导出的类型的值（`ProblemStatus` 403、`ProblemCode` `"forbidden"`、`Error()` 是 detail），它满足 `httpserver.ProblemError` 这个接口（`apierrors.go:39-43`：那是接口，httpserver 不导出具体的 problem 类型，复核 A N-5）。

**按路由的上限**：组合根算 `mcpBodyLimit = 6*(page.MaxContentBytes+1<<20) + 64<<10`（37,814,272 字节，00 号文档 4.4），并进 `apiConfig` 的 `BodyLimits`（与 page 的两个路由合成一张表，键是模式），交给 mcpserver 的 `Options.MaxBodyBytes`（SDK 的 `MaxRequestBodyBytes`，`sdk/mcp/streamable.go:224`）。`page.MaxContentBytes` 由 page 的模块根导出（今天只在 `page/domain/content.go:12`）。`BodyLimits` 里的路由期限另加 `read_timeout`（`api.go:225-238`）：一个 `/mcp` 请求至多 45 秒。合并在 `wire.go` 里做，那里调 `apiConfig`。

**超过 64 KiB 的只能是工具的调用**（S1 审查 A-M1、修复核对第一轮 F1 与 F2，主持者的决定）：SDK 把读不了的参数照 `%q` 写进 -32602 的消息、再编码一次，请求体里的 `\"` 两个字节答出八个；36 MiB 的请求体答 144 MiB、分配约 2.6 GB，任何 PAT、任何方法都到得了。所以 `ServeHTTP` 在凭据在不在之后、会话令牌的 403 之前调 `sizeError`：请求体（`bodyshape.Bytes(r)`，中间件读过的，不复制）超过 64 KiB（`maxOtherBytes`）时，只有同时满足这几条才过，别的答 413 `payload_too_large`（同请求体的上限）：
- 是 `tools/call`，`params.name` 是这个入口登记了的工具：`askedEntry` 照 `entryOf` 的读法，查询串解析成功又没有 `notebook` 时是账户入口，别的是笔记本入口（那时 `entryOf` 要么取它、要么在 SDK 读之前答 404）；没有工具服务的名字也是 413，不再是 SDK 的不认识的工具；
- `jsonrpc` 恰好是 `"2.0"`（SDK 把错的写进它的纯文字 400），`id` 照写的（`jsontext.Value`）至多 256 字节（`maxIDBytes`；SDK 在每个答复里写回 `id`）；
- `params._meta` 照 SDK 的读法解进 `map[string]any`：float64 放不下的数（`1e400`）在这里失败，SDK 解它失败时会回显整个 `params`。

读法用 `encoding/json/v2`：名字照确切的大小写读，同 SDK 的解码（`encoding/json` 会把 `METHOD` 当作 `method`）；go1.27.1 里它在默认打开的 GOEXPERIMENT `jsonv2` 之后，升级工具链时要核对它仍打开、或已离开实验（`sizeError` 的注释写明）。`prompts/get` 与 `tools/call` 之外的每个方法一样至多 64 KiB：M10 的 prompt 的参数要放得下，或者另设例外。重量（同一进程、`/mcp` 的上限、各在自己的进程里）：这一类的请求答 126–222 字节、分配 113–186 MB（原来 0.8–2.7 GB）；64 KiB 之内的回显至多约 4 × 64 KiB；上限处合法的调用分配约 1.4 GB、峰值约 +0.3–0.5 GB（四个同时 +1.4 GB），不是 00 号文档第 10 节估的约 150 MiB，并发的界线给 P2（第 8 节）。P2 的 S1 照它加了大的请求体的名额（整个服务两个、每个凭证一个）与请求体的 JSON 值的上限（[P2](02-P2-writes-history.md) 3.2、3.2.6）。

**`Server.ServeHTTP`**（mcpserver 的入口，在 `API.Wrap` 之内，也就在结构检查读完请求体之后）：

```go
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ns := &noStore{ResponseWriter: w}                    // 3.4.3
	cred, ok := s.credentials(r.Context())
	if !ok {                                             // authenticate always ran: a wiring fault
		s.errors.Write(ns, r, errNoCredential)           // 500, logged at error
		return
	}
	if err := s.sizeError(r); err != nil {               // over 64 KiB and no tools/call of the entry's tool
		s.errors.Write(ns, r, err)                       // 413 payload_too_large
		return
	}
	if cred.TokenID == (uuid.UUID{}) {                   // a session's access token
		s.errors.Write(ns, r, notPAT())                  // 403 forbidden
		return
	}
	req := &request{ctx: r.Context(), read: time.Now(), cred: cred, entry: AccountEntry}
	if r.Method == http.MethodPost {                     // other methods: the SDK's 405, before a read
		entry, notebook, err := s.entryOf(r.Context(), r.URL) // 3.2.4
		if err != nil {
			s.errors.Write(ns, r, err)                   // 404 notebook.not_found
			return
		}
		req.entry, req.notebook = entry, notebook
	}
	s.handler.ServeHTTP(ns, r.WithContext(context.WithValue(r.Context(), requestKey{}, req)))
	if !ns.wrote {                                       // r's context ended while a method ran: nobody wrote
		err := r.Context().Err()                         // REST's: gone 499 at debug, deadline 500 at warn
		if err == nil {
			err = errUnwritten                           // a live context nobody wrote for: 500 at error
		}
		s.errors.Write(ns, r, err)
	}
}
```

最后一个分支是设计审查 A-M3 的路径：SDK 的 `hangResponse` 在请求的 ctx 结束时返回（`sdk/mcp/streamable.go:1267-1273`），`defer session.Close()` 等方法做完（jsonrpc2 的 `Close` 等进行中的请求，`sdk/internal/jsonrpc2/conn.go:504-509`），谁也没写头与正文，net/http 就以 SDK 留下的头答一个空的 200（`no-cache, no-transform`、`application/json`）。请求的 ctx 在方法跑着时结束有两种：客户端走了（Ctrl-C、客户端自己的超时、网络断了；复核 A N-2 实测在 `slow` 跑着时断开 5 次，5 次都没写），或方法比请求自己的期限（45 秒）活得久（调用的期限，读完请求体 + 15 秒，通常先结束方法，所以这要一个不看 ctx 的方法，例如 P4 的 `grep` 漏了一次检查）。这一支把 `r.Context().Err()` 交给 `Write`，用 REST 的分类（`apierrors.go:232-237`）：`Canceled` 记 debug "client went away"、答 499（没有人读）；`DeadlineExceeded` 记 warn "API request deadline exceeded"、答 500 problem+json，`no-store` 由 `ns` 守住。ctx 还活着而 SDK 什么也没写时（不该有）`Err()` 为 nil，交 `errUnwritten`（nil 的错误没有可记、可分类的），走 `Write` 的默认分支：error "API handler failed" 与 500，那是一个接线的错。第一轮修订写的是先记 error "MCP answer not written"、再总交一个固定的 `errUnwritten`：它走 `Write` 的默认分支，每次断开都记两行 error、往关了的连接里写 500，与 3.3.6 的 `FaultGone`（debug，"没有人读答复"）相悖（复核 A N-2），不要。`noStore.wrote` 在 `WriteHeader` 或第一次 `Write` 时设上（3.4.3）。

`s.handler` 是 `mcp.NewStreamableHTTPHandler(s.getServer, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true, DisableLocalhostProtection: true, MaxRequestBodyBytes: o.MaxBodyBytes, Logger: sdkLogger})`；`getServer` 按记在 ctx 里的入口选两个 server 之一，从不返回 nil（nil 时 SDK 答 400，`sdk/mcp/streamable.go:400-403`）。不按笔记本建 server，也就没有按笔记本、按 `AGENTS` 的 server 缓存：说明在握手时由接收的中间件填（3.15.2），缓存的是说明的文字，键是（笔记本、`AGENTS` 节点或没有、版本号），在 page 里（3.15.1）。

所以 `/mcp` 的码的次序（13.1 第 4 条的写法；与 00 号文档 4.4、4.15 相同）：

| 次序 | 答复 | 谁答 | 形状 |
|---|---|---|---|
| 1 | 401（没有令牌）→ 429（失败闸门）→ 401（无效、撤销、过期的令牌）→ 429（`authenticated` 的桶） | `authenticate`、`rateLimit` | 401 是 problem+json 带 `WWW-Authenticate`，429 是 problem+json 带 `Retry-After`；闸门在核对令牌之前（`platform/httpserver/api.go:269-277`），与 REST 相同 |
| 2 | 413（声明的长度超过 37,814,272 字节，不取名额）→ 503 `server_busy`（超过 64 KiB 而等了 2 秒仍取不到大的请求体的名额；M9/P2 S1，[P2](02-P2-writes-history.md) 3.2）→ 413、400（bodyshape：读到超过 37,814,272 字节；不是一个 JSON 文档；JSON 值多过 `mcpserver.MaxBodyValues` = 10,000 个（P2 S1 审查 A2-I1，P2 3.2.6）；名称重复、不是合法的 Unicode；结构；读超时） | `largeBody`、`bodyLimit`、`bodyshape` | problem+json，503 带 `Retry-After: 1` |
| 3 | 413 `payload_too_large`（超过 64 KiB 而不是这个入口登记的工具的 `tools/call`，见上）→ 403 `forbidden`（会话的访问令牌） | mcpserver | problem+json，403 的 detail "MCP takes a personal access token, not a session's access token." |
| 4 | 不是 POST 的（GET、DELETE 等）：400 `Unsupported protocol version`（`Mcp-Protocol-Version` 头不认识，排在 `2026-07-28` 之前，见下）→ 405 `Allow: POST` | SDK（版本在方法之前，`streamable.go:347-355`；`:381-386`） | 纯文字 |
| 5 | 404 `notebook.not_found`（查询串解析不了，`notebook` 出现两次以上、为空、不是 UUID、看不到、已删除） | mcpserver 经 page 的 `Gate` | problem+json |
| 6 | POST 的不认识的协议版本 400、415、Accept 不对的 400、空的请求体的 400 | SDK（`:347-355`、`:388-398`） | 纯文字 |
| 7 | JSON-RPC 的答复（含工具的错误） | SDK 与工具 | `application/json` |

**不认识的协议版本的例外**（S1 审查 A-N3）：第 4、6 行的"不认识的版本答纯文字 400"只对排在 `2026-07-28` 之前的版本成立：SDK 的检查是 `… && protocolVersion < protocolVersion20260728`（`sdk/mcp/streamable.go:354`），`2026-07-28`、`9999`、`x` 这样的头过得去：GET 答 405，通知的 POST 答 202，调用答 `application/json` 的 JSON-RPC 400（`-32602`、`-32020` 或 `-32022`）；这些里都没有我们的代码在跑。GET 的"不认识的版本 400 在 405 之前"由 `TestTheEntryGatesPostsOnly` 钉住，升级 SDK 改了次序就失败。

`/mcp/` 之下与 `/.well-known/` 之下的任何方法、任何路径都是 404 problem+json `not_found`，不认证（它们不经 `API.Wrap`）：`Subtree` 的 detail "No MCP endpoint at <path>; the endpoint is /mcp."，`WellKnown` 的是 "This server has no OAuth: MCP takes a personal access token in the Authorization header."。两个处理器都经 `httpserver.WriteProblem`；它们也经平台链，固定头因前缀给 `no-store`（3.2.3）。

**`POST /register`**（S1 的试用的发现，主持者的决定）：会走 OAuth 的客户端（TypeScript SDK 带 OAuth 的 provider，Claude Code 一类）在 401 之后读四个 `/.well-known/` 的元数据，都是 404，就照 MCP 2025-03-26 的默认端点在源站 `POST /register` 动态注册；那里原是网页，答纯文字的 405，客户端报 `HTTP 405: Invalid OAuth error response: SyntaxError …`，看不到提示。所以 `Register` 答 400 `application/json`、`{"error":"invalid_client_metadata","error_description":"This server has no OAuth: MCP takes a personal access token in the Authorization header."}`（RFC 7591 3.2.2，客户端照原样显示 `error_description`），不读请求体。它经 `api.Wrap`（请求信息、期限、请求体上限、公开的跳过认证、匿名的桶），结构表是空的；由组合根的 `mcpPublicOperations()` 声明为公开（契约之外，v0.1 总体设计 13.1 第 17 条的例外）。只服务 POST：GET、HEAD 与别的方法、`/register` 之下的路径照旧落到 `/`（网页），所以名叫 `register` 的已有工作区不被遮住，只拒绝新建的（`register` 进保留名单的 `[server]`，13.1 第 26 条）。负责人要撤回时去掉处理函数、挂载、公开的模式与保留名，接受 OAuth 客户端显示网页的 405。

**限流**：一个 POST 一个 `authenticated` 的单位，与 REST 共用 `pat:<id>` 的桶；403、405 之前单位已取，接受（00 号文档 4.4）。`/register` 是公开的，取 `anonymous` 的桶（按客户端的 IP）；`TestTheMCPRoutesTakeTheBuckets` 核对两个路由的 429（S1 审查 C-M1）。

#### 3.2.3 固定头

```go
// setFixedHeaders … A response under apiPrefix, of the MCP route and below
// it, under /.well-known/, and of a POST to RegisterPath also gets
// Cache-Control: no-store.
	if strings.HasPrefix(r.URL.Path, apiPrefix) || r.URL.Path == MCPPath ||
		strings.HasPrefix(r.URL.Path, MCPPath+"/") || strings.HasPrefix(r.URL.Path, wellKnownPrefix) ||
		r.Method == http.MethodPost && r.URL.Path == RegisterPath {
		header.Set("Cache-Control", "no-store")
	}
```

`MCPPath = "/mcp"`、`RegisterPath = "/register"`（导出）与 `wellKnownPrefix = "/.well-known/"` 是 httpserver 的常量（平台包不导入 mcpserver：mcpserver 在 httpserver 之下，反过来导入会成环）。mcpserver 写 `const Pattern = httpserver.MCPPath`，两处是同一个常量，不要测试核对（设计审查 A-N3：未导出的常量 mcpserver 的测试读不到）。SDK 在带结果的答复上改写成 `no-cache, no-transform`（`sdk/mcp/streamable.go:1686`），由 3.4.3 的写者设回。`/register` 只有 POST 不存：它的 400 与之前边缘的拒绝（429 等）都 `no-store`，`Register` 自己不设；GET 与它之下的路径是网页的、照网页的头（S1 审查 A-N1）。

**没有令牌的 401**（S1 审查 A 对主持者的附加问题，主持者的决定）：平台的 401 是与 REST 共用的 "This operation requires a bearer token."，不走 OAuth 的客户端照原样显示它，看不出要个人访问令牌（试用的第 4a 步）。httpserver 的 `APIConfig.NoTokenDetails`（按路由的模式，同 `RequestTimeouts`、`BodyLimits`；复制一份，空的说明拒绝）给一个路由没有令牌时 401 的说明，`unauthorized()` 在没有令牌时用它；组合根交 `{mcpserver.Pattern: mcpserver.NoOAuth}`。所以 `/mcp` 没有令牌的 401、`/.well-known/` 的 404 与 `/register` 的错误说同一句（`TestTheMCPTellsEveryClientOneHint`）；挑战仍是不带错误参数的 `WWW-Authenticate: Bearer`，失败的令牌的 401（`Bearer error="invalid_token"`）与 REST 的 401 不变。

#### 3.2.4 入口与 `notebook` 的解析

```go
// entryOf is the entry of a POST to Pattern (M9 design 4.6): the account
// entry without a notebook parameter; the notebook entry with exactly one
// that uuid.Parse takes and that Gate lets through. A query that does not
// parse, the parameter twice or more, empty, or no UUID is
// notebook.not_found from Gate on the zero id, which it answers without a
// read; the query is parsed once, here.
func (s *Server) entryOf(ctx context.Context, u *url.URL) (Entry, uuid.UUID, error)
```

`url.ParseQuery(u.RawQuery)`，不用 `u.Query()`：`Query()` 丢掉 `ParseQuery` 的错误、悄悄去掉它解析不了的那一对（设计审查 A-M1 实测：`notebook=%zz` 与 `notebook=<id>;x=1` 在 `Query()` 里都没有 `notebook`），截断的粘贴或地址里的 `;` 就让客户端落到账户入口，正是 00 号文档 4.6 不要的。所以：解析出错 → `Gate(ctx, uuid.UUID{})`（404，不查库；不管错的那一对是不是 `notebook`：查询串只有这一个参数有意义，坏的查询串不猜它的意思）；没有 `notebook` 键 → 账户入口；恰好一个值而 `uuid.Parse` 收下 → 笔记本入口，交 `Gate(ctx, id)`；别的 → `Gate(ctx, uuid.UUID{})`，page 对零值的 id 不查库就答 `notebook.not_found`（码只在 page）。别的查询参数不看（客户端可能带自己的）。

id 照 REST 的解析：`uuid.Parse` 收下的大写、带花括号与 `urn:uuid:` 的写法都收（同 `bodyshape` 的 `FormatUUID`，`bodyshape.go:377-378`），不另立一条更严的规则（设计审查 A 的过度设计第 2 条）；日志与 ctx 里记的是解析出的 id，写作规范的小写（`id.String()`），所以日志的 `notebook_id` 总是同一个写法。

page 的 `Gate`（S1 新加，`page/app/gate.go`）：

```go
// NotebookGate decides an MCP entry into a notebook (M9 design 4.6):
// node.list, unlocked, notebook.not_found for none, a deleted one, one
// the caller cannot see, and the zero id, without a read.
type NotebookGate struct{ notebooks Notebooks; auth shared.Authorizer }

func (g *NotebookGate) Execute(ctx context.Context, notebookID uuid.UUID) error
```

次序同 `ListNodes`：`RequireActor` → 零值的 id 答 404 → `WorkspaceOf` → `authorize(…, domain.ActionList, …, domain.ErrNotebookNotFound)`（`page/app/list_nodes.go:29-43`）。它不是安全的边界：PAT 有账户的全部权限，入口是"cd 进去"（总体设计 6.3）。

### 3.3 S1：mcpserver 的类型与登记

#### 3.3.1 选项与两个 server

```go
// Options configure the MCP server; bootstrap fills them.
type Options struct {
	Name, Version string // serverInfo
	Tools        []Tool
	Prompts      []Prompt // none in M9's production wiring; the platform test's double (3.4.7)
	Credentials  func(ctx context.Context) (Credential, bool)
	Notebooks    Notebooks   // the notebook entry's gate and instructions
	Account      string      // the account entry's instructions (S3; "" sends none)
	Names        ClientNames // the client names (S3; nil: every call's client is UnknownClient)
	Errors       httpserver.APIErrors
	// RequestTimeout is server.request_timeout: a call's work from the
	// request's body read, MaxBodyBytes the route's body limit.
	RequestTimeout time.Duration
	MaxBodyBytes   int64
	Logger         *slog.Logger
}

// Credential is the account a request acts as, and its personal access
// token: TokenID is the zero id for a session's access token.
type Credential struct{ UserID, TokenID uuid.UUID }

// Notebooks decide the notebook entry (page's MCPEntry).
type Notebooks interface {
	Gate(ctx context.Context, notebookID uuid.UUID) error
	// Instructions are the notebook's instructions for an initialize, ""
	// for none (S3).
	Instructions(ctx context.Context, notebookID uuid.UUID) (string, error)
}

// ClientNames clean the names clients tell and remember them on their
// personal access tokens (M9 design 4.5).
type ClientNames interface {
	Clean(name string) string // shared.CleanClientName: "" for none left
	Recorded(ctx context.Context, tokenID uuid.UUID) (string, error)
	Record(ctx context.Context, tokenID uuid.UUID, name string) (written bool, err error)
}

// New builds the server: a wiring fault panics (a tool or a prompt named
// twice, a tool of no entry, an input type that is no struct, a schema it
// cannot normalize, a Notebook tool that is not in the account entry,
// where alone the flag adds its "notebook"; a tool without its handler, a
// prompt without its Get; no Credentials, Notebooks, Logger or Errors).
func New(o Options) *Server
```

`Notebook` 的含义是"在账户入口里它多一个必填的 `notebook`"（3.3.2），所以没有意义的组合是一个 `Notebook` 的工具不在账户入口：标志在笔记本入口里什么也不做。只在账户入口的、作用于一本笔记本的工具（`open_notebook`）正是 `Entry: AccountEntry, Notebook: true`，必须登记得上（设计审查 A-I1、B-I4：草稿把规则写反了，S3 的 `wireApp` 会在 `New` 里 panic）。

两个 server 同样的选项，只差登记的工具：

```go
mcp.NewServer(&mcp.Implementation{Name: o.Name, Version: o.Version}, &mcp.ServerOptions{
	Logger:                    sdkLogger, // 3.4.5
	SupportedProtocolVersions: []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"},
	Capabilities:              &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{ListChanged: false}},
	SetCacheable: func(_ context.Context, _ mcp.Request, c *mcp.Cacheable) {
		c.CacheScope, c.TTLMs = "private", 0
	},
})
```

再各 `AddReceivingMiddleware(s.outer, s.handshake)`（`outer` 最外：`sdk/mcp/shared.go:133-137` 从右往左套，第一个最先执行）。不设 `ServerOptions.Instructions`：说明由 `handshake` 按请求填（3.15.2）。`o.Prompts` 不空时 `Capabilities` 另设 `Prompts: &mcp.PromptCapabilities{ListChanged: false}`：有了一个 prompt 而 `Capabilities.Prompts` 为 nil 时 SDK 自己填 `{ListChanged: true}`（`sdk/mcp/server.go:681-684`，设计审查 A-M7）；为空时不声明（生产的接线没有 prompt，M10 登记时照此，第 8 节）。`serverInfo.version` 是二进制的版本（`version.Version`，同 `GET /api/v0/instance`）。

#### 3.3.2 注册表

```go
// Entry is which entry a tool is listed in.
type Entry uint8

const (
	NotebookEntry Entry = 1 << iota // /mcp?notebook=<id>
	AccountEntry                    // /mcp
	BothEntries = NotebookEntry | AccountEntry
)

// Annotations are a tool's hints (M9 design 5): openWorldHint is always
// false, destructiveHint only for a write.
type Annotations struct{ ReadOnly, Destructive, Idempotent bool }

// Spec describes a tool to its clients.
type Spec struct {
	Name, Title, Description string
	Annotations              Annotations
	Entry                    Entry
	// Notebook marks a tool that acts on a notebook: in the account entry
	// it takes a required "notebook", the notebook's id, which the
	// notebook entry takes from its address.
	Notebook bool
}

// Tool is a tool of the registry, as a module's adapter/mcp builds it.
type Tool struct {
	spec   Spec
	in     reflect.Type
	handle func(ctx context.Context, c Call, in any) (Result, error)
}

// Spec is t's description: the composition root's wiring test reads it
// (3.5).
func (t Tool) Spec() Spec

// NewTool is the tool s, whose arguments decode into In, a struct: its
// fields without omitempty are required, a pointer field is optional and
// takes null as absent, and a string field is the one way to pass an id.
func NewTool[In any](s Spec, handle func(ctx context.Context, c Call, in In) (Result, error)) Tool

// Call is what a handler is told of its call.
type Call struct {
	Entry Entry
	// Notebook is the notebook entry's, gated, or the account entry's
	// argument: the zero id when it is no UUID, which the use cases
	// answer notebook.not_found without a read.
	Notebook        uuid.UUID
	Client          string // cleaned; UnknownClient when none is known
	UserID, TokenID uuid.UUID
}

// UnknownClient names a client that told no name (M9 design 4.5).
const UnknownClient = "unknown"

// Result is a tool's answer: Structured for a structured tool, whose
// content is its JSON text; Blocks for a text tool (read), which has no
// structuredContent. Exactly one is set.
type Result struct {
	Structured any
	Blocks     []Block
}

// Block is one content block: an image, a link, or else text, empty
// too; text beside an image or a link is a block of two kinds, which
// the answer refuses (internal_error).
type Block struct {
	Text  string
	Image *Image
	Link  *Link
}

type Image struct {
	Data []byte // the file's bytes; the SDK writes them in base64
	MIME string
}

type Link struct {
	URI, Name, MIME string
	Size            int64
}
```

模块不导入 SDK 的类型：`Result` 到 `*mcp.CallToolResult`、`Annotations` 到 `*mcp.ToolAnnotations`（`ReadOnlyHint`、`IdempotentHint`，`DestructiveHint` 只在 `Destructive` 时为 `&true`，`OpenWorldHint` 总是 `&false`）、`Block` 到 `TextContent`、`ImageContent`、`ResourceLink`（`sdk/mcp/content.go:28`、`:57-78`、`:128-153`）都在 mcpserver 里。`Title` 只写在工具的顶层 `title`，不重复进注解。

`New` 的登记：按 `Entry` 分进两个 server，每个经 SDK 的低层 `Server.AddTool(&mcp.Tool{Name, Title, Description, InputSchema, Annotations}, handler)`（`sdk/mcp/server.go:315`）：低层的登记不校验参数、不写 `structuredContent`、不要输出 schema（`:315-368`），那些都由 mcpserver 做。同一个入口里名称重复 panic（SDK 的 `featureSet.add` 会悄悄替换，`sdk/mcp/features.go:40-44`）；`validateToolName` 不合时 SDK 只记一行 error（`server.go:316-318`），所以 mcpserver 自己先查名称（`^[a-z_]{1,32}$`）并 panic。server 记下每个入口登记的名字（`s.tools`），超过 64 KiB 的请求体只给它们（3.2.2）；挑两个入口的 server 只在 `serverOf(e)` 一处（`getServer`、`addTools`、`addPrompts` 都用它）。没有处理函数的工具（`NewTool(spec, nil)`）在 `New` 里 panic，不等到调用时（S1 审查 B 的细节 5）。

#### 3.3.3 输入 schema 与参数

**推导与整理**（`schema.go`）：

1. `jsonschema.ForType(in, &jsonschema.ForOptions{})`（jsonschema-go v0.4.3）。实验（附录）：`omitempty` 的指针字段推导成 `{"type": ["null", "integer"]}`，别的字段进 `required`，结构是 `additionalProperties: false`；没有字段的结构是 `{"type": "object", "additionalProperties": false}`。字段的说明来自 `jsonschema:"…"` 标签。
2. 整理：
   - 必填的字段、数组与数组的项去掉 `"null"`（推导给切片的是 `["null", "array"]`，`jsonschema-go/infer.go:219-240`）；可选的标量保留 `["null", T]`：模型（尤其 Codex 里的 GPT）常对可选参数发 `null`，照"没有"收下。
   - 遇到 `[16]byte`（`uuid.UUID`）、`time.Time`、映射、接口：panic。这一条走 Go 的类型（`reflect` 逐字段），不看推导出的 schema：jsonschema-go 把 `time.Time`、`slog.Level`、`big.Int`/`big.Rat`/`big.Float` 推导成 `{"type":"string"}`（`jsonschema-go/infer.go:383-391`，设计审查 A-N7），在 schema 上认不出来；`[16]byte` 与别的定宽整数在 schema 上带 `minItems`、`maxItems` 或 `minimum`、`maximum`，下面的关键字检查也会拦住。id 一律是字符串字段（00 号文档 4.1），时刻不是输入。
   - 带 JSON 名字的内嵌结构 panic（jsonschema-go 把它摊平到外层，`encoding/json` 却读作一个成员：schema 说谎；不带名字的内嵌两边读得一样，照收）；含它自己的类型（经指针或切片）panic，不在 `New` 里栈溢出；`float32` panic（表的 `Number` 是 float64 的范围，`1e300` 过得了检查、解不进它；更窄的整数 jsonschema-go 已以 `minimum`、`maximum` 限住，下面的关键字检查拦住它们）。都是 S1 审查 B 的细节 2–4。
   - 不是指针而带 `omitempty` 的字段 panic：它在推导里是可选的、却收不下 `null`，与"可选参数的 `null` 照没有收下"相悖；可选的字段一律写成指针。
   - 账户入口里 `Spec.Notebook` 的工具在属性的最前加 `"notebook": {"type": "string", "description": "The notebook's id, from notebooks."}`，并放在 `required` 的最前（3.18 写 `["notebook", "path"]`，比较时 `required` 按次序）；笔记本入口里没有它（多传答 `bad_request` 的 `not_allowed`）。`In` 本身从不带 `notebook`：两个入口用同一个 Go 类型。
3. 整理之后的 schema 是 `tools/list` 答出的，也就是 `api/mcp/tools.json` 写的（3.5）。

**结构表**：同一份整理之后的 schema 转成一张 `bodyshape.Table`（每个入口每个工具一个根，键是工具名）：`type` 到 `Types`（`"integer"` 是 `Integer`，`["null", "string"]` 是 `Null|String`），`properties` 到 `Props`，`required`，`additionalProperties: false` 到 `Closed`，数组的 `items`。只有这些关键字：整理之后出现别的关键字（`enum`、`minimum`、`pattern`）panic，取值的检查是用例的 422（13.1 第 4 条的次序）。

**一次调用**（工具的包装，`tool.go`）：

```go
func (s *Server) handlerOf(t Tool, e Entry) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// 1. args: req.Params.Arguments, "{}" when absent or JSON null
		// 2. the table's Check(name, args): *bodyshape.Error → bad_request (3.3.6)
		// 3. the account entry, a Notebook tool: notebook out of args, uuid.Parse → Call.Notebook
		//    (as REST parses an id; no UUID: the zero id); the member is removed before decoding
		// 4. decode into a new In: encoding/json, DisallowUnknownFields; a failure here
		//    is the wiring's (the check passed): internal_error
		// 5. Call{Entry, Notebook, Client, UserID, TokenID}; t.handle(ctx, call, in)
		// 6. the result or the error answer (3.3.6), the answer's bound (3.4.4), the log (3.4.5)
		// never a result and an error together: always (result, nil)
	}
}
```

结构检查在解码之前，所以 `encoding/json` 的大小写不敏感的字段匹配碰不到：结构表是 `Closed`，`Path` 一类的成员已经答 `not_allowed`。请求体整个已经过边缘的歧义检查（重复的成员、不成对的代理项），参数是它的一部分，不再查一次。NUL（`\u0000` 的转义）是合法的 JSON 字符串，过得了结构检查，由用例的取值检查答 422（路径的段的检查，3.9.1；`TestToolStringsDoNotAnswerInternalErrors`，3.13）。

#### 3.3.4 两种成功的答复

- **结构的工具**（`Result.Structured`）：mcpserver 以 `json.NewEncoder` 加 `SetEscapeHTML(false)` 编码一次，去掉末尾的换行，`content` 是一块 `TextContent{Text: 这段 JSON}`，`structuredContent` 是同一段字节（`json.RawMessage`）：两份不会各说各的。SDK 的默认 `json.Marshal` 会把路径与正文里的 `<`、`>`、`&` 写成六个字节的转义（`sdk/mcp/server.go:455`），模型读到的就是转义本身；mcpserver 自己编码，模型读到原样的字符。线上的 JSON-RPC 答复仍由 SDK 编码，它在信封里转义这三个字符（实验），客户端解码之后照原样。
- 结构的答复编码出来不是 JSON 对象（数组、带类型的 nil 的 `null`）答 `internal_error`，不记那些字节：MCP 2025-11-25 的 `structuredContent` 是对象，严格的客户端（TypeScript SDK 的 schema）会拒绝别的（S1 审查 B 的细节 9）。
- **文字的工具**（`Result.Blocks`，`read`）：`content` 是这些块，没有 `structuredContent`。没有图片也没有链接的块是文字块，空的也是（线上 `{"text":"","type":"text"}`；`read` 读零行时第二块就是它，S1 审查 B-I1）；文字与图片或链接同在一块答 `internal_error`（`block %d is of two kinds`）。
- 不声明 `outputSchema`（00 号文档 4.7）。

#### 3.3.5 page 的 `MCPEntry()`

```go
// MCPEntry is the notebook entry's decisions, for mcpserver (M9 design
// 4.1): Gate (S1) and Instructions (S3).
func (m *Module) MCPEntry() MCPEntry

type MCPEntry struct{ gate *app.NotebookGate; instructions *app.Instructions }

func (e MCPEntry) Gate(ctx context.Context, notebookID uuid.UUID) error
func (e MCPEntry) Instructions(ctx context.Context, notebookID uuid.UUID) (string, error)
```

它按结构满足 `mcpserver.Notebooks`，组合根以编译期断言固定（13.1 第 12 条）。S1 的 `Instructions` 答 `""`（握手不发说明），S3 换成 3.15。

#### 3.3.6 `ProblemOf` 与工具的错误

```go
// Fault is what of an error is the server's.
type Fault uint8

const (
	FaultNone     Fault = iota // a ProblemError, a body too large: the caller's
	FaultGone                  // the client went away: debug, nothing to answer
	FaultDeadline              // the request's deadline: warn, internal_error
	FaultUnknown               // anything else: error, internal_error
)

// Answer is how an error is answered.
type Answer struct {
	Problem Problem       // internal_error without detail for a fault
	Retry   time.Duration // Retry-After, when positive
	Fault   Fault
}

// ProblemOf classifies err, met while serving the request whose context is
// ctx, as Write answers it (M9 design 4.1): the one classification of
// REST and MCP.
func ProblemOf(ctx context.Context, err error) Answer
```

分类照今天 `Write` 的四支（`apierrors.go:220-241`）：`ProblemError` → `problemOf(pe)`；`*http.MaxBytesError` → 413；`context.Canceled` 而 `ctx.Err() != nil` → `FaultGone`；`context.DeadlineExceeded` 而 `ctx` 的期限到了 → `FaultDeadline`；别的 → `FaultUnknown`。`Write` 改为调它，日志的文字与级别、499 与 `WWW-Authenticate` 照旧（`apierrors.go` 的测试照旧通过就是证明）。

```go
// ToolError is a tool's error whose words its module writes (M9 design
// 4.7): Err is classified as any error is; Text, when set, is the answer's
// message, and Extra adds members to structuredContent.error.
type ToolError struct {
	Err   error
	Text  string
	Extra map[string]any
}

func (e *ToolError) Error() string {
	if e.Err == nil { // a wiring fault: internal_error, its log's error in words
		return "mcpserver: a ToolError without its Err"
	}
	return e.Err.Error()
}
func (e *ToolError) Unwrap() error { return e.Err }
```

工具的错误一律是 `isError: true` 的结果，不是协议的错误：

| `ProblemOf(调用的 ctx, err)` | `content` 的文字 | `structuredContent.error` | 日志 |
|---|---|---|---|
| `FaultNone` | `ToolError.Text`，没有时 `<code>: <detail>`，有字段时另加 ` (<field>: <code>: <message>; …)`，有 `Retry` 时另加 ` Try again in <n> s.` | `code`、`message`（同 `content` 的文字）、`fields`（`[{field, code, message}]`）、`lock`、`locks`（M9 的读不会有，P2 的写有）、`retry_after_seconds`，加 `Extra` | 不记（同 REST） |
| 参数不合 schema（`*bodyshape.Error`） | `bad_request: The arguments do not match the tool's input schema. (path: required: is required; …)` | `code: bad_request`、`fields` | 不记 |
| `FaultGone` | `internal_error: the client went away` | `code: internal_error`、`request_id` | debug "MCP client went away"（没有人读答复） |
| `FaultDeadline` | `internal_error: the call ran out of time; request id <id>` | `code: internal_error`、`request_id` | warn "MCP call deadline exceeded" |
| `FaultUnknown` | `internal_error: the server failed; request id <id>` | `code: internal_error`、`request_id` | error "MCP tool failed"，带工具名、`request_id` 与错误 |
| panic（3.4.2） | 同上 | 同上 | error "MCP method panicked"，带栈 |

字段错误的 `field` 用 REST 的写法（bodyshape 的 `tags[1].name`，2.1 第 3 条）；路径为空（`arguments` 本身不是对象）时写作 `arguments`，不写空的括号（设计审查 A-N5 实测草稿的写法给出 `(: invalid_format: …)`）。日志的字段同 13.1 第 10 条，从不带参数（3.4.5）。不认识的工具由 SDK 答 -32602（`sdk/mcp/server.go:1005-1012`；实验）：它在消息里复述工具名，不设上限（3,000 字节的名称答 3,078 字节，设计审查 A-N12），无害：名称是客户端自己写的，答复只回到它那里。

错误的答复同样受 `MaxAnswerBytes` 管（3.4.4）：文字块与 `structuredContent.error` 的 JSON 文字都按同一个量计、合计在上限之内（`structuredContent.error` 按文字块的量计是高估，简单而保守；设计审查 A-N11：Codex 把 `structuredContent` 交给模型）。M9 的读的错误各成员都有界（码、至多 16 个字段、各 256 字节的路径与有界的消息、`request_id`）；`lock`、`locks` 与 `Extra` 只在 P2 的写里有，P2 给它们上限（第 8 节）。超过时同成功的答复：error 日志、答只带码与 `request_id` 的 `internal_error`。

例外是参数不合 schema 的 `bad_request`（实施时的偏离 3，S1 审查 B-M3、C-M3）：16 条路径 × 256 字节 × 6–7 字节的转义约 55 KB，超过上限；这时丢掉 `fields`、只答开头一句 `bad_request: The arguments do not match the tool's input schema.`（70 字节的常量），同样经上限的检查，不记 error（错在客户端）。`structuredContent.error` 编码一次，就是 SDK 发出、上限数的那个 `json.RawMessage`；编码不了时（`Extra` 里的函数、NaN、channel）立刻答 `internal_error`、记 error "MCP tool failed"，不挂到调用的期限（S1 审查 B-M2）。`internalError` 的成员只有字符串，总编码得了：它在 goroutine 最外层的恢复里调用，不能 panic。`Err` 为 nil 的 `ToolError` 是 `internal_error`，日志的错误写成文字（"a ToolError without its Err"），不是 slog 恢复出的 `!PANIC`。prompt 的处理函数的错误不经这张表：记 error "MCP prompt failed"、答 -32603，分类与上限是 M10 的（3.4.7）。

### 3.4 S1：调用的 ctx、恢复、`no-store`、答复的上限、日志与测试

#### 3.4.1 每个方法的 ctx 从哪里来

无状态时 SDK 为每个 POST 建一个临时的会话，以这个 POST 的 ctx 连上（`connectStreamable(req.Context(), …)`，`sdk/mcp/streamable.go:434`），jsonrpc2 把它包上 `notDone`：值照样委托给这个 POST 的 ctx（所以 `requestKey`、执行者、请求号都读得到），期限与取消被去掉（`sdk/internal/jsonrpc2/conn.go:229-245`、`:813-821`）；每个请求再派生自己的 ctx（`conn.go:566` 起），交给接收的中间件链（`sdk/mcp/shared.go:206-225`）。

**无状态时通知到不了中间件**（设计审查 A-M12 实测：40 个通知的 POST 只有一个到了最外层的中间件，都答 202）：`servePOST` 发出通知、写 202（`sdk/mcp/streamable.go:1649-1661`），`serveStateless` 的 `defer session.Close()`（`:440`）随即让连接进入关闭，读循环之后不再收任何请求，"连通知也不收"（`sdk/internal/jsonrpc2/conn.go:503`、`:616-620`）。所以在 v1.8.0 的无状态模式里通知是空操作，什么也不能依赖它：名称、日志、取消都不经通知。`notifications/cancelled` 取消不了一次调用：它在另一个 POST、另一个临时的会话里。**取消只来自客户端结束它自己的 HTTP 请求**，经下面的 `AfterFunc(r.ctx, cancel)`；没有它，断开的客户端留下的工具一直跑到调用的期限（审查实测：300 ms 断开，工具跑到 1.5 秒的期限）。

所以每个方法——`initialize`、`ping`、`tools/list`、`tools/call`、旧协议的 `subscriptions/listen`——在 mcpserver 的最外层中间件里都拿到同一种 ctx：值是这个 POST 的，没有期限。实验（附录）：中间件里 `ctx.Deadline()` 答没有，`ctx.Value(requestKey{})` 是 `ServeHTTP` 放的。

```go
// request is what ServeHTTP tells the methods of one POST.
type request struct {
	ctx      context.Context // the POST's, with the per-route deadline
	read     time.Time       // when its body had been read: ServeHTTP runs after bodyshape read it
	cred     Credential
	entry    Entry
	notebook uuid.UUID
}

// outer is the outermost receiving middleware of both servers: every
// method's call context, and the recovery (M9 design 4.3).
func (s *Server) outer(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (res mcp.Result, err error) {
		r := requestOf(ctx) // never nil: the SDK is reached through ServeHTTP alone
		deadline := r.read.Add(s.requestTimeout)
		if d, ok := r.ctx.Deadline(); ok && d.Before(deadline) {
			deadline = d
		}
		call, cancel := context.WithDeadline(ctx, deadline) // keeps the SDK's values and its cancellation
		defer cancel()
		stop := context.AfterFunc(r.ctx, cancel) // the client went, or the server stops
		defer stop()
		defer s.recover(call, method, req, &res, &err)
		return next(call, method, req)
	}
}
```

期限是"请求的期限"（45 秒，3.2.2）与"读完请求体时 + `server.request_timeout`"（15 秒）中较早的：同 `API.Stream` 读完请求体之后的一步（13.1 第 14 条）。工具、填说明的（`handshake`）与记名称的都在 `next` 里，都用它：握手时读 `AGENTS`、写 `client_name` 也有期限、客户端走了也停。停机时 HTTP 的 `Shutdown` 等进行中的请求（`server.shutdown_timeout` 20 秒，长于一次调用的 15 秒），之后才关连接池（`bootstrap/app.go:96-103`）。不用 SDK 的 `PropagateRequestCancellation`：只对 2026-07-28 有效（`sdk/mcp/streamable.go:210-219`）。

#### 3.4.2 恢复 panic

SDK 里没有 `recover`，每个方法（含 `tools/call`）都在 jsonrpc2 的 `handleAsync` 起的 goroutine 里（`sdk/internal/jsonrpc2/conn.go:656-688`；`Async` 只是放开队列，`sdk/mcp/server.go:2003-2005`，只有 `initialize` 留着它），平台的 `withRecover`（`platform/httpserver/middleware.go:97-136`）看不到：一个工具里的空指针让整个进程退出（设计审查实测，退出码 2）。

```go
// recover answers a panic of a method as withRecover answers one of a
// handler: an error log with the stack, the method, the tool and the
// request id; a tool error internal_error for tools/call, a JSON-RPC
// internal error for any other method. It never re-panics: no one above
// it in this goroutine would recover.
func (s *Server) recover(ctx context.Context, method string, req mcp.Request, res *mcp.Result, err *error)
```

JSON-RPC 的内部错误写作 `&jsonrpc.Error{Code: jsonrpc.CodeInternalError, Message: "internal error"}`（`go-sdk/jsonrpc` 是公开的包）：一个普通的 Go 错误出去是 `{"code":0,…}`（设计审查 A-N4 实测）。

工具的包装（3.3.3）另 `defer` 同一个恢复：它在 `callTool` 之内（`sdk/mcp/server.go:1013-1025`），所以 panic 的工具也照常记那一行调用的日志、答出 `internal_error`。两处都从不同时返回结果与错误：`processResult` 在两者都有时以 `%#v` 把结果（含正文）写进 error 级的日志（`sdk/internal/jsonrpc2/conn.go:694-701`，经 `sdk/mcp/transport.go:234`），单元测试逐条核对（含 panic 的路径）。工具的恢复（`recoverTool`）在调用一行的日志之内：去掉它或换两个 defer 的次序时 panic 落到 `outer` 的恢复，答复仍是 `internal_error`，调用一行却写 `outcome: ok`，所以单元与整个程序的测试都核对 `boom` 一行的 `internal_error`（S1 审查 A-M2、B-M4、C-M4）。`outer` 的 panic 日志只在名字合 `toolName` 时记工具名：客户端发来的名字长度不限（S1 审查 A-N2）。实验（附录）：panic 的调用答 `isError: true` 的 `internal_error`，同一个 server 的下一次调用照常。SDK 自己的代码（参数的解析）在中间件之外 panic 时仍会让进程退出：那是 SDK 的缺陷，不在 v1.8.0 的已知行为里。

#### 3.4.3 `no-store` 的写者

```go
// noStore keeps Cache-Control: no-store on every answer of the route: the
// SDK sets no-cache, no-transform on the answers that carry a result
// (sdk/mcp/streamable.go:1686), and a tool's result is the notebook's.
type noStore struct {
	http.ResponseWriter
	wrote bool
}

func (w *noStore) WriteHeader(code int)          // sets no-store, once, then writes the header
func (w *noStore) Write(b []byte) (int, error)   // a first Write writes the header 200 first
func (w *noStore) Unwrap() http.ResponseWriter   // for http.ResponseController
func (w *noStore) Flush()                        // when the underlying writer flushes
```

`ServeHTTP` 一进来就包上，所以 403、404 与 SDK 的 405、415、400、202 都带它；`API.Wrap` 里更早的拒绝（401、429、413、400）与两个 404 子树由 `setFixedHeaders` 的前缀给（3.2.3）。实验（附录）：200、202、400 都留住了 `no-store`。`wrote` 在 `WriteHeader` 或第一次 `Write` 时设上；SDK 返回之后它仍是假，就是 3.2.2 的"答复没写"那一支：`ServeHTTP` 把请求的 ctx 为什么结束交给 `Write`（客户端走了答 499，期限到了答 500 problem，都带 `no-store`；设计审查 A-M3，复核 A N-2）。

#### 3.4.4 答复的上限

**一个量**（设计审查 B-I1、A-M4）：答复的上限数的是每块文字在答复里由 `encoding/json` 写出的字节。文字块的 `text` 由 SDK 的 `TextContent.MarshalJSON` 以 `json.Marshal` 写（`sdk/mcp/content.go:47`），所以除了 A-M4 量出的转义表，`<`、`>`、`&` 也各是六个字节（`exp/wire` 实测：`"text":"<>&\"q\nn"`）。go1.27.1 实测（`exp/measure`，与审查 A 的表相同）：

| 字符 | 写出的字节 |
|---|---|
| `"` `\` 与 `\n` `\r` `\t` `\b` `\f` | 2 |
| 别的 U+0000–U+001F，U+2028、U+2029 | 6 |
| `<` `>` `&` | 6（`json.Marshal` 的 HTML 转义） |
| U+007F | 1（原样） |
| C1 控制字符 U+0080–U+009F | 2（原样的 UTF-8） |
| 不是 UTF-8 的字节 | 3（原样的 U+FFFD；正文与名称都是合法的 UTF-8，碰不到；把它写成六个字节的转义的工具链是 6，S1 审查 B 的存疑） |
| 别的字符 | 它的 UTF-8 字节数 |

结构的工具的文字块就是结果的 JSON 文字（3.3.4），所以它的字符串被转义两次：mcpserver 的编码（`SetEscapeHTML(false)`）一次、文字块一次。一个成员里的引号与反斜杠是 4 个字节，换行等五个是 3 个，别的控制字符与 U+2028、U+2029 是 7 个，`<>&` 仍是 6 个（第一次原样）。这对线上是诚实的：答复里真有这么多字节。

```go
// MaxAnswerBytes bounds a tool's answer: the sum, over its text blocks,
// of TextBytes. For a structured tool the one text block is the result's
// JSON text, whose strings are thus escaped twice. Images are bounded on
// their own (read's MaxInlineImageBytes) and a link by its parts (M9
// design 4.7).
const MaxAnswerBytes = 48 << 10

// TextBytes is s's bytes in a text block as the SDK writes it:
// encoding/json's string escapes ("  \  \n \r \t \b \f two bytes; another
// C0 control, U+2028 and U+2029 six; U+007F one; a C1 control two) and,
// since TextContent marshals with json.Marshal, <, > and & six. Without
// the surrounding quotes.
func TextBytes(s string) int

// MemberBytes is s's bytes as a string member of a structured tool's
// answer: TextBytes of s as mcpserver's encoder writes it (" and \ four,
// the five short escapes three, another control, U+2028 and U+2029 seven,
// <, > and & six).
func MemberBytes(s string) int

// Fit is the answer of k ≤ n items that MaxAnswerBytes holds, where k is
// n, or k items fit and k+1 do not: build(k) is the answer with the first
// k items, which marks the others as left out (a next_cursor, or
// truncated) when k < n. The candidates gallop (1, 2, 4, … at most n)
// until one is over, then bisect between it and the last that fit; each
// is encoded as the answer is and measured exactly, so no builder counts
// on its own. A cursor's length follows the last item, so the measure is
// not monotone in k, and k need not be the largest that fits.
// Result.Structured is the encoded JSON. An answer of no item over the
// bound is errNothingFits.
func Fit(n int, build func(k int) any) (Result, int, error)
```

**一个帮助函数**：`list`、`notebooks`、`open_notebook` 的根下节点都经 `Fit` 填。候选的 k 从 1 起倍增（1、2、4……，至多 n），到第一个放不下的为止，再在它与上一个放得下的之间二分：答出的 k 是 n，或者 k 项放得下而 k + 1 项放不下。游标的长短随最后一项变（glob 的键至多约 5.6 KiB，经 JSON 的转义与 base64 之后游标至多 22,484 字节，S2 审查 A-N4），量不随 k 单调，所以 k 不一定是放得下的最大的那个，可能少放几项：无害，`next_cursor` 接着列；答出的从不超过上限（复核 A N-3、B N-6：第一轮写的"至多少放一项"不成立）；连零项的答复都放不下时 `Fit` 答 `errNothingFits`，工具答 `internal_error`（S1 审查 B 的细节 1：测试原来以 `k > 0` 豁免它）。候选至多约 2 log₂ n + 1 个（`limit` 1,000 时约 21 个；测试的界线是 `2⌈log₂ max(n, 2)⌉ + 1`），每个是一次编码与一次量；倍增让每个候选至多是第一个放不下的那个，即放得下的一份再加同样多的项：项的大小相近时约 2 × 48 KiB，最坏（后面的项都是 2,818 字节、全是 `&` 的路径）也不超过 `build(n)`。第一轮写的"每次至多约 50 KiB"不对：它先量 `build(n)`，`list` 的 1,000 项通常约 200 KB，最坏约 2.8 MB 的 JSON（复核 A N-3）。适配器从不自己数字节：草稿让 `list`、`notebooks` 的适配器"加上这一项超过上限就停"，没说怎么数，按 `len(JSON)` 数就少算一半（B-I1）。`read` 是文字的工具，它的第二块经 page 的 `Cut` 按 `TextBytes` 切（3.11.1）；`open_notebook` 的说明按 `MemberBytes` 切（3.17.3）。两个计数函数由 page 的模块根或 `adapter/mcp` 交给 page 的 `domain` 与 `app`（一个 `func(string) int` 的参数），page 里没有第二份计数，也就不要草稿的"两份一致"的测试（page 的 `app` 不导入平台，交函数值不算导入）。

**握手的说明**另有自己的 32 KiB 规则（3.15.3），用同一个量 `TextBytes`：`initialize` 的结果由 jsonrpc2 不带 HTML 转义地编码（`exp/wire`：`"instructions":"<>&\"q\nn"`），所以对说明，`TextBytes` 在 `<>&` 上多算五个字节，是偏保守的上界；一个量好过两个差五个字节的量。

mcpserver 在答复之前再量一次，超过是缺陷：error 级的日志 "MCP answer over its bound"（工具名、字节数、`request_id`），答 `internal_error`，不发那份答复。经 `Fit` 与 `Cut` 填的答复到不了这一支；它守的是忘了填的新工具。

#### 3.4.5 日志与 SDK 的 logger

- **每次工具调用一行**（info，"MCP tool call"）：`tool`、`notebook_id`（账户入口里是参数的，零值时不写）、`user_id`、`token_id`、`client`、`outcome`（`ok` 或错误的码）、`duration_ms`、`request_id`。从不记参数（路径、模式、正文都有标题或正文，13.1 第 10 条），也不记答复。
- **握手一行**（info，"MCP initialize"）：`client`（客户端自己报的名称，清理之后，不是个人信息）、`protocol_version`（协商出的，从结果读）、`entry`、`notebook_id`、`user_id`、`token_id`、`request_id`。
- 访问日志只记 `/mcp`，不记查询串（`platform/httpserver/middleware.go:151-181` 的 `loggedPath`，`notebook` 不进日志）。
- **SDK 的 logger**：交给 `ServerOptions.Logger` 与 `StreamableHTTPOptions.Logger` 的是平台的 logger 在 warn 级以上、带 `component=mcp-sdk`：

```go
// sdkLogger is logger for the SDK: warn and above. The SDK logs every
// stateless POST's session at info (sdk/mcp/server.go:1394, :1414, :1498),
// without a request id. The attributes go inside the filter: Logger.With
// would call the wrapper's WithAttrs, which an embedded Handler answers
// with the inner handler, unfiltered.
func sdkLogger(logger *slog.Logger) *slog.Logger {
	return slog.New(warnAndAbove{logger.Handler().WithAttrs([]slog.Attr{slog.String("component", "mcp-sdk")})})
}
```

  草稿写的 `slog.New(warnAndAbove{…}).With(…)` 留不住过滤（设计审查 A-M2 实测：`Info` 照样打出），SDK 每个无状态的 POST 约九行 info 就会不带请求号地涌进日志。`warnAndAbove` 另把 `WithAttrs`、`WithGroup` 包回自己，以后谁再 `With` 也留得住过滤；单元测试：经 mcpserver 的一个无状态 POST 之后，日志收集器里没有 SDK 的 info 行，warn 照样有。不交丢弃的 handler：那会连 SDK 真正的错误日志也藏起来。SDK 在 error 级记的"method invalid during initialization""duplicate initialize request"（`server.go:1990`、`:2095`）是客户端的错，照样记下（不带参数）。

#### 3.4.6 握手的中间件（S1 的部分）

```go
// handshake fills an initialize's result: the instructions (S3), and
// logs it; it records a client's name told by any request (S3).
func (s *Server) handshake(next mcp.MethodHandler) mcp.MethodHandler
```

S1 只记握手的日志。S3 加说明与名称（3.15.2、3.16.4）。

#### 3.4.7 `Prompt` 的替身

00 号文档第 8 节只要一个替身证明登记、列出与 `prompts/get` 到达，M10 才写真的 prompt，所以 P1 的类型是最小的（设计审查 A-M7 与 A 的过度设计第 1 条）：

```go
// Prompt is a prompt of the registry (M10 registers the first). Its
// arguments are strings, as MCP's PromptArgument are: no JSON schema.
type Prompt struct {
	Name, Title, Description string
	Arguments                []PromptArgument
	Get func(ctx context.Context, c Call, args map[string]string) (description string, messages []string, err error)
}

// PromptArgument is one named string a prompt takes.
type PromptArgument struct {
	Name, Description string
	Required          bool
}
```

两个入口都登记 prompt；没有 `Get` 的 `Prompt` 在 `New` 里 panic。`Get` 的错误不经 3.3.6 的分类与上限（记 error "MCP prompt failed"、答 -32603），是 M10 的（`promptHandler` 的注释写明，S1 审查 B 的细节 7）；`prompts/get` 的请求体至多 64 KiB（3.2.2）。`New` 把它转成 `mcp.Prompt{Name, Title, Description, Arguments: []*mcp.PromptArgument{…}}` 与一个处理函数：缺必填的参数答 `prompts/get` 的 -32602（同 SDK 对工具），`messages` 每条是一个 `role: user` 的 `TextContent`；同样经 `outer` 与恢复。不给参数的类型、结构检查与结果的形状：M10 需要时在这个注册表上加（第 8 节）。`Options.Prompts`（3.3.1）；M9 没有生产的 prompt。mcpserver 的单元测试以一个替身 prompt 证明：登记之后 `initialize` 的能力解码出 `prompts` 且 `listChanged` 为假、`prompts/list` 列出它、`prompts/get` 到达它的处理函数；没有 prompt 时能力里没有 `prompts`。整个程序的最后一跳写进给 M10 的移交（第 8 节）。

#### 3.4.8 mcpserver 的单元测试

经 `httptest` 与官方 Go 客户端（`sdk/mcp/client.go:51`，`StreamableClientTransport`），不连数据库，凭证与入口是替身：

- **schema**：`list`、`read` 形状的输入类型在两个入口的整理结果（逐字段）；必填与可选、`null` 的去留、`notebook` 的加与去、`notebook` 在 `required` 的最前；只在账户入口的 `Notebook` 工具（`open_notebook` 的形状：`Entry: AccountEntry, Notebook: true`）登记得上，它的 schema 有必填的 `notebook`；`uuid.UUID`、`time.Time`（走 Go 的类型，它在 schema 上是 `string`）、映射、`enum` 与不是指针的 `omitempty` 的 panic。
- **参数**：缺必填的、多一个成员、类型不对、可选参数的 `null` 照没有收下、`arguments` 缺省或为 `null` 当作 `{}`、`arguments` 不是对象时文字写 `arguments`（不是空的括号）、笔记本入口多传 `notebook` 答 `not_allowed`、账户入口的 `notebook` 不是 UUID 时 `Call.Notebook` 是零值、大写的 UUID 照 `uuid.Parse` 收下；文字照 3.3.6、`fields` 的路径是 REST 的写法；17 个问题只报 16 个、超长的路径截到 256 字节。
- **结果**：结构的工具 `content` 与 `structuredContent` 是同一段 JSON、`<>&` 原样（解码 `content` 的文字之后没有六个字节的转义）；文字的工具没有 `structuredContent`；`Result` 两个都设或都没设是 `internal_error`。
- **错误**：表 3.3.6 的每一行；`ToolError` 的 `Text` 与 `Extra`；处理函数同时返回结果与错误、返回 nil 的结果、panic，包装都只答结果（`err == nil`）；方法的 panic 答 `code` 是 -32603 的 JSON-RPC 错误（不是 0）；错误的答复连同 `structuredContent.error` 也在上限之内。
- **量与上限**：`TextBytes` 等于真的文字块在答复里的字节（经 SDK 编码一个 `TextContent` 再减去外面的部分），`MemberBytes` 等于真的结构的答复里那个成员的字节，都在随机的字符串上（含 A-M4 表的每一行、`<>&`、U+2028、多字节的字符）；表里每一行另有一个固定的例子。答复恰好 `MaxAnswerBytes` 照答、多一个字节答 `internal_error` 并记 error；`Fit`：n 项全放得下时答 n 项；放不下时答的 k 项放得下而 k + 1 项放不下（测试另量 `build(k)` 与 `build(k+1)` 核对）；替身的 `build` 记下每次的 k：候选至多 2 log₂ n + 1 个，每个不大于第一个放不下的；全是引号与换行的成员按两次转义计。
- **ctx**：处理函数看到的期限是"读完请求体 + 请求的期限"中较早的；请求的 ctx 取消时处理函数的 ctx 取消，而且很快（在调用的期限之前很久）；握手的中间件也拿到它。
- **答复没写**（A-M3，复核 A N-2）：请求的期限设得很短、一个不看 ctx 的方法跑过它：答 500 problem+json、`Cache-Control: no-store`、warn 日志 "API request deadline exceeded"（`Write` 对期限的分类，同 REST），不是空的 200；客户端在这个方法跑着时断开：debug "client went away"，日志里没有 error 级的行。断开的一例问两个工具：一个活过客户端，它的答复总是没写、记 debug；一个停在自己的 ctx 上，它的调用记客户端走了；两个都不记 error。只问后一个时 SDK 可能先把它的错误写进已关的连接，就没有"没写"（S1 合并之后 main 的 CI 上竞态的一份摆动一次，测试的修复 `c87caad7`）。
- **入口**：`notebook` 的取值（没有、一个合规的、大写的与带花括号的（`uuid.Parse` 收下，日志记小写的）、两个、空、不是 UUID、`notebook=%zz`、`notebook=<id>;x=1`）；后两种答 404 而不是落到账户入口；GET、DELETE 是 405 而不调 `Gate`。
- **两个 server**：两个入口的 `tools/list` 各是自己的工具、能力解码出来 `tools.listChanged` 为假（SDK 把假省掉，写成 `"tools":{}`，测试解码而不找字面，A-N15）、结果带 `cacheScope: "private"`、`ttlMs: 0`；名称重复、不在任何入口、`Notebook` 而不在账户入口（只在笔记本入口）、名称不合规则都 panic。
- **`no-store`**：`WriteHeader` 之前、只 `Write`、SDK 改写之后都是 `no-store`；`Flush` 与 `Unwrap`。
- **SDK 的 logger**：一个无状态的 POST 之后，收集器里没有 `component=mcp-sdk` 的 info 行（A-M2）。
- **`Prompt` 的替身**（3.4.7）。
- **审查与两轮修复核对加的**：空的文字块（`TestAnEmptyBlockIsText`）与不合格的块（文字与图片或链接同块，`TestAMalformedResultIsInternal`）；编码不了的 `Extra` 立刻 `internal_error`（`TestAnUnwritableErrorIsInternalAtOnce`）；参数超过 48 KiB 的回退（16 条各 252 字节的控制字符的路径）与空的字段写作 `arguments`（`TestAProblemOfTheArgumentsRootIsAtArguments`）；调用一行的 `outcome`（处理函数的问题、panic）与账户入口的 `notebook_id`；不是回环的 Host（`TestAProxysHostIsServed`）；GET 的不认识的版本 400 在 405 之前；`?notebook=<id>&x=%zz`；`Fit` 的跳跃停在 n、零项放不下的 `errNothingFits`；panic 的日志只记合规的工具名；带名字的内嵌、递归的类型、`float32`、没有处理函数、零值的 `Errors` 的 panic；没有凭据的接线的错（`TestARequestWithoutACredentialIsAFault`）；超过 64 KiB 只给这个入口的工具的调用（`TestOnlyACallMayBeLong`：`jsonrpc` 原样的 U+0080 与没有、256 与 257 字节的 `id`、`_meta` 的 `1e300` 与 `1e400`、每个工具在自己的与另一个入口、`?notebook=%zz`、`?x=1`；200 的行另要有 JSON-RPC 的 `result`）与 `params` 的形状（`TestTheParamsAreShapedBeforeTheSDK`）；bodyshape 的 `TestBytesIsTheBodyRead`；httpserver 的 `TestARouteSaysWhyItNeedsAToken` 与 `RegisterPath` 的 `no-store` 的行。

#### 3.4.9 S1 的整个程序的测试

`bootstrap/mcp_test.go`，经 serve 与真实的数据库，令牌是经接口建的 PAT。S1 没有生产的工具：`wireApp` 的最后一个参数改为测试的接缝

```go
// wiring is what a test may add to the app: a batch of its own for the
// transfer jobs, and MCP tools beside the modules'.
type wiring struct {
	batch    transfer.Batch
	mcpTools []mcpserver.Tool
}
```

（今天只有 `interleavings_workspace_test.go:78` 交 `batch`），测试登记三个工具：`echo`（两个入口，`Notebook: true`，答 `Call` 的笔记本、客户端与一个字符串参数）、`boom`（panic）、`slow`（等 ctx 结束，答它的错误）；边缘的测试另登记两个接缝工具 `seam_notebook`、`seam_account`（超过 64 KiB 的请求体，3.2.2）。生产的 `newApp` 交零值。

- **PAT 与边缘**：没有令牌、无效、已撤销的 PAT 答 401 带 `WWW-Authenticate`；会话的访问令牌答 403 `forbidden`（在 `notebook` 之前：带一个看不到的笔记本也是 403）；`notebook` 看不到、已删除、不是 UUID、出现两次、为空、查询串解析不了（`notebook=%zz`、`;` 分隔）答 404 `notebook.not_found`；没有令牌又坏的请求体答 401（次序没有拼反）；超过 37,814,272 字节答 413、恰好 37,814,272 字节的 `tools/call` 照答；64 KiB 的 `ping` 答 200、多一个字节答 413，每个接缝工具在自己的入口 64 KiB 加一个字节答 200、在另一个入口与 `missing` 答 413；`?notebook=<id>&x=%zz` 答 404；`Host: wiki.example.com` 的握手答 200；GET、DELETE 答 405 `Allow: POST`。
- **JSON-RPC 的批**：一个 `[{…tools/call echo…}, …]` 不带 `Mcp-Protocol-Version` 头、带 2025-03-26、带 2025-11-25 都答 400，`echo` 一次也没有执行（它数被调用的次数）；而且断言答复是 problem+json 的 `bad_request`：带 2025-11-25 时 SDK 自己也拒绝批，却是纯文字的 400（设计审查 A-M8），只看码分不出是边缘拒绝的还是 SDK。
- **panic**：`boom` 答 `internal_error`、日志里有 error 级的一行带栈（日志收集器，同 `bootstrap` 已有的），它的调用一行的 `outcome` 是 `internal_error`、带它的笔记本，同一个应用的下一次 `echo` 照常。
- **期限与断开**：`slow` 在 `server.request_timeout`（测试的 4 秒）之后答 `internal_error: the call ran out of time`、记 warn；客户端在 300 ms 时断开，`slow` 的 ctx 在远早于期限时（1 秒之内）取消，日志是 `FaultGone` 的 debug "MCP client went away"，不是期限的 warn（A-M12：只断言"结束了"测不出去掉 `AfterFunc` 的变异，它一样会在期限时结束），而且日志里没有 error 级的行（`ServeHTTP` 的最后一支经 `Write` 的分类记 debug、答 499；复核 A N-2：第一轮的写法每次断开记两行 error，而这个测试只断言有 debug 的那一行，照样通过）。
- **`no-store`**：200 的 `tools/call`、202 的通知、401、403、404、超过 64 KiB 的 413、`/.well-known/oauth-protected-resource` 的 404 与 `/register` 的 400 都带 `Cache-Control: no-store`（`TestEveryMCPAnswerIsNotStored`）。
- **没有 OAuth**：`TestTheMCPRefusesOAuthRegistration`：`POST /register` 答 400、`application/json`、`no-store` 与确切的 JSON，说明与从服务端读的 `/.well-known/` 的 404 的说明相同，GET 与 DELETE 同没人服务的路径（网页的 200 与 405）；`TestTheMCPTellsEveryClientOneHint`：两个入口没有令牌的 401、`/.well-known/` 的 404 与 `/register` 说同一句，失败的令牌与 REST 的 401 不变，挑战是 `Bearer`。
- **限流**：`TestTheMCPRoutesTakeTheBuckets`：小的桶用完时 PAT 的第三个 POST、失败的令牌之后的闸门、`/register` 的第二次都答 429 `rate_limited` 带 `Retry-After`，每个答复都 `no-store`（S1 审查 C-M1）。
- **工具错误的码**：整个程序的测试读到的每个工具错误经 `toolError` 对照契约声明的码（`ProblemCodes`；反向对照 `TestAToolErrorsCodeIsDeclared`），S2 起另对照按工具的码的表（3.13）。
- **协议版本**：官方 Go 客户端（默认的选项，先试 `server/discover` 再 `initialize`）协商出 2025-11-25；要 2025-06-18 的握手答 2025-06-18；只带 2026-07-28 的 `tools/call` 由 SDK 拒绝（设计审查实测的 400）；能力解码出 `tools` 而 `listChanged` 为假、没有 `logging` 与 `prompts`。
- **旧协议的 `subscriptions/listen`**：请求体是 `{"notifications":{"toolsListChanged":true,"promptsListChanged":true}}`（真的订阅；`params: {}` 立刻答参数不对、`{"notifications":{}}` 即使 `ListChanged` 为真也立刻答完，那样的测试什么也证明不了，设计审查 A-M9 实测），带 2025-11-25 的头：答 200 `text/event-stream` 并在请求之内答完（1 秒之内，而不是等到期限）。反向对照见 3.19。
- **`/.well-known/` 与 `/mcp/`**：`/.well-known/oauth-protected-resource`、`/.well-known/oauth-authorization-server`、`/.well-known/openid-configuration`、`/.well-known/x/y` 与 `/mcp/x` 都答 404 problem+json，不是前端的 `index.html`。
- **服务重启**：同一个库上建第二个应用，客户端经一个换了目标的反向代理（`httptest` 的代理），不重新握手就调用 `echo` 成功（无状态：没有会话）。
- **两个入口的工具**：账户入口的 `echo` 要 `notebook`；笔记本入口的 `echo` 拿到的 `Call.Notebook` 是地址里的。

### 3.5 S1：`api/mcp/tools.json` 与它的检查

文件是先写的契约（13.1 第 13 条的"先写描述"）：

```json
{
  "notebook": [ <the notebook entry's tools/list tools, by name> ],
  "account": [ <the account entry's tools/list tools, by name> ]
}
```

每项是 SDK 答出的工具对象的这些成员：`name`、`title`、`description`、`inputSchema`、`annotations`（SDK 总是写出 `idempotentHint` 与 `readOnlyHint`，`sdk/mcp/protocol.go:1997-2010`）。S1 两个都是空的数组；S2、S3 照 3.18 的全文加进去，审查读的就是 3.18。文件照仓库的 oxfmt 排版（`make lint-web` 的 `oxfmt --check .` 照常查它），不进 `make gen`。

```go
// The tools each entry lists are the contract's, api/mcp/tools.json,
// member by member (M9 design 4.1): a tool added, dropped or reworded
// without the file, or the file without the code, fails here with the
// difference. Nothing rewrites the file.
func TestTheToolListsAreTheContract(t *testing.T)
```

`bootstrap/mcp_tools_test.go`：经 serve 与真实的数据库（认证要读 PAT，笔记本入口要过 `Gate`），对两个入口各发一个原始的 `tools/list` POST（笔记本入口用一本新建的笔记本；先 `initialize`），取答复里 `result.tools` 的原样 JSON，与文件都解析成 `map[string]any`，按名称排、逐个工具以 `reflect.DeepEqual` 比较**整个工具对象**：不经官方 Go 客户端解码成 `mcp.Tool` 再比五个成员，那样线上多出的 `outputSchema`、`_meta`、`icons`、`execution` 都看不到（设计审查 A-M10）。不同时打印两边的 JSON（缩进的、成员排过序的）。文件恰好有 `notebook` 与 `account` 两个键，多一个、少一个、拼错都失败。它是整个程序的测试，在 `make test-go-bootstrap` 里跑；`make gen-check` 不跑 Go 测试（`Makefile:127-136`）。

**反向对照**（13.4 第 1 条）：

- `TestTheToolListCheckCatchesEachDifference`：比较的函数 `toolListDiff(got, want)` 抽出来，在合成的清单上各改一处，它都报出：少一个工具、多一个工具、一个说明不同、一个属性的类型不同、一个注解不同、多一个成员（`outputSchema`）、`required` 的次序不同、两个入口对调。
- `TestTheToolListCheckReadsTheApp`：经真的路径（serve、两次原始的 POST、读文件、比较），应用经测试的接缝多登记两个工具，`seam_notebook`（`NotebookEntry`）与 `seam_account`（`AccountEntry`）（`wiring.mcpTools`）：比较失败，报出的差异恰好是"the notebook entry lists seam_notebook…"与"the account entry lists seam_account…"；草稿的一个两个入口都登记的 `seam` 抓不到换掉或重复的入口（S1 审查 C-M5）。这一条守的是取数的一段（客户端 → `tools/list` → 映射；文件 → 键）：S1 两边都是空的，没有它，列错入口、读错键、什么也没读都会通过。

**它在各 Step 证明什么**：S1 两个入口与文件都是空的，主测试只证明"两边都空、文件的两个键在"，真正的证明是上面两条反向对照（取数的路径与比较的函数都会失败）；S2 起它核对 `list`、`read` 的全部成员，S3 再加 `notebooks`、`open_notebook`。

**接线的测试**（设计审查 A-I1、B-I4）：`TestTheMCPToolsBuildBothServers`（`bootstrap/mcp_tools_test.go`）以 serve 的同一条路建应用（`buildApp`，同一个 `mcpTools(pg, ln, nb)`，S2 起 `mcpTools(pg, as, ln, nb)`），`mcpserver.New` 的 panic 由测试 `recover` 并以 panic 的文字报出；再对 `mcpTools` 的每个工具的 `Spec()`（mcpserver 的访问器，3.3.2；复核 A N-4：`Tool.spec` 不导出，没有它测试读不到）核对它恰好在 `Entry` 说的入口的 `tools/list` 里、账户入口里 `Notebook` 的工具有必填的 `notebook`。`New` 的 panic 也会让 `TestTheToolListsAreTheContract` 与别的整个程序的测试失败，契约的文件也已定下哪个入口列哪个工具；这个测试多出的是与 `Spec.Entry` 的核对与点名的 panic 文字，留着它，代价是一个访问器。S1 的 `mcpTools` 是空的，它只证明空的工具集建得起两个 server；S2 证明 `list`、`read`（`BothEntries, Notebook: true`）；S3 证明 `notebooks`（`AccountEntry`）与 `open_notebook`（`AccountEntry, Notebook: true`）。变异（S3 核对）：把 `New` 的规则改回草稿的"`Notebook` 而不在笔记本入口就 panic"，这个测试以 `open_notebook` 的 panic 失败。

### 3.6 S1：按工具的权限矩阵的框架

`bootstrap/permission_matrix_mcp_test.go` 与 REST 的矩阵共用账户、工作区 `lab` 与笔记本（`permission_matrix_seeded_test.go` 的 `prepareMatrix`）：

- **PAT 的列**：笔记本级的 13 列（`notebookColumns()`，`permission_matrix_test.go:112-118`）各有一个 PAT，由那一列的账户经 `POST /api/v0/me/api-tokens` 建（2.1 第 10 条），存在这个测试自己的 `pats[c]`：在 MCP 矩阵测试自己的准备里建，不进共用的 `prepareMatrix`（它有六个使用者：`events_lab`、`notebook_visibility` 两个、`page_visibility` 两个、`permission_matrix`，设计审查 A-N8），别的测试不为它多付 13 次请求。只有笔记本级的 13 列有 PAT：`notebooks` 一行也只用这些列（3.17.4），工作区级的列不调 MCP。
- **行**：

```go
// mcpRow is a tool's row: how each column's call answers, through each
// entry the tool is listed in.
type mcpRow struct {
	tool    string
	entries mcpserver.Entry
	write   bool // the call writes: its cells run on a copy of the database (P2 on)
	args    func(d matrixData, c caller) map[string]any // the call's arguments, the account entry's notebook added by the runner
	want    func(c caller) mcpCell                      // the cell
	check   func(t *testing.T, d matrixData, c caller, a mcpAnswer) // the answer's content, when the cell is a success
}

// mcpCell is how a call answers: an HTTP status before the SDK, or a tool
// result with its error code ("" for a success).
type mcpCell struct {
	http int
	code string
}
```

- **格**：每列的目标是 `notebookOf(c)`（`permission_matrix_notebook_test.go:154-166`）。笔记本入口里看不到的列（与 REST 的 `listNodes` 一行答 404 的列相同：同一条规则 `node.list`，测试从那一行取期望，不另抄一份）答 HTTP 404 `notebook.not_found`；账户入口里它们答工具的错误 `notebook.not_found`；看得到的列（读者以上）读得到。P1 的工具都只读，没有 `forbidden` 的格；写的格（P2 起）用库的副本。
- **`TestThePermissionMatrixCoversEveryTool`**：`api/mcp/tools.json` 恰好有 `notebook` 与 `account` 两个键（拼错的键不会让覆盖永远通过）；每个入口的每个工具，在 `mcpRows()` 里恰好有一行、行的 `entries` 是它所在的入口；一行指着清单里没有的工具也报；`annotations.readOnlyHint` 为假的工具，它的行要有 `write`，为真的不能有（照 REST 的 `matrixViolations` 对写的行的检查，`bootstrap/permission_matrix_coverage_test.go:52-55`；设计审查 A-M11：P2 的写会继承这个框架，没有这一条它们的格就在读的那份库上跑）。反向对照 `TestMCPMatrixViolationsCatchesEachGap`：合成的清单与行，各缺一处（少一行、行多一个入口、行指着没有的工具、写的工具的行没有 `write`、读的工具的行有 `write`、文件少一个键）都报出，照 `TestMatrixViolationsCatchesEachGap`（`permission_matrix_coverage_test.go:160`）。
- **账户入口的 `notebook`**（S1 审查 B-M1）：runner 只在 `tools.json` 里这个入口的 schema 要 `notebook` 时给账户入口的调用加列的笔记本（`takesNotebook(readToolLists(t))`），S3 的 `notebooks` 不收它；`TestTheMatrixGivesANotebookAsTheFileSays` 在合成的清单上测（`notebook` 键下放一个诱饵）。S1 的试跑在生产的接线里加两个假的工具（只在账户入口、输入 `struct{}` 的 `nbs`，两个入口的 `echo`）与它们的行：39 格都通过；对照（每个账户入口的调用都加）`nbs` 的 13 格失败。
- **码**：每格的工具错误经 `toolError` 对照契约声明的码（3.4.9），S2 起另对照按工具的码的表（3.13）。
- **它在各 Step 证明什么**：S1 没有工具，矩阵没有行（`TestThePermissionMatrixOverMCP` 没有行时跳过）：覆盖的测试只证明文件的两个键在，函数会失败由反向对照证明（合成的数据）。S2 加 `list`、`read`，S3 加 `notebooks`、`open_notebook`（3.13、3.17.4），覆盖的测试从那时起核对真的清单与行。

### 3.7 S1：archtest

```go
// mcpSDKStaysInMCPServer: the MCP SDK and jsonschema-go are imported by
// platform/httpserver/mcpserver alone (M9 design 4.1): the modules
// register mcpserver.Tool values and never meet the SDK's types. The graph
// holds no test files, so bootstrap's tests, which drive the official Go
// client, are outside it.
```

照 `riverStaysInJobs`（`archtest/rules_test.go:326-336`）写，前缀 `github.com/modelcontextprotocol/go-sdk/` 与 `github.com/google/jsonschema-go/`。反向对照写进 `rules_cases_test.go` 的合成的边：`modules/page/adapter/mcp → github.com/modelcontextprotocol/go-sdk/mcp` 与 `platform/httpserver → github.com/google/jsonschema-go/jsonschema` 各报一次，`platform/httpserver/mcpserver → …/mcp` 不报。`archtest/binary_test.go` 的禁单不变：SDK 不带 `google/uuid`（`go-sdk@v1.8.0/go.mod`；源码里没有它的导入）、kin-openapi、testcontainers 与 docker，二进制的检查照样通过就是证明。

**测试文件**：archtest 的图不含测试文件（设计审查 A-N10），一个模块的测试导入 SDK 它看不到，而 00 号文档 4.1 只许 bootstrap 的测试。所以另加一条 depguard 规则（`server/.golangci.yml`，lint 看得到测试文件）：`github.com/modelcontextprotocol/go-sdk` 与 `github.com/google/jsonschema-go` 只许 `internal/platform/httpserver/mcpserver/` 与 `internal/bootstrap/` 之下的文件导入（`files` 用 `!` 排除这两处），desc 写"MCP SDK 只在 mcpserver 与组合根的测试（M9 4.1）"。它与 archtest 的规则重叠在生产代码上，各守一半：archtest 有合成的边的反向对照，depguard 管住测试文件；反向对照是 lint 的一次手工核对（在一个模块的 `_test.go` 里加导入，`make lint-go` 报出），记在 S1 的审查里。

**它在各 Step 证明什么**：S1 起 SDK 进了 `go.mod` 而只有 mcpserver 与 bootstrap 的测试导入它，规则从 S1 就不是空的（合成的边证明它会失败）；S2、S3 的 page 与 notebook 的 `adapter/mcp` 只导入 mcpserver，规则照样通过就是证明。

### 3.8 S2 的文件

| 文件 | 包 | 新/改 | 内容 |
|---|---|---|---|
| `page/domain/path.go` | page/domain | 新 | `MaxPathBytes`、`MaxPathSegments`、`Path`、`ParsePath`、`PathOf`、`PathNotFound`（3.9.1–3.9.2） |
| `page/domain/glob.go` | page/domain | 新 | `MaxGlobBytes`、`MaxGlobWildcards`、`MaxGlobWork`、`GlobEntryWork`、`Glob`、`ParseGlob`、`GlobState`、`Start`、`Step`、`Accepts`、`GlobNode`、`Walk(ctx, …)` 与它答的 `GlobWalk`（按树走与接着走，3.10.1） |
| `page/domain/version_token.go` | page/domain | 新 | `VersionToken`（3.9.3；M8 的 `domain.Version` 已占 `version.go` 与这个名字） |
| `page/domain/read.go` | page/domain | 新 | `DefaultReadLines`、`Window`、`CheckWindow`、`Slice`、`Cut`（计数函数由调用者交）、`InlineImage`、`InlineType` 与两个上限（3.11） |
| `shared/title.go` | shared | 改 | `TitleHolds(r)`：一个字符标题里能不能有，即 `CheckTitle` 逐字符的规则，`ParseGlob` 用它（3.10.1） |
| `page/domain/list.go` | page/domain | 新 | `DefaultListLimit` = 200、`MaxListLimit` = 1,000、`CheckListLimit`、`ChildrenCursor`、`GlobCursor`（3.10.2） |
| `page/app/ports.go` | page/app | 改 | `PathStore`、`PageStates`、`PageState`、`AliveNodes`、`AttachmentReader`、`Attachment`、`ErrNoAttachmentFile` |
| `page/app/authorize.go`、`gate.go` | page/app | 改 | `decideNotebook`：`NotebookGate`、`ListByPattern`、`ReadPath` 共用的笔记本的判定（S2 审查 C-N5） |
| `page/app/node_paths.go` | page/app | 新 | `NodePaths`（`Find`；`Paths` 随 P4，3.9.2） |
| `page/app/list_by_pattern.go`、`read_path.go` | page/app | 新 | 用例 `ListByPattern`（3.10.4）、`ReadPath`（3.11.1） |
| `page/adapter/postgres/queries/nodes.sql`、`contents.sql`、`trash.sql`、`store.go` | page/adapter/postgres | 改 | `RootByKey`、`ChildByKey`、`AliveNodes`、`PageStates`（3.9.2、3.10.3）；`nameTaken` 认 00042 的两个索引；`trash.sql` 的注释（父节点的索引）；`make gen` 重新生成 sqlc |
| `server/migrations/sql/00042_page_nodes_path_keys.sql`、`migrations/schema_test.go`、`server/sqlc.yaml` | migrations | 新、改 | 根与子节点的部分唯一索引，去掉 00012 的兄弟的索引（3.9.2）；索引的行与 `TestThePathKeysGoDownToTheSiblingsIndex`；sqlc 的 page 一项列上 00042 |
| `page/adapter/mcp/tools.go`、`list.go`、`read.go`、`text.go` | page/adapter/mcp（新包） | 新 | 工具 `list`、`read`；`PathNotFound` 等错误的文字；元数据行（3.10.5、3.11） |
| `page/module.go`、`page/tools.go` | page | 改、新 | `Module` 留下 MCP 的用例；`MaxContentBytes` 的别名；`ToolDeps`、`Tools(d)`（`page.NewNodePaths(pool)` 随 P4） |
| `asset/module.go`、`asset/attachment_reads.go` | asset | 改、新 | `Deps.PublicURL`；`Module` 留下 `reads`、`blobs`、`sniffer`；`AttachmentReads`、`Attachment`（3.11.2） |
| `asset/adapter/sniff/sniff.go`、`webp.go`、`testdata/*.webp`、`asset/domain/types.go`、`asset/app/blobs.go`、`ports.go` | asset | 改、新 | `webpSize` 与 `Sniffer.Size` 的分支（`Size` 取 `io.ReadSeeker`，以 seek 读文件的长度）；`asset/app.ImageSize(sniffer, r)`：上传与 `Content` 对旧行同一条只留 1–65,535 的边的规则；`HasSize` 加 `image/webp`（3.11.2） |
| `api/modules/asset.yaml`、生成的 `api.gen.go` 与前端的类型 | — | 改 | `width`、`height` 的说明"(PNG, JPEG, GIF)"加 WebP；`make gen`（3.11.2、第 7 节） |
| `platform/config/config.go`、`validate.go`、`configs/config.yaml` | config | 改 | `server.public_url`（3.12） |
| `bootstrap/mcp.go`、`deps.go`、`wire.go` | bootstrap | 改 | `mcpTools(pg, as, ln, nb)` 拼上 `pg.Tools(page.ToolDeps{Attachments: pageAttachments{as.AttachmentReads()}})`；`assetDeps` 交 `PublicURL`；`pageAttachments` 的适配与它的错误的换法 `pageAttachmentErr`（`asset.ErrNotFound` 到 `page.ErrNotFound`、`ErrNoFile` 到 `ErrNoAttachmentFile`） |
| `api/mcp/tools.json` | — | 改 | 两个入口加 `list`、`read`（3.18） |
| `page/domain/*_test.go`、`page/app/*_test.go`、`page/adapter/postgres/*_test.go`、`page/adapter/mcp/*_test.go`、`asset/attachment_reads_test.go`、`asset/adapter/sniff/*_test.go`、`asset/app/blobs_test.go`、`asset/domain/types_test.go` | — | 新、改 | 3.9–3.11 的单元、存储、计划与性质测试，WebP 的头与它的模糊测试，守卫；`TestHasSize` 的 `image/webp` 改为 `true`（`types_test.go:107`，复核 B N-3） |
| `bootstrap/mcp_reads_test.go`、`permission_matrix_mcp_test.go`、`mcp_client_test.go` | bootstrap | 新、改 | 3.13；按工具的码的表 `mcpToolCodes()` 与它的检查 |
| `Makefile`、`.github/workflows/ci.yml` | — | 改 | `test-go-costs` 加 glob 的守卫 `TestTheWorstGlobsStopWithinTheirWork`（3.10.1）；守卫那一份的作业时限由 20 分钟提到 25 分钟（S2 时作业已到 11:05，照约两倍的规则） |
| `e2e/fixtures/mcp.ts`、`e2e/fixtures/server.ts`、`e2e/fixtures/assets.ts`、`e2e/stories/mcp/mc2-read-list.spec.ts` | e2e | 新、改 | 脚本化的客户端、`NWIKI_SERVER__PUBLIC_URL`、`webpBytes`、MC2（第 4 节） |

### 3.9 S2：段、路径与版本令牌

#### 3.9.1 写法与段的检查（`page/domain/path.go`）

```go
// MaxPathBytes and MaxPathSegments bound a path an MCP tool names (M9
// design 4.8): ten levels of pages and an attachment.
const (
	MaxPathBytes    = 4096
	MaxPathSegments = MaxDepth + 1
)

// Path is a path from a notebook's root (M9 design 4.8).
type Path struct {
	Titles []string // each segment's title, the last a page's title without ".md" or an attachment's name
	Keys   []string // shared.TitleKey of each
	Kind   Kind     // the last segment's: KindPage when it ends with ".md", in any case
	Dir    bool     // list's "A/": the children of the page A; no Titles for the root
}

// ParsePath checks s as field, a path: one leading "/" may go; "./", ".."
// and empty segments are refused; a trailing "/" names a page's children
// when dir allows it (list), and is refused otherwise; each segment, the
// last without its ".md", is a title shared.CheckTitle keeps as it is, in
// NFC. A path that breaks a rule is 422 on field, before any read.
func ParsePath(field, s string, dir bool) (Path, error)

// PathOf is the path of node, whose ancestors from the root are ancestors:
// their titles, then its own, with ".md" for a page.
func PathOf(ancestors []Node, node Node) string
```

`ParsePath` 的次序：长于 4,096 字节 → `too_long`；去掉开头的一个 `/`；`dir` 时空的或以 `/` 结尾的是子节点（去掉结尾的一个 `/`），不 `dir` 时以 `/` 结尾 → `invalid_format`（"only list takes a path ending with /"）、空的 → `required`；按 `/` 分段，多于 11 段 → `too_long`；有空的段、`.`、`..` → `invalid_format`（"must not hold empty, . or .. segments"）；最后一段（不 `dir` 时）以 `.md` 结尾（ASCII 不区分大小写）是页，去掉 `.md` 是标题，否则是附件的名称；每段 `got, ferr := shared.CheckTitle(field, seg)`，要 `ferr == nil && got == norm.NFC.String(seg)`：`CheckTitle` 去掉首尾的空白之后再 NFC（`shared/title.go:23-43`），所以首尾有空白的段不相等、被拒（`A /B.md` 不会查不到 `A ` 再去建一个 `A`）；NFD 写的段照样收下，键是 NFC 之后算的。码照 `CheckTitle` 的（2.1 第 12 条），消息前加第几段："segment 2 must not contain / \ : * ? " < > | # ^ [ ] or control characters"。`CheckTitle` 收下而只是 `got != NFC(seg)` 的段（首尾的空白：`CheckTitle` 先去掉它，`shared/title.go:29`）没有 `CheckTitle` 的码，写定为 422 `invalid_format`，"segment 2 must not start or end with white space"（00 号文档 4.8 把它列为 `invalid_format`；设计审查 B-M7）；这时 `got` 与 NFC 之后的段只差首尾的空白，测试核对这一点，别的不等不会落到这一条。带 NUL 的段在这里被拒（`unshowable` 含 Cc），不到查询。

#### 3.9.2 找节点（`page/app/node_paths.go`）

```go
// NodePaths finds a notebook's nodes by their paths (M9 design 4.1, 4.8),
// unlocked; it decides nothing: its callers decided the notebook. P4 adds
// Paths, the paths of nodes by id, with linking, its one consumer.
type NodePaths struct{ store PathStore }

// Find is the node path names in notebookID, with its ancestors from the
// root: page.not_found, a *domain.PathNotFound, for none, for an
// attachment where the path has a page or the other way round, and for a
// path through an attachment.
func (p NodePaths) Find(ctx context.Context, notebookID uuid.UUID, path domain.Path) (Found, error)

// Found is a node a path named and its ancestors from the root.
type Found struct {
	Node      domain.Node
	Ancestors []domain.Node
}
```

草稿在 P1 另写了 `Paths`（按 id 给路径）并测它，而 P1 里没有人调它，唯一的使用者是 P4 的 linking（`backlinks` 的来源页）：它随 P4 来，P4 写它时照 B-M2 只为问到的 id 拼路径（设计审查 B 的过度设计第 1 条；第 8 节）。

```go
// PathStore looks nodes up by their title keys, unlocked.
type PathStore interface {
	RootByKey(ctx context.Context, notebookID uuid.UUID, key string) (domain.Node, bool, error)
	ChildByKey(ctx context.Context, notebookID, parentID uuid.UUID, key string) (domain.Node, bool, error)
}
```

```sql
-- name: RootByKey :one
-- The node not deleted at the notebook's root whose title key is name_key: a path's first segment (M9/P1 design
-- 3.9.2), by the roots' unique index (00042), its two columns equalities: a point lookup with statistics or without.
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at
FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id IS NULL AND name_key = sqlc.arg(name_key)
    AND deleted_at IS NULL;

-- name: ChildByKey :one
-- The child not deleted of parent_id whose title key is name_key: a path's next segment, by the children's unique
-- index (00042), its two columns equalities, the notebook checked on the row: a point lookup once the table is
-- analyzed. Without statistics the planner may read the notebook's nodes of the key by the titles' index (00019)
-- instead, 00042's header says why. IS NOT DISTINCT FROM would not use either (M9/P1 design 2.1, item 13).
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at
FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id = sqlc.arg(parent_id) AND name_key = sqlc.arg(name_key)
    AND deleted_at IS NULL;
```

**两条语句的索引**（S2 审查 A-I1、C-I2 与修复核对，主持者的决定；实施时的偏离 8 由它取代）：设计写的"都走兄弟的唯一索引"不成立。btree 只在每列都有 `=` 时把唯一索引估作一行，根下的 `parent_id IS NULL` 是 NullTest，`NULLS NOT DISTINCT` 的兄弟的索引得不到这份信用：C 量出 2 万个根各有子节点 `notes` 时，没有统计两条语句都走 00019 的 `(notebook_id, name_key)`、滤掉 2 万行，ANALYZE 之后 `RootByKey` 顺序扫描整张表；一个 2 万个子节点的文件夹，没有统计时走 `(notebook_id, parent_id)`。改写语句（`ORDER BY`、行的比较、显式的 `parent_id IS NOT NULL`）都移不动没有统计时的选择。所以迁移 `00042_page_nodes_path_keys.sql` 加两个部分唯一索引，`nodes_notebook_id_name_key_root_key ON nodes (notebook_id, name_key) WHERE parent_id IS NULL AND deleted_at IS NULL` 与 `nodes_parent_id_name_key_key ON nodes (parent_id, name_key) WHERE parent_id IS NOT NULL AND deleted_at IS NULL`，两列都是等值的条件；再去掉 00012 的 `nodes_notebook_id_parent_id_name_key_idx`：两个新的索引合起来就是它的约束（子节点由复合外键 `nodes_notebook_id_parent_id_fkey` 在父节点的笔记本里），行不相交，一个撞名恰好撞一个；留着它只多写的代价（约 1.8%）、多一个错误的名字，还让计划测试脆弱。Down 照 00012 重建它，再去掉两个。`nameTaken` 把两个新名字都认作 `ErrTitleTaken`（新建、改名、移动、恢复）。`RootByKey` 有没有统计、自定义与通用的计划都是根的索引的点查（2–4 个缓冲）。`ChildByKey` 在表 ANALYZE 之后是子节点的索引的点查；之前可能经 00019 的标题索引读这本笔记本里这个键的全部节点（A 的"只有文件夹"：1,000 个文件夹 × C1–C100，滤掉 999 行、1,002 个缓冲）：00019 的索引不唯一，B-tree 的去重把一个键的项折进一个 posting list，比任何唯一的子节点索引浅一层，下降的代价正是决定近乎平手的 0.12；实施者量了五种子节点的索引定义，没有一种补得上。主持者接受这个计划，到 autovacuum 第一次 ANALYZE 为止，不用 A 验证过的 `(notebook_id = $1) IS TRUE` 一类的规划器的技巧（第 8 节交 M12）。

`Find` 逐段点查（至多 11 条语句，每条一次索引的点查；计划测试 `TestThePathLookupsArePointLookups`：A 与 C 的四种形状（共用子节点的键的 1,000 个文件夹另加 5 万个别的笔记本的根、只有文件夹、2,002 本笔记本里共用的根键、2 万个兄弟），没有统计与 ANALYZE 之后、自定义与通用的计划，核对确切的索引名与它的每列都是等值的条件；`ChildByKey` 的各例在 ANALYZE 之后跑；`TestThePathKeysGoDownToTheSiblingsIndex` 核对 Up 与 Down 之后名称键的唯一索引的确切 `indexdef`）：第 i 段不是最后一段而找到的是附件 → 找不到（附件没有子节点）；最后一段的类型与路径的不同 → 找不到，带提示：路径是附件的写法而找到的是页时"A page's path ends with .md: "A/x.png.md"."，路径是页的写法而找到的是附件时"That is an attachment: its path is "A/x"."（`PathOf` 给出）；路径是 `A/y.md` 而那里有标题是 `y.md` 的页时"A page there is titled with the .md: its path is "A/y.md.md"."（多一次点查，S2 审查 A-N3）。错误是

```go
// PathNotFound is page.not_found for a path, with the path as written and
// a hint, which page's adapter/mcp writes out.
type PathNotFound struct {
	Path string
	Hint string
}

func (e *PathNotFound) Error() string { return ErrNotFound.Error() }
func (e *PathNotFound) Unwrap() error { return ErrNotFound }
```

读的工具在判定之后不加锁地找：各段之间并发的移动可能让找到的链不是任何一刻的路径，读接受它（答复的路径以找到的节点的名称写出）；写的工具在单元的锁下找（P2）。P1 里 `read` 经 `ReadPath` 用 `Find`；模块根的 `page.NewNodePaths(pool)`（只凭连接池，13.1 第 21 条的形状）与 linking 的 `Deps.NodePaths` 随 P4，最后一跳在 P4 的 `backlinks`。P1 在 page 的 app 与存储上测 `Find`（`page/app/node_paths_test.go` 的替身、`page/adapter/postgres` 的真实数据库），不建模块根的端口。

**与链接的规则不同的四处**（00 号文档 4.8 的表，M7 的移交第 3 项）：MCP 的路径不经 `ParseTarget`、`Resolve`，不做后缀匹配、不经别名。`A/x.png` 只指附件（页 `x.png` 写作 `A/x.png.md`）；`A/x.md` 只指页 `A/x`（标题是 `x.md` 的页写作 `A/x.md.md`）；没有扩展名的附件 `A/data` 有路径（`read` 的元数据里 `link` 为 null）；`A/` 只在 `list` 里、是 `A` 的子节点。四种各有表格测试。

**两种实现的一致**（13.4 第 10、13 条，`page/adapter/postgres` 的 `TestEveryNodesPathFindsIt`，真实的数据库）：随机的树（至多 6 层，标题以 `.md`、`.MD` 结尾的页，叫 `x.png` 的页，有与没有扩展名的附件，NFC、NFD 与大小写折叠的变体，`ß` 与 `SS`）上，每个活着的节点：`Find(ParsePath(PathOf(n)))` 找回的就是它；以它为模式的 `list`（不带通配的 glob）恰好列出它（路径在 `MaxGlobBytes` = 1,024 字节之内时；更深的由父节点的 `祖先/` 列出，五层 254 字节的标题的一例核对 `read` 找得到、字面的 `list` 答 `too_long`、父节点列得出它，S2 审查 A-M3）；它的父页的 `祖先/` 列出的子节点等于 `Children`。另以大写、NFD 写的路径找回同一个节点。变体可能越过上限（设计审查 B 的 Nit 9）：`ǰ` 大写之后由 2 字节变 3 字节，255 字节的标题就成了太长的段；255 字节的 NFC 韩文在 NFD 里是 510–765 字节，11 段可以超过 4,096。生成器为变体留出余量（变体的每段至多 255 字节、整个至多 4,096 字节才拿去找），越过的变体断言答 `too_long`，不当作找不到。

#### 3.9.3 版本令牌的写法（`page/domain/version_token.go`）

```go
// VersionToken is a page's version as MCP answers it (M9 design 4.10): its
// node and its content's revision, written "<node id>:<revision>". It is
// no Version, a row of a page's versions (M8 design 4.3).
type VersionToken struct {
	Node     uuid.UUID
	Revision int
}

// String is the token: the id in its canonical form, the revision in
// decimal without a sign or leading zeros.
func (v VersionToken) String() string { return v.Node.String() + ":" + strconv.Itoa(v.Revision) }
```

名字不是设计写的 `Version`：M8 的 `domain.Version`（`version.go`）是一页的版本的一行，已占这个名字（实施时的偏离 1）。P1 只写它（`list` 的页、`read`）；P2 加 `ParseVersionToken`：在最后一个 `:` 处分开、`uuid.Parse`、`strconv.Atoi`、版本号至少 1，而且 `String()` 等于原文（大写的 id、`+1`、`01` 都答 422 `base_version` `invalid_format`），所以每个版本恰好一种写法。单元测试核对写法（规范的小写 id、没有前导的零）。

### 3.10 S2：glob 与 `list`

#### 3.10.1 glob（`page/domain/glob.go`）

```go
// MaxGlobBytes and MaxGlobWildcards bound a glob (M9 design 4.8);
// MaxGlobWork bounds the matching of one list call: when it runs out, list
// stops and answers next_cursor, as at the answer's bound (3.10.2).
const (
	MaxGlobBytes     = 1024
	MaxGlobWildcards = 32
	MaxGlobWork      = 300_000_000 // a word × a key's byte; GlobEntryWork an entry, on entering its parent
	GlobEntryWork    = 256
)

// Glob is a pattern over the paths of a notebook's nodes (M9 design 4.8):
// * any characters within a segment, ? one, ** any segments. Compiled: a
// bit-parallel automaton per segment, and one over the segments.
type Glob struct{ /* segments: folded positions, masks; the segments' automaton */ }

// ParseGlob checks s as field, a glob, 422 on field when it breaks a rule.
func ParseGlob(field, s string) (Glob, error)

// GlobState is where a path's walk through the glob is: the set of the
// glob's segments matched so far. A node's state is its parent's stepped by
// its own key, so a walk of the tree steps each key once, not once a path.
type GlobState uint64

func (g Glob) Start() GlobState
// Step is st after one more segment of key at depth (the segments before
// it), adding its work, the mask words × len(key), to work; states that
// cannot reach the end within MaxPathSegments drop, so 0 means nothing
// below can match.
func (g Glob) Step(st GlobState, key string, depth int, work *int64) GlobState
func (g Glob) Accepts(st GlobState) bool

// GlobNode is a node with its children, as a glob's walk reads the tree:
// the use case groups the nodes it read by parent, once.
type GlobNode struct {
	Node     *Node
	Children []*GlobNode
}

// Walk walks the tree under roots in path-key order (each segment's
// NameKey, a page's last with ".md") after the path key after ("" from the
// start), each node's state stepped from its parent's: each(keys, n) for
// every node g accepts, until each answers false. It stops between nodes
// once its work reaches max, after at least one node was examined in this
// call, and answers the path key of the last node examined; resuming skips
// what sorts before after at each level and steps only after's ancestors.
// Every globCheck entries it looks at ctx, and answers its error once it
// ended.
func (g Glob) Walk(ctx context.Context, roots []*GlobNode, after string, max int64, each func(keys []string, n *GlobNode) bool,
) (GlobWalk, error)

// GlobWalk is what a walk did: the path key of the last node it examined;
// whether it stopped because its work reached its bound; its work, and
// the work before it examined its first node, re-entering the cursor's
// ancestors and the parents on the way.
type GlobWalk struct {
	Last    string
	Stopped bool
	Work    int64
	Before  int64
}
```

`Walk` 带 ctx（每 1,024 个项查一次）、答 `GlobWalk` 是实施时的偏离 2：`Work` 与 `Before` 只有测试读，`Before` 让测试给续走的重进定界（`TestTheGlobCountsItsWork` 在 10 万项的父节点里续走，核对 `Before` 恰好是它的项乘 256）。

- **写法**：长于 1,024 字节 → `too_long`，消息说更深的节点由它的父节点的路径加 `/`（`"A/B/"`）列出、由它的路径读（S2 审查 A-M3）；去掉开头的一个 `/`；以 `/` 结尾、空的段、`.`、`..` → `invalid_format`；`[`、`]`、`\` → `invalid_format`（"has no character classes or escapes"）；`**` 不独占一段（`a**b`）→ `invalid_format`；`*`、`?`、`**` 合计多于 32 个 → `invalid_format`（"holds more than 32 wildcards"）；不是 `**` 的段多于 11 个 → `too_long`（同路径的段数，它什么也配不上）。字面的部分里标题不能有的字符（`shared.CheckTitle` 禁的 `` : " < > | # ^ ``，与 `unshowable` 的字符，含 NUL 与控制字符）→ `invalid_format`（"segment 2 holds a character no title can hold"），段的首尾是空白 → `invalid_format`：它们什么也配不上，agent 打错了应当知道，与拒绝 `[ ] \` 是同一个道理（设计审查 B-M5 的 (a)：草稿在 3.13 说 `pattern` 带 NUL 答 422，而 `ParseGlob` 没有这条规则）。`{`、`}`、`,`、`!` 是普通的字符。空的模式、`/` 与以 `/` 结尾的不是 glob：`list` 先把它们当子节点（3.10.2）。
- **折叠**：每段按通配切成字面的片段，每个片段 `shared.TitleKey`；`.md` 的大小写随之不论。`?` 数的是折叠之后的字符（`ß` 折成 `ss` 是两个）。
- **一段之内：位并行的自动机**（Shift-And，Baeza-Yates 与 Gonnet）。去掉 `*` 之后一段有 m 个位置（字面的字符与 `?`），状态是 m + 1 位的掩码，⌈(m+1)/64⌉ 个 64 位的字；读键的一个字符是每个字一次"左移、与这个字符的掩码（它在哪些位置上、加上 `?` 的位置）、或上前面有 `*` 的位置（它们读任何字符都留着）"。整段读完、接受的位在就是配上。代价随键的字符数乘字数（计作键的字节数乘字数，见工作的上限），线性，没有回溯，对 `*` 与 `?` 是准确的（不要草稿的"贪心取最左的出现"的论证）。一个 glob 所有的段合计至多 1,024 个位置，所以所有段的字数合计至多约 16 + 段数。
- **段之间：段上的自动机**。状态 j 是"模式的前 j 段已配上"，`**` 是一个读任何一段都留着、并可以不读就进到下一段的状态，结尾的 `/**` 是"恰好一段"加 `**`（至少一段：`A/**` 是 `A` 之下的全部，不含 `A` 本身，也不含根下一个叫 `A` 的附件），别处的 `**` 可以是零段。状态集至多 11 + 32 + 2 个，放在一个 `uint64` 里。还要 k 段才能接受的状态在深度不够时去掉，所以深的模式在浅的子树里很快就空了。
- **按树走**：一个节点的状态是父节点的状态读它的键一步；子节点从它开始，所以每个键只读一次，代价随节点数增长，不随节点数乘深度（B-I5 的"按树的节点记住状态"）。状态空了的子树不往下走：模式开头的字面段因此自然地把走查收窄到那棵子树，不要草稿的 `Prefix`（它的定义把最后一段也算进去，`Guide/Setup.md` 会去找一个键是 `setup.md` 的节点，设计审查 B-M4；去掉它，这个问题也就没有了）。
- **次序**：按路径的键（每段的标题键以 `/` 连起，页的最后一段带 `.md`）走：每个父节点下的项是页自己（`键.md`）、页的子树（块 `键/`）与附件（`键`），按字节排；标题键里没有 `/`，所以一个页的子树在连起的次序里是连续的一块，排好每个父节点的项就是整串的次序（`a/b` 与 `a-c` 比的是 `/` 与 `-`，逐段比较会排错，设计审查 B-M2；这里比的正是整串在那一处的字节）。所以 `list` 按次序产出，不拼每个节点的路径、不排序全部匹配：栈上留着祖先的键（共享的切片），只为答出的那一页（至多 `limit` + 1 项）拼路径。走查是 `page/domain` 的 `Glob.Walk`：用例传读出的树与上限（`NewListByPattern` 自己设 `MaxGlobWork`，测试经 `export_test.go` 的 `WithMaxWork` 换小的；`TestAListsGlobsAreBoundByMaxGlobWork` 钉住生产的值，S2 审查 C-N1），守卫与走查的测试不连数据库直接调它。
- **为什么不编译成 RE2**：Go 的 `regexp` 也是线性的，但它对每个节点的整条路径从头跑，祖先的前缀不共享，每个字符的代价是程序的大小而不是字数：11 段、2,815 字节的一条路径上，163 字节与 1,024 字节的模式各 1.8 ms 与 1.5 ms（`exp/glob/re2_test.go`），10 万条这样的路径要 150–180 秒。
- **工作的上限**：一个单位是"一个字乘键的一个字节"，每个项另算 `GlobEntryWork` = 256（项的字符串、它在排序里的位置与状态的一步，本机约 250 ns），在进入父节点时为它的全部项一次算上。一次 `list` 至多 `MaxGlobWork` = 3 亿个单位（S2 审查 A-M2 之后由 4 亿降下，见实测），在项之间、这一次至少走过一个项之后检查；到了 `list` 就停，答已经配上的与 `next_cursor`（最后走过的那个项的路径键），与答复到 48 KiB 时停下是同一个意思："后面可能还有，拿游标接着列"；这一页可能是空的而有 `next_cursor`。不是错误，不答 422：键的次序已经给了接着走的地方，agent 接着调或先列一个文件夹（工具的说明写明，3.18）。另每 1,024 个项查一次 `ctx.Err()`。
  - **按字节而不是按字符**（复核 B N-1）：第一轮按字符计，而 ASCII 之外的字符每个约贵 1.6–1.9 倍（B 实测）：它的掩码不在 128 项的表里，要解码 UTF-8 再查映射。按字节计，`é` 记 2 个单位、`漢` 记 3 个，字数的因子照旧在模式那一边，也更便宜算（字数乘 `len(key)`）。另外，模式里不是 ASCII 的字符的低 10 位记进一个 1,024 位的筛子，键里的字符在筛子里没有的就不查映射、直接取 `?` 的掩码：汉字的模式在 `é` 的键上、或反过来，不付映射的查找（`exp/glob` 实测：`é` 的键上 `**/*需求?*/**` 到上限由 1.21 秒降到 0.61 秒）。
  - **进入父节点时算**（复核 B N-8）：每个父节点的项（页 `键.md`、块 `键/`、附件 `键`）在进入它时建好、排好，排序在那时就整个付了；第一轮按走过的项算，一个 10 万项的父节点只走一项就停时，排序不在上限里。现在进入时为每一项算 256。键的公共前缀长时排序比它记的单位贵：B 的守卫树的根 20 万个项、公共前缀约 250 字节，只排序就 0.09–0.23 秒，约是它记的 5,120 万个单位的 2–4 倍（主机负载 7–10 时量的）；仍在下面的用时之内（守卫的每一次都含它）。
- **接着走**（复核 B N-2）：游标 K 是上一次最后走过的那个项的路径键。从 K 接着走时，每一层只看 K 在这一层的那一段 c（K 的最后一段是项自己的字符串 `键.md` 或附件的 `键`；之前的各段加 `/`，是块 `键/`）：这一层里排在 c 之前的项都跳过，不走、不算它们的步（排好之后二分找到 c）；等于 c 的块进去，在下一层照样找；等于 c 的项上次已走过，跳过；之后的项照常走。所以接着走只重走 K 的祖先：至多 10 个块的一步，加上进入这些父节点时它们的项（照算，排序也照付）。K 的节点被删了或移走了，二分照样落到它之后的第一项：不重复、不跳过别的项。上限只在这一次走过一个项之后检查，所以每次调用至少走过一个新的项，`next_cursor` 总是严格往后（按键的字节序）。
  - **一次调用的界**：工作至多是 `MaxGlobWork` 加重进的工作，再加一个父节点的项乘 256 与两个项的步（检查在项之间：最后一次检查之后至多再进一个块、它的父节点与一个项）；重进的工作是 K 的祖先所在的至多 11 个父节点的项乘 256，加至多 10 个块的步。每次调用走过第一个新项之后的工作都是新的，所以合起来的新工作不超过一次走完；每次停下的调用的新工作至少是 `MaxGlobWork` 减去重进的工作，调用的次数至多约 ⌈总工作 /（`MaxGlobWork` − 重进）⌉ + 1。B 的守卫树上重进是根的 20 万个项、5,120 万个单位，上限的 17%（4 亿时 13%）。一个文件夹要有约 58.5 万个子节点（117 万个项；4 亿时约 78 万个子节点），重进才吃掉整个上限、每次只前进一项：交 M12（第 8 节；S2 修复核对第一轮 A2-N2）。
  - 草稿与第一轮只说"游标之后起"：从根重走而计数，第二次调用把上限花在回到 K 上、`next_cursor` 不动，agent 一直循环；从根重走而不计数，第 n 次调用的真实工作约是 n 倍的上限（B 的守卫树上最后一次就是不设上限的 6.3 秒）。
  - `exp/glob` 的 `TestResumeBoundsEachCall`：3,000 棵随机的树、小的上限，逐次接着走：合起来等于一次走完、游标严格往后、每次在上面的界之内、新工作之和不超过一次走完的。
- **实测**（`exp/glob/`，Apple M5 Max、go1.27.1，不连数据库；主机同时有别的负载，load average 4–10，各次的值随它浮动；与正则的翻译对照 30 万个随机的模式与路径，0 处不同）。树：真的树（共享祖先）是平衡的 10 叉 5 层（111,110 个节点）、4 叉 8 层（87,380）、10 层的链加 99,990 个叶子、10 万个平铺的，名称都是 255 字节；设计审查 B 的守卫树是 10 万条路径、每条 11 段、每段约 255 字节，彼此不共享（110 万个节点），键各由 `a`、`é`、`漢` 重复而成（1、2、3 个字节的字符，复核 B N-1）。模式：B 的 `**/*a?a?…?b*/**`（28 个 `?`）与 `**/*` 加 126 或 160 个字面字符与 28 个 `?` 加 `*/**`、`**/*u?u*/**`、三或五组 `*u*` 夹在 `**` 之间；草稿的 `**/*a?…?b`、`**/a*a*…*b/**`；1,024 字节上限处的长片段（900–1,000 个字面字符与 20–26 个 `?`）、几组 `*P*` 之间夹 `**`、`**/` 加 1,000 个字面字符加 `?/**`、15 个 `*a` 加 900 个 `a`；汉字的 `**/*需求?*/**`。守卫树上的模式的字面字符换成那棵树的字符。

| 树 | 不设上限 | 上限 4 亿个单位 |
|---|---|---|
| 平衡 10 叉 5 层，111,110 | 最慢 0.39 s（1,007 字节的模式，5.26 亿个单位）；B 的 28 个 `?` 0.08 s；126 个字面 0.12 s | 935 与 1,007 字节的两个模式停下（它们要 5.04 亿、5.26 亿），别的走完 |
| 平衡 4 叉 8 层，87,380 | 最慢 0.36 s（1,024 字节，4.78 亿） | 1,024 与 936 字节的停下，别的走完 |
| 链加叶子、平铺，各 10 万 | 最慢 0.30 s（平铺，1,007 字节，4.29 亿） | 平铺上 935 与 1,007 字节的停下 |
| B 的守卫树，110 万，键是 `a` | `**/*.md` 1.43 s；B 的 28 个 `?` 1.36 s；126 个字面 1.88 s；1,024 字节 6.3 s | 停在 0.36–0.65 s |
| 同，键是 `é` | — | 停在 0.32–0.82 s |
| 同，键是 `漢` | — | 停在 0.30–0.81 s |
| 平衡 10 叉 6 层，1,111,110 | — | 停在 0.26–0.46 s |
| 守卫树，从末尾附近的游标接着走一次（1,024 字节的模式） | — | `a` 0.37 s、`é` 0.33 s、`漢` 0.32 s |

  所以到了上限，一次调用的匹配在本机（负载约 8）至多约 0.8 秒，在慢 2.5 倍的主机上约 2 秒；目标是本机约 1 秒之内。到上限时每个单位 0.75–2.1 ns（单位少的便宜的模式、不是 ASCII 的键每单位更贵，但都在这个用时里）。第一轮的上限是 5 亿个单位、按字符计，`é` 的键上到上限约 1 秒（负载 4–5），所以降到 4 亿。

  **S2 之后是 3 亿**（S2 审查 A-M2，主持者的决定）：一个 255 字节的标题的键至多 510 字节（`և` 折叠为 `եւ`；A 扫过每个字符，最多 2 倍），守卫的三棵树的键与名字一样长；父节点的项的排序每项算 256，实际随键的公共前缀增长。同样 15 个最坏的模式在 `և` 的守卫树上 0.90 秒，比 `é` 慢约 1.36 倍，对 2 秒的时限只有 2.2 倍。守卫加 `և` 的树之后交错着量（每种模式取 3 次里最小、15 个模式里取最大，负载 6–19）：

| 树 | 4 亿 | 3 亿 |
|---|---|---|
| 平衡 10 叉 5 层 | 260、271 ms | 190、205 ms |
| 守卫树，键是 `a` | 518、539 ms | 404、414 ms |
| 同，`é` | 643、634 ms | 545、465 ms |
| 同，`漢` | 515、543 ms | 398、388 ms |
| 同，`և`（键是名字的两倍） | 888、879 ms | 684、657 ms |
| `և`，从末尾第 10 个根接着走 / 从开头附近接着走 | 200 / 327 ms | 206 / 277 ms |

  对 2 秒的时限，4 亿时最慢的 `և` 是 2.25 倍，3 亿时 2.9–3.04 倍；照 3 倍的规则 `MaxGlobWork` = 3 亿，代价是大的笔记本上多约三分之一次 `list` 的调用。实际的树上，只有 900 字节以上的模式在约 10 万个节点上会停下、要第二次调用。读树（`ListNodes`，10 万个节点约 70 ms）另算，交 M12。对照：草稿的贪心匹配在 B 的守卫树上、B 的形状要 12.1 秒（逐字符）或 3.2–5.9 秒（逐字节）。
- **时间的守卫**（13.4 第 8 条，`page/domain` 的 `TestTheWorstGlobsStopWithinTheirWork`，不连数据库，加进 `make test-go-costs`，`Makefile:209-221`）：B 的守卫树（键是 `a`、`é`、`漢`、`և` 的四棵，键都由 `TitleKey` 取）与平衡的 10 叉 5 层的树上，上面每个模式各走一次、上限是 `MaxGlobWork`：每次都停下或走完，工作不超过 `MaxGlobWork` 加一个父节点的项乘 256 与两个项的步，用时在 2 秒之内；守卫树上从末尾第 10 个根接着走一次（不能停下）、从开头附近再一次（1,024 字节的模式），同样在 2 秒之内。每次走查与每棵树最慢的一次以 `t.Logf` 打出（`-v`），CI 的日志里读得到。余量（3 亿）：门禁里最慢的 `և` 741 ms（负载 6–10，2.7 倍）、710 ms（负载约 5，2.8 倍）；S2 修复核对第一轮 A 单独跑三次（负载 8–13），`և` 0.76–0.82 秒，单次走查是 2.45–2.6 倍（守卫查的就是单次）；取 3 次里最小、正常负载下约 3 倍（0.66–0.68 秒）。与别的耗 CPU 的包在一个 `go test` 里跑时 A 与 C 各见它失败一次（C：32 秒的那一次之后单独跑最慢 745 ms），`make test-go-costs` 与 CI 的成本那一份都单独跑它；CI 上的耗时还没人读过（第 8 节交 M12）。单元测试另核对：单位的计数（`TestTheGlobCountsItsWork`：一段键的字节数乘字数，`é` 与 `漢` 的键按字节；进入父节点时每项 256，只走一项就停的 10 万项的父节点也记下它的全部项）、每段的结果与正则的翻译、与独立的 DP 预言（`oracleSegmentMatches`，取自 S2 审查 A 的测试）都相同（随机的性质测试，22,000 个模式；模式与键另有 62–65 与 126–129 个字符的段，即两个、三个字的状态，S2 审查 A-I2：原来的段都短于 64 个位置，跨字的进位与末位的字没有测试）；字面的 glob 含折叠之后 64 个字符以上的标题与差一个字符的诱饵；按树走与逐条路径匹配的结果相同、接着走（`TestTheGlobResumesFromItsCursor`：随机的树、小的上限，合起来等于一次走完、不重复也不跳过，`next_cursor` 严格往后，每次调用在上面的界之内，新工作之和不超过一次走完的；游标的节点被删了时接着走照样落到它之后）。

#### 3.10.2 `list` 的语义

- `pattern` 为空、`/` 或缺省：根的子节点；`祖先/`（以 `/` 结尾，`ParsePath(…, dir: true)`）：这个页的子节点（一层），这个页在树里按键找（同 `Find` 的规则，在读出的整棵树上），找不到或是附件答 `page.not_found`（`Guide.md/` 这样以页的写法加 `/` 的提示 "Guide.md is a page; list "Guide/" for its children."，名字是附件时提示它的路径，S2 审查 A-N3）；子节点按树的次序（`sort_order`，再按 id，同 `PreOrder` 与网页，`page/domain/tree.go:12`）。
- 别的 `pattern` 是 glob，按路径的键排（每段的标题键以 `/` 连起，页的最后一段带 `.md`；兄弟的键唯一、附件的名称不以 `.md` 结尾，所以没有相等的键）。不带通配的 glob 就是那个节点（至多一项）。没有匹配的答空的列表，不是错误。不带通配的 glob 没有配上、而它加上 `.md` 是一个页时（`list("Guide")`，有 `Guide.md`），答复另带 `hint`："Guide.md is a page; list "Guide/" for its children."（设计审查 B 的 Nit 11：静悄悄的空列表让 agent 以为没有）；走查时顺便知道，不另读。
- `limit` 1–1,000，默认 200（`domain.MaxListLimit`、`DefaultListLimit`）；之外答 422 `limit` `out_of_range`。
- **游标**（13.1 第 27 条，`shared.EncodeCursor`）：

```go
// ChildrenCursor is where a list of children goes on (M9 design 4.9):
// after the item of sort order O and id I.
type ChildrenCursor struct {
	O float64   `json:"o"`
	I uuid.UUID `json:"i"`
}

// GlobCursor is where a glob's list goes on: after the path key K. Neither
// binds the pattern: an item moved by a concurrent write is neither
// repeated nor skipped among the others.
type GlobCursor struct {
	K string `json:"k"`
}
```

  两种载荷各是一个类型、没有 `omitempty`（设计审查 B-M6 与 B 的过度设计第 2 条）：模式先解析，游标解进模式说的那一种；`shared.DecodeCursor` 逐字节地重编码再比（`shared/cursor.go:59`），所以 `{}`、缺 `o` 或 `i`、子节点的模式拿到 `{"k":…}`、glob 拿到 `{"o":…,"i":…}` 都重编码不回原样，答 400 `bad_request`（`cursor`），不要草稿的"形状与模式不配"那一步与它的指针。`float64` 的 JSON 写法往返不变（Go 写出能读回同一个值的最短写法），`sort_order` 排除了 NaN 与无穷（`00012_page_nodes.sql:16-18`）。兄弟的次序值被重新编号时（间隔小于 1e-9，`page/domain/order.go:3-40`）每个兄弟的次序值都变，跨过一次重新编号的（o, i）游标可能重复或跳过别的项：罕见，接受（设计审查 B 的 Nit 12）。glob 的键可以比名称长：完全折叠把 `ŉ`、`İ` 由 2 字节变 3 字节，一段的键至多约 1.5 × 255 字节，键至多约 5.6 KiB（B 的 Nit 2）；游标经 JSON 的转义与 base64 之后至多 22,484 字节（S2 审查 A-N4 量的最坏的真的游标），仍放得下。
- **每项**：

```json
{"path": "Guide/Setup.md", "kind": "page", "has_children": false,
 "version": "0192f3a4-…:3", "byte_size": 1204, "updated_at": "2026-10-17T08:30:00Z"}
{"path": "Guide/diagram.png", "kind": "attachment", "has_children": false, "mime": "image/png", "byte_size": 34567}
```

  页的 `version`、`byte_size`、`updated_at`（正文的最后改动）来自 `PageStates`；附件的 `mime`、`byte_size` 来自 page 已有的 `Deps.Attachments`（`page/app/ports.go:389-400`）。读树与读状态之间被删除的节点不列（不记日志，同 asset 的列表）：页由 `PageStates` 的 `deleted_at IS NULL` 去掉；附件的信息（`AttachmentInfos.Of`）不论删没删，所以这一页有附件时另以一条 `AliveNodes`（`id = ANY($1) AND deleted_at IS NULL`，经 `nodes_pkey`，计划测试 5 万个节点）去掉其间进了回收站的（S2 审查 A-N2）。附件与页一起按 `sort_order` 排，网页在自己的面板里按名字列附件：照设计（S2 审查 A-N4 只是指出）。`kind` 对模型写作 `attachment`（不是存储的 `asset`）。
- **答复**：`{"items": [...], "next_cursor": "…"}`（另可有 `hint`），`next_cursor` 没有下一页时为 null。它在三种情形下不为 null，意思相同（"后面可能还有，拿它接着列"）：这一页到了 `limit`、答复到了 48 KiB、glob 的匹配用完了 `MaxGlobWork`（3.10.1；这时 `items` 可能是空的）。

#### 3.10.3 `PageStates`

```go
// PageStates reads each page's version, size and last write of ids, alive,
// unlocked: one statement, planned with its argument (M9 design 4.1).
PageStates(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]PageState, error)

// PageState is a page's content as list tells it.
type PageState struct {
	Revision  int
	Bytes     int
	UpdatedAt time.Time
}
```

```sql
-- name: PageStates :many
-- The revision, size and last write of each content not deleted of the pages (M9/P1 design 3.10.3): list's pages.
-- The array is a page of list, at most 1,000: the store plans it with its argument.
SELECT node_id, revision, byte_size, updated_at FROM page_contents
WHERE node_id = ANY(sqlc.arg(node_ids)::uuid[]) AND deleted_at IS NULL;
```

经 `planned(ctx)`（`page/adapter/postgres/store.go:43-45`）；计划测试（13.4 第 9 条）：1,000 个 id 在没有统计时走 `page_contents_pkey`。今天的 `ContentSizes` 不给版本号，照留给导出。

#### 3.10.4 用例 `ListByPattern`（`page/app/list_by_pattern.go`）

```go
// ListByPattern lists a notebook's nodes for MCP's list (M9 design 4.9).
type ListByPattern struct {
	notebooks   Notebooks
	nodes       Nodes
	states      PageStates
	attachments AttachmentInfos
	auth        shared.Authorizer
	// maxWork bounds a glob's matching a call: domain.MaxGlobWork, a
	// test's less.
	maxWork int64
}

// Execute lists the nodes of notebookID that pattern names, the page that
// limit and cursor ask for, unlocked, without a transaction.
func (l *ListByPattern) Execute(ctx context.Context, notebookID uuid.UUID, pattern string, limit *int, cursor *string) (Listing, error)

// Listing is a page of list: each item with the cursor that goes on after
// it, so the adapter can stop at the answer's bound; Next, when more may
// follow, the cursor after the page: its last item found, or the last node
// a glob examined when its work ran out, which may be past every item, or
// with none. Hint is set for a glob without wildcards that matched nothing
// while its path with ".md" is a page.
type Listing struct {
	Items []Listed
	Next  string
	Hint  string
}

type Listed struct {
	Path        string
	Node        domain.Node
	HasChildren bool
	Page        *PageState
	Attachment  *AttachmentInfo
	Cursor      string
}
```

**码的次序**（13.1 第 4 条；00 号文档 4.15 的"通用"）：`RequireActor` → `pattern` 422 → 游标解不开（解进模式说的那一种，3.10.2）400 → `limit` 422 → 笔记本（零值的 id、`WorkspaceOf` 找不到、判定 `node.list` 看不到）`notebook.not_found`（`decideNotebook`，`Gate` 与 `ReadPath` 共用；`WorkspaceOf` 失败时照原样答）→ 读树（`ListNodes`）→ `祖先/` 的页 `page.not_found` → 子节点按树的次序取游标之后的 `limit + 1` 项，或 glob 按 3.10.1 走（游标之后起：跳过之前的项、只重走游标的祖先；按路径键的次序、找到 `limit + 1` 项或用完 `MaxGlobWork` 就停）→ `PageStates` 与 `Attachments.Of`（只为这一页的项），这一页有附件时再 `AliveNodes`。草稿把 400 排在 422 之前、再加一步"游标与模式不配"；两种载荷之后游标要按模式解，所以 `pattern` 的 422 在前（设计审查 B-M6）。

**不把每个节点的路径拼出来**（设计审查 B-M2）：草稿"树与路径在 Go 里拼一次"在 10 万个节点、10 层长名称的祖先时要约 280 MB 的路径加差不多的键，`**/*.md` 还要全部排序；一个写者可以经导入建出这样的树，调用又是并发的。改为：读出的节点按父节点分组成子节点的列表（指针，一次 O(n)），子节点的模式只看那一个父节点；glob 按 3.10.1 在树上走，栈上留着祖先的键（共享的切片），按路径键的次序产出，路径只为答出的至多 `limit` + 1 项拼。内存是树本身加每个父节点排好的项，不随路径的长度乘节点数。读树本身（`ListNodes`，10 万个节点约 70 ms，[M12 的性能移交](../M12-release/handoffs/M4-performance.md)第 48 行）与它的内存交 M12（第 8 节）。

#### 3.10.5 `list` 的工具（`page/adapter/mcp/list.go`）

```go
type listIn struct {
	Pattern *string `json:"pattern,omitempty" jsonschema:"…"`
	Limit   *int    `json:"limit,omitempty" jsonschema:"…"`
	Cursor  *string `json:"cursor,omitempty" jsonschema:"…"`
}
```

`Spec{Name: "list", Title: "List nodes", Annotations: {ReadOnly: true}, Entry: BothEntries, Notebook: true}`；说明与 schema 见 3.18。处理函数调 `ListByPattern`，再经 `mcpserver.Fit(len(Items), build)`（3.4.4）：`build(k)` 是前 k 项加 `next_cursor`（k 小于项数时是第 k 项的游标；全放下时是 `Next`，没有时为 null：`Listing` 不另有 `More`，`Next` 在到了 `limit` 或用完工作时都设上，同样的三种情形，实施时的偏离 3），量是整个答复的 `TextBytes`，适配器不自己数。一项的路径至多 2,818 字节、全是 `&` 时（标题里能有的最贵的字符，六个字节）至多约 17 KiB，游标至多 22,484 字节（键至多约 5.6 KiB，经转义与 base64），所以最坏也放得下一项；通常一项一两百字节。错误照 3.3.6；`PathNotFound` 写成 `ToolError{Text: "page.not_found: Nothing is at "A/B/". …"}`。

### 3.11 S2：`read`

#### 3.11.1 页（`page/domain/read.go`、`page/app/read_path.go`）

```go
// DefaultReadLines is read's limit when none is given.
const DefaultReadLines = 2000

// Window is what of a page read asks for.
type Window struct{ Offset, Column, Limit int }

// CheckWindow checks offset (from 1), column (from 0) and limit (from 1),
// nil for the defaults: out_of_range on each, before any read.
func CheckWindow(offset, column, limit *int) (Window, error)

// Slice is the part of a content read gives.
type Slice struct {
	Text                   string // the content's bytes from the window's start, exactly
	Lines, TotalLines      int
	NextOffset, NextColumn int    // 0: none
}

// Cut cuts content for w within budget, as measure counts a text block
// (mcpserver.TextBytes, which page's adapter/mcp passes: page counts no
// bytes of its own): column out_of_range when w.Column is not on a
// character's boundary of its line or past it, or when the line is past
// the end and the column is not 0.
func Cut(content string, w Window, budget int, measure func(string) int) (Slice, error)
```

- **行**：以 `\n` 分行，`\r` 留在行里；最后的 `\n` 结束最后一行而不另起一行；空的正文没有行（`TotalLines` 0）。正文原样：frontmatter、BOM、`\r\n` 都在（`page/domain/content.go:10-24`）。
- **一段**：从第 `Offset` 行的第 `Column` 个字节起，一行一行地加（含它的 `\n`），加上之后超过 `budget` 就停；第一行就放不下时取它放得下的最长的前缀（在字符的边界上，`TextBytes` 计），`NextOffset` 还是这一行、`NextColumn` 是切处；到了 `Limit` 行而后面还有，`NextOffset` 是下一行、`NextColumn` 为 0（答 null）。`Text` 是正文里的一段原样的字节，依次读完的各段连起来就是正文（性质测试），文字里不加截断的标记：读回来整页写回的 agent 不会写进标记。
- **`offset` 超过末尾**：零行，`NextOffset` 为 0，不是错误；这时 `column` 只能是 0。
- **`column`**：这一行里第几个字节（从 0 起），要在字符的边界上、至多是这一行（不含 `\n`，含 `\r`）的长度，否则 422 `column` `out_of_range`（排在 404 与 403 之后：要读到正文才知道）。`column` 为负、`offset`、`limit` 小于 1 是与资源无关的 422，在笔记本之前。`limit` 没有上限（字节数管着，2.1 第 6 条）。
- **预算**：`page/adapter/mcp` 先写出元数据那一行（下一行的数要等切完才知道，所以先以各成员取最宽时写一份：整数 20 位、`next_column` 非空），`budget = mcpserver.MaxAnswerBytes − mcpserver.TextBytes(那一行)`，再 `Cut(…, budget, mcpserver.TextBytes)`；真的元数据行从不比最宽的那份长，所以两块合计在上限之内（测试核对）。路径至多 2,818 字节（11 段、每段 255 字节、10 个 `/`、`.md`；全是 `&` 时计 6 倍），所以文字的预算通常约 47 KiB、至少约 31 KiB。草稿在 page 里另有一份 `TextBytes` 与 `MaxAnswerBytes` 和核对两份一致的测试；改为由适配器交 mcpserver 的计数函数（3.4.4），page 里没有第二份。

```go
// ReadPath reads a page by its path for MCP's read (M9 design 4.9), or
// finds the attachment there, which the adapter reads through asset.
type ReadPath struct {
	notebooks Notebooks
	paths     NodePaths // Find (3.9.2)
	nodes     Nodes
	auth      shared.Authorizer
}

func (r *ReadPath) Execute(ctx context.Context, notebookID uuid.UUID, path string, offset, column, limit *int) (Read, error)

// Read is a read's node: a page's slice, or an attachment to read.
type Read struct {
	Path       string // the node's path, as its names are
	Node       domain.Node
	Page       *PageRead
}

type PageRead struct {
	Version  domain.VersionToken
	Window   domain.Window
	Slice    domain.Slice
	Bytes    int
}
```

**码的次序**：`RequireActor` → `path` 422（`ParsePath`，不收结尾的 `/`）→ `offset`、`column`（正负）、`limit` 422 → 笔记本 `notebook.not_found`（零值的 id 不查库；`WorkspaceOf`；判定 `page.read`，读者以上；`decideNotebook`）→ 节点 `page.not_found`（`NodePaths.Find`）→ 页：正文（`Nodes.PageContent`，读到之前被删了答 `page.not_found`，同 `GetPageContent`，`page/app/get_page_content.go:28-37`；文字是照写的路径 `Nothing is at "<path>".`，不是笼统的 "No such page."，S2 审查 C-N3）→ `column` 不在边界上或超过这一行 422 → 切。附件：`Read{Path, Node}` 交回适配器（3.11.2）。没有 403：读者以上的每种角色都能读（规则 `page.read`、`asset.read`，`access/domain/rules.go:92-93`、`:129`），看不到的已是 404。`TestReadPathAnswersInOrder` 钉住调用的次序 `WorkspaceOf`、`Authorize page.read`、`RootByKey`、`PageContent`：没有授权时存在的 `Guide.md` 与不存在的 `Missing.md` 都答 `notebook.not_found`、不找路径；先找路径的变体会让看不到的笔记本与其中的路径的存在都泄露（S2 审查 C-I1）。不取解析的预算：只读字节，不解析（00 号文档 4.9）。

**答复**（`page/adapter/mcp/read.go`，没有 `structuredContent`）：

```
block 1 (text): {"path":"Guide/Long.md","kind":"page","version":"0192f3a4-…:7","offset":1,"column":0,"lines":312,"total_lines":2051,"byte_size":187340,"next_offset":313,"next_column":null}
block 2 (text): the slice's bytes, exactly
```

元数据是一行 JSON（mcpserver 的同一个编码，`SetEscapeHTML(false)`），`lines` 是这次给出的行数（切开的一行算一行），零行时第二块是空的字符串（照样给，客户端少一种情形；mcpserver 的空的文字块，S1 审查 B-I1）。

#### 3.11.2 附件（`asset` 的 `AttachmentReads`）

```go
// AttachmentReads reads an attachment for page's MCP read (M9 design 4.1,
// 4.9): decided on asset.read as GET /api/v0/assets/{node_id} decides it.
func (m *Module) AttachmentReads() AttachmentReads

type AttachmentReads struct {
	reads     *app.Reads
	blobs     app.Blobs
	publicURL string
	logger    *slog.Logger
}

// Read reads the attachment nodeID: asset.not_found for none and for one
// the caller cannot read; its row, its link's writing, and its content's
// address signed now, absolute: server.public_url and the content route's
// path and query; "" without public_url.
func (a AttachmentReads) Read(ctx context.Context, nodeID uuid.UUID) (Attachment, error)

// Content reads the file of at, at most max bytes: ErrNoFile, logged at
// warn as a download logs it, when the store has none; an error when its
// size is not its row's. The sides are the row's or, for an image whose
// row has none (a WebP uploaded before M9/P1), read from data's header by
// app.ImageSize, the upload's rule; 0 and 0 when neither knows them.
func (a AttachmentReads) Content(ctx context.Context, at Attachment, max int64) (data []byte, width, height int, err error)

type Attachment struct {
	NodeID, BlobID       uuid.UUID
	Name, MIME, Link     string
	Bytes                int64
	SHA256               []byte
	Width, Height        int // the row's; 0 when the server read no size
	URL                  string
	Expires              time.Time
}
```

`Read` 是 `Reads.Get`（`asset/app/reads.go:57-91`：找节点、判定、读行、读链接、签名）加上绝对的地址 `publicURL + httpadapter.ContentURL(n.ID, b.ID, s, true)`（`asset/adapter/http/upload.go:186-194`，模块根可以用自己的适配器）：下载的写法（`d=1`，`Content-Disposition: attachment` 带文件名）。地址是给 agent 取文件的（能看的图片已在第二块内联），存下来的文件因此带它的名称；签名只盖节点、blob、`e` 与 `d`（`asset/adapter/mac/sign.go:44-62`），换源照样成立（设计审查 B 的 Nit 7，两种都行，取下载的）。`Content` 经 `Blobs.Open`（`asset/blobs.go:124-134`）与 `io.LimitReader(f, max+1)`，读到的字节数与行的 `byte_size` 不同时是错误（内部的：文件与它的行不符）。`asset.ErrNotFound` 是 asset 的 `domain.ErrNotFound` 的别名，组合根的 `pageAttachmentErr` 把它（包着与否）换成 `page.ErrNotFound`、`ErrNoFile` 换成 `ErrNoAttachmentFile`，别的不变（`TestPageAttachmentErrors`，S2 审查 B-M3、C-M3）。`asset.Deps` 加 `PublicURL string`（组合根交去掉结尾的 `/` 的 `cfg.Server.PublicURL`；源带 `/` 的一例经整个程序核对，同上）；`asset.New` 把 `blobs` 与 `reads` 留在 `Module` 上。不经公开的下载路由：不走 `asset_content` 的桶，也不让 MCP 的调用依赖签名的地址。

page 一侧（页不导入 asset）：

```go
// AttachmentReader reads an attachment for read: bootstrap adapts asset's
// AttachmentReads to it, asset.not_found to page.not_found and its
// ErrNoFile to ErrNoAttachmentFile.
type AttachmentReader interface {
	Read(ctx context.Context, nodeID uuid.UUID) (Attachment, error)
	Content(ctx context.Context, a Attachment, max int64) (data []byte, width, height int, err error)
}

// ToolDeps are what page's tools need beyond the module's use cases: asset
// is wired after page (v0.1 design 13.1, item 11).
type ToolDeps struct{ Attachments AttachmentReader }

// Tools are page's MCP tools; no Attachments panics.
func (m *Module) Tools(d ToolDeps) []mcpserver.Tool
```

```go
// InlineImage reports whether read gives an attachment's bytes as an
// image (M9 design 4.9): PNG, JPEG, GIF or WebP, at most
// MaxInlineImageBytes, both sides known and at most MaxInlineImageSide.
func InlineImage(mime string, bytes int64, width, height int) bool

const (
	MaxInlineImageBytes = 3 << 20
	MaxInlineImageSide  = 8000
)
```

**附件的答复**：

```
block 1 (text): {"path":"Guide/diagram.png","kind":"attachment","mime":"image/png","byte_size":34567,"sha256":"9f2c…","width":640,"height":480,"link":"diagram.png","url":"https://wiki.example.com/api/v0/assets/…/content?b=…&e=…&s=…&d=1","expires_at":"2026-10-17T10:00:00Z"}
block 2: an image (InlineImage), or a resource_link {uri: url, name, mimeType, size} (not inline, url set), or nothing
```

**内联的次序**（不论有没有 `server.public_url`：内联不要地址，只有下载的链接要；主持者的决定，实施时的偏离 10 照工具的说明、说明的文字与 3.13 写的"只给元数据"不内联，审查之后改回）：MIME 是四种之一（`InlineType`，读字节之前先看类型）、`byte_size` 至多 3 MiB、行的宽高未知或都至多 8,000 时调 `Content`（3 MiB），宽高以它答的为准（行有的就是行的；没有时从读到的字节的头里读出），再 `InlineImage`：宽高已知而且在上限之内才内联，否则给 `resource_link`。所以 P1 之前上传的 WebP（行里没有宽高，下一节）照样内联，不另迁移；宽高读不出时（头坏了）给地址。字节可能白读（读出来的边超过 8,000）：至多 3 MiB，接受。

`link` 是 REST 的链接目标，即 `![[…]]` 或 `[[…]]` 里写的那一段（例子里是 `diagram.png`；没有扩展名时 null，同 REST；设计原来的例子 `![[Guide/diagram.png]]` 不是 REST 的值，实施时的偏离 17）；`width`、`height` 是内联的判断用的那一对（行的或从字节读出的），都未知时为 null；`public_url` 为空时没有下载的地址：图片照样内联，`url`、`expires_at` 为 null；不内联的附件另加 `"note": "server.public_url is not set, so there is no address to download it; ask the server's admin."`；文件丢了（`ErrNoAttachmentFile`）时没有第二块，`note` 是 "The attachment's file is missing from the server's storage."。SVG（`image/svg+xml`）不内联：它是 XML，模型不当图看（00 号文档 4.9）。内联的图片不计入 `MaxAnswerBytes`（它自己有 3 MiB 的上限）；`resource_link` 的成员很短（名称至多 255 字节、地址是源加签名的路径），计入元数据。签出的地址进了模型的上下文，失去访问之后至多 2 小时仍能下载：同 M7 的风险，接受（00 号文档第 10 节）。

**码的次序**（附件）：同页到"节点"为止 → `AttachmentReads.Read`（它再判定 `asset.read`，只有并发地失去访问时才答 `asset.not_found`，组合根换成 `page.not_found`）→ 可能内联时 `Content`（3 MiB）。`Read` 与 `Content` 各自之前附件走了（进了回收站、失去访问）答照写的路径的 `page.not_found: Nothing is at "<path>".`（S2 审查 C-N3 与修复核对第一轮 B）。`column` 与 `limit` 对附件不起作用（不答 422）。文件丢了只在调 `Content` 时发现：PDF 与别的不内联的类型不读字节，所以"删了文件的附件答 `note`、记 warn"的测试用一个 PNG（设计审查 B 的 Nit 6）。

**WebP 的宽高**（设计审查 B-I3）：今天服务端只读 PNG、JPEG、GIF 的宽高（`asset/domain/types.go:107-111` 的 `HasSize`；`asset/adapter/sniff/sniff.go` 只导入这三个解码器；`asset/app/blobs.go:82` 只在 `HasSize` 时存），WebP 的宽高总是 0，`InlineImage` 就从不内联它，与 00 号文档 4.9、第 11 节的取舍第 7 条（"PNG、JPEG、GIF、WebP 在 3 MiB、每边 8,000 像素之内内联"）相悖。`golang.org/x/image` 不在 `go.mod` 里，也不在模块的缓存里；不加依赖，自己读 RIFF 头：

```go
// webpSize reads a WebP's width and height from the first bytes of its
// RIFF container (RFC 9649): "RIFF", a size, "WEBP", then the first chunk:
// VP8 (lossy: the frame tag, the start code 9d 01 2a, then 14-bit width
// and height), VP8L (lossless: the signature 0x2f, then 14-bit width-1 and
// height-1, the alpha bit, and 3 version bits, which must be 0) or VP8X
// (extended, a chunk of 10 bytes: 24-bit canvas width-1 and height-1). ok
// is false for anything else, for a header cut short, and for a RIFF
// larger than the file (length bytes) or a first chunk larger than the
// RIFF: a file cut short, which a decoder refuses; so is a VP8L of
// another version or a VP8X chunk of another size, which libwebp and
// x/image refuse.
func webpSize(head []byte, length int64) (width, height int, ok bool)
```

- **只凭文件头担保的**（S2 审查 B 的存疑 1、修复核对第一轮 B，主持者的决定）：截短的 WebP、版本位不是 0 的 VP8L 原来也给宽高、也内联；模型的接口拒绝这张图时，客户端的对话可能一直失败（图片留在历史里）。所以 RIFF 的大小加 8 要至多是文件的长度、第一个块的大小加 20 要至多到 RIFF 的末尾（libwebp 的 `ParseRIFF` 与 `ParseVP8Header` 的同两条），VP8L 的版本位要是 0（RFC 9649），VP8X 的块要恰好 10 字节，否则没有宽高、就不内联、给链接。`Sniffer.Size` 取 `io.ReadSeeker`，以 seek 读文件的长度（上传的 `File` 是 `ReadSeekCloser`，`Content` 交整个文件的 `bytes.Reader`）。核对者对 x/image 的差分：没有拒绝一个 x/image 解得出的一致的文件，只多拒绝比它的 RIFF 说的短的文件（缺了填充字节的一类，libwebp 答 NOT_ENOUGH_DATA），是安全的一边。PNG 与 JPEG 正文坏了而头有效的，照头内联：接受。
- 在 `asset/adapter/sniff` 里、`image.DecodeConfig` 旁边：`Sniffer.Size` 把 1 MiB 的 `LimitReader` 包进一个 `bufio.Reader`，先 `Peek(30)`：以 `RIFF`、`WEBP` 开头的交 `webpSize`，别的照旧交 `image.DecodeConfig`（读同一个 `bufio.Reader`，从头读起）。读到的不足 30 个字节（比 30 个字节短的文件，`Peek` 同时答 `io.EOF`）不是错误：照样看读到的前缀，`webpSize` 对截短的头答 `ok == false`，别的交 `DecodeConfig`；一个合法的 GIF 可以只有二十几个字节（复核 B N-3）。不以 `image.RegisterFormat` 登记一个格式：那会改变整个进程里 `image.Decode` 认得的东西，而我们只要宽高。
- `asset/domain/types.go:110` 的 `HasSize` 加 `image/webp`：上传从此也记下 WebP 的宽高（同样只留 1–65,535 的边，`blobs.go:97-110`）。**这改了 M7 的上传的行为**（简报写作 M4；上传是 M7/P2 的）：新上传的 WebP 的行有了 `width`、`height`，REST 的附件答复与网页随之看得到它们；写进第 7 节的文档改动（M7/P2 的设计、`api/modules/asset.yaml` 的说明与 `make gen`）与 2.1。
- 之前上传的、没有宽高的行：不迁移，`read` 从它为内联读的字节里读（上面的 `Content`）；REST 照旧答 null。
- 测试（`asset/adapter/sniff`）：三种块各一个真实的小文件（Chromium 的 libwebp 经 `canvas.toDataURL` 写出、去掉 ICC 块，入库在 `testdata/`，100、118、140 字节，都是 7×5；这里没有 `cwebp`，实施时的偏离 9）的宽高；审查之后另有缩放位置上的 VP8、alpha 位置上的 VP8L、坏的起始码、0x2e 的 VP8L 签名、带 `VP8 ` 块的 AVI 的 RIFF、截短一个字节与截在头之后的文件、多两个字节的文件（照收）、超过 RIFF 的块、版本 1 与 4 的 VP8L、12 与 9 字节的 VP8X 块各一行（S2 审查 B-M2：原来三个文件的这些位都是 0，五个变异存活），fuzz 的种子置上缩放位、alpha 位与版本位；`VP8X` 的 24 位边（大于 16,383 的画布）；截短的头（每一个长度从 0 到 29）都答 `ok == false`、不 panic；不是 WebP 的 RIFF（`WAVE`）交给 `DecodeConfig`；不足 30 个字节的 GIF 照样答出宽高（`Peek` 的短读不当作错误）；`FuzzWebPSize`：任意的字节不 panic，`ok` 时两边都在 1–16,384（VP8、VP8L）或 1–2^24（VP8X）之内。变异：把 VP8L 的 `+1` 去掉，三种块的测试里 VP8L 那一条失败。`asset/app` 的上传测试加一个 WebP：行有宽高。page 的 `read` 的测试加一个没有宽高的 WebP 行：照样内联、答复的宽高是读出的。
- 第 11 节负责人的那一条不变：它说的正是这样的内联。

#### 3.11.3 `read` 的工具

```go
type readIn struct {
	Path   string `json:"path" jsonschema:"…"`
	Offset *int   `json:"offset,omitempty" jsonschema:"…"`
	Column *int   `json:"column,omitempty" jsonschema:"…"`
	Limit  *int   `json:"limit,omitempty" jsonschema:"…"`
}
```

`Spec{Name: "read", Title: "Read a page or an attachment", Annotations: {ReadOnly: true}, Entry: BothEntries, Notebook: true}`。

### 3.12 S2：`server.public_url`

| 处 | 内容 |
|---|---|
| `configs/config.yaml` 的 `server` 一节 | `public_url: ""`，注释："实例对外的地址，只到源（如 `https://wiki.example.com`，可以带端口与结尾的 `/`）。MCP 的 `read` 给附件的绝对下载地址时用它；为空时没有下载地址：图片照样直接给出，别的附件只给元数据。服务端不按请求的 `Host` 与 `X-Forwarded-*` 补全：反向代理配错时会签出别人的主机名"（S2 之后的写法，`config.go` 的注释同） |
| `platform/config/config.go` | `ServerConfig.PublicURL string \`koanf:"public_url"\``；`LogValue` 在 `server` 组里列出 `public_url`（不是机密） |
| `platform/config/validate.go` | 为空可以；否则 `url.Parse` 收下、`Scheme` 是 `http` 或 `https`、`Host` 不空、没有 `User`、`Path` 为空或 `/`、没有 `RawQuery`、`ForceQuery`、`Fragment`（原文里不能有 `#`），否则 `server.public_url: must be an origin such as "https://wiki.example.com", got %q`；被拒的值带 `@` 时不引用它：`got a value with an "@", which no origin has (not shown: it may hold a password or a token)`（没有源站带 `@`；S2 审查 B-M1 与修复核对第一轮 B：照录会把 `https://admin:hunter2@…`、没有 scheme 的 `admin:hunter2@…`、以令牌作用户名的 `https://ghp_…@…` 打进启动的日志，主持者的决定） |
| 组合根 | 交给 asset 的是去掉结尾的 `/` 的值（`strings.TrimSuffix`） |
| 环境变量 | `NWIKI_SERVER__PUBLIC_URL`；e2e 与 Go 的整个程序测试设 `http://wiki.example`（3.13、第 4 节） |

单元测试：每种被拒的写法（`ftp:`、带路径、带查询、带空的查询 `?`、带片段、带用户信息、没有主机）、两种收下的写法（带端口、带结尾的 `/`）、`LogValue` 列出它；带 `@` 的各行（两种 `hunter2`、令牌、`alice@…#`）的错误里出现 `hunter2` 或 `TOKEN` 就失败。MCP 的其余的量都是代码里的常量（00 号文档 4.14），不进配置（13.1 第 15 条只管配置项）。

### 3.13 S2：矩阵的两行与整个程序的测试

- **矩阵**：`list`（`pattern` 为空）与 `read`（`pageOf(c)` 的路径，`permission_matrix_page_test.go:643-649`；种下的附件没有文件，`permission_matrix_seeded_test.go:681-683`，所以 `read` 的行读页）两个入口各一格：看不到的列 404（笔记本入口在 HTTP，账户入口是工具的 `notebook.not_found`），别的列答复里有那一页（`check` 核对 `list` 列出它、`read` 的路径与版本令牌）。`TestThePermissionMatrixOverMCP` 从 S2 起真的跑：2 个工具 × 2 个入口 × 13 列，52 格；两个工具的账户入口的 schema 都要 `notebook`，runner 都给（3.6）。
- **整个程序**（`bootstrap/mcp_reads_test.go`，`server.public_url` 设 `http://wiki.example`）：
  - `list` 与 `read` 经两个入口到达 page 的用例（组合根交空的 `mcpTools` 时失败）；
  - 附件读的最后一跳（13.1 第 21 条；自己一个测试 `TestTheMCPReadsGiveAttachments`，在 `TestTheMCPReadsReachTheUseCases` 旁边，实施时的偏离 15）：上传的 PNG、JPEG、GIF、WebP 经 `read` 答 `image` 块、字节等于上传的（WebP 的行有宽高：上传读出的，3.11.2）；一个行里没有宽高的 WebP（测试在库里把这一行的宽高设回空，代替 P1 之前的上传）照样内联、答复的宽高是读出的；PDF 与 SVG 答 `resource_link`，地址以 `http://wiki.example/api/v0/assets/` 起头、带 `d=1`，把源换成测试服务的地址下载答 200、`Content-Disposition: attachment`、字节相同；`list` 列 `Guide/`，逐项对照上传的 MIME 与大小，`resource_link` 的 `mimeType` 与 `size` 也核对（S2 审查 C-M2、C-N4）；`public_url` 为空的应用（`TestTheMCPReadsWithoutAPublicURL`）：PNG 照样内联，PDF 只给元数据与 `note`；源带结尾的 `/` 的应用（`TestTheMCPReadsWithAPublicURLEndingInASlash`）签出的地址在源之下只有一个 `/`；删了文件的 PNG 答 `note`、记 warn；
  - `read` 带 NUL 的路径、`pattern` 带 NUL 答 422 `invalid_format`（`ParseGlob` 拒绝标题不能有的字符，3.10.1；直接的两个用例断言码），不是 `internal_error`（`TestToolStringsDoNotAnswerInternalErrors`：从两个入口的 `tools/list` 推导出每个字符串参数，含嵌套在数组的对象里的，各取 NUL 调用一次，答复不是 `internal_error`；各取不成对的代理项写进原始的 JSON，答 HTTP 400。反向对照：推导的函数在一个合成的 schema（只在 `ops[].path` 里有字符串）上漏掉它时失败，照 `TestFreeTextParametersDoNotAnswer5xx`，`bootstrap/contract_test.go:247-272`）；
  - 可选参数为 `null`（`limit: null`、`cursor: null`）照没有收下；
  - **码**（`TestListAndReadErrorsCarryDeclaredCodes`，S3 改名 `TestTheMCPToolsErrorsCarryTheirCodes`，3.17.4）：每个工具答出它的每个码，`bad_request`（`limit: "two"`、坏的游标、没有 `path`）、`validation_failed`（`limit: 0`、`a/./b`、`Guide/`、`offset: 0`、过了一行的 `column`）、`page.not_found`（`Missing/`、`Missing.md`）、`notebook.not_found`（账户入口里看不到的笔记本，含 bob 真的看不到的笔记本里的 `Missing/` 与 `Missing.md`）；走遍码的表的每一行。**码的表**（S2 审查 C 的提议，主持者的决定）：组合根的 `mcpToolCodes()` 在矩阵的行旁边，`list`、`read` 各是 `bad_request`、`validation_failed`、`notebook.not_found`、`page.not_found`；`TestEveryMCPToolHasItsCodes` 核对 `tools.json` 两个入口的每个工具都有一行、每行都是某个工具的、每个码都是契约声明的；`mcpAnswer.toolError` 拿整个程序读到的每个工具错误对照它的工具的那行（`internal_error` 除外）。S3 与之后的 Phase 随工具加行。不按工具对应的 REST 操作约束（S1 留给 S2 的一种做法）：`list` 答 `listNodes` 不声明的 `validation_failed`、`page.not_found`，`read` 答 `getPageContent` 不声明的 `notebook.not_found`、`validation_failed`，工具拒绝的参数是它们自己的；只按并集查又挡不住一个工具答另一个模块声明的码（`asset.not_found` 的映射就是这样漏的）；
  - `pageAttachmentErr`（`TestPageAttachmentErrors`，组合根的单元测试）：包着与否的 `asset.ErrNotFound` 给 `page.ErrNotFound`，`asset.ErrNoFile` 给 `page.ErrNoAttachmentFile`，别的不变；
  - glob 的工作的上限（这一条在 `page/app` 的用例测试里，替身的树：上限是 `ListByPattern` 的一个字段，`NewListByPattern` 自己设 `MaxGlobWork`，测试经 `export_test.go` 的 `WithMaxWork` 交小的）：200 个节点的树上列 `**/*x?y*/**`，答 `items: []` 与 `next_cursor`，拿游标接着列，几页之后列完，合起来等于不设上限的一次（不重复、不跳过）；每次的 `next_cursor` 严格大于上一次的，调用的次数有上限（每次至少前进一项，至多是节点数：测试以它为循环的上限，到了就失败）；每次调用的工作的界在 `page/domain` 的走查测试里核对（3.10.1，复核 B N-2）。

### 3.14 S3 的文件

| 文件 | 包 | 新/改 | 内容 |
|---|---|---|---|
| `platform/markdown/frontmatter.go` | markdown | 改 | `FrontmatterEnd(src []byte) int`（3.15.3） |
| `page/app/ports.go`、`page/adapter/markdown/markdown.go` | page/app、page/adapter/markdown | 改 | `Markdown` 端口加 `FrontmatterEnd(content string) int`；`AgentsStore`、`OverviewStore` |
| `page/domain/instructions.go` | page/domain | 新 | `AgentsTitle` = "AGENTS"、`MaxInstructionsBytes` = 32,768、`Agents`、`AgentsBody`、`AgentsText`（计数由调用者交，3.15.3） |
| `page/app/instructions.go` | page/app | 新 | 用例 `Instructions` 与说明的缓存（3.15.1） |
| `page/app/open_notebook.go`、`page/domain/overview.go` | page/app、page/domain | 新 | 用例 `OpenNotebook`；`MaxOverviewRoot` = 100（3.17.3） |
| `page/adapter/postgres/queries/nodes.sql`、`history.sql`、`store.go`、`overview.go` | page/adapter/postgres | 改、新 | `AgentsPage`、`NotebookCounts`、`NotebookLastChange`、`FirstChildren`（只在根下，3.15.1、3.17.3） |
| `page/adapter/mcp/instructions.go`、`open_notebook.go`、`tools.go` | page/adapter/mcp | 新、改 | 两段内置的说明（3.15.4）；工具 `open_notebook`，`UseCases` 加 `Open` |
| `page/mcp_entry.go`、`page/module.go`、`page/tools.go` | page | 改 | `MCPEntry().Instructions`；`AccountInstructions()`；说明的缓存与 `OpenNotebook` 在 `New` 里各建一个；`Tools` 带上 `open_notebook` |
| `shared/client_name.go` | shared | 新 | `MaxClientNameChars` = 128、`CleanClientName`（3.16.1） |
| `server/migrations/sql/00043_identity_api_tokens_client_name.sql`、`server/migrations/schema_test.go`、`server/sqlc.yaml` | — | 新、改 | `api_tokens.client_name`（3.16.2） |
| `identity/adapter/postgres/queries/api_tokens.sql`、`api_tokens.go`、`identity/client_names.go` | identity | 改、新 | `APITokenClientName`、`RecordAPITokenClientName`；`NewClientNames(pool)`（3.16.3） |
| `platform/httpserver/mcpserver/names.go`、`log.go`、`errors.go`、`server.go`、`tool.go`、`prompt.go` | mcpserver | 新、改 | 名称的来源、按 PAT 的缓存、记下（`names.go`）；`handshake` 填说明，方法的错误 `methodError`；`Options.Account`、`Names`；工具与 prompt 的 `Call.Client` 读 ctx（3.15.2、3.16.4） |
| `workspace/memberships.go` | workspace | 改 | `Membership` 加 `Slug`、`Name`（3.17.1） |
| `notebook/app/ports.go`、`list_account_notebooks.go`、`notebook/module.go`、`notebook/adapter/mcp/notebooks.go` | notebook | 改、新 | `AccountWorkspaces`；用例 `ListAccountNotebooks`；`Deps.AccountWorkspaces`（交空 panic）；`Tools()`；工具 `notebooks`（3.17.2） |
| `bootstrap/mcp.go`、`deps.go`、`wire.go` | bootstrap | 改 | `mcpTools` 拼上 `nb.Tools()`（page 的 `Tools` 带上 `open_notebook`）；`mcpServer(cfg, logger, api, pg, names, tools)` 多一个名称的端口（实施时的偏离 10），`wire.go` 交 `mcpNames{identity.NewClientNames(pool)}`（清理经 `shared.CleanClientName`）；`Options.Account` 是 `page.AccountInstructions()`；`notebookDeps` 交 `accountWorkspaces{workspace.NewMemberships(pool)}` |
| `api/mcp/tools.json` | — | 改 | 账户入口加 `notebooks`、`open_notebook`（3.18） |
| `bootstrap/mcp_instructions_test.go`、`mcp_names_test.go`、`mcp_account_test.go`、`notebook_visibility_test.go`、`permission_matrix_mcp_test.go`、`permission_matrix_mcp_notebooks_test.go`、`mcp_tools_test.go`、`mcp_reads_test.go` | bootstrap | 新、改 | 3.15–3.17 的整个程序的测试与矩阵的两行（notebook 的一行在自己的文件里，`mcpRows()` 拼起 page 与 notebook 的）；内置的说明讲到每个工具（3.15.4）；码的表加两行，答出码的测试改名 `TestTheMCPToolsErrorsCarryTheirCodes`（3.17.4） |
| `e2e/package.json`、`pnpm-lock.yaml`、`e2e/fixtures/mcp.ts`、`e2e/fixtures/assert/identity.ts`、`e2e/stories/mcp/mc1-connect.spec.ts` | e2e | 改、新 | `@modelcontextprotocol/sdk` 1.32.1；导出 `mcpURL`；`expectTokenClient`；MC1（第 4 节） |
| `Makefile` | — | 改 | `test-go-costs` 加概况的守卫 `TestTheOverviewReadsInTimeAsLongAsTheTree`（带 `-v`，3.17.3）；`BOOTSTRAP_FIRST` 移到 `TestEveryS` 之前（6.1） |

### 3.15 S3：`AGENTS` 与说明

#### 3.15.1 找法与缓存

```go
// AgentsStore finds a notebook's AGENTS page: its root node whose title
// key is TitleKey("AGENTS"), with its title now and its content's
// revision, unlocked.
type AgentsStore interface {
	AgentsPage(ctx context.Context, notebookID uuid.UUID, key string) (node uuid.UUID, kind domain.Kind, title string, revision int, ok bool, err error)
}
```

```sql
-- name: AgentsPage :one
-- The notebook's root node whose title key is name_key, TitleKey("AGENTS"), with its title now and its content's
-- revision when it is a page, 0 for none (M9 design 4.6; M9/P1 design 3.15.1): one point lookup by the roots' unique
-- index (00042), as RootByKey, and the content by its key. The revision is a subquery of the node found, not a join: a
-- key that many nodes share, an AGENTS page in each folder, is estimated as many rows once analyzed, and a join of
-- them read the contents of every notebook by a hash.
SELECT n.id, n.kind, n.name,
    coalesce((SELECT c.revision FROM page_contents c WHERE c.node_id = n.id AND c.deleted_at IS NULL), 0)::integer
        AS revision
FROM nodes n
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.parent_id IS NULL AND n.name_key = sqlc.arg(name_key)
    AND n.deleted_at IS NULL;
```

```go
// Instructions are a notebook's instructions for MCP (M9 design 4.6): its
// root page AGENTS's content without its frontmatter, cut at
// MaxInstructionsBytes as measure counts (mcpserver.TextBytes, which the
// module root passes); ok is false when it has none (no such node, an
// attachment holding the key, a content empty once cut), which the entry
// answers with the built-in ones. It decides nothing: the entry's Gate or
// open_notebook decided the notebook.
type Instructions struct {
	store   AgentsStore
	nodes   Nodes
	md      Markdown
	measure func(string) int
	cache   *instructionsCache // 256 entries, each a copy of at most MaxInstructionsBytes: 8 MiB at most
}

// NewInstructions returns the use case and its cache: the module builds
// one, which the notebook entry and open_notebook share.
func NewInstructions(store AgentsStore, nodes Nodes, md Markdown, measure func(string) int) *Instructions

func (i *Instructions) Execute(ctx context.Context, notebookID uuid.UUID) (a domain.Agents, ok bool, err error)

// instructionsKey is what an AGENTS text is cached by (G16).
type instructionsKey struct {
	notebook, node uuid.UUID
	revision       int
}
```

次序：`AgentsPage`（一次点查，带这一页现在的标题）→ 没有或是附件 → `ok` 为假；是页 → 键（笔记本、节点、版本号）命中就答缓存的正文 → 不命中读正文（`Nodes.PageContent`，正文与版本号一起，`contents.sql:12-16`）：读到之前被删了 → `ok` 为假；以读到的版本号（点查之后又有人写了，存的是新的版本号与它的正文）存。缓存的是 `domain.AgentsBody` 的结果：去掉 frontmatter、截到 32 KiB 之内的正文与"截过没有"，**不含**末尾的说明行，也不为它预留；说明行在每次用时以点查读到的现在的标题拼上（3.15.3）。所以只改了大小写的改名（`AGENTS` → `agents`：键、节点、版本号都不变）之后，说明行与 `instructions_from` 说的是新的写法（设计审查 B 的 Nit 1）。缓存的是正文的副本（`strings.Clone`）：`AgentsBody` 答的是 pgx 为整个正文分配的一个字符串（至多 `MaxContentBytes` = 5 MiB）的一段，Go 释放不了一个字符串的一部分，原样缓存时每项钉住整页（S3 审查 A-I1：1 MiB 的页写 256 次、每个版本读一次，缓存留着 256.1 MiB（A 的探针 258.1 MiB），5 MiB 的页约 1.25 GiB；复制之后 8.1 MiB）。所以每项至多 32 KiB，256 项至多 8 MiB。G16 的五种换法都换了键或文字相同：替换（另一个节点）、改名进出（有、没有或换了节点）、删除（没有）都换键；从回收站恢复回来的是同一个节点、同一个版本号、同样的正文，键与文字都相同；还原版本、撤销写回都是新的版本号。缓存在 `page.New` 里建一个，`MCPEntry` 与 `OpenNotebook` 共用。

**版本号是子查询**（实施时的偏离 22）：设计原来写 `LEFT JOIN page_contents`，它只在没有统计时是点查：ANALYZE 之后规划器把一个常见的键（每个文件夹里都有一个 `AGENTS`）的根估作这个键在全表里的份额（实施者的数据上 10,969 行，虽然索引唯一），以 Hash Right Join 顺序扫描 `page_contents`，读每本笔记本的正文。子查询只为找到的那个节点按主键读一次：没有统计与 ANALYZE 之后、定制与通用的计划里都是根的部分唯一索引加 `page_contents_pkey` 的 SubPlan（S3 审查 A 在自己的数据上核对：5,000 本笔记本、111,041 个节点，子查询 8 + 4 个缓冲、0.01–0.02 ms，LEFT JOIN 读 105,041 行、2,594 个缓冲）。答复相同；附件占着这个键时没有正文，版本号是 0，用例按种类答"没有"（偏离 9）。

测试：存储的 `TestTheAgentsPageIsTheRootOfItsKey`（子节点的 `AGENTS`、别的笔记本的、删掉的都不是；附件答版本号 0；标题是节点现在的写法）与计划测试 `TestTheAgentsPageIsAPointLookup`（不许整表的扫描：节点经根的唯一索引 `nodes_notebook_id_name_key_root_key`，两列都是它的条件里的等值，正文经 `page_contents_pkey`；没有统计与 ANALYZE 之后、定制与通用；数据是 20,000 个根各有子节点 `AGENTS`、一个根下的 `AGENTS`、2,000 本笔记本各一个，每页都有正文；写回 LEFT JOIN 的变异在 ANALYZE 之后失败）；用例的（替身）`TestTheInstructionsFollowTheAgentsPage`（G16 的五种换法、只改大小写的改名、附件占着键、去掉 frontmatter 之后为空）、`TestTheInstructionsCacheByTheRevisionRead`（点查与读正文之间写或删掉页：新的正文从不存在旧的键下）、`TestTheInstructionsCacheHoldsTheLastUsed`、`TestTheInstructionsCacheHoldsItsTextsAlone`（A 的方法：1 MiB 的 `AGENTS` 页写 256 次、每个版本读一次，两次回收之后量堆，至多界线的两倍 16 MiB；不复制的变异 256.1 MiB）。

#### 3.15.2 握手时填说明（mcpserver 的 `handshake`）

```go
func (s *Server) handshake(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		r := requestOf(ctx)
		ctx = withClient(ctx, s.names.of(ctx, r.cred.TokenID, toldName(req))) // 3.16.4
		res, err := next(ctx, method, req)
		init, ok := res.(*mcp.InitializeResult)
		if !ok || err != nil {
			return res, err
		}
		text, err := s.instructionsOf(ctx, r) // the notebook entry: Notebooks.Instructions; the account entry: Options.Account
		if err != nil {
			return nil, s.methodError(ctx, method, fmt.Errorf("mcpserver: the instructions: %w", err)) // a JSON-RPC internal error, classified and logged by ProblemOf
		}
		init.Instructions = text
		// one info line "MCP initialize": client (clientOf(ctx)), protocol_version, entry, the credential
		return init, nil
	}
}
```

SDK 只在 `initialize` 的结果里发说明（`sdk/mcp/server.go:2078-2107`），结果是这个请求自己的 `*InitializeResult`，改它不影响别的请求（实验：两个入口各拿到自己的说明）。读 `AGENTS` 失败时握手答 JSON-RPC 的内部错误，而不是悄悄发内置的说明：那会让 agent 以为这本笔记本没有约定。失败经新的 `methodError`：同工具的错误一样经 `ProblemOf` 分类与记日志（error 级的"MCP method failed"，期限到了是 warn），答 -32603，文字是 S1 的 `internal_error: …; request id …`；这时不写"MCP initialize"一行（实施时的偏离 5）。page 的 `MCPEntry().Instructions` 在 `ok` 为假时答笔记本入口的内置说明，有时答 `domain.AgentsText(a, fits)`，`fits` 是"`mcpserver.TextBytes` 至多 32,768"（3.15.3），所以 mcpserver 总拿到要发的文字。

测试（mcpserver 的单元测试 `handshake_test.go`，一个数调用次数的 `Notebooks` 替身；设计审查 A-M15）：`TestTheHandshakeSendsItsEntrysInstructions`（两个入口各发自己的说明）；`TestTheHandshakeReadsInstructionsOnlyForInitialize`（一次握手调 `Instructions` 恰好一次；`ping`、`tools/list`、`tools/call` 一次也不调；6.3 的变异"handshake 在 `next` 之前读说明而不看结果的类型"由它抓住）；`TestInstructionsThatFailAreTheHandshakesError`（-32603、带请求号，error 级的一行，没有"MCP initialize"）；`TestTheInstructionsAreReadWithinTheCallsDeadline`（S3 审查 A-N1：`Options.RequestTimeout` 300 ms，替身的 `Instructions` 只在 `ctx.Done()` 时结束：读以 `context.DeadlineExceeded` 结束，warn "MCP call deadline exceeded" 带 `method: initialize`；以 `context.WithoutCancel(ctx)` 读的变异失败。测试留着路由的请求体上限，只缩短调用的期限：`noBodyLimit` 时两个期限相等，请求结束先取消调用的 ctx）。

#### 3.15.3 文字

```go
// FrontmatterEnd is where src's frontmatter ends, past its closing line,
// found as the parser finds it (a byte order mark maybe before it); 0
// without one.
func FrontmatterEnd(src []byte) int
```

`platform/markdown` 导出它：今天的 `frontmatterSpan` 未导出（`platform/markdown/frontmatter.go:66-97`），`frontmatterOf` 答的第二个值就是它（`:100-108`），所以去掉的与解析读作 frontmatter 的是同一段。page 的 `app` 不导入平台，经 `Markdown` 端口（`page/app/ports.go:532-546`）调它。`FrontmatterEnd` 读解析自己的 `frontmatterSpan`（实施时的偏离 11）：`TestFrontmatterEndIsWhereTheParseReadsIt` 在每种形式（BOM、CRLF、没有闭合、只有 frontmatter）上对照 `frontmatterOf`；page 的适配器答平台的（`TestTheAdapterFindsTheFrontmattersEnd`）。

```go
// MaxInstructionsBytes bounds the initialize instructions, as
// mcpserver.TextBytes counts them, the note included (M9 design 4.6).
const MaxInstructionsBytes = 32 << 10

// Agents is an AGENTS page's text as cached: Body without the frontmatter
// and cut to MaxInstructionsBytes, Cut when it was; Title is the page's
// title now, for the note.
type Agents struct {
	Body  string
	Cut   bool
	Title string
}

// AgentsBody is the body of a content whose frontmatter ends at end: the
// content after it and after a byte order mark, without the white space
// around it; cut, when measure counts more than MaxInstructionsBytes, at
// its last line break within it, else at a character's boundary. "" when
// nothing is left.
func AgentsBody(content string, end int, measure func(string) int) (body string, cut bool)

// AgentsText is a's instructions within fits: the body when it was not cut
// and fits; else its longest prefix that ends at a line break (else at a
// character's boundary) and fits with the note, which names a.Title, and
// the note. fits must be monotone in the prefix, as both measures are
// (they add up per character), so the prefix is found by a binary search
// over the line breaks: about 15 measures of at most 32 KiB.
func AgentsText(a Agents, fits func(string) bool) string
```

末尾的说明行（英文，13.5 第 4 条）：`(The rest of <title>.md is cut here: read it whole with read, path "<title>.md".)`，前面空一行；`<title>` 是这一页现在的标题（按键找到的可以叫 `agents`）。截法：从 `content[end:]` 去掉开头的 BOM（U+FEFF 不是 `unicode.IsSpace`，`TrimSpace` 不去它），`strings.TrimSpace`；量（`mcpserver.TextBytes`，3.4.4）超过 32,768 时，在 32,768 之内找最后一个 `\n`（找不到就退到字符的边界），截下、去掉末尾的空白：这是缓存的 `Body`，不为说明行预留。用时 `AgentsText` 以这次的 `fits` 再截一次（只在截过或放不下时）：在正文的换行处二分，找最长的、加上空行与说明行仍放得下的前缀（两种量都按字符相加，前缀越长量越大，所以可以二分；约 15 次量、每次至多 32 KiB）。握手的 `fits` 是"`TextBytes` 至多 32,768"，所以说明行用真的标题、留给正文的就是 32,768 减去它与空行；`open_notebook` 的 `fits` 是"`TextBytes` 至多 32,768 而且 `MemberBytes` 至多它剩下的预算"（3.17.3）。第一轮把缓存的正文截到 32,768 减去最长的说明行，再以真的标题截一次只会更短，握手的说明就比"32,768 减去说明行"至多短约 1.5 KiB（复核 B N-7）；`AgentsText` 往回逐个换行地量，在 `open_notebook` 的测试页（32 KiB 的 `"` 与 `\n`）上每次调用要量几千个约 16K 个字符的前缀，这段文字因预算随调用而变也不缓存（复核 A N-6）。两处同一个截法、同一个量的一族（2.1 第 7 条），握手的 32 KiB 规则用的就是答复的量 `TextBytes`。测试：frontmatter 带 BOM 与 `\r\n`、没有闭合的 `---`（不算 frontmatter，照 `frontmatterSpan`）、只有 frontmatter（空了，内置的说明）、量恰好 32,768 不截、多一个字节就截、没有换行的长行、截处是多字节的字符、CRLF 的正文（`\r` 两个字节，不是草稿的六个）、全是引号与换行的正文（按 `TextBytes` 截；`open_notebook` 按 `MemberBytes` 再截）、标题只改了大小写之后说明行用新的写法、截过的说明是放得下说明行的最后一个换行处（与往回逐个换行地量的写法在随机的正文与两种 `fits` 上相同，性质测试）。连一个字符加说明行都放不下时 `AgentsText` 只答说明行（实施时的偏离 7，设计没有说）。测试名：`TestAgentsBody`、`TestAgentsText`、`TestAgentsTextIsTheLastLineBreakThatFits`（性质）；握手一层的上限处是 page 包的 `TestTheHandshakesInstructionsAreWholeUpToTheirBound`（S3 审查 A-M1：`MCPEntry` 接内存的端口与 `mcpserver.TextBytes`，正文是 `<漢 x` 的行、以 `x` 补齐，先核对量恰好是 32,768 与 32,769：前者整个给出、没有说明行，后者截短、以说明行结尾、留下的是正文的前缀；`fits` 写成 `<` 的变异失败）。最后一个换行离界线很远时截下的远少于 32 KiB（一行 7,000 字节、量 12,002 字节，之后一行 20,766 字节跨过界线：只留 12,085 字节）：照设计不改，进 M12 的打磨移交第 30 条（S3 审查 A 的存疑 1，主持者的决定 3）。

#### 3.15.4 内置的说明

只写英文（2.2）。两段都是 `page/adapter/mcp/instructions.go` 的常量（账户入口那段经 `page.AccountInstructions()` 交给组合根）。S3 的全文（P1 只有 `list`、`read`、`notebooks`、`open_notebook`；P2–P4 加工具时各自改写"Tools"一段，测试守住，见下）：

**笔记本入口**（没有 `AGENTS` 页时握手发它，`open_notebook` 也答它）：

```text
This is a notebook of Nerve Wiki, a team's wiki of Markdown pages. It has no AGENTS page, so these are the server's built-in instructions.

Paths
- Every path is from the notebook's root, its segments joined by "/". A leading "/" is optional; "./", ".." and empty segments are refused.
- A page is a Markdown file: its path is its ancestors' titles, then its own title and ".md", as in "Projects/Roadmap.md". A page holds pages and attachments, as a folder does.
- An attachment is a file under a page or at the root, as in "Projects/diagram.png". Its name never ends with ".md"; a page titled "x.png" is "x.png.md".
- Segments compare without case: "projects/roadmap.md" finds "Projects/Roadmap.md".
- Paths address nodes; they are not link targets. In a page's content, link as the wiki does: [[Projects/Roadmap]].

Tools
- list: the children of the root ("" or "/") or of a page ("Projects/"), in the tree's order. Any other pattern is a glob over the paths, in path order: * and ? within a segment, ** across segments; "**/*.md" is every page. Go on with next_cursor while it is set: a costly glob may answer a page with no items and a cursor.
- read: a page's lines exactly as stored, from offset for limit lines, about 45 KiB at most per call. Go on with next_offset, and with next_column within a line too long for one call. An attachment gives its metadata, and the image or a link to download it.
- A page's version, "<node id>:<revision>", tells which content you read.

Conventions
- A root page titled AGENTS gives this notebook's agents their instructions: its content, without its frontmatter, replaces these at the next connection.
```

**账户入口**：

```text
This is Nerve Wiki, a team's wiki of Markdown pages, for your whole account: pages live in notebooks, notebooks in workspaces.

- notebooks lists the notebooks you can see, with their ids.
- open_notebook gives a notebook's instructions for agents and an overview of it. Follow those instructions while you work in it.
- Every other tool takes the notebook's id as notebook.

Paths
- Every path is from the notebook's root, its segments joined by "/". A leading "/" is optional; "./", ".." and empty segments are refused.
- A page is a Markdown file: its path is its ancestors' titles, then its own title and ".md", as in "Projects/Roadmap.md". A page holds pages and attachments, as a folder does.
- An attachment is a file under a page or at the root, as in "Projects/diagram.png". Its name never ends with ".md"; a page titled "x.png" is "x.png.md".
- Segments compare without case: "projects/roadmap.md" finds "Projects/Roadmap.md".
- Paths address nodes; they are not link targets. In a page's content, link as the wiki does: [[Projects/Roadmap]].

Tools
- list: the children of the root ("" or "/") or of a page ("Projects/"), in the tree's order. Any other pattern is a glob over the paths, in path order: * and ? within a segment, ** across segments; "**/*.md" is every page. Go on with next_cursor while it is set: a costly glob may answer a page with no items and a cursor.
- read: a page's lines exactly as stored, from offset for limit lines, about 45 KiB at most per call. Go on with next_offset, and with next_column within a line too long for one call. An attachment gives its metadata, and the image or a link to download it.
- A page's version, "<node id>:<revision>", tells which content you read.
```

两段的"Paths"与"Tools"是同一段文字（一个常量拼进两处）。测试分两处：

- `page/adapter/mcp` 的 `TestTheBuiltInInstructionsAreShort`：每段按 `TextBytes` 至多 4 KiB、都含同一段路径与工具；只有笔记本入口那段提到 `AGENTS`。
- bootstrap 的 `TestTheBuiltInInstructionsNameEveryTool`（挨着 `TestTheToolListsAreTheContract`，设计审查 B-M1）：`api/mcp/tools.json` 每个入口的每个工具，在那个入口的内置说明里有一行以 `- <name>:` 或 `- <name> ` 起头（`(?m)^- <name>[: ]`，锚在行首）。草稿把它放在 `page/adapter/mcp`、工具名取自 page 的 `Tools()`：它看不到 notebook 的 `notebooks`，也看不到 P4 linking 的 `backlinks`、`index`；"`<name> ` 出现在段里的任何地方"在 P2 加了 `write`、`edit`、`move`、`delete` 这些普通的英文词之后会被正文配上，反向对照就不再失败。反向对照（`TestUnnamedToolsCatchesEachGap`）：去掉 `read` 那一行、把 `- list:` 改成正文里的 "list" 都失败；`unnamedTools` 不锚在行首的变异也让它失败。账户入口那段因此也要有以 `- notebooks ` 与 `- open_notebook ` 起头的两行（上面的全文有）。

### 3.16 S3：客户端名称

#### 3.16.1 `shared.CleanClientName`

```go
// MaxClientNameChars bounds a client's name, in characters (M9 design 4.2).
const MaxClientNameChars = 128

// CleanClientName is name as a changeset's client and a token's
// client_name hold it (M9 design 4.5): its bytes that are not UTF-8 and its
// characters unshowable in a name dropped, in NFC, without the white space
// around it, cut to MaxClientNameChars at a normalization boundary, and
// trimmed again; "" when nothing is left. It is idempotent: P2's
// domain.MCPClient cleans a recorded name again.
func CleanClientName(name string) string
```

挨着 `unshowable`（`shared/name.go:34-36`：Cc、Zl、Zp、Bidi_Control）。次序与理由见 2.1 第 2 条；截法：多于 128 个字符时（r 是它的字符），第 129 个字符之前是规范化的边界（`norm.NFC.PropertiesString(string(r[128])).BoundaryBefore()`：它是起始字符、不与前面的组合）就截在第 128 个之后；否则截在前 128 个字符的 `norm.NFC.LastBoundary` 处（第 129 个是组合字符，退到它所属的起始字符之前）。截下的仍是 NFC、至多 128 个字符，能留的都留下。`LastBoundary` 答的是前缀里最后一个起始字符之前的边界（那个起始字符还可能接上组合字符），除非前缀的末尾本身就是边界（最后一个字符不会与后面的组合：汉字、数字、`-`、emoji）：草稿在前 128 个字符上取它，ASCII 的名称总是多丢一个（设计审查 A-M5 实测：129 个 ASCII 变成 127 个；127 个 `a` 加 `é` 加 `x` 变成 127 个 `a`，丢了放得下的 `é`）；第一轮修订改成在前 129 个字符上取它，而第 129 个字符之后也是边界时它答整个前缀，留下 129 个（复核 A N-1 实测：128 个 `x` 加 `中`、129 个 `中`、140 个 `-`、140 个 emoji 都留下 129 个），`api_tokens_client_name_check`（`char_length BETWEEN 1 AND 128`）就拒绝每次的 `Record`，`Call.Client` 与日志里的名称也多一个字符。这条规则在审查 A 的 30 万个随机的名称与 2 万个全是起始字符的名称上核对过：幂等、NFC、至多 128 个字符、截在边界上（`review-A/server/internal/shared/cleanclient_exp5_test.go`）。不因名称拒绝调用：名称不是用户写的。测试：表格（`TestCleanClientName`：控制字符、U+200E 夹在组合之间、BOM（留下：U+FEFF 不是控制字符也不是 Bidi_Control，实施时的偏离 8）、全是空白、恰好 128 个字符、129 个 ASCII 留下 128 个、128 个 `x` 加 `中` 留下 128 个、129 个 `中` 留下 128 个、140 个 `-` 留下 128 个、127 个 `a` 加 `é` 加 `x` 留下 128 个、截在组合字符之间、不是 UTF-8 的字节）与性质测试（`TestCleanClientNameIsIdempotent`、`TestCleanClientNameKeepsAllItCan`；随机的码点：幂等、至多 128 个字符、没有 `unshowable`、`norm.NFC.IsNormalString`、首尾没有空白；另：全是起始字符、去掉首尾空白之后不少于 129 个字符的 NFC 名称恰好留下 128 个，起始字符取自字母、汉字、谚文、数字、`-`、`.` 与 emoji）。只由看不见的字符组成的名称（U+200B、U+2060、标签字符 U+E0041 U+E0042、U+00AD、U+FEFF、U+180E、U+3164、U+115F、U+2800）照样留下，日志里显示为空，P2 起是 `mcp:` 后面看不见的东西：同 shared 的名称规则（`CheckName` 对显示名同样留下），进 M12 的打磨移交第 30 条，两处一起改（S3 审查 B-N2，主持者的决定 5）。

#### 3.16.2 迁移 `00043_identity_api_tokens_client_name.sql`

```sql
-- api_tokens.client_name: the name the token's MCP client last told, as shared.CleanClientName cleans it (M9
-- design 4.2, 4.5; M9/P1 design 3.16): a call that tells no name, after a restart too, is still its client's.
-- NULL: no MCP client told one.

-- +goose Up
ALTER TABLE api_tokens ADD COLUMN client_name text
    CONSTRAINT api_tokens_client_name_check CHECK (char_length(client_name) BETWEEN 1 AND 128);

-- +goose Down
ALTER TABLE api_tokens DROP COLUMN client_name;
```

编号是 00043：S2 的修复一轮的 00042 先合并（实施时的偏离 1）。可空、没有默认值：不重写表；但 `ADD COLUMN … CHECK` 要在 ACCESS EXCLUSIVE 锁下扫一遍表核对新的约束（新列全是 NULL，核对必过），所以不是"只改目录"（设计审查 A-N13）。`api_tokens` 是小表（每个账户几个令牌），扫一遍是毫秒级，接受。`deploy/runtime-grants.sql` 不改（`api_tokens` 的 DML 已有，`:18`；13.1 第 16 条只管新的表与序列）。`server/sqlc.yaml` 的 identity 一条加上这个文件；`schema_test` 的约束表加 `api_tokens_client_name_check c`（`server/migrations/schema_test.go:137-145`），违反的情形加"空的名称"（`UPDATE api_tokens SET client_name = ''`）与"129 个字符"（`repeat('名', 129)`：按字符数，不按字节）。

#### 3.16.3 identity 的 `NewClientNames(pool)`

```go
// ClientNames remembers the client name each personal access token's MCP
// client last told (M9 design 4.5), over pool alone: bootstrap wires it to
// mcpserver, cleaning the names with shared.CleanClientName first.
func NewClientNames(pool *pgxpool.Pool) ClientNames

type ClientNames struct{ store *postgresadapter.Store }

// Recorded is the name tokenID last told; "" for none, and for no such token.
func (c ClientNames) Recorded(ctx context.Context, tokenID uuid.UUID) (string, error)

// Record writes name, cleaned and not empty, as tokenID's when it
// differs: written is false when no row was written, as when the row
// already holds name, or another transaction holds it and it is skipped
// as TouchAPIToken skips it, or the token is gone.
func (c ClientNames) Record(ctx context.Context, tokenID uuid.UUID, name string) (written bool, err error)
```

```sql
-- name: APITokenClientName :one
-- The client name a token's MCP client last told (M9/P1 design 3.16.3).
SELECT client_name FROM api_tokens WHERE id = sqlc.arg(id);

-- name: RecordAPITokenClientName :execrows
-- The client name a token's MCP client told, cleaned (M9/P1 design 3.16.3), written only when it differs: one
-- client's calls write it once. Best effort as TouchAPIToken: a row another transaction holds (a revocation) is
-- skipped rather than waited for; updated_at stays, as a use changes nothing of the token.
UPDATE api_tokens
SET client_name = sqlc.arg(client_name)
WHERE id = (
    SELECT t.id FROM api_tokens t
    WHERE t.id = sqlc.arg(id) AND t.client_name IS DISTINCT FROM sqlc.arg(client_name)
    FOR NO KEY UPDATE SKIP LOCKED
);
```

一条语句、事务之外，不进加锁的次序（同 `TouchAPIToken`，`identity/adapter/postgres/queries/api_tokens.sql:16-26`）。`:execrows`（设计审查 A-M6）：`:exec` 在 `SKIP LOCKED` 跳过一行时也答 nil，草稿就把没写下的名称当作记下了缓存起来，进程不再重试，重启之后不报名称的调用拿到旧的名称或 `unknown`，正是 00 号文档 4.5 要防的。0 行有三种原因（已是这个名称、被别的事务锁着跳过、令牌没了），mcpserver 以之后的一次点读分辨行里是不是它（下一条）。注册者的形状照 13.1 第 21 条（只凭连接池，构造函数不叫 `New`）。测试：`TestTheClientNamesRecordTheLastToldOnTheToken`、`TestTheClientNamesSkipARowInUse`。

#### 3.16.4 mcpserver 的名称：来源、缓存与记下

```go
// names is the client of each call (M9 design 4.5): the name the request
// tells; else the name its token last told this process; else, for a
// token the process has not heard, the name its row holds, read once (""
// too); else UnknownClient. It remembers 1,024 tokens (an LRU).
//
// A told name is remembered at once, so that the token's next calls are
// its, and recorded, best effort: it is known to be on the row once a
// record writes it, or once a read after a record that wrote nothing (the
// name already there, a row another transaction holds, the token gone)
// finds it there. Until then each call of the token records it again, a
// failure a warning with the token's id. A told name the row is known to
// hold is no write and no read. Without a port every call's client is
// UnknownClient.
type names struct {
	port   ClientNames
	logger *slog.Logger
	mu     sync.Mutex
	lru    *tokenLRU
}

func (n *names) of(ctx context.Context, tokenID uuid.UUID, told string) string

// namedToken is a token's client name, and whether its row is known to
// hold it: a told name is not until it is recorded.
type namedToken struct {
	token uuid.UUID
	name  string
	onRow bool
}

// toldName is the client name a request tells: an initialize's
// clientInfo.name, or any request's _meta clientInfo (ClientInfo()); ""
// for none, as a call after a stateless initialize tells none. Only the
// name, never the title (M9 design 11). An initialize reaches the server
// as a ServerRequest of InitializeParams: mcp.InitializeRequest is the
// client's.
func toldName(req mcp.Request) string
```

- **来源**（实验，附录）：`initialize` 的 `req.GetParams().(*mcp.InitializeParams).ClientInfo.Name`；任何请求的 `_meta` 里的 `io.modelcontextprotocol/clientInfo` 经 `req.(interface{ ClientInfo() *mcp.Implementation })` 读到（`sdk/mcp/shared.go:693-708`）；握手之后的无状态调用读到 nil（SDK 合成的参数没有它，`sdk/mcp/streamable.go:504-514`）。只取 `name`，不取 `title`（第 11 节）。`toldName(req)` 只看请求：initialize 由参数的类型 `*mcp.InitializeParams` 认出，不要方法名（实施时的偏离 2）；它在 `next` 之前跑，那时 `ClientInfo()` 对 initialize 还答 nil，所以先读 `GetParams()`（S3 审查 A 的 A15、C 的 C9 与 H12 去掉这一支，都被抓到）。
- **清理**：`port.Clean(told)`（组合根接 `shared.CleanClientName`），空了当作没有报。
- **缓存与记下**（S3 审查 B-M1、B-M2、C-M2 之后照 B 的方向，主持者的决定 2）：报了名称（清理之后不空）→ 缓存里已是它、而且已知在行里 → 不写也不读（热行的规则：同一个客户端的调用不碰这一行）；否则立刻记住为 `{name, onRow: false}`（这次与之后的调用都是它），再 `Record`（只在不同时写）：写了一行 → 标 `onRow`；0 行（已是这个名称、被别的事务持着而跳过、令牌没了）→ 一次点读 `Recorded`，行里就是它时标 `onRow`；记下或读失败 → warn（"MCP client name not recorded" / "MCP client name not read"，带 `token_id`、`request_id` 与错误；读的那条是实施时的偏离 6），不标。标没设时这个令牌的每次调用（报不报名称）都再记下：每次至多一次记下与一次读，没有循环；行一直被持着时 1,000 个同时的调用是 1,001 次记下与 1,001 次读，行空出来之后一次写、之后什么也没有（修复核对第一轮 B）。没报名称 → 缓存命中就用（说出的或读到的）；不命中就 `Recorded`：读到了（含空的）就记住为在行里，除非读的期间有说出的名称先记住了（`learn`：读回来的不盖过说出的）；读失败记 warn、这次当作没有、**不记住**（否则这个令牌在进程的余生里都是 `unknown`）；空的是 `UnknownClient`。晚到的记下只标仍是它自己的名称（`tokenLRU.onRow` 比较名称），从不标其间说出的别的名称。记住的名称是一份副本（`strings.Clone`：清理答的是请求里那个字符串的一段，不复制时每项可以钉住整个请求体；S3 审查 A-I1 之后的审计）；读回来的是 pgx 自己的、至多 128 个字符的字符串，不复制。重启之后说出行里已有的名称是一次 0 行的记下与一次读、只一次，之后没有语句（C-M2）。第一轮的写法（写了一行才缓存，0 行或失败都不缓存，接受重启之后每次报名称的调用一次空的写）在跳过或失败之后让这个会话不报名称的调用读到行里的旧名称并缓存（B-M1 的情形 1），失败的记下被缓存也没有测试抓得到（B-M2）。
- **两个客户端用一个 PAT**：进程里是最后报给它的名称，行里是最后写下的。测过的次序（A 记住并写、B 记住并写、A 的答复晚到）两边都停在最后报的 B，B 的记下被跳过的变体也是（`TestTheNamesOfTwoClientsAtOnce`）；镜像的次序（A 记住而它的语句被推迟，等连接池或调度，B 随后记住、写下、标在行里，A 的语句之后才跑，写下 A）里进程是 `{B, onRow: true}`、行是 A：B 再报不跑语句，修不回来，直到报出缓存之外的名称（例如 A 再报）、这一项被挤出或进程重启（重启之后这个令牌不报名称的调用是 A 的；修复核对第一轮 B 的 N-1）。要两个客户端在同一刻报不同的名称、一个请求在记住与语句之间被推迟；两个名称都是"最后报的"之一，不加按令牌的锁（主持者的决定 8；00 号文档 4.5、第 10 节：接入文档建议一个 agent 一个 PAT）。
- **在哪里记**：`handshake` 中间件，在每个方法之前（3.15.2），用调用的 ctx：握手、带 `_meta` 的调用都记；名称放进 ctx（`withClient`、`clientOf(ctx)`），工具与 prompt 的包装读它写进 `Call.Client`，调用与握手的日志行也读它。S1 的 `(*Server).clientOf(req)` 删掉：它那一支 `*mcp.InitializeRequest` 是客户端的类型、到不了，但在 `next` 之后它的 `ClientInfo()` 一支会从会话读到名称；S1 的握手一行写 `unknown` 是因为 S1 没有名称（`Options.Names` 为 nil，每个客户端都是 `UnknownClient`；实施时的偏离 3，原因由 S3 审查 A 更正）。P1 只记在 PAT 上、写进日志；`mcp:<名>` 进 `changesets.client` 是 P2 的（`domain.MCPClient`，最后一跳也在 P2）。
- **测试**（mcpserver 单元 `names_test.go`，替身的端口，数每个方法被调的次数；设计审查 A-M6、A-M15，S3 审查 B-M1、B-M2、C-M2、B-N1）：`TestTheNamesTakeTheirSources`（三种来源的优先级；握手一行的 `client`）；`TestTheNamesRecordOnlyWhatDiffers`（不同时才写；已知在行里的同一个名称不写也不读）；`TestTheNamesRememberAnEmptyName`（从没报过名称的令牌两次不报名称的调用只读一次端口）；`TestTheNamesKeepATellingUnrecorded`（B 的情形 1，跳过与失败都是：行里是 `old-client`、握手报 `new-client`，之后不报名称与报它的调用都答 `new-client`、都再记下，行仍是 `old-client`；能写之后下一次调用写，之后的调用不写也不读）；`TestTheNamesFindATellingOnTheRow`（C-M2：说出行里已有的名称是一次 0 行的记下与一次读，之后不报名称的调用答它、没有语句）；`TestTheNamesRetryAFailedRead`（读失败之后下一次又读；失败的记下下一次重试，行随之是它）；`TestTheNamesOfTwoClientsAtOnce`（上一条的交错与它的变体）；`TestTheNamesKeepATellingOverAReadBeforeIt`（读的期间说出的名称不被读回来的盖过）；`TestTheNamesForgetTheLeastRecentlyUsed`（第 1,025 个令牌之前再用令牌 0：令牌 1 出去、0 留下；FIFO 的变异失败，B-N1）；`TestTheNamesHoldTheirOwnBytes`（说出 `agent-a` 加 1 MiB 的空白：记住的名称不与请求共用内存，以 `unsafe.StringData` 核对，同 linking 的 `sharesMemory`）；清理之后为空的名称不写。整个程序（`bootstrap/mcp_names_test.go` 的 `TestTheClientNameIsTheTokens`）：没报过名称时是 `unknown`；带 `"  Claude Code\t"` 握手之后 `api_tokens.client_name` 是 `Claude Code`，同一个应用里不带名称的调用（测试工具 `echo` 答 `Call.Client`）是它；同一个库上的第二个应用、不握手的调用仍是它（读库）；第三个应用 `restarted` 的客户端说出已记下的 `Claude Code`（记下 0 行），之后不报名称的调用答它（C-M2；0 行之后记住 `""` 的变异经 serve 失败）；一个调用的 `_meta` 带 `Codex` 时改成它；U+202E 加 200 个 `n` 记下的是 128 个 `n`；组合根交空的名称端口时这些测试失败。

### 3.17 S3：`notebooks` 与 `open_notebook`

#### 3.17.1 workspace 与 notebook 的端口

```go
// Membership is a workspace an account is an active member of, its role
// there, and the workspace's slug and name.
type Membership struct {
	WorkspaceID uuid.UUID
	Slug, Name  string
	Role        shared.WorkspaceRole
}
```

`memberships.WorkspacesOf` 照抄 `ListWorkspacesOf` 已经读出的 `l.Workspace.Slug`、`l.Workspace.Name`（`workspace/memberships.go:49-59`），语句不变（2.1 第 1 条）；`TestMembershipsListTheWorkspacesOfAnAccount` 核对两个新字段。

```go
// AccountWorkspaces are the workspaces an account is an active member of,
// guests included, with its role there and their slugs and names:
// bootstrap hands it workspace's Memberships (M9 design 4.1).
type AccountWorkspaces interface {
	WorkspacesOf(ctx context.Context, userID uuid.UUID) ([]AccountWorkspace, error)
}

type AccountWorkspace struct {
	ID         uuid.UUID
	Slug, Name string
	Role       shared.WorkspaceRole
}
```

`notebook.Deps.AccountWorkspaces`；`notebook.New` 交空时 panic（照 `page.New`，`page/module.go:170-179`），模块根的测试 `TestTheModuleRefusesNoAccountWorkspaces`。组合根的 `accountWorkspaces{workspace.NewMemberships(pool)}`（`bootstrap/mcp.go`）把 `Membership` 换成 `AccountWorkspace`：两个模块互不导入。

#### 3.17.2 `ListAccountNotebooks` 与 `notebooks`

```go
// ListAccountNotebooks lists the notebooks the caller sees in each
// workspace it is a member of (M9 design 4.6): by the rule ListNotebooks
// filters by, unlocked.
type ListAccountNotebooks struct {
	workspaces AccountWorkspaces
	notebooks  NotebookLister
}

func (l *ListAccountNotebooks) Execute(ctx context.Context) (AccountNotebooks, error)

// AccountNotebooks are the caller's notebooks, at most MaxAccountNotebooks.
type AccountNotebooks struct {
	Notebooks []AccountNotebook
	Truncated bool
}

type AccountNotebook struct {
	Notebook  domain.Notebook
	Role      shared.NotebookRole
	Workspace AccountWorkspace
}

const MaxAccountNotebooks = 1000
```

次序：`RequireActor` → `WorkspacesOf(actor.UserID)`，按 slug 排 → 每个工作区 `ListNotebooks(ctx, ws.ID, actor.UserID, shared.ReachedByAccess(ws.Role))`、角色 `shared.EffectiveNotebookRole(n.Explicit, n.Notebook.Access, ws.Role)`（同 `notebook/app/list_notebooks.go:36-48`；同事件流按工作区的组合，`events/app/open_stream.go:51-58`）→ 到 1,000 项就停、`Truncated`（不再读余下的工作区）。一个工作区里照 `ListNotebooks` 的次序（名称的小写、名称、id）。测试：`TestListAccountNotebooksListsEachWorkspaces`、`TestListAccountNotebooksStopsAtItsBound`（含恰好 1,000 项）。

**`MaxAccountNotebooks` 留着**（设计审查 B 的过度设计第 4 条与 Nit 14）：经工具时答复的上限总先到（一项按两次转义计至少约 150 字节，48 KiB 约在 330 项处停），而 `notebooks` 不分页（00 号文档 4.6：笔记本是小的集合，没有游标），所以简报的"答复的上限总先到、而且 `notebooks` 按游标分页时去掉它"的两个条件只满足一个。它管的是用例自己的工作：没有它，一个在很多工作区里、看得到很多笔记本的账户，用例要把它们全读出、拿在内存里，适配器再丢掉 330 项之后的；有它，用例读到 1,000 项就停。所以"1,000 项的截断"是用例一层的测试（替身的端口给 1,001 项），不经工具（6.2）。不经规则表：只涉及调用者自己（13.1 第 3 条的第一类，00 号文档 4.6）；角色与可见性照 `ListNotebooks` 的同一条规则，所以"列表等于逐项判定读取"成立，由集成测试守住。逐个工作区一条语句：一个账户的工作区很少，M12 的性能移交已记（00 号文档"写给后面的 M"）。

工具 `notebooks`（`notebook/adapter/mcp/notebooks.go`）：`Spec{Name: "notebooks", Title: "List your notebooks", Annotations: {ReadOnly: true}, Entry: AccountEntry}`，输入是空的结构。答复：

```json
{"notebooks": [{"id": "0192…", "name": "Handbook", "workspace": {"slug": "acme", "name": "Acme"}, "role": "editor"}],
 "truncated": false}
```

经 `mcpserver.Fit`（3.4.4）：`build(k)` 是前 k 项，k 小于项数或用例的 `Truncated` 为真时 `truncated` 为真；量是整个答复的 `TextBytes`，适配器不自己数（设计审查 B-I1）。一项的笔记本名称至多 255 字节，工作区的名称是 `varchar(80)` 个字符、至多 320 字节（`00006_workspace_workspaces.sql:12`；B 的 Nit 3），通常一项约 150–300 字节；S3 审查 B 量出最贵的一项约 2.2 KiB（笔记本的名称是 255 个 `<`、工作区的 80 个），所以至少放得下 20 项、通常两三百项。没有游标：笔记本是小的集合（6.1）。测试：`TestTheNotebooksAnswerTheirItems`、`TestTheNotebooksFitTheirAnswer` 与模块根的 `TestTheModulesToolIsNotebooks`。

#### 3.17.3 `OpenNotebook` 与 `open_notebook`

```go
// OpenNotebook opens a notebook for MCP's open_notebook (M9 design 4.6):
// node.list, then its instructions and an overview, unlocked.
type OpenNotebook struct {
	gate         *NotebookGate // the notebook entry's: one decision for both (node.list)
	overview     OverviewStore
	instructions *Instructions // the entry's, with their one cache
}

func NewOpenNotebook(gate *NotebookGate, overview OverviewStore, instructions *Instructions) *OpenNotebook

func (o *OpenNotebook) Execute(ctx context.Context, notebookID uuid.UUID) (Opened, error)

type Opened struct {
	Agents      domain.Agents // ok false: the built-in ones, which the adapter writes
	HasAgents   bool
	Pages       int64
	Attachments int64
	LastChange  *time.Time // nil for a notebook without a changeset
	Root        []RootNode // at most MaxOverviewRoot, in the tree's order
	MoreRoot    bool
}

type RootNode struct {
	Path string
	Kind domain.Kind
}

// OverviewStore reads a notebook's overview, unlocked.
type OverviewStore interface {
	NotebookCounts(ctx context.Context, notebookID uuid.UUID) (pages, attachments int64, err error)
	// NotebookLastChange is the tree's last write: its changesets' latest
	// updated_at, the pages' part of the web's activity; nil for a
	// notebook without a changeset.
	NotebookLastChange(ctx context.Context, notebookID uuid.UUID) (*time.Time, error)
	// FirstChildren are the root's first n children, in order.
	FirstChildren(ctx context.Context, notebookID uuid.UUID, n int) ([]domain.Node, error)
}
```

```sql
-- name: NotebookCounts :one
-- A notebook's pages and attachments not deleted (M9/P1 design 3.17.3): open_notebook's overview, read from the
-- notebook's nodes by an index of them.
SELECT
    count(*) FILTER (WHERE n.kind = 'page')::bigint AS pages,
    count(*) FILTER (WHERE n.kind = 'asset')::bigint AS attachments
FROM nodes n
WHERE n.notebook_id = sqlc.arg(notebook_id) AND n.deleted_at IS NULL;

-- name: NotebookLastChange :one
-- When a notebook's tree was last written, the pages' part of its activity on the web (activity.sql): the latest
-- updated_at of its changesets, which an edit session's later writes move on (M9/P1 design 3.17.3). GROUP BY answers
-- no row for a notebook without a changeset, which the store maps to nil, so the column is never NULL.
SELECT max(c.updated_at)::timestamptz AS last_change
FROM changesets c
WHERE c.notebook_id = sqlc.arg(notebook_id) AND c.deleted_at IS NULL
GROUP BY c.notebook_id;

-- name: FirstChildren :many
-- The root's first n children not deleted, in order: open_notebook's root, n = MaxOverviewRoot + 1 to tell whether
-- more follow (M9/P1 design 3.17.3). parent_id IS NULL is an index condition; IS NOT DISTINCT FROM is not.
SELECT id, notebook_id, parent_id, kind, name, name_key, sort_order, created_by_id, updated_by_id, created_at,
    updated_at
FROM nodes
WHERE notebook_id = sqlc.arg(notebook_id) AND parent_id IS NULL AND deleted_at IS NULL
ORDER BY sort_order, id
LIMIT sqlc.arg(n);
```

**最后的变更是单独的一条语句**（设计审查 B-I2）：草稿把它写成计数语句里的一个标量子查询，sqlc 照 `NOT NULL` 的列把它生成为 `time.Time`（B 在 `review-B/server/` 上实测，加 `::timestamptz` 也一样），没有变更集的笔记本（新建的、空的，`activity.sql:2-4` 说"没有变更集的笔记本从没有过页"）扫描 NULL 失败，`open_notebook` 答 `internal_error`，而这正是 agent 在一本新笔记本里做的第一件事。分开之后，`GROUP BY` 让没有变更集时答零行，存储把 `pgx.ErrNoRows` 换成 nil，列本身从不为 NULL。时刻取网页的活动里页的那一部分（B-M12，复核 B N-4：网页的活动另从笔记本的行自己的 `updated_at` 起，2.1 第 17 条）：变更集的 `max(updated_at)`，编辑会话之后的写会把它往后挪；草稿取的是最后一个变更集**开始**的时刻（`created_at`），长的编辑会话里或之后，`open_notebook` 会把会话的开始说成"最后改动"。代价同网页的活动：读这本笔记本的全部变更集（`notebook_id` 的前缀，`updated_at` 不在索引里），随写增长，交 M12（第 8 节）。

**`FirstChildren` 只在根下**（设计审查 A-M16、B-M3 与 B 的过度设计第 3 条）：它只以根调用，写 `parent_id IS NULL`：ANALYZE 之后是 `nodes_notebook_id_parent_id_idx`（`00012_page_nodes.sql:36`）上的索引条件；没有统计时规划器读 00042 的根的部分唯一索引 `nodes_notebook_id_name_key_root_key`（`WHERE parent_id IS NULL AND deleted_at IS NULL`），`parent_id IS NULL` 是它的谓词（实施时的偏离 15）；草稿的 `IS NOT DISTINCT FROM sqlc.narg(parent_id)` 不能做索引条件，计划要读这本笔记本的全部节点再筛（`nodes.sql:47-50` 的 `AssetsUnder` 的注释说的就是它），与 2.1 第 13 条自己的理由相悖。计划测试（13.4 第 9 条，`TestTheOverviewReadsUseTheirIndexes`，没有统计与 ANALYZE 之后，不许整表的扫描）：`FirstChildren` 只读根下的，`parent_id IS NULL` 在索引条件里或在所读的索引的谓词里（`IS NOT DISTINCT FROM` 两样都不是，变异失败）；`NotebookCounts` 只读这本笔记本的节点、`NotebookLastChange` 只读它的变更集，各经任何以 `notebook_id` 起头的索引（从 `pg_index` 读，不列名字：去掉列出的节点的索引的变异让规划器取约束的 `nodes_notebook_id_id_key`，同样只读这本笔记本；原来列的 `changesets_notebook_id_idx` 在 00037 之后已没有；实施时的偏离 16），断言计划里没有 `Seq Scan`。10 万个节点、10 万个变更集的笔记本上两条的耗时进 `test-go-costs` 的守卫（`TestTheOverviewReadsInTimeAsLongAsTheTree`，带 `-v`：不超过读整棵树的时间；autovacuum 关掉、在一个连接上，ANALYZE 前后各取 8 次的中位数与 `ListNodes` 的中位数比，一次回收或计划的切换定不了它，实施时的偏离 17）：本机树 57.9 与 55.5 ms、计数 6.3 与 5.6 ms、最后改动 5.7 与 4.3 ms，各次的门禁里树 51–100 ms、计数 6–10 ms、最后改动 4–8 ms，约低 9 倍。`FirstChildren` 读根下的全部节点再取前 101 个（`sort_order` 不在索引里）：一个根下 10 万个节点时 10–18 ms，对读树的 66 ms（S3 审查 B），不在守卫里，交 M12（第 8 节）。存储测试 `TestTheOverviewReadsANotebook`：空的笔记本（0、0、nil、没有节点）；计数不含删掉的与别的笔记本的；最后改动是没删的变更集最新的 `updated_at`；根下的前几个节点按次序、不含根之下的。

**码的次序**：笔记本入口的 `NotebookGate`（`RequireActor`、零值的 id 不查库、`WorkspaceOf`、判定 `node.list`；看不到的、删掉的、没有的都是 `notebook.not_found`，在读它的任何东西之前；用同一个判定而不另抄一份，实施时的偏离 4）→ 计数 → 最后的变更 → 根下的节点（`FirstChildren(101)`，留 100 个与 `MoreRoot`，路径经 `domain.PathOf`）→ 说明（`Instructions`；没有时是笔记本入口的内置说明）。`TestOpenNotebookAnswersInOrder` 断言调用的次序、看不到的笔记本在判定之后什么也不读；`TestOpenNotebookOfTheRootsFirstNodes`。

工具 `open_notebook`（`page/adapter/mcp/open_notebook.go`）：`Spec{Name: "open_notebook", Title: "Open a notebook", Annotations: {ReadOnly: true}, Entry: AccountEntry, Notebook: true}`，输入只有组合根加的 `notebook`。答复：

```json
{"notebook": "0192…",
 "instructions": "…",
 "instructions_from": "AGENTS.md",
 "overview": {"pages": 120, "attachments": 8, "last_changed_at": "2026-10-17T08:30:00Z",
              "root": [{"path": "Inbox.md", "kind": "page"}, {"path": "logo.png", "kind": "attachment"}]},
 "truncated": false}
```

`instructions_from` 是 `AGENTS` 页现在的路径（它现在的标题加 `.md`），内置的说明时为 null；`last_changed_at` 没有变更集时为 null；`truncated` 为真时根下还有没列出的节点（多于 100 个，或放不下）。名称与角色在 `notebooks` 里。

**按剩下的预算填**（设计审查 B-I1）：草稿给说明 32 KiB、概况 16 KiB（`MaxOverviewBytes`），说"合计在 48 KiB 之内（加上键名与 id 约 200 字节）"：32,768 + 16,384 已是 49,152 的全部，键名在其上；而结构的工具的文字块就是 JSON 文字（3.3.4），说明里的引号与换行在答复的量里转义两次，一个按握手的量截到 32 KiB 的说明在这里可以占到约 64 KiB。mcpserver 在答复之前再量一次，超过就答 `internal_error`：草稿的 `open_notebook` 会对长的 `AGENTS` 页失败。改为按剩下的预算填，说明在先：

1. **骨架**：说明为空、根下没有节点的答复，按 `TextBytes` 量出 S（通常 237 字节；`instructions_from` 是 255 字节、全是 `&` 的标题时 1,761 字节，复核 A 实测）。
2. **说明**：留给根下的节点 R = 8,192 字节（`openNotebookRootReserve`：放得下 100 个短名称的节点，名称都是 255 字节时 28 个），说明的预算是 49,152 − S − R（S 为 237 时 40,723 字节，约 39.8 KiB），按 `MemberBytes` 计（说明是答复里的一个字符串成员，3.4.4）：`domain.AgentsText(a, fits)`，`fits` 是"`TextBytes` 至多 32,768 而且 `MemberBytes` 至多这个预算"。所以它从不长于同一时刻握手的说明，只在 `AGENTS` 页的引号、换行、反斜杠多到两次转义放不下时更短，那时带说明行（`read` 读全文）。内置的说明约 2–3 KiB，不截。
3. **根下的节点**：`mcpserver.Fit(len(Root), build)`，`build(k)` 是带前 k 个节点的整个答复（k 小于节点数或 `MoreRoot` 时 `truncated` 为真），用说明之外剩下的全部；说明短时根下的节点可以用到 48 KiB 的几乎全部。

所以答复按构造在 48 KiB 之内，到不了 mcpserver 的"超过上限"那一支。测试（`page/adapter/mcp`，替身的用例；设计审查 B 的用例）：一个 32 KiB（按握手的量）的 `AGENTS` 页、内容全是 `"` 与 `\n`，加根下 100 个 255 字节名称的节点：答复的 `TextBytes` 至多 49,152、不是 `internal_error`；说明以说明行结尾、`MemberBytes` 至多它的预算；根下的节点至少放进 R 能放的个数、`truncated` 为真。另：没有 `AGENTS` 时是内置的说明、根下 100 个短名称的节点全列出、`truncated` 为假；101 个时 `truncated` 为真；标题是 255 字节的 `&` 时骨架变大、说明随之变短而仍在上限之内。测试名：`TestTheOpenNotebookAnswer`（确切的 JSON；空的情形）与 `TestOpenNotebookFitsItsAnswer`。后者的预算用字面的 8,192（`49,152 − TextBytes(骨架) − 8192`，不用常量本身），并断言截得最长：留下的是正文在换行之前的前缀，多一行加说明行就超过预算或 `TextBytes` 的 32,768；两种标题都核对（S3 审查 C-M1：原来以同一个常量算预算，12 KiB 的余地也过得了每个测试）。余地改为 12 KiB、16 KiB 或 8,192 ± 8（这个测试的一行是 7 个成员字节）都失败：两边都钉到一行；经 serve 的一步在 3.17.4。S3 审查 B 另以 3,000 个随机的正文与标题核对：没有错误，每个答复在 48 KiB 之内，说明总是握手的说明的前缀（带说明行）。

#### 3.17.4 S3 的矩阵与整个程序的测试

- **矩阵**（`permission_matrix_mcp_notebooks_test.go`；`mcpRows()` 拼起 page 与 notebook 的行）：`notebooks` 一行没有目标，每列都答成功，`check` 比较列出的笔记本与 `accountNotebooksOf(c)`：lab 的照 REST 的列表（`listedNotebooks`），在 lab 之外的那一列另有 `other` 的 `other-nb`，按工作区再按名称，各带工作区的 slug 与名称，没有截断；`open_notebook` 一行：看不到的列答工具的 `notebook.not_found`，看得到的 `check` 核对笔记本、内置的说明（`instructions` 解出来与 `pagemcp.NotebookInstructions` 比较，`instructions_from` 为 null；S3 审查 C-N1：原来只看 `instructions_from`）、种下的页与附件的数目、最后改动不为 null、根下的路径照树的次序。矩阵的笔记本都种了变更集，测不到空的笔记本（设计审查 B-I2），所以空的笔记本另有三处：整个程序的 `TestOpenNotebookOfAnEmptyNotebook`（`bootstrap/mcp_account_test.go`：经 REST 新建一本笔记本、什么也不写，`open_notebook` 答 `pages: 0`、`attachments: 0`、`last_changed_at: null`、`root: []`、内置的说明，不是 `internal_error`）；`page/adapter/postgres` 的存储测试（空的笔记本 `NotebookLastChange` 答 nil）；MC1 的一步（4.3）。
- **"列表等于逐项判定读取"**（13.1 第 3 条，`bootstrap/notebook_visibility_test.go` 的 `TestTheAccountsNotebooksAreWhatEachReadAllows`）：矩阵的数据上每一列（工作区与笔记本的列）各建一个 PAT，MCP 的 `notebooks` 列出的集合（带角色）等于它逐本 `GET /api/v0/notebooks/{id}` 答 200 的集合，角色等于那个答复的 `role`；在自己的副本上让两列跨两个工作区（lab 的笔记本编辑者另是 `other` 的成员、`other-nb` 的读者，lab 之外的那一列另是 lab 的成员），各自断言，不另加账户（实施时的偏离 19）。反向对照 `TestAccountNotebooksDiffReportsEachDifference`（多列一本、少列一本、角色不同）。
- **G16 的五种换法**（`bootstrap/mcp_instructions_test.go` 的 `TestTheInstructionsFollowTheAGENTSPage`，设计原来叫 `TestTheInstructionsFollowAgents`；经 serve 的 REST 改、经 MCP 握手读）：替换（把 `AGENTS` 改名走、另建一个 `AGENTS`）、改名进出（`Notes` 改名为 `agents`，再改回）、删除、从回收站恢复（与删除之前的文字相同）、还原版本与撤销（新的版本号、那时的文字）；另有写正文之后的下一次握手、根下叫 `AGENTS` 的附件（内置的说明）、子页里的 `AGENTS`（内置的说明）、frontmatter 去掉、32 KiB 处截断带说明行、去掉 frontmatter 之后为空（内置的说明）。`open_notebook` 的 `instructions` 与同一时刻的握手相同。修复一轮加一步（S3 审查 C-M1）：20,000 个 `"` 与换行的页，`open_notebook` 的说明是留出字面的 8,192 之后放得下的最多的行，骨架从答复量（`skeletonOf`）；12 KiB 的余地经 serve 失败（留下 36,621 个成员字节，多一行 36,628，预算 40,718）。
- **最后一跳**（13.1 第 21 条）：`notebooks` 的工作区名称与 slug 经组合根的 `AccountWorkspaces` 到达（`bootstrap/mcp_account_test.go` 的 `TestNotebooksListsTheAccountsWorkspaces`：名叫 `Zeta Labs`、slug 是 `zeta` 的工作区，删掉的笔记本与 bob 的不列；组合根把名称换成 slug 的变异失败；交空时 `notebook.New` panic，整个程序的测试随之失败）；客户端名称（3.16.4 的 `TestTheClientNameIsTheTokens`）；内置的说明讲到每个工具（3.15.4 的 `TestTheBuiltInInstructionsNameEveryTool`）。
- **码**（`TestTheMCPToolsErrorsCarryTheirCodes`，S2 的 `TestListAndReadErrorsCarryDeclaredCodes` 改名：S2 修复二轮起它的循环走遍码的表的每一行，S3 的工具的调用也在它的表里；实施时的偏离 23）：`notebooks` 带它不收的参数答 `bad_request`；`open_notebook` 没有 `notebook`、多一个参数答 `bad_request`，看不到的、删掉的、没有的与不是 UUID 的笔记本答 `notebook.not_found`。码的表（3.13 的 `mcpToolCodes()`）加两行：`notebooks: {bad_request}`、`open_notebook: {bad_request, notebook.not_found}`（不是 UUID 的 id 是零值，所以是 `notebook.not_found`）；矩阵的 `open_notebook` 各格也经 `mcpAnswer.toolError` 对照它。
- **交错**（00 号文档 4.15"握手时 `AGENTS` 页被替换"）：读不加锁，数据库挡不住两次读之间的写，所以确定性的测试在 `page/app`（替身的存储在点查与读正文之间换掉页、写正文）：缓存以读到的版本号存，下一次点查是新的版本号时读新的正文，从不把新的正文存在旧的键下；整个程序一侧由上面的"下一次握手是新的"守住。

### 3.18 `api/mcp/tools.json` 的全文（P1 结束时）

先写的契约，审查读它；S2 写进 `list`、`read`（两个入口），S3 写进 `notebooks`、`open_notebook`。两个入口的 `list`、`read` 除 `notebook` 之外相同（说明里不提入口）。成员的次序不要紧（解析之后比较），文件照 oxfmt 排版。

```json
{
  "notebook": [
    {
      "name": "list",
      "title": "List nodes",
      "description": "List the notebook's pages and attachments. With pattern empty or \"/\", the root's children; with \"A/B/\", the children of the page A/B.md; both in the tree's order. Any other pattern is a glob over the paths, in path order: * matches within a segment, ? one character, ** any number of segments as a segment of its own (\"**/*.md\" is every page, \"A/**\" everything under A). Matching ignores case. Each item has its path, its kind (page or attachment) and whether it has children; a page also its version, size and last change, an attachment its type and size. An answer holds at most limit items and about 48 KiB, and a costly glob over a large notebook stops after a bounded amount of matching, maybe with no items: whenever next_cursor is set, pass it back as cursor to go on. A path without wildcards that names nothing may come with a hint, as when it names a page without its .md.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "pattern": {
            "type": ["null", "string"],
            "description": "\"\" or \"/\" for the root's children, \"A/B/\" for the children of A/B.md, or a glob. Default: the root's children."
          },
          "limit": { "type": ["null", "integer"], "description": "At most this many items, 1 to 1000. Default 200." },
          "cursor": { "type": ["null", "string"], "description": "next_cursor of the previous answer, to go on from there." }
        },
        "additionalProperties": false
      },
      "annotations": { "idempotentHint": false, "openWorldHint": false, "readOnlyHint": true }
    },
    {
      "name": "read",
      "title": "Read a page or an attachment",
      "description": "Read a page or an attachment by its path from the notebook's root; a page's path ends with .md. A page comes as two text blocks: a line of JSON metadata (path, kind, version, offset, column, lines, total_lines, byte_size, next_offset, next_column), then its lines exactly as stored, frontmatter, byte order mark and CRLF included. offset is the first line, from 1; limit the number of lines, 2000 by default. An answer holds about 48 KiB at most, its metadata line included: go on from next_offset, and from next_column when a line was too long for one answer; the parts read in turn are the whole content. An attachment comes as its metadata (type, size, SHA-256, width and height, how to link it, a download address valid for an hour or two), then the image itself for a PNG, JPEG, GIF or WebP of at most 3 MiB and 8000 pixels a side, or else a link to download it. Without the server's public address set (server.public_url) there is no download address: an image still comes itself, any other attachment as its metadata only, with a note saying so.",
      "inputSchema": {
        "type": "object",
        "properties": {
          "path": {
            "type": "string",
            "description": "The path from the notebook's root: \"A/B.md\" for a page, \"A/diagram.png\" for an attachment."
          },
          "offset": { "type": ["null", "integer"], "description": "The first line, from 1. Default 1." },
          "column": {
            "type": ["null", "integer"],
            "description": "The byte of the first line to start from, from 0: next_column of the previous answer. Default 0."
          },
          "limit": { "type": ["null", "integer"], "description": "At most this many lines, from 1. Default 2000." }
        },
        "required": ["path"],
        "additionalProperties": false
      },
      "annotations": { "idempotentHint": false, "openWorldHint": false, "readOnlyHint": true }
    }
  ],
  "account": [
    {
      "name": "list",
      "title": "List nodes",
      "description": "<the notebook entry's list description, word for word>",
      "inputSchema": {
        "type": "object",
        "properties": {
          "notebook": { "type": "string", "description": "The notebook's id, from notebooks." },
          "pattern": "<as in the notebook entry>",
          "limit": "<as in the notebook entry>",
          "cursor": "<as in the notebook entry>"
        },
        "required": ["notebook"],
        "additionalProperties": false
      },
      "annotations": { "idempotentHint": false, "openWorldHint": false, "readOnlyHint": true }
    },
    {
      "name": "notebooks",
      "title": "List your notebooks",
      "description": "List the notebooks you can see in every workspace you belong to, each with its id, its name, its workspace's slug and name, and your role (admin, editor or reader), by workspace, then by name. Open one with open_notebook, then pass its id as notebook to the other tools. truncated is true when there were more than one answer holds.",
      "inputSchema": { "type": "object", "additionalProperties": false },
      "annotations": { "idempotentHint": false, "openWorldHint": false, "readOnlyHint": true }
    },
    {
      "name": "open_notebook",
      "title": "Open a notebook",
      "description": "Open a notebook: its instructions for agents (its root page AGENTS without its frontmatter, or the server's built-in ones when it has none) and an overview: how many pages and attachments it holds, when it last changed (null when nothing was ever written in it), and the nodes at its root. Follow its instructions while you work in it, and pass its id as notebook to the other tools. truncated is true when the root's nodes were cut short.",
      "inputSchema": {
        "type": "object",
        "properties": { "notebook": { "type": "string", "description": "The notebook's id, from notebooks." } },
        "required": ["notebook"],
        "additionalProperties": false
      },
      "annotations": { "idempotentHint": false, "openWorldHint": false, "readOnlyHint": true }
    },
    {
      "name": "read",
      "title": "Read a page or an attachment",
      "description": "<the notebook entry's read description, word for word>",
      "inputSchema": {
        "type": "object",
        "properties": {
          "notebook": { "type": "string", "description": "The notebook's id, from notebooks." },
          "path": "<as in the notebook entry>",
          "offset": "<as in the notebook entry>",
          "column": "<as in the notebook entry>",
          "limit": "<as in the notebook entry>"
        },
        "required": ["notebook", "path"],
        "additionalProperties": false
      },
      "annotations": { "idempotentHint": false, "openWorldHint": false, "readOnlyHint": true }
    }
  ]
}
```

（`<…>` 只在本文里省略重复的文字，入库的文件逐字写出。）`title` 写在工具的顶层，不重复进注解；读的工具不写 `destructiveHint`（协议说它只在 `readOnlyHint` 为假时有意义）。

### 3.19 各个新检查的反向对照

13.4 第 1 条：每个新的检查先证明它会失败。

| 检查 | 反向对照（各改坏一处，测试要失败） |
|---|---|
| 工具清单（`TestTheToolListsAreTheContract`） | `toolListDiff` 在合成的清单上：少一个、多一个、说明不同、属性的类型不同、注解不同、多一个成员、`required` 的次序不同、两个入口对调；经真的路径：接缝多登记的 `seam_notebook`、`seam_account` 各让比较失败、各点名自己的入口（3.5，设计审查 A-M10，S1 审查 C-M5） |
| 接线（`TestTheMCPToolsBuildBothServers`） | `New` 的规则改回"`Notebook` 而不在笔记本入口就 panic"：S3 的 `open_notebook` 让测试以 panic 的文字失败（3.5，A-I1） |
| 矩阵覆盖每个工具 | 合成的清单与行：少一行、行多一个入口、行指着没有的工具、写的工具的行没有 `write`、读的工具的行有 `write`、文件少一个键（3.6，A-M11） |
| archtest 的 SDK 规则 | 合成的边：模块包导入 SDK、httpserver 导入 jsonschema-go 报出，mcpserver 导入不报（3.7）；depguard 的规则：一个模块的 `_test.go` 导入 SDK 时 `make lint-go` 报出（S1 的审查手工核对一次，A-N10） |
| NUL 与代理项的推导 | 推导的函数在只在 `ops[].path` 里有字符串的合成 schema 上漏掉它时失败（3.13） |
| `API.Wrap` 的次序 | 以不倒序的 `Middlewares` 手拼的链：没有令牌又坏的请求体答 400，"答 401"的测试失败（3.2.1） |
| `no-store` | 不包写者的同一个 handler：200 的 `tools/call` 是 `no-cache, no-transform`，断言失败（证明 SDK 确实改写） |
| 恢复 panic | 子进程里跑不带 mcpserver 的包装的 SDK 的 server（不是不带恢复的同一个 server；证明等价，S1 审查 C-N2）：一次 `boom` 让进程以 2 退出（`os/exec` 跑测试二进制自己，环境变量选这一支）；带恢复时进程活着 |
| JSON-RPC 的批 | 结构表的根改成 `Any`：同一个批不带 `Mcp-Protocol-Version` 头（或带 2025-03-26）时 `echo` 被执行（计数大于 0）；带 2025-11-25 的那一次由 SDK 自己拒绝，所以反向对照不用它，整个程序的测试另断言 problem+json（A-M8） |
| 旧协议的 listen | `Capabilities.Tools.ListChanged` 改为真：带真的订阅（`toolsListChanged: true`）的 listen 一直持到调用的期限，"1 秒之内答完"的断言失败（A-M9） |
| 调用的期限 | 处理函数直接用 SDK 交的 ctx 时 `Deadline()` 答没有（单元测试同时断言两者） |
| 断开即取消 | 去掉 `AfterFunc(r.ctx, cancel)`：断开的客户端的 `slow` 跑到期限、日志是期限的 warn 而不是 `FaultGone` 的 debug，测试失败（A-M12） |
| SDK 的 logger | 写回草稿的 `.With(…)`：一个无状态的 POST 之后收集器里有 SDK 的 info 行（A-M2） |
| 答复没写 | 去掉 `ServeHTTP` 的最后一支：短期限、不看 ctx 的方法答空的 200，"500 problem、`no-store`"的断言失败（A-M3）；最后一支写回不看原因的 `errUnwritten`：断开的客户端让日志里出现 error 级的两行，"断开时没有 error 级的行"的断言失败（复核 A N-2） |
| 不同时返回结果与错误 | 一个直接返回两者的包装：日志收集器里出现 jsonrpc2 的 `%#v` 那一行 |
| 答复的上限与量 | 恰好上限照答、多一个字节答 `internal_error`；`TextBytes` 或 `MemberBytes` 改掉一条转义的规则（如 `\r` 记 6、`<` 记 1）时，与真的 SDK 编码比较的测试失败；`Fit` 改为按 `len(JSON)` 量时，全是引号的成员的答复超过上限、答 `internal_error`，测试失败（3.4.4，B-I1、A-M4） |
| `open_notebook` 的预算 | 写回草稿的"说明 32 KiB 加概况 16 KiB"：全是引号与换行的 32 KiB `AGENTS` 加 100 个长名称的节点答 `internal_error`，测试失败（3.17.3）；余地改为 12 KiB、16 KiB 或 8,192 ± 8：单元的预算用字面的 8,192 并断言截得最长，经 serve 的引号的一步同样，都失败（S3 审查 C-M1） |
| 最后的变更 | 写回草稿的标量子查询：空的笔记本 `open_notebook` 答 `internal_error`，`TestOpenNotebookOfAnEmptyNotebook` 失败（B-I2） |
| 路径的给出与找回 | 故意写错的 `PathOf`（页不带 `.md`、不 NFC）让性质测试失败（3.9.2） |
| 段的检查 | 去掉"答回的就是它的 NFC"的比较时，首尾带空白的段被收下，表格测试失败 |
| glob 的匹配 | 与正则的翻译在随机的模式与路径上比较（差分测试，`exp/glob` 30 万对、0 处不同）；改坏一处（`*` 的位置不留着、结尾的 `/**` 收零段）时失败；按树走与逐条路径匹配比较，状态不从父节点接时失败；去掉跨字的进位、末位放在第一个字里时，两个、三个字的段对正则与 DP 预言失败（S2 审查 A-I2） |
| glob 的工作上限 | 不检查上限：`TestTheWorstGlobsStopWithinTheirWork` 在 B 的守卫树上超过 2 秒、工作超过上限，失败（3.10.1，B-I5）；接着走时从根重走：计数时 `next_cursor` 不前进，不计数时守卫里接着走的那一次超过 2 秒，失败；单位按字符计、父节点的项在走过时才计：`TestTheGlobCountsItsWork` 失败（复核 B N-1、N-2、N-8） |
| WebP 的宽高 | 去掉 VP8L 的 `+1`：三种块的测试里 VP8L 那一条失败；`HasSize` 不加 `image/webp`：上传的 WebP 的行没有宽高，`asset/app` 的测试失败（3.11.2，B-I3）；审查之后：去掉 VP8 的掩码、VP8L 高的掩码、起始码、签名、`WEBP` 的比较、RIFF 与块的大小的检查、VP8L 的版本、VP8X 的大小，`TestWebPSize` 或 `TestSizeOfAWebP` 失败（S2 审查 B-M2 与修复核对） |
| `CleanClientName` 的幂等 | 照 00 号文档原来的次序写的变体在 `e`、U+200E、U+0301 上不幂等，表格里的这一行失败（2.1 第 2 条） |
| `CleanClientName` 的截法 | 在前 128 个字符上取 `LastBoundary`：129 个 ASCII 留下 127 个，"恰好留下 128 个"的性质失败（A-M5）；在前 129 个字符上取它（第一轮的写法）：128 个 `x` 加 `中`、129 个 `中` 留下 129 个，表格的这两行与"至多 128 个"的性质失败（复核 A N-1） |
| 名称的缓存与记下 | 读失败也缓存：下一次不再读，测试失败（A-M6）。S3 修复一轮的规则下：说出的名称不先记住（第一轮的写法）、失败的记下标作在行里、0 行之后记住 `""` 或不读就标、`onRow` 标任何缓存的名称、不报名称的调用不再记下、同名而不在行里的不再记下、同名而在行里的再记下、读回来的盖过其间说出的名称、去掉 `get` 的 `MoveToFront`（FIFO）、名称留作请求的一段：3.16.4 的各个测试失败（S3 审查 B-M1、B-M2、C-M2、B-N1） |
| 内置的说明提到每个工具 | `TestUnnamedToolsCatchesEachGap`：去掉 `read` 那一行、把 `- list:` 改成正文里的 "list" 时失败；`unnamedTools` 不锚定也失败（3.15.4，B-M1） |
| "列表等于逐项判定读取" | `TestAccountNotebooksDiffReportsEachDifference`：比较的函数在合成的数据上（多列一本、少列一本、角色不同）各报出；比较不看角色的变异失败 |
| `reserved_test` 的例外 | 只跳过以点开头的第一段：合成的模式 `/mcpx` 不在 `[server]` 时仍报出 |
| 计划测试（`RootByKey`、`ChildByKey`、`AgentsPage`、`PageStates`、`NotebookCounts`、`NotebookLastChange`、`FirstChildren`） | 照已有的计划测试：在测试的事务里去掉所靠的索引，断言的索引名不在计划里时失败；S2 的两条：`ChildByKey` 写回 `IS NOT DISTINCT FROM`、去掉根或子节点的索引（00019 的就被选上）、根的索引不唯一、语句不带 `parent_id IS NULL`、`parent_id` 当文字比、不带 `notebook_id`，`TestThePathLookupsArePointLookups` 在每种形状失败；去掉旧的兄弟的索引、Down 忘了重建它或把它建成 `NULLS DISTINCT`，`TestThePathKeysGoDownToTheSiblingsIndex` 失败；`nameTaken` 不认根或子节点的名字，`TestEverySiblingsIndexTellsANameTaken` 失败；`FirstChildren` 写回 `IS NOT DISTINCT FROM` 时 `parent_id IS NULL` 既不是索引条件也不是所读的索引的谓词，失败（A-M16、B-M3）；S3 的：去掉根的索引（`FirstChildren` 与 `AgentsPage` 各一次）、去掉以 `notebook_id` 起头的节点的索引、计数或最后改动不按列（`notebook_id::text`）、去掉日志的两个变更集的索引，`TestTheOverviewReadsUseTheirIndexes` 或 `TestTheAgentsPageIsAPointLookup` 失败；`AgentsPage` 写回设计的 LEFT JOIN，ANALYZE 之后有 `page_contents` 的 Seq Scan，失败（实施时的偏离 22） |
| `public_url` 的校验 | 每种被拒的写法各一行；去掉"路径只许 `/`"时带路径的一行失败；`@` 的检查从不匹配、只对带 scheme 的或只对带密码的，`TestValidatePublicURL` 失败（S2 审查 B-M1） |
| 按工具的码的表（S2 加的） | 一行少了它答的码、多了不声明的或从不答出的码、少了一个工具的行、一个工具答它的行没有的码：`TestEveryMCPToolHasItsCodes`、`TestTheMCPToolsErrorsCarryTheirCodes`（S3 改名）或整个程序读到它的那个测试失败（3.13） |
| `/register`（S1 加的） | 纯文字的答复、别的码、别的说明、不 `no-store`、不公开、不挂、不分方法：`TestTheMCPRefusesOAuthRegistration` 或 `TestEveryMCPAnswerIsNotStored` 失败（3.2.2） |
| 没有令牌的提示（S1 加的） | `NoTokenDetails` 不接、给每个路由：`TestTheMCPTellsEveryClientOneHint` 失败；`NoOAuth` 与 `/.well-known/` 的说明不同：它与 `/register` 的测试都失败（3.2.3） |
| 超过 64 KiB 的请求体（S1 加的） | 界线改为 128 KiB 或 32 KiB、`<` 代 `<=`、去掉名字的查找或写回只看名字像不像的规则、`jsonrpc` 不查或只查非空、`id` 的界线 255、512 或去掉、`_meta` 读作 `json.RawMessage`、入口总取账户的或任一个、只把空的查询串当账户入口：`TestOnlyACallMayBeLong` 的行失败，整个程序的 `TestTheMCPEdgeAnswersInOrder` 也失败（3.2.2） |
| 工具错误的码、矩阵的 `notebook`（S1 加的） | `toolError` 不查或什么码都收：`TestAToolErrorsCodeIsDeclared` 失败；`takesNotebook` 总答真：`TestTheMatrixGivesANotebookAsTheFileSays` 失败（3.6） |
| 限流与代理的 Host（S1 加的） | `Wrap` 跳过限流、`/register` 不经 `Wrap`：`TestTheMCPRoutesTakeTheBuckets` 失败；打开 SDK 的本机保护：`TestAProxysHostIsServed` 与整个程序的那一次握手答 403、失败（3.2.2、3.4.8） |
| 说明的缓存与名称存副本（S3 审查加的） | 不复制正文：`TestTheInstructionsCacheHoldsItsTextsAlone` 量出 256.1 MiB、超过 16 MiB，失败；不复制名称：`TestTheNamesHoldTheirOwnBytes` 失败（3.15.1、3.16.4） |
| 握手的上限处与期限（S3 审查加的） | 握手的 `fits` 写成 `<`：`TestTheHandshakesInstructionsAreWholeUpToTheirBound` 失败；以 `context.WithoutCancel(ctx)` 读说明：`TestTheInstructionsAreReadWithinTheCallsDeadline` 失败（3.15.2、3.15.3） |
| 矩阵的 `open_notebook` 的说明（S3 审查加的） | 答账户入口的文字：看得到这本笔记本的列失败（3.17.4） |
| MC1 的概况（S3 审查加的） | `last_changed_at` 总为 null、不数附件：MC1 的第 2 步失败（4.3） |

### 3.20 实施与审查之后的改动（S1–S3）

三个 Step 的审查与修复核对见 [S1 的审查记录](reviews/P1-S1-platform-review.md)、[S2 的审查记录](reviews/P1-S2-reads-review.md)、[S3 的审查记录](reviews/P1-S3-account-review.md)。与上文最初的设计不同之处已改在各节里：

- **S1，实施时定下的**（实施者的 22 条偏离，审查 C 判过）：SDK 随第 3 步提交（偏离 1）；ctx 还活着而没写答复时交 `errUnwritten`（2，3.2.2）；参数的错误超过 48 KiB 时丢掉 `fields`（3，3.3.6）；prompt 两个入口都登记、没有 `Get` 的 `Prompt` 在 `New` 里 panic（4，3.4.7）；`Bodies()` 的 `Items: Open`（8，3.2.2）；`/mcp` 的上限在 `wire.go` 合并（10，3.1、3.2.2）；`app.mcpTools`（11，3.1）；`POST /register`（19，3.2.2：试用的第 4b 步，主持者的决定）；提示原是 `/.well-known/` 的 404 的那句（20，修复一轮由 `NoTokenDetails` 取代，3.2.3）；`BOOTSTRAP_FIRST`（21，6.1）；panic 的子进程跑不带包装的 SDK（没列出的，审查 C-N2，3.19）；别的（5–7、9、12–18、22）是写法、测试的形状或流程。
- **S1，审查与修复核对带来的**：空的文字块（B-I1，3.3.2、3.3.4）；超过 64 KiB 的只能是这个入口登记的工具的 `tools/call`，`params` 的形状，`jsonrpc`、`id` 与 `_meta` 照 SDK 的读法（A-M1、修复核对第一轮 F1、F2 与可选的一条，3.2.2）；没有令牌的 401 的提示（A 的附加问题，3.2.3）；`/register` 的 `no-store` 在边缘（A-N1，3.2.3）；协议版本的例外（A-N3，3.2.2）；编码不了的 `Extra`、`Err` 为 nil、不是对象的结构答复、零项放不下的 `Fit`、拒绝的输入类型、没有处理函数、零值的 `Errors`（B-M2、B 的细节、C-N3，3.3.1–3.3.6、3.4.4）；调用一行的 `outcome` 与 panic 的日志（A-M2、A-N2，3.4.2）；`serverOf`（C-N7，3.3.2）；矩阵的 `notebook` 与工具错误的码（B-M1、B 的存疑，3.6）；两个接缝工具（C-M5，3.5）；测试（A-M3、C-M1–M7、C-N4，3.4.8、3.4.9、3.19、6.3）。修复二轮之后没有剩下的发现。合并之后的测试的修复（`c87caad7`）见 3.4.8。
- **S2，实施时定下的**（17 条偏离，审查 C 判过）：`domain.VersionToken`（1，3.9.3）；`Walk(ctx)` 与 `GlobWalk`（2，3.10.1）；`Listing` 没有 `More`（3，3.10.4、3.10.5）；`Path.Written`（4）；`InlineType`（5，3.11.2）；`shared.TitleHolds`（6，3.8）；`asset.ErrNotFound` 与 `ImageSize`（7，3.11.2）；WebP 的测试文件由 Chromium 写出（9，3.11.2）；`contentOf` 的空块（11，变基时撤回：S1 的修复做了同样的改动）；`mcpTools(pg, as, ln, nb)`（12，3.8）；第 5 步的提交带 `tools.json` 与矩阵的行（13）；三个 e2e 的类型不导出（14，4.1）；附件的最后一跳自己一个测试（15，3.13、6.3）；为让变异失败加的断言（16）；`link` 是 REST 的链接目标（17，3.11.2）。偏离 8（计划测试接受两种键的索引）不成立，由 00042 取代；偏离 10（没有 `public_url` 时不内联图片）由主持者改回。
- **S2，审查与修复核对带来的**：00042 的两个部分唯一索引、去掉兄弟的索引、`ChildByKey` 没有统计时的计划（A-I1、C-I2、A2-M1，3.9.2）；多字的段的测试与 DP 预言（A-I2，3.10.1）；`read` 先判定笔记本的测试（C-I1，3.11.1）；`MaxGlobWork` = 3 亿与 `և` 的守卫树、守卫的 `-v` 的行（A-M2，3.10.1）；长路径的列法与几处提示（A-M3、A-N3，3.9.2、3.10.1、3.10.2）；进了回收站的附件不列（A-N2，3.10.2）；游标的大小（A-N4，3.10.2）；`public_url` 的 `@`（B-M1，3.12）；WebP 的检查与测试（B-M2、B 的存疑 1，3.11.2）；`read` 的说明的余地（B-N2，3.18）；组合根的两行与按工具的码的表（B-M3、C-M3、C 的提议，3.13）；`has_children`、`mime`、`resource_link` 的测试（C-M1、C-M2、C-N4，3.13、4.4）；`decideNotebook`（C-N5，3.10.4）；照写的路径的 `page.not_found`（C-N3，3.11.1、3.11.2）；守卫那一份的作业时限（3.8）。修复二轮之后没有剩下的发现。
- **S3，实施时定下的**（实施者的 24 条偏离，审查 C 判过）：迁移是 00043（1，3.16.2）；`toldName(req)`（2，3.16.4）；S1 潜伏的名称的缺陷（3，原因由审查 A 更正，3.16.4）；`OpenNotebook` 以笔记本入口的 `NotebookGate` 判定（4，3.17.3）；握手读说明失败是 JSON-RPC 的内部错误、不写"MCP initialize"（5，3.15.2）；读行失败另一条 warn（6，3.16.4）；只答说明行（7，3.15.3）；名称留着 BOM（8，3.16.1）；附件的版本号 0（9，3.15.1）；`mcpServer` 多一个名称的端口（10，3.14）；`FrontmatterEnd` 读 `frontmatterSpan`（11，3.15.3）；`FirstChildren` 与计数、最后改动的计划测试的判据（15、16，3.17.3）；概况的守卫比较中位数（17，3.17.3）；MC1 不导入 MCP 的类型（18，4.1）；"列表等于逐项判定读取"覆盖每一列（19，3.17.4）；只改大小写的改名的变异由 page/app 与整个程序抓到（20，6.3）；锁文件加 76 个包（21，4.1）；`AgentsPage` 的子查询（22，3.15.1）；码的测试改名（23，3.17.4）；`BOOTSTRAP_FIRST` 移到 `TestEveryS` 之前（24，6.1）；别的（12–14）是测试的名字与提交的形状。
- **S3，审查与修复核对带来的**：说明的缓存与名称的 LRU 存副本（A-I1，3.15.1、3.16.4）；握手的上限处与期限的测试（A-M1、A-N1，3.15.2、3.15.3）；说出的名称立刻记住、到知道在行里为止再记下（B-M1、B-M2、C-M2，3.16.4）；LRU 的测试（B-N1，3.16.4）；`open_notebook` 的余地从下面也钉住（C-M1，3.17.3、3.17.4）；矩阵的 `open_notebook` 比较内置的说明（C-N1，3.17.4）；MC1 的附件与最后改动（C-N2，4.3）；Makefile 的注释与 6.1 取两次的平均（C-N4，6.1）；`NotebookCounts` 的注释（B-N3、C-N5，3.17.3）；两个客户端的镜像的交错（修复核对第一轮 B 的 N-1，3.16.4）。修复核对第一轮之后没有 Important 与 Minor；任务的等待与竞态那一份的时限在只改测试的分支 `m9-job-waits` 里改（合并 `0c934bb9`，第 9 节）。

## 4. 前端：只有 e2e

P1 没有网页的改动。e2e 的改动随第一个用它的故事进来（2.1 第 9 条）：S2 是脚本化的客户端与 MC2，S3 是 TypeScript SDK 与 MC1。

### 4.1 文件

| 文件 | Step | 内容 |
|---|---|---|
| `e2e/fixtures/mcp.ts` | S2 | 脚本化的 MCP 客户端（4.2） |
| `e2e/fixtures/server.ts` | S2 | `nervewikiEnv` 加 `NWIKI_SERVER__PUBLIC_URL: "http://wiki.example"`（`e2e/fixtures/server.ts:139-149`）：e2e 的服务端监听 `127.0.0.1:0`，写配置时不知道自己的地址，所以用一个固定的源；断言绝对地址以它起头，下载时把源换成 `baseURL`（签名只盖路径与查询） |
| `e2e/fixtures/assets.ts` | S2 | `webpBytes`：一张 1×1 的无损 WebP（VP8L，几十个字节），MC2 上传它 |
| `e2e/stories/mcp/mc2-read-list.spec.ts` | S2 | MC2（4.4） |
| `e2e/package.json`、`pnpm-lock.yaml` | S3 | `devDependencies` 加 `"@modelcontextprotocol/sdk": "1.32.1"`（精确的版本，13.5 第 2 条）；过仓库的 pnpm 门禁（`minimumReleaseAge: 1440`、`trustPolicy: no-downgrade`、`strictDepBuilds: true`，`pnpm-workspace.yaml`）：设计复核在照搬这三项的临时目录里以 pnpm 12.8.1 装过，新加 92 个包、没有要运行的安装脚本（00 号文档 4.16）；S3 的实施者在仓库里以 `pnpm --filter @nervewiki/e2e add -D -E` 装上，过了门禁（"Lockfile passes supply-chain policies"，没有要允许的构建），`pnpm-lock.yaml` 只加 76 个包、不删包：它与它的依赖（express 5、hono、ajv、jose、eventsource、pkce-challenge、zod-to-json-schema 等），`zod` 等已在锁文件里（实施时的偏离 21）；它必须的对等依赖 `zod`（`^3.25 || ^4.0`）取锁文件里已有的 `zod@4.6.5`（门禁的临时目录里是 `zod@4.6.7`；设计审查 B 的 Nit 10）；`pnpm install --frozen-lockfile` 与 `web` 的 `--offline --frozen-lockfile` 干净 |
| `e2e/fixtures/assert/identity.ts` | S3 | `expectTokenClient(db, tokenId, name)`：`SELECT client_name FROM api_tokens WHERE id = $1` 等于 `name` |
| `e2e/fixtures/mcp.ts` | S3 | 导出 `mcpURL`（MC1 的 TypeScript SDK 的传输要地址）；注释改写：`McpContent`、`McpTool`、`McpAnswer` 仍不导出（实施时的偏离 18） |
| `e2e/stories/mcp/mc1-connect.spec.ts` | S3 | MC1（4.3） |

**knip 在每个 Step 的末尾都要通过**（设计审查 B-M9）：knip 6.38 只在另一个文件导入一个值的导出时才算它被用着（它的 `referencedIn` 只跟类型的引用，`node_modules/knip/dist/typescript/visitors/walk.js:60-72`），仓库没有设 `ignoreExportsUsedInFile`（`knip.jsonc`），`make knip` 带 `--treat-config-hints-as-errors`。所以每个导出都随第一个导入它的故事进来：

| Step | 新的导出 | 谁导入 |
|---|---|---|
| S1 | 无（e2e 没有改动） | — |
| S2 | `mcp.ts`：值 `mcpClient`、`postMcp`；类型 `McpClient`。`assets.ts`：值 `webpBytes` | MC2 导入 `mcpClient`、`postMcp`（`notebook=nope` 答 404，4.4 第 7 步）与 `webpBytes`；`McpClient` 在 `mcpClient` 的签名里，同今天 `events.ts` 的 `BrowserStreams` 的做法。`McpAnswer`、`McpContent`、`McpTool` 不导出：knip 6.38 数一个被导出的函数的签名用到的类型，不数只被导出的 interface 的成员用到的类型，导出时它们报作没人用（S2 审查 C 核对过；实施时的偏离 14，这张表原来写错了）；`mcp.ts` 的注释写明 |
| S3 | `mcp.ts`：值 `mcpURL`（S2 里它是文件内的函数）；`assert/identity.ts`：值 `expectTokenClient`；开发依赖 `@modelcontextprotocol/sdk`。`McpAnswer`、`McpContent`、`McpTool` 仍不导出：MC1 按名称比较 `listTools()` 的答复与文件，不写它们的类型（实施时的偏离 18） | MC1 导入这三样（SDK 的 `Client` 与 `StreamableHTTPClientTransport`）；审查 C 的三个 knip 的对照（不用 `mcpURL`、不导入 SDK、不用 `expectTokenClient`）各让 knip 报出预期的一项 |

草稿在 S2 就导出 `mcpURL` 与 `postMcp`，而 S2 只有 MC2、它不用这两个：S2 的 `make knip` 会失败。

### 4.2 `e2e/fixtures/mcp.ts`

原生的 JSON-RPC 经 Node 的 `fetch`（同事件流的 `fetchEvents`，`e2e/fixtures/events.ts:46-48`），一个消息一个 POST：

```ts
/** A tool's answer as a client hands it on: whether it is an error, its structuredContent, and its text blocks. */
export interface McpAnswer {
  isError: boolean;
  structured: unknown;
  text: string[];
  content: McpContent[];
}

/** A scripted MCP client of one entry, initialized (M9 design 9). */
export interface McpClient {
  readonly protocolVersion: string;
  readonly instructions: string | undefined;
  listTools(): Promise<McpTool[]>;
  call(tool: string, args?: Record<string, unknown>): Promise<McpAnswer>;
}

/** The MCP address of baseURL: the account entry, or a notebook's. */
export function mcpURL(baseURL: string, notebook?: string): string;

/** credential's POST of body to the entry, as the server answers it: for the refusals before the SDK. */
export function postMcp(baseURL: string, credential: string | undefined, body: unknown, notebook?: string): Promise<Response>;

/** credential's client of the entry, initialized as client (default "e2e-agent"), then notifications/initialized sent. */
export async function mcpClient(
  baseURL: string,
  credential: string,
  options?: { notebook?: string; client?: string }
): Promise<McpClient>;
```

每个请求带 `Accept: application/json, text/event-stream` 与 `Content-Type: application/json`（否则 SDK 答 400，`sdk/mcp/streamable.go:388-398`）、`Authorization: Bearer …`；握手之后带协商出的 `MCP-Protocol-Version`（否则每个调用都按 2025-03-26 处理）。`initialize` 要 2025-11-25、`clientInfo` 是 `{name, version: "1"}`。期望的答复（设计审查 B-M8）：带 `id` 的请求是 200 `application/json`；没有 `id` 的消息（`notifications/initialized`）是 202、空的正文。别的状态、请求的答复不是 JSON、JSON-RPC 的错误（`error` 成员）都抛出，带状态与答复。

### 4.3 MC1 接入与说明（S3）

`newTeam` 建团队与它的管理员（`Team` 里没有 PAT 的 id，`e2e/fixtures/workspaces.ts:76-81`），所以 MC1 用的两个 PAT 甲、乙都由管理员以 `login(api, team.adminEmail)` 拿到会话之后经 `createToken(api, session.access_token, {name})` 另建（`e2e/fixtures/auth.ts:35`、`:67-85`，答复带 id；设计审查 B-M10）。"另一个团队"是另一个账户：`newTeam` 每次都注册 `emailFor(testInfo, "admin")`，同一个测试里第二次会撞上，所以第二个团队的管理员以邮箱的标签 `"other-admin"`（`register(api, emailFor(testInfo, "other-admin"))`）注册，再建自己的工作区（`slugFor(testInfo, "other")`，`e2e/fixtures/workspaces.ts:14`）与笔记本：

1. 建笔记本 `Handbook`，根下的页 `AGENTS` 的正文是 `---\ntags: [rules]\n---\n\n# Conventions\nWrite in English.\n` 与另两页 `Guide`、`Notes`；`Guide` 之下上传附件 `chart.txt`（根下的列表不变；S3 审查 C-N2）。PAT 甲以 `mcpClient(…, {notebook})` 连上：`protocolVersion` 是 `2025-11-25`；`instructions` 等于 `# Conventions\nWrite in English.`（去掉 frontmatter 与首尾的空白）；`listTools()` 解析之后等于 `api/mcp/tools.json` 的 `notebook`（读仓库里的文件，按名称比较）；`expectTokenClient(db, 甲的 id, "e2e-agent")`。
2. 账户入口（同一个 PAT 甲）：`instructions` 以 `This is Nerve Wiki` 开头；`listTools()` 等于文件的 `account`；同一个账户在另一个工作区里另有一本笔记本，`notebooks` 列出这两本（id、名称、工作区的 slug 与名称、角色 `admin`），不列另一个团队的笔记本，`truncated` 为假；`open_notebook` 以 `Handbook` 的 id 答出与第 1 步相同的说明、`instructions_from` 是 `AGENTS.md`、`overview.pages` 是 3、`attachments` 是 1、`last_changed_at` 不为 null、`root` 里有 `AGENTS.md`（S3 审查 C-N2：`last_changed_at` 总为 null 与不数附件的变异让 MC1 失败）。角色只有 `admin`：别的角色是用例算的，由 Go 的矩阵（`notebooks` 一行比较带角色的整个列表）与"列表等于逐项判定读取"逐列对照 REST，MC1 守的是线上与 SDK 的漂移（主持者的决定 7）。
3. 换一个 `AGENTS`：经 REST 把它改名为 `Old rules`、另建一页 `AGENTS`（正文 `Second rules.`）：新的握手是 `Second rules.`；删掉它：新的握手以 `This is a notebook of Nerve Wiki` 开头。
4. 拒绝：会话的访问令牌 `postMcp` 一个 `initialize` 答 403 problem+json `forbidden`；另一个团队的笔记本、已删除的笔记本答 404 `notebook.not_found`；三者都带 `Cache-Control: no-store`。
4a. 空的笔记本（设计审查 B-I2）：经 REST 新建一本 `Empty`，什么也不写：账户入口的 `open_notebook` 答 `overview.pages` 0、`attachments` 0、`last_changed_at` null、`root` 为空、内置的说明（以 `This is a notebook of Nerve Wiki` 开头）、`instructions_from` null，不是工具的错误。
5. TypeScript SDK（`@modelcontextprotocol/sdk` 1.32.1 的 `Client` 与 `StreamableHTTPClientTransport`，令牌经 `requestInit.headers`），PAT 乙（同一个 PAT 上 `expectTokenClient` 只看得到最后报的名称，00 号文档 4.5）：`new Client({name: "e2e-sdk-agent", version: "1.0.0"})` 经 `new StreamableHTTPClientTransport(new URL(mcpURL(baseURL, notebook)), …)` 连笔记本入口；`getInstructions()` 是这时的说明；`listTools()` 的名称是 `list`、`read`；`callTool({name: "list", arguments: {}})` 不是错误，`structuredContent.items` 列出根下的页（`Old rules.md`、`Guide.md`、`Notes.md`，树的次序）；`close()`；`expectTokenClient(db, 乙的 id, "e2e-sdk-agent")`。它守的是 SDK 或协议的漂移（握手之后它试开 GET 的流、得到 405 就不再试：不报错）。

### 4.4 MC2 读与列（S2）

经 REST 建笔记本 `Plans`：根下页 `Guide`（正文 `\uFEFF---\r\ntitle: Guide\r\n---\r\nLine one\r\nLine two\r\n` 的字节：BOM 与 CRLF，在测试里以 `"\ufeff"` 的转义写出），它下面依次建 `Setup`、`Alpha`、`Long`（3,000 行、每行 40 个字节）、`Wide`（一行 100,000 个 `x`、换行、`end`、换行），上传附件 `diagram.png`（`pngBytes`）、`dot.webp`（`webpBytes`）、`spec.pdf`、`logo.svg`；再经 REST 把 `Wide` 移到 `Setup` 之前（`moveNode`，`e2e/fixtures/pages.ts:91`）：树的次序从此是 `Wide`、`Setup`、`Alpha`、`Long`，既不是建的次序与 id 的次序（v7，随时间增长：`Setup`、`Alpha`、`Long`、`Wide`），也不是名称的次序（`Alpha`、`Long`、`Setup`、`Wide`），按 id 或按名称排兄弟的错误都测得出来（设计审查 B-M11）。四个上传各以 `expectUploaded`（`e2e/fixtures/assert/asset.ts:31`）核对它的行（00 号文档第 9 节"MC2 核对附件的行"；`dot.webp` 的上传另断言答复的 `width === 1 && height === 1`：`expectUploaded` 拿行与接口的答复比，两边都是 null 也通过；P1 起上传读 WebP 的宽高，复核 B N-3）。笔记本入口：

1. `list`（`pattern` 缺省）答 `Guide.md`，`has_children` 为真；`list("Guide/")` 答 `Guide` 的八个子节点，次序等于 REST 的 `listNodes` 里它们的次序（树的次序，`e2e/fixtures/pages.ts:75`；`Wide` 在最前），只有 `Setup` 的 `has_children` 为真（它下面只有一个附件 `table.csv`，`list("Guide/Setup/")` 答它；S2 审查 C-M1），页带 `version`（`<id>:1`）与 `byte_size`，附件带 `mime` 与 `byte_size`，都等于上传的（S2 审查 C-M2）；`list("**/*.md")` 按路径的键答 `Guide.md`、`Guide/Alpha.md`、`Guide/Long.md`、`Guide/Setup.md`、`Guide/Wide.md`（按键的字母序，与上面树的次序不同，所以两种次序都证明了）；`list("guide/*.PNG")` 答 `Guide/diagram.png`（不论大小写）；`list("Guide/", limit 2)` 答两项与 `next_cursor`，带它再列得到其后的两项。
2. `read("Guide.md")`：没有 `structuredContent`，第一块的 JSON 里 `total_lines` 是 5、`next_offset` 为 null，第二块的字节等于正文（BOM 与 `\r` 原样）；`read("/Guide.md", offset 2, limit 1)` 答第二行 `title: Guide\r\n`。
3. `read("Guide/Long.md")`：第一次的第二块至多 48 KiB、`next_offset` 不为 null；依次带 `next_offset` 读到它为 null，连起来等于正文。
4. `read("Guide/Wide.md")`：第一次答 1 行、`next_offset` 是 1、`next_column` 大于 0；依次带 `offset` 与 `column` 读完，连起来等于正文。
5. 附件：`read("Guide/diagram.png")` 与 `read("Guide/dot.webp")` 的第二块是 `image`、`mimeType` 各是 `image/png`、`image/webp`，`data` 解码之后等于上传的字节，元数据的 `mime`、`byte_size`、`sha256`、`width`、`height` 等于库里那一行的；`read("Guide/spec.pdf")` 与 `read("Guide/logo.svg")` 的第二块是 `resource_link`，`mimeType` 与 `size` 等于上传的（S2 审查 C-N4），`uri` 以 `http://wiki.example/api/v0/assets/` 起头，把源换成 `baseURL` 下载答 200、`Content-Disposition` 是 `attachment`、字节等于上传的（`download`，`e2e/fixtures/assets.ts:69-71`）。
6. `read("Guide")`（附件的写法、有同名的页）答工具的错误：文字以 `page.not_found` 起头并提示 `Guide.md`；`list("Guide")`（不带通配、没有配上）答空的 `items` 与提示 `Guide.md` 的 `hint`。
7. `postMcp` 一个 `initialize` 到 `notebook=nope`：404 problem+json `notebook.not_found`、`Cache-Control: no-store`。

## 5. 实施步骤

每个 Step 单独合并都要是绿的：`make lint-go`（含 `gochecknoglobals` 与 3.7 的 depguard）、`make knip`、工具清单的检查、矩阵的覆盖、archtest 的规则，加上照常的门禁（6.1）。每个 Step 的末尾写明这几项在这个 Step 证明了什么：在 S1 里有两项是空的（两个入口与文件都没有工具），它们的证明来自反向对照（设计审查 A-M10、A-M11）。

### S1（分支 `m9-p1-s1`，从 `main`）

1. `go get github.com/modelcontextprotocol/go-sdk@v1.8.0`，`go mod tidy`（3.1 的版本，`asm` 是 v1.2.1）；archtest 的规则与合成的边、depguard 的规则（3.7）；`archtest/binary_test.go` 照样通过。
2. httpserver：`ProblemOf`、`Fault`、`Answer`，`Write` 改用它（已有的测试照旧通过）；`API.Wrap` 与它的测试；导出的 `MCPPath`、`setFixedHeaders` 的前缀与它的测试（审查之后另有 `RegisterPath`、`APIConfig.NoTokenDetails`、bodyshape 的 `Bytes` 与 apitest 的 `ProblemCodes`）。
3. mcpserver：类型（3.3.2）、schema 的推导、整理与结构表（3.3.3）、两个 server 与 `ServeHTTP`（含"答复没写"的一支，3.2.2、3.3.1）、调用的 ctx 与恢复（3.4.1–3.4.2）、`no-store`（3.4.3）、量、`Fit` 与答复的上限（3.4.4）、错误（3.3.6）、日志与 SDK 的 logger（3.4.5）、`handshake` 的日志部分、最小的 `Prompt` 与替身（3.4.7）；单元测试（3.4.8）。包级的东西只有 `errors.New` 的哨兵与常量（`Bodies()`、`Subtree()`、`WellKnown()`、`notPAT()` 是函数）。
4. page：`NotebookGate`、`MCPEntry()`（`Instructions` 答 `""`），导出 `MaxContentBytes`。
5. 组合根：`bootstrap/mcp.go`、`/mcp` 的挂载与上限、两个 404、`wiring` 的接缝；`mcp` 移到 `[server]`、`reserved_test` 的例外；`configs/config.yaml` 的注释。试用之后加 `POST /register`：挂载、公开的操作、`register` 进 `[server]`（3.2.2）。
6. `api/mcp/tools.json`（空的两个数组）、`TestTheToolListsAreTheContract`、`TestTheMCPToolsBuildBothServers` 与两条反向对照（3.5）；矩阵的框架与覆盖、反向对照（3.6）。
7. 整个程序的测试（3.4.9）；各检查的反向对照（3.19 的 S1 部分）。
8. 仓库之外的 TypeScript 客户端试用的传输的几步（6.4 的第 2–5 步，S1 的二进制）：结果写进本文第 9 节；发现的不合在 S1 合并之前改（会走 OAuth 的客户端在 `/register` 拿到网页的 405：实施者停下，主持者定了 `POST /register`，`e40142e8`，第三次运行核对）。
9. 门禁、审查。组合根两份的界线照 6.1 移到 `TestEveryR` 之前（`043185ba`）。

**S1 合并时各项证明什么**：

| 检查 | S1 证明的 |
|---|---|
| `make lint-go` | 新代码没有包级的可变状态（`gochecknoglobals`）；只有 mcpserver 与 bootstrap 的文件导入 SDK，测试文件也算（depguard） |
| `make knip` | e2e 没有改动，照旧通过（不证明 MCP 的什么） |
| 工具清单 | 两个入口与文件都是空的、文件恰好两个键：主测试只证明"都空"；`TestTheToolListCheckCatchesEachDifference`（比较的函数会失败）与 `TestTheToolListCheckReadsTheApp`（取数的路径读的真是应用的两个入口与文件的两个键，接缝的 `seam_notebook`、`seam_account` 各让它失败、点名自己的入口）是这一 Step 的证明 |
| 接线 | 空的工具集建得起两个 server；`New` 的规则由 mcpserver 的单元测试证明（只在账户入口的 `Notebook` 工具登记得上） |
| 矩阵的覆盖 | 没有行也没有工具：只证明文件的两个键在；`TestMCPMatrixViolationsCatchesEachGap` 在合成的数据上证明它会失败（含 `write` 的两种） |
| archtest | 真的：SDK 已在 `go.mod` 里，只有 mcpserver 导入它；合成的边证明它会失败 |
| 整个程序的测试 | 边缘、传输、恢复、期限与断开、批、listen、`no-store`、重启，经接缝的测试工具，是真的；审查之后另有 `/register` 与没有令牌的提示、两个路由的桶、超过 64 KiB 的请求体、代理的 Host |

### S2（分支 `m9-p1-s2`，S1 合并之后）

1. page 的 domain：`ParsePath`、`PathOf`、`PathNotFound`、`VersionToken`、`ParseGlob` 与位并行的匹配（3.10.1；与正则的差分测试、按树走与逐条路径的对照、单位的计数、接着走的界）、`CheckWindow`、`Cut`（计数函数由调用者交）、`InlineImage`、`ChildrenCursor`、`GlobCursor` 与上限；单元与性质测试；`TestTheWorstGlobsStopWithinTheirWork` 加进 `make test-go-costs`（`MaxGlobWork` 已定，3.10.1 的实测）。
2. page 的存储：`RootByKey`、`ChildByKey`、`PageStates`，`make gen`；存储的测试与计划测试。审查之后：迁移 00042（根与子节点的部分唯一索引，去掉兄弟的索引；`schema_test`、`sqlc.yaml`）、`nameTaken`、`AliveNodes`（3.9.2、3.10.2）。
3. page 的用例：`NodePaths.Find`、`ListByPattern`（按树走、不拼全部的路径、工作的上限、`hint`）、`ReadPath`；表格测试（码的次序）；`TestEveryNodesPathFindsIt`。
4. asset：`webpSize`、`Sniffer.Size` 的分支、`HasSize` 加 `image/webp`（模糊测试、三种块、截短的头，3.11.2）；`api/modules/asset.yaml` 的说明、`make gen`；`Deps.PublicURL`、`AttachmentReads`（`Read`、`Content` 与旧行的宽高）与它的测试（真实的库与存储）；config 的 `server.public_url`（校验、`LogValue`、`config.yaml`）。
5. page 的 `adapter/mcp`（`list` 经 `Fit`、`read` 经 `Cut` 与 `TextBytes`、错误的文字、元数据行）与它的测试；`Tools(ToolDeps)`；组合根的 `mcpTools`、`pageAttachments`。
6. `api/mcp/tools.json` 加 `list`、`read`；矩阵的两行；整个程序的测试（3.13）。
7. e2e：`mcp.ts`（导出 `mcpClient`、`postMcp` 与类型 `McpClient`；另三个类型 S3 才导出，4.1）、`assets.ts` 的 `webpBytes`、`server.ts` 的环境变量、MC2。
8. 仓库之外的 TypeScript 客户端试用的读的一步（6.4 的第 1 步：`list`、`read`），结果写进第 9 节；反向对照、门禁、审查。审查之后另有按工具的码的表（3.13）与守卫那一份的作业时限（3.8）。

**S2 合并时各项证明什么**：工具清单核对 `list`、`read` 在两个入口的全部成员（账户入口的 `notebook` 在 `required` 的最前）；接线证明 `BothEntries, Notebook: true` 的两个工具进了两个 server、账户入口里有必填的 `notebook`；矩阵的覆盖核对两行在、`entries` 对、都没有 `write`（两个都是 `readOnlyHint`）；archtest 与 depguard 证明 page 的 `adapter/mcp` 只导入 mcpserver；knip 证明 `mcp.ts` 与 `assets.ts` 的每个新导出都被 MC2 导入（4.1 的表）；lint 同 S1；计划测试证明两条点查在四种形状上用的是 00042 的索引（`ChildByKey` 在 ANALYZE 之后）；码的表证明 `list`、`read` 只答它们的行的码。

### S3（分支 `m9-p1-s3`，S2 合并之后）

1. `platform/markdown.FrontmatterEnd` 与它的测试（与 `frontmatterOf` 答的位置相同）；page 的 `Markdown` 端口。
2. `AgentsBody`、`AgentsText`、`AgentsPage`（带标题；计划测试）、`Instructions` 与缓存（替身的存储上的 G16、"读之间换了页"、只改大小写的改名）、两段内置的说明与它们的测试、`MCPEntry().Instructions`；mcpserver 的 `handshake` 填说明（数调用次数的测试）。
3. `shared.CleanClientName`（截到 128 个字符之内的规范化边界）与性质测试；迁移 00043、`schema_test`、`sqlc.yaml`；identity 的两条语句（`:execrows`）与 `NewClientNames`；mcpserver 的 `names`（实施时不缓存没写下的与读失败的；修复一轮改为说出的立刻记住、到知道在行里为止再记下，3.16.4）；组合根的 `mcpNames`。
4. workspace 的 `Membership`；notebook 的 `AccountWorkspaces`、`ListAccountNotebooks`、`Tools()` 与 `notebooks`（经 `Fit`）；page 的 `OpenNotebook`、`NotebookCounts`、`NotebookLastChange`、只在根下的 `FirstChildren`（计划测试与守卫）与 `open_notebook`（按剩下的预算填，3.17.3）。
5. `api/mcp/tools.json` 加 `notebooks`、`open_notebook`；矩阵的两行；整个程序的测试（3.16.4、3.17.4，含空的笔记本）；`TestTheBuiltInInstructionsNameEveryTool`。
6. e2e：`@modelcontextprotocol/sdk` 1.32.1、`mcp.ts` 导出 `mcpURL`、`expectTokenClient`、MC1。
7. 反向对照（含 `TestTheMCPToolsBuildBothServers` 对 `New` 的规则的变异）、门禁、审查；收尾的文档（第 7 节）。组合根两份的界线照 6.1 移到 `TestEveryS` 之前（`19bb107b`）。

**S3 合并时各项证明什么**：工具清单核对账户入口的四个工具（`open_notebook` 只在账户入口、`notebook` 必填）；接线证明 `notebooks`（`AccountEntry`）与 `open_notebook`（`AccountEntry, Notebook: true`）登记得上（设计审查 A-I1、B-I4 的那一处）；矩阵的覆盖核对四行；内置的说明讲到两个入口的每个工具；archtest 与 depguard 证明 notebook 的 `adapter/mcp` 只导入 mcpserver；knip 证明 `mcpURL`、`expectTokenClient` 与 SDK 都被 MC1 用着；计划测试证明 `AgentsPage` 在四种计划下是根的索引加正文的主键、概况的三条读只读这本笔记本；守卫证明计数与最后改动不比读树慢；码的表证明 `notebooks`、`open_notebook` 只答它们的行的码。

## 6. 测试与验证

### 6.1 门禁

`make lint-go lint-web knip gen-check test-go test-web build`、e2e 全量，三个 Step 各一遍。`test-go-costs` 加 glob 的守卫与概况的守卫（S2、S3）。

**bootstrap 的两半**（设计审查 A-N9）：`BOOTSTRAP_FIRST = ^Test[A-D]`（`Makefile:198`），本文的整个程序的测试都叫 `TestThe…`、`TestMCP…`、`TestOpen…`、`TestTool…`，全落在第二半，而它在 M8 收尾时已是较大的一半（206 秒对 201 秒）。估计 P1 给第二半加约 25–35 秒：期限的测试至少等 4 秒、断开的一秒、重启的测试建第二个应用（约 2 秒）、矩阵建 13 个 PAT 再调约 100 次、G16 的五种换法各几次握手、名称的测试重建一次应用、MC 之外的整个程序测试约十个。S1 的实施者在 S1 合并之前量两半；第二半超过第一半一成以上时（S3 起按每份单独跑两次的平均，见本段末），照 M8/P4A 的分法把 `BOOTSTRAP_FIRST` 的界线往后挪（例如 `^Test[A-F]`），挪了之后两半各自的时间写进 S1 的审查记录；S2、S3 照样再量一次。量的结果（各自单独、竞态下）：S1 原来的 `^Test[A-D]` 212.7 秒与 250.1 秒（+17.6%），移到 `^Test([A-D]|E([a-u]|very[A-Q]))`（`TestEveryR` 之前；整个 `TestE` 51.5 秒都移会让前一份长约三分之一）之后 217.0 秒与 235.9 秒（+8.7%），CI 上前一份反而长（7:10 对 6:02）；S2 没有动：第一次门禁 224.3 秒与 239.9 秒（+7.0%），修复一轮之后 219.7 秒与 242.7 秒（+10.5%；CI 上 6:20 与 6:41），S3 的整个程序的测试也都落在第二份，照主持者的决定在 S3 量了之后移一次（[S2 的审查记录](reviews/P1-S2-reads-review.md)主持者的决定 7）。S3：变基之前 233.0 与 246.6 秒（+5.9%），在 S2 修复一轮上 213.3 与 241.8 秒（+13.4%：第二份几乎没动，是第一份在两次运行之间从 233.0 降到 213.3 秒），移到 `^Test([A-D]|E([a-u]|very[A-R]))`（`TestEveryS` 之前：`TestEveryR` 开头的 vault 的导出与导入两个、约 18 秒进第一份）之后 237.9 与 228.5 秒，main 上 234.0 与 230.5 秒；是重新平衡，S3 自己的组合根的测试不带竞态约 3 秒。审查 C 在移动之后的界线上跑一次：260.1 与 225.1 秒（第一份 +15.5%）。单独跑一次的时间前后可差一成以上（同一棵树上第一份 213–260 秒，CI 上同一份 5:44 到 7:59），"多过十分之一"在一次运行的摆动之内，所以从 S3 起判断挪不挪取每份单独跑两次的平均（S3 审查 C-N4，主持者的决定 6；Makefile 的注释写明）：修复一轮各两次、交替，236.0 与 250.4 秒、260.1 与 238.3 秒，平均 248.1 与 244.4 秒（第一份 +1.5%），不移。CI 上两份在同一条界线上交替长短（一次第一份长 17%，下一次第二份长 18%）；第二份的 go test 一步到过 7:54（15 分钟的约 53%），P2 的整个程序的测试多半落在第二份，在那里再量（第 8 节）。

**每个 Step 的绿**：第 5 节逐个 Step 写明 lint、knip、工具清单、矩阵的覆盖与 archtest 在那时证明了什么。

### 6.2 按项的测试（汇总，细节在第 3 节）

| 项 | 测试 |
|---|---|
| 边缘（401、429、413、400、403、405、404 的次序，`no-store`） | 3.2.1、3.4.8、3.4.9 |
| 传输（无状态、协议版本、能力与缓存、旧协议的 listen、批、重启） | 3.4.8、3.4.9 |
| 调用的 ctx、断开即取消、恢复、结果与错误、量与 `Fit`、答复的上限、答复没写、日志与 SDK 的 logger | 3.4.1–3.4.5、3.4.8 |
| 清单、接线、矩阵、archtest 与 depguard | 3.5、3.6、3.7 |
| 路径、段、版本令牌 | 3.9（表格、四种情形、性质测试、计划测试） |
| glob 与 `list` | 3.10（单元、与正则的差分、按树走与逐条路径的对照、`MaxGlobWork` 的守卫与用完时的 `next_cursor`、两种游标、表格、计划、上限处：`limit` 1 与 1,000、0 与 1,001，48 KiB 处的 `next_cursor`、`hint`） |
| `read` | 3.11（行、offset、column 的边界与 422、limit、48 KiB 处的切法、零行、连起来等于正文的性质测试；附件的四种图片、3 MiB 与 8,000 像素的上限处、SVG、没有宽高、行里没有宽高的 WebP、没有 `public_url`、PNG 的文件丢了；WebP 的头：三种块、截短、模糊测试） |
| `server.public_url` | 3.12 |
| NUL 与代理项的推导、可选参数的 `null` | 3.13 |
| `AGENTS`、说明、G16 | 3.15（表格、上限处：`TextBytes` 32,768，`AgentsBody` 与握手一层各测；只改大小写的改名；整个程序的五种换法与引号的一步；交错；`Instructions` 只为 `initialize` 调、在调用的期限之内读；缓存存副本（量堆）；内置的说明讲到每个工具） |
| 客户端名称 | 3.16（性质测试含 128 个字符的截法、迁移与约束；缓存：说出的立刻记住、到知道在行里为止再记下，失败的读不记住，空的结果记住，两个客户端的交错，读的期间说出的名称，LRU 而不是 FIFO，名称的副本；三种来源；重启之后，含说出已记下的名称） |
| `notebooks`、`open_notebook` | 3.17（"列表等于逐项判定读取"、用例一层的 1,000 项、48 KiB 处的截断；`open_notebook` 按剩下的预算填：全是引号与换行的 32 KiB `AGENTS` 加 100 个长名称的根节点、101 个根节点、长标题，余地用字面的 8,192 而且截得最长（单元与经 serve）；空的笔记本；计划测试、守卫） |
| e2e | MC1、MC2（第 4 节） |

### 6.3 反向对照与变异

- 3.19 的每一行在实施时先证明失败，再改回。
- 另做的变异（审查时抽查，各改坏一处）：每一个都由下面点名的测试抓住（设计审查 A-M15：草稿的两个变异没有测试抓得住）。测试名在实施时照此；括号里是写它的那一节。

| 变异 | 抓住它的测试 |
|---|---|
| 边缘的 403 放到 `notebook` 的判定之后 | `TestTheMCPEdgeAnswersInOrder`：会话的访问令牌带一个看不到的笔记本答 403（3.4.9） |
| GET 也调 `Gate` | mcpserver 的 `TestTheEntryGatesPostsOnly`：替身的 `Gate` 数调用，GET、DELETE 答 405 而调用数为 0（3.4.8） |
| `entryOf` 改回 `u.Query()` | `TestTheEntryOfAMalformedQueryIsNotFound`：`notebook=%zz`、`notebook=<id>;x=1` 答 404 而不是账户入口（3.4.8） |
| `getServer` 对账户入口答 nil | `TestBothEntriesListTheirTools`：账户入口的 `tools/list` 答 SDK 的 400 而不是工具（3.4.8） |
| `New` 的规则改回草稿的 | `TestTheMCPToolsBuildBothServers`（S3 的 `open_notebook`，3.5）与 mcpserver 的 `TestAnAccountNotebookToolRegisters`（3.4.8） |
| `handshake` 在 `next` 之前读说明而不看结果的类型 | `TestTheHandshakeReadsInstructionsOnlyForInitialize`：数调用的替身，`ping`、`tools/list`、`tools/call` 为 0（3.15.2） |
| 调用的期限取较晚的 | `TestTheCallContextTakesTheEarlierDeadline`（3.4.8） |
| `AfterFunc` 去掉 | `TestTheMCPCallStopsWhenTheClientGoes`：断开之后 1 秒之内取消、日志是 `FaultGone` 的 debug（3.4.9） |
| 恢复不区分 `tools/call` | `TestRecoveryAnswersEachMethod`：`tools/call` 的 panic 是 `isError`，`initialize` 的是 -32603 的 JSON-RPC 错误（3.4.8） |
| 结构的工具用 SDK 的 `json.Marshal` | `TestTheStructuredResultKeepsHTMLCharacters`：解码 `content` 的文字之后 `<>&` 原样（3.4.8） |
| `sdkLogger` 写回 `.With(…)` | `TestTheSDKLogsOnlyWarnings`（3.4.5） |
| `ServeHTTP` 不管没写的答复 | `TestAnUnwrittenAnswerIsAProblem`（3.4.8；问两个工具，`c87caad7`） |
| `ServeHTTP` 的最后一支不看原因（写回第一轮的 `errUnwritten`） | `TestTheMCPCallStopsWhenTheClientGoes`：断开时日志里有 error 级的行，失败（3.4.9，复核 A N-2） |
| `Fit` 按 `len(JSON)` 量 | `TestFitHoldsTheAnswersMeasure`：全是引号的成员（3.4.8） |
| 去掉 `recoverTool`、换两个 defer 的次序（S1 审查加的） | `TestTheCallsAndHandshakesLogOneLineEach`、`TestRecoveryAnswersEachMethod` 与整个程序的 `TestTheMCPRecoversAToolsPanic`：`boom` 一行的 `outcome` 是 `internal_error`（3.4.2） |
| 空的文字块被拒、文字与图片或链接同块被收（S1 审查加的） | `TestAnEmptyBlockIsText`、`TestAMalformedResultIsInternal`（3.3.4） |
| 错误的成员不编码一次交给 SDK、参数超过 48 KiB 时不回退（S1 审查加的） | `TestAnUnwritableErrorIsInternalAtOnce`；`TestTheArgumentsAreCheckedAsTheSchemaSays`（3.3.6） |
| `/register` 与没有令牌的提示（S1 加的） | `TestTheMCPRefusesOAuthRegistration`、`TestEveryMCPAnswerIsNotStored`、`TestTheMCPTellsEveryClientOneHint`（3.4.9） |
| 超过 64 KiB 的请求体的各条（S1 审查加的） | `TestOnlyACallMayBeLong`、`TestTheParamsAreShapedBeforeTheSDK`，整个程序的 `TestTheMCPEdgeAnswersInOrder`（3.2.2） |
| `TextBytes` 把 `\r` 记 6、`<` 记 1 | `TestTheMeasuresAreTheWires`：与真的 SDK 编码比较（3.4.8） |
| `ParsePath` 不去开头的 `/`、收结尾的 `/` | `TestParsePath` 的表格（3.9.1） |
| `ChildByKey` 不带 `notebook_id` | `TestThePathLookupsArePointLookups`：计划是子节点的唯一索引、两列都是等值的条件，`notebook_id` 在行的过滤里（3.9.2；S2 之前写的"兄弟的唯一索引的三列"随 00042 不再成立） |
| `Find` 不查最后一段的类型 | `TestFindByPath` 的四种情形（3.9.2） |
| `list` 的子节点按名称或按 id 排 | `TestListByPatternAnswersInOrder` 的表格（替身的树，次序值与名称、id 的次序都不同）与 MC2 第 1 步（4.4） |
| 两种游标解进同一个结构 | `TestTheListCursorsRefuseTheOtherShape`：`{}`、子节点的模式拿到 glob 的游标、反过来，都答 400（3.10.2） |
| glob 的匹配不检查 `MaxGlobWork` | `TestTheWorstGlobsStopWithinTheirWork`（3.10.1） |
| glob 的状态不从父节点接（每个节点从根重算） | `TestTheGlobWalkIsThePathsMatch` 照样通过，而守卫的时间超过 2 秒失败（3.10.1） |
| glob 接着走时从根重走（计数或不计数） | `TestTheGlobResumesFromItsCursor`：`next_cursor` 不前进，或每次的工作超过界、新工作之和超过一次走完（3.10.1，复核 B N-2） |
| glob 的单位按字符而不是字节计 | `TestTheGlobCountsItsWork`：`é` 与 `漢` 的键的计数（3.10.1，复核 B N-1） |
| 父节点的项在走过时才计，而不是进入时 | `TestTheGlobCountsItsWork`：只走一项就停的 10 万项的父节点记下它的全部项（3.10.1，复核 B N-8） |
| `Cut` 在 `\r` 处分行、切开时丢掉 `\n` | `TestCutRebuildsTheContent`：依次读完的各段连起来等于正文（性质测试，3.11.1） |
| `column` 的检查放到笔记本之前 | `TestReadPathAnswersInOrder`：看不到的笔记本上不在边界的 `column` 答 404（3.11.1） |
| `InlineImage` 收 SVG、不看宽高 | `TestInlineImage` 的表格（3.11.2） |
| `webpSize` 的 VP8L 不加 1 | `TestWebPSize` 的三种块（3.11.2） |
| 绝对地址用请求的 `Host` | `TestTheMCPReadsGiveAttachments`（附件的最后一跳，实施时的偏离 15）：地址以 `http://wiki.example` 起头，测试服务的 `Host` 是 127.0.0.1（3.13） |
| `read` 先找路径、后判定笔记本（S2 审查加的） | `TestReadPathAnswersInOrder`；整个程序的 `TestTheMCPToolsErrorsCarryTheirCodes`（S3 改名）与矩阵（3.11.1） |
| 只数页的子节点的 `has_children`、`list` 的附件的 `mime` 写死（S2 审查加的） | `TestListByPatternAnswersInOrder`、`TestTheListAnswersItsItems`、`TestTheMCPReadsGiveAttachments` 与 MC2（3.10.2、4.4） |
| 子节点的游标不按 id 断平手（S2 审查加的） | `TestListByPatternPagesOnFromItsCursor`：相同的 `sort_order`、id 的次序与名字和存的次序都不同（3.10.2） |
| 没有 `public_url` 时不内联图片（S2 审查加的） | `read_test.go` 没有地址的各行、`TestTheMCPReadsWithoutAPublicURL`（3.11.2） |
| `AgentsPage` 不看 `parent_id IS NULL` | `TestTheInstructionsFollowTheAGENTSPage`：子页里的 `AGENTS` 答内置的说明（3.17.4） |
| 缓存以点查的版本号存 | `TestTheInstructionsCacheByTheRevisionRead`（替身的存储在点查与读正文之间换掉页，3.17.4） |
| `AgentsBody` 不去 BOM | `TestAgentsBody` 的表格（3.15.3） |
| 缓存连说明行与标题一起存 | page/app 的 `TestTheInstructionsFollowTheAgentsPage` 与整个程序的只改大小写的改名（`TestAgentsText` 看不到它：`AgentsText` 的标题是参数；实施时的偏离 20，3.15.1） |
| `CleanClientName` 先 NFC 再去 `unshowable` | `TestCleanClientName` 的 `e`、U+200E、U+0301 一行（3.16.1） |
| `CleanClientName` 在前 128 个字符上取 `LastBoundary` | `TestCleanClientNameKeepsAllItCan`：全是起始字符的长名称恰好留下 128 个（3.16.1） |
| `CleanClientName` 在前 129 个字符上取 `LastBoundary`（第一轮的写法） | `TestCleanClientName` 的 128 个 `x` 加 `中`、129 个 `中` 两行，与 `TestCleanClientNameKeepsAllItCan` 的汉字、数字、`-` 起始字符（3.16.1，复核 A N-1） |
| `Record` 每次都写 | `TestTheNamesRecordOnlyWhatDiffers`（3.16.4） |
| `names` 不缓存空的结果 | `TestTheNamesRememberAnEmptyName`：从没报过名称的令牌两次调用只读一次端口（3.16.4） |
| `names` 缓存没写下的名称或失败的读（修复一轮之后：写了一行或读到行里是它之前就标作在行里） | `TestTheNamesKeepATellingUnrecorded`（原来的 `TestTheNamesRetryASkippedWrite`）、`TestTheNamesRetryAFailedRead`、`TestTheNamesFindATellingOnTheRow`、`TestTheNamesOfTwoClientsAtOnce`（3.16.4） |
| `ListAccountNotebooks` 不经 `ReachedByAccess` | `TestTheAccountsNotebooksAreWhatEachReadAllows`（3.17.4） |
| `open_notebook` 不判定 | `TestThePermissionMatrixOverMCP` 的 `open_notebook` 一行：看不到的列答 `notebook.not_found`（3.17.4） |
| `open_notebook` 写回固定的 32 KiB 加 16 KiB；余地改为 12 KiB、16 KiB 或 8,192 ± 8（S3 审查加的） | `TestOpenNotebookFitsItsAnswer`（3.17.3）；12 KiB 经 serve 由 `TestTheInstructionsFollowTheAGENTSPage` 的引号的一步抓住（3.17.4） |
| 最后的变更写回标量子查询 | `TestOpenNotebookOfAnEmptyNotebook`（3.17.4） |
| `FirstChildren` 写回 `IS NOT DISTINCT FROM` | `TestTheOverviewReadsUseTheirIndexes`（3.17.3） |
| `AgentsPage` 写回设计的 LEFT JOIN（实施时加的） | `TestTheAgentsPageIsAPointLookup`：ANALYZE 之后有 `page_contents` 的 Seq Scan（3.15.1） |
| 说明的缓存不复制正文（S3 审查加的） | `TestTheInstructionsCacheHoldsItsTextsAlone`（3.15.1） |
| 名称的 LRU 不复制名称（S3 审查加的） | `TestTheNamesHoldTheirOwnBytes`（3.16.4） |
| 握手的 `fits` 写成 `<`（S3 审查加的） | `TestTheHandshakesInstructionsAreWholeUpToTheirBound`（3.15.3） |
| 以 `context.WithoutCancel` 读说明（S3 审查加的） | `TestTheInstructionsAreReadWithinTheCallsDeadline`（3.15.2） |
| `get` 不 `MoveToFront`（FIFO，S3 审查加的） | `TestTheNamesForgetTheLeastRecentlyUsed`（3.16.4） |
| 0 行之后记住 `""`、读回来的盖过其间说出的名称、`onRow` 标任何缓存的名称（S3 审查加的） | `TestTheNamesKeepATellingUnrecorded`、`TestTheNamesFindATellingOnTheRow`、`TestTheNamesKeepATellingOverAReadBeforeIt`、`TestTheNamesOfTwoClientsAtOnce`；经 serve 的 `TestTheClientNameIsTheTokens`（3.16.4） |
| 矩阵的 `open_notebook` 答账户入口的文字（S3 审查加的） | `TestThePermissionMatrixOverMCP` 的 `open_notebook` 一行：看得到的列（3.17.4） |
| `last_changed_at` 总为 null、不数附件（S3 审查加的） | MC1 的第 2 步（4.3） |

### 6.4 仓库之外的 TypeScript 客户端试用（第 2–5 步在 S1 的末尾，第 1 步在 S2 的末尾）

照 M0/P1 的做法（[01-P1-spikes](../M0-foundation/01-P1-spikes.md) 第 7 节）：在临时目录（不在仓库里）照搬 `pnpm-workspace.yaml` 的三项门禁，以 pnpm 12.8.1 装 `@modelcontextprotocol/sdk@1.32.1`，一个 Node 脚本连本机以 test 配置运行的 `bin/nervewiki`（自己的 PostgreSQL 容器）：

1. 笔记本入口：握手、`tools/list`、调用 `list` 与 `read`，记下协商的版本与说明；
2. 记下它在握手之后发的 GET 与 405 之后的行为（不再试、不报错）；
3. 客户端进程活着时停掉并重启服务，下一次调用不重新握手就成功；
4. 不带令牌连接：记下它在 401 之后找 `/.well-known/` 之下哪些路径、拿到 404 之后给出什么提示；
5. 带会话的访问令牌：记下 403 的提示。

分两次跑（设计审查 A-M14、B-M14）：第 2–5 步只要 S1 的二进制（握手与空的 `tools/list` 已经活着），在 S1 的末尾、合并之前跑：00 号文档第 10 节靠的"P1 在仓库之外先试重启与 GET"就在无状态的传输与边缘进 `main`、S2 建在它们之上之前做了；第 1 步要真的工具，在 S2 的末尾跑。代码用完丢掉，结果（版本、每一步的请求与提示的原文）写进第 9 节；与 00 号文档 4.3、4.4 的假设不符的（例如它对 405 报错、对 404 的 OAuth 元数据当成配置错误）先停下，改设计再合并那个 Step。

**结果**（逐步的原文在第 9 节）：S1 跑了四次。第 4b 步（带 OAuth 的 provider）不符：四个 `/.well-known/` 的 404 之后客户端 `POST /register`，拿到网页的 405；实施者停下，主持者定了 `POST /register`（3.2.2），第三次运行核对；修复一轮的 `NoTokenDetails`（3.2.3）之后第四次运行核对第 4a 步也显示同一句。第 2、3、5 步与假设相符。S2 的第 1 步没有不符。

## 7. 完成标准

- 第 2 节的每一项做完，第 3、4 节的测试通过，门禁全绿；3.19 的反向对照都做过。
- glob 的工作上限（`MaxGlobWork`，S2 之后 3 亿个单位）与它的实测已在 3.10.1；守卫在 CI 上每次走查的耗时打在成本那一份带 `-v` 的行里，还没人读过（日志要权限才读得到），交 M12（第 8 节）。TypeScript 客户端试用的两次结果写进第 9 节。
- 审查记录：设计 `reviews/P1-design-review.md`；实施 `reviews/P1-S1-platform-review.md`、`reviews/P1-S2-reads-review.md`、`reviews/P1-S3-account-review.md`；00 号文档的进度表与本文档的"结果"一节随每个 Step 的合并更新。
- **P1 的文档提交做的改动**（随本文档一起，主持者以 `apply-00m9.py` 落下）：`00-M9-design.md` 的 2.1 的各条（含 A-M4 对 4.6 转义规则的更正）与它们碰到的文字；进度表 P1 一行指向本文；变更记录一行。
- **随各 Step 的合并改的**：
  - `v0.1-design.md` 第 13 节（行号以那时的为准）：
    - 13.1 第 2 条（S3）：MCP 的客户端名称取请求的 `clientInfo.name`（`initialize` 或 `_meta`），否则这个 PAT 记下的（`api_tokens.client_name`，进程内按 PAT 的缓存），都没有时 `unknown`；清理是 `shared.CleanClientName`（次序与幂等），组合根在交给 identity 之前清理。
    - 13.1 第 3 条（S1、S3）：`notebooks` 没有规则行，进"列表等于逐项判定读取"的测试（`TestTheAccountsNotebooksAreWhatEachReadAllows`）；MCP 的矩阵按工具（`permission_matrix_mcp_test.go`，13 列各有 PAT），`TestThePermissionMatrixCoversEveryTool`。
    - 13.1 第 4 条（S1–S3）：`/mcp` 边缘的次序（没有令牌 401 → 闸门 429 → 坏的令牌 401 → 桶 429 → 413、400 → 413（超过 64 KiB 而不是这个入口的工具的调用）→ 403 → 不是 POST 的 400（协议版本）、405 → 404）；查询串解析不了、`notebook` 出现两次、为空、`uuid.Parse` 不收时答 404 不查库；账户入口的笔记本由用例判定、排在与资源无关的 422 之后；`list`、`read`、`notebooks`、`open_notebook` 的次序（3.10.4、3.11.1、3.17.2、3.17.3）。
    - 13.1 第 8 条（S1）：工具的错误是 `isError` 的结果，文字与 `structuredContent.error` 同一段，码同 REST；`ProblemOf(ctx, err)` 是 REST 与 MCP 共用的分类；参数的结构错误的字段写法同 REST。
    - 13.1 第 10 条（S1）：MCP 的调用与握手各一行 info 的日志、从不记参数与答复；SDK 的 logger 在 warn 以上。
    - 13.1 第 11 条（S1–S3）：模块根的 `Tools()`（page 的 `Tools(ToolDeps)` 在 asset 之后）、`MCPEntry()`、`(*asset.Module).AttachmentReads()`、`identity.NewClientNames(pool)`、`page.AccountInstructions()`（`page.NewNodePaths(pool)` 随 P4）。
    - 13.1 第 12 条（S1、S3）：mcpserver 的小接口（凭证、笔记本入口；S3 的名称，编译期断言在 `wire.go`）由组合根按结构接上，平台不读 `shared`。
    - 13.1 新增第 32 条（S1；设计审查 B-M13，照第 23 条 River 的写法；排在最后，不给第 24–31 条重新编号，另记 `encoding/json/v2` 的实验开关）：MCP SDK（`github.com/modelcontextprotocol/go-sdk`）与 `github.com/google/jsonschema-go` 只由 `platform/httpserver/mcpserver` 与组合根的测试导入：archtest 的 `mcpSDKStaysInMCPServer` 管生产代码，depguard 的规则连测试文件也管；模块登记 `mcpserver.Tool`，不碰 SDK 的类型。
    - 13.1 第 13 条（S1）：MCP 的契约是先写的 `api/mcp/tools.json`，整个程序的测试解析之后比较，没有 `-update`。
    - 13.1 第 14 条（S1）：改写"保持响应的路由"一段：去掉"（SSE、MCP 流）"里的 MCP 与"M9 的 MCP 流照此挂载"；加"`/mcp` 经 `API.Wrap` 挂在根路由器上（逐路由的中间件、只许对象的请求体、按路由的上限），不是保持响应的路由；SDK 在自己的 goroutine 里执行方法，mcpserver 的最外层中间件派生调用的 ctx、恢复 panic；`no-store` 由写者守住"。
    - 13.1 第 15 条（S2）：`server.public_url`（只到源，为空时不签绝对地址、图片照样内联；被拒的值带 `@` 的不照录）。
    - 13.1 第 17 条（S1）：`/mcp` 只收 PAT，会话的访问令牌答 403 `forbidden`（不是 401：凭证本身有效）；`/mcp` 不在契约里，`TestEveryOperationAcceptsAPersonalAccessToken` 不涉及它；`POST /register` 与 `/.well-known/` 是组合根挂的、契约之外的公开路由（公开的操作只由模块声明的例外）。
    - 13.1 第 20 条（S1）：一个 `/mcp` 的 POST 一个 `authenticated` 的单位，与 REST 共用 `pat:<id>` 的桶，没有 MCP 自己的桶。
    - 13.1 第 21 条（S1–S3）：MCP 工具与 prompt 的注册表的形状与最后一跳（附件读、`AccountWorkspaces`、名称；prompt 以替身证明，最后一跳写给 M10；`NodePaths` 的最后一跳在 P4）。
    - 13.1 第 26 条（S1）：`mcp` 与 `register` 在 `[server]`（`register` 只服务 POST，不遮住已有的同名工作区）；`/.well-known/` 之下答 404，`reserved_test` 不比较以点开头的第一段。
    - 13.1 第 27 条（S2）：`list` 的游标是两种载荷（子节点的次序值与 id、glob 的路径键），按模式解；glob 的匹配用完工作的上限时同样答 `next_cursor`。
    - 13.1 第 28 条（S2）：MCP 的路径与 glob 按段以 `TitleKey` 比较；兄弟的唯一改由 00042 的根与子节点的两个部分唯一索引兜底（00012 的兄弟的索引去掉）。
    - 13.1 第 31 条（S1–S3）：MCP 的上限（答复 48 KiB，按每块文字在答复里由 `encoding/json` 写出的字节、结构的工具的字符串转义两次，经 mcpserver 的 `Fit` 一个帮助函数填；路径；glob 的长度、通配与一次的工作 `MaxGlobWork`；`list`、`read`、内联的图片、说明、`notebooks`、`open_notebook`、客户端名称）。
    - 13.4 第 1 条：3.19 的反向对照；第 3 条：e2e 的 MCP 客户端 `fixtures/mcp.ts`，MC1 另经 TypeScript SDK 的客户端；第 5 条：工具清单、PAT、NUL 与不成对的代理项、JSON-RPC 的批、panic、`no-store` 的整个程序测试；第 6 条：按工具的矩阵；第 8 条：glob 的最坏形状到工作的上限（B 的守卫树）与概况的守卫；第 9 条：路径的点查、`AgentsPage`、`PageStates`、`NotebookCounts`、`NotebookLastChange`、`FirstChildren` 的计划测试；第 10 条与第 13 条：路径的给出与找回、glob 按树走与逐条路径的匹配、量与真的编码。
  - 第 13 节之外的总体设计：6.3 与 8.4 已随 M9 的总设计改过，P1 不另改；P1 偏离的写法（2.1 第 1、3、4 条）不在总体设计里；WebP 的宽高（2.1 第 14 条）：总体设计的 :273、:724 只说"宽高"，不列类型，不改。
  - **M7 的上传读 WebP 的宽高**（S2，2.1 第 14 条）：这改了 M7 的上传的行为（新上传的 WebP 的行有了宽高），也碰到 Obsidian 的导入（同一条 `Blobs.Put`：导入的 WebP 也有宽高）与阅读视图（嵌入的 WebP 写出 `width`、`height`）：都是想要的，在下面 M7/P2 的那一句里一并说（复核 B N-3）。契约 `api/modules/asset.yaml:403-407` 的 `width`、`height` 的说明"(PNG, JPEG, GIF)"加 WebP，`make gen` 重新生成（S2 的一步）；`docs/v0.1/M7-assets-transfer/02-P2-assets-server.md:206` 是 M7/P2 那时的设计记录，不改写，在它的"宽高"一条后加一句"M9/P1 起 WebP 也读（RIFF 头；上传与 Obsidian 的导入都是，阅读视图随之写出它的宽高，[M9/P1](../M9-mcp/01-P1-platform-reads.md) 3.11.2）"，同前面的 M 被后面的 Phase 改动时的做法。
  - `README.md`：
    - "运行后端"加一小节"MCP"（S3）：两种地址（`/mcp`、`/mcp?notebook=<id>`）、只收 PAT（会话的访问令牌 403）、无状态的 JSON 答复（协议版本到 2025-11-25）、P1 的四个工具、`AGENTS` 页与内置的说明、`server.public_url`；"接入 Claude Code、Codex"的配置与反向代理的要求在 P5 写（00 号文档 4.16）。
    - "附件"的"签名地址"一段（S2）：MCP 的 `read` 给的是 `server.public_url` 起头的绝对地址，为空时没有地址：图片照样内联，别的附件只给元数据。
    - 没有配置令牌时客户端显示的那一句（S1 的试用第 4 次运行之后的写法，第 8 节 P5）：00 号文档 4.16 把"401、403、404、429 各说明什么"放在 P5 的接入一节，留到那里，按人工清单里 Claude Code 与 Codex 真的显示的核对。
    - "上传"一段（`README.md:143`，S2）："PNG、JPEG、GIF 读出宽高"加 WebP。
  - `server/configs/config.yaml`：`public_url` 与它的注释（S2），长连接一句的注释（S1）。
  - `docs/README.md`：不改（三个 Step 都写在本文里，没有另开 `specs/`、`plans/`）。
  - **移交**：
    - M10：新建 [`M10-llm-wiki/handoffs/M9-P1-mcp-registry.md`](../M10-llm-wiki/handoffs/M9-P1-mcp-registry.md)（`status: open`，`from: M9/P1`，`to: M10`），见第 8 节；随 S3 的合并建起：第 8 节的 M10 各条都在其中，00 号文档的"写给后面的 M"指向它。
    - M12：`M12-release/handoffs/M4-performance.md` 加一项（第 18 项，接在 M8/P5 的第 17 项之后），见第 8 节（S1、S2 的部分随它们的合并写进，S3 的随 S3）；同一个文件的第 11 项（阅读视图里的大图，`:37`）的"WebP、SVG、AVIF 不知道宽高"改为"SVG、AVIF 不知道宽高（M9/P1 起 WebP 读 RIFF 头）"（S2，复核 B N-3）。S3 另在打磨移交 `M12-release/handoffs/M5-polish.md` 加第 30 条（三处，第 8 节）。

## 8. 交给后面的 Phase

- **P2**（写的工具与历史的读）：
  - `ParseVersionToken`（3.9.3 写定的格式，`String()` 的往返；名字随 `domain.VersionToken`）与"令牌的节点是路径的节点"的比较；写的工具在单元的锁下按路径找（经 `NodePaths` 的同一个 `PathStore`，在单元的事务里）。
  - `domain.MCPClient(Call.Client)`：`Call.Client` 已清理（幂等，3.16.1），`mcp:` 的前缀与 `Client.Valid`、两张表的检查；`mcp:<名>` 的最后一跳（两种来源与重启之后记下的名称：P1 的 `mcp_names_test.go` 已备好三种情形的应用）；S3 起说出的名称立刻是这个令牌之后的调用的，记下被跳过或失败时也是（3.16.4）。
  - 工具的错误的文字：`page.locked` 带路径与持锁人、`page.revision_mismatch` 带当前的令牌（`ToolError.Extra` 的 `current_version`）。`structuredContent.error` 的 `lock`、`locks` 与 `Extra` 要有上限（例如 `linking.pages_locked` 至多列 N 页、其余计数）：错误的答复连同它们也在 48 KiB 之内（3.3.6，设计审查 A-N11），P1 的读没有它们。
  - 写的工具在矩阵里的行带 `write`（3.6 的覆盖检查要它：`readOnlyHint` 为假的工具的行没有 `write` 时失败）。
  - 内置的说明的"Tools"一段加写的工具与 `batch`、锁、版本令牌的用法（3.15.4 的测试守住）。
  - 写的工具的矩阵格用库的副本；`read` 与 `write` 之间同一路径换了一页的交错。
  - **超过 64 KiB 的请求体的并发界线**（S1 的主持者的决定，审查 A 的界线）：整个服务同时至多两个超过 64 KiB（`maxOtherBytes`）的请求体，在 httpserver 里、bodyshape 读请求体之前取：有 `Content-Length` 时按它，没有的读过 64 KiB 时；取不到的答 503 `server_busy` 带 `Retry-After`。上限处一个合法的调用峰值约 +0.5 GB、分配约 1.4 GB，四个同时 +1.4 GB（00 号文档第 10 节），两个的界线让峰值约 1 GB；P2 的 `write`、`edit` 的大正文正是这样的请求体。REST 的正文路由（约 31 MiB）同一类，可以共用这份预算。
  - **按工具的码的表**（3.13）：P2 的每个工具在组合根的 `mcpToolCodes()` 里加一行（它答的每个码，契约声明的），`TestEveryMCPToolHasItsCodes` 要它，`TestTheMCPToolsErrorsCarryTheirCodes`（S3 起它走遍码的表的每一行）答出每行的每个码；P3 的 `page.batch_too_large` 没有 REST 的操作，明列进声明的码（00 号文档 4.7）。
  - **组合根两份在 P2 再量**（6.1）：第二份的 go test 一步到过 7:54（15 分钟的约 53%），P2 的写的矩阵与整个程序的测试多半落在第二份；判断挪不挪取每份单独跑两次的平均。
- **P3**：`batch` 的 `ops` 是数组里的对象：结构表与 NUL 的推导已覆盖嵌套（3.3.3、3.13）；`page.batch_too_large` 的文字经 `ToolError`。
- **P4**：
  - `NodePaths.Paths`（按 id 给路径）与模块根的 `page.NewNodePaths(pool)` 随 P4 来（P1 没有使用者，设计审查 B 的过度设计第 1 条，3.9.2）：只为问到的 id 拼路径（B-M2），linking 的 `Deps.NodePaths` 接上、`backlinks` 的最后一跳在 P4。
  - `grep` 的 glob 复用 `domain.Glob` 的位并行的匹配与按树走（3.10.1，设计审查 B-I5）：它在走查之前与候选取交，同一个 `MaxGlobWork` 管它的匹配（与 `grep` 自己的工作分开计），守卫加 `grep` 的形状。
  - `grep` 与 `search` 的答复经 `mcpserver.Fit` 停在 48 KiB（3.4.4），不自己数字节。
- **P5**：接入文档、反向代理（`/mcp` 不是 SSE，`client_max_body_size` 不小于约 36 MiB，`proxy_read_timeout` 不短于 45 秒）、人工清单；第 9 节的 TypeScript 客户端试用的结果供接入文档的"401、403、404、429 各说明什么"。
  - **人工清单加的几步**（00 号文档 4.16；S1 审查 A、C 与 S2 审查 B 的存疑）：Claude Code 与 Codex 没有配置令牌时各显示什么（401 的说明、`/register` 的错误；Codex（rmcp）会不会 `POST /register`，退到源站的 `GET /authorize`、`/token` 时拿到的是网页的 200 `index.html`）；一个请求里 20 张以上的图片时每张的上限降为 2,000 像素，Claude Code 把工具结果的图片留在历史里，读过 20 张以上、其中一张宽过 2,000 像素之后的请求会不会被拒；Claude Code 是否把 3 MiB 的内联图片（4 MiB 的 base64）算进它的 MCP 输出上限（约 25,000 个 token）、截断或拒绝。
  - **接入文档里没有令牌时的那一句**（S1 的试用第 4 次运行之后的写法，留到 P5 的接入一节，照人工清单里两个客户端真的显示的核对、改写）：

    > 没有配置令牌时，客户端连不上 `/mcp`，显示的都是同一句提示："This server has no OAuth: MCP takes a personal access token in the Authorization header."：不走 OAuth 的客户端从 401 的说明里读到它；会走 OAuth 的客户端（例如 Claude Code）先找 OAuth 的元数据、再试注册，从注册的错误里读到它。照提示在客户端的配置里加上 `Authorization: Bearer <个人访问令牌>` 即可。

- **M10**（[`M9-P1-mcp-registry.md`](../M10-llm-wiki/handoffs/M9-P1-mcp-registry.md)）：
  - `mcpserver.Prompt` 已有，是最小的形状（名称、标题、说明、字符串的参数、返回说明与文字消息的处理函数，3.4.7），M9 以替身证明登记、`prompts/list`、`prompts/get` 与 `prompts.listChanged: false`；M10 要的别的（参数的检查、别的消息的类型）在这个注册表上加。M10 登记 ingest、query、lint 三个 prompt 时写整个程序的最后一跳（经组合根拼起的 prompt 到达处理函数，交空时失败），并把它们加进 `api/mcp/tools.json` 的检查（清单的形状加 `prompts`）。
  - 虚拟的 `index.md`、`log.md` 要进 `list` 与 `read`：`NodePaths.Find` 与 `ListByPattern` 在树之外认得它们（根下的这两个路径），glob 的 `**/*.md` 是否列出它们由 M10 定；写它们答"系统生成"。
  - `AGENTS` 页在 `llm_wiki` 下的保护由模式的守卫做；说明的找法与缓存照 3.15（替换、改名都换键）。
  - `prompts/get` 的请求体同 `tools/call` 之外的每个方法至多 64 KiB（3.2.2）：prompt 的参数要放得下，或者另设例外；`Get` 的错误还没有 `ProblemOf` 的分类与 48 KiB 的上限（3.3.6、3.4.7；S1 审查 B 的细节 7、修复核对第一轮 A）。
- **M12**（性能移交第 18 项）：
  - `list` 每次读整棵树（`ListNodes`，10 万个节点约 70 ms），内存是树本身与每个父节点排好的项（3.10.4：不再拼每个节点的路径，设计审查 B-M2）；glob 的匹配到 `MaxGlobWork`（S2 之后 3 亿）时本机约 0.2–0.8 秒（3.10.1 的表，主机另有负载；最慢的是键为名字两倍长的 `և` 的树，0.66–0.82 秒），慢的主机按比例，真实负载下的值再量。接着走的每次调用都重新进入游标的祖先所在的父节点、为它们的项建串与排序（照算进上限，复核 B N-8）：B 的守卫树的根 20 万个项约 0.1–0.2 秒，键的公共前缀长时排序比它记的单位贵（3.10.1）；3 亿时守卫树的重进占上限的 17%，一个文件夹约 58.5 万个子节点（117 万个项；4 亿时约 78 万个）时重进就吃掉整个上限，每次调用只前进一项。要时可以把游标之后的项先滤出再排，或按需取出（堆），由 M12 定。
  - glob 的守卫在 CI 上的耗时没人读过：读一次成本那一份带 `-v` 的行（每次走查一行、每棵树最慢的一行）；本机单次走查的余量 2.45–2.6 倍，与别的耗 CPU 的包一起跑时失败过。守卫那一份的作业到过约 11 分钟（11:08、11:05；时限已提到 25 分钟），同一份代码在不同的 runner 上差约 1.5–2 倍（S2 修复二轮的 CI 上 6:43，上一轮 11:05；组合根两份交替长短）。竞态那一份的作业到过 8:17–8:22（S2 的 8:20，S3 修复一轮的 8:17、合并之后的 8:22），时限照约两倍的规则从 15 分钟提到 20 分钟（只改测试的分支 `m9-job-waits`，合并 `0c934bb9`）。
  - `ChildByKey` 没有统计时的计划（3.9.2）：到 autovacuum 第一次 ANALYZE 之前可能经 00019 读这本笔记本里这个键的全部节点；大的导入之后一个变常见的键可能还不在 `nodes` 的 MCV 里，规划器以为它少见（S2 审查 A 的存疑，没有造）。恢复、导入之后先 ANALYZE 已在性能移交第 10 项。
  - `read` 每次读整页的正文（至多 5 MiB）再切：一页 5 MiB 的页依次读完约 110 次调用，从数据库搬约 550 MiB（设计审查 B 的 Nit 13）；按行的范围读正文要存储的新读法，留给 M12 定。
  - `open_notebook` 的计数读这本笔记本的全部活的节点；最后的变更与网页的活动读的一样，是这本笔记本的全部变更集（`updated_at` 不在索引里，3.17.3）。概况的守卫：10 万个节点、10 万个变更集上本机的中位数是树 51–100 ms、计数 6–10 ms、最后改动 4–8 ms，约低 9 倍。`FirstChildren` 读根下的全部节点再取前 101 个（`sort_order` 不在索引里）：一个根下 10 万个节点时 10–18 ms，对读树的 66 ms（S3 审查 B），不在守卫里。
  - `notebooks` 逐个工作区一条语句（一个账户的工作区很少），用例读到 1,000 项就停。
  - `/mcp` 的大的请求体：上限处一个合法的调用实测分配约 1.4 GB、峰值约 +0.3–0.5 GB（四个同时 +1.4 GB；S1 审查 A 与实施者在同一进程里量的，原来写的"36 MiB 时约 150 MiB"不对），P2 限为同时两个（上面）；压测量负载下的；一个 PAT 的桶仍允许 200 个同时的请求。说明的缓存 256 项至多 8 MiB（存副本：原来钉住整页的正文，1 MiB 的页写 256 次留着 256 MiB，S3 审查 A-I1）；名称的 LRU 1,024 项，每项是至多 128 个字符的副本。
- **M12**（[打磨移交](../M12-release/handoffs/M5-polish.md)第 30 条；S3 审查 A 的存疑 1、B-N2、B 的存疑 1）：
  - 截在最后一个换行可能远低于 32 KiB：一行 7,000 字节（量 12,002 字节）之后一行 20,766 字节跨过界线，说明只留 12,085 字节；一种改法是最后的换行离界线太远时退回字符的边界，是设计的问题（3.15.3）。
  - 只由看不见的字符组成的客户端名称照样记下（U+200B、U+2060、标签字符、U+00AD、U+FEFF、U+180E、U+3164、U+115F、U+2800）：`CleanClientName` 与 `CheckName` 一样留下，shared 的 `unshowable` 是 Cc、Zl、Zp 与 Bidi_Control；一种做法是没有图形字符的名称算空，也许另去掉 U+E0000–U+E007F 的标签块，两处一起改（3.16.1）。
  - 客户端在名称的语句里走了，记 warn（带 `context canceled`），S1 对客户端走了记 debug；标没设时不报名称的调用也记下，一直失败的记下每次调用一行 warn。只是日志的级别与数量（3.16.4）。

## 9. 结果

### 9.1 S1：平台、传输与边缘（2026-10-18，合并 `384d9f96`）

- 提交：实施：httpserver 的 `ProblemOf`、`API.Wrap` 与 `/mcp`、`/.well-known/` 的 `no-store` `d2955e9f`，mcpserver 与 SDK v1.8.0、archtest 与 depguard 的规则 `3d080ec4`，page 的 `NotebookGate` 与 `MCPEntry` `398894b8`，组合根挂载 `/mcp`、它的请求体上限与两个 404 `9cd54989`，整个程序的 MCP 测试、工具清单的契约与矩阵的框架 `4346eed8`，`POST /register` 答 OAuth 的错误与 PAT 的提示 `e40142e8`，组合根两份的界线移到 `TestEveryR` 之前 `043185ba`（从 `main` 的 `7f03403c` 开始）；审查的修复：空的块是文字、工具的答复的每种错立刻答出 `0f5beb11`，只有 `tools/call` 可以长到 `/mcp` 的上限、`params` 的形状在 SDK 之前查 `19cd0aba`，没有令牌的 401 带提示、`/register` 的答复在边缘不存 `b216368e`，两个路由的桶、每个入口一个接缝工具、矩阵的 `notebook` 照文件、工具错误的码 `da48b92a`；修复核对第一轮之后的修复二轮：超过 64 KiB 只给这个入口的工具、`jsonrpc`、`id`、`_meta` 照 SDK 的读法 `03898697`，C 的四条细节 `abb045dd`，带客户端自己的参数的长调用是账户入口的 `d1d6e923`。合并 `384d9f96`（第二个父提交 `d1d6e923`）。合并之后 main 的 CI 上 `TestAnUnwrittenAnswerIsAProblem` 在竞态的一份失败（run 39053755609）：测试的竞态，SDK 可能先把停下的工具的错误写进已关的连接；测试改为两个工具 `c87caad7`，合并 `cfb48a50`。
- 与设计不同之处已改在第 3 节（3.1–3.7、3.19）、第 5、6 节与 2.1 第 23–26 条，3.20 分实施时定下的与审查带来的列出。
- TypeScript 客户端试用（6.4 的第 2–5 步）：在仓库之外的临时目录（用完删掉）照搬三项门禁（`minimumReleaseAge: 1440`、`strictDepBuilds: true`、`trustPolicy: no-downgrade`），pnpm 12.8.1 装 `@modelcontextprotocol/sdk@1.32.1`（92 个包、没有安装脚本），Node v24.15.0；服务是 `heavy.sh make build` 的 `bin/nervewiki` 0.1.0-dev（第 1、2 次运行 `4346eed8`，第 3 次 `e40142e8`，第 4 次 `da48b92a`），`NWIKI_ENV=test`、固定的端口，自己的 `postgres:18.6-trixie` 容器；账户、工作区、笔记本与 PAT 经 REST 建。四次运行：1 加 `/register` 之前；2 在客户端的 fetch 里模拟 `/register` 的答复；3 加 `/register` 之后；4 修复一轮的 `NoTokenDetails` 之后。
  - 第 2 步（握手与 GET，各次相同）：协商出 2025-11-25；`serverInfo` `{"name":"nervewiki","version":"0.1.0-dev"}`，能力 `{"tools":{}}`，没有说明、没有会话；握手之后 GET 答 405 `text/plain` `Allow: POST`，之后 5 秒里什么也没有，`transport.onerror` 与 `client.onerror` 都没有调；`tools/list` 答 `{"tools":[],"ttlMs":0,"cacheScope":"private"}`。
  - 第 3 步（客户端进程活着时以 SIGTERM 停掉服务、在同一个端口重启）：`tools/list` 与 `ping` 都答 200，没有新的 `initialize`；`close()` 什么也不发（没有会话可删）。
  - 第 5 步（会话的访问令牌）：`connect threw StreamableHTTPError: Streamable HTTP error: Error POSTing to endpoint: {"status":403,"code":"forbidden","title":"Forbidden","detail":"MCP takes a personal access token, not a session's access token."}`。
  - 第 4a 步（没有令牌、没有 OAuth 的 provider）：第 1、3 次是平台的 401 `{"status":401,"code":"unauthorized","title":"Unauthorized","detail":"This operation requires a bearer token."}`（带 `WWW-Authenticate: Bearer`）；第 4 次 detail 是 "This server has no OAuth: MCP takes a personal access token in the Authorization header."。
  - 第 4b 步（没有令牌、带 OAuth 的 provider，同 Claude Code）：401 之后依次 `GET /.well-known/oauth-protected-resource/mcp?notebook=<id>`、`/.well-known/oauth-protected-resource`、`/.well-known/oauth-authorization-server`、`/.well-known/openid-configuration`，都是 404 problem+json（detail 是上面那一句），然后 `POST /register`。第 1 次那里是网页：`405 text/plain; charset=utf-8 Allow: GET, HEAD`，客户端报 `ServerError: HTTP 405: Invalid OAuth error response: SyntaxError: Unexpected token 'M', "Method Not Allowed" is not valid JSON. Raw body: Method Not Allowed`，与 00 号文档 4.4 的"给出平常的失败"不符：实施者停下，主持者定了 `POST /register`（3.2.2）。第 2 次模拟 `/register` 答我们的 404 problem+json：客户端照样报 OAuth 的错误的格式不对（`HTTP 404: Invalid OAuth error response: [ { "expected": "string", "code": "invalid_type", "path": ["error"], … } ]`）。第 3、4 次：`POST /register` 答 `400 application/json {"error":"invalid_client_metadata","error_description":"This server has no OAuth: MCP takes a personal access token in the Authorization header."}`，客户端报 `InvalidClientMetadataError: This server has no OAuth: MCP takes a personal access token in the Authorization header.`。
  - 对照 00 号文档 4.3、4.4：GET 的 405、重启、403 的文字、不走 OAuth 的客户端的 401 都相符；会走 OAuth 的客户端的失败只在有 `/register` 之后相符；第 4 次之后两种客户端显示同一句。服务端的日志：每次握手一行"MCP initialize"（S1 里 `client: unknown`）；唯一的 error 行是编排停掉服务时 River 的 `PeriodicJobEnqueuer: Error starting transaction: context canceled`，不是我们的。
- 审查：三位审查者（Opus）。一条 Important：空的文字块被拒，S2 的 `read` 读零行时正要发它（中高，B-I1）。中低一条：SDK 把读不了的参数四倍地回显进 -32602，任何方法、任何 PAT，36 MiB 的请求体答 144 MiB、分配约 2.6 GB（A-M1；除了这个入口的工具的调用，每个方法至多 64 KiB，`params` 的形状在 SDK 之前查）。其余是低（矩阵的 `notebook`、编码不了的 `Extra` 挂到期限、几处没测的路径与 `register` 的保留名的约定）与细节；A 对附加问题的回答：没有令牌的 401 带提示。修复核对第一轮：A-M1 只改对一部分，一个工具名的 `tools/call` 仍能经 `_meta` 里 float64 放不下的数让 SDK 4 倍地回显整个请求体（F1）、经 `jsonrpc` 与 `id` 2–3 倍（F2），各记中低；别的都改对了；C 的四条新的细节；修复二轮照改，并只让这个入口登记了的工具名过 64 KiB；第二轮：没有新的发现。逐条见[审查记录](reviews/P1-S1-platform-review.md)。
- 实测（实施者照 A 的测试，客户端与服务在同一个进程里，`/mcp` 的上限 37,814,272 字节，每种情形各在自己的进程里）：A 的情形之前答 144 MiB、分配 2,594 MB，之后 400、217 字节、113 MB；这一类的请求答 126–222 字节、分配 113–186 MB（原来 0.8–2.7 GB）；修复核对的 F1 之前 151,256,615 字节（4 倍）、2,666 MB，F2 的 `jsonrpc` 113,442,438 字节（3 倍）、`id` 75,628,667 字节（2 倍），之后都是 413、126 字节、150–186 MB；上限处合法的调用分配 1,381 MB、峰值 +298 MB（A 量的 +532 MB、1,417 MB，四个同时 +1,393 MB、5,670 MB）；`sizeError` 解码 `_meta`，大头在 `_meta` 里的合法调用多分配 54 MB（1,417 → 1,471 MB），峰值不变。
- 反向对照：实施之后 57 个都被抓到（单元 32、整个程序 18、`/register` 7）；审查者存活的：A 的 3 个与整个程序的 1 个、B 的 8 个（另有 `fit-zero-unmeasured-err`）、C 自己 22 个里的 10 个；修复一轮单元 99 个（94 个被抓到，2 个等价，3 个由整个程序抓到）、整个程序 20 个都被抓到；修复核对第一轮 A、C 的都被抓到（`e-bound-128k` 在单元之下存活、整个程序抓到）；修复二轮单元 26 个、整个程序 4 个都被抓到；修复核对第二轮 A 的 5 个都被抓到；测试的修复的 2 个都被抓到。
- 门禁：本机 `043185ba`、`da48b92a`、`d1d6e923` 上 `make lint-go gen-check knip`（`043185ba` 另有 `lint-web`）通过，`make test-go` 经 `heavy.sh` 888、873、864 秒（`da48b92a` 上第一次在 M8 的 `TestABackgroundRevertOfAnImportThroughServe` 上失败，负载 16，审查 C 判为负载下的摆动），`make e2e` 都是 251 个；修复一轮的每个提交单独 build、vet、lint 与快的测试。组合根两份 212.7、250.1 秒（+17.6%）移到 `^Test([A-D]|E([a-u]|very[A-Q]))` 之后 217.0、235.9 秒（+8.7%）。CI（作业，括号里是测试一步）：`043185ba`（run 39032853170）全绿，组合根一 7:30（7:10）、组合根二 6:17（6:02）、守卫 10:36（10:17）、e2e 4:44（3:54）；`da48b92a`（run 39044005262）全绿，组合根一 6:38（6:16）、组合根二 5:41（5:22）；`d1d6e923`（run 39052310544）全绿，竞态 4:30（4:13）、组合根一 6:13（5:51）、组合根二 7:38（7:20）、成本 8:04、守卫 9:01（8:40）、web 3:08、e2e 4:10、镜像 1:28；合并之后 run 39053755609（`384d9f96`）竞态的一份失败（上面的摆动），测试的修复的分支 run 39054910759 全绿，run 39056005809（`cfb48a50`）e2e 的 A4 失败（M1 的故事：事件流重连的刷新恰在第一次 `context.route` 打开拦截时发出，Playwright 对那个窗口里发出的请求不报事件；不是产品的缺陷，只改测试 `07ae13a0`，合并 `c0636ec3`），run 39063742977（`c0636ec3`）全绿。各次的作业时间见审查记录的门禁一节。
- 交给后面的：P2：超过 64 KiB 的请求体同时至多两个、码的表加行（第 8 节）；P5：人工清单的几步与没有令牌时的那一句（第 8 节）；M10：`prompts/get` 至多 64 KiB，prompt 的错误的分类与上限（第 8 节，00 号文档"写给后面的 M"）；M12：大的请求体的内存（[性能移交](../M12-release/handoffs/M4-performance.md)第 18 项）。
- 负责人可以改判的取舍（主持者的决定）：`POST /register` 答 OAuth 的错误（撤回时去掉它，接受 OAuth 客户端显示网页的 405）；没有令牌的 401 带同一句提示；超过 64 KiB 的只能是这个入口登记的工具的调用，`jsonrpc`、`id`、`_meta` 照 SDK 的读法；内存的界线给 P2；64 KiB 之内的回显至多约 4 × 64 KiB 接受。

### 9.2 S2：路径、`list` 与 `read`（2026-10-18，合并 `a36e92df`）

- 提交：实施：page 的路径、线性的 glob、版本令牌、`read` 的切与 `list` 的游标，存储按标题键找节点与读 `list` 的页的状态，`NodePaths`、`ListByPattern` 与 `ReadPath`，WebP 的宽高、asset 的 `AttachmentReads` 与 `server.public_url`，MCP 的工具 `list` 与 `read`，经 serve 的 `list` 与 `read`、附件的最后一跳，MC2（从 S1 的分支 `043185ba` 开始是 `ed9822fd`–`c2bf398c`，变基到 S1 修复一轮之后的 `da48b92a` 是 `0d4d2573`–`4fdadb47`，审查时在它上面；之后变基到 main 的 `384d9f96` 与 `cfb48a50`，`0e0ca836`–`56990e8c`）；审查的修复（在 `cfb48a50` 上 `4007513d`–`b9a6c7c0`，修复二轮之前变基到 `c0636ec3`）：路径的段由自己的唯一索引读 `647014d5`，`read` 先判定笔记本再找路径 `1a5914ca`，两个、三个字的段 `3d4ea5cc`，没有 `server.public_url` 时图片照样内联 `36384951`，`list` 的匹配 3 亿个单位、守卫有最长的键 `82f5b1da`，WebP 的 RIFF 的大小 `5e46873a`，`public_url` 的错误不引用密码 `0fa5a3d7`，几处细节（进了回收站的附件、提示、文字）`c44222e2`，按工具的码的表 `17491384`；修复核对第一轮之后的修复二轮：00042 去掉兄弟的索引 `58a0fab1`，带 `@` 的被拒的 `public_url` 不引用 `913e3eef`，别的版本的 VP8L 与别的大小的 VP8X 没有宽高 `9fd67a1e`，读字节之前走了的附件答它的路径 `988b3423`，码的表的每一行都答出 `970f72f2`，守卫那一份的时限 25 分钟 `78b7d2e9`。main 上实施的七个提交是 `3c91620d`、`8762c077`、`ee0e7a65`、`a1ffa3fa`、`4e2787e6`、`a1f49285`、`05335d97`。合并 `a36e92df`（第二个父提交 `78b7d2e9`）。
- 与设计不同之处已改在第 3 节（3.8–3.13、3.18、3.19）、第 4、5、6 节与 2.1 第 27–31 条，3.20 分实施时定下的与审查带来的列出；S3 的 `client_name` 的迁移随之是 00043。
- glob（3.10.1）：实施时照 4 亿，本机的门禁里守卫最慢的走查 0.68 秒（2.9 倍）；审查 A 加了键是名字两倍长的 `և` 的树之后 4 亿只有 2.25 倍，降为 3 亿（2.9–3.04 倍，每种取 3 次里最小）；门禁里最慢的 `և` 741 ms（2.7 倍）、710 ms（2.8 倍），修复核对第一轮 A 单独跑的单次走查 0.76–0.82 秒（2.45–2.6 倍）；续走的重进占 17%；守卫在 CI 的成本那一份里每次都过，每次走查的耗时还没读。
- TypeScript 客户端试用（6.4 的第 1 步，S1 的同样的门禁与 SDK）：服务是 `f9080d3c`（`c2bf398c` 的树，变基之前）的 `bin/nervewiki`，`NWIKI_SERVER__PUBLIC_URL=http://127.0.0.1:18765`、固定的端口 18765，自己的 PostgreSQL 容器；经 REST 建笔记本 `Plans`、页 `Guide`（BOM 与 CRLF）、`Setup`、`Long`（3,000 行 × 40 字节）、`Wide`（10 万个 `x`），附件 `diagram.png`、`spec.pdf`；客户端 `new Client({name: "trial-agent", version: "1.0.0"})` 与 `StreamableHTTPClientTransport(/mcp?notebook=<id>)`，令牌经 `requestInit.headers`。
  - 握手同 S1（2025-11-25、没有说明、没有会话，GET 的 405 只试一次、不报错）；`tools/list` 的两个工具与 `api/mcp/tools.json` 逐字相同，SDK 收下 `["null","string"]` 一类的类型。
  - `list`：每次一个 POST、200 `application/json`、一个文字块，`structuredContent` 是同一段 JSON；`list {}` 答 `{"items":[{"path":"Guide.md","kind":"page","has_children":true,"version":"01a14a4b-ad80-…:1","byte_size":47,"updated_at":"2026-10-17T14:37:16.031388Z"}],"next_cursor":null}`；`Guide/` 照树的次序、`**/*.md` 照路径的次序；`limit` 2 给 `next_cursor`、带它接着列；`list {"pattern":"Guide"}` 答空的 `items` 与 `"hint":"Guide.md is a page; list \"Guide/\" for its children."`。
  - `read`：`Guide.md` 两个文字块、没有 `structuredContent`，元数据 `{"path":"Guide.md","kind":"page","version":"…:1","offset":1,"column":0,"lines":5,"total_lines":5,"byte_size":47,"next_offset":null,"next_column":null}`，正文与存的字节相同（BOM 与每个 `\r` 经 SDK 不变）；`Long.md` 三次（1,190、1,190、620 行）、`Wide.md` 三次（`next_column` 48,791、97,582），连起来都等于正文；`diagram.png` 答文字块与 `image`（`image/png`、73 字节、等于上传的；`"link":"diagram.png"`、带 `d=1` 的绝对地址）；`spec.pdf` 答 `resource_link`（`mimeType` `application/pdf`、`size` 15）。
  - 错误的原文：`read {"path":"Guide"}` 答 `page.not_found: Nothing is at "Guide". A page's path ends with .md: "Guide.md".`；`read {"path":"Guide/Missing.md"}` 答 `page.not_found: Nothing is at "Guide/Missing.md".`；`list {"limit":0}` 答 `validation_failed: The request has invalid values. (limit: out_of_range: must be between 1 and 1000)`；`list {"limit":"two"}` 答 `bad_request: The arguments do not match the tool's input schema. (limit: invalid_format: has the wrong type or format)`；`read {}` 答 `bad_request: The arguments do not match the tool's input schema. (path: required: is required)`。SDK 自己不校验参数，错误都是带 `isError` 的结果，不是异常。
  - 对照 00 号文档 4.3、4.4：没有不符。服务端的日志：1 行"MCP initialize"与 20 行"MCP tool call"（各带工具、笔记本、用户、令牌、`client: unknown`、结果、1–3 ms）。审查 C 核对过：结论都出自记下的消息；它跑在变基之前的树上，S1 的修复一轮只改 S2 已经照做的块的规则；它没有读零行（SDK v1.8.0 的 `TextContent` 空的也写 `"text"`）。
- 审查：三位审查者（Opus）。四条 Important：路径的点查不是点查、计划测试抓不到设计点名的回退（中高，A-I1、C-I2；00042 的两个部分唯一索引，修复二轮去掉兄弟的索引）；多字的段没有测试（中高（测试），A-I2）；`read` 先找路径、后判定笔记本没有测试抓得到（中高（测试），C-I1）。中低 2 条：守卫没有最长的键（A-M2；3 亿）；被拒的 `public_url` 照录密码（B-M1）。其余是低与细节；审查之前的偏离 10 由主持者改为照样内联图片。修复核对第一轮：没有统计时 `ChildByKey` 在"只有文件夹"的形状上仍读这个键的全部节点（中低，A2-M1，接受）、B-M1 剩下两种照录（中低），几条细节，修复二轮照改；第二轮：没有新的发现。逐条见[审查记录](reviews/P1-S2-reads-review.md)。
- 反向对照：实施之后 42 个都被抓到（2 个第一次存活，加断言之后抓到）；审查者存活的：A 的 4 个（另 1 个等价）、B 的 7 个、C 自己 28 个里的 7 个；修复一轮 82 个都被抓到（5 个在补了测试之后），审查者存活的都重跑抓到（等价的除外）；修复核对第一轮 A、B、C 的都被抓到（B 的 `chunk-with-pad` 过：算上填充字节的更严的块的结尾，同样说得通，不改）；修复二轮 30 次运行都照应有的失败（`child-index-not-unique` 由兄弟撞名的测试与约束名的测试抓到）；修复核对第二轮 A、B 的都被抓到。
- 门禁：本机 `c2bf398c`、`4fdadb47`、`b9a6c7c0`、`78b7d2e9` 上 `make lint-go gen-check knip lint-web`（`c2bf398c` 另有 `test-web` 3,338 个测试与 `build-web`）通过，`make test-go` 经 `heavy.sh` 878、899、910、871 秒，变基到 `384d9f96` 之后 910 秒（`c2bf398c` 上第一次在 S1 的 `TestAnUnwrittenAnswerIsAProblem` 上失败；变基到 `384d9f96` 时第 4、5 次在 M8 的 `TestABackgroundRevertOfAnImportThroughServe` 与 `platform/jobs` 连不上 Docker 上失败，单独重跑都通过；`b9a6c7c0` 上第一次在 archtest 的 `TestSQLCSchemaScope` 上失败，`sqlc.yaml` 漏了 00042，改进 `4007513d`），`make e2e` 都是 252 个；组合根两份 224.3、239.9 秒（+7.0%），修复一轮之后 219.7、242.7 秒（+10.5%，照主持者的决定不动）。CI（作业，括号里是测试一步）：`c2bf398c`（run 39047015191）、`4fdadb47`（run 39050604095）、`b9a6c7c0`（run 39067326556）、`78b7d2e9`（run 39073889832）全绿，最后一次竞态 7:53（7:34）、组合根一 4:25（4:09）、组合根二 7:59（7:35）、成本 6:09（测试 4:10）、守卫 6:43（6:27，上一次 11:05）、web 4:05、e2e 3:31（2:48）、镜像 1:18；合并之后 run 39075360591（`a36e92df`）全绿。各次的作业时间见审查记录的门禁一节。
- 交给后面的：S3：`McpContent`、`McpTool`、`McpAnswer` 随 MC1 导出（4.1），`AgentsPage` 读根的索引，`client_name` 的迁移是 00043，它的工具在码的表里加行，`BOOTSTRAP_FIRST` 量了之后移一次；P2：`ParseVersionToken`、码的表（第 8 节）；P5：Claude Code 一侧的两条进人工清单（第 8 节）；M12：glob 的续走与很大的文件夹、守卫在 CI 上的耗时与 runner 的摆动、`ChildByKey` 没有统计时的计划（[性能移交](../M12-release/handoffs/M4-performance.md)第 18 项）；性能移交第 11 项与 README 的 WebP 随合并改了。
- 负责人可以改判的取舍（主持者的决定）：00042 的两个部分唯一索引与去掉兄弟的索引；`ChildByKey` 没有统计时的计划接受；不论有没有 `server.public_url` 都内联图片；WebP 只在头合得上文件、版本与块的大小对时给宽高，PNG 与 JPEG 坏了的正文照头内联；`MaxGlobWork` = 3 亿；守卫那一份的作业时限 25 分钟；`BOOTSTRAP_FIRST` 到 S3 再动；按工具的码的表。

### 9.3 S3：说明与账户（2026-10-18，合并 `c9468e5b`）

- 提交：实施：platform/markdown 的 `FrontmatterEnd` 与 page 的 Markdown 端口 `06980323`，`AGENTS` 页的说明、它的缓存与发它的握手 `6e909994`，客户端名称的清理与记在 PAT 上 `55b7e101`，账户入口的工具 `notebooks` 与 `open_notebook`（带第 5 步的 `tools.json`、矩阵的行与码的行）`dea706ae`，经 serve 的说明、客户端名称与账户的笔记本 `bdeb75c5`，MC1 `4cf22084`，组合根两份的界线移到 `TestEveryS` 之前 `19bb107b`（从 S2 的分支 `4fdadb47` 开始，先后变基到 S2 修复一轮的 `b9a6c7c0`、二轮的 `78b7d2e9` 与 main 的 `a36e92df`；较早的一次推送 `9f9092f4` 在修复一轮上）；审查的修复（接在 `19bb107b` 之后，没有再变基）：说明的缓存存副本 `ab466335`，握手的说明在 32,768 与 32,769 字节 `a4e76a44`，握手在调用的期限之内读说明 `173a6553`，说出的名称立刻是令牌的、到记下为止 `8d82f584`，`open_notebook` 给根下留 8,192 字节、不多不少 `f8a02f49`，矩阵比较 `open_notebook` 的内置说明与照实的注释 `f2a5096e`，MC1 读写过的笔记本的最后改动与附件 `c8542c8e`。合并 `c9468e5b`（第二个父提交 `c8542c8e`，在 S1、S2 的文档提交 `e06db48f` 之后）。之后主持者的只改测试的分支 `m9-job-waits`（`380b890c`，合并 `0c934bb9`）：等 River 的任务结束的五处（`endedJob`、`endedBackground`、`awaitImports`、`awaitExports`、`awaitBackground`，15 或 30 秒）共用一个 `jobWait` = 60 秒，竞态那一份的作业时限从 15 分钟提到 20 分钟。
- 与设计不同之处已改在第 3 节（3.14–3.17、3.19）、第 4、5、6 节与 2.1 第 32、33 条，3.20 分实施时定下的与审查带来的列出。
- TypeScript 客户端试用：S3 没有（S1、S2 的末尾各一次）；MC1 的第 5 步是官方 SDK 对 e2e 的服务：连上、读到说明、列出并调用 `list`、关掉，没有错误。
- 审查：三位审查者（Opus）。一条 Important：说明的缓存钉住每个 `AGENTS` 页的整个正文，"256 项至多 8 MiB"不成立（中高，A-I1：1 MiB 的页写 256 次留着 258.1 MiB，5 MiB 的页约 1.25 GiB；存副本）。中低一条：说出的名称没写下或写失败之后，这个会话里不报名称的调用记成行里的旧名称，两个客户端同时用一个 PAT 时进程与行也会停在不同的名称上（B-M1；照 B 的方向，说出的名称立刻记住、到知道在行里为止每次调用再记下）。低（测试）四条：握手处的 32,768（A-M1）、失败的记下被缓存（B-M2）、`open_notebook` 的余地只从上面钉住（C-M1）、重启之后说出已记下的名称（C-M2）。其余是细节与存疑；审查之前实施者另发现设计的 `AgentsPage` 在 ANALYZE 之后不是点查（偏离 22），与 S1 的握手一行的名称总是 `unknown`（偏离 3，原因由 A 更正：S1 没有名称）。修复核对第一轮：各条都改对了；新的只有细节：镜像的次序里两个客户端的名称在竞态之后仍不同（B 的 N-1：改设计的那一句，不加锁）、竞态那一份的 CI 时限离"约两倍"的规则最近（C：随 `m9-job-waits` 改）。逐条见[审查记录](reviews/P1-S3-account-review.md)。
- 实测：说明的缓存的堆（`TestTheInstructionsCacheHoldsItsTextsAlone`）：不复制 256.1 MiB（A 的探针 258.1 MiB），复制 8.1 MiB（三次与竞态下相同）。概况的守卫（10 万个节点、10 万个变更集，8 次的中位数，ANALYZE 前后）：本机树 57.9 与 55.5 ms、计数 6.3 与 5.6 ms、最后改动 5.7 与 4.3 ms，门禁里树 51–100 ms、计数 6–10 ms、最后改动 4–8 ms，约低 9 倍。`AgentsPage`（A 的数据：5,000 本笔记本、111,041 个节点）：LEFT JOIN 读 105,041 行、2,594 个缓冲、5 ms，子查询 8 + 4 个缓冲、0.01–0.02 ms。`FirstChildren` 在一个根下 10 万个节点时 10–18 ms（B）。`notebooks` 最贵的一项约 2.2 KiB（B）。
- 反向对照：实施之后 32 个都被抓到（3.19 与 6.3 的 S3 各行 24 个、计划测试 8 个）；审查者存活的：A 自己 15 个里的 2 个（A01、A13），B 的 10 个里的 2 个（M1、M2；M5 只在单元测试下存活、经 serve 抓到），C 自己 41 个 Go 的变异里的 4 个（C13、H11、C15、H4；另 1 个等价、2 个只由整个程序抓到）；修复一轮自己的 23 个都被抓到，审查者存活的 8 个照它们自己的命令重跑都被抓到，S3 的快的与名称的 17 个在修好的树上都被抓到；修复核对第一轮 A 的 20 个、B 的 9 个都失败，C 的各个对照都失败、快的 6 个与重的 4 个 ALL GOOD。
- 门禁：本机 `19bb107b`（树 `182bea5a`）上 `make lint-go gen-check knip lint-web`、`test-web`（166 个文件、3,338 个测试）与 `build-web` 通过，`make test-go` 经 `heavy.sh` 14:32（103 个包，竞态下组合根 478.5 秒），S2 修复一轮上 16:52、二轮上 16:06，`make e2e` 都是 253 个（S2 的 252 个加 MC1）；修复一轮 `c8542c8e`：每个提交的树单独 build、vet 与 `go test -short`，`make test-go` 第二次 15:05 通过（第一次在 M8 的 `TestABackgroundRevertOfAnImportThroughServe` 上失败：`endedJob` 等 1,501 页的导入 15 秒，负载 8.9；单独 10.99 秒通过），`make e2e` 253 个。组合根两份：移到 `TestEveryS` 之前之后 237.9 与 228.5 秒、main 上 234.0 与 230.5 秒，修复一轮两次的平均 248.1 与 244.4 秒（6.1）。CI（作业，括号里是测试一步）：`9f9092f4`（run 39072470833）、`19bb107b`（run 39077166122）、`c8542c8e`（run 39092430813）全绿，最后一次竞态 8:17（8:00）、组合根一 6:47（6:26）、组合根二 8:16（7:54）、成本 9:16（测试 6:30）、守卫 11:05（10:42）、web 2:42、e2e 3:40（2:55）、镜像 1:12；合并之后 run 39095183240（`c9468e5b`）全绿：竞态 8:22（7:59）、组合根一 4:59（4:39）、组合根二 5:52（5:34）、成本 9:02（测试 6:24）、守卫 10:45（10:26）、web 2:49、e2e 3:59（3:08）、镜像 1:18。M9/P1 里 `TestABackgroundRevertOfAnImportThroughServe` 在负载下超过 `endedJob` 的 15 秒不止一次（S1 修复一轮负载 16、S3 修复一轮负载 8.9；S2 变基时也失败过一次），`m9-job-waits` 的 60 秒随之（CI run 39095275235 全绿）。各次的作业时间见审查记录的门禁一节。
- 交给后面的：P2：码的表加行与 `TestTheMCPToolsErrorsCarryTheirCodes`、组合根第二份再量（第 8 节）；M10：P1 给它的各条写进新建的[移交](../M10-llm-wiki/handoffs/M9-P1-mcp-registry.md)（prompt 的注册表与最后一跳、虚拟的 `index.md` 与 `log.md`、`llm_wiki` 下 `AGENTS` 的保护、`prompts/get` 的上限与错误，第 8 节）；M12：`open_notebook` 的概况的读与 `FirstChildren`、`notebooks` 逐个工作区、说明的缓存与名称的 LRU 的内存、竞态那一份的时限（[性能移交](../M12-release/handoffs/M4-performance.md)第 18 项）；截在最后一个换行、只由看不见的字符组成的名称、客户端走了的 warn（[打磨移交](../M12-release/handoffs/M5-polish.md)第 30 条）。
- 负责人可以改判的取舍（主持者的决定）：说明的缓存存副本；客户端名称照 B 的方向（说出的立刻记住、到知道在行里为止再记下，热行的规则保住）；两个客户端的镜像的交错只改设计的那一句、不加按令牌的锁；"最后一个换行"的截法不改；只由看不见的字符组成的名称照 shared 的名称规则；客户端走了的 warn 不改；组合根两份的界线不再动，判断取两次的平均；MC1 不加非管理员的角色；任务的等待与竞态那一份的时限在只改测试的分支里改。

## 附：查过的代码

行号是 `main` 的 `e16a98f7`。服务端的从 `server/internal/` 起，模块在 `modules/` 下（`page/…` 指 `server/internal/modules/page/…`）；SDK 是 `~/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.8.0/`。"实验"指 `scratchpad/m9/p1/exp/`（离线、只用模块缓存：`main.go` 是两个入口、接收的中间件、恢复、`no-store` 与内容块的形状；`tag/main.go` 是 jsonschema-go 的推导；修订时加的：`measure/` 是 `encoding/json` 的转义表与两次转义，`wire/` 是 SDK 在线上怎么写文字块与说明，`glob/` 是位并行的 glob、按树走、工作的上限与 RE2 的对照，`tidy/` 是仓库副本上离线的 `go mod tidy`），"审查"指 M9 的 [设计审查](reviews/M9-design-review.md) 的实测与 `scratchpad/m9/review-A/exp*`，"P1 的审查"指 [P1 设计审查](reviews/P1-design-review.md) 与它的实验 `scratchpad/m9/p1/review-A/`（`server/internal/platform/httpserver/mcpexp_test.go` 经真的链与 SDK v1.8.0）、`scratchpad/m9/p1/review-B/`（`server/` 是本文的 SQL 经 sqlc 生成，`glob/` 是草稿的匹配的计时）。

| 决定 | 依据 |
|---|---|
| 00 号文档的修订与要落实的范围（2.1） | `docs/v0.1/M9-mcp/00-M9-design.md:35`（第 2 节 workspace）、`:73`（`ProblemOf(err)`）、`:81`（`Summaries`）、`:83`（清单的检查）、`:127`（答复头）、`:142`（清理的次序）、`:148-149`（入口、"每次调用都先过一次 Gate"）、`:153`（说明的文字）、`:166`、`:170`（参数不合 schema 的 JSON 指针）、`:185-187`（段的检查、点查）、`:215-217`（`read`）、`:334-358`（4.14）、`:362-366`（4.15 的次序）、`:426`（P1 一行）、`:506`（第 8 节 workspace）、`:528`（矩阵）、`:618-656`（第 11 节） |
| `Summaries` 不需要（2.1 第 1 条，3.17.1） | `workspace/adapter/postgres/queries/workspaces.sql:12-20`（`ListWorkspacesOf` 读 `w.slug`、`w.name`）；`workspace/memberships.go:29-32`（`Membership` 只有 id 与角色）、`:49-59`（丢掉 `l.Workspace` 的别的字段）；`workspace/workspaces.go:19-33`；`notebook/app/ports.go:23-31` |
| `CleanClientName` 的次序（2.1 第 2 条，3.16.1） | `shared/name.go:34-36`（`unshowable`：Cc、Zl、Zp、Bidi_Control；U+200E 是 Bidi_Control 而 ccc 为 0，挡住组合）；`shared/title.go:29`（`CheckTitle` 先 `TrimSpace` 再 NFC）；x/text 的 `norm.NFC.LastBoundary` 与 `PropertiesString(…).BoundaryBefore()`；复核 A 的 `review-A/server/internal/shared/cleanclient_exp4_test.go`（第一轮的截法留下 129 个）、`cleanclient_exp5_test.go`（现在的规则的核对） |
| 参数的检查用 bodyshape（2.1 第 3 条，3.3.3） | `platform/httpserver/bodyshape/bodyshape.go:37-86`（类型、`Closed`、`Node`、`Table`）、`:100-117`（`FieldError` 的消息，不带取值）、`:170-191`（`Check`：空的不说话）、`:255-337`（16 个问题、256 字节的路径、按名称的次序）；`jsonschema-go/validate.go:126-130`（"type: %v has type %q"，把实例写进消息）；实验 `tag/main.go`（推导：可选与必填的切片都是 `["null", "array"]`、说明来自标签、`additionalProperties: false`） |
| 账户入口的判定由用例做（2.1 第 4 条） | 00 号文档 `:149` 与 `:362-366` 的次序；`page/app/list_nodes.go:24-47`（先 `WorkspaceOf` 再判定）；`access/domain/rules.go:92-93`、`:129`（`node.list`、`page.read`、`asset.read` 都是读者） |
| `ProblemOf` 带 ctx（2.1 第 5 条，3.3.6） | `platform/httpserver/apierrors.go:202-242`（`Write` 的四支看 `r.Context()`）、`:254-299`（`problemOf`）、`:18-77`（可选的接口）、`:81`（499） |
| e2e 的客户端随第一个故事（2.1 第 9 条） | `e2e/package.json:10-18`；`knip.jsonc`；`pnpm-workspace.yaml`（门禁三项） |
| 矩阵的 PAT 经接口建（2.1 第 10 条，3.6） | `bootstrap/contract_test.go:278-307`（`createToken`、`TestEveryOperationAcceptsAPersonalAccessToken`）；`bootstrap/permission_matrix_test.go:76-118`（列）；`identity/app/authenticate.go:93-113`（PAT 的格式与哈希只在 identity） |
| 固定头的前缀（2.1 第 11 条，3.2.3） | `platform/httpserver/middleware.go:25`（`apiPrefix`）、`:29-31`（平台链）、`:45-52`（`setFixedHeaders`）；`workspace/domain/reserved_slugs.txt:29`（只保留 `mcp`） |
| 段的检查的码（2.1 第 12 条，3.9.1） | `shared/title.go:23-43`（`CheckTitle` 的码：required、too_long、invalid_format、not_allowed）、`:64`（禁的字符）；`shared/title_key.go:13-18` |
| 点查是两条语句（2.1 第 13 条，3.9.2） | `migrations/sql/00012_page_nodes.sql:32-33`（兄弟的唯一索引，`NULLS NOT DISTINCT`）、`:36`（`(notebook_id, parent_id)`）；`00019_page_nodes_name_key.sql:5`；`page/adapter/postgres/queries/nodes.sql:37-44`（`Children` 的 `IS NOT DISTINCT FROM`）、`:74-80`（递归的语句的计划，M6 收尾 FA-M1、FA4-M1） |
| 内置的说明只写英文（2.2，3.15.4） | `docs/v0.1/v0.1-design.md:1240`（13.5 第 4 条）；`web/apps/web/src/i18n/`（网页按浏览器的中英文）；`sdk/mcp/protocol.go:1037-1047`（`InitializeParams`：`_meta`、能力、`clientInfo`、版本，没有区域设置） |
| `API.Wrap` 与边缘的次序（3.2） | `platform/httpserver/api.go:165-187`（`Middlewares` 倒序）、`:225-238`（`BodyLimits` 的路由另加 `read_timeout`）、`:243-248`、`:263-297`（认证与闸门）、`:301-315`（401）；`limit.go:24-36`；`bodyshape/middleware.go:17-37`（按 `r.Pattern` 找根、读完整个请求体）；`routes.go:40-77`（`/api/` 的 404 problem、`Handle`）；`problem.go:15-27`（平台的码）；`bootstrap/wire.go:126-173`（构建次序、`apiConfig` 的 `BodyLimits`、模块注册、`/` 最后）；`page/adapter/http/limits.go:5-17`；`page/domain/content.go:10-24` |
| 事件流是挂在契约之外的先例 | `events/module.go:86-90`；`platform/httpserver/longlived.go:37-76` |
| 无状态的 handler 与 `getServer`（3.2.2、3.3.1） | `sdk/mcp/streamable.go:128-220`（选项）、`:224`（4 MiB）、`:316-371`（`ServeHTTP`：回环保护、`MaxBytesReader`、带版本头时调 `getServer`）、`:373-447`（`serveStateless`：405 `Allow: POST`、415、Accept 的 400、`getServer` 为 nil 答 400、`connectStreamable(req.Context(), …)`）、`:504-514`（合成的 `InitializeParams` 没有 clientInfo）；审查 A（GET 405、listen 立刻答完、批并发执行） |
| 协议版本、能力与缓存（3.3.1） | `sdk/mcp/server.go:72-213`（`ServerOptions`）、`:657-677`（有工具时默认 `ListChanged: true`）、`:2078-2107`（`initialize` 的结果：协商的版本、能力、`s.opts.Instructions`）；`sdk/mcp/protocol.go:1161-1194`（`Cacheable`）；审查 A（2026-07-28 被协商降到 2025-11-25，`cacheScope` 的默认 `public`）；`review-A/exp2/main.go` |
| 低层的 `AddTool` 与列出（3.3.2） | `sdk/mcp/server.go:315-368`（低层不校验参数、`type` 要是 object）、`:370-488`（泛型的丢掉错误的结果、写 `structuredContent`、`json.Marshal` 在 `:455`）、`:979-996`（`listTools`）、`:1005-1025`（`callTool`，不认识的 -32602）、`:36-37`（`DefaultPageSize` 1000）；`sdk/mcp/features.go:40-44`（同名替换）、`:76-81`（按名称排）；`sdk/mcp/protocol.go:1895-1947`（`Tool`）、`:1964-2010`（注解与 `MarshalJSON`）；`sdk/mcp/content.go:28`、`:57-78`、`:128-153`；实验（`resource_link`、`image` 的线上形状、`<>&` 在信封里转义） |
| 接收的中间件包住每个方法、可改说明（3.3.1、3.15.2） | `sdk/mcp/server.go:1859-1863`（`AddReceivingMiddleware`，从右往左）、`:1942-2045`（`ss.handle`：除 `initialize` 外的调用 `Async`，`:2003-2005`；`handleReceive`）；`sdk/mcp/shared.go:133-137`（`addMiddleware`）、`:206-225`（`handleReceive` 经中间件链）；实验（两个入口各自的说明、`initialize` 也经过中间件） |
| 调用的 ctx（3.4.1） | `sdk/internal/jsonrpc2/conn.go:212-245`（不 `PropagateCancellation` 时包上 `notDone`）、`:566`（每个请求的 ctx）、`:813-821`（`notDone`：值委托、没有期限与取消）；`sdk/mcp/streamable.go:210-219`（`PropagateRequestCancellation` 只对 2026-07-28）；实验（中间件里 `Deadline()` 没有、`Value` 读到 `ServeHTTP` 放的；派生之后工具看到 2 秒的期限）；`bootstrap/app.go:96-103`（停机的次序） |
| 恢复与"不同时返回结果与错误"（3.4.2） | `sdk/internal/jsonrpc2/conn.go:656-688`（`handleAsync` 在自己的 goroutine 里）、`:694-701`（两者都有时 `%#v` 写出结果）；`sdk/mcp/transport.go:233`（`OnInternalError` 记 error）；`platform/httpserver/middleware.go:97-136`（`withRecover`）；审查 A（panic 让进程以 2 退出）；实验（中间件里恢复之后答 `internal_error`，下一次调用照常） |
| `no-store` 的写者（3.4.3） | `sdk/mcp/streamable.go:1356`、`:1686`（`no-cache, no-transform`）；审查 A（只有 202、400、405 留住）；实验（包上写者之后 200、202、400 都是 `no-store`） |
| 答复的上限与量（3.4.4、3.3.4） | 00 号文档 4.7；实验 `measure/`（go1.27.1：`"` `\` 与五个短的转义 2 字节，别的 C0、U+2028、U+2029 6 字节，U+007F 1，C1 2，不是 UTF-8 的 3；`json.Marshal` 另把 `<>&` 写成 6 字节；成员里两次转义的 4、3、7、6），同 P1 的审查 A-M4 的表；实验 `wire/`（文字块的 `text` 里 `<>&` 是 `\u003c` 一类，`initialize` 的 `instructions` 里原样）；`sdk/mcp/content.go:47`（`TextContent.MarshalJSON` 用 `json.Marshal`）；`sdk/internal/jsonrpc2/messages.go:165-166`、`:236-244`（信封不带 HTML 转义） |
| 日志与 SDK 的 logger（3.4.5） | `sdk/mcp/server.go:1394`、`:1414`、`:1498`（每个无状态的 POST 的 info）、`:1990`、`:2095`（error 级的客户端错）；`platform/httpserver/middleware.go:151-181`（`loggedPath` 不记查询）、`:212-240`（访问日志）；`docs/v0.1/v0.1-design.md` 13.1 第 10 条 |
| 名称的来源（3.16.4） | `sdk/mcp/shared.go:562-600`（`_meta` 的 `io.modelcontextprotocol/clientInfo`）、`:693-708`（`ClientInfo()`）；`sdk/mcp/server.go:1895-1910`（`initialize` 的参数）；实验（`initialize` 的 `Params.ClientInfo` 与 `ClientInfo()` 都读到；握手之后的调用 nil；`_meta` 的调用读到 `meta-client`） |
| `api_tokens.client_name` 与记下（3.16.2–3.16.3） | `migrations/sql/00004_identity_api_tokens.sql:5-26`；`identity/adapter/postgres/queries/api_tokens.sql:16-26`（`TouchAPIToken` 的 `SKIP LOCKED`）；`identity/app/authenticate.go:101-107`（尽力而为、记 warn）；`migrations/schema_test.go:137-145`（约束表）、`:496-498`（违反的情形）；`deploy/runtime-grants.sql:18`（`api_tokens` 的 DML 已有）；`shared/actor.go:15-22`（`APITokenID` 是值，不是指针） |
| 入口的判定（3.2.4、3.3.5） | `page/app/list_nodes.go:29-43`；`page/app/readable.go:13-37`；`page/domain/errors.go`（`ErrNotebookNotFound`） |
| 保留名单与例外（3.1） | `workspace/domain/reserved_slugs.txt:9`、`:19-23`、`:29`；`bootstrap/reserved_test.go:18-36`；`web/apps/web/src/app/reserved-slugs.test.ts`（只查 `[app]`） |
| 清单的检查（3.5） | `Makefile:127-136`（`gen-check` 不跑 Go 测试）、`:198-207`（`test-go-bootstrap` 的两份）；`bootstrap/contract_test.go:27-50`（由契约推导的整个程序的测试的写法） |
| 矩阵（3.6、3.13、3.17.4） | `bootstrap/permission_matrix_test.go:60-136`；`permission_matrix_coverage_test.go:21`、`:152-160`；`permission_matrix_notebook_test.go:154-166`；`permission_matrix_page_test.go:643-649`；`permission_matrix_seeded_test.go:202-213`、`:681-683`（种下的附件没有文件） |
| archtest（3.7） | `archtest/rules_test.go:35-51`（规则表）、`:100-113`（子包算平台包本身）、`:313-320`、`:326-336`（`riverStaysInJobs`）；`archtest/rules_cases_test.go`（合成的边）；`archtest/binary_test.go:14-21`（禁单）；`go-sdk@v1.8.0/go.mod:5-20`；SDK 的源码里没有 `google/uuid` 的导入（`grep`）；`server/go.mod:3-5`、`:57`（`google/uuid` 只是间接的） |
| 测试的接缝（3.4.9） | `bootstrap/wire.go:46-52`（`wireApp` 的 `batch`）；`bootstrap/interleavings_workspace_test.go:78`；`bootstrap/app_test.go:52-98`（测试的配置：请求 4 秒） |
| 路径的解析与给出（3.9） | `page/domain/node.go`（`KindPage`、`KindAsset`、`MaxDepth` 10）；`page/domain/name.go:15-27`（附件的名称不以 `.md` 结尾）；`page/domain/tree.go:12`（`PreOrder`）；`linking/app/access.go:25-38`（linking 的 `PageTree.Paths`，P4 改接 `NodePaths`）；`linking/domain/target.go:47-121`、`linking/domain/resolve.go:105-149`（链接的规则，MCP 不经它们） |
| 版本令牌（3.9.3） | 00 号文档 4.10（`:226`）；`page/app/unit_create.go:67`（版本号每个节点从 1 数） |
| glob 的匹配与工作的上限（3.10.1） | 实验 `glob/`（`glob.go` 每段的 Shift-And 与段上的自动机、按键的字节计的单位与不是 ASCII 的字符的筛子、`tree.go` 按路径键的次序走树、进入父节点时每项 256 个单位与接着走的跳过、`round2_test.go` 的 `a`、`é`、`漢` 三棵守卫树与 `TestResumeBoundsEachCall`、`sortcost_test.go` 的排序的代价、`main.go` 与正则的翻译的差分 30 万对与四种树、`adv_test.go` 的守卫树与上限、`re2_test.go` 的 RE2 每条路径 1.5–1.8 ms）；P1 的审查 B-I5（`review-B/glob/`：草稿的匹配在 B 的形状上 12.1 秒与 3.2–5.9 秒）；复核 B 的 `review-B/check/glob/`（`check_test.go` 的走查对照与接着走、`cost2_test.go`、`cost3_test.go` 的不是 ASCII 的键） |
| glob 与 `list`（3.10） | 00 号文档 4.8（代价与审查的 4–8 秒，本文改为线性与工作的上限）；`page/adapter/postgres/queries/nodes.sql:30-35`（`ListNodes`）；`page/app/ports.go:389-400`（`AttachmentInfos`）；`shared/cursor.go:21-84`（`EncodeCursor`、`DecodeCursor` 的重编码、`InvalidCursor`）；`migrations/sql/00012_page_nodes.sql:16-18`（`sort_order` 没有 NaN 与无穷）；[M12 的性能移交](../M12-release/handoffs/M4-performance.md)第 48 行（10 万个节点读树 62–71 ms） |
| `PageStates`（3.10.3） | `migrations/sql/00013_page_page_contents.sql:5-21`（主键、`revision`、`byte_size`、`updated_at`、`deleted_at`）；`page/adapter/postgres/queries/contents.sql:6-16`（`ContentMeta`、`PageContent`）、`:34-38`（`ContentSizes` 没有版本号）；`page/adapter/postgres/store.go:42-45`（`planned`） |
| `read` 的页（3.11.1） | `page/app/get_page_content.go:24-38`；`page/domain/content.go:10-24`（原样、5 MiB、UTF-8 与 NUL）；`page/app/ports.go`（`PageContent{Content, Revision, Hash}`） |
| `read` 的附件（3.11.2） | `asset/app/reads.go:57-91`（`Get`）、`:174-187`（`decide`：`asset.read`，看不到的答 `notFound`）；`asset/app/upload.go:140-145`（`Asset`）；`asset/app/sign.go:8-12`（`Signed`）；`asset/adapter/mac/sign.go:34-37`（一到两小时）；`asset/adapter/http/upload.go:175-194`（`ContentURL` 是相对的）；`asset/blobs.go:124-134`（`Open`、`ErrNoFile`）；`asset/app/content.go:67-76`（文件丢了记 warn）；`asset/domain/blob.go:12-23`（宽高，0 是没读出）；`migrations/sql/00027_asset_asset_blobs.sql:8-25`；`asset/module.go:69-118`（`Deps`、`New`）；`asset/adapter/sniff/sniff.go:28-33`（宽高由嗅探读出） |
| `server.public_url`（3.12） | `platform/config/config.go:42-56`（`ServerConfig`）、`:298-316`（`LogValue`）；`platform/config/validate.go:19-70`（`server.*` 的校验的写法）；`configs/config.yaml:5-26`；`platform/httpserver/clientip.go:26-81`（不读 `Host` 与转发头） |
| NUL 与代理项的推导（3.13） | `bootstrap/contract_test.go:247-272`（`TestFreeTextParametersDoNotAnswer5xx`）；`platform/httpserver/bodyshape/ambiguity.go:25-157`（重复的成员、不成对的代理项） |
| `AGENTS` 与说明（3.15） | `platform/markdown/frontmatter.go:5`（BOM）、`:66-97`（`frontmatterSpan`）、`:100-108`（`frontmatterOf` 答结束的位置）；`page/app/ports.go:532-546`（`Markdown` 端口）；`migrations/sql/00012_page_nodes.sql:32-33`；`page/adapter/postgres/queries/contents.sql:12-16`（正文与版本号一条语句） |
| `notebooks`（3.17.2） | `notebook/app/list_notebooks.go:27-49`；`notebook/adapter/postgres/queries/notebooks.sql:42-55`（按名称、id 排，`member_count` 子查询）；`shared/notebook_role.go:11-13`；`shared/authorize.go:15-17`（访客是工作区的角色）；`access/domain/rules.go:73`（`notebook.list` 每种角色）；`events/app/open_stream.go:51-58`（按工作区的组合）；`bootstrap/notebook_visibility_test.go`（"列表等于逐项读"的写法） |
| `open_notebook`（3.17.3） | `migrations/sql/00037_page_changesets_log.sql:9-11`（日志的索引不是部分的）；`migrations/sql/00014_page_changesets.sql:18-23`（`updated_at NOT NULL`、`deleted_at`）；`migrations/sql/00019_page_nodes_name_key.sql:5`（活的节点的部分索引）；`page/adapter/postgres/queries/activity.sql:1-10`（网页的活动：`max(cs.updated_at)`、`GROUP BY`，没有变更集的笔记本不在答复里）；P1 的审查 B-I2（`review-B/server/`：草稿的标量子查询生成为 `time.Time`）；`migrations/sql/00012_page_nodes.sql:36`、`page/adapter/postgres/queries/nodes.sql:47-50`（`IS NOT DISTINCT FROM` 不是索引条件） |
| WebP 的宽高（3.11.2） | `asset/domain/types.go:31`（类型表有 `image/webp`）、`:107-111`（`HasSize` 只有 PNG、JPEG、GIF）；`asset/adapter/sniff/sniff.go`（只导入三个解码器，`Size` 是 1 MiB 之内的 `image.DecodeConfig`）；`asset/app/blobs.go:82`（只在 `HasSize` 时读）、`:97-110`（只留 1–65,535 的边）；`api/modules/asset.yaml:403-407`；`README.md:143`；`docs/v0.1/M7-assets-transfer/02-P2-assets-server.md:206`；`server/go.mod` 与模块的缓存里没有 `golang.org/x/image`；RFC 9649（WebP 的容器与三种块） |
| 依赖的版本（3.1） | 实验 `tidy/`（仓库的副本加一个导入 `mcp`、`jsonrpc`、`jsonschema` 的包，`go mod tidy` 离线：`go.mod` 多七行，`asm` v1.2.1）；`github.com/pressly/goose/v3@v3.28.0/go.mod`（要 `segmentio/asm v1.2.1`）；P1 的审查 A-N6 |
| knip 与导出（4.1） | `knip.jsonc`（没有 `ignoreExportsUsedInFile`）；`Makefile` 的 `knip`（`--treat-config-hints-as-errors`）；P1 的审查 B-M9（`node_modules/knip/dist/typescript/visitors/walk.js:60-72`）；`e2e/fixtures/events.ts`（`BrowserStreams` 在签名里的类型的先例） |
| P1 的审查的实验 | A：`review-A/server/internal/platform/httpserver/mcpexp_test.go`（边缘的次序、批、listen、通知、断开、答复没写、panic；复核加的 `TestXUnwrittenOnGone`：断开时答复没写、`Err()` 是 `Canceled`）、`internal/shared/cleanclient_exp*_test.go`（两种次序与截法）、`review-A/jsonesc/main.go`（转义、`url.Query`、`slog` 的过滤）；B：`review-B/server/`（sqlc 生成的类型）、`review-B/glob/`（草稿的匹配的计时） |
| `page.New` 交空就 panic 的写法（3.17.1） | `page/module.go:170-179` |
| e2e（第 4 节） | `e2e/fixtures/server.ts:96-149`（`127.0.0.1:0`、`nervewikiEnv`）；`e2e/fixtures/events.ts:46-48`（`fetch`）；`e2e/fixtures/auth.ts:58-85`（`bearer`、`createToken`）；`e2e/fixtures/assets.ts:69-71`（`download`）、`:83`（`pngBytes`）；`e2e/fixtures/pages.ts:75`（`listNodes`）；`e2e/fixtures/test.ts:111-190`（fixture）；`e2e/fixtures/assert/identity.ts`；`e2e/stories/asset/as1-upload.spec.ts`（故事的写法、`newTeam`） |
| 移交（第 8 节） | `docs/v0.1/M9-mcp/handoffs/M0-P1-mcp-sessions.md`、`M4-P1-client-check.md`、`M5-locks.md`、`M7-assets.md`（第 1–3 项：附件的读、基地址、路径）；`docs/v0.1/M12-release/handoffs/M4-performance.md`（第 17 项之后接第 18 项）；`docs/v0.1/M10-llm-wiki/handoffs/`（没有 M9 的移交） |
