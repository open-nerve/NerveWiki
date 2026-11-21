# M12/P4 S7（B4 的树写）：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m12-p4-s7`，从 `main` 的 `70ac0074`（S7 的设计提交）开，九个代码提交与两次合并 `main`（下面"提交"）；实施者的报告、量的脚本、变异表与提交说明在设计工作目录的 `m12/p4s7/`（`impl-report.md`、`impl/`），原始数据在本机 `~/.cache/m12-p4-s7/`（`m/` 是 6.3 的量，`logs/`、`ci/` 是门禁、变异与 CI 的日志）。本提交是 S7 的文档提交。 |
| 规格 | [P4/S7 设计](../04-P4-S7-tree-writes.md)（决定 652–667）与它的[设计审查记录](P4-S7-design-review.md)；[P4 设计](../04-P4-performance.md) 3.6 的 B4、3.9、3.5.4（R = 2.046）；[00 号文档](../00-M12-design.md) 4.7.1 的 B4、4.7.5，决定 642（负责人 2026-11-21 第 13 问第 1 题） |
| 审查方式 | 三位代码审查者并行，各在 `869c458c`（前四个提交）的分离 worktree 里、经 `heavy.sh` 跑，用完删了：A 宽读与它的选择，B 参与者与观察者的分工、备忘的生命周期，C 测试、组合根的守卫、`perf ops`、CI 与变异（下面"分工"）。两轮修复、一轮核对：核对第 1 轮（分支头 `46aacd60`）三位各核自己的原发现，并看之后的四个提交、合并与 6.3 的量（A、B 经 `heavy.sh` 重跑，C 从原始数据只读复算）；原发现全关，新的只有 Nit，由主持者核（负责人 2026-11-19 的规矩）。 |
| 日期 | 2026-11-21（实施、量、审查、两轮修复与核对） |
| 结论 | 键与目标合起来至少 4,000 个的到达改读宽读（一次按页取；平常 `LinksReachedWide`，`force_generic_plan` 下 `LinksReachedWideGeneric`），单个改名、移动的单元里观察者用参与者读到的链接与解析。在原规模上交错着量（改前 `9504e2b7`、改后 `46aacd60`）：删除、移动、改名 `vault` 的主行推断 7763 上 17.21、27.09、26.42 → **9.96、6.82、5.96 秒**（× 1.25 12.49、8.52、7.45），**B4 过**；改被链得最多的页（0.40 → 0.41 秒）与删 `f1`（2.97 → 3.04 秒）不退步。病态的三种形状仍过期限（推断），已知限制。审查：A 1M 4N，B 1M 4N 1Q，C 1M 3N，没有 Important，主持者全部接受；最要紧的三条是随机对照在门槛之上走不到并行的计划（A-M1）、观察者用备忘时跳过 `spread` 没有测试守（B-M1）、`perf ops -quiet` 的最后一跳没有测试（C-M1）。核对第 1 轮原发现全关，新的 A 2N、B 1N、C 2N 在修复第 2 轮改了。 |

## 提交

