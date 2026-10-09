```yaml
status: done
from: M4 收尾
to: M7
created: 2026-10-03
```

# 附件的 Markdown 扩展与粘贴上传：注册与最后一跳

M4 建了两条管线，M7 各注册一个：附件内联是 `platform/markdown` 的扩展，粘贴上传是编辑器的扩展（[M4 总设计](../../M4-pages/00-M4-design.md)第 8 节；总体设计 12.4）。M4 没有注册者，组合根交空集合（[M4 收尾审查](../../M4-pages/reviews/M4-closeout-review.md) C-I3）。

1. **附件内联**（M6 收尾修订：改走 `obsidian.Options` 里附件嵌入的渲染参数，由 M7 建立，见 [M6 的移交](M6-links.md)第 1 项；下面的写法以那里为准）：照 [Markdown 的扩展](../../M6-links/handoffs/M4-P3-markdown-extensions.md)第 1–7 项注册；现在外部与相对图片都渲染为链接（`<span class="nw-image">`，P3 文档 3.6），附件的内联由扩展输出，标记写进 `Markup`，嵌入的展开设预算（那份的第 4 项）。
2. **粘贴上传**：编辑器扩展经 `web/apps/web/src/editor/registry.ts` 注册，按组合的次序与别的扩展互不干扰（`editor/extensions.test.ts` 用 M7 形态的示例证明过）。`EditorControls` 没有上传的手段，M7 加进控制或上下文（见 [M5 的编辑器移交](../../M5-collab-editing/handoffs/M4-P6-editor.md)第 5 项）；粘贴时编辑器可能正处在输入法组合中，插入等 `whenComposed`。
3. **最后一跳的测试**（总体设计 13.1 第 21 条）：两个注册者各在整个程序上加一个行为测试，组合根交空时失败：附件扩展经 `markdownExtensions(resolve, assets)`（M7/P3B 起的形状）到达阅读视图（`GET …/view` 用组合根的实例）；粘贴上传经组合根的 `editorExtensions` 到达页面上的编辑器，用 `test/render.tsx` 的 `renderApp` 的 `editorExtensions` 选项（M5/P4 加的）。
4. 导入若在一个单元里先建后删，见[一个单元里先建后删的节点](M4-P2-unit-merge.md)；附件的活动来源见[它的移交](M3-notebook-activity.md)。

## 处理进展

- M7/P2（2026-10-09，合并 `48c62c0`）：第 4 项的活动来源落实（见[它的移交](M3-notebook-activity.md)）；附件内联与粘贴上传随 P3、P4。
- M7/P3A（2026-10-09，合并 `5138ad6`）：附件进链接的解析与改写；第 1 项附件的内联与第 3 项服务端的最后一跳随 P3B，第 2 项与第 3 项编辑器的一跳随 P4。
- M7/P3B（2026-10-09，合并 `f3bf03c`）：第 1 项落实：附件的内联由方言扩展按 `obsidian.Options` 的 `Assets` 写出（组合根的 `assetEmbeds`），标记写进 `Markup`，`CheckSize` 以一个视图至多 2000 个地址限住附件的总量；第 3 项服务端的一跳是 `bootstrap/assets_view_test.go`（组合根交空时失败）。第 2 项与第 3 项编辑器的一跳随 P4。见 [P3 文档](../03-P3-assets-links.md)第 9.2 节。
- M7/P4C（2026-10-09，合并 `008f81f`）：第 2 项落实：编辑器的扩展 `assetUpload` 经 `editor/registry.ts` 注册（`load`，最后一个），`EditorContext.uploadAsset` 上传，`EditorControls.whenComposed` 等组合（另加 `tell`、`going`）；第 3 项编辑器的一跳是 `pages/page/page-asset-upload.test.tsx`（经组合根的 `editorExtensions`，组合根交空时失败）。四项都已落实，移交关闭。见 [P4 文档](../04-P4-assets-web.md)第 5、9.3 节。
