# M1/P6 前端个人设置：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M1/P6 前端个人设置 |
| 状态 | 进行中 |
| 基线 | P5 合并之后的 main（`a991330`） |
| 上级文档 | [M1 总设计](00-M1-design.md) 第 3、4、7 节；[P5 文档](05-P5-web-session.md) |

---

## 1. 基线

P5 之后，浏览器里能注册、登录、退出、完成引导：`src/session/` 是唯一创建客户端的模块，每次登录一代 `RootStore`，守卫决定去处，表单有 `FormField`、`formErrors`、提交失败之后的焦点，顶栏的用户菜单只有显示名与退出。服务端（P3）已有设置页要用的全部接口：`updateMe`、`changePassword`（204，其他会话结束、当前会话保留，PAT 照常，按账户限流）、`deactivateMe`（204，所有会话结束、PAT 停用，只有管理员能重新启用）、`listApiTokens`（不分页，未撤销的，含已过期的，新的在前）、`createApiToken`（要求当前密码，答复里有令牌本身，仅此一次）、`revokeApiToken`（204；不存在、已撤销、别人的都是 404 `identity.api_token_not_found`）。主题与语言属于设备（`nwiki.theme`、`nwiki.locale`，M0/P5），顶栏已能切换。

Nerve 的设置页（Plane 的分支）用 react-hook-form、headlessui、date-fns 与服务端保存的偏好，本项目都没有：页面的逻辑、错误的映射与端到端故事有参考价值，组件不拷贝（调研见本文第 7 节的记录）。

## 2. 目标与范围

**目标**：登录之后在设置页改显示名、切换主题与语言、改密码、停用账户、管理 PAT；本 M 的全部故事（页面、接口、命令行）通过，M1 可以收尾。

**做**：
- 路由 `/settings/{profile,security,tokens}`（M1 总设计第 4 节），`/settings` 转到 `/settings/profile`；设置的布局（导航与内容）；用户菜单加"设置"。
- 资料：显示名；偏好：主题、语言（设备上保存，即时生效）；邮箱只读，说明由服务器管理员修改。
- 安全：改密码；停用账户（确认对话框）。
- 访问令牌：列表（名称、创建、到期、最后使用）；创建（名称、有效期、当前密码），令牌只显示一次；撤销（确认对话框）。
- UI：`Dialog`、`AlertDialog`（radix-ui 已有，不加依赖）；日期按界面语言格式化。
- 端到端：A7、A8、A10、A11 的页面版本；`settings-pages.ts`。

**不做**：头像、时区、每周第一天（v0.1 没有）；令牌的描述字段与自定义日期（接口只有名称与到期时间，预设的有效期够用）；把令牌下载成文件（Nerve 自动下载 CSV，秘密因此留在磁盘上）；全局的 toast（结果显示在所在的区块里）；会话列表（v0.1 没有）。

## 3. 设计

### 3.1 文件

```
web/apps/web/src/
  app/routes.tsx                            /settings 与三个子页，挂在 Onboarded 之下
  app/user-menu.tsx                         加"设置"
  pages/settings/
    settings-layout.tsx                     导航（NavLink）与内容；窄屏时导航在上方
    profile-page.tsx                        显示名的表单；偏好（主题、语言）
    security-page.tsx                       改密码的表单；停用的区块
    deactivate-dialog.tsx                   停用的确认
    tokens-page.tsx                         列表、空状态、创建与撤销的入口
    create-token-dialog.tsx                 创建的表单与只显示一次的令牌
    token-row.tsx                           一个令牌：名称、日期、撤销的确认
  services/account.service.ts               另有 changePassword、deactivate
  services/api-token.service.ts             list、create、revoke
  stores/account.store.ts                   另有 changePassword、deactivate
  stores/api-token.store.ts                 本次登录的令牌列表
  stores/root.store.ts                      一代另有 apiTokens（登录时）
  stores/auth.store.ts                      另有 endSession：服务端已经结束的会话，在本浏览器忘掉它
  app/problem-messages.ts                   formErrors 可以把 problem 码放到字段下方
  i18n/format.ts                            日期按界面语言格式化
  components/ui/dialog.tsx、alert-dialog.tsx
  i18n/messages/{en,zh-CN}.ts
e2e/fixtures/settings-pages.ts；stories/identity/a7、a8、a10、a11（页面版本）
```

