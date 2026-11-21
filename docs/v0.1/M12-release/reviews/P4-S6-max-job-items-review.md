# M12/P4 S6（`MaxJobItems` 降到 45,000）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-p4-s6`，从 `main` 的 `21f9578d`（S3 的记号提交）开，八个提交（下面"提交"）；实施者的报告、记录、grep 的逐处处置与脚本在设计工作目录的 `m12/p4s6/`（`impl-report.md`、`results.md`、`grep-list.md`、`work/`），原始数据在本机 `~/.cache/m12-p4-s6/`。本提交是 S6 的文档提交。 |
| 规格 | [P4 设计](../04-P4-performance.md) 3.12（S6 的设计，不另写设计文档）、3.6 的 B5、3.9；[00 号文档](../00-M12-design.md) 4.7.5，决定 642（负责人 2026-11-21 第 13 问第 2 题）、658（`-part` 由 S6 加）；[P4/S7 设计](../04-P4-S7-tree-writes.md) 5.2（`-part` 的规格） |
| 审查方式 | 三位审查者并行，各在 `89d9c701` 的分离 worktree 里、经 `heavy.sh` 跑，用完删了：A 常量、守卫与文字，B `server/tools/perf`（导入上限的形状与 `-part`），C 45,000 上的实测、`arrays` 基准的修复、分支 CI 与文档草稿的数（下面"分工"）。两轮修复、两轮核对：核对第 1 轮三位各核自己的原发现并看新的三个提交（含审查开始之后加的第 5 个 `45aa3026`），核对第 2 轮只有 C 核 C2-M1，其余 Nit 由主持者核（负责人第 12 问）。 |
| 日期 | 2026-11-21（实施、量、审查、两轮修复与核对） |
| 结论 | 常量 60,000 → 45,000，导入的节点上限随之；在原规模上两轮（`89d9c701`）每格 ≤ 5,000 ms，至多 3,683 ms，**过**；改前 6,107–7,382 ms。守卫照常量变小、界不动（决定 670）；分支 CI 五次全绿，两次 `guards-core` 落在 7763 上，最大的一步 6.93–6.97 秒，约为界的一半。`perf vault -part k/n` 的两份与 S3 的 zip 逐个条目相同，P4/S7 用它搭 6 万个节点的最上层。审查：A 1M 5N 1Q，B 1M 4N 1Q，C 2M 5N 1Q，没有 Important，主持者全部接受；最要紧的是 `ImportLimits()` 与服务端的常量之间没有测试（A-M1），改成读服务端的文字对照。核对第 1 轮原发现全关，新的 1M 4N 在修复第 2 轮改了。 |

## 提交

| # | 提交 | 说明 | 做了什么 |
|---|---|---|---|
| 1 | `bd7149b0` | server: MaxJobItems, and with it an import's nodes, at 45,000 | 常量与注释（照 S3 的量重写，M8 的数留作历史）、`jobs_test` 断言 45,000；`jobSizes` 的默认档 4、4.5、5 万；文件头的例换成 B5 的命令；`source_plan_cost_test.go` 写死的 `SourceHeads`、`ProbeSources` 各 6 次改成由常量算（不改就红，变异 c1）；`Makefile` 一行注释 |
| 2 | `28fb5844` | api, server, web, docs: an import's nodes and a background revert's items, 45,000 at most | 契约的 `too_many_nodes` 与 `make gen` 的三份生成物、`config.yaml` 的注释、README 三处 |
| 3 | `d544acbe` | tools: perf's vault within an import's 45,000 nodes, and -part k/n writes a larger vault in parts | `ImportLimits()` 的节点 45,000；`Spec.Part`、`ParsePart`、`contentsOf`（写之前的核对拆出来，全尺寸只数不写）、`perf vault -part k/n`；`doc.go`；`perf` 包第一次有测试 |
| 4 | `89d9c701` | server: the background steps' bench of the arrays' statements raises the bound past its 50,000 ids, as the other sizes do | `BenchmarkBackgroundSteps/arrays` 的 5 万个 id 过了新界，B5 的命令整个失败（第一次量撞出来的）；抬界到 `1<<30`，同别的规模 |
| 5 | `45aa3026` | e2e: perf's large shapes import in archives of at most 45,000 nodes, the same pages in the same tree | `shapes.ts` 的包改成 `{zip, under?}`，每包核对节点 ≤ 45,000；`flat50k`、`flat100k` 分包，`folder50k` 第二包以 `parent_id` 导到第一包建的文件夹之下（决定 668）；`setup.ts` 的 `rootPageId`；`jobs.perf.ts` 先导第一包 |
| 6 | `e3566f2e` | tools: perf's import limits read from the server's constant and config, and the whole vault pinned by what it holds | 修复第 1 轮：`TestImportLimitsAreTheServers`（A-M1）；整个 vault 钉内容不钉压缩（B-N1）；`Part` 的注释（B-N2）；`doc.go`（B-N3、B-Q1） |
| 7 | `14808583` | server: MaxJobItems' test lists what follows the constant, and the benches' sizes and the guards' comments follow it | 修复第 1 轮：`jobs_test.go` 的清单（A-M1）、守卫的两段注释（A-N1、A-N2）、`jobSizes` 由常量算（A-N4）、常量的注释加 S6 的两轮（A-N5） |
| 8 | `b447dd28` | server, tools, e2e: the two comments reflowed, and folder50k's first archive one page at least | 修复第 2 轮：两段注释重新折行（A2-N1、B2-N1）；`oneFolder` 的第一包至少一页（B2-N2） |

