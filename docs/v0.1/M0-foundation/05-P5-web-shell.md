# M0/P5 前端外壳与内嵌：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M0/P5 前端外壳与内嵌 |
| 状态 | 进行中 |
| 基线 | `eff461d`（P4 完成：接口契约、代码生成、`instance` 模块、TS 客户端） |
| 上级文档 | [M0 总设计](00-M0-design.md) 第 4、5、7 节；[总体设计](../v0.1-design.md) 第 9 节 |

---

## 1. 基线

P4 留下的：
- `GET /api/v0/instance` 与它的契约；TS 客户端 `@nervewiki/api-client`（openapi-typescript 生成的类型 + openapi-fetch 的 `createClient`），它的 `check:types` 由 `make lint-web` 执行。
- 平台的路由器：`/healthz`、`/readyz`、`/api/` 兜底（404 problem+json），模块的接口挂在 `/api/v0/…`。中间件链统一设置安全头（`X-Content-Type-Options`、`Referrer-Policy`、`X-Frame-Options` 等），`/api/` 的响应不缓存。

pnpm 工作区只有 `web/packages/api-client` 一个包；还没有前端应用，二进制不提供页面。

前序 Phase 对本 Phase 的要求（M0 总设计"前序 Phase 对后续 Phase 的要求"）：

1. 前端不引入 unified / remark / rehype；编辑器（CodeMirror 6）不在 M0 引入。
2. oxlint 加入 React、jsx-a11y 插件与浏览器全局变量的限制，按路径设定 `web/**` 的运行环境；`.gitignore`、`.oxlintrc.json` 加入前端产物目录；评估 turbo 是否真的需要。
3. `webui` 挂在组合根的 `/`（不带方法），不遮住平台的 `/api/` 兜底；页面 CSP 与静态文件的缓存由 `webui` 设置，安全头由中间件链统一设置。
4. 前端整体用哪个 TypeScript 版本由 P5 决定（P4 文档 3.7）。

## 2. 目标与范围

**目标**：前端能构建、内嵌进二进制、在浏览器里运行；首页显示实例版本。定下以后每个页面照着做的写法：service → store → 组件，多语言，明暗主题，错误边界。

**做**：
- 前端应用 `web/apps/web`：Vite + React 19 + React Router（数据路由）+ TypeScript；路由、首页、应用内 404、错误边界。
- UI 基座：Tailwind CSS 4、shadcn/ui 的写法（Radix + `cva` + `tailwind-merge`），明暗主题（跟随系统 / 浅色 / 深色）。
- 多语言：`zh-CN` 与 `en`，键在编译期一致。
- 分层样板：`instance` 的 service、store、组件；`RootStore` 与 SWR 的接法（总体设计 9.4）。
- 平台包 `webui`：内嵌前端、SPA 兜底、页面 CSP、静态文件缓存；组合根挂在 `/`。
- 命令：`make build`（前端 + 内嵌 + 二进制）、`make web-dev`（Vite 开发服务器，代理后端）、`make dev`（一条命令起数据库、后端、前端热更新）；`make test` 含 vitest。
- 门禁：oxlint 的 React 与无障碍规则、knip、vitest、类型检查、构建进入持续集成。

**不做**，以及与 M0 总设计的差异（本 Phase 完成时同步修订 00 号文档与总体设计）：

| 内容 | 决定 | 理由 |
|---|---|---|
| React Router 的框架模式（`@react-router/dev`、SPA Mode） | 不用，改用数据路由（`createBrowserRouter`） | 数据由 SWR 与 MobX 负责，用不上框架模式的 `clientLoader` 与类型生成；框架模式的 SPA Mode 在构建时预渲染根路由，`index.html` 带内联脚本，页面 CSP 只能按哈希放行。数据路由是普通的 Vite 应用：没有内联脚本，`script-src 'self'` 即可；按路由拆包用 `lazy` |
| M0 总设计第 4 节的 `packages/i18n`、`packages/ui`、`packages/tsconfig` | 不拆包：文案与 UI 组件放在 `apps/web/src` 下，共享的 TypeScript 配置是 `web/tsconfig.base.json` | 按 M0 总设计"两个以上的使用方共用、或者有独立的检查规则时才拆成包"：三者都只有 `apps/web` 一个使用方；文案的一致性由类型检查与 vitest 保证，不需要单独的包 |
| turbo | 不引入 | 工作区只有两个包，`api-client` 不需要构建（导出源码）；`pnpm -r` 按依赖顺序执行各包的脚本，Makefile 是唯一入口。包多到需要任务缓存时再引入。总体设计 9.1 的工具链相应修订 |
| React Router 的版本 | 8.x | M0 总设计写的是 7；8 是 7 的延续（v7 的 future 标志全部生效，ESM only，要求 React 19.2.7+、Vite 7+），按 M0 总设计第 5 节"取当时的最新稳定版" |
| 字体 | 系统字体栈，不内嵌字体文件 | 中文字体体积大；M0 不定视觉风格 |

