# M9/P1 平台与读的工具设计：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | [01-P1-platform-reads.md](../01-P1-platform-reads.md)（初稿，2026-10-17），对照 `main` `e16a98f7` 的代码、[00 号文档](../00-M9-design.md)、`v0.1-design.md` 第 13 节、SDK v1.8.0 与 jsonschema-go v0.4.3（模块缓存，只读） |
| 审查方式 | 两位审查者（Opus）并行，只读：<br>A：平台、传输、边缘、安全与客户端名称（1、2.1–2.2 的这几部分、3.1–3.7、3.15.2、3.16、3.17 的码的次序、3.18、3.19、第 5 节每个 Step 的"合并是绿的"、6.3、6.4）。在一份 `server/` 副本里照初稿改了 `middleware.go` 的前缀与 `api.go` 的 `Wrap`，以 httpserver 包内的测试 `mcpexp_test.go` 在真的 `middleware()` 链、真的 `bodyshape` 与 SDK v1.8.0 上重写了初稿写出的 mcpserver 的部分（`ServeHTTP`、`noStore`、403 与 `notebook` 的判定、`outer`、`handshake`、包装与恢复），经 `httptest` 驱动；另有 `CleanClientName` 两种次序的 30 万例的性质测试、`encoding/json` 的转义与 `url.Query`、`slog` 过滤的小程序，副本上以 `sqlc generate` 编译了 00042 与两条 identity 的语句。工具链 go1.27.1，`GOPROXY=off`，没有 Docker 与数据库。<br>B：读的工具、说明、账户入口、测试与文档。一份 `server/` 副本加上初稿的全部 SQL，以仓库的 sqlc 生成（都编译，生成的类型逐个对过端口）；照 3.10.1 写了初稿的 glob 匹配（贪心、取最左、带 `?` 的片段逐个比），在 10 万个节点、每个 11 段、每段 255 个 `a` 的树上计时，另以逐字节的写法计一次。没有跑数据库与测试。 |
| 日期 | 2026-10-17 |
| 结论 | A：Important 1、Minor 16、Nit 15，过度设计 2。B：Important 5、Minor 14、Nit 14，过度设计 4，另旁及两处。同一处的四对：A-I1 与 B-I4（注册表的规则），A-M16 与 B-M3（`FirstChildren`），A-M14 与 B-M14（试用的时机），A-M13 与 B 旁及的 `var Bodies`（包级的变量）。<br>全部处理，修订直接写进设计，与这份记录一起提交。处置由主持者定下（下面"主持者的决定"一节），负责人可以改判。<br>六个 Important 都按主持者的方向改了：注册表的规则改正并加接线的测试（A-I1、B-I4）；答复只有一个量、一个帮助函数，`open_notebook` 按剩下的预算填（B-I1、A-M4）；最后的变更单独一条语句（B-I2）；WebP 的宽高自己读 RIFF 头（B-I3）；glob 换成线性的匹配并定下一次的工作上限 `MaxGlobWork` = 5 亿个单位（第二轮按键的字节计、降为 4 亿），实测写进 3.10.1（B-I5）。<br>两位的过度设计全部采纳，`MaxAccountNotebooks` 一条照主持者的条件留下并写明理由（见末节）。<br>设计 2.1 由 13 条变为 22 条（第 2、3、7、9、10、12、13 条按审查改写，另加第 14–22 条）；2.2 仍是"无"。00 号文档的改动由 `apply-00m9.py` 落下（含 A-M4 对 4.6 的转义规则的更正）。 |

## 发现与处置

节号是修订后的。

