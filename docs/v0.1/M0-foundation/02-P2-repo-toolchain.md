# M0/P2 仓库与工具链：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M0/P2 仓库与工具链 |
| 状态 | 进行中 |
| 基线 | `610d0c7`（设计文档与 `tools/md-fixtures/`，没有代码） |
| 上级文档 | [M0 总设计](00-M0-design.md) 第 7 节 |

---

## 1. 基线

仓库里有设计文档和 P1 留下的 `tools/md-fixtures/`：61 个提取样例、4 个改写样例，以及两个 Node 脚本，`check.mjs` 和 `obsidian/verify.mjs`。此外没有代码、没有工具链配置、没有持续集成。

P1 对本 Phase 的要求（M0 总设计"P1 结论对 M0 各 Phase 的要求"）：
- 开发用 compose 以 builtin `C.UTF-8` 初始化 PostgreSQL；
- 持续集成运行样例集自检；
- `.gitignore` 忽略 `.playwright-mcp/` 这类工具产物。

## 2. 目标与范围

**目标**：一个几乎为空的仓库，本地与持续集成的全部门禁都能跑通并且为绿；之后每个 Phase 只往里加代码，不再搭工具链。

**做**：
- 根目录文件：`LICENSE`、`README.md`（项目简介与开发环境）、`.editorconfig`、`.gitattributes`、`.gitignore`、`.node-version`、`Makefile`。
- Go：`server` 模块、`server/.golangci.yml`，以及第一个包 `internal/platform/buildinfo`。
- Node：根目录的 `package.json`、`pnpm-workspace.yaml`，oxlint、oxfmt、knip 的配置。
- 开发用 compose：`deploy/compose.dev.yaml`。
- 持续集成：`.github/workflows/ci.yml` 的 `server` 与 `web` 两个任务。

**不做**：
- 其余平台层与命令行（P3）；接口契约与代码生成（P4）；前端（P5）；端到端测试与镜像（P6）。
- `make run`、`gen`、`build`、`e2e` 这些命令，随对应的 Phase 加入。

**与 M0 总设计的差异**（本 Phase 完成时同步修订 00 号文档）：

| 总设计写的 | 本 Phase 的做法 | 理由 |
|---|---|---|
| P2 建 `server/tools` 模块 | 推迟到 P4 | 它要放的是代码生成工具（oapi-codegen、bodyshapegen），P4 才有；空模块没有可验证的内容 |
| P2 引入 turbo | 推迟到 P5 | turbo 编排的是工作区包的任务，P5 才有第一个包；P2 只有根目录的脚本，由 Makefile 直接调用 |
| `buildinfo` 在 P3 拷贝 | 提前到 P2 | 门禁需要一个真实的 Go 包来检查；它没有依赖，P3 的其他包都用得到它 |

## 3. 设计

### 3.1 文件与来源

"拷贝即接管"：从 Nerve 拷贝的文件逐个审查，改名、删掉 Plane 与 River 相关的内容和指向 Nerve 文档的引用。

| 文件 | 来源 | 处理 |
|---|---|---|
| `LICENSE` | Nerve | AGPL-3.0 全文，原样 |
| `README.md` | 新写 | 项目简介、开发环境、常用命令；之后每个 Phase 补充自己的部分 |
| `.editorconfig` | Nerve | 去掉 River 迁移的条目；新增样例集的条目（3.3） |
| `.gitattributes` | 新写 | 样例集不做换行符转换（3.3） |
| `.gitignore` | Nerve | 去掉参考代码目录；前端产物随 P5 加入；新增 `.playwright-mcp/` |
| `.node-version` | Nerve | `24` |
| `Makefile` | Nerve | 只保留本 Phase 用得到的命令（3.4） |
| `package.json`、`pnpm-workspace.yaml` | Nerve | 只有根目录；Plane 的 catalog、overrides、补丁全部不要 |
| `.oxlintrc.json`、`.oxfmtrc.json`、`knip.jsonc` | Nerve | 去掉 Plane 的路径；oxlint 从零警告起步，不设警告上限 |
| `server/go.mod`、`server/.golangci.yml` | Nerve | 模块路径 `github.com/open-nerve/NerveWiki/server`；depguard 只保留本项目适用的禁用项 |
| `server/internal/platform/buildinfo` | Nerve | 拷贝、改名 |
| `deploy/compose.dev.yaml` | Nerve | 改名；建库参数按 P1 结论；默认端口改为 55433 |
| `.github/workflows/ci.yml` | Nerve | 只保留 `server`、`web` 两个任务，内容按本 Phase 的命令裁剪 |

