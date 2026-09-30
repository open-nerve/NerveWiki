# M0/P4/S2 错误映射与逐路由中间件：实施计划

上级：[P4 文档](../04-P4-api-contract.md) 3.4。

## 任务

1. `httpserver/apierrors.go`：`ProblemError`（含两个可选方法）、`APIErrors`（`BadRequest`、`BodyError`、`Write`）；平台码加 `bad_request`、`payload_too_large`。从 Nerve 拷贝，去掉 401 的 `WWW-Authenticate`（随 M1）。
2. `httpserver/api.go`：`APIConfig{Logger, MaxBodyBytes, RequestTimeout}`、`NewAPI`、`API.Errors`、`API.Middlewares(bodies)`（期限 → 上限 → 结构，按相反顺序返回）。不含认证、限流、客户端 IP（M1）。
3. `httpserver/bodyshape`：从 Nerve 拷贝（`bodyshape.go`、`ambiguity.go`、`middleware.go` 与测试），改名。
4. 配置：`server.request_timeout`（默认 15s，必须为正且短于 `write_timeout`）、`server.max_body_bytes`（默认 1 MiB，至少 1）；`LogValue` 与各 profile 的期望值同步。

## 测试

- 拷贝 Nerve 的 `apierrors_test`、`api_test` 中与期限、上限、结构检查、顺序相关的部分，以及 `bodyshape` 的全部测试。
- 新增：长连接路由的 context 没有请求期限，接口操作的有。
- 配置：新键的默认值、交叉校验、日志。

## 完成检查

`make check` 为绿。