提交说明都以 `(M12/P4/S6)` 结尾、带 `Co-Authored-By`。grep（`21f9578d`，`60,000|60_000|6 万|6万|MaxJobItems|60000`，一行里每处一条）727 处：改 36、不改 202、文档 489；逐处的理由在 `m12/p4s6/grep-list.md`。不改的主要是三类：只有名字没有数的；与这个上限无关的 6 万（毫秒、限流、id、M10 来源探测的游标 `X=60000`、M8 搜索的量）；历史的设计、记录与审查。文档在本提交照"现行的陈述改、历史的记录不改"处理：现行的陈述改了 P4 设计 3.4.3、3.6 的 B4 与 B5 各一句、2.1 第 5 条、第 1 节的基线、4.9、4.11 第 9 条、3.12 的状态，00 号文档 4.7.1 的 B4、B5 两行与第 3 节第 6 条的证据，总体设计 13.1 第 31 条与第 14 节一行，移交 M4-performance 一条；其余（M1–M11 的设计与记录、各审查记录、00 号文档的决定与第 13 问的原话、P4 设计 3.11、3.12 的正文与 10.3 的记录、P2 设计当时的"一个导入至多 6 万个节点"）不改。grep 之外另找到的四处在实施报告第 8 节第 1、4 条与第 9 节第 1 条（来源计划的块数、`arrays` 基准、`e2e/perf` 的形状）。

## 在原规模上再量

