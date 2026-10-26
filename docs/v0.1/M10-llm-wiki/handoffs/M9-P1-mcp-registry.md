```yaml
status: open
from: M9/P1
to: M10
created: 2026-10-18
```

# MCP 的 prompt、虚拟的 `index.md` 与 `log.md`、`AGENTS` 的保护

M9/P1 建了 MCP 的平台与读的工具（[M9/P1 文档](../../M9-mcp/01-P1-platform-reads.md)，[M9 总设计](../../M9-mcp/00-M9-design.md) 4.1–4.9）：`platform/httpserver/mcpserver` 的工具与 prompt 的注册表，笔记本入口（`/mcp?notebook=<id>`）与账户入口（`/mcp`），`list` 与 `read`，握手时按根下的 `AGENTS` 页给的说明。M10 的 LLM Wiki 在这上面登记 prompt、加两个虚拟的文件、保护 `AGENTS` 页（[总体设计](../../v0.1-design.md) 5.5、5.7）。这里是 M9/P1 第 8 节给 M10 的几条与之后的 Phase 加的第 6 条；M9 总设计"写给后面的 M"给 M10 的别的（`lint`、`pending_sources` 两个工具，来源区的守卫要认得的几种写）原该由之后的 Phase 写进来，M9 收尾时才补上，是第 7、8 条，第 3 条另补 `index.md` 的界线与代价、`grep` 与虚拟的两页（[M9 收尾审查](../../M9-mcp/reviews/M9-closeout-review.md) C-I1 = A-F4、C-M1）。留给 M10 的：

1. **prompt 的注册表是最小的**（[M9/P1](../../M9-mcp/01-P1-platform-reads.md) 3.4.7，P1 设计审查 A-M7 与 A 的过度设计第 1 条）：
   - `mcpserver.Prompt`（`platform/httpserver/mcpserver/prompt.go`）只有名称、标题、说明、字符串的参数（`PromptArgument`：名称、说明、是否必填，没有 JSON schema）与处理函数 `Get(ctx, Call, args) (description, messages, err)`。两个入口都登记；名称同工具的规则（1–32 个 `a-z` 与 `_`），重名或没有 `Get` 时 `New` panic；缺必填的参数答 -32602，每条消息是一个 `role: user` 的 `TextContent`。有 prompt 时能力声明 `prompts`、`listChanged` 为假，没有时能力里没有 `prompts`。
   - M9 的生产接线没有 prompt（`Options.Prompts` 为空，组合根不传）：mcpserver 的 `TestThePromptDoubleIsListedAndReached` 以一个替身证明登记、能力、`prompts/list` 与 `prompts/get` 到达处理函数。参数的检查、别的消息的类型与结果的形状，M10 需要时在这个注册表上加；模块根以后给 `Prompts()`（同 `Tools()`，M9 总设计第 8 节）。
