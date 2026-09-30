# M1/P3/S1 PAT 的数据与认证：实施计划

上级：[P3 文档](../03-P3-accounts-tokens.md) 3.2、3.3、3.5。

## 任务

1. 迁移 `00004_identity_api_tokens.sql`：列、约束名（`api_tokens_token_hash_key`、`api_tokens_token_hash_check`、`api_tokens_name_check`、`api_tokens_expires_at_check`）、列表的部分索引。
2. 领域：`api_token.go`（`PATPrefix`、`PAT`、`String`、`Hash`、`ParsePAT`；`APIToken`、`APITokenSpec`、`CheckAPIToken`）；`session.go` 加 `RevokeReason` 与六个值；错误 `current_password_incorrect`、`api_token_not_found`、`account_not_found`。
3. 查询与仓储：PAT 按哈希、按 id 读（连同账户是否可用）、`last_used_at` 的条件更新；账户行锁加读邮箱；`RevokeSessions`。
4. `app`：`ports_api_tokens.go`；`authenticate.go` 的 PAT 路径与 `last_used_at`；`credential_lock.go`。
5. `authn`：PAT 的限流键 `pat:<id>`；过期的 PAT 不是 `ExpiredCredential`。

## 测试

- PAT 的解析：正确的写法往返；前缀、长度、非 base64url 字符、末位未用比特非零、夹带换行，各答 false。
- `CheckAPIToken`：名称的空白、长度、控制字符，期限的过去与截断，一次报出全部问题。
- `RevokeReason` 的值与 `auth_sessions_revoke_reason_check` 一致（数据库测试逐个写入）。
- 认证的单元测试：PAT 的每种失败答 401；`last_used_at` 的节流与写失败。
- 仓储：`RevokeSessions` 只撤销未撤销、未过期、不是 keep 的会话；`last_used_at` 的条件；CHECK 的反例。
- `CredentialLock`：会话、PAT 各自的每种失效在锁下答 401；账户不存在、停用答 401。

## 完成检查

`make check`、`make gen-check` 为绿。
