# M1/P5/S2 分代与请求的错误：实施计划

上级：[P5 文档](../05-P5-web-session.md) 3.3、3.4。

## 任务

1. `RootStore` 一代：共用的 `preferences`、`instance`、`auth`，本次登录的 `account`（未登录时没有）。
2. `app/session-root.tsx`：订阅 `session.tokens`，每个 `loginId` 一代，`<AppProviders key>` 包着 `RouterProvider`；`main.tsx` 渲染它。
3. `lib/one-at-a-time.ts`（拷贝，加它自己的测试）；`services/auth.service.ts`（登录、注册，公开客户端）、`account.service.ts`（`getMe`、`updateMe`、`recordOnboardingStep`）；`stores/auth.store.ts`（登录、注册之后 `tokens.signIn`；退出）、`account.store.ts`（`me`，写操作经 `oneAtATime`，忽略 `SessionChangedError`）。
4. `services/api.ts`：`unwrap` 接受 204；`ApiError.retryAfter`。
5. `app/retry.ts`：`retryDelay(error, attempt)`，接到 `AppProviders` 的 `onErrorRetry`，替换 `isRetryable`。
6. `app/problem-messages.ts` 与文案；契约核对的测试（按行读 `api/dist/openapi.yaml`）。
7. `test/render.tsx`：按会话渲染应用的辅助函数（假的服务端、记录、状态）。

## 测试

- 分代：换代之后旧一代的请求以 `SessionChangedError` 结束、不写入新一代的 store；新一代的 SWR 缓存为空；偏好与实例信息跨代保留；同一个 `loginId` 的 `unavailable` 与 `signed-in` 不换代。
- `AccountStore`：两个写操作依次发出，`me` 是后一个的答复。
- `unwrap` 的 204；`retryAfter` 的解析；`retryDelay` 的判定表；契约核对。
- 反向对照：`AppProviders` 不带 key → 缓存的测试失败；`retryDelay` 对 429 不重试 → 判定表失败；契约加一个码 → 核对失败。

## 完成检查

`make check`、`make gen-check` 为绿。
