# M7/P6 导入：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M7/P6 导入 |
| 状态 | 进行中（A） |
| 基线 | `27e5348`（P5 合并、文档补完之后的 main）；本文提交之后开分支 `m7-p6a`，A 合并之后开 `m7-p6b` |
| 上级文档 | [M7 总设计](00-M7-design.md) 4.1、4.2（变更集的类型与并入、先解析后单元）、4.8、4.9、4.11–4.14、第 5、7–9 节；[P5 文档](05-P5-export.md)第 8 节（交给 P6 的各项）；移交：[M6 链接与附件、导入、导出](handoffs/M6-links.md)第 4 项；总体设计 13.1 第 1、2、5、16、19、31 条，13.4 第 4 条 |

---

## 0. 分两部分

P6 把一个 zip（Obsidian 的库、本系统的导出、别的 Markdown 的 zip）导入到一本笔记本的某个位置。照 P5 的先例分开实现、审查、核对修复、合并：

- **A：服务端**：page 的变更集类型与并入、导入的写入单元（`TreeWrites` 的 `Parse`、`Import`）、名称的修正与序号；asset 的写端口；三个模块的统计端口与 `MAINTAIN`；平台的 multipart 表单读法（从 asset 移出，两处共用）；transfer 的开始导入、读 zip（结尾记录）、校验、映射、分批写入、报告、取消、收拾与清扫；契约、配置、`InstanceInfo.import_max_bytes`；Obsidian 样例与导出再导入的核对。e2e：TR2–TR4 的接口版本。
- **B：前端**：导入的服务（经上传的传输）、设置里的导入对话框（选文件、选位置、进度与取消）、任务行与报告认导入的码。e2e：TR2–TR4 的页面版本。

第 1、2 节两部分共用，第 3 节是 A 的设计，第 4 节是 B 的，第 5–9 节共用。实施之后第 3、4 节照实际改写，差异列在第 9 节。

## 1. 基线