- **被量的提交** `89d9c701`（分支的头，前四个提交；决定 674）。被量的后台代码与 S3 的 `f1317bc3` 相同：`server/internal` 的生产代码只差常量与注释、`server.gen.go` 一行说明；两个基准的界都抬到 `1<<30`，规模由 `NW_JOB_SIZES` 定。
- **机器**：aarch64、8 vCPU、30 GiB，Parallels 虚拟机；交换 3.8 GiB 里 2.5 GiB 已用（照记，没清）；Docker 29.9.2（cgroup v2），Go 1.27.4，PostgreSQL 是 testcontainers 的 `postgres:18.7-trixie`；`TMPDIR`、`GOTMPDIR` 在根分区上 `~/.cache/m12-p4-s6/tmp`。
- **命令**（3.6 的 B5，只差规模；`server/` 下，每轮一次）：`NW_JOB_LINKS=20 NW_JOB_SIZES=45000 go test -count=1 -v -run '^$' -bench '^(BenchmarkBackgroundSteps|BenchmarkBackgroundJobs)$' -benchtime 1x -timeout 3h ./internal/bootstrap/`。
- **锁与负载**：整段经一次 `heavy.sh`，07:30:36–08:27:18；r1 07:30:43–07:58:59，r2 07:59:07–08:27:18，各 rc 0。每 5 秒记 `/proc/loadavg` 与前五名进程（S3 审查 B-M2）：r1 338 个样本、r2 337 个，1 分钟负载至多 2.77、2.71，只有本轮的 `bootstrap.test`、PostgreSQL、`docker-proxy` 与常驻进程，没有锁外的构建或测试（审查 C 逐个样本对过各格的时间窗）。
- **读数**（ms，本机不乘，两轮取较大，> 5,000 为没过）：`top-ms` 无统计 3,419、3,455，有统计 3,683、3,592；`group-ms` 1,578、1,599 与 962、964；深的 `revert-max-batch-ms` 3,495、3,391 与 3,663、3,644；深的 `restore-max-batch-ms` 至多 270；平铺、带文件夹、附件的一批至多 374。**过**，最大 3,683（`top` 有统计，r1），余约 26%；3.9 第 2 条的复核不需要。
- **只记的量与改前并排**（05-lessons 第 9 节；审查 C-M1）：用户删深的最上层（一个请求，15 秒的期限）5,794–5,828 / 8,031–8,129 ms，改前 10,195–10,309 / 13,550–13,997，照 S3 的算法推断 7763 上约 16.6 秒；导入的撤销的任务（秒）平铺 77.8–83.4、深 3.60–3.87、带文件夹 32.7–36.9、附件 62.1–66.3（改前 127.1–143.3、6.52–7.09、52.8–59.1、91.1–158.6）；撤销的撤销 35.1–222.0 秒（改前 53.0–326.9），它的请求 468–731 ms（836–1,427），一批至多 2,655 ms（3,632）；深的那一项的恢复 89.9–93.2 秒（136.7–143.0）；`plans` 293–312 / 106–109 ms（516–584 / 145–158）；与规模无关的 `frontier` 第一批 57–62、共 742–751 ms（65–72、871–935），`arrays` 的 `NodesNow` 296–301 ms（329–363），一页 100 万行历史的删除 7,574–7,637、恢复一批 10,866–11,019 ms（9,363–9,677、13,129–14,023，已知限制）。规模小了四分之一，只记的量小了约三到五成；与规模无关的三项也低 10–20%，同下面"低约 17%"的读法。
- **作废的第一次**（`d544acbe`，06:55:18–07:22:24）：锁外有别人的 `compile`（至多 346%）、`vet`、`link`、`app.test`、`drive.test`（07:01:14–07:19:53），与第一轮重叠，照 05-lessons 第 9 节作废；第一轮又在 `arrays` 失败（第 4 个提交改）。它的数只作对照：`top` 在锁外的负载停了之后，与正式两轮差不到 1%，深的那一批 ≤ 3.5%；落在锁外负载里的几格高 18–68%（附件无统计的一批 391 对 230–233，附件有统计 248 对 157–165，深的有统计的恢复 437 对 251–270，平铺有统计 248 对 204–210）。
- **45,000 与 50,000**（审查 C-Q1，决定 672）：S6 比 S3 在 45,000 上的候选（4,114–4,454，批次 D 开始四到五个半小时之后）低约 17%，代码相同；审查 C 经 `heavy.sh` 在新的一段上各跑一轮只跑 `top`（新进程，S3 候选的写法）：45,000 是 3,612 / 3,748，与 S6 的两轮差 2–6%；50,000 是 3,968 / 4,154（S3 是 4,637–4,667 / 5,070–5,136）。只一轮，不足以判；仍取 45,000，收官时列给负责人。
- **原始数据**：`~/.cache/m12-p4-s6/B5/`（`r1.log`、`r2.log` 每行带 `EPOCHREALTIME`，`r1-load.txt`、`r2-load.txt`，`machine.txt`、`machine-end.txt`，`summary.md`）与 `B5-attempt1/`，各有一份逐个文件的 `sha256.txt`（12 个与 6 个文件；清单自己的 sha256 `1ebc9fb89cae70880dec02b5d0ff0d783ee768733f85fde3eacb59ad4bc6d870`、`ad549206fcbbf8cc4b7c8d8bc3c8288e12b5709e777dc9322b924112b1354a64`），两个目录留着（审查 C-M2，取舍 P4-15）；记录照 3.5.5 在 `m12/p4s6/results.md`；审查 C 的重跑在 `m12/p4s6/review-c-work/rerun/`（带 `sha256.txt`）。

## 变异与对照

