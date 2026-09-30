# M0/P6 端到端测试与交付：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M0/P6 端到端测试与交付 |
| 状态 | 已完成 |
| 基线 | `f189644`（P5 完成：前端外壳、`webui`、`make build`） |
| 上级文档 | [M0 总设计](00-M0-design.md) 第 3、7、9 节；[总体设计](../v0.1-design.md) 第 10 节 |

---

## 1. 基线

P5 留下的：
- `make build` 构建前端并内嵌进 `bin/nervewiki`，版本号取 `VERSION`（ldflags 注入）；页面带固定的 CSP，没有内联脚本；任意页面路径答 `index.html`，`/api/` 下的未知路径答 404 problem+json。
- 首页显示实例版本；`zh-CN` 与 `en`；明暗主题；应用内 404；错误边界。

还没有端到端测试，也没有镜像；持续集成中还没有任务完整执行 `make build`。

前序 Phase 对本 Phase 的要求（M0 总设计"前序 Phase 对后续 Phase 的要求"）：

1. e2e 的 PostgreSQL 容器按 builtin `C.UTF-8` 初始化，镜像用 `postgres:18.6-trixie`（与开发库、`pgtest` 相同）。
2. 在 Docker 中构建时没有 `.git`，`buildinfo` 的提交信息要用 ldflags 注入，或者把 `.git` 带进构建上下文。
3. `serve` 启动时要连上数据库，连不上就拒绝启动。S1 的"数据库不可用"要在启动之后制造（例如删掉那个库）；"迁移未完成"用 `auto_migrate: false` 启动在未迁移的库上。
4. 持续集成中完整的 `make build`（同时需要 Go 与 Node）由 e2e 任务执行；S2 的"控制台没有警告"会拦住 React Router 缺 `HydrateFallback` 这类警告。

## 2. 目标与范围

**目标**：M0 的冒烟故事 S1–S4 在本地（`make e2e`）与持续集成里通过；产出以非 root 用户运行的镜像，镜像通过 S1、S3。定下以后每个 M 的端到端故事照着写的骨架。

**做**：
- `e2e/`（`@nervewiki/e2e`）：Playwright（Chromium）+ testcontainers。一次运行启动一个 PostgreSQL，全局准备时用 `bin/nervewiki migrate up` 迁移模板库；每个 worker 从模板库复制自己的库、运行自己的 `nervewiki serve`。
- fixtures：数据库（查询、失败时导出）、服务（启动、就绪、停止、命令）、浏览器监视（接口请求与失败、页面异常、控制台错误与警告、CSP 违规）、类型化的 API 客户端。
- 故事 S1–S4（`e2e/stories/smoke/`）。
- S3 核对 `make build` 注入的版本号（P5 已加入 `VERSION`）。
- `deploy/Dockerfile`：多阶段构建（前端 → Go → 运行时），运行时以非 root 用户运行；`make image` 构建，`make image-smoke` 在镜像上跑 S1、S3。
- 持续集成：`e2e` 任务（失败时上传报告、trace 与每个 worker 的服务日志）；`image` 任务。

**不做**：
- 多浏览器：只跑 Chromium，其余浏览器随有浏览器差异的功能加入。
- 镜像发布到仓库、签名、多架构：随 v0.1 的发布流程（M12）。
- 数据库断言函数的目录 `e2e/fixtures/assert/`：M0 的故事只查一次 `pg_stat_activity`，第一个有业务表的 M 建立它（M0 总设计第 8 节的约定照旧）。

## 3. 设计

### 3.1 目录

```
e2e/
  package.json              @nervewiki/e2e：check:types。不设 test 脚本：make test-web（pnpm -r run test）
                            不跑需要 Docker 与 Chromium 的故事，入口是 make e2e
  tsconfig.json
  playwright.config.ts      testDir stories；全局准备；en-US；html 报告；失败时保留 trace 与截图
  global-setup.ts           检查 bin/nervewiki，启动 PostgreSQL，迁移模板库；返回全局清理
  fixtures/
    db.ts                   PostgreSQL 容器、模板库、复制与打开数据库、pg_dump
    server.ts               bin/nervewiki：命令、serve 的启动（等就绪或只等存活）与停止
    browser.ts              页面监视、安静的控制台
    test.ts                 test：worker 级的 db、nervewiki；测试级的 api、newDatabase、nervewikiWith、
                            失败时的数据库快照；stampedVersion
  stories/smoke/
    s1-server-ready.spec.ts
    s2-web-app.spec.ts
    s3-instance-info.spec.ts
    s4-routing-fallback.spec.ts
deploy/
  Dockerfile
  image-smoke.sh            在镜像上跑 S1、S3（持续集成与 make image-smoke）
.dockerignore               排除的正好是 .gitignore 忽略的
```

