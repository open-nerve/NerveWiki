# 项目背景

权威是 [v0.1-design.md](../v0.1-design.md)；这里是一页的地图。

## 1. 产品

Nerve Wiki：面向团队的自部署在线 Markdown 笔记（单租户），工作区 → 笔记本 → 有层级的页面与附件。灵感是 Obsidian（原样的 Markdown、链接与属性写在文本里）与 Karpathy 的 LLM Wiki（原始资料 → wiki 页 → `AGENTS` 约定；ingest / query / lint）。**产品不调用任何 LLM**；对 agent 友好靠原样的 Markdown 与内置的 MCP。

核心原则（总体设计 0.2，实现处处受它约束）：PG 是唯一真相、写入一个事务；字节保真（只有移动与改名时的链接改写例外，记在变更集里）；结构写在 Markdown 里、数据库只做派生索引；权限边界是笔记本；Obsidian 兼容是验收尺子；单写多读（编辑锁，人与 agent 同一套）；页面能做的接口都能做；一个可执行文件加一个 PostgreSQL。

## 2. 技术与仓库

| 部分 | 技术 | 位置 |
|---|---|---|
| 服务端 | Go 1.27（toolchain go1.27.1），模块化单体、端口与适配器；PostgreSQL 18，sqlc；oapi-codegen 的契约；River 的后台任务；官方 Go SDK 的 MCP 服务 | `server/` |
| 契约 | OpenAPI（每个模块一份 `api/modules/*.yaml`，根 `api/openapi.yaml`，生成 `api/dist/`）；MCP 的工具清单 `api/mcp/tools.json` | `api/` |
| 网页 | TypeScript/React、React Router（数据路由）、MobX、SWR、Radix、CodeMirror 6；Node 24、pnpm 12.8.1（corepack）；oxlint、oxfmt、knip、vitest | `web/apps/web`，生成的客户端 `web/packages/api-client` |
| e2e | Playwright，一个故事一个文件 | `e2e/stories/<分组>/`，夹具 `e2e/fixtures/` |
| 部署 | 一个镜像，内嵌前端 | `deploy/`，`make image`、`make image-smoke` |
| 工具 | Obsidian 的对照夹具等 | `tools/` |

服务端的分层（`server/internal/`）：
- `platform/`：平台层，有 `httpserver`（含 `mcpserver`）、`postgres`、`markdown`、`jobs`、`storage`、`config`、`logging`、`clock`、`ratelimit`、`webui`；
- `shared/`：内核；
- `modules/`：各模块，有 `identity`、`workspace`、`access`（规则表）、`notebook`、`page`（树、正文、版本、变更集、编辑锁、写入单元、搜索、MCP 的大部分工具）、`linking`（链接索引与改写）、`asset`、`transfer`（导入导出与后台任务）、`events`（实时推送）、`instance`、`wiki`（M10 新建：模式的契约、模板，之后的 lint、虚拟文件、playbook）；
- `bootstrap/`：组合根与整个程序的测试；
- `archtest/`：架构规则。

每个模块是 `domain` / `app` / `adapter/{http,postgres,mcp}`，只经模块根接入。模块之间只经组合根交的端口相连，迁移只改自己模块的表（13.1）。

## 3. 关键的机制（读代码之前先知道）

- **写入单元**（M4，`page/app/unit.go` 的 `Writer.Run`）：页面的每个写入入口都是一个单元。它在事务里依次做这些：
  1. 锁工作区行 `FOR SHARE`，再锁笔记本行；
  2. 在锁下判定；
  3. 每一步依次过守卫（M10 起是 `[mode, lock]`）；
  4. 写下变更集；
  5. 调参与者（链接改写）；
  6. 调观察者（链接索引、附件、事件）。

  M10 起它在锁下读笔记本的模式（`Write.Notebook`），并带着一个授权的备忘（`Write.Grants`）。`TreeWrites().Seed` 是唯一加入调用方事务的入口，给建 LLM Wiki 的模板用。
- **扩展点**（总体设计 12.4）：前面的 M 建立，后面的 M 只注册。目前有：
  - 停用、笔记本删除、可见性的订阅者；
  - 页面事件；
  - 写入单元的参与者与观察者；
  - 写入守卫；
  - 编辑会话的否决者与订阅者；
  - Markdown 扩展；
  - 编辑器扩展；
  - 实时推送的事件类型；
  - 导出贡献者；
  - MCP 的工具与 prompt 注册表；
  - M10 新加的两个：笔记本创建的播种者、模式变化的订阅者。
