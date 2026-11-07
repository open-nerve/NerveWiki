# M10 / P5：MCP playbook 与按模式的说明

| 项 | 内容 |
|---|---|
| 状态 | S1 已合并 `ba1fd4c9`，CI 的两处修复已合并 `bbeb9fbb` |
| 权威 | [00](00-M10-design.md) §4.15、§4.19、§5、§7，故事 LW1、LW11；[总体设计](../v0.1-design.md) §5.9、§12.4、§13；[M9 移交](handoffs/M9-P1-mcp-registry.md)；[02-workflow](../claude_to_codex/02-workflow.md) |
| 基线 | `c5fff600`（M10/P4 done），tree `d2fc220e` |
| Step | S1，一个能单独合并为绿的 Step |
| 审查 | [设计审查](reviews/P5-design-review.md)、[S1 审查](reviews/P5-S1-playbooks-review.md) |

## 0. Step 的划分

只有一个 Step。S1 同时交付：Prompt 注册表补齐、三份英文 playbook、两个入口共用的说明、`open_notebook` 的描述、真实 SDK 与组合根的验证、LW11 与 LW1 的增量，以及 README、M9 人工清单、移交和总体设计的文字。

P5 不运行 LLM，不加资源注册体系、Completion、账户入口的 prompt、公开配置、数据库结构，也不做 M11 的事。

## 1. 基线

三位审查者在这个基线上写过原型，下面的事实都实测或对代码核过。

1. MCP 用 go-sdk v1.8.0。平台已有 `Prompt`、字符串参数、`Get`，以及名称、重名、空处理函数的启动检查。组合根还没有传 `Options.Prompts`，`newEntry` 按全局列表给两个入口声明能力，`addPrompts` 登记两个入口。
2. `lint`、`pending_sources` 已由 wiki 在两个入口提供。playbook 不靠执行 lint 取得。
3. 笔记本入口的握手和账户入口的 `open_notebook` 各自选 AGENTS 或内置文字。缓存只存 `(notebook, node, revision)` 的正文和截断点，至多 256 项，每项 32 KiB，总正文至多 8 MiB。它的修订号、删除竞态和 LRU 语义都不变。
4. `PageRead` 已是真实页与虚拟页的联合：真实页的 version 必填，虚拟页没有 version。00 §4.15 里"`version` 改为可选"的旧句不再实施。
5. 内置说明实测：笔记本入口 raw 3044、SDK 3093 字节，账户入口 3113、3163 字节，离 4 KiB 剩 1003、933 字节。P5 不改内置的两份，余量远超留给 M11 的 300 字节。
6. 模板的英文 AGENTS 第一句就是 "This notebook is an LLM Wiki: people add sources…"，与系统段开头的字一样。测试判断系统段有无时不能用这个前缀（§7.1）。

## 2. 目标与范围

可读 LLM Wiki 的 agent 在笔记本入口拿到三份可执行的工作说明。普通笔记本静态列出同样三项，取用时答错误。账户入口只在 `open_notebook` 的说明里指向笔记本入口。工具、写权限、来源守卫、ingest 的资格与预算都沿用已有的实现。

### 2.1 对 00 号文档的修订

这次设计提交一起修订 00（02 §2 第 3 步的脚本，不进仓库）：

1. §4.15：系统段放在 AGENTS 的正文之前（决定 161）。原文只写"加一段"，没有定次序。
2. §4.15：答复的界算上 `description`，`TextBytes` 的规则写准：`\n`、`\r`、`\t`、`\b`、`\f`、引号、反斜杠 2 字节，其余 C0 与 `<`、`>`、`&` 6 字节。参数只在消息里回显一次。
3. §4.15：`PageRead.meta` 的 `version` 那半句删掉，见 §1 第 4 条。
4. §4.15：`prompts/get` 的日志只记已登记的 prompt（决定 162）。
5. 第 11 节加决定 161–165，第 12、13 节写 P5 的设计。

### 2.2 需要负责人确认

没有。

### 2.3 主持者的决定（负责人可以改判）

**决定 161，系统段放在正文之前。** 系统段放在末尾时，客户端截断说明，最先丢的就是它。Claude Code 公开的更新记录写 2.1.84 起给 server instructions 设了上限，后来默认改为 4,096 字符；这是文档上的说法，没有在真实客户端上实测。服务端能保证的只是自己不截掉它，放在前面对任何客户端都不更差。预算代码不变，只改拼接的次序；截断提示留在末尾。

**决定 162，SDK 不认识的 prompt 不记调用日志。** 未知的工具今天也没有 `MCP tool call` 这一行，00 说的是"同工具的调用"。这样日志放进 `promptHandler` 的 defer 就够了，不要包在 SDK 外面的一层，也不用定义 `invalid_name`。

**决定 163，空的父页只改 playbook，不改 lint。** `write` 给 `Concepts/K.md` 建的空父页 `Concepts.md` 进 `missing_type`，填上类型又进 `orphans`，还要 agent 为一个空文件夹编元数据。两条规则是 00 §4.11 定的，这里不动，playbook 教 agent 保持它为空并在报告里说明。

**决定 164，query 不主动写页。** 00 说"值得留下的答案另写一页"，playbook 写成"只有请求要求时才写"。回答问题不该未经要求就改知识库。

**决定 165，`prompts/get` 进权限矩阵。** 13.1 第 3 条要矩阵覆盖每个操作。`mcpMatrixViolations` 只看工具，加一个小测试，同 `TestTheWikiToolsReadLLMWikiMatrix`。

## 3. 服务端

### 3.1 文件表

没有 SQL、迁移和 OpenAPI 的改动。`tools.json` 只改 `open_notebook` 的描述（§3.9）。

