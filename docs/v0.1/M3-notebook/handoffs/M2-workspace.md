```yaml
status: open
from: M2
to: M3
created: 2026-10-01
```

# 工作区留给 M3 的部分

M2 建好了工作区的三个扩展点（成员身份结束、恢复、工作区删除）、权限规则表与判定级别、软删除的清理注册表（总体设计 12.4，[M2 总设计](../../M2-workspace/00-M2-design.md)第 8 节），但它们都还没有注册者：第一批注册者是 M3 的笔记本。下面是 M2 各 Phase 的审查与结果留给 M3 的事，写 M3 的 00 号文档时一并考虑。所有 M 都要遵守的写法（加锁顺序、扩展点的注册、清理、前端按工作区的页面与 store）在 M2 收尾时补进总体设计第 13 节，这里不重复。

1. **注册者的行为测试**：`bootstrap/registrants.go` 的 `workspaceRegistrants()` 现在返回空。组合检查（`archtest/composition_test.go`）只证明 serve 与两个命令行入口静态地到达它，不证明注册者交给了模块：M2/P4 的审查把停用注册者与 `Workspaces` 拿到的成员身份结束、恢复的注册者都换成 nil，全部测试照样通过（[P4 审查](../../M2-workspace/reviews/P4-deactivation-commands-purge-review.md) Q1）。所以 M3 第一个成员身份结束或恢复的注册者，要在整个程序上经每条路径各有行为测试：移出、离开、停用（接口与命令行）、接受邀请、`nervewiki workspaces reactivate-member`、删除工作区。
2. **`EndCause` 的常量**：模块根以类型别名公开 `workspace.EndCause`，三个值（`removed`、`left`、`deactivated`，`app.EndRemoved` 等）还没有从模块根导出，现在没有使用者（[P2 审查](../../M2-workspace/reviews/P2-workspace-members-review.md) T9）。组合根转换"成员身份结束"的值、或注册者按原因分支时，在模块根加上它们。
3. **加锁顺序**：M3 的行接在工作区一支的 `workspace_members` 之后（总体设计 13.1 第 5 条）。成员身份结束、恢复、删除的订阅者在工作区行已锁住的事务里运行；订阅者若写引用别的账户的列（外键检查取 `FOR KEY SHARE`），复核它与 `users set-email` 的 FOR UPDATE 级行锁：现在不成环，因为改邮箱只碰账户与会话（[P4 审查](../../M2-workspace/reviews/P4-deactivation-commands-purge-review.md) Q6）。
4. **清理器**（`jobs.Purger` 的契约，总体设计 13.1 第 6 条；`bootstrap/registrants.go` 的 `purgers(pool)` 从叶到根排列，笔记本的排在工作区之前）：
   - 清理器跳过别的事务持有的行；同一模块之内，还跳过仍被之前的清理器跳过的行引用的行：父行的 `ON DELETE CASCADE` 不能连带删除子行，否则会等锁（[P4 审查](../../M2-workspace/reviews/P4-deactivation-commands-purge-review.md) T1；工作区的清理器以 `NOT EXISTS` 等子行先清掉）。
   - **跨模块的外键用 `ON DELETE RESTRICT`**：工作区的清理器看不到笔记本的表（sqlc 按模块限定），写不出 `NOT EXISTS`。所以 `notebooks → workspaces` 这类外键用 RESTRICT，不用 CASCADE：每一行只由它自己模块的清理器删除；工作区仍被引用时，这一批删除失败、清理停下，River 重试，笔记本清掉之后完成（[M2 收尾审查](../../M2-workspace/reviews/M2-closeout-review.md) A-I1）。第一条跨模块外键到来时，在 `TestPurgersComeBeforeTheTablesTheyReference` 旁边加一条检查：指向被清理表、来自别的模块迁移的外键不是 CASCADE（`confdeltype <> 'c'`，模块取自迁移文件名）。
   - `TestPurgersComeBeforeTheTablesTheyReference` 要求引用被清理表的每张表都有自己的清理器：只靠 CASCADE、没有 `deleted_at` 的子表过不了它，M3 加这样的表时决定是给它清理器，还是在测试里写明豁免（P4 审查 Q2）。
   - "失败即停"：一个永久失败的清理器让之后的都不跑，只在日志里看得到（P4 审查 Q2）。表多了之后要不要改成跳过失败的继续，在 M3 定。
   - 表上没有 `deleted_at` 的索引，每批顺序扫描；River 的任务期限 1 分钟，超时取消重试，已删的批不丢（P4 审查 Q3）。v0.1 的规模不需要；笔记本或页面的表大到一批扫不完时，加部分索引或调长清理的期限。
5. **前端**：
   - 让整页离开的操作之后的焦点（删除、离开工作区，接受邀请进入工作区：触发的按钮都已不在），以及左栏用 `aside` 承载主导航的语义，与页面树一起设计（[P5 审查](../../M2-workspace/reviews/P5-web-shell-workspaces-review.md) T11，M2 收尾审查 B-N5）。
   - 新手引导的第二个注册者（第一个个人笔记本，总体设计 12.4）：步骤 id 同时加进 e2e 的 `onboardingSteps`（`e2e/fixtures/auth.ts`），否则 `registerOnboarded` 的账户会落到引导页（M1 移交第 5 项，P5 照此办理）；A9 断言的步数随之改变。
6. **加入与改角色没有事件**：总体设计 12.4 的可见性变化事件由 M3 建立，M5 用它关闭看不到的笔记本的事件流，触发源包括成员身份。M2 只为结束、恢复、删除建了扩展点：接受邀请插入新行时（`app/accept_invitation.go` 的 `joinAdded`）不调用订阅者（只有恢复已结束的行才调），改角色（`app/update_member.go`）也没有订阅者；而降为访客会失去 `workspace_access` 给的默认角色（总体设计 3.3）。M3 设计可见性事件时决定是否按 12.1 第 6 条的例外，在这两条路径上加事件，加的话同时修订 12.4；现在不建，因为还没有使用者（M2 总设计第 4 节"角色变化没有事件"，[M2 收尾审查](../../M2-workspace/reviews/M2-closeout-review.md) C-M1）。
