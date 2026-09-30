# M0/P3/S5 架构测试与静态检查：实施计划

上级：[P3 文档](../03-P3-server-platform.md) 3.6。

## 任务

1. `internal/archtest`：从 Nerve 拷贝 `rules_test`、`rules_cases_test`、`repo_test`、`binary_test`、`deps_test`、`purity_test`、`generated_test`、`composition_test`。
   - 规则按 P3 文档 3.6 调整；`sqlc`、`rawsql` 的规则随 M1。
   - 被禁止的依赖清单按本项目的依赖更新。
   - 依赖 `golang.org/x/tools/go/packages`。
2. `server/.golangci.yml`：启用 `gochecknoglobals`、`errorlint`。现有的例外（`buildinfo.version`、`pgtest` 的共享容器）用带理由的 `//nolint` 标出。实施时发现 linter 本身放过名为 `version` 的变量，`buildinfo.version` 不需要标，见 P3 文档 3.6。
3. README：运行后端（`make run`）、配置与环境变量、集成测试需要 Docker、`go test -short` 跳过集成测试。
4. 持续集成：`server` 任务运行集成测试。GitHub 的 ubuntu runner 自带 Docker，只需确认耗时。

## 测试

- 规则的构造用例全部通过；真实导入图没有违规。
- 反向对照（验证后撤销）：
  - 平台包导入 `shared`、两个平台包互相导入 → 失败；
  - 新增包级变量 → `gochecknoglobals` 失败；
  - 用 `==` 比较错误 → `errorlint` 失败。

## 完成检查

`make check` 本地与持续集成为绿。
