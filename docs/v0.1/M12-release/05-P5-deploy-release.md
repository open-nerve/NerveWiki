# M12/P5 部署文档与发布流水线：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M12/P5 部署文档与发布流水线（三个 Step：S1 版本号与流水线的构建；S2 推送、签名、证明、SBOM、许可证与 Release；S3 部署文档与发布说明） |
| 日期 | 2026-11-16（初稿；设计审查；三次修订；两轮修订核对）；2026-11-18 照 S1 的实施与审查修订；2026-11-19 照 S2 的实施与审查修订 |
| 状态 | 进行中（设计已审查，[设计的审查记录](reviews/P5-design-review.md)；S1 已合并 `3ef1501f`（2026-11-18），[S1 的审查记录](reviews/P5-S1-version-pipeline-review.md)，结果在 10.1；S2 已合并 `（待填：合并号）`，[S2 的审查记录](reviews/P5-S2-publish-release-review.md)，结果在 10.2；S1、S2 的实施与审查之后与本文不同之处照实改在各节，3.24、3.25 列出） |
| 上级文档 | [M12 总设计](00-M12-design.md) 第 2 节，第 3 节第 1、7–11 条，4.3，4.8–4.13，5.1 末行，5.3，5.4，第 7 节 P5 一行与"收尾与发布""次序与并行""移交的落实""总体设计的修订"第 6、7、9 条，9.5–9.7，第 10 节，第 11 节决定 306、308、320–324、332、347 与"需要负责人确认"；[M12 设计审查](reviews/M12-design-review.md) B 的 I3、M4–M11、N1–N7、Q1 与"未核实的怀疑"；[移交](handoffs/) M0-P6-image-release 与其余移交里标给 P5 的部署项；[P1 设计](01-P1-foundation.md) 第 9 节；[v0.1 总体设计](../v0.1-design.md) 第 11 节、13.5、第 14 节 |
| 代码位置的写法 | 同 00 号文档与 P1：模块写作 `page/…`（`server/internal/modules/` 之下）；`platform/…`、`bootstrap/…` 在 `server/internal/` 之下；其余写仓库根起的路径。行号都是 `main` 的 `86565dd6` 的。00 号文档的行号以它头上的 `5e8af5b5` 为准，本文引它时只写节号；2.1 写出的旧文字与行号是 `86565dd6` 上的 00 号文档 |
| 工作笔记 | 本文引的每个行号都在 `86565dd6` 上读过（附录）。公开的东西只读：十二个 action 与工具的 tag 与它指的提交（设计者起草时以 `git ls-remote` 读过一次，审查没能复核，见 3.21）；`actions/attest` v4.2.2 的 README 与 `src/sbom.ts`；`anchore/sbom-action` v0.24.3 的 `action.yml` 与 `src/SyftVersion.ts`；`sigstore/cosign-installer` v4.1.3 的 `action.yml`；`docker/build-push-action` v7.4.0 的 `action.yml`；actionlint 的 CHANGELOG；GitHub 文档（Container registry、immutable releases）；Docker 文档（`imagetools create`）；arm 的 runner 的软件表；本机 gh 2.103.2 的帮助；公开接口上 `ci.yml` 的运行。小实验在临时的 worktree 里（用完已删），输出在 `m12/p5-design/`。三位审查者的探针在 `m12/p5-review-{a,b,c}/`，两轮修订核对的在 `m12/p5-check{1,2}-{a,b,c}/`。actionlint 不在本机跑（设计时被权限分类器拒过，3.5） |

---

## 0. 分三个 Step

照 00 号文档第 7 节，P5 分三个 Step，各自一个分支（`m12-p5-s<m>`）、一次审查与修复核对、一次合并。每个提交单独为绿（02-workflow 第 3.1 节）。

- **S1 版本号与流水线的构建**（`m12-p5-s1`，P1 之后就能开，与 P2–P4 并行）：
  - 版本号默认值的守卫：`Makefile`、`buildinfo`、Dockerfile 的两个 `ARG` 与 README 写的默认值相同（3.2）；
  - MCP 的 `serverInfo` 取 `buildinfo.Get().Version`，由 bootstrap 的一个短测试与 mcpserver 的一个握手测试守住；`make test-go-short` 以一个不是默认值的版本号构建测试的二进制，写死默认值的变异才抓得到（3.3）；
  - 镜像补 `revision`、`created`、`description` 三个 label，`make image` 传它们，`image-smoke.sh` 逐字核对（3.4）；
  - `ci.yml` 只在分支上跑；actionlint 由 `make lint-actions` 只在 runner 上跑，CI 的 `costs-1` 一份里一步（3.5）；
  - 闸门 `deploy/release-gate.sh` 与说明 `deploy/release-notes.sh`（3.6、3.7）；
  - `.github/workflows/release.yml`：每次推送的只读演练（两种原生 runner 各自构建、smoke），tag 上的闸门与两种架构按摘要推送、拉回来 smoke（3.8）；
  - 读三份工作流的守卫，闸门与说明对着假输入的测试，都是 Go 的短测试（3.9）；
  - **单独为绿**：tag 的作业在分支上不触发；分支的 CI（`ci.yml` 十九项）与 `release.yml` 的两个演练作业都绿。演练证明两种架构的构建与 smoke；不推送任何东西。
- **S2 推送、签名、证明、SBOM、许可证与 Release**（`m12-p5-s2`，S1 之后）：
  - 镜像里的第三方许可证 `/licenses`，由 Dockerfile 的两个阶段收集，`image-smoke.sh` 核对（3.11）；
  - 两份 SBOM：镜像的（syft）与内嵌前端的生产依赖（`pnpm sbom`）（3.12）；
  - 版本号的 tag 与 Release 的两步写成脚本，对着假的 `docker`、`gh` 测试（3.13）；
  - `release.yml` 的发布部分：只读的 `sbom` 作业；`publish` 合成索引（先打 `sha-<提交>`）、来源证明、两份 SBOM 的证明、cosign 签名，最后复制出版本号的 tag；`draft` 建 Release 的草稿、传附件；`verify` 在两种架构上按 tag 拉回来校验；`release` 公开 Release（3.14）；
  - 演练加三步：从本地的镜像取出 `/licenses`、生成两份 SBOM，只读（3.15）；
  - **单独为绿**：分支上只有演练与守卫在跑。推送、签名、证明与 Release 这条路**只有推 tag 才证明得了**，所以要 4.11 的 rc（3.23）。
- **S3 部署文档与发布说明**（`m12-p5-s3`，P4/S3 与它引出的条件修复、P5/S2 之后；不等 P2 全部合并，2.1 第 9 条）：
  - `deploy/compose.yaml` 与它的初始化脚本、`make deploy-check`，CI 的 `image` 一项跑它；PostgreSQL 镜像的守卫加上它（3.17）；
  - `deploy/README.md` 作运维手册，硬件下限与容量的数取自 P4/S3（3.18）；
  - 根 README 的"部署"一节缩成入口，守卫里 README 一项换成 `deploy/README.md`；`CHANGELOG.md` 的 0.1.0 一节；`SECURITY.md`；`manual/` 的读屏清单与验收入口（3.19）；
  - 照 `deploy/README.md` 用 `deploy/compose.yaml` 在本机装一遍、升一次级、备份再恢复一遍（3.20）；
  - **单独为绿**：只改文档、一个样例、一个检查、一个守卫与两份 ignore 文件。

## 1. 基线

从 `main` 的 `86565dd6` 开始（`docs: M12/P1 done`）。P1 留下了 Go 1.27.4 与它的守卫（`platform/buildinfo/toolchain_test.go:16-84`）、PostgreSQL 18.7、`make vuln`、`make outdated` 与 `vuln.yml`，以及按工作流取运行的 `ci-wait.sh`（`b54cd679`）。

**版本号**：

- 唯一的注入点是 ldflags 写 `buildinfo.version`（`platform/buildinfo/buildinfo.go:6-9`）。默认值 `0.1.0-dev` 有四份：`Makefile:13`、`buildinfo.go:9`、`deploy/Dockerfile:32`、`:45`（P1/S2 在 Dockerfile 的 server 阶段加了三行，00 号文档写的 `:29`、`:42` 是 `5e8af5b5` 的）；另有 `README.md:43` 写着"默认 `0.1.0-dev`"。没有守卫：改了 `Makefile` 漏了 Dockerfile，`make image` 不受影响（它总传 `--build-arg`），直接 `docker build` 得到旧的默认值。`Makefile:12` 的注释只说"与 buildinfo 中的相同"。
- 读它的地方：`nervewiki version`、`/api/v0/instance`（`instance/adapter/buildinfo/source.go:15-16`）、用户菜单、MCP 的 `serverInfo`（`bootstrap/mcp.go:90`，经 `mcpserver.Options.Version` 到 `mcp.Implementation`，`platform/httpserver/mcpserver/server.go:153`）。`image-smoke.sh` 只核对前两样（`deploy/image-smoke.sh:76-78`、`:131-136`）；`e2e` 的 S2、S3 核对用户菜单与 `/api/v0/instance`（`e2e/stories/smoke/s2-web-app.spec.ts:60`、`s3-instance-info.spec.ts:10`）。`serverInfo` 没有任何测试读；mcpserver 的测试夹具写死 `Version: "test"`（`mcpserver/harness_test.go:246`）。
- 测试的二进制不带 ldflags，`buildinfo.Get().Version` 是默认值。所以"`serverInfo.Version` 等于 `buildinfo.Get().Version`"的断言，抓不到把 `bootstrap/mcp.go:90` 写死成 `"0.1.0-dev"` 的错：两边一样（3.3）。
- `buildinfo_test.go:44-48` 的 `TestGetReportsStampedVersion` 钉住 `Get()` 把 `version` 传给 `fromSettings`（改成别的它就红），不钉默认值。
- 0.1.0 之后改 `0.1.1-dev` 时随之变的（00 号文档 4.9）：`api/modules/instance.yaml:80` 的示例，经 `make gen` 的 `api/dist/openapi.yaml:3293`、`instance/adapter/http/gen/server.gen.go:63` 与 00 号文档漏列的 `web/packages/api-client/src/schema.gen.ts:2211`，`instance/domain/info.go:15` 的注释，`README.md:43`。`platform/httpserver/apitest/apitest_test.go:19`、`:110` 是独立的假数据，不随之改。

**镜像与 CI**：

- Dockerfile 三个阶段（`deploy/Dockerfile:7`、`:23`、`:44`），整个上下文连 `.git` 复制进 server 阶段取 VCS 信息（`:21-22`、`:30`）。label 只有 title、version、source、licenses（`:46-49`）。`.dockerignore` 不排除 `.git`，排除的正好是 `.gitignore` 忽略的（13.5 第 3 条）。
- `make image` 在 worktree 里拒绝（`Makefile:89`），判断与 `docker build` 分在两行（`:89`、`:90`）：`make -i image` 会接着构建（05-lessons 第 7 节"Make 的配方里，拒绝的判断与干活写在同一个 shell 行"）。S1 改这个配方时一并合成一行。
- `ci.yml` 的 `on: push` 不带过滤（`.github/workflows/ci.yml:3-5`），推 tag 会再跑一遍十九项。工作流级的权限只有 `contents: read`（`:13-14`）。`image` 一项（`:318-334`）只在 `ubuntu-24.04` 上 `make image-smoke`，只有 amd64。它的 Image smoke 一步在最近五次 `main` 上是 68–80 秒（run 43650412259、43689756257、43693474739、43729199362、43745380462，公开的接口读的）：构建加 smoke 都在里面。00 号文档估演练"各 10–15 分钟"，照这个数偏大（2.1 第 1 条）。
- `costs-1` 一份的 Lint 一步（`make lint-go`，`ci.yml:169-171`）在上面几次 `main` 上 92–143 秒。`short` 一份跑 `make test-go-short`（`Makefile:600`，`go test -short -count=1 ./...`）。
- `make vuln`、`make outdated` 以 `$(ci-only)` 起头，只在 runner 上跑（`Makefile:171-193`，决定 308）。
- `.github/workflows/` 只有 `ci.yml` 与 `vuln.yml`。仓库里没有 tag，没有 Release，没有 `CHANGELOG.md`、`SECURITY.md`、`deploy/README.md`、`deploy/compose.yaml`。
- `image-smoke.sh` 照工作区算期望的 `modified`（`:15-20`，`$repo` 是仓库根）：脏的工作区也能过，所以发布的作业要在构建之前断言工作区干净（00 号文档 4.10 第 1 步）。它收任何镜像引用，`:77` 的 `docker run` 会拉取。它要 `docker`、`curl`、`jq`、`openssl`（`:8`）。arm 的 runner 的软件表列了 Docker、Buildx、Compose、Git、GitHub CLI、jq、Curl，没有列 openssl。

**部署文档**：根 README 的"部署"一节是 `README.md:498-633`：`:500` 是 M10 的压缩写法；`:502-509` 用 `make image` 构建；`:511-518` 环境与迁移；`:520-525` 附件目录与备份；`:527-544` 六条只对开发版有意义的升级说明（其中 `:535` 有对新装也成立的数）；`:546-550` 两个角色；`:552-627` 代理（`:552` 写着在代理之后要设 `NWIKI_SERVER__TRUSTED_PROXIES`）；`:628` 内存；`:630` 建库参数与 `postgres:18.7-trixie`；`:631-633` 探针、后台任务、停机。指向它的：`README.md:45`、`:229`，`deploy/runtime-grants.sql:2`，`bootstrap/runtime_role_test.go:31`、`:464` 的注释，总体设计第 11 节的首尾两段与写着"README 的部署一节"的四处（`v0.1-design.md:923` 两处、`:929`、`:1311`）。PostgreSQL 镜像的守卫核对 `compose.dev.yaml`、`e2e/fixtures/db.ts`、`image-smoke.sh` 与 README 四处加常量（`platform/postgres/pgtest/image_test.go:13-46`，`strings.Contains`）。

**工具链的事实**（2.3、3.21 用）：

- 本机 pnpm 12.8.1 自带 `pnpm sbom`（SPDX 或 CycloneDX，`--prod`，`--lockfile-only` 时不读 store）与 `pnpm licenses list --prod --json`。
- `go.yaml.in/yaml/v3` 是 `server/go.mod:28` 的直接依赖，测试可以读 YAML：解码进 `map[string]any` 时 `on` 是字符串键，`permissions: {}` 是空的映射，`if:` 是原样的字符串（审查 A 的探针核对过）。
- 本机的 Docker Compose 是 v5.6.1：`${VAR:?说明}` 没设时 `config -q` 以 1 退出、写出说明；`secrets` 的 `file` 不存在时 `config -q` 照过；插值与 schema 校验覆盖整份文件，不看 profile，`--profile` 只对服务之间的引用起作用（审查 C 的探针）。
- 官方 PostgreSQL 镜像的 `POSTGRES_USER` 总是超级用户；宿主机上的进程经发布的端口连进容器时，容器看到的对端是 compose 网络的网关（审查 C 照初稿的样例装了一遍：`172.19.0.1`，子网由 Docker 当场分配）。

## 2. 目标与范围

做的事对应 00 号文档第 7 节 P5 一行：版本号与流水线的构建（S1）；推送、签名、证明、SBOM、许可证与 Release（S2）；部署文档与发布说明（S3）。

不做：

- 推任何 tag。rc 与 `v0.1.0` 在收尾之后、问过负责人之后推（00 号文档 4.11）；本文只写那时的步骤（3.23）。
- 本机起 registry 演练推送与签名（00 号文档 9.5，设计审查 B 的 M10）。
- 在本机跑 actionlint（3.5）。
- 浮动的 tag（`latest`、`0.1`）、独立的二进制附件、宣传 `go install`（00 号文档 4.9、4.10）。
- `0.1.1-dev`：发布期的改动，在 `m12-release` 上（4.11 第 7 步）。
- `outdated.sh` 列 actionlint、syft、cosign、BuildKit 的版本：它们写在工作流的输入或 `Makefile` 里，不是 `uses:` 或 `FROM`，发布之前手动核一次（3.23 第 1 步），以后的进 v0.2 的移交。
- 构建与 pnpm 的缓存（P5-7）。

**移交的落实**（00 号文档"移交的落实"里 P5 的部分）：

| 移交的项 | P5 的落实 |
|---|---|
| [M0-P6-image-release](handoffs/M0-P6-image-release.md) 第 1 条（推送） | S1：tag 上两种架构按摘要推送，写权限只在 tag 的作业里、由守卫读工作流核对；S2：合成索引、证明与签名之后才复制出 `0.1.0`，已有的不改指 |
| 同上第 2 条（多架构） | S1：两种原生 runner 各自构建、各跑一次 image-smoke；演练在每次推送上 |
| 同上第 3 条（签名、SBOM、来源证明、校验说明） | S2：`actions/attest` 的来源证明与两份 SBOM 的证明、cosign keyless、公开 Release 之前两种架构上的校验；校验的命令在 Release 的说明（S1 的说明脚本）与 `deploy/README.md`（S3） |
| [M4-performance](handoffs/M4-performance.md) 第 2、3 条的内存、第 10 条（恢复之后先 `ANALYZE`、连接池代理只能用会话模式）、第 14 条（经负责人定的部署要求，若有）、第 17 条（WAL 的实测进部署文档）、第 18 条（慢的发送者与 Caddy 早答：推荐 nginx、写明代价）、补记 `:490`（硬件下限）、`:492`（慢的 4 核离 15 秒不远） | S3：`deploy/README.md` 的"要求""反向代理与 HTTPS""备份与恢复""运行与监控""容量"，数取 P4/S3（3.18） |
| [M5-performance](handoffs/M5-performance.md) 第 1、2、5 条 | S3：单实例与原因；时钟一条不适用；大版本升级复核 `NOTIFY` 那一条，总体设计第 14 节的那一行归 P5/S3 改（2.1 第 20 条） |
| [M7-transfer](handoffs/M7-transfer.md) 第 1 条（结束的任务行不清理）、第 3 条（`exports/`、`imports/`；几个 GB 的导出经代理的超时与缓冲）、第 4 条（校时）、第 5、6 条的部署项（停机时 `/mcp` 在 20 秒处被切断；由管理员监控磁盘） | S3 |
| [M8-history](handoffs/M8-history.md) 第 2 条（版本的增长，容量）、第 4 条（没有永久删除与变通） | S3 |

### 2.1 对 00 号文档的修订

照代码、公开文档、三份审查与主持者的决定，00 号文档有下面 24 处要改（第 17 条与第 12 条改同一句）。接受本文的文档提交以脚本 `apply_p5_design.py` 落进 00 号文档：每处一个带标签的替换，一处不匹配就什么也不写；P2、P3、P4 的设计也改的地方（第 11 节末尾、"需要负责人确认"的表尾、第 13 节的表尾）动态地找，几份设计的提交次序不拘（第 8 节第 4 条）。每条下面列出的是 `86565dd6` 上的旧文字与新文字（「」里），行号是锚点所在的行。

**第 1 条 演练的用时**（4.10"演练"的代价、第 7 节 P5/S1 一行、第 10 节一行）。00 号文档推断各 10–15 分钟；`ci.yml` 的 `image` 一项构建加 smoke 是 68–80 秒（第 1 节），改为分钟级的推断。不改任何决定。

- `1a-rehearsal-cost`（`:566`）：「推断各 10–15 分钟，与 `ci.yml` 并行」→「推断 amd64 约 2–3 分钟、arm64 约 2–5 分钟（`ci.yml` 的 `image` 一项构建加 smoke 是 68–80 秒，arm 的 runner 没有量过；P5/S2 在 amd64 上另加取许可证与两份 SBOM，约 1–2 分钟；P5 设计 2.1 第 1 条，原来推断各 10–15 分钟），与 `ci.yml` 并行」
- `1b-s1-row-cost`（`:772`）：「（推断各 10–15 分钟，与 `ci.yml` 并行）」→「（推断 amd64 约 2–3 分钟、arm64 约 2–5 分钟，与 `ci.yml` 并行；P5 设计 2.1 第 1 条）」
- `1c-risk-cost`（`:1107`）：「多两个各约 10–15 分钟的作业（推断）」→「多两个作业，推断 amd64 约 2–3 分钟、arm64 约 2–5 分钟（P5 设计 2.1 第 1 条；原来推断各 10–15 分钟）」

**第 2 条 只读的 `sbom` 作业**（4.10 的权限表与"来源证明与 SBOM"、决定 322）。原写法里 SBOM 在"合并、证明、签名"那个带 `id-token: write`、`packages: write` 的作业里：`pnpm install` 的安装脚本与 syft 的 action 不该在能签名、能写 GHCR 的作业里跑。权限表加一行（P5-8，3.14）。

- `2a-permissions-sbom`（`:574` 之后加一行）：「  | 取许可证、生成两份 SBOM（P5 设计 2.1 第 2 条） | tag | `contents: read`、`packages: read` |」
- `2b-sbom-readonly`（`:541`）：「覆盖二进制里的 Go 模块。」→「覆盖二进制里的 Go 模块。两份 SBOM 与取许可证在一个只读的作业里（`contents: read`、`packages: read`）：`pnpm install` 的安装脚本与 syft 不在能签名、能写 GHCR 的作业里跑（P5 设计 2.1 第 2 条）。」
- `2c-web-sbom-tool`（`:542`）：「用什么工具，P5/S2 的设计里先做个小实验。」→「用 pnpm 自带的 `pnpm --filter @nervewiki/web sbom --sbom-format spdx --prod`（SPDX 2.3；P5 设计 P5-9 的实验：要先装依赖，只读锁文件时许可证都是 NOASSERTION）。」
- `2d-decision-322-sbom`（`:1173`）：「前端另出一份 SBOM；」→「前端另出一份 SBOM，SBOM 的生成在只读的作业里（P5 设计 2.1 第 2 条）；」

**第 3 条 Release 先草稿，两种架构上核对过才公开**（4.10"Release""次序与重跑""发布之后的核对"与权限表、第 7 节 P5/S2 一行、决定 322、4.11"rc 不过时"）。GitHub 的 immutable releases 打开时，公开之后不能再传附件、tag 也不能再动，原来"没有就建、有就改、附件 `--clobber`"的写法整个失败。核对挪到公开之前是主持者定的（审查 B 的 Q1）：核对失败时只留下一个没公开过的草稿，可以删掉；代价是多一个 `contents: write` 的作业。镜像的版本号 tag 仍在核对之前出现，这一点改不了（P5-11，3.14）。同一段"次序与重跑"里"`imagetools` 得到同一个摘要"写成了事实（3.14 是推断，整次重跑时也不成立），"只有产品的问题才出下一个 rc"与决定 324、3.23 不合，一并改（修订核对 B 的 N14）。

- `3a-release`（`:546`）：「- **Release**（I1-D6）：用 runner 自带的 `gh`。Release 没有就建（`gh release create "$GITHUB_REF_NAME" --verify-tag --notes-file <文件>`），有就改（`gh release edit`）；`gh release edit` 不换附件，附件用 `gh release upload --clobber` 传，重跑时换掉旧的（修订核对第 1 轮 B 的 N8）。」→「- **Release**（I1-D6；P5 设计 2.1 第 3 条改）：用 runner 自带的 `gh`，经脚本 `deploy/github-release.sh`。先建草稿：没有就建（`gh release create --draft --verify-tag`），是草稿就改说明；附件用 `gh release upload --clobber` 传；发布之后的核对过了，最后一个作业才公开它（`gh release edit --draft=false`）。已经公开的不再改，只核对附件都在；`gh release view` 的失败只有"release not found"当作没有，别的让这一步失败。原来是"没有就建、有就改，附件 `upload --clobber`"：GitHub 的 immutable releases 打开时，公开之后不能再传附件、tag 也不能再动，那样写整个失败；先草稿后公开在两种设置下都能用，核对失败时只留下一个没公开过的草稿（主持者定，P5 设计审查 B 的 Q1）。」
- `3b-order`（`:557`）：「  5. 建或改 Release。」→「  5. 建或改 Release 的草稿，传附件；
  6. 两种架构上按版本号的 tag 核对（见"发布之后的核对"）；
  7. 公开 Release（P5 设计 2.1 第 3 条；原来第 5 步是最后一步，核对在它之后）。」
- `3c-rerun`（`:559`）：「Release 没有就建、有就改。」→「Release 的草稿没有就建、有就改，已公开的不动（P5 设计 2.1 第 3 条）。」
- `3l-rerun-digest`（`:559`）：「`imagetools` 得到同一个摘要，」→「`imagetools` 对同样的来源大概得到同一个摘要（推断；不同时版本号的 tag 照第 4 步不改指），」
- `3m-rerun-who`（`:559`）：「只有产品的问题才出下一个 rc 或 v0.1.1（4.11）。」→「产品的问题出下一个 rc，`v0.1.0` 之后出 v0.1.1（4.11）；流水线的问题出下一个 rc（决定 324），`v0.1.0` 上流水线自己的错不出 v0.1.1（4.11；P5 设计 2.1 第 3、16 条）。」
- `3d-verify-place`（`:560`）：「- **发布之后的核对**：最后一个作业是矩阵，」→「- **发布之后的核对**：公开 Release 之前的那个作业是矩阵（P5 设计 2.1 第 3 条；原来是最后一个作业），」
- `3e-verify-tail`（`:560`）：「本机只在包设为公开之后不登录地拉一次（4.11）。」→「本机只在包设为公开之后不登录地拉一次（4.11）。版本号的 tag 在核对之前就已出现（"次序与重跑"第 4 步）；核对失败时 Release 还是没公开过的草稿。」
- `3f-permissions-draft`（`:576`）：「| 建或改 Release |」→「| 建或改 Release 的草稿、传附件 |」
- `3g-permissions-publish`（`:577` 之后加一行）：「  | 公开 Release（P5 设计 2.1 第 3 条） | tag | `contents: write` |」
- `3h-s2-row`（`:773`）：「Release 没有就建、有就改；发布之后的核对作业（两种架构的矩阵）」→「Release 先建草稿、传附件；发布之后的核对作业（两种架构的矩阵）；核对过了才公开 Release（P5 设计 2.1 第 3 条）」
- `3i-decision-322-verify`（`:1173`）：「发布之后的核对在两种架构上；」→「发布之后的核对在两种架构上、在公开 Release 之前；」
- `3j-decision-322-release`（`:1173`）：「Release 不附二进制、没有就建有就改、附件 `upload --clobber`，」→「Release 不附二进制、先草稿后公开（P5 设计 2.1 第 3 条改，原来是"没有就建有就改、附件 `upload --clobber`"），」
- `3k-rc-fails`（`:603`）：「- **rc 不过时**：不删 tag、不删 Release，标明作废，修了推下一个 rc。」→「- **rc 不过时**：不删 tag；已公开的 Release 不删、标明作废，没公开过的草稿删掉（P5 设计 3.23）；修了推下一个 rc。」

**第 4 条 版本号的 tag 不改指**（4.10"次序与重跑"、决定 322）。整次运行重跑时构建得到新的摘要（层里的文件时间每次不同），合成的索引也是新的；原写法最后一步会把已有的 `0.1.0` 改指到新的摘要（P5-12）。

- `4a-version-tag`（`:556`）：「（只给一个来源时原样复制，摘要不变）；」→「（只给一个来源时原样复制，摘要不变）；已有的 `0.1.0` 指着别的摘要时这一步失败、不改指，指着同一个时不动：整次运行重跑时构建得到新的摘要（P5 设计 2.1 第 4 条）；」
- `4b-decision-322-tag`（`:1173`）：「证明与签名之后才复制出版本号的 tag；」→「证明与签名之后才复制出版本号的 tag，已有的不改指（P5 设计 2.1 第 4 条）；」

**第 5 条 核对与说明里的校验命令**（4.10"发布之后的核对"、5.4）。`gh attestation verify` 默认只认 SLSA 的来源证明，SBOM 的证明另加 `--predicate-type https://spdx.dev/Document/v2.3`（`actions/attest` 照 SBOM 的 `spdxVersion` 定，`src/sbom.ts`）。只带 `-R` 时仓库里任何工作流、任何 ref 做的证明都算数，所以给用户的命令与核对作业都带 `--cert-identity`，与 cosign 的身份逐字相同（审查 B 的 M4）；`gh` 要先登录（审查 B 的 N8）；cosign 写明 v3（主持者，审查 B 未核实的第 3 条）。

- `5a-verify-commands`（`:560`）：「跑 `gh attestation verify oci://ghcr.io/open-nerve/nervewiki:<tag> -R open-nerve/NerveWiki` 与 `cosign verify …`」→「跑 `gh attestation verify oci://ghcr.io/open-nerve/nervewiki:<版本> -R open-nerve/NerveWiki --cert-identity <同 cosign 的身份>`（来源证明；加 `--predicate-type https://spdx.dev/Document/v2.3` 是 SBOM 的证明，镜像的与前端的都要在；P5 设计 2.1 第 5 条）与 `cosign verify …`」
- `5b-5.4-gh`（`:687`）：「-R open-nerve/NerveWiki`」→「-R open-nerve/NerveWiki --cert-identity https://github.com/open-nerve/NerveWiki/.github/workflows/release.yml@refs/tags/v0.1.0`（来源证明；加 `--predicate-type https://spdx.dev/Document/v2.3` 是两份 SBOM 的证明；要先 `gh auth login`；P5 设计 2.1 第 5 条）」
- `5c-5.4-cosign`（`:688`）：「--certificate-oidc-issuer=https://token.actions.githubusercontent.com`」→「--certificate-oidc-issuer=https://token.actions.githubusercontent.com`（cosign v3，流水线用 v3.1.4）」

**第 6 条 MCP 版本的短测试要盖版本号**（4.9"MCP 的版本"、第 3 节第 8 条、9.6）。不盖时两边都是默认值，写死默认值的变异照过（第 1 节，P5-2，3.3）。9.6 的 P5 两行改为指向本文 7.2。

- `6a-mcp`（`:511`）：「image-smoke 照旧（P5/S1；设计审查 B 的 M9、C 的 M2）。」→「image-smoke 照旧（P5/S1；设计审查 B 的 M9、C 的 M2）。测试的二进制不带 ldflags 时两边都是默认值，写死默认值的错抓不到：`make test-go-short` 以 `-ldflags -X …buildinfo.version=0.0.0-short` 构建测试并设 `NWIKI_TEST_VERSION`，测试先核对盖上了；mcpserver 另有一个握手测试读 `serverInfo`（P5 设计 2.1 第 6 条）。」
- `6b-criteria-8`（`:122`）：「一个短测试断言两者相等，有反向对照（P5/S1）。」→「一个盖了版本号的短测试断言两者相等，有反向对照（P5/S1；P5 设计 2.1 第 6 条）。」
- `6c-9.6`（`:1089`）：「- P5/S1：四份默认值有一份不同（守卫抓）；MCP 的版本不取 buildinfo（短测试抓）；闸门的各条各去掉一条；闸门不分工作流；演练作业加了 write（权限检查抓）。」→「- P5/S1：照 P5 设计 7.2，其中：四份默认值与 README 写的有一处不同（守卫抓）；MCP 的版本写死成默认值（盖了版本号的 `make test-go-short` 抓，直接 `go test` 抓不到）；闸门的各条各去掉一条、闸门不分工作流、闸门放过空的一节（闸门的测试抓）；演练作业加了 write、tag 的作业的 `if` 放宽、闸门的漏洞一步 `continue-on-error`（守卫抓）（P5 设计 2.1 第 6、7 条）。
- P5/S2、S3：照 P5 设计 7.2，其中：版本号的 tag 改指、Release 已公开时照样上传（发布脚本的测试抓）；`/licenses` 缺 Go 本身的许可证（image-smoke 抓）；`compose.yaml` 引用错的 secret（`make deploy-check` 抓）；它的 PostgreSQL 镜像不同（守卫抓）。」

**第 7 条 守卫是 Go 的短测试，不是 `make lint` 的一步**（5.3、9.5 第 1 层）。守卫读三份工作流（`ci.yml` 不在 tag 上跑、`vuln.yml` 只读，也一起核对），闸门、说明与 S2 的两个脚本对着假的 `gh`、`docker` 与临时的仓库跑；都在 `platform/buildinfo`，与 P1/S2 的 Go 版本守卫相邻（决定 347 的建议），CI 的 `short` 与 `race` 两份跑它们（P5-1，3.9）。

- `7a-5.3-guard`（`:677`）：「| `release.yml` 的权限检查 | 读工作流，演练作业不许有 write，带反向对照 | `make lint` 的一步 | P5/S1 |」→「| 工作流与发布脚本的守卫（P5 设计 2.1 第 7 条） | Go 的短测试（`platform/buildinfo/release_test.go`）：读三份工作流，演练作业不许有 write，tag 的作业的权限、`if` 与调的脚本照表；闸门、说明与发布的脚本对着假的 `gh`、`docker` 与临时的仓库跑；都带反向对照 | `make test-go-short`、`make test`（CI 的 `short` 与 `race`）；本机与 CI | P5/S1、S2 |」
- `7b-9.5-guard`（`:1073`）：「   - 读 `release.yml` 的检查：演练作业没有 write。反向对照：给它加 `packages: write`，检查失败。」→「   - 读三份工作流的守卫（Go 的短测试，P5 设计 2.1 第 7 条）：演练作业没有 write，tag 的作业照表。反向对照：给它加 `packages: write`，守卫失败。」

**第 8 条 9.5 第 1 层的 `make vuln`**。`make vuln` 不在闸门脚本里，是 `gate` 作业的下一步（它只在 runner 上跑，脚本要在本机对着假输入跑）。"`moderate` 不挡、`high` 挡"是 `pnpm audit --audit-level` 的行为。钉住的是两件：守卫读 `gate` 作业的那一步恰是 `make vuln VULN_AUDIT_LEVEL=high`；级别从 `make` 传到 `pnpm audit`，由 PATH 里 `go`、`pnpm` 两个桩核对一次（同 P1/S2 的变异 15，7.2 的 v-gate），CI 里没有东西常驻地守着后一件（审查 A 的 N3）。

- `8a-9.5-vuln`（`:1074`）：「`CHANGELOG.md` 没有这一节、`make vuln` 红（npm 一侧照 `high`：一个 `moderate` 的公告不挡、一个 `high` 的挡），各被拒或放过一次（反向对照）。」→「`CHANGELOG.md` 没有这一节或它是空的，各被拒或放过一次（反向对照）。`make vuln` 不在脚本里，是闸门作业的下一步（它只在 runner 上跑）：守卫核对那一步是 `make vuln VULN_AUDIT_LEVEL=high`；级别传到 `pnpm audit`，由 P1/S2 的变异 15 那样的 `go`、`pnpm` 两个桩核对一次；"`moderate` 不挡、`high` 挡"是 `pnpm audit --audit-level` 的行为，不是我们的代码（P5 设计 2.1 第 8 条）。」

**第 9 条 P5/S3 不等 P2 全部合并**（第 7 节"次序与并行"、"收尾与发布"第 3 步）。主持者定（审查 C 的 Q2）：S3 只等 P4/S3（与它引出的条件修复）与 P5/S2；P2 合并晚于 S3 的部分（推后的 C 类等），收尾时补进 `CHANGELOG.md`（P5-22）。

- `9a-order`（`:798`）：「P5/S2 之后。」→「P5/S2 之后，不等 P2 全部合并：P2 合并晚于 P5/S3 的部分（推后的 C 类等），收尾时补进 `CHANGELOG.md`（主持者定，P5 设计 2.1 第 9 条）。」
- `9b-closeout`（`:784`）：「（4.13）。」→「（4.13）；P5/S3 之后才合并的 P2 的已知限制补进 `CHANGELOG.md`（P5 设计 2.1 第 9 条）。」

**第 10 条 actionlint 只在 runner 上跑**（4.10"actionlint"、5.3、9.5 第 1 层、第 7 节 P5/S1 一行）。主持者照决定 308 的先例定（审查 A 的 I1、C 的 Q1）：设计时在本机跑它被权限分类器拒过，规矩是不换个办法再做；放进 `make lint` 就要求每个实施者、每个提交在本机跑它。`make lint-actions` 以"只在 runner 上"的判断起头，CI 的 `costs-1` 一份跑它，出错写成公开的注解。不调 shellcheck：runner 装了版本不由我们定的 shellcheck（P5-6，3.5）。

