# M7 附件与导入导出：总设计

| 项 | 内容 |
|---|---|
| 里程碑 | M7 附件与导入导出（`M7-assets-transfer`） |
| 日期 | 2026-10-08 |
| 状态 | 进行中 |
| 依赖 | M4、M5、M6 |
| 上级文档 | [v0.1 总体设计](../v0.1-design.md) 第 3.5、3.7、3.8、3.10、4.4、4.5、6.4、7.2、8.5、11、12.4、13 节 |

---

## 1. 目标

1. **附件**：页面下可以挂文件。网页上从附件面板上传，或在编辑器里粘贴、拖入；附件按 Obsidian 的写法嵌入（`![[架构图.png]]`、`![说明](架构图.png)`），图片、音频、视频在阅读视图里内联显示，其他类型显示为可下载的链接。
2. **安全地下发**：附件经短期签名地址读取（`<img>` 带不了 Bearer 令牌），响应头让可能执行脚本的类型不能在本站的源里运行。
3. **附件是节点**：与页面共用命名空间、回收站与清理；改名、移动时，指向它的链接照页面的规则改写；删除到期之后文件随之删除。
4. **导出**：把一个笔记本或一棵子树导出为 zip，结构与 Obsidian 的库一致（总体设计 3.5 的导出映射），附 `.nerve/meta.json`。
5. **导入**：把 zip（Obsidian 的库、Notion 导出的 Markdown、本系统的导出）导入到一个笔记本的某个位置，后台运行，有进度与报告；导入的页进链接索引，导出再导入得到同样的树。
6. **部署**：附件目录是镜像的卷，不可写时拒绝启动；备份照总体设计第 11 节先数据库、后附件目录。

## 2. 范围

**做**：

- 平台的存储端口 `platform/storage` 与本地磁盘实现。
- `asset` 模块：附件的元数据（`asset_blobs`）、上传、签名的下载、响应头、跟着节点删除、清理文件、孤儿文件的清扫、笔记本的活动。
- page 模块：附件节点的新建（写入单元的新操作）、改动带上节点类型、深度只数页面、变更集的类型（`import`）、接口的 `NodeKind` 加上 `asset`。
- 链接：附件进解析；附件嵌入的渲染（M7 建立的扩展点）；Markdown 图片指向附件时内联；改名、移动附件时改写链接；落点拒绝附件的目标。
- 前端：页面树只列页面；附件面板；编辑器的粘贴、拖入上传（编辑器扩展）；阅读视图里的附件。
- `transfer` 模块：导入导出的任务（`transfer_jobs`）、只投递的 River 客户端与自己的队列、导出、导入、报告、取消、到期清理；导出贡献者（M7 建立的扩展点）。
- 部署：镜像的卷与属主、启动时的可写检查、`image-smoke` 的附件一步、README 的挂载与备份。

**不做**：

- 对象存储（S3 等）：端口留着，v0.1 只有本地磁盘（单实例，总体设计第 11 节）。
- 替换附件的内容：附件写入后不变；要换就删了再传。
- 配额：不限每个工作区或笔记本的总量，只有单个文件与导入包的上限（第 10 节的风险）。
- 页面嵌入的展开（`![[页面]]` 的转写）：照 M6 显示为链接。
- 附件的版本、回收站里的恢复（恢复是 M8）。
- 命令行的导入导出：只经接口（M2/P4 的移交第 3 项；组合规则不改）。
- 导入时改写链接：正文逐字节保留，链接按名称解析（导入的结构与原库一致时自然解析到）。
- 预览 Office 文档、给图片生成缩略图。

## 3. 完成标准

总体设计 12.5 的通用标准之外：

1. **存储端口**：本地实现过端口的契约测试（写入的原子性、半途失败不留可见的文件、按键读、删除不存在的键算成功、按前缀与时间列出）；附件目录不可写时 `serve` 拒绝启动，错误里写明 uid。
2. **上传与下载**：
   - 上传流式写入、边写边算 SHA-256，超过上限立即 413，慢速但达到最低速率的上传能完成，不发正文的连接按时断开；
   - 签名地址的伪造、过期、改动任何一个参数都答 404；同一小时里签出的地址相同；
   - 每种类型的响应头有表格测试（内联的、带 `sandbox` 的、按附件下载的），e2e 在浏览器里证明 svg 与 html 不在本站的源里执行。
3. **删除与清理**：删除子树、删除笔记本时附件的行随之软删除；清理先删文件、后删行，"文件删了、行没删"之后的下一次运行能完成；孤儿文件被清扫；附件计入笔记本的活动。每条路径都有整个程序上的行为测试，组合根不登记时失败（总体设计 13.1 第 21 条）。
4. **链接**：
   - 附件的解析与改写有样例（`resolve/`、`rename/`），与 Obsidian 1.12.7 核对；
   - 附件嵌入的渲染、Markdown 图片指向附件，有渲染测试与样例（`render/`、`cases/`）；
   - M6 的性质测试（增量维护的索引等于重建）加上附件的新建、改名、移动、删除；
   - 不变式 `checkLinks` 认附件。
5. **导出与导入**：
   - 导出的 zip 与总体设计 3.5 的映射逐项核对；
   - 导出再导入得到同样的树、正文与附件（性质测试：随机的树）；
   - 一个 Obsidian 库的样例导入之后，链接与 Obsidian 解析的一致；
   - 恶意的 zip（越出目录、符号链接、压缩炸弹、坏的名称、过深、过多）都被拒绝或跳过并写进报告，不留下文件；
   - 导入中途失败或取消时，报告写明已导入的部分。
