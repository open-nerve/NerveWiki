# M2/P6 前端成员与邀请：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M2/P6 前端成员与邀请 |
| 状态 | 已完成 |
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
  app/problem-messages.ts、form.ts     ProblemTexts：errorText、formErrors、useForm 可按 problem 码换说法
  app/confirm-dialog.tsx               可选的 texts：按 problem 码换这个对话框的说法
  app/guards.tsx                       导出 useSession（邀请页用）
  app/invitation-link.ts               邀请链接的拼出与解析：纯函数
  components/ui/native-select.tsx      原生 select 的样式（邀请的角色）
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
  pages/workspace/member-row.tsx       一行成员：角色的菜单、移出
  pages/workspace/role-options.tsx     角色的列表与邀请表单的选项
  pages/workspace/invitations-section.tsx  邀请：表单、列表、复制、撤回
  pages/credentials-form.tsx           登录、注册的本地检查（signInProblems、signUpProblems）与 texts，邀请页共用
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

`leave` 与 `accept` 放在 `WorkspaceService`：两者改变的是账户的工作区列表。邀请链接的类型 `InvitationLink`（`{id, token}`）在 `services/invitation.service.ts`：services 不导入 `app/`。

**stores**：

- **`MemberStore`、`InvitationStore`：一个工作区的一份列表。**
  - 写法同 `ApiTokenStore`：`list`、`load()`（读出去之后已有写答复的丢弃）、写把答复放进列表。
  - 成员：`changeRole(id, role)` 换掉那一项；`remove(id)` 移出那一项，答 404 `workspace.member_not_found` 也移出（已不在了，结果就是要的）。
  - 邀请：`invite(body)` 放到最前（服务端新的在前；先按 id 去掉已有的，同 P5 审查 T3）；`withdraw(id)` 移出，答 404 `workspace.invitation_not_found` 也移出（已被接受或撤回）。
- **按工作区缓存**：`RootStore.membersOf(workspace)`、`invitationsOf(workspace)` 每代按工作区 id 各缓存一个，store 带着 slug 去请求。
  - 按 id 而不是 slug：同一代里删掉一个工作区、再用同一个 slug 建一个，不会看到前一个的成员。
  - SWR 的键是 `["members", id]`、`["invitations", id]`。
  - 两份缓存共用 `once(cache, key, make)`：渲染中取用是幂等的，严格模式渲染两遍也只建一个。
  - 放弃的写法：一个 store 用 `Map<slug, 列表>` 装全部工作区。那样每个写方法都要带 slug，每份列表也要各自的写计数，等于把单列表的 store 在 map 里再写一遍。
- **`WorkspaceStore`**：
  - `leave(slug)` 同 `remove`：移出列表，记下这个 slug（外壳转到 `/`）；答 404 `workspace.not_found` 也一样（已不是成员）。
  - `accept(link)` 把答复的工作区放进列表（按 id 去重，按名称排序），并返回它。列表还没读过时，留给第一次读取（同 `create`）。
- **`InvitationPreviewStore`**（每代都有，未登录时也有）：`preview(link)` 只转调公开的 service，不存状态（同 `checkSlug`）。页面用 SWR 的键 `["invitation-preview", id, token]` 缓存答复：链接改了令牌就是另一次预览。
- **`AuthStore.signUp(email, password, invitation?)`**：注册带上邀请。

### 3.3 成员页 `/:slug/settings/members`

工作区设置的导航加"成员"，所有成员都看得到这一页。页面分三节：

