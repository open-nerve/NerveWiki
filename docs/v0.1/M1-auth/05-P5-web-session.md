# M1/P5 前端会话、登录与引导：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M1/P5 前端会话、登录与引导 |
| 状态 | 已完成 |
| 基线 | P4 合并之后的 main |
| 上级文档 | [M1 总设计](00-M1-design.md) 第 3、4、6–8 节；[M0/P5 文档](../M0-foundation/05-P5-web-shell.md) 3.3；[总体设计](../v0.1-design.md) 9.4、12.5 |

---

## 1. 基线

前端停在 M0/P5：`main.tsx` 是唯一创建 API 客户端的地方（同源、没有中间件），交给唯一的 `RootStore`（偏好与实例信息）；`AppProviders`（store、SWR、多语言、文档同步）包着 `RouterProvider`；路由只有首页与应用内 404；UI 组件只有 `Button` 与 `DropdownMenu`；`services/api.ts` 的 `unwrap` 对 204 也抛错，`isRetryable` 不重试任何 4xx。vitest 全局 jsdom，`test/setup.ts` 要求没有声明的 `console.warn`、`console.error` 一次也不出现。

服务端（P1–P4）已有全部认证接口：注册、登录、续期（轮换、重复使用检测、`auth.refresh_deadline` 4 秒）、退出、`getMe`、`updateMe`、`recordOnboardingStep`；401 只有 `unauthorized` 一个码，客户端分不出过期与吊销；公开的登录也答 401（`identity.invalid_credentials`）；429 与 503 `server_busy` 带 `Retry-After`（整秒）。服务端配置校验要求续期在 8 秒（`webRefreshTimeout`）之内答复。

M0 移交给本 Phase 的：[P5 前端外壳](handoffs/M0-P5-web-shell.md)全部 4 项（每次登录一代 `RootStore`、SWR 的缓存跟着一代走、429 按 `Retry-After` 重试、客户端只在会话装配模块创建）。

## 2. 目标与范围

**目标**：浏览器里能注册、登录、退出、完成新手引导；访问令牌只在内存，过期之前自动续期，多个标签页串行续期、同步退出与换账户；每次登录一代 stores 与 SWR 缓存，上一个会话的请求与数据碰不到下一个。

**做**：
- `src/session/`：从 Nerve 拷贝令牌管理器、续期锁、认证中间件与它们的测试、测试替身；全新编写会话装配（不在导入时启动、没有模块级单例）。
- 每次登录一代 `RootStore`；`AppProviders` 按 `loginId` 重新挂载。
- 请求的错误：204、按 `Retry-After` 重试、`SessionChangedError`；problem 码到文案的映射，与契约核对。
- 路由守卫；登录、注册页；会话暂不可用的视图；`next` 的校验。
- 顶栏的用户菜单（只有显示名与退出）。
- 新手引导：步骤注册表（扩展点）与第一步"个人资料"。
- 表单组件（输入框、标签、字段错误、提示）。
- 端到端：`signedInPage`、声明预期的控制台输出；A1–A6、A9、A14 的页面版本；S2、S4 随登录守卫调整。

**不做**：设置页、改密码与停用的页面、PAT 的管理（P6，用户菜单在 P6 加上设置的入口）；会话列表；记住我、找回密码（v0.1 没有邮件）；按 `Retry-After` 自动重新提交表单（用户的操作由用户重试）。

## 3. 设计

### 3.1 文件

