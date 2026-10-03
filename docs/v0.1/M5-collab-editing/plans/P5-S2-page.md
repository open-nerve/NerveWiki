# M5/P5/S2 页面：注册、PageEdit 的控制、闲置退出的说明：实施计划

上级：[P5 文档](../05-P5-autosave-idle.md) 3.6–3.9；[M5 总设计](../00-M5-design.md) 4.7–4.9。

## 任务

1. `editor/registry.ts`：`editorExtensions = [lockReadOnly, autosave, idleExit]`，说明随之改。
2. `pages/page/page-edit.tsx`：
   - 控制的 `save`：有冲突时什么也不做，否则照 Mod+S 的 `save()`；
   - 控制的 `leave("idle")`：会话已失时什么也不做；否则经 `composed` 走 `leave()`，离开时 `done({idle: true})`；
   - `PageEditProps.done` 带可选的 `{idle}`。
3. `pages/page/page-layout.tsx`：`PageShell` 记下闲置退出，阅读视图上方显示说明（`role="status"`）；再进入编辑时清掉；焦点照 Done 回到 Edit。
4. 文案：`page.idleLeft`（en："Editing ended after 30 minutes without input."；zh-CN："长时间没有输入，已退出编辑。"）。
5. `pages/page/page-lock.test.tsx`：测锁的用例经 `[lockReadOnly]`，保留一个经组合根 `editorExtensions` 的只读最后一跳。

## 测试

P5 文档第 5 节"组件"的各项，放在 `pages/page/page-autosave.test.tsx`（新，StrictMode，假的计时器）：经组合根的自动保存（停顿之后一次 PUT、"已保存"；组合中不发、确认之后发；Mod+S 之后计时到了不再发；冲突时不抢焦点、不发）；闲置（30 分钟之后回到阅读视图、说明、焦点在 Edit、会话结束；有输入时重新计时；会话已失时不离开；保存失败时留下，下一个 30 分钟再试）；注册表交空时自动保存与闲置的测试失败。反向对照：自动保存不注册；冲突时照 Mod+S；失锁时闲置照样离开；`onClose` 不清计时器（StrictMode 多一次保存）；组合中不等就保存。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿；`page-autosave.test.tsx` 与 `page-lock.test.tsx` 另连跑 10 次。
