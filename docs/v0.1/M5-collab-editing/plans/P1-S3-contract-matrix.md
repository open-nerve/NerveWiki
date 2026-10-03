# M5/P1/S3 契约、HTTP、矩阵与文案：实施计划

上级：[P1 文档](../01-P1-edit-lock.md) 3.6、3.7。

## 任务

1. `api/modules/page.yaml`：`openEditSession` 的请求体与码；心跳、结束、写正文、删除的码与描述；`getEditLock`、`releaseEditLock` 与 `EditLock`、`EditLockHolder`；`make gen`。
2. HTTP（`adapter/http`）：两个新处理器；`openEditSession` 读请求体（可以没有）；每个新码在本模块的 `adapter/http` 测试里经 `apitest.CheckResponse` 答出一次。
3. 规则表与权限矩阵：`getEditLock`、`releaseEditLock` 两行；种子改为每页至多一个活着的会话；`leased()` 改为 120 秒。
4. 前端的文案：`page.locked`、`page.edit_session_taken_over`、`page.edit_session_unlocked` 的中英文（不带名称，名称的插值在 P4）；`app/problem-messages.test.ts` 的"没有文案的码"换一个示例；`make gen-web`。
5. README 的编辑会话一节：租约 120 秒、锁、接管、强制解锁、读锁、新码。

## 测试

- `apitest` 的规则测试、契约测试、矩阵的覆盖检查都为绿。
- 前端的文案测试：契约里每个码都有文案。

## 完成检查

`GOFLAGS=-p=3 make check`、`make gen-check`、前端的 lint、format、types 与 `make knip` 为绿。