| 文件 | 改什么 |
|---|---|
| `api/mcp/prompts.json`（新） | 两个入口的 prompt 全文，§3.3 |
| `api/mcp/tools.json`、`page/adapter/mcp/open_notebook.go` | `open_notebook` 的描述，§3.9，同一个提交 |
| `mcpserver/{prompt.go,server.go,log.go}` | `Entry`、按入口的能力、参数与答复的界、分类、调用日志 |
| `mcpserver/errors.go` | `methodError` 的注释改：握手现在会答 `notebook.not_found`（§3.7） |
| `mcpserver/{prompt_test.go,handshake_test.go,call_test.go,tools_test.go}`，新 `prompt_bounds_test.go` | 平台的测试。`call_test.go:431` 的 `panicky` 要加 `Entry`，否则 `New` panic，同包的测试都跑不下去；`tools_test.go:386-387` 的三个 Prompt 也要加，否则那张表的子测试失败 |
| `wiki/{module.go,lint.go}`，新 `wiki/prompts.go`、`wiki/app/playbooks.go`、`wiki/domain/playbook.go` 与 `wiki/domain/playbook-{ingest,query,lint}.md`、`wiki/adapter/mcp/prompts.go` | 模块根的 `Prompts()`、读用例、三份正文 |
| `page/{mcp_entry.go,module.go}`、`page/app/{notebook_instructions.go,open_notebook.go}`、`page/adapter/mcp/{notebook_instructions.go,open_notebook.go}` | 说明的资料和唯一的拼接函数。`app/instructions.go` 的缓存、`domain.AgentsBody`、`AgentsText` 不变 |
| `page/mcp_entry_test.go`、`page/app/{open_notebook_test.go,virtual_list_test.go}`、`page/adapter/mcp/*_test.go` | `MCPEntry` 的字面量要加字段；`TestOpenNotebookAnswersInOrder`、`TestVirtualListing/overview` 的夹具根 id 是随机或零值，要建真的根节点 |
| `bootstrap/{mcp.go,wire.go}` | `mcpServer` 加 `prompts` 参数，`wire.go` 把 `wk.Prompts()` 交给 `Options.Prompts` |
| `bootstrap/batch_cost_test.go:63` | `mcpServer` 的第二个调用者，随参数改 |
| `bootstrap/{mcp_prompts_test.go（新）,mcp_test.go,mcp_instructions_test.go,mcp_account_test.go,wiki_downgrade_test.go}` | 最后一跳、能力、三个处理函数、共用的说明、降级。`mcp_account_test.go:90` 的 `TestNotebooksAnswerTheirModeOverMCP` 钉住 LLM Wiki 的说明等于 AGENTS，要改 |
| `e2e/fixtures/mcp.ts`、`e2e/stories/wiki/{lw11-playbook.spec.ts（新）,lw1-create.spec.ts}` | §5 |
| §8 列的文档 | 按最终事实改 |

只复用 notebook 的 `WorkspaceOf`、`StateOf`，page 的 `AgentsPage`、`PageContent`、`FindNodeIn`、`Ancestors`。模块不直接引入 go-sdk。

### 3.2 注册表与签名

```go
// mcpserver：Prompt 加一个字段，Get 的签名不变。
type Prompt struct {
    Name, Title, Description string
    Entry                    Entry
    Arguments                []PromptArgument
    Get                      func(context.Context, Call, map[string]string) (string, []string, error)
}

// wiki：模块根与 adapter。
func (m *Module) Prompts() []mcpserver.Prompt
func mcpadapter.Prompts(p PlaybooksUseCase) []mcpserver.Prompt

// wiki/app：Playbook 是 wiki/domain 的 ingest、query、lint 三个常量。
func NewPlaybooks(notebooks Notebooks, auth shared.Authorizer) *Playbooks
func (p *Playbooks) Execute(ctx context.Context, notebook uuid.UUID,
    kind domain.Playbook, argument *string) (string, error)
```

`Module` 加一个 `Playbooks` 字段，`New(Deps)` 用已有的依赖建它。组合根的 `mcpServer` 加 `prompts []mcpserver.Prompt` 参数，不用可选的类型断言。

Prompt 的名称仍是 1–32 个 `[a-z_]`，重名仍 panic。`Entry` 为 0 或含 `BothEntries` 以外的位时 panic。`newEntry` 拿到这个入口的 prompt，有才声明 `prompts`，`listChanged` 为 false。M10 的三项都是 `NotebookEntry`：两种模式的笔记本列表相同；账户入口 initialize 没有 `prompts` 能力，列表是非 nil 的 `[]`，取用由 SDK 答 -32602 unknown prompt，处理函数不被调用。

### 3.3 合同与参数

`api/mcp/prompts.json` 的全文如下，`TestThePromptListsAreTheContract` 比较整个对象。次序是 SDK 的名称次序。`lint` 没有 `arguments`，可选参数不写 `required`。文件要过 oxfmt，`arguments` 按格式器的写法分行。

```json
{
  "notebook": [
    {
      "name": "ingest",
      "title": "Ingest a source",
      "description": "Turn an LLM Wiki source into maintained pages, in one batch, then check the result. LLM Wiki only.",
      "arguments": [
        {
          "name": "source",
          "description": "The source path in this notebook. Omit it to list pending sources first."
        }
      ]
    },
    {
      "name": "lint",
      "title": "Check and maintain the wiki",
      "description": "Check an LLM Wiki, make justified mechanical repairs, inspect recent changes, and report. LLM Wiki only."
    },
    {
      "name": "query",
      "title": "Answer from the wiki",
      "description": "Answer a question from this LLM Wiki with references to its pages and sources. LLM Wiki only.",
      "arguments": [
        {
          "name": "question",
          "description": "The question to answer from this notebook.",
          "required": true
        }
      ]
    }
  ],
  "account": []
}
```

取用答同一个静态 description 和一条 `role: user` 的文字消息。正文见附录 B。`ingest`、`query` 的参数只在消息末尾原样附一次，description 不回显。`argument == nil` 是没有提供；空字符串不改成 nil，也不当缺失。用例不读来源、不解码路径、不调用 LLM、不写库，之后 agent 调用的 `read`、`batch` 自己校验与授权。

SDK 解成 map 之后分不出 null 与空字符串：`{"question":null}` 过 `Required`，处理函数收到 `""`；`{"source":null}` 算提供了空串，而工具把 null 当缺席。这是 SDK 的形状，不处理。

### 3.4 错误次序

