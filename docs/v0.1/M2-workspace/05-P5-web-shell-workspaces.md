# M2/P5 前端外壳与工作区：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M2/P5 前端外壳与工作区 |
| 状态 | 进行中 |
| 基线 | `b4657d0`（P4 合并、P4 文档更新之后的 main） |
| 上级文档 | [M2 总设计](00-M2-design.md) 第 1 节第 6 条、第 3、4、7、9 节；[M1 移交](handoffs/M1-identity.md)第 5、6 项；[总体设计](../v0.1-design.md) 9.2、9.4、13.2 |

---

## 1. 基线

P1–P4 之后，工作区的 15 个接口都已就绪，P5 用到的是：
- `listWorkspaces`：调用者有效成员关系所在的工作区，按名称排序，每个带调用者的 `role`。
- `createWorkspace`：关闭时答 403 `workspace.creation_disabled`。
- `checkWorkspaceSlug`：`{available, reason?: taken | reserved | invalid}`。
- `updateWorkspace`（改名）、`deleteWorkspace`：非管理员答 403 `forbidden`。
- `GET /instance` 的 `workspace_creation_enabled`。

前端还停在 M1：
- `/` 是 M0 的实例版本页；
- 引导只有 `profile` 一步；
- 文案表已有工作区的 8 个码（契约核对要求）；
- 保留名单的 `[app]` 段由 vitest 核对路由的顶层段，`create-workspace` 还在 `[reserved]`。

M1 移交留给本 Phase 的两项：
- 第 5 项，引导的新步骤：注册表追加，e2e 的 `onboardingSteps` 同步，A9 的步数随之改变。
- 第 6 项，首页：换成工作区的外壳，S2 随之调整。

## 2. 目标与范围

**目标**：登录之后落进工作区。左栏切换工作区；没有工作区时去创建，或者等邀请。管理员在常规设置里改名、删除。新账户在引导中创建工作区。W1–W4、W11 的页面版本通过；W3、W11 的接口版本补上。

**做**：
- **外壳**：`/:slug` 的工作区布局，左栏有工作区切换与导航，笔记本的位置留给 M3；工作区首页。
- **落点**：`/` 按"最后访问的工作区 → 按名称排第一的 → 创建页"落下，由纯函数决定；最后访问的工作区记在设备上（`nwiki.workspace`）。
- **创建工作区页** `/create-workspace`：名称与 slug；slug 随名称生成，直到用户改它；本地检查格式，接口检查可用性。创建关闭时，这一页说明怎样才能进入工作区。
- **工作区设置**：`/:slug/settings/general`，管理员改名、删除，删除要输入 slug 确认；P6 加成员页。
- **引导的工作区一步**（id `workspace`）。
- **首页与版本**：`HomePage` 去掉，实例的版本移到用户菜单。
- **测试**：S2、A3、A4、A6、A9 随首页与引导调整。

**不做**：

| 事项 | 理由 |
|---|---|
| 成员页、离开、公开的邀请页 | P6 |
| 左栏的折叠、拖动宽度、窄屏的抽屉 | 左栏现在只有切换与两个链接；M3 加页面树时一并设计 |
| 改 slug | 契约里 slug 创建之后不变（地址稳定） |
| 引导里"跳过、以后再说" | 注册打开时，没有工作区的账户落到创建页，跳过之后还是这一页。等邀请的人随时可以打开邀请链接（P6） |
| 在前端按保留名单检查 slug | 检查接口已答 `reserved`；前端再带一份名单，就要和服务端的名单保持一致 |
| 跨设备的最后访问 | M2 总设计第 4 节：记在设备上 |

## 3. 设计

### 3.1 文件

