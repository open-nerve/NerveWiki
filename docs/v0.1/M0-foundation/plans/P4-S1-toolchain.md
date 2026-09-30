# M0/P4/S1 生成工具链：实施计划

上级：[P4 文档](../04-P4-api-contract.md) 3.1–3.3、3.8、3.9。

## 任务

1. `server/tools`：独立的 Go 模块（`go 1.27`、`toolchain go1.27.1`），`tool` 指令引入 oapi-codegen v2.8.0。
   - `bodyshapegen`：从 Nerve 拷贝 `main.go`、`generate.go`、测试与 `testdata`，改名；包级变量改成函数（`gochecknoglobals`）。
2. `api/`：`openapi.yaml`（入口）、`common.yaml`（`Problem`、`FieldError`；字段错误码与 `shared.FieldCodes()` 相同）、`modules/instance.yaml`、`redocly.yaml`；根 `package.json` 加 `@redocly/cli` 2.55.0。
3. `platform/httpserver/apigen/oapi-codegen.yaml` 与生成的 `components.gen.go`；`instance` 的 `gen/oapi-codegen.yaml`（生成物在 S4 随模块提交之前先确认能生成、能编译）。
4. Makefile：`gen`、`gen-go`、`gen-web`、`gen-check`、`gen-check-go`、`gen-check-web`；`lint-go` 与 `test` 覆盖 `server/tools`。
5. oxfmt 排除 `api/dist/`。

## 测试

- `bodyshapegen` 的 golden 测试（拷贝）。
- `make gen` 之后 `git status` 干净，`make gen-check` 通过。

## 完成检查

`make check` 与 `make gen-check` 为绿。
