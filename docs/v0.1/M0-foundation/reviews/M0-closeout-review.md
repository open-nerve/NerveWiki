# M0 基础骨架：收尾审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | `main` 的 `58a43af`（M0 的六个 Phase 全部合并之后），对照[文档约定](../../../README.md)的"M 完成"、[M0 总设计](../00-M0-design.md)与[总体设计](../../v0.1-design.md)中涉及 M0 的部分 |
| 审查方式 | 独立审查者在仓库的克隆上：<br>• 跑 `make check`、`make gen-check`（干净工作区）、`make e2e`、`make image-smoke`，查持续集成在该提交上的结果；<br>• 逐条核对 M0 的完成标准与总体设计 12.5；<br>• 检查跨 Phase 的一致性：接缝（配置 ↔ 组合根 ↔ 命令，httpserver ↔ apitest ↔ 模块 ↔ 生成代码，契约 ↔ TS 客户端 ↔ service，webui ↔ `make build` ↔ e2e ↔ 镜像）、命名、错误模型、门禁、版本固定；<br>• 核对 README、文档约定、M0 总设计、各 Phase 的"结果"、总体设计与代码是否一致；<br>• 核对 M0 产生的全部 handoff；<br>• 对照代码核实 M0 总设计第 8 节，起草总体设计第 13 节的条目；<br>• 用探针核实发现（临时故事、改动镜像版本、计数 Docker 卷等） |
| 日期 | 2026-09-30 |
| 结论 | M0 达到完成标准，可以收官：<br>• 门禁、冒烟故事、镜像、持续集成（run 36712814500）全绿；<br>• 依赖方向由 archtest 守住；最大的 Go 文件 381 行，前端 94 行，没有上帝文件；Go 约 4.3k 行非生成代码、7.3k 行测试；<br>• 防御性代码都有真实的缺陷作依据。<br>没有 Important；12 项 Minor、9 项 Nit：Minor 全部处理，Nit 处理 3 项，其余 6 项说明理由后保持现状。收尾的事务（第 13 节、进度表、本记录）一并完成 |

## 完成标准核对

