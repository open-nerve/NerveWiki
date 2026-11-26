# M13/P2 设计审查记录

| 项 | 内容 |
|---|---|
| 对象 | [02-P2-shell.md](../02-P2-shell.md) 的初稿（2026-11-22，未提交；`m13-ui-refresh` 的头 `4c3f10b9`） |
| 方法 | 三位审查者并行，代码只读，各自在会话的临时目录里跑探针（不进仓库）：A 管 S1 与 S3 的行为（右上角、账户菜单、抽屉、⌘K），以仓库自己的 React 19.3、React Router 8.4、`radix-ui` 1.6.7 搭了最小外框，在 Playwright 1.63 的三种引擎与 jsdom 30 里跑；B 管 S2 的全部与三个 Step 的外观，React 19.3 加 jsdom 模拟 effect 的次序，Tailwind 4.3.3 编译 `W/styles.css`，照设计的类名画出侧栏、对照样稿截图实量；C 管测试、e2e、变异、门禁、CI 余量、完成标准与文档文字，在自己的 detached worktree 里照设计做了 S1、S2 的原型、跑了 vitest，写了三个 jsdom 探针，数了 2.1 与 8.2 的锚点（worktree 已删） |
| 结论 | A：1 Important、6 Minor、3 Nit、1 Q；B：2 Important、5 Minor、7 Nit、1 Q；C：5 Important、12 Minor、17 Nit、3 Q。八条 Important 里有两对同一个问题：抽屉开着时在里面开页或搜索，困住焦点的 passive 清理晚于去向页 `h1` 的到达焦点，焦点被拉回、最后落到 `body`（A-I1、C-I1）；当前行滚进侧栏从第三次导航起就不做了（B-I1、C-M12）。其余是"更多"的图标 24 px（B-I2）、两个没列进去会红的 vitest 文件（C-I3、C-I4）与 2.1 一处丢了锚点的替换（C-I5）。主持者照下表定处置，全部接受，另有九条决定 790–798；设计者照它修订，复用审查者的探针重跑了改过的说法（见文末"修订时的复核"） |
| 定稿 | 设计审查与三轮修订核对之后定稿（2026-11-23）：修订核对第 3 轮里 A、C 都没有 Important、Minor（B 在第 2 轮就没有），主持者补上最后的三条 Nit（02-workflow 第 1 节第 7 步），见文末"修订核对第 3 轮" |

## 审查 A（S1、S3 的行为）

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P2A-I1 | Important | 照 782 的写法，在抽屉里开页或搜索之后焦点落到 `body`：困住焦点的监听由 Radix 在 passive effect 里拆，去向页的 `h1` 也在 passive effect 里取焦点；设计在地址变了的 layout effect 里 `setOpen(false)`，React 处理这个同步更新之前先冲掉上一次提交的 passive effect，`h1.focus()` 在困住还在时跑、被拉回抽屉，下一次提交 `inert`，落到 `body`（三种引擎；jsdom 停在链接上）。关上时给菜单按钮也在 layout effect 里，jsdom 里被拉回 | 接受，并入 C-I1：`trapping` 在渲染时由 `openedAt === location.key` 推出，`inert`、`data-drawer`、`role`、`trapped`、`loop` 都跟它；菜单按钮的焦点放进 passive effect，导航带 `arrived` 时不给（决定 790；4.3.4） |
| P2A-M1 | Minor | `FocusScope` 总给根 `tabIndex=-1`，宽屏上点侧栏空白处焦点落在根上，之后"只从无人处取"的标题不再取 | 接受：`tabIndex={trapping ? -1 : undefined}`；`shell-drawer` (6) 一格（决定 781；变异 58） |
| P2A-M2 | Minor | `motion-reduce:transition-none` 的特异度低于 `max-md:data-[drawer=open]:transition-[translate]`，减少动效时照样滑入 | 接受：只在 `motion-safe:` 之下滑动；设计者编译后另发现只挪属性不够（见"修订时的复核"）；UI1 第一格加减少动效的一步（决定 781；变异 59） |
| P2A-M3 | Minor | 快捷键挪到 `WorkspaceLayout` 之后，读取中、读取失败、404 时也 `preventDefault`；窄屏读取中按 Mod+K 会让侧栏以开着、困住而焦点不在里面的样子挂上 | 接受：没有框时直接返回，不 `preventDefault`、不开抽屉；`search-box.test` 一格（决定 783；变异 60） |
| P2A-M4 | Minor | 右上角把"还不知道"当成"未登录"：会话启动中先出现"外观"再换成"Ada"；`:slug` 上会话不可用时没人交还右上角 | 接受，并入 C-M1、C-Q1：右上角按会话定，自己以守卫的 SWR 键读 `me`，启动中与读取中不放；`SessionUnavailablePage` 交还（决定 792） |
| P2A-M5 | Minor | 变异 35 抓不到：`loop` 单独就让 Tab 回绕 | 接受，并入 C-I2：UI1 第一格与 `shell-drawer` (1) 加"以程序把焦点放到抽屉外、回到里面"；变异 35 照旧是"去掉 `trapped`" |
| P2A-M6 | Minor | `layout-error.test.tsx:18` 的 `banner` 断言在 S1 之后恒为空 | 接受：改为"没有 Appearance 按钮、也没有账户按钮"，列进 7.1 与 S1（决定 793） |
| P2A-N1 | Nit | 4.1.6 说点按子菜单会把焦点给第一项，实际只在键盘操作时 | 接受：改正 4.1.6 |
| P2A-N2 | Nit | 变异 42 的"UI1 第二格"抓不到（按键时抽屉关着） | 接受，同 C-N16：表里只留 `shell-drawer` (3) |
| P2A-N3 | Nit | 打开时在 layout effect 里聚焦，早于困住的监听，`FocusScope` 记不到这个焦点 | 接受：随 I1 的改法消失（打开的聚焦在 passive effect 里） |
| P2A-Q1 | Q | 00 号文档第 10 节"WebKit 没有 CSS 的滚动锚定"与设计者、A 的探针不符 | 主持者定：由 2.1 第 11 条照实改正，P4 在它自己的页上再核对 |
| 过度设计 | — | 根上 Esc 的 `!event.defaultPrevented` 是死代码；`arrivedAt` 认 `Located` 没有用处 | 接受：两处都去掉（决定 782、790） |