- **实施**（`mut.py`，`m12/p4s6/work/mut.json`，`d544acbe`，一次 `heavy.sh`；先在没改的树上把九个命令各跑一遍）：21 条，20 条红（另一条反向对照是第一次量本身）。设计的三条：常量改回 60,000，`jobs_test` 红；`ImportLimits` 改回 60,000，`vault` 的两条测试红；`deps.go` 写死 60,000，`TestTransferDepsLeaveTheBatchesTheImports` 红。`-part` 的十二种、17 条（几种各在两条测试上跑）：漏掉每段第一个、最后一个第一层文件夹；各份自己从头生成随机序列（红在"几份合起来是整个"，在 `1/1` 的那一格上等价，`1/1` 恒等）；`Write` 拿整个 vault 比上限；一份数整个的文件夹；附件不分份；`ParsePart` 收 k > n、k = 0；n 多于第一层的文件夹；命令行读了 `-part` 却不用；段的上界向上取整；按第二层分份。反向对照：来源计划的守卫照旧写死 6 块，红；`arrays` 照旧取组合根的界，第一次量就红在它上面（不在 `mut.py` 里）。
- **修复第 1 轮**（`m12/p4s6/work/mut-fix1.json`，`e3566f2e` 提交之前的工作树）：读文字的测试在 `config.yaml` 的 `import_max_entries` 改成 40000、`import_max_bytes` 改键名、`import_max_unpacked_bytes` 翻倍、`const MaxJobItems int = 45_000`、常量改成 50,000 时都红（读不到或格式变了是失败，不跳过）；**反向对照**：常量与 `jobs_test` 一起改回 60,000，`jobs_test` 绿、读文字的测试红（"ImportLimits() = {… Nodes:45000 …}, the server's {… Nodes:60000 …}"）；审查 B 的"每份另起种子"两格都红；"页倒着写，整个与各份都倒"在"几份合起来是整个"上活着（两边一起倒），红在新的内容的钉上；**正对照**：另一种 deflate（`flate.BestSpeed`）写的 zip 变了（239,972 字节，原来 239,323），内容的钉照样绿。
- **审查者自己的**：A 在 `89d9c701` 上常量改回 60,000 只红 `jobs_test`，与它的测试一起改则全绿（成了 A-M1），`deps.go` 写死 60,000 红；核对第 1 轮再跑五条（同一起改回 60,000、挪走 `jobs.go`、`config.yaml` 行尾加注释、只改解压的数、常量两行），都红。B 跑了自己的九条（每份另起种子、跨份的链接不写、一份的解压按整个的附件或页算、深层文件夹的第一层取错、页倒着写等，倒着写的那条只红在钉上）与实施者在 `perf` 的 18 条（结果同实施）；核对第 1 轮在新的钉上跑五条：页倒序、附件改名、正文少一个字节、每份另起种子都红，`zip.Store` 换 `zip.Deflate` 的正对照绿。

## 分工

- **A：常量、守卫与文字**（`m12/p4s6/review-a.md`）：常量、`deps.go`、三个守卫与界、来源计划的块数、`config.yaml`、README、契约与三份生成物、grep 的完整与"不改"的理由。在 `~/.cache/m12-p4s6-review-a`（`89d9c701`）经一次 `heavy.sh` 跑变异（08:44:11–08:46:52）。
- **B：`server/tools/perf`**（`review-b.md`）：导入上限的新形状与 `-part`；编出分支与 `main` 的 `perf` 三方比 sha256，全尺寸比 S3 留下的 zip（S3 的 vault 两份、三份，批次 E 的两个各两份，比解压之后的字节）；九种 `-part` 写法的报错。在 `~/.cache/m12-p4s6-review-b`（`89d9c701`）经一次 `heavy.sh`（09:08:09–09:10:05）。
- **C：实测、`arrays` 的修复、CI 与草稿的数**（`review-c.md`）：从原始日志重算每一格与每个"只记"的数，负载的每个样本与各格的时间窗对齐，读分支与 `main` 的 CI 作业日志，核草稿的旧文字恰好一次；经 `heavy.sh` 在 45,000、50,000 各重跑一轮 `top`（08:46:59–08:48:31）。

