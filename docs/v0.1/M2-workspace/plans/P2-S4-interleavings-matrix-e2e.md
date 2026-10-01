# M2/P2/S4 交错、矩阵与端到端：实施计划

上级：[P2 文档](../02-P2-workspace-members.md) 3.9–3.11。

## 任务

1. `bootstrap/interleavings_workspace_test.go`：照 3.9，测试持锁、按先后发出两个请求、`WaitForLockWaitsOn` 确认、提交；三个交错，各两种先后；之后核对数据库。
2. 矩阵：
   - 种子：已删除的工作区里另种一个成员；"已结束的成员"经 `removeWorkspaceMember`、"已删除的工作区"经 `deleteWorkspace` 准备；
   - 六个操作的行，外加"改自己""移出自己"；成员列表的行用 `check` 核对显示名、邮箱（访客为 `null`）；
   - `matrixExempt` 不变：没有新的公开操作，`{workspace_member_id}` 由 `{…_id}` 的分支核对。
3. e2e：`fixtures/workspaces.ts` 加改名、删除；`fixtures/assert/workspace.ts` 加软删除的断言；`stories/workspace/w4-rename-delete.spec.ts`（接口版本）。

## 测试

- 交错的反向对照：离开与改角色先判定后加锁，交错 1、2 失败。
- 矩阵的反向对照：规则表的 `workspace_member.update` 加上成员，矩阵失败；删除不连带成员行，"已删除"一列失败。
- e2e：W4 另跑 `--repeat-each 3`。

## 完成检查

`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿。