## 审查 B（S2 与外观）

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P2B-I1 | Important | 当前行滚进侧栏从第三次导航起就不做了：子的 layout effect 先于父的，行调 `reveal` 时读到的是上一次导航的看守，`end()` 的也是它；看守又算整页的任何输入；两个探针都是"4 次里滚 2 次"，PG16 与 vitest 都只做两次导航 | 接受，并入 C-M12：由 `PageTree` 自己的 layout effect 定，行只登记链接；看守按 `location.key` 懒建、只算 `[data-sidebar-scroll]` 里的输入；`location` 的 ref 在渲染时更新；vitest 加三次以上导航与"首页、按键、开页"两格，PG16 第二格加第三次导航（决定 791；变异 52、53） |
| P2B-I2 | Important | "更多"一排的图标是 lucide 默认的 24 px | 接受：链接带 `[&_svg]:size-4`，7.4 写明核对（决定 780） |
| P2B-M1 | Minor | 当前笔记本那一行与别的笔记本左右差 3 px，也偏离样稿 | 接受：两种行都 `gap-2.25`，⌄ 带 `-ml-0.5 -mr-1`，14 / 33 / 58 同样稿（决定 778） |
| P2B-M2 | Minor | 齿轮与＋在悬停整个容器（含整棵树）时就显出；悬停＋时笔记本行的底反而消失 | 接受：只在悬停笔记本那一行（或它们自己）时显出，同样稿；指针在＋上时行的底留着、首页上照旧是当前的底；偏离与对齐写进决定 778 |
| P2B-M3 | Minor | 地址里的笔记本 id 是大写时，`toLowerCase()` 让它的那一项画成空的 `SlotHolder`，这本笔记本从列表里消失 | 接受：不转小写；`SlotHolder` 只给列表里找得到的，备用位置只给编辑中留住的（决定 776；变异 49） |
| P2B-M4 | Minor | S3 之后 se1 的"button"量到的是 `--sidebar`，不是它画在上面的胶囊的底 | 接受，并入 C-M9（决定 794；见"修订时的复核"） |
| P2B-M5 | Minor | 笔记本行能点的只有中间 20 px，悬停的底却盖满整行 | 接受：链接盖满行高与左边距，⌄ 在链接里（决定 778） |
| P2B-N1 | Nit | 齿轮 14 px、＋ 16 px；搜索图标写的 `size-3.5` 被 `[&_svg]:size-4` 盖掉 | 接受：都 16 px，照实写 |
| P2B-N2 | Nit | 齿轮与＋的过渡不一致 | 接受：都 `transition-[opacity,color,background-color]` |
| P2B-N3 | Nit | ＋禁用时反而更亮，窄屏上看不出禁用 | 接受：`disabled:opacity-25!`（编译与实量核对过） |
| P2B-N4 | Nit | ⌘K 提示是等宽字体（preflight 给 `kbd` 的） | 接受：`font-sans` |
| P2B-N5 | Nit | 几处间距与样稿差 1–3 px | 接受：顶部 `space-y-2 … pb-1.5`、按钮 `left-1.75`、框 `pl-8.75`、账户行 `gap-2.25`、树的 `mr-2.25`、树的 `ul` 去掉 `gap-0.5` |
| P2B-N6 | Nit | 账户行上方的 `border-t`、头像一个字素，偏离样稿没写理由 | 主持者定：两处都留，作为有意的偏离写进决定 770 与 7.4 |
| P2B-N7 | Nit | 稳定容器那一格的第 (2) 条不可能成立（模态的对话框拿走焦点）；jsdom 挪动节点时也把焦点交给 `body`，那一格抓得住变异 18 | 接受：拆成 (2a) 焦点在按钮上时换组、(2b) 对话框开着打了字时换组 |
| P2B-Q1 | Q | LLM Wiki 的"目录""历史"在树里与"更多"里各标一次当前，两种样子、两种图标 | 主持者定：两处都留，同一个去处同一个图标，"当前"只有树的当前行那一种样子（决定 796） |
| 过度设计 | — | `keepFocus`、`takeFocus` 按 id 存取，同时只留一个容器时一个字段就够；看守算全页的输入 | 前者随决定 798（各 id 的容器都留）按 id 正合用；后者随决定 791 改 |

## 审查 C（测试、e2e、门禁与文档）

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P2C-I1 | Important | 抽屉关上时的焦点被 `FocusScope` 拉回：Esc 与遮罩（layout effect 里给菜单按钮时监听还在）、开页与搜索（同 A-I1）；UI1 三格与 `shell-drawer` (1)(4) 照设计会红 | 接受，同 A-I1（决定 790）；设计者改写了 C 的两个 jsdom 探针复核 |
| P2C-I2 | Important | 变异 35 抓不住 | 接受，同 A-M5 |
| P2C-I3 | Important | `layout-error.test.tsx` 的工厂 mock 只给 `Layout`，`RouteError` 从 `./layout` 引 `useCornerBack` 时 vitest 抛错，S1 会红 | 接受：右上角的上下文与 `useCornerBack` 放进 `W/app/corner.tsx`，`RouteError` 从那里引（决定 793）；连同 A-M6 列进 7.1 与 S1 |
| P2C-I4 | Important | `general-page.test.tsx:36-40` 断言笔记本设置页上侧栏的"Plans"是当前，S2 的 `NavLink end` 之后为 `null` | 接受：列进 S2，断言在设置页上笔记本行不带 `aria-current`，其余由 P3 的设置模式定 |
| P2C-I5 | Important | 2.1 第 3 条的第二处替换，锚点被第一处删掉了；新次序丢了 s2 的出处 | 接受：给出整句的旧文字与新文字，新的里写"子菜单里就地切换（选了之后菜单不关）"、留着 s2 的出处；第二处删去（2.1 第 4 条） |
| P2C-M1 | Minor | route-error 的新格过不了：首次载入就出错时守卫没画、`me` 没读过，右上角放"外观" | 接受，同 A-M4（决定 792）；新格照它写 |
| P2C-M2 | Minor | 门户的 key 与同级的 `<Outlet key={notebook.id}>` 撞 | 接受：`tree-${id}` |
| P2C-M3 | Minor | `routes.test.tsx:27` 在菜单开着时找不到"首页"（`hideOthers`） | 接受：先按 Esc；7.1 里"菜单仍开着"之后的断言逐格查过，只有这一处 |
| P2C-M4 | Minor | "读取中没有 Ada"那格可能在守卫的 Loading 时就空过（变异 2） | 接受：扣住 `GET workspaces`，等它发出之后再断言 |
| P2C-M5 | Minor | `useCornerBack` 清理时不减数，没有格抓 | 接受：`layout.test` 加应用内 404 的一格，变异 46 |
| P2C-M6 | Minor | 每个提交另跑的故事列得不全 | 接受：改了行为、夹具或故事的提交跑完整的 `make e2e`（决定 795(b)） |
| P2C-M7 | Minor | 合并门禁里只有 e2e 经 `heavy.sh` | 接受：整条命令经 `heavy.sh`（决定 795(c)） |
| P2C-M8 | Minor | 本机允许的失败是整格，会盖住 P2 的回退（se2:230 之后的步骤本机从不跑；PG15 [webkit] 是焦点与滚动的必需格） | 接受：S1 之前做基线，允许的失败要同行同消息；se2 那几步只靠 Linux 的 CI，写明（决定 795(a)(d)） |
| P2C-M9 | Minor | se1 的"button"量的不是它真正的底 | 接受，同 B-M4（决定 794） |
| P2C-M10 | Minor | UI1 的 Mod+K 本机与 CI 各只走一支 | 接受：`Control+K` 的 e2e 只靠 Linux 的 CI，本机由 vitest 走（决定 795(d)） |
| P2C-M11 | Minor | 认领了 35.7 却没有测试 | 接受：UI1 第三格加"375 宽下从搜索打开文件夹里的页，`h1` 有焦点且在视口里" |
| P2C-M12 | Minor | `PageRow` 的 layout effect 先于 `PageTree` 的 | 接受，同 B-I1（决定 791） |
| P2C-N1 | Nit | 新键没有看中文文字的格 | 接受：7.1 各格加简体中文的断言 |
| P2C-N2 | Nit | 变异 27 的格只在 `end` 无关紧要的地址上看 | 接受，改正对象：`end` 有关的是"历史"（`history/:changesetId` 是它的子路由，"目录"没有子路由）；格在 `…/history/:id` 上断言"History"不带 `aria-current` |
| P2C-N3 | Nit | jsdom 没有 `scrollIntoView`，`vi.spyOn` 会抛 | 接受：先赋上一个再 spy |
| P2C-N4 | Nit | 变异 11 的理由写错（tailwind-merge 去掉了 `rounded-lg`） | 接受：改正理由 |
| P2C-N5 | Nit | 2.1 还应改第 12 节 P2 一行的"状态" | 接受："设计定稿；S1 实施中"（P1 的先例，2.1 第 12 条） |
| P2C-N6 | Nit | 8.2 第 1 条"没有左栏的页面……放账户按钮"三处不准 | 接受：照决定 769、792 的分法重写 |
| P2C-N7 | Nit | UI1 的 `inert` 与"关上之后"的写法 | 接受：会重试的 `toHaveJSProperty`，正面的等待 |
| P2C-N8 | Nit | PG16 第二格"约多 2 秒"偏乐观 | 接受：S2 之后照实记 |
| P2C-N9 | Nit | 7.5 的"墙钟不到半分钟"没按 worker 数算 | 接受：照 4 vCPU、2 个 worker 改写 |
| P2C-N10 | Nit | 00 号文档 4.2.2 第 3 条"'工作区设置'带 `arrived`"没随 778 留给 P3 | 接受：2.1 第 2 条 |
| P2C-N11 | Nit | 13.2 第 20 条"看得见的搜索框是主要入口"在窄屏上不再成立 | 接受：8.2 第 8 条加一小条，S3 改 |
| P2C-N12 | Nit | S2、S3 的提交动了 `check.mjs`，作用域只写 `web, e2e` | 主持者定：写上 `tools` |
| P2C-N13 | Nit | `search-box.test.tsx:34` 的标题过时 | 接受：S3 改 |
| P2C-N14 | Nit | 2.1 第 8 条把新句插在 M12-polish 第 14 项的出处之前 | 接受：放到出处之后（2.1 第 9 条） |
| P2C-N15 | Nit | "三格都是 P1 之前就在的环境问题"，PG15 是 P1 里出现的 | 接受：7.3 改正 |
| P2C-N16 | Nit | 变异 42 的 UI1 第二格抓不到 | 接受，同 A-N2 |
| P2C-N17 | Nit | 2.1 第 7、9 条没写插入点的锚点 | 接受：写出（2.1 第 7、10 条） |
| P2C-Q1 | Q | 出错页上的右上角按会话判断登录、自己读 `me`，还是按 `me` | 主持者定：按会话，自己读 `me`（决定 792） |
| P2C-Q2 | Q | 26.8、26.10 是否一起认领；窄屏时框里的 Esc 会顺带关掉抽屉 | 主持者定：26.8、26.10 留给 v0.2；抽屉的 Esc 让给组字与有字的搜索框（决定 797） |
| P2C-Q3 | Q | M12-polish 第 8 项的 `onMac` 挪进 `lib/`，这次做还是留下 | 主持者定：S3 认领（决定 797） |
| 过度设计 | — | `slot.adopted`、`onAdopted` 的"后被收养"在产品里大概走不到 | 主持者定：走不到就去掉、留一句注释；设计者的探针表明走不到，去掉（决定 776，初稿的变异 46 随之去掉） |

