# M3/P5 前端无主与级联：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M3/P5 前端无主与级联 |
| 状态 | 已完成（`219f127` 合并，审查见 [P5 审查](reviews/P5-web-ownerless-review.md)） |
| 基线 | `a62f732`（P4 合并、P4 文档更新之后的 main） |
| 上级文档 | [M3 总设计](00-M3-design.md) 第 3、4、5、7 节；[P3 文档](03-P3-cascade-ownerless.md) 3.2–3.4；[P4 文档](04-P4-web-notebooks.md)（store、外壳、到达的焦点）；[总体设计](../v0.1-design.md) 3.4、13.2 |

---

## 1. 基线

P1–P4 之后：

- 接口：无主清单、接管、删除无主、审计列表（游标分页，缺省 50 条，1–100）都已就绪；`leaveWorkspace`、`deactivateMe` 答 409 `notebook.sole_admin`（规则二），`removeWorkspaceMember` 不拒绝，移出之后他独自管理的笔记本成为无主（M3 总设计第 4 节）。
- 前端：
  - 左栏、笔记本的首页与设置（P4）；工作区设置只有常规、成员两页。
  - 离开工作区的对话框按通用文案说明 `notebook.sole_admin`（"在每一本的设置里……"）；停用对话框只为 `workspace.sole_admin` 有自己的文案。
  - 移出成员的说明没有提到无主。
- e2e：N7–N11 只有接口与命令行版本；N13 只有接口版本。

## 2. 目标与范围

**目标**：工作区的管理员在工作区设置里看到无主笔记本（谁的、何时成为无主、活动），接管或删除它们，并查看审计记录；首页提醒他有无主笔记本。离开工作区、停用被规则二拒绝时，对话框说清怎么办；移出成员之前知道他的笔记本会成为无主。

**做**：

- 无主与审计的 service 与按工作区的 store（13.2 第 1、15 条）。
- 工作区设置的第三页"无主笔记本"（只给工作区管理员）：清单（接管、删除）、审计记录（新的在前，"加载更多"）。
- 工作区首页：管理员有无主笔记本时的一行提醒与链接。
- 对话框：离开工作区、停用的 `notebook.sole_admin` 各有自己的文案；移出成员的说明加上无主；笔记本成员页在没有管理员时说明。
- e2e：N7–N11、N13 的页面版本；本 M 与之前各 M 的全部故事通过。

**不做**：

| 事项 | 理由与去处 |
|---|---|
| 在对话框里列出挡住离开的笔记本 | 服务端的原因只给 slug 与数量（M3 总设计第 4 节规则二）；前端只知道自己的角色与成员数，不知道别的管理员，列出来会不准 |
| 无主笔记本按名称搜索、筛选 | 一个工作区里无主的笔记本很少；与成员列表不分页同理 |
| 审计记录的筛选与导出 | v0.1 不做 |
| 接管之后自动进入那本笔记本 | 管理员常常一次处理几本：留在清单上，状态里给出打开它的链接 |

## 3. 设计

### 3.1 文件

```
web/apps/web/src/
  services/ownerless.service.ts            OwnerlessService：list、takeOver、remove、auditEvents
  stores/ownerless.store.ts                OwnerlessStore：一个工作区的无主笔记本
  stores/audit.store.ts                    AuditStore：一个工作区的审计记录（第一页与"加载更多"）
  stores/root.store.ts、context.tsx         ownerlessOf、auditOf；useOwnerless、useAudit
  stores/notebook.store.ts                 receive：接管的笔记本按答复放进列表
  lib/one-at-a-time.ts                     oneAtATimeById：按 id 一次一个（P4 的队列抽出共用）
  app/follow-role.ts                       useFollowRole：只给管理员的读答 403 时重读工作区列表
  app/member-summary.tsx                   memberWho 只要名称与邮箱（原所有者也用）
  i18n/format.ts                           formatBytes
  app/routes.tsx                           /:slug/settings/ownerless
  pages/workspace/settings-layout.tsx      第三项，只给管理员
  pages/workspace/ownerless-page.tsx       页面：清单与审计两节
  pages/workspace/ownerless-row.tsx        一行无主笔记本：接管、删除
  pages/workspace/audit-section.tsx        审计记录
  pages/workspace/workspace-home.tsx       管理员的无主提醒
  pages/workspace/members-page.tsx         离开的 notebook.sole_admin 文案；移出的说明
  pages/settings/deactivate-dialog.tsx     停用的 notebook.sole_admin 文案
  pages/notebook/members-page.tsx          没有管理员时的说明
  i18n/messages/{en,zh-CN}.ts
  test/fakes.ts                            ownerlessJSON、auditEventJSON；每个工作区的无主清单缺省为空
e2e/
  fixtures/ownerless-pages.ts              无主页的操作
  fixtures/test.ts                         anotherPage：另一个账户的标签页，同样受检
  fixtures/workspaces.ts                   newOnboardedTeam：管理员完成引导的团队
  stories/notebook/n7–n11、n13             页面版本（n2 的第二个标签页改用 anotherPage）
```

