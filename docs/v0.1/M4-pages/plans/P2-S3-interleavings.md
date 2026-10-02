# M4/P2/S3 整个程序与交错：实施计划

上级：[P2 文档](../02-P2-tree-operations.md) 3.7、3.8。

## 任务

1. `page_visibility_test.go`：这一份数据另经接口删除 team 的页（带一个子页），对每一列树与逐项读取仍一致。
2. `interleavings_page_test.go`：交错 34–37（P2 文档 3.8），持笔记本行的 `FOR SHARE`（`sharedNotebookRow`），两种先后各一个用例，结束时 `checkPages` 与 `checkNotebooks`。

## 测试

上面每一项本身就是测试。反向对照：树锁改成 `FOR SHARE`（交错 34–37 不再等笔记本行，在等锁的期限失败）；层级的检查只看被移动的页（交错 35 的"新建在先"一种失败）。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿。