## 未核实的怀疑（审查者列出，主持者的去向）

- 组字中按 Esc 连抽屉一起关上（A）：决定 797，抽屉的 Esc 让给 `composing`；`shell-drawer` (5) 一格。
- 在抽屉里点当前页那一行（A）：同一地址不带 `arrived`、照旧 `opened`，焦点给菜单按钮；设计者的探针 T8 核实（带 `arrived` 时三种引擎都落到 `body`）。
- 右上角的"个人设置"与 Radix 的还焦点（A）：照 `page-tree-item.tsx` 的 `chosen` 做法，选了会离开的项就 `preventDefault`（决定 770）；T7 两种写法三种引擎各 15 次都到 `h1`，是预防。
- 窄屏上主区开着模态菜单时按 Mod+K（A）：主持者没有分派；设计者的看法：S3 实施时在 UI1 之外探一次，成立的话让 `dialogOpen` 把开着的菜单也算上（只 `preventDefault`，宽屏上今天 Mod+Shift+F 的焦点也会被菜单拉回，看不出差别）。
- `containerFor` 在渲染里淘汰别的 id（B）：决定 798，各 id 的容器都留。
- 换组时开着的树菜单停在原处（B）：设计者核对过，今天不成立：换组总伴着列表换新，`PageTree` 与 `TreeMenu` 在 `NotebookNav` 的同一次提交里重画，`TreeMenu` 每次提交之后的重量在 `SlotHolder` 挪完容器之后才跑，菜单跟着按钮走；不另关它，`close` 不记成稳定的回调，(2c) 一格守着（决定 776，变异 51）。
- 换组之后还回的焦点在可见范围之外（B）：还焦点之后以 `reveal` 的 `scrollTop` 做法带进侧栏（`intoColumn`，决定 776）。
- 改放 passive effect 之后"总给菜单按钮"（变异 39）会不会成等价变异（C）：不会：`useDrawer` 在 `WorkspaceLayout` 里，它的 passive effect 在去向页的之后跑，菜单按钮赢、测得出；照决定 790 的规则，去向会取焦点的导航侧栏不动焦点，两者不抢（设计者的探针）。
- 宽屏上 `FocusScope` 的根 `tabIndex=-1`（C）：同 A-M1。
- `slot.adopted` 走不到（C）：设计者的探针核实，去掉。
- 离开工作区时 `onUnmountAutoFocus` 会不会抢焦点（C）：设计给了 `prevent`；`profile-page.test.tsx:47-56`（从 `/lab` 去个人设置，`h1` 有焦点）在 S3 之后照跑，抢了会红。
- Linux 的 Firefox 上 Ctrl+K 是浏览器自己的搜索栏（C）：UI1 在 CI 的 Linux 上跑 Firefox 的 `Control+K`，S3 推送之后看；吞掉时照 6.3 提交 2 的退路改按 `Mod+Shift+F`，写进记录。

## 主持者的决定

769–789 照初稿（有改写的见本文与设计的 2.3），新的 790–798，全文在 [02-P2](../02-P2-shell.md) 2.3（主持者的决定，负责人可以改判）：

- 790 抽屉的困住在渲染时推出，焦点在 passive effect 里给（A-I1、C-I1）；
- 791 当前行由 `PageTree` 自己的 layout effect 定，看守按导航懒建、只看侧栏里的输入（B-I1、C-M12）；
- 792 右上角按会话定，自己以守卫的 SWR 键读 `me`（A-M4、C-M1、C-Q1）；
- 793 右上角的上下文在 `W/app/corner.tsx`（C-I3、A-M6）；
- 794 se1 的"button"量它真正的底（B-M4、C-M9）；
- 795 P2 的本机门禁：基线、每个改行为的提交跑完整的 `make e2e`、合并前整条命令经 `heavy.sh`、只靠 Linux CI 的那几步（C-M6、M7、M8、M10）；
- 796 LLM Wiki 的"目录""历史"两处都留，同图标、同一种当前（B-Q1）；
- 797 26.8、26.10 留给 v0.2；抽屉的 Esc 让给组字与有字的搜索框；S3 认领 `onMac` 挪进 `lib/`（C-Q2、C-Q3）；
- 798 容器按笔记本 id 各留一个，渲染时不淘汰（B 的怀疑 1）。

