# M1/P3/S2 PAT 的操作：实施计划

上级：[P3 文档](../03-P3-accounts-tokens.md) 3.2、3.4、3.7、3.8。

## 任务

1. `app/current_password.go`：事务外按快照校验、只做一次的准备、锁下核对快照并写入、哈希变了再试一次。
2. 用例：`create_api_token.go`（先查规格，再经当前密码，锁下插入）、`list_api_tokens.go`、`revoke_api_token.go`；查询与仓储。
3. 配置 `ratelimit.password_user`；组合根装配这个桶；HTTP 适配器的 `limitPassword`（按账户）。
4. 契约：`listApiTokens`、`createApiToken`、`revokeApiToken`；生成；`api_tokens.go` 的 handler。
5. 架构测试：`isBannedFromBinary` 按导入者判断，`TestBannedImports` 加 runtime 的用例。
6. `TestParametersThatDoNotBindAnswer400` 的守卫：推导出 0 个用例时失败。
7. 整个程序的测试：契约中每个需要令牌的操作，用新账户的有效 PAT 调用，回答不是 401。

## 测试

- 当前密码：错误的密码答 `current_password_incorrect` 且不开事务；快照变化时对新哈希再校验；第二次变化答 `current_password_incorrect`；准备只做一次。
- 创建：规格错误先于密码校验；PAT 调用也可以；回答的期限与库中一致；日志只有 id。
- 撤销：别人的、已撤销的、不存在的同答 404；撤销之后答 401；PAT 撤销自己。
- 列表：不含已撤销的，含已过期的，新的在前，不含明文与哈希。
- handler 测试答出每个声明的码；`password_user` 超出答 429。
- 反向对照：runtime 的例外按被导入的路径判断 → `TestBannedImports` 失败；撤销不带 `user_id` → 撤销别人令牌的测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
