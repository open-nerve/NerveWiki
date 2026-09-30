# M0/P6 端到端测试与交付：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m0-p6-e2e-delivery`（`a813547..4d19b54`），对照 [06-P6-e2e-delivery.md](../06-P6-e2e-delivery.md)、各 Step 计划、[M0 总设计](../00-M0-design.md)第 3、7、8、9 节与[总体设计](../../v0.1-design.md)第 10、11 节 |
| 审查方式 | 独立审查者在仓库的克隆上实测：<br>• 跑全部门禁；`make e2e` 重复运行：`--repeat-each 5`、`--workers 1 --repeat-each 5`、`--repeat-each 20 --workers 8`；<br>• `make image-smoke` 用默认版本与非默认版本各跑一次；<br>• 检查镜像的层与内容、基础镜像的摘要、CI 中各 action 的 SHA；<br>• 重跑第 5 节的反向对照，另外自拟探针：服务起不来、挂住、忽略 SIGTERM 时 fixture 的报错；失败时的产物；knip 是否覆盖 `e2e/`；image-smoke.sh 在各种失败与中断下的清理 |
| 日期 | 2026-09-30 |
| 结论 | 基本达到生产级：<br>• fixtures 分工清楚：数据库、进程、页面监视各一个文件，`test.ts` 只接线；最大的文件 184 行；<br>• 从 Nerve 拷来的部分裁剪得当，新增的开关都是故事必需的；<br>• 门禁全绿，故事稳定（140 次重复全部通过），反向对照有效。<br>1 项 Important（镜像的提交信息总是 `modified`）、5 项 Minor、9 项 Nit，全部处理。各项有意的决定经审查确认合理 |

审查之前，持续集成的反向对照另用一次临时提交验证：S4 故意失败时，e2e 任务上传了 `playwright-report` artifact（约 650 KB，含报告与 `test-results/`）；临时分支已删除。

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| I1 | Important | 从干净的提交构建的镜像，`nervewiki version` 总是报告 `modified=true`，而本地 `make build` 的是 `false`。<br>原因：server 阶段只复制了 `.git` 与 `server/`，其余 316 个文件在 git 看来都已删除。镜像是 M0 要发布的产物，它的来源信息错了，却没有测试守着 | • server 阶段改为复制整个构建上下文（`.git` 每次提交都变，这一层本来就不能复用，构建成本不变）；<br>• `.dockerignore` 排除的正好是 `.gitignore` 忽略的，`webui/dist/.gitkeep` 保留；README 写明两者同步；<br>• image-smoke.sh 用镜像执行 `version`，核对版本号、提交与本地 `git rev-parse HEAD` 相同、`modified` 与本地 `git status` 一致；`/api/v0/instance` 的 `commit` 也改为核对确切的提交。<br>反向对照：在干净的提交上恢复原来的两行 COPY，image-smoke 报 `modified=true`、应为 `false` |
| M1 | Minor | `USER nonroot:nonroot` 是用户名。Kubernetes 在 `runAsNonRoot: true` 而没有设 `runAsUser` 时，无法核对用户名，拒绝启动 | 改为 `USER 65532:65532`，注释写明原因 |
| M2 | Minor | `.dockerignore` 的模式从上下文根目录算起，与 `.gitignore` 不同：`.env`、`.env.*` 只排除根目录的。子目录中的 `web/apps/web/.env.local` 会进入构建，Vite 会读它 | 与 I1 一起处理：任意层级的写成 `**/`（`.env`、`.env.*`、`config.local.yaml`、`node_modules`、`coverage` 等） |
| M3 | Minor | image-smoke.sh 中 `migrate up` 的容器没有名字，清理删不到它。在迁移期间中断脚本，会残留这个容器和网络（网络因容器仍连着而删不掉，错误被吞掉） | migrate 容器命名为 `<前缀>-migrate`，列入清理。<br>复验：迁移期间向脚本发 SIGTERM，退出码 143，没有残留 |
| M4 | Minor | S2 的注释说图标也检查了加载成功，但无界面的 Chromium 根本不请求 `/favicon.svg` | 注释去掉图标 |
| M5 | Minor | image-smoke.sh 的 `curl` 没有超时：服务接受连接却不回答时，等待会一直挂住，只能靠持续集成任务的超时兜底 | 所有请求经 `get()`，带 `--max-time 5` |
| N1 | Nit | 运行时阶段的 `FROM` 没写标签，与"注释中是对应的标签"不符；阶段没有命名 | 写成 `static-debian13:latest@sha256:…`，阶段命名 `runtime` |
| N2 | Nit | test 配置的日志级别是 warn：服务日志在成功时是空的，失败时也看不到请求 | e2e 起的服务与命令设 `NWIKI_LOG__LEVEL=info`：日志写在文件里 |
| N3 | Nit | 三处注释不准：<br>• 快照"与服务日志一起附进测试"：实际上 worker 的日志不附进测试；<br>• 快照只导出 worker 的库；<br>• fixture 预算"可能短于启动超时"：两者实际相等 | 注释改为如实：<br>• 日志的位置；<br>• `newDatabase` 的库不导出；<br>• 默认预算容不下启动与停止两个超时之和 |
| N4 | Nit | S2、S3 重复读取与检查 `NWIKI_E2E_VERSION` | `fixtures/test.ts` 提供 `stampedVersion()`，没有设置时报错并说明用 `make e2e` |
| N5 | Nit | S2 自己收集的失败请求也包括接口，与 `watch.apiFailures` 重复 | 只收静态资源 |
| N6 | Nit | `Database.name` 没有使用方，是从 Nerve 带来的 | 删除 |
| N7 | Nit | image-smoke 还需要 `curl` 与 `jq`，文档只写了 Docker；缺参数时只报 `$1: unbound variable` | Makefile、README、脚本头部写明依赖；参数缺失时打印用法 |
| N8 | Nit | image-smoke 不检查内嵌的前端：按 I1 保留 `.gitkeep` 之后，漏掉 `COPY --from=web` 不再编译失败，镜像会静默地答"未构建"的 404 | image-smoke 请求 `/`：200、带页面的 CSP、是前端的 `index.html`。<br>反向对照：删掉 `COPY --from=web`，失败 |
| N9 | Nit | 没有构建二进制就运行故事，全局准备只报 `spawn … ENOENT` | 全局准备先检查 `bin/nervewiki`，不存在时说明用 `make e2e`；PostgreSQL 还没启动。<br>"应用容器退出后 `wait_for` 仍等满 60 秒"保持现状：只影响失败时的耗时 |

