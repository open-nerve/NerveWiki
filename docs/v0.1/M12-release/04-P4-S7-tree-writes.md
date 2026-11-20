# M12/P4/S7 树写的期限（B4）：设计 & 实施规划

| 项 | 内容 |
|---|---|
| Step | M12/P4/S7 树写的期限（B4；P4 的条件修复，P4 设计 3.9；一个 Step，分支 `m12-p4-s7`） |
| 日期 | 2026-11-21（初稿；同日第一、第二次修订；设计已审查） |
| 状态 | 设计已审查（2026-11-21）：三份审查（A：1I 4M 8N 1Q；B：1I 4M 6N 2Q；C：1I 13M 7N 1Q）、两次修订、两轮核对，见[设计的审查记录](reviews/P4-S7-design-review.md)；决定 652–667 在 [00 号文档](00-M12-design.md) 第 11 节。逐条的对照在设计工作目录的 `m12/p4s7/revision-1-notes.md`；初稿存为 `p4s7/draft/draft-v1.md`，第一次修订稿存为 `draft-v2.md` |
| 上级文档 | [M12 总设计](00-M12-design.md) 4.7.1 的 B4 一行、4.7.5、第 7 节"次序与并行"；[P4 设计](04-P4-performance.md) 3.6 的 B4、3.9、3.5.4（回收站与撤销族的比 R = 2.046）；负责人 2026-11-21 第 13 问第 1 题（开修复 Step，7763 上 ≤ 15 秒，"设计若要大改先回来问"）与第 2 题（`MaxJobItems` 降到 45,000，P4/S6）；P4/S3 批次 F 的结果（scratchpad 的 `p4s3/results/F.md`，含修复第 1 轮的同口径）与 S3 审查的 A-Q1、A-M4、C-M3；[v0.1 总体设计](../v0.1-design.md) 13.4 第 9、10、13 条；M10/P4/S2 把 `LinksReached` 改成四条互斥的 `UNION ALL`（M10 决定 143，`6d1549fc`） |
| 代码位置的写法 | 行号是 `main` 的 `f1317bc3` 的（S3 的合并 `a8b51831` 没有改 `server/internal` 与迁移，行号照样对得上）。`L/` 是 `server/internal/modules/linking/`，`P/` 是 `server/internal/modules/page/`，`B/` 是 `server/internal/bootstrap/`。`links.sql` 指 `L/adapter/postgres/queries/links.sql` |
| 工作笔记 | 调查在 scratchpad 的 `m12/p4s7/investigation.md`（run1、run2：服务端的剖面、`pg_stat_statements`、`auto_explain`）。设计的三次探针 run3、run4、run5 在 `psql` 上直接比几种写法，都经 `heavy.sh`：脚本 `p4s7/scripts/run3.sh`、`run4.sh`、`run5.sh`、`exp3*.sql`、`exp4*.sql`、`exp5a.sql`，读数在 `~/.cache/m12-p4s7/run3/`、`run4/`、`run5/`，归属过的读数在 `~/.cache/m12-p4s7/parsed-all.txt`；CI 上现有守卫的读数与各份在哪种 CPU 上取自 `gh` 的作业日志（第一次修订：`~/.cache/m12-p4s7/ci/jobs-all.tsv`、`cpu-all.tsv`，`main` 上 36 次成功的运行，审查 C 的 11 次加更早的 25 次；4.5）（调查留下的 vault 的库 `run1/vault.dump` 恢复进一个自己的 PostgreSQL 18.7 容器，`autovacuum=off`，用完删）。第一次修订另跑一次 run6（2026-11-21 05:52:50–05:53:43 持锁，`df` 37G，负载 0.47–1.35）：审查 C 的守卫探针（`p4s7/review-c-work/zz_review_c_probe_test.go`，`ZZ_N=10000 ZZ_SCATTER=1`：链接打散、最上层是没人链着的页）在 `f1317bc3` 的分离 worktree 里打上审查 C 的 `probe-links.diff`（调查的宽读原型，只有 A、没有 B），今天的读与原型各两轮（读数 `~/.cache/m12-p4s7/run6/r1.txt`、`r2.txt`，脚本 `scripts/run6.sh`；worktree 与临时目录用完删，testcontainers 的容器由 Ryuk 收走），数在 4.4。三位审查者的探针在 `~/.cache/m12-p4s7-review-a/`、`p4s7/review-c-work/`。调查的临时 worktree 已删 |

## 0. Step

一个 Step，`m12-p4-s7`，改链接索引的一条读、链接的参与者与观察者在一个单元里的分工，加测试、一个组合根的守卫与 `perf ops` 的一个开关，加文档。量的搭法要的 `perf vault -part` 归 P4/S6（主持者的决定，审查 C 的 M7）。

**原则**：用户碰到的只有用时。答复、链接索引的每一行、链接与页的事件、错误码都不变；不改接口、网页、事务与锁（主持者 2026-11-21：A 加 B 不是负责人说的"大改"，不再问负责人）。

- **A：大的到达换一种读法**（3.1）。一个单元的到达（`domain.Reach`）的键与目标合起来至少 4,000 个时，`Store.Links` 改读宽读：按键、按目标一次按页取（无统计时只取本笔记本的位图；有统计时由规划器按代价定，见 3.1），再滤掉出发页在来源里的行；行与今天的 `LinksReached` 逐行相同。宽读有两条语句，按 `plan_cache_mode` 选：平常（按参数规划）读 `LinksReachedWide`，成员与排除都是常量数组的 `= ANY`，PostgreSQL 哈希它们、不看大小与 `work_mem`；设了 `force_generic_plan` 时读 `LinksReachedWideGeneric`，成员与"出发页不在来源里"都写成对 `unnest` 的子查询（排除是 `NOT EXISTS`），通用计划里都是哈希的子计划（1.3 第 3、4 条）。三条读各在一个同形的转发函数里，archtest 的规则不放宽；`Store.Links` 过门槛时经 sqlc 的 `PlanCacheMode` 读设置、选语句。少于门槛的照旧读 `LinksReached`（每个键一次索引查找）。今天 6 万个节点的移动、改名，键那一支做 50,099 次索引查找、取出 135 万行再一行行滤掉，一次 3.5 秒，一个单元读两次（1.2）。
- **B：一个单元只读一次、只解析一次**（3.2）。单个改名或移动的单元里，链接的参与者（改写）与观察者（索引）各自读同一个到达、各自解析同一批链接。条件写死（3.2）：树单元、不是 batch（一个显式的布尔）、是重定位、观察者看到的变化与参与者跟着的逐个相同；都满足时观察者用参与者读到的链接与解析，不再读；有一条不满足就照今天读。
- **测试与守卫**（第 4 节）：今天的读与两条宽读逐行相同（随机参数，长期留着，13.4 第 13 条）；宽读在三种统计、三种计划方式下的计划与物理工作量；门槛两侧、两种计划方式下选对语句、宽读经 `Planned`；B 的每一条件不满足都回到今天的读法；组合根一个约 1 万个节点的最上层的移动、改名、删除的守卫，以按语句名数调用的结构断言为主、计时只兜期限（4.4）。
- **在原规模上再量**（6.3）：照 P4 设计 3.6 的 B4，6 万个节点的最上层改由两次导入搭成（决定 658，`perf vault -part` 由 P4/S6 加），改前、改后用同一对 zip，交错着各量两轮，`fsync=off` 两边在同一段里。

**读者看得到的**：只有用时。改名、移动、删除一个约 6 万个节点的最上层，推到 7763 上在 15 秒的期限之内（推断，1.4）。

**开工的时机与名额**：

- 次序写死 S6 → S7（审查 C 的 N6）：S6 把导入上限降到 45,000，并加 `perf vault -part`（主持者的决定）；量要 S6 合并之后；改前取 S7 分支的合并基点（S6 合并之后的 `main`），在那上面重量（决定 659、6.3 第 3 条）。
- 代码的提交只有 `tools/perf/main.go` 与 S6 同文件、不同函数（`opsCommand` 对 `vault` 一段，5.2），可以先写；守卫的规模（约 1 万个节点）与 S6 无关。
- 占"同时在实施的 Step 不多于两个"的一个名额（00 号文档第 7 节），由主持者排。

**工作量**（推断）：

- `links.sql` 两条语句约 80 行、读设置一条；两个转发的文件 `links_reached_wide.go`、`links_reached_wide_generic.go` 各约 15 行；`Store.Links` 约 15 行；archtest 的规则加两个同形的路径、用例加两组；B 在 `L/app` 约 120 行、`P/app` 与 `B/` 各约 25 行。
- 测试约 1,100 行：逐行相同的随机对照约 200，宽读的计划与物理工作量约 350，`Planned` 与选语句约 80，B 的单元测试约 300（含 `P/app` 一格），组合根的守卫约 200（含按语句名数调用的 tracer）。
- `perf ops -quiet notebook` 约 30 行与测试（6.3 第 5 条）。
- 一个实施者约两到三天。
- 量的机器时间约 7.5 小时（改前、改后各两轮加 `fsync=off` 的一轮，每轮约 1 小时；编辑者负载下改前、改后各一次约 25 分钟；20 万个节点外推多一个新库约 15 分钟；6.3 第 7 条的八个守卫改前改后各两轮），两夜。

**不在本 Step**：

- 删除随子树的条目与版本数变慢的那一部分（调查的 E 与 3.3，本文 1.2）：已知限制（决定 663，第 9 节）。
- 调查 F 节的两个小项（节点长度的缓存、路径读的排序溢出）：不做（决定 663）。
- 语义上收窄到达（调查的 C）、改成后台任务（调查的 D）：2.4 甲、乙。
- `LinksReachedExcept`（LLM Wiki 导入带准备好的来源时的读）：不改（决定 655）。
- 病态形状（20 万个节点的最上层、400 页 × 2,000 条链接的 `move`、被几千页链着的页的改名）：仍是已知限制，照改后的数重新换算（6.3 第 6 条）；后两种 S7 不改它们的读与写（到达在门槛之下，A 不碰；参与者改写页，B 不用）。

## 1. 基线

### 1.1 P4/S3 的结论（批次 F，`p4s3/results/F.md`）

`main` 的 `f1317bc3`，本机，vault `-seed 1201`（45,000 页、5,000 个附件、`fanout 10,10,98`、每页 30 条链接，导入成根下一页 `vault` 之下约 59,911 个节点的最上层）。每轮每种操作三次的中位；删除的主行 = `fsync=off` × R + (本轮中位 − `fsync=off`)，移动与改名 = 本轮中位 × R，R = 2.046（推断）。

| 操作 | 本机三次（秒，各轮） | `fsync=off` | 主行（推断） | × 1.25 | 结论 |
|---|---|---|---|---|---|
| 删除 `vault` | r1 7.42 / 8.62 / 9.14；r2 7.69 / 8.53 / 9.22；r3 8.04 / 8.48 / 9.49；r4 7.68 / 8.60 / 9.06 | 7.90（新库上的第 1 次） | 同口径 17.46–18.14 | 21.8–22.8 | 超 |
| 移动 `vault` | r1 14.96，第二次本机 500（15.006 秒）；r2 14.41 / 14.19 / 14.68；r3 14.72 / 14.80 / 14.72 | 14.54 | 29.5–30.6 | 37.6 | 超（本机就 500） |
| 改 `vault` 的名 | r2 14.22 / 13.60 / 13.61；r3 14.32 / 14.33 / 14.07 | 14.10 | 27.9–29.3 | 36.6 | 超 |
| 改被链得最多的页（被 59 页链着） | 0.188–0.229 | 0.225 | 0.43、0.47 | 0.58 | 过 |
| 删第一层文件夹 `f1`（6,871 个节点，只记） | 1.44–1.68 | 1.49 | 3.01、3.06 | 3.82 | 只记 |

- 每轮的三次删除依次变慢：每次删除与恢复给这一项的条目与版本加约 6 万行的历史（审查 A-I1；调查 3.3 量到了，本文 1.2）。所以本文的删除写明每一次是新库上的第几次（决定 657）。
- S3 没量的两样交给 S7（S3 审查 A-M4、C-M3）：编辑者负载下的四种操作各一次只记；病态形状换算到 7763（F.md 已用 M5 Max 的数换算过一次，S7 用改后的数再换一次，6.3 第 6 条）。

### 1.2 原因（调查，`investigation.md`）

调查的 vault 同一个种子，附件小、导入之后 `VACUUM ANALYZE`，同样的操作比 S3 快约 15%（没核实原因，6.3 因此改前也重量）。

**移动、改名**（本机 10.7–12.7 秒）：

- 一个单元读两次 `LinksReached`：参与者（`Rewrite.Participate` → `reached`，`L/app/rewrite.go:290`）与观察者（`Index.PagesChanged`，`L/app/index.go:42`）各一次，一共 7.1–8.7 秒，是数据库时间的 81–85%、请求的 62–70%。
- 每一次里，键那一支（`links.sql:48-65`，每个键一次 `CROSS JOIN LATERAL` 的索引查找）占 3.51 / 4.27 秒：50,099 次查找、取出 135 万行，全是子树里的页写的链接，再被 `(l.source_id = ANY(a.sources)) IS NOT TRUE` 一行行滤掉；来源那一支 0.44 秒，目标那一支 0.24 秒。
- 一个单元也解析两次（`resolutions`，`L/app/resolver.go:56`）：Go 一边两次合计约 2 秒（v0 改名的剖面里 `resolutions` 共 1.88 秒 CPU；按两边累计，参与者一侧 1.45、观察者一侧 1.38 秒，审查 B 的 N2），路径的两条读（`LinkTargetsByKeys`、`ByIDs`）每一对约 0.75 秒、排序溢出到磁盘。
- 读子树约 0.11 秒；Go 的 CPU 一共约 3.1 秒（参与者 1.4–1.6，观察者约 1.46）。

**删除**（新库上的第 1 次 7.12 秒）：

- 观察者删掉这些页的行（`DeleteLinksOf` 0.90 秒）之后读 `LinksReached`：一行也没有，仍 3.38 秒（键那一支照样 50,099 次查找）。删除也叫参与者（`P/app/unit_delete.go:53`），但它不重定位任何节点，参与者在 `followed` 开头就返回、不读（`rewrite.go:254-257`），所以删除只读一次，B 对删除没有用。
- 其余：`DeleteNodes` 1.32、`RecordItems` 0.67、外键的核对 0.15、`QueueSearch` 0.13 秒，Go 约 0.5–0.8 秒。

**删除随历史变慢**（调查 3.3；`DeleteNodes` 一条，同一个库上依次删除、恢复）：

| 第几次删除 | `DeleteNodes`（秒） | 共享缓冲的命中 | WAL（MB） |
|---|---|---|---|
| 1 | 1.32 | 348 万 | 187 |
| 2 | 1.92（开着 `auto_explain`） | 587 万 | 278 |
| 3 | 3.61 | 794 万 | 410 |
| 4 | 3.44 | 963 万 | 500 |
| 5 | 4.15 | 1,217 万 | 547 |
| 6 | 4.92 | 1,453 万 | 664 |

`DeleteNodes` 给子树里每个节点所有活着的条目与版本打 `deleted_at`，恢复再清掉；每一轮给这一项加约 2 × 59,911 行，更新不是 HOT（部分索引 `changeset_items_deleted_at_idx`）。这一部分随这一项的历史长，不随 S7 的改动变（第 9 节）。

**已经试过的**（调查 run2）：在服务端里把门槛之上的读换成一条宽读（本文 1.3 的 v2），移动 5.48–5.61 秒、改名 5.49–5.57 秒（对 10.69–11.48、10.95），删除（第 6 次）7.87 秒；两次 `LinksReached` 一共 1.87 秒。`serve.log` 没有 ERROR、WARN。

### 1.3 设计的探针：几种写法（run3、run4、run5）

调查的库（59,911 个节点、135 万条链接的一本笔记本）恢复进自己的 PostgreSQL 18.7 容器（`autovacuum=off`，默认的 `shared_buffers` 128 MB、`work_mem` 4 MB、`hash_mem_multiplier` 2），`psql` 上 `PREPARE` 几种写法、`EXPLAIN (ANALYZE, BUFFERS, TIMING OFF)` 执行，ms。到达照真的算：一页、一个 7 个节点的文件夹、一个第二层文件夹（687 个节点）、`f1` 的前 3 与前 6 个第二层文件夹（2,061、4,122 个节点）、`f1`（6,871）、`vault`（59,911）；另有"加垫"的 `vault`：键与 id 各加 20 万个不中的，看大数组。键是这些节点的名字的键（去重），目标、来源是它们的 id；括号里是键与目标合起来的个数。"无统计"是删掉 `page_links` 的统计、`reltuples` 置 −1（同 M10 的 `no_statistics`）。

