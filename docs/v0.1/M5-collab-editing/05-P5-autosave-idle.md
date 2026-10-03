# M5/P5 自动保存与闲置（前端）：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M5/P5 自动保存与闲置（前端） |
| 状态 | 进行中 |
| 基线 | `753dfbf`（P4 合并与它的文档提交之后的 main）；本文与各 Step 计划提交之后开分支 `m5-p5` |
| 上级文档 | [M5 总设计](00-M5-design.md) 第 3 节（C5、C6 的闲置）、4.7、4.8、4.9、4.13、第 7、9 节；[P4 文档](04-P4-edit-lock-web.md)；[M4/P6 移交](handoffs/M4-P6-editor.md) 第 1、3、5 项；[P6 的输入法清单](../M4-pages/manual/P6-ime-checklist.md) |

---

## 1. 基线

前端的调研（Opus，2026-10-04），路径在 `web/apps/web/src` 之下：

- **扩展**：`EditorExtension.extension(context, controls)` 返回 CodeMirror 的扩展，每次建状态（构造、"放弃我的"的 `load()`）建一次；StrictMode 下宿主建两次，第一个随即销毁。注册表在主包里，`build/editor-out-of-main.ts` 在构建时拒绝主包里出现 CodeMirror，所以扩展在运行时不能导入它；`lockReadOnly` 只经控制，返回 `[]`。
- **扩展看不到正文的变化，也没有拆除的时机**：变化只有宿主的 `updateListener` 看得到（计版本、`onChange`、跑等组合的动作）；宿主只在载入与销毁时取消 `onSessionChange` 的订阅。扩展在闭包里起的计时器会活过 `load()`、StrictMode 的第一个宿主与销毁，而它的控制经 `live` 仍然够得到活着的 `PageEdit`。
- **控制**：`save`（经 `whenComposed` 等组合结束）、`saving`、`setReadOnly`、`session`、`onSessionChange`；没有 `leave()`。`SourceEditor` 的 `controls` 属性是去掉 `setReadOnly` 的 `EditorControls`。
- **nt-3**：`destroy()` 不处理等组合的动作，组合中销毁时 `controls.save()` 的 promise 永不 settle（今天没有人等它）。句柄的 `whenComposed` 在编辑器不在之后立即执行；"等组合的动作随编辑器走"的测试钉住句柄的动作销毁之后不执行。
- **保存**：`PageEditing.save` 有一个在途时把最新的一份留到它之后（合并），版本等于已保存的版本时不发；503 带 `Retry-After` 重试 3 次，别的失败显示在状态栏，直到下一次保存。`PageEdit.save()` 在冲突时把焦点移到冲突的标题。离开守卫的对话框开着时保存完成就放行。
- **一次会话一个变更集**：服务端已经这样（`unit_content.go`：会话之后的写并进同一个变更集，`page_revisions` 一行，`revision` 前移）；同样的正文不写。e2e 的 `expectOneSessionRevision` 断言会话活着时的这一行。
- **e2e**：PG7、PG9、PG10 数 PUT、钉 revision；PG7 的"留下"与 C4 的"未保存的修改"要正文停在未保存，2 秒的自动保存会让它们不确定。`page.clock` 覆盖整个浏览器上下文（含 `anotherTab`），快进 30 分钟会让令牌续期、事件流的看门狗重连、会话心跳。CDP 的输入法只有 PG9 用。
- **组件测试**：`page-lock.test.tsx` 经组合根的整个注册表渲染；注册自动保存之后，它"未保存然后失锁"的测试要与真实的 2 秒赛跑。

## 2. 目标与范围

**目标**：编辑器停顿 2 秒就保存，输入法组合中不保存、确认之后补存；30 分钟没有输入就保存、退出编辑、结束会话，阅读视图说明原因；编辑器在组合中销毁时等着的保存以失败结束。

**做**：

