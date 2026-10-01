# M3/P4 前端笔记本：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M3/P4 前端笔记本 |
| 状态 | 已完成（`273ebe2` 合并，审查见 [P4 审查](reviews/P4-web-notebooks-review.md)） |
| 基线 | `f9c36e7`（P3 合并、P3 文档更新之后的 main） |
| 上级文档 | [M3 总设计](00-M3-design.md) 第 1 节第 6 条、第 3、4、5、7、9 节；[M2 移交](handoffs/M2-workspace.md)第 5 项；[M2/P5](../M2-workspace/05-P5-web-shell-workspaces.md)、[M2/P6](../M2-workspace/06-P6-web-members-invitations.md)（外壳、store 的写法、成员页）；[总体设计](../v0.1-design.md) 3.3、13.2 |

---

## 1. 基线

P1–P3 之后：

- 接口：笔记本与成员的 10 个操作、无主与审计的 4 个都已就绪（M3 总设计第 5 节）；四个 notebook 码都有中英文案。
- 前端：
  - 工作区外壳 `/:slug`：左栏是 `aside`（"工作区"），里面是切换器与 `nav`（首页、设置）；首页只说"还没有笔记本"。
  - 工作区设置的常规页、成员页；成员行的角色菜单 `RoleMenu` 写死工作区的三种角色。
  - 删除、离开工作区之后外壳 `<Navigate replace to="/">`，落点之后焦点在 `body`（M2 移交第 5 项、13.2 第 17 条留给 M3）。
  - 引导两步：资料、工作区。
- e2e：N1–N6 只有接口版本；引导的注册表在 `e2e/fixtures/auth.ts` 是两步。

## 2. 目标与范围

**目标**：工作区的成员在左栏看到自己的笔记本（"我的笔记本""团队笔记本"），新建笔记本，进入笔记本的首页与设置；管理员改名、改开放程度、删除、管理成员；成员离开。新账户在引导里建第一个个人笔记本。整页离开的操作之后，焦点到落点的主标题。

**做**：

- 笔记本的 service 与 store：按工作区的笔记本列表，按笔记本的成员列表（13.2 第 15 条）。
- 左栏：外层改为不带地标的容器，`nav` 里是工作区的各节、"我的笔记本""团队笔记本"两组与"新建笔记本"（M2 移交第 5 项）。工作区首页列出两组。
- 笔记本：`/:slug/notebooks/:id` 首页；`/:slug/notebooks/:id/settings/general`（改名、开放程度、删除）与 `…/members`（列表、添加、改角色、移出、离开）。
- 整页离开之后的焦点：删除、离开工作区，接受邀请进入工作区，删除、离开笔记本，新建笔记本之后，焦点到落点的主标题。
- 引导的第三步 `notebook`。
- e2e：N1–N6、N12 的页面版本，N12 的接口版本；引导多了一步的故事（A9、W11、A4、W7）随之调整。

**不做**：

| 事项 | 理由与去处 |
|---|---|
| 无主笔记本页、审计、`notebook.sole_admin` 在离开工作区与停用对话框里的说明 | P5 |
| 页面树、笔记本首页的内容 | M4：首页在 M3 只显示名称与"还没有页面" |
| 笔记本的搜索、排序选项、拖动排序 | 服务端按名称排序；v0.1 不做 |
| 添加成员时的搜索框 | 候选是工作区的有效成员，小团队一个下拉框够用（与 M2/P6 成员列表不分页同理） |
| 笔记本名称的完整本地检查 | 名称规则（`shared.CheckTitle`）在服务端；本地只查必填与长度（按 UTF-8 字节），其余由 422 的字段错误说明，字段下方另有一句规则的提示。在前端再写一份字符表，两处会走样 |

## 3. 设计

### 3.1 文件

