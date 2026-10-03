# M5/P1 编辑锁：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m5-p1`（`465c051..85e3437`：5 个提交，S1 `a126b03`、S2 `e7d8838`、S3 `f19d1a5`、S4 `e175856`、S5 `85e3437`；77 个文件，+3759/−313），对照 [01-P1-edit-lock.md](../01-P1-edit-lock.md) 第 1–6 节、五份 Step 计划、[M5 总设计](../00-M5-design.md)第 3、4、9 节，以及作者的偏差说明（as-built 笔记，8 条）与四组反向对照（S1+S2 28、S3 4、S4 8、S5 e2e 4 项） |
| 审查方式 | 两位独立审查者（Opus）在仓库上只读：A 看正确性、并发与接口行为，B 看测试的质量与覆盖、代码与契约和文档的一致。A 跑了 page 模块的全部测试；想用 `go test -overlay` 跑探针复现 I1 时被权限拦下，I1 是读 SQL 与代码得出的，没有实测。B 跑了 page 的 app、domain、shared、httpserver、HTTP 适配器的单元测试、模块根与仓储的锁测试、bootstrap 的锁交错与最后一跳，交错 `-count=5`、交错 49 `-count=10` 连跑，都没有不稳定。修复之后另由一位 Opus 核对修复（见"修复的核对"） |
| 日期 | 2026-10-03 |
| 结论 | 没有阻断合并的问题，核心不变式"一页至多一个活着的会话"在任何时钟偏差下都成立，加锁次序没有成环。合并之前修 A 的 Important I1（结束的时刻早于会话的开启时违反表的检查，接管或强制解锁答 500）与 Minor m1、m2（B 把 m2 记为 Important，即 B1），其余随手处理。A：Important 1、Minor 3、Nit 1；B：Important 1（同 m2）、Minor 10、Nit 8 |

## 发现与处置

