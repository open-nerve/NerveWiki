# M2/P3 邀请与带邀请注册：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m2-p3-invitations`（`main...8ea051f`，S1–S5，109 个文件），对照 [03-P3-invitations.md](../03-P3-invitations.md)、各 Step 计划、[M2 总设计](../00-M2-design.md)第 4、5、8、9 节、[P2 文档](../02-P2-workspace-members.md)第 7 节与 [P2 审查记录](P2-workspace-members-review.md)、总体设计 13.1、13.4 |
| 审查方式 | 独立审查者在 `git archive` 的快照上实测，仓库与 Docker 的容器都没动过：<br>• 门禁：`go test -race -p 3`（38 个包）、lint、knip、tools 的测试、vitest 446 个、`make build-web`、`make gen-check`、`make e2e`（58 个，快照 `git init` 过）；W4–W9 `--repeat-each 3`，24 次全部通过；交错 1–8 `-race -count=10`，160 个子测试全部通过；令牌的已知答案用 Python 的 hmac 独立复算一致。`make image-smoke` 会覆盖本地镜像标签，没跑（作者跑过）；<br>• 反向对照：在同一份快照上逐项改代码、跑测试、复原，Go 22 项、e2e 5 项（汇总见下） |
| 日期 | 2026-10-01 |
| 结论 | 可以合并。<br>• 写操作的顺序：锁 → 判定 → 检查 → 写。<br>• 加锁顺序：users → workspaces → workspace_invitations → workspace_members。<br>• 接受与删除邀请都在锁下重读邀请；成员关系结束与工作区删除连带邀请，同一个时刻。<br>• 提交之后没有会失败的步骤；错误码先后 404 → 403 → 422；令牌在任何查询之前核对；日志里没有令牌与邮箱。<br>• 交错 4–8 的先后由 PostgreSQL 的锁队列决定，是确定的。<br>• 不会死锁（Q1）。<br>• 27 项反向对照全部失败，没有找到测试缺口。<br>没有 Major、Minor，5 项 Nit 合并前全部处理（`81f7f26`）；5 个疑问：Q1 的外键一句改进 M2 总设计第 8 节，Q4 交给 P4，其余不改；文档偏差全部同步 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| T1 | Nit | "恢复的订阅者失败"的用例测试断言 `!slices.Contains(calls, "AcceptInvitation")`，永远成立：fake 记的是 `"AcceptInvitation by … at … in tx"`。目前由 `rolledBack` 与另一个测试的调用顺序守住，这句不承重 | 改为按前缀匹配。反向对照：把消费邀请挪到恢复之前，这个测试失败 |
| T2 | Nit | 整个程序的注册测试里，`tampered` 只改令牌的最后一个字符。测试密钥与邀请 id 每次随机，末位恰是 `A` 时（约 1/4）改成 `B` 只动未用的位，测到的是严格解码，不是"令牌不对"：去掉 Strict 时这个测试约 1/4 的运行失败，不稳定 | 改中间的字符，与 W6 的 `tampered` 相同。严格解码由 `TestTokens` 的"its unused bits set"守住 |
| T3 | Nit | 契约中 deleteWorkspace、leaveWorkspace、removeWorkspaceMember 的描述折行参差；`delete_workspace.go` 有一行注释约 120 列 | 重新折行（折叠标量，生成物不变） |
| T4 | Nit | workspace 模块根的包注释没有列出 `NewInvitationCheck`、`InvitationKeyInfo` | 补上 |
| T5 | Nit | `app.InvitationCheck`（结构体）与模块根的 `workspace.InvitationCheck`（接口）同名而种类不同 | app 的改名 `CheckInvitation`，与包里用例的命名一致，文件名本就是 `check_invitation.go` |

## 疑问与判断

| # | 疑问 | 判断 |
|---|---|---|
| Q1 | 会不会死锁 | 不会：<br>• 同一个账户上，接受以账户行的 `FOR SHARE` 开头，改邮箱、停用以 `FOR NO KEY UPDATE` 开头，在同一行排队（交错 8 两种先后）。<br>• 工作区一支插入引用账户的行（如 `updated_by_id`）时，外键检查对账户行取 `FOR KEY SHARE`。审查者的探针：改 `display_name` 不挡它，改 `email` 挡它（有唯一约束的列，取 FOR UPDATE 级的行锁）。<br>• 改邮箱只碰账户与会话，从不等工作区一支的锁，所以只是等待，不成环。<br>M2 总设计第 8 节"外键检查与停用不冲突"一句对改邮箱不成立：已补上改邮箱的情形。P4 若让改邮箱或停用进入工作区一支，重新核对（P3 文档第 7 节） |
| Q2 | 接受时，账户已停用的 403 排在工作区已删除的 404 之前，看起来与"404 → 403"不一致 | 不改：`ShareActiveAccount` 必须是第一条语句（13.1 第 18 条），这个 403 说的是调用者自己的账户，不泄露资源。与 P1 的创建工作区相同 |
| Q3 | 邀请行的 `FOR UPDATE` 有没有必要 | 有必要：接受与删除邀请已由工作区锁串行，行锁实际只在两位管理员同时撤回同一份邀请时起作用（两边都持 `FOR SHARE`）；去掉它，仓储的两个锁测试失败。不加专门的交错 |
| Q4 | P4 拆 `MembershipEnder.End`（P2 审查 Q1）时，删邀请放在哪一步 | 写的那一步：停用持有账户行，那条路上读到的邮箱是稳定的。交给 P4（P3 文档第 7 节） |
| Q5 | `InvitationUpdater` 有 6 个方法，使用者各用 1–3 个 | 不改：与 P2 的 `MemberUpdater` 同一风格，端口是一组用例要的 |

