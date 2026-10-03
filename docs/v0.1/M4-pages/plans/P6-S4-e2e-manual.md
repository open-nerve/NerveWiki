# M4/P6/S4 e2e 与人工验收：实施计划

上级：[P6 文档](../06-P6-source-editor.md) 3.12、3.13。

## 任务

1. `fixtures/wiki-pages.ts`：编辑器的页面对象（进入、输入、保存、完成、冲突区、离开确认）。
2. PG7–PG10 的页面版本，PG12 的编辑入口（3.12）。
3. 组合输入：Chromium 的 CDP（`Input.imeSetComposition`、`Input.insertText`），组合中 `Mod+S` 不发请求，确认之后存进确认的文字，CRLF 页面的其余字节不变。
4. `docs/v0.1/M4-pages/manual/P6-ime-checklist.md`：三种浏览器、真实中文输入法的清单。
5. 第 7 节：分包体积、给 M5 的移交内容。

## 测试

- e2e 全部通过（之前的故事照旧）。
- 反向对照（e2e，留日志核对失败原因）：拼回不用标记（PG9）；保存不带会话（PG7 的变更集变成两个）；会话结束不重开（PG10）；组合中照存（组合输入）；阅读者有"编辑"（PG12）。

## 完成检查

`make check`、`make e2e` 为绿；清单交给负责人。
