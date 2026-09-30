# M0/P1 技术验证：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | [01-P1-spikes.md](../01-P1-spikes.md) 的结论与证据、`tools/md-fixtures/`、[总体设计](../../v0.1-design.md)中据此所做的修订 |
| 审查方式 | 独立审查者逐项核对证据是否支撑结论、有没有遗漏的风险、样例集是否符合 Obsidian 的行为。审查者额外做了四项实测：逐条核对样例的字节范围；用实验的 Go 提取器跑 15 个补充边界样例；在 `postgres:18.6` 上复测 pg_trgm 与 locale 的关系；实测 bluemonday 对 goldmark 输出的清洗结果 |
| 日期 | 2026-09-30 |
| 结论 | 五项可行性结论都成立，"服务端作为 Markdown 唯一权威"的方向正确，不需要重跑实验。没有 Critical；10 项 Important、1 项文档一致性问题（D-1）、5 项 Minor，全部处理完毕 |

## 处理方式

每条发现先核实，再处理。两处关键判断没有照搬审查意见，而是先实测：

- **I-1**：在同一个 PostgreSQL 实例上建了 `LC_CTYPE 'C'` 与 `'C.UTF-8'` 两个库复现，结论成立。
- **I-3、I-4**：审查提出的规则大多标着"待核实"。处理办法是写了一个核对工具，启动隔离的 Obsidian（独立的数据目录与临时库，不影响本机已有的库），通过 DevTools 协议读取它的 `metadataCache`。74 个探针文件加上原有的 32 个样例，把每一条"待核实"都换成了实测结果，然后再改规则。

## 发现与处置