6. **前端**：附件面板、粘贴与拖入上传、阅读视图的附件、导入导出的对话框各有 vitest；e2e 的 AS、TR 系列故事（第 9 节）通过；粘贴上传的输入法步骤加进清单。
7. **部署**：`make image-smoke` 上传一个附件、重启容器之后读出同样的字节。

## 4. 关键决定

### 4.1 模块与端口

| 位置 | 内容 |
|---|---|
| `platform/storage` | 存储端口 `Store` 与本地磁盘实现。只认键与字节，不知道附件、笔记本 |
| `modules/asset` | `asset_blobs`、上传、签名的下载与响应头、跟着节点删除、清理、孤儿清扫、笔记本的活动、嵌入渲染要的附件信息 |
| `modules/transfer` | `transfer_jobs`、导出、导入、报告、取消、到期清理、导出贡献者 |
| `modules/page` | 附件节点的新建、改动带类型、深度、变更集的类型；节点仍只在这里 |

- 模块之间照旧不互相导入：asset、transfer 要的 page 的能力，由 page 的模块根给出（照 `page.NewLinkTargets`），组合根接线。
- **存储端口**（`platform/storage`）：

  ```go
  type Store interface {
      Create(ctx context.Context, key string) (Writer, error) // 写到临时文件；Commit 才可见
      Open(ctx context.Context, key string) (File, error)     // io.ReadSeekCloser + Size、ModTime
      Delete(ctx context.Context, key string) error           // 不存在算成功
      List(ctx context.Context, prefix string, before time.Time, each func(key string) error) error
  }
  type Writer interface { io.Writer; Commit() error; Abort() error }
  ```

  - 键由调用方给：`blobs/<blob id>`、`exports/<job id>.zip`、`imports/<job id>.zip`。本地实现按键的前两段散开目录（`blobs/01/9a/<id>`），避免一个目录里几十万个文件。
  - 写入：在根下的 `tmp/` 里建临时文件，`Commit` 时 `fsync`、`rename` 到位、`fsync` 目录；`Abort` 删掉临时文件。读者只看得到完整的文件。
  - 启动检查：根目录不存在就建；建一个探测文件再删掉，不可写时返回带 uid 的错误，`serve` 拒绝启动（M0/P6 的移交）。
  - 契约测试放在 `platform/storage/storagetest`，本地实现与以后的实现都跑它。

### 4.2 附件节点（page 模块的改动）

- **新建**：写入单元加一个操作 `Unit.CreateAsset(parent, name)`：节点 `kind = asset`，没有正文、没有版本；父节点必须是页面（`lineOf` 已经如此），名称与页面共用命名空间（`titleFree` 已经如此）。page 的模块根给出 `page.NewAssetNodes(writer)`，供 asset 模块在一个单元里建节点、并在同一个事务里写自己的行（回调 `after(ctx, node)`，在单元的事务里运行，照参与者）。
- **名称**：照 `shared.CheckTitle`，另加：附件的名称不能以 `.md` 结尾（不分大小写；总体设计 3.7："`.md` 文件总是页面"），新建与改名都查（新的码 `node.markdown_asset_name`）。改名可以改扩展名：下发的类型来自上传时测定的 MIME，不看名称。
- **改动带类型**：`domain.Change` 加 `Kind`。观察者据此区分：M5 的推送把类型带给前端（页面树只列页面）；M6 的索引把附件当作链接的目标（没有正文，不提取）；asset 的观察者跟着删除（4.5）。
- **深度只数页面**：附件不能有子节点，它不算一层。`Subtree.Height()` 与移动、新建的深度检查只数页面：深度 10 的页面下仍可以挂附件。（总体设计 3.5 随之写明。）
- **接口**：`NodeKind` 加上 `asset`（`api/modules/page.yaml`）；`ListNodes` 照旧返回全部节点。读正文、编辑会话、勾选任务这些接口对附件仍答 `page.not_found`（现在已如此）。
- **变更集的类型**：`changesets.kind` 加上 `import`（总体设计 3.8 的"导入"），`UnitSpec` 加 `Kind`，默认 `edit`。附件的上传是普通的单次写入（`edit`）。
- **一个单元里先建后删**（M4/P2 的移交）：M7 的导入只建不删，不会遇到；由 M9 的 batch 定下（M7 的这份移交以"不适用"关闭，指向 M9 的那份）。

### 4.3 上传

- 接口：`POST /api/v0/notebooks/{notebook_id}/assets`，`multipart/form-data`。字段依次是 `parent_id`（可选，没有就挂在根下）、`name`（可选，没有就用文件部分的文件名）、`file`（最后一个）。答 201 和附件（节点的字段，加 MIME、字节数、SHA-256、签名地址）。
- **流式**：用 `mime/multipart.Reader` 逐部分读，不用 `ParseMultipartForm`（它会把大文件落到系统临时目录）；文件之前的字段不超过 4 KiB。
- **次序**：
  1. 认证之后先做不加锁的预检：笔记本可写（角色）、父节点是这个笔记本里活着的页面、名称合法且此刻没被占用。不通过就在读文件之前答 403、404、409、422，不让人白传 50 MB。
  2. 把文件流进 `Store.Create("blobs/<新 blob id>")`：边写边算 SHA-256，记下前 512 字节用来测定类型；超过 `asset.max_bytes`（默认 50 MiB）立即中止，答 413 `payload_too_large`。
  3. `Commit` 之后开写入单元：`CreateAsset` 加 asset 的行（同一个事务）。预检之后被抢先占用的名称在这里答 409。
  4. 单元失败时删掉刚写的文件（尽力而为；漏掉的由孤儿清扫收拾，4.5）。
