# M4/P5/S5 快速切换与 e2e：实施计划

上级：[P5 文档](../05-P5-tree-reading.md) 3.10、3.13。

## 任务

1. `app/shortcuts.ts`（`Mod+O` 的识别）；`quick-switch.tsx`（输入框、列表框、键盘、树没读到时的说明与重试）。
2. `e2e/fixtures/wiki-pages.ts`：树、菜单、拖拽、对话框、外壳、快速切换的页面对象。
3. e2e：PG1–PG6、PG11、PG13、PG14 的 "(page)" 版本，PG12 的入口。

## 测试

- vitest：快速切换的查找、键盘、`Mod` 在 macOS 与其余平台；浏览器默认行为被阻止。
- e2e：上面每个故事；之前的故事全部通过。
- 反向对照（服务端或前端变异之后重新 `make build`）：树的写不重读（PG3 的页面版本失败）；高亮不运行（PG5 失败）；阅读者看得到菜单（PG12 失败）。

## 完成检查

`make check`、`make e2e` 为绿。
