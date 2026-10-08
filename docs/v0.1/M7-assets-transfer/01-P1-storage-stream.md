# M7/P1 平台：存储与流式路由：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M7/P1 平台：存储与流式路由 |
| 状态 | 进行中 |
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
| `platform/storage/free_*.go`（新） | 剩余字节数（`syscall.Statfs`；darwin 与 linux 的字段类型不同） |
| `platform/storage/storagetest/storagetest.go`（新） | 契约测试 `Run(t, open func(t) Store)` |
| `platform/httpserver/stream.go`（新） | `StreamPolicy`、`API.Stream`、`Bounded`、`Sending` |
| `platform/httpserver/api.go` | `APIConfig.WriteTimeout`；`NewAPI` 的校验 |
| `platform/httpserver/limit.go` | 路由的桶：有 `Bucket` 时取代 `anonymous` 与 `authenticated` |
| `platform/httpserver/server.go` | `NewServer` 的说明改指 `API.Stream` |
| `platform/config/config.go`、`validate.go`、`configs/config*.yaml` | `storage` 一节 |
| `bootstrap/wire.go`、`deps.go`、`app_test.go` | serve 打开存储；`apiConfig` 交 `WriteTimeout`；测试的配置用临时目录 |
| `internal/archtest/rules_test.go` | `storagetest` 进 `testHelpersOnlyInTests` |
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
- **写入**：`Create` 先看余量，低于 `min_free_bytes` 答 `ErrFull`；写、`Commit` 时磁盘写满（`ENOSPC`）同样答 `ErrFull`，临时文件删掉。`Commit` 与 `Abort` 至多一个生效，之后的调用答错误；`Commit` 之前文件不可见。
- **读**：`Open` 答的 `File` 有 `ReaderAt`（`archive/zip` 要它，P6）、大小与修改时刻。
- **列出**：只列提交了的文件，不列临时文件；`each` 答错误即停。

### 3.3 本地实现

