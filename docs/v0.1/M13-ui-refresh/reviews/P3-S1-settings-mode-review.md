# M13/P3/S1 设置模式与个人设置：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m13-p3-s1`（从 `m13-ui-refresh` 的 `446d7e99` 拉出）：`7c5b7517`（设置模式、目录与"返回"，照 6.1 提交 1：三个设置路由的 `handle` 与 `IndexTo`，`Layout` 改成 `observer`、`bounced`、`useOriginRecord`，新文件 `settings-origin.ts`、`settings-directory.tsx`、`settings-page.tsx`（只有 `SettingsPage`、`useSettingsHeading`），`WorkspaceLayout` 的设置模式、`hidden` 的 `SlotHolder` 与 `ScrollKept`，变更集页、回收站的预览与邀请页的 `h1` 取到达的焦点，令牌页去掉 `h2`，`NavItem` 当前一行的透明边框；5.2 的 8 个故事与照实加的 tb2、a8 与 a15 的提交 1 部分、夹具四个帮手，UI2、`threeBrowsers`、`REQUIRED`；52 个文件）、`276c888a`（卡片、`NameRow`、外观与语言，照 6.1 提交 2：`SettingsSection`、`SettingsCard`、`SettingsRow`、`SavedOutput` 与一行的类名常量，`name-row.tsx`，`.nw-saved`，个人资料、安全、令牌三页换成卡片，新的外观与语言页与它的路由，`DeactivateDialog` 改 `destructive sm pill`，`Button` 的 `default` 加透明边框，`create-token-dialog.tsx:184` 改 `space-y-1.5`；24 个文件）、`2979f1c2`（审查的修复：变更集页读到之后才挂载的 `ChangesetRead` 拿着 `h1` 与到达的焦点，`SettingsRow` 是 `observer`（决定 849），令牌页说明与"Create token"那一行，审查指出的测试缺口；9 个文件）、`6987baa9`（修复核对第 1 轮之后的修复：滚动一格的桩像浏览器那样在值变了时发 `scroll`，令牌行的创建与到期各自不折行；2 个文件）；设计 [03-P3](../03-P3-settings.md) 4.1、5.1–5.3、6.1 |
| 方法 | 实施者一位（子 agent，在自己的 worktree 里）；三位审查者并行，各在自己的 detached worktree 里构建、跑、改坏代码（02-workflow 3.2；00-M13 4.8 的分工）：A 管行为（来处与"返回"的各种进入法、左栏的滚动、每条路由的账户按钮、引导中的弹回、设置模式里的 Mod+K 与 Mod+Shift+F、375 跨外框、`NameRow`、外观页，Playwright 探针在三种引擎里各跑 1280 与 375，vitest 探针看变更集页的到达，只跑了提交 1 的 web 门禁，另写 10 条变异）；B 管外观（`make build` 之后以带 `ui2-` 前缀的临时故事在三种引擎里截图，对照 `mockups/settings.html` 方案 A 与 4.1.4、4.1.6–4.1.10、7.4 的 S1、决定 817、820–824、841、844、845、848：亮暗 × 1440、375，另有 320、800–1000 每 10 px、强制色、减少动效，逐项量位置、大小与颜色；造数用名字很长的中文工作区与笔记本、三个令牌（一个已过期）；令牌页一行的三种写法另以静态页比）；C 管测试、变异、门禁与完成度（独立重跑提交 1 的 `lint-web knip test-web` 与 `make e2e`、`276c888a` 上合并之前的整条命令，重跑实施者的变异，自写变异找漏洞，逐条看 7.1 的格、e2e 与工具，核对 10.1 与"照实"的草稿，重跑 8.2 第 1–11 条的替换核对，列出文档提交要带的）。重的运行都经 `heavy.sh` |
| 结论 | 实施中：提交 1 的第一次 `make e2e` 另红 tb2:274（5.2 漏列）、UI2 第 2 格的读取计数（事件流连上时的刷新）与 WebKit 里同外框链接的焦点；提交 2 的第一次 `make e2e` 红在 a8:79，是外观页的真 bug（`SettingsRow` 的函数子元素不受观察），先在页里先读绕开，后由决定 849 改成 `SettingsRow` 是 `observer`。第一轮：A 1 Minor、2 Nit、1 Q（另一条未核实的怀疑）；B 1 Minor、2 Nit、1 Q；C 1 Minor、4 Nit；没有 Important。A 的 N1 与 C 的 M1 是同一处。主持者的决定 849–851 答了 A 的 Q1、B 的 N2 与 Q1；B 的 N1 接受、照记。修复在 `2979f1c2`，文档的几条随本文档提交。修复核对第 1 轮：A 没有新的；B 1 Nit（令牌行的日期在窄屏上折在中间），C 1 Minor（滚动一格的桩不发 `scroll`，`ScrollKept` 监听的生命周期没人守），都修在 `6987baa9`；B 的 Nit 照 M12 的规则由主持者核对（重截）。修复核对第 2 轮：C 核对 C1-M1 修好，没有新的发现；没有未结的 Important、Minor |

## 基线

S1 不另跑基线（决定 827）：P2 最后一次门禁（S3 合并之前的 `bb00d33f`）的代码与 `ba66557c` 相同，第一个改行为的提交（`7c5b7517`）的 `make e2e` 兼作核对。对照的数是 03-P3 第 1 节的：vitest 216 个文件、5237 格；E2E 609 过、3 败、1 跳过。已知的本机失败同 [P2/S3 的记录](P2-S3-search-drawer-review.md)：

| 格 | 错误所在的行（格的开头行） | 消息的第一行 |
|---|---|---|
| SE2 [chromium] | `e2e/stories/search/se2-search-visibility.spec.ts:231`（:188） | `Error: expect(locator).toHaveValue(expected) failed` |
| PG15 [webkit] | `e2e/stories/page/pg15-reading-updates.spec.ts:113`（:75） | `Error: expect(received).toEqual(expected) // deep equality` |
| LW8 [webkit] | `e2e/stories/wiki/lw8-health.spec.ts:836`（:822） | `Error: expect(locator).toBeFocused() failed` |

S1 的 `make e2e`（实施者与审查 C 的）除了下面几次都只败这三格或其中两格，同行同消息：PG15 只在实施者 `6987baa9` 上合并之前的整条命令里过了一次（619 过、2 败）。另有的红：提交 1 的第一次 `make e2e` 与提交 2 的第一次 `make e2e`（都在提交之前，见"实施中的发现"）；审查 C 在 `2979f1c2` 上合并之前的整条命令第一次另败两格 WebKit，UI2 `:95` 与 LW10 `:23`，造数的 API 调用答 500，服务端日志两条同在 01:44:38，都是 `commit transaction: timeout: context deadline exceeded`，负载下的数据库，页面还没打开就失败，与这个只改 `web` 的提交无关；重跑 `make e2e` 只败基线三格（门禁一节）。偶发的 AS6 `as6:153`、LW8 `lw8:771`、`markdown.test.ts` 的"not slower"与 `trash-preview.test.tsx:587` 都没有出现，也没有 flaky。UI2 三格在三种浏览器里共 9 次，`make e2e` 由 609 过变成 618 过。

## 实施中的发现

