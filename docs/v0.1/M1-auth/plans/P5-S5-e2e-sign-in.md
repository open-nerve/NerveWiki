# M1/P5/S5 端到端（一）：实施计划

上级：[P5 文档](../05-P5-web-session.md) 3.9。

## 任务

1. `fixtures/test.ts`：`signedInPage`（接口注册，`addInitScript` 只写一次 `nwiki.auth`）；`pageWatch.expectConsole({errors, warnings})`，自动检查按声明比较。
2. `fixtures/auth.ts`：`newRecord`、`recordOf`、`writeRecord`；`fixtures/auth-pages.ts`：填写、提交登录与注册（等接口的答复）、表单上方的错误。
3. 冒烟故事：S2（未登录到登录页，只请求实例信息；登录之后显示版本）、S4（未登录的深链带 `next`；登录之后 404、刷新保持）。
4. A1、A2、A3、A14 的页面版本。
5. M0/P6 文档中的 `expectQuietConsole` 改为 `expectQuietPage`。

## 测试

- `make e2e` 全部通过；`--repeat-each 3` 与 `--workers 1` 各一轮。
- 反向对照：`next` 放过 `//evil.example` → A3 失败；声明的控制台输出少一条 → 自动检查失败。

## 完成检查

`make check`、`make e2e` 为绿。