- **page**：`TreeWrites` 只有 `Check` 与 `CreateAsset`（P2）；写入单元一个单元一个变更集，`changesets.kind` 只有 `edit`，`UnitSpec` 没有类型与并入；`ContentParser` 在单元之前按预算解析（`Budget.Take`，取不到等一会儿答 503）；`app.Facts` 对 page 不透明，只有 `Tasks` 看进去；`CreatePage` 每次读一遍兄弟与祖先，撞名答 409。
- **asset**：模块根的 `NewBlobs(pool, store)` 只有 `Of`、`Open`（P5A）；`Put`（流式写入、算 SHA-256、测定类型与宽高）、`Attach`、`Drop` 只在模块之内给上传用；上传的 multipart 读法（先读文件之前的部分、判定之后才读文件、文件之后不许再有部分）在 `asset/adapter/http/upload.go`。
- **transfer**：只有导出：`StartExport`、`Export`（心跳、结束、失败码都写在 `export.go` 的 `run` 里）；收拾只核对排队的导出（`QueuedExports`、`Held` 只读 `transfer.export`）；清扫只扫 `exports/`；报告的 `Counts.Skipped` 恒为 0；`Archives` 能写导出的 zip、读、删、列；唯一索引只有"每人每本笔记本一个导出"。`shared.Actor` 有 `JobID`，没有 `Valid()`。
- **统计**：没有模块给出 `ANALYZE` 的端口；`runtime-grants.sql` 只给 `river_job` 的 `MAINTAIN`。
- **实例信息**：`asset_max_bytes`、`export_ttl_seconds`。
- **Go 的 `archive/zip`**（Go 1.27.1，读过源码）：`NewReader` 从中央目录的起点一直读目录记录，直到读到不是目录记录的签名为止，结尾记录里的条目数只比较低 16 位；预分配看条目数与文件大小，但读的条数不受结尾记录约束：512 MiB 的 zip 可以有约一千一百万条，每条一个 `*File`。`zipinsecurepath` 默认不查路径（`..`、绝对路径、`\` 照样交出来）。没有 UTF-8 标志、而名称是含非 ASCII 的合法 UTF-8 时 `NonUTF8` 为真（macOS 的"压缩"就是这样写的）。
- **前端**：设置的"导入与导出"有导出一节与任务列表（P5B），任务行已认 `kind: import` 的标题（"导入「名称」"），没有导入的入口；`services/upload-fetch.ts` 是附件上传用的 `XMLHttpRequest` 传输（总设计 4.8："导入的 zip 用同一个"）。

## 2. 目标与范围

**做**（总设计第 7 节 P6 一行）：第 0 节的两部分；13.1 第 1、2 条（导入的单元、并入变更集）、第 5 条（导入任务的创建）、第 16 条（`MAINTAIN`）、第 19 条（单元之前解析）、第 31 条（单元的上限），13.4 第 4 条（导入的交错）随实现改写。[P5 文档](05-P5-export.md)第 8 节交给 P6 的各项、M6 移交第 4 项在这里落实。

**不做**：

- 名称不是 UTF-8 的条目按 GBK、CP437 解读（跳过并写进报告，M12 的移交，总设计第 2 节）。
- 导入时改写链接（总设计第 2 节）：正文逐字节保留；改过名称的进报告。
- 导入的整组撤销（M8 的移交）；命令行的导入（总设计第 2 节）。
- 网页上传文件夹（只收 zip）；导入到附件之下（导入的位置是根或页面）。
- 页面菜单的"导入到此页"：导入对话框里选位置（总设计 4.8），入口只在设置里；多一个入口留到 v0.1 收官之后的打磨。

## 3. A：服务端

实施之前的设计；实施、审查与修复核对之后照实际改写。

### 3.1 文件

| 位置 | 内容 |
|---|---|
| `shared/title.go`、`shared/actor.go` | `FixTitle`：把任意字符串修成合法的标题（3.4）；`Actor.Valid()`：恰好一种凭据 |
| `platform/httpserver/form.go` | multipart 表单的读法，从 asset 的上传移来（3.7）；asset 的上传改用它 |
| `migrations/sql/00030_page_changesets_import.sql` | `changesets.kind` 加 `import` |
| `modules/page/app/unit.go`、`unit_import.go`、`import_writes.go` | `UnitSpec.Kind`、`UnitSpec.Changeset` 与并入的核对；导入的单元（兄弟与祖先的缓存、撞名加序号、太深时不写）；`ImportWrites`（`Parse`、`Import`） |
| `modules/page/domain/name.go` | `Numbered`：撞名时的序号写法 |
| `modules/page/adapter/markdown/markdown.go` | `Links(facts)`：一份正文的链接数（单元的上限） |
| `modules/page/tree_writes.go`、`export_nodes.go`、`statistics.go` | `TreeWrites` 加 `CheckContent`、`Parse`、`Import`；读端口加 `Depth`；`NewStatistics(pool)` |
| `modules/linking/statistics.go`、`modules/asset/statistics.go` | 各自表的 `ANALYZE` |
| `modules/asset/blobs.go` | `NewBlobs(pool, store, logger)` 加 `Put`、`Attach`、`Drop` |
| `modules/transfer/domain` | 导入的动作、失败与问题的码；条目的分类（`entry.go`）；映射（`import_plan.go`）；`meta.json` 的读法 |
| `modules/transfer/adapter/archive` | 导入的 zip：写入（上传）、打开、结尾记录与目录的预读（`zipdir.go`） |
| `modules/transfer/app` | `StartImport`、`Import`；导出与导入共用的任务骨架（`job_run.go`：心跳、结束、失败码）；收拾与清扫认导入 |
| `modules/transfer/adapter/postgres`、`river`、`http` | 导入的语句；`transfer_import` 队列与 worker；`startImport` 的处理器（`x-raw`） |
| `migrations/sql/00031_transfer_one_import.sql` | "每本笔记本一个导入"的唯一索引 |
| `api/modules/transfer.yaml`、`instance.yaml` | `startImport`；失败与问题的码；`InstanceInfo.import_max_bytes` |
| `platform/config` | `transfer.import_*`、`jobs.import_workers` 与交叉规则 |
| `bootstrap/transfer.go`、`deps.go` | 组合：`TreeWrites`、asset 的写端口、三个统计端口、导入的配置与队列 |
| `deploy/runtime-grants.sql` | page、linking、asset 的表加 `MAINTAIN` |
| `web/apps/web/src/app/problem-messages.ts`、`i18n` | 开始导入可能答的错误码的文案（其余随 B） |

### 3.2 page：变更集的类型与并入

- **迁移**（归 page，13.1 第 7 条）：`changesets_kind_check` 改为 `kind IN ('edit', 'import')`。
- **`UnitSpec.Kind`**：`edit`（零值）或 `import`；新建的变更集照它写。
- **`UnitSpec.Changeset`**：非零时单元不建变更集，并入这一个：在笔记本行的锁之下、第一次写之前锁住那一行（`FOR NO KEY UPDATE`），核对它属于这本笔记本、没删、类型与 `Kind` 相同、`created_by_id` 是这个单元的执行者、`client` 相同，不合答错误（不是用户的错：调用方的缺陷）；之后 `TouchChangeset`，笔记本的活动与日志的次序看得到之后的批。编辑会话的写不能并入导入的变更集（类型不同；`changesetOf` 照旧只认会话自己的）。
- `Outcome.ChangesetID` 照旧答出：导入把第一个单元的交给之后的单元。13.1 第 2 条改为"一个写入单元一个变更集，导入的各个单元并入第一个单元的"。
- **`Actor.Valid()`**：`Writer.Run` 与 `Allowed` 先核对，凭据不是恰好一种时答错误（编程错误，不是 401）。

### 3.3 page：导入的写入端口

`(*page.Module).TreeWrites()` 加三个方法（接好线的模块给出，13.1 第 11 条的例外照旧）：

- **`CheckContent(content) error`**：`domain.CheckContent` 的规则（5 MiB、UTF-8、没有 NUL），不碰数据库：校验那一遍用它，规则只有一处。
- **`Parse(ctx, content) (Parsed, error)`**：`ContentParser.Decided` 那一支：预算排队取正文的字节（`Budget.Take`，此时不持锁，13.1 第 19 条），解析，留下提取结果那一份；`Parsed` 带着提取结果、`Links()`（链接数：page 的 markdown 适配器数 obsidian 扩展取到的链接，同 `Tasks` 的做法）与 `Release()`。取不到预算答 `shared.ServerBusy`，调用方退避重试（3.13）。
- **`Import(ctx, spec ImportSpec, do func(ctx, u ImportUnit) error) (uuid.UUID, error)`**：跑一个写入单元：`Tree: true`、`Kind: import`、动作是 `spec.Action`（`transfer.import`，写者）、客户端是 `spec.Client`（任务记下的）、`spec.Changeset` 非零时并入；答出单元的变更集。`do` 里只有两种操作：
  - **`CreatePage(ctx, ImportedPage{ParentID, Name, Content, Parsed}) (NodeInfo, error)`**：照 `CreatePage` 的规则，另有三处不同：
    - 名称撞上兄弟（按标题键，原有的子节点与这个单元先建的都算）时不答 409，取 `Numbered(name, 2)`、`3`……里第一个空着的；答出的 `NodeInfo.Name` 是实际的名称。名称的规则（`CheckTitle`）照旧检查，不合答 422：调用方已修正过（3.4）。
    - 深度超过 10 时答 `domain.ErrTooDeep`，此前什么都没写：调用方把这一页与它的子孙记成跳过，单元里其余的照常（3.13）。
    - 兄弟与祖先按父节点在单元里读一次、缓存，之后新建的接在缓存上（一个单元至多 100 个节点；一个父节点下几千个子节点时，不必每建一个读一遍）；单元持着笔记本行的 `FOR NO KEY UPDATE`，别的树写进不来，缓存在单元里一直对。重排次序（`placeAmong` 的 `renumber`）时缓存随之更新。
  - **`CreateAsset(ctx, ImportedAsset{ParentID, Name, Meta}, after) (NodeInfo, error)`**：照 `CreateAsset`，名称同样加序号（在扩展名之前），同样用缓存；`after` 在同一个事务里写附件的行（asset 的 `Attach`）。
  - 每个新建的节点照常经守卫、参与者、观察者：新的页经 linking 的观察者进索引，之后建的页让先前解析不到的链接重新解析到它们（M6 的观察者按新节点的名称键找）；每个单元发一次 `pages` 事件（`tree: true`），前端的整树重读已经合并（P2）。
- **读端口**：`page.NewExportNodes(pool)` 加 `Depth(ctx, notebookID, pageID) (int, bool, error)`：活着的页的深度（根下的页是 1），附件与不在的答 `false`。开始导入时核对位置，任务运行时据它算"超过深度 10 的部分"。
- 组合根交给 transfer 的是 `TreeWrites` 的适配（`bootstrap/transfer.go`）；命令行的组合照旧到不了它（`archtest/composition_test.go` 已有这条断言）。

### 3.4 名称的修正与序号

- **`shared.FixTitle(s string, keepExtension bool) string`**（与 `CheckTitle` 放在一起，规则只有一处）：非 UTF-8 的字节换成 `U+FFFD` 再处理（导入只对合法 UTF-8 的名称调用）；NFC；禁止的字符（`/ \ : * ? " < > | # ^ [ ]`）与不可显示的字符（`unshowable`）换成 `_`；反复去掉首尾的空白与 `.`；Windows 保留名在第一个 `.` 之前加 `_`（`con.txt` → `con_.txt`）；超过 255 字节时在字符边界截短，`keepExtension` 时保留最后一个 `.` 起的扩展名（扩展名本身超过 255 字节时整体截短）；截短之后再去一次尾部的空白与 `.`。全部去掉时答 `""`，调用方给默认名。性质测试：任意输入的结果要么是 `""`，要么过 `CheckTitle` 且等于 `CheckTitle` 的输出（已经合法的名称不变）。
- **页面与附件**：条目的最后一段先去掉首尾的空白与 `.`，以 `.md`（不分大小写）结尾、前面还有字符的是页面，标题是去掉 `.md` 的部分；否则是附件（总设计 4.11："去掉之后再判断"）。之后页面的标题经 `FixTitle(_, false)`、附件的名称经 `FixTitle(_, true)`；附件的结果以 `.md` 结尾时（理论上到不了）再加 `_`。空的叫"未命名"（总设计 4.11；不随界面语言：它是存下来的内容，负责人可以改判）。文件夹的名称照页面。
- **序号**（`page/domain.Numbered(name, n, asset)`）：页面写 `name n`；附件在扩展名之前写（`a 2.png`；没有扩展名的照页面）。超过 255 字节时截短名称的主体，保留序号与扩展名；结果过 `CheckTitle`（表格与性质测试）。
- **报告**：最终的名称与 zip 里的原名（NFC 之后）不同的，记一条 `renamed`：`path` 是 zip 里的路径，`to` 是导入之后在笔记本里的路径（导入位置之下的名称以 `/` 连接）。只差 NFC 的不算改名（标题键本来就在 NFC 之后比较，指向它的链接照样解析到）。

