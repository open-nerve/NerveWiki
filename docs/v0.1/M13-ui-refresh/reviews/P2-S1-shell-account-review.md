# M13/P2/S1 外框与账户行：审查记录

| 项 | 内容 |
|---|---|
| 对象 | 分支 `m13-p2-s1`（从 `m13-ui-refresh` 的 `58c5fa92` 拉出）：`0611c586`（右上角、账户行与头像菜单，去掉顶栏）、`0986356e`（侧栏三段、`--sidebar`、宽屏粘住、切换器在顶部）、`a9d7c030`（窄屏的账户菜单把外观直接列在菜单里，决定 801）、`50e67f1c`（审查的修复）、`1d8414a9`（审查发现的测试缺口）、`63810dff`（修复核对第 1 轮的 Nit，主持者）；设计 [02-P2](../02-P2-shell.md) 4.1、6.1 |
| 方法 | 实施者一位（子 agent，在自己的 worktree 里）；三位审查者并行，各在自己的 detached worktree 里构建、跑、改坏代码（02-workflow 3.2；00-M13 4.8 的分工）：A 管行为（右上角跨各路由与会话状态、账户菜单的键盘与指针、`arrived` 与焦点、窄屏的内联菜单、粘住的侧栏与滚轮，Playwright 探针在三种引擎里，另写 vitest 变异）；B 管外观（Chromium 亮暗、1440 与 375、强制色，Firefox、WebKit 在 1440 下的侧栏、账户行与菜单，逐项量大小与间距，对照 palette.html）；C 管测试、变异、门禁与完成度（独立重跑每个提交的检查与合并之前的整条命令、7.2 的 S1 各条与实施者的变异、自写变异找漏洞、核对 10.1 与"照实"的草稿）。重的运行都经 `heavy.sh` |
| 结论 | 实施中：实施者在 375 宽的截图里看到外观子菜单开在屏幕之外，交主持者，由决定 801 修（`a9d7c030`）。第一轮：A 2 Minor、5 Nit、1 Q；B 1 Minor、5 Nit；C 2 Minor、8 Nit、1 Q；没有 Important。Minor 是：`/` 上账户按钮先在右上角再挪进侧栏（A）；Chromium 里没溢出的中段吃掉滚轮（A，决定 802 修）；外观子菜单贴着窗口底边被裁（B）；会话不可用的右上角与粘住的侧栏都没有测试守着（C）。A 的 Q（新建工作区页回不去）由决定 803 答。修复在 `50e67f1c`、`1d8414a9`。修复核对第 1 轮：三位在代码与测试上都没有 Important、Minor；C 的 S1C1-M1 是"照实"草稿漏了 802、803 要改的设计文字，由本文档提交改；C 另有 3 条 Nit，一条由 `63810dff` 修，一条改这份记录的说法，一条并进 10.1 |

## 基线

S1 开工之前（决定 795(a)），在没改的 `m13-ui-refresh` 的头 `58c5fa92` 上经 `heavy.sh` 跑 `CI=1 make e2e`：594 过、3 败、1 跳过，Playwright 3.1 min，整条 3 分 12 秒，负载 10 → 55。

| 格 | 错误所在的行（格的开头行） | 消息的第一行 |
|---|---|---|
| SE2 [chromium] | `e2e/stories/search/se2-search-visibility.spec.ts:231`（:188） | `Error: expect(locator).toHaveValue(expected) failed` |
| PG15 [webkit] | `e2e/stories/page/pg15-reading-updates.spec.ts:113`（:75） | `Error: expect(received).toEqual(expected) // deep equality` |
| LW8 [webkit] | `e2e/stories/wiki/lw8-health.spec.ts:836`（:822） | `Error: expect(locator).toBeFocused() failed` |

没有别的红；偶发的 AS6 `as6:153`、LW8 `lw8:771` 与 `markdown.test.ts` 的"not slower"这次没有出现。02-P2 第 1 节与 7.3 照这张表改写。SE2 错在 :231（第二下方向键之后的 `toHaveValue`），决定 795(d) 写的 se2:230 是那下方向键本身；"se2:230 之后的步骤只在 Linux 的 CI 上走"照旧成立。

## 实施中的发现

实施者自己截 7.4 的 S1 几张时看到的（Chromium，亮暗 × 1440、375）：

| 发现 | 处置与提交 |
|---|---|
| 账户行的菜单与行同宽，版本行里 40 位的提交号被裁掉 | 版本行 `wrap-anywhere`（`0611c586`） |
| 375 宽时右上角的"回到工作区"折成两行、溢出 30 px 的胶囊；箭头是 24 px | 链接带 `shrink-0 whitespace-nowrap [&_svg]:size-4`，账户按钮带 `min-w-0 shrink`，名字 `max-w-32 min-w-0`（`0611c586`） |
| 自写变异"账户行的菜单不向上开""右上角的菜单不靠右"活下来 | `user-menu.test.tsx` 加一格，读 Radix 的 `data-side`、`data-align`（`0611c586`） |
| 375 宽时外观子菜单开在屏幕之外：账户行的菜单与行同宽（359 px），子菜单至少 200 px、左右都放不下，Radix 照样向右开（`@radix-ui/react-menu` 的 `index.mjs:741`），只露出一条边；指针与触摸都选不了主题与语言 | 交主持者：决定 801（`a9d7c030`） |
| 决定 801 的截图里，375 宽时右上角的菜单被版本行里 40 位的提交号撑得比屏幕宽，伸出屏幕右边，版本行被截断 | `collisionPadding={12}` 与以屏宽为限的宽度（`a9d7c030`）；之后照 S1B-N3 定为 16rem 与屏宽取小（`50e67f1c`） |

## 第一轮的发现与处置

