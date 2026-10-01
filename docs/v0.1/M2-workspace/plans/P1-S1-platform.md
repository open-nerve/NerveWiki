# M2/P1/S1 平台与组合根：实施计划

上级：[P1 文档](../01-P1-access-workspaces.md) 3.2、3.3、3.6、3.9。

## 任务

1. `bootstrap/app.go` 拆成三个文件，只移动与提取，行为不变：
   - `app.go`：生命周期；
   - `wire.go`：`newApp`。连接池与迁移器建好之后，失败的清理由一处 `defer` 负责；
   - `deps.go`：`identityDeps`、`instanceDeps` 等构建函数。
2. `apitest/api.go`：`NewAPI(t, APIOptions)`，返回丢弃日志、桶用不完的 `*httpserver.API`。identity 的两处、instance 的一处测试改用它。
3. `postgres.InTx(ctx)`；identity 仓储的 `ShareAccount` 在事务之外返回故障。
4. `CheckName` 移到 `shared/name.go`，测试一起移动；identity 改为调用 `shared.CheckName`。

## 测试

- 现有测试原样通过：组合根、identity、instance。
- `InTx`：`WithinTx` 内为真，外为假。
- `ShareAccount`：事务之外答错误，数据库里没有残留的锁；事务之内照旧。
- 反向对照：去掉 `ShareAccount` 的检查，新测试失败。

## 完成检查

`make check` 为绿；`app.go` 不超过约 200 行。
