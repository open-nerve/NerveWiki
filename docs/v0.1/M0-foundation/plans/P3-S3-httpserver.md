# M0/P3/S3 HTTP 平台：实施计划

上级：[P3 文档](../03-P3-server-platform.md) 3.4。

## 任务

1. `internal/platform/httpserver`：从 Nerve 拷贝 `server.go`、`routes.go`、`middleware.go`、`problem.go`。
   - 改名，删掉 Nerve 文档的编号引用。
   - `problem.go` 只保留 P3 用到的平台错误码。
   - `routes.go` 的 `/readyz` 与 `/api/` 兜底原样保留。
2. 新写 `longlived.go`：
   - `Server` 在 `BaseContext` 中放入"开始停机"的信号，用 `RegisterOnShutdown` 发出；
   - `LongLived(logger, h)` 解除连接的读写期限，并在停机开始时取消 handler 的 context。解除期限失败（写入器不支持）时记录错误并回答 500。
   - 实施时修正：只解除写期限，读期限保留以约束请求体，见 P3 文档 3.4。

## 测试

- 拷贝 Nerve 的 `server_test`、`routes_test`、`middleware_test`、`problem_test`，改名、裁掉 P4 与 M1 的部分。
- `longlived_test`，在真实监听上：
  - 服务器的读写期限设为 200 ms；
  - 普通 handler 等 500 ms 再写，客户端读不到完整响应；
  - 长连接 handler 每 100 ms 推送一次、共推 1 秒，客户端全部收到；
  - 调用停机：长连接 handler 的 context 被取消并返回，`Serve` 在停机期限之内返回 nil，同时进行的普通请求照常完成。

## 完成检查

`make check` 为绿。