## 3. 设计

### 3.1 目录与职责

```
web/
  tsconfig.base.json          共享的编译选项（api-client 与 apps/web 继承）
  apps/web/                   @nervewiki/web
    index.html                只有一个模块脚本；主题初始化脚本 /theme-init.js
    public/theme-init.js      在首次绘制前按偏好设置 <html class="dark">
    vite.config.ts            React、Tailwind 插件；开发服务器代理 /api、/healthz、/readyz
    src/
      main.tsx                入口：创建 RootStore、路由器，渲染 Providers
      app/
        providers.tsx         StoreProvider、SWRConfig、I18nProvider
        router.tsx            路由表：布局、首页、404；每个页面 lazy 加载
        layout.tsx            外壳：顶栏（产品名、语言与主题切换）+ <Outlet>
        error-boundary.tsx    路由错误边界：错误页，可重试
      pages/
        home.tsx              首页：显示实例版本
        not-found.tsx         应用内 404
      services/
        api.ts                ApiError；把 openapi-fetch 的结果转成值或异常
        instance.service.ts   InstanceService：GET /api/v0/instance
      stores/
        root.store.ts         RootStore：各个 store 与 service 的唯一装配处
        instance.store.ts     InstanceStore：实例信息
        preferences.store.ts  PreferencesStore：主题与语言，存在 localStorage
        context.tsx           StoreProvider、useStore
      i18n/
        messages/en.ts        文案的源头：键的集合由它决定
        messages/zh-CN.ts     类型为 Messages，缺键、多键编译失败
        i18n.tsx              t()、插值、I18nProvider、useT
      components/ui/          shadcn/ui 风格的基础组件（Button、DropdownMenu …），按需加入
      lib/cn.ts               clsx + tailwind-merge
      styles.css              Tailwind 入口、主题变量（浅色、深色）
  packages/api-client/        P4
server/internal/platform/webui/
  embed.go                    //go:embed all:dist；FS()
  handler.go                  静态文件、SPA 兜底、缓存头、页面 CSP
  csp.go                      页面的 Content-Security-Policy
  dist/.gitkeep               make build 把前端复制到这里
```

依赖方向（前端）：

```
pages / components ──► stores（useStore）──► services ──► @nervewiki/api-client
pages / components ──► i18n、components/ui
app/ ──► pages、stores、i18n
```

组件不直接调用 service 或 api-client；service 没有状态，只做请求与结果转换；store 持有状态，由 SWR 驱动加载。

服务端新增的边：`bootstrap ──► platform/webui`。`webui` 只依赖标准库。

### 3.2 页面与路由

数据路由（`createBrowserRouter`），根路由是布局，带错误边界：

| 路径 | 页面 |
|---|---|
| `/` | 首页：产品名、实例版本、接口版本 |
| `*` | 应用内 404：说明页面不存在，链接回首页 |

- 每个页面 `lazy` 加载，构建时各自成块。
- 错误边界（`ErrorBoundary`）接住渲染与加载中的异常：显示错误页与"重试"，`ApiError` 显示它的 `code`；不把异常细节显示给用户，写进 `console.error`。
- 未知路径不请求服务端：服务端对所有非 `/api/` 路径返回 `index.html`，由前端路由决定显示 404。

### 3.3 service → store → 组件

照总体设计 9.4 定下以后的写法；M0 没有登录，只有一代 `RootStore`，会话相关的部分（按会话重建、`loginId`、`SessionChangedError`）随 M1：

- **service**：构造函数拿 `ApiClient`，方法返回领域值或抛出 `ApiError`（带 `status`、`code`、`problem`）。网络错误照原样抛出。没有模块级的客户端。
- **store**：MobX 的可观察状态与 action；`InstanceStore.fetch()` 调 service，结果写进 `info`。store 之间只经由 `RootStore` 互相引用。
- **RootStore**：唯一的装配处：创建 `ApiClient`（同源，`baseUrl` 为空）、各个 service 与 store。测试用假的 service 构造 store。
- **组件**：`observer` 包裹，读 store；需要的接口类型从 service 导出，不直接导入 api-client（oxlint 检查）；加载由 SWR 驱动：`useSWR(key, () => store.instance.fetch())`，SWR 负责去重、重试、聚焦时刷新，store 负责保存与派生。页面据 SWR 的 `error`、`isLoading` 显示加载与错误状态。

### 3.4 多语言

