# M7/P1（平台：存储与流式路由）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m7-p1`（`9ac1ad8..bac9a66`：S1 `e33dacf`、S2 `09c962e`、S3 `f7ecf6a`，负对照补的测试 `bac9a66`；修复 `13f5ab9`、`bf9f54e`、`d671e17`、`02ab8c7`、`7dd7f76`、`e2ecdc9`、`cd3fa37`），对照 [01-P1-storage-stream.md](../01-P1-storage-stream.md)，[M7 总设计](../00-M7-design.md) 4.1、4.3 |
| 审查方式 | 两位审查者（Opus）并行、只读，各在 `git archive` 的快照里做实验：存储与流式路由（A：对照 Go 1.27.1 的 `net/http` 源码，真实服务器的探针，`-race`、`-cpu=1`、6 路并发各 4 次，交叉编译 linux/amd64、arm64）；配置、组合根、部署与文档（B）。修复之后由 Opus 核对，直到一轮没有行为上的发现 |
| 日期 | 2026-10-08 |
| 结论 | 审查：A 的 High 1（没有请求体的流式请求在 `read_timeout` 时被取消上下文）、Medium 2（契约不钉修改时刻；停机取消读完请求体之后的一步），B 的 Medium 2（`image-smoke` 可能卡住；总设计的遗漏），Low 与 Nit 二十余条，都已处理（不做的见各条：`flock`、区名的长度）。CI 的 Linux 上下载测试失败两次（小于回环 MSS 的套接字缓冲让吞吐塌到约 3 MB/s），在 Linux 容器里复现后改写。修复的核对五轮，每轮一位 Opus：第一轮 Low 4；第二轮 Low 1（写满时拒绝启动、残留删不掉）；第三轮 Low 2（第二轮措辞写错的契约、配额写满没有 WARN）；第四轮 Low 1（第三轮的 `FullAtOpen` 是清理之前的）；第五轮没有行为上的发现。负责人可推翻的取舍：停机不取消读完请求体之后的一步（A-M2）；不加文件锁（A-L9）；磁盘写满照常启动（c2-L1） |

