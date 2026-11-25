# M13/P5 设计审查记录

| 项 | 内容 |
|---|---|
| 对象 | [05-P5-pages.md](../05-P5-pages.md) 的初稿（2026-11-25，1,022 行，未提交；`m13-ui-refresh` 的头 `f62d0390`）。审查与修订期间分支的头走到 `3e288acb`（P4 之后 CI 的两处修复，[04-P4](../04-P4-reading.md) 10.4），定稿的设计落在它之上提交 |
| 方法 | 三位审查者并行，代码只读，各自在会话的临时目录与用完即删的分离工作树里跑探针（不进仓库）：A 管三个 Step 的行为（决定 912–939），对着 `f62d0390` 的代码逐条核对，在分离工作树 `nw-p5-rev-a` 里跑了五个 vitest 探针（`sectionAt`、子页面数与索引的重建、版本页的横幅、历史行链接之后的焦点、knip），在设计者的参照页 `ref-history.html` 上用 Playwright 1.63 的三种引擎量了 `revealClass`，用 tailwind-merge 3.7.0 核对了 `cn` 合并之后的类名；B 管三种引擎里的样子，照设计第 4 节逐字抄类名写了 29 种探针页（外框照 `layout.tsx`、`shell-sidebar.tsx`、`corner.tsx` 抄，中英两版，图标用 lucide-react 画），以网页自己的 Tailwind 4.3.3 与 `styles.css`（含 `reading.css`）编出，在 Chromium、Firefox、WebKit 里 1440/1024/375/320 宽、亮暗、中英实量与截图（700 多张），强制色在 Chromium、Firefox 里另跑，照 `emailFor` 的名字扫了右栏版本行的"-"（200–240 px 逐 1 px），把设计的 83 串类名放进工作树的 `src/p5probe.ts` 经 `heavy.sh` 跑 `design-tokens.test.ts`，`cn` 的合并以 `extendTailwindMerge` 加 P1 的登记在 node 里核对，se1 用 `contrast.ts` 的 `measure` 原样量；C 管测试、e2e、变异、门禁、CI 余量、完成标准与文档，以 `apply82.py` 依次在 `f62d0390` 的总体设计上做 8.2 的十五条替换、逐条数旧文字与新文字，在三种引擎里量了卡片的高与页宽（`probe/cards.mjs`、`cards-nogrid.mjs`），以 `mobx-computed.mjs` 数 MobX 计算值被观察时重建的次数，从第 4 节抽出 231 串类名跑 `design-tokens.test.ts`，在 `loading.tsx` 加一个只在本文件用的导出跑 knip，跑了改过的 `page-tree-scale.test.tsx:243` 与 `node tools/e2e-projects/check.mjs`（单个测试都经 `heavy.sh`） |
| 结论 | A：2 Important、4 Minor、8 Nit、2 Q；B：2 Important、12 Minor、8 Nit、2 Q，另一条过度设计；C：3 Important、9 Minor、1 Nit（一组十一处零碎）、2 Q，另有顺手看到的三件。七条 Important 里 A-I2 与 C-M1 是同一处（`iconTileClass` 在 S1 提交 2 只在本文件用，knip 报未使用的导出；C 按 Minor 记），A-I1 与 C-M5 合并处置（横幅进列之后各支的格）。按 Minor 或 Nit 处理的同一个问题：变异 15 是等价的（A-N1、C-M3），`revealClass` 的 `transition-opacity` 经 `cn` 换掉按钮的 `transition-colors`（A-N2、B-N1），子页面列表里的名字是截短、不是折行（A-N5、B-M4、C-M7），`column-top.tsx:5` 的注释（A-N8、C-N1 的一项）。其余 Important 是去掉回退之后版本页的读取中、找不到两支没有横幅，逐路由的格不渲染"找不到"的各支（A-I1）、404 与出错页的"回到首页"因 `buttonVariants` 不经 `cn` 成了 8 px 的圆角（B-I1）、各列表页的行标题 14 px 对 16 px、有的带下划线（B-I2）、320 宽的两段 `expectFits` 拿常量 375 比，在对的实现上也红（C-I1）、变异 6 写给的那一格比卡片的高、抓不到它（C-I2）、2.1 的几处旧文字照写脚本会失败（C-I3）。主持者照下表定处置，全部接受，另有决定 940–945；设计者照它修订 |
| 定稿 | 设计审查与五轮修订核对之后定稿（2026-11-25）：B 在修订核对第 2 轮、A 在第 4 轮都没有新的发现，C 在第 5 轮没有 Important、Minor；第 3 轮 A、C 的两条 Minor（其中一条两人同报）与两条 Nit、第 4 轮 C 的一条 Minor 与一条 Nit、第 5 轮 C 的一条 Nit 由主持者改进设计（02-workflow 第 1 节第 7 步；第 3、4 轮的 Minor 都用 C 给的文字，改了之后交下一轮核对），见文末"修订核对第 3 轮"到"修订核对第 5 轮" |

## 审查 A（三个 Step 的行为）

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5A-I1 | Important | `VersionPage` 在 `VersionShell` 之前有两支早退（`version-page.tsx:76-78` 树读取中或读不到、`:79-82` 页不在或版本号不合法），4.3.1 只在 `VersionShell` 的列里画 `<ColumnTop />`；`columned` 只看路由的 `handle`，从 S3 提交 1 给版本路由加 `handle` 起这两支就没有横幅，提交 2 去掉回退之后固定下来（探针：升级中、`nodesDown` 时横幅 0 个，打开不存在的页的版本，"Page not found"上 0 个；今天回退在，两处都是 1）：直接打开版本的地址时树读到之后横幅才插进来，标题下跳（P4 的决定 857 要的"不闪"）。7.1 的遍历格只渲染读取中、失败，没有"找不到"的各支（版本页的 `PageGone`、`changeset-page.tsx:69`、`:92`，`trash-preview.tsx:66`、`:249`、`:270`），"失败"也没写让哪个读失败；S2 加 `handle` 的六个路由在 S2 只核对读到的状态，漏了一支要到 S3 提交 2 才查得出。4.3.1、4.3.6、7.1、第 0 节 | 接受：去掉回退之前，每一处早退都要在列里画 `<ColumnTop />`（`VersionPage` 的 `NotLoaded`、`PageGone` 之前，画在 `VersionPage` 里，同 `page-layout.tsx:76-93`；`changeset-page.tsx:69`、`:92`、`:95`；`trash-preview.tsx:66`、`:249`、`:270`、`:307`，以 `f62d0390` 的行号逐处列出）；横幅的格改成从 S1 起的一个"路由 × 支"的表（与 C-M5 合并），每格断言恰好一个横幅、在第一个 `h1` 或 `alert` 之前、同在那一页的列里；读取中、失败（写明让哪一个读失败）、找不到各支都渲染；各支随加 `handle` 的那个 Step 进表；变异表加"版本页 `PageGone` 之前不画"等 |
| P5A-I2 | Important（C 的领域） | `iconTileClass` 在 S1 提交 2 导出，别的文件要到提交 3 的 `StatusPage` 才用它；knip 照默认不把只在本文件的用处算作用处，探针照 4.1.1 写出 `empty-state.tsx` 跑 `knip --treat-config-hints-as-errors`，报 `Unused exports (1) iconTileClass`，提交 2 单独不绿（02-workflow 3.1）。4.1.1、4.5、第 0 节"knip 不报" | 接受（同 C-M1）：S1 提交 2 不导出 `iconTileClass`（提交 3 再导出），或让它在提交 2 就有别的文件用；"knip 不报"照实改 |
| P5A-M1 | Minor | React Router 8.4 的匹配不分大小写，`sectionAt` 分；决定 923 说"slug 与 id 大小写不同时外框自己就 404"，管不到 `settings` 与页那一段（个人设置的地址里没有 slug）：`/settings/Tokens`、`/lab/settings/Members`、`/lab/SETTINGS/members` 整页报错（`useSettingsHeading is used outside SettingsPage`），`/settings/Profile`、`/lab/settings/General` 画出来了、没有 `h1`。第 1 节 (e) 也写轻了：今天带尾随斜杠的 `tokens/`、`members/` 是整页报错，只有 `profile/`、`general/` 是没有 `h1`；4.1.7 的改法修好了尾随斜杠 | 接受：`sectionAt` 与 `useSettingsHeading` 比较前去掉末尾斜杠并转小写（或设置的子路由 `caseSensitive: true`，设计者定，写明理由）；第 1 节 (e) 照实写今天的情形（带斜杠是整页报错）；7.1 的格加一个用 `useSettingsHeading` 的页（tokens 或 members）与大写的地址；改写 923 的理由 |
| P5A-M2 | Minor | 决定 924 只给回收站预览的"See the jobs"加 `arrived`，漏了同类的两处：变更集页的（`changeset-page.tsx:344`，`startUnread`）与回收站页的（`trash-page.tsx:274`），后台的答复丢了、任务读不到时它们随左栏离开，导入导出页的 `h1` 不取焦点，焦点落到 `body` | 接受：`changeset-page.tsx:344`、`trash-page.tsx:274` 的"See the jobs"加 `state={arrived}`，各配一格与一条变异，进 924 与 8.2 |
| P5A-M3 | Minor | 4.2.8 第 1 条让 `shell` 的列以 `<ColumnTop />` 起头，第 3 条又让 `Gone` 的 `section`"进同样的列（`ColumnTop` 在前）"；`NotInTrash` 与 `Gone` 都经 `shell(...)` 画，照字面是两个 `<output>` 横幅、两层 `md:-mt-3`，实施者只能偏离设计 | 接受：`Gone` 留在 `shell` 的列里，只换 `h1` 的类名，不自画 `ColumnTop` |
| P5A-M4 | Minor | 版本页的"还原这个版本"进了头一行，要等版本与版本列表（另一个 SWR 键）都读到才出现；375 宽时排不进一行，版本列表晚到时头一行从一行变两行，`h1` 与正文下移约 36 px（按类名推算），别人还原成最新时又上移；与 P4 的决定 894"行数只看宽度"、P3 的决定 848"出现的东西预留位置"不一致。宽屏不跳，变更集页的"撤销"与 `h1` 同时出现、不跳 | 接受：版本页窄屏上给能编辑的人预留"还原"那一行（照 848 的教训，例如按钮在 `newest` 读到之前以 `invisible` 占位，或面包屑的 basis 让出位置），设计者定；7.4 加"扣住版本列表"的截图 |
| P5A-N1 | Nit | `PageTreeStore.tree` 是 `makeAutoObservable` 的 getter，即 computed（`page-tree.store.ts:161-183`、`:240-243`），首页的 observer 读着它就一直缓存；探针 1,000 个顶层页、每十个带两个子页，两种写法画完都只调 `indexTree` 1 次，加一个子页之后也是 1 次，变异 15 是等价的；第 1 节引的 `page-tree.ts:240-243` 那个文件只有 191 行 | 接受（同 C-M3）：变异 15 判等价，写明依据（`PageTreeStore.tree` 是首页 observer 保活的 computed，`page-tree.store.ts:161-183,240-243`；探针读一千次只建 1 次索引），删去"量时间"，改正文件路径；风险 4 随之改 |
| P5A-N2 | Nit | "只看此人"是 `Button` 加 `revealClass`，tailwind-merge 3.7.0 把 `transition-colors` 与 `transition-opacity` 当作同一组，留下后者，悬停时底色与字色的 150 ms 过渡没了 | 接受（同 B-N1）：`revealClass` 用 `transition-[opacity,color,background-color]`（同 `page-tree.tsx:39-40`） |
| P5A-N3 | Nit（风险 3 的答复） | 版本页与变更集页的主操作挪到 `h1` 之前，到达时焦点在 `h1`，从它按 Tab 会跳过主操作；4.3.8"次序其实相同"只对从页首 Tab 成立。挪过去可以接受：同 P4 的"编辑"（决定 865、894），主操作统一在头一行右边，`RevertDialog` 自己的 `useFocusLeaving`（`revert-dialog.tsx:98`）不丢交接 | 接受：变更集页"撤销这次改动"挪到 `h1` 之前可以接受（同 P4 的"编辑"，决定 865、894）；改掉"次序其实相同"一句（从 `h1` 起 Tab 会跳过主操作），进读屏清单 |
| P5A-N4 | Nit | 子页面数与成员数没有照语言区写（`index.inbound` 传的是 `toLocaleString(locale)`，`index-rows.tsx:102-103`），过千时写成"1000 subpages" | 接受：子页面数与成员数照语言区写（`toLocaleString`） |
| P5A-N5 | Nit（C 的领域） | 4.1.4 的 `SubpageRow` 是 `truncate`（今天也是），决定 931、UI1 第 4 格的标题（"broken in its row"）与 8.2 第 4 条却写"折行"；UI1 第 4 格只 `expectFits`，截短、折行都过，写进总体设计的那句会是错的 | 接受（同 B-M4、C-M7）：子页面列表与卡片照代码截短（与今天一样，行尾的数留位置）；931 改成"卡片与子页面列表的行截短，其余列表的行与标题折行"；UI1 第 4 格的标题、8.2 第 4 条同改；第 4 格要真抓得到截短（C-I2 的写法：断言长名字的 `span` 一行且 `scrollWidth > clientWidth`）；它的 `REQUIRED` 两行与 TB1 的"WebKit 由 UI1 第 4 格覆盖"照实重写或去掉 |
| P5A-N6 | Nit | 子页面数在链接之外，浏览模式读得到；按 Tab 一个个走链接、或打开读屏的链接列表时只听到名字 | 接受：子页面数用 `aria-describedby` 指向一个 `hidden` 的句子（按 Tab 的读屏用户也听到），或照实进读屏清单，设计者定 |
| P5A-N7 | Nit | 虚线卡只在列表不空时画；它的对话框开着时列表被别处变空（另一位管理员删了最后一本、或本人被移出），虚线卡卸下，对话框连同打了的名字一起没了，焦点落到 `body`；今天空状态的对话框反方向已有同样的事，不是新类型 | 照实：虚线卡的对话框开着时列表变空、它随卡片卸下，与今天相同，写明 |
| P5A-N8 | Nit | 变异 56 记"e2e 不量它"不对：菜单开着时焦点在 portal 里、指针移开之后也不悬停，只有 `aria-expanded` 让触发按钮显着，Chromium 的 e2e 以 `toHaveCSS("opacity", "1")` 量得到（参照页上三种引擎都量到 1）；`W/app/column-top.tsx:5` 的注释也提到 `handle: { column: true }`，S3 提交 2 要一并改 | 接受：变异 56 的"e2e 不量它"改正（菜单开着、指针移开时 Chromium 的 e2e 量得到，见 C-M4）；S3 改 `column-top.tsx:5` 的注释 |
| P5A-Q1 | Q | 历史行的主链接、"Reverted"与"Reverts …"、回收站行、版本页的作者、变更集页的"See it""Reverts …""Reverted by"、升级页通往页面的链接都不带 `arrived`，随页离开之后去处的 `h1` 不取焦点（探针：历史页点一行的主链接，变更集页的 `h1` 出来之后 `activeElement` 是 `BODY`）；13.2 第 17 条的规则，不是 P5 引入的，但 P5 正好改写这几行。P5 顺带加，还是交 v0.2 | 主持者定（决定 940）：P5 改写的行与头里通往别处的内容链接（历史行、回收站行、版本页的作者、变更集的"撤销了/被撤销"、升级页通往页面的链接）照 13.2 第 17 条补上到达的状态（`arrived` 或该去处需要的），随改写那一页的 Step；配格（例如一格逐个链接断言 `state`）与变异；进 8.2 |
| P5A-Q2 | Q | 已登录的邀请页两个按钮是接受与退出登录，不接受就回不到自己的工作区，只能用浏览器的后退（进"设置"再"返回"也回到邀请页，决定 834）；00-M13 4.2.3 把这条路留给 P5，决定 921 的理由不成立。把 `Corner` 的 `back` 扩到已登录、有工作区、在 `/invitations/:id` 上，还是照 921 不加、改写理由 | 主持者定（决定 941）：已登录的邀请页右上角照决定 803 加"回到工作区"（`Corner` 的那一个，去最近的工作区，没有时去首页，带 `arrived`）；921 的理由改；配格 |

