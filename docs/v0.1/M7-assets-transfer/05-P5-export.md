# M7/P5 导出：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M7/P5 导出 |
| 状态 | 进行中（A 合并 `e8f02d5`；B：前端） |
| 基线 | `ce0e39e`（P4 合并、文档补完之后的 main）；本文提交之后开分支 `m7-p5a`，A 合并之后开 `m7-p5b` |
| 上级文档 | [M7 总设计](00-M7-design.md) 4.1、4.8（导入与导出的界面）、4.9、4.10、4.12–4.14、第 5、7–9 节；移交：[M2/P4 只投递的 River 客户端](handoffs/M2-P4-insert-only-client.md)第 1–4 项、[M6 链接与附件、导入、导出](handoffs/M6-links.md)第 5 项；总体设计 3.3、3.5、8.5、13.1 第 2、10、17、23、25 条 |

---

## 0. 分两部分

P5 把一个笔记本或一棵子树导出为 zip。照 P3、P4 的先例分开实现、审查、核对修复、合并：

- **A：服务端**：`platform/jobs` 的队列、超时与只投递的客户端；`platform/postgres` 的 `TxFrom`、`WithinSnapshot`；`shared.Actor` 的 `JobID`；transfer 模块（`transfer_jobs`、开始导出、导出的任务、映射与 `meta.json`、导出贡献者、读与取消、签名的下载、到期、收拾、清扫、生命周期）；契约；导出的库在 Obsidian 里逐条解析的核对。e2e：TR1 的接口版本。
- **B：前端**：transfer 的服务；笔记本设置的"导入与导出"一节（任务列表、导出整个笔记本、取消、下载、报告）；页面标题旁的菜单"导出此页"。e2e：TR1 的页面版本。

第 1、2 节两部分共用，第 3 节是 A 照实际改写的设计，第 4 节是 B 的，第 5–9 节共用。

## 1. 基线

- **后台任务**：`platform/jobs` 只有服务的客户端（`jobs.New`）：默认队列 2 个 worker，River 默认的任务超时 1 分钟、`RescueStuckJobsAfter` 1 小时；`Job` 只有 `Add` 与 `Periodic`。模块的任务都是定时的：会话清理、编辑会话清理、软删除的清理、附件的孤儿清扫（`SweepTimeout` 50 分钟，留在 River 的 1 小时救援之内）。没有由请求投递的任务。
- **事务**：`TxManager.WithinTx`、`postgres.DB(ctx, pool)`、`InTx`；没有 `TxFrom`、没有只读的快照。
- **`shared.Actor`**：`UserID` 与两种凭据（`SessionID`、`APITokenID`）；page 的 `clientOf` 按凭据给出 `web`、`api`。
- **读端口**：page 有 `NewAssetNodes`、`NewLinkTargets`（`Subtree`、`Content`），没有按范围读整棵树与正文的；asset 的模块根有 `NewEmbeds`、生命周期与活动，没有给别的模块读文件的端口（总设计 4.1 的 `asset.NewBlobs` 还没出现：P2 的上传在模块之内）；linking 的 `page_links` 有 `resolved_id`，没有"哪些页是这些链接的目标"的端口。
- **组合根**：模块按 page → asset → linking 建；`purgers(pool, tx, store, logger)`；命令行的组合规则（`archtest/composition_test.go`）禁止它们建 `jobs.New` 与存储。
- **前端**：笔记本设置有"常规"与"成员"两节，看得到笔记本的人都进得来；页面标题旁只有写者的"编辑"。
- **工具**：`tools/md-fixtures/obsidian/verify-resolve.mjs` 在隔离的 Obsidian 里核对 `resolve/` 样例的解析。

## 2. 目标与范围

**做**（总设计第 7 节 P5 一行）：第 0 节的两部分；13.1 第 2 条（任务的客户端与 `Actor.JobID`）、第 5 条（加锁次序）、第 6 条（清理器）、第 10 条（日志）、第 11 条（模块入口）、第 15 条（配置）、第 17 条（签名地址的公开读）、第 21 条（注册者）、第 23 条（队列、超时、只投递的客户端、心跳）、第 25 条（导出下载的密钥），13.4 第 4、6 条与 6.4 随实现改写。M2/P4 只投递客户端的移交第 1–4 项、M6 移交第 5 项在这里落实。

**不做**：

- 导入（P6）：它的接口、队列 `transfer_import` 的 worker、`transfer.import` 动作、`imports/` 的清扫、`jobs.import_workers` 与 `transfer.import_*` 的配置随 P6。P5 的表、状态机与报告的形状照两种任务定下（`kind` 有 `import`），P6 不再改表。
- 删除任务的接口：成功的导出只留最新的一份，24 小时之后到期，不另给删除。
- 命令行的导出（总设计第 2 节"不做"；组合规则不改）。
- 导出贡献者的注册者（M10）：组合交空，模块根有示例的测试。

## 3. A：服务端

照实际改写（审查与修复核对之后）；与实施前的设计不同之处在第 9.1 节列出。

### 3.1 文件

| 位置 | 内容 |
|---|---|
| `platform/jobs/jobs.go` | `Config.Queues`（不许用 River 默认队列的名字）、`Config.RescueAfter`；`Job.Start` |
| `platform/jobs/inserter.go` | `Inserter`：只投递的客户端，`NewInserter(pool, logger)`、`InsertTx`、`Unfinished(kind)`（River 还没结束的这类任务的参数，分页读） |
| `platform/postgres/tx.go` | `TxFrom`；`TxManager.WithinSnapshot`；快照里的 `WithinTx` 答 `ErrTxInSnapshot` |
| `platform/postgres/pgtest/lockwait.go` | `WaitForTableLockWaits`：等在表锁本身上的连接（不算等行锁的） |
| `platform/storage/local.go` | 写入途中每 64 MiB 重读一次磁盘余量 |
| `platform/httpserver/disposition.go` | `Disposition`：`Content-Disposition` 的两种文件名（从 asset 的下载移来，两处共用） |
| `shared/actor.go`、`shared/tx.go` | `Actor.JobID`；`Snapshots` 端口 |
| `modules/page/export_nodes.go` | `page.NewExportNodes(pool)`：范围里的节点（含正文的字节数）、单页的名称、按批的正文 |
| `modules/asset/blobs.go` | `asset.NewBlobs(pool, store)`：附件节点的 blob 与写入时刻（`Of`）、`Open` |
| `modules/linking/export.go` | `linking.NewLinkedPages(pool)`：哪些页是范围里的链接的目标 |
| `modules/notebook/notebooks.go` | `NameOf`：笔记本名，不加锁、在调用方的事务里 |
| `modules/transfer/` | 模块根（`module.go`、`contributors.go`、`lifecycle.go`、`purgers.go`；`contributors_test.go`）；`domain`（状态、动作、错误、映射、`meta.json`、报告、压缩方法）；`app`（开始、任务、读、视图、取消、下载、到期、收拾、清扫、跟随、清理）；`adapter/postgres`（sqlc）、`adapter/river`、`adapter/http`（含生成的 `gen`）、`adapter/mac`、`adapter/archive`（写 zip，存储的打开与删除） |
| `migrations/sql/00029_transfer_transfer_jobs.sql` | 表、检查、索引；`deploy/runtime-grants.sql` |
| `api/modules/transfer.yaml` | 契约（3.6） |
| `bootstrap/transfer.go`、`deps.go`、`wire.go`、`display_names.go` | 组合：端口的适配（`transferDeps`）、任务的配置（`jobsConfig`）、注册者；显示名的适配由 page 与 transfer 共用（原 `page_names.go`） |
| `tools/md-fixtures/obsidian/verify-export.mjs` | 导出的库在 Obsidian 里逐条解析（3.15） |
| `web/apps/web/src/app/problem-messages.ts`、`i18n` | transfer 的三个错误码的中英文（`transfer.not_found`、`transfer.busy`、`transfer.not_cancellable`）；其余文案随 B |