```
web/packages/api-client/src/index.ts        另导出 openapi-fetch 的 Middleware 类型
web/apps/web/src/
  main.tsx                                  构造设备偏好、会话、实例信息、路由器，交给 SessionRoot
  session/                                  唯一创建 API 客户端的模块
    token-manager.ts、refresh-lock.ts、auth-middleware.ts    从 Nerve 拷贝、改名
    session.ts                              会话装配：公开客户端、令牌管理器、续期锁的选择、带令牌的客户端
    *.test.ts；testing/{fake-browser,fake-server,fake-time}.ts  拷来的测试与替身
  lib/one-at-a-time.ts                      写请求按顺序发出（拷贝）
  app/
    session-root.tsx                        订阅会话，每个 loginId 一代 RootStore，带 key 挂载 AppProviders
    routes.tsx                              守卫与页面的路由表
    guards.tsx                              GuestOnly、SignedIn、Onboarded
    next-path.ts                            next 的校验
    problem-messages.ts                     problem 码与字段码到文案键
    retry.ts                                SWR 的重试策略
    user-menu.tsx                           顶栏：显示名、退出
    session-unavailable.tsx                 会话暂不可用
  pages/sign-in.tsx、sign-up.tsx、onboarding.tsx；credentials-form.tsx   登录、注册共用的邮箱与密码表单
  onboarding/steps.ts、profile-step.tsx      步骤注册表（组件按需加载）；第一步
  services/api.ts                           unwrap 接受 204；ApiError 带 retryAfter
  services/auth.service.ts、account.service.ts
  stores/root.store.ts                      AppStores（页面的一生：设备偏好、实例信息、会话）；RootStore（一代：共用的 + 本次登录的账户）
  stores/context.tsx                        useStore；useAccount（SignedIn 之下，已加载的账户）
  stores/auth.store.ts、account.store.ts
  components/ui/input.tsx、label.tsx、alert.tsx
  components/form-field.tsx、loading.tsx    带文案，不是 ui 原件：标签、输入、字段错误、明文切换；加载中
  i18n/messages/{en,zh-CN}.ts
.oxlintrc.json                              createClient 的放行从 main.tsx 移到 session/
e2e/fixtures/{test,browser,auth,auth-pages,onboarding-pages}.ts；stories/identity/a1–a6、a9、a14（页面版本）；smoke/s2、s4
```

### 3.2 会话模块

**拷贝**（M1 总设计第 6 节）：Nerve 的 `token-manager.ts`、`refresh-lock.ts`、`auth-middleware.ts`、它们的 6 个测试文件与 3 个替身（`fake-nerve.ts` 改名 `fake-server.ts`），`one-at-a-time.ts`。它们只依赖 api-client 的类型，与框架无关。改名：记录 `nwiki.auth`（`{refresh_token, login_id}`，访问令牌只在内存）、锁 `nwiki.auth.refresh`、租约 `nwiki.auth.lease`；删掉指向 Nerve 文档的注释。行为不改：

- **login_id**：每次登录或注册由前端生成（16 字节随机，`crypto.getRandomValues`，非安全上下文也有），续期沿用，服务端不认识它。它区分"别的标签页续期了"（同一个 login_id，什么也不做）与"别的标签页登录了"（不同，跟过去）。
- **状态**：`starting`（第一次续期中）、`signed-in`、`signed-out`、`unavailable`（第一次续期因 429、5xx、断网、超时失败，记录保留，到 `retryAt` 再试）。没有记录时直接 `signed-out`，不发请求。
- **续期**：到期前 30 秒续期；同一标签页的并发请求共用一次续期；锁内先读记录，已被别的标签页换掉就跟过去；只有续期答 401 才删除记录。请求超时 8 秒，与服务端的 `webRefreshTimeout` 是同一个数：服务端 4 秒加提交 2 秒 < 客户端 8 秒 < 租约 10 秒。两边的常量各自注明对方的位置。
- **续期锁**：有 `navigator.locks` 用 Web Locks；没有（非安全上下文，例如局域网的 HTTP）用 localStorage 租约（写入、等 100 毫秒读回确认，storage 事件加 200 毫秒轮询）。
- **认证中间件**：每个客户端绑定一个 login_id；会话已变就以 `SessionChangedError` 结束请求；401 时续期一次、重发一次，仍是 401 就结束会话；续期不可用时以 `SessionUnavailableError` 结束。
- **退出**：锁内读记录，还是自己的会话才用最新的刷新令牌调 `logout`（尽力而为），删除记录；其他标签页经 storage 事件跟着退出。

**测试的运行环境**（M1 总设计第 10 节的风险）：拷来的测试在 Nerve 里跑在 node 环境。vitest 5 的 jsdom 环境保留 Node 的 `fetch`、`AbortSignal`，`Request` 换成与之兼容的类，所以先让它们在本项目的 jsdom 与 `test/setup.ts` 下原样运行（控制台检查也覆盖它们）。不行时这几个文件声明 node 环境，`setup.ts` 对 `document` 的操作加判断。S1 的第一件事就是确认，结果记入第 7 节。

