# M7/P5 导出：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M7/P5 导出 |
| 状态 | 进行中（A：服务端） |
| 基线 | `ce0e39e`（P4 合并、文档补完之后的 main）；本文提交之后开分支 `m7-p5a`，A 合并之后开 `m7-p5b` |
| 上级文档 | [M7 总设计](00-M7-design.md) 4.1、4.8（导入与导出的界面）、4.9、4.10、4.12–4.14、第 5、7–9 节；移交：[M2/P4 只投递的 River 客户端](handoffs/M2-P4-insert-only-client.md)第 1–4 项、[M6 链接与附件、导入、导出](handoffs/M6-links.md)第 5 项；总体设计 3.3、3.5、8.5、13.1 第 2、10、17、23、25 条 |

---

## 0. 分两部分

P5 把一个笔记本或一棵子树导出为 zip。照 P3、P4 的先例分开实现、审查、核对修复、合并：

- **A：服务端**：`platform/jobs` 的队列、超时与只投递的客户端；`platform/postgres` 的 `TxFrom`、`WithinSnapshot`；`shared.Actor` 的 `JobID`；transfer 模块（`transfer_jobs`、开始导出、导出的任务、映射与 `meta.json`、导出贡献者、读与取消、签名的下载、到期、收拾、清扫、生命周期）；契约；导出的库在 Obsidian 里逐条解析的核对。e2e：TR1 的接口版本。
- **B：前端**：transfer 的服务；笔记本设置的"导入与导出"一节（任务列表、导出整个笔记本、取消、下载、报告）；页面标题旁的菜单"导出此页"。e2e：TR1 的页面版本。

第 1、2 节两部分共用，第 3 节是 A 的设计，第 4 节是 B 的，第 5–9 节共用。

## 1. 基线

- **后台任务**：`platform/jobs` 只有服务的客户端（`jobs.New`）：默认队列 2 个 worker，River 默认的任务超时 1 分钟、`RescueStuckJobsAfter` 1 小时；`Job` 只有 `Add` 与 `Periodic`。模块的任务都是定时的：会话清理、编辑会话清理、软删除的清理、附件的孤儿清扫（`SweepTimeout` 50 分钟，留在 River 的 1 小时救援之内）。没有由请求投递的任务。
- **事务**：`TxManager.WithinTx`、`postgres.DB(ctx, pool)`、`InTx`；没有 `TxFrom`、没有只读的快照。
- **`shared.Actor`**：`UserID` 与两种凭据（`SessionID`、`APITokenID`）；page 的 `clientOf` 按凭据给出 `web`、`api`。
- **读端口**：page 有 `NewAssetNodes`、`NewLinkTargets`（`Subtree`、`Content`），没有按范围读整棵树与正文的；asset 的模块根有 `NewEmbeds`、生命周期与活动，没有给别的模块读文件的端口（总设计 4.1 的 `asset.NewBlobs` 还没出现：P2 的上传在模块之内）；linking 的 `page_links` 有 `resolved_id`，没有"哪些页是这些链接的目标"的端口。
- **组合根**：模块按 page → asset → linking 建；`purgers(pool, tx, store, logger)`；命令行的组合规则（`archtest/composition_test.go`）禁止它们建 `jobs.New` 与存储。
- **前端**：笔记本设置有"常规"与"成员"两节，看得到笔记本的人都进得来；页面标题旁只有写者的"编辑"。
- **工具**：`tools/md-fixtures/obsidian/verify-resolve.mjs` 在隔离的 Obsidian 里核对 `resolve/` 样例的解析。

## 2. 目标与范围

**做**（总设计第 7 节 P5 一行）：第 0 节的两部分；13.1 第 10 条（transfer 的日志）、第 23 条（队列、超时、只投递的客户端、心跳）、第 25 条（导出下载的密钥）、第 2、17 条（`Actor.JobID`）随实现改写。M2/P4 只投递客户端的移交第 1–4 项、M6 移交第 5 项在这里落实。

**不做**：

- 导入（P6）：它的接口、队列 `transfer_import` 的 worker、`transfer.import` 动作、`imports/` 的清扫、`jobs.import_workers` 与 `transfer.import_*` 的配置随 P6。P5 的表、状态机与报告的形状照两种任务定下（`kind` 有 `import`），P6 不再改表。
- 删除任务的接口：成功的导出只留最新的一份，24 小时之后到期，不另给删除。
- 命令行的导出（总设计第 2 节"不做"；组合规则不改）。
- 导出贡献者的注册者（M10）：组合交空，模块根有示例的测试。

## 3. A：服务端

实现、审查与修复核对之后照实际改写（第 9.1 节）。

### 3.1 文件