- 控制加 `onChange`、`onClose`、`leave(reason)`；宿主在状态结束时取消订阅、调用 `onClose`；nt-3。
- 两个扩展：自动保存（`editor/autosave.ts`）、闲置退出（`editor/idle-exit.ts`），注册在只读之后。
- 页面：自动保存不抢冲突的焦点；闲置的离开；阅读视图上的说明。
- e2e：C5、C6（闲置）的页面版本；改写 PG7、PG9、PG10、C4；人工清单补上 M5 的部分。

**不做**：推送的外部更新（持锁期间用不到，转给 M6）；失败之后的自动重试；状态栏读屏播报的节流（v0.1 收官之后的打磨）。

## 3. 设计

### 3.1 文件

```
web/apps/web/src/
  editor/registry.ts                     控制加 onChange、onClose、leave；注册 autosave、idleExit（3.2）
  editor/source-editor.tsx               变化的订阅、状态结束时的关闭、nt-3（3.3）
  editor/autosave.ts                     自动保存（3.4）
  editor/idle-exit.ts                    闲置退出（3.5）
  pages/page/page-edit.tsx               控制的 save 与 leave（3.6）
  pages/page/page-layout.tsx             闲置退出之后的说明（3.7）
  i18n/messages/en.ts、zh-CN.ts           文案
e2e/fixtures/wiki-pages.ts               holdContentWrites（3.9）
e2e/stories/collab/c5-autosave.spec.ts   C5（新）
e2e/stories/collab/c6-expired.spec.ts    C6 的闲置
e2e/stories/page/pg7、pg9、pg10，collab/c4  改写（3.9）
docs/v0.1/M4-pages/manual/P6-ime-checklist.md  M5 的部分（3.10）
```

### 3.2 控制

`EditorControls` 加三项（`EditorContext` 不变）：

- **`onChange(listener): () => void`**：正文每次变了（用户的输入、撤销，组合的每一步）调用 `listener`；载入新正文（"放弃我的"）不算，它建新的状态。返回取消订阅。
- **`onClose(listener): void`**：这个状态结束时（载入新正文、编辑器销毁，含 StrictMode 的第一个宿主）调用 `listener` 一次。扩展在这里清掉自己的计时器。
- **`leave(reason: "idle"): Promise<void>`**：像 Done 一样离开编辑：先保存，结束会话，回到阅读视图，阅读视图说明原因（3.7）。离不开时（会话已失、有冲突、保存失败）留在编辑里，promise 照样完成。`reason` 只有 `"idle"`，留给以后别的原因。

`save()` 的说明改为：像 Mod+S 一样保存，有冲突时什么也不做（冲突的面板决定）。

宿主把 `onSessionChange`、`onChange` 的取消与 `onClose` 的监听记在每个状态上（P4 的 `following` 扩为关闭时要做的事），载入新正文与销毁时依次调用。扩展因此不会在状态结束之后再收到变化，计时器由 `onClose` 清掉。

### 3.3 nt-3：组合中销毁

等组合的动作记成 `{act, drop}`：

- 句柄的 `whenComposed(act)`：`drop` 什么也不做（照"等组合的动作随编辑器走"的测试，销毁之后不执行）；
- 控制的 `save()`：`drop` 以 `EditorClosed`（新的错误类）拒绝它的 promise。

销毁时依次 `drop` 等着的动作再清空；载入新正文不动它们（视图还在，组合结束之后照样执行）。等 `save()` 的扩展都接住拒绝。

### 3.4 自动保存

`editor/autosave.ts`，`autosavePause = 2_000`（前端的常量，不进配置）：

- `onChange` 时重新计时，到时调用 `controls.save()`，接住它的拒绝（失败已显示在状态栏，`EditorClosed` 无须处理）；`onClose` 时清掉计时器。
- 组合：`controls.save()` 等组合结束（M4/P6），所以组合中停顿 2 秒也不发，确认之后随即补存；组合的每一步都是变化，重新计时。浏览器始终不发 `compositionend` 时，保存一直等着（M4/P6 审查 Q4，接受，状态栏照常显示"有未保存的修改"）。
- Mod+S 在计时中按下：立即保存；之后计时到了，版本等于已保存的，什么也不发。保存在途时再到：`PageEditing` 合并成一份。
- 失败不自动重试：下一次停顿、Mod+S、闲置的离开再试；503 照 M4 由 store 重试。
- 失锁之后：`PageEditing` 拒绝（`current()` 以 `EditEnded` 拒绝），什么也不发、不显示失败（P4），扩展不必知道。

