# M1/P5/S3 登录与注册：实施计划

上级：[P5 文档](../05-P5-web-session.md) 3.5、3.6。

## 任务

1. 表单组件：`Input`、`Label`、`Alert`、`FormField`（标签、输入、字段错误，`aria-invalid`、`aria-describedby`）；密码框的明文切换。
2. `app/next-path.ts` 与拷贝的用例。
3. `app/guards.tsx`：`GuestOnly`、`SignedIn`（加载中、会话暂不可用、带 `next` 去登录页、取 `/me`）；`routes.tsx` 的新路由表（`Onboarded` 在 S4，这一步首页与 404 直接挂在 `SignedIn` 下）。
4. `app/session-unavailable.tsx`：说明与"重试"。
5. `pages/sign-in.tsx`、`sign-up.tsx`：本地检查、字段错误、表单上方的错误、429 的秒数、注册关闭；成功之后不自己跳转。
6. `app/user-menu.tsx`：显示名、退出；接进 `Layout`。
7. 文案（`en`、`zh-CN`）。

## 测试

- 守卫：`starting`、`unavailable`、`signed-out`（带 `next`，含查询与片段）、`signed-in`（已登录访问登录页去 `next` 或 `/`）；取 `/me` 失败显示会话暂不可用。
- `next` 的判定表（含 Nerve 的恶意用例）。
- 登录：401 与 403 的文案、输入保留；429 的秒数；提交中禁用。注册：本地检查；422 在字段下方；409 在上方；注册关闭。
- 退出：状态变为 `signed-out`，去登录页。
- 反向对照：`next` 放过 `//` → 判定表失败；登录页自己跳转 → 守卫的测试失败。

## 完成检查

`make check` 为绿。`make e2e` 中 S2、S4 预期失败（S5 调整），其余照旧。
