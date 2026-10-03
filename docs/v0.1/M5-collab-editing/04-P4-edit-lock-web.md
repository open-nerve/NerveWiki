# M5/P4 编辑锁（前端）：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M5/P4 编辑锁（前端） |
| 状态 | 进行中 |
| 基线 | `29dd7e5`（P3 合并与它的文档提交之后的 main）；本文与各 Step 计划提交之后开分支 `m5-p4` |
| 上级文档 | [M5 总设计](00-M5-design.md) 第 3 节（C1–C4、C6）、4.1–4.9、4.13、第 9、11 节；[P1 文档](01-P1-edit-lock.md)（服务端的码与成员）；[P3 文档](03-P3-event-stream-web.md)（事件流、`EditLockNote`）；[M4/P6 移交](handoffs/M4-P6-editor.md) 第 5、6、8 项；[M4/P4 移交](handoffs/M4-P4-edit-sessions.md) 第 6 项 |

---

## 1. 基线

前端的调研（Opus，2026-10-04），路径在 `web/apps/web/src` 之下：

- **编辑**：`stores/page-editing.ts` 的 `PageEditing`（约 390 行）同时管正文、保存的队列、冲突与会话。`start()` 同时开会话与读正文，开会话失败照样编辑（M4 的"失败也能编辑"）；心跳每 20 秒、重新可见时一次；心跳 404 悄悄重开，403 记为失去访问，别的错误（含 409 `taken_over`、`unlocked`）下一拍再试；保存遇 `page.edit_session_ended` 重开一次再发，别的 409 只在状态栏显示通用文案，编辑器照样可写。`end()` 发出结束不等答复；`end()` 之后再 `start()` 重开（StrictMode）。
- **页面**：`PageShell` 的 `editing` 状态在 `ReadingView` 与 `PageEdit` 之间切换，`PageEdit` 挂上时以 `useNewPageEditing` 建 `PageEditing` 并 `start()`。树里没有这一页时 `PageLayout` 显示 404 或去父页，编辑器随之卸载，未保存的文字丢了（M4/P6 移交第 6 项）。
- **服务与会话**：`openEditSession(id)` 不带请求体（契约已有 `{take_over}`）；problem 的 `lock`、`ended_by` 已在生成的类型里，`ApiError.problem` 拿得到。`Session.public` 是不挂认证中间件的客户端；`TokenManager.accessToken()` 是异步的，没有同步取令牌的方法。openapi-fetch 没有中间件时同步调用 `fetch`，选项里的 `keepalive` 进 `Request`。
- **编辑器**：注册表（`editor/registry.ts`）是空的；`EditorControls` 只有 `save`、`saving`、`setReadOnly`，没有会话的状态，也没有把变化推给扩展的办法；注册表在主包里，扩展不能在运行时导入 CodeMirror。`renderApp` 没有编辑器扩展的参数。
- **生命周期**：`pagehide`、`pageshow` 只在 P3 的事件 hub 里经 `PageLifecycle`，监听不带事件（拿不到 `persisted`）；`EventDeps` 在测试的 `signedInApp` 里没有。
- **退出登录**：会话的结束经这一代的客户端，换代之后以 `SessionChangedError` 失败，锁留到租约过期（2 分钟）。
- **测试**：`page-editing.test.ts` 的假服务记 `OPEN`、`BEAT`、`PUT`、`END`；`test/page-server.ts` 的开启从不拒绝，锁是静态的；e2e 的 `startEditing` 等编辑器拿到焦点，C1–C4、C6 只有接口版本。

## 2. 目标与范围

**目标**：先拿锁再编辑；同一页同时只有一个编辑器能写；接管、强制解锁、页面不在、失去访问、过期之后被别人拿走，编辑器都切为只读并说明原因，正文留着可以复制；关闭标签页、退出编辑、退出登录立即释放锁。

**做**：