## 审查 B（三种引擎里的样子）

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5B-I1 | Important | 404 与出错页的"回到首页"写 `className={buttonVariants({ variant: "outline", size: "sm", shape: "pill" })}`：cva 只拼类名、不经 tailwind-merge，串里同时有 `rounded-lg` 与 `rounded-full`、`text-sm` 与 `text-compact`，样式表里 `.rounded-lg` 在后，三种引擎实量都是 8 px 的圆角，旁边经 `cn` 的"Reload"是胶囊；字号 13 px 只是 `.text-compact` 恰好排在后面。P1–P4 的"cn 合并、样式表次序"那条教训。截图 `errorRaw-*` | 接受：链接写 `cn(buttonVariants({…}))`，或在 `button.tsx` 导出已合并的 `buttonClass(...)`（设计者定，优先后者：不导出裸的 `buttonVariants`）；变异表加"去掉 `cn`"，由一格 vitest 读类名里没有 `rounded-lg` 守 |
| P5B-I2 | Important | 各列表页行的第一行不是一种（Chromium 实量）：搜索的命中 14 px、无下划线；历史、目录 16 px（链接在没写字号的 `p`、`li` 里）；回收站的行、变更集的改动 16 px、有下划线；健康的项 14 px、400、有下划线；P3 设置的行 14 px、无下划线。只有搜索用了 `lineTitleClass`；目录的摘要写成 13 px 灰的 `factsClass`，搜索的摘录留 `text-sm` 正文色。与 00-M13 4.6"卡片里的行"、4.4.2 的 14 px、4.7"列表里的链接不加下划线"相矛盾，用户在搜索、历史、回收站之间切换看到三种行。截图 `rows-*` | 接受：各列表页行的第一行一律 `lineTitleClass`、链接一律 `lineLinkClass`（14 px、500、悬停才有下划线）：历史、目录、回收站行（`trash-row.tsx:500-509`）、变更集的改动（`changeset-change.tsx:106-134`）、健康的项（`health-item.tsx:58-60`）；目录的摘要与搜索的摘录同一种样子；7.4 加"各列表的一行并排"的截图；守：至少一格（vitest 读类名或 e2e 量字号）跨几页核对同一种行 |
| P5B-M1 | Minor | 笔记本首页取 928 px：没有卡片的网格，只有一列行与附件一节，1440 宽时行尾的子页面数离名字约 850 px；进一页时内容列从 928 变 720、左边从 x=382 跳到 486；附件一节在首页与页的底部不一样宽；设计者自己的参照页用的是 720 | 接受：笔记本首页用 720 的列（`listColumnClass`），928 只给工作区首页；916 改 |
| P5B-M2 | Minor | 小于 640 px 时卡片一列，没有"同一行"可对齐，`min-h-28` 只在名字与"开放程度 · 成员数"之间留约 40 px 的空，五本笔记本在 375 宽占约 650 px | 接受：卡片与虚线卡 `sm:min-h-28`（一列时随内容高） |
| P5B-M3 | Minor | 子页面列表的行链接铺满整行、没有圆角，全局的 `:focus-visible` 外框偏移 2 px，第一行与最后一行画出直角的外框，盖在卡片 14 px 的圆角外、左右各冲出约 3 px（Chromium、Firefox，亮暗；WebKit 在 macOS 上 Tab 不走到链接） | 接受：行链接加 `rounded-[inherit] focus-visible:-outline-offset-2`，外框收进卡片 |
| P5B-M4 | Minor | 同 A-N5：决定 931"行里与标题里的名字折行"与 4.1.4 的 `truncate`、UI1 第 4 格的标题、8.2 第 4 条互相矛盾，探针三种引擎都是一行加省略号 | 接受，同 A-N5 |
| P5B-M5 | Minor | 右栏版本行单独一行的"-"可以复现，第 1 节 (b)"没有复现"不成立：照 `page-versions.tsx:116-139` 的真类名，40 个照 `emailFor` 造的名字 × 四种时间 × 有没有"当前"，217 px（xl 时的内容宽）时 320 行里 Chromium 8、WebKit 68、Firefox 0 行，200–240 px 逐 1 px 是 256、920、96 对，全在带"当前"或"编辑中"小块的行；`wrap-anywhere` 次数不变，每个"-"之后加 `<wbr>` 更糟；设计者只试了两种名字、两种时间，没有扫宽度 | 主持者定（决定 943）：右栏版本行单独一行的"-"可复现。设计者在探针里试 B 的改法（小块内联在"· Edit"之后，同版本页的事实一行），三种引擎 200–240 px 逐 1 px 扫，若"-"不再单独一行，进 S3（右栏的版本行，P4 的文件，S3 改）；否则把交 v0.2 的文字照 B 的数据改成"可复现" |
| P5B-M6 | Minor | 回收站行的文字一块没有 `flex-1 basis-*`，它的宽就是最长一行事实；长的"来自"路径在 1440 宽把"恢复"挤到下一行最左，同一列表别的行在右上；P3 为此给了 `listTextClass`（P3S2B-M1） | 接受：回收站行的文字一块用 `listTextClass` |
| P5B-M7 | Minor | 标题行的 `h1` 没有 `flex-1`，长的笔记本名让它占满一行，`justify-between` 把"新建页面"放到下一行开头（1024 宽 `buttonLeft` 276，在标题之下 51 px；375 宽也在左下）；健康页同一写法 | 接受：标题行的 `h1` 加 `min-w-0 flex-1 basis-56`（按钮留在右边），笔记本首页、健康页同；回收站预览的"恢复"同理（B-Q2） |
| P5B-M8 | Minor | 历史行的"Reverted"照旧 4 px 圆角、12 px、带下划线（`history-row.tsx:107-112`），同一行的"Trimmed""Ingest"换成全圆角、11 px 的 `Mark`，一行里两套小块 | 接受：历史行的"Reverted"用 `Mark` 普通那种的形状，悬停才有下划线 |
| P5B-M9 | Minor | 三种文档类页 32 px 标题下的事实写成三种：版本页一行 12.5 px `text-faint`，变更集页的 `ChangesetFacts` 照旧 `space-y-1 text-sm`、多行 `text-muted-foreground`，回收站的预览 13 px 的 `factsClass` | 接受：三种文档类页标题下的事实取一种（版本页那种），变更集只换类名、结构照旧（`nextElementSibling` 的格） |
| P5B-M10 | Minor | 决定 934"对话框里只有这两处"不对：粘贴来源对话框的 `textarea`（`source-add-dialog.tsx:263-271`）手写 `rounded-md border bg-transparent`，与旁边的 `Input` 底色、圆角、聚焦时的边都不同；没有 `focus-visible:outline-hidden`，强制色下聚焦时 Chromium、Firefox 都量到 `outline: none`、`box-shadow: none`，只剩光标（P1 的 S2A-M1、决定 768 的老问题）；说明是 `text-sm`，`FormField` 是 `text-compact` | 接受：粘贴来源对话框的 `textarea` 用 `Input` 抽出的共用类名（含 `focus-visible:outline-hidden`），说明 `text-compact`；934 的说法照实改；强制色下核对 |
| P5B-M11 | Minor | 附件一节上传中的 `UploadRows`（`upload-rows.tsx:54`，`divide-y rounded-md border text-sm`）照旧 6 px 圆角、默认边色的框，画在新的 14 px 卡片之上；粘贴来源对话框与编辑时的上传也用它（沿代码走，没有截图） | 接受：`UploadRows` 的 `ul` 换 `cardClass`，行的内边距同附件行 |
| P5B-M12 | Minor | 带名字的"在后台恢复"（`trash-preview.tsx:440-443`、`trash-page.tsx:201-204`）是 `whitespace-nowrap`，名字没有空格时 375 与 320 宽三种引擎 `scrollWidth` 649–650，全部探针里唯一的横向滚动；今天也这样，但 P5 认领了"回收站预览的长名"（决定 937），TB1 的新格看不到这个按钮 | 接受：带名字的"在后台恢复"按钮可折行（`h-auto min-h-7.5 py-1 whitespace-normal text-start wrap-anywhere`，或截短名字，设计者定）；TB1 那一格加一步它出现时的 `expectFits`；进 937 的认领（与 C-M8 的 24.5 一起） |
| P5B-N1 | Nit | 同 A-N2 | 接受，同 A-N2 |
| P5B-N2 | Nit | 搜索结果里页面与文件的图标换成 `text-faint`，附件的 `AssetIcon`（`attachment-row.tsx:30`）照旧 `text-muted-foreground`，同一列结果两种灰 | 接受：`AssetIcon` 与搜索结果的图标同一种灰（设计者定哪一种） |
| P5B-N3 | Nit | `entryTitleClass` 与 `statusTitleClass` 一字不差，分在两个新模块里 | 接受：`entryTitleClass` 与 `statusTitleClass` 留一个 |
| P5B-N4 | Nit | 对话框里的框是 `bg-surface-2`，暗色下描边按钮也是 `dark:bg-surface-2`，只剩约 1.07:1 的边，"Keep going"看起来像一行字 | 接受：对话框里的框只要边、不要底（或 `bg-surface`），暗色下描边按钮看得出 |
| P5B-N5 | Nit | 新的升级横幅成了一块中性的灰底，"升级中、只能读"不显眼；4.1"琥珀只标状态" | 接受：升级横幅状态那一行前加琥珀的小圆点（样稿 `.meta .dot`，不是填色） |
| P5B-N6 | Nit | 卡片与子页面行里截短的名字没有 `title`，前缀相同的两本长名字的笔记本用鼠标的人分不出（今天也这样） | 接受：截短的卡片名与子页面名加 `title`（与 P4 的做法一致的话照它） |
| P5B-N7 | Nit | 历史的"谁"下拉的宽由最长的名字定，e2e 式的名字让它宽 382 px，筛选条参差 | 接受：历史的"谁"下拉 `max-w-64` |
| P5B-N8 | Nit | 设计者的参照页与正文有好几处不一样（历史行链接多了 `text-sm`、筛选下拉是 `h-7.5`、`revealClass` 没有 `max-md:`、笔记本首页 720 宽、标题 `leading-tight`、404 的结构与 `StatusPage` 不同），第 1 节 (b) 的截图看的是参照页，I2 在参照图里因此看不出来 | 接受：第 1 节 (b) 与 7.4 的结论以正文为准，不引参照页；参照页与正文的差别照 B 的清单改正参照页或写明 |
| P5B-Q1 | Q | 虚线的"新建笔记本"自成一行，改了负责人批准的 00-M13 4.6"卡片列表最后一张虚线卡"的写法；1024 宽时看起来可以接受，与"最后一张"不同；请确认算 M 内可定的修订 | 主持者定（决定 942）：虚线的"新建笔记本"在两组之后自成一行（918），算 M 内的修订（00-M13 4.6 的"卡片列表最后一张"随 2.1 改）；918 的理由照实改（两组的语义：它不是任何一组里的笔记本；不是测试，C-M2 的 10 说那几格数的是链接） |
| P5B-Q2 | Q | 文档类页的页级操作分在两处：版本页与变更集页在头一行最右，回收站预览的"恢复"在 720 列里、标题旁，名字长时折到标题下最左（1440 宽也会）；设计写明这是焦点交接的取舍，外观能接受，请确认 | 主持者定：回收站预览的"恢复"留在标题旁（929 的焦点交接取舍），照 B-M7 的写法长名字时它不折到左下 |
| 过度设计 | — | 把 P4 页头的三串（头一行、面包屑、标题）挪进 `page-classes.ts`，要再动 P4 已核对过的 `page-layout.tsx`，生成的类不变，好处只是"同一个来源" | 接受：不搬动 P4 已核对的文件里的串；在原处导出（或 `page-classes.ts` 从原处再导出），新的三页引用它，同一来源、不改 P4 的类名；设计者定写法 |