- `10a-4.10-actionlint`（`:580`）：「- **actionlint**：进 `make lint`，核对三份工作流。」→「- **actionlint**：`make lint-actions`，只在 GitHub 的 runner 上跑：CI 的 `costs-1` 一份在 `make lint-go` 之后跑它，本机的 `make lint` 与逐个提交的门禁不含它，出错写成 GitHub 的注解（主持者照决定 308 的先例定：设计时在本机跑它被权限分类器拒过；P5 设计 2.1 第 10 条）。核对三份工作流，不调 shellcheck（`-shellcheck=`：runner 装了版本不由我们定的 shellcheck，actionlint 看到它才查 `run:`）。」
- `10b-5.3-actionlint`（`:676`）：「| `make lint` 加 actionlint | 核对三份工作流；至少 1.7.10，`go run …@版本` | 本机与 CI | P5/S1 |」→「| `make lint-actions` | actionlint 核对三份工作流；至少 1.7.10，`go run …@版本`，`-shellcheck=` | CI 的 `costs-1` 一份；**本机不跑**（P5 设计 2.1 第 10 条） | P5/S1 |」
- `10c-9.5-actionlint`（`:1072`）：「   - actionlint 核对三份工作流（`make lint`）。」→「   - actionlint 核对三份工作流（`make lint-actions`，只在 runner 上：CI 的 `costs-1` 一份；P5 设计 2.1 第 10 条）。」
- `10d-s1-row-actionlint`（`:772`）：「actionlint 进 `make lint`」→「actionlint 在 CI 上（`make lint-actions`，本机不跑）」

**第 11 条 版本号的清单**（4.9）。Dockerfile 的两行现在是 `:32`、`:45`；README 写的默认值进守卫（P5-3）；`0.1.1-dev` 时随之改的加 `schema.gen.ts:2211`。原稿说"守卫之后漏一处就红"说过了头：守卫只看四份默认值与 README，示例与注释照清单改，生成物由 `make gen-check` 管（审查 A 的 M8）。

- `11a-defaults`（`:509`）：「`deploy/Dockerfile:29`、`:42`，没有守卫（I1 第 1 节）。」→「`deploy/Dockerfile:29`、`:42`（`86565dd6` 上是 `:32`、`:45`），没有守卫（I1 第 1 节）；`README.md:43` 写着这个默认值。」
- `11b-guard`（`:510`）：「- **守卫**：一个短测试核对四份一致，带反向对照（P5/S1）。」→「- **守卫**：一个短测试核对四份与 README 写的一致，带反向对照（P5/S1；README 一项是 P5 设计 P5-3）。」
- `11c-after-release`（`:513`）：「`api/dist/openapi.yaml:3293` 与 `instance/adapter/http/gen/server.gen.go:63` 经 `make gen` 随之。」→「`api/dist/openapi.yaml:3293`、`instance/adapter/http/gen/server.gen.go:63` 与 `web/packages/api-client/src/schema.gen.ts:2211` 经 `make gen` 随之（最后一个是 P5 设计 2.1 第 11 条补的）。四份默认值与 README 漏一处，P5/S1 的守卫就红；示例与注释照这张清单改，生成物由 `make gen-check` 核对；`platform/httpserver/apitest/apitest_test.go:19`、`:110` 是独立的假数据，不改。」

**第 12 条 sbom-action 带的 syft；SHA 的核对**（4.10"新加的 action"）。v0.24.3 自带的是 syft v1.54.0（`src/SyftVersion.ts`），不是 I1 写的 v1.57.1（那是 syft 当时的最新），工作流里显式写 `syft-version`（P5-18）。十二个 SHA 只与 I1 的记录、现有的工作流对照过，审查没能以公开仓库复核，P5/S1 实施时核对（主持者，3.21）。第 17 条改同一句。

- `12a-syft`（`:579`）：「`anchore/sbom-action` v0.24.3、」→「`anchore/sbom-action` v0.24.3（自带 syft v1.54.0，不是 I1 写的 v1.57.1；工作流里显式写 `syft-version`，P5 设计 2.1 第 12 条）、」
- `12b-shas`（`:579`）：「实施时再核一次最新版。」→「实施时再核一次最新版：这些 SHA 只与 I1 的记录、现有的工作流对照过，P5 的设计审查没能以公开仓库复核，P5/S1 实施时核对（P5 设计 3.21）。`docker/setup-buildx-action` 起的 BuildKit 以 `driver-opts: image=moby/buildkit:<版本>@sha256:…` 钉住，演练与构建是同一个；能签名的作业不用 `docker/login-action`，以 shell 的 `docker login` 登录（P5 设计 2.1 第 17 条）。」

**第 13 条 读 tag 的那次 `release.yml`**（4.10"`ci-wait.sh` 按工作流取"）。tag 指的提交在 `main` 上已有一次演练，`ci-wait.sh <sha> release.yml` 取最后建的那一次，tag 先推时可能等到演练；tag 的运行的 `head_branch` 是 tag 名（推断，rc 时核对）。06-environment 第 4 节随之加一句（第 8 节第 6 条）。

- `13a-tag-run`（`:526`）：「读 `release.yml` 的演练时第二个参数写它。」→「读 `release.yml` 的演练时第二个参数写它。读 tag 的那次运行用 `gh run list -R open-nerve/NerveWiki --workflow release.yml --branch <tag>`：tag 指的提交在 `main` 上已有一次演练，`ci-wait.sh` 取最后建的那一次，可能取到演练（P5 设计 2.1 第 13 条）。」

**第 14 条 负责人的设置**（第 11 节"需要负责人确认"）。第 2 项补 `artifact-metadata: write` 与"演练用到的第三方 action 受限时哪个 Step 的分支 CI 会红"（审查 B 的 N3）；加两项：打开 immutable releases；GHCR 的 `open-nerve` 下没有别的凭证推过的 `nervewiki` 包（GitHub 文档："the token will not have permission to push the package if you have previously pushed a package to the same namespace"）。都在推 rc 之前。号接表尾，P2–P4 若也加行，脚本照当时的最后一行续号。

- `14a-owner-2-permissions`（`:1369`）：「`id-token: write`、`attestations: write`（I1-D19；审查 B 指出最后一项可能本来不限，照实核对）」→「`id-token: write`、`attestations: write`、`artifact-metadata: write`（I1-D19；审查 B 指出这几项可能本来不限，照实核对）」
- `14b-owner-2-when`（`:1369`）：「| 推 rc 之前（P5/S2 合并之后就可以） |」→「| 推 rc 之前（P5/S2 合并之后就可以）；演练在每次推送上用 `docker/*`（P5/S1 起）与 `anchore/sbom-action`（P5/S2 起），"允许的 actions"受限时那个 Step 的分支 CI 就红（P5 设计 2.2） |」
- `owner rows`（"需要负责人确认"的表尾，号接最后一行，`86565dd6` 上是 11、12）：「| <n> | 打开仓库的 immutable releases（P5 设计 2.1 第 14 条） | 打开：公开之后 tag 与附件都不能改，与"不重推同一个 tag"一致；流水线先草稿后公开，打开不打开都能用 | 推 rc 之前 |」
- `owner rows`（"需要负责人确认"的表尾，号接最后一行，`86565dd6` 上是 11、12）：「| <n> | GHCR 的 `open-nerve` 下没有、也不要有手动推的 `nervewiki` 包：别的凭证推过的同名包，`GITHUB_TOKEN` 推不了（GitHub 文档；P5 设计 2.1 第 14 条） | 照做：第一次由 `release.yml` 推，包自动关联仓库 | 推 rc 之前 |」

**第 15 条 发布物的名字与注解**（5.4）。附件名、索引的三个注解（P5-14），仓库里新加的文件。

- `15a-annotations`（`:685`）：「label 加 `revision`、`created`、`description`。」→「label 加 `revision`、`created`、`description`；索引带 `description`、`source`、`licenses` 三个注解，值取自 amd64 那一份的 label（GHCR 的多架构镜像从索引的注解读说明；P5 设计 2.1 第 15 条）。」
- `15b-assets`（`:689`）：「附件是镜像的 SBOM、前端的 SBOM、第三方许可证。」→「附件是 `nervewiki-<版本>-image.spdx.json`（镜像的 SBOM）、`nervewiki-<版本>-web.spdx.json`（前端的 SBOM）、`nervewiki-<版本>-licenses.tar.gz`（第三方许可证，与镜像里的 `/licenses` 相同）（P5 设计 2.1 第 15 条）。」
- `15c-files`（`:690`）：「`deploy/release-notes.sh`、」→「`deploy/release-notes.sh`、`deploy/release-version-tag.sh`、`deploy/github-release.sh`、`deploy/compose-initdb.sh`、`tools/licenses/`、」

**第 16 条 流水线自己的错不出 `v0.1.1`**（4.11）。主持者定（审查 B 的 Q2，P5-21）：`v0.1.0` 的核对因校验命令或流水线本身的错红了、产品没有问题时，修流水线、照说明里的命令手动校验、记进收尾记录；手动公开那个草稿是对外的一步，先问负责人。产品的问题照 4.11 出 `v0.1.1`。

- `16a-pipeline-errors`（`:604`）：「出 `v0.1.1`。」→「出 `v0.1.1`。流水线自己的错（例如校验的命令写错、一个 action 换了行为）而产品没有问题时，不出 `v0.1.1`：在 `m12-release` 上修流水线，照 Release 说明里的命令手动校验，记进收尾记录；手动公开留下的草稿是对外的一步，先问负责人（主持者定，P5 设计 2.1 第 16 条）。」

**第 17 条 BuildKit 钉住；能签名的作业不用 `docker/login-action`**（4.10"新加的 action"，与第 12 条同一句，标签 `12b-shas`）。`docker/setup-buildx-action` 默认按浮动的 tag 拉 BuildKit（本机 buildx v0.38.1 里是 `moby/buildkit:buildx-stable-1`，审查 B 的 M5）；执行 Dockerfile、拿推送凭证的正是它，照 13.5 第 2 条按摘要钉住，演练与构建同一个（P5-18）。`publish` 的登录改成 shell 的 `docker login`，那个作业里第三方的 action 只剩 cosign-installer（审查 B 的 M6，P5-8）。

**第 18 条 "总体设计的修订"第 6 条的新文字**（13.5 加第 6 条）。比定稿时的多了 README、`CHANGELOG.md`、actionlint 只在 runner 上、写权限只在 tag 的作业里、版本号的 tag 不改指与 Release 先草稿；S1 写前一半，S2 在末尾加后一半（审查 C 的 M6）。

- `18a-revision-6`（`:935`，整行）：「6. **13.5** 加第 6 条，随 P5/S1、S2 改：`6. **版本号与发布**（M12）：版本号只由 ldflags 注入，Makefile、buildinfo 与 Dockerfile 的默认值相同，由测试核对；发布之后改为下一个补丁版本加 \`-dev\`。发布由推 \`v*\` 的 tag 触发 \`release.yml\`，闸门只放过 \`main\` 上 \`ci.yml\` 已绿、\`make vuln\` 为绿的提交；依赖的漏洞扫描另在每天的 \`vuln.yml\` 里跑，只在 runner 上。镜像两种架构，带签名、SBOM 与来源证明，只打确切的版本号，不打 \`latest\` 与浮动的"大版本.小版本"。推 tag 之前问负责人；收尾之后为发布做的改动在 \`m12-release\` 这样的分支上，合并之后才推 tag。`」→「6. **13.5** 加第 6 条，随 P5/S1、S2 改（S1 写到"合并之后才推 tag。"为止，S2 在末尾加后一句；P5 设计 2.1 第 18 条改，比定稿时多了 README、`CHANGELOG.md`、actionlint、写权限只在 tag 的作业里、版本号的 tag 不改指与 Release 先草稿）：`6. **版本号与发布**（M12）：版本号只由 ldflags 注入，\`Makefile\`、\`buildinfo\`、Dockerfile 的两个 \`ARG\` 与 README 写的默认值相同，由测试核对；发布之后改为下一个补丁版本加 \`-dev\`。发布由推 \`vX.Y.Z\` 或 \`vX.Y.Z-rc.N\` 的 tag 触发 \`release.yml\`，闸门只放过 \`main\` 上 \`ci.yml\` 已绿、\`make vuln\` 为绿、\`CHANGELOG.md\` 有这一节的提交；依赖的漏洞扫描另在每天的 \`vuln.yml\` 里跑，actionlint 在 \`ci.yml\` 里，都只在 runner 上。写权限只在 tag 触发的作业里，演练只读，由测试读工作流核对。推 tag 之前问负责人；收尾之后为发布做的改动在 \`m12-release\` 这样的分支上，合并之后才推 tag。 镜像两种架构，带来源证明、两份 SBOM 的证明与签名，镜像里有第三方许可证；只打确切的版本号，不打 \`latest\` 与浮动的"大版本.小版本"，版本号的 tag 在证明与签名之后才打、已有的不改指；Release 先草稿，两种架构上核对过才公开。`」

**第 19 条 总体设计另有四处"README 的部署一节"**（"总体设计的修订"第 7 条加一项）。S3 把那一节缩成入口之后它们落空（审查 C 的 M8）。

- `19a-revision-7-more`（`:941` 之后加一行）：「   - 另有四处写着 `README 的部署一节`（`86565dd6` 上第 11 节的 `:923` 两处、`:929`，第 14 节的 `:1311`）：S3 把那一节缩成入口之后它们落空，改为 deploy/README（第 11 节里的写成链接 `[deploy/README.md](../../deploy/README.md)`；第 14 节的那一处不单独改，由第 9 条的 SSE 一行整句替换完成）；S3 时核对第 11 节里恰好三处、第 14 节里恰好一处（P5 设计 2.1 第 19 条，P5 设计审查 C 的 M8）。」

**第 20 条 第 14 节的 `NOTIFY` 与 SSE 两行归 P5/S3**（"总体设计的修订"第 9 条）。主持者定（审查 C 的 M7、修订核对 C 的 N14，P5-23）：两行都不是实测的事，部署文档是 P5/S3 写的；P4 的第 8 节不再改它们（同 P4 设计的 P4-26，主持者已告诉 P4 的设计者）。两行的新文字在第 8 节第 5 条。

- `20a-revision-9-notify`（`:945`）：「（设计审查 C 的 M7）。」→「（设计审查 C 的 M7）。`NOTIFY` 与 SSE 两行不是实测的事，归 P5/S3 改、P4/S3 不改它们（主持者定，P5 设计审查 C 的 M7 与修订核对 C 的 N14，同 P4 设计的 P4-26）：锚点 `升级 PostgreSQL 大版本时复核（论证见` 与 `两份配置写在 README 的部署一节，M12 的部署文档沿用。`（各恰好出现一次），新文字写复核的步骤在 deploy/README 的"升级与回退"、两份配置在 deploy/README 的"反向代理与 HTTPS"（P5 设计 2.1 第 20 条、第 8 节第 5 条）。」

**第 21 条 部署文档的数与 P4 对齐**（4.8"要求""升级与回退""容量"）。P4 的修订在批次 E 另量一组小机器档（P4 设计的 P4-27：`page.parse_budget_bytes` 5 MiB 加 `GOMEMLIMIT=1536MiB`，B2 的 k = 8），实测比推算好，小机器一档取它（主持者定，修订核对 C 的 M12）；P4 不量 `reindex` 的 WAL，沿用 M8 的数；P4 另给每 1,000 页的库的大小、树的 JSON 的压缩比与 `GOMEMLIMIT` 的建议值（`p4-draft.md` 第 9 节），容量补上这三样（审查 C 的 M11，3.18 的表）。

- `21a-small-machine`（`:479`）：「数取自 P4。」→「数取自 P4；小机器一档是 P4/S3 批次 E 另量的一组（P4 设计的 P4-27：`page.parse_budget_bytes` 5 MiB 加 `GOMEMLIMIT=1536MiB`），不是推算（P5 设计 2.1 第 21 条）。」
- `21b-reindex`（`:486`）：「`nervewiki reindex` 在 30 万页约 26 分钟、约 290 GB 的 WAL，之后表约是原来的 1.7 倍；」→「`nervewiki reindex` 在 30 万页约 26 分钟、约 290 GB 的 WAL，之后表约是原来的 1.7 倍（M8 的数，`README.md:534`，`86565dd6` 上是 `:535`，P4 不重量；P5 设计 2.1 第 21 条）；」
- `21c-capacity`（`:488`）：「10 万页的树被几个标签页重读时的 CPU 与出口）」→「10 万页的树被几个标签页重读时的 CPU 与出口、每 1,000 页的库的大小、树的 JSON 的压缩比、`GOMEMLIMIT` 的建议值；对照 P4 记录的"给部署文档"一节，P5 设计 3.18）」

**第 22 条 compose 样例的库角色、附件目录与可信的代理**（4.8"`deploy/compose.yaml`"）。照初稿的样例部署，宿主机上的代理连发布的端口，服务看到的所有客户端都是 compose 网络的网关这一个 IP，按 IP 的限流（`login_ip`、`register_ip`、`auth_failure`、`anonymous`、`asset_content`）全实例共用一个桶（审查 C 的 I1）；官方镜像的 `POSTGRES_USER` 是超级用户，服务以它连库比两个角色的写法宽得多（审查 C 的 M4）；具名卷里的 `blobs/` 在没有 shell 的镜像里备份不了，`docker compose cp` 放回的属主不对（审查 C 的 M3）。`make deploy-check` 的说法随审查 C 的 M1 改（P5-16，3.17）。

- `22a-compose-roles`（`:500`）：「  - 写明样例用一个角色还是两个；两个时 `deploy/runtime-grants.sql` 在哪一步执行。」→「  - 样例用一个不是超级用户的库角色：官方镜像的 `POSTGRES_USER` 总是超级用户，样例的初始化脚本另建 `NOSUPERUSER` 的 `nervewiki` 作库与表的所有者；两个角色的写法与 `deploy/runtime-grants.sql` 在哪一步执行写在 `deploy/README.md`。附件目录绑定到 compose 文件旁的 `./data`（属主 65532），`blobs/` 在宿主机上打包备份、放回；默认网络钉一个子网，`server.trusted_proxies` 信任它：宿主机上的代理经端口转发进来，服务看到的对端是这个网络的网关，不设时所有客户端共用按 IP 的限流（P5 设计 2.1 第 22 条，P5 设计审查 C 的 I1、M3、M4）。」
- `22b-deploy-check`（`:501`）：「它只查语法，上面那一条由演练核对。」→「它只查语法与变量（插值与 schema 本来就覆盖整份文件，`--profile migrate` 另让 `migrate` 里的引用也核对，P5 设计 3.17），上面那一条由演练核对。」

**第 23 条 移交与已知限制的去处**（4.13）。`M12-release.md` 补本文推后的几件（第 9 节）；第三方许可证里没有文本的包进已知限制、有去处（审查 C 的 M10）；GHCR 里留下的摘要清不清放 v0.2 的 `M12-release.md`，不放 v0.1.1 的（审查 C 的 N10）。

- `23a-release-handoff`（`:636`）：「多实例、独立的二进制；」→「多实例、独立的二进制、actionlint 查 `run:` 的脚本、构建的缓存、`outdated.sh` 不列的工具（actionlint、syft、cosign、BuildKit）、第三方许可证里没有文本的包、rc 与 `v0.1.0` 留在 GHCR 里的 `sha-` 与没有 tag 的摘要清不清（P5 设计 第 9 节）；」
- `23b-known-limit-licenses`（`:630` 之后加一行）：「  - 第三方许可证里没有文本的包（P5 设计 P5-10，今天三个），进 `M12-release.md`。」

**第 24 条 闸门的写法与 tag 指的提交**（4.10"闸门"、4.11 第 6 步）。tag 只认两种写法、`CHANGELOG.md` 的一节不能是空的、正式版本要写日期（P5-5）；闸门只认 tag 指的提交自己的 `ci.yml` 运行，合并提交要单独推上 `main`（审查 A 的 N9）。

- `24a-tag-forms`（`:528`）：「  - tag 名是 semver，」→「  - tag 名是 `vX.Y.Z` 或 `vX.Y.Z-rc.N`（数字不带前导的 0，别的预发布写法不收；P5 设计 P5-5），」
- `24b-changelog`（`:532`）：「  - `CHANGELOG.md` 里有这个版本的一节（rc 取去掉 `-rc.N` 之后那个版本的一节），在构建之前就核对。」→「  - `CHANGELOG.md` 里有这个版本的一节、不是空的（rc 取去掉 `-rc.N` 之后那个版本的一节；正式版本的标题要写发布的日期），在构建之前就核对：闸门照说明脚本取一次这一节（P5 设计 P5-5、3.6）。」
- `24c-step-6`（`:596`）：「合并，`main` 的 CI 绿；推 `v0.1.0`」→「合并（合并提交单独推上 `main`：闸门只认 tag 指的提交自己的 `ci.yml` 运行，P5 设计 3.23），`main` 的 CI 绿；推 `v0.1.0`」

第 11 节另加"P5 的设计审查之后的取舍"一节（2.3 的 23 条，正式编号 430–452），第 12 节的 P5 一行、第 13 节一行，见第 8 节第 4 条。

### 2.2 需要负责人确认

没有挡住 S1、S2 开工的。00 号文档第 11 节"需要负责人确认"里 P5 用到的几项，与 2.1 第 14 条新加的两项，各写"什么时候要"：

| # | 事项 | 推荐 | 什么时候要 | 00 号文档 |
|---|---|---|---|---|
| 1 | 组织允许公开的容器包；"允许的 actions"包括 `docker/login-action`、`docker/setup-buildx-action`、`docker/build-push-action`、`sigstore/cosign-installer`、`anchore/sbom-action`；组织的 Actions 设置不限制工作流申请 `packages: write`、`id-token: write`、`attestations: write`、`artifact-metadata: write` | 照做 | 推 rc 之前。演练在每次推送上用 `docker/*`（S1 起）与 `anchore/sbom-action`（S2 起）："允许的 actions"受限时，那个 Step 的分支 CI 就红，到时看得出，不必先问。**S1**：分支 CI 上 `docker/setup-buildx-action`、`docker/build-push-action` 没被挡住（`release.yml` 的 run 44073973297 起，10.1）；`docker/login-action` 只在 tag 的 `build` 里，还没跑过，它与 S2 的几个和写权限仍在推 rc 之前核对。**S2**：分支 CI 上 `anchore/sbom-action` 没被挡住（`release.yml` 的 run 44451246873 起，10.2）；`docker/login-action`、`sigstore/cosign-installer` 与写权限只在 tag 的作业里，推 rc 之前核对 | 第 2 项 |
| 2 | 打开仓库的 immutable releases | 打开：公开之后 tag 与附件都不能改，与"不重推同一个 tag"一致；流水线先草稿后公开，打开不打开都能用 | 推 rc 之前 | 新（2.1 第 14 条） |
| 3 | GHCR 的 `open-nerve` 下没有、也不要有手动推的 `nervewiki` 包 | 照做：第一次由 `release.yml` 用 `GITHUB_TOKEN` 推，包自动关联仓库 | 推 rc 之前 | 新 |
| 4 | `v*` 的 tag 加规则集，只有负责人与主持者能推 | 照做；删 `v0.1.0` 的 git tag 只在 3.23 的那一种失败里由负责人决定 | 推 rc 之前 | 第 3 项 |
| 5 | 开私密的漏洞报告 | 照做 | S3 写 `SECURITY.md` 之前 | 第 4 项 |
| 6 | 有没有 v0.1.0 之前就在跑的实例 | — | P4/S3 开始之前；S3 照回答写或不写开发版的附录 | 第 7 项 |
| 7 | 输入法清单同样在发布之后验收 | 是 | S3 写 `CHANGELOG.md` 与 `manual/README.md` 之前 | 第 8 项 |
| 8 | 推 `v0.1.0-rc.1` 与 `v0.1.0` 两个 tag | 照 4.11 | 收尾合并、`main` 的 CI 绿之后 | 第 1 项 |
| 9 | 把 GHCR 的包设为公开（不能撤回） | rc 核对之后 | 4.11 第 5 步 | 第 5 项 |

推 rc 之前，主持者先以 `gh api` 只读地看一次第 2–4 项做好了没有，读不到的请负责人回一句（3.23 第 3 步，审查 B 的 N11）。

### 2.3 本文的取舍（主持者的决定，负责人可以改判）

正式编号 430–452，主持者在设计审查之后给的（P5-k 是 429 + k）；括号里是临时号，审查记录用它。P5-6、P5-11 里"核对在公开之前"、P5-21、P5-22、P5-23 是主持者在审查之后定的，其余是设计者的取舍、照审查改过。

- **430（P5-1）发布的守卫都是 `platform/buildinfo` 的 Go 短测试**：版本号的默认值（`version_test.go`）；三份工作流的权限、触发与调的脚本，闸门、说明、版本号的 tag 与 Release 的脚本对着假的 `gh`、`docker` 与临时的仓库（`release_test.go`）。理由：P1/S2 的 Go 版本守卫已在这里（决定 347 建议版本号的守卫与它相邻）；CI 的 `short` 与 `race` 两份都跑，本机 `make test-go-short` 与 `make check` 也跑，不必新开一个 Makefile 目标；YAML 有现成的库。代价：这几个测试要 `bash`、`git`、`jq`，README 的"需要安装"加 jq（`image-smoke` 本来就要它）。脚本由测试直接执行，可执行位与 `#!` 行也核对到（审查 A 的 M5）。
- **431（P5-2）MCP 的版本**：`bootstrap/mcp.go` 把 `mcpserver.Options` 的组装拆成 `mcpOptions`，短测试断言它的 `Name`、`Version`；`make test-go-short` 以 `-X …buildinfo.version=0.0.0-short` 构建测试的二进制并设 `NWIKI_TEST_VERSION`，测试先核对盖上了；mcpserver 另有一个握手测试，`serverInfo` 是 `Options` 给的两个值。不进 e2e：e2e 本机默认的 `VERSION` 就是默认值，同样抓不到。审查 A 经 `heavy.sh` 盖了版本号跑过全部短测试，94 个包都过。
- **432（P5-3）README 写的默认值也进版本号的守卫**：同 P1/S2 的 Go 版本守卫把 README 核对进去；`instance.yaml` 的示例与 `info.go` 的注释不进：它们不是默认值。
- **433（P5-4）三个 label**：`revision` 是提交，`created` 是提交的时间（`git log -1 --format=%cI`），不是构建的时间：同一个提交重建得到同样的 label。`image-smoke.sh` 照仓库逐字核对两者（审查 A 的 N4）。`description` 用中文，同 README 的第一句。
- **434（P5-5）tag 只认两种写法**：`vX.Y.Z` 与 `vX.Y.Z-rc.N`（数字不带前导的 0），别的（`-beta`、`v0.1`）闸门拒绝。`CHANGELOG.md` 一个版本一节，标题 `## X.Y.Z（YYYY-MM-DD）`，发布之前写 `## X.Y.Z（未发布）`；rc 取 `X.Y.Z` 的一节，括号里是什么都行，正式版本的括号里要是日期；这一节不能是空的，闸门照说明脚本取一次（审查 A 的 M4）。4.11 第 6 步漏写日期时，闸门拒绝 `v0.1.0`。
- **435（P5-6）actionlint 只在 runner 上跑**（主持者定，2.1 第 10 条）：`make lint-actions`，CI 的 `costs-1` 一份；`-shellcheck=`。要查 `run:` 的脚本时另议，进 v0.2 的 `M12-release.md`。
- **436（P5-7）演练与发布都不用构建的缓存与 pnpm 的 store 缓存**：`ci.yml` 的 `image` 一项从头构建 68–80 秒，缓存省不了多少；发布的构建不读别的运行留下的缓存，少一处要信任的东西。演练只装 `@nervewiki/web` 的依赖，与 `ci.yml` 共用 store 的键会让 web、e2e 两项取回半个 store（审查 B 的 M3）。演练另断言镜像的架构是 runner 的架构（原生构建，不是仿真）。
- **437（P5-8）一个只读的 `sbom` 作业**（2.1 第 2 条）：`pnpm install`、syft 与取 `/licenses` 在 `contents: read`、`packages: read` 下跑。`publish` 里第三方的 action 只有 cosign-installer（装 cosign、核对它的校验和），登录用 shell 的 `docker login`（审查 B 的 M6）。
- **438（P5-9）SBOM 的内容**：镜像的一份由 syft 从 amd64 的那一份镜像生成（SPDX 2.3），证明挂在索引的摘要上。两种架构的二进制由同一份 `go.sum`、`CGO_ENABLED=0` 构建，Go 模块相同（审查 B 核对过两种架构的 `go list -deps` 都是同样的 47 个）；distroless 的几个包只差架构字段。前端的一份由 `pnpm --filter @nervewiki/web sbom --sbom-format spdx --prod` 生成：pnpm 12.8.1 自带，不加依赖，本机实验 235 个包，许可证从 store 读（`--lockfile-only` 时 233 个都是 NOASSERTION）。它列的是 `@nervewiki/web` 的生产依赖图，多于 vite 实际打包的，宁多不少。
- **439（P5-10）`/licenses` 在 Dockerfile 里收集**：Go 一侧在 server 阶段从模块缓存复制每个链接进二进制的模块的许可证文件，加 Go 自己的 `LICENSE`、`PATENTS`；没有许可证文件的模块让构建失败（今天 47 个都有）。npm 一侧在 web 阶段照 `pnpm licenses list --prod --json` 复制（S2 实际：`--filter "@nervewiki/web..."`，带上工作区包的依赖，另收代码随产物发出的 tailwindcss，决定 612；两侧的清单对着二进制与前端的 SBOM 核对，614）；包里没有许可证文本的照实记成"没有文本"、不失败（今天 3 个：fastdom 1.0.12、react-remove-scroll-bar 2.3.8、strictdom 1.0.1，`package.json` 都写 MIT），进已知限制与 v0.2 的 `M12-release.md`（审查 C 的 M10）。Nerve Wiki 自己的 `LICENSE` 也放进去。`make image` 与发布同一条路；Release 的附件从推上去的镜像里取。
- **440（P5-11）Release 先草稿、传附件，两种架构上核对过才公开；推荐打开 immutable releases**（2.1 第 3 条；核对在公开之前是主持者定的）。
- **441（P5-12）版本号的 tag 已在、指着别的摘要时失败，不改指**（2.1 第 4 条）。判"不在"只认 registry 答的 not found，别的错误让这一步失败（以免一时读不到就当作不在、改指）。`gh release view` 同样只认"release not found"（审查 B 的 M1）；S2 改为读 REST 的列表，任何失败都失败、多于一个也失败：gh 会把一次查找的临时错误也说成 release not found（S2 审查 C 的 M4，决定 615）。
- **442（P5-13）只签索引的摘要**，不 `--recursive` 签每个架构的那一份：`cosign verify <镜像>:<tag>` 解析到的是索引；用户按架构拉的那一份由索引的摘要盖住。
- **443（P5-14）索引带三个注解**：`description`、`source`、`licenses`，值从 amd64 那一份镜像的 label 读，不在工作流里再写一遍。构建时 `oci-mediatypes=true`（注解要 OCI 的索引）。
- **444（P5-15）rc 失败时**：核对之前失败的，留下的草稿（从没公开过）删掉；公开之后才发现问题的 rc，Release 不删，标题与说明标"作废"；GHCR 里留下的没有 tag 的摘要与 `sha-<提交>` 不删，收尾记录列出、v0.2 的移交写清不清（3.23）。
- **445（P5-16）`deploy/compose.yaml`**（审查 C 的 I1、M3、M4）：
  - 库角色：官方镜像的超级用户只在容器里用（初始化、备份、恢复），初始化脚本 `deploy/compose-initdb.sh` 另建 `NOSUPERUSER` 的 `nervewiki` 作库与表的所有者，服务与迁移都用它，`run --rm migrate` 之后不必补授权；两个角色的写法与 `runtime-grants.sql` 写在 `deploy/README.md`。`pg_trgm` 自 PostgreSQL 13 起是 trusted 的扩展，库的所有者能建：审查 C 的修订核对在 18.7 上照样例装过三遍，54 条迁移都过，`pg_trgm` 的所有者是 `nervewiki`；演练照样看这一格。
  - 附件目录绑定 compose 文件旁的 `./data`（第一次装时 `chown 65532:65532`）：`blobs/` 在宿主机上打包备份、放回、改属主；`down -v` 也不会删掉它。具名卷的那一种要借别的镜像的 `tar` 与 `chown`，命令更绕。
  - 默认网络钉一个子网，`serve` 的 `server.trusted_proxies` 信任整个子网：宿主机上的代理经端口转发进来，服务看到的对端是网关；无 root 的 Docker 与 Docker Desktop 上对端不一定是网关（审查 C 未核实的第 2 条），信任整个子网更稳，子网里只有本项目的容器。
  - 版本号与两个密码写在同目录的 `.env`，没写时 `docker compose` 直接报错、写出说明；私钥是 `secrets`。
- **446（P5-17）升级的演练从哪里升上来**（审查 C 的 M2）：M12 有新的迁移（`git log --diff-filter=A --format=%H 5e8af5b5..HEAD -- server/migrations/sql/`）时，取最早那一个的父提交，`run --rm migrate` 真的执行它们；没有时（P3 的设计不加迁移）从 `5e8af5b5`（M12 的基点）升，记录写明"没有待执行的迁移"。不取 `19b2ae6f^`：它会执行 00058、00059，但它是 M11 一个 Step 中途的状态，不是任何人会部署的版本。
- **447（P5-18）syft、cosign 与 BuildKit 的版本在工作流里显式写**：`syft-version: v1.54.0`（sbom-action v0.24.3 自带的）、`cosign-release: v3.1.4`（cosign-installer v4.1.3 的默认）、BuildKit 的 `driver-opts: image=moby/buildkit:<版本>@sha256:…`（S1 实施时取当时的稳定版，审查 B 的 M5），换 action 的版本时不会悄悄换掉工具；守卫核对演练与构建的 BuildKit 相同。
- **448（P5-19）闸门不等 CI**：tag 指的提交的 `ci.yml` 还没跑完或红着，闸门拒绝。CI 绿了之后在同一次运行上 Re-run failed jobs，不重推 tag。
- **449（P5-20）版本号的 tag 与 Release 的两步写成脚本**（审查 B 的 M2）：`deploy/release-version-tag.sh`、`deploy/github-release.sh`，对着假的 `docker`、`gh` 把每一支各跑一格；只在出错时才走的几支（"已在、别的摘要""已公开""读不到"）正常的 rc 一次也走不到，不测就等于没测（05-lessons 第 1 节）。
- **450（P5-21）`v0.1.0` 只因流水线自己的错没过核对时不出 `v0.1.1`**（主持者定，2.1 第 16 条）。
- **451（P5-22）P5/S3 只等 P4/S3 与 P5/S2**（主持者定，2.1 第 9 条）。
- **452（P5-23）总体设计第 14 节的 `NOTIFY` 与 SSE 两行归 P5/S3 改**（主持者定，2.1 第 20 条；同 P4 设计的 P4-26）。

## 3. 实现

### 3.1 S1 的文件

| 文件 | 改动 |
|---|---|
| `server/internal/platform/buildinfo/version_test.go` | 新：`TestTheDefaultVersionIsOneEverywhere`（3.2） |
| `Makefile` | `:12` 的注释；`image` 的配方合成一行，先取提交与它的时间、git 读不出就停，再传 `REVISION`、`CREATED`（3.4）；`test-go-short` 盖版本号（3.3）；新目标 `lint-actions`（3.5）；`image`、`image-smoke`、`test-go-short` 的说明（"需要 Docker 与 git"；label 与 git；`## ` 后的空格与"盖一个不是默认值的版本号"） |
| `server/internal/bootstrap/mcp.go` | `mcpServer`（`:86-93`）的 `Options` 拆成 `mcpOptions`（3.3） |
| `server/internal/bootstrap/mcp_version_test.go` | 新：`TestMCPServerInfoIsTheBuild`（3.3） |
| `server/internal/platform/httpserver/mcpserver/handshake_test.go` | 加 `TestTheHandshakeNamesTheServerAndItsVersion`（3.3） |
| `deploy/Dockerfile` | `:1-3` 的用法注释；runtime 阶段加 `ARG REVISION`、`ARG CREATED` 与三个 label（3.4） |
| `deploy/image-smoke.sh` | 核对 label（3.4）；头注释的"需要"加 git |
| `.github/workflows/ci.yml` | `:3-5` 的 `on.push` 加 `branches: ["**"]`；`costs-1` 一份加 Lint workflows 一步（3.5） |
| `deploy/release-gate.sh`、`deploy/release-notes.sh` | 新，可执行（3.6、3.7） |
| `.github/workflows/release.yml` | 新：`rehearse`、`gate`、`build`（3.8） |
| `server/internal/platform/buildinfo/release_test.go` | 新：工作流的守卫、闸门与说明的测试（3.9） |
| `README.md` | `:15-17` 的"需要安装"加"bash、git、jq 与 awk"（`make test-go-short` 里发布脚本的测试要这四样，决定 559）；`:29` 的 `make lint` 一句之后加"`make lint-actions`（actionlint 核对 `.github/workflows`）只在 CI 上跑"；`:31` 的 `make test-go-short` 一句写它盖版本号；`:508` 的 image-smoke 一行：清单加"镜像的 label"，"另需"加 git（S1 审查 A 的 N2） |

### 3.2 S1：版本号的默认值与它的守卫

`server/internal/platform/buildinfo/version_test.go`，短测试（不要数据库，`-short` 照跑），文件从 `../../../../` 读，同 `toolchain_test.go:19`：