### 审查 A（行为）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S1A-M1 | Minor | 已登录的人打开 `/`：`LandingPage` 读工作区列表时，`Layout` 以为这里没有侧栏，右上角画出账户按钮，读完 `Navigate` 到 `/:slug`，右上角那个卸下、侧栏里再挂一个（探针 A11，三种引擎：`none → corner → col`）。决定 769 不要的正是这一闪与挂两次；一 `goto('/')` 就点账户菜单的故事会点到正要卸下的那个 | 接受：index 路由加 `handle: { sidebar: true }`（去处要么是有侧栏的工作区，要么是没有 `handle` 的新建工作区页）；`GiveCornerBack` 从工作区外框挪进 `W/app/corner.tsx` 导出（没有给 `useCornerBack` 加 `on` 参数），`LandingPage` 读列表失败时交还右上角；`layout.test.tsx` 两格（扣住列表时没有"Ada"、放开之后恰好一个在 `[data-shell]` 里；列表答 500 时在右上角），变异 n01、n02（`50e67f1c`） |
| S1A-M2 | Minor | 宽屏、长页、侧栏中段没有溢出（多数工作区首页与小笔记本）时，Chromium 里滚轮停在中段上滚不动页面（探针 E1：`scrollY` 400 → 400；Firefox、WebKit 照滚；把内联的 `overscroll-behavior` 设成 `auto` 就滚）。Blink 把没有溢出的 `overflow-y: auto` 也算进滚动链，`contain` 在那里截断了它。S1 之前侧栏不是滚动区，三种都滚窗口，是回归；第 0 步只量了溢出的情形 | 决定 802：去掉中段的 `overscroll-contain`，滚到头带着窗口走，同 S1 之前（`50e67f1c`）。三种引擎重量见"第 0 步与滚轮" |
| S1A-N1 | Nit | Firefox 里中段溢出、已滚到头，停一下再滚，窗口会被带走（Chromium、WebKit 不会） | 随决定 802 了结：滚到头本来就带着窗口走 |
| S1A-N2 | Nit | 还有引导步骤的账户在 `/onboarding` 从右上角选"Settings"：`Onboarded` 把它送回 `/onboarding?next=…`，没有标题取焦点；`leaving` 挡住了 Radix 把焦点还给还在的按钮，焦点落到 `body`（探针 A10，三种引擎） | 接受：`leaving.current = !atProfile && pendingSteps(me).length === 0`；`profile-page.test.tsx` 一格，变异 n03（`50e67f1c`；那一格的等待由 `63810dff` 收紧，见修复核对） |
| S1A-N3 | Nit | 菜单开着时窗口跨过 md（转平板、拖窗口）：宽变窄时子菜单卸下、焦点到 `body`，方向键失效，Esc 能恢复；窄变宽时焦点落在别的项上，仍可用 | 接受，不改（主持者：少见，Esc 能恢复） |
| S1A-N4 | Nit | 自写变异三条活下来：r01（`onCloseAutoFocus` 不把 `leaving` 复位）、r09（`Corner` 不认 `unavailable`）、r28（"回到工作区"只在 `/settings/profile`） | 接受：`profile-page.test.tsx` 加"从 `/settings/security` 在右上角选 Settings、再打开菜单按 Esc，焦点回到按钮"一格；`layout.test.tsx` 加会话不可用两格（同 S1C-M1）；"回到工作区"一格改从 `/settings/security` 开始（`1d8414a9`） |
| S1A-N5 | Nit | 文档要一并带上的：决定 801、4.1.3 的右上角菜单与版本行、4.1.2 的 `useCornerBack` 不带参数、4.3.1 的 `narrow.ts`、7.1 与 7.2 的新行、N1 的说法 | 本文档提交（02-P2 4.1 的"照实"与各节） |
| S1A-Q1 | Q | 新建工作区页没有回到工作区的路（旧顶栏的"Nerve Wiki"链接去 `/`）；从切换器点了"新建工作区"又不想建的人只能用浏览器的后退 | 决定 803（`50e67f1c`）：`layout.test.tsx` 两格（有工作区时有链接、点了到 `/lab`、`h1` 取焦点；没有工作区的账户从 `/` 落到新建页时没有），变异 n04、n05 |

### 审查 B（外观）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S1B-M1 | Minor | 账户行在侧栏底部，展开"外观"后子菜单被 Radix 的碰撞处理推到窗口最底（三种引擎、两种主题都是 `bottom: 0`，y 663、高 237），下边框与两个圆角落在窗口外；这是宽屏上换主题、语言的唯一路径 | 接受：`DropdownMenuSubContent` 默认 `collisionPadding={8}`；实量离窗口底边 8 px（1440×900 与 1280×720，三种引擎，实施者与 B 各量一次）（`50e67f1c`） |
| S1B-N1 | Nit | 一个菜单里语言名、主题名与普通项的文字各在一条左边线上（375 宽的内联菜单：标签 24、语言名 46、"个人设置"50、主题名 72；登录页的"外观"里语言名贴着主题的图标） | 接受：语言项前一个 `<span aria-hidden className="size-4 shrink-0" />`，与主题名同线：侧栏的子菜单 307、右上角向左翻开的子菜单 1032、375 宽的内联 72（`50e67f1c`） |
| S1B-N2 | Nit | 分组标题比样稿矮 2 px（`text-xs` 的行高 16 px，样稿 18 px），标题紧贴列表（样稿 2 px 的 gap）：标题顶到第一本笔记本 34 px，样稿 38 px | 接受：标题加 `leading-4.5`，`groupClassName="flex flex-col gap-0.5"`；标题高 36、到列表 2 px、到第一行 38 px，三种引擎同样稿（`50e67f1c`） |
| S1B-N3 | Nit | 右上角的账户菜单在宽屏有 444 px（版本行的 40 位提交号不折行），侧栏的菜单是 235 px | 接受：`max-w-[min(16rem,var(--radix-dropdown-menu-content-available-width))]`，1440 下 256 px（版本行折三行），375 下 x 107–363（`50e67f1c`） |
| S1B-N4 | Nit | 切换器的菜单（`w-56`，224 px）比按钮（235 px）窄 11 px | 接受：`w-(--radix-dropdown-menu-trigger-width)`，1440 下 235、375 下 359，与按钮同宽（`50e67f1c`） |
| S1B-N5 | Nit | 强制色模式下菜单的分隔线（`h-px bg-border`）看不见，账户菜单有三四条（P1/S2 起就有） | 接受：`h-0 border-t border-border`，强制色下亮色画成 `1px solid rgb(0,0,0)`、暗色白色；平时与改之前看不出差别（账户、切换器、树、附件四处菜单都看过）（`50e67f1c`） |