## 审查的发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| A-H1 | High | **没有请求体的流式请求（下载、`HEAD`、`Content-Length: 0`），处理器的上下文在 `read_timeout` 被取消**：没有请求体时 net/http 在调用处理器之前就开始后台读（清掉读截止时间），`streaming` 的 `arm(0)` 又设上"开始 + `read_timeout`"，到时后台读超时、`cancelCtx`。复现：300 ms 的 `read_timeout`，处理器 `Sending` 之后等 900 ms，`context canceled after 301ms`；答复因写截止时间被 `Sending` 推后仍写出。P2 的下载用 `ServeContent` 不看上下文，眼下能用，但 `Bounded` 的步骤与按上下文读的存储会失败；原有测试用假 writer，没有后台读，发现不了 | 已修（`13f5ab9`）：没有请求体（`http.NoBody`）的请求从一开始就算"读完"，不设读截止时间、不包 `rateBody`；`TestAStreamWithoutABodyOutlivesTheReadTimeout`（真实服务器，处理器活过 `read_timeout`，上下文没有结束）；按阶段的表格断言没有读截止时间 |
| A-M1 | Medium | 契约没有钉住"修改时刻是真实时间"：把 `Local` 的 `ModTime` 与 `List` 的比较整体挪后 48 小时，契约全部通过；这样的实现在孤儿清扫 `List("blobs", now-24h)` 时会列出刚提交、行还没写的文件 | 已加（`13f5ab9`）：提交之后修改时刻在写入前后 2 秒之内，以一小时前为界的 `List` 不列它；两个挪后的变异都失败 |
| A-M2 | Medium | **停机时，已读完请求体、正在 `Bounded` 步骤里的上传也被取消**：默认 `shutdown_timeout` 20 秒长于 `request_timeout` 15 秒，这一步本可做完；现在每次重启都丢掉已传完的上传，P2 的取消落在 `COMMIT` 上时结果不明（行已提交而客户端看到失败，重试答 409） | 已修（`13f5ab9`）：停机只切断还在传字节的流（请求体没读到结尾，或已 `Sending`）；之间的一步照普通请求在 `shutdown_timeout` 之内做完，此时 `Sending` 答错误。`TestAShutdownCutsAStreamOnlyWhileItMovesBytes`（假 writer，五个阶段）、`TestShutdownLetsAStreamFinishTheStepAfterItsBody`（真实服务器，停机中答 200）。这是对 P1 设计 3.4 的修订，写进 P1 设计 3.4 与总设计 4.3（负责人可推翻） |
| A-L1 | Low | `Commit` 在 `rename` 之后同步分片目录失败时，文件已在键上而 `Commit` 答错误；注释"失败时文件不在"不对 | 已改注释（`13f5ab9`）：只在最后的同步失败时文件已在键上、崩溃后未必还在，调用方按孤儿处理；P1 设计 3.2 写明 |
| A-L2 | Low | 区目录在根目录里的条目从未同步（`Create` 的 `MkdirAll` 建它）；两个写入者并发建同一个分片时，后来者看到目录已在就继续，先来者可能还没同步上级 | 已修（`13f5ab9`）：`Create` 经 `ensureDir` 建区与 `.tmp/`（同步到根）；建目录加互斥，一次一个 goroutine |
| A-L3 | Low | Linux 的余量应该用 `Frsize`（`statfs` 的块数以它为单位）；FUSE 等可能报 `Bsize` ≠ `Frsize`，余量会被高估，`min_free_bytes` 失效 | 已修（`13f5ab9`）：`free_linux.go` 用 `Frsize`（为 0 时退回 `Bsize`），`free_darwin.go` 用 `Bsize` |
| A-L4 | Low | `MinRate` 很低时达到速率的客户端也被切断：截止时间每 64 KiB 才推后，要求 64 KiB / `MinRate` ≤ `read_timeout`；300 ms、64 KiB/s 时以 128 KiB/s 上传在 302 ms 断开（生产值下 `MinRate` 低于约 2.2 KB/s 才出现） | 已修（`13f5ab9`）：一步取 64 KiB，或 64 KiB 要超过半个 `read_timeout` 时取半个 `read_timeout` 按 `MinRate` 的字节数（`streamStep`），达到速率的请求体总有半个 `read_timeout` 的余地；表格测试，`TestASlowStreamAboveItsRateIsReadAndAnswered` 加 64 KiB/s 一档 |
| A-L5 | Low | `context.AfterFunc` 的 stop 不等已经开始的回调：停机时 `stop()` 可能在处理器返回之后才执行，切断 net/http 对已完成答复的最后一次写 | 已修（`13f5ab9`）：处理器返回时记下，之后的 `stop()` 不碰截止时间；表格测试的最后一项。这个竞态本身没有确定的测试（只有回调已开始、处理器同时返回才出现），去掉 `defer s.finish()` 的变异存活，接受 |
| A-L6 | Low | 停机取消上下文的断言是 net/http 自己满足的：去掉 `cancel()`，测试照样通过（截止时间一到读出错，net/http 自己 `cancelCtx`）；只靠 `cancel()` 的情形（处理器还没读请求体时停机）没有测试 | 已加（`13f5ab9`）：按阶段的表格用假 writer（没有后台读），"读请求体之前"一项只有 `cancel()` 能取消；去掉它的变异失败 |
| A-L7 | Low | 契约没有钉住"同一个键已有文件时，新文件提交之前旧的仍可见、`Abort` 不影响它"：`Create` 先删掉旧文件，契约全部通过 | 已加（`13f5ab9`）：`commitReplaces` 写新文件期间与 `Abort` 之后读出旧的 |
| A-L8 | Low | 启动检查只探根目录：根可写而 `blobs/` 的属主不对（以 root 恢复了备份）时启动通过，运行时每次上传都 500；`dropTemporaries` 的错误没有 uid、gid | 已修（`13f5ab9`）：逐个已有的区同样探测，错误写明那个目录与 uid、gid；测试加"一个区"一项 |
| A-L9 | Low | "单进程"只写在文档里，没有强制；建议对 `<dir>/.lock` 加 `flock` | 不做（同 B-Q2）：锁会让先起新、后停旧的重启起不来；审查者核实了误用时是失败即安全的（后起的删掉 `.tmp/`，前一个的 `Commit` 答 `ENOENT`，不写坏）。README 写明升级先停旧的（Kubernetes 的 `Recreate`） |
| A-L10 | Low | `EDQUOT` 没有映射为 `ErrFull`：带配额的卷写满时答 500 而不是 507 | 已修（`13f5ab9`）：`noSpace` 认 `ENOSPC` 与 `EDQUOT`；测试注入 `EDQUOT` |
| A-Nit | Nit | P1 设计的文件表（`free_*.go`、`limit.go`）与实现不一致；3.4 说进入处理器时的读截止时间"与服务器的相同"，实际从处理器开始算；`Sending` 可能缩短写截止时间，应写成"重设"；`streamAPI` 只是转调；`Writer` 没写不可并发；`List` 第二级分片不见了报错而第一级算"没有"；根下中途崩溃留下的 `.probe-*` 不清；路由桶的拒绝记 debug，而 13.1 第 20 条只说模块的桶记 info；`CheckArea` 没有长度上限 | 已改（`13f5ab9`）：文档的文件表与 3.4；`Sending` 的注释与文档写"设为"；`streamAPI` 去掉；`Writer` 的注释；第二级分片不见了跳过；启动时删掉根下的探测文件；13.1 第 20 条写明流式路由的桶同平台的记 debug。区的长度不设上限：区是代码里的常量 |
| A-Q1 | 疑问 | Linux 上"经 `Sending` 的答复"一测可能不稳：读速取决于每次 `Read` 拿到多少 | 确实不稳：CI 的 Linux 上先后失败两次（`bac9a66` 与按字节限速的 `13f5ab9`：8 秒读到 1.3 MB）。在 Linux 容器（限 2 核、`-race`）里量过：发送或接收缓冲小于回环的 64 KiB 段时，每次发送都等内核的计时器，吞吐约 3 MB/s。已改（`bf9f54e`）：服务端的发送缓冲 256 KiB、客户端用系统的；客户端停读 1.5 秒（过写超时）再读完 16 MiB（停读时缓冲装下约 650 KB）。macOS 与 Linux 容器各跑多次通过 |
| A-Q2 | 疑问（给 P2） | multipart 用 chunked 传输时，处理器在结束边界之后没有把请求体读到结尾，后台读不开始，读截止时间不解除 | 写进总设计 4.4：最后一部分之后 `io.Copy(io.Discard, r.Body)`；导入同此 |
| A-Q3 | 疑问（给 P2） | 每条连接都可以占满 `read_timeout + MaxBytes/MinRate`，能开多少条只由桶限制 | 留给 P2 定：考虑按凭证限制同时的上传 |
| B-M1 | Medium | `image-smoke` 的拒绝启动一步以前台运行：拒绝不发生时（回归）脚本卡住 | 已修（`13f5ab9`）：后台运行，等它退出（有上限），核对退出码与日志 |
| B-M2 | Medium | 总设计第 7 节漏了 P2 的交付（asset 的配置、`ratelimit.asset_content`）与验证（契约测试认 `x-raw`），修订表漏了 P1 的配置与 P2 的 13.1 第 15 条 | 已改（`13f5ab9`） |
| B-L1 | Low | `LogValue` 的 storage 两项没有测试 | 已加（`13f5ab9`） |
| B-L2 | Low | archtest 的命令行组合不含迁移的三个命令 | 已加（`13f5ab9`）：`MigrateUp`、`MigrateDown`、`MigrateStatus` 到达 `postgres.NewMigrator`，不打开存储 |
| B-L3、B-N5 | Low | P1 设计的 e2e 写的是 `mkdtemp`、停下后删除，实际是日志旁的目录、启动前清空；文件表多了 `limit.go` | 已改（`13f5ab9`） |
| B-L4 | Low | 总体设计 8.5、11 仍写 `tmp/` | 已改（`13f5ab9`）：临时文件在各区的 `.tmp/`，启动时删掉 |
| B-L5、B-Q3 | Low | 总设计的签名与措辞 | 已改（`13f5ab9`） |
| B-L6 | Low | 非 darwin、linux 的系统编不过（`freeBytes` 未定义） | 已修（`13f5ab9`）：`free_other.go` 答"不支持"；交叉编译 windows、freebsd 通过 |
| B-L7 | Low | README 没写镜像的环境变量 `NWIKI_STORAGE__DIR` 优先于配置文件的 `storage.dir` | 已改（`13f5ab9`） |
| B-L8 | Low | `image-smoke` 不核对镜像的卷与环境变量 | 已加（`13f5ab9`）：`docker inspect` 核对 `/data` 卷与 `NWIKI_STORAGE__DIR=/data` |
| B-N1、N2、N3、N7 | Nit | `Makefile` 的说明；`composesMore` 的注释；`config.yaml` 的超时注释不提流式路由；CI 上传测试结果时带了存储目录 | 已改（`13f5ab9`） |
| B-N4 | Nit | 启动日志写的是配置里的相对路径；余量已低于下限时不提示 | 已改（`13f5ab9`）：记绝对路径，低于下限时 WARN |
| B-N6 | Nit | README：宿主机目录的例子、Kubernetes 的 `fsGroup`、恢复的成对、nginx 的超时是两次读写之间的间隔 | 已改（`13f5ab9`） |
| B-N8 | Nit | dev 的目录 `server/data/` 在 `./...` 之下，文件多了拖慢 go 的模式匹配 | 已改（`13f5ab9`）：dev 用 `_data`（下划线开头，go 不进去），两个忽略文件都加 |
| B-N9 | Nit | 13.1 第 15 条的存储一句插在前后端的量之间 | 已挪到末尾（`13f5ab9`） |
| B-Q1 | 疑问 | 更新记录的行何时写 | 收尾时写 |
| B-Q2 | 疑问 | 要不要加 `flock` | 不加，见 A-L9 |

