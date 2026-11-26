# M13/P3/S2 工作区与笔记本设置：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m13-p3-s2`（从 `m13-ui-refresh` 的 `58305870` 拉出，即 S1 合并之后的头）：`b0768903`（卡片、页名作 `h1`、危险操作在最后，照 6.2 提交 1：两个设置布局的 `title` 换成页名；工作区的常规、成员（含邀请）、无主笔记本（含审计日志），笔记本的常规、成员（含添加）、导入导出换成卡片，两个常规页的"基本信息"；两个名称用 `NameRow`；列表行内不可逆的操作是描边的胶囊，红只在"危险操作"组；随页离开的六个链接带 `arrived`；`SettingsCard` 加 `ordered` 与 `label`，导出列表行的类名常量；`Input` 的 `file:hover:hover-layer`；`transfer.startedBy` 改"By {name}"；四处标签组 `space-y-1.5`；n4、UI2 与读旧 `h1` 的 S1 测试；38 个文件）、`5cb1de3f`（开放程度即生效，照 6.2 提交 2：`AccessRow`（草稿、`sent`、只认最后一次的答复、被拒时退回并说原因、`aria-busy` 取草稿、不禁用），`useChoice` 的 Esc，`changeAccessWith` 改选下拉，新故事 UI3、`threeBrowsers`、`REQUIRED`；9 个文件）、`397241c7`（照实加的：成员页与无主笔记本页分组的格（实施者的变异 t19 活下来）；实施者截图看出的三处：邀请与添加成员的"Role"、导入对话框的"Import under"下拉在标签之下，添加成员写出的那句在按钮旁边，成员的日期不折行；两处 `no-shadow`；7 个文件）、`1e6231e9`（审查的修复：列表行的文字块伸缩（`listTextClass`），空的 `<output>` 不占流，明细与日期不折（`WholeParts`），徽记跟在名字之后，安全页写出的那句在按钮旁边，导入一行不能写时那句是行的值，组说明同宽（决定 852），移动页面对话框的两个下拉在标签之下（决定 853）；12 个文件）、`dd6ef368`（审查的修复：UI3 比较两次写出的 `animationName`，工作区与个人设置的 `h1` 重挂各一格，`useChoice` 的注释；3 个文件）、`8d98f64f`（B 修复核对的 P3S2B1-M1：安全页、邀请与添加成员写出的那句与按钮在可折的一行，长了落到按钮之下；邀请与添加成员的 output 另加 `min-w-0 break-words`；3 个文件）；设计 [03-P3](../03-P3-settings.md) 4.2、5.4、5.5、6.2 |
| 方法 | 实施者一位（子 agent，在自己的 worktree 里）；三位审查者并行，各在自己的分离工作树里构建、跑、改坏代码（02-workflow 3.2；00-M13 4.8 的分工）：A 管行为（读代码、构建分支头，以一个 11 格的临时故事在三种引擎里各跑 1280 与 375：开放程度连选两次与被拒、发送中离开、macOS 的键盘与 Esc、375 的抽屉、`useChoice` 的其他用者、两个名称、决定 839 的链接、删除之后的焦点；在第二个工作树里对前两个提交各跑一次 web 门禁，另写 12 条变异，一个 vitest 探针看 `SettingsPage` 的 `key`）；B 管外观（`make build` 之后以三个临时故事在三种引擎里截了 194 张（Chromium 94、Firefox 53、WebKit 47），对照 `mockups/settings.html` 方案 A 与 7.4 的 S2：亮暗、中英、1440、375、320，开放程度一行 768–1200 每 8 px，强制色、减少动效，长的中文名字；逐项量位置、大小与颜色（`measures{,2,3}-<引擎>.txt`）；静态探针 `collapse{,2,3}.mjs` 比空 `<output>` 的外边距；`classcheck.mjs` 核对 S2 新加的 72 个类名都进了 CSS）；C 管测试、变异、门禁与完成度（独立跑 `b0768903`、`5cb1de3f` 的 `lint-web knip test-web` 与 `make e2e`、`397241c7` 上合并之前的整条命令，UI3 两格各跑 3 次，重跑实施者的 vitest 变异，自写 s1–s10 与 56e，探针 `probe-round.py`，逐条看 7.1 的格、e2e 与工具，以 `check82-s2.py` 重跑 8.2 第 12–20 条的替换核对，列出文档提交要带的）。重的运行都经 `heavy.sh` |
| 结论 | 实施中：照实多一个代码提交 `397241c7`（实施者的变异 t19 活下来，补成员页与无主笔记本页分组的格；实施者自己截图看出三处明显不对）；S1 的三个测试文件读旧的 `h1`，随提交 1 改；UI2 第 2 格也读旧的标题（5.4 漏列）。第一轮：A 2 Nit；B 2 Minor、5 Nit、1 Q；C 1 Minor、3 Nit；没有 Important。主持者的决定 852、853 答了 B 的 N5 与 Q1。修复在 `1e6231e9`（B 的 M1、M2、N1–N4，决定 852、853）与 `dd6ef368`（C 的 M1，A 的 N1、N2）；C 的 N1–N3 随本文档提交。修复核对第 1 轮：A 这一轮只有 Nit，主持者对照 `dd6ef368` 核对；C 没有新的发现；B 的八条已处理，N3 的改法带出一条新的 Minor（P3S2B1-M1：安全页写出的长句挤在按钮右边），修在 `8d98f64f`。修复核对第 2 轮：B 核对 `8d98f64f` |

## 基线

S2 不另跑基线（决定 827）：`58305870` 是 S1 合并之后的头，第一个改行为的提交（`b0768903`）的 `make e2e` 兼作核对。对照的数是 S1 合并时的：vitest 221 个文件、5296 格；E2E 618 过、3 败、1 跳过。已知的本机失败同 [S1 的记录](P3-S1-settings-mode-review.md)：

| 格 | 错误所在的行（格的开头行） | 消息的第一行 |
|---|---|---|
| SE2 [chromium] | `e2e/stories/search/se2-search-visibility.spec.ts:231`（:188） | `Error: expect(locator).toHaveValue(expected) failed` |
| PG15 [webkit] | `e2e/stories/page/pg15-reading-updates.spec.ts:113`（:75） | `Error: expect(received).toEqual(expected) // deep equality` |
| LW8 [webkit] | `e2e/stories/wiki/lw8-health.spec.ts:836`（:822） | `Error: expect(locator).toBeFocused() failed` |

S2 的 `make e2e`（实施者与审查 C 的）都只败这三格或其中两格，同行同消息：PG15 在审查 C 的 `397241c7` 合并之前那一次过了（625 过、2 败）。另有的红：实施者在 `397241c7` 上合并之前的整条命令另败 AS6 [chromium]（`e2e/stories/asset/as6-reading-links.spec.ts:153`，在 `:167`，`apiResponse.json: Response has been disposed`），是 03-P3 7.3 列过的抖动，单独重跑 AS6（Chromium）3 格都过，C 那里没有复现；实施者变异的对照（没改的树）第一次跑 n4 时 testcontainers 等端口超时（`Timed out after 10000ms while waiting for container ports to be bound to the host`，`e2e/fixtures/db.ts:46`），基础设施，重跑 3 格都过。偶发的 LW8 `lw8:771`、`markdown.test.ts` 的"not slower"、`trash-preview.test.tsx:587` 与造数的 500 都没有出现，也没有 flaky。UI3 两格在三种浏览器里共 6 次，`make e2e` 由 618 过变成 624 过（628 格）。

## 实施中的发现

