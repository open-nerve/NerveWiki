# M5/P5/S1 编辑器：控制、宿主的关闭、nt-3 与两个扩展：实施计划

上级：[P5 文档](../05-P5-autosave-idle.md) 3.2–3.5；[M5 总设计](../00-M5-design.md) 4.7、4.8；[M4/P6 移交](../handoffs/M4-P6-editor.md) 第 1、5 项。

## 任务

1. `editor/registry.ts`：`EditorControls` 加 `onChange(listener): () => void`、`onClose(listener): void`、`leave(reason: "idle"): Promise<void>`；`save()` 的说明（有冲突时什么也不做）。注册表暂不变（S2 注册）。
2. `editor/source-editor.tsx`：
   - 宿主记下变化的监听，正文变了（`docChanged`）时调用；载入新正文建新状态，不调用；
   - P4 的 `following` 扩为状态结束时要做的事：取消 `onSessionChange`、`onChange` 的订阅，调用 `onClose` 的监听；载入与销毁时执行；
   - nt-3：等组合的动作记成 `{act, drop}`；销毁时 `drop`（控制的 `save()` 以 `EditorClosed` 拒绝，句柄的动作不执行）再清空；
   - `SourceEditor` 的 `controls` 属性随之要 `leave`（宿主给 `onChange`、`onClose`，`PageEdit` 给 `leave`）。
3. `editor/autosave.ts`：`autosavePause = 2_000`；`onChange` 时重新计时，到时 `controls.save()` 并接住拒绝；`onClose` 时清掉。返回 `[]`，不在运行时导入 CodeMirror。
4. `editor/idle-exit.ts`：`idleLimit = 30 * 60_000`；建成时计时，`onChange` 时重新计时，到时 `controls.leave("idle")`，完成之后没关闭就重新计时；`onClose` 时清掉。
5. `pages/page/page-edit.tsx`：给 `controls.leave`，暂时直接走 `leave()`（S2 加闲置的说明与失锁的判断）；测试的控制（`source-editor.test.tsx`、`extensions.test.ts`）补上 `leave`。`extensions.test.ts` 的 M5 形态的示例改用 `onChange`、`onClose`。

## 测试

P5 文档第 5 节"单元"的各项：`source-editor.test.tsx`（变化的订阅与载入、关闭的时机含 StrictMode 的第一个宿主、nt-3）、`autosave.test.ts`、`idle-exit.test.ts`（新，假的控制与 `vi.useFakeTimers`）。反向对照：计时不在变化时重置；`onClose` 不清计时器；nt-3 不拒绝；闲置不重新计时；载入新正文也通知变化。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿（含 `editor-out-of-main` 的构建检查：两个扩展不把 CodeMirror 带进主包）。
