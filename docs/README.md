# Nerve Wiki 文档约定

本目录存放 Nerve Wiki 的全部设计与过程文档。任何人（或 Agent）开始工作前，先读本文件，再读当前版本的总体设计 [v0.1/v0.1-design.md](v0.1/v0.1-design.md)，再读当前里程碑的 00 号总设计。

## 目录结构

```
docs/
  README.md                          本文件：文档约定
  v0.1/                              版本目录：v0.1 = 首个生产可用版本（M0–M12）
    v0.1-design.md                   版本总体设计：范围、架构、决策、里程碑、扩展点、长期约定、变更记录
    M<n>-<slug>/                     里程碑目录，例如 M0-foundation
      00-M<n>-design.md              M 级总设计：目标、范围、完成标准、Phase 划分与进度表
      01-P1-<slug>.md                Phase 文档：架构设计 & 实施规划（开始该 Phase 时才写）
      02-P2-<slug>.md
      …
      specs/                         Step 级设计：大 Phase 拆成 Step 时使用
      plans/                         Step 级实施计划
      reviews/                       每个 Phase 的代码审查与修复记录（必需）；M 收尾审查
      manual/                        人工验收清单：自动测试做不到、由负责人执行的检查（例如真实输入法）
      handoffs/                      阻塞点、决定延后处理的事项（TODO）
```

## JIT 滚动推进

不一次性写出所有 M 或所有 Phase 的设计。高层级文档约束范围和方向，下一层级在开工时才设计，并且以当时最新的代码为基础：

1. **版本总体设计**（`v0.1-design.md`）定下范围、架构、M 的编排、扩展点和长期约定。
2. **开始一个 M**：先处理它 `handoffs/` 里所有 `open` 的事项；对照最新代码复核这个 M 的范围；写 `00-M<n>-design.md`，列出 Phase 划分。Phase 在 00 号文档里只到"目标、交付、验证"的粒度。
3. **开始一个 Phase**：基于上一个 Phase 完成后的最新代码，写 `NN-Pn-<slug>.md`（架构设计 & 实施规划）。Phase 较大时拆成 Step，Step 的设计和计划放进 `specs/`、`plans/`；边界和目标已经很清楚的 Step 可以跳过 spec，直接写 plan。
4. **实现 → 测试 → 审查 → 修复**，然后更新进度表，进入下一个 Phase。
5. **向上修订**：某个 Phase 的实际情况改变了后续 Phase 的规划，就修订 00 号文档；改变了 M 的边界、跨 M 的契约或扩展点，就修订 `v0.1-design.md`，并在它的"变更记录"里写明原因。

## Phase 文档的骨架

`NN-Pn-<slug>.md` 固定包含以下几节：

1. **基线**：本 Phase 基于哪个提交开始；上一个 Phase 留下了什么。
2. **目标与范围**：做什么，明确不做什么。
3. **设计**：模块与文件、端口与依赖方向、数据结构、接口、关键取舍。
4. **实施步骤**：按顺序列出，每一步可独立验证；大 Phase 在这里列出 Step 并链接到 `specs/`、`plans/`。
5. **测试与验证**：单元、集成、契约、E2E 各覆盖什么；怎样证明做完了。
6. **完成标准**。
7. **结果**（完成后补写）：实际与设计的差异、审查结论链接、遗留的 handoff。

## 完成标准

**Phase 完成**：

- 代码与本 Phase 范围内的测试（单元、集成、契约、E2E）全部通过；所有门禁（Go 与前端的 lint、类型检查、knip、生成物一致性、架构测试）为绿。
- 完成代码审查，记录在 `reviews/Pn-<slug>-review.md`；审查发现的问题已经修复，或者以 handoff 明确延后并写明理由。
- 00 号文档的进度表、Phase 文档的"结果"一节已更新。

