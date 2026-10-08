# M7/P2 附件（服务端）：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M7/P2 附件（服务端） |
| 状态 | 进行中 |
| 基线 | `5a94218`（P1 合并、文档补完之后的 main）；本文提交之后开分支 `m7-p2` |
| 上级文档 | [M7 总设计](00-M7-design.md) 4.1、4.2、4.4–4.6、4.13、4.14、第 5、7–9 节；[P1 文档](01-P1-storage-stream.md)第 7 节（交给 P2 的五项）；移交：[M2/P4 附件的清理](handoffs/M2-P4-attachment-purge.md)、[M3 笔记本活动](handoffs/M3-notebook-activity.md)、[M4 附件的扩展](handoffs/M4-extensions.md)第 4 项的活动、[M0/P6 镜像里的附件目录](handoffs/M0-P6-image-volumes.md)第 3 项；总体设计 13.1 第 1、5、6、8、10、15、21、25、28 条，13.4 第 4、5、6 条 |

---

## 1. 基线

- **P1 已有**：`platform/storage`（`Store`、本地实现、`ErrFull`、`FullAtOpen`），`httpserver.API.Stream`（`StreamPolicy`、`Bounded`、`Sending`、`ErrShuttingDown`）；serve 打开存储，组合根持有它但还没有使用者。
- **page**：`nodes.kind` 的检查已允许 `asset`，`domain.KindAsset` 已声明，但只建页面。链接目标（`link_targets.go`、`links.sql`）只认页面；`lineOf` 拒绝不是页面的父节点；`titleFree` 比较全部兄弟节点（附件与页面共用命名空间）；`Subtree.Height()` 与 `checkPages` 的深度数全部节点。读正文、编辑会话、勾选任务对不是页面的节点答 `page.not_found`。接口的 `NodeKind` 只有 `page`。
- **契约**：每个模块一份 `api/modules/<模块>.yaml`，逐模块生成 strict 服务端；`x-long-lived` 的 events 整个模块不生成（`Makefile` 的 `API_MODULES`）。没有一个模块里既有生成的、又有手写的操作的先例。`apitest` 的规则只认 JSON 的请求体与答复。
- **平台码**：`shared.Kind` 没有 507；平台码表里没有 `storage_full`。
- **组合根**：`purgers(pool)`；`notebookRegistrants` 的活动只有页面；`pageRegistrants` 的观察者是事件流与链接索引。
- **前端**：`PageTreeStore` 把 `listNodes` 的全部节点放进树；事件 `pages` 带 `tree` 时直接 `mutate(["pages", notebook])`，不经 `refresher`。
- **e2e**：`fixtures/purge.ts` 的 `deletedDaysAgo` 挪 `changeset_items`、`page_revisions`、`page_contents`、`nodes`……，没有 `asset_blobs`。

## 2. 目标与范围

**做**（总设计第 7 节 P2 一行）：

1. **契约与平台**：`x-raw` 的契约规则、整个程序的测试认它、代码生成按操作排除（模块的 `oapi-codegen.yaml` 列出，测试核对与契约一致）；平台码 `storage_full`（507，`shared.KindStorageFull`）；asset 的配置与 `ratelimit.asset_content`（test 配置调到用不完）；`InstanceInfo.asset_max_bytes`；access 的规则表加 `asset.upload`、`asset.read`。
2. **page**：`(*page.Module).TreeWrites()`（`Check`、`CreateAsset`）；附件的名称规则（新建与改名）；深度只数页面（`Subtree.Height`、`checkPages`）；`NodeKind` 加 `asset`；asset 用的读端口 `page.NewAssetNodes(pool)`。
3. **asset 模块**：`asset_blobs`（迁移、`runtime-grants.sql`）；`asset.NewBlobs(pool, store)`（`Put`、`Attach`、`Open`）；上传（`x-raw`、multipart、`API.Stream`）、类型测定与宽高；签名的下载与响应头（含 PDF 的实测）；元数据与列表；页面观察者与笔记本删除的订阅者；清理器（要存储）；孤儿清扫（每天）；笔记本的活动。
4. **前端**：页面树只列页面（`PageTreeStore` 留着全部节点，树只取页面；"未命名 N"仍看全部子节点）；`pages` 事件的整树重读经 `refresher`。
5. **e2e 与部署**：`deletedDaysAgo` 先挪 `asset_blobs`；AS1 的接口版本（上传、列出、元数据、下载）、AS4（删除与清理、活动）、AS5（各类型的响应头；svg 与 html 在浏览器里，直接打开签名地址）；`image-smoke` 加"上传、重启、读出同样的字节"。

**不做**（与总设计第 7 节的出入，随这份文档改 00 号文档）：

