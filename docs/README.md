# Nerve Wiki 文档约定

本目录存放 Nerve Wiki 的全部设计与过程文档。任何人（或 Agent）开始工作前，先读本文件，再读当前版本的总体设计 [v0.1/v0.1-design.md](v0.1/v0.1-design.md)。

## 目录结构

```
docs/
  README.md                          本文件：文档约定
  v0.1/                              版本目录：v0.1 = 首个可用版本（M0–M12）
    v0.1-design.md                   版本总体设计：范围、架构、决策、里程碑、扩展点、长期约定
    M<n>-<slug>/                     里程碑目录，例如 M0-foundation
      M<n>-design.md                 里程碑设计文档（开始该 M 时创建）
      specs/                         各 Phase 的设计说明
      plans/                         各 Phase 的实施计划
      reviews/                       各 Phase 的评审记录
      handoffs/                      计划外发现的交接事项（见下文"handoff 规则"）
```

## 层级与节奏

- **版本（v0.1、v0.2…）**：一个版本 = 一组里程碑。版本总体设计定义范围、架构、路线，以及跨里程碑的扩展点和长期约定。
- **里程碑（M0、M1…）**：一个 M 一个 M 串行推进。开始某个 M 时，先在它的目录里写 `M<n>-design.md`：用户故事清单、Phase 划分、Phase 进度表，以及本 M 要建立或注册的扩展点（与总体设计的扩展点表一致）。
- **Phase（P1、P2…）**：M 内部再切分成多个 Phase，串行推进。每个 Phase 依次产出 spec → plan → 实现 → review。

## 命名规则

- 目录名、文件名用英文 kebab-case；正文用中文，技术名词保留英文。
- 同一个 Phase 在四个子目录中使用**同一个前缀和 slug**，便于对照：
  - `specs/P2-page-tree.md`
  - `plans/P2-page-tree.md`
  - `reviews/P2-page-tree-review.md`（第二轮评审加 `-r2`，依此类推）
  - `handoffs/P2-page-tree-<主题>.md`
- Phase 编号只在所属 M 内部有效，跨文档引用写成 `M4/P2`。

## 进度追踪（唯一来源）

- `v0.1-design.md` 中的"里程碑进度表"记录各 M 的状态。
- 每个 `M<n>-design.md` 中的"Phase 进度表"记录本 M 各 Phase 的状态，并链接到对应的 spec / plan / review。
- 状态取值：`未开始` / `进行中` / `已完成`。

## 计划内的跨 M 工作不写 handoff

跨里程碑的交互分三类，各有固定的去处：

| 类型 | 去处 |
|---|---|
| 后面的 M 要在前面的 M 上挂接能力（例如 M5 给 M4 的编辑会话加锁） | 总体设计的**扩展点表**（v0.1-design 第 12.4 节）。建立扩展点的 M 负责把它做出来；注册的 M 只注册，不改写前面 M 的用例 |
| 所有后续 M 都要遵守的横切约定（例如 stores 按会话分代、写入必须经过写入管线） | 总体设计的**长期约定**（v0.1-design 第 13 节）。在第一个用到它的 M 里一次做完整，并写进该节 |
| 做的过程中才发现、事先没有计划的事项 | **handoff**，按下面的规则处理 |

## handoff 规则

handoff 只用于**计划外的发现**。每个 handoff 文档开头写明：

```yaml
status: open        # open | done
from: M4/P3         # 在哪里产生
to: M6              # 应由哪个阶段处理
created: 2026-10-01
```

- 移交给其他 M 的事项，直接放进目标 M 的 `handoffs/` 目录，文件名以来源开头（例如 `M6-links/handoffs/M4-P3-title-normalization.md`），并在来源 Phase 的 review 中链接过去。
- 产生一个跨 M 的 handoff 时，先判断它是不是扩展点表或长期约定漏掉的内容：如果是，同时修订总体设计的对应章节，handoff 只记录修订的原因。
- 开始一个 M 之前，先处理它 `handoffs/` 中所有 `open` 的事项。
- 一个 M 完成前，不能留有 `open` 的 handoff。每一项要么处理完（改为 `done`），要么明确移交给后续 M（目标 M 在其设计中接收）。

## 其他

- 空目录放 `.gitkeep`，保证目录结构能提交进 git。
- 参考代码（例如 Nerve 项目）只读取、不修改，也不放进本仓库。
