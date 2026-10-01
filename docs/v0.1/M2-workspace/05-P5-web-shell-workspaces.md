# M2/P5 前端外壳与工作区：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M2/P5 前端外壳与工作区 |
| 状态 | 已完成 |
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
  app/slug.ts                         slug 与名称的本地检查、由名称生成 slug
  app/not-loaded.tsx                  加载中，或失败的原因与"重试"：外壳、落点、创建页、引导的一步共用
  components/nav-item.tsx             导航的链接：外壳、工作区设置、账户设置共用
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
  services/workspace.service.ts       list、create、rename、remove、checkSlug
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
  - 列表已加载而没有它：显示 404 页（W3"不是成员的 slug 答 404"），包括格式不对的 slug；这个标签页刚删掉的转到 `/`（3.6）；
  - 加载中、加载失败：显示加载中，或错误与"重试"。
- 切换器本来就要这份列表，再单独 `GET /workspaces/{slug}` 只是多一个请求。列表在获得焦点时重新读取：在别处被移出或删除的，下一次读取之后就变成 404。
- 子页用 `useWorkspace()` 取当前的工作区（列表里的那一项，随改名更新）。布局在找到之前不渲染子页。
- 子页按工作区的 id 重新挂载（`<Outlet key={workspace.id}>`）：React Router 在参数变化时复用组件，表单里属于前一个工作区的状态（输入的名称、"已保存"、确认框的输入）不能带到下一个工作区（审查 T1）。M3 在 `/:slug` 下的页面同样由此重置。

**左栏**（宽屏在左，窄屏在上）：
- 切换器：显示当前工作区的名称，读屏另读出"切换工作区"。展开后是单选的菜单项，当前的选中，选一项转到 `/:slug`；创建打开时，最后一项是到创建页的链接"创建工作区"。用单选项而不是链接：读屏读得出当前是哪个，键盘操作是菜单现成的；在新标签页打开别的工作区，用地址栏（审查 Q3）。
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
- `lastWorkspace()` 每次读存储，所以别的标签页刚访问的也算；存储读不了，或者本页写不进（被阻止、已满）时，用本页的内存（审查 T14）。
- `setLastWorkspace(slug)` 由工作区布局在找到工作区之后调用。
- 只存 slug，不存账户：同一设备换了账户，旧的 slug 不在新列表里，第 1 条不成立，落到第 2 条。

### 3.4 工作区的 service 与 store

**`WorkspaceService`**：`list()`、`create(body)`、`rename(slug, name)`、`remove(slug)`、`checkSlug(slug)`。类型（`Workspace`、`WorkspaceCreate`、`SlugAvailability`）由它转出（13.2 第 6 条）。

**`WorkspaceStore`**（每代一个，`RootStore.workspaces`，未登录时为 `undefined`）：
- `list: Workspace[] | undefined`；`bySlug(slug)`。
- `load()`：由 SWR 调用。读出去之后已有写答复的，丢弃这次读（与 `ApiTokenStore` 相同的计数）。
- `create(body)`、`rename(slug, name)`：把答复放进列表。创建先按 id 去掉列表里已有的这一项：读取在创建提交之后发出、早于创建的答复回来时，它已含新项，而且不会被丢弃（审查 T3）。排序照服务端的 `lower(name), name, id`，按 UTF-16 码元比较；数据库按码位，两者只在 BMP 之外的字符上不同，`toLowerCase` 也有少数字符映射成两个。差别只到下一次读取为止，以服务端为准（审查 T9）。
- `remove(slug)`：204 之后从列表删除，并记下这一代删掉的 slug（`wasRemoved`，3.6）。答 404 `workspace.not_found` 也一样：已被删除或已被移出，这个账户都已经没有它（审查 Q1）。
- `checkSlug(slug)`：只转调 service，不存状态（同 `changePassword`）。
- 写不经 `oneAtATime`：各自改列表里不同的项，答复的都是单个工作区，不是整个列表。

### 3.5 创建工作区

**`CreateWorkspaceForm`**（创建页与引导的一步共用）：