- **时间**：服务的 `read_timeout` 管整个请求体，50 MB 在慢网上读不完。上传的处理用 `http.ResponseController` 把读的截止时间放宽到 `read_timeout + asset.max_bytes / asset.upload_min_rate`（最低速率默认 64 KiB/s，约 13 分钟）；不发正文的连接仍按原来的时间断开。路由的请求体上限是 `asset.max_bytes` 加 64 KiB（`BodyLimits`）。body 的形状检查只读 JSON，上传不在它的表里。
- **类型的测定**：`http.DetectContentType` 嗅探前 512 字节，再看扩展名（自己的一张表，不读系统的 `mime.types`）：
  - 扩展名的类型在内联的白名单里（4.4），且嗅探的结果与它同类或嗅探不出（`application/octet-stream`、`text/plain`；svg 嗅出 `text/xml`）→ 用扩展名的类型；
  - 扩展名不认识、嗅探的结果在白名单里 → 用嗅探的；
  - 其余 → `application/octet-stream`（按附件下载）。嗅探出 HTML 的一律不内联。
  - 测定的类型存在行里，下发时只用它，不看客户端声明的类型，也不看之后改过的名称。
- **重名**：接口照旧答 409，不静默改名；网页在已加载的子节点上先取一个空着的名字（照"未命名 2"的规则）。

### 4.4 下载：签名地址与响应头

- **地址**：`GET /api/v0/assets/{node_id}/content?b=<blob id>&e=<到期的 Unix 秒>&s=<签名>`，加 `&d=1` 时按附件下载。公开的操作（`security: []`），签名就是凭据。
- **签名**：`HMAC-SHA256(key, "asset-content" ‖ node id ‖ blob id ‖ e ‖ d)` 截成 16 字节，base64url。密钥由组合根派生：`SigningKeys.Derive("nervewiki asset-content mac v1")`（总体设计 13.1 第 25 条），有钉住它的测试。
- **到期**：签名时取 `e = (⌊now / 1 小时⌋ + 2) × 1 小时`：地址在 1 到 2 小时之间有效，同一个小时里签出的地址相同，浏览器的缓存因此有用（总体设计 6.4）。
- **校验**：签名对、未到期、行存在且没有软删除、节点与 blob 对得上，才下发；任何一项不对都答 404（不区分"过期"与"不存在"）。
- **谁签**：只有已经核对过读权限的读取会签：阅读视图的渲染（嵌入与链接）、附件面板的列表、上传的答复、`GET /assets/{node_id}`。失去访问的账户手里已签出的地址在到期之前仍能读（至多 2 小时，第 10 节）。
- **响应头**（全局的 `nosniff`、`X-Frame-Options: DENY` 照旧）：

  | 类型 | `Content-Type` | `Content-Disposition` | `Content-Security-Policy` |
  |---|---|---|---|
  | 内联的图片、音频、视频（png、jpeg、gif、webp、avif、bmp；mp3、ogg、wav、m4a、flac、webm；mp4、webm、ogv） | 测定的类型 | `inline` | `sandbox` |
  | svg | `image/svg+xml` | `inline` | `sandbox`（直接打开时脚本不运行、源是不透明的） |
  | pdf | `application/pdf` | `inline` | P1 在 Chromium 与 Firefox 里实测：内置的阅读器在 `sandbox` 下能用就加，不能用就不加（PDF 的脚本在阅读器自己的环境里，不在本站的源里），写进 P1 文档 |
  | 其余（含 html、xml、js、`application/octet-stream`） | `application/octet-stream` | `attachment` | `sandbox` |

  - `d=1` 时一律 `attachment`。文件名按 RFC 6266 写 `filename*=UTF-8''…`，另带一个 ASCII 的 `filename` 兜底。
  - `Cache-Control: private, max-age=<到期前的秒数>, immutable`，覆盖 `/api/` 默认的 `no-store`；`ETag` 是 SHA-256；`Cross-Origin-Resource-Policy: same-origin`。
  - 用 `http.ServeContent` 下发：支持 `Range`（视频拖动）与条件请求。
- **限流**：签名的下载另用一个按 IP 的桶（`ratelimit.asset_content`），一页几百张图第一次打开时不撞匿名请求的桶。
- **路由**：下载与上传都要逐字节地读写请求或响应，不走 oapi-codegen 的 strict 处理（它不支持 `ServeContent`，也会缓冲）。它们仍写在接口描述里（`/api/` 下的路由都必须是描述里的操作），标 `x-raw: true`，由模块手写处理并登记，像 `x-long-lived` 的路由一样（总体设计 13.1 第 14 条）。P1 把这条约定写进第 13 节。

### 4.5 删除、清理与孤儿文件

