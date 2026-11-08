```yaml
status: open
from: M9 收尾
to: M11
created: 2026-10-23
```

# 升级的 MCP 工具、冻结的守卫与回滚的调度

M9 建了 MCP 的工具与 prompt 的注册表（[M9 总设计](../../M9-mcp/00-M9-design.md)第 8 节）与 batch 的整组撤销（多轮的调度与临时名称，4.12）。M9 总设计"写给后面的 M"给 M11 的几条原该由发现它的 Phase 写进这里，M9 收尾时才补上（[M9 收尾审查](../../M9-mcp/reviews/M9-closeout-review.md) C-I1 = A-F4；[M9/P3](../../M9-mcp/03-P3-batch-revert.md) 第 8 节）。M8 写的[回滚一次升级](M8-P4-background.md)照旧，这里是 M9 加的。留给 M11 的：

1. **`upgrade_verify`、`upgrade_submit` 两个工具**（总体设计 5.8、12.4）：登记在 MCP 的注册表上（模块根的 `Tools()`、`mcpserver.Tool`，由组合根的 `mcpTools` 拼接），照 M9 的每个工具：先写进 `api/mcp/tools.json`（`TestTheToolListsAreTheContract` 比较两个入口的整个工具对象），在权限矩阵里有行（`TestThePermissionMatrixCoversEveryTool`），声明的码进 `mcpToolCodes()` 的表，答复经 `Fit` 至多 48 KiB，注解照它们做的，内置的说明有它们的一行（`TestTheBuiltInInstructionsNameEveryTool`；M10/P3/S1 产品输入 `18b922cfdec5471de622f1bcce9366c497a519e6` 的两轮原生 race 中，`TestTheBuiltInInstructionsAreShort` 实测内置说明：笔记本入口 raw/SDK 为 3044/3093 字节，账户入口为 3113/3163 字节；按实际 SDK 文本编码计，距 4 KiB 分别余 1003/933 字节，均留有至少 300 字节。实际输入和最终本机门禁见 [P3/S1 审查记录](../../M10-llm-wiki/reviews/P3-S1-lint-protocol-review.md)的 F01，数值与范围见 [M10 移交的处理进展](../../M10-llm-wiki/handoffs/M9-P1-mcp-registry.md#处理进展)。M10/P5 没有改内置的两份，这些数仍成立：LLM Wiki 的系统段在拼接时加在说明之前，不算进内置的 4 KiB；32 KiB 的拼接与 512 字节的系统段是两个不同的界。继续为 M11 的两个工具在每个入口至少预留 300 字节。upgrade 的工具走原来的 `Tools`、合同、权限与 `Fit`，将来的 upgrade prompt 走 P5 已有的按入口注册；M11 尚未开始）。
2. **冻结的守卫放行的写**（总体设计 5.8 第 2 步）：放行发起升级的管理员经 MCP 的写：`Write.Client` 以 `mcp:` 开头（`mcp:<客户端名>`，名称取 `clientInfo.name`，清理之后至多 128 个字符，M9 总设计 4.5），执行者是发起人；放行回滚的撤销（`Write.Kind` 是 `revert`，同 M8 那一份的第 4 条）；挡其余的。撤销的步里有临时名称的改名与移到根下（`<名称> ~<id 的后 8 位>`，以普通的 `OpRename`、`OpMove` 到达，只凭名称认不准，[M10 的移交](../../M10-llm-wiki/handoffs/M9-P1-mcp-registry.md)第 7 条）：冻结的守卫按 `Kind` 放行它们，不看名称。一个 `batch` 是一个单元，守卫看到它的每一步（同一页的正文合成一步，同上第 7 条）：放行时整个 batch 一起放行，挡时整个回滚。
3. **回滚经多轮的调度**（M9 总设计 4.12）：回滚一组变更集时照撤销的 `PlanRevert`，它的 `schedule()`（`page/domain/revert_schedule.go`，M9/P3 的 S2 代替 M8 的单轮 `round()`）按依赖排步、卡住时取三条后备之一，对任何变更集都成立；轮数的上限（开始时待做的步数加 2N + 1，N 是变更集碰过的节点数）让缺陷答 `ErrRevertUnordered`（500）而不在笔记本的锁下打转。回滚不另写自己的循环，也不在 `ErrRevertUnordered` 上重试：它是缺陷，不是冲突。后台的撤销（M9/P3 的 S3 起）按任务开始时的计划切批，只在不可分的段之间切，一段放不进一批时以 `revert_too_large` 停下，已撤销的部分留着；停下时照 M8 那一份的第 2 条。
4. **MCP 开的后台任务**：升级若要由 MCP 开后台任务，先放开 `transfer_jobs.client`（M9 没有放开：agent 的历史只读、MCP 不开任务，M9 总设计第 11 节第 1 条；M8 那一份的第 5 条）。

## 处理进展

- M11 开工（2026-11-08）：照[M11 总设计](../00-M11-design.md)第 7 节"移交的落实"：第 1 条（`upgrade_verify`、`upgrade_submit`，两个入口）在 P2/S2；第 2 条（冻结放行发起人的 MCP 写与回滚的写，后者按 `UpgradeOwn` 而不按 `Kind`，第 175、177 条；`batch` 整个放行或整个回滚）在 P1/S1 与 P2/S1；第 3 条（回滚照用 `PlanRevert` 与 `schedule`，`ErrRevertUnordered` 是缺陷）在 P1/S3；第 4 条不适用（MCP 不放弃、不开任务）。仍是 `open`，做完的 Step 各加一行，M11 收尾时改为 `done`。
- P1/S1（2026-11-09，合并 `27b30c7f`）：第 2 条的 P1 一半：冻结放行发起人在 `upgrading` 中经 MCP 的写（`mcp:` 的客户端、执行者是发起人、每个单元经 `Grants` 问一次 `notebook.change_mode`，发起人被降为写者就拒），别的 MCP 写、发起人的 REST 写与 `reviewing`、`rolling_back` 中的一切写都答 `notebook.upgrading`；升级自己的单元按 `UpgradeOwn` 放行，不看 `Kind` 与名称（回滚的单元在 P1/S3 设它）；`batch` 是一个单元，被拒时整个回滚、失败项是那个操作（`TestUpgradeFreezeReachesEveryPath`、`TestUpgradeFreezeFollowsTheInitiatorsRole`、`TestUpgradeToolsErrorsCarryTheirCodes`）；放行的写里删除的护栏拒绝删基线节点与补写的来源根（`page.upgrade_protected`），其余护栏在 P2/S1。仍是 `open`。