| 位置 | 内容 |
|---|---|
| `platform/jobs/jobs.go` | `Config.Queues`、`Config.RescueAfter`；`Job.Start` |
| `platform/jobs/inserter.go` | `Inserter`：只投递的客户端，`InsertTx` |
| `platform/postgres/tx.go` | `TxFrom`；`TxManager.WithinSnapshot` |
| `shared/actor.go` | `Actor.JobID` |
| `modules/page/export_nodes.go` | `page.NewExportNodes(pool)`：范围里的节点与正文 |
| `modules/asset/blobs.go` | `asset.NewBlobs(pool, store)`：节点的文件（`Of`、`Open`） |
| `modules/linking/module.go` | `linking.NewLinkedPages(pool)`：哪些页是范围里的链接的目标 |
| `modules/transfer/` | 模块根（`module.go`、`contributors.go`、`lifecycle.go`、`purgers.go`）；`domain`（状态、动作、错误、映射、`meta.json`、报告）；`app`（开始、任务、读、取消、下载、到期、收拾、清扫、跟随、清理）；`adapter/postgres`（sqlc）、`adapter/river`、`adapter/http`（含生成的 `gen`）、`adapter/mac`、`adapter/archive`（写 zip）、`adapter/files` |
| `migrations/sql/00029_transfer_transfer_jobs.sql` | 表、检查、索引；`deploy/runtime-grants.sql` |
| `api/modules/transfer.yaml` | 契约（3.6） |
| `bootstrap/transfer.go` | 组合：端口的适配、注册者、任务 |
| `tools/md-fixtures/obsidian/verify-export.mjs` | 导出的库在 Obsidian 里逐条解析（3.15） |

### 3.2 平台：任务的队列、超时与只投递的客户端

- **队列**：`jobs.Config.Queues map[string]int`（队列名 → worker 数），与默认队列（2 个，定时任务）一起交给 River。组合根给 `transfer_export: jobs.export_workers`（默认 1）。导出不占定时任务的队列，几个小时的导出也不挡住清理。
- **超时**：worker 自己的 `Timeout()`（River 的接口）是 `transfer.job_timeout`（默认 6 小时）。`jobs.Config.RescueAfter` 是 River 的 `RescueStuckJobsAfter`，组合根给 `transfer.job_timeout + 1 小时`：`MaxAttempts` 为 1 的任务被救援就丢弃，救援要晚于任何 worker 的超时。附件清扫的 `SweepTimeout` 的注释随之改（它不再贴着 1 小时）。
- **启动时的收拾**：`jobs.Job.Start func(ctx) error`，在 `Runner.Start` 里、River 开始取任务之前依次运行；失败时 `Start` 失败（serve 退出，与 River 起不来相同）。transfer 用它把上一个进程留下的"运行中"的任务记成失败（3.12）：此时这个进程还没运行任何任务，单实例下它们都已中断。
- **只投递的客户端**：`jobs.NewInserter(pool) (*Inserter, error)`：不配 `Queues`、不 `Start` 的 River 客户端，`InsertTx(ctx, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) error`。投递与业务的行同一个事务：回滚时任务也不存在；River 在同一个事务里 `NOTIFY`，服务的客户端随即取到。`platform/jobs` 仍只导入 River 与 pgx：事务由调用方经 `postgres.TxFrom` 取出交进来（3.3）。
- **组合规则**：`archtest/composition_test.go` 的禁止集加上 `jobs.NewInserter`（命令行的组合不投递任务，M2/P4 移交第 3 项）；serve 到达它。
- **停机**：HTTP 先于任务停（M1/P4 文档 3.4，不变）：正在处理的请求投递任务时任务一侧还在。River 在 `jobs.shutdown_timeout` 之后取消任务的上下文，导出照 3.8 记成失败。

### 3.3 平台：快照与事务

- **`TxFrom(ctx) (pgx.Tx, bool)`**：`WithinTx`、`WithinSnapshot` 放进上下文的事务，给要把事务交给别的库的适配器（transfer 的 river 适配器投递任务）。
- **`TxManager.WithinSnapshot(ctx, fn)`**：`REPEATABLE READ READ ONLY` 的事务，`fn` 的上下文带着它，读端口经 `postgres.DB(ctx, pool)` 进入；提交与回滚照 `WithinTx`（不随调用方取消，限时 `database.commit_timeout`）。上下文已带事务时返回错误（快照必须是最外层，否则读到的不是一个时刻）。`shared` 加 `Snapshots` 端口（`WithinSnapshot`），组合根照 `TxManager` 在编译时核对。

### 3.4 `shared.Actor` 的 `JobID`

- `Actor` 加第三种凭据 `JobID`：后台任务代账户执行时是任务的 id；三种恰好一个（注释写明，`Actor.Valid()` 核对，测试钉住）。
- 导出的任务以 `Actor{UserID: 发起人, JobID: 任务}` 判定权限（access 只看 `UserID`，判定不变）；日志带 `job_id`。P6 的写入单元同样以它调用，客户端用任务记下的（不按凭据推断：page 的 `clientOf` 不会见到任务的 `Actor`）。

### 3.5 `transfer_jobs`