字段：
- 名称：必填，1–80 个字符（去掉首尾空白之后，按码位）。超过时的文案用自己的键 `field.workspace_name.too_long`：服务端的 `field.name.too_long` 说的是账户显示名的 100。
- slug：
  - 随名称生成，直到用户自己改它：NFKD 分解并去掉附加符号（Café → cafe），转小写，`a–z0–9_` 以外的连续字符（`-` 也算）换成一个 `-`，去掉开头的 `-`，截到 48 个字符，再去掉末尾的 `-`。中文名称生成的是空串，用户自己填。
  - 本地检查格式，与服务端的 `ValidSlug` 相同：1–48 个 `a–z0–9_-`。
  - 格式通过、停止输入 300 ms 之后调用检查接口（SWR 的键 `["workspace-slug", slug]`），字段下方显示"可以使用"或"已被占用""是保留名"。只在格式通过时请求。

提交：
- 本地检查不过不发。
- 答复的错误：
  - 409 `workspace.slug_taken` 放到 slug 字段下方（`onField`）；slug 改了之后不再显示这个错误，而是新 slug 的可用性；
  - 422 的字段错误放在各自字段下方；
  - 403 `workspace.creation_disabled`、`identity.account_deactivated` 显示在表单上方。
- 成功之后调用 `onCreated(workspace)`。

**创建页** `/create-workspace`：
- 读实例信息。创建打开时显示表单，成功之后转到 `/:slug`（W1"进入工作区"）。
- 关闭时不显示表单，说明工作区由服务器管理员创建，或者等成员邀请（W2"页面没有入口"；落点的"等待邀请"也是这一页）。
- 有工作区的人也可以来这一页（切换器的入口）；回到 `/` 用顶栏的 Nerve Wiki，页面上不另放链接。

### 3.6 工作区设置：常规页

- 布局与账户设置相同：导航（常规；P6 加成员）与内容。
- **名称**：
  - 管理员看到改名表单：本地检查同创建，422 显示在字段下方。成功之后框里是去掉首尾空白的名称，显示"已保存"，左栏随之更新。
  - 非管理员只读，说明只有管理员能改。
- **slug**：只读显示，说明创建之后不能改。
- **删除**（只有管理员）：
  - 区块写明后果：所有成员都看不到它；M3 起它的笔记本一并删除，包括成员的私密笔记本（M2 总设计第 4 节）。
  - `ConfirmDialog` 新增可选的 `typedConfirmation`：打开时焦点在输入框；输入 slug 一致之前"删除"按钮不可用，之后在输入框里按 Enter 也确认；对话框关闭时清空输入。
  - 204 之后 store 从列表删除它，并记下这个 slug。外壳找不到它、而它是这个标签页刚删掉的，就 `<Navigate replace to="/">`：落点选出下一个，或者到创建页。页面不自己跳转：先从列表删除、页面再跳转的话，外壳会在跳转之前先显示 404（审查 T2）。
  - 答 404：已被删除或已被移出，与 204 相同（审查 Q1）。答 403（被降级）：对话框保持打开，显示原因。
  - 删除之后的焦点（触发按钮已不在）留到 M3 加页面树时与左栏的语义一起设计（审查 T11）。

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
| `PreferencesStore` 的最后访问：读写、存储不可用或写不进时用内存、别处写入的也读到 | 3.3 |
| slug：本地检查的表格；由名称生成（大小写、空白与符号、中文、截断） | 3.5 |
| `WorkspaceStore`：读不覆盖之后答复的写；读取已含创建的新项时不重复；创建、改名按名称排序，同名按 id；删除，答 404 也算删除；新一代从空开始 | 3.4 |
| 路由：`/` 转到最后访问的、第一个、创建页；`/:slug` 不是成员时是 404，之后的读取里没有了也是；加载失败与重试；保留名单（已有的测试） | 3.2、3.3 |
| 外壳：切换器列出全部、标出当前、转到别的、带"切换工作区"的说明；创建关闭时没有入口；访问之后记下 slug；导航标出当前；页面随工作区重新挂载 | 3.2 |
| 创建：本地检查、slug 随名称直到被改、可用性三种答复、409 在 slug 下方、成功之后进入新工作区（按名称不是第一个）；创建关闭时只有说明；实例信息加载失败与重试 | 3.5 |
| 常规页：管理员改名、422、已保存、去空白与左栏更新；输入 slug 之前不能删除；打开时焦点在输入框，Enter 确认；删除（204、404）之后转到落点，其间不出现 404；403 时对话框保持；非管理员只读 | 3.6 |
| 引导的一步：已有工作区时自动记录一次（创建打开与关闭）；创建之后记录；关闭时说明与继续；列表加载失败与重试；记录失败与重试 | 3.7 |
| 用户菜单显示版本 | 3.8 |

