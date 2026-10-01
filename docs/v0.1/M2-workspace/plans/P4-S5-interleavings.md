# M2/P4/S5 交错 9–13：实施计划

上级：[P4 文档](../04-P4-deactivation-commands-purge.md) 3.5。

## 任务

`bootstrap/interleavings_deactivation_test.go`，沿用 `acmeTeam`、`interleaveOn` 与 `orders`，每个交错两种先后，之后核对数据库：

- 9 两位管理员同时停用：持工作区行；一位经接口，一位经 `Users` 在进程内。
- 10 停用与接受邀请：持账户行；新行与恢复已结束的行各一组。
- 11 停用与创建工作区：持账户行。
- 12 停用与 `reactivate-member`：持账户行；命令经 `Workspaces` 在进程内。
- 13 停用与移出他：持工作区行。

## 测试

- 反向对照：否决阶段用锁外读到的人数判定，交错 9 失败；`ShareAccountByEmail` 去掉 `FOR SHARE`，交错 12 失败。
- `-race -count=5` 稳定。

## 完成检查

`make check` 为绿。