| 发现 | 处置与提交 |
|---|---|
| 提交 1 的第一次 `test-web`：S1 的 `settings-origin.test.tsx`、`settings-directory.test.tsx`、`workspace-layout.test.tsx` 共 18 格红，读的是旧的 `h1`（"Workspace settings""Plans settings"） | 改读页名；同外框链接的焦点由比 `textContent` 改为比元素：`h1` 现在与目录里的链接同名（"General""Security"），比文字分不出焦点在哪（`b0768903`；4.2.1 的照实）。审查 A、C：更严，测的东西不变 |
| UI2 第 2 格也读"Plans settings"（5.4 只列了第 1、3 格） | 改为"General"（`b0768903`；5.4 的照实）。审查 C：对 |
| 工作区名称扣住的一格：打字"Acme Works "途经当前名"Acme"，"保存"卸下再挂上，先取到的元素已脱离 | 打完再取一次"保存"（`b0768903`；4.2.5 的照实） |
| `native-select.test.tsx` 父元素一格：包着下拉、听 `keydown` 的 `div` 过不了 jsx-a11y（静态元素上的交互） | 包装是 `role="toolbar"` 加 `tabIndex={-1}`，不加豁免（`5cb1de3f`；4.2.5 的照实） |
| 实施者的变异 t19（工作区的"离开"不在"危险操作"组里）没有格抓 | 成员页（工作区、笔记本）与无主笔记本页加分组的格：列表无标题，其后的组名依次，离开在"Danger zone"里（`397241c7`）。t19 与随之加的 t21–t23 都红；这几格守提交 1 已有的行为，不是先红的 |
| 实施者截 7.4 的 S2 时看到三处：邀请与添加成员的"Role"、导入对话框的"Import under"与下拉同一行（`Label` 是行内的，main 上就这样）；添加成员的卡片底下多 16 px（空的 `<output>` 是块，`space-y-4` 照算）；375 宽成员行的"Joined Nov 24"折成"No / v 24"（整行 `break-all`） | 三个下拉 `block`（决定 821 的字段竖排）；添加成员写出的那句（"{name} added."）与"Add"并排，同邀请的"Invited {email}. …"；只有地址 `break-all`，"Joined …"进 `whitespace-nowrap`（`397241c7`；4.2.3、4.2.4 的照实）。`move-page-dialog.tsx` 的两个下拉没截到，交审查 B（P3S2B-Q1，决定 853） |
| 800 宽开放程度"选前后行高与下拉位置相同"第一次量出行高 131.5 → 112.75 | 是说明随选择变：viewer 的说明在 800 宽两行，none 与 editor 的一行，与"已保存。"无关；在说明一样的两个值之间来回选（read → edit → read），800、1440、375 宽前后行高与下拉位置都相同（7.4 的照实） |
| 实施者的变异 t1 初写时旧文字的缩进不对（运行器报找到 0 次） | 改正后被抓住（`ownerless-page.test.tsx` 四格） |
| `397241c7` 提交之前的门禁卡在 oxlint 的 `no-shadow`（新格里的 `heading`、`members`）；`1e6231e9`、`dd6ef368` 提交之前各卡一次 `consistent-function-scoping`（新格里的 `readAgain`、`menu`） | 改名、挪到外层（零警告，不加豁免） |
| `dd6ef368` 的个人一格初写时在没改的树上红：从工作区设置到个人设置换了布局，先取的抽屉按钮已卸下，点它不开抽屉，点到的是左栏里同外框的链接，焦点照决定 816 留在链接上 | 抽屉按钮每次重新取（`navigation()`），没改的树上绿，72 之下红 |
| `8d98f64f` 的初版（三处的行只加可折）重截时：邀请写出的那句在 375 宽比表单宽 8 px，320 宽整页横向滚动（341 / 320 px）：长地址最后一段没有可断处，flex 项的 `min-width: auto` 按它撑开 | 邀请与添加成员的 output 加 `min-w-0 break-words`：那一段在表单边上断开，320 宽不再横向滚动（`8d98f64f`，提交之前并进；视觉一节） |

## 第一轮的发现与处置

### 审查 A（行为）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P3S2A-N1 | Nit | 工作区设置与个人设置布局的 `SettingsPage` 的 `key`（`W/pages/workspace/settings-layout.tsx:25`、`W/pages/settings/settings-layout.tsx:46`）没有格守着（决定 815 的按页挂载）：只去掉工作区的 `key`，相关 6 个文件 99 格全绿；只去掉个人设置的，8 个文件 83 格全绿；在 S1 的头 `58305870` 上去掉工作区的，94 格全绿（S1 就有的缺口，S1 的变异 11 是三个一起去掉，只靠笔记本的两格抓住）；只去掉笔记本的，2 格红。A 的 vitest 探针：375 宽从抽屉的"Settings"到 General 再点"Members"，没有 `key` 时 `h1` 是同一个元素、焦点在 `body`。A 另说 `settings-directory.test.tsx:301` 的 `.not.toBe(general)` 比的是名字不同的两个 `h1`，恒真 | `settings-directory.test.tsx` 加两格，375 宽"到达之后再到达"：工作区从首页抽屉的"Settings"进 General、再从抽屉点"Members"；个人从"Profile"再点"Security"（抽屉按钮每次重取）；都断言第二个 `h1` 是新元素、拿着焦点。变异 71、72 各红自己那一格（`dd6ef368`）。`:301` 照留：C 实跑，没有 `key` 时同一个 `h1` 元素被复用、只换了文字，它照样红（C 的 s9） |
| P3S2A-N2 | Nit | `useChoice` 的注释（`W/components/ui/native-select.tsx:58-61`）说 Esc"不会按到对话框或抽屉的 Esc"，与决定 840 不符：Radix 对话框的 Esc 是 document 上的捕获监听，先于下拉的 `onKeyDown`；抽屉碰不到它，是因为抽屉只听自己的根；`stopPropagation` 挡住的只是冒泡的祖先监听（A 的探针：document 上的冒泡监听收不到）。今天的用者都不在对话框里，没有行为问题 | 注释改为：Esc 不再往上冒泡，冒泡的监听收不到；Radix 对话框的 Esc 是 document 上的捕获，照旧先到，连同待用的选择关上对话框（`dd6ef368`） |

A 另核实（三种浏览器，1280 与 375）：前两个提交单独为绿（`lint-web knip test-web`：`b0768903` 221 个文件、5306 格，`5cb1de3f` 5316 格）；`AccessRow`：连选两次（可读、再可编辑），第一次答复扣住时下拉显示"可编辑"、说明随它、带 `aria-busy`、只发出一个请求（`inTurn` 让第二个等着），两次都成功是"Saved."、没有 `aria-busy`、焦点在下拉上，第一次被拒第二次成功不显示横幅，第一次成功第二次被拒显示列表的值（"可读"）与"Only the notebook's admins can change its workspace access."、没有"Saved."；发送中去 Members 再回来，值是新的、页面不报错；macOS 的键盘：打字找到一项算一步、不发出、说明随它，Esc 退回原值与原说明、这一键不冒泡到 document，没有待用的选择时 Esc 不被 `preventDefault`，Esc 之后失焦不发，走一步再失焦或回车（Chromium、Firefox）就发出；375 宽有待用的选择时打开抽屉，选择随失焦用上（决定 807），Esc 关上抽屉、焦点在菜单按钮上；`useChoice` 的其他用者（搜索的"Where to look"、历史的"Client"）走一步再 Esc，值退回、地址不变、焦点留在下拉上；导入对话框的"Import under"不是 `useChoice`，一次 Esc 关上对话框、焦点回"Import a zip…"；两个名称：回车或点"保存"都存，目录的组名随之改，焦点回输入框（WebKit 用指针点"保存"焦点在 `body`，同 S1 的平台差异），普通成员看到只读的值；839 的链接：任务行的"See in the history"（导入的、撤销的）与"The change undone"进入从没读过的变更集，`h1` 拿着焦点；撤回邀请之后焦点在"Invitations"的 `h2`，移出成员之后在 `h1`"Members"；S2 对 S1 测试的改动更严、测的东西不变（唯一的例外是 N1 里 `:301` 那一句，C 实跑证明它仍有意义）。

