# M2/P4 停用、管理命令与清理：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m2-p4-deactivation-commands-purge`（`main...308dfd0`，S1–S6，88 个文件，+4047/−634），对照 [04-P4-deactivation-commands-purge.md](../04-P4-deactivation-commands-purge.md)、各 Step 计划、[M2 总设计](../00-M2-design.md)第 4、7、8、9 节、[M1 移交](../handoffs/M1-identity.md)第 1、2、9、10 项、[P2 审查记录](P2-workspace-members-review.md)、[P3 审查记录](P3-invitations-review.md)、总体设计 13.1、13.4 |
| 审查方式 | 独立审查者在 `git archive` 的快照上实测，仓库与别的 Docker 容器都没动过：<br>• 门禁：`go test -race -p 3`（38 个包）、lint、knip、tools 的测试、vitest 446 个、`make build-web`、`make gen-check`、`make e2e`（62 个，快照 `git init` 过）；W2、W10、W12 `--repeat-each 3`，15 次全部通过；交错 1–13 `-race -count=10`，0 失败。`make image-smoke` 会覆盖本地镜像标签，没跑（作者跑过）；<br>• 反向对照：Go 34 项、e2e 2 项（汇总见下）；另有 4 个 PostgreSQL 探针，核实 3.2 的不成环论证与清理的行为 |
| 日期 | 2026-10-01 |
| 结论 | 可以合并。<br>• 停用的两个阶段：锁 → 锁下重读 → 规则二 → 否决者；订阅阶段重读同一集合，删邀请用锁下的邮箱。<br>• 加锁顺序 3.2 的不成环论证成立（Q6）。<br>• 第 5 节的 8 项反向对照全部失败。<br>没有 Major；4 项 Minor（T1 一处设计说法与级联的实际行为不符，T2–T4 三处测试缺口）与 5 项 Nit 合并前处理（`d816b29`，T9 不改），见下；6 个疑问：Q1、Q2 与 T1 的跨模块约束写进 M2 收尾时的 M3/M7 移交，Q6 的措辞在收尾补进 13.1 第 5 条，其余不改；文档偏差全部同步 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| T1 | Minor | 清理"不等锁、删工作区时已没有子行"不成立：子表的清理器用 `SKIP LOCKED` 跳过一行之后，同一次运行里删工作区，`ON DELETE CASCADE` 会去删这一行并等它的锁。探针：持有一行已软删的成员行时，`PurgeWorkspaces` 等到期限。M2 里只有重叠的两次清理会持有已软删的行；但 M7 的附件行若被父行的级联连带删掉，会绕过删文件那一步（M2 总设计第 10 节的风险），M3 起不带 CASCADE 的外键会让整批失败 | 工作区的清理器只删已没有成员与邀请行的工作区（`NOT EXISTS`）：子行被跳过时，工作区随它推迟到之后的运行，级联实际不会触发。`jobs.Purger` 的契约写明这一条，M3 起的清理器照此办理。新测试 `TestPurgeWorkspacesWaitsForTheirChildren`：持有一份邀请与一行成员，三个清理器都不等，两个工作区都留下；放开之后下一次运行全部删掉。反向对照：去掉任一个 `NOT EXISTS`，测试等到期限而失败 |
| T2 | Minor | 否决者收到哪个集合，没有测试守住：交给它锁住的集合 `ids` 而不是锁下重读的 `ending`（A7），全部通过。"ended while locking" 只有一个工作区，`standings` 为空时提前返回，分不出两者 | 加 `TestDeactivationEndsWhatIsLeftAfterTheLock`：两个工作区，一个在加锁时已结束，否决者与订阅者都只看到剩下的那个。A7 让它失败 |
| T3 | Minor | 批的上限与 `SKIP LOCKED` 只测了邀请：去掉成员、工作区两条语句的 `SKIP LOCKED`（A15）或 `LIMIT`（A16），全部通过 | `TestPurge`、`TestPurgeTakesBatches`、`TestPurgeSkipsALockedRow` 按三个清理器做成表格，夹具每张表至少两行可删。6 项反向对照（三条语句各去掉 `SKIP LOCKED`、各让批的上限失效）各有测试失败 |
| T4 | Minor | `TestLockWorkspacesOfLocksInIDOrder` 测不出缺了 `ORDER BY id`（A1）：只偶然被 `TestLockWorkspacesOf` 的返回顺序抓到 | 作者查了计划：没有 `ORDER BY` 时按 slug 的唯一索引取行，而测试的 slug 顺序恰好等于 id 顺序。改为 id 小的 slug 排在后面，并把它的工作区行与成员行重写一次，让表里的顺序也相反。A1 让这个测试 3/3 失败 |
| T5 | Nit | 另两处排序没有测试守住：去掉规则二的 slug 排序（A8）、订阅阶段的 id 排序（A28），全部通过：测试数据的 id 顺序恰好等于 slug、名称顺序 | 用例测试的第二个工作区改为 `abc`（id 在 acme 之后，slug 在前），断言 `(abc, acme)`；模块测试的 `beta` 改为 `abacus`（名称在前），断言按 id。A8、A28 各让一个测试失败 |
| T6 | Nit | `nervewiki workspaces` 不带子命令时打印帮助，没有测试（E1） | 加 `TestBareWorkspacesPrintsHelp`，断言两个子命令都列出。`RunE` 什么都不打印、少挂任一个子命令，都让它失败 |
| T7 | Nit | `nervewiki workspaces` 缺少 debug 级的"日志不含邮箱"检查 | 加 `TestWorkspacesLogNoAddress`：debug 级下依次创建、停用、启用、恢复，日志里有 `workspace_id=`，没有邮箱。恢复的日志加上邮箱时它失败 |
| T8 | Nit | 三处注释与代码或测试不一致：`domain/errors.go` 说停用由"本人、他的管理员或服务器管理员"发起；`TestLockWorkspacesOf` 说排除"只有成员行被软删"的工作区，夹具里没有（A3 全部通过）；`purge.sql`、`bootstrap/purge_test.go` 的级联说法见 T1 | `errors.go` 改为本人或服务器管理员；夹具补一个只有成员行被软删的工作区，A3 让它失败；两处级联的注释随 T1 改写 |
| T9 | Nit | 模块根构造的 `MembershipEnder` 的 `Profiles` 为 nil，若有人对它调用 `End` 会 panic，只靠注释提醒 | 不改：停用只调用 `Veto` 与 `Write`，这个值不出模块根；拆成两个类型要多一层组合，换来的只是防一个模块内部的误用 |
| T10 | Nit | 清理器中途失败时，它之前几批删掉的行不记日志 | `purge` 拆出 `purgeTable`，失败时也带回已删的行数，先记日志再返回错误。先返回、后记日志时，`TestPurgeStopsAtAFailure` 失败 |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| Q1 | workspace 自己扩展点的注册者交没交下去，没有测试证明：把停用注册者与 `Workspaces` 拿到的成员身份结束、恢复的注册者都换成 nil（B6），bootstrap 与 archtest 全部通过。组合检查只证明静态可达 | 现在没有注册者，测不了，不改。M2 收尾时写进 M3 的移交：M3 第一个成员身份结束或恢复的注册者，要在整个程序上经每条路径各有行为测试——移出、离开、停用（接口与命令行）、接受邀请、`reactivate-member`、删除工作区。已写进[M3 的移交](../../M3-notebook/handoffs/M2-workspace.md)第 1 项 |
| Q2 | 清理注册表的两条数据库测试能否守住 M3 以后的情形 | 部分能：漏登记、顺序颠倒守得住。要留给 M3/M7 决定的：自引用外键被排除在外（M4 的页面树）；只靠 CASCADE、没有 `deleted_at` 的子表过不了第二条测试；"失败即停"下一个永久失败的清理器让之后的都不跑，只在日志里看得到。M2 收尾时写进 M3/M7 的移交，连同 T1 的约束：见[M3 的移交](../../M3-notebook/handoffs/M2-workspace.md)第 4 项、[M4 的移交](../../M4-pages/handoffs/M2-P4-purge-page-tree.md)（自引用外键）、[M7 的移交](../../M7-assets-transfer/handoffs/M2-P4-attachment-purge.md)（先删文件） |
| Q3 | River 的任务期限默认 1 分钟，积压多时清理被取消重试；定时任务可能重叠 | 不改：每批各自提交，进度不丢；重叠的两次清理靠 `SKIP LOCKED` 互不等待（T1 之后对工作区也成立） |
| Q4 | `ShareActiveAccountByEmail` 等锁期间地址被另一个账户改用时答 not found；`reactivate-member` 恢复之后，发给他邮箱的待接受邀请会留下 | 不改：命令行重试即可；留下的邀请在接受时被消费，与 P3 的 set-email 路径相同 |
| Q5 | `PurgeJob` 用 `time.Now()` 算截止时刻，没有注入时钟 | 不改：13.1 第 9 条说的是用例；平台包不能导入 `platform/clock`，worker 的截止时刻由测试用前后两个时刻夹住 |
| Q6 | 3.2 的不成环论证是否成立 | 成立。探针：停用持有账户行并已写它时，工作区一支引用他的插入与更新都不等待（外键检查的 `FOR KEY SHARE`）；改邮箱之后两者都等。`set_email.go` 只碰 users 与 sessions。`LockWorkspacesOf` 的 `LockRows` 在最上层，按 id 取锁。两处措辞在 M2 收尾补进 13.1 第 5 条时修正：第 1 点还有"外键检查等改邮箱"（第 3 点已覆盖）；`KEY SHARE` 会越过排队的独占等待者。M3 的订阅者若写引用别的账户的列，审查时复核（[M3 的移交](../../M3-notebook/handoffs/M2-workspace.md)第 3 项） |

