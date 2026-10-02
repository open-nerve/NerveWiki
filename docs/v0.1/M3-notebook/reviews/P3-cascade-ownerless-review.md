# M3/P3 级联与无主：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m3-p3-cascade-ownerless`（`main...db3bae4`，S1–S5，97 个文件，+6662/−120），对照 [03-P3-cascade-ownerless.md](../03-P3-cascade-ownerless.md)、各 Step 计划、[M3 总设计](../00-M3-design.md)第 3、4、5、7、8、9 节、[P1](../01-P1-notebooks-access.md)、[P2](../02-P2-notebook-members.md) 文档与它们的审查记录、[M2 移交](../handoffs/M2-workspace.md)第 1–3 项、总体设计 3.3、3.4、6.1、8.2、12.4、13 |
| 审查方式 | 独立审查者在两份 `git archive` 快照上实测，仓库与别的 Docker 容器都没动过：<br>• 门禁：`go vet`；`go test -race -count=1 -p 3`（server 全部包）；`make gen-check`、`make lint`、`make knip`；vitest 710 个；`make e2e`（97 个）；笔记本的故事与 W10 `--repeat-each 3`（54 个）；交错 20–29 与级联的行为测试 `-race -count=10`，notebook 模块与 `shared` `-race -count=5`，0 失败；接好线的应用上 9 个探针请求。`make image-smoke` 没跑（作者跑）；<br>• 反向对照：Go 33 项、e2e 3 项（汇总见下） |
| 日期 | 2026-10-02 |
| 结论 | 修复后可以合并。<br>• 规则二（只对离开与停用、锁下读、只数有效的管理员与显式成员、原因按 slug 计数）、级联的写（成员行的结束时刻与结束者、`ownerless_since` 等于工作区成员身份的结束、`updated_at` 不动）、归还（只归还仍是无主、原所有者是他的；恢复原来那一行、保留 `created_at`、核对数目）、接管的三种成员行、删除无主复用删除笔记本、按 id 的加锁与锁下重判、审计表与游标分页，都与设计一致。<br>• 不变量"未删除的笔记本有效管理员与无主恰好其一"在交错、行为测试与探针的结束时都成立。<br>• M2 移交第 1 项闭合：serve、停用、命令行三处交给 workspace 的注册者逐一换成空，各有测试失败。<br>• 模块隔离、端口在消费方、组合根的转换、隐私（邮箱只给工作区管理员，日志只记 id）成立。<br>没有 Critical、Major；2 项 Minor（测试缺口）与 5 项 Nit 合并前处理（`2d711fe`），见下；8 个疑问：Q2、Q5、Q6 写进文档（Q6 另转给 M4，[移交](../../M4-pages/handoffs/M3-notebooks.md)第 4 项），Q3、Q4 加注释，Q8 补进 N8，其余同意；文档偏差全部同步 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| T1 | Minor | P2 3.6 为每个事件触发点立的"在事务内、值对、失败整体回滚"没有延伸到 P3 的触发点：接管拿不到可见性订阅者（R04）、删除无主拿不到删除的订阅者（R05）、接管、归还、删除无主吞掉订阅者的错误（R06、R07、R08），全部测试照样通过；用例测试的替身从不设 `err` | 模块根：`TestTheVisibilityTriggers` 加"接管无主笔记本"一行（成功与订阅者失败，失败时 500，成员行、无主与审计一起回滚）；`TestANotebookDeletionPassesTheSubscriber` 加删除无主一种（审计随之回滚）；新的 `cascade_test.go` 在事务里调用 `NewMembershipEnd`、`NewMembershipRestore`，订阅者读得到写入，失败时整体回滚。R04–R08 与"结束吞掉错误"各让它们失败。顺带改正：归还在订阅者失败时与别的错误一样返回 0 本，不再带出数量。R03（组合根交给结束、恢复注册者的可见性订阅者）是最后一跳，写进 [M5 的移交](../../M5-collab-editing/handoffs/M3-P2-visibility.md) |
| T2 | Minor | 审计分页没有测同一时刻的行落在页边界：游标只比 `created_at`（R01），Go 与 e2e 全部通过；一次归还 n 本就写 n 条同一时刻的 `returned`，R01 会漏掉 | `TestListAuditEvents` 按页大小 1 翻过同一时刻的两条；R01 让它失败 |
| N1 | Nit | 无主清单的次序没有被区分：改为只按 `id`（R02）全部通过，先成为无主的恰好 id 也较小 | `TestListOwnerless` 让先成为无主的一本 id 较大（并核对这个前提）；R02 让它失败 |
| N2 | Nit | "归还保留 `created_at`"只有 e2e N11 守着（R09） | 仓储测试的 `memberState` 加 `CreatedAt`，核对归还前后相同；R09 让它失败 |
| N3 | Nit | `LockOwnerlessOf` 的行锁与次序没有测试（R11）；M3 里它是冗余的（Q6） | `TestLockOwnerlessOfLocksThemByID`，照 `TestLockHoldingsLocksThemByID`；R11 让它失败 |
| N4 | Nit | 英文文案仍是单数（"a notebook's only admin… delete the notebook"），它也用于离开工作区与停用，可能涉及多本 | 改为"You are the only admin of one or more notebooks. In each one's settings, make another member an admin first, or delete it." |
| N5 | Nit | `app/extension.go` 一处注释超长；`profilesOf` 与 `withProfiles` 重复"缺资料即故障" | 注释重排；`withProfiles` 改用 `profilesOf` |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| Q1 | 作者已定的偏差 1–9 | 审查者都同意，并补充：`gone-nb` 走与"从未存在"相同的 `FindNotebook` 路径，真正不存在的 id 由 `TestOwnerlessByIDRefusals` 守着；交错 21 只有持工作区行才排得出先后；复做 R33 失败的正是 24、25、28；命令行的计数指针每次新建、`WithinTx` 不重试、出错只打印错误 |
| Q2 | 账户既是工作区唯一的管理员、又是笔记本唯一的管理员时，先答 `workspace.sole_admin`，解决之后才见 `notebook.sole_admin`（探针 P5） | 接受：工作区自己的规则在否决者之前。P3 3.2 写明，P5 的对话框按此设计 |
| Q3 | 组合根的 `Voluntary` 正向列举离开与停用：将来新增的 `EndCause` 默认按移出处理 | 加注释：新原因在这里决定 |
| Q4 | 矩阵种子里 `orphan` 的原所有者仍是 lab 的有效成员，级联产生不了这种状态 | 对现有各行无害；种子旁加注释 |
| Q5 | 审计列表的次序：游标（400）→ 工作区与判定（404/403）→ 页大小（422）；局外人带坏游标、slug 不存在都答 400 | 不泄露工作区是否存在，契约也写"先于一切"；改 P3 3.4。游标不绑定工作区，拿 A 的游标翻 B 只是一个位置，无害 |
| Q6 | `LockHoldings`、`LockOwnerlessOf` 的笔记本行锁在 M3 的写路径下都是冗余的（结束、恢复已持工作区行，别的笔记本写都要工作区行）；`LockHoldings` 在等锁之后计数，用的是语句开始时的快照，靠工作区行才可信 | 保留（M3 总设计第 8 节，为 M4 只锁笔记本行的写准备）；第 8 节写明计数靠工作区行，M4 若加只锁笔记本行、又改成员行的写，要重做这一推理 |
| Q7 | 归还的审计执行者取事件的 `By`，改成 `UserID` 是等价变异（两条恢复路径的 `By` 都是本人） | 与 P3 3.2 一致，不改 |
| Q8 | "停用通过并留下无主"只有 Go 的行为测试守着（E3 让 N7 失败、N8 通过） | N8（接口）让账户在另一个工作区有一本只有自己的笔记本，停用之后核对它无主、成员行由本人结束 |