```
server/internal/modules/workspace/domain/reserved_slugs.txt   create-workspace 从 [reserved] 移到 [app]
web/apps/web/src/
  app/routes.tsx                      /、/create-workspace、/:slug 与它的子路由
  app/layout.tsx                      main 的内边距：外壳（data-shell）自己负责
  app/landing.ts                      落点：纯函数
  app/user-menu.tsx                   实例的版本
  app/confirm-dialog.tsx              可选的"输入 … 以确认"
  app/create-workspace-form.tsx       名称、slug、可用性：创建页与引导的一步共用
  app/slug.ts                         slug 的本地检查、由名称生成 slug
  pages/landing.tsx                   / ：读列表，转到落点
  pages/create-workspace.tsx          创建页；关闭时的说明
  pages/workspace/
    workspace-layout.tsx              左栏与内容；不是成员的 slug 显示 404；记下最后访问；useWorkspace
    workspace-switcher.tsx            切换：列表、当前、创建入口
    workspace-home.tsx                工作区首页（M3 放笔记本）
    settings-layout.tsx               工作区设置的导航（P6 加成员）
    general-page.tsx                  改名、删除
  onboarding/steps.ts                 追加 workspace
  onboarding/workspace-step.tsx       已有工作区就继续；否则创建，或看说明后继续
  services/workspace.service.ts       list、create、update、remove、checkSlug
  stores/workspace.store.ts           一代的工作区列表
  stores/root.store.ts、context.tsx   一代另有 workspaces；useWorkspaces
  stores/preferences.store.ts         最后访问的工作区
  pages/home.tsx                      删除
  i18n/messages/{en,zh-CN}.ts
e2e/
  fixtures/auth.ts                    onboardingSteps 加 workspace
  fixtures/workspace-pages.ts         创建、切换、改名、删除、落点
  stories/workspace/w1、w2、w3、w4、w11
  stories/smoke/s2；stories/identity/a3、a4、a6、a9
```

### 3.2 路由与外壳

```
SignedIn
└ Onboarded
  ├ /                      落点（3.3）
  ├ /create-workspace      创建页（3.5）
  ├ /settings/…            账户设置（M1）
  ├ /:slug                 WorkspaceLayout：左栏 + 内容
  │ ├ (index)              工作区首页
  │ └ settings             工作区设置的布局；index 转到 general
  │   └ general            常规（3.6）
  └ *                      404
```

**`/:slug` 不需要单独请求工作区**：
- 布局用这一代的工作区列表（SWR 的键 `workspaces`），在列表里找 slug：
  - 找到：渲染左栏与内容，并记下最后访问（3.3）；
  - 列表已加载而没有它：显示 404 页（W3"不是成员的 slug 答 404"），包括格式不对的 slug；
  - 加载中、加载失败：显示加载中，或错误与"重试"。
- 切换器本来就要这份列表，再单独 `GET /workspaces/{slug}` 只是多一个请求。列表在获得焦点时重新读取：在别处被移出或删除的，下一次读取之后就变成 404。
- 子页用 `useWorkspace()` 取当前的工作区（列表里的那一项，随改名更新）。布局在找到之前不渲染子页。

**左栏**（宽屏在左，窄屏在上）：
- 切换器：显示当前工作区的名称。展开后列出全部工作区，当前的带勾，每项是到 `/:slug` 的链接；创建打开时，最后一项是"创建工作区"。
- 导航：首页（`/:slug`）、设置（`/:slug/settings`）。
- M3 在两者之间放笔记本。

**外壳的边距**：顶栏下的 `main` 原来统一有内边距。外壳要贴边，所以 `main` 用 `has-[[data-shell]]:p-0`：页面里有外壳时，内边距由外壳自己给内容。其余页面不变。

**保留名单**：`create-workspace` 从 `[reserved]` 移到 `[app]`（vitest 核对路由的顶层段）。服务端读全部三段，行为不变。

### 3.3 落点与最后访问的工作区

```ts
// app/landing.ts
landingPath(workspaces: readonly { slug: string }[], last: string | undefined): string
```

依次是：
1. `last` 在列表里：`/${last}`；
2. 列表的第一个：服务端按名称排序；
3. 列表为空：`/create-workspace`。

创建关闭时，创建页自己说明（3.5），所以落点不读实例信息。

