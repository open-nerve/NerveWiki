# M7/P1 平台：存储与流式路由：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M7/P1 平台：存储与流式路由 |
| 状态 | 已完成（合并 `d7af7cf`） |
| 基线 | `022da5b`（M7 总设计与它的审查之后的 main）；本文提交之后开分支 `m7-p1` |
| 上级文档 | [M7 总设计](00-M7-design.md) 4.1、4.3、4.13、第 7 节；[M0/P6 镜像里的附件目录](handoffs/M0-P6-image-volumes.md)第 1、2 项；总体设计 11、13.1 第 14、15、20 条 |

---

## 1. 基线

- `platform/` 下没有存储；镜像（`deploy/Dockerfile`）的运行时阶段只有 `/nervewiki`，没有可写的目录。
- 逐路由的中间件只有两套：生成代码的 `API.Middlewares`（请求期限 `request_timeout`，带正文的路由另加 `read_timeout`）与长连接的 `API.LongLived`（没有请求期限、不读请求体）。`server.read_timeout` 30 秒管整个请求体，`server.write_timeout` 60 秒管答复；`NewServer` 的说明让要更久的处理器"自己用 `http.ResponseController` 放宽"，没有平台的做法。
- 限流：公开操作用 `anonymous`（按 IP），其余用 `authenticated`（按凭证）；没有按路由换桶的办法。

## 2. 目标与范围

**做**：

1. `platform/storage`：端口 `Store`、本地磁盘实现 `Local`、契约测试 `storagetest`、启动检查、磁盘余量（`ErrFull`）。
2. `platform/httpserver`：流式路由 `API.Stream(h, StreamPolicy)` 与处理器用的 `Bounded`、`Sending`。
3. 配置 `storage.dir`、`storage.min_free_bytes`；serve 在开始服务之前打开存储，不可写时拒绝启动。命令行（`migrate`、`users`、`workspaces`、`reindex`）不打开存储。
4. 镜像：`/data` 卷，属主 65532，`NWIKI_STORAGE__DIR=/data`；`image-smoke.sh`：数据目录用具名卷，另加"`/data` 不可写时拒绝启动"一步。
5. README 的部署一节：`/data` 的挂载与属主、匿名卷、反向代理对上传的设置；`.gitignore`、`.dockerignore` 加开发时的 `server/data/`。
6. e2e：每个服务（含 `nervewikiWith`）有自己的存储目录。

**不做**（与 [M7 总设计](00-M7-design.md)第 7 节的出入，随这份文档改 00 号文档）：

- `x-raw` 的契约规则与代码生成的排除挪到 P2：P1 还没有这样的操作，没有它们可核对的东西。P1 的 `API.Stream` 由 httpserver 自己的测试路由验证。
- 平台码 `storage_full`（507）挪到 P2：答出它的第一个操作在 P2。P1 的存储答 `storage.ErrFull`。
- `ratelimit.asset_content` 与 `asset.upload_min_rate` 是 asset 的配置，随 P2。
- `image-smoke` 的"上传、重启、读出"一步随 P2（第一个写附件的接口）。