A 的编号是 I1、m1–m3、n1，B 的是 B1–B12 与 Nit。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| I1 | Important | **接管或强制解锁读时钟早于会话开启时答 500**：`00018` 的检查要求 `ended_at >= created_at`，`EndAliveSessions` 写 `ended_at = at`；单元在笔记本锁之后、取正文行之前读时钟，两个单元读时钟的先后与取正文行的先后可以相反：解锁读了 `At_R`，别人的开启读了更晚的 `At_O`、先取正文行提交，解锁再把这个会话变成墓碑，`ended_at < created_at`，SQLSTATE 23514，500。同一账户两个标签页的开启与接管、墙钟回拨、多实例的时钟偏差同样触发；交错的测试工具总是先启动第一个请求，造不出这个次序 | 已修：`ended_at = greatest(at, created_at)`，租约照旧至少到结束之后一个租约（开启时 `expires_at >= created_at + 租约`）；告诉订阅者的结束时刻同样取两者中较晚的。仓储测试 `TestATombstoneEndsNoEarlierThanItsOpening`（开启在 now+1s、解锁在 now）、用例测试（解锁读时钟早于开启，告知与墓碑都在开启时刻）；FIX-I1、FIX-TOLD-BEFORE-OPENING 失败 |
| m1 | Minor | **强制解锁不删过期的行，迟到的心跳能救活解锁跳过的会话**：总设计 4.1"迟到的心跳续不了刚过期的会话"只在开启之后成立。bob 的会话在 E 到期，节流的标签页的心跳读到 T0 < E；管理员在 T1 ≥ E 解锁，会话在 T1 不算活着，什么也没结束，答 204；心跳随后把租约续到 T0+120，锁又回来了，bob 也不知道被解锁过。不带会话的写正文同样（令牌在 T1 写进去，之后心跳救活会话，bob 下一次保存答 `revision_mismatch`） | 已修解锁：取正文行之后、结束活着的会话之前先 `DeleteExpiredSessionsOf`，与开启相同。用例测试核对调用次序与过期的行删掉；模块根的 `TestALateHeartbeatFindsItsRowDeletedByAnUnlock`（与交错 49 同法：订阅者在删除之后拦住解锁，晚一分钟的时钟发心跳，等在会话行上，放行之后心跳 404、行不在）；FIX-UNLOCK-KEEPS-EXPIRED、FIX-UNLOCK-KEEPS-EXPIRED-DB 失败。不带会话的写正文不改，记在 P1 第 7 节：后果是会话的下一次保存答 `revision_mismatch`，编辑器按冲突处理，不丢字 |
| m2 = B1 | Minor（B 记 Important） | **墓碑的心跳不经授权就答原因**：`notAlive` 在 `WorkspaceOf` 与授权之前答 `taken_over` 或 `unlocked`，与契约"在笔记本里没有角色了答 `page.edit_session_not_found`"不符：被移出笔记本的人从 `ended_by` 得知管理员的名字，只能读的人答 409 而不是 403；同一会话里的保存却因单元先判定而答 404 | 已修（B 的方案 A）：墓碑先判定笔记本（`decide`：不加锁读工作区，授权 `page.edit`，看不到答 `page.edit_session_not_found`），再答原因；用例测试加"笔记本看不到"与"阅读者"两格；FIX-TOMBSTONE-BEAT-UNDECIDED 失败。契约的心跳描述改写 |
| m3 | Minor | **M4 的网页编辑器刷新之后最多 120 秒保存不了**：`page-edit.tsx` 只在卸载时结束会话，`pagehide` 的释放在 P3；刷新或第二个标签页时旧会话还活着，新的开启答 `page.locked`（持锁人是自己），自动保存一直失败。总设计第 3 节接受 P1 到 P3 之间的"通用错误"，但刷新是日常操作。建议 P3 之前不发布 `main`，或者让 M4 的编辑器在持锁人是自己时带 `take_over` 重试一次 | 不改，记为 P1 到 P3 之间的已知差异（P1 第 7 节）：v0.1 不在 P3 之前发布；自动带 `take_over` 重试会让两个标签页互相接管。P3 的离开释放与"在这里编辑"解决它 |
| n1 | Nit | 显示名查不到时静默给 `""`（`edit_lock.go`、`unit_session.go`、`get_edit_lock.go`）；契约允许，外键加上用户从不硬删除使它到不了 | 已改：`Names` 端口的注释写明查不到时名字为空，会话的 id 引用的用户从不删除 |
| B2 | Minor | 交错 47、48 的"墓碑没有被延长"检查（`expires_at > ended_at + 121 秒`）不会失败：结束在先时心跳只晚几十毫秒，延长了也到不了 +121 秒 | 已改：核对 `expires_at = ended_at + 120 秒`；FIX-TOMBSTONE-LEASE-DOUBLED 失败 |
| B3 | Minor | 单元测试的同类空检查：`Alive(now())` 对任何墓碑都是假 | 已改：与原来的 `ExpiresAt`、`EndedReason` 比较 |
| B4 | Minor | `session_test.go` 的新测试插在清理测试与它的注释之间 | 已改 |
| B5 | Minor | 模块根的订阅者只记最后一次调用是否在事务里 | 已改：每次调用都记，有一次不在事务里就不算 |
| B6 | Minor | 仓储测试依赖 `UPDATE … RETURNING` 的未规定次序；假的端口按开启排序，SQL 不排 | 已改：按集合比较，假端口的注释写明排序只为测试 |
| B7 | Minor | `expires_in` 的取整只测了一半（剩 90.001 秒得 91，错误的 `+1` 也得 91） | 已改：加"恰好剩 90 秒得 90"；FIX-READ-LOCK-ROUNDS-UP-WHOLE 失败 |
| B8 | Minor | 交错 45、46 只核对状态码：保存在先时没核对写进了旧会话的变更集，接管或解锁在先时没核对正文仍是版本 1 | 已改：保存在先核对旧会话的 `changeset_id` 与 `revision = 2`，另一种次序核对版本 1 |
| B9 | Minor | 总设计第 9 节"租约边界 119/120 秒、墓碑保留同样"只覆盖了一部分：没有测试显示定期清理会删过期的墓碑 | 已改：仓储测试 `TestDeleteExpiredSessions` 加墓碑：结束之后一个租约差 1 微秒时清理不删，到点删 |
| B10 | Minor | "解锁不告诉观察者"没有测试 | 已改：解锁的用例测试注册观察者，核对它什么也没收到 |
| B11 | Minor | 过时或不准的注释与契约：`WriteContent` 的码漏了 `taken_over`、`unlocked`；`unit.go`、`registrants.go` 写"结束的订阅者"（现在也跟开启）；`registrants.go`"测试都从这里取"；`EndSession` 的 SQL 注释说过期的留给清理；过期而没清理的墓碑结束答 204、心跳答原因，`end_edit_session.go` 与契约却说过期答 404 | 已改：注释、契约（心跳与结束）、README 都写明墓碑在清理之前答原因、结束答 204，以及谁来清理（这一页下一次开启或强制解锁，或后台任务） |
| B12 | Minor | 四条偏差（注册从 S2 挪到 S4、矩阵的草稿页、交错 49 在模块根、PG10（页面）改写）都对，要写进第 7 节 | 已写进 P1 第 7 节 |
| Nit | Nit | `checkPages`"墓碑的三列不全"永远是 0（表的检查拒绝它）；矩阵 `e.ID == s.session(draftOf(c), c)` 恒假；`lockOf` 不看 `Lock.UserID`；整个程序的测试只拒绝过锁在子树顶上的删除；`TestTheEditLockRefusesASecondOpening` 不在 `lock_test.go`；`domain/session.go` 注释断行；README 三处（`expires_in` 也是 `null`、墓碑的结束答 204、"至少一个租约"）；子页的锁拒绝父页的删除时，`page.locked` 的文案读起来别扭 | 前七条已改（`lockOf` 核对名字是这个 id 的，FIX-LOCK-WRONG-USER 失败；最后一跳加"Notes 移到 Beside 下之后 alice 删 Beside 答 `page.locked`，指向 Notes"，FIX-DELETION-TOP-ONLY 失败）。文案留给 P4 与 v0.1 收官之后的统一打磨 |