**落点页** `/`：
- 用 SWR 加载列表，然后 `<Navigate replace>` 到落点。
- 加载失败时显示错误与"重试"。
- 列表已在这一代的缓存里时，立即转走，同时重新读取。

**最后访问**：
- 存在 `PreferencesStore`：它属于设备，跨代存活。
- `lastWorkspace()` 每次读存储，所以别的标签页刚访问的也算；存储不可用时，退回本页的内存。
- `setLastWorkspace(slug)` 由工作区布局在找到工作区之后调用。
- 只存 slug，不存账户：同一设备换了账户，旧的 slug 不在新列表里，第 1 条不成立，落到第 2 条。

### 3.4 工作区的 service 与 store

**`WorkspaceService`**：`list()`、`create(body)`、`update(slug, body)`、`remove(slug)`、`checkSlug(slug)`。类型（`Workspace`、`WorkspaceCreate`、`SlugAvailability`）由它转出（13.2 第 6 条）。

**`WorkspaceStore`**（每代一个，`RootStore.workspaces`，未登录时为 `undefined`）：
- `list: Workspace[] | undefined`；`bySlug(slug)`。
- `load()`：由 SWR 调用。读出去之后已有写答复的，丢弃这次读（与 `ApiTokenStore` 相同的计数）。
- `create(body)`、`rename(slug, name)`：把答复放进列表。排序照服务端的 `lower(name), name, id`，按码位比较。服务端按数据库的排序规则比较，两者可能有少量差别，下一次读取时以服务端为准。
- `remove(slug)`：204 之后从列表删除。
- `checkSlug(slug)`：只转调 service，不存状态（同 `changePassword`）。
- 写不经 `oneAtATime`：各自改列表里不同的项，答复的都是单个工作区，不是整个列表。

### 3.5 创建工作区

**`CreateWorkspaceForm`**（创建页与引导的一步共用）：

字段：
- 名称：必填，1–80 个字符（去掉首尾空白之后）。
- slug：
  - 随名称生成，直到用户自己改它：转小写，`a–z0–9_-` 以外的连续字符换成一个 `-`，去掉首尾的 `-`，截到 48 个字符。中文名称生成的是空串，用户自己填。
  - 本地检查格式，与服务端的 `ValidSlug` 相同：1–48 个 `a–z0–9_-`。
  - 格式通过、停止输入 300 ms 之后调用检查接口（SWR 的键 `["workspace-slug", slug]`），字段下方显示"可以使用"或"已被占用""是保留名"。只在格式通过时请求。

提交：
- 本地检查不过不发。
- 答复的错误：
  - 409 `workspace.slug_taken` 放到 slug 字段下方（`onField`）；
  - 422 的字段错误放在各自字段下方；
  - 403 `workspace.creation_disabled`、`identity.account_deactivated` 显示在表单上方。
- 成功之后调用 `onCreated(workspace)`。

**创建页** `/create-workspace`：
- 读实例信息。创建打开时显示表单，成功之后转到 `/:slug`（W1"进入工作区"）。
- 关闭时不显示表单，说明工作区由服务器管理员创建，或者等成员邀请（W2"页面没有入口"；落点的"等待邀请"也是这一页）。
- 有工作区的人也可以来这一页（切换器的入口）；页面上有回到 `/` 的链接。

### 3.6 工作区设置：常规页

- 布局与账户设置相同：导航（常规；P6 加成员）与内容。
- **名称**：
  - 管理员看到改名表单：本地检查同创建，422 显示在字段下方。成功之后显示"已保存"，左栏随之更新。
  - 非管理员只读，说明只有管理员能改。
- **slug**：只读显示，说明创建之后不能改。
- **删除**（只有管理员）：
  - 区块写明后果：所有成员都看不到它；M3 起它的笔记本一并删除，包括成员的私密笔记本（M2 总设计第 4 节）。
  - `ConfirmDialog` 新增可选的 `typedConfirmation`：要输入 slug，输入一致之前"删除"按钮不可用；对话框关闭时清空输入。
  - 204 之后从列表删除并转到 `/`：落点选出下一个，或者到创建页。
  - 失败时对话框保持打开，显示原因：被降级之后答 403，已被删除答 404。

