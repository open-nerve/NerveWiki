# M2/P6 前端成员与邀请：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M2/P6 前端成员与邀请 |
| 状态 | 进行中 |
| 基线 | `0de5022`（P5 合并、P5 文档更新之后的 main） |
| 上级文档 | [M2 总设计](00-M2-design.md) 第 1 节第 2、3、6 条、第 3、4、5、7 节；[P5 文档](05-P5-web-shell-workspaces.md)（外壳、store 的写法、删除的流程）；[总体设计](../v0.1-design.md) 13.2 |

---

## 1. 基线

**P1–P5 之后**：
- 接口：成员与邀请的 9 个接口都已就绪（M2 总设计第 5 节）：
  - 成员：`listWorkspaceMembers`、`updateWorkspaceMember`、`removeWorkspaceMember`、`leaveWorkspace`；
  - 邀请：`listWorkspaceInvitations`、`createWorkspaceInvitation`、`deleteWorkspaceInvitation`、`previewWorkspaceInvitation`（公开）、`acceptWorkspaceInvitation`；
  - `register` 的请求体可以带 `invitation {id, token}`。
- 前端：
  - 外壳 `/:slug` 与工作区设置的常规页；
  - 子页随工作区重新挂载；
  - 删除工作区时，store 记下这个 slug，外壳转到 `/`。
- 文案：成员与邀请的 5 个 problem 码已有文案（契约核对要求）。`workspace.sole_admin` 的文案说的是离开。
- 保留名单：`invitations` 还在 `[reserved]`。
- e2e：W5–W10 只有接口版本（W10 另有命令行版本）。

## 2. 目标与范围

**目标**：
- 工作区设置里有"成员"页：所有成员看到成员列表；管理员改角色、移出、邀请、复制邀请链接、撤回邀请；每个成员都能离开。
- 拿到链接的人打开公开的邀请页：先看到预览，未登录时在页内登录或注册，然后接受，进入工作区。
- 停用账户被唯一管理员规则拒绝时，对话框说明原因。
- W5–W10 的页面版本通过。

**做**：
- `/:slug/settings/members`：
  - 成员列表；
  - 改角色、移出（管理员，不对自己）；
  - 邀请：创建、列表、复制链接、撤回（管理员）；
  - 离开（每个成员）。
- `/invitations/:id`：
  - 公开的邀请页：预览；页内登录、注册（带邀请）；接受；邮箱不符时说明并可退出；
  - `invitations` 移进保留名单的 `[app]`。
- 停用对话框对 `workspace.sole_admin` 的说明。
- e2e：W5–W10 的页面版本。

**不做**：

| 事项 | 理由 |
|---|---|
| 停用对话框列出是哪些工作区 | problem 只有英文的 `detail` 列出 slug（给命令行看）。页面要列出，就得在 problem 里加结构化的成员，改平台的 `Problem`、`shared.Error` 与契约的 `Problem`（`additionalProperties: false`）。这只为一个对话框。页面说明原因与做法，账户在切换器里看得到自己的工作区 |
| 邀请页"忽略邀请" | M2 总设计第 2 节：不理会即可 |
| 已是成员时，邀请页不接受而直接进入 | 接受对已是成员的账户只消费邀请，角色不变（M2 总设计第 4 节），结果就是进入工作区。页面不先判断，免得留下一份永远待接受的邀请 |
| 成员列表的搜索、分页 | v0.1 的工作区是小团队；接口一次答全部有效成员 |
| 角色说明的弹层 | 角色的含义在 M3 有了笔记本之后才完整，那时一起写 |

## 3. 设计

### 3.1 文件