## 修复的核对

### 第一轮（`13f5ab9`）

一位 Opus：对照 Go 1.27.1 的 `net/http` 逐条推演状态机，真实服务器上做三个实验（无请求体、已 `Sending`、写到一半停机；chunked 读到结尾后停机；处理器从不读请求体），`-race -count=5` 与 36 个满载进程下的 `-cpu 2`，交叉编译十余个系统，变异。没有 Critical、High、Medium。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| c1-L1 | Low（测试） | `finish` 的接线没有测试：去掉 `defer s.finish()` 全部通过 | 接受（同 A-L5）：要钉住只能给回调加测试钩子 |
| c1-L2 | Low（接口） | 停机的错误没有导出：P2 的下载分不清"停机"与接线的错误，不知道答 503 还是 500 | 已改（`d671e17`）：导出 `httpserver.ErrShuttingDown`；总设计 4.5 写明停机后的下载答 503（码由 P2 定） |
| c1-L3 | Low（测试） | `TestShutdownLetsAStreamFinishTheStepAfterItsBody` 要求停机在 300 ms 内到达流；请求体读失败时主协程卡在 `<-read` 到 10 分钟超时；`TestShutdownEndsAStreamAtOnce` 的 `<-reading` 同样 | 已改（`d671e17`）：处理器轮询流已知停机（`awaitStopped`，至多 2 秒），主协程 `select` 读的结果、限时 5 秒 |
| c1-L4 | Low（文档） | 总体设计 13.1 第 14 条的 `API.Stream` 仍是旧写法 | 已改（`d671e17`）：一步、无请求体、按阶段停机、`ErrShuttingDown` |
| c1-N5 | Nit | 在区里探测时被杀，留下的 `<区>/.probe-*` 永远不删 | 已改（`d671e17`）：在区的 `.tmp/` 里探测，随后整个删掉 |
| c1-N6 | Nit | 建目录的锁跨过 `fsync`：上线初期大多数提交都新建分片目录，会被串行 | 记下，v0.1 接受（上传不是热路径） |
| c1-N7 | Nit | "没有请求体"靠 `r.Body == http.NoBody`，中间件包了请求体就认不得 | 已改（`d671e17`）：`r.ContentLength == 0`（服务端二者等价） |
| c1-N8 | Nit | `Sending` 之后的步骤在停机时也被切断 | 写进 `Sending` 的注释与 P1 设计 3.4 |
| c1-N9 | Nit | "失败时文件可能已在键上"只写在实现上，端口没写 | 已挪到 `Writer.Commit` 的注释 |
| c1-N10 | Nit | README "子目录"、总设计 4.1 的启动检查与 4.3 的写截止时间是旧措辞；测试注释"无请求体从不设读截止时间"不准（停机时设） | 已改（`d671e17`） |
| c1-N11 | Nit | `image-smoke` 拒绝启动不发生时，失败信息里没有那个容器的日志 | 已加（`d671e17`） |
| c1-N12 | Nit | 启动时余量低于下限的 WARN、其他系统上 `Free` 出错拒绝启动，没有测试 | 不改：前者只是日志，后者与"只支持 Linux 与 macOS"一致 |

