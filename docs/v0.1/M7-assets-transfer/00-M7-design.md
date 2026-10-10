# M7 附件与导入导出：总设计

| 项 | 内容 |
|---|---|
| 里程碑 | M7 附件与导入导出（`M7-assets-transfer`） |
| 日期 | 2026-10-08 |
| 状态 | 进行中 |
| 依赖 | M4、M5、M6 |
| 上级文档 | [v0.1 总体设计](../v0.1-design.md) 第 3.3、3.5、3.7、3.8、3.10、3.11、4.3–4.6、6.1、6.4、7.2、8.3–8.5、9.2–9.4、10.1、11、12.4、13 节 |

---

## 1. 目标

1. **附件**：页面下、笔记本的根下可以挂文件。网页上从附件面板上传，或在编辑器里粘贴、拖入；附件按 Obsidian 的写法嵌入与链接（`![[架构图.png]]`、`![说明](架构图.png)`、`[[报告.pdf]]`），图片、音频、视频在阅读视图里内联显示，其他类型显示为可下载的链接。
2. **安全地下发**：附件经短期签名地址读取（`<img>` 带不了 Bearer 令牌），响应头让附件不能在本站的源里执行脚本，也不能向外站发请求。
3. **附件是节点**：与页面共用命名空间、回收站与清理；改名、移动时，指向它的链接照页面的规则改写；删除到期之后文件随之删除。
4. **导出**：把一个笔记本或一棵子树导出为 zip，结构与 Obsidian 的库一致（总体设计 3.5 的导出映射），附 `.nerve/meta.json`；导出的库在 Obsidian 里打开，链接解析得与本系统相同。
5. **导入**：把 zip（Obsidian 的库、本系统的导出）导入到一个笔记本的某个位置，后台运行，有进度与报告；导入的页进链接索引，导出再导入得到同样的树。其他 Markdown 的 zip（如 Notion 导出的）按同样的映射导入，不另做适配。
6. **部署**：附件目录是镜像的卷，不可写时拒绝启动；备份照总体设计第 11 节先数据库、后附件目录。

## 2. 范围

**做**：

- 平台：存储端口 `platform/storage` 与本地磁盘实现；流式路由 `httpserver.API.Stream`（逐路由的中间件、按字节的截止时间、路由自己的限流桶）；后台任务的队列与超时可配、只投递的客户端。
- `asset` 模块：附件的元数据（`asset_blobs`）、上传、签名的下载与响应头、列表、跟着节点删除、清理文件、孤儿文件的清扫、笔记本的活动。
- page 模块：给 asset、transfer 的树写入端口（附件节点的新建、导入的页面新建）、深度只数页面、附件的名称规则、变更集的类型（`import`）与并入、接口的 `NodeKind` 加上 `asset`、读端口。
- 链接：附件进解析；指向附件的一切写法（嵌入、wikilink、Markdown 链接与图片、属性链接）渲染为附件的标记（M7 建立的扩展点）；改名、移动附件时改写链接；落点与补全认附件。
- 前端：页面树只列页面；附件面板（页面的中栏、笔记本首页）；上传的服务（进度、取消，经会话的中间件）；编辑器的粘贴、拖入上传（编辑器扩展）；阅读视图里的附件（签名地址的到期、媒体的保留）；导入导出的对话框与笔记本设置里的任务列表。
- `transfer` 模块：导入导出的任务（`transfer_jobs`）、自己的队列、导出、导入、报告、取消、中断的收拾、到期清理、文件的清扫；导出贡献者（M7 建立的扩展点）。
- 部署：镜像的卷与属主、启动时的可写检查、`image-smoke` 的附件一步、README 的挂载、反向代理与备份。

**不做**：

- 对象存储（S3 等）：端口留着，v0.1 只有本地磁盘（单实例，总体设计第 11 节）。
- 替换附件的内容：附件写入后不变；要换就删了再传。
- 按工作区或笔记本的总量配额（第 10 节的风险）：只有单个文件、导入包的上限，同时进行的任务数，磁盘余量的检查。
- 附件的去重（按 SHA-256；M10 的来源区要，写进它的移交）。
- 上传 `.md`：拒绝，网页提示用导入或新建页面（总体设计 3.7 随之修订）。
- 页面嵌入的展开（`![[页面]]` 的转写）：照 M6 显示为链接。
- 附件的版本；回收站里的恢复（M8，写进它的移交）。
- 命令行的导入导出：只经接口（M2/P4 的移交第 3 项；组合规则不改）。
- 导入时改写链接：正文逐字节保留，链接按名称解析（导入的结构与原库一致时自然解析到）。
- 名称不是 UTF-8 的 zip 条目（旧版 Windows 资源管理器按本地代码页写的）：跳过并写进报告（M12 的移交）。
- 预览 Office 文档、给图片生成缩略图、点开图片放大。

## 3. 完成标准

总体设计 12.5 的通用标准之外：

1. **存储端口**：本地实现过端口的契约测试（写入的原子性、半途失败不留可见的文件、按键读、`ReaderAt`、删除不存在的键算成功、按区与时间列出）；附件目录不可写时 `serve` 拒绝启动，错误里写明 uid 与目录。
2. **流式路由**：慢而达到最低速率的上传能完成，停住的连接在 `server.read_timeout` 加已读字节按最低速率应得的时间之内断开，限速下的大文件下载能写完，停机开始时取消；中间件的次序与按路由的桶各有测试，反过来的次序有测试失败；签名的下载不消耗匿名请求的桶。
3. **上传与下载**：
   - 上传流式写入、边写边算 SHA-256，超过上限即中止并答 413；预检的每个码在读文件之前答出（handler 测试）；
   - 签名地址的伪造、过期、改动任何一个参数、多出或重复的参数都答 404；同一小时里签出的地址相同；密钥由已知答案的测试钉住；
   - 每种类型的响应头有表格测试（内联的、按附件下载的，都带限制外站请求的 `sandbox` CSP）；e2e 在浏览器里证明 svg 不在本站的源里执行、不向外站发请求，html 被下载。
4. **删除与清理**：删除子树、删除笔记本（三条路径）时附件的行随之软删除；清理先删文件、后删行，"文件删了、行没删"之后的下一次运行能完成；提交结果不明时不删文件；孤儿文件被清扫；附件计入笔记本的活动。每条路径都有整个程序上的行为测试，组合根不登记时失败（总体设计 13.1 第 21 条）。
5. **链接**：
   - 附件的解析与改写有样例（`resolve/`、`rename/` 加附件），与 Obsidian 1.12.7 核对；
   - 每种写法指向附件时的渲染有测试与样例，HTML 里从不出现指向附件的 `data-nw-node`；`CheckHTML` 只许 `img`、`audio`、`video` 的地址是附件的签名地址；
   - M6 的性质测试（增量维护的索引等于重建）加上附件的新建、改名、移动、删除；不变式 `checkLinks` 认附件。
6. **导出与导入**：
   - 导出的 zip 与总体设计 3.5 的映射逐项核对；导出的库在隔离的 Obsidian 里逐条解析链接，与本系统的索引一致；
   - 导出再导入得到同样的树、正文、附件与次序（性质测试：随机的树）；
   - 一个 Obsidian 库的样例导入之后，链接与 Obsidian 解析的一致；
   - 恶意的 zip（越出目录、符号链接、压缩炸弹、中央目录的条目数谎报、坏的名称、过深、过多）都被拒绝或跳过并写进报告，不留下文件；
   - 导入中途失败、取消、服务重启时，报告写明已导入的部分，任务不停在"运行中"。
7. **并发与权限**：第 4.14 节列出的交错各有确定性的测试；权限矩阵覆盖新操作的每一格。
8. **前端**：附件面板（页面与笔记本首页）、上传的服务、粘贴与拖入、阅读视图的附件、导入导出的对话框与任务列表各有 vitest，经组合根到达；e2e 的 AS、TR 系列故事（第 9 节）通过；粘贴上传的输入法步骤加进清单。
9. **部署**：`make image-smoke` 上传一个附件、重启容器之后读出同样的字节。

## 4. 关键决定

### 4.1 模块与端口

| 位置 | 内容 |
|---|---|
| `platform/storage` | 存储端口 `Store` 与本地磁盘实现。只认键与字节，不知道附件、笔记本 |
| `platform/httpserver` | 流式路由 `API.Stream`（4.3） |
| `platform/jobs` | 队列与超时可配、只投递的客户端（4.9） |
| `modules/asset` | `asset_blobs`、上传、签名的下载与响应头、列表、跟着节点删除、清理、孤儿清扫、笔记本的活动、渲染要的附件信息 |
| `modules/transfer` | `transfer_jobs`、导出、导入、报告、取消、收拾、到期清理、文件的清扫、导出贡献者 |
| `modules/page` | 节点仍只在这里；给 asset、transfer 的树写入端口与读端口 |

- **树写入端口**：附件节点与导入的页面要在 page 的写入单元里建，而写入单元要守卫、参与者与观察者，只有接好线的 page 模块有（`page.New` 里的 `app.Writer`）。所以 page 的模块根在接好线的模块上给出 `(*page.Module).TreeWrites()`：一个窄端口，跑一个写入单元，单元里只有两种操作：
  - `CreateAsset(parent, name, meta, after)`：建 `kind = asset` 的节点，`meta`（MIME、字节数、SHA-256）随操作交给守卫（M10 的来源区按哈希判断），`after(ctx, node)` 在同一个事务里运行（asset 写自己的行）；
  - `CreatePage(parent, name, draft)`：`draft` 是事先经同一端口的 `Parse` 解析好的正文与提取结果（P6）；
  - 另有不加锁的预检 `Check`（角色、父节点是这本笔记本里活着的页面、名称合法且此刻没被占用），同 `Writer.Allowed`。
  - 这是 13.1 第 11 条的例外：别的模块的端口原本只凭连接池构造。组合根按 page → asset → transfer 的次序建，命令行的组合不建 `page.New`，所以到不了它（`archtest/composition_test.go` 加上这一断言）。
- **附件的端口**：asset 的模块根给出 `asset.NewBlobs(pool, store, logger)`：`Put(ctx, r)` 把文件写进存储（边写边算 SHA-256、测定类型），`Attach(ctx, node, blob)` 在调用方的事务里写行（经 `postgres.DB(ctx, pool)`），`Open` 按节点读文件。transfer 的导入与导出用它，asset 自己的上传也用它。
- **读端口**：asset、transfer 读节点经 page 的只凭连接池的读端口（一个父节点下的附件、节点的笔记本与类型、导出范围里的树与正文），linking 给 transfer "导出范围里哪些没有正文的页是链接的目标"、给 asset 附件的 `link`（4.7）。asset 的 sqlc 看不到 `nodes`。
- **存储端口**（`platform/storage`）：

  ```go
  type Store interface {
      Create(ctx context.Context, key string) (Writer, error) // 写到临时文件；Commit 才可见
      Open(ctx context.Context, key string) (File, error)     // io.ReadSeekCloser、io.ReaderAt、Size、ModTime
      Delete(ctx context.Context, key string) error           // 不存在算成功
      List(ctx context.Context, area string, before time.Time, each func(key string) error) error
      Free(ctx context.Context) (int64, error)                // 剩余的字节数
  }
  type Writer interface { io.Writer; Commit() error; Abort() error }
  ```

  - 键由调用方给：`blobs/<blob id>`、`exports/<job id>.zip`、`imports/<job id>.zip`。本地实现按名的哈希散开目录（`blobs/3f/a2/<id>`；UUIDv7 的前段是时间，按它散开，一段时间的文件都在一个目录里）。
  - 写入：在这个区自己的 `.tmp/` 里建临时文件，`Commit` 时 `fsync`、`rename` 到位、`fsync` 目录（新建的分片目录同样）；`Abort` 删掉临时文件。读者只看得到完整的文件；`rename` 总在一个区之内，不会跨文件系统。
  - 启动检查：根目录不存在就建；写一个探测文件再删掉，不可写时返回带 uid 的错误，`serve` 拒绝启动（M0/P6 的移交）；已有的区先删掉残留的临时文件（只有进程中途被杀才会留下），再同样探测（在它的 `.tmp/` 里）；根下残留的探测文件也删掉。磁盘写满不算不可写：照常启动，写入答 507（P1 修复核对第二轮 L1）。
  - 磁盘余量：写入之前看 `Free`，低于 `storage.min_free_bytes`（默认 1 GiB）时答 507 `storage_full`（新的平台码，6.1）；写到一半写满同样。
  - 契约测试放在 `platform/storage/storagetest`（加进 archtest 的 `testHelpersOnlyInTests`），本地实现与以后的实现都跑它。

### 4.2 附件节点（page 模块的改动）