- `InstanceInfo.import_max_bytes` 随 P6（第一个用它的接口在 P6）。
- 附件进解析、链接与渲染、落点、补全：P3。P2 里指向附件的写法照旧是未解析的链接（有测试钉住：同名的附件不是链接目标）。
- AS1 的页面版本、附件面板、粘贴上传：P4。
- 按凭证限制同时的上传（P1 第 7 节第 4 项）：P2 不做，写进第 3.13 节的取舍：上传走 `authenticated` 的桶，一个凭证的并发由桶与 `asset.max_bytes / asset.upload_min_rate` 的上界约束（M12 的负载实测再定）。

## 3. 设计

### 3.1 文件

| 文件 | 内容 |
|---|---|
| `api/modules/asset.yaml`（新）、`api/openapi.yaml` | asset 的四个操作（3.2），`Asset`、`AssetList` |
| `api/modules/page.yaml` | `NodeKind` 加 `asset`；`TreeNode`、改名的说明；改名的 `name: not_allowed` |
| `api/modules/instance.yaml` | `InstanceInfo.asset_max_bytes` |
| `api/common.yaml` | 平台码 `storage_full`；`Problem.code` 的说明 |
| `server/internal/platform/httpserver/apitest/` | `x-raw`：`Operation.Raw`，规则（3.2），整个程序的测试跳过原样操作的 JSON 用例 |
| `server/internal/platform/httpserver/` | 507 的映射（`errors.go`） |
| `server/internal/shared/error.go` | `KindStorageFull`、`CodeStorageFull`、`StorageFull()` |
| `server/internal/platform/config/` | `AssetConfig{MaxBytes, UploadMinRate}`、`RateLimitConfig.AssetContent`、校验、`LogValue` |
| `server/configs/config.yaml`、`config.test.yaml` | `asset` 一节、`ratelimit.asset_content` |
| `server/internal/modules/access/domain/rules.go` | `asset.upload`（写者）、`asset.read`（读者） |
| `server/internal/modules/page/` | `tree_writes.go`（新：`TreeWrites`、`NewAsset`）、`asset_nodes.go`（新：读端口）；`app/unit_create_asset.go`（新）、`unit_rename.go`、`domain/name.go`（新：附件的名称规则）、`domain/tree.go`（`Height` 只数页面）；`adapter/postgres/queries`（附件节点的分页） |
| `server/internal/modules/asset/`（新） | `module.go`（`New`、`Register`、`Jobs`、`ContentKeyInfo`）、`blobs.go`（`NewBlobs`）、`observer.go`、`deletion.go`、`purgers.go`、`activity.go`；`domain/`（`Blob`、`Meta`、类型表、签名的字段、名称）；`app/`（上传、元数据、列表、下载的用例，签名，清扫）；`adapter/http/`（生成的两个操作 + 手写的上传与下载）、`adapter/postgres/`（sqlc）、`adapter/sniff/`（类型测定与宽高，`net/http`、`image` 在这里） |
| `server/migrations/sql/00027_asset_asset_blobs.sql`（新）、`deploy/runtime-grants.sql` | `asset_blobs` |
| `server/internal/bootstrap/` | 组合：page → asset；`purgers(pool, store)`；观察者、笔记本删除、活动的登记；`instanceDeps`；整个程序的测试（`assets_*_test.go`）、交错（`interleavings_assets_test.go`）、权限矩阵（`permission_matrix_asset_test.go`）；`checkPages` |
| `server/internal/archtest/` | 命令行的组合到不了 `asset.New`；asset 的 app 与 domain 不引 `net/http`、`image` |
| `web/apps/web/src/stores/page-tree.store.ts`、`events/handlers.ts` | 树只列页面；`pages` 的整树重读经 `refresher` |
| `e2e/fixtures/purge.ts`、`fixtures/assert/asset.ts`（新）、`stories/asset/`（新：AS1、AS4、AS5） | 3.12 |
| `deploy/image-smoke.sh`、`README.md` | 上传、重启、读出；附件的接口与 PDF 的说明 |

### 3.2 契约：`x-raw` 与 asset 的操作

**`x-raw: true`** 标在逐字节读写、由模块手写处理器的操作上（总设计 4.3）。规则（`apitest`，各有反例）：

- 原样的操作照样有 tag、`operationId`、`security`、`x-problem-codes`、`default` 的 problem；参数照常（`apitest` 按参数推出的 400 用例照常跑）。
- 请求体可以是 `multipart/form-data`（只有原样的操作可以），它的 schema 是关闭的对象，各字段 `format: binary` 或字符串；不参与 `BodyCases`（整个程序的 JSON 请求体用例）。
- 成功的答复可以是 `*/*`（只有原样的操作可以，schema 是 `format: binary` 的字符串），可以声明 206、304、416（下载）。
- 原样的操作不进代码生成：模块的 `oapi-codegen.yaml` 在 `output-options.exclude-operation-ids` 里列出它们；`TestRawOperationsAreNotGenerated`（`apitest`）读每个模块的配置与契约，二者的集合不等就失败。生成的 strict 服务端只有其余的操作；模块的 `Register` 另把原样的处理器经 `API.Stream` 挂在根路由器上（`TestAPIRoutesAreTheContractsOperations` 照旧核对路由与契约一一对应）。TS 客户端照常生成全部操作的类型（前端经同一个客户端调用，P4）。