### 3.5 asset 的写端口

`asset.NewBlobs(pool, store, logger)`（多一个 logger：`Put` 中止失败要记）加：

- `Put(ctx, name, r, maxBytes) (Blob, error)`：照上传的 `Blobs.Put`（边写边算 SHA-256，按名称与开头的字节测定类型，图片读宽高）；超过 `maxBytes` 答 `ErrTooLarge`，存储写满答 `ErrStorageFull`，`r` 读失败原样带出；不留文件。
- `Attach(ctx, blob, node)`：在调用方的事务里写行（经 `postgres.DB(ctx, pool)`），节点、笔记本、上传者、时刻取自节点。
- `Drop(ctx, blob)`：删掉没有行的文件。
- `Blob` 加 `MIME`、`Bytes`、`SHA256`（交给守卫的 `AssetMeta`）。

transfer 的 app 另起一个端口 `Attachments`（`Put`、`Attach`、`Drop`），与读的 `Blobs` 分开；组合根把同一个 `asset.Blobs` 适配给两者。

### 3.6 统计与 `MAINTAIN`

- page、linking、asset 的模块根各给出 `NewStatistics(pool)`，`Analyze(ctx)` 对本模块导入会写到的表 `ANALYZE`（page：`nodes`、`page_contents`、`page_revisions`、`changesets`、`changeset_items`；linking：`indexed_pages`、`page_links`、`page_tags`、`page_properties`、`page_aliases`；asset：`asset_blobs`）。transfer 不对别人的表执行 SQL（M6 移交第 4 项、总设计 4.11），经组合根交来的 `[]Analyzer` 调用。语句经 sqlc（`rawsql_test`）；sqlc 解析不了 `ANALYZE` 时在实施时定下替代并写进第 9 节。
- `deploy/runtime-grants.sql` 给这些表 `MAINTAIN`（PostgreSQL 17 起；`ANALYZE` 要它或属主）；`runtime_role_test.go` 跑一次导入，断言这些表的 `pg_stat_user_tables.last_analyze` 前进了。
- 导入每写 10,000 个节点与结束时调用（3.13）；失败只记警告、不让导入失败（统计是优化），运行时角色的测试经 `last_analyze` 抓到没给的授权。

### 3.7 平台：multipart 表单的读法

asset 的上传与导入读同一种表单：几个小的文字部分、一个文件部分（流式）、之后什么都没有。把 `asset/adapter/http/upload.go` 的 `form`、`preface`、`value`、`end`、`readCause` 移到 `platform/httpserver/form.go`（`Form`：`NewForm(r, maxPreface)`、按给定的次序读文字部分各至多一次、`File()` 交出文件部分、`Open()` 之后才放行文件的字节、`End(r)`；`BodyReadError` 取代 asset 的 `ReadError`），asset 的处理器改用它，行为不变（它的 `upload_body_test.go` 原样通过）。各自的 `early`、`readFailed` 留在模块里（错误到答复的映射是各自的）。

### 3.8 契约

`api/modules/transfer.yaml`：