- 文案的源头是 `messages/en.ts`（`as const`），`Messages` 是它的类型；`messages/zh-CN.ts` 声明为 `Messages`，缺键、多键、键名拼错都在类型检查时失败。
- `t(key, params?)`：键是 `keyof Messages` 的联合类型；插值写 `{name}`。vitest 检查两种语言的同一个键有相同的占位符、没有空串。
- 语言的选择：用户保存的偏好（`PreferencesStore`，localStorage）→ 浏览器语言（`zh` 开头为 `zh-CN`）→ `en`。切换语言时更新 `<html lang>`。
- `I18nProvider` 是 `observer`，读当前语言，经 React context 提供 `t`：语言变化时每个用 `useT()` 的组件都重新渲染，不依赖组件自己是不是 `observer`。
- 日期、数字用 `Intl`，不引入 i18n 库：M0 的需求只有键值与插值；复数、日期格式到了用得上的 M 再用 `Intl.PluralRules` 等补上。

### 3.5 主题与 UI 基座

- Tailwind CSS 4（`@tailwindcss/vite`），主题变量写成 CSS 变量（浅色在 `:root`，深色在 `.dark`），shadcn/ui 的语义色名（`background`、`foreground`、`primary` …）。
- 组件按 shadcn/ui 的写法放在 `components/ui/`：Radix 原语（`radix-ui`）+ `cva` 变体 + `cn()`；只加入用到的组件。
- 主题偏好：`system` / `light` / `dark`，存在 localStorage；`system` 跟随 `prefers-color-scheme` 并监听它的变化。`PreferencesStore` 从构造函数拿存储与媒体查询（测试传入假的），给出解析后的主题；把它写到 `<html>` 上的是 `app/` 中的一个 `observer` 组件。
- 首次绘制前的主题：`index.html` 在 `<head>` 里同步加载 `/theme-init.js`（外部脚本，`script-src 'self'` 放行），读同一个存储键设置 `dark` 类，避免深色用户每次加载先闪一下浅色。存储键在 `preferences.store.ts` 中定义，vitest 核对 `theme-init.js` 用的是同一个键。

### 3.6 平台：`webui`

从 Nerve 拷贝 `webui`，按本项目裁剪：

- `Handler(files fs.FS)`：只接受 GET、HEAD；存在的文件照原样提供，`assets/` 下的文件（Vite 的内容哈希文件名）`Cache-Control: public, max-age=31536000, immutable`，其余 `no-cache`；`assets/` 下不存在的路径答 404（旧版本的块不会变成 HTML）；其余路径答 `index.html`。隐藏文件（名字以 `.` 开头的部分）不提供。没有构建前端时每个请求答 404 并提示 `make build` 或 `make web-dev`。
- 页面 CSP 只加在 HTML 响应上，是固定的策略：`default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'`。`style-src` 放行内联样式：Radix 的弹层用 style 属性定位。Nerve 按内联脚本的哈希生成 `script-src`，本项目的页面没有内联脚本，不需要它；页面出现内联脚本时，端到端测试 S2 的 CSP 违规检查会失败。
- 组合根把 `webui.Handler(webui.FS())` 挂在 `/`；`newApp` 接收前端文件系统作为参数，测试传入 `fstest.MapFS`。

### 3.7 工具链与门禁

| 项 | 做法 |
|---|---|
| TypeScript | `apps/web` 用 7.x（原生编译器）；`api-client` 仍用 5.9.3（openapi-typescript 调用 TypeScript 的 JS API）。`apps/web` 的类型检查也检查它导入的 `api-client` 源码 |
| oxlint | 加 `react`、`react-perf`、`jsx-a11y` 插件；`web/**` 的运行环境是浏览器，配置文件（`vite.config.ts` 等）是 Node。`no-restricted-globals` 禁止与局部变量容易混淆的浏览器全局名（`name`、`event`、`status`、`length`、`top`、`parent`、`self`、`close`、`open`），要用时写 `window.name`。`no-restricted-imports`：`pages/`、`components/`、`app/` 不导入 `@nervewiki/api-client`，类型从 service 取 |
| knip | `apps/web` 工作区：入口由 Vite 插件识别（`index.html`、`src/main.tsx`）；`public/theme-init.js` 作为入口 |
| vitest | `apps/web`，jsdom 环境，Testing Library；`make test` 执行 `pnpm -r run test` |
| 构建 | `pnpm --filter @nervewiki/web build`，产物在 `web/apps/web/dist`（加入 `.gitignore` 与 oxlint、oxfmt 的忽略） |
| pnpm catalog | 两个以上的包共用的依赖写进 `pnpm-workspace.yaml` 的 `catalog`；TypeScript 两个版本不同，各自写在包里 |

### 3.8 命令与持续集成