### 3.2 数据：service 与 store

**service**（13.2 第 2、6 条）：`OwnerlessService` 的 `list(slug)`、`takeOver(id)`（答 `Notebook`）、`remove(id)`、`auditEvents(slug, cursor?)`（答 `{data, next_cursor}`；页长用服务端的缺省 50，没有用处的 `limit` 不加）。

**stores**：

- **`OwnerlessStore`**（按工作区，`ownerlessOf(workspace)`）：服务端的次序（成为无主早的在前），`load()`（读出去之后已有写答复的丢弃，同 `NotebookStore`）。
  - `takeOver(id)`：答复之后从清单移出，返回接管的笔记本。
  - `remove(id)`：答复之后移出。
  - 两者答 404 `notebook.not_found`（别人已接管、删除，或已归还）也从清单移出，然后原样抛出：页面说明它已不是无主的，并重读。不当作成功：接管没有发生，页面不能说"已接管"。
  - 同一本的两个写经按 id 的 `oneAtATime` 一次一个（同 P4 审查 M1）。
- **`AuditStore`**（按工作区，`auditOf(workspace)`）：`events`、`nextCursor`。
  - `load()`（SWR 调用）：读第一页放在原处。第一页的最后一条在已有的事件里时，保留其后的事件与游标：窗口取得焦点等引起的重读不收起"加载更多"读出的页（P5 审查 Q2）；接不上或第一页已是最后一页时整体替换。几次重读重叠时最后发出的胜出。
  - `more()`：用 `nextCursor` 读下一页，按 id 去重接在后面；同一游标一次只走一个。答复时它的游标已不是 `nextCursor`（第一页重读换掉了已有的）就丢弃；两种先后、重读失败都对（P5 审查 M2）。
- **SWR 键**：`["ownerless", workspace.id]`、`["notebook-audit", workspace.id]`。
- **何时重读**：接管之后把答复的笔记本放进笔记本列表（`receive`，答复是权威的：角色与接管之后的成员数；`put` 计为写答复，重叠的读被丢弃）并重读审计；删除无主之后重读审计；接管、删除答 404 时重读无主清单、审计与工作区列表（见 3.3）；只给管理员的三处读（清单、审计、首页提醒）答 403 时重读工作区列表（`useFollowRole`）；工作区管理员移出成员之后重读无主清单（新的无主可能出现）。

### 3.3 无主笔记本页

- **路由**：`/:slug/settings/ownerless`，工作区设置导航的第三项"无主笔记本"，只给工作区管理员（成员与访客读清单答 403，不提供）。成员或访客直接打开：说明"只有工作区的管理员能查看无主笔记本"，不发请求。
- **清单**（`h2`"无主笔记本"）：每行是名称、开放程度、剩余成员数、原所有者（显示名与邮箱）、成为无主的日期、最后活动的日期、大小（`formatBytes`，M3 里如实为 0）。没有时说"没有无主笔记本"。列表读不到时 `NotLoaded`（13.2 第 7 条）。
  - **接管**：按钮直接发出（不确认：接管只给他加一个管理员的身份，可以再转交）；同一本一次只发一个（按钮在发出期间禁用）；接管、删除按钮的可访问名称带原所有者（同名的笔记本可以不止一本）；成功之后行离开，焦点到本节标题，`<output>` 说"已接管 {name}"并带"打开"的链接（到它的首页）。
  - **删除**：`ConfirmDialog`，输入笔记本名称确认（同删除笔记本，M3 总设计第 4 节），`focusAfter` 到本节标题。
  - 拒绝：404 `notebook.not_found` 在清单上方说"这本笔记本已经不是无主的了：已被接管、删除，或已归还原所有者"（`texts`），行随之离开。服务端对已不是工作区管理员的账户同样答 404（`lockOwnerless`），接管、删除不会答 403：404 时也重读审计（别人做的有记录）与工作区列表（角色变了，导航与页面随之变化，P5 审查 M1）。
