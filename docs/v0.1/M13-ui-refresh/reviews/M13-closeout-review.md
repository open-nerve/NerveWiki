# M13 界面焕新：收尾审查记录

| 项 | 内容 |
|---|---|
| 审查对象 | `m13-ui-refresh` 的 `eb48f4a1`（`docs: M13/P5 done`）：五个 Phase 全部合并之后，P1 完成于 `4c3f10b9`，P2 于 `ba66557c`，P3 于 `072dcec2`，P4 于 `f62d0390`（CI 跑出的两处 `9871c7d4`、`3e288acb`），P5 于 `eb48f4a1`；`git diff main...m13-ui-refresh`（合并基 `3c315037`）349 个文件 |
| 方式 | 三位独立审查者并行（[00 号文档](../00-M13-design.md)第 7 节"收尾"）。A 外框与设置：设计基础、新外框与设置模式（P1–P3），以及后面的 Phase 怎样落在它们上面；分离的 worktree 里构建，探针经 `heavy.sh` 在三种引擎 × 1440、1024、768、375、320 × 亮暗 × 中英里普查 23 种页加登录、邀请与引导（2,000 余格），另做行为、长名字、强制色与 Firefox 的 Tab 次序几组。B 阅读、编辑与其余页面（P4、P5）：分离的 worktree 里构建，探针 r1–r10 经 `heavy.sh`，18 种页 × 浅深 × 五种宽度 × 中英、三种引擎，两处修法在工作树里试过再退回。C 完成标准、文档与移交：只读（`git`、`gh` 只读、`check_links.py`、两个 `check.mjs`），一位只读的帮手核对 M12-polish 的"处理进展"。heavy 锁由 A、B 轮流占着，C 没有跑重的东西。修复核对里三位各在自己的分离工作树里经 `heavy.sh` 跑：A、B 重跑自己的探针、相关的故事与 vitest；C 跑合并之前的整条命令、重放实施者的变异并另加，定文档脚本的文字。审查与核对用的工作树都已删；主检出与仓库没有改动、没有提交 |
| 日期 | 2026-11-26 |
| 结论 | A：0 Important、3 Minor、3 Nit、2 Q；普查的 2,000 余格没有一格横向溢出，没有一个元素逃出视口。B：1 Important（编辑时大纲的第一次跳转把标题停在视口之上）、3 Minor、2 Nit；18 种页 × 各条件不横滚，强制色下 70 个可聚焦项都有环。C：1 Important（`main` 上本机的 `make e2e` 不会是 rc 0）、6 Minor、10 Nit、2 Q；第 3 节七条、修订 1–9、移交、五份 Phase 文档第 9 节全表。同一件的：A-N3 = B-M2 = P5S3B-N3（Firefox 面包屑里单独一行的"/"）；A-N1 = B 第 9 节第 4 项 = 05-P5 第 9 节的齿轮；B-M1 = 05-P5 第 9 节的"按住的键"；B 第 9 节第 7 项 = P5S3C-F1-N3（`expectFits` 在弹入中量）；A-Q2 与 C 的未核实一条（"工作区设置"）。全部接受；主持者的决定 961–971（00 号文档第 11 节"M13 收尾的决定"）。修复在分支 `m13-closeout`，十五个提交，只改 `web/`、`e2e/` 与 `tools/e2e-projects/check.mjs`：五个提交到 `f5d84965`（`f181683f`、`8f8e6145`、`8ff51ade`、`7cacd2a2`、`f5d84965`）。修复核对第 1 轮 A 2 Nit，B 1 Minor、2 Nit，C 2 Minor、5 Nit、1 Q，全部接受，修复轮 2 是 `e1b24d5e`、`0d6b2b04`、`e404d0d7`，另有 AS6 的偶发 `aad1ab44`。第 2 轮 A 2 Nit，B 1 Nit，C 2 Minor、2 Nit；C-F2-M2 不改，其余接受，修复轮 3 是 `91a62e60`、`41fbce8f`、`f2130709`，另有 C8 的偶发 `4470c25a`。第 3 轮 A 没有新的，B 1 Nit、1 Q：F3-N1 接受，只改测试，修复轮 4 是 `524734f1`；F3-Q1 不改。C 的第 3 轮 1 Nit：F3-N1 接受，修复轮 5 是 `d734488f`，B 核（第 4 轮）。合并回 `m13-ui-refresh` 是 `<待填>`；`m13-ui-refresh` 合并进 `main` 是 `<待填>` |

## 完成标准核对

00 号文档第 3 节逐条，证据是 C 在 `eb48f4a1` 上读代码、`git` 与 CI 得的（文件与行号是 `eb48f4a1` 的；`W/` 是 `web/apps/web/src/`），A、B 另核了外框、设置、阅读与其余页面的部分。