### 审查 B（外观）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P3S2B-M1 | Minor | 列表行的文字块不伸缩，宽屏上按钮常掉到左下：`member-summary.tsx:33`、`ownerless-row.tsx:78`、`transfer-job-row.tsx:124`、`invitations-section.tsx:201`、`token-row.tsx:31` 的文字块只有 `min-w-0`，flex 按它的最大内容宽度决定折不折行，最长的一行一超过约 460 px，按钮整组掉到文字下面、靠左。1440 宽时无主笔记本页每一行（明细约 610 px）、已结束的导出任务行（约 660 px）、地址长的成员行都这样；同一张卡片里有的行按钮在右、有的在左下。4.1.7 说简单的行"控件在右"，样稿 `.set` 的文字也是伸缩的；与 S1 令牌页 M1 同一个原因 | 文字块是新导出的 `listTextClass`（`min-w-0 flex-1 basis-56`，同 `rowTextClass`），五种行都用（`1e6231e9`）。B 的探针（三种引擎）与实施者的重截：1440、768 宽按钮回到右边、离卡片 19 px，375 宽照旧在文字之下；控件宽的一行（工作区成员页里带角色菜单与"移出"的）在 768 宽落到文字之下（224 + 16 + 约 186 超过 414 px 的行宽），是 `basis-56` 的规则 |
| P3S2B-M2 | Minor | Chromium 里空的 `<output>` 挡住外边距折叠（静态探针：`output` 20，`div`、`span`、`p` 10；Firefox、WebKit 都是 10）：无主笔记本页说明与卡片之间（`ownerless-page.tsx:134`）20 px，应为 10；成员页邀请卡片之下（`invitations-section.tsx:75`）到"Danger zone"36 px，应为 26 | 两个 `<output>` 加 `empty:absolute`：空的时候不占流，仍挂着、仍在无障碍树里，有字时照旧在原处（`1e6231e9`）。B 的 `collapse3.mjs`：中间、最后两种位置三种引擎都是 10 / 26；实施者重截（Chromium，1440 与 375）：10 与 26。B 量过、不取的：`empty:mb-0` 只治中间那处，负外边距在 Firefox、WebKit 里变成 0 / 16，`empty:hidden` 让活区域在有字时才进树 |
| P3S2B-N1 | Nit | 任务行与无主行的第二、三行折在日期与地址中间：任务行 1440 宽"Kept until Nov 25, 2026, / 4:05 AM"，375 宽"Ended Nov / 24, 2026"；无主行原所有者一行整行 `break-all`，地址放得下也被切开（"(fei.oolmu@example.co / m)"） | 明细的每一段放进 `whitespace-nowrap` 的 `span`，段间" · "（新导出的 `WholeParts`，同令牌行），只在" · "处折；任务行的"By {name}"不加（名字长度不定，375 宽会横向滚动）；原所有者一行与 `MemberSummary` 的地址改 `break-words`，只在比一行还长时才断（`1e6231e9`）。重截：1440 与 375 宽都只在" · "处折 |
| P3S2B-N2 | Nit | 名字折行时"You"（令牌行的"Expired"同样）独占一行：名字的 `p` 是 `flex flex-wrap items-center gap-2`，名字是一个 flex 项、占满整行。样稿的 `.badge` 是行内的、`margin-left: 6px`，跟在名字最后一个字之后 | 名字的 `p` 是普通的流，`badgeClass` 前面加 `ms-1.5 inline-block`（`1e6231e9`）。重截：1440 宽名字一行、375 宽名字两行，"You"都在最后一行的末尾 |
| P3S2B-N3 | Nit（S1 的页） | 安全页密码卡片底下多 16 px（`security-page.tsx:106` 的块级 `<output>`）："Change password"之下到卡片底 32 px，第一个标签之上 21 px，三种引擎一样；与 `397241c7` 修的添加成员同一个问题 | 并进 S2：output 与按钮放进 `flex items-center gap-3`，同添加成员（`1e6231e9`）。重截：卡片顶到第一个标签 22 px，按钮到卡片底 17 px。写出之后长句挤在按钮右边：修复核对第 1 轮的 P3S2B1-M1，修在 `8d98f64f` |
| P3S2B-N4 | Nit | 不能写时（读者、升级中）导入一行右边那句（`transfer-page.tsx:122` 的 `<p className="max-w-xs text-sm">`）是前景色、靠左，与左边灰的说明读起来像一段话断成两半 | 不能写时那句作 `SettingsRow` 的 `value`（`rowValueClass`：灰、靠右、最多 60%、长了折行，与常规页的只读值一致），能写时照旧传 `children`；`rowValueClass` 照旧不导出（经 `value` 用它，导出了没有别的用者）；`transfer-import.test.tsx` 加一格：编辑者在页上变成读者、再变回来，导入一行跟着变（`1e6231e9`，变异 73、74）。重截：升级中 1440 宽那句宽 319.8 px、右边离卡片 19 px、灰、靠右；375 宽在说明之下、占满一行 |
| P3S2B-N5 | Nit（可不改） | 组说明的宽度不一：邀请、添加成员是 `max-w-xl`，无主、审计、最近的任务是 `max-w-2xl`；英文的邀请说明在 1440 宽折成"no e- / mail." | 决定 852：都是 `max-w-2xl`（`1e6231e9`）。重截：邀请的说明 1440 宽 672 px，不再这样折 |
| P3S2B-Q1 | Q（S2 之外） | 移动页面对话框（`move-page-dialog.tsx:161,174`）的"Parent page""Position"下拉与标签同一行、间距 0（标签 x=527，下拉 x=599），三种引擎都是；与 `397241c7` 给邀请、添加成员、导入的下拉加 `block` 是同一件事。并进 S2 还是交给 P4？ | 决定 853：并进 S2，两个下拉加 `block`（P2 的文件，`1e6231e9`）。历史与目录筛选的标签组照旧随 P4、P5 剩下的标签组（03-P3 第 9 节）。重截：两个下拉都在标签之下（标签底 417.1、下拉顶 420.1） |

B 对实施者"留作观察"的答复：长名字让列表行的按钮折到文字之下，正常数据也会（M1）；"You"独占一行（N2）；明细在日期中间折行，1440 宽的任务行也会，地址还被 `break-all` 切开（N1）；亮色下悬停的文件按钮接近输入框的底，不改：悬停是决定 757 的叠层，三种引擎亮暗都看得出由白变灰，按钮的边框一直在，它在 `surface-2` 的输入框里变得与底同色是整套叠层在灰底上的通例；`move-page-dialog.tsx` 的下拉确在标签旁边（Q1）。

B 另核对过、没有问题的（1440，三种引擎相同到 0.5 px）：内容列 x=498、宽 696，`h1` 26 px、650、高 39、下到第一组 28；组间 26（除 M2 两处），组标题 13 px、600、`#5f6168`（暗 `#a4a5ac`）、离卡片 10；卡片 14 px 圆角，底 `#fff`（暗 `#212226`），边 `#e7e7ea`（暗 `#2b2c31`）；设置行 14 / 18、最低 64，列表行 12 / 18、最低 56；行内按钮 30 px 胶囊、13 px，靠右的控件离卡片 19 px；说明 12.5 px、宽 420，列表名字 14 px、500（决定 850）；徽记 11 px、左右 6、高 17.7、`surface-2`；开放程度下拉 30 px、9 px 圆角、13 px，名称输入框 240×32；邀请与添加成员的表单 36 px、8 px 圆角（决定 821）。决定 844、851：两个常规页有"基本信息"，成员与无主两页的第一组没有标题；决定 826、838：危险操作在每页最后、红字描边的胶囊，行内的移出、撤回、删除、接管、取消是描边的胶囊，非管理员的常规页没有危险组；开放程度一行（决定 848）：768–1200 每 8 px，中英文、三种引擎，"已保存。"出现前后下拉的位置与行高都不变（110 / 110），宽 68 px、在下拉左边 8 px；1440、375、320 宽都不横向滚动（Chromium 50 页，Firefox、WebKit 各 36 页），含长的中文名字与值；强制色（Chromium、Firefox）下卡片、行内按钮、徽记、危险按钮都有系统色的边框，开放程度的 ✓ 是系统色，"Report"是 ghost、没有边框；减少动效时三种引擎里"已保存。"0.2 s 时不透明、2.4 s 时淡出、文字还在；S2 新加的 72 个类名都编进了 `dist/assets/index-*.css`，`design-tokens.test.ts` 与 `settings-page.test.tsx` 11 格全过；升级横幅在 `main` 全宽（x=276、宽 1140），内容列 x=498，留给 P4；导入对话框的"Import under"在标签之下，添加成员写出的那句在按钮旁边；中文与暗色的六页 1440、375 看过。