### 3.2 路由与布局

```
SignedIn
└ Onboarded
  ├ /settings → /settings/profile（replace）
  └ SettingsLayout          导航：资料、安全、访问令牌（NavLink，当前页 aria-current）
    ├ /settings/profile
    ├ /settings/security
    └ /settings/tokens
```

- 设置页只有路由一个入口（Nerve 另有同样内容的对话框，两处呈现同一内容）：用户菜单的"设置"就是到 `/settings/profile` 的链接（radix 的 `DropdownMenuItem asChild` 包一个 `Link`）。
- 三个子页各自按需加载，布局与导航随第一个子页进入同一个 chunk。
- 页面宽度与首页一致；宽屏时导航在左侧，窄屏时在上方横排。

### 3.3 资料与偏好

- **显示名**：预填当前的，本地只查非空；保存调 `account.update({ display_name })`（经 `oneAtATime`，P5 审查 M1 的读写保护同样适用：保存之后 SWR 的重新读取不会让名字回弹）。422 显示在字段下方；成功在按钮旁显示"已保存"（`role=status`），顶栏的用户菜单随 `me` 更新。没改就不发请求，直接显示"已保存"。
- **邮箱**：只读显示，说明"邮箱由服务器管理员修改"（改邮箱是 `users set-email`，M1/P4）。
- **偏好**：主题（跟随系统、浅色、深色）与语言（English、简体中文）用单选组，选中即调 `preferences.setTheme`、`setLocale`，与顶栏的菜单是同一个 store，互相同步；保存在设备上，刷新之后保持，不经服务端（M1 总设计第 2 节）。

### 3.4 安全：改密码

- 字段：当前密码、新密码（都可切换明文；不要确认框，与注册一致）。本地检查：两者非空，新密码 8–128 个字符（UTF-16 长度，与注册共用常量）。常见密码与"由邮箱构成"只有服务端知道。
- **problem 码放到字段下方**：`formErrors(error, t, shown, onField?)` 增加可选的 `onField: { [code]: field }`。改密码传 `{ "identity.current_password_incorrect": "current_password" }`，创建令牌同样；字段下方显示它的文案，上方不再重复。其余照旧：422 的字段错误在字段下方，429（按账户的限流，带秒数）、503 在上方。
- **成功**：清空两个字段，在表单下方显示"密码已修改。其他会话已退出；这个会话与访问令牌照常可用"（`role=status`）。页面不退出、不刷新：服务端保留当前会话（同一浏览器的其他标签页共用它，照常工作）；其他浏览器的会话在下一次续期时答 401，由它们的令牌管理器回到登录页。
- 写请求经 `AccountStore.changePassword`，不改 `me`。

### 3.5 安全：停用

- 区块说明后果：所有会话退出、访问令牌停止工作、只有服务器管理员能重新启用；按钮"停用账户"打开 `AlertDialog`。
- **确认**：对话框重复后果，"取消"与"停用"（危险样式）。不要求密码，也不要求键入确认词（接口不要求密码，M1 总设计第 4 节；Nerve 同样只有确认）。发送中按钮禁用并显示"停用中"，双击只发一次。
- **成功**：`account.deactivate()` 答 204 之后 `auth.endSession()`：服务端已经结束了会话，本浏览器删除记录、状态变为 `signed-out`，所有标签页经 storage 事件跟上；守卫把页面带到 `/sign-in?next=/settings/security`。不调 `logout`（会话已经结束，多一个请求没有意义）。登录页不另显示"已停用"：对话框已经说明，此后登录答 403 时有文案。
- **失败**：对话框保持打开，错误显示在对话框里（`Alert`），会话保留。M2 的否决者答出的码经文案表显示（契约核对覆盖 `deactivateMe`）。
- `AuthStore.endSession()` 调令牌管理器已有的 `endSession(loginId)`（认证中间件在 401 时也用它）。