**asset 的操作**（`api/modules/asset.yaml`，tag `asset`）：

| 操作 | 说明 | 码 |
|---|---|---|
| `uploadAsset` `POST /api/v0/notebooks/{notebook_id}/assets`（`x-raw`） | multipart：`parent_id`（可选）、`name`（可选）、`file`（最后）；201 `Asset` | `bad_request`、`notebook.not_found`、`forbidden`、`page.not_found`（父节点）、`validation_failed`（名称：`name` 的规则、`not_allowed`）、`page.title_taken`、`payload_too_large`、`storage_full`、`server_busy`（停机） |
| `listAssets` `GET /api/v0/notebooks/{notebook_id}/assets?parent_id=&cursor=&limit=` | 一个父节点（没有：根）下的附件，按名称键、id，游标分页（`limit` 1–100，默认 50） | `notebook.not_found`、`page.not_found`、`bad_request`（游标） |
| `getAsset` `GET /api/v0/assets/{node_id}` | 附件的元数据与签名地址 | `asset.not_found` |
| `getAssetContent` `GET /api/v0/assets/{node_id}/content?b=&e=&s=[&d=1]`（`x-raw`、公开） | 下载；200、206、304、416，`*/*` | `not_found`、`rate_limited`、`server_busy` |

`Asset`：`id`（节点）、`notebook_id`、`parent_id`、`name`、`mime`、`byte_size`、`sha256`（十六进制）、`width`、`height`（可空）、`created_by`、`created_at`、`content_url`（内联）、`download_url`（`d=1`）、`expires_at`。地址是相对的（`/api/v0/assets/...`）。

**`storage_full`**：`shared.KindStorageFull` → 507，`Retry-After` 不带；平台码表、`common.yaml` 的说明、`errors.go` 的映射与它的表格测试。存储的 `ErrFull` 由 asset 的适配器译成它。

### 3.3 page：树写入端口、名称、深度、读端口

**`TreeWrites`**（`tree_writes.go`）：接好线的模块给出（`(*page.Module).TreeWrites()`，模块持有它的 `*app.Writer`），只有两个方法：

```go
// NewAsset is an attachment to create: under ParentID (nil: the notebook's
// root), named Name, with the file's Meta, decided on Action from Client.
type NewAsset struct {
    NotebookID uuid.UUID
    ParentID   *uuid.UUID
    Name       string
    Meta       AssetMeta // MIME, Bytes, SHA256: the guards see them (M10)
    Action     shared.Action
    Client     domain.Client
}
type TreeWrites interface {
    // Check decides as CreateAsset would, unlocked: notebook.not_found,
    // forbidden, page.not_found for the parent, the name's rules, and
    // page.title_taken for a name a sibling holds now.
    Check(ctx context.Context, a NewAsset) error
    // CreateAsset creates the attachment node in a tree unit, the node last
    // among its siblings; after runs in the unit's transaction once the node
    // is written (asset writes its row), and its error rolls the unit back.
    CreateAsset(ctx context.Context, a NewAsset, after func(ctx context.Context, n Node) error) (Node, error)
}
```

- `CreateAsset` 是一个 `Tree: true` 的单元（变更集类型 `edit`），操作 `OpCreate`，改动只有树的状态（`Revision` 0、没有 `Facts`）；`Step` 加 `Asset *AssetMeta`（只在新建附件时非空），守卫读它。没有正文、没有版本；变更集的条目照常记（`Moves()`）。
- 次序：名称的规则 → `lineOf(parent)`（不是这本笔记本里活着的页面就 `page.not_found`……沿用 `lineOf` 已有的答法）→ 兄弟 → `titleFree` → 不查深度（附件不算一层）→ 写节点 → `after`。
- `Check` 与 `Writer.Allowed` 同一套判定，再不加锁地读父节点与兄弟。
- 这是 13.1 第 11 条的例外（总设计 4.1）：命令行的组合不建 `page.New`，到不了它（`archtest`）。

**附件的名称**（`domain/name.go`，新建与改名都查，按节点的类型）：在 `shared.CheckTitle` 之上——

- 不能以 `.md` 结尾（不分大小写，按 NFC 之后的名称）；
- 改名时，旧名有扩展名（最后一个 `.` 不在开头，之后至少一个字符）则新名也要有，可以改；
- 违反时是 422 的字段错误 `name: not_allowed`，`message` 说明原因（前端的文案，P4）。

**深度只数页面**：`domain.Subtree.Height()` 只数页面的层数（一个附件单独的高度是 0）；移动、新建的深度检查照用它；`checkPages` 的"深于十层"只走页面。页面的删除、移动带着它下面的附件（`Subtree` 照旧读全部节点）。