- **`startImport`**：`POST /api/v0/notebooks/{notebook_id}/imports`，`x-raw`（代码生成排除，契约测试按描述核对），`multipart/form-data`：`parent_id`（可省，导入到根下）、`file`（zip），依次、各至多一次；答 202 `TransferJob`。
  - 错误码：`notebook.not_found`、`forbidden`（读者）、`page.not_found`（`parent_id` 不是这本笔记本里活着的页）、`transfer.busy`（409：这本笔记本已有排队或运行中的导入，任何人的）、`server_busy`（503，排队的任务已满，带 `Retry-After`）、`storage_full`（507）、`payload_too_large`（413，zip 超过 `transfer.import_max_bytes`）、`bad_request`（400：表单不对、请求体没读完整）。
  - 任务的 `root_id` 是导入的位置（`null`：根），`name` 是上传的文件名（经 `FixTitle(_, true)`，空的叫"未命名"）。
- **`TransferFailure`** 加：`not_zip`（不是 zip，或结尾记录、中央目录坏了）、`too_many_entries`（条目超过 `transfer.import_max_entries`，或中央目录超过上限）、`unpacked_too_large`（解压后的总字节数超过 `transfer.import_max_unpacked_bytes`）、`tree_changed`（导入途中，已导入的页被删除或移走，后面的写不下去）；`root_not_found` 的说明加上"导入的位置已不存在"。
- **`TransferProblem.code`** 加：`unsafe_path`（越出根：`..`、绝对路径、盘符）、`special_file`（符号链接与其他特殊文件）、`encrypted`、`unsupported_method`（不是 Store、Deflate）、`too_compressed`（压缩比超过 200）、`name_not_utf8`、`invalid_content`（`.md` 不是 UTF-8 或含 NUL）、`too_large`（`.md` 超过 5 MiB、附件超过 `asset.max_bytes`）、`too_deep`（相对导入位置超过深度 10）、`duplicate`（同一路径的第二个条目）、`unreadable`（解压失败、校验和不对、大小与条目头不符）。导入的 `renamed`：`path` 是 zip 里的路径，`to` 是笔记本里的路径（3.4）。
- **`TransferCounts`** 的说明：导入的 `pages`、`attachments` 是建了的，`renamed` 是改了名的，`skipped` 是跳过的条目，`missing` 恒为 0。进度：导入的是已处理的节点数 / 要建的节点数（校验完之前总数为 0）。
- `api/modules/instance.yaml`：`InstanceInfo.import_max_bytes`（必填，至少 1 MiB）。

### 3.9 开始导入

处理器读表单、调用例 `StartImport` 的三步，照上传的写法（先判定、后读文件）：

1. **`Check`**（读文件之前，不加锁）：笔记本所在的工作区（没有答 `notebook.not_found`）→ 判定 `transfer.import`（看不到 `notebook.not_found`，读者 `forbidden`）→ 有 `parent_id` 时它是这本笔记本里活着的页（`page.not_found`）→ 这本笔记本有排队或运行中的导入（409 `transfer.busy`）→ 排队与运行中的任务到 `transfer.max_queued`（503）→ 磁盘余量低于 `storage.min_free_bytes`（507）。这样 512 MiB 的上传不会传完才被拒。
2. **`Store`**：任务的 id 先取好（UUIDv7），文件部分流式写到 `imports/<id>.zip`（`Archives.CreateImport`），超过 `transfer.import_max_bytes` 中止、答 413；读失败答 400；写满答 507；都不留文件。之后读到表单与请求体的结尾（文件之后还有部分答 400，删掉文件）。路由的请求体上限是 `import_max_bytes` 加 64 KiB（文件之前的部分与边界），经 `API.Stream`，最低速率 `asset.upload_min_rate`。
3. **`Create`**：一个事务：工作区行 `FOR SHARE` → 笔记本行 `FOR SHARE` → 判定 → 位置仍是活着的页 → 任务创建的咨询锁之下数排队与运行中的（503）→ 这本笔记本有进行中的导入（409，唯一索引兜底）→ 写行（`queued`，`kind: import`，`root_id` 是位置，`name`，`client` 按凭据）并投递（队列 `transfer_import`，`MaxAttempts` 1）。失败时删掉 zip（写行之前的每一种失败都删；提交结果不明时不删，留给清扫）。答 202 与任务。
- 日志：`job_id`、`notebook_id`、`user_id`、`client`、字节数；文件名不进日志。

### 3.10 读 zip：结尾记录与目录

`adapter/archive` 的 `OpenImport(ctx, id)` 打开 `imports/<id>.zip`（`storage.File` 有 `ReaderAt` 与 `Size`），先读目录、再交给 `zip.NewReader`：

- **结尾记录**（`zipdir.go`）：照 Go 的 `readDirectoryEnd` 找结尾记录（最后 1 KiB，再最后 65 KiB），有 zip64 的定位记录时读 zip64 的结尾记录，算出中央目录的起点（含 Go 的 `baseOffset` 修正）。找不到、越界：失败 `not_zip`。结尾记录的条目数超过 `transfer.import_max_entries`、或中央目录的大小超过 64 MiB（5 万条、每条连名称与扩展字段约 1 KiB 还有余量）：失败 `too_many_entries`。
- **目录的预读**：从起点照 Go 的读法逐条读目录记录（46 字节的头，跳过名称、扩展字段与注释），直到不是目录记录的签名或读不满为止，只计数、不分配；条数超过上限、或读过的字节超过 64 MiB 就停下，失败 `too_many_entries`。这样 `zip.NewReader` 读到的条数（它的停法相同）有上限，不信结尾记录的条目数（总设计 4.11"中央目录的条目数谎报"）。交给 `zip.NewReader` 的 `ReaderAt` 另有字节的上限（预读的字节加结尾的部分），作为兜底：它若读得比预读多，答错误而不是继续分配。`NewReader` 之后条数与预读的不同：`not_zip`。
- **条目**：交给 domain 的是名称的原始字节、是否目录、模式（`File.Mode()`：符号链接与其他特殊文件）、加密标志、压缩方法、压缩后与条目头的大小，与按序号打开的读者。条目头的大小只作参考（3.11 按实际读出的计）。