| 发现 | 处置与提交 |
|---|---|
| 提交 1 的第一次 `CI=1 make e2e`（613 过、8 败、1 跳）红在 `e2e/stories/trash/tb2-background-restore.spec.ts:274`（[chromium]）：`treeTitles` 等到的是 `[]`，应为 `["Index", "Big"]`。它从回收站状态行的"See its progress"进入导入导出页，然后在那里读树；设置模式里树在 `hidden` 里。5.2 漏列了它 | 照 hi5 改：读树之前 `settingsBack(page, "Plans")`，等回收站的 `h1` 拿着焦点（`7c5b7517`）；照实写进 4.1.1 与 5.2 的表（"9 个"，审查 C 的 N2） |
| 同一次：UI2 第 2 格读列表的计数在 Chromium、Firefox 里是 2，不是 1。事件流连上时 `refreshedOnConnect` 含 `["workspaces"]`，在目录那一次读还在途时再读一次列表（P3 之前就有的行为） | 计数之前 `holdStream`、计数之后放开，计的是目录自己的那一次，为 1（`7c5b7517`；5.3 的照实）。审查 C：有意义，不是 S1 的回退 |
| 同一次：WebKit 里 UI2 第 1 格（"Members"）与第 2 格（"General"）同外框链接的 `toBeFocused` 红：WebKit 点按链接不给它焦点（同 Safari），焦点留在 `body` | 第 1、2 格的同外框链接先 `focus()` 再按 Enter（`7c5b7517`；5.3 的照实）。审查 C：要核对的是 `opened` 不取焦点，这样写没有放过什么。改了之后 tb2 与 UI2 单独跑 11 格都过，整条重跑只败基线三格 |
| 提交 1 提交之前，`make lint-web` 红在 `e2e/stories/wiki/lw12-downgrade.spec.ts:230` 的 `no-await-in-loop`（oxlint 零警告） | writer 标签页回到笔记本首页之后，它的树里"Directory""History"两行的 `toHaveCount(0)` 以 `Promise.all` 一起等（`7c5b7517`） |
| 提交 2 的第一次门禁卡在 knip：`rowValueClass`（`W/app/settings-page.tsx:90`）导出了却没人用 | 不导出（S1 只有 `SettingsRow` 用，S2 要用时再导出；4.1.7 的照实，`276c888a`） |
| 提交 2 的第一次 `make e2e`（617 过、4 败）红在 a8:79 [chromium]，`locator.check: Clicking the checkbox did not change its state`：外观页点"Dark"之后主题换了、单选不跟着变。`SettingsRow` 的 `children` 是在它自己的渲染里调用的函数，`SettingsRow` 不是 `observer`，函数里读到的 store 没有人跟踪（页的 `observer` 只跟踪页自己渲染时读的）；vitest 那一格只在换语言、整页重画之后才看单选，抓不住 | 先绕开：外观页在自己的渲染里先取 `const { theme, locale } = preferences`，`SettingsRow` 的注释写明；`appearance-page.test.tsx` 加"选了即显示"两句（先红后绿），变异 t14b 守（`276c888a`）。审查 A 的 Q1 指出 S2 会再踩，由决定 849 换成 `SettingsRow` 是 `observer`、外观页在 `children` 里读，变异 67（`2979f1c2`） |
| 849 一格（`settings-page.test.tsx`）里读出主题的元素：oxlint 的 `prefer-tag-over-role` | 用 `<output aria-labelledby>`（格以 `status` 的角色读它，`2979f1c2`） |
| 实施者自己截 7.4 的 S1 几张时看到：令牌页在 696 宽时说明（`max-w-xl`，576 px）加间距再加"Create token"放不下一行，按钮折到说明之下靠左（1440 与 375 都是），类名是 4.1.10 原样 | 交审查 B：P3S1B-M1 |

## 第一轮的发现与处置

### 审查 A（行为）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P3S1A-M1 | Minor | 变更集页的数据还没读到时到达，`h1` 不取焦点（决定 835 只在有缓存时成立）：`useArrivalFocus` 在 `ChangesetShell` 里（`W/pages/notebook/changeset-page.tsx:101`），它的 effect 只在挂载与 `[arrival, fromNoOne]` 变时跑；挂载时数据未到、返回 `NotLoaded`（`:122-123`），`h1`（`:216-217`）还没画，`takeArrivalFocus` 把这次导航记为已提供，数据到了不再跑，焦点留在 `body`（vitest 探针：没读过的变更集以 `arrived` 进入，焦点在 `body`；已在 SWR 缓存里的，在 `h1`）。hi5 与 `settings-directory.test.tsx` 的变更集格都先看过这页再返回，所以绿；S2 的 839 给任务行的"See in the history""The change undone"加上 `arrived` 之后会常见 | `ChangesetShell` 只读（`useSWR`、标签页标题、`NotFoundPage`、`NotLoaded`），读到之后才挂载 `ChangesetRead`，`h1` 与 `useArrivalFocus` 在里面，`heading` 交给 `RevertDialog` 与 `ChangesetChanges`；`changeset-page.test.tsx` 加"读到之前到达，读到之后 `h1` 取焦点"一格（`276c888a` 的代码上红，焦点在 `body`）；变异 65（`2979f1c2`） |
| P3S1A-N1 | Nit | `name-row.test.tsx:113-117` 的"扣住时再回车、再点都不发"抓不住去掉 `NameRow` 的 `sending` 守卫（`W/app/name-row.tsx:81`；A 的 m4，19 格全绿）：`useForm` 的 `submit` 不拦并发（`W/app/form.ts:55-74`），账户 store 的 `oneAtATime` 把后来的 `PATCH` 排在第一个答复之后，扣住期间只数得到一个；没有守卫时放开之后会再发同样的 `PATCH`。同审查 C 的 P3S1C-M1 | 在 `server.release()` 之后 `await settle(); expect(server.sent).toHaveLength(1)`；变异 64（= A 的 m4、C 的 c7）在 `name-row.test.tsx:129:23` 红，got 3（`2979f1c2`） |
| P3S1A-N2 | Nit | `useBack` 里"列表还没读到"的分支（`W/app/settings-directory.tsx:101`）没有格：A 的 m2 把它改成一律 `/`，`settings-origin`、`settings-directory` 两个文件 29 格全绿。走得到：工作区列表读取失败时，从右上角的账户菜单进入个人设置，"返回"应回原来那一页 | `settings-origin.test.tsx` 加一格：列表答 503，经右上角的账户菜单进入，"Back"回 `/lab`；变异 66（= A 的 m2）红，`['Back', '/']` 不等于 `['Back', '/lab']`（`2979f1c2`） |
| P3S1A-Q1 | Q | `SettingsRow` 的 `children` 在一个不被观察的渲染里调用（`W/app/settings-page.tsx:106,109,143`）：外观页的 bug 靠"页在自己的渲染里先读 store"绕开（`W/pages/settings/appearance-page.tsx:18`）；S2 的开放程度一行若在 `children` 里读笔记本列表会再踩，jsdom 的格抓不住。请主持者在"`SettingsRow` 包成 `observer`"与"`children` 直接传元素、ids 另经 context"之间定 | 决定 849：`SettingsRow` 是 `observer`，外观页改为在 `children` 里读；`settings-page.test.tsx` 加"一行的控件跟着它读的 store 变"一格（`276c888a` 的代码上红，'Light' 应为 'Dark'）；变异 67（`2979f1c2`） |
| 未核实的怀疑 | — | `ScrollKept` 必须排在中段最后（`W/pages/workspace/workspace-layout.tsx:197`）只有注释守着：A 把它挪到 `nav` 之前，vitest 14 格全绿，浏览器探针（Chromium、WebKit）里返回之后仍是 618；可能是那本笔记本的树太矮，夹断与树的 reveal 碰巧得到同一个值 | 实施者的探针证实约束是真的：60 页的树、Chromium 1280×720、键盘按齿轮进出设置，头上返回之后 1445；挪到 `nav` 之前，放回的 1445 被夹成 0（笔记本的容器还没回到左栏），树的 reveal 再停在 1372。指针点齿轮时 Playwright 先把那一行滚进视野、左栏滚到 0，记下的也是 0（真实行为）。注释照旧；滚动一格的桩改为看不见树时设成 0（夹断的模型），变异 68 守（`2979f1c2`；A 的同一条在修复核对时红） |

A 另核实：提交 1 单独为绿（`heavy.sh make lint-web knip test-web` 219 个文件、5280 格），没有用户碰得到的半成品（个人设置照旧是旧内容，目录的个人一组三项，令牌页没有 `h2`，没有东西指向还不存在的 `/settings/appearance`）；来处（三种引擎，1280）：带片段的页返回时不带片段、`h1` 拿着焦点，`?q=notes` 的搜索返回"Back to Acme"、回到带查询的地址、焦点在"Search “notes”"，邀请页是不带名字的"Back"、`href` 带着令牌的片段，直接打开笔记本设置是"Back to Acme"、回工作区首页；读代码：833 以 `REPLACE` 进入不记，`Origin.workspace` 取上一次渲染的 `slug`（m1 被 4 格抓住），设置之间保留，POP 回到设置时来处是刚离开的那一页；`ScrollKept` 三种都是 618 → 0 → 618、当前一行在视野内（m7 被抓住）；直接打开笔记本设置，树在 `hidden` 里，没有控制台报错；9 条路由（个人资料、外观、工作区首页、工作区设置、笔记本成员设置、新建工作区、三种 404）各 1 个账户按钮，引导中的账户打开 `/settings/profile` 被送到 `/onboarding?next=%2Fsettings%2Fprofile`、菜单按钮 1 个（832），列表读取中 `/settings/security` 是左栏底部的 1 个，工作区页与工作区设置页显示"Loading…"、0 个，同 P2；设置模式里 Mod+K 与 Mod+Shift+F 不被 `preventDefault`，工作区模式里照旧拦下、聚焦搜索框（819）；375 跨外框（三种）：从工作区的抽屉经账户菜单进个人资料、从个人设置的抽屉点工作区一组的 General 再点个人一组的 Security、点"Back to Plans"回到 Notes 页，`h1` 都拿着焦点，抽屉都 `inert`，`html` 的 `overflow` 都还原，细栏写"Settings"；同外框的链接经 `useFollow`，跨外框的、"返回"、齿轮、导航的"设置"、账户菜单都带 `arrived`（m3 被抓住），`NavItem` 转交 `state` 与 `onClick`，`IndexTo` 转交路由状态；`NameRow`（三种）：失焦不发，扣住时"保存"是 `aria-disabled` 与 `aria-busy`，在途时再编辑草稿保留、答复之后"保存"还在、没有"已保存。"，`data-round` 依次 0、1、0，清空后回车 `aria-invalid`、`aria-describedby` 指向问题，打回原样之后都消失，用指针点"保存"Chromium、Firefox 焦点回输入框、WebKit 落到 `body`（WebKit 点按钮不给焦点，同应用里别的按钮）（m5、m6、m15 被抓住）；外观页（三种）单选点了立即选中、`html` 立即换主题，中文 `h1`"外观与语言"（m16 被抓住）；三页的到达焦点、`SlotHolder` 的守卫与 `hidden` 的容器、`Layout` 渲染时 `setState` 只改自己（读代码）。