| # | 提交 | 说明 | 做了什么 |
|---|---|---|---|
| 1 | `44169266` | server: a reach of many keys and targets reads the links it meets in one pass | A：`links.sql` 的 `LinksReachedWide`、`LinksReachedWideGeneric`、`PlanCacheMode` 与 `make generate`；两个转发；`Store.Links` 按门槛（`linksReachedWideFrom` = 4,000）与计划方式选；archtest 认两个新文件；4.1、4.2 的测试；`costs-2` 两行，时限 45 → 60 分钟。14 个文件，+1,579 −17 |
| 2 | `5e8484a6` | server: a rename or a move reads and resolves the links it reaches once | B：`P/app` 的 `UnitMemo` 与 `Write.Memo`；`L/app` 的 `unit_reach.go`（`keepReach`、`dropReach`、`takenReach`、`sameChange`），`Moved` 加 `Tree`、`Batch`、`Memo`，`PagesChanged` 加 `Memo`；组合根两个适配器传它们；`NewIndex` 给 `Index.Logger`；4.3 的测试。15 个文件，+1,085 −45 |
| 3 | `899fa82f` | server: a guard of a large top's move, rename and deletion, by the statements they read | 4.4 的守卫 `TestTreeWritesOfALargeTopReadTheLinksOnce`（`!race`，1 万个节点的最上层，读的语句计数加界）；`newTrashBenchTraced`；`costs-3` 一行。4 个文件，+221 −1 |
| 4 | `869c458c` | tools: perf ops waits for its notebook's background only, under the editors' load | `perf ops -quiet all|notebook`（6.3 第 5 条）：`client.QuietIn`、`drive.Ops.QuietNotebook`。6 个文件，+92 −11 |
| 5 | `631ff86b` | server: the wide reads' comparison reads parallel plans past the threshold, and their tests count what they compare | 修复第 1 轮：A-M1、A-N1–N4；`ci.yml` 的注释 |
| 6 | `810dbb54` | server: a single rename or move's tests count the observer's spread too, and its comments say what holds | 修复第 1 轮：B-M1、B-N1–N4 |
| 7 | `6e720688` | tools: perf ops reads its flags in a function its test runs, and its usage names -quiet | 修复第 1 轮：C-M1、C-N1 |
| 8 | `5aa8e460` | server: the random index test makes units whose participant writes a page, and B leaves them alone | B-N1 的读数是 0 之后主持者定（决定 683）：随机测试每个种子造两个参与者会写页的单元，断言 B 不用、计数加下限 |
| 9 | `46aacd60` | Merge branch 'main' into m12-p4-s7: P4/S6, MaxJobItems at 45,000 and perf vault -part, before S7's measures | 合进 S6 的合并 `9504e2b7`，没有冲突；6.3 的改前是它（决定 659） |
| 10 | `1786459f` | server: the random index test checks a step's random unit before the unit it makes, and a comment reflowed | 修复第 2 轮：B2-N1、A2-N1 |
| 11 | `29ae9a9b` | Merge branch 'main' into m12-p4-s7: S6's record, under S7's documents | 合进 `ac90bc5c`（S6 的记号，只改文档），没有冲突；本提交的旧文字以它为锚 |

提交说明都以 `(M12/P4/S7)` 结尾、带 `Co-Authored-By`。S7 相对 `main` 的 `ac90bc5c`：39 个文件，+3,257 −92，都在 `server/`、`Makefile` 与 `ci.yml`，没有改接口、网页、迁移、事务与锁。

## 在原规模上再量（设计 6.3）