- **审计记录**（`h2`"审计记录"）：新的在前；每条一句话加时刻：
  - 接管："{actor} 接管了 {notebook}（原所有者 {former}）"；
  - 删除："{actor} 删除了 {notebook}（原所有者 {former}）"；
  - 归还："{notebook} 归还给了回到工作区的 {former}"。
  有 `next_cursor` 时一个"加载更多"按钮；加载更多失败在按钮旁说明原因，按钮仍可重试；没有记录时说"还没有记录"。
- **首页提醒**：工作区管理员的首页在有无主笔记本时显示一行"没有管理员的笔记本：{count} 本"与到无主页的链接（同一个 SWR 键，与无主页共用一次读）；没有时不显示；成员与访客不读。

### 3.4 对话框与说明

- **离开工作区**：`notebook.sole_admin` 用自己的文案（`texts`）："你是这个工作区里某些还有其他成员的笔记本唯一的管理员。先在这些笔记本的设置里让另一位成员成为管理员，或者删除它们，再离开。"工作区自己的规则先答（`workspace.sole_admin`，P3 审查 Q2），两条都触发时先见工作区的文案。
- **停用**：`notebook.sole_admin` 的文案说明可能在多个工作区："你是某些还有其他成员的笔记本唯一的管理员。先在那些笔记本的设置里让另一位成员成为管理员，或者删除它们，再停用。"
- **移出成员**：说明加一句"对方独自管理的笔记本会成为无主，可以在'无主笔记本'里接管"：对移出者，即工作区管理员说。
- **笔记本成员页**：成员列表里没有管理员（笔记本无主，成员仍看得到它）时，列表上方说明"这本笔记本没有管理员"；工作区管理员另有到无主页的链接，其余的人读到"工作区的管理员可以接管它"。

### 3.5 文案

无主页、审计、首页提醒、三处对话框与成员页的说明；中英一起加（13.2 第 4 条）。

### 3.6 前端的测试（vitest）

- **store**：`OwnerlessStore` 的读与写（表格）：重叠的读丢弃、接管与删除移出、404 移出并抛出、同一本一次一个；`AuditStore`：第一页、`more` 接在后面并去重、重读回到第一页并丢弃在途的 `more`、最后一页没有游标。
- **缓存**：`ownerlessOf`、`auditOf` 按工作区（`root.store.test.ts`）。
- **页面**：
  - 无主页：清单的列、空、接管（焦点、状态与链接、左栏出现它）、删除（输入名称、焦点）、404 与 403 的拒绝、读不到与重读（13.2 第 7 条）；成员与访客看到说明、不发请求；设置导航只对管理员有第三项。
  - 审计：三种句子、加载更多、失败与重试、空。
  - 首页提醒：有、无、成员不读。
  - 对话框：离开工作区与停用的两种文案；移出的说明；笔记本成员页没有管理员时的说明。
- 反向对照：接管答 404 当作成功、`more` 不去重、重读不回到第一页、导航给成员第三项、首页提醒给成员读，各让对应的测试失败。

### 3.7 端到端

`fixtures/ownerless-pages.ts`：无主页的清单、接管、删除、审计的读取与加载更多，`<动作>With(page, …)` 返回请求的答复；断言与接口版本共用 `assert/notebook.ts`。

