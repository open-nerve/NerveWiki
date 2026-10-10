# M7/P2 附件（服务端）：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M7/P2 附件（服务端） |
| 状态 | 完成 |
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
| `server/internal/platform/httpserver/apitest/` | `x-raw` 的规则（3.2）与 `TestRawOperationsAreNotGenerated`；描述了正文的答复都要带 `Content-Type`（没有它的答复原先被当作 `*/*` 放过）。没有 `Operation.Raw`：multipart 的请求体本来就不进 `BodyCases` |
| `server/internal/platform/httpserver/` | `API.Stream`：`Sending(r)` 与按步写出的答复，处理器之前的答复关闭连接（3.6）；`httpservertest.APIOptions` 可传平台的桶 |
| `server/internal/shared/error.go` | `KindStorageFull`、`CodeStorageFull`、`StorageFull()`：507 由 `Kind` 的状态码表给出，httpserver 不改 |
| `server/internal/platform/config/` | `AssetConfig{MaxBytes, UploadMinRate}`、`RateLimitConfig.AssetContent`、校验、`LogValue` |
| `server/configs/config.yaml`、`config.test.yaml` | `asset` 一节、`ratelimit.asset_content` |
| `server/internal/modules/access/domain/rules.go` | `asset.upload`（写者）、`asset.read`（读者） |
| `server/internal/modules/page/` | `tree_writes.go`（新：`TreeWrites`、`NewAsset`）、`asset_nodes.go`（新：读端口）；`app/unit_create_asset.go`（新）、`unit_rename.go`、`domain/name.go`（新：附件的名称规则）、`domain/tree.go`（`Height` 只数页面）；`adapter/postgres/queries`（附件节点的分页） |
| `server/internal/modules/asset/`（新） | `module.go`（`New`、`Register`、`Jobs`、`ContentKeyInfo`）、`lifecycle.go`（页面观察者、笔记本删除、活动）、`purgers.go`；`domain/`（`Blob`、类型表、动作、错误）；`app/`（上传、元数据与列表、下载的用例，`Blobs`，签名的端口，清理，清扫）；`adapter/http/`（生成的两个操作 + 手写的上传与下载）、`adapter/postgres/`（sqlc）、`adapter/files/`（存储的端口）、`adapter/mac/`（签名）、`adapter/sniff/`（类型测定与宽高，`net/http`、`image` 在这里）、`adapter/river/`（清扫的定时任务） |
| `server/migrations/sql/00027_asset_asset_blobs.sql`（新）、`deploy/runtime-grants.sql` | `asset_blobs` |
| `server/internal/bootstrap/` | 组合：page → asset；`purgers(pool, tx, store, logger)`；观察者、笔记本删除、活动的登记；`instanceDeps`；整个程序的测试（`assets_*_test.go`）、交错（`interleavings_assets_test.go`）、权限矩阵（`permission_matrix_asset_test.go`）；`checkPages` |
| `server/internal/archtest/` | 命令行的组合到不了 `asset.New`；asset 的 app 与 domain 照已有的分层规则只引标准库（`net/http`、`database/sql` 除外） |
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
| `uploadAsset` `POST /api/v0/notebooks/{notebook_id}/assets`（`x-raw`） | multipart：`parent_id`（可选）、`name`（可选）、`file`（最后）；201 `Asset` | `bad_request`、`notebook.not_found`、`forbidden`、`validation_failed`（父节点不是这本笔记本里活着的页面，页模块 `Check` 的既有答法；名称的规则、`not_allowed`）、`page.title_taken`、`payload_too_large`、`storage_full`。没有 `server_busy`：停机切断读到一半的上传（3.5） |
| `listAssets` `GET /api/v0/notebooks/{notebook_id}/assets?parent_id=&cursor=&limit=` | 一个父节点（没有：根）下的附件，按名称键、id，游标分页（`limit` 1–100，默认 50） | `notebook.not_found`、`page.not_found`、`bad_request`（游标、绑定不了的参数）、`validation_failed`（`limit` 越界） |
| `getAsset` `GET /api/v0/assets/{node_id}` | 附件的元数据与签名地址 | `asset.not_found` |
| `getAssetContent` `GET /api/v0/assets/{node_id}/content?b=&e=&s=[&d=1]`（`x-raw`、公开） | 下载；200、206、304、416，`*/*` | `bad_request`（`node_id` 不是 UUID，同所有操作的路径参数）、`not_found`、`server_busy`；`rate_limited` 只写在顶层的 `x-problem-codes`（13.1 第 20 条） |