```
web/apps/web/src/
  app/routes.tsx                         /:slug/notebooks/:id，其下 settings/general、settings/members
  app/arrival.ts                         整页离开之后的到达：arrived 状态、useArrivalFocus
  app/notebook-name.ts                   notebookNameProblem（必填、255 字节）
  app/role-menu.tsx                      RoleMenu 改为按角色列表与文案通用（从 member-row.tsx 移出）
  app/member-summary.tsx                 成员是谁（名称、"你"、邮箱、加入日期），两种成员行共用
  services/notebook.service.ts           NotebookService：list、create、get、update、remove、leave
  services/notebook-member.service.ts    NotebookMemberService：list、add、update、remove
  stores/notebook.store.ts               一个工作区的笔记本；groupNotebooks
  stores/notebook-member.store.ts        一个笔记本的成员
  stores/order.ts                        byName（工作区与笔记本共用）
  stores/root.store.ts、context.tsx      notebooksOf、notebookMembersOf；useNotebooks、useNotebookMembers
  pages/workspace/workspace-layout.tsx   左栏：容器、nav、两组、新建
  pages/workspace/notebook-nav.tsx       左栏的两组与"新建笔记本"
  pages/workspace/notebook-groups.tsx    两组（标题与有名称的列表），左栏与首页共用一次读
  pages/workspace/create-notebook-dialog.tsx   新建笔记本的对话框
  pages/workspace/workspace-home.tsx     首页列出两组；主标题接受到达的焦点
  pages/notebook/notebook-layout.tsx     在工作区的笔记本列表里找到它；useNotebook
  pages/notebook/notebook-home.tsx       名称与"还没有页面"
  pages/notebook/settings-layout.tsx     笔记本设置的导航
  pages/notebook/general-page.tsx        改名、开放程度、删除
  pages/notebook/members-page.tsx        成员、添加、离开
  pages/notebook/notebook-member-row.tsx 一行成员
  pages/notebook/access-options.tsx      开放程度的三项与说明
  pages/notebook/notebook-roles.tsx      笔记本的三种角色、下拉框的选项
  pages/workspace/member-row.tsx         用 app/role-menu.tsx
  pages/landing.tsx、create-workspace.tsx、invitation.tsx   转交、接受到达的状态
  onboarding/steps.ts、notebook-step.tsx、go-on.tsx（GoOn，工作区与笔记本两步共用）
  i18n/messages/{en,zh-CN}.ts
  test/fakes.ts                          byRoute 的 * 是一段路径；每个工作区的笔记本缺省为空
  test/notebook-server.ts                笔记本设置两页的假服务器
e2e/
  fixtures/auth.ts                       onboardingSteps 加 notebook
  fixtures/notebook-pages.ts             笔记本页面的操作
  fixtures/workspace-pages.ts、onboarding-pages.ts   左栏不再是 complementary；笔记本一步
  stories/notebook/n1–n6                 页面版本
  stories/notebook/n12-onboarding-notebook.spec.ts   N12 的接口与页面版本
  stories/identity/a9、a4，stories/workspace/w7、w11   引导多一步
```

### 3.2 数据：service 与 store

**services**（13.2 第 2、6 条）：

| service | 方法 |
|---|---|
| `NotebookService` | `list(slug)`、`create(slug, {name, workspace_access?})`、`update(id, {name?, workspace_access?})`、`remove(id)`、`leave(id)` |
| `NotebookMemberService` | `list(notebookId)`、`add(notebookId, {user_id, role})`、`update(id, role)`、`remove(id)` |

`leave` 放在 `NotebookService`：它改变的是账户看得到的笔记本（同 M2/P6 把离开放进 `WorkspaceService`）。

**stores**（13.2 第 1、15 条）：

- **`NotebookStore`**：一个工作区的笔记本，服务端的顺序（`lower(name)`、`name`、`id`，`byName` 与 `WorkspaceStore` 共用，移到 `stores/order.ts`）。
  - `load()`：读出去之后已有写答复的丢弃。第一次读还没答、已有写答复时（列表加载之前就能新建），再读一次（P4 审查 m2）。
  - `create(body)`：按 id 去掉已有的再放进，排序；返回它。
  - `update(id, patch)`：换掉那一项，排序（改名改变次序）。
  - `remove(id)`：删除；答 404 `notebook.not_found` 也当作已完成。从列表移出，记下 id（`wasRemoved`）。
  - 同一本笔记本的 `update`、`remove`、`leave` 经按笔记本 id 的 `oneAtATime` 一次一个（13.2 第 1 条）：常规页的两个表单可以各自发出（P4 审查 M1）。
  - `leave(id)`：离开之后**再读工作区的笔记本列表**：没有它就同 `remove` 移出并记下；有就换掉那一项（对工作区开放的笔记本，离开之后仍以默认角色看得到，留在页面上）。不读这一本：看不到时它答 404，浏览器把它记为错误（13.4 第 3 条，S5 的 N5 发现），`NotebookService` 因此没有 `get`。离开答复之后再读失败，列表不动、记下 `wasRemoved`，下一次读没有它时外壳回到工作区首页（P4 审查 m5）。离开答 404 `notebook.not_found` 当作已移出；答 404 `notebook.member_not_found`（已不是显式成员）原样抛出，由对话框说明。