| 次序 | 发生在哪里 | 结果 |
|---|---|---|
| 1 | HTTP 边缘：请求体大小与形状、PAT、`notebook` 查询参数、`NotebookGate` | 全部保留。认证失败、笔记本看不到都在 SDK 之前，看不到是 HTTP 404 的 problem，不是 JSON-RPC |
| 2 | SDK 解码参数 | 不进中间件，没有调用日志。无 params 是 HTTP 400，`params:null` 是 -32600，非字符串参数是 -32602，SDK 把整个 params 回显给调用者。所以"不回显值"和 48 KiB 只约束成功路径和第 4 步，这是 `edge.go` 已接受的 M9 行为 |
| 3 | SDK 找 prompt | 账户入口和未知名称答 SDK 的 -32602，处理函数不调用，不记调用日志（决定 162） |
| 4 | `mcpserver` 按声明次序查参数 | 逐个参数先查 `Required` 的键，再查这个参数的字节数 ≤ 4,096。两者都是 -32602，`message` 写参数名与界，不回显值，`data` 是 `{"code":"validation_failed"}`，都不调用 `Get`。`validation_failed` 在 `shared` 里，`mcpserver` 不导入 shared，用私有常量 |
| 5 | wiki 用例：`RequireActor` → `WorkspaceOf` → `Authorize(lint.read)` → `StateOf` | 不启动 Runs、快照或 lint。`WorkspaceOf` 没有、`Authorize` 答 `ErrNotVisible`、`StateOf` 没有都是 `notebook.not_found`（`ErrNotVisible` 自己的码是空的，要换成这个码），普通笔记本是 `ErrNotLLMWiki`，端口的错误保持原因 |
| 6 | 读静态资源、拼一次消息 | `Get` 出错时不发送它一起返回的部分内容 |
| 7 | 平台数 `TextBytes(description) + ΣTextBytes(消息)` | 超 48 KiB 是缺陷，-32603，不截成成功，只有带缺陷的处理函数走得到。这里记一行 error，带 prompt、字节数与 `request_id` |

第 4 步在第 5 步之前，所以普通笔记本加 4,097 字节的参数答 `validation_failed`，不答 `notebook.not_llm_wiki`。这是 00 §4.15 的"处理函数之前"，与 lint 规则页的次序不同，不算矛盾。

`Get` 的错误经 `httpserver.ProblemOf` 分类。一个小函数只写 `FaultNone`：4xx 是 -32602，5xx 是 -32603，`message` 是 `<code>: <detail>`，`data` 是 `{"code":…}`。普通笔记本答 `notebook.not_llm_wiki: The notebook is not an LLM Wiki.`。其余的 `return s.methodError(...)`，它的合同不改：fault 不带 `data`，`FaultGone`、`FaultDeadline`、未知分别记 debug、warn、error。panic 由 `outer` 恢复，答 `{"code":-32603,"message":"internal error"}`。

`RequireActor` 失败、`forbidden`（`node.list` 与 `lint.read` 都是读者以上）在生产到不了，由 wiki 的单元测试覆盖。

### 3.5 界

- 三份正文各至多 16 KiB，只有英文，`go:embed`，没有公开的配置。
- 每个已声明的参数至多 4,096 字节。整个 POST 仍至多 `MaxOtherBytes`（64 KiB），`prompts/get` 没有 `tools/call` 那样的例外：边缘的条件是 `len(body) <= MaxOtherBytes`，65,536 字节放行，65,537 字节答 413。
- 成功答复的 `TextBytes(description) + ΣTextBytes(消息)` 至多 48 KiB。真实上限约 27.9 KB，因为正文小、参数只回显一次。
- `TextBytes` 用 `answer.go` 现有的规则，与真实 SDK 的线上字节实测相等，只有 `<`、`>`、`&` 比握手的线上字节多（线上是 1 字节），这是偏保守。
- 请求期限、48 KiB 的工具答复、4 KiB 的内置说明、留给 M11 的 300 字节都不变。

### 3.6 日志

调用日志放进 `promptHandler` 的 defer：prompt、入口、`notebook_id`、`duration_ms`、`outcome`，和已有的请求、凭证字段。已登记 prompt 的成功、验证拒绝、业务拒绝、panic 各一行。`outcome` 先设成 `internal_error`，正常返回之前才改，所以 panic 那一行是 `internal_error`；不吞 panic，`outer` 照旧恢复。不增加 args、source、question、消息或 description 字段。

### 3.7 说明的资料

缓存的 `Instructions.Execute(ctx, nb)` 保留。加一个不缓存、不做授权的 `Instructions.Read`：调用 `Execute` 得正文；只有 LLM 模式才接着 `FindNodeIn(nb, SourceRootID)` 和一次 `Ancestors(SourceRootID)`，得到根的标题和祖先名。`Ancestors` 从根到父、不含自身，所以 `FindNodeIn` 不能省。按树路径连接这些名字，不给根加 `.md`。

```go
// page/app：一次请求自有的资料，不是缓存，也不是公开的 DTO。
type NotebookInstructions struct {
    NotebookID                      uuid.UUID
    Mode                            domain.Mode
    Agents                          domain.Agents
    HasAgents                       bool
    SourceRootPath, SourceRootTitle string
}
func (i *Instructions) Read(context.Context, uuid.UUID, domain.NotebookState) (NotebookInstructions, error)

// page/adapter/mcp：两个入口都调这个纯函数，fits 判断完整的拼接文字。
func NotebookText(i app.NotebookInstructions, entry mcpserver.Entry,
    fits func(string) bool) (string, error)
```

- `MCPEntry` 持有 `NotebookStates`。`Gate` 放行之后 `StateOf` → `Instructions.Read` → `NotebookText`。`StateOf` 答没有时，同 `OpenNotebook` 答 `ErrNotebookNotFound`，经 `methodError` 是 -32603 加 `notebook.not_found: …`，不能把零值当普通笔记本。
- `OpenNotebook.Execute` 保持 `Gate` → `StateOf` → 计数 → 最后改动 → 根的节点 → AGENTS 的次序，把最后一步换成 `Read`。来源根的读只在 AGENTS 成功之后。`Opened` 用一个 `Instructions NotebookInstructions` 值代替 `Agents`、`HasAgents`，并去掉它重复的 `Mode`，只留一处。REST、工具答复和 `instructions_from` 没有字段变化。
- 这些读不加事务和锁。设计接受三种并发结果，都在下一次请求恢复：降级落在 `StateOf` 之后，答一次带系统段的说明；一步移动加改名落在 `FindNodeIn` 与 `Ancestors` 之间，答一次从未存在过的混合路径；根或笔记本在读之间被删，`app.ErrNotFound` 不是 problem，握手答 -32603 并记 error，`open_notebook` 答 `internal_error`。握手的说明本来就在整个连接期间不刷新，来源守卫按节点 id 判，不靠这句话。
- 根读取失败保持原因，不静默降成普通说明。

### 3.8 拼法与预算