## 文档与代码的不一致

作者已知的 7 处，审查者逐条核实，都合理：用例文件名本来就是 `create_workspace_for.go`；`Member.EndedAt` 取代 `FindMembership` 的 `active`（P4 3.3 与 S1 已写明，P3 文档补一句指向）；store 拆出 `members.go`、`purge.go` 与两份查询（文件表要改）；`openAdminCommand` 在 `commands_admin.go`；e2e 夹具 `users.ts` 改名 `admin.ts`；W10 的接口版本也在 P4；运行时角色的测试另跑了两条 `nervewiki workspaces` 命令。

另外的：

- D1：P4 3.4 "它不等锁""删工作区时已没有子行"，与级联的实际行为不符（T1）。
- D2：P4 3.1、S2 计划写"经替身否决者答出"，实际是替身用例返回否决者的错误。
- D3：S2 计划说模块根测试核对了"账户仍可用、会话仍在"：模块测试不运行 identity，只有整个程序上的规则二测试看得到。
- D4：S3 计划的"子命令出现在帮助里"没有测试（T6）。
- D5：S1 计划与测试注释里"只有成员行被软删"的一例，夹具里没有（T8）；S3 计划的"恢复的订阅者失败时整体回滚"做在模块根（真实数据库），因为组合根没有注册者可接。
- D6：M2 总设计第 4、8 节写"模块经 `Purgers()` 提供清理器"，实际是包级的 `workspace.Purgers(pool)`。
- D7：合并时要更新 P4 第 7 节、M2 总设计"M1 移交的落实"表第 1、2、9、10 项与进度表；M2 收尾补 13.1 第 5 条。