```sql
CREATE TABLE transfer_jobs (
    id uuid PRIMARY KEY,                                   -- UUIDv7
    notebook_id uuid NOT NULL REFERENCES notebooks ON DELETE RESTRICT,
    root_id uuid,                                          -- 不建外键：指向节点会挡住节点的清理
    kind text NOT NULL CHECK (kind IN ('import', 'export')),
    state text NOT NULL CHECK (state IN ('queued', 'running', 'succeeded', 'failed', 'cancelled', 'expired')),
    name text NOT NULL,                                    -- 导出的根（笔记本或页）的名称：任务列表与下载的文件名
    created_by_id uuid NOT NULL REFERENCES users,
    client text NOT NULL CHECK (client IN ('web', 'api')),
    progress_done bigint NOT NULL DEFAULT 0, progress_total bigint NOT NULL DEFAULT 0,
    cancel_requested_at timestamptz, heartbeat_at timestamptz,
    started_at timestamptz, finished_at timestamptz,
    report jsonb,                                          -- 结束时写下（3.9）
    result_bytes bigint,                                   -- 导出的 zip 的字节数
    created_at timestamptz NOT NULL, deleted_at timestamptz
);
```

- **状态机的检查**：`queued` 没有 `started_at`；`running` 有 `started_at` 与 `heartbeat_at`；结束的四种（`succeeded`、`failed`、`cancelled`、`expired`）有 `finished_at`、有 `report`，没结束的两种都没有；`expired` 只是导出的；`result_bytes` 恰好在导出的 `succeeded`、`expired` 时有；`progress_done ≤ progress_total`。转移只经带 `WHERE state = …` 的语句：`queued → running | cancelled`、`running → succeeded | failed | cancelled`、`succeeded → expired`；收拾的 `running → failed`。
- **索引**：笔记本的列表 `(notebook_id, created_at DESC, id DESC) WHERE deleted_at IS NULL`；每人每本笔记本一个进行中的导出：唯一索引 `(notebook_id, created_by_id) WHERE kind = 'export' AND state IN ('queued', 'running') AND deleted_at IS NULL`（P6 加导入的那一个）；进行中的 `(state) WHERE state IN ('queued', 'running')`（数量、收拾）；到期 `(finished_at) WHERE kind = 'export' AND state = 'succeeded'`；清理 `(deleted_at) WHERE deleted_at IS NOT NULL`。
- 不存凭据的 id（会话清理会删 `auth_sessions` 的行）。写进 `runtime-grants.sql`。

### 3.6 契约

`api/modules/transfer.yaml`：

| 操作 | 说明 |
|---|---|
| `startExport`：`POST /api/v0/notebooks/{notebook_id}/exports` | 请求体 `{ "root_id": uuid \| null }`（可省）；答 202 `TransferJob` |
| `listTransferJobs`：`GET /api/v0/notebooks/{notebook_id}/transfer-jobs?cursor=&limit=` | 新的在前，一页至多 50（默认 20）；本人的，笔记本管理员是全部 |
| `getTransferJob`：`GET /api/v0/transfer-jobs/{job_id}` | `TransferJobDetail`：`TransferJob` 加报告的问题 |
| `cancelTransferJob`：`POST /api/v0/transfer-jobs/{job_id}/cancel` | 答 200 `TransferJob` |
| `downloadExport`：`GET /api/v0/transfer-jobs/{job_id}/download?e=&s=` | `x-raw`、公开（`security: []`），签名就是凭据（3.11） |

- `TransferJob`：`id`、`notebook_id`、`root_id`（可为 null）、`name`、`kind`（`import`、`export`）、`state`、`client`、`created_by`（`user_id`、`display_name`，经 identity 的目录）、`created_at`、`started_at`、`finished_at`、`progress`（`done`、`total`）、`result_bytes`、`report`（结束之后：`failure`、`counts`）、`download`（成功的导出：`url`、`expires_at`）。列表里的每一项都签下载地址（签名只是计算）。
- `TransferReport`：`failure`（失败时的原因码，否则 null）、`counts`（`pages`、`attachments`、`renamed`、`missing`、`skipped`，P5 不用 `skipped`，恒为 0）。`TransferJobDetail` 另有 `problems`（至多 1,000 条：`path`、`code`、`to`）与 `problems_truncated`。原因与问题都是码，前端按码给文案（总设计 4.8）。
  - 失败的码（P5）：`interrupted`（服务停止或重启）、`timeout`、`forbidden`（运行时已不能读这个笔记本）、`root_not_found`（运行时导出的页已不在）、`storage_full`、`contributor_conflict`、`internal`。
  - 问题的码（P5）：`renamed`（`to` 是导出用的路径）、`file_missing`（附件的文件不在存储里）。
- 错误码：`transfer.not_found`（404：没有、已删、看不到、不是本人的而又不是笔记本管理员）、`transfer.busy`（409：本人在这本笔记本有排队或运行中的导出）、`transfer.not_cancellable`（409：已结束）；导出的页不是这本笔记本里活着的页答 `page.not_found`（404）；平台码 `server_busy`（503，排队的任务已满，带 `Retry-After`）、`storage_full`（507）。
- 下载的操作照 P2 的 `x-raw` 规则（代码生成排除，契约测试按描述核对状态、头与媒体类型）。