| # | 条 | 证据 | 结论 |
|---|---|---|---|
| 1 | 六条目标、亮暗 × 1440/375、每个 Step 的截图写明看了哪些 | 13 份 Step 记录都有"视觉的核对"一节，每份都写了亮暗、1440 与 375 的页；截图不进仓库（决定 725）。A、B 的普查另在五种宽度、两种主题、两种语言下核对 | 成立（外观由 A、B 判） |
| 2 | 现有 vitest 与 e2e 通过；新交互有 vitest；四个新故事通过 | 新加的 vitest 文件 30 个（`shell-drawer`、`settings-directory`、`settings-origin`、`name-row`、`current-heading`、`edit-outline`、`code-blocks`、`notebook-slot` 等）；⌘K 的让路在 `search-box.test.tsx:247-248,282`；新故事 UI1–UI3、PG16（另 PG17）都在 `threeBrowsers` 与 `check.mjs` 的 `REQUIRED`；`node tools/e2e-projects/check.mjs` rc 0（chromium 434、firefox 105、webkit 105、webkit-narrow 3，647 格）；各 Step 合并之前的整条命令（05-P5 7.3：`8a89e184` 上 643 / 3 / 1，三格是本机环境）与 CI | 成立：`eb48f4a1` 的 CI e2e 646 过、1 跳（A15 在 WebKit 上的第 3 格），没有重试。收尾之后 vitest 234 个文件 5623 格，E2E 656 格，`check.mjs` 437/108/108/3（`524734f1` 的门禁） |
| 3 | 门禁照旧为绿 | 每个 Step 的逐提交 `make lint-web knip test-web` 与合并之前的整条命令；`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 各 Step 以 0 退出，`main...m13-ui-refresh` 上重跑也是 0；CI 见"CI 的数" | 成立；E2E 一步变长（C-M6） |
| 4 | 13.2 第 31 条与对比度三个故事 | 13.2 第 31 条的对比度与焦点照 4.7 改写（P1/S1）；`bdi`、`min-w-0`、`contain-paint`、375 px 几句在；pg4、se1、l5 的守卫在；13.4 第 3 条"正文的字至少 7:1，其余按实量的值减 0.1" | 成立 |
| 5 | 没有服务端与契约的改动 | `git diff --stat main...m13-ui-refresh -- server api` 为空；改动只在 `web/apps`（271）、`e2e`（42）、`docs`（35）、`tools/e2e-projects/check.mjs`（1） | 成立。收尾的修复同样：`eb48f4a1..d734488f` 的 68 个文件是 `web/` 60、`e2e/` 7、`tools/e2e-projects/check.mjs` 1；上面那条 `git diff --quiet` 实施者在 `f5d84965`、`e404d0d7`、`f2130709` 上、C 在 `f5d84965`、`aad1ab44` 上都以 0 退出，`eb48f4a1..d734488f` 上也是 0 |
| 6 | 本 M 没有 `open` 的移交；M13 认领、做掉的进"处理进展" | `docs/v0.1/M13-ui-refresh/handoffs/` 只有 `.gitkeep`；M12-polish 的处理进展有 P1/S2、P2/S2、P2/S3、P3/S2、P4/S1–S3、P5/S1–S3 十行，对着代码都成立 | 成立；C-M1、M2、M5 由收尾补（见"移交"） |
| 7 | 收尾之后合并进 `main`、完整门禁、`main` 的 CI 绿、"实现完成，待人工验收" | 合并进 `main` 之后写：合并 `<待填>`、"全量门禁"一节与 `main` 的 CI | 本机的 `make e2e` 照决定 961；合并进 `main` 之后写 |

## 发现与处置

### A（外框与设置）

| 发现 | 处置 |
|---|---|
| M1 左栏里截短的名字没有 `title`，只在末尾不同的名字分不出来；P5 的卡片已经有（决定 931） | 接受：工作区切换的按钮与菜单项、笔记本列表、笔记本那一行、树的行（名字包进带 `title` 的 `span`，不放在链接上）、账户行与菜单、设置目录的"返回"与组名带完整名字的 `title`；决定 971：切换器的菜单宽改为 `min-w-(--radix-dropdown-menu-trigger-width)` 加一个不超过窗口的上限。vitest 断言，变异一行（`f181683f`）。修复核对之后菜单离窗口两边各 8 px（A-F1-N1，`0d6b2b04`），菜单里的名字最多两行（A-F2-N2，`f2130709`） |
| M2 强制色下，树里当前页的那一行、LLM Wiki 的"目录""历史"的当前一支、笔记本首页时的笔记本那一行没有记号；设置目录的当前行有 | 接受：三处加一圈只在强制色下才有的外框（同决定 845），平时不变；类名断言，变异一行（`f181683f`）。修复核对第 1 轮之后，行里的链接有键盘焦点时外框让位给焦点环（A-F1-N2，`0d6b2b04`） |
| M3 Firefox 里左栏会滚的中段（`[data-sidebar-scroll]`）是一个没有名字的 Tab 站 | 接受：`tabIndex={-1}`；焦点在树里时 PageDown 照样滚它。另查 P4 的右栏：Firefox 里正文的 Upload 之后先停在右栏的 `aside`，是同一种 Tab 站，`page-panel.tsx` 的 `aside` 同样改（Chromium 两处都不停）。e2e：PG16 第 2 格加一步，Chromium 与 Firefox 里从"搜索"按 Tab 下一站是"首页"，变异红在 Firefox；PG17 第 1 格加右栏的一步（`f181683f`）。REQUIRED 加 PG16 第 2 格的 Firefox（C-F1-N1，`e1b24d5e`） |
| N1 齿轮与＋没有 `any-pointer-coarse:opacity-100`（= B 第 9 节第 4 项 = 05-P5 第 9 节） | 决定 966：齿轮与＋、树行的"⋯"、列表行的 `revealClass` 用同一套条件（悬停或聚焦、`[@media(hover:none)]`、`any-pointer-coarse`、强制色），各自静止时的不透明度照旧；`shown` 加类名断言，变异一行（`f181683f`）。树行"⋯"的窄屏是修复核对第 1 轮补的（C-F1-N5，`0d6b2b04`）。真机没有核实，进 v0.2 的移交第 10 项 |
| N2 窄屏下名称的输入框占满一行、下拉只有自己的宽度 | 决定 970：下拉在 `md` 以下 `max-md:w-full`（开放程度、成员与邀请的角色）；375、320 宽截图核对（`f181683f`） |
| N3 Firefox 的长面包屑在 1440 宽也有一行只有"/"（= B-M2） | 见 B-M2 |
| Q1 名称一行的草稿：Esc 不退回，离开这页就悄悄丢掉 | 决定 965：Esc 退回已保存的名字，"保存"随之消失（同下拉的 Esc，00 号文档 4.4.3）；离开时丢掉草稿照旧。vitest 两格，变异一行（`f181683f`） |
| Q2 左栏的"设置"与账户菜单的"设置"同名，样稿与 00-M13 4.2.1 写的是"工作区设置"（= C 的未核实一条） | 决定 964：左栏的改名"工作区设置"（"Workspace settings"）；i18n 两种文字与按名字找它的测试与故事（UI2）；读屏清单 H 第 6 项与 README 随之写（`f181683f`） |

### B（阅读、编辑与其余页面）

| 发现 | 处置 |
|---|---|
| I1 编辑时大纲的第一次跳转，标题停在视口之上（"Tasks" −44 px、"Callouts" −56、"Code" −59，三种引擎同样；CodeMirror 对没画过的行的高度是估计的） | 接受：跳转之后在 CodeMirror 量完行高时再对准。实施者取的是确定的信号 `update.heightChanged`，不隔帧，理由见"实施中定的"。e2e：PG17 新一格，三种引擎，长页、页顶进编辑、点大纲里还没画过的"Tasks"与"Callouts"，按位置等那一行连续五帧不动，断言它的 `top` 在 [0, 48]；vitest 没改：jsdom 不量高；变异一行，红在 e2e 那一格（`8ff51ade`）。修复核对里又加两条结束对准的条件（B-F1-N1 `e404d0d7`、B-F2-N1 `41fbce8f`），PG17 那一步的量法改过一次（B-F3-N1 `524734f1`） |
| M1 按住 Enter 关掉对话框，重复的 Enter 落在触发它的按钮上，又打开它；从树的菜单进来的还会新建一页（= 05-P5 第 9 节的"按住的键"） | 决定 967：`dialog.tsx` 与 `alert-dialog.tsx` 两种 `Content` 的 `onCloseAutoFocus` 里挡掉重复的 Enter 与空格，直到放开一个键，传进来的 `onCloseAutoFocus` 照旧调到；B 量到的各条路（还原、撤销、导出、新建笔记本、树的"⋯"→ Move/Delete、附件的 Delete、导入的 Cancel）都核对；导入表单自己的 `onKeyDown`（决定 960）照旧。`dialog.test.tsx` 三格，PG16 第 1 格加一步，变异一行（`8f8e6145`）。修复核对扩到菜单与编辑时的大纲（B-F1-M1 `e1b24d5e`），再扩到按下一项之后焦点去的地方（C-F2-M1、A-F2-N1 `91a62e60`） |
| M2 Firefox：面包屑里单独一行的"/"，各种宽度、不只连字符加数字（= A-N3 = P5S3B-N3） | 决定 968：两种放法都量了，取前一段链接的末尾（焦点环圈住"名字 /"），`8ff51ade`；扫描又找到正好占满一行的名字仍剩"/"，补齐的 `f5d84965` 让名字的最后一个字素与分隔符一起不断行。`aria-hidden` 照旧，链接的名字不变。e2e：PG17 新一格，三种引擎，页与版本页在 320、375、768、1440 宽，并把面包屑的列表从 120 到 1200 px 每 1 px 设一遍，按位置核对每个分隔符与旁边的字在同一行；变异红在它（第一种放法只红在 Firefox，补齐的一条三种引擎都红） |
| M3 "一页"有三个图标，同一个图标在别处又是别的意思 | 决定 969：页一律用左栏树的 `File`（共用一个 `PageIcon`：快速切换、子页面列表、首页的空状态、搜索的命中、回收站里的页），来源页照旧 `FileText`；附件自己一套：图片照旧，PDF 用 `FileType`，别的文件与没有附件记录的命中用 `Paperclip`（照 lucide-react 1.48.0 核对），回收站里附件随它。vitest 断言，变异一行（`7cacd2a2`） |
| N1 改名的对话框没有"取消"，"保存"在左边 | 接受：`W/app/rename-form.tsx` 在对话框里时按钮行右对齐、加 `DialogClose` 的"取消"（`flex flex-wrap justify-end gap-2`，同决定 958）；页与附件的改名都用它。vitest 两格（`8f8e6145`） |
| N2 冲突面板里两种按钮 | 接受：对比视图的"上一处""下一处"与"显示没改的行"随决定 878 换成 `size="sm" shape="pill"`，历史页的对比一起变，`buttonClass` 连带比较失败时的"重试""重新加载"；类名断言（`8ff51ade`）。`ChunkNotLoaded` 的两个按钮是修复核对第 1 轮补的（B-F1-N2，`0d6b2b04`） |
| 第 9 节第 7 项 `expectFits` 可能在弹入中量（= P5S3C-F1-N3） | 接受：`e2e/fixtures/narrow.ts` 的 `expectFits` 与 `expectButtonsInside` 共用"先等框与祖先的有限动画结束"那一段（`animationsEnded`），量页面时照旧（`7cacd2a2`）。`expectFits` 那一半只由探针证明（见"变异"） |

### C（完成标准、文档与移交）

| 发现 | 处置 |
|---|---|
| I1 收尾第 4 步"`main` 上的完整门禁"里的 `make e2e` 在这台机器上不会是 rc 0（SE2 `se2:231`、LW8 `lw8:836` 每次红，PG15 `pg15:113` 时红时过；三格在 M13 的基线就有，Linux 的 CI 上都过，决定 764、795(a)） | 决定 961：`main` 上本机的 `make e2e` 只允许这三格失败，同行同消息，行号或消息变了先停、查清；绿的证据是 `main` 合并之后那一次 CI 的 e2e；"全量门禁"的表照实写 rc 与三格 |
| M1 00-M13 说写进 v0.2 移交的两件（搜索摘要、像素级回归），v0.2 的移交里没有 | 决定 962：新建 [v0.2 的移交](../../../v0.2/handoffs/M13-ui-refresh.md)第 1、2 项 |
| M2 Phase 第 9 节交给 v0.2 的几件（决定 891、871、914，05-P5 第 9 节收尾的五件，P4 各 Step"另记给 v0.2"的六件）没有落到编号的条目 | 决定 962：同上，第 3–9 项；05-P5 第 9 节收尾的五件里收尾修了四件（决定 966、967、968，B 第 9 节第 7 项），958 的七行是第 9 项；另加第 10 项（可能用手指点的设备，决定 953、966） |
| M3 P4 交给负责人的 Safari 核对没有进人工清单 | 接受：manual/README.md"其余各项"加第 15 项；v0.1.1 移交第 1 项"十五项"、"A–H" |
| M4 总体设计的头仍写"覆盖 M0–M12" | 接受：改"覆盖 M0–M13"，第 15 节的收尾一行 |
| M5 M12-polish.md 的条目里有 M13 做掉、已不成立的文字 | 接受：逐项注明"M13/P?/S? 做了"（第 1 项的 12.10、13.1、16.7、19.2、26.3、35.7、37、38.5，第 5、6、8、11、14 项），第 1 项重数（余下 81 项 C 类）；"处理进展"加"M13 收尾"一行 |
| M6 `eb48f4a1` 的 CI 数还没写进文档；E2E 一步涨到 12:01 | 接受：写进 00-M13 第 9 节、05-P5 的 7.3、7.5、00-M13 第 13 节；看法见"CI 的数" |
| N1 00-M13 收尾第 1 项"输入法清单第 26 步"：清单只有 23 步，是第 26 行 | 接受：改"第 26 行" |
| N2 01-P1 的结果两节仍是"合并见 00 号文档第 12 节"；P1-S1、P1-S2 的记录里没有合并号 | 接受：`a08bf50f`、`e2678ff7`（`git log --first-parent m13-ui-refresh` 核对过），两份记录的结论里同样写上 |
| N3 `reviews/M13-design-review.md` 的结论仍是"待修订核对第 1 轮" | 接受：写上第 1、2 轮的数与"设计定稿" |
| N4 `reviews/P3-design-review.md` 末尾多一个空行 | 接受：去掉 |
| N5 13.2 第 31 条决定 958 那一句读起来是通则，七行没有 `flex-wrap` | 接受：补"今天放得下的七行照旧，文字变长时照此改"；v0.2 的移交第 9 项 |
| N6 M12-polish 的 P2/S3 一行说 `aria-keyshortcuts` 列"`Mod+K` 与 `Mod+Shift+F`" | 接受：写成代码的两种（`Meta+K Meta+Shift+F` / `Control+K Control+Shift+F`） |
| N7 05-P5 第 9 节说齿轮"静止时看不见"，代码是 `opacity-40` | 接受：改成"静止时只是淡（0.4）" |
| N8 00-M13 收尾第 4 项的命令表有两处重复，少 `pnpm install --frozen-lockfile` | 接受：去重、补上；镜像在合并之后的头上另 `git clone` 的普通克隆里建（C 第 6 节：主检出有未跟踪的 `.claude/launch.json`、`summary.txt`，会被记成 modified） |
| N9 M12-polish 第 9 项引的 `trash-preview.tsx:124` 漂了 | 接受：写成 `startItem` |
| N10 00-M13 4.7 的"琥珀对白约 2.99:1、对侧栏约 2.74:1"是换色之前的数 | 接受：`#c2871f` 按 WCAG 的公式是 3.10 与 2.84 |
| Q1 发布说明要不要写一条"视觉优先"的已知限制 | 决定 963：`CHANGELOG.md` 0.1.0 的"已知限制"加"对比度让位于外观"，比值照现在的色值重算 |
| Q2 M13 自己推后的事放在哪 | 决定 962：新建 `docs/v0.2/handoffs/M13-ui-refresh.md`，M12-polish 加一行指过去 |

## 主持者的决定

写在 [00 号文档](../00-M13-design.md)第 11 节"M13 收尾的决定"，负责人可以改判：决定 961 `main` 上本机的 `make e2e` 只允许三格，多出来的红先看服务端的日志，是数据库或机器的停顿就单独重跑那几格、过了照实写（C-I1、C-F1-Q1）；962 新建 v0.2 的移交 `M13-ui-refresh.md`，十一项（C-Q2、M1、M2）；963 发布说明加"对比度让位于外观"（C-Q1）；964 左栏的"设置"改名"工作区设置"（A-Q2）；965 名称一行的草稿按 Esc 退回（A-Q1）；966 三种悬停才显出的控件用同一套条件（A-N1、C-F1-N5）；967 按住的 Enter 与空格不再按到焦点去的地方：对话框关上之后、菜单项与编辑时的大纲上、按下一项之后焦点去的地方（B-M1、B-F1-M1、C-F2-M1、A-F2-N1）；968 面包屑的分隔符不再单独落到一行（B-M2）；969 一页一个图标（B-M3）；970 设置的行里下拉在 `md` 以下占满一行（A-N2）；971 切换器的菜单宽跟着名字走、有上限，离窗口两边 8 px，名字最多两行（A-M1、A-F1-N1、A-F2-N2）。

主持者另定、没有编号的：AS6 与 C8 的偶发都是格自己的时序，现在就修、只改测试（见"核实修复"）；B-F3-Q1 不改。

## 实施中定的