### A：平台、传输、边缘、安全与客户端名称

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| A-I1 | Important | `New` 对"`Notebook` 而不在笔记本入口的工具"panic，3.4.8 还测它；而 `open_notebook` 正是 `Entry: AccountEntry, Notebook: true`，S3 的 `wireApp` 会在 `New` 里 panic，`serve` 与每个整个程序的测试都失败。按 3.3.2 自己的定义，`Notebook` 只在账户入口里有作用，规则写反了 | 规则改为"`Notebook` 而不在账户入口就 panic"（3.3.1）；3.4.8 的 panic 的列表随之改，schema 的单元测试加只在账户入口的 `Notebook` 工具（`TestAnAccountNotebookToolRegisters`）。另加 `TestTheMCPToolsBuildBothServers`：以 serve 的同一条路建应用、`recover` 住 `New` 的 panic 并报出，再核对 `mcpTools` 的每个 `Spec` 恰好在它的入口里；变异"把规则改回初稿的"在 S3 由它抓住（3.5、3.19、6.3） |
| A-M1 | Minor | `r.URL.Query()` 丢掉 `ParseQuery` 的错误、悄悄去掉解析不了的一对：`notebook=%zz`、`notebook=<id>;x=1` 都落到账户入口，正是 00 号文档 4.6 不要的 | 改用 `url.ParseQuery(u.RawQuery)`；解析出错一律答 404 `notebook.not_found`、不查库（不只在原文提到 `notebook` 时：查询串只有这一个参数有意义，坏的查询串不猜它的意思）；两种情形进 3.4.8 的取值与 3.4.9；00 号文档 4.6 与第 9 节随之（2.1 第 18 条，3.2.4） |
| A-M2 | Minor | `slog.New(warnAndAbove{…}).With(…)`：`With` 调包装的 `WithAttrs`，嵌入的 `Handler` 答出里面的、不过滤的；实测 `Info` 照样打出，SDK 每个无状态的 POST 约九行 info、不带请求号 | 属性放进包装之内（`logger.Handler().WithAttrs(…)`），`warnAndAbove` 另把 `WithAttrs`、`WithGroup` 包回自己；单元测试 `TestTheSDKLogsOnlyWarnings`：一个无状态的 POST 之后收集器里没有 SDK 的 info 行（3.4.5、3.19） |
| A-M3 | Minor | 方法比请求自己的期限活得久时，SDK 的 `hangResponse` 先返回、`session.Close()` 等方法做完，谁也没写，net/http 以 SDK 留下的头答一个空的 200（`no-cache, no-transform`）；实测 3.5 秒后的空 200。"每个答复都带 `no-store`"不成立，客户端也解析不了 | `ServeHTTP` 在 SDK 返回之后看 `noStore.wrote`：没写就记 error "MCP answer not written"、写 500 problem（`no-store`）；单元测试：短的请求期限、不看 ctx 的方法（`TestAnUnwrittenAnswerIsAProblem`）（3.2.2、3.4.3、3.4.8） |
| A-M4 | Minor | 3.4.4 与 00 号文档 4.6 的"别的控制字符六个"不是 `encoding/json` 的规则：`\r`、`\t`、`\b`、`\f` 是两个字节，U+007F 一个，C1 两个，不是 UTF-8 的三个；CRLF 的页每行少四个字节的预算，00 号文档的规则还管着说明与 `open_notebook` | 与 B-I1 一起：答复只有一个量，表照 A 实测的写进 3.4.4（另加 `<>&` 六个字节：文字块由 SDK 以 `json.Marshal` 写，`exp/wire` 实测）；00 号文档 4.6 的规则由 `apply-00m9.py` 更正（2.1 第 7 条） |
| A-M5 | Minor | `LastBoundary` 取在前 128 个字符上，总多丢一个字符：129 个 ASCII 变 127 个，127 个 `a` 加 `é` 加 `x` 丢了放得下的 `é` | 取前 129 个字符的 `LastBoundary`；表格加这两例，性质测试加"全是起始字符的长名称恰好留下 128 个"（`TestCleanClientNameKeepsAllItCan`）；2.1 第 2 条随之（3.16.1、3.19、6.3） |
| A-M6 | Minor | `RecordAPITokenClientName :exec` 在 `SKIP LOCKED` 跳过一行时也答 nil，被缓存成"记下了"，进程不再重试；"读失败当作没有并缓存"可以读成失败的读也被缓存，那个令牌余生都是 `unknown` | 语句改为 `:execrows`，`Record` 答"写没写"；0 行不缓存，下一次报名称时再试（不另读一次去分辨"已是它"与"跳过"，见末节）；读失败不缓存。单元测试：跳过之后、读失败之后的下一次都再到端口（`TestTheNamesRetryASkippedWrite`、`TestTheNamesRetryAFailedRead`）（3.16.3、3.16.4、3.19） |
| A-M7 | Minor | 有 prompt 而 `Capabilities.Prompts` 为 nil 时 SDK 自己填 `listChanged: true`；`Options` 没有 `Prompts`，`Prompt` 的形状没写，而 MCP 的 prompt 参数是字符串的 `PromptArgument`、不是 JSON schema | `Options.Prompts`；有 prompt 时 `Capabilities.Prompts` 设 `{ListChanged: false}`；`Prompt` 写成最小的形状：名称、标题、说明、`[]PromptArgument`、返回说明与文字消息的处理函数（同 A 的过度设计第 1 条）；00 号文档 4.1 的"同形"随之（2.1 第 20 条，3.3.1、3.4.7） |
| A-M8 | Minor | 结构表的根改成 `Any` 之后，带 `2025-11-25` 头的批由 SDK 自己拒绝（纯文字 400，`echo` 0 次）：带协商出的头的反向对照抓不住变异 | 反向对照的批不带头或带 2025-03-26；3.4.9 的正向测试对三种头都断言 problem+json 的 `bad_request`，分得出边缘的拒绝与 SDK 的（3.4.9、3.19） |
| A-M9 | Minor | 旧协议的 listen：`params: {}` 立刻答参数不对，`{"notifications":{}}` 即使 `ListChanged` 为真也立刻答完；不订阅的测试什么也证明不了 | 请求体是 `{"notifications":{"toolsListChanged":true,"promptsListChanged":true}}`；反向对照：`Tools.ListChanged` 改为真时它持到调用的期限（3.4.9、3.19） |
| A-M10 | Minor | 工具清单的检查在 S1 是空的、也只在合成的清单上有反向对照：列错入口、读错键、什么也没读都通过；只比五个成员，线上多出的 `outputSchema`、`_meta` 看不到；`required` 按次序比，`notebook` 要放在最前 | 主测试取原始的 `tools/list` POST 的 `result.tools`，比整个工具对象；文件恰好两个键；另加经真的路径的反向对照 `TestTheToolListCheckReadsTheApp`（接缝多登记一个 `seam` 工具，比较失败并点名它）；写明 `notebook` 在 `required` 的最前；第 5 节写明它在各 Step 证明什么（3.3.3、3.5、5、3.19） |
| A-M11 | Minor | MCP 的矩阵没有 REST 的 `write` 的检查，P2 的写会在读的那份库上跑；S1 也是空的，拼错的入口键永远通过 | `mcpRow` 加 `write`；`readOnlyHint` 为假而行没有 `write`、为真而有，都报；文件的两个键必须在；反向对照随之加三种（3.6、3.19、第 8 节 P2） |
| A-M12 | Minor | 无状态时通知到不了中间件（40 个只到了一个，都 202）：`servePOST` 写了 202，`serveStateless` 的 `session.Close()` 就让读循环不再收；3.4.1 的"`notifications/cancelled` 取消它"不成立，取消只来自客户端结束请求，`AfterFunc` 的变异只在断言 `FaultGone` 时抓得住 | 3.4.1 与第 1 节改写：通知是空操作，名称、日志、取消都不依赖它，取消只经 `AfterFunc(r.ctx, cancel)`；3.4.9 的断开测试断言 1 秒之内取消、`FaultGone` 的 debug（`TestTheMCPCallStopsWhenTheClientGoes`）；00 号文档 4.3 加这一句（2.1 第 16 条） |
| A-M13 | Minor | `var Bodies`、处理器的值 `Subtree`、`WellKnown` 是包级的变量，`gochecknoglobals` 让 S1 的 lint 是红的 | 都改成函数：`Bodies()`、`Subtree()`、`WellKnown()`；要答 problem 的哨兵也写成函数 `notPAT()`，只留 `errors.New` 的哨兵（3.2.2、3.1、5） |
| A-M14 | Minor | 试用整个放在 S2 的末尾，而只有第 1 步（`list`、`read`）要真的工具；第 2–5 步在 S1 的二进制上就能做，00 号文档第 10 节靠的正是它们 | 第 2–5 步在 S1 的末尾、合并之前，第 1 步在 S2 的末尾；2.1 第 9 条随之（6.4、5） |
| A-M15 | Minor | 两个变异抓不住："`handshake` 在 `next` 之前读说明"（没有测试数 `Instructions` 的调用）、"`names` 不缓存空的结果"（淘汰的测试用的是有名称的令牌） | 数调用的替身：一次握手 `Instructions` 恰好一次，`ping`、`tools/list`、`tools/call` 为 0；从没报过名称的令牌两次调用只读一次端口。6.3 改成表，每个变异点名抓住它的测试（3.15.2、3.16.4、6.3） |
| A-M16 | Minor | `FirstChildren` 的 `parent_id IS NOT DISTINCT FROM` 不能做索引条件，10 万个节点的笔记本每次 `open_notebook` 都读全部节点；只以根调用 | 写 `parent_id IS NULL`、去掉 `narg`，计划测试断言它是索引条件（同 B-M3，3.17.3） |
| A-N1 | Nit | SDK 先查 `Mcp-Protocol-Version` 再看方法（GET 带不认识的版本答 400 而不是 405，POST 的在 404 之后）；闸门的 429 在坏的令牌的 401 之前 | 次序表改写：没有令牌 401 → 闸门 429 → 坏的令牌 401 → 桶 429；不是 POST 的 400 → 405；POST 的版本 400 在 404 之后；00 号文档 4.4、4.15 随之（2.1 第 22 条，3.2.2） |
| A-N2 | Nit | "nil checks none"不对：`bodyshape.Middleware` 读 nil 表的 `Roots`，`Wrap(h, nil)` panic | 去掉那句，写明不能为 nil、`/mcp` 总交 `Bodies()`；不为它改 `bodyshape`（3.2.1） |
| A-N3 | Nit | 未导出的常量 mcpserver 的测试读不到 | 导出 `httpserver.MCPPath`，`const Pattern = httpserver.MCPPath`，不要测试（3.2.2、3.2.3） |
| A-N4 | Nit | 普通的 Go 错误出去是 `code: 0` | 写 `&jsonrpc.Error{Code: jsonrpc.CodeInternalError, …}`；单元测试断言 -32603（3.4.2、3.4.8） |
| A-N5 | Nit | `arguments` 不是对象时根的路径写成 `(: invalid_format: …)` | 空的路径写作 `arguments`（3.3.6、3.4.8；2.1 第 3 条） |
| A-N6 | Nit | `segmentio/asm` 会是 v1.2.1（goose v3.28.0 要它） | 3.1 改为离线 `go mod tidy` 真选的版本；作者在 `exp/tidy/` 上重核了全部：七个模块，`jsonschema-go` 是直接依赖（见"作者的更正"）（3.1；2.1 第 21 条） |
| A-N7 | Nit | jsonschema-go 把 `time.Time`、`slog.Level`、`big.*` 推导成 `string`，"`time.Time` panic"要走 Go 的类型；不是指针的 `omitempty` 的意思没写 | 走 Go 的类型；不是指针的 `omitempty` panic（可选的一律写成指针）（3.3.3、3.4.8） |
| A-N8 | Nit | PAT 的数目不一（13 与 19）；`prepareMatrix` 有六个使用者 | 13 个，只给笔记本级的列，在 MCP 矩阵测试自己的准备里建（同 B-N4；2.1 第 10 条，3.6） |
| A-N9 | Nit | 新的整个程序测试的名字都落在 bootstrap 的第二半（`^Test[A-D]` 之外），它已是较大的一半 | 6.1 写明落在第二半、估计加 25–35 秒、S1 合并之前量两半，差一成以上时照 M8/P4A 挪界线 |
| A-N10 | Nit | archtest 的图不含测试文件，一个模块的测试导入 SDK 看不到 | 另加 depguard 的规则（连测试文件），只许 mcpserver 与 bootstrap 之下；反向对照在 S1 的审查里手工核对一次（3.7、3.19；2.1 第 21 条） |
| A-N11 | Nit | `structuredContent.error` 的 `fields`、`locks`、`Extra` 不计入上限，而 Codex 把 `structuredContent` 交给模型 | 两样都做：错误的答复连同 `structuredContent.error` 也按同一个量计、在上限之内；`lock`、`locks`、`Extra` 的上限交给 P2（3.3.6、第 8 节） |
| A-N12 | Nit | SDK 的 -32602 复述工具名、不设上限 | 写明无害（名称是客户端写的，答复只回到它那里）（3.3.6） |
| A-N13 | Nit | `ADD COLUMN … CHECK` 要在 ACCESS EXCLUSIVE 下扫一遍表核对约束，不是"只改目录" | 改写这句，写明 `api_tokens` 小、毫秒级、接受（3.16.2） |
| A-N14 | Nit | 每个方法（含 `tools/call`）都在 `handleAsync` 起的 goroutine 里，`Async` 只放开队列 | 第 1 节与 3.4.2 改写 |
| A-N15 | Nit | `listChanged: false` 在线上写成 `"tools":{}` | 测试解码能力，不找字面（3.4.8、3.4.9） |
| A 过度设计 1 | — | P1 的 `mcpserver.Prompt` 带自己的参数与结果 | 采纳：最小的形状（同 A-M7），M10 按需要加（3.4.7、第 8 节；2.1 第 20 条） |
| A 过度设计 2 | — | `entryOf` 的"规范的 UUID"（`id.String() == value`）比 REST 严 | 采纳：照 REST 的 `uuid.Parse`，日志与 ctx 记规范的小写（3.2.4；2.1 第 18 条） |