**读端口**（`asset_nodes.go`，只凭连接池）：

```go
type AssetNodes interface {
    // Node is the node id not deleted, of whatever kind; false for none.
    Node(ctx context.Context, id uuid.UUID) (NodeInfo, bool, error)
    // Parent tells whether parentID is a page not deleted of notebookID.
    Parent(ctx context.Context, notebookID, parentID uuid.UUID) (bool, error)
    // Assets is the attachments not deleted under parentID (nil: the root)
    // of notebookID, by title key and id, after the cursor's, at most limit.
    Assets(ctx context.Context, notebookID uuid.UUID, parentID *uuid.UUID, after *AssetCursor, limit int) ([]NodeInfo, error)
}
```

**接口**：`NodeKind` 加 `asset`；`ListNodes` 照旧答全部节点。

**链接**：P2 不碰 linking。链接目标只认页面（已如此）。附件的新建、改名、删除经观察者到链接索引：改动没有 `Revision`，索引只重解析受名称影响的链接，附件不是目标，结果不变。整个程序的测试钉住：页面写 `[[x.png]]`，同一父节点下上传 `x.png`，链接仍未解析；改名附件不改写任何页面。

### 3.4 asset 模块与 `asset_blobs`

```sql
-- 00027_asset_asset_blobs.sql
CREATE TABLE asset_blobs (
    id            uuid PRIMARY KEY,               -- blob id (UUIDv7); the store's key is blobs/<id>
    node_id       uuid NOT NULL UNIQUE,
    notebook_id   uuid NOT NULL REFERENCES notebooks(id) ON DELETE RESTRICT,
    mime          text NOT NULL,
    byte_size     bigint NOT NULL CHECK (byte_size >= 0),
    sha256        bytea NOT NULL CHECK (octet_length(sha256) = 32),
    width         integer CHECK (width > 0),
    height        integer CHECK (height > 0),
    created_by_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at    timestamptz NOT NULL,
    deleted_at    timestamptz,
    FOREIGN KEY (notebook_id, node_id) REFERENCES nodes(notebook_id, id) ON DELETE RESTRICT,
    CHECK ((width IS NULL) = (height IS NULL))
);
CREATE INDEX asset_blobs_activity ON asset_blobs (notebook_id) WHERE deleted_at IS NULL;
CREATE INDEX asset_blobs_purge ON asset_blobs (deleted_at) WHERE deleted_at IS NOT NULL;
```

（复合外键引用 `nodes` 已有的 `nodes_notebook_id_id_key`；`runtime-grants.sql` 给 `SELECT, INSERT, UPDATE, DELETE`。）

**`asset.NewBlobs(pool, store)`**（transfer 的导入与导出也用，P5、P6）：

- `Put(ctx, name string, r io.Reader, max int64) (Blob, error)`：`Create("blobs/<新 id>")`，边写边算 SHA-256，记下前 512 字节；超过 `max` 字节 `Abort`、答 `ErrTooLarge`；读出错 `Abort`、答它；`Commit` 之后按名称的扩展名与前 512 字节测定类型（3.5），图片再 `Open` 读宽高。`ErrFull` 原样带出。
- `Attach(ctx, node uuid.UUID, notebook uuid.UUID, by uuid.UUID, at time.Time, b Blob) error`：在调用方的事务里写行（`postgres.DB(ctx, pool)`）。
- `Open(ctx, nodeID) (storage.File, Blob, error)`：按节点读活着的行与文件。
- `Drop(ctx, b Blob) error`：删文件（单元以领域错误回滚时）。

### 3.5 上传

处理器（`adapter/http/upload.go`）经 `API.Stream`：`MaxBytes = asset.max_bytes + 64 KiB`（multipart 的外包装），`MinRate = asset.upload_min_rate`，桶照用 `authenticated`。

1. `notebook_id` 绑定失败 400；`Content-Type` 不是带 boundary 的 `multipart/form-data` 答 400。
2. 逐部分读（`r.MultipartReader()`）：只认 `parent_id`、`name`、`file`，次序如此，前两者可以没有；未知、重复、次序不对、`file` 之后还有部分都是 400；`parent_id` 是 UUID，`name` 至多 1 KiB；`file` 之前读过的字节（含部分的头）超过 4 KiB 答 400。名称：`name`，没有就用 `file` 部分的文件名（`filepath.Base` 之后），都没有 422。
3. **预检**（`Bounded` 之内，读文件之前）：`TreeWrites.Check` 与存储的余量（`Free` 低于下限：507）。不通过就答，不读文件（net/http 至多再读约 256 KB 就关连接，客户端可能只看到重置：网页在发送之前先查，P4）。
4. `Blobs.Put(name, file, asset.max_bytes)`：超过上限 413；`ErrFull` 507；读请求体失败：先判断 `httpserver.ErrShuttingDown`（不记 ERROR，尽力答 503），超时与客户端断开记 INFO，不再答复。
5. `file` 之后 `NextPart` 必须是 `io.EOF`（否则 400，删文件），然后把请求体读到结尾（`io.Copy(io.Discard, r.Body)`，P1 第 7 节第 1 项）。
6. **单元**（`Bounded` 之内）：`TreeWrites.CreateAsset(…, after = Blobs.Attach)`。单元返回领域错误（`*shared.Error`：权限、父节点、重名，必然已回滚）时删文件（`Blobs.Drop`）、答它；其余错误（基础设施、`COMMIT` 结果不明）不删，留给孤儿清扫，答 500（`COMMIT` 结果不明时行可能已提交，删了就是永远打不开的附件，总设计 4.4）。
7. 答 201 `Asset`（签好的地址）；日志 `asset uploaded`（`notebook_id`、`node_id`、`blob_id`、`user_id`、`mime`、`bytes`、`client`）。