### 3.7 引导的工作区一步

注册表追加 `{ id: "workspace", title: "onboarding.workspace.title" }`，组件按需加载。组件读工作区列表与实例信息，然后按情况：

| 情况 | 显示 |
|---|---|
| 已有工作区（经邀请加入） | 直接继续：挂载时记录这一步一次（ref 守着，StrictMode 的重复 effect 不会再发）。失败时显示原因与"重试" |
| 没有，创建打开 | `CreateWorkspaceForm`，按钮是"创建并继续"。创建成功之后列表不再为空，于是进入上一行：记录由同一处完成，不另调 |
| 没有，创建关闭 | 说明（同创建页），"继续"记录这一步 |

- 引导结束之后去 `next`，没有时去 `/`，落点选出刚创建的工作区。
- 服务端只记录步骤的 id。创建、加入工作区的接口自己核对真实的成员关系，不以这一步为前提（M1 移交第 5 项）。

### 3.8 首页与用户菜单

- `HomePage` 删除，文案 `home.*` 删除。
- 用户菜单的最后一行是不可点的说明 `Nerve Wiki {version} ({commit})`，读实例信息（这一代的 SWR 键 `instance`），S2 由此核对版本已经打进构建。

### 3.9 前端的测试（vitest）

| 测试 | 守住 |
|---|---|
| `landingPath`：最后访问的在列表里、不在、列表为空 | 3.3 |
| `PreferencesStore` 的最后访问：读写、存储不可用时退回内存、别处写入的也读到 | 3.3 |
| slug：本地检查的表格；由名称生成（大小写、空白与符号、中文、截断） | 3.5 |
| `WorkspaceStore`：读不覆盖之后答复的写；创建、改名按名称排序；删除；新一代从空开始 | 3.4 |
| 路由：`/` 转到最后访问的、第一个、创建页；`/:slug` 不是成员时是 404；保留名单（已有的测试） | 3.2、3.3 |
| 外壳：切换器列出全部、标出当前、链接；创建关闭时没有入口；访问之后记下 slug | 3.2 |
| 创建：本地检查、slug 随名称直到被改、可用性三种答复、409 在 slug 下方、成功之后进入工作区；创建关闭时只有说明 | 3.5 |
| 常规页：管理员改名、422、已保存与左栏更新；输入 slug 之前不能删除；删除之后转到落点；失败时对话框保持；非管理员只读 | 3.6 |
| 引导的一步：已有工作区时自动记录一次；创建之后记录；关闭时说明与继续；记录失败与重试 | 3.7 |
| 用户菜单显示版本 | 3.8 |

### 3.10 端到端

**夹具**：
- `workspace-pages.ts`：
  - `createWorkspaceWith(page, {name, slug?})`：答复的状态码；
  - `switchWorkspace(page, name)`、`renameWorkspaceWith(page, name)`、`deleteWorkspaceWith(page, slug)`；
  - `expectCreatePage(page)`：没有工作区的账户落在 `/create-workspace`；
  - `workspaceHeading(page, name)`。
- `auth.ts` 的 `onboardingSteps` 加 `workspace`：`registerOnboarded` 的账户没有工作区，落在创建页。

**故事**：