- **两个提交**：改后 `H = 46aacd60`（量的时候分支的头），改前 `B = git merge-base H main = 9504e2b7`（S6 的合并）。三条核对都过：S6 的合并是 B 的祖先（B 就是它）；`git log --first-parent B..H` 只有 S7 的 8 个提交与合并 `46aacd60`；`git diff --stat B H` 39 个文件都是 S7 的。修复第 2 轮（`1786459f`）只改测试，不重量。
- **机器与搭法**：本机 8 核 arm64（Parallels）；两个分离的 worktree 各 `make build`，每轮以 `stack.sh` 的 `$NERVEWIKI` 换 serve，`nervewiki version` 写着那一边的提交、`modified=false`；`perf` 用 H 编一次。zip 用 S3 的参数（`-seed 1201`）的 `-part 1/2`、`2/2`（sha256 `0140f376…`、`bd614dab…`），两次导到同一页 `vault` 之下。九轮每轮的子树 59,911 个节点、1,350,000 条链接、没解析的 0 条、被链得最多的页被 59 页链着，都逐字等于调查的 `run1/vault.dump`（一次导入）。
- **锁与负载**：四段各持一次 `heavy.sh`：交错的六轮 11:55:59–15:52:41、编辑者负载 15:52:46–17:02:54、`f1` 17:02:54–17:13:22、守卫 17:13:27–17:42:10。每 5 秒记 `/proc/loadavg` 与前五名进程：锁外只有零星的 `gh`、`fwupdmgr`、`localsearch-ext`、`containerd-shim`、`(udev-worker)` 与常驻进程，审查 C 把记录对上每一次计时的请求（前后各放宽 5 秒），没有一样落在里面；没有作废的轮。
- **读数**（秒；三次的中位，两轮取较大；R = 2.046；删除的主行 = `fsync=off` 三次的中位 × R +（本轮中位 − `fsync=off` 中位））：

  | 操作 | 改前 r1 / r2 中位 | 改后 r1 / r2 中位 | `fsync=off` 中位（改前 / 改后） | 主行（改前 → 改后） | × 1.25（改前 → 改后） |
  |---|---|---|---|---|---|
  | 删除 `vault`（新库上第 1、2、3 次） | 8.42 / 7.83 | 4.80 / 4.63 | 8.41 / 4.94 | 17.21 → **9.96** | 21.51 → 12.49 |
  | 移动 `vault` | 13.24 / 12.49 | 3.33 / 3.13 | — | 27.09 → **6.82** | 33.86 → 8.52 |
  | 改 `vault` 的名 | 12.91 / 12.15 | 2.91 / 2.84 | — | 26.42 → **5.96** | 33.02 → 7.45 |
  | 改被链得最多的页（59 页） | 0.19 / 0.20 | 0.20 / 0.19 | — | 0.40 → 0.41 | 0.50 → 0.51 |
  | 删 `f1`（6,871 个节点，只记） | 1.45 / 1.35 | 1.44 / 1.49 | — | 2.97 → 3.04 | 3.72 → 3.80 |

  **B4 过**：主行与 × 1.25 一行都 ≤ 15 秒，没有一次 5xx，不复核。不退步的两行过（0.200 ≤ 0.195 × 1.2；1.486 ≤ 1.453 × 1.2）。审查 C 从 `*-ops.jsonl` 的 `done − sent` 复算每一个数，与 `took_ms` 差都在 1.5 ms 之内，主行逐个相同。这台机器上 `fsync=off` 不比 `fsync=on` 快，删除的主行实际是中位 × R（式子比"较大的中位 × R"高 0.15 秒，偏保守）。
- **删除随序号涨**（S7 设计第 9 节第 1 条）：改后 `fsync=off` 第 1、2、3 次 4.19、4.94、5.70 秒（主行 8.44、9.96、11.51），改后六个增量 0.65–0.94 秒；同一个最上层第 5–6 次删除的主行过 15 秒（推断：第 5 次 14.6–15.4 秒，随增量取哪一轮；第 6 次 16.1–17.3 秒；审查 C2-N1，决定 686）。
- **编辑者负载下**（只记，决定 661）：`perf editors -n 20 -for 25m` 在另一本笔记本里，`perf ops -reps 1 -quiet notebook -idle 30s` 在它稳态之后起、结束之前完（改前、改后各一次）。删除、移动、改名、改被链得最多的页、删 `f1`：改前 8.28、14.14、14.07、0.25、1.44 秒，改后 4.47、3.41、2.96、0.26、1.20 秒；编辑者的保存各 14,980 次全 200，p99 125.4 → 117.5 ms。
- **病态形状**（推断）：20 万个节点的最上层的删除用两点（`fsync=off` 新库上改后的第 1 次：`vault` 59,911 个节点 4.19 秒，另一个新库上 `f1` 6,871 个节点 1.14 秒）连线，20 万处 12.25 秒，× R 约 25 秒，到 15 秒约 11.4 万个节点；约 60 万个元素的到达可能先因 64 MB 的 `/dev/shm` 报错。400 页 × 2,000 条链接的 `move`、被 2,500 页链着的页的改名 S7 不改，7763 上仍约 40–41、29 秒。
- **别的走到宽读的单元**（6.3 第 7 条）：`guards-core` 的五个与 `guards-sources` 的三个，改前、改后、改前、改后各一次，都过。审查 A 打出每个守卫的到达：过门槛的六个（恢复、撤销一次删除、后台删 45,000 个节点的那一步、来源的三个）持平或快（那一步 4.82、4.71 → 4.25、4.13 秒）；撤销多个最上层（3,000）、batch 的改写的界（803）与导入的撤销（每批至多约 200）不过门槛，交错的两轮里改后慢 5–9%，审查 A 以平衡次序（改后、改前、改前、改后）各跑四次不慢，接受（决定 689）。
- **原始数据**：`~/.cache/m12-p4-s7/m/`（`steps.log`、每轮的 `*-ops.jsonl`、`sample-*.csv`、`load.log`、`snapshots.txt`、`*-check.txt`、`*-tables.txt`、`serve.log`，守卫的 `-v` 输出），脚本在 `m12/p4s7/impl/measure/`；`serve.log` 九轮 ERROR 0、WARN 0。两个 zip 已删，可由命令与 sha256 复现。

