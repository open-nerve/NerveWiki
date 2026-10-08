# M7/P2（附件的服务端）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m7-p2`（`613a7fa..53facf6`：pgtest 的修正 `613a7fa`，S1 `7d44789`、S2 `d64fa80`、S3 `3720da9`、S4 `943f410`、S5 `87f8621`、S6 `b8415f6`、S7 `01603e6`，负对照的修补 `53facf6`；审查的修复 `9b3fc00`、`fd371d5`、`ad071d0`、`39d0d44`；修复核对的修复 `dda31f0`、`8fe29c4`、`e804286`、`302df06`、`a838cd0`、`10318f2`、`c4b364e`、`550c8dd`、`a2a5658`、`1e2090d`），对照 [02-P2-assets-server.md](../02-P2-assets-server.md)，[M7 总设计](../00-M7-design.md) 4.1–4.8 |
| 审查方式 | 三位审查者（Opus）并行、只读，各在 `git archive` 的快照里做实验：服务端的正确性与并发（A）；接口的边缘与文件的提供（B：multipart 的解析、签名与下载、响应头、平台的流式路由）；测试、前端、e2e、部署与文档（C）。修复之后由 Opus 核对，直到一轮没有行为上的发现 |
| 日期 | 2026-10-08 |
| 结论 | 审查：B 的 Medium 1（不是合法 UTF-8 的文件名绕过名称校验，500 并留下文件）、C 的 Medium 1（树的重读最多晚 5 秒），Medium-low 7（A1、A2、B2、B3、C2、C3、C4），Low 与文档、风格三十余条，都已处理；接受的见各条（B10、C9、缺结尾 `--`）。修复的核对九轮：第一到三轮核对修复本身（Low 与测试缺口）；第四到八轮用变异扫描把 P2 的服务端代码逐层扫过——`content.go`、`upload.go`、`handler.go` 与 `server.go`、asset 的 `app` 与 `domain` 与各适配器、page 为附件做的改动、平台 `API.Stream` 与契约规则的改动——找到的都是测试抓不住的改变行为的回退，没有代码缺陷（第五轮的一处契约不符除外：文件之后的部分的头超过路由上限时答 413），每一处都补了测试；第九轮没有行为问题。负对照约 270 个，都被抓住。负责人可推翻的取舍：`storage.min_free_bytes` 对同时的上传是软的（B10）；停读的媒体元素在 `read_timeout` 加缓冲的字节应得的时间之后断开，靠浏览器的 `Range` 重新请求（B2）；`SweepTimeout` 50 分钟；缺结尾 `--` 的请求体被接受；`parent_id` 认 `uuid.Parse` 的写法（大写、没有连字符），不认括号与 URN；`Sending` 调两次不拒绝 |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| B1 | 中 | 文件名不是合法 UTF-8 时（`filename*` 可以这样写）绕过名称校验：单元写节点时数据库拒绝，答 500，留下没有行的文件 | 已修（`9b3fc00`）：`shared.CheckTitle` 先查 UTF-8（JSON 的字符串解码后必是 UTF-8，表单与 zip 的文件名不必）；整个程序的 `TestAnUploadNamedOutsideUTF8IsRefused`（422，不留文件），`shared` 的表格两例 |
| C1 | 中 | 树的重读经 refresher 之后，别人连续改树最多晚 5 秒才显示（节流不是防抖） | 已修（`ad071d0`、`dda31f0`、`e804286`）：refresher 按键的间隔，树 500 毫秒（`TREE_INTERVAL_MS`）；测试钉住可见时、隐藏时请求、回到前台还在间隔之内三种情形 |
| A1 | 中低 | 读元数据、列表时并发删除，误报 ERROR "attachment node without its row"（两次读不是同一快照） | 已修（`9b3fc00`）：读不到行时再读节点，还活着才记 ERROR；测试"读后被删"一例 |
| A2 | 中低 | 清扫跑在 River 默认 1 分钟的 `JobTimeout` 下，大的存储扫不到尾；一个删不掉的文件让整轮停下 | 已修（`9b3fc00`、`8fe29c4`）：`SweepWorker.Timeout` 50 分钟（低于 River 默认 1 小时的 `RescueStuckJobsAfter`，复核 1 指出原定的 1 小时与它相等）；删不掉的记 WARN（带 `blob_id`）、接着删其余、最后答错误；测试 |
| B2 | 中低 | 下载的写截止时间由 `Sending(n)` 一次设为 `read_timeout + n / MinRate`：停读的客户端占住连接到那时 | 已修（`fd371d5`，平台）：`Sending(r)` 不再带字节数；答复经 `streamWriter` 按步写出，每步之前把写截止时间设为"宣告时刻 + `read_timeout` + 已写字节 / `MinRate`"；停读的客户端在 `read_timeout` 加缓冲住的字节应得的时间之后断开。`TestAStreamMovesItsDeadlinesWithItsBytes`（假 writer，每步一秒）、`TestAnAnswerAnnouncedWithSendingLeavesAtItsRate`（真实服务器：4 倍速率读完 8 MiB、停读 3 秒被切断、不宣告的被写超时切断）；停机之后的下一步答 `ErrShuttingDown`、不再推后（`8fe29c4`） |
| B3 | 中低 | `If-Match`、`If-Unmodified-Since` 让 `ServeContent` 答 412，带着文件的类型与缓存头，不合契约 | 已修（`9b3fc00`）：下发之前去掉这两个头（地址所下发的内容从不改变，没有要守的）；测试；00-M7 4.5 改写 |
| C2 | 中低 | 严格读法表格的"b escaped"一例拼错（`%3` 接上原值去掉第一个字符，不是同一个 b），"不反转义"没被钉住 | 已修（`9b3fc00`）：照原值的第一个字符转义，并断言反转义之后等于原值 |
| C3 | 中低 | 活动的"最后上传"与上传单元的变更集同一时刻：冗余，也测不出 | 已改（`9b3fc00`）：附件只报字节数；注释与 00-M7 4.6 改正 |
| C4 | 中低 | AS5 页面版本的"不向外站请求"只靠被拦下的脚本证明 | 已修（`ad071d0`）：SVG 加外站的图片与 `@import`，断言每个外站请求都被 CSP 拦下（Chromium 报为失败的请求，原因 `csp`）、没有答复；控制台的拒绝按条声明（新旧两种措辞）。负对照：`img-src` 放开 `https:` 时故事失败 |
| A3、B7 | 低 | 类型按原始名称测定，不是规范化之后的（`"x.svg "` 测成 octet） | 已修（`9b3fc00`）：`Store` 先经 `CheckTitle` 规范化；`TestStoreTellsTheTypeByTheNameAsKept` |
| A4、B4 | 低 | 前置的 4 KiB 截断在头部行中间时误报"没读完整"；日志带着解析器引用的原始头部文字 | 已修（`9b3fc00`）：读错误按前置部分是否耗尽判断；日志只记原因的分类（`readCause`：太慢、提前结束、连接失败、格式不对）；测试：截断在行中间答前置的 400、4 KiB 之内的畸形头记 `cause=malformed` 而不记原文、`readCause` 的分类（`8fe29c4`、`e804286`） |
| A5、B8 | 低 | `bounded` 里的超时以原请求写出，记成 ERROR 而不是 WARN | 已修（`9b3fc00`）：`bounded` 交回派生的请求，错误照它写；测试下载、上传的预检与单元三处（`8fe29c4`） |
| A6 | 低 | `Abort` 失败被吞 | 已修（`9b3fc00`）：记 WARN 带 `blob_id`；测试 |
| A7 | 低 | 文件之后的部分要先读完才答 400 | 已修（`9b3fc00`）：读到它的头就答；测试用读不完的部分（`8fe29c4`） |
| A8、B11 | 低 | 下载路径的 id 有多种写法（大写、无连字符、带括号）都能用；查询的次序不限 | 已修（`9b3fc00`、`8fe29c4`、`dda31f0`、`302df06`）：路径的 id 与查询都照服务端的写法读（路径转义过的答 404，参数依次、键名对位、至多四段）；严格读法的表格逐例 |
| A9 | 需核实 | `AssetsUnder` 的注释说兄弟的索引服务它，`IS NOT DISTINCT FROM` 不能作索引条件 | 已改注释（`9b3fc00`） |
| B5 | 低 | 404、503、429、400 不带 CSP（契约说每个答复都带） | 已修（`9b3fc00`）：内容路由的最外层设 CSP 与 CORP；表格逐码核对 |
| B6 | 低 | 空的 `name` 部分答 422 | 已修（`9b3fc00`）：空或空白取文件名；测试 |
| B9 | 低 | `API.Stream` 自己答的 401、429 不带 `Connection: close`：请求体没读，net/http 先再读至多 256 KiB | 已修（`fd371d5`，平台）：处理器之前的答复对有请求体的请求关闭连接，到了处理器再去掉；测试与无请求体的一例 |
| B10 | 低 | `storage.min_free_bytes` 对并发的上传是软的 | 接受（总设计 4.1）：预检只看当时的余量 |
| B12 | 低 | multipart 解码 quoted-printable，改写了文件的字节 | 已修（`9b3fc00`）：`NextRawPart`；测试 |
| B13 | 低 | 一次操作读两次时钟（下载的核对与剩余时间；列表每项各签一次） | 已修（`9b3fc00`）：每次操作读一次（13.1 第 9 条）；签名器挪进 `adapter/mac`，按调用方给的时刻签 |
| C5 | 低 | `domain.Inline` 是 `mime != Octet`，表外的类型也内联 | 已修（`9b3fc00`）：`Served`、`Inline` 按表判断；测试 |
| C6 | 低 | README 说没发完文件的客户端也收得到预检的答复 | 已改（`ad071d0`）：可能只看到连接被重置 |
| C7 | 低 | 交错的"失去写权限"实为移出工作区 | 已加（`ad071d0`）：降为只读的两种次序（403；先上传的照样读得到） |
| C8 | 低 | `checkAssets` 不查没有行的文件 | 已加（`ad071d0`）：只有清扫的测试种下这种文件 |
| C9 | 低 | 依赖时间的等待 | 接受：CI 再出现时先查 |
| C10 | 低 | image-smoke 重启之后用新签的地址下载；登录失败时信息不明；变量名 `read` | 已改（`ad071d0`、`dda31f0`）：重启之前签的地址下载（密钥由 JWT 的私钥导出，不随重启变），核对确切的大小，登录拿不到令牌即失败 |
| C11 | 低 | 第 7 节要补的出入 | 写进第 7 节 |
| B 小 | 低 | 缺结尾 `--` 的请求体被接受（文件完整） | 接受，记下 |
| A、B、C 文档 | — | 契约的 `rate_limited` 写在操作的码里（13.1 第 20 条）；契约没列安全头；签名密钥进了 app 层（13.1 第 25 条）；停机与超时的处理三处对齐；PDF 的 Firefox 未实测、README 的"提示下载"未证实；13.1 第 6、14、15 条与 13.4 第 4 条；几处注释与超长的行 | 已改（`9b3fc00`、`fd371d5`、`ad071d0`）：契约只在顶层写 `rate_limited`，列出 CSP、`Cache-Control`、CORP；签名器挪进 `adapter/mac`；README 写"其他浏览器未实测，显示不了时用 `download_url`" |
| A、B 风格 | — | `Purge.Batch` 空批也 `DELETE`；`byExtension` 每次重建 | 前者已改（`9b3fc00`，测试 `8fe29c4`）；后者保留（lint 不许包级变量） |