```go
// The version a build gets when nobody stamps one is the same everywhere it is written (00-M12 4.9,
// M12/P5/S1): Makefile's default, buildinfo's, both of deploy/Dockerfile's ARGs, and the one README
// tells contributors. A direct `docker build` takes the Dockerfile's ARG; make passes its own; a test
// binary has buildinfo's. The source is read, not the variable: make test-go-short stamps the variable.
func TestTheDefaultVersionIsOneEverywhere(t *testing.T)
```

它有自己的帮手 `matches(t, file, re string, n int) []string`：读文件、要求正则恰好匹配 `n` 次，不符时写出文件、找到的每一处与行号，返回各处的第一个子匹配。`toolchain_test.go:27-48` 的 `only` 是那个测试函数里的闭包、只认"恰一个"，不照抄（审查 A 的 N8）。断言：

1. `server/internal/platform/buildinfo/buildinfo.go`：`(?m)^var version = "([^"]+)"$` 恰一个，记作 `want`；
2. `Makefile`：`(?m)^VERSION \?= (\S+)$` 恰一个，等于 `want`（`TEST_VERSION := …` 不会被它认成默认值）；
3. `deploy/Dockerfile`：`(?m)^ARG VERSION=(\S+)$` 恰两个（server 与 runtime 两个阶段），都等于 `want`；
4. `README.md`：``版本号取 `VERSION`，默认 `([^`]+)` `` 恰一个，等于 `want`。

读源码而不读变量：3.3 让 `make test-go-short` 盖上 `0.0.0-short`，那时变量不是默认值。审查 A 以一个只记日志的临时测试核对过四个正则在 `86565dd6` 上的匹配数与值。

`Makefile:12` 的注释改为"默认值与 `buildinfo`、`deploy/Dockerfile` 的两个 `ARG`、README 写的相同，由 `server/internal/platform/buildinfo/version_test.go` 核对；发布时由 tag 经 ldflags 注入（`release.yml`）"。

不去掉 Dockerfile 的 `ARG` 默认值（00 号文档 4.9）。

### 3.3 S1：MCP 的版本

**拆出 `mcpOptions`**（`bootstrap/mcp.go:82-93`）：

```go
// mcpOptions are the MCP server's options: tools and prompts, the personal access tokens of the per-route
// middlewares' authentication, the notebook entry and the account entry's instructions, the client names on
// the tokens, each call's work within server.request_timeout, and a serverInfo that names nervewiki and the
// build's version, the one /api/v0/instance and `nervewiki version` tell (00-M12 4.9, M12/P5/S1).
func mcpOptions(cfg config.Config, logger *slog.Logger, errs httpserver.APIErrors, notebooks mcpserver.Notebooks,
	names mcpserver.ClientNames, tools []mcpserver.Tool, prompts []mcpserver.Prompt) mcpserver.Options

// mcpServer is the MCP server of both entries, page's notebook entry among its options.
func mcpServer(cfg config.Config, logger *slog.Logger, api *httpserver.API, pg *page.Module, names mcpserver.ClientNames,
	tools []mcpserver.Tool, prompts []mcpserver.Prompt) *mcpserver.Server {
	return mcpserver.New(mcpOptions(cfg, logger, api.Errors, pg.MCPEntry(), names, tools, prompts))
}
```

`mcpServer` 的签名不变：`wire.go:324` 与 `batch_cost_test.go:63` 照旧调它；`mcp_virtual_test.go:272` 自己组 `Options`，不受影响。`httpserver.APIErrors` 是结构体、`mcpserver.Notebooks` 是接口，测试里传零值与 `nil` 能编译（审查 A 核对过）。

**bootstrap 的短测试** `server/internal/bootstrap/mcp_version_test.go`：

```go
// The MCP server's serverInfo is nervewiki and the build's version (00-M12 4.9, M12/P5/S1). make
// test-go-short stamps its test binaries with a version that is not the default and says which in
// NWIKI_TEST_VERSION: then a serverInfo written as the default, not read from buildinfo, differs.
func TestMCPServerInfoIsTheBuild(t *testing.T)
```

- `o := mcpOptions(config.Config{}, slog.New(slog.DiscardHandler), httpserver.APIErrors{}, nil, nil, nil, nil)`；`o.Name` 是 `"nervewiki"`，`o.Version` 等于 `buildinfo.Get().Version`。
- `NWIKI_TEST_VERSION` 设了时，`buildinfo.Get().Version` 等于它（盖上了），并且不等于 `buildinfo.go` 源码里的默认值（用 3.2 同一个正则读）。没设时（直接 `go test`）只做第一条，日志写一句"没有盖版本号，写死默认值的错看不出"。`NWIKI_TEST_VERSION` 没有 `__`，配置的 `envKey` 跳过它（`platform/config/load.go:137-145`），先例 `NWIKI_TEST_SIGNAL_CHILD`。
- 名字以 `TestMCP` 起头，不落进第一、二、三、五、六、七、八份的模式（`Makefile:272-365`，审查 A 逐个对过模式与各份的 `-run`、`-skip`），在组合根的第四份（补集）跑，不到 1 毫秒，不要数据库。

**`make test-go-short`**（`Makefile:593-600`）：

```make
# 测试的二进制盖一个不是默认值的版本号：MCP 的 serverInfo 若写死成默认值，TestMCPServerInfoIsTheBuild 就失败（M12/P5/S1）。
# -X 只在链接时起作用；没有链接 buildinfo 的测试二进制照旧
TEST_VERSION := 0.0.0-short
...
	cd server && NWIKI_TEST_VERSION=$(TEST_VERSION) go test -short -count=1 \
		-ldflags "-X github.com/open-nerve/NerveWiki/server/internal/platform/buildinfo.version=$(TEST_VERSION)" ./...
```

受影响的测试逐个看过：`buildinfo_test.go:44-48` 比较 `Get()` 与变量，照过；`instance/adapter/buildinfo/source_test.go:19` 比较 `buildinfo.Get()`，照过；`cmd/nervewiki/main_test.go:31` 用 `\S+`、`bootstrap/api_test.go:64` 与 `Get()` 比，照过（审查 A 的 N10）；`apitest_test.go:19`、`:110` 是独立的字面值。审查 A 经 `heavy.sh` 原样与盖了版本号各跑一遍 `go test -short -count=1 ./...`，94 个包都过，盖过的二进制里有 `0.0.0-short`。`README.md:31` 的"即 `cd server && go test -short ./...`"改为"它以 `0.0.0-short` 盖测试的版本号；直接 `cd server && go test -short ./...` 也能跑，只是看不出 MCP 写死了版本号"。

**mcpserver 的握手测试**（`mcpserver/handshake_test.go`，用 `harness_test.go` 的 `serveMCP`）：

```go
// A handshake's serverInfo is the Name and Version the server was given, on either entry and through the Go
// client (M12/P5/S1): bootstrap gives it buildinfo's version.
func TestTheHandshakeNamesTheServerAndItsVersion(t *testing.T)
```

`setup.options` 设 `Name: "nervewiki-handshake"`、`Version: "9.8.7-handshake"`（与夹具的 `"nervewiki"`、`"test"` 不同，也互不相同）。两个入口的 `initialize` 的 `result.serverInfo.name`、`.version` 各等于它们；`s.connect(…).InitializeResult().ServerInfo` 同样。

**反向对照**：7.2 的 m1–m4。

### 3.4 S1：镜像的 label 与 `make image`

**Dockerfile**：`:1-3` 的用法注释改为

```dockerfile
# Nerve Wiki 的镜像（docs/v0.1/M0-foundation/06-P6-e2e-delivery.md 3.5）。构建上下文是仓库根目录：
#   docker build -f deploy/Dockerfile --build-arg VERSION=0.1.0 \
#     --build-arg REVISION="$(git rev-parse HEAD)" --build-arg CREATED="$(git log -1 --format=%cI)" .   （make image）
```

runtime 阶段（`:44-49`）：

```dockerfile
FROM gcr.io/distroless/static-debian13:latest@sha256:1a64f46364620465ef9e9a8f5413757dfdf22695e050bbfd7303eeaf17a59383 AS runtime
ARG VERSION=0.1.0-dev
# 提交与它的时间（git log -1 --format=%cI），不是构建的时间：同一个提交重建得到同样的 label。
# make image 与 release.yml 传它们；直接 docker build 不传时是空的，image-smoke.sh 会失败
ARG REVISION
ARG CREATED
LABEL org.opencontainers.image.title="Nerve Wiki" \
      org.opencontainers.image.description="自托管的团队在线笔记：工作区、笔记本与层级的 Markdown 页面，内置 MCP" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.created="${CREATED}" \
      org.opencontainers.image.source="https://github.com/open-nerve/NerveWiki" \
      org.opencontainers.image.licenses="AGPL-3.0-only"
```

**`make image`**（`Makefile:87-90`），判断与构建写在同一个 shell 行（第 1 节）：

```make
image: ## 构建镜像 $(IMAGE)，版本号取 VERSION，label 带提交与它的时间（需要 Docker 与 git）
	test -d .git || { echo "make image 要在普通的克隆中运行：…"; exit 1; }; \
	revision=$$(git rev-parse HEAD) && created=$$(git log -1 --format=%cI) || \
		{ echo "make image：git 读不出提交与它的时间，镜像的 label 要它们"; exit 1; }; \
	docker build -f deploy/Dockerfile --build-arg VERSION=$(VERSION) \
		--build-arg REVISION="$$revision" --build-arg CREATED="$$created" \
		-t $(IMAGE) .
```

先取提交与它的时间，git 读不出（例如 dubious ownership）就停下（S1 审查 A 的 N1，`bc71d5fe`）：命令替换失败不会让一行失败，原来的写法以 0 退出、镜像带着空的 label 构建出来。`make -i` 也绕不过：`exit 1` 在同一个 shell 行里。`image-smoke.sh` 的头注释"需要"加 git（S1 核对第 1 轮 A 的 N4）。

**`image-smoke.sh`**：`:78` 的版本核对之后（那时镜像已拉取）加：

```bash
# label：版本号、提交与它的时间逐字相同，说明不空（M12/P5/S1）。make image 与 release.yml 传入提交与时间
created=$(git -C "$repo" log -1 --format=%cI)
labels=$(docker image inspect -f '{{json .Config.Labels}}' "$image")
jq -e --arg version "$version" --arg commit "$commit" --arg created "$created" \
  '."org.opencontainers.image.version" == $version and ."org.opencontainers.image.revision" == $commit
   and ."org.opencontainers.image.created" == $created
   and (."org.opencontainers.image.description" | length > 0)
   and ."org.opencontainers.image.source" == "https://github.com/open-nerve/NerveWiki"' \
  <<<"$labels" >/dev/null || fail "镜像的 label 与预期不符：$labels"
```

没有 `created` 时 `jq` 以 5 退出，`|| fail` 接住（审查 A 核对过）。`ci.yml` 的 `image` 一项照旧 `make image-smoke`，它的 `VERSION` 是 `0.0.0-ci.<run>`（`ci.yml:326`）。

### 3.5 S1：`ci.yml` 只在分支上跑；actionlint 只在 runner 上

**`ci.yml:3-5`**：

```yaml
on:
  # 只在分支上：推 tag 时发布的闸门只认 tag 指的提交在 main 上已有的那次运行（release.yml），不再跑一遍十九项（M12/P5/S1）
  push:
    branches: ["**"]
  pull_request:
```

`ci-wait.sh` 按工作流取，不受影响。守卫（3.9）核对 `ci.yml` 的 `on.push` 只有 `branches: ["**"]`。

**`make lint-actions`**（主持者照决定 308 的先例定，2.1 第 10 条）。`Makefile` 在 `make vuln` 一组之后：

```make
# 三份工作流的写法（00-M12 4.10、M12/P5/S1）：actionlint 至少 1.7.10（artifact-metadata 的权限），go run 钉版本，
# 不进 server/tools/go.mod（同 govulncheck）。只在 GitHub 的 runner 上跑（ci.yml 的 costs-1 一份），本机拒绝：设计时在本机
# 跑它被权限分类器拒过，照 make vuln 的先例（M12 决定 308）。判断与干活写在同一个 shell 行里（make -i 绕不过）。
# -shellcheck=：runner 装了版本不由我们定的 shellcheck，actionlint 看到它才查 run:。不给文件：actionlint 在仓库根找
# .github/workflows 下的 .yml 与 .yaml。出错写成 GitHub 的注解：作业日志要认证，注解是公开的。-format 里的 \n 由 actionlint
# 自己换成换行（它的 unescapeBackslash）：写进模板的 {{"\n"}} 因此成了没结束的字符串，模板解析不了
ACTIONLINT_VERSION := v1.7.12
runner-only = test "$$GITHUB_ACTIONS" = true || { echo "make $@ 只在 GitHub 的 runner 上跑：结论看 ci.yml 的 costs-1 一份（M12/P5/S1）"; exit 1; }

.PHONY: lint-actions
lint-actions: ## actionlint 核对 .github/workflows；只在 CI 上跑
	@$(runner-only); \
	go run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION) -shellcheck= -verbose \
		-format '{{range $$e := .}}::error file={{$$e.Filepath}},line={{$$e.Line}},col={{$$e.Column}}::{{$$e.Message}}\n{{end}}'
```

- 不另用 `$(ci-only)`：它的提示说"把依赖的清单发给外部的服务、结论看 `vuln.yml`"，对 actionlint 不对。
- 在仓库根执行：那里没有 `go.mod`，`go run …@版本` 照常；actionlint 在当前目录找 `.github/workflows`。`-verbose` 在作业日志里写出读了哪几份文件，S1 的记录核对是三份（审查 A 的 N5：`.yaml` 也算）。
- `ci.yml` 的 `costs-1` 一份，在 Lint 一步（`:169-171`）之后：

  ```yaml
      # 三份工作流的写法：只在 runner 上跑，本机的 make lint 不含它（M12/P5/S1）
      - name: Lint workflows
        if: matrix.leg == 'costs-1'
        run: make lint-actions
  ```

  那一份不取 Go 的缓存（决定 355），每次下载并编译 actionlint，推断多 15–40 秒，Lint 一步今天 92–143 秒、作业时限 45 分钟。S1 实测这一步 2–3 秒：`go run` 的下载加编译在 Lint 一步之后，构建缓存已热（S1 审查 C；10.1）。
- **本机的 `make lint`、`make check`、逐个提交的门禁都不含它**；实施者与审查者都不在本机跑 actionlint。
- **现有两份工作流过不过 actionlint，设计时不知道**：S1 的第 5 个提交加这个目标与这一步，推上分支之后读 `costs-1` 的 Lint workflows 一步的注解；报出的在下一个提交里改（范围写 `build`），写进 S1 的偏离。第 5 个提交本身在 CI 的这一步上可能是红的：这一步只在 runner 上，本机的逐个提交的门禁不含它，记录写明。S1 实际：报出的是 `make lint-actions` 自己的 `-format`（run 44068674790，"unterminated quoted string"），不是现有的工作流；下一个提交 `2db96b0a` 改成上面裸的 `\n`，之后三份 0 错，没有"the workflows pass actionlint"那个提交（决定 557）。

### 3.6 S1：闸门 `deploy/release-gate.sh`

可执行，`set -uo pipefail`，照 bash 3.2 写（不用 `mapfile`、关联数组）。全文：

```bash
#!/usr/bin/env bash
# 发布的闸门（00-M12 4.10、M12/P5/S1）。推 tag 时 release.yml 在构建之前先跑它，在仓库根执行：
#   deploy/release-gate.sh <tag>
# 依次核对，一条不过就以 1 退出并写明是哪一条：
#   1. tag 是 vX.Y.Z 或 vX.Y.Z-rc.N，数字没有前导的 0；
#   2. tag 指的提交，与触发这次运行的 GITHUB_SHA 一致（设了时）；
#   3. 它在 origin/main 上（检出要 fetch-depth: 0）；
#   4. CHANGELOG.md 恰有一个"## X.Y.Z（…）"的标题（rc 取去掉 -rc.N 的那一个）；正式版本的括号里要是日期 YYYY-MM-DD；
#      这一节照 deploy/release-notes.sh 取得出、不是空的；
#   5. 这个提交在 main 上由 push 触发的 ci.yml 的运行已成功：只查 ci.yml，取最后建的那一次，它的结论是最后一次尝试的；
#      release.yml 的演练绿了不算。还在跑的也拒绝：CI 绿了之后在同一次运行上重跑这个作业。
# 都过时打出 version=、prerelease=、sha= 三行，设了 GITHUB_OUTPUT 时也写进它。make vuln（npm 一侧照 high）是工作流的
# 下一步：它只在 runner 上跑，这个脚本要能在本机对着假的输入跑。
# 要 git、jq、awk、gh（GH_TOKEN 要 actions: read）、GITHUB_REPOSITORY 与 IMAGE（第 4 条取一节时用）。
# server/internal/platform/buildinfo/release_test.go 用假的 gh 与临时的仓库核对每一条。
set -uo pipefail

fail() {
  echo "release-gate: $*" >&2
  exit 1
}