**M 完成**：所有 Phase 完成；做一次 M 收尾审查（`reviews/M<n>-closeout-review.md`，检查跨 Phase 的一致性、扩展点、文档与代码是否一致）；满足 `v0.1-design.md` 第 12.5 节的通用完成标准。

## 命名规则

- 目录名、文件名用英文 kebab-case；正文用中文，技术名词保留英文。
- Phase 文档：`NN-Pn-<slug>.md`，`NN` 是两位序号（`01`、`02`…），与 Phase 编号一致。
- Step 文档：`specs/Pn-Sm-<slug>.md`、`plans/Pn-Sm-<slug>.md`。
- 审查：`reviews/Pn-<slug>-review.md`，第二轮加 `-r2`；M 收尾审查 `reviews/M<n>-closeout-review.md`。
- handoff：`handoffs/<来源>-<主题>.md`，来源写成 `M4-P3` 这样的形式。
- Phase 与 Step 编号只在所属 M 内部有效，跨文档引用写成 `M4/P2`、`M4/P2/S1`。

## 进度追踪（唯一来源）

- `v0.1-design.md` 的"里程碑进度表"记录各 M 的状态。
- 每个 `00-M<n>-design.md` 的"Phase 进度表"记录本 M 各 Phase 的状态，并链接到 Phase 文档与审查记录。
- 状态取值：`未开始` / `进行中` / `已完成`。

## 跨 M 的事项放在哪里

| 类型 | 去处 |
|---|---|
| 后面的 M 要在前面的 M 上挂接能力（例如 M5 给 M4 的编辑会话加锁） | `v0.1-design.md` 的**扩展点表**（12.4）。建立扩展点的 M 负责把它做出来；注册的 M 只注册，不改写前面 M 的用例 |
| 所有后续 M 都要遵守的横切约定 | `v0.1-design.md` 的**长期约定**（第 13 节）。在第一个用到它的 M 里一次做完整，并写进该节 |
| 阻塞点、决定延后处理的事项 | **handoff**，按下面的规则处理 |

## handoff 规则

每个 handoff 文档开头写明：

```yaml
status: open        # open | done
from: M4/P3         # 在哪里产生
to: M6              # 应由哪个阶段处理
created: 2026-10-01
```

- 移交给其他 M 的事项，直接放进目标 M 的 `handoffs/` 目录，并在来源 Phase 的审查记录中链接过去。
- 产生一个跨 M 的 handoff 时，先判断它是不是扩展点表或长期约定漏掉的内容：如果是，同时修订 `v0.1-design.md` 的对应章节。
- 开始一个 M 之前，先处理它 `handoffs/` 中所有 `open` 的事项。
- 一个 M 完成前，不能留有 `open` 的 handoff：每一项要么处理完（改为 `done`），要么明确移交给后续 M。

## Git 工作方式

- 文档（版本设计、M 总设计、Phase 文档）直接提交到 `main`。
- 代码按 Phase 在分支上开发（例如 `m0-p3-server-platform`），审查、修复、门禁全绿之后以 `--no-ff` 合并回 `main` 并推送。
- 提交信息：
  - 代码提交写成 `<范围>: <说明> (M<n>/P<n>/S<m>)`，例如 `server: add the httpserver platform package (M0/P3/S3)`。范围是改动所在的部件（`server`、`web`、`api`、`e2e`、`deploy`、`build` 等），多个用逗号分隔；审查修复只写到 Phase，例如 `(M0/P3)`。
  - 文档提交写成 `docs(M<n>/P<n>): <说明>`，M 级的文档写成 `docs(M<n>): <说明>`。
  - 合并提交写成 `Merge branch '<分支>': M<n>/P<n> <名称>`。

## 其他

- 空目录放 `.gitkeep`，保证目录结构能提交进 git。
- 参考代码（例如 Nerve 项目）只读取、不修改，也不放进本仓库。从参考代码拷贝进来的文件即由本项目接管：逐个审查，删掉指向参考项目文档的注释和引用。