写法：

- **v0**：今天的 `LinksReached`（`links.sql:30-95`）。
- **v2**：两支。来源那一支同今天；键、别名键、目标合成一支 `(target_key = ANY(keys) OR target_alt_key = ANY(keys) OR resolved_id = ANY(targets))`，排除写成 `(source_id = ANY(sources)) IS NOT TRUE`。调查 run2 在服务端里量过的就是它。
- **v3**：v2 的排除换成 `NOT EXISTS (SELECT 1 FROM unnest(a.sources) AS s(id) WHERE s.id = l.source_id)`（计划里是子计划，PostgreSQL 估得下时哈希）。
- **v4h**：键、别名键、目标的成员也写成 `IN (SELECT … FROM unnest(…))`，排除同 v3：四个子计划。
- **v4j**：键、别名键、目标各是对 `unnest` 的连接，每支各自对 `unnest(sources)` 反连接，三支 `UNION`。

**逐行相同**：v3 与 v0 在五种到达上（90、292、16,193、160,006、1,350,000 行）、v4h 与 v0 在 687、2,061、6,871、59,911 个节点上、v4j 与 v0 在这几种与 4,122 上，两个方向的 `EXCEPT ALL` 都是 0；v2 与 v0 在调查 run2 里（160,006、1,350,000 行）同样。

**按参数规划**（生产走的：`Store.Links` 经 `postgres.Planned` 每次描述再执行，`auto` 下不具名的语句每次都按参数规划；数组是常量，PostgreSQL 把 `= ANY(常量数组)` 哈希，不论多大、不看 `work_mem`）：

| 到达 | v0 有统计 | v2 有统计 | v3 有统计 | v4h 有统计 | v4j 有统计 | v0 无统计 | v2 无统计 | v3 无统计 |
|---|---|---|---|---|---|---|---|---|
| 一页（2） | 0.1–0.3 | | 0.1 | | | 0.2 | | 189 |
| 7 个节点（14） | 0.2–0.7 | | 0.1–0.2 | | | 1.7 | | 248 |
| 687（1,373） | 7.3–17.6 | 21.9 | 11.1–20.7 | 529–562 | 18.6–20.1 | 195–212 | 236 | 307–314 |
| 2,061（3,923） | 45.8–56.8 | 48.8–52.9 | 39.5–45.5 | 537–547 | 51.7–55.9 | 254–271 | 226 | 308–320 |
| 4,122（7,748） | 88.3–114.3 | 54.8–60.9 | 129.6–135.8 | 540–565 | 122–129 | 349–387 | 238 | 330–333 |
| 6,871（12,849） | 111.8–180 | 138–143 | 97.2–110.6 | 561–587 | 189–201 | 511–589 | 244 | 364–380 |
| 59,911（110,010） | 3,022–3,066 | 511–537 | 681–708 | 809–821 | 5,724–5,798 | 3,143–3,350 | 937 | 876–908 |
| 删除的到达（同一事务里先删掉这些页的行） | 2,664–2,670 | （没量） | 386–401 | 381–397 | 4,832–4,898 | | | |
| 加垫的 `vault`（约 51 万） | 报错 ※ | 报错 ※ | 过 45 秒被取消 | 过 45 秒被取消 | 6,180 | | 报错 ※ | 过 45 秒被取消 |

※ `could not resize shared memory segment … No space left on device`：并行的计划要把几十万个元素的常量数组放进共享内存，探针的容器是 Docker 默认的 64 MB `/dev/shm`。今天的 v0 一样（第 9 节）。

**通用计划**（`plan_cache_mode = force_generic_plan`：`Planned` 的注释写明这种模式也作用于不具名的语句，语句要在它之下也能用，`server/internal/platform/postgres/planned.go:10-20`）：

| 到达 | v0 有统计 | v0 无统计 | v3 有统计 | v3 无统计 | v4h 有统计 | v4h 无统计 | v4j 无统计 |
|---|---|---|---|---|---|---|---|
| 一页 | 0.2 | | 0.1–0.4 | 174 | | | |
| 7 个节点 | 0.6 | | 0.1–0.6 | 256 | | | |
| 687 | 39–40 | 62 | 5.3–8.6 | **7,908** | 433 | 240 | **20,069** |
| 2,061 | 269 | | | | | | |
| 4,122 | **964** | | 45 | | 456 | 286 | **过 45 秒** |
| 6,871 | **2,578** | **2,645** | 69–74 | **过 60 秒** | 480 | 341 | **过 45 秒** |
| 59,911 | （没量） | | 558–573 | **过 60 秒** | 815 | 895–898 | **过 45 秒** |
| 删除的到达 | | | 359–377 | | 401 | | |
| 加垫的 `vault` | | | 622 | | 905 | 958–975 | |

两本同样大的笔记本（另一本是这一本的拷贝、换了来源与目标，表 270 万行），v0 / v3：有统计按参数规划 687 个节点 30 / 17，6,871 个 245 / 187，59,911 个 3,265 / 601；无统计 254 / 316、634 / 396、3,662 / 905，扫描限在本笔记本（`notebook_id` 打头的索引的位图）；v3 无统计的通用计划同样 687 个 8,200、更大的过 60 秒。

审查 A 的探针（`~/.cache/m12-p4s7-review-a/`，同样两本各 135 万行）补量了 v2 与 v4h：无统计时两者都只读本笔记本（v2 按参数规划 223–954 ms，v4h 通用计划 268–916 ms）；有统计时 v2（687 个节点到 `vault`）与 v4h 的通用计划也只读本笔记本，走位图或 `BitmapOr`；但 v2 在 `work_mem = 64kB` 下，4,122、6,871 个节点的两支都成了整表的 `Parallel Seq Scan`，三个进程合计约 270 万行（两本都读了），193、209 ms。只有一本笔记本时（run5），有统计的 v2 在 `f1`、`vault` 上选了整表的 `Parallel Seq Scan`，v4h 的通用计划选了整个主键的索引扫描、`notebook_id = $1` 只作 `Filter`。所以"只读本笔记本"在无统计时成立；有统计时由规划器按代价定，本笔记本占表的大半或 `work_mem` 小时会读整张表，用时仍是线性的（3.1、第 9 节）。

读到的：

1. **宽读快在把"一键一查"换成按页取**：v2、v3 的键那一支是三个位图索引扫描的 `BitmapOr`（有统计时）或本笔记本的位图（无统计时），堆的每一页读一次（24,770 页），排除用哈希；v0 是 50,099 次索引查找、135 万次随机的堆读（共享缓冲 136 万次）。
2. **小的到达宽读不划算**：有统计时 687 个节点 v0 快 1.5–3 倍；无统计时宽读对任何到达都先读整本笔记本的位图（一页也要约 180 ms）。按参数规划时 v2 对 v0 的切点：有统计在 3,923（2,061 个节点）处持平，7,748 处 v2 快 1.6 倍；无统计在 1,373 处 v0 稍快，3,923 起 v2 快。切点不单调：有统计时 12,849（`f1`）处 v2 138–143 ms，v0 热的 111.8–128.5 ms（首次的 177–180 是冷的），v2 慢约 10–25%，它在这里选了整表的 `Parallel Seq Scan`（审查 A 的 N1）；约 20 ms 对删 `f1` 的 1.4–1.7 秒，在 2.1 的 × 1.2 之内。通用计划下的切点在 v0 的 3,923 个 269 ms、7,748 个 964 ms 与 v4h 的 1,373 个 433 ms、7,748 个 456 ms 之间（审查 A 的 N6）。所以门槛取 4,000（决定 653）。
3. **按参数规划时 v2 最稳**：常量数组的 `= ANY` 由 PostgreSQL 哈希，不看大小与 `work_mem`。v3、v4h 的子计划只在估出的大小放得进 `work_mem × hash_mem_multiplier`（默认 8 MB；`uuid` 约 21 万个、`text` 约 15 万个，`work_mem` 1 MB 时四分之一）时哈希，放不下就逐行把数组从头比一遍：加垫的 `vault` 两者都过 45 秒（计划里的子计划没有 `hashed`）。B4 的 6 万个在界线之内，但它随运维的 `work_mem` 变。v4h 还因为键不能当索引条件，有统计时也读整本笔记本（687 个节点 529 ms）。v4j 的键是逐个的索引查找，同 v0 一样慢（59,911 个 5.7 秒）。
4. **通用计划里只有 v4h 处处线性**：参数数组的 `= ANY` 不能哈希。v2 的排除、v0 的排除都是逐行比参数数组（v0 有统计时 4,122 个节点 964 ms、6,871 个 2.6 秒，按参数规划 0.1 秒）；v3 的排除是哈希的，但无统计时规划器只用 `notebook_id` 取位图、把 `target_key = ANY($2) OR …` 留作过滤，687 个节点 7.9 秒、更大的过 60 秒；v4j 的连接在无统计时选了嵌套循环，687 个 20 秒。v4h 的四个子计划在通用计划里都哈希（估出的 `unnest(参数)` 是 10 行；哈希表在执行时照实建，不再看 `work_mem`），有统计、无统计、加垫都在 1 秒之内。
5. **今天的 v0 在通用计划下对中等以上的到达已经是平方的**（第 4 条）：M10 的计划测试的到达很小（至多 400 个来源，`links_reached_dense_plan_test.go`），看不到。它的花费是"键、别名键、目标三支取出的行数 × 来源的个数"次比较（每一支都把 `(source_id = ANY($4)) IS NOT TRUE` 留作逐行的 `Filter`，审查 A 的 `b_analyzed.out`），取出的行数随有多少链接指向到达的键，不随到达的大小：vault 的密度（每页约 30 条链接）下 3,923 个时 269 ms，被链得多的子树在门槛之下也可以远过它（例如 2,000 页、每页被链 1,000 次，推断），没有界（审查 A 的 M4）。只在 `force_generic_plan` 下出现；门槛之下留着，记进第 9 节（决定 664）。
6. **几十万个元素的大数组碰到共享内存**：加垫的 `vault`（约 51 万个元素）按参数规划时 v0、v2 选了并行的计划，在 64 MB 的 `/dev/shm` 上报错。这是 20 万个节点以上的病态形状，今天就这样，记进第 9 节。

（探针的 `psql` 输出里错误走 stderr、计划走 stdout，两者的先后会错开一行；读数按"每条语句的结果依次对上它的标签"重新归属过，`~/.cache/m12-p4s7/parse4.py`。）

### 1.4 改后的预计（推断）

| 操作 | 本机今天（S3 的口径） | 只有 A | A 加 B | 推到 7763（主行） | × 1.25 |
|---|---|---|---|---|---|
| 移动、改名 `vault` | 13.6–14.8 | 约 6.3（调查 run2 的 5.5 乘 1.15） | 约 3.2–3.5 | 约 7 秒 | 约 9 秒 |
| 删除 `vault`，`fsync=off` 的第 2 次 | 约 8.6 | 约 5.3–5.7 | 同左（B 不管删除） | 约 11–12 秒 | 约 14–15 秒 |
| 改被链得最多的页 | 0.19–0.23 | 不变（门槛之下） | 不变（参与者改写 59 页，B 不用，照今天读两次） | 约 0.4 秒 | |

- 只有 A：移动、改名省掉两次读里的约 2 × 2.4 秒，主行约 13 秒、× 1.25 一行约 16 秒，贴着期限；所以 A 加 B（主持者已定）。
- A 加 B 再省观察者的一份（审查 B 的 N2，照调查的原型、run2 的改名 `b-rename-vault-1.pprof` 与 `.pgss.tsv`）：Go 1.47 秒 CPU（其中 `Index.resolve` 0.98–0.99），宽读 1,866 ms 的一半约 0.93 秒，路径的一对 (684 + 650) / 2 约 0.67 秒，子树约 0.11 秒，合计约 2.7 秒，乘 1.15 约 3.1 秒；6.3 − 3.1 约 3.2 秒，表里写 3.2–3.5（偏保守）。
- B 要参与者什么也没改写：vault 的链接只写页名、不带路径，整个最上层移动或改名不让任何链接换去处，不改写页（推断；调查的剖面里没有改写的写）。改写了页的大子树（例如有页用库内的绝对路径链进来）B 不用，只有 A 的余量：主行约 13 秒、× 1.25 约 16 秒（决定 666，第 9 节）。
- 删除：删除的到达 v3、v4h 381–401 ms（1.3；`LinksReachedWide` 即 v2 的没在 `psql` 上量），对 v0 的 2.7–3.4 秒；调查 run2 在服务端里换成 v2 之后，同一个库上第 6 次删除 7.87 秒，第 3–5 次是 8.46、9.53、10.50（v0，每次多约 1 秒），推断省约 3.5 秒。第 2 次删除的历史部分（1.2 的表）不变。× 1.25 一行可能贴着 15 秒，照 3.9 第 3 条只列给主持者。
- 这些是推算，6.3 照实量。

## 2. 目标与范围与取舍

### 2.1 目标

- **期限**：约 6 万个节点的最上层（决定 658 的搭法）的删除、移动、改名，主行推到 7763 上 ≤ 15 秒（负责人第 13 问第 1 题）。判法照 P4 设计 3.6 的 B4（决定 657）：
  - 每轮每种操作三次的中位；两轮取较大；
  - 移动、改名：本机 × R（2.046）；
  - 删除：`fsync=off` 的库上新库的第 1、2、3 次删除的中位 × R，加（本轮中位 − `fsync=off` 的中位），差不乘；每一次删除写明是新库上的第几次（S3 的决定、审查 A-Q1）；
  - 本机就有一次答 500、503 的，超；
  - × 1.25 那一行过 15 秒而主行没过的，列给主持者（3.9 第 3 条），不自动判超。
- **不退步**：改被链得最多的页（S3：被 59 页链着），改后两轮的较大中位不高于改前两轮的较大中位 × 1.2（S3 的三次相差约 ±10%）；删第一层文件夹同样（只记的那一行，也不许变慢）。违反了照 P4 设计 3.9 第 2 条复核两轮（同样交错，6.3 第 3 条），仍过就停下列给主持者，写出两边的读数（审查 C 的 N7）。
- **结果不变**：链接索引的每一行、链接事件、页事件与答复都与今天相同（4.1、4.3 守着）。
- **测试与守卫**：第 4 节；各份在 7763 上加的秒数写进第 10 节（决定 660）。

### 2.2 范围

改：

- `links.sql`：加 `LinksReachedWide`、`LinksReachedWideGeneric` 与 `PlanCacheMode`（3.1）；`make generate` 生成的 `gen/links.sql.go`。
- `L/adapter/postgres/links_reached_wide.go`、`links_reached_wide_generic.go`（新）：各一个与 `linksReached` 同形的转发函数 `linksReachedWide`、`linksReachedWideGeneric`，共用 `linksReachedBuffer`（3.1）。`links_reached.go` 一字不改。
- `L/adapter/postgres/store.go`：`Links` 按门槛与 `PlanCacheMode` 选三者之一，常量 `linksReachedWideFrom`（3.1）。
- `server/internal/archtest/rawsql_test.go`：`rawSQLCollectorCall` 的 `switch` 加两个新文件的路径；结果类型那一行（`:219`，`if name == "linksReached" || name == "linksReachedExcept"`）加两个新名字，不加的话规则对两个新文件答 nil、报 `calls Query`（审查 A 第 2 轮的 N3）；`rawSQLLinksReachedQuery` 改成按语句常量名的匹配器（`LinksReached`、`LinksReachedWide`、`LinksReachedWideGeneric` 各配各的文件）；`rawsql_links_reached_cases_test.go` 照现有的形状加两组（审查 A 的 I1、C 的 M13；规则本身不放宽，3.1）。
- `L/app/rewrite.go`、`L/app/index.go`、`L/app/ports.go`（或新文件 `L/app/unit_reach.go`）：B（3.2）；`Moved` 加 `Batch`、`Memo`，`PagesChanged` 加 `Memo`，接口 `UnitMemo`。
- `P/app/extension.go`、`P/app/unit.go`：`UnitMemo` 与 `Write.Memo`（3.2）。
- `B/linking.go`：两个适配器把备忘与 `Batch` 交给链接。
- `L/module.go`：`NewIndex`（`:163-172`）把 `IngestDeps.Logger` 也填进 `Index.Logger`（B 那一行 `Debug` 的 logger，3.2；审查 B 第 2 轮的 N2）。
- `B/trash_cost_test.go`：`newTrashBenchOf` 可以带一个 `pgx.QueryTracer`（同 `B/wire.go:129-136` 的做法），给 4.4 的守卫。
- 测试：第 4 节的几个文件；`Makefile` 的 `test-go-costs-2` 加两行、`test-go-costs-3` 加一行；`.github/workflows/ci.yml` 的注释记各份加的秒数，`costs-2` 的时限照约两倍的规则改（决定 660）。
- `server/tools/perf`：`perf ops` 加 `-quiet notebook`（`main.go`、`drive/ops.go`、`client/psql.go` 与测试；6.3 第 5 条，决定 661）。
- 文档：第 8 节。

