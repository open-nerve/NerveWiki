# M3/P3/S2 级联：实施计划

上级：[P3 文档](../03-P3-cascade-ownerless.md) 3.2、3.5、3.6；[M2 移交](../handoffs/M2-workspace.md)第 1、2、3 项。

## 任务

1. 存储：按账户与一组工作区，`FOR NO KEY UPDATE`、按 `id` 升序锁住他有有效成员行的未删除笔记本，带各自的有效管理员数与别的有效显式成员数；结束这些成员行；设置无主；按原所有者锁住一个工作区里他名下的无主笔记本；清除无主；恢复他的行为 `admin`；插入审计。
2. `notebook/app/cascade.go`：`MembershipEnd` 一个类型两个方法（实际）：`VetoMembershipEnd`（只对 `Voluntary`）、`MembershipEnded`（结束、无主、每个工作区一次可见性）；`MembershipRestore.MembershipRestored`（归还、审计 `returned`、可见性，返回数量）。
3. 模块根 `cascade.go`：`NewMembershipEnd(pool, workspaces, subscribers)`（实际：规则二的原因经 `WorkspaceSlugs` 读 slug）、`NewMembershipRestore(pool, subscribers)` 与它们的值类型（恢复的与 workspace 的逐字段相同；结束的以 `Voluntary` 代替 `Cause`，组合根转换）。
4. 组合根：`workspaceRegistrantsWith` 加结束的否决者与订阅者、恢复的订阅者；按 `EndCause` 转换为 `Voluntary`；`workspaces reactivate-member` 的输出带归还的数量（计数的包装）。
5. 契约：`leaveWorkspace`、`deactivateMe` 的 `x-problem-codes` 加 `notebook.sole_admin`；前端文案改为不指某一个笔记本。

## 测试

- 用例（假的存储）：否决只看离开与停用；无主只给他是唯一有效管理员的笔记本；可见性每个工作区一次；归还的值与审计。
- 存储（真实数据库）：锁的次序与范围（只锁他是有效成员的、未删除的）；设置、清除无主不改 `updated_at`；审计的列。
- 行为测试，整个程序（`bootstrap/notebook_cascade_test.go`）：移出（成为无主、不成为）、离开（拒绝、通过）、自助停用（拒绝、通过）、`users deactivate`（拒绝、通过）、接受邀请恢复（归还）、`reactivate-member`（归还，输出的数量），每条结束时 `checkNotebooks`。
- 反向对照：移出也否决、离开或停用不否决（转换写错）→ 行为测试失败；不注册结束的订阅者、恢复的订阅者 → 行为测试失败；计数不累加 → `reactivate-member` 的测试失败；无主只给有别的成员的 → 用例失败；以成员回来不告知可见性 → 用例失败；锁不按 id、计数算上已结束的 → 存储测试失败；设置无主改了 `updated_at` → 存储测试失败；归还不要求已结束的行 → 存储测试失败。

## 完成检查

`make check`、`make gen-check` 为绿；M2 的测试与交错全部保留。