**类型的测定**（`adapter/sniff`，总设计 4.4）：

| 扩展名 | 类型 | 同一种容器（嗅探的结果） |
|---|---|---|
| png、jpg/jpeg、gif、webp、bmp | `image/png`、`image/jpeg`、`image/gif`、`image/webp`、`image/bmp` | 同名的类型 |
| avif | `image/avif` | 嗅探不出 |
| svg | `image/svg+xml` | `text/xml`、`text/plain` |
| mp3、wav、flac | `audio/mpeg`、`audio/wave`、`audio/flac` | `audio/mpeg`；`audio/wave`；嗅探不出 |
| ogg/oga、m4a | `audio/ogg`、`audio/mp4` | `application/ogg`；`video/mp4` |
| mp4、webm、ogv | `video/mp4`、`video/webm`、`video/ogg` | `video/mp4`；`video/webm`；`application/ogg` |
| pdf | `application/pdf` | `application/pdf` |

- 扩展名在表里，且嗅探的结果属于同一种容器或嗅探不出（`application/octet-stream`、`text/plain`）→ 用扩展名的类型；扩展名不认识、嗅探的结果在表里 → 用嗅探的；其余 → `application/octet-stream`。嗅探出 HTML（`text/html`）的一律 `application/octet-stream`。
- **宽高**：`image/png`、`image/jpeg`、`image/gif` 在 `Commit` 之后 `Open`、`image.DecodeConfig`（至多读 1 MiB），读不出不记；宽或高超过 65,535 不记。

### 3.6 签名与下载

**签名**（`app/sign.go`，总设计 4.5）：

- 键：`SigningKeys.Derive(asset.ContentKeyInfo)`，`ContentKeyInfo = "nervewiki asset-content mac v1"`，由模块根导出；已知答案的测试钉住（13.1 第 25 条）。
- `s = base64url_nopad(HMAC-SHA256(key, "asset-content" ‖ node 16 字节 ‖ blob 16 字节 ‖ e 8 字节大端 ‖ d 1 字节)[:16])`，22 个字符。
- `e = (⌊now / 1h⌋ + 2) × 1h`（Unix 秒）：同一个小时签出的地址相同，有效 1–2 小时。
- 只有核对过读权限的读取签：元数据、列表、上传的答复（P3 起加阅读视图）。

**下载**（`adapter/http/content.go`）经 `API.Stream`：公开、桶 `ratelimit.asset_content`（按 IP，`BucketName: "asset_content"`），`MinRate = asset.upload_min_rate`，没有请求体。

1. **严格的读法**（任何查询之前）：查询串只有 `b`、`e`、`s`、`d`，各至多一次，`b`、`e`、`s` 必有；`b` 是规范写法的 UUID（小写、带连字符）；`e` 是规范的十进制（没有前导零、符号）；`s` 恰好 22 个 base64url 字符；`d` 没有或是 `1`；签名对；`e` 晚于现在。任何一项不对都答 404 `not_found`（`no-store`，与"不存在"不分）。
2. 读行：节点与 blob 对得上、没有软删除；否则 404。文件不在：404，记 WARN（`asset file missing`，`blob_id`）。
3. 响应头（签名核对通过之后才设，覆盖 `/api/` 默认的 `no-store`）：

   | 测定的类型 | `Content-Type` | `Content-Disposition` |
   |---|---|---|
   | 图片、音频、视频（3.5 的表，svg、pdf 之外） | 测定的类型 | `inline` |
   | `image/svg+xml` | `image/svg+xml` | `inline` |
   | `application/pdf` | `application/pdf` | `inline`（实测见下） |
   | 其余 | `application/octet-stream` | `attachment` |

   - `d=1` 一律 `attachment`；文件名按 RFC 6266：`filename*=UTF-8''<百分号编码>`，另带 ASCII 的 `filename`（非 ASCII 与引号、反斜杠换成 `_`）。
   - 每个答复：`Content-Security-Policy: sandbox; default-src 'none'; img-src 'self' data:; media-src 'self'; style-src 'unsafe-inline'`；`Cache-Control: private, max-age=<e − now>, immutable`；`ETag: "<sha256 十六进制>"`；`Cross-Origin-Resource-Policy: same-origin`。全局的 `nosniff`、`X-Frame-Options: DENY`、`Referrer-Policy: same-origin` 照旧。