### 审查 C（测试、变异、门禁）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| P3S2C-M1 | Minor | 7.2 第 56 行（"已保存。"的轮次）写着 UI3 第 1 格抓得住，实测抓不住：`setRound((n) => 1 - n)` 换成 `setRound((n) => n)`，UI3 第 1 格在三种浏览器里各跑 3 次，9 次全过（`56e.log`）。两次写出之间隔着一次网络往返，浏览器总会先画出空的一态，`:not(:empty)` 不再匹配、动画重来，不靠轮次；轮次只在清空与写回落在同一帧时起作用，UI3 造不出。所以 56 只有 `settings-page.test.tsx` 的轮次一格抓得住，`styles.css:242-244` 那条 `[data-round="1"]` 换 `animation-name` 的规则没有任何格守着 | UI3 第 1 格在第一次"Saved."之后读 `getComputedStyle(saved).animationName`，第二次断言不同（C 给的写法，`dd6ef368`）；7.2 第 56 行的抓手改写，加 70（第二轮的 CSS 规则去掉）。实施者实跑：56e、70 在三种浏览器里都红，头上过。C 的 `probe-round.py` 先跑过同样的写法：头上 3 过，56 之下 3 红，删掉规则 3 红 |
| P3S2C-N1 | Nit | 标签组的处数：03-P3 4.2.7"剩下的八处"、6.2 提交 3"S2 的四处"、01-P1 第 9 节"P3 的其余五处在 S2"说法不一。P1 审查记录的 S2B-N3 列了 12 处：S1 改了 `create-token-dialog.tsx:184`；S2 改了列表里的四处（`invitations-section.tsx:130`、`import-dialog.tsx:318,361`、`add-member-section.tsx:132`），另加列表外的 `add-member-section.tsx:102`，共五处；`move-page-dialog.tsx:159,172` 在 P2/S2（`831e50a6`）就已是 `space-y-1.5`；列表里真正剩下五处：`asset-dialogs.tsx:221`、`search-page.tsx:221`、`index-filters.tsx:60`、`history-filters.tsx:154`、`source-add-dialog.tsx:261` | 本文档提交：03-P3 4.2.7、第 9 节、10.2 与 01-P1 第 9 节照这样写；决定 846 补上的 `history-filters.tsx:132,143` 与 `index-filters.tsx:48` 不在那 12 处里，照旧列在 P4、P5，与列表里剩下的五处共八处 |
| P3S2C-N2 | Nit | 实施者照实 4.2.5 的草稿"名称与开放程度'扣住'的格在发出期间也点按、回车，放开之后 `settle` 再数"对开放程度不成立：名称的两格在发出期间按回车、点"保存"，放开之后再数；开放程度的扣住格（指针一格）没有按也没有点（下拉没有提交，无可按），只在放开之后再数 | 本文档提交：照实 4.2.5 改为"名称的扣住格在发出期间点按、回车，放开之后 `settle` 再数；开放程度的扣住格在发出期间不点也不按，只在放开之后再数" |
| P3S2C-N3 | Nit | 实施者的第一份报告里没有 10.2 的草稿，只有照实的草稿；10.2 要照 10.1 写出：提交（`397241c7` 是照实加的）、测试（5296 → 5306 → 5316 → 5322，先红的数，`397241c7` 的四格不是先红）、变异（M1 改过的 56 与新的一行）、视觉、门禁（AS6 的抖动）、UI3 的时间对 7.5、偏离、N1 的标签组、交给后面 | 本文档提交：03-P3 10.2 照这样写，审查之后的两个提交随之 |

C 另核实：`b0768903` 与 `5cb1de3f` 单独为绿（`lint-web knip test-web` 5306、5316 格，`make e2e` 618 / 3 / 1 与 624 / 3 / 1，各 2.8 min，只败基线三格）；`397241c7` 上合并之前的整条命令（下表）；`m13-ui-refresh` 就是 `58305870`，合并是快进；重跑实施者的 vitest 变异（7.2 的 31–37、39、39b、41–43、58a–f、59–61、63，实施者的 t1–t23），都抓住、失败的格与实施者的一致，e2e 那几条（31e、32e、37e、38、39e、40、42e、43e）没有重跑；7.1 的格都在、写法有意义（`notebook/general-page.test.tsx` 的指针、键盘、Esc、轮次、被拒中英两格、连选两次两格、"View the upgrade"两处、名称扣住与放开之后再数、分组与危险操作只给管理员；`native-select.test.tsx` 的 Esc 两格；工作区名称扣住格打字途经"Acme"之后再取按钮；`transfer-page`、`transfer-background` 两格到达尚未读取的变更集，正好核对 `ChangesetRead`）；键盘几格没有桩 `onMac`：jsdom 的 `navigator.platform` 是空串，`onMac()` 在任何主机上都是假，结果确定；S1 测试的改动没有削弱它们，C 的 s9 照样抓住；`changeAccessWith` 先挂 `answerTo` 再选，40 由 n3 与 UI3 抓；UI3 先 `focus()` 再选、第二次先等文字再 poll 透明度，32e、37e 三种都红，两格各跑 3 次 18 次全过；`check.mjs` 与 `threeBrowsers`；新导出都有生产代码在用，`rowValueClass` 照旧不导出，knip 三次门禁都过；`AccessRow` 照 4.2.5（草稿、`sent`、只认最后一次、被拒退回并吞掉旧的拒绝、`aria-busy` 取草稿、说明取 `choice.value`、不禁用）；`useChoice` 的 Esc 只在有待用的选择时 `preventDefault` 与 `stopPropagation`；决定 850、851 与 S1 交接的"放开之后再数"都照做；以 `check82-s2.py` 在 `58305870` 的设计与 `v0.1-design.md` 上依次替换 8.2 第 12–20 条：旧文字各一次，新文字在各自那一步之后各一次，与实现相符（第 18 条的六处链接、第 14 条的 Esc 与 `aria-busy`）；每次跑完工作树都干净。

## 修复核对第 1 轮

在 `dd6ef368` 上（两个修复提交 `1e6231e9`、`dd6ef368`，接在 `397241c7` 之上）。

- **A**：A 这一轮只有 Nit，没有另起修复核对；主持者对照 `dd6ef368` 核对：N1 的两格（工作区、个人"到达之后再到达"，断言第二个 `h1` 是新元素、拿着焦点）与它们的变异 71、72（各红自己那一格），`:301` 照留；N2 的注释改了说法，代码没动。
- **B**：在 `dd6ef368` 上（分离工作树，`make build`，三个临时故事在三种浏览器里都过；量值 `measures-*.txt`、`measures-c-*.txt`，逐行比的 `rows-*.txt`）。八条已处理，N3 的改法带出一条新的：
  - **M1**：三种引擎按钮的位置完全相同。1440 宽无主、任务、令牌、两个成员页的按钮都在右边、离卡片 19 px，含长地址与长的中文名字；768 宽无主、任务、令牌、笔记本成员在右边，工作区成员页长地址那一行在文字之下，是 `basis-56` 的规则（224 + 约 186 > 414），接受；375、320 宽在文字之下。
  - **M2**：三种引擎无主页说明到卡片 10 px，邀请卡片到"Danger zone"26 px。Chromium 的无障碍树里空的 output 仍是 `status`、`live=polite`、没被忽略；接管之后有字，output 回到 `static`，说明、output、卡片之间各 10 px。
  - **N1**：三种引擎、1440 到 320、中英文的 74 页里，`whitespace-nowrap` 的段没有一个被折开，也没有横向滚动（WebKit 有 0.8 px 的亚像素越出，落在内边距里，看不出）；地址只在比一列还长时才断；任务行的"By {name}"照常折行，"By Christopher Montgomery-Whitfield"在 375 宽自成一段，后面每段都完整。
  - **N2**："You""Expired""已过期"跟在名字最后一个字之后、隔 6 px，比文字行的中线低 1.4 px，与样稿 `.badge` 的基线对齐一样；320 宽长的中文令牌名最后一行正好占满时，徽记折到下一行、前面多 6 px 的缩进，行内元素的常态，只在这种边界出现，接受。
  - **N3**：空的时候按钮到卡片底 16 px，对了；写出之后长句挤在按钮右边（下面的 P3S2B1-M1）。
  - **N4**：升级中与读者的导入一行灰、靠右，宽 320 / 338 px（中文 210 / 238），离卡片 19 px；375 宽在说明之下。
  - **N5**（决定 852）：邀请的说明 1440 宽 672 px，不再折成"e- / mail"。**Q1**（决定 853）：移动对话框的两个下拉都在标签之下，三种引擎一样。
  - 另核对过、没有问题的：强制色（Chromium、Firefox）下"You""Expired"是系统色的实线边框，导入一行的值正常；暗色与中文的九个视图（1440、375）与亮色一致；接管之后审计日志多一条，说明、output、卡片的间距都对。
  - **P3S2B1-M1**（Minor，新）：安全页写出的那句（`security.changed`，"Password changed. Your other sessions are signed out; this one and your access tokens keep working."）在 `1e6231e9` 之后与按钮同在 `flex items-center gap-3` 的一行，挤成窄的一列：1440 宽 284 px、三行，375 宽 125 px、七行。是 N3 给的改法造成的：添加成员的"{name} added."短，这句长。B 的改法：这一行加 `flex-wrap gap-y-2`；探针（三种引擎）：写出之后那句落到按钮之下、占满表单宽（1440 宽 448×40，375 宽 289×60），空的时候 output 宽 0、仍与按钮同行，按钮到卡片底照旧 16 px。同样结构的邀请"Invited {email}. Copy the link and send it to them."（S2 之前就这样）与添加成员的"{name} added."（`397241c7`）建议一并改（读代码推断，没截）。
    处置：`8d98f64f`。三处的行都是 `flex flex-wrap items-center gap-x-3 gap-y-2`（`gap-x-3 gap-y-2` 是代码里已有的写法，算出来与 `gap-3 gap-y-2` 一样）；邀请与添加成员的 output 另加 `min-w-0 break-words`：初版重截时邀请的那句里长地址的最后一段没有可断处，375 宽比表单宽 8 px，320 宽整页横向滚动（实施中的发现）。只改类名，没有格与变异，由重截看（视觉一节）。
  - B 未核实的：读屏的实际朗读没试，Firefox、WebKit 只以 Playwright 的角色查询确认 output 在（`status` 1 个）；邀请与添加成员挤成一列是推断，由实施者重截核对（视觉一节）。