| 标准 | 结论 | 证据 |
|---|---|---|
| 冒烟故事 S1–S4，本地与持续集成 | 满足 | 本地 `make e2e` 7 个用例通过；持续集成的 e2e 任务为绿 |
| 镜像能启动并通过 S1、S3 | 满足 | `make image-smoke` 与持续集成的 image 任务为绿。image-smoke 覆盖 S1 的就绪检查，两种 503 由 e2e 覆盖；完成标准的措辞已改正（N1） |
| 持续集成的全部任务为绿 | 满足 | server（lint、生成物、测试）、web（lint、生成物、knip、测试、构建）、e2e、image |
| P1 五项结论与总体设计的修订 | 满足 | M0 总设计第 7 节有结论；总体设计第 15 节有两条 P1 修订；M4、M5、M8、M9 各有移交 |
| 文档约定的"M 完成" | 满足（收尾后） | 六个 Phase 各有审查记录；本记录、第 13 节、M0 总设计与总体设计的进度表在收尾时完成 |
| 12.5：之前所有 M 的故事通过 | 满足 | M0 之前没有 M |
| 12.5：用 PAT 完整操作 | 不适用 | `instance` 是公开操作（`security: []`） |
| 12.5：扩展点 | 满足 | 扩展点表各行的"建立于"最早是 M1，没有 M0 的条目 |
| 12.5：先写描述，再写代码 | 满足 | P4 的契约提交（`291e987`）早于 `instance` 的实现（`172034c`） |
| 12.5：架构测试、depguard、前端静态检查 | 满足 | golangci-lint 0 issues；oxlint 零警告；三个包的类型检查；knip |
| 12.5：没有 `open` 的 handoff | 满足 | `M0-foundation/handoffs/` 没有事项；M0 产生的 8 份移交都在目标 M 的目录中 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| M1 | Minor | `make build` 与镜像的构建参数不同：前者链接 cgo，后者 `CGO_ENABLED=0`、`-trimpath`。e2e 测的不是要发布的那种构建，与总体设计 10.1"被测对象是真正要发布的产物"不符 | `make build` 改用与 Dockerfile 相同的参数，两处注释写明要同步；`go version -m bin/nervewiki` 显示 `CGO_ENABLED=0`、`-trimpath=true` |
| M2 | Minor | e2e 的控制台、CSP、页面异常检查要靠故事自己调用：一个不调用的故事在页面里打出 `console.error` 照样通过；vitest 已经全局强制，e2e 没有 | 覆盖 `page` fixture：每个页面从第一次导航之前就被监视，测试通过时由 fixture 核对页面安静（`expectQuietPage`）；故事用 `pageWatch` 只断言接口请求。<br>反向对照：不调用任何检查的临时故事打出 `console.error`、抛出未捕获的异常，都失败；S2 的内联脚本、`console.warn` 两项照样失败 |
| M3 | Minor | PostgreSQL 镜像写在四处，只有 `pgtest` 与开发用 compose 之间有测试：把 e2e 与 image-smoke 改成 `18.5-trixie`，测试照样通过 | `pgtest` 的测试改为核对 compose、`e2e/fixtures/db.ts`、`deploy/image-smoke.sh` 三处的镜像与建库参数。反向对照：改掉后两处，测试报出两处 |
| M4 | Minor | image-smoke 每次运行都留下一个 PostgreSQL 的匿名卷：`docker rm -f` 没有 `-v`，而镜像声明了数据卷 | 改为 `docker rm -fv`；复验卷的数量不变。e2e（testcontainers）不留卷，已核实 |
| M5 | Minor | 浏览器本地存储键的前缀与总体设计不一致：设计是 `nwiki.auth`，M0 用 `nervewiki.theme`、`nervewiki.locale`；等有了用户再改会丢掉他们的偏好 | 统一为 `nwiki.` 前缀（`nwiki.theme`、`nwiki.locale`）；总体设计 2.3 写明是前缀 |
| M6 | Minor | README 写的是 `apitest.Main(m, "<模块>")`，实际签名是 `Main(m)`（P4 审查改过） | README 改正，并说明模块名取自测试所在的路径 |
| M7 | Minor | 文档约定的提交信息 `<type>(M0/P3): …` 与实际做法不符：P3 起的代码提交是 `<范围>: <说明> (M0/Pn/Sm)`，文档提交是 `docs(M0/Pn): …`；P6 另有 3 个提交两种都不是 | 改约定，不改做法：P3 起的写法已稳定四个 Phase，历史已推送；范围加 Step 比 type 信息多，没有工具消费 type。文档约定写明代码、文档、合并三种提交的写法 |
| M8 | Minor | README 与 Makefile 说 `check` 加 `gen-check` 就是 server、web 任务的门禁，但 web 任务还构建前端，两者都不包含 | `make check` 加入 `build-web`（约半秒），这句话因此成立 |
| M9 | Minor | M0 总设计第 8 节的"配置"与"端到端故事的写法"两条在 M0 没有实例，模块配置的落点也没写 | 第 13 节写明落点（13.1 第 15 条、13.4 第 3 条）；第 8 节注明两条在 M0 还没有实例 |
| M10 | Minor | 工具链的待办事项在 M0 之后没有归属：Node 26 成为 LTS 之后评估升级、corepack 的提醒、是否引入 Dependabot | [M12 的移交](../../M12-release/handoffs/M0-P6-image-release.md)第 4 项扩为"依赖、工具链与基础镜像的更新" |
| M11 | Minor | 总体设计中与 M0 有关的几处没跟上实现：9.1 写"SPA 模式"（实际是数据路由）；8.4 的配置层次漏了 `$NWIKI_CONFIG_DIR`；8.2 的平台包清单没有 `buildinfo`；第 11 节写"数据目录与附件目录挂载为卷"（已移交 M7） | 逐处修订，写进总体设计的变更记录 |
| M12 | Minor | `httpserver.LongLived` 绕过全部逐路由中间件，不只是请求期限：请求体上限，以及以后的认证与限流也不经过。M1 的移交只提了认证；M9 的 MCP 流是带请求体的 POST | 第 13 节写明（13.1 第 14 条）；[M5](../../M5-collab-editing/handoffs/M0-P1-sse-proxies.md)、[M9](../../M9-mcp/handoffs/M0-P1-mcp-sessions.md) 的移交各加一项 |
| N1 | Nit | M0 总设计说"镜像通过 S1、S3"，而 image-smoke 只覆盖 S1 的就绪检查 | 措辞改为"S1 的就绪检查与 S3" |
| N2 | Nit | `/readyz` 的 detail "migrations is not ready" 语法不对 | 保持：detail 是给人看的说明，检查的名字是 `migrations`，为语法单独处理不值得 |
| N3 | Nit | README 与 `api/openapi.yaml` 把接口的写法约定指向 P4 文档 3.2；Phase 文档不再更新 | 改为指向规则测试 `apitest/rules_test.go` 与总体设计 13.1。代码注释中其余对 Phase 文档的引用说明的是"为什么"，作为历史保留 |
| N4 | Nit | 平台码与字段码的字面量在 httpserver、bodyshape、shared 各有一份，`bodyshape` 直接写 `"bad_request"` | 保持：`httpserver` 导入 `bodyshape`，后者不能反过来引用常量；它们按结构对接，由契约测试核对码与枚举一致 |
| N5 | Nit | 有几件预置的平台件在 M0 没有生产使用者（`database.commit_timeout`、`clock.System`、`LongLived`、`postgres.DB`、shared 的几个错误构造） | 保持：都在 M0 的设计范围内并有测试，M1 接入 |
| N6 | Nit | `NewAPI` 重复校验了配置校验过的值 | 保持：它是平台库的构造函数，校验自己的不变量（期限与上限为正），测试直接构造 `APIConfig` 时同样需要 |
| N7 | Nit | `.node-version` 只写大版本，Node 的补丁版本在本地与持续集成中浮动 | 保持：开发机不必逐个补丁安装；镜像中的 Node 按摘要固定。写进 13.5 第 2 条 |
| N8 | Nit | e2e 继承 `web/tsconfig.base.json`，位置暗示它只属于 web | 保持：出现第三个使用方时移到根目录 |
| N9 | Nit | "故事只从 `fixtures/test.ts` 取 `test` 与 `expect`"没有 lint 规则 | 保持：M2 之后故事不再需要 `browser.ts`；S1 使用 `server.ts` 的命令函数是正当的，规则的收益不大 |