## 审查 C（测试、e2e、变异、门禁与文档）

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5C-I1 | Important | `expectFits` 的第一句比 `clientWidth` 与常量 `phoneWidth`（375，`e2e/fixtures/narrow.ts:4,15`），窗口 320 时恒红，与页面溢不溢出无关：UI1 第 4 格第 3 步（S1 提交 2）与 hi2 的一段（S3 提交 1）照写不能为绿，S1、S3 照设计合并不了；6.1 说第 3 步"在加 `grid-cols-1` 之前也红"，红的其实是这一句；变异 5 在 375 宽就抓得到（去掉 `grid-cols-1` 时三种引擎页宽 567） | 接受：`expectFits` 加宽度参数（读 `page.viewportSize()`，夹具 `e2e/fixtures/narrow.ts`，写进第 5 节，列出今天的调用方照旧），320 宽的两段用它；6.1 的"加 `grid-cols-1` 之前也红"照实（红在哪一句）；变异 5 的抓手写 375 宽 |
| P5C-I2 | Important | 卡片带 `min-h-28`（112 px），内容约 97 px；60 个字符、没有空格的名字把 `truncate` 换成 `break-words`，375 宽折成两行，卡片仍是 112 px，第 2 步"长名字的卡片高与 Plans 的相同"照过，变异 6 活下来（三种引擎量过；320 宽时才是 112 对 124.8） | 接受：变异 6 的格改为 375 宽直接断言长名字的 `span` 一行且被截短 |
| P5C-I3 | Important | 2.1 是散文、没有引用块，设计者只对 8.2 跑过脚本：第 1 条的"五个 Phase、12 个 Step"在 00-M13 里有两处（`:344` 第 7 节开头、`:385` 引的 12.2 那一行），断言恰好一次的脚本会停；第 4、5 条的旧文字用了中文引号，文件里是 ASCII 的 `"`，照写匹配 0 次 | 接受：2.1 照 P4 写成"旧文字 / 换成"的引用块（00-M13 的"五个 Phase、12 个 Step"出现两次，`:344`、`:385`，逐处给；引号照文件里的 ASCII `"`），在 `f62d0390` 的副本上跑一遍，旧文字恰好一次、新文字之后恰好一次 |
| P5C-M1 | Minor | 同 A-I2（探针在 `loading.tsx` 加一个只在本文件用的导出，knip 报 `Unused exports (1) probeTileClass`） | 接受，同 A-I2 |
| P5C-M2 | Minor | 变异 9、10、13、17、52 写给了抓不到它的格：9 的 `n1:116` 是访客、两组都空，虚线卡本来就不画；10 的 `workspace-home.test.tsx:36-37,48,60-61` 数的是链接，`li > button` 不多一个链接（决定 918 的理由也不成立）；13 的 `page-tree-scale.test.tsx:147` 是平的树，数不画；17 的默认 `pageServer()` 有 Guide、Notes，不是空状态；52 的 `upgrade-page.test.tsx:853` 以 `closest("section")` 取最近的一节，根换成 `section` 不影响它 | 接受：变异 9、10、13、17、52 的抓手照 C 改正（写真正抓它的格，必要时加格）；918 的理由（见 942） |
| P5C-M3 | Minor | 同 A-N1（`probe/mobx-computed.mjs`：没人观察时读一千次建 1,000 次索引，被一个 reaction 观察时在它之外读一千次只建 1 次） | 接受，同 A-N1 |
| P5C-M4 | Minor | 显出的规则缺变异：`revealClass` 的 `max-md:opacity-100`、`[@media(hover:none)]:opacity-100` 去掉什么都不红（窄屏上"只看此人"静止时透明）；出错页"回到首页"的 `arrived` 没有变异行；第 46、56、60 条都有便宜的格（`SettingsCard` 的 `ul` 的 `className === cardClass`、as1 打开菜单之后移开指针量触发按钮的 `opacity`、标签的父元素带 `space-y-1.5`） | 接受：`revealClass` 的 `max-md:opacity-100` 加变异与格（mc3 的 `@narrow` 在 375 宽断言"只看此人"的 `opacity` 是 1）；出错页"回到首页"的 `arrived` 加变异行；46、56、60 照 C 的便宜写法各加格 |
| P5C-M5 | Minor | 横幅进列的格：S3 提交 2 的"遍历 12 个路由"在改代码之前就绿（每个路由都已带 `handle`，回退走不到），6.3 的"看到红"不成立，只有变异 57、58 证；S1、S2 每页一格只看读到之后，读取中、失败、找不到的一支漏画 `<ColumnTop />` 要到 S3 才抓；"同 `:141-148` 的写法"比 `banner.parentElement === heading.parentElement`，笔记本首页、健康页与文档类三页的 `h1` 在下一层的 `div` 里，对的实现上就红 | 接受：与 A-I1 合并：从 S1 起只写一个遍历的格，每个 Step 加自己的路由与各支，断言"横幅的父元素是那一页的列的根"（不比 `h1` 的父元素）；S3 的那一格"看到红"照实（由变异 57、58 证） |
| P5C-M6 | Minor | 2.1 漏了：00-M13 第 13 节的一行；第 12 节 P5 一行的"Phase 文档 / 状态 / 审查"三格（P4 的设计提交 `92cbffd9` 都改了）；`v0.1-design.md` 第 15 节改 12.2 的记录；00-M13 4.6 第三条把版本、导入导出列进"页面标题 + 筛选条 + 卡片里的行"，与决定 929、939 不一致；第 4 条换完之后连着两个括号，也没写"两组都空时不画" | 接受：2.1 补 00-M13 第 13 节的一行、第 12 节 P5 一行的文档链接与状态与审查三格、`v0.1-design.md` 第 15 节改 12.2 的记录；00-M13 4.6 的列表页一条照决定 929、939 与 C-Q2 改（版本页的版式） |
| P5C-M7 | Minor | 同 A-N5，另：UI1 第 4 格 `REQUIRED` 两行的理由是 Firefox 不在连字符与数字之间断（P4S1B-F1），格里却没有一个会折的长名字；TB1 说"WebKit 由 UI1 第 4 格的长名字覆盖（同一个 `wrap-anywhere`）"不成立，UI1 第 4 格里卡片与行都是 `truncate` | 接受，同 A-N5 |
| P5C-M8 | Minor | 决定 937 漏了 M12-polish 第 1 项里落在 P5 页上的 1、9.4、24.4、24.5、27.1、31.2、34，与第 5、11 项"同类约二十处 `bdi`"里落在 P5 页上的（P4 第 9 节把"P4 的页之外的"交了出来），既没认领也没写不认领；24.5 是 S2 重排的页上的布局问题 | 接受：937 逐项写 M12-polish 第 1 项里落在 P5 页上的 1、9.4、24.4、24.5、27.1、31.2、34 与第 5、11 项里 P5 页上的 `bdi`：认领的写明 Step，不认领的写明不认领 |
| P5C-M9 | Minor | 8.2 的几条新文字与设计不符：第 1 条"顶层页面的列表（各带子页面数）"（只有有子页面的行带数）、"没有东西的列表是一个图标、一句说明与主操作"（搜索没有命中照旧只有状态行，右栏照旧一行灰字）；第 4 条"在列表里折行"；第 11 条"升级的页……各节是卡片"（只有两个列表进卡片） | 接受：8.2 第 1、4、11 条照实改 |
| P5C-N1 | Nit | 零碎十一项：`:648` 的"变异 3"应是 5；7.1 的两个测试文件路径（`import-dialog.test.tsx` 不存在，在 `transfer-import.test.tsx` 里测；`source-uploads.test.tsx` 在 `W/pages/notebook/`）；决定 933 的"三十多处"实数 27；`page-tree.ts:240-243` 应是 `page-tree.store.ts`；`hi2:132` 是"没有改动"之后，不是被拒；TB1 取 id 不必 `listTrash`；`ui2:143-169`、`ui1:40-54` 改它们页上东西的是 S1；`page-tree-scale.test.tsx:243` 在旧树上就绿，不该列在"看到红"；`column-top.tsx:5` 的注释（同 A-N8）；8.2 第 7、8 条的新文字以空格开头；`trash-preview.test.tsx:587` 是偶发格，S2 往这个文件加格会挪它的行号 | 接受，逐条照 C 改（变异编号、两个测试文件的路径、27 处、`hi2:132` 的说法、去掉 TB1 的 `listTrash`、`ui2:143-169` 与 `ui1:40-54` 归 S1、`page-tree-scale.test.tsx:243` 不列在"看到红"、`column-top.tsx:5` 的注释、8.2 第 7、8 条开头的空格、`trash-preview.test.tsx:587` 偶发格的行号与加格的位置） |
| P5C-Q1 | Q | M13 还没在 CI 上跑过：远端没有 `m13-ui-refresh`，也没有任何 `m13-*` 分支（决定 746）；7.3 说 hi3 的 `emulateMedia({ forcedColors })`"L4 已在 CI 上用过，P4/S2"不成立；7.5 的"CI 的 E2E 一步约 6 分 21 秒"是 M12 收尾的数。是否在 7.3、7.5 写明这些都等负责人推送之后的 CI | 主持者定（决定 944）：7.3、7.5 写明 M13 还没在 CI 上跑过（远端没有 `m13-*` 分支，决定 746）；"L4 已在 CI 上用过"改掉；6 分 21 秒写明是 M12 的数；只靠 CI 的各步都等负责人推送之后的 CI |
| P5C-Q2 | Q | 00-M13 4.6 把"版本"列进"页面标题 + 筛选条 + 卡片里的行"，决定 929 让它照页的头一行排；这算对 00-M13 的修订（进 2.1），还是 4.6 只是举例 | 主持者定（决定 945）：00-M13 4.6 随 2.1 修订：版本页、变更集页、回收站的预览照页的头一行排（929），不算"列表类"；导入导出页已由 P3 换过（939） |
| 顺手看到的三件 | — | （B 的领域）页的阅读视图那一块的拖放区（`attachment-drop.tsx:51`）照旧 `ring-primary`，附件一节换成 `ring-accent`，同一页两种颜色；历史筛选的小块 `rounded-full`，`mc3:213` 的 120 个字符的页名在 375 宽折成多行，字贴着弧形的边；"子页面"一节的 `h2` 照旧 `text-sm font-medium`（`page-layout.tsx:507`），附件一节改成 `text-compact font-semibold` | 设计者看：页的阅读视图的拖放区与附件一节的环同色（取琥珀则一并改，`attachment-drop.tsx:51`）；历史页筛选小块长名字折行贴边（`rounded-full` 换成随行高的圆角或截短）；"子页面""附件"两节的小标题同一种样子 |