2. **最后一跳与契约**：M10 登记 ingest、query、lint 三个 prompt 时写整个程序的最后一跳：经组合根拼起的 prompt 到达它的处理函数，组合根交空时失败。并把它们加进 `api/mcp/tools.json` 的检查：文件今天恰好两个键 `notebook`、`account`（各是那个入口的 `tools/list` 的每个工具对象），别的键 `toolListDiff` 报错（`bootstrap/mcp_tools_test.go`）；清单的形状加 `prompts`（两个入口同一组），契约的测试比较 `prompts/list` 的每个成员。
3. **虚拟的 `index.md`、`log.md` 进 `list` 与 `read`**（总体设计 5.5）：今天找路径的 `NodePaths.Find`（`page/app/node_paths.go`，`read` 经它，`read_path.go`）与 `ListByPattern`（`page/app/list_by_pattern.go`）只认树里的节点，要在树之外认得根下的这两个路径；glob 的 `**/*.md` 是否列出它们由 M10 定。写它们由模式的守卫拒绝，答"系统生成"的文字（总体设计 5.5），不答 `page.not_found`。`index.md` 复用 `ListIndex`、`log.md` 复用 `ListChangesets`，承袭它们的界线；一个文件的虚拟页要读全部，要有它自己的界线与截断的说法（M9 总设计"写给后面的 M"）。`ListIndex` 的界线（[M9/P4](../../M9-mcp/04-P4-grep-search.md) 3.9.3 与第 8 节）：索引里没有行的页不在其中，每页至多 20 个标签，入链数数到 1,000，一页至多 100 项，`type`、`summary` 与每个标签截到 256 字节、以 `cut` 标出；它的代价：每次调用读这本笔记本的全部索引的页与每页的类型的 `value_start`，10 万页的一次约 1 秒、分配约 158 MB（P4 的 S2 的守卫以 250 MB 界住一次调用），虚拟的文件按页读时同样。`grep` 只搜树里的页的正文：虚拟的两页不在树里，`**/*.md` 的 glob 也不含它们；M10 定 `grep` 要不要找到它们（`NodePaths.Find` 与 `ListByPattern` 在树之外认得它们时，`grep` 的走查同样要加，M9/P4 第 8 节）。
4. **`AGENTS` 页在 `llm_wiki` 下的保护**（不能删、改名、移动，总体设计 5.7）由模式的守卫做。说明的找法与缓存照 M9/P1 3.15（M9 总设计 4.6）：根下标题为 `AGENTS` 的页（不分大小写）去掉 frontmatter 的正文，至多 32 KiB；缓存的键是笔记本、这个节点与正文的版本号，替换、改名都换键，下一次连接就是新的。
5. **`prompts/get` 的上限与错误**：
   - 请求体同 `tools/call` 之外的每个方法至多 64 KiB（`mcpserver/edge.go` 的 `MaxOtherBytes`，M9/P1 3.2.2）：prompt 的参数要放得下，或者另设例外（今天超过 64 KiB 的只能是这个入口登记的工具的 `tools/call`）。
   - `Get` 的错误还没有分类与上限：任何错误都记 error "MCP prompt failed"、答 JSON-RPC 的 -32603（`promptHandler` 的注释写明是 M10 的）；工具的错误经 `httpserver.ProblemOf` 分类、在 48 KiB 之内（M9/P1 3.3.6），prompt 的要照做。`prompts/get` 的答复也不经 `Fit`、没有 48 KiB 的上限（M9/P1 S1 审查 B 的细节 7，修复核对第一轮 A）。
6. **之后的 Phase 留下的三条**（[M9/P3](../../M9-mcp/03-P3-batch-revert.md)、[M9/P4](../../M9-mcp/04-P4-grep-search.md) 第 8 节，[M9/P5](../../M9-mcp/05-P5-stories-docs.md) 第 11 节）：
   - **内置的说明的余量**：账户入口 4,027 字节、笔记本入口 3,957 字节，离 4 KiB 只剩 69 字节。M10 往说明里加字（prompt、虚拟的两页、`llm_wiki` 的规则）的先缩短已有的行，`TestTheBuiltInInstructionsAreShort`（`page/adapter/mcp/instructions_test.go:15-27`）照旧守 4 KiB。
   - **故事的 MCP 版本的约定**（[M9 总设计](../../M9-mcp/00-M9-design.md)第 9 节，M9/P5 3.1）：在故事自己的文件里、名为"`<故事> (MCP)`"，种下的数据照 API 版本经 REST，每个 agent 一个 PAT，值来自答复（`e2e/fixtures/mcp.ts` 的 `writtenOf`、`readPage`），收编辑会话的断言以 `expectVersions` 代之，不开浏览器。
   - **prompt 的说法**：人工清单（`M9-mcp/manual/P5-mcp-clients-checklist.md`）记下 agent 不照说明做的地方（C3–C6 的 `edit` 与 `move`，C12、C15 的 `partial_lines`、`cut` 与 `read(offset, column)`）；M10 写 ingest、query、lint 的 prompt 时照它们改说法。清单在 v0.1 的最后才跑，M10 在那之前写的照工具的说明。