## 审查者对有意决定的判断

| 决定 | 判断 |
|---|---|
| `e2e` 包不设 `test` 脚本 | 同意：否则 `make test-web` 会跑到需要 Docker 与 Chromium 的故事 |
| `newDatabase("migrated" \| "empty")`；`nervewikiWith(databaseUrl, { env, until })`，`until: "live"` 只等 `/healthz`；新建的库不删 | 同意：`until` 是"迁移未完成"唯一需要的开关；按 URL 传参，与 `Database` 解耦；库随本次运行的容器消失 |
| distroless 默认变体加显式 `USER` | 同意：去掉 `USER` 的反向对照因此有意义（实测 uid 为 0，脚本失败）；用户要写成数字（M1） |
| 不设 `NWIKI_SERVER__ADDR` | 同意：默认 `:8080` 监听所有地址 |
| CI 不另设构建步骤 | 同意 |
| worktree 中 `make image` 直接报错；提交信息只有一个来源 | 同意；单一来源在 I1 修复后仍然成立 |
| `expectQuietConsole` 不带"预期的日志" | 同意：M0 没有需要点名的第三方日志 |
| S2 断言 CSP 的完整字符串 | 可以接受：从外部核对实际下发的头，中间件改写了也能拦住；改 CSP 时必须有意识地同步 e2e，对安全头是合理的代价 |
| image-smoke 核对 SIGTERM 后退出码为 0；OCI labels | 同意：前者证明 exec 形式的入口下 PID 1 能处理信号 |
| "数据库不可用"在复制出的另一个库上制造 | 同意：不破坏 worker 共用的库 |

## 反向对照

设计第 5 节的各项，审查者重跑，全部按预期失败：

- 页面加内联脚本：S2 失败，CSP 违规 `script-src-elem inline`；
- 首页多请求一个不存在的接口：S2 失败；
- 组件打出 `console.warn`：S2 的控制台检查失败；
- 不注入版本：S3、S2 失败。这一项只在 `VERSION` 不是默认值时成立：默认的 `0.1.0-dev` 与 `buildinfo` 的默认值相同，持续集成用 `0.0.0-ci.<运行号>` 证明注入；
- `webui` 对深层链接答 404：S4 失败；
- `/readyz` 不检查数据库：S1 的"数据库不可用"失败；
- 去掉 `USER`：image-smoke 报 uid 为 0；
- 持续集成的 e2e 任务在失败时上传报告与日志：见上文。

审查者的探针：
- fixture 的超时与报错先于 Playwright 的超时出现，带日志路径：服务以非零码退出（2.5 秒）、挂住不就绪（30 秒）、忽略 SIGTERM（约 32 秒后 SIGKILL）；没有残留进程；
- 镜像的最终层只有 distroless 与 18 MB 的二进制，没有 `.git`、源码或 `/src` 路径；
- 加 `-buildvcs=false`：image-smoke 报 commit 为 `unknown`。

修复后新增的反向对照见上表 I1、M3、N8。

## 核实修复

- 本地 `make check`、`make e2e` 为绿；`make image-smoke` 在有改动的工作区报告 `modified=true`、在干净的提交上报告 `modified=false`，都通过。
- 修复后的持续集成（run 36712342804）为绿。

## 移交

审查中讨论到、不属于 M0 的两项：
- [M7](../../M7-assets-transfer/handoffs/M0-P6-image-volumes.md)：镜像里的附件目录、卷与非 root 用户的写权限；
- [M12](../../M12-release/handoffs/M0-P6-image-release.md)：镜像的推送、多架构、签名与来源证明、基础镜像摘要的更新。