## 修订时的复核

设计者照上面的处置修订 [02-P2](../02-P2-shell.md)，复用审查者的探针重跑了改过的说法（会话的临时目录 `p2-design/`，不进仓库；环境同审查）。与主持者的处置有出入的几处，理由与证据如下，主持者可以改判。

- **抽屉（决定 790）**：审查 A 的最小外框加一个照修订写的变体，A 的 T1–T7 在三种引擎与 jsdom 里都过；另加的 T8–T14：同一地址、后退到带 `opened` 的一条、有字的搜索框里按 Esc、以程序把焦点放到抽屉外（变异 35）、变异 39、组字中的 Esc、从 Mod+K 打开。两处细化：
  - 主持者写的"带 `arrived`/`opened` 的导航不给菜单按钮"，`opened` 只在后退、前进（`POP`）时排除：抽屉里点当前的地址照旧带 `opened`（`replace`），要给菜单按钮（主持者的另一条处置）；这与 `useArrivalFocus` 自己的 `arrival`、`fromNoOne` 一致。
  - 导航那次提交的 layout effect 先把抽屉里的焦点 `blur()` 交给 `body`（不 `setState`，不触发 I1 的那种提前冲刷）：不这样做时，后退到带 `opened` 的一条，Chromium 里标题取不到焦点（焦点还在已 `inert` 的切换器上，Chromium 到下一次渲染才修正），最后落到 `body`；Firefox、WebKit 取得到。加上之后三种与 jsdom 都到 `h1`（变异 54）。
- **C 的两个 jsdom 探针**按决定 790 改写成一个 vitest 文件，四格都过：树的链接到 `h1`、首页到菜单按钮、Esc 到菜单按钮、变异 39 时菜单按钮赢（不是等价变异）。
- **右上角（决定 792）**：C 的 `zz-route-error-probe` 加一个照决定写的右上角：首次载入就出错时是"Ada"、`GET /me` 只一次；平常的页上 2.5 秒里不多读；`GET /me` 答 500 时是"外观"。细化：主持者写的是"未登录放外观、已登录放账户、启动中与读取中不放"；会话不可用（`unavailable`）与账户读取失败两种不在其中，`SessionUnavailablePage` 交还之后右上角会是空的，A-M4 情形 2 要的主题与语言仍然没有；所以这两种也放"外观"。
- **当前行（决定 791）**：B 的 `reveal-order` 照决定改写：收养之后五次导航（其间在主区按键、点树的行、在侧栏里按 ↓），有行的五次都滚；在侧栏里滚轮之后晚到的行不滚，在主区滚轮之后照滚。
- **收养**：`:slug` 懒加载、`Layout` 先有侧栏的位置时，同一次提交里 `SlotHolder` 的 layout effect 先于树的；只有工作区与 `Layout` 在同一次提交里第一次画出（不懒加载）时，树的 effect 才先跑。产品里走不到，去掉。
- **se1 的"button"（决定 794）**：主持者给的做法是把胶囊的底放到包着框与按钮的那层上、框透明。照 `contrast.ts` 的 `measure` 在 Chromium 里量了三种结构：初稿（底在框上）"button"量到侧栏（亮 2.84、暗 8.41，实际 3.10、6.95）；主持者的做法"button"量对了，可"box"与"invalid"的环画在框外、落在侧栏上，`under(input.parentElement)` 却读到那一层，反而量错（亮 3.10、4.38，暗 7.31、6.33，实际 2.84、4.02、8.41、7.28）；改为框照旧自己带底、按钮放进一个与框同色的垫底 `span`（跟着框的聚焦换色），三项都量对（亮 2.84、3.10、4.02，暗 8.41、6.95、7.28），看不出垫底。主持者的处置留了退路（"不行就记下局限、另算真实的一对"）；这里的问题不是框的样子，是另两项被量错，垫底的做法三项都量对，所以不退到记录局限。
- **抽屉的滑动（决定 781）**：照 A 的建议把属性放进 `motion-safe:` 不够：另写的 `duration-150` 留在外面，减少动效时计算值是 `transition: all 150ms`（`transition-property` 回到初值 `all`），连 `translate` 一起动；改为只写 `max-md:data-[drawer=open]:motion-safe:transition-[translate]`（时长用它自带的 150 ms），减少动效时 `transition-duration` 是 `0s`（Chromium 实量）。
- **有字的搜索框里按 Esc（决定 797）**：Chromium、Firefox 先清空、抽屉留着，再按才关；Playwright 的 WebKit 不清空，只是"让给它"的话 Esc 在那里失了效。所以浏览器没清空时（下一个任务里框还是有字）照常关上。
- **换组时开着的树菜单（B 的怀疑 2）**：主持者写的是"成立就关上"。读代码核对的结果是今天不成立：读取与改动都给 `this.list` 新的对象（`notebook.store.ts:96,327`），`NotebookLayout`、`PageTree`、`TreeMenu`（`close` 每次渲染新建）与 `NotebookNav` 在同一次提交里重画，`SlotHolder` 的 layout effect 在侧栏的子树里、先跑，`TreeMenu` 每次提交之后的重量（`page-tree-item.tsx:389-406`）随后量到按钮的新位置。所以不关；它靠的是 `close` 每次新建这件顺带的事，写一句注释、加一格 (2c) 与变异 51 守着。没有在浏览器里跑过。
- **外观的实量**（`row-probe.mjs`，Tailwind 4.3.3 编译 `W/styles.css`，Chromium）：两种笔记本行的 ⌄、书本、名字都在 14 / 33 / 58；行的链接高 30 px；齿轮、＋、"更多"的四个图标都是 16 px；齿轮与＋静止 0.4，悬停行、＋、齿轮都是 1，悬停树的行仍是 0.4；指针在＋上时行的底是 `--hover`，首页上是 `--active`；发送中的＋是 0.25；`kbd` 是无衬线。
- **2.1 的脚本**：`rev00-p2.py`（会话的临时目录）在 `git archive m13-ui-refresh` 的副本上 `--check` 与实跑都过，再跑一遍 `--check` 报出已不在的锚点、什么也不写。

## 修订核对第 1 轮