依赖方向：`stories ──► fixtures/test.ts ──► fixtures/{db,server,browser}`。故事从 `fixtures/test.ts` 取 `test`、`expect` 与各个 fixture，也直接使用 `server.ts`（`runNervewiki`、`applicationName`）与 `browser.ts`（页面监视）中的函数；fixtures 之间只有 `test.ts` 导入其余三个。

### 3.2 运行方式

- `playwright.config.ts`：Chromium 一个项目，`locale: en-US`（故事读英文文案）；`fullyParallel` 取默认值 false（同一个 worker 的测试共用它的库与服务）；worker 数取 Playwright 的默认值；`forbidOnly` 在持续集成中开启；报告 `list` + `html`。
- **全局准备**：testcontainers 启动 `postgres:18.6-trixie`（`POSTGRES_INITDB_ARGS=--locale-provider=builtin --locale=C.UTF-8`），建模板库 `nervewiki_template`，`bin/nervewiki migrate up`（test 配置）；把服务器地址与容器 id 通过环境变量交给 worker。全局清理停止容器；进程意外退出时由 testcontainers 的 Ryuk 回收。
- **worker**：`CREATE DATABASE e2e_w<n> TEMPLATE nervewiki_template`；`bin/nervewiki serve`，`NWIKI_ENV=test`，`NWIKI_SERVER__ADDR=127.0.0.1:0`、`NWIKI_SERVER__ADDR_FILE` 取实际地址，日志级别 info、写进 worker 的输出目录；等 `/readyz` 为 200。调用者自己的 `NWIKI_*` 变量不带进去。
- **测试级**：
  - `api` 是指向本 worker 服务的类型化客户端；
  - `newDatabase("migrated" | "empty")` 在同一个 PostgreSQL 上另建一个库（模板库的副本，或未迁移的空库），随本次运行的容器消失；
  - `nervewikiWith(databaseUrl, { env, until })` 在给定的库上、带额外的环境变量再起一个服务，测试结束时停止；`until: "live"` 只等 `/healthz`，给不会就绪的服务用；
  - 测试失败时把本 worker 的库 `pg_dump` 下来附进报告（`newDatabase` 的库不导出）；
  - `stampedVersion()` 给出 `make e2e` 交来的版本号，没有时报错并说明用 `make e2e`。
- **稳定性**：Playwright 把 `repeatEachIndex` 算进 worker 的标识，`--repeat-each` 每一轮都换新的 worker；要检验同一个 worker 复用库与服务，配合 `--workers 1`。
- 每个超时都有上限：就绪 30 秒、停止 30 秒、命令 60 秒；worker fixture 的预算大于它们之和，fixture 自己的错误先报出来。

### 3.3 故事

| 故事 | 断言 |
|---|---|
| S1 健康检查 | 迁移完成后 `/healthz`、`/readyz` 为 200，服务持有到本库的连接（`pg_stat_activity`）；`migrate status` 列出全部迁移、都已执行。**数据库不可用**：在复制出的另一个库上起服务，就绪后删掉那个库，`/readyz` 答 503 `not_ready`、`/healthz` 仍为 200。**迁移未完成**：在未迁移的空库（builtin `C.UTF-8`）上以 `auto_migrate: false` 起服务，`/readyz` 答 503，detail 指出迁移 |
| S2 首页 | 打开 `/`：文档 200、`text/html`、带 P5 的 CSP；静态资源全部同源、全部成功；接口请求只有 `GET /api/v0/instance`，没有失败；没有页面异常、CSP 违规，控制台没有错误与警告；页面显示实例版本（与 `make build` 注入的版本相同） |
| S3 实例接口 | 类型化客户端 `GET /api/v0/instance`：200，`{product: "Nerve Wiki", version: 注入的版本, commit: 40 位十六进制, api_version: "v0"}` |
| S4 路由兜底 | 直接打开深层链接（例如 `/acme/notebooks/1`）：文档与 `/` 的字节相同，页面显示应用内 404，刷新后仍是；控制台安静。`GET /api/v0/nope`：404 problem+json，内容确切 |

"控制台安静"先写一条探针消息再断言，证明监视确实在听。M0 的故事没有需要放过的浏览器日志，`expectQuietConsole` 不带参数；以后的故事遇到第三方库的警告、刻意引起的 4xx 在控制台的报告时，再给它加上逐条点名的参数。

### 3.4 版本注入