- **C**：P3S2C-M1 修好：UI3 第 1 格第一次"Saved."之后读 `animationName`，第二次断言不同（`ui3-instant-save.spec.ts:64-78`，与 C 给的写法一致）；C 重跑 56e、70 在三种浏览器里各 2 次，6 次全红，红在新加的 `not.toBe`；头上 UI3 两格各跑 3 次，18 次全过。读 `animationName` 不会抖：`toHaveText("Saved.")` 等到的那次渲染里文字与 `data-round` 在同一次提交写上，`getComputedStyle` 同步重算，读到的就是这一轮的关键帧；第二次的 `toHaveText` 不会匹配到上一轮的文字（发出时先清空）。门禁：`1e6231e9` 的 `lint-web knip test-web` 221 个文件、5323 格，`CI=1 make e2e` 624 / 3 / 1，2.5 min；头 `dd6ef368` 上合并之前的整条命令 221 个文件、5325 格，e2e 624 / 3 / 1，2.6 min，都只败基线三格，受保护路径与 `git diff --check` 0；UI3 Chromium 4.2 / 0.90 s、Firefox 4.6 / 1.1 s、WebKit 4.4 / 1.3 s，约 16.5 s。变异：71、72 各红新的那一格，73 红新的导入一格，74 红导入一格与 `upgrade-read-only` 的管理员一格，C 另加的"只去掉笔记本布局的 `key`"红原有的两格，没改的树绿。另核对：新的两格测的是它们说的（71、72 各自只红自己那一格）；导入一格经 `readAgain` 把角色在编辑者、读者之间来回换，按钮与读者的那句随之出现与消失（"作为这一行的 `value`"只在注释里，没有断言，是样子，由重截看）；`WholeParts` 以文字作 `key`，两处调用里各段的前缀都不同，不会重复；`useChoice` 只改了注释。新的发现：没有；N1–N3 随本文档提交。

## 修复核对第 2 轮

在 `8d98f64f` 上（B 的 P3S2B1-M1，接在 `dd6ef368` 之上）。

- **B**：P3S2B1-M1 已处理，另加的 `min-w-0 break-words` 也对，没有新的发现（在自己的 worktree 里 `make build`，临时故事经 `heavy.sh` 在三种浏览器里跑 1440、375、320，中英文、亮暗、强制色）。空的时候三个 output 宽 0、在按钮旁边，按钮到块底 16 px（邀请 17 px），不多出空行；写出之后安全页与邀请在按钮之下 8 px、与表单同宽（安全页 1440：448×40，中文 392×20；375：289×60；320：234×80。邀请 1440：576×40；375：289×60；320：234×100，中文 80），添加成员放得下一行时在按钮旁边（英文长名 1440 宽 404 px，中文 480 px），放不下（375、320）在按钮之下占满宽；320 宽时邀请的地址在 `-` 处与行末断开，`scrollWidth` 等于 320，所有组合都不横向滚动、不越出表单；三种引擎的位置相同（只有不到 4 px 的亚像素文字宽差），亮暗、强制色相同。修复核对到此为止，三位都没有未了的 Important、Minor。

## 主持者的决定（负责人可以改判）

全文在 [03-P3](../03-P3-settings.md) 2.3。

- **852 卡片之上的组说明同宽**（审查 B 的 P3S2B-N5）：组标题与卡片之间的说明（邀请、添加成员、无主笔记本、审计日志、最近的任务）都是 `max-w-2xl`；邀请与添加成员原来是 `max-w-xl`，英文的邀请说明在 1440 宽折成"no e- / mail."。
- **853 移动页面对话框的两个下拉在标签之下**（审查 B 的 P3S2B-Q1）：`move-page-dialog.tsx:161,174` 的两个下拉加 `block`，同 `397241c7` 给邀请、添加成员、导入的下拉（决定 821 的字段竖排）；文件是 P2 的，并进 S2。历史与目录筛选的标签组照旧随 P4、P5 剩下的标签组（03-P3 第 9 节）。

其余的处置不是决定：A 的 N1、N2 与 C 的 M1 修在 `dd6ef368`；B 的 M1、M2、N1–N4 修在 `1e6231e9`（N3 是 S1 的页，一并修）；B 修复核对的 P3S2B1-M1 修在 `8d98f64f`；C 的 N1–N3 随本文档提交。

## 变异

每条命令先在没改的树上跑过、全绿；改坏前备份，跑完从备份复原，核对 `git status` 干净（02-workflow 3.1）；改坏期间不跑 `make gen*`、不用 `git checkout --`。实施者以 `p3s2/mut/run.py` 一条一条地写、跑、还原（清单 `spec.json`；vitest 跑相关的文件；e2e 的先 `make build` 再跑那个故事；`check.mjs` 直接跑），对照以 `control.py` 先跑每条不同的命令（n4 第一次是基础设施的超时，基线一节）；审查之后的几条在 `p3s2/mut2/`（`spec-a.json`、`spec-b.json`，含一条不改代码的对照）。31–63 与 t1–t23 在各自提交的树上，70–74 与 56e 在 `1e6231e9`、`dd6ef368` 上。

设计 7.2 的 S2 各条（带 e 的是同一条改坏的 e2e 一份）：

| # | 改坏 | 抓住它的 |
|---|---|---|
| 31、31e | 开放程度被拒不退回（`catch` 里不清草稿） | `general-page.test.tsx` 被拒两格（中英）；UI3 第 2 格，三种浏览器 |
| 32、32e | 发送中禁用下拉 | `general-page.test.tsx` 指针一格；UI3 第 1 格，三种浏览器 |
| 33 | 绕过 `useChoice`（`onChange` 直接发） | `general-page.test.tsx` 走一步、Esc 两格 |
| 34 | 不持草稿 | `general-page.test.tsx` 指针一格、连选两次 |
| 35 | 前一次的成功清掉草稿 | `general-page.test.tsx` 连选两次（成功）一格 |
| 36 | 前一次的拒绝显示横幅 | `general-page.test.tsx` 连选两次（拒绝）一格 |
| 37、37e | 发出时不先清空"Saved." | `general-page.test.tsx` 换轮次一格；UI3 第 1 格，三种浏览器 |
| 38 | ✓ 的淡出去掉 | UI3 第 1 格，三种浏览器 |
| 39、39e | 列表删除之后焦点不到 `h1` | `notebook/members-page.test.tsx` 移出一格；n4（Chromium） |
| 39b | 离开对话框的 `focusAfter` 不到 `h1` | `notebook/members-page.test.tsx` 离开之后仍在页上一格 |
| 40 | `changeAccessWith` 留着单选 | n3 的 page 格、UI3（Chromium） |
| 41 | `transfer.startedBy` 照旧"Started by {name}" | `transfer-page.test.tsx` 任务的明细一格 |
| 42、42e | "模式"的 `h2` 去掉 `tabIndex` | `general-page.test.tsx` 降级三格；lw12（Chromium） |
| 43、43e | 403 不经 `texts` 说成通用的"forbidden" | `general-page.test.tsx` 被拒两格；UI3 第 2 格，三种浏览器 |
| 44 | `threeBrowsers` 漏写 `ui3-` | `check.mjs`（firefox、webkit 都报"lists no cell UI3…"；运行器把它记作 unnamed，是输出不是 vitest 或 Playwright 的格式） |
| 56e | `SavedOutput` 的轮次不换（S1 的 56 的 e2e 一份） | 第一轮没抓住（P3S2C-M1，C 的 9 次）；`dd6ef368` 之后 UI3 第 1 格的 `animationName`，三种浏览器都红 |
| 58a–f | 随页离开的六个链接各去掉 `arrived`："Open it"、模式一行与横幅里的"View the upgrade"、`NoAdmin` 的链接、"See in the history"、"The change undone" | 各自的格：`ownerless-page.test.tsx`、`general-page.test.tsx` 两格、`notebook/members-page.test.tsx`、`transfer-page.test.tsx`、`transfer-background.test.tsx` |
| 59 | `useChoice` 不处理 Esc | `native-select.test.tsx` 与 `general-page.test.tsx` 的 Esc 格 |
| 60 | 说明取列表的值，不取 `choice.value` | `general-page.test.tsx` 指针、走一步两格 |
| 61 | Esc 不 `stopPropagation` | `native-select.test.tsx` 父元素一格 |
| 63 | 开放程度的 `aria-busy` 取 `useForm` 的 `sending` | `general-page.test.tsx` 连选两次两格 |