## 3. 设计

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `platform/storage/storage.go`（新） | 端口：`Store`、`Writer`、`File`；`ErrNotFound`、`ErrFull`；键的规则 `CheckKey` |
| `platform/storage/local.go`（新） | `OpenLocal(dir, minFree)`：启动检查、清掉残留的临时文件；`Create`、`Open`、`Delete`、`List`、`Free` |
| `platform/storage/free_darwin.go`、`free_linux.go`、`free_other.go`（新） | 剩余字节数（`syscall.Statfs`：darwin 是 `Bavail × Bsize`，linux 是 `Bavail × Frsize`，`Frsize` 为 0 时用 `Bsize`；别的系统答错误） |
| `platform/storage/storagetest/storagetest.go`（新） | 契约测试 `Run(t, open func(t) Store)` |
| `platform/httpserver/stream.go`（新） | `StreamPolicy`、`API.Stream`、路由的桶（`routeLimit`：有 `Bucket` 时取代 `anonymous` 与 `authenticated`）、`Bounded`、`Sending` |
| `platform/httpserver/api.go` | `APIConfig.WriteTimeout`；`NewAPI` 的校验 |
| `platform/httpserver/server.go` | `NewServer` 的说明改指 `API.Stream` |
| `platform/config/config.go`、`validate.go`、`configs/config.yaml`、`config.dev.yaml` | `storage` 一节；dev 的目录 `_data` |
| `bootstrap/wire.go`、`deps.go`、`app_test.go` | serve 打开存储；`apiConfig` 交 `WriteTimeout`；测试的配置用临时目录 |
| `internal/archtest/rules_test.go`、`composition_test.go` | `storagetest` 进 `testHelpersOnlyInTests`；命令行的组合（含迁移的三个）不打开存储，serve 打开 |
| `deploy/Dockerfile`、`deploy/image-smoke.sh`、`README.md`、`.gitignore`、`.dockerignore` | 第 3.5 节 |
| `e2e/fixtures/server.ts` | 每个服务一个存储目录 |

### 3.2 存储端口

```go
// Store keeps files by key (M7 design 4.1). It knows keys and bytes only.
type Store interface {
    // Create starts a file at key; it is visible once its Writer commits.
    Create(ctx context.Context, key string) (Writer, error)
    Open(ctx context.Context, key string) (File, error)   // ErrNotFound when there is none
    Delete(ctx context.Context, key string) error         // none is success
    // List calls each with the key of every file of area last modified before before.
    List(ctx context.Context, area string, before time.Time, each func(key string) error) error
    Free(ctx context.Context) (int64, error)               // bytes left to write
}
type Writer interface { io.Writer; Commit() error; Abort() error }
type File interface { io.ReadSeekCloser; io.ReaderAt; Size() int64; ModTime() time.Time }
```

- **键**：`<区>/<名>` 两段。区是小写字母（`blobs`、`imports`、`exports`），名是 `[0-9a-z.-]`、不以 `.` 开头、至多 128 字节。别的键答错误（`CheckKey`），不碰磁盘：键来自调用方，但写错的键不能越出根目录。
- **写入**：`Create` 先看余量，低于 `min_free_bytes` 答 `ErrFull`；写、`Commit` 时磁盘写满（`ENOSPC`）或超出配额（`EDQUOT`）同样答 `ErrFull`，临时文件删掉。`Commit` 与 `Abort` 至多一个生效，之后的调用答错误；`Commit` 之前文件不可见，同一个键上原有的文件在此之前照旧可读，`Abort` 不影响它。`Commit` 只在最后的 `fsync` 失败时文件已在键上（崩溃后未必还在），调用方按孤儿处理。一个 `Writer` 同时只给一个 goroutine 用。
- **读**：`Open` 答的 `File` 有 `ReaderAt`（`archive/zip` 要它，P6）、大小与修改时刻。
- **列出**：只列提交了的文件，不列临时文件；`each` 答错误即停。修改时刻是写入的真实时刻（孤儿清扫按它删一天以前的文件），契约测试钉住。

### 3.3 本地实现