`Asset`：`id`（节点）、`notebook_id`、`parent_id`、`name`、`mime`、`byte_size`、`sha256`（十六进制）、`width`、`height`（可空）、`created_by`、`created_at`、`content_url`（内联）、`download_url`（`d=1`）、`expires_at`。地址是相对的（`/api/v0/assets/...`）。

下载的查询参数 `b`、`e`、`s`、`d` 在契约里是字符串（`e` 若是整数，整个程序的参数用例要它答 400，而下载要 404）；下载答复的 `Cache-Control`、`Content-Security-Policy`、`Cross-Origin-Resource-Policy` 写在契约的头里（components）。

**`storage_full`**：`shared.KindStorageFull` → 507，`Retry-After` 不带；平台码表、`common.yaml` 的说明、`shared.Kind` 的状态码与它的表格测试（`shared/error_test.go`）。存储的 `ErrFull` 由 asset 的 `adapter/files` 译成 `domain.ErrStorageFull`，答它。

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
    notebook_id   uuid NOT NULL,
    mime          text NOT NULL CHECK (mime <> ''),
    byte_size     bigint NOT NULL CHECK (byte_size >= 0),
    sha256        bytea NOT NULL CHECK (octet_length(sha256) = 32),
    width         integer CHECK (width > 0),
    height        integer CHECK (height > 0),
    created_by_id uuid NOT NULL REFERENCES users,
    created_at    timestamptz NOT NULL,
    deleted_at    timestamptz,
    FOREIGN KEY (notebook_id, node_id) REFERENCES nodes(notebook_id, id) ON DELETE RESTRICT,
    CHECK ((width IS NULL) = (height IS NULL))
);
CREATE INDEX asset_blobs_notebook_id_idx ON asset_blobs (notebook_id) WHERE deleted_at IS NULL;
CREATE INDEX asset_blobs_deleted_at_idx ON asset_blobs (deleted_at) WHERE deleted_at IS NOT NULL;
```

（复合外键引用 `nodes` 已有的 `nodes_notebook_id_id_key`，它也定了行的笔记本，不再单列到 `notebooks` 的外键；约束与索引照命名表起名（`asset_blobs_notebook_id_node_id_fkey`、`asset_blobs_deleted_at_idx`……）；`runtime-grants.sql` 给 `SELECT, INSERT, UPDATE, DELETE`。）

**`app.Blobs`**（`app.NewBlobs(files, rows, sniffer, logger)`，模块内部；transfer 的导入与导出用到时由模块根导出，P5、P6）：

- `Put(ctx, name, r, maxBytes) (Blob, error)`：`Create("blobs/<新 id>")`，边写边算 SHA-256，记下前 512 字节；超过 `maxBytes` 字节 `Abort`、答 `ErrTooLarge`；读出错 `Abort`、答 `*ReadError`；`Abort` 失败记 WARN（`blob_id`）；`Commit` 之后按名称的扩展名与前 512 字节测定类型（3.5），图片再 `Open` 读宽高。存储写满答 `domain.ErrStorageFull`。上传交给它的名称是 `CheckTitle` 规范化之后的（`"x.svg "` 按 svg 测定）。
- `Attach(ctx, blob)`：在调用方的事务里写行。
- `Open(ctx, nodeID, blobID) (File, Blob, error)`：按节点读活着的行；行的文件不是地址里的那个也算不存在；文件不在答 `ErrNoFile`（带着行）。
- `Drop(ctx, blob)`：删文件（单元以领域错误回滚时）。
- 端口（`app/ports.go`）：存储经 `adapter/files`（只有它引 `platform/storage`），行经 `adapter/postgres`，签名经 `adapter/mac`（3.6），类型与宽高经 `adapter/sniff`。

### 3.5 上传

处理器（`adapter/http/upload.go`）经 `API.Stream`：`MaxBytes = asset.max_bytes + Envelope`（64 KiB，multipart 的外包装），`MinRate = asset.upload_min_rate`，桶照用平台的（失败的门按 IP，`authenticated` 按凭据）。

1. `notebook_id` 在鉴权之前绑定（`bindID` 包在 `API.Stream` 外面，同生成的操作，M1/P1 3.9）：不是 UUID 答 400 `invalid_format`。`Content-Type` 不是带 boundary 的 `multipart/form-data` 答 400。
2. 逐部分读（`NextRawPart`：不解码 quoted-printable，文件的字节原样）：只认 `parent_id`、`name`、`file`，次序如此，前两者可以没有；未知、重复、次序不对是 400；`parent_id` 是 `uuid.Parse` 认的写法（大写、没有连字符也行，括号与 URN 不行），否则 400 `invalid_format`；`name` 至多 1 KiB。文件的字节之前（各部分的头与值、解析器的预读）至多 4 KiB，超过答 400；文件一开始这个限额就放开。名称：`name`，空或只有空白就用 `file` 部分的文件名（`filepath.Base` 之后），都没有 422；名称不是合法 UTF-8 的 422（`CheckTitle` 先查，表单的文件名可以这样写）。
3. **预检**（`Bounded` 之内，读文件之前）：`Tree.Check`（父节点不是这本笔记本里活着的页面答 422 `validation_failed`）与存储的余量（`Free` 低于 `storage.min_free_bytes`：507；对同时的上传是软的）。
4. `Store`（`Blobs.Put`）：超过上限 413；写满 507。读请求体失败：停机切断的（`httpserver.ErrShuttingDown`）中止连接（`http.ErrAbortHandler`），记 INFO——截止时间已切到当下，答复送不出去；其余（太慢、提前结束、连接断开、格式不对）记 INFO `upload not received`，只带原因的分类（`cause`），不带解析器引用的原文，连接还在就答 400。
5. **结尾**（`end`）：`file` 之后 `NextRawPart` 必须是 `io.EOF`；还有部分答 400 "a part after the file"，读到它的头就答，头超过路由的上限、多于解析器的 10000 行也是；之后把请求体读到结尾（P1 第 7 节第 1 项），超过路由的上限答 400。都删文件、不建节点。
6. **单元**（`Bounded` 之内）：`Tree.CreateAsset(…, after = Blobs.Attach)`。单元返回领域错误（`*shared.Error`：权限、父节点、重名，必然已回滚）时删文件（`Blobs.Drop`，删不掉记 ERROR）、答它；其余错误（基础设施、`COMMIT` 结果不明）不删，留给孤儿清扫，答 500（`COMMIT` 结果不明时行可能已提交，删了就是永远打不开的附件，总设计 4.4）。`Bounded` 的一步过了请求的期限答 500，记 WARN（`APIErrors.Write`）。
7. 答 201 `Asset`（签好的地址）；日志 `asset uploaded`（`notebook_id`、`node_id`、`blob_id`、`user_id`、`mime`、`bytes`、`client`）。

文件之前的答复（参数、表单、预检、`Store`、结尾）都带 `Connection: close`：net/http 答复之前会先读至多 256 KiB 没读的请求体，等着听答复的客户端要等到读截止时间；浏览器还在发送时可能只看到连接被重置（网页在发送之前先查，P4）。读完请求体之后的答复（单元的失败）保留连接。停机：读到一半的上传被切断；文件已写完、单元还没做的，删文件。

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

**签名**（`adapter/mac`，`app.Signer` 端口；总设计 4.5）：

- 键：`SigningKeys.Derive(asset.ContentKeyInfo)`，`ContentKeyInfo = "nervewiki asset-content mac v1"`，由模块根导出，组合根经 `Deps.ContentKey` 交给 `adapter/mac`，派生的键不进 app 层；已知答案的测试钉住（13.1 第 25 条）。
- 按调用方给的时刻签与核对：一次操作只读一次时钟（13.1 第 9 条；列表的各项用同一时刻）。
- `s = base64url_nopad(HMAC-SHA256(key, "asset-content" ‖ node 16 字节 ‖ blob 16 字节 ‖ e 8 字节大端 ‖ d 1 字节)[:16])`，22 个字符。
- `e = (⌊now / 1h⌋ + 2) × 1h`（Unix 秒）：同一个小时签出的地址相同，有效 1–2 小时。
- 只有核对过读权限的读取签：元数据、列表、上传的答复（P3 起加阅读视图）。

**下载**（`adapter/http/content.go`）经 `API.Stream`：公开、桶 `ratelimit.asset_content`（按 IP，`BucketName: "asset_content"`），`MinRate = asset.upload_min_rate`，没有请求体。

1. **严格的读法**（任何查询之前）：路径的 `node_id` 不是 UUID 答 400（同所有操作的路径参数），不是规范写法（小写、带连字符）或经过转义（`RawPath` 不空）答 404；查询串依次是 `b`、`e`、`s`，可以再有 `d=1`，键名逐个对位，至多四段；`b` 是规范写法的 UUID；`e` 是规范的十进制（没有前导零、符号）；`s` 恰好 22 个 base64url 字符；签名对；`e` 晚于现在。任何一项不对都答 404 `not_found`（`no-store`，与"不存在"不分）。
2. 先读节点（活着、是附件），再读行（blob 对得上、没有软删除），名称取自节点；否则 404。文件不在：404，记 WARN（`asset file missing`，`blob_id`）。
3. 响应头（签名核对通过之后才设，覆盖 `/api/` 默认的 `no-store`）：

   | 测定的类型 | `Content-Type` | `Content-Disposition` |
   |---|---|---|
   | 图片、音频、视频（3.5 的表，svg、pdf 之外） | 测定的类型 | `inline` |
   | `image/svg+xml` | `image/svg+xml` | `inline` |
   | `application/pdf` | `application/pdf` | `inline`（实测见下） |
   | 其余 | `application/octet-stream` | `attachment` |

   - `d=1` 一律 `attachment`；文件名按 RFC 6266：`filename*=UTF-8''<百分号编码>`，另带 ASCII 的 `filename`（非 ASCII 与引号、反斜杠换成 `_`）。
   - 每个答复（含 400、404、429、503，内容路由的最外层 `sandboxed` 设）：`Content-Security-Policy: sandbox; default-src 'none'; img-src 'self' data:; media-src 'self'; style-src 'unsafe-inline'`、`Cross-Origin-Resource-Policy: same-origin`。签名核对通过之后：`Cache-Control: private, max-age=<e − now>, immutable`；`ETag: "<sha256 十六进制>"`；`Last-Modified`。全局的 `nosniff`、`X-Frame-Options: DENY`、`Referrer-Policy: same-origin` 照旧。
   - 内联与否按表判断（`domain.Served`、`domain.Inline`）：表外的类型一律 `application/octet-stream` 与 `attachment`。
4. `Sending(r)`（平台改为不带字节数）：答复按步写出，每步之前把写截止时间设为"宣告时刻 + `read_timeout` + 已写字节 / `MinRate`"，停读的客户端在 `read_timeout` 加缓冲住的字节应得的时间之后断开（暂停的媒体元素靠浏览器的 `Range` 重新请求）。答 `ErrShuttingDown` 时 503 `server_busy`（`Retry-After: 5`，`no-store`，不带文件的头）。
5. 去掉 `If-Match`、`If-Unmodified-Since`（地址所下发的内容从不改变，没有要守的；留着它们 `ServeContent` 会答 412，带着文件的头），再 `http.ServeContent`（`Range`、`If-None-Match`、`If-Modified-Since`；`HEAD` 由路由在 GET 上答，契约不单列）。`Bounded` 的一步过了请求的期限答 500，记 WARN。日志：下载不逐个记（访问日志已有）。

**PDF 的实测**（总设计 4.5 要求写进本文）：Playwright 的 Chromium 带完整的 CSP（含 `sandbox`）能显示，保持 `inline`；本机没有 Playwright 的 Firefox，没测。README 写"其他浏览器未实测；显示不了时用 `download_url` 下载"。结果见第 7 节。

### 3.7 元数据与列表

- `getAsset`：读节点（`AssetNodes.Node`，不是附件也算不存在）→ 判定 `asset.read`（看不到笔记本、没有角色都答 `asset.not_found`）→ 读行 → 答 `Asset`。
- `listAssets`：判定 `asset.read`（笔记本不存在或看不到：`notebook.not_found`）→ 父节点（给了就要是这本笔记本里活着的页面，否则 `page.not_found`）→ `AssetNodes.Assets` 一页 → 按节点 id 读行，拼成 `Asset`（节点有、行没有：不列；再读一次节点，还活着才记 ERROR，违反"每个活着的附件节点恰好一行"，两次读之间被删的不算。`getAsset` 同）。游标经 `shared.EncodeCursor`，内容是最后一项的名称键与 id。

### 3.8 生命周期

- **页面观察者**（`asset.NewPageObserver(pool)`，登记在 `pageRegistrants` 的观察者末尾）：事件里 `After == nil` 的节点，`UPDATE asset_blobs SET deleted_at = 事件的时刻 WHERE node_id = ANY($1) AND deleted_at IS NULL`。删除子树时事件带每个后代，附件也在其中。
- **笔记本删除的订阅者**（`asset.NewNotebookDeletion(pool)`，登记在 `notebookRegistrants` 的删除订阅者里、页面之后）：`UPDATE … WHERE notebook_id = ANY($1) AND deleted_at IS NULL`。三条路径（删除笔记本、删除无主笔记本、删除工作区）都经它。
- 两者只凭连接池，命令行的组合也建得到（`archtest` 允许）；命令行没有删除笔记本或工作区的命令，经它们的是 serve 的三条路径。
- **清理器**（`asset.Purgers(pool, tx, store, logger)`，`purgers(pool, tx, store, logger)` 里排在 page 之前：一批一个事务要 `TxManager`，删不掉的文件记 ERROR 要 logger）：一批在一个事务里——`SELECT id FROM asset_blobs WHERE deleted_at < $before ORDER BY deleted_at LIMIT $batch FOR UPDATE SKIP LOCKED` → 逐个删文件（不存在算成功）→ `DELETE` 这些行 → 提交；空批不 `DELETE`。删不掉的文件让这一批失败（记 ERROR，带 `blob_id`），清理停在这里；提交失败时下一次运行再删一次文件（已不存在）、删掉行（M2/P4 的移交第 2 项）。
- **孤儿清扫**（asset 的 River 定时任务，每 24 小时；`asset.Module.Jobs()`，`adapter/river`）：存储里 24 小时之前的 `blobs` 的文件，每 500 个键一批 `SELECT id FROM asset_blobs WHERE id = ANY($1)`（含软删除的），删掉没有行的文件；删了的记 INFO（几个）。删不掉的记 WARN（`blob_id`），接着删其余，最后答错误；区里不是 blob 的文件记 WARN、留下。一次运行的时限 `SweepTimeout` 50 分钟（River 默认的 1 分钟扫不完大的存储；低于 River 默认 1 小时的 `RescueStuckJobsAfter`，免得还在跑就被当作卡住的任务重跑）。一天远长于文件提交到行提交的时间，不会删掉正在上传的。
- **笔记本的活动**（`asset.NewNotebookActivity(pool)`，登记在 `notebookRegistrants` 的活动里）：未删除的行的 `byte_size` 之和（M3 的移交第 1 项）。不报最晚的上传：每次上传是树的单元，页面的来源已按它的变更集算作同一时刻的写（审查 C3）。
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
- `events/handlers.ts`：`pages` 带 `tree` 时的整树重读经 `refresher.request`，间隔 500 毫秒（`TREE_INTERVAL_MS`；`refresher` 按键取间隔）：一连串的单元合成每秒至多两次读，别人连续改树时至多晚半秒显示（审查 C1，原定的 5 秒太迟）。
- `nodes` 每次读都换；树取自带相等比较的计算值（只比页面），附件变了树不重画；新页的"未命名 N"看页面与附件。
- vitest：附件不进树、不进祖先、"未命名 N"跳过附件的名字、重读经 `refresher`。

### 3.11 组合根与架构测试

- `newApp`：`pg := page.New(…)` 之后 `as := asset.New(asset.Deps{Pool, Tx, Clock, Logger, Authorizer, Store: store, Tree: pg.TreeWrites(), Nodes: page.NewAssetNodes(pool), ContentKey: keys.Derive(asset.ContentKeyInfo), MaxBytes, MinRate, ContentBucket})`；`as.Register(router, api)`；`as.Jobs()` 进 River；`purgeJob(cfg, pool, store, logger)`。
- `pageRegistrants` 的观察者加 `asset.NewPageObserver(pool)`；`notebookRegistrants` 的删除订阅者加 `asset.NewNotebookDeletion(pool)`、活动加 `asset.NewNotebookActivity(pool)`（同 `pageActivity` 的转换）。
- `archtest`：命令行的组合到不了 `asset.New`、`storage`（`composesMore` 已禁模块根的 `New` 与 `platform/storage`）；asset 的 `app`、`domain` 照已有的分层规则只引标准库（`net/http`、`database/sql` 除外）。`image` 是标准库、不禁，实际只在 `adapter/sniff` 里用。

### 3.12 权限、交错、日志

- **权限**：`asset.upload`（写者）、`asset.read`（读者）进规则表；权限矩阵加 asset 的行（上传、列表、元数据各个角色；下载不经角色，矩阵记一行"公开，签名即凭据"）。附件的改名、移动、删除照旧是 `node.*`（矩阵已有）。
- **交错**（`interleavings_assets_test.go`，13.4 第 4 条）：上传的单元与父页的删除、与笔记本的删除、与失去写权限（各两种次序）；附件的删除与清理（清理锁住的行不被恢复……P2 没有恢复：删除的单元与清理同一行，清理 `SKIP LOCKED`）。结束时 `checkPages`（只数页面的深度）与 `checkAssets`："每个活着的附件节点恰好一行活着的 `asset_blobs`""删除了的附件节点的行已删除""每一行的文件在存储里"。
- **日志**（13.1 第 10 条）：asset 只记 `notebook_id`、`node_id`、`blob_id`、`user_id`、`mime`、`bytes`、`client`；文件名与签名不进日志（`loggedPath` 已不记查询串）。

### 3.13 e2e 与部署

- **`deletedDaysAgo`**：先挪 `asset_blobs`（`node_id IN (…)`），否则 `RESTRICT` 让已有的清理故事停住。
- **AS1（接口版本）**：上传（multipart 的 `fetch`）、列出、元数据、按签名地址下载出同样的字节；`fixtures/assert/asset.ts` 核对行与存储里的文件（按名称的 SHA-256 前两字节算分片）。
- **AS4**：删除子树、删除笔记本之后行被软删除；挪后清理之后行与文件都不在；笔记本的活动算上附件的字节。
- **AS5**：接口版本核对各类型的响应头；浏览器里直接打开 svg 的签名地址：`sandbox` 拦下脚本（控制台的 "Blocked script execution"，按条声明）；SVG 里外站的图片与 `@import` 都被 CSP 拦下（Chromium 报为失败的请求，原因 `csp`）、没有答复；html 附件触发下载（`waitForEvent("download")`）。
- **`image-smoke`**：管理员登录、建工作区与笔记本，上传一个小 PNG，重启容器，用重启之前签的地址下载（密钥由 JWT 的私钥导出，不随重启变），字节与大小相同（M0/P6 的移交第 3 项）。
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

- 提交：
  - 实施：pgtest 的修正 `613a7fa`（main 上 `5a94218` 的 CI 偶发：查表的 OID 在截止之前没拿到连接，答原样的 "context deadline exceeded"；改为截止之后交给第一次轮询报统一的消息），S1 `7d44789`、S2 `d64fa80`、S3 `3720da9`、S4 `943f410`、S5 `87f8621`、S6 `b8415f6`、S7 `01603e6`，负对照的修补 `53facf6`。
  - 审查的修复：`9b3fc00`、`fd371d5`、`ad071d0`、`39d0d44`。
  - 九轮修复核对的修复：`dda31f0`、`8fe29c4`、`e804286`、`302df06`、`a838cd0`、`10318f2`、`c4b364e`、`550c8dd`、`a2a5658`、`1e2090d`。
  - 合并 `48c62c0`。
- 审查：三位审查者（Opus）。
  - A（服务端的正确性与并发）：Medium-low 2（读与删并发时误报 ERROR；清扫在 River 默认 1 分钟的时限下扫不完、一个删不掉的文件停下整轮），Low 6，需核实 1。
  - B（接口的边缘与文件的提供）：Medium 1（不是合法 UTF-8 的文件名绕过名称校验），Medium-low 2（下载的写截止时间一次设定，停读的客户端占住连接；`If-Match` 让 `ServeContent` 答 412），Low 10。
  - C（测试、前端、e2e、部署与文档）：Medium 1（树的重读最多晚 5 秒），Medium-low 3，Low 7。
  - 修复核对九轮：第一到三轮核对修复本身；第四到八轮用变异扫描把 P2 的服务端代码逐层扫过（asset 的 http、app、domain 与各适配器，page 为附件做的改动，平台 `API.Stream` 与契约规则的改动），找到的都是测试抓不住的改变行为的回退，每一处都补了测试，只有一处契约不符（第五轮：文件之后的部分的头超过路由上限时答 413）；第九轮没有行为问题。逐条见[审查记录](reviews/P2-assets-server-review.md)。
- 审查之后改了的设计（第 3 节已改写）：
  - 签名挪进 `adapter/mac`，按调用方给的时刻签与核对，一次操作只读一次时钟；派生的键不进 app 层。
  - 下载：路径与查询按服务端的写法严格地读（转义过的路径、别的次序、别的键名都答 404）；CSP 与 CORP 在内容路由的最外层，每个答复都带；`If-Match`、`If-Unmodified-Since` 先去掉；`Sending(r)` 不再带字节数，答复按步写出（平台，总设计 4.3）。
  - 上传：`NextRawPart`（不解码 quoted-printable）；文件之前至多 4 KiB，文件一开始放开；空的 `name` 取文件名，不是 UTF-8 的名字 422；文件之后的部分读到头就答 400（头超过路由上限或解析器的 10000 行也是），请求体超过路由上限 400；处理器之前的答复带 `Connection: close`（平台）；停机切断读到一半的上传。
  - 生命周期：清扫的时限 50 分钟、删不掉的文件记 WARN 接着删；`ExpiredBlobs` 在事务之外拒绝；活动只报字节数。
  - 前端：树的重读 500 毫秒（`refresher` 按键取间隔）。
- PDF：Chromium 带完整的 CSP 能显示；本机没有 Playwright 的 Firefox，没测，README 写"其他浏览器未实测，显示不了时用 `download_url`"。留给负责人的人工清单顺带看一眼。
- 反向对照：实施时 25 个（服务端 20、前端 4、浏览器 1），修复与核对约 245 个，都被测试抓到。
- CI 与发布：分支的 CI 在每个修复提交上全部通过（server、web、image、e2e；`image` 一步跑 `make image-smoke`）。审查修复之后在本机跑过 `make image-smoke`（`dda31f0`）；合并时本机的 Docker Desktop 起不来，合并之后的 `image-smoke` 由 main 的 CI 跑：文档提交 `4926432` 的 CI（run 37811108041）全部通过，含 `image-smoke`。
- 交给后面的：
  1. P3：附件进解析、链接与渲染；指向附件的写法现在是未解析的链接（有测试钉住）。
  2. P4：附件的面板与上传；上传前先查 `InstanceInfo.asset_max_bytes`，浏览器还在发送时被拒，可能只看到连接被重置（3.5）；PDF 的 Firefox。
  3. P5、P6：导入与导出用 `app.Blobs`，届时由模块根导出；`InstanceInfo.import_max_bytes` 随 P6。
  4. 观察到一次 M6 的偶发（`TestAHeartbeatAfterTheRenamesPrecheckRefusesTheRename` 答 200，单独跑 35 次全过），CI 再出现就查。
- 负责人可以改判的取舍：`storage.min_free_bytes` 对同时的上传是软的（B10）；停读的媒体元素在 `read_timeout` 加缓冲的字节应得的时间之后断开，靠浏览器的 `Range` 重新请求（B2）；`SweepTimeout` 50 分钟（低于 River 的 1 小时）；缺结尾 `--` 的请求体被接受（文件完整）；`parent_id` 认大写与没有连字符的 UUID，不认括号与 URN；一个凭证同时的上传不另设上限；单元的非领域错误一律不删文件、留给孤儿清扫。