- **`asset_blobs`**（asset 模块）：`id`（blob id，UUIDv7）、`node_id`（指向 `nodes`，`ON DELETE RESTRICT`，唯一）、`notebook_id`（指向 `notebooks`，`RESTRICT`）、`mime`、`byte_size`、`sha256`、`uploaded_by`、`created_at`、`deleted_at`。存储的键由 blob id 推出，不另存。
- **跟着节点删除**：page 的删除只改自己的表，asset 看不到 `nodes`。asset 模块登记：
  - **页面领域事件的观察者**（事务内）：改动里被删除的附件节点，把它们的行以同一个 `deleted_at` 软删除。（总体设计 12.4 这一行的注册者加上 M7。M8 的恢复照同一个观察者恢复行，写进 M8 的移交。）
  - **笔记本删除事件的订阅者**：软删除这些笔记本的全部行（总体设计 12.4）。
- **清理器**（软删除的清理注册表，排在 page 的清理器之前；外键是 `RESTRICT`，次序不对时 page 那一批失败，下一次再完成）：取一批到期的行，**先删文件、后删行**；删文件不在事务里，文件已不存在算成功；"文件删了、行没删"时下一次运行再删一次文件、删掉行（M2/P4 的移交）。
- **孤儿清扫**（定时任务，每天）：`blobs/` 下修改时刻早于一天、而 `asset_blobs` 里（含软删除的）没有它的文件，删掉；`tmp/` 下早于一天的临时文件删掉。上传在 `Commit` 之后、单元提交之前失败，或删除文件失败，留下的就是这些。
- **笔记本的活动**：字节数是未删除的附件的大小之和，最后写入是它们最晚的 `created_at`（M3 的移交）。

### 4.6 附件进链接

照 [M6 的移交](handoffs/M6-links.md)，细节在 P2 文档，与 Obsidian 1.12.7 逐项核对：

- **解析**：附件是链接的候选。目标的最后一段带扩展名时，先找名称恰好如此的附件或页面（照 Obsidian 的 `getFirstLinkpathDest`），没有再找加上 `.md` 的页；并入"只读作一种"（M6 总设计 4.4）。page 给 linking 的读端口（`LinkTargets`、`All`）带上节点的类型；接口的 `LinkTargetKind` 加上 `asset`。
- **嵌入的渲染**（M7 建立的扩展点）：`obsidian.Options` 的 `Resolve` 的结果带上类型，另加 `Assets(ctx, ids)`：组合根经 asset 模块给出每个附件的类型、名称、大小与签名地址。
  - `![[x.png]]`、`![[x.png|300]]` → `<img class="nw-asset" src=… alt=… width=…>`（尺寸照 Obsidian：`|宽` 或 `|宽x高`）；
  - 音频 → `<audio controls preload="metadata">`，视频 → `<video controls preload="metadata">`；
  - 其余（含 pdf）→ 带文件名与大小的链接，在新标签页打开；
  - `Assets` 交空时嵌入照 M6 渲染为链接。标记写进 `Markup`，`CheckHTML`、`CheckSize` 随之更新。
- **Markdown 图片**：`![说明](架构图.png)` 解析到图片附件时内联为 `<img>`；解析不到或是外部地址时照 M4 的 `<span class="nw-image">`（不加载外部的图片，CSP 的 `img-src` 只许本站）。
- **指向附件的 wikilink**（`[[报告.pdf]]`）：`<a … data-nw-asset=… href=签名地址>`，前端在新标签页打开，不经路由。
- **改名、移动**：附件改名、移动时，解析到它的链接照页面的规则改写（M6 的改写参与者；Obsidian 打开"始终更新内部链接"时同样改写附件的嵌入），与 Obsidian 核对。
- **落点**：读作附件的目标，落点答新的原因（`target_is_asset`），前端照"没有落点"说明，解决 M6 的移交第 3 项的循环。
- **地址在 HTML 里**：M6 的链接地址由前端给，是因为服务端不知道工作区的 slug；附件的签名地址服务端完全知道，所以渲染时直接写进 `src` 与 `href`（相对地址，过 `SafeURL`）。阅读视图本来就不缓存（总体设计 4.3），签名依赖时钟与密钥不是问题。
- **索引**：附件的新建、改名、移动、删除经观察者重新解析指向它们的链接；`checkLinks` 认附件；M6 的性质测试加上附件的操作。

### 4.7 前端

- **页面树只列页面**：`stores/page-tree.ts` 按类型分开：侧栏、子页列表、面包屑、快速切换只看页面；"未命名 N"的取名看全部子节点（共用命名空间）。树的事件带类型（4.2）。P1 先做这一步，免得附件在 P3 之前显示成页面。
- **附件面板**：右栏加一节"附件"（与大纲、反链、属性并列，窄屏在正文之后）：列出这一页的附件（名称、大小、类型的图标），上传按钮与拖到这一节上传（带进度、可取消），每一项的菜单：打开、下载、复制嵌入（`![[名称]]`）、改名、删除。随树的事件实时更新。
- **上传的服务**：`XMLHttpRequest`（`fetch` 没有上传进度），带取消；重名时先在已加载的子节点上取空着的名字。
- **粘贴、拖入上传**（编辑器扩展，经 `editor/registry.ts` 注册）：
  - `EditorContext` 加 `uploadAsset(file, name)`；扩展处理 `paste` 与 `drop` 里的文件，上传完成后在原位置（随之后的输入映射）插入 `![[名称]]`，输入法组合中等 `whenComposed`；
  - 剪贴板里没有文件名的图片取名为"粘贴的图片 20261008123045.png"（照 Obsidian 的 "Pasted image …"，按界面语言）；
  - 上传失败时提示，不插入任何东西。
