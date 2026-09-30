# M1/P1/S4 契约的整个程序测试：实施计划

上级：[P1 文档](../01-P1-identity-foundation.md) 3.9。

## 任务

1. `apitest/operations.go` 的其余部分：`Operation.Public`、`Target`、`ParamCases`、`BodyCases`、`HasJSONBody` 及测试。
2. 两条写法规则：参数写 `schema` 不写 `content`；JSON 请求体是对象。
3. 整个程序的测试：`TestParametersThatDoNotBindAnswer400`、`TestBodiesThatBreakTheStructureAnswer400`、`TestTheAnswerToABrokenBodyStaysSmall`。

## 测试

- 反向对照：请求体结构检查放过未知字段 → 请求体测试失败；错误回答带上整个请求体 → 回答大小的测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