**装配**（全新编写，`session/session.ts`）：

```ts
export type SessionDeps = {
  storage: Pick<Storage, "getItem" | "setItem" | "removeItem">;
  onStorage: (listener: (key: string | null) => void) => () => void;   // 别的标签页改了哪个键
  locks: Pick<LockManager, "request"> | undefined;   // navigator.locks；没有时用租约
  now: () => number;
  randomHex: (bytes: number) => string;
  client?: ClientOptions;                  // 测试注入 fetch；页面同源
};
// browserSessionDeps(storage)：只收 storageArea 是这个 storage 的事件；有 navigator.locks 就用它

export class Session {
  readonly public: ApiClient;              // 不挂中间件：实例信息、注册、登录，以及令牌管理器自己的续期与退出
  readonly tokens: TokenManager;
  clientFor(loginId: string): ApiClient;   // 挂上绑定 loginId 的认证中间件
  start(): Promise<void>;                  // 订阅 storage 事件，开始第一次续期；只执行一次
  dispose(): void;                         // 取消订阅（测试用）
}
```

- `main.tsx` 用浏览器的依赖构造它、显式调用 `start()`：导入任何模块都没有副作用（M1 总设计第 4 节放弃 Nerve 的模块级单例的理由）。localStorage 不可用时（隐私模式抛错）退回内存，与设备偏好一致：登录只在本页有效。
- 公开客户端也只在这里创建：登录会答 401，不能经过会续期的中间件。

**oxlint**：`createClient` 的放行从 `main.tsx` 移到 `src/session/**`（移交第 4 项），`test/**` 照旧；services 与 stores 只能导入类型。api-client 另导出 `Middleware` 类型（它本来就依赖 openapi-fetch），`apps/web` 不直接依赖 openapi-fetch。

### 3.3 每次登录一代

```
main.tsx
  └ SessionRoot(app: AppStores(preferences, session), router)
      useSyncExternalStore(session.tokens.subscribe, () => session.tokens.state)
      store = useMemo(() => new RootStore(app, state.loginId), [app, state.loginId])
      └ <AppProviders key={state.loginId ?? "signed-out"} store={store}>
          └ <RouterProvider router={router} />     路由器在 main.tsx 创建一次：换代不丢位置
```

- **一代**：页面一生只有一个 `AppStores`（设备偏好、实例信息、会话）；`RootStore` 持有其中共用的 `preferences`（设备）、`instance`（公开信息）、`auth`（登录、注册、退出，用公开客户端与令牌管理器），以及本次登录的 `account`（`AccountStore`，客户端来自 `session.clientFor(loginId)`；未登录时没有）。`loginId` 变了（登录、退出、别的标签页换账户）就换一代；同一个 `loginId` 的 `starting`、`signed-in`、`unavailable` 是同一代。
- **SWR 的缓存**：每代重新挂载 `AppProviders`，`SWRConfig` 随之新建缓存（移交第 2 项）。缓存已经按代隔离，键不再带 `loginId`：总体设计 9.4 的这条约定由按代的缓存取代（同步修订）。
- **旧一代的请求**：认证中间件绑定 login_id，换代之后旧客户端的请求以 `SessionChangedError` 结束（移交第 1 项）；旧一代的组件已经卸载；store 不吞掉这个错误，由用到它的地方忽略：`errorText` 对它不给文案，`SignedIn` 取 `/me` 遇到它显示加载中，SWR 不重试它。
- **写请求的顺序**：`AccountStore` 的写操作（`updateMe`、`recordOnboardingStep`，都答完整的 `User`）经 `oneAtATime` 依次发出，最后到达的答复就是最后一次写入的结果。读（SWR 取 `/me`，聚焦时也会）不排队：读出去之后有写答复了，读到的可能是写之前的行，丢弃它，保留写的答复（审查 M1）。

### 3.4 请求的错误