### 审查 C（测试、变异、门禁）

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S1C-M1 | Minor | 右上角"会话不可用"一态没有测试：`POST /auth/refresh` 答 503 时 status 是 `unavailable`，变异 y05（去掉 `status === "unavailable"`）五个文件全过；7.1 那一格（`GET /me` 答 500）走的是 `failed` 一支 | 接受：`layout.test.tsx` 的 `test.each(["/settings/profile", "/lab"])`，refresh 答 503：标题"Cannot reach the server"、有"Appearance"、没有"Ada"；y05、A 的 r09 都红（`1d8414a9`） |
| S1C-M2 | Minor | 宽屏粘住的侧栏没有自动的守卫：变异 y09（去掉 `md:sticky md:top-0 md:h-svh md:self-start`）vitest 一个也抓不到，Chromium 全部 417 格只红了基线的 SE2；S3 的抽屉要重写这一串类名 | 接受：L1 (page) 一格的末尾（Chromium，1280×720，长页）把窗口滚到底，断言 `scrollY` 大于 1000、账户按钮 `toBeInViewport()`、`[data-shell]` 的 y 在 1 px 之内为 0（页高带小数、窗口的滚动是整数，滚到最底时粘住的侧栏被容器底边顶上去 0.42 px）；y09 红在 y（3083），C 的 z1（只去掉 `md:sticky`）红在 `toBeInViewport`（`1d8414a9`） |
| S1C-N1 | Nit | 头像那一句在找不到元素时也通过（`undefined` 对 `undefined`）：y33（账户行头像的字换成空串）活下来 | 接受：先 `expect(avatar).toBeDefined()`（`1d8414a9`） |
| S1C-N2 | Nit | 窄屏的内联菜单只测了侧栏那一个：y11（`narrow && row`）活下来，即 375 宽时右上角仍用子菜单；y10（`subscribe` 的清理不移除监听）活下来 | 接受：窄屏一格改成 `test.each(["/lab", "/settings/profile"])`；`narrow.test.ts` 让桩交出监听的集合，卸下之后断言它为空（`1d8414a9`） |
| S1C-N3 | Nit | 右上角的几个小分支没有测试：y01（"回到工作区"出现在所有已登录的右上角上）、y02（右上角用自己的 SWR 键，`GET /me` 读两次）、y03、y04（去掉对 `SessionChangedError` 的排除） | 接受：404 一格断言没有"回到工作区"（y01）；会话启动中一格数 `GET /me` 为 1：守卫与右上角同时在等账户，同一个键只读一次（y02）；`route-error.test.tsx` 首次载入那一格也数 1（那条路上只有右上角在读，抓不住 y02）；y03、y04 照主持者的决定有意不抓（`1d8414a9`） |
| S1C-N4 | Nit | `profile-page.tsx:45` 的注释还说"the top bar's menus" | 接受：改成"the account menu's Appearance"（`50e67f1c`） |
| S1C-N5 | Nit | `dropdown-menu.test.tsx:107` 的注释说"the focus stays on its item"，没有断言 | 接受：加 `expect(document.activeElement).toBe(appearance)`，在 jsdom 里成立（`1d8414a9`） |
| S1C-N6 | Nit | `fireEvent.click` 测的是选中这一步，与用户相同，只缺指针移进子菜单的那一段（jsdom 没有布局）；没有任何地方用真指针在子菜单里选一项 | 接受：a8 用指针在子菜单里选"浅色"（`html` 去掉 `dark`、子菜单仍开着）再选"深色"；指针分十步移过去（见"接受与推后"的 Radix 的方向判断）；C 的 m06e（`keepOpen` 不 `preventDefault`）让 a8 红，z4（去掉分步）让 a8 超时（`1d8414a9`） |
| S1C-N7 | Nit | `initial.test.ts` 里零宽连接符与组合重音是直接写进源码的不可见字符，编辑器做 NFC 规范化时测试悄悄变弱 | 接受：写成转义（`1d8414a9`） |
| S1C-N8 | Nit | 提交 1 的说明说 7.1 的 S1 各格在没改的树上红，`layout.test.tsx` 的会话启动中与列表读取失败两格却不在 `red1.log` 的失败里（旧顶栏下本来就成立） | 记录：这两格不是先红后绿，分别由 m47、y34 守着；审查之后加的"`/` 上列表答 500""没有工作区的账户"两格同样，由 n02、n05 守着 |
| S1C-Q1 | Q | 10.1 与"照实"的草稿在实施者的临时目录里找不到 | 草稿在实施者的报告里，交给了审查者；准确性见修复核对的 S1C1-M1、S1C1-N3 |

## 修复核对第 1 轮

上一轮的发现（三位在 `1d8414a9` 上，各自的 detached worktree，经 `heavy.sh`）：