### 第二轮（`bf9f54e`、`d671e17`）

一位 Opus：对照 `net/http`（含 `internal/http2`）核对"没有请求体"的两种判断，实测停读时内核缓冲（macOS 停读 1.5–8 秒都是约 640 KB），18 核机器上 30 个 2 核、54 个 1 核的进程并行跑真实服务器的测试，`dropTemporaries` 的七种情形，五个变异。没有 Critical、High、Medium；下载测试的两半在 Linux 与 macOS 上都稳（推理与实测；Linux 由 `d671e17` 的 CI 证实）。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| c2-L1 | Low（行为） | **先探测、后清理**：卷写满（或配额用尽）时探测的写失败，`serve` 拒绝启动，只读的页面也打不开；写满了盘的往往正是崩溃留下的半截文件，探测在清理之前，它们永远删不掉 | 已修（`02ab8c7`）：各区先删 `.tmp/` 再探测；探测时 `ENOSPC`、`EDQUOT` 不算不可写（写入答 `ErrFull`，启动的 WARN 已有）。`TestAFullDiskOpensAndDropsWhatWasLeft`（注入写满的文件） |
| c2-N1 | Nit（行为） | 区里的 `.tmp` 是文件时拒绝启动（d671e17 引入：探测的 `MkdirAll` 答 `ENOTDIR`） | 已修（`02ab8c7`）：先删后探，同上；测试加这一形状 |
| c2-N2 | Nit（测试） | `r.ContentLength == 0` 没有测试钉住：改回 `http.NoBody` 全部通过 | 已加（`02ab8c7`）：表格加"请求体被包了一层"的一项 |
| c2-N3 | Nit | `ErrShuttingDown` 的注释只说是 `Sending` 的；被切断的请求体读也答它，P2 的上传不该记成 ERROR | 已改（`02ab8c7`）：注释与 P1 设计 3.4 |
| c2-N4、N5 | Nit | 13.1 第 14 条："此时 `Sending` 答"应是"停机之后一律答"，"不设读截止时间"应是"从一开始不设"；测试注释没写完 | 已改（`02ab8c7`） |
| c2-N6 | Nit | `smallBuffers` 名不副实（256 KiB，Linux 翻倍并受 `wmem_max` 限） | 改名 `boundedSendBuffer`，注释写"不到 1 MB"（`02ab8c7`） |
| c2-N7 | Nit | 旧布局（`d671e17` 之前）在区里留下的 `.probe-*` 不再删 | 不改：只在跑过旧分支的开发机上，`List` 不看它 |
| c2-N8 | Nit（行为） | 作为符号链接的区既不探测也不清理（`DirEntry.IsDir()` 对链接为假） | 已修（`02ab8c7`）：按 `os.Stat` 判断；测试加指向别处的区 |
| c2-N9 | Nit | `image-smoke` 在 `pipefail` 下 `docker logs \| grep -q`，`grep` 先退出时可能误报 | 已改（`02ab8c7`）：`grep -q … <<<"$(docker logs …)"` |

