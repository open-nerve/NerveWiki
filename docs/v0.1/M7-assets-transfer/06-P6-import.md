# M7/P6 导入：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M7/P6 导入 |
| 状态 | 已完成（A 合并 `b60cf66`，B 合并 `c9a48dd`） |
| 基线 | `27e5348`（P5 合并、文档补完之后的 main）；本文提交之后开分支 `m7-p6a`，A 合并之后开 `m7-p6b` |
| 上级文档 | [M7 总设计](00-M7-design.md) 4.1、4.2（变更集的类型与并入、先解析后单元）、4.8、4.9、4.11–4.14、第 5、7–9 节；[P5 文档](05-P5-export.md)第 8 节（交给 P6 的各项）；移交：[M6 链接与附件、导入、导出](handoffs/M6-links.md)第 4 项；总体设计 13.1 第 1、2、5、16、19、31 条，13.4 第 4 条 |

---

## 0. 分两部分

P6 把一个 zip（Obsidian 的库、本系统的导出、别的 Markdown 的 zip）导入到一本笔记本的某个位置。照 P5 的先例分开实现、审查、核对修复、合并：

- **A：服务端**：page 的变更集类型与并入、导入的写入单元（`TreeWrites` 的 `Parse`、`Import`）、名称的修正与序号；asset 的写端口；三个模块的统计端口与 `MAINTAIN`；平台的 multipart 表单读法（从 asset 移出，两处共用）；transfer 的开始导入、读 zip（结尾记录）、校验、映射、分批写入、报告、取消、收拾与清扫；契约、配置、`InstanceInfo.import_max_bytes`；Obsidian 样例与导出再导入的核对。e2e：TR2–TR4 的接口版本。
- **B：前端**：导入的服务（经上传的传输）、设置里的导入对话框（选文件、选位置、进度与取消）、任务行与报告认导入的码。e2e：TR2–TR4 的页面版本。

第 1、2 节两部分共用，第 3 节是 A 照实际改写的设计，第 4 节是 B 的，第 5–9 节共用。第 4 节在 B 实施之后照实际改写，差异列在第 9 节。

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

照实际改写（审查与修复核对之后）；与实施前的设计不同之处在第 9.1 节列出。

### 3.1 文件

| 位置 | 内容 |
|---|---|
| `shared/title.go`、`shared/actor.go` | `FixTitle`：把任意字符串修成合法的标题（3.4）；`Actor.Valid()`：恰好一种凭据、带账户 |
| `platform/httpserver/form.go` | multipart 表单的读法，从 asset 的上传移来（3.7）；asset 的上传改用它 |
| `migrations/sql/00030_page_changesets_import.sql` | `changesets.kind` 加 `import`（下行把导入的变更集改回 `edit`） |
| `modules/page/app/unit.go`、`unit_import.go`、`unit_create.go`、`unit_create_asset.go`、`unit_move.go`、`unit_place.go`、`import_writes.go`、`ports.go` | `UnitSpec.Kind`、`UnitSpec.Changeset` 与并入的核对（第一次写时）；导入的单元（兄弟与祖先的缓存、撞名加序号并跳过保留的名称、太深时不写）；`ImportWrites`（`CheckContent`、`Parse`、`Import`）；新建与放置拆出供两种单元共用 |
| `modules/page/domain/name.go`、`changeset.go` | `Numbered`：撞名时的序号写法；变更集的类型 |
| `modules/page/adapter/markdown/markdown.go` | `Links(facts)`：一份正文的链接数（单元的上限） |
| `modules/page/adapter/postgres/changesets.go` | `LockChangeset`：并入时锁住变更集的行 |
| `modules/page/tree_writes.go`、`export_nodes.go`、`statistics.go` | `TreeWrites` 加 `CheckContent`、`Parse`、`Import`；读端口加 `Depth`；`NewStatistics(pool)` |
| `modules/linking/statistics.go`、`modules/asset/statistics.go` | 各自表的 `ANALYZE`；三个模块各有 `queries/statistics.sql` |
| `modules/asset/blobs.go` | `NewBlobs(pool, store, logger)` 加 `Put`、`Attach`、`Drop` 与 `File`、`Owner` |
| `modules/transfer/domain` | 导入的动作、失败与问题的码；条目的分类（`entry.go`）；映射（`import_plan.go`）；`meta.json` 的读法；`ErrImportBusy` |
| `modules/transfer/adapter/archive` | 导入的 zip：上传（`Upload`）、打开、结尾记录与目录的预读（`zipdir.go`）、按种类列与删 |
| `modules/transfer/app` | `StartImport`（`start_import.go`）、进行中的上传（`uploads.go`）；`Import`（`import.go` 读与校验、`import_write.go` 分批写入、`import_ports.go`）；导出与导入共用的任务骨架（`job_run.go`：心跳、报告、结束、失败码）；收拾与清扫认导入 |
| `modules/transfer/adapter/postgres`、`river`、`http` | 导入的语句（心跳带报告、收拾保留报告）；`transfer_import` 队列与 worker；`startImport` 的处理器（`start_import.go`，`x-raw`） |
| `migrations/sql/00031_transfer_one_import.sql`、`00032_transfer_running_report.sql` | "每本笔记本一个导入"的唯一索引；运行中的行可以有报告（排队的仍不行） |
| `api/modules/transfer.yaml`、`instance.yaml` | `startImport`；失败与问题的码；`InstanceInfo.import_max_bytes` |
| `platform/config` | `transfer.import_*`、`jobs.import_workers` 与交叉规则 |
| `bootstrap/transfer.go`、`deps.go`、`wire.go` | 组合：`TreeWrites`、asset 的写端口（错误映射到 transfer 的）、三个统计端口、导入的配置与队列 |
| `deploy/runtime-grants.sql`、`deploy/image-smoke.sh` | page、linking、asset 的表加 `MAINTAIN`；镜像冒烟的实例信息 |
| `web/apps/web/src/pages/notebook/transfer-report.tsx`、`i18n` | 导入的失败与问题的码的文案（任务行与报告已能显示经接口开始的导入）；`problem.transfer.busy` 与按导出写的报告文案随 B |

### 3.2 page：变更集的类型与并入

- **迁移**（归 page，13.1 第 7 条）：`changesets_kind_check` 改为 `kind IN ('edit', 'import')`；下行先把 `import` 的变更集改回 `edit`。
- **`UnitSpec.Kind`**：`edit`（零值）或 `import`；新建的变更集照它写。
- **`UnitSpec.Changeset`**：非零时单元不建变更集，并入这一个：单元第一次写的时候（`ensureChangeset`；什么都没写的单元不碰它）在笔记本行的锁之下锁住那一行（`LockChangeset`，`FOR NO KEY UPDATE`），核对它属于这本笔记本、没删、类型与 `Kind` 相同、`created_by_id` 是这个单元的执行者、`client` 相同，不合答错误（不是用户的错：调用方的缺陷）；之后 `TouchChangeset`，笔记本的活动与日志的次序看得到之后的批。并入的单元不收编辑会话的写，不论类型（`changesetOf` 照旧只认会话自己的）。
- `Outcome.ChangesetID` 照旧答出：导入把第一个写了东西的单元的交给之后的单元。13.1 第 2 条改为"一个写入单元一个变更集，导入的各个单元并入第一个单元的"。
- **`Actor.Valid()`**：恰好一种凭据（会话、PAT、任务），且带账户。`Writer.Run` 与 `Allowed` 先核对，不合答错误（编程错误，不是 401）。

### 3.3 page：导入的写入端口

`(*page.Module).TreeWrites()` 加三个方法（接好线的模块给出，13.1 第 11 条的例外照旧）：