- **A**：M1、M2、N2、N4、Q1 已修，N1 随决定 802 了结，N3 照决定接受，N5 等本文档提交。
  - A11 三种引擎：在 `/` 扣住列表时没有账户按钮，放开之后只在侧栏（`none → col`，`max` 为 1）；I2：`/` 上列表答 500 时账户按钮在右上角，Try again 之后回到侧栏。
  - E1 三种引擎：中段没溢出时，指针在页上、中段、顶部、底部滚，都滚窗口（400 → 800 → 1200 → 1600）；E2：中段滚到头之后 Chromium、WebKit 在同一次滚动里带着窗口走，Firefox 另起一次才带。
  - A10：在 `/onboarding` 从右上角选 Settings，回到引导页，焦点在账户按钮上。
  - H1–H3（决定 803）：从切换器进新建页、直接载入 `/create-workspace`（右上角自己读完列表之后链接出现；载入到落地共 `GET /workspaces` 2 次）都有"回到工作区"，点了到工作区的 `h1`；没有工作区的账户从 `/` 落到新建页时没有。
  - 回归：index 路由的 `handle` 在各种去向下（引导页、新建工作区页、工作区、未登录、会话不可用、从设置页回到工作区时列表还没读完、落地页的块载入失败）右上角都只出现一次（I1–I5）；`GiveCornerBack` 挪进 `corner.tsx` 前后的序列相同；子菜单的 8 px 在侧栏向右开时每条路径都留着；右上角菜单在 320 宽时 x 59.7–308，页面不横向滚动。
  - 右上角翻到左边的子菜单，Playwright 一步跳到项上就关（K、L，三种引擎）：Radix 用指针在主菜单里最后一次横向移动的方向判断"是不是正往子菜单去"，一步跳进菜单时第一次算出的总是"向右"；按手的走法（从胶囊分步移到 Appearance、再快甩到 Dark）都留着。是测试的假象，a8 分步移动是对的。
  - A 的 r01、r09、r28 与新写的 r30（右上角在新建页上从不读列表）都红。
- **B**：S1B-M1 与 N1–N5 已修（各条的实量见上表）；回归：分隔线换成 `h-0 border-t` 之后账户、切换器、树、附件四处菜单的平时样子没有变化（高 1 px、`--border`、上下各 4 px、宽度不变）；右上角菜单的 16rem 上限加上约 55 个字符的显示名与版本行不溢出；从右上角打开的子菜单向左翻开（x 968–1168，右缘贴着主菜单左缘，顶边对齐"外观"那一项，箭头仍朝右，同 macOS，可以接受）；决定 802 之后在中段与主区滚，侧栏一直 top 0、高 900，没有跳动；新建工作区页的右上角与设置页的完全相同。
- **C**：S1C-M1、M2、N1–N7 已修，N8、Q1 交本文档提交。每个新提交单独为绿、合并之前的整条命令只败基线里的格（门禁表）。实施者的 `final.json` 44 条在 `1d8414a9` 上重跑：42 条红，y03、y04 照预期通过；C 另写的 y07、y16、y34、c02 红，y04b、c03、c04 通过（见变异表）；e2e 的 y09、z1、z4、m06e 红，z3 通过。

新的发现：

| # | 级别 | 发现 | 处置与提交 |
|---|---|---|---|
| S1C1-M1 | Minor | "照实"的草稿漏了 802、803 会改到的设计文字，其中一处会误导 P3：第 9 节的 P3 一条要 P3 去掉"`corner.tsx` 的那一段与 `shell.backToWorkspace`"，照做会连新建工作区页的"回到工作区"一起删掉（n04 那一格会红）；8.2 第 2 条只写了个人设置页的"回到工作区"；802 要改 4.1.4 的代码、决定 774 的一句、8.1 第 4 条与 10.1 的"第 0 步成立"；801 要改 4.3.1、6.3 提交 2 与 7.1"窄屏的查询"一行 | 本文档提交：第 9 节的 P3 一条改为只去掉个人设置页的那一个、留着决定 803 的；8.2 第 2 条加新建工作区页；4.1.4 的代码与决定 774 去掉 `overscroll-contain`；8.1 第 4 条与 10.1 注明第 0 步 (b) 由决定 802 取代；4.3.1、6.3、7.1 的 `narrow.ts` 由 S3 改为 S1；都列进 4.1 的"照实" |
| S1C1-N1 | Nit | `notebook-step.test.tsx` 的改法对，原因写得不对。"A private notebook in Lab"要等懒加载的步骤、`GET workspaces`、`GET notebooks` 都到了才画出来，进度行在步骤外面先出现，原来那一句同步的 `getByText` 本来就靠时序。C 的实测与实施者的相反：旧断言带着引入、`src/onboarding` 跑 10 次 0 次红，去掉引入 3 次红；notebooks 的应答延迟 30 ms 时旧断言 2 次都红、新断言 2 次都绿 | 接受：这是测试里潜在的竞态，与 user-menu 引入 `onboarding/steps` 无关（实施者的 9 次对 9 次是随机的偏差；产品里 `guards.tsx` 早已引入它，打包的产物不变）；`50e67f1c` 的说明里的归因以这里为准。下一行本来就 `await findByLabelText`，改成 `findByText` 与它一致，不掩盖什么 |
| S1C1-N2 | Nit | `profile-page.test.tsx` 新加的引导页一格，`await waitFor(() => expect(router.state.location.pathname).toBe("/onboarding"))` 在点之前就成立，证明不了重定向已经发生 | 接受：改等 `router.state.location.search` 为 `"?next=%2Fsettings%2Fprofile"`，再断言路径与焦点（`63810dff`，主持者；`heavy.sh make lint-web knip test-web` 绿，5157 格；n03 在收紧的这一格上红） |
| S1C1-N3 | Nit | 10.1 草稿的完整性：活下来的变异要补上 y04b 与只关布局的 z3、c03、c04；截图在会话的临时目录，要写明不进仓库、谁看过；"合并之前只有基线三格"对实施者那一次对，C 那一次 PG15 过了 | 并进 02-P2 10.1 |

