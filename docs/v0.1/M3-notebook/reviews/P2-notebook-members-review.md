# M3/P2 笔记本成员：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m3-p2-notebook-members`（`main...7e9b94e`，S1–S4，64 个文件，+5131/−138），对照 [02-P2-notebook-members.md](../02-P2-notebook-members.md)、各 Step 计划、[M3 总设计](../00-M3-design.md)第 4、5、7、8、9 节、[P1 文档](../01-P1-notebooks-access.md)与 [P1 审查](P1-notebooks-access-review.md)、[M2 移交](../handoffs/M2-workspace.md)第 6 项、总体设计 12.4、13 |
| 审查方式 | 独立审查者在 `git archive` 的快照上实测，仓库与别的 Docker 容器都没动过：<br>• 门禁：`go vet`；`go test -race -count=1 -p 3`（44 个包与 server/tools）；lint、knip；vitest 706 个；`make gen-check`；`make e2e`（90 个，快照 `git init` 过）；笔记本的故事 `--repeat-each 3`（24 个）；交错 17–19 与组合根的转发 `-race -count=10`，notebook 与 workspace 模块 `-race -count=5`，0 失败；S1–S3 的中间提交各自 `go vet` 与 `go test -short`；接好线的应用上 7 个探针请求。`make image-smoke` 没跑（作者跑）；<br>• 反向对照：Go 50 项、e2e 3 项（汇总见下） |
| 日期 | 2026-10-02 |
| 结论 | 修复后可以合并。<br>• 规则一（锁下计数、改与移出自己 409）、按成员关系寻址的次序（不加锁读 → 工作区行 `FOR SHARE` → 笔记本行 → 判定 → 锁下重读 → 校验 → 规则一 → 写）、码的次序 404 → 403 → 422 → 409、看不到的笔记本一律 `member_not_found`、恢复保留 `created_at`，都与设计一致。<br>• 可见性事件的五个触发点在写入之后、同一事务内、值对、失败回滚；不该触发的四类不触发；workspace 的两个事件只在写入之后加了调用，M2 的测试与交错一条没删。<br>• 邮箱只给工作区的管理员与成员，日志只记 id；模块隔离成立。<br>没有 Critical、Major；1 项 Minor（测试缺口）与 4 项 Nit 合并前处理（`84aba1e`），见下；6 个疑问：Q1 写进 M5 的移交，Q2 写明例外，Q3、Q6 同意，Q4 由 P3 解决，Q5 留给 P4；文档偏差全部同步 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| T1 | Minor | 成员用例在读资料失败、缺资料时的路径没有测试：缺资料时不报错（P01）、添加吞掉 `withProfiles` 的错误（P02），全部测试照样通过；`fakeProfiles.err` 从未被设置。资料在事务里读，就是为了失败时回滚、不在提交之后答 500（13.1 第 19 条） | `TestMemberProfilesFailing`：添加、改角色在资料读取失败与缺资料时答出错误并回滚；列表同样答出错误。P01、P02 各让它失败 |
| T2 | Nit | 组合根转换资料时只核对了邮箱：转换丢掉显示名（C07），Go 与 e2e 全部通过。矩阵的 `notebookMemberAnswer` 没有 `display_name`，N4 用 `expect.any(String)` | 矩阵的成员行核对显示名（默认是邮箱的本地部分，照工作区成员的行）；N4 断言访客的显示名。C07 让矩阵的 `listNotebookMembers` 各格失败 |
| N1 | Nit | 偏差 6（不在工作区时不再报"已是成员"）没有测试：把 `else if` 改为独立的 `if`（N07），全部通过；P3 之前离开工作区的人仍是笔记本的有效成员，这个组合可达 | `TestCheckAddition` 加"不在工作区、仍是有效成员"一行；N07 让它失败 |
| N2 | Nit | 整个程序上没有"管理员改、移出一个已结束的成员关系"：`FindActiveMember` 不看 `ended_at`（R05）只有仓储测试失败 | N4 移出访客之后再发一次改角色与移出，断言 404 `notebook.member_not_found`，落库的行不变 |
| N3 | Nit | `MemberWriter` 的注释只说"改"，它也承载锁下的读；添加、离开的 `Deps` 把写端口叫 `Members`，改角色、移出叫 `Writer` | 注释改为"reads and changes … under its lock"；添加、离开的字段改名 `Writer` |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| Q1 | 组合根的最后一跳没有测试：去掉 `deps.go` 交给 workspace 的两组订阅者、交给 notebook 的可见性订阅者，全部通过。`TestTheWorkspaceMemberEventsReachTheVisibility` 只证明到 `workspaceRegistrantsWith`。M3 没有可见性的订阅者，与 P1 审查 Q3、M2 移交第 1 项同类 | 写进 [M5 的移交](../../M5-collab-editing/handoffs/M3-P2-visibility.md)：M5 的第一个订阅者要在整个程序上经每条触发路径（P2 与 P3 的）各有行为测试；`Reached` 为真时 `UserIDs` 可以为空；订阅者被调用之后事务仍可能回滚，对外的效果随提交生效 |
| Q2 | `VisibilityChange` 没有执行者；13.1 第 21 条说"事件的值带时刻与执行者"，M3 总设计第 4、8 节的值本来没有 | 不加：M5 关事件流用不到。M3 总设计第 8 节写明这个例外，M5 的移交第 4 点；13.1 第 21 条留到 M3 收尾一起改 |
| Q3 | notebook 的 `lockMember` 先判定、后重读，workspace 的先重读、后判定：成员关系在不加锁读与加锁之间结束时，笔记本里有角色的非管理员得到 403，工作区的同类答 404 | 同意，不改：两种次序都不泄露（有角色的人本来就能列出成员）。P2 3.5 补一句理由 |
| Q4 | "笔记本成员 ⊆ 工作区有效成员"只在添加这一侧成立：P2 合并之后、P3 合并之前，离开工作区的人仍是笔记本的有效成员（判定挡住他，但他仍在成员列表与 `member_count` 里，回来后再加答 `duplicate`，唯一的管理员离开后笔记本账面上仍有管理员） | 按 Phase 划分由 P3 的成员身份结束的订阅者解决，v0.1 还没有发布；P3 的交错 20、26、27 覆盖。P2 的修复合并进 main 后，P3 分支变基 |
| Q5 | `notebook.member_not_found` 的文案用在 `leaveNotebook` 上不贴切：离开时它的意思是"只靠默认角色使用，不是显式成员" | 留给 P4：只给显式成员显示"离开"，或离开的对话框另给文案 |
| Q6 | 作者已定的偏差 1–7 | 审查者都同意，补充的三点见 Q1、N1、Q1 |