- **目录**：`<dir>/<区>/<分片 1>/<分片 2>/<名>`，两级分片取名的 SHA-256 的前两个字节（各 256 个目录）：一个目录里不会有几十万个文件。名可以是 UUIDv7（前段是时间，按它分片一段时间的文件都在一个目录里），所以按哈希，不按名的前缀。总设计 4.1 写的"按 id 随机的尾部"改为按哈希。
- **临时文件**在每个区自己的 `<dir>/<区>/.tmp/` 里：`rename` 总在一个区之内，分开挂载子目录也不会跨文件系统（总设计 4.1 原写在启动时检查跨文件系统，这样就不必）。`Commit`：`fsync` 文件、关闭、建分片目录（新建的目录 `fsync` 它的上级）、`rename` 到位、`fsync` 分片目录。`Abort`：关闭、删除。
- **启动**：`OpenLocal(dir, minFree)` 建根目录（`0o750`）；在根下写一个探测文件、`fsync`、删掉；不可写时答错误，写明目录与进程的 uid、gid（`serve` 打印它并退出，退出码 1）。然后删掉各区 `.tmp/` 里残留的临时文件：只有进程中途被杀才会留下，单实例下启动时没有在途的写。总设计 4.6 的孤儿清扫因此只管 `blobs/`。
- **余量**：`Free` 是 `statfs` 的 `Bavail × Bsize`。

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
// Sending extends the write deadline for n more bytes at the route's MinRate.
func Sending(w http.ResponseWriter, r *http.Request, n int64) error
```

- **次序**：请求信息 → 在 `request_timeout` 之内的失败闸门与认证（公开的操作不认证，照 `PublicOperations`）→ 路由的桶（没有 `Bucket` 时照 `rateLimit`）→ 请求体上限（`MaxBytesReader`）→ 处理器。认证与限流的期限照 `API.LongLived` 的 `opening`、`opened`。
- **处理器的上下文**没有请求期限，开始停机时取消；不是流的步骤经 `Bounded` 取 `request_timeout` 的期限（模块的处理器照此，P2 的测试核对）。
- **读**：进入处理器时读截止时间是"开始时刻 + `read_timeout`"（与服务器的相同）。请求体每读过 64 KiB，重设为"开始时刻 + `read_timeout` + 已读字节 / `MinRate`"：平均速率达不到就断开；不发正文的连接在原来的时刻断开。
- **写**：写截止时间总是"读截止时间 + (`write_timeout` − `read_timeout`)"，随读截止时间一起推后：读完请求体之后，处理器有 `request_timeout` 做它的写、再写出答复（配置已要求 `read_timeout + request_timeout < write_timeout`）。`Sending(n)` 把它设为"现在 + `read_timeout` + n / `MinRate`"，给下载用。
- **停机**：开始停机时把读、写的截止时间都设为现在、取消上下文；之后不再推后截止时间，`Sending` 答错误。上传读到一半失败，处理器 `Abort`；下载写到一半失败。
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

  `dir` 不能为空，`min_free_bytes` 不能为负（`validate`，各有测试）；`LogValue` 列出两项。dev 照基础配置（`make run` 在 `server/` 下运行，即 `server/data/`）；test 的配置不改，测试与 e2e 各自给目录。
- **组合根**：`newApp` 在接线之前 `storage.OpenLocal(cfg.Storage.Dir, cfg.Storage.MinFreeBytes)`，失败就返回错误（`serve` 不启动）；P1 只持有它，P2 交给 asset。`apiConfig` 交 `WriteTimeout`。`testConfig` 用 `t.TempDir()`。
- **镜像**：构建阶段 `mkdir /out/data`，运行时阶段 `COPY --from=server --chown=65532:65532 /out/data /data`、`ENV NWIKI_STORAGE__DIR=/data`、`VOLUME /data`。
- **`image-smoke.sh`**：服务的容器挂具名卷 `-v <名>-data:/data`（不留匿名卷，结束时删掉）；另起一次 `serve`，`/data` 是属主为 root 的 tmpfs（`--tmpfs /data:uid=0,gid=0,mode=0755`）：退出码非零，日志里有 `uid 65532` 与目录。
- **README 的部署一节**：`/data` 要挂载（不挂载时 Docker 建匿名卷，删容器就丢），宿主机目录的属主是 65532（`chown 65532:65532`）；只需备份 `blobs/`，先数据库后目录；反向代理：上传的路由（附件、导入）不缓冲请求体（nginx `proxy_request_buffering off`），`client_max_body_size` 不小于导入包的上限，读写超时容得下 64 KiB/s 的最低速率。
- **`.gitignore`、`.dockerignore`**：`/server/data/`（13.5 第 3 条：两者同步）。
- **e2e**：`fixtures/server.ts` 给每个服务一个 `mkdtemp` 的目录（`NWIKI_STORAGE__DIR`），服务停下之后删掉。

## 4. 实施步骤

| 步 | 内容 | 提交 |
|---|---|---|
| S1 | `platform/storage`：端口、本地实现、契约测试、启动检查与余量；archtest | `storage: a store of files by key on the local disk, committed atomically (M7/P1/S1)` |
| S2 | `httpserver`：`API.Stream`、路由的桶、`Bounded`、`Sending`；`WriteTimeout` | `httpserver: stream routes, timed by bytes at a minimum rate (M7/P1/S2)` |
| S3 | 配置、组合根、镜像、`image-smoke`、README、忽略的文件、e2e 的目录；总体设计 13.1 第 15、20 条 | `config, bootstrap, deploy, e2e: the storage directory, refused when not writable (M7/P1/S3)` |

## 5. 测试与验证

- **存储的契约**（`storagetest.Run`，本地实现跑它）：
  - 提交之前 `Open` 答 `ErrNotFound`、`List` 不列；提交之后读出同样的字节，大小与修改时刻对；
  - `Abort` 之后什么也没有；`Commit` 之后再 `Abort`、`Abort` 之后再 `Commit`、两次 `Commit` 都答错误；
  - `ReaderAt` 与 `Seek` 读出指定的部分；
  - 删除之后读不到，删除不存在的键答 nil；
  - `List` 按区与时刻：修改时刻早于 `before` 的才列，别的区不列，`each` 的错误让它停下并答出；
  - 坏的键（`..`、绝对路径、三段、大写、空、以 `.` 开头、过长）都被拒绝，什么也不写。
- **本地实现**：
  - 分片的位置；两个区各自的 `.tmp/`；
  - 进程中途退出（只写不提交、不 `Abort`）留下的临时文件不可见，下一次 `OpenLocal` 删掉它；
  - 余量：`min_free_bytes` 大于磁盘的剩余时 `Create` 答 `ErrFull`；写满（测试注入 `ENOSPC` 的文件）答 `ErrFull` 并删掉临时文件；
  - 启动检查：目录不可写（`0o500`）时 `OpenLocal` 的错误写明目录与 uid；目录不存在时建出来；根目录是文件时答错误。以 root 运行时跳过不可写的那一个（root 写得进去）。
- **流式路由**（`stream_test.go`；截止时间用一个记下截止时间的假 writer 断言，再用真的服务器跑一遍）：
  - 次序：未认证答 401、桶空答 429，都在处理器之前、读请求体之前；公开的操作不认证；有 `Bucket` 时不消耗 `anonymous`、`authenticated`，没有时照平台的；
  - 读截止时间：开始时是 `read_timeout`，每 64 KiB 按速率推后，写截止时间跟着；
  - 真的服务器（`read_timeout`、`write_timeout` 都很短）：比 `read_timeout` 久、但达到速率的上传读完，处理器之后的写完成；停在第一个 64 KiB 的上传在 `read_timeout` 加它应得的时间之内断开；`Sending` 之后的大答复在慢读的客户端上写完（两端的套接字缓冲设小），不经 `Sending` 的同一个答复在 `write_timeout` 失败；
  - 请求体超过 `MaxBytes` 答 `*http.MaxBytesError`；
  - `Bounded` 在 `request_timeout` 之后到期；处理器的上下文没有期限；
  - 停机：上传读到一半时开始停机，读立即失败、上下文取消，`Shutdown` 不等它；
  - 截止时间的计算在极大的字节数与 1 字节每秒时饱和；
  - 不支持截止时间的 writer 答 500。
- **配置**：`storage.dir` 为空、`min_free_bytes` 为负时启动失败；`TestBuiltInProfiles` 照旧。
- **组合根**：存储目录不可写时 `newApp` 失败，错误里有目录与 uid；命令行的组合不打开存储（`archtest/composition_test.go` 的起点不到达 `storage.OpenLocal`）。
- **镜像**：`make image-smoke`（3.5）。
- **反向对照**：
  - 存储：`Commit` 不 `rename`（直接写到位）；`Commit` 之前可见；`List` 列出临时文件；`OpenLocal` 不删残留；`CheckKey` 放过 `..`；不看余量；
  - 流式路由：不推后读截止时间；按已读字节而不按开始时刻算（停住的连接永远不断）；写截止时间不跟着；`Sending` 不设；桶照用 `anonymous`；认证在桶之后；停机不设截止时间；
  - 组合根：不打开存储；镜像不建 `/data`（smoke 失败）。
  每个都要有测试失败。
- `make check`、`make gen-check`、`make e2e`、`make image-smoke`。

## 6. 完成标准

- 存储过契约测试；提交之前不可见、半途不留文件；启动时不可写就拒绝启动并写明 uid。
- `API.Stream`：慢而达到速率的上传与下载完成，停住的连接按时断开，次序与桶各有测试，停机时取消。
- 镜像有 `/data` 卷，`image-smoke` 证明不可写时拒绝启动。
- 总体设计 13.1 第 15、20 条与 00 号文档的出入落档。

## 7. 结果

（完成后补写。）