### 3.6 访问令牌

- **列表**（`ApiTokenStore.load`，SWR 的键 `api-tokens`，缓存按代）：每行名称；"创建于 {日期}"；"{日期} 到期"、"已于 {日期} 过期"（`expires_at <= now`，另显示"已过期"标记）或"永不过期"；"最后使用 {日期时间}"或"从未使用"（服务端精确到分钟）。空列表显示说明与创建按钮。加载失败显示错误与"重试"（SWR 的 `mutate`）。
- **日期**：`i18n/format.ts` 的 `formatDateTime(iso, locale)`、`formatDate`，用 `Intl.DateTimeFormat`（`dateStyle: "medium"`，时间 `timeStyle: "short"`），按界面语言与浏览器时区；不引入日期库。
- **创建**（`Dialog`）：名称（必填，本地只查非空）；有效期：原生 `<select>`，30 天、90 天（默认）、1 年、永不过期；当前密码。提交时按所选天数从现在算出 `expires_at`（`Date.now() + days * 86_400_000`，服务端只要求在将来）。`identity.current_password_incorrect` 显示在密码下方，名称与到期的 422 在各自字段下方。
- **只显示一次**：201 之后对话框换成令牌视图：只读的输入框显示令牌（聚焦即全选），"复制"按钮（`navigator.clipboard` 存在时才显示：局域网 HTTP 这类非安全上下文没有它，用户自己选中复制），警告"现在复制它，关闭之后不会再显示"；只有"完成"关闭（Esc 与点击外部在这个视图不关闭）。令牌只在对话框组件的 state 里：store 把新令牌（不含秘密）加到列表开头，不保存秘密；对话框关闭即卸载，秘密随之消失。
- **迟到的答复**：创建发出之后用户取消了对话框，答复到达时组件已经卸载，令牌不会显示在任何地方；列表照样得到这个令牌（不含秘密），用户可以撤销它。不需要 Nerve 的代数计数与延时清理：它们是因为 Nerve 的对话框关闭之后仍挂载着做淡出。
- **撤销**：每行"撤销"（`aria-label` 带名称）打开 `AlertDialog`："使用它的程序将立即失去访问，不能撤回"。204 之后从列表删除；404 `identity.api_token_not_found`（别的标签页已撤销）同样从列表删除、关闭对话框：结果就是用户要的。其余错误显示在对话框里。
- **store**：`ApiTokenStore { tokens; load(); create(); revoke() }`。写与读的交错按 `AccountStore` 的办法：读出去之后有写答复了，丢弃读的答复（否则刚创建的令牌会从列表消失）。创建与撤销之间不需要排队：各自改列表中不同的项。

### 3.7 前端的测试（vitest）

| 测试 | 守住 |
|---|---|
| 设置的路由：未完成引导去 `/onboarding`；`/settings` 转到资料；导航的当前页 | 3.2 |
| 资料：非空检查、422 在字段下方、已保存、用户菜单随之更新、没改不发请求；偏好与顶栏同步、写入设备 | 3.3 |
| 改密码：本地检查、`current_password_incorrect` 在字段下方、429 的秒数、成功清空并提示 | 3.4 |
| 停用：确认之后 `signed-out` 并到登录页、记录被删；失败时对话框保持、会话保留；双击只发一次 | 3.5 |
| 令牌：列表的日期与状态、空状态；创建的本地检查与字段错误；令牌只在对话框里、完成之后不在页面上；迟到的答复不显示令牌；撤销与 404 | 3.6 |
| `ApiTokenStore`：读不覆盖之后答复的写；创建加在开头、不存秘密 | 3.6 |
| `formErrors` 的 `onField`；契约核对加上 `changePassword`、`deactivateMe`、`listApiTokens`、`createApiToken`、`revokeApiToken` | 3.4 |
| `formatDateTime` 按语言 | 3.6 |

