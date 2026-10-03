# M5/P4/S3 端到端：实施计划

上级：[P4 文档](../04-P4-edit-lock-web.md) 3.12；[M5 总设计](../00-M5-design.md)第 3 节的 C1–C4、C6，第 9 节。

## 任务

1. 夹具：`wiki-pages.ts` 的 `editRefused`、`lostBanner`；`pages.ts` 的开启带 `takeOver`。
2. `e2e/stories/collab/c1`–`c4`、`c6` 加页面版本（C6 的关闭核实 `keepalive` 的请求到达）。
3. 跑全部故事，处理先拿锁带来的变化：PG8、PG10 的页面版本；请求清单里离开时的结束；`expectConsole` 声明被拒的开启与失锁的 409。

## 测试

每条故事核对页面上看得到的结果与落库的会话；反向对照（e2e）：开会话失败照样编辑（C1）；`taken_over` 照旧重开（C2）；锁事件不心跳（C2、C3 超时）；`pagehide` 不释放（C6）。

## 完成检查

`make e2e` 为绿。
