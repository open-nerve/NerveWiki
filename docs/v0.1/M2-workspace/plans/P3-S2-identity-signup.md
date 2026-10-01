# M2/P3/S2 identity 与注册策略：实施计划

上级：[P3 文档](../03-P3-invitations.md) 3.3、3.4、3.6。

## 任务

1. `ShareActiveAccount(ctx, id) (email string, err error)`：交回 `FOR SHARE` 锁下读到的邮箱。workspace 的 `Accounts` 端口与创建工作区随之改。
2. 目录：`identity.NewProfiles` 改名 `NewDirectory`，加 `AccountIDByEmail(ctx, email) (uuid.UUID, bool, error)`（不加锁）；组合根的适配器改名 `bootstrap/directory.go`，实现 workspace 的 `MemberProfiles` 与新的 `AccountFinder`。
3. 注册策略：`SignupPolicy.AllowSignup(ctx, SignupAttempt)`；`Register` 把规范化之后的邮箱与请求体的 `invitation` 交给它；`api/modules/identity.yaml` 的 `register` 请求体加可选的 `invitation {id, token}`。
4. workspace：`invitations.go` 的 `NewInvitationCheck(pool, key)`：`Admits`。
5. 组合根：`bootstrap/signup.go` 的策略（开关或邀请），替换 `signupSwitch`；`deps.go` 交出 `InvitationKey`。

## 测试

- identity：`ShareAccount` 交回邮箱；`AccountIDByEmail` 读到、读不到；`Register` 把规范化的邮箱与邀请交给策略。
- `Admits` 的表格（真实数据库）：令牌不对、邀请已删除、工作区已删除、邮箱不同、全都对。
- 组合根的策略：开着时不看邀请；关着时只有 `Admits` 为真才允许；`Admits` 的错误原样上抛。
- 整个程序：注册关闭时带有效邀请注册成功，邮箱不同 403 `identity.signup_disabled`。
- 反向对照：策略关着时不看邮箱，策略的测试失败。

## 完成检查

`make check`、`make gen-check` 为绿。