[ $# -eq 1 ] || fail "用法：deploy/release-gate.sh <tag>"
tag=$1
repo=${GITHUB_REPOSITORY:-}
[ -n "$repo" ] || fail "要 GITHUB_REPOSITORY（owner/repo）"
[ -n "${IMAGE:-}" ] || fail "要 IMAGE（镜像名，不带 tag）"

# 1. 写法
[[ $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$ ]] ||
  fail "tag \"$tag\" 不是 vX.Y.Z 或 vX.Y.Z-rc.N"
version=${tag#v}
base=${version%%-rc.*}
prerelease=false
[ "$version" = "$base" ] || prerelease=true

# 2. 提交
sha=$(git rev-parse --verify --quiet "refs/tags/$tag^{commit}") || fail "本地没有 tag $tag"
if [ -n "${GITHUB_SHA:-}" ]; then
  pushed=$(git rev-parse --verify --quiet "$GITHUB_SHA^{commit}") || fail "本地没有 GITHUB_SHA $GITHUB_SHA"
  [ "$pushed" = "$sha" ] || fail "tag $tag 指 $sha，触发这次运行的是 $GITHUB_SHA"
fi

# 3. 在 main 上
git rev-parse --verify --quiet refs/remotes/origin/main >/dev/null || fail "没有 origin/main：检出要 fetch-depth: 0"
git merge-base --is-ancestor "$sha" refs/remotes/origin/main || fail "提交 $sha 不在 origin/main 上"

# 4. CHANGELOG.md 的一节。按字面比，同 release-notes.sh：版本号里的 . 不当通配
[ -f CHANGELOG.md ] || fail "没有 CHANGELOG.md"
heads=$(awk -v h="## $base（" 'index($0, h) == 1 && /）$/' CHANGELOG.md)
count=$(printf '%s' "$heads" | grep -c '^')
[ "$count" = 1 ] || fail "CHANGELOG.md 里\"## $base（…）\"的标题有 $count 个，要恰好一个"
if [ "$prerelease" = false ]; then
  [[ $heads =~ （[0-9]{4}-[0-9]{2}-[0-9]{2}）$ ]] ||
    fail "正式版本 $version 的标题要写发布的日期：## $base（YYYY-MM-DD）"
fi
"$(dirname "$0")/release-notes.sh" "$tag" "sha256:$(printf '0%.0s' {1..64})" >/dev/null ||
  fail "CHANGELOG.md 里 $base 的一节取不出或是空的（见上一行）"

# 5. ci.yml 在 main 上的运行
runs=$(gh api "repos/$repo/actions/workflows/ci.yml/runs?head_sha=$sha&branch=main&event=push&per_page=100") ||
  fail "读不到 ci.yml 在 main 上的运行"
run=$(jq -c --arg sha "$sha" '[.workflow_runs[]
    | select(.path == ".github/workflows/ci.yml" and .head_sha == $sha and .head_branch == "main" and .event == "push")]
  | sort_by(.created_at) | last // empty' <<<"$runs") || fail "ci.yml 的运行读不懂：$(head -c 300 <<<"$runs")"
[ -n "$run" ] || fail "提交 $sha 在 main 上没有 push 触发的 ci.yml 的运行：tag 要打在单独推上 main、有自己的 CI 运行的提交上"
read -r status conclusion id attempt <<<"$(jq -r '"\(.status) \(.conclusion) \(.id) \(.run_attempt)"' <<<"$run")"
[ "$status" = completed ] && [ "$conclusion" = success ] ||
  fail "ci.yml 的运行 $id（第 $attempt 次尝试）是 $status $conclusion，要 completed success"

printf 'version=%s\nprerelease=%s\nsha=%s\n' "$version" "$prerelease" "$sha"
if [ -n "${GITHUB_OUTPUT:-}" ]; then
  printf 'version=%s\nprerelease=%s\nsha=%s\n' "$version" "$prerelease" "$sha" >>"$GITHUB_OUTPUT"
fi
```

- **字段是公开接口的真样子**：`/actions/workflows/ci.yml/runs?head_sha=86565dd6…&branch=main&event=push` 答 `total_count` 与 `workflow_runs`，每项有 `path`、`head_sha`、`head_branch`、`event`、`status`、`conclusion`、`run_attempt`、`created_at`（`m12/p5-design/ci-runs.json`）。重跑过的 43745380462 的运行对象是 `run_attempt: 2`、`conclusion: success`，它的 `/attempts/1` 是 `failure`：运行对象的结论就是最后一次尝试的。接口按建的时间从新到旧给；脚本自己排序，不依赖次序。`select` 再核一遍 `path` 与分支：假的 `gh` 答错端点时（7.2 的 g4）照样拒绝。
- **第 4 条按字面比**（审查 A 的 M2）：`awk` 的 `index` 与说明脚本取一节的写法相同，`0.1.0` 里的 `.` 不是通配，也不必在 bash 里转义（原稿 `${base//./\\.}` 在 bash 3.2 下的结果没人核对过）。日期的正则只用在取出的那一行上，`（`、`）` 是字面的多字节字符，C 与 C.UTF-8 下一样（审查 A 核对过两个 `grep -E` 与 `awk` 在 C、POSIX、C.UTF-8 与 mawk、gawk 下数得一样）。
- **日期只核对 `YYYY-MM-DD` 的形状**（`2026-99-99` 放行；S1 审查 B 的 N2，决定 563）：它挡的是忘了写日期；日期由主持者在 3.23 第 7 步照当天写。
- **第 4 条取一次这一节**（审查 A 的 M4）：原稿只数标题，标题下面是空的也放行，之后构建、推送、签名、复制出版本号的 tag 都做完，`draft` 作业的说明脚本才拒；`v0.1.0` 那时包已公开，说明取自 tag 指的提交，重跑改不了。闸门用一个全零的假摘要跑一次说明脚本，与真的那一次走同一段代码。
- **第 5 条的报错写明 tag 要打在哪里**（审查 A 的 N9）：合并提交若与"记合并号"的文档提交一起推上 `main`，它就没有自己的运行。
- `GITHUB_SHA` 经 `^{commit}` 解析：tag 的推送里它是提交（推断），万一是 tag 对象也解得到同一个提交；比较的是 tag 指的提交，不是 tag 对象（7.2 的 g11）。
- 名字里不插 `${{ … }}`：工作流以环境变量 `$GITHUB_REF_NAME` 传 tag（3.8），不把 tag 名拼进 `run:` 的文本。

### 3.7 S1：说明 `deploy/release-notes.sh`

```bash
#!/usr/bin/env bash
# 发布说明（00-M12 4.10、M12/P5/S1）。release.yml 建 Release 的草稿时在仓库根执行，说明写到标准输出：
#   deploy/release-notes.sh <tag> <索引的摘要>
# 取 CHANGELOG.md 里这个版本的一节（rc 取去掉 -rc.N 的那一节，前面加一句"预发布"），后面接镜像、摘要、两条校验的
# 命令与附件。tag 的写法不对、摘要不是 sha256:<64 位十六进制>、没有这一节或它是空的，就以 1 退出。闸门也调它一次。
# 要 IMAGE（镜像名，不带 tag）与 GITHUB_REPOSITORY。server/internal/platform/buildinfo/release_test.go 对着假的 CHANGELOG.md 核对。
set -uo pipefail
```

做的事：

1. 参数恰两个；tag 照 3.6 第 1 条的正则；摘要 `^sha256:[0-9a-f]{64}$`；`IMAGE`、`GITHUB_REPOSITORY` 不空。
2. 一节：先判有没有以 `## $base（` 起头、以 `）` 结尾的一行（`awk -v h="## $base（" 'index($0, h) == 1 && /）$/ { found = 1 } END { exit !found }' CHANGELOG.md`），没有就拒绝；再取 `awk -v h="## $base（" 'on && /^## / { exit } index($0, h) == 1 && /）$/ { on = 1; next } on { print }' CHANGELOG.md`，去掉开头与结尾的空行（脚本在 awk 的 `END` 里去）；去掉空白之后什么也没有就拒绝。说明认的标题与闸门数的是同一种（以"）"结尾）：同一个版本写坏了的标题在前不是起点，在后结束这一节（"下一个 `## `"先判；S1 审查 B 的 N1、核对第 1 轮 B 的 N5，决定 558）。`index` 比字节，`## 0.1.0（` 不会匹配 `## 0.1.00（`，也不在 `### ` 处停（审查 A 核对过 mawk、gawk 与两种 locale）。
3. rc 时先写一句：`这是预发布版本 v0.1.0-rc.1，供核对与试用；正式版本是 v0.1.0。`，空一行。
4. 一节的正文。
5. 生成的部分（heredoc，`<版本>` 是 tag 去掉 `v`，`<tag>` 原样，镜像名与仓库取自 `IMAGE`、`GITHUB_REPOSITORY`）：

   ````markdown
   ---

   ## 镜像

   `linux/amd64` 与 `linux/arm64` 两种架构，只有确切的版本号，没有 `latest`：

   ```bash
   docker pull ghcr.io/open-nerve/nervewiki:<版本>
   ```

   索引的摘要：`sha256:…`。部署、升级与备份照 [deploy/README.md](https://github.com/open-nerve/NerveWiki/blob/<tag>/deploy/README.md)。

   ## 校验

   来源证明（GitHub 的 artifact attestation，`gh` 要先 `gh auth login`）与签名（Sigstore，keyless，用 cosign v3；流水线用 v3.1.4）：

   ```bash
   gh attestation verify oci://ghcr.io/open-nerve/nervewiki:<版本> -R open-nerve/NerveWiki \
     --cert-identity https://github.com/open-nerve/NerveWiki/.github/workflows/release.yml@refs/tags/<tag>
   cosign verify ghcr.io/open-nerve/nervewiki:<版本> \
     --certificate-identity=https://github.com/open-nerve/NerveWiki/.github/workflows/release.yml@refs/tags/<tag> \
     --certificate-oidc-issuer=https://token.actions.githubusercontent.com
   ```

   两份 SBOM 的证明：第一条加 `--predicate-type https://spdx.dev/Document/v2.3`。

   ## 附件

   - `nervewiki-<版本>-image.spdx.json`：镜像的 SBOM（SPDX 2.3，syft 从 amd64 的镜像生成，含二进制里的 Go 模块）；
   - `nervewiki-<版本>-web.spdx.json`：内嵌的前端的生产依赖（SPDX 2.3）；
   - `nervewiki-<版本>-licenses.tar.gz`：第三方许可证，与镜像里的 `/licenses` 相同。
   ````

两条校验的身份逐字相同，都钉住这个 tag 上的 `release.yml`（审查 B 的 M4）：只带 `-R` 时仓库里任何工作流、任何 ref 做的证明都算数。`verify` 作业用同一个身份（3.14）。

### 3.8 S1：`release.yml` 的演练、闸门与两种架构的构建

S1 的 `.github/workflows/release.yml`（S2 加的部分在 3.14）：

```yaml
name: Release

# 发布（00-M12 4.10、M12/P5）。推 vX.Y.Z 或 vX.Y.Z-rc.N 的 tag：闸门 → 两种架构各自构建、按摘要推送、拉回来 smoke → …（S2）。
# 分支与 main 的每次推送只走演练：两种原生 runner 各自构建（不推送）、smoke，只读。写权限只在 tag 的作业里（移交 M0-P6 第 1 条），
# server/internal/platform/buildinfo/release_test.go 读这份文件核对每个作业的权限、if 与调的脚本。推 tag 之前问负责人（00-M12 4.11）。
# 读 tag 的那次运行：gh run list --workflow release.yml --branch <tag>；ci-wait.sh <sha> release.yml 可能取到 main 上的演练
on:
  push:
    # 两样都写：只写 tags 时分支的推送不触发，只写 branches 时 tag 的推送不触发；tag 的推送本来就不看 paths
    branches: ["**"]
    tags: ["v[0-9]+.[0-9]+.[0-9]+*"]

permissions: {}

# tag 的运行不取消：发布做到一半被取消，只留下一半。分支上新的推送取消较早的演练；main 的每次运行各占一组（同 ci.yml）
concurrency:
  group: ${{ github.ref_type == 'tag' && format('release-{0}', github.ref) || (github.ref == 'refs/heads/main' && format('rehearse-main-{0}', github.run_id) || format('rehearse-{0}', github.ref)) }}
  cancel-in-progress: ${{ github.ref_type != 'tag' }}

env:
  # 仓库名有大写，镜像名必须小写（00-M12 4.10）
  IMAGE: ghcr.io/open-nerve/nervewiki
  # build-push-action 默认把构建记录传成 artifact，这里不留
  DOCKER_BUILD_RECORD_UPLOAD: "false"

jobs:
  # 演练：两种原生 runner 各自构建（不推送）、smoke。Dockerfile 的 web 阶段在目标架构上 pnpm install 与构建，有按架构选的原生
  # 可选依赖：只在 arm64 上坏的构建在合并时就看得到（00-M12 4.10）
  rehearse:
    name: rehearse (${{ matrix.arch }})
    if: github.ref_type != 'tag'
    permissions:
      contents: read
    strategy:
      fail-fast: false
      matrix:
        include:
          - arch: amd64
            runner: ubuntu-24.04
          - arch: arm64
            runner: ubuntu-24.04-arm
    runs-on: ${{ matrix.runner }}
    timeout-minutes: 30
    env:
      # 与默认值不同：image-smoke.sh 由此确认版本号是注入的
      VERSION: 0.0.0-rehearsal.${{ github.run_number }}
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          # .git 进入镜像的构建阶段（取提交信息），其中不能有令牌
          persist-credentials: false
      # image-smoke.sh 照工作区算期望的 modified：工作区要干净，二进制才一定是 modified=false（设计审查 B 的 N5）
      - name: The tree is clean
        run: test -z "$(git status --porcelain)" || { git status --short; exit 1; }
      # image-smoke.sh 要的工具；arm 的 runner 的软件表没有列 openssl
      - name: Tools
        run: |
          uname -m
          for t in docker curl jq openssl git; do command -v "$t" >/dev/null || { echo "缺 $t"; exit 1; }; done
          openssl version
      # BuildKit 按摘要钉住，演练与 build 是同一个（M12/P5/S1；守卫核对两处相同）
      - uses: docker/setup-buildx-action@f87e5991a6d7451dcb8d9637bfbc97413f497069 # v4.4.1
        with:
          driver-opts: image=moby/buildkit:v0.34.2@sha256:c5f8862f52499773af85bc07695b2379ef08e8002352b0df2c4864c91d78dabe
      # 提交与它的时间，同 make image（M12/P5/S1）
      - name: Labels
        id: labels
        run: |
          echo "revision=$(git rev-parse HEAD)" >> "$GITHUB_OUTPUT"
          echo "created=$(git log -1 --format=%cI)" >> "$GITHUB_OUTPUT"
      # 路径上下文：build-push-action 默认的 Git 上下文不带 .git，二进制就没有提交信息（00-M12 4.10）
      - name: Build
        uses: docker/build-push-action@c3c9e263c25d99ce0380d002d59b67737d91b0dc # v7.4.0
        with:
          context: .
          file: deploy/Dockerfile
          load: true
          push: false
          provenance: false
          sbom: false
          tags: nervewiki:rehearsal
          build-args: |
            VERSION=${{ env.VERSION }}
            REVISION=${{ steps.labels.outputs.revision }}
            CREATED=${{ steps.labels.outputs.created }}
      # 原生构建，不是仿真
      - name: The image is the runner's architecture
        run: test "$(docker image inspect -f '{{.Architecture}}' nervewiki:rehearsal)" = "${{ matrix.arch }}"
      - name: Image smoke
        run: deploy/image-smoke.sh nervewiki:rehearsal "$VERSION"

  # 闸门（00-M12 4.10）：tag 的写法、提交在 main 上、它的 ci.yml 已绿、CHANGELOG.md 有这一节；再 make vuln，npm 一侧照 high
  gate:
    if: github.ref_type == 'tag'
    runs-on: ubuntu-24.04
    timeout-minutes: 20
    permissions:
      contents: read
      actions: read
    outputs:
      version: ${{ steps.gate.outputs.version }}
      prerelease: ${{ steps.gate.outputs.prerelease }}
      sha: ${{ steps.gate.outputs.sha }}
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          # origin/main 与 tag 都要在
          fetch-depth: 0
          persist-credentials: false
      - name: Gate
        id: gate
        env:
          GH_TOKEN: ${{ github.token }}
        run: deploy/release-gate.sh "$GITHUB_REF_NAME"
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: server/go.mod
          cache: false
      - uses: actions/setup-node@949feb2413d6458794dcd2491c4babbbce0c15c1 # v7.1.0
        with:
          node-version-file: .node-version
      - name: Enable corepack
        run: corepack enable
      # 挡发布的是 high 及以上；vuln.yml 每天照 low 报给人看（00-M12 4.3）
      - name: Vulnerabilities
        run: make vuln VULN_AUDIT_LEVEL=high

  # 每种架构在原生 runner 上构建、按摘要推送（不打 tag），再拉回刚推的摘要来 smoke：smoke 过的就是推上去的；
  # 失败时 GHCR 里只留一个没有 tag 的摘要（00-M12 4.10）
  build:
    name: build (${{ matrix.arch }})
    needs: gate
    if: github.ref_type == 'tag'
    permissions:
      contents: read
      packages: write
    strategy:
      fail-fast: false
      matrix:
        include:
          - arch: amd64
            runner: ubuntu-24.04
          - arch: arm64
            runner: ubuntu-24.04-arm
    runs-on: ${{ matrix.runner }}
    timeout-minutes: 30
    env:
      VERSION: ${{ needs.gate.outputs.version }}
      SHA: ${{ needs.gate.outputs.sha }}   # S2，决定 561
    steps:
      - uses: actions/checkout@… # v7.0.1（同上，persist-credentials: false）
      - name: The tree is clean
        run: （同演练）
      # 构建的就是闸门放过的提交：检出取 GITHUB_SHA 是推断（S2，决定 561；3.25 第 7 条）
      - name: The commit is the gate's
        run: test "$(git rev-parse HEAD)" = "$SHA" || { echo "检出的是 $(git rev-parse HEAD)，闸门放过的是 $SHA"; exit 1; }
      - name: Tools
        run: （同演练）
      - uses: docker/setup-buildx-action@f87e5991a6d7451dcb8d9637bfbc97413f497069 # v4.4.1
        with:
          driver-opts: image=moby/buildkit:v0.34.2@sha256:c5f8862f52499773af85bc07695b2379ef08e8002352b0df2c4864c91d78dabe   # 与演练逐字相同
      - uses: docker/login-action@8aa5831fb8d48d8f76594c7ddde61ae58019f550 # v4.8.0
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ github.token }}
      - name: Labels
        id: labels
        run: （同演练）
      # oci-mediatypes：索引的注解要 OCI 的媒体类型（S2 的 publish）
      - name: Build and push by digest
        id: build
        uses: docker/build-push-action@c3c9e263c25d99ce0380d002d59b67737d91b0dc # v7.4.0
        with:
          context: .
          file: deploy/Dockerfile
          provenance: false
          sbom: false
          outputs: type=image,name=${{ env.IMAGE }},push-by-digest=true,name-canonical=true,push=true,oci-mediatypes=true
          build-args: |
            VERSION=${{ env.VERSION }}
            REVISION=${{ steps.labels.outputs.revision }}
            CREATED=${{ steps.labels.outputs.created }}
      - name: Image smoke on the pushed digest
        env:
          DIGEST: ${{ steps.build.outputs.digest }}
        run: |
          deploy/image-smoke.sh "$IMAGE@$DIGEST" "$VERSION"
          test "$(docker image inspect -f '{{.Architecture}}' "$IMAGE@$DIGEST")" = "${{ matrix.arch }}"
      # 摘要经 artifact 交给后面的作业（矩阵的作业输出会互相盖掉）；重跑时同名的 artifact 要能覆盖
      - name: Digest
        env:
          DIGEST: ${{ steps.build.outputs.digest }}
        run: mkdir -p "$RUNNER_TEMP/digests" && printf '%s\n' "$DIGEST" > "$RUNNER_TEMP/digests/${{ matrix.arch }}"
      - uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        with:
          name: digest-${{ matrix.arch }}
          path: ${{ runner.temp }}/digests/${{ matrix.arch }}
          if-no-files-found: error
          overwrite: true
          retention-days: 30
```

（"同上""同演练"在文件里照抄完整的步骤；这里省掉重复。不抽成 composite action：那要再开一个文件，守卫也要跟着读它。BuildKit 的版本与摘要在 S1 实施时取当时的稳定版：v0.34.2，3.21、决定 555。）

要点：

- **演练用 `GITHUB_TOKEN` 的 `contents: read`**，没有 `packages`；`build` 的 `packages: write` 只在 tag 的运行里（`if`），`jobs.<id>.permissions` 不能随事件变，所以是两个作业（00 号文档 4.10）。
- **演练不加路径条件**，每次推送（分支与 `main`）都跑（00 号文档 4.10，修订核对第 1 轮 B 的 M14）；不缓存，不传 artifact（P5-7；审查 B 的 N10 与过度设计）。
- **`gate` 先跑脚本再 `make vuln`**：便宜的先拒；`make vuln` 要 Go 与 Node，只在 runner 上（`Makefile:181`）。`IMAGE` 由工作流级的 `env` 给脚本（第 4 条取一节时用）。
- **S1 合并之后、收尾之前若有人推了 tag**：S3 合并之前仓库里没有 `CHANGELOG.md`，闸门第 4 条拒绝任何 tag，`build` 不会跑；S3 之后、收尾之前推的 rc 会走完整条路，所以推 tag 只在收尾之后（4.11；审查 B 的 N1）。
- `build` 的检出不用 `fetch-depth: 0`：VCS 信息在浅克隆里照样取得到（`ci.yml` 的 `image` 一项就是浅克隆），`git log -1` 也是。

### 3.9 S1：脚本与工作流的测试（`release_test.go`）

`server/internal/platform/buildinfo/release_test.go`，三个短测试（S2 加第四个，3.13）。要 `bash`、`git`、`jq`、`awk`（`exec.LookPath`；缺了就 `t.Fatalf` 写明"README 的'需要安装'"，不跳过）。

**脚本直接执行**（审查 A 的 M5）：`exec.Command(<仓库>/deploy/release-gate.sh, tag)`，不经 `bash`：可执行位与 `#!/usr/bin/env bash` 都核对到。子进程的环境只给（审查 A 的 N1）：

- `PATH`：假的 `gh`（S2 另有假的 `docker`）所在的临时目录在最前，其后接真的 `PATH`：脚本还用 `dirname`、`grep`、`head`、`sed`，假的 `gh` 用 `cat`，不逐个列（修订核对 A 的 N12）；
- `HOME`（临时目录）、`GIT_CONFIG_GLOBAL=/dev/null`、`GIT_CONFIG_NOSYSTEM=1`（不读用户的 git 配置，例如签名）；
- `GIT_AUTHOR_NAME`、`GIT_AUTHOR_EMAIL`、`GIT_COMMITTER_NAME`、`GIT_COMMITTER_EMAIL`：没有全局配置时 `git commit` 以 128 退出；
- `LC_ALL=C.UTF-8`；
- 各用例要的变量。

**`TestOnlyATagsJobsMayWrite`**：`os.ReadDir(".github/workflows")` 列出整个目录（审查 A 的 N5），用 `go.yaml.in/yaml/v3` 解码进 `map[string]any`，核对：

1. 目录里恰是 `ci.yml`、`release.yml`、`vuln.yml` 三个文件：新加的工作流（`.yml` 或 `.yaml`）先让这个测试失败，写它的人在这里写下它的权限。
2. `ci.yml`：`on.push` 恰是 `{branches: ["**"]}`（不在 tag 上跑，3.5）；顶层 `permissions` 恰是 `{contents: read}`；没有作业另写 `permissions`。`server` 作业里 `run` 含 `lint-actions` 的步骤恰有一步，恰是 `{name: Lint workflows, if: matrix.leg == 'costs-1', run: make lint-actions}`，没有别的键（`shell`、`env`、`working-directory` 会让它不跑而照绿，`continue-on-error` 让它失败了照绿），`costs-1` 是这个作业的一份：actionlint 没有别处跑（S1 审查 C 的 M3、核对第 1 轮 C 的 N6）。
3. `vuln.yml`：顶层 `permissions` 恰是 `{contents: read}`；没有作业另写 `permissions`；`on.push` 有 `branches`、没有 `tags`。
4. 三份里每个 `uses: actions/checkout@…` 的步骤都有 `with.persist-credentials: false`（审查 B 的 N12：`.git` 进镜像的构建上下文；今天六处都写了）。
5. 三份里每个 `run:` 的文本，其中的 `${{ … }}` 只许 `matrix.` 起头的：从每一处 `${{` 起头的文字要匹配 `^\$\{\{\s*matrix\.[a-z_]+\s*\}\}`。tag 名、提交说明这类外来的文字只经环境变量进脚本，空格写不写都抓得到（审查 A 的 M6）；表达式里可以有 `}`（`format('{0}', …)`），先截出 `${{ … }}` 再看的写法会漏掉它（S1 审查 B 的 M1）。三份里每个作业的 `strategy` 的字符串都不含 `${{`：矩阵的值写死，否则外来的文字经 `${{ matrix.… }}` 进脚本（S1 核对第 1 轮 B 的 N8）。今天 `ci.yml`、`vuln.yml` 的 `run:` 里只有 `matrix.leg`，本文的 `release.yml` 只有 `matrix.arch`（修订核对 A 读过），守卫在今天的文件上就过。
6. `release.yml`：
   - `on` 只有 `push`，恰是 `{branches: ["**"], tags: ["v[0-9]+.[0-9]+.[0-9]+*"]}`（没有 `workflow_dispatch`、`pull_request`、`workflow_run`、`paths`）；
   - 顶层 `permissions` 是空的映射；
   - 作业恰是这张表里的（S1 三个，S2 八个），每个都写了 `permissions` 映射（不是 `read-all` 这样的字符串），与表恰好相等；`needs` 含表里的；
   - `if` 是整个字符串相等：不许 `||`、`always()`、`!cancelled()` 之类让它在别的事件或前面失败时也跑；
   - 没有作业、没有步骤写 `continue-on-error`（审查 A 的 M7）；`gate` 作业的步骤都不写 `if`；
   - `rehearse` 与 `build`（S2 加 `verify`）各恰有一步跑 image-smoke；`release.yml` 里每个这样的步骤都不写 `if`，`run` 里有一行 `deploy/image-smoke.sh … "$VERSION"`（`(?m)^deploy/image-smoke\.sh [^|;&]*"\$VERSION"$`），这一行后面没有别的（`|| true` 之类）（S1 审查 B 的 N3、核对第 1 轮 B 的 N6；决定 560）。它按行核对：前面一行 `set +e`、这一步或作业写不带 `-e` 的 `shell` 时，smoke 失败了照绿，交 S2（第 9 节，决定 562）。S2 改成：跑 smoke 的步骤与 `verify` 的每个跑命令的步骤只有 `name`、`env`、`run` 三个键，`run` 里没有关掉选项的 `set … +x` 与 `shopt -o`，这些作业与工作流不写 `defaults.run.shell`（3.25 第 3 条）；
   - `rehearse` 作业与工作流顶层的 `env`、`defaults` 里，没有字符串匹配 `(?i)\bsecrets\b`（各种写法、大小写，与每个作业都继承的）；演练的步骤 `uses` 与 `run` 合起来不匹配 `(?i)login`（S1 审查 B 的 N3、核对第 1 轮 B 的 M3；决定 560）；
   - `gate` 的步骤里恰有一步 `run` 是 `deploy/release-gate.sh "$GITHUB_REF_NAME"`，恰有一步是 `make vuln VULN_AUDIT_LEVEL=high`，前者在前；
   - `rehearse` 与 `build` 的 `docker/setup-buildx-action` 的 `with.driver-opts` 逐字相同，匹配 `^image=moby/buildkit:v[0-9.]+@sha256:[0-9a-f]{64}$`（P5-18）；
   - S2 加（S2 实际另核对的见 3.25 第 4 条，决定 616）：`publish` 恰有一步 `run` 是 `deploy/release-version-tag.sh "$IMAGE" "$VERSION" "$DIGEST"`，且没有 `uses: docker/login-action`；`draft` 恰有一步是 `deploy/github-release.sh draft "$GITHUB_REF_NAME" "$RUNNER_TEMP/notes.md" "$RUNNER_TEMP/out"`；`release` 恰有一步是 `deploy/github-release.sh publish "$GITHUB_REF_NAME"`；`verify` 的校验一步含 `--cert-identity "$identity"`、`--predicate-type https://spdx.dev/Document/v2.3`。

   | 作业 | `if` | `permissions` | `needs` 含 |
   |---|---|---|---|
   | `rehearse` | `github.ref_type != 'tag'` | `contents: read` | — |
   | `gate` | `github.ref_type == 'tag'` | `contents: read`、`actions: read` | — |
   | `build` | 同上 | `contents: read`、`packages: write` | `gate` |
   | `sbom`（S2） | 同上 | `contents: read`、`packages: read` | `gate`、`build` |
   | `publish`（S2） | 同上 | `contents: read`、`packages: write`、`id-token: write`、`attestations: write`、`artifact-metadata: write` | `gate`、`build`、`sbom` |
   | `draft`（S2） | 同上 | `contents: write` | `gate`、`publish`、`sbom` |
   | `verify`（S2） | 同上 | `contents: read`、`packages: read`、`attestations: read` | `gate`、`publish`、`draft` |
   | `release`（S2） | 同上 | `contents: write` | `gate`、`verify`、`draft` |

**`TestTheReleaseGate`**，表驱动。每个用例在临时目录里建 `origin.git`（`git init --bare`）与它的克隆（`git init -b main`、提交、`git push origin HEAD:main`、`git fetch`）；另有一个只在旁支上的提交；打带注释的 tag；检出 `GITHUB_SHA` 指的提交（没设时是 tag 指的），detached，同 `gate` 作业的 `actions/checkout`（S1 审查 B 的 M2、C 的 M2，核对第 1 轮 B 的 N7）；写用例的 `CHANGELOG.md`。执行的是仓库里的 `deploy/release-gate.sh`（绝对路径），工作目录设为临时的克隆：闸门按 `$(dirname "$0")` 找仓库里的说明脚本，`CHANGELOG.md` 与 git 从工作目录读。脚本不复制，可执行位才是仓库里的那一个（g14 成立的前提；修订核对 A 的 M5，A 的驱动就这样跑通了每一格）。`IMAGE`、`GITHUB_REPOSITORY` 取与生产不同、彼此不同的假值（`registry.test/nw-gate`、`o/r`）。假的 `gh`（测试写进临时目录，可执行）：

```bash
#!/usr/bin/env bash
# 把调用记进 FAKE_GH_LOG；只答 api，按路径（含查询）在 FAKE_GH_ROUTES（"路径<TAB>文件"）里找答复；没有就同 gh 一样报 404、以 1 退出
printf '%s\n' "$*" >>"$FAKE_GH_LOG"
[ "$1" = api ] || exit 2
file=$(awk -F '\t' -v p="$2" '$1 == p { print $2 }' "$FAKE_GH_ROUTES")
[ -n "$file" ] || { echo "gh: Not Found (HTTP 404)" >&2; exit 1; }
cat "$file"
```

答复照公开接口的真样子写（3.6，`m12/p5-design/ci-runs.json` 的字段），多次运行时**从新到旧**排（接口的次序；审查 A 的 M3）。每个用例给两个端点：`repos/o/r/actions/workflows/ci.yml/runs?head_sha=<sha>&branch=main&event=push&per_page=100`（只有 `ci.yml` 的运行）与 `repos/o/r/actions/runs?head_sha=<sha>&per_page=100`（所有工作流的，其中 `release.yml` 的演练建得更晚、是绿的）：闸门若改去问后一个、又不核对 `path`，"`ci.yml` 红、`release.yml` 绿"就会放行（7.2 的 g3）。

**每格只有它要拒的那一条不过，其余照"rc 放行"一格**（审查 A 的 M3）：都有 `## 0.1.0（未发布）` 下一段正文、`ci.yml` 一次 `completed success`、`GITHUB_SHA` 设为 tag 指的提交。拒绝的格断言退出码 1、标准输出为空、标准错误含那一条的字样（下表"信息"一列），不只断言 1：否则前面的一条早退会盖住后面的检查（05-lessons 第 6 节）。

| 用例 | 期望 | 信息 |
|---|---|---|
| rc 放行 | 0；标准输出与 `GITHUB_OUTPUT` 都恰是 `version=0.1.0-rc.1`、`prerelease=true`、`sha=<提交>` 三行；假 `gh` 的日志恰一行，是 `ci.yml` 的端点 | — |
| rc 放行，不设 `GITHUB_SHA`（本机的用法） | 0 | — |
| 正式，`## 0.1.0（2026-11-20）` | 0；`prerelease=false` | — |
| 正式，`## 0.1.0（未发布）` | 1 | `要写发布的日期` |
| 没有 `0.1.0` 的一节（只有 `0.0.9`） | 1 | `的标题有 0 个` |
| 两个 `## 0.1.0（…）` | 1 | `的标题有 2 个` |
| 只有 `## 0.1.00（2026-11-20）` 与 `## 0.1.0-rc.1（…）` | 1 | `的标题有 0 个` |
| 只有 `## 0x1x0（未发布）`（`.` 若被当作通配就放行；审查 A 的 M2） | 1 | `的标题有 0 个` |
| `## 0.1.0（未发布）` 紧接 `## 0.0.9（…）`，这一节是空的 | 1 | `取不出或是空的` |
| `## 0.1.0（2026-11-20）` 之下只有空行，正式 | 1 | `取不出或是空的` |
| tag 是 `v0.1`、`v0.1.0-beta.1`、`v01.0.0`、`v0.1.0-rc.0`、`0.1.0` | 1；假 `gh` 的日志是空的（写法先于一切） | `不是 vX.Y.Z` |
| tag 指旁支上的提交 | 1；日志是空的 | `不在 origin/main 上` |
| `GITHUB_SHA` 是另一个提交 | 1 | `触发这次运行的是` |
| `ci.yml` 是 `completed failure`，`release.yml` 的演练是 `completed success` | 1 | `completed failure` 与运行号 |
| `ci.yml` 是 `in_progress`（`conclusion: null`） | 1 | `in_progress null` |
| `ci.yml` 的运行 `run_attempt: 2`、`conclusion: success`（第一次是失败） | 0 | — |
| 两次 `ci.yml` 的运行（同一提交两次推上 `main`），答复从新到旧：新的红、旧的绿 | 1（取最后建的） | `completed failure` |
| `workflow_runs` 是空的 | 1 | `单独推上 main` |
| 假 `gh` 以 1 退出 | 1 | `读不到 ci.yml` |
| 答复里的运行 `path` 是 `release.yml`（端点答错） | 1 | `单独推上 main` |
| 没有 `IMAGE` | 1 | `release-gate: 要 IMAGE`（闸门自己的字样：没有它的判断时说明也拒绝，晚一步、用说明的字样；S1 改） |
| 只有缺右括号的 `## 0.1.0（未发布`（S1 加） | 1 | `的标题有 0 个` |
| `ci.yml` 在 `main` 上的运行红，同一提交另一个分支（`m12-release`）上更新的一次绿（S1 加；`select` 核对 `head_branch`） | 1 | `completed failure` 与 `main` 上那次的运行号 |
| 同上，更新的那次是 `pull_request` 的（S1 加；核对 `event`） | 1 | 同上 |
| 同上，更新的那次是另一个提交的（S1 加；核对 `head_sha`） | 1 | 同上 |

审查 A 把原稿的闸门原样取出、对着一个照这张表的驱动跑过每一格（`m12/p5-review-a/gate/`）；本稿新加的几格（不设 `GITHUB_SHA`、`0x1x0`、空的一节、从新到旧）照 S1 实施时先在没改过的脚本上跑一遍。S1 跑了：原稿五格失败（两个"这一节是空的"放行，"没有 IMAGE"放行，"没有运行"与"端点答错"两格的消息里没有"单独推上 main"），新加的几格在原稿上照过；标"S1 加"的四格与"没有 IMAGE"的字样是 `38916978` 补的，没有它们 g15–g18、g20 照过（决定 558）。

**`TestTheReleaseNotes`**：执行仓库里的 `deploy/release-notes.sh`（绝对路径），工作目录是只有 `CHANGELOG.md` 的临时目录。`CHANGELOG.md` 有 `## 0.1.0（未发布）` 与 `## 0.0.9（2026-01-01）` 两节，0.1.0 的一节前后有空行、里面有 `### 已知限制` 这样的三级标题。`IMAGE`、`GITHUB_REPOSITORY` 取与生产不同、彼此不同的假值（`registry.test/nw-notes`、`o2/r2`；审查 A 的 N6），期望里写的是这些假值：脚本里写死 `ghcr.io/open-nerve/nervewiki` 的变异抓得到（7.2 的 n5）。

- rc：整份输出与写死的期望**整个比**（05-lessons 第 1 节"整个答复与写死的期望比"）：第一句预发布、0.1.0 的正文（含三级标题，不含 0.0.9 的）、生成的部分里 `0.1.0-rc.1` 与两处 `refs/tags/v0.1.0-rc.1`。
- 正式：同样整个比，没有预发布那一句。
- 拒绝：没有这一节；这一节只有空行；摘要是 `sha256:abc`；tag 不合写法；没有 `IMAGE`；只有写坏了的 `## 0.1.0（草稿`（"没有"）。
- `CHANGELOG.md` 另有一行空格与一行前文，前文里出现 `### 0.1.0（草稿）`：结尾的空格行要去掉，只认行首的标题（S1 加，n8、n12；决定 558）。
- 同一个版本：正式的一节之后紧接写坏了的 `## 0.1.0（草稿`、写坏的在前、两个完整的标题（取第一节），都整个比（S1 审查 B 的 N1、核对第 1 轮 B 的 N5）。

### 3.10 S2 的文件

| 文件 | 改动 |
|---|---|
| `tools/licenses/go.sh` | 新：Go 一侧的许可证（3.11） |
| `tools/licenses/npm.mjs` | 新：npm 一侧的许可证（3.11） |
| `tools/licenses/README.txt` | 新：镜像里 `/licenses/README.txt` 的说明 |
| `tools/licenses/check.mjs` | 新（S2 修复第 1 轮）：两个收集的脚本对着桩跑，10 格（3.25 第 14 条） |
| `Makefile` | `lint-web` 加 `node tools/licenses/check.mjs`（S2 修复第 1 轮） |
| `deploy/Dockerfile` | web 阶段、server 阶段各收集一侧，runtime 阶段放进 `/licenses`（3.11） |
| `knip.jsonc` | 根的 `entry`（`:7-15`）加 `tools/licenses/npm.mjs`（S2 修复第 1 轮另加 `tools/licenses/check.mjs`） |
| `deploy/image-smoke.sh` | 核对 `/licenses`（3.11） |
| `deploy/release-version-tag.sh`、`deploy/github-release.sh` | 新，可执行（3.13） |
| `.github/workflows/release.yml` | `sbom`、`publish`、`draft`、`verify`、`release` 五个作业；演练加三步（3.14、3.15）；`build` 加 `test "$(git rev-parse HEAD)" = "$SHA"`（第 9 节"P5/S2"，决定 561；S2 实际是"The tree is clean"之后、Tools 之前单独一步，3.25 第 7 条） |
| `server/internal/platform/buildinfo/release_test.go` | 加 `TestTheReleaseScripts`；守卫的表加五个作业（3.9、3.13）；跑 image-smoke 的步骤与它们的作业不许写 `shell`、`defaults.run.shell`、`set +e`（第 9 节"P5/S2"，决定 562；S2 实际写成只有 name、env、run 三个键，另核对的见 3.25 第 3、4 条） |

### 3.11 S2：第三方许可证 `/licenses`

**镜像里的样子**：

```
/licenses/README.txt                         这个目录是什么、怎样读
/licenses/LICENSE                            Nerve Wiki 自己（AGPL-3.0-only，仓库根的 LICENSE）
/licenses/go/INDEX.tsv                       每行：<模块>@<版本> TAB <文件名，逗号分隔>
/licenses/go/go@go1.27.4/LICENSE、PATENTS    Go 本身（标准库与运行时随二进制发出）
/licenses/go/<模块路径>@<版本>/LICENSE…       每个链接进二进制的模块
/licenses/npm/INDEX.tsv                      每行：<包>@<版本> TAB <package.json 写的许可证> TAB <文件名，或"-"：包里没有许可证的文本>
/licenses/npm/<包名>@<版本>/LICENSE…          @nervewiki/web 与它用的工作区包的每个生产依赖，加上代码随产物发出的 tailwindcss（S2）
```

**`tools/licenses/go.sh <构建出的程序> <输出目录>`**（server 阶段，`/src/server` 下执行；`set -euo pipefail`；程序一个参数是 S2 修复第 1 轮加的）：

- `CGO_ENABLED=0 go list -deps -f '{{with .Module}}{{if not .Main}}{{.Path}}@{{.Version}}{{"\t"}}{{.Dir}}{{end}}{{end}}' ./cmd/nervewiki | sort -u`：与构建同一个 `GOOS`/`GOARCH`、同一个模块缓存（`go mod download` 之后），`CGO_ENABLED=0` 同 `go build` 那一行（`deploy/Dockerfile:34`；审查 B 的 N6：golang 镜像有 gcc，那一层的 `RUN` 默认是 1，今天两种取值的模块集合相同）。
- 每个模块：根目录里名字匹配 `^((un)?licen[cs]e|copying|notice|patents)([.-].*)?$`（不分大小写）、又不以 js、cjs、mjs、jsx、ts、cts、mts、tsx、go、json、map、sh、py 结尾的文件复制过去（S2 修复第 1 轮，决定 613：原稿的正则收代码，cytoscape 的 `license-update.mjs` 进了镜像，只有 `license.go` 的模块骗过了下一句）；一个也没有就失败，写明模块（P5-10）。
- 收完之后，`go list` 的模块要恰是 `go version -m <程序>` 的 `dep` 行，程序的 Go 版本要是收的那个 Go 的；不等就失败、构建失败（S2 修复第 1 轮，决定 614）。
- `$(go env GOROOT)/LICENSE`、`PATENTS` 进 `go@$(go env GOVERSION)/`。
- 本机实验（`86565dd6`）：47 个模块，都有许可证文件：44 个 `LICENSE`、3 个 `LICENSE.txt`，另有 1 个 `NOTICE`、7 个 `PATENTS`（`m12/p5-design/gomods.txt`；审查 B 复核相同，两种架构的模块逐行相同）。`uuid` 是 Go 1.27 的标准库，算在 Go 本身里。

**`tools/licenses/npm.mjs <输出目录>`**（web 阶段，`/src` 下执行，`pnpm install` 之后）：

- `execFileSync("pnpm", ["--filter", "@nervewiki/web...", "licenses", "list", "--prod", "--json"])`（S2 修复第 1 轮，决定 612：原稿的 `--filter @nervewiki/web` 不往工作区包 `@nervewiki/api-client` 里走，漏了打包进前端的 `openapi-fetch`），按许可证分组的 JSON，每项有 `name`、`versions`、`paths`、`license`；名字与版本读每个包自己的 `package.json`（S2）。另固定收 tailwindcss（`SHIPPED`，从 `web/apps/web/node_modules` 读，没装就失败）：它是 devDependency，preflight 与默认主题编进 CSS。
- 每个包的每个路径：同样的文件名规则复制；没有就在 INDEX 里写 `-`，不失败（P5-10）。`license` 是 `Unknown` 的照写（今天 khroma 2.1.0：`package.json` 没有 `license` 字段，包里有 `license` 文件）。
- 输出排序，同一份锁文件得到同样的目录。
- 本机实验：226 个包（S2 实施时 230 个：pnpm 列 226 项，其中 4 项各有两个版本；修复第 1 轮之后 233 个，232 个加 tailwindcss）；没有许可证文本的 3 个（fastdom 1.0.12、react-remove-scroll-bar 2.3.8、strictdom 1.0.1，`package.json` 都写 MIT）（`m12/p5-design/web-licenses.json.gz`）。锁文件里带 `cpu:` 的 159 个包没有一个在 `@nervewiki/web` 的生产依赖里（审查 B），两种架构的 `/licenses/npm` 应当相同。

**Dockerfile**：

```dockerfile
# web 阶段，pnpm install 之后、COPY web/ 之前：只随锁文件与脚本变，源码变了这一层照样复用
COPY tools/licenses/npm.mjs tools/licenses/
RUN node tools/licenses/npm.mjs /out/licenses/npm
COPY web/ web/

# server 阶段，go build 之后
RUN /src/tools/licenses/go.sh /out/nervewiki /out/licenses/go   # S2：程序与输出目录

# runtime 阶段
COPY LICENSE /licenses/LICENSE
COPY tools/licenses/README.txt /licenses/README.txt
COPY --from=server /out/licenses/go /licenses/go
COPY --from=web /out/licenses/npm /licenses/npm
```

**`image-smoke.sh`** 加（`docker create` 不运行镜像；容器名进 `cleanup`）：

```bash
# 第三方许可证（M12/P5/S2）：Go 本身、Nerve Wiki 自己与两份清单都在，清单上的每个文件都在
docker create --name "$licenses" "$image" >/dev/null
docker cp "$licenses:/licenses" "$files/licenses"
[[ -s $files/licenses/LICENSE && -s $files/licenses/go/INDEX.tsv && -s $files/licenses/npm/INDEX.tsv ]] || fail "…"
grep -q '^go@go[0-9.]*	LICENSE' "$files/licenses/go/INDEX.tsv" || fail "…"
grep -q '^github.com/jackc/pgx/v5@' "$files/licenses/go/INDEX.tsv" || fail "…"
grep -q '^react@' "$files/licenses/npm/INDEX.tsv" || fail "…"
# S2 修复第 1 轮：经工作区包打包进前端的那一个；两份清单至少 40、200 行（今天 48、233）
grep -q '^openapi-fetch@' "$files/licenses/npm/INDEX.tsv" || fail "…"
[ "$(wc -l <"$files/licenses/go/INDEX.tsv")" -ge 40 ] && [ "$(wc -l <"$files/licenses/npm/INDEX.tsv")" -ge 200 ] || fail "…"
# 清单上写的每个文件都在
```

**大小**：两侧约 280 份文本，推断 1–2 MB。S2：293 个文件、532,441 字节（10.2）。

**`tools/licenses/check.mjs`**（S2 修复第 1 轮）：`make lint-web` 调用，对着假的 `go`、`pnpm` 跑两个脚本，10 格（3.25 第 14 条）。

### 3.12 S2：两份 SBOM

| SBOM | 怎样生成 | 在哪里 | 格式 |
|---|---|---|---|
| 镜像 | `anchore/sbom-action` v0.24.3（syft v1.54.0），`image: <镜像>@<amd64 的摘要>` | `sbom` 作业（tag）；演练的 amd64 一份对本地的镜像（3.15） | `spdx-json`（SPDX 2.3） |
| 前端 | `pnpm --filter @nervewiki/web sbom --sbom-format spdx --prod --out <文件>`，之前 `pnpm install --frozen-lockfile --filter @nervewiki/web...` | 同上 | SPDX 2.3 |

- **实验**（P5-9）：本机 `pnpm sbom` 在 store 模式下 235 个包，许可证：MIT 184、ISC 34、BSD-3-Clause 7、Apache-2.0 4、AGPL-3.0-only 2（我们自己的两个包）、Unlicense 1、0BSD 1、`(MPL-2.0 OR Apache-2.0)` 1（dompurify）、NOASSERTION 1（khroma）。`--lockfile-only` 时 233 个依赖都是 NOASSERTION，所以要先装（`m12/p5-design/web.spdx.json.gz`、`web-lockonly.spdx.json.gz`）。文档的 `name` 是 `@nervewiki/web`。
- 根包的版本写 `0.0.0`（`web/apps/web/package.json` 没有版本）；文件名带版本号，不改 SBOM 的内容。
- 两份都过 `jq` 的核对：`spdxVersion == "SPDX-2.3"`；镜像的那份有 `github.com/jackc/pgx/v5`（证明 syft 读到了二进制里的模块，不只是 distroless 的几个包），前端的有 `react`。S2 起另核对两份的文档 `name`（决定 617），并核对前端 SBOM 里 `@nervewiki/*` 之外的每个包都在 `/licenses/npm/INDEX.tsv`（3.25 第 13 条，决定 614）。
- `actions/attest` 收 SPDX 时以 `https://spdx.dev/Document/v` 加 `spdxVersion` 的后半作证明的类型（`src/sbom.ts` 的 `generateSPDXPredicate`），两份都是 `https://spdx.dev/Document/v2.3`；大小上限 16 MB，两份都在 1 MB 以下（推断）。
- syft 给镜像的 SBOM 的文档 `name` 是镜像的引用（推断）：`verify` 作业靠两份的 `name` 分出镜像的与前端的（3.14），S2 的演练先记下本地镜像那一份的 `name`，与推断不同就照实改 `verify` 的那一行，写进偏离。S2：演练读出 `nervewiki`，是引用的仓库名，不带 tag 与摘要（syft v1.54.0 的源码同此），tag 上推断是 `ghcr.io/open-nerve/nervewiki`；`verify` 认 `$IMAGE`（3.25 第 1 条）。
- `sbom-action` 的 `upload-artifact`、`upload-release-assets` 都关掉：它默认在 release 事件上往 Release 传附件、并另存 artifact；附件由我们的 `draft` 作业传。

### 3.13 S2：版本号的 tag 与 Release 的两个脚本

正常的 rc 只走"不在 → 新建"与"没有 Release → 建草稿"，P5-12 要防的"已在、别的摘要"、读不到时不当作不在、已公开的不再上传，都只在出错或重跑时才走（审查 B 的 M2）。所以这两步与闸门一样写成脚本，对着假的 `docker`、`gh` 跑，不与别处等价的每一处检查各有一格让它单独变红（P5-20；S2 审查 C 的 M1，原稿写"每一支各跑一格"）。两个都可执行，`set -uo pipefail`，`fail` 同闸门。

**`deploy/release-version-tag.sh <镜像> <版本> <索引的摘要>`**（`publish` 作业的最后一步）：

```bash
[ $# -eq 3 ] || fail "用法：deploy/release-version-tag.sh <镜像> <版本> <索引的摘要>"
image=$1 version=$2 digest=$3
[[ $version =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-rc\.[1-9][0-9]*)?$ ]] || fail "版本号 \"$version\" 不对"
[[ $digest =~ ^sha256:[0-9a-f]{64}$ ]] || fail "摘要 \"$digest\" 不对"
err=$(mktemp) && trap 'rm -f "$err"' EXIT || fail "mktemp"
# 已在：指着同一个摘要就不动，指着别的就失败、不改指（整次运行重跑时构建得到新的摘要）。
# 不在：只认 registry 答的 not found；别的错误（一时读不到、没有权限）不当作不在
if have=$(docker buildx imagetools inspect "$image:$version" --format '{{json .Manifest}}' 2>"$err"); then
  have=$(jq -r .digest <<<"$have") || fail "读不懂 $image:$version 的清单"
  [ "$have" = "$digest" ] || fail "$image:$version 已指向 $have，不改指到 $digest"
  echo "$image:$version 已指向 $digest，不动"
else
  grep -qiE 'not found|manifest unknown' "$err" || fail "读不到 $image:$version：$(cat "$err")"
  docker buildx imagetools create -t "$image:$version" "$image@$digest" || fail "复制不出 $image:$version"
fi
got=$(docker buildx imagetools inspect "$image:$version" --format '{{json .Manifest}}' | jq -r .digest)
[ "$got" = "$digest" ] || fail "$image:$version 是 $got，要 $digest"
```

`imagetools inspect` 对不存在的 tag 的错误信息含 `not found`（containerd 的解析器；推断），假 `docker` 的答复照此写，rc 时若看到别的写法照实改正则。S2 审查 C 以 buildx v0.38.1 匿名读公开的包核过：答 `ERROR: <引用>: not found`；读不到的包答 `failed to authorize: … 403 Forbidden`，不会被当作不在；登录之后读自己的私有包，rc 才证明得了。

**`deploy/github-release.sh draft <tag> <说明文件> <附件目录>`**（`draft` 作业）与 **`deploy/github-release.sh publish <tag>`**（`release` 作业）。附件恰是三个：`nervewiki-<版本>-image.spdx.json`、`nervewiki-<版本>-web.spdx.json`、`nervewiki-<版本>-licenses.tar.gz`，名字由 tag 算出；rc（tag 带 `-rc.`）加 `--prerelease`。仓库取 `GH_REPO`。

- `state`（S2 修复第 1 轮改，决定 615）：`gh api --paginate "repos/$GH_REPO/releases?per_page=100" --jq '.[] | select(.tag_name == "<tag>") | .draft'`：空是没有，`true` 是草稿，`false` 是已公开；任何失败都失败，同一个 tag 多于一个、或读不懂，也失败。原稿读 `gh release view "$tag" --json isDraft`，只有标准错误含 `release not found` 才算没有（审查 B 的 M1：gh 的任何失败都以 1 退出，分不开；一时读不到就再建一个草稿，草稿不占 tag，会有两个）；可 gh v2.103.2 的 `view` 并行查 REST 与 GraphQL，两次都失败、其中一次是 not found 时也报 `release not found`，一次临时的错就被当作没有（S2 审查 C 的 M4）。
- **draft**：附件目录恰有那三个文件，否则失败。
  - 没有：`gh release create "$tag" --draft --verify-tag --title "Nerve Wiki $tag" --notes-file <说明> [--prerelease]`；
  - 是草稿：`gh release edit "$tag" --notes-file <说明> [--prerelease]`；
  - 已公开（重跑）：只核对它的附件恰是那三个（`gh release view --json assets`），以 0 退出，不再上传、不改说明；
  - 之后 `gh release upload "$tag" <三个文件> --clobber`，再核对附件恰是那三个。
- **publish**：
  - 没有：失败（草稿该已由 `draft` 建好）；
  - 是草稿：附件恰是那三个，否则失败、不公开；然后 `gh release edit "$tag" --draft=false`，再读一次要是已公开；
  - 已公开：附件恰是那三个就以 0 退出。

本机 gh 2.103.2 的帮助与二进制核对过：`create --draft --verify-tag --prerelease --notes-file --title`、`edit --draft=false`、`upload --clobber` 都在；`gh release view <tag>` 按 tag 找得到草稿（二进制里有 `release(tagName:)` 的 GraphQL 查询，审查 B；`view`、`edit`、`upload` 都经 `FetchRelease`，S2 审查 C 读源码核过；S2 起附件仍由 `view --json assets` 读）；`gh api` 的 `--paginate` 与 `--jq` 合用时每一页各算一次查询（`pkg/cmd/api/api.go`，S2）；`gh release create --draft` 不查已有的草稿（`create.go`，S2 审查 C）；immutable releases 的保护在公开时才生效，草稿的附件可以改（`gh release create --help`）。

**`TestTheReleaseScripts`**（`release_test.go`，S2），表驱动，执行仓库里的两个脚本（绝对路径，不复制），PATH 照 3.9。

假的 `docker` 只答 `buildx imagetools inspect <ref> --format …` 与 `buildx imagetools create -t <ref> <src>`：状态在 `FAKE_DOCKER_STATE` 里（`absent`、`sha256:…`，或 `error:<信息>`）；`create` 把来源的摘要写进状态（`FAKE_DOCKER_CREATE_FAILS` 设了时以 1 退出），`FAKE_DOCKER_AFTER` 设了时 `create` 之后的 `inspect` 答它（模拟复制出别的）；每次调用记进日志。

| 版本号的 tag | 期望 |
|---|---|
| 不在（`not found`） | 0；日志有一次 `create -t img:0.1.0 img@<摘要>` |
| 已在、同一个摘要 | 0；没有 `create` |
| 已在、别的摘要 | 1，`不改指`；没有 `create` |
| `inspect` 答 `unauthorized` | 1，`读不到`；没有 `create` |
| `create` 失败 | 1 |
| `create` 之后 `inspect` 是别的 | 1，`要 sha256:` |
| 摘要或版本号写法不对 | 1；日志是空的 |

假的 `gh`（S2 修复第 1 轮起）只答 `api --paginate …/releases?per_page=100 --jq …`（三页，这个 tag 的在第二页、混在别的 tag 之间，每页各跑一次 `jq`）与 `release view --json assets`、`create`、`edit`、`upload`：状态（`absent`、`draft`、`published`、`two`，或 `error:<信息>`）与附件名在两个文件里，`create` 把状态改成 `draft`，`upload` 加附件，`edit --draft=false` 改成 `published`；`FAKE_GH_EDIT_FAILS`、`FAKE_GH_UPLOAD_FAILS` 让那两样失败。两个假命令都以 NUL 分隔记下每次调用的每个参数（S2 审查 C 的 N6）。失败的地方分两种：照真的（对没有的 Release `view`、`edit`、`upload`；对已公开的 Release `upload`，打开 immutable releases 时）；绊线，不许脚本去要（已在的 tag 再 `imagetools create`，真的会改指；有 Release 的 tag 再 `gh release create --draft`，gh 不查已有的草稿，接口再建一个是本节的前提）（S2 审查 C 的 M3、核对第 1 轮 C 的 N1）。

| Release | 期望 |
|---|---|
| draft，没有（`release not found`），rc | 0；日志依次 `create … --draft --verify-tag … --prerelease`、`upload … --clobber`；附件三个 |
| draft，没有，正式 | 0；`create` 没有 `--prerelease` |
| draft，已是草稿 | 0；`edit --notes-file`、`upload` |
| draft，已公开、附件齐 | 0；没有 `upload`、没有 `edit` |
| draft，已公开、缺一个附件 | 1；没有 `upload` |
| draft，列表答 `HTTP 502`（S2 改） | 1；没有 `create` |
| draft，同一个 tag 两个 Release（S2 加） | 1，`不止一个`；没有 `create` |
| draft，草稿的 `edit` 失败（S2 加） | 1；没有 `upload` |
| draft，前一次已传三个附件、`upload` 失败（S2 加） | 1，`传不上` |
| draft，附件目录少一个或多一个文件（S2 拆成两格） | 1；日志是空的 |
| publish，草稿、附件齐 | 0；`edit --draft=false`，之后是已公开 |
| publish，草稿、缺附件 | 1；没有 `edit --draft=false` |
| publish，已公开 | 0；没有 `edit` |
| publish，没有 | 1 |
| 别的动作名（S2 加） | 1，用法；日志是空的 |

S2 另加的格（共 31 格）见 3.25 第 5 条。

### 3.14 S2：`release.yml` 的发布部分

在 3.8 的 `build` 之后加五个作业（检出、`Tools` 的写法同 3.8；检出都是 `persist-credentials: false`）：

```yaml
  # 读的作业（M12/P5/S2）：取许可证、生成两份 SBOM。pnpm install 的安装脚本与 syft 只在这里跑，不在能签名、能写 GHCR 的 publish 里
  sbom:
    needs: [gate, build]
    if: github.ref_type == 'tag'
    runs-on: ubuntu-24.04
    timeout-minutes: 20
    permissions:
      contents: read
      packages: read
    env:
      VERSION: ${{ needs.gate.outputs.version }}
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1（persist-credentials: false）
      - uses: actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333 # v8.0.2
        with:
          pattern: digest-*
          path: ${{ runner.temp }}/digests
          merge-multiple: true
      - uses: docker/login-action@8aa5831fb8d48d8f76594c7ddde61ae58019f550 # v4.8.0（ghcr.io，github.actor，github.token）
      - name: Digests
        id: digests
        run: |
          for arch in amd64 arm64; do
            d=$(cat "$RUNNER_TEMP/digests/$arch"); [[ $d =~ ^sha256:[0-9a-f]{64}$ ]] || { echo "$arch 的摘要不对：$d"; exit 1; }
            echo "$arch=$d" >> "$GITHUB_OUTPUT"
          done
      # 附件与镜像里的逐字节相同；两种架构的 /licenses 要一样，否则一份附件说不清
      - name: Licenses
        env:
          AMD64: ${{ steps.digests.outputs.amd64 }}
          ARM64: ${{ steps.digests.outputs.arm64 }}
        run: |
          mkdir -p "$RUNNER_TEMP/out"
          for arch in amd64 arm64; do
            if [ "$arch" = amd64 ]; then ref="$IMAGE@$AMD64"; else ref="$IMAGE@$ARM64"; fi
            cid=$(docker create --platform "linux/$arch" "$ref")
            docker cp "$cid:/licenses" "$RUNNER_TEMP/$arch"; docker rm "$cid" >/dev/null
          done
          diff -r "$RUNNER_TEMP/amd64" "$RUNNER_TEMP/arm64"
          tar -czf "$RUNNER_TEMP/out/nervewiki-$VERSION-licenses.tar.gz" -C "$RUNNER_TEMP" --transform 's,^amd64,licenses,' amd64
      - name: Image SBOM
        uses: anchore/sbom-action@66cbf4bc1f1c0d2edc94016e65bc221b6bb0ad6c # v0.24.3
        with:
          image: ${{ env.IMAGE }}@${{ steps.digests.outputs.amd64 }}
          format: spdx-json
          output-file: ${{ runner.temp }}/out/nervewiki-${{ env.VERSION }}-image.spdx.json
          syft-version: v1.54.0
          upload-artifact: false
          upload-release-assets: false
      - uses: actions/setup-node@949feb2413d6458794dcd2491c4babbbce0c15c1 # v7.1.0（.node-version）
      - name: Enable corepack
        run: corepack enable
      # 不缓存 pnpm 的 store（P5-7）
      - name: Install the web app's dependencies
        run: pnpm install --frozen-lockfile --filter @nervewiki/web...
      - name: Web SBOM
        run: pnpm --filter @nervewiki/web sbom --sbom-format spdx --prod --out "$RUNNER_TEMP/out/nervewiki-$VERSION-web.spdx.json"
      # 文档的 name 是 verify 分两份证明用的：镜像的是 $IMAGE（推断，这一步先核对，rc 上证明），前端的是包名（M12/P5/S2，
      # 3.25 第 1 条）；前端 SBOM 里的包都在 /licenses（3.25 第 13 条）
      - name: Check the SBOMs
        run: |
          out=$RUNNER_TEMP/out
          for f in "$out/nervewiki-$VERSION-image.spdx.json" "$out/nervewiki-$VERSION-web.spdx.json"; do
            jq -r --arg f "${f##*/}" '"\($f)：name=\(.name)，\(.packages | length) 个包"' "$f"
          done
          jq -e --arg image "$IMAGE" '.spdxVersion == "SPDX-2.3" and .name == $image and any(.packages[]; .name == "github.com/jackc/pgx/v5")' "$out/nervewiki-$VERSION-image.spdx.json" >/dev/null
          jq -e '.spdxVersion == "SPDX-2.3" and .name == "@nervewiki/web" and any(.packages[]; .name == "react")' "$out/nervewiki-$VERSION-web.spdx.json" >/dev/null
          export LC_ALL=C
          sbom=$(jq -r '.packages[] | select(.name | startswith("@nervewiki/") | not) | "\(.name)@\(.versionInfo)"' "$out/nervewiki-$VERSION-web.spdx.json" | sort -u)
          index=$(cut -f1 "$RUNNER_TEMP/amd64/npm/INDEX.tsv" | sort -u)
          absent=$(comm -23 <(echo "$sbom") <(echo "$index"))
          missing=$(grep -v '^@types/' <<<"$absent" || true)
          [ -n "$sbom" ] && [ -z "$missing" ] || { echo "前端 SBOM 里这些包不在 /licenses/npm/INDEX.tsv："; echo "$missing"; exit 1; }
          echo "前端 SBOM 里 @nervewiki/* 之外的 $(wc -l <<<"$sbom") 个包，不在 /licenses/npm/INDEX.tsv（$(wc -l <<<"$index") 行）的只有 @types/：$(paste -sd' ' - <<<"$absent")"
      - uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        with:
          name: release-files
          path: ${{ runner.temp }}/out/
          if-no-files-found: error
          overwrite: true
          retention-days: 30

  # 合成索引、来源证明、两份 SBOM 的证明、签名，最后复制出版本号的 tag（00-M12 4.10"次序与重跑"）：
  # 0.1.0 出现时已经签好、证明好；包已公开时不会有一段公开却没签名的 0.1.0。第三方的 action 只有 cosign-installer
  publish:
    needs: [gate, build, sbom]
    if: github.ref_type == 'tag'
    runs-on: ubuntu-24.04
    timeout-minutes: 20
    permissions:
      contents: read
      packages: write
      id-token: write
      attestations: write
      artifact-metadata: write
    env:
      VERSION: ${{ needs.gate.outputs.version }}
      SHA: ${{ needs.gate.outputs.sha }}
    outputs:
      digest: ${{ steps.index.outputs.digest }}
    steps:
      - 检出（取 release-version-tag.sh）；download-artifact：digest-*（同 sbom）与 release-files（path: ${{ runner.temp }}/out）
      # 登录用 shell，不多一个第三方的 action；actions/attest 与 cosign 读同一份 docker 的凭证
      - name: Log in to GHCR
        env:
          GH_TOKEN: ${{ github.token }}
        run: printf '%s' "$GH_TOKEN" | docker login ghcr.io -u "$GITHUB_ACTOR" --password-stdin
      # 先打一个不是版本号的 tag；注解从 amd64 那一份的 label 读，不另写一遍（runner 自带的 buildx）。推上去的镜像来自
      # 闸门放过的提交：label() 之后先 [ "$(label revision)" = "$SHA" ]，不等就失败（S2，决定 561）
      - name: Index
        id: index
        run: |
          amd64=$(cat "$RUNNER_TEMP/digests/amd64"); arm64=$(cat "$RUNNER_TEMP/digests/arm64")
          docker pull -q "$IMAGE@$amd64" >/dev/null
          label() { docker image inspect -f "{{index .Config.Labels \"org.opencontainers.image.$1\"}}" "$IMAGE@$amd64"; }
          docker buildx imagetools create -t "$IMAGE:sha-$SHA" \
            --annotation "index:org.opencontainers.image.description=$(label description)" \
            --annotation "index:org.opencontainers.image.source=$(label source)" \
            --annotation "index:org.opencontainers.image.licenses=$(label licenses)" \
            "$IMAGE@$amd64" "$IMAGE@$arm64"
          digest=$(docker buildx imagetools inspect "$IMAGE:sha-$SHA" --format '{{json .Manifest}}' | jq -r .digest)
          platforms=$(docker buildx imagetools inspect "$IMAGE@$digest" --format '{{json .Manifest}}' |
            jq -r '[.manifests[].platform | "\(.os)/\(.architecture)"] | sort | join(" ")')
          [ "$platforms" = "linux/amd64 linux/arm64" ] || { echo "索引的平台是 $platforms"; exit 1; }
          echo "digest=$digest" >> "$GITHUB_OUTPUT"
      - name: Provenance
        uses: actions/attest@1e69f48acb82d1966a394da916b4c1698aa569d6 # v4.2.2
        with:
          subject-name: ${{ env.IMAGE }}
          subject-digest: ${{ steps.index.outputs.digest }}
          push-to-registry: true
      - name: Image SBOM attestation
        uses: actions/attest@1e69f48acb82d1966a394da916b4c1698aa569d6 # v4.2.2
        with:
          subject-name: ${{ env.IMAGE }}
          subject-digest: ${{ steps.index.outputs.digest }}
          sbom-path: ${{ runner.temp }}/out/nervewiki-${{ env.VERSION }}-image.spdx.json
          push-to-registry: true
      - name: Web SBOM attestation
        uses: actions/attest@…（同上，sbom-path 换成 web 的那一份）
      - uses: sigstore/cosign-installer@756fe2d284c7292c4e9b62c8a28f1e5629c59532 # v4.1.3
        with:
          cosign-release: v3.1.4
      # keyless：Sigstore 的证书来自这次运行的 OIDC 令牌，身份是这个 tag 上的 release.yml
      - name: Sign
        env:
          DIGEST: ${{ steps.index.outputs.digest }}
        run: cosign sign --yes "$IMAGE@$DIGEST"
      # 最后才打给人用的版本号；已有的不改指（3.13）
      - name: Version tag
        env:
          DIGEST: ${{ steps.index.outputs.digest }}
        run: deploy/release-version-tag.sh "$IMAGE" "$VERSION" "$DIGEST"

  # Release 的草稿与附件（M12/P5/S2）：公开在 verify 之后（release 作业）
  draft:
    needs: [gate, publish, sbom]
    if: github.ref_type == 'tag'
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    permissions:
      contents: write
    env:
      DIGEST: ${{ needs.publish.outputs.digest }}
      GH_TOKEN: ${{ github.token }}
      GH_REPO: ${{ github.repository }}
    steps:
      - 检出；download-artifact release-files 到 ${{ runner.temp }}/out
      - name: Notes
        run: deploy/release-notes.sh "$GITHUB_REF_NAME" "$DIGEST" > "$RUNNER_TEMP/notes.md"
      - name: Draft
        run: deploy/github-release.sh draft "$GITHUB_REF_NAME" "$RUNNER_TEMP/notes.md" "$RUNNER_TEMP/out"

  # 公开 Release 之前在两种架构上按版本号的 tag 拉回来校验（00-M12 4.10；核对在公开之前是主持者定的）。
  # rc 还是私有的包，本机拉要负责人的令牌
  verify:
    name: verify (${{ matrix.arch }})
    needs: [gate, publish, draft]
    if: github.ref_type == 'tag'
    （matrix 同 build）
    runs-on: ${{ matrix.runner }}
    timeout-minutes: 20
    permissions:
      contents: read
      packages: read
      attestations: read
    env:
      VERSION: ${{ needs.gate.outputs.version }}
      DIGEST: ${{ needs.publish.outputs.digest }}
      GH_TOKEN: ${{ github.token }}
    steps:
      - 检出（tag 的提交，image-smoke 照它核对提交与时间）；Tools（另查 `gh`，S2）；docker/login-action（ghcr.io）
      - uses: sigstore/cosign-installer@756fe2d284c7292c4e9b62c8a28f1e5629c59532 # v4.1.3（cosign-release: v3.1.4）
      - name: The version tag is the index
        run: test "$(docker buildx imagetools inspect "$IMAGE:$VERSION" --format '{{json .Manifest}}' | jq -r .digest)" = "$DIGEST"
      # 与 Release 说明里给用户的两条命令同一个身份（deploy/release-notes.sh）
      - name: Provenance and SBOMs
        run: |
          identity="https://github.com/$GITHUB_REPOSITORY/.github/workflows/release.yml@$GITHUB_REF"
          gh attestation verify "oci://$IMAGE:$VERSION" -R "$GITHUB_REPOSITORY" --cert-identity "$identity"
          names=$(gh attestation verify "oci://$IMAGE:$VERSION" -R "$GITHUB_REPOSITORY" --cert-identity "$identity" \
            --predicate-type https://spdx.dev/Document/v2.3 --format json \
            --jq '[.[].verificationResult.statement.predicate.name] | unique | join(" ")')
          case " $names " in *" @nervewiki/web "*) ;; *) echo "没有前端的 SBOM 证明：$names"; exit 1 ;; esac
          case " $names " in *" $IMAGE "*) ;; *) echo "没有镜像的 SBOM 证明：$names"; exit 1 ;; esac   # S2：$IMAGE，3.25 第 1 条
      # 验过的载荷记进日志，打出每份的类型：rc 上看验过的是 cosign sign 的签名，不只是 actions/attest 的证明（3.23 第 5 步）
      - name: Signature
        run: |
          cosign verify "$IMAGE:$VERSION" --output json \
            --certificate-identity "https://github.com/$GITHUB_REPOSITORY/.github/workflows/release.yml@$GITHUB_REF" \
            --certificate-oidc-issuer https://token.actions.githubusercontent.com > "$RUNNER_TEMP/cosign.json"
          # 只打出、不断言：输出的形状不是预期的也不让这一步红，校验由上面 cosign verify 的退出码断言
          jq -r '.[] | .critical.type // "（没有 critical.type）"' "$RUNNER_TEMP/cosign.json" ||
            { echo "cosign 的输出不是预期的形状："; cat "$RUNNER_TEMP/cosign.json"; }
      - name: Image smoke
        run: |
          deploy/image-smoke.sh "$IMAGE:$VERSION" "$VERSION"
          test "$(docker image inspect -f '{{.Architecture}}' "$IMAGE:$VERSION")" = "${{ matrix.arch }}"

  # 两种架构上都核对过，才公开 Release（M12/P5/S2）
  release:
    needs: [gate, verify, draft]
    if: github.ref_type == 'tag'
    runs-on: ubuntu-24.04
    timeout-minutes: 10
    permissions:
      contents: write
    env:
      GH_TOKEN: ${{ github.token }}
      GH_REPO: ${{ github.repository }}
    steps:
      - 检出（取 github-release.sh）
      - name: Publish the Release
        run: deploy/github-release.sh publish "$GITHUB_REF_NAME"
```

（几处"同上""检出"在文件里写全；`verify` 的 `env` 不写 `${{ github.ref }}` 的拼接，身份在 `run:` 里由默认的环境变量拼，守卫的第 5 条因此照过。）

要点：

- **次序**：每个架构构建、smoke、按摘要推送（`build`）→ 取许可证、两份 SBOM（`sbom`，只读）→ 合成索引、打 `sha-<提交>`、来源证明、两份 SBOM 的证明、签名、复制出版本号（`publish`）→ Release 的草稿与附件（`draft`）→ 两种架构上按 tag 校验（`verify`）→ 公开 Release（`release`）。版本号的 tag 在 `verify` 之前就出现：包已公开时（`v0.1.0`），`verify` 失败的那段时间里 `0.1.0` 已能拉到，只是没有公开的 Release（3.23）。
- **签名的载荷**（修订核对 B 的 M7）：`cosign verify` 默认把验过的载荷以 JSON 打到标准输出（推断，实施时核对），`--output json` 写明；`jq` 打出每份的 `critical.type`，`cosign sign` 的签名应是 `cosign container image signature`。只打出、不断言：cosign v3 怎样列 `actions/attest` 的 bundle 没有核实，rc 上读了再定要不要断言（3.23 第 5 步）；`jq` 读不懂（元素不是对象、输出不是 JSON）时打出原文，不让 `verify` 红（修订核对 B 的 N16）。
- **校验的身份**：`gh attestation verify` 带 `--cert-identity`，与 cosign 的身份逐字相同，也与说明里给用户的命令相同（审查 B 的 M4）。SBOM 的证明按两份的文档 `name` 分：一份 `@nervewiki/web`，一份是 `$IMAGE`（S2：3.12 推断的是以 `$IMAGE@sha256:` 起头，演练读出的是仓库名，3.25 第 1 条），重跑多出的几份去重之后不影响（审查 B 的 N5）。
- **重跑**：一时的失败在同一次运行上 Re-run failed jobs。`imagetools create` 对同样的两个来源得到同一个索引（推断；不同时 `sha-` 改指、版本号的 tag 照 P5-12 不动）；证明与签名多一份无害；`github-release.sh` 认得草稿与已公开；`overwrite: true` 让重跑的 `build`、`sbom` 能再传同名的 artifact。重跑的作业能不能下载前一次尝试传的 artifact，rc 之前看不到（审查 B 未核实的第 2 条），两种结果的收拾都写在 3.23 的表里。
- **整次运行重跑**：`build` 得到新的摘要，`publish` 在新的索引上证明、签名，最后一步因为版本号的 tag 已指着旧的而失败，不改指；`draft` 不跑。这时用 rc.N+1。
- **版本号以外的 tag**：只有 `sha-<提交>` 与确切的版本号；没有 `latest`、`0.1`（决定 322）。
- **gh 的仓库**：`draft`、`release` 设 `GH_REPO`，不靠检出的 remote 去猜。

### 3.15 S2：演练加的三步

在 3.8 的 `rehearse` 的 Image smoke 之后，只在 amd64 上（`if: matrix.arch == 'amd64'`）：

1. **许可证**：`docker create`、`docker cp nervewiki:rehearsal` 的 `/licenses`，`tar` 成 `nervewiki-$VERSION-licenses.tar.gz`（`image-smoke.sh` 已核对内容）。
2. **镜像的 SBOM**：`anchore/sbom-action`，`image: nervewiki:rehearsal`（syft 先找本机的 Docker），同 3.14 的参数；打出文档的 `name`（3.12）。
3. **前端的 SBOM**：同 3.14 的安装与 `pnpm sbom`，不缓存 store。

之后同 3.14 的"Check the SBOMs"（S2：演练要镜像的那份 `name` 是 `nervewiki`，决定 617）。不传 artifact（审查 B 的过度设计）：要看的在作业日志里。演练的权限照旧只有 `contents: read`。

这样 rc 之前只剩推送、GHCR 上的 `imagetools`、证明与签名、Release 与按 tag 的校验没有在分支上走过（3.23）。`sbom` 作业的跨架构 `docker create --platform` 与 `docker cp`、`diff -r`，审查 B 在本机对称地核对过（本机 arm64 上建、拷一个 amd64 的镜像，不要仿真）。

### 3.16 S3 的文件

| 文件 | 改动 |
|---|---|
| `deploy/compose.yaml`、`deploy/compose-initdb.sh` | 新（3.17） |
| `.gitignore`、`.dockerignore` | 加 `deploy/.env`、`deploy/secrets/`、`deploy/data/`、`deploy/backups/`：有人在仓库里照样例试时，私钥不进构建上下文、二进制不报 `modified=true`（13.5 第 3 条要两份一致；审查 C 的 N8） |
| `Makefile` | `deploy-check` 目标（3.17） |
| `.github/workflows/ci.yml` | `image` 一项在 Image smoke 之前加 Deploy check 一步（3.17） |
| `platform/postgres/pgtest/image_test.go` | 加 `deploy/compose.yaml`；README 一项换成 `deploy/README.md`（3.17、3.19） |
| `deploy/README.md` | 新：运维手册（3.18） |
| `README.md` | `:9` 不动（发布时改，第 9 节）；`:45`、`:229` 的"见部署"改指 `deploy/README.md`；`:498-633` 缩成入口（3.19） |
| `deploy/runtime-grants.sql:2`、`bootstrap/runtime_role_test.go:31`、`:464` | 注释里的 README "部署"改为 `deploy/README.md` |
| `CHANGELOG.md`、`SECURITY.md` | 新（3.19） |
| `docs/v0.1/M12-release/manual/screen-reader-checklist.md`、`manual/README.md` | 新（3.19） |

### 3.17 S3：`deploy/compose.yaml` 与 `make deploy-check`

```yaml
# Nerve Wiki 的部署样例（deploy/README.md 的"安装"）：PostgreSQL、迁移与服务，一台机器、单实例。
# 同目录要有：.env（chmod 600）写 NERVEWIKI_VERSION（只写确切的版本号，例如 0.1.0）、NWIKI_DB_ADMIN_PASSWORD、NWIKI_DB_PASSWORD、
# NWIKI_PUBLIC_URL；secrets/jwt.pem；data/（属主 65532）；compose-initdb.sh。
# 升级：docker compose stop serve，备份，改 NERVEWIKI_VERSION，docker compose run --rm migrate，docker compose up -d serve
# （deploy/README.md 的"升级与回退"）。make deploy-check 核对它的写法；PostgreSQL 的镜像与建库参数由
# server/internal/platform/postgres/pgtest/image_test.go 核对
name: nervewiki

services:
  db:
    image: postgres:18.7-trixie
    environment:
      # 超级用户 postgres 只在容器里用：初始化、备份与恢复。服务与迁移用 compose-initdb.sh 建的 nervewiki，它不是超级用户
      POSTGRES_PASSWORD: ${NWIKI_DB_ADMIN_PASSWORD:?写进 .env，例如 openssl rand -hex 24 的输出}
      # builtin provider 的 C.UTF-8：不满足时服务拒绝启动（总体设计 7.1）
      POSTGRES_INITDB_ARGS: --locale-provider=builtin --locale=C.UTF-8
      # compose-initdb.sh 第一次建库时读它
      NWIKI_DB_PASSWORD: ${NWIKI_DB_PASSWORD:?写进 .env，例如 openssl rand -hex 24 的输出（放进连接地址，只用十六进制）}
    volumes:
      # PostgreSQL 18 的数据目录在 /var/lib/postgresql/18/docker，卷挂在 /var/lib/postgresql
      - db:/var/lib/postgresql
      - ./compose-initdb.sh:/docker-entrypoint-initdb.d/10-nervewiki.sh:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -h 127.0.0.1 -U nervewiki -d nervewiki"]
      interval: 2s
      timeout: 5s
      retries: 30
    restart: unless-stopped

  # 不随 up 启动：备份之后显式执行 docker compose run --rm migrate。serve 依赖它完成的写法会在改了版本号再 up 时、
  # 备份之前就迁移（00-M12 4.8，设计审查 B 的 M11）
  migrate:
    profiles: [migrate]
    image: ghcr.io/open-nerve/nervewiki:${NERVEWIKI_VERSION:?写进 .env：确切的版本号，例如 0.1.0}
    command: [migrate, up]
    environment: &environment
      NWIKI_DATABASE__URL: postgres://nervewiki:${NWIKI_DB_PASSWORD:?写进 .env}@db:5432/nervewiki?sslmode=disable
      NWIKI_AUTH__JWT__PRIVATE_KEY_FILE: /run/secrets/jwt
    secrets: [jwt]
    depends_on:
      db:
        condition: service_healthy

  serve:
    image: ghcr.io/open-nerve/nervewiki:${NERVEWIKI_VERSION:?写进 .env：确切的版本号，例如 0.1.0}
    environment:
      <<: *environment
      NWIKI_SERVER__PUBLIC_URL: ${NWIKI_PUBLIC_URL:?写进 .env：对外的地址，例如 https://wiki.example.com}
      # 宿主机上的反向代理经发布的端口进来，服务看到的对端是下面这个网络的网关；不信任它，所有客户端就共用按 IP 的限流
      # （deploy/README.md 的"反向代理与 HTTPS"）。与 networks 的子网一起改
      NWIKI_SERVER__TRUSTED_PROXIES: 10.213.47.0/24
    secrets: [jwt]
    volumes:
      # 附件、导入导出（/data）。第一次装时 mkdir data && sudo chown 65532:65532 data；不可写时服务拒绝启动
      - ./data:/data
    ports:
      # 只听本机：前面是反向代理
      - "127.0.0.1:${NERVEWIKI_PORT:-8080}:8080"
    # 停机的各段之和（server.shutdown_timeout 20 秒 + 通知 2 秒 + 任务 10+1 秒 + 连接池 5 秒 = 38 秒），deploy/README.md 的"运行与监控"
    stop_grace_period: 40s
    depends_on:
      db:
        condition: service_healthy
    restart: unless-stopped

networks:
  default:
    ipam:
      config:
        # 钉住子网，serve 的 trusted_proxies 信任它。与宿主机上已有的网络重叠时，两处一起改
        - subnet: 10.213.47.0/24

secrets:
  # 私钥：openssl genpkey -algorithm ed25519 -out secrets/jwt.pem。uid 65532 要能读（chmod 644），
  # 所在的目录只给部署的用户（chmod 700），宿主机上别的用户就读不到
  jwt:
    file: ./secrets/jwt.pem

volumes:
  db:
```

**`deploy/compose-initdb.sh`**（只在数据卷是空的、第一次建库时由官方镜像执行一次）：

```sh
#!/bin/sh
# deploy/compose.yaml 的数据库初始化（M12/P5/S3）：建一个不是超级用户的 nervewiki，作库与之后各表的所有者；
# 服务与迁移都用它。只在第一次建库时执行：改密码要在库里 ALTER ROLE，只改 .env 不够（deploy/README.md）
set -eu
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres -v pw="$NWIKI_DB_PASSWORD" <<'SQL'
CREATE ROLE nervewiki LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD :'pw';
CREATE DATABASE nervewiki OWNER nervewiki;
SQL
```

- 新库从 `template1` 继承 initdb 的 builtin `C.UTF-8`。迁移 00001 建 `pg_trgm`：它自 PostgreSQL 13 起是 trusted 的扩展，库的所有者能建。审查 C 的修订核对在 18.7 上照这份样例装了三遍（初始化脚本 644 被引入、755 被执行两种），`rolsuper=f`、`rolcreatedb=f`，54 条迁移都过，`pg_trgm` 的所有者是 `nervewiki`（`m12/p5-check1-c/`）；S3 的演练照样看这一格。
- 环境变量的名字照 `server/configs/config.yaml` 与 README 的"配置"写，审查 C 照初稿装过一遍核对过；`NWIKI_SERVER__TRUSTED_PROXIES` 用逗号分隔的 CIDR（`README.md:106`）。`.env` 里的 `NWIKI_DB_PASSWORD`、`NWIKI_PUBLIC_URL` 没有 `__`，服务不把它们当配置键（`config/load.go:137-139`）。
- 子网 `10.213.47.0/24` 不在 Docker 默认的地址池（172.17–31 与 192.168）里，少撞。与别的 Docker 网络撞了时 `up` 报 "Pool overlaps"；与宿主机自己的网段（局域网、VPN）重叠时 Docker 未必拒绝，可能让宿主机到那一段的路由失效（推断，没有实测，修订核对 C 未核实的一条）：`deploy/README.md` 让用户第一次 `up` 之前看一眼 `ip route`，撞了照注释两处一起改。
- **`make deploy-check`**：

  ```make
  # deploy/compose.yaml 的写法（M12/P5/S3）：docker compose config 查语法、变量与 schema（整份文件都查，不看 profile），
  # --profile migrate 另让 migrate 里的引用（secrets、depends_on）也核对。必填的变量给假值。升级的次序由 S3 的演练核对
  .PHONY: deploy-check
  deploy-check: ## 核对 deploy/compose.yaml 的写法（需要 Docker Compose v2）
  	cd deploy && NERVEWIKI_VERSION=0.0.0-check NWIKI_DB_ADMIN_PASSWORD=check NWIKI_DB_PASSWORD=check \
  		NWIKI_PUBLIC_URL=https://wiki.example.com docker compose -f compose.yaml --profile migrate config -q
  ```

  本机的 Compose v5.6.1：缺必填变量时 `config -q` 以 1 退出并写出说明；`secrets/jwt.pem`、`data/` 不在时照过；只把 `migrate` 的 `secrets` 写成 `[jwtx]` 时，带 `--profile migrate` 失败、不带照过（审查 C 的探针，7.2 的 d2）。
- **CI**：`ci.yml` 的 `image` 一项在 Image smoke 之前加 `- name: Deploy check` / `run: make deploy-check`（13.5 第 1 条：新门禁同时进 Makefile 与 CI），约多几秒。
- **守卫**：`TestContainerMatchesTheOtherDatabases`（`pgtest/image_test.go:14-34`，`strings.Contains`）加 `{file: "deploy/compose.yaml", want: {"image: " + image + "\n", "POSTGRES_INITDB_ARGS: " + initdbArgs + "\n"}}`；注释加"the deployment sample"。

### 3.18 S3：`deploy/README.md` 的大纲与数的出处

照 00 号文档 4.8 的八节加附录。每节写"做什么"的命令，不写开发过程。

1. **要求**：
   - 硬件两档：默认配置；小机器（`page.parse_budget_bytes` 5 MiB 加 `GOMEMLIMIT=1536MiB`，P4/S3 批次 E 另量的一组，见下表）。每档写内存、核数、磁盘。最低配置是 EPYC 7763 的 4 核一类，那一档用 CI 守卫在 7763 上的实数；写明最大合法的 `batch`、回收站与撤销在慢的 4 核上离 15 秒的期限不远（移交 M4-performance 补记 `:490`、`:492`）。
   - PostgreSQL 18.7 或更新，builtin `C.UTF-8`，带 LZ4；托管的 PostgreSQL 要能这样建库（`platform/postgres/check.go:53-66` 不满足就拒绝启动）。
   - 磁盘：正文约 5 倍给搜索，加版本的增长、导出与导入的临时文件、WAL。
   - v0.1 只支持单实例与原因：附件在本地磁盘；限流、大请求体的名额、正在上传的导入只在进程内计数；启动时的收拾（P3/S3 的 T4）假定只有一个实例。多实例之间的时钟因此不适用（移交 M5-performance 第 1、2 条）。
2. **安装**：
   - 拉镜像并校验：Release 说明里的两条命令与 SBOM 那一条；`gh` 要先 `gh auth login`，cosign 用 v3；
   - 用 `deploy/compose.yaml`：放 `compose.yaml`、`compose-initdb.sh`、`.env`（`chmod 600`）、`secrets/jwt.pem`；第一次 `up` 之前看一眼 `ip route`，没有与 `10.213.47.0/24` 重叠的网段；`mkdir data && sudo chown 65532:65532 data`（没有 sudo 时借已有的镜像当 root：`docker run --rm -v "$PWD":/w --entrypoint sh postgres:18.7-trixie -c 'chown 65532:65532 /w/data'`，修订核对 C 的 M13）；`docker compose up -d db`；`docker compose run --rm migrate`；`docker compose up -d serve`；第一个管理员 `printf '%s\n' '<密码>' | docker compose run --rm -T serve users create --email …`（链到根 README 的"管理员命令"）；看 `/readyz` 与 `/api/v0/instance` 的版本号；
   - 为什么设 `trusted_proxies` 与钉子网（审查 C 的 I1）：宿主机上的代理连 `127.0.0.1:8080`，经 Docker 的端口转发进容器，服务看到的对端是 compose 网络的网关；不信任它时服务不采信 `X-Forwarded-For`，所有客户端算作这一个 IP，按 IP 的限流（登录、注册、认证失败、匿名的读、附件的内容）全实例共用一个桶，一个人连着输错密码整个团队都登录不了，会话记下的 IP 也都是网关。代理也放进 compose 时，可信的是代理容器的地址；
   - 库角色：样例用一个不是超级用户的 `nervewiki`，它是表的所有者（P5-16）；要把迁移与服务分成两个角色的，照"不用 compose"的写法；
   - 不用 compose：建库（`CREATE DATABASE … LOCALE_PROVIDER builtin LOCALE 'C.UTF-8' TEMPLATE template0`），两个角色与 `deploy/runtime-grants.sql`（`migrate up` 之后、`serve` 之前执行），`docker run` 的 `migrate up` 与 `serve`（从根 README `:511-518` 挪来，地址改 GHCR）；
   - 配置：`NWIKI_ENV=prod`（镜像默认）、`server.public_url`、`server.trusted_proxies`、附件目录的属主 65532（`:520-524` 挪来）。
3. **反向代理与 HTTPS**：nginx 与 Caddy 的整站样例（`README.md:555-618` 挪来）；JSON 开压缩（00 号文档 4.7.2 的缓解）；几个 GB 的导出下载的超时与缓冲（移交 M7-transfer 第 3 条）；上传与 MCP 的几条（`:619-627`）；大的 MCP 请求体与正文写较多时推荐 nginx，经 Caddy 时早答的拒绝成 502 的代价（移交 M4-performance 第 18 条）。
4. **备份与恢复**（审查 C 的 M3、N11）：
   - 次序：先停 `serve`，先库后 `blobs/`（`:525` 的道理）；
   - compose 的命令：`mkdir -p backups`；`docker compose stop serve`；`docker compose exec -T db pg_dump -U postgres -Fc nervewiki > backups/nervewiki-<日期>.dump`（`-T`：不分配终端，二进制的输出才完整）；`sudo tar -C data -czf backups/blobs-<日期>.tar.gz blobs`；`.env` 与 `secrets/jwt.pem` 另存；`docker compose up -d serve`；
   - 不用 compose 的命令：`pg_dump -Fc`；两个角色时 `pg_dumpall --roles-only`；附件目录的 `blobs/` 打包；私钥与配置；
   - 恢复（compose）：停掉、把旧的 `data/` 挪开、`docker compose down -v`（只删本项目的 `db` 卷；`data/` 是绑定的目录，不受影响）；`docker compose up -d db`（初始化脚本重建角色与空库）；`docker compose exec -T db pg_restore -U nervewiki -d nervewiki --exit-on-error < 备份`（以库的所有者恢复，`pg_trgm` 与各表的所有者同新装一样都是 `nervewiki`；以 `postgres` 恢复时 `pg_trgm` 归 `postgres`，修订核对 C 的 N12 实测）；`mkdir data && sudo tar -C data -xzf 备份 && sudo chown -R 65532:65532 data`；**先 `docker compose exec -T db psql -U postgres -d nervewiki -c ANALYZE` 再开放服务**（移交 M4-performance 第 10 条）；`docker compose up -d serve`，看 `/readyz`；
   - 没有 sudo 时，`tar`、`chown` 与删掉属主是 65532 的目录都借已有的 `postgres:18.7-trixie` 当 root 做（`docker run --rm -v "$PWD":/w --entrypoint sh postgres:18.7-trixie -c '…'`），写出三条命令；
   - 一次演练的步骤（S3 的演练就照它做，3.20）。
5. **升级与回退**：
   - 通用的步骤：拉新镜像并校验；**先停** `docker compose stop serve`（或 `docker stop -t 40`；审查 C 的 M5：迁移可能持 `ACCESS EXCLUSIVE` 重写整张表，不混跑旧、新的 writer）；备份；改 `NERVEWIKI_VERSION`；`docker compose run --rm migrate`；两个角色时补授权；有说明时 `nervewiki reindex`；`docker compose up -d serve`；看 `/readyz` 与版本号；
   - 数：`nervewiki reindex` 在 30 万页约 26 分钟、约 290 GB 的 WAL，之后 `page_search` 约是原来的 1.7 倍（从 `README.md:535` 挪来，M8 的数，P4 不重量）；
   - 回退靠恢复备份；`migrate down` 不保证数据；
   - PostgreSQL 的小版本（18.7 的 GIN 修复：怀疑损坏时 `REINDEX`；18.7 说明里 TOAST 索引的提示）与大版本（`pg_upgrade` 或 dump/restore、builtin locale、复核"一个事务里几条 `NOTIFY` 只取一次全局锁"，移交 M5-performance 第 5 条，总体设计第 14 节的 `NOTIFY` 一行指到这里）。
6. **运行与监控**：探针；日志；磁盘与 507、由管理员监控磁盘（没有总量配额，移交 M7-transfer 第 6 条）；停机的次序与 `docker stop -t 40`；`server.shutdown_timeout` 20 秒短于一次 `/mcp` 调用（至多约 45 秒），停机时还在传大请求体的调用被切断、整个回滚，要它们做完就两者一起调长（移交 M7-transfer 第 6 条）；连接数（`max_conns` + 2，`:632`）；`GOMEMLIMIT`；连接池代理（PgBouncer）只能用会话模式（移交 M4-performance 第 10 条）；校时：下载地址的到期按客户端的时钟比较（移交 M7-transfer 第 4 条）；`exports/`、`imports/` 是过程中的文件（移交 M7-transfer 第 3 条）。
7. **容量**：下表的数。
8. **运维相关的已知限制**：单实例；没有总量配额；结束的任务行不清理（移交 M7-transfer 第 1 条）；没有永久删除与它的变通（移交 M8-history 第 4 条）；`batch` 没有单独的期限，最大合法的 `batch` 在慢的机器上可能超过请求的期限、整个回滚；P4 经负责人改判的超出（若有，移交 M4-performance 第 14 条）。
- **附录"从 v0.1.0 之前的开发版升级"**：只在负责人说有这样的实例时写（2.2 第 6 条），照它所在的迁移号核对 `README.md:527-544` 的六条；没有就删掉那六条（它们在 git 的历史里）。

**数从哪里来**（审查 C 的 M11：与 `p4-draft.md` 第 9 节"给部署文档"对过）。S3 不自己量；P4/S3 没给的，S3 写"推算"与依据，或问主持者：

| 数 | 出处 |
|---|---|
| 默认配置的内存：服务进程空闲与 N = 20 个编辑者稳态的 RSS；B2、B3 的峰值；PostgreSQL 容器的 `memory.peak` | P4/S3 的 B2、B3 与 `editors`（P4 记录的"给部署文档"） |
| `GOMEMLIMIT` 的建议值 | 同上 |
| 小机器档的内存 | P4/S3 批次 E 的小机器一组（P4 设计的 P4-27：`page.parse_budget_bytes` 5 MiB 加 `GOMEMLIMIT=1536MiB`，B2 的 k = 8；修订核对 C 的 M12，主持者定） |
| 最低 CPU（EPYC 7763 4 核）的期限余量 | P4/S3 的 B5 硬期限一层在 7763 上的数与余量、本机对 7763 之比（补记 `:492`） |
| 磁盘：每 1,000 页（2 KiB 正文、30 条链接）的库的大小；`page_search` 对正文的倍数 | P4/S3 的 B1 与"给部署文档"；M8 的"约 5 倍"（`README.md:535`）作对照 |
| 磁盘：附件与导出的 zip | P4/S3 批次 F |
| WAL：导入上限的一次导入、30 万页从空建搜索 | P4/S3（移交 M4-performance 第 17 条） |
| WAL 与时长：`nervewiki reindex` | M8 的数（`README.md:535`），P4 不重量 |
| 版本的增长 | P4/S3 批次 G 的公式与两三组代入的数（推算） |
| 时长：导入、导出、搜索的队列清空 | P4/S3 批次 F |
| 10 万页的树被 k 个标签页重读时的 CPU 与出口字节；树的 JSON 的压缩比 | P4/S3 批次 E 与"只记的"（3.18 第 3 条的"JSON 开压缩"要压缩比） |
| 迁移的停机 | 只在批次 H 量了时（2.2 第 6 条） |
| 已知限制的清单 | P4 记录的"给部署文档"（每项写出处、数与 v0.2 的去处） |

**S3 的记录里的对照表**（00 号文档第 3 节第 7、10 条）：

- 部署项逐条对照：上面"移交的落实"表（第 2 节）每一行、00 号文档"移交的落实"里写着"部署文档"的每一行（M4-performance 的第 2、3、10、14、17、18 条与补记 `:490`、`:492`），对到 `deploy/README.md` 的节与句。
- 已知限制逐条对照：00 号文档 4.13 的来源（H1、24.1、Shift-JIS 等的误读与 CP437、`batch` 的期限、P4 只量的、运维相关的、续期途中刷新被登出、P2 推后的 C 类、第三方许可证里没有文本的包）对到 `CHANGELOG.md` 的句子，最后一列写进了哪一份移交（v0.2 的五份与 v0.1.1 的一份，收尾时写）；没有去处的不许有。S3 合并时 P2 还没合并完的那一部分，表里写"收尾时补"（2.1 第 9 条）。

### 3.19 S3：根 README、`CHANGELOG.md`、`SECURITY.md`、`manual/`

**根 README**：

- "部署"一节（`:498-633`）换成：

  > 部署、升级、备份与恢复见 [deploy/README.md](deploy/README.md)：镜像 `ghcr.io/open-nerve/nervewiki`（amd64 与 arm64，只有确切的版本号），`deploy/compose.yaml` 样例，nginx 与 Caddy 的样例，校验签名与来源证明的命令。在本地构建镜像：`make image VERSION=…`、`make image-smoke VERSION=…`，要在普通的克隆里（`git worktree` 的 `.git` 是文件，镜像取不到提交信息）；`.dockerignore` 排除的正好是 `.gitignore` 忽略的，改一个时同步另一个。

- `:500` 的 M10 段落与 `:540` 等的"M12 须实机复核"不搬：照 P4 的结论写进 `deploy/README.md` 的"升级"或已知限制（00 号文档 4.8）。
- `:45`、`:229` 的"见下文'部署'"改为 `见 [deploy/README.md](deploy/README.md)`。
- `:630` 的 PostgreSQL 一句挪进 `deploy/README.md` 的"要求"；守卫里 README 一项换成 `deploy/README.md`（要 `` `postgres:18.7-trixie` ``）。`:16` 的 Go 版本不动（Go 版本的守卫读它）。
- `:9`"项目处于 v0.1 开发阶段"在发布时改（第 9 节）。

**`CHANGELOG.md`**（仓库根，中文，决定 323）：

```markdown
# 变更记录

每个版本一节，标题 `## X.Y.Z（发布的日期）`，发布之前写 `## X.Y.Z（未发布）`。发布的闸门（`deploy/release-gate.sh`）认这个写法；
Release 的说明取这一节（`deploy/release-notes.sh`）。

## 0.1.0（未发布）

…
```

0.1.0 一节：做了什么（按 M 一句）；人工验收在发布之后、还没在真实的读屏软件、输入法与 MCP 客户端上逐项核对（4.12）；改了已有的键与 id 的两处（4.6.3）：标题键的切罗基文，带这样的标题的试用库升级之后跑一次 `nervewiki reindex`（同一父页下已有只差切罗基文大小写的兄弟页时它报撞键，先改名其中一页）；标题 id 带上自动链接与原始 HTML 的文字，不要 reindex，只是这类标题的锚点变了（审查 C 的 N4）；已知限制（4.13，按主题十来句，含第三方许可证里三个包没有文本）；升级与校验指向 `deploy/README.md`。日期在 4.11 第 6 步写。

**`SECURITY.md`**：怎样报告漏洞（GitHub 的私密漏洞报告，2.2 第 5 条；不要开公开的 issue）；支持的版本（0.1.x 的最新补丁版本）；怎样校验镜像（指 `deploy/README.md`）。

**`manual/`**（4.12）：

- `screen-reader-checklist.md`：附录 A 的八项（16.8、18.4a、21.15、21.16、22.10、26.1、26.2 的后一半、28.3）、M 类里与读屏有关的各项与第 39 条，每项写在哪一页、怎样操作、期望读出什么；用哪种读屏软件与浏览器；结果怎样记（日期、软件与版本、过或不过、不过时的现象）。P2 各 Step 记录的"读屏与人工清单"一节照抄进来（S1：[记录](reviews/P2-S1-focus-and-shared-review.md)；S2：[记录](reviews/P2-S2-text-review.md)；S4：[记录](reviews/P2-S4-reading-view-review.md)）。
- `README.md`：汇总的入口，链到输入法清单（`docs/v0.1/M4-pages/manual/P6-ime-checklist.md`）、MCP 的清单 A–K（`docs/v0.1/M9-mcp/manual/P5-mcp-clients-checklist.md`）、读屏清单与 M 类的其余各项、P2/S3 交的两项（真的 Safari 上窄屏的搜索页与原生下拉的焦点，[S3 的记录](reviews/P2-S3-narrow-bdi-contrast-review.md)"读屏与人工清单"）；写明都在发布之后做，发现的问题出 v0.1.1。

### 3.20 S3：三遍演练

照 `deploy/README.md` 本身做，用 `deploy/compose.yaml` 与 `compose-initdb.sh`，在本机。

- **写成脚本、整个经 `heavy.sh` 跑**（审查 C 的 N7）：三遍各一个脚本，放在 `~/.cache/m12-p5-s3/` 下自己的目录，PostgreSQL 与服务跨好几步一直开着，整个持锁（06-environment 第 3 节）；脚本的命令与输出就是记录（大的 `gzip`）。哪一处与文档不符、怎样改了文档，记录照实写。
- **镜像**：`make image` 要在普通的克隆里（`Makefile:89`）：构建之前看 `df -h /`（旧提交的基础镜像要另拉），`git clone` 本仓库到 `~/.cache/m12-p5-s3/repo`，各检出要的提交，`make image VERSION=0.1.0-drill.N IMAGE=ghcr.io/open-nerve/nervewiki:0.1.0-drill.N`。新的是 S3 分支的头；旧的照 P5-17：`git log --diff-filter=A --format=%H 5e8af5b5..HEAD -- server/migrations/sql/` 有输出时取最早那一个的父提交，没有时取 `5e8af5b5`（审查 C 的 M2：原稿的 `git log -1 -- server/migrations/` 取到的是只改了测试的提交）。
- **本机没有 sudo**（`sudo -n true` 要密码；修订核对 C 的 M13）：文档里的 `sudo chown`、`sudo tar` 与清理时删 `data/`，演练借已有的 `postgres:18.7-trixie` 当 root 做，例如 `docker run --rm -v "$PWD":/w --entrypoint sh postgres:18.7-trixie -c 'chown 65532:65532 /w/data'`（C 的 `p5-check1-c/drill.sh` 就这样跑通了装与恢复）。记录写明这是与文档唯一的出入，也正是 `deploy/README.md` 写的"没有 sudo 时"的写法。
- **隔离**：每遍一个自己的目录与项目名（`docker compose -p nervewiki-drill-<n>`），`NERVEWIKI_PORT` 用一个空闲的端口，三遍依次跑（钉住的子网同一时刻只能有一个项目用）；结束时 `docker compose -p … down -v` 只删这个项目的容器、网络与 `db` 卷，借 `postgres:18.7-trixie` 删那个目录下的 `data/` 与恢复那一遍挪开的旧目录（属主都是 65532、`blobs/` 是 750，用户删不掉；修订核对 C 的 N17），删本机构建的演练镜像（按名字）。
- **从零装一遍**：照"安装"：`.env`、私钥、`data/` 的属主、`up -d db`、`run --rm migrate`（核对非超级用户建得了 `pg_trgm`）、`up -d serve`、建管理员、登录；建工作区、笔记本（照 `image-smoke.sh:154-167`）、一页（`POST /api/v0/notebooks/{id}/pages`，审查 C 的 N5）、一个附件；`/readyz` 与 `/api/v0/instance` 的版本号。另核对：
  - 库角色：`select rolsuper from pg_roles where rolname = 'nervewiki'` 是 `f`，表与 `pg_trgm` 的所有者是 `nervewiki`（恢复之后也是）；
  - 可信的代理（审查 C 的 I1）：经发布的端口发一个带 `X-Forwarded-For: 203.0.113.9` 的登录（照 `image-smoke.sh` 登录的那一步），服务的日志里没有 `ignored X-Forwarded-For from a peer that is not a trusted proxy`，`auth_sessions` 里这次会话记下的 IP 是 `203.0.113.9`。
- **升一次级**：在旧的镜像上装好、写入数据；照"升级"：停 `serve`、备份、改 `.env` 的版本号、`run --rm migrate`（记下执行了哪几条迁移，或"没有待执行的迁移"）、`up -d serve`；版本号是新的，页面与附件的字节都在。
- **备份再恢复一遍**：照"备份与恢复"：备份；挪开 `data/`（挪开的旧目录留到清理时借容器删）、`down -v`；初始化脚本重建的空库、恢复、放回 `blobs/`、改属主、`ANALYZE`、启动；`data/blobs` 的属主是 65532、`serve` 起得来，页面、附件的字节与备份之前相同。
- **之后**：包设为公开之后，用 GHCR 的 rc 再装一遍（4.11 第 5 步，3.23）。

### 3.21 用到的 action 与工具的版本

按提交引用、注释写 tag（13.5 第 2 条）。设计者起草时（2026-11-16）以 `git ls-remote` 读过一次 tag 与它指的提交；审查 B 在沙箱里连不上 GitHub，没能复核，只对照了 `m12/survey-i1/actions-new-shas.txt` 与今天 `ci.yml`、`vuln.yml` 钉的 SHA，都相同。**这些 SHA 不当作已核对：P5/S1 实施时以公开仓库核对**，那时也再核一次最新版（主持者）。S1 实施时（2026-11-17）以 `git ls-remote` 从公开仓库读过十一个 action 的 tag，每个 SHA 都是表里写的（`anchore/sbom-action` v0.24.3 的 tag 对象 `fffec3d8…` 解开是 `66cbf4bc…`）；S1 的审查 B 以 `gh api` 另核了 `release.yml` 用的七个。最新版：只有 `actions/setup-go` 出了 v7.1.0，三份工作流照旧 v7.0.0，一起升走 4.3 的路（决定 556）；其余十个与 actionlint v1.7.12 都是最新。

| action 或工具 | 版本 | 钉到 | 用在 | 第三方 |
|---|---|---|---|---|
| `actions/checkout` | v7.0.1 | `3d3c42e5aac5ba805825da76410c181273ba90b1` | 各作业（同 `ci.yml`） | 否 |
| `actions/setup-go` | v7.0.0 | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` | `gate` | 否 |
| `actions/setup-node` | v7.1.0 | `949feb2413d6458794dcd2491c4babbbce0c15c1` | `gate`、`sbom`、演练 | 否 |
| `actions/upload-artifact` | v7.0.2 | `cf430e030ddbb5b0abf93d22962f4752f3646cd9` | `build`、`sbom` | 否 |
| `actions/download-artifact` | v8.0.2 | `9000827ccba6bdab643e8b6fd33ac0654aef8333` | `sbom`、`publish`、`draft` | 否 |
| `actions/attest` | v4.2.2 | `1e69f48acb82d1966a394da916b4c1698aa569d6` | `publish`（三次） | 否 |
| `docker/setup-buildx-action` | v4.4.1 | `f87e5991a6d7451dcb8d9637bfbc97413f497069` | 演练、`build` | 是 |
| `docker/build-push-action` | v7.4.0 | `c3c9e263c25d99ce0380d002d59b67737d91b0dc` | 演练、`build` | 是 |
| `docker/login-action` | v4.8.0 | `8aa5831fb8d48d8f76594c7ddde61ae58019f550` | `build`、`sbom`、`verify`（`publish` 用 shell） | 是 |
| `sigstore/cosign-installer` | v4.1.3（cosign v3.1.4） | `756fe2d284c7292c4e9b62c8a28f1e5629c59532` | `publish`、`verify` | 是 |
| `anchore/sbom-action` | v0.24.3（syft v1.54.0） | `66cbf4bc1f1c0d2edc94016e65bc221b6bb0ad6c`：tag 是带注释的，它的对象是 `fffec3d8…`，钉解出来的提交 | `sbom`、演练 | 是 |
| BuildKit（`moby/buildkit`） | v0.34.2（S1 实施时 `buildx-stable-1` 指它；v0.35.0 才发 6 天，决定 555） | `v0.34.2@sha256:c5f8862f52499773af85bc07695b2379ef08e8002352b0df2c4864c91d78dabe`（index，含 linux/amd64 与 linux/arm64） | 演练、`build`（`driver-opts`） | 是 |
| actionlint | v1.7.12 | `go run …@v1.7.12`（`Makefile`） | `make lint-actions` | — |
| `gh` | runner 自带 | — | `gate`、`draft`、`verify`、`release` | — |
| `docker buildx imagetools` | runner 自带的 buildx | — | `publish`、`verify` | — |

- `build-push-action` v7.4.0 的输入有 `context`、`file`、`build-args`、`load`、`push`、`outputs`、`provenance`、`sbom`、`tags`，输出有 `digest`（`action.yml`）。`DOCKER_BUILD_RECORD_UPLOAD` 这个名字没有复核（审查 B 未核实的第 5 条），错了只是多传一个 artifact。S1 的演练核实了：构建记录只写在本地，运行的 artifact 是 0 个。
- `actions/attest` v4.2.2 的输入有 `subject-name`、`subject-digest`、`sbom-path`、`push-to-registry`（要恰好一个带摘要的 OCI 主体）、`create-storage-record`（默认开，要 `artifact-metadata: write`）；只给主体时是 SLSA 的来源证明（README）。
- `docker/metadata-action` 不用（决定 322）。

### 3.22 每个新检查的反向对照

| 检查 | 反向对照 | 在哪里 |
|---|---|---|
| 版本号的默认值 | 7.2 的 v1–v6 | 3.2 |
| MCP 的版本 | m1–m4：写死默认值由盖了版本号的 `make test-go-short` 抓，直接 `go test` 照过（写明）；去掉盖版本号由测试的第二条抓 | 3.3 |
| label | l1–l4 | 3.4 |
| 三份工作流的守卫 | w1–w16（S2 的 w15、w16 在 7.2）；S2 修复第 1 轮的 33 个（b5–b24 等，7.2 与 S2 的审查记录） | 3.9 |
| 闸门 | g1–g14；3.9 的表的每一格先在没改过的脚本上跑 | 3.6、3.9 |
| 说明 | n1–n5 | 3.7、3.9 |
| 闸门作业的 `make vuln` 照 `high` | v-gate：PATH 里 `go`、`pnpm` 两个桩加 `GITHUB_ACTIONS=true` 跑 `make vuln VULN_AUDIT_LEVEL=high`，`pnpm` 的桩记下 `audit --prod --audit-level=high`（同 P1/S2 的变异 15）；`go` 也要桩，否则会在本机真跑 `govulncheck`。实施时手做一次，CI 不常驻 | 7.2 |
| 版本号的 tag、Release 的草稿与公开 | r1–r7 与 S2 修复第 1 轮的 c1–c3、c15、s1–s6（假的 `docker`、`gh`） | 3.13 |
| `/licenses` | x1–x3；S2 修复第 1 轮的 l1–l13（`tools/licenses/check.mjs` 的 10 格）与 f1–f5（`make image`、`make image-smoke`）；前端 SBOM 对清单的核对由 f1 的 INDEX 在本机照工作流的原文反向对照；收口的 n1–n1c | 3.11 |
| actionlint | 它的规则不是我们的，不做反向对照；"本机拒绝"是我们的代码：7.2 的 a1、a2，PATH 里只放一个记参数的假 `go`，不跑 actionlint（修订核对 A 的 N11） | 3.5 |
| `deploy-check` | d1、d2 | 3.17 |
| 守卫加 `compose.yaml` 与 `deploy/README.md` | p1、p2 | 3.17 |
| 可信的代理、库角色 | 演练里的两项核对（3.20），不进 CI | 3.20 |

### 3.23 只有推 tag 才证明得了的：rc 的步骤与失败时的收拾

**分支上证明不了的**（9.5 第 3 层；S2 另加 REST 的 Release 列表对 `contents: write` 的令牌含草稿、tag 上镜像的 SBOM 的 `name`，第 9 节"发布"）：GHCR 的登录与按摘要推送（包第一次建出、关联仓库）；`imagetools create` 在 GHCR 上合成索引与注解；`actions/attest` 推进 GHCR 与存储记录；cosign keyless 与 GHCR 怎样存 cosign v3 的签名；Release 的草稿、附件与公开；按 tag 的 `gh attestation verify`、`cosign verify`；重跑时能不能下载前一次尝试的 artifact；闸门对真接口的读（字段照真样子写了，3.6）；tag 的运行的 `head_branch` 是不是 tag 名。

**4.11 的步骤，写成命令**（收尾合并、`main` 的 CI 绿之后；主持者做，第 6 步负责人做）：

1. **之前**：
   - `main` 的头在 `ci.yml` 与 `release.yml` 的演练都绿（`ci-wait.sh <sha>`、`ci-wait.sh <sha> release.yml`）；
   - `gh run list --workflow vuln.yml --branch main --limit 1` 为绿；
   - 手动触发一次 `make outdated`（`gh workflow run vuln.yml --ref main`），读 `outdated` 一项的输出，六节各有结果（P1 设计第 9 节）；要升的照 4.3 开旁支，或进 v0.2 的 `M12-release.md`；
   - actionlint、syft、cosign、BuildKit 的版本手动核一次（第 2 节"不做"）；
   - `CHANGELOG.md` 有 `## 0.1.0（未发布）`，下面不是空的。
2. **问负责人**（一次问全，回答与日期记进收尾记录）：推 `v0.1.0-rc.1` 与 `v0.1.0` 两个 tag；同时请他做 2.2 第 1–4 条的设置；说明 rc 会在 Sigstore 的公开透明日志里留下记录、在 GitHub 上留下一个公开的预发布 Release，镜像在设为公开之前是私有的。
3. **核对设置**（审查 B 的 N11）：主持者以 `gh api` 只读地看规则集（`repos/open-nerve/NerveWiki/rulesets`）、immutable releases 是否打开、`open-nerve` 下有没有 `nervewiki` 包（`orgs/open-nerve/packages/container/nervewiki`，要 404）；接口读不到的（权限或名字不对，推断的接口），请负责人回一句"做好了"，记进收尾记录。
4. **推 rc**：`git tag -a v0.1.0-rc.1 -m 'Nerve Wiki v0.1.0-rc.1' <main 的头>`（这个提交有它自己的 `ci.yml` 运行，审查 A 的 N9），`git push origin v0.1.0-rc.1`。
5. **核对 rc**：`gh run list -R open-nerve/NerveWiki --workflow release.yml --branch v0.1.0-rc.1`、`gh run watch <id>`；九个作业（`gate`、两个 `build`、`sbom`、`publish`、`draft`、两个 `verify`、`release`；审查 B 的 N2）都绿；`gh release view v0.1.0-rc.1`：预发布、说明里有"预发布"一句、三个附件；读一遍说明。记下 run 号、索引的摘要、两个架构的摘要、`verify` 的输出，另读两个 `verify` 作业的日志：Signature 一步打出的每份载荷的类型，看验过的是 `cosign sign` 的签名，不只是 `actions/attest` 的证明（主持者，审查 B 未核实的第 4 条；修订核对 B 的 M7：本机拉私有的 rc 要负责人的令牌，另开工作流又过不了守卫）；GHCR 怎样存 cosign v3 的签名也记下。
6. **负责人把包设为公开**（不能撤回）。之后主持者在本机：`docker logout ghcr.io`；`docker pull ghcr.io/open-nerve/nervewiki:0.1.0-rc.1`（本机是 arm64）；`gh attestation verify oci://ghcr.io/open-nerve/nervewiki:0.1.0-rc.1 -R open-nerve/NerveWiki --cert-identity …@refs/tags/v0.1.0-rc.1`（本机没有 cosign，签名由两个 `verify` 作业核过）；照 `deploy/README.md` 用这个镜像从零装一遍（3.20 最后一条）。
7. **`v0.1.0`**：从 `main` 开 `m12-release`，`CHANGELOG.md` 的标题写日期 `## 0.1.0（YYYY-MM-DD）`，`README.md:9` 改为已发布的说法；`--no-ff` 合并，**合并提交单独推上 `main`**（不与记合并号的文档提交一起推：闸门只认 tag 指的提交自己的 `ci.yml` 运行），等它的 CI 与演练绿；`git tag -a v0.1.0 -m 'Nerve Wiki v0.1.0' <合并提交>`、推；同第 5 步核对（不是预发布、没有那一句）；本机不登录再拉一次。
8. **`0.1.1-dev`**：在 `m12-release` 上改 2.1 第 11 条列的各处与 `make gen`，版本号的守卫照过；合并，`main` 的 CI 绿。
9. **记录**：收尾记录的"发布"一节（00 号文档 4.11 第 8 步）。

**失败时**（P5-15；只改流水线、部署文档或说明的 rc.N+1 不再问负责人，改了产品代码或依赖的再问，决定 324）：

| 在哪里失败 | 那时对外有什么 | 怎样收拾 |
|---|---|---|
| `gate` | 只有 git 的 tag | CI 还在跑或一时的错：CI 绿之后 Re-run failed jobs。`CHANGELOG.md` 或脚本的问题：`m12-release` 上改、合并，推 rc.N+1。`make vuln` 红：照 4.3，要升依赖或加 `audit.ignore` 的再问负责人 |
| `build`（某个架构） | GHCR 里可能有没有 tag 的摘要（私有） | 一时的错（runner、GHCR）：Re-run failed jobs。只在 arm64 上坏的构建：改了产品代码，再问负责人，rc.N+1 |
| `sbom` | 同上 | Re-run failed jobs；syft 或 pnpm 的问题改流水线，rc.N+1。Check the SBOMs 报文档的 `name` 不对，或前端 SBOM 里有不在 `/licenses` 的包：改 `verify` 的那一行或 `npm.mjs`，rc.N+1（S2） |
| `publish` | 加上 `sha-<提交>` 的索引，可能已有证明与签名（透明日志里有记录）；没有版本号的 tag | Sigstore 或 GHCR 一时的错：Re-run failed jobs；重跑的作业下载不到前一次尝试的 artifact 时，版本号的 tag 还没出现，可以 Re-run all jobs（新的摘要、`sha-` 改指）。流水线的问题：rc.N+1；留下的 `sha-` 与签名不删 |
| `draft` | 版本号的 tag（rc 是私有的包）；可能有一个草稿 | Re-run failed jobs；下载不到前一次的 artifact 时，Re-run all jobs 会在版本号的 tag 一步失败（P5-12），只能删掉草稿、rc.N+1。S2：Release 的列表读不到（一时的错）：Re-run failed jobs；报"不止一个"：在 GitHub 上删掉多出来的草稿（只留一个），再 Re-run failed jobs |
| `verify` | 版本号的 tag；Release 是没公开过的草稿 | 校验的命令或流水线写错：改流水线与说明，删掉草稿，rc.N+1。镜像真有问题：同上，且是产品的问题时再问负责人 |
| `release`（公开） | 草稿 | Re-run failed jobs |
| rc 公开之后才发现（例如第 6 步的安装） | 公开的预发布 Release | rc.N 的 Release 不删，`gh release edit` 把标题改为"Nerve Wiki v0.1.0-rc.N（作废，见 rc.N+1）"、说明前面加一句（immutable 的 Release 也能改标题与说明）；修了推 rc.N+1 |
| `v0.1.0` 的任何一步，版本号的 tag 还没出现 | 只有 git 的 tag `v0.1.0` 与没有 tag 的摘要 | 一时的错：Re-run failed jobs。别的：停下问负责人：删掉 `v0.1.0` 的 git tag（规则集不许，要负责人）、修好之后重推（还没有发布任何东西），还是出 `v0.1.1`（4.11 的"不重推同一个 tag"） |
| `v0.1.0` 的 `verify` 因校验命令或流水线本身的错红了，产品没有问题 | 公开的 `0.1.0` 镜像（包已公开）；没公开过的草稿 | 主持者定（P5-21）：不出 `v0.1.1`；在 `m12-release` 上修流水线；照 Release 说明里的命令手动校验（本机不登录拉、`gh attestation verify`；cosign 由负责人）；记进收尾记录；手动公开那个草稿（`deploy/github-release.sh publish v0.1.0`）是对外的一步，先问负责人 |
| `v0.1.0` 有产品的问题 | 公开的 `0.1.0` | 照 4.11：不重推，出 `v0.1.1` |

GHCR 里留下的 rc 的镜像、没有 tag 的摘要与 `sha-` 的 tag 不删（删包的版本要负责人的管理员权限），收尾记录列出，v0.2 的 `M12-release.md` 写清不清（审查 C 的 N10）。

### 3.24 S1 的实施对设计的修正

S1 的审查与修复见 [S1 的审查记录](reviews/P5-S1-version-pipeline-review.md)，结果在 10.1。与上文最初的设计不同之处已改在各节里（头的状态与日期，2.2 第 1 项，3.1 的文件表，3.4，3.5，3.6，3.7 第 2 条，3.8 的 `driver-opts` 与代码块后一句，3.9 第 2、5、6 条、`TestTheReleaseGate` 的建法与表、`TestTheReleaseNotes`，3.10 的两行，3.21，6.1 第 5、6 项与其后一段，7.2，7.4，第 8 节第 5–7 条，第 9 节"P5/S2"）；00 号文档第 7 节"总体设计的修订"第 6 条落了前一半，第 11 节加主持者的决定 555–563。写了行号的，代码是 S1 的头 `ce8aaf64`。

1. **第 5 个之后的条件提交**（3.5、6.1）：报出的是 `make lint-actions` 自己的 `-format`，不是现有的工作流；下一个提交是 `2db96b0a`，改成裸的 `\n`，没有"the workflows pass actionlint"（决定 557）。
2. **BuildKit**（3.8、3.21）：`v0.34.2` 与它的 index 摘要（决定 555）。
3. **3.21 的核对**：十一个 SHA 都与表相同；`actions/setup-go` 有 v7.1.0，三份工作流照旧 v7.0.0（决定 556）。
4. **作业的表写成函数 `releaseJobs()`**（3.9）：gochecknoglobals 不许全局的 map；设计没有写形式。
5. **README 的"需要安装"**（3.1、6.1）：写"bash、git、jq 与 awk"（决定 559）。
6. **第 1 个提交的标题带 `docs`**（6.1）：照 6.1 写的；它不改文档文件，它的守卫读 README。
7. **闸门与说明的测试**（3.9）：`38916978` 加的格子与"没有 IMAGE"的字样；每格照 `actions/checkout` 检出 `GITHUB_SHA` 的提交，没设时检出 tag 的（审查 B 的 M2、N7，C 的 M2；决定 558）。
8. **说明的一节**（3.7 第 2 条）：只从以"）"结尾的标题起，"下一个 `## `"先判（审查 B 的 N1、N5；决定 558）。
9. **闸门的日期只核对形状**（3.6）：不改脚本（审查 B 的 N2，决定 563）。
10. **守卫**（3.9 第 2、5、6 条）：`run:` 里在每一处 `${{` 上核对（审查 B 的 M1）；`strategy` 的字符串不含 `${{`（B 的 N8）；`ci.yml` 的 actionlint 一步恰是三个键（C 的 M3、N6）；跑 smoke 的步骤与演练（B 的 N3、M3、N6，决定 560）。关掉 `bash -e` 的写法交 S2（决定 562）。
11. **`make image` 先取提交与时间，git 读不出就停**（3.4，审查 A 的 N1，`bc71d5fe`）：命令替换失败不会让一行失败，原来的写法会带着空的 label 构建出来；说明改成"需要 Docker 与 git"。`make help` 与 README `:508` 的 image-smoke 一行写上 label 与 git（A 的 N2，`e08b98b0`），`image-smoke.sh` 的头注释也是（核对 A 的 N4，`ce8aaf64`）；`test-go-short` 那一行的 `## ` 改回（A 的 M1，C 的 M1）。
12. **MCP 版本的测试分开报两种失败**（3.3，审查 A 的 N3，`719c59de`）：盖的就是 `buildinfo.go` 的默认值时报 `TEST_VERSION` 要与它不同，二进制的版本号不是盖的那个时报 `-ldflags` 没有到。
13. **用时**（3.5、7.4）：Lint workflows 一步 2–3 秒，不是推断的 15–40 秒；演练 amd64 1:18–2:21、arm64 1:21–1:26（10.1）。

### 3.25 S2 的实施对设计的修正

S2 的审查与修复见 [S2 的审查记录](reviews/P5-S2-publish-release-review.md)，结果在 10.2。与上文最初的设计不同之处已改在各节里，各处标"S2"（头的日期与状态，2.2 第 1 项，2.3 的决定 439、441，3.8 的 `build`，3.9 第 6 条，3.10 的文件表，3.11，3.12，3.13，3.14 的 `sbom`、`publish` 的 Index、`verify` 的 Tools 与 SBOM 那一行，3.15，3.22，3.23，7.2，第 8 节第 3、5–7 条，第 9 节）。写了行号的，代码是 S2 的头 `9a6b20ae`。

1. **镜像的 SBOM 的文档 `name` 是镜像引用的仓库名，不带 tag 与摘要**（3.12、3.14）：演练（run 44451246873）是 `nervewiki`；syft v1.54.0 的 `stereoscopesource/image_source.go` 的 `Describe()` 取 `reference.Parse(UserInput).Name()`，`spdxutil/helpers/document_name.go` 先取它，所以 tag 上是 `ghcr.io/open-nerve/nervewiki`（由源码推出，rc 上证明；审查 A 另读了 sbom-action v0.24.3 把 `image` 原样交给 syft）。`verify` 认 `$IMAGE`；`sbom` 作业要它等于 `$IMAGE`、演练要 `nervewiki`，前端的都要 `@nervewiki/web`：命名变了，tag 的运行停在只读的 `sbom`，不到 `verify`（决定 617）。`release.yml` 的三处注释写明这是推断。
2. **`verify` 的 Tools 多查 `gh`**（3.14）。
3. **决定 562**（3.9 第 6 条）：跑 smoke 的步骤，与 `verify` 的每个跑命令的步骤（修复第 1 轮，审查 B 的 M2），只有 `name`、`env`、`run` 三个键（`shell`、`if`、`working-directory`、`continue-on-error` 都不许）；`run` 里没有关掉选项的 `set … +x`（行首，或 `;`、`&`、`|`、`{`、`(` 之后；`set +e`、`set -u +e`、`set +o errexit` 都算）与 `shopt -o`（审查 B 的 N3）；跑它们的作业与工作流不写 `defaults.run.shell`；`rehearse`、`build`、`verify` 各恰一步 smoke。有意绕开的写法（`{ …; } || true`、`BASH_ENV`、`trap`）照 562 不追。原来"不写 `if`"一条并进了三个键。
4. **守卫另核对**（3.9 第 6 条；决定 616）：
   - `publish`：最后一步是版本号的 tag；第三方的 action 只有 cosign-installer（按前缀判）；步骤一个也不写 `if`（同 `gate`）；恰有一步 `cosign sign --yes "$IMAGE@$DIGEST"`，`DIGEST` 是 `steps.index.outputs.digest`；恰有三步 `actions/attest`，`subject-digest` 都是索引的摘要、`push-to-registry: true`，来源证明一步、两份 SBOM 各一步（审查 B 的 M1）。
   - `verify`：每一条 `gh attestation verify` 都带 `--cert-identity "$identity"`，来源证明与 SBOM 各至少一条；`identity=` 一行与唯一的一步 `cosign verify` 的 `--certificate-identity` 逐字是 tag 上的 `release.yml`，`cosign verify` 带 Actions 的签发者，整个作业里没有 `-regex`、`-regexp`（审查 B 的 N1）；矩阵恰是 amd64、arm64，`strategy` 与 `build` 的相同（审查 B 的 M2；不同时消息打出两边的 `strategy`，核对第 1 轮 B 的 N1）。
   - 只有 `release` 公开 Release：别的作业的 `run` 不含 `github-release.sh publish`、`--draft=false`、`gh release`（审查 B 的 M2）。
   - 决定 561：`build` 在构建之前核对 `HEAD` 是 `needs.gate.outputs.sha`，`publish` 核对镜像的 `revision`；这两步只有 `name`、`run`（Index 加 `id`），不写自己的 `env`、`if`（审查 B 的 N2）。
   - 三份工作流的每个 `uses:`（步骤的与作业的）都是 `<owner>/<repo>[/path]@<40 位十六进制>`（总体设计 13.5 第 2 条；审查 B 的 Q1）。
5. **`TestTheReleaseScripts`**（3.13）：版本号的 tag 9 格（多 `manifest unknown` 一格），Release 22 格（附件目录少一个与多一个分开；多：已是草稿却有别名的附件、没有说明、tag 的写法、没有 `GH_REPO`、公开之后仍是草稿、已公开却缺附件；修复第 1 轮：列表答 502、同一个 tag 两个 Release、草稿的 `edit` 失败、前一次已传三个附件时 `upload` 失败、别的动作名）。不与别处等价的每一处检查，都有一格让它单独变红（审查 C 的 M1）；等价的几处（`create` 失败不管、参数个数、rc.0、空的说明等）列在 S2 的审查记录。调用整个、按次序比，每个参数的边界也比（假命令以 NUL 分隔记参数，审查 C 的 N6）。假的 `docker`、`gh` 失败的地方分两种：照真的（对没有的 Release `view`、`edit`、`upload`；对已公开的 Release 上传，打开 immutable releases 时）；绊线，不许脚本去要（已在的 tag 再 `imagetools create`：真的并不失败，会改指；有 Release 的 tag 再 `gh release create --draft`：gh 不拦，不查已有的草稿（gh v2.103.2 的源码），接口会再建一个是 3.13 的前提，没有核过，只有 tag 的运行证明得了）。审查 C 的 M3、核对第 1 轮 C 的 N1。
6. **两个脚本比代码块多的**（3.13）：
   - `github-release.sh`：要 `GH_REPO`、核对 tag 的写法与说明不空、传过之后与公开之后各再读一次；有没有、是不是草稿读 REST 的列表（第 12 条）；可能为空的数组写成 `${a[@]+"${a[@]}"}`，bash 3.2 也能跑（审查 C 的 M2）；`paste` 写 `-`。
   - `release-version-tag.sh`：复制之后再读一次，读不到另写一句"复制之后读不到"；最后一行打出 tag 与摘要；`mktemp` 与 `trap` 分成两行。`manifest unknown` 一支是代码块里本来就有的（审查 C 的 N2）。
7. **`build` 的 HEAD 核对**（第 9 节"P5/S2"）是单独一步"The commit is the gate's"，在"The tree is clean"之后、Tools 之前（设计写"Labels 之前"）。
8. **`image-smoke.sh`**（3.11）：另核对 `README.txt` 在、`LICENSE` 与仓库的逐字节相同、npm 一侧每行三列都在；pgx 与 react 那两行带着文件名；修复第 1 轮加 `openapi-fetch` 一行与两份清单行数的下限（40、200；今天 48、233），本机一侧的底线（审查 A 的 M2）。
9. **`npm.mjs`**（3.11）：名字与版本读每个包自己的 `package.json`，不按位置配 pnpm 的 `versions` 与 `paths`。修复第 1 轮（决定 612，审查 A 的 I1、Q1）：列 `--filter "@nervewiki/web..."`，带上工作区包 `@nervewiki/api-client` 的依赖（`openapi-fetch`、`openapi-typescript-helpers`：只写 `@nervewiki/web` 时 pnpm 不往工作区包里走，`openapi-fetch` 打包进了前端，却没有它的许可证）；另固定收 tailwindcss（devDependency，preflight 与默认主题编进 CSS，从 `web/apps/web/node_modules` 读）。vite 的 modulepreload polyfill 不算（S2 的审查记录"接受与推后的"）。
10. **l1–l4**（7.2）：照 S1 修复之后的 `make image`（先取 `revision`、`created`）改写了变异的锚点。
11. **许可证文件的名字**（3.11；决定 613，审查 A 的 M1）：`^((un)?licen[cs]e|copying|notice|patents)([.-].*)?$`，不分大小写，另排除以 js、cjs、mjs、jsx、ts、cts、mts、tsx、go、json、map、sh、py 结尾的名字；`go.sh` 与 `npm.mjs` 同一条。cytoscape 的 `license-update.mjs` 不再收；只有 `license.go` 的模块让构建失败。
12. **Release 的状态读 REST 的列表**（3.13；决定 615，审查 C 的 M4）：`gh api --paginate "repos/$GH_REPO/releases?per_page=100" --jq '.[] | select(.tag_name == "<tag>") | .draft'`：任何失败都失败，同一个 tag 多于一个也失败。gh v2.103.2 的 `release view` 经 `FetchRelease` 并行查 REST（只找已公开的）与 GraphQL（找草稿），两次都失败、其中一次是 not found 时报 `release not found`：第 1 次尝试建了草稿之后重跑 `draft`、GraphQL 一时答 502，旧的写法会当作没有、再建一个草稿。列表含草稿：GitHub 文档"Only users with push access will receive listings for draft releases"，`draft`、`release` 两个作业的令牌有 `contents: write`（推断它算 push access，rc 上证明）；`--jq` 对每一页各算一次（gh v2.103.2 `pkg/cmd/api/api.go` 的分页循环每页调一次 `processResponse`，`--slurp` 不能与 `--jq` 同用）。附件仍由 `gh release view --json assets` 读（它找得到草稿，审查 C 的 N5）。
13. **清单完整的核对**（3.11、3.14、3.15；决定 614，审查 A 的 M2）：
    - `go.sh <程序> <输出目录>`：收完之后，`go list` 的模块要恰是 `go version -m <程序>` 的 `dep` 行，程序的 Go 版本要是收的那个 Go 的，不等就失败、构建失败；
    - 演练与 `sbom` 作业的 Check the SBOMs：前端 SBOM 里除 `@nervewiki/*` 之外的每个包都在 `/licenses/npm/INDEX.tsv`；不在的 `@types/` 包打出名字、不算缺（只有类型声明，构建丢掉；今天是 `@types/trusted-types`，dompurify 的可选依赖，pnpm 的 `licenses list --prod` 不走各包的可选依赖，`npm.mjs` 收不到它）。`LC_ALL=C` 之下 `sort` 与 `comm`。
14. **`tools/licenses/check.mjs`**（3.11、7.2；修复第 1 轮，实施中定）：`make lint-web` 跑它，对着假的 `go` 与假的 `pnpm`、各自一个目录的桩跑 `go.sh` 与 `npm.mjs`，10 格（每个模块的文件与 Go 本身；只有 `license.go` 的模块；不在模块缓存里；程序多一个、`go list` 多一个；程序的 Go 版本不同；Go 本身没有 PATENTS；npm 的生产依赖与 tailwindcss；tailwindcss 没装；pnpm 什么也没列）。x2 由它的一格常驻。收口（核对第 1 轮 A 的 N1）：假的 `go` 照真的每个包一行，每个模块三行、按依赖的次序，`go.sh` 的 `sort -u` 由第 1 格管着。
15. **`go.sh` 的注释**（审查 A 的 N1）：子目录一句改写为事实："只看模块的根目录：今天链接进来的包所在的子目录里没有另外的许可证（M12/P5/S2 审查 A 核过）"。

## 4. 网页

不改网页的代码与测试。

## 5. e2e

不改 e2e。MCP 的版本不进 e2e（P5-2）。

## 6. 实施步骤

每个 Step 一个分支，从前一个合并之后的 `main` 开始（S1 从当时的 `main`）；每个提交单独为绿。提交的写法照 00 号文档第 7 节"提交说明"：代码 `<范围>: <说明> (M12/P5/S<m>)`，分支上的文档 `docs: … (M12/P5/S<m>)`，合并之前的修复写到 Step，核对写 `fix check <k>`。接受本文的文档提交同时以 `apply_p5_design.py` 改 00 号文档（第 8 节第 4 条）。

### 6.1 S1（`m12-p5-s1`）

1. `server, build, docs: one default version everywhere, held by a test (M12/P5/S1)`：`version_test.go`、`Makefile:12` 的注释（3.2）。
2. `server, build: the MCP server tells the build's version, which the short run stamps (M12/P5/S1)`：`mcpOptions`、两个测试、`test-go-short` 与 README `:31`（3.3）。三样同一个提交：测试要盖了版本号才有意义。
3. `deploy, build: the image's revision, creation and description labels (M12/P5/S1)`：Dockerfile（含 `:1-3` 的注释）、`make image`（合成一行）、`image-smoke.sh`（3.4）。同一个提交：`image-smoke` 核对 `make image` 传的。
4. `build: ci.yml runs on branches, not on tags (M12/P5/S1)`（3.5）。
5. `build: actionlint lints the workflows on the runner, make lint-actions (M12/P5/S1)`：`Makefile` 的目标、`ci.yml` 的一步、README `:29`（3.5）。之后推送分支，读 `costs-1` 的 Lint workflows 一步的注解与 `-verbose` 列的文件。报出的在下一个提交 `build: the workflows pass actionlint (M12/P5/S1)` 里改；没有报出就没有这个提交。S1 报出的是 `make lint-actions` 自己的 `-format`，下一个提交是 `build: make lint-actions writes its newline bare, which actionlint unescapes itself (M12/P5/S1)`（`2db96b0a`），三份工作流 0 错，没有"pass actionlint"那个提交（决定 557）。
6. `deploy, server: the release gate and the release notes, run against a fake gh and throwaway repositories (M12/P5/S1)`：两个脚本（可执行）、`release_test.go` 的 `TestTheReleaseGate`、`TestTheReleaseNotes`；README 的"需要安装"加 jq（3.6、3.7、3.9；S1 写成"bash、git、jq 与 awk"，决定 559）。表里新加的几格先在原稿的脚本上跑，记下它们怎样失败。
7. `build, server: release.yml rehearses both architectures on every push, and gates and builds them on a tag (M12/P5/S1)`：`release.yml`（BuildKit 照 3.21 取版本与摘要）、`TestOnlyATagsJobsMayWrite`（3.8、3.9）。守卫的第 4、5 条先在没改过的 `ci.yml`、`vuln.yml` 上跑（修订核对 A 读过，应当照过），报出的同一个提交改。之后推送分支，读 `ci.yml` 与 `release.yml` 的两次运行，Lint workflows 一步也读了 `release.yml`。
8. `docs: … (M12/P5/S1)`：审查记录、10.1、照实改的各节、00 号文档与总体设计的 S1 部分、06-environment、移交（第 8 节）。

S1 实际另有第 7 个之后的 `server: the gate's and the notes' tests hold the checks the design's cases left free (M12/P5/S1)`（`38916978`，决定 558）与修复两轮的 15 个提交（[S1 的审查记录](reviews/P5-S1-version-pipeline-review.md)）。

### 6.2 S2（`m12-p5-s2`）

1. `deploy, tools, build: the image carries its third-party licenses in /licenses (M12/P5/S2)`：`tools/licenses/`、Dockerfile、`knip.jsonc`、`image-smoke.sh`（3.11）。之前先在临时的 worktree 里对两个脚本做 x2 的反向对照。
2. `build: the rehearsal takes out the licenses and builds both SBOMs, read only (M12/P5/S2)`（3.15）。之后推送，读演练的日志，记下镜像的 SBOM 的文档 `name`（3.12）。
3. `deploy, server: the version tag and the Release's draft and publication are scripts, run against a fake docker and gh (M12/P5/S2)`：两个脚本（可执行）、`TestTheReleaseScripts`（3.13）。
4. `build, server: on a tag, the SBOMs, the index, its attestations and signature, the version tag, the Release's draft, the check and the publication (M12/P5/S2)`：五个作业、守卫的表（3.14、3.9）。
5. `docs: … (M12/P5/S2)`。

### 6.3 S3（`m12-p5-s3`）

1. `deploy, server, build: deploy/compose.yaml with its database setup, make deploy-check in the image job, and the PostgreSQL guard reading it (M12/P5/S3)`：`compose.yaml`、`compose-initdb.sh`、`.gitignore`、`.dockerignore`、`Makefile`、`ci.yml`、守卫（3.17）。
2. `docs, server, deploy: deploy/README.md as the operators' manual, and the root README's deploy section as its entry (M12/P5/S3)`：`deploy/README.md`、README 的各处、守卫里 README 一项换成 `deploy/README.md`、三处注释（3.18、3.19）。同一个提交：守卫跟着那一句走。
3. `docs: CHANGELOG.md with 0.1.0's section, and SECURITY.md (M12/P5/S3)`（3.19）。
4. `docs: the screen-reader checklist and the entry to the acceptance after the release (M12/P5/S3)`（3.19）。
5. 三遍演练（3.20），照它改文档与样例的地方并进一个 `docs, deploy: … what the drills found (M12/P5/S3)`。
6. `docs: … (M12/P5/S3)`：审查记录（含两张对照表与演练的命令与输出）、10.3、00 号文档、总体设计第 11 节、13.5 第 1 条与第 14 节的 `NOTIFY` 一行、移交。

### 6.4 合并的门禁

02-workflow 第 3.1 节：`make lint gen-check knip build`、`make test`、`make test-go-short`、`make e2e`（经 `heavy.sh`；`TMPDIR` 放 `~/.cache` 下自己的目录，锁固定在 `/tmp`）；7.2 的每个变异；分支的 CI 全绿（`ci-wait.sh <sha>`，另读 `ci-wait.sh <sha> release.yml`：两个演练作业都绿）。actionlint 不在本机的门禁里，它的结论是分支 CI 的 `costs-1` 一份（3.5）。

- **镜像**：S1、S2 改了 Dockerfile 与 `image-smoke.sh`，S3 改了 `.dockerignore`、为演练构建镜像（审查 C 的 N6）：`make image`、`make image-smoke` 在普通的克隆里跑一次（06-environment 第 2 节），S1、S2 并跑 l1–l4、x1–x3。
- **合并之前**看一眼 `vuln.yml` 在 `main` 上最近一次的结论（00 号文档第 7 节 P1）。
- **S1 的分支 CI 之后**：记下两个演练作业的 run 号、各自的用时与 image-smoke 的最后一行（版本、提交、`modified=false`），Lint workflows 一步读的文件，进 10.1。

## 7. 测试与验证

### 7.1 各项与它们的测试

| 项 | 测试 | 在哪里 | Step |
|---|---|---|---|
| 四份默认值与 README 一致 | `TestTheDefaultVersionIsOneEverywhere` | `platform/buildinfo`，`short` 与 `race` | S1 |
| MCP 的 `serverInfo` 是 buildinfo 的版本 | `TestMCPServerInfoIsTheBuild`（盖了版本号）；`TestTheHandshakeNamesTheServerAndItsVersion` | `bootstrap`（第四份与 `short`）；`mcpserver`（`race` 与 `short`） | S1 |
| label | `image-smoke.sh` | CI 的 `image`、演练、`build`、`verify` | S1 |
| `ci.yml` 不在 tag 上跑；三份工作流的权限、`if`、检出、`run:` 里的表达式、BuildKit、调的脚本 | `TestOnlyATagsJobsMayWrite` | `platform/buildinfo` | S1、S2 |
| 工作流的写法 | actionlint（`make lint-actions`） | CI 的 `costs-1`；本机不跑 | S1 |
| 闸门的五条 | `TestTheReleaseGate` | `platform/buildinfo` | S1 |
| 说明 | `TestTheReleaseNotes` | 同上 | S1 |
| 两种架构的构建与 smoke | 演练作业 | `release.yml`，每次推送 | S1 |
| `/licenses` | `image-smoke.sh`；`sbom` 作业的 `diff -r` | CI 的 `image`、演练、`build`、`verify` | S2 |
| 两份 SBOM | 演练的 amd64 一份与 `sbom` 作业的 `jq` 核对 | `release.yml` | S2 |
| 版本号的 tag、Release 的草稿与公开 | `TestTheReleaseScripts` | `platform/buildinfo` | S2 |
| 推送、证明、签名、Release | `verify` 作业 | rc | S2 |
| `compose.yaml` 的写法 | `make deploy-check` | CI 的 `image` | S3 |
| PostgreSQL 镜像在 `compose.yaml` 与 `deploy/README.md` | `TestContainerMatchesTheOtherDatabases` | `pgtest` | S3 |
| 安装、升级、备份恢复；库角色；可信的代理 | 三遍演练 | 本机，S3 的记录 | S3 |

### 7.2 变异

每个变异在本机改一处，跑旁边写的命令，要它失败（不提交；改回从备份复原，不用 `git checkout --`；改坏时不跑 `make gen*`）。先在没改过的树上跑一遍同样的命令。第 10 节记下结果。Go 的用 `docs/v0.1/claude_to_codex/tools/mut.py`，改两处的写 `also`。

| # | 变异 | 让它失败的 | Step |
|---|---|---|---|
| v1 | `Makefile:13` 的默认值改成 `0.1.0-dev2` | `TestTheDefaultVersionIsOneEverywhere` | S1 |
| v2、v3 | Dockerfile 的两个 `ARG VERSION=` 各改一处 | 同上 | S1 |
| v4 | `buildinfo.go:9` 改 | 同上（`make test-go-short` 盖了版本号，读源码的写法照样抓到） | S1 |
| v5 | `README.md:43` 改 | 同上 | S1 |
| v6 | Dockerfile 多一行 `ARG VERSION=0.1.0-dev` | 同上："3 matches, want 2" | S1 |
| m1 | `mcp.go` 的 `Version: buildinfo.Get().Version` 写成 `"0.1.0-dev"`（S1 写成 `buildinfo.Get().Version[:0] + "0.1.0-dev"`：直接写字面值时 `buildinfo` 没人用，编译不过） | `make test-go-short` 里的 `TestMCPServerInfoIsTheBuild`；直接 `go test -short` 照过（写明：这就是要盖版本号的理由） | S1 |
| m2 | `Name: "nervewiki"` 写成别的 | 同上，直接 `go test` 也抓到 | S1 |
| m3 | `mcpserver/server.go:153` 的 `Version: o.Version` 写成 `o.Name` | `TestTheHandshakeNamesTheServerAndItsVersion` | S1 |
| m4 | `test-go-short` 去掉 `-ldflags`、留着 `NWIKI_TEST_VERSION` | `TestMCPServerInfoIsTheBuild` 的盖版本号一条 | S1 |
| l1 | `make image` 不传 `REVISION` | `make image-smoke`（普通的克隆） | S1 |
| l2 | `REVISION` 传 `HEAD~1` | 同上 | S1 |
| l3 | Dockerfile 去掉 `description` | 同上 | S1 |
| l4 | `CREATED` 传构建的时间 `$(date -Iseconds)`（审查 A 的 N4） | 同上 | S1 |
| w1 | `rehearse` 加 `packages: write` | `TestOnlyATagsJobsMayWrite` | S1 |
| w2 | `build` 的 `if` 写成 `github.ref_type == 'tag' \|\| always()` | 同上 | S1 |
| w3 | `build` 去掉 `if` | 同上 | S1 |
| w4 | `on` 加 `workflow_dispatch:` | 同上 | S1 |
| w5 | `ci.yml` 的 `on.push` 去掉 `branches`（又在 tag 上跑） | 同上 | S1 |
| w6 | `vuln.yml` 的作业加 `permissions: {contents: write}` | 同上 | S1 |
| w7 | `gate` 的 `make vuln` 去掉 `VULN_AUDIT_LEVEL=high` | 同上 | S1 |
| w8 | 一个 `run:` 写 `${{ github.ref_name }}`；另一次写 `${{github.ref_name}}`；另一次写 `${{ github.ref }}`（审查 A 的 M6）；另一次写 `${{ format('{0}', github.ref_name) }}`（表达式里有 `}`，S1 审查 B 的 M1） | 同上，四次都抓 | S1 |
| w9 | `publish` 的 `needs` 去掉 `sbom` | 同上 | S2 |
| w10 | `gate` 的漏洞一步加 `continue-on-error: true`；另一次加 `if: always()`（审查 A 的 M7） | 同上 | S1 |
| w11 | 一处 `actions/checkout` 去掉 `persist-credentials: false`（审查 B 的 N12） | 同上 | S1 |
| w12 | `rehearse` 的 `driver-opts` 与 `build` 的不同 | 同上 | S1 |
| w13 | `draft` 的那一步改成直接 `gh release create` | 同上 | S2 |
| w14 | `verify` 作业加 `continue-on-error: true` | 同上 | S2 |
| w15 | `release` 的 `needs` 去掉 `verify`（修订核对 B 的 N13） | 同上（"`needs` 含"） | S2 |
| w16 | `verify` 的校验一步去掉 `--cert-identity "$identity"`（同上） | 同上 | S2 |
| a1 | `make lint-actions` 去掉 `$(runner-only)`（修订核对 A 的 N11） | 本机 `make lint-actions`，PATH 最前一个只记参数的假 `go`：原样以 2 退出、假 `go` 没被调；变异之后调到了（不跑 actionlint） | S1 |
| a2 | 判断挪到单独一行 | `make -i lint-actions`，同上的假 `go`：变异之后调到了 | S1 |
| g1 | 写法的正则不要 `-rc.N` 之外的限制（`(-.*)?`） | `TestTheReleaseGate` 的 `v0.1.0-beta.1` 一格 | S1 |
| g2 | 不核对在 `origin/main` 上 | 旁支一格 | S1 |
| g3 | 查 `repos/…/actions/runs?head_sha=…`（所有工作流），**并且** `select` 去掉 `.path` 的条件（`also`；只换端点时选出的仍是红的 `ci.yml`，审查 A 的 N2） | "`ci.yml` 红、`release.yml` 绿"一格 | S1 |
| g4 | `select` 不核对 `path` | 端点答错的一格 | S1 |
| g5 | 取最早建的一次（`first`） | 两次运行的一格 | S1 |
| g6 | `[ "$conclusion" != failure ]` 就放行（审查 A 的 M1） | `in_progress` 一格 | S1 |
| g7 | 正式版本不核对日期 | `## 0.1.0（未发布）` 的正式一格 | S1 |
| g8 | 标题不要求恰好一个 | 两个标题的一格 | S1 |
| g9 | 不核对 `GITHUB_SHA` | `GITHUB_SHA` 另一个提交的一格 | S1 |
| g10 | 标题改用 `grep -cE "^## ${base}（"`（不转义；审查 A 的 M2） | `0x1x0` 一格 | S1 |
| g11 | `GITHUB_SHA` 与 `git rev-parse refs/tags/$tag`（tag 对象）比（审查 A 的 M3） | rc 放行一格（带注释的 tag，`GITHUB_SHA` 是提交） | S1 |
| g12 | 去掉 `sort_by(.created_at)` | 两次运行的一格（答复从新到旧） | S1 |
| g13 | 闸门不取一次这一节（审查 A 的 M4） | 两个"这一节是空的"格 | S1 |
| g14 | 去掉闸门的可执行位（审查 A 的 M5） | 每一格（测试直接执行它） | S1 |
| n1 | 说明的 rc 不加预发布一句 | `TestTheReleaseNotes` 的 rc（整个比） | S1 |
| n2 | 一节读到下一个 `## ` 之后（`exit` 去掉） | 同上 | S1 |
| n3 | 身份写 `refs/tags/v<base>`（rc 时写成 `v0.1.0`），两条命令一起 | 同上 | S1 |
| n4 | 空的一节不拒绝 | 只有空行的一格 | S1 |
| n5 | 镜像名写死成 `ghcr.io/open-nerve/nervewiki`（审查 A 的 N6） | rc 与正式两格（期望里是假的 `IMAGE`） | S1 |
| gc | 第 3 条与 `HEAD` 比（`--is-ancestor "$sha" HEAD`） | 旁支一格：测试照 `actions/checkout` 检出之后才抓得到（S1 审查 B 的 M2、C 的 M2） | S1 |
| gc2 | 闸门的提交取 `HEAD`，tag 只核对存在 | "`GITHUB_SHA` 是另一个提交"一格：测试检出 `GITHUB_SHA` 的提交之后才抓得到（S1 核对第 1 轮 B 的 N7） | S1 |
| cw5、cw6 | 删掉 `ci.yml` 的 Lint workflows 一步；它的 `if` 写成 `'cost-1'`；另两次给这一步加 `shell: bash -c 'exit 0' {0}`、`env: {MAKEFLAGS: n}`（cx1、cx2） | `TestOnlyATagsJobsMayWrite`（S1 审查 C 的 M3、核对第 1 轮 C 的 N6） | S1 |
| k1–k3 | 删掉 `build` 或演练的 smoke 一步；`build` 的 smoke 那一行后加 `\|\| true`；另一次 `build` 的 smoke 一步加 `if: false`（b9） | 同上（S1 审查 B 的 N3、核对第 1 轮 B 的 N6） | S1 |
| s1–s5 | 演练的 `env` 写 `${{ secrets['GHCR_PAT'] }}`、`${{ toJSON(secrets) }}`、`${{ SECRETS.GHCR_PAT }}`；顶层的 `env` 写 secret，再加 `docker --config … login`；另几次在演练里以 `docker/login-action`、`Docker/login-action` 登录（b10、l2） | 同上（S1 审查 B 的 N3、核对第 1 轮 B 的 M3 与 C 的 N7） | S1 |
| e4 | 演练的 `include` 加 `note: ${{ github.event.head_commit.message }}`，smoke 一步 `echo "${{ matrix.note }}"` | 同上（S1 核对第 1 轮 B 的 N8） | S1 |
| fn5、fn6 | 说明的"下一个 `## `"挪回起点之后；起点去掉 `&& /）$/` | `TestTheReleaseNotes` 的"同一个版本两个完整的标题""写坏的标题在前"（S1 审查 B 的 N1、核对第 1 轮 B 的 N5） | S1 |
| r1 | 版本号的 tag 已在时不比摘要 | `TestTheReleaseScripts` 的"已在、别的摘要" | S2 |
| r2 | `inspect` 的任何错误都当作不在 | `unauthorized` 一格 | S2 |
| r3 | `create` 之后不再读一次 | "`create` 之后是别的"一格 | S2 |
| r4 | Release 已公开时照样上传 | "已公开、附件齐"一格（日志里不许有 `upload`） | S2 |
| r5 | `view` 的任何错误都当作没有 | `HTTP 502` 一格 | S2 |
| r6 | 公开之前不核对附件 | "publish，草稿、缺附件"一格 | S2 |
| r7 | rc 不加 `--prerelease` | rc 的 draft 一格 | S2 |
| v-gate | `make vuln` 的 `--audit-level=$(VULN_AUDIT_LEVEL)` 写死成 `low` | PATH 里 `go`、`pnpm` 两个桩加 `GITHUB_ACTIONS=true` 跑 `make vuln VULN_AUDIT_LEVEL=high`，`pnpm` 的桩记下的参数不是 `--audit-level=high`（手做一次，审查 A 的 N3） | S1 |
| x1 | `npm.mjs` 什么也不写 | `make image-smoke`：`npm/INDEX.tsv` 是空的 | S2 |
| x2 | `go.sh` 遇到没有许可证的模块不失败 | 今天的 47 个都有，真树上抓不到；在临时目录里以 PATH 的桩 `go` 列一个空目录的模块，原样以 1 退出、变异以 0（记录写明）；S2 修复第 1 轮起由 `tools/licenses/check.mjs` 的一格常驻 | S2 |
| x3 | runtime 阶段不放 Go 的 `LICENSE` | `make image-smoke` | S2 |
| l1–l13 | `go.sh`、`npm.mjs` 收代码、不收 `unlicense`、不比模块与 Go 的版本、少收一个模块、不要 `PATENTS`；`npm.mjs` 退回 `--filter @nervewiki/web`、不收或跳过 tailwindcss、不按 `名@版本` 去重 | `tools/licenses/check.mjs` 的 10 格（S2 审查 A 的 I1、M1、M2、Q1） | S2 |
| f1–f5 | 镜像里 npm 一侧退回旧的 filter、只收 react、只收 react 与 `openapi-fetch`；Go 一侧少收一个模块、只收 pgx | `make image-smoke` 的 `openapi-fetch` 一行与行数的下限；f3、f4 让 `make image` 在 `go.sh` 一步失败（同上） | S2 |
| n1–n1c | `go.sh` 的 `go list` 之后去掉 `sort -u`、只 `sort`、只 `uniq` | `tools/licenses/check.mjs` 的第 1 格（S2 核对第 1 轮 A 的 N1） | S2 |
| b5–b24 等 | `publish` 的签名与证明删掉或写 `if`；`verify` 的校验写 `if`、`shell`，矩阵只剩 amd64；`draft` 公开 Release；身份写 `-regexp`；决定 561 的两步另写 `env`、`if`；`set +e` 在 `;` 之后、`shopt -o`；`uses:` 写 tag 或短 SHA（33 个，S2 的审查记录） | `TestOnlyATagsJobsMayWrite`（S2 审查 B 的 M1、M2、N1–N3、Q1） | S2 |
| c1–c3、c15、s1–s6 | 未知的动作、草稿的 `edit` 失败、`upload` 失败都不管；`--title` 的参数拆开；Release 的列表读不到当作没有、多于一个当作草稿、查询不按 tag 选或按前缀选、不分页、仓库取 `$GITHUB_REPOSITORY` | `TestTheReleaseScripts`（S2 审查 C 的 M1、M4、N6） | S2 |
| d1 | `compose.yaml` 的 `${NERVEWIKI_VERSION:?…}` 写成 `${NERVEWIKI_VERSIO:?…}` | `make deploy-check` | S3 |
| d2 | `migrate` 的 `secrets` 写成 `[jwtx]`（审查 C 的 M1） | `make deploy-check`（有 `--profile migrate` 时失败；去掉它时照过，记录写明） | S3 |
| p1 | `compose.yaml` 的镜像改回 18.6 | `TestContainerMatchesTheOtherDatabases` | S3 |
| p2 | `deploy/README.md` 不写 `postgres:18.7-trixie` | 同上 | S3 |

等价的、不列的：原稿的 g6"只看 `conclusion`、不看 `status`"：运行没完成时 `conclusion` 是 `null`，`[ "$conclusion" = success ]` 照样不成立，与原样等价（审查 A 的 M1；`mut-g6-no-status.sh`）。

S1 的实施者另加的（v7、w17–w27、g3b、g15–g20、n6–n12）与三位审查者、两轮核对的全部变异，在 [S1 的审查记录](reviews/P5-S1-version-pipeline-review.md)"变异与反向对照"。

### 7.3 门禁

6.4。另：

- `make gen-check` 照过（P5 不改契约）。
- `make knip`：S2 的 `tools/licenses/npm.mjs` 进根的 `entry`，否则报没人用的文件。
- 每个提交单独是绿的（6.1–6.3 的次序说明了为什么）；S1 第 5 个提交在 CI 的 Lint workflows 一步上可能是红的（3.5），记录写明。

### 7.4 CI 的余量

- **组合根**：只多 `TestMCPServerInfoIsTheBuild`，在第四份，不到 1 毫秒，不要数据库。各份照 00 号文档第 7 节"分份"，每个 Step 合并之后两次 `main` 看平均。
- **`costs-1`**：多 Lint workflows 一步，actionlint 的下载与编译推断 15–40 秒（Lint 一步今天 92–143 秒，作业时限 45 分钟）。S1 实测 2–3 秒（10.1）。
- **`short`、`race`**：多四个文件的短测试，闸门、说明与发布脚本每格建一个临时仓库或假的状态（十几条 git 命令），推断合计 3–8 秒。S1 的 `platform/buildinfo` 一个包 CI 上 short 1.1–2.5 秒、race 2.7 秒（10.1）。
- **`image`**：S2 的两层许可证，推断多 5–15 秒；S3 的 `deploy-check` 几秒（今天 Image smoke 一步 68–80 秒，时限 20 分钟）。
- **`release.yml`**：演练两个作业与 `ci.yml` 并行，不进组合根，不算进各份。推断 amd64 约 2–3 分钟、arm64 约 2–5 分钟（S1 实测 1:18–2:21、1:21–1:26，10.1）；S2 在 amd64 上加 1–2 分钟（`pnpm install` 与两份 SBOM，不缓存）。S1、S2 的记录写出分支 CI 与合并之后两次 `main` 的实数。
- **tag 的运行**：推断 `gate` 3–5 分钟（`make vuln` 0:39，`vuln.yml` 的数）、`build` 3–6 分钟、`sbom` 2–4 分钟、`publish` 1–3 分钟、`draft` 1 分钟、`verify` 3–5 分钟、`release` 1 分钟，合计约 15–25 分钟。

### 7.5 P5 怎样量

- **S1**：分支 CI 与合并之后两次 `main` 的 `costs-1` 的 Lint 与 Lint workflows 两步、`short` 的 Test 一步、两个演练作业的用时与 runner 的 CPU；本机 `make test-go-short` 改前改后各一次的用时（经 `heavy.sh`）。
- **S2**：同上的两个演练作业；`image` 的 Image smoke 一步；本机 `make image` 改前改后的用时；`/licenses` 的文件数与大小、没有文本的包。
- **S3**：`image` 一项的 Deploy check 一步；三遍演练各自的用时。

## 8. 完成标准

P5 完成，当且仅当：

1. 三个 Step 各自合并进 `main`，合并之前 6.4 的门禁全绿、审查与修复核对结束；每个 Step 合并之后两次 `main` 的组合根每一份的 Test 一步平均在 450 秒之下（06-environment 第 4 节）。
2. 第 3 节的每个测试与检查在，7.2 的每个变异被它旁边的测试抓到（m1 写明只由盖了版本号的运行抓到，x2 写明在桩上，v-gate 是手做的一次），3.22 的每个反向对照在；第 10 节记下结果。
3. **00 号文档第 3 节的证据**：
   - 第 8 条：版本号的守卫与它的反向对照（v1–v6）；MCP 的短测试与 m1–m4；`0.1.0` 的镜像里 `nervewiki version` 与 `/api/v0/instance` 是 `0.1.0` 由 `verify` 作业在 rc 与 `v0.1.0` 上核对（收尾记录）；
   - 第 9 条：演练在 S1 的分支与 `main` 上两种架构都绿的 run 号（S1 的记录）；`TestOnlyATagsJobsMayWrite` 与 w1–w16（S2 的 w15、w16）；闸门的"`ci.yml` 红、`release.yml` 绿"一格与 g3；rc 与 `v0.1.0` 的证据在收尾记录（3.23）；
   - 第 7 条：两张对照表、三遍演练的命令与结果在 S3 的记录；`make deploy-check` 在 CI 的 `image` 一项；
   - 第 10 条里 P5 的部分：`CHANGELOG.md`、`SECURITY.md`、`manual/` 的两份在；已知限制的对照表在 S3 的记录。
4. **00 号文档**（`docs/v0.1/M12-release/00-M12-design.md`）：
   - **接受本文的文档提交**：以 `apply_p5_design.py` 落进 2.1 的 24 条（连同下面几样共 66 处带标签的改动，旧文字与行号见 2.1）、第 11 节"P5 的设计审查之后的取舍"（2.3 的 23 条，正式编号 430–452）、"需要负责人确认"的两行、第 12 节的 P5 一行（`| P5 部署文档与发布流水线（S1–S3） | [05-P5](05-P5-deploy-release.md) | 进行中（设计已审查） | [设计](reviews/P5-design-review.md) | — |`）、第 13 节一行。脚本先 `--check`；P2、P3、P4 的设计提交改的第 11 节末尾、表尾与第 13 节表尾是动态找的，在 `86565dd6` 上与 P2、P3 的脚本按三种次序叠着跑过，在 `48bbfc99`（P3 的设计已提交）上 `--check` 与写入都过、与 P2 的脚本两种次序都过。头的"状态"由主持者照提交时的状态写（P2–P4 的设计提交也在改它；审查 C 的 N3）。
   - **每个 Step 的文档提交**：第 12 节的 P5 一行写到这个 Step，"审查"一格加 `reviews/P5-S<m>-<slug>-review.md`，"合并"一格写占位，合并之后在 `main` 的小文档提交里填入；第 13 节加一行。
   - **`docs: M12/P5 done (M12/P5)`**：第 12 节"完成"；头的"状态"；第 13 节。
5. **v0.1 总体设计**（`docs/v0.1/v0.1-design.md`；00 号文档"总体设计的修订"第 6、7、9 条，用脚本断言锚点恰好出现一次）：
   - **S1，13.5 加第 6 条**（`:1289` 第 5 条的末行之后，`:1291` 是 `---`；审查 C 的 N1）："6. **版本号与发布**（M12）：版本号只由 ldflags 注入，`Makefile`、`buildinfo`、Dockerfile 的两个 `ARG` 与 README 写的默认值相同，由测试核对；发布之后改为下一个补丁版本加 `-dev`。发布由推 `vX.Y.Z` 或 `vX.Y.Z-rc.N` 的 tag 触发 `release.yml`，闸门只放过 `main` 上 `ci.yml` 已绿、`make vuln` 为绿、`CHANGELOG.md` 有这一节的提交；依赖的漏洞扫描另在每天的 `vuln.yml` 里跑，actionlint 在 `ci.yml` 里，都只在 runner 上。写权限只在 tag 触发的作业里，演练只读，由测试读工作流核对。推 tag 之前问负责人；收尾之后为发布做的改动在 `m12-release` 这样的分支上，合并之后才推 tag。"S1 的文档提交照它落了（锚点在 `b34e7ac3` 上是第 5 条的 `:1291` 之后、`:1293` 的 `---` 之前）。
   - **S2，同一条末尾加**："镜像两种架构，带来源证明、两份 SBOM 的证明与签名，镜像里有第三方许可证；只打确切的版本号，不打 `latest` 与浮动的"大版本.小版本"，版本号的 tag 在证明与签名之后才打、已有的不改指；Release 先草稿，两种架构上核对过才公开。"S2 的文档提交照它落了（00 号文档第 7 节第 6 条的原文里这句前面多一个空格，落的时候不带）。
   - **S3，第 11 节**：照 00 号文档"总体设计的修订"第 7 条的五处（锚点在 `86565dd6` 的 `:919`、`:931`（最后一段；`:933` 是 `---`，审查 C 的 N1）、`:925`、`:926`、`:927`，S3 时重核恰好出现一次）；"恢复之后先 `ANALYZE`"一处的"见 deploy/README"写成链接；第 11 节另三处 `README 的部署一节`（`:923` 两处、`:929`）改为 deploy/README，写成链接 `[deploy/README.md](../../deploy/README.md)`（2.1 第 19 条）；第 14 节 `:1311` 的那一处不单独改，由下面 SSE 一行的整句替换完成（同 `19a`；修订核对 C 的 N16）。
   - **S3，13.5 第 1 条**：`` `make e2e`、`make image-smoke` 对应 e2e、image 任务。`` → `` `make e2e`、`make image-smoke` 与 `make deploy-check` 对应 e2e、image 任务；`make lint-actions` 只在持续集成里（M12）。``（审查 C 的 N9）。
   - **S3，第 14 节的 SSE 一行**（2.1 第 20 条，P5/S3 改，P4 不改；修订核对 C 的 N14）：`两份配置写在 README 的部署一节，M12 的部署文档沿用。` → `两份配置写在 [deploy/README.md](../../deploy/README.md) 的"反向代理与 HTTPS"（M12/P5/S3）。`（`48bbfc99` 上恰好出现一次；第 19 条四处里第 14 节的那一处由它完成）。
   - **S3，第 14 节的 `NOTIFY` 一行**（2.1 第 20 条，P5/S3 改，P4 不改）：`升级 PostgreSQL 大版本时复核（论证见 [M5 总设计](M5-collab-editing/00-M5-design.md) 4.10；[M12 的移交](M12-release/handoffs/M5-performance.md)第 5 项） |` → `升级 PostgreSQL 大版本时复核：步骤在 [deploy/README.md](../../deploy/README.md) 的"升级与回退"（M12/P5/S3；论证见 [M5 总设计](M5-collab-editing/00-M5-design.md) 4.10；[M12 的移交](M12-release/handoffs/M5-performance.md)第 5 项） |`。
   - **第 15 节**：每个 Step 的文档提交加一行。S1 的文档提交加了（"五处默认值"，S1 核对第 1 轮 C 的 N8）。
6. **其余的文档**：
   - **S1，`06-environment.md`**：第 1 节"`make image-smoke` 另要 curl、jq、openssl"之前加"`make test-go-short` 要 bash、git、jq（发布脚本的测试）"；第 4 节 `:49` 的"工作流是 `.github/workflows/ci.yml`。"改为"工作流有三份：`ci.yml`（每次推送与 PR）、`release.yml`（每次推送的只读演练，tag 上的发布）、`vuln.yml`（每天、手动与改了依赖清单的推送）。`ci.yml` 里同一分支的新推送会取消正在跑的那次，等它跑完再推。"；第 4 节加两条："- **发布的工作流**：`release.yml` 在每次推送上跑两个只读的演练作业（两种架构的构建与 smoke），读法 `tools/ci-wait.sh <sha> release.yml`；tag 的那次运行用 `gh run list -R open-nerve/NerveWiki --workflow release.yml --branch <tag>`（同一个提交在 `main` 上还有一次演练）。"与"- **actionlint**：`make lint-actions` 只在 runner 上跑（本机拒绝，`make -i` 也一样），结论看 `costs-1` 一份的 Lint workflows 一步的注解。"（审查 C 的 N9）。S1 的文档提交照它落了：`make test-go-short` 要的写成"bash、git、jq 与 awk"（决定 559）；actionlint 一条另加两句（`-verbose` 列出读了哪几份；actionlint 自己出错时注解只有 exit code 2，S1 审查 C 的 N3）；第 5 节 `mut.py` 一条加一句（`.mutbak` 不放在 `.github/workflows/` 里）。
   - **S1，README**：`:15-17`、`:29`、`:31`（3.1）。S1 的提交改了，另改了 `:508`（S1 审查 A 的 N2）。
   - **S2，`06-environment.md`**（S2 的文档提交）：第 1 节加 `make lint` 的网页一半要 bash（`tools/licenses/check.mjs`）；第 3 节"跑重的之前看一眼 `df`"加镜像的构建缓存一句；第 4 节"发布的工作流"加演练的 Check the SBOMs 打出什么；第 5 节 `mut.py` 一条加 `file` 可以写 `../deploy/…`。
   - **S3，README**、三处注释与两份 ignore 文件（3.16、3.19）。
7. **移交的"处理进展"**各加一行（状态仍是 `open`；引号里的链接写进移交时去掉反引号）：
   - **S1**，`handoffs/M0-P6-image-release.md`："- M12/P5/S1（<日期>，合并见 `[00-M12](../00-M12-design.md)` 第 12 节）：第 1、2 条的构建一半：`release.yml` 在每次推送上以两种原生 runner 只读地演练构建与 image-smoke，tag 上闸门之后按摘要推送、拉回来 smoke；写权限只在 tag 的作业里，由守卫读工作流核对；版本号的默认值与 MCP 的版本有守卫（`[P5 设计](../05-P5-deploy-release.md)` 10.1）。"S1 的文档提交照它落了。
   - **S2**，同一份："第 1–3 条的推送与签名一半：索引、来源证明、两份 SBOM 的证明、cosign keyless、`/licenses`、Release 先草稿、两种架构上核对过才公开；版本号的 tag 不改指；只有推 tag 才走得到，rc 在收尾之后（10.2）。"S2 的文档提交照它落了。
   - **S3**，`M0-P6-image-release.md`（第 3 条的校验说明）、`M4-performance.md`（第 2、3、10 条、第 14 条（若经负责人定为部署要求）、第 17、18 条与补记 `:490`、`:492` 的部署部分；审查 C 的 M9）、`M5-performance.md`（第 1、2、5 条）、`M7-transfer.md`（第 1、3、4 条与第 5、6 条的部署项）、`M8-history.md`（第 2、4 条的部署部分）：各一行，写进了 `deploy/README.md` 的哪一节（10.3）。

## 9. 交给后面的 Phase

- **收尾**：审查 C 管"发布流水线"（00 号文档第 7 节"收尾与发布"第 1 条）时，读本文 3.14 与 3.23；P5/S3 合并之后才合并的 P2 的已知限制补进 `CHANGELOG.md`，已知限制的对照表随之补（2.1 第 9 条）。
- **发布**：照 3.23 的九步与失败的表；`README.md:9`、`CHANGELOG.md` 的日期在第 7 步；`0.1.1-dev` 照 2.1 第 11 条。
  S2 留给 rc 核对的（3.23 第 5 步一起看；S2 审查 C 的 N5 在分支上能核的先核了）：
  - 镜像的 SBOM 的文档 `name` 在 tag 上是 `ghcr.io/open-nerve/nervewiki`（由 syft v1.54.0 的源码推出，sbom-action 原样交 `image`），`sbom` 作业先核对（决定 617）；
  - REST 的 Release 列表对 `contents: write` 的令牌含草稿（文档："Only users with push access will receive listings for draft releases"）（决定 615）；记下 runner 的 `gh --version`；
  - GHCR 对不存在的 tag：匿名读公开的包，buildx v0.38.1 答 `ERROR: …: not found`（S2 审查 C 读 `ghcr.io/actions/actions-runner:no-such-tag-p5s2c`），与假的 `docker` 同一种写法；读不到的包答 `failed to authorize: … 403 Forbidden`，不会被当作不在。登录之后读自己的私有包，rc 才证明得了；
  - `{{json .Manifest}}` 的 `.digest` 是索引的摘要：已核（S2 审查 C 读 `moby/buildkit:v0.34.2`，是 3.21 钉的那一个）；
  - `gh release view <不存在的 tag>` 答 `release not found`、以 1 退出（S2 审查 C 以真的 gh 读过）；`view`、`edit`、`upload` 都经 `FetchRelease`，它找得到草稿（gh v2.103.2 的源码，不只由二进制推出）；附件由它读；
  - `sbom` 作业在 amd64 上 `docker create --platform linux/arm64` 一个只有 arm64 清单的摘要（设计审查 B 在本机对称地核过），两种架构的 `/licenses` 逐字节相同（`diff -r`）；
  - 重跑的作业能不能下载前一次尝试的 artifact（3.23 的表）；
  - `cosign verify`（v3.1.4）对着索引的 referrers 里也有 `actions/attest` 的 bundle 时怎样（Signature 一步打出每份的类型）；
  - `publish` 的 shell `docker login` 写进 `~/.docker/config.json`，`actions/attest` 与 cosign 由此读凭证（推断 runner 没有设 `credsStore`）；arm 的 runner 自带的 gh 的版本与 `attestation verify` 的参数（S2 审查 B 未核实的第 3、4 条）。
- **收尾写的移交**，v0.2 的 `M12-release.md`（2.1 第 23 条）：actionlint 查 `run:` 的脚本（P5-6）；`outdated.sh` 不列 actionlint、syft、cosign、BuildKit；独立的二进制；构建的缓存（P5-7）；第三方许可证里没有文本的三个包（fastdom 1.0.12、react-remove-scroll-bar 2.3.8、strictdom 1.0.1；审查 C 的 M10）；rc 与 `v0.1.0` 留在 GHCR 里的 `sha-` 与没有 tag 的摘要清不清（负责人定；审查 C 的 N10，原稿放在 v0.1.1 的移交）。P5 不往 v0.1.1 的 `M12-after-release.md` 加东西。
- **P4/S3**：`deploy/README.md` 要的数在 3.18 的表里（P4 设计里引的"P5 设计 3.17"现在是 3.18，主持者已告诉 P4 的设计者），与 P4 设计第 9 节"给部署文档"对过；P4 的记录照那张表各给一个数或写"推算"。小机器一档是批次 E 另量的一组（P4-27，`GOMEMLIMIT=1536MiB`）；`reindex` 不要 P4 量（2.1 第 21 条）。
- **P4 的设计**：第 8 节不再改总体设计第 14 节的 `NOTIFY` 与 SSE 两行（P4-26；2.1 第 20 条）。
- **P5/S2**（S1 的审查交来的；S2 做了，10.2）：
  - 决定 561：`build` 在 Tools 之前单独一步核对 `HEAD` 是 `needs.gate.outputs.sha`，`publish` 的 Index 一步核对 amd64 镜像的 `revision`；守卫也核对，两步只有 `name`、`run`（Index 加 `id`），不写自己的 `env`、`if`（S2 审查 B 的 N2）。
  - 决定 562：跑 smoke 的步骤与 `verify` 的每个跑命令的步骤只有 `name`、`env`、`run`，`run` 里没有关掉选项的 `set … +x` 与 `shopt -o`，这些作业与工作流不写 `defaults.run.shell`（3.9 第 6 条、3.25 第 3 条）。
  - S1 的实施报告 9.4 的三条：跳过的矩阵作业仍不展开名字（分支的运行里是 `build (${{ matrix.arch }})` 与 `verify (${{ matrix.arch }})`），只是外观；`DOCKER_BUILD_RECORD_UPLOAD: "false"` 之后演练的 artifact 仍是 0 个（sbom-action 的 `upload-artifact: false`）；根分区与构建缓存见 10.2 与 06-environment 第 3 节。

## 10. 结果

每个 Step 合并之前在这里写它的实施、审查与验证的结果；合并的提交见 00 号文档第 12 节。

### 10.1 S1：版本号与流水线的构建（2026-11-17–18，合并见 00 号文档第 12 节）

- **提交**（分支 `m12-p5-s1`，从 `main` 的 `5e58e844`；实施 9 个，修复第 1 轮 8 个、第 2 轮 7 个，一共 24 个；合并 `main` 一次，`b34e7ac3`；文档是本提交）：相对 `5e58e844` 13 个文件，加 1,424 行、减 24 行，不加依赖。
  - 6.1 的第 1–7 个照次序：`68777b97`、`41df19a2`、`53332084`、`6504c341`、`b937d820`、`ff453005`、`96bf93e1`。
  - 第 5 个之后的 `2db96b0a`：actionlint 先把 `-format` 里的 `\n` 换成换行再解析模板，`{{"\n"}}` 成了没结束的字符串；改成裸的 `\n`（决定 557）。actionlint 对三份工作流 0 错，所以没有"the workflows pass actionlint"那个提交。
  - 最后的 `38916978`：闸门与说明的测试加了几格，让设计的表没盖住的检查各自变红（决定 558）：闸门 +4 格（另一个分支、事件、提交的更新的绿运行，缺右括号的标题），"没有 IMAGE"要闸门自己的字样；说明加了结尾的空格行与行中出现的标题。
  - 修复第 1 轮（`e08b98b0`…`a14d9ee5`）：`make help` 又列出 `test-go-short`；`make image` 在 git 读不出时停下；守卫在每一处 `${{` 上核对、守着 actionlint 那一步，跑 smoke 的步骤不写 `if`、演练不引用 secret 不登录（决定 560）；闸门的测试检出 tag 的提交；MCP 版本的测试分开报两种失败；说明在同一个版本的第二个标题处结束。
  - 修复第 2 轮（`ee8f2ab4`…`ce8aaf64`）：演练的 secret 查 `(?i)\bsecrets\b`、范围加顶层的 `env` 与 `defaults`，登录查 `(?i)login`；跑 smoke 的步骤各恰一步、那一行后面没有别的；actionlint 那一步恰是三个键；矩阵的值写死；闸门的测试检出 `GITHUB_SHA` 的提交；说明认的标题与闸门数的同一种；`image-smoke.sh` 的"需要"加 git。
  - BuildKit 钉 `v0.34.2@sha256:c5f8862f…`：`buildx-stable-1` 那时指它，v0.35.0 才发 6 天（决定 555）。3.21 的十一个 SHA 都以公开仓库核对过，相同；只有 setup-go 有更新的 v7.1.0，三份工作流照旧用 v7.0.0（决定 556）。
- **与设计不同之处**（3.24 列出）：条件提交是 `2db96b0a`；BuildKit 的版本；作业的表写成函数；README 列四样工具；`make image` 先取提交与时间；闸门与说明的测试的格子与检出；说明的一节的起止；守卫的几条；闸门的日期只核对形状。
- **门禁**：
  - 逐个提交在干净的 worktree 里，build、vet、`make lint`、`make test-go-short`、gen-check、knip 全绿：实施的 9 个 `test-go-short` 46–47 秒，修复的 15 个 40–42 秒。
  - 头 `38916978`：`make lint gen-check knip build` 绿；`make test-go-short` 52 秒，改前（`main` 的 `cfa31e00`）45 秒；`make image-smoke`（普通的克隆）绿，label 逐字相同；`make test-go` 185 个包、`make test-web` 4,694 个、`make e2e` 576 个都绿。
  - 修复两轮的头 `a14d9ee5`、`ce8aaf64`：`make lint`、`make test-go-short`（94 个包）、`make image-smoke`（18 秒）绿。
- **变异**：7.2 里 S1 的都抓到。
  - m1 只由盖了版本号的运行抓到，直接 `go test -short` 照过；等价的 g6x 照过。
  - v-gate、a1、a2 是手做的，PATH 里放假的 `go`、`pnpm`，没有跑 actionlint。
  - l1–l4 都抓到；l1 在清了构建缓存之后从头构建，171 秒。
  - 自加的 v7、w17–w27、g3b、g15–g20、n6–n12 都抓到；其中 g15–g18、g20、n8、n12 是 `38916978` 补了格子之后才抓到的。
  - `mut.py` 的备份在 `.github/workflows/` 里会被守卫当作第四份工作流，w 组改用把备份放在别处的脚本重跑。
  - 审查与两轮核对的变异照过的，各成一条发现（7.2 加的 gc、gc2 等几行），修复之后都抓到；核对第 2 轮照过的、关掉 `bash -e` 的写法交 S2（第 9 节，决定 562）。
- **分支 CI**（每次第一次 attempt，没有重跑）：
  - `b937d820` 的 `ci.yml`（run 44068674790）红在 costs-1 的 Lint workflows：`-format` 解析不了，注解只有 "Process completed with exit code 2"；`2db96b0a` 改了。
  - `96bf93e1`：`ci.yml` run 44073973273，19 项全绿；`release.yml` run 44073973297，两个演练作业绿，最后一行都是"image-smoke: nervewiki:rehearsal 通过（版本 0.0.0-rehearsal.1，提交 96bf93e19b9b3975cc631b9438ed8b0ef9e0c4a2，modified=false）"。
  - 实施的头 `38916978`：`ci.yml` run 44077699226、`release.yml` run 44077699224；修复第 1 轮的头 `a14d9ee5`：44164542578、44164542691；修复第 2 轮的头 `ce8aaf64`：44177949348、44177952360。都是 19 项全绿、两个演练绿。
  - costs-1 的 Lint workflows：`-verbose` 列的是 `vuln.yml`、`release.yml`、`ci.yml` 三份，0 错。
  - `a14d9ee5` 的推送改了 `Makefile`，`vuln.yml` 跑了一次：run 44164542725，push，success；`ce8aaf64` 没改 `Makefile` 与依赖清单，没有 `vuln.yml` 的运行。
  - 演练两种架构（`uname -m` 是 `x86_64`、`aarch64`），令牌只有 `Contents: read`，没有登录的一步，artifact 0 个；`gate`、`build` 在分支上跳过。没有推任何 tag、镜像，没有建 Release。
- **用时**（7.5）：
  - 演练：amd64 1:18–2:21（`96bf93e1` 1:18；`38916978` 2:21，其中 setup-buildx 一步 36 秒；`a14d9ee5` 1:29；`ce8aaf64` 1:40），推断 2–3 分钟；arm64 1:21–1:26，推断 2–5 分钟。演练的 runner 两种都是 4 个 CPU、约 15.6 GiB（审查 C 读 `docker info`；CPU 型号读不到）。
  - costs-1 的 Lint workflows 一步 2–3 秒（推断多 15–40 秒；`go run` 的下载加编译在 Lint 一步之后，构建缓存已热，审查 C）；Lint 一步 93–112 秒，同 `main` 的 93–111 秒。
  - short 一份的 Test 一步：`38916978` 197 秒（runner 的 CPU 是 AMD EPYC 9V74），`a14d9ee5` 249 秒（AMD EPYC 7763），`ce8aaf64` 163 秒（Intel Xeon 6975P-C）；`main` 两次 178、196 秒。`internal/bootstrap` 一个包 101、129、78 秒，`platform/buildinfo` 1.8、2.5、1.1 秒：差在 runner，不在这个 Step 的测试。
  - `platform/buildinfo` 一个包（闸门、说明与守卫三个测试在里面）：CI 上 short 1.1–2.5 秒、race 2.7 秒（run 44077699226），本机 1.3–2.2 秒；推断三个合计 3–8 秒。
  - 组合根八份的 Test 一步：三个头的运行 231–388 秒，都在 450 秒之下；S1 只在第四份加一个不到 1 毫秒的测试。
  - 本机 `make test-go-short`（经 `heavy.sh`）：改前 45 秒，头 52 秒，逐个提交 40–47 秒；盖版本号只多一次编译，这点差别在噪声之内（推断）。
  - 合并之后两次 `main` 的组合根与这几步，主持者在记合并号的文档提交里补。
- **审查**：三份（A 版本号、MCP、镜像 0I 1M 3N；B `release.yml`、闸门与说明 0I 2M 4N；C `ci.yml`、actionlint、变异、提交与文档 0I 4M 5N；Minor 去重 5 条）；核对两轮（第 1 轮 A 2N、B 1M 5N、C 3N；第 2 轮 A 1N、B 2N、C 1N，其中三位的各一条是同一条）。主持者的决定 555–563。见 [S1 的审查记录](reviews/P5-S1-version-pipeline-review.md)。

### 10.2 S2：推送、签名、证明、SBOM、许可证与 Release（2026-11-19，合并见 00 号文档第 12 节）

- **提交**（分支 `m12-p5-s2`，从 `main` 的 `ab766f02`；实施 4 个，修复第 1 轮 3 个，收口 1 个，一共 8 个；合并 `main` 一次，`2ea85cd5`；文档是本提交）：相对 `ab766f02` 12 个文件，加 1,716 行、减 26 行，不加依赖。
  - 6.2 的第 1–4 个照次序：`d0876318`、`785d48be`、`54099f3e`、`e5ecae06`。第 4 个在推送之前改过一次：第 2 个的演练读出了镜像的 SBOM 的文档 `name`（3.25 第 1 条）。
  - 修复第 1 轮：`dcaa7fe3`（`/licenses` 收齐、不收代码、对着二进制与前端的 SBOM 核对）、`e6f535aa`（守卫）、`0fe3d663`（Release 的状态读 REST 的列表、bash 3.2、假命令记下每个参数的边界）。
  - 收口 `9a6b20ae`：核对第 1 轮的三条 Nit（假的 `go` 每个包一行；守卫的消息写出 `strategy` 的差别；"gh 会再建一个草稿"写成设计 3.13 的前提）。
  - `/licenses`：Go 一侧 47 个模块加 Go 本身，与二进制的 `go version -m` 逐个相同；npm 一侧 233 个包（`pnpm --filter "@nervewiki/web..." licenses list --prod` 232 个，加 tailwindcss），没有许可证文本的 3 个（fastdom 1.0.12、react-remove-scroll-bar 2.3.8、strictdom 1.0.1）；293 个文件、532,441 字节（本机 arm64 的镜像，与本机直接跑两个脚本的输出 `diff -r` 相同；实施时 291 个、530,297 字节：多了三个包的 `LICENSE`，少了 cytoscape 的 `license-update.mjs`），演练打的包 74,693 字节（实施时 79,261）。
  - 两份 SBOM（演练）：镜像的 56 个包、785,752 字节，文档 `name` 是 `nervewiki`；前端的 235 个包、约 378 KB，`name` 是 `@nervewiki/web`。都在 1 MB 以下（3.12 的推断成立）。前端 SBOM 里 `@nervewiki/*` 之外的 233 个包，不在 `/licenses/npm/INDEX.tsv` 的只有 `@types/trusted-types@2.0.7`（run 44484245064 的 Check the SBOMs 一步）。
  - 3.21 的三个新 action 以 `git ls-remote` 从公开仓库再核：`actions/attest` v4.2.2 `1e69f48a…`、`anchore/sbom-action` v0.24.3（tag 对象 `fffec3d8…` 解开是 `66cbf4bc…`）、`sigstore/cosign-installer` v4.1.3 `756fe2d2…`，与表相同，都是最新版；`actions/download-artifact` v8.0.2 `9000827c…` 也相同。sbom-action v0.24.3 的 `src/SyftVersion.ts` 是 `v1.54.0`，cosign-installer v4.1.3 的 `cosign-release` 默认 `v3.1.4`（`gh api` 读的；审查 B 另核）。
- **与设计不同之处**（3.25）：镜像的 SBOM 的 `name` 是 `$IMAGE`，演练与 `sbom` 作业先核对；`verify` 的 Tools 查 `gh`；决定 562 写成"只有 name、env、run"，并扩到 `verify` 的每个跑命令的步骤；守卫另核对决定 561 的两步、`publish` 的签名与三份证明、`verify` 的身份与两种架构、只有 `release` 公开、`uses:` 钉 SHA；`TestTheReleaseScripts` 31 格；`github-release.sh` 读 REST 的列表；`/licenses` 的 npm 一侧取工作区包的依赖与 tailwindcss，文件名不收代码，两侧对着二进制与前端的 SBOM 核对，`tools/licenses/check.mjs` 对着桩跑；`build` 的 HEAD 核对在 Tools 之前；`image-smoke.sh` 多核对几样。主持者的决定 612–617。
- **门禁**：
  - 逐个提交（实施 4 个、修复 3 个）在干净的 worktree 里，build、vet、`make lint`、`make test-go-short`、gen-check、knip 全绿：实施的 48–49 秒，修复的各 50 秒。
  - 修复第 1 轮的头 `0fe3d663`：`make lint gen-check knip build` 绿；`make test-go-short` 49 秒；`make test-go` 191 个 `ok`、没有 `FAIL`（3,680 秒）；`make test-web` 197 个文件、4,801 个测试；`make e2e` 582 个通过、1 个跳过。实施的头 `e5ecae06`：`make test-go-short` 53 秒，改前（`main` 的 `ab766f02`）47 秒；`make test-go` 191 个 `ok`（3,673 秒），`make test-web`、`make e2e` 同上。
  - `make image-smoke`（普通的克隆）在两个头上都绿，`0fe3d663` 的最后一行"image-smoke: nervewiki-p5s2:head 通过（版本 0.0.0-p5s2，提交 0fe3d66397dc7e5dc5e9ce3a7faaa3f99e5975b2，modified=false）"。`make image` 15 秒（构建缓存是热的）；镜像 71,499,968 字节（arm64）。
  - `bash:3.2`（3.2.57）的容器里 `TestTheReleaseScripts` 31 格全过；`github-release.sh` 退回 `"${prerelease[@]}"` 时在那里红（只手做，不进 CI）。`tools/licenses/go.sh` 在同一个容器里对着假的 `go` 跑过三种情形。
  - 收口 `9a6b20ae` 照负责人 2026-11-19 的规矩只跑相关的：`make lint`、`make knip`、`make test-go-short`（48 秒）、`platform/buildinfo`、`tools/licenses/check.mjs` 的 10 格都绿；没改 Dockerfile，不跑 `make image`。合并 `main`（`2ea85cd5`）之后 `make lint`、`make test-go-short`（49 秒）绿。
- **变异**：7.2 里 S2 的（w9、w13–w16、r1–r7、x1–x3）都抓到；x2 在桩上（`tools/licenses/check.mjs` 的一格；真的 47 个模块都有许可证）。
  - 守卫：实施时加的 44 个与修复第 1 轮的 33 个（审查 B 的 b5–b24、q1，另加的 p1–p5、v1、v2、b15d–b15f、q1b–q1d、n1、n1b），每个都由该红的那一条抓到；修复之前的守卫上那 33 个都照过。S1 改工作流的 71 个对着新的守卫再跑，都抓到。
  - `TestTheReleaseScripts`：实施时的 30 个再跑，28 个抓到，r5 的代码已删（由 s1 代替）、r15 的锚点随写法改了（r15b 抓到）；修复第 1 轮的 c1–c3、c15、s1–s6 都抓到。b32（`"${prerelease[@]}"`）在 bash 5 上照过，在 bash 3.2 上红。
  - `/licenses`：`tools/licenses/check.mjs` 的 10 格，l1–l13 各让该红的格红；修复之前的两个脚本上 10 格全红。镜像的 f1（`npm.mjs` 退回 `--filter @nervewiki/web`）、f2（npm 一侧只剩 react）由 `make image-smoke` 的 `openapi-fetch` 一行抓到，f5（npm 一侧只剩 react 与 openapi-fetch）由行数的下限抓到；f3（Go 一侧少一个模块）、f4（Go 一侧只剩 pgx）让 `make image` 在 `go.sh` 一步失败；f1 另由 Check the SBOMs（本机照工作流的原文、`bash -e` 跑）报出 `openapi-fetch@0.17.0`、`openapi-typescript-helpers@0.1.0`。实施时的镜像变异 13 个（l1–l4、x1、x1b、x3、x3b–x3d、x4）没有重跑。
  - 收口：`go.sh` 去掉 `sort -u`、只 `sort`、只 `uniq`（n1–n1c）在新的桩上让第 1 格红；`verify` 只改 `fail-fast`（n7）红在打出两边 `strategy` 的消息上。
- **分支 CI**（每次第一次 attempt）：
  - `785d48be`（6.2 第 2 个之后）的 `ci.yml` run 44451246879 十九项全绿、`release.yml` run 44451246873 两个演练绿（amd64 2:14、arm64 1:19）；`e5ecae06` 的 `ci.yml` run 44461178186 十九项全绿、`release.yml` run 44461178210 两个演练绿（amd64 1:57、arm64 1:19）。
  - 修复第 1 轮的头 `0fe3d663`：`ci.yml` run 44484245001 十九项全绿（web 的 Lint 一步里 `tools/licenses/check.mjs` 10 格过）；`release.yml` run 44484245064 两个演练绿（amd64 1:40、arm64 1:24），最后一行都是"image-smoke: nervewiki:rehearsal 通过（版本 0.0.0-rehearsal.56，提交 0fe3d66397dc7e5dc5e9ce3a7faaa3f99e5975b2，modified=false）"；amd64 的 Check the SBOMs 打出"前端 SBOM 里 @nervewiki/* 之外的 233 个包，不在 /licenses/npm/INDEX.tsv（233 行）的只有 @types/：@types/trusted-types@2.0.7"；`/licenses` 293 个文件、532,441 字节，与本机 arm64 的镜像相同。
  - 收口 `9a6b20ae`：`ci.yml` run 44490478831 十九项全绿；`release.yml` run 44490478847 两个演练绿（amd64 1:59、arm64 1:29）。
  - 本提交（与合并 `main` 一起推送）：`（待填：本提交的分支 CI）`。
  - costs-1 的 Lint workflows 读了三份、0 错；`release.yml` 的运行 artifact 都是 0 个。
  - 演练加的三步（与修复第 1 轮加的核对）在 amd64 上 15–22 秒（`785d48be` 22 秒，`e5ecae06` 15 秒，`0fe3d663` 15 秒；推断 1–2 分钟）。
  - `ci.yml` 的 `image` 一项的 Image smoke 一步（7.5）：`785d48be` 82 秒、`e5ecae06` 81 秒、`0fe3d663` 86 秒；`main` 上 `ab766f02`（还没有许可证的两层）81 秒、`105ff7c8` 59 秒。推断的"多 5–15 秒"与 runner 之间的差别分不开。
  - short 一份的 Test 一步 2:52–2:54（`main` 的 `ab766f02` 3:16、`105ff7c8` 2:53）；组合根八份的 Test 一步都在 450 秒之下（最长 `e5ecae06` 的 bootstrap-1，378 秒），S2 不改组合根。合并之后两次 `main` 的组合根与这几步，主持者在记合并号的文档提交里补。
- **第 9 节"P5/S2"的五条**：决定 561、562 做了（守卫也核对）；跳过的矩阵作业仍不展开名字：分支的运行里是 `build (${{ matrix.arch }})` 与 `verify (${{ matrix.arch }})`；`DOCKER_BUILD_RECORD_UPLOAD: "false"` 之后演练的 artifact 仍是 0 个（sbom-action 的 `upload-artifact: false`）；磁盘：根分区开工 24G，实施的 15 次构建（两次 `make image`、13 个镜像的变异）之后 17G，Docker 的构建缓存从约 10G 到 17.6G（共用的，没动），平均每次构建多留约 0.5 GB；修复第 1 轮开始与结束都是 18–19G（一次 `make image` 与 5 个镜像的变异）。
- **没有推任何 tag、镜像，没有建 Release，没有写 GHCR**：tag 的七个作业在分支上都跳过；分支 CI 与演练没有红在权限或"action 不被允许"上，`anchore/sbom-action` 在演练里跑过（`docker/login-action`、`actions/attest`、`sigstore/cosign-installer`、`actions/download-artifact` 只在 tag 的作业里，还没跑过）。
- **审查**：三份（A `/licenses` 与演练 1I 2M 1N 1Q；B 五个作业与守卫 0I 2M 4N 1Q；C 脚本、提交、变异与文档 0I 4M 6N 1Q）；核对第 1 轮 A、B、C 各 1N，照负责人 2026-11-19 的规矩不再开核对，收口一个提交、主持者核对。主持者的决定 612–617。见 [S2 的审查记录](reviews/P5-S2-publish-release-review.md)。

## 附录：核对过的代码与公开的东西

在 `86565dd6` 上读过：

- `Makefile:12-14`、`:83-94`、`:151-156`、`:158-165`、`:168-169`、`:171-193`、`:272-365`（八份的模式）、`:593-600`。
- `deploy/Dockerfile:1-60`（`:1-3`、`:7`、`:23`、`:26`、`:30`、`:32`、`:34`、`:44-49`）；`deploy/image-smoke.sh:1-188`（`:15` 的 `$repo`）；`deploy/compose.dev.yaml:1-30`；`deploy/runtime-grants.sql:1-40`；`.gitignore`、`.dockerignore`。
- `.github/workflows/ci.yml:1-334`（`:3-5`、`:13-14`、`:145-171`、`:237-250`、`:318-334`）；`.github/workflows/vuln.yml:1-50`。
- `platform/buildinfo/buildinfo.go:1-45`、`buildinfo_test.go:1-48`、`toolchain_test.go:1-84`。
- `bootstrap/mcp.go:80-93`、`:143-148`；`bootstrap/wire.go:324`；`bootstrap/batch_cost_test.go:63`；`modules/page/mcp_entry.go:15-26`；`platform/httpserver/api.go:95-104`；`platform/httpserver/mcpserver/server.go:35-49`、`:146-160`；`mcpserver/harness_test.go:120-124`、`:224-250`；`mcpserver/handshake_test.go:32-63`。审查 A 另读了 `cmd/nervewiki/main_test.go:31`、`bootstrap/api_test.go:64`、`mcp_virtual_test.go:272`、`platform/config/load.go:137-145`。
- `platform/postgres/pgtest/image_test.go:1-46`；`instance/adapter/buildinfo/source.go:15-16`；`instance/domain/info.go:15`；`api/modules/instance.yaml:80`；`api/dist/openapi.yaml:3293`；`instance/adapter/http/gen/server.gen.go:63`；`web/packages/api-client/src/schema.gen.ts:2211`；`platform/httpserver/apitest/apitest_test.go:19`、`:110`。
- `README.md:9`、`:13-18`、`:29-31`、`:43`、`:45`、`:104-108`、`:229`、`:498-633`；`v0.1-design.md:917-933`、`:1283-1291`、`:1311-1312`；`docs/v0.1/claude_to_codex/tools/ci-wait.sh:1-41`；`06-environment.md:1-20`、`:45-53`；`knip.jsonc:1-25`；`server/go.mod:28`；`server/.golangci.yml` 的 depguard。
- `bootstrap/runtime_role_test.go:31`、`:464` 的注释。
- 00 号文档（`86565dd6`）：2.1 引的每一行；"需要负责人确认"`:1362-1380`；第 12 节 `:1381-1390`；第 13 节 `:1392-1416`。
- `m12/p4-draft.md`（只读）：B2 `:359-369`、第 8 节 `:856`、第 9 节 `:868-875`。

公开的（2026-11-16，只读）：

- `git ls-remote` 读的 tag 与提交：3.21 的表（审查没能复核，P5/S1 实施时核对）；actionlint 最新 v1.7.12（审查 A 复核），cosign 最新 v3.1.4，syft 最新 v1.57.1。
- `actions/attest` v4.2.2：README（输入、权限、16 MB、GHCR 的例子），`src/sbom.ts`（SPDX 的证明类型）。
- `anchore/sbom-action` v0.24.3：`action.yml`（`upload-artifact`、`upload-release-assets` 默认开），`src/SyftVersion.ts`（v1.54.0）。
- `sigstore/cosign-installer` v4.1.3：`action.yml`（`cosign-release` 默认 v3.1.4）。
- `docker/build-push-action` v7.4.0：`action.yml` 的输入与输出。
- actionlint 的 CHANGELOG（`artifact-metadata`、`ubuntu-24.04-arm`）。
- GitHub 文档：Container registry（默认私有；`GITHUB_TOKEN` 推的包自动关联；之前别的凭证推过的同名包它推不了；多架构的说明读索引的注解）；immutable releases（公开之后 tag 与附件不能改，标题与说明能改；先草稿、传附件、再公开）。
- Docker 文档：`imagetools create`（单个来源是索引时原样复制；注解要 OCI 的索引）。
- arm 的 runner 的软件表（没有列 openssl）。
- 公开的接口：`ci.yml` 在 `main` 上 `86565dd6` 的运行（字段，`m12/p5-design/ci-runs.json`）；43745380462 的运行对象与 `/attempts/1`；五次 `main` 的 `image` 与 `costs-1` 的步骤用时。
- 本机：gh 2.103.2 的 `attestation verify`、`release create/view/edit/upload` 的帮助（`--format json`、`--jq`、`--cert-identity`、`isDraft`、`isImmutable`）。