### 3.7 开始导出

1. 不加锁的预检之后进事务：工作区行 `FOR SHARE` → 笔记本行 `FOR SHARE` → 判定 `transfer.export`（读者；看不到答 `notebook.not_found`）→ 有 `root_id` 时它是这本笔记本里活着的页（附件不行）→ 事务级的咨询锁（一个常数键，只有任务的创建取它）之下数全部排队与运行中的任务，到 `transfer.max_queued` 答 503 → 本人在这本笔记本有进行中的导出答 409（唯一索引兜底，撞上同样答 409）→ 磁盘余量低于 `storage.min_free_bytes` 答 507 → 写行（`queued`，`name` 是此刻的笔记本名或页名，`client` 按凭据）并投递（队列 `transfer_export`，`MaxAttempts` 1，参数只有任务的 id）。
2. 答 202 与任务。日志：`notebook_id`、`job_id`、`user_id`、`client`。

### 3.8 导出的任务

worker 只调用用例 `Export.Run(ctx, jobID)`：

1. **开始**：`queued → running`（`started_at`、`heartbeat_at`）。没有这样的行（已取消、已删）就什么都不做、成功返回。
2. **心跳与取消**：另起一个 goroutine，每秒在池上（不在快照里）写一次 `heartbeat_at` 与进度，同一条语句读回 `cancel_requested_at` 与 `deleted_at`：有取消就以原因"取消"取消任务的上下文，行被删（笔记本删除）就以原因"已删"取消。进度的写入因此不进快照、不占快照的连接；取消至多一秒生效。
3. **判定**：以 `Actor{UserID, JobID}` 再判定 `transfer.export`（看不到、不是读者：失败 `forbidden`）。
4. **写 zip**：`store.Create("exports/<job id>.zip")`（余量不足：`storage_full`），经 `adapter/archive` 写：
   1. **快照**（`WithinSnapshot`）：范围里活着的节点（`page.ExportNodes`：id、父节点、类型、名称、标题键、次序、正文是否为空、更新时刻；有 `root_id` 时它不再是活着的页：失败 `root_not_found`）→ 没有正文、有子节点的页里哪些是范围里的链接的目标（`linking.LinkedPages`：来源是范围里的页，含属性链接；索引一页只记前 10,000 条，13.1 第 31 条）→ 附件的 blob（`asset.Blobs.Of`）→ 映射（3.9）→ 贡献者（3.10）→ 按映射的次序分批（200 页一批）读正文（`page.ExportNodes.Contents`），逐字节写 `.md`，进度随之前进；只有目录的页处理到就算一个。之后提交。
   2. **附件**：快照之外，按映射的次序经 `asset.Blobs.Open` 读文件写进 zip；文件不在的写进报告（`file_missing`），不让导出失败。不在 GB 级的复制期间持着快照与连接。
   3. **`.nerve/meta.json`** 最后写（只列写进 zip 的节点）。关闭 zip，`Commit`。
5. **结束**（`context.WithoutCancel`，限时 1 秒，在停机的宽限之内）：
   - 成功：一个事务里 `running → succeeded`（`result_bytes`、`report`、`name` 改成快照里的根名），并把本人在这本笔记本之前成功的导出记成 `expired`；提交之后删掉它们的文件（删不掉的留给清扫）。
   - 取消：`Abort` 文件，`running → cancelled`，报告写明到哪里（进度与问题）。
   - 行已删：`Abort` 文件，不写行。
   - 失败：`Abort` 文件，`running → failed`，原因按上下文的原因与错误：超时（River 的期限）`timeout`，停机 `interrupted`，判定 `forbidden`，`root_not_found`，存储写满 `storage_full`，贡献者冲突 `contributor_conflict`，其余 `internal`（错误本身记日志，不进报告）。
   - 写结束状态失败时（数据库不可用）返回错误；行留在"运行中"，由收拾记成失败（3.12），已提交的文件由清扫删掉。
6. 日志：开始与结束各一条，`notebook_id`、`job_id`、`user_id`、`client`、节点数、字节数、原因的码；文件名、zip 里的路径不进日志。

### 3.9 映射、报告与 `meta.json`

`domain.Plan`：纯函数，节点、链接目标与 blob 进，条目、改名与 `meta` 的节点出，逐项的表格测试。