### 3.2 平台：任务的队列、超时与只投递的客户端

- **队列**：`jobs.Config.Queues map[string]int`（队列名 → worker 数），与默认队列（2 个，定时任务）一起交给 River；名字是 River 的默认队列时 `New` 失败。组合根（`jobsConfig`）给 `transfer_export: jobs.export_workers`（默认 1）。导出不占定时任务的队列，几个小时的导出也不挡住清理。
- **超时**：导出的 worker 的 `Timeout()` 是 `transfer.job_timeout`（默认 6 小时）。`jobs.Config.RescueAfter` 是 River 的 `RescueStuckJobsAfter`，组合根给 `transfer.job_timeout + 1 小时`：`MaxAttempts` 为 1 的任务被救援就丢弃，救援要晚于任何 worker 的超时。River 只拿它与自己的默认超时比较，所以这条由组合根算出、由整个程序上的测试钉住（`TestTheExportsHaveTheirQueueAndOutlastRiversRescue`）。附件清扫的 `SweepTimeout` 的注释随之改。
- **启动时的收拾**：`jobs.Job.Start func(ctx) error`，在 `Runner.Start` 里、River 开始取任务之前依次运行；失败时 `Start` 失败（serve 退出，与 River 起不来相同）。`jobs.New` 按任务的次序收集它们。transfer 用它把上一个进程留下的"运行中"的任务记成失败（3.12）。
- **只投递的客户端**：`jobs.NewInserter(pool, logger)`：不配队列、不启动的 River 客户端。`InsertTx(ctx, tx, args, opts)` 与业务的行同一个事务投递：回滚时任务也不存在；River 在同一个事务里 `NOTIFY`，服务的客户端随即取到。`Unfinished(ctx, kind)` 列出 River 还没结束的这类任务（`available`、`pending`、`retryable`、`running`、`scheduled`，每次 1,000 个、按游标翻页）的参数，给收拾核对排队的行（3.12）。`platform/jobs` 仍只导入 River 与 pgx：事务由调用方经 `postgres.TxFrom` 取出交进来。
- **组合规则**：`archtest/composition_test.go` 的禁止集加上 `jobs.NewInserter` 与 `transfer.New`（命令行的组合不投递任务，M2/P4 移交第 3 项）；serve 到达它们。
- **停机**：HTTP 先于任务停（M1/P4 文档 3.4，不变）：正在处理的请求投递任务时任务一侧还在。River 在 `jobs.shutdown_timeout` 之后取消任务的上下文，导出照 3.8 记成失败 `interrupted`。

### 3.3 平台：快照与事务

- **`TxFrom(ctx) (pgx.Tx, bool)`**：`WithinTx`、`WithinSnapshot` 放进上下文的事务，给要把事务交给别的库的适配器（transfer 的 river 适配器投递任务）。
- **`TxManager.WithinSnapshot(ctx, fn)`**：`REPEATABLE READ READ ONLY` 的事务，`fn` 的上下文带着它，读端口经 `postgres.DB(ctx, pool)` 进入；提交与回滚照 `WithinTx`（不随调用方取消，限时 `database.commit_timeout`）。上下文已带事务时答 `ErrNestedSnapshot`（快照必须是最外层，否则读到的不是一个时刻）；快照里的 `WithinTx` 答 `ErrTxInSnapshot`（它的写会在只读事务里失败，只读的又会被当成自己的事务）。`shared` 加 `Snapshots` 端口，组合根照 `TxManager` 在编译时核对。

### 3.4 `shared.Actor` 的 `JobID`

- `Actor` 加第三种凭据 `JobID`：后台任务代账户执行时是任务的 id；三种恰好一个，写在注释里（没有 `Valid()`：P6 的写入单元第一个以它调用时再加核对）。
- 导出的任务以 `Actor{UserID: 发起人, JobID: 任务}` 判定权限（access 只看 `UserID`，判定不变）；日志带 `job_id`。P6 的写入单元同样以它调用，客户端用任务记下的（不按凭据推断：page 的 `clientOf` 不会见到任务的 `Actor`）。

### 3.5 `transfer_jobs`