- **204**：`unwrap` 在 `response.ok` 时返回 `data`（204 为 `undefined`）；声明了响应体的操作答了空体才是错误。退出（P5）与 P6 的改密码、停用、撤销都答 204。
- **`ApiError.retryAfter`**：从 `Retry-After` 头读秒数（429 `rate_limited`、503 `server_busy`）。
- **SWR 的重试**（`app/retry.ts`，移交第 3 项）：纯函数 `retryDelay(error, attempt): number | undefined` 接到 `onErrorRetry`（重试时原样带上 SWR 给的选项，保留去重）：
  - `SessionChangedError`：不重试（这一代已经结束）；
  - 带 `retryAfter` 的：按它等，最多 5 次；
  - 其他 4xx：不重试；
  - 5xx、断网：指数退避（5 秒起，封顶 60 秒），最多 5 次。
- **表单**（登录、注册、引导）：不自动重新提交。429 显示"尝试太频繁，请 N 秒后再试"（N 取 `retryAfter`），503 `server_busy` 显示"服务器繁忙"。
- **文案**（`app/problem-messages.ts`）：problem 码到文案键、字段码到文案键；没有映射的码显示通用的错误与码本身。测试以 Vite 的 `?raw` 导入 `api/dist/openapi.yaml`、按行读（不引入 YAML 依赖），页面会显示其错误的每个操作（`getInstance`、`register`、`login`、`getMe`、`updateMe`、`recordOnboardingStep`；续期与退出的答复只由令牌管理器处理）的 `x-problem-codes` 与平台码都要有文案：契约加一个码，测试就失败，直到它有文案。

### 3.5 路由与守卫

```
Layout（顶栏：品牌、语言、主题；登录后加用户菜单）
├ GuestOnly                 starting：加载中；signed-in、unavailable：去 next 或 /
│ ├ /sign-in?next=
│ └ /sign-up?next=
└ SignedIn                  starting：加载中；unavailable：会话暂不可用；signed-out：去 /sign-in?next=<原路径>
  │                         取 /me；引导所需的 onboarding_steps 在这里取得
  ├ /onboarding?next=       已完成就去 next 或 /
  └ Onboarded               有未完成的步骤就去 /onboarding?next=<原路径>
    ├ /（首页，M2 换成工作区）
    └ *（应用内 404）
```