- **`NotebookMemberStore`**：一个笔记本的有效显式成员，按加入时间。`add` 放到最后（按 id 去掉已有的）；`changeRole` 换掉那一项；`remove` 答 404 `notebook.member_not_found` 也移出。
- **缓存**：`RootStore.notebooksOf(workspace)` 按工作区 id，`notebookMembersOf(notebook)` 按笔记本 id，都经 `once`。SWR 键 `["notebooks", workspace.id]`、`["notebook-members", notebook.id]`。
- **分组**：`groupNotebooks(list)` 是纯函数：`workspace_access` 为 `none` 且 `member_count` 为 1 的进"我的笔记本"，其余进"团队笔记本"（M3 总设计第 5 节），各自保持列表的顺序。
- **何时重读笔记本列表**：添加、移出成员之后（`member_count` 变化，可能换组：N2），页面重读 `["notebooks", workspace.id]`；改角色不变。改名、改开放程度由答复换掉那一项。

### 3.3 左栏与工作区首页

- **结构**（M2 移交第 5 项）：外层是不带地标的 `div`；切换器在上；其下一个 `nav`（名称是工作区名）：首页、设置，然后两组笔记本与"新建笔记本"。去掉 `workspace.sidebar` 的 `aside`。
- **两组**：每组一个标题（`h2`，"我的笔记本""团队笔记本"）与一个有名称的列表（`aria-labelledby` 指向标题），每项一个 `NavItem` 指向笔记本首页。空的组不显示；两组都空时显示"还没有笔记本"。列表还没读到时显示 `NotLoaded`（读不到时说明原因与重试，13.2 第 7 条）。
- **新建笔记本**：按钮只给工作区的管理员与成员（访客建笔记本答 403，不提供）。对话框（`components/ui/dialog.tsx`）里是名称与开放程度（缺省私密），经 `useForm`：本地检查必填与长度，422 的字段错误在字段下方。名称字段下方一句规则的提示。成功之后关闭对话框，进入新笔记本的首页（带到达的状态）。
- **工作区首页**：主标题是工作区名（接受到达的焦点）；下面同样的两组，以卡片的链接列出；没有笔记本时说"还没有笔记本"，管理员与成员另有"新建笔记本"。

**实现**：两组由 `NotebookGroups` 画出，左栏与首页共用一次读。首页的"新建笔记本"只在没有笔记本时出现（左栏始终有）。新建对话框每次开、关记一轮：发出新建时记下轮次，答复时轮次变了（取消、再开）就不跳转，笔记本照样进左栏；首页空状态里的对话框随列表有了笔记本而卸载，轮次不变，照样进入（P4 审查 m1）。新建之后阻止 Radix 把焦点还给触发按钮。

### 3.4 笔记本的页面

**路由**：`/:slug/notebooks/:id` 挂在工作区外壳之下（不是新的顶层段，13.1 第 26 条不涉及）：

```
:slug
  notebooks/:id          NotebookLayout
    (index)              NotebookHomePage
    settings             NotebookSettingsLayout
      (index)            → general
      general            NotebookGeneralPage
      members            NotebookMembersPage
```

- **`NotebookLayout`**：在 `notebooksOf(workspace)` 的列表里找 `id`（与工作区外壳同理，13.2 第 16 条）。列表还没读到：`NotLoaded`；找不到：这一代删除或离开过它（`wasRemoved`）就 `<Navigate replace to="/:slug" state={arrived}>`，否则 404 页。找到时 `<Outlet key={notebook.id}>`，子页经 `useNotebook()` 取它。不另发 `GET /notebooks/{id}`：列表就是"看得到的笔记本"，开放程度与有效角色都在里面。
- **首页**：主标题是笔记本名（接受到达的焦点），"还没有页面"；一行"设置"的链接。
- **设置**：标题"笔记本设置"，导航"常规""成员"，与工作区设置相同的布局。所有看得到它的人都能进：读者能列出成员（规则表），非管理员只读。

**常规页**：