- **B-I1 的信号**（处置让实施者选）：用 CodeMirror 的 `update.heightChanged`，不数帧。`reveal` 照旧派发光标与 `scrollIntoView(at, y: start, yMargin 16)` 并聚焦；之后每个 `heightChanged` 的更新在自己的微任务里再派发一次同一个 `scrollIntoView`，条件是文档与光标没变；至多 4 次、1 秒之内；读者的 wheel、pointerdown、touchstart、keydown（window 捕获、passive）与编辑器的销毁都结束它。理由：`requestMeasure` 只请求下一轮量，不说量完了没变；`heightChanged` 恰是"量出来的高与估计不同"，没变就不再动。4 次与 1 秒是实施者定的。实施者的探针里修好之后 45 次跳转都停在窗口顶下 9.3–11.5 px；修之前 Firefox 里 Callouts −31.28、Code −34.72，Diagram、Part 5 本来就对。
- **I1 那一格每跳一次重新打开页，不按"完成"**：Firefox 里长页跳下去之后按"完成"报"scroll-linked positioning effect"的警告，页面的安静检查就红（第一次门禁里 PG17 [firefox] 因此红过，改格之后重跑才提交）。修之前也报，是 04-P4 第 9 节记给 v0.2 的那一条；375 宽从页下面的大纲往上跳也报。警告本身没修，写进 v0.2 的移交第 8 项。
- **决定 968 的放法**：两种都量（提交 2 的二进制，Firefox，300–1440 每 4 px，页与版本页，三组名字）。原样 53–70 档有孤零零的"/"；放进后一段链接的开头，页 48、49、70 档，版本页 0、0、61 档；放在前一段链接的末尾，页 0、0、13 档，版本页 0、2、25 档。焦点环：放在末尾的圈住"名字 /"，放在开头的圈住"/ 名字"；取前一段的末尾，读起来是"这一段到此为止"。之后的扫描找到它在名字"哪里都能断"、正好占满一行时仍剩"/"（三种引擎，页 6/286 档、版本页 5/286，Firefox 10/286），所以加了第五个提交 `f5d84965`：名字的最后一个字素（`Intl.Segmenter`，应用里已有同样用法）与分隔符一起包进 `whitespace-nowrap` 的 `span`；之后扫描三种引擎全部 0/286。按 `textContent` 读一段的测试去掉 `aria-hidden` 的部分（`W/test/read-text.ts`，e2e 的 `breadcrumbs()`）。照"修正是新的提交"，没有改提交 3，所以修复是五个提交，不是处置说的三到四个。
- **A-M3 的右栏**：基线的二进制上量到 Firefox 里正文的 Upload 之后先停在右栏的 `aside`（About this page），再到 Outline 的 summary；Chromium 不停，WebKit 在 macOS 上只 Tab 到表单与 summary。是同一种 Tab 站，`page-panel.tsx` 的 `aside` 同样加 `tabIndex={-1}`，`page-panel.test.tsx` 断言属性，PG17 第 1 格的一步在 Firefox 先红过。
- **决定 969 的图标**：页 `File`（新文件 `W/components/page-icon.tsx` 的 `PageGlyph`、`PageIcon`），PDF `FileType`，别的文件 `Paperclip`；类名照已装的 lucide-react 1.48.0 核对。处置没列的两处一起改：笔记本首页"还没有页"的空状态原来是 `FileText`（树里它是来源页），改用 `PageGlyph`；搜索命中与回收站项没有附件记录时原来是 `File`，改 `Paperclip`。附件行里标"来源"的 `FileText` 小图标照旧，与树里来源页同一个意思。
- **决定 971 的上限**：`max-w-[min(20rem,var(--radix-dropdown-menu-content-available-width))]`，三种引擎里菜单都是 320 px。修复核对之后加 `collisionPadding={8}`（320 宽时 8–312，宽 304 px），菜单里的名字 `line-clamp-2` 加 `[overflow-wrap:anywhere]`、项 `h-auto min-h-8 py-1.5`（两行的项 53 px，一行的 32 px）。
- **`swallowHeldKeys` 比 B 的补丁多一条**：新按下的 Enter 或空格也结束它，所以用指针关上的对话框不吞之后按住的键（`dialog.test.tsx` 的守卫一格）。
- **B-N2 的 `buttonClass`**：它也是比较失败时"重试""重新加载"的类，两个一起成了小药丸（编辑器不 import `components/ui`，写的是同样的类）。
- **`pressedOnce` 也给单选项**（B-F1-M1）：处置只点名 `DropdownMenuItem`；`DropdownMenuRadioItem` 是同一个机理（工作区切换、外观的选择项按住会选、关、再开），一并加了。
- **笔记本行的让位写成 `not-has-[h2>a:focus-visible]`**（A-F1-N2）：齿轮也是 `a`，它聚焦时首页的框留着、齿轮的环画在框里；树行只有一个链接，写 `not-has-[a:focus-visible]`。构建产物里核过生成的选择器。
- **B-F1-N1 用 CodeMirror 自己的 scroll 观察者**（`EditorView.domEventObservers` 的 `scroll`），不用 window 的监听：第一版用 window 捕获阶段的监听，Firefox 照样拉回（3/3），因为 CodeMirror 的 gap `IntersectionObserver` 先于 scroll 事件触发量（那一次的日志被同名的下一次覆盖了）。写成 PG17 一格确定的格。
- **REQUIRED 加的格**：PG17 的 I1 与面包屑两格的 Firefox、WebKit（`8ff51ade`，`check.mjs` 434/105/105/3 → 436/107/107/3）；PG16 第 2 格的 Firefox（C-F1-N1，`e1b24d5e`，数不变）；PG17 的 B-F1-N1 那一格的 Firefox、WebKit（`e404d0d7`，→ 437/108/108/3）。
- **PG17 滚动那一步的断言改了两次**：`41fbce8f` 写成"每一帧离滚后的位置不超过这次滚动的一半"：往上滚到编辑器只估计过行高的地方，编辑器量完行高把窗口挪 166 px 让屏幕上的内容不动，这不是拉回，按 100 px 写修好的代码也红。B 第 3 轮指出这一挪因页而异，`524734f1` 改成比那一行在屏幕上的位置（见"核实修复"的修复轮 4）。

## 变异

做法照 02-workflow 3.1：一次只改一处；先在不带变异的头上跑对着的格（各轮合并之前的整条命令里它们都是绿的）；跑完从备份拷回，以 `git diff --quiet HEAD -- <文件>` 核对，不用 `git checkout --`，改坏期间不跑 `make gen*`。e2e 的变异各自 `heavy.sh make build`，最后不带变异再建一次，对着的格全过（第一轮 27 格；修复轮 2 是 27 格加 6 格；修复轮 3 是 27 格）。实施者在 `f5d84965`、`e404d0d7`、`f2130709` 上各做一轮（vitest 的一轮另在 `7cacd2a2` 上跑过，结果相同）；C 在 `f5d84965`（19:28–19:31）、`aad1ab44`（21:41–21:44）与 `524734f1`（23:06–23:09）上各重放一轮、另加自己的。日志在 scratchpad 的 `m13-close/impl/mut/` 与 `m13-close-c/fc1/`、`fc2/`、`fc3/`，不在仓库里。

"没改的树"是修之前的代码（`eb48f4a1`，或那一条之前的提交与它的二进制）上跑抓手的结果，即先红；"修复之后"是在修好的头上做这一处变异、再跑抓手的结果。