| 项 | 内容 |
|---|---|
| 对象 | 照上面修订的 [02-P2](../02-P2-shell.md) 与本记录（2026-11-23，都未提交），2.1 的脚本 `rev00-p2.py`；代码仍是 `4c3f10b9` |
| 方法 | 三位审查者同前分工，先逐条核对上一轮的发现，再找新的。A 重跑设计者的 `probe-a/run-rev.mjs`（三种引擎）与 `jsdom-rev.mjs`，另写三个探针：`blur` 的理由、新的滑动写法的编译、编辑中的"留下"（设计者的外框加 `useBlocker` 与 Radix 的 `AlertDialog`）。B 重跑设计者的 `reveal-791.cjs`，照修订的类名重画整条侧栏，三种引擎里截图、实量、量对比度，另有 hash 导航、`:has` 的性能与树里提示的探针。C 在新导出的 `git archive` 上跑 2.1 的脚本（`--check`、实跑、再 `--check`）与 diff，把 8.2 的旧文字与新文字全部从修订后的 8.2 里取出来按 S1→S2→S3 模拟，在自己的 detached worktree 里跑设计者 `probe-c` 的两个文件、另写一个 `useMatches` 的探针（worktree 已删），逐行走了变异表 |
| 结论 | 上一轮的发现全部已处理（A 的 11 条、B 的 15 条与 3 条怀疑、C 的 37 条），C 的 M7、M8 各留一处尾巴（下表 P2C1-M2、M3）。没有 Important；A 2 Minor、2 Nit，B 1 Minor、5 Nit，C 4 Minor、5 Nit。主持者全部接受，另有两条决定 799、800；设计者照它再修订，复用审查者的探针重跑了改过的说法（本节末"再修订时的复核"） |

### 审查 A

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P2A1-M1 | Minor | 窄屏、编辑中有未保存的改动，在抽屉里点树的另一页被拦下，选"留下"：确认框的 `onCloseAutoFocus` 先把焦点给 `main` 里的编辑器，Radix 之后才恢复抽屉那一层，抽屉仍开着、遮罩还在，焦点在它后面，打的字进了被遮住的编辑器（三种引擎；初稿就有） | 接受：决定 799，`WorkspaceLayout` 经上下文给出"关上抽屉、不给菜单按钮"，`UnsavedGuard` 在"留下"时先调它；`shell-drawer` (7) 一格；变异 63 |
| P2A1-M2 | Minor | `open`、`closed` 读渲染时的 `location.key`：监听照今天只注册一次时，应用内导航之后 `open` 记下旧的 key、`trapping` 恒为假，Mod+K 被 `preventDefault` 而抽屉不开；计划里的格都在第一页上按 | 接受：几个函数经 ref 读当前的 key 与 `trapping`、在 `useDrawer` 的一生里不变（决定 790，4.3.3、4.3.4）；`shell-drawer` (3) 加"在抽屉里点 Home 之后 Mod+K 仍打开、框有焦点"；变异 64 |
| P2A1-N1 | Nit | 已在个人设置页上从右上角再选"个人设置"：同一路径的 `replace`，标题的 effect 不重跑，`leaving` 又挡住 Radix 还焦点，落到 `body` | 接受：去处就是当前的地址时不设 `leaving`（决定 770，4.1.3）；`profile-page` 的格加一步；变异 65 |
| P2A1-N2 | Nit | 修订去掉了 `Located`：抽屉开着时后退到带 `Located` 的一条，菜单按钮先拿走焦点，附件行不再取 | 接受：`headingTakes` 算上 `Located`（决定 790；设计者改为不论导航的类型，见"再修订时的复核"），由 `arrival.ts` 导出；`arrival.test` 一格；变异 66 |

A 的未核实：
- 导入的环（`corner.tsx` → `guards.tsx` → `session-unavailable.tsx` → `corner.tsx`）：接受，以结构避开：`useSession` 挪进新的 `W/app/session-state.ts`（决定 793）；设计者核对过 oxlint 启用的类别里没有 `import/no-cycle`。
- 主区开着模态菜单时按 Mod+K、Mod+Shift+F：留作 S3 实施时的探针，写进 6.3 提交 1（预期与不成立时的做法）。
- 只在 jsdom 里会出现的抢焦点（抽屉里切换器选了工作区之后 Radix 把焦点还给切换器）：写进 7.1 `shell-drawer` 一行，提醒写 vitest 的人；"在抽屉里切换工作区"只在 UI1 里核对（C 的未核实）。

### 审查 B

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P2B1-M1 | Minor | 同一页换 hash（点右栏的大纲、正文里本页某一节的链接、后退前进到同一页的另一个 hash）是一次新的 `location.key`：`PageTree` 的 effect 为它新建一个没动过的看守，读者滚开的侧栏被拉回当前行（`reveal-791-hash.cjs`；修订带进来的，初稿行的 effect 依赖 `[active]`，不会这样） | 接受：只在路径变了时定，同一页换 hash 或查询参数、`replace` 到当前的地址算同一次导航（决定 779、791，4.2.5）；`page-tree-reveal` 一格；变异 62 |
| P2B1-N1 | Nit | S3 亮色"button"的新下限：3.0977 − 0.1 向下取到 0.01 是 2.99，不是 3.00 | 接受：决定 794、变异 61、第 5 节 |
| P2B1-N2 | Nit | 指针在＋上时行有了悬停的底，名字却还是灰的 | 接受：行加 `…:group-has-[[data-notebook-new]:hover]/notebook:text-foreground`（决定 778，4.2.3） |
| P2B1-N3 | Nit | `group-has-[…:hover]` 编译出来不在 `@media (hover: hover)` 之下，触屏上点过＋之后行的底留着 | 接受：行的两条另套 `[@media(hover:hover)]:`（决定 778）；齿轮与＋的透明度在触屏上本来是 100%，不套 |
| P2B1-N4 | Nit | 树的 `nav` 去掉间距之后，失败的提示贴着上下的行 | 接受：树里的 `Alert` 加 `my-1`（决定 780） |
| P2B1-N5 | Nit | 读者没有＋，行仍留着 `pr-14`，长名字早约 26 px 截断（初稿就有） | 接受：读者 `pr-8`（决定 778，4.2.3） |

B 对 776 的注释的小建议（把 `create` 也写上，或只写"靠 (2c) 守着"）：由决定 800 定，换组改靠通知，注释写明不靠回调。B 的另两条未核实（触屏上的"粘住"没有真机；收养先于树的 layout effect 只从 fiber 次序推断）照旧：前者由 N3 的改法消去，后者有设计者的 `slot-order.cjs` 与 PG16 第二格。

