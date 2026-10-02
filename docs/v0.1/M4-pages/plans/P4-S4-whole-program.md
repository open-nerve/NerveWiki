# M4/P4/S4 整个程序与交错：实施计划

上级：[P4 文档](../04-P4-content-sessions.md) 3.8、3.10、3.11。

## 任务

1. 活动：模块根的 `NewNotebookActivity(pool)`；`registrants.go` 转换值、`notebookRegistrants(pool)` 交出它；`page_registrants_test.go` 经 `listOwnerlessNotebooks` 断言大小与最后活动（会话的第二次保存推后了它）。
2. `page_registrants_test.go` 的三条删除路径：页面各开一个会话，删除之后会话没了。
3. `page_content_test.go`：字节保真的往返（CRLF、只用 `\r`、混合换行、BOM、行尾空白、制表符、NBSP、零宽字符、U+2028、NFD）逐字节相同、`content_hash` 与 `byte_size` 对；孤立代理项与非法 UTF-8 答 400；请求体上限（P4 文档 3.8）。
4. `interleavings_page_test.go`：`checkPages` 加会话的不变量；交错 38–43（P4 文档 3.11），持正文行的 `contentRow`、持会话行的 `sessionRow`。

## 测试

上面每一项本身就是测试。反向对照：组合根不交活动来源（活动的测试失败）；任一条删除路径不删会话；`LockContent` 不加锁（交错 38、42 不再等正文行）；清理不带 `SKIP LOCKED`（交错 43 等锁）；`BodyLimits` 不交给 `apiConfig`（5 MiB 的正文 413）。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿。