| 故事 | 页面版本 | 接口版本 |
|---|---|---|
| W1 创建工作区 | 引导完成的账户落在创建页；slug 随名称生成；输入保留名、已占用的 slug，字段下方分别显示；提交被占用的 slug 答 409，字段下方显示；成功之后进入 `/:slug`，标题是名称。落库断言同接口版本 | 已有 |
| W2 创建开关 | 关闭的服务上：没有工作区的账户落在创建页，只有说明；已有工作区（经命令行创建）的切换器没有"创建工作区" | 已有（接口、命令行） |
| W3 外壳与切换 | 三个工作区（经接口创建）：<br>• `/` 落到按名称的第一个；<br>• 切换到第三个之后，`/` 落到它；<br>• 它被删除之后，落到第一个；<br>• 不是成员的 slug 显示 404；<br>• 全部离开或删除之后，落在创建页 | 新写：列表按名称；不是成员的 slug 答 404；离开之后不在列表里 |
| W4 改名与删除 | 管理员改名，左栏随之更新（落库同接口版本）。删除：输入 slug 之前按钮不可用；删除之后落到另一个工作区，所有成员都看不到它，slug 可以再用。成员看到只读的名称，没有删除 | 已有 |
| W11 引导的工作区一步 | 新账户：第 2 步创建工作区，引导结束之后进入它。经邀请加入的账户：第 2 步自动继续，进入那个工作区。关闭的服务上没有工作区：看到说明，继续之后落在创建页的说明 | 新写：PAT 记录 `workspace` 并创建工作区，落库相同 |

**随之调整**：
- S2：登录之后落在创建页；用户菜单显示版本。
- A3、A4、A6：登录之后的落点由"首页标题"改为 `expectCreatePage`。
- A9：步数 "Step 1 of 2"；第 2 步创建一个 slug 不是 `acme` 的工作区之后，回到 `/acme?view=list`（404 页）；引导完成之后访问 `/onboarding`，落到刚创建的工作区。

## 4. 实施步骤

分支 `m2-p5-web-shell-workspaces`，每个 Step 结束时 `make check` 为绿。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 外壳：<br>• 工作区的 service、store 与 `useWorkspaces`；<br>• 最后访问与落点；<br>• 路由：`/`、`/:slug`、工作区首页、404；<br>• 左栏与切换器；`main` 的边距；<br>• 首页删除、版本移到用户菜单；<br>• 保留名单 | [P5-S1](plans/P5-S1-shell.md) |
| S2 | 创建工作区：slug 的本地检查与生成、`CreateWorkspaceForm`、可用性、创建页与关闭时的说明 | [P5-S2](plans/P5-S2-create.md) |
| S3 | 工作区设置：设置的布局、常规页（改名、删除）；`ConfirmDialog` 的输入确认 | [P5-S3](plans/P5-S3-settings.md) |
| S4 | 引导的工作区一步 | [P5-S4](plans/P5-S4-onboarding.md) |
| S5 | 端到端：W1–W4、W11 的页面版本，W3、W11 的接口版本；S2、A3、A4、A6、A9 的调整；夹具 | [P5-S5](plans/P5-S5-e2e.md) |

规模估计：生产代码约 1,000 行，测试约 1,000 行，端到端约 600 行。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元（vitest） | 3.9 |
| 端到端 | 3.10；本 M 与之前各 M 的全部故事照旧通过 |

**反向对照**（13.4 第 1 条）：

| 改动 | 应当失败的测试 |
|---|---|
| 落点不看最后访问 | `landingPath`、路由测试、W3 页面版本 |
| 布局不记下最后访问 | 外壳测试、W3 页面版本 |
| `WorkspaceStore.load` 不管之后答复的写 | store 的交错测试 |
| 不是成员的 slug 照样渲染外壳 | 路由测试、W3 页面版本 |
| 创建关闭时切换器仍有入口 | 外壳测试、W2 页面版本 |
| 删除不要求输入 slug | 常规页测试 |
| 引导的一步在已有工作区时不自动继续 | 引导测试、W11 页面版本（邀请加入） |
| `[app]` 漏掉 `create-workspace` | 保留名单测试 |
| e2e 的 `onboardingSteps` 漏掉 `workspace` | 用 `registerOnboarded` 的故事 |

## 6. 完成标准

- 第 2 节"做"的各项完成；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 本地与持续集成为绿。
- M1 移交第 5、6 项落实，在 M2 总设计第 7 节的表中核对。
- 审查记录 `reviews/P5-web-shell-workspaces-review.md`；本文第 7 节、M2 总设计的进度表已更新。

## 7. 结果

（完成后补写）