A、B 没有新的发现。修复核对到此为止。

## 主持者的决定（负责人可以改判）

全文在 [02-P2](../02-P2-shell.md) 2.3。

- **801 窄屏的账户菜单把外观直接列在菜单里**（改 770）：md 以下（`(width < 48rem)`，与 `max-md:` 和 S3 的抽屉同一个查询）菜单是个人设置、分隔线、"主题"标签与三个选项、分隔线、"语言"标签与两个选项、分隔线、退出登录、分隔线、版本行，选项照旧 `preventDefault`；md 起照旧是子菜单。两套按条件渲染。`useNarrow()`（`W/app/narrow.ts`）与 `narrow.test.ts` 由此在 S1 建好，S3 只加调用者。
- **802 中段不再 `overscroll-contain`**（改 774）：滚到头带着窗口走，同 S1 之前侧栏还不是滚动区时；Firefox 只在另起一次滚动时带。第 0 步 (b) 的"侧栏滚到底、窗口不动"由本条取代。
- **803 新建工作区页也有"回到工作区"**：账户有工作区时，`/create-workspace` 的右上角放同一个链接（`arrived`）；没有工作区时不放（`/` 会落回这一页）。已登录的邀请页与出错页留给 P5。
- **S1A-N3 接受**：菜单开着时窗口跨过 md 的焦点问题少见，Esc 能恢复，不改。
- **`SessionChangedError` 的三处排除有意不由测试守**：`Corner`、工作区外框与 `LandingPage` 判断读取失败时都排除它（y03、y04、y04b）：一次读取被会话的更换打断是短暂的，下一代的守卫接手；去掉排除的后果只是短暂显示"外观"或交还右上角。

## 变异

每条命令先在没改的树上跑过、全绿；改坏前备份到临时目录，跑完从备份复原（02-workflow 3.1）。实施者写一条跑一条，之后在 `0986356e` 上把 vitest 的各条重跑一遍，在 `1d8414a9` 上把全部 44 条 vitest 与 4 条 e2e 再跑一遍；审查 C 在 `a9d7c030`、`1d8414a9` 上各独立重跑一次。

设计 7.2 的 S1 各条：

| # | 改坏 | 抓住它的 |
|---|---|---|
| 1 | `Layout` 不看 `handle`，右上角总在 | `layout.test.tsx`："一个账户按钮在侧栏里"、`/` 扣住列表、404 回首页、读取中等五格；`user-menu.test.tsx` 两格（两个"Ada"） |
| 2 | 工作区读取中也交还右上角 | `layout.test.tsx`"读取中没有 Ada" |
| 3 | `NotFoundPage` 时不交还 | `layout.test.tsx` 的 404 两格；e2e 的 a6 :42、:111 |
| 4 | `RouteError` 不交还 | `route-error.test.tsx` 两格 |
| 5 | 未登录时右上角不放"外观" | `layout.test.tsx` 未登录格与中文格；`route-error.test.tsx` 的第一格 |
| 6 | 外观的选项不 `preventDefault` | `user-menu.test.tsx` 的"仍开着"与窄屏两格；`routes.test.tsx`；`layout.test.tsx` 的主题格；e2e 的 a8（C 的 m06e） |
| 7 | "个人设置"不带 `arrived` | `profile-page.test.tsx` 两格 |
| 8 | 设置的 `h1` 不取到达的焦点 | 同上 |
| 9 | 头像的首字不 `aria-hidden` | `user-menu.test.tsx` 十三格；e2e 的 a6 三格、a8、s2、c6 :83 |
| 10 | 切换器放回 `nav` 里 | `workspace-layout.test.tsx` 的切换器格 |
| 11 | `shape: pill` 写成 `rounded-[999px]` | `button.test.tsx` 的胶囊格 |
| 12 | 子菜单内容加 `data-[state=closed]:animate-fade-in` | `dropdown-menu.test.tsx` 的子菜单格 |
| 13 | 亮色 `--sidebar` 调深成 `#e9e9eb` | se1 :252（亮色的框 2.555 < 2.74） |
| 14 | 切换器的"新建工作区"不带 `arrived` | `workspace-layout.test.tsx` 的新建工作区格 |
| 15 | "回到工作区"不带 `arrived` | `layout.test.tsx` 的设置格与新建工作区页格 |
| 46 | `useCornerBack` 的清理不减数 | `layout.test.tsx` 的 404 回首页格 |
| 47 | 右上角按 `account?.me` 判断（不读会话、不自己读 `me`） | `layout.test.tsx` 的会话启动中格；`route-error.test.tsx` 的首次载入格 |
| 48 | `SessionUnavailablePage` 不交还右上角 | `layout.test.tsx` 的 `GET /me` 答 500 格与 `/lab` 上会话不可用的格 |
| 65 | 在个人设置页上再选"个人设置"也设 `leaving` | `profile-page.test.tsx` 的设置格（焦点落到 `body`，不是"Ada"） |

实施者另写的与修复带来的：

