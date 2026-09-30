```yaml
status: open
from: M0/P4
to: M1
created: 2026-09-30
```

# 接口契约留给 M1 的部分

M0/P4 只做了 `instance` 用得到的部分（[P4 文档](../../M0-foundation/04-P4-api-contract.md) 第 2 节"不做"一表）。以下几项在 Nerve 中都有现成实现，M1 引入第一个需要令牌、带参数或带请求体的操作时一起拷贝、裁剪，按"拷贝即接管"逐个文件审查。与 [P3 的移交](M0-P3-platform.md) 第 1–3 项（客户端 IP、限流、认证的接入）一起做：

1. **401 与 `WWW-Authenticate`**：`APIErrors.Write` 对 401 加 `WWW-Authenticate: Bearer`；每个模块文件的 `components.responses.Problem` 声明这个头，整个程序的测试 `TestEveryProblemResponseDeclaresItsHeaders` 把它加进要求的头；接口描述加 `securitySchemes.bearer`，需要令牌的操作写 `security: [{bearer: []}]`。`bootstrap` 的 `TestEveryKindBecomesItsProblem` 加上 401 的 `WWW-Authenticate` 断言。`apitest` 已经认得"需要令牌的操作可以答 `unauthorized`"（`checkProblemCode`），规则测试已经要求每个操作显式声明 `security`、引用的 scheme 必须存在。
2. **公开操作的清单**：模块的 `PublicOperations()`，以及整个程序的测试"模块声明的公开操作正好是契约中 `security: []` 的操作""不是公开的操作没有令牌时答 401"。`apitest.Operation` 随之恢复 `Public` 字段。
3. **参数与请求体的整个程序测试**：Nerve 的 `TestParametersThatDoNotBindAnswer400`、`TestBodiesThatBreakTheStructureAnswer400`、`TestTheAnswerToABrokenBodyStaysSmall`，以及 `apitest` 为它们推导用例的 `Operation.Target`、`ParamCases`、`BodyCases`、`HasJSONBody`（`operations.go` 的其余部分）和对应的测试。同时恢复两条只为它们服务的写法规则：参数写 `schema` 不写 `content`；JSON 请求体是对象。
4. **oapi-codegen runtime 的例外**：第一个带参数的操作让生成代码导入 `github.com/oapi-codegen/runtime`，它会把 `github.com/google/uuid` 带进二进制，架构测试 `binary_test` 的禁用清单要为这条路径开例外（Nerve 的 `isBannedFromBinary` 按导入者判断）。`generated_test` 已经保证生成代码本身不用 runtime 的 `UUID`。
5. **逐路由中间件的扩充**：按 Nerve 的顺序插进 `API.Middlewares`：客户端信息（IP、User-Agent）→ 请求期限 → 请求体上限 → 认证 → 限流 → 请求体结构检查。`APIConfig` 随之加 `Authenticator`、`PublicOperations`、限流桶与 IPv6 前缀长度；`NewAPI` 的参数校验与测试同步。