审查 B 在 08:39–08:42 之间在锁外起过一个不睡的等待循环（`read -t 5 < /dev/zero`），约 3 分钟占满一核，发现之后停了；那时持锁的是 P4/S7 的变异，只看红不红（06-environment 第 3 节加了一条）。

## 审查的发现与处置

主持者的处置在 `m12/p4s6/fix-1.md`、`fix-2.md`，全部接受，照审查者的建议改；改在哪写在实施报告第 12、13 节。

| 号 | 级 | 发现 | 处置与修复 |
|---|---|---|---|
| A-M1 | M | `vault.ImportLimits()` 与 `page.MaxJobItems` 之间没有测试：`server/tools` 是另一个模块，常量与它自己的测试一起改时什么都不红 | 第 1 轮两样都做（决定 669）：读服务端文字对照的测试与反向对照（`e3566f2e`）；`jobs_test.go` 列出随常量手改的各处（`14808583`） |
| A-N1 | N | 守卫一段的总注释仍说界"是这里量的三倍" | 第 1 轮：改成 60,000 时的过去时 |
| A-N2 | N | "界不放宽也不收紧"只在数值上成立，对着变小的工作量守卫实际松了 | 第 1 轮：界的注释与 10.4 写本机约 3.5 倍、6975P-C 约 2.7 倍，指向 3.13 |
| A-N3 | N | grep 清单里 P4 设计 66、107、879 行的理由不对，107、879 是不再成立的设计陈述 | 第 1 轮：归"文档，主持者"；本提交改了这三处与同类的 904 |
| A-N4 | N | `jobSizes` 的默认档写死 | 第 1 轮：由常量算 |
| A-N5 | N | 常量的注释没写 S6 的两轮 | 第 1 轮：加一句（S3 的数原样，见 C-Q1） |
| A-Q1 | Q | 两次 `main` 的数之后守卫的界要不要照"本机三倍"重定 | 不重定，写明理由（决定 670，P4 设计 3.13 第 6 条） |
| B-M1 | M | CI 余量的推断把 `perf/vault` 一个包的用时当成了 `costs-3` 那一行的用时 | 第 1 轮：那一行由最慢的 `perf/drive` 决定，79.6 对 79.3 秒不变，7763 推断同样被盖住；S7 的余量只在 `vault` 超过 `drive` 时才算 |
| B-N1 | N | 整个 vault 的 sha256 钉的是 deflate 的输出，升 Go 可能无关地红 | 第 1 轮：钉内容（每个条目的路径与解压之后的字节，取自 `main` 的 `perf`，一段独立的 Python 照同一规则算出） |
| B-N2 | N | `-part` 的区间读不出开闭 | 第 1 轮：注释与草稿写明从 0 数、含前不含后 |
| B-N3 | N | S3 的整个 vault 不再写得出，文字没说 | 第 1 轮：`doc.go` 与 3.4.3 写明 |
| B-N4 | N | 批次 E 分份导入，内容相同、库与历史不同 | 第 1 轮：3.4.3 写明，重跑只与同搭法的比 |
| B-Q1 | Q | 导入上限的新形状要不要让解压也逼近上限 | 照现在的写法，写明不是字节上的最大导入（决定 671） |
| C-M1 | M | "只记"的量没有逐个与改前并排，漏了用户删 45,000 个节点的最上层 | 第 1 轮：记录加改前 / S6 一张表；10.4 写用户删最上层，作 S7 的改前对照 |
| C-M2 | M | 原始数据没有 sha256，打算审查之后删 | 第 1 轮：`B5/`、`B5-attempt1/` 各一份 `sha256.txt`、留着，记录写路径 |
| C-N1–N5 | N | 请求的上端取自作废的一次；"差 ≤ 2%"只对点到的几格成立；改前深的一批写成范围；草稿的占位"10.x"；`-part` 的段的写法 | 第 1 轮：都照改 |
| C-Q1 | Q | S6 比 S3 同代码同规模低约 17%，S3 的 50,000 也在同一条件下量的，要不要写明、补量 | 仍取 45,000，写明事实，不补量，收官时列给负责人（决定 672） |
| A2-N1 | N | `jobs.go` 的注释插句之后断了一行 | 第 2 轮：重排 |
| B2-N1 | N | `Part` 的注释一行 106 个字符 | 第 2 轮：重新折行 |
| B2-N2 | N | `oneFolder` 在 `count` 恰好 45,000 时第一包是空的，`Folder` 建不出来 | 第 2 轮：第一包至少一页，注释写明两包适用 45,000–89,999 |
| C2-M1 | M | 修复的那次分支 CI 有作业落在 7763 上，记录没有读 | 第 2 轮：实施报告补上每个作业的 CPU；3.13 第 6 条与 10.4 写 7763 的数，CI 写成五次；`14808583` 的注释不写 7763 的倍数（决定 673） |
| C2-N1 | N | 新表 `plans` 一行的注释与"三到四成" | 第 2 轮：任务里的计划两边另写；"三到五成" |