### 第三轮（`02ab8c7`）

一位 Opus：反向对照六处，注入 `ENOSPC`、`EROFS` 与不可写的各种形状，五轮实测被切断的请求体读答什么。没有 Critical、High、Medium。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| c3-L1 | Low（契约） | **被停机切断的请求体读并不答 `ErrShuttingDown`**（第二轮的措辞修改写进了导出的注释、P1 设计与 13.1 第 14 条）：`stop()` 只让截止时间到期，读答的是 `i/o timeout`，只有恰好跨过一步、走进 `arm` 时才答它（不加 `-race` 10/10 次不是）；测试只断言了"有错" | 已修（`7dd7f76`）：`rateBody.Read` 在停机之后的读错误包上 `ErrShuttingDown`（原错误仍在）；`TestShutdownEndsAStreamAtOnce` 断言 `errors.Is(err, ErrShuttingDown)` |
| c3-L2 | Low（可观测） | "磁盘写满时日志有 WARN"不总成立：配额（`EDQUOT`）、inode 用尽、btrfs 元数据满时 `statfs` 的余量并不低，`min_free_bytes: 0` 也合法，启动时没有 WARN，此后每次写都 507 | 已修（`7dd7f76`）：`Local.FullAtOpen()` 交出启动探测遇到的写满错误，组合根在它或"余量低于下限"时记 WARN `storage is full` |
| c3-N1 | Nit（行为） | 区的 `os.Stat` 出错一律跳过：指向不存在之处的链接照样启动，之后写入答 500；而同样不可用的真目录拒绝启动 | 已改（`7dd7f76`）：出错时拒绝启动、写明那个区；只有"存在而不是目录"才跳过；`TestOpeningAnAreaLinkedToNothingFails` |
| c3-N2 | Nit | `errors.Join` 里有一个 `ENOSPC` 就整体放行（如写 `ENOSPC` 加 `Sync` 的 `EIO`） | 不改：建文件成功说明有写权限，`EACCES` 不会与之同现；已满且不可写的目录在 `MkdirAll`、`CreateTemp` 就答 `EACCES` |
| c3-N3 | Nit | 根目录不存在、`MkdirAll` 答 `ENOSPC` 时探测放行，随后 `ReadDir` 的错误没有 uid | 已改（`7dd7f76`）：`ReadDir` 的错误也写明目录与 uid |
| c3 措辞 | Nit | P1 设计 3.3 的"这些"指代不清；README 的括号读来像只有盘满时才删残留；测试注释 "until" 应为 "unless"；`Create`、`Write` 的错误里有两次 `storage:` 前缀 | 已改（`7dd7f76`）：措辞；错误经 `failed` 只带一次前缀（测试数前缀） |

