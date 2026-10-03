```yaml
status: open
from: M4/P6
to: M5
cc: [M6, M7]
created: 2026-10-03
```

# 源码编辑器：自动保存、推送与扩展管线

M4/P6 交付了源码编辑器与页面的编辑模式（[P6 文档](../../M4-pages/06-P6-source-editor.md) 3.3–3.8）：换行写法与 BOM 的记录、保存的队列与冲突、编辑会话、编辑器扩展管线（`editor/registry.ts`、`editor/extensions.ts`）。M5 的只读（锁）与自动保存是管线的第一个注册者，推送是第一个外部更新。

1. **输入法组合**：自动保存在组合中不触发、组合结束后补存。`SourceEditor` 句柄的 `whenComposed(act)` 与扩展拿到的 `controls.save()` 都等组合结束（之后的第一次更新，或 `compositionend` 之后 50 毫秒）。组合进行中编辑器被销毁时，`controls.save()` 的 promise 不 settle（[P6 审查](../../M4-pages/reviews/P6-source-editor-review.md)修复的核对 nt-3）：自动保存要等它时先补上。浏览器始终不发 `compositionend` 时，排队的动作一直等着（P6 审查 Q4）：自动保存时考虑在状态栏提示"等输入法确认"。
2. **推送的外部更新**：组合进行中收到推送的更新时不打断组合（总体设计 9.3：只替换差异、不进撤销历史）。撤销恢复的换行写法按历史映射的位置放回，外部更新不进历史时要复核这一点（`editor/line-breaks.ts` 的 `invertedEffects`）。
3. **人工验收**：按 [P6 的输入法清单](../../M4-pages/manual/P6-ime-checklist.md)补验自动保存与推送的部分（"组合中不触发自动保存、组合结束后补存"，"组合进行中收到别人的更新"），这是 [M0/P1 编辑器移交](../../M4-pages/handoffs/M0-P1-editor.md)第 1 项的后半。
4. **心跳与锁**：见 [M4/P4 的移交](M4-P4-edit-sessions.md)第 6 项：后台标签页的心跳会被节流，有锁之后锁会被别人拿走。
5. **扩展管线**：
   - 只读：扩展的 `controls.setReadOnly(on)`，构造时就能调用，载入新正文（"放弃我的"）照样只读；编辑模式离开时自己的 `hold` 与它分开记，任一为真即只读。
   - 上下文与控制：`EditorContext` 是工作区、笔记本、页面与角色，`EditorControls` 只有 `save()`、`saving()`、`setReadOnly()`（`editor/registry.ts`），没有会话或锁的状态，也没有按代取 store 或 service 的入口。锁的扩展要的状态由 M5 加进上下文或控制；M6 的补全要页面树、M7 的粘贴上传要上传的手段，各自加（[M4 收尾审查](../../M4-pages/reviews/M4-closeout-review.md) B-Q1）。阅读视图的 `ReadingContext` 同样没有写入的手段，M5 的任务复选框按 revision 写正文时一并加。
   - `composeExtensions` 返回的 `reconfigure`（按名字换掉或卸下一个扩展）编辑器没有对外给（P6 审查 m9①）：要从外面换掉时接到 `SourceEditorHandle` 上。
   - **最后一跳的测试**（总体设计 13.1 第 21 条）：管线的组合测试（`editor/extensions.test.ts`）用 M5、M6、M7 形态的示例证明按顺序组合、互不干扰。注册第一个扩展时，经组合根的 `editorExtensions` 加一个页面上的行为测试，注册表交空时失败；`test/render.tsx` 的 `renderApp` 现在只能注入阅读视图的增强，要先给它加编辑器扩展的参数。M6 的补全、M7 的粘贴上传同样各加一个。
6. **别处删掉正在编辑的页**：别的标签页或别人删掉这一页之后，树重读时页面显示 404，编辑器随之卸载，未保存的文字不提醒就丢了（只有本标签页删掉的才去父页；P6 文档 3.10，P6 审查 Q2）。有推送与锁之后，重新设计这时的外壳：例如有未保存的修改时留住编辑器、说明这一页已不在。
7. **删除与新建之后的导航**：M4 收尾的待定项，转给 M5，与第 6 项一起设计：删除之后的去向现在由旧位置渲染的 `<Navigate>` 决定，可能覆盖用户刚发起的导航；新建的答复只看已提交的地址（[P5 审查](../../M4-pages/reviews/P5-tree-reading-review.md) Q3 与修复的核对的 Nit）。
8. **先拆出会话**：`stores/page-editing.ts`（约 390 行）里会话的生命周期（开启、心跳、结束、重开）与保存、冲突在一起。加锁与自动保存之前，先把会话拆成自己的类，`PageEditing` 持有它（M4 收尾审查 B-N5）。