```
server/internal/modules/workspace/domain/reserved_slugs.txt   invitations 从 [reserved] 移到 [app]
web/apps/web/src/
  app/routes.tsx                       /invitations/:id（守卫之外）；/:slug/settings/members
  app/confirm-dialog.tsx               可选的 messages：按 problem 码换这个对话框的说法
  app/invitation-link.ts               邀请链接的拼出与解析：纯函数
  components/ui/native-select.tsx      原生 select 的样式（角色）
  services/member.service.ts           list、update、remove
  services/invitation.service.ts       InvitationService：list、create、remove；InvitationPreviewService：preview（公开客户端）
  services/workspace.service.ts        加 leave、accept
  services/auth.service.ts             register 可带邀请
  stores/member.store.ts               一个工作区的成员列表
  stores/invitation.store.ts           一个工作区的待接受邀请；InvitationPreviewStore
  stores/workspace.store.ts            加 leave、accept
  stores/auth.store.ts                 signUp 可带邀请
  stores/root.store.ts、context.tsx    membersOf、invitationsOf（按工作区缓存）；invitationPreviews
  pages/workspace/settings-layout.tsx  导航加"成员"
  pages/workspace/members-page.tsx     成员页：成员、邀请、离开三节
  pages/workspace/member-row.tsx       一行成员：角色、移出
  pages/workspace/invitations-section.tsx  邀请：表单、列表、复制、撤回
  pages/invitation.tsx                 公开的邀请页
  pages/settings/deactivate-dialog.tsx sole_admin 的说法
  i18n/messages/{en,zh-CN}.ts
e2e/
  fixtures/member-pages.ts             成员页的操作
  fixtures/invitation-pages.ts         邀请页的操作
  stories/workspace/w5–w10             页面版本
```

### 3.2 数据：service 与 store

**services**（13.2 第 2、6 条；每个 service 一个客户端，类型由它转出）：

| service | 客户端 | 方法 |
|---|---|---|
| `MemberService` | 带令牌 | `list(slug)`、`update(id, role)`、`remove(id)` |
| `InvitationService` | 带令牌 | `list(slug)`、`create(slug, {email, role})`、`remove(id)` |
| `InvitationPreviewService` | 公开 | `preview({id, token})` |
| `WorkspaceService` | 带令牌 | 加 `leave(slug)`、`accept({id, token})` |
| `AuthService` | 公开 | `register(email, password, invitation?)` |

`leave` 与 `accept` 放在 `WorkspaceService`：两者改变的是账户的工作区列表。

**stores**：

- **`MemberStore`、`InvitationStore`：一个工作区的一份列表。**
  - 写法同 `ApiTokenStore`：`list`、`load()`（读出去之后已有写答复的丢弃）、写把答复放进列表。
  - 成员：`changeRole(id, role)` 换掉那一项；`remove(id)` 移出那一项，答 404 `workspace.member_not_found` 也移出（已不在了，结果就是要的）。
  - 邀请：`invite(body)` 放到最前（服务端新的在前；先按 id 去掉已有的，同 P5 审查 T3）；`withdraw(id)` 移出，答 404 `workspace.invitation_not_found` 也移出（已被接受或撤回）。
- **按工作区缓存**：`RootStore.membersOf(workspace)`、`invitationsOf(workspace)` 每代按工作区 id 各缓存一个，store 带着 slug 去请求。
  - 按 id 而不是 slug：同一代里删掉一个工作区、再用同一个 slug 建一个，不会看到前一个的成员。
  - SWR 的键是 `["members", id]`、`["invitations", id]`。
  - 放弃的写法：一个 store 用 `Map<slug, 列表>` 装全部工作区。那样每个写方法都要带 slug，每份列表也要各自的写计数，等于把单列表的 store 在 map 里再写一遍。
- **`WorkspaceStore`**：
  - `leave(slug)` 同 `remove`：移出列表，记下这个 slug（外壳转到 `/`）；答 404 `workspace.not_found` 也一样（已不是成员）。
  - `accept(link)` 把答复的工作区放进列表（按 id 去重，按名称排序），并返回它。列表还没读过时，留给第一次读取（同 `create`）。
- **`InvitationPreviewStore`**（每代都有，未登录时也有）：`preview(link)` 只转调公开的 service，不存状态（同 `checkSlug`）。页面用 SWR 的键 `["invitation-preview", id, token]` 缓存答复：链接改了令牌就是另一次预览。
- **`AuthStore.signUp(email, password, invitation?)`**：注册带上邀请。