## 变异与对照

- **实施**（`mut.py`，`m12/p4s7/impl/mut/` 的 `light.json`、`heavy.json`、`guard.json`、`extra.json`，经 `heavy.sh`）：设计 6.1 的 a1–a19、b1–b13、g1、g2 与自加的 x1–x6、b6b–e、w1–w5。全部先绿后红，除了：a15、b8 照绿，等价（a15：比较取列的隐式 `"C"`，`"C"` 与库的默认排序规则都是确定性的，`text` 的相等逐字节；b8：过滤只去掉 `Written`、`Gone`、`Frozen` 命中的行，三样只由 `batchMoved` 在 `s.Batch != nil` 时填，条件 2 把它们挡在外面，多一个给非 batch 填它们的调用者就不再等价）；**b3 不等价**（设计写等价）：只改大小写的一步参与者存了备忘、观察者在 `Affected` 答否时返回，备忘留到单元末，红在 `TestTheIndexOfUnitsOfManyWritesIsItsRebuild`（决定 682）；b7 写成"取了、打了 debug 行、不用"时组合根的 `TestTheIndexIsItsRebuild` 照绿（它数 debug 行），由守卫与单元测试抓，写成"不取"时红。最早一轮 `light.json` 在锁外跑，撞上 S6 的量（S6 那一轮作废重量），之后都经 `heavy.sh` 重跑。
- **反向对照**（6.2）：撤 A、撤 B、都撤时 1 万个节点的守卫红在结构断言（读的语句数），不在计时（撤 A 只慢 1.1–1.2 倍，撤 B 1.8–2.3 倍）；`Generic` 换成 v3、v2，`Wide` 换成 v4h，红在 `TestLinksReachedWideReadPlans` 的 `EXPLAIN` 一步。
- **修复第 1 轮**（`m12/p4s7/impl/fix1/`）：R1（先 `spread` 再取备忘）、N4（条件 4 只全比第一项）、AN1-r2、r3（吞掉或当作 `auto` 读设置的错）、AM1b（门槛之上不设并行的代价）、P1–P3（`-quiet` 被忽略、反过来接、默认改成 `notebook`）都红；AM1（门槛之上的段当作门槛之下）连断言一起关了，不算。**随机测试造写页的单元**（`5aa8e460`，`impl/fix2/`）：W1、W3（写页之前不丢备忘）红在"观察者取了参与者的链接"，W2（造出的页不带维基链接）红在下限。
- **审查者自己的**：A 的 r1（archtest 收下三个常量中任一个，红）、r2、r3（吞掉或当作 `auto` 读设置的错，照绿，成了 A-N1）与两个探针（门槛之上的计划，成了 A-M1）；核对第 1 轮打出八个守卫的到达、以平衡次序重跑三处慢的守卫。B 的 R1（先 `spread` 再取备忘，照绿，成了 B-M1）、N1、T1（今天等价）、B3（只有随机测试抓，成了 B-Q1）、W4（红）；核对第 1 轮 C1-R1、C1-N4 红，C1-W0（去掉造出的单元）三个计数回到 742、64、0，红在下限。C 的 P1–P3 在 `869c458c` 上照绿（成了 C-M1），在 `46aacd60` 上各红一处。