**成员**：
- 读 `membersOf(workspace)` 的列表，按加入时间（服务端的顺序）。每行：显示名（自己的一行标"你"）、邮箱、角色、加入日期（`formatDate`）。访客看到的邮箱是 `null`，这时整列不显示。列表的名称是"成员"（`aria-label`），与邀请列表区分。
- **管理员**：别人的一行有角色与"移出"。
  - 角色是一个菜单（`DropdownMenu` 的单选项），按钮显示当前的角色：选中就发 `PATCH`，菜单关闭时焦点回到按钮；发送中按钮 `aria-busy`，不接受新的选择，所以一行一次只发一个。放弃了原生 `<select>`：发送中禁用它，焦点落到 body（审查 T3）；Windows、Linux 的 Chrome 在收起的 select 上按方向键就触发 change，每按一次发一个中间角色（审查 Q9）。
  - 控件的名称带上邮箱（"成员，Bob（bob@…）的角色""移出 Bob（bob@…）"）：两位成员可能同名，管理员看得到邮箱（审查 T18）。
  - 移出经 `ConfirmDialog`："把 Bob 移出 Acme？"，说明他的待接受邀请一并删除。成功之后重新读取邀请：服务端一并撤回了对他邮箱的待接受邀请（审查 T1）。
  - 自己的一行没有这两个控件：接口答 409 `workspace.own_membership`，页面不提供。
- **写的失败**：
  - 改角色的原因显示在列表上方（`role=alert`），下一次改角色时清掉（审查 T2）。不论哪种失败，页面都重新读取成员与工作区列表，不按码分支：404 `member_not_found` 时那一行随之消失，403 `forbidden`（在别处被降级）时控件跟着新角色变化，5xx 多读一次也无害（审查 Q3）。
  - 移出、撤回、邀请的失败留在对话框或表单里，不另读：下一次获得焦点时的读取更新控件（同 P5 审查 Q1，审查 Q2）。

**邀请**（只有管理员看得到，也只有管理员去读）：
- **表单**：邮箱与角色（默认成员），经 `useForm`。
  - 本地检查：必填。邮箱先 `trim()`：`type=email` 的值浏览器只去掉 ASCII 空白，输入法打出的全角空格还在（审查 Q8）。
  - 422 的字段错误在邮箱下方，表单上方不另说：`not_allowed` 是"已经是这个工作区的成员。"，`duplicate` 是"已经邀请过：在下面复制那份邀请的链接。"。
  - 成功之后清空邮箱，`<output>` 说"已邀请 bob@…：复制链接发给他"，邮箱是服务端保存的（小写），不是输入的原文。
- **列表**：邮箱、角色、邀请日期、"复制链接""撤回"。
  - 复制：`navigator.clipboard.writeText(链接)`，成功之后 `<output>` 说"已复制给 bob@… 的邀请链接"，并收起这一行的链接框；撤回、复制失败时清掉这句（审查 T14）。按钮的名称"复制链接：bob@…"以可见文字开头（WCAG 2.5.3，审查 T12）。
  - 没有剪贴板（非安全上下文，如局域网地址的 HTTP），或写入被拒绝时，这一行下方显示只读的链接框，获得焦点时选中其中的文字，请用户自己复制。
  - 撤回经 `ConfirmDialog`："撤回给 bob@… 的邀请？链接随即失效"。
- **链接**：`app/invitation-link.ts` 的 `invitationLink(origin, {id, token})` 拼出 `${origin}/invitations/${id}#${token}`。令牌只在片段里（M2 总设计第 4 节）。origin 是管理员访问本站的地址：经内网地址访问时复制的是内网链接，服务端没有配置公开地址（README 写明，审查 Q5）。
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
| 预览加载中、失败 | `NotLoaded`：说明原因（404 是链接已失效）与重试，同别处的加载失败（审查 Q1）。不另放回到 `/` 的链接：顶栏的 Nerve Wiki 就是 |
| 未登录 | 预览：`{name}` 邀请你以{角色}加入。下面是"登录"与"注册"两个表单，两个按钮切换，默认登录。注册带上邀请：注册关闭时，只有被邀请的邮箱能注册，别的邮箱答 403 `identity.signup_disabled`，显示在表单上方，说法换成"这台服务器只为受邀的邮箱创建账户：请用收到邀请的邮箱注册。"（`CredentialsForm` 的 `texts`，3.5）。两个表单的本地检查与登录页、注册页共用（`signInProblems`、`signUpProblems`） |
| 已登录 | 预览，"以 {显示名}（{邮箱}）登录"，"接受邀请"与"退出"两个按钮 |
| 接受成功 | 转到 `/:slug`（`replace`：后退不回到用过的链接）。账户还没完成引导时，`Onboarded` 先把它带进引导，工作区一步因为已有工作区而自动继续 |
| 接受答 403 `workspace.invitation_email_mismatch` | 说明这份邀请发给了别的邮箱；"退出"之后用那个邮箱登录 |
| 接受答 404 | 链接已失效（被撤回、已被接受、工作区已删除） |