### 3.3 成员页 `/:slug/settings/members`

工作区设置的导航加"成员"，所有成员都看得到这一页。页面分三节：

**成员**：
- 读 `membersOf(workspace)` 的列表，按加入时间（服务端的顺序）。每行：显示名（自己的一行标"你"）、邮箱、角色、加入日期（`formatDate`）。访客看到的邮箱是 `null`，这时整列不显示。
- **管理员**：别人的一行有角色的下拉框与"移出"。
  - 角色用原生 `<select>`，选了就发 `PATCH`，发送中禁用。原生控件的键盘与读屏都是现成的，e2e 用 `selectOption`。
  - 移出经 `ConfirmDialog`："把 Bob 移出 Acme？"，说明他的待接受邀请一并删除。
  - 自己的一行没有这两个控件：接口答 409 `workspace.own_membership`，页面不提供。
- **写的失败**：原因显示在列表上方（`role=alert`）。答 403 `forbidden`（在别处被降级）时，页面重新读取工作区列表，控件随之跟着新角色变化。移出的失败留在对话框里（`ConfirmDialog` 的写法）。

**邀请**（只有管理员看得到，也只有管理员去读）：
- **表单**：邮箱与角色（默认成员），经 `useForm`。
  - 本地检查：必填。
  - 422 的字段错误在邮箱下方：`not_allowed` 是"已是这个工作区的成员"，`duplicate` 是"已有一份待接受的邀请，在下面复制它的链接"。
  - 成功之后清空邮箱，`<output>` 说"已邀请 bob@…：复制链接发给他"。
- **列表**：邮箱、角色、邀请日期、"复制链接""撤回"。
  - 复制：`navigator.clipboard.writeText(链接)`，成功之后 `<output>` 说"已复制给 bob@… 的链接"。
  - 没有剪贴板（非安全上下文，如局域网地址的 HTTP），或写入被拒绝时，这一行下方显示只读的链接框，选中其中的文字，请用户自己复制。
  - 撤回经 `ConfirmDialog`："撤回给 bob@… 的邀请？链接随即失效"。
- **链接**：`app/invitation-link.ts` 的 `invitationLink(origin, {id, token})` 拼出 `${origin}/invitations/${id}#${token}`。令牌只在片段里（M2 总设计第 4 节）。
- 邀请链接不是只显示一次的秘密：13.2 第 13 条已说明，列表持有令牌。

**离开**（每个成员）：
- 区块说明后果："你将看不到这个工作区；之后要再加入，需要新的邀请。"
- 经 `ConfirmDialog` 调用 `workspaces.leave(slug)`。成功之后外壳转到 `/`，与删除相同（P5 3.6）。
- 唯一的管理员答 409 `workspace.sole_admin`：对话框保持打开，说明原因（文案说的正是离开）。

### 3.4 公开的邀请页 `/invitations/:id`

**路由**：挂在布局之下、`GuestOnly` 与 `SignedIn` 之外（13.2 第 10 条的例外）。`invitations` 从 `[reserved]` 移进 `[app]`，vitest 核对。

**链接**：
- `linkOf(id, hash)` 从地址的片段取令牌。
- 片段为空时不发预览，直接显示"链接不完整"（`workspace.invitation_not_found` 的文案已包括这种情形）。

**页面**按会话与预览依次决定显示：

| 情况 | 显示 |
|---|---|
| 会话启动中；会话不可用 | 加载中；`SessionUnavailable` 与重试（同守卫） |
| 预览加载中、失败 | `NotLoaded`；答 404 时显示链接已失效的说明，已登录时另有回到 `/` 的链接 |
| 未登录 | 预览：`{name}` 邀请你以{角色}加入。下面是"登录"与"注册"两个表单，两个按钮切换，默认登录。注册带上邀请：注册关闭时，只有被邀请的邮箱能注册，别的邮箱答 403 `identity.signup_disabled`，显示在表单上方 |
| 已登录 | 预览，"以 {显示名}（{邮箱}）登录"，"接受邀请"与"退出"两个按钮 |
| 接受成功 | 转到 `/:slug`（`replace`：后退不回到用过的链接）。账户还没完成引导时，`Onboarded` 先把它带进引导，工作区一步因为已有工作区而自动继续 |
| 接受答 403 `workspace.invitation_email_mismatch` | 说明这份邀请发给了别的邮箱；"退出"之后用那个邮箱登录 |
| 接受答 404 | 链接已失效（被撤回、已被接受、工作区已删除） |