不改：`links_reached.go` 与 `LinksReached`（门槛之下照旧，M10 的计划测试不动）、`LinksReachedExcept`、事务与锁、REST 与 MCP 的接口、网页、`config.yaml`、`perf vault`（`-part` 归 P4/S6，决定 658）。

### 2.3 本文的取舍（决定 652–667）

都是主持者的决定，负责人可以改判。正式编号是主持者在设计提交时给的，依次是审查与修订时的临时号 P4S7D-1 到 P4S7D-16（审查记录的"P4/S7 的取舍"一表并列两者）。

1. **652 A 与 B 都做**（主持者 2026-11-21 已定：不改接口、网页、事务与锁，不是"大改"，不问负责人）。只有 A 时移动、改名的主行约 13 秒、× 1.25 一行约 16 秒（1.4）。
2. **653 门槛：到达的键与目标合起来至少 4,000 个**（3.1）。只数键与目标，不数来源：两条读的来源那一支相同，差在键与目标；改名、移动、删除的到达里键与目标都约是子树的大小（一个节点约 2 个）；内容的写的到达是来源加改了的别名的键，再多的来源也留在今天的读上（M10 的稠密形状不变）。切点（1.3 第 2 条）：有统计时 3,923 处两者持平、7,748 处宽读快 1.6 倍；无统计时 1,373 处今天的稍快、3,923 起宽读快；通用计划下切点在 1,373 与 7,748 之间。取 4,000：约 2,000 个节点以上的改名、移动、删除走宽读，B4 的 110,010 远在其上。`f1`（12,849）也在其上，有统计时那一格宽读反而慢约 20 ms（1.3 第 2 条），在"不退步"之内。
3. **654 宽读两条、按 `plan_cache_mode` 选**（3.1）：
   - 平常读 `LinksReachedWide`（1.3 的 v2）：按参数规划时数组是常量，`= ANY` 由 PostgreSQL 哈希，不看大小、不看 `work_mem`；59,911 个节点有统计 511–537 ms、无统计 937 ms。
   - 设了 `force_generic_plan` 时读 `LinksReachedWideGeneric`（1.3 的 v4h）：参数数组不能哈希，成员与排除都写成对 `unnest` 的子查询，通用计划里四个都哈希；有统计、无统计、加垫的 51 万个元素都在 1 秒之内。
   - 读设置在 `Store.Links` 里、过门槛时才读：sqlc 生成的 `PlanCacheMode`，经 `s.queries(ctx)`（缓存的语句，一个往返；在单元的事务与连接上），不经 `Planned`（它要两个往返，读设置没有数组参数，用不着）。
   - 三条读各在自己的文件里、各是一个同形的转发函数，archtest 的 `rawSQLCollectorCall` 不放宽（审查 A 的 I1）。
   - 与设计简报的出入：简报要"出发页不在来源里"写成对 `unnest(sources)` 的反连接、两种计划都线性。只写成反连接（1.3 的 v3）两种计划都不线性：按参数规划时子计划只在 `work_mem × hash_mem_multiplier` 放得下时哈希（默认 8 MB，约 21 万个来源；`work_mem` 1 MB 时约 5 万个，B4 的 6 万个就放不下），通用计划无统计时键的过滤逐行比参数数组（687 个节点 7.9 秒）。所以反连接只用在通用计划的那一条里，平常的那一条用常量数组的 `= ANY`（2.4 丁、戊、己、庚、卯、辰）。
4. **655 `LinksReachedExcept` 不改**：它只在 LLM Wiki 的笔记本里、一个编辑单元写了准备好的来源时用（`L/app/prepared_sources.go:106-118`）；B4 的四种操作都不走它：移动、改名不写内容也不删页，不起导入（`L/app/ingest.go:47-58`）；删除在 LLM Wiki 的笔记本里会起导入，但不填 `supplied`，`ingest.links` 仍走 `Store.Links`（`prepared_sources.go:107-108`），也就走得到宽读。它的到达要大，得是一个 batch 同时写准备好的来源、又删除、移动或改名一个大的子树。到达的上界（审查 A 的 Q1，推断）：
   - **删除**：batch 计划把删掉的子树里每个节点各数一项（`P/domain/batch_plan.go:674`），项过 `MaxUnitNodes` 1 万就拒（`:356-357`）；所以至多约 1 万个节点、键与目标约 2 万个。按 1.3 的 v0 推：按参数规划约 0.2–0.3 秒（6,871 个节点 112–180 ms 线性外推），通用计划约 5 秒（6,871 个 2.6 秒按平方外推）。
   - **移动、改名**：推不出比笔记本小的界。原地改名只数被改名的那一个节点、不读子树（`batch_plan.go:572-577`）；换父的移动数它搬的节点，但只在多于一个移动时比 `MaxUnitNodes`（`:609`，"a lone move as P2's move"）；batch 至多 100 步、400 次改写（`MaxBatchOps`、`MaxBatchRewrites`）都不限子树的大小。所以一个写来源又移动或改名整个最上层的 batch，到达就是那个最上层：6 万个节点今天的读一次约 3 秒（1.3 v0 有统计 3,022–3,066 ms），同 B4 没修之前。
   - 一起改要多一条宽读、多一组计划测试与逐行对照；这种 batch 少见（写来源的 agent 一般不同时搬整个最上层），记进第 9 节的已知限制。
5. **656 B 的条件写死、经 `Write` 带一份单元的备忘**（3.2）：同 `Write.sources`（`P/app/extension.go:51`，`unit.go:296` 每个单元一份）的做法，跨模块交给链接的先例是 `Event.BeforeIngest`（`unit.go:787-789`，`B/linking.go:43-45`）。条件：树单元、不是 batch（`s.Batch != nil` 给的显式布尔，审查 B 的 I1）、是重定位（参与者一侧判），单元的变化与参与者跟着的逐个相同（观察者一侧判）。参与者要写页之前丢掉自己存的，观察者在 `spread` 之前取走（不论用不用），不存到达（审查 B 的 M2）；方法对 nil 接收者安全（审查 B 的 N3）。
6. **657 判据照 3.6 的 B4**（2.1）：删除的 `fsync=off` 取三次的中位、记序号；× 1.25 只列给主持者；改被链得最多的页"不退步"是改后两轮的较大中位 ≤ 改前 × 1.2，违反了复核两轮、仍过列给主持者。
7. **658 6 万个节点的最上层由两次导入搭成**（6.3 第 1 条）：P4/S6 给 `perf vault` 加 `-part k/n`（主持者的决定，审查 C 的 M7、N6：S6 本来就改 `vault.ImportLimits()` 与导入上限的 vault 形状，两处改同一个文件），同一个种子与 S3 的参数（S6 改了默认的形状，量时逐个写出 S3 的参数）的 vault 按第一层文件夹分成两份（f1–f5、f6–f10，各约 3 万个节点，在 45,000 之内），两次 `perf transfer -import` 都导到同一页 `vault` 之下，不用再移动。内容与 S3 的 vault 相同，存储与历史有三处不同（审查 C 的 M8）：
   - **变更集**：一次导入一个、约 59,910 条条目；两次导入是两个。
   - **行的次序**：vault 的页按 `i % 叶数` 轮着放进叶（`tools/perf/vault/vault.go:110-112`、`:127`），一次导入里 f1–f10 的行轮着出现约 4.6 遍；两次导入是 f1–f5 的行全在 f6–f10 之前，删 `f1` 碰的堆页因此更集中。
   - **跨份的链接**：一成"整本里均匀抽"的链接，约一半从第 1 份指到第 2 份（推断约 6–7 万条），第一次导入时写成没解析，第二次导入的观察者再逐条更新（不是 HOT，留下死行）。

   所以 6.3 第 1 条的核对比节点数、链接数多，删 `f1` 只与同搭法的改前比，S3 的数只作历史引用。
8. **659 改前重量，交错着量，不用 S3 的数**：（a）S6 之后一次导入搭不出这个最上层，S3 的数是一次导入的；（b）调查的库同一个种子、同样的代码就比 S3 快约 15%（1.2），搭法与库的状态影响读数；（c）删除随历史变慢，改前改后要同样的序号；（d）隔几夜的机器漂移可以抵过"不退步"的 × 1.2（审查 C 的 M7）。所以改前、改后用同一对 zip、同一个 `perf`，在同一段里交错着跑；改前钉成 S7 分支的合并基点（量那一刻 S7 的头与 `main` 的 `git merge-base`），改前、改后只差 S7 的提交（6.3 第 3 条；审查 A 第 2 轮的 M1）。
9. **660 组合根的守卫约 1 万个节点、以按语句名数调用的结构断言为主，进 `test-go-costs-3`；4.1、4.2 的两个 `!race` 测试进 `test-go-costs-2`，`costs-2` 的时限照约两倍的规则提高**（4.4、4.5；审查 C 的 I1、Q1、M2、M3，主持者第一次修订说明的第 3 条、4）：
   - 规模：45,000 个节点的计时复现不出 B4（今天的读对只有 A 只慢 1.13–1.25 倍，审查 C 的探针），又给 7763 上已在两倍线上的 `guards-core` 加 2–3 分钟；1 万个节点的键与目标约 2 万个，远过门槛，结构的断言照样抓住 A、B 的回退。
   - 界：max（本机两轮均值 × 3.6，本机两轮均值 + 2 秒）。3.6 倍是"本机三倍"与"本机 × 2.4 × 1.25 × 1.2"中较大的（后者总是较大）；1 万个节点上的操作是亚秒的、删除与无统计的移动只有一个样本，加 2 秒的绝对下限（审查 C 第 2 轮的 M1，依据在 4.4）。它守结构与期限，不守 B4 的余量。
   - 份：按 7763 上的作业时间算（4.5 的表）。七份在 7763 上只有 `costs-1` 与 `costs-3` 还在约两倍的线之下，`costs-3` 余 1.1–2.4 分钟，放得下守卫（推断约 0.5 分钟）；两个 `L/adapter/postgres` 的测试推断约 4 分钟，哪一份都放不下，放在 M10 的 `LinksReached` 计划测试旁的 `costs-2`（它在 7763 上已过线 2.2–3.1 分钟），时限照先例从 45 分钟提到〈约 60〉分钟，写进 `ci.yml` 的注释。
10. **661 编辑者负载下的四种操作，改前、改后各一次、只记**（S3 审查 A-M4、C-M3；审查 C 的 M10）：`perf editors` N = 20 跑在另一本笔记本 `editors` 里，`perf ops -reps 1 -quiet notebook` 在它稳态之后起、在它结束之前完（6.3 第 5 条）。`perf ops` 现在每次操作之前等全局安静（`client/psql.go:87-90`，`drive/ops.go:34-47`），编辑者每 2 秒一次保存，搜索的队列一直有东西，等不到；S7 的第 4 个提交加 `-quiet notebook`：只等 `vault` 那本笔记本的搜索队列与搜索之外的 River 任务。
11. **662 病态形状照改后的数重新换算到 7763**（6.3 第 6 条），标推断，仍是已知限制。400 页 × 2,000 条链接的 `move` 与被 2,500 页链着的页的改名 S7 不改它们的读与写（到达在门槛之下，A 不碰；参与者改写页，B 不用），照 F.md 的换算（审查 C 的 M11）。
12. **663 调查的 E 是已知限制（主持者已定）；F 的两小项不做**：A 加 B 之后移动、改名的主行推断约 7 秒（1.4），用不着。改后哪一行的主行仍过 15 秒，停下列给主持者，F 的两项与它们的读数（节点长度每边约 0.35 秒 CPU；路径读每对约 0.75 秒、推断能省一半）作候选，不自动做。
13. **664 今天的 `LinksReached` 在通用计划下对中等以上的到达是平方的**（1.3 第 5 条），门槛之下留着：花费随指向到达的键的链接数乘来源数，没有界（vault 的密度下 3,923 个时 269 ms）；生产不走通用计划（`Planned`，`auto` 下总是按参数规划），只在运维设了 `force_generic_plan` 时出现。记进第 9 节。
14. **665 次序写死 S6 → S7**（审查 C 的 N6）：S6 合并之后再量；代码可先写（5.2）。
15. **666 移动、改名的期限只在不改写任何页时成立**（主持者第一次修订说明的第 7 条，审查 B 的 Q2）：改写了页的大子树 B 不用，只有 A 的余量，推断主行约 13 秒、× 1.25 约 16 秒；主行仍在 15 秒之内，满足负责人定的目标；× 1.25 那一行照 3.9 第 3 条只列给主持者。记进第 9 节。
16. **667 观察者不另算到达再与参与者的比**（主持者第一次修订说明的第 6 条，审查 B 的 Q1）：今天两边的到达在 B 的条件之下相同（3.2"为什么"第 4 条）；以后有人改 `spread`、`Affected` 的用法，由代码旁的注释与 4.3 第 5 条"两边读 `Store.Links` 的到达相同"一格守着，不在生产里每次多读一次子树（约 0.1 秒）。

### 2.4 比较过、不取的

**甲、语义上收窄到达（调查的 C）**：移动一个自成一体的子树，子树里的页的链接大多不会换去处，可以只读"可能换去处的"。要重写 M6/P3 3.4"候选为什么找全"的论证，靠"增量等于重建"的性质测试兜着，风险大；省下的读 A 已经省了大半。不在 v0.1 做，进第 9 节。

**乙、改成后台任务（调查的 D）**：负责人第 13 问第 1 题的"大改"；接口、网页与事务都要变。A 加 B 推断已在期限之内，不取。

**丙、提高请求期限或给树写单独的期限**：B4 的期限就是 15 秒的请求期限（`config.yaml:17`），提高它是改预算不是修；负责人选的是修。不取。

**丁、只一条宽读，用 v3（排除写成对 `unnest(sources)` 的反连接）**：设计简报的写法。按参数规划时排除只在 `work_mem × hash_mem_multiplier` 放得下时哈希（加垫的 `vault` 过 45 秒；`work_mem` 1 MB 时 B4 的 6 万个就放不下）；通用计划无统计时键的过滤逐行比参数数组（687 个节点 7.9 秒，更大的过 60 秒）。不取。

**戊、只一条宽读，用 v4h（成员与排除都是子查询）**：通用计划里处处线性，按参数规划时同丁一样受 `work_mem` 管（加垫的过 45 秒），而且有统计时也读整本笔记本（687 个节点 529 ms、59,911 个 809 ms，v2 511）。只用在通用计划里。

**己、只一条宽读，用 v2**：平常最稳，通用计划下排除逐行比参数数组，平方。只用在平常。

**庚、用简单协议发宽读，让数组都成常量**：计划方式管不着，一条 v2 就够，不必读设置；但 2–3 MB 的语句文本每次解析，`pgx` 的简单协议要自己把 `uuid[]`、`text[]` 写成字面量，项目只用 `DescribeExec` 与缓存两种方式（`planned.go`、`store.go:485-487`）。不取；主持者若不愿多一条语句与一次读设置，这是替代。

**辛、v4j（成员与排除都是连接）**：键是逐个的索引查找，同 v0 一样慢（59,911 个 5.7 秒）；通用计划无统计时选了嵌套循环，687 个节点 20 秒。不取。