- **`CheckContent(content) error`**：`domain.CheckContent` 的规则（5 MiB、UTF-8、没有 NUL），不碰数据库：校验那一遍用它，规则只有一处。
- **`Parse(ctx, content) (Parsed, error)`**：`ContentParser.Decided` 那一支：预算排队取正文的字节（`Budget.Take`，此时不持锁，13.1 第 19 条），解析，留下提取结果那一份；`Parsed` 带着提取结果、`Links`（链接数：page 的 markdown 适配器数 obsidian 扩展取到的链接，同 `Tasks` 的做法）与 `Release()`。空的正文不排队。取不到预算答 `shared.ServerBusy`，调用方退避重试（3.13）。
- **`Import(ctx, spec ImportSpec, do func(ctx, u ImportUnit) error) (uuid.UUID, error)`**：跑一个写入单元：`Tree: true`、`Kind: import`、动作是 `spec.Action`（`transfer.import`，写者）、客户端是 `spec.Client`（任务记下的）、`spec.Changeset` 非零时并入；答出单元的变更集，什么都没写时答零。`ImportUnit` 是模块根的接口，`do` 里只有两种操作：
  - **`CreatePage(ctx, ImportedPage{ParentID, Name, Content, Parsed, Reserved}) (NodeInfo, error)`**：照 `CreatePage` 的规则，另有几处不同：
    - 名称撞上兄弟（按标题键，原有的子节点与这个单元先建的都算）时不答 409，取 `Numbered(name, 2)`、`3`……里第一个空着、且不是 `Reserved` 的（同一个库里后面的兄弟原样的名称，3.4）；单元按名称键与是否附件记下一个键上次给到的序号，下一个从它之后试（同一个键的几千个兄弟不必每个从 2 试起）。答出的 `NodeInfo.Name` 是实际的名称。名称的规则（`CheckTitle`）照旧检查，不合答 422：调用方已修正过（3.4）。
    - 深度超过 10 时答 `domain.ErrTooDeep`，此前什么都没写：调用方把这一页与它的子孙记成跳过，单元里其余的照常（3.13）。
    - 父节点不是这本笔记本里活着的页（删了、是附件、别的笔记本的）答 `ErrNoParent`（用户的 `CreatePage` 照旧答 422）。
    - 兄弟与祖先按父节点在单元里读一次、缓存，之后新建的接在缓存上；单元持着笔记本行的 `FOR NO KEY UPDATE`，别的树写进不来，缓存在单元里一直对。重排次序（`placeAmong` 的 `renumber`）时缓存随之更新。
  - **`CreateAsset(ctx, ImportedAsset{ParentID, Name, Meta, Reserved}, after) (NodeInfo, error)`**：照 `CreateAsset`，名称同样加序号（在扩展名之前）、同样跳过保留的、同样用缓存；`Meta` 是 `AssetMeta`（类型、字节数、SHA-256，不带图片的宽高）；`after(ctx, NodeInfo)` 在同一个事务里写附件的行（asset 的 `Attach`）。附件不算树的一层。
  - 每个新建的节点照常经守卫、参与者、观察者：新的页经 linking 的观察者进索引，之后建的页让先前解析不到的链接重新解析到它们（M6 的观察者按新节点的名称键找）；每个单元发一次 `pages` 事件（`tree: true`），前端的整树重读已经合并（P2）。
- **读端口**：`page.NewExportNodes(pool)` 加 `Depth(ctx, notebookID, pageID) (int, bool, error)`：活着的页的深度（根下的页是 1），附件与不在的答 `false`。开始导入时核对位置，任务运行时据它算"超过深度 10 的部分"。
- 组合根交给 transfer 的是 `TreeWrites` 的适配（`bootstrap/transfer.go`，带过 `Reserved`）；命令行的组合照旧到不了它（`archtest/composition_test.go` 已有这条断言）。

### 3.4 名称的修正与序号