### B：读的工具、说明、账户入口、测试与文档

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B-I1 | Important | `open_notebook` 的算术超出 `MaxAnswerBytes`：32,768 + 16,384 已是 49,152，键名在其上；结构的工具的文字块是 JSON 文字，说明被转义两次，一个按 `TextBytes` 截到 32 KiB 的说明可以占约 64 KiB；mcpserver 的再量一次答 `internal_error`。`list`、`notebooks` 的"加上这一项超过上限就停"没说怎么数 | 照主持者的决定：一个量（每块文字在答复里由 `encoding/json` 写出的字节，A-M4 的表加 `<>&`；结构的工具的字符串转义两次，对线上诚实）、一个帮助函数（mcpserver 的 `TextBytes`、`MemberBytes` 与 `Fit`，`list`、`notebooks`、`open_notebook` 都经它，适配器不自己数；page 不再有第二份计数，由调用者交计数函数）。`open_notebook` 按剩下的预算填：骨架 S、给根下的节点留 R = 8,192 字节，说明按 `MemberBytes` 截到 49,152 − S − R（通常约 40.7 KiB，带说明行），再经 `Fit` 放根下的节点（至多 100 个）。B 的测试：全是 `"` 与 `\n` 的 32 KiB `AGENTS` 加 100 个 255 字节名称的根节点，答复至多 49,152、不是 `internal_error`（`TestOpenNotebookFitsItsAnswer`）。握手的 32 KiB 用同一个量（3.4.4、3.10.5、3.15.3、3.17.2、3.17.3；2.1 第 7 条） |
| B-I2 | Important | `NotebookOverview` 的 `last_change` 被 sqlc 生成为不可空的 `time.Time`，没有变更集的笔记本（新的、空的）扫描 NULL 失败，`open_notebook` 答 `internal_error`；矩阵、MC1 与 6.2 都测不到 | 最后的变更单独一条 `NotebookLastChange`：`max(updated_at)`（B-M12）加 `GROUP BY`，没有变更集时零行，存储把 `ErrNoRows` 换成 nil；计数另一条 `NotebookCounts`。空的笔记本进三处：整个程序的 `TestOpenNotebookOfAnEmptyNotebook`、存储的测试、MC1 的第 4a 步（矩阵的笔记本都种了变更集，所以不放在矩阵里）（3.17.3、3.17.4、4.3；2.1 第 17 条） |
| B-I3 | Important | 服务端只读 PNG、JPEG、GIF 的宽高，WebP 总是 0，`read` 从不内联它，与 00 号文档 4.9、第 11 节的取舍第 7 条与工具的说明相悖 | 照主持者的决定：不加依赖（`golang.org/x/image` 不在 `go.mod` 与缓存里），`asset/adapter/sniff` 在 `image.DecodeConfig` 旁边自己读 RIFF 头（VP8、VP8L、VP8X），不登记 `image` 的格式；`HasSize` 加 `image/webp`，上传从此记下 WebP 的宽高（改了 M7 的上传，写进 2.1 第 14 条与第 7 节的文档改动：`asset.yaml` 的说明与 `make gen`、M7/P2 设计的一句注、README）；之前的、没有宽高的行由 `read` 从它为内联读的字节里读（`AttachmentReads.Content` 答宽高）。测试：模糊测试、三种块、每个长度的截短的头、`VP8L` 的 `+1` 的变异；整个程序与 MC2 各有一个 WebP。第 11 节那一条不变（3.11.2、3.13、4.4、6.3） |
| B-I4 | Important | 同 A-I1 | 同 A-I1 |
| B-I5 | Important | 初稿的贪心匹配下，3.10.1 点名的"最坏"模式是便宜的；真正贵的是中间组里带 `?` 的中间片段（`**/*P*/**`）：B 的守卫树上逐字符 12.1 秒、逐字节 3.2–5.9 秒，S2 的"超过 5 秒才加上限"量的是错的负载 | 照主持者的决定，现在定：匹配换成线性的（每段一个位并行的 Shift-And 自动机、段之间一个自动机、按树走时每个节点从父节点接过状态，代价随节点数而不是节点数乘深度；RE2 也线性但每条路径从头跑、不共享祖先，实测每条 1.5–1.8 ms，不取）；一次 `list` 的匹配至多 `MaxGlobWork` = 5 亿个单位（一个字乘一个字符，每项另 256），用完就停、答 `next_cursor`（按路径键的次序，同 48 KiB 时的意思）。守卫用 B 的形状与 B 的守卫树（另有真的树），实测写进 3.10.1：到上限时本机 0.39–0.57 秒停下（慢 2.5 倍的主机约 1.5 秒），与正则的差分 30 万对 0 处不同。`grep` 复用它写进第 8 节（3.10.1、3.10.4、3.18、6.3；2.1 第 15 条） |
| B-M1 | Minor | 内置说明的测试在 `page/adapter/mcp`，看不到 `notebooks` 与 P4 的 linking 的工具；"`<name> ` 出现在段里"在 P2 加了 `write` 等普通的词之后会被正文配上 | 移到 bootstrap 的 `TestTheBuiltInInstructionsNameEveryTool`，取 `api/mcp/tools.json` 每个入口的工具，锚在行首 `(?m)^- <name>[: ]`（3.15.4、3.19） |
| B-M2 | Minor | `list` 为每个节点拼路径与键：10 万个节点、10 层长名称时约 280 MB，`**/*.md` 还要全部排序 | 按树走、栈上共享祖先的键、每个父节点排好的项（页 `键.md`、子树的块 `键/`、附件）就是整串的次序，只为答出的一页拼路径；内存写进 M12 的移交；P4 的 `Paths` 只为问到的 id 拼（3.10.1、3.10.4、第 8 节） |
| B-M3 | Minor | 同 A-M16 | 同 A-M16 |
| B-M4 | Minor | `Prefix` 的定义把字面的 glob 的最后一段也算进去，`list("Guide/Setup.md")` 会答 `[]` | 去掉 `Prefix`：按树走时状态在第一段不配的子树上就空了，收窄是自然的，最后一段总参与匹配（3.10.1） |
| B-M5 | Minor | 3.13 说 `pattern` 带 NUL 答 422，而 `ParseGlob` 没有这条规则 | 取 (a)：字面里标题不能有的字符（`CheckTitle` 禁的与 `unshowable`）与段首尾的空白答 422 `invalid_format`；3.13 的两个直接的用例断言码（3.10.1、3.13） |
| B-M6 | Minor | 游标的形状检查不全（缺 `o` 或 `i`、glob 没有 `k`、`{}`） | 两种载荷类型（`ChildrenCursor`、`GlobCursor`），不带 `omitempty`，按模式解，`DecodeCursor` 的逐字节重编码按构造拒绝别的形状；次序改为 `pattern` 422 再游标 400（3.10.2、3.10.4；同 B 的过度设计第 2 条） |
| B-M7 | Minor | 首尾有空白的段过了 `CheckTitle`，只在 `got == NFC(seg)` 上失败，没有码与消息 | 422 `invalid_format`，"segment N must not start or end with white space"（3.9.1；2.1 第 12 条） |
| B-M8 | Minor | `mcp.ts` 的"HTTP 不是 200 时抛出"在 `notifications/initialized` 的 202 上抛出 | 写明：有 `id` 的是 200 JSON，没有 `id` 的是 202 空的正文（4.2） |
| B-M9 | Minor | knip 只认别的文件的导入，S2 里 `postMcp` 与 `mcpURL` 没人导入，`make knip` 失败 | MC2 用 `postMcp`（`notebook=nope` 答 404）；`mcpURL` 在 S2 是文件内的函数，S3 随 MC1 导出；4.1 列出每个 Step 的导出与导入它的故事（4.1、4.4、5） |
| B-M10 | Minor | `newTeam` 不给 PAT 的 id；同一个测试里第二个 `newTeam` 会撞上同一个邮箱 | 甲、乙都经 `createToken` 另建；第二个团队以 `emailFor(testInfo, "other-admin")` 注册、`slugFor(testInfo, "other")`（4.3） |
| B-M11 | Minor | 建页的次序就是 id 与次序值的次序，按 id 排的错误测不出；`has_children` 与 00 号文档第 9 节的"MC2 核对附件的行"都没有 | 经 REST 把 `Wide` 移到最前（树的次序既不是 id 的也不是名称的，见"作者的更正"）；断言 `has_children`；四个上传以 `expectUploaded` 核对行，`read` 的元数据等于行（4.4） |
| B-M12 | Minor | 初稿取最后一个变更集的 `created_at`，网页的活动取 `max(updated_at)`：长的编辑会话里会把会话的开始说成最后改动 | 取 `max(updated_at)`，同网页的活动，单独一条语句；代价同网页（读这本笔记本的全部变更集）写进 M12 的移交；00 号文档 4.6 随之（3.17.3；2.1 第 17 条） |
| B-M13 | Minor | 第 7 节的 13.1 改动漏了 SDK 的导入规则 | 加"13.1 第 23 条之后另起一条"，照 River 的写法，含 depguard（第 7 节） |
| B-M14 | Minor | 同 A-M14 | 同 A-M14 |
| B-N1 | Nit | 缓存的键没有标题，只改大小写的改名之后说明行与 `instructions_from` 留着旧的写法 | 缓存只存正文与"截过没有"，说明行在每次用时以点查读到的现在的标题拼上（`AgentsPage` 也读 `n.name`）（3.15.1、3.15.3） |
| B-N2 | Nit | 键可以比名称长（`ŉ`、`İ`），游标至多约 5.6 KiB | 改写（3.10.2、3.10.5） |
| B-N3 | Nit | 工作区的名称是 `varchar(80)` 个字符、至多 320 字节 | 改写（3.17.2） |
| B-N4 | Nit | 2.1 第 10 条的"19 次"与 3.6 的 13 列 | 同 A-N8：13 个，只给笔记本级的列 |
| B-N5 | Nit | 账户入口里读其余的部分也要 `notebook` | 取审查者的第二种（不改）：账户入口的说明已写"每个工具都带 `notebook`"，说明行在两个入口是同一段（见末节） |
| B-N6 | Nit | 文件丢了只在内联时发现，"删了文件的附件"的测试要用 PNG | 写明，测试用 PNG（3.11.2、3.13） |
| B-N7 | Nit | 地址用的是内联的签名，下载的写法（`d=1`）也许更好 | 取下载的：地址是给 agent 取文件的，存下的文件带名称；签名照样换源成立（3.11.2、3.13、4.4） |
| B-N8 | Nit | `read` 的说明没说没有 `public_url` 时没有地址 | 加一句（3.18） |
| B-N9 | Nit | 性质测试的大写与 NFD 的变体会越过上限（`ǰ`、韩文） | 生成器留余量，越过的变体断言 `too_long`（3.9.2） |
| B-N10 | Nit | TypeScript SDK 有必须的对等依赖 `zod` | 写进 4.1 的锁文件的核对 |
| B-N11 | Nit | `list("Guide")` 静悄悄地答 `[]` | 不带通配、没有配上、加 `.md` 是页时答复带 `hint`（3.10.2、4.4、3.18） |
| B-N12 | Nit | 跨过重新编号的（o, i）游标可能重复或跳过 | 写明，接受（3.10.2） |
| B-N13 | Nit | `read` 每次读整页的正文，5 MiB 的页依次读完约搬 550 MiB | 进 M12 的移交（第 8 节） |
| B-N14 | Nit | `MaxAccountNotebooks` 经工具到不了，"1,000 项的截断"只在用例一层 | 留着并写明理由（见末节），测试放在用例一层（3.17.2、6.2） |
| B 过度设计 1 | — | `NodePaths.Paths` 与 `page.NewNodePaths(pool)` 在 P1 没有使用者 | 采纳：随 P4，写进第 8 节；00 号文档第 7 节的 P1、P4 两行随之（3.9.2；2.1 第 19 条） |
| B 过度设计 2 | — | 一个带形状检查的 `ListCursor` | 采纳：两种载荷类型（同 B-M6） |
| B 过度设计 3 | — | `FirstChildren` 的可空的父节点 | 采纳：只在根下（同 A-M16、B-M3） |
| B 过度设计 4 | — | `MaxAccountNotebooks` | 照主持者的条件留下：见末节 |
| B 旁及 1 | — | `var Bodies` 是包级的变量 | 同 A-M13 |
| B 旁及 2 | — | 3.4.4 的量的后果落在读的工具上，`list` 与 `notebooks` 要与 mcpserver 同样数 | 同 B-I1：一个帮助函数 |