## 分工

- **A：宽读与它的选择**（`m12/p4s7/code-review-a.md`）：`links.sql` 的两条宽读、`Store.Links` 的门槛与计划方式、archtest、4.1 与 4.2 的测试、计划的形状与物理工作量。经 `heavy.sh` 跑探针与变异；核对第 1 轮另在 `Store.Links` 临时打出八个守卫的到达，并以平衡次序重跑三处慢的守卫（17:49:39–17:52:01、17:53:03–17:54:40）。
- **B：参与者与观察者**（`code-review-b.md`）：`unit_reach.go` 的四个条件、备忘的生命周期、`P/app` 的 `UnitMemo`、组合根的接线、4.3 的测试与随机测试。核对第 1 轮经 `heavy.sh`（17:48 起）跑 `L/app` 与组合根的测试和三条变异。
- **C：测试、守卫、CI 与变异**（`code-review-c.md`）：4.4 的守卫、`Makefile` 与 `ci.yml` 的时限、变异表、`perf ops` 的开关、分支 CI 的作业日志；核对第 1 轮从 `~/.cache/m12-p4-s7/m/` 只读复算 6.3 的每一个数（`recompute.py`），对齐负载与每一次请求。

## 审查的发现与处置

主持者的处置在 `m12/p4s7/code-fix-1.md`、`code-fix-2.md`，全部接受，照审查者的建议改；改在哪写在实施报告第 11、13 节。