- `stores/edit-session.ts`：`EditSession`（开启与接管、心跳、重新可见时心跳、锁事件时心跳、结束、`keepalive` 的释放、从 bfcache 回来、会话为什么结束）；`PageEditing` 持有它，只管正文、保存的队列与冲突。
- 服务与会话：`openEditSession(id, {takeOver})`；`keepalive` 的结束（不挂中间件的客户端）；`TokenManager.currentAccessToken(loginId)`；`PageLifecycle` 的监听带 `persisted`。
- 页面：先拿锁再进入编辑；被锁着时停在阅读视图，`EditLockNote` 显示持锁人，自己在别处时给"在这里编辑"；失锁时只读的横幅与"回到阅读"；页面或笔记本不在而有未保存的修改时留住外壳；`page.locked` 带名称的文案（删除的拒绝）；退出登录先结束编辑。
- 编辑器：控制加会话的状态与它的变化；第一个注册的扩展"失锁时只读"；`renderApp` 加编辑器扩展的参数。
- 文案；e2e：C1–C4、C6（退出、关闭）的页面版本，改写受影响的故事。

**不做**：自动保存与闲置、控制的 `leave()`（P5）；任务项（P6）；工作区被删时留住编辑器（见 3.9）。

## 3. 设计

### 3.1 文件

```
web/apps/web/src/
  stores/edit-session.ts                        EditSession（3.2、3.3）
  stores/page-editing.ts                        持有 EditSession；begin、keep/letGo、end（3.4）
  stores/root.store.ts、stores/auth.store.ts    editPage 的依赖；正在编辑的记录；退出登录先结束编辑（3.4、3.9）
  services/page.service.ts                      openEditSession 的 takeOver；EditLeaveService（3.5）
  session/token-manager.ts                      currentAccessToken（3.5）
  events/hub.ts、events/deps.ts                 PageLifecycle 的监听带 persisted；browserPageLifecycle（3.3）
  editor/registry.ts、editor/source-editor.tsx  控制的会话状态与订阅；lockReadOnly 注册（3.7）
  pages/page/page-layout.tsx                    先拿锁；留住外壳（3.6、3.9）
  pages/page/page-edit.tsx、edit-lost-banner.tsx 失锁的横幅（3.8）
  pages/page/edit-lock-note.tsx                 在这里编辑（3.6）
  pages/notebook/notebook-layout.tsx            留住笔记本（3.9）
  app/problem-messages.ts、pages/notebook/page-tree.tsx  page.locked 带名称（3.10）
  i18n/messages/en.ts、zh-CN.ts                 文案
  test/render.tsx、test/page-server.ts          编辑器扩展的参数；锁随会话（3.11）
e2e/fixtures/wiki-pages.ts、pages.ts            进入编辑被拒；开启带接管（3.12）
e2e/stories/collab/c1–c4、c6                    页面版本
```

### 3.2 EditSession：开启与码

`EditSession` 是一页的一个编辑会话（M5 总设计 4.1–4.3、4.6、4.7），MobX 可观察的只有 `lost`（为什么不能再写）与 `id`：

- **`open(takeOver)`**：`POST …/edit-sessions`，`takeOver` 时带 `{take_over: true}`。答 `{opened: true}`，或 409 `page.locked` 时 `{opened: false, lock}`（problem 的 `lock`）；别的错误照样抛出。开到之后开始心跳。
- **`current()`**：给保存用的会话 id；没有（重开中）就等重开。
- **心跳**每 20 秒（`editSessionHeartbeat`，不变），重新可见时、收到这一页的锁事件（`session_id` 是自己的）时、事件流每次连上时各一次。
- **码**（总体设计 4.3 的"编辑器的规则"）：

| 来自 | 答复 | 做什么 |
|---|---|---|
| 心跳 | 404 `page.edit_session_not_found`（过期、不见了） | 重开，不带接管 |
| 保存 | 409 `page.edit_session_ended` | 重开，不带接管，再发一次（照 M4） |
| 重开 | 409 `page.locked` | 失锁：过期之后被拿走（`taken`，带持锁人，是自己时说"你在别处") |
| 重开、保存 | 404 `page.not_found` | 失锁：页面不在（`gone`） |
| 心跳、保存 | 409 `page.edit_session_taken_over` | 失锁：被接管（`taken_over`） |
| 心跳、保存 | 409 `page.edit_session_unlocked` | 失锁：被解锁（`unlocked`，带 `ended_by` 的名称） |
| 心跳、重开、保存 | 403 | 失锁：失去访问（`no_access`），取代 M4 的 `lostAccess` |
| 心跳 | 别的（网络） | 下一拍再试 |