### 审查 B（外观）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P3S1B-M1 | Minor | 令牌页的"Create token"在英文界面里任何宽度都折到说明之下、靠左（实施者交来的那一处）：那一行是 `flex flex-wrap items-center justify-between gap-x-6 gap-y-3 px-0.5`，`p` 的 flex-basis 是 auto、被 `max-w-xl` 限在 576 px，加 24 px 间距与 107 px 的按钮共 707 px，比 692 px 的行宽多；中文 1024 宽以上在同一行、900 以下折下去；三种引擎相同。单加 `min-w-0 flex-1` 不对：basis 变成 0，375、320 宽也不折行，说明被挤到 192 / 137 px、4 到 10 行 | 说明的 `p` 加 `min-w-0 flex-1 basis-56`（写法同 `rowTextClass`；`2979f1c2`）；4.1.10 的类随之。实施者重截（中英文、亮暗 × 1440、768、375、320）：英文 1440 说明 561 px、按钮同一行靠右（x=1085），768 同一行，375、320 折到说明之下靠左，没有横向溢出；B 的修复核对三种引擎同 |
| P3S1B-N1 | Nit | 名称一行出了问题时有焦点的输入框上移 6 px（y 132 → 126），"保存"同移，行高 73 → 88：`rowClass` 是 `items-center`，问题自成一行之后控件区变高；横向不动（三种都在 867）。7.4 的"输入框在问题出现时不动"在竖直方向不成立 | 接受，照记（主持者）：4.1.8 写明"出了问题时控件区变高，行是 `items-center`，输入框与'保存'上移约 6 px，行高 73 → 88，横向不动"（本文档提交）。不为 6 px 改 `rowClass` 或让问题不占流（前者影响所有行，后者问题会压到下一行；B 也不建议） |
| P3S1B-N2 | Nit | 令牌行的名字加了 `text-sm`（14 px，原来随正文 16 px）：与样稿 `.set .t` 的 14 px、500 一致，但与 4.1.7"左边的文字块照旧"不符、没有记下；S2 的成员、邀请、无主笔记本、任务行照 4.1.7 是 16 px（`member-summary.tsx`），设置里会有两种列表名字的字号 | 决定 850：设置卡片里列表的名字都是 `text-sm`（14 px，样稿的 `.set .t`），S2 的列表随之；4.1.10 照实记下（本文档提交）。B 在修复核对时补充：`MemberSummary` 只在设置里用（成员行、无主行、审计、添加成员），改它只影响设置；邀请行与任务行不经它，S2 要各自改 |
| P3S1B-Q1 | Q | 个人设置三页（个人资料、外观与语言、令牌）的第一张卡片上没有分组标题，卡片在 `h1` 下 28 px（1440 宽卡片上沿 111；样稿是 `h1` → 分组标题 → 卡片，上沿 140.5）；决定 844 正是以此给两个常规页加了"基本信息"，安全页有"Password""Danger zone"两个标题，所以设置模式里有的页有、有的没有；7.4 的有意偏离没有列。是有意的吗 | 决定 851（照 B 在修复核对时的改写）：第一组的标题只会重复页名时不写（个人资料、外观与语言、访问令牌；S2 的成员页与无主笔记本页），否则照写（两个常规页的"基本信息"，决定 844）；7.4 的有意偏离加这一句（本文档提交）。不改代码 |

B 另核对过、没有问题的：类名与设计（03-P3 `446d7e99`）逐字相同（`SettingsPage` 与 `h1` 那串、`SettingsSection`、`SettingsCard`、一行的类名常量含 `rowProblemClass` 的 `md:-me-2`、`SavedOutput`、`NameRow` 的输入框与槽、问题在调用处的 `md:-me-21 md:ps-19`（tailwind-merge 换掉了 `md:-me-2`）、`SettingsTop`、`GroupTitle`、三个 `nav`、`NavItem` 当前一行、`Button` 的 `default` 加 `border border-transparent`、"Create token""Revoke""Deactivate account"都是 `sm pill`、后者 `destructive`、外观页的单选组、安全页的 `px-4.5 py-4`、`.nw-saved` 的 CSS）；`design-tokens.test.ts` 与 `button.test.tsx` 在 `276c888a` 上 12 格全过，新 CSS 不中琥珀底的规则；目录（三种相同）："返回"一行 (8, 10, 235×30) 与样稿逐像素相同，"Settings" 18 px、650，组标题 12 px、高 34（样稿 36），三组上沿 92 / 254 / 384（样稿 89 / 255 / 389，设计审查已接受的 5 px 以内），行 30 px、间距 32 px，图标都在 x=16（当前一行也是），长名字的组标题一行、名字省略；名称一行（1440 宽，三种）：四态输入框都在 867–1107，"保存" 58 px、离卡片右边 19 px，"已保存。"预留 791–859、字靠右，问题的字从 867 起，说明列 250 px；800–1000 宽每 10 px 编辑中与已保存的行高、输入框位置都相同（决定 848 在真页面上成立）；320 与 375 宽个人设置四页、工作区与笔记本设置六页与抽屉的 `scrollWidth` 都等于视口宽，抽屉 252 px、"返回"省略；强制色（Chromium、Firefox）下目录当前一行 1 px 系统色边框、别的行 0，"Save""Create token""Change password"有系统色边框，✓ 是系统文字色、13 px，"返回"是链接色；淡出：减少动效与否，三种引擎都是写出时 1、0.8 s 时 1、2.1 s 时 0，淡到一半（1.6 s，约 0.8）再存一次，0.3 s 后回到 1，`data-round` 由 0 变 1；亮暗 × 1440、375 四页与抽屉与样稿相符，暗色卡片 `--surface`、"保存" 30 px 是设计记下的偏离；S1 期间六页旧内容在新外框里、712 px 的列里，长的中文笔记本名在 375 宽折三行、不溢出，没有坏掉的地方（成员页添加表单在窄屏上"Role"挤在下拉左边是今天就有的，S2 改）。

