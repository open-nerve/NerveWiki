# M0/P4/S5 TS 客户端与持续集成：实施计划

上级：[P4 文档](../04-P4-api-contract.md) 3.7、3.8。

## 任务

1. `web/packages/api-client`：`package.json`（`@nervewiki/api-client`，`private`，`type: module`，`exports` 指向 `src/index.ts`；脚本 `gen`、`check:types`）、`tsconfig.json`、`src/index.ts`、`src/schema.gen.ts`（生成）、`test/client.typecheck.ts`。
2. 根目录：`lint-web` 执行各包的 `check:types`；oxlint 覆盖 `web/`；oxfmt、oxlint 排除生成物；knip 的工作区配置。
3. 持续集成：`server` 任务加 `make gen-check-go`；`web` 任务加 `make gen-check-web`。
4. README：新增"接口与代码生成"一节。

## 测试

- `tsc --noEmit` 通过；去掉一处 `@ts-expect-error` 后失败。
- 推送一个"改描述不重新生成"的临时提交，持续集成的两个任务都在生成物检查失败。

## 完成检查

`make check`、`make gen-check` 本地与持续集成为绿。