**壬、只留一条语句、改今天的 `LinksReached`**：小的到达宽读更慢，无统计时一页也要约 180 ms（1.3），会让每次保存都变慢。不取，用门槛。

**癸、把数组先写进临时表**：每个单元多建表、写表、删表，计划另有统计的问题。不取。

**子、B 让参与者提前取索引的咨询锁**：能让"读到的就是观察者会读到的"不靠树单元的行锁，但改了锁的次序（主持者：不改锁）。不需要：树单元从头就拿着笔记本的行（3.2 的论证）。不取。

**丑、一并改 `LinksReachedExcept`**：决定 655。

**寅、F 的两小项**：决定 663。

**卯、宽读前后改设置**（审查 A 的 N7）：读之前 `set_config('plan_cache_mode', 'force_custom_plan', true)`、读完改回，只要一条 v2、不必两条语句。但它盖过运维的设置，与 `planned.go:12-15`"不强制按参数规划，语句要在通用计划下也能用"相悖。不取。

**辰、一条 v4h、数组藏在初始计划后面**（审查 A 的 N7）：写成 `(SELECT a.keys)` 一类，估成 10 行，两种计划都哈希（推断，没量）；但它对任何到达都读整本笔记本，2,000–4,000 个节点 537–565 ms，对 v2 的 49–61 ms，余量不同。不取。

## 3. 改法

### 3.1 A：大的到达走宽读（`L/adapter/postgres`）

**两条语句**（`links.sql`，接在 `LinksReached` 之后；参数与列同 `LinksReached`，`gen` 另生成两个行类型，收集器照旧扫进同样的七个变量，`links_reached.go:67-69`；sqlc 给两条同样的列生成同样字段的行类型，若生成了 `LinksReachedWideRow` 等，收集器不用它们）：

```sql
-- name: LinksReachedWide :many
-- LinksReached's rows, row by row, for a reach of many keys and targets (M12/P4/S7 design 3.1): the links its
-- keys and targets meet in one pass over the rows they meet, not an index search a key. Planned with its
-- parameters (postgres.Planned), the arrays are constants, whose = ANY PostgreSQL hashes at any size and any
-- work_mem. Under plan_cache_mode = force_generic_plan they are parameters it compares one by one: Store.Links
-- reads LinksReachedWideGeneric then.
WITH args AS NOT MATERIALIZED (
    SELECT sqlc.arg(notebook_id)::uuid AS notebook_id,
           sqlc.arg(keys)::text[] AS keys,
           sqlc.arg(targets)::uuid[] AS targets,
           sqlc.arg(sources)::uuid[] AS sources
), reached AS (
    (
        SELECT l.source_id, l.range_start, l.target, l.resolved_id,
               l.ambiguous, l.resolved_asset, l.aliases
        FROM args a
        JOIN page_links l ON l.notebook_id = a.notebook_id
        WHERE l.source_id = ANY(a.sources)
        ORDER BY l.source_id, l.range_start
    )

    UNION ALL
    (
        SELECT l.source_id, l.range_start, l.target, l.resolved_id,
               l.ambiguous, l.resolved_asset, l.aliases
        FROM args a
        JOIN page_links l ON l.notebook_id = a.notebook_id
        WHERE (l.target_key = ANY(a.keys) OR l.target_alt_key = ANY(a.keys) OR l.resolved_id = ANY(a.targets))
          AND (l.source_id = ANY(a.sources)) IS NOT TRUE
        ORDER BY l.source_id, l.range_start
    )
)
SELECT l.source_id, l.range_start, l.target, l.resolved_id,
       l.ambiguous, l.resolved_asset, l.aliases
FROM reached l
ORDER BY l.source_id, l.range_start;

-- name: LinksReachedWideGeneric :many
-- LinksReachedWide under plan_cache_mode = force_generic_plan (M12/P4/S7 design 3.1): a parameter's = ANY is
-- not hashed there, so the keys, the targets and the sources are sub-selects of their arrays, each a hashed
-- sub-plan in a generic plan (unnest of a parameter is estimated at 10 rows; the hash is built from the array as
-- it is). Each sub-select reads its array through args: unnest(a.sources) refers to the outer row, so NOT EXISTS
-- stays a sub-plan, hashed; unnest($4) would be pulled up into an anti-join whose method the planner chooses.
-- Not for plans with the parameters: a sub-plan estimated past work_mem * hash_mem_multiplier is not hashed, and
-- the keys are no index condition here, so it reads the notebook's rows whatever the reach.
WITH args AS NOT MATERIALIZED (
    …同上…
), reached AS (
    (
        …来源那一支同上…
    )

    UNION ALL
    (
        SELECT l.source_id, l.range_start, l.target, l.resolved_id,
               l.ambiguous, l.resolved_asset, l.aliases
        FROM args a
        JOIN page_links l ON l.notebook_id = a.notebook_id
        WHERE (l.target_key IN (SELECT k.value COLLATE "C" FROM unnest(a.keys) AS k(value))
               OR l.target_alt_key IN (SELECT k.value COLLATE "C" FROM unnest(a.keys) AS k(value))
               OR l.resolved_id IN (SELECT t.id FROM unnest(a.targets) AS t(id)))
          AND NOT EXISTS (SELECT 1 FROM unnest(a.sources) AS s(id) WHERE s.id = l.source_id)
        ORDER BY l.source_id, l.range_start
    )
)
SELECT …同上…;

-- name: PlanCacheMode :one
-- The session's plan_cache_mode, which chooses between LinksReachedWide and LinksReachedWideGeneric
-- (M12/P4/S7 design 3.1).
SELECT current_setting('plan_cache_mode')::text AS mode;
```

- 两条都与 `LinksReached` 逐行相同：来源那一支同今天；另一支取键、别名键、目标中了任一个、出发页不在来源里的行，正是今天后三支的并（今天三支互斥，是为了不去重）。`NULL` 的键与目标不中任何行，同今天的 `k.value IS NOT NULL`。`(source_id, range_start)` 是主键，两支不相交（第二支排除了来源），`ORDER BY` 给出同样的全序。审查 A 逐类核对过（别名键、目标与来源在子树里外、别的笔记本、大小写与 `ß`、空数组与 `NULL`、重复的元素、同一事务里刚删的行、并行的 worker）。
- `target_key`、`target_alt_key` 是 `COLLATE "C"` 的列（`00021_linking_page_links.sql:20-21`），`= ANY(a.keys)` 取列的排序规则，用得上 `(notebook_id, target_key)` 与 `(notebook_id, target_alt_key)` 两个索引（1.3 的计划）。子查询里的 `COLLATE "C"` 与去掉它等价：比较取列的隐式 `"C"` 与默认排序规则的合并，非默认的隐式排序规则胜出，仍是 `"C"`（审查 A 的 N3）；留着，同今天的键那一支，读得明白。
- **读哪些行**：无统计时两条都只用 `notebook_id` 打头的索引取本笔记本的位图（1.3：两本笔记本时 v0、v3 与审查 A 补量的 v2、v4h 都只读本笔记本）。有统计时读不读整张表由规划器按代价定：本笔记本占表的大半、或 `work_mem` 小时，见过整表的扫描（1.3 两本笔记本一段末），用时仍是线性的（审查 A 的 M1）。

**三个转发函数**（archtest 的 `rawSQLCollectorCall` 不放宽，`server/internal/archtest/rawsql_test.go:135-264`：函数体的第一条语句必须正是对自己那个 `gen` 常量的 `q.Query`，受保护的 `ctx`、`q`、`gen`、`notebookID`、`r` 只在这次调用的实参上，整个文件只有一次匹配的调用）：

```go
// links_reached_wide.go
// linksReachedWide reads LinksReachedWide through the planned querier
// supplied by Store.Links, with linksReached's collector (M12/P4/S7 design 3.1).
func linksReachedWide(ctx context.Context, q postgres.Querier, notebookID uuid.UUID, r domain.Reach) ([]app.Link, error) {
	rows, err := q.Query(ctx, gen.LinksReachedWide, notebookID, r.Keys, r.Targets, r.Sources)
	if err != nil {
		return nil, err
	}
	var links linksReachedBuffer
	return links.read(rows)
}
```

`links_reached_wide_generic.go` 的 `linksReachedWideGeneric` 同形，读 `gen.LinksReachedWideGeneric`。`links_reached.go` 一字不改。archtest：`switch` 加这两个路径（名字、参数、结果同 `linksReached`），`:219` 判结果类型可以是 `[]app.Link` 的那一行加这两个名字；`rawSQLLinksReachedQuery`（`:267-271`）改成"第二个实参是这个文件自己的常量"的匹配器；`rawsql_links_reached_cases_test.go` 照现有的形状给两个新文件各加一组（含 `local_sql_variable`、`wrong_sql_constant`：把 `gen.LinksReachedWide` 换成另两个常量之一也报）。

**选语句**（`store.go`）：

```go
// linksReachedWideFrom is the keys and targets together of a reach read in
// one pass over the rows they meet (LinksReachedWide, or under
// force_generic_plan LinksReachedWideGeneric): fewer, one index search a
// key (LinksReached), which is faster for a few and without statistics
// (M12/P4/S7 design 1.3, 3.1).
const linksReachedWideFrom = 4_000

// Links implements app.Store, …（今天的注释，加一句宽读）
func (s *Store) Links(ctx context.Context, notebookID uuid.UUID, r domain.Reach) ([]app.Link, error) {
	read := linksReached
	if len(r.Keys)+len(r.Targets) >= linksReachedWideFrom {
		mode, err := s.queries(ctx).PlanCacheMode(ctx)
		if err != nil {
			return nil, fmt.Errorf("links of %s: %w", notebookID, err)
		}
		read = linksReachedWide
		if mode == "force_generic_plan" {
			read = linksReachedWideGeneric
		}
	}
	out, err := read(ctx, postgres.Planned(postgres.DB(ctx, s.pool)), notebookID, r)
	if err != nil {
		return nil, fmt.Errorf("links of %s: %w", notebookID, err)
	}
	return out, nil
}
```

- `PlanCacheMode` 经 `s.queries(ctx)`（`store.go:41-43`，缓存的语句，一个往返），与读链接都经 `postgres.DB(ctx, s.pool)`：有事务时是单元的事务、同一个连接，会话里 `SET`、`SET LOCAL` 的，角色、库上设的（`revert_cost_test.go:134`），连接参数设的（`grep_cost_test.go:236`）都读得到。设置名在 `current_setting` 里是规范的小写。
- 门槛之上的读都在单元的事务里；事务外的 `Store.Links` 只带来源（`batch_links.go:93`、`rebuild_pages.go:48`），走不到门槛（审查 A 的 N5）。走到了，两次经池，可能落在两个连接上，设置只有会话自己 `SET` 的才会不同；今天没有这样的调用。
- 键与目标在这之前已去重（`spread` 末尾的 `reach.Compact()`，`L/app/index.go:122`；参与者同一个 `spread`）。
- 三条读都经 `postgres.Planned`，同今天：每次描述再执行，`auto` 下每次按参数规划（4.1 第 4 条守着）。

**别的走到宽读的单元**：门槛只看到达，不看是哪种操作。过门槛的还有（审查 A 的 M2）：

- `test-go-guards-core`：恢复在单元的上限（`TestARestoreAtTheUnitsBoundTakesItsTime`）、撤销一次删除与撤销多个最上层（`TestARevertAtTheUnitsBoundTakesItsTime`、`TestARevertOfManyTopsAtItsBoundTakesItsTime`，同一行）、后台任务里删除一个最上层的那一步（`TestARevertJobOfAnImportAtMaxJobItems`）、batch 里改写的界（`TestABatchAtItsRewriteBoundTakesItsTime`）；
- `test-go-guards-sources`：来源的移动、恢复、撤销（`TestSourceMoveAtTenThousandTakesItsTime`、`TestSourceRestoreAtTenThousandTakesItsTime`、`TestSourceRevertAtTenThousandTakesItsTime`，`Makefile:605-617`）：一页带 99 个来源、9,900 个附件，共 10,000 个节点（`source_cost_test.go:122-134`），键与目标约 2 万个；三种阶段 `no_statistics`、`analyzed`、`force_generic_setting`（`source_http_cost_states_test.go:46`），后者是整个程序里唯一一条走到 `LinksReachedWideGeneric` 的路；界：移动 1.4 秒，恢复与撤销 7.6 秒（`source_cost_test.go:26-30`）。来源的移动也是 B 的条件第一次在 LLM Wiki 的笔记本里成立的地方（移动不起导入，3.2）。

导入的单元（每个 100 个节点，`server/internal/modules/transfer/app/batches.go:35`）与内容的写留在今天的读上。S7 不改这些守卫的界；改前改后各跑两轮，用时写进第 10 节，变慢的列给主持者（6.3 第 7 条）。

**宽读的花费随什么**：没有统计时宽读读本笔记本的全部行的位图（1.3：135 万行的笔记本里，687 到 6,871 个节点都是 226–244 ms），不随到达；今天的读随到达。一本远大于 135 万条链接、又没有统计的笔记本里，刚过门槛的到达会比今天慢（推断；导入每 1 万个节点 `ANALYZE` 一次，`batches.go:38`，生产里少见）。有统计时规划器可以选整表的扫描，花费随整张表（审查 A 的 M1）；今天的 v0 无统计时目标那一支也整表扫（审查 A 的 `b_nostats.out`：`Parallel Seq Scan`，三个进程合计 32 万行），不是新的一类。都记进第 9 节。

### 3.2 B：一个单元只读、只解析一次（`L/app`、`P/app`、`B/`）

**备忘怎样带**：

- `P/app` 的 `Write`（`extension.go:27`）加一个导出的字段 `Memo *UnitMemo`。`UnitMemo` 是一个小类型：`Store(key, value any)`、`Take(key any) (any, bool)`（读出即删）、`Drop(key any)`；三个方法对 nil 接收者安全（nil 时 `Take` 答没有，另两个什么也不做），所以没带备忘的 `page.Event`（例如 `B/assets_observer_test.go:39` 造的）经适配器成了"非 nil 的接口、nil 的指针"也不 panic（审查 B 的 N3）。
- `Writer.run` 建 `Write` 时同 `sources` 一起建一份（`unit.go:293-296`）。`u` 在 `WithinTx` 的闭包里建（`unit.go:274-298`），重试的一次自然是新的一份，写进注释。参与者拿到的 `Step` 与观察者拿到的 `Event` 都嵌着这个 `Write`（`extension.go:57-58`、`:228-229`），是同一份。
- 先例：`Write.sources` 是页模块里每个单元一份的东西，不导出；跨模块、带单元的东西交给链接的先例是 `Event.BeforeIngest`，单元自己经 `linkIngestBefore` 交给观察者（`unit.go:787-789`，`B/linking.go:43-45`）（审查 B 的 N3）。
- `B/linking.go`：`linkRewrite.Participate`（`:72`）把 `s.Memo`、`s.Tree` 与 `Batch: s.Batch != nil` 放进 `linking.Moved`；`linkIndex.PagesChanged`（`:25`）把 `e.Memo` 放进 `linking.PagesChanged`。链接一侧在 `ports.go` 定义接口 `UnitMemo`（同样三个方法），不依赖页模块。
- 键是链接模块里一个不导出的类型，别的模块拿不到。
- 不并发：备忘只在参与者的 `reached`、`Participate` 与观察者的 `PagesChanged` 里碰，三者都在单元的 goroutine 上；改写的窗口另起的解析 goroutine（`L/app/rewrite_window.go:63`）不碰它（审查 B 的 N4）。

**参与者一侧**（`Rewrite.reached`，`rewrite.go:290`）：读完 `links`、算完 `after`（`resolutions`）之后，下面三条都满足就把 `{changes, links, after}` 存进备忘（不存到达：B 之后没有人用它，`ingest.links` 被跳过，`publish` 不要它；审查 B 的 M2）：