- Makefile 的 `VERSION ?= 0.1.0-dev` 与 ldflags 由 P5 加入。
- `make e2e`：`make build` 之后运行 Playwright，把 `VERSION` 交给故事（`NWIKI_E2E_VERSION`）。持续集成用 `0.0.0-ci.<run_number>`，与默认值不同，证明注入生效；本地用默认的 `0.1.0-dev` 时它与 `buildinfo` 的默认值相同，证明不了注入。
- 提交信息仍取自 Go 工具链嵌入的 VCS 信息，只有一个来源，本地、持续集成与镜像的提交信息一致：
  - 镜像的构建阶段复制整个构建上下文，含 `.git`；构建阶段的 Go 镜像带有 git。
  - `.dockerignore` 排除的正好是 `.gitignore` 忽略的，构建阶段里 git 看到的工作区与本地相同，`modified` 如实。`image-smoke.sh` 核对镜像的提交、`modified` 与本地一致。
  - `git worktree` 的 `.git` 是指向别处的文件，`make image` 在那里直接报错。

### 3.5 镜像

`deploy/Dockerfile`，构建上下文是仓库根目录：

| 阶段 | 基础镜像 | 内容 |
|---|---|---|
| web | `node:24-trixie-slim` | `corepack enable`；只复制 pnpm 的清单与锁文件先 `pnpm install --frozen-lockfile`（依赖层可缓存），再复制前端源码与生成的接口类型，`pnpm --filter @nervewiki/web build` |
| server | `golang:1.27.1-trixie` | 先 `go mod download`（依赖层），再复制整个构建上下文（含 `.git`）；把前端复制进 `webui/dist`；`CGO_ENABLED=0 go build -trimpath -ldflags "-X …version=$VERSION"` |
| runtime | `gcr.io/distroless/static-debian13`（默认变体） | 只有二进制；`USER 65532:65532`（distroless 的 nonroot）；`ENV NWIKI_ENV=prod`；OCI labels（标题、版本、源码、许可证）；`EXPOSE 8080`；`ENTRYPOINT ["/nervewiki"]`，`CMD ["serve"]` |

- 基础镜像按摘要（digest）固定，`FROM` 行同时写明标签。
- 用默认变体加显式的 `USER`，而不是 `:nonroot` 变体：去掉 `USER` 时进程是 root，"去掉 `USER`"的反向对照才有意义。用户写成数字：Kubernetes 的 `runAsNonRoot` 无法核对用户名。
- 不设监听地址：配置的默认值 `:8080` 已监听所有地址。
- 不写 `HEALTHCHECK`：运行时镜像没有 shell 与 curl；编排系统用 `/healthz`、`/readyz` 做探针（README 写明）。
- prod 配置不自动迁移：部署时先 `docker run … migrate up`，再 `serve`（README 写明）。

`deploy/image-smoke.sh <镜像> <版本号>`（需要 Docker、curl、jq）：
1. 用镜像执行 `version`：版本号是注入的，提交与 `modified` 与本地工作区一致；
2. 在一个临时的 Docker 网络里启动 PostgreSQL（builtin `C.UTF-8`）→ 用镜像执行 `migrate up` → 以镜像启动 `serve` → 等 `/readyz` 为 200；
3. S1：`/healthz`、`/readyz`；
4. 内嵌的前端：`/` 答 200，带页面的 CSP，是前端的 `index.html`；
5. S3：`/api/v0/instance` 的产品名、注入的版本、确切的提交、接口版本；
6. 容器内进程的 uid 是 65532；
7. SIGTERM 之后退出码为 0；
8. 无论成败都清理容器与网络，失败时打印服务的日志；每个请求最多等 5 秒。

### 3.6 命令与持续集成

| 命令 | 作用 |
|---|---|
| `make e2e` | `make build`，然后运行全部端到端故事（需要 Docker 与 Playwright 的 Chromium：`pnpm --filter @nervewiki/e2e exec playwright install chromium`） |
| `make image` | 构建镜像 `nervewiki:$(VERSION)`；在 `git worktree` 中直接报错 |
| `make image-smoke` | 构建镜像，在它上面跑 S1、S3（`image-smoke.sh`） |

持续集成：
- `e2e` 任务（在 `server`、`web` 之后）：Go 与 Node → `pnpm install` → 安装 Chromium（`--with-deps --only-shell`）→ `make e2e`（含 `make build`）；失败或取消时上传 `e2e/playwright-report/` 与 `e2e/test-results/`（含每个 worker 的服务日志与数据库快照）。
- `image` 任务（在 `server`、`web` 之后）：`make image-smoke`（含 `make image`）。检出时不留令牌：`.git` 进入镜像的构建阶段。
- `lint-web` 覆盖 `e2e/`（oxlint 的 Node 环境、类型检查）；knip 从 `pnpm-workspace.yaml` 自动识别 `e2e` 工作区，不需要改配置。

### 3.7 依赖版本

| 包 | 版本 |
|---|---|
| @playwright/test | 1.63.0 |
| @testcontainers/postgresql | 12.2.0 |
| pg / @types/pg | 8.23.0 / 8.23.1 |
| @types/node | 24.19.0（与 Node 24 对应） |
| typescript | 7.0.2（与 `apps/web` 相同，写进 pnpm 的 catalog） |