- **新建**：`CreateAsset` 建的节点没有正文、没有版本；父节点必须是页面或根（`lineOf` 已经如此），名称与页面共用命名空间（`titleFree` 已经如此）。上传是普通的单次写入，变更集类型 `edit`。
- **名称**：照 `shared.CheckTitle`，另加两条（新建与改名都查，按类型；422 的字段错误 `name: not_allowed`，前端的文案说明原因）：
  - 附件的名称不能以 `.md` 结尾（不分大小写；总体设计 3.7："`.md` 文件总是页面"）；
  - 附件改名不能去掉已有的扩展名：没有扩展名的附件链接解析不到（Obsidian 给没有扩展名的目标加 `.md`），去掉就保不住指向它的链接（4.5 的原则）。可以改扩展名：下发的类型来自上传时测定的 MIME，不看名称。没有扩展名的文件可以上传与导入，只是链接不到，与 Obsidian 相同。
- **深度只数页面**：附件不能有子节点，它不算一层。`Subtree.Height()` 与移动、新建的深度检查、不变式 `checkPages`、前端拖动时的深度只数页面：深度 10 的页面下仍可以挂附件；单独移动一个附件时它的高度是 0（总体设计 3.5 随之写明）。
- **不改 `domain.Change`**：初稿给它加 `Kind`，但没有真正的使用者：观察者按被删的节点 id 找自己的行，事件只带 `tree`，前端从节点树取类型，linking 从 `LinkTargets` 取类型。所以不改。
- **接口**：`NodeKind` 加上 `asset`（`api/modules/page.yaml`，`TreeNode.name` 与 `NodeKind` 的说明随之改）；`ListNodes` 照旧返回全部节点（前端的"未命名 N"要看全部子节点）。读正文、编辑会话、勾选任务这些接口对附件仍答 `page.not_found`（现在已如此）。
- **变更集的类型与并入**（P6）：`changesets.kind` 加上 `import`（迁移归 page），`UnitSpec` 加 `Kind`（默认 `edit`）与 `Changeset`（并入已有的变更集）。并入在笔记本行的锁下核对那个变更集属于同一本笔记本、类型相同、执行者与客户端相同，再 `TouchChangeset`，笔记本的活动与日志的次序看得到之后的批；编辑会话的写不能并入导入的变更集。13.1 第 2 条"一个写入单元一个变更集"改为"……，导入的各个单元并入第一个单元的变更集"。
- **先解析，后单元**：导入的页在开单元之前经端口的 `Parse` 排队取解析预算（`Budget.Take`，此时不持锁）解析，提取结果那一份留到单元结束；取不到时退避重试，至多到任务的超时。不在持锁时解析（总体设计 4.3、13.1 第 19 条）。
- **一个单元里先建后删**（M4/P2 的移交）：导入只建不删，不会遇到；由 M9 的 batch 定下。这份移交以"不适用"关闭，指向 M9 的那份。

### 4.3 平台：流式路由

上传与下载要逐字节地读写，时间按字节数放宽，生成代码的处理器与逐路由的中间件都做不到：中间件的请求期限（`request_timeout` 加 `read_timeout`，15–45 秒）会取消处理器的上下文，`server.write_timeout`（60 秒）会让长的写失败，而 strict 的处理器拿不到 `ResponseWriter`，不能放宽截止时间，也不能 `ServeContent`。

- **`API.Stream(h, StreamPolicy)`**（照 `API.LongLived`，模块不重写这些部件）：
  - 次序：请求信息 → 在 `request_timeout` 之内的失败闸门与认证（公开的操作不认证）→ 路由的限流桶 → 请求体上限 → 处理器。处理器之前的答复（401、429）对有请求体的请求带 `Connection: close`：请求体没读，net/http 本会先再读至多 256 KiB，或等一个听到答复才发的客户端（P2 审查 B9）；处理器自己决定它的答复。
  - `StreamPolicy`：`MaxBytes`（请求体上限）、`MinRate`（最低速率）、`Bucket`（这条路由用的桶，取代 `anonymous` 或 `authenticated`）与它在日志里的名字 `BucketName`；公开与否照模块的 `PublicOperations()`。
  - 读：连接的读截止时间每读 64 KiB（`MinRate` 低到 64 KiB 要超过半个 `read_timeout` 时取更小的一步）重设为"开始时刻 + `read_timeout` + 已读字节 / `MinRate`"：平均速率达不到最低速率就断开，不发正文的连接在原来的 `read_timeout` 断开。没有请求体的请求（下载）不设读截止时间：net/http 从一开始就在后台读连接，截止时间一到会取消处理器的上下文（P1 审查 A-H1）。
  - 写：写截止时间是"读截止时间 + (`write_timeout` − `read_timeout`)"，跟着读截止时间放宽：上传读完之后还有 `request_timeout` 写入、答复。下载由处理器经平台宣告（`Sending`）：写截止时间从"此刻 + `read_timeout`"起，每写一步（同读的一步）重设为"宣告时刻 + `read_timeout` + 已写字节 / `MinRate`"，一次长的写按步写出；照最低速率读的客户端拿到整个答复，停读的客户端在 `read_timeout` 加缓冲装下的字节应得的时间之后断开，而不是占住连接到"字节数 / `MinRate`"（P2 审查 B2，原先由 `Sending(n)` 一次设定）。
  - 期限：读完请求体之后的一步（写入单元）在一个新的 `request_timeout` 期限里运行。
  - 停机：开始停机时切断还在传字节的流（请求体没读完的上传、已经 `Sending` 的下载）：截止时间立刻到期、处理器的上下文取消，上传中止、临时文件删掉，下载断开。读完请求体之后的一步照普通请求在 `shutdown_timeout` 之内做完并答复，不在 `COMMIT` 上被取消（P1 审查 A-M2）；这时的 `Sending` 答错误。
- **按路由的桶**：签名的下载用 `ratelimit.asset_content`（按 IP），不消耗 `anonymous`（登录、续期、注册共用它，一页几百张图会让同一出口的同事续期失败、被登出）。上传用 `authenticated`。
- **契约**：这些操作照样写在接口描述里（`/api/` 下的路由都必须是描述里的操作，`TestAPIRoutesAreTheContractsOperations`），标 `x-raw: true`；代码生成按操作排除它们（oapi-codegen 的 `exclude-operation-ids`，由脚本从 `x-raw` 取出，有测试）；契约测试按描述核对它们的状态、头与媒体类型（下载写 `*/*` 与 206、304、416）。TS 的客户端照常生成，前端经同一个客户端调用（4.8）。
- **交叉规则**（`validate`）：`asset.upload_min_rate > 0`，`asset.max_bytes / asset.upload_min_rate` 有上限（默认约 13 分钟）。
- 总体设计 13.1 第 14 条加上 `API.Stream` 的次序，13.4 第 5 条加上 `x-raw` 的契约测试。

### 4.4 上传

- 接口：`POST /api/v0/notebooks/{notebook_id}/assets`，`multipart/form-data`。字段依次是 `parent_id`（可选，没有就挂在根下）、`name`（可选，没有或为空白就用文件部分的文件名）、`file`（最后一个）。未知或重复的字段、多个文件部分、文件之后还有部分答 400；文件之前的部分（含部分的头）不超过 4 KiB。答 201 和附件（4.6 的元数据）。
- **流式**：用 `mime/multipart.Reader` 逐部分读，不用 `ParseMultipartForm`（它会把大文件落到系统临时目录）。最后一部分之后把请求体读到结尾（`io.Copy(io.Discard, r.Body)`，结束边界之后至多几个字节）：读到结尾之前读截止时间不解除，停机时这个请求也算"还在传"（P1 审查 A 的疑问）。导入同此（4.11）。
- **次序**：
  1. 认证之后、读文件之前，不加锁的预检（`TreeWrites.Check`，同一判定与守卫的不加锁预检）：笔记本可写、父节点是这个笔记本里活着的页面、名称合法且此刻没被占用、磁盘余量。不通过就答 403、404、409、422、507。注意：处理器不读完请求体就答复时，net/http 至多再读约 256 KB 就关连接，浏览器与反向代理后面的客户端常常只看到连接被重置；所以网页在发送之前自己先查（4.8），这些码的测试在 handler 层。
  2. 把文件流进 `Store.Create("blobs/<新 blob id>")`：边写边算 SHA-256，记下前 512 字节测定类型；超过 `asset.max_bytes`（默认 50 MiB）即中止，答 413 `payload_too_large`（平台码，不另设 `asset.too_large`）。
  3. `Commit` 之后开写入单元：`CreateAsset` 加 asset 的行（同一个事务）。预检之后被抢先占用的名称、被删的父节点、失去的权限在这里答 409、404、403。
  4. 单元在提交之前失败（领域错误、回滚）时删掉刚写的文件；`COMMIT` 没有答复（`TxManager` 的"结果不明"）时不删：行可能已经提交，删了就是永远打不开的附件；留给孤儿清扫（4.6）。清扫只删一天以前的文件，远长于文件提交到行提交的时间，所以不会删掉正在上传的。
- **类型的测定**（适配器里，`net/http` 不进 app 与 domain）：嗅探前 512 字节（`http.DetectContentType`）与扩展名（自己的一张表，不读系统的 `mime.types`），再按一张容器兼容表决定：
  - 扩展名的类型在内联的白名单里（4.5），且嗅探的结果与它属于同一种容器（表里逐项列出：m4a 嗅出 `video/mp4`、ogg 嗅出 `application/ogg`、音频的 webm 嗅出 `video/webm`，都算同一种），或嗅探不出（`application/octet-stream`、`text/plain`；svg 嗅出 `text/xml`；avif、flac 嗅探不出）→ 用扩展名的类型；
  - 扩展名不认识、嗅探的结果在白名单里 → 用嗅探的；
  - 其余 → `application/octet-stream`（按附件下载）。嗅探出 HTML 的一律不内联。
  - 测定的类型存在行里，下发时只用它，不看客户端声明的类型，也不看之后改过的名称。
- **图片的宽高**：标准库能读的格式（png、jpeg、gif）上传时以 `image.DecodeConfig` 读出宽高存在行里，渲染时写在 `<img>` 上，避免图片载入时内容跳动（锚点与焦点的定位靠它）；读不出的不记。
- **重名**：接口照旧答 409，不静默改名；网页先取一个空着的名字（编号在扩展名之前："报告 2.pdf"，与 Obsidian 的写法一致）。

### 4.5 下载：签名地址与响应头

- **地址**：`GET /api/v0/assets/{node_id}/content?b=<blob id>&e=<到期的 Unix 秒>&s=<签名>`，加 `&d=1` 时按附件下载。公开的操作（`security: []`），签名就是凭据。
- **签名**：`HMAC-SHA256(key, "asset-content" ‖ node id 的 16 字节 ‖ blob id 的 16 字节 ‖ e 的 8 字节大端 ‖ d 的 1 字节)` 截成 16 字节，无填充的 base64url。密钥由组合根派生：`SigningKeys.Derive("nervewiki asset-content mac v1")`，info 由 asset 的模块根导出，由已知答案的测试钉住（总体设计 13.1 第 25 条）。
- **严格的读法**：地址照服务端的写法读：路径的 id 是规范写法（小写、带连字符、不转义，转义过的路径答 404；不是 id 的答 400，与任何操作的路径参数相同）；参数依次是 `b`、`e`、`s`、`d`，`b`、`e`、`s` 各恰好一次，`d` 至多一次，都不反转义；`b` 是规范写法的 id；`s` 是恰好那么长的无填充 base64url；`e` 是规范的十进制；`d` 没有或是 `1`；有别的参数答 404。在任何查询之前核对签名。
- **到期**：签名时取 `e = (⌊now / 1 小时⌋ + 2) × 1 小时`：地址在 1 到 2 小时之间有效，同一个小时里签出的地址相同，浏览器的缓存因此有用（总体设计 6.4）；跨过整点的地址变了，图片要重新下载一次（第 10 节）。
- **校验**：签名对、未到期、行存在且没有软删除、节点与 blob 对得上，才下发；任何一项不对都答 404（不区分"过期"与"不存在"），答复照旧 `no-store`。
- **谁签**：只有已经核对过读权限的读取会签：阅读视图的渲染、附件的列表与元数据、上传的答复。失去访问的账户手里已签出的地址在到期之前仍能读（至多 2 小时，第 10 节）。
- **响应头**（全局的 `nosniff`、`X-Frame-Options: DENY`、`Referrer-Policy: same-origin` 照旧，后者让签名不进 Referer）：

  | 类型 | `Content-Type` | `Content-Disposition` |
  |---|---|---|
  | 内联的图片、音频、视频（png、jpeg、gif、webp、avif、bmp；mp3、ogg、wav、m4a、flac；mp4、webm、ogv） | 测定的类型 | `inline` |
  | svg | `image/svg+xml` | `inline` |
  | pdf | `application/pdf` | `inline`（`sandbox` 下内置的阅读器能否用：P2 在 Chromium 里实测可用，写进 P2 文档；Firefox 与 Safari 没有实测，归 M12 的打磨第 17 项；不能用时按附件下载） |
  | 其余（含 html、xml、js、`application/octet-stream`） | `application/octet-stream` | `attachment` |

  - 每个附件的答复都带 `Content-Security-Policy: sandbox; default-src 'none'; img-src 'self' data:; media-src 'self'; style-src 'unsafe-inline'`：直接打开的 svg 不运行脚本、源是不透明的，也不能向外站请求图片、字体与样式（否则外站拿到读者的 IP，违反总体设计 4.6 的"`img-src` 只许本站"）。
  - `d=1` 时一律 `attachment`。文件名按 RFC 6266 写 `filename*=UTF-8''…`，另带一个 ASCII 的 `filename` 兜底。
  - 签名核对通过之后才设 `Cache-Control: private, max-age=<到期前的秒数>, immutable`（覆盖 `/api/` 默认的 `no-store`）、`ETag`（SHA-256）。`Content-Security-Policy` 与 `Cross-Origin-Resource-Policy: same-origin` 在每个答复上，拒绝（400、404、429、503）也带（P2 审查 B5）。
  - 用 `http.ServeContent` 下发：支持 `Range`（视频拖动）与对客户端所持副本的条件请求（`If-None-Match`、`If-Modified-Since`、`If-Range`）；Go 在 416 时去掉 `Cache-Control`，照它。一个地址所下发的内容从不改变，对改动的条件（`If-Match`、`If-Unmodified-Since`）没有要守的，下发之前去掉，不答 412（412 会带上文件的类型与缓存头，P2 审查 B3）。多段的 `Range`（带逗号）也去掉，整份答 200：每一段是答复里单独的一部分，几千段的请求头能换来文件大小许多倍的答复（[M7 收尾审查](reviews/M7-closeout-review.md) A-N1）；平台的 `httpserver.ServeFixed` 做这三件事，附件与导出的下载共用。