**会话的变化**（13.2 第 10 条）：
- 页内登录、注册、退出都只改变会话，页面不跳转。新的一代挂载时，这一页还在同一个带片段的地址上，于是从"未登录"变成"已登录"，或者反过来。
- 令牌从不进入 `next`：这一页不在守卫之下，没有 `next`。
- 接受不是会话的变化，之后由页面跳转。
- 页面在 `SignedIn` 之外，所以它经 `useSession` 看会话的状态，自己读账户（SWR 的键 `me`），用来显示登录的是谁。

**请求**：预览与接受都把令牌放在请求体里。W6 的页面版本核对页面发出的请求，地址里都没有令牌。

**已是成员**：照样接受（3 节"不做"），答复的工作区带他原来的角色，页面进入工作区。

### 3.5 停用对话框

- `ConfirmDialog` 加可选的 `texts`（`ProblemTexts`）：problem 码 → 这个对话框的文案键，没有列出的码照常经 `errorText`。键只能是已有文案的码，写错不能编译。
- 同一机制也在 `errorText(error, t, texts)`、`formErrors`、`useForm` 的选项（第二个参数是 `{ onField, texts }`）与 `CredentialsForm`：邀请页的注册被拒也要自己的说法（3.4）。
- `DeactivateDialog` 把 `workspace.sole_admin` 换成停用的说法："你是某个还有其他成员的工作区唯一的管理员。先在那里（工作区设置 → 成员）让另一位成员成为管理员，再停用。"
- 不列出工作区（第 2 节"不做"）。

### 3.6 前端的测试（vitest）

| 测试 | 守住 |
|---|---|
| `invitationLink`、`linkOf`：拼出与解析，空片段，片段里的编码 | 3.3、3.4 |
| `MemberStore`、`InvitationStore`：读不覆盖之后答复的写；改角色换掉那一项；移出、撤回，答 404 也移出；邀请放到最前、不重复 | 3.2 |
| `WorkspaceStore`：离开同删除（含 404）；接受加入列表，不重复，列表未读时留给读取 | 3.2 |
| `RootStore`：同一工作区同一个 store；别的工作区、新的一代是新的 | 3.2 |
| 成员页：列表与"你"；访客看不到邮箱列；管理员改别人的角色（焦点回到按钮，一次一个，被拒时角色不变、原因留到下一次），自己的一行没有控件；失败之后重新读取成员与工作区；移出（含 404）之后邀请一并不在；再读一次时列表更新；每个工作区读自己的成员与邀请；读不到时说明，重试之后加载；成员与访客看不到邀请一节，也不去读 | 3.3 |
| 邀请：本地检查；去首尾空白（含全角）；422 的两种在邮箱下方、上方不另说；默认角色是成员；创建之后在最前；复制写进剪贴板；没有剪贴板、写入被拒时显示链接框；"已复制"在撤回、复制失败时清掉；撤回（含 404）；读不到时说明，重试之后加载 | 3.3 |
| 离开：成功之后落到另一个工作区，其间不出现 404；唯一的管理员留在对话框里，说明原因 | 3.3 |
| 邀请页：链接不完整；预览 404；预览、账户读不到时说明，重试之后加载；会话不可用；未登录时登录、注册（请求体带邀请，按注册检查，被拒的说法）；已登录接受并进入受邀的工作区（另有按名称排在前面的工作区）；接受答 404；邮箱不符的说明与退出（发出 logout）；已是成员时进入，角色不变；接受之后未完成引导的账户进入引导 | 3.4 |
| 停用对话框：`workspace.sole_admin` 是停用的说法 | 3.5 |
| 保留名单：`invitations` 在 `[app]`（已有的测试） | 3.4 |

### 3.7 端到端