4. `Sending(r, byte_size)`：答 `ErrShuttingDown` 时 503 `server_busy`（`Retry-After`）。
5. `http.ServeContent`（`Range`、条件请求；`HEAD` 由同一路由答）。日志：下载不逐个记（访问日志已有）。

**PDF 的实测**（总设计 4.5 要求写进本文）：实现之后用 Playwright 的 Chromium 与 Firefox 各打开一个签名的 PDF 地址（带上面的 CSP），看内置阅读器能否显示。不能显示的那一种照常 `inline` 也无妨（浏览器提示下载）；两种都不能，就改为按附件下载、文档与契约随之改。结果写进第 7 节。

### 3.7 元数据与列表

- `getAsset`：读节点（`AssetNodes.Node`，不是附件也算不存在）→ 判定 `asset.read`（看不到笔记本、没有角色都答 `asset.not_found`）→ 读行 → 答 `Asset`。
- `listAssets`：判定 `asset.read`（笔记本不存在或看不到：`notebook.not_found`）→ 父节点（给了就要是这本笔记本里活着的页面，否则 `page.not_found`）→ `AssetNodes.Assets` 一页 → 按节点 id 读行，拼成 `Asset`（节点有、行没有：不列，记 ERROR：违反"每个活着的附件节点恰好一行"）。游标经 `shared.EncodeCursor`，内容是最后一项的名称键与 id。

### 3.8 生命周期

- **页面观察者**（`asset.NewPageObserver(pool)`，登记在 `pageRegistrants` 的观察者末尾）：事件里 `After == nil` 的节点，`UPDATE asset_blobs SET deleted_at = 事件的时刻 WHERE node_id = ANY($1) AND deleted_at IS NULL`。删除子树时事件带每个后代，附件也在其中。
- **笔记本删除的订阅者**（`asset.NewNotebookDeletion(pool)`，登记在 `notebookRegistrants` 的删除订阅者里、页面之后）：`UPDATE … WHERE notebook_id = ANY($1) AND deleted_at IS NULL`。三条路径（删除笔记本、删除无主笔记本、删除工作区）都经它。
- 两者只凭连接池：命令行的组合（停用、删除工作区）也到达它们（`archtest` 允许）。
- **清理器**（`asset.Purgers(pool, store)`，`purgers(pool, store)` 里排在 page 之前）：一批在一个事务里——`SELECT id FROM asset_blobs WHERE deleted_at < $before ORDER BY deleted_at LIMIT $batch FOR UPDATE SKIP LOCKED` → 逐个 `store.Delete("blobs/<id>")`（不存在算成功）→ `DELETE` 这些行 → 提交。删不掉的文件让这一批失败（记 ERROR，带 `blob_id`），清理停在这里；提交失败时下一次运行再删一次文件（已不存在）、删掉行（M2/P4 的移交第 2 项）。
- **孤儿清扫**（asset 的 River 定时任务，每 24 小时；`asset.Module.Jobs()`）：`store.List("blobs", now − 24h)`，每 500 个键一批 `SELECT id FROM asset_blobs WHERE id = ANY($1)`（含软删除的），删掉没有行的文件；记 INFO（删了几个）。一天远长于文件提交到行提交的时间，不会删掉正在上传的。
- **笔记本的活动**（`asset.NewNotebookActivity(pool)`，登记在 `notebookRegistrants` 的活动里）：未删除的行的 `byte_size` 之和、最晚的 `created_at`（M3 的移交第 1 项）。
- **加锁**（总设计 4.14）：上传的单元照页面一支（工作区行 `FOR SHARE` → 笔记本行 `FOR NO KEY UPDATE` → `nodes` → `asset_blobs`）；观察者与订阅者在已持的锁下写；清理器 `SKIP LOCKED`。13.1 第 5 条写明 `asset_blobs` 在 `nodes` 之后。

### 3.9 配置与实例信息

```yaml
asset:
  # 单个附件的上限（字节），超过答 413
  max_bytes: 52428800
  # 上传与下载的最低速率（字节/秒）：流式路由的截止时间按它放宽（M7/P1）
  upload_min_rate: 65536
ratelimit:
  # 签名的下载，按客户端 IP
  asset_content: {per_minute: 6000, burst: 1000}
```

- 校验：`max_bytes` 在 1 KiB 到 4 GiB 之间；`upload_min_rate > 0`；`max_bytes / upload_min_rate ≤ 1 小时`（默认约 13 分钟：一个上传占住连接的上界）；`LogValue` 列出。test 配置把 `asset_content` 调到用不完（`TestBuiltInProfiles` 核对）。
- `InstanceInfo.asset_max_bytes`：前端在发送之前先查（P4）。