- **目录**：`<dir>/<区>/<分片 1>/<分片 2>/<名>`，两级分片取名的 SHA-256 的前两个字节（各 256 个目录）：一个目录里不会有几十万个文件。名可以是 UUIDv7（前段是时间，按它分片一段时间的文件都在一个目录里），所以按哈希，不按名的前缀。总设计 4.1 写的"按 id 随机的尾部"改为按哈希。
- **临时文件**在每个区自己的 `<dir>/<区>/.tmp/` 里：`rename` 总在一个区之内，分开挂载子目录也不会跨文件系统（总设计 4.1 原写在启动时检查跨文件系统，这样就不必）。`Commit`：`fsync` 文件、关闭、建分片目录、`rename` 到位、`fsync` 分片目录。`Abort`：关闭、删除。
- **建目录**（区、`.tmp/`、分片）：新建的目录 `fsync` 它的上级，一直到根目录；一次只有一个 goroutine 建，另一个看到目录已在时它已经同步过（P1 审查 A-L2）。
- **启动**：`OpenLocal(dir, minFree)` 建根目录（`0o750`）；先删掉全部残留：逐个已有的区（目录，或指向目录的链接；指向不存在或够不着之处的链接拒绝启动，写明它）删掉整个 `.tmp/`（残留的临时文件，或占着这个名字的文件），根下残留的探测文件也删掉。这些残留只有进程中途被杀才会留下，单实例下启动时没有在途的写；总设计 4.6 的孤儿清扫因此只管 `blobs/`。然后探测：在根下写一个探测文件、`fsync`、删掉，再在每个区的 `.tmp/` 里同样探测（以别的用户恢复了备份时，根可写而区不可写）、删掉它；删不掉或不可写时答错误，写明目录与进程的 uid、gid（`serve` 打印它并退出，退出码 1）。先删后探：写满了盘的往往正是半截的文件，探测看到的是删完之后的盘。探测时写满（`ENOSPC`、`EDQUOT`）不算不可写：照常启动（只读的页面照样能看），写入答 `ErrFull`；遇到的错误由 `FullAtOpen` 交给组合根，它与"余量低于 `min_free_bytes`"都让启动记一条 WARN（配额、inode 用尽时 `statfs` 的余量并不低）。"单实例"只写在文档里，不加文件锁：锁会让先起新、后停旧的重启起不来；两个进程误用一个目录时，后起的删掉前一个的临时文件，前一个的 `Commit` 失败，不会写坏（P1 审查 B-Q2、A-L9）。
- **余量**：`Free` 是 `statfs` 的可用块数乘块的大小（3.1）。

### 3.4 流式路由 `API.Stream`

```go
// StreamPolicy is how a stream route reads its body and sends its answer
// (M7 design 4.3).
type StreamPolicy struct {
    // MaxBytes bounds the request body; 0 is server.max_body_bytes.
    MaxBytes int64
    // MinRate is the slowest average rate, in bytes a second, at which the
    // body may arrive and a response announced with Sending may leave.
    MinRate int64
    // Bucket, when set, is the route's rate-limit bucket in place of the
    // platform's anonymous and authenticated ones; BucketName names it in logs.
    Bucket     Limiter
    BucketName string
}

func (a *API) Stream(h http.Handler, p StreamPolicy) http.Handler
// Bounded bounds a stream handler's step that is not the stream (a check
// before the body, the write after it) by server.request_timeout from now.
func Bounded(ctx context.Context) (context.Context, context.CancelFunc)
// Sending announces the answer, once, with the body read: its write deadline
// moves on with its bytes at the route's MinRate (P2 replaced Sending(r, n):
// the answer's length is not always known before it is written).
// ErrShuttingDown once the server is shutting down.
func Sending(r *http.Request) error
```