## 主持者的决定

主持者在修订的简报里定下的处置（作者照做；负责人可以改判）：

1. **A-I1、B-I4**：注册表的规则照 `Notebook: true` 的真正含义；加一个"S3 的真的工具集建得起两个 server"的测试，并以变异核对它。
2. **B-I1 与 A-M4：一个量**。上限数每块文字在答复里由 `encoding/json` 写出的字节，规则照 `encoding/json` 的转义（A-M4 的表）；结构的工具的文字块是 JSON 文字，它的引号算两次：对线上诚实。所有的填法经 mcpserver 的一个帮助函数，`list`、`notebooks`、`open_notebook` 用它，适配器从不自己数。`open_notebook` 按剩下的预算填，说明在先、给根下的节点留一份，写明数字；加 B 的测试；握手的说明保留自己的 32 KiB 规则，用同一个量。
3. **B-I2**：最后的变更单独一条语句、没有行时为 nil，时刻取网页的活动的那个（B-M12）；空的笔记本进测试与 MC1 或矩阵。
4. **B-I3**：自己读 WebP 的 RIFF 头，不加依赖；`HasSize` 放宽，写明改了上传的行为；旧的行由 `read` 从它为内联读的字节里读；测试含模糊测试、三种块、截短的头与一个变异；第 11 节那一条不变。
5. **B-I5**：现在就定 glob 的工作上限：线性的匹配（说明理由）、一次调用按检查的字节计的工作预算、用完答 `next_cursor`（同 48 KiB 时的意思）、守卫用 B 的真的最坏形状、目标是到上限时每次调用约 1 秒、在草稿区实测并写进设计。
6. **A-M12**：3.4.1 与第 1 节改正，00 号文档里说法不同的经脚本改正。
7. **A-M14、B-M14**：传输的几步在 S1 的末尾，读的几步在 S2 的末尾。
8. **A-M16、B-M3**：`FirstChildren` 只在根下，`parent_id IS NULL`。
9. **A-M13**：用函数，不用包级的变量，S1 的 lint 保持绿。
10. **B-M9**：每个 Step 的末尾 knip 都通过，写明 S2 有哪些导出、谁导入。
11. **3.1 的依赖**用 `go mod tidy` 真选的版本，离线重核。
12. **过度设计**：`Prompt` 最小；`notebook=` 照 REST 的 `uuid.Parse` 并记规范的 id；`NodePaths.Paths` 移到 P4 并写进第 8 节；两种游标载荷；`FirstChildren` 只在根下；`MaxAccountNotebooks` 在"答复的上限总先到、而且 `notebooks` 按游标分页"时去掉，否则留下并写明理由。
13. **别的**：每个 Step 单独合并仍是绿的（lint、knip、工具清单、矩阵、archtest），写明各检查在各 Step 证明什么；6.3 的每个变异由一个点名的测试抓住；2.1、2.2 随之更新，2.2 保持为空，除非碰到第 11 节。
14. 其余的 Minor 与 Nit 照审查者的建议改，作者认为建议不对时写明理由、另想办法（末节）。

