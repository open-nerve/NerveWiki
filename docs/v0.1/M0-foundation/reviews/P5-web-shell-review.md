# M0/P5 前端外壳与内嵌：审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | 分支 `m0-p5-web-shell`（`60977b5..bf56200`），对照 [05-P5-web-shell.md](../05-P5-web-shell.md)、各 Step 计划、[M0 总设计](../00-M0-design.md) 与[总体设计](../../v0.1-design.md)第 9 节 |
| 审查方式 | 独立审查者在仓库的 worktree 上实测：<br>• 跑全部门禁，web 测试重复多遍，Go 测试 `-race -count=3`；<br>• `make build` 出的二进制接自建的 PostgreSQL（builtin `C.UTF-8`），在 Chromium 中检查：控制台、CSP、请求、深链接刷新、主题与语言、深色无闪烁、404、删掉页面块后的错误页；<br>• 开发服务器：代理、StrictMode、热更新；<br>• 对照 react-router、swr、rolldown、Vite 与 Go 标准库的源码核实行为；<br>• 重跑第 5 节的反向对照，另外自拟 15 项探针 |
| 日期 | 2026-09-30 |
| 结论 | 基本达到生产级：分层清楚，没有上帝文件（最大的源文件 94 行），M0 对 P5 的四条要求都已落实，反向对照有效，门禁全绿，测试稳定。<br>没有 Important；8 项 Minor、7 项 Nit，全部处理。各项有意的决定经审查确认合理 |

另外，持续集成在审查期间发现一项：错误边界的测试断言 `console.error` 已被调用，而错误边界在 effect 中记录，在较慢的机器上 effect 晚于断言。已改为等待（`f074466`）。

## 发现与处置

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| M1 | Minor | react 块用的 `output.advancedChunks` 在 rolldown 中已弃用：每次构建打印警告，将来这个选项移除时，vendor 块会静默消失 | 改用 `codeSplitting`，构建不再警告，react 块仍在 |
| M2 | Minor | `no-restricted-imports` 只覆盖 `app/`、`pages/`、`components/` 三个目录：`lib/`、`stores/`、将来的 `hooks/` 都能导入 API 客户端，甚至在模块顶层创建它；以相对路径进入 `packages/api-client` 也拦不住。"没有模块级的客户端"是总体设计 9.4 的约定，M1 按登录重建 RootStore 靠它 | 规则改为默认禁止：<br>• `src/**` 不导入 `@nervewiki/api-client`；<br>• `services/` 与 `root.store.ts` 可以导入，但不能导入 `createClient`；<br>• 只有 `main.tsx` 与 `test/` 可以导入 `createClient`；<br>• 任何位置都不能以相对路径进入 `packages/api-client`。<br>反向对照：在 `lib/`、`stores/`、`services/`、新目录中各创建一个客户端，从 `pages/` 以相对路径转出导出，全部报错；service 导入类型照常通过 |
| M3 | Minor | 注释说"每个 RootStore 一份 SWR 缓存"，不成立：SWR 只在 `SWRConfig` 挂载时建一次缓存，只换 `store` 属性，上一个会话的缓存会留下（审查者用临时测试复现） | 注释改为"每次挂载一份；每个 RootStore 要重新挂载 `AppProviders`（带 key）"，写进 [M1 的移交](../../M1-auth/handoffs/M0-P5-web-shell.md) |
| M4 | Minor | 设计规定 `ApiError` 带 `status`、`code`、`problem`，实现没有 `problem`，`Problem.errors`（字段错误）因此丢失，而 M1 的表单要按字段显示错误 | `ApiError` 保留 `problem`，`code` 取自它；新增测试 |
| M5 | Minor | README 说 `make run` 起的服务"不带前端"，只在从未构建时成立：`make build` 之后，`go run` 会嵌入上一次复制进 `webui/dist` 的前端 | README 改为如实的描述 |
| M6 | Minor | 两处与文档不符：<br>• 设计说 `make build` 用 ldflags 注入版本号，实际没有；<br>• 设计第 6 节说 `make build` 在持续集成中为绿，而持续集成只跑 `build-web` | `make build` 以 `VERSION`（默认 `0.1.0-dev`）注入版本号。<br>持续集成中完整执行 `make build` 的是 P6 的 e2e 任务（它同时需要 Go 与 Node），本 Phase 的第 7 节写明 |
| M7 | Minor | 依赖方向有两个问题：<br>• `i18n` 导入 `stores`（`useStore`），`stores` 又导入 `i18n/locale`，目录级循环；<br>• `<html lang>` 在 providers 里同步，深色类却在布局里同步，布局出错被根错误边界替换时，主题同步随之卸载 | `I18nProvider` 改为接收 `locale`，`i18n` 成为只被依赖的叶子。<br>`app/document-sync.tsx` 同时同步 `lang` 与深色类，挂在 `AppProviders` 里、路由器之上 |
| M8 | Minor | 两处测试缺口：<br>• 计划中的"渲染时抛出的异常由错误边界显示"、错误码分支、根错误边界、重试判定都没有测试；<br>• 没有测试检查控制台警告：删掉 `HydrateFallback`，React Router 打出警告，测试照样全部通过 | 新增测试：<br>• 页面渲染时抛出 `ApiError`，错误页显示错误码、不显示 detail；<br>• 布局出错，根错误边界替换布局；<br>• 重试判定提成 `isRetryable`，测 4xx、5xx、非 problem 的 502、网络错误。<br>`test/setup.ts` 监视 `console.warn` 与 `console.error`，出现意外输出即让测试失败；预期会输出的测试用 `mockImplementation` 声明。<br>反向对照：删掉 `HydrateFallback`，3 个测试失败 |
| N1 | Nit | 开发服务器的代理键按前缀匹配，`/api-tokens`、`/healthzfoo` 这类页面路径在开发时会被转给后端，与生产不一致 | 改为正则：`^/api(/|$)`、`^/healthz$`、`^/readyz$` |
| N2 | Nit | `fakeApi` 的注释说它拒绝意外的请求，实际不会 | 注释改为如实的描述 |
| N3 | Nit | 插值参数没有按键定类型：`t("home.version")` 不带参数也能编译 | `Translate` 由文案的模板字面量类型推出每个键的占位符：<br>• 带占位符的键必须恰好提供它们；<br>• 没有占位符的键不接受参数。<br>由带 `@ts-expect-error` 的类型检查用例固定 |
| N4 | Nit | `no-cache` 的文件（`index.html`、`theme-init.js`、`favicon.svg`）没有校验器（嵌入的文件没有修改时间），每次加载都重新下载，阻塞渲染的 `theme-init.js` 也是 | `webui` 在创建时为 `assets/` 之外的文件计算内容的 ETag，`If-None-Match` 答 304。新增测试 |
| N5 | Nit | 所有 4xx 都不重试，包括带 `Retry-After` 的 429 与 408 | M0 没有限流，保持现状；写进 M1 的移交，随限流一起处理 |
| N6 | Nit | 文档漂移，涉及 P5 文档 3.1、3.3、3.7、3.9：文件名、客户端在哪里创建、`react-perf`、测试依赖。另有设计自己列出的、M0 总设计与总体设计还没做的修订 | 收尾时一并修订，见 P5 文档第 7 节 |
| N7 | Nit | `routes.tsx` 的注释只从体验解释 `HydrateFallback`，没提它还让 React Router 不打警告 | 注释补上 |