## 未核实的怀疑（审查者列出，去向）

处置没有单列这些；去向是修订稿里落到的地方，设计里没有写的照实记下。

| 来自 | 怀疑 | 去向 |
|---|---|---|
| A | 横幅随页重挂：P5 之后每次导航都新插一个带文字的 `role=status`，有的读屏（例如 Safari 加 VoiceOver）可能在插入时读出，与 `upgrade-banner.tsx:24-26` 的"每页不再读"相背；A 建议进收尾的读屏清单 | 设计里没有写（第 9 节的读屏清单只记横幅在每一支的列里、小圆点不读） |
| A | 主要输入能悬停的触屏笔电报 `hover: hover`，手指点到透明的"只看此人"或"操作"会直接触发；iPadOS 接触控板时报哪一种 | 设计里没有写 |
| A | NVDA 浏览模式的光标停在透明按钮上时，视觉的高亮框在一块空白上 | 第 9 节的读屏清单记了"透明时照样可聚焦，读屏照样读到"，高亮框没有写 |
| A | M4 的位移（约 36 px）只是按类名推算的 | A-M4 的改法（定宽的一格）之后不再位移，第 1 节 (h) 在 320–1024 宽量过 `h1` 的上边 |
| B | 探针是带设计类名的静态页，不是应用；健康的规则卡片与升级页的各节只照设计的文字排了几处 | 各 Step 的审查 B 照 7.4 以应用的截图看 |
| B | Linux 的字体下卡片的截短、版本页 320 宽的一行与"-"的次数 | 7.3 的"CI"一条，等负责人推送之后的 CI（决定 944） |
| B | Windows 高对比的暗色方案；真 Safari 的 `:focus-visible`；Playwright 的 WebKit 在 macOS 上 Tab 不走到透明的按钮与行链接 | 设计里没有写 |
| B | 暗色下命中的高亮字在卡片上 B 量到 7.418，设计推算 7.28 | 决定 936：S2 在应用里量，下限照实量改 |
| B | 回收站页上方的"在后台恢复"（`offered`）与预览里的是同一个按钮，没有单独量 | B-M12 的改法两处一起改（决定 937），TB1 那一格核对 |
| B | M5 的改法（小块内联在"· Edit"之后）没有验证 | 决定 943 的探针 (g) |
| C | Linux 字体下 UI1 第 4 格的卡片高可能差一档 | C-I2 的改法不再比卡片的高 |
| C | `SubpageRow` 多收一个 `t`，`useT` 每次给的函数不是同一个时 memo 失效，一千行随首页每次渲染重画 | 4.1.4：`SubpageList` 在一次渲染里取一次 `t` 与语言区、给每行算好两串，行只收字符串，memo 只比字符串 |
| C | 遍历那一格要的读取中、失败各支，`lint-server`、`health-server`、`jobs-server` 能不能都扣住或答错 | 4.3.6 的表逐行写了让哪个读失败；A 在修订核对第 1 轮逐个对过假服务端，C 的探针把 36 行都造了出来 |
| C | 入口卡片内联的 `svg`（`aria-hidden`）会不会碰上 oxlint 的 jsx-a11y 规则 | 设计里没有写（每个提交的门禁照旧跑 `make lint-web`） |

## 主持者的决定

（主持者的决定，负责人可以改判；编号接本文 2.3 的 912–939。）

- **940** P5 改写的行与头里通往别处的内容链接（历史行、回收站行、版本页的作者、变更集的"撤销了/被撤销"、升级页通往页面的链接）照 13.2 第 17 条补上到达的状态（`arrived` 或该去处需要的），随改写那一页的 Step；配格与变异，进 8.2（A-Q1）。
- **941** 已登录的邀请页右上角照决定 803 加"回到工作区"：`Corner` 的那一个，去最近的工作区，带 `arrived`；921 的理由随之改，配格（A-Q2）。
- **942** 虚线的"新建笔记本"在两组之后自成一行（918），算 M 内的修订，00-M13 4.6 的"卡片列表最后一张"随 2.1 改；918 的理由照实写成语义：它不是任何一组里的笔记本（B-Q1、C-M2）。
- **943** 右栏版本行单独一行的"-"：设计者试 B 的改法（小块内联在"· Edit"之后），三种引擎 200–240 px 逐 1 px 扫，不再出现就进 S3（右栏的版本行是 P4 的文件，S3 改），否则照 B 的数据把交 v0.2 的文字改成"可复现"（B-M5）。
- **944** 7.3、7.5 写明 M13 还没在 CI 上跑过（远端没有 `m13-*` 分支，决定 746），"L4 已在 CI 上用过"改掉，6 分 21 秒写明是 M12 的数；只靠 CI 的各步等负责人推送之后的 CI（C-Q1）。修订核对第 3 轮之后照实改成"只跑过一次"（C-R3-M2）。
- **945** 00-M13 4.6 随 2.1 修订：版本页、变更集页、回收站的预览照页的头一行排（929），不算"列表类"；导入导出页已由 P3 换过（939）（C-Q2）。

## 修订时的复核

（据设计者交回的要点，主持者存在会话的临时目录 `p5-design/report-r1.md`，2026-11-25；各条的落处另照三位审查者第 1 轮核对的表补了变异的编号。修订稿 `05-P5-pages.md` 1,370 行；探针在 `p5-design/`，设计第 1 节 (g)–(k)；`f62d0390` 的分离工作树 `nw-p5-rev` 用完已删。"照处置"即照上表的处置做了。）

**修订的记录**：头的日期一行"同日照设计审查修订（……三份审查全部接受，主持者的决定 940–945）"，状态"修订稿（照设计审查），待修订核对"；改了工作笔记、第 1 节 (g)–(k)、2.3 的头一段（913、915–925、927–935、937 的文字照审查改了）、7.2 的头一段（1–60 与 43b 改了哪些抓手，61–91 是修订加的）与附录"修订时另读"。

**审查 A**