## 文档与代码的不一致

- D1：3.7 写审计在种子里是空页；种子实际写了一条（`gone-nb` 被删除），矩阵核对它。
- D2：3.4 的游标载荷是数组 `[created_at, id]`；游标"先于一切"判断（Q5）；M3 总设计第 5 节"结构在边界"——解码在用例里做，答复相同。
- D3：3.1 的文件清单漏列查询、仓储、用例、HTTP、workspace 的 `WorkspaceSlugs`、`schema_test.go`、`sqlc.yaml`、e2e 的夹具与 W10，以及各层的测试。
- D4：S2 计划第 8–9 行：否决者与订阅者是同一个类型的两个方法；`NewMembershipEnd(pool, workspaces, subscribers)`；"值逐字段相同"只对恢复成立，结束以 `Voluntary` 代替 `Cause`。
- D5：S3 计划第 17、19 行关于清单次序与同一时刻分页的说法不实（T2、N1）。
- D6：偏差 1、6、7、8、9 同步进 3.5、3.7、3.8、S4 计划与 M3 总设计第 9 节。
- D7：3.2 没写工作区自己的规则先于规则二（Q2）。
- D8：合并时更新本文第 7 节、M3 总设计第 7 节（移交第 1、2、3 项）、第 11、12 节；游标封套留到 M3 收尾补进总体设计第 13 节。