1. **系统段**。普通笔记本是空的。LLM Wiki 用下面的英文句，完整一句 raw 至多 512 字节，512 是 raw，不是编码之后的字节。放不下路径时换成根的标题；还放不下就只截这个变量，按 UTF-8 的完整字符加 `…`。固定的前后句、反引号与 `/` 不截。每次重算，不进 AGENTS 的缓存。
2. **拼接**。LLM Wiki 是 系统段 + 两个换行 + AGENTS 的正文（决定 161）。正文经原来的 `AgentsText`，它的 `fits` 量的是完整的拼接文字，所以先给系统段和截断提示留出位置。系统段为空时不加换行，普通笔记本的文字逐字不变。没有 AGENTS 或正文为空时用内置说明，同样拼上系统段。账户入口全局的 initialize 仍是 `AccountInstructions`，它不指定某一本。
3. **两个预算**。`NotebookText` 保证完整文字至多 32 KiB `TextBytes`，并满足调用处的 `fits`。握手传一个总为真的 `fits`，32 KiB 由 `NotebookText` 自己守。`open_notebook` 的 `fits` 还要 `MemberBytes` 不超过 48 KiB 减去实际的空骨架（含 `instructions_from`、mode、计数与 `last_changed_at`）再减 8 KiB 的根预留。
4. **放不下**。`AgentsText` 放不下时无条件 `return note`，不给信号，所以 `NotebookText` 在它返回之后再量一次完整的文字，内置说明那一支也量。不成立时返回内部错误，不丢系统段。比提示短的完整正文放得下时照常成功。生产到不了：最坏要约 5 KB，预算是 32 KiB 和约 40 KB。
5. `instructions_from` 只表达 AGENTS 的来源，有正文时是标题加 `.md`，回退时是 null。

笔记本入口的系统段：

> This notebook is an LLM Wiki: pages under `<source-root path>/` are sources you add once and never change; index.md and log.md are generated; run the prompts ingest, query and lint.

账户入口的系统段：

> This notebook is an LLM Wiki: pages under `<source-root path>/` are sources you add once and never change; index.md and log.md are generated; its prompts ingest, query and lint are served at `/mcp?notebook=<id>`.

固定部分 163、226 字节，所以路径到 349、286 字节时还放得下，再长一个字节就退成标题。两个入口的界不同，同一条路径可能一边保留、一边退标题。255 字节的标题是 418、481 字节。合法的标题到不了 `…` 这一支，只有纯函数的测试走它。

### 3.9 `open_notebook` 的描述

`openNotebookDescription` 与 `tools.json` 同一个提交改，`TestTheToolListsAreTheContract` 比较整个对象。把