## 修复与核对

- **修复第 1 轮**（`fix-1.md`）：提交 `e3566f2e`（tools）、`14808583`（server），各经 `heavy.sh` 跑相关的门禁与变异；记录与报告就地改（原件 `*.pre-fix1`）。
- **核对第 1 轮**（`check1-brief.md`；三位各在 `14808583` 的分离 worktree 里，A、B 经 `heavy.sh`，C 只读）：A 原 7 条、B 原 6 条与变异表、C 原 8 条都关上。B 看了第 5 个提交 `45aa3026`：用 node 直接跑 `main` 与分支两版的 `shapes.ts`，各形状逐个条目比过，导入之后的树与原来相同（导入建的页排在同级之后，`/nodes` 不分页，第二包的名字不与第一包的重）。新发现 A 1N、B 2N、C 1M 1N。
- **修复第 2 轮**（`fix-2.md`）：提交 `b447dd28`，经 `heavy.sh` 跑 `make lint-go`、`page/domain` 与 `perf/vault` 的测试、`make lint-web`、`make knip`；C2-M1、C2-N1 只改记录（原件 `*.pre-fix2`）。
- **核对第 2 轮**：C 核 C2-M1，关上；A2-N1、B2-N1、B2-N2、C2-N1 由主持者核，都改了。

## 实施中定的

照 P2/S1 的写法，不逐条编号；主持者在交回之后都同意了。

- **导入上限的形状**照 3.12 的例按比例缩到四分之三（44,910 个节点，同 S3 离上限 90）；代价是解压不再接近 4 GiB（之后成了决定 671）。
- **批次 E** 照旧两个 5 万页的 vault 各分两份，不改成"每份 4 万页"（3.13 第 2 条）。
- **`-part` 的测法**：全尺寸只数不写（`contentsOf`，`Write` 写之前核对的同一个函数），zip 的上限在全尺寸上只由一次实写核对；另在小的 vault 上以"整个过、每份恰好不过"的上限证明 `Write` 核的是这一份。
- **3.12 没列的两处随常量改了**：来源计划的块数（不改就红）、`arrays` 基准的界（第一次量撞出来的，第 4 个提交）。前三个提交上这个基准是坏的；基准不在门禁与 CI 里（CI 只编译），照门禁的定义各自为绿。
- **在原规模上再量取分支的头**（之后成了决定 674）。
- **文档的草稿**：S6 不碰 `docs/`（README 在仓库根，改了），草稿逐条写旧文字与新文字，交本提交。

## 门禁与 CI

