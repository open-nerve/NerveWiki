# M0/P5/S1 工作区与工具链：实施计划

上级：[P5 文档](../05-P5-web-shell.md) 3.1、3.7、3.8。

## 任务

1. `web/tsconfig.base.json`：共享的编译选项；`api-client` 的 `tsconfig.json` 改为继承它。
2. `web/apps/web`（`@nervewiki/web`）：`package.json`（脚本 `dev`、`build`、`check:types`、`test`）、`tsconfig.json`、`vite.config.ts`（React 插件；开发服务器 127.0.0.1:5173，代理 `/api`、`/healthz`、`/readyz` 到 127.0.0.1:8080；vitest 的 jsdom 环境）、`index.html`、`src/main.tsx` 与一个最小的首页和测试。
3. 共用的依赖写进 `pnpm-workspace.yaml` 的 `catalog`。
4. oxlint：`react`、`react-perf`、`jsx-a11y` 插件；`web/**` 为浏览器环境，配置文件为 Node；`no-restricted-globals`；`no-restricted-imports`（`pages/`、`components/`、`app/` 不导入 `@nervewiki/api-client`）。忽略 `web/apps/web/dist`。
5. oxfmt、`.gitignore` 忽略前端产物；knip 加 `web/apps/web` 工作区。
6. Makefile：`web-dev`、`build-web`、`test-web`（`make test` 包含它）、`dev`；`lint-web` 已经执行各包的 `check:types`。
7. 持续集成的 `web` 任务加 `make test-web`、`make build-web`。

## 测试

- `pnpm -r run check:types`、`pnpm -r run test`、`make build-web` 通过。
- 反向对照：组件导入 `@nervewiki/api-client` → oxlint 失败；未使用的导出 → knip 失败。

## 完成检查

`make check` 为绿；`make build-web` 产出 `web/apps/web/dist/index.html`，其中没有内联脚本。