- **A-I1**（与 C-M5 合并）：4.3.6 的"路由 × 支"表 36 行（S1 8、S2 提交 1 7、提交 2 13、S3 8，12 个路由），每处早退写行号与让哪个读失败；断言三句：横幅恰好一个；父元素是列的根（`firstElementChild === banner`、带 `mx-auto`、包含先等到的元素）；在先等到的元素与每个 `h1` 之前。表取代 `upgrade-banner.test.tsx:141-177`、`:209-220`，保留 `:179-207`；P4 的页的两支早退（`page-layout.tsx:77-84`、`:86-93`）在 S1 包进 `PageColumn`；变更集页、预览、版本页的早退照处置逐行列在 4.2.6、4.2.8、4.3.1，版本页的两支画在 `VersionPage` 里；变异 57、58、61、67、81、82。
- **A-I2**（同 C-M1）：`iconTileClass` 在 S1 提交 2 不导出，提交 3 才导出；第 0 节与 4.5 的 knip 清单同改。
- **A-M1**：`sectionAt` 去掉末尾斜杠并转小写（S1 提交 1，4.1.7）；不取 `caseSensitive: true`，理由写在 923；第 1 节 (e) 照实写今天整页报错；7.1 加大写的地址与令牌页的格；变异 64。
- **A-M2**：三处"See the jobs"都带 `arrived`（924、4.2.6、4.2.7）；格是 `revert-background.test.tsx:432-435`、`trash-background.test.tsx:388-391` 加的一句；变异 74、75。
- **A-M3**：4.2.8：`NotInTrash`、`Gone` 留在 `shell` 的列里，只换 `h1` 的类名，自己不画 `ColumnTop`；变异 81。
- **A-M4**：4.3.1：能编辑的人那一格总在、定宽（`cn(headActionsClass, "min-h-7.5 flex-[0_1_10rem]")`），有没有按钮 `h1` 的上边一样（第 1 节 (h)）；7.1 扣住版本列表的一格；7.4 加截图；变异 83。处置给的两种由设计者定，取的是给那一格定宽。
- **A-N1**（同 C-M3）：变异 15 判等价、不跑，依据写全；"量时间"删了，路径改成 `page-tree.store.ts`，风险 4 随之改。
- **A-N2**（同 B-N1）：`revealClass` 写 `transition-[opacity,color,background-color]`。
- **A-N3**：4.2.11 改那一句（从 `h1` 起 Tab 跳过"撤销这次改动"）；进第 9 节的读屏清单。
- **A-N4**：成员数与子页面数都 `toLocaleString`；7.1 加"1,200 members"。
- **A-N5**（同 B-M4、C-M7）：卡片与子页面列表截短、标题折行（931）；UI1 第 4 格的标题、8.2 第 4 条同改。
- **A-N6**：设计者定 `aria-describedby` 指向这一行里一个 `hidden` 的句子（`useId`），看得见的数 `aria-hidden`；UI1 第 5 步、变异 14。
- **A-N7、A-N8**：照实写进 918；变异 56 改由 as1 的一句抓，`column-top.tsx` 的注释在 S3 改（4.3.6）。
- **A-Q1**：决定 940 列出 S2、S3 要带 `arrived` 的链接（4.2.5、4.2.6、4.2.7、4.3.1、4.3.4），格与变异 70–73、87、88，进 8.2 第 8、13 条。
- **A-Q2**：决定 941：`Corner` 加 `inviting`（`useMatch("/invitations/:id")`），`back` 与读列表的 `useSWR` 都看它；`invitation.test.tsx` 的格，变异 68、69。账户没有工作区时不放：`/` 会落到新建工作区的页，"回到工作区"说的不对（同 803），处置的"没有时去首页"在这里就是不放。

**审查 B**

- **B-I1**：`button.tsx` 导出合并过的 `buttonClass`，不导出 `buttonVariants`；`button.test.tsx` 新格、变异 66。
- **B-I2**：各列表的行一律 `lineTitleClass`、`lineLinkClass`；新文件 `list-rows.test.tsx` 横跨各页读类名；变异 76–78。
- **B-M1、M2**：笔记本首页 720（916 改）；卡片与虚线卡 `sm:min-h-28`。
- **B-M3**：子页面行的链接 `rounded-[inherit] focus-visible:-outline-offset-2`，写进 8.2 第 3 条（卡片里整行链接的焦点外框成了长期约定的一句）。
- **B-M4**：同 A-N5。
- **B-M5**：决定 943 的探针 (g)：960 行 × 200–240 px 每 1 px，39,360 对；今天的写法三次是 Chromium 716、522、585 对，Firefox 679、391、537，WebKit 1,632、1,605、1,444；内联之后三次都是 0，没有横向溢出。进 S3 提交 1（4.3.2），交 v0.2 的一条删了；要跟着改的是 `version-restore.test.tsx:157`、hi1:259、hi2:114（"· EditCurrent"连写）与 `page-versions.test.tsx:111-141`。
- **B-M6–M11**：回收站行的文字一块 `listTextClass`；标题行 `h1` 加 `min-w-0 flex-1 basis-56`、按钮 `ms-auto`；"Reverted"用 `markClass`；三页的事实一种 `docFactsClass`；文本框用 `Input` 抽出的 `fieldClass`、说明 `text-compact`，934 照实改；`UploadRows` 用 `cardClass`。
- **B-M12**：带名字的"在后台恢复"可折行，另加 `shrink`（只照 B 的写法时，`Button` 的 `shrink-0` 让它在 `justify-end` 的一行里向左伸出屏幕，第 1 节 (h)）；TB1 核对；变异 79、80；进 937 的认领。
- **B-N1–N7**：同 A-N2；`AssetIcon` 与搜索的图标同为 `text-faint`；只留 `entryTitleClass`；对话框里的框只要边；横幅小圆点 `border-3` 画的圈、不填色（变异 89）；截短的名字加 `title`；"谁" `max-w-64`。
- **B-N8**：结论以正文的类名为准，不再引参照页；`rev.mjs` 的探针页取正文的类名，39 串与正文逐字核对过（第 1 节 (h)）。
- **B-Q1、Q2**：照决定 942 与处置。
- **过度设计**：P4 头一行的串在 `page-layout.tsx` 原处导出，变更集页与预览从那里导入；探针 (i)：变更集页的闭包从 44 块 976,309 字节变成 47 块 1,047,564 字节，写进 925 与 4.5。

**审查 C**

- **C-I1**：`expectFits` 取 `page.viewportSize()?.width ?? phoneWidth`（5.1）；今天 17 处调用都在 375 宽、照旧比 375（pg2:184,211、pg17:288,417,654、ui2:169、ui1:54,67、lw3:346,357,364、mc3:202,214,220,226,233、se1:420）；变异 5 的抓手写成 375 宽那一步。
- **C-I2**：UI1 第 4 格第 2 步在 375 宽断言长名字的 `span` 一行（高除以 `line-height` 小于 1.5）且截短（`scrollWidth > clientWidth`）；变异 6 同改。
- **C-I3**：2.1 改写成 14 条引用块（"旧文字 / 换成 / 之后加 / 之后另起一行加"），回放见下。
- **C-M2、M4**：变异 9、10、13、17、52 的抓手照 C 改；变异 63 由 mc3 的一句抓，出错页"回到首页"是变异 65，46、56、60 都加了格。
- **C-M6**：2.1 补第 11 条（第 12 节 P5 一行的四格）、第 12 条（第 13 节一行）、第 14 条（`v0.1-design.md` 第 15 节）、第 8 条（4.6 的文档类三页与导入导出）；第 7 条的两个括号合成一个，写上"两组都空时不画"。
- **C-M7**：同 A-N5，另：UI1 第 4 格加长名字的笔记本首页一步（`h1` 折行），三种引擎才有可比的；TB1 的"WebKit 由 UI1 覆盖"改成照实（只在 Chromium）。
- **C-M8**：937 逐项归到认领或不认领；24.5 不认领（放进那一行要改拒绝的提示与焦点，算行为），S2 只改按钮的折行（B-M12）。
- **C-M9、C-N1**：8.2 第 1、4、11 条照实改；零碎各项照改，`trash-preview.test.tsx` 的新格加在文件末尾。
- **C-Q1、Q2**：照决定 944、945。
- **顺手看到的三件**：拖放区两处都 `ring-accent`（933）；筛选小块 `rounded-[18px]`（926）；附件的小标题同"子页面"（933）。
- C-M1、M3、M5 见 A-I2、A-N1、A-I1。

**回放**：2.1 在 `f62d0390` 的 00 号文档与总体设计的副本上做 14 条、15 处，每段旧文字做之前恰好一次、新文字做之后恰好一次；8.2 在 2.1 做完的副本上做 16 条、17 处（第 3 条两处），同样。脚本 `p5-design/replay.py`。

**设计者另记的**：修订会动 P4 的 `page-layout.tsx:76-93`、`:388`、`:430`；8.2 第 3 条多了一句（卡片里整行链接的焦点外框）；审查记录的链接还写在反引号里，等本文进仓库。`check_links.py` 没有新坏的链接。

## 修订核对第 1 轮

三位审查者核对修订后的 05-P5（1,370 行）与上一节的复核。原来的发现：A 16 条都已落实；B 两条 Important 都解决（三种引擎实量），12 条 Minor 里 10 条解决：M5 的"-"不再单独一行，但内联之后中文的小块会断在两行（B-R1-M2），M9 只解决了一半（B-R1-M3、M4），M7 的改法在手机宽度上带来新的问题（B-R1-M1），Nit 全部照处置落了（其中 N2、N5、N8 各带出一个小尾巴）；C 的各条都已处理，只是 M4 照处置加的两处写错（C-R1-I2、C-R1-M2），M5 的表本身两处写错（C-R1-I1、C-R1-M1）。C 以自己的脚本独立重放 2.1 与 8.2 的 32 处，与设计者的逐字节相同；B 照修订的类名排了 19 种页 × 四种宽 × 亮暗 × 中英 × 三种引擎，都不横向溢出（M12 那一处也没有了）。新发现：A 2 Minor、2 Nit；B 4 Minor、7 Nit；C 2 Important、4 Minor、1 Nit（一组零碎）。两对同一个问题：页与笔记本设置"读到"两行在旧树上就绿（A-R1-N2、C-R1-M1，按 Minor 处理），`list-rows.test.tsx` 的 `rounded-card` 断言对健康一行不成立（B-R1-N6、C-R1-M4，按 Minor 处理）。主持者照下表定处置，全部接受，没有新的决定。

### 审查 A

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5A-R1-M1 | Minor | 941 让邀请页读工作区的列表，带进一个竞态：`WorkspaceStore.load` 只在"读的过程中有改动被答复、而且已有列表"时留着已有的列表；从邮件打开邀请链接时列表还没读过，第一次读还没回来就按"接受邀请"，那次读答的是不含刚加入的工作区的旧列表，`WorkspaceLayout` 与它同一个 SWR 键，`bySlug` 找不到，画出"Page not found"（`p5reva-invite-race.test.tsx` 复现；今天邀请页不读这个列表，不会这样） | 接受：`WorkspaceStore.load` 加 A 的那一句（接受邀请之后、第一次读之前发出的读不盖掉刚加入的工作区），配一格与一条变异；写进 941 与 4.1 |
| P5A-R1-M2 | Minor | 943 漏了 `page-versions.test.tsx:104`（中文的"5分钟前Ada · 撤销当前编辑中"），照 4.3.2 在小块前各加一个空格之后这一格红；4.3.2、943、(f)、7.1 的清单都没列 | 接受：`page-versions.test.tsx:104` 加进 4.3.2、943、(f) 与 7.1 的清单 |
| P5A-R1-N1 | Nit | 941 没看引导：邀请页在 `Onboarded` 之外，账户有工作区、引导还有步骤时也放"回到工作区"，按下到 `/onboarding?next=%2F`；7.1 与变异 69 写的去处 `/lab` 不对，`invitationServer` 的账户只有 Acme，`/` 落到 `/acme`；"没有工作区"那一格要 `invitationServer` 加一个 `joined` 选项；`layout.test.tsx:113` 的注释随 941 不再对 | 接受：941 另要 `pendingSteps(me).length === 0`；`invitationServer` 加 `joined` 选项，7.1 与变异 69 的去处照实（`/acme`）；`layout.test.tsx:113` 的注释 |
| P5A-R1-N2 | Nit | 4.3.6 说"每一行在加它的那个提交里先红"，页与笔记本设置的"读到"两行在 `f62d0390` 上已经绿（P4 已把横幅放进这两处的列） | 接受，同 C-R1-M1 |