7. **来源区的守卫要认得的几种写**（M9 收尾补上，[M9 收尾审查](../../M9-mcp/reviews/M9-closeout-review.md) C-I1；[M9 总设计](../../M9-mcp/00-M9-design.md)"写给后面的 M"、4.10–4.12，[M9/P3](../../M9-mcp/03-P3-batch-revert.md) 第 8 节）：守卫看到单元的每一步，M9 加了四种它以前没见过的：
   - **一步里同时的移动与改名**：单个 `move` 的 `Unit.Relocate`（`page/app/unit_relocate.go`）与撤销的放回（`Unit.place`，`page/app/unit_revert.go`）以一条语句同时改父节点与名称，作为一步到达守卫：父节点变了是 `OpMove`，否则 `OpRename`，这一步的变更同时带新的父节点与名称。守卫要两样都看，不能从步的种类推出只变了一样。
   - **batch 的合成的正文**：一个 batch 里同一页的几个正文操作合成一步，在这一页第一个正文操作的位置以它最后的正文写一次（一个版本行、一次守卫），守卫看到的是 agent 要的最后的正文，不是中间态；推后的链接改写的追加写在全部操作之后另经守卫（M9 总设计 4.11）。
   - **撤销里的临时名称**：撤销的步带 `Write.Kind = revert`（[来源区的守卫与撤销、还原、恢复](M8-history.md)）。其中临时的改名与移到根下的步（`<名称> ~<节点 id 的后 8 个十六进制字符>`，冲突时取全部 32 个，再冲突接 `-2`、`-3`……，`domain.AsideName`）以普通的 `OpRename` 或 `OpMove` 的一步到达，临时名称在 `Changes[0].After`，步上没有它是临时的步的标记；模式 `" ~<hex>"` 也是合法的用户标题，放回一个真叫 `x ~1a2b3c4d` 的节点的撤销在守卫看来相同，所以只凭名称认不准（M9 总设计第 11 节主持者的决定第 65 条，M9/P3 S2 审查 B-N4）。M10 可以在 `Step` 上带步的种类（`domain.RevertOpKind`）或一个 `Aside` 标记。同一节点之后的放回改回最后的名称，条目、索引、搜索与事件里没有临时名称。
   - **先恢复、再写回**：同一单元里恢复又写回的页，守卫先看到恢复的一步与它的事实（回收站里的正文的），再看到写回（`Base` 的正文）。

   后台撤销的每一批是普通的单元，各步照上面到达守卫。M10 定撤销里的这几种步写到来源页时算不算"修改来源"（同 M8 的那一份）。
8. **`lint` 与 `pending_sources` 两个工具**（M9 收尾补上，[M9 收尾审查](../../M9-mcp/reviews/M9-closeout-review.md) C-I1；M9 总设计第 1 节第 8 条与"写给后面的 M"）：登记在 MCP 的注册表上（模块根的 `Tools()`、`mcpserver.Tool`，由组合根的 `mcpTools` 拼接），照 M9 的每个工具：先写进 `api/mcp/tools.json`（两个入口的 `tools/list` 各是一个数组，`TestTheToolListsAreTheContract` 比较整个工具对象，没有 `-update`），在权限矩阵里有行（`TestThePermissionMatrixCoversEveryTool`），声明的码进 `mcpToolCodes()` 的表、每个都要答出过，答复经 `Fit` 至多 48 KiB，内置的说明有它的一行（`TestTheBuiltInInstructionsNameEveryTool`；说明的余量见第 6 条）。

## 处理进展

- M10 开工（2026-10-24）：照[M10 总设计](../00-M10-design.md)第 7 节"移交的落实"的"M9/P1 MCP 的 prompt、虚拟文件与 `AGENTS`"各行做：第 4 项（`AGENTS` 的保护）在 P1 的 S2；第 7 项（守卫要认得的四种步）在 P1 的 S2 与 P2 的 S1；第 8 项（`lint` 与 `pending_sources`）与第 6 项的余量在 P3 的 S1；第 3 项（虚拟文件）在 P4 的 S1；第 1、2、5 项（注册表、最后一跳、`prompts/get`）与第 6 项的其余在 P5（4.15）。仍是 `open`，做完的 Phase 改为 `done`，收尾核对。
- P1/S2（2026-10-25，合并见 [00 号文档](../00-M10-design.md)第 12 节）：第 4 项（`AGENTS` 不能删除、改名、移动，正文照常）做完：模式的守卫的 R1 以"根下、标题键是 `agents`"认出它，经 REST 与 MCP 的整个程序的测试（`TestTheModeGuardReachesEveryPath`、`TestTheModeGuardReachesEveryMCPWrite`：MCP 的 `move`、`delete` 与 `batch` 答 `page.protected`，`edit` 与 `write` 覆盖 `AGENTS.md` 照常写下）与 e2e 的 LW2 (API)、LW2 (MCP)；第 7 项的移动加改名（R1、R3、R4 同时看父节点与名称）与撤销的临时名称照普通的改名、移动判，合成的正文与先恢复再写回随 P2/S1；仍是 `open`。
- P1/S3（2026-10-26，合并见 [00 号文档](../00-M10-design.md)第 12 节）：第 7 项涉及的撤销步骤在不加锁与加锁的预检里照它们实际的操作与先后状态判同一份契约；后台撤销每批仍经单元的守卫。`RevertFacts` 复用调度器的模拟并由 `TestARevertsFactsFollowItsSteps` 核对逐步事实；预检的码、后台的最后一跳与反向对照见 [01-P1](../01-P1-contract-frame.md) 10.3。第 1、2、3、5、6、8 项与第 7 项的来源正文规则仍按开工时的 Phase 分工推进；仍是 `open`。
