# M4/P1/S4 矩阵、整个程序与交错：实施计划

上级：[P1 文档](../01-P1-page-module-pipeline.md) 3.9、3.10、3.12、3.13。

## 任务

1. 矩阵：`permission_matrix_seeded_test.go` 种下页面（priv 根页与子页、team、wiki、orphan 各一页、gone-nb 的页面随笔记本软删除；都带正文行），`seeded.pages` 与 `page(name)`，`workspaceOfRow` 认得页面；`permission_matrix_page_test.go` 的四行与 `editorsOnly`。
2. `page_visibility_test.go`：树与逐项读取一致（P1 文档 3.12）。
3. `page_registrants_test.go`：删除笔记本、删除工作区、删除无主笔记本各自连带页面（节点与跟随的行的 `deleted_at` 等于笔记本的），之后读答 404。
4. `purge_test.go`：`TestThePurgeDeletesWhatOutlivedTheRetention` 种三层页面与跟随的行、`live` 里的超期子树；等任务完成的查询加 `errors IS NULL`。
5. `interleavings_page_test.go`：`checkPages`；交错 30–33（P1 文档 3.13），两种先后各一个用例，结束时调用 `checkPages` 与 `checkNotebooks`。

## 测试

上面每一项本身就是测试。反向对照：

- 组合根的 `deps.go` 把笔记本删除的订阅者交空：删除笔记本与删除无主笔记本的两个测试失败；`workspaceRegistrantsWith` 交空：删除工作区的测试失败。
- `nodes` 的清理只删当前的叶子：整个清理任务的测试失败（任务带错误，或页面与笔记本还在）。
- `workspaceOfRow` 不认页面：矩阵的覆盖检查失败。
- `createPage` 的判定挪到加锁之前：交错 33 的"移出在先"一种失败。
- `checkPages` 的"父页未删"一条写错（例如 `IS NOT NULL`）：交错的测试失败。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿。
