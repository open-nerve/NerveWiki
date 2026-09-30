```yaml
status: open
from: M1
to: M2
created: 2026-10-01
```

# 身份与账户留给 M2 的部分

M1 建好了账户停用的扩展点与新手引导的步骤列表（[M1 总设计](../../M1-auth/00-M1-design.md)第 8 节），但没有注册者：第一批注册者随工作区到来。下面几项是各 Phase 的审查与结果里留给 M2 的，写 M2 的 00 号文档时一并考虑。扩展点本身的约定见 M1 总设计第 8 节与总体设计 12.4，这里不重复。

1. **第一个停用注册者要有经命令行的行为测试**：`archtest/composition_test.go` 只证明 `bootstrap.Users` 与 `bootstrap.newApp` 静态地到达 `deactivationRegistrants`，不证明结果交给了 `identity.NewAdmin` 与 `identity.New`（[P4 文档](../../M1-auth/04-P4-admin-jobs.md)第 7 节第 5 项）。M2 的第一个否决者要有一个测试经 `nervewiki users deactivate` 被它否决：退出码 1、打印原因、账户照旧可用；自助停用一路同样答出它的码。
2. **注册者的构造函数不能叫 `New`**：组合检查把模块根的 `New` 当作 HTTP 一侧（命令行不应到达它）；`NewAccounts`、`NewVetoer` 一类的名字可以。注册者只凭连接池构造，在 `bootstrap/registrants.go` 一处组合，serve 与命令行都从那里取。
3. **否决者的码**：否决者返回的 `*shared.Error` 的码追加到 `deactivateMe` 的 `x-problem-codes`，在所属模块 `adapter/http` 的测试中经 `apitest.CheckResponse` 答出一次；前端的文案表（`web/apps/web/src/app/problem-messages.ts`，两种语言）同时加上它：契约核对要求每个声明的码都有文案。页面上它显示在停用的确认对话框里，会话保留（[P6 文档](../../M1-auth/06-P6-settings.md) 3.5）；管理员的 `users deactivate` 打印同一个原因。
4. **`ShareActiveAccount` 在事务之外调用**：加入工作区、接受邀请这类让账户获得新访问的写事务，以 `identity.NewAccounts(pool).ShareActiveAccount(ctx, userID)` 作为第一条语句。在事务之外调用时 `FOR SHARE` 随语句结束，与停用不再串行，目前只有注释提醒（[P3 审查](../../M1-auth/reviews/P3-accounts-tokens-review.md)）。由 M2 取舍：在 `ShareActiveAccount` 里检查上下文中有事务、没有就返回故障，或者只靠增长路径的并发测试守住。
5. **注册策略**：注册的开关是端口 `SignupPolicy`（`identity/app/ports.go`），组合根目前把配置 `auth.signup_enabled` 适配成它（`bootstrap/app.go` 的 `signupSwitch`）。M2 的"持有效邀请可以注册"由组合根换成考虑邀请的策略，identity 不认识邀请；`GET /instance` 的 `signup_enabled` 与注册页的"未开放注册"随之决定含义。
6. **引导的新步骤**：前端的注册表（`web/apps/web/src/onboarding/steps.ts`）在后面追加，组件用 `React.lazy`；已有账户下次访问时只看到新的步骤（M1 总设计第 8 节）。步骤 id 的格式与个数上限由领域与数据库检查，服务端不认识具体的步骤。
7. **首页**：`/` 目前显示 M0 的实例版本（[P5 文档](../../M1-auth/05-P5-web-session.md) 3.5），M2 换成工作区的外壳。所有页面都要登录、未完成引导先去 `/onboarding`，由守卫决定，新页面挂在 `Onboarded` 之下即可。
8. **接口测试的构造辅助**：`httpserver.NewAPI` 在测试中各自构建了三处（identity 的 `adapter/http` 与 `module_test.go`、instance 的 `adapter/http`），P2 给 `APIConfig` 加字段时改了三处。M2 出现第四处时在 `apitest` 提取构造辅助（[P2 审查](../../M1-auth/reviews/P2-sessions-ratelimit-review.md)第 8 项决定）。
9. **组合检查的起点**：M2 加 `nervewiki workspaces` 这类管理命令时，把它的入口加进 `composition_test.go` 的起点列表，并列出它应当到达的管理用例。
10. **只投递的 River 客户端**：M1 的后台任务只有会话清理，是周期任务，没有请求投递任务。第一个由请求投递的任务出现时（M2 或之后）加只投递的客户端（[P4 文档](../../M1-auth/04-P4-admin-jobs.md)第 2 节"不做"）；停机顺序已经让 HTTP 先于任务停（同一文档 3.4）。
