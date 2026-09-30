# M0/P4/S3 契约测试工具：实施计划

上级：[P4 文档](../04-P4-api-contract.md) 3.6。

## 任务

1. `httpserver/apitest`：从 Nerve 拷贝 `apitest.go`（`Load`、`CheckResponse`、`CheckRequest`、`CheckSchema`、`Enum`）、`problems.go`（错误码核对、`Main`）、规则（`rules_test`、`rules_cases_test`）；`operations.go` 只保留整个程序的测试用到的部分（操作清单），参数与请求体用例的推导随 M1。
   - 规则加一条：每个操作显式声明 `security`。
   - problem 响应的头只要求 `Retry-After`（`WWW-Authenticate` 随 M1）。
2. `httpserver/contract_test.go`：平台写出的每一种 problem 都符合 `Problem` schema。
3. 依赖：kin-openapi v0.149.0。
4. 架构测试：规则 8 加入 `apitest`；`binary_test` 的禁用清单加入 kin-openapi；恢复 `generated_test`（生成代码不使用 runtime 的 `UUID`）。

## 测试

- `apitest` 自己的测试（拷贝、裁剪）。
- 反向对照：改掉 `Problem` 的一个 JSON 字段名，平台契约测试失败；非测试代码导入 `apitest`，架构测试失败。

## 完成检查

`make check` 为绿。