- N7：成员离开工作区被拒（还有别的成员的笔记本只有他一个管理员），对话框说明；让另一位成员成为管理员之后离开成功，他独自的私密笔记本出现在管理员的无主页。
- N8：工作区管理员在成员页移出唯一的管理员（说明里提到无主），无主页列出那一本、原所有者是他，读者照常读到它、成员页说明没有管理员；自助停用被规则二拒绝、对话框说明，单独成一个测试（自己的团队）。
- N9：管理员在无主页接管，左栏出现它，状态里的链接打开它；审计记录有这一条；成员没有第三项，直接打开看到说明。
- N10：管理员经"加载更多"读完审计（51 条，经接口删除，过服务端的缺省页长），再在页面上删除无主笔记本（输入名称）：新记录在前，已读的保留。
- N11：原所有者经邀请链接回到工作区，他的笔记本回到"我的笔记本"；管理员的无主页不再有它，审计记录有"归还"。
- N13：管理员在常规页删除有笔记本的工作区；笔记本、成员行与审计随之删除（落库断言）。
- 首页提醒：N8 的管理员在首页看到提醒并经链接进入无主页。
- 另一个账户的标签页用 `anotherPage(tokens)`：同样从第一次导航之前受监视，测试通过时要求安静，测试结束时关闭；管理员的页面用 `newOnboardedTeam`。

## 4. 实施步骤

分支 `m3-p5-web-ownerless`。

| Step | 内容 | 计划 |
|---|---|---|
| S1 | service、两个 store、缓存、`formatBytes` | [P5-S1](plans/P5-S1-data.md) |
| S2 | 无主页（清单、接管、删除、审计）、设置导航、路由、首页提醒 | [P5-S2](plans/P5-S2-ownerless-page.md) |
| S3 | 离开工作区、停用、移出的文案；笔记本成员页的说明 | [P5-S3](plans/P5-S3-dialogs.md) |
| S4 | 端到端：N7–N11、N13 的页面版本 | [P5-S4](plans/P5-S4-e2e.md) |

每个 Step 结束时 `make check` 为绿；S4 之后 `make e2e` 为绿。

## 5. 测试与验证

照 P4：vitest（3.6）、e2e（3.7）；反向对照见 3.6、各 Step 计划与审查记录。另按 13.2 第 15 条补了按工作区的页面测试：无主与审计，以及 P4 漏下的笔记本左栏。

## 6. 完成标准

- 第 2 节"做"的各项完成；`make check`、`make gen-check`、`make e2e`、`make image-smoke` 为绿，持续集成四个任务为绿。
- 审查记录 `reviews/P5-web-ownerless-review.md`；本文第 7 节、M3 总设计的进度表已更新。M3 的全部 Phase 完成之后做 M3 收尾审查。

## 7. 结果

- 分支 `m3-p5-web-ownerless`：S1 `e0c64ad`、S2 `9303fd0`、S3 `f45a873`、S4 `901af12`；审查修复 `9b1469e`；`219f127` 合并（`--no-ff`）。
- 门禁：每个 Step 的 `make check` 为绿；`make gen-check`、`make e2e`（117 个）、`make image-smoke` 为绿；持续集成四个任务为绿。
- 审查：[P5 审查](reviews/P5-web-ownerless-review.md)，2 项 Major（被降级的管理员接管、删除答 404；重读在途时"加载更多"留下缺口）、2 项 Minor、7 项 Nit 已处理或说明。

**与设计的偏差**（已同步进上文）：

1. 接管以答复把笔记本放进列表，不重读（3.2）。
2. `auditEvents` 不带 `limit`（3.2）。
3. 重读审计时接得上的尾部保留；`more` 的页按游标接上（3.2）。
4. 404 时重读审计与工作区列表；只给管理员的读答 403 时重读工作区列表（3.2、3.3）。
5. 接管、删除按钮的名称带原所有者（3.3）。
6. 移出的说明对移出者说；没有管理员时非管理员也有一句说明（3.4）。
7. `anotherPage`、`newOnboardedTeam`；N8 的停用单独成测试；N10 先读完再删除（3.7）。

**留给后面的**：`anotherPage` 给出它的 watch，等故事需要对第二个标签页声明控制台输出时再加（P5 审查 N7）；M3 收尾审查。