## 作者的更正

修订时作者自己发现的：

| # | 发现 | 处置 |
|---|---|---|
| 作者-1 | 初稿与 A-M4 的表都按 `SetEscapeHTML(false)` 数，而文字块的 `text` 由 SDK 的 `TextContent.MarshalJSON` 以 `json.Marshal` 写：`<`、`>`、`&` 在答复里是六个字节（`exp/wire` 实测）；`initialize` 的 `instructions` 却由 jsonrpc2 不带 HTML 转义地写 | 量的表加 `<>&` 六个字节，对文字块是准确的；对握手的说明多算五个字节，偏保守，取一个量而不是差五个字节的两个（3.4.4） |
| 作者-2 | 第一版的工作单位只算字，每项 32：便宜的模式每单位约 8 ns，到上限的时间就不止 1 秒（5 亿个单位 ÷ 32 是 1,560 万项，每项约 250 ns 就是约 4 秒） | 实测每项的固定开销约 250 ns，每项改计 256 个单位；之后到上限的每单位 0.64–1.03 ns（3.10.1） |
| 作者-3 | 状态集放在 `uint64` 里，要模式的段数有界：初稿只限通配的个数 | 不是 `**` 的段多于 11 个答 `too_long`（同路径的段数，它什么也配不上），状态至多 11 + 32 + 2 个（3.10.1） |
| 作者-4 | B-M11 若把 `Alpha` 移到 `Setup` 之前，树的次序就等于名称的次序，按名称排的错误又测不出 | 移的是 `Wide`：树的次序 `Wide`、`Setup`、`Alpha`、`Long`，既不是 id 的也不是名称的（4.4） |
| 作者-5 | 审查 A 的副本里 `jsonschema-go` 是间接依赖（它的实验没有直接导入），而 mcpserver 直接导入它 | `exp/tidy/` 上加一个导入 `mcp`、`jsonrpc`、`jsonschema` 的包重核：两个直接依赖、五个间接的，`go.sum` 只多这七个模块（3.1） |
| 作者-6 | 2.1 第 12、13 条引的 00 号文档的行号差一行（4.8 的"段的检查"是 :186，"按 `TitleKey` 比较"是 :188） | 改正（2.1） |
| 作者-7 | 旧的 WebP 行要在 `read` 时读宽高，`AttachmentReads.Content` 只答字节，宽高的读法在 asset | `Content` 另答宽高（行的，没有时以同一个 `Sniffer.Size` 从字节读、只留 1–65,535 的边），page 一侧的端口随之；不另加读法（3.11.2） |

## 审查者核对过、没有问题的