### 3.5 闲置退出

`editor/idle-exit.ts`，`idleLimit = 30 * 60_000`（前端的常量）：

- 建成时开始计时，`onChange` 时重新计时；到时调用 `controls.leave("idle")`，完成之后状态还在（离不开）就重新计时，下一个 30 分钟再试；`onClose` 时清掉。
- 隐藏的标签页同样计时，计时器被节流时至多晚约一分钟（总体设计 4.7）；被冻结的标签页在恢复之后才到时，其间锁由租约兜底。
- 组合：离开像 Done 一样等组合结束（3.6），组合中不离开。

### 3.6 PageEdit 的控制

- **`save`**：有冲突时什么也不做（不把焦点移到冲突的标题，那是 Mod+S 的）；否则照 Mod+S 的 `save()`。
- **`leave("idle")`**：
  - 会话已失：什么也不做，横幅留着（有未保存的修改时只能由用户复制、离开）；
  - 否则经 `composed` 走 `leave()`：先保存，结束会话，回到阅读视图，并告诉 `PageShell` 原因（`done({idle: true})`）；
  - 保存失败或有冲突时 `leave()` 本来就留在编辑里（照 M4），锁照旧持有，下一个 30 分钟再试。

### 3.7 闲置退出之后的阅读视图

`PageShell` 的 `done` 带可选的 `{idle}`；闲置退出时阅读视图上方显示一行说明（`role="status"`，不是 `refusal` 的错误样式）："长时间没有输入，已退出编辑。"。焦点照 Done 回到 Edit。再进入编辑时清掉；`PageShell` 按页 id 换，离开这一页也就没了。

### 3.8 状态栏与离开守卫

- 状态栏不变：自动保存时照样显示"正在保存""已保存"，经 `<output>` 礼貌地播报；每 2 秒一次的播报算不算吵，留给 v0.1 收官之后的打磨。
- 离开守卫照 M4：有未保存的修改时先问，对话框开着时保存完成就放行；有了自动保存，这个窗口约 2 秒（总体设计 4.8）。

### 3.9 测试的接线

- **组件**：`renderApp` 默认不注册扩展（M4/P6）。`page-lock.test.tsx` 测锁，经 `[lockReadOnly]`；保留一个经组合根 `editorExtensions` 的最后一跳（只读）。自动保存与闲置各一个经组合根的页面测试，注册表交空时失败；它们用假的计时器（`vi.useFakeTimers({shouldAdvanceTime: true})`，`advanceTimersByTimeAsync`）。
- **e2e 的夹具** `holdContentWrites(page, pageId)`：扣住这一页正文的 `PUT`（`page.route`，照 P3 的 `holdStream`），返回放行。扣住时保存在途，正文停在未保存（`version` 不等于已保存的），故事因此确定地有"未保存的修改"，不靠赶在 2 秒之内。
- **改写**：
  - PG7：两次 Ctrl+S 仍是一个变更集；"留下"之前扣住写，放行之后由 Ctrl+E 离开；revision 不再钉死中间值，断言最后的正文与会话的那一行；
  - PG9：字节的故事等"已保存"与落库，不数 PUT；输入法的故事改为组合中不发、确认之后发出、落库是确认的字；
  - PG10：会话过期之后的重开：等"已保存"与落库，PUT 的清单只断言最后一个是 200、其中有一个 409 `edit_session_ended`；
  - C4：A 输入之前扣住写，删页之后横幅说有未保存的修改；离开之后放行，答 404，在 `expectConsole` 里声明；
  - 别的故事（C1–C3、C6、PG8、PG12）核对之后不改，PG8 的冲突中自动保存什么也不发。
