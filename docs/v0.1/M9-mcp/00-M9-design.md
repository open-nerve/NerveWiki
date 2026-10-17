# M9 MCP：总设计

| 项 | 内容 |
|---|---|
| 里程碑 | M9 MCP（`M9-mcp`） |
| 日期 | 2026-10-17 |
| 状态 | 设计中（初稿，待设计审查） |
| 依赖 | M6、M7、M8 |
| 上级文档 | [v0.1 总体设计](../v0.1-design.md) 第 3.8、3.9、4.4、4.5、5.3、5.5、5.7、6.1–6.4、10.1、11、12.2–12.6、13、14 节 |
| 代码位置的写法 | 模块写作 `page/app/unit.go:168`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…`、`shared/…`、`archtest/…` 在 `server/internal/` 之下；迁移写作 `00014_page_changesets.sql:13`（`server/migrations/sql/`）；SDK 写作 `sdk/mcp/streamable.go:400`（`~/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.8.0/`） |

---

## 1. 目标

1. **接入**：Claude Code、Codex 一类的 agent 以 PAT 经 `/mcp`（整个账户）或 `/mcp?notebook=<id>`（一本笔记本）连上同一个二进制。传输是无状态的 Streamable HTTP、答复是 JSON：服务重启、多次连接都不需要会话；每次工具调用在它自己的 HTTP 请求里，带这个请求的执行者、期限与取消（4.3）。
2. **说明**：笔记本入口握手时下发这本笔记本根下的 `AGENTS` 页（去掉 frontmatter，约 32 KB 截断），没有时下发内置的简短说明；`AGENTS` 页被替换、改名、恢复之后，下一次握手就是新的（4.6）。
3. **工具**：照 coding agent 熟悉的文件工具设计（总体设计 6.3）：`list`、`read`、`write`、`edit`、`move`、`delete`、`batch`、`grep`、`search`、`backlinks`、`index`、`log`、`changeset`、`versions`，账户入口另有 `notebooks`、`open_notebook`。每个工具调用与 REST 相同的用例，经同一个写入单元、守卫、变更集与事件；路径从笔记本根精确寻址（4.8）。
4. **归属**：MCP 的写的变更集记 `mcp:<客户端名>`，名称来自 MCP 的 clientInfo；网页的历史照 M8 按它筛选、显示为"MCP（名）"（4.5）。
5. **batch 与整组撤销**：一个 `batch` 是一个写入单元、一个变更集，带说明；任何一步被拒，整个回滚。batch 的变更集能在网页上整组撤销：撤销按依赖多轮应用、必要时经临时名称（M8/P3 的移交），满足总体设计 5.3"失败的 ingest 可以整组撤销"（4.11、4.12）。
6. **锁**：MCP 的写是一次性的，不开编辑会话；正被网页编辑的页（包括调用者本人自己的编辑器）拒绝 MCP 的写，答复说出是谁在编辑；agent 不能接管（4.10）。
7. **接到后面的 M**：MCP 的工具与 prompt 注册表（总体设计 12.4），M10 的 `lint`、`pending_sources` 与 prompts、M11 的升级工具在它上面登记；`index` 的查询供 M10 的虚拟 `index.md` 复用（第 8 节）。

## 2. 范围

**做**：

- 平台：`platform/httpserver/mcpserver`（新）：无状态的 SDK handler、两种入口、PAT、按调用的 ctx、工具的登记、结果与错误、日志、握手时的说明；`/mcp` 经逐路由的中间件挂上根路由器；`server.public_url`；`mcp` 从工作区 slug 保留名单的 `[reserved]` 移到 `[server]`；SDK v1.8.0 与 archtest 的一条规则。
- page 模块：按路径找节点与给出路径；`AGENTS` 与说明；工具 `list`、`read`、`write`、`edit`、`move`、`delete`、`batch`、`grep`、`search`、`log`、`changeset`、`versions`、`open_notebook` 与它们的新用例；一条语句的移动加改名（`Unit.Relocate`）；`batch` 的单元（计划、合成、推后的链接改写、界线）；整组撤销的多轮应用、临时名称与后台撤销的切批；编辑锁对 MCP 删除的规则；客户端名称的检查改为不随区域设置；变更集记下有没有改名、移动；迁移（4.2）。
- linking：`backlinks`（带路径）与 `index`（新查询：按类型、标签、父页面，带入链数，分页）两个工具。
- asset：给 page 的 `read` 的附件读（判定、元数据、字节、签名地址），不登记工具。
- notebook：`notebooks`：调用者在各个工作区里看得到的笔记本（新的读）。
- identity：PAT 记下客户端最近一次握手时报的名称（4.5）。
- 测试：工具清单、每个工具的权限矩阵、PAT、NUL 与不是 UTF-8 的参数、最后一跳、锁与交错、撤销的性质测试加 `batch`；e2e 的脚本化 MCP 客户端，本 M 的故事与前面 M 的故事的 MCP 版本（第 9 节）。
- 文档：Claude Code、Codex 的接入（README），反向代理的说明；真实客户端的人工清单（`manual/`）。

**不做**：

- agent 的历史写：还原版本、从回收站恢复、整组撤销只在网页（与 REST）由人做；MCP 不开后台任务，`transfer_jobs.client` 仍只许 `web`、`api`（第 11 节第 1 条）。
- 编辑会话、接管、强制解锁（第 11 节第 2 条）；上传附件、导入导出。
- 有状态的 SDK 会话、GET 的 SSE 流、服务端发给客户端的请求（sampling、elicitation、roots）、resources；prompts 的内容（M10）；OAuth 与会话的访问令牌（4.4）。
- 跨笔记本、跨工作区的 `grep`、`search`：一次一本笔记本；`grep` 不搜名称（`list` 的 glob 做）、不搜附件的内容。
- `backlinks` 给附件（今天的反链只对页，`page/link_targets.go:210-222`）；`index` 按任意属性的值筛选（只按 `type`、标签、父页面）。
- 网页上显示 MCP 的地址与接入的帮助：v0.1 收尾之后的打磨（第 11 节第 4 条）。
- 按笔记本限定的 PAT（v0.2，总体设计 6.3）。

## 3. 完成标准

总体设计 12.5 的通用标准之外：

1. **接入与说明**：两种入口经 PAT 连得上；会话的访问令牌答 403，没有、无效、已撤销的令牌答 401；笔记本入口的 `notebook` 不是 UUID 或看不到时答 404 `notebook.not_found`，在 SDK 之前；`initialize` 的说明是 `AGENTS` 页去掉 frontmatter 的正文（32 KiB 处截断并说明），没有时是内置的说明；`AGENTS` 页被替换、改名进出、删除、从回收站恢复、还原版本之后的下一次握手给出那时的说明（G16）。服务重启之后同一个客户端的下一次调用照常成功，不需要重新握手（整个程序的测试）。
2. **工具清单**：两个入口的 `tools/list` 等于入库的 `api/mcp/tools.json`（名称、说明、输入 schema、注解）；每个工具在权限矩阵里有行；工具的结果与错误照 4.7。
3. **路径**：4.8 的规则与四种定下的情形各有测试；glob 的三种通配与被拒的写法各有测试。
4. **读**：`list`、`read`（行的 offset 与 limit、48 KiB 的上限、换行与 BOM 原样、附件的图片内容与地址）、`versions`、`log`、`changeset`、`backlinks`（带路径）、`index`（按类型、标签、父页面，入链数）、`search`（照 M8，限一本笔记本）各有表格测试，码照 4.15 的次序。
5. **写**：`write` 一个单元补建缺失的父页并建页、拒绝已有的名称、带 `base_revision` 覆盖；`edit` 照 Claude Code 的 Edit（唯一、`replace_all`、CRLF 的规则、版本号的比较与一次重试）；`move` 移动加改名是一步、默认改写链接、`update_links=false` 不改写；`delete` 有子节点（附件也算）时要 `recursive`。每个写的变更集记 `mcp:<名>`（最后一跳，适配器不交客户端时测试失败）。
6. **锁**：被网页编辑的页（别人的、本人别处的）拒绝 MCP 的正文写与删除，答 `page.locked` 带持锁人与"是你"；改名、移动的链接改写碰上被编辑的页答 `linking.pages_locked`；MCP 不结束任何编辑会话；`batch` 里任何一步被锁，整个回滚，答复带这一步的序号。
7. **batch**：一个单元、一个变更集、必有说明；4.11 的约束与界线各有测试，答 422 时什么都没写；同一单元里两次改名、移动，各自指向它们的链接都跟着改（M6/P4 的移交第 1 条）；写完立即搜得到。
8. **整组撤销**：batch 的变更集在网页上整组撤销：互换名称、改名之后新建同名、删除之后新建同名、移入新建的文件夹、移动加改名的中间态、长名称与附件的临时名称、先写正文再删除同一页，都在真的数据库上测；撤销的性质测试的操作序列加 `batch`，从新到旧连续撤销 k 个之后树、正文与索引等于之前；后台撤销 batch 的大变更集，名称的环与"恢复再写回"不跨批（M8/P4 的移交第 2 条）。
9. **grep**：从正则取键、候选只读这本笔记本的键（块数的测试，13.4 第 12 条）、Go 的 RE2 按行匹配、行号、glob、不取键的正则有界地走查并给出继续的游标、界线；计划测试与时间的守卫。
10. **最后一跳与扩展点**（13.1 第 21 条）：注册表经组合根拼起的工具到达各模块的用例，组合根交空时整个程序的测试失败；`mcp:<名>` 到达 `changesets.client`；asset 的附件读、page 的路径端口到达 linking 的工具；M10 的 prompt 以测试替身登记、列出、取到。
11. **真实客户端**：Claude Code 与 Codex 的接入清单（`manual/`）写好，接入文档里的配置样例出自它；负责人在 v0.1 的最后与输入法清单一起执行，通过之后 M9 才改为已完成（4.16）。

## 4. 关键决定

### 4.1 模块、端口与构建次序

| 项 | 决定 | 放弃的方案 |
|---|---|---|
| 平台包 | `platform/httpserver/mcpserver`（新）：唯一导入 SDK 的生产代码。它装配无状态的 SDK handler、两个入口各一个 `*mcp.Server`、工具的登记、按调用的 ctx、结果与错误、日志与握手时的说明。它在 httpserver 之下，算 httpserver 的一部分（`archtest/rules_test.go:103-114`），可以用 httpserver 导出的 `ProblemError`、`RequestID`、`WriteProblem` | 独立的 `platform/mcp`：平台包之间互不导入（`archtest/rules_test.go:313-320`），拿不到问题的接口与请求号；放在 bootstrap：模块的 `adapter/mcp` 要用注册表的类型，导入不了组合根 |
| 工具从哪里来 | 各模块的 `adapter/mcp` 由同一组用例的实例构建工具，模块根以 `Tools()` 交出（同 `Jobs()`）。page 的 `New` 把用例建一次，同时交给 `adapter/http` 与 `adapter/mcp`（今天只交给 `httpadapter.UseCases`，`page/module.go:194-226`；`adapter/mcp` 不能导入 `adapter/http`，`archtest/rules_test.go:262-271`）。登记工具的是 page（13 个）、linking（`backlinks`、`index`）、notebook（`notebooks`） | 把用例的集合挪进 `app` 或模块根：五个模块的重构，只为让第二个适配器拿到实例；在 bootstrap 里拼工具：组合根有了业务 |
| 注册表的类型 | `mcpserver.Tool`：名称、标题、说明、注解、入口（笔记本、账户或两者）、输入的 Go 类型（schema 由 jsonschema-go 推导）与处理函数 `func(ctx, Call, In) (Out, error)`；`Call` 带这次调用的笔记本与客户端名称。`mcpserver.Prompt` 同形（M9 不登记，M10 用；以测试替身证明，第 8 节）。模块不导入 SDK 的类型 | 模块直接调 SDK 的 `AddTool`：SDK 散到每个模块，archtest 管不住 |
| 跨模块的工具 | `read` 读附件经 asset 接好线的模块的方法 `(*asset.Module).AttachmentReads()`（判定 `asset.read`、读行与元数据、经 `Blobs.Open` 读字节、签地址）；page 的 `Tools(ToolDeps{Attachments})` 在 asset 建好之后由组合根调用，构建次序仍是 page → asset → transfer（13.1 第 11 条）。linking 的工具按路径找页、给来源页的路径，经 page 只凭连接池的新端口 `page.NewNodePaths(pool)`（按路径找节点、按 id 给路径，不判定：调用之前已判定过笔记本，4.6） | asset 自己登记一个 `read`：一个工具名只能有一个处理函数；linking 读 `nodes`：越过模块的边界 |
| 入口的判定与说明 | page 的 `MCPEntry()`：`Gate(ctx, notebook)`（判定 `node.list`，看不到答 `notebook.not_found`）与 `Instructions(ctx, notebook)`（4.6）；以小接口交给 mcpserver，平台不读 `shared`（13.1 第 12 条） | 在 `getServer` 里判定：SDK 对已有会话不理会它的结果，返回 nil 答 400 而不是 404（`sdk/mcp/streamable.go:347-353`、`:400-403`，G1） |
| 凭证与名称 | 凭证是不是 PAT、是哪一个，由组合根以 `shared.RequireActor` 的适配交给 mcpserver；客户端名称的记忆是 identity 只凭连接池的 `identity.NewClientNames(pool)`（4.5） | mcpserver 读 `shared.Actor`：平台不导入 `shared` |
| 组合 | `bootstrap/mcp.go`：`mcpTools(pg, ln, nb)` 依次拼接，交给 `mcpserver.New`，名称重复、入口不对 panic（接线的错）；`/mcp` 在模块注册之后、`/` 之前挂上（4.4）。`bootstrap/mcp_tools_test.go` 把两个入口的 `tools/list` 与入库的 `api/mcp/tools.json` 比较（同契约测试的"路由正好是契约的操作"，`bootstrap/contract_test.go:27-49`），`-update` 重写它，`make gen-check` 核对 | 工具清单只在代码里：说明与 schema 的改动在审查里看不出来 |
| SDK 的版本 | `server/go.mod` 固定 `github.com/modelcontextprotocol/go-sdk` v1.8.0（今天最新，M0/P1 验证过的）。archtest 加一条：它与 `github.com/google/jsonschema-go` 只由 `platform/httpserver/mcpserver` 与 bootstrap 的测试导入（照 River，`archtest/rules_test.go:326-345`）。新加的模块：jsonschema-go v0.4.3、segmentio/encoding v0.5.4（与 asm）、yosida95/uritemplate/v3、x/oauth2、x/time，许可证 MIT 与 Apache-2.0；不含被禁的 google/uuid（`archtest/binary_test.go:14-21`）。v1.9.0 去掉 `MCPGODEBUG` 的开关：我们一个也不用，升级时跑 MCP 的整个程序测试与 e2e（第 14 节） | 跟着最新版本走：SDK 的行为在小版本之间改过（v0.8.0 → v1.8.0 的 16 处，见调查） |

page 的新用例：读 `ListByPattern`（`list`）、`ReadPath`（`read` 的页）、`Instructions`、`OpenNotebook`、`Grep`、`Search` 加按笔记本的入口 `InNotebook`；写 `WritePath`、`EditPage`、`RelocatePath`、`DeletePath`、`ApplyBatch`。复用的：`ListVersions`、`GetVersion`、`ListChangesets`、`GetChangeset`、`ListChanges`。linking 的新用例 `ListIndex`；notebook 的新用例 `ListAccountNotebooks`。

`page.Deps` 不加端口（附件读在 `Tools` 时交）；`linking.Deps` 加 `NodePaths`；`notebook.Deps` 用已有的 workspace 端口读调用者所在的工作区。

### 4.2 数据：迁移与配置

| 表 | 改动 | 为什么 |
|---|---|---|
| identity `api_tokens` | 加 `client_name text`（可空，`CHECK (octet_length(client_name) BETWEEN 1 AND 512)`）：这个 PAT 最近一次 `initialize` 时客户端报的名称，原样截到 512 字节 | 无状态时旧协议的调用不带 clientInfo，重启之后仍要知道是哪个客户端（4.5） |
| page `changesets`、`edit_sessions` | `changesets_client_check` 与 `edit_sessions_client_check` 改为不随区域设置的写法：`web`、`api`、`cli`，或 `mcp:` 加 1–128 个字符，不含 Cc（U+0000–U+001F、U+007F–U+009F）、Zl、Zp（U+2028、U+2029）与 Bidi_Control（U+061C、U+200E、U+200F、U+202A–U+202E、U+2066–U+2069），即 `shared` 的 `unshowable`（`shared/name.go:34-36`）；`domain.Client.Valid` 改用同一个判断（今天只拒 `unicode.IsControl`，`page/domain/client.go:22-30`）。`schema_test` 逐类核对：应用收的数据库都收 | M4/P1 的移交第 1、2 项：今天的 `[:cntrl:]` 随数据库的区域设置，应用比数据库宽时检查答 500 |
| page `changesets` | 加 `renames boolean NOT NULL DEFAULT false`、`moves boolean NOT NULL DEFAULT false`：单元第一次记下一条改名、移动的条目时设上；迁移按已有的条目补值。`getChangeset` 的 `renames`、`moves` 读它们，不再只看前 1,000 项条目（`page/adapter/postgres/queries/history.sql:73-81`） | batch 的变更集至多 1 万个条目（4.11），改名可能在第 1,000 项之后，网页的撤销就不提示"链接不跟着改回"（M8/P3 的移交"另"第 1 条） |
| linking | `index` 的读要不要新的索引（`page_properties` 今天只有主键与 `notebook_id` 的索引，`00023_linking_page_properties.sql:6-17`），由 P4 的计划测试定 | 4.9 |

没有 MCP 自己的表：无状态，没有会话；工具清单不在库里。`transfer_jobs` 不改（第 11 节第 1 条）。

**配置**（13.1 第 15 条）：`server.public_url`，默认空：实例对外的地址，只到源（`https://wiki.example.com`，可以带端口与结尾的 `/`），`validate` 拒绝别的协议、路径、查询、片段与用户信息；`LogValue` 列出它（不是机密）。用处只有 `read` 给附件的绝对地址（4.9）：签名地址今天是相对的（`asset/adapter/http/upload.go:186-194`），服务端不读 `Host` 与 `X-Forwarded-*`（`platform/httpserver/clientip.go:26-81`），按请求补全要信任转发头，反向代理的配置一错就签出别人的主机名。为空时 `read` 只给元数据并说明。MCP 的其余量都是代码里的常量（4.14），不进配置。

