# M10/P5 设计审查

设计：[05-P5-playbooks.md](../05-P5-playbooks.md)。基线 `c5fff600`，受审的版本是 Codex 在 P4 收尾时写的候选 `c1443732`。

三位审查者相互独立，各自写了原型或探针：A 管平台（注册、参数、答复的界、错误、日志），B 管说明（资料、拼法、预算、并发），C 管 playbook 的正文、e2e、测试与完成度。

结论：结构可用。`prompts.json` 的全文与真实列表一致，预算的算术正确，照 playbook 写的页确实得到 ingest 标记。候选的问题在三处：漏了 `open_notebook` 的描述；几条测试抓不到它声称要抓的错；playbook 里有几句和工具的实际行为不符。全文也写得太省，C 说读不懂，所以设计是重写的，不是打补丁。

## 三位共同的发现

**Important：漏了 `open_notebook` 的描述。** 00 §5、§7 都写了要改，候选的文件表里 `tools.json` 不动。设计 §3.9 补上，`tools.json` 与 `openNotebookDescription` 同一个提交改，`TestTheToolListsAreTheContract` 比较整个对象。

## A：平台

| 发现 | 严重度 | 处置 |
|---|---|---|
| P08 没有抓住 raw 与编码字节的差别。要 `description` 4,096 个 `<` 加消息 4,097 个才过界，恰好 49,152 是成功的例子 | Important | §7.1 写出这组数 |
| 请求体的测试把填充放在已声明的参数里，会被 4,096 的界先挡住，抓不到 P09 | Important | §7.1 改放未声明的参数：65,536 答 200、`Get` 1 次，65,537 答 413、`Get` 0 次 |
| 漏了 `call_test.go:431`、`tools_test.go:386`、`batch_cost_test.go:63` | Minor | §3.1 补上。前两处不加 `Entry` 时 `New` panic |
| 没有写 SDK 解码这一层 | Minor | §3.4 第 2 行。这里的 `params` 回显是 M9 已接受的行为，"不回显"只约束第 4 步之后 |
| 缺键用什么码没写 | Minor | §3.4：`validation_failed`。`mcpserver` 不导入 shared，用私有常量 |
| P05 的抓手写多了 | Minor | §7.3：只有 `TestThePromptListsAreTheContract` |
| 看不到的笔记本写成 JSON-RPC | Minor | §3.4 第 1 行：HTTP 404 的 problem，映射由 wiki 的单元测试覆盖 |
| fault 的形状没写 | Minor | §3.4：fault 不带 `data`，panic 是 `{"code":-32603,"message":"internal error"}` |
| null 参数、过界日志、`client_cancel` 要轮询、普通笔记本加 4,097 字节的次序 | Nit | §3.3、§3.4、§7.1 都写了 |
| 过度设计：外面包一层日志、`invalid_name`、零 ID 检查、第二次 ctx 检查、"种类不存在"的分支 | — | 全部删掉，日志进 `promptHandler` 的 defer（决定 162） |

## B：说明

| 发现 | 严重度 | 处置 |
|---|---|---|
| "只机械适配"不对：`mcp_account_test.go:90` 钉住说明等于 AGENTS，`open_notebook_test.go:53` 与 `virtual_list_test.go` 的夹具要真的根节点 | Minor | §3.1 列出 |
| `MCPEntry` 的 `StateOf` 没有时没写 | Minor | §3.7：`ErrNotebookNotFound`，改 `errors.go` 的注释，加测试 |
| 路径到界的测试缺具体的点 | Minor | §3.8、§7.1：349、350 与 286、287 字节，P22 |
| 判断系统段有无的办法不成立：模板第一句也是 "This notebook is an LLM Wiki" | Minor | §1 第 6 条、§7.1：`HasPrefix(整句 + "\n\n")` 与 `== AGENTS 的正文` |
| P14 直接测 `NotebookText` 抓不到调用点的错；缺"丢祖先""按笔记本记住根""open 只量正文"三个单点 | Minor | §7.3 的 P14、P19、P20、P21 |
| 并发的结果没写 | Minor | §3.7 写出三种，§7.2 用假端口钉住 |
| 空系统段不加换行、`Opened.Mode` 重复、握手测试写 `≤`、512 是 raw、降级之后也看 open、不改内置两份 | Nit | §3.7、§3.8、§5、§7.1、§1 第 5 条 |
| 过度设计：握手的 `fits` 重复、组合的追踪、透明的包装 | — | 删掉 |

B 问了两件事。系统段放在哪里，见决定 161；降级之后 AGENTS 还写着 LLM Wiki，这是 00 §4.5 的结果，接受，人工清单第 J11 步写明要看的是 "pages under" 这一句。

## C：playbook、e2e 与完成度