### 3.10 端到端

**夹具**：
- `workspace-pages.ts`：
  - `createWorkspaceWith(page, {name, slug?})`：答复的状态码；
  - `switchWorkspace(page, name)`、`renameWorkspaceWith(page, name)`、`deleteWorkspaceWith(page, slug)`；
  - `expectCreatePage(page)`：没有工作区的账户落在 `/create-workspace`；
  - `workspaceHeading(page, name)`。
- `auth.ts` 的 `onboardingSteps` 加 `workspace`：`registerOnboarded` 的账户没有工作区，落在创建页。
- `onboarding-pages.ts` 的 `stepRecorded(page, id)`：等记录这一步的答复。按答复的 `onboarding_steps` 匹配：页面经 `fetch` 发出的 POST，在 Playwright 里 `postData()` 为 `null`。

**故事**：

| 故事 | 页面版本 | 接口版本 |
|---|---|---|
| W1 创建工作区 | 引导完成的账户落在创建页；slug 随名称生成；输入保留名、已占用的 slug，字段下方分别显示；提交被占用的 slug 答 409，字段下方显示；成功之后进入 `/:slug`，标题是名称。落库断言同接口版本 | 已有 |
| W2 创建开关 | 关闭的服务上：没有工作区的账户落在创建页，只有说明；已有工作区（经命令行创建）的切换器没有"创建工作区" | 已有（接口、命令行） |
| W3 外壳与切换 | 三个工作区（经接口创建）：<br>• `/` 落到按名称的第一个；<br>• 切换到第三个之后，`/` 落到它；<br>• 它被删除之后，落到第一个；<br>• 不是成员的 slug 显示 404；<br>• 全部离开或删除之后，落在创建页；<br>• 外壳里 `main` 的内边距为 0，创建页不为 0 | 新写：列表按名称；不是成员的 slug 答 404；离开之后不在列表里 |
| W4 改名与删除 | 两个测试。管理员改名，左栏随之更新（落库同接口版本）。删除：输入 slug 之前按钮不可用；删除之后落到另一个工作区，所有成员都看不到它，slug 可以再用。成员看到只读的名称，没有删除 | 已有 |
| W11 引导的工作区一步 | 新账户：第 2 步创建工作区，引导结束之后进入它。经邀请加入的账户：第 2 步自动继续，进入那个工作区。关闭的服务上没有工作区：看到说明，继续之后落在创建页的说明 | 新写：PAT 记录 `workspace` 并创建工作区，落库相同 |

**随之调整**：
- S2：登录之后落在创建页；用户菜单显示版本。
- A3：登录之后的落点由"首页标题"改为 `expectCreatePage`。A4、A6 的账户没有完成引导，落在引导的第二步。
- A9：步数 "Step 1 of 2"；第 2 步创建一个 slug 不是 `acme` 的工作区之后，回到 `/acme?view=list`（404 页）；引导完成之后访问 `/onboarding`，落到刚创建的工作区。

## 4. 实施步骤

分支 `m2-p5-web-shell-workspaces`，每个 Step 结束时 `make check` 为绿。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 外壳：<br>• 工作区的 service、store 与 `useWorkspaces`；<br>• 最后访问与落点；<br>• 路由：`/`、`/:slug`、工作区首页、404；创建页的骨架（切换器的入口与落点要它）；<br>• 左栏与切换器；`main` 的边距；<br>• 首页删除、版本移到用户菜单；<br>• 保留名单 | [P5-S1](plans/P5-S1-shell.md) |
| S2 | 创建工作区：slug 的本地检查与生成、`CreateWorkspaceForm`、可用性、创建页的表单与关闭时的说明 | [P5-S2](plans/P5-S2-create.md) |
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