- **次序**：请求信息 → 在 `request_timeout` 之内的失败闸门与认证（公开的操作不认证，照 `PublicOperations`）→ 路由的桶（没有 `Bucket` 时照 `rateLimit`）→ 请求体上限（`MaxBytesReader`）→ 处理器。认证与限流的期限照 `API.LongLived` 的 `opening`、`opened`。
- **处理器的上下文**没有请求期限，停机时按下面的规则取消；不是流的步骤经 `Bounded` 取 `request_timeout` 的期限（模块的处理器照此，P2 的测试核对）。
- **读**：进入处理器时读截止时间是"处理器的开始时刻 + `read_timeout`"（比服务器的晚出读请求头与认证的时间）。请求体每读过一步，重设为"开始时刻 + `read_timeout` + 已读字节 / `MinRate`"：平均速率达不到就断开；不发正文的连接在原来的时刻断开。一步是 64 KiB；`MinRate` 低到 64 KiB 要超过半个 `read_timeout` 时，取半个 `read_timeout` 按 `MinRate` 传的字节数，达到速率的请求体总有半个 `read_timeout` 的余地（P1 审查 A-L4）。读到请求体结尾的那一次不推后：之后 net/http 在后台读连接，不设读截止时间，设了到期就会取消上下文。同样的缘故，**没有请求体的请求**（下载、`HEAD`、`Content-Length: 0`：服务端的 `r.ContentLength == 0`，请求体被中间件包了一层也认得）从一开始就不设读截止时间（P1 审查 A-H1）。
- **写**：写截止时间总是"读截止时间 + (`write_timeout` − `read_timeout`)"，随读截止时间一起推后：读完请求体之后，处理器有 `request_timeout` 做它的写、再写出答复（配置已要求 `read_timeout + request_timeout < write_timeout`）。`Sending(n)` 把它设为"现在 + `read_timeout` + n / `MinRate`"，给下载用（是重设，n 很小时可能比原来的早）。
- **停机**：开始停机时，还在传字节的流（请求体没读到结尾、或已经 `Sending`）被切断：读、写的截止时间都设为现在、上下文取消；上传读到一半失败，处理器 `Abort`；下载写到一半失败。处理器尚未读请求体时（例如在预检里）同样取消。两者之间的一步（读完请求体之后写入单元、答复）照普通请求在 `shutdown_timeout` 之内做完：默认 20 秒长于 `request_timeout` 的 15 秒，已经传完的上传不因重启丢掉，取消也不会落在 `COMMIT` 上（P1 审查 A-M2）。停机开始之后 `Sending` 一律答 `ErrShuttingDown`（导出给处理器分辨，P2 的下载据此答 503；被切断的请求体读也答它（读本身失败时包着原错误，测试分辨"超时"与"停机"），P2 的上传不把它记成 ERROR），截止时间不再推后；`Sending` 之后直到处理器返回都算在传字节。处理器返回之后开始的停机不碰截止时间，net/http 最后的写照常完成（P1 审查 A-L5）。
- **不支持截止时间的 writer**（中间件包了一层却没有 `Unwrap`）：是接线的错误，记日志、答 500，同 `LongLived`。认证、限流的拒绝在这之前答出。
- 截止时间的计算饱和，不溢出（字节数很大、速率很小）。

### 3.5 配置、组合根与部署

- **配置**：

  ```yaml
  storage:
    # 附件、导入导出的文件所在的目录（M7）。镜像里是 /data（镜像的环境变量 NWIKI_STORAGE__DIR）。
    # 只能由一个 nervewiki 进程使用（v0.1 只支持单实例）
    dir: data
    # 剩余空间低于它时不再接受写入
    min_free_bytes: 1073741824
  ```

  `dir` 不能为空，`min_free_bytes` 不能为负（`validate`，各有测试）；`LogValue` 列出两项。dev 的目录是 `_data`（`make run` 在 `server/` 下运行，即 `server/_data/`：下划线开头，go 的 `./...` 不进去）；test 的配置不改，测试与 e2e 各自给目录。