- **名称**：管理员改名（`RenameForm` 的写法：只在变化时发，去空白回写，记编辑次数）；非管理员只读，并说明"只有管理员能修改"。
- **开放程度**：三个单选项，各带一句说明：私密（只有成员）、工作区成员可阅读、工作区成员可编辑；访客始终只经显式加入。管理员选中之后点"保存"发出（不在选中时就发：单选组用方向键移动就会选中）。非管理员只读。
- **删除**（管理员）：`ConfirmDialog`，要输入笔记本名称（`typedConfirmation`，M3 总设计第 4 节）。成功之后外壳回到工作区首页（`wasRemoved`），焦点到首页的主标题。

**成员页**：

- **成员**：显示名（自己一行标"你"）、邮箱（访客看到 `null`，整列不显示）、角色、加入日期。管理员对别人一行有角色菜单（`app/role-menu.tsx`，三种笔记本角色）与"移出"（`ConfirmDialog`，`focusAfter` 到本节标题）；自己一行没有（409 `notebook.own_membership`）。改角色失败的原因在列表上方，并重读成员与笔记本列表（M2/P6 3.3 的做法）。
- **添加**（管理员）：候选是工作区的有效成员（`membersOf(workspace)`，SWR `["members", workspace.id]`）中还不是这个笔记本有效成员的，原生 `select` 按显示名（看得到邮箱时带上邮箱），角色缺省"编辑者"。成功之后清空选择，`<output>` 说"已添加 …"，重读笔记本列表。422 `user_id: not_allowed`（他刚离开工作区）、`duplicate`（刚被别人加入）显示在字段下方，并重读两份列表。没有候选时说明"工作区的成员都已在这个笔记本里"。
- **离开**：只给显式成员（成员列表里有自己；只靠默认角色看到它的人没有可结束的成员关系，P2 审查 Q5）。`ConfirmDialog`；唯一的管理员答 409 `notebook.sole_admin`，对话框说明；`notebook.member_not_found` 在这个对话框里说"你已经不是这个笔记本的成员了"（`texts`）。成功之后：看不到它了，外壳回到工作区首页；仍以默认角色看得到（开放的笔记本），留在成员页，成员列表重读。

**实现**：首页的设置链接叫"笔记本设置"（左栏已有工作区的"设置"）；设置页的主标题是"{name} 的设置"（P4 审查 m12）。非管理员以文字显示名称与开放程度。改名与开放程度两个表单只在编辑之后才有草稿，没有草稿时显示列表里的值（别处的改动也跟着，"保存"不会把旧值写回），保存成功且期间没有再编辑才清掉草稿、显示"已保存"；工作区的改名表单同样改了（P4 审查 m3、m4）。添加的下拉框在没有候选时也保留，"工作区的成员都已在这个笔记本里"作为它的提示，拒绝的原因与焦点在列表重读之后仍在；被拒时连同笔记本列表一起重读。`user_id` 的问题用全局文案 `field.user_id.*`。离开之后笔记本仍在列表里才重读成员（看不到时成员列表答 404）。

### 3.5 整页离开之后的焦点（M2 移交第 5 项，13.2 第 17 条）

- **到达**：`app/arrival.ts` 定义路由状态 `arrived = { arrived: true }` 与 `useArrivalFocus()`：页面的主标题（`h1`，`tabIndex={-1}`）在挂载时，若地址的状态带着 `arrived`，取得焦点。
- **谁带上它**：
  - 工作区外壳对这一代删除或离开的工作区 `<Navigate replace to="/" state={arrived}>`；`LandingPage` 把收到的状态转交给它的 `<Navigate>`；落到的工作区首页或创建页的主标题取得焦点。
  - 接受邀请之后 `navigate("/:slug", { replace: true, state: arrived })`。
  - 笔记本外壳对删除或离开的笔记本 `<Navigate replace to="/:slug" state={arrived}>`。
  - 新建笔记本之后进入它的首页，带上 `arrived`（触发它的对话框随之关闭，焦点无处可回）。
- **接受焦点的主标题**：工作区首页、创建工作区页、笔记本首页。
- 放弃的做法：在 store 里记"下一页要聚焦"。状态随着 `replace` 留在这一条历史里，返回或刷新到它时再聚焦一次，无害；store 的标记要清理，且跨不过 `LandingPage` 的转交。

### 3.6 引导的笔记本一步

`onboarding/steps.ts` 加第三步 `notebook`（M3 总设计第 4 节）：

