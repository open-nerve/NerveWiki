# M4/P6/S2 编辑模式：实施计划

上级：[P6 文档](../06-P6-source-editor.md) 3.6、3.7、3.9。

## 任务

1. `services/page.service.ts`：`getPageContent`、`putPageContent`、`openEditSession`、`heartbeatEditSession`、`endEditSession`。
2. `stores/edit-session-timing.ts`：`editSessionLease`、`editSessionHeartbeat`，注释与服务端的常量互指。
3. `stores/page-editing.ts`：`PageEditing`：开始（正文与会话并行）、心跳（20 秒、404 重开、403 停止、可见时补一次）、保存的队列与各种答复、结束不等答复。
4. 页面外壳：写者的"编辑"按钮；`document` 上的 `Mod+E`、`Mod+S`（有对话框时不接）；`lazy` 加载编辑器分包；退出先保存、等阅读视图重读、焦点回"编辑"。
5. `page-editing-bar.tsx`：状态的 `<output>`、"保存"、"完成"。
6. 组合中的 `Mod+S`、`Mod+E`：阻止默认、记下，组合结束之后做。
7. `unsaved-guard.tsx`：`useBlocker` 的离开确认（留下、离开），`beforeunload`。
8. 文案：中英两份。

## 测试

- `PageEditing`（假的 service、假计时器）：开始、心跳的间隔与三种答复、可见时补一次、保存的队列（在途时再存）、200 换基准、409 会话结束重开再存一次、503 按 `Retry-After` 至多三次、400、422 的文案、结束；常量的数值。
- 组件（编辑器分包换成一个假的 `SourceEditor`：jsdom 里 CodeMirror 量不了布局）：写者有"编辑"、阅读者没有，阅读者按 `Mod+E` 不进入；`Mod+E` 进入与退出、退出先保存、保存失败留下；组合中的 `Mod+S` 延后；未保存时点树里的链接先确认，留下与离开；`beforeunload` 只在有未保存的修改时拦。
- 反向对照：保存不排队；会话结束不重开；心跳 404 不重开；组合中照存；退出不先保存；离开不确认；阅读者能进入。

## 完成检查

`make check` 为绿；`make build` 之后在浏览器里进入编辑、保存、完成，阅读视图是新内容，没有 CSP 违规。
