# M5/P1/S4 交错、改写的 M4 测试与最后一跳：实施计划

上级：[P1 文档](../01-P1-edit-lock.md) 3.8、第 5 节；[M5 总设计](../00-M5-design.md)第 3 节。

## 任务

1. 交错 45–51，改写 42（P1 文档 3.8）；`checkPages` 的两条新不变量。
2. 改写 M4 的测试：`page_content_test.go` 的 `TestASessionsSavesAroundAnotherWrite`（会话活着时别人的写现在被拒；"夹进别人的写另起变更集"移到 page/app 的单元测试，不注册锁）与两人在同一页开会话、再删子树的用例（改为本人的与过期的会话随删除删掉，别人活着的会话挡住删除）。
3. 最后一跳（`bootstrap/page_lock_test.go`）：否决者经开启（带与不带接管），守卫经新建（带与不带正文）、改名、移动、删除、写正文（带与不带会话），各在整个程序上；`pageRegistrants(pool)` 交空时失败。相同的保存与开启会话不经守卫，也钉住。
4. problem 的成员经 HTTP：`page.locked` 带 `lock`、`page.edit_session_unlocked` 带 `ended_by`，答复符合契约。

## 测试

上面每一项本身就是测试；另跑一遍 P1 文档第 5 节的反向对照。

## 完成检查

`GOFLAGS=-p=3 make check` 为绿；交错测试另跑 `-race -count=10`。