| 变异（一处） | 抓手 | 没改的树 | 修复之后 |
|---|---|---|---|
| **B-I1** `source-editor.tsx`：`realignments = 4` 改成 `0`（不再对准） | PG17 "edited from the page's top"，三种引擎 | 提交 2 的二进制上三种引擎都红在 `pg17:576`（"where ## Tasks settles"：Chromium −18.94、Firefox −18.57、WebKit −18.94） | 死：三种引擎都红在 `pg17:576`（`f5d84965`、`e404d0d7` 上各一次；C 在 `f5d84965` 上重放同） |
| **967**（C 记作 967a）`swallowHeldKeys` 删掉 `event.preventDefault()`，只挡传播 | `dialog.test.tsx`；PG16 第 1 格 | vitest 两格红（dialog、alertdialog：对话框又开了）；修之前的二进制上 PG16 第 1 格三种引擎都红在 `pg16:71` | vitest 死：两格红。PG16 三种引擎都过（C）：树的"⋯"开菜单靠 `keydown`，不靠按钮默认的 click，那一步只守"挡传播"那一半；"挡默认"那一半（靠 click 打开的，例如版本页的"还原"）只由 vitest 守，读屏清单 H 第 22 项的人工核对补上 |
| **967b**（C 加的）`DialogContent` 的 `onCloseAutoFocus` 里删掉 `swallowHeldKeys();` | `dialog.test.tsx`；PG16 第 1 格 | 同上 | `f5d84965` 上：vitest 死（dialog 一格红，alertdialog 有自己的一处、照过）；PG16 在 Firefox、WebKit 红在 `pg16:79`，Chromium 过（C-F1-M1）。`e1b24d5e` 把那一步改成同步的记录之后，`e404d0d7` 与 `aad1ab44` 上三种引擎都红在 `pg16:137`（`menus [1,0]`） |
| **968** `breadcrumbs.tsx`：分隔符放回链接之前做兄弟（原来的放法） | PG17 "the trail keeps each separator"；`page-layout.test.tsx` | 提交 2 的二进制上 Firefox 红在 `pg17:654`（320 宽，两个分隔符与名字不在一行），Chromium、WebKit 过；结构断言红 | 死：Firefox 红在页的 320 宽，Chromium、WebKit 过（同 B 量的，只有 Firefox）；vitest 红；C 重放同 |
| **968 的补齐** `Named` 换回"名字加分隔符"，不粘最后一个字素 | 同上 | 提交 4 的二进制上三种引擎都红在 `pg17:674`（面包屑设到 156、157、230–232 px 等宽时） | 死：三种引擎都红；vitest 红；C 重放同 |
| **969** `page-icon.tsx`：`import { ScrollText as File }` | quick-switch、notebook-home、search-page、trash-page、trash-preview-head、page-tree 的 vitest | 7 格红；其中来源页一格第一次红是测试自己写错，改正之后它与快速切换一格都是守卫 | 死：6 格红 |
| **A-M1** `page-tree-item.tsx`：`<span title={name}>` 改成 `<span>` | `page-tree.test.tsx` | 红 | 死 |
| **A-M2** `page-tree-item.tsx`：去掉 `forced-colors:data-active:outline …` | `page-tree.test.tsx` 强制色一格 | 红 | 死 |
| **A-M3** `shell-sidebar.tsx`：去掉 `tabIndex={-1}` | PG16 第 2 格 | Firefox 红在 `pg16:110`，Chromium 过（本来就不停）；右栏的一步 PG17 [firefox] 红在 `pg17:169` | 死：Firefox 红在 `pg16:136`，Chromium、WebKit 过；C 重放同。右栏的 `tabIndex` 没有单做变异，由 `page-panel.test.tsx` 的属性断言与 PG17 第 1 格的先红守 |
| **966** `page-tree.tsx` 的 `shown`：去掉 `any-pointer-coarse:opacity-100` | `page-tree.test.tsx` | 红 | 死 |
| **970** `general-page.tsx` 的下拉：去掉 `max-md:w-full` | `general-page.test.tsx` | 红 | 死 |
| **965** `name-row.tsx`：Esc 的分支里删掉 `setDraft(undefined)` | `name-row.test.tsx` | 两格红 | 死：两格红 |
| **964** `en.ts`："Workspace settings" 改回 "Settings" | `workspace-layout.test.tsx`、`settings-directory.test.tsx` | vitest 红；UI2 三种引擎都红在 `ui2:76` | 死：5 格红；C 重放同 |
| **B-N1** `rename-form.tsx`：删掉 `DialogClose` 的"取消" | `page-tree-writes.test.tsx`、`attachments-section.test.tsx` | 两格红 | 死：两格红 |
| **B-N2** `text-diff.tsx` 的 `buttonClass` 换回 `h-9 … rounded-lg … px-4 text-sm` | `conflict.test.tsx`、`version-restore.test.tsx` | 两格红 | 死：两格红 |
| **B 第 9 节第 7 项** `narrow.ts` 的 `animationsEnded`：一开头就 return | 用 `expectButtonsInside` 的四格（`hi5:144`、`tr2:195`、`lw3:287`、`up3:234`） | `expectFits` 那一半由探针证：一个从 0.97 弹入 1 秒、进来后 378 px 宽的框（窗口 375），旧的 `expectFits` 量到 366.7 px，三种引擎都放过；改后三种引擎都抛 | 死：UP3 [chromium] 红在 `up3:281`，LW3 [chromium]、[webkit] 红在 `lw3:366`；C 重放红的是 UP3 [chromium]、LW3 [chromium] 与 [firefox]（谁抓到看 150 ms 的弹入落在哪）。`expectFits` 那一半没有提交的格守 |
| **B-F1-M1 菜单** `dropdown-menu.tsx` 的 `pressedOnce`：删掉 repeat 的 `preventDefault()` | `dropdown-menu.test.tsx`；PG16 第 1 格 | vitest 两格红（按住 4 次之后菜单关着：开、按下、开、按下）；`f5d84965` 的二进制上 PG16 三种引擎都红在 `pg16:151`（Chromium `menus [1,1]`，Firefox、WebKit `[2,2]`，`rows 1`，新建页的 POST） | 死：vitest 两格（Enter、空格）红；PG16 三种引擎都红在 `pg16:151`；C 重放同 |
| **B-F1-M1 大纲** `edit-outline.tsx`：删掉 `detail === 0` 时的 `swallowHeldKeys()` | `edit-outline.test.tsx` | 代替编辑器的 textarea 得到 5 个换行 | 死：`'\n\n\n\n\n'`。C 另把 `=== 0` 改成 `=== 1`（只挡指针的），也死 |
| **C-F1-N1** PG16 第 2 格的标题改掉 | `node tools/e2e-projects/check.mjs` | 守卫：加之前也过 | 死：rc 1，"firefox … lists no cell" |
| **B-F1-N2** `comparison.tsx` 的"重试"：去掉 `size="sm" shape="pill"` | `conflict.test.tsx`、`version-page.test.tsx` | 两格红 | 死：两格红 |
| **C-F1-N5** `page-tree-item.tsx` 的"⋯"：去掉 `max-md:opacity-100` | `page-tree.test.tsx` 966 一格 | 红 | 死 |
| **A-F1-N1** `workspace-switcher.tsx`：去掉 `collisionPadding={8}` | `workspace-layout.test.tsx` | 红：可用宽 320px | 死：`'320px'` 不是 `'304px'` |
| **A-F1-N2 树行** `not-has-[a:focus-visible]` 的两类换回 `forced-colors:data-active:outline …` | `page-tree.test.tsx` 强制色一格 | 红 | 死 |
| **A-F1-N2 笔记本行** `not-has-[h2>a:focus-visible]` 的两类换回 `forced-colors:data-home:outline …` | 同上 | 红 | 死 |
| **B-F1-N1** `source-editor.tsx`：删掉 `domEventObservers` 里的 `scroll: () => this.scrolled()` | PG17 "a scroll of the window after the outline's jump"，三种引擎 × 3 | 提交 7 的二进制上 Firefox 3/3 红在 `pg17:652`（窗口回到 5681，滚之前的位置），Chromium、WebKit 过 | 死：Firefox 3/3 红在 `pg17:652`，Chromium、WebKit 过；C 重放同（75 帧回到远处） |
| **C-F2-M1 / A-F2-N1** `pressedOnce` 不带 repeat 时不调 `swallowHeldKeys()` | `dropdown-menu.test.tsx`；PG16 第 1 格 | vitest 三格红（单选项：菜单又开了；对话框：表单提交了 3 次）；`aad1ab44` 的二进制上 PG16 的 Chromium、Firefox 红在 `pg16:176`（Rename：`dialogs [1,1]`，表单提交后对话框关了），WebKit 红在 `pg16:175`（Move to…：`dialogs [1,1]`，发出 `POST …/move`） | 死：vitest 三格红；PG16 同上；C 在 `524734f1` 上重放同 |
| **A-F2-N2** 菜单里的名字换回 `truncate` | `workspace-layout.test.tsx` | 红 | 死；C 重放同 |
| **r3space**（C 加的）`pressedOnce` 第一次按下时只给 Enter 调 `swallowHeldKeys()`，空格不调 | `dropdown-menu.test.tsx` | — | 死：单选项上按住空格的一格红（菜单又开了） |
| **B-F2-N1** `scrolled()` 去掉 `\|\| landed \|\| windowAtFoot()`，只留 [0, 48]（等于 `41fbce8f` 之前） | PG17 滚动那一格新加的一步：跳到 Section 40，再 `scrollBy(0, −1500)`，三种引擎 × 3 | 旧的 `source-editor.tsx` 另建二进制：9 次全红在 `pg17:688`（窗口回到页底 14741，Firefox 14804） | 死：`524734f1` 上这条变异 9 次全红，`91a62e60` 的原文件（即 `41fbce8f^` 的）9 次全红，都红在 `pg17:705`：落定时那一行的 `top` 是 303.23（Firefox 303.03），1.2 秒里每一帧都回到那里（差 0.14 px，Firefox 0.70 px）；C 在 `524734f1` 上两组重放同，各 9 次全红在 `pg17:705` |
| **r3-foot** 去掉"窗口到底"那一条 | PG17 滚动那一格与 I1 那一格，三种引擎 × 3 | — | **活**：18 次全过 |
| **r3-landed** 去掉"编辑器自己的滚动已经落到"那一条 | 同上 | — | **活**：18 次全过 |
| **n1placed**（C 加的）到过位之前也判断（`else if (aligned.placed)` 改成 `else`） | 同上 | — | **活**：C 在 `aad1ab44` 上、实施者在 `f2130709` 上各 18 次全过 |
| **n1own**（C 加的）`realign` 不再每次更新记下行的位置 | 同上 | — | **活**：同上 |
| **C-F3-N1 / r5-top** "窗口到底"那一条去掉新加的"那一行在窗口里"，回到 `\|\| windowAtFoot()` | PG17 滚动那一格新加的一步：跳到 Section 15，再 `scrollBy(0, 100000)`，三种引擎 × 3 | `524734f1` 的二进制：9 次全红在 `pg17:711`，1.2 秒里每一帧那一行都回到落定处（差 0.0 px，Chromium 与 WebKit 11.30，Firefox 11.20） | 死：`d734488f` 上 9 次全红在 `pg17:711`；I1 那一格 9 次全过 |
| **r5-nofoot** 去掉整条"窗口到底" | PG17 滚动那一格与 I1 那一格，三种引擎 × 3 | — | **活**：`d734488f` 上 18 次全过 |

**活下来的四条只由推理守着**（实施者第四部分；B 第 3 轮读了 `scrolled()` 的三条，同意它们在 Section 40 那一步同时成立）：
- r3-foot 与 r3-landed：跳到 Section 40 之后第一次滚动时，行就在编辑器上次更新记下的位置，窗口也在页底，两条都成立，拿掉任一条都过。两条一起拿掉就是修之前的代码，红 9/9。这一步守的是"至少有一条成立"。只有一条成立的情形只在一两帧之间：编辑器量完行高、再对准之前那一帧（行在 [0, 48] 之外，窗口不在页底）；或再对准在页底又滚了一次、滚完没有更新。用滚动的时刻造不出确定的格。
- n1placed：现在"到过位"在跳转后编辑器第一次自己的滚动时就成立；在那之前的滚动只可能来自外面、落在编辑器第一次量之前，随后被跳转自己的滚动盖掉，有没有这一条结果都一样。
- n1own：跳到能到顶的行靠第一条（[0, 48]），跳到页尾的行靠窗口到底；要靠更新记下的位置才分得出的，与 r3-landed 的情形相同。
- r5-nofoot（`d734488f` 上的 r3-foot）：理由同 r3-foot。跳到 Section 40 时"自己的滚动已经落到"也成立；新的一步里窗口到底时那一行不在窗口里，这一条本来就不成立。只有它一条成立的情形只在量行高与再对准之间的一两帧里。
- 真的拖滚动条、页内查找、读屏没有在真机上核对，都只用 `scrollBy` 代替。写进 v0.2 的移交第 8 项。

**重复**（不带变异，跑时都有别的格并行）：
- 实施者：PG16 第 1 格三种引擎各 6 次，18 次全过（`e1b24d5e` 之后）；PG16 第 1 格与 PG17 滚动那一格三种引擎各 6 次，36 次全过（`f2130709`）；PG17 滚动那一格三种引擎各 10 次，30 次全过（`524734f1`）；PG17 I1 与滚动两格三种引擎各 10 次，60 次全过（`d734488f`）。
- C 第 1 轮（`f5d84965`）：PG17 两个新格三种引擎各 6 次，36 次全过；PG16 两格各 6 次，36 次全过。
- C 第 2 轮（`aad1ab44`）：PG16 整个文件各 6 次，36 次全过；PG17 三个新格各 6 次，54 次全过；AS6 `as6:153` 40 次全过（只在 Chromium）。
- C 第 3 轮（`524734f1`）：PG16 整个文件三种引擎各 3 次，18 次全过；PG17 I1 与滚动两格各 6 次，36 次全过；C8 `c8:159` 40 次全过。修之前的 C8 那一格（`f2130709` 的写法）在这台机器上 40 次也全过，先红只有实施者的 220 次红 12 次。

## 移交