### 审查 C（测试、变异、门禁）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P3S1C-M1 | Minor | `NameRow` 发送中的守卫没有测试守着，7.1 的"扣住时……再点、再回车都不发（变异 22 的抓手）"那一句是空的（同 A 的 N1，从测试一侧）：C 的 c7（去掉 `sending ||`）`name-row.test.tsx` 与 `profile-page.test.tsx` 19 格全过；在那一格放开之后加 `await settle(); expect(server.sent).toHaveLength(1)`，头上绿、c7 下红，got 3：回车与再点各排了一个 `PATCH`，放开之后都发出去，`aria-busy` 也在第一次的 `finally` 里提前消失 | 同 P3S1A-N1（`2979f1c2`）；7.2 加 64，10.1 的变异随之；S2 的名称经别的 store，写那几格时同样放开之后再数（交接） |
| P3S1C-N1 | Nit | 照实 7.1 的"`workspace-layout.test.tsx` 的 Mod+K 一格也核对……离开设置之后 `/acme` 上照旧拦下"与测试不符：那一格（`:144-156`）只在进入设置之前在 `/acme` 上核对（`:148-149`）；离开之后能否再拦取决于搜索框重挂时的 `field` ref，没有格 | 那一格在"返回"之后再按一次 Mod+K，断言拦下、焦点到搜索框（`2979f1c2`）；照实改为"进入设置之前与返回之后两处"。C 的修复核对：快捷键在按键时读 `field.current`，这一步守着重挂的搜索框的 ref；没有配变异 |
| P3S1C-N2 | Nit | 10.1 草稿"与设计不同之处：见 4.1 的照实"指得不全：照实另在 5.2（tb2、a8 的注释）、5.3（`focus()` 加 Enter、`holdStream`）、7.1、7.2；tb2 加进来之后 5.2 的表少一行，表末的"8 个"不对 | 本文档提交：10.1 列全各节；5.2 的表加 tb2 一行（`tb2:274` 读树之前 `settingsBack(page, "Plans")`，回收站的 `h1` 拿着焦点），表末"9 个"，注明是照实加的 |
| P3S1C-N3 | Nit | 01-P1 第 9 节记 `create-token-dialog.tsx:184` 时要写明是哪一版的行号：改过的 `space-y-1.5` 在头上是 `:186`，`:184` 是 `ba66557c`、`446d7e99` 的（提交 2 在上面加了两行） | 本文档提交：01-P1 第 9 节写 `:184` 与它的版本 |
| P3S1C-N4 | Nit | 确认 7.2 第 8 行的 `user-menu.test.tsx` 要去掉：C 重跑 8，`layout.test.tsx` 1 格红；只跑 `user-menu.test.tsx` 13 格全过（7.1 已把那两格的地址换成 `/create-workspace`） | 本文档提交：7.2 第 8 行只写 `layout.test.tsx` |

C 另核实：`7c5b7517` 的 `lint-web knip test-web` 与 `make e2e` 实跑为绿（只败基线三格），`276c888a` 由合并之前的整条命令覆盖；重跑实施者的 2、3、8、10、11、14、16、18、21、22、46、52、54、55、56、62 都抓住、与他的结果同格，30 与 t11 照他说的抓不住；7.1 的格逐条看过，全部在、写法有意义（`settings-origin.test.tsx` 9 格，`settings-directory.test.tsx` 20 格，读取计数先 `advanceTimersByTimeAsync(2_000)`，`IndexTo` 两态都断言，`null` 的等 50 ms 断言不取焦点；`settings-page.test.tsx` 的轮次 `0 1 1 0 0 0 1 1`；模拟 `inert` 的一格用 `vi.spyOn`，`restoreMocks` 复原；`workspace-layout.test.tsx` 滚动一格 `sets[0]` 是 300、最后是 738；12 个文件的改法照表逐行对过）；jsdom 的限制与 SWR 的去重照设计处理，`renderApp` 不用 StrictMode，`useOriginRecord` 渲染时 `setState` 是随 props 调整状态的写法，重复执行幂等；tb2:274 的改法对，别的故事没有再从内容进入设置、在设置页上读左栏的；UI2 第 3 格经到达进入之后再选一次是 55 的 e2e 抓手；`check.mjs` 的 `REQUIRED` 两行与 `threeBrowsers` 的 `shell\/ui2-`；knip（`export type Origin` 是导出的 hook 的返回类型，不报）；`SavedOutput` 的类名合决定 848（`W/app/settings-page.tsx:167`），问题的 `md:-me-21 md:ps-19` 合 4.1.8；`Layout` 里 `useMatches()` 无条件调用（4.3）；以修好的 `check82-head.py` 在头上重跑 8.2 第 1–11 条：旧文字各一次，新文字在它那一步之后各一次（2、5、7 的新文字由 S2 改写，属预期），`v0.1-design.md` 自 `ba66557c` 没有改动；10.1 草稿的数与 C 的实跑对得上。

## 修复核对第 1 轮

在 `2979f1c2` 上，三位各在自己的 detached worktree 里（经 `heavy.sh`）。

- **A**：M1、N1、N2、Q1（849）与 `ScrollKept` 的怀疑都已处理，没有新的发现。M1（三种）：离开时没读完、"返回"之后才读到，`h1` 拿着焦点（Chromium 另重复 3 次）；暂把 839 加到任务行的"See in the history"上（只在 A 的工作树里），点它进入一个从没读过的变更集，`h1` 拿着焦点；首次读取答 503，点"Try again"之后读到，`h1` 拿着焦点；撤销对话框 Esc 之后焦点回到"Undo change"，确认答 200 之后是"Undone. See the undo"、焦点在 `h1`。A 的 m2、m4 与挪 `ScrollKept` 的那一条现在都被抓住；去掉 `observer` 时 `settings-page.test.tsx` 的新格与 `appearance-page.test.tsx` 的"选了即生效"都红；外观页三种里选了即选中、即生效；上一轮的全部探针结果相同，浏览器里仍是 618 → 0 → 618。读代码：读取、标签页标题、"Page not found"与 `NotLoaded` 留在外层，撤销的状态（`said`、`attempt`、`shown`、`refused`）、`findOut` 与 `findStarted` 原样进了 `ChangesetRead`，SWR 重新读取时 `data` 不会变成 `undefined`，`ChangesetRead` 不因此重挂。
- **B**：M1 已处理：三种引擎、中英、亮暗，1440、768 说明在按钮左边，375、320 在按钮之上、占满宽，`scrollWidth` 等于窗口宽；1440 宽时行离卡片 10 px、按钮离卡片右边 2 px。N1 的接受没有意见；850 补充 `MemberSummary` 的用处（上表）；851：主持者转述的"多于一组的页才有组标题"与 S2 的设计（4.2.3、4.2.4）不合，工作区成员页、笔记本成员页、无主笔记本页都是多组、第一组没有标题（标题会与 `h1` 重复，4.2.2 去掉的就是它）、其后有，建议改写成"第一组的标题会重复页名时不写，否则照写"，决定 851 照它改写。变更集页读取中、失败、未找到、读到四态的 `main` DOM 在 `276c888a` 与 `2979f1c2` 之间相同（jsdom，React 的 id 归一）；外观页选"暗"之后单选即时跟着选中（三种）；`SettingsRow` 改成 `observer`、外观页挪到子函数里读，标记不变。新的 P3S1B1-N1。
- **C**：M1、N1 已处理（64 即 c7 实跑红，got 3，头上绿）；N2 在实施者修复报告的草稿里已处理，要等文档提交落实；N3、N4 实施者的修复报告没提，交主持者的文档提交。64–68 都红；先红：把 `276c888a` 的 `changeset-page.tsx`、`settings-page.tsx` 换回去跑两个新格，都红（焦点在 `body`；'Light' 应为 'Dark'）；67 的两格确实靠 `observer`（`settings-page.test.tsx` 的新格单独渲染 `SettingsRow`，外面没有别的 observer）。合并之前的整条命令第一次另败 UI2 `:95` 与 LW10 `:23` [webkit]，查清是基础设施（基线一节、门禁一节），重跑 `make e2e` 只败基线三格。新的 P3S1C1-M1。

新的发现：

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P3S1B1-N1 | Nit | 令牌行的第二行在 375 宽时折在日期中间（早于 M13，修复没碰）：英文"Expires Dec 24, / 2026"，中文"2026年12月24日 到 / 期""11月22 / 日 过期"，320 宽中文还折在"2026年12 / 月24日"。B 的改法：两半（创建、到期）各放进 `inline-block` 或 `whitespace-nowrap` 的 span，只在" · "处折；最长的一半约 180 px，320 宽的 268 px 放得下 | `W/pages/settings/token-row.tsx` 的创建、到期各在一个 `whitespace-nowrap` 的 span，中间 `" · "`（`6987baa9`）。实施者重截 `tokens-{375,320}-{light,dark}{,-zh}.png`：两种宽度、中英文，第二行只在 `·` 之后折，每一半一个盒子，各半 139–169 px，行宽 375 时 289、320 时 234，没有横向溢出 |
| P3S1C1-M1 | Minor | 滚动一格的桩（`workspace-layout.test.tsx:183-185`）不发 `scroll`，`ScrollKept` 监听的生命周期（`workspace-layout.tsx:229`、`:237`；注释 `:211-219` 靠的正是浏览器在 `scrollTop` 变了之后异步发的 `scroll`）没人守。夹断的桩对先后次序是忠实的（68 在它之下红，438 ≠ 738），但两个真实的回退在头的桩下都绿：c12 去掉 `:237` 的清理（进入设置时旧的 `keep` 还挂着，设置把左栏设成 0 之后的 `scroll` 把 `at` 写成 0，返回就回到 0，每进出一次还多挂一个监听）；c13 设置模式里也监听（结果同上） | 桩照 C 的写法：记下夹断后的值，值变了以 `setTimeout(…, 0)` 发 `scroll`，注释补一句（`6987baa9`）；7.2 加 69（不移掉监听，= c12）、69b（设置模式里也监听，= c13），都在 `:221:19` 红（+0 ≠ 300），旧桩下都绿；68 照旧红（`:222:26`，438 ≠ 738）。由 C 在修复核对第 2 轮核对 |