```sql
CREATE TABLE transfer_jobs (
    id uuid PRIMARY KEY,                                   -- UUIDv7：River 任务的参数、zip 的文件名
    notebook_id uuid NOT NULL REFERENCES notebooks ON DELETE RESTRICT,
    root_id uuid,                                          -- 不建外键：指向节点会挡住节点的清理
    kind text NOT NULL CHECK (kind IN ('import', 'export')),
    state text NOT NULL CHECK (state IN ('queued', 'running', 'succeeded', 'failed', 'cancelled', 'expired')),
    name text NOT NULL CHECK (octet_length(name) BETWEEN 1 AND 255), -- 导出的根的名称：列表与下载的文件名
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

- **状态机的检查**：`0 ≤ progress_done ≤ progress_total`；`queued` 没有 `started_at`、`heartbeat_at`；`running` 两者都有；`cancel_requested_at` 只在开始之后；结束的四种有 `finished_at` 与 `report`，没结束的两种都没有；`expired` 只是导出的；`result_bytes` 恰好在导出的 `succeeded`、`expired` 时有，且不为负（到期之后仍留着，列表照样显示大小）。转移只经带 `WHERE state = …` 的语句：`queued → running | cancelled`、`running → succeeded | failed | cancelled`、`succeeded → expired`；收拾的 `running → failed` 与 `queued → failed`。
- **索引**：笔记本的列表 `(notebook_id, created_at DESC, id DESC) WHERE deleted_at IS NULL`；每人每本笔记本一个进行中的导出：唯一索引 `(notebook_id, created_by_id) WHERE kind = 'export' AND state IN ('queued', 'running') AND deleted_at IS NULL`（P6 加导入的那一个）；进行中的 `(state) WHERE state IN ('queued', 'running') AND deleted_at IS NULL`（数量、收拾）；到期 `(finished_at) WHERE kind = 'export' AND state = 'succeeded' AND deleted_at IS NULL`；清理 `(deleted_at) WHERE deleted_at IS NOT NULL`。
- 不存凭据的 id（会话清理会删 `auth_sessions` 的行）。写进 `runtime-grants.sql`。

### 3.6 契约

`api/modules/transfer.yaml`：

| 操作 | 说明 |
|---|---|
| `startExport`：`POST /api/v0/notebooks/{notebook_id}/exports` | 请求体 `{ "root_id": uuid \| null }`（必须有请求体，`root_id` 可省）；答 202 `TransferJob` |
| `listTransferJobs`：`GET /api/v0/notebooks/{notebook_id}/transfer-jobs?cursor=&limit=` | 新的在前，一页默认 50、至多 100；本人的，笔记本管理员是全部；每一项不带报告的问题 |
| `getTransferJob`：`GET /api/v0/transfer-jobs/{job_id}` | `TransferJobDetail`：`TransferJob` 加报告的问题 |
| `cancelTransferJob`：`POST /api/v0/transfer-jobs/{job_id}/cancel` | 答 200 `TransferJob` |
| `downloadExport`：`GET /api/v0/transfer-jobs/{job_id}/download?e=&s=` | `x-raw`、公开（`security: []`），签名就是凭据（3.11） |

- `TransferJob`：`id`、`notebook_id`、`root_id`（可为 null）、`name`、`kind`（`import`、`export`）、`state`、`client`、`created_by`（`user_id`、`display_name`，经 identity 的目录）、`created_at`、`started_at`、`finished_at`、`cancel_requested_at`、`progress`（`done`、`total`）、`result_bytes`、`report`（结束之后：`failure`、`counts`）、`download`（成功、未过 TTL 的导出：`url`、`expires_at`）。列表里的每一项都签下载地址（签名只是计算）。说明写明每人每本笔记本只留最新的一份成功导出。
- `TransferReport`：`failure`（失败时的原因码，否则 null）、`counts`（`pages`、`attachments`、`renamed`、`missing`、`skipped`，P5 不用 `skipped`，恒为 0；计数是任务写下结束时写进 zip 的，停机时自己写下结束的照样计数，来不及写、由收拾记成失败的计数为 0、进度留着）。`TransferJobDetail` 另有 `problems`（至多 1,000 条：`path`、`code`、`to`）与 `problems_truncated`。原因与问题都是码，前端按码给文案（总设计 4.8）。
  - 失败的码（P5）：`interrupted`（服务停止或重启、心跳停了、River 在开始之前丢了任务）、`timeout`、`forbidden`（运行时已不能读这个笔记本）、`root_not_found`（运行时导出的页已不在）、`storage_full`、`contributor_conflict`、`internal`。
  - 问题的码（P5）：`renamed`（`to` 是导出用的路径）、`file_missing`（附件的文件不在存储里）。
- 错误码：`transfer.not_found`（404：没有、已删、看不到、不是本人的而又不是笔记本管理员）、`transfer.busy`（409：本人在这本笔记本有排队或运行中的导出）、`transfer.not_cancellable`（409：已结束）；导出的页不是这本笔记本里活着的页答 `page.not_found`（404）；平台码 `server_busy`（503，排队的任务已满，带 `Retry-After`）、`storage_full`（507）。
- 下载的操作照 P2 的 `x-raw` 规则（代码生成排除，契约测试按描述核对状态、头与媒体类型）；描述写明 id 不是 uuid 时路由答 400、条件请求只认 `If-Modified-Since`、走 `anonymous` 桶。

### 3.7 开始导出

1. 不加锁的预检（只读笔记本所在的工作区；没有答 `notebook.not_found`）之后进事务：工作区行 `FOR SHARE` → 笔记本行 `FOR SHARE`（任一不在：`notebook.not_found`）→ 判定 `transfer.export`（读者；看不到答 `notebook.not_found`）→ 名称：有 `root_id` 时它是这本笔记本里活着的页（附件不行，`page.not_found`），否则笔记本名 → 事务级的咨询锁（一个常数键，只有任务的创建取它）之下数全部排队与运行中的任务，到 `transfer.max_queued` 答 503 → 本人在这本笔记本有进行中的导出答 409（唯一索引兜底，撞上同样答 409）→ 磁盘余量低于 `storage.min_free_bytes` 答 507 → 写行（`queued`，`client` 按凭据）并投递（队列 `transfer_export`，`MaxAttempts` 1，参数只有任务的 id）。
2. 答 202 与任务。日志：`job_id`、`notebook_id`、`user_id`、`client`。

### 3.8 导出的任务

worker 只调用用例 `Export.Run(ctx, jobID)`：

1. **开始**：`queued → running`（`started_at`、`heartbeat_at`）。没有这样的行（已取消、已删、已被收拾）就什么都不做、成功返回。
2. **心跳与取消**：另起一个 goroutine，每秒在池上（不在快照里）写一次 `heartbeat_at` 与进度，同一条语句读回 `cancel_requested_at` 与 `deleted_at`：有取消就以原因"取消"取消任务的上下文；行被删（笔记本删除）或已不在运行（被收拾记成失败）就以原因"已不在"取消。心跳一直持续到 zip 提交之后。写不进的下一秒再试，一连串失败只在第一次记警告、再写进时记一条；心跳停得久了由收拾记成失败。
3. **判定**：笔记本所在的工作区（笔记本已删：同"已不在"，什么都不写）；以 `Actor{UserID, JobID}` 再判定 `transfer.export`（看不到、不是读者：失败 `forbidden`）。
4. **写 zip**：`store.Create("exports/<job id>.zip")`（余量不足：`storage_full`），经 `adapter/archive` 写：
   1. **快照**（`WithinSnapshot`）：笔记本名与子树的根页（不在：`root_not_found`）→ 范围里活着的节点（`page.ExportNodes.Scope`：id、父节点、类型、名称、次序、正文的字节数、修改时刻；有 `root_id` 而没有节点：`root_not_found`）→ 没有正文、有子节点的页里哪些是范围里的链接的目标（`linking.LinkedPages`，只在有这样的页时读：来源是范围里的页，含属性链接；索引一页只记前 10,000 条，13.1 第 31 条）→ 映射（3.9；改名进报告）→ 附件的 blob（`asset.Blobs.Of`，没有附件时不读）→ 贡献者（3.10，它们的文件先写进 zip）→ 按映射的次序分批读正文（`page.ExportNodes.Contents`：200 页或 16 MiB 的正文一批，先到者为准，单页超过 16 MiB 自成一批），逐字节写 `.md`；只有目录的页写目录条目。每一页写进之后才计数、进度前进。之后提交。
   2. **附件**：快照之外，按映射的次序经 `asset.Blobs.Open` 读文件写进 zip，复制随任务的上下文停下；修改时刻是 blob 的创建时刻。文件不在的写进报告（`file_missing`），不让导出失败。不在 GB 级的复制期间持着快照与连接。
   3. **`.nerve/meta.json`** 最后写（只列写进 zip 的节点）。这时任务已被停下（取消、已不在、River 的上下文结束）就不提交、放弃文件；否则关闭 zip，`Commit`；提交失败时文件可能已在它的键上，删掉。写满（`storage.ErrFull`，包括写入途中余量降到下限之下）一律映射为 `storage_full`。
5. **结束**（`context.WithoutCancel`）：限时 30 秒；River 的上下文因停机被取消时限时 900 毫秒（在 River 停机的宽限之内）。超时不算停机，照样 30 秒。
   - 成功：一个事务里先以 `FOR SHARE` 锁笔记本行（与笔记本删除同一个次序：都先锁笔记本、再锁任务行，不成环），再 `running → succeeded`（`result_bytes`、`report`、`name` 改成快照里的根名），并把本人在这本笔记本之前成功的导出记成 `expired`；提交之后删掉它们的文件（删不掉的留给清扫）。笔记本已删、行已不在运行时，删掉自己的文件，记"停下"的日志。
   - 已不在（行已删、笔记本已删、被收拾）：`Abort` 文件，不写行，记"停下"的日志。
   - 取消：`Abort` 文件，`running → cancelled`，报告写明到哪里（进度与问题）。
   - 失败：`Abort` 文件，`running → failed`，原因：River 的期限 `timeout`，River 的上下文另被取消（停机）`interrupted`，判定 `forbidden`，`root_not_found`，存储写满 `storage_full`，贡献者冲突 `contributor_conflict`，其余 `internal`（错误本身记日志，不进报告）。写的时候行已不在运行（被删、被收拾）：记"停下"的日志。
   - 写结束状态失败时（数据库不可用）返回错误；行留在"运行中"，由收拾记成失败（3.12），已提交的文件由清扫删掉。
6. 日志：开始与结束各一条，`job_id`、`notebook_id`、`user_id`、`client`、状态、原因的码、节点数、字节数；文件名、zip 里的路径不进日志。

### 3.9 映射、报告与 `meta.json`

`domain.Plan`：纯函数，节点与"是否被链接"进，条目、改名与 `meta` 的节点出，逐项的表格测试。标题键由 domain 按标题的规则算出（节点不带）。

- **根目录**：zip 里有一层根目录，整个笔记本时是笔记本名（遵守标题的规则，总体设计 3.3），子树时是那一页的名称；库是它里面的内容。子树的那一页是库的根下的一页：`<页>/<页>.md`，它的子节点在 `<页>/<页>/`。
- **页面**：在目录 `D` 里的页 `E` 写 `D/E.md`，子节点在 `D/E/`。`E.md` 写出的条件：有正文（字节数大于 0）；或没有子节点（空的 `.md`）；或没有正文、有子节点而是范围里的链接的目标（空的 `.md`，M6 移交第 5 项）。只有子节点、没有正文、没被链接的页只有目录：写一个目录条目（`D/E/`），`meta.json` 里它的路径以 `/` 结尾。
- **附件**：在它的父页的目录里（根下的在库的根下），名称就是文件名。
- **兄弟的次序**：`sort_order`，再按 id（与树相同）；条目、`meta` 的节点都按深度优先的这个次序。
- **冲突与改名**：页面 `N.md` 有子节点时它的目录 `N.md/` 与兄弟 `N` 的文件 `N.md` 同名（按标题键比较：文件系统可能不分大小写）。有目录的那一页改名：取 `N.md 2`、`N.md 3`……里第一个标题键不与这个目录里任何名称、文件、目录相同的，文件与目录一起改。页面的文件名（名称加 `.md`）超过 255 字节（多数文件系统一段名称的上限）时同样改名，名称在字符边界截短到放得下后缀。报告记 `renamed`（`path` 是原来的路径，父目录也照原来的名称；`to` 是导出用的路径），指向它的链接在 Obsidian 里解析不到（报告的说明写明）。附件的名称不以 `.md` 结尾、与页面共用命名空间（P2），所以不会有别的冲突；映射对任意输入仍核对"一个目录里没有同键的条目、没有文件与目录同键"，不成立时导出失败 `internal`（不变式）。
- **zip 的条目**：名称是 `根/路径`，`/` 分隔；Go 的 `archive/zip` 对非 ASCII 的名称设 UTF-8 标志、超过 4 GiB 用 zip64。修改时刻：页面是节点与正文的更新时刻里较晚的，附件是 blob 的创建时刻，贡献者的文件与 `meta.json` 是导出的时刻。已经压缩过的类型（png、jpeg、gif、webp、avif；音视频；zip、gzip、7z、rar；docx、xlsx、pptx、epub 等 zip 容器）以 `Store` 写入，其余 `Deflate`。
- **`.nerve/meta.json`**（在根目录里）：

  ```json
  { "format": 1, "exported_at": "2026-10-09T12:00:00Z",
    "notebook": { "id": "…", "name": "…" }, "root": null,
    "nodes": [ { "path": "项目A.md", "kind": "page", "id": "…", "sort_order": 1.5 },
               { "path": "项目A/需求/", "kind": "page", "id": "…", "sort_order": 3 } ],
    "contributed": [] }
  ```

  `path` 相对库的根：页面是它的 `.md`，只有目录的页是目录（以 `/` 结尾），附件是它的文件；`root` 在子树时是 `{ "id", "name" }`。P6 据它恢复兄弟的次序，`id` 只作参考。
- **报告**：计数（页、附件、改名、缺文件：写进 zip 的）与前 1,000 条问题（路径至多 1,024 字节，在字符边界截断，`problems_truncated` 记下截断）。

### 3.10 导出贡献者

- `transfer.ExportContributor`：`Contribute(ctx, scope ExportScope, sink ExportSink) error`，在快照里运行（读端口经上下文进入它）。`ExportScope`：笔记本 id、根（可为 null）、范围里每个节点的 id、是否附件与在库里的路径。`ExportSink.Add(path, content []byte) error`：路径相对库的根、`/` 分隔、每一段遵守标题的规则、不在 `.nerve/` 之下；与节点的路径、已加的文件按标题键相同，或文件与目录相撞时返回错误，导出失败 `contributor_conflict`。加的文件写进 zip（在页面之前），路径记进 `meta.json` 的 `contributed`。
- 按登记的次序调用，第一个错误即停（导出失败：冲突是 `contributor_conflict`，其余 `internal`）。
- M7 组合交空（`transfer.Deps.Contributors`）。模块根的 `contributors_test.go` 在真的数据库与 River 上经 `transfer.New` 登记贡献者：次序、加的文件进 zip 与 `contributed`、第一个错误即停（冲突与其余）、`JobTimeout` 到达 worker。整个程序上的行为测试在 M10 注册时加（12.4 的通用规则：组合根交空时那条测试失败）。

### 3.11 读、取消与下载

- **读**：`getTransferJob` 先读行（没有、已删：404 `transfer.not_found`），再判定 `transfer.read`（读者；看不到这本笔记本：同样 404），不是本人的而又不是笔记本管理员：404。列表照样判定，非管理员只列本人的；列表不读报告的问题。读与取消都重新判定读权限（报告里有页的名称与路径）。
- **视图**：成功的导出带 `download`，地址签到整点之后一小时的末尾与 `finished_at + transfer.export_ttl`（取整到秒）里较早的那一刻，`expires_at` 就是它；地址撑不过这一秒的（过了 TTL 而到期还没轮到）显示为 `expired`、不带地址。
- **取消**：判定 `transfer.cancel`（读者），只许发起人与笔记本管理员（否则 404）；锁任务行：`queued → cancelled`（直接结束，River 之后取到它时什么都不做）；`running` 记下 `cancel_requested_at`（重复取消不改时刻；任务在下一次心跳停下，除非它先结束）；已结束答 409 `transfer.not_cancellable`。只写任务行，不碰笔记本行，与任务的读不成环。日志：`job_id`、`notebook_id`、`user_id`、取消时的状态。
- **签名**：`HMAC-SHA256(key, "export-download" ‖ job id 的 16 字节 ‖ e 的 8 字节大端)` 截成 16 字节、无填充的 base64url；`e = min((⌊now / 1 小时⌋ + 2) × 1 小时, ⌊导出的到期⌋)`（1–2 小时有效、不晚于导出的到期，同一小时签出的相同；签名签的就是地址里的 `e`）。密钥 `SigningKeys.Derive(transfer.DownloadKeyInfo)`，info `nervewiki export-download mac v1`，已知答案的测试钉住（13.1 第 25 条），整个程序上核对组合根交的是派生的密钥。只有核对过读权限的读（列表、单个）签。
- **下载**：`GET /api/v0/transfer-jobs/{job_id}/download?e=…&s=…`，照 P2 的严格读法（路径的 id 是规范写法，转义过的答 404；参数依次是 `e`、`s`，各恰好一次，不反转义；`e` 规范的十进制；`s` 22 个 base64url 字符；别的参数答 404），在任何查询之前核对签名；签名对、未到期、任务是活着的、成功的、`finished_at` 在 TTL 之内、文件在，才下发，否则一律 404 `not_found`。签名就是凭据：下载不再判定读权限，只有判定过的读签出地址。文件不在时再读一次行，导出仍活着才记警告（到期刚删掉的不记）。经 `API.Stream`（无请求体，写出按 `asset.upload_min_rate` 放宽截止时间），公开的操作走平台的 `anonymous` 桶；停机开始之后答 503。头：`Content-Type: application/zip`、`Content-Disposition: attachment; filename="<ASCII 兜底>.zip"; filename*=UTF-8''<名称>.zip`、`/api/` 默认的 `no-store`、每个答复（含拒绝）都有 `Content-Security-Policy: sandbox; default-src 'none'`；`http.ServeContent` 支持 `Range`（断点续传），去掉 `If-Match` 与 `If-Unmodified-Since`（一个地址下发的内容不变），其余条件照 `http.ServeContent`（没有 ETag：304 看 `If-Modified-Since`（没有 `If-None-Match` 时）或 `If-None-Match: *`，`If-Range` 按时刻）。

### 3.12 到期、收拾、清扫与生命周期

- **到期**（定时，每 15 分钟，启动时也跑一次，限时 10 分钟）：成功的导出 `finished_at` 早于 `transfer.export_ttl`（默认 24 小时）的，每批 100 个 `succeeded → expired`（`SKIP LOCKED`），提交之后删文件；删不掉的留给清扫。
- **收拾**：
  - serve 启动时（`jobs.Job.Start`，River 取任务之前）把全部活着的"运行中"记成失败 `interrupted`（别的事务持着的行跳过，留给定时的收拾）。
  - 之后每 5 分钟：心跳早于 `transfer.heartbeat_timeout`（默认 5 分钟）的"运行中"同样记成失败；再核对排队的导出：先读排队的导出的行、再读 River 还没结束的导出（`Inserter.Unfinished`），行在而 River 已不持有的（唯一的一次尝试在开始之前就用掉了：开始的写入失败）记成失败 `interrupted`。先读行再读 River：那时排队的行与它的任务同一个事务投递，River 不是还持有、就是已经开始（行不再排队）或已丢掉；写失败的语句只改仍在排队的行（`SKIP LOCKED`）。进程恰在 River 取到任务、写开始之前停掉的，River 把任务记在运行中，直到它的救援（`job_timeout + 1 小时`）丢掉它，行才被记成失败；其间本人在这本笔记本开始不了新的导出，可以取消这个排队的任务。
  - 报告只写原因，计数为 0，进度留着（到哪里）。任务不再运行。日志按任务各一条警告。
- **清扫**（定时，每天，启动时也跑一次，限时 50 分钟）：`exports/` 下一天以前的文件，没有活着的成功导出对应的，删掉（结束状态写失败时已提交的文件、到期与"只留最新"删不掉的、笔记本删除之后的），每 500 个查一次行；删不掉的记警告，跑完之后整体失败。P6 加 `imports/`。
- **存储的余量**：本地存储的写入每 64 MiB 重读一次余量，低于 `storage.min_free_bytes` 答写满（导出失败 `storage_full`）；读不到余量时跳过这一次。默认大小之内的附件上传不受影响。
- **生命周期**：笔记本删除的订阅者软删除这些笔记本的任务（三条路径都经笔记本删除事件：删笔记本、删工作区、删无主的笔记本）：排队的被取到时什么都不做，运行中的下一次心跳看到行已删、停下，不写结束（行保留删除时的状态）；下载与读随之 404。清理器 `transfer_jobs` 排在 notebooks 之前（`notebook_id` 是 `RESTRICT`）：一批一个事务、`SKIP LOCKED`，先删文件、再删行，文件删不掉时这一批失败、行留着。结束的任务在笔记本活着时不清理（M12 移交）。

### 3.13 配置

| 键 | 默认 | 约束 |
|---|---|---|
| `transfer.export_ttl` | 24 h | 至少 10 分钟 |
| `transfer.job_timeout` | 6 h | 1 分钟到 7 天（实际至少多于 `heartbeat_timeout`） |
| `transfer.heartbeat_timeout` | 5 min | 至少 1 分钟（心跳每秒一次）、短于 `job_timeout` |
| `transfer.max_queued` | 20 | 至少 1 |
| `jobs.export_workers` | 1 | 1–8，且至多 `database.max_conns` 的一半（每个导出在快照里占一个连接） |

River 的救援（`job_timeout + 1 小时`）由组合根算出，不是配置。各项进 `LogValue`；`config.yaml` 的注释写明（含 `anonymous` 桶也管导出的签名下载）。`config.test.yaml` 不改（e2e 的导出几秒就完成）。`database.max_conns` 为 1 的配置因此启动失败（变更记录写明）。

### 3.14 组合根、权限、交错与日志

- **组合根**：transfer 在 asset 之后建：`transfer.New(Deps{Pool, Tx, Snapshots, Store, Inserter, Clock, Logger, Authorizer, Workspaces, Notebooks, Names, Nodes: page.NewExportNodes(pool), Linked: linking.NewLinkedPages(pool), Blobs: asset.NewBlobs(pool, store), Contributors: nil, DownloadKey, ExportTTL, JobTimeout, HeartbeatTimeout, MaxQueued, MinFreeBytes, MinRate})`（`bootstrap/transfer.go` 的 `transferDeps`）；它的 `Jobs()`（导出的 worker、收拾含 `Start`、到期、清扫）进 `jobs.New`，`Queues` 与 `RescueAfter` 随之；笔记本删除的注册者加 transfer；`purgers` 在 notebooks 之前加 transfer 的。命令行的组合（`notebookRegistrants`、`purgers` 只凭连接池与存储）照旧到不了 `transfer.New` 与 `Inserter`。整个程序上核对显示名、下载的密钥、`storage.min_free_bytes`（507）都接到了。
- **权限**：规则表加 `transfer.export`、`transfer.read`、`transfer.cancel`（都是读者）；`transfer.Actions()` 进动作表的核对；权限矩阵加四个操作的每一格（下载是公开的、凭签名，单独测）。
- **交错**（`bootstrap/interleavings_transfer_test.go`，13.4 第 4 条）：
  - 任务的创建与笔记本的删除，两种先后：先删，开始答 404；先建，任务排队时（测试锁住另一个排队任务的行，占住唯一的导出 worker）随笔记本软删除，worker 取到它时什么都不做。
  - 取消与运行中的任务：任务停在快照里（测试持着 `asset_blobs` 的表锁），取消之后结束为 `cancelled`、文件不留。附件复制途中的取消只在 app 的测试里。
  - 快照与同时的保存：任务停在快照里，另一个连接保存一页并提交，zip 里是保存之前的正文。
  - 导出的成功与笔记本的删除：测试的事务照删除的次序先锁笔记本行、再锁本人更早的那次导出的行，放开 `asset_blobs`，等任务的成功事务等在笔记本行上，再写其余的任务行、提交：任务随笔记本删除，文件被删掉，没有死锁。
  - 结束时核对不变式：River 的导出都已完成；没有活着而停在排队或运行中的任务；`exports/` 里只有成功的导出（含随笔记本删掉、还没清理的）的文件。
- **日志**（13.1 第 10 条）：只记 `job_id`、`notebook_id`、`user_id`、`client`、数量与原因的码；文件名、zip 里的路径、签名不进日志（测试钉住开始、取消、结束的日志里没有名称）。

### 3.15 Obsidian 的核对

- `bootstrap/transfer_obsidian_test.go`：对 `tools/md-fixtures/resolve/` 的每个样例建一本笔记本（样例的页、附件，图片是真的 PNG；第 i 条链接写在与它出发的页同一个父页下的页 `q<iii>` 里，三位数字，同 `verify-resolve.mjs` 的做法），经 serve 导出，核对 zip 的结构与映射（每一页、每个附件、只有目录的页、被链接的空页）。环境变量 `NWIKI_EXPORT_VAULTS=<目录>` 设了时另写出每个样例的 zip 与"每个 `q<iii>.md` 在本系统里解析到的路径"（`go test -count=1`，要 Docker）。
- `tools/md-fixtures/obsidian/verify-export.mjs prepare <workdir> <vaults>`：解开每个 zip 为一个库（去掉唯一的根目录，根目录旁有别的文件就报错；打开"检测所有类型的文件"），隔离的用户数据目录；`check`：在 Obsidian 里读每个 `q<iii>.md` 的解析，与本系统的比较：`obsidian-verified` 的样例必须一致，`nerve-defined` 的只报告差异。结果见第 9.1 节。只启动隔离的实例，用完按它自己的 PID 停掉。

### 3.16 e2e：TR1 的接口版本

- `e2e/fixtures/transfer.ts`（PAT 的导出、轮询到结束、下载）、`e2e/fixtures/zip.ts`（从中央目录读 zip，用 node 的 `zlib`，不加依赖），数据库断言 `e2e/fixtures/assert/transfer.ts`（13.4 第 3 条：任务行的状态、`client`、发起人、进度、字节数、报告的计数；存储目录里的文件与字节；到期之后轮询文件消失）。
- `stories/transfer/tr1-export.spec.ts` 的接口版本：一本笔记本（有正文的页、非 ASCII 名称的页、只有子页的页、被链接的空页、根下与页下的附件、`N` 与 `N.md` 的冲突），导出整个笔记本与一棵子树：zip 的条目（次序、字节、压缩方法）、`meta.json`（节点的路径、类型、id，`sort_order` 与数据库一致，`root`）、改名进报告、计数；第二次导出成功之后第一次的记成已过期、文件删掉、地址答 404；签名地址改一个字符答 404。

## 4. B：前端

A 合并之后细化（下面），实现、审查与修复核对之后照实际改写（第 9.2 节）。

### 4.1 文件

| 位置 | 内容 |
|---|---|
| `server/internal/modules/instance`、`api/modules/instance.yaml` | `InstanceInfo.export_ttl_seconds`（`transfer.export_ttl`）：确认对话框说成功的导出保留多久（13.1 第 15 条："服务端可配的量经接口告诉前端"） |
| `services/transfer.service.ts` | `TransferService`：`startExport(notebookId, rootId)`、`list(notebookId, cursor)`、`get(id)`、`cancel(id)`，经会话的客户端 |
| `stores/transfer.store.ts` | `TransferStore`：一本笔记本的任务，新的在前，一页 50 个，"加载更多"；每代一个（13.2 第 15 条），`RootStore.transfersOf`、`useTransfers` |
| `pages/notebook/transfer-page.tsx`、`transfer-job-row.tsx`、`transfer-report.tsx` | 设置的"导入与导出"：导出、任务列表、一行、报告 |
| `pages/notebook/export-dialog.tsx` | 导出的确认对话框：整个笔记本与"导出此页"共用 |
| `pages/page/page-menu.tsx` | 页面标题旁的菜单："导出此页" |
| `pages/notebook/settings-layout.tsx`、`app/routes.tsx` | 第三节 `transfer` |
| `i18n/messages/en.ts`、`zh-CN.ts` | 状态、原因与问题的码、按钮与说明 |
| `e2e/stories/transfer/tr1-export.spec.ts`、`e2e/fixtures/wiki-transfer.ts` | TR1 的页面版本 |

### 4.2 服务与 store

- **服务**：四个操作对应契约的四个；下载是 `<a href={download.url} download>`，不经客户端（地址是签名的、公开的）。
- **`TransferStore`**：照 `AuditStore`（第一页、"加载更多"的后几页，后读到的第一页赢；重读第一页时保留已经加载的后几页）。另有：
  - 第一页里有的任务替换已持有的同一任务（状态、进度、报告在第一页里变）；
  - `active`：持有的任务里有排队或运行中的；
  - `start(rootId)`：开始导出，答出的任务放到最前面；
  - `cancel(id)`：答出的任务替换持有的那一个；
  - 报告不进 store：展开时以 SWR 键 `["transfer-job", id]` 读 `get(id)`。
- **轮询**：设置页的 SWR 键 `["transfer-jobs", 笔记本 id]`，`refreshInterval` 是一个函数：
  - 有排队或运行中的任务时 1 秒；
  - 否则在最早的下载地址到期之前一分钟（至少 30 秒），没有地址时 0（不轮询）。
  - 隐藏的标签页不轮询（SWR 的默认），显示时照常重读（`revalidateOnFocus`）。
  - 地址一两个小时就到期：页面开着不动时，列表里的"下载"不会指向已到期的地址。

### 4.3 "导入与导出"

- **入口**：笔记本设置的第三节 `settings/transfer`，看得到笔记本的人都进得来（读者可以导出）。
- **导出**一节：一句说明与"导出整个笔记本"按钮。按钮打开导出的对话框（4.5）；成功之后对话框关掉，新任务在列表最前面，焦点到它那一行。
- **任务**一节，标题"最近的任务"，说明"你的导入与导出；笔记本管理员看到所有人的"：
  - 列表是 `<ol>`，没有任务时说"还没有任务"，读不到经 `NotLoaded`（13.2 第 7 条）。
  - "加载更多"照附件一节：读完最后一页时焦点到它加进来的第一项，读者其间动过就不移（13.2 第 26 条）。
- **一行**：
  - **名称**：整个笔记本的导出写"导出整个笔记本"，子树写"导出页面「名称」"（`name` 是开始时的名称，成功之后是快照里的）。
  - **状态**：排队中、运行中、已成功、已失败、已取消、已过期。排队与运行中带 `<progress>`：总数为 0 时不定；`aria-label` 是"进度：n / 总数"；有 `cancel_requested_at` 时写"正在取消"。
  - **发起人**：不是本人的写"由 {名字} 发起"。
  - **时刻**：开始的时刻（`formatDateTime`）；结束的写结束的时刻。
  - **大小**：成功、已过期的导出写 `result_bytes`（`formatBytes`）。
  - **成功的导出**：写保留到哪一刻（`finished_at + export_ttl_seconds`）。
  - **动作**：
    - 有 `download` 的有"下载"（链接，`download` 属性）。
    - 排队、运行中的有"取消"（只有发起人与管理员看得到这些行，不另判断）。取消被拒（`transfer.not_cancellable`：已结束）时在行里说原因，并重读列表。
    - 结束的有"报告"（`aria-expanded`、`aria-controls`），展开读详情（4.4）。
- **到达时的焦点**：从"导出此页"来的（路由的 `state.focusJob`），任务出现在列表里时焦点到它那一行，一次。

### 4.4 报告

- **计数**：页、附件、改名、缺文件。
- **失败的原因**：按码给文案，读屏读出：
  - `interrupted`：服务停止或重启，任务中断；
  - `timeout`：超过了实例的时限；
  - `forbidden`：运行时已不能读这本笔记本；
  - `root_not_found`：导出的页已不在；
  - `storage_full`：服务器的存储空间不足；
  - `contributor_conflict`：服务器加的文件与页面同名；
  - `internal`：服务器出错。
  - 不认识的码写通用的"任务失败"。
- **问题**：路径与码的文案。
  - `renamed`：改名为 `to`，指向它的链接在 Obsidian 里解析不到。
  - `file_missing`：附件的文件不在存储里，没有写进 zip。
  - 截断了的说只列出前 1,000 条。
- 读不到详情时经 `NotLoaded`。

### 4.5 导出的对话框与"导出此页"

- **对话框**（`ConfirmDialog` 的写法）：
  - 说明：zip 里是一个以笔记本（或这一页）命名的文件夹，就是一个 Obsidian 的库；成功的导出保留 `export_ttl_seconds`（按小时或分钟写），每人每本笔记本只留最新的一份。
  - 确认之后发送，按钮写"正在开始"。
  - 被拒时在对话框里说原因，对话框不关：
    - `transfer.busy`（已有进行中的导出）；
    - `server_busy`（排队的任务已满，稍后再试）；
    - `storage_full`；
    - `notebook.not_found`、`page.not_found`（这一页已不在）。
- **"导出此页"**：
  - 页面标题旁、"编辑"之前加一个菜单按钮（`⋯`，`aria-label`"页面操作"），阅读时显示，读者也有（树的操作菜单只给写者）；页面已从树上消失时不显示。
  - 一项"导出此页"打开同一个对话框（`root_id` 是这一页）。
  - 成功之后导航到设置的"导入与导出"，`state.focusJob` 是新任务。

### 4.6 测试

- **vitest**：
  - 服务（请求的路径、参数与请求体）。
  - store：第一页的合并、`active`、开始放在最前、取消替换、加载更多。
  - 设置页：
    - 轮询只在进行中（假的定时器：进行中每秒读，结束之后不再读，地址到期之前一分钟再读）；
    - 状态与进度的文案；取消与被拒；下载的地址；
    - 报告按码；加载更多与焦点；到达时的焦点。
  - 对话框：保留时长的文案；409、503、507 与已不在的文案；成功之后的焦点。
  - 页面菜单：读者有；编辑时没有；开始之后导航到设置并带着任务。
  - 经路由到达：文档标题与导航。
- **服务端**：实例信息的处理器与契约测试带 `export_ttl_seconds`；整个程序上核对它接的是配置的 `transfer.export_ttl`。
- **e2e**：TR1 的页面版本（`tr1-export.spec.ts` 里第二个测试）：
  - 设置里导出整个笔记本，等到这一行"已成功"，点"下载"（`waitForEvent("download")`），读 zip 核对条目与 `meta.json`；
  - 页面菜单"导出此页"，到设置、焦点在新的一行，等到成功；
  - 同一组数据库断言（`client` 是 `web`）。

## 5. 实施步骤

| 步 | 内容 | 提交 |
|---|---|---|
| A1 | 平台：`Queues`、`RescueAfter`、`Job.Start`、`Inserter`；`TxFrom`、`WithinSnapshot`；`Actor.JobID`；组合规则 | `jobs, postgres, shared: queues, an insert-only client, snapshots and the job's actor (M7/P5A)` |
| A2 | 迁移与授权；transfer 的 domain（状态、映射、`meta.json`、报告）、postgres 适配器 | `transfer: the jobs' rows and the export's mapping (M7/P5A)` |
| A3 | 读端口（page、asset、linking）；开始导出、导出的任务、`adapter/archive`、贡献者 | `transfer: exports run as background jobs (M7/P5A)` |
| A4、A5 | 契约与 HTTP：开始、读、列表、取消；签名的下载；到期、收拾、清扫、生命周期、清理器；配置；组合根；整个程序、交错、权限矩阵、运行时角色（实际合成一个提交） | `transfer, bootstrap: the exports' API, wired into serve (M7/P5A)` |
| A6 | Obsidian 的核对；e2e TR1 的接口版本；总体设计的修订 | `e2e, tools: exports end to end and in Obsidian (M7/P5A)` |
| B1 | 实例信息的 `export_ttl_seconds`：模块、契约、生成、测试 | `instance: the exports' TTL in the instance's info (M7/P5B)` |
| B2 | 服务、store、设置的"导入与导出"、报告、导出的对话框、页面菜单、i18n、vitest | `web: exports from the notebook's settings and a page's menu (M7/P5B)` |
| B3 | e2e TR1 的页面版本 | `e2e: exports in the browser (M7/P5B)` |