## 文档与代码的不一致

作者已知的 10 处，审查者逐条核实，都合理：

1. `accepted_check` 加上 `deleted_at IS NOT NULL`：文档的写法在 `deleted_at` 为空时放行，审查者把 CHECK 改回文档的写法，迁移的反例测试失败。
2. 签名密钥前移到组合根：环是真的。接线干净：坏私钥在连数据库之前报错（`bootstrap/auth_test` 守住）；组合根只能调用 `Derive`；邀请密钥只经 `Deps.InvitationKey` 与 `NewInvitationCheck` 进入 mac 适配器；共用一份私钥的两个程序互认对方的令牌（`TestSignupClosedOpensToAnInvitation`）。
3. 用例文件按"动词_名词"命名；4. 端口由使用方声明（见 Q5）；5. 严格解码；6. `byCredential`；7. 交错 7 的触发器：测试自己先插入同一个键时，两个插入醒来之后谁先是竞争，触发器让先到的插入在索引里已有自己的键；8. "整个程序"的测试挪到 S3；9. `interleave` 泛化，P2 的三个交错照样通过；10. W6、W9 经 `users set-email` 达到前提状态：P3 里只有这条路。

另外 6 处：

- D1：P3 文档 3.3 与 M2 总设计第 5 节写的路径参数是 `{invitation_id}`，契约是 `{workspace_invitation_id}`。
- D2：P3 文档第 5 节"接受用请求里带的邮箱"的反向对照：请求里没有邮箱。
- D3：S1 计划第 6 项"按账户是否有效成员"实际是 `FindMembership`。
- D4：P3 文档 3 节文件表写 workspace 的 `Deps` 加"Accounts 目录"，字段实际叫 `Directory`。
- D5：M2 总设计第 8 节的外键一句（Q1）。
- D6：P3 文档第 7 节、M2 总设计的进度表与"M1 移交的落实"表。

全部处理：P3 文档 3.1、3.2、3.3、3.9、3.11、第 4、5、7 节，S1 计划第 3、6 项，M2 总设计第 4、5、7、8、11、12 节。

核对过、与实现一致的：README 的邀请与签名私钥两节、`identity.yaml` 的 `register` 与 `SignupInvitation`、`WorkspaceMember.created_at` 的描述、两种语言的文案。

## 反向对照

会失败的（审查者与作者分别做过）：

- 接受不比较邮箱（用例两项、交错 8、W6）；接受不在锁下重读邀请（用例、交错 5、交错 6）；成员关系结束不删邀请（用例三项、接好线的模块测试、交错 6、W9）；`ShareAccount` 去掉 `FOR SHARE`（交错 8 两种先后、identity 的账户行测试）。
- 规则表让成员能创建邀请（矩阵的成员格与访客格）；唯一索引去掉 `WHERE deleted_at IS NULL`；`Valid` 不比较；去掉严格解码；CHECK 照文档的写法。
- 邀请检查不看邮箱（W7 与整个程序的注册测试）；删除工作区不连带邀请（交错 4、W4）；邀请的锁去掉 `FOR UPDATE`、去掉 `deleted_at`；恢复时改 `created_at`（W6）；`DeleteInvitationsTo` 不看 `deleted_at`；接受的日志记邮箱；共享锁去掉 `FOR SHARE`；接受不锁工作区；创建不查有效成员；预览不核对令牌；已是成员时改成邀请的角色；`FindPendingInvitation` 不看工作区是否已删除。
- 作者另做：`pgtest.WaitForKeyWaitOn` 去掉元组锁或写入的过滤；注册策略丢掉邮箱、邀请检查跳过令牌；T1 的新断言。

没有找到没失败的改动。只由一层测试抓到的：共享锁的模式只由仓储测试抓到，用例是否用共享锁由用例测试的调用顺序守住。"令牌比较用 `==`"测不出来，由代码审查守住：代码用的是 `hmac.Equal`，有注释。

## 没能验证的风险

- 审查者没跑 `make image-smoke`；作者跑过，通过。持续集成为绿（`8ea051f`、`81f7f26`）。
- 审查者的 e2e 在 `git init` 过的快照上跑，与持续集成的环境不完全相同。
- P4 停用"在两支之间交替"的路径还不存在，Q1 的死锁分析只覆盖 P3 已有的路径。
- 计时侧信道无法用测试验证。