P3S1B1-N1 由主持者核对（照 M12 的规则）：读了 `6987baa9` 在 `token-row.tsx` 的改动（两半各一个 `whitespace-nowrap` 的 span，中间 `" · "`），对照 B 的复现与改法；看了实施者的重截（375、320，亮暗，中英文）与他量的值（上表），第二行只在 `·` 之后折、没有横向溢出。C 的 N3、N4 与 N2 的落实随本文档提交。

## 修复核对第 2 轮

在 `6987baa9` 上，审查 C 在自己的分离工作树里（经 `heavy.sh`）。

- **C**：P3S1C1-M1 修好。
  - 滚动一格的桩记下夹断后的值，值变了时以 `setTimeout(…, 0)` 发 `scroll`（`workspace-layout.test.tsx:187-192`），同 C 给的写法。头上绿；68b、69、69b 红；换回旧桩时 69、69b 绿，与实施者的结果同格。
  - `6987baa9` 单独为绿：`heavy.sh make lint-web knip test-web`，221 个文件、5296 格；`git diff --check` 与受保护路径的 `git diff --quiet` 以 0 退出。只改了一格测试与令牌行的两个 `span`，没有重跑全量 e2e（实施者在它上面的合并之前的整条命令是 619 / 2 / 1）。
  - 异步的 `scroll` 不靠时序上的巧合（`probe-order.py`）：userEvent 设 `delay: null`、进入设置之后另等 20 ms，头上照样绿，69、69b 照样红；整个文件连跑 12 次、打乱次序（`--sequence.shuffle`，种子 1–6）跑 6 次都全过。这一格结束后还没触发的定时器落在已卸下的元素上，监听已移掉，没有状态更新，不漏到后面的格；同文件用假定时器的那一格由 `afterEach(() => vi.useRealTimers())` 复原。
  - `renderApp` 不用 StrictMode；若用，`ScrollKept` 的布局 effect 成对执行，第二次设同一个值，桩不发事件，`sets[0]` 仍是 300（读代码，没有实跑）。
  - `token-row.tsx` 的两个 `span` 之间留 `{" "}`，`textContent` 不变，`tokens-page.test` 全过；样子由主持者看重截。

新的发现：没有。C 的 N3、N4 随本文档提交。修复核对到此为止，没有未结的 Important、Minor。

## 主持者的决定（负责人可以改判）

全文在 [03-P3](../03-P3-settings.md) 2.3。

- **849 `SettingsRow` 是 `observer`**（审查 A 的 P3S1A-Q1）：`children` 在它自己的渲染里调用，读到的 store 受它跟踪；外观页改为在 `children` 里读，S2 的开放程度一行同样可以在 `children` 里读列表。取 A 给的两个做法里的前者（另一个是 `children` 直接传元素、ids 另经 context）。
- **850 设置卡片里列表的名字是 `text-sm`**（审查 B 的 P3S1B-N2）：14 px，样稿的 `.set .t`；令牌行已是，S2 的成员、邀请、无主笔记本与任务行随之。`MemberSummary` 只在设置里用，改它只影响设置；邀请行与任务行不经它，S2 各自改（B 的修复核对）。
- **851 第一组的标题只会重复页名时不写**（审查 B 的 P3S1B-Q1；照 B 修复核对时的改写）：个人资料、外观与语言、访问令牌，S2 的成员页与无主笔记本页，第一组不写标题（会与 `h1` 重复，4.2.2 去掉的就是它）；其余照写（两个常规页的"基本信息"，决定 844）。先前的说法"一组一卡的页不加分组标题"（转述作"多于一组的页才有组标题"）与 S2 的设计（4.2.3、4.2.4）不合：成员页与无主笔记本页是多组、第一组也没有标题。

## 变异

每条命令先在没改的树上跑过、全绿；改坏前备份，跑完从备份复原，核对 `git status` 干净（02-workflow 3.1）；改坏期间不跑 `make gen*`、不用 `git checkout --`。实施者以 `p3s1/mutate.py` 在各自提交的树上写一条跑一条（清单 `mutations.py`、`mutations-e2e.py`、`mutations-fix.py`、`mutations-fix2.py`；vitest 跑相关的文件；e2e 的经 `heavy.sh` 重建前端，再在 Chromium 里跑 UI2），没改的树先跑同样的命令（base-a、base-b、base-check、base-ui2、base-fix、base-fix2），e2e 那批之后 base-ui2-after 也绿；1–62 与 t1–t14b 在 `276c888a` 上，64–68 在 `2979f1c2` 上，69、69b 在 `6987baa9` 上。审查 C 在 `276c888a` 上独立重跑实施者的 18 条（`rerun-impl.py`），修复核对时在 `2979f1c2` 上跑 64–68 与自己的；A 在 `276c888a` 与 `2979f1c2` 上跑自己的。行号是各自那个提交的。

设计 7.2 的 S1 各条：

| # | 改坏 | 抓住它的 |
|---|---|---|
| 1 | `useOriginRecord` 不记来处 | `settings-origin.test.tsx` 5 格、`settings-directory.test.tsx` 5 格 |
| 2 | 设置页之间不保留来处 | `settings-origin.test.tsx` 2 格 |
| 3 | 不在设置里时不调 `setSeen` | `settings-origin.test.tsx` 3 格（含从搜索再进入） |
| 4 | "返回"不带 `arrived` | `settings-origin.test.tsx` 1 格、`settings-directory.test.tsx` 3 格 |
| 5 | 目录里跨外框的链接不带 `arrived` | `settings-directory.test.tsx` 焦点格 |
| 6 | 同外框的链接不经 `useFollow`、总带 `arrived` | 同上 |
| 7 | 抽屉里同外框的链接固定 `opened` | `settings-directory.test.tsx` 窄屏格 |
| 8 | `/settings` 不带 `sidebar` | `layout.test.tsx` 1 格；`user-menu.test.tsx` 抓不住（审查 C 的 N4，7.2 随之改） |
| 9 | `WorkspaceLayout` 不看设置模式 | `workspace-layout.test.tsx` 3 格、工作区的 `general-page.test.tsx` 2 格、笔记本的 `general-page.test.tsx` 4 格 |
| 10 | `IndexTo` 不转交状态 | `settings-directory.test.tsx` 的 `IndexTo` 格 |
| 11 | 三个布局的 `SettingsPage` 去掉 `key` | `settings-directory.test.tsx` 2 格；UI2 第 3 格 `:159`（Chromium，重建前端） |
| 12 | "Workspace settings"一组改名 | `members-page.test.tsx` 1 格、`ownerless-page.test.tsx` 5 格、`settings-directory.test.tsx` 9 格（n9 没有单独重建跑） |
| 13 | 最近的工作区不对照列表 | `settings-directory.test.tsx`"不在列表里" |
| 14 | 列表读过也再读 | `settings-directory.test.tsx` 的读取计数 |
| 15 | 设置模式里不画 `hidden` 的 `SlotHolder` | 笔记本的 `general-page.test.tsx` 5 格、`transfer-background.test.tsx` 13 格 |
| 16 | 去掉 `hidden` | `settings-directory.test.tsx` 2 格；UI2 第 1 格 `:52`（"Pages of Plans"的 `toHaveCount`，Chromium） |
| 17 | 设置模式里 Mod+K 照旧 `preventDefault` | `workspace-layout.test.tsx` 1 格 |
| 18 | `bounced` 总是假 | `profile-page.test.tsx` 1 格 |
| 19 | ✓ 写进文字 | `settings-page.test.tsx` 1 格、`name-row.test.tsx` 4 格 |
| 20 | "保存"一直显示 | `name-row.test.tsx` 7 格 |
| 21 | 去掉 `useFocusLeaving` | `name-row.test.tsx` 1 格 |
| 22 | 发送中用 `disabled` | `name-row.test.tsx` 2 格 |
| 23 | `aria-describedby` 写成两个 id | `name-row.test.tsx` 2 格 |
| 24 | 主题的单选去掉 `onChange` | `appearance-page.test.tsx` 1 格 |
| 25 | 组标题的名字不放 `bdi` | `settings-directory.test.tsx` 1 格 |
| 26 | 齿轮不带 `arrived` | 笔记本的 `general-page.test.tsx` 1 格 |
| 27 | 导航的"设置"不带 `arrived` | `workspace-layout.test.tsx` 1 格 |
| 28 | 撤回之后焦点不到 `h1` | `tokens-page.test.tsx:261` |
| 29 | `threeBrowsers` 漏写 `ui2-` | `node tools/e2e-projects/check.mjs`（firefox、webkit 都报 "lists no cell UI2 …"） |
| 30 | ✓ 的淡出去掉 | 没有格抓，照设计（`settings-page.test.tsx` 只核对类名；S2 的 UI3 第 1 格抓）；S1 里由截图看（实施者的减少动效一张、B 的淡出实量） |
| 45 | "Notebook settings"一组改名 | `settings-directory.test.tsx` 5 格、`transfer-page.test.tsx` 1 格（tr1、tr2 没有单独重建跑） |
| 46 | 以 `replace` 进入也记来处 | `settings-origin.test.tsx` 2 格（登录、引导） |
| 47 | 工作区之外的来处不带片段 | `settings-origin.test.tsx` 邀请一格 |
| 48 | 工作区里的来处也带片段 | `settings-origin.test.tsx` `#h` 一格 |
| 49 | 来处的工作区不在已读的列表里时也回来处 | `settings-origin.test.tsx` 404 一格 |
| 50a–d | 邀请页、变更集页、回收站的 `Preview` 与 `Gone` 的 `h1` 照旧 `useRef`（各改一次） | `settings-origin.test.tsx` 邀请一格；`settings-directory.test.tsx` 变更集一格；`trash-preview.test.tsx` 两个新格 |
| 51 | 先取最近的工作区 | `settings-directory.test.tsx` Lab 与 Acme 一格 |
| 52 | 读列表的键不看 `wanted` | `settings-directory.test.tsx` 一次也不读一格 |
| 53 | 问题不看 `changed` | `name-row.test.tsx` 打回原名一格 |
| 54 | `ScrollKept` 不还原 | `workspace-layout.test.tsx` 滚动一格 `:202:19`（438 ≠ 300） |
| 55 | 在个人资料页上也带 `arrived` | `profile-page.test.tsx` 2 格（含窄屏模拟 `inert` 一格）；UI2 第 3 格 `:188`（"Navigation"的 `toBeFocused`，Chromium） |
| 56 | `SavedOutput` 的轮次不换 | `settings-page.test.tsx` 轮次一格 |
| 57 | 组标题与名字之间不留空格 | `settings-directory.test.tsx` 7 格 |
| 62 | 主按钮去掉透明边框 | `button.test.tsx` 的类名表 |