### 4.3 传输：无状态的 JSON 答复

| 项 | 决定 | 放弃的方案 |
|---|---|---|
| 模式 | `StreamableHTTPOptions{Stateless: true, JSONResponse: true}`：每个 POST 自己是一个完整的请求与答复，SDK 为它建一个临时的会话、答完就关（`sdk/mcp/streamable.go:373-447`）；GET 与 DELETE 答 405 `Allow: POST`（`:381-386`）；不发 `Mcp-Session-Id` | 有状态：工具的 ctx 是建会话的那个请求的，值冻结、没有期限、请求结束也不取消（`sdk/mcp/streamable.go:700-703`；`sdk/internal/jsonrpc2/conn.go:231-234`、`:803-821`，G6），每次调用的执行者、期限与请求号都要从 `req.Extra` 重建；会话在进程内，重启之后客户端拿到 404，Claude Code 与 Codex 会不会重新握手没有验证过（M0/P1 的移交第 1 项）；会话要绑定账户就得用 SDK 的 `RequireBearerToken`，它拒绝没有期限的 PAT（G2）；GET 的流要另做重新认证（G5）。协议也在往无会话走：2026-07-28 的协议只在无状态时服务（`sdk/mcp/streamable.go:868-876`），有状态答 -32022 |
| 每次调用的 ctx | 无状态时工具的 ctx 是这个 POST 的 ctx 包上 `notDone`：值是这个请求的（执行者、请求信息、凭证的期限），期限与取消被去掉（`sdk/mcp/streamable.go:434`；`sdk/internal/jsonrpc2/conn.go:231-234`）。mcpserver 在交给 SDK 之前把请求的 ctx 记在值里，工具的包装由它派生调用的 ctx：执行者、日志的字段照请求的，期限取"请求的期限"与"读完请求体时 + `server.request_timeout`"中较早的（同 `API.Stream` 读完请求体之后的一步，13.1 第 14 条），请求的 ctx 结束（客户端断开、停机）时取消 | 依赖 SDK 的 `PropagateRequestCancellation`：只对 2026-07-28 的请求有效（`sdk/mcp/streamable.go:210-219`、`:431`） |
| 重启 | 没有会话，重启对客户端不可见：同一个客户端的下一次 POST 照常（整个程序的测试：同一个库上重建应用，官方 Go 客户端不重新连接就能调用）；Claude Code、Codex 的实际表现进人工清单（4.16） | 设 `SessionTimeout`、`KeepAlive`：无状态时没有会话可回收 |
| 握手与旧协议 | 旧协议（2025-11-25 及更早）的客户端照常 `initialize`；之后的请求没有会话，SDK 为它合成没有 clientInfo 的 `InitializeParams`（`sdk/mcp/streamable.go:508`），所以名称另有来源（4.5）。2026-07-28 的客户端每个请求带 `_meta`，含 clientInfo（`sdk/mcp/shared.go:564-604`） | 只服务 2026-07-28：今天的 Claude Code、Codex 不一定发它 |
| 停机 | 调用的 ctx 派生自请求的 ctx：HTTP 的停机等进行中的请求做完（`server.shutdown_timeout`，20 秒，长于一次调用的 15 秒），之后才停任务、关连接池（`bootstrap/app.go:96-102`） | 有状态时调用在会话的 ctx 上，可能比 HTTP 活得久、碰上已关的连接池（调查的风险） |
| 写的期限 | JSON 答复是普通的请求与答复，`server.write_timeout`（60 秒）长于它的期限（45 秒：`request_timeout` 加 `read_timeout`，4.4），停止读取的客户端占不住连接 | `LongLived` 解除写的期限，SDK 自己不设（M5/P2 审查 A-I1 的失败方式） |
| JSON-RPC 的批 | 2025-06-18 起的协议 SDK 拒绝（`sdk/mcp/streamable.go:1450-1453`）；更早的协议的一个批里每条消息各是一次调用、各自的期限，整个 POST 一个限流的单位 | 自己拒绝批：更早的客户端连不上 |

### 4.4 挂载、认证与 HTTP 的边界

- **普通的路由**：`router.Handle("/mcp", …)`（不带方法：GET、DELETE 也由 SDK 答 405，而不落到前端答 `index.html`），经平台的逐路由中间件 `API.Middlewares` 的同一串：请求信息 → 期限 → 请求体上限 → 失败闸门与认证 → 限流 → 请求体的结构（`platform/httpserver/api.go:165-187`）。结构检查用只有一个节点的表（根是对象或数组、成员不限）：它只做 I-JSON 的那一半，重复的成员名与不成对的代理项答 400（`platform/httpserver/bodyshape/ambiguity.go:10-19`），正文里不会出现客户端没写的 U+FFFD。`/mcp/` 与它之下的路径答 404 problem（`router.Handle("/mcp/", …)`），不落到前端。13.1 第 14 条随之改写：MCP 不是保持响应的路由，不经 `API.LongLived`。
- **请求体与期限**：组合根给 `/mcp` 一个按路由的上限 `6 × (MaxContentBytes + 1 MiB) + 64 KiB`（37,814,272 字节）：一次调用的字符串参数至多 5 MiB 的正文加 1 MiB 的别的（路径、原文、说明，4.14），每个字节按 JSON 最长的转义算 6 字节（同 page 的 `contentBodyLimit`，`page/adapter/http/limits.go:5-17`）。在 `BodyLimits` 里的路由期限另加 `server.read_timeout`（`platform/httpserver/api.go:219-238`），所以一个请求至多 45 秒，其中工具的工作至多 15 秒（4.3）。SDK 的 `MaxRequestBodyBytes` 设成同一个值（它默认 4 MiB，`sdk/mcp/streamable.go:224`）。
- **只收 PAT**（G3）：认证之后 mcpserver 看执行者，不是 PAT（会话的访问令牌）答 403 `forbidden`，detail 说"MCP 只接受个人访问令牌"。访问令牌 15 分钟过期、MCP 客户端不会续期，收下它只会让 agent 一刻钟之后失败；答 403 而不是 401：凭证本身有效（13.1 第 17 条的"401 只表示凭证无效"），RFC 6750 把"凭证不够"写作 403。这时闸门的单位已退回、限流的单位已取（`platform/httpserver/api.go:294-295`），接受。13.1 第 17 条加这个例外。
- **SDK 的选项**：`DisableLocalhostProtection: true`（同一台机器上的反向代理转发的 `Host` 不是回环地址，默认的保护答 403，`sdk/mcp/streamable.go:316-325`；由 Bearer 把关，总体设计 6.3）；不设 `CrossOriginProtection`：浏览器发不出带 `Authorization` 与 `application/json` 的跨源请求而不先预检，我们不答 CORS。
- **限流**：一个 POST 一个 `authenticated` 的单位，与 REST 共用 `pat:<id>` 的桶（`platform/httpserver/limit.go:21-36`）；一个 `batch` 也是一个单位，它的大小由 4.11 的界线管。每个客户端启动时握手、`notifications/initialized`、`tools/list` 三个单位。不加 MCP 自己的桶。
- **答复头**：`/mcp` 的答复也带 `Cache-Control: no-store`（`platform/httpserver/middleware.go:44-51` 今天只给 `/api/`）：工具的结果是笔记本的内容。
- **SDK 之前的拒绝**是 problem+json（401、403、404、413、429、400）；SDK 自己的拒绝是纯文字（405、415、Accept 不对的 400，`sdk/mcp/streamable.go:381-398`）。401 不带 `resource_metadata`：我们没有 OAuth。`/.well-known/` 仍由前端答 `index.html`（第 10 节）。
- **保留名单**：`mcp` 从 `reserved_slugs.txt` 的 `[reserved]` 移到 `[server]`（`workspace/domain/reserved_slugs.txt:29`；13.1 第 26 条），`bootstrap/reserved_test.go:18-36` 随之通过；它从 M2 起就保留着，没有工作区占着它。

### 4.5 客户端名称

变更集记 `mcp:<名>`（总体设计 6.3、13.1 第 5 条的"客户端由适配器定"）。无状态时名称按这个次序取：

1. **这次调用自己的 clientInfo**：2026-07-28 的请求的 `_meta`，或 `initialize` 请求本身（`req.ClientInfo()`，`sdk/mcp/shared.go:693-708`）。
2. **这个 PAT 记下的名称**：每次 `initialize` 时，mcpserver 经 `identity.NewClientNames(pool)` 把名称写进 `api_tokens.client_name`（只在不同时写，尽力而为，失败只记日志，同 `TouchAPIToken`，`identity/app/authenticate.go:101-107`）；没有 clientInfo 的调用按执行者的 PAT 读它（进程内按 PAT 缓存，`initialize` 时更新）。所以重启之后、客户端没有重新握手时，名称还在。
3. 都没有时是 `unknown`（升级之后一个旧协议的客户端从没握过手，罕见）。

一个 PAT 给两个客户端同时用时，第 2 步的名称是最后握手的那个：接入文档建议一个 agent 一个 PAT（同时也是撤销的单位）。

**清理与检查**（M4/P1 的移交第 1 项）：名称经 page 的 `domain.MCPClient(name)` 变成客户端：按 NFC、去掉首尾的空白、去掉 `unshowable` 的字符、截到 128 个字符，空了就是 `unknown`；不因为名称拒绝调用（名称不是用户写的）。`Client.Valid` 与数据库的检查是同一个集合（4.2），所以单元不会因为客户端答 500（今天答，`page/app/unit.go:172-174`）。记下的原样名称至多 512 字节，用时才清理。

**最后一跳**（M4/P1 的移交第 3 项，13.1 第 21 条）：经 serve 用两种来源（握手之后的旧协议调用、带 `_meta` 的 2026-07-28 调用）各写一页、各做一个 `batch`，断言 `changesets.client` 是 `mcp:<名>`；页的适配器不交客户端时测试失败。MCP 不开会话，`edit_sessions.client` 不涉及。

### 4.6 两种入口、`AGENTS` 与说明