### 3.10 前端

- `PageTreeStore`：`nodes` 照旧是全部节点；`tree`（索引、子节点、祖先、拖动的深度）只取 `kind === "page"`；"未命名 N"取全部子节点的名称（附件也占名字）。附件出现在 `nodes` 里不让树重画（`sameTree` 只比页面）。
- `events/handlers.ts`：`pages` 带 `tree` 时的整树重读经 `refresher.request`（合并：一次上传一个事件，连传多个只重读一次）。
- vitest：附件不进树、不进祖先、"未命名 N"跳过附件的名字、重读经 `refresher`。

### 3.11 组合根与架构测试

- `newApp`：`pg := page.New(…)` 之后 `as := asset.New(asset.Deps{Pool, Tx, Clock, Logger, Authorizer, Store: store, Tree: pg.TreeWrites(), Nodes: page.NewAssetNodes(pool), Key: keys.Derive(asset.ContentKeyInfo), MaxBytes, MinRate, ContentBucket})`；`as.Register(router, api)`；`as.Jobs()` 进 River；`purgeJob(cfg, pool, store, logger)`。
- `pageRegistrants` 的观察者加 `asset.NewPageObserver(pool)`；`notebookRegistrants` 的删除订阅者加 `asset.NewNotebookDeletion(pool)`、活动加 `asset.NewNotebookActivity(pool)`（同 `pageActivity` 的转换）。
- `archtest`：命令行的组合到不了 `asset.New`、`storage`（`composesMore` 已禁模块根的 `New` 与 `platform/storage`）；asset 的 `app`、`domain` 不引 `net/http`、`image`、`platform/storage` 之外的平台包（照已有的分层规则）。

### 3.12 权限、交错、日志

- **权限**：`asset.upload`（写者）、`asset.read`（读者）进规则表；权限矩阵加 asset 的行（上传、列表、元数据各个角色；下载不经角色，矩阵记一行"公开，签名即凭据"）。附件的改名、移动、删除照旧是 `node.*`（矩阵已有）。
- **交错**（`interleavings_assets_test.go`，13.4 第 4 条）：上传的单元与父页的删除、与笔记本的删除、与失去写权限（各两种次序）；附件的删除与清理（清理锁住的行不被恢复……P2 没有恢复：删除的单元与清理同一行，清理 `SKIP LOCKED`）。结束时 `checkPages`（只数页面的深度）与 `checkAssets`："每个活着的附件节点恰好一行活着的 `asset_blobs`""删除了的附件节点的行已删除""每一行的文件在存储里"。
- **日志**（13.1 第 10 条）：asset 只记 `notebook_id`、`node_id`、`blob_id`、`user_id`、`mime`、`bytes`、`client`；文件名与签名不进日志（`loggedPath` 已不记查询串）。

### 3.13 e2e 与部署

- **`deletedDaysAgo`**：先挪 `asset_blobs`（`node_id IN (…)`），否则 `RESTRICT` 让已有的清理故事停住。
- **AS1（接口版本）**：上传（multipart 的 `fetch`）、列出、元数据、按签名地址下载出同样的字节；`fixtures/assert/asset.ts` 核对行与存储里的文件（按名称的 SHA-256 前两字节算分片）。
- **AS4**：删除子树、删除笔记本之后行被软删除；挪后清理之后行与文件都不在；笔记本的活动算上附件的字节。
- **AS5**：接口版本核对各类型的响应头；浏览器里直接打开 svg 的签名地址：`sandbox` 拦下脚本（控制台的 "Blocked script execution"），不向外站请求；html 附件触发下载（`waitForEvent("download")`）。
- **`image-smoke`**：管理员登录、建工作区与笔记本，上传一个小 PNG，重启容器，读元数据、按签名地址下载，字节相同（M0/P6 的移交第 3 项）。
- **README**：附件的接口一句（上传的上限、签名地址 1–2 小时）；PDF 的结论。

**取舍**（负责人可以推翻）：一个凭证同时的上传不另设上限（第 2 节）；单元的非领域错误一律不删文件、留给孤儿清扫（不区分"结果不明"与别的基础设施错误：比区分更不会丢文件）；附件放在兄弟的最后（上传不带位置，面板按名称排）。

## 4. 实施步骤