### 审查 B

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5B-R1-M1 | Minor | M7 的改法带来的：`basis-56` 让 `h1` 至少 224 px，加间距与"新建页面"（英文 112、中文 102 px）宽过 375 宽时的列（327 px），名字只有"Plans""计划"的笔记本首页在 320、375、390 宽"新建页面"也独自在下一行（今天同一行）；健康的"重新检查"、预览的"恢复"在 320 宽掉下去 | 接受：标题 `min-w-0 flex-1 basis-56 max-sm:basis-auto`（B 三种引擎实量过） |
| P5B-R1-M2 | Minor | 943 内联之后，`Mark` 是普通的行内 `span`，中文的"当前""编辑中"在两字之间断行，一个圆角小块劈成两半：照 4.3.2 重扫 1,280 行 × 200–240 px、52,480 对，Chromium 1,920、Firefox 2,756（217 px 实宽上 8 行）、WebKit 1,224 对，英文 0；单独一行的"-"与横向溢出都是 0；两串加 `whitespace-nowrap` 之后三种引擎都是 0 | 接受：`markClass` 与 `current` 两串加 `whitespace-nowrap`，加一条变异；943 的扫描写明也数了小块断行 |
| P5B-R1-M3 | Minor | 变更集页的事实只换了根的类名（`text-faint`），各行自带的 `text-muted-foreground`（`changeset-page.tsx:365,368,405`）还在，同一块两种灰交错；用户写的消息（`:378`）随根变成最淡的一行，亮色约 3.1:1；三页仍不一致（M9 的另一半） | 接受：变更集页的事实各行去掉自带的 `text-muted-foreground`，用户写的消息给 `text-sm text-foreground`（内容，同搜索的摘录），设计者定 |
| P5B-R1-M4 | Minor | `IngestSources`（`history-ingest.tsx:37-38,49`）没有动：历史行里事实 13 px，其下摄取一块 14 px，来源是正文色、带下划线的链接；变更集页里它在 12.5 px 的事实里仍是 14 px；LLM Wiki 的每一次摄取都有这一块 | 接受：`IngestSources` 收 `className` 或去掉自己的 `text-sm`，来源链接用 `lineLinkClass`；`list-rows` 的历史一行用一条摄取的变更造数 |
| P5B-R1-N1 | Nit | 横幅的状态一行折行时（375 宽的中文），`items-center` 让 6 px 的小圆点落在两行中间 | 接受：横幅小圆点 `items-start` 加 `mt-[calc(0.5lh-3px)]`，或行内 `inline-block`，设计者定 |
| P5B-R1-N2 | Nit | N2 的改法在回收站带出两处不一致：行里页面与无类型文件的图标照旧 muted、附件的是 faint；预览给 `ItemIcon` 的 `className` 到不了只收 `mime` 的 `AssetIcon`，附件预览的图标 16 px、贴顶，页面的 24 px、下移 6 px | 接受：回收站行与预览的图标一律 faint；`AssetIcon` 收 `className` 经 `cn` 合并 |
| P5B-R1-N3 | Nit | "Reverted"悬停时的下划线压在 19 px 高的小块的下边框上、伸进圆角里 | 接受："Reverted"悬停改成 `hover:border-border-strong hover:text-foreground`（不用下划线） |
| P5B-R1-N4 | Nit | 变更集改动行第二行的"Content"仍是常驻下划线（`changeset-change.tsx:148-154`），第一行的次要链接已是悬停才有 | 接受：变更集改动行的"内容"链接悬停才有下划线 |
| P5B-R1-N5 | Nit | 透明的"只看此人"（`size="sm"`，30 px 高）撑高历史行的第一行，标题到第二行比搜索的行多约 5 px（1440 宽实量 28 对 23 px），看不出原因 | 接受："只看此人"加 `-my-1`（或 `h-6`），历史行第一行同搜索的行高 |
| P5B-R1-N6 | Nit | `list-rows.test.tsx` 的"行在类名有 `rounded-card` 的列表里"对健康一行不成立：它的 `ul` 没有圆角，卡片是外面的 `section` | 接受，同 C-R1-M4 |
| P5B-R1-N7 | Nit | 设计说回收站行的"恢复""总在右上"，375、320 宽时它在文字之下、靠左（同 P3 的行），同一个 Step 里标题行的按钮折下去时靠右 | 接受：回收站行的"恢复"加 `ms-auto`（同标题行），说法照实 |

### 审查 C

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5C-R1-I1 | Important | 4.3.6 的表里 `first` 是"加载中"的五行（页、笔记本首页、版本页的树读取中，变更集页、预览的读取中）：照"`main` 里文字是'Loading…'的 `output`"去 `findByText`，等到的是外框在笔记本列表读到之前画的那一个（`notebook-layout.tsx:75-77` 的 `NotLoaded`），它随即卸下，第 2、3 句在对的实现上也红（C 在粗做的对的实现上跑出 `containsFirst=false`、`connected=false`）；S1 提交 2 照写不能为绿 | 接受：4.3.6 表里"加载中"五行的 `first` 改成"先等横幅，再 `getByText`"（`:756` 的定义），照 C 的 `hack.py` 验过的写法 |
| P5C-R1-I2 | Important | as1 加的那一句（C 第一次审查 P5C-M4 给的写法）找不到"操作"：Radix 的 `DropdownMenu` 默认是模态的，开着时 `hideOthers` 给菜单之外加 `aria-hidden`，附件的 `ul` 也是，`getByRole` 不带 `includeHidden` 数到 0，`toHaveCSS` 等到超时；S3 提交 2 照写不绿，变异 56 的抓手随之失效 | 接受：as1 那一句加 `includeHidden: true`（Radix 模态菜单给菜单之外加 `aria-hidden`） |
| P5C-R1-M1 | Minor | 同 A-R1-N2（C 的探针在 `f62d0390` 上逐行跑：那两行三句全过，其余 34 行红在第 2 句的 `mx-auto`） | 接受：表里页与笔记本设置"读到"两行写明是搬进来的已有行为（在 `f62d0390` 上就绿），改 `:797`、`:937` |
| P5C-R1-M2 | Minor | 变异 63 的理由写反：`webkit-narrow` 是 `Desktop Safari`，三个项目在 375 宽都报 `hover: hover`，去掉 `max-md:opacity-100` 之后 mc3 两次都红；连带 `[@media(hover:none)]:opacity-100` 在任何项目里都不生效，去掉它什么都不红 | 接受：变异 63 的理由改正（`webkit-narrow` 是 `Desktop Safari`，`hover: hover`）；`[@media(hover:none)]` 加守卫（`history-page.test.tsx` 的类名断言）与变异 92，7.2、7.5 的计数随之改 |
| P5C-R1-M3 | Minor | 变异 62 只在 Firefox 里红：Chromium、WebKit 在连字符之后断，`h1` 不带 `min-w-0` 也缩得下；设计者探针 (h) 的长名字是连字符接字母的 `n000-n001-…`，Firefox 在那里也断，拿它量不出这条变异 | 接受：变异 62 只在 Firefox 红，`:1114` 照实改；探针 (h) 的名字换成连字符接数字 |
| P5C-R1-M4 | Minor | 同 B-R1-N6 | 接受：`list-rows.test.tsx` 改成 `closest(".rounded-card")` |
| P5C-R1-N1 | Nit | 零碎：4.1.4 的 `countOf` 照写不过 `tsc`（`pages.tree` 可能是 `undefined`，`childrenOf` 只收 `TreeIndex`）；决定 940 与 7.1 用的英文是代码里的叫法，界面的是"Undone""Undid “…”""See the undo""Undone by:""History of this node"，测试照设计里的英文找不到；`:753` 回退画的横幅的父元素就是 `main` 本身，三项里只有 `mx-auto` 分得出回退与列；8.2 第 3 条的新句读起来像 375 及更窄的检查都在 Chromium 与 WebKit、都带 `@narrow`；hi1:258、hi2:113 与 `page-versions.test.tsx` 的注释要随测试改 | 接受：`countOf` 的 `tree` 可能 `undefined`；940 与 7.1 用界面上的英文（Undone、Undid “…”、See the undo、Undone by:、History of this node）；`:753` 回退画在 `main` 本身、只有 `mx-auto` 分得出；8.2 第 3 条的措辞；hi1、hi2、`page-versions.test.tsx` 的注释随测试文字改 |

### 未核实的怀疑（第 1 轮）

| 来自 | 怀疑 | 去向 |
|---|---|---|
| A | 直接打开的 `/create-workspace` 今天大概就有 R1-M1 的竞态（`create` 也只经 `changed`），没有跑探针 | R1-M1 改在 `WorkspaceStore.load` 里，不看是哪一页发出的读 |
| A | 升级页自己若另有一个 `status`、文字也含"being upgraded"，表的第 1 句会数到两个 | C 的表探针在 `f62d0390` 上逐行造数，第 1 句 36 行都是 1，升级页也是 |
| B | 探针是静态页；`IngestSources`、`ChangesetFacts` 的结构照代码抄，假定 S2 只换根的类名 | 各 Step 的审查 B 照 7.4 以应用的截图看 |
| B | Linux 的字体下 R1-M2 的断行次数与 R1-M1 的阈值会变 | `whitespace-nowrap` 与字体无关（B）；标题行的阈值由 C 在修订核对第 3 轮以五种字体量过（变异 96） |
| B | 真 Safari 的 `:focus-visible` 与 Windows 高对比的暗色方案 | 设计里没有写 |

## 再修订时的复核（第 1 轮）

（据设计者交回的要点，2026-11-25。再修订稿 `05-P5-pages.md` 1,398 行；探针 (l) 在 `p5-design/rev2.mjs`，设计第 1 节；`f62d0390` 的分离工作树 `nw-p5-rev2` 用完已删。"照处置"即照上表的处置做了。）

**修订的记录**：头的日期一行加"又照修订核对第 1 轮再修订（三份报告全部接受，没有新的决定）"，状态"再修订稿（照修订核对第 1 轮），待修订核对第 2 轮"；2.3 的头一段记下照它改了的决定 919、925、930、932、935、940、941、943；7.2 的头一段记下加了变异 92–95，改了 62、63 抓手的理由与 69 的去处；工作笔记加探针 (l)，附录加"修订核对第 1 轮之后另读"。

**审查 A 第 1 轮**

- **A-R1-M1**：4.1.6 加 `W/stores/workspace.store.ts` 一条：读的过程中有改动被答复、又没有可留的列表时 `return this.list ?? this.load()`；第 0 节、6.1 提交 3（说明与"看到红"）与 941 写进去；`workspace.store.test.ts` 一格与 `invitation.test.tsx` 的竞态一格；变异 93。处置要一格，设计者另在店这一层加了一格。
- **A-R1-M2**：`page-versions.test.tsx:104` 进 4.3.2、943、(f) 与 7.1，期望改成 `"5分钟前Ada · 撤销 当前 编辑中"`。
- **A-R1-N1**：`Corner` 在邀请页上另要 `account?.me !== undefined && pendingSteps(account.me).length === 0`；`invitationServer` 加 `joined` 选项；去处改成 `/acme`（7.1 与变异 69）；`layout.test.tsx:113` 的注释随改；另加一格与变异 95（处置只要一格）。
- **A-R1-N2**：同 C-R1-M1。

