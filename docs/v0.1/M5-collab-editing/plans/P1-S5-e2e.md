# M5/P1/S5 端到端：实施计划

上级：[P1 文档](../01-P1-edit-lock.md) 3.9。

## 任务

1. 夹具 `e2e/fixtures/collab.ts`：带 `take_over` 的开启、读锁、强制解锁；`e2e/fixtures/assert/collab.ts`：这一页活着的会话、墓碑的三列。
2. 故事 `e2e/stories/collab/`：C1–C4 与 C6（过期）的接口版本。
3. 改写 PG4、PG8 的两个版本（P1 文档 3.9）。

## 测试

故事本身；e2e 的反向对照：守卫不拒删除、接管不限本人、解锁不留墓碑、开启不删过期的行，各让对应的故事失败。

## 完成检查

`make e2e` 全部通过。
