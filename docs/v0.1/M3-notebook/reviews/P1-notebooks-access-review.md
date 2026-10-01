# M3/P1 笔记本与权限：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m3-p1-notebooks-access`（`main...6ac6a3e`，S1–S5，90 个文件，+7179/−63），对照 [01-P1-notebooks-access.md](../01-P1-notebooks-access.md)、各 Step 计划、[M3 总设计](../00-M3-design.md)第 4、5、7、8、9 节、[M2 移交](../handoffs/M2-workspace.md)第 1、4 项、总体设计 13.1、13.4 |
| 审查方式 | 独立审查者在 `git archive` 的快照上实测，仓库与别的 Docker 容器都没动过：<br>• 门禁：`go test -race -p 3`（43 个包）、lint、knip、vitest 698 个、`make gen-check`、`make e2e`（87 个，快照 `git init` 过）；笔记本的五个故事与 W12 `--repeat-each 3`，18 次全部通过；交错 15、16、注册者的行为测试、列表一致性 `-race -count=10`，notebook 与 workspace 模块 `-race -count=5`，0 失败；S1–S4 的中间提交各自 `go vet` 与 `go test -short`。`make image-smoke` 没跑（作者跑过）；<br>• 反向对照：Go 37 项、e2e 2 项（汇总见下） |
| 日期 | 2026-10-02 |
| 结论 | 可以合并。<br>• 判定的五个分支、"看不到"先于 403、写路径的次序（工作区行 `FOR SHARE` → 笔记本行 `FOR NO KEY UPDATE` → 判定 → 校验 → 写）、同一时刻的软删除、事务内的订阅者、工作区删除的注册者、清理器的顺序与 RESTRICT，都与设计一致。<br>• 列表的 SQL 与逐个判定在矩阵的数据上一致；私密笔记本对工作区管理员不可见；模块隔离成立。<br>没有 Critical、Major；2 项 Minor（测试缺口）与 8 项 Nit 合并前处理（`97ca3f4`、`8f97488`、`bb80fb5`、`660a01b`），见下；5 个疑问：Q2、Q4 改代码，Q3 写进 M4 的移交，Q1、Q5 改文档；文档偏差全部同步 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| T1 | Minor | 建笔记本时"笔记本与管理员成员行同一个时刻"没有确定的测试（13.1 第 9 条，同 M2/P2 审查 M2）：用例只读一次时钟，但用例测试的固定时钟读两次也是同一个值。把成员行改为再读一次时钟（M17），Go 测试全部通过，e2e N1 `--repeat-each 10` 只有 9 次失败 | 用例测试改用每读一次前进一微秒的时钟（`tickingClock`，照模块根测试），现有的"成员行的时刻等于笔记本的"断言因此有效。M17 让 `TestCreateNotebookMakesTheCallerItsAdmin` 失败 |
| T2 | Minor | `PurgeNotebooks` 的 `SKIP LOCKED` 没有测试（同 M2/P4 审查 T3）：测试只持有一行成员，笔记本被跳过靠的是 `NOT EXISTS`。去掉它（M21）全部通过 | `TestPurgeSkipsALockedRowAndWaitsForTheMembers` 另持有一本成员已清空的笔记本（`older`）：第一次运行两本都留下（`[3, 0]`），放开之后全部删掉（`[1, 2]`）。M21 让它等到期限而失败 |
| N1 | Nit | N13 的 `deletedDaysAgo` 按根到叶分五条语句推后时刻：清理恰好落在中间时，工作区的删除撞上 `notebooks_workspace_id_fkey` 的 RESTRICT，整次清理失败、记一条错误日志（故事靠 15 秒的轮询照样通过） | 改为叶到根：笔记本成员、笔记本、邀请、成员、工作区。竞态取决于后台任务的时机，没有确定的反向对照 |
| N2 | Nit | 列表排序的两个次要键没有测试：去掉 `n.id`（M25）或 `n.name`（M25b）全部通过 | `TestListNotebooks` 加 `notes`/`Notes` 与两本 `same`，每对都按与应有顺序相反的次序写入（后者 id 大的先插入）。M25、M25b 各让它失败 |
| N3 | Nit | `orNotFound` 在 `create_notebook.go`，`manage.go` 也用（同 M2/P2 审查 T6） | 挪到 `app/authorize.go`，与 `found` 放在一起 |
| N4 | Nit | 改名的日志 `notebook updated` 没有 `workspace_id`，建与删都有 | 补上；`TestUpdateNotebook` 核对日志里的工作区与笔记本 id，去掉它时失败 |
| N5 | Nit | `workspaces.go` 的注释与子测试名有语病；`ShareWorkspaceByID` 的注释只提邀请；`DeleteNotebooksOf` 与 `WorkspaceDeletion` 说"没有别的事务持有这些行"，只在 M4 之前成立 | 前两处改写；第三处随 Q2 改写 |
| N6 | Nit | `EffectiveNotebookRole` 的表格是 23 个组合加 5 个未知值，P1 第 5 节说的是 36 个的乘积 | 改为完整的乘积：九行（工作区角色 × 开放程度），每行四个期望值（显式角色依次为空、`reader`、`editor`、`admin`），未知值另列。把默认角色也给访客、或有显式角色时不取默认角色，各让它失败 |
| N7 | Nit | 矩阵的 `listNotebooks` 只有笔记本列：`notebook.list` 是工作区级的规则，"已结束的成员""已删除的工作区"在这个操作上没有格子 | 加变体"按工作区的列"：在 acme 上，管理员、成员、访客 200 且列表为空，其余 404 `workspace.not_found`。列表不看判定的结果时，"从未加入""已结束"两格失败 |
| N8 | Nit | `expectNotebooksDeletedWith` 不核对笔记本与成员行的 `updated_at` 等于工作区的删除时刻 | 补上两项比较。e2e 反向对照：工作区删除不写成员行的 `updated_at`、不写笔记本的 `updated_at`，N13 各失败一次 |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| Q1 | 作者已定的偏差（空的 PATCH 答 200、步骤调整、笔记本列在 `lab`、`FindBySlug` 只答 id、不变量的检查在 `createNotebook` 辅助里） | 审查者都同意。补充：P1 的交错结束时没有存活的笔记本，全表断言是空转的；P2 的交错 17–19 结局里有存活的笔记本，那时它才起作用；它只看 `ended_at`，P3 加无主时不用改 |
| Q2 | 工作区删除的注册者按扫描顺序批量锁笔记本行；M3 总设计第 8 节写的是"多行按 id 升序"。M4 的页面写只以 `FOR SHARE` 锁笔记本行、不锁工作区行，持两本的写（跨笔记本移动页面）与按扫描顺序的批量 `UPDATE` 可能成环（推理，没有复现） | 改代码，与总设计一致：`DeleteNotebooksOf` 先 `SELECT … ORDER BY id FOR NO KEY UPDATE`，再更新这些行。新测试 `TestDeleteNotebooksOfLocksThemByID`：id 大的一本先插入并被别的事务持有，删除等它时 id 小的一本已经锁住（`NOWAIT` 拿不到）。旧的写法与去掉 `ORDER BY` 都让它失败 |
| Q3 | 笔记本删除事件现在没有注册者：`deleteNotebook` 与工作区删除拿到的都是空集合，换成 nil 测试照样通过（同 M2 移交第 1 项） | 写进 [M4 的移交](../../M4-pages/handoffs/M3-P1-notebook-deletion.md)：M4 的第一个注册者要在整个程序上经 `deleteNotebook`、删除工作区、P3 的删除无主笔记本三条路径各有行为测试 |
| Q4 | Windows 保留名只认 `COM1`–`9`、`LPT1`–`9`；审查者记得还有 `COM0`、上标的 `COM¹`–`³` 等，没有核对原文 | 作者核对了微软的 [Naming Files, Paths, and Namespaces](https://learn.microsoft.com/en-us/windows/win32/fileio/naming-a-file)：保留的是 `COM1`–`9`、`COM¹`–`³`、`LPT1`–`9`、`LPT¹`–`³`（Windows 把 ISO 8859-1 的上标 ¹ ² ³ 当作数字），不含 `COM0`、`LPT0`、`CONIN$`。`CheckTitle` 补上上标三个；`COM⁴` 允许。P1 3.2、总体设计 3.5 同步 |
| Q5 | 跨模块外键检查里表的归属取自建表的迁移，P1 3.9 写的是"取自定义约束的迁移文件名" | 两者按 13.1 第 7 条等价，实现更稳。改文档 |

## 文档与代码的不一致

作者已知的偏差（Q1）之外：

- D1：3.1 的文件清单：`app/workspace_deletion.go` 实际在 `app/extension.go`；workspace 复用已有的 `ShareWorkspaceByID`，没有新查询；漏列 `app/manage.go`、`app/view.go`、`domain/member.go`、`adapter/postgres/purge.go`、`queries/purge.sql`、`notebook/extension_test.go`、`workspace/workspaces_test.go`、`bootstrap/notebook_registrants_test.go`、`shared.ReachedByAccess`（3.3 也没提）、e2e 的 `expectNotebooksDeletedWith`。
- D2：3.7 的 `createNotebook` 把 `FindBySlug` 写在事务里，代码在事务之前（与改、删的"先读、再锁"一致）。
- D3：3.7 与 S3 计划的 `minProperties: 1`（Q1 的第一项）；3.11 的"目标在 acme"（Q1 的笔记本列在 `lab`）；第 4 节的步骤表（矩阵在 S3，清理器、外键检查、事实端口的接线在 S2）。
- D4：3.9 的"模块取自定义约束的迁移文件名"（Q5）；第 5 节的乘积（N6）。
- D5：S2 计划写有 `NewNotebook`，代码是 `CheckDraft`、`CheckChange`、`Apply`；S2 计划的"清理跳过别的事务持有的行"原先只对成员行成立（T2）。
- D6：M3 总设计第 8 节"多行按 id 升序"与 P1 3.8 不一致（Q2，改代码后一致）。
- D7：合并时要更新 P1 第 7 节；M3 总设计第 7 节移交表的第 1 项（删除工作区这一条路径）、第 4 项与进度表。收尾时可以补进 13.1：第 6 条的测试补 `TestCrossModuleForeignKeysToPurgedTablesRestrict`，第 21 条的例子补 `workspace.NewWorkspaces`、`notebook.NewFacts`、`notebook.NewWorkspaceDeletion`。

全部处理：P1 文档 3.1、3.2、3.3、3.7、3.8、3.9、3.11、第 4、5、7 节，S2、S3 计划的偏差注记，M3 总设计第 7、11、12 节，总体设计 3.5；13.1 的两项留到 M3 收尾。

核对过、与实现一致的：3.2 的规则次序与码；3.4 的判定五个分支与 `Authorizer` 先读笔记本；3.5 的端口（slug 不合格式时不查库，`ShareByID` 在事务之外报错）；3.6 的列、约束与索引名、CHECK 的反例；3.8 的事件与注册者；3.9 的清理顺序与 RESTRICT；3.10 的加锁；3.11 的十二列与五行的答案；3.12 的五个故事；契约五个操作的码、结构与描述；前端的中英文案。

## 反向对照

会失败的（审查者与作者分别做过）：

- 第 5 节的 7 项：默认角色给访客（表格、矩阵两列、一致性）；列表的 SQL 忽略 `reached`（仓储、一致性、矩阵）；去掉"有效角色为空即看不到"（领域、矩阵 15 格）；外键改为 CASCADE（跨模块检查、schema）；清理器顺序颠倒（顺序检查、清理的整个程序测试）；删除笔记本不连带成员行（仓储、模块根、N6）；建笔记本不锁工作区行（交错 15 两种先后）。
- 审查者另做：成员关系不看 `ended_at`、成员数算上已结束的、去掉 `lower()`、去掉工作区过滤；去掉工作区的比较（只有 `Authorizer` 的单元测试：P1 的操作从笔记本取工作区，HTTP 走不到这条分支）；规则表给编辑者改名；用例不按工作区角色传 `reached`；事实不看 `ended_at`、不在调用方的事务里读；改、删不锁工作区行（交错 16 的握手抓到）；`LockNotebook` 去掉 `FOR NO KEY UPDATE` 或 `deleted_at`；删除读两次时钟；订阅者挪到提交之后；工作区删除不连带成员行、组合根不交注册者（交错 15/16 与注册者的行为测试；archtest 照样通过，与 M2 移交第 1 项的判断一致）、连已删除的笔记本再删一次；笔记本清理去掉 `NOT EXISTS`、批量上限失效；成员清理去掉 `SKIP LOCKED`；`FindBySlug` 不先判断格式、`ShareByID` 不检查事务。
- 改了却没有失败、已由上面的修复补上的：M17（T1）、M21（T2）、M25 与 M25b（N2）。
- 作者在审查修复中做的 13 项：T1、T2、N2 两项、Q2 两项、N4、N6 三项（两项判定、一项保留名）、N7、N8 两项（e2e），各见上表。
