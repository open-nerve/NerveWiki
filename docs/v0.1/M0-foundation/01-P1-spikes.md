# M0/P1 技术验证：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M0/P1 技术验证 |
| 状态 | 进行中 |
| 基线 | `2ed0380`（只有文档，没有代码） |
| 上级文档 | [M0 总设计](00-M0-design.md) 第 7 节 |

---

## 1. 基线

仓库里只有设计文档。本 Phase 的实验代码写在仓库之外的临时目录中，用完即弃，不进入产品代码；进入仓库的只有结论（本文第 7 节）和 Markdown 样例集的雏形（`tools/md-fixtures/`）。

## 2. 目标与范围

在打地基（P2–P6）之前，回答五个会影响架构的问题。每一项的产出是一个明确的结论：**可行**、**不可行**，或者**可行但需要某种做法**，并写明对后续 Phase 或 M 的具体影响。

不做：任何产品代码；性能调优；超出"能否这样做、需要注意什么"的深入实现。

## 3. 设计：五项实验

### ① pg_trgm 与中文、数据库 locale、大小写折叠

**问题**：
1. 在哪种 locale 下，pg_trgm 能把中文切成三元组？
2. 1、2、3 个字和中英混合的查询，能否用上 GIN 索引？
3. 用不上索引时，ILIKE 顺序扫描的代价有多大？
4. 标题"不区分大小写唯一"应该用 `lower()` 还是 PostgreSQL 18 的 `casefold()`？各自受 locale 影响吗？

**做法**：
- 用 `postgres:18.6` 分别以三种方式初始化数据库：libc `en_US.UTF-8`（官方镜像的默认值）、builtin `C.UTF-8`、`C`。
- 在每个库里：
  - 用 `show_trgm()` 看中文、中英混合文本的切分；
  - 造 5 万行、平均约 3 KB 的中英混合正文，建 GIN（`gin_trgm_ops`）索引；
  - 对 1、2、3、4 个字以及中英混合的 `LIKE` / `ILIKE` 查询，看 `EXPLAIN ANALYZE` 的计划与耗时。
- 对比 `lower()`、`casefold()` 在德语 ß、土耳其语 İ、希腊语 Σ、全角字母上的结果。

**判定**：
- 能定出所有环境统一使用的 locale；
- 能给出"几个字以上走索引、几个字以下兜底"的明确界线，以及兜底的实测代价；
- 定出标题查重使用的函数。

### ② goldmark 与 remark 的提取一致性

**问题**：
1. 服务端（goldmark）与前端（remark）对同一段 Markdown，识别出的链接、嵌入、标签、frontmatter 是否一致？
2. goldmark 能否给出每个链接在原文中的精确字节范围，供 M6 改写链接使用？

**做法**：
- 编写约 30 个样例，覆盖以下边界：
  - 代码块与行内代码中的 `[[x]]`、转义的 `\[[x]]`；
  - 表格里的 `[[a\|b]]`；
  - `[[a|b]]`、`[[a#h]]`、`[[#h]]`、`![[img.png|100]]`；
  - 标题与列表中的链接；
  - URL 中的 `#`、`a#b`、`#123`、`#中文`、`#a/b`；
  - 注释 `%%[[x]]%%`；
  - HTML 块；
  - frontmatter 中的链接与写坏的 YAML；
  - 带空格的 Markdown 链接 `[t](<a b.md>)`；
  - 跨行的 `[[`。
- 每个样例写一份期望结果（以 Obsidian 的行为为准，Obsidian 行为不明确的由我们定义并写明）。
- Go 侧：goldmark + GFM + 候选的 wikilink 扩展；不满足时，评估自写 inline parser 的代价。
- TS 侧：unified + remark-parse + remark-gfm + remark-frontmatter + 候选的 wikilink 插件。
- 两边各写一个提取器，把结果与期望逐项对比。

**判定**：
- 得出一份差异清单，每条差异有处理办法（选定扩展，或者自写解析器）；
- 确认字节范围能拿到；
- 样例集的格式定稿，放进 `tools/md-fixtures/`。

### ③ 事务中的 NOTIFY → SSE

**问题**：
1. `NOTIFY` 是否只在提交后送达、回滚时不送达？
2. pgx 的 `LISTEN` 连接怎样管理，断线后怎样重连？
3. 浏览器能否用 `fetch` 流式读取带 `Authorization` 头的 SSE？
4. 经过 Caddy 时有没有缓冲？
5. HTTP 服务的 `WriteTimeout` 与请求期限中间件，对长连接有什么影响？
6. `NOTIFY` 的负载上限是多少？