全部处理：P3 文档 3.1–3.9、第 5、7 节，S2–S5 计划的注记，M3 总设计第 7、8、9、11、12 节，M5 的移交；13.1 的游标封套留到 M3 收尾。

核对过、与实现一致的：规则表四行；按 id 的加锁次序与"判定的 403 答 404"；`LockHoldings` 的范围与按 id 的次序；设置、清除无主不碰 `updated_at`；`RestoreAdmins` 只认已结束的行并核对数目；`LockOwnerlessOf` 过滤已删除的笔记本（删除无主之后 `former_owner_id` 仍在，没有这个过滤归还会因数目不符整体失败，交错 28 与 `TestATakenOverOrDeletedNotebookIsNotReturned` 守着）；迁移、约束、索引名与 RESTRICT；清理器的 `SKIP LOCKED` 与次序；运行时授权；没有笔记本时也删审计；游标的严格拼法与页大小；四个操作的码与结构、三处契约描述、`reactivate-member` 的输出；与 `users set-email`、停用之间的外键锁不成环（M2 移交第 3 项）。

## 反向对照

会失败的（审查者 Go 22 项、e2e 2 项）：

- 规则二与级联：规则二不看别的成员；移出也算自愿；每个管理员的笔记本都设无主；订阅者只取第一个工作区；移出时不发可见性；`LockHoldings` 去掉行锁（存储测试）；不限工作区。
- 归还与清单：不核对恢复的行数；清单去掉无主的过滤。
- 接管与删除：接管不清除无主；判定的 403 原样答出（用例、交错 29、矩阵多格）；锁下用锁前读到的笔记本判断无主（交错 24、25、28）；接管保留原来的较低角色；接管的回答仍带无主。
- 审计：游标在判定之后判断；只在有笔记本时删审计；最后一页也给 `next_cursor`。
- M2 移交第 1 项的每一跳：serve 不交结束的否决者、结束的订阅者、恢复的订阅者；停用拿不到结束的注册者；命令行拿不到恢复的订阅者。
- e2e：归还改写 `created_at`（N11）；自愿结束时不设无主（N7）。

改了却没有失败、已由上面的修复补上的：R01、E2（T2）；R02（N1）；R04–R08（T1）；R09（N2）；R11（N3）。R03 写进 M5 的移交；R20 是等价变异（Q7）。

作者的反向对照里，S3 计划所称"漏掉 `id` → 同一时刻的分页测试失败"审查时不成立（T2），修复之后成立；其余审查者复现了。
