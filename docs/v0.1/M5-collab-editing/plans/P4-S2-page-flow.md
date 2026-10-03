# M5/P4/S2 页面：先拿锁、失锁只读、留住外壳：实施计划

上级：[P4 文档](../04-P4-edit-lock-web.md) 3.6–3.11；[M5 总设计](../00-M5-design.md) 4.5、4.9；[M4/P6 移交](../handoffs/M4-P6-editor.md) 第 5、6、8 项。

## 任务

1. `pages/page/page-layout.tsx`：`PageShell` 建编辑、先拿锁再挂 `PageEdit`（进行中、被锁、别的错误）；卸下时结束还在开启的编辑；留住外壳（最后找到的节点）。
2. `pages/page/edit-lock-note.tsx`：`editHere`（持锁人是自己、能写时"在这里编辑"）。
3. `pages/page/page-edit.tsx`：收下编辑；`keep`/`letGo`；离开时等结束；失锁的横幅（`edit-lost-banner.tsx`）与"回到阅读"；控制的 `session`/`onSessionChange`；状态栏不再显示失锁的码。
4. `editor/registry.ts`、`editor/source-editor.tsx`、`editor/lock-read-only.ts`：控制的会话状态与订阅；`lockReadOnly` 注册。
5. `test/render.tsx`：编辑器扩展的参数（选项对象）。`test/page-server.ts`：锁随会话（开启的 409、接管、墓碑码、读锁与解锁由会话算出）。
6. `pages/notebook/notebook-layout.tsx`：留住笔记本。
7. `stores/auth.store.ts`、`stores/root.store.ts`：退出登录先结束编辑（至多 2 秒）。
8. `app/problem-messages.ts`、`pages/notebook/page-tree.tsx`：`page.locked` 带名称。
9. `i18n/messages/en.ts`、`zh-CN.ts`：文案。

## 测试

P4 文档第 5 节"组件"的各项；改写受影响的页面测试（`page-edit.test.tsx`、`conflict.test.tsx`、`page-layout.test.tsx`、`edit-lock-note.test.tsx`、`page-tree-writes.test.tsx`）。反向对照：开会话失败照样编辑；只读的扩展没注册；`letGo` 立刻结束（StrictMode）；页面不在时不留外壳。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿；`pnpm exec knip` 没有新的未用导出。