- **根目录**：zip 里有一层根目录，整个笔记本时是笔记本名（遵守标题的规则，总体设计 3.3），子树时是那一页的名称；库是它里面的内容。子树的那一页是库的根下的一页：`<页>/<页>.md`，它的子节点在 `<页>/<页>/`。
- **页面**：在目录 `D` 里的页 `E` 写 `D/E.md`，子节点在 `D/E/`。`E.md` 写出的条件：有正文；或没有子节点（空的 `.md`）；或没有正文、有子节点而是范围里的链接的目标（空的 `.md`，M6 移交第 5 项）。只有子节点、没有正文、没被链接的页只有目录。
- **附件**：在它的父页的目录里（根下的在库的根下），名称就是文件名。
- **兄弟的次序**：`sort_order`，再按 id（与树相同）；条目、`meta` 的节点都按深度优先的这个次序。
- **冲突**：页面 `N.md` 有子节点时它的目录 `N.md/` 与兄弟 `N` 的文件 `N.md` 同名（按标题键比较：文件系统可能不分大小写）。有目录的那一页改名：取 `N.md 2`、`N.md 3`……里第一个标题键不与这个目录里任何名称、文件、目录相同的，文件与目录一起改；报告记 `renamed`（原路径与 `to`），指向它的链接在 Obsidian 里解析不到（报告的说明写明）。附件的名称不以 `.md` 结尾、与页面共用命名空间（P2），所以不会有别的冲突；映射对任意输入仍核对"一个目录里没有同键的条目、没有文件与目录同键"，不成立时导出失败 `internal`（不变式）。
- **zip 的条目**：名称是 `根/路径`，`/` 分隔；Go 的 `archive/zip` 对非 ASCII 的名称设 UTF-8 标志、超过 4 GiB 用 zip64。修改时刻：页面是节点的更新时刻，附件是 blob 的创建时刻。已经压缩过的类型（png、jpeg、gif、webp、avif；音视频；zip、gzip、7z、rar；docx、xlsx、pptx、epub 等 zip 容器）以 `Store` 写入，其余 `Deflate`。不写目录条目（文件的路径隐含了目录；只有目录、没有子节点的页不存在：没有子节点的页写 `.md`）。
- **`.nerve/meta.json`**（在根目录里）：

  ```json
  { "format": 1, "exported_at": "2026-10-09T12:00:00Z",
    "notebook": { "id": "…", "name": "…" }, "root": null,
    "nodes": [ { "path": "项目A.md", "kind": "page", "id": "…", "sort_order": 1.5 },
               { "path": "项目A/需求/", "kind": "page", "id": "…", "sort_order": 3 } ],
    "contributed": [] }
  ```

  `path` 相对库的根：页面是它的 `.md`，只有目录的页是目录（以 `/` 结尾），附件是它的文件；`root` 在子树时是 `{ "id", "name" }`。P6 据它恢复兄弟的次序，`id` 只作参考。
- **报告**：计数（页、附件、改名、缺文件）与前 1,000 条问题（路径至多 1,024 字节，在字符边界截断，`problems_truncated` 记下截断）。

### 3.10 导出贡献者

- `transfer.ExportContributor`：`Contribute(ctx, scope ExportScope, sink ExportSink) error`，在快照里运行（读端口经上下文进入它）。`ExportScope`：笔记本 id、根（可为 null）、范围里每个节点的 id、类型与在库里的路径。`ExportSink.Add(path, content []byte) error`：路径相对库的根、`/` 分隔、每一段遵守标题的规则、不在 `.nerve/` 之下；与节点的路径、已加的文件按标题键相同，或文件与目录相撞（加的文件的某一级是节点的文件，或加的文件是节点的目录）时返回错误，导出失败 `contributor_conflict`。加的文件写进 zip，路径记进 `meta.json` 的 `contributed`。
- 按登记的次序调用，第一个错误即停（导出失败：冲突是 `contributor_conflict`，其余 `internal`）。
- M7 组合交空（`transfer.Deps.Contributors`）；模块根的示例测试（`contributors_test.go`）在真的数据库上登记两个：次序、第一个错误、冲突、加的文件进 zip 与 `contributed`。整个程序上的行为测试在 M10 注册时加（12.4 的通用规则：组合根交空时那条测试失败）。

### 3.11 读、取消与下载

- **读**：`getTransferJob` 先读行（没有、已删：404 `transfer.not_found`），再判定 `transfer.read`（读者；看不到这本笔记本：同样 404），不是本人的而又不是笔记本管理员：404。列表照样判定，非管理员只列本人的。读与取消都重新判定读权限（报告里有页的名称与路径）。
- **取消**：判定 `transfer.cancel`（读者），只许发起人与笔记本管理员（否则 404）；锁任务行：`queued → cancelled`（直接结束，River 之后取到它时什么都不做）；`running` 记下 `cancel_requested_at`（重复取消不改时刻）；已结束答 409 `transfer.not_cancellable`。只写任务行，不碰笔记本行，与任务的读不成环。
- **签名**：`HMAC-SHA256(key, "export-download" ‖ job id 的 16 字节 ‖ e 的 8 字节大端)` 截成 16 字节、无填充的 base64url；`e = (⌊now / 1 小时⌋ + 2) × 1 小时`（1–2 小时有效，同一小时签出的相同）。密钥 `SigningKeys.Derive(transfer.DownloadKeyInfo)`，info `nervewiki export-download mac v1`，已知答案的测试钉住（13.1 第 25 条）。只有核对过读权限的读（列表、单个）签。
- **下载**：`GET /api/v0/transfer-jobs/{job_id}/download?e=…&s=…`，照 P2 的严格读法（路径的 id 是规范写法，转义过的答 404；参数依次是 `e`、`s`，各恰好一次，不反转义；`e` 规范的十进制；`s` 22 个 base64url 字符；别的参数答 404），在任何查询之前核对签名；签名对、未到期、任务是活着的、成功的导出、文件在，才下发，否则一律 404 `not_found`。经 `API.Stream`（无请求体，写出按 `asset.upload_min_rate` 放宽截止时间），公开的操作走平台的 `anonymous` 桶；停机开始之后答 503。头：`Content-Type: application/zip`、`Content-Disposition: attachment; filename="<ASCII 兜底>.zip"; filename*=UTF-8''<名称>.zip`、`/api/` 默认的 `no-store`、`Content-Security-Policy: sandbox; default-src 'none'`；`http.ServeContent` 支持 `Range`（断点续传），去掉对改动的条件（照 P2）。