### 3.11 校验

任务先把全部条目过一遍，不写任何东西（总设计 4.11"先校验，后写入"）。

- **分类**（domain，纯函数，表格测试）：
  - 名称不是合法的 UTF-8：跳过 `name_not_utf8`（不看 UTF-8 标志：macOS 写的 UTF-8 名称不带它；第 1 节）。
  - 路径：`\` 当作分隔符；去掉每一层的 `.` 与空段；以 `/` 开头、盘符（`C:`）开头、含 `..` 段的：跳过 `unsafe_path`。NFC。
  - 忽略（不计数、不报告）：任何一层以 `.` 开头的（`.obsidian/`、`.trash/`、`.git/`、`.DS_Store`……）、`__MACOSX`、`Thumbs.db`（不分大小写）；库的根下的 `.nerve/meta.json` 另读（下）。
  - 符号链接与其他特殊文件：跳过 `special_file`；加密的：`encrypted`；压缩方法不是 Store、Deflate：`unsupported_method`。
  - 同一路径（NFC 之后逐字节相同）的第二个及以后的条目：跳过 `duplicate`。
  - 去掉库外的一层：去掉忽略的条目之后，只有一个顶层目录、而它下面有 `.obsidian/` 或 `.nerve/` 时，这一层是库本身，路径从它下面算起（本系统的导出、打包整个库文件夹的 zip）。
- **读字节**（app）：按 zip 里的次序，每个要导入的文件读到结尾（Go 的读者核对 CRC 与大小），计实际读出的字节：
  - 全部加起来超过 `transfer.import_max_unpacked_bytes`：整包失败 `unpacked_too_large`；
  - 读出超过 1 MiB 之后，读出的字节超过压缩后大小的 200 倍：停下、跳过 `too_compressed`；
  - `.md` 超过 5 MiB、附件超过 `asset.max_bytes`：读到上限加一字节就停，跳过 `too_large`；
  - `.md` 读完之后过 `TreeWrites.CheckContent`，不过的跳过 `invalid_content`；
  - 解压出错、CRC 不对、大小与条目头不符：跳过 `unreadable`。
  - 读的时候随任务的上下文停下（取消、超时）。
- **`.nerve/meta.json`**：库的根下的，至多 16 MiB（超过的不读）；`format` 是 1 才用；`nodes` 的 `path` → `sort_order`（只有目录的页以 `/` 结尾，P5 3.9）；读不懂就当没有。`contributed` 列出的文件不导入（贡献者生成的，不是节点；M7 没有贡献者，有测试）。

### 3.12 映射

`domain.NewImportPlan`（纯函数，表格测试）：校验之后的条目、`meta` 的次序、导入位置的深度进，按层排好的节点出。

- `X.md` 是页面 `X`，正文是它的字节；目录 `X/`（显式的目录条目，或作为别的条目路径的前缀）是 `X` 的子节点，同级没有配对的 `X.md` 时 `X` 是没有正文的页（正文为空）。配对按原名（NFC、大小写折叠：`shared.TitleKey`）在修正之前进行：`a:b.md` 与 `a_b/` 不配对，各自修正之后撞名，后者加序号（3.3）。同一个目录里同键的 `.md` 有几个时，第一个与目录配对。
- 其余文件是附件，在它所在目录对应的页之下（库的根下的在导入位置之下）。
- **兄弟的次序**：`meta.json` 里有的按 `sort_order`（相同时按路径），之后没有的按修正后的名称（标题键，再按名称）；导入时依次接在导入位置原有的子节点之后。
- **深度**：导入位置的深度 `d`（根是 0）；第 `k` 层的页的深度是 `d + k`，超过 10 的页与它下面的一切（含附件）跳过，每个条目一条 `too_deep`。
- **按层排**：先第一层、再第二层……（父节点先于子节点建）；每一层里照兄弟的次序、深度优先地按父节点排（同一个父节点的连在一起，缓存命中）。

### 3.13 写入

worker 只调用用例 `Import.Run(ctx, jobID)`。导出与导入共用任务的骨架（`app/job_run.go`，从 `export.go` 移出）：`queued → running`、每秒的心跳读回取消与删除、`failed` 的原因码、结束时限时写结局。导出的行为不变（它的测试原样通过）。

1. 开始、心跳同导出。判定：以 `Actor{UserID, JobID}` 判定 `transfer.import`（看不到、不是写者：失败 `forbidden`；笔记本已删：已不在，什么都不写）。
2. 打开 zip（不在：`internal`），读目录（3.10），校验（3.11），读位置的深度（不在：`root_not_found`），映射（3.12）。进度的总数是要建的节点数。
3. **分批**：按层、照次序切批，一批至多 100 个节点、8 MiB 正文、20,000 条链接（链接数来自解析）。每一批：
   1. 停下的检查：心跳已要求取消、行已删，就在这里停（"在两批之间停下"）。
   2. 解析这一批的页（`TreeWrites.Parse`，在单元之前、不持锁）：取不到预算（503）时退避重试（1 秒起、每次加倍、至多 30 秒），直到任务的上下文结束；解析结果留到单元结束。
   3. 附件的文件从 zip 写进存储（`Attachments.Put`，随任务的上下文停下：取消不等一个 50 MiB 的文件写完）；写满：失败 `storage_full`。
   4. 单元（`TreeWrites.Import`，第一个单元建变更集，之后的并入它）：依次 `CreatePage`、`CreateAsset`（附件的 `after` 写行）；父节点是这次导入建的页时用它的 id。`ErrTooDeep` 的页记跳过，它的子孙在后面的批里同样跳过。单元用 River 的上下文（停机、超时才打断它），不用心跳的取消：已开始的单元做完。
   5. 单元之后：释放解析的预算；单元被拒（`*shared.Error`）时删掉这一批写进存储的文件，别的失败（提交结果可能不明）留给 asset 的孤儿清扫；名称与原名不同的记 `renamed`；进度前进。
   - 单元答的错误：判定拒绝（失去写权限、账户停用）：失败 `forbidden`；笔记本不在：已不在；父节点不是活着的页（导入的位置或已导入的页途中被删、被移进回收站）：位置是导入位置时 `root_not_found`，否则 `tree_changed`；其余 `internal`。
4. **统计**：每写 10,000 个节点、与结束时（写过节点的话），依次调用三个模块的 `Analyze`（3.6）。
5. **结束**：同导出的骨架：成功（`report`：计数与问题）、取消（报告写明已导入的部分）、失败（同样）、已不在（不写）。每种结局都删掉 `imports/<id>.zip`（删不掉的留给清扫）。导入没有 `result_bytes`，不让别的任务过期。
6. **日志**：开始、结束各一条：`job_id`、`notebook_id`、`user_id`、`client`、状态、原因的码、节点数、跳过数；路径与文件名不进日志。

### 3.14 收拾、清扫与生命周期

- **唯一索引**（迁移 `00031`）：`(notebook_id) WHERE kind = 'import' AND state IN ('queued', 'running') AND deleted_at IS NULL`；撞上答 409 `transfer.busy`。
- **收拾**：启动时与每 5 分钟的"运行中"照旧（不分种类）；排队的核对改为两种都核对：`QueuedJobs(kind)` 与 River 还没结束的那一种（`Held(kind)`：`transfer.export`、`transfer.import`）。被收拾的导入的 zip 由清扫删掉。
- **清扫**：`imports/` 下一天以前的文件，没有排队或运行中的导入对应的，删掉（结束时删不掉的、写行失败没删掉的、收拾之后的）。
- **生命周期**：笔记本删除照旧软删除它的任务（排队的被取到时什么都不做；运行中的下一次心跳看到、在两批之间停下，zip 删掉）；清理器删行之前删 zip（`Archives.Delete` 已按种类）。

### 3.15 配置

| 键 | 默认 | 约束 |
|---|---|---|
| `transfer.import_max_bytes` | 512 MiB | 至少 1 MiB；不小于 `asset.max_bytes` |
| `transfer.import_max_entries` | 50,000 | 1–1,000,000 |
| `transfer.import_max_unpacked_bytes` | 4 GiB | 不小于 `transfer.import_max_bytes` |
| `jobs.import_workers` | 1 | 1–8，与 `jobs.export_workers` 合起来至多 `database.max_conns` 的一半 |

交叉规则（总设计 8.4、4.13）：`asset.max_bytes` ≤ `transfer.import_max_bytes` ≤ `transfer.import_max_unpacked_bytes`。各项进 `LogValue`；`config.yaml` 的注释写明（含反向代理要放过 `import_max_bytes`）。`config.test.yaml` 不改。

### 3.16 组合根、权限、交错与日志

- **组合根**：`transferDeps` 加 `Tree`（page 的 `TreeWrites` 的适配）、`Attachments`（`asset.NewBlobs` 的写端口）、`Statistics`（page、linking、asset 的 `NewStatistics(pool)`，按这个次序）、导入的三个上限、`AssetMaxBytes`；`jobsConfig` 加 `transfer_import: jobs.import_workers`；`instance.Deps.ImportMaxBytes`。整个程序上的测试核对它们都接到了（交空时测试失败）：导入的页进索引（linking 的观察者经组合根到达）、附件的行写了、统计跑了、上限是配置的。
- **权限**：规则表加 `transfer.import`（写者）；`transfer.Actions()` 进动作表的核对；权限矩阵加 `startImport` 的每一格（读者 403、看不到 404、写者与管理员 202）。
- **交错**（`bootstrap/interleavings_transfer_test.go`，13.4 第 4 条）：
  - 导入的单元与同一页的保存：任务停在单元里（测试持着 `asset_blobs` 的表锁，单元建附件时等它），另一个连接保存导入位置那一页的正文：保存等笔记本行（`FOR SHARE` 对 `FOR NO KEY UPDATE`），放开之后两者都成功。
  - 导入的单元与同一父节点下的新建：测试先建一页"a"、再让导入建"a"：导入的成了"a 2"，报告 `renamed`；另一种先后：导入的单元持锁时用户新建"a"，等单元提交之后答 409 `page.title_taken`。
  - 任务的创建与笔记本的删除：两种先后（同导出）。
  - 取消与运行中的导入：任务停在第二批的单元前（测试持着表锁），取消之后第一批留着、第二批没有，结束为 `cancelled`，报告的计数是第一批的，zip 删掉。
  - 导入途中位置被删：第二批的单元答 `root_not_found`，第一批留着（照常可以删）。
  - 结束时核对不变式：`checkPages`、`checkLinks`、"每个活着的附件节点恰好一行"；没有活着而停在排队或运行中的任务；`imports/` 里只有排队与运行中的导入的文件。
- **日志**（13.1 第 10 条）：只记 id、`client`、数量与原因的码；测试钉住开始、结束的日志里没有文件名与路径。

### 3.17 Obsidian 样例与导出再导入

- **Obsidian 样例**（`bootstrap/transfer_import_obsidian_test.go`）：对 `tools/md-fixtures/resolve/` 的每个样例（已与 Obsidian 1.12.7 逐条核对），在测试里写一个 zip 库：样例的页是 `.md`（父页有子节点时同时有 `X.md` 与 `X/`）、附件是真的文件（图片是真的 PNG）、第 i 条链接写在与它出发的页同一个父页下的 `q<iii>.md` 里（同 `verify-resolve.mjs`），再加上 `.obsidian/app.json`、`__MACOSX/` 的条目与一层库外的目录；经 serve 导入一本新笔记本，核对每个 `q<iii>` 的链接在索引里解析到样例写的目标（`obsidian-verified` 与 `nerve-defined` 都要一致：后者是本系统的定义）。另一个库的条目名用 NFD 与 `\` 分隔，结果相同。
- **导出再导入**（`bootstrap/transfer_roundtrip_test.go`，性质测试，固定种子的随机树、每次几十棵）：随机的页（正文含链接、空的、只有子页的、被链接的只有子页的）、附件（随机字节、几种扩展名）、次序（含插入与移动之后的次序）、非 ASCII 与大小写不同的名称（不生成会被导出改名的 `N.md` 冲突与超长的名称）；经 serve 导出、把 zip 导入另一本笔记本，核对树（名称、类型、父子、兄弟的次序）、正文逐字节、附件的字节与类型、链接解析到的对应节点相同。

### 3.18 e2e：TR2–TR4 的接口版本

- `e2e/fixtures/zip-write.ts`：写 zip（node 的 `zlib` 的 `deflateRawSync`、`crc32`，不加依赖），能写出恶意的条目（符号链接的模式、谎报的结尾记录、`..`、加密标志、不支持的方法）。`e2e/fixtures/transfer.ts` 加 PAT 的导入（multipart）、轮询到结束。数据库断言 `e2e/fixtures/assert/transfer.ts` 加导入的（任务行、计数、`client`；建了的页、附件的行与存储里的文件；一个 `kind = import` 的变更集；`imports/` 里没有文件），链接经 `assert/links.ts` 的 `expectIndexedLinks`。
- **TR2**（`stories/transfer/tr2-import.spec.ts`）：一个带 `.obsidian/`、嵌套的页、页下与根下的附件、链接与嵌入、要修正与撞名的名称的库，导入到根下与一页之下；进度、报告（计数、`renamed`）、页与附件、链接解析。
- **TR3**（`tr3-roundtrip.spec.ts`）：TR1 的那本笔记本导出、导入另一本，树、正文、附件、次序相同。
- **TR4**（`tr4-malicious.spec.ts`）：越出根、符号链接、压缩炸弹（压缩比）、加密、不支持的方法、重复、名称不是 UTF-8 的条目各跳过并进报告，其余照常导入；条目数谎报的、中央目录过大的、不是 zip 的、解压后过大的整包失败，没有建任何节点，存储里不留文件；超过 `import_max_bytes` 的上传答 413（`nervewikiWith` 起一个上限很小的服务）。

## 4. B：前端

实施之前的设计；照实际改写的在 A 合并之后。

### 4.1 文件

| 位置 | 内容 |
|---|---|
| `services/transfer.service.ts` | `startImport(notebookId, parent, file, options)`：经会话的客户端与 `uploadFetch`（进度、取消），表单依次 `parent_id`、`file` |
| `stores/transfer.store.ts` | `startImport`：不经 `oneAtATime`（几分钟的上传不挡取消，13.2 第 1 条的例外，同附件的上传）；答复之后放到最前、丢弃在途的读；换代时中止在途的上传 |
| `pages/notebook/transfer-page.tsx`、`import-dialog.tsx` | 导入一节（写者）与导入的对话框 |
| `pages/notebook/transfer-report.tsx`、`transfer-job-row.tsx` | 导入的计数、失败与问题的码 |
| `i18n/messages/en.ts`、`zh-CN.ts`、`app/problem-messages.ts` | 文案 |
| `test/jobs-server.ts` | 假的服务端加导入 |
| `e2e/stories/transfer/tr2-import.spec.ts`、`tr3-roundtrip.spec.ts`、`tr4-malicious.spec.ts` | 页面版本；`e2e/fixtures/wiki-transfer.ts` 加 `importWith` |

### 4.2 导入一节与对话框

- **导入一节**（导出一节之前，写者才有；读者看到一句"只有写者能导入"）：一句说明（"把 Obsidian 的库或别的 Markdown 文件的 zip 导入这本笔记本"）与"导入 zip…"按钮。
- **对话框**：
  - 选文件（`accept=".zip,application/zip"`）；位置：根（默认）或一页，照移动对话框的上级列表（可搜索，只列页面）；说明：导入在后台进行；`.md` 是页面、文件夹是页面的子页、其余文件是附件；名称不合规的会被修正、同名的加序号，报告列出；相对导入位置超过 10 层的部分不导入；zip 至多 `import_max_bytes`（实例信息读不到时不写）。
  - 发送之前先查（总设计 4.8）：大小超过 `import_max_bytes` 时不发送，直接说明；不是 `.zip` 的提示但仍可发送（服务端判定）。
  - 上传时显示进度（`<progress>`，"已上传 n / 总数"）与"取消上传"；有在途的上传时挂上 `beforeunload`（13.2 第 21 条）；关对话框等于取消上传（先确认）。
  - 被拒时在对话框里说原因、不关：`transfer.busy`（"这本笔记本已有进行中的导入"）、`server_busy`、`storage_full`、`payload_too_large`、`page.not_found`（"导入的位置已不存在"）、`notebook.not_found`、`forbidden`、上传失败（传输错误）。
  - 成功之后关掉，任务在列表最前，焦点到它那一行（同导出）。
- **任务行**：导入的进度照导出（总数为 0 时不定：校验中）；报告的计数加"跳过"，不显示"缺文件"；失败的原因与问题按码给文案（3.8 的新码，不认识的照旧给通用的）；导入的 `renamed` 写"改名为 `to`，正文里指向原名的链接到不了它"。

### 4.3 测试

- vitest：服务（表单的部分与次序、进度、取消）；store（不排队、放到最前、换代中止）；对话框经路由到达（设置页）：写者有、读者没有、选文件与位置、大小的预查、进度与取消、关闭时的确认、每种被拒、成功之后的焦点、`beforeunload`；任务行与报告的新码（表）。
- e2e：TR2–TR4 的页面版本：在设置里选 zip 与位置导入，进度到完成，报告；导出再导入；恶意的 zip 的报告。

## 5. 实施步骤

| 步 | 内容 | 提交 |
|---|---|---|
| A1 | `shared.FixTitle`、`Actor.Valid`；`httpserver.Form` 与 asset 改用；page：变更集的类型与并入（迁移）、导入的单元与 `TreeWrites` 的三个方法、`Numbered`、链接数、读端口的 `Depth`；asset 的写端口；三个统计端口与授权 | `shared, page, asset: the import's units, names and statistics (M7/P6A)` |
| A2 | transfer 的 domain（码、分类、映射、`meta.json` 的读法）；`adapter/archive` 的导入 zip（写、打开、结尾记录与目录的预读） | `transfer: an import's archive read and mapped (M7/P6A)` |
| A3 | 任务的骨架从导出移出；`StartImport`、`Import`；收拾、清扫认导入；唯一索引；river 的队列与 worker | `transfer: imports run as background jobs (M7/P6A)` |
| A4 | 契约与 HTTP（`startImport`）；配置；实例信息；组合根；整个程序、交错、权限矩阵、运行时角色 | `transfer, bootstrap: the imports' API, wired into serve (M7/P6A)` |
| A5 | Obsidian 样例与导出再导入；e2e TR2–TR4 的接口版本；总体设计的修订 | `e2e: imports end to end (M7/P6A)` |
| B1 | 服务、store、导入一节与对话框、报告的码、i18n、vitest | `web: imports from the notebook's settings (M7/P6B)` |
| B2 | e2e TR2–TR4 的页面版本 | `e2e: imports in the browser (M7/P6B)` |