- 失锁之后不再心跳、不再重开、不再保存；M4 的"保存成功就清掉失去访问"随之去掉。
- **`end()`**：停止心跳，`DELETE` 会话，返回它的 promise（离开编辑等它，见 3.6）；失锁或已结束时什么也不发。
- 重开同时只有一个（照 M4 的 `opening`）；结束之后到的开启答复随即结束它（照 M4）。

### 3.3 EditSession：页面的生命周期

- **`pagehide`**：同步地以 `keepalive` 结束会话（3.5），记下"离开时释放了"。令牌已过期就不发，由租约兜底（总体设计 4.7）。
- **`pageshow` 且 `persisted`**（从 bfcache 回来）：先对旧会话心跳。还活着（释放没发出去）就接着用；答 404 就照上表重开，锁空着就拿回，被别人拿了就失锁。不先心跳就重开会被自己的旧会话锁住。
- **`visibilitychange`** 到可见：心跳一次（照 M4）。
- 生命周期经注入的 `PageLifecycle`：P3 的类型，监听加一个可选的参数 `{persisted?: boolean}`（hub 不用它）。`events/deps.ts` 拆出 `browserPageLifecycle()`，`browserEventDeps` 与 `RootStore` 共用；没有 `EventDeps` 的测试用它（jsdom 的 `window`、`document`），单元测试用 `FakePage`（`fire` 加 `persisted`）。

### 3.4 PageEditing 与接线

- `PageEditing` 持有一个 `EditSession`，管正文、保存的队列、冲突：
  - **`begin(takeOver)`**：开会话，拿到了才读正文；答 `{opened}` 或 `{opened: false, lock}`；
  - 保存经 `session.current()` 取会话，把保存的错误交给会话判定（3.2 的表），失锁就不再保存；
  - **`keep()` / `letGo()`**：编辑的视图挂上时 `keep()`，卸下时 `letGo()`：`letGo()` 之后一个任务（`setTimeout` 0）之内没再 `keep()` 就 `end()`。StrictMode 的卸下与再挂在同一个任务里，会话留着；M4 的"结束之后再 `start()` 重开"去掉：有锁之后，重开会撞上自己还没结束的会话。
  - **`end()`**：结束会话（等它答复），排队的保存不再发（照 M4）。
- `RootStore.editPage(id)` 交给 `PageEditing` 依赖：页面的服务、生命周期（`app.events?.page ?? browserPageLifecycle()`）、锁事件的订阅（这一代的 hub，没有就不订阅）、`keepalive` 的结束（3.5）。
- **正在编辑的记录**：`RootStore` 记下开着的编辑（可观察；`editPage(notebookId, pageId)`，编辑知道自己的笔记本），`begin` 拿到锁时记上，`end` 时去掉（失锁之后还记着：页面不在时外壳靠它留住）。外壳的留住（3.9）与退出登录（3.9）读它。

### 3.5 keepalive 的结束与同步的令牌

- `TokenManager.currentAccessToken(loginId)`：这一代的访问令牌还没到期（不留 30 秒的余量）就给出，否则 `undefined`。同步，不续期。
- `services/page.service.ts` 加 `EditLeaveService`：用 `Session.public`（不挂认证中间件），`endOnLeave(id, token)` 同步地发出 `DELETE /api/v0/edit-sessions/{id}`，带 `Authorization` 与 `keepalive: true`，不等答复、不抛错。
- `RootStore` 在构造时建它，交给 `EditSession` 的是 `(id) => { const token = …currentAccessToken(loginId); if (token) leave.endOnLeave(id, token) }`。

### 3.6 先拿锁再编辑

`PageShell` 建编辑并持有它（`useNewPageEditing` 去掉），拿到锁之后交给 `PageEdit`；`PageShell` 卸下时结束还在开启的编辑。进入编辑（Edit 按钮、Mod+E）：

1. 进行中：按钮 `aria-busy`、禁用，再按不重复；
2. `editPage(id).begin(takeOver)`：
   - 拿到：挂 `PageEdit`（照 M4 读正文、挂编辑器、焦点）；
   - 409 `page.locked`：停在阅读视图，`mutate(["edit-lock", id])` 让 `EditLockNote` 读到持锁人，焦点到它；
   - 别的错误：阅读视图上方显示原因（`errorText`），焦点留在 Edit。
3. `EditLockNote` 加一个可选的 `editHere`：持锁人是自己、而这个账户能写这一页时显示"在这里编辑"，按下即以接管进入编辑（负责人的决定 1）。