- **条件 1，树单元**：`m.Tree`。只在参与者这一边判：备忘是单元的，只有参与者存，内容的单元里观察者找不到什么（审查 B 的 N6）。
- **条件 2，不是 batch**：`!m.Batch`，由 `s.Batch != nil` 给出的显式布尔（审查 B 的 I1）。不靠 `m.Written`、`m.Gone`、`m.Frozen`、`m.CreatedAfter` 都空来推：只有移动、改名（都带 `update_links`）的 batch 四样都是空的（`Written` 来自 `a.decided`，只收内容操作写过的页，`P/app/apply_batch.go:582-599`；`Gone` 是计划里删掉的节点，`:893`；`CreatedAfter` 只记新建的节点，`P/domain/batch_plan.go:827-839`；`Frozen` 只记 `update_links` 为假的移动，`unit.go:603-605`），那条路上延后的参与者与 `tell` 之间还有 `DropUnchanged` 与 `answer`（`apply_batch.go:657-662`），下面的论证没有走它。
- **条件 3，是改名或移动的重定位**：`relocates` 为真。今天它只免得存一份没人取的备忘：`relocates` 为假时（同一个父下只改大小写），每一项都保留节点、版本为 0、无一重定位，`domain.Affected` 答否（`L/domain/change.go:200-229`），观察者在 `index.go:43-46` 就返回，取锁之前、看备忘之前（审查 B 的 M1 的探针："Note"改成"note"，参与者读一次，观察者读 0 次）。所以去掉它是等价的变异（6.1 的 b3），依据随 `Relocation` 与 `Affected` 共用 `relocates()` 成立。"Straße"改成"STRASSE"一类（键同、长度不同）`relocates` 为真，两边的到达都来自 `Affected`（`rewrite.go:293-301` 的到达被覆盖），B 照样对。

存的 `changes` 是 `m.Changes` 的一份拷贝；`links` 是读到的、过滤之前的（条件 2 之下过滤不去掉任何一行，`DeleteFunc` 在存之前）。参与者之后照旧决定改写哪些页；**要写页时先丢掉自己存的那一项**（`Participate` 里 `readTree` 之后 `len(all)+len(met.pages) > 0`，在写之前 `Drop`；审查 B 的 M2）：改写了页的单元不用 B，参与者那一份（6 万个节点的最上层约 135 万个 `Link` 与同样多的 `Resolution`，推断约 150 MB）不留到单元结束。写了又全被留下的少见情形因此也不用 B，安全。

**观察者一侧**（`Index.PagesChanged`，`index.go:42`）：`Affected` 答是、取了锁、起完导入（`beginIngest`）、处理完内容的写与删除之后、`spread` 之前，**取走**备忘里参与者存的那一项（不论下面成不成立，取走即删；审查 B 的 M2），且：

- **条件 4，单元的变化就是参与者跟着的那一步的变化**：`e.Changes` 与存的 `changes` 逐个相同（个数、次序、节点、之前与之后的父与名、版本）。单元里还有别的步（batch 之外的多步单元今天没有，守着以后）、有删除的，都不同；参与者改写了页时它已丢掉了那一项，条件 4 是第二道。它也让导入不起（导入只看写了内容或删除了的页，`ingest.go:51-57`），`ingest.links` 只会走 `Store.Links`。

满足：`links` 用存的，`changed` 由存的 `after` 与 `links` 比出（同 `resolve` 里的比较，`resolver.go:25-30`），跳过 `spread`、`ingest.links` 与 `resolve`；之后的 `SetResolutions`、`ingest.finish`、`publish` 照旧（`publish` 只读 `links` 与 `changed`）。不满足：照今天。B 用上的那一处打一行 `Debug` 日志（4.3 第 8 条数它）：`Index` 今天没有 logger（`index.go:25-31`），加一个 `Logger` 字段，由 `NewIndex`（`L/module.go:163-172`）从 `IngestDeps.Logger` 填（应用的 logger 经 `B/deps.go:152`、`registrants.go:267-270` 已到这里），nil 时不记（替身里 `Ingest` 与它都是 nil）；审查 B 第 2 轮的 N2。

**为什么参与者读到的就是观察者会读到的**（审查 B 逐条对过代码）：

1. **同一个事务**：两者都在单元的事务里（`unit.go:274`），READ COMMITTED，每条语句一个新的快照。
2. **其间别的事务不写这本笔记本的 `page_links`、`page_aliases`、`indexed_pages` 与 `nodes` 里链接一侧读的行**：树单元从头就拿着笔记本的行 `FOR NO KEY UPDATE`（`unit.go:278-282` 的 `LockByID`），内容单元的 `ShareByID` 是 `FOR SHARE`，二者冲突（`notebook/adapter/postgres/queries/notebooks.sql:16-34`）。
   - `page_links`、`page_aliases`（`L/adapter/postgres/queries/pages.sql:27-58`）、`indexed_pages` 的写都在 `L/adapter/postgres/store.go`，只有三处调用，都要这一行：页的单元里的观察者（至少 `FOR SHARE`，`unit.go:278`）；重建索引 `Rebuild.Notebook`（`LockByID`，`L/app/reindex.go:94`，之后取索引的锁，`:103`）；删除笔记本（`FOR NO KEY UPDATE`，`server/internal/modules/notebook/adapter/postgres/queries/cascade.sql:12`、`:45`；链接的那一步在 `L/app/deletion.go:24`）。仓库别处的 SQL 不碰这三张表。
   - `nodes` 的写者：页的单元；删除笔记本（行已被锁住）；`Rebuild.Notebook` 的 `Rekey`（在 `LockByID` 之下）；回收站的清理（`page/adapter/postgres/queries/purge.sql:48`）不拿笔记本的行，但它只删已删的节点，链接一侧的读都滤 `deleted_at IS NULL`（`page/adapter/postgres/queries/links.sql:13`、`:32`），不影响。`page_links` 没有外键（`migrations/sql/00021_linking_page_links.sql:2`），清理不会经级联删到它（审查 B 的 N1）。
   - 索引的咨询锁只有观察者与 `Rebuild` 取，二者都在笔记本行之下，所以参与者在咨询锁之外读，对树单元无妨。

   所以这期间提交的别的写一个也没有，第二次读的快照与第一次看到同样的别人的行。
3. **单元自己其间不写它们**（非 batch 的路；batch 由条件 2 挡在外面）：参与者在 `applyStep` 里（`unit.go:424`），之后 `RenameNode`、`MoveNode` 的 `do` 只再读一次 `FindNodeIn`，然后是 `markMoves`（变更集的两个标记，`:740-755`）、`queueSearch`（搜索的队列，`:765`）与 `tell`（`:782`）里 `pageEvents` 之后、`assetObserver` 之前的观察者（`B/registrants.go:273`）。生产的参与者只有 `linkRewrite` 一个（`registrants.go:291-296`）。参与者写的内容在 B 用上时不存在；`page_links` 只由观察者自己在这之后写。
4. **算法相同**：条件 2、4 之下，观察者的到达是 `domain.Affected(e.Changes)` 加 `spread(…, renamed, nil)`，参与者的是 `domain.Affected(m.Changes)` 加 `spread(…, renamed, setsOf(m.CreatedAfter))`，`CreatedAfter` 空，变化相同；观察者的解析是 `resolutions(links, nil, nil)`（`resolver.go:17`），参与者的是 `resolutions(links, nil, gone)`，`gone` 空时与 nil 相同，`missing` 两边报同一个错。两边的端口是同样的实现：`Store` 都是 `postgresadapter.New(pool)`，`Pages` 都是无缓存的 `linkTargets{page.NewLinkTargets(pool)}`（`B/registrants.go:267`、`:292-294`；`B/linking.go:285-297`；`server/internal/modules/linking/module.go:169-170`、`:247`），都经 `postgres.DB(ctx, pool)` 读单元的事务。观察者不另算到达再比（决定 667）。

**内存**：B 用上时峰值不升（参与者那一份交给观察者，不另读一份）；B 不用时不多占（参与者写页之前丢掉，观察者在 `spread` 之前取走）。6.3 由 `perf sample` 记服务的 `VmHWM`（S3 至多 827 MiB）。

**以后的人改这几处时**（落成代码旁的注释，在 `rewrite.go` 存与丢的两处、`index.go` 取的那一处与 `UnitMemo` 上，指向本节；决定 667）：改 `spread`、`Affected`、`resolutions` 的参数或用法，在 `markMoves` 与 `tell` 之间加写，让树写的单元不拿笔记本的行，加新的 `page_links`、`page_aliases`、`nodes` 的写者，让 batch 也走 B，都要重看条件；4.3 第 5 条"两边读 `Store.Links` 的到达相同"一格抓两边算法分开的那一类。

### 3.3 读者看得到的

只有用时（第 0 节）。删除的历史部分照旧随子树的条目与版本数长（第 9 节）。

### 3.4 不改的，与注释

- `LinksReached` 的语句与注释、`links_reached.go` 不变；`links_reached_plan_test.go`、`links_reached_dense_plan_test.go` 不动（到达在门槛之下：键与目标合起来至多 3）。
- `planned.go` 的注释不变；`links.sql` 里 `LinksReached` 的注释末尾加一句"键与目标多的到达见 `LinksReachedWide`"；`Store.Links` 的注释加一句宽读。
- `rewrite.go:247-252`（`followed` 的注释）与 `index.go:33-41`（`PagesChanged` 的注释）各加一句 B；3.2 末的"以后的人改这几处时"。

## 4. 测试

各测试实施时本机量两轮，秒数写进第 10 节；进哪一份见 4.5（审查 C 的 M3）。

### 4.1 三条语句逐行相同，选对语句（`L/adapter/postgres`）

1. **`TestLinksReachedUnionSemantics`**（`links_reached_union_test.go:22`）：同一个夹具与每一种到达，三条语句（`LinksReached`、`LinksReachedWide`、`LinksReachedWideGeneric`）各跑一次，都等于手写的期望（每一行写死，不由被测的语句生成；它已有 NULL、空串、零 UUID、别的笔记本、各支重叠与只中一支的行）。不加 build tag（小，`race` 一份里跑）。
2. **新：`TestLinksReachedWideIsLinksReached`**（新文件 `links_reached_wide_random_test.go`，`//go:build !race`；13.4 第 13 条的长期对照，替 M10 审查 B 的 3,000 组随机参数的一次性对照）：
   - 一个随机的夹具：两本笔记本，几千页，键有大小写、`ß`、长度不同的、空串与 NULL 的，目标有零 UUID 与不存在的，链接有别名键、指向资产、歧义与别名的值。
   - 3,000 组随机的到达（键、目标、来源各从夹具里抽、混进不中的与 NULL），在 `auto` 下三条语句直接各跑一次（不经门槛），结果作为有序的列表逐个相同。大小在门槛之下（0–3,999），按对数均匀抽，多数是几十到几百个。
   - 另有固定的 80 组，键与目标合起来在 4,000–10,000（生产走宽读的都在门槛之上；让对照也覆盖大数组的计划，例如并行的计划与 `BitmapOr`；审查 A 的 N8）。组数固定而不是按比例：审查 A 在仿 4.2 的夹具上量过键与目标 8,192 个时三条语句一组约 0.2 秒（v0 91–96 ms、v2 42–65 ms、v4h 60–86 ms），五分之一的 600 组就约 2 分钟（审查 A 第 2 轮的 N2）。
   - 另一个种子的夹具在无统计、`ANALYZE` 之后各跑 300 组（门槛之下）加 20 组大的，在 `force_generic_plan` 下跑 300 组，这 300 组的键、目标、来源合起来至多 1,000 个：今天的读与 `LinksReachedWide` 在通用计划下是平方的（1.3 第 4 条），大的组单个就到秒级（审查 C 的 M3）。
   - 失败时写出种子、组号、哪两条与第一处不同。
   - 预算：本机约 60 秒之内（推断：大的 120 组约 25 秒，门槛之下的 3,900 组每组几毫秒到十几毫秒约 30 秒，夹具几秒）；超了先减门槛之下的组数，大的组不减，写明减到多少。
3. **新：`TestLinksReadWideFromItsThreshold`**（不加 build tag）：经 `Store.Links`、用 `newLinksCollectorPool` 的 SQL 记录（`links_reached_union_test.go:24` 的 `trace`）。格的大小写字面的 3,999 与 4,000（出处 1.3 第 2 条），不跟常量走，常量改了这两格红（审查 C 的 M6）：
   - 键 2,000 加目标 1,999、来源 1 个：发 `LinksReached`，不读设置（"数来源"或"三者相加"的门槛在这一格过门槛，红）；
   - 正好 4,000 个、来源 10 个，三种：键 1,000 加目标 3,000；只有键；只有目标（"只数键""只数来源"的门槛在其中一格不过，红）：先读 `PlanCacheMode`，`auto` 与 `force_custom_plan` 下发 `LinksReachedWide`；同一个事务里 `SET LOCAL plan_cache_mode = force_generic_plan` 之后发 `LinksReachedWideGeneric`（设置若经池读，读不到 `SET LOCAL`，红）；连接参数设的（`RuntimeParams["plan_cache_mode"] = "force_generic_plan"`，同 `B/grep_cost_test.go:236`）也发 `LinksReachedWideGeneric`（审查 A 的 N6）；
   - 每次宽读的 `LeadingOptions` 是 `DescribeExec`（同 `links_reached_plan_test.go` 的 `c.LeadingOptions`；审查 A 的 M3）；
   - 几次读到的相同。
4. **`TestTheLinksReachedAreReadPlannedWithTheReach`**（`store_test.go:299`，不加 build tag；审查 A 的 M3）：加一个门槛上的到达（键与目标合起来 4,000 个），读 8 次，核对 `LinksReachedWide` 不在 `pgtest.CachedStatements` 里；在一个 `RuntimeParams` 设了 `force_generic_plan` 的池上同样核对 `LinksReachedWideGeneric`。宽读若被改成经缓存的查询器发，第 6 次起服务端可能只规划一次，`LinksReachedWide` 的排除就逐行比参数数组。
5. **archtest**（`rawsql_links_reached_cases_test.go`）：两个新文件各一组，同 `linksReached` 现有的那一组（3.1）。

### 4.2 宽读的计划与物理工作量（`L/adapter/postgres`，新文件 `links_reached_wide_plan_test.go`，`//go:build !race`）

`TestLinksReachedWideReadPlans`，用 M10 的计划测试的那一套（`stablePlanFixture`、`stablePlanSettings`、逐关系的物理工作量与回滚，`links_reached_plan_test.go:29-95`），经 `Store.Links` 发（于是各格读的是 3.1 选的那一条）：

- **三种统计 × 三种计划方式**：`no_statistics`、`analyzed`、`app_rebuild_after_analyzed` × `auto`、`force_custom_plan`、`force_generic_plan`，同 `TestLinksReachedReadPlans`。`force_custom_plan` 与 `auto` 另各有一格 `SET LOCAL work_mem = '64kB'`：常量数组的哈希不看它，子查询写法的会在这里不哈希（1.3 第 3 条）。
- **夹具**：本笔记本 6,000 页、每页 20 条链接（12 万行），另一本笔记本同样的键与 20 万行（键在两本里都中）；到达是 4,096 页的子树那样的：这些页的名字的键、它们的 id 作目标与来源，键与目标合起来约 8,192 个；期望由算术写出（同稠密测试的做法，`links_reached_dense_plan_test.go:191`），不由被测的语句生成。
- **先看形状、再计时**（审查 C 的 N2）：每格先 `EXPLAIN`（不带 `ANALYZE`）核对计划的形状，不对就失败，不执行；a12、a13 一类的变异在 4,096 页的到达上是平方的，执行一次可能十几秒。
- **核对**：
  - 读的是哪一条（`force_generic_plan` 下是 `LinksReachedWideGeneric`，别的是 `LinksReachedWide`）；
  - 行等于期望，也等于 `LinksReached` 的；
  - 没有 DML；
  - 另一本笔记本的行一行也不读：这是夹具比例的性质（另一本比本笔记本大，有统计时整表扫比位图贵），不是语句的（3.1"读哪些行"；审查 A 的 M1）。审查 A 仿这个夹具量过，v2、v4h 每一格（三种统计 × 两种计划，加 64kB 两格）都只读本笔记本。实施时若哪一格红，不改语句，把那一格的这一条改成"每行至多读一遍：读的 `page_links` 行 ≤ 整张表的行数"（守住线性、放过整表扫；在这个夹具里"读了另一本"只会来自整表扫，读 32 万行，"本笔记本 + 10%"的退路照样红；审查 A 第 2 轮的 N1），计划形状的核对照旧，写明；
  - 计划的形状：扫 `page_links` 的节点上，没有对参数数组（`$n`）的 `= ANY` 留作 `Filter`、`Join Filter`，或在有损的位图上 `Recheck`（索引条件可以）；每个子计划都是 `hashed`；`LinksReachedWideGeneric` 的排除是 `hashed` 的子计划，不是 `Anti Join`（审查 A 的 N2）；没有内侧是 `unnest` 的 `Function Scan` 又 `loops > 1` 的嵌套循环（这几样是 1.3 第 3、4 条的平方）；
  - 用时：这一条语句在每格三次的最小值 ≤ 本机两轮的三倍 + 100 ms（只作兜底，不靠它判平方；几十毫秒的语句在 CI 上的抖动由 100 ms 盖住）。