- **停机**：停机开始之后 `Sending` 答 `httpserver.ErrShuttingDown`，下载不再开始，答 503（码由 P2 定）。
- **元数据**：`GET /api/v0/assets/{node_id}` 答附件的元数据：节点的字段，加 MIME、字节数、SHA-256、宽高、内联与下载的两个签名地址（`d` 在签名里，所以是两个），P3 起加 `link`（4.7）。

### 4.6 删除、清理与孤儿文件

- **`asset_blobs`**（asset 模块，迁移 `00027_asset_asset_blobs.sql`）：`id`（blob id，UUIDv7）、`node_id` 唯一、`notebook_id`、`mime`、`byte_size`、`sha256`、`width`、`height`、`created_by_id`、`created_at`、`deleted_at`。外键：`(notebook_id, node_id)` 指向 `nodes(notebook_id, id)`（经它已经到达笔记本，`notebook_id` 不另指向 `notebooks`）、`created_by_id` 指向 `users`，指向别的模块的都是 `ON DELETE RESTRICT`。索引：活动用的（`notebook_id WHERE deleted_at IS NULL`）、清理用的。存储的键由 blob id 推出，不另存。表写进 `runtime-grants.sql`。
- **跟着节点删除**：page 的删除只改自己的表，asset 看不到 `nodes`。asset 登记：
  - **页面领域事件的观察者**（事务内）：改动里被删除的节点，`UPDATE … SET deleted_at = 事件的时刻 WHERE node_id = ANY(被删的 id) AND deleted_at IS NULL`（删子树时改动已带每个后代，附件也在其中）。总体设计 12.4 这一行的注册者加上 M7；M8 的恢复照同一个观察者恢复行，写进 M8 的移交。
  - **笔记本删除事件的订阅者**：软删除这些笔记本的未删的行。笔记本删除的三条路径（删除笔记本、删除无主笔记本、删除工作区）都经它。
  - 两者只凭连接池构造：命令行的组合（停用经 `notebookRegistrants`）也到达它们，它们不要存储与密钥。
- **清理器**：`asset.Purgers(pool, tx, store, logger)`（要存储，13.1 第 6 条的"登记"随之改写，组合根的 `purgers(pool)` 改为 `purgers(pool, tx, store, logger)`），排在 page 的清理器之前。一批在一个事务里：`FOR UPDATE SKIP LOCKED` 锁住到期的行 → 删文件（不存在算成功）→ 删行 → 提交。删文件不能回滚：提交失败时下一次运行再删一次文件（已不存在，算成功）、删掉行（M2/P4 的移交）。M8 的恢复锁同样的行，与清理串行。删不掉的文件让这一批失败、清理停下（"失败即停"，记日志带 blob id）。
- **孤儿清扫**（asset 的定时任务，每天）：`blobs/` 下修改时刻早于一天、而 `asset_blobs` 里（含软删除的）没有它的文件，删掉。上传在 `Commit` 之后、单元提交之前失败，提交结果不明，删除文件失败，留下的就是这些。临时文件由存储在启动时删掉（4.1）。transfer 的 `imports/`、`exports/` 由它自己的清扫收拾（4.9）。
- **笔记本的活动**：字节数是未删除的附件的大小之和（M3 的移交）。上传是树的一个单元，它的变更集已算作页面一侧的写，附件不再另报最晚的上传时刻（P2 审查 C3）。

### 4.7 附件进链接

照 [M6 的移交](handoffs/M6-links.md)，细节在 P3 文档，与 Obsidian 1.12.7 逐项核对：

- **解析**：附件是链接的候选。page 给 linking 的读端口（`LinkTargets`、`All`）带上节点的类型；接口的 `LinkTargetKind` 加上 `asset`。写法的规则：
  - 目标的最后一段不以 `.md` 结尾、笔记本里任何地方有名称（按标题键）等于它的附件时，只读作附件：候选只有这个名称的附件（按 4.4 的次序：相对、完整路径、后缀），同名的页面不算，找不到即解析不到；没有这样的附件时按页面的写法（样例 `resolve/` 016–019）；
  - 去掉 `.md` 的判断（"笔记本里有没有去掉 `.md` 的那个名称"）与 `.md` 的写法只数页面：`[[x.png.md]]` 只指向页面 `x.png`，永远不是附件 `x.png`（Obsidian 里 `x.png` 与 `x.png.md` 是两个文件）；
  - 没有扩展名的附件不是候选（Obsidian 给这样的目标加 `.md`）。
  - 要核对的形状写成 `resolve/` 样例：A 里的附件 `x.png` 与 B 里的页面 `x.png`、从 B 里链接；`[[B/x.png]]` 而 `x.png` 只在 A；`[[x.png.md]]` 时两者都在；没有扩展名的附件；大小写与 Unicode 的折叠；与附件同名的别名。
- **一种标记**：解析到附件的每一种写法（`![[…]]`、`[[…]]`、`[t](x.pdf)`、`![t](x.png)`、frontmatter 的属性链接）都写附件的标记（class `nw-asset`），地址是签名的 `src` 或 `href`，路径里有附件的 id（P3 起不另写 `data-nw-asset`：它与路径重复，多出的字节让 `CheckSize` 的放大超过 64 倍），**从不**写 `data-nw-node`（`appLinks` 会把它当作页面，去 `/…/pages/<id>`，那是 404）。
  - 嵌入与 Markdown 图片：图片 → `<img class="nw-asset" src=… alt=… width=… height=… loading="lazy">`；音频 → `<audio controls preload="none">`；视频 → `<video controls preload="none">`；其余（含 pdf）→ 附件的链接。Obsidian 对指向音频、视频的 Markdown 图片同样内联。
  - 尺寸照 Obsidian：`![[x.png|300]]`、`|300x200`，Markdown 的 `![说明|300](x.png)`；`|` 后面不是尺寸的是说明。数值限在 1–10,000。
  - 一个视图至多内联 20 个音频、视频（第 21 个起是附件的链接，有上限处的测试）：每个播放器占资源，Chromium 对一帧的播放器数有上限，超出的报错。
  - 一个视图至多写 2000 个附件的地址（图片、音视频、链接一样算，按文档的次序，之后是文字）：几个字节的写法就写出两百字节上下的标记，表格的行还会补齐，按字节算会超过 `CheckSize` 的 64 倍；有了上限是一个总量（约 0.4 MB，P3B 审查 B1）。
  - 链接里的图片（`[![](x.png)](url)`，README 里常见）：`<img>` 在 `<a>` 里不是链接套链接，照 Obsidian 渲染，不再只写说明。
  - 文件名与大小不写成界面的文字：服务端写 `data-nw-size`，前端按界面语言格式化（`formatBytes`）。`alt` 默认是写下的目标（照 Obsidian：`A/x.png`、`x.png > a`），音视频的 `aria-label` 取说明或写下的目标。
  - 标记写进 `Markup`，`CheckHTML`、`CheckSize` 随之更新；`CheckHTML` 只许 `img`、`audio`、`video` 的地址是附件内容的路径；`CheckSize` 的放大输入（`Amplifying()`）加上附件的嵌入：几个字节的 `![[x]]` 写出带签名地址的整个标记。
- **扩展点**（M7 建立并注册）：
  - `obsidian.Options` 的 `Resolve` 答出目标的节点与是否附件（`map[int]Target`，原来是 `map[int]uuid.UUID`）；另加 `Assets(ctx, notebookID, ids)`：组合根经 asset 模块给出每个附件的类型、大小、宽高与签名地址（没有名称：文字照写法）。
  - 核心的 Markdown 图片由 `platform/markdown` 渲染，扩展原来只能换它的 `<a>` 的属性（`Extension.Links`）。核心加一个钩子：图片按目标的起点问扩展，扩展答一个写自己登记的标记的函数，或不管（照旧 `<span class="nw-image">`）。嵌入与 Markdown 图片因此走同一段代码。
  - `Assets` 交空、或没有某个附件时，解析到附件的写法渲染为不带地址的文字，不写成页面的链接；整个程序的最后一跳测试在组合根交空时失败。
  - 渲染用的 `Assets` 只在 serve 的组合里给；reindex 的组合交空（它只提取），有测试证明提取不受它影响（`markdownExtensions(resolve, assets)`）。
- **属性链接**：`PropertyLink` 加 `kind` 与 `url`（附件的签名地址）；右栏的属性对附件给地址，不经 `href(lead)`。
- **改名、移动**：附件改名、移动时，解析到它的链接照页面的规则改写（M6 的改写参与者；Obsidian 打开"始终更新内部链接"时同样改写附件的嵌入）；移动、改名一页时，它子树里的附件同样进候选。`Linktexts` 给被同名附件遮住的页写 `x.png.md`。与 Obsidian 核对（`rename/`）。
- **落点**：落点的 `node_id` 从不是附件；读作附件的目标答新的原因 `target_is_asset`，读作页面、落点旁边却有同名附件的也这样答，`parents` 只有页面；前端照"没有落点"说明并重读视图（解决 M6 的移交第 3 项的循环）。
- **补全**：`LinkTarget` 带类型，补全列出附件并标出类型（`![[` 之后附件排前），没有扩展名的附件不列（任何写法都读不到它）；`link` 与名称的说明随之改（`linking.yaml`）。
- **附件的 `link`**：附件的元数据、列表与上传的答复带 `link`（与 `LinkTarget.link` 同一个算法：笔记本里只有这一个附件有这个名称时写名称，否则写完整路径；没有扩展名的附件为 null），粘贴插入与"复制嵌入"用 `![[link]]`，不会嵌到别处的同名文件。asset 经组合根从 linking 取，路径与同名附件的个数由 page 的一条语句读出（P3）。
- **地址在 HTML 里**：M6 的链接地址由前端给，是因为服务端不知道工作区的 slug；附件的签名地址服务端完全知道，所以渲染时直接写进 `src` 与 `href`（相对地址，过 `SafeURL`）。阅读视图本来就不缓存（总体设计 4.3），签名依赖时钟与密钥不是问题；总体设计 13.3 第 6 条写明这个例外。`PageView` 带 `assets_expire_at`（视图里最早的到期时刻），前端据此重读（4.8）。CSP 不改：`img-src 'self'`，`media-src` 退到 `default-src 'self'`。
- **索引**：附件的新建、改名、移动、删除经观察者重新解析指向它们的链接；`checkLinks` 认附件；M6 的性质测试与改写的随机测试加上附件的操作。

### 4.8 前端