| # | 改坏 | 抓住它的 |
|---|---|---|
| x1 | 右上角不看账户读取失败 | `layout.test.tsx` 的 `GET /me` 答 500 格 |
| x2 | 切换器的首字不 `aria-hidden` | `workspace-layout.test.tsx` 六格 |
| x3 | 账户行的菜单不向上开 | `user-menu.test.tsx` 的位置格（加这一格之前活下来） |
| x4 | 账户行挪到侧栏顶部 | `layout.test.tsx`、`workspace-layout.test.tsx` |
| x5 | 右上角的菜单不靠右 | 同 x3 |
| m801 | 窄屏仍用子菜单 | `user-menu.test.tsx` 的窄屏两格 |
| x6 | `useNarrow` 的查询写成 `(width < 40rem)` | `narrow.test.ts`；窄屏两格 |
| x7 | `useNarrow` 不听 `change` | `narrow.test.ts` |
| x8 | 外观的 `keepOpen` 什么也不做 | `user-menu.test.tsx` 的"仍开着"与窄屏两格 |
| x9 | 右上角的菜单不设宽度上限 | `user-menu.test.tsx` 的位置格 |
| n01 | index 路由不带 `handle` | `layout.test.tsx` 的"`/` 扣住列表"格 |
| n02 | `LandingPage` 读取失败时不交还 | `layout.test.tsx` 的"`/` 列表答 500"格 |
| n03 | `leaving` 不看引导步骤 | `profile-page.test.tsx` 的引导页格（`63810dff` 收紧之后照红） |
| n04 | 新建工作区页不放"回到工作区" | `layout.test.tsx` 的新建工作区页格 |
| n05 | 新建工作区页总放"回到工作区" | `layout.test.tsx` 的"没有工作区"格 |

审查者另写的：

| # | 谁 | 改坏 | 结果 |
|---|---|---|---|
| r01 | A | `onCloseAutoFocus` 里不把 `leaving` 复位 | 第一轮活下来；`1d8414a9` 之后由 `profile-page.test.tsx` 的"从 `/settings/security` 选 Settings、再开菜单按 Esc"格抓住 |
| r02 | A | 换代时把 `SessionChangedError` 也算失败 | 活下来；A 判为可以不补（后果只是短暂显示"外观"），同 y03 |
| r03 | A | 语言项不 `preventDefault` | 抓住（`routes.test.tsx`、`user-menu.test.tsx`） |
| r09 | A | `Corner` 不认 `unavailable` | 第一轮活下来；由 `layout.test.tsx` 的会话不可用两格抓住 |
| r13 | A | `Corner` 总读 `me` | 抓住（`guards.test.tsx`） |
| r25 | A | 设置的 `h1` 总取焦点 | 活下来；近乎等价：`SettingsLayout` 跨设置各页不卸下，effect 的依赖不变 |
| r28 | A | "回到工作区"只在 `/settings/profile` | 第一轮活下来；由"回到工作区"格（改从 `/settings/security` 开始）抓住 |
| r29 | A | `useCornerBack` 改用 passive effect | 活下来；jsdom 里等价，真浏览器里会不会闪一帧没有量（决定 793 的理由照旧） |
| r30 | A | 右上角在新建工作区页上从不读列表 | 抓住（新建工作区页格） |
| y01 | C | "回到工作区"出现在所有已登录的右上角 | 第一轮活下来；由 404 格与"没有工作区"格抓住 |
| y02 | C | 右上角用自己的 SWR 键 | 第一轮活下来；由会话启动中一格的 `GET /me` 计数抓住 |
| y03、y04、y04b | C | `Corner`、工作区外框、`LandingPage` 去掉对 `SessionChangedError` 的排除 | 活下来，有意不抓（主持者的决定） |
| y05 | C | `Corner` 不认 `unavailable` | 第一轮活下来；由会话不可用两格抓住 |
| y06 | C | `giveBack` 每次渲染新建 | 活下来；等价：同一次提交里减一又加一，状态不变，React 放弃这次渲染 |
| y07 | C | 切换器放到搜索之后 | 抓住（`workspace-layout.test.tsx`） |
| y08 | C | 账户行挪进中段的滚动区 | 活下来（`63810dff` 上重跑仍活）：账户行在底部是布局，jsdom 看不到；由截图与 A 的 D1（窗口滚动之后账户行留在视口里）看 |
| y09 | C | 去掉 `md:sticky md:top-0 md:h-svh md:self-start` | 第一轮只红了基线的 SE2；由 L1 的粘住守卫抓住（y 为 3083） |
| y10 | C | `useNarrow` 的清理不移除监听 | 第一轮活下来；由 `narrow.test.ts` 卸下之后的断言抓住 |
| y11 | C | 窄屏只对账户行内联 | 第一轮活下来；由窄屏格的 `/settings/profile` 一次抓住 |
| y16 | C | 设置的 `h1` 没有 `tabIndex` | 抓住（`profile-page.test.tsx` 两格） |
| y33 | C | 账户行头像的字换成空串 | 第一轮活下来；由 `toBeDefined` 抓住 |
| y34 | C | 工作区读取失败不交还 | 抓住（`layout.test.tsx`） |
| z1 | C | 只去掉 `md:sticky` | 抓住（L1 的 `toBeInViewport`，可见比例 0） |
| z3 | C | 只去掉 `md:h-svh` | 活下来：L1 的侧栏内容比视口矮，照样粘住、y 为 0；交 S2 的 PG16 第二格（滚动区的 `scrollTop` 大于 0） |
| z4 | C | a8 去掉指针的分步 | 抓住（a8 在 `light.click()` 上超时） |
| m06e | C | `keepOpen` 不 `preventDefault`（e2e） | 抓住（a8 :101：选了之后子菜单没了） |
| c02 | C | 右上角不读工作区 | 抓住（新建工作区页格） |
| c03 | C | 子菜单的 `collisionPadding` 改回 0 | 活下来：只关样子，jsdom 量不出；由实施者与 B 的实量看（离底 8 px） |
| c04 | C | 分隔线改回 `bg-border` | 活下来：只关样子；由强制色的截图看 |

## 实量与下限（Chromium，macOS，Playwright 1.63）