- **各提交**（经 `heavy.sh`，日志 `~/.cache/m12-p4-s6/c1/`–`c5/`、`fix1/`、`fix2/`，rc 都是 0）：第 1 个 `make lint-go`、`page/...` 与 `transfer/...` 的竞态测试、组合根里碰到常量的六个测试（含三个守卫各单独一行）、`make test-go-short`；第 2 个 `make lint`、`make gen-check`、`make knip`、`make build`、配置与 transfer 的测试、`make test-go-short`；第 3 个同上加 `go -C server/tools test -race -count=1 -v ./perf/...`；第 4 个 `make lint-go`、`BenchmarkBackgroundSteps/arrays`、两个后台守卫、`make test-go-short`；第 5 个 `make lint-web`、`make knip`；第 6 个 `make lint-go`、`server/tools` 的竞态测试、`make test-go-short` 与变异；第 7 个 `make lint-go`、`page/domain` 的竞态测试、组合根基准的编译、`make test-go-short`；第 8 个 `make lint-go`、`page/domain` 与 `perf/vault` 的测试、`make lint-web`、`make knip`。
- **分支 CI**（普通推送，`ci-wait.sh`；各作业的 CPU 取作业日志 `Runner CPU` 一步，`gh run view --job … --log` 只读，`web`、`e2e`、`image` 不打 CPU）：

  | run | 提交 | 结论 | 守卫的作业 | 其余作业落在哪 |
  |---|---|---|---|---|
  | 44832498021 | `d544acbe` | 19 项全绿 | `guards-core` 6975P-C：请求 369、806 ms，最大的一步 5.116 s，一组的第一批 898 ms；`guards-sources` 8573C：来源计划 1.85–2.12 s | 6975P-C 12 个，8573C `bootstrap-3`，9V74 `costs-2` |
  | 44836886662 | `89d9c701` | 19 项全绿 | `guards-core` 6975P-C：364、789 ms，5.101 s，908 ms；`guards-sources` 6975P-C：至多 1.52 s | 6975P-C 9 个，9V74 `bootstrap-3`、`-4`，9V45 `bootstrap-7`、`costs-3`，7763 `bootstrap-8` |
  | 44846276416 | `45aa3026` | 19 项全绿 | `guards-core` 6975P-C：374、786 ms，5.075 s，907 ms；`guards-sources` 9V74：1.42–1.66 s | 7763 `bootstrap-2`、`-5`、`-6`、`-7`，9V45 `bootstrap-3`、`-8`、`costs-lint`，9V74 `bootstrap-4`，8573C `race`，6975P-C 5 个 |
  | 44850050358 | `14808583` | 19 项全绿 | `guards-core` **7763**（18:33）：请求 536 ms、1.172 s，最大的一步 **6.966 s**（界 14，约一半），一组的第一批 1.342 s；`guards-sources` **7763**：1.81–2.03 s | 7763 `bootstrap-3`、`-5`（Test 6:56，在 450 秒之下）、`-7`；6975P-C `costs-1`、`-2`、`-3`、`bootstrap-4`、`-8`；9V74 `short`、`costs-lint`、`bootstrap-1`、`-2`、`-6`；8573C `race` |
  | 44854195196 | `b447dd28` | 19 项全绿 | `guards-core` **7763**（19:01）：558 ms、1.190 s，**6.933 s**，1.344 s；`guards-sources` 9V74：1.74–1.93 s | 6975P-C 11 个，9V74 `bootstrap-4`、`costs-3`，9V45 `bootstrap-5` |

  60,000 时 7763 上的最大的一步是 10.237 s（`main` 的 `21f9578d`，job 136610343148）；`guards-core` 落在 7763 上的 18–19 分钟与那次（19:12）同一档，不是改动带来的。`costs-3` 里 `server/tools` 那一行 79.1–81.7 秒（9V74 的 `70ac0074` 93.3），由 `perf/drive` 决定。
- **本提交**只改文档：之前跑 `python3 docs/v0.1/claude_to_codex/tools/check_links.py origin/main` 与 `git diff --check`（负责人第 12 问第 1 点）。分支 CI：本文档提交 `0ae0a6a1` 的 run 44858379817 第一次 attempt 19 项全绿，Release 44858379823 绿。S6 合并为 `9504e2b7`，之后第一次 `main` 的 run 44860539898 第一次 attempt 19 项全绿，Release 44860539789、Vulnerabilities 44860539853 绿。。

## 秘密

没有：S6 没有起服务、没有准备账户或 PAT；`perf vault` 不要凭证；CI 的作业日志只读、存在本机，不抄进文档。

## 环境

实施在 worktree `/home/parallels/Projects/nerve-wiki-m12-p4-s6`（分支 `m12-p4-s6`）；审查者各在 `~/.cache/m12-p4s6-review-*` 与 `~/.cache/m12-p4s6-check1-b` 的分离 worktree 里，用完删了。要留的在设计工作目录的 `m12/p4s6/`（`brief.md`、`fix-1.md`、`fix-2.md`、`check1-brief.md`、`impl-report.md`、`results.md`、`grep-list.md`、`review-*.md`、`work/`）与本机 `~/.cache/m12-p4-s6/B5/`、`B5-attempt1/`（带 `sha256.txt`）；`~/.cache/m12-p4-s6/` 的其余在合并之后删。