| 发现 | 严重度 | 处置 |
|---|---|---|
| playbook 把 `partial_lines` 当成有 offset 与 column | Important | 附录 B：grep 给的是行，不是 offset；`partial_lines` 没有 column，从行首读 |
| 空的父页：`missing_type`、`missing_summary` 会列出它，填上又进 `orphans` | Important | 决定 163，ingest、lint 各加一句 |
| README 缺降级与重新导入的限制 | Important | §8.1 |
| `prompts.json` 要过 oxfmt；第 2、3 个提交要跑 TS 的检查 | Minor | §3.3、§6 |
| 漏 `mcp_account_test.go`、`mcp_entry_test.go:53` | Minor | §3.1 |
| §7.4 写了 `test-web` | Minor | 改成 `make test` |
| 缺"账户入口没有能力"、`Playbooks` 没有 `Authorize`、根路径加 `.md`、空 AGENTS 丢系统段四个变异 | Minor | §7.3 的 P23–P26 |
| 用 `TextBytes` 判 `TextBytes` | Minor | §7.1：用 SDK 真实的文字量 |
| `prompts/get` 不在权限矩阵里 | Minor | 决定 165 |
| §8.3 缺总体设计 §15、00 §12/§13、13.4 第 1/3/5 条、13.1 的 prompt 条目、M9-P1 移交转 `done` | Minor | §8.3 |
| README 的位置没写 | Minor | §8.1 |
| §8.2 没有照 00 §4.19 写成一步一步的表 | Minor | §8.2 |
| playbook 的十几处措辞：读完全文的办法、附件只有元数据、`sources` 的写法、带冒号的 summary、`incomplete`、确认 ingest 的办法、`count_capped` 与 `target_cut`、`stale_pages`、`log` 只给前三项、最新的 log 是自己写的 | Minor | 附录 B 逐条改，工具的字段名对着 `tools.json` 与实现核过 |
| 新测试的分份 | Minor | §7.4 |
| 全文电报体，"minimum、身份、selector、唯一锚、preimage"这类词读不懂 | Minor | 重写 |

C 另列了 M10 收尾要做的事，收尾时逐条处理：

- P5 完成之后三席的收尾审查；
- `handoffs/M9-P1-mcp-registry.md`、`handoffs/M7-transfer.md` 转 `done`；
- 00 §7"写给后面的 M"里还没进 M11 移交的几条（`raw 2`、upgrade prompt、保留的标题）；
- 00 §7 两个成本守卫的 CI 稳定性说明；
- 总体设计 12.6 写"实现完成，待人工验收"；
- 普通克隆里跑 `make image-smoke`；
- 最后两次 main 的均值。

## 决定

决定 161–165 记在设计 §2.3 与 00 第 11 节，都是主持者的决定，负责人可以改判。

## 原席核对

三位各自读了重写之后的设计，对着代码核，全部初审发现都已关闭，没有 Important。核对又提了几条小的，都改了：

- **A**：`WorkspaceOf` 没有、`Authorize` 答 `ErrNotVisible` 都要写成 `notebook.not_found`，因为 `ErrNotVisible` 自己的码是空的；P23 靠组合根的测试核能力，要写明确切的能力对象（A 用 overlay 跑了这个变异：账户入口多了 `prompts`，列表仍是空的）；真实上限改成约 27.9 KB；panic 那一行的 `outcome` 写明。
- **B**：系统段在前时 `fits` 仍然量的是实际返回的字符串，6,000 组随机输入没有超界。`AgentsText` 放不下时不给信号，所以 `NotebookText` 要再量一次，内置说明一支也量；这个护栏没有测试，补 `no_room` 与变异 P27。两条说"删掉了"的过度设计其实还留着一半（握手的 `fits`、组合根那一行的五项），改掉，五项挪到假端口的测试。00 的 LW1 一行照新的次序写。
- **C**：README 的"说明"那一条重写时漏了，补上，并写明新的一节放在哪里；`limit 1` 时 `next_offset` 会走出这一行，playbook 改成"until next_column is null"；确认 ingest 改用 batch 答复的 `changeset_id`，比翻 `pending_sources` 与 `log` 准；`sources` 的说法不再假定 `pending_sources` 被调过；分份的秒数改成最近五次 main 的实际范围；人工清单的步号改成 J1–J11，免得和 Phase 撞名。

C 实测了 playbook 现在教的写法都记下 ingest 标记，`invalid_sources`、`invalid_frontmatter` 都为空；`prompts.json` 原样过 oxfmt。B 的原型上，七个说明、降级与权限的测试除了 §3.1 列出要改的 `TestNotebooksAnswerTheirModeOverMCP` 都通过，P14–P22、P25、P26 都杀得掉。

这些都是一句话的改动，不再开一轮。真实的杀死与 LW1、LW11 要到实施之后才有。

设计提交 `3b0d55f2` 的时候，我的脚本在改 00 时中断，后面的命令没有看返回值，把这一节和 00 的 LW1 一行漏在外面就推了。下一个提交补上，历史不改。