- **新建** [v0.2/handoffs/M13-ui-refresh.md](../../../v0.2/handoffs/M13-ui-refresh.md)（`status: open`、`from: M13`、`to: v0.2`、`created: 2026-11-26`；决定 962）：十一项，搜索摘要的 Markdown（00-M13 第 2 节，决定 721）、像素级回归（决定 725）、骨架（决定 914）、l5 的底（决定 891）、mermaid 的主题与作者的浅色图（决定 871，P4/S2）、代码块的头与页内查找、焦点（决定 869、905）、锚点的首次焦点与"编辑于现在"（决定 900，P4/S1）、编辑器的远跳与 Firefox 的警告（P4/S3；另写 375 宽往上跳也报，对准的 4 次与 1 秒、结束对准的几种先后只由推理守着、真的拖滚动条与页内查找没有核对）、决定 958 的七行（C-N5）、可能用手指点的设备（决定 953、966）、Linux X11 下成对的按键重复（修复核对第 1 轮，A 的未核实）。05-P5 第 9 节收尾的另四件（齿轮、"/"、按住的键、`expectFits`）收尾修了，不在里面。"Chromium 强制色下齿轮聚焦时笔记本那一行的框不画"不在里面：A 核过，是 Chromium 的元素截图不带这一圈，整页截图带（修复核对第 2 轮，C-F2-M2 不改）。
- **[M12-polish.md](../../../v0.2/handoffs/M12-polish.md)**（仍 `open`）：条目里 M13 做掉的逐项注明，第 1 项重数（C-M5）；P2/S3 一行的 `aria-keyshortcuts`（C-N6）、第 9 项的位置（C-N9）；"处理进展"加"M13 收尾"一行，指到新的一份。
- **[M12-after-release.md](../../../v0.1.1/handoffs/M12-after-release.md)**（仍 `open`，to v0.1.1）：第 1 项"读屏清单 A–H""十五项"（C-M3）；"处理进展"加"M13 收尾"一行。
- **总体设计 12.6 的 M13 与 00-M12 第 12 节"收尾与发布"一行**，都在 `main` 上记录合并的提交里（C-F1-M2，00 号文档第 7 节"收尾"）：12.6 改为"实现完成，待人工验收"，第 15 节那一行随之写上；00-M12 第 12 节那一行改成"收尾已合并；M13 收尾已合并（`<待填>`），发布未开始（推 tag 之前问负责人）"。00-M12 的头与第 13 节不改：M13 设计定稿时也只改了第 12 节那一行（`56caf0d7`）。
- 收尾之后 v0.1 里 `open` 的移交只剩 `M4-pages/handoffs/M0-P1-editor.md` 与 `M9-mcp/handoffs/M0-P1-mcp-sessions.md`，它们等发布之后的清单；M13 自己的 `handoffs/` 是空的。

## 人工清单

- **读屏清单**加 H 一节（新外框、设置界面、阅读与编辑、其余页面，二十二项：02-P2、03-P3、04-P4、05-P5 第 9 节"收尾"的各条，加决定 964 的"工作区设置"、965 的 Esc、967 的按住的键、A-M1 的 `title`、A-M2 的强制色外框）；C 一节第 3 项改成账户行。
- **输入法清单**：`:6` 的执行人、准备第 2 条（第 26 行）的 `$PAT` 改成账户菜单与设置目录、第 21 步的 (a) 与新的 (d) `Ctrl+K`。
- **MCP 客户端清单**第 46 行同输入法清单的第 26 行。
- **manual/README.md**：`:3`、`:11`、`:13`、`:14` 改到 M13，"其余各项"加第 15 项（Safari 里编辑时显着的"保存"，04-P4 第 9 节"负责人"）。
- 这几处都由文档脚本 `closeout-docs.py` 写；00 号文档的四处由 `closeout-m13-sections.py` 接着写（见"文档"）。两个脚本的输出与这份记录在 `m13-closeout` 上同一个文档提交里（`docs: … (M13 closeout)`）。

## 文档

- **总体设计**：头的表"覆盖 M0–M13"（C-M4）；9.2 的左栏一段与 13.2 第 17 条照决定 964 写"工作区设置"，账户菜单的"设置"照界面写（C-F1-N2），左栏中段与右栏不是 Tab 的一站（A-M3）；9.2 的右栏（编辑时大纲的再对准，B-I1、F1-N1）；13.2 第 12 条（965 的 Esc，967 的对话框、菜单与大纲）、第 31 条（958 的七行，971 的 `title` 与切换器的菜单，970，968 的分隔符，A-M2 的强制色外框，966 的同一套条件，`expectFits` 先等弹入）；13.4 第 3 条（PG17 的三格，PG16 加的几步，REQUIRED）；第 15 节一行。12.6 在合并的提交里改（见"移交"）。`docs/README.md` 不变（已是 M0–M13）。
- **00 号文档**：第 7 节收尾第 1 项"第 26 行"（C-N1）、第 4 项的命令表（C-N8 与第 6 节，决定 961 的那一句）、"写给后面的 M"指到新移交；4.7 的比值（C-N10）；第 9 节 CI 的数（C-M6）。头的状态：收尾的审查与修复完成、合并回 `m13-ui-refresh` 于 `<待填>`，下一步合并进 `main`，`main` 上的完整门禁与 CI 绿之后改为"实现完成，待人工验收"（与 12.6 同在记录合并的提交里）；第 11 节加"M13 收尾的决定"一节（决定 961–971，修复轮里归在决定之下的补充写进各条）；第 12 节"收尾"一行（合并号 `<待填>`）；第 13 节一行（含 `eb48f4a1` 的 CI，C-M6）。这四处由主持者的脚本 `closeout-m13-sections.py` 在 `closeout-docs.py` 之后写。
- **05-P5**：7.3、7.5 的 CI 的数（C-M6）；第 1 节的已知偶发注明 AS6 由 `aad1ab44` 修了，7.3 的偶发清单里同样注明；第 9 节齿轮的"静止时 0.4"（C-N7），收尾各项注明做了什么、去了哪里。**02-P2、03-P3、04-P4** 第 9 节的"收尾"各注明对应读屏清单 H 的哪几项，04-P4 的 v0.2 一项指到新移交。
- **01-P1**：10.1、10.2 的合并号；P1-S1、P1-S2 的记录同样（C-N2）。**M13-design-review** 的结论（C-N3）；**P3-design-review** 末尾的空行（C-N4）。
- **README**："登录、会话、引导与设置"一组加"外框"一条；笔记本的设置、页面与左栏、编辑的"保存"、右栏、历史与回收站的入口、搜索与 `Ctrl+K`、个人设置与外观、版本的位置照新界面改；升级的入口与 PAT 的入口（`:253`、`:271`）。
- **`CHANGELOG.md` 0.1.0**："有什么"加"网页"一条；"网页的打磨"删去"编辑时没有大纲"；已知限制加"对比度让位于外观"（决定 963）。比值照 `W/styles.css` 的色值按 WCAG 的公式算：说明与占位的字（`--faint`）浅色对白 3.07、对左栏 2.81，深色对底 3.80、对卡片 3.51、对左栏 4.03；焦点圈（`--accent` `#c2871f`）浅色对白 3.10、对左栏 2.84（深色 `#d9a84e` 7.92、8.41）；删除类的红字浅色 `#d64545` 对白 4.38（深色 `#ff7b7b` 对底 6.87、对卡片 6.34）；正文浅色 17.58、深色 14.07。一节里没有相对链接。

## CI 的数

E2E 一步的趋势：M12 收尾 6:21 → `f62d0390` 8:22 → `e8809170` 8:05 → `af33b81e` 8:05 → `eb48f4a1` 12:01（时限 18 分钟，00-M12 第 3 节第 11 条）。

| 头 | 运行 | 结果 | E2E 一步 |
|---|---|---|---|
| `f62d0390`（P4 完成） | 45551442052 | e2e 红三格（PG17 两格、C8），`9871c7d4` 修（04-P4 10.4） | 8:22 |
| `e8809170`（P5 的设计，CI 的两处修好之后） | 45579305353；release 45579305372 | 全绿 | 8:05 |
| `af33b81e`（P5/S1 合并之后） | 45631867718；release 45631867695 | 全绿；646 格，"645 passed (7.9m)"、1 跳 | 8:05 |
| `eb48f4a1`（P5 完成） | 45773568132；release 45773568094（amd64 1:50、arm64 1:45） | 十九项全绿（web 5:04，e2e 作业 13:19，image 1:31）；647 格，"646 passed (11.7m)"、1 跳 | 12:01 |
| 收尾合并之后的 `m13-ui-refresh`（`<待填>`） | 合并回 `m13-ui-refresh` 之后写 | | |
| 合并进 `main` 之后第一次 `main`（`<待填>`） | 合并进 `main` 之后写 | | |

- `eb48f4a1` 比 `af33b81e` 只多一格：各故事的用时合计多约 433 test-seconds，S2、S3 加的几步约八十秒，其余落在 S2、S3 没碰的故事上（lw8、lw4、a15、l6、pg17、lw6、lw13 各慢 20–100%），三种引擎都慢（各引擎的 test-minutes：Chromium 8.3 → 11.6、WebKit 3.6 → 5.2、Firefox 2.8 → 5.1）；本机同期一直 2.7–2.9 min。推断是那一次的 runner 慢，没有核实（不是同一台机器，不能复跑同一个头）。
- S2、S3 只靠 CI 的几步（TB1、UP1 的变更集与版本页两步、hi3 静止的一步、hi2 320 宽的三句、tr2:290 的点击记录、lw3/up3/hi5 的 `expectButtonsInside`）在 run 45773568132 上第一次跑，都过了。
- 看法（C-M6）：收尾合并之后的 `m13-ui-refresh` 与合并进 `main` 之后第一次 `main` 的 E2E 一步都回到 8–9 分钟，就记作一次慢的 runner、结案；两次都在 11 分钟以上，推 tag 之前先查（作业日志开头的 `Cloud:`、`Region:` 与各引擎的分钟，06-environment 第 4 节）。`m13-closeout` 自己不推送，所以第一次是收尾合并之后的 `m13-ui-refresh`。收尾多了 9 格（647 → 656）；本机合并之前的整条命令里 Playwright 计 2.8–3.1 分钟。两次的数与结论：合并进 `main` 之后写。

## 核实修复

报告都在 scratchpad：`m13-close-{a,b,c}/fix-check-<n>.md`；实施者的在 `m13-close/impl/report.txt`。

**第 1 轮**（`f5d84965`）：A 的七条、B 的 I1、967、968、969、N1、N2、第 9 节第 7 项都修好。
- A：普查原样重跑，与第一轮逐格相同；故事 42 格、本区 vitest 64 个文件 981 格全过。967 在设置里的六种对话框里照旧正常，用指针关掉之后按住的键没有被误吞。A 更正第一轮的一句：Chromium 关上对话框之后焦点在 `body` 只有一帧（11–13 ms），不是"一秒之内"。
- B：I1 的落点三种引擎共 135 次，除两个页底的节外都在窗口顶下 9.4–11.4 px；按画出来的帧量 90 次，0 帧离开 [0, 48]，实施者记的"第一帧还在旧处"没有复现。967 的 13 条路三种引擎都是又打开 0 次、新建 0 页。
- C：合并之前的整条命令 273 秒，三格基线之外另有两格 Firefox（`lw3:129`、`lw12:126`）在 19:17:44 同一刻的数据库停顿上红（`commit transaction: timeout`），单独重跑是绿的；重放八条变异都红，但 PG16 按住 Enter 的那一步在 Chromium 里放过了 967b（C-F1-M1）。