- **页面树只列页面**：`stores/page-tree.ts` 分出页面的索引（侧栏、子页列表、面包屑、快速切换、移动对话框的上级列表、`findPages`、`canHold`、`heightOf`）与全部子节点（"未命名 N"的 `freeTitle` 看全部：共用命名空间）。节点树的事件不改：`tree: true` 已让看着的人重读。P2 先做这一步，免得附件在 P4 之前显示成页面。
- **树的重读经合并**：`pages` 事件里的整树重读改经 `refresher`（第一次立即，之后至多每 500 毫秒一次，`events/handlers.ts` 的 `TREE_INTERVAL_MS`）：一次导入几百个单元，每个都让每个打开的标签页读一次整树。原定的 5 秒让别人连续改树时最多晚 5 秒才显示（P2 审查 C1）；500 毫秒仍把一连串的单元合成每秒至多两次读。
- **附件面板**：照总体设计 9.2 放在中栏、子页面列表之下；笔记本首页（`notebook-home.tsx`）显示根下的附件（Obsidian 默认把附件放在库的根下，导入的库会有很多）。
  - 列出附件（名称、大小、类型的图标），游标分页，一页 100 项，"加载更多"；上传按钮与拖到这一节上传（带进度、可取消）；每一项的菜单：打开（内联类型在新标签页）、下载、复制嵌入（`![[link]]`）、改名（只改主名，保留扩展名）、移动到…（上级只列页面）、删除。行可以拖进编辑器（`text/plain` 是 `![[link]]`，CodeMirror 自己的拖放就插在落点）。
  - SWR 键 `["assets", 笔记本 id, 父节点 id 或 "root"]`，重读读已有的页数，同一父节点一次一个；`pages` 事件的 `tree: true`（树读完、显示之后，已不在的页不读）与连上时的整体刷新（与阅读视图、右栏同一层）重读它；读不到经 `NotLoaded`（13.2 第 7 条）；一个笔记本每代一个 store（13.2 第 15 条）。
  - 上传不经 `PageTreeStore` 的 `oneAtATime` 队列（50 MB 的上传会挡住每个树的写），是 13.2 第 1 条的例外；每个答复之后重读树（`wrote()`：一个在途，其间答复的合成下一次）与这一节，成功的上传读到它所在的页才离开。对话框关闭之后焦点回到这一行的菜单按钮，行已不在时到这一节的标题；"加载更多"读完最后一页时焦点到它加进来的第一项，读者其间动过就不移（13.2 第 26 条）。细节在 [P4 文档](04-P4-assets-web.md)第 3 节。
  - 删除页面的确认对话框把子树里的附件一起数上。
- **上传的服务**（13.2 第 1、6 条）：经会话的客户端调用，不另起一条路：openapi-fetch 支持按请求换 `fetch` 与 `bodySerializer`，上传的 service 交一个由 `XMLHttpRequest` 实现的 `fetch`（`fetch` 没有上传进度）：交给中间件的请求不带正文，传输发闭包里的 `FormData`、用中间件交来的 `Request` 的头，传输经 `AppStores` 注入；中间件 401 之后续期重发（`options.fetch(copy)`）时照样带新令牌重传，换代的核对不变。换代时 store 中止在途的上传。vitest 注入假的传输。导入的 zip 用同一个。字段按 `parent_id`、`name`、`file` 的次序加进 `FormData`。
- **发送之前先查**：`InstanceInfo` 加 `asset_max_bytes`、`import_max_bytes`（导出的确认对话框另用 P5B 的 `export_ttl_seconds`；13.1 第 15 条："服务端可配的量经接口告诉前端"）。网页在发送之前查角色、大小、名称合法、不是 `.md`、按标题键（`lib/title-key.ts`，近似服务端的大小写折叠）在树的兄弟与同一父节点下在途的上传之间取空着的名字，并照导入的规则替换名称里禁止的字符（`#[]|^:` 等换成 `_`，NFC）；上传途中的传输错误显示通用的"上传失败"。有在途的上传时挂上 `beforeunload`（13.2 第 21 条）。
- **粘贴、拖入上传**（编辑器扩展 `assetUpload`，`load` 的扩展，模块在 `editor/loaded/`，13.2 第 23 条；细节在 [P4 文档](04-P4-assets-web.md)第 5 节）：
  - `EditorContext` 加 `uploadAsset(file)`（经 `AssetStore` 上传到这一页，答出附件）；`EditorControls` 加 `whenComposed`（原来只在 `SourceEditorHandle` 上）、`tell`（编辑器说一句，显示并播报）、`going`（离开编辑要等的工作）。
  - 扩展先于 CodeMirror 自己的拖放接住带文件的拖入（它会把文件当文本读进来）：拖入插在落点（`posAtCoords`，能改的正文有落点的光标），粘贴插在选区；上传完成后在原位置（随之后的输入映射）插入 `![[link]]`，输入法组合中等 `whenComposed`；光标正好在那里时移到嵌入之后，同一位置后粘贴的在后面。
  - 剪贴板里同时有文字与图片的（Excel、Word、Numbers 复制的单元格）粘贴文字；有文件、没有 `text/plain` 时才上传。剪贴板里没有文件名的图片取名为 "Pasted image 20261008123045.png"（照 Obsidian 的写法，不随界面语言：它是存下来的内容）。
  - 编辑器发起的上传行显示在编辑器旁（进度与取消），文档里不加任何东西。上传完成时编辑器已换了正文、已关闭或已只读（失锁）的，与没有扩展名的，不插入，这一批答完之后说一句（"已上传，未插入"），附件留在面板里；上传失败时它的行说原因，不插入任何东西。Done、Mod+E 先等编辑器在途的上传落定、嵌入插完再保存离开，等待中失锁或撞上冲突就不走；闲置退出遇到在途的上传不走。
  - 拖入的 `.md` 交给 CodeMirror（插入它的文字）；文件夹不接，提示用导入（粘贴的也是）。
- **页面之外的拖入**：文件拖到拖放区之外时浏览器会打开它、离开应用，所以文档上对文件的 `dragover`、`drop` 一律 `preventDefault`（编辑器里的除外）；拖到阅读视图上上传到这一页；页内开始的拖动（Chromium 拖图片时带着文件）不上传。
- **阅读视图**（阅读视图的交互增强 `assets`，12.4 这一行加上 M7；细节在 [P4 文档](04-P4-assets-web.md)第 4 节）：
  - 指向附件的链接：内联类型在新标签页打开（`rel=noopener`，带看不见的"在新标签页打开"提示），其余由服务端写 `download`，直接下载、不开新标签页（Firefox 会留下空白的标签页）；链接之后按界面语言写大小。点图片不做什么（与 Obsidian 的默认相同）。
  - 地址到期：视图与属性从读到的时刻起，在最早的到期前一分钟重读（至少 30 秒、至多 59 分钟；隐藏的标签页显示时再读），缓存里第一次见到时已过期的不显示。加载失败（捕获阶段的 `error`，它不冒泡）时，若视图的到期时刻已过，合并成一次重读，同一个到期时刻至多重读一次：文件不在（404）、限流（429）、解不开的图片在同一小时里重读也一样失败，不能循环。
  - 已开始播放的音频、视频在 HTML 换掉时保留（按附件 id（地址的路径里）与出现的次序配对，同 M6 保留焦点的做法）：HTML 每小时因签名而变，别人的编辑、`links` 事件也让它变，`innerHTML` 会毁掉正在播放的媒体。开始了的加载失败时（通常是地址过期）经 `GET /assets/{id}` 就地重签，接着播放的位置、速率与音量；出错的不保留。
- **导入与导出的界面**：笔记本设置加"导入与导出"一节，列出最近的任务（状态、进度、报告、下载、已过期），关掉对话框、重新加载页面之后仍找得到；有运行中的任务时以 SWR 的 `refreshInterval` 每秒读一次，否则在最早的下载地址到期之前一分钟再读（P5B，[P5 文档](05-P5-export.md) 4.2）；报告的原因是码（前端按码给文案），不是服务端的文字。页面标题旁的菜单加"导出此页"（读者也有：树的操作菜单只给写者）。导入时选位置的对话框说明深度的限制。

### 4.9 后台任务

- **只投递的客户端**（M2/P4 的移交）：`platform/jobs` 加一个不配队列的 River 客户端，在请求的事务里 `InsertTx`，与 `transfer_jobs` 的行同一个提交，回滚时任务也不存在。事务由 `transfer/adapter/river` 经 `postgres.TxFrom(ctx)` 取出（`platform/jobs` 不导入 `platform/postgres`）。命令行不投递任务：组合规则（`archtest/composition_test.go`）不改。
- **队列与超时**（`platform/jobs` 改为可配）：导入、导出各一个队列（`transfer_import`、`transfer_export`，各 1 个 worker，`jobs.import_workers`、`jobs.export_workers`），不占定时任务的默认队列，几个小时的导入也不挡住导出。River 默认的任务超时是 1 分钟，worker 的 `Timeout()` 设为 `transfer.job_timeout`（默认 6 小时）；River 的 `RescueStuckJobsAfter`（默认 1 小时，`MaxAttempts` 为 1 时它会丢弃仍在运行的长任务）设为长于它。
- **不自动重试**：`MaxAttempts` 是 1。任务失败、被取消、超时时，任务自己经 `context.WithoutCancel`（停机时限时在 River 的宽限之内，其余 30 秒）把 `transfer_jobs` 记成失败或已取消并写报告。
- **心跳与收拾**：运行中的任务每秒写一次 `heartbeat_at` 与进度，同一条语句读回取消与删除。transfer 的 SQL 不读 River 的表（sqlc 的范围），靠心跳收拾：serve 启动时（单实例），把还记着"运行中"的任务都记成失败（"服务重启，任务中断"）；之后每 5 分钟，心跳早于 `transfer.heartbeat_timeout`（默认 5 分钟）的同样，再把排队而 River 已不持有的导出与导入（River 在开始之前丢掉了它）记成失败，River 持有哪些经只投递客户端的 `Unfinished` 读（P5A，[P5 文档](05-P5-export.md) 3.12）。导入的报告随心跳写（报告变了、至多每 10 秒），收拾只把失败并入它，报告写明到最后一次心跳为止已导入的部分（P6A，[P6 文档](06-P6-import.md) 3.14）。
- **任务的身份**：`transfer_jobs` 记下发起的账户与客户端（`web`、`api`）。`shared.Actor` 加第三种凭据 `JobID`（后台任务代账户执行时是任务的 id；三者恰好一个），worker 以 `Actor{UserID, JobID}` 与记下的客户端调用写入单元，每个单元照常授权：中途失去写权限或账户被停用时那个单元被拒绝，任务失败并写明。凭据之后失效（退出登录、撤销 PAT）不打断已开始的任务。日志带 `job_id`。
- **数量**：每本笔记本同时至多一个排队或运行中的导入（409 `transfer.busy`）；每人每本笔记本同时至多一个导出（同样 409），成功的导出只留最新的一份（新的成功时删掉旧的文件，记成已过期）；全部排队与运行中的任务至多 `transfer.max_queued`（默认 20，正在上传的导入也算），超出答 503 `server_busy`。开始之前看磁盘余量（507；导入按声明的长度，别的上传还没存下的字节也算）。正在上传的导入在进程内计数（`app.Uploads`，导出与导入共用）：同一本笔记本同时只有一个上传，第二个在读文件之前答 409（P6A，[P6 文档](06-P6-import.md) 3.9）。

### 4.10 导出

- **范围与权限**：整个笔记本，或一页及其子孙；能读这个笔记本的人都能导出（导出就是读，权限码 `transfer.export`）。开始时与任务开始运行时各判定一次。
- **一致的快照**：`TxManager` 加 `WithinSnapshot`（`REPEATABLE READ READ ONLY`），读端口经 `postgres.DB(ctx, pool)` 进入它。快照里只读数据库：范围内活着的节点、正文、附件的 blob、"没有正文的页里哪些是链接的目标"（linking 的端口；一页只看前 10,000 条链接，13.1 第 31 条），写出 `.md` 与 `meta.json`；之后提交，再从存储读附件的文件写进 zip（写入后不变）。不在 GB 级的导出期间持着快照与连接（`xmin`、`idle_in_transaction` 的超时）。附件的文件不在时写进报告，不让导出失败。进度在另一个连接上写。
- **映射**（总体设计 3.3、3.5）：zip 里有一层以笔记本（导出子树时是那一页）命名的根目录，库在它里面：页面写 `<标题>.md`，子节点在 `<标题>/` 里，附件是它的父页面目录里的文件，根下的附件在根目录里。
  - 没有正文、只有子节点的页：只有目录，除非导出的内容里有链接解析到它，这时另写一个空的 `.md`，让 Obsidian 里的链接同样解析到它（M6 的移交第 5 项）；没有正文也没有子节点的页写空的 `.md`。
  - 页面 `X.md` 带子节点时它的目录 `X.md/` 与页面 `X` 的文件 `X.md` 同名：检测出来，改名并写进报告。
  - zip 的条目名用 UTF-8（设置 UTF-8 标志），超过 4 GiB 时用 zip64（`archive/zip` 自动）。已经压缩过的格式（图片、音视频、zip）以 `Store` 写入，不再压缩。
- **`.nerve/meta.json`**（在根目录里）：

  ```json
  { "format": 1, "exported_at": "…", "notebook": { "id": "…", "name": "…" }, "root": null,
    "nodes": [ { "path": "项目A.md", "kind": "page", "id": "…", "sort_order": 1.5 } ],
    "contributed": [ "index.md" ] }
  ```

  导入时据它恢复兄弟的次序；`id` 只作参考，导入不复用它；`contributed` 列出贡献者加的文件。
