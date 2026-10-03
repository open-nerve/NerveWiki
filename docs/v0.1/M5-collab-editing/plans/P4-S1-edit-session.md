# M5/P4/S1 EditSession 与 PageEditing 的拆分：实施计划

上级：[P4 文档](../04-P4-edit-lock-web.md) 3.2–3.5；[M5 总设计](../00-M5-design.md) 4.3、4.6、4.7。

## 任务

1. `session/token-manager.ts`：`currentAccessToken(loginId)`（同步；这一代、未到期才给出）。
2. `services/page.service.ts`：`openEditSession(id, {takeOver})`；`EditLeaveService.endOnLeave(id, token)`（`Session.public`，`keepalive`，同步发出、不等不抛）。
3. `events/hub.ts`、`events/deps.ts`：`PageLifecycle` 的监听带可选的 `{persisted}`；`browserPageLifecycle()`，`browserEventDeps` 用它；`FakePage.fire` 可带 `persisted`。
4. `stores/edit-session.ts`：`EditSession`（`open`、`current`、心跳与它的触发、码的表、`lost`、`end`、`pagehide` 的释放、`pageshow` 的先心跳）。
5. `stores/page-editing.ts`：持有 `EditSession`；`begin(takeOver)`（拿到锁才读正文）、`keep`/`letGo`、`end()` 返回 promise；去掉 `start`、`lostAccess`、自己的心跳与可见监听；保存的失锁码交给会话。
6. `stores/root.store.ts`：`editPage(notebookId, pageId)` 的依赖（服务、生命周期、锁事件、离开时的结束）；正在编辑的记录。页面暂照旧接线（S2 改为先拿锁）：`PageEdit` 挂上时 `begin(false)`，保持现有页面测试为绿。

## 测试

P4 文档第 5 节"单元"的各项：`edit-session.test.ts`（新）、`page-editing.test.ts`（改写会话部分）、`token-manager.test.ts`、`page.service` 的离开。反向对照：重开带接管；`taken_over` 照旧重开；从 bfcache 回来不先心跳就重开；`letGo` 立刻结束；锁事件不心跳；令牌过期仍发。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿；`edit-session.test.ts` 另连跑 10 次。