testcontainers 经 dockerode 间接依赖 `cpu-features`、`protobufjs`、`ssh2`，它们带安装脚本。`pnpm-workspace.yaml` 的 `allowBuilds` 逐个写明不运行：前两个只为经 SSH 连接 Docker 编译可选扩展，`protobufjs` 只检查版本号。

## 4. 实施步骤

在分支 `m0-p6-e2e-delivery` 上依次进行，每个 Step 结束时 `make check` 为绿。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | `e2e` 包与 fixtures；版本注入；S1–S4；`make e2e`；持续集成的 `e2e` 任务 | [P6-S1-e2e.md](plans/P6-S1-e2e.md) |
| S2 | Dockerfile、`.dockerignore`、`image-smoke.sh`；`make image`、`make image-smoke`；持续集成的 `image` 任务；README 的部署一节 | [P6-S2-image.md](plans/P6-S2-image.md) |

之后是反向对照、独立审查、修复、合并，然后是 M0 收尾审查。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 端到端 | S1–S4，本地与持续集成 |
| 镜像 | `image-smoke.sh`：S1、S3，进程不是 root |
| 其余 | 既有的门禁；`e2e` 的类型检查、oxlint、knip |

反向对照（验证后撤销）：
- 页面加一个内联脚本 → S2 的 CSP 违规检查失败；
- 首页多请求一个不存在的接口 → S2 的"只请求 instance、没有失败"失败；
- 组件打出一条 `console.warn` → S2 失败；
- `make build` 不注入版本 → S3 失败；
- `webui` 对深层链接答 404 → S4 失败；
- `/readyz` 不检查数据库 → S1 失败；
- 运行时镜像去掉 `USER nonroot` → `image-smoke.sh` 失败；
- 持续集成的 e2e 任务在失败时上传了报告与日志（用一次临时提交验证）。

## 6. 完成标准

- 第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 本地与持续集成为绿。
- 审查完成（`reviews/P6-e2e-delivery-review.md`），发现的问题已修复。
- M0 总设计进度表更新；M0 的完成标准（第 3 节）逐条核对，交给 M0 收尾审查。

## 7. 结果

**完成**：第 5 节全部通过，反向对照按预期失败。`make check`、`make gen-check`、`make e2e`、`make image-smoke` 本地与持续集成（run 36712342804）为绿。
- 故事稳定：审查者跑了 `--repeat-each 20 --workers 8`（140 次）与 `--workers 1 --repeat-each 5`，全部通过；持续集成中 e2e 任务约 1 分钟，image 任务约 1 分钟。
- 持续集成的 e2e 任务在失败时上传报告与日志，用一次临时提交（S4 故意失败）验证，临时分支已删除。
- 镜像：压缩后约 10 MB，最终层只有 distroless 与 18 MB 的二进制，没有 `.git` 与源码。

**与设计的差异**（第 3 节已是修订后的版本）：
1. `e2e` 包不设 `test` 脚本，端到端故事只由 `make e2e` 运行。
2. 测试级 fixture 改为 `newDatabase` 加 `nervewikiWith(databaseUrl, { env, until })`：S1 的两个变体一个要复制出的库，一个要空库，其中一个服务不会就绪（只等 `/healthz`）。
3. `expectQuietConsole` 不带"预期的日志"参数：M0 没有要放过的浏览器日志。
4. 运行时镜像用 distroless 的默认变体加 `USER 65532:65532`，不用 `:nonroot` 变体；不设监听地址；加了 OCI labels。
5. 镜像的构建阶段复制整个上下文，`.dockerignore` 与 `.gitignore` 对应（审查 I1）；`make image` 在 `git worktree` 中直接报错。
6. image-smoke 比设计多核对：提交与 `modified` 与本地一致、内嵌的前端、uid 等于 65532（比"不是 root"更严）、SIGTERM 之后退出码为 0。
7. e2e 起的服务日志级别为 info；Playwright 的 `locale` 固定为 `en-US`。
8. 版本注入的反向对照只在 `VERSION` 不是默认值时成立，由持续集成证明（3.4）。
9. P6 的前三个提交的说明没有按仓库的写法（`<范围>: <说明> (M0/P6/S1)`），已推送，没有改写历史；之后的提交照常。

**审查**：[P6 审查记录](reviews/P6-e2e-delivery-review.md)，1 项 Important、5 项 Minor、9 项 Nit，全部处理。

**移交**：
- [M7 的移交](../M7-assets-transfer/handoffs/M0-P6-image-volumes.md)：镜像里的附件目录、卷与非 root 用户的写权限；
- [M12 的移交](../M12-release/handoffs/M0-P6-image-release.md)：镜像的推送、多架构、签名与来源证明、基础镜像摘要的更新。