离开编辑（Done、Mod+E）：照 M4 先保存，再 `await end()`（会话的结束答复之后），再回到阅读视图；之后 `EditLockNote` 读锁时这个会话已经不在，不会闪一下"你正在别处编辑"。

### 3.7 编辑器：会话的状态与只读的扩展

- `EditorControls` 加 `session(): {lost: boolean}` 与 `onSessionChange(listener): () => void`；`EditorContext` 不变。`SourceEditor` 的宿主把订阅记在每个状态上，载入新正文与销毁时取消。`PageEdit` 用 MobX 的 `reaction` 实现订阅。
- **`lockReadOnly`**（`editor/lock-read-only.ts`）：第一个注册的扩展，`extension(context, controls)` 里按 `controls.session().lost` 调 `controls.setReadOnly`，订阅它的变化；返回空的扩展（不导入 CodeMirror，留在主包里）。`editorExtensions = [lockReadOnly]`。
- **最后一跳**：`renderApp` 加编辑器扩展的参数（选项对象，默认空）。页面上的行为测试经组合根的 `editorExtensions` 证明失锁时编辑器只读，注册表交空时失败（M4/P6 移交第 5 项）。

### 3.8 失锁的横幅

`PageEdit` 在 `editing.session.lost` 时，编辑器之上显示横幅（`role="alert"`，焦点移过去）：

- 原因：被接管"你在另一处继续编辑了这一页"；被解锁"{name} 解除了你的编辑"；页面不在"这一页已不在"；失去访问（M4 的 `editor.lostAccess`）；过期之后被拿走"你的编辑已过期，{name} 正在编辑这一页"（是自己时"……你正在别处编辑这一页"）。
- 有未保存的修改时加一句"这里有没保存的修改：离开之前先复制出来"。
- "回到阅读"：结束编辑、回到阅读视图，不保存；有未保存的修改时先问（M4 的离开对话框）。
- 状态栏不再显示这些码的通用文案；Mod+S 什么也不做。

### 3.9 页面或笔记本不在；退出登录

- **留住外壳**（M4/P6 移交第 6 项）：`PageLayout` 记下最后找到的树节点；这一页从树里没了、而这个标签页正在编辑它且有未保存的修改时，照旧渲染 `PageShell`（用最后的节点），编辑器经心跳或锁事件得知 `gone`。没有未保存的修改时照 M4（404，本标签页删的去父页）。`NotebookLayout` 同样：笔记本从列表里没了、而里面有一页正被编辑且有未保存的修改时，留住最后的笔记本。
- 工作区被删时不留住：`WorkspaceLayout` 卸下整个工作区，P5 的自动保存之后未保存的至多约 2 秒。记在第 7 节。
- **退出登录**：`AuthStore.signOut` 先经这一代的客户端结束这一代所有开着的编辑，等它们答复（至多 2 秒，断网时不让退出卡住），再照 M1 退出。不用 `keepalive` 的同步结束：它与随后的登出同时在途，服务端先处理登出时，结束会被拒（凭证已撤销），锁留到租约过期。同一账户的别的标签页随存储事件换代，它们的结束经旧一代的客户端以 `SessionChangedError` 失败，锁由租约兜底（至多 2 分钟），记在第 7 节。

### 3.10 page.locked 带名称

总体设计 4.5：`errorText` 的选项加 `lock: {me, title(pageId)}`；有它而错误是 `page.locked`、带 `lock` 成员时，说"{name} 正在编辑「{page}」"，是自己时"你正在别处编辑「{page}」"，页的标题找不到时不带页名。页面树的删除对话框用它（删除这一页或它的上级页被拒）。

### 3.11 测试的服务

`test/page-server.ts`：锁随会话：`OPEN` 在这一页有活着的会话时答 409 `page.locked`（带 `lock`），带接管且是本人时把旧会话记为被接管；心跳、保存对墓碑答 `taken_over`、`unlocked`（带 `ended_by`）；`GET edit-lock` 由会话算出；`DELETE edit-lock` 把会话记为被解锁。测试改服务端的状态模拟别的标签页与管理员。

### 3.12 端到端