- **入口**：`/mcp` 是账户入口，`/mcp?notebook=<id>` 是笔记本入口；只看这一个查询参数。笔记本入口在每个请求里、在 SDK 之前判定（总体设计 6.3"每个请求都判定，而且在查缓存之前"；G1）：不是 UUID 答 404 `notebook.not_found`，不查库（13.1 第 4 条）；`Gate` 判定 `node.list`（读者以上），看不到答 404 `notebook.not_found`。不是安全边界：PAT 有账户的全部权限，入口只是"cd 进去"（总体设计 6.3）。
- **调用里的笔记本**：笔记本入口的工具作用于地址里的笔记本，它们的输入 schema 没有 `notebook`（mcpserver 从推导出的 schema 里去掉它，客户端多传答 `bad_request`）；账户入口的这些工具都要 `notebook`（笔记本的 id）。两个入口里，每次调用都先过一次 `Gate`（同一个判定），之后工具才按路径找节点：v0.1 没有按页的权限，能读笔记本的人读得到它的每个节点，所以按路径的寻址在判定之后不泄露什么。用例照旧各自判定自己的动作（写者才能写）。
- **两个 server**：启动时各建一个，`getServer` 按 mcpserver 记在请求 ctx 里的入口选；`tools/list` 两边不同（账户入口的工具多 `notebook`，另有 `notebooks`、`open_notebook`）。
- **说明只在握手时**：SDK 只在 `initialize` 与 2026-07-28 的 `server/discover` 的结果里发说明（`sdk/mcp/server.go:2105`、`:956`）。mcpserver 在两个 server 上各装一个接收的中间件（`AddReceivingMiddleware`，`sdk/mcp/server.go:1860`），对这两个方法在结果里填上这次请求的说明：笔记本入口是 `Instructions(ctx, notebook)`，账户入口是内置的账户说明。所以不按笔记本建 server，也就不用回收 server（eviction 不关它的会话的问题随之没有，调查 D6）。
- **说明的缓存**（G16）：`Instructions` 先以一次点查找根下的 `AGENTS` 页（父节点为空、`name_key` 等于 `TitleKey("AGENTS")`、类型是页；用兄弟的唯一索引 `00012_page_nodes.sql:32-33`），键是（笔记本、这个节点的 id 或"没有"、它的版本号）；命中进程内的 LRU（256 项，每项至多 32 KiB）就不读正文。替换（另一个节点）、改名进出（有、没有或换了节点）、删除（没有）都换了键；从回收站恢复回来的是同一个节点、同一个版本号、同样的正文，键相同、文字也相同；还原版本、撤销写回都是新的版本号。根下名叫 `AGENTS` 的附件占着这个键时就是"没有"。
- **文字**：正文去掉 frontmatter（与解析同一条规则：`platform/markdown` 导出 `FrontmatterEnd`，今天的 `frontmatterSpan` 未导出，`platform/markdown/frontmatter.go:71`），去掉首尾的空白；空了算没有。长于 32 KiB 时截到 32,768 字节之内最后一个换行（没有换行时截到字符的边界），末尾加一行英文"`AGENTS.md` 更长，用 `read` 读全文"。说明只是建议：agent 不一定照做；`AGENTS` 页的保护（不能删除、改名）是 M10 的 `llm_wiki` 模式的（总体设计 5.7）。
- **内置的说明**（英文，约 1–2 KB，入库、有测试）：笔记本入口讲这本笔记本的路径（4.8）、工具各做什么、`batch` 与说明、锁与 `page.locked`、"写一个 `AGENTS` 页可以给 agent 写约定"；账户入口另讲先 `notebooks` 再 `open_notebook`，每个工具都带 `notebook`。
- **`notebooks`**（账户入口）：调用者所在的每个工作区（成员与访客）里看得到的笔记本，每项 id、名称、工作区的 slug 与名称、有效角色；按工作区的 slug、笔记本的名称排，不分页（6.1：笔记本是小的集合），至多 1,000 项、多了标出截断。notebook 的新用例 `ListAccountNotebooks` 经已有的 workspace 端口读调用者的工作区，再逐个工作区按 `ListNotebooks` 的同一条可见性规则读（同事件流的组合，`events/app/open_stream.go:51-58`）；集成测试核对"列表等于逐项判定读取"（13.1 第 3 条的约定，`bootstrap/notebook_visibility_test.go`）。没有规则行：它只涉及调用者自己（13.1 第 3 条的第一类）。
- **`open_notebook`**（账户入口）：`notebook` → 这本笔记本的说明（同握手时）与概况：页与附件的数目、根下的节点（路径，至多 100 个）、最后一个变更集的时刻。名称与角色在 `notebooks` 里。

### 4.7 结果与错误

- **登记**：mcpserver 经 SDK 的低层 `Server.AddTool`（`sdk/mcp/server.go:315`），输入 schema 由 jsonschema-go 从输入类型推导（结构是 `additionalProperties: false`，没有 `omitempty` 的字段必填），参数由 mcpserver 自己按 schema 校验、解码。不用泛型的 `AddTool`：它在出错时丢掉处理函数的结果，成功时把输出写进 `structuredContent` 并按输出 schema 校验，一个带细节的错误结果放不进去（`sdk/mcp/server.go:395-488`）。不声明 `outputSchema`：有它时错误的结果也要合它。
- **成功**：`structuredContent` 是结果的 JSON；`content` 给模型看：`read`、`versions` 读的正文是原样的文字（结构里只有元数据，不重复正文，免得模型读两遍），其余的是结构的 JSON 文字（SDK 的默认做法）。图片是一个 `ImageContent` 加元数据的文字。
- **错误**：一律是工具的错误（`isError: true`），不是协议的错误，模型看得见、能自己改（`sdk/mcp/protocol.go:289-304`）：

| 来源 | `content` 的文字 | `structuredContent.error` |
|---|---|---|
| `*shared.Error` | `<码>: <英文的说明>`，带成员的白话：字段（`ops[3].path: invalid_format: …`）、持锁人（下一条）、`linking.pages_locked` 的每一页、`server_busy` 与 `rate_limited` 的稍后再试的秒数 | `code`、`message`、`fields`、`lock`、`locks`、`retry_after_seconds`，batch 另有 `op`（第几步） |
| 参数不合 schema | `bad_request: <JSON 指针>: <原因>` | `code: bad_request`、`fields` |
| 期限到了 | `internal_error: the call ran out of time` | `code: internal_error`、`request_id` |
| 别的（未分类） | `internal_error: the server failed; request id <id>`，错误进日志（error 级，带工具名与请求号） | `code: internal_error`、`request_id` |

  不认识的工具由 SDK 答协议的错误 -32602（`sdk/mcp/server.go:1005-1012`）。码与 REST 的相同（13.1 第 8 条的码），MCP 只多一个 `page.batch_too_large`（4.11）。说明的文字是英文（13.5 第 4 条），码是给程序判断的。
- **锁的拒绝**（M5 的移交第 1 项，D34）：`page.locked` 的文字说出页的路径与持锁人："`A/B.md` is being edited by Alice in the web editor; try again after they leave edit mode. The agent cannot take the lock over." 持锁人就是调用者本人时（按会话的规则，本人别处的编辑器也拒，`page/app/edit_lock.go:47-48`）说"by you, in a browser"，`lock` 带 `you: true`。不说"自动重试"：锁跟着编辑器开着，关掉浏览器之后至多 120 秒才过期（`page/domain/session.go:13-16`）。`page.revision_mismatch` 说"重读再写"。

### 4.8 路径与 glob

**路径**（总体设计 6.3 的写法，D15）：

- 从笔记本根写起，段之间用 `/`；开头的一个 `/` 可有可无（coding agent 常写），`./`、`..`、空的段（`A//B`）答 422 `invalid_format`。标题里不能有 `/`（`shared/title.go:23-43`），所以分段没有歧义。
- 页是 `祖先/…/标题.md`：最后一段以 `.md` 结尾（ASCII 不区分大小写），去掉 `.md` 是标题；祖先写标题本身，不带 `.md`。附件是 `祖先/…/文件名`：最后一段不以 `.md` 结尾。附件的名称永远不以 `.md` 结尾（`page/domain/name.go:18-27`），所以最后一段的形状说出类型，同一个路径至多一个节点（兄弟的标题键唯一，`00012_page_nodes.sql:32-33`）。
- `祖先/`（以 `/` 结尾）是这个页的子节点，只在 `list` 里用；根是空的路径或 `/`。
- 每段按 `shared.TitleKey` 比较（NFC 与完整折叠，同兄弟的唯一性），不去空白；按段一次点查 `(notebook_id, parent_id, name_key)`。读的工具在判定之后不加锁地找；写的工具在单元里、笔记本行的锁下找（`write` 覆盖正文时是 `FOR SHARE`，改树的写是 `FOR NO KEY UPDATE`，两者都挡住树的写），找到的节点在单元里不会变（调查的风险 12）。
- 答复里的路径是节点现在的标题，照这条规则写。找不到时答 `page.not_found`，文字给出路径，最后一段没有 `.md` 而有同名的页时提示"页的路径以 `.md` 结尾"。
- 长度：一个路径至多 4,096 字节、`MaxDepth` + 1 段（10 层页加一个附件）。

**与链接的规则不同的四处**（M7 的移交第 3 项，G7）：MCP 的路径是寻址，不是链接的写法，不经 `ParseTarget`、`Resolve`（`linking/domain/target.go:47-121`、`resolve.go:86-149`），不做后缀匹配、不经别名：

| 情形 | 链接的规则 | MCP 的路径 |
|---|---|---|
| `A/x.png`，笔记本里没有叫 `x.png` 的附件 | 读作页 `A/x.png`（样例 019） | 只指附件 `A/x.png`；没有就是找不到。页 `x.png` 写作 `A/x.png.md` |
| `A/x.md`，笔记本里没有叫 `x` 的页 | 读作标题是 `x.md` 的页（`target.go:117-120`） | 只指页 `A/x`；标题是 `x.md` 的页写作 `A/x.md.md` |
| 没有扩展名的附件 `A/data` | 没有写法（样例 020，`link` 为 null） | 有路径 `A/data`；`read` 的元数据里 `link` 照旧为 null |
| `A/`（结尾的斜杠） | 不是合法的目标（`target.go:62-65`） | `A` 的子节点（只在 `list`） |

链接仍照 M6、M7 的规则写：agent 在正文里写 `[[A/x]]` 或 `[[A/x.md]]`（`.md` 结尾而有这个页时读作页，`target.go:105-121`），工具的说明讲清两者的不同。

**glob**（D16，`list` 与 `grep` 用）：

- 只有三种通配：`*`（一段之内任意个字符，不跨 `/`）、`?`（一个字符，不跨 `/`）、`**`（任意多段，包括零段，必须独占一段：`a**b` 答 422）。
- `{`、`}`、`,`、`!` 是普通字符：它们可以出现在标题里（`shared/title.go:64` 只禁 `` / \ : * ? " < > | # ^ [ ] ``），当作选择或否定的语法就和这样的标题有歧义。`[`、`]`、`\` 答 422：它们不能出现在标题里，只能是字符类或转义的语法，这个方言没有。
- 匹配节点的路径（页带 `.md`）：`**/*.md` 是全部的页，`A/**` 是 `A` 之下的全部节点（不含 `A.md` 本身），`*.png` 是根下的 png 附件。按标题键匹配：模式的字面部分与路径的每段都按 `TitleKey` 折叠之后比，同路径的寻址；`?` 数的是折叠之后的字符（`ß` 折叠成 `ss` 是两个，写明）。
- 线性的匹配（一段之内的 `*` 用不回溯的两指针；`**` 按段的动态规划，代价是段数乘模式的段数）；模式至多 1,024 字节、至多 32 个通配。
- 在 Go 里对整本笔记本的树匹配：读树与拼路径是 `ListNodes` 的代价（10 万个节点 62–71 ms，[M12 的性能移交](../M12-release/handoffs/M4-performance.md)第 48 行），大的笔记本交 M12。

### 4.9 读的工具

- **`list`**（`pattern?`、`limit?`、`cursor?`）：`pattern` 为空或 `/` 是根的子节点，`祖先/` 是这个页的子节点（一层，同 `ls`），别的是 glob（不带通配的就是那个节点）。每项：路径、类型、有没有子节点；页另有版本号、字节数、正文的最后改动时刻；附件另有 MIME 与字节数（page 已有的 `Deps.Attachments`，M8 4.1）。按路径的键（每段的标题键，以 `/` 连起，页带 `.md`）排，游标是最后一项的键，下一页从比它大的起：并发的写挪动了节点也不重复、不跳过别的节点。`limit` 1–1,000，默认 200。新用例 `ListByPattern`：判定 `node.list`，读整棵树（`ListNodes`）、在 Go 里拼路径与匹配，再为这一页的节点读页的大小（`ContentSizes` 一条语句，`page/adapter/postgres/queries/contents.sql:34-38`）与附件的信息。
- **`read`**（`path`、`offset?`、`limit?`）：
  - 页：原样的正文（含 frontmatter、BOM、`\r\n`，`page/domain/content.go:10-24`），按行给：`offset` 是第一行（从 1 起），`limit` 是行数（默认 2,000），以 `\n` 分行，`\r` 留在行里；文字至多 `MaxReadBytes` = 48 KiB，到了就停在那一行之前，结构里给 `next_offset`；一行长于 48 KiB 时只给它的前 48 KiB（到字符的边界）并标出截断。结构里另有路径、版本号、总行数、总字节数。48 KiB 的理由：Claude Code 把一次 MCP 工具的结果限在约 25,000 个 token，中文约每字一个 token 以上，48 KiB 约 1.6 万字（人工清单核对，4.16）。复用 `GetPageContent`（不取解析的预算，`page/app/get_page_content.go:24-38`），切行在 page 的新用例 `ReadPath` 里。
  - 附件（M7 的移交第 1、2 项）：元数据（路径、MIME、字节数、SHA-256、宽高、`link` 的写法）；PNG、JPEG、GIF、WebP 至多 `MaxInlineImageBytes` = 3 MiB 时另给 `ImageContent`（Claude 的接口一张图约 5 MB，以 base64 计时原文约 3.75 MB，取 3 MiB）；SVG（是 XML，模型不当图看，也不该当图片内联）、别的类型与更大的图给绝对的签名地址（`server.public_url` 加签名的路径，有效 1–2 小时，`asset/adapter/mac/sign.go:31-37`），以 `ResourceLink` 给出；`public_url` 为空时只给元数据并说明。附件的字节经 asset 的 `AttachmentReads`（判定 `asset.read` 之后 `Blobs.Open`，`asset/blobs.go:102-133`），不经公开的下载路由。签出的地址进了模型的上下文与对话记录，失去访问之后仍能读到到期：同 M7 的风险，接受（第 10 节）。