### 审查 C

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P2C1-M1 | Minor | 变异 51 是等价变异：只稳住 `close`，`create` 每次渲染仍新建，`TreeMenu` 照样在换组那一次重画、重量，(2c) 照样通过；真正依赖的是 `TreeMenu` 跟着 `PageTree` 重画 | 接受：决定 800，换组由 `SlotHolder` 通知 `TreeMenu` 重量；变异 51 改为"不通知"；(2c) 就地改，确保没有顺带的重画救它（设计者没有稳住代码里的回调，见"再修订时的复核"） |
| P2C1-M2 | Minor | 每个提交的 `make lint-web knip test-web` 不经 `heavy.sh`，与 06-environment 第 3 节不合，`markdown.test.ts` 的"not slower"会误报 | 接受：决定 788、795(b)、第 6 节开头、7.3 |
| P2C1-M3 | Minor | 基线记的"行"没定是哪一行，第 1 节写的都是格的开头行：同一格里更早的一步坏在同一种断言上，回退会被当成允许的失败 | 接受：记 Playwright 报告里错误所在的那一行与消息的第一行（决定 795(a)、6.1、7.3）；第 1 节的三格在基线之后照它改写 |
| P2C1-M4 | Minor | 00 号文档第 11 节的决定 731、733 与改过的 4.2.5、4.2.3 说法相反，修订没有改也没说取代 | 接受：2.1 第 13 条在两条末尾注明；决定 769、790 写明改了哪几个字 |
| P2C1-N1 | Nit | 00 号文档 4.2.3"已登录"一条仍写"工作区之外与工作区之内的出错页与 404" | 接受：2.1 第 1 条第 5 小条 |
| P2C1-N2 | Nit | `onMac` 的调用者是五个，`shortcuts.ts` 不调用它；照"从那里引入"会留下没用的导入 | 接受：决定 797、4.3.1、4.3.3 |
| P2C1-N3 | Nit | `initial.test` 的"é → É"：`toLocaleUpperCase` 不合成，结果是分解的形式 | 接受：7.1 的期望值照写分解的形式 |
| P2C1-N4 | Nit | 变异 55 括号里的"落到 `body`"与决定 782 的理由只对一半：原来那一条不带 `arrived` 时，`arrival` 由假变真、effect 重跑，焦点到 `h1` | 接受：变异 55 的说明与决定 782 照 C 的写法改 |
| P2C1-N5 | Nit | `shell-drawer` 在 jsdom 里要看 `hasAttribute("inert")`；(4) 的带 `opened` 的那一条要另造 | 接受：7.1 写明两处 |

C 的未核实：
- 抽屉里切换工作区，焦点会不会被切换器抢走：UI1 第一格加一步（真浏览器；jsdom 没有 `inert`）。
- (2c) 只在 jsdom 里核对：照旧。
- Linux 的 Firefox 上的 `Control+K`：照旧，退路在 6.3 提交 2。
- S1–S3 的本机总时长：7.5 加一段粗算（只在本机）。

### 主持者的决定

全文在 [02-P2](../02-P2-shell.md) 2.3（主持者的决定，负责人可以改判）：

- 799 被"未保存"拦下、选了"留下"时抽屉安静地关上：`WorkspaceLayout` 经上下文给出不把焦点给菜单按钮的关上，`UnsavedGuard` 在"留下"时调它，焦点留在确认框还给的地方（A 的 P2A1-M1）；
- 800 换组时树的菜单由 `SlotHolder` 通知重量，不靠顺带的重画；变异 51 改为去掉通知（C 的 P2C1-M1）。

### 再修订时的复核

设计者照上面的处置再修订 02-P2，复用审查者的探针重跑（会话的临时目录 `p2-design/`，不进仓库）。与处置有出入的几处，理由与证据如下，主持者可以改判。

- **决定 799**：A 的 `r1/` 加一个 `QuietClose` 上下文（`probe-a-r1/run.mjs`）。三种引擎里，不关时与 A 的复现相同（"留下"之后焦点在 `#editor`、抽屉开着、打的"xyz"进了被遮住的编辑器）；安静地关上时抽屉关上、焦点在 `#editor`。之后的 Tab 走页面自己的次序（探针的最小页上编辑器之后没有可聚焦的，Chromium、WebKit 到 `body`）。A 的另一个建议"点链接时若有拦截先关上抽屉"不取：抽屉不知道有没有拦截，选"离开"时也不该先关。
- **P2A1-M2**：同一个外框里 Ctrl+K 的监听只注册一次。经 ref 读时，在抽屉里点"首页"之后 Ctrl+K 打开抽屉、焦点在框里（三种引擎）；读渲染时的值时抽屉不开、焦点留在菜单按钮，复现了 A 的说法。
- **P2A1-N2，`Located` 不论导航的类型**：主持者写的是"后退、前进到带 `Located` 的一条"。附件行与"找不到"的标题对带 `Located` 的一条都会取焦点，不看类型（`attachments-section.tsx:531-537`、`not-found.tsx:22`），所以 `headingTakes` 只看状态；从抽屉里实际只有后退、前进走得到它，两种写法在产品里一样，只看状态的更简单。`arrival.ts` 不从 `attachment-row.tsx` 引 `locatedOf`，另写一个不导出的 `isLocated`：`arrival.ts` 在主包里，`attachment-row` 在懒加载的包里（`routes.tsx` 的 `lazy`），引了会把它连同下拉菜单的部件拉进主包。
- **P2B1-M1，"同一次导航"而不是"只记下 key"**：B 建议同一页换 hash 等只记下 key、不滚。照字面做（把新 key 记成已定）时，树还没读到就换了 hash，行到了不再滚。改为这些导航不算新的一次：要定的那一次导航（`at`）只在路径变了时更新，看守与"至多一次"跟着它。`probe-b/reveal-r1.cjs`（B 的 `reveal-791-hash.cjs` 加这条规则）：在 p60 上于侧栏里滚轮、再到 `#section`，回到不带 hash、`replace` 到当前的地址，侧栏都不动；树没读到时换 hash，行到了照滚，其间在侧栏里滚过轮时不滚；之后开 p30 照滚；原来的五次导航与晚到的行照旧（滚的是 k1、k2、k3、k4、k6、h5、g1、w2、x1）。
- **决定 800，`observer` 加 `memo` 下行得通；回调不稳住**：`probe-b/slot-move-r1.cjs`（jsdom；`TreeMenu` 是 mobx-react-lite 的 `observer`，`NotebookLayout` 是 `memo`；`Slot` 一个计数，`TreeMenu` 以 `useSyncExternalStore` 读它）六种情形：列表换新（新的对象）、回调每次新建时，通知与不通知都量到"team"（不通知时靠重画）；列表换新、回调稳住时，不通知不量（C 说的将来），通知量到；就地改（同一个对象，(2c) 的做法）时，通知量到，不通知不量（变异 51 被抓）。通知在 `SlotHolder` 的 layout effect 里发出，`TreeMenu` 在同一次提交里、绘制之前重画，没有依赖的重量的 effect 随之量到新位置。主持者写了"之后 `TreeMenu` 的回调可以稳住"，设计者没有稳住：列表重读时同一组里上面多了或少了一本，容器不挪、`SlotHolder` 不发通知，按钮却移了，今天（M12）靠的是回调每次新建带来的重画；稳住它们就是上面"回调稳住、不通知"那一种，菜单留在旧的位置。所以由测试隔开这条路（(2c) 就地改 `workspace_access`，`PageTree` 不读它），代码里的回调照旧。
- **导入的环**：oxlint 1.86.0 照仓库的 `.oxlintrc.json`（类别 correctness、suspicious、perf）对一个两个文件的环只报了另放的一个未用变量；加 `-D import/no-cycle` 才报"Dependency cycle detected"（`p2-design/cycle/`）。`useSession` 的调用者是 `guards.tsx`、`event-stream.tsx`、`pages/invitation.tsx`，测试里没有 mock `./guards` 的；`sign-out.tsx`、`session-unavailable.tsx`、`user-menu.tsx` 都不引 `guards` 与 `corner`，挪走之后没有环。
- **笔记本行**（`probe-b/row-probe-r1.mjs`，Tailwind 4.3.3 编译 `W/styles.css`，Chromium）：指针在＋上时行的底 `rgba(15, 16, 20, 0.043)`、字 `rgb(24, 25, 28)`，同悬停行；行的两条在编译出的 CSS 里都在 `@media (hover:hover)` 之下；别的读数同上一轮。
- **设计者另发现的一处**：决定 795(b) 与第 6 节的"`make e2e` 本机约 2.5 分钟"只算了 Playwright；`make e2e` 先 `make build`（`Makefile:81`），P1/S2 的记录是 build 152–172 秒加 Playwright 2.5–2.8 分钟，改为约 5 分钟。7.5 的本机总时长照它算。
- **2.1 的脚本**：`rev00-p2.py` 加三处（1e 出错页的说法，13a、13b 决定 731、733 的注），改两处的新文字（5 加 `Located` 与"留下"，12b 第 13 节的一行），共 27 处。在新导出的 `git archive m13-ui-refresh docs` 上 `--check` 报 `check ok: 27`，实跑报 `revised: 27`（diff 21 段，新的落在 4.2.3"已登录"一条与第 11 节的 731、733），再跑 `--check` 拒绝（18 处旧文字已不在，什么也不写）；仓库上 `--check` 报 `check ok: 27`。
- **8.2**：第 6、7 条的新文字改了（同一次导航；`Located` 与"留下"）。设计者的 `sim82.py` 与 C 的 `sim82b.py`（从修订后的 8.2 里取全部旧文字与新文字）按 S1→S2→S3 都通过。