| 号 | 级 | 发现 | 处置与修复 |
|---|---|---|---|
| A-M1 | M | 随机对照门槛之上的组走不到并行与 `BitmapOr` 的计划（夹具 2.7 万条链接，默认代价下规划器不并行），4.1 第 2 条要的大数组的计划没有对照 | 取建议 1（决定 681）：门槛之上的段在会话上把并行的四个代价设为 0，每段前 10 组 `EXPLAIN`，至少 7 组有 `Gather`（实测 10 / 10）；`BitmapOr` 这个规模上造不出，不强求。整行 58.7、58.5 秒（`631ff86b`） |
| A-N1 | N | "读设置失败就是读链接的错"一格抓不到吞掉设置的错 | 只让 `PlanCacheMode` 失败，断言 `context.Canceled`、前缀、trace 只有它一条 |
| A-N2 | N | 随机对照不断言比的不是两个空集合 | 每段 `rows < groups` 就错 |
| A-N3 | N | 哈希子计划的检查不数子计划 | 另断言个数：`Generic` 4 个、`Wide` 0 个 |
| A-N4 | N | `LinksExcept` 没有一句说它不走宽读 | 注释，指向决定 655 |
| B-M1 | M | 观察者用上备忘时跳过 `spread`，没有测试守：把 `spread` 挪回取备忘之前全部照绿 | 替身给 `Subtree`、`AliasKeys` 计数，`reads()` 带上；R1 红在移动、改名两格（`810dbb54`） |
| B-N1 | N | `participantsWritten` 只加不读 | 计数并进合计；读数是 0（世界的参与者从不写页），主持者定另起 `5aa8e460` 造这样的单元（决定 683） |
| B-N2 | N | `TestTheIndexIsItsRebuild` 的第二个下限在构造上恒满足 | 只给 B 用上的单元设下限 60；去处变了的合计只记（决定 678） |
| B-N3 | N | 注释说的"重试"不存在 | 改成"随单元在事务的闭包里建，不是别的单元的" |
| B-N4 | N | 条件 4 的格只改第一项；两条防御性的判断没有一格会红 | 加一格改最后一项（N4 红在它）；N1、T1 写进变异表作等价 |
| B-Q1 | Q | b3 的处置 | 记为"不等价，随机测试抓"，保留比设计严的断言（决定 682） |
| C-M1 | M | `perf ops -quiet` 的最后一跳与默认值没有测试，P1–P3 都照绿 | `opsFlags`、`opsOptions.ops` 提出来，`ops_quiet_test.go` 三格；P1–P3 各红一处（`6e720688`） |
| C-N1 | N | `doc.go` 的 `perf ops` 用法没有 `-quiet` | 加 `[-quiet notebook]` 与一句；合并 S6 时照 S6 的为准再补 |
| C-N2 | N | 不加 build tag 的测试没有秒数 | 照 CI `race` 一份的包时间与本机的读数补进设计 10.2 |
| C-N3 | N | 变异表的 b3、b8 的文字 | b3 改"不等价"，b8 补依据（设计 6.1） |
| A2-N1 | N | 随机对照的说明有一行 135 个字没有折 | 修复第 2 轮重折（`1786459f`） |
| A2-N2 | N | 设计 3.1 与 6.3 第 7 条把三个不过门槛的守卫算作走到宽读的；实施报告 12.7 的推断（刚过门槛的宽读慢）不成立 | 文档改：3.1、6.3 第 7 条更正，过门槛的是六个；10.8 写到达与平衡次序的读数（决定 689） |
| B2-N1 | N | 第 13、33 步一次跑两个单元，只核对、只报后一个 | 修复第 2 轮：随机单元之后先核对一次，再跑造出的单元（`1786459f`） |
| C2-N1 | N | 删除过 15 秒的次数取了最平的一轮的增量；第 5 次 14.59 秒对增量敏感 | 写成"第 5–6 次"，写出敏感（决定 686） |
| C2-N2 | N | `VmHWM` 两处写法不一；锁外进程的清单不全 | 报告与设计 10.6 分开写交错的六轮与编辑者两轮；清单写全 |

## 修复与核对

- **修复第 1 轮**（`code-fix-1.md`）：提交 `631ff86b`、`810dbb54`、`6e720688`，各经 `heavy.sh` 在干净的临时 worktree 上跑 `check.sh`（build、vet、`make lint`、gen-check、knip、build、short、`server/tools` 的测试）与相关的测试和变异；之后主持者看 B-N1 的读数 0，定了 `5aa8e460`（`L/app` `-race` 与变异 W1–W3）。报告就地改（原件 `*.pre-fix1`）。
- **合并 `main`**：S6 合并之后 `46aacd60`（量的前提），之后量（第二次交回）。
- **核对第 1 轮**（`code-check1-brief.md`，分支头 `46aacd60`）：A 原 5 条、B 原 6 条、C 原 4 条都关上；新的 A2-N1、A2-N2、B2-N1、C2-N1、C2-N2，都是 Nit。
- **修复第 2 轮**（`code-fix-2.md`）：提交 `1786459f`，经 `heavy.sh` 跑 `make lint`、两个包的 `go vet`、`L/app` 整个包 `-race`（18.9 秒，三个计数仍 824、73、600）；合并 `ac90bc5c`（`29ae9a9b`）；报告就地改（原件 `*.pre-fix2`）。Nit 由主持者核。

## 实施中定的

主持者在交回之后都同意了，编了号（00 号文档第 11 节）：