| # | 级别 | 发现 | 处置 | 位置 |
|---|---|---|---|---|
| I-1 | Important | 中文三元组切分取决于 libc 的 `LC_CTYPE`，与 builtin provider 无关；"不依赖 glibc"的结论错误 | 复现成立。建库命令固定 `LC_CTYPE`，并实测了 `CREATE DATABASE` 与 `POSTGRES_INITDB_ARGS` 两种写法；更正 glibc 的结论，升级 glibc 后 `REINDEX`；自检增加 `datlocprovider` 与 `datctype` | 总体设计 1.3、7.1、14；P1 ①；M0 总设计 |
| I-2 | Important | 搜索延迟的结论外推过度：随机文本只代表罕见词；2 个字是最常见的查询长度，只能顺序扫描 | 结论写明适用范围；复核提前到 M8 设计之前，用真实语料；备选方案为 CJK 二元组的 `tsvector` | P1 ①；总体设计 14；[M8 handoff](../../M8-history-search/handoffs/M0-P1-cjk-search.md) |
| I-3 | Important | 样例集没有对照 Obsidian 验证，却被定为规范 | 新增 `tools/md-fixtures/obsidian/verify.mjs`，在 Obsidian 1.12.7 与 1.13.7 上核对。每个样例标明 `source`：52 个 `obsidian-verified`，9 个 `nerve-defined`（写明理由）；工具报告的差异与理由逐条吻合 | `tools/md-fixtures/`；P1 ② |
| I-4a | Important | 数学公式用 micromark 规则，`$5 和 [[预算]] 以及 $10` 的链接被吞掉 | Obsidian 实测不是公式。改用 Obsidian 的规则：`$` 内侧不能是空白，结尾的 `$` 后面不能是数字；公式块以第一个以 `$$` 结尾的行结束 | 样例 022、033–038；README 规则 5 |
| I-4b | Important | Markdown 链接优先于 wikilink，`[[Page]](2023)` 丢失 wikilink | Obsidian 实测是 wikilink 优先。规则改为"行内结构从左到右识别，先开始的优先"，一并解释了代码段、网址与 wikilink 的相互关系 | 样例 030、039–042；README 规则 3 |
| I-4c | Important | 标签字符集缺 `\p{M}`；emoji 与全角标点的处理应标明是有意偏离 | 加上 `\p{M}`；全角标点与 emoji 截断标为 `nerve-defined`。另外实测发现 `#` 位于一段文字开头时也是标签（`**#t**`、`[[a]]#t`），规则一并修正 | 样例 016、045–048；README 规则 9 |
| I-4d | Important | `%%` 注释里的链接不进索引，重命名时会变成断链 | Obsidian 实测照样索引注释里的链接和标签。规则改为"注释只影响显示，不影响提取"；没有采用审查建议的 `hidden` 标记，因为没有使用方需要它 | 样例 021、043、044；README 规则 6；总体设计 4.1 |
| I-4e | Important | 属性链接按 frontmatter 原文扫描，YAML 注释和嵌套列表也被算进去 | Obsidian 实测：只有整个值恰好是一个 wikilink 或 Markdown 链接的字符串才算。照此定义，并记下属性路径 `key`（M10 的来源规则要用）；`kind` 改为只表示写法 | 样例 017、031、060；README 规则 10；总体设计 4.4 |
| I-4f | Important | YAML 按 1.1 解析：日期变成时间、`010` 变成 8 | 按 YAML 1.2 core schema（与 Obsidian 实测一致） | 样例 055；README 规则 1 |
| I-5 | Important | "同一次解析"与实验中验证的算法不符；提取与渲染本来就是两次解析；重叠规则未定义 | 定义唯一的解析入口 `Parse(原文) → (语法树, 提取结果)`，wikilink、标签、注释都是语法树节点，提取与渲染都从它出发；实验中"排除区间加原文扫描"的算法不再采用。字面自动链接纳入同一套规则；重叠规则即"从左到右，先开始的优先" | 总体设计 4.3、12.2、12.4 |
| I-6 | Important | 整页清洗的模型不成立：清掉功能标记，放宽又让用户 HTML 能伪造标记 | 用户 HTML 用严格策略单独清洗；渲染器的标记在清洗之外生成；前端据此发起的操作由服务端核对（勾选任务提交 `(revision, 字节位置)`）；KaTeX `trust: false`、mermaid `strict`；CSP 的 `style-src` 在 M6 确定；标题 id 加前缀 | 总体设计 4.6、12.2 |
| I-7 | Important | 渲染的链接解析来源与缓存没有定义；耗时预算缺失 | 链接状态按字节位置取自链接索引；不按 `revision` 缓存 HTML，响应带 `revision`；新增链接状态变化的推送（M6 注册）；编辑切到阅读之前先保存，不需要未保存内容的渲染接口；提取移到写事务之外；M4 定耗时预算并做耗时模糊测试 | 总体设计 4.3、12.4、14 |
| I-8 | Important | SSE 授权在连接建立后不再校验；HTTP/1.1 的 6 连接上限 | 每个浏览器一条面向账户的事件流，标签页之间共享；令牌到期时关闭；新增 M3 的"可见性变化事件"扩展点，触发关闭相关账户的连接；反向代理的验证移交 M5 | 总体设计 3.11、12.4；[M5 handoff](../../M5-collab-editing/handoffs/M0-P1-sse-proxies.md) |
| I-9 | Important | MCP 会话生命周期未验证 | `SessionTimeout`、server 缓存的 LRU 回收、每个请求先判权限再查缓存、工具注解、审批选项的说明写进设计；重启恢复与无状态模式移交 M9 | 总体设计 6.3、14；[M9 handoff](../../M9-mcp/handoffs/M0-P1-mcp-sessions.md) |
| I-10 | Important | 切换页面也用"只替换差异"会串撤销历史；输入法只在 Chromium 上模拟；扩展管线问题未回答 | 切换页面新建 `EditorState`，外部替换不进撤销历史；`compositionend` 后补存；真实输入法、扩展管线、分包体积移交 M4 | 总体设计 3.9、9.3；[M4 handoff](../../M4-pages/handoffs/M0-P1-editor.md) |
| D-1 | Important | M0 总设计没有接住 P1 的结论 | 新增"P1 结论对 M0 各 Phase 的要求"：P2 的建库参数与样例集自检；P3 的 pg_trgm 迁移、locale 自检、长连接路由的豁免；P5 不引入 remark；P6 的建库参数。另外 P1 验证表加"结论"一列，新增变更记录。frontmatter 空白化由 M4 与 M6 共用同一个 `Parse`，写进总体设计 4.3 与 12.2 | [M0 总设计](../00-M0-design.md) |
| M-1 | Minor | 文档里残留的旧说法 | 13.3-2、第 10 节、4.3、12.2 的 M0 行、M0 总设计的 ② 行均已更新；"前端不解析 Markdown"改为"不做语义解析，lezer 只用于编辑辅助" | 总体设计；M0 总设计 |
| M-2 | Minor | 缺少 M6 需要的样例和重命名改写样例 | 新增：BOM、表格外用反斜杠转义的竖线、多个竖线与 `#`、`[[p.md]]`、NFD 目标、未闭合 / 非映射 / 空的 frontmatter、标签的大小写与重复、锚点解码、`/abs.md` 等。新增 `rename/` 的格式与 4 个样例，覆盖写法保持、别名、CRLF、YAML 单引号 | 样例 045、051–059；`tools/md-fixtures/rename/` |
| M-3 | Minor | 标题键的定义与 Unicode 升级 | 键为 `NFC(fold(NFC(s)))`；升级 Unicode 数据后用 `nervewiki reindex` 重算；完整折叠比 Obsidian 严，写明导入时按重名处理 | 总体设计 3.5 |
| M-4 | Minor | 证据的可复现性；97.4% 的残差里有提取器自身的缺陷 | 造数 SQL 与对抗性文档生成器写进 P1 文档第 8 节；残差的说法已更正 | P1 ②、第 8 节 |
| M-5 | Minor | 带 `NOTIFY` 的提交在全局锁上串行 | 每个事务最多一条 `NOTIFY`，导入与 `batch` 合并；超过负载上限时改发整体刷新事件 | 总体设计 3.11 |

## 核实修复

- `node tools/md-fixtures/check.mjs`：61 个样例、4 个改写样例全部通过。故意改坏三处（字节范围错位、属性链接缺 `key`、`nerve-defined` 缺 `note`）的反向对照，三处都报错。
- `node tools/md-fixtures/obsidian/verify.mjs check`：在 Obsidian 1.12.7 与 1.13.7 上，52 个 `obsidian-verified` 样例全部一致；9 个 `nerve-defined` 样例的差异与各自的说明逐条吻合。
- PostgreSQL 18.6：`LOCALE_PROVIDER builtin LOCALE 'C.UTF-8'` 建出的库，以及用 `POSTGRES_INITDB_ARGS="--locale-provider=builtin --locale=C.UTF-8"` 初始化后以默认参数建出的库，都是 provider `b`、collate 与 ctype 均为 `C.UTF-8`，`show_trgm('中文搜索')` 正常。
- 文档中不再有"前后端各自解析""micromark 规则""不依赖 glibc"等旧说法。

## 遗留

没有未处理的发现。延后的验证项都以 handoff 移交，并登记在总体设计的风险表中：

- [M4：编辑器的遗留验证项](../../M4-pages/handoffs/M0-P1-editor.md)
- [M5：SSE 经过反向代理的验证](../../M5-collab-editing/handoffs/M0-P1-sse-proxies.md)
- [M8：中文搜索方案的复核](../../M8-history-search/handoffs/M0-P1-cjk-search.md)
- [M9：MCP 会话生命周期的验证](../../M9-mcp/handoffs/M0-P1-mcp-sessions.md)
