# M0/P4/S4 instance 模块与整个程序的契约测试：实施计划

上级：[P4 文档](../04-P4-api-contract.md) 3.5、3.6。

## 任务

1. `modules/instance`：`domain`、`app`、`adapter/buildinfo`、`adapter/http`（`httpadapter`）、`module.go`，按 3.5；从 Nerve 拷贝后去掉时区与实例设置项。
2. 组合根：`newApp` 建 `httpserver.NewAPI`，`instance.New(...).Register(router, api)`；编译期断言 `*shared.Error` 满足 `httpserver.ProblemError`。
3. 整个程序的测试（`bootstrap`）：`GET /api/v0/instance`；兜底仍然有效；`/api/v0` 下的路由等于契约的操作；每个 `Kind` 的 problem；字段错误码与契约的枚举；problem 响应的头。

## 测试

- 模块：用例、适配器、处理器的测试；`apitest.Main(m)`（实施时由 `Main(m, "instance")` 改为自己得出模块名，见 P4 审查 M6）。
- 反向对照：契约多一个操作、实现多一个路由、`shared` 多一个字段错误码，都各自失败。

## 完成检查

`make check`、`make gen-check` 为绿；`make run` 后手工检查 3 个请求（P4 文档第 6 节）。