- **所有页面都要登录**，只有登录、注册两页例外；不存在的路径也先登录，再显示 404：路由规则只有一条，M2、M3 加的真实路由与今天的未知路径表现一致。
- **`next`**（`app/next-path.ts`）：照 Nerve 的规则重写并拷贝它的测试用例（M1 总设计第 6 节的补充）：有控制字符的拒绝；去掉首尾空白之后必须以 `/` 开头；拒绝 `//` 开头与任何 `\`；否则当作没有 `next`，去 `/`。满足这三条的只能被解析为本站的路径（第二个字符既不是 `/` 也不是 `\`，URL 解析器只能进入路径），Nerve 的同源解析与恶意路径模式都不再需要：它们与这三条完全重叠，反向对照分不出来。参数名是 `next`（M1 总设计第 4 节），不是 Nerve 的 `next_path`。`withNext(page, path)` 生成带 `next` 的登录页与引导页地址（`/` 不带），`keepNext` 让登录、注册页互相链接时带上它。
- **会话暂不可用**：不是路由，在当前地址上显示（`role="alert"`）：说明"仍在登录状态，暂时连不上服务器"，按钮"重试"调 `tokens.retry()`；令牌管理器自己也会在 `retryAt` 重试。另有"退出"：连不上服务器也能在本浏览器忘掉这个会话（审查 M2）。取 `/me` 失败（断网、5xx）同样显示它，重试调 SWR 的 `mutate`。
- **首页**：M0 的实例版本照旧；M2 换成工作区的外壳。

### 3.6 页面

- **登录**：邮箱、密码（可显示明文的切换按钮）；注册入口按 `instance.signup_enabled` 显示。错误：401 `invalid_credentials` 与 403 `account_deactivated` 显示在表单上方，输入保留；429、503 见 3.4。成功之后 `auth.signIn(email, password)` 取得令牌、交给令牌管理器写入记录、换代，`GuestOnly` 带去 `next` 或 `/`：页面自己不跳转，守卫是唯一决定去处的地方。
- **注册**：邮箱、密码（同样可切换明文，不要确认框：切换按钮让用户看得见自己输入的，NIST 800-63B 也建议这样）。本地只检查邮箱非空与密码 8–128 个字符（UTF-16 长度，与服务端相同）；常见密码只有服务端知道，422 的字段错误显示在字段下方，409 `email_taken` 显示在表单上方。注册关闭时页面显示"本服务器未开放注册"与回登录页的链接，不显示表单；硬提交答 403 也有文案。成功之后同登录，守卫把新账户带到 `/onboarding`。
- **用户菜单**（顶栏，登录之后）：显示名与"退出"。退出调 `tokens.signOut()`，状态变为 `signed-out`，所有标签页的守卫把它们带到登录页。P6 在菜单中加设置的入口（M1 总设计第 7 节 P6 的"顶栏的用户菜单"提前到这里，A6 的页面版本需要退出）。
- **表单组件**：按 shadcn 的写法新增 `Input`、`Label`（必须有 `htmlFor`）、`Alert`，以及把标签、输入、字段错误（`aria-invalid`、`aria-describedby`）连起来的 `FormField`（`components/form-field.tsx`：它带文案，不放进 `ui/`）。明文切换的按钮标签固定，状态在 `aria-pressed`。提交中按钮禁用，防止重复提交；提交失败之后焦点移到第一个有错误的字段，读屏随之读出错误（审查 M4）。`formErrors(error, t, shown)`：422 的问题全在表单显示的字段上时只显示在字段下方，否则显示在表单上方。

### 3.7 新手引导

**步骤注册表**（扩展点，M1 总设计第 8 节）：

```ts
// onboarding/steps.ts
export type OnboardingStep = {
  id: string;                               // 服务端记录的 id：[a-z][a-z0-9_]{0,31}
  title: Extract<MessageKey, `onboarding.${string}.title`>;
  Component: ComponentType<{ complete: () => Promise<void> }>;   // React.lazy：随引导页加载
};
export const onboardingSteps: readonly OnboardingStep[] = [
  { id: "profile", title: "onboarding.profile.title", Component: lazy(() => import("./profile-step")…) },
];   // M2、M3 在后面追加
```

- 守卫在每个页面都读注册表，只需要 id；步骤的组件用 `React.lazy` 随引导页加载，不进入口的包（审查 N2）。
- **未完成的步骤** = 注册表中 id 不在 `me.onboarding_steps` 里的，按注册表的顺序；为空就是引导完成。服务端记着、注册表里已经没有的 id 忽略。M2 加一步之后，老用户下次访问也会看到它：注册表决定"完成"的含义（M1 总设计第 4 节）。
- **页面**：显示"第 i 步，共 n 步"与第一个未完成步骤的组件；组件完成之后调 `complete()`：`account.recordStep(id)`（幂等），`me` 随答复更新，页面显示下一步；没有了就去 `next` 或 `/`。
- **第一步"个人资料"**（id `profile`）：显示名，预填当前的（注册时取邮箱 @ 之前的部分）。"继续"：改过就先 `updateMe`（422 显示在字段下方），再记录这一步；两个写请求经 `oneAtATime` 依次发出。失败时停在这一步，再点"继续"重做：两个请求都是幂等的。
- **测试**：注册表的 id 符合服务端的格式、不重复、不超过 32 个。

### 3.8 前端的测试（vitest）

| 测试 | 守住 |
|---|---|
| 拷来的令牌管理器、续期锁、中间件的测试（多标签页的用例分别在 Web Locks 与租约下跑） | 3.2 的行为原样 |
| 装配：没有记录不发请求；公开客户端没有中间件；`clientFor` 绑定 loginId；storage 事件接到令牌管理器与租约；`locks` 缺失时用租约 | 3.2 |
| 分代：换代之后旧一代的请求以 `SessionChangedError` 结束、不写入新一代；新一代的 SWR 缓存是空的；偏好与实例信息跨代保留 | 3.3 |
| `unwrap` 的 204；`ApiError.retryAfter`；`retryDelay` 的判定表 | 3.4 |
| 文案表与契约的核对 | 3.4 |
| 守卫：每个会话状态与引导状态的去处；`next` 的判定表（含 Nerve 的恶意用例） | 3.5 |
| 页面：字段错误、表单上方的错误、429 的秒数、注册关闭、重复提交 | 3.6 |
| 引导：注册表的顺序与格式；记录之后显示下一步；完成之后离开；失败停在原步 | 3.7 |

### 3.9 端到端

**夹具**：
- `signedInPage(tokens, baseURL?)`：故事用接口注册账户，把令牌交给它，经 `addInitScript` 在这个源第一次加载时写入 `nwiki.auth`（新的 login_id 与刷新令牌），只写一次（标记键），之后的加载、刷新与其他标签页都用页面自己维护的记录；访问令牌不进浏览器，页面第一次加载时自己续期。`recordOf(page)`、`writeRecord(page, record)`（在 `nwiki.auth.refresh` 锁下写入，模拟别的标签页登录）。
- **预期的控制台输出**：页面上的每个 4xx 都会在 Chromium 的控制台留下一条 "Failed to load resource" 错误。`pageWatch.expectConsole({errors, warnings})` 逐条声明，测试结束时的自动检查要求控制台恰好是声明的加上探针，按顺序。没有声明的仍然一条也不许有。M0/P6 文档里的名字 `expectQuietConsole` 改为代码中的 `expectQuietPage`。
- `auth-pages.ts`：填写、提交登录与注册表单（等对应接口的答复），表单上方的错误。

**故事的页面版本**：

| 故事 | 页面版本 |
|---|---|
| A1 注册 | `/sign-up` 提交之后到 `/onboarding`，是应用内跳转；浏览器只存 `nwiki.auth`（恰好 `login_id` 与 `refresh_token`），没有 cookie；两个令牌不出现在其他存储、地址、控制台与请求地址中 |
| A2 注册被拒 | 409 在表单上方、输入保留；常见密码的 422 在字段下方；注册关闭（另起的服务）时登录页没有入口、注册页显示关闭；硬提交（`page.route` 让页面以为注册开放）答 403 的文案 |
| A3 登录 | 带查询与片段的深链登录之后回到原处；已登录访问带恶意 `next`（`//evil.example`、`/\evil.example`、`javascript:alert(1)`、`/\t/evil.example`）的登录页去 `/`；错误的密码与不存在的地址同样的错误、输入保留 |
| A4 续期 | 访问令牌 3 秒过期（另起的服务），两个标签页同时保存引导的一步（第一次续期扣住 1 秒，没有锁时另一个的续期就会与它重叠），续期的代数恰好 0、1、2…各一次；有与没有 `navigator.locks` 各跑一遍 |
| A5 重复使用检测 | 测试先用页面的刷新令牌续期一次，页面刷新之后续期答 401，回到登录页，记录被删 |
| A6 退出 | 一个标签页退出，两个标签页都到登录页，会话 `logout`；另一个标签页写入别的账户的记录，第一个标签页跟着换账户，旧会话不被吊销；退出之后在另一个标签页登录，两个都跟上 |
| A9 新手引导 | 新账户访问深链先到引导（"第 1 步，共 1 步"，预填的显示名），改显示名、继续，引导结束回到深链；再访问 `/onboarding` 直接离开；空的显示名在本地拦下，过长的（101 个字符）422 在字段下方 |
| A14 登录限流 | 小桶的服务上表单依次显示错误的密码、错误的密码、"请 N 秒后再试" |