## 修订核对第 2 轮

| 项 | 内容 |
|---|---|
| 对象 | 照修订核对第 1 轮再修订的 [02-P2](../02-P2-shell.md) 与本记录（都未提交），2.1 的脚本（27 处）；代码仍是 `4c3f10b9` |
| 方法 | 分工同前。A 重跑设计者的 `probe-a-r1/run.mjs`，另写 `ws.mjs` 核对 UI1 新加的"在抽屉里换工作区"（Radix 的单选项，指针三种引擎各 10 次、键盘 Chromium 与 Firefox 各 5 次）。B 重跑 `reveal-r1.cjs`、照设计原文写 `slot-move-r2.cjs`，照修订的类名重画侧栏、三种引擎截图实量（含触屏模拟），另写 `reveal-r2.cjs`、`reveal-transition.cjs`、`put-identity.cjs`。C 在新导出的 `git archive` 上跑 2.1 的脚本、`sim82b.py`，与设计者存下的旧稿逐字比对，逐行走新的变异，以仓库的 `.oxlintrc.json` 对设计写的几个 hook 跑 oxlint 1.86.0（`p2-rev-c/r2/lint/`） |
| 结论 | 第 1 轮的发现全部已处理。没有 Important；A 1 Minor、1 Nit，B 3 Nit，C 1 Minor、5 Nit。主持者全部接受，没有新的编号决定（下一个是 801）；设计者照它再修订（本节末"再修订时的复核"） |

### 审查 A

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P2A2-M1 | Minor | "目录""历史"两行是 `NavLink`，`state` 在渲染时就定了；`inDrawer()` 经 ref 读、抽屉开关时 `PageTree` 不重画，带的总是 `opened`，抽屉里点它焦点去了菜单按钮，与决定 782 不符；没有一格点它们（初稿就有） | 接受：两行改在点按时定，同树的行（`onClick` 里 `linkFollowed` 判断之后 `preventDefault`、`navigate(to, { state: inDrawer() && !here ? arrived : opened, replace: here })`）（4.2.3、4.3.4、决定 782）；`shell-drawer` (4) 加 LLM Wiki 的"Index"一格；变异 67 |
| P2A2-N1 | Nit | `headingTakes(location, type)` 与 `headingTakes(location.state, type)` 两种写法 | 接受：各处 `headingTakes(state, type)`（4.3.4） |

A 的 `ws.mjs`：在抽屉里换工作区之后焦点都在菜单按钮、抽屉关上（不论 Radix 的 `setTimeout(0)` 先于还是后于导航的提交，切换器那时都已 `inert`）；WebKit 的键盘那一路 A 没跑通，不是应用的问题。

### 审查 B

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P2B2-N1 | Nit | 同一路径时提前 `return`：行恰好画在换 hash 的那次提交里（`show` 看到 `settled` 还是旧的 key、不调），或换 hash 的渲染被挂起时先提交的一块行里，之后没人再调 `reveal`，当前行不滚（修订带进来的；`reveal-r2.cjs` 的 A 格、`reveal-transition.cjs`） | 接受：照 B 的写法总调 `reveal()`，路径变了才更新 `path`、`at`（4.2.5、决定 791）；另见"再修订时的复核"的 insertion effect |
| P2B2-N2 | Nit | 8.2 第 6 条"读者自这次导航起有过动作"不准（只算侧栏滚动区里的输入）；"同一页的 `replace` 算同一次导航"与 13.2 第 26 条到达的看守的说法相反，容易被人统一掉 | 接受：照 B 的写法改，并注明与到达的看守不同 |
| P2B2-N3 | Nit | 决定 800 不稳住回调的理由只对列表整个重读成立：`put`、`gone` 留着别的笔记本的原对象，同一组里的位移今天就不重量（不是回归） | 接受：机制照决定 800；理由改为"列表整个重读时"，记下 `put`、`gone` 的情形照旧、接受 |

B 的另三条未核实（P2B2-N1 能否在应用里碰上；换组时 Radix 的菜单在真浏览器里是否贴着挪走的按钮；版本页规范化修订号的 `replace` 换了路径、算新的一次导航）照旧，不改设计。

### 审查 C

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P2C2-M1 | Minor | 照设计写的 hook 过不了零警告的 oxlint：渲染时写 ref（`react/refs`），`useSearchShortcuts` 与 `useDrawer` 的几个 effect 依赖不全（`react-hooks/exhaustive-deps`、`react/exhaustive-effect-dependencies`）；最顺手的修法（把 `useDrawer` 的返回值写进依赖）让监听随抽屉开关重新注册，变异 64 成了等价的 | 接受：`useDrawer` 返回一生不变的 `actions`，`useSearchShortcuts` 只拿它、依赖 `[mac, field, actions]`；别的 effect 写全依赖；791 的 ref 改在 `useInsertionEffect` 里（探针核实）；第 4 节的 hook 照写进探针文件、零警告跑过；豁免列在 4.4；7.2 写明变异 64 为什么仍有意义 |
| P2C2-N1 | Nit | `shell-drawer` (4) 造带 `opened` 的那一条之后少了"再开抽屉"，变异 54 唯一的 vitest 格测不到 | 接受：7.1 |
| P2C2-N2 | Nit | `page-tree-reveal` 的同一页换 hash 格，桩的 `top` 不随 `scrollTop` 时抓不到变异 62 | 接受：7.1 写明"行的偏移 − 滚动区的 `scrollTop`" |
| P2C2-N3 | Nit | 决定 794 的 S1 暗色下限 8.4057 − 0.1 应是 8.30，不是 8.31 | 接受 |
| P2C2-N4 | Nit | 8.2 第 7 条只写了"焦点留在编辑器" | 接受：照 2.1 第 5 条写"编辑器、横幅或冲突的标题" |
| P2C2-N5 | Nit | 4.3.1 工作区外框那一行没写 `useDrawer` 与上下文的 provider | 接受 |

