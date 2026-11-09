```yaml
status: open
from: M5 收尾
to: M11
created: 2026-10-04
```

# 实时推送的事件类型：笔记本模式的变化

笔记本模式的变化要实时推送时，它是一个新的事件类型（总体设计 12.4；[M5 总设计](../../M5-collab-editing/00-M5-design.md)第 8 节）。服务端怎么发布、前端的处理表在哪里加一项、最后一跳的测试怎么写，以及工作区级的事件到不了一部分人的问题，都写在给 M6 的[事件类型移交](../../M6-links/handoffs/M5-events.md)里，照那份办。到那时那份若已关闭，以它的处理情况与总体设计第 13 节为准。

## 处理进展

- M11 开工（2026-11-08）：照[M11 总设计](../00-M11-design.md)第 7 节"移交的落实"：M10 已建类型 `mode`；M11 的每个升级状态的变化都经模式变化的订阅链发它，`EventMode` 不加字段、只改说明（00-M11 4.12）。说明在 P1/S1，最后一跳 `TestUpgradeStatesReachTheStreams` 在 P1/S2。仍是 `open`，做完的 Step 各加一行，M11 收尾时改为 `done`。
- P1/S1（2026-11-09，合并 `27b30c7f`）：`events.yaml` 的 `mode` 与 `EventMode` 的说明写上升级的转移（发起、提交、退回、批准、驳回、放弃、回滚的停下与结束），`EventMode` 不加字段；S1 还没有发它的转移，最后一跳 `TestUpgradeStatesReachTheStreams` 在 P1/S2。仍是 `open`。
- P1/S2（2026-11-09，合并 `9c7c6821`）：发起经模式变化的订阅链发 `mode`，最后一跳 `TestUpgradeStatesReachTheStreams` 建成（发起之后流里先是补写的页的 `pages`、再是 `mode`；订阅者交空时失败）；放弃、回滚的停下与结束在 P1/S3 加进同一个测试。仍是 `open`。
- P1/S3（2026-11-10，合并 `8af1bff6`）：放弃、回滚的停下、继续、做完与结束回滚加进 `TestUpgradeStatesReachTheStreams`（八步：放弃、停下、继续、做完、第二次发起、放弃、停下、结束回滚，每步该收到的各一帧 `mode`、不该收到的没有；去掉放弃、停下或结束回滚的 `Publish` 都被它抓到）；回滚的进度没有事件，升级页轮询任务在 P1/S4。仍是 `open`。