- **阅读视图**：图片 `loading="lazy"`，点开在新标签页；`<img>` 加载失败时（地址过期）重读一次阅读视图；指向附件的链接在新标签页打开。

### 4.8 后台任务

- **只投递的客户端**（M2/P4 的移交）：`platform/jobs` 加一个不配队列的 River 客户端，在请求的事务里 `InsertTx`，与 `transfer_jobs` 的行同一个提交，回滚时任务也不存在。事务由 `transfer/adapter/river` 从上下文取出（`platform/postgres` 加一个导出的取法）。
- **自己的队列**：导入导出走队列 `transfer`，默认 1 个 worker（`jobs.transfer_workers`），不占定时任务的队列（默认队列只有 2 个）。
- **不自动重试**：任务的 `MaxAttempts` 是 1。失败（含服务重启时被中断）由任务自己把 `transfer_jobs` 记成失败并写报告；启动时与每小时，把还记着"运行中"、River 里已不在运行的任务记成失败（"服务重启，任务中断"）。
- **任务的身份**：`transfer_jobs` 记下发起的账户、凭据（登录会话或个人访问令牌）与客户端；worker 以它们组成 `shared.Actor` 调用写入单元，每个单元照常授权：中途失去写权限时那个单元被拒绝，任务失败并写明。凭据之后失效不打断已开始的任务（只看账户的角色）。
- 命令行不投递任务：组合规则（`archtest/composition_test.go`）不改。

### 4.9 导出

- **范围与权限**：整个笔记本，或一页及其子孙；能读这个笔记本的人都能导出（导出就是读，权限码 `transfer.export`）。
- **一致的快照**：任务在一个 `REPEATABLE READ READ ONLY` 事务里读出范围内活着的节点与正文，边读边写 zip；附件的文件写入后不变，从存储读。
- **映射**（总体设计 3.5）：页面写 `<标题>.md`，子节点在 `<标题>/` 里，附件是它的父页面目录里的文件。
  - 没有正文、只有子节点的页：只有目录，除非导出的内容里有链接解析到它，这时另写一个空的 `.md`，让 Obsidian 里的链接同样解析到它（M6 的移交第 5 项）；没有正文也没有子节点的页写空的 `.md`。
  - zip 的条目名用 UTF-8（设置 UTF-8 标志），超过 4 GiB 时用 zip64（`archive/zip` 自动）。
- **`.nerve/meta.json`**：

  ```json
  { "format": 1, "exported_at": "…", "notebook": { "id": "…", "name": "…" }, "root": null,
    "nodes": [ { "path": "项目A.md", "kind": "page", "id": "…", "sort_order": 1.5 } ] }
  ```

  导入时据它恢复兄弟的次序；`id` 只作参考，导入不复用它。
- **导出贡献者**（M7 建立的扩展点，M10 注册虚拟的 `index`、`log`）：`transfer.ExportContributor`：`Contribute(ctx, scope, sink) error`，往 zip 里加自己的文件（路径不能与节点的冲突，冲突时导出失败）。M7 组合交空，模块根有示例的测试。
- **结果**：写到 `exports/<job id>.zip`，任务成功；下载经签名地址 `GET /api/v0/transfer-jobs/{job_id}/download?e=…&s=…`（`attachment`，文件名是笔记本或页的名称），`transfer.export_ttl`（默认 24 小时）之后定时任务删掉文件，任务记成"已过期"。
- **进度**：写入的节点数 / 总数。

### 4.10 导入

- **接口**：`POST /api/v0/notebooks/{notebook_id}/imports`，`multipart/form-data`：`parent_id`（可选）、`file`（zip）。流式写到 `imports/<job id>.zip`（上限 `transfer.import_max_bytes`，默认 512 MiB，时间照 4.3），然后在一个事务里写 `transfer_jobs` 的行并投递任务，答 202 与任务。权限码 `transfer.import`（写者）。
- **先校验，后写入**：任务先把全部条目过一遍，不写任何东西：
  - 整包拒绝（任务失败，报告写明）：不是 zip、条目超过 `transfer.import_max_entries`（默认 50,000）、解压后的总字节数超过 `transfer.import_max_unpacked_bytes`（默认 4 GiB，按实际读出的字节计，不信条目头）、单个条目的压缩比超过 200；
  - 逐条跳过并写进报告：路径越出根（`..`、绝对路径、盘符）、符号链接与其他特殊文件、加密的条目、名称不是 UTF-8、`.md` 不是合法的 UTF-8 或超过 5 MB、附件超过 `asset.max_bytes`、深度超过 10 的部分；
  - 忽略：`.obsidian/`、`.trash/`、`.git/`、`__MACOSX/`、`.DS_Store`、`Thumbs.db` 与其他以 `.` 开头的条目（`.nerve/meta.json` 读作次序）。
