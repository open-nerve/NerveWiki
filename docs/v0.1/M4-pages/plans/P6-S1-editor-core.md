# M4/P6/S1 编辑器分包：实施计划

上级：[P6 文档](../06-P6-source-editor.md) 3.2–3.5、3.11。

## 任务

1. 依赖：`@codemirror/state`、`view`、`commands`、`language`、`search`、`lang-markdown`、`merge`，`@lezer/highlight`，锁定版本（`@lezer/markdown` 只经 `lang-markdown` 间接引入，见 P6 文档 3.2）。
2. `editor/line-breaks.ts`：`splitBreaks`、`joinBreaks`、记录的 `StateField`（`MapMode.TrackAfter` 的点标记）、`invertedEffects` 的撤销恢复。
3. `editor/markdown.ts`、`editor/theme.ts`：`markdownLanguage` 与高亮样式（取应用的 CSS 变量），列表续行与删除标记的键位。
4. `editor/commands.ts`：`Mod+B`、`Mod+K`。`editor/phrases.ts`：查找替换面板的短语，中英两份。
5. `editor/registry.ts`、`editor/extensions.ts`：扩展的类型、`EditorExtensions` context、空的注册表；`composeExtensions`（每个扩展一个 `Compartment`，换掉、卸下、构造抛错的隔离）；`main.tsx` 交给它。
6. `editor/source-editor.tsx`：`SourceEditor`（`EditorView` 只建一次、`setState` 换正文、只读的 `Compartment`、句柄 `text`、`focus`、`composing`、`load`，`onChange(version)`）。
7. 分包体积：先用 `markdown()` 构建一次、再用 `markdownLanguage` 构建一次，记下编辑器分包的体积（gzip 前后），选定后写进第 7 节。
8. `vite.config.ts` 的构建插件：入口 chunk 与它静态引用的 chunk 不含 `@codemirror`、`@lezer`。

## 测试

- 换行与 BOM：往返（CRLF、单独的 `\r`、混合、`\r\r\n`、末尾有无换行、空文档、只有 BOM）；行尾打字、删行尾的字、行首退格、跨行选区删除、回车、粘贴多行、BOM 之后打字；撤销恢复、重做再删。
- 命令：`Mod+B` 加与去、空选区；`Mod+K` 有无选区；列表续行与删除标记（有序、无序、任务项）。
- 管线：三个示例扩展（M5、M6、M7 形态）组合生效、互不干扰、卸下一个、换掉一个、构造抛错被隔离；只读的切换。
- `SourceEditor`：StrictMode 下只有一个编辑器；`load` 之后撤销不回到上一份正文；`onChange` 的 `version` 递增。
- 反向对照：拼回不用标记；映射用 `TrackDel`；撤销不恢复；管线不按注册顺序；构建插件放过 CodeMirror。

## 完成检查

`make check` 为绿；分包体积的两组数字记下；构建产物里编辑器在自己的分包。