- **目标工作区**：落点的顺序（本设备最后访问的，再按名称）中第一个他是管理员或成员的工作区（`notebookTarget(list, last)`，纯函数，与 `landingPath` 并列在 `app/landing.ts`）。
- **没有目标**（没有工作区，或处处只是访客）：说明"在一个你是成员的工作区里才能建笔记本；加入之后在左栏新建"，"继续"完成这一步。
- **目标里已有看得到的笔记本**：直接继续（`GoOn`，从工作区一步导出共用）。完成这一步失败之后重试，不会多建一个：建好的笔记本已在列表里。
- **否则**：名称缺省"我的笔记"（随界面语言），可以改；建一本私密笔记本，列表随之有了它，这一步随即像上一条那样继续。
- e2e 的 `onboardingSteps` 同时加 `notebook`（M2 移交第 5 项第二点）。

**实现**：`GoOn` 单独成 `onboarding/go-on.tsx`；建好之后把目标工作区记为本设备最后访问的，引导结束时落在它（有 `next` 时仍去 `next`，P4 审查 Q1）。

### 3.7 文案

- 笔记本的角色 `notebookRole.{admin,editor,reader}`，开放程度 `access.{none,viewer,editor}` 与说明，左栏两组与新建，设置两页、成员页、引导一步，到达不需要文案。
- 中英一起加（13.2 第 4 条）。

**实现**：笔记本名称的 422 按标题的规则说明：`useForm` 与 `formErrors` 加表单自己的 `fieldTexts`（"字段.码"到文案，按表单的字段定类型），即 13.2 第 11 条在同一码对不同操作含义不同时的表单路径；`field.notebook_name.*` 三条，名称字段下方一句规则的提示。

### 3.8 前端的测试（vitest）

- **store**：`NotebookStore`、`NotebookMemberStore` 的写按表格驱动：重叠的读丢弃、写放进列表（去重、排序）、删除类的写答本资源的 404 当作完成、答 403 保留并抛出；`leave` 之后再读的两种结果。`groupNotebooks` 的表格。
- **缓存**：`notebooksOf` 同一工作区同一个、别的工作区与新的一代是新的；`notebookMembersOf` 按笔记本（`root.store.test.ts`）。
- **页面**：
  - 左栏：两组、空的组不显示、访客没有"新建"、新建对话框（本地检查、422、成功之后进入并聚焦）、读不到时说明与重试、再读一次显示别处的变化。
  - 笔记本外壳：找到、404、删除或离开之后回到工作区首页；从一本直接到另一本，各读各的成员（13.2 第 15 条）。
  - 常规页：管理员改名、改开放程度、删除（输入名称）；非管理员只读。
  - 成员页：列表、添加（候选、成功、422）、改角色（失败时重读）、移出（焦点）、离开（只给显式成员、409、离开之后仍看得到或看不到）。
  - 到达的焦点：删除、离开工作区与笔记本、接受邀请、新建笔记本之后，主标题取得焦点；没有到达状态时不抢焦点。
  - 引导一步：目标工作区的选择（表格）、没有目标、已有笔记本直接继续、新建之后继续、完成失败之后重试不多建。
- 每个由 SWR 加载的区块有"读不到时说明原因、重试之后加载"与"再读一次时显示别处的变化"两种测试（13.2 第 7 条）。

### 3.9 端到端

- **页面版本**（`fixtures/notebook-pages.ts` 的 `<动作>With(page, …)`，返回请求的答复；断言与接口版本共用 `assert/notebook.ts`）：
  - N1：成员在左栏新建，进入首页（主标题取得焦点），它在"我的笔记本"；访客没有"新建"；名称不合规则时字段下方说明。
  - N2：私密笔记本对别人不在左栏、直接打开是 404 页；加了第二位成员，两人的左栏都把它移到"团队笔记本"。
  - N3：改开放程度，工作区的成员在"团队笔记本"里看到它，有效角色随之；访客看不到。
  - N4：管理员添加（含访客）、改角色、移出；非管理员只读；自己一行没有控件。
  - N5：成员离开，回到工作区首页；唯一的管理员离开被拒，对话框说明。
  - N6：改名；删除要输入名称，之后回到工作区首页，焦点在主标题。
  - N12：新账户的引导第三步建"我的笔记"，进入工作区之后它在"我的笔记本"；已有看得到的笔记本直接继续；只是访客的看到说明后继续。
- **N12 的接口版本**：建笔记本、记下引导的一步 `notebook`（`POST /me/onboarding-steps`）。
- **引导多了一步**：A9 改为三步（"第 3 步，共 3 步"，记下的步骤三个）；W11 的页面版本在工作区一步之后经过笔记本一步；A4、W7 经过它；`fixtures/onboarding-pages.ts` 加笔记本一步的定位。
- **工作区的焦点**：W4（删除工作区）的页面版本断言落点的主标题取得焦点。