**会话的变化**（13.2 第 10 条）：
- 页内登录、注册、退出都只改变会话，页面不跳转。新的一代挂载时，这一页还在同一个带片段的地址上，于是从"未登录"变成"已登录"，或者反过来。
- 令牌从不进入 `next`：这一页不在守卫之下，没有 `next`。
- 接受不是会话的变化，之后由页面跳转。
- 页面在 `SignedIn` 之外，所以它自己读账户（SWR 的键 `me`），用来显示登录的是谁。

**请求**：预览与接受都把令牌放在请求体里。W6 的页面版本核对页面发出的请求，地址里都没有令牌。

**已是成员**：照样接受（3 节"不做"），答复的工作区带他原来的角色，页面进入工作区。

### 3.5 停用对话框

- `ConfirmDialog` 加可选的 `messages`：problem 码 → 这个对话框的文案键，没有列出的码照常经 `errorText`。
- `DeactivateDialog` 把 `workspace.sole_admin` 换成停用的说法："你是某个还有其他成员的工作区唯一的管理员。先在那里（工作区设置 → 成员）让另一位成员成为管理员，再停用。"
- 不列出工作区（第 2 节"不做"）。

### 3.6 前端的测试（vitest）

| 测试 | 守住 |
|---|---|
| `invitationLink`、`linkOf`：拼出与解析，空片段，片段里的编码 | 3.3、3.4 |
| `MemberStore`、`InvitationStore`：读不覆盖之后答复的写；改角色换掉那一项；移出、撤回，答 404 也移出；邀请放到最前、不重复 | 3.2 |
| `WorkspaceStore`：离开同删除（含 404）；接受加入列表，不重复，列表未读时留给读取 | 3.2 |
| `RootStore`：同一工作区同一个 store；别的工作区、新的一代是新的 | 3.2 |
| 成员页：列表与"你"；访客看不到邮箱列；管理员改别人的角色，自己的一行没有控件；403 之后重新读取工作区；移出（含 404）；成员与访客看不到邀请一节，也不去读 | 3.3 |
| 邀请：本地检查；422 的两种在邮箱下方；创建之后在最前；复制写进剪贴板；没有剪贴板时显示链接框；撤回（含 404） | 3.3 |
| 离开：成功之后落到另一个工作区，其间不出现 404；唯一的管理员留在对话框里，说明原因 | 3.3 |
| 邀请页：链接不完整；预览 404；未登录时登录、注册（请求体带邀请）；已登录接受并进入；邮箱不符的说明与退出；已是成员时进入，角色不变；接受之后未完成引导的账户进入引导 | 3.4 |
| 停用对话框：`workspace.sole_admin` 是停用的说法 | 3.5 |
| 保留名单：`invitations` 在 `[app]`（已有的测试） | 3.4 |

### 3.7 端到端

**夹具**：
- `member-pages.ts`：`membersTable(page)`（每行的名称、邮箱、角色）、`changeRoleWith`、`removeMemberWith`、`leaveWith`。
- `invitation-pages.ts`：`inviteWith`（答复的状态码与邀请）、`copiedLink`（读剪贴板）、`withdrawWith`、`invitationPage(page, link)`（打开链接）、`acceptWith`。
- 剪贴板：W5 的上下文授予 `clipboard-read`、`clipboard-write`（Chromium）。

**故事**（页面版本，落库断言同接口版本，经 `fixtures/assert/workspace.ts`）：