| 发现 | 处置与提交 |
|---|---|
| B-F1-M1 按住的 Enter 在菜单与编辑时的大纲上照样按到焦点去的地方（树的"⋯"→"New subpage"新建了页；附件的"Open"反复开标签页；大纲在标题行首插进换行） | 归在决定 967 之下：`DropdownMenuItem`（与 `DropdownMenuRadioItem`）对重复的 Enter 与空格 `preventDefault()`；大纲的项由键盘按下时先调 `swallowHeldKeys()`；vitest 各一格，变异各一行（`e1b24d5e`） |
| C-F1-M1 PG16 按住 Enter 的那一步在 Chromium 里放过了它要守的毛病（断言不等事情落定） | 按下之前在页里装同步的记录（`MutationObserver` 记菜单、对话框与树里新出现的行，`page.on("request")` 记非 GET 的请求），放开之后断言；另加在菜单按钮上按住 Enter 的一步；967b 与菜单那一条的变异三种引擎都红（`e1b24d5e`） |
| C-F1-M2 文档脚本还在改 12.6 | 去掉；12.6 与 00-M12 第 12 节在 `main` 上记录合并的提交里改（文档脚本） |
| B-F1-N1 Firefox 里对准期间没有输入事件的滚动被拉回 | 对准过一次之后，来了不是它自己引起的 `scroll` 就结束，用 CodeMirror 自己的 scroll 观察者；写成 PG17 一格确定的格（四十节长文点 Section 15，到位之后 `scrollBy(0, 500)`），REQUIRED 加它的 Firefox 与 WebKit（`e404d0d7`） |
| B-F1-N2 `ChunkNotLoaded` 的"重试""重新加载"还是方角 | `size="sm" shape="pill"`，类名断言（`0d6b2b04`） |
| C-F1-N1 REQUIRED 没有 PG16 第 2 格的 Firefox | 加一行（`e1b24d5e`） |
| C-F1-N2 13.2 第 17 条还有两处"个人设置" | 照界面写"设置"（文档脚本） |
| C-F1-N3 新移交第 9 项是 `eb48f4a1` 的行号 | 照 `f5d84965` 改（文档脚本） |
| C-F1-N4 读屏清单 H.6、H.22 的说法 | 照 C 改（文档脚本） |
| C-F1-N5 树行的"⋯"没有 `max-md:opacity-100` | 加上，三者真的同一套条件（决定 966）；类名断言（`0d6b2b04`） |
| A-F1-N1 切换器的菜单在 320 宽贴着窗口两边 | `collisionPadding={8}`（`0d6b2b04`） |
| A-F1-N2 强制色下当前行的外框与焦点环叠在一起 | 行里的链接有键盘焦点时外框让位（同 `placeRow` 的 `not-focus-visible`），三处一样；类名断言（`0d6b2b04`） |
| C-F1-Q1 决定 961 没说多出一格环境的红怎么办 | 加进决定 961：先看服务端日志，停顿就单独重跑、照实写，不是就先停、查清 |
| A 的未核实：Linux X11 下成对的 keyup/keydown 重复 | 没有环境核实，进 v0.2 的移交第 11 项 |

修复轮 2（`e1b24d5e`、`0d6b2b04`、`e404d0d7`）：八条都做了，十一条变异都死；`e404d0d7` 上合并之前的整条命令 236 秒，只败 961 的两格（SE2、LW8；PG15 这次过）。逐提交的门禁里多出的两次红见"门禁与分支 CI"：`e404d0d7` 的 `a10:145`、`pg17:128` 是同一刻的数据库停顿，照 961 单独重跑都过；`0d6b2b04` 的 AS6 不是停顿。

**AS6 的偶发**（`aad1ab44`，只改 `e2e/stories/asset/as6-reading-links.spec.ts`，+4 行）：AS6 [chromium] `as6:153` 红在 `as6:167`（"Response has been disposed"）。这一格用 `page.route` 改写 view 的回答，最后只等到"再读一次 view"的请求发出就结束；trace 里第二个 `route.fetch` 在 7931 ms 开始，After Hooks 在 7943 ms，页面关了，回答被释放。修法：格末 `await page.unrouteAll({ behavior: "wait" })`，正在跑的 handler 在页面还开着时跑完再撤。不取 `"ignoreErrors"`：那样 handler 仍会在页面关掉时还在跑，只是丢掉它的错。先红：`e404d0d7` 的二进制上单独跑，四组共 160 次红 8 次（0/20、2/60、0/20、6/60；后两组原本记成修后，实际是改动还没落到文件上时跑的），都是 `as6:167`。修后 20 次、60 次全过，共 80 次；C 第 2 轮又跑 40 次全过。`as6:153`、`as6:167` 从已知偶发的清单里去掉（05-P5 第 1 节注明）。

**第 2 轮**（`f5d84965..aad1ab44`，四个提交）：
- A：两条 Nit 修好，三种引擎相同；`pressedOnce` 在切换器、账户菜单、外观、成员的角色、树的"⋯"里按住都只按一下，也挡住了"按住 Enter 把成员改成 Admin"；普查重跑同第一轮，故事 42 格、本区 vitest 64 个文件 984 格全过。实施者另记的"Chromium 强制色下齿轮聚焦时笔记本行的框不画"，A 对照了截法：整页截图里画着，只有这一行的元素截图不带这一圈，计算样式一直是 `solid 1px`。不是产品的行为，不进移交。新 2 Nit。
- B：F1-M1、F1-N1、F1-N2 修好，I1 的落点同第 1 轮；实施者说的"提前结束"只在 WebKit、要在跳转后头几帧里同时有编辑器之上的布局变化与一次没有输入事件的滚动才造得出（标题停在 129 px），不用改。新 1 Nit。
- C：合并之前的整条命令 260 秒，只败 961 的三格，AS6 过；重放的 967b、r2-menu、F1-N1 三条变异都死，PG16 那一步在 Chromium 里也红了（F1-M1 修好）；C 加的三条里大纲一条死，n1placed、n1own 活。新 2 Minor、2 Nit；文档的最后文字套在 `m13-close-c/fc2/closeout-docs-try.py` 上，`--check` 过。

| 发现 | 处置与提交 |
|---|---|
| C-F2-M1 / A-F2-N1 Enter 按住在会打开对话框的菜单项上，重复落进对话框里（Rename 一出来就被提交，Delete 一出来就被取消，WebKit 里"Move to…"发出 `POST …/move`）；在会关掉菜单的项上按住，菜单又开了 | 归在决定 967 之下：`pressedOnce` 在第一次（不带 repeat）的 Enter 或空格按下一项时也调 `swallowHeldKeys()`；vitest 三格，PG16 第 1 格加一步（菜单由指针打开，在"Move to…"与"Rename"上按住），变异红（`91a62e60`） |
| B-F2-N1 / C-F2-N1 跳到页尾一屏之内的标题之后，没有输入事件的滚动三种引擎都被拉回页底；F1-N1 的两个条件没有格守 | "到过位"放宽为三种情形之一：行进了 [0, 48]；编辑器自己的滚动已经落到（`\|top − aligned.top\| ≤ 1`）；窗口已经滚到底。PG17 那一格加一步：跳到最后一节，再往上 `scrollBy(0, −1500)`（`41fbce8f`）。C 的两条与实施者的两条活下来，照实写成"只由推理守着"（见"变异"） |
| A-F2-N2 320 宽时只在末尾不同的工作区名在菜单里又分不出来 | 菜单里的名字最多两行（`line-clamp-2`），按钮上照旧一行截短；320、375、1440 宽截图核对（`f2130709`） |
| C-F2-M2 新移交缺"Chromium 强制色下齿轮聚焦时笔记本那一行的框不画" | 不加：A 核过，是 Chromium 的元素截图不带这一圈，整页截图带 |
| C-F2-N2 文档第二稿里几处与最后的代码不合 | C 的 `closeout-docs-try.py` 换成现行的脚本（旧的留作 `closeout-docs.v2.py`），修复轮 3 之后由 C 照最后的代码再补 |

修复轮 3（`91a62e60`、`41fbce8f`、`f2130709`）：三条都做了；`f2130709` 上合并之前的整条命令 260 秒，只败 961 的三格；vitest 234 / 5623，`check.mjs` 437/108/108/3。PG17 那一步的断言写成"不超过这次滚动的一半"，理由见"实施中定的"。`91a62e60` 的门禁多红一格 C8，不是停顿。

**C8 的偶发**（`4470c25a`，只改 `e2e/stories/collab/c8-stream-life.spec.ts`，+11 行）：C8 [chromium] `c8:159`（"B, reading a page of Eng, is removed from it"）的安静检查多了一条控制台错误（404）。这一格在标题出现之后立刻把 B 移出 Eng，这时 B 的事件流刚连上、正在把 Eng 的树和页重读一遍，还没读到的那一样被移除答成 404；先红的日志里不只是 backlinks，还有 view、versions、assets、properties、edit-lock。同一文件的下一格专门挡了这种时序，这一格没有。修法照下一格：打开页之前先 `holdStream`，标题出现后放行，等页的 view、edit-lock、backlinks、properties、versions 与 Eng 的 nodes、assets 七样各答过两次（页自己读一次，事件流连上后又重读一次），再移除 B；比下一格多数 versions、assets、nodes，是探针量到事件流连上后它们也重读。先红：`f2130709` 的二进制上只跑 `c8:159`，三组共 220 次红 12 次（5/100、2/20、5/100；后两组原本记成修后，实际是改动还没写进文件时跑的）。修后 20 次、100 次全过，共 120 次。`c8:159` 不在仓库文档的已知偶发清单上（只在实施者的报告与这份记录里），文档里不另加注明。

**第 3 轮**：
- A（`aad1ab44..f2130709`；`41fbce8f` 是 B 的区域，没有核）：F2-N1、F2-N2 修好，三种引擎、Enter 与空格都一样；放开之后再按一次，第一下照常起作用（主题、成员角色、切换器、改名对话框里打空格）；故事 42 格、本区 vitest 64 个文件 987 格全过；没有新的问题。
- B（同上）：F2-N1 与 C 的 F2-M1 修好；I1 的落点与第 2 轮逐次比差都不到 1 px，画出来的帧 90 次 0 帧偏离；"挪 166 px 让内容不动"对，但数因页而异（Guide 往上 1500 是 +1117）。新 1 Nit、1 Q。B 没有另造格守那四条变异，读了 `scrolled()` 的三条，同意它们在 Section 40 那一步同时成立。
- C（`524734f1` 上：测试、门禁、变异、文档脚本的最后文字与本记录）：合并之前的整条命令 257 秒，只败 961 的两格（SE2、LW8；PG15 这次过）。`91a62e60`、`41fbce8f`、`f2130709` 各有变异红：第一次按下不吞，vitest 三格红，PG16 红在 `pg16:175`、`:176`；只留 [0, 48] 与 `41fbce8f^` 的 `source-editor.tsx`，PG17 那一格三种引擎各 9 次全红在 `pg17:705`；`truncate`，vitest 红。C 加的"第一次按下只吞 Enter"也死。只由推理守着的四条照实写进新移交第 8 项，可以接受。C8 修之前的写法在这台机器上 40 次全过，没能复现；在浏览器这边把第二次 backlinks 的读压 1.5 秒也造不出确定的红（3/3 过）。改法只等一个确定的状态，接受。另造一格，找到 `41fbce8f` 的一处回归（C-F3-N1）。文档脚本照最后的代码补完（93 处，19 个文件，新移交十一项）；两个脚本在干净的树上依次跑过，`check_links.py` 没有新的坏链，`git diff --check` 没有输出。新 1 Nit。

| 发现 | 处置与提交 |
|---|---|
| B-F3-N1 PG17 那一步的"不超过这次滚动的一半"，在估计差大的页上会误红（Guide 往上 1500 时编辑器一挪约 1120 px，超过 750） | 接受，只改测试：改成比跳转落定的位置（`524734f1`，见下面修复轮 4） |
| C-F3-N1 对准过之后，不是它引起的滚动若正好到了页底，"窗口到了底"那一条把它当成对准引起的，跳去的那一行又被拉回（`41fbce8f` 带进来的：三种浏览器 9 次全拉回，`41fbce8f^` 9 次都不拉回） | 接受，这一轮修：窗口到底时还要那一行在窗口里，PG17 滚动那一格加一步一直滚到页底（`d734488f`，见下面修复轮 5） |
| B-F3-Q1 WebKit：跳转后头几帧里编辑器之上插进内容，现在不加滚动也停在 131 px，不再拉回 11 px | 不改：应用里没有会在那几帧变动的东西（头一行定高，锁的说明在进编辑时就有了）；Chromium、Firefox 有滚动锚定，同一格停在 11 px |