- **长期约定**（第 13 节）：13.1 后端（单元、客户端、规则表、码的次序 404 → 403 → 422 → 409、锁序、时间、日志、模块入口、解析、扩展点的形状与最后一跳等），13.2 前端，13.3 Markdown，13.4 测试，13.5 工程。新 Phase 的设计里每一处偏离都要写明理由。
- **码**：problem 的码（`page.locked`、`notebook.not_llm_wiki` 等）列在每个操作的 `x-problem-codes`，测试双向核对；每个码有中英两种文字（`web/apps/web/src/i18n`、`problem-messages.ts`）。
- **MCP**（M9）：`/mcp`（账户入口）与 `/mcp?notebook=<id>`（笔记本入口），只收 PAT。读写的工具按路径；`batch` 一次多项；`grep`/`search`/`index`/`backlinks`；`AGENTS` 页给说明。历史对 agent 只读。
- **M10 的模式**：`standard` 与 `llm_wiki`。LLM Wiki 有来源区（`raw`，写一次之后只读，P2）、根下的 `AGENTS`（受保护）、虚拟的 `index`/`log`（P4）、lint 与健康度面板（P3）、三份英文 playbook 作 MCP 的 prompt（P5）。

## 4. 测试与门禁

- **测试分层**：
  - 单元测试；
  - 适配器测试：真的 PostgreSQL，经 testcontainers；
  - 整个程序的测试：`server/internal/bootstrap`，经接好线的应用发 HTTP 或 MCP 请求，含权限矩阵、交错、成本守卫、计划测试；
  - 契约测试；
  - 网页的 vitest；
  - e2e。
  
  交错用持锁加等锁的确定办法（`pgtest.WaitForLockWaits` 一类）逼出来，不靠 sleep。
- **Makefile 的门禁**：
  - `make lint`（golangci-lint 2.14.0 在 `./bin`，`make tools` 安装；网页的 oxlint、oxfmt、tsc）；
  - `make gen-check`（生成物一致）；
  - `make knip`；
  - `make build`；
  - `make test`（`test-go` 加 `test-web`）；
  - `make test-go-short`；
  - `make e2e`；
  - `make image-smoke`（发镜像之前）。
  
  `make help` 列出全部。
- **CI**（`.github/workflows/ci.yml`）：
  - server 有八份：`race`、`bootstrap-1/2/3/4`、`costs`、`guards`、`short`；
  - 另有 `web`、`e2e`、`image`，合计11项。

  组合根按顶层测试名分成四份：FIRST、SECOND、THIRD减SECOND、三者并的补集，表达式以 `Makefile` 为准；子测试跟随顶层。`TestSource…`在第二份、`TestTheSource…`在第三份，其余名称依实际规则分配，不默认第二份。`make test-go` 的race段以 `-j5` 同时跑 nonbootstrap及四份bootstrap，受控本机门禁按 [06](06-environment.md) 的heavy规则逐份串行。每份看CI Test一步，两次均值须低于7:30（`-timeout 15m`的一半）；过线仅在邻居接收后仍有余量时挪界，否则增加分片，不提高timeout。M10/P2/S2的触发、实测及未决项见 [S2审查](../M10-llm-wiki/reviews/P2-S2-source-hash-review.md#最终验证与集成)；旧三份/十项记录保持历史。
- **变异**：每个 Phase 设计的变异表要实施时逐条证明会被测试抓到（[02-workflow.md](02-workflow.md) 3.1）。

## 5. 文档的地图

- `docs/README.md`：约定。
- `docs/v0.1/v0.1-design.md`：总体设计。
- `docs/v0.1/M<n>-<slug>/`，里面有：
  - `00-M<n>-design.md`；
  - `NN-Pn-<slug>.md`（Phase 设计与结果）；
  - `reviews/`（每次设计审查、Step 审查、收尾、Codex 评审的记录）；
  - `handoffs/`；
  - `manual/`（人工清单）。
- 读一段代码为什么这样写：先查它所在的 Phase 设计（3.x 的"照实"与"结果"一节），再查那个 Phase 的审查记录。决定几乎都写了理由与可以改判的人。

## 6. 兄弟项目 Nerve

本项目最初从兄弟项目 Nerve 一次性拷贝过平台层与认证的代码，之后完全独立，不同步、不耦合。远程开发机上没有它，也不需要它。

M10/P2/S2 当前精确代码33b4的CI40718770843全部11作业成功，四片Test444/419/302/271秒仅一个分支样本；两次main均值仍按上述规则另验。本机四份双轮及来源差异见[S2记录](../M10-llm-wiki/reviews/P2-S2-source-hash-review.md#33b4最终代码控制与本机验证)，不以本机量替代CI。