### 第四轮（`7dd7f76`）

一位 Opus：停机的三个测试 `-race -count=15`、存储 `-race -count=50` 没有抖动；五个变异；注入"残留在时写满、删掉之后有余量"的形状。没有 Critical、High、Medium。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| c4-L1 | Low（日志） | **`FullAtOpen` 记的是清理之前的写满**：根的探测在删残留之前，区按名字的次序逐个"删、探"，前面的区探测时后面的还没删；设计点名的场景（半截的导入包写满了盘）里，删掉之后写入照常成功，启动却记 `storage is full`，同一行的 `free_bytes` 是几 GiB（第三轮的改动引入） | 已修（`e2ecdc9`）：先建根、删掉全部残留，再探测根与各区；`TestFullAtOpenIsAfterTheDeletions` |
| c4-N1 | Nit（测试） | "只在停机时才包上 `ErrShuttingDown`"没有测试：改成每个读错误都包，全部通过 | 已加（`e2ecdc9`）：太慢、停住、超过上限的读断言不是它 |
| c4-N2 | Nit（测试） | 区一级的写满记录没有钉住（测试里根也满） | 已加（`e2ecdc9`）：`TestFullAtOpenFindsAnAreaFullOnItsOwn`（只有区的 `.tmp` 答 `EDQUOT`） |
| c4-N3 | Nit | 没有写满错误时 WARN 也记 `error=<nil>`，与全仓库的写法不一 | 已改（`e2ecdc9`）：有错误时才记 |
| c4-N4、N5 | Nit | P1 设计 3.5 与第 5 节没跟上；3.3 写"指向不存在之处"，代码是任何 `Stat` 的错误 | 已改（`e2ecdc9`） |
| c4-N6 | Nit | README 写"答 507"，507 从 P2 才有 | 不改：README 的附件目录一节整体是 M7 的，v0.1 在 M7 收尾之前不发布 |
| c4-N7 | 给 P2 | 包上之后的错误同时满足 `ErrShuttingDown`、`*http.MaxBytesError`、超时，P2 先判断 `ErrShuttingDown`；`APIErrors.Write` 没有它的分支（会记 ERROR、答 500），上传不要原样交给它；经 `arm` 切断时答的是裸的 `ErrShuttingDown` | 写进 P2 的阶段文档 |