- **映射**：`X.md` 是页面 `X`（正文逐字节）；目录 `X/` 是 `X` 的子节点，同级没有 `X.md` 时 `X` 是没有正文的页；其他文件是附件（`.md` 总是页面）。兄弟的次序照 `.nerve/meta.json`，没有就按名称。
- **名称**：照节点的规则修正并写进报告：禁止的字符换成 `_`，首尾的空白与 `.` 去掉，Windows 保留名后加 `_`，过长的在字符边界截到 255 字节（保留扩展名），空的叫"未命名"；同一父节点下（按标题键）撞名的，后来的加序号（" 2"）。导入位置原有的子节点同样参与撞名。改过名称的，正文里指向旧名称的链接会解析不到，报告逐条列出。
- **写入**：
  - 按层分批：一个写入单元至多 100 个节点、8 MiB 正文，单元的类型是 `import`；第一个单元建变更集，之后的单元并入同一个（"一次导入一个变更集"，总体设计 3.8），照编辑会话的做法。
  - 每页的解析照参与者不排队地取预算（`TakeNow`）；取不到时这一批稍后重试，不让任务失败。
  - 附件的文件先从 zip 流进存储，再在单元里建节点与行（照 4.3 的次序）。
  - 新建的页经观察者进索引，之后建的页会让先前解析不到的链接重新解析到它们（M6 的观察者）。导入结束后对 linking 与 page 的几张表 `ANALYZE`（M6 的移交第 4 项；服务的角色要加 `MAINTAIN`，写进 `runtime-grants.sql`）。
- **取消**：`POST /api/v0/transfer-jobs/{job_id}/cancel` 立一个标记，任务在两批之间停下，报告写明已导入的部分。中途失败同样如此。已导入的部分照常是笔记本里的页，用户可以删除。
- **报告**：计数（页、附件、跳过、改名、错误）与前 1,000 条问题（路径、原因），存在 `transfer_jobs.report`。
- **进度**：已处理的条目 / 总数。网页的对话框每秒读一次任务（不加新的事件类型）。

### 4.11 部署与配置

- **配置**：

  | 键 | 默认 | 说明 |
  |---|---|---|
  | `storage.dir` | `data`（镜像里 `/data`） | 附件、导入导出的文件 |
  | `asset.max_bytes` | 50 MiB | 单个附件 |
  | `asset.upload_min_rate` | 64 KiB/s | 放宽上传的读截止时间 |
  | `transfer.import_max_bytes` | 512 MiB | 导入包 |
  | `transfer.import_max_entries` | 50,000 | |
  | `transfer.import_max_unpacked_bytes` | 4 GiB | |
  | `transfer.export_ttl` | 24 h | |
  | `jobs.transfer_workers` | 1 | |
  | `ratelimit.asset_content` | 每分钟 6,000、突发 1,000（每 IP） | |

  交叉规则（总体设计 8.4）：`asset.max_bytes` ≤ `transfer.import_max_bytes` ≤ `transfer.import_max_unpacked_bytes`。
- **镜像**（M0/P6 的移交）：构建阶段建好 `/data`、属主 65532，`COPY --chown` 进运行时阶段，`VOLUME /data`；README 的部署一节写挂载方式（宿主机目录的属主要是 65532）与备份次序。
- **e2e**：每个 worker 的服务有自己的存储目录（`fixtures/server.ts`）。

### 4.12 权限

access 的规则表加上：`asset.upload`（写者及以上）、`transfer.export`（能读的都可以）、`transfer.import`（写者及以上）。附件的改名、移动、删除照旧是 `node.*`。下载靠签名，签名之前已按读权限核对。

## 5. 接口

| 方法与路径 | 说明 |
|---|---|
| `POST /api/v0/notebooks/{notebook_id}/assets` | 上传（`x-raw`，multipart） |
| `GET /api/v0/assets/{node_id}` | 附件的元数据与签名地址 |
| `GET /api/v0/assets/{node_id}/content` | 下载（`x-raw`，公开，靠签名） |
| `GET /api/v0/pages/{page_id}/assets` | 一页的附件（面板用；元数据与签名地址） |
| `POST /api/v0/notebooks/{notebook_id}/exports` | 开始导出（`root_id` 可选） |
| `POST /api/v0/notebooks/{notebook_id}/imports` | 上传 zip 并开始导入（`x-raw`，multipart） |
| `GET /api/v0/notebooks/{notebook_id}/transfer-jobs` | 这个笔记本的任务（发起人自己的，与管理员看到的全部） |
| `GET /api/v0/transfer-jobs/{job_id}` | 任务的状态、进度、报告；导出成功时带下载地址 |
| `POST /api/v0/transfer-jobs/{job_id}/cancel` | 取消 |
| `GET /api/v0/transfer-jobs/{job_id}/download` | 下载导出的 zip（`x-raw`，公开，靠签名） |

改动：`NodeKind`、`LinkTargetKind` 加 `asset`；落点的原因加 `target_is_asset`。错误码：`asset.markdown_name`、`asset.too_large`（413 的细分）、`transfer.not_found`、`transfer.not_cancellable` 等，写进 P1、P4、P5 的文档。

## 6. 从 Nerve 借鉴

Nerve 的文件里程碑还没开始，只有计划与平台的做法（只读参考，不改它的文件）：

- 按用途派生密钥、截断的 HMAC、严格的令牌编码（`identity/adapter/signing/mac.go`）：本仓库已有同样的 `SigningKeys.Derive`，签名地址照用。
- 上传、下载按请求放宽截止时间（`http.ResponseController`），不动全局的超时；大小的上限与时间的上限分开（Nerve M0 的对抗评审）。
- `/api/` 默认 `no-store`，下载要显式覆盖；`X-Frame-Options: DENY` 下不能用同源的 iframe 预览 PDF，所以 PDF 在新标签页打开。
- 服务端自己测定类型，不信客户端声明的（Nerve 继承自 Plane 的弱点）；svg、html 这类能执行脚本的类型强制隔离或下载。
- 复制附件之类按 id 的操作先核对读权限，"读不到"与"不存在"答同样的 404（Nerve 的越权读取教训）：M7 的签名只在核对过读权限之后发出。
- 长任务用自己的队列，不占定时任务的 worker。