- **组合根**：`newApp` 在接线之前 `storage.OpenLocal(cfg.Storage.Dir, cfg.Storage.MinFreeBytes)`，失败就返回错误（`serve` 不启动）；日志记绝对路径与余量；余量已低于 `min_free_bytes`、或打开时遇到写满（`FullAtOpen`）时另记 WARN `storage is full`，后者带着那个错误。P1 只持有它，P2 交给 asset。`apiConfig` 交 `WriteTimeout`。`testConfig` 用 `t.TempDir()`。
- **镜像**：构建阶段 `mkdir /out/data`，运行时阶段 `COPY --from=server --chown=65532:65532 /out/data /data`、`ENV NWIKI_STORAGE__DIR=/data`、`VOLUME /data`。
- **`image-smoke.sh`**：镜像声明了卷 `/data` 与 `NWIKI_STORAGE__DIR=/data`（`docker inspect`）；服务的容器挂具名卷 `-v <名>-data:/data`（不留匿名卷，结束时删掉）；另起一次 `serve`（后台运行，等它退出，不会卡住），`/data` 是属主为 root 的 tmpfs（`--tmpfs /data:uid=0,gid=0,mode=0755`）：退出码非零，日志里有 `cannot write in /data as uid 65532`。
- **README 的部署一节**：`/data` 要挂载（不挂载时 Docker 建匿名卷，删容器就丢），环境变量优先于配置文件的 `storage.dir`；宿主机目录的属主是 65532（`chown 65532:65532`，Kubernetes 的 `fsGroup`）；单实例，升级先停旧的；只需备份 `blobs/`，先数据库后目录，成对恢复；反向代理：上传的路由（附件、导入）不缓冲请求体（nginx `proxy_request_buffering off`），`client_max_body_size` 不小于导入包的上限；nginx 的超时是两次读写之间的间隔，按整个请求计时的代理才要容得下最慢的传输。
- **`.gitignore`、`.dockerignore`**：`/server/data/`、`/server/_data/`（13.5 第 3 条：两者同步）。
- **e2e**：`fixtures/server.ts` 给每个服务一个目录（`NWIKI_STORAGE__DIR`）：日志文件旁的 `<名>.data`，启动前清空，留着供失败时查看；CI 上传测试结果时不带它。

## 4. 实施步骤

| 步 | 内容 | 提交 |
|---|---|---|
| S1 | `platform/storage`：端口、本地实现、契约测试、启动检查与余量；archtest | `storage: a store of files by key on the local disk, committed atomically (M7/P1/S1)` |
| S2 | `httpserver`：`API.Stream`、路由的桶、`Bounded`、`Sending`；`WriteTimeout` | `httpserver: stream routes, timed by bytes at a minimum rate (M7/P1/S2)` |
| S3 | 配置、组合根、镜像、`image-smoke`、README、忽略的文件、e2e 的目录；总体设计 13.1 第 15、20 条 | `config, bootstrap, deploy, e2e: the storage directory, refused when not writable (M7/P1/S3)` |

## 5. 测试与验证

- **存储的契约**（`storagetest.Run`，本地实现跑它）：
  - 提交之前 `Open` 答 `ErrNotFound`、`List` 不列；提交之后读出同样的字节，大小对，修改时刻是写入的时刻（前后 2 秒），一小时前为界的 `List` 不列它；
  - 同一个键上写新文件时，提交之前读出旧的，`Abort` 之后仍是旧的，提交之后是新的；
  - `Abort` 之后什么也没有；`Commit` 之后再 `Abort`、`Abort` 之后再 `Commit`、两次 `Commit` 都答错误；
  - `ReaderAt` 与 `Seek` 读出指定的部分；
  - 删除之后读不到，删除不存在的键答 nil；
  - `List` 按区与时刻：修改时刻早于 `before` 的才列，别的区不列，`each` 的错误让它停下并答出；
  - 坏的键（`..`、绝对路径、三段、大写、空、以 `.` 开头、过长）都被拒绝，什么也不写。
- **本地实现**：
  - 分片的位置；两个区各自的 `.tmp/`；
  - 进程中途退出（只写不提交、不 `Abort`）留下的临时文件不可见，下一次 `OpenLocal` 删掉它；
  - 余量：`min_free_bytes` 大于磁盘的剩余时 `Create` 答 `ErrFull`；写满（测试注入 `ENOSPC` 的文件）答 `ErrFull` 并删掉临时文件；
  - 余量：超出配额（注入 `EDQUOT`）同样答 `ErrFull`；探测时磁盘已满（注入写满的文件）照常打开、删掉残留，写入答 `ErrFull`；
  - 区里的 `.tmp` 是文件、区是指向别处的链接时，启动照样删掉它的 `.tmp`；区是指向不存在之处的链接时拒绝启动、写明它；
  - `FullAtOpen`：有余量时为空；删掉残留之后有了余量时为空；只有一个区写满（它自己的配额）时是那个错误；
  - 启动检查：根目录或其中一个区不可写（`0o500`）时 `OpenLocal` 的错误写明那个目录与 uid；目录不存在时建出来；根目录是文件时答错误；残留的探测文件删掉。以 root 运行时跳过不可写的那一个（root 写得进去）。