- **冒烟故事**：S2 未登录访问 `/` 到登录页，只请求 `GET /api/v0/instance`（不续期、不取 `/me`）；登录之后首页照旧显示版本。S4 未登录的深链到 `/sign-in?next=…`，同样只请求实例信息；登录之后显示 404，刷新之后仍是。
- A5 的页面版本是 M1 总设计第 3 节没有列的（同步修订）：只有它证明前端把"会话被吊销"变成"回到登录页"。

**浏览器实测**（记入第 7 节）：局域网地址的 HTTP（非安全上下文，没有 `navigator.locks`，走租约）两个标签页的续期与退出；同一浏览器两个账户（两个配置文件）各两个标签页互不干扰。

### 3.10 对 M1 总设计的修订

- 第 3 节故事表：A5 的版本改为"页面、接口"。
- 第 6 节拷贝清单补充：Nerve 的 `next` 校验规则与用例、problem 码的文案映射与它的契约测试、e2e 的 `signedInPage` 与记录的辅助函数。
- 第 7 节：用户菜单（显示名与退出）从 P6 提前到 P5，P6 加设置的入口。
- 总体设计 9.4：SWR 的键不再带 `loginId`，由按代的缓存取代。

## 4. 实施步骤

在分支 `m1-p5-web-session` 上依次进行，每个 Step 结束时 `make check` 为绿。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 会话模块：确认测试环境；拷贝与改名；`Middleware` 类型；装配；oxlint 的放行；`main.tsx` 用会话的公开客户端（页面不变） | [P5-S1-session.md](plans/P5-S1-session.md) |
| S2 | 分代与请求的错误：`RootStore` 一代、`SessionRoot`、`AccountStore`、`oneAtATime`；`unwrap` 的 204、`retryAfter`、重试策略；文案映射与契约测试 | [P5-S2-generations.md](plans/P5-S2-generations.md) |
| S3 | 登录与注册：表单组件、守卫与路由表、`next`、登录与注册页、会话暂不可用、用户菜单与退出 | [P5-S3-sign-in.md](plans/P5-S3-sign-in.md) |
| S4 | 新手引导：步骤注册表、引导页、个人资料一步、`Onboarded` 守卫 | [P5-S4-onboarding.md](plans/P5-S4-onboarding.md) |
| S5 | 端到端（一）：`signedInPage`、预期的控制台输出、页面辅助；S2、S4；A1、A2、A3、A14 的页面版本 | [P5-S5-e2e-sign-in.md](plans/P5-S5-e2e-sign-in.md) |
| S6 | 端到端（二）：A4、A5、A6、A9 的页面版本；浏览器实测 | [P5-S6-e2e-tabs.md](plans/P5-S6-e2e-tabs.md) |