se1 在侧栏（`--sidebar`）上的三项照 02-P2 第 5 节重量（临时打印读数，不提交）；下限 = 实量 − 0.1，向下取到 0.01；注释写"re-measured at M13/P2/S1"。

| 项 | 亮：实量 → 下限 | 暗：实量 → 下限 |
|---|---|---|
| 搜索框的焦点 | 2.8431 → 2.74 | 8.4057 → 8.30 |
| "搜索"按钮的焦点 | 2.8431 → 2.74 | 8.4057 → 8.30 |
| 无效的框 | 4.0185 → 3.91 | 7.2832 → 7.18 |

范围下拉与命中链接在结果页上，读数不变（3.0977 / 7.9228），下限照旧（2.99 / 7.82）。无效框的亮色下限照实量取 3.91（第 5 节预期 4.02）。审查 C 按 WCAG 重算过各项，与注释里三位小数的读数相符；B 量到焦点在 `--sidebar` 上亮 2.84、暗 8.41，在白底（右上角）上亮 3.10、暗 7.92。

## 第 0 步与滚轮

第 0 步（00 号文档第 10 节；02-P2 6.1）：提交 2 改完、提交之前，经 `heavy.sh` 跑 `make build`，用 perf 的 `setup.ts`（`NW_PERF_SHAPES=flat3k`，导入一本 3000 页的笔记本，起生产配置的应用），临时的探针与三引擎配置放进 `e2e/perf/` 跑、跑完删掉（会话的临时目录，不进仓库）。

- **(a) 树分批画行**：冷开一页长文，正文一出现就把窗口滚到 1500。Chromium 2500 → 3002 行用 5 帧，Firefox 2100 → 3004 行用 6 帧，WebKit 1300 → 3006 行用 15 帧；每一帧窗口的 `scrollY` 都是 1500，侧栏的顶都是 0。
- **(b) 侧栏自己滚到底**：在滚动区上滚轮到底（Chromium、WebKit 51 次，Firefox 202 次），再多滚 10 次：滚动区到底（约 102,000 px），窗口的 `scrollY` 仍是 1500，侧栏的顶仍是 0。后由决定 802 取代（下面）。
- **(c) 读者之上长高**：读者的段落在窗口正中，其上的折叠 callout 在视口之外；打开它，长高约 824 px：窗口的 `scrollY` 随之 +823～824，读者的段落移动 Chromium 0.62 px、Firefox 0 px、WebKit −0.38 px。

三条在三种引擎里都成立；审查 C 读过 `results.jsonl`，A 的 D1、E2 另核粘住。

决定 802 之后（实施者在 `50e67f1c` 上，照 A 的 E1、E2 写法，1280×800，每次滚 400；A 在修复核对里另跑一次，结果相同）：

| 情形 | Chromium | Firefox | WebKit |
|---|---|---|---|
| E1 中段没溢出：依次在页上、中段、顶部、底部滚，`scrollY` | 400 → 800 → 1200 → 1600 | 同左 | 同左 |
| E2 中段溢出，窗口从 600 起，连续 12 次滚到中段尽头（1045） | 剩下的带着窗口走，到 4200 | 同一次滚动之内窗口不动，600 | 4200 |
| E2 停一下再滚 3 次 | 5400 | 1800 | 5400 |

修复之前 E1 的中段一项 Chromium 停在 400。

## 视觉的核对与探针

截图都在会话的临时目录，不进仓库。

- **实施者**（7.4 的 S1，Chromium，亮暗 × 1440、375）：工作区首页、账户菜单与外观子菜单、个人设置页、工作区的 404、长文滚到中间、登录页与它的外观菜单；决定 801 的 375 宽账户菜单（侧栏与右上角，选了中文之后）；修复之后的侧栏分组标题、切换器菜单、外观子菜单（离底 8 px、语言名与主题名同线）、右上角菜单（1440 下 256 px，375 下 x 107–363）、375 宽的内联菜单、新建工作区页的"回到工作区"、登录页的外观菜单、强制色下的账户菜单（分隔线 `1px solid`）。与样稿相符：侧栏 252 px、`--sidebar`、切换器的方形头像与 ⇕、30 px 的导航行与图标、灰色分组标题、宽屏滚到中间时侧栏粘住。有意的偏离：账户行上方的 `border-t`、头像一个字素（决定 770）。看到并已修的在"实施中的发现"与 B 的各条。
- **审查 B**：第一轮 Chromium 亮暗 × 1440、375 与强制色，Firefox、WebKit 在 1440 下的侧栏、账户行与菜单，逐项量：侧栏 252、底色与右边线、粘住；切换器、导航行、账户行、右上角（48 px 的条、胶囊、"回到工作区"）都等于样稿与 4.1 的类串；强制色下各控件键盘聚焦时 `outline: solid 2px`；对话框的遮罩盖住粘住的侧栏；`cn` 的合并实测不需要新登记。修复核对同样范围，另加强制色暗色、树与附件的菜单、决定 802 的滚动与决定 803 的右上角。
- **审查 A**（行为，Playwright 探针，三种引擎）：第一轮 A–G（右上角跨各路由与会话状态、账户菜单的键盘与指针、`arrived` 与焦点、窄屏的内联菜单、粘住与滚轮、375 与 320 宽的横向溢出、窄屏菜单里的键盘），修复核对另加 H–L（新建工作区页、`/` 的 `handle`、右上角翻到左边的子菜单）。

## 门禁与 CI