| 故事 | 页面版本 |
|---|---|
| W5 邀请 | 管理员在成员页邀请一个邮箱，复制的链接就是列表里那份邀请的；邀请已是成员的邮箱、重复邀请，邮箱下方说明；撤回一份之后它的链接答 404。成员看不到邀请一节 |
| W6 接受邀请 | 已有账户的被邀请人：未登录打开链接，看到预览，页内登录，接受，进入工作区。别的账户登录时看到邮箱不符的说明，退出之后页面回到登录。页面发出的请求，地址里都没有令牌 |
| W7 带邀请注册 | 注册关闭的服务上，在邀请页用被邀请的邮箱注册，接受，经引导进入工作区；用别的邮箱注册看到注册已关闭 |
| W8 成员与角色 | 成员页列出成员；访客看不到邮箱；管理员把成员改成访客，落库；自己的一行没有角色控件 |
| W9 移出与离开 | 管理员移出成员，他的待接受邀请一并删除；成员离开，落到 `/`；唯一的管理员离开被拒，对话框说明原因 |
| W10 停用与恢复 | 唯一的管理员在安全页停用，对话框说明原因，会话仍在；另一位成员成为管理员之后停用成功，回到登录页 |

## 4. 实施步骤

分支 `m2-p6-web-members-invitations`，每个 Step 结束时 `make check` 为绿。每个 Step 是一段完整的功能，所以 knip 不会遇到没有用到的导出。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 成员页的成员与离开：`MemberService`、`MemberStore`、`membersOf`；`WorkspaceService.leave`、`WorkspaceStore.leave`；设置导航的"成员"；成员列表、改角色、移出、离开；原生 select | [P6-S1](plans/P6-S1-members.md) |
| S2 | 邀请：`InvitationService`、`InvitationStore`、`invitationsOf`；邀请链接；成员页的邀请一节（表单、列表、复制、撤回） | [P6-S2](plans/P6-S2-invitations.md) |
| S3 | 公开的邀请页：`InvitationPreviewService`、`InvitationPreviewStore`；`WorkspaceStore.accept`；带邀请的注册；路由与保留名单 | [P6-S3](plans/P6-S3-invitation-page.md) |
| S4 | 停用对话框：`ConfirmDialog` 的 `messages`；sole_admin 的说法 | [P6-S4](plans/P6-S4-deactivate.md) |
| S5 | 端到端：W5–W10 的页面版本与夹具 | [P6-S5](plans/P6-S5-e2e.md) |

规模估计：生产代码约 1,000 行，测试约 1,100 行，端到端约 550 行。

## 5. 测试与验证

| 层次 | 覆盖 |
|---|---|
| 单元（vitest） | 3.6 |
| 端到端 | 3.7；本 M 与之前各 M 的全部故事照旧通过 |

**反向对照**（13.4 第 1 条）：

| 改动 | 应当失败的测试 |
|---|---|
| `MemberStore.load` 不管之后答复的写 | store 的交错测试 |
| 管理员自己的一行也有角色控件 | 成员页测试、W8 页面版本 |
| 访客照样显示邮箱列 | 成员页测试、W8 页面版本 |
| 非管理员也去读邀请 | 成员页测试（请求记录） |
| 离开之后不记下 slug（外壳显示 404） | 离开测试、W9 页面版本 |
| 邀请链接把令牌放进查询参数 | `invitationLink` 测试、W5、W6 页面版本 |
| 注册不带邀请 | 邀请页测试、W7 页面版本 |
| 邀请页挂在 `SignedIn` 之下 | 邀请页测试（未登录）、W6 页面版本 |
| 接受之后不把工作区放进列表 | `WorkspaceStore` 测试；外壳在下一次读取之前显示 404 |
| 停用对话框不换 sole_admin 的说法 | 停用测试、W10 页面版本 |
| `[app]` 漏掉 `invitations` | 保留名单测试 |

## 6. 完成标准

- 第 2 节"做"的各项完成；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 本地与持续集成为绿。
- 审查记录 `reviews/P6-web-members-invitations-review.md`；本文第 7 节、M2 总设计的进度表已更新。
- 之后是 M2 收尾审查（M2 总设计第 11 节）。

## 7. 结果

（完成后补写）