- **`versions`**（`path`、`revision?`、`offset?`、`limit?`、`cursor?`）：没有 `revision` 时是这一页的版本列表（`ListVersions`，按版本号倒序、游标分页，每项带变更集的类型、客户端、执行者的显示名与时刻）；有时是那个版本的正文，切行同 `read`（`GetVersion`）。回收站里的页没有路径，不能经它读（网页能）。
- **`log`**（`path?`、`actor?`、`client?`、`limit?`、`cursor?`）：`ListChangesets` 原样（`page/app/list_changesets.go:11-108`），`path` 换成 `page_id`，`client` 照 `ParseClientFilter`（`mcp` 是全部的 MCP 客户端）。每条带类型、客户端、执行者（id 与显示名）、时刻、说明、计数、撤销与被撤销，与前三项变化（M8 4.7）。
- **`changeset`**（`id`、`limit?`、`cursor?`）：`GetChangeset` 加 `ListChanges` 的一页；任务的变更集（导入、后台的恢复与撤销）照常读到（M8/P4 的移交第 3 项）。
- **`backlinks`**（`path`、`limit?`、`cursor?`）：linking 的 `ListBacklinks` 原样（分页、计数到 1,000、上下文的规则，`linking/app/backlinks.go:35-93`；M6/P4 的移交第 4 项），来源页加上路径（`PageTree.Paths`，`linking/app/access.go:34-36`）；路径经 `NodePaths` 找页。只对页：附件答 `page.not_found`，同 REST。
- **`index`**（`type?`、`tag?`、`parent?`、`limit?`、`cursor?`；总体设计 5.5，M6/P4 的移交第 4 项）：linking 的新查询 `ListIndex`，不在 linking 之外连它的表。列出索引里有的页，每项：路径、`type`（frontmatter 的 `type`，字符串，别的形状为空）、`summary`（同）、标签（至多 20 个）、正文的最后改动时刻（经 page 的端口与路径一起读）、入链数（来源页的个数，数到 1,000，同反链的计数）、父页的路径。筛选：`type` 按值相等、`tag` 含其下的标签（同 `getTag`，`linking/adapter/postgres/queries/reads.sql:96-105`）、`parent` 是这个页的直接子页。按（`type` 的键，空的在后；页的 id）排，游标是它；`limit` 1–100，默认 50。索引里没有行的页（M6 之前写的、`nervewiki reindex` 之前）不在其中，提取器旧了的照旧行在：写进工具的说明（调查的事实，`reads.sql:84-105`）。M10 的虚拟 `index.md` 按 `type` 分组、没有 `type` 的按父页分组，复用这个查询。

### 4.10 写的工具与编辑锁

- **`write`**（`path`、`content`、`base_revision?`；D19）：
  - 不带 `base_revision` 是新建：页已存在答 409 `page.title_taken`（文字说"带 `base_revision` 覆盖"）。缺的祖先补建成空的页，与叶子在同一个改树的单元里（新用例 `WritePath`，`Tree: true`，照 `ImportUnit` 在单元里缓存兄弟与祖先，`page/app/unit_import.go:134-170`），都排在兄弟的最后；祖先里有一段是附件答 422 `path` `not_allowed`（附件不能有子节点），超过 10 层答 409 `page.too_deep`。撞名不加序号（与导入不同）：agent 写的路径就是它要的。答复列出建了哪些路径、叶子的版本号 1。
  - 带 `base_revision` 是覆盖正文：照 `PutPageContent`（不改树的单元、正文行 `FOR NO KEY UPDATE`、经守卫，`page/app/put_page_content.go:27-79`）：页不存在答 `page.not_found`，版本号不符答 409 `page.revision_mismatch`，与当前相同不写、答 `changed: false`。
  - 路径要是页（`.md`）：附件的路径答 422（MCP 不上传）。
- **`edit`**（`path`、`old_string`、`new_string`、`replace_all?`、`base_revision?`；D20）：服务端的新用例 `EditPage`，照 Claude Code 的 Edit：
  - 按字节精确匹配：`old_string` 不空、与 `new_string` 不同；出现 0 次答 422 `old_string` `not_allowed`（"not found"），不带 `replace_all` 而出现多次答同一个码（"found N times; add context or set replace_all"）。
  - 换行：页的换行全是 `\r\n`、`old_string` 里只有 `\n` 而原样匹配不到时，把 `old_string` 与 `new_string` 的 `\n` 换成 `\r\n` 再匹配（模型常丢掉 `\r`；混用两种换行的页不这样做）。BOM 是正文的字节，照原样。
  - 原子：不加锁地读出正文与版本号 r（带 `base_revision` 而不等于 r 答 409），替换、检查正文（5 MiB、NUL）、解析（单元之前取预算，13.1 第 19 条），再以 `Base: r` 写（`ToggleTask` 的做法，`page/app/toggle_task.go:48-100`）。锁下版本号不再是 r（这期间有人写过）时：调用者没给 `base_revision` 就从读起重做一次（替换要在新的正文上重新成立），还不行或给了 `base_revision` 就答 409 `page.revision_mismatch`。替换只作用于它读到的那一版的字节，不会丢别人的写。不在正文行的锁下替换：那样要在事务里解析，13.1 第 19 条不许。
  - 答复：路径、新的版本号、替换的次数。
- **`move`**（`from`、`to`、`update_links?`，默认真；D21）：移动、改名或两者，按路径。`to` 的父节点要已存在（`page.not_found`，不补建：同 `mv`），已有节点答 409 `page.title_taken`，成环 `page.cycle`，太深 `page.too_deep`；两端的类型要相同（页对页，附件对附件，附件改名照 `CheckAssetRename` 保留扩展名，`page/domain/name.go:29-42`）。父节点变了时排在新兄弟的最后，只改名时次序不变。
  - **一步**：新的单元操作 `Unit.Relocate(id, parent, position, name)`：一条语句 `RelocateNode` 同时改父节点、次序值与名称（键以 `TitleKey`），兄弟的唯一索引只看最后的父节点与键；作为一步交给守卫与参与者（父节点变了是 `OpMove`，否则 `OpRename`），这一步的变更同时带新的父节点与名称。所以一次 `move` 的链接改写只有一次，"之前"是单元开始之前的索引，正确（M6/P4 的移交第 1 条的单个移动的情形：两步时第二步看到的"之前"是旧的，`linking/app/rewrite.go:107`、`:121`）。整组撤销的放回用同一条语句（4.12）。
  - `update_links` 经 `UnitSpec.Options.UpdateLinks` 交给参与者（M4/P1 的移交第 4 项、M6/P4 的移交第 2 条）：关掉时参与者什么都不做、也不预检锁（`linking/app/rewrite.go:48-51`），观察者照样重新解析，链接可能断开或指向别处。REST 的十个用例照旧写死为真（`page/app/create_page.go:37` 等）：网页总是改写。
- **`delete`**（`path`、`recursive?`，默认假；D22）：有子节点（页或附件：附件是这个页的子节点，删页就删它们）而不带 `recursive` 时答 422 `recursive` `required`，文字给出其下的页数与附件数；带时照 `DeleteNode` 删进回收站（一个单元，`page/app/delete_node.go:24-47`），没有大小的界线（同 REST；[M12 的性能移交](../M12-release/handoffs/M4-performance.md)第 14 项）。
- **编辑锁**（第 11 节第 2 条；M5 的移交第 1、3、5 项）：MCP 的写是一次性的，工具不开编辑会话、不心跳、没有 `take_over`，所以 M5 移交的服务端闲置上限不涉及。规则：

| 操作 | 网页与 REST（今天） | MCP |
|---|---|---|
| 正文的写（`write` 覆盖、`edit`、`batch` 的写） | 按会话：任何不是这次写所带的会话都拒，本人别处的编辑器也拒（`page/app/edit_lock.go:47-48`） | 同（MCP 的写不带会话，任何活着的会话都拒） |
| 删除 | 按账户：别的账户的会话拒，本人的会话随删除结束（`edit_lock.go:49-50`） | **任何活着的会话都拒，本人的也拒**，不结束任何会话 |
| 改名、移动 | 守卫放行；改写要碰的页被编辑（本人的也算）时答 `linking.pages_locked`（`linking/app/rewrite.go:134-145`） | 同 |
| 新建 | 放行 | 同 |
| 开会话、接管、强制解锁 | 有 | 没有 |

  删除的规则由守卫按 `Write.Client` 认出 MCP（`OpDelete` 的 `holds` 加"客户端是 `mcp:`"），这是改 M5 的守卫（12.1 第 6 条的例外，第 8 节写明）：在用例里预检挡不住预检与单元之间打开的编辑器，那时单元会悄悄结束它。理由：网页的删除是本人在场的动作，结束本人别处的编辑器是他自己的选择；agent 删除时用户的浏览器里正开着这一页，悄悄结束它就违反了第 11 节第 2 条。版本号不符照旧排在锁之前（`page/app/unit_content.go:65-66`）。

### 4.11 batch

`batch`（`ops`、`message`）：几个写操作与一句说明，一个变更集原子执行（总体设计 6.3、5.3）。

- **操作**：`ops` 1–100 项，每项 `{op: write | edit | move | delete, …}`，其余字段同名工具的参数；没有恢复、还原与撤销（第 11 节第 1 条）。`message` 必填（1–4,096 字节，`page/domain/changeset.go:144-160`），经 `UnitSpec.Message` 写进新建的变更集（`page/app/unit.go:354-357`；M8/P3 的移交"另"第 3 条）。变更集的类型是 `edit`。
- **语义**：依次执行，每一步看到前面各步的结果（路径按那一刻的树）；`base_revision` 指 batch 开始之前的版本号。任何一步被拒，整个回滚，答复带这一步的序号（`op`）与它自己的码与细节（M5 的移交第 2 项）。
- **两段**（照撤销的"未加锁一遍、锁下一遍"）：
  1. 单元之前、不加锁：读这本笔记本的树与要读的正文，在 domain 的纯函数 `PlanBatch` 里依次模拟每一步：找路径、核对名称与层级、下面的约束与界线、算出每一页最后的正文；然后按（页、正文的哈希）解析每页最后的正文（`DecidedAll`，从大到小、等待合计有界，`page/app/content.go:87-131`）。
  2. 单元里（`Tree: true`，笔记本行 `FOR NO KEY UPDATE`，新动作 `page.batch`，写者）：按次序重做每一步，路径在锁下重新找；同一页的几个正文操作合成一步，在这一页第一个正文操作的位置写它最后的正文（一个版本行、一次守卫：M10 的模式守卫看到的是 agent 要的最后正文，而不是中间态）；与计划不同（这期间别人改了树或正文）就照那一步的码拒绝；只有 `edit`（不带 `base_revision`）的页的版本号变了时，整个 batch 从第 1 段重做一次（同 `edit`）。
- **约束**（单元里核对，违反答 422 `ops[i]` `not_allowed`，文字说哪一步与为什么）：
  - **B1** 不能删除这个 batch 新建的节点，或含有它的子树：合并成前后都空的条目被检查拒绝（`00015_page_changeset_items.sql:25-28`），节点又会在回收站里留下一项谁也没删的东西。
  - **B2** 不能先删子孙、再删含有它的祖先：同一时刻的删除被 `DeleteNodes` 的 `stray` 拒绝（500，`page/adapter/postgres/store.go:286-297`）。agent 直接删祖先。
  - **B3** 删除的子树里不能有这个 batch 改过名、移动过的节点（包括移进来的）：条目合并之后只留第一次的"前"与删除（`changesets.sql:41-44`），删除时的位置与名称没有记下，而撤销按"C 删除它时的位置与名称"比较回收站里的项（`page/domain/revert.go:460-500` 的 `sameState(n.Node, *it.Before)`），改过名再删除的项会永远 `changed`；"移出 P、删除、再删 P"也会按"前"的父节点折进 P 的一组（`revert.go:290-304`，M8/P3 的移交第 4 条调查补的情形）。写过正文再删除可以（第 7 条，4.12）。
- **合并的语义**（M4/P2 的移交）：新建之后改名、移动、写正文合并到新建（条目没有"前"，版本行从 1 升，`base_revision` 留空，`changesets.sql:46-57`）；改名、移动之后写正文是一项条目加一行版本；写正文之后删除是删除的条目、带删除时的版本号，版本行留着（撤销时先恢复、再写回 `Base`，4.12）；新建后删除（B1）、改名移动后删除（B3）被拒；删除后恢复、恢复后删除不会出现（没有恢复的操作；撤销的一个单元里一个节点至多删除或恢复一次，4.12）。所以合并成前后都空的条目不会出现，它仍被检查拒绝（缺陷，500）。`Change.Then` 不改（`page/domain/change.go:63-73`），`recordItems` 的注释改为写明这几种（`page/app/unit.go:367-369` 今天说没有单元先建后删）。真的数据库上每种各有一个多操作的单元的测试。
- **链接改写**（M6/P4 的移交第 1、3 条）：batch 里每个改名、移动的参与者推到全部操作之后、观察者之前，以合并的改动一次调用（只留单元前后都活着的、`update_links` 为真的那些节点）：之前是单元开始之前的索引，之后是锁下最后的树，指向这些节点的链接照"改名不改变链接的指向"改写（总体设计 4.5）。batch 自己写了正文的页不在改写之中：它们的正文照 agent 写的，单元结束时由观察者按最后的树解析（参与者本来就跳过版本号比索引新的页，`linking/app/rewrite.go:227-231`；batch 把这些页交给它，不记警告）。写进工具的说明与总体设计 4.5。改写的页数另有界线 `MaxBatchRewrites`，P3 照 M8 的规则实测（满负载下单元不超过 5 秒的最大一档），超过答 422 `page.batch_too_large`，文字提示 `update_links=false` 或拆成几次 `move`。单个 `move` 不受它限（同 REST，M6/P4 实测 5,000 页到 15 秒的期限，[M12 的性能移交](../M12-release/handoffs/M4-performance.md)第 3 项）。改写要碰的页被编辑时答 `linking.pages_locked`，`op` 是第一个改名、移动的序号。
- **加锁的次序**（D31，13.1 第 5 条）：batch 是改树的单元，持笔记本行 `FOR NO KEY UPDATE`；页面的每个单元都先锁笔记本行（`FOR SHARE` 与它冲突），所以同一笔记本的别的单元此刻不持这本笔记本的任何正文行，batch 按操作的次序锁正文行不会与它们成环。`page/app/unit_content.go:38-41` 的"按 id 的次序"只对 `FOR SHARE` 之下写几页的单元，今天没有这样的单元，写进 13.1 第 5 条。
- **界线**（13.1 第 31 条；超过答 422 `page.batch_too_large`，文字给出哪一项、多少，什么都不写）：操作至多 100 项；字符串参数合计至多 6 MiB，其中正文与 `new_string` 合计至多 5 MiB（`MaxContentBytes`，与请求体的上限配套，4.4）；要解析的页至多 100 页、8 MiB、20,000 条链接（`MaxUnitPages`、`MaxUnitBytes`、`MaxUnitLinks`，`page/domain/trash.go:46-73`，只有一页时不限）；条目（新建、删除的子树里的节点、移动与改名的节点）至多 `MaxUnitNodes` = 10,000；改写的页至多 `MaxBatchRewrites`。条目的界线让 batch 的变更集在网页上总能撤销：1 万项之内在请求里（页数与字节超过时在后台，M8 4.9），远在后台的 `MaxJobItems` = 60,000 之内。
- **搜索**（M8/P5 的移交第 2 条）：batch 照常以准确入队，agent 写完立即搜得到（`page/app/unit.go:427-446`）；一次至多约 100 页的正文，远小于 M8 实测的 1,000 页的积压。P3 在一个 100 项的 batch 之后重量第一页为空的比例，比 M8 的平时多出很多时再改为批量入队（`UnitSpec.Background`）。
- **事件**：一个单元一次（3.11）：`pages` 多于 20 页时为 `null`（`events/app/publisher.go:50-64`），网页整体重读；`links` 同 M6。
- **答复**：变更集的 id 与每一步的结果（路径、版本号、建了哪些路径）。

