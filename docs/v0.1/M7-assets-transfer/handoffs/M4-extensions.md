```yaml
status: open
from: M4 收尾
to: M7
created: 2026-10-03
```

# 附件的 Markdown 扩展与粘贴上传：注册与最后一跳

M4 建了两条管线，M7 各注册一个：附件内联是 `platform/markdown` 的扩展，粘贴上传是编辑器的扩展（[M4 总设计](../../M4-pages/00-M4-design.md)第 8 节；总体设计 12.4）。M4 没有注册者，组合根交空集合（[M4 收尾审查](../../M4-pages/reviews/M4-closeout-review.md) C-I3）。

1. **附件内联**：照 [Markdown 的扩展](../../M6-links/handoffs/M4-P3-markdown-extensions.md)第 1–7 项注册；现在外部与相对图片都渲染为链接（`<span class="nw-image">`，P3 文档 3.6），附件的内联由扩展输出，标记写进 `Markup`，嵌入的展开设预算（那份的第 4 项）。
2. **粘贴上传**：编辑器扩展经 `web/apps/web/src/editor/registry.ts` 注册，按组合的次序与别的扩展互不干扰（`editor/extensions.test.ts` 用 M7 形态的示例证明过）。`EditorControls` 没有上传的手段，M7 加进控制或上下文（见 [M5 的编辑器移交](../../M5-collab-editing/handoffs/M4-P6-editor.md)第 5 项）；粘贴时编辑器可能正处在输入法组合中，插入等 `whenComposed`。
3. **最后一跳的测试**（总体设计 13.1 第 21 条）：两个注册者各在整个程序上加一个行为测试，组合根交空时失败：附件扩展经 `markdownExtensions()` 到达阅读视图（`GET …/view` 用组合根的实例）；粘贴上传经组合根的 `editorExtensions` 到达页面上的编辑器，用 `test/render.tsx` 的 `renderApp` 的 `editorExtensions` 选项（M5/P4 加的）。
4. 导入若在一个单元里先建后删，见[一个单元里先建后删的节点](M4-P2-unit-merge.md)；附件的活动来源见[它的移交](M3-notebook-activity.md)。
