# M1/P1/S2 凭证与领域规则：实施计划

上级：[P1 文档](../01-P1-identity-foundation.md) 3.4、3.7。

## 任务

1. `shared/actor.go`（`Actor`、放进与取出 ctx）、`shared/email.go`（规范化与校验）及测试。
2. `shared` 的字段码加 `common_password`；`api/common.yaml` 的枚举同步。
3. identity 的 `domain`：`errors.go`（P1 用到的码）、`user.go`（邮箱、显示名 1–100、引导步骤 id）、`password.go`（8–128、名单、邮箱主干）、`session.go`（刷新令牌的格式与解析、撤销原因、User-Agent 的清洗）。
4. `tools/password-blocklist`：取 SecLists 固定提交中 NCSC 的前 10 万，核对 SHA-256，取 8 位以上、小写、去重，写出 `domain/common_passwords.txt`；knip 的入口。
5. `adapter/signing`（私钥解析、临时密钥、JWT、HKDF 派生的 MAC）与 `adapter/argon2`（PHC、并发名额、超时 503）及测试。

## 测试

- 表格驱动的单元测试；JWT 的各种无效令牌（错的算法、缺 `exp`、过期、篡改、伪造的 claim 不进错误文本）。
- 反向对照：名单为空 → 密码规则测试失败；HKDF 的 info 改变 → 已知答案的测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