## 修复的核对

每轮的快照是上一轮修复之后的提交，审查者在 `git archive` 的快照里只读地实验，不起数据库；需要数据库的改动由分支的 CI 证明。

| 轮 | 快照 | 发现 | 修复 |
|---|---|---|---|
| 1 | `ad071d0`（两位：服务端与平台 A，测试、前端与文档 C） | 没有中高的行为问题。A：206 的 `Content-Range` 必填与多段 Range 冲突（修复引入）；转义过的路径 id 答 200；`SweepTimeout` 等于 River 的 `RescueStuckJobsAfter`；`armSent` 停机的检查没有测试；几处修复的变异存活。C：00-M7 4.8 的树间隔；键名没被钉住（值按位置取）；refresher 隐藏时的间隔没测；image-smoke 改后没跑 | `dda31f0`（C）、`8fe29c4`（A）：206 不再要求 `Content-Range`（多段答 `multipart/byteranges`）；`RawPath` 不空答 404；`SweepTimeout` 50 分钟；停机之后的下一步答 `ErrShuttingDown`；键名的两例；image-smoke 核对大小 |
| 2 | `8fe29c4` | refresher 间隔的边界没钉住；读错误的 `io.EOF` 一例；`addressKeys` 的界；注释；00-M7 的严格读法 | `e804286` |
| 3 | `e804286` | `d` 两次、`d` 之后还有成员两例；00-M7 的措辞；event-stream 的折行 | `302df06` |
| 4 | `302df06`（对 `content.go` 做变异扫描） | 测试放过的改变行为的变异：下载签名之下 `d` 不为 1、显示签名之下五段、长于一个字母的键；停机的 503 带文件的头；`Last-Modified` 与 `If-Modified-Since` 的 304；打开的文件没关；2038 之后的地址；签名的拼写 | `a838cd0`：逐项补测试（假存储数打开的文件） |
| 5 | `a838cd0`（对 `upload.go` 做 140 个变异） | 42 个存活且改变行为，最重的是去掉"文件一开始放开 4 KiB 的限额"（大于 4 KiB 的上传全部失败）没有测试发现；契约不符：文件之后的部分的头超过路由的上限时答 413，detail 写文件的上限 | `10318f2`：`upload_body_test.go`；`end()` 把 `NextRawPart` 的 `MaxBytesError` 答 400 "a part after the file"，`readFailed` 去掉不再可达的分支；整个程序上传、下载 3 MiB 与 Range |
| 6 | `10318f2`（对 `handler.go`、`server.go` 做变异） | 本轮的修复没有行为问题。15 个存活且改变行为：生成路由的错误处理接线、平台的中间件与桶、流路由的策略、`bindID` 的错误类型、列表的拼装；`Envelope` 改小两个；`end()` 对多于 10000 行的头（`multipart.ErrMessageTooLarge`）答"not read whole"（低） | `c4b364e`：`handler_test.go`（参数绑定、期限、平台的桶、策略表），`uploadPolicy`/`downloadPolicy`，`httpservertest.APIOptions` 可传桶；列表一页两条；`Envelope` 从下方钉住；`end()` 加 `ErrMessageTooLarge` |
| 7 | `c4b364e`（两位：A 核对第六轮并对 `domain`、`adapter/sniff|files|mac` 做变异，B 对 `app` 做变异） | 第六轮的修复没有行为问题，它的存活变异都被抓住。新扫出的都是测试缺口，没有代码缺陷：A 21 个（sniff 的测试导入编码器，掩盖了解码器的注册；1 MiB 的上限与整段 head；`files` 适配器九处；`MaxSide` 用符号；最后一个点；过期时间的 UTC）；B 48 个（端口的失败被吞或换成 404、空页；`Commit` 失败；一边为 0 的尺寸；读尺寸的文件没关；单元与清理的事务的 ctx；清扫的分批、遍历、ctx 与第一个失败；清理的删除与提交；`Free` 失败；一次操作读一次时钟；短读的 head；日志的级别与键；没有 actor；页面不判定） | `550c8dd`：假实现加失败、计数与 ctx 的标记，测试并进现有的表；sniff 的样图写成字面量；avif、flac 认自己的嗅探类型；`ExpiredBlobs` 在事务之外拒绝（同其他模块的持锁读） |
| 8 | `550c8dd`（两位：A 核对第七轮并对 page 为附件做的改动做变异，B 对平台 httpserver 的 P2 改动做变异） | 第七轮的修复没有让别的测试变弱，`ExpiredBlobs` 的路径都在事务里；没有代码缺陷。测试缺口：A 18 个（asset 的假实现失败时仍答"找到/有空间"，抓不住次序的回退；附件名的长度守卫、单字符的基名与扩展名、改名答修整后的名字；附件节点的字段与变更集的客户端；最后一个兄弟之后的重编号；读父节点、兄弟失败）；B 21 个（`closingEarly` 只在分块的请求体上测过——浏览器的 FormData 带 `Content-Length`；`Sending` 的起点；未宣告的写；低速率的步；`Write` 的字节数；失败的步；`Unwrap`；接线故障的 500；处理器拿到的 `Connection`；契约规则经组件绕过 `x-raw`、可为 null、开放的 parts、没有 schema、答复的头、引用的答复；`httpservertest` 的匿名桶） | `a2a5658`：测试并进现有的表；`streamWriter.armed` 冗余，删掉 |
| 9 | `a2a5658`（一位，只核对第八轮） | 没有行为问题：第八轮的修复没有带来问题，它的 39 个存活变异按新代码都被抓住，删去 `armed` 不改变行为（新旧代码上的探针逐字相同），依赖时间的测试在 72 个满载进程下 p99 2.2 ms、负对照 300 ms。低优先级的测试缺口：第二步失败、两步之间停机时 `Write` 报告的字节数（唯一的调用方 `ServeContent` 丢弃它，HTTP 上观察不到）；风格五条 | `1e2090d`：失败的步改为表格（第一步、第二步、两步之间停机）；风格四条照改，`TestSendingCountsFromItsAnnouncement` 不设 `writeSlack` 保留（`TestAStreamMovesItsDeadlinesWithItsBytes` 钉住它） |

## 负对照

实施时（`p2-mutants.json`）：服务端 20 个、前端 4 个、浏览器 1 个（CSP 去掉 `sandbox`），都被抓住；初跑存活的两个（去掉清理器的 `SKIP LOCKED` 让测试挂起、去掉节点删除的 `deleted_at IS NULL`）补了测试。

修复与核对（`p2-fix-mutants1`–`21`）：约 245 个，每轮的修复都用审查者列出的存活变异复跑，全部被抓住。其中值得记下的：`sign-at-wall-clock` 第一次存活只因运行的时刻与测试夹具同在一个小时（换个时刻重跑被抓）；`order-loose` 起初误判为等价（键名没被钉住）；去掉"文件一开始放开 4 KiB 的限额"会让大于 4 KiB 的上传全部失败，第五轮之前没有测试发现；sniff 的测试为了编码样图导入了编码器，掩盖了解码器的注册（样图改为字面量）；假实现在失败时仍答"找到了"，掩盖了"先判结果再判错误"的次序（假实现改为照真实端口答零值）。已知存活、判为等价的另列在各轮的实验目录里（如 `hmac.Equal` 换成 `==` 只差计时）。