### 第五轮（`e2ecdc9`）

一位 Opus：新旧两版 `local.go` 在 14 种失败形状下逐一比对错误文本与留下的文件（每种都写明目录与 uid、gid，没有退化；根目录不存在又写满时，错误比原来准），七个变异。**没有行为上的发现**，核对到此为止。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| c5-L1 | Low（测试） | `TestFullAtOpenIsAfterTheDeletions` 只有一个区，钉不住"所有区删完才探测任何一个"：保留逐区"删、探、删"、只把根的探测挪后，照样通过 | 已加（`cd3fa37`）：多一个排在前面的空区 `blobs`；"逐区删、探交错"的变异失败 |
| c5-N1 | Nit（测试） | 写满的假文件 `Sync` 总答 `ENOSPC`，"只有区答 `EDQUOT`"的测试其实是两者的合并 | 已改（`cd3fa37`）：`Sync` 也答它自己的 `errno`；"不认 `EDQUOT`"的变异让它失败 |
| c5-N2、N3 | Nit | 几种问题同时存在（或只读挂载）时先报区的错误；根不可写而区的 `.tmp` 可写时，先删残留再拒绝启动 | 不改：信息不丢；单实例下无害，命令行不打开存储 |
| c5-N4、N5 | Nit | P1 设计的反向对照清单没有列新次序；字段注释"记下的错误"未说是第一个；"these"指代不清；3.3 没说删不掉时同样答错误 | 已改（`cd3fa37`） |

## 负对照

`scratchpad` 的 `mut-p1.json`，最终 55 个（存储 26、流式路由与 `API` 26、配置 2、组合根 1），每个都有测试失败。另有一个已知存活、接受的：去掉 `defer s.finish()`（A-L5、c1-L1，竞态没有确定的测试）。修复过程中补的测试抓到的有：没有请求体时也设读截止时间、停机总是切断、停机不取消、`Sending` 不记下、一步总是 64 KiB、修改时刻挪后、`Create` 先删旧文件、不探测区、留下探测文件、`EDQUOT` 不算写满、先探后删、逐区删探交错、写满不放行或不记下、断链的区被跳过、被切断的读不包 `ErrShuttingDown`、每个读错误都包、错误带两次前缀、"没有请求体"只认 `http.NoBody`。