- **A**：边缘的次序（401 → 429 → 413、400 → 403 → 405 → 404 → SDK，每种都带 `no-store`，SDK 之前的是 problem+json、SDK 的是纯文字；`/mcpx` 不被前缀盖住）；JSON-RPC 的批在 SDK 之前没有旁路（`bodyshape` 要整个请求体是以对象开头的合法 JSON）；请求体的上限、期限与请求信息；每个方法的调用的 ctx（期限、断开、值）；两层的恢复与之后的调用照常、中间件之外只有 SDK 的三处；`no-store` 在每条路上（A-M3 之外）；`DisableLocalhostProtection`；协议版本与能力（Go 客户端协商出 2025-11-25、`cacheScope: "private"`、`ttlMs: 0`、一次连接五个限流的单位）；参数的检查从不复述取值（jsonschema-go 会，2.1 第 3 条是对的）；旧协议的 listen 在两种设置下都有界；`CleanClientName` 的新次序过了 30 万例的性质测试（旧的 133,409 例不幂等），只差截法；`client_name` 的迁移与两条语句经 sqlc 编译、不变的名称不锁不写；archtest 的规则、`go list -deps` 里没有被禁的模块；"矩阵覆盖每个工具"能失败；`reserved_test` 的例外；账户入口的码的次序；`getServer` 从不答 nil；`ProblemOf(ctx, err)` 的分类；2.1 里说 00 号文档有错的六条（第 2、3、4、5、7、13 条）都是真的。
- **B**：初稿的全部 SQL 经 sqlc 编译，类型合端口（B-I2 之外）；点查是兄弟的唯一索引上的索引条件、至多 11 次，`PageStates` 走主键，`AgentsPage` 一次点查；给出与找回的往返（存着的名称是 `CheckTitle` 的输出、附件不以 `.md` 结尾所以路径的键唯一、四种与链接不同的情形）；树的次序与（o, i）游标的往返；`read` 在 `Cut` 之下逐字节地重建正文（`\r`、没有末尾的换行、长的一行、字符的边界、超过末尾、整数参数严格）；附件的判定、签名只盖节点、blob、`e` 与 `d`、地址是相对的、SVG 按 MIME 排除、`public_url` 的校验；G16 的五种换法与恢复、还原、撤销；`FrontmatterEnd` 与解析一致；账户入口（`ListWorkspacesOf` 已读 slug 与名称、`Membership` 的字面量都带键、`ListNotebooks` 的可见性、角色）；TypeScript SDK 1.32.1 过 pnpm 的门禁；第 7 节点名的 13.1、13.4 各条都在，M12 的移交接第 18 项。

## 未采纳

或采纳的方式与审查者的建议不同：

- **B-I5 的"用完答 422 `pattern` `out_of_range`"**：照主持者的决定答 `next_cursor`。按路径键的次序已经给了接着走的地方，用完上限与答复到 48 KiB 是同一个意思；agent 拿游标接着调，或先列一个文件夹（说明写明）。422 会让一个合法而贵的模式在大的笔记本上永远答不出来。
- **B-I5 的"按比较的字节计工作"**：单位取"一个 64 位的字乘键的一个字符"，每项另 256。位并行的匹配一次比 64 个位置，按字节计会把一个 1,024 字节的模式与一个 7 字节的模式算成一样贵；每项的固定开销实测约 250 ns，不计它，节点多而模式便宜时到上限的时间就失控（作者-2）。
- **B-I1 的 (a)"结构的工具只数 JSON 文字自己的字节"**：照主持者的决定取答复里真的字节（两次转义）。只数一次会让答复在线上比上限大近一倍。
- **B-I3 的 (b)"从 4.9 与第 11 节去掉 WebP"**：没有采纳：那要负责人改判第 11 节的一条，而读 RIFF 头只要几十行、不加依赖。
- **B-M12 的另一种改法"字段改名 `last_changeset_at`"**：没有采纳：取网页的活动的时刻，名称照旧说"最后改动"，两处一致。
- **B-N5 的说明行加"（与这本笔记本的 id）"**：没有采纳：说明行在两个入口是同一段文字，账户入口的说明已写"每个工具都带 `notebook`"，`open_notebook` 的答复也带 id。
- **A-M1 的"只在原文提到 `notebook` 时答 404"**：取"解析出错一律答 404"：坏的查询串里 `notebook` 那一对可能正是解析不了的那一对（`%zz`），按名称去找它要自己再解析一遍。
- **A-M6 的"0 行时再读一次，分辨'已是它'与'跳过'"**：没有采纳：不缓存、下一次报名称时再写一次更简单；那是一条带 `IS DISTINCT FROM` 的点查，已是它时不锁不写。
- **A-N2 的"让 `bodyshape.Middleware` 对 nil 安全"**：取另一半：去掉那句、写明不能为 nil。`/mcp` 总有表，为一个不会发生的调用改共用的中间件没有收益。
- **A-M2 的两种改法**：两样都做：属性放进包装之内，`warnAndAbove` 也包回 `WithAttrs` 与 `WithGroup`，以后谁再 `With` 也留得住。
- **A-N11 的"计入或交给 P2"**：两样都做：P1 计入（同一个量，偏保守），P2 给 `lock`、`locks`、`Extra` 上限。
- **B 的过度设计第 4 条与 B-N14：`MaxAccountNotebooks`**：留着。主持者的条件是"答复的上限总先到、而且 `notebooks` 按游标分页时去掉"：前一半成立（经工具约在 330 项处停），后一半不成立（00 号文档 4.6：`notebooks` 不分页）。它管的是用例自己的工作：没有它，一个看得到很多笔记本的账户，用例要全部读出、拿在内存里，适配器再丢掉；有它，用例读到 1,000 项就停、不再读余下的工作区。测试在用例一层（3.17.2）。
- **B-M2 的"按与整串相同次序的比较函数排序"**：取等价而更省的做法：每个父节点下把页的子树当作一个块 `键/` 与兄弟一起排，键里没有 `/`，所以排好每个父节点就是整串的次序，不拼整串、不在全部的匹配上排序（3.10.1）。

## 新的问题

- 无要负责人决定的。实施时要在 CI 的机器上再量一次 glob 的守卫：`MaxGlobWork` 是在本机（Apple M5 Max）上定的，CI 的机器慢得多时守卫的 2 秒与"约 1 秒"的目标照实测调，数值写进设计的第 9 节。

## 第二轮

| 项 | 内容 |
|---|---|
| 复核对象 | 第一轮修订后的 [01-P1-platform-reads.md](../01-P1-platform-reads.md) 与上面的处置，对照 `main` `e16a98f7` 的代码 |
| 复核方式 | 两位审查者（Opus）各复核自己的一半，只读，不用容器：<br>A：经真的 SDK 核对两种量（3,000 个随机的字符串，文字块、结构的工具的文字块、一个成员的份额三种比较都准确）；`CleanClientName` 的截法（`cleanclient_exp3/4/5_test.go`）；答复没写（`mcpexp_test.go` 的 `TestXUnwrittenOnGone`）；`apply-00m9.py --check`；依赖的 `go.sum`。<br>B：修订的五条语句经 sqlc 编译；glob 的差分重跑与它自己的走查对照（随机的树 0/60,000、跨字的长段、接着走）；不是 ASCII 的键的每单位代价（`cost2_test.go`、`cost3_test.go`）。 |
| 日期 | 2026-10-17 |
| 结论 | 第一轮的处置：A 的 A-M3 只改对一半（N-2）、A-M5 没改对（N-1），别的都改对了；B 的 B-I5 只改对一半（N-1、N-2），别的都改对了。新的发现：A Minor 2、Nit 4，另两处小的写在第一轮的条目下（A-M6 的措辞、A-M1 的可选的 detail）；B Minor 2、Nit 6。没有 Important。<br>全部处理（下表），修订写进设计；`apply-00m9.py` 改了 8 处编辑的新文字（2、7b、14a、14d、15a、15c、15d、26），`--check` 46 处都对得上，没有写。 |

