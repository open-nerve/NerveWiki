# M1/P1/S3 认证接入与注册：实施计划

上级：[P1 文档](../01-P1-identity-foundation.md) 3.5–3.9。

## 任务

1. 配置：`auth` 节、`server.trusted_proxies`（列表与 `netip.Prefix` 的解码、校验）、日志表示（私钥只记是否设置）；各环境的配置文件；prod 缺私钥拒绝启动。
2. httpserver：`clientip.go`；请求信息中间件；`Authenticator`、公开操作清单、认证中间件；`APIErrors` 对 401 加 `WWW-Authenticate`；`APIConfig` 与 `NewAPI` 的扩充。
3. identity 的 `app`（ports、issuer、register、get_me、authenticate）、`adapter/authn`、`adapter/http`（handler、auth、me）与 `module.go`；`api/modules/identity.yaml`（register、getMe）、`api/openapi.yaml`（tag、路径、`securitySchemes.bearer`）；生成。
4. `instance`：`signup_enabled` 与公开操作清单。
5. 组合根：读签名私钥、装配 identity、`APIConfig`、`warnIfExposed`。
6. 整个程序的测试：公开操作与契约一致、没有令牌答 401、problem 响应声明两个头、401 带 `WWW-Authenticate`。
7. 前端：重新生成的类型使 `InstanceInfo` 多一个字段，更新测试替身。

## 测试

- 各用例在真实 PostgreSQL 上的测试（并发的同邮箱注册只有一个成功；认证的各种会话状态）；handler 测试答出每个声明的码。
- 反向对照：契约把 `getMe` 标成公开 → 公开操作的测试失败；认证中间件放过没有令牌的请求 → 401 的测试失败；注册关闭时先校验请求体 → handler 测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