C 的未核实："`useInsertionEffect` 的做法"由设计者的探针核实（下面）；"`newOnboardedTeam` 能不能给出建第二个工作区要用的 PAT"：能，它返回 `teamOf` 的结果（含 `pat`）再加 `tokens`（`e2e/fixtures/workspaces.ts:89-102`），UI1 第一格的 `createWorkspace` 用它；02-P2 第 5 节照此写明。

### 再修订时的复核

- **`useInsertionEffect`**（`p2-design/probe-r2/insertion.cjs`，React 19.3.0、jsdom）：父组件以 `useInsertionEffect` 写 ref，子的行以 `useLayoutEffect` 读，树门户进另一个节点。新挂上的行、`active` 变了的行都在同一次提交里读到这次的 key（insertion effect 在突变阶段跑，全部 layout effect 在其后），只有一行自己重画的提交里读到上一次提交的；挂起的过渡（Suspense 挡着，没有提交）里一行自己重画时，insertion effect 写的读到提交了的 key，渲染时写的读到没提交的（B 的 P2B2-N1 情形二）。所以采用它，不用豁免。B 的 `reveal-r2.cjs`、`reveal-transition.cjs` 加一个"总调 `reveal()`、key 在 insertion effect 里"的变体：A 格滚，B–D 格同原来；过渡里那一块行一提交就滚（B 的写法要等换 hash 的提交）；第 1 轮的 `reveal-r1.cjs` 照同样改（`reveal-r1-r3.cjs`），滚的仍是 k1、k2、k3、k4、k6、h5、g1、w2、x1。
- **oxlint**：第 4 节写到的 hook 与 JSX 照写进 15 个文件（`p2-design/lint-r2/web/apps/web/src/` 下，目录照 `W/`，复制仓库的 `.oxlintrc.json`）。在 `lint-r2/` 里跑 `node_modules/.bin/oxlint -c .oxlintrc.json --max-warnings=0 web`（oxlint 1.86.0）：第一次只报两条 `react(set-state-in-effect)`（导航之后与变宽时的 `setDrawer(null)`），审查 C 报的三类一条也没有；变宽改在渲染时按上一次的宽度清掉（`search-box.tsx:83-88` 的做法），导航之后的一处加豁免（理由在 02-P2 4.4）；再跑以 0 退出、没有输出。仓库现有的 `page-tree-item.tsx`、`page-tree.tsx`、`search-box.tsx`、`arrival.ts` 用同一个命令也以 0 退出（校准）。
- **jsx-a11y 的豁免不要**：同一配置下，字面的 `role="dialog"` 报 `prefer-tag-over-role`，条件表达式的 `role={trapping ? "dialog" : undefined}` 不报；不带 `aria-hidden` 的遮罩报 `click-events-have-key-events` 与 `no-static-element-interactions`，带 `aria-hidden` 的不报。初稿 4.3.4 写的三条豁免去掉；4.4 列的是一条新的与两处沿用的。
- **与处置不同的地方**：
  - 处置写的是"`CloseDrawer` 的 provider"。设计没写过 `inDrawer` 怎样从 `WorkspaceLayout` 到 `PageTree` 的 `context`（树在 `Outlet` 里，抽屉的状态在外框）；P2A2-M1 的改法又要它。所以 `CloseDrawer`、`useCloseDrawer` 并成一个 `Drawer` 上下文与 `useDrawerActions()`，给出的就是那份一生不变的 `actions`：`UnsavedGuard` 调 `quietClose`，`PageTree` 取 `inDrawer`。决定 799 的做法不变，只是名字与载体。
  - 处置写的是"别的 effect 写全依赖"。`useDrawer` 的导航 effect 原来读 `drawer` 的 ref `was`，写全依赖时要么在渲染时写那个 ref（又是 `react/refs`），要么依赖 `drawer` 本身；取后者，多跑的几次条件不成立。变宽的 effect 改成渲染时的判断，比再加一条豁免少一处例外。
  - `reveal` 的看守原来每次换成新的对象，卸下时的清理若在 effect 开头取下 `watch.current`，结束的是最初那个对象的（`undefined`）读者；改为同一个对象只改字段。这是写全依赖时顺带看到的。
- **变异 64 仍有意义**：`actions` 由没有依赖的 `useMemo` 建，一生不变；`useSearchShortcuts` 的依赖 `[mac, field, actions]` 不变，监听只注册一次。变异是"`actions` 的函数不经 `now`、读建它时的值"：第一次渲染时 `trapping` 为假、key 是第一页的，"Home 之后 Mod+K"时 `closed()` 照旧为真、`open` 记下旧的 key，抽屉不开，`shell-drawer` (3) 红。"让 `actions` 随这些值新建"是另一种对的写法（监听随之重新注册），不是这条变异。
- **2.1 没有改**：这一轮的改动都在 02-P2 自己的各节与 8.2（第 6、7 条的新文字）；`rev00-p2.py` 不动，仓库上 `--check` 仍报 `check ok: 27`。8.2 用设计者的 `sim82.py` 与 C 的 `sim82b.py` 重跑，都通过。

## 修订核对第 3 轮

| 项 | 内容 |
|---|---|
| 对象 | 照修订核对第 2 轮再修订的 [02-P2](../02-P2-shell.md) 与本记录（都未提交）；代码仍是 `4c3f10b9` |
| 方法 | A、C 核对自己第 2 轮的发现与它们带出的改动；B 第 2 轮已没有 Important、Minor，不再起，它的三条 Nit 由主持者对着 4.2.5 与 8.2 第 6 条核对 |
| 结论 | A：P2A2-M1、N1 已改，新的 1 Nit；C：P2C2-M1、N1–N5 已改，新的 2 Nit；没有 Important、Minor。主持者补上这三条 Nit（02-workflow 第 1 节第 7 步），设计定稿 |

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P2A3-N1 | Nit | 导航之后的 passive effect 用闭包里旧的 `drawer` 判断、`setDrawer(null)`：提交与这个 effect 之间按下的"打开"会被吞掉 | 主持者补上：写成 `setDrawer((d) => (d !== null && d.at !== location.key ? null : d))`，豁免照旧在这一行（4.3.4、4.4）；只是时机，不设测试格，正文一句 |
| P2C3-N1 | Nit | `reveal` 的看守在卸下时只 `end()`：StrictMode 模拟的卸下再挂上之后，`reveal()` 接着用一个已结束的读者 | 主持者补上：清理里 `watching.reader?.end(); watching.reader = undefined;`（同 `arrival.ts` 的 `useArrivalWatch`，4.2.5）；`page-tree-reveal` 一格（树没读到、看守开着时离开笔记本，`readersInput` 的四种事件各 `removeEventListener` 一次）；变异 68 |
| P2C3-N2 | Nit | `shell-drawer` (6) 只看变宽，没看再变窄时抽屉不自己开 | 主持者补上：(6) 加"`matchMedia` 再答窄屏：根带 `inert`、不是 `dialog`，焦点没有动"；变异 69 |

设计者照写之后，把 4.4 的探针文件同样改了再跑一次 oxlint（`lint-r2/`，`--max-warnings=0`），以 0 退出；2.1 没有改，`rev00-p2.py` 在仓库上 `--check` 仍报 `check ok: 27`。