## 修复的核对

修复（`5323f6e`）之前另由一位 Opus 只读核对：`sqlc diff` 干净，`go vet` 通过，生成的契约与 `page.yaml` 一致。结论：三处修复正确、完整，没有新的竞态（解锁的删除只删真过期的行，在授权与正文行的锁之后；清理 `SKIP LOCKED`；本人的结束不取正文行；不告诉过期的行），墓碑的心跳先判定不泄露什么。没有 Important；Minor 3、Nit 7，处置如下。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| F1 | Minor | 契约的心跳描述"先于一切"说过头：本人过期的（不是墓碑）会话在只能读时答 404 而不是 403，别人的会话不判定就答 404 | 已改：按建议改写为"本人的会话（活着，或被接管、被解锁）在笔记本里没有角色了答 404，只能读答 403；否则……" |
| F2 | Minor | m1 只有假端口的调用次序钉住，没有数据库上的测试 | 已加 `TestALateHeartbeatFindsItsRowDeletedByAnUnlock`（见 m1 一行） |
| F3 | Minor | `DeleteExpiredSessionsOf` 的注释（SQL 与端口）只说"开启的第一步" | 已改：开启与解锁的第一步 |
| F4 | Nit | 告诉订阅者的结束时刻在 I1 的竞态里早于同一会话的开启 | 已改：取单元的时刻与开启中较晚的（见 I1 一行） |
| F5 | Nit | 竞态路径上心跳判定两次 | 不改：只多两次查询，只在心跳与接管、解锁相撞时 |
| F6 | Nit | I1 的仓储测试最后的 `LockSession` 什么也没核对 | 已改：与结束返回的墓碑比较 |
| F7 | Nit | 交错 46 与 45 的失败信息没提新加的版本检查 | 已改 |
| F8 | Nit | README：过期的墓碑也由下一次开启或强制解锁删除；墓碑只在主人还能编辑时答原因 | 已改 |
| F9 | Nit | 文档的漂移：P1 文档的解锁步骤、心跳的次序、`checkPages` 的检查，总设计 4.3 的心跳一条；代码引用的"P1 审查"在 `reviews/` 里没有 | 已改：P1 文档与总设计随代码；本记录即"M5/P1 review" |
| F10 | Nit | 提交说明漏了四处测试改动 | 已补 |

## 反向对照

修复的反向对照 10 项，都失败：FIX-I1（`ended_at` 不取较晚的）、FIX-UNLOCK-KEEPS-EXPIRED（用例测试）、FIX-UNLOCK-KEEPS-EXPIRED-DB（模块根）、FIX-TOMBSTONE-BEAT-UNDECIDED、FIX-TOMBSTONE-LEASE-DOUBLED（交错 47、48）、FIX-TOMBSTONE-NO-LEASE（仓储的清理边界）、FIX-READ-LOCK-ROUNDS-UP-WHOLE、FIX-LOCK-WRONG-USER、FIX-DELETION-TOP-ONLY（最后一跳）、FIX-TOLD-BEFORE-OPENING。