## 7. Phase 划分

| P | 名称 | 交付 | 验证 |
|---|---|---|---|
| P1 | 存储与附件（服务端） | `platform/storage`；asset 模块：`asset_blobs`、上传、类型测定、签名的下载与响应头、`x-raw` 路由的约定、观察者与笔记本删除、清理器、孤儿清扫、活动；page：`CreateAsset`、改动带类型、深度只数页面、`.md` 名称、`NodeKind`；前端的页面树只列页面；配置、镜像的卷、启动检查、`image-smoke` | 存储的契约测试；上传（流式、上限、慢速、截止时间）与下载（签名、过期、改参数、响应头的表格）的测试；整个程序上的删除、清理（含文件已删）、活动、组合根交空时失败；e2e：AS1、AS4、AS5 |
| P2 | 附件与链接（服务端） | 附件进解析；嵌入的渲染（`obsidian.Options`）；Markdown 图片指向附件；指向附件的链接；改名、移动的改写；落点拒绝附件；`LinkTargetKind`；`checkLinks` | `resolve/`、`rename/`、`render/`、`cases/` 的附件样例与 Obsidian 1.12.7 核对；索引的性质测试加附件；渲染的测试、`CheckHTML`、`CheckSize`；整个程序上的最后一跳 |
| P3 | 附件（前端） | 附件面板；上传的服务（进度、取消）；编辑器的粘贴、拖入上传；阅读视图的附件（图片、音视频、链接、过期重读）；输入法清单的粘贴一步 | vitest：面板、上传、扩展经组合根到达编辑器；e2e：AS2、AS3 |
| P4 | 导出 | transfer 模块、`transfer_jobs`、只投递的客户端与队列、任务的身份与中断的收拾；导出（快照、映射、`meta.json`、空页的规则）、导出贡献者、下载、到期清理；前端的导出对话框 | 映射的逐项测试；贡献者的示例测试；e2e：TR1 |
| P5 | 导入 | 导入的上传、校验、写入（分批、同一个变更集、预算、`ANALYZE`）、报告、取消；前端的导入对话框 | 恶意 zip 的表格测试；导出再导入的性质测试；Obsidian 库样例的解析核对；e2e：TR2–TR4 |

收尾：三位 Opus 审查者并行（后端、前端与端到端、完成标准与文档），然后 Opus 核对修复。

### 移交的落实

| 移交 | 项 | 落实 |
|---|---|---|
| [M0/P6 镜像里的附件目录](handoffs/M0-P6-image-volumes.md) | 1 目录与卷、2 可写检查、3 `image-smoke` | P1（4.1、4.11） |
| [M2/P4 只投递的 River 客户端](handoffs/M2-P4-insert-only-client.md) | 1 客户端、2 停机顺序、3 命令行、4 权限 | P4（4.8）：命令行不投递 |
| [M2/P4 附件的清理](handoffs/M2-P4-attachment-purge.md) | 1 先删文件、`RESTRICT`、排在父表之前；2 事务边界与"文件删了、行没删"的测试；3 清理器的其余约束 | P1（4.5） |
| [M3 笔记本活动](handoffs/M3-notebook-activity.md) | 1 字节数与最后写入、2 整个程序的测试 | P1（4.5） |
| [M4 附件的扩展与粘贴上传](handoffs/M4-extensions.md) | 1 附件内联（以 M6 的移交第 1 项为准）、2 粘贴上传与 `whenComposed`、3 最后一跳的两个测试、4 先建后删与活动 | 1：P2；2、3：P2、P3；4：见下两行 |
| [M4/P2 一个单元里先建后删](handoffs/M4-P2-unit-merge.md) | — | 不适用：导入只建不删（4.2），由 M9 定下；以"不适用"关闭，指向 M9 的那份 |
| [M6 链接与附件、导入、导出](handoffs/M6-links.md) | 1 嵌入的渲染由 M7 建立；2 附件进解析；3 落点；4 导入是多操作的单元；5 导出没有正文的页；6 最后一跳 | 1–3、6：P2；4：P5；5：P4 |
| [M4/P3 Markdown 的扩展](../M6-links/handoffs/M4-P3-markdown-extensions.md)（已关闭，抄送 M7） | 第 1–7 项的注册约束 | P2 照做 |
| [M4/P6 编辑器](../M5-collab-editing/handoffs/M4-P6-editor.md)（已关闭，抄送 M7） | 第 5 项：上传的手段、最后一跳 | P3 |

## 8. 本 M 建立与注册的扩展点