S2 的实施者另写的（与 S1 的 t 各自编号）：

| # | 改坏 | 抓住它的 |
|---|---|---|
| t1 | 卡片的 `ul` 去掉 `aria-label` | `ownerless-page.test.tsx` 四格（初写的缩进不对，改正后） |
| t2 | 卡片的 `ol` 去掉 `aria-label` | `ownerless-page.test.tsx` 审计各格 |
| t3 | `ordered` 也画成 `ul` | 同上 |
| t4 | 工作区成员页移出之后焦点不到 `h1` | `workspace/members-page.test.tsx` 两格 |
| t5 | 无主笔记本页删除之后焦点不到 `h1` | `ownerless-page.test.tsx` 两格 |
| t6 | 邀请组没有 `heading` | `invitations-section.test.tsx` 撤回两格 |
| t7 | 最近的任务一组没有 `heading` | `transfer-page.test.tsx` 三格 |
| t8 | 工作区常规页没有"基本信息" | `workspace/general-page.test.tsx` 成员、访客与分组各格 |
| t9 | 工作区"危险操作"一组没有标题 | `workspace/general-page.test.tsx` 分组两格（中英） |
| t10 | 笔记本"危险操作"一组的标题错 | `notebook/general-page.test.tsx` 分组两格（中英） |
| t11 | 中文的"基本信息"写成英文 | 两个常规页的中文分组格 |
| t12 | 只读的开放程度没有"只有管理员能改"一句 | `notebook/general-page.test.tsx` 编辑者、读者两格 |
| t13 | 只读的开放程度没有值 | 同上 |
| t14 | Esc 不看有没有待用的选择 | `native-select.test.tsx` 两格 |
| t15 | 笔记本设置布局的标题换回"X settings" | `notebook/members-page.test.tsx` 两格 |
| t16 | 工作区设置布局的标题换回"Workspace settings" | `workspace/members-page.test.tsx` 等三格 |
| t17 | 工作区名称不做本地检查 | `workspace/general-page.test.tsx` 问题一格 |
| t18 | 笔记本名称没有 `fieldTexts` | `notebook/general-page.test.tsx` 问题一格 |
| t19 | 工作区的离开不在"危险操作"组里 | 第一轮之前没有格抓；`397241c7` 的分组两格红 |
| t20 | `sent` 不计数 | `notebook/general-page.test.tsx` 连选两次两格 |
| t21 | 笔记本的离开不在"危险操作"组里 | `notebook/members-page.test.tsx` 分组两格 |
| t22 | 审计日志一组没有标题 | `ownerless-page.test.tsx` 两格 |
| t23 | 添加成员一组没有标题 | `notebook/members-page.test.tsx` 两格 |

修复的各条（实施者；7.2 新的几行）：

| # | 改坏 | 结果 |
|---|---|---|
| 70 | `styles.css` 去掉第二轮的 `.nw-saved[data-round="1"]:not(:empty) { animation-name: nw-saved-out-again }` | UI3 第 1 格（`:43`），三种浏览器都红（`dd6ef368`；C 的 `probe-round.py` 与修复核对同） |
| 71（= A 的 key-ws） | 工作区设置布局的 `SettingsPage` 去掉 `key` | `settings-directory.test.tsx` 工作区"到达之后再到达"一格 |
| 72（= A 的 key-personal） | 个人设置布局的 `SettingsPage` 去掉 `key` | `settings-directory.test.tsx` 个人"到达之后再到达"一格 |
| 73 | 导入一行的"能不能写"只在挂载时取（`useRef(writesNotebook(notebook)).current`） | `transfer-import.test.tsx` 编辑者变读者再变回一格 |
| 74 | 能写时导入一行也给 `value` | 同上一格；`upgrade-read-only.test.tsx` 管理员一格 |
| 对照 | 不改代码（`settings-directory.test.tsx` 的两格） | 绿（个人一格初写时红，实施中的发现） |

审查者另写的（B 没写变异）：

| # | 谁 | 改坏 | 结果 |
|---|---|---|---|
| key-personal | A | 只去掉个人设置布局的 `key` | 第一轮活下来（8 个文件 83 格，P3S2A-N1）；`dd6ef368` 之后红（= 72） |
| key-ws | A | 只去掉工作区设置布局的 `key` | 第一轮活下来（6 个文件 99 格）；`dd6ef368` 之后红（= 71） |
| key-ws-s1 | A | 在 S1 的头上去掉工作区的 `key` | 活下来（94 格）：S1 就有的缺口 |
| key-nb | A | 只去掉笔记本设置布局的 `key` | 抓住（2 格）；C 在修复核对时同样 |
| applied-list | A | `useChoice` 的 `applied` 取列表的值 | 抓住（3 格） |
| esc-stepped | A | Esc 时不重置 `stepped` | 活下来，等价：`change` 之后它已是假 |
| esc-prevent | A | Esc 退回时不 `preventDefault` | 抓住（2 格） |
| saved-reset | A | 选择时不清"Saved." | 抓住（1 格） |
| invit-focus | A | 撤回邀请之后焦点不到标题 | 抓住（2 格） |
| ownerless-taken-arrived | A | 接管之后的"Open it"不带 `arrived` | 抓住（1 格） |
| banner-arrived | A | 升级横幅的链接不带 `arrived` | 抓住（1 格） |
| job-undone-arrived | A | "The change undone"不带 `arrived` | 抓住（1 格） |
| s1 | C | `useChoice` 取列表的值，不取草稿 | 抓住（3 格） |
| s2 | C | `NameRow` 发送中不拦（S2 的两个名称） | 抓住：两页的扣住格，放开之后再数 |
| s3 | C | Esc 退回时不 `preventDefault` | 抓住（两个文件各 1 格） |
| s4 | C | Esc 退回时不清 `stepped` | 活下来，等价（同 A 的 esc-stepped） |
| s5 | C | 最后一次的成功不清草稿 | 抓住（3 格） |
| s6 | C | 下拉没有 `aria-describedby` | 抓住（5 格） |
| s7、s8 | C | 危险操作组不看是不是管理员（工作区、笔记本） | 抓住：成员、访客、编辑者、读者各格 |
| s9 | C | S1 的 11（三个布局都去掉 `key`） | 抓住：目录 2 格（`:301` 照样红） |
| s10 | C | 前一次的拒绝也退回、说出来 | 抓住：连选两次一格 |
| 56e | C | 轮次不换（e2e） | 第一轮没抓住（P3S2C-M1，9 次全过）；`dd6ef368` 之后红 |

实施者另试过、不进表的：导入一组改成 `observer` 再去掉，没有格红：笔记本换了对象时父组件（`NotebookTransferPage`，`observer`）照样重画，等价，`ImportSection` 照旧是普通函数。

没有格守的：`Input` 的 `file:hover:hover-layer`、危险操作组的 `destructive`、审查修复里的类名（`listTextClass`、两个 `<output>` 的 `empty:absolute`、`WholeParts` 的 `whitespace-nowrap`、徽记的 `ms-1.5 inline-block`、安全页与添加成员的并排、`max-w-2xl`、四个下拉的 `block`；`8d98f64f` 的可折与 `min-w-0 break-words`），由截图看。等价的：A 的 esc-stepped 与 C 的 s4（同一条），导入一组的 `observer`。