- **流式路由**（`stream_test.go`；截止时间用一个记下截止时间的假 writer 断言，再用真的服务器跑一遍）：
  - 次序：未认证答 401、桶空答 429，都在处理器之前、读请求体之前；公开的操作不认证；有 `Bucket` 时不消耗 `anonymous`、`authenticated`，没有时照平台的；
  - 读截止时间：开始时是 `read_timeout`，每 64 KiB 按速率推后，写截止时间跟着；一步的大小（低速率时取半个 `read_timeout` 的字节数）有表格测试；
  - 真的服务器（`read_timeout`、`write_timeout` 都很短）：比 `read_timeout` 久、但达到速率的上传读完，处理器之后的写完成，64 KiB 要超过 `read_timeout` 的低速率也一样；停住的上传在 `read_timeout` 加已读字节应得的时间之内断开；`Sending` 之后的大答复（16 MiB，远多于套接字缓冲装得下的约 650 KB）在停读 1.5 秒的客户端上写完，不经 `Sending` 的同一个答复在 `write_timeout` 失败（服务端的发送缓冲 256 KiB：比回环的 64 KiB 段小的缓冲在 Linux 上让每次发送都等内核的计时器，吞吐只剩约 3 MB/s，CI 上失败过两次）；没有请求体的请求活过 `read_timeout`，上下文不被取消；
  - 请求体超过 `MaxBytes` 答 `*http.MaxBytesError`；
  - `Bounded` 在 `request_timeout` 之后到期；处理器的上下文没有期限；
  - 停机：上传读到一半时开始停机（客户端还在发、或已经停住），读立即失败、答 `ErrShuttingDown`、上下文取消，`Shutdown` 不等它；太慢、停住、超过上限的读不答它；按阶段的表格（假 writer）：读请求体之前与 `Sending` 之后切断并取消，读完之后、没有请求体的不切断、不取消，`Sending` 都答错误，处理器返回之后不切断；真的服务器上读完请求体的上传在停机中做完它的一步并答 200；
  - 截止时间的计算在极大的字节数与 1 字节每秒时饱和；
  - 不支持截止时间的 writer 答 500。
- **配置**：`storage.dir` 为空、`min_free_bytes` 为负时启动失败；`TestBuiltInProfiles` 照旧。
- **组合根**：存储目录不可写时 `newApp` 失败，错误里有目录与 uid；命令行的组合不打开存储（`archtest/composition_test.go` 的起点不到达 `storage.OpenLocal`）。
- **镜像**：`make image-smoke`（3.5）。
- **反向对照**：
  - 存储：`Commit` 不 `rename`（直接写到位）；`Commit` 之前可见；`List` 列出临时文件；`OpenLocal` 不删残留；`CheckKey` 放过 `..`；不看余量；`EDQUOT` 不算写满；不探测区；留下探测文件；`Create` 先删掉旧文件；修改时刻与 `List` 的比较整体挪后两天；先探测、后删残留；逐区删、探交错；探测时写满不放行或不记下；
  - 流式路由：不推后读截止时间；按已读字节而不按开始时刻算（停住的连接永远不断）；写截止时间不跟着；`Sending` 不设；桶照用 `anonymous`；认证在桶之后；停机不设截止时间；没有请求体时也设读截止时间；读到结尾不记下；停机总是切断；不看处理器已返回；`Sending` 不记下；停机不取消上下文；一步总是 64 KiB；
  - 组合根：不打开存储；镜像不建 `/data`（smoke 失败）。
  每个都要有测试失败。
- `make check`、`make gen-check`、`make e2e`、`make image-smoke`。

## 6. 完成标准

- 存储过契约测试；提交之前不可见、半途不留文件；启动时不可写就拒绝启动并写明 uid。
- `API.Stream`：慢而达到速率的上传与下载完成，停住的连接按时断开，次序与桶各有测试，停机时取消。
- 镜像有 `/data` 卷，`image-smoke` 证明不可写时拒绝启动。
- 总体设计 13.1 第 15、20 条与 00 号文档的出入落档。