修复轮 4（`524734f1`，只改 `e2e/stories/page/pg17-page-head.spec.ts`，+32 / −15）：
- 量法：不比窗口的 `scrollY`，比跳转那一行在屏幕上的 `top`。编辑器量行高会让页变长，拉回时 `scrollY` 不回到落定时的值（多出 261–955 px）；修好的代码在 Guide 往上 600 时最近离落定时的 `scrollY` 只有 91 px，两边交叉，没有 `scrollY` 的门槛分得开。那一行的 `top` 拉回时回到落定处（差 0.12–0.87 px），修后离开至少 0.87 倍的滚动距离。
- 断言：没有一帧离落定时的 `placedTop` 在 `|by| / 4` 以内（往下 500 是 125 px，往上 1500 是 375 px）；行没画出来的那一帧记 NaN、算作离开；另断言滚动之前行确实在位（`inPlace`），免得 `placedTop` 是 NaN 时这一步什么也没查。"不超过滚动的一半"去掉了。量到最紧的是 Guide 往上 1500：1301 对门槛 375。这一格仍只用 Prose，不为 Guide 另加格：比行的位置之后 Guide 的余量与 Prose 相当。
- 先红与修后：见"变异"B-F2-N1 一行（两组各 9 次全红在 `pg17:705`）；修后 9 次全过，拷回原文件之后三种引擎各 1 次又全过。那四条只由推理守着的变异照旧，没有重做：跳转与滚动的时刻没有变。
- 门禁：`lint-web knip test-web` rc 0，69 秒，vitest 234 / 5623，`check.mjs` 437/108/108/3；`CI=1 make e2e` 656 格，652 过、3 败（961 的三格）、1 跳，182 秒；PG17 那一格三种引擎各 10 次，30 次全过。

修复轮 5（`d734488f`，`web/apps/web/src/editor/source-editor.tsx` 一行条件与注释、`e2e/stories/page/pg17-page-head.spec.ts` 一步，+24 / −9）：
- 修法：`scrolled()` 里"窗口到底"那一条改成 `windowAtFoot() && top >= 0 && top < window.innerHeight`，同 PG17 的 `placed()`。"窗口在页底、那一行在窗口里"正是跳到一节到不了顶的行之后留下的样子，仍算到位；外来的滚动把窗口带到页底、那一行已出了窗口，就不算。
- 没挡的一种：外来的滚动落到页底、那一行却还在窗口里。跳转把行放在窗口顶下约 16 px，只有不到这个距离的滚动会落进去，那时窗口与行都还在跳转放的地方附近，不另加条件（实施者第七部分）。
- 新的一步：跳到 Section 15，`scrollBy(0, 100000)`，断言 `inPlace`、滚完那一刻窗口在页底（`atFoot`），`expectStayed` 的门槛取实际滚动距离的四分之一（Chromium 与 WebKit 8917 px 的四分之一 2229 px，Firefox 2223 px）；修后那一行停在 −9425 px（Firefox −9372），离落定处是门槛的四倍多。
- 先红与修后：见"变异"C-F3-N1 一行；修后 PG17 两格三种引擎各 3 次，18 次全过，Section 40 那一步与 I1 照旧。vitest 里没有查 `scrolled()` 条件的格，没有改。
- 门禁：`lint-web knip test-web` rc 0，66 秒，vitest 234 / 5623，`check.mjs` 437/108/108/3；`CI=1 make e2e` 656 格，653 过、2 败（SE2、LW8，961；PG15 过）、1 跳，173 秒；PG17 两格三种引擎各 10 次，60 次全过。

**第 4 轮**（`d734488f`，只请 B）：
- B：C-F3-N1 修好。探针 jumpend 8 种、jumpend2 6 种、jumpscroll 22 种、jumpshift 16 种与 I1 的落点（每种引擎 45 次，画出来的帧 30 次，0 帧偏离）与第 3 轮逐次差都不到 1 px，三种引擎没有一次拉回；B-F3-Q1 那一格照旧停在 131.1 px。跳到 Section 15 再用 `scrollBy`、滚轮（Firefox 的滚轮只送到 6541）、`Ctrl/Cmd+End` 滚到页底都不拉回，Section 36–39 正好滚到页底也不拉回。实施者说没挡的一种不用管：跳转把行放在窗口顶下 10.3–11.4 px，往下滚到页底而行还在窗口里时 [0, 48] 那一条本来就成立，剩下的余地是 [0, 48] 原有的；把窗口高度调到跳转后只差 2–7 px 到底再滚到底，1.3 秒里窗口一 px 也没动。PG17 新的一步不空查（条件改回 `\|\| windowAtFoot()`，3/3 红在 `pg17:711`），误红的风险低（门槛约 2229 px，行最后离原处约 9437 px，编辑器量行高那一挪的方向是离开原处）。没有新的问题。真机上拖滚动条、浏览器的页内查找与读屏没有看，只用 `scrollBy`、滚轮与按键代替。

## 全量门禁

合并进 `main` 之后写。`main` 上合并之后的头（`<待填>`），干净的分离 worktree 经 `heavy.sh` 一次持锁：

| 项 | 用时 | 结果 |
|---|---|---|
| `pnpm install --frozen-lockfile` | | |
| `make lint`（含两个 `check.mjs`；M13 里第一次跑 `lint-go`，`make tools` 要下载 golangci-lint） | | |
| `make gen-check` | | |
| `make knip` | | |
| `make build` | | |
| `make test` | | |
| `make test-go-short` | | |
| `make e2e` | | 照决定 961：只败 SE2 [chromium] `se2:231`（`toHaveValue`）、LW8 [webkit] `lw8:836`（`toBeFocused`），PG15 [webkit] `pg15:113`（deep equality）时红时过，同行同消息；多出来的红先看服务端的日志，停顿的单独重跑、照实写；rc 照实写 |
| `make image-smoke`（另一次持锁，在合并之后的头上另 `git clone` 的普通克隆里） | | （`modified=false`） |

## 门禁与分支 CI

- **逐个提交**（都经 `heavy.sh`）。提交 1–5 的门禁在提交之前的同一棵树上跑，之后的在提交了的树上跑。基线 `eb48f4a1`：vitest 233 个文件 / 5598 格，E2E 647 格（643 过、3 败、1 跳），`check.mjs` 434/105/105/3。每次 E2E 失败的都有决定 961 的三格，同行同消息（PG15 有时过），表里只写多出来的。

| 提交 | `make lint-web knip test-web` | vitest（文件 / 格） | `check.mjs` | `CI=1 make e2e`（过 / 败 / 跳，共） |
|---|---|---|---|---|
| `f181683f` | rc 0，71 s | 233 / 5609 | 434/105/105/3 | rc 2，171 s；643 / 3 / 1，647 |
| `8f8e6145` | rc 0，69 s | 233 / 5614 | 434/105/105/3 | rc 2，169 s；643 / 3 / 1，647 |
| `8ff51ade` | rc 0，64 s；改格之后重跑 65 s | 233 / 5614 | 436/107/107/3 | 第一次 rc 2，177 s，648 / 4 / 1：多一格 PG17 [firefox]，页面的安静检查收到 scroll-linked 的警告（见"实施中定的"）；改格之后重跑 rc 2，166 s，649 / 3 / 1，653，才提交 |
| `7cacd2a2` | rc 0，75 s | 233 / 5616 | 436/107/107/3 | rc 2，186 s；649 / 3 / 1，653 |
| `f5d84965` | rc 0，66 s | 233 / 5616 | 436/107/107/3 | rc 2，172 s；649 / 3 / 1，653 |
| `e1b24d5e` | rc 0，65 s | 234 / 5619 | 436/107/107/3 | rc 2，176 s；649 / 3 / 1，653 |
| `0d6b2b04` | rc 0，69 s | 234 / 5620 | 436/107/107/3 | rc 2，179 s；648 / 4 / 1，653：多一格 AS6 [chromium] `as6:167`，服务端日志里没有停顿，查清是这一格自己的时序（`aad1ab44` 修） |
| `e404d0d7` | rc 0，66 s | 234 / 5620 | 437/108/108/3 | rc 2，179 s；650 / 5 / 1，656：多两格 Firefox `a10:145`、`pg17:128`，20:56:02.3 被送回登录页（续期答 500，又 401），同一刻服务端日志各有一条 `commit transaction: timeout: context deadline exceeded`（2.0 秒）；照决定 961 单独重跑，2 过 |
| `aad1ab44`（只改测试） | rc 0，70 s | 234 / 5620 | 437/108/108/3 | 只跑 AS6 这个文件，三个浏览器项目都指定：3 passed（照设计只在 Chromium 里跑）；二进制同 `e404d0d7` |
| `91a62e60` | rc 0，75 s | 234 / 5623 | 437/108/108/3 | rc 2，173 s；656 格，多一格 C8 [chromium] `c8:159`，不是停顿，单独跑 20 次全过，查清是这一格自己的时序（`4470c25a` 修） |
| `41fbce8f` | rc 0，71 s | 234 / 5623 | 437/108/108/3 | rc 2，172 s；656 格 |
| `f2130709` | rc 0，70 s | 234 / 5623 | 437/108/108/3 | rc 2，178 s；656 格 |
| `4470c25a`（只改测试） | rc 0，76 s | 234 / 5623 | 437/108/108/3 | 只跑 C8 这个文件，三个浏览器项目都指定：11 格全过（照设计只在 Chromium 里跑） |
| `524734f1`（只改测试） | rc 0，69 s | 234 / 5623 | 437/108/108/3 | rc 2，182 s；652 / 3 / 1，656 |
| `d734488f` | rc 0，66 s | 234 / 5623 | 437/108/108/3 | rc 2，173 s；653 / 2 / 1，656（PG15 过） |

- **合并之前的整条命令**（`CI=1 heavy.sh make lint-web knip test-web gen-check-web build-web e2e`；`gen-check-web`、`build-web` 每次都过）：