**审查 B 第 1 轮**

- **B-R1-M1**：`page-classes.ts` 在 S1 加 `titleTextClass = "min-w-0 flex-1 basis-56 max-sm:basis-auto"`，笔记本首页、健康页、预览的 `h1` 都用它；919、932、4.5 的 knip 清单、变异 62 随之改。探针 (l)：短名字在 320–1024 宽都与按钮同行。
- **B-R1-M2**：`markClass` 与 `current` 都加 `whitespace-nowrap`；7.1 加类名一格，变异 94；943 的扫描第四次（照 B 的 `dash7.mjs` 加"只有编辑中"一种，1,280 行 × 41 个宽度 = 52,480 对，另数劈开的小块）：今天的写法单独一行的"-" Chromium 1,075 对（217 px 时 62 行）、Firefox 647（18）、WebKit 3,406（120），小块不劈；内联、不带 `nowrap` 时"-"都是 0，劈开的小块 Chromium 1,740（217 px 时 0）、Firefox 2,752（28）、WebKit 1,092（0），全是中文；加 `whitespace-nowrap` 之后"-"、劈开的小块与横向溢出都是 0。
- **B-R1-M3**（设计者定）：变更集页事实里自带 `text-muted-foreground` 的三行（`:365,368,405`）去掉它，随根变成一种灰；消息一行（`:378`）给 `text-sm text-foreground`：它是用户写的内容，不该比元数据淡。
- **B-R1-M4**：`IngestSources` 收 `className`，根去掉 `text-sm`，来源链接用 `lineLinkClass`；历史行传 `factsClass`，变更集页随根；`list-rows` 的历史一行改用摄取的变更造数并加断言。
- **B-R1-N1**（设计者定）：横幅状态行 `items-start`，小圆点 `mt-[calc(0.5lh-3px)]`；(l) 量到折成 2–4 行时点的中心都离行顶 10 px，正是第一行的中线。
- **B-R1-N2**：`AssetIcon` 收 `className`、改成 `text-faint`，从 S3 挪到 S2 提交 1（搜索在 S2 就用它，不挪的话搜索的图标要到 S3 才变淡）；`ItemIcon` 的两个图标也改成 faint，`className` 交给 `AssetIcon`。
- **B-R1-N3、N4、N5、N7**："Reverted"悬停 `hover:border-border-strong hover:text-foreground`；改动行的"内容"链接用 `lineLinkClass`；"只看此人"加 `-my-1`（第一行 22 px，搜索的行 20 px）；回收站行的"恢复"加 `ms-auto`，说法照实。
- **B-R1-N6**：同 C-R1-M4。

**审查 C 第 1 轮**

- **C-R1-I1**：4.3.6 表里"加载中"的定义改成先 `waitFor` 到横幅，再 `within(main).getByText("Loading…", { selector: "output" })`，用 C 的文字，写明直接 `findByText` 会先拿到外框那个随即卸下的"加载中"。
- **C-R1-I2**：5.3 的 as1 那一句加 `includeHidden: true`，写明 Radix 的模态菜单开着时菜单之外都是 `aria-hidden`。
- **C-R1-M1**（与 A-R1-N2）：表后一段与 6.1 写明页与笔记本设置"读到"两行在旧树上就绿，是搬进表里守着不退回的行。
- **C-R1-M2**：变异 63 的理由改正；`history-page.test.tsx` 的类名断言加 `[@media(hover:none)]:opacity-100`，变异 92。
- **C-R1-M3**：变异 62 写明只在 Firefox 红；(h) 照实记下当时的名字是连字符接字母，连字符接数字的在 (l) 重量。
- **C-R1-M4**（与 B-R1-N6）：`list-rows` 的断言改成 `row.closest(".rounded-card")`。
- **C-R1-N1**：`countOf` 的判断写成 `tree === undefined`；940 加界面的英文名，7.1 的两格改用界面的英文；`:753` 照 C 改；8.2 第 3 条的旧文字延长到"在 Chromium 与 WebKit 上跑"，新文字写明哪些格在哪些浏览器上跑；hi1:258、hi2:113、`page-versions.test.tsx:109` 的注释随测试一起改（行号在修订核对第 2 轮改正为 `:110`）。

**回放**：在新取的 `f62d0390` 副本上先做 2.1 的 14 条、15 处，再在结果上做 8.2 的 16 条、17 处，32 处每段旧文字做之前恰好一次、新文字做之后恰好一次；2.1 的 00 号文档的输出与上一轮逐字节相同。第一次跑 8.2 时第 3 条的标题里出现了"旧文字"三个字，脚本把它当成切分点而报错，改了措辞之后重跑通过。

**计数**：变异 96 条（1–95 与 43b）：72 条只跑 vitest，21 条要跑故事，1 条等价，2 条看截图。

## 修订核对第 2 轮

三位审查者核对再修订的 05-P5（1,398 行）与上一节的复核。第 1 轮的发现：A 4 条、B 11 条、C 7 条都已落实（C-R1-N1 里一处注释的行号写错，由 C-R2-N1 接着处理）。A 照四种写法（`f62d0390` 原样、照设计全做、变异 93、变异 95）跑了店与邀请页的新格：`load` 的再读不转圈、不留下旧的列表、StrictMode 下与接受撞在一起也对，相关的 15 个测试文件、236 格都绿；B 照再修订的类名排了 21 种页 × 四种宽 × 亮暗 × 中英 × 三种引擎，设计者定的两处（变更集页的消息、横幅的小圆点）放在样稿旁边看是对的，带 `whitespace-nowrap` 的小块在 217、272、327、720 px 的实宽上三种引擎 0 溢出、0 劈开、0 单独的"-"；C 独立重放 2.1 与 8.2 的 32 处，与设计者的逐字节相同，7.2 与 7.5 的 96 条对得上，6.1 各提交单独为绿（提交 3 的粗版在七个现有的测试文件、123 格上全过）。新发现：A 1 Minor、1 Nit；B 没有；C 1 Minor、2 Nit。A-R2-M1 与 C-R2-M1 同一个问题。主持者照下表定处置，全部接受，没有新的决定。

### 审查 A

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5A-R2-M1 | Minor | 竞态那一格在 `f62d0390` 上是绿的：今天邀请页不读列表，第一次读要到 `/lab` 才由 `WorkspaceLayout` 发出，那时已接受，答的已含 Lab（探针 V0：`askedBefore: 0`，`h1` "Lab"）；只有改了 `Corner`、没改 `load`（变异 93）时才红，提交 3"在没改的树上看到红"做不到，这一格也分不出"竞态补上了"与"竞态根本没发生"；"没有工作区""引导还有步骤"两格是否定的，旧树上也先绿，要等列表读到再断言"没有"，不然变异 95 下也过 | 接受（同 C-R2-M1）：邀请页竞态那一格按"Accept invitation"之前先等被扣住的那次读已经发出（照 `layout.test.tsx:9-21` 的 `held().asked`），于是在 `f62d0390` 上红在这一步（今天的邀请页不读工作区列表），只改 `Corner` 时红在"Page not found"，全做之后绿；`:960`（6.1 提交 3 的"看到红"）与 `:1011` 照实改，用 C 给的文字；两格否定的格（没有工作区时、引导没走完时没有"回到工作区"）写明在旧树上就绿、由变异 68、95 守着，且先等列表答复再断言"没有" |
| P5A-R2-N1 | Nit | 7.1 店那一行把 `:36-60` 不再读的理由写反：那一格不再读是因为第一次 `load` 已留下列表，"读出去之前改动已答复"只对 `:93-101`、`:188-196` 成立 | 接受：7.1 店那一行把 `:36-60` 不再读的理由（已经有列表）与 `:93-101`、`:188-196` 的（读出去之前改动已答复）分开写 |

### 审查 B

没有新的发现。

### 审查 C

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5C-R2-M1 | Minor | 同 A-R2-M1（C 的探针 `probe2/` 在三棵树上跑：`f62d0390` 上绿，只加 `Corner` 的读时红在"Page not found"，两样都加时过）；"回到工作区"的两格否定的格在旧树上也是绿的，由变异 68、95 证 | 同 A-R2-M1 |
| P5C-R2-N1 | Nit | `page-versions.test.tsx` 的注释是 `:110`，写成了 `:109`；决定 941 加了"引导已走完"之后，2.1 第 6 条（00-M13 4.2.3）与 8.2 第 2 条（13.2 第 17 条）的总述仍只说"账户已有工作区时" | 接受：`:735` 的 `:109` 改成 `:110`；2.1 第 6 条与 8.2 第 2 条的总述加上"引导已走完"，用 C 给的文字，再重放 |
| P5C-R2-N2 | Nit | `titleTextClass` 的 `max-sm:basis-auto`（B-R1-M1 的修法）没有格，也没有变异的行，去掉它什么都不红 | 接受第一种：UI1 第 4 格第 5 步加一句（375 宽时"Plans"与"New page"同一行），加变异 96（去掉 `max-sm:basis-auto`），7.2、7.5 的计数照 C 的写法改；同一行用两者的 `top` 相等或按钮的 `top` 小于标题的 `bottom` 来量，不用行数（Linux 的字体更宽，见 CI 上 PG17 的教训） |

**未核实的怀疑（第 2 轮）**：只有 B 列了，同第 1 轮的三条（静态页；Linux 的字体下标题行与横幅的行数会变，点对第一行的写法与行数无关；真 Safari 与 Windows 高对比的暗色方案），去向同上。

## 再修订时的复核（第 2 轮）

（据设计者交回的要点，2026-11-25。三修稿 `05-P5-pages.md` 1,399 行。这一轮没有跑浏览器探针，没有开工作树。"照处置"即照上表的处置做了。）

**修订的记录**：头的日期一行加"照修订核对第 2 轮三修（A、C 的报告全部接受，B 没有新的发现，没有新的决定）"，状态"三修稿（照修订核对第 2 轮），待修订核对第 3 轮"；2.3 的头一段加"修订核对第 2 轮之后再修订：941 的文字照它改了"，941 补了一句；7.2 的头一段加"修订核对第 2 轮之后再修订加了 96"。

- **A-R2-M1 与 C-R2-M1**：7.1 邀请页一行（`:1011`）的竞态一格改用 `layout.test.tsx:9-21` 的 `held()` 扣住第一次读，按"Accept invitation"之前先 `await` 它的 `asked`：旧树上红在这一步，只改 `Corner` 时红在"Page not found"，全做之后绿。两格否定的格写明在旧树上就绿、由变异 68、95 守，断言"没有"之前先等列表答复（账户按钮与"Accept invitation"都出现、`sent` 里有 `GET /api/v0/workspaces`）。6.1 提交 3 的"看到红"（`:960`）照 C 的文字改，941 补同样的说明。（"旧树上就绿"一句与新加的等待矛盾，由修订核对第 3 轮的 A-R3-M1、C-R3-M1 接着处理。）
- **A-R2-N1**：7.1 店那一行分开写理由：`:36-60` 是第一次 `load` 已经留下列表，`:93-101`、`:188-196` 是改动在读发出之前就答复了、计数不变。
- **C-R2-N1**：`page-versions.test.tsx:109` 改成 `:110`；2.1 第 6 条与 8.2 第 2 条的总述加上"引导已走完"，用 C 的文字。
- **C-R2-N2**：取第一种（有格）：UI1 第 4 格第 5 步加一句，"Plans"与"New page"在同一行，按位置量（按钮的上边在 `h1` 的下边之上），不数行，理由写明是 Linux 的字体更宽；变异 96，由 UI1 第 4 格第 5 步抓，三种浏览器。