## 7. 结果

- 提交：S1 `e33dacf`（存储），S2 `09c962e`（流式路由），S3 `f7ecf6a`（配置、组合根、镜像、e2e），负对照补的测试 `bac9a66`；审查的修复 `13f5ab9`；CI 的 Linux 上下载测试的改写 `bf9f54e`；五轮修复核对的修复 `d671e17`、`02ab8c7`、`7dd7f76`、`e2ecdc9`、`cd3fa37`。合并 `d7af7cf`。
- 审查：两位审查者（Opus）。A（存储与流式路由，对照 `net/http` 源码与真实服务器的探针）：High 1（没有请求体的流式请求在 `read_timeout` 时被取消上下文：net/http 在调用处理器之前就后台读连接），Medium 2（契约不钉修改时刻；停机取消读完请求体之后的一步），Low 10。B（配置、部署与文档）：Medium 2（`image-smoke` 的拒绝一步可能卡住；总设计的遗漏），Low 8，Nit 9。修复核对五轮，第五轮没有行为上的发现。逐条见[审查记录](reviews/P1-storage-stream-review.md)。
- 审查之后改了的设计（3.3、3.4 已改写）：
  - 停机只切断还在传字节的流（请求体没读到结尾、或已 `Sending`），之间的一步照普通请求在 `shutdown_timeout` 之内做完；之后 `Sending` 与被切断的读答 `httpserver.ErrShuttingDown`（P2 的下载据此答 503，上传不记 ERROR）。
  - 没有请求体的请求（`r.ContentLength == 0`）不设读截止时间；低速率时一步小于 64 KiB，达到速率的请求体总有半个 `read_timeout` 的余地。
  - 启动先删掉全部残留、再探测根与各区；磁盘或配额写满照常启动、写入答 `ErrFull`，`FullAtOpen` 与余量一起决定启动的 WARN；断链的区拒绝启动。Linux 的余量按 `Frsize`。
- CI 的 Linux 上"经 `Sending` 的大答复"一测失败两次：发送或接收缓冲小于回环的 64 KiB 段时，每次发送都等内核的计时器，吞吐约 3 MB/s（在 Linux 容器里限 2 核、`-race` 复现）。改为服务端的发送缓冲 256 KiB、客户端停读 1.5 秒再读完 16 MiB。
- 反向对照：55 个（存储 26、流式路由与 `API` 26、配置 2、组合根 1）都被测试抓到；去掉 `defer s.finish()` 的一个存活，接受（竞态没有确定的测试）。
- CI 与发布：分支 `cd3fa37` 的 CI 全部通过（server、web、image、e2e）。合并之后 `make image-smoke` 通过（`d7af7cf`）。
- 交给 P2 的：
  1. multipart 的上传、导入在最后一部分之后把请求体读到结尾（总设计 4.4），否则读截止时间不解除、停机时算"还在传"。
  2. 读请求体的错误先判断 `httpserver.ErrShuttingDown`（它同时满足超时与 `*http.MaxBytesError` 的判断）；`APIErrors.Write` 没有它的分支（会记 ERROR、答 500），上传不要原样交给它。经 `arm` 切断时答的是不包原错误的 `ErrShuttingDown`。
  3. 停机开始之后的下载答 503（码由 P2 定，总设计 4.5）。
  4. 每条连接都能占满 `read_timeout + MaxBytes/MinRate`，同时的上传数只由桶限制：P2 考虑按凭证限制同时的上传。
  5. `Commit` 失败时文件可能已在键上：上传按孤儿处理（孤儿清扫收拾），不在 `Commit` 失败后再删。
- 负责人可以改判的取舍：停机不取消读完请求体之后的一步（A-M2）；不加文件锁，单实例只写在文档里（A-L9、B-Q2）；磁盘写满照常启动（第二轮 L1）。