### 3.8 端到端

**夹具** `settings-pages.ts`：`changePasswordWith(page, current, next)`（等接口的答复）、`createTokenWith(page, {name, expiry?, password})`、`holdAnswer(page, method, path)`（`page.route` 扣住答复，返回放行的函数）；`registerOnboarded(api, email)`（P5 的 `completeOnboarding` 之上）。

**故事的页面版本**：

| 故事 | 页面版本 |
|---|---|
| A7 改密码 | 另有一个接口登录的会话与一个 PAT；错误的当前密码在字段下方（422），常见的新密码在字段下方（422），库不变；正确的：204、提示，另一个会话的访问令牌与续期答 401，页面刷新之后仍在登录状态，PAT 照常，旧密码登录答 401 |
| A8 资料与偏好 | 改显示名：库与用户菜单更新，刷新之后保持；过长的显示名 422 在字段下方；设置页切换深色与简体中文，立即生效，刷新之后保持，与顶栏的菜单一致 |
| A10 PAT | 空状态；错误的密码在密码下方（422），什么也没建；创建之后令牌只显示一次（格式 `nwk_pat_`），能复制（授予剪贴板权限，读回核对）；完成之后令牌不在 DOM、存储、地址与控制台中，刷新之后也不在；列表"从未使用"，用这个令牌调一次接口，刷新之后显示"最后使用"；取消之后才到达的创建答复不显示令牌、列表有它；撤销之后令牌答 401 |
| A11 停用 | 另有一个接口登录的会话与一个 PAT；确认停用：204，到登录页，记录被删，另一个会话与 PAT 答 401；登录答 403 的文案；`nervewiki users activate` 之后登录回到设置页，PAT 恢复。停用答 503（`page.route`）时对话框保持、会话保留 |

## 4. 实施步骤

在分支 `m1-p6-settings` 上依次进行，每个 Step 结束时 `make check` 为绿。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 设置的路由与布局、用户菜单的入口；资料与偏好 | [P6-S1-profile.md](plans/P6-S1-profile.md) |
| S2 | 安全：改密码、停用；`Dialog`、`AlertDialog`；`formErrors` 的 `onField`；`AuthStore.endSession` | [P6-S2-security.md](plans/P6-S2-security.md) |
| S3 | 访问令牌：service、store、列表、创建与只显示一次、撤销；日期格式化 | [P6-S3-tokens.md](plans/P6-S3-tokens.md) |
| S4 | 端到端：A7、A8、A10、A11 的页面版本；README；M1 的全部故事 | [P6-S4-e2e.md](plans/P6-S4-e2e.md) |

规模估计：生产代码约 1,100 行，测试约 1,000 行，端到端约 500 行。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元（vitest） | 第 3.7 节 |
| 端到端 | 第 3.8 节；本 M 与 M0 的全部故事照旧通过 |

反向对照（验证后撤销）：
- 创建的令牌存进 store（或对话框关闭之后仍挂载）→ 令牌"只显示一次"的单元测试与 A10 失败；
- 迟到的答复写进对话框的状态而不管是否已卸载（例如状态放到 store）→ 迟到答复的测试失败；
- `ApiTokenStore.load` 不管之后的写 → store 的交错测试失败；
- 停用成功之后不结束本地会话 → 停用的单元测试与 A11 失败；
- `current_password_incorrect` 不放到字段下方 → 改密码与创建令牌的测试失败；
- 设置页的偏好只改页面、不写设备 → A8 刷新之后失败。

## 6. 完成标准

- 第 5 节全部通过，反向对照按预期失败；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 本地与持续集成为绿。
- 审查完成（`reviews/P6-settings-review.md`），发现的问题已修复。
- M1 总设计的进度表更新；M1 第 3 节的故事表全部有对应的版本。

## 7. 结果

（完成后补写）