| 命令 | 作用 |
|---|---|
| `make build` | 构建前端 → 复制到 `server/internal/platform/webui/dist` → 构建 `bin/nervewiki`（版本号用 ldflags 注入） |
| `make web-dev` | Vite 开发服务器（127.0.0.1:5173），`/api`、`/healthz`、`/readyz` 代理到 127.0.0.1:8080 |
| `make dev` | `make dev-db`，然后同时运行 `make run` 与 `make web-dev`；Ctrl-C 一起停止 |
| `make test` | 另跑 `pnpm -r run test`（`test-web`） |

持续集成的 `web` 任务：`make lint-web`（含类型检查）→ `make gen-check-web` → `make knip` → `make test-web` → `make build-web`。

### 3.9 依赖版本

取 2026-09-30 的最新稳定版，满足 `minimumReleaseAge`（发布满 1 天）：

| 包 | 版本 |
|---|---|
| react、react-dom、@types/react、@types/react-dom | 19.3.0 |
| react-router | 8.4.0 |
| vite / @vitejs/plugin-react | 8.3.1 / 6.1.1 |
| typescript（apps/web） | 7.0.2 |
| tailwindcss、@tailwindcss/vite | 4.3.3 |
| radix-ui / class-variance-authority / clsx / tailwind-merge / lucide-react | 1.6.7 / 0.7.1 / 2.1.1 / 3.7.0 / 实施时取满 1 天的最新版 |
| mobx / mobx-react-lite | 7.0.5 / 5.1.0 |
| swr | 2.5.1 |
| vitest / jsdom | 5.0.2 / 30.1.1 |
| @testing-library/react / user-event / jest-dom | 16.3.3 / 14.6.7 / 7.0.1 |

## 4. 实施步骤

在分支 `m0-p5-web-shell` 上依次进行，每个 Step 结束时 `make check` 为绿。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 工作区与工具链：`web/tsconfig.base.json`、`apps/web` 骨架（Vite + React + React Router + TS 7）、oxlint、knip、vitest、Makefile（`web-dev`、`build-web`、`test-web`、`dev`）、持续集成 | [P5-S1-toolchain.md](plans/P5-S1-toolchain.md) |
| S2 | `webui` 与组合根的挂载；`make build` | [P5-S2-webui.md](plans/P5-S2-webui.md) |
| S3 | UI 基座：Tailwind、基础组件、主题（store、`theme-init.js`）、布局外壳 | [P5-S3-ui-base.md](plans/P5-S3-ui-base.md) |
| S4 | 多语言；分层样板（service、store、SWR）；首页、404、错误边界；README | [P5-S4-app.md](plans/P5-S4-app.md) |

之后是反向对照、独立审查、修复、合并。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| Go 单元 | `webui`：文件、缓存头、SPA 兜底、`assets/` 下的 404、隐藏文件、方法、未构建时的提示、CSP 只加在 HTML 上 |
| Go 整个程序 | 组合根挂上 `webui` 之后：`/` 与任意前端路径答 `index.html` 带 CSP 与安全头；`/api/` 下的未知路径仍是 404 problem+json；`/healthz` 不受影响 |
| vitest | 文案的占位符与非空；`theme-init.js` 与 store 的存储键一致；`InstanceService` 对 200、problem、网络错误的转换；`InstanceStore` 用假 service；`PreferencesStore` 的读取、保存、跟随系统；首页渲染出版本；未知路由渲染 404；错误边界渲染错误页 |
| 类型 | `tsc --noEmit`（两个包）；缺一个 `zh-CN` 文案键时失败 |
| 构建 | `make build` 产出的二进制提供页面：`curl /` 返回带 CSP 的 HTML，`/assets/*` 带长缓存 |

反向对照（验证后撤销）：
- 删掉 `zh-CN.ts` 的一个键 → 类型检查失败；改掉一个占位符 → vitest 失败；
- 把 `webui` 挂在 `GET /{path...}` 之类会遮住 `/api/` 兜底的模式上 → 整个程序的测试失败；
- CSP 去掉 `script-src 'self'` → `webui` 测试失败；
- `theme-init.js` 换一个存储键 → vitest 失败；
- 组件里直接导入 `api-client` → oxlint 的 `no-restricted-imports` 失败；
- 未使用的导出 → knip 失败。

浏览器中的行为（CSP 违规、控制台错误、刷新任意路由）由 P6 的冒烟故事 S2、S4 覆盖。

## 6. 完成标准

- 第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check`、`make build` 本地与持续集成为绿。
- `make build` 后运行 `bin/nervewiki serve`，浏览器打开首页显示实例版本；切换语言与主题生效且刷新后保持；打开 `/nope` 显示应用内 404；控制台没有错误与 CSP 违规。
- `make dev` 起开发环境，修改组件后页面热更新。
- 审查完成（`reviews/P5-web-shell-review.md`），发现的问题已修复。
- M0 总设计、总体设计按第 2 节的差异修订，进度表更新。

## 7. 结果

（完成后补写）