## 4. 实施步骤

分支 `m3-p4-web-notebooks`。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | service、store、缓存与分组；`byName` 共用；到达的焦点与它在工作区一侧的用处（删除、离开、接受邀请） | [P4-S1](plans/P4-S1-data-arrival.md) |
| S2 | 左栏（容器、nav、两组、新建对话框）、工作区首页、笔记本外壳与首页、路由 | [P4-S2](plans/P4-S2-shell.md) |
| S3 | 笔记本设置：常规页、成员页；`RoleMenu` 通用 | [P4-S3](plans/P4-S3-settings.md) |
| S4 | 引导的笔记本一步；e2e 的注册表与引导多一步的故事 | [P4-S4](plans/P4-S4-onboarding.md) |
| S5 | 端到端：N1–N6、N12 的页面版本，N12 的接口版本，W4 的焦点 | [P4-S5](plans/P4-S5-e2e.md) |

每个 Step 结束时 `make check` 为绿；S4、S5 之后 `make e2e` 为绿。

## 5. 测试与验证

照 M2/P5、P6：vitest（3.8）、e2e（3.9）；反向对照：分组只看 `workspace_access`、`leave` 不再读、删除答 404 时不移出、到达状态不转交、引导一步已有笔记本时仍建、访客也有"新建"、离开给非显式成员，各让对应的测试失败。

实际跑过的反向对照与结果见 [P4 审查](reviews/P4-web-notebooks-review.md)"反向对照"一节。

## 6. 完成标准

- 第 2 节"做"的各项完成；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿，持续集成四个任务为绿。
- M2 移交第 5 项落实，在 M3 总设计第 7 节的表中核对；13.2 第 17 条的"整页离开之后的焦点"写定（M3 收尾补进总体设计）。
- 审查记录 `reviews/P4-web-notebooks-review.md`；本文第 7 节、M3 总设计的进度表已更新。

## 7. 结果

- 分支 `m3-p4-web-notebooks`：S1 `024edb6`、S2 `728e8de`、S3 `c637163`、S4 `ec50cea`、S5 `037d87b`；审查修复 `b829c23`；`273ebe2` 合并（`--no-ff`）。
- 门禁：每个 Step 的 `make check` 为绿；`make gen-check`、`make e2e`（110 个）、`make image-smoke` 为绿；持续集成四个任务为绿。
- 审查：[P4 审查](reviews/P4-web-notebooks-review.md)，1 项 Major（同一本笔记本的两次改动并发）、12 项 Minor、12 项 Nit 已处理。
- M2 移交第 5 项落实：左栏不再是 `aside`，`nav` 以工作区命名；整页离开之后焦点到落点的主标题；引导第三步加进 e2e 的 `onboardingSteps`，A9 为三步。M3 收尾时改为 done，到达的焦点（13.2 第 17 条）补进总体设计。

**与设计的偏差**（已同步进上文）：

1. `NotebookGroups` 由左栏与首页共用（3.3）。
2. `useForm` 的 `fieldTexts`（3.7）。
3. 首页的"新建"只在没有笔记本时出现（3.3）。
4. 首页的设置链接叫"笔记本设置"，设置页主标题带名称（3.4）。
5. 非管理员以文字显示开放程度（3.4）。
6. 添加的下拉框在没有候选时保留（3.4）。
7. `GoOn` 单独成模块；引导落在建笔记本的工作区（3.6）。
8. 离开之后再读列表而不是这一本，`NotebookService` 没有 `get`（3.2）。
9. 新建对话框不把焦点还给触发按钮，按轮次忽略取消之后的答复（3.3）。
10. 同一本笔记本的改动一次一个；第一次读的再读；离开之后再读失败也算完成（3.2）。
11. 设置布局与首页的设置链接在 S3 随两页一起做（S2 计划）。

**留给后面的**：无主笔记本页、审计、`notebook.sole_admin` 在离开工作区与停用对话框里的说明（P5）；成员行、改名表单的合并与有效角色的提示（M4，P4 审查 N3、Q3）；外壳的 `main` 与左栏的关系（M4 的页面树，P4 审查 Q2）；到达的焦点、表单的 `fieldTexts` 补进总体设计第 13 节（M3 收尾）。
