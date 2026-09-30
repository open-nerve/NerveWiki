```yaml
status: open
from: M1
to: M2
created: 2026-10-01
```

# 账户认证留给 M2 的部分

M1 建好了账户停用的扩展点、新手引导的步骤列表与注册策略（总体设计 12.4），但没有注册者：第一批注册者随工作区到来。下面是 M1 各 Phase 的审查、结果与[收尾审查](../../M1-auth/reviews/M1-closeout-review.md)留给 M2 要做的事，写 M2 的 00 号文档时一并考虑。所有 M 都要遵守的写法（注册者的构造与组合、管理命令、守卫、端到端的夹具等）已在总体设计第 13 节，这里不重复。

1. **停用的第一个注册者**：`bootstrap/registrants.go` 的 `deactivationRegistrants()` 现在没有参数、返回空；M2 让它接收连接池，构造否决者与订阅者（总体设计 13.1 第 21 条）。组合检查（`archtest/composition_test.go`）只证明 `bootstrap.Users` 与 `bootstrap.newApp` 静态地到达它，不证明结果交给了 `identity.NewAdmin` 与 `identity.New`：收尾审查把两处都换成空的列表，全部测试照样通过。所以第一个否决者要有两条行为测试：经 `nervewiki users deactivate` 被它否决（退出码 1、打印原因、账户照旧可用），经 `POST /me/deactivate` 被它否决（答出它的码、会话保留）。
2. **否决者的码**：码写进 `deactivateMe` 的 `x-problem-codes`（在 `api/modules/identity.yaml`），并在 **identity** 的 `adapter/http` 测试中经 `apitest.CheckResponse` 答出一次：`apitest.Main` 按测试所在模块的描述核对，M2 模块自己的测试答不出 identity 的操作。前端的文案表（`web/apps/web/src/app/problem-messages.ts`，两种语言）同时加上它，契约核对会要求；页面上它显示在停用的确认对话框里（[P6 文档](../../M1-auth/06-P6-settings.md) 3.5），管理员的命令打印同一个原因。
3. **`ShareActiveAccount`**：加入工作区、接受邀请这类让账户获得新访问的写事务，以 `identity.NewAccounts(pool).ShareActiveAccount(ctx, userID)` 作为第一条语句（总体设计 13.1 第 18 条）。
   - 它在事务之外调用时 `FOR SHARE` 随语句结束，与停用不再串行，目前只有注释提醒（[P3 审查](../../M1-auth/reviews/P3-accounts-tokens-review.md)）。由 M2 取舍：在它里面检查上下文中有事务、没有就返回故障，或者只靠增长路径的并发测试守住。
   - 它答 403 `identity.account_deactivated` 与 404 `identity.account_not_found`（`identity/app/accounts.go`）。调用它的 M2 操作要么在自己的 `x-problem-codes` 中声明这两个码，要么把它们译成自己的码；前端的文案表目前没有 `identity.account_not_found`。
4. **注册策略**：端口 `SignupPolicy.AllowSignup(ctx) (bool, error)` 今天只回答开关，拿不到邮箱与邀请，注册的请求体也只有邮箱与密码。M2 的"持有效邀请可以注册"要扩展端口的参数与 `register` 的请求体（总体设计 12.4 写明这是例外），组合根把 `bootstrap/app.go` 的 `signupSwitch` 换成考虑邀请的策略，identity 不认识邀请。另外 `auth.signup_enabled` 接了两条线：`instance` 直接取配置作为 `GET /instance` 的 `signup_enabled`（前端据此显示"未开放注册"），identity 经 `signupSwitch`；M2 改变注册的含义时两处一起改。
5. **引导的新步骤**：前端的注册表（`web/apps/web/src/onboarding/steps.ts`）在后面追加，组件用 `React.lazy`（M1 总设计第 8 节）。同时把新步骤的 id 加进 e2e 的 `onboardingSteps`（`e2e/fixtures/auth.ts`），否则 `registerOnboarded` 的账户会落到引导页，用它的故事（S2、S4、A3、A6、A7、A8、A10、A11）失败；A9 断言的 "Step 1 of 1" 随之改变。
6. **首页**：`/` 目前显示 M0 的实例版本（[P5 文档](../../M1-auth/05-P5-web-session.md) 3.5），M2 换成工作区的外壳。S2 的冒烟故事核对首页，随之调整。
7. **接口测试的构造辅助**：`httpserver.NewAPI` 在测试中各自构建了三处（identity 的 `module_test.go` 与 `adapter/http/handler_test.go`，instance 的 `adapter/http/handler_test.go`），P2 给 `APIConfig` 加字段时改了三处。M2 出现第四处时在 `apitest` 提取构造辅助（[P2 审查](../../M1-auth/reviews/P2-sessions-ratelimit-review.md)有意决定第 8 项）。
8. **组合根的拆分**：`bootstrap/app.go` 332 行，每加一个模块约多 20 行，出错时关闭迁移器与连接池的清理重复三次。M2 加模块之前把组装与生命周期分成两个文件，模块 `Deps` 的构建抽成函数。
9. **只投递的 River 客户端**：M1 的后台任务只有会话清理，是周期任务，没有请求投递任务。第一个由请求投递的任务出现时（M2 或之后）加只投递的客户端（[P4 文档](../../M1-auth/04-P4-admin-jobs.md)第 2 节"不做"）；停机顺序已经让 HTTP 先于任务停（同一文档 3.4）。M2 用不到时，移交给用到它的 M。