### 4.12 batch 的整组撤销：多轮应用与临时名称

M8 的撤销按状态比较、对 S\* 一次核对、单轮应用（M8 4.8）；batch 是第一个让一个变更集里有几种操作的地方，单轮会答 500、给错的正文或假的冲突（G22）。M9 照 [M8/P3 的移交](handoffs/M8-P3-revert.md)改 `PlanRevert` 的应用，L 与 S\* 的核对与冲突的种类不变，网页与接口不变（第 11 节第 1 条：撤销是人在网页上做的）。

- **按依赖的调度**（第 1 条）：`round()`（`page/domain/revert.go:810-859`）换成 `schedule()`。待做的步：放回（C 的节点现在与 S\* 都活着而父节点、名称或次序不同）、删除（S\* 里不存在的最上层）、恢复（C 删除的各组）、写回（S\* 里活着、有 `Base` 的页）。每一轮按固定的次序（放回、删除、恢复、写回，各按节点 id）试每一个待做的步，前提此刻成立就做、更新模拟的状态，不成立的留到下一轮；有做成的就再来一轮。前提：放回是目标父节点此刻活着或是根、不在节点此刻的子树里、目标的键在那里此刻空着（节点自己占着的不算）、子树的层级不超过；删除是此刻的子树里没有 S\* 里活着的节点；恢复是 S\* 的父节点此刻活着或是根、根的键空着；写回是页此刻活着。
  - 改名之后又新建同名、删除之后又新建同名：放回、恢复等删除新的那个先做；移入新建的文件夹：删除文件夹等移出的放回先做；恢复到另一组之下（第 4 条）：等那一组先恢复。
  - 一轮什么也没做而还有待做的，只能是名称的环（互换名称）或中间态的父节点的环：取待做的放回里 id 最小的、目标的键被另一个待做的放回占着的那个，先把占着者改成临时名称；没有名称的环时，把 id 最小的待做的放回先移到根下（也带临时名称）；然后继续。S\* 合法时总能做完；做不完仍答 `ErrRevertUnordered`（缺陷，500）。M8 的每种变更集照旧一轮做完，次序与今天相同（M8 的测试不改）。
- **一条语句的放回**（第 2 条）：放回用 4.10 的 `RelocateNode`，父节点、次序与名称一起改，中间没有"带着现在的名称挂到新父节点下"的状态（`MoveNode` 只改父节点与次序，`page/adapter/postgres/store.go:270-284`，撞上兄弟答 `ErrTitleTaken`）；一个放回一步（`place` 今天是移动再改名两步，`page/app/unit_revert.go:124-150`）。
- **临时名称**（第 3 条）：`<名称> ~<节点 id 的前 8 个十六进制字符>`，名称部分在 UTF-8 的边界上截短，使整个不超过 255 字节（今天的写法从 246 字节起超出 `nodes_name_check`，`00012_page_nodes.sql:14`，移交写的 250 更正）；附件把后缀放在扩展名之前（`照片 ~1a2b3c4d.png`），扩展名照旧；与兄弟的键冲突时多取 4 个字符再试。临时名称只在单元之中：同一节点之后的放回改成 S\* 的名称，条目合并成第一次的"前"与最后的"后"，观察者拿到合并的改动，所以它不进条目、索引、搜索与事件。守卫看得到每一步：M10、M11 的守卫要认得撤销里的临时名称（`Write.Kind` 是 `revert`，写给后面的 M）。
- **先恢复、再写回**（第 5、7 条）：写回包括 C 删除的、有版本行（`Base` 不空）的页：S\* 里它回来、正文是 `Base` 的（今天的 `RevertWrites` 只取既没新建、恢复也没删除的页，`revert.go:278-283`，以删除时的正文恢复它，结果是错的正文）。调度让写回排在恢复之后。解析按（页、正文的哈希）：恢复的一步带回收站里正文的 Facts，写回带 `Base` 的；两份都算进页数与字节的界线（`withinBounds` 已分开数，`page/app/revert_changeset.go:262-272`）；锁下的配对也按（页、哈希）（`page/app/unit_revert.go:222-237`）。
- **先改名再删除**（第 6 条）：batch 的 B3 不许（4.11），由 B3 的整个程序的测试与合并取删除的版本号的适配器测试（`page/adapter/postgres/store_test.go:757`）守住。网页里先改名、之后另一个变更集删除照 M8，不涉及。
- **步数**（第 8 条）：放回、临时的改名或移动、删除、恢复各一步，合计至多 `MaxRevertSteps` = 1,500，未加锁时与锁下照旧核对；每个节点至多两步（一次临时、一次放回）。batch 至多 100 项操作，它的撤销远在界线之内。后台的计划也核对步数（今天不传，`page/app/jobs.go:296`）。
- **后台撤销**（M8/P4 的移交第 2 条）：任务开始时按调度算出整个计划（步的次序），切批沿着这个次序（`revertSteps`，`page/app/jobs.go:303-358`），另守两条：一个名称环的各步（临时的改名与之后的放回）、一页的恢复与它的写回不跨批，所以临时名称不会在两批之间提交、恢复的页不会带着回收站里的正文停在两批之间；每批在锁下按计划的次序核对它的步的前提（今天每批对只含这一批的节点再跑 `PlanRevert`、批内是单轮，互换在一批之内答 500、跨批答假的 `title_taken`，调查 1.8 节），不成立就照冲突停下（`revert_conflict`）。被推后的步排在它等的步之后，之前的批已经提交了它们。
- **测试**（第 9 条）：在真的数据库上（不只 domain 的纯函数）：互换名称、改名之后新建同名、删除之后新建同名、移入新建的文件夹、移动加改名的中间态（目标父节点下有节点占着它现在的名称）、长名称（246–255 字节）与附件的临时名称、先写正文再删除、恢复到另一组之下（由撤销的撤销构造：B3 让 batch 本身做不出）；撤销的性质测试（`bootstrap/revert_property_test.go:94-162`）的操作加 `batch`（随机的 2–6 项操作，经 MCP），从新到旧连续撤销 k 个之后树、正文的字节与索引等于之前的；后台撤销一个跨批的、带名称环的 batch 变更集（批的大小注入为 2）。

### 4.13 grep 与 search

**`grep`**（`pattern`、`glob?`、`ignore_case?`、`limit?`、`cursor?`；D23，[M8/P5 的移交](handoffs/M8-P5-search.md)第 1、4 条）：

- **语义**：Go 的 RE2 语法（`regexp`，同 ripgrep 默认的那一类），在一本笔记本的页的正文里按行匹配；正文含 frontmatter；不搜标题（`list` 的 glob 做）与附件。以 `\n` 分行，行尾的 `\r` 不参与匹配、也不在答复里。答复每处：路径、行号（与 `read` 的 `offset` 同一种数法）、这一行（长于 1,000 字节时以第一处命中为中心截到 1,000 字节并标出）。`limit` 是命中的行数，1–500，默认 100；答复至多 48 KiB。
- **在 Go 里匹配，不在 PostgreSQL 里复查**：RE2 对输入是线性的，PostgreSQL 的 ARE 不是（回溯、反向引用）；agent 写的是 RE2 一类的语法，ARE 的 `\b`、`.` 与换行、`\w`、内联选项、`{m,n}` 的上限都不同，逐一拒绝不如不用它；要行号与每一行，SQL 只给第一处命中与摘要（`page/adapter/postgres/queries/search.sql:189-224`）；PostgreSQL 从不编译用户的模式，错的模式不会成为只带 SQLSTATE 的 500（`page/adapter/postgres/search.go:35-55`）。NFC：每一行与模式里的字面量都按 Go 的 NFC 比，键也是 Go 的 NFC 算的（索引的词由 Go 分词），两边是同一个规整，没有 `nfcGaps` 那样的空隙（M8 Codex 评审 R2）；行号不受影响（换行不与任何字符组合）。
- **键**：`regexp/syntax` 解析、化简，取必须出现的字面串：顶层与必有的拼接里的字面量（`+` 与下限至少 1 的重复算一次），选择、可选、可以零次的部分不取；`ignore_case` 或 `(?i)` 时同样取折叠的键（键本来就按完整折叠算，`page/domain/search.go:230-240`，是超集，正则复查）。CJK 2 个字起给二元组，字母与数字 3 个起给三元组；1–2 个的整词不取：RE2 的 `\b` 只认 ASCII 的单词字符，`goé` 在 RE2 里有边界、在分词里是一个三个字母的词，取整词会漏。键至多 32 个，多了丢掉多的（仍是超集）。取键的函数在 page/domain，挨着分词（那些函数未导出，调查 grep 4 节）。
- **候选**：有键时是 `page_search_hits(notebooks = [这本], 正文的键, NULL)` 的命中（只读这本笔记本的键，范围是必需的参数，`00041_page_search_scope.sql:90-103`）并上队列里准确的节点，照搜索的正文层的读法（同一快照里数出的命中数作 `LIMIT`、只带笔记本的条件连 `nodes` 与 `page_contents`、别的检查在 `OFFSET 0` 之上，`search.sql:150-227`）；批量的节点不是候选，答复带 `building`。没有键时是这本笔记本活着的全部页，有界地走查（下一条）：拒绝它会让 agent 去 `read` 每一页，更贵；而且只有不经索引才找得到不进索引的文字（超过 64 个的 ASCII 字母与数字的连续串、长的 `data:` 串、一页约 1 MB 的词之后，`page/domain/search.go:53-68`），写进工具的说明。
- **走查**：候选与 glob 的交（树在 Go 里读一次、拼路径，4.8），按（最后改动, id）倒序，一批一条语句按 id 读正文，直到 500 页（`MaxSearchWalk`）、32 MiB 正文（`MaxGrepRead`，同反链一次读的上限，`linking/domain/context.go:23-26`）或命中满了 `limit`；停在中途时给游标（页与行）。所以一次调用至多读 32 MiB 的正文，与正则无关。
- **范围**（13.1 第 4 条的新规则、13.4 第 12 条）：只读这一本笔记本：键经两个函数只读它的倒排，树、正文都按笔记本读。它的读加进 `TestHiddenNotebooksLeaveTheSearchsReadsAlone`（看不到的笔记本里写什么不改变读的块）、计划测试（候选是位图、正文按 id、不取键时只按笔记本的索引读）与时间的守卫（无统计、`ANALYZE` 之后、通用计划）。
- **上限**：模式至多 1,024 字节，`regexp/syntax` 照 Go 的限制（嵌套 1,000、重复至多 1,000，Go 源码的 `regexp/syntax/parse.go:94`、`:1003-1005`）；含 `\n` 的模式答 422（按行匹配）；NUL 可以写（正文里没有 NUL，什么也找不到）；时间由 15 秒的期限与 32 MiB 的读管住（RE2 线性）。
- 新用例 `Grep` 在 page，判定 `page.read`；整个读在一个只读快照里（同搜索）。语句的错误只记 SQLSTATE（13.1 第 10 条）。

**`search`**（`q`、`limit?`、`cursor?`；D24、第 11 节第 4 条）：M8 的全文搜索原样（两层、约数、`next_cursor`、`building`、摘要；[M8/P5 的移交](handoffs/M8-P5-search.md)第 3 条），范围是这一本笔记本。`Search` 加入口 `InNotebook(ctx, notebook, req)`：经 `Notebooks.WorkspaceOf` 得到工作区（今天只有按 slug 的入口，`page/app/search.go:87`），在工作区上判定 `page.search`，看得到的笔记本里要有它（否则 404 `notebook.not_found`），之后同一套读（`SearchScope` 只有这一本，`page/app/ports.go:668-673`）。答复每项：路径、类型、摘要（纯文字，不带网页用的 UTF-16 位置）、最后改动的时刻；第一页另有计数（`exact`、`about`、`at_least`）。工具的说明讲：第一页可能少于 `limit` 个甚至为空而带游标；单个汉字只找标题；`building` 时结果可能不全。`grep` 与 `search` 的分工写进内置的说明：找字面的词与中文用 `search`，正则与行号用 `grep`。

### 4.14 上限

13.1 第 31 条：写者与调用者控制的数量都有上限，每个上限处有测试。

| 量 | 上限 | 在哪里 |
|---|---|---|
| 请求体 | `6 × (MaxContentBytes + 1 MiB) + 64 KiB` | 4.4 |
| 一次调用的字符串参数 | 合计 6 MiB，其中正文与 `new_string` 合计 5 MiB | 4.4、4.11 |
| 一次调用的工作 | `server.request_timeout`（15 秒），从读完请求体算起 | 4.3 |
| 路径 | 4,096 字节、`MaxDepth` + 1 段 | 4.8 |
| glob | 1,024 字节、32 个通配 | 4.8 |
| `list` | `limit` 1–1,000，默认 200 | 4.9 |
| `read`、`versions` 的正文 | 48 KiB 一次；`limit` 默认 2,000 行 | 4.9 |
| 内联的图片 | PNG、JPEG、GIF、WebP，3 MiB | 4.9 |
| 说明 | 32 KiB；缓存 256 项 | 4.6 |
| `notebooks` | 1,000 项，多了标出截断 | 4.6 |
| `open_notebook` 的根下节点 | 100 个 | 4.6 |
| `index` | `limit` 1–100；每页 20 个标签；入链数到 1,000 | 4.9 |
| `backlinks`、`log`、`changeset`、`versions`、`search` | 照 M6、M8 的用例 | 4.9、4.13 |
| `edit` 的重试 | 1 次 | 4.10 |
| `batch` | 100 项；页、字节、链接照 `MaxUnit*`；条目 `MaxUnitNodes`；改写 `MaxBatchRewrites`（实测） | 4.11 |
| `grep` | 模式 1,024 字节、键 32 个、走查 500 页与 32 MiB、`limit` 1–500、一行 1,000 字节、答复 48 KiB | 4.13 |
| 客户端名称 | 128 个字符；记下的原样 512 字节 | 4.5 |
| 撤销的步 | `MaxRevertSteps` = 1,500，含临时的步 | 4.12 |

### 4.15 权限、码的次序、加锁、日志与交错