| 头 | 谁 | 结果 |
|---|---|---|
| `7cacd2a2` | 实施者 | rc 2，253 秒，结果同下一行；之后加了提交 5，以下一行为准 |
| `f5d84965` | 实施者 | rc 2，248 秒（18:45:47 起）；vitest 233 / 5616；E2E 653 格，649 / 3 / 1，Playwright 2.9m，只败三格；`check.mjs` 436/107/107/3；受保护的路径（上面第 5 条那条 `git diff --quiet`）rc 0；`git diff --check m13-ui-refresh...HEAD` 没有输出，62 个文件，+1140 / −127 |
| `f5d84965` | C（第 1 轮） | rc 2，273 秒（19:14:23–19:18:56）；vitest 233 / 5616；E2E 653 格，647 / 5 / 1，3.1m：三格之外两格 Firefox（`lw3:129` account MCP 的 write 答 `internal_error`，`lw12:126` 的 DELETE 答 500），19:17:44.0–44.5 同一刻 `commit transaction: timeout`，单独重跑 3 passed；受保护的路径 rc 0，`git diff --check` 没有输出 |
| `e404d0d7` | 实施者 | rc 2，236 秒（21:01:38 起）；vitest 234 / 5620；E2E 656 格，653 / 2 / 1，2.8m（PG15 过）；`check.mjs` 437/108/108/3；受保护的路径 rc 0；66 个文件，+1547 / −135 |
| `aad1ab44` | C（第 2 轮） | rc 2，260 秒（21:18:19–21:22:39）；vitest 234 / 5620；E2E 656 格，652 / 3 / 1，2.9m，只败三格，AS6 过；`check.mjs` 437/108/108/3；受保护的路径 rc 0；67 个文件，+1551 / −135 |
| `f2130709` | 实施者 | rc 2，260 秒（22:15:26 起）；vitest 234 / 5623；E2E 656 格，652 / 3 / 1，只败三格；`check.mjs` 437/108/108/3；受保护的路径 rc 0；67 个文件，+1722 / −136 |
| `524734f1` | C（第 3 轮） | rc 2，257 秒（23:02:11–23:06:28）；vitest 234 / 5623；E2E 656 格，653 / 2 / 1，2.9m，只败 961 的两格（SE2 `se2:231`、LW8 `lw8:836`，同行同消息；PG15 过）；`check.mjs` 437/108/108/3；受保护的路径 rc 0；`git diff --check` 没有输出；68 个文件，+1750 / −136 |
| `d734488f` | 主持者 | rc 2，270 秒（干净的分离 worktree，先 `pnpm install --frozen-lockfile`）；vitest 234 / 5623；E2E 656 格，652 / 3 / 1，3.0m，只败 961 的三格（SE2 `se2:231` `toHaveValue`、PG15 `pg15:113` deep equality、LW8 `lw8:836` `toBeFocused`，同行同消息）；`check.mjs` 437/108/108/3；受保护的路径 rc 0；`git diff --check` 没有输出；68 个文件，+1765 / −136 |

  `f2130709` 之后的三个提交：`4470c25a`、`524734f1` 只改测试，`d734488f` 改一行代码、加一步测试，各自跑了上表的门禁；`524734f1`、`d734488f` 的 E2E 是整套的 656 格。C 的第 3 轮在 `524734f1` 上跑了整条命令，见上表；合并之前在最后的头 `d734488f` 上由主持者再跑一次，见上表末行。
- **分支 CI**：`m13-closeout` 自己不推送。收尾合并回 `m13-ui-refresh`（`<待填>`）之后的 CI：合并回 `m13-ui-refresh` 之后写。合并进 `main` 之后第一次 `main` 的 CI：合并进 `main` 之后写。

## 没能验证的风险

- **只靠 CI 的几步**：Linux 的字体下 hi2 的位置断言、UP1 的版本页一步、tr2 的点击记录、`expectButtonsInside`；已在 run 45773568132 上过了。收尾的修复里第一次上 CI 的：PG17 的三个新格（I1 的 [0, 48]、面包屑的逐像素设宽、跳转之后的 `scrollBy` 三步与 `|by| / 4` 的门槛）、PG16 第 1 格按住的键的四步与第 2 格的 Tab、UI2 的"Workspace settings"，等收尾合并之后的 `m13-ui-refresh` 的 CI。B 没有量 Linux 的字体下 Prose 的估计差有多大；`524734f1` 之后断言比的是那一行在屏幕上的位置，不比 `scrollY`。
- **B-I1 的修法在慢的机器上**：再对准至多 4 次、1 秒之内（实施者定的）。很慢的 runner 上，再对准要的两三帧加起来超过 1 秒，就会停在半路，那一格会红，读者也会看到；按 16–50 ms 一帧算，余量在 20 倍以上（C）。375 宽往上跳 B 核过 15 次都对；Firefox 在 375 宽从页下面的大纲往上跳报 scroll-linked 的警告，没修（v0.2 的移交第 8 项）。真的拖滚动条、页内查找、读屏的滚动只用 `scrollBy` 代替；结束对准的几种先后里有四条只由推理守着（见"变异"）；WebKit 里跳转后头几帧编辑器之上的布局变了会停在 131 px（B-F3-Q1，应用里没有找到会在那几帧变的东西）。
- **B-M2 的修法在 Linux 的字体下**：PG17 那一格把面包屑从 120 到 1200 px 逐像素设宽，不靠某种字体下的某个宽度，所以与字体无关；macOS 上三种引擎的扫描全是 0/286。RTL 的名字没有看（B）；`Intl.Segmenter` 在 Firefox 125 起才有，只看了代码（B）。
- **967 的"挡默认"那一半**（只去掉 `preventDefault()`）没有 e2e 守，只由 vitest 守，读屏清单 H 第 22 项的人工核对补上。Linux X11 下成对的 keyup/keydown 重复挡不住，没有环境核实（v0.2 的移交第 11 项）。系统真实的重复节奏只用"等焦点到了再重复"代替，"⋯"自己的 `onKeyDown` 对每次 keydown 都开菜单，比菜单拿到焦点还快的重复会落在"⋯"上（只是同一个菜单再设一次），没有量。
- **`expectFits` 先等弹入**：只由实施者的探针证明，没有提交的格守（C）。
- **Linux 的 WebKit 下拉画 2 px 的环**（P1/S2）：没有故事断言它，CI 绿证明不了。
- **真的 Safari 里编辑时显着的"保存"**（04-P4 第 9 节"负责人"）：进人工清单"其余各项"第 15 项。
- **外观的判断只靠截图**（决定 725）；Windows 的高对比度只在 Chromium 与 Firefox 的模拟里看过，Chromium 的模拟不认暗色，暗色只在 Firefox 看了（A）。
- **可能用手指点的设备**（决定 953、966）：CDP 模拟不了只有 `any-pointer` 是 coarse 的设备，进 v0.2 的移交第 10 项；`hover: none` 那一支 A 在触屏上下文里核到了。
- **真的读屏**：A、B 都只核了代码与可访问树，H 一节的各项没有在读屏里听过；VoiceOver 会不会把链接里子元素的 `title` 当作提示读出来，没有核实（A）。
- **E2E 一步的 12:01 是 runner 慢**：推断，等"CI 的数"里的两次运行。

## 列给负责人的

照 M12 收尾记录的做法：只列要负责人做、要负责人知道、或负责人可能想改判的几件，不逐条抄主持者的决定。决定 715–960 共 246 条（C 扫过，没有缺号、没有两条共用一个号）：715–746 在 00 号文档第 11 节，747–960 在各 Phase 设计与各 Step 的记录；961–971 在 00 号文档第 11 节"M13 收尾的决定"，都可以改判。

| # | 哪件 | 现在怎样 | 出处 |
|---|---|---|---|
| 1 | 推送收尾合并之后的 `m13-ui-refresh` 与合并进 `main` 之后的 `main` | 主持者没有推送的凭据时，本机的门禁替代分支 CI，负责人推送之后 CI 在头上补跑；两次的 CI 写进"CI 的数" | 决定 746、761 |
| 2 | 推 `v0.1.0-rc.1` 与 `v0.1.0` 两个 tag（M12 的发布） | 没问；M13 合并进 `main`、`main` 的 CI 绿之后照 00-M12 第 7 节问；[M12 收尾记录](../../M12-release/reviews/M12-closeout-review.md)"列给负责人的"那张表照旧 | 负责人 2026-11-22 第 5 问；00-M12 4.11 第 2 步 |
| 3 | CI 的 E2E 一步 12:01 | 推断是 runner 慢；两次都回到 8–9 分钟就结案，都在 11 分钟以上推 tag 之前先查 | C-M6；"CI 的数" |
| 4 | 本机的 `make e2e` 只允许三格失败 | `main` 上的完整门禁照此；绿的证据是合并之后 `main` 的 CI | 决定 961、764、795(a) |
| 5 | 视觉优先的取舍 | 说明与占位的字、焦点圈、删除类的红字的对比度低于 WCAG 的门槛（浅色对白 3.07、3.10、4.38）；`CHANGELOG.md` 0.1.0 的已知限制写了 | 负责人 2026-11-22 第 1 问；决定 716、736、963；00 号文档 4.7 |
| 6 | 亮色琥珀比样稿深一档（`#c2871f`，样稿 `#c58a22`） | Chromium 的 `accent-color` 不到 3:1 时给复选框画深色的勾，与样稿相反；色相看不出差别 | 决定 766（P1/S2） |
| 7 | 左栏的"设置"改名"工作区设置" | 与账户菜单的"设置"（个人设置）分开，照样稿 | 决定 964；A-Q2 |
| 8 | 收尾里界面上的小处 | 名称的 Esc 退回草稿（965）、悬停才显出的同一套条件（966）、按住的键只按一下（967）、面包屑的分隔符（968）、一页一个图标（969）、窄屏的下拉（970）、切换器的菜单（971） | 00 号文档第 11 节"M13 收尾的决定" |
| 9 | 搜索框与快速切换不合并成跳转框 | 照做；⌘K 只聚焦搜索框 | 决定 719 |
| 10 | 只用系统字体 | 照做；中文网页字体以 MB 计 | 决定 722 |
| 11 | 不建像素对比的门禁 | 外观靠审查时的截图并排；v0.2 的移交第 2 项 | 决定 725 |
| 12 | 搜索摘要里的 Markdown 原文 | 要改服务端，不在 M13；v0.2 的移交第 1 项 | 决定 721 |
| 13 | 列表不加骨架 | 读取中照旧一行"加载中…"；v0.2 的移交第 3 项 | 决定 914 |
| 14 | 带锚点打开的页，焦点给锚点、不看读者其间的动作 | 照现状；v0.2 的移交第 7 项 | 决定 900 |
| 15 | 可能用手指点的设备上，行内的次要操作与左栏的齿轮、＋、"⋯"静止也显着 | 只按代码推，真机没有看过；v0.2 的移交第 10 项 | 决定 953、966 |
| 16 | 对话框里放不下的按钮各占一行，没改的七行 | 今天在 320 宽中英都放得下，照旧；v0.2 的移交第 9 项 | 决定 958；C-N5 |
| 17 | 还欠的人工验收 | 读屏清单（A–H）、输入法清单（23 步，第 21 步带 (d)）、MCP 客户端清单（A–K）与"其余各项"十五项，都在 v0.1.0 发布之后，入口 [manual/README.md](../../M12-release/manual/README.md) | [v0.1.1 的移交](../../../v0.1.1/handoffs/M12-after-release.md) |
| 18 | Safari 里编辑时显着的"保存" | 进"其余各项"第 15 项 | 04-P4 第 9 节"负责人"；C-M3 |

## 秘密

没有：收尾的 e2e 与探针照 `make e2e` 的做法起服务、用夹具与探针在测试库里造的账户与令牌，都在跑完即弃的测试库里，没有用真的账户或 PAT；三位审查者对 `gh` 都只读；提交里没有密码、私钥与令牌（`eb48f4a1..524734f1` 只改 `web/`、`e2e/` 与 `tools/e2e-projects/check.mjs`，文档起草者以 `git diff` grep 过）。

## 人工验收

在 v0.1.0 发布之后由负责人做，清单与要关的移交在 [v0.1.1 的移交](../../../v0.1.1/handoffs/M12-after-release.md)；入口是 [manual/README.md](../../M12-release/manual/README.md)。通过之后改 00 号文档的头与第 12 节、总体设计 12.6 与第 15 节。