## 6. 测试与验证

- **shared**：`FixTitle` 的表（每种禁止的字符、首尾的空白与点、保留名、截短与扩展名、全部去掉）与性质测试（结果过 `CheckTitle`；合法的不变）；`Actor.Valid`。
- **page**：并入的核对（别的笔记本、类型不同、执行者或客户端不同、已删的变更集都拒绝；`updated_at` 前进）；导入的单元：撞名加序号（原有的、同一单元先建的、大小写与 `ß` 的标题键）、太深时什么都没写而单元里其余的照常、缓存与重排、附件的序号在扩展名之前；`Numbered` 的表；`Parse` 答链接数、释放预算；`CheckContent`。迁移的上下行。
- **transfer 的 domain**（表格）：分类的每一条（含 NFD、`\`、盘符、`..` 在中间、只有点的段、`__MACOSX`、隐藏文件、库外的一层与有别的顶层条目时不去掉）；映射（配对、只有目录的页、附件的位置、次序照 `meta.json` 与按名称、深度、按层与父节点排）；`meta.json` 的读法（坏的、别的版本、过大）。
- **`adapter/archive`**：结尾记录（普通、zip64、带注释、`baseOffset` 的修正、找不到）；条目数谎报（结尾记录写 1 而目录有 70,000 条、低 16 位相同）被预读拦下、`NewReader` 没有被调用；中央目录过大；预读与 Go 的读法条数一致（对 `archive/zip` 写出的各种 zip 比较）。
- **transfer 的 app**（假的端口）：开始导入的每个码（读文件之前答出；写行之前失败删掉 zip）；任务的每种结局（成功、取消在两批之间、行已删、超时、停机、失去写权限、位置不在、途中父节点不在、写满、预算忙时重试）；校验的每个跳过与每个整包失败（不建任何节点）；分批的三个上限；单元被拒时删文件；统计的调用与失败只记警告；每种结局删 zip；导出的行为不变。
- **postgres 适配器**（真的数据库）：唯一索引；`QueuedJobs(kind)`；清扫的语句。
- **HTTP**：处理器的表格（202 与每个码、表单的次序与多余的部分、413、读文件之前答出）；契约测试；asset 的上传测试原样通过。
- **整个程序**（13.1 第 21 条，组合根交空时失败）：真的 River 跑一次导入，页进索引、附件的行与文件、一个变更集、统计、上限；启动时的收拾认导入；笔记本删除之后导入停下；清扫 `imports/`。
- **交错、权限、运行时角色**：3.16、3.6。
- **Obsidian 与导出再导入**：3.17。
- **e2e**：TR2–TR4（A 接口、B 页面）；全部已有的故事照常通过。
- **反向对照**（每个都要有测试失败）：信结尾记录的条目数（不预读）；预读的停法与 Go 的不同；不查压缩比；按条目头而不是实际读出的字节计；`.md` 的检查不用 `CheckContent`；不去掉库外的一层或总是去掉；按修正之后的名称配对；撞名答 409 而不是加序号；序号加在扩展名之后；太深时整个单元失败；单元不并入第一个变更集；并入不核对执行者；单元里解析；不在两批之间看取消；取消打断进行中的单元；单元被拒时不删文件；结局不删 zip；收拾不核对排队的导入；清扫删掉进行中的导入的 zip；没有唯一索引；`ANALYZE` 不调用；`meta.json` 的次序不用；`contributed` 的文件被导入。
- `make check`、`make gen-check`、`make e2e`。

## 7. 完成标准

- 导入 Obsidian 的库与本系统的导出：页、附件、目录的映射、次序、名称的修正与序号照总设计 4.11；正文逐字节；导入的页进链接索引，`resolve/` 的样例导入之后与 Obsidian 解析的一致；导出再导入得到同样的树、正文、附件与次序。
- 恶意的 zip（越出根、符号链接、压缩炸弹、条目数谎报、中央目录过大、坏的名称、过深、过多、过大）被拒绝或跳过并写进报告，不留下文件。
- 一次导入一个变更集；每个单元在上限之内、在单元之前解析；取消在两批之间停下，中途失败、取消、服务重启时报告写明已导入的部分，任务不停在"运行中"，zip 被删掉或被清扫。
- 统计在导入期间与结束时更新，运行时角色有 `MAINTAIN`。
- 设置里的导入对话框经路由到达；TR2–TR4 的两个版本通过。
- 总体设计 13.1 第 1、2、5、16、19、31 条与 13.4 第 4 条修订；M6 移交第 4 项、P5 交给 P6 的各项记下落实。

## 8. 交给后面的 M

- **M8**：导入的整组撤销（一次导入一个变更集，可能有几万个节点）；回收站里的导入的页照常恢复。
- **M10**：导出贡献者加的文件在导入时不导入（`contributed`），M10 注册贡献者时核对这一条仍对。
- **M12**：名称不是 UTF-8 的条目按 GBK、CP437 解读；几个 GB 的导入的负载（校验时的解压两遍、每批的附件写入、`ANALYZE` 的时长）；反向代理的 `client_max_body_size` 与超时写进部署文档（[M12 的移交](../M12-release/handoffs/M7-transfer.md)）。
- **v0.1 收官之后的打磨**：页面菜单的"导入到此页"。

## 9. 结果

（实施之后填写。）