- 宽读的计划测试从表达式判子计划是否哈希（PostgreSQL 18 的 `EXPLAIN` 把 "hashed" 写在用它的表达式里），决定 675；
- 随机对照门槛之下 2,000 组（3,000 组本机 69 秒，过了设计的 60 秒），决定 676；
- `force_generic_plan` 的计划格只比夹具的算术（`LinksReached` 在通用计划下对 8,192 个是平方的），决定 677；
- `TestTheIndexIsItsRebuild` 每个种子造六步、只给 B 用上的单元设下限 60，决定 678；
- 守卫的结构断言用 `Errorf`、放在 `costs-3` 的倒数第二行，决定 679；
- `costs-2` 的时限 60 分钟、`costs-3` 不变，决定 680；
- 量里的五条（`f1` 的那一点用 `curl`、编辑者一轮的令牌分开、〈n〉的外推、`fsync=off` 一轮的那一点不进判法、守卫的 batch 一行只跑一个测试），决定 684–688。

## 门禁与 CI

- **各提交**（经 `heavy.sh`）：前四个推送之前在干净的临时 worktree 上逐个 `check.sh` 全 0，并跑设计 5.1 列的相关测试（`L/adapter/postgres` 与 archtest、`guards-core` 的五个与 `guards-sources` 的三个、导入与 LLM Wiki 的几个、`L/app`、`P/app` 与组合根里改名、移动、事件、链接的 23 个测试 `-race`、守卫与恢复的守卫）；修复的三个与 `5aa8e460` 同样；`1786459f` 跑 `make lint`、两个包的 `go vet`、`L/app` `-race`。日志在 `~/.cache/m12-p4-s7/logs/`（`percommit*.out`、`f1-verify.out`、`fix3-gates.out` 等）。
- **分支 CI**（普通推送，`ci-wait.sh`；各作业的 CPU 取作业日志 `Runner CPU` 一步，`gh` 只读）：

  | run | 提交 | 结论 | 与 S7 相关的作业 | 各作业落在哪（`server` 的十六份） |
  |---|---|---|---|---|
  | 44849297290 | `869c458c` | 19 项全绿 | `costs-2` 19:42（9V74）：随机对照一行 81.6 秒、计划测试一行 38.0 秒（本机的 1.54 倍）；`costs-3` 13:21（6975P-C）：守卫一行 14.3 秒，无统计的移动、改名、移动、删除 637、577、619、827 ms | 6975P-C 10 个；7763 `bootstrap-5`、`-6`；8573C `bootstrap-7`、`costs-1`；9V74 `costs-2`、`short` |
  | 44856274644 | `6e720688` | 19 项全绿 | `costs-2` 17:03（6975P-C）：随机对照 76.4 秒（门槛之上三段都是 10 / 10 组有 `Gather`）、计划测试 33.1 秒；`costs-3` 19:38（一台慢的 9V74）：守卫 23.3 秒，953、902、907、1,197 ms | 6975P-C 13 个；7763 `bootstrap-4`；8370C `bootstrap-6`；9V74 `costs-3` |
  | 44859429082 | `5aa8e460` | `server` 十六份与 web、image 绿；e2e 跑到 2:23 时被接着推的合并提交的并发组取消（同样的代码在下一次全绿） | `costs-2` 16:32、`costs-3` 13:19（都是 6975P-C）；`guards-core` 15:19（9V74） | 6975P-C 12 个；9V74 `costs-lint`、`guards-core`、`race`；9V45 `costs-1` |
  | 44861069727 | `46aacd60` | 19 项全绿 | `guards-core` 19:27（**7763**，Test 19:18）：恢复 67.9、撤销一次删除 68.5、撤销多个最上层 6.5、后台最大一步 54.7、batch 改写的界 8.7 秒；`race` 15:05（**7763**）：`L/adapter/postgres` 包 110.9 秒；`costs-2` 20:58（9V74）；`costs-3` 13:02（6975P-C） | 7763 `bootstrap-1`、`-4`、`guards-core`、`race`；9V74 `bootstrap-3`、`costs-2`、`guards-sources`、`short`；6975P-C `bootstrap-8`、`costs-1`、`costs-3`、`costs-lint`；8370C `bootstrap-2`；6973P-C `bootstrap-5`；8573C `bootstrap-6`；9V45 `bootstrap-7` |
  | 44899766157 | `29ae9a9b` | 19 项全绿 | `costs-2` 27:21（**7763**，Test 27:12 = 1,632 秒）：随机对照一行 113.5 秒、计划测试一行 47.5 秒，计划测试十五格最快一次 125–236 ms、用到界的 47–58%；`guards-core` 12:25、`costs-3` 13:11、`race` 9:28（都是 6975P-C） | 7763 `bootstrap-2`、`-8`、`costs-2`、`costs-lint`、`short`；6975P-C `bootstrap-6`、`costs-1`、`costs-3`、`guards-core`、`race`；9V74 `bootstrap-1`、`-3`、`-4`、`-5`；9V45 `bootstrap-7`、`guards-sources` |

  组合根的八份作业时间至多 6:53（`bootstrap-4`，7763）。CPU 由 `m12/p4s7/impl/tools/ci-cpus.sh` 从各作业日志的 `Runner CPU` 一步读出（`~/.cache/m12-p4-s7/logs/ci-cpus-*.tsv`）。