- **反向**：v3、v2、v4h 三种写法各换进对应的那一条跑一次（6.2 第 4–6 条），相应的格红。

### 4.3 B 的测试（`L/app` 新文件 `unit_reach_test.go`；`P/app`；`B/`）

替身（审查 B 的 N5、C 的 N1）：能同时跑参与者与观察者的是 `rewrite_test.go:24-48` 的 `rewriting`，不是光的 `world`；观察者的变化照页模块合并（参与者经 `appender` 写的页并进去，同一节点合成一项，参照 `random_test.go:81-93` 的 `record`）。`store.Links` 今天只记最后一次的到达（`fakes_test.go:155-163`、`:221-231`），`tree.ByKeys`、`Paths` 不记：给它们加计数器，`store.Links` 记下每一次的到达。备忘用一个测试的 `UnitMemo`，看得到里面有几项。

1. **单个移动、单个改名，树单元，没有改写**：参与者与观察者一共调一次 `Store.Links`、一次 `Pages.ByKeys`、一次 `Store.Aliases`、一次路径的读。世界里放一条之前没有去处、移动之后才解析到的链接（例如移动之后 `A/X` 才存在的 `[[A/X]]`）、一条改名之后才中的链接（`[[新名]]`）与一条别名的值随之变的链接（实施时核对参与者不改写它们，单元里没有内容的变化；不然换形状）。断言 `SetResolutions` 的参数与事件的 `Pages`、`Targets` 非空，且与关掉 B（备忘为 nil）时逐个相同（审查 B 的 M3、C 的 M4）。
2. **每一条条件不满足，调用的次数与结果都与关掉 B 时相同**（不写死"两次"；审查 B 的 M1）：
   - 不是树单元（`Tree` 为假）；
   - batch：只有移动、改名（`Written`、`Gone`、`Frozen`、`CreatedAfter` 四样都空，`Batch` 为真；审查 B 的 I1）；另各带 `Written`、`Gone`、`Frozen` 或 `CreatedAfter` 一种；
   - 只改了标题的大小写（今天参与者读一次、观察者在看备忘之前返回，读 0 次）；
   - 参与者改写了一页，含被移动的页自己（同个数、同次序、同位置，只有版本不同；照真的造）；
   - 单元的变化与参与者跟着的不同（替身直接造，测条件 4 本身）：多一个、少一个、次序不同、同个数但一个变化的之后的名不同、版本不同（各一种）；
   - 有删除；
   - `UpdateLinks` 为假（参与者在 `rewrite.go:63` 就返回，只有观察者读一次）；
   - 没带备忘（`Memo` 为 nil 的指针放进接口）：不 panic，B 不用。
3. **备忘活多久**（审查 B 的 M2）：参与者改写了页之后备忘里没有它的项；观察者之后（不论用没用）备忘里没有那一项。
4. **备忘不跨单元（链接一侧）**（审查 B 的 M4）：第一个单元的参与者存了备忘、之后回滚（替身里把树复原，观察者不跑）；第二个单元同样的变化、`UpdateLinks` 为假，参与者不存，两个单元传各自的备忘。断言第二个观察者读 `Store.Links` 一次。链接一侧若把备忘放在包级的变量里（而不是单元的那一份），它会拿到第一个的，读 0 次，红（b9a）。
5. **两边的到达相同**（决定 667；主持者第一次修订说明的第 6 条）：关掉 B，在第 1 条的每一种形状与"Straße"改成"STRASSE"一格里，参与者与观察者读 `Store.Links` 的到达相同（替身记下的每一次）。
6. **`TestTheIndexOfUnitsOfManyWritesIsItsRebuild`**（`random_test.go:24`，300 个种子各 40 步，今天只跑观察者）：单个重定位的单元先跑参与者、带备忘，再跑观察者；"索引等于重建"照旧（审查 B 的 M3 的可选一条，取：比组合根的 6 × 60 步多得多的形状——`ß`、长度、附件——走到 B）。
   - 端口（审查 B 第 2 轮的 N4）：`randomUnits` 用的是光的 `newWorld`；参与者要的 `Contents`、`Sizes`、`Locks`、`Parser` 照 `rewriting`（`rewrite_test.go:24-48`）在同一个世界上建。
   - 参与者经 `appender` 写的页丢掉、只记次数：写了页的单元本来就不用 B（参与者写之前已丢掉备忘），观察者的变化仍只是这一步的、世界照旧，"索引等于重建"的对照对象不变。
   - 数 B 用上、且这一步有链接的去处变了的单元，下限取实测的三分之二（与第 8 条一起作主下限，见那一条末）。
7. **`P/app`：每个单元一份**（审查 B 的 M4、C 的 M5）：经 `Writer` 跑两个单元，一个桩参与者往备忘里存、记下指针，一个桩观察者记下它拿到的指针并 `Take`：同一单元里 `Step` 与 `Event` 拿到的是同一份，第二个单元的是另一份、`Take` 是空的；nil 的 `*UnitMemo` 三个方法不 panic。`Writer` 若共用一份，红（b9b）。
8. **`B/`：`TestTheIndexIsItsRebuild`**（`B/links_rebuild_test.go:45`，经整个程序、六个种子，改名、移动的改写在内）：
   - 数法现在定（审查 B 的 M3、C 的 M4）：`newAcmeTeamLogged` 用 `slog.NewJSONHandler(logs, nil)`（`B/mcp_batch_test.go:138`），只收 Info 及以上；这个测试改用 `newAcmeTeamLogging`（`B/interleavings_workspace_test.go:73`，收 logger），自己的 `slog.Handler` 在 `Debug` 级只数 B 那一行的消息，别的照原样交给 JSON 的 Info 处理器（不让全程序的 `Debug` 都写出来）。
   - 两个下限：B 用上的单元（钉住组合根的接线：两个适配器漏传备忘，B 就不再用上）；B 用上、且这一步前后有一条链接的去处不同的单元（测试已有每步前后的 `ledTo`，`links_rebuild_test.go:80-96`）。量一次，下限取实测的三分之二（05-lessons 第 1 节"性质测试的下限"），写进测试的注释与第 10 节。低于就失败：这个测试的"索引等于重建"由此也覆盖 B 出错会显出来的形状。
   - 今天这个测试的写者没有专门造第二种步（没解析到的链接移动之后解析到、别名的值，单个重定位又不改写页）。实测第二个数少于约 10 个时，让写者每个种子稳定地造这种步（例如先写一条指向还不存在的 `A/X` 的链接，再单独移动一页使 `A/X` 出现），并以它计数；否则三分之二的下限几乎什么也不钉（05-lessons 第 1 节"要靠的形状……稳定地造"；审查 C 第 2 轮的 N1）。第 6 条 `random_test` 的同类计数是另一个下限。
   - `B/` 另加一格：`linkIndex.PagesChanged` 收到没带备忘的 `page.Event`（同 `B/assets_observer_test.go:39` 的造法），不 panic。

### 4.4 组合根的守卫（`B/`，新文件 `tree_cost_test.go`，`//go:build !race`）

`TestTreeWritesOfALargeTopReadTheLinksOnce`（决定 660；审查 C 的 I1、M1、M2、Q1，主持者第一次修订说明的第 3 条）：

- **台子**：`newTrashBenchOf` 带一个 tracer（2.2；同 `B/wire.go:129-136`：取池的配置、换上 tracer 再建池）、配置同 `newJobBench(t, 0)`（`B/background_cost_test.go:95`：进程内的服务、真的 REST、生产的请求期限 15 秒、自己的库，表在 `ANALYZE` 之前没有统计）。开头核对 `SHOW fsync` 是 `off`（`pgtest` 的容器用 testcontainers 的 PostgreSQL 模块，默认带 `-c fsync=off`；不是就失败，不靠默认）。
- **tracer**：照 `B/ingest_cost_test.go:62-87` 的先例，按 sqlc 的 `-- name:` 数每条语句的调用；`start`、`stop` 夹着每一次请求。只看 `PlanCacheMode`、`LinksReached`、`LinksReachedWide`、`LinksReachedWideGeneric` 四个名字，名字逐字相等（不按前缀：`LinksReached` 是另两个的前缀）。
- **规模与夹具**：`bench.importOf(t, shapeDeep, 10_000)`（`:130`：根下一页 `p0`，其下 5 层，9,000 页、1,000 个附件，每页 20 条不带路径的链接指向这次导入的别的页、都已解析）。之后经 REST 在根下新建一页 `top`，SQL 把 `p0` 的父、与导入的变更集里 `p0` 那一项的 `after_parent_id` 改成 `top`（同导入在 `top` 之下）。移动、改名、删除的都是 `top`：同 B4 的 `vault`，没有链接指向它（审查 C 的 M1：`importOf` 有 20 页链着 `p0`，改名 `p0` 要改写它们，`range_start` 又对不上正文，审查 C 的探针里每次改名错写 2 页，B 也不用）。开头断言 `page_links` 里没有指向 `top` 的行，`top` 的子树 10,001 个节点（键与目标约 2 万个，远过门槛 4,000）。
- **操作**（都经 REST，每次核对答复）与断言：
  1. 没有统计时把 `top` 移到根下新建的一页 `q` 之下，一次，再移回；
  2. `bench.analyze(t)`；
  3. 改 `top` 的名，在两个名字之间来回三次，取最小（`fastestOf`，`:916`）；
  4. 移动 `top` 到 `q` 之下、移回、再移去，三次取最小；
  5. 删除 `top`，一次；之后 `checkPages(t, bench.pool)`，索引里没有 `top` 子树写的行。
  - 每一次移动、改名：`PlanCacheMode` 1 次、`LinksReachedWide` 恰 1 次、`LinksReached` 与 `LinksReachedWideGeneric` 0 次（A 与 B 都在）；前后 `page_revisions` 的行数不变、没有页的 `revision` 过 1（没有改写，B 用得上）。
  - 删除：`PlanCacheMode` 1 次、`LinksReachedWide` 1 次、`LinksReached` 0 次（参与者不读，观察者读一次）。
- **反向对照**（6.2）：撤回 A，移动、改名是 `LinksReached` 1 次（B 仍在，观察者用参与者的）、宽读 0 次，删除 `LinksReached` 1 次；撤回 B，移动、改名宽读与 `PlanCacheMode` 各 2 次；都撤是 `LinksReached` 2 次。都确定地红，与用时无关。审查 C 的"撤回 A 是 2 次"是 A、B 都撤时的数。
- **界**：计时只兜期限。每种操作的界 = max（本机两轮均值 × 3.6，本机两轮均值 + 2 秒）（审查 C 第 2 轮的 M1，主持者的第二次修订说明）。3.6 倍是"本机三倍"与"本机 × 2.4 × 1.25 × 1.2"中较大的（后者总是较大；审查 C 的 M2，主持者第一次修订说明的第 3 条）；绝对的下限 2 秒的依据：
  - 这些操作在 1 万个节点上是亚秒的：run6 只有 A 时本机删除 0.56–0.58 秒、无统计的移动 0.78–0.84 秒，A 加 B 的移动、改名推断 0.6–0.8 秒，3.6 倍只有 2.0–2.9 秒；
  - 请求一路的守卫在 7763 上到过本机的约 3 倍，多出的绝对量至多约 1.5 秒（`B/background_cost_test.go:923-936`，决定 249：0.3 秒的请求 760–875 ms，0.7 秒的 1.71–2.16 秒）；
  - 删除与无统计的移动各只有一个样本（删除每做一次再恢复，下一次就因历史变慢，1.2；无统计只有一次），亚秒级上一次 GC 或检查点就能吃掉几百毫秒。

  所以两项取较大：删除 max(0.57 × 3.6, 0.57 + 2) 约 2.6 秒，移动、改名约 2.6–2.9 秒（推断），对 7763 上推断的 1.7–2.4 秒余约 20–50%，又远在 15 秒的期限之下。有结构的断言守回退，界宽不丢什么；实施时量、写进测试的注释与第 10 节。
  - 推断：run6 在近似的夹具上（1 万个节点、链接打散、最上层没人链着；调查的宽读原型只有 A、没有 B，门槛 1,000）两轮本机：无统计的移动 0.78–0.84 秒，改名三次的最小 0.79–0.80，移动三次的最小 0.79–0.84，删除 0.56–0.58；今天的读 0.91–0.95、0.95–0.96、0.94–0.95、0.64–0.65，只慢 1.12–1.20 倍。A 加 B 的移动、改名推断约 0.6–0.8 秒。
  - 合并之前看分支 CI 上这一行的 CPU 与读数，落在 7763 上的才算对照；没落上就照比推断，写"推断"（审查 C 的 M2）。
- **它守什么**（审查 C 的 I1 第 3 条）：经整个程序的路——组合根把备忘交给链接、`Store.Links` 走哪条读——与期限的兜底；不守 B4 的余量。这个夹具比 vault 便宜得多（1 万个节点今天的代码本机约 1 秒；45,000 个约 5.3 秒，B4 的 vault 6 万个节点 13.6–14.8 秒），缩小规模本身去掉了 B4 那份代价（键那一支 50,099 次查找、`page_links` 410 MB 大过 128 MB 的共享缓冲），计时复现不出 B4；B4 由 6.3 量。
- 宽读也让现有的"最大的一步"变快：它删一个 `MaxJobItems`（S6 之后 45,000）个节点的最上层，最近十次 `main` 上 Xeon 6975P-C 七次 7.45–8.03 秒、EPYC 9V74 两次 8.30、10.45 秒、7763 一次 10.95 秒（run 44575718826），界 14 秒；S7 之后照记，写进第 10 节，不改它的界。

### 4.5 进哪一份，加多少秒（决定 660；审查 C 的 M3）

`main` 上 36 次成功的运行（`gh` 只读，`~/.cache/m12-p4s7/ci/jobs-all.tsv`、`cpu-all.tsv`；审查 C 的 11 次加更早的 25 次），7763 上的作业时间：

| 份 | 7763 上的次数 | 作业（秒） | 时限 / 约两倍的线（分钟） | 线下余 |
|---|---|---|---|---|
| `costs-1` | 5 | 1,291–1,330（21.5–22.2 分钟） | 45 / 22.5 | 0.3–1.0 分钟 |
| `costs-2` | 6 | 1,481–1,536（24.7–25.6） | 45 / 22.5 | 已过 2.2–3.1 分钟 |
| `costs-3` | 9 | 1,055–1,133（17.6–18.9） | 40 / 20 | 1.1–2.4 分钟 |
| `costs-lint` | 5 | 775–804（12.9–13.4） | 25 / 12.5 | 已过 |
| `guards-core` | 6 | 1,187–1,250（19.8–20.8） | 40 / 20 | 已到线 |
| `guards-sources` | 3 | 998–1,039（16.6–17.3） | 30 / 15 | 已过 |
| `race` | 5 | 901–907（15.0–15.1） | 20（每个包 `-timeout 15m`） | `L/adapter/postgres` 包 109 秒 |