## 6. 测试与验证

- **平台**：`Queues` 交给 River（真库上投递进自己的队列；`RescueAfter` 晚于导出的超时由组合根算出、整个程序上的测试钉住）；`Unfinished` 分页、只列 River 还没结束的；`Job.Start` 依次运行、在 River 之前、失败时 `Start` 失败；`Inserter` 在事务里投递、回滚时没有任务（真的数据库）；`WithinSnapshot` 是只读的（写答错误）、看不到之后提交的写、嵌套时报错；`TxFrom`。
- **transfer 的 domain**（表格）：映射的每一条（有正文、空而无子节点、只有子节点、被链接的只有子节点、根下的附件、页下的附件、子树的根、深的树、次序、`N` 与 `N.md` 的冲突、改名之后再撞、标题键的大小写与 `ß`）；`meta.json` 的金样；报告的截断（1,000 条、1,024 字节、字符边界）；压缩方法的表。状态的转移在 SQL 里，由库上的测试与表的检查守住。
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
- 任务在自己的队列、自己的超时里运行，与行同一个事务投递；取消、超时、停机、服务重启、River 丢掉任务都不让活着的任务停在排队或运行中，报告写明，笔记本删除的任务随之停下；成功的导出只留最新的一份，到期删除，孤儿文件被清扫。
- 下载经签名地址，严格核对，地址不晚于导出的到期；读与取消重新判定读权限，只有判定过的读签出地址；看不到笔记本的人看不到自己的任务。
- 设置里的任务列表与页面菜单的"导出此页"经路由到达；TR1 的两个版本通过。
- 总体设计 13.1 第 2、10、17、23、25 条修订；移交（M2/P4 只投递客户端第 1–4 项、M6 第 5 项）记下落实。