### 3.12 到期、收拾、清扫与生命周期

- **到期**（定时，每 15 分钟）：成功的导出 `finished_at` 早于 `transfer.export_ttl`（默认 24 小时）的，`succeeded → expired`，提交之后删文件；删不掉的留给清扫。
- **收拾**：serve 启动时（`jobs.Job.Start`，River 取任务之前）把全部"运行中"记成失败 `interrupted`；之后每 5 分钟把心跳早于 `transfer.heartbeat_timeout`（默认 5 分钟）的"运行中"同样记成失败。排队的照常由 River 运行。报告写明中断（计数取已写的进度）。
- **清扫**（定时，每天）：`exports/` 下一天以前的文件，没有活着的成功导出对应的，删掉（结束状态写失败时已提交的文件、到期与"只留最新"删不掉的、笔记本删除之后的）。P6 加 `imports/`。
- **生命周期**：笔记本删除的订阅者软删除这些笔记本的任务（三条路径都经笔记本删除事件）：排队的被取到时什么都不做，运行中的下一次心跳看到行已删、停下；下载与读随之 404。清理器 `transfer_jobs` 排在 notebooks 之前（`notebook_id` 是 `RESTRICT`）：一批一个事务、`SKIP LOCKED`，先删文件、再删行。

### 3.13 配置

| 键 | 默认 | 约束 |
|---|---|---|
| `transfer.export_ttl` | 24 h | 至少 10 分钟 |
| `transfer.job_timeout` | 6 h | 1 分钟到 7 天 |
| `transfer.heartbeat_timeout` | 5 min | 至少 1 分钟（心跳每秒一次）、短于 `job_timeout` |
| `transfer.max_queued` | 20 | 至少 1 |
| `jobs.export_workers` | 1 | 1–8 |

River 的救援（`job_timeout + 1 小时`）由组合根算出，不是配置。各项进 `LogValue`；`config.yaml` 的注释写明。`config.test.yaml` 不改（e2e 的导出几秒就完成）。

### 3.14 组合根、权限、交错与日志

- **组合根**：transfer 在 asset 之后建：`transfer.New(Deps{Pool, Tx, Snapshots, Store, Clock, Logger, Authorizer, Workspaces, Notebooks, Names, Nodes: page.NewExportNodes(pool), Blobs: asset.NewBlobs(pool, store), Linked: linking.NewLinkedPages(pool), Inserter, DownloadKey, Contributors: nil, …})`；它的 `Jobs()`（导出的 worker、收拾含 `Start`、到期、清扫）进 `jobs.New`，`Queues` 与 `RescueAfter` 随之；笔记本删除的注册者加 transfer；`purgers` 在 notebooks 之前加 transfer 的。命令行的组合（`notebookRegistrants`、`purgers` 只凭连接池与存储）照旧到不了 `transfer.New` 与 `Inserter`。
- **权限**：规则表加 `transfer.export`、`transfer.read`、`transfer.cancel`（都是读者）；`transfer.Actions()` 进动作表的核对；权限矩阵加四个操作的每一格（下载是公开的、凭签名，单独测）。
- **交错**（`bootstrap/interleavings_transfer_test.go`，13.4 第 4 条）：任务的创建与笔记本的删除（两种先后：先删答 404；先建则任务随笔记本软删除、worker 什么都不做）；取消与运行中的任务（在快照与附件之间停住的任务，取消之后结束为 `cancelled`、文件不留）；快照与同时的保存（任务停在快照里，另一个连接保存一页并提交，zip 里是保存之前的正文）。结束时核对不变式：没有停在"运行中"的任务，没有活着的成功导出之外的 `exports/` 文件。
- **日志**（13.1 第 10 条）：只记 `notebook_id`、`job_id`、`user_id`、`client`、数量与原因的码；文件名、zip 里的路径、签名不进日志。

### 3.15 Obsidian 的核对