- **4.5 要的 7763 上的作业时间**：`guards-core` 落在 7763 上的一次 19:27（Test 19:18）= 1,167 秒，设计 4.5 的表里 7763 上 `main` 是 1,187–1,250 秒，相当，40 分钟的时限是它的 2.06 倍；`race` 15:05（Test 14:53）= 905 秒，对 901–907 秒，`L/adapter/postgres` 包 110.9 秒对设计的 109 秒；`costs-2` 27:21（Test 27:12 = 1,632 秒），推断是 1,536 + 82.3 × 2.4 = 1,734 秒，60 分钟的时限是它的 2.2 倍，两条新测试一行 113.5、47.5 秒。`costs-3` 还没有落在 7763 上：照决定 690，合并之后两次 `main` 里若 `costs-3` 落在 7763 上、过了 1,200 秒，另起提交把时限提到 45 分钟。`costs-3` 那次 19:38 在一台慢的 9V74 上（逐行对 `main` 的 9V74 慢 20–35%，审查 C）。
- **本提交**只改文档：之前跑 `python3 docs/v0.1/claude_to_codex/tools/check_links.py origin/main` 与 `git diff --check`（负责人第 12 问第 1 点）。分支 CI：本文档提交 `da003f2e` 的 run 44903038611 第一次 attempt 19 项全绿，Release 44903038616 绿。S7 合并为 `75d31f1f`，之后第一次 `main` 的 run 44905222643 第一次 attempt 19 项全绿（`costs-3` 13:26、`guards-core` 19:01，都在时限的一半之内），Release 44905222689、Vulnerabilities 44905222745 绿。

## 秘密

没有：6.3 的每轮在本机起了服务与 PostgreSQL，账户与 PAT 由 `stack.sh`、`perf prepare` 临时生成，只在本机的数据目录里，每轮 `down` 之后连数据一起删了；记录与日志里没有口令与令牌；CI 的作业日志只读、存在本机，不抄进文档。

## 环境

实施在 worktree `/home/parallels/Projects/nerve-wiki-m12-p4-s7`（分支 `m12-p4-s7`）；量用的两个分离 worktree（`~/.cache/m12-p4-s7/wt-before`、`wt-after`）与审查者的 `~/.cache/m12-p4s7-*` 用完删了。要留的在设计工作目录的 `m12/p4s7/`（`impl-brief.md`、`code-review-*.md`、`code-fix-1.md`、`code-fix-2.md`、`code-check1-brief.md`、`docs-brief.md`、`impl-report.md`、`impl/`）与本机 `~/.cache/m12-p4-s7/`（`m/`、`logs/`、`ci/`），后者在合并之后删。