| 提交 | 实施者 | 审查 C |
|---|---|---|
| `0611c586` | `heavy.sh make lint-web knip test-web` 绿（5145 格，58 s）；`CI=1 make e2e` 594 过、3 败、1 跳，2.3 min（本地改过两次之后重跑；第一版 `60f7e582` 595 过、2 败，PG15 那次过了） | 绿，210 个文件、5145 格；e2e 594 / 3 / 1，2.3 min |
| `0986356e` | 绿（5145 格）；e2e 在第一版 `ffe38bbf` 上 594 / 3 / 1，2.5 min，`0986356e` 的由合并之前的整条命令跑（下表） | 绿；e2e 594 / 3 / 1 |
| `a9d7c030` | 第一版 `9abb0720` 绿（211 个文件、5148 格）、e2e 594 / 3 / 1，2.3 min；`a9d7c030` 的由合并之前的整条命令跑 | 合并之前的整条命令（下表） |
| `50e67f1c` | 绿（5153 格，55 s）；e2e 在 `5ec8c649`（应用代码与 `50e67f1c` 相同，只差 `notebook-step.test.tsx`）上 594 / 3 / 1，2.3 min，负载 4.6 → 31；`5ec8c649` 上的 `test-web` 两次都红在 `notebook-step.test.tsx` 那一格（S1C1-N1） | 绿，5153 格；e2e 在 `50e67f1c` 上 594 / 3 / 1，2.4 min |
| `1d8414a9` | 改过的 L1、A8 在 Chromium 里连跑 3 遍都绿；整套由合并之前的整条命令跑 | 绿，211 个文件、5157 格 |
| `63810dff` | — | 主持者：`heavy.sh make lint-web knip test-web` 绿，5157 格；只改测试，没有跑 e2e |

| 合并之前（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`） | 结果 |
|---|---|
| `0986356e`（实施者，决定 801 之前） | 网页一侧绿，210 个文件、5145 格；e2e 594 过、3 败、1 跳，三格是基线的 se2:231、pg15:113、lw8:836，消息相同；Playwright 2.3 min，整条 3 分 24 秒，负载 6–57 |
| `a9d7c030`（实施者；C） | 实施者：211 个文件、5148 格；e2e 594 / 3 / 1（基线三格，同行同消息），2.3 min；整条 3 分 20 秒，负载 17.8 → 62.9。C：同样的格数与三格，2.2 min，整条 3 分 19 秒，负载 20–93 |
| `1d8414a9`（实施者；C） | 实施者：211 个文件、5157 格；e2e 594 / 3 / 1（基线三格，同行同消息），2.2 min；整条 3 分 11 秒，负载 7.5 → 27.6。C：e2e 595 过、2 败（SE2 se2:231、LW8 lw8:836，同行同消息；PG15 这次过了）、1 跳，2.3 min；整条 3 分 18 秒，负载 6–78 |
| `2456d6ed`（主持者；加上文档提交的头，`m13-ui-refresh` 没有动过，即合并的树） | 211 个文件、5157 格；`gen-check-web` 之后树干净；e2e 594 过、3 败（基线三格 se2:231、pg15:113、lw8:836，同行同消息）、1 跳，2.2 min；整条 192 秒，负载 3 起；合并 `fdad6b9e` |

- 每次都只败基线里同行同消息的格；`gen-check-web` 之后树是干净的。
- Go 一侧：`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 每次以 0 退出；`git diff --check` 干净；分支是快进。
- E2E 用时：基线 3.1 min（负载 10 → 55），S1 之后 2.2–2.5 min（随负载）。
- vitest：5120（基线）→ 5145（提交 1，+25）→ 5148（提交 3）→ 5153（提交 4）→ 5157（提交 5）。
- **没有重现的 CI 项**：照 [01-P1](../01-P1-foundation.md) 7.3 的表（Go 一侧的全部（Go 的树没变）、`lint-actions`、`deploy-check`、`image-smoke`、`release.yml` 的演练、`vuln.yml`、Linux 与 macOS 的差别）；se2 在 se2:230 之后的步骤只在 Linux 的 CI 上走（决定 795(d)）。
- **CI**：本机没有推送的凭据（决定 746、761）：负责人推送之后在最后一个头上补跑。

## 接受与推后

- S1A-N3（菜单开着时窗口跨过 md）：接受，不改。
- `SessionChangedError` 的三处排除（y03、y04、y04b）与 A 的 r02：有意不抓。
- y08（账户行挪进滚动区）、c03（子菜单的 `collisionPadding`）、c04（分隔线的画法）：只关布局与样子，jsdom 看不到，由截图与实量看。
- z3（只去掉 `md:h-svh`）：交 S2 的 PG16 第二格。
- r25、r29、y06：等价或近乎等价。
- Radix 判断"指针是不是正往子菜单去"靠指针在主菜单上的横向移动：右上角的子菜单翻到左边时，Playwright 一步跳到项上会关上子菜单（三种引擎）；人的移动经过主菜单，不受影响（A 的 K、L）。不改，故事里分步移动指针（a8）。
- P3：给设置页侧栏之后去掉个人设置页的"回到工作区"，留着新建工作区页的（决定 803；02-P2 第 9 节）。
- P5：已登录的邀请页与出错页回到工作区的路（决定 803）。
- S3：抽屉复用 `useNarrow`；抽屉里的账户菜单同样是行内的外观。
- 审查 A 范围之外看到的：404 页的"Go to the home page"不带 `arrived`，点了之后焦点落到 `body`（S1 之前就有）：留给 P5（其余页面，同已登录的邀请页与出错页回去的路）。
- 没有实测的：真的 Chrome 加触控板（惯性、latching）下的滚轮（A；按 Blink 的逻辑应相同）；`useCornerBack` 改用 passive effect 时真浏览器里会不会闪一帧（A 的 r29）；窄变宽时 Chromium 把焦点落到"Sign out"的原因（A）；强制色模式只在 Chromium 里模拟过（B）；Linux 上的引擎。