- `bootstrap/transfer_obsidian_test.go`：对 `tools/md-fixtures/resolve/` 的每个样例建一本笔记本（样例的页、附件，图片是真的 PNG；第 i 条链接写在与它出发的页同一个父页下的页 `q<i>` 里，同 `verify-resolve.mjs` 的做法），经接口导出，核对 zip 的结构与映射（每一页、每个附件、只有目录的页、被链接的空页）。环境变量 `NWIKI_EXPORT_VAULTS=<目录>` 设了时另写出每个样例的 zip 与"每个 `q<i>.md` 在本系统里解析到的路径"。
- `tools/md-fixtures/obsidian/verify-export.mjs prepare <workdir> <vaults>`：解开每个 zip 为一个库（去掉根目录，打开"检测所有类型的文件"），隔离的用户数据目录；`check`：在 Obsidian 里读每个 `q<i>.md` 的解析，与本系统的比较：`obsidian-verified` 的样例必须一致，`nerve-defined` 的只报告差异。在 Obsidian 1.12.7 里跑一次，结果写进第 9.1 节。只启动隔离的实例，用完按它自己的 PID 停掉。

### 3.16 e2e：TR1 的接口版本

- `e2e/fixtures/transfer.ts`（PAT 的导出、轮询到结束、下载）、`e2e/fixtures/zip.ts`（读 zip：开发依赖 `fflate`），数据库断言 `e2e/fixtures/assert/transfer.ts`（13.4 第 3 条：任务行的状态、`client`、发起人、进度、字节数、报告的计数；存储目录里的文件与字节）。
- `stories/transfer/tr1-export.spec.ts` 的接口版本：一本笔记本（有正文的页、只有子页的页、被链接的空页、根下与页下的附件、`N` 与 `N.md` 的冲突），导出整个笔记本与一棵子树：zip 的条目与字节、`meta.json`（节点的路径、次序、`root`）、改名进报告；第二次导出成功之后第一次的记成已过期、文件删掉；签名地址改一个字符答 404。

## 4. B：前端

A 合并之后细化，实现、审查与修复核对之后照实际改写（第 9.2 节）。

- **服务**：`services/transfer.service.ts`：`startExport`、`listTransferJobs`、`getTransferJob`、`cancelTransferJob`（经会话的客户端）；下载是 `<a href={download.url} download>`，不经客户端。
- **笔记本设置的"导入与导出"**（`settings/transfer`，看得到笔记本的人都进得来）：
  - "导出整个笔记本"按钮（确认对话框说明 zip 的结构与 Obsidian 的库一致、成功的导出保留 24 小时、只留最新的一份）；409 `transfer.busy` 说已有进行中的导出。
  - 任务列表：SWR 键 `["transfer-jobs", 笔记本 id]`，有排队或运行中的任务时 `refreshInterval` 每秒读一次，否则不轮询；游标分页"加载更多"。每一项：名称（整个笔记本或页名）、类型、状态与进度（`<progress>`，读屏读"n / 总数"）、发起人（管理员看到别人的时）、时刻；成功的导出有"下载"，进行中的、本人的或管理员的有"取消"；结束的有"报告"（展开时读 `getTransferJob`：计数、原因、问题，按码给文案）。重新加载页面之后照样找得到。
- **"导出此页"**：页面标题旁加一个菜单（读者也有），一项"导出此页"：确认之后开始，导航到设置的"导入与导出"、焦点在新任务那一行。
- **i18n**：中英文的状态、原因与问题的码、按钮与说明。
- **vitest**：服务；列表（轮询只在进行中、取消、下载的地址、报告按码、加载更多）；导出的对话框（409、503、507 的文案）；页面菜单（读者有、开始之后导航）；经路由到达。
- **e2e**：TR1 的页面版本：设置里导出整个笔记本、等到成功、点下载（`waitForEvent("download")`）、读 zip 核对结构与 `meta.json`；页面菜单导出此页；同一组数据库断言。

## 5. 实施步骤

| 步 | 内容 | 提交 |
|---|---|---|
| A1 | 平台：`Queues`、`RescueAfter`、`Job.Start`、`Inserter`；`TxFrom`、`WithinSnapshot`；`Actor.JobID`；组合规则 | `jobs, postgres, shared: queues, an insert-only client, snapshots and the job's actor (M7/P5A)` |
| A2 | 迁移与授权；transfer 的 domain（状态、映射、`meta.json`、报告）、postgres 适配器 | `transfer: the jobs' rows and the export's mapping (M7/P5A)` |
| A3 | 读端口（page、asset、linking）；开始导出、导出的任务、`adapter/archive`、贡献者 | `transfer: exports run as background jobs (M7/P5A)` |
| A4 | 契约与 HTTP：开始、读、列表、取消；签名的下载 | `transfer: the API and the signed download (M7/P5A)` |
| A5 | 到期、收拾、清扫、生命周期、清理器；配置；组合根；整个程序、交错、权限矩阵、运行时角色 | `transfer, bootstrap: jobs expire, are rescued and follow their notebooks (M7/P5A)` |
| A6 | Obsidian 的核对；e2e TR1 的接口版本；总体设计的修订 | `e2e, tools: exports end to end and in Obsidian (M7/P5A)` |
| B1 | 服务、设置的"导入与导出"、页面菜单、i18n、vitest | `web: exports from the notebook's settings and a page's menu (M7/P5B)` |
| B2 | e2e TR1 的页面版本 | `e2e: exports in the browser (M7/P5B)` |