- **4.4 的守卫 → `test-go-costs-3` 末尾一行**，`-v`。run6 的一个子测试连建库、SQL 导入（约 1.75 秒）在内 10.2–12.7 秒，推断守卫本机约 10–13 秒，乘这一份的比 2.4 约 25–30 秒，7763 上这一份约 18.1–19.4 分钟，仍在线下。`costs-3` 已编译不带竞态的组合根（`TestIngestMigrationCostsLongHistory` 等），不另加编译。实施时量本机两轮，本机均值取整个 `Makefile` 那一行的墙钟（新的 `go test` 进程要起 testcontainers 的容器、迁移模板库，不只是子测试的用时；run6 的 10.2–12.7 秒是子测试的；审查 C 第 2 轮的 N2，它量过暖缓存下组合根一行的编译与链接约 0.76 秒）：若"1,133 秒 + 本机均值 × 2.4"过了 1,200 秒，照先例把 `costs-3` 的时限从 40 分钟提到 45 分钟，写进注释。
- **4.1 第 2 条与 4.2 → `test-go-costs-2`**，在 `TestLinksReachedDenseReadPlans` 一行之后各一行，`-v`。推断本机约 60 + 40 秒（同样取整行的墙钟），7763 上约 4 分钟，`costs-2` 到约 29.6 分钟。七份里放得下的只有 `costs-3`，它放了守卫之后不够；所以照先例（`ci.yml:99-101` 的 35 → 45）改 `costs-2` 的时限：取"2 ×（1,536 秒 + 两行本机均值之和 × 2.4）"向上取到 5 分钟的整数，推断 60 分钟（2 × 29.6 = 59.2）；每行自己的 `go test` 带 Go 默认的 10 分钟，一行的 7763 估计过 5 分钟就加 `-timeout 15m`（M9/P2 设计 6.5）。注释写这两行、各自本机两轮的秒数与新时限的算法。
- **不加 build tag 的**（4.1 第 1、3、4、5 条，4.3 各条）进 `race` 与 `short` 等现有的份：4.1 的几条各约一秒（推断），4.3 是替身上的，毫秒级；4.3 第 6 条多跑参与者，竞态检测下推断多几秒。`race` 在 7763 上 15.0–15.1 分钟，按包 `-timeout 15m` 的 `L/adapter/postgres` 包 109 秒，不改。
- `ci.yml` 的注释与 `costs-2` 的时限在第 1 个提交，`costs-3` 的那一行与注释在第 3 个提交（审查 C 的 N4）。合并之前看分支 CI 上这几份的 CPU 与作业时间，落在 7763 上的写进第 10 节。

## 5. 实施的次序与提交

### 5.1 提交

1. `server: a reach of many keys and targets reads the links it meets in one pass (M12/P4/S7)`：3.1 的两条宽读与读设置的一条、`gen`、两个转发的文件、`Store.Links`、archtest 的规则与用例；4.1 的各条、4.2；`Makefile` 的 `test-go-costs-2` 两行，`ci.yml` 记它们的注释与 `costs-2` 的时限（4.5）。
2. `server: a rename or a move reads and resolves the links it reaches once (M12/P4/S7)`：3.2 的 `UnitMemo`、`Write.Memo`、两个适配器、参与者与观察者、`Index.Logger` 与 `NewIndex`、注释；4.3。
3. `server: a guard of a large top's move, rename and deletion, by the statements they read (M12/P4/S7)`：`newTrashBenchOf` 的 tracer、4.4，`Makefile` 的 `test-go-costs-3` 一行与 `ci.yml` 的注释。
4. `tools: perf ops waits for its notebook's background only, under the editors' load (M12/P4/S7)`：`perf ops -quiet notebook`（6.3 第 5 条）与它的测试（`client` 的语句与 `drive` 的选择各一格；默认 `-quiet all` 不变）。
5. `docs: S7's review record, the tree writes measured again, and the designs as built (M12/P4/S7)`：6.3 的表写进本文第 10 节与 P4 设计 10.3；第 8 节其余的文字；在代码审查之后（02-workflow 3.5）。

每个代码提交单独为绿：`make lint`、`make generate` 之后没有差、`make test-go-short`，加下面相关的；重的经 `heavy.sh`；全量交给分支 CI（M12 11-19 的规则：推送前只跑相关的门禁；审查 C 的 N4）：

- 第 1 个：`L/adapter/postgres` 的包（带 `-race`，同 `race` 一份）与 `costs-2` 的两行（本机两轮，写出秒数）；archtest 的包；它改了所有大到达的读，所以组合根里走大到达的几格各跑一次：3.1 末列的 `guards-core` 五个与 `guards-sources` 三个守卫、`TestTheIndexIsItsRebuild`、LLM Wiki 里删除起导入的测试（`TestIngest` 一族里删除的那几格，实施时 `grep` 列出）。
- 第 2 个：`L/app`、`P/app` 的包，带 `-race`（备忘在参与者与观察者之间共享）；组合根里改名、移动与事件的测试（`TestTheIndexIsItsRebuild` 与实施时按名字列出的几个），带 `-race`。
- 第 3 个：新的守卫两轮（定界）与 6.2 第 1–3 条的反向对照。第 3 个的结构断言要第 1、2 个在，所以排在它们之后。
- 第 4 个：`server/tools` 的测试；与前三个无关，可以先做。

### 5.2 与别的 Step 的关系

- **P4/S6**（导入上限降到 45,000，P4 设计 3.12）：次序写死 S6 → S7（决定 665）。S6 改 `MaxJobItems`、`config.yaml` 的 `import_max_entries` 的注释、`perf` 的 `vault.ImportLimits()` 与导入上限的 vault 形状，并加 `perf vault -part k/n`（主持者第一次修订说明的第 5 条；实施说明由主持者写）。S7 的代码不碰这些文件；`perf ops` 的开关改 `tools/perf/main.go` 的 `opsCommand`，S6 改同一个文件的 `vault` 一段，合并时 `git` 多半自己合得上，冲突了照 S6 的为准。S7 的守卫（1 万个节点）与 `MaxJobItems` 无关。`-part` 的测试给主持者写 S6 的说明时参考（审查 C 的 N3，原本 S7 4.5 的）：两份的条目合起来与整个 vault 的逐个相同（路径、字节），没有重复；`-part 1/1` 与不带 `-part` 的 zip 的 sha256 相同；每一份在 `ImportLimits()` 之内；格式错（`0/2`、`3/2`、`1/0`）报错；变异"漏掉一个第一层文件夹""各份自己从头生成随机序列"各红在前两条。
- **P5/S3**（部署文档的硬件下限）：用 S7 改后的 B4 的数与 × 1.25 一行（P4 设计 3.9 末、F.md 的"另记"）。
- **P2 的各 Step、P4/S4、S5**：不碰这些文件。
- **`main` 上的组合根八份**：S7 不加竞态的组合根测试，`TestTheIndexIsItsRebuild` 多数两个数；合并之后两次 `main` 照 00 号文档第 3 节第 11 条看。

## 6. 测试与验证

### 6.1 变异表

| 变异 | 改了什么 | 红在 | 读数 |
|---|---|---|---|
| a1 | `Store.Links` 总读 `LinksReached`（撤回 A） | 4.1 第 3 条；4.4 的结构断言（`LinksReached` 1、宽读 0） | |
| a2 | 门槛差一：`>` 代 `>=` | 4.1 第 3 条正好 4,000 的几格 | |
| a3 | 门槛只数键 / 只数来源 / 三者相加 | 4.1 第 3 条"只有目标" / 4,000 的几格（来源 10 个）/ 3,999 那一格（来源 1 个） | |
| a3b | 门槛降到 100（或任何 3,999 以下） | 4.1 第 3 条 3,999 那一格（M10 的计划测试至多 3 个，抓不到；审查 C 的 M6） | |
| a4 | 不读设置，总读 `LinksReachedWide` | 4.1 第 3 条的 `SET LOCAL` 与 `RuntimeParams` 两格；4.2 的 `force_generic_plan` 各格（参数数组的过滤） | |
| a5 | 不读设置，总读 `LinksReachedWideGeneric` | 4.1 第 3 条的 `auto`；4.2 的 `work_mem = 64kB` 两格（子计划不哈希） | |
| a6 | 设置比成 `!= "force_custom_plan"`（`auto` 下读 `Generic`） | 4.1 第 3 条的 `auto`；4.2 的 `auto` 两格 | |
| a7 | 两条宽读的第二支去掉对来源的排除 | 4.1 第 1、2 条（来源的行出现两次） | |
| a8 | 两条宽读去掉别名键 | 4.1 第 1、2 条 | |
| a9 | 两条宽读的第二支不限本笔记本（`JOIN` 的条件去掉 `notebook_id`） | 4.1 第 1、2 条（另一本的行）；4.2（读了另一本） | |
| a10 | 两条宽读去掉最外的 `ORDER BY` | 4.1 第 2 条（按序比较） | |
| a11 | `LinksReachedWide` 的排除换成子查询（1.3 的 v3） | 4.2 的 `work_mem = 64kB` 两格（子计划不哈希） | |
| a12 | `LinksReachedWideGeneric` 的成员换回 `= ANY`（1.3 的 v3） | 4.2 的 `no_statistics` × `force_generic_plan`（参数数组的过滤，`EXPLAIN` 就红；1.3：687 个节点 7.9 秒） | |
| a13 | `LinksReachedWideGeneric` 的排除换成 `IS NOT TRUE` 的 `= ANY` | 4.2 的 `force_generic_plan` 各格（`EXPLAIN` 就红） | |
| a14 | `LinksReachedWideGeneric` 的子查询引用外层的行（写成相关的，不再哈希） | 4.2 的 `force_generic_plan` 各格（子计划不是 `hashed`） | |
| a15 | 宽读的子查询去掉 `COLLATE "C"` | 等价：比较取列的隐式 `"C"`（3.1；审查 A 的 N3） | 等价 |
| a16 | `LinksReachedWideGeneric` 的子查询直接写参数（`unnest($4)`，不经 `args`） | 4.2 的"排除是 `hashed` 的子计划，不是 `Anti Join`"（审查 A 的探针：无统计 `Hash Anti Join`、有统计 `Merge Anti Join`；审查 A 的 N2） | |
| a17 | 设置经池读（不在单元的事务与连接上） | 4.1 第 3 条的 `SET LOCAL` 那一格 | |
| a18 | 宽读经缓存的查询器发（不经 `Planned`） | 4.1 第 4 条（`CachedStatements`）；4.1 第 3 条的 `LeadingOptions` | |
| a19 | 转发函数里多一次调用，或改用局部的语句变量 | archtest 的用例（`local_sql_variable` 等现有的格，两个新文件各一组；写明即可） | |
| b1 | 去掉参与者的条件 1（树单元） | 4.3 第 2 条"不是树单元" | |
| b2 | 去掉条件 2 的布尔 | 4.3 第 2 条"只有移动、改名的 batch" | |
| b3 | 去掉条件 3 | 等价：`relocates` 为假时观察者在看备忘之前返回（3.2；审查 B 的 M1），只多存一份没人取的小备忘 | 等价 |
| b4 | 去掉条件 4 | 4.3 第 2 条"变化不同"的各格（替身直接造）。生产里今天没有条件 4 起作用的单元：参与者写页之前丢掉备忘，batch 由布尔挡掉，撤销的单元 `UpdateLinks` 为假（`revert_changeset.go:90-91`），MCP 的 `relocate_path` 只有一步；所以组合根的 `TestTheIndexIsItsRebuild` 照绿，不写它（审查 B 第 2 轮的 N1） | |
| b5 | 条件 4 只比个数 | 4.3 第 2 条"同个数、之后的名不同" | |
| b6 | 条件 4 不比版本 | 4.3 第 2 条"版本不同"（替身直接造；生产里版本不同只来自参与者改写被移动的页自己，那时它已丢掉备忘） | |
| b7 | 观察者不用备忘（撤回 B） | 4.3 第 1 条（`Store.Links` 两次）；4.3 第 8 条（数到 0）；4.4 的结构断言（宽读 2 次） | |
| b8 | 观察者用备忘时 `changed` 由参与者的过滤之后的链接算 | 等价：条件 2 之下过滤不去掉任何一行（审查 B 的 N5） | 等价 |
| b9a | 链接一侧把备忘放在包级的变量里 | 4.3 第 4 条 | |
| b9b | `Writer` 的几个单元共用一份备忘 | 4.3 第 7 条（`P/app`） | |
| b10 | B 的一支不调 `SetResolutions`；或 `changed` 恒空；或 `changed` 恒为全部 | 4.3 第 1 条（非空、与关掉 B 相同）；4.3 第 8 条的第二个下限 | |
| b11 | 参与者写页之前不丢掉它的那一项 | 4.3 第 3 条 | |
| b12 | 观察者只读不取 | 4.3 第 3 条 | |
| b13 | `UnitMemo` 的方法对 nil 接收者不安全 | 4.3 第 2 条"没带备忘"、第 7 条、第 8 条末格 | |
| g1 | 守卫的最上层被一页链着（审查 C 的 M1 的形状） | 4.4 的"`page_revisions` 不变"与"宽读恰 1 次"（参与者改写、丢掉备忘，观察者再读） | |
| g2 | 守卫的规模缩到门槛之下（例如 1,000 个节点） | 4.4 开头"子树至少……"与结构断言 | |

实施时每条先绿后红，读数写进第 10 节；等价的写明依据。

### 6.2 反向对照

1. **撤回 A**（`Store.Links` 只用窄读）：4.1 第 3 条红；4.4 的结构断言红（移动、改名 `LinksReached` 1 次、宽读 0 次，删除 `LinksReached` 1 次）；用时写下。
2. **撤回 B**：4.3 第 1、8 条红；4.4 移动、改名宽读与 `PlanCacheMode` 各 2 次，红；用时写下。
3. **A、B 都撤**：4.4 移动、改名 `LinksReached` 2 次，红；用时写下（run6 推断今天的读只慢约 1.15–1.2 倍，计时红不了，也不靠它）。
4. **`LinksReachedWideGeneric` 换成 v3**（排除是反连接，键仍是参数数组的 `= ANY`）：4.2 的 `no_statistics` × `force_generic_plan` 在 `EXPLAIN` 一步就红（1.3：687 个节点 7.9 秒，6,871 个过 60 秒）。
5. **`LinksReachedWideGeneric` 换成 v2**：4.2 的 `force_generic_plan` 各格红。
6. **`LinksReachedWide` 换成 v4h**：4.2 的 `work_mem = 64kB` 两格红（子计划不是 `hashed`，同 1.3 加垫的 `vault`）。有统计的格它读整本笔记本（1.3：687 个节点 529 ms），但这个夹具的到达本来就占本笔记本的大半，物理工作量不一定红，不靠它（审查 C 的 N3）。

### 6.3 在原规模上再量

1. **搭法**（决定 658）：
   - **zip**：用 S6 合并之后的 `perf vault`，照 S3 的参数（`-seed 1201` 与 F.md 的命令里其余的参数，逐个写出，不依赖 S6 改过的默认形状）写出 `-part 1/2`、`-part 2/2` 两份，一次写好，记两份的 sha256；改前、改后与编辑者负载的各轮都用这一对（审查 C 的 M7）。
   - 每轮 `stack.sh up`，空闲 130 秒；两次 `perf transfer -import` 依次导到同一页 `vault` 之下，每次等搜索的队列清空。
   - **核对**（审查 C 的 M8）：`vault` 的子树节点数、链接数、没解析的链接数、"被链得最多的页"的名字与被几页链着（S3 是 59 页），都等于一次导入的。一次导入的参照数从调查留下的 `run1/vault.dump`（同一个种子、一次导入）在自己的容器里算一次，写进第 10 节。
   - 每轮开量之前记 `page_links`、`nodes`、`changeset_items`、`page_revisions` 的 `n_live_tup`、`n_dead_tup` 与最后一次 autovacuum、autoanalyze 的时刻，改前、改后并列。
2. **操作**：照 P4 设计 3.6 的 B4（`perf ops`，四种各 3 次，每次之前等后台安静；删除之后经后台的恢复）；另删 `f1` 3 次只记。**每次删除记下是这个库上的第几次删除**。`fsync=off` 的一轮：新库，同样的搭法，删除 3 次（第 1、2、3 次，各经恢复），取中位（决定 657）。
3. **改前与改后，交错着跑**（决定 659；审查 C 的 M7）：
   - **两个提交**（审查 A 第 2 轮的 M1）：改后取量那一刻 S7 分支的头 `H`；改前取它的合并基点 `B = git merge-base H main`，即 S7 分支最后一次合进来的那个 `main` 提交（S7 没合过 `main` 时就是开分支时、S6 合并之后的 `main`）。核对：`git merge-base --is-ancestor <S6 的合并> B` 为真（改前含 S6）；`git log --first-parent B..H` 只有 S7 的提交（S7 把 `main` 合进来的合并提交除外，它的另一边就是 `B` 之前的），`git diff --stat B H` 只有 S7 的文件。所以改前、改后只差 S7。两个提交号写进第 10 节。代码审查的修复若又合了 `main`，`B` 随之换成新的合并基点，改前也重量。
   - **换 serve**（审查 C 第 2 轮的 N3）：两个分离的 worktree 各 `make build`，每轮以 `stack.sh` 的 `$NERVEWIKI`（`tools/perf/stack.sh:26`）指向那一边的 `bin/nervewiki`，每轮开头记 `nervewiki version`（带提交与 `modified=false`）。`perf` 用 `H` 编一次（含 S6 的 `-part` 与 S7 的 `-quiet notebook`），各轮共用，写明提交。
   - 次序：改前 r1、改后 r1、改前 r2、改后 r2；`fsync=off` 两边在同一段里、紧接着跑（改前一轮、改后一轮）。同一台机器，改前、改后之间不改设施。
   - 写出每轮每种操作的三次、中位、主行与 × 1.25 一行，改前对改后。
   - 代码审查的修复改到读、备忘或观察者时，重量改后（两轮加 `fsync=off`），并与一轮改前交错着跑，看漂移。