### 发现与处置

第一轮的条目照原样留着；被这一轮改掉的处置（A-M3 的那一支、A-M5 的截法、B-I5 的单位与接着走、"未采纳"里按字节计的一条）以下表为准。节号是这一轮修订后的。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| A N-1 | Minor | A-M5 的修订（在前 129 个字符上取 `LastBoundary`）在第 129 个字符之后也是边界时留下 129 个：128 个 `x` 加 `中`、129 个 `中`、140 个 `-`、140 个 emoji。`client_name` 的检查（`char_length BETWEEN 1 AND 128`）拒绝每次的 `Record`，名称从不记下，重启之后是 `unknown`；`Call.Client`、日志与 P2 的 `MCPClient` 都看到 129 个。2.1 第 2 条与 00 号文档 4.5（`apply-00m9.py` 的第 2 处）带着同样的写法 | 照 A 的规则：第 129 个字符之前是规范化的边界（`norm.NFC.PropertiesString(string(r[128])).BoundaryBefore()`）就截在第 128 个之后，否则截在前 128 个字符的 `LastBoundary` 处（A 在 30 万个随机的名称与 2 万个全是起始字符的名称上核对过：幂等、NFC、至多 128 个、截在边界上）。A 的两个反例与 140 个 `-` 进表格；性质测试的起始字符取自字母、汉字、谚文、数字、`-`、`.` 与 emoji；2.1 第 2 条、`apply-00m9.py` 的第 2 处与第 26 处（变更记录）随之；3.19、6.3 各加一行（3.16.1） |
| A N-2 | Minor | A-M3 的那一支在每次客户端断开时也走：SDK 的 `hangResponse` 在请求的 ctx 结束时返回，谁也没写（A 实测断开 5 次、5 次没写）。设计就记 error "MCP answer not written"，再以 `errUnwritten` 走 `Write` 的默认分支记第二行 error、往关了的连接写 500：每次 Ctrl-C、客户端超时、网络断了都是两行 error，与 3.3.6 的 `FaultGone`（debug）相悖；3.4.9 的断开测试只断言有 debug 的一行，照样通过 | `if !ns.wrote { s.errors.Write(ns, r, r.Context().Err()) }`：用 REST 的分类，客户端走了答 499、记 debug，期限到了答 500 problem+json、记 warn，`no-store` 由写者守住；ctx 还活着而什么也没写（不该有）时 `Err()` 为 nil，走默认的 error 与 500，那是接线的错。断开的测试断言日志里没有 error 级的行；3.4.8 的"答复没写"改为断言 warn，另断言断开时没有 error；3.19、6.3 各加一行（3.2.2、3.4.3、3.4.8、3.4.9） |
| A N-3 | Nit | `Fit` 的"每次至多约 50 KiB"不对：第一个候选是 `build(n)`，`list` 的 1,000 项通常约 200 KB、最坏约 2.8 MB 的 JSON。"最大的 k"与"至多少放一项"也不对：游标的长短随最后一项变，量不随 k 单调，二分得的是"k 放得下而 k + 1 放不下" | 照 A 的建议：从 1 倍增到第一个放不下的，再在它与上一个放得下的之间二分；注释与 3.4.4 写"k 是 n，或 k 放得下而 k + 1 放不下"，可能少放几项，无害；候选至多约 2 log₂ n + 1 个，每个至多是第一个放不下的那个。A 说的"每个候选在 2 × 48 KiB 加一项之内"只在后面的项不比前面的长得多时成立（后一半都是 2,818 字节、全是 `&` 的路径时可以大得多），但总不超过 `build(n)`，设计照这样写。3.4.8 的测试去掉"（或……）"的例外，另记下每个候选的 k（3.4.4、3.4.8） |
| A N-4 | Nit | `TestTheMCPToolsBuildBothServers` 读不到每个工具的 `Spec`（`Tool.spec` 不导出）；它与契约的测试多半重复 | 加访问器 `func (t Tool) Spec() Spec`（3.3.2），留着这个测试：它多出的是与 `Spec.Entry` 的核对与点名的 panic 文字，代价是一个访问器；重复的部分在 3.5 写明 |
| A N-5 | Nit | `notPAT()` 返回 `*httpserver.ProblemError`，而 `ProblemError` 是接口（`apierrors.go:39-43`），httpserver 不导出具体的 problem 类型 | mcpserver 自己一个未导出的类型：`ProblemStatus` 403、`ProblemCode` `"forbidden"`、`Error()` 是 detail（3.2.2） |
| A N-6 | Nit | `AgentsText` 拿一个不透明的 `fits`，往回逐个换行地量：在 `open_notebook` 的测试页（32 KiB 的 `"` 与 `\n`）上每次调用要量几千个约 16K 个字符的前缀，而这段文字随预算变、不缓存 | 在换行处二分：两种量都按字符相加，前缀的量随长度单调，约 15 次量、每次至多 32 KiB；性质测试对照往回逐个量的写法（3.15.3） |
| A 对 A-M6 | Nit | "0 行而库里已是它时多一次空的写"是每次报名称的调用一次，不是一次：每个请求都在 `_meta` 里报名称的客户端，重启之后名称从不进缓存。今天没有这样的 2025-11-25 的客户端，2026-07-28 也没有提供 | 照写"每一次报名称的调用"，接受（3.16.4） |
| A 对 A-M1 | 可选 | 账户入口上客户端自己的坏参数（`/mcp?x=%zz`）得到 `notebook.not_found`；detail 可以说地址的查询串解析不了 | 没有采纳，见下面的"第二轮未采纳" |
| B N-1 | Minor | 单位按字符计，而 `é`、`漢` 的一个字符约贵 ASCII 的 1.6–1.9 倍（它的掩码不在 128 项的表里，要解码 UTF-8 再查映射），守卫只有 ASCII 的键：中文的笔记本上 `**/*需求?*/**` 到上限约是守卫所量的两倍，"约 1 秒"与 2 秒的守卫都可能不成立，守卫看不到 | 单位改为"一个字乘键的一个字节"（`é` 2 个单位、`漢` 3 个），字数的因子照旧在模式那一边。另加一个 1,024 位的筛子（模式里不是 ASCII 的字符的低 10 位）：键里的字符在筛子里没有就不查映射、取 `?` 的掩码。守卫加键是 `é`、`漢` 的两棵守卫树与汉字的模式。在 `exp/glob` 重量（主机负载 7–10）：4 亿个单位时 `a` 0.36–0.65 秒、`é` 0.32–0.82 秒、`漢` 0.30–0.81 秒；没有筛子时 `é` 的键上汉字的模式要 1.21 秒；5 亿个单位时 `é` 最慢约 1 秒（负载 4–5），所以 `MaxGlobWork` 由 5 亿降到 4 亿。差分照旧 0/300,000。2.1 第 15 条、`apply-00m9.py` 的 15a、15c、15d 与第 7、8 节的数随之（3.10.1） |
| B N-2 | Minor | 接着走怎么回到游标没写，两个测试也都不限接着走的一次调用：从根重走而计数，第二次把上限花在回到游标上、`next_cursor` 不动，agent 一直循环；不计数，第 n 次的真实工作约是 n 倍的上限，两个测试都通过 | 写下跳过的规则（3.10.1"接着走"；走查写成 `page/domain` 的 `Glob.Walk`，签名在 3.10.1，用例、守卫与走查的测试都经它）：每一层排在游标那一段之前的项都跳过，不走、不算步；等于它的块进去，等于它的项跳过；只重走游标的祖先（至多 10 步，加进入这些父节点时它们的项，照算）。上限只在这一次走过一个新项之后检查，所以 `next_cursor` 总是严格往后。一次调用的界：`MaxGlobWork` 加重进的工作，再加一个父节点的项乘 256 与两个项的步；新工作之和不超过一次走完，调用次数至多约 ⌈总工作 /（上限 − 重进）⌉ + 1。测试：`page/domain` 的 `TestTheGlobResumesFromItsCursor`（每次的界、游标前进、新工作之和、游标的节点被删了）；守卫从末尾附近的游标接着走一次（三棵 0.32–0.37 秒）；3.13 的用例测试断言游标前进、循环有上限。`exp/glob` 的 `TestResumeBoundsEachCall` 在 3,000 棵随机的树上核对过这几条（3.10.1、3.10.4、3.13、3.19、6.3） |
| B N-3 | Nit | `HasSize` 加 WebP 的波及没列全：(1) `asset/domain/types_test.go:107` 断言 `"image/webp": false`；(2) Obsidian 的导入（同一条 `Blobs.Put`）与阅读视图（嵌入的图写出宽高）也随之变；(3) M12 移交第 11 项写着"WebP、SVG、AVIF 不知道宽高"；(4) `Peek(30)` 对不足 30 字节的文件答 `io.EOF`，一个合法的 GIF 可以二十几个字节；(5) MC2 的 `expectUploaded` 拿行与接口的答复比，两边都是 null 也通过 | 五处都照做：3.8 列 `types_test.go`（改为 `true`）；2.1 第 14 条与第 7 节写明导入与阅读视图（都是想要的），`apply-00m9.py` 的 14a、14d 随之；第 7 节改 M12 移交的第 11 项为"SVG、AVIF"；3.11.2 写明短读不是错误，加不足 30 字节的 GIF 的测试；MC2 断言答复的 `width === 1 && height === 1`（4.4） |
| B N-4 | Nit | `last_changed_at` 是网页的活动里页的那一部分，不是网页的活动：网页还从笔记本的行自己的 `updated_at` 起（`notebook/app/extension.go:125-152`），空的笔记本在网页上有时刻，改名挪动网页的时刻 | 照 B 的写法改字，语句不变：树的最后一次写，网页的活动里页的那一部分（2.1 第 17 条、3.17.3，`apply-00m9.py` 的 7b 与第 26 处） |
| B N-5 | Nit | `NotebookLastChange` 的计划测试点名 `changesets_notebook_id_created_at_id_idx`，而部分索引 `changesets_notebook_id_created_by_id_created_at_id_idx`（`WHERE deleted_at IS NULL`）也合用、大小相近，选哪个随数据 | 两个都收下，断言计划里没有 `Seq Scan`（3.17.3） |
| B N-6 | Nit | `Fit` 的两个数：同 A N-3 | 同 A N-3 |
| B N-7 | Nit | 缓存的正文已截到 32,768 减去最长的说明行，再以真的标题截只会更短，"留给正文的就是 32,768 减去它"做不到（至多短约 1.5 KiB） | 缓存的正文截在 32,768 之内、不为说明行预留，用时以真的标题再截（与 A N-6 的二分是同一步），那句话就成立（3.15.1、3.15.3） |
| B N-8 | Nit | 每个父节点的项在进入时就整个排好，却按走过的项计：一个 10 万项的父节点只走一项就停时排序不在上限里；接着走时游标路径上的父节点每次重排 | 进入父节点时为它的全部项各算 256（`exp/glob` 照此重量）；重进的代价写进一次调用的界（B N-2）与 M12 第 18 项。另量出：键的公共前缀长时排序比它记的单位贵，守卫树的根 20 万个项只排序就 0.09–0.23 秒，约是记下的 5,120 万个单位的 2–4 倍，仍在守卫的用时之内；一个文件夹要有约 78 万个子节点，重进才吃掉整个上限、每次只前进一项：交 M12（先滤出游标之后的项再排，或按需取出）（3.10.1、第 8 节） |