**回放**：在新取的 `f62d0390` 副本上先做 2.1 的 14 条 15 处，再做 8.2 的 16 条 17 处，32 处每段旧文字在它那一步之前恰好一次，新文字之后也恰好一次；与上一轮的输出比，00 号文档只差第 6 条那一行（加了"引导已走完"），做完 2.1 的总体设计逐字节相同。

**计数**：变异 97 条（1–96 与 43b），7.2 表里逐行数过也是 97 行：72 条只跑 vitest，22 条要跑故事（UI1 四条），第 15 条等价，第 90、91 条看截图；7.5 照改，故事一项约 70 分钟，总时长仍约 4.5 小时。

**链接与空白**：`check_links.py` 没有新坏的链接；没有行尾空白。

## 修订核对第 3 轮

A、C 核对三修的 05-P5（1,399 行）与上一节的复核（B 第 2 轮没有新的发现，这一轮只动测试与文字，不另请 B）。第 2 轮的发现：A 2 条、C 3 条都已落实。A 在旧树、全做、变异 93、变异 95 与变异 68 上跑了竞态一格与两格否定的格，竞态的三种结果与设计者说的一致（旧树上红在等 `asked`，只改 `Corner` 时红在"Page not found"，全做之后绿），两格否定的格确实等到列表答复才断言、变异 68、95 各由一格抓到；C 在 `f62d0390` 与新头 `3e288acb` 上各重放 2.1 与 8.2 的 32 处，都各恰好一次、与设计者逐字节相同，变异 96 的位置比较在三种引擎、五种字体、320/375/390 宽的 90 种里两次都对，7.2 的 97 条对得上。核对期间 `m13-ui-refresh` 的头从 `f62d0390` 走到了 `3e288acb`（合并 `9871c7d4` 与文档 `3e288acb`，CI 的两处修复，04-P4 10.4），C-R3-M2 由此而来。新发现：A 1 Minor、1 Nit；C 2 Minor、1 Nit。A-R3-M1 与 C-R3-M1 同一个问题。主持者照下表定处置，全部接受，没有新的决定。照 P4 设计的先例，最后这一轮只剩文字，由主持者直接改进设计（02-workflow 第 1 节第 7 步）：用 C 给的十处文字（`p5-rev-c/check-3.md.applied.md` 与改之前的设计只差这十处），A-R3-N1 照 A 的意思补进 `:1011`；改动的 diff 在 `p5-design/r3-moderator.diff`，再请 A、C 做第 4 轮。

### 审查 A

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5A-R3-M1 | Minor | 两格否定的格断言之前要等 `sent` 里出现工作区列表的 GET（第 2 轮加的），`f62d0390` 的邀请页不读这个列表，这一步一直等不到，两格在旧树上红（探针 V0 两格都超时）；设计在 941（`:245`）、6.1 提交 3（`:960`）、7.1（`:1011`）三处写成"旧树上就绿"；实施者若为求"绿"去掉那一步等待，又回到第 2 轮的漏洞 | 接受（同 C-R3-M1）：`:245`（941）、`:960`（6.1 提交 3）、`:1011`（7.1）照 C 的文字改成"两格否定的格在旧树上红在等列表那一步（邀请页还不读列表），全做之后绿，链接的条件由变异 68、95 证" |
| P5A-R3-N1 | Nit | `layout.test.tsx:9-21` 的 `held()` 每一次请求都扣住，`release` 只放开当时已在等的；全做之后 `load` 会再读一次，整条路由都交给 `held()` 时这次再读永远不答，对的实现停在"加载中"（探针"every read held"）；`invitationServer` 记的是完整地址（`GET http://nervewiki.test/api/v0/workspaces`），写成 `toContain("GET /api/v0/workspaces")` 在对的实现上也等不到 | 接受：`:1011` 竞态一格写明只有第一次读交给 `held()`、之后照 `joined` 答；否定的两格写明 `sent` 里有一项以 `/api/v0/workspaces` 结尾的 GET（`endsWith`），不比全文 |

### 审查 C

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5C-R3-M1 | Minor | 同 A-R3-M1（C 的探针 `probe3/`，每种 15 次：`f62d0390` 上两格 15/15 都在等待上超时；变异 95 的树上"引导还有步骤"一格 15/15 红；改好的树上都绿） | 同 A-R3-M1 |
| P5C-R3-M2 | Minor | 基线与 CI 的说法在 `3e288acb` 之后不再成立：负责人已推送 `f62d0390`，CI 跑了 run 45551442052（web、server 各分片、image 绿；e2e 红在 PG17 第 2 格的 Chromium、WebKit 与 C8，由 `9871c7d4` 修好，修好的头还没推送），E2E 一步 8 分 22 秒；944（`:248`）、7.3（`:1180`）、7.5（`:1193`）还写"M13 还没在 CI 上跑过"；`:10` 写"行号是 `m13-ui-refresh` 的头 `f62d0390` 的"，`f62d0390..3e288acb` 设计引的行里只挪了 pg17:654 与 `notebook-pages.ts:17-37,55`。`05-lessons.md` 新加的两条，设计里的新检查都合乎，不另立条 | 接受：`:10` 写明行号以 `f62d0390` 为准、之后的 `9871c7d4`、`3e288acb` 只挪了 pg17:654 → `:677` 与 `notebook-pages.ts:17-37,55` → `:17-41,59`；`:248`（944）、`:1180`（7.3）、`:1193`（7.5）照 run 45551442052 改，用 C 的文字 |
| P5C-R3-N1 | Nit | 邀请页现在加五格，变异 68 的抓手"`invitation.test.tsx` 新格的后一半"、变异 69 的"新格"是一格时代的写法，指不清是哪一格 | 接受：变异 68、69 的抓手写明是哪一格 |

**主持者另改的**：头的日期一行加"主持者照修订核对第 3 轮改了 A、C 的两条 Minor 与两条 Nit（大半用审查 C 给的文字，没有新的决定）"，状态"四修稿（主持者照修订核对第 3 轮），待修订核对第 4 轮"；2.3 的头一段记下 941、944 的文字照第 3 轮改了。另把 2.1 第 12 条（00 号文档第 13 节的一行）的锚点从"它不隐去（04-P4 第 9 节"负责人"一条））。"换成"只改测试，由主持者核对。"，题注加"在 CI 的两处修复那一行之后，04-P4 10.4"：`3e288acb` 的 00 号文档第 13 节里，P4 完成的那一行之后是 CI 修复的一行，P5 的那一行要接在它后面；照旧的锚点会把 P5 的一行插在两者之间。新锚点是 CI 修复那一行的末句，在 `3e288acb` 上恰好一次，`f62d0390` 上没有。

## 修订核对第 4 轮

A、C 核对主持者照第 3 轮改的四修稿（1,399 行；`p5-design/r3-moderator.diff`）。A：第 3 轮的两条都照它的意思改了，改后的文字与它第 3 轮的探针结果一一对得上（竞态一格只有第一次读交给 `held()`；否定的两格以 `endsWith` 等到完整地址的那次读），变异 68、69 的抓手与探针一致；`f62d0390..3e288acb` 没有动它探针依赖的文件，pg17 里 `expectFits(` 的调用前后都是 3 处；没有新的发现。C：第 3 轮三条都解决，全文再没有说两格否定的格"在旧树上就绿"的地方，也不再有"还没在 CI 上跑过"；主持者换的 2.1 第 12 条的锚点对，照旧的锚点会让 P5 的一行落在"P4 完成"与 CI 修复那一行之间（C 第 3 轮在新头上只核对了次数，没有看出这一处，自己记下是疏漏）；在 `3e288acb` 上以自己的脚本重放 2.1 的 15 处、8.2 的 17 处，全部 OK，输出与主持者的逐字节相同，00 号文档只差 P5 那一行从第 579 行挪到第 580 行；A 在 `:1011` 的两处补充读得接上、所说的对。新发现：C 1 Minor、1 Nit。主持者照下表定处置，全部接受，没有新的决定。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5C-R4-M1 | Minor | 换了 2.1 第 12 条的锚点之后，2.1 在 `f62d0390` 上已经放不了（第 12 条的旧文字在那里 0 次），可是 `:10`、`:67`（第 1 节 (k)）、`:82`（2.1 的头一段）、`:1208`（8.2 的头一段）四处仍说重放在 `f62d0390` 的副本上做；失败是响的，但这几句是门禁的记录，要与做法一致。行号仍以 `f62d0390` 为准，不用改 | 接受：主持者换锚点引出的，四处改成 `3e288acb`；照 C 的 `check-4.md.applied.md`（与设计只差这四行）原样换上；主持者在 `3e288acb` 上重放 2.1 的 15 处、8.2 的 17 处，三份输出与 C 的 `r4c-applied/` 逐字节相同 |
| P5C-R4-N1 | Nit | 2.3 头一段新加的一句写成"两格否定的格"，多一个"格"（C 报告的前一版立了这一条；主持者先已改好，C 的定稿报告只记一句、不再立条） | 接受：`:211` 改成"否定的两格" |

另：头的日期与状态照实记定稿（`p5-design/r4-moderator.diff`）。02-workflow 第 1 节第 7 步要一轮一轮核到没有 Important 与 Minor，C 的这一条 Minor 由主持者改，另请 C 做第 5 轮核对。

## 修订核对第 5 轮

C 核对第 4 轮的改动（`p5-design/r4-moderator.diff` 与处置）：P5C-R4-M1 的四处新文字原样在设计里、各出现一次，旧文字一处不剩；"否定的两格"在设计里（处置把它记作 P5C-R4-N1、头里记作"一条 Nit"，C 认为说得过去）；在 `3e288acb` 上读今天的设计重放 2.1 的 15 处、8.2 的 17 处，全部 OK，三个输出与 `r4c-applied/` 逐字节相同；`check-4.md.applied.md` 与设计只差头的两行；头的日期一行对，状态一行的 `[审查记录](reviews/P5-design-review.md)` 是真链接，本文与设计在同一个提交里进仓库就不断（P4 的先例 `92cbffd9`）。没有 Important、Minor，新发现一条 Nit。

| # | 级别 | 发现 | 处置 |
|---|---|---|---|
| P5C-R5-N1 | Nit | 状态一行写到 C 第 4 轮的 Minor 由主持者改、C 重放过为止，没有记下 C 的第 5 轮；照 P4 的先例，状态记的是每位审查者最后那一轮没有 Important、Minor | 接受，主持者补上（02-workflow 第 1 节第 7 步）：状态一行记下 C 在第 5 轮核对过、没有 Important、Minor |

设计到此定稿：B 在第 2 轮、A 在第 4 轮、C 在第 5 轮都没有 Important、Minor。2.1 的十四条（十五处）由主持者随设计的提交在 `3e288acb` 之上落进 00 号文档与总体设计（脚本依次替换，每段旧文字、新文字各恰好一次）。