## 文档与代码的不一致

- D1：3.1 的文件清单：workspace 模块根的测试是 `member_events_test.go`；notebook 模块根的是 `visibility_test.go`；矩阵的行在 `permission_matrix_notebook_members_test.go`；交错在 `interleavings_notebook_members_test.go`（S3 计划同）；漏列 app、domain、HTTP、仓储的测试与 workspace 的测试改动。
- D2：3.7 的查询名：代码是 `ListMembers`、`FindMemberOf`、`CountAdmins`。
- D3：3.4 的资料端口是 `MemberProfiles.MemberProfiles`（偏差 4）；S2 计划第 5、7 项同。
- D4：S2 计划第 3 项漏了 `CheckAddition`、`CheckLeave`。
- D5：偏差 3、5、7 进 3.6，偏差 1、2 进 3.8。
- D6：M3 总设计第 8 节把工作区成员关系结束、恢复的可见性交给 M3 的注册者，第 7 节 P3 一行与 P2 的"不做"没写。
- D7：合并时要更新 P2 第 7 节、M3 总设计第 7、11、12 节；13.1 第 11 条的例子补 `bootstrap/notebook_profiles.go`，第 21 条的例子补 `notebook.NewWorkspaceMemberEvents`（与 P1 留给收尾的同一处）。

全部处理：P2 文档 3.1、3.4、3.5、3.6、3.7、3.8、第 2、7 节，S2、S3 计划的偏差注记，M3 总设计第 7、8、11、12 节；13.1 的两项留到 M3 收尾。

核对过、与实现一致的：规则表的五行；按成员关系与按笔记本寻址的加锁次序；规则一；404 不泄露（矩阵 19 格守着）；添加的三类问题一次列出；恢复与离开；可见性的五个触发点与四类不触发；workspace 的两个事件；隐私（访客当上笔记本管理员时，添加的答复里 email 是 `null`）；模块隔离与值在组合根转换；契约五个操作的码与结构；三个新码的中英文案；交错 17–19 结果确定。

## 反向对照

会失败的（审查者做 Go 44 项、e2e 2 项）：

- 交错：离开时在加锁之前数管理员（18 两种先后与用例）；成员操作先判定、后加锁（17 两种先后变成 200/200、0 位管理员，不变量失败）；添加不锁笔记本行（19 在握手处超时）。
- 规则一与添加：`admins < 1`；允许改、移出自己；不核对工作区成员关系；去掉重复检查；码的次序颠倒；已结束的还能再离开；成员操作答 `notebook.not_found`（用例与矩阵 19 格）；不在锁下重读。
- 恢复：改写 `created_at`（仓储、e2e N4）；答复的 `created_at` 用新时刻。
- 转发与可见性：访客加入也转发、管理员↔成员也转发；移出在提交之后通知、开放程度在写入之前通知；建笔记本不带 `Reached`、每次改开放程度都通知、离开时不带账户。
- workspace 的两个事件：在写入之前、同角色也通知、恢复也当作加入、忽略订阅者的错误。
- 接线：workspace 模块根不交角色变化的订阅者、notebook 模块根的添加不交可见性订阅者、组合根不交加入事件的注册者、资料转换丢掉邮箱。
- 邮箱：访客也看得到、成员看不到。
- 仓储：列表不看 `ended_at`、去掉 `id` 的次要排序；管理员计数不看 `ended_at`、不看角色；结束成员关系不写 `ended_at`（仓储、模块根、交错 18、e2e N4、N5）；`FindActiveMember`、`FindMemberOf` 的过滤（仓储）。
- 规则表：编辑者也能添加、只有管理员能离开、只有管理员能列出（矩阵各 5 格）；覆盖检查：矩阵的行指向别的工作区的成员行。

改了却没有失败、已由上面的修复补上的：P01、P02（T1）；C07 与 e2e 的 E3（T2）；N07（N1）；R05 在整个程序上（N2）。C01、C02（Q1）写进 M5 的移交。

作者在审查修复中做的：P01、P02、C07（Go）、N07，各见上表。
