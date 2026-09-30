# M0/P5/S4 多语言、分层样板与页面：实施计划

上级：[P5 文档](../05-P5-web-shell.md) 3.2、3.3、3.4。

## 任务

1. 多语言：`i18n/messages/en.ts`（源头）、`zh-CN.ts`（类型为 `Messages`）、`i18n.tsx`（`t`、插值、`I18nProvider`、`useT`）；`PreferencesStore` 加语言；切换时更新 `<html lang>`；顶栏加语言切换。
2. 分层样板：`services/api.ts`（`ApiError`、结果转换）、`services/instance.service.ts`、`stores/instance.store.ts`、`stores/root.store.ts`、`stores/context.tsx`；`app/providers.tsx`（`StoreProvider`、`SWRConfig`、`I18nProvider`）。
3. 页面与路由：`app/router.tsx`（数据路由，页面 `lazy`）；首页显示产品名、实例版本、接口版本，含加载与错误状态；应用内 404；根路由的错误边界。
4. README 新增"前端"一节：`make dev`、`make web-dev`、`make build`，以及 service → store → 组件的写法。

## 测试

- 文案：两种语言同一个键的占位符相同、没有空串。
- `InstanceService`：200 → 值；problem → `ApiError`（带 `status`、`code`）；网络错误原样抛出（用 openapi-fetch 的 `fetch` 选项注入假的实现）。
- `InstanceStore`：`fetch()` 把结果写进 `info`；service 失败时抛出、不改状态。
- 页面：首页渲染出版本；未知路由渲染 404；渲染时抛出的异常由错误边界显示错误页；切换语言后文案改变。
- 反向对照：删掉 `zh-CN.ts` 的一个键 → 类型检查失败；改掉一个占位符 → vitest 失败。

## 完成检查

`make check` 为绿；`make build` 后浏览器打开首页显示实例版本，语言与主题的切换在刷新后保持，`/nope` 显示应用内 404，控制台没有错误与 CSP 违规。