- **新的动作**：`page.batch`（写者；规则表与矩阵各一行，`bootstrap/actions_test.go` 的"规则 = 动作"）。别的工具用已有的动作：入口的 `Gate` 是 `node.list`；`list`、`read`、`grep` 是 `node.list` 或 `page.read`；`versions`、`log`、`changeset` 是 `page.history`；`backlinks`、`index` 是 `backlink.list`、`tag.read`（`index` 判定 `tag.read`，同读标签）；`search` 是 `page.search`；写照各自的（`page.create`、`page.write`、`node.move`、`node.rename`、`node.delete`）；附件是 `asset.read`；`notebooks` 不经规则表（只涉及调用者自己）。
- **码的次序**（13.1 第 4 条的写法，工具的错误照它排）：参数的结构 `bad_request` → 与资源无关的取值 422（正文的大小与 NUL、路径与 glob 的写法、正则、`message`、`batch` 的项数与字节）→ 笔记本 `notebook.not_found` → 节点（按路径）`page.not_found` → 403 → 503 `server_busy`（解析的预算）→ 其余的 422（`edit` 的匹配、`recursive`、约束 B1–B3、`page.batch_too_large`）→ 409（名称、成环、太深、版本号）→ 守卫的 `page.locked` 与改写的 `linking.pages_locked`。新建（`write` 不带 `base_revision`）没有节点的 404，403 紧跟笔记本；`batch` 的项按次序，第一个出错的项定答复。各工具的完整次序在 Phase 文档里写，测试照它。
- **加锁**：`write` 新建、`move`、`delete`、`batch` 是改树的单元（笔记本行 `FOR NO KEY UPDATE`）；`write` 覆盖与 `edit` 是正文的写（笔记本 `FOR SHARE`、正文行 `FOR NO KEY UPDATE`）；读不加锁，`grep`、`search` 在只读快照里。`batch` 的正文行按操作的次序（4.11）。
- **日志**（13.1 第 10 条）：mcpserver 每次工具调用记一行（info）：工具名、`notebook_id`、`user_id`、`token_id`、`client`、结果的码、耗时、`request_id`；从不记参数（路径、正文、模式、`q` 都有标题或正文）。握手记客户端名称与协议版本（名称是客户端自己报的，不是个人信息）。用例照旧记 id 与数目；`batch` 另记操作的项数与改写的页数。访问日志只记 `/mcp`，查询串（`notebook`）不记（`platform/httpserver/middleware.go:138-181`）。
- **交错**（确定性的测试，结束时核对 `checkPages`、`checkLinks`、`checkSearch`）：MCP 的正文写与网页开会话在正文行上相遇（`page.locked`）；MCP 的删除与本人的编辑器（拒绝，会话还活着）；`batch` 与网页的改名在笔记本行上相遇；`batch` 的推后的改写与要碰的页上开会话；`edit` 的重做与并发的正文写；`write` 新建与并发新建同名（`page.title_taken`）；路径的寻址与并发的改名（单元里在锁下找）；多轮撤销与撤销之后的写；后台撤销一个带名称环的 batch 变更集与批之间的写；握手时 `AGENTS` 页被替换（下一次握手是新的）。

### 4.16 真实客户端与接入文档

- **自动的**：整个程序的测试用官方 Go SDK 的客户端（`sdk/mcp/client.go:51`、`StreamableClientTransport`，令牌经 `HTTPClient.Transport` 加，`sdk/mcp/streamable.go:1966-2000`）跑两种协议：旧协议握手之后的调用、2026-07-28 带 `_meta` 的调用；服务重启之后不重新连接继续调用。P1 另在仓库之外用 TypeScript 官方 SDK 的客户端试一遍无状态的 JSON 答复（握手、`tools/list`、调用、GET 的 405、重启），照 M0/P1 的做法丢掉代码、在 P1 文档里写结果：M0 的实验验证的是有状态的模式。
- **人工的**（M0/P1 的移交第 1、3 项）：`manual/P5-mcp-clients-checklist.md`：Claude Code 与 Codex 各一遍，不改负责人的客户端配置：Claude Code 用 `claude -p … --mcp-config <临时文件> --strict-mcp-config`（不写 `~/.claude.json`）；Codex 用 `-c` 覆盖配置或一个临时的 `CODEX_HOME`（只复制登录文件），不改 `~/.codex/config.toml`。步骤：两种入口连上、收到说明（让 agent 复述 `AGENTS` 的第一行）；读、写、`edit`、`batch`、`grep`；服务重启之后的下一次调用；撤销 PAT 之后的提示；被锁的页的提示；`readOnlyHint` 的工具在各审批选项下要不要确认（Claude Code 的许可模式，Codex 的 `default_tools_approval_mode` 的 `auto`、`prompt`、`writes`、`approve`，M0/P1 实验第 7 节）；48 KiB 的 `read` 放得下；读图片看得到图。负责人在 v0.1 的最后与输入法清单一起执行（第 11 节）；P5 的实施者在负责人同意时可以先跑一遍。
- **接入文档**（README 新的一节"接入 agent（MCP）"，D37）：一个 agent 一个 PAT；两种地址；Claude Code 的 `claude mcp add --transport http … --header "Authorization: Bearer …"` 与 `.mcp.json` 的样例；Codex 的 `config.toml` 样例（`url`、`bearer_token_env_var`）；审批选项各是什么意思，不一律推荐 `approve`（总体设计 6.3）；`server.public_url`；上限（`read` 48 KiB、`batch`）；401、403、404 各说明什么。样例出自人工清单的那一遍。
- **反向代理**（D10）：README 的部署一节改正：`/mcp` 是普通的 JSON 答复，不是 SSE，`X-Accel-Buffering` 只关事件流；`client_max_body_size` 不小于 `/mcp` 的上限（约 36 MiB，已有的"不小于附件与导入包的上限"即 50 MiB 覆盖它）；`proxy_read_timeout` 不短于 45 秒（默认 60 秒够）；`Host` 照常转发（回环保护已关）。

## 5. 接口

没有新的 REST 操作；`api/` 的 OpenAPI 不变（`/mcp` 不在契约里，契约的路径只许 `/api/v0/`，`platform/httpserver/apitest/rules_test.go:171-174`）。MCP 的"契约"是入库的 `api/mcp/tools.json`（4.1）。

| 入口 | 方法与路径 | 说明 |
|---|---|---|
| 账户 | `POST /mcp` | PAT；GET、DELETE 由 SDK 答 405；工具带 `notebook` |
| 笔记本 | `POST /mcp?notebook=<id>` | 同上；`notebook` 看不到答 404 |

工具（笔记本入口；账户入口另带 `notebook`）：

| 工具 | 参数 | 答复 | 注解 | 用例 |
|---|---|---|---|---|
| `list` | `pattern?`、`limit?`、`cursor?` | 节点（路径、类型、大小、版本号、MIME、有没有子节点）、`next_cursor` | 只读 | page `ListByPattern` |
| `read` | `path`、`offset?`、`limit?` | 页：这些行（文字）与元数据；附件：元数据与图片或地址 | 只读 | page `ReadPath`；asset `AttachmentReads` |
| `write` | `path`、`content`、`base_revision?` | 路径、版本号、建了哪些路径、`changed` | 破坏性、幂等 | page `WritePath` |
| `edit` | `path`、`old_string`、`new_string`、`replace_all?`、`base_revision?` | 路径、版本号、替换的次数 | 破坏性 | page `EditPage` |
| `move` | `from`、`to`、`update_links?` | 新的路径 | 破坏性、幂等 | page `RelocatePath` |
| `delete` | `path`、`recursive?` | 删除的页数与附件数 | 破坏性、幂等 | page `DeletePath` |
| `batch` | `ops`、`message` | 变更集的 id、每一步的结果 | 破坏性 | page `ApplyBatch` |
| `grep` | `pattern`、`glob?`、`ignore_case?`、`limit?`、`cursor?` | 命中（路径、行号、行）、`next_cursor`、`building` | 只读 | page `Grep` |
| `search` | `q`、`limit?`、`cursor?` | 结果（路径、类型、摘要）、`count`、`next_cursor`、`building` | 只读 | page `Search.InNotebook` |
| `backlinks` | `path`、`limit?`、`cursor?` | 来源页（路径、链接数、上下文）、`next_cursor` | 只读 | linking `ListBacklinks` |
| `index` | `type?`、`tag?`、`parent?`、`limit?`、`cursor?` | 页（路径、类型、摘要、标签、时刻、入链数、父页）、`next_cursor` | 只读 | linking `ListIndex` |
| `log` | `path?`、`actor?`、`client?`、`limit?`、`cursor?` | 变更集（M8 4.7 的形状）、`next_cursor` | 只读 | page `ListChangesets` |
| `changeset` | `id`、`limit?`、`cursor?` | 变更集与一页明细 | 只读 | page `GetChangeset`、`ListChanges` |
| `versions` | `path`、`revision?`、`offset?`、`limit?`、`cursor?` | 版本列表，或一个版本的行 | 只读 | page `ListVersions`、`GetVersion` |
| `notebooks`（只在账户入口） | — | 笔记本（id、名称、工作区、角色） | 只读 | notebook `ListAccountNotebooks` |
| `open_notebook`（只在账户入口） | `notebook` | 说明与概况 | 只读 | page `OpenNotebook` |

注解：只读的 `readOnlyHint: true`；写的 `destructiveHint: true`（都会改已有的东西），`idempotentHint` 只在同样的参数再调一次不再改变什么时为真（`write` 同样的正文答"未改动"，`move`、`delete` 第二次找不到源）；全部 `openWorldHint: false`。SDK 总是发 `readOnlyHint` 与 `idempotentHint`（`sdk/mcp/protocol.go:1997-2010`）。

16 个工具（12.1 第 4 条的"约 20 个接口操作"之内）。MCP 新加的码只有 `page.batch_too_large`（422，只在工具的错误里）。配置加 `server.public_url`（4.2）。

## 6. 从 Nerve 借鉴

Nerve 没有 MCP 的适配器（只读核对：`nerve-project/server/go.mod` 没有 MCP 的库，历史里也没有；它的 agent 走 REST，`nerve-project/docs/v0/v0-design.md:97-98`），没有可以照搬的。沿用的是本项目自己的：M0/P1 的 MCP 实验（挂载、认证、按笔记本下发说明、真实客户端的接入与 Codex 的审批选项，[01-P1-spikes](../M0-foundation/01-P1-spikes.md)第 7 节）；平台的逐路由中间件与按路由的请求体上限（M1、M4）；写入单元、守卫与参与者（M4–M6）；撤销的按状态比较与 S\*（M8/P3）；搜索的索引与读法（M8/P5）。

## 7. Phase 划分

| P | 名称 | 交付 | 验证 |
|---|---|---|---|
| P1 | 平台与读的工具 | SDK v1.8.0 与 archtest；`platform/httpserver/mcpserver`（无状态的 JSON、两种入口、`Gate`、PAT、按调用的 ctx 与期限、结果与错误、日志、握手时的说明、客户端名称）；`/mcp` 与 `/mcp/` 的挂载、按路由的上限、`no-store`、保留名单；`server.public_url`；identity 的 `client_name`；page 的路径（`NodePaths`、按段的点查）、`AGENTS` 与说明的缓存、`list`、`read`（页与附件，asset 的 `AttachmentReads`）、`versions`、`log`、`changeset`、`open_notebook`；notebook 的 `notebooks`；工具清单 `api/mcp/tools.json`；按工具的权限矩阵与"矩阵覆盖每个工具"；e2e 的 MCP 客户端（`e2e/fixtures/mcp.ts`） | 用例与工具的表格测试（码的次序）；G16 的五种换法；服务重启之后继续调用（官方 Go 客户端，两种协议）；PAT、访问令牌、NUL 与不成对的代理项；TypeScript SDK 客户端的仓库之外的试用；e2e：MC1、MC2 |
| P2 | 写的工具 | `write`（补建父页）、`edit`（匹配、CRLF、重做）、`move`（`Unit.Relocate`、`RelocateNode`、`update_links`）、`delete`（`recursive`）；编辑锁对 MCP 删除的规则；客户端检查的显式范围与 `domain.MCPClient` | 最后一跳（`mcp:<名>`，两种来源）；锁的每一格（4.10 的表）经 MCP 的整个程序测试（照 `bootstrap/page_lock_test.go:20-64`）与交错；`update_links=false` 的整个程序测试（链接不改、被锁的页不拒、`checkLinks`）；`schema_test` 的六类字符；e2e：MC3、MC4，C4 的 MCP 版本 |
| P3 | batch 与多轮撤销 | `ApplyBatch`（`PlanBatch`、B1–B3、正文合成、推后的改写、界线与 `MaxBatchRewrites` 的实测、`page.batch`）；`PlanRevert` 的调度、临时名称、一条语句的放回、先恢复再写回、按（页、哈希）解析；后台撤销按计划的次序切批；`changesets.renames`、`moves` | 合并的每种在真的库上的测试；同一单元两次改名（M6/P4 的移交第 1 条）；4.12 的撤销测试与加了 `batch` 的性质测试；后台撤销跨批的名称环；100 项 batch 之后的搜索第一页；e2e：MC5（页面版本在网页上整组撤销） |
| P4 | grep、search、index、backlinks | `grep`（取键、候选、Go 的 RE2、走查与游标、glob）；`Search.InNotebook` 与 `search`；linking 的 `ListIndex`（入链数）与 `index`；`backlinks` 带路径 | 块数的测试、计划测试、时间的守卫加上 `grep` 的读（13.4 第 8、9、12 条）；键是超集的性质测试（随机的正则与正文：Go 直接匹配全文找到的，经键也找到）；`index` 的可见性与"列表等于逐项读"；e2e：MC6、MC7 |
| P5 | 前面的故事的 MCP 版本与接入文档 | 第 9 节表里的 MCP 版本；README 的接入与反向代理；人工清单 `manual/P5-mcp-clients-checklist.md`；总体设计随之的修订 | 全部 e2e 通过（本 M 与之前所有 M）；接入文档的样例与清单一致 |

各 P 都只有服务端（与 e2e），没有网页的半边：网页上显示 MCP 地址的入口留给 v0.1 之后的打磨（第 11 节第 4 条），网页的历史已能按 MCP 筛选、显示说明与"MCP（名）"（`web/apps/web/src/pages/notebook/history-filters.tsx:17-18`、`history-row.tsx:89-91`）。每个 P 带它自己的故事的 e2e，P5 只做前面 M 的故事的 MCP 版本：每次合并都是绿的、故事随功能进来。P3 最重（批的计划与撤销的调度），开工时可再分 Step。收尾：三位 Opus 审查者并行（服务端、e2e 与文档、完成标准），然后 Opus 核对修复。

### 移交的落实

