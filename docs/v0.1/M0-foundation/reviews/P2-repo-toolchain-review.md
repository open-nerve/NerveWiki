# M0/P2 仓库与工具链：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m0-p2-repo-toolchain`（`b4b7e95..5c66cd9`）对照 [02-P2-repo-toolchain.md](../02-P2-repo-toolchain.md) 与 [M0 总设计](../00-M0-design.md) |
| 审查方式 | 独立审查者在仓库副本上实测：跑全部门禁；对每道门禁做反向对照；以 `core.autocrlf=true` 克隆；起开发库核对 locale；核对版本、许可证与拷贝文件的清理；在 Nerve 平台层的副本上试加 linter |
| 日期 | 2026-09-30 |
| 结论 | 门禁工作正常，P1 的要求都满足，三项差异合理。2 项 Important、9 项 Minor、3 项 Nit，全部处理，修复后持续集成为绿 |

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| I1 | Important | `.gitattributes` 只保护了样例集。`core.autocrlf=true`（Git for Windows 的默认值）检出后，其余文件变成 CRLF，gofmt 与 oxfmt 的检查全部失败 | 第一行加 `* text=auto eol=lf`，样例集的 `-text` 在后面覆盖它。`git add --renormalize .` 没有产生变更。以 `autocrlf=true` 克隆验证：样例集以外没有文件含 CR，样例逐字节一致，`make lint` 为绿。P2 文档的验证项同步加强 |
| I2 | Important | 格式检查按文件清单列举，漏了 `.github/`、`deploy/`、`server/.golangci.yml`、根 README，后续 Phase 还要不断改清单 | 改为 `oxfmt --check .`，覆盖整个仓库（遵守 `.gitignore`）。排除 `docs/`（oxfmt 会重排中文表格的对齐）、`pnpm-lock.yaml`（由 pnpm 维护）和样例集。覆盖的文件由 8 个增加到 12 个 |
| M1 | Minor | `golangci-lint run` 会静默忽略配置中拼错的键 | `lint-go` 先执行 `golangci-lint config verify`；反向对照：拼错一个键即失败 |
| M2 | Minor | 没有检查 `go.mod` 是否整洁 | `lint-go` 加 `go mod tidy -diff`；反向对照：多一个用不到的依赖即失败 |
| M3 | Minor | `pnpm-workspace.yaml` 的注释说"只是写出默认值"，但显式写出 `minimumReleaseAge` 会让 `minimumReleaseAgeStrict` 变为真 | 注释改为实际行为；采纳 `trustPolicy: no-downgrade`。`pnpm install` 确认设置有效、锁文件通过供应链检查 |
| M4 | Minor | `.oxlintrc.json` 留有 Plane 的设置（`polymorphicPropName`）；`*.config.*` 的忽略会让任意配置文件跳过检查；提前放进了 P5 的产物目录；browser 与 node 全局变量同时打开 | 重写为最小配置：三类规则，零警告；`tools/**` 用 `overrides` 设为 Node 环境；React、jsx-a11y、浏览器全局变量与产物目录移到 P5（写进 M0 总设计的要求表）。探针验证：`tools/` 外不认 `process`，`*.config.ts` 受检 |
| M5 | Minor | actions 按大版本 tag 引用；checkout 把令牌留在 `.git/config` 里，而之后的步骤会运行第三方代码 | 按完整的提交 SHA 引用，注释写明版本；checkout 设 `persist-credentials: false`。没有采纳 Dependabot：它会在仓库里自动开 PR，是否需要由负责人决定 |
| M6 | Minor | `postgres:18.6` 没有固定发行版，而三元组切分依赖 glibc | 改为 `postgres:18.6-trixie`（Debian 13，glibc 2.41）。这条约定写进总体设计 7.1，并在 M0 总设计中要求 P3、P6 使用同一个镜像。实测发现，本地旧的 `postgres:18.6` 与 `18.6-trixie` 的镜像 ID 已经不同，印证了不带发行版名的 tag 会漂移 |
| M7 | Minor | `make lint` 不含 CI 会跑的 `knip`；没有 Go 的格式修复入口 | 新增 `make check`（`lint`、`knip`、`test`，与 CI 一一对应）和 `make fmt`（`golangci-lint fmt` 与 `oxfmt`）；README 改用它们 |
| M8 | Minor | M0 进度表中 P2 仍是"未开始" | 已更新，并修订 P2 行、拷贝范围表与变更记录 |
| M9 | Minor（可选） | CI 的测试不开竞态检测 | 采纳：`-race` 直接写进 `make test`，本地与 CI 一致；P3 起有事务、监听、长连接这类并发代码 |
| N1 | Nit | compose 注释以"Nerve 的开发库"为理由，对外部贡献者没有意义 | 改为"避开 5432 以及本机其他项目常用的端口" |
| N2 | Nit | `.gitignore` 的 `bin/` 匹配任意深度 | 改为 `/bin/` |
| N3 | Nit | `.editorconfig` 的样例集条目缺 `indent_*` | 补上 `indent_style`、`indent_size = unset` |

另外，审查者给后续 Phase 的提示都已写进 M0 总设计的"前序 Phase 对后续 Phase 的要求"，由对应 Phase 开工时落实：

- P3：测试容器使用同一个镜像；环境变量覆盖只处理带 `__` 的 `NWIKI_` 变量；`buildinfo.version` 是架构测试的例外；linter 集合最多加 `errorlint`。
- P5：React 相关的 oxlint 配置；Node 26 不再自带 corepack；评估 turbo 是否需要。
- P6：镜像构建时 `buildinfo` 的提交信息。

## 核实修复

- 本地 `make check` 为绿；修复后的持续集成（run 36684831116）为绿。
- 以 `core.autocrlf=true` 克隆：样例集以外没有文件含 CR，样例逐字节一致，`make lint` 为绿。
- 新增检查的反向对照（配置拼错的键、多余的依赖）都按预期失败；原有的反向对照重新跑过一遍。
- 开发库：PG 18.6（Debian 18.6-1.pgdg13+2），provider `b`，`datctype` 为 `C.UTF-8`。

## 遗留

没有未处理的发现。
