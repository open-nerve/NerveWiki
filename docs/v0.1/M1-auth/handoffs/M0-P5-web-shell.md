```yaml
status: open
from: M0/P5
to: M1
created: 2026-09-30
```

# 前端外壳留给 M1 的部分

M0/P5 按总体设计 9.4 定下了 service → store → 组件的写法，但 M0 没有登录，会话相关的部分留给 M1（[P5 文档](../../M0-foundation/05-P5-web-shell.md) 3.3）：

1. **每次登录一代 `RootStore`**：会话变化时新建 `RootStore`（带新的 API 客户端），旧一代的请求以 `SessionChangedError` 结束。`PreferencesStore` 属于设备，照旧传入新的一代。
2. **SWR 的缓存跟着一代走**：SWR 只在 `SWRConfig` 挂载时建一次缓存，只换 `AppProviders` 的 `store` 属性，上一个会话的缓存会留下（P5 审查 M3）。每一代 `RootStore` 都要重新挂载 `AppProviders`，例如 `<AppProviders key={generation} store={…}>`；SWR 的键带上 `loginId` 的约定照旧。
3. **429 与重试**：`isRetryable` 目前不重试任何 4xx。M1 引入限流后，`429 rate_limited`（带 `Retry-After`）应当按 `Retry-After` 重试，而不是当作永久拒绝（P5 审查 N5）。
4. **客户端只在一处创建**：oxlint 只允许 `main.tsx`（与测试）导入 `createClient`，services 从 `RootStore` 拿客户端。M1 为每次登录创建带令牌的客户端时，把这件事放进一个专门的会话装配模块，并把 oxlint 的放行从 `main.tsx` 移到那个模块，而不是放宽规则。
