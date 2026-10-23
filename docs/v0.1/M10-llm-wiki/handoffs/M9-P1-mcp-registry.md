```yaml
status: open
from: M9/P1
to: M10
created: 2026-10-18
```

# MCP 的 prompt、虚拟的 `index.md` 与 `log.md`、`AGENTS` 的保护

M9/P1 建了 MCP 的平台与读的工具（[M9/P1 文档](../../M9-mcp/01-P1-platform-reads.md)，[M9 总设计](../../M9-mcp/00-M9-design.md) 4.1–4.9）：`platform/httpserver/mcpserver` 的工具与 prompt 的注册表，笔记本入口（`/mcp?notebook=<id>`）与账户入口（`/mcp`），`list` 与 `read`，握手时按根下的 `AGENTS` 页给的说明。M10 的 LLM Wiki 在这上面登记 prompt、加两个虚拟的文件、保护 `AGENTS` 页（[总体设计](../../v0.1-design.md) 5.5、5.7）。这里是 M9/P1 第 8 节给 M10 的几条；M9 总设计"写给后面的 M"给 M10 的别的（`lint`、`pending_sources` 两个工具，来源区的守卫要认得的几种写）由之后的 Phase 写进 M10 的移交，M9 收尾时核对。留给 M10 的：

1. **prompt 的注册表是最小的**（[M9/P1](../../M9-mcp/01-P1-platform-reads.md) 3.4.7，P1 设计审查 A-M7 与 A 的过度设计第 1 条）：
   - `mcpserver.Prompt`（`platform/httpserver/mcpserver/prompt.go`）只有名称、标题、说明、字符串的参数（`PromptArgument`：名称、说明、是否必填，没有 JSON schema）与处理函数 `Get(ctx, Call, args) (description, messages, err)`。两个入口都登记；名称同工具的规则（1–32 个 `a-z` 与 `_`），重名或没有 `Get` 时 `New` panic；缺必填的参数答 -32602，每条消息是一个 `role: user` 的 `TextContent`。有 prompt 时能力声明 `prompts`、`listChanged` 为假，没有时能力里没有 `prompts`。
   - M9 的生产接线没有 prompt（`Options.Prompts` 为空，组合根不传）：mcpserver 的 `TestThePromptDoubleIsListedAndReached` 以一个替身证明登记、能力、`prompts/list` 与 `prompts/get` 到达处理函数。参数的检查、别的消息的类型与结果的形状，M10 需要时在这个注册表上加；模块根以后给 `Prompts()`（同 `Tools()`，M9 总设计第 8 节）。
2. **最后一跳与契约**：M10 登记 ingest、query、lint 三个 prompt 时写整个程序的最后一跳：经组合根拼起的 prompt 到达它的处理函数，组合根交空时失败。并把它们加进 `api/mcp/tools.json` 的检查：文件今天恰好两个键 `notebook`、`account`（各是那个入口的 `tools/list` 的每个工具对象），别的键 `toolListDiff` 报错（`bootstrap/mcp_tools_test.go`）；清单的形状加 `prompts`（两个入口同一组），契约的测试比较 `prompts/list` 的每个成员。
3. **虚拟的 `index.md`、`log.md` 进 `list` 与 `read`**（总体设计 5.5）：今天找路径的 `NodePaths.Find`（`page/app/node_paths.go`，`read` 经它，`read_path.go`）与 `ListByPattern`（`page/app/list_by_pattern.go`）只认树里的节点，要在树之外认得根下的这两个路径；glob 的 `**/*.md` 是否列出它们由 M10 定。写它们由模式的守卫拒绝，答"系统生成"的文字（总体设计 5.5），不答 `page.not_found`。`index.md` 复用 `ListIndex`、`log.md` 复用 `ListChangesets`，承袭它们的界线；一个文件的虚拟页要读全部，要有它自己的界线与截断的说法（M9 总设计"写给后面的 M"）。
4. **`AGENTS` 页在 `llm_wiki` 下的保护**（不能删、改名、移动，总体设计 5.7）由模式的守卫做。说明的找法与缓存照 M9/P1 3.15（M9 总设计 4.6）：根下标题为 `AGENTS` 的页（不分大小写）去掉 frontmatter 的正文，至多 32 KiB；缓存的键是笔记本、这个节点与正文的版本号，替换、改名都换键，下一次连接就是新的。
5. **`prompts/get` 的上限与错误**：
   - 请求体同 `tools/call` 之外的每个方法至多 64 KiB（`mcpserver/edge.go` 的 `maxOtherBytes`，M9/P1 3.2.2）：prompt 的参数要放得下，或者另设例外（今天超过 64 KiB 的只能是这个入口登记的工具的 `tools/call`）。
   - `Get` 的错误还没有分类与上限：任何错误都记 error "MCP prompt failed"、答 JSON-RPC 的 -32603（`promptHandler` 的注释写明是 M10 的）；工具的错误经 `httpserver.ProblemOf` 分类、在 48 KiB 之内（M9/P1 3.3.6），prompt 的要照做。`prompts/get` 的答复也不经 `Fit`、没有 48 KiB 的上限（M9/P1 S1 审查 B 的细节 7，修复核对第一轮 A）。
6. **之后的 Phase 留下的三条**（[M9/P3](../../M9-mcp/03-P3-batch-revert.md)、[M9/P4](../../M9-mcp/04-P4-grep-search.md) 第 8 节，[M9/P5](../../M9-mcp/05-P5-stories-docs.md) 第 11 节）：
   - **内置的说明的余量**：账户入口 4,027 字节、笔记本入口 3,957 字节，离 4 KiB 只剩 69 字节。M10 往说明里加字（prompt、虚拟的两页、`llm_wiki` 的规则）的先缩短已有的行，`TestTheBuiltInInstructionsAreShort`（`page/adapter/mcp/instructions_test.go:15-27`）照旧守 4 KiB。
   - **故事的 MCP 版本的约定**（[M9 总设计](../../M9-mcp/00-M9-design.md)第 9 节，M9/P5 3.1）：在故事自己的文件里、名为"`<故事> (MCP)`"，种下的数据照 API 版本经 REST，每个 agent 一个 PAT，值来自答复（`e2e/fixtures/mcp.ts` 的 `writtenOf`、`readPage`），收编辑会话的断言以 `expectVersions` 代之，不开浏览器。
   - **prompt 的说法**：人工清单（`M9-mcp/manual/P5-mcp-clients-checklist.md`）记下 agent 不照说明做的地方（C3–C6 的 `edit` 与 `move`，C12、C15 的 `partial_lines`、`cut` 与 `read(offset, column)`）；M10 写 ingest、query、lint 的 prompt 时照它们改说法。清单在 v0.1 的最后才跑，M10 在那之前写的照工具的说明。