### 第二轮未采纳

或采纳的方式与审查者的建议不同：

- **A 对 A-M1 的"detail 说查询串解析不了"**：没有采纳。404 的码与文字只在 page 的 `Gate`（零值的 id 不查库），为这一种情形在 mcpserver 另写一段 404 的文字就有了两处；客户端自己的参数坏到解析不了很少见，`notebook.not_found` 也让人去看地址。
- **A N-3 的"每个候选在 2 × 48 KiB 加一项之内"**：倍增照做，这个界不照写：它要后面的项不比前面的长得多，设计写的是"至多是第一个放不下的那个，总不超过 `build(n)`"。
- **B N-1 的另一种改法"把字符的查找做成稠密的表"**：取了按字节计（主持者的选择）；另加的筛子只省掉不在模式里的字符的查找，在模式里的字符仍查映射，它们的每单位代价已在三棵守卫树上量过（0.75–2.1 ns）。
- **B N-2 的"调用次数至多 ⌈总工作 / 上限⌉ + 1"**：写为"⌈总工作 /（上限 − 重进）⌉ + 1"：B 的规则让重进照算进上限，每次停下的调用的新工作至少是上限减去重进，不是整个上限。B 的守卫树上重进是上限的 13%。
- **第一轮"未采纳"里的"按比较的字节计工作"**：第二轮改成了按键的字节计（B N-1），但单位仍是"一个字乘一个字节"：第一轮的理由（字数的因子）照旧成立，B 在复核里也这样说。

### 第二轮的新问题

- 无要负责人决定的。实施时在 CI 的机器上再量 glob 的守卫（键是 `a`、`é`、`漢` 的三棵与接着走的一次）：这次的数是在主机另有负载（load average 4–10）时量的，2 秒的界是本机最慢一次（0.82 秒）的约 2.4 倍。

## 第二轮的复核

两位审查者各自复核第二轮的改动（只读，没有 Docker 与数据库），2026-10-17：

- **A**：N-1 到 N-6 都已改正，两条未采纳的理由成立，第二轮没有带进新的问题。重跑了 `CleanClientName` 的新表格（128 个 `x` 加 `中`、129 个 `中`、140 个 `-`、140 个 emoji 等，都恰好留下 128 个字符）；`TextBytes`、`MemberBytes` 照旧与 SDK 的真实编码一致。旁及：`Fit` 在整页放得下时也先倍增，常见的情形多编码约一倍，小，不改。
- **B**：N-1 到 N-8 都已改正，两条未采纳的理由成立（调用次数的界以"上限减去重进"为准；很大的文件夹的排序交 M12），没有新的问题。在三棵守卫树（键是 `a`、`é`、`漢`）上到 4 亿的上限都在 0.24–0.63 秒停下，接着走的一次 0.26–0.33 秒；自己写的对照：按树走对逐条路径的正则 8 万例、带游标接着走 4,000 棵随机树共 41,067 次调用、两次调用之间删去游标的节点或插入新节点 3,000 棵，都没有不同、游标总往前、每次的工作在界之内；1,024 位的筛子在两个字符低 10 位相同时也对。

没有遗留的 Important 与 Minor，设计定稿。