实施者另写的：

| # | 改坏 | 抓住它的 |
|---|---|---|
| t1 | 横幅不看 `changed` | `name-row.test.tsx` 拒绝一格 |
| t2 | 不数编辑次数 | `name-row.test.tsx` 1 格、`profile-page.test.tsx` 1 格 |
| t3 | 不 trim 就比较 | `name-row.test.tsx` 只差空白一格 |
| t4 | 保存之后不清草稿 | `name-row.test.tsx` 2 格 |
| t5 | `SettingsRow` 不画 `value` | `profile-page.test.tsx` 1 格 |
| t6 | 目录里没有外观与语言 | `settings-directory.test.tsx` 2 格、`appearance-page.test.tsx` 1 格 |
| t7 | 危险操作一组没有标题 | `security-page.test.tsx` 1 格 |
| t8 | 笔记本不在了也回来处 | `settings-directory.test.tsx` 1 格 |
| t9 | `SettingsPage` 的 `h1` 去掉 ref | `settings-page.test.tsx` 2 格 |
| t10 | `NavItem` 当前一行去掉透明边框（845） | 没有格抓，由强制色的截图看 |
| t11 | `notebook-slot.tsx` 去掉焦点守卫 | 没抓，等价（7.2 表后） |
| t12 | `Layout` 给 `SettingsOrigin` 的是 `undefined` | `settings-origin.test.tsx` 5 格、`settings-directory.test.tsx` 5 格 |
| t13 | 令牌为空的文字去掉 | `tokens-page.test.tsx` 3 格 |
| t14 | 单选在 render 函数里重读 store | 没抓：写法等价，作废，换成 t14b |
| t14b | 撤回外观页的绕法（不在页里先读，单选读 `preferences.theme`、`preferences.locale`） | `appearance-page.test.tsx` 1 格（`276c888a`；849 之后外观页本来就在 `children` 里读，由 67 守） |

修复的各条（实施者；7.2 新的几行）：

| # | 改坏 | 结果 |
|---|---|---|
| 64（= A 的 m4、C 的 c7） | `NameRow` 发送中不拦提交（`if (!changed)`） | `name-row.test.tsx` 扣住一格 `:129:23`，got 3（`2979f1c2`） |
| 65 | 变更集页的 `useArrivalFocus` 回到外层 `ChangesetShell`，ref 交给 `ChangesetRead` | `changeset-page.test.tsx` 新格 `:87:54`（焦点在 `body`） |
| 66（= A 的 m2） | `useBack` 列表没读到也回 `/` | `settings-origin.test.tsx` 新格（`['Back', '/']` 不等于 `['Back', '/lab']`） |
| 67 | `SettingsRow` 不是 `observer` | `settings-page.test.tsx` 新格 `:85:69`（'Light' 应为 'Dark'）、`appearance-page.test.tsx`"选了即生效" `:21:55` |
| 68（= A 挪 `ScrollKept` 的那一条） | `ScrollKept` 挪到 `nav` 之前 | `workspace-layout.test.tsx` 滚动一格 `:215:26`（438 ≠ 738）；`6987baa9` 上重跑 `:222:26` |
| 69（= C 的 c12） | `ScrollKept` 不移掉监听 | 同一格 `:221:19`（+0 ≠ 300；`6987baa9`） |
| 69b（= C 的 c13） | 设置模式里也监听 | 同一格 `:221:19`（+0 ≠ 300） |
| 69、69b 配旧桩（对照） | 同上，桩只夹断、不发 `scroll` | 都绿：即 P3S1C1-M1 的缺口 |

审查者另写的（B 没写变异；令牌页一行的三种写法以静态页比过）：

| # | 谁 | 改坏 | 结果 |
|---|---|---|---|
| m1 | A | 来处的工作区取本次渲染的 `slug` | 抓住（`settings-origin`、`settings-directory` 4 格） |
| m2 | A | `useBack` 一律回 `/` | 第一轮活下来（P3S1A-N2）；`2979f1c2` 之后由新格抓住（= 66） |
| m3 | A | 笔记本一组的链接改作跨外框 | 抓住（`settings-directory.test.tsx` 焦点格） |
| m4 | A | 去掉 `NameRow` 的 `sending` 守卫 | 第一轮活下来（P3S1A-N1）；`2979f1c2` 之后 `:129:23` 红（= 64） |
| m5 | A | 编辑时不清"已保存。" | 抓住（`profile-page.test.tsx` 1 格） |
| m6 | A | 轮次在两个方向都换 | 抓住（`settings-page.test.tsx` 轮次格） |
| m7 | A | 设置模式里不把左栏放到顶上 | 抓住（`workspace-layout.test.tsx` 滚动格） |
| m15 | A | 去掉"保存"的 `aria-disabled` | 抓住（`name-row.test.tsx` 扣住格） |
| m16 | A | 主题一组去掉 `aria-describedby` | 抓住（`appearance-page.test.tsx` 1 格） |
| 挪 `ScrollKept` | A | `ScrollKept` 挪到 `nav` 之前 | 第一轮活下来（vitest 14 格全绿，浏览器里仍 618；未核实的怀疑）；`2979f1c2` 之后 `:215:26` 红（= 68） |
| observer | A | 修复核对：`SettingsRow` 去掉 `observer` | 抓住（= 67，两格） |
| c1 | C | 同 m1 | 抓住（`settings-origin` 2 格（`#h`、404）、`settings-directory` 2 格） |
| c2 | C | POP 进入也不记来处（`type !== "PUSH"`） | 抓住（`settings-origin`"后退两条"一格） |
| c3 | C | `bounced` 在账户读取中也为真 | 活下来，等价：账户没读到时 `Corner` 只留出高度、不放账户按钮，读到的那一次渲染 `bounced` 变假；差别只是空白页上多一行留高 |
| c4 | C | `ScrollKept` 不听 `scroll` | 抓住（滚动一格） |
| c5 | C | 设置模式里不把左栏滚到 0 | 抓住（同一格） |
| c6 | C | `IndexTo` 总带 `arrived` | 抓住（`IndexTo` 的 `null` 一格） |
| c7 | C | `NameRow` 发送中不拦提交 | 第一轮活下来（P3S1C-M1）；修复核对时红（= 64） |
| c8 | C | `atProfile` 取 `/settings/*` | 抓住（`profile-page`"安全页上选 Settings"一格） |
| c9 | C | 账户菜单在个人资料页上也不把焦点还给按钮 | 抓住（`profile-page` 宽屏一格） |
| c10 | C | "保存"不写 `aria-busy` | 抓住（`name-row` 扣住一格） |
| c11 | C | `SaveButton` 离开时焦点不回输入框 | 抓住（同一格） |
| c12 | C | 修复核对：`ScrollKept` 不移掉监听 | 头的桩下活下来（P3S1C1-M1）；C 的桩与 `6987baa9` 下红（= 69） |
| c13 | C | 修复核对：设置模式里也监听 | 同上（= 69b） |