分支 `m2-p5-web-shell-workspaces`：S1 `17b6ea1`、S2 `e0107c2`、S3 `bd01477`、S4 `d4c4e7e`、S5 `748c2a6`，审查修复 `13123b8`，合并 `0c20db9`。第 5 节全部通过，反向对照按预期失败（作者在各 Step 单元 27 项、e2e 4 项，审查修复另单元 21 项、e2e 2 项；审查者单元 53 项、e2e 13 项）；`make check`（vitest 560 个）、`make gen-check`、`make e2e`（72 个；新写与改过的故事另跑 `--repeat-each 3`）、`make image-smoke` 本地与持续集成为绿（本地的 e2e 与 image-smoke 在合并提交上跑；工作区里有未跟踪的 `.claude/`，image-smoke 报 `modified=true`）。S1–S4 的提交上 `make check` 为绿，e2e 到 S5 才随首页与引导改好（审查 Q6）。审查见 [P5 审查记录](reviews/P5-web-shell-workspaces-review.md)：1 项 Major（T1）、7 项 Minor 与 6 项 Nit 已处理，T11 的删除之后的焦点与左栏的 `aside` 留到 M3；7 个疑问中 Q1 照建议改，Q3、Q5 改文档或加断言，其余不改。规模（新增行数）：生产代码约 1,120 行（含文案约 150 行），测试约 1,050 行，端到端约 490 行。

与设计的出入（已同步进上文）：

1. service 的改名方法叫 `rename(slug, name)`，与 store 一致（3.4）。
2. 新增 `app/not-loaded.tsx` 与 `components/nav-item.tsx`（3.1）：加载中或失败与重试有四处、导航的链接有三处，各自共用一份；M1 设置页的样式不变。
3. 创建页没有回到 `/` 的链接：顶栏的 Nerve Wiki 就是（3.5）。
4. `create-workspace` 移进 `[app]` 与创建页的骨架在 S1（切换器的入口与落点要这一页），表单在 S2（第 4 节）。
5. 名称的本地检查用自己的文案键 `field.workspace_name.too_long`（3.5）。
6. slug 改了之后不再显示提交时的 409，而是新 slug 的可用性（3.5）。
7. slug 的生成另做 NFKD 去附加符号，`-` 也折叠，先截断再去末尾的 `-`（3.5，审查 D2）。
8. 切换器是单选的菜单项，不是链接（3.2，审查 Q3）。
9. 删除之后由外壳转到 `/`，页面不自己跳转；答 404 也算删除（3.6，审查 T2、Q1）。
10. 测试替身 `byRoute` 支持 `/prefix/*`，可用性检查的每个 slug 都记下；W4 的页面版本拆成管理员与成员两个测试；A4、A6 落在引导的第二步；e2e 加 `stepRecorded`（3.10）。

审查之后的修复（详见审查记录）：T1 工作区的页面随工作区重新挂载；T2 删除之后不闪现 404；T3 创建与读取交错时不重复（`ApiTokenStore` 一并改）；T4–T8、T10 补上没有守住的测试，W3 断言外壳的内边距；T9 排序的注释与同名按 id 的测试；T11 输入确认的对话框聚焦输入框、Enter 确认，切换器的说明；T12 文案；T13 改名之后去空白；T14 写不进存储时读内存；README 的前端一节（D6）。

留给之后的：

- **M3（审查 T11）**：删除工作区之后的焦点（触发按钮已不在），以及左栏用 `aside` 承载主导航的语义，与页面树一起设计。
- **M3 的 `/:slug` 下的页面**：外壳按工作区 id 重新挂载子页（3.2），页面不必自己处理换工作区；删除、离开之类让当前工作区消失的操作，照 3.6 由 store 记下、外壳转走，页面不自己跳转。