## 门禁与 CI

| 提交 | 实施者 | 审查 A | 审查 C |
|---|---|---|---|
| `b0768903` | 提交之前第一次 `test-web` 红 18 格（S1 的三个文件读旧的 `h1`，实施中的发现），一次 oxfmt；之后绿（221 个文件、5306 格）；`CI=1 make e2e` 618 过、3 败（基线）、1 跳，3.0 min，负载最高 50.55 | 绿，5306 格 | 绿，5306 格；`make e2e` 618 / 3 / 1，2.8 min |
| `5cb1de3f` | 绿（5316 格）；`make e2e` 624 / 3 / 1，2.8 min，负载最高 39.35 | 绿，5316 格 | 绿，5316 格；`make e2e` 624 / 3 / 1，2.8 min |
| `397241c7` | 提交之前第一次卡在 `no-shadow` 两处，之后绿（5322 格）；e2e 由合并之前的整条命令跑（下表） | — | 合并之前的整条命令（下表） |
| `1e6231e9` | 提交之前第一次卡在 `consistent-function-scoping`，之后绿（5323 格）；e2e 由 `dd6ef368` 上合并之前的整条命令跑 | — | 修复核对：绿，5323 格；`CI=1 make e2e` 624 / 3 / 1，2.5 min |
| `dd6ef368` | 提交之前第一次卡在 `consistent-function-scoping`，之后绿（5325 格）；它改了 UI3，`CI=1 make e2e` 由合并之前的整条命令跑（下表） | — | 修复核对：合并之前的整条命令（下表） |
| `8d98f64f` | 只改类名：绿（5325 格），初版（只有可折）也绿；e2e 由合并之前的整条命令跑（下表） | — | — |

| 合并之前（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`） | 结果 |
|---|---|
| `397241c7`（实施者；C） | 实施者：lint、knip、gen-check、build 过；221 个文件、5322 格；e2e 628 格，623 过、4 败、1 跳，Playwright 2.5 min：基线三格（同行同消息）与 AS6 [chromium] `as6:153`（在 `:167`，`apiResponse.json: Response has been disposed`），单独重跑 AS6（Chromium）3 格都过；负载 14.59 → 22.08；受保护路径与 `git diff --check` 0。C：221 个文件、5322 格；e2e 625 过、2 败（se2:231、lw8:836；pg15 这次过了）、1 跳，2.8 min（负载最高约 58）；`git diff --quiet 58305870...HEAD -- …` 与 `git diff --check` 0，跑完树干净 |
| `dd6ef368`（实施者；C） | 实施者：lint、knip、gen-check、build 过；221 个文件、5325 格；e2e 624 过、3 败（基线三格）、1 跳，2.5 min；负载 11.69 → 33.95；受保护路径与 `git diff --check` 0。C（修复核对）：221 个文件、5325 格；e2e 624 / 3 / 1，2.6 min；受保护路径（含 Go 树）与 `git diff --check` 0 |
| `8d98f64f`（实施者） | lint、knip、gen-check、build 过；221 个文件、5325 格；e2e 624 过、3 败（基线三格）、1 跳，2.5 min；负载 15.45 → 28.89；受保护路径（含 Go 树）与 `git diff --check` 0，跑完树干净 |
| `633df29f`（主持者；文档提交，加上它的头即合并 `12b0b734` 的树，`m13-ui-refresh` 在这期间没有动过） | 与 `8d98f64f` 只差 `docs/`（`git diff 8d98f64f..633df29f -- . ':(exclude)docs'` 为空），合并之前的整条命令已在 `8d98f64f` 上跑过，不重跑；`git diff --check` 与 `check_links.py m13-ui-refresh`（6 个文件，0 个新的坏链接）干净 |

- 每次都只败基线里同行同消息的格（SE2 `se2:231:23`、PG15 `pg15:113:5`、LW8 `lw8:836:29`），C 在 `397241c7` 上那一次 PG15 过了；实施者在 `397241c7` 上另有 AS6 的抖动（上表）。
- Go 一侧：`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 每次以 0 退出；`git diff --check` 干净；`m13-ui-refresh` 就是 `58305870`，分支是快进（C）。
- 先写、先红：提交 1 相关的 12 个文件在没改的代码上 11 个文件、51 格红；提交 2 的新格在提交 1 的树上 vitest 12 格红（`general-page.test.tsx`、`native-select.test.tsx`），e2e 7 格红（UI3 两格 × 三种浏览器、n3 的 page 格；n3 的 API 格照旧过）。不是先红的：`397241c7` 的分组四格（t19、t21–t23 之下红），审查之后的三格（71、72、73 之下红），UI3 的 `animationName`（56e、70 之下红）；它们守已有的行为。
- vitest：5296（221 个文件，S2 之前）→ 5306（提交 1）→ 5316（提交 2）→ 5322（`397241c7`）→ 5323（`1e6231e9`）→ 5325（`dd6ef368`；`8d98f64f` 只改类名），都是 221 个文件。E2E：618 / 3 / 1（S2 之前）→ 624 / 3 / 1（UI3 两格 × 三种浏览器）。
- UI3 的时间（第 1 / 2 格）：

| 跑 | Chromium | Firefox | WebKit | 合计 |
|---|---|---|---|---|
| 实施者，`5cb1de3f` 的 `make e2e` | 4.1 / 0.93 s | 4.5 / 1.4 s | 4.9 / 1.4 s | 约 17 s |
| 审查 C，`5cb1de3f` 的 `make e2e` | 4.2 / 0.90 s | 4.5 / 1.4 s | 4.6 / 1.5 s | 约 17 s |
| 实施者，`397241c7` 合并之前的整条命令 | 4.1 / 0.77 s | 4.4 / 1.8 s | 4.5 / 1.3 s | 约 17 s |
| 审查 C，`397241c7` 合并之前的整条命令 | 4.0 / 0.80 s | 4.7 / 1.5 s | 4.5 / 1.3 s | 约 17 s |
| 实施者，`dd6ef368` 合并之前的整条命令 | 4.0 / 0.81 s | 4.4 / 1.3 s | 4.5 / 1.2 s | 约 16 s |
| 审查 C，`dd6ef368` 合并之前的整条命令 | 4.2 / 0.90 s | 4.6 / 1.1 s | 4.4 / 1.3 s | 约 16.5 s |
| 实施者，`8d98f64f` 合并之前的整条命令 | 4.2 / 0.78 s | 4.4 / 1.5 s | 4.6 / 1.4 s | 约 17 s |

  03-P3 7.5 推断第 1 格每次 5–7 s、第 2 格 3–5 s，实测 4.0–4.9 s 与 0.77–1.8 s，都低于推断；E2E 多了 6 格，Playwright 2.5–3.0 min。
- **只靠 CI 的**（决定 795(d)）：照旧是 se2 的 page 格在 se2:230 之后的步骤与 UI1 的 `Control+K`（[P2/S3 的记录](P2-S3-search-drawer-review.md)门禁一节）；S2 没有新的（UI3 不按 `Control+K`，03-P3 7.3）。Linux 上 WebKit 在关着的下拉上按方向键立即生效（M12-polish 第 8 项），UI3 只用 `selectOption`；键盘逐项走的规则由 vitest 守，在浏览器里只在 macOS 上走过（审查 A 未核实的怀疑）。
- **没有重现的 CI 项**：照 [01-P1](../01-P1-foundation.md) 7.3 的表（Go 一侧的全部（Go 的树没变）、`lint-actions`、`deploy-check`、`image-smoke`、`release.yml` 的演练、`vuln.yml`、Linux 与 macOS 的差别）。
- **CI**：本机没有推送的凭据（决定 746、761）：负责人推送之后在最后一个头上补跑。

## 视觉的核对与探针

截图都在会话的临时目录，不进仓库。