**夹具**：
- `member-pages.ts`：`membersList`、`membersListed`（每行的名称与其下一行）、`who(name, email)`（控件怎样称呼成员）、`roleOf`（角色的按钮）、`changeRoleWith`、`removeMemberWith`、`leaveWith`。
- `invitation-pages.ts`：`inviteWith`（答复的状态码，201 时还有邀请）、`copyLinkWith`（复制并读剪贴板）、`withdrawWith`、`linkTo(invitation, baseURL)`（链接的地址）、`acceptWith`。
- 剪贴板：W5 的上下文授予 `clipboard-read`、`clipboard-write`（Chromium）。

**故事**（页面版本，落库断言同接口版本，经 `fixtures/assert/workspace.ts`）：

| 故事 | 页面版本 |
|---|---|
| W5 邀请 | 管理员在成员页邀请一个邮箱，复制的链接就是列表里那份邀请的；邀请已是成员的邮箱、重复邀请，邮箱下方说明；撤回一份之后它的链接答 404。成员看不到邀请一节 |
| W6 接受邀请 | 已有账户的被邀请人（另有一个按名称排在前面的工作区）：未登录打开链接，看到预览，页内登录，接受，进入受邀的工作区。别的账户登录时看到邮箱不符的说明，退出之后页面回到登录。页面发出的请求，地址里都没有令牌 |
| W7 带邀请注册 | 注册关闭的服务上，在邀请页用被邀请的邮箱注册，接受，经引导进入工作区；用别的邮箱注册看到注册已关闭 |
| W8 成员与角色 | 成员页列出成员；访客看不到邮箱；管理员把访客改成成员，落库，焦点回到角色的按钮；自己的一行没有角色控件 |
| W9 移出与离开 | 管理员移出成员，他的待接受邀请在页面上与库里一并删除；访客离开，落到 `/` 的落点；唯一的管理员离开被拒，对话框说明原因 |
| W10 停用与恢复 | 唯一的管理员在安全页停用，对话框说明原因，会话仍在；另一位成员成为管理员之后停用成功，回到登录页 |

## 4. 实施步骤

分支 `m2-p6-web-members-invitations`，每个 Step 结束时 `make check` 为绿。每个 Step 是一段完整的功能，所以 knip 不会遇到没有用到的导出。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | 成员页的成员与离开：`MemberService`、`MemberStore`、`membersOf`；`WorkspaceService.leave`、`WorkspaceStore.leave`；设置导航的"成员"；成员列表、改角色、移出、离开；角色的控件（审查之后改为菜单） | [P6-S1](plans/P6-S1-members.md) |
| S2 | 邀请：`InvitationService`、`InvitationStore`、`invitationsOf`；邀请链接；成员页的邀请一节（表单、列表、复制、撤回） | [P6-S2](plans/P6-S2-invitations.md) |
| S3 | 公开的邀请页：`InvitationPreviewService`、`InvitationPreviewStore`；`WorkspaceStore.accept`；带邀请的注册；路由与保留名单 | [P6-S3](plans/P6-S3-invitation-page.md) |
| S4 | 停用对话框：`ConfirmDialog` 的 `texts`；sole_admin 的说法 | [P6-S4](plans/P6-S4-deactivate.md) |
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
| 邀请链接把令牌放进查询参数 | `invitationLink` 测试、复制的测试、W5 页面版本（W6 打开夹具拼出的链接，不经页面拼出，审查 D2） |
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

分支 `m2-p6-web-members-invitations`：S1 `e58e051`、S2 `0815d7b`、S3 `a914234`、S4 `8c1cec5`、S5 `f51b526`，审查修复 `7f999e3`、`6733b53`，合并 `33ca368`。第 5 节全部通过，反向对照按预期失败（作者在各 Step 单元 39 项、e2e 6 项，审查修复另单元 26 项、e2e 4 项；审查者单元 46 项、Go 2 项、e2e 6 项）；"令牌放进查询参数"让 W5 而不是 W6 失败，第 5 节已改正（审查 D2）。`make check`（vitest 679 个）、`make gen-check`、`make e2e`（82 个；页面版本另跑 `--repeat-each 3`，54 次全部通过）、`make image-smoke` 本地与持续集成为绿（本地的 e2e 与 image-smoke 在合并提交上跑；工作区里有未跟踪的 `.claude/`，image-smoke 报 `modified=true`）。审查见 [P6 审查记录](reviews/P6-web-members-invitations-review.md)：没有 Major，11 项 Minor 与 7 项 Nit 已处理；12 个疑问中 Q4、Q8、Q9 随 T3、T13 改，Q2、Q3、Q5 改文档或 README，其余不改。规模（新增行数）：生产代码约 1,310 行（含文案约 110 行），测试约 1,250 行，端到端约 480 行。