- **导出贡献者**（M7 建立的扩展点，M10 注册虚拟的 `index`、`log`）：`transfer.ExportContributor`：`Contribute(ctx, scope, sink) error`，在快照里运行，往 zip 里加自己的文件（路径不能与节点的冲突，冲突时导出失败），加的文件记进 `contributed`。按登记的次序调用，第一个错误即停。M7 组合交空，模块根有示例的测试（次序、第一个错误、冲突）。
- **结果**：写到 `exports/<job id>.zip`，任务成功。下载经签名地址 `GET /api/v0/transfer-jobs/{job_id}/download?e=…&s=…`（`x-raw`、公开；签名照 4.5 的写法，密钥的 info 是 `nervewiki export-download mac v1`，地址 1–2 小时有效、不晚于导出的到期，每次读任务时重签：读任务要读权限）；`attachment`，文件名是笔记本或页的名称加 `.zip`。`transfer.export_ttl`（默认 24 小时）之后定时任务删掉文件，任务记成"已过期"。笔记本删除之后任务随之软删除，下载答 404。
- **进度**：写入的节点数 / 总数。

### 4.11 导入

- **接口**：`POST /api/v0/notebooks/{notebook_id}/imports`，`multipart/form-data`：`parent_id`（可选）、`file`（zip）。先判定（`transfer.import`，写者）与不加锁的预检（父节点、同时的导入与上传、队列、磁盘余量；声明的长度超过上限时立即 413），再读请求体，流式写到 `imports/<job id>.zip`（上限 `transfer.import_max_bytes`，默认 512 MiB，`API.Stream`），然后在一个事务里写 `transfer_jobs` 的行并投递任务，答 202 与任务。事务以工作区行 `FOR SHARE` → 笔记本行 `FOR SHARE` → 判定 → 写行与投递，同 13.1 第 5 条的页面一支。写行之前失败的，删掉已写的 zip。
- **先校验，后写入**：任务先把全部条目过一遍，不写任何东西：
  - 先读 zip 的结尾记录（EOCD 与 zip64 的 EOCD），条目数超过上限或中央目录大于上限时整包拒绝，然后才 `zip.NewReader`（它会先建出每一条的记录：512 MiB 的 zip 可以有约 700 万条，几个 GB 的内存）；
  - 整包拒绝（任务失败，报告写明）：不是 zip、条目超过 `transfer.import_max_entries`（默认 50,000，至多 100,000）、解压后的总字节数超过 `transfer.import_max_unpacked_bytes`（默认 4 GiB，按实际读出的字节计，不信条目头）；
  - 逐条跳过并写进报告：路径越出根（`..`、绝对路径、盘符）、符号链接与其他特殊文件、加密的、压缩方法不支持的、压缩比超过 200（小于 1 MiB 的不看：小文件能压得远超 200 倍）、名称不是 UTF-8、`.md` 不是合法的 UTF-8 或超过 5 MiB 或含 NUL、附件超过 `asset.max_bytes`、在笔记本里超过深度 10 的页与它下面的一切（附件不算一层）；
  - 忽略：`.obsidian/`、`.trash/`、`.git/`、`__MACOSX/`、`.DS_Store`、`Thumbs.db` 与其他以 `.` 开头的条目；`.nerve/meta.json` 读作次序（至多 16 MiB，超过的不读）。