- **实施者，第一轮**（7.4 的 S2，`397241c7`，Chromium；文件按钮另在 Firefox、WebKit）：亮暗 × 1440、375 的六页（工作区的常规、成员、无主笔记本，笔记本的常规、成员、导入导出（展开报告））；800、1440、375 宽开放程度的前后；成员看长的只读值（70 个汉字的工作区名，1440、375）；被拒的横幅（1440、375）；名称编辑中；升级横幅（亮暗 × 1440、375）；强制色（开放程度的 ✓、名称的"保存"、笔记本常规页、工作区成员页）；文件按钮的悬停（三种引擎、亮暗）；导入对话框（亮暗）。实量：内容列 x=498、宽 696，卡片右边 1194，控件右边 1175（离卡片 19 px）；开放程度宽屏"Saved."占 68 px、在下拉左边 8 px，说明一样的两个值之间来回选，800 宽行高 131.5、下拉 y=363.5，1440 宽 91.5、y=264.75，375 宽 131.5、y=387.5，前后都相同；长的名字 1440 宽右对齐、395 px、三行，375 宽在说明之下占满，都不横向滚动；升级横幅 x=276、宽 1140，内容列 x=498，`h1` 从 y=64 推到 138；强制色下 ✓ 是 CanvasText（`rgb(0, 0, 0)`），"保存"有边框；文件按钮的悬停亮色 255 → 245、暗色 54 → 63（WebKit 63–64）。看出的三处修在 `397241c7`（实施中的发现）。
- **实施者，审查之后**（`1e6231e9`、`dd6ef368`，Chromium 亮暗 × 1440、768、375）：工作区成员、无主笔记本、笔记本成员、导入导出（展开报告）、令牌、安全、升级中的导入导出，读者的导入导出（1440、375），移动页面对话框（亮暗）。实量：1440 宽成员（地址长的也是）、无主、任务、令牌各行的按钮都在右边、离卡片 19 px；768 宽除工作区成员页 bob 那一行（控件宽）外都在右边；375 宽在文字之下（管理员一行只有"Admin"，在右边）；所有页都不横向滚动；无主页说明到卡片 10 px，邀请卡片到"Danger zone"26 px（1440 与 375）；"You"在名字最后一行的末尾（1440 名字一行、375 两行）；安全页卡片顶到第一个标签 22 px、按钮到卡片底 17 px；升级中导入一行那句宽 319.8、右边离卡片 19 px、`rgb(95, 97, 104)`、靠右，375 宽在说明之下；邀请的说明 1440 宽 672 px；移动对话框的两个下拉在标签之下（标签底 417.1、下拉顶 420.1）；任务行与无主行的明细只在" · "处折。
- **实施者，修复核对之后**（`8d98f64f`，Chromium 亮暗 × 1440、375、320）：安全页改了密码之后；工作区成员页向长地址发出邀请之后（`christopher.alexander.montgomery-whitfield.<宽>.<亮暗>@engineering.example.com`）；笔记本成员页添加名字长的成员之后（64 个字母的英文名、37 个字的中文名、42 与 33 个字母的英文名，320 宽是注册时默认的那种没有空格的名字）。实量（亮暗相同）：空的时候 output 宽 0、与按钮同行，按钮到表单那一块的底 16 px（邀请 17 px），与改之前一样；写出之后三处的那句都在按钮之下 8 px、从表单左边起：安全页 1440 宽 448×40（表单宽）、375 宽 289×60、320 宽 234×80；邀请 1440 宽 576×40、375 宽 289×60、320 宽 234×100（去掉 `min-w-0 break-words` 时 297 与 298 px 宽，比表单宽 8 与 64 px，320 宽整页 341 / 320 横向滚动）；添加成员 1440 宽 508 与 567 px、一行，375 宽 289×40 与 281×20，320 宽 234×40；都不横向滚动。
- **审查 B**：194 张（Chromium 94、Firefox 53、WebKit 47），三个临时故事（`ui2-zz-revb-s2{,b,c}.spec.ts`）在三种引擎里各 3 格都过，量值在 `measures{,2,3}-<引擎>.txt`；静态探针 `collapse{,2,3}.mjs`（空 `<output>` 的外边距，M2 的改法）；`classcheck.mjs`（72 个新类名）；M1 的改法以探针在三种引擎里试过（`measures2-*.txt`、`chromium-probe-*-{asis,probed}.png`）。
- **审查 A**（行为）：临时故事 `ui1-zz-s2rev-a.spec.ts` 11 格，在三种引擎里各过（1280 与 375）；vitest 探针 `zz-probe-ws-key.test.tsx`（工作区设置的 `key`，N1）。
- **审查 C**：`mut-c.py`（s1–s10）、`probe-round.py`（M1 的写法在头、56、删掉规则三种情形下）、`check82-s2.py`（8.2 第 12–20 条）、`ui3-base.log`（UI3 两格各 3 次）。

## 接受与推后

- 亮色下悬停的文件按钮与输入框的底接近（实施者的观察）：不改，决定 757 的叠层在灰底上的通例（B）。
- 有意的偏离（03-P3 已写）：模式一行不带样稿的"当前"徽记（4.2.4）；名称一行没编辑时右边空出 68 px（决定 848 的代价）；危险按钮写全名（"Delete notebook"），样稿是"删除…"。
- 768 宽工作区成员页里控件宽的一行按钮在文字之下：`basis-56` 的规则，不另改。
- 320 宽长的中文令牌名最后一行正好占满时，徽记折到下一行、前面多 6 px（B 修复核对）：行内元素的常态，接受。WebKit 里 `whitespace-nowrap` 的段 0.8 px 的亚像素越出，落在内边距里（同上）：看不出，接受。
- 升级横幅与内容列对齐：P4（03-P3 第 9 节）。
- 01-P1 第 9 节剩下的标签组（S2B-N3 列表里五处与决定 846 补上的三处）、历史与目录筛选里可能与标签并排的下拉（B 未核实的怀疑，决定 853）：P4、P5。
- 没有格守的类名由截图看（变异一节）；A 的 esc-stepped、C 的 s4、导入一组的 `observer` 等价。
- C 的 N1–N3 随本文档提交落实（03-P3 4.2.7、第 9 节、照实 4.2.5、10.2；01-P1 第 9 节）。

## 未核实的怀疑

- **非 macOS 的键盘路径**（A）：方向键在关着的下拉上逐项走只在 jsdom 里核对（测试的平台不是 macOS）；本机三种浏览器跑的是 macOS 的行为。照旧：Linux 的真实行为靠负责人推送之后 CI 上的 UI3 与 n3。
- **WebKit 里连续打字找选项**（A）：WebKit 的打字缓冲较长，第二次按"p"没跳到"Private"，像与上一次的"w"连成了"wp"。平台行为，照旧。
- **A 没在浏览器里走的**：离开笔记本、离开工作区、无主笔记本的接管与"Open it"、`NoAdmin` 的链接、升级横幅的链接，只读了代码，靠 vitest 的格与 A 的变异（接管与横幅的 `arrived` 都被抓住）；e2e 没有按提交单独跑，以实施者与 C 的为准。照旧。
- **历史与目录的筛选**（B）：`history-filters.tsx`、`index-filters.tsx` 的标签与下拉在 flex 项里，大概也并排，没有截图。决定 853：随 P4、P5 剩下的标签组一起看。
- **"加载更多"**（B）：审计日志与任务的"加载更多"是 36 px、8 px 圆角的按钮，在卡片之下，与卡片里 30 px 的胶囊不同；数据不到一页，没截到。照旧。
- **`empty:absolute` 对活区域的朗读**（B）：只按"元素仍挂着、在树里"推断，没有用读屏试；B 的修复核对：Chromium 的无障碍树里空的 output 仍是 `status`、`live=polite`、没被忽略，Firefox、WebKit 只以角色查询确认在。照旧，交收尾的读屏清单"H. M13"（03-P3 第 9 节已列"已保存。"的 `<output>`）。
- **UI3 第 1 格第二次写出之后 poll 透明度回到 1**（C）：要在写出之后约 1.5 s 内读到，负载很高时理论上会错过。C 3×3 次、实施者与 C 的几次门禁都没出现；修复核对时 UI3 两格各 3 次，18 次全过。照旧。
- **AS6 的抖动**（C）：实施者那一次的 `as6:167` 在 C 那里没复现，属 7.3 列过的抖动。照旧。
- **实施者的几条观察**：`move-page-dialog.tsx` 的下拉（决定 853，修在 `1e6231e9`）；375 宽"You"独占一行（P3S2B-N2，修了）；任务行与无主行明细在日期中间折行（P3S2B-N1，修了）。都已了结。
- **邀请与添加成员写出的那句也挤成一列**（B 修复核对，读代码推断）：实施者重截核对，同安全页，一并修在 `8d98f64f`（视觉一节）。了结。