- **C5（页面）**：停顿约 2 秒保存（不按键，状态"已保存"，落库）；再输入、再停顿，仍是一个变更集（`expectOneSessionRevision`，不钉 revision）；输入法组合中停顿超过 2 秒不发，确认之后发出确认的字（CDP 的 `Input.imeSetComposition`、`Input.insertText`）；Ctrl+S 立即保存（扣住之前计数，按下之后不到 2 秒就有 PUT：页面的时钟 `install` 之后 `pauseAt`，计时器不走，PUT 只能来自 Ctrl+S）。
- **C6（页面，闲置）**：页面的时钟 `install`，A 编辑、保存，快进 30 分钟：阅读视图说明长时间没有输入，焦点在 Edit；会话已删；B 随即能编辑。快进带来的令牌续期、事件流重连、心跳照样发生，故事不对它们做断言，有 4xx 时在 `expectConsole` 里声明。

### 3.10 人工清单

`P6-ime-checklist.md` 补上 M5 的部分（M4/P6 移交第 3 项，负责人在 M5 收尾之后执行整份）：

- 组合中停顿超过 2 秒，网络面板没有保存的请求；确认之后约 2 秒一次保存，内容是确认的字；
- 组合中这一页的树或锁有推送（另一个标签页改名、另一账户读锁），组合不被打断；
- 已有各行的"有未保存的修改""一次保存"按自动保存改写（停顿之后变为"已保存"）。

## 4. 实施步骤

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 编辑器：控制的 `onChange`、`onClose`、`leave`；宿主的订阅与关闭；nt-3；自动保存与闲置的扩展（未注册） | [P5-S1](plans/P5-S1-editor.md) |
| S2 | 页面：注册两个扩展；`PageEdit` 的 `save` 与 `leave`；闲置退出的说明；组件测试 | [P5-S2](plans/P5-S2-page.md) |
| S3 | 端到端：`holdContentWrites`；C5、C6 的闲置；改写 PG7、PG9、PG10、C4；人工清单 | [P5-S3](plans/P5-S3-e2e.md) |

## 5. 测试与验证

- **单元**（vitest）：
  - 宿主：`onChange` 只对正文的变化、载入新正文不算；载入与销毁时取消订阅、调用 `onClose`（StrictMode 的第一个宿主也调用）；nt-3：组合中销毁，`controls.save()` 以 `EditorClosed` 拒绝，句柄的动作不执行；
  - 自动保存（假的控制与计时器）：停顿 2 秒保存一次；连续输入只在最后一次之后 2 秒保存；`onClose` 之后不保存；拒绝被接住；
  - 闲置（同上）：建成 30 分钟后离开；变化重新计时；离不开时重新计时；`onClose` 之后不离开。
- **组件**（`renderApp`，StrictMode）：经组合根的自动保存（停顿之后 PUT 一次，状态"已保存"；组合中不发，确认之后发；冲突时不抢焦点、不发）；闲置（30 分钟之后回到阅读视图，说明与焦点；会话已失时不离开；保存失败时留下）；注册表交空时这两项失败。
- **e2e**：3.9。
- **反向对照**：
  - 自动保存不注册（组合根的组件测试失败、C5 失败）；
  - 计时不在变化时重置（单元测试失败）；
  - `onClose` 不清计时器（StrictMode 的组件测试多保存一次或离开）；
  - 组合中不等就保存（组合的组件测试、C5 的输入法失败）；
  - nt-3 不拒绝（单元测试超时）；
  - 冲突时照 Mod+S（焦点的组件测试失败）；
  - 闲置不重新计时（单元测试失败）；
  - 失锁时闲置照样离开（组件测试失败）；
  - 不扣住写（C4、PG7 的"留下"不确定：以重复跑核实扣住的必要，不作对照）。

## 6. 完成标准

- `GOFLAGS=-p=3 make check`、`make gen-check`、`make e2e` 为绿；上面的反向对照都按预期失败。
- 合并之前经 Opus 审查，修复之后另由 Opus 核对。

## 7. 结果

（合并之后填写。）