**做法**：
- 一个最小的 Go 服务：
  - 写入接口在事务里调用 `pg_notify` 后提交，或者回滚；
  - 一条专用连接做 `LISTEN`，把事件分发给 SSE 连接；
  - SSE 接口校验 Bearer；
  - 用 `http.ResponseController` 处理写超时。
- 用 `pg_terminate_backend` 杀掉 `LISTEN` 连接，观察重连。
- 在真实的浏览器（Playwright 驱动的 Chromium）里用 `fetch` 读流；再在前面加一层 `caddy:2.10-alpine` 重复一遍。
- 超长负载单独测试。

**判定**：
- 提交与回滚的语义得到确认；
- 定出 SSE 路由对中间件的要求（期限、写超时、响应头）；
- 定出重连策略；
- 确认 Caddy 的默认配置能用，或者写出需要的配置。

### ④ CodeMirror 6 与 React 19

**问题**：
1. 在 `StrictMode` 下的挂载与卸载是否干净？
2. 外部更新文档（阅读时收到推送、切换页面）怎样做才不打断编辑？
3. 用 `Compartment` 动态切换只读（对应编辑锁）是否可行？
4. 扩展能否按"管线"组合（对应总体设计的编辑器扩展管线）？
5. 中文输入法的组合输入是否正常？
6. 打包体积有多大？

**做法**：
- 一个最小的 Vite + React 19 应用，引入 `@codemirror/{state,view,commands,language,lang-markdown,search,autocomplete}`。
- 用 Playwright 驱动 Chromium 验证挂载与卸载、外部更新、只读切换。
- 中文输入法：通过 Chrome DevTools Protocol 的 `Input.imeSetComposition` 与 `Input.insertText` 模拟组合输入，检查文档内容与光标位置。
- 构建后统计编辑器相关代码的体积（gzip）。

**判定**：
- 得出 React 集成的推荐写法；
- 给出外部更新与只读切换的做法；
- 输入法组合输入正常（模拟不可行时，列为 M4 的人工验证项）；
- 给出体积数据。

### ⑤ Go 的 MCP SDK

**问题**：
1. 官方 Go SDK（`github.com/modelcontextprotocol/go-sdk`）能否：
   - 把 Streamable HTTP 挂在我们自己的路由上；
   - 由我们的中间件校验 `Authorization` 头，并把当前账户传进工具处理函数；
   - 按 `?notebook=` 下发不同的 instructions；
   - 提供 prompts；
   - 在工具处理函数里拿到 clientInfo？
2. Claude Code 与 Codex 能否实际连上并调用工具？

**做法**：
- 一个最小的 Go 服务，用 SDK 暴露两个工具与一个 prompt，前面加 Bearer 中间件。
- 先用 SDK 自带的客户端做完整的协议测试；再用 `claude mcp add --transport http … --header "Authorization: Bearer …"` 与 Codex 的 MCP 配置实际连接，检查连接状态与工具列表。

**判定**：
- SDK 可用，或者给出替代方案；
- 定出挂载方式、认证接入方式、按请求区分笔记本的做法；
- 写出客户端配置样例。

## 4. 实施步骤

1. 在分支 `m0-p1-spikes` 上工作；实验代码放在仓库之外的临时目录。
2. 依次完成 ①、③、⑤、④、②，每完成一项，就把结论写进第 7 节。② 最费时，放在最后；它的样例集进入仓库。
3. 汇总结论对后续 Phase 与 M 的影响，修订 [M0 总设计](00-M0-design.md)、[v0.1 总体设计](../v0.1-design.md)，并在变更记录中写明。
4. 审查：由另一个独立的审查者核对每项结论的证据是否充分、是否有遗漏的风险、样例集的期望结果是否符合 Obsidian 的行为；修复审查发现的问题。
5. 合并回 `main`，推送。

## 5. 测试与验证

本 Phase 没有产品代码。验证的方式是：每项结论都附上可复现的证据（命令、查询计划、耗时、日志片段），写在第 7 节。样例集的期望结果由 Go 与 TS 两个提取器分别跑过。

## 6. 完成标准

- 五项实验都有结论与证据。
- `tools/md-fixtures/` 的格式与首批样例进入仓库。
- 受影响的上级文档已修订，变更记录已写明。
- 审查完成，发现的问题已修复。

## 7. 结果

（完成后补写）
