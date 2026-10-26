# 从 Claude Code 交接给 Codex

| 项 | 内容 |
|---|---|
| 日期 | 2026-10-26 |
| 交接人 | Claude Code（Opus 5.5），M0 起在负责人的笔记本上推进本项目 |
| 接手人 | Codex，在一台远程开发机上 |
| 交接时的 `main` | 本目录的提交（它之前是 `f63c84b9`，M10/P1 的 S2 已合并、CI 绿） |
| 负责人的安排 | 之后整个项目由 Codex 推进；**只在一个 M 做完之后**，负责人（与 Claude）审核。一个 M 之内的 Phase、Step 由 Codex 自己推进，不必停下来问 |

## 先读什么（按次序）

1. 本目录：
   - [01-progress.md](01-progress.md)：现在做到哪里、下一步是什么（M10/P1 的 S3）、还欠负责人的事；
   - [02-workflow.md](02-workflow.md)：推进方式，从一个 M 到一个 Step 的每一步，审查、变异、文档、提交、合并与 CI；
   - [03-project-background.md](03-project-background.md)：项目是什么、架构、仓库与文档的地图、门禁与 CI；
   - [04-owner-decisions-and-preferences.md](04-owner-decisions-and-preferences.md)：负责人的决定与偏好（从 Claude 的本机记忆提炼）；
   - [05-lessons.md](05-lessons.md)：M0–M10 的审查一再发现的问题与对策；
   - [06-environment.md](06-environment.md)：远程开发机上的工具、命令与规矩；
   - [tools/](tools/)：Claude 用过的四个小工具（变异、串行化重的运行、等 CI、链接检查），可以照用或换成自己的。
2. 仓库自己的文档（它们是权威，本目录只是导读）：
   - [docs/README.md](../../README.md)：文档约定、完成标准、handoff 规则、Git 的写法；
   - [v0.1-design.md](../v0.1-design.md)：版本总体设计，第 12 节的里程碑与扩展点、第 13 节的长期约定（每一条都约束实现）；
   - [M10 总设计](../M10-llm-wiki/00-M10-design.md)：当前的 M，第 11 节是负责人与主持者的决定；
   - [M10/P1 设计](../M10-llm-wiki/01-P1-contract-frame.md)：当前的 Phase，第 6.3、6.4 节是 S3、S4 的提交次序。

## 一句话的现状

M0–M9 的实现都做完了（M4–M9 等负责人在 v0.1 最后统一做人工验收）；M10"模式契约与 LLM Wiki"的总设计与 P1 的设计已审查定稿，P1 的 S1（模式与模板）、S2（守卫的骨架）已合并；**下一步是 M10/P1 的 S3**（预检、后台的拒绝的去处、保留标题加序号），然后 S4（网页）、P2–P5 与 M10 的收尾。

## 接手之后的界线

- 一直推进到 **M10 收尾完成**（收尾审查、修复、文档、`main` 的 CI 绿），然后**停下**，写一份给负责人的中文汇报（见 [02-workflow.md](02-workflow.md) 最后一节），等审核。**不开始 M11**，除非负责人说开始。
- 一个 M 之内只在这几种情形停下来问负责人：设计与代码有重大的架构冲突、要做破坏性或不可逆的操作（删数据、改历史、强推 `main`）、外部服务长时间不可用。产品上的小决定照设计里"推荐"的那一种做，记作"主持者的决定（负责人可以改判）"，在 M 的汇报里列出来。
- 体验上的小问题不在 M 里修，留给 v0.1 之后的一次统一打磨（[04](04-owner-decisions-and-preferences.md)）。
