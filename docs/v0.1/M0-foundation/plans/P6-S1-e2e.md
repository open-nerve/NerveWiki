# M0/P6/S1 端到端测试：实施计划

上级：[P6 文档](../06-P6-e2e-delivery.md) 3.1–3.4、3.6。

## 任务

1. `e2e/`（`@nervewiki/e2e`）：`package.json`、`tsconfig.json`（继承 `web/tsconfig.base.json`，加 Node 类型）、`playwright.config.ts`、`global-setup.ts`。
2. fixtures：`db.ts`、`server.ts`、`browser.ts`、`test.ts`，从 Nerve 拷贝、改名、裁剪（去掉认证、工作区相关的部分）；PostgreSQL 按 builtin `C.UTF-8` 初始化。
3. 故事 S1–S4（`stories/smoke/`）。
4. Makefile：`make e2e`（`VERSION` 与 ldflags 已由 P5 加入）。
5. 工作区：`pnpm-workspace.yaml` 的 catalog 放 `typescript`（`apps/web` 与 `e2e` 共用）；oxlint 的 `e2e/**` 是 Node 环境；knip 加 `e2e` 工作区；`.gitignore` 忽略 `e2e/playwright-report/`、`e2e/test-results/`。
6. 持续集成：`e2e` 任务，失败时上传报告与日志。
7. README：端到端测试一节（安装 Chromium、`make e2e`、看报告）。

## 测试

- `make e2e` 本地通过，重复运行（`--repeat-each 3`）也通过。
- 反向对照：P6 文档第 5 节中与 S1–S4 相关的各项。

## 完成检查

`make check`、`make e2e` 为绿；持续集成的 `e2e` 任务为绿。