| 扩展点 | 角色 | 落实 |
|---|---|---|
| 附件嵌入的渲染（`obsidian.Options` 的 `Assets` 与 `Resolve` 带类型） | 建立并注册 | P2；整个程序上的最后一跳，组合根交空时嵌入照链接渲染、测试失败 |
| 导出贡献者 | 建立（M10 注册） | P4；组合交空，模块根的示例测试 |
| 软删除的清理注册表 | 注册（附件，排在 page 之前） | P1 |
| 笔记本删除事件 | 注册（附件的行） | P1 |
| 笔记本的活动 | 注册（附件） | P1 |
| 页面领域事件（观察者） | 注册（附件的行跟着节点删除） | P1；总体设计 12.4 这一行加上 M7 |
| Markdown 解析与渲染扩展 | 注册（附件嵌入，经方言的参数） | P2 |
| 编辑器扩展管线（前端） | 注册（粘贴、拖入上传） | P3 |
| 实时推送的事件类型 | 不加新类型：树的事件带上类型（4.2）；导入导出的进度靠读任务 | P1 |

## 9. 测试策略

- **存储**：契约测试（`storagetest`）；本地实现另测原子性（`Commit` 之前不可见、`Abort` 不留文件、进程中途退出不留半个文件）与启动检查。
- **上传与下载**：handler 层的表格测试（字段次序、缺字段、上限、类型测定、预检的每个码）；慢速上传与不发正文的连接（截止时间）；签名的伪造、过期、改每一个参数；响应头的表格；`Range` 与条件请求。
- **整个程序**（总体设计 13.1 第 21 条，组合根交空时失败）：删除子树与删除笔记本之后行被软删除；清理先删文件；活动；嵌入渲染的最后一跳；导入经观察者进索引。
- **链接**：样例与 Obsidian 核对；M6 的性质测试加上附件的操作；改写的随机测试加上附件。
- **导入导出**：映射的逐项测试；恶意 zip 的表格测试；导出再导入的性质测试（随机的树、正文、附件、次序）；一个 Obsidian 库的样例；取消与中途失败的报告；中断的任务被收拾。
- **e2e**（新的两组，页面版本与 PAT 的接口版本，总体设计 12.5）：
  - AS1：上传、列出、下载（接口）；面板上传与打开（页面）。
  - AS2：编辑器里粘贴、拖入图片，插入嵌入并显示。
  - AS3：嵌入的图片、音频、视频、附件链接显示；附件改名之后嵌入改写；未解析的嵌入。
  - AS4：删除子树、删除笔记本之后附件的行与文件被清理（`deletedDaysAgo`）；活动算上附件。
  - AS5：svg 与 html 附件在浏览器里不在本站的源里执行。
  - TR1：导出笔记本与子树，zip 的结构与 `meta.json`。
  - TR2：导入 zip，进度、报告，导入的页与附件，链接解析。
  - TR3：导出再导入得到同样的树。
  - TR4：恶意的 zip 被拒绝或跳过，报告写明。
- **人工**：输入法清单加"组合中粘贴图片"一步（P3）。

## 10. 风险

| 风险 | 缓解 |
|---|---|
| 磁盘写满：没有配额 | 单个文件与导入包有上限；存储写失败答 503 `storage_unavailable` 并记日志；README 写明监控磁盘。配额留到 v0.1 之后 |
| 失去访问之后，已签出的地址仍能读（至多 2 小时） | 签名只在核对过读权限之后发出；到期短；写进安全说明。要立刻撤销时，轮换签名私钥（所有地址失效） |
| 浏览器把附件当作本站的页面执行脚本 | 服务端测定类型；svg 与一切非白名单类型带 `sandbox`，非白名单的强制下载；`nosniff`；e2e 在浏览器里证明 |
| 导入长时间持有笔记本的锁 | 小批量的单元；批与批之间放开锁；预算取不到时稍后重试 |
| 压缩炸弹、越出目录、海量条目 | 先校验后写入；按实际读出的字节计；上限可配 |
| 服务重启时导入导出被打断 | 不自动重试；任务被收拾成失败并写报告；导入的部分照常可见、可删 |
| 文件与行不一致（崩溃、手工改动卷） | 先写文件后提交、先删文件后删行；孤儿清扫；下载时文件不在答 404 并记日志 |
| 反向代理限制了请求体或超时 | README 的部署一节写明 `client_max_body_size` 与超时的设置 |

## 11. 负责人确认的决定

M7 开工时负责人确认进入 M7（2026-10-08："可以了"）。下面是设计里的取舍，负责人可以改判：

1. 附件不算一层：深度 10 的页面下仍可以挂附件。
2. 能读的人都能导出整个笔记本。
3. 导入时不合规的名称被修正（并写进报告），而不是跳过。
4. 只有子页、没有正文、但被链接到的页，导出时另写一个空的 `.md`。
5. 附件改名、移动时改写指向它的链接（照 Obsidian）。
6. 签名地址有效 1–2 小时；失去访问之后仍能读到到期。
7. 不做配额；不做命令行的导入导出。
8. PDF 内联在新标签页打开；`sandbox` 是否加由 P1 的实测定。

## 12. Phase 进度表

| P | 名称 | 状态 | Phase 文档 | 审查 |
|---|---|---|---|---|
| P1 | 存储与附件（服务端） | 未开始 | — | — |
| P2 | 附件与链接（服务端） | 未开始 | — | — |
| P3 | 附件（前端） | 未开始 | — | — |
| P4 | 导出 | 未开始 | — | — |
| P5 | 导入 | 未开始 | — | — |

## 13. 变更记录

| 日期 | 变更 | 依据 |
|---|---|---|
| 2026-10-08 | 初稿 | 总体设计 12.2；7 份移交；对照 `629f741` 的代码复核 |
