# M2/P3/S4 交错 4–8：实施计划

上级：[P3 文档](../03-P3-invitations.md) 3.9。

## 任务

`bootstrap/interleavings_invitations_test.go`，沿用 P2 的 `acmeTeam` 与 `interleave`（必要时把持锁的语句参数化：工作区行、账户行），每个交错两种先后，之后核对数据库：

- 4 接受与删除工作区；5 接受与删除邀请；6 接受与移出他；
- 7 两位管理员同时邀请同一邮箱：`WaitForKeyWaitOn("workspace_invitations")`；
- 8 接受与 `users set-email`：持账户行，改邮箱经 `bootstrap.Users` 在进程内运行。

## 测试

- 反向对照：接受不在锁下重读邀请，交错 5 失败；成员关系结束不删邀请，交错 6 失败；接受不用锁下读到的邮箱，交错 8 失败。
- `-race -count=5` 稳定。

## 完成检查

`make check` 为绿。