规模估计：拷贝的生产代码约 580 行、测试与替身约 2,000 行；新写的生产代码约 1,300 行（页面、守卫、装配、stores、组件、文案），测试约 1,500 行，端到端约 900 行。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元（vitest） | 第 3.8 节 |
| 端到端 | 第 3.9 节；本 M 与 M0 之前的故事照旧通过 |
| 浏览器实测 | 局域网 HTTP 的租约；两个账户 |

反向对照（验证后撤销）：
- 认证中间件不检查会话是否已变 → 分代的测试失败；
- `AppProviders` 不带 key → SWR 缓存的测试失败；
- `next` 放过 `//evil.example` → 判定表与 A3 失败；
- 续期锁换成空锁（不串行）→ A4 的代数断言与拷来的多标签页测试失败；
- 登录页自己跳转、绕过守卫 → 已登录访问登录页的守卫测试失败；
- `retryDelay` 对 429 不重试 → 判定表失败；
- 契约加一个码而文案表没有 → 契约核对失败；
- 退出不广播（不删记录）→ A6 失败。

## 6. 完成标准

- 第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 本地与持续集成为绿。
- 审查完成（`reviews/P5-web-session-review.md`），发现的问题已修复。
- M1 总设计进度表更新；M0/P5 移交的 4 项核对。

## 7. 结果

分支 `m1-p5-web-session`：S1 `392e2e9`、S2 `5c212cd`、S3 `02f61d8`、S4 `7f43b97`、S5 `cf54c7f`、S6 `6446141`，审查修复 `2793ede`。第 5 节全部通过，反向对照按预期失败；`make check`（vitest 303 个）、`make gen-check`、`make e2e`（41 个；另跑 `--repeat-each 3` 与 `--workers 1`）、`make image-smoke`（在克隆上，`modified=false`）本地与持续集成为绿。S3、S4 的持续集成只有端到端失败（S2、S4 的冒烟故事，按计划在 S5 调整）。审查见 [P5 审查记录](reviews/P5-web-session-review.md)：4 项 Minor、7 项 Nit，N7 的后半不做，其余已修复。规模：生产代码约 1,930 行（其中拷贝约 560 行），测试约 3,360 行（其中拷来的测试与替身约 2,100 行），端到端约 790 行；与估计相当。

与设计的出入（已同步进上文）：

