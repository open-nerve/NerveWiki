```yaml
status: done
from: M5 收尾
to: M10
created: 2026-10-04
```

# 实时推送的事件类型：笔记本模式的变化

笔记本模式的变化要实时推送时，它是一个新的事件类型（总体设计 12.4；[M5 总设计](../../M5-collab-editing/00-M5-design.md)第 8 节）。服务端怎么发布、前端的处理表在哪里加一项、最后一跳的测试怎么写，以及工作区级的事件到不了一部分人的问题，都写在给 M6 的[事件类型移交](../../M6-links/handoffs/M5-events.md)里，照那份办。到那时那份若已关闭，以它的处理情况与总体设计第 13 节为准。

## 处理进展

- M10 开工（2026-10-24）：照[M10 总设计](../00-M10-design.md)第 7 节"移交的落实"的第一行做：类型 `mode`、它的发布者与契约的 `EventMode` 在 P1 的 S1，前端的处理表在 P1 的 S4（4.5）。仍是 `open`，做完的 Phase 改为 `done`，收尾核对。
- P1/S1（2026-10-25，合并见 [00 号文档](../00-M10-design.md)第 12 节）：类型 `mode`、发布者 `notebookEvents.ModeChanged` 与契约的 `EventMode`（最后一跳 `TestTheModeEventReachesTheStreams`，帧按契约的 `EventMode` 核对）。前端的处理表在 P1 的 S4；仍是 `open`。
- P1/S3（2026-10-26，合并见 [00 号文档](../00-M10-design.md)第 12 节）：本 Step 的预检、后台的拒绝与加序号没有新增模式事件的处理；S1 已有的服务端发布不变，前端的处理表仍在 P1/S4；仍是 `open`。
- P1/S4（2026-10-27，合并见 [00 号文档](../00-M10-design.md)第 12 节）：前端的 `modeChanged` 已登记在实际事件处理表，`EventMode` 的导出有真实消费者；扩展替身类型换成 `later`。`app/event-stream.test.tsx` 的真实表最后一跳及第 54 条登记移除变异覆盖列表重读，LW12 页面覆盖独立流上的更新。给 M10 的服务端与前端两项都有实现；三份原始完整 Step 独立审查及修复后的范围核对已由主持者接受；B-I1（Important，测试覆盖）已关闭，没有剩余核实的 Important、Minor、Nit 或 Q。原始审查头为a0，最终接受头为13ee；B由原审查者重放核对，A/C由主持者按工作流3.4的测试修复例外核对未变范围，记录分别保存。 本分支文档仍是 `open`，在 `docs: M10/P1 done` 时改为 `done`。
- P1 完成（2026-10-27）：S1 的 `mode`、`notebookEvents.ModeChanged`、`EventMode` 与 S4 的真实 `modeChanged` 处理表、编造的 `later` 扩展测试均已落地。服务端 `TestTheModeEventReachesTheStreams` 与前端 `app/event-stream.test.tsx` 的真实登记最后一跳、对应变异及 LW12 已核对，四个 Step 合并和合并后 CI 见 [P1](../01-P1-contract-frame.md) 10.5 与 [S4 审查](../reviews/P1-S4-web-review.md)。给 M10 的两项均完成，改为 `done`。