- **夹具**：`wiki-pages.ts` 的 `startEditing` 不变（拿到锁之后编辑器才拿到焦点）；加 `editRefused(page)`（按 Edit 之后停在阅读视图、显示持锁人）与 `lostBanner(page)`；`pages.ts` 的开启带 `takeOver`。
- **C1（页面）**：A 在页面上编辑；B（另一账户）打开同一页看到"A 正在编辑"，按 Edit 被拒、停在阅读视图；A 退出编辑之后 B 能编辑。落库：至多一个活着的会话。
- **C2（页面）**：同一账户的第二个标签页（`anotherTab`）按 Edit：说"你正在别处编辑"，给"在这里编辑"；按下之后第二个编辑，第一个标签页的编辑器只读并说明被接管（经锁事件，不等心跳）；落库：第一个会话是 `taken_over` 的墓碑。
- **C3（页面）**：笔记本的管理员（另一账户的页面）解除 A 的锁：A 的编辑器只读并说明是谁解除的，之前保存的都在。
- **C4（页面）**：A 持锁时 B 在树里删这一页、删它的上级页，对话框说"A 正在编辑「…」"；A 在别的标签页删掉这一页：A 的编辑器只读并说明页面已不在，有未保存的修改时外壳留着。
- **C6（页面）**：A 退出编辑之后 B 随即能编辑；A 关掉标签页之后 B 随即能编辑（核实 `keepalive` 的请求到达：会话已删）。
- **改写**：PG8、PG10 的页面版本照新的进入方式；离开页面时的 `keepalive` 结束是新的请求，故事的请求清单与 `apiFailures` 照此核对；被拒的开启与失锁的心跳、保存在 `expectConsole` 里声明 409。

## 4. 实施步骤

| Step | 内容 | 计划 |
|---|---|---|
| S1 | `EditSession`、`PageEditing` 的拆分、服务与令牌、生命周期 | [P4-S1](plans/P4-S1-edit-session.md) |
| S2 | 页面：先拿锁、在这里编辑、失锁的横幅与只读的扩展、留住外壳、退出登录释放、带名称的 `page.locked` | [P4-S2](plans/P4-S2-page-flow.md) |
| S3 | 端到端：C1–C4、C6 的页面版本，改写受影响的故事 | [P4-S3](plans/P4-S3-e2e.md) |

## 5. 测试与验证

- **单元**（vitest，假的服务、`FakePage`、锁事件与计时器）：
  - `EditSession`：开启（拿到、被锁、接管）；3.2 表里的每一行；失锁之后不再心跳与重开；锁事件与连上时心跳；`pagehide` 的 `keepalive`（令牌过期时不发）；从 bfcache 回来先心跳（活着接着用，404 重开，被拿走失锁）；`end()` 之后的开启答复随即结束；
  - `PageEditing`：拿到锁才读正文；保存的失锁码交给会话；`keep`/`letGo`（同一个任务里再挂不结束，之后结束）；
  - `currentAccessToken`：到期与换代时没有；
  - `EditLeaveService`：同步发出、带令牌与 `keepalive`。
- **组件**（`renderApp`，StrictMode 的页面测试照旧）：被锁时停在阅读视图并显示持锁人；自己在别处时"在这里编辑"以接管进入；失锁的五种原因与横幅；"回到阅读"（有未保存的修改先问）；页面不在而有未保存的修改时外壳留着、没有时 404；笔记本同理；退出登录先结束编辑；删除被拒的带名称文案；最后一跳（注册表交空时只读的测试失败）。
- **e2e**：3.12。
- **反向对照**：
  - 开会话失败照样编辑（被锁的组件测试、C1 失败）；
  - 重开带接管（"过期之后被拿走"的单元测试失败）；
  - `taken_over` 照旧重开（C2 失败）；
  - `pagehide` 不释放（C6 的关闭失败）；
  - 从 bfcache 回来不先心跳就重开（单元测试失败）；
  - `letGo` 立刻结束（StrictMode 的组件测试失败）；
  - 锁事件不心跳（C2、C3 的页面版本超时）；
  - 只读的扩展没注册（最后一跳失败）。

## 6. 完成标准

- `GOFLAGS=-p=3 make check`、`make gen-check`、`make e2e` 为绿；上面的反向对照都按预期失败。
- 合并之前经 Opus 审查，修复之后另由 Opus 核对。

## 7. 结果

（合并之后填写。）