### 3.2 版本

取 2026-09-30 的最新稳定版：

| 工具 | 版本 | 说明 |
|---|---|---|
| Go | 1.27.1 | `go.mod` 的 `go 1.27` 与 `toolchain go1.27.1`；本机 Go 1.26 以上、`GOTOOLCHAIN=auto` 时自动下载 |
| golangci-lint | 2.14.0 | `make tools` 装到 `./bin`，版本写在 Makefile |
| Node | 24（LTS） | Node 26 仍是 Current，10 月下旬才成为 LTS；那之后单独评估升级 |
| pnpm | 12.8.1 | 由 `package.json` 的 `packageManager` 锁定，经 corepack 启用 |
| oxlint / oxfmt / knip | 1.86.0 / 0.71.0 / 6.38.0 | 精确版本 |
| PostgreSQL | 18.6 | 开发用 compose；与 P1 实验相同 |
| GitHub Actions | checkout v7、setup-go v7、setup-node v7、cache v6 | |

### 3.3 样例集的字节保护

样例集的输入是逐字节的规范，包括 CRLF、BOM、行尾空格和缺失的末尾换行，任何工具都不能改动它们：

- `.gitattributes`：`tools/md-fixtures/cases/**` 与 `tools/md-fixtures/rename/**` 设为 `-text`。否则在 `core.autocrlf=true` 的 Windows 上检出时，LF 会被换成 CRLF，全部字节偏移都会错。
- `.editorconfig`：这两个目录的 `end_of_line`、`charset`、`trim_trailing_whitespace`、`insert_final_newline` 设为 `unset`，编辑器不改动换行、BOM 和空白。
- `.oxfmtrc.json`：忽略这两个目录。

### 3.4 Makefile

所有命令的入口，兼容 macOS 自带的 GNU Make 3.81。按工具链分区：`*-go` 只需要 Go，`*-web` 需要 Node（先执行 `pnpm install`）。

| 命令 | 内容 |
|---|---|
| `help`（默认） | 列出所有命令 |
| `dev-db` / `dev-db-down` / `dev-db-reset` | 启动（等待就绪）/ 停止 / 停止并删除数据卷 |
| `tools` | 把锁定版本的 golangci-lint 装到 `./bin` |
| `lint` | 依次执行 `lint-go`、`lint-web` |
| `lint-go` | golangci-lint（含格式检查） |
| `lint-web` | 样例集自检（`node tools/md-fixtures/check.mjs`）、oxlint（零警告）、oxfmt 格式检查 |
| `knip` | 未使用的文件、导出与依赖；配置里过时的条目也算失败 |
| `test` | `server` 下的 `go test -count=1 ./...` |

### 3.5 Go

- 模块 `github.com/open-nerve/NerveWiki/server`，`go 1.27`、`toolchain go1.27.1`。
- `.golangci.yml`（v2）：
  - `standard` 一组（errcheck、govet、ineffassign、staticcheck、unused）；
  - `depguard`：禁用第三方 uuid 库、viper、`pkg/errors`、`log`；oapi-codegen 运行时的规则随 P4；
  - `gochecknoinits`：禁止 `init()`，对应总体设计 8.1；
  - 格式：`gofmt` 与 `goimports`，本地前缀 `github.com/open-nerve/NerveWiki`。
- `buildinfo`：版本号默认 `0.1.0-dev`，发布时用 `-ldflags -X` 注入；提交信息取自 Go 工具链嵌入的 VCS 信息。

### 3.6 Node

- `package.json`：`private`、`AGPL-3.0-only`、`engines.node ^24`、`packageManager` 锁定 pnpm。脚本：
  - `check:lint`：`oxlint --max-warnings=0 tools`；
  - `check:format` / `fix:format`：oxfmt 作用于 `tools`、根目录的 JSON 与 YAML 配置。
- `pnpm-workspace.yaml`：`packages` 列出 `web/apps/*`、`web/packages/*`、`e2e`（目录随 P5、P6 出现）；供应链相关的设置放在这里：
  - 新发布的包要等一段时间才能安装（`minimumReleaseAge`）；
  - 默认不运行依赖的安装脚本，需要的逐个放行。