- **`shared.FixTitle(s string, keepExtension bool) string`**（与 `CheckTitle` 放在一起，规则只有一处）：非 UTF-8 的字节换成 `U+FFFD` 再处理（导入只对合法 UTF-8 的名称调用）；NFC；禁止的字符（`/ \ : * ? " < > | # ^ [ ]`）与不可显示的字符（`unshowable`）换成 `_`；反复去掉首尾的空白与 `.`；Windows 保留名在第一个 `.` 之前加 `_`（`con.txt` → `con_.txt`）；超过 255 字节时在字符边界截短，`keepExtension` 时保留最后一个 `.` 起的扩展名（扩展名本身不短于 255 字节时整体截短）；截短之后再去一次尾部的空白与 `.`。全部去掉时答 `""`，调用方给默认名。性质测试：任意输入的结果要么是 `""`，要么过 `CheckTitle` 且等于 `CheckTitle` 的输出（已经合法的名称不变）。
- **页面与附件**：条目的最后一段先去掉首尾的空白与 `.`，以 `.md`（不分大小写）结尾、前面还有字符的是页面，标题是去掉 `.md` 的部分；否则是附件（总设计 4.11："去掉之后再判断"）。之后页面的标题经 `FixTitle(_, false)`、附件的名称经 `FixTitle(_, true)`；附件的结果以 `.md` 结尾时再加 `_`（到得了：扩展名太长、整体截短，正好截在 `x.md` 之后）。空的叫"未命名"（总设计 4.11；不随界面语言：它是存下来的内容，负责人可以改判）。文件夹的名称照页面。
- **序号**（`page/domain.Numbered(name, n, asset)`）：页面写 `name n`；附件在扩展名之前写（`a 2.png`；没有扩展名的照页面）。超过 255 字节时截短名称的主体，保留序号与扩展名；结果过 `CheckTitle`（表格与性质测试）。
- **保留的名称**：任务按父节点记下计划里每个名称键最后出现的位置；建一个节点要加序号时，序号的候选若是这个父节点下后面（还没建）的兄弟的名称键，就跳过：库 {`Untitled`、`Untitled 2`} 导入到已有 `Untitled` 的位置，前者成了 `Untitled 3`，库自己的 `Untitled 2` 原样，库里的 `[[Untitled 2]]` 照样到它。候选只按键核对，节点自己的名称保留与否不变。
- **报告**：最终的名称与原名（NFC 之后）不同的，记一条 `renamed`：`path` 是库里的路径（去掉库外的一层、`\` 换成 `/`、NFC，只有目录的页以 `/` 结尾），`to` 是导入之后在笔记本里的路径（导入位置之下的名称以 `/` 连接）。只差 NFC 的不算改名（标题键本来就在 NFC 之后比较，指向它的链接照样解析到）。跳过的条目的 `path` 是 zip 里的原名（还没分类，可能越出根或不是 UTF-8），`too_deep` 的是库里的路径；契约写明两种。路径至多 1,024 字节。

### 3.5 asset 的写端口

`asset.NewBlobs(pool, store, logger)`（多一个 logger：`Put` 中止失败要记）加：

- `Put(ctx, name, r, maxBytes) (File, error)`：照上传的写法（边写边算 SHA-256，按名称与开头的字节测定类型，图片读宽高）；`File{ID, MIME, Bytes, SHA256, Width, Height}`；超过 `maxBytes` 答 `ErrTooLarge`，存储写满答 `ErrStorageFull`，`r` 读失败原样带出；不留文件。
- `Attach(ctx, File, Owner{NodeID, NotebookID, CreatedBy, CreatedAt})`：在调用方的事务里写行（经 `postgres.DB(ctx, pool)`），归属由调用方给出。
- `Drop(ctx, File)`：删掉没有行的文件。

transfer 的 app 另起一个端口 `Attachments`（`Put`、`Attach`、`Drop`），与读的 `Blobs` 分开；组合根把同一个 `asset.Blobs` 适配给两者，`asset.ErrTooLarge`、`ErrStorageFull` 映射到 transfer 的。

### 3.6 统计与 `MAINTAIN`

- page、linking、asset 的模块根各给出 `NewStatistics(pool)`，`Analyze(ctx)` 对本模块导入会写到的表 `ANALYZE`（page：`nodes`、`page_contents`、`page_revisions`、`changesets`、`changeset_items`；linking：`indexed_pages`、`page_links`、`page_tags`、`page_properties`、`page_aliases`；asset：`asset_blobs`）。transfer 不对别人的表执行 SQL（M6 移交第 4 项、总设计 4.11），经组合根交来的 `[]Analyzer` 调用。语句经 sqlc（各模块的 `queries/statistics.sql`，sqlc 能解析 `ANALYZE`）。
- `deploy/runtime-grants.sql` 给这些表 `MAINTAIN`（PostgreSQL 17 起；`ANALYZE` 要它或属主）；`runtime_role_test.go` 跑一次导入，断言这些表的 `pg_stat_user_tables.last_analyze` 有了。
- 导入每写 10,000 个节点与结束时调用（3.13）；River 的上下文已结束（超时、停机）时不做结束时的那次；失败只记警告、不让导入失败（统计是优化），运行时角色的测试经 `last_analyze` 抓到没给的授权。

### 3.7 平台：multipart 表单的读法

asset 的上传与导入读同一种表单：几个小的文字部分、一个文件部分（流式）、之后什么都没有。把 `asset/adapter/http/upload.go` 的读法移到 `platform/httpserver/form.go`：`NewForm(r, texts, file, maxPreface)`；`Fields(maxLens)` 按给定的次序读文字部分各至多一次、交出文件部分（未知的、重复的、次序不对的、超过长度的答 `FormError`，400，字段错误经 `InvalidPart`）；`Open()` 之后才放行文件的字节；`End(r)` 读到结尾（文件之后还有部分答 400）。请求体读失败是 `BodyReadError`，`ReadCause` 给出日志用的原因（`TestReadCause` 随之移到平台）。asset 的处理器改用它，行为不变（`upload_body_test.go` 原样通过）。各自的 `early`、`readFailed` 留在模块里（错误到答复的映射是各自的）；transfer 的 app 另有 `ReadError`（`Store` 读文件失败）。

### 3.8 契约

`api/modules/transfer.yaml`：

- **`startImport`**：`POST /api/v0/notebooks/{notebook_id}/imports`，`x-raw`（代码生成排除，契约测试按描述核对），`multipart/form-data`：`parent_id`（可省，导入到根下）、`file`（zip），依次、各至多一次；文件之前至多读 4 KiB；`Content-Transfer-Encoding` 不解码；答 202 `TransferJob`。
  - 错误码（`x-problem-codes` 列模块的码与 `forbidden`、`server_busy`、`storage_full`）：`notebook.not_found`、`forbidden`（读者）、`page.not_found`（`parent_id` 不是这本笔记本里活着的页）、`transfer.busy`（409：这本笔记本已有排队、运行中或正在上传的导入，任何人的）、`server_busy`（503，排队、运行中的任务与正在上传的导入已满，带 `Retry-After`）、`storage_full`（507：声明的长度与别的上传还没存下的写进去之后余量不够，或写的时候写满）、`payload_too_large`（413：声明的长度超过上限时立即答，否则文件超过 `transfer.import_max_bytes` 时）、`bad_request`（400：表单不对、请求体没读完整或太慢）。读文件之前答出的拒绝关闭连接，客户端可能看到连接被重置。`Create` 再做读文件之前的各项核对，存储的余量除外。
  - 任务的 `root_id` 是导入的位置（`null`：根），`name` 是上传的文件名（经 `FixTitle(_, true)`，空的叫"未命名"）。
- **`TransferFailure`** 加：`not_zip`（不是 zip，或结尾记录、中央目录坏了、越出文件）、`too_many_entries`（条目超过 `transfer.import_max_entries`，或中央目录超过 64 MiB）、`unpacked_too_large`（解压后的总字节数超过 `transfer.import_max_unpacked_bytes`）、`tree_changed`（导入途中，已导入的页被删除或移走，后面的写不下去）；`root_not_found` 的说明加上"导入的位置已不存在"。
- **`TransferProblem.code`** 加：`unsafe_path`（越出根：`..`、绝对路径、盘符）、`special_file`（符号链接与其他特殊文件）、`encrypted`、`unsupported_method`（不是 Store、Deflate）、`too_compressed`（压缩比超过 200）、`name_not_utf8`、`invalid_content`（`.md` 不是 UTF-8 或含 NUL）、`too_large`（`.md` 超过 5 MiB、附件超过 `asset.max_bytes`）、`too_deep`（在笔记本里超过 10 层）、`duplicate`（同一路径的第二个条目）、`unreadable`（解压失败、校验和不对、大小与条目头不符）。路径的写法见 3.4。
- **`TransferCounts`** 的说明：导入的 `pages`、`attachments` 是建了的，`renamed` 是改了名的，`skipped` 是跳过的条目，`missing` 恒为 0；导出被收拾时计数为 0，导入的计数与问题随心跳写（至多每 10 秒），被收拾时保留最后写的。进度：导入的是已处理的节点数 / 要建的节点数（校验完之前总数为 0）。
- `api/modules/instance.yaml`：`InstanceInfo.import_max_bytes`（必填，至少 1 MiB）。

### 3.9 开始导入

处理器读表单、调用例 `StartImport` 的几步，照上传的写法（先判定、后读文件）：

0. 凭据的客户端；声明的 `Content-Length` 超过 `import_max_bytes` 加 64 KiB 时立即答 413、关闭连接。读文件之前的部分（`parent_id` 不是 id 时 400，读者也是）。
1. **`Check`**（读文件之前，不加数据库的锁）：笔记本所在的工作区（没有答 `notebook.not_found`）→ 判定 `transfer.import`（看不到 `notebook.not_found`，读者 `forbidden`）→ 有 `parent_id` 时它是这本笔记本里活着的页（`page.not_found`）→ 占住这本笔记本的上传（进程内，`app.Uploads`；已被占住答 409 `transfer.busy`）→ 这本笔记本有排队或运行中的导入（409）→ 排队与运行中的任务加上别的笔记本正在上传的到 `transfer.max_queued`（503）→ 磁盘余量减去声明的长度与别的上传还没存下的字节低于 `storage.min_free_bytes`（507）。通过之后这个上传被准入，到请求结束时放开（`Release`）；任何一步失败或 panic 都放开。各个 `Check` 的占住与判定一次一个（进程内的锁），两个同时的不互相拒绝。这样 512 MiB 的上传不会传完才被拒。
2. **`Store`**：任务的 id 先取好（UUIDv7），文件部分流式写到 `imports/<id>.zip`（`Archives.Upload`：`Write`、`Commit`、`Abort`），每次写的字节告诉这个上传（别的启动按"还没存下的"计）；超过 `transfer.import_max_bytes` 中止、答 413；读失败答 400（停机打断时中止连接、不记错误）；写满答 507；都不留文件。答 `Stored{ID, Bytes}`。之后读到表单与请求体的结尾（文件之后还有部分、请求体在表单之后超过路由的上限答 400，`Discard` 删掉文件）。路由的请求体上限是 `import_max_bytes` 加 64 KiB（文件之前的部分与边界），经 `API.Stream`，最低速率 `asset.upload_min_rate`。
3. **`Create`**：一个事务：工作区行 `FOR SHARE` → 笔记本行 `FOR SHARE` → 判定 → 位置仍是活着的页 → 任务创建的咨询锁之下数排队与运行中的、加上别的被准入的上传（503）→ 这本笔记本有进行中的导入（409，唯一索引兜底）→ 写行（`queued`，`kind: import`，`root_id` 是位置，`name`，`client` 按凭据）并投递（队列 `transfer_import`，`MaxAttempts` 1）。写行之后这个上传对持着咨询锁的启动（导出、别的创建）不再计数（它们在锁之下看得到提交了的行），提交之后对 `Check` 也不再计数。失败时删掉 zip（写行之前的每一种失败都删；提交结果不明时不删，留给清扫）。答 202 与任务。
- **计数的先后**：`Check` 先读上传、再数行（被告知已写的上传，它的行在数之前已提交）；持锁的启动先数行、再读上传（持锁时没有上传的行在提交，`Check` 在它数行时准入的上传也数到）。`Check` 与导出同时判定时两者都可能通过，队列因此多一个，下一个持锁的启动被拒（`room` 的注释）。存储的余量先读上传、再读磁盘：其间存下的字节数两遍，不漏。
- 日志：`job_id`、`notebook_id`、`user_id`、`client`、字节数；文件名不进日志。

### 3.10 读 zip：结尾记录与目录

`adapter/archive` 的 `OpenImport(ctx, id, most)` 打开 `imports/<id>.zip`（`storage.File` 有 `ReaderAt` 与 `Size`），先读目录、再交给 `zip.NewReader`：

- **结尾记录**（`zipdir.go`）：照 Go 的 `readDirectoryEnd` 找结尾记录（最后 1 KiB，再最后 65 KiB），有 zip64 的定位记录时读 zip64 的结尾记录（定位记录的偏移是负的时照 Go 忽略，越过文件的答 `not_zip`），算出中央目录的起点（含 Go 的 `baseOffset` 修正）。找不到、越界：失败 `not_zip`。结尾记录的条目数超过 `most`（`transfer.import_max_entries`）、或中央目录的大小超过 64 MiB：失败 `too_many_entries`。
- **目录的预读**：从起点照 Go 的读法逐条读目录记录（46 字节的头，跳过名称、扩展字段与注释），直到不是目录记录的签名或读不满为止，只计数、不分配；条数超过上限、或读过的字节超过 64 MiB 就停下，失败 `too_many_entries`。这样 `zip.NewReader` 读到的条数（它的停法相同）有上限，不信结尾记录的条目数（总设计 4.11"中央目录的条目数谎报"）。交给 `zip.NewReader` 的 `ReaderAt` 另有字节的上限（预读的字节加结尾的 128 KiB），作为兜底：它若读得比预读多，答 `too_many_entries` 而不是继续分配。`NewReader` 之后条数与预读的不同：`not_zip`。`zip.ErrInsecurePath` 不算错（路径由 domain 分类）。
- **条目**：交给 domain 的是 `RawEntry{Index, Name, Folder, Special, Encrypted, Method}`：名称的原始字节、是否目录（名称以 `/` 或 `\` 结尾，或模式是目录）、是否特殊文件（符号链接与其他不是普通文件的）、加密标志、压缩方法；压缩后的字节数经 `ImportArchive.Packed(i)`，按序号打开读者。条目头写的解压后的大小不交出（3.11 按实际读出的计）。

### 3.11 校验

任务先把全部条目过一遍，不写任何东西（总设计 4.11"先校验，后写入"）。

- **分类**（`domain.Classify`，纯函数，表格测试），按 zip 里的次序，每个条目依次：
  - 名称不是合法的 UTF-8：跳过 `name_not_utf8`（不看 UTF-8 标志：macOS 写的 UTF-8 名称不带它；第 1 节）。报告里的路径是名称的字节换掉不合法的部分。
  - 路径：`\` 当作分隔符；去掉 `.` 与空段；以 `/` 开头、盘符开头（字母加冒号，之后是 `/` 或结尾：`a:b.md` 不是盘符，是要修正的名称）、含 `..` 段的：跳过 `unsafe_path`。每段 NFC。
  - 这两项在忽略之前核对：忽略的目录下的条目也会因此进报告。
  - 去掉库外的一层：去掉忽略的条目之后，顶层只有一个目录、没有顶层的文件，而它下面有 `.obsidian/` 或 `.nerve/` 时，这一层是库本身，路径从它下面算起（本系统的导出、打包整个库文件夹的 zip）。
  - 库的根下的 `.nerve/meta.json`（第一个）另读（下）。
  - 忽略（不计数、不报告）：任何一层以 `.` 开头的（`.obsidian/`、`.trash/`、`.git/`、`.DS_Store`……）、`__MACOSX`（区分大小写）、`Thumbs.db`（不分大小写）。
  - 目录条目：同一路径的第二个不计数、不报告（目录本身不导入，只决定页的父子）。
  - 文件：符号链接与其他特殊文件跳过 `special_file`；加密的 `encrypted`；压缩方法不是 Store、Deflate 的 `unsupported_method`；同一路径（NFC 之后逐字节相同）的第二个及以后的 `duplicate`。报告里的路径是 zip 里的原名。
- **读字节**（app 的 `validate`）：按 zip 里的次序，每个要导入的文件读到结尾（Go 的读者核对 CRC 与大小），每次 32 KiB，计实际读出的字节；每读一次依次核对：
  - 全部加起来超过 `transfer.import_max_unpacked_bytes`：整包失败 `unpacked_too_large`；
  - `.md` 超过页的正文上限（page 模块的，5 MiB）、附件超过 `asset.max_bytes`：停下、跳过 `too_large`；
  - 读出超过 1 MiB 之后，读出的字节超过压缩后大小的 200 倍：停下、跳过 `too_compressed`；
  - 打不开、解压出错、CRC 不对、大小与条目头不符：跳过 `unreadable`；
  - `.md` 读完之后过 `TreeWrites.CheckContent`，不过的跳过 `invalid_content`；
  - 读的时候随任务的上下文停下（取消、超时）。
  - 记下每个留下的文件解压后的字节数（分批用）。
- **`.nerve/meta.json`**：至多 16 MiB（`domain.MaxMeta`，超过的不读），它的字节不计入解压后的总数；`format` 是 1 才用；`nodes` 的 `path` → `sort_order`（只有目录的页以 `/` 结尾，P5 3.9）；读不懂就当没有。`contributed` 列出的文件不读、不导入、不报告（贡献者生成的，不是节点；M7 没有贡献者，有测试）。

### 3.12 映射

`domain.NewImportPlan`（纯函数，表格测试）：校验之后的条目、`meta` 的次序、导入位置的深度进，按层排好的节点出（`ImportNode{Parent, Asset, Name, Original, Path, Entry, Level}`）。

- `X.md`（扩展名不分大小写，去掉两端的空白与点之后）是页面 `X`，正文是它的字节；目录 `X/`（显式的目录条目，或作为别的条目路径的前缀）是 `X` 的子节点，同级没有配对的 `X.md` 时 `X` 是没有正文的页（正文为空，报告里的路径以 `/` 结尾，zip 里没有这个条目也一样）。配对按原名的键（NFC、大小写折叠：`shared.TitleKey`）在修正之前进行：`a:b.md` 与 `a_b/` 不配对，各自修正之后撞名，后者加序号（3.4）。同一个键的 `.md` 与目录各有几个时，按次序一一配对。
- 其余文件是附件，在它所在目录对应的页之下（库的根下的在导入位置之下）；附件不算一层：深度 10 的页的附件照常导入。
- 名称经 `shared.FixTitle` 修正，空的叫"未命名"；附件的名称截短之后以 `.md` 结尾的，再加 `_` 修正。
- **兄弟的次序**：`meta.json` 里有的按 `sort_order`（相同时按路径），之后没有的按修正后的名称的键、原样的先于修正过的、再按名称与路径；导入时依次接在导入位置原有的子节点之后。
- **深度**：导入位置的深度 `d`（根是 0）；第 `k` 层的页的深度是 `d + k`，超过 10 的页与它下面的一切（含附件）跳过，每个一条 `too_deep`。
- **按层排**：先第一层、再第二层……（父节点先于子节点建）；每一层里按父节点的次序、同一个父节点的连在一起。

### 3.13 写入

worker 只调用用例 `Import.Run(ctx, jobID)`。导出与导入共用任务的骨架（`app/job_run.go`，从 `export.go` 移出）：`queued → running`、每秒的心跳读回取消与删除、`failed` 的原因码、结束时限时写结局。导出的行为不变（它的测试原样通过）。

1. 开始、心跳同导出。判定：以 `Actor{UserID, JobID}` 判定 `transfer.import`（看不到、不是写者：失败 `forbidden`）；笔记本删除时任务行随之软删除，心跳看到之后停下，什么都不写。
2. 打开 zip（`not_zip`、`too_many_entries`，3.10），分类、读 `meta.json`、校验（3.11），读位置的深度（不在：`root_not_found`），映射（3.12）。进度的总数是要建的节点数，之前是 0；已处理的含跳过的。
3. **分批**：按计划的次序切批，一批至多 100 个节点、8 MiB 正文、20,000 条链接（链接数来自解析）；一页单独超过的自成一批。每一批：
   1. 停下的检查：心跳已要求取消、行已删，就在这里停（"在两批之间停下"）。
   2. 读这一批的页的正文、解析（`TreeWrites.Parse`，在单元之前、不持锁）：取不到预算（503）时，手上已有解析好的页就先写这一批（放开它们的预算），没有就退避重试（1 秒起、每次加倍、至多 30 秒），直到心跳停下或任务的上下文结束。
   3. 附件的文件从 zip 写进存储（`Attachments.Put`，随心跳的停下而停：取消不等一个 50 MiB 的文件写完）；写满：失败 `storage_full`。
   4. 单元（`TreeWrites.Import`，第一个单元建变更集，之后的并入它）：依次 `CreatePage`、`CreateAsset`（附件的 `after` 写行），带上计划里后面的兄弟要用的名称键（`Reserved`，3.4）；父节点是这次导入建的页时用它的 id。单元答 503 时同样退避重试；每次尝试之前再做一次停下的检查。单元用 River 的上下文（停机、超时才打断它），不用心跳的：已开始的单元做完。`ErrTooDeep` 的页记跳过，它的子孙在这一批与后面的批里同样跳过。
   5. 单元之后：释放解析的预算。单元失败时，`do` 失败或被拒（`*shared.Error`，事务已回滚）就删掉这一批写进存储的文件，提交结果可能不明的留给 asset 的孤儿清扫；提交了的单元里因太深而丢下的附件，文件删掉。名称与原名不同的记 `renamed`（报告写出原路径与新路径）；进度前进；报告交给心跳（3.14）。
   - 单元答的错误：判定拒绝、看不到笔记本、未认证（失去写权限、账户停用、笔记本在行删除之前已删）：失败 `forbidden`；父节点不是活着的页（导入的位置或已导入的页途中被删、被移进回收站）：位置是导入位置时 `root_not_found`，否则 `tree_changed`；其余 `internal`。
4. **统计**：每写 10,000 个节点、与结束时（写过节点、River 的上下文还没结束的话），依次调用三个模块的 `Analyze`（3.6）。
5. **结束**：同导出的骨架：成功（`report`：计数与问题）、取消（报告写明已导入的部分）、失败（同样）、已不在（不写）。每种结局都删掉 `imports/<id>.zip`（删不掉的留给清扫）。导入没有 `result_bytes`，不让别的任务过期。
6. **日志**：开始、结束各一条：`job_id`、`notebook_id`、`user_id`、`client`、状态、原因的码、节点数（含跳过的）、跳过数；路径与文件名不进日志。

### 3.14 报告、收拾、清扫与生命周期

- **报告随心跳写**：导入的计数与问题在运行中交给心跳（原子的指针），心跳在报告变了、且距上一次写至少 10 拍（10 秒）时随心跳一起写（`BeatJob` 的 `coalesce`），写失败下一拍再试；结束时照常写最终的。迁移 `00032` 允许运行中的行有报告（排队的仍不行）；读接口在任务结束之前不交出它。进程崩溃之后，收拾把失败合并进留着的报告（`InterruptJobs`），报告写明到最后一次心跳为止已导入的部分；导出的报告运行中为空，照旧只有失败。
- **唯一索引**（迁移 `00031`）：`(notebook_id) WHERE kind = 'import' AND state IN ('queued', 'running') AND deleted_at IS NULL`；撞上答 409 `transfer.busy`。
- **收拾**：启动时与每 5 分钟的"运行中"照旧（不分种类）；排队的核对改为两种都核对：`QueuedJobs(kind)` 与 River 还没结束的那一种（`Held(kind)`：`transfer.export`、`transfer.import`）。被收拾的导入的 zip 由清扫删掉。
- **清扫**：先导出、再导入，`imports/` 下一天以前的文件，没有排队或运行中的导入对应的（`LiveArchives(ctx, kind, ids)`），删掉（结束时删不掉的、写行失败没删掉的、收拾之后的）；一种列不出或删不掉不妨碍另一种，错误合并之后返回。
- **生命周期**：笔记本删除照旧软删除它的任务（排队的被取到时什么都不做；运行中的下一次心跳看到、在两批之间停下，zip 删掉）；清理器删行之前删 zip（`Archives.Delete` 已按种类）。

### 3.15 配置

| 键 | 默认 | 约束 |
|---|---|---|
| `transfer.import_max_bytes` | 512 MiB | 至少 1 MiB；不小于 `asset.max_bytes`；按 `asset.upload_min_rate` 在 3 小时之内传完（`MaxImportTransfer`） |
| `transfer.import_max_entries` | 50,000 | 1–100,000（64 MiB 的中央目录装得下的量级） |
| `transfer.import_max_unpacked_bytes` | 4 GiB | 不小于 `transfer.import_max_bytes` |
| `jobs.import_workers` | 1 | 1–8，与 `jobs.export_workers` 合起来至多 `database.max_conns` 的一半（`export_workers` 本身合法时才核对） |

交叉规则（总设计 8.4、4.13）：`asset.max_bytes` ≤ `transfer.import_max_bytes` ≤ `transfer.import_max_unpacked_bytes`。`transfer.max_queued` 的说明加上"正在上传的导入也算"。各项进 `LogValue`；`config.yaml` 的注释与 README 写明（含反向代理要放过 `import_max_bytes`、上传至多 3 小时）。`config.test.yaml` 不改。

### 3.16 组合根、权限、交错与日志

- **组合根**：`transferDeps` 加 `Tree`（page 的 `TreeWrites` 的适配，含 `Reserved` 的转交）、`Attachments`（`asset.NewBlobs` 的写端口）、`Statistics`（page、linking、asset 的 `NewStatistics(pool)`，按这个次序）、导入的三个上限、`AssetMaxBytes`、`MaxContentBytes`（页的正文上限）；`app.NewUploads()` 一个，交给导出与导入的开始；`jobsConfig` 加 `transfer_import: jobs.import_workers`；`instance.Deps.ImportMaxBytes`。整个程序上的测试（`transfer_import_test.go`）核对它们都接到了：导入的页进索引（linking 的观察者经组合根到达）、附件的行写了、统计跑了、上限是配置的、序号避开后面的兄弟的名称（页与附件各一例）。
- **权限**：规则表加 `transfer.import`（写者）；`transfer.Actions()` 进动作表的核对；权限矩阵加 `startImport` 的每一格（读者 403、看不到 404、写者与管理员 202）。
- **交错**（`bootstrap/interleavings_import_test.go`，13.4 第 4 条）：
  - 导入的单元与同一页的保存：任务停在单元里（测试持着 `asset_blobs` 的表锁，单元建附件时等它），另一个连接保存导入位置那一页的正文：保存等笔记本行，放开之后两者都成功。
  - 导入的单元与同一父节点下的新建：两种先后（先建的"a"让导入的成了"a 2"、报告 `renamed`；单元持锁时用户新建"a"，等单元提交之后答 409 `page.title_taken`）。
  - 任务的创建与笔记本的删除：两种先后（同导出）。
  - 取消与运行中的导入：任务停在第一个单元的附件上，直到心跳读到取消；放开之后第一批留着、之后的没有，结束为 `cancelled`，报告的计数是第一批的，zip 删掉。
  - 导入途中位置被删：之后的单元答 `root_not_found`，已写的留着。
  - 结束时核对不变式：`checkPages`、`checkLinks`、"每个活着的附件节点恰好一行"；没有活着而停在排队或运行中的任务；`imports/` 里只有没删的排队与运行中的导入的文件。
- **日志**（13.1 第 10 条）：只记 id、`client`、数量与原因的码；测试钉住开始、结束的日志里没有文件名与路径。

### 3.17 Obsidian 样例与导出再导入

- **Obsidian 样例**（`bootstrap/transfer_import_obsidian_test.go`）：对 `tools/md-fixtures/resolve/` 的每个样例（已与 Obsidian 1.12.7 逐条核对），在测试里写一个 zip 库：样例的页是 `.md`（父页有子节点时同时有 `X.md` 与 `X/`）、附件是真的文件（图片是真的 PNG）、第 i 条链接写在与它出发的页同一个父页下的 `q<iii>.md` 里（同 `verify-resolve.mjs`），再加上 `.obsidian/app.json`、`__MACOSX/` 的条目与一层库外的目录；经 serve 导入一本新笔记本，核对每个 `q<iii>` 的链接在索引里解析到样例写的目标（`obsidian-verified` 与 `nerve-defined` 都要一致）。另一个库的条目名用 NFD 与 `\` 分隔，结果相同。
- **导出再导入**（`bootstrap/transfer_roundtrip_test.go`，性质测试，固定种子的随机树）：随机的页（正文含链接、空的、只有子页的、被链接的只有子页的）、附件（随机字节、几种扩展名）、次序（含插入与移动之后的次序）、非 ASCII 与大小写不同的名称；经 serve 导出、把 zip 导入另一本笔记本，核对树（名称、类型、父子、兄弟的次序）、正文逐字节、附件的字节与类型、链接解析到的对应节点相同。

### 3.18 e2e：TR2–TR4 的接口版本

- `e2e/fixtures/zip-write.ts`：写 zip（node 的 `zlib` 的 `deflateRawSync`、`crc32`，不加依赖），能写出恶意的条目（符号链接的模式、谎报的结尾记录、`..`、加密标志、不支持的方法、坏的 CRC、记录的注释）。`e2e/fixtures/transfer.ts` 加 PAT 的导入（multipart）、轮询到结束。数据库断言 `e2e/fixtures/assert/transfer.ts` 加导入的（任务行、计数、`client`；建了的页、附件的行与存储里的文件；一个 `kind = import` 的变更集；`imports/` 里没有文件，`.tmp` 不算），链接经 `assert/links.ts` 的 `expectIndexedLinks`。
- **TR2**（`stories/transfer/tr2-import.spec.ts`）：一个带 `.obsidian/`、嵌套的页、页下与根下的附件、链接与嵌入、要修正与撞名的名称的库，导入到根下与一页之下；进度、报告（计数、`renamed`）、页与附件、链接解析。
- **TR3**（`tr3-roundtrip.spec.ts`）：TR1 的那本笔记本导出、导入另一本，树、正文、附件、次序相同；例外是导出时改了名的文件夹（`Plan.md 2`），导入之后是这个名称的页（M7 收尾审查 B-N2）。
- **TR4**（`tr4-malicious.spec.ts`）：越出根、符号链接、压缩炸弹（压缩比）、加密、不支持的方法、重复、名称不是 UTF-8 的条目各跳过并进报告，其余照常导入；条目数谎报的、中央目录过大的、不是 zip 的、解压后过大的整包失败，没有建任何节点，存储里不留文件；超过 `import_max_bytes` 的上传答 413（`nervewikiWith` 起一个上限很小的服务）。

## 4. B：前端

照实际改写（实施之前的设计见第 9.2 节的不同之处）。

### 4.1 文件

| 位置 | 内容 |
|---|---|
| `services/transfer.service.ts` | `startImport(notebookId, parent, file, options)`：表单依次 `parent_id`（根时不发）、`file`，经会话的客户端与 `uploadFetch`（进度、取消；传输可注入，同 `AssetService`，根 store 给 `app.transfer`）；`UploadOptions` 导出 |
| `stores/transfer.store.ts` | `startImport(parent, file, options)`：不经 `starts`（几分钟的上传不挡导出的开始与取消，13.2 第 1 条的例外，同附件的上传）；答复之后放到最前、丢弃在途的读；`uploading` 数在途的上传，`warn(uploading > 0)`；`importing`：列表里这本笔记本排队或运行中的导入（列表里有的：自己的，管理员看得到所有人的；看不到的由服务端判定） |
| `stores/root.store.ts` | `transfersOf` 把上传期间的离开提醒接到根 store 的 `UnloadWarning`，键 `import <笔记本 id>`（附件的键是笔记本 id：附件的上传跨页面继续，结束时不清掉导入的）；`UnloadWarning` 第一次需要时才建，附件与导入共用 |
| `pages/notebook/transfer-page.tsx` | `ImportSection` 在导出一节之前：写者（`writesPages`）有"导入 zip…"，读者一句说明；导入开始之后焦点到它那一行，列表读不到时到"最近的任务"的标题（同导出） |
| `pages/notebook/import-dialog.tsx` | `ImportDialog`：开关、关闭与离开时的询问（`useBlocker`）、"停止上传"按钮的引用；`ImportForm`：表单、预查、上传与被拒；`UploadProgress`：进度单独渲染（进度不让整个表单与位置的下拉重绘，审查 A-1） |
| `pages/notebook/transfer-report.tsx`、`transfer-job-row.tsx` | 计数、失败与问题按任务的种类：`shownCounts`（导入：页、附件、改名、跳过；不认识的种类：页、附件、改名）；`failureText(kind, …)`（导入的 `forbidden` 说"不能编辑"、`root_not_found` 说"导入的位置"）；`problemText(kind, …)`（导入的 `renamed` 说"正文里指向原名的链接到不了它"，不认识的码说"没有照原样导入"）；问题的标题"没有照原样导入的条目"；失败或取消的导入、页或附件的计数大于 0 时说"上面计数的页面与附件留在了笔记本里"（审查 B-6） |
| `i18n/messages/en.ts`、`zh-CN.ts` | `transfer.import*`、`transfer.exportBusy`、`transfer.importFailure.*`、`transfer.importProblem.*`、`transfer.count.skipped`、`field.file.too_long`、`field.place.gone`；`problem.transfer.busy` 改成不分种类的说法，导出与导入的对话框经 `useForm` 的 `texts` 各用自己的；编辑器上传的 `asset.folders`、`asset.pageFile` 说"打成 zip，在笔记本设置里导入"（审查 B-9）。`app/problem-messages.ts` 不改 |
| `test/jobs-server.ts` | 导入：读表单，照服务端判 `parent_id`（不是 id 答 400，不是这本笔记本的页答 404 `page.not_found`，没有这一部分才是根），排队、放到最前。被拒、连接中断与上传的进度由测试经 `answers` 与可注入的传输给出 |
| `e2e/fixtures/wiki-transfer.ts`、`transfer.ts` | `importWith`（在设置里选 zip 与位置导入）、`openReport`；`holdImport`：经 node 的 `http` 慢慢上传、占住笔记本（声明 256 MiB、不超过声明的长度、由测试在 `finally` 里停下） |
| `e2e/stories/transfer/tr2-import.spec.ts`、`tr3-roundtrip.spec.ts`、`tr4-malicious.spec.ts` | 页面版本 |

### 4.2 导入一节与对话框

- **导入一节**（导出一节之前）：一句说明与"导入 zip…"按钮；读者看到"只有能编辑这本笔记本的人才能导入"。
- **对话框**：
  - 说明：上传 zip 期间保持对话框打开，之后在后台进行；`.md` 是页面、文件夹里是同名页面的子页面、其余文件是附件；隐藏的文件与文件夹（如 `.obsidian`）、`__MACOSX`、`Thumbs.db` 不导入；名称修正、已占用的加序号、不能安全导入的条目跳过，报告列出；在笔记本里超过 10 层的页面不导入。zip 至多 `import_max_bytes`（实例信息读不到时不写、不查）。
  - 文件：`accept=".zip,application/zip"`；名称不以 `.zip` 结尾（不分大小写）时提示，仍可发送（服务端判定）。
  - 位置：根（默认）或一页，按路径列出；只列还能再放一层的页（深度小于 10）。页面树读不到时只有根，位置下有 `NotLoaded` 可重试（审查 A-8）。选的页在重读的树里不再可选（别处删除、移得太深）时，下拉显示"请选择位置"（那一页的 id、不可选），提交时位置下说"选的页面已不在可选的位置里：请重新选择"，不发送；随便选一项（根也算）都改掉它（审查 A-3，核对第一轮 F-2）。
  - 发送之前（总设计 4.8）：没有文件、超过 `import_max_bytes`（正好等于照发）时在文件下说明、不发送；列表里有这本笔记本进行中的导入时说明，"导入"标为不可用（`aria-disabled`，仍可聚焦：被拒之后焦点在它上面、重读的列表才显示别人的导入时，焦点不落到对话框外，核对第三轮 H-1），提交时什么都不发。
  - 上传：`<progress>` 与"已上传 n / 总数"；"取消上传"代替"取消"，开始时焦点到它（审查 A-9）；有在途的上传时挂上关标签页的提醒（4.1 的键）。上传期间关对话框（Esc、点外面、关闭按钮）或在应用里离开这一页（`useBlocker`，只在路径变化时拦，同 13.2 第 21 条）先问"取消上传？没有传完的 zip 不会导入"：焦点在"继续上传"，Esc 即继续，继续之后焦点回到"取消上传"；另一个按钮是"取消并关闭"或"取消并离开"。问的时候上传结束了（成功、被拒、中断）就不再问，离开的那次照常离开，下一次上传不带着这次的询问（审查 A-2、A-5，核对第一轮 F-1、F-7）。对话框卸载（离开、换代、角色降为读者）即中止上传；导入一节不在了（离开、换代）就不再重读列表（换代时旧一代的缓存已不在，核对第一轮 F-3、第二轮 G-1、第三轮 H-3）。"取消上传"与"取消"是两个按钮（同一个节点在焦点下变成"取消"，再按一次就关掉了对话框）；发送结束而没有导入、焦点落空时（"取消上传"已不在，也没有出错的字段取得焦点），焦点到"导入"，便于再试（核对第二轮）。
  - 被拒：在对话框里说原因、不关，文件留着可以再发：`transfer.busy`（"已有进行中的导入，可能是别人的"）、`page.not_found`（导入的位置已不存在）、`server_busy`（排队的任务已满，几分钟之后再试）、`storage_full`、`payload_too_large`（zip 超过服务器接收的大小）、`bad_request`（上传没有完整到达或太慢）、`notebook.not_found`、`forbidden`（通用的文案）。连接中断（`TypeError`）：说上传没有完成，服务器可能在接收之前拒绝了它（列出几种原因）或上传太慢，稍后再试；不重试。
  - Chromium 的实际：服务端在读文件之前答 409、浏览器还在发送 32 MiB 的文件时，Chromium 读到 409，对话框说"已有进行中的导入"，控制台记一条 409 的加载失败（TR2 页面版本，连续 8 次一致）。连接中断的说法留给别的浏览器与中间的代理。
  - 每次结束（成功、被拒、停止、中断，连同"取消并关闭"），只要导入一节还在，都重读任务列表（被拒可能是别人的导入；读失败过的列表不再轮询，审查 A-6）；成功时关掉，任务在最前，焦点到它那一行；之后再打开又取消时焦点回到按钮。
- **任务行**：导入的进度照导出（总数为 0 时不定：校验中）；报告与失败的文案按种类（4.1）。

### 4.3 测试

- vitest：
  - 服务：表单的部分与次序、根时不发 `parent_id`、进度、取消。
  - store：不排队、放到最前、丢弃在途的读、`importing`、被拒与停止之后不再提醒、两次同时的上传都结束才不提醒；根 store：附件的上传结束不清掉导入的提醒。
  - 对话框经路由到达设置页（`transfer-import.test.tsx`，30 个，同应用在 StrictMode 里）：编辑者导入到一页、焦点、列表重读；位置的列法；读者；大小与文件名的预查（正好等于上限照发、实例信息读不到时不查）；进行中的导入；每种被拒（再发一次、之后关闭不问）与连接中断（只发一次）；进度、离开的提醒、关闭时的询问（焦点、Esc、继续）；"取消上传"与离开时的询问；问的时候上传结束；选的页被别处删除（经事件流）；页面树读不到；别处退出登录时中止；打开时焦点在文件上，本地的问题同样，上传期间用户放进表单的焦点结束时不动；被拒而重读显示别人的导入时焦点留在对话框里；被拒、停止、停止并关闭之后重读列表，前两者焦点到"导入"；列表读不到时焦点到标题。
  - 报告（`transfer-report.test.ts`）：按种类的失败与问题的文案的表；保留的说明。
- e2e：TR2 页面版本：根与一页之下，焦点、报告；两个经接口的慢上传先占住笔记本（被拒的那个立即停下，另一个在 `finally` 里停下），浏览器的 32 MiB 被拒；TR3：导出、下载、导入到另一本，相同；TR4：恶意的 zip 的报告、不是 zip 的失败。页面版本 `test.slow()`；存下的 zip 轮询到没有。

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
- **M12**：名称不是 UTF-8 的条目按 GBK、CP437 解读；几个 GB 的导入的负载（校验时的解压两遍、每批的附件写入、`ANALYZE` 的时长）；按人的队列上限（[审查](reviews/P6A-import-review.md) B-9）；反向代理的 `client_max_body_size` 与超时（至多 3 小时）写进部署文档（[M12 的移交](../M12-release/handoffs/M7-transfer.md)）。
- **v0.1 收官之后的打磨**：页面菜单的"导入到此页"。

## 9. 结果

### 9.1 A：服务端（2026-10-10，合并 `b60cf66`）

- 提交：
  - 实施：page、asset 与名称 `524fe99`、zip 的读与映射 `5bec183`、后台任务 `6a5b72b`、接口与组合 `c4d2b2e`、名称的决胜 `23f5193`、整个程序、e2e 与 Obsidian `05ae611`；负对照的补测 `e47c22e`、镜像冒烟的实例信息 `6225dfb`。
  - 审查的修复 `66ae232`；修复核对的修复 `571530d`、`3f5d0ba`、`219f22b`、`2fd5e1b`、`8531d9f`、`e95eedf`；CI 的测试不用 testcontainers 的回收器 `77f8632`。合并 `b60cf66`。
- 审查：三位审查者（Opus）。没有高。中 2 条：序号占用同一个库里后面的兄弟原样的名称，库里的链接因此解析到别的页（A-1）；并发的上传不受限，第二个上传传完才被拒、许多上传可以把磁盘写到下限（B-1）。中低 1 条：进程崩溃之后报告的计数与改名的列表丢失（第 7 节要求写明已导入的部分）。低三十余条，都已处理，接受与推后的见审查记录。
- 修复核对六轮：
  - 第一轮两位核对者（服务端；测试与文本）：刚写行的导入被数两遍、导出不看正在上传的、507 不看别的上传的字节、`Check` 里 panic 时占住不放、每拍都重写整个报告、组合根转交保留的名称没有测试；序号的记忆按拼写而不是按键；超时的测试靠 50 毫秒。
  - 第二轮中 2 条：测试的替身答出运行的取消原因，遮住了运行自己的停下检查；已存下的上传字节被数两遍（声明的与存储里的）。另有同时的 `Check` 互相拒绝、提交时刻的双计。
  - 第三至第五轮：上传的准入与队列的计数在并发下的窄窗口（行在提交之中、`Check` 数行与读上传之间、持锁的启动读上传与数行之间）与对应的测试；第五轮把"持锁的先数行再读上传、`Check` 先读上传再数行"定下，`Check` 与导出同时判定时队列可能多一个，接受（`Create` 在锁下才是作数的判定）。
  - 第六轮没有行为问题，改了两处措辞。
  - 逐条见[审查记录](reviews/P6A-import-review.md)。
- 与实施前的设计不同之处（第 3 节已照实际改写）：
  1. 序号跳过同一个库里后面的兄弟原样的名称（`Reserved`，任务按父节点记下计划里每个名称键最后的位置，经组合根转交 page）；单元按名称键与是否附件记下一个键上次给到的序号（同一个键的大量兄弟不再 O(k²)）。原设计只写"后来的加序号"。
  2. 兄弟的次序多一层决胜：同一个键里原样的名称先于修正成它的（`23f5193`），原样的保住名称，指向它的链接照样解析到。
  3. 并发的上传：进程内的 `app.Uploads`（导出与导入共用）：`Check` 占住笔记本、一次一个；被准入的上传在队列里算一个任务，声明的字节里还没存下的算进存储；行在提交之中时持锁的启动不再计它、提交之后都不计；计数的先后见 3.9。原设计只数行。
  4. 声明的 `Content-Length` 超过上限时立即 413、关闭连接；`Check` 按声明的长度核对余量；`Create` 不再核对余量。读文件之前的拒绝 `Connection: close`；文件之前至多 4 KiB；`parent_id` 在 `Check` 之前解析，读者也答 400。
  5. 导入的报告随心跳写（报告变了、至多每 10 秒），收拾保留它、只并入失败；迁移 `00032` 允许运行中的行有报告；读接口在结束之前不交出。原设计只在结束时写。
  6. `Archives.Upload` 答 `Upload{Write, Commit, Abort}`（原写 `CreateImport`）；`Store` 自己取 UUIDv7、答 `Stored{ID, Bytes}`，另有 `Discard`；清扫的端口是 `LiveArchives(ctx, kind, ids)`。
  7. zip：`OpenImport(ctx, id, most)`；`RawEntry` 不带模式与大小，压缩后的大小经 `Packed(i)`，条目头的解压后大小不交出；以 `\` 结尾的名称是目录；`zip.ErrInsecurePath` 不算错；读过预读的字节答 `too_many_entries`，结尾的余量 128 KiB；zip64 定位记录的偏移是负的照 Go 忽略、越过文件的答 `not_zip`（A-2）。
  8. 盘符只认字母加冒号之后是 `/` 或结尾：`a:b.md`、`Q: notes.md` 是要修正的名称（A-4）。
  9. 分类：`name_not_utf8`、`unsafe_path` 在忽略之前核对（忽略的目录下的也进报告）；`__MACOSX` 区分大小写、`Thumbs.db` 不分；重复的目录不报告；`contributed` 的文件不报告。
  10. 读字节：每次 32 KiB，依次核对总数、过大、压缩比；打不开的条目 `unreadable`；`meta.json` 的字节不计入总数、第一个为准。
  11. 映射：同一个键的几个 `.md` 与目录按次序一一配对；只有目录的页在 `too_deep` 里以 `/` 结尾；附件不算一层（深度 10 的页的附件照常导入）；附件的名称截短之后以 `.md` 结尾到得了，再加 `_`。
  12. 报告的路径：`renamed` 与 `too_deep` 是库里的路径，其余跳过的是 zip 里的原名；`renamed` 另有 `to`；至多 1,024 字节。契约写明。
  13. 写入：解析忙时手上已有解析好的页就先写这一批；单元答 503 同样退避重试，每次尝试之前看停下；`do` 失败也删这一批的文件，提交了的单元里因太深而丢下的附件的文件删掉；进度与日志的节点数含跳过的。
  14. 单元答看不到笔记本、未认证时同样失败 `forbidden`（原写笔记本已删时"已不在"：笔记本删除时任务行随之软删除，心跳先看到）。
  15. 统计：sqlc 能解析 `ANALYZE`，各模块的语句照常在 `queries/statistics.sql`；River 的上下文已结束时不做结束时的那次。
  16. asset：`NewBlobs(pool, store, logger)`；`Put` 答新的 `asset.File`；`Attach` 由调用方给出归属（`Owner`）；组合根把 `ErrTooLarge`、`ErrStorageFull` 映射到 transfer 的。
  17. page：并入的核对在单元第一次写时做（什么都没写的单元不碰变更集）；任何并入的单元都不收编辑会话的写；下行迁移把导入的变更集改回 `edit`；`Actor.Valid()` 也要账户；`page.Parsed` 是结构（`Links int`）；`ErrNoParent`；`ImportUnit` 是模块根的接口，`after` 收 `NodeInfo`；`Import` 什么都没写时答零。
  18. 表单：`NewForm`、`Fields`、`Open`、`End`、`InvalidPart`、`FormError`、`ReadCause`；transfer 的 app 另有 `ReadError`；asset 的 `TestReadCause` 移到平台。
  19. 契约：`Content-Transfer-Encoding` 不解码、连接可能被重置；`x-problem-codes` 不列 `payload_too_large`、`bad_request`（平台的）。
  20. 配置：`import_max_entries` 至多 100,000（原写 1,000,000：64 MiB 的中央目录装不下，B-5）；`import_max_bytes` 按 `asset.upload_min_rate` 在 3 小时之内传完（`MaxImportTransfer`，B-8）；`import_workers` 的半数规则在 `export_workers` 合法时才核对；`max_queued` 计入正在上传的。
  21. 清扫两种都扫、错误合并（A-5）。
  22. 组合根：`transfer.Deps` 多 `MaxContentBytes`；交错在新文件 `interleavings_import_test.go`；取消的交错停在第一个单元的附件上，直到心跳读到取消（原写停在第二批之前）。
  23. e2e：`zip-write.ts` 也写坏的 CRC 与记录的注释；`storedImports` 不算 `.tmp`；e2e 的 Postgres `max_connections` 改为 300（每个 worker 的服务与测试的连接池，大机器上超过默认的 100）；镜像冒烟与 S3 核对 `import_max_bytes`。
  24. 网页：导入的失败与问题的码的文案随 A；`problem.transfer.busy` 与按导出写的报告文案随 B（审查 B-3、B-4）。
- Obsidian 的核对（3.17）：`resolve/` 的 27 个样例（P5 已与 Obsidian 1.12.7 逐条核对，133 条链接）各写成一个库，照原样与 NFD 加 `\` 两种 zip 经 serve 导入，每个页、附件在它的路径上，每条链接解析到样例写的目标（`obsidian-verified` 与 `nerve-defined` 都一致）。本阶段没有再启动 Obsidian：库的写法同 `verify-resolve.mjs`。
- 反向对照：
  - 实施之后 49 个：11 个起初没被抓到（6 个存活、5 个变异本身编不过），补测与改写之后全部被抓到（`e47c22e`）。
  - 审查的修复之后 25 个，3 个存活，补测之后被抓到；五轮修复核对的修复之后 10、5、5、5、3 个，存活的补了测试。全部被抓到。
  - 第 6 节的"按条目头而不是实际读出的字节计"写不出来：端口不交出条目头的大小，`archive/zip` 自己拒绝比条目头长的数据。
- CI：分支的 `6225dfb`、`571530d`、`3f5d0ba`、`2fd5e1b`、`77f8632` 上 server、web、image、e2e 全部通过。本机每轮跑全套 Go 测试、golangci-lint、前端检查、`gen-check` 与整个 e2e（223 个）。失败的几次：
  - `c4d2b2e`：镜像冒烟与 e2e S3 漏改了对实例信息的精确比较（`6225dfb` 补上）；`66ae232`：拉取镜像超时；`8531d9f`：被下一次推送取消。
  - `219f22b`、`e95eedf`：bootstrap 在同一个子测试（第 37 个库）失去数据库，此后端口拒绝连接；P6A 之前没有出现过。testcontainers 让 `go test ./...` 的各个包共用一个回收器（Ryuk）的会话，一个包连它出错时终止它、另起一个，新的那个在自己的客户端走光之后清掉整个会话的容器，连同没连上它、还在跑的包的数据库。CI 的测试改为不用回收器（runner 用完即弃），失败时另把容器、磁盘、内存与内核的 OOM 记录写成注解（`77f8632`）。原因是读源码推断的（运行日志要管理员权限），再出现时注解看得出。
- 交给 B 与后面的 M：
  1. B：第 4 节。网页的 `problem.transfer.busy` 只说导出；报告的文案按导出写（`forbidden` 说读、`root_not_found` 说导出的页、`renamed` 说在 Obsidian 里），导入的报告不显示 `missing`、要显示跳过的计数。
  2. M8、M10：第 8 节。
  3. M12：[导入导出的移交](../M12-release/handoffs/M7-transfer.md)第 5 项：按人的队列上限（B-9）、几个 GB 的导入的负载、反向代理的请求体上限与 3 小时、正在上传的导入只在进程内计数、名称不是 UTF-8 的条目。
- 负责人可以改判的取舍：
  - 正在上传的导入只在进程内计数（v0.1 一个进程）；`Check` 与导出同时判定时队列可能多一个，下一个持锁的启动被拒；
  - 导入的报告至多每 10 秒随心跳写一次，进程崩溃时报告少最后几秒的；
  - 看不到笔记本、未认证的单元记 `forbidden`；
  - 序号可能跳过一个空着的数（拼写不同的同一个键、截短的长名称、为太深而丢下的页保留的名称），不撞名；
  - "未命名"存下来是中文，不随界面语言；
  - `import_max_entries` 至多 100,000，上传至多 3 小时；
  - 一个人占满队列推到 M12（B-9）。

### 9.2 B：前端（2026-10-10，合并 `c9a48dd`）

- 提交：实施：前端 `c2b5dd8`、e2e `8b2867a`；审查的修复 `f9c6440`；修复核对的修复 `1a45cb4`、`d200a2e`、`200cf6e`。合并 `c9a48dd`。
- 审查：三位审查者（Opus）。没有高、中高。A、B 各十余条中低与低：上传期间应用内的离开悄悄中止上传（A-2）、选的位置消失时悄悄改成根（A-3）、进度让整个表单重绘（A-1）、几处文案与服务端的实际不符（B-1、B-2、B-5、B-8）。C 二十一条测试的缺口，属实的两条：导入与附件的离开提醒共用键时会被附件清掉（C-1），假的服务端不判 `parent_id`（C-2）。都已处理，推后的见[审查记录](reviews/P6B-import-web-review.md)。
- 修复核对四轮：第一轮 3 条修复带进的行为问题（再次上传时带着上次的询问、位置下拉卡住、别处退出时未处理的拒绝）；第二轮"取消并关闭"不再重读列表；第三轮被拒之后焦点在随即禁用的"导入"上；第四轮修复里没有发现，两处 8b2867a 起就有的提示细节推到打磨。
- 与实施前的设计不同之处（第 4 节已照实际改写）：
  1. 可选的位置只有还能再放一层的页（深度小于 10）。服务端接受深度 10 的页（附件不算一层，库根上的附件会放到那里），对话框不列：库里的页面在那里都会因太深而跳过。
  2. 上传期间在应用里离开同样先问（`useBlocker`），原设计只写关对话框时问；询问只在上传期间。
  3. 选的位置在重读的树里消失时，位置下说明、不发送（原设计没写；实施时悄悄改成根，审查 A-3）。
  4. 进行中的导入时"导入"是 `aria-disabled`，不是 `disabled`（核对第三轮 H-1）。"自己的导入进行中"实际是列表里看得到的进行中的导入（管理员看得到所有人的）。
  5. 每次结束都重读任务列表（原设计只在成功之后）；导入一节不在了就不读。
  6. 被拒的文案：`server_busy`、`payload_too_large`、`bad_request` 用导入自己的；`problem-messages.ts` 不改，导入的码经 `useForm` 的 `texts`。
  7. 报告：失败或取消的导入说已写的页面与附件留在笔记本里（审查 B-6）；不认识的任务种类只显示页、附件、改名。
  8. 编辑器上传文件夹与 `.md` 的提示改为"打成 zip，在笔记本设置里导入"（审查 B-9）；`tree_changed` 只说删除，契约随之。
  9. 假的服务端不自己做读文件之前的拒绝：被拒、连接中断由测试经 `answers` 给；它照服务端判 `parent_id`（审查 C-2）。
  10. e2e：`importWith`、`openReport` 之外加 `holdImport`（经接口慢慢上传、占住笔记本）；页面版本看不到进度（上传在本机一瞬间完成），只看到"完成"。
- 4.3 要求"照实写进测试与文档"的：Chromium 在服务端读文件之前答 409、浏览器还在发送 32 MiB 时读到 409，对话框说"已有进行中的导入"（TR2 页面版本，连续 8 次一致）。连接中断的说法留给别的浏览器与中间的代理。
- 反向对照：实施之后 27 个、审查的修复之后 32 个（一个等价）、四轮核对的修复之后 5、4、4 个，全部被抓到，除第三轮 H-3 的一个（角色降为读者时的重读，没有测试，已记下）。
- CI：分支的 `8b2867a`、`200cf6e` 上 server、web、image、e2e 全部通过（中间三次被下一次推送取消）。本机每轮跑前端的 lint、knip、测试（2477 个）、构建、`gen-check` 与整个 e2e（226 个）。
- 交给后面的：
  1. [M12 的打磨](../M12-release/handoffs/M5-polish.md)第 20 项：成功时"进行中"的提示闪一下（J-1）、被拒之后两条相近的提示（J-2）、双击"取消上传"。
  2. M8、M10、M12 的其余：第 8 节。
- 负责人可以改判的取舍：
  - 第 10 层的页不作为导入的位置；
  - 上传属于对话框：关对话框、离开设置页即中止（问过之后），不在后台继续；
  - 被拒或中断之后不自动重试；
  - "自己的导入进行中"按列表里看得到的判断，看不到的由服务端答 409。
