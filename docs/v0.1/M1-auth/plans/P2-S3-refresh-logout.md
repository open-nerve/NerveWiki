# M1/P2/S3 续期与退出：实施计划

上级：[P2 文档](../02-P2-sessions-ratelimit.md) 3.5、3.6、3.8。

## 任务

1. 领域：`JudgeRefresh`（第 3.5 节的判定表）、撤销原因（`reuse_detected`、`logout`）。
2. 查询：按 id 读会话、条件轮换、为重复使用撤销、条件退出；生成。
3. `app`：`refresh.go`（解析、事务内读判、条件轮换、未命中时重读重判一次、重复使用的撤销先提交）；`logout.go`；错误码 `identity.refresh_token_invalid`。
4. 契约：`refresh`、`logout`；handler 设 `auth.refresh_deadline`。

## 测试

- 判定表逐行的单元测试。
- 真实数据库：轮换之后旧代失效、真实的旧代撤销会话、伪造的旧代不改会话、换钥之后当前代仍可续期而旧代不再触发撤销、过期与撤销的会话、并发续期恰有一次轮换；退出只撤销当前代的有效会话，之后访问令牌失效。
- 反向对照：条件轮换去掉代数条件 → 并发测试失败；旧代不核对 MAC 就撤销 → 伪造旧代的测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