4. **负载与采样**（审查 C 的 M9；S3 审查 B-M2 的处置；05-lessons 第 9 节末条）：
   - 每轮开跑前记 `uptime` 与锁外的进程；
   - 量的那一段里一个后台的记录器每 5 秒写一行：`/proc/loadavg` 与 CPU 前五名的进程（`ps -eo pid,pcpu,comm --sort=-pcpu | head -6`）；与锁外负载重叠的轮作废重量；
   - 每轮带 `perf sample -pid <serve> -container <pg 的容器>`（P4 设计 3.6 B4 的采样），服务的 `VmHWM` 由它来（3.2 的内存）。
5. **编辑者负载下**（决定 661；审查 C 的 M10）：改前、改后各一次，各约 25 分钟，在交错的四轮之后：
   - 另起一个库，同样的搭法，加 `perf editors` 要的那本笔记本 `editors`（照 P4 设计 3.6 的编辑者负载的准备）。
   - 后台起 `perf editors -n 20 -for 25m -container … -monitor …`。它开头建页、`VACUUM FULL page_revisions`（`drive/editors.go:48-50`，持 ACCESS EXCLUSIVE），结束时再来一次；所以等 monitor 的文件里出现编辑者保存的行、开头的 `VACUUM FULL` 过了（约 1 分钟），再起 `ops`。
   - `perf ops -reps 1 -quiet notebook -idle 30s`：`-quiet notebook` 是 S7 的第 4 个提交加的，只等 `vault` 那本笔记本的 `page_search_queue` 为空、River 里 `page.index_search` 之外没有在跑与可跑的任务。搜索的任务是全局的、不带参数（`page/adapter/river/search.go:36`），编辑者的保存一直往队列里排，照现在的全局安静等不到。恢复照旧由 `awaitJob` 等到成功。
   - `-for` 盖住 `ops` 的全程：两次删除各要后台恢复（S3 量过一次 156 秒）与恢复之后 `vault` 的重建搜索，加五次空闲，推断 10–15 分钟，取 25 分钟。`ops` 的最后一行要早于编辑者结束、早于它最后的 `VACUUM FULL`，不然这一次作废、加大 `-for` 重来。
   - 编辑者在另一本笔记本里，只争 CPU、IO、WAL、autovacuum 与连接，不争 `vault` 的笔记本行：这是要的。B4 问的是操作本身的大小到没到期限；同一本笔记本的保存排在树写之后，是 v0.1 设计第 14 节"同一笔记本的保存串行"一行的已知限制，另一回事。
   - 只记（P4 设计 3.6 的"只记"）：答复与用时、编辑者的 monitor 与每个版本的字节、负载的记录；改前、改后并排（05-lessons 第 9 节）。
6. **病态形状**（决定 662；审查 C 的 M11、N5）：用改后的数换算到 7763，标推断。
   - 20 万个节点的最上层的删除：按节点数线性。`f1` 那一点是在 `vault` 删除、恢复三次之后删的，它的节点已带三次的历史，偏高、外推的斜率偏低；改用位置相同的两点：`fsync=off` 那一轮新库上 `vault` 的第 1 次删除，与另一个新库上 `f1` 的第 1 次删除（多一个新库，约 15 分钟，只在 `fsync=off` 下）。换成主行：两点各 × R，取过这两点的直线在 20 万个节点处的值，不加差（两点都是 `fsync=off` 的第 1 次，没有同位置的本轮中位可减），标推断（审查 C 第 2 轮的 N5）。
   - 20 万个节点的到达约 60 万个元素，过了 1.3 第 6 条在约 51 万个时就因 64 MB 的 `/dev/shm` 报错的大小：这一形状改后可能先报错、而不是超时，换算处与第 8 节第 6 条写一句。
   - 400 页 × 2,000 条链接的 `move` 与被 2,500 页链着的页的改名：S7 不改它们的读与写（到达在门槛之下，A 不碰；参与者改写页，B 不用），照 F.md 的换算。
7. **别的走到宽读的单元**（3.1 末；审查 A 的 M2）：`guards-core` 的五个（恢复在单元的上限、撤销一次删除与撤销多个最上层、后台的最大的一步、batch 的改写的界）与 `guards-sources` 的三个（来源的移动、恢复、撤销，三种阶段），改前（第 3 条的合并基点 `B`）与改后（`H`）交错着各跑两轮：改前、改后、改前、改后（同样的漂移理由；审查 C 第 2 轮的 N3），`-v` 打出的用时写进第 10 节；变慢的列给主持者。
8. **另记**：`serve.log` 的 ERROR、WARN；每轮的 `pg_stat_statements` 不要求（`stack.sh` 不带它）。
9. **判**：2.1。删除的主行没过而 × 1.25 一行过 15 秒，列给主持者。哪一行的主行仍过 15 秒，照 P4 设计 3.9 第 2 条复核两轮，仍过就停下列给主持者（决定 663）。

## 7. 完成标准

- 6.3：删除、移动、改名 `vault` 的主行推到 7763 上 ≤ 15 秒；改被链得最多的页与删 `f1` 不退步（2.1）；× 1.25 一行与编辑者负载下的改前、改后写进第 10 节，列给主持者。
- 第 4 节的各条在；6.1 的变异逐条抓住或写明等价；6.2 做过，写进第 10 节；各新测试本机两轮的秒数写进第 10 节。
- 6.3 第 7 条的八个守卫改后不比改前慢，或慢的已列给主持者。
- `make lint`、`make generate` 之后没有差；相关的本机跑过（5.1），全量交给分支 CI，CI 绿；`costs-2`、`costs-3` 加了之后的作业时间（落在 7763 上的）写进第 10 节，`ci.yml` 的注释与时限照 4.5；合并之后两次 `main` 的组合根八份照 00 号文档第 3 节第 11 条。
- 第 8 节的文字改了。

## 8. 文档的新文字

每一条写成"旧文字 → 新文字"，文档提交的脚本断言每段旧文字恰好出现一次（02-workflow 3.5）。旧文字的锚点取 S3、S6 都合并之后的 `main`（S3 的文档提交改过 P4 设计 3.4.3、3.6 B4；S6 改导入上限的文字；审查 C 的 N6），实施时重核。带〈〉的随 6.3 与实施填。

1. **v0.1 总体设计 13.4 第 9 条**（查询计划的测试）：
   - `每个写单元读到的链接（\`LinksReached\`，改名时 75–83 对 16–26 ms）（FA5-Q1）；` → `每个写单元读到的链接（\`LinksReached\`，改名时 75–83 对 16–26 ms）（FA5-Q1；键与目标合起来至少 4,000 个的到达另读宽读，一次按页取：平常读 \`LinksReachedWide\`，常量数组的 \`= ANY\` 由 PostgreSQL 哈希、不看 \`work_mem\`；设了 \`force_generic_plan\` 时读 \`LinksReachedWideGeneric\`，成员与排除都是对 \`unnest\` 的子查询、通用计划里都哈希；三种统计与三种计划方式下的计划与物理工作量由 \`TestLinksReachedWideReadPlans\` 守，M12/P4/S7）；`
2. **13.4 第 10 条**（派生数据等于重建）：
   - `经整个程序的是 \`bootstrap/links_rebuild_test.go\` 的 \`TestTheIndexIsItsRebuild\`（含改名、移动的改写）` → `经整个程序的是 \`bootstrap/links_rebuild_test.go\` 的 \`TestTheIndexIsItsRebuild\`（含改名、移动的改写；M12/P4/S7 起单个改名、移动的单元里观察者用参与者读到的链接与解析，这个测试数这样的单元与其中链接的去处变了的单元，下限〈各为实测的三分之二〉）`
3. **13.4 第 13 条**（两处实现同一条规则）：在 `以后两处实现同一条规则（规整、折叠、分词）时照此` 之前插入 `链接索引按到达的大小与计划方式有三条读（\`LinksReached\`、\`LinksReachedWide\`、\`LinksReachedWideGeneric\`），\`TestLinksReachedWideIsLinksReached\` 以随机的夹具与到达核对三者逐行相同（M12/P4/S7）。`
4. **P4 设计**：
   - 10.3 的 B4 之后加"B4，S7 之后"的表（6.3）；
   - 3.6 的 B4 的"规模与形状"一句之后加：`P4/S6 之后由两次导入搭成（\`perf vault -part\`，P4/S6；量法见 P4/S7 设计 6.3）。`（S6 已写了同样意思的就不加；`-part` 在 3.4.3 的文字归 S6。）
5. **`links.sql`、`store.go`、两个转发的文件、`rewrite.go`、`index.go`、`extension.go` 的注释**：3.4，在代码的提交里落。
6. **移交 M4-performance**（`docs/v0.1/M12-release/handoffs/M4-performance.md`，在 S7 的文档提交里做；审查 C 的 M11、M12）：
   - 第 16、18 项（20 万个节点的最上层的删除、400 页 × 2,000 条链接的 `move`）的"处理进展"：S7 改了什么、改后换算到 7763 的规模（6.3 第 6 条），20 万个节点的可能先因共享内存报错，仍是已知限制、迁移进 v0.2；
   - 第 14 项"删除没有历史的界线"一条（`:374`）的"处理进展"：S7 量到的第 1、2、3 次删除与〈n〉（第 9 节第 1 条）；
   - 被 2,500 页链着的页的改名：S7 不改，去处照 v0.1 设计第 14 节"同一笔记本的保存串行"一行写一句。
7. **00 号文档**：
   - **设计提交时**（主持者随本设计提交）：第 7 节加一条（号接在已有的之后）：`**13.4 第 9、10、13 条**，随 P4/S7 改：第 9 条加宽读与它的计划测试，第 10 条加 B 与数法，第 13 条加两条读的逐行对照。新文字在 P4/S7 设计第 8 节第 1–3 条。`；头的状态写 P4/S7 的设计已审查；第 11 节加本设计的决定；第 12 节的 P4 一行写条件的 S7 设计已审查，"审查"一格加 S7 设计的审查记录；第 13 节加一行。
   - **S7 的文档提交时**：第 8 节（M12 改到的前面 M 的代码）加两项：`**链接（M6、M10）**：键与目标多的到达另读宽读（两条，按计划方式选）；单个改名、移动的单元里观察者用参与者读到的链接与解析（P4/S7）`、`**页（M4）**：写单元的 \`Write\` 带一份单元的备忘（P4/S7）`；4.7.1 的 B4 一行的"处置"一格补"P4/S7 修了，改后〈主行〉"；第 12 节的 P4 一行写 S7 已合并；第 13 节加一行：S7 的结论一句。
8. **v0.1 总体设计第 15 节**加一行：`2026-11-xx：M12/P4/S7：13.4 第 9、10、13 条加链接索引的宽读、一个单元只读一次的条件与两条读的逐行对照。依据 [P4/S7 设计](M12-release/04-P4-S7-tree-writes.md) 10 与 [S7 记录](M12-release/reviews/P4-S7-tree-writes-review.md)。`
9. **S7 的审查记录**：设计审查的是 `reviews/P4-S7-design-review.md`（主持者随设计提交，含三份审查、本次修订与修订核对），实施的是 `reviews/P4-S7-tree-writes-review.md`（S7 的文档提交）。

## 9. 交给后面的

**已知限制**（收尾时进 v0.1 的已知限制与 `docs/v0.2/handoffs/M12-performance.md`，00 号文档 4.13）：

- **删除的用时随子树的条目与版本数涨**（调查的 E，主持者已定；审查 C 的 M12）：`DeleteNodes` 给子树里每个节点所有活着的条目与版本打 `deleted_at`（1.2）。删除、恢复各一次给每个节点添一行条目；有编辑历史的页每个版本一行，所以有编辑历史的子树第一次删除就一样慢，不只是删过、恢复过的。与已有的两条同一机制：M4-performance 第 14 项的"删除没有历史的界线"（`handoffs/M4-performance.md:374`）与 S3 审查 C-I2 的"一页 100 万行历史的删除，7763 上推断约 19–20 秒"。6 万个节点的最上层改后推到 7763 上〈6.3 的第 1–3 次〉；照 6.3 量到的整个删除的增长推：主行 = `fsync=off` 第 k 次 × R +（本轮中位 − `fsync=off` 中位），解出过 15 秒的 k =〈n〉（推断）。改法候选：条目与版本另记"属于哪一项"的一列、只给最上层打标；或删除不碰历史、读历史时连节点判断（v0.2 定）。
- **移动、改名的期限只在不改写任何页时成立**（决定 666）：改写了页的大子树（例如有页用库内的绝对路径链进来）B 不用，只有 A 的余量：推断主行约 13 秒，在期限之内；× 1.25 约 16 秒，照 3.9 第 3 条列给主持者。
- **病态形状**：照 6.3 第 6 条换算的规模（推断），仍到期限；20 万个节点的可能先因共享内存报错。
- **`LinksReachedExcept` 没有宽读**（决定 655）：LLM Wiki 的笔记本里，一个 batch 同时写准备好的来源、又删除、移动或改名大的子树时走它。删除至多约 1 万个节点，按参数规划推断约 0.2–0.3 秒；移动、改名推不出比笔记本小的界，到达可以是整个最上层，今天的读一次约 3 秒（推断）。
- **宽读的花费随什么**（3.1 末）：无统计时随本笔记本的链接数，一本很大、又没有统计的笔记本里，刚过门槛的到达比今天慢（推断）；有统计时规划器可以整表扫，花费随整张表（今天的读无统计时目标那一支也整表扫，不是新的一类）。
- **今天的 `LinksReached` 在通用计划下对中等以上的到达是平方的**（决定 664）：门槛之下留着，花费随指向到达的键的链接数乘来源数，没有界（vault 的密度下 3,923 个时 269 ms）；只在设了 `force_generic_plan` 时出现。
- **几十万个元素的到达碰到共享内存**（1.3 第 6 条）：约 51 万个键与 id 时，今天的读与 `LinksReachedWide` 按参数规划都选了并行的计划，在 Docker 默认 64 MB 的 `/dev/shm` 上报错（探针的容器；仓库里没有设 `shm_size` 的地方）。这是 20 万个节点以上的病态形状，本来就到期限；部署文档（P5/S3）可以写一句 `shm_size`，或这条读关掉并行，由 v0.2 定。
- **语义上收窄到达**（2.4 甲）：移动一个自成一体的子树仍读、解析子树里的全部链接（6 万个节点的最上层 135 万条），只读一次（B）。
- **F 的两小项**（决定 663）：节点长度每次算（每边约 0.35 秒 CPU）、路径的两条读排序溢出到磁盘（每对约 0.75 秒）。
- **组合根的守卫是缩小规模的**（4.4；审查 C 的 N7）：结构加期限，不量 B4 的余量；B4 的数在 6.3，7763 的读数在第 10 节。

**主持者**：

- × 1.25 一行（6.3）过 15 秒的，列在第 10 节；决定 666的那一行。
- 编辑者负载下的四种操作，改前、改后的读数（6.3 第 5 条）。
- "不退步"复核两轮仍过的（2.1）。
- 部署文档的硬件下限（P5/S3）用 S7 改后的 B4 的数。

## 10. 结果

（实施之后填：6.3 的表与核对；各新测试本机两轮的秒数；6.1 的读数；6.2；4.4 的界与 7763 上的对照；4.5 各份加了之后的作业时间；4.3 第 8 条的两个下限。）