## 第 13 节

M0 总设计第 8 节的约定经审查者对照代码核实（模块入口的签名、规则测试、配置的强类型与校验、模板库），连同第 8 节漏掉的、M0 实际建立的约定，补进[总体设计](../../v0.1-design.md)第 13 节：
- 13.1：第 8 条（错误码）改写，新增第 11–16 条：模块入口、平台与共享内核、接口先行与生成物入库、长连接路由、配置、迁移；
- 13.2：新增第 6–9 条：接口客户端、加载、页面与 CSP、控制台；
- 新增 13.4 测试、13.5 工程。

提交信息的写法放在[文档约定](../../../README.md)，不放进第 13 节。

## handoff

M0 产生 8 份移交，都以 yaml 头开头、状态 `open`，都在来源的审查记录中有链接，内容与代码相符，目标 M 合理：
- P1：[M4](../../M4-pages/handoffs/M0-P1-editor.md)、[M5](../../M5-collab-editing/handoffs/M0-P1-sse-proxies.md)、[M8](../../M8-history-search/handoffs/M0-P1-cjk-search.md)、[M9](../../M9-mcp/handoffs/M0-P1-mcp-sessions.md)；
- P3、P4、P5：[M1](../../M1-auth/handoffs/)各一份，合计约 19 项，写 M1 的 00 号文档时要把这部分工作量算进去；
- P6：[M7](../../M7-assets-transfer/handoffs/M0-P6-image-volumes.md)、[M12](../../M12-release/handoffs/M0-P6-image-release.md)。

`M0-foundation/handoffs/` 没有事项。

## 核实修复

- 本地 `make check`（含前端构建）、`make e2e`、`make image-smoke` 为绿；生成物没有差异。
- 修复后的持续集成（run 36715510055）为绿。