- **映射**：
  - 只有一个顶层目录、而且其中有 `.obsidian/` 或 `.nerve/` 的，这一层是库本身，去掉（本系统的导出、打包整个库文件夹的 zip 都是这样）；
  - `X.md` 是页面 `X`（`.md` 不分大小写；正文逐字节）；目录 `X/` 是 `X` 的子节点，同级没有 `X.md` 时 `X` 是没有正文的页；`X.md` 与 `X/` 按标题键（NFC 之后：macOS 打的 zip 是 NFD）配对；其他文件是附件（`.md` 总是页面）；
  - `\` 当作路径的分隔符；任何一层的 `.`、空段忽略；重复的条目留第一个，其余写进报告；
  - 兄弟的次序照 `.nerve/meta.json`，没有就按名称。
- **名称**：照节点的规则修正并写进报告：禁止的字符换成 `_`，首尾的空白与 `.` 去掉（去掉之后再判断是页面还是附件），Windows 保留名后加 `_`，过长的在字符边界截到 255 字节（保留扩展名），空的叫"未命名"。撞名（按标题键，导入位置原有的子节点同样参与）在单元里、按锁下读到的兄弟决定：后来的加序号，在扩展名之前（"a 2.png"），跳过同一个库里后面的兄弟原样的名称（库里的 `[[a 2]]` 仍到库自己的那一页，P6A）；预先算好会被同时新建的兄弟撞上，让单元答 409、整个任务失败。改过名称的，正文里指向旧名称的链接会解析不到，报告逐条列出。
- **写入**：
  - 按层分批：一个写入单元至多 100 个节点、8 MiB 正文、20,000 条链接（一个单元都在笔记本行的 `FOR NO KEY UPDATE` 下，挡着这本笔记本的每一次保存）；单元的类型是 `import`；第一个单元建变更集，之后的单元并入它（"一次导入一个变更集"，总体设计 3.8；4.2）。
  - 每批的 `.md` 在单元之前经 `TreeWrites.Parse` 排队取预算解析（4.2）；每批的附件文件在它的单元之前才从 zip 写进存储（经 `asset.NewBlobs` 的 `Put`）；单元答 503 时退避重试、用同一批文件，单元失败时删掉它们，提交结果不明的留给孤儿清扫。
  - 新建的页经观察者进索引，之后建的页会让先前解析不到的链接重新解析到它们（M6 的观察者）。
  - 统计：导入期间每 10,000 个节点与结束时，对 page、linking、asset 的表 `ANALYZE`（M6 的移交第 4 项）：每个模块经自己模块根的端口做自己的表（transfer 不对别人的表执行 SQL）；服务的角色要这些表的 `MAINTAIN`，写进 `runtime-grants.sql`，运行时角色的测试跑一次导入。
- **取消**：`POST /api/v0/transfer-jobs/{job_id}/cancel` 只写任务行（记下取消的时刻），不碰笔记本行，所以与任务的单元不成环；任务在两批之间停下，报告写明已导入的部分。中途失败同样如此；服务重启之后收拾保留最后一次心跳写的报告（P6A）。已导入的部分照常是笔记本里的页，用户可以删除（M8 的整组撤销写进它的移交）。
- **报告**：计数（页、附件、跳过、改名、错误）与前 1,000 条问题（路径至多 1,024 字节、原因的码），存在 `transfer_jobs.report`，运行中随心跳写（至多每 10 秒）；只在任务结束之后的读取里给出，运行中只给进度。
- **进度**：已处理的条目 / 总数。不加新的事件类型。
- 导入结束（成功、失败、取消）时删掉 `imports/<job id>.zip`。

### 4.12 `transfer_jobs` 与文件

- **表**（transfer 模块，迁移 `00029_transfer_transfer_jobs.sql`）：`id`、`notebook_id`（`RESTRICT`）、`root_id`（不建外键：指向节点会挡住节点的清理）、`kind`（`import`、`export`）、`name`（导出的笔记本或页、导入的文件的名称）、`state`（`queued`、`running`、`succeeded`、`failed`、`cancelled`、`expired`，带检查的状态机）、`created_by_id`、`client`、进度的两个数、`cancel_requested_at`、`heartbeat_at`、`started_at`、`finished_at`、`report`、结果的字节数、`created_at`、`deleted_at`。不存凭据的 id（会话清理会删 `auth_sessions` 的行）。写进 `runtime-grants.sql`。
- **生命周期**：笔记本删除的订阅者软删除这些笔记本的任务（排队与运行中的任务下一批看到行已删，停下）；清理器排在 notebooks 之前；读、下载、取消都重新判定读权限（报告里有标题）。
- **文件的清扫**（transfer 的定时任务，每天）：`imports/`、`exports/` 下没有活着的任务对应的文件，删掉。

### 4.13 部署与配置

- **配置**：

  | 键 | 默认 | 说明 |
  |---|---|---|
  | `storage.dir` | `data`（镜像里 `/data`，经镜像的环境变量） | 附件、导入导出的文件 |
  | `storage.min_free_bytes` | 1 GiB | 低于它不再接受写入 |
  | `asset.max_bytes` | 50 MiB | 单个附件 |
  | `asset.upload_min_rate` | 64 KiB/s | 上传与下载的最低速率 |
  | `transfer.import_max_bytes` | 512 MiB | 导入包 |
  | `transfer.import_max_entries` | 50,000 | |
  | `transfer.import_max_unpacked_bytes` | 4 GiB | |
  | `transfer.export_ttl` | 24 h | |
  | `transfer.job_timeout` | 6 h | |
  | `transfer.heartbeat_timeout` | 5 min | |
  | `transfer.max_queued` | 20 | |
  | `jobs.import_workers`、`jobs.export_workers` | 1、1 | |
  | `ratelimit.asset_content` | 每分钟 6,000、突发 1,000（每 IP） | test 配置调到用不完 |

  交叉规则（总体设计 8.4，照实际：`platform/config/validate.go`）：`asset.max_bytes` ≤ `transfer.import_max_bytes` ≤ `transfer.import_max_unpacked_bytes`；`asset.max_bytes` 按 `asset.upload_min_rate` 传完不超过 1 小时，`transfer.import_max_bytes` 不超过 3 小时，`import_max_bytes` 至少 1 MiB；`transfer.import_max_entries` 在 1–100,000 之间；`transfer.heartbeat_timeout` 至少 1 分钟、短于 `transfer.job_timeout`（运行中的任务每秒心跳一次）；`jobs.export_workers` 与 `jobs.import_workers` 各在 1–8 之间，合起来至多 `database.max_conns` 的一半；River 的救援时间长于 `transfer.job_timeout`。各项进 `LogValue`。
- **镜像**（M0/P6 的移交）：构建阶段建好 `/data`、属主 65532，`COPY --chown` 进运行时阶段，`VOLUME /data`，镜像的环境变量把 `storage.dir` 指向它；README 的部署一节写挂载方式（宿主机目录的属主要是 65532；不挂载时 Docker 建匿名卷，删容器就丢）、反向代理（`client_max_body_size`、上传路由不缓冲请求体、超时）与备份次序（只需备份 `blobs/`）。开发时的 `data/` 加进 `.gitignore` 与 `.dockerignore`。
- **e2e**：每个 worker 的服务、`nervewikiWith` 起的服务各有自己的存储目录（`fixtures/server.ts`）。

### 4.14 权限、加锁与交错

- **权限**：access 的规则表加上 `asset.upload`（写者）、`asset.read`（读者：元数据与列表）、`transfer.export`（读者）、`transfer.import`（写者）、`transfer.read`（读者：列表与单个；用例只给本人发起的，笔记本管理员看全部）、`transfer.cancel`（读者：用例只许发起人与笔记本管理员）。附件的改名、移动、删除照旧是 `node.*`。下载靠签名，签名之前已按读权限核对。看不到笔记本的人看不到自己的任务（404）。
- **加锁**：上传的单元照页面一支（工作区行 → 笔记本行 `FOR NO KEY UPDATE` → `nodes` → `asset_blobs`）；asset 的观察者与订阅者只在已持的锁下写自己的行；任务的创建见 4.11；取消与进度、心跳只写任务行；清理器 `SKIP LOCKED`。总体设计 13.1 第 5 条写明 `asset_blobs` 在 `nodes` 之后、任务行不进页面一支。
- **交错**（13.4 第 4 条，`bootstrap/interleavings_assets_test.go`、`interleavings_transfer_test.go`）：
  - 上传的单元与父页的删除、笔记本的删除、失去写权限；
  - 附件的删除与清理；
  - 导入的单元与同一页的保存、与同一父页下的新建（名称在锁下决定）；
  - 任务的创建与笔记本的删除；取消与运行中的任务。
  - 结束时 `checkPages`（只数页面的深度）、`checkLinks` 与"每个活着的附件节点恰好一行"核对不变式。
- **日志**（13.1 第 10 条）：asset 只记 `notebook_id`、`node_id`、`blob_id`、`user_id`、MIME、字节数；transfer 只记 `notebook_id`、`job_id`、`user_id`、`client`、数量与原因的码。文件名、zip 里的路径、签名都不进日志；`loggedPath` 已不记查询串。签名地址是地址里的秘密，是总体设计 6.1"秘密不进地址"的例外：它们短期有效、只给已核对读权限的人，反向代理的访问日志会记下它们（第 10 节）。

## 5. 接口

| 方法与路径 | 说明 |
|---|---|
| `POST /api/v0/notebooks/{notebook_id}/assets` | 上传（`x-raw`，multipart） |
| `GET /api/v0/notebooks/{notebook_id}/assets?parent_id=` | 一个父节点下的附件，没有 `parent_id` 是根下（游标分页） |
| `GET /api/v0/assets/{node_id}` | 附件的元数据与签名地址 |
| `GET /api/v0/assets/{node_id}/content` | 下载（`x-raw`，公开，靠签名） |
| `POST /api/v0/notebooks/{notebook_id}/exports` | 开始导出（`root_id` 可选），答 202 与任务 |
| `POST /api/v0/notebooks/{notebook_id}/imports` | 上传 zip 并开始导入（`x-raw`，multipart），答 202 与任务 |
| `GET /api/v0/notebooks/{notebook_id}/transfer-jobs` | 这个笔记本的任务（本人的；笔记本管理员是全部；游标分页，新的在前） |
| `GET /api/v0/transfer-jobs/{job_id}` | 任务的状态与进度；结束之后带报告，导出成功时带下载地址 |
| `POST /api/v0/transfer-jobs/{job_id}/cancel` | 取消 |
| `GET /api/v0/transfer-jobs/{job_id}/download` | 下载导出的 zip（`x-raw`，公开，靠签名） |

改动：`NodeKind`、`LinkTargetKind` 加 `asset`；`PropertyLink` 加 `kind`、`url`、`inline`；`PageView`、`PageProperties` 加 `assets_expire_at`；落点的原因加 `target_is_asset`；`InstanceInfo` 加 `asset_max_bytes`、`export_ttl_seconds`、`import_max_bytes`。错误码：平台码 `storage_full`（507）；名称的规则是字段错误 `name: not_allowed`；`asset.not_found`；`transfer.not_found`、`transfer.busy`、`transfer.not_cancellable` 等，写进 P2、P5、P6 的文档。413 用平台的 `payload_too_large`。

## 6. 从 Nerve 借鉴

Nerve 的文件里程碑还没开始，只有计划与平台的做法（只读参考，不改它的文件）：

- 按用途派生密钥、截断的 HMAC、严格的令牌编码（`identity/adapter/signing/mac.go`）：本仓库已有同样的 `SigningKeys.Derive`，签名地址照用。
- 上传、下载按请求放宽截止时间（`http.ResponseController`），不动全局的超时；大小的上限与时间的上限分开（Nerve M0 的对抗评审）。本仓库在平台里做成 `API.Stream`，读写两边都放宽，并按速率而不是一个绝对的时刻。
- `/api/` 默认 `no-store`，下载要显式覆盖；`X-Frame-Options: DENY` 下不能用同源的 iframe 预览 PDF，所以 PDF 在新标签页打开。
- 服务端自己测定类型，不信客户端声明的（Nerve 继承自 Plane 的弱点）；svg、html 这类能执行脚本的类型强制隔离或下载。
- 复制附件之类按 id 的操作先核对读权限，"读不到"与"不存在"答同样的 404（Nerve 的越权读取教训）：M7 的签名只在核对过读权限之后发出。
- 长任务用自己的队列，不占定时任务的 worker。

## 7. Phase 划分

| P | 名称 | 交付 | 验证 |
|---|---|---|---|
| P1 | 平台：存储与流式路由 | `platform/storage`（端口、本地实现、契约测试、启动检查、磁盘余量）；`API.Stream`（次序、按速率的截止时间、`Bounded`、停机时取消、路由的桶）；配置 `storage.*`；镜像的 `/data` 卷、`.gitignore`、README 的挂载与反向代理；e2e 的存储目录 | 存储的契约测试与原子性；流式路由的测试（第 3 节第 2 条）与次序的反向对照；`image-smoke` 的不可写检查 |
| P2 | 附件（服务端） | `x-raw` 的契约规则与代码生成的排除；平台码 `storage_full`；asset 的配置与 `ratelimit.asset_content`（test 配置调到用不完）；page：`TreeWrites`（`CreateAsset`、预检）、读端口、深度只数页面、附件的名称规则、`NodeKind`；asset 模块：`asset_blobs`、上传、类型测定与宽高、下载与响应头（含 PDF 的实测）、元数据与列表、观察者与笔记本删除、清理器、孤儿清扫、活动；`InstanceInfo.asset_max_bytes`；前端：页面树只列页面、树的重读经合并；e2e 的 `deletedDaysAgo` 先挪 `asset_blobs`；`image-smoke` 的附件一步 | 契约测试认 `x-raw`；handler 的表格测试；签名与响应头的表格；整个程序上的删除（三条路径）、清理（含文件已删、结果不明）、活动，组合根交空时失败；交错与权限矩阵；e2e：AS1 的接口版本、AS4、AS5 |
| P3 | 附件与链接（服务端） | 附件进解析；附件的标记与核心的图片钩子；属性链接；改名、移动的改写；落点、补全；附件的 `link`；`PageView.assets_expire_at`；`checkLinks` | `resolve/`、`rename/` 的附件样例与 Obsidian 1.12.7 核对（真的二进制文件，打开"检测所有类型的文件"）；渲染样例加图片与媒体；索引的性质测试与改写的随机测试加附件；`CheckHTML` 的第三种模式；整个程序上的最后一跳 |
| P4 | 附件（前端） | 附件面板（页面与笔记本首页）；上传的服务；编辑器的粘贴、拖入上传；文档的拖放保护；阅读视图的 `assets` 增强（新标签页、到期重读、保留媒体）；输入法清单的粘贴一步 | vitest：面板、上传、扩展经组合根到达编辑器、增强；e2e：AS1 的页面版本、AS2、AS3 |
| P5 | 导出 | `platform/jobs` 的队列与超时、只投递的客户端；`shared.Actor` 的 `JobID`；transfer 模块：`transfer_jobs`、任务的身份、心跳与收拾、数量、文件的清扫；导出（快照、映射、`meta.json`、空页的规则）、导出贡献者、签名的下载、到期清理；前端：导出的对话框、笔记本设置里的任务列表、"导出此页" | 映射的逐项测试；导出的库在 Obsidian 里逐条解析；贡献者的示例测试；收拾与超时；e2e：TR1 |
| P6 | 导入 | page：`CreatePage` 进 `TreeWrites`、`Parse`、变更集的类型与并入；`InstanceInfo.import_max_bytes`；导入的上传、校验（EOCD）、写入（分批、名称在单元里决定、`ANALYZE`）、报告、取消；前端的导入对话框 | 恶意 zip 的表格测试；导出再导入的性质测试；Obsidian 库样例的解析核对；交错；e2e：TR2–TR4 |

收尾：三位 Opus 审查者并行（后端、前端与端到端、完成标准与文档），然后 Opus 核对修复。

### 移交的落实

| 移交 | 项 | 落实 |
|---|---|---|
| [M0/P6 镜像里的附件目录](handoffs/M0-P6-image-volumes.md) | 1 目录与卷、2 可写检查、3 `image-smoke` | 1、2：P1（4.1、4.13）；3：P1 的不可写检查、P2 的附件一步 |
| [M2/P4 只投递的 River 客户端](handoffs/M2-P4-insert-only-client.md) | 1 客户端、2 停机顺序、3 命令行、4 权限 | P5A（4.9）：已落实，命令行不投递 |
| [M2/P4 附件的清理](handoffs/M2-P4-attachment-purge.md) | 1 先删文件、`RESTRICT`、排在父表之前；2 事务边界与"文件删了、行没删"的测试；3 清理器的其余约束 | P2（4.6） |
| [M3 笔记本活动](handoffs/M3-notebook-activity.md) | 1 字节数与最后写入、2 整个程序的测试 | P2（4.6） |
| [M4 附件的扩展与粘贴上传](handoffs/M4-extensions.md) | 1 附件内联（以 M6 的移交第 1 项为准）、2 粘贴上传与 `whenComposed`、3 最后一跳的两个测试、4 先建后删与活动 | 1：P3；2：P4；3：P3（服务端）、P4（编辑器）；4：先建后删见下一行，活动 P2 |
| [M4/P2 一个单元里先建后删](handoffs/M4-P2-unit-merge.md) | — | 不适用：导入只建不删（4.2），由 M9 定下；本次提交以"不适用"关闭，指向 M9 的那份 |
| [M6 链接与附件、导入、导出](handoffs/M6-links.md) | 1 嵌入的渲染由 M7 建立；2 附件进解析；3 落点；4 导入是多操作的单元；5 导出没有正文的页；6 最后一跳 | 1–3、6：P3；4：P6A；5：P5A |
| [M4/P3 Markdown 的扩展](../M6-links/handoffs/M4-P3-markdown-extensions.md)（已关闭，抄送 M7） | 第 1–8 项的注册约束 | P3 照做 |
| [M4/P6 编辑器](../M5-collab-editing/handoffs/M4-P6-editor.md)（已关闭，抄送 M7） | 第 5 项：上传的手段、最后一跳 | P4 |

### 写给后面的 M

由发现它的 Phase 写进目标 M 的 `handoffs/`，收尾时核对（收尾时补齐，[收尾审查](reviews/M7-closeout-review.md) C-I1）：[M8](../M8-history-search/handoffs/M7-assets-transfer.md)、[M9](../M9-mcp/handoffs/M7-assets.md)、[M10](../M10-llm-wiki/handoffs/M7-transfer.md)、[M12](../M12-release/handoffs/M7-transfer.md)（与[打磨](../M12-release/handoffs/M5-polish.md)第 15–20 项、[性能](../M12-release/handoffs/M4-performance.md)第 11–13 项）：

- **M8**：恢复时恢复附件的行（同一个观察者）；清理与恢复的先后（清理器锁行）；导入的整组撤销（一次导入一个变更集，可能很大）；搜索是否列出附件（`nodes.name` 的三元组索引包括它们）。
- **M9**：MCP 的 `read` 读附件；MCP 的客户端要绝对地址，签名地址是相对的，要一个对外的基地址；附件的路径写法。
- **M10**：来源区按 SHA-256 去重；贡献者加的文件记在 `meta.json` 的 `contributed`；来源区页面完整列出附件（总体设计 3.7 的例外）。
- **M12**：负载（一页很多图片、每小时换地址的重新下载）；长时间的导出与导入（导出部分与结束的任务行：[M12 的移交](../M12-release/handoffs/M7-transfer.md)）；慢上传与反向代理；磁盘的监控；名称不是 UTF-8 的 zip（按 GBK、CP437 解读）；总量配额；从解析不到的附件链接直接上传；附件很多时节点树的大小；按人的任务队列上限（[P6A 审查](reviews/P6A-import-review.md) B-9）。

### 总体设计的修订

随这份设计一起改的：0.2 第 2 条、3.5、3.7、3.8、4.4、6.1、6.4、7.2、8.5、9.2、9.3、10.1、11、12.2、12.4、12.6、13.1 第 11、14 条、13.4 第 5 条、14（变更记录第 15 节）。其余的长期约定随实现它们的 Phase 改：

| Phase | 总体设计的条目 |
|---|---|
| P1 | 13.1 第 14 条（`API.Stream` 的规则）、第 15 条（存储的配置）、第 20 条（路由自己的桶）；8.5、11（临时文件在各区的 `.tmp/`） |
| P2 | 13.1 第 1 条（树写入端口）、第 15 条（asset 的配置与交叉规则）、第 5 条（`asset_blobs` 在 `nodes` 之后）、第 6 条（清理器要存储、先删文件）、第 8 条（名称的字段错误）、第 10 条（asset 的日志）、第 21 条（注册者）、第 25 条（签名地址的密钥）、第 28 条（附件的文件名同样经标题键）；13.4 第 4 条（附件的交错）、第 6 条（权限矩阵的新行） |
| P3 | 13.1 第 31 条（一个视图内联的媒体数）；13.3 第 2 条（样例集的附件）、第 4 条（附件的标记）、第 6 条（附件的地址由服务端写） |
| P4 | 13.2 第 1 条（上传不经 `oneAtATime`）、第 6 条（上传经会话的客户端）、第 23 条（`uploadAsset`、`whenComposed`、`assets` 增强）、第 25 条（媒体与面板的上限） |
| P5 | 13.1 第 2 条（任务的客户端与 `Actor.JobID`）、第 5 条（导出的加锁次序）、第 6 条（任务行的清理器）、第 10 条（transfer 的日志）、第 11 条（模块入口）、第 15 条（transfer 的配置）、第 17 条（签名地址的公开读）、第 21 条（注册者）、第 23 条（队列、超时、只投递的客户端、心跳）、第 25 条（导出下载的密钥）；13.4 第 4 条（导出的交错）、第 6 条（权限矩阵的新行）；6.4（导出的下载） |
| P6 | 13.1 第 1、2 条（导入的单元、并入变更集）、第 5 条（任务的创建）、第 16 条（`MAINTAIN`）、第 19 条（单元之前解析）、第 31 条（单元的上限）；13.4 第 4 条（导入的交错） |

## 8. 本 M 建立与注册的扩展点

注册（总体设计 12.4）：

- 软删除的清理注册表：附件的行（排在 page 之前，要存储）、任务的行（排在 notebooks 之前）。
- 笔记本删除事件：附件的行、任务的行。
- 笔记本的活动：附件。
- 页面领域事件的观察者：附件的行跟着节点删除（12.4 这一行加上 M7）。
- Markdown 解析与渲染扩展：附件的标记，经方言的参数。
- 阅读视图的交互增强：`assets`（新标签页、到期重读、保留媒体；12.4 这一行加上 M7）。
- 编辑器扩展管线：粘贴、拖入上传。
- 实时推送的事件类型：不加新类型，不改 `pages` 的载荷；导入导出的进度靠读任务。

建立：

- **附件嵌入的渲染**（`obsidian.Options` 的 `Assets` 与 `Resolve` 带类型，核心的图片钩子）：建立并注册，P3；整个程序上的最后一跳，组合根交空时解析到附件的写法渲染为文字、测试失败。
- **导出贡献者**：建立（M10 注册），P5A；组合交空，模块根的示例测试在真库与 River 上（次序、第一个错误、冲突、超时）。

改前面 M 的扩展点与代码（12.1 第 6 条的例外，逐项写明）：

- page：`(*page.Module).TreeWrites()`（接好线的模块给别的模块的端口，13.1 第 11 条的例外）；`UnitSpec.Kind`、`UnitSpec.Changeset` 与并入的核对；`changesets.kind` 加 `import`；深度只数页面（`Subtree.Height`、`checkPages`、前端的拖动）；附件的名称规则；`NodeKind`；读端口。
- `shared.Actor` 加 `JobID`（第三种凭据）。
- `platform/httpserver`：`API.Stream` 与按路由的桶（`APIConfig` 加字段）；平台码 `storage_full`。
- `platform/jobs`：队列与超时可配、`Job.Start`、只投递的客户端（含 `Unfinished`）；`platform/postgres`：`TxFrom`、`WithinSnapshot`（快照里拒绝 `WithinTx`）；`platform/storage`：写入途中每 64 MiB 看一次余量。
- `platform/markdown`：核心的图片钩子（M4 的 `Extension` 加字段）；`obsidian.Resolve` 的签名（答类型）与 `Options.Assets`；`CheckHTML` 认附件的标记。
- linking：`LinkTargetKind`、`PropertyLink.kind`、落点的 `target_is_asset`、`Linktexts` 的 `.md` 写法、补全数据带类型；`markdownExtensions(resolve, assets)`。
- 组合根：`purgers(pool)` 改为 `purgers(pool, tx, store, logger)`；模块的构建次序 page → asset → transfer。
- 前端：`EditorContext.uploadAsset`、`EditorControls.whenComposed`；`PageView.assets_expire_at` 与阅读视图对缓存的视图的处理；M5 的 `pages` 处理的整树重读改经 `refresher`；`appLinks` 不碰附件的链接（`nw-asset`）；`InstanceInfo`；`PageTreeStore` 分出页面的索引；附件的上传不经 `oneAtATime`（13.2 第 1 条的例外）；右栏的属性对附件给地址。

## 9. 测试策略

- **存储**：契约测试（`storagetest`）；本地实现另测原子性（`Commit` 之前不可见、`Abort` 不留文件、进程中途退出不留半个文件）、启动检查（不可写、残留的临时文件）与余量。
- **流式路由**：慢而达到速率的上传完成、停住的在 `read_timeout` 加已读字节应得的时间之内断开、限速下的大文件下载写完、读完之后的期限、停机时取消；次序的反向对照（认证与桶对调等）；签名的下载不消耗 `anonymous`。
- **上传与下载**：handler 层的表格测试（字段次序、未知与重复的字段、缺字段、上限、类型测定的容器表、预检的每个码）；签名的伪造、过期、改每一个参数、多出与重复的参数；已知答案的密钥；响应头的表格；`Range` 与条件请求；提交结果不明时文件留着。
- **整个程序**（总体设计 13.1 第 21 条，组合根交空时失败）：删除子树与笔记本删除的三条路径之后行被软删除；清理先删文件；活动；附件标记的最后一跳；导入经观察者进索引；导出贡献者到达导出。
- **并发与权限**：4.14 的交错，结束时核对不变式；权限矩阵的新格；运行时角色的测试跑一次导入（`MAINTAIN`）。
- **链接**：样例与 Obsidian 核对（`resolve/`、`rename/` 的格式加 `assets`，核对脚本建真的二进制文件、打开"检测所有类型的文件"）；渲染样例加图片与媒体的记法；`TestTheAppsMarkdownRendersCheckedHTML` 加"全部解析到附件"的模式（每种类型，真实的签名地址）；M6 的性质测试与改写的随机测试加上附件的操作。
- **导入导出**：映射的逐项测试；导出的库经 `verify-resolve` 在 Obsidian 里逐条解析；恶意 zip 的表格测试（含 EOCD 谎报、NFD 的名称、`\` 分隔）；导出再导入的性质测试（随机的树、正文、附件、次序）；一个 Obsidian 库的样例；取消、中途失败、超时、服务重启的报告与收拾。
- **e2e**（新的两组，页面版本与 PAT 的接口版本，总体设计 12.5；数据库断言放在 `e2e/fixtures/assert/asset.ts`、`transfer.ts`，13.4 第 3 条）：
  - AS1：上传、列出、下载（P2 接口；P4 页面：面板上传与打开，根下的附件在笔记本首页）。
  - AS2：编辑器里粘贴、拖入图片，插入嵌入并显示（接口版本：上传，再写带 `![[link]]` 的正文，同一组数据库断言）。
  - AS3：嵌入的图片、音频、视频（WebM、Ogg：Playwright 的 Chromium 没有 H.264、AAC）、附件链接显示；附件改名之后嵌入改写；未解析的嵌入；`expectIndexedLinks` 带附件的 id（M6 的移交第 6 项）。
  - AS4：删除子树、删除笔记本之后附件的行与文件被清理（`deletedDaysAgo` 先挪 `asset_blobs`，否则 `RESTRICT` 让已有的清理故事停住）；活动算上附件。页面版本（收尾时补上，[收尾审查](reviews/M7-closeout-review.md) C-I4）：左栏删除一页时确认框数出它下面的附件，删掉之后它们的行随节点删除、地址读不到；设置里删除笔记本，其余的随笔记本删除；无主列表的大小与最后活动算上附件。清理是后台任务的，只有接口版本。
  - AS5：svg 在浏览器里不在本站的源里执行、不向外站请求（声明 sandbox 的 "Blocked script execution" 控制台消息）；html 附件被下载（`waitForEvent("download")`）。接口版本核对各类型的响应头；浏览器里的执行只有页面版本能证明（10.1 的例外）。
  - TR1：导出笔记本与子树，zip 的结构与 `meta.json`（zip 的读取是 e2e 自己的 `fixtures/zip.ts`，不加依赖）。
  - TR2：导入 zip，进度、报告，导入的页与附件，链接解析。进度只核对结束时的值（接口版本的 `done/total`）：几十个条目的库在一秒之内导完；中途的进度由 transfer 的单元测试核对（两批之后取消时是 200/250，`transfer/app/import_test.go`；[P6 文档](06-P6-import.md) 9.2 第 10 项；[收尾审查](reviews/M7-closeout-review.md) B-M8）。
  - TR3：导出再导入得到同样的树，只有一处例外：导出时改了名的文件夹导入之后是这个名称的页：页 `Plan` 写成文件 `Plan.md`，名为 `Plan.md` 的页的子页所在的文件夹因此写成 `Plan.md 2`，导入之后那一页名为 `Plan.md 2`（收尾审查 B-N2）。
  - TR4：恶意的 zip 被拒绝或跳过，报告写明。
- **人工**：输入法清单加两步（P4，第 19、20 步）："组合输入时上传完成：嵌入在 `compositionend` 之后插在映射后的位置，组合出的文字完好"；浏览器"复制图片"、访达复制文件与文件夹之后粘贴，拖入之后取消（三种浏览器）。

## 10. 风险

| 风险 | 缓解 |
|---|---|
| 磁盘写满：没有总量配额 | 单个文件与导入包有上限；同时的任务有数量上限；写入之前看余量，答 507 `storage_full`；README 写明监控磁盘。配额留到 v0.1 之后（M12 的移交） |
| 失去访问之后，已签出的地址仍能读（至多 2 小时） | 签名只在核对过读权限之后发出；到期短；写进安全说明。要立刻撤销时，轮换签名私钥（所有地址失效） |
| 签名地址进了反向代理的访问日志 | 短期有效；README 写明；是"秘密不进地址"的例外（4.14） |
| 每小时换地址，图片每小时重新下载一次 | 接受（同一小时之内有缓存）；M12 实测负载 |
| 浏览器把附件当作本站的页面执行脚本、向外站请求 | 服务端测定类型；每个附件答复带 `sandbox` 与 `default-src 'none'` 的 CSP；非白名单的强制下载；`nosniff`；e2e 在浏览器里证明 |
| 导入长时间持有笔记本的锁 | 小批量的单元（节点、正文、链接各有上限）；批与批之间放开锁；单元之前解析，不在锁里排队 |
| 压缩炸弹、越出目录、海量条目、谎报的中央目录 | 先读 EOCD；先校验后写入；按实际读出的字节计；上限可配 |
| 服务重启、超时时导入导出被打断 | 不自动重试；心跳与启动时的收拾记成失败并写报告；导入的部分照常可见、可删 |
| 文件与行不一致（崩溃、手工改动卷、提交结果不明） | 先写文件后提交、先删文件后删行；结果不明时不删；孤儿清扫；下载时文件不在答 404 并记日志 |
| 反向代理限制了请求体或超时、缓冲了上传 | README 写明 `client_max_body_size`、上传路由不缓冲、超时；网页在发送之前先查，提前的拒绝不靠答复 |
| 旧版 Windows 打的 zip 名称不是 UTF-8 | 跳过并写进报告（M12 的移交） |

## 11. 负责人确认的决定

M7 开工时负责人确认进入 M7（2026-10-08："可以了"）。下面是设计里的取舍，负责人可以改判：

1. 附件不算一层：深度 10 的页面下仍可以挂附件。
2. 能读的人都能导出整个笔记本。
3. 导入时不合规的名称被修正（并写进报告），而不是跳过。
4. 只有子页、没有正文、但被链接到的页，导出时另写一个空的 `.md`。
5. 附件改名、移动时改写指向它的链接（照 Obsidian）；附件改名不能去掉扩展名。
6. 签名地址有效 1–2 小时；失去访问之后仍能读到到期；每小时换一次地址。
7. 不做总量配额、不做命令行的导入导出；每本笔记本同时一个导入，每人每本笔记本同时一个导出、只留最新的一份。
8. PDF 内联在新标签页打开；`sandbox` 下能否用由 P2 的实测定。
9. 上传 `.md` 被拒绝，网页提示用导入或新建页面（总体设计 3.7 原写"按页面处理"）。
10. 附件面板照总体设计 9.2 在中栏的子页面列表之下；根下的附件在笔记本首页（初稿写在右栏）。
11. 导出的 zip 有一层以笔记本（或页）命名的根目录（照 3.3）；导入时只有一个顶层目录、其中有 `.obsidian/` 或 `.nerve/` 的，去掉这一层。
12. 粘贴的图片名照 Obsidian："Pasted image 20261008123045.png"，不随界面语言。
13. 没有扩展名的附件可以上传与导入，但链接解析不到（与 Obsidian 相同）。
14. 点图片不做什么（与 Obsidian 的默认相同）。
15. 名称不是 UTF-8 的 zip 条目跳过并写进报告。
16. 解析不到的 `[[报告.pdf]]` 照 M6 提议新建页面"报告.pdf"（Obsidian 建 `报告.pdf.md`）。
17. 发起人的凭据失效（退出登录、撤销 PAT）不打断已开始的任务；失去写权限、账户停用会打断。
18. 笔记本管理员看得到这本笔记本的全部任务，别人只看自己的；看不到笔记本的人看不到自己的任务。
19. Notion 等其他 Markdown 的 zip 按同样的映射导入，不另做适配、不作完成标准。
20. 改名嵌在自己正在编辑的页里的附件，同样因自己的锁被拒（M6 的规则，总体设计 3.5）。
21. M7 加的输入法步骤与 M4–M6 一样：收尾之后等负责人执行完整份清单，M7 才改为已完成。

## 12. Phase 进度表

| P | 名称 | 状态 | Phase 文档 | 审查 |
|---|---|---|---|---|
| P1 | 平台：存储与流式路由 | 已完成（合并 `d7af7cf`） | [01-P1-storage-stream.md](01-P1-storage-stream.md) | [P1 审查](reviews/P1-storage-stream-review.md) |
| P2 | 附件（服务端） | 已完成（合并 `48c62c0`） | [02-P2-assets-server.md](02-P2-assets-server.md) | [P2 审查](reviews/P2-assets-server-review.md) |
| P3 | 附件与链接（服务端） | 已完成（A 合并 `5138ad6`，B 合并 `f3bf03c`） | [03-P3-assets-links.md](03-P3-assets-links.md) | [P3A 审查](reviews/P3A-assets-links-review.md)、[P3B 审查](reviews/P3B-render-review.md) |
| P4 | 附件（前端） | 已完成（A 合并 `e44b417`，B 合并 `32e175c`，C 合并 `008f81f`） | [04-P4-assets-web.md](04-P4-assets-web.md) | [P4A 审查](reviews/P4A-assets-web-review.md)、[P4B 审查](reviews/P4B-assets-web-review.md)、[P4C 审查](reviews/P4C-paste-upload-review.md) |
| P5 | 导出 | 已完成（A 合并 `e8f02d5`，B 合并 `ab4562a`） | [05-P5-export.md](05-P5-export.md) | [P5A 审查](reviews/P5A-export-review.md)、[P5B 审查](reviews/P5B-export-web-review.md) |
| P6 | 导入 | 已完成（A 合并 `b60cf66`，B 合并 `c9a48dd`） | [06-P6-import.md](06-P6-import.md) | [P6A 审查](reviews/P6A-import-review.md)、[P6B 审查](reviews/P6B-import-web-review.md) |

## 13. 变更记录

| 日期 | 变更 | 依据 |
|---|---|---|
| 2026-10-08 | 初稿（`293cdf0`） | 总体设计 12.2；7 份移交；对照 `629f741` 的代码复核 |
| 2026-10-08 | 按设计审查修订：树写入端口（接好线的 page 模块给出）、平台的流式路由、提交结果不明时不删文件、任务的表与生命周期、心跳与超时、数量与磁盘余量、EOCD、附件答复的 CSP；附件的一种标记与核心的图片钩子、解析的写法规则、根下的附件与面板的位置、上传经会话的客户端、到期与媒体的保留；12.1 第 6 条的例外逐项列出；拆成六个 Phase | [设计审查记录](reviews/M7-design-review.md) |
| 2026-10-08 | P1 完成：4.1 的启动检查（先删掉全部残留、再探测根与各区；写满照常启动、写入答 507；断链的区拒绝启动）；4.3 的流式路由（低速率时更小的一步、没有请求体不设读截止时间、停机只切断还在传字节的流、`ErrShuttingDown`）；4.4 的 multipart 读到结尾；4.5 停机之后的下载答 503；第 7 节 P2 的交付与验证补项、修订表补 P1 的配置 | P1 的实施、审查与五轮修复核对：[01-P1-storage-stream.md](01-P1-storage-stream.md)、[P1 审查](reviews/P1-storage-stream-review.md) |
| 2026-10-08 | P2 开工：第 7 节 P2 的 `InstanceInfo` 只加 `asset_max_bytes`，`import_max_bytes` 随 P6 | [02-P2-assets-server.md](02-P2-assets-server.md) 第 2 节 |
| 2026-10-09 | P2 完成：4.3 处理器之前的答复对有请求体的请求关闭连接，宣告的答复按步写出、写截止时间随写出的字节前移（`Sending(r)`）；4.4 上传的 `name` 为空取文件名；4.5 严格的读法（规范的路径，参数依次、键名对位），CSP 与 CORP 在每个答复上，去掉对改动的条件；4.6 活动只报字节数；4.8 树的重读至多每 500 毫秒一次 | P2 的实施、审查与九轮修复核对：[02-P2-assets-server.md](02-P2-assets-server.md)、[P2 审查](reviews/P2-assets-server-review.md) |
| 2026-10-09 | P3 开工：分 A（解析、索引与改写）、B（渲染）两部分合并；与 Obsidian 1.12.7 实测之后定下附件的三种读法（带扩展名、笔记本里有这个名称的附件时只读作附件），被同名附件遮住的页、被抢走的页的 wikilink 写 `.md` 的写法（Obsidian 写出解析不到的，`nerve-defined`），附件的显示文字按去掉扩展名的名称跟着改；索引记下解析到的是附件（`page_links.resolved_asset`）；4.7 的 `Assets` 由 asset 只凭连接池给出（`asset.NewEmbeds`）；平台的 `Render` 答 `View`（带到期） | [03-P3-assets-links.md](03-P3-assets-links.md) 第 2、4、5 节 |
| 2026-10-09 | P3A 完成：没有扩展名的附件 `link` 为 null、补全不列它；附件的 `link` 由 page 的一条语句读出路径与同名附件的个数（`AssetLinktext`）；落点在同名附件旁边也答 `target_is_asset`；down 迁移先把指向附件的链接置为解析不到；契约写明附件的改名、移动会改写、会被编辑锁拒绝 | P3A 的实施、审查与两轮修复核对：[03-P3-assets-links.md](03-P3-assets-links.md) 第 4、9 节、[P3A 审查](reviews/P3A-assets-links-review.md) |
| 2026-10-09 | P3B 完成：附件的标记不写 `data-nw-asset`（id 在地址的路径里），`alt` 与 `aria-label` 默认是写下的目标，`Assets` 不给名称；一个视图至多写 2000 个附件的地址（`MaxShown`），附件的标记有了总量的上界；视图的到期是写出的地址里最早的；`PropertyLink` 另加 `url` | P3B 的实施、审查与两轮修复核对：[03-P3-assets-links.md](03-P3-assets-links.md) 第 5、9 节、[P3B 审查](reviews/P3B-render-review.md) |
| 2026-10-09 | P4 开工：分 A（面板与上传）、B（阅读视图里的附件）、C（编辑器的粘贴与拖入）三部分合并；B 里服务端给不内联的附件的链接写 `download`，`PropertyLink` 加 `inline`、`PageProperties` 加 `assets_expire_at`（右栏的附件链接分开打开与下载、到期重读，P3B 审查 C12、C13）；面板不加载缩略图 | [04-P4-assets-web.md](04-P4-assets-web.md) 第 0、2、4 节 |
| 2026-10-09 | P4A 完成：上传的请求不带正文、表单由注入的传输发出；上传答复之后的树读合并（`wrote()`），被重叠的读交回最近发出的那次；列表重读读已有的页数、接页按 id 去重，成功的上传读到它所在的页；附件在树读完、显示之后再读；名称按 `titleKey` 比较；拖放保护不管编辑器，页内开始的拖动不上传；附件一节的播报、复制的退路与焦点的交接 | P4A 的实施、审查与四轮修复核对：[04-P4-assets-web.md](04-P4-assets-web.md) 第 3、9 节、[P4A 审查](reviews/P4A-assets-web-review.md) |
| 2026-10-09 | P4B 完成：不内联的附件的链接写 `download`（12 字节）；`PropertyLink.inline`、`PageProperties.assets_expire_at`；阅读视图的增强 `assets`：新标签页与提示、链接之后按语言写大小、加载失败的重读、保留音视频与就地重签（开始了的都签，重签途中的失败不理会，一分钟之内不再签，地址没给时按视图的过期重读；出错的不保留）；到期重读从读到的时刻算（`stamped`、`eachRead`），快一小时以上的时钟每 30 秒，隐藏的标签页显示时读；4.8 的阅读视图随之改写 | P4B 的实施、审查与五轮修复核对：[04-P4-assets-web.md](04-P4-assets-web.md) 第 4、9 节、[P4B 审查](reviews/P4B-assets-web-review.md) |
| 2026-10-09 | P4C 完成，P4 完成：编辑器的扩展 `assetUpload`（粘贴、拖入的文件上传，插入 `![[link]]`；光标移到嵌入之后，同一位置后粘贴的在后面；每个上传一创建就接住拒绝；没插入的一批答完说一次）；`EditorContext.uploadAsset`、`EditorControls.whenComposed`、`tell`、`going`，`SourceEditorHandle.working`、`settled`；Done、Mod+E 先等编辑器的上传与其后的组合，等待中失锁或撞上冲突就不走，等待期间说的带到阅读视图，离开之后的焦点按“编辑之内与之外”；上传行按来源分开（`Upload.fromEditor`）；正文至少 20rem 高；能改的正文有落点的光标；4.8 的粘贴、拖入随之改写 | P4C 的实施、审查与七轮修复核对：[04-P4-assets-web.md](04-P4-assets-web.md) 第 5、9 节、[P4C 审查](reviews/P4C-paste-upload-review.md) |
| 2026-10-09 | P5 开工：分 A（服务端）、B（前端）两部分合并；子树的导出里那一页是库的根下的一页（`<页>/<页>.md`）；`meta.json` 里只有目录的页的路径以 `/` 结尾；冲突时有目录的那一页改名（`N.md 2`）；任务表加 `name`（导出的根名：任务列表与下载的文件名）；`jobs.Job.Start` 承担启动时的收拾；心跳每秒一次、同一条语句读回取消；列表不带报告的问题 | [05-P5-export.md](05-P5-export.md) 第 0、3 节 |
| 2026-10-09 | P5A 完成：导出的服务端照实际改写（[05-P5-export.md](05-P5-export.md) 第 3 节）：River 在开始之前丢掉的排队导出由收拾记成失败（只投递的客户端读 River 还没结束的任务），收拾的报告计数为 0；成功的事务先锁笔记本行；正文按 200 页或 16 MiB 一批，本地存储的写入每 64 MiB 看一次余量；下载与视图按 TTL，地址的期限不晚于导出的到期；结束限时停机 900 毫秒、其余 30 秒；`jobs.export_workers` 至多 `database.max_conns` 的一半；4.9 的心跳与收拾、4.10 的地址随之改 | P5A 的实施、审查与两轮修复核对：[05-P5-export.md](05-P5-export.md) 第 3、9 节、[P5A 审查](reviews/P5A-export-review.md) |
| 2026-10-09 | P5B 开工：实例信息加 `export_ttl_seconds`（确认对话框说成功的导出保留多久）；任务列表在有进行中的任务时每秒读，否则在最早的下载地址到期之前一分钟再读；页面标题旁的菜单（读者也有）只有"导出此页"，开始之后到设置、焦点在新任务那一行 | [05-P5-export.md](05-P5-export.md) 第 4 节 |
| 2026-10-10 | P5B 完成：导出的前端照实际改写（[05-P5-export.md](05-P5-export.md) 第 4 节）：任务列表的重读照反链读回已加载的页数，读到之前与读不到时说明；行与控件以做什么、发起人与开始的时刻命名；控件带着焦点离开时交给行；轮询至多一小时；`ConfirmDialog` 按触发按钮打开时成功之后复位；实例信息的 `export_ttl_seconds` 进镜像的冒烟 | P5B 的实施、审查与两轮修复核对：[05-P5-export.md](05-P5-export.md) 第 4、9 节、[P5B 审查](reviews/P5B-export-web-review.md) |
| 2026-10-10 | P6A 完成：导入的服务端照实际改写（[06-P6-import.md](06-P6-import.md) 第 3 节）：序号跳过同一个库里后面的兄弟原样的名称；正在上传的导入在进程内计数（一本笔记本一个，算进队列与存储，导出也看它），声明的长度过大时立即 413；导入的报告随心跳写、收拾保留；`import_max_entries` 至多 100,000，`import_max_bytes` 按最低速率在 3 小时之内传完；4.9 的收拾与数量、4.11 的接口、校验、名称、写入、取消与报告随之改 | P6A 的实施、审查与六轮修复核对：[06-P6-import.md](06-P6-import.md) 第 3、9 节、[P6A 审查](reviews/P6A-import-review.md) |
| 2026-10-10 | P6B 开工：导入一节与对话框照 A 的实际修订（[06-P6-import.md](06-P6-import.md) 第 4 节）：报告、失败与问题的文案按任务的种类；`transfer.busy` 的通用文案不分种类，导出与导入的对话框各用自己的；自己的导入进行中时对话框先说明、不发送；上传属于对话框，卸载即中止；读文件之前的拒绝到浏览器可能只是连接中断，对话框说明可能的原因、不重试 | [06-P6-import.md](06-P6-import.md) 第 4 节；P6A 的交接（第 9.1 节） |
| 2026-10-10 | P6B 完成：导入的前端照实际改写（[06-P6-import.md](06-P6-import.md) 第 4 节）：上传期间在应用里离开同样先问，询问只在上传期间；选的位置在重读的树里消失时说明、不发送；只列还能再放一层的页；进行中的导入按列表里看得到的判断（开工时写的是"自己的"），这时"导入"仍可聚焦、什么都不发；每次结束都重读任务列表；Chromium 读到读文件之前答的 409；`server_busy`、`payload_too_large`、`bad_request` 用导入自己的文案；`tree_changed` 只说删除（契约随之）；编辑器上传文件夹与 `.md` 的提示指到导入 | P6B 的实施、审查与四轮修复核对：[06-P6-import.md](06-P6-import.md) 第 4、9 节、[P6B 审查](reviews/P6B-import-web-review.md) |