与设计的出入（已同步进上文）：

1. 按 problem 码换说法的选项叫 `texts`（`ProblemTexts`，设计写的是 `messages`），不只在 `ConfirmDialog`：`errorText`、`formErrors`、`useForm` 的选项（第二个参数改为 `{ onField, texts }`）与 `CredentialsForm` 都接受。邀请页的注册被拒也要自己的说法（3.4、3.5）。
2. 登录、注册的本地检查挪进 `credentials-form.tsx`（`signInProblems`、`signUpProblems`），登录页、注册页与邀请页共用；`useSession` 从 `guards.tsx` 导出（3.1、3.4）。
3. `InvitationLink` 类型在 `services/invitation.service.ts`：services 不导入 `app/`（3.2）。
4. 角色的列表抽成 `pages/workspace/role-options.tsx`；`RootStore` 的两份缓存共用 `once`（3.1、3.2）。
5. 预览失败（含 404）用 `NotLoaded`，没有另放回到 `/` 的链接（3.4，审查 Q1）。
6. 邀请成功之后显示服务端保存的邮箱（小写）。邮箱的 `trim()` 曾因 `type=email` 由浏览器去空白而去掉，审查发现全角空格不在其列，恢复（3.3，审查 Q8、T13）。
7. 成员列表有名称（`aria-label`"成员"），与邀请列表区分（3.3）。
8. 角色是菜单的单选项，不是原生 select（3.3，审查 T3、Q9）；写的失败一律重新读取成员与工作区，不只是 403（3.3，审查 Q3）。
9. `watchFor` 挪到 `src/test/watch.ts`，常规页与成员页的测试共用；邀请一节的测试单独一个文件。
10. e2e：W5、W6、W8、W9 的页面版本各两个测试；W6 自己收集页面请求的完整地址（`pageWatch.apiRequests` 只记路径）；夹具名与 3.7 原来写的不同，W8 改的是访客的角色，W9 离开的是访客（3.7，审查 D3–D5）。

审查之后的修复（详见审查记录）：T1 移出之后重新读取邀请；T2 改角色的失败在下一次清掉；T3 角色改为菜单，焦点回到按钮，发送中不接受新的选择，控件的名称带上邮箱（T18）；T4–T11 补上没有守住的测试，W6 的被邀请人另有按名称排在前面的工作区，W8 断言焦点，W9 断言页面上的邀请；T12 复制按钮的名称以可见文字开头；T13 恢复 `trim()`；T14 "已复制"适时清掉；T15 `ProblemTexts` 的键收窄、`useWorkspaces` 的注释；T16、T17 文案与夹具的类型；README 的前端一节写明链接的 origin（Q5）与页面自己的 `texts`（D8）。

留给之后的：

- **M2 收尾（审查 D8）**：总体设计 13.2 第 12 条（表单）补上页面自己的 `texts`。
- **之后各 M（审查 Q7）**：`field.email.not_allowed`、`field.email.duplicate` 是全局的字段文案，现在只有邀请答出它们；别的操作对 `email` 答出这两个码而意思不同时，要么改写文案让两处都适用，要么到那时给 `formErrors` 加按字段码换说法的选项。`texts` 只换 problem 码，换不了字段码（M2 收尾审查 B-M1，总体设计 13.2 第 11 条）。
- **M3 的按工作区的 store**：照 `membersOf`、`invitationsOf` 按工作区 id 每代缓存（`once`），SWR 的键带工作区 id，并有"每个工作区读自己的"测试（审查 T6）。