| 步 | 内容 | 提交 |
|---|---|---|
| S1 | 契约与平台：`x-raw` 的规则与测试、代码生成的排除；`storage_full`；asset 的配置与 `asset_content`；`InstanceInfo.asset_max_bytes`；规则表的两个动作 | `api, httpserver, config: raw operations, storage_full, the asset settings (M7/P2/S1)` |
| S2 | page：`TreeWrites`、附件的名称、深度只数页面、`NodeKind`、读端口；`checkPages` | `page: attachment nodes, created through a port of the wired module (M7/P2/S2)` |
| S3 | asset 的核心：迁移与授权、`Blobs`、类型测定与宽高、上传 | `asset: the attachments' rows and files, uploaded as they stream (M7/P2/S3)` |
| S4 | 签名与下载、元数据、列表；PDF 的实测 | `asset: signed downloads, the metadata and the list (M7/P2/S4)` |
| S5 | 生命周期：观察者、笔记本删除、清理器、孤儿清扫、活动；组合根；整个程序、交错、权限矩阵 | `asset, bootstrap: attachments follow their nodes, purged files first (M7/P2/S5)` |
| S6 | 前端：树只列页面、重读经 `refresher` | `web: the page tree lists pages, its reads merged (M7/P2/S6)` |
| S7 | e2e（`deletedDaysAgo`、AS1、AS4、AS5）、`image-smoke`、README；总体设计 13.1、13.4 的修订 | `e2e, deploy, docs: attachments end to end (M7/P2/S7)` |

## 5. 测试与验证

- **契约**：`x-raw` 的规则各有反例（非原样的操作用 multipart 或 `*/*`、原样的操作没有 problem 的默认答复……）；`TestRawOperationsAreNotGenerated`；契约测试照描述核对上传与下载的状态、头与媒体类型（`*/*`、206、304、416）；`storage_full` 的映射。
- **page**（app 的单元测试与存储的测试）：`CreateAsset` 的每个码（父节点不是页面、重名、名称以 `.md` 结尾、看不到的笔记本、读者）；附件放在最后；`Step.Asset` 交给守卫；`after` 的错误回滚整个单元；`Check` 与单元同判；改名的两条规则（去掉扩展名、以 `.md` 结尾；页面不受影响）；`Height` 只数页面（深度 10 的页面下可以挂附件，带附件的子树能移到深度 9 之下）；读端口的分页。
- **asset**：
  - `Blobs`：写满（`ErrFull`）、超过上限、读出错都不留文件；SHA-256 与字节数；宽高（png、jpeg、gif、坏的图片、超大的尺寸）。
  - 类型测定：容器表逐项（扩展名 × 嗅探的结果），HTML 不内联，没有扩展名，大写的扩展名。
  - 上传的处理器（表格）：字段的次序、未知与重复的字段、缺 `file`、`file` 之后还有部分、超过 4 KiB 的前置部分、名称取自文件名、413、507、预检的每个码在读文件之前答出（一个读了就失败的请求体）、领域错误删文件、非领域错误不删、停机的错误不记 ERROR。
  - 签名：已知答案的 MAC 与密钥；同一小时同一地址；改每一个参数、多出与重复的参数、非规范的写法、过期都答 404。
  - 下载：响应头的表格（每一种类型、`d=1`、非 ASCII 的文件名）；`Range` 与条件请求；行删除、文件不在、节点与 blob 对不上都 404；停机 503；不消耗 `anonymous`（`asset_content` 的桶空时 429）。
- **整个程序**（13.1 第 21 条：组合根交空时失败）：删除子树、删除笔记本（三条路径）之后行被软删除；清理先删文件、"文件已删、行没删"之后下一次完成；孤儿清扫删掉没有行的旧文件、留下新文件与有行的；活动；附件不是链接目标、改名不改写页面。
- **交错与权限**：3.12。
- **前端**：3.10 的 vitest。
- **e2e**：AS1（接口）、AS4、AS5；已有的清理故事照常通过。
- **反向对照**：附件当作一层（深度）；附件名称以 `.md` 结尾放行；改名去掉扩展名放行；`after` 的错误不回滚；观察者不登记（组合根交空）；订阅者不登记；清理器先删行后删文件；清理器不跳过锁住的行；孤儿清扫不看一天；签名不含 `d`；`e` 不核对；严格读法放过重复的参数；CSP 缺 `sandbox`；HTML 内联；领域错误不删文件；非领域错误删文件；树列出附件；整树重读不经 `refresher`。每个都要有测试失败。
- `make check`、`make gen-check`、`make e2e`、`make image-smoke`。

## 6. 完成标准

- 上传流式写入、按上限中止，预检的码在读文件之前答出；签名的地址严格核对，每种类型的响应头与 CSP 有表格测试，浏览器里 svg 不执行、html 被下载。
- 附件跟着节点、笔记本删除，清理先删文件，孤儿被清扫，活动算上附件；每条路径经组合根到达。
- 树只列页面；深度只数页面；附件的名称规则。
- `x-raw` 的契约规则与代码生成的排除有测试；`storage_full` 进平台码。
- `image-smoke` 证明附件在卷上、重启之后读得出。
- 总体设计 13.1 第 1、5、6、8、10、15、21、25、28 条，13.4 第 4、5、6 条修订；00 号文档第 7 节的出入落档。

## 7. 结果

（完成后补写。）