1. 拷来的测试在本项目的 jsdom 与 `test/setup.ts` 下原样通过（会话的 108 个），M1 总设计第 10 节的风险解除，没有文件需要声明 node 环境。
2. 装配：`SessionDeps` 的 `onStorage` 只交出键名，`locks` 只要 `request`，`baseUrl` 换成 `client`（测试注入 fetch）；`browserSessionDeps(storage)` 只收这个 storage 的事件，有 `navigator.locks` 就用它。
3. 页面一生的 `AppStores`（设备偏好、实例信息、会话）与每代的 `RootStore` 分开；`SessionRoot` 只收 `AppStores` 与路由器。
4. store 不吞掉 `SessionChangedError`：`errorText` 对它不给文案，`SignedIn` 显示加载中，SWR 不重试。
5. `AuthStore` 另公开 `state`、`subscribe`、`retry`（守卫经它读会话），`signIn(email, password)` 自己取令牌。
6. 文案表的契约测试以 `?raw` 导入 `openapi.yaml`。
7. `next`：Nerve 的恶意路径模式与同源解析都不要，只留三条规则（理由见 3.5；同源解析在的时候，放过 `//` 的反向对照不失败）。`signInPath` 推广为 `withNext(page, path)`（登录页与引导页共用，`/` 不带 `next`），另有 `keepNext`。
8. 表单：`FormField` 在 `components/`（带文案）；登录、注册共用 `pages/credentials-form.tsx`；`formErrors` 在文案表旁边；`Label` 必须有 `htmlFor`（oxlint 的 a11y 规则）。
9. `GuestOnly` 在 `starting` 显示加载中，在 `unavailable` 去 `next`（仍在登录状态，登录页不是它该显示的）；3.5 原来只写了已登录的情形。
10. 注册页等实例信息到了再决定显示表单还是"未开放注册"；取不到时显示表单，硬提交的 403 有文案。
11. 引导：`useAccount` 放在 `stores/context.tsx`（放在守卫里会形成守卫 → 注册表 → 步骤 → 守卫的导入环）；引导页的组件 `Onboarding({ steps })` 可以注入注册表（测试用两步）；"第 i 步，共 n 步"按注册表的位置；个人资料一步本地只查非空，名字去掉首尾空白，没改就不发 `updateMe`。
12. 端到端：`signedInPage` 收令牌，由故事注册；另有 `onboarding-pages.ts`、`newRecord`、`writeRecord`、`followAccessToken`、`failedToLoad`、`generationOf`、`displayNameOf`、`completeOnboarding`。A2 的硬提交靠 `page.route` 改写实例信息；A3 的恶意 `next` 以已登录访问登录页检查；A9 的 422 用 101 个字符（空的在本地拦下）；A6 以应用内 404 的 `/acme` 作为所在的页面；`saveProfileStep` 在标签页到了登录页时答 "signed out"，锁失效时 A4 几秒内失败并说出原因，而不是等到超时。
13. 登录之后新一代的 SWR 会再取一次实例信息（缓存按代，实例信息的 store 跨代保留），S2 只核对登录之前的请求。
14. 审查之后：读的答复不覆盖之后答复的写（M1）；会话暂不可用时可以退出（M2）；`GuestOnly` 两个状态与重试接线的测试（M3）；提交失败之后焦点移到第一个有错误的字段（M4）；重试保留 SWR 的选项（N1）；步骤组件按需加载（N2）；去掉 `root.store.ts` 的 oxlint 放行（N3）；明文切换的标签固定（N4）；拷贝文件的措辞与 `first_name`（N5）；`Loading` 单独成文件、是 `output`，首页也用它（N6）；422 的问题在表单没有的字段上时显示在上方（N7）。

浏览器实测（S6，临时的 Playwright 脚本，没有提交）：

- 局域网地址的 HTTP：服务监听 `10.10.20.104`，访问令牌 3 秒。页面 `isSecureContext` 为 false、没有 `navigator.locks`，走租约。两个标签页同时保存引导的一步，6 次续期的代数 0–5 各一次、都是 200，另一个标签页收到租约键的 storage 事件；一个标签页退出，两个都到登录页，记录被删。
- 两个配置文件（两个浏览器上下文）、两个账户各两个标签页：一个账户退出，它的两个标签页到登录页；另一个账户的两个标签页不受影响，刷新之后仍在登录状态。

留给之后的：P6 的设置页读写账户经同一个 `AccountStore`（M1 的修复同样保护显示名不回弹）；引导的新步骤按 3.7 追加，组件照样 `lazy`。