- `.oxlintrc.json`：插件与规则沿用 Nerve（correctness、suspicious、perf 三类，禁用易误用的全局变量），去掉 Plane 路径；以 `--max-warnings=0` 运行，任何警告都是失败。
- `knip.jsonc`：`tools/md-fixtures/check.mjs` 由 Makefile 调用，`obsidian/verify.mjs` 由人手动运行，`package.json` 的脚本里都看不到，登记为入口。

### 3.7 开发用 compose

```yaml
name: nervewiki-dev
services:
  db:
    image: postgres:18.6
    environment:
      POSTGRES_USER: nervewiki
      POSTGRES_PASSWORD: nervewiki
      POSTGRES_DB: nervewiki
      POSTGRES_INITDB_ARGS: --locale-provider=builtin --locale=C.UTF-8
    ports: ["127.0.0.1:${NWIKI_DEV_DB_PORT:-55433}:5432"]
```

- 默认端口 55433：Nerve 的开发库用 55432，两个项目常在同一台机器上同时开发。
- 数据卷挂在 `/var/lib/postgresql`：PostgreSQL 18 的镜像把数据目录改到了 `/var/lib/postgresql/18/docker`。
- 健康检查用 `pg_isready`，`make dev-db` 用 `--wait` 等待就绪。

### 3.8 持续集成

- 触发：`push` 与来自 fork 的 `pull_request`（同仓库分支的 PR 已由 `push` 跑过）。同一分支上较早的运行被取消；`main` 的每次运行各自保留。
- `server` 任务：`setup-go`（版本取自 `server/go.mod`）→ `make lint-go` → `make test`。
- `web` 任务：`setup-node`（版本取自 `.node-version`）→ `corepack enable` → 按锁文件缓存 pnpm 的包存储 → `pnpm install --frozen-lockfile` → `make lint-web` → `make knip`。
- 权限只读（`contents: read`）；每个任务设超时。

## 4. 实施步骤

在分支 `m0-p2-repo-toolchain` 上：

1. 根目录文件：`LICENSE`、`.editorconfig`、`.gitattributes`、`.gitignore`、`.node-version`；核对样例集的属性生效。
2. Go：`server/go.mod`、`.golangci.yml`、`buildinfo`；`make tools lint-go test` 为绿。
3. Node：`package.json`（`corepack use pnpm@12.8.1`）、`pnpm-workspace.yaml`、oxlint / oxfmt / knip 配置；`pnpm install`；`make lint-web knip` 为绿，包括 P1 留下的两个脚本通过零警告与格式检查。
4. `deploy/compose.dev.yaml` 与 `make dev-db`，核对数据库 locale。
5. `Makefile` 收尾、`README.md`。
6. `.github/workflows/ci.yml`；推送分支，持续集成为绿。
7. 反向对照（第 5 节），审查，修复，合并。

## 5. 测试与验证

本 Phase 的产物是门禁本身，所以每道门禁都要证明两件事：正常情况下为绿，出问题时确实会失败。

| 门禁 | 正常 | 反向对照（临时改动，验证后撤销） |
|---|---|---|
| golangci-lint | `make lint-go` 为绿 | 导入 `log` → depguard 报错；写一个 `init()` → gochecknoinits 报错；打乱 import 分组 → goimports 报错 |
| go test | `make test` 为绿（buildinfo 的表格驱动测试） | 改错一个期望值 → 失败 |
| oxlint | 零警告 | `tools/` 下的脚本里留一个未使用的变量 → 失败 |
| oxfmt | 格式检查通过 | 打乱一个配置文件的缩进 → 失败 |
| knip | 为绿 | 加一个没有用到的依赖 → 失败 |
| 样例集自检 | 通过 | P1 已做过反向对照 |
| 样例集的字节保护 | `git check-attr` 显示 `text: unset` | 以 `core.autocrlf=true` 克隆一份，样例的字节与仓库中完全一致 |
| 开发库 | `make dev-db` 就绪 | 查询 `datlocprovider`、`datctype`，`show_trgm('中文')` 不为空 |
| 持续集成 | 分支推送后 `server`、`web` 为绿 | — |

## 6. 完成标准

- 第 5 节的全部检查通过，反向对照都按预期失败。
- 持续集成在本 Phase 的分支上为绿。
- 审查完成（`reviews/P2-repo-toolchain-review.md`），发现的问题已修复。
- M0 总设计的 P2 行与"从 Nerve 拷贝的范围"按第 2 节的差异修订，进度表更新。

## 7. 结果

（完成后补写）