## 6. 测试与验证

- **平台**：`Queues` 交给 River（假的客户端看配置）、`RescueAfter` 小于某个 worker 的超时时 `New` 失败；`Job.Start` 依次运行、在 River 之前、失败时 `Start` 失败；`Inserter` 在事务里投递、回滚时没有任务（真的数据库）；`WithinSnapshot` 是只读的（写答错误）、看不到之后提交的写、嵌套时报错；`TxFrom`。
- **transfer 的 domain**（表格）：映射的每一条（有正文、空而无子节点、只有子节点、被链接的只有子节点、根下的附件、页下的附件、子树的根、深的树、次序、`N` 与 `N.md` 的冲突、改名之后再撞、标题键的大小写与 `ß`）；`meta.json` 的金样；报告的截断（1,000 条、1,024 字节、字符边界）；压缩方法的表；状态的转移。
- **transfer 的 app**（假的端口）：开始导出的每个码（看不到、页不在或是附件、503、409、507）；任务的每种结局（成功并让旧的过期、取消、行已删、超时、停机、判定失败、页不在、写满、贡献者冲突与出错、结束状态写不进）；心跳写进度、读回取消；附件的文件不在进报告；贡献者的路径规则。
- **postgres 适配器**（真的数据库）：状态机的检查拒绝不一致的行；唯一索引；转移语句只在对的状态生效；列表的游标与可见性；到期、收拾、清理的语句（`SKIP LOCKED`）。
- **HTTP**：处理器的表格（202 与每个码）；签名的已知答案；同一小时同一地址；改每一个参数、多出与重复的参数、非规范的写法、过期都答 404；下载的头、`Range`；停机 503；契约测试。
- **整个程序**（13.1 第 21 条，组合根交空时失败）：真的 River 跑一次导出（笔记本与子树），zip 的结构与 `meta.json`；被链接的空页（linking 的端口经组合根到达：交空时它只有目录，测试失败）；启动时的收拾；笔记本删除之后任务软删除、下载 404；清理器先删文件；到期；清扫。
- **交错、权限、运行时角色**：3.14；运行时角色的测试跑一次导出（`transfer_jobs` 的授权）。
- **Obsidian**：3.15。
- **e2e**：TR1（A 接口、B 页面）；全部已有的故事照常通过。
- **反向对照**（每个都要有测试失败）：快照不是只读或不是 `REPEATABLE READ`；任务不与行同一个事务投递；没有队列（导出进默认队列）；超时用 River 的默认；救援不晚于超时；`Start` 在 River 之后；被链接的空页不写 `.md`；空而无子节点的页不写；冲突不改名；按名称而不是标题键比较冲突；`Store` 用于文本；`meta.json` 列出没写进的附件；附件的文件不在让导出失败；心跳不读取消；取消排队的任务不直接结束；成功不让旧的过期；收拾不看心跳的时刻；到期不删文件；清扫删掉活着的导出的文件；清理器先删行；签名不含任务 id；`e` 不核对；严格读法放过多出的参数；非本人又非管理员读得到；贡献者冲突放行；判定只在开始时做一次。
- `make check`、`make gen-check`、`make e2e`。

## 7. 完成标准

- 导出整个笔记本与子树，zip 的结构照总体设计 3.5 的映射逐项核对，`meta.json` 写明次序；被链接的空页写空的 `.md`；冲突改名进报告；导出的库在 Obsidian 里逐条解析，`obsidian-verified` 的样例与本系统一致。
- 任务在自己的队列、自己的超时里运行，与行同一个事务投递；取消、超时、停机、服务重启、笔记本删除都不让任务停在"运行中"，报告写明；成功的导出只留最新的一份，到期删除，孤儿文件被清扫。
- 下载经签名地址，严格核对；读、取消、下载都重新判定读权限，看不到的人看不到自己的任务。
- 设置里的任务列表与页面菜单的"导出此页"经路由到达；TR1 的两个版本通过。
- 总体设计 13.1 第 2、10、17、23、25 条修订；移交（M2/P4 只投递客户端第 1–4 项、M6 第 5 项）记下落实。

## 8. 交给 P6 与后面的 M

- **P6**：`Inserter` 与 `transfer_import` 队列（`jobs.import_workers`）；`transfer.import` 动作与"每本笔记本一个导入"的唯一索引；`imports/` 的清扫；报告的 `skipped` 与导入的问题码；任务的 `Actor{UserID, JobID}` 调用写入单元，客户端用任务记下的；`meta.json` 的读法照 3.9 的 `path`（只有目录的页以 `/` 结尾）。
- **M10**：注册导出贡献者（`index`、`log`），加整个程序上的行为测试。
- **M12**：几个 GB 的导出的负载（附件复制的速率、磁盘余量的预估）。

## 9. 结果

### 9.1 A：服务端

（实现之后填写。）

### 9.2 B：前端

（实现之后填写。）