对照（只改测试，或测试加变异）：C 的 `probe-c7.py`（那一格加放开之后的计数：头上绿，c7 下红，got 3）；C 的 `probe-scroll.py`（头的桩与发 `scroll` 的桩各配没改、c12、c13、68：头的桩下只有 68 红，发 `scroll` 的桩下三条都红、没改的绿）；先红（实施者与 C 各一次）：把 `276c888a` 的 `changeset-page.tsx`、`settings-page.tsx` 换回去跑两个新格，都红；C 只跑 `user-menu.test.tsx` 的 8（13 格全过）。

没有格守的：30（✓ 的淡出，S2 的 UI3 第 1 格抓）；t10（`NavItem` 当前一行的透明边框，强制色的截图看）；令牌页说明的 `min-w-0 flex-1 basis-56` 与令牌行日期的 `whitespace-nowrap`（截图看）；"已保存。"的预留宽度（决定 848，7.2 表后：800 与 940 宽的截图看）；Mod+K 一格"返回之后再按"那一步没有配变异。等价的：t11、t14、c3。第 8 行只有 `layout.test.tsx` 抓。

## 门禁与 CI

| 提交 | 实施者 | 审查 C |
|---|---|---|
| `7c5b7517` | 提交之前：第一次 `heavy.sh make lint-web knip test-web` 红一格（新写的 `settings-directory.test.tsx` 中文一格以英文的"Navigation"找菜单按钮，格的写法，`strip` 改为收菜单的名字），第二次红在 lw12:230 的 `no-await-in-loop`，之后绿（219 个文件、5280 格）；第一次 `CI=1 make e2e` 613 过、8 败、1 跳，2.5 min（实施中的发现）；改了之后 tb2 与 UI2 单独跑 11 格都过；重跑 618 过、3 败（基线）、1 跳，2.8 min，负载 29.9 | 绿，219 个文件、5280 格；`CI=1 make e2e` 618 / 3 / 1，2.6 min（基线三格，同行同消息）。A 也跑了它的 web 门禁：219 个文件、5280 格 |
| `276c888a` | 提交之前第一次门禁卡在 knip（`rowValueClass`）；第一次 `make e2e` 617 过、4 败（a8:79，实施中的发现），2.7 min；修好之后绿（221 个文件、5293 格），`make e2e` 618 / 3 / 1，2.8 min，负载 29.86 | 合并之前的整条命令（下表） |
| `2979f1c2` | 绿（221 个文件、5296 格）；`CI=1 make e2e` 618 / 3 / 1，2.6 min，负载 29.05 | 合并之前的整条命令（下表） |
| `6987baa9` | 绿（221 个文件、5296 格，没有新格）；它是头，e2e 由合并之前的整条命令跑（下表） | 修复核对第 2 轮：绿，221 个文件、5296 格；`git diff --check` 与受保护路径以 0 退出；没有重跑全量 e2e |

| 合并之前（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`） | 结果 |
|---|---|
| `276c888a`（实施者；C） | 实施者：lint、knip、gen-check、build 过；221 个文件、5293 格（55.3 s）；e2e 618 过、3 败、1 跳（基线三格，同行同消息），Playwright 2.5 min，整条约 3 分 40 秒，负载 26.03；受保护路径与 `git diff --check` 0。C：221 个文件、5293 格；e2e 618 / 3 / 1，2.5 min（开始时负载约 20–38）；`git diff --quiet 446d7e99...HEAD -- …` 与 `git diff --check 446d7e99...HEAD` 0，跑完树干净 |
| `2979f1c2`（实施者；C） | 实施者：221 个文件、5296 格（56.9 s）；e2e 618 / 3 / 1（基线三格），Playwright 2.7 min，整条约 3 分 50 秒，负载 31.65；受保护路径与 `git diff --check` 0。C 第一次（开始时负载 20.08）：221 个文件、5296 格；e2e 616 过、5 败、1 跳，3.1 min：基线三格之外 UI2 [webkit] `:95`（5.0 s）与 LW10 [webkit] `:23`，两格都是造数的 API 调用（建工作区、建页）答 500，服务端日志两条同在 01:44:38，`commit transaction: timeout: context deadline exceeded`，页面还没打开就失败；数据库在负载下卡住，不是这个只改 `web` 的提交造成的（7.3 的"别的红了先停、查清"：查清了，是基础设施）。重跑 `CI=1 heavy.sh make e2e`（开始时负载 15.03）：618 / 3 / 1，2.9 min，只败基线三格；受保护路径与 `git diff --check` 0，树干净 |
| `6987baa9`（实施者） | lint、oxfmt、knip、gen-check、build 过；221 个文件、5296 格（81 s）；e2e 619 过、2 败（se2:231 [chromium] `toHaveValue`、lw8:836 [webkit] `toBeFocused`）、1 跳，pg15:113 这次过了；Playwright 3.2 min，整条约 4 分 50 秒，负载 27.47；受保护路径与 `git diff --check` 0 |

- 每次都只败基线里同行同消息的格（SE2 `se2:231:23`、PG15 `pg15:113:5`、LW8 `lw8:836:29`），`6987baa9` 那一次 PG15 过了；C 在 `2979f1c2` 上的第一次另有上表的两格（基础设施）。提交之前的两次第一次 `make e2e` 见"实施中的发现"。
- Go 一侧：`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 每次以 0 退出；`git diff --check` 干净；`m13-ui-refresh` 就是 `446d7e99`，分支是快进（C）。
- 先写、先红：提交 1 新写与改写的 vitest 格在没改的代码上 338 格里 54 格红，改写的格单独换回 HEAD 的代码 34 格里 23 格红，UI2 三格 × 三种浏览器 9/9 红；提交 2 的新格在提交 1 的树上 15 格红。在提交 1 的树上本来就绿的三格守 `DisplayNameForm` 原有的行为（`name-row.test.tsx` 的"扣住时又编辑""422 的问题与描述"，`profile-page.test.tsx` 改写的"扣住时又编辑"）。`2979f1c2` 的新格里 changeset 一格与 849 一格先红；name-row 放开之后的计数、settings-origin 的列表 503 一格与滚动一格的夹断桩在头上本来就绿，由变异 64、66、68 看到红。
- vitest：5237（216 个文件，S1 之前）→ 5280（219，提交 1）→ 5293（221，提交 2）→ 5296（221，`2979f1c2`；`6987baa9` 不变，滚动一格只改桩）。E2E：609 / 3 / 1（S1 之前）→ 618 / 3 / 1（UI2 三格 × 三种浏览器）；`6987baa9` 上 619 / 2 / 1（PG15 过了）。
- UI2 的时间（第 1 / 2 / 3 格）：