> its instructions for agents (its root page AGENTS without its frontmatter, or the server's built-in ones when it has none)

改成

> its instructions for agents (in an LLM Wiki a system paragraph first, then its root page AGENTS without its frontmatter, or the server's built-in ones when it has none)

## 4. 网页

没有新的路由、表单、SWR 或设置，prompt 只经 MCP 提供。业务错误复用 `notebook.not_llm_wiki`，没有未翻译的新码。`unresolved.*` 等文字不变。

## 5. e2e

`e2e/fixtures/mcp.ts` 把 `send` 拆成 `exchange` 加 `send`，加 `listPrompts()` 和 `getPrompt(name, args?)`。后者返回 `{result} | {error}`，只有它接受 RPC 错误，`send`、`listTools`、`call` 的严格判断不变。类型不导出。

- **LW11 (MCP)** 新的 `lw11-playbook.spec.ts`：REST 建真实的 LLM Wiki、来源和独立的 PAT。笔记本入口的列表与 `prompts.json` 全对象相等，比 `.prompts`，不比整个 result。`ingest` 带真实的来源路径，第一行和路径可见；`query`、`lint` 各有自己的第一行。普通笔记本列表仍是三项，取用答确切的 -32602、`data.code` 和 message。账户入口列表为空，取用答 unknown，没有 `data`。未知与普通笔记本都是 -32602，要比 `data.code`。只读，不写数据库。
- **LW1 (MCP)** 保留原来的断言，握手从"等于 AGENTS"改成"等于系统段、两个换行、AGENTS 的正文"，账户入口的 open 另核带实际 id 的句子。
- `TestTheDowngradeEndsTheContract` 加一步：降级之前先握手一次，让同一 AGENTS 版本缓存命中；真实降级之后，下一次 initialize 和下一次 `open_notebook` 都没有系统段。

LW1、LW11 不在 `threeBrowsers` 里，只跑 Chromium。LW11 不开浏览器。

`e2e/fixtures/server.ts` 起服务时再等 `jobs started` 这一行（决定 167）。

## 6. 实施步骤

设计提交在先。三个产品提交，每个都过 02-workflow 的最低要求：build、vet、模块的 lint、短测试；第 2、3 个改了 e2e 的 TS，再跑 `pnpm -r run check:types`、`check:format` 与 `make knip`。

1. **`server: register prompts by entry, with bounds, kinds and logs (M10/P5/S1)`**：`Entry`、按入口的能力、参数与答复的界、分类、日志，平台的真实 SDK 测试。`call_test.go`、`tools_test.go`、`prompt_test.go` 的替身加 `Entry`，保留原来两个入口的意图。生产还没有 prompt，组合根的期待不用动。
2. **`server, e2e: the three playbooks over MCP (M10/P5/S1)`**：`prompts.json`、三份正文、wiki 用例与模块根、组合根接线，一起落；能力、列表、最后一跳和权限的测试同步，加 `listPrompts`、`getPrompt` 和 LW11。先证明测试编译得过并且因行为失败，再实现。
3. **`server, e2e, docs: one set of instructions for both entries (M10/P5/S1)`**：资料读取、唯一的拼接函数、`Opened` 的内部构造、`open_notebook` 的描述、两个入口的预算、说明缓存的测试（M9 的 G16）与降级、LW1 的增量，以及 README、M9 清单、M11 移交的用法。

审查的修复之后，再按 02 的 §3.5 做文档提交。

## 7. 测试与验证

### 7.1 测试

| 测试 | 必须断言 |
|---|---|
| 平台 `TestPromptEntries` | 0、未知位、名称、重名、nil 启动 panic；三种 Entry 实际登记；能力是确切的对象，没有 `listChanged`；账户入口是非 nil 的空列表；未知名称时 `Get` 0 次 |
| 平台 `TestPromptArgumentsBeforeGet` | 缺键拒绝，有键的空串进入；`source` 缺与空串不同；4,096、4,097 的 ASCII 和多字节边界；60 KiB 的请求没有过 64 KiB，`Get` 0 次；未知参数照旧透传；缺键与超界的 `message`、`data` |
| 平台 `TestPromptAnswersUseSDKTextBytes` | 经真实 SDK：一条和多条消息，`description` 加消息 ASCII 共 49,152 成功、加 1 失败；`description` 单独过界；**`description` 放 4,096 个 `<`、消息放 4,096 个恰好 49,152 成功，消息放 4,097 个失败**（`escaped_over`）；`>`、`&`、NUL、短转义、U+2028、U+2029、引号、反斜杠；过界没有部分成功，有那一行 error |
| 平台 `TestPromptRequestBodyHasNoException` | **填充放未声明参数**：65,536 字节答 200，`Get` 1 次；65,537 字节答 413、`payload_too_large`，`Get` 0 次。填充放已声明参数会被第 4 步挡住，抓不到 P09 |
| 平台 `TestPromptErrorsAndLogs` | 4xx、5xx 的 problem 与 `data`；普通错误；真实的取消、到期；活的 ctx 上裸的 Canceled、DeadlineExceeded；未知；panic 的形状（无 `data`），那一行的 `outcome` 是 `internal_error`；日志级别；已登记的 prompt 一行调用日志；日志从子测试开始之后切片，传独特的 source、question，任何字段都不含它们。`client_cancel` 要轮询日志，分类日志晚于客户端返回和处理函数退出，同 `TestTheInstructionsAreReadWithinTheCallsDeadline` |
| wiki `TestPlaybooksAuthorizeAndRequireMode` | 没有 actor、未找到、看不到（答 `notebook.not_found`）、拒绝、状态错误、普通笔记本、LLM Wiki 各自最先答的错误与调用次序；用假授权者判 `Authorize` 确实被调用；不碰 Runs；参数不裁不改；三份正文的第一行、各自的关键句、不超 16 KiB |
| 组合根 `TestThePromptListsAreTheContract` | 真实的两个入口、两种模式，整个对象比较，3 项与 0 项；能力是确切的对象，笔记本入口 `{"prompts":{},"tools":{}}`，账户入口 `{"tools":{}}`；比较器对少一个、多一个、说明不同要报错 |
| 组合根 `TestTheWikiPromptsAreReached` | `New` → `wk.Prompts` → `Options` → 三个真实的 `Get`；第一行字面，`source`、`question` 只出现一次；普通笔记本确切的 RPC 错误；普通笔记本加 4,097 字节答 `validation_failed`；账户入口 unknown；空的 source、带空白的 source 原样到达，没有哪个 prompt 声明的键不进正文；看不到的笔记本是 HTTP 404 的 problem（`NotVisible` 的映射只由 wiki 的单元测试覆盖） |
| 组合根 `TestThePromptReadsLLMWikiMatrix` | 决定 165：`prompts.json` 的每个 prompt × 13 列的角色 × 两种模式，78 格，同 `TestTheWikiToolsReadLLMWikiMatrix`。答复要有这个 prompt 的第一行，没有 `data.code` 的错误不算答复，所以读者能取用在这里判 |
| page `TestNotebookInstructionsShareTheirBudget` | 普通笔记本逐字不变；两个入口的系统句；AGENTS、BOM、frontmatter、空的回退、截断提示；**按 SDK 真实的文字量判**（同 `TestTheBuiltInInstructionsAreShort` 的量法），不用 `TextBytes` 自己判自己：32,768 恰好整段保留，多 1 字节截断而系统段保留；实际的 open 骨架加 `MemberBytes`、8 KiB 预留、最终 SDK 不超 48 KiB；路径 349、350 与 286、287 字节四个点；`…` 这一支用明确标为超长的纯函数输入；`no_room`：纯函数在所需字节数减 1 时答错误、等于所需时答系统段加提示，AGENTS 与内置说明两支都测 |
| M9 的说明缓存测试（G16）加 `TestTheWikiInstructionsFollowTheirSourceRoot` | 真实 SDK：缓存命中与新版本；来源根实际改名、移动之后两个入口都是新路径；10 层 255 字节退成标题 |
| page/app `TestInstructionsReadTheSourceRoot`，page `TestMCPEntryAnswersItsState` | 用假端口：普通笔记本不读根；只读一次祖先，没有 `ListNodes`；AGENTS、读、祖先的真实错误保持原因；`StateOf` 答没有时握手答 `notebook.not_found` |
| `TestTheDowngradeEndsTheContract` | §5 那一步 |
| 原 `TestTheBuiltInInstructionsAreShort`、`NameEveryTool`，LW11、LW1 | 每个入口 SDK 至多 3,796 字节，每个工具一行 |

**系统段的判断要精确**（§1 第 6 条）：有，就是 `HasPrefix(整句 + "\n\n")`；没有，就是 `== AGENTS 的正文`。握手的 SDK 字节写 `≤`，不写 `==`。

### 7.2 交错

根改名、移动、AGENTS 更新与降级都在写完成之后才发下一次 initialize 或 open，这样顺序是确定的，验证缓存命中而系统段是新的。AGENTS 读取中的修订与删除竞态继续用 M9 的说明缓存测试。§3.7 的三种并发结果用假端口钉住结果和日志级别，不逼真实的竞态。取消的测试在处理函数真正进入屏障之后才取消。

### 7.3 变异

每一行是一个单点的改坏，健康 → 改坏 → 复原。编译失败、准备失败不算杀死。不运行 `make gen*`。

| 编号 | 改坏 | 抓住它的测试 |
|---|---|---|
| P01 | wiki 的列表去掉 `query` | `TestThePromptListsAreTheContract` |
| P02 | 多登记一个合法名字的 prompt | 同上 |
| P03 | 只改 `ingest` 的 description | 同上 |
| P04 | 组合根不交 `wk.Prompts` | `TestTheWikiPromptsAreReached` |
| P05 | 一个产品 prompt 的 `Entry` 改成两个入口 | `TestThePromptListsAreTheContract`：账户入口列表非空、能力带 `prompts` |
| P06 | 超长参数仍调用 `Get` | `TestPromptArgumentsBeforeGet/over4096` |
| P07 | 答复的界漏 description | `TestPromptAnswersUseSDKTextBytes/description_over` |
| P08 | 答复的界改用 raw 长度 | 同上 `/escaped_over` |
| P09 | 给 `prompts/get` 大请求体的例外 | `TestPromptRequestBodyHasNoException/over65536` |
| P10 | 4xx 的 problem 一律 -32603 | `TestPromptErrorsAndLogs/problem4xx` |
| P11 | `FaultGone` 记 error | 同上 `/client_cancel` |
| P12 | 调用日志加入参数 | 同上 `/no_arguments` |
| P13 | wiki 绕过普通笔记本的拒绝 | `TestTheWikiPromptsAreReached/standard` |
| P14 | 账户入口用笔记本的系统句 | `TestNotebookInstructionsShareTheirBudget/account`，要经 `openNotebook` 走，直接测 `NotebookText` 抓不到调用点的错 |
| P15 | 系统段换成固定的路径 | `TestTheWikiInstructionsFollowTheirSourceRoot/rename` |
| P16 | 先把 AGENTS 占满再加系统段 | `TestNotebookInstructionsShareTheirBudget/at_bound` |
| P17 | open 的预算用 `TextBytes` 代替 `MemberBytes` | 同上 `/member_escaping`，现有 `TestOpenNotebookFitsItsAnswer` 也抓 |
| P18 | 降级后仍加系统段（state 记在 `MCPEntry`） | `TestTheDowngradeEndsTheContract` |
| P19 | 丢掉祖先，只用标题 | `TestTheWikiInstructionsFollowTheirSourceRoot/move`，`/rename` 抓不到 |
| P20 | 根的资料按笔记本记住 | 同上 `/rename`，这才是缓存语义的单故障 |
| P21 | open 的 `fits` 只量正文，不量系统段 | `TestNotebookInstructionsShareTheirBudget` 的 8 KiB 预留，答复仍在 48 KiB 内 |
| P22 | 512 的 `≤` 改 `<` | 同上，349、286 字节的点 |
| P23 | `newEntry` 仍按全局声明能力 | `TestPromptEntries`、`TestThePromptListsAreTheContract` |
| P24 | `Playbooks` 去掉 `Authorize` | `TestPlaybooksAuthorizeAndRequireMode`，组合根的 Gate 先答 404，抓不到 |
| P25 | 根的路径加 `.md` | `TestTheWikiInstructionsFollowTheirSourceRoot` |
| P26 | 空 AGENTS 的回退漏掉系统段 | `TestNotebookInstructionsShareTheirBudget` |
| P27 | `NotebookText` 去掉 `AgentsText` 之后的再量 | `TestNotebookInstructionsShareTheirBudget/no_room`，生产到不了，只有纯函数的测试抓得到 |
| P28 | 握手丢掉 problem 的码 | `TestAProblemOfTheInstructionsIsTheHandshakesError` |
| P29 | `Call` 不带客户端 | `TestPromptCallKnowsItsCaller` |
| P30 | 日志行不带客户端 | 同上 |
| P31 | 500 的 problem 答 -32602 | `TestPromptErrorsAndLogs/problem500` |
| P32 | 名字为空的键当成参数 | `TestThePromptsPassTheirArgument` |
| P33 | LLM Wiki 里每次取用都失败 | `TestThePromptReadsLLMWikiMatrix` |
| P34 | `query` 绕过用例 | 同上，普通笔记本的列 |
| P35 | 没有 AGENTS 时不读来源根 | `TestInstructionsReadTheSourceRoot`、`TestTheWikiInstructionsFollowTheirSourceRoot` |
| P36 | 握手读失败时答内置说明 | `TestTheInstructionsShareNoTransaction` |
| P37 | `open_notebook` 吞掉读的错误 | `TestOpenNotebookKeepsTheInstructionsError` |
| P38 | 512 按转义之后的字节算 | `TestNotebookInstructionsShareTheirBudget/path_at_bound` |
| P39 | `WorkspaceOf` 的失败答 `notebook.not_found` | `TestPlaybooksAuthorizeAndRequireMode` |
| P40 | 处理函数一律以为是笔记本入口 | `TestPromptCallKnowsItsCaller` |
| P41 | 日志行不带 `request_id` | `TestPromptErrorsAndLogs/ok` |

### 7.4 门禁与 CI 的余量

推送之前：`make lint gen-check knip build`、`make test`、`make test-go-short`、`make e2e`，重的运行经 `heavy.sh`。`make gen-check` 不用加检查：`tools.json`、`prompts.json` 都是手写的，由测试比较。

新测试的分份：`TestThePromptListsAreTheContract`、`TestTheWikiPromptsAreReached`、`TestThePromptReadsLLMWikiMatrix`、`TestTheWikiInstructionsFollowTheirSourceRoot` 落第三份，`TestTheDowngradeEndsTheContract` 在第四份，`TestNotebookInstructionsShareTheirBudget` 在 page，归 `test-go-race`。最近五次 main 第三份 265–405 秒，第四份 330–415 秒，都低于 450，新测试都是小的。两次 main 的均值严格低于 450 秒，分片要调整时按 M9/P2 §6.5。

审查发现第六份在 EPYC 7763 上 483–493 秒，别的 CPU 上 301–373 秒，两次 main 都落在 7763 时就过线。`TestIngestCandidates`、`TestIngestCoverage` 移到第五份（决定 166）。

P5 不碰成本的负载，时限照决定 150、153、159、160 的做法：同样的工作在 EPYC 7763 上慢约一倍，再碰到线先查 CPU。

## 8. 完成标准

`【…】` 是实施之后要填的实测值。

### 8.1 README

`### MCP` 第一段：核对内置说明的字节数（P5 不改内置的两份，应仍是 §1 第 5 条的数），删掉"P4 虚拟页与 P5 prompts 仍待"。"说明"那一条（README:224）写的是握手给 AGENTS 的正文，加上 LLM Wiki 的系统段在正文之前。`### 上限` 的表加 prompt 一行：参数至多 4,096 字节，请求至多 64 KiB，答复文字至多 48 KiB。

`### MCP` 之后、`## 接入 agent（MCP）` 之前新加一节 `### LLM Wiki 与 playbook`：

> 新建笔记本时可以选择 LLM Wiki。它有来源区和根下的 AGENTS 约定：来源添加后不修改，知识页由人或 agent 按约定维护；AGENTS 的正文可以更新，但在这种模式下不能删除、改名或移动。`index.md` 与 `log.md` 是系统生成的视图，不是要写回的页面。
>
> 在这本笔记本的 MCP 入口 `/mcp?notebook=<id>`，`prompts/list` 提供英文的 ingest、query、lint。ingest 可带 source 路径，省略时先列 `pending_sources`，读完来源之后用一个带说明的 `batch` 更新知识页，写明 `type`、`summary` 与带引号的 `sources`，再做 lint。query 带 question，查找并引用页和来源。lint 先做能机械修的，再看最近的变更集，报告其余。
>
> 普通笔记本也列出同样三个名字，取用答 JSON-RPC -32602，码是 `notebook.not_llm_wiki`。账户入口 `/mcp` 没有 prompt，`open_notebook` 的说明给出对应笔记本的入口。
>
> 握手和 `open_notebook` 的说明先是一段系统的文字，后面跟 AGENTS 去掉 frontmatter 的正文，空正文时用内置说明。系统的文字写当前的来源根和生成的视图，来源根改名或移动之后下一次读取就更新。AGENTS 太长时被截短并提示用 `read` 读全，系统的文字不截；来源根的路径太长时，这段文字改写它的标题。
>
> **降级**：管理员可以把 LLM Wiki 降级成普通笔记本，有写入任务排队或运行时拒绝。降级不动页面，`AGENTS` 与来源都留着，守卫和生成的视图不再生效，下一次握手不再带系统的文字。AGENTS 的正文仍是原来写的，里面可能还写着"这是 LLM Wiki"。
>
> **重新导入的限制**：把 LLM Wiki 的导出导进一本新的 LLM Wiki，根下的 `AGENTS.md` 因为已有 `AGENTS` 成为 `AGENTS 2`，档案的 `raw/` 因为已有 `raw` 成为 `raw 2`，它里面的来源落在来源区之外。这是 v0.1 写明的限制。

"降级"和"重新导入的限制"的字句在实施时对照 00 §4.5、§4.9 与 §7 核实。

### 8.2 M9 人工清单

`docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md` 加一节"prompts 与 LLM Wiki"，照 M9 的写法一步一步写成表格，每步写明看什么，末行的"prompts：M10 登记"换成指向它。没有实际执行的步骤保持未执行。

准备：一个隔离的客户端环境，记下 Claude Code 与 Codex 的版本和日期，不碰自己的配置；建一本 LLM Wiki，种两份来源（一页、一个附件），加一本普通笔记本和一个 PAT。

| 步 | 做什么 | 看什么 |
|---|---|---|
| J1 | 接笔记本入口 | 客户端怎样列出 ingest、query、lint 与它们的参数 |
| J2 | 取 `ingest`，不带 source，再带一个已有的来源路径 | 返回的说明和完整路径，不要求模型一定照做 |
| J3 | 取 `query`，question 用多个词 | 多个词怎样传过去 |
| J4 | 在普通笔记本取同一个 prompt | 对 -32602 与 `notebook.not_llm_wiki` 显示什么 |
| J5 | 接账户入口 | 没有 prompt，`open_notebook` 的说明指向笔记本的地址；不把客户端缓存的旧清单当成服务器的能力 |
| J6 | 看握手的说明 | agent 看得到系统的文字吗（决定 161）；AGENTS 很长时看得到吗 |
| J7 | 让 agent 照 ingest 做一份已批准的维护 | 先读完整来源，小改用 `edit`，`move` 一步到位，一个 `batch`，保留带引号的 `sources`，遵守锁和错误提示；`changeset` 上有 ingest 标记 |
| J8 | 用 query 答一个查得到的问题 | 引用；遇到 `partial_lines`、`cut`、`next_offset`、`next_column` 时用 `read` 补齐，不把片段当全文 |
| J9 | 用 lint 先修再看最近的 `log` | 来源没有变；空的父页没有被编造元数据 |
| J10 | 改来源根的位置、改 AGENTS，重连 | 说明更新了 |
| J11 | 降级之后重连 | 没有 "pages under" 这句系统的文字。AGENTS 自己可能还写着 LLM Wiki，不算失败 |

逐客户端记通过或失败和处置。发现 playbook 的措辞有偏差，记下原文，由负责人定怎样改，不放松服务端的守卫。M11 的 upgrade prompt 不在这一节。

### 8.3 移交与总体设计

- `handoffs/M9-P1-mcp-registry.md`："处理进展"加 P5/S1 一行，写明第 2 项的 `tools.json` 扩键、第 5 项的请求例外由 00 §4.15 取代；做完之后改 `status: done`。
- M11 的 `handoffs/M9-mcp-upgrade.md` 第 1 条：核对内置说明的字节与余量；32 KiB 的拼接与 512 字节的系统段是不同的界；upgrade 的工具走原来的 Tools、合同、权限、`Fit`，将来的 upgrade prompt 走已有的按入口注册。
- 00 §7 的 P5 一行、§12、§13。
- 总体设计 §5.9、§12.4 标实现，§13.1 的 prompt 条目，§13.4 第 1、3、5 条（`prompts.json` 的反例、MCP 客户端的 prompt 方法、prompt 列表的整个程序的测试），§15 一行。
- `reviews/P5-S1-playbooks-review.md`，本设计 §10。
- `docs: M10/P5 done (M10/P5)`。

## 9. 交给后面

P5 是 M10 最后的实现 Phase，之后是 M10 收尾（02 §4），不自动开 M11。M11 继承按入口的 Prompt 注册表和实测的余量。真实客户端的清单在 v0.1 的最后由负责人执行，改措辞要跑相应的回归。

## 10. 结果

### 10.1 S1

S1 建成了设计第 0 节列的全部内容：按入口的 Prompt 注册表和它的界，三份英文 playbook，两个入口共用的说明。门禁、CI 与分份的数都在 [S1 审查记录](reviews/P5-S1-playbooks-review.md)。

与设计不同的地方：

- `TestThePromptReadsLLMWikiMatrix` 从 13 列扩到 78 格，读者能取用改由它判，见 §7.1。
- 三个 `Get` 由 `playbook` 这一个闭包给出，`PlaybookText` 在 domain 里拼参数，设计附录 B 原写"由 adapter 追加"，已改。
- 过界的 error 行多带 `request_id`。
- playbook 比设计初稿多两句：ingest 分清没记标记和失败，lint 写明 `log` 只给名字。附录 B 是最终的文字。
- 不在设计里的两处：第六份的两个测试移到第五份（决定 166），e2e 的服务等 `jobs started`（决定 167）。
- 设计 §6 第 2 步要先看到测试因行为失败再实现。实际先写了实现，靠变异补上：P01–P05、P13 证明这些测试会因行为失败。

## 附录 A：核对过的代码

- 平台：`prompt.go`、`server.go`、`call.go`、`log.go`、`errors.go`、`answer.go`、`edge.go`；SDK v1.8.0 的空列表、未知 prompt 与排序。
- 组合根：`bootstrap/mcp.go`、`wire.go`、`batch_cost_test.go`；wiki `lint.go`、`app/lint.go`、`app/lint_ports.go`。
- 说明：page `mcp_entry.go`、`app/open_notebook.go`、`app/instructions.go`、`domain/instructions.go`、`adapter/mcp/open_notebook.go`；`Nodes` 的点读与祖先端口。
- 消费者：`e2e/fixtures/mcp.ts`、LW1。

## 附录 B：三份英文正文

每份的第一行是最后一跳的字面判据。参数追加在正文之后，不作为模板执行。三份里的工具名、字段名都对着 `tools.json` 和实现核过，写法经真实 MCP 实测（[设计审查](reviews/P5-design-review.md)）。

### playbook-ingest.md

```text
Ingest a source into this LLM Wiki.

Read and follow AGENTS.md and this notebook's instructions. If no source path was supplied, call pending_sources and choose from its first page; continue its next_cursor only when you need more. Read the chosen source whole: follow read's next_offset, with next_column when it is set, until next_offset is null. A source is evidence you add once and never change. Do not edit, move, rename or delete it as part of ingestion. If read answers an attachment with metadata only, there is no text to use: report it, and do not cite what you could not read.

Use index.md or index, search, grep, backlinks and read to find related knowledge before deciding what to add or amend. Do not write to generated index.md or log.md. Keep claims grounded in the source and distinguish evidence from your interpretation.

Plan one batch for the knowledge pages you can justify from this source. Give the batch a message in the form "ingest: <source path>". Follow AGENTS for page organization. Each page you create or maintain must have type and summary frontmatter and quoted sources links that repeat the source's path, for example sources: ["[[raw/Note.md]]"] or sources: ["[[raw/paper.txt]]"]. Quote a summary that holds a colon. Link new pages from the pages they belong with. The message describes the work; it does not itself mark a source as ingested.

A write under a folder that does not exist creates its empty parent pages. They are folders: lint lists them under missing_type and missing_summary. Leave them empty and say so in your report, unless one should become a real page with content and links.

For a small existing change, use edit with exact text rather than replacing the whole page. Read the current version before a replacement write. A move takes the full new path in one operation, not a destination folder or a sequence of guessed intermediate moves. Respect the batch limits and permissions. If a coherent change cannot fit, narrow the work and explain what remains; do not silently split a promised atomic batch or bypass a guard.

Treat grep's partial_lines and cut hits, and an incomplete page read, as partial evidence. grep gives a line, not an offset: call read with offset set to that line and limit 1, with column when the hit has one, and repeat from next_offset and next_column until next_column is null. partial_lines have no column: read the line from its start. Continue next_cursor where needed; an empty page, a capped count or grep's incomplete does not prove there is no more. On stale versions or locks, re-read or wait as the tool says; do not overwrite a source or take over an edit lock.

After the successful batch, call lint and check the relevant results. Confirm the ingestion: read the batch's changeset_id with changeset and check its ingest_count and ingest_sources. A batch can succeed and record nothing: lint's invalid_frontmatter or invalid_sources may say why, and a stale_pages above zero means the links were still being indexed. A source already digested adds no mark, and a source gone from pending_sources is digested even without one: report the missing mark, and do not repeat the ingestion. Report the pages changed, the source references and any remaining work. Do not claim ingestion succeeded just because a prompt or a batch message was produced.
```

没有提供 source 时不追加用户段。提供了（包括空串）就追加 `Source path supplied by the caller:\n` 和原值一次，路径是否有效由之后的 `read` 判断。

### playbook-query.md

```text
Answer a question from this LLM Wiki.

Read and follow AGENTS.md and this notebook's instructions. Use index.md or index to orient yourself, then search, grep, backlinks and read to inspect the pages and their cited sources. Prefer the underlying evidence to a title or excerpt. Generated index.md and log.md are read-only views.

Continue next_cursor when the answer needs more results. A capped count, an empty page with next_cursor, grep's incomplete or building does not prove absence. grep gives a line, not an offset: for partial_lines or a cut hit, call read with offset set to that line and limit 1, with column when the hit has one, and repeat from next_offset and next_column until next_column is null, before relying on the text.

Answer the supplied question with references to the relevant pages and sources. Separate what the evidence says from inference, disagreement and missing information. Do not invent a source or present an unverified excerpt as a complete document.

If the answer is worth keeping, propose a separate knowledge page, and create one only when the requested work asks for it, following AGENTS and the existing write permissions. Include type, summary and quoted sources. For small existing changes use edit with exact text; for a move use its full destination path in one operation. Use current versions and respect locks and all batch limits. Never change a source or write a generated view to make the answer appear supported.
```

追加 `Question supplied by the caller:\n` 和 question 原值一次。缺键由平台在处理函数之前拒绝，空串不另当缺失。

### playbook-lint.md

```text
Check and maintain this LLM Wiki.

Read and follow AGENTS.md and this notebook's instructions. Start with lint without a rule to read its summary. If stale_pages is not zero, a clean result is not yet clean: say so. Inspect the relevant rule items and continue next_cursor. count_capped marks a lower bound, not the end of the list; target_cut marks a shortened display target: use item_key and the actual page or source paths when checking a finding.

Read the affected knowledge pages and their evidence. Make only justified mechanical repairs for missing_type, missing_summary and invalid_sources, in a batch. Do not invent metadata or replace a broken source reference with a guessed one. If the evidence is insufficient, report the finding instead of deleting or fabricating anything. An empty parent page made by a write is a folder: leave it empty and report it, unless it should become a real page with content and links. Sources must never be changed as part of these repairs; index.md and log.md are generated views, not editable pages.

Use edit for small changes with exact old text and current versions for replacements. Move to the full new path in one operation. Respect permissions, edit locks and all batch limits; a failed batch is not a successful partial repair. grep gives a line, not an offset: for partial_lines or a cut hit, call read with offset set to that line and limit 1, with column when the hit has one, and repeat from next_offset and next_column until next_column is null.

After mechanical repairs, read recent changesets with log and inspect the pages they touched for semantic problems: unsupported claims, contradictions, duplicated concepts and stale references. The newest entries are your own repairs: look past them. log shows each changeset's first three changes by name; read one with changeset for all its changes and their paths, and read the pages and their cited sources as needed. Choose the relevant recent changes; do not assume a fixed number is always enough. Do not infer semantic correctness solely from a clean mechanical lint result.

Run lint again after successful repairs. Report what changed, which pages and sources justify it, what remains uncertain and what needs human judgment. Never claim a fix that the tools did not commit.
```
