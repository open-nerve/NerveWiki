```yaml
status: done
from: M5
to: M9
created: 2026-10-03
```

# 编辑锁与 MCP 的写

M5 的编辑锁（[M5 总设计](../../M5-collab-editing/00-M5-design.md)第 4.1–4.4 节）：

- 正文的写按会话：一页有活着的编辑会话时，不带这个会话的正文写都答 409 `page.locked`，包括持锁人自己的令牌与 MCP 代理；problem 的 `lock` 成员带被锁的页与持锁人。
- 删除按账户：别的账户持锁时删除被拒，本人的会话随删除结束。
- 接管：同一账户的任何客户端开会话时带 `take_over` 就能接管，前一处切为只读（负责人的决定 1）。

M9 要定：

1. **MCP 的写碰上被锁的页**：工具把 `page.locked` 连同持锁人告诉代理（"某某正在编辑，稍后再试"），不重试到底。
2. **batch**：一个 batch 是一个写入单元，其中一页被锁，整个 batch 回滚。工具的说明写明这一点，并在错误里说出是哪一页。
3. **接管**：MCP 的工具是否给出 `take_over`。给出就意味着代理能把本人浏览器里的编辑器切成只读；建议不给，或只在用户明确要求时用。由 M9 的设计决定，必要时问负责人。
4. **测试**：MCP 的写在页面被锁时的行为测试。
5. **服务端没有闲置上限**（[M5 收尾审查](../../M5-collab-editing/reviews/M5-closeout-review.md) A-Q2）：30 分钟没有输入就退出只是网页编辑器的常量（`editor/idle-exit.ts`），服务端不判断闲置。一个一直心跳的令牌或 MCP 代理能一直持锁，只有笔记本的管理员能强制解锁。M9 若让代理开编辑会话，在设计里定：代理的会话要不要有服务端的上限（例如很久没有写就不再续期），还是只在工具的说明里要求代理写完就结束会话。

## 处理进展

- M9/P1–P3、P5（2026-10-18 至 10-23）：第 1 项：MCP 的写碰上被编辑的页答 `page.locked`，文字说出持锁人（本人的说"by you, in a browser"）、稍后再试、不能接管，不说自动重试；`lock` 带页、路径、持锁人与 `you`，删除另带子树里的 `locks`（至多 20 项）与 `locks_count`（`page/adapter/mcp/locks.go`；[M9 总设计](../00-M9-design.md) 4.7、4.10）。第 2 项：batch 是一个单元，锁的预检（`page/app/apply_batch.go` 的 `locked`）在第一个写或删除被编辑的页的项答 `page.locked`，带 `op` 与持锁人，什么都不写；`TestTheMCPBatchIsRefusedWhole`、MC5 第三步（4.11）。与分开调用的一处不同：batch 里一个与页的字节相同的 `write` 也是正文操作，碰上被编辑的页整个被拒，分开的 `write` 答 `changed: false`（M9 收尾 C-M13，写进 4.11 的那一条规则）。第 3 项：不给接管（M9 总设计第 11 节负责人的决定第 2 条）。第 4 项：`TestTheEditLockReachesEveryMCPWrite`、`interleavings_mcp_test.go`、C4 (MCP)。第 5 项：MCP 不开编辑会话，不涉及。
- M9 收尾（2026-10-23）：核对过各项的落实（[M9 收尾审查](../reviews/M9-closeout-review.md)"handoff"，C 逐项对照代码），关闭。