| 移交 | 项 | 落实 |
|---|---|---|
| [M0/P1 MCP 会话](handoffs/M0-P1-mcp-sessions.md) | 1 重启与超时、2 无状态、3 审批与注解、4 长连接路由绕过的中间件 | 1：无状态，没有会话，重启不可见（4.3）；整个程序的测试（官方 Go 客户端），真实客户端进人工清单（4.16，P1、P5）；2：选无状态，名称的来源见 4.5（P1）；3：注解（第 5 节），各审批选项的效果进人工清单、写进接入文档（P5）；4：不经 `LongLived`，经逐路由的中间件：期限、请求体上限、认证、限流、结构检查都有（4.4，P1）；只收 PAT（访问令牌答 403） |
| [M4/P1 客户端的检查](handoffs/M4-P1-client-check.md) | 1 先检查名称、2 区域设置、3 最后一跳、4 写入选项 | 1：`domain.MCPClient` 清理，不因名称拒绝（4.5，P2）；2：两张表的检查改为显式的范围，与 `unshowable` 同集合，`schema_test` 逐类核对（4.2，P2）；3：两种来源的最后一跳（4.5，P2）；4：MCP 的写用新用例，`Options` 由 `move` 的 `update_links` 交下去；REST 的十处照旧写死（调查更正：六处已是十处）（4.10，P2） |
| [M4/P2 一个单元里先建后删](handoffs/M4-P2-unit-merge.md) | 条目的检查、事件与守卫的值、删除再恢复、恢复再删除 | 新建后删除由 B1 拒绝（422），删除与恢复不会在一个单元里相遇（没有恢复的操作；撤销里一个节点至多一次）；各种合并的语义写定（4.11），每种在真的库上测；`recordItems` 的注释改写（P3） |
| [M5 编辑锁与 MCP 的写](handoffs/M5-locks.md) | 1 告诉 agent、2 batch 整个回滚、3 接管、4 测试、5 闲置上限 | 1：4.7 的文字与 `lock`（含"是你"），不说自动重试（P1、P2）；2：4.11，答复带第几步（P3）；3：不给（第 11 节第 2 条）；4：P2 的锁的整个程序测试与交错，C4 的 MCP 版本；5：MCP 不开会话，不涉及 |
| [M6/P4 链接改写](handoffs/M6-P4-rewrite.md) | 1 一个单元里几次改名、2 `update_links=false`、3 大笔记本的改名、4 `backlinks` 与 `index` | 1：单个 `move` 是一步（`Unit.Relocate`）；`batch` 把参与者推到最后、以合并的改动一次调用；"同一单元两次改名、链接各跟着改"的整个程序测试（4.10、4.11，P2、P3）；2：P2 的整个程序测试；3：batch 的改写另有实测的界线 `MaxBatchRewrites`，单个改名的提速仍在 [M12 的性能移交](../M12-release/handoffs/M4-performance.md)第 3 项（4.11）；4：`backlinks` 复用 `ListBacklinks` 加路径，`index` 是 linking 里的新查询，索引里没有的页不在其中写进说明（4.9，P4） |
| [M7 附件与 MCP](handoffs/M7-assets.md) | 1 `read` 读附件、2 对外的基地址、3 附件的路径写法、4 写 | 1：四种图片 3 MiB 之内内联，SVG 与别的给地址（4.9，P1）；2：`server.public_url`（4.2）；3：MCP 的路径是寻址，四处与链接的规则不同的情形逐一定下，链接仍照 M7 的规则（4.8，P1）；4：不上传，正文里的链接经同一个观察者与参与者（P2） |
| [M8/P3 batch 的整组撤销](handoffs/M8-P3-revert.md) | 1–9 与"另" | 1：按依赖的调度与临时名称；2：一条语句的放回；3：临时名称的截短（246 字节起，更正移交的 250）与附件的扩展名；4：调度让恢复等它的父组，batch 的 B3 让"移出再删除"的折叠不出现；5、7：先恢复再写回，按（页、哈希）解析；6：B3 不许先改名再删除，合并取删除的版本号照留；8：临时的步计入 `MaxRevertSteps`，后台的计划也核对；9：4.12 的测试（P3）。另：`renames`、`moves` 另记在变更集上（4.2）；`log` 照 `ListChangesets`（P1）；`batch` 写说明（P3）；agent 的历史工具只读：`log`、`changeset`、`versions`，不给还原、恢复、撤销（第 11 节第 1 条） |
| [M8/P4 MCP 开的后台任务](handoffs/M8-P4-background.md) | 1 MCP 开任务、2 batch 的大撤销、3 `log` 看得到任务的变更集、4 撤销正在写的撤销任务答 409 | 1：不做（第 11 节第 1 条），`transfer_jobs.client` 不放开；2：后台撤销按计划的次序切批，名称环与恢复加写回不跨批（4.12，P3）；3：不用改，`changeset` 工具读到（P1）；4：agent 不撤销，不涉及；网页照 M8 |
| [M8/P5 grep 用搜索的索引](handoffs/M8-P5-search.md) | 1 键与候选、2 写完即搜、3 `search`、4 上限 | 1：照移交取键（整词不取，理由见 4.13），候选照正文层的读法、只读这本笔记本，复查改在 Go 里以 RE2 按行做（不在 SQL 里以 ARE），不取键的正则有界地走查；移交把两个陷阱写成一个（5–40 秒的整表扫描与 97–143 ms 的连接），各自的修法照旧（P4）；2：batch 准确入队，P3 重量（4.11）；3：`search` 加按笔记本的入口（4.13，P4）；4：4.14 |

收尾时对照代码逐项核对，九份改为 `done`、写明落实。

### 写给后面的 M

由发现它的 Phase 写进目标 M 的 `handoffs/`，收尾时核对：

- **M10**：在注册表上登记 `lint`、`pending_sources` 与 ingest、query、lint 三个 prompt（`mcpserver.Prompt`，M9 以测试替身证明）；虚拟的 `index.md`、`log.md` 进 `list` 与 `read`（页的路径解析要认得它们），写它们由模式的守卫拒绝；`index.md` 复用 `ListIndex`；`AGENTS` 页在 `llm_wiki` 下的保护（不能删、改名、移动）由模式的守卫做，说明照 4.6 取；来源区的守卫要认得 `OpMove` 一步里同时的改名（4.10 的 `Unit.Relocate`）、`batch` 的正文合成之后的一步、撤销里的临时名称（`Write.Kind` 是 `revert`）。
- **M11**：`upgrade_verify`、`upgrade_submit` 登记在注册表上；冻结的守卫挡 MCP 的写（客户端 `mcp:`），回滚的撤销经多轮调度（4.12）；升级若要由 MCP 开后台任务，先放开 `transfer_jobs.client`（[M11 的移交](../M11-upgrade/handoffs/M8-P4-background.md)第 5 条，M9 没有放开）。
- **M12**：`list`、`grep`、`index` 每次读整棵树（10 万个节点约 70 ms）、`grep` 不取键时每次排序整本笔记本的页、`notebooks` 逐个工作区读、MCP 的单个删除没有界线（同 REST）、单个改名改写 5,000 页到期限：进性能移交；48 KiB 与 3 MiB 两个上限按真实客户端的反馈再看；网页上显示 MCP 的地址与接入的帮助进打磨移交；部署文档沿用 4.16 的反向代理说明。

### 总体设计的修订

随这份设计一起改的（审查之后由主持者落到 `v0.1-design.md`）：

- 1.3 加一条：MCP 用无状态的 Streamable HTTP 与 JSON 答复，客户端名称取自调用的 clientInfo 或 PAT 记下的名称（4.3、4.5）。
- 3.8：MCP 的客户端名称的来源；`batch` 的变更集带说明、类型是 `edit`；变更集记下有没有改名、移动（4.2）。
- 3.9：MCP 的写是一次性的、不开会话、不能接管；MCP 的删除被任何活着的会话拒绝，本人的也算（4.10）。
- 4.5：`batch` 里的改名、移动在全部操作之后一次改写，batch 自己写的页不改写；撤销的放回是一条语句（4.11、4.12）。
- 5.3、5.5：失败的 ingest 在网页上整组撤销（agent 不撤销）；`index` 工具的字段与筛选（4.9）。
- 6.2：`/mcp` 只收 PAT（4.4）。
- 6.3 整节按 4.3–4.13 改写：无状态的 JSON 答复，没有会话、`SessionTimeout` 与按笔记本的 server 缓存，说明在握手时由中间件按（笔记本、`AGENTS` 节点、版本号）的缓存填上；入口的判定在 SDK 之前；`AGENTS` 的找法、去 frontmatter 与 32 KB；路径的规则与四种情形、glob 的方言；工具表加 `search`、`changeset`、`versions`，`grep` 的语义；`read` 的行、上限与附件；客户端名称的来源。
- 6.4：附件的绝对地址经 `server.public_url`（4.2）。
- 7.1、7.2：`api_tokens.client_name`；`changesets.renames`、`moves`；客户端的检查改为显式的范围（4.2）。
- 8.4：`server.public_url`。
- 10.1：MCP 版本的故事由 e2e 的脚本化客户端（原生的 JSON-RPC，经 `fetch`）执行；只能由 MCP 完成的故事只有 MCP 版本（同"只能由命令行完成的"），第 9 节列出哪些故事有 MCP 版本、哪些豁免与理由。
- 11：反向代理对 `/mcp` 的要求（4.16）。
- 12.2 的 M9 一行（内容与规模：中偏重，五个 Phase，都在服务端）；12.3（M9 依赖 M6–M8 不变，M10 依赖 M9 的注册表）；12.4 的"MCP 工具与 prompt 注册表"写明形状（`mcpserver.Tool`、`Prompt`，模块的 `Tools()`，组合根拼接，入库的清单），"变更集的客户端字段"由 M9 注册；12.6。
- 随实现它们的 Phase 改（第 13 节）：
  - 13.1 第 1 条（`batch` 的单元：计划、合成、推后的参与者；`Unit.Relocate`；撤销的调度）、第 4 条（工具的码的次序、`page.batch_too_large`）、第 5 条（客户端检查的集合；`batch` 的正文行的次序）、第 8 条（工具的错误的形状）、第 10 条（MCP 的日志）、第 11 条（模块的 `Tools()`、`page.NewNodePaths`、asset 的 `AttachmentReads`、`identity.NewClientNames`）、第 14 条（`/mcp` 经逐路由的中间件，不是保持响应的路由；去掉"M9 的 MCP 流照此挂载"）、第 15 条（`server.public_url`）、第 17 条（`/mcp` 只收 PAT 的例外）、第 20 条（一个 POST 一个单位）、第 21 条（M9 注册的与最后一跳）、第 23 条不改、第 26 条（`/mcp` 进 `[server]`）、第 31 条（4.14 的上限）。
  - 13.4 第 3 条（e2e 的 MCP 客户端 `fixtures/mcp.ts` 与 MCP 版本调同一组断言）、第 5 条（工具清单、PAT、NUL 与不成对的代理项的整个程序测试）、第 6 条（按工具的权限矩阵）、第 12 条（`grep` 的读进块数的测试）。
- 14：MCP SDK 一行改为"M9 固定 v1.8.0，archtest 管住导入"；会话一行改为"M9 选无状态，重启不可见；真实客户端进人工清单"；新的风险见第 10 节。

## 8. 本 M 建立与注册的扩展点

建立（总体设计 12.4）：

- **MCP 工具与 prompt 注册表**：`mcpserver.Tool` 与 `mcpserver.Prompt`，模块根的 `Tools()`（以后 `Prompts()`），组合根的 `mcpTools` 拼接并核对入库的清单。M9 登记自己的 16 个工具；prompt 没有注册者，模块根上以测试替身证明登记、`prompts/list` 与 `prompts/get` 到达（13.1 第 21 条），整个程序的最后一跳写进给 M10 的移交。

注册：

- **变更集的客户端字段**（M4 建立）：MCP 的适配器给 `mcp:<名>`，最后一跳见 4.5。

改前面 M 的扩展点与代码（12.1 第 6 条的例外，逐项写明）：

- page：`Unit.Relocate` 与 `RelocateNode`（M4 的单元加一种操作；撤销的 `place` 改用它）；`batch` 的单元（`UnitSpec` 加批的模式：参与者推后、合并调用）；`PlanRevert` 的调度、临时名称、`RevertWrites` 含删除的页、按（页、哈希）的解析、后台撤销的切批（M8/P3、P4）；`changesets.renames`、`moves`；`domain.Client.Valid` 与两张表的检查；编辑锁的守卫对 MCP 的删除（M5 的注册者）；`Search.InNotebook`（M8/P5）；`page.NewNodePaths`、`MCPEntry()`、`Tools()`；`New` 把用例同时交给两个适配器。
- linking：`ListIndex` 与入链数；`Deps.NodePaths`；`Tools()`。
- asset：`(*asset.Module).AttachmentReads()`。
- notebook：`ListAccountNotebooks`、`Tools()`。
- identity：`api_tokens.client_name`、`identity.NewClientNames(pool)`。
- workspace：`mcp` 移到 `[server]`。
- 平台：`platform/httpserver/mcpserver`；`/mcp` 的 `no-store`；`platform/markdown` 导出 `FrontmatterEnd`；`server.public_url`。

## 9. 测试策略

- **用例与工具**：每个工具的表格测试，码照 4.15 的次序；工具的错误的形状（4.7）；结果的 `content` 与 `structuredContent`。
- **路径与 glob**：4.8 的每条规则、四种情形、三种通配与被拒的写法；glob 线性的分配测试（病态的模式在上限处，13.4 第 8 条）。
- **batch 与撤销**：4.11 的约束、合并的每种、界线（每个上限处）；4.12 的撤销测试与性质测试。
- **grep**：键的提取（每种正则的形状）；键是超集的性质测试；走查与游标；`building`；不进索引的文字只在不取键时找到。
- **整个程序**（13.1 第 21 条，组合根交空时失败）：
  - 工具清单等于 `api/mcp/tools.json`（两个入口），`-update` 重写；
  - PAT：没有、无效、撤销的 401，访问令牌的 403，看不到与不是 UUID 的笔记本 404；
  - NUL 与不成对的代理项：每个工具的每个字符串参数取 NUL（工具的错误，不是 `internal_error`）与不成对的代理项（400），照 `TestFreeTextParametersDoNotAnswer5xx`；
  - 最后一跳：`mcp:<名>`、附件读、路径端口、prompt 的替身；
  - 服务重启之后继续调用；G16 的五种换法之后握手的说明；
  - 锁：4.10 的表的每一格经 MCP。