## 8. 交给 P6 与后面的 M

- **P6**：`Actor.Valid()`；收拾读排队的导入（`QueuedExports` 只读导出）；`Inserter` 与 `transfer_import` 队列（`jobs.import_workers`）；`transfer.import` 动作与"每本笔记本一个导入"的唯一索引；`imports/` 的清扫；报告的 `skipped` 与导入的问题码；任务的 `Actor{UserID, JobID}` 调用写入单元，客户端用任务记下的；`meta.json` 的读法照 3.9 的 `path`（只有目录的页以 `/` 结尾）。
- **M10**：注册导出贡献者（`index`、`log`），加整个程序上的行为测试。
- **M12**：[导入导出的移交](../M12-release/handoffs/M7-transfer.md)：结束的任务行的保留期；几个 GB 的导出的负载（附件复制的速率、正文分批的内存、磁盘余量）；部署文档。

## 9. 结果

### 9.1 A：服务端（2026-10-09，合并 `e8f02d5`）

- 提交：
  - 实施：平台 `cfaafaf`、任务表与映射 `9d89c77`、导出的任务 `89a5cf4`、接口与组合 `55ba107`（A4、A5 合在一起）、e2e 与 Obsidian `2fd761d`；负对照的补测 `edf9466`。
  - 审查的修复 `571cc57`、`159a4c9`；修复核对的修复 `9c0fb7c`、`da6831a`、`c5d7e71`。合并 `e8f02d5`。