全部处理：P4 文档 3、3.1、3.4、第 4、5、7 节，S1、S2、S3、S4、S6 计划，P3 文档 3.3 与第 7 节，M2 总设计第 4、7、8、11、12 节。

核对过、与实现一致的：README 的工作区命令、"停用账户""软删除与清理"与后台任务；`identity.yaml` 的 `deactivateMe`；`config.yaml` 的两项配置与校验；M7 的 handoff（[只投递的客户端](../../M7-assets-transfer/handoffs/M2-P4-insert-only-client.md)）；3.3 的三种输出；3.5 的交错表；3.6 的三个故事。

## 反向对照

会失败的（审查者与作者分别做过）：

- 第 5 节的 8 项：规则二恒为允许（领域、用例、模块、整个程序的规则二测试、交错 9、W10 两个故事）；规则二用锁前的人数（交错 9 两种先后都把两人停用、0 个管理员）；`deactivationRegistrants` 返回空（两条行为测试与交错 9；只去掉 serve 或命令行一侧时各自那一半失败）；订阅阶段不删邀请；`ShareAccountByEmail` 去掉 `FOR SHARE`（交错 12 两种先后）；清理器顺序颠倒（只有外键顺序测试，有 CASCADE 时功能测试照样通过）；清理不看保留期（worker 与 SQL 各一处，W12）；去掉一个清理器。
- 审查者另做：`LockWorkspacesOf` 改用 `FOR SHARE`；`ListStandings` 计入已结束的成员；恢复时固定角色、已是成员也恢复、不发事件、先锁工作区后共享账户行；清理失败后继续、不在启动时运行、`newApp` 不接清理任务；不规范化邮箱；契约去掉 `workspace.sole_admin`；成员清理的边界改成 `<=`。
- 作者在审查修复中做的 18 项：T1 两项、T3 六项、T2、T4、T5 两项、T6 三项、T7、T8、T10 各一项，见上表。

审查时改了却没有测试失败的 9 项（A1、A3、A7、A8、A28、A15、A16、E1、B6），除 B6（Q1）外都已补上测试，现在都会失败。

## 没能验证的风险

- 审查者没跑 `make image-smoke`；作者跑过，通过。持续集成为绿（`308dfd0`、`d816b29`）；修复之后作者在本地重跑门禁、`make gen-check` 与 `make e2e`（62 个），都为绿。
- e2e 在 `git init` 过的快照上跑，与持续集成的环境不完全相同。
- 大数据量下的清理没有验证：表上没有 `deleted_at` 的索引，每批都顺序扫描（Q3）。
- 多实例下 River 选主、定时任务重叠没有验证。
- T1、Q2 对 M3/M7 的影响只是推理加单次探针，等那些表的形状定下来再复核。