## 审查者对有意决定的判断

| 决定 | 判断 |
|---|---|
| 数据路由，不用框架模式 | 同意：构建出的 `index.html` 没有内联脚本，固定 CSP 零违规 |
| 固定 CSP，`theme-init` 用外部文件 | 同意：注入的内联脚本被拦截并报告；Radix 的内联样式在 `style-src 'unsafe-inline'` 下没有违规 |
| 不引入 turbo | 同意 |
| 不拆 i18n、ui、tsconfig 包 | 同意：三者都只有一个使用方 |
| 应用用 TS 7，api-client 留在 5.9.3 | 同意：在 api-client 源码中注入的类型错误，web 的 TS 7 也报出 |
| 自写的类型化 i18n | 同意：约 40 行，键在编译期检查 |
| PreferencesStore 传入 RootStore | 同意：偏好属于设备 |
| 每次挂载一份 SWR 缓存 | 可以接受，说法已改正（M3） |
| react 块单独打包 | 值得：只改文案时 react 块的哈希不变 |
| 去掉 react-perf | 同意：实测只报惯用的内联处理函数 |
| ES2023 | 同意：浏览器的实际下限由 Vite 的默认目标决定 |

## 反向对照

设计第 5 节的各项，审查者重跑，全部按预期失败：

- `zh-CN` 缺一个键、多一个键，类型检查失败；
- 占位符不一致、空串，vitest 失败；
- `webui` 挂在 `GET /{path...}`，ServeMux 在注册时就报冲突；
- CSP 去掉 `script-src` 或加上 `'unsafe-inline'`，测试失败；
- `theme-init.js` 换一个存储键或改动规则，测试失败；
- `pages/` 导入 api-client，oxlint 报错；
- 未使用的导出，knip 失败。

修复后新增的反向对照见上表各项。

## 核实修复

- 本地 `make check` 为绿，web 测试 55 项。
- `make build VERSION=9.9.9-test` 后，`nervewiki version` 打印注入的版本；构建不再警告。
- 修复后的持续集成（run 36707183052）为绿。