- 审查：三位审查者（Opus）。没有高。中 6 条：River 丢掉的排队任务永远停在排队；正文只按页数分批；列表读整个报告；写入期间不看磁盘余量；成功与笔记本删除的死锁；测试与文档的几处。中低与低三十余条，都已处理，接受与推后的见审查记录。
- 修复核对两轮：
  - 第一轮高 1 条：修复引入的回归，截到导出到期的地址没有重签，每份导出最后一两个小时的下载地址全部 404。中 1 条：River 的分页翻到第三页报错。另有契约、文档与测试的缺口。
  - 第二轮没有行为问题，补了几处会挂住或测不出的测试，改了几处措辞。
  - 逐条见[审查记录](reviews/P5A-export-review.md)。
- 与实施前的设计不同之处（第 3 节已照实际改写）：
  1. 只有目录的页写一个目录条目（`meta.json` 里以 `/` 结尾）。
  2. 列表一页默认 50、至多 100。
  3. e2e 读 zip 用 node 的 `zlib`，不加 `fflate`。
  4. `jobs.export_workers` 至多 `database.max_conns` 的一半，`max_conns: 1` 因此启动失败；`job_timeout` 的下限 1 分钟实际到不了（要长于 `heartbeat_timeout`）。
  5. 结束的限时：停机 900 毫秒，其余 30 秒（原写 1 秒）。
  6. 收拾：报告的计数为 0、进度留着（原写计数取进度）；River 在开始之前丢掉的排队导出也收拾（原写排队的照常由 River 运行）。
  7. zip 里的修改时刻：页面取节点与正文的更新时刻里较晚的；附件取 blob 的创建时刻。
  8. 文件名超过 255 字节的页同样改名。
  9. 快照里的次序：名称 → 范围 → 链接目标 → 映射 → blob → 贡献者 → 正文；贡献者的文件在页面之前；没有附件时不读 blob。
  10. 正文按 200 页或 16 MiB 一批（原只按页数）；本地存储的写入每 64 MiB 看一次余量。
  11. 成功的事务先锁笔记本行（原没写，与删除会死锁）；快照里的 `WithinTx` 被拒；提交之前看上下文。
  12. 下载与视图按 TTL；地址的期限不晚于导出的到期；答复带 `cancel_requested_at`。
  13. `NewInserter(pool, logger)`，另有 `Unfinished`；`Queues` 不许用默认队列的名字。
  14. 没有 `Actor.Valid()`（推到 P6）。
  15. 贡献者的示例测试在模块根、真库与 River 上；收拾在同一处另有一条。
  16. 文件：没有 `adapter/files`；`LinkedPages` 在 `linking/export.go`；加了 `platform/httpserver/disposition.go`、`pgtest.WaitForTableLockWaits`、`display_names.go`；transfer 三个错误码的文案随 A。
  17. 表的检查多了：名称 1–255 字节、`cancel_requested_at` 只在开始之后、`result_bytes` 不为负、`progress_done` 不为负；两个部分索引带 `deleted_at IS NULL`。
  18. 心跳读到行已不在运行（被收拾）同样停下、不写结束；运行时笔记本已删同样。
  19. 到期与清扫启动时也跑；到期限时 10 分钟、清扫 50 分钟。
  20. 交错：取消停在快照里（`asset_blobs` 的表锁），附件复制途中的取消只在 app 的测试里；另加成功与笔记本删除。
  21. 不变式只看活着的行；随笔记本删除、还没清理的成功导出的文件也算。
  22. 下载不再判定读权限：签名就是凭据，只有判定过的读签出地址（原第 7 节写"下载重新判定"）。
  23. `RescueAfter` 与超时的关系由组合根算出、整个程序上的测试钉住；River 只拿它与自己的默认超时比较（原第 6 节写 `New` 失败）。
  24. 队列在真库上测（投递进 `transfer_export`）；没有 domain 的状态转移表：转移在 SQL 里，库上的测试与表的检查守住。
  25. 节点不带标题键（domain 算），带正文的字节数（取代"是否为空"）。
  26. 开始导出的预检只读笔记本所在的工作区。
  27. Obsidian 核对的查询页是三位数字（`q000`）。
  28. TR1 的 `meta.json` 核对路径、类型、id 与次序（与数据库比较，一页移到两页之间）。
  29. 总体设计修订了 13.1 第 2、5、6、10、11、15、17、21、23、25 条，13.4 第 4、6 条与 6.4。第 2 节原写第 17 条是为 `JobID` 而改，实为签名地址的公开读。