| 跑 | Chromium | Firefox | WebKit | 合计 |
|---|---|---|---|---|
| 实施者，`276c888a` 合并之前的整条命令 | 0.95 / 0.68 / 2.6 s | 2.9 / 1.0 / 2.9 s | 1.9 / 1.1 / 3.0 s | 约 17 s |
| 审查 C，`276c888a` 合并之前的整条命令 | 1.4 / 0.75 / 2.5 s | 2.6 / 1.1 / 3.1 s | 1.8 / 1.4 / 3.1 s | 约 18 s |
| 实施者，`2979f1c2` 合并之前的整条命令 | 1.4 / 0.64 / 2.5 s | 3.1 / 1.3 / 3.4 s | 1.8 / 1.0 / 3.0 s | 约 18 s |
| 审查 C，`2979f1c2` 重跑的 `make e2e` | 1.3 / 0.70 / 2.6 s | 3.0 / 1.1 / 3.4 s | 3.7 / 1.3 / 3.0 s | 约 20 s |
| 实施者，`6987baa9` 合并之前的整条命令 | 1.6 / 0.89 / 3.0 s | 3.5 / 1.1 / 3.2 s | 1.6 / 1.2 / 3.1 s | 约 20 s |

  7.5 推断 UI2 每次 3–6 s，实测远低于它；C 第一次的 WebKit `:95` 是 5.0 s，就是那次 500。E2E 多了 9 格，Playwright 2.5–2.9 min，负载更高时 3.1–3.2 min。
- **只靠 CI 的**（决定 795(d)）：照旧是 se2 的 page 格在 se2:230 之后的步骤与 UI1 的 `Control+K`（[P2/S3 的记录](P2-S3-search-drawer-review.md)门禁一节）；S1 没有新的（UI2 不按 `Control+K`，03-P3 7.3）。
- **没有重现的 CI 项**：照 [01-P1](../01-P1-foundation.md) 7.3 的表（Go 一侧的全部（Go 的树没变）、`lint-actions`、`deploy-check`、`image-smoke`、`release.yml` 的演练、`vuln.yml`、Linux 与 macOS 的差别）。
- **CI**：本机没有推送的凭据（决定 746、761）：负责人推送之后在最后一个头上补跑，看上面只靠 CI 的几步。

## 视觉的核对与探针

截图都在会话的临时目录，不进仓库。

- **实施者**（7.4 的 S1，Chromium）：46 张：个人资料亮暗 × 1440、375 的四态（没改、改了、问题、已保存），外观与语言、安全、令牌（含没有令牌），940 宽保存前后，工作区与笔记本设置的目录（亮暗，长名字，中文一张），375 宽的笔记本设置与抽屉，强制色（个人资料改了、已保存，目录），减少动效（淡出之后），样稿亮暗；实量在 `shots-measures.json`：内容宽 696 px（列宽 712）；`h1` 26 px、行高 39 px、650、−0.26 px、下边 28 px；1440 宽名称一行四态输入框都在 x=867，"保存"离卡片右边 19 px，问题的字从 867 起（盒子从 791，`ps-19`），出了问题时行高 73 → 88、输入框只上下动；940 宽三态行高 115、输入框在 x=581（在说明之下，848 的代价）；375 宽依次是输入框、问题、"保存"，"已保存。"在 x=43，没编辑时行高 115；长名字组标题一行 34 px、名字省略；强制色下目录当前一行 1 px 系统色边框、✓ 是 CanvasText、"保存"有边框、"返回"是链接色；减少动效时淡出照常（0 s、1 s、2.2 s 透明度 1、1、0）。`2979f1c2` 之后重截令牌页（`tokens-{1440,768,375,320}-{light,dark}[-zh]`，P3S1B-M1 一行），`6987baa9` 之后重截 375、320 的令牌行（P3S1B1-N1 一行）。截图用的临时故事已删。
- **审查 B**：139 张（Chromium 45、Firefox 45、WebKit 35，S1 期间旧内容的 12 张，令牌页一行三种写法的对照 2 张），临时故事 `ui2-zz-revb.spec.ts` 在三种引擎里 12 格过、3 格跳（WebKit 不模拟强制色），实量在 `measures.txt`，令牌页一行的三种写法另以静态页 `tokens-row.mjs` 试；修复核对另截 27 张（令牌页中英文 × 1440、768、375、320，三种引擎；外观页选"暗"之后，三种），以 jsdom 比变更集页四态的 `main` DOM。
- **审查 A**（行为，Playwright 探针，三种引擎，1280 与 375）：来处的各种进入法、左栏滚动、每条路由的账户按钮、引导中的弹回、Mod+K、375 跨外框、`NameRow`、外观页，列表读取中的探针（Chromium、WebKit）；vitest 探针 `zz-probe-changeset-arrival.test.tsx`（变更集页的到达，M1）；修复核对重跑全部探针，另加变更集页的四个（读到之前离开再返回、暂加 839 的任务行链接、首次 503、撤销对话框）。
- **审查 C**：`probe-c7.py`（N1、M1 那一格放开之后再数）、`fix1/probe-scroll.py`（两种桩 × 四种改法）、`check82-head.py`（8.2 第 1–11 条的旧文字与新文字）、`rerun-impl.py`（重跑实施者的变异）。
- **实施者的 `ScrollKept` 探针**（`scroll-probe.spec.ts`，不进仓库）：60 页的树，Chromium 1280×720，键盘按齿轮进出设置：头上返回之后 1445；挪到 `nav` 之前时放回的 1445 被夹成 0，树的 reveal 再停在 1372；指针点齿轮时记下的是 0（Playwright 先把那一行滚进视野）。
- **主持者**：修复核对时看了实施者的令牌行重截与量值（上面"修复核对第 1 轮"末尾）。

## 接受与推后

- 名称一行出了问题时输入框与"保存"上移约 6 px，行高 73 → 88（审查 B 的 P3S1B-N1）：接受，4.1.8 与 7.4 照记。
- 30、t10、令牌页那一行与令牌行日期的类、"已保存。"的预留宽度只由截图看；Mod+K"返回之后再按"一步没有配变异；t11、t14、c3 等价（变异一节）。
- C 的 N2、N3、N4 随本文档提交落实（10.1 列全照实各节，5.2 的 tb2 一行与"9 个"；01-P1 第 9 节写明 `:184` 的版本；7.2 第 8 行去掉 `user-menu.test.tsx`）。
- 交给 S2：`rowValueClass` 要用时再导出；列表的名字用 `text-sm`（决定 850），邀请行与任务行不经 `MemberSummary`、各自改；名称"扣住时再提交"的格放开之后再数（P3S1C-M1）；第一组的标题照决定 851；839 的任务行链接会到达尚未读取的变更集，`ChangesetRead` 已能处理（A 在修复核对时以暂加的 `state` 走过"See in the history"）。

## 未核实的怀疑

- **Chromium 并行时撤销对话框的 Esc**（A 的修复核对）：多个故事并行时，按 Esc 关上那一刻焦点在 `body`，随后才回到"Undo change"（`toBeFocused` 的重试等到了）；看来是 Radix 在 `setTimeout` 里还焦点、负载下变慢；单独跑与 Firefox、WebKit 里都立即回到按钮。没有追是否与拆分有关；按代码，按钮所在的 `ChangesetRead` 不会重挂。照旧。
- **"The change undone"**（A 的修复核对）：S2 的 839 那一路只在 A 暂加的 `state` 上核对了"See in the history"，"The change undone"没有单独走。S2 带上决定 839 时核对。
- **`ChangesetRead` 在数据被清空时重挂**（C 的修复核对）：同一个 id 的数据若被清空再读（不是重新验证），`ChangesetRead` 会卸下再挂，`said` 等状态随之清空。没找到会清空的路径：外层按 id 加了 `key`，事件流与撤销只 `mutate` 重新验证，SWR 保留数据。照旧，只是一点疑虑。
- **强制色**（B）：只在 Chromium 与 Firefox 的模拟下看过，Firefox 的模拟不认暗色，那两张实际是亮色；修复核对没有重截（修复只动令牌页说明的宽度类与令牌行的 span）。
- **B 的其余几条**：测试账户的长邮箱（50 个十六进制字符）在 940 宽折成三行、靠右，没另造中等长度的邮箱看；中文界面只看了长名字的组标题与"返回"，没有逐页截中文（实施者截过 `notebook-settings-1440-zh.png`，样稿本身是中文的）；修复核对时变更集页只比了 jsdom 的 DOM，没截图（修复没改 CSS 类）。
- **A 第一轮的其余几条**：列表读取中的探针只跑了 Chromium 与 WebKit；Mod+K 只核对了 `defaultPrevented`，没有核对浏览器自己的动作；提交 1 的 `make e2e` A 没有单独重跑（C 跑了，618 / 3 / 1）。`ScrollKept` 的先后一条已由实施者的探针证实、由 68 守（第一轮 A 的表）。
- **C 第一轮的两条**都已了结：离开设置之后 Mod+K 照旧拦下，由 P3S1C-N1 的修复加了那一步；令牌页"新建令牌"折行是 B 的 P3S1B-M1。