- **权限矩阵**（13.4 第 6 条）：`bootstrap/permission_matrix_mcp_test.go`：工具 × 笔记本级的 13 列（`notebookColumns()`），每列的身份另有 PAT（种子加 `api_tokens` 的行），经 MCP 调用：看不到的列答 404（笔记本入口在 HTTP，账户入口是工具的 `notebook.not_found`），读者读得到、写答 `forbidden`，写者以上写得到；写的格用库的副本；`notebooks` 一行按列比较列出的笔记本；`TestThePermissionMatrixCoversEveryTool`：清单里的每个工具都有行。
- **并发**：4.15 的交错。
- **e2e**（13.4 第 3 条）：脚本化的 MCP 客户端 `e2e/fixtures/mcp.ts`：原生的 JSON-RPC 经 Node 的 `fetch`（同事件流的 `openEvents`，`e2e/fixtures/events.ts:46-48`），`mcpClient(credential, {notebook?, client})` 先 `initialize`（clientInfo 是 `e2e-agent`），`call(tool, args)` 答 `{isError, structured, text}`；不加 TypeScript 的 MCP SDK（无状态的 JSON 答复用不着它，也免了 pnpm 的年龄、信任与 knip 的门禁）。MCP 版本调同一组数据库断言：工具的结构答复给出断言要的字段（id、时刻），给不出的经 SQL 读（时刻的精确比较本来就在 SQL 里）；`expectToggleRevision` 一类把客户端的类型放宽到 `mcp:${string}`。
- **人工**：4.16 的接入清单。

**本 M 的故事**（MCP 版本；"页面版本"只在有网页一侧的动作时有；没有 PAT 的 REST 版本：它们的动作只能经 MCP，10.1 的例外）：

| 故事 | 内容 | 版本 |
|---|---|---|
| MC1 接入与说明 | 笔记本入口握手的说明是 `AGENTS` 去掉 frontmatter 的正文；换成另一个 `AGENTS` 页之后的握手是新的；没有时是内置的；访问令牌 403、看不到的笔记本 404；`tools/list` 与清单一致；账户入口 `notebooks` 列出两个工作区里看得到的、`open_notebook` 给说明与概况 | MCP |
| MC2 读与列 | `list` 按文件夹与 glob；`read` 的 offset 与 limit、CRLF 与 BOM 原样、48 KiB 处给 `next_offset`；附件：PNG 是图片内容，PDF 与 SVG 给绝对地址（`nervewikiWith` 配 `server.public_url`），地址下载 200 | MCP |
| MC3 写与编辑 | `write` 补建两层父页并建页（一个变更集，`mcp:e2e-agent`），已有时拒绝，带 `base_revision` 覆盖，过时的版本号 409；`edit` 唯一、多处、`replace_all`、CRLF；网页的历史按 MCP 筛选、显示"MCP（e2e-agent）"，打开的页随事件更新 | MCP、页面 |
| MC4 移动、改名与删除 | 一次 `move` 移动加改名、指向它的链接跟着改（一个变更集）；`update_links=false` 不改写、索引照样维护；有子节点不带 `recursive` 被拒并说出数目；`recursive` 删进回收站，网页的回收站列出它 | MCP、页面 |
| MC5 batch 与整组撤销 | 一个 ingest 样的 batch（建三页、改一页、改名一页、删一页，带说明）是一个变更集；其中一页正被网页编辑时整个被拒、什么都没写、答复说出第几步与持锁人；网页整组撤销一个含互换名称与改名之后新建同名的 batch，树、正文与链接回到之前 | MCP、页面 |
| MC6 grep 与 search | `grep` 正则的行号、glob、`ignore_case`；没有字面量的正则走查并给出继续的游标；`search` 的摘要与计数；MCP 刚写的立即两者都找到 | MCP |
| MC7 index、backlinks 与 log | `index` 按类型、标签、父页面与入链数；`backlinks` 带路径；`log` 按客户端 `mcp` 与按页筛选、`changeset` 的明细、`versions` 的列表与读一个版本 | MCP |

**前面 M 的故事的 MCP 版本**（总体设计 10.1"涉及页面读写的故事另有一个 MCP 版本"；P5）：有 MCP 版本的，故事的动作都能经 M9 的工具做，调同一组断言；豁免的按两类：动作 MCP 没有（编辑会话、接管、上传、导入导出、恢复与撤销、链接的落点与补全），或只有浏览器能证明（渲染、事件流的生命）。

| 故事 | MCP 版本 | 说明 |
|---|---|---|
| PG1 新建页面 | 有 | `write` 新建 |
| PG2 改名 | 有 | `move` 改名 |
| PG3 移动 | 有 | `move` |
| PG4 删除子树 | 有 | `delete` 带 `recursive`；不带时被拒 |
| PG5 阅读视图 | 豁免 | MCP 不渲染 |
| PG6 渲染安全 | 豁免 | 同上 |
| PG7 编辑与保存 | 有 | `edit`，之后 `read` |
| PG8 保存冲突 | 有 | 过时的 `base_revision` |
| PG9 字节保真 | 有 | `write` 与 `read`：CRLF、BOM、结尾的空白原样 |
| PG10 编辑会话 | 豁免 | MCP 不开会话 |
| PG11 导航 | 有 | `list` 与 `read` 的路径 |
| PG12 页面权限 | 有 | 读者写被拒、看不到 404、访客 |
| PG13 删除笔记本 | 豁免 | 动作是删除笔记本；之后入口 404 由 MC1 覆盖 |
| PG14 笔记本的活动 | 豁免 | 动作是网页的保存；MCP 的写计入活动由 MC3 断言 |
| C1–C3、C5、C6、C8、C9 | 豁免 | 锁、接管、释放、自动保存、会话过期、事件流：MCP 没有这些动作 |
| C4 守卫 | 有 | 被编辑的页拒绝 MCP 的写与删除，含本人的编辑器 |
| C7 推送 | 有 | MCP 的写经事件到达网页（页面一侧的断言照旧） |
| C10 任务 | 豁免 | 勾选是网页的瞬时写；`edit` 能改但不是这个动作 |
| L1 链接 | 有 | `write` 带链接，索引 |
| L2 新建链接的页 | 豁免 | 动作是链接的落点（网页） |
| L3 改写 | 有 | `move`，`update_links` 两种 |
| L4 标签与公式 | 有 | 标签由 `index` 读；公式的渲染不在 MCP 版本里 |
| L5 补全 | 豁免 | 编辑器 |
| L6 右栏 | 有 | `backlinks` |
| L7 软换行 | 豁免 | 渲染 |
| AS1 上传 | 豁免 | MCP 不上传 |
| AS2 粘贴 | 有 | `write` 嵌入已有的附件，`read` 读它 |
| AS3 渲染 | 有 | `move` 改名附件，嵌入它的链接跟着改 |
| AS4 生命周期 | 有 | `delete` 附件，行随之删除 |
| AS5 内容安全 | 豁免 | 只有浏览器能证明 |
| TR1–TR4 | 豁免 | 导入导出是任务 |
| TB1、TB2 | 豁免 | 恢复是网页的动作；MCP 的删除由 PG4 覆盖 |
| HI1 版本 | 有 | `versions` |
| HI2–HI5 | 豁免 | 还原与撤销是网页的动作；MCP 的 `log` 由 MC7 覆盖 |
| SE1 搜索 | 有 | `search`，限一本笔记本 |
| SE2 搜索的可见性 | 有 | 看不到的笔记本 404；访客 |

21 个 MCP 版本，16 个豁免的故事（C 组合起来算七个）。数据库断言：MC3、MC4、MC5 核对变更集的客户端、说明、条目与版本（`page.ts`）、索引（`links.ts`）；MC2 核对附件的行（`asset.ts`）；MC1、MC6、MC7 不写库（10.1 的例外，同 SE1、SE2）。

## 10. 风险

| 风险 | 缓解 |
|---|---|
| 无状态的 JSON 答复没有在 Claude Code、Codex 上验证过（M0 的实验是有状态的） | 协议里会话是可选的、JSON 答复是客户端必须支持的；P1 在仓库之外用 TypeScript 官方 SDK 的客户端先试；人工清单两种客户端各一遍。真的不通时回到 4.3，退回有状态（每次调用的执行者与期限从 `req.Extra` 重建，4.3 的"放弃的方案"） |
| 旧协议的客户端在重启之后、重新握手之前的调用：名称来自 PAT 记下的；一个 PAT 给两个客户端用时记的是最后握手的那个 | `api_tokens.client_name`（4.5）；接入文档建议一个 agent 一个 PAT；2026-07-28 的客户端每次调用自己带名称 |
| `read` 的 48 KiB 与图片的 3 MiB 对某个客户端太大或太小 | 人工清单核对；常量，M12 按反馈调 |
| 签出的附件地址进了模型的上下文与对话记录，失去访问之后至多 2 小时仍能下载 | 同 M7 的风险；要立刻撤销时轮换签名的密钥；`server.public_url` 为空时不签地址 |
| PAT 无效时客户端去找 OAuth 的 `/.well-known/…`，前端答 `index.html`，客户端的提示难懂 | 接入文档的排错一节；不在 v0.1 加 `/.well-known/` 的路由（它不是合法的 slug，要改保留名单的测试）；负责人可以改判 |
| 回环保护关掉之后 Bearer 是唯一的门 | 总体设计 6.3 定的；PAT 只有哈希入库、可撤销；浏览器发不出不经预检的带令牌请求 |
| 大的请求体（约 36 MiB）在 SDK 与结构检查里各读一遍进内存 | 与 REST 的正文路由同一个量级；按凭证限流；M12 的压测 |
| `grep` 不取键时每次读整本笔记本的页并排序，有键时候选按笔记本的倒排数随键与笔记本增长 | 一次至多 500 页、32 MiB；只读一本；时间的守卫；M12 |
| batch 的约束 B1–B3 拒绝 agent 偶尔会做的组合（新建又删、改名又删） | 文字说明怎样改写成合法的 batch；它们让每个 batch 的变更集都能整组撤销；负责人可以改判为"记下删除时的状态"（要在条目上多存一组状态） |
| 多轮调度与临时名称的正确性 | 纯函数、每种依赖的数据库测试、加了 `batch` 的性质测试；做不完仍是缺陷（500），不会悄悄写错 |
| MCP 的删除比网页的严（本人的编辑器也拒） | 第 11 节第 2 条的延伸，负责人可以改判；文字说"关掉那个编辑器再试" |
| agent 不读或不照 `AGENTS` 的约定 | 说明只是建议；M10 的模式契约与守卫才是强制的 |
| SDK v1.x 仍在快速演进，1.9.0 去掉兼容开关 | 固定 v1.8.0，archtest 管住导入；升级时跑 MCP 的整个程序测试与 e2e |
| PAT 有账户的全部权限 | v0.2 的按笔记本限定的 PAT（总体设计 14） |

## 11. 负责人确认的决定

负责人 2026-10-17 定下四件事：

1. **历史对 agent 只读**：agent 读日志与单个变更集（版本也读，便宜就给）；整组撤销、还原版本、从回收站恢复仍是人在网页上的动作。`batch` 的变更集仍要能在网页上整组撤销，所以 M9 实现 M8/P3 移交的多轮应用与临时名称，以及 M8/P4 移交里跨批的那一项，兑现 5.3"失败的 ingest 可以整组撤销"。v0.1 里 MCP 不开后台任务。
2. **编辑锁**：MCP 的写是一次性的，没有编辑会话；网页上正被编辑的页（包括同一个人自己的编辑器）拒绝 MCP 的写，答 `page.locked` 并说出是谁持锁；agent 不能接管。
3. **`AGENTS` 页**：是笔记本根下标题为 AGENTS 的页，按标题键不区分大小写地找；它去掉 frontmatter 的正文就是 server 的说明，约 32 KB 截断；没有时 server 给一段简短的内置说明：有哪些工具、路径怎么写；server 的缓存键要经得起 `AGENTS` 页被替换、改名、恢复（G16）。
4. **工具与入口**：在 `grep` 之外加一个 `search` 工具：M8 的全文搜索，带摘要与排序；`grep` 是正则、按行命中。接入写进 Claude Code 与 Codex 的文档。网页上显示 MCP 地址的界面等 v0.1 之后的打磨。

下面是设计里的取舍，负责人可以改判：

1. 无状态的 Streamable HTTP 与 JSON 答复，不用 SDK 的会话（4.3）。
2. 客户端名称：调用自己的 clientInfo，否则这个 PAT 最近一次握手报的名称（记在 `api_tokens` 上，重启之后还在），都没有时 `unknown`（4.5）。
3. `/mcp` 只收 PAT，会话的访问令牌答 403（4.4）。
4. MCP 的删除被任何活着的编辑会话拒绝，本人的也算；网页的删除照旧按账户（4.10）。这是第 2 条在删除上的延伸。
5. `batch` 不许删除自己新建的、不许先删子孙再删祖先、不许删除自己改过名或移动过的节点（B1–B3）；正文与 `new_string` 合计至多 5 MiB、至多 100 项、1 万个条目（4.11）。
6. batch 里的改名、移动在最后一次改写链接，batch 自己写的页不改写（4.11）。
7. 路径从根精确寻址，不照链接的规则做后缀匹配；四种情形照 4.8 的表；glob 只有 `*`、`?`、`**`（4.8）。
8. `read` 一次至多 48 KiB 文字；PNG、JPEG、GIF、WebP 在 3 MiB 之内内联，SVG 与别的给地址；地址要配置 `server.public_url`，不按请求的 `Host` 补全（4.9、4.2）。
9. `grep` 在 Go 里以 RE2 按行匹配；取不出键的正则有界地走查（每次至多 500 页、32 MiB），不拒绝（4.13）。
10. `search` 与 `grep` 一次一本笔记本，不跨笔记本、工作区（4.13）。
11. 第 9 节表里豁免 MCP 版本的故事与理由。
12. 真实客户端的接入清单与输入法清单一样：负责人在 v0.1 的最后统一执行，通过之后 M9 才改为已完成（4.16）。
13. 不为 `/.well-known/` 加路由（第 10 节）。

## 12. Phase 进度表

| P | Phase 文档 | 状态 | 审查 | 合并 |
|---|---|---|---|---|
| P1 平台与读的工具 | — | 未开始 | — | — |
| P2 写的工具 | — | 未开始 | — | — |
| P3 batch 与多轮撤销 | — | 未开始 | — | — |
| P4 grep、search、index、backlinks | — | 未开始 | — | — |
| P5 前面的故事的 MCP 版本与接入文档 | — | 未开始 | — | — |
| 收尾 | — | 未开始 | — | — |

## 13. 变更记录

| 日期 | 变更 | 依据 |
|---|---|---|
| 2026-10-17 | 初稿 | 移交 9 份、代码的调查（`fff7c560`）、负责人 2026-10-17 的四个决定（第 11 节） |