- Obsidian 的核对（3.15）：Obsidian 1.12.7，27 个样例、133 条链接，`obsidian-verified` 的全部一致。`nerve-defined` 的差异 7 条：
  - 012 别名 3 条：Obsidian 不按别名解析。
  - 015 Obsidian 不同的写法 3 条：重名取最短路径、后缀匹配、`./`。
  - 026 1 条：`[[y.png]]` 本系统解析到别名为它的页。
  - 009 兄弟按 id 决胜的样例 Obsidian 一致，可以考虑改判为 `obsidian-verified`。
- 反向对照：
  - 实施之后 38 个：3 个起初没被抓到或要等测试的总超时，补测之后都被抓到（`edf9466`）。
  - 审查的修复之后 45 个；两轮修复核对之后 16、4 个。
  - 全部被抓到。
- CI：分支的 `2fd761d`、`159a4c9`、`da6831a`、`c5d7e71` 上 server、web、image、e2e 全部通过。本机每轮跑全套 Go 测试、golangci-lint、前端检查、`gen-check` 与 e2e TR1。
- 交给 B、P6 与后面的 M：
  1. B：第 4 节。读接口的答复已带 `cancel_requested_at`；地址撑不过这一秒的导出显示为 `expired`。
  2. P6：`Actor.Valid()`，写入单元第一个以 `JobID` 调用时加；收拾读排队的导入另加（`QueuedExports` 只读导出）。其余见第 8 节。
  3. M10：导出贡献者的注册与整个程序上的行为测试（第 8 节）。
  4. M12：[导入导出的移交](../M12-release/handoffs/M7-transfer.md)，包括结束的任务行的保留期、几个 GB 的导出的压测、部署文档。
- 负责人可以改判的取舍：
  - `database.max_conns: 1` 启动失败；
  - 收拾记成失败的任务计数为 0；
  - 进程恰在 River 取到任务时停掉，排队的任务要等 River 的救援才记成失败（默认 7 小时，其间可以取消）；
  - 导出下载走 `anonymous` 桶、`no-store`；
  - 每人每本笔记本只留最新的一份。

### 9.2 B：前端

（实现之后填写。）
