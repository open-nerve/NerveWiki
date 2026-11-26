# M13/P1 设计基础：架构设计 & 实施规划

| 项 | 内容 |
|---|---|
| Phase | M13/P1 设计基础（两个 Step：S1 token 与主题；S2 基础组件） |
| 日期 | 2026-11-22（初稿；同日照三份设计审查修订，照修订核对第 1 轮再修订，第 2 轮之后补上最后的 Nit，[审查记录](reviews/P1-design-review.md)；同日照 S1 的实施与审查修订，[S1 的审查记录](reviews/P1-S1-tokens-theme-review.md)；同日照 S2 的实施与审查修订，[S2 的审查记录](reviews/P1-S2-components-review.md)） |
| 状态 | 完成（2026-11-22）。设计定稿（修订核对第 2 轮之后三位都没有 Important、Minor）；S1 已合并 `a08bf50f`（[S1 的审查记录](reviews/P1-S1-tokens-theme-review.md)）；S2 已合并 `e2678ff7`（[S2 的审查记录](reviews/P1-S2-components-review.md)） |
| 上级文档 | [M13 总设计](00-M13-design.md) 4.1、4.3、4.7、4.8、第 7 节 P1 两行与"移交的落实""总体设计的修订"第 5、6、7 项、9.1 的"对比度门槛""原生控件"两行、第 10 节"下拉的 `appearance: none`"一行、第 11 节决定 716、718、722、735–737、745、746；[v0.1 总体设计](../v0.1-design.md) 9.1、13.2 第 26、31 条、13.4 第 3 条；移交 [v0.2/handoffs/M12-polish.md](../../v0.2/handoffs/M12-polish.md) 第 5 项 |
| 视觉参照 | [mockups/palette.html](mockups/palette.html)（色值、按钮、菜单、输入框、下拉、单选）；[mockups/settings.html](mockups/settings.html)（危险按钮、输入框） |
| 代码位置的写法 | 同 00 号文档：`W/` 是 `web/apps/web/src/`；e2e 写作 `e2e/stories/…:行`。行号是 `3c315037` 的；`m13-ui-refresh` 的头 `65dbfcbb` 与它之间只改了文档（`git diff --stat 3c315037 HEAD -- web e2e tools` 为空） |
| 工作笔记 | 设计只读。设计者跑过 Tailwind 4.3.3 的 `compile`（取舍 752）；三位审查者另跑过 `compile`、tailwind-merge 3.7.0、vitest 5.0.2 的 `?raw`、本机缓存里旧版的 Chromium（截图与计算之后的颜色）；探针在会话的临时目录，不进仓库。对比度的预期值由 4.1.2 的十六进制按 WCAG 2 算出（8 位合成，与 `e2e/fixtures/contrast.ts` 同），实量在 S1 做。本机还没有 Playwright 1.63 的三个浏览器（第 6 节第 0 步） |

---

## 0. 分两个 Step

照 00 号文档第 7 节，P1 分两个 Step，串行，各自一个分支（`m13-p1-s<m>`，从 `m13-ui-refresh` 拉出、合并回它）、一次审查与修复核对、一次合并。每个提交单独为绿（7.3）。

- **S1 token 与主题**（`m13-p1-s1`）：`W/styles.css` 换成 4.1.2 的变量（亮、暗）与 Tailwind 的映射，`--ring` 取 `var(--accent)`；`accent` 按意思分开改；控件的焦点环 3 px → 2 px；危险按钮改成红字描边，描边按钮换底、悬停叠一层；对话框的面与遮罩、搜索命中的高亮、三处不存在的 `bg-muted` 改用新的变量（取舍 748）；tailwind-merge 登记新的圆角、阴影与字号（取舍 756）；一个守住"只用语义名"的 vitest（取舍 751）；pg4、se1、l5 的门槛改成新配色的回归守卫（4.1.8）；总体设计 9.1 的一句、13.2 第 31 条的改写与新的第 32 条、13.4 第 3 条的一句（8.2）。**单独为绿**：只改样式、类名、`cn` 的配置、三处钉住 `bg-destructive` 的 vitest 与三个故事；没有新的导出。
- **S2 基础组件**（`m13-p1-s2`，S1 合并之后开）：按钮、输入框、标签、表单字段、提示条、下拉、单选与复选、菜单、对话框、标签页照 4.3 换装；进场动画；菜单项的环只在键盘操作时出现（取舍 760）；令牌期限的裸 `<select>` 换成 `NativeSelect`；两处"删除"菜单项用危险的样子；l5 加一句"关上菜单之后一帧之内没有 `menu`"。**单独为绿**：只改组件、用它们的地方的类名、一个新模块 `W/app/input-modality.ts`（由 `main.tsx` 调用，knip 不报）与 l5 的一句；三种引擎里核对换装的下拉（4.2.10）。

00 号文档 P1/S2 一格里的"空状态、骨架"，与 4.3 的按钮 30 px 尺寸、胶囊形、菜单的子菜单与快捷键，不在 P1：见 2.1 第 1、2 条。

## 1. 基线

- **变量**（`W/styles.css:1-73`）：shadcn 的中性 `oklch` 一套，亮在 `:root`、暗在 `.dark`；`--accent` 是"悬停与选中的底"，另有 `--accent-foreground`；`--ring` 是灰（`oklch(0.556 0 0)`）；`--radius` 是 `0.625rem`，`--radius-md`、`--radius-lg` 由它算出（`:23-24`）。没有侧栏、次一级的面、悬停与当前分开的底、阴影与圆角的分级、字号的变量。
- **`accent` 的用处**：17 处 `bg-accent`（含 `hover:`、`focus:`）、5 处 `text-accent-foreground`，在 10 个文件里；另有 `var(--accent)` 3 处（`W/editor/theme.ts:34`、`W/editor/text-diff.tsx:417`、`W/editor/loaded/link-completion.ts:131`）与 `var(--accent-foreground)` 1 处（`link-completion.ts:132`）。逐条在 4.1.3。
- **不存在的颜色类**：`bg-muted` 3 处（`W/pages/notebook/upgrade-page.tsx:659` 的行内代码、`W/pages/notebook/upgrade-banner.tsx:39`、`W/pages/notebook/transfer-report.tsx:236`）。`styles.css` 没有 `--color-muted`，这个类从来不产出 CSS，那三处一直没有底（审查 A、C 的 `compile` 探针）。
- **焦点**：全局的 `:focus-visible` 是 2 px 的 `--ring` 外框、偏移 2 px（`W/styles.css:66-69`）；按钮、输入框、下拉、标签页与两处卡片是 `focus-visible:ring-[3px] ring-ring`（7 个文件，4.1.4）；菜单项（`focus:` 的内嵌 2 px 环）、快速切换的选项与补全的选中项是内嵌的 2 px 环。Radix 的菜单在指针移过时就把焦点给菜单项，所以今天用鼠标时菜单项也带着环（审查 B 的探针）。阅读视图的链接有自己的外框（`W/reading/reading.css:122-127`），不走全局的规则。
- **危险按钮**：白字红底（`W/components/ui/button.tsx:14-15`，暗色 `/60`、悬停 `/50`）；3 处 vitest 认它：`W/app/confirm-dialog.test.tsx:88`（`classList.contains`）、`W/pages/notebook/upgrade-page.test.tsx:1188,1422`（`className` 的子串）。
- **Tailwind 的调色板与色值**：调色板只有 3 处（`W/components/ui/dialog.tsx:12`、`alert-dialog.tsx:16` 的遮罩 `bg-black/50`，`button.tsx:15` 的 `text-white`）；源码里写死的色值只有 `W/editor/theme.ts:15-26` 的编辑器链接色与搜索匹配色（P4/S3 改）。
- **`cn`**（`W/lib/cn.ts:4-7`）：`twMerge(clsx(…))`，tailwind-merge 没有配置，不认 Tailwind 主题之外的名字：`twMerge("rounded-row","rounded-full")` 两个都留，结果由 Tailwind 输出的字母序定（审查 B、C 的探针）。
- **vitest 里 CSS 的 `?raw` 是空串**：vitest 5.0.2 的 `vitest:css-empty-post` 把 `.css?raw` 也当成 CSS，`vite.config.ts:41-49` 没有 `test.css.include`，于是 `import s from "./styles.css?raw"` 得到 `""`；TS 文件的 `import.meta.glob(…, { query: "?raw" })` 照常读到原文（519 个文件）。读 CSS 的先例是 `W/reading/patch.test.ts:1-2,14` 的 `readFileSync`。
- **对比度的三个故事**（只在 Chromium）：pg4 的"删除"白字 ≥ 4.5（`e2e/stories/page/pg4-delete-subtree.spec.ts:191-205`）；se1 的命中字 ≥ 4.5、五处焦点 ≥ 3、命中链接是 `outline solid`（`e2e/stories/search/se1-search.spec.ts:279-308,322-332,369-380`）；l5 的补全说明 ≥ 4.5、编辑器聚焦框与补全选中项 ≥ 3、阅读视图链接的焦点与菜单项的指示 ≥ 3（`e2e/stories/links/l5-completion.spec.ts:127-140,165-183`）。夹具 `e2e/fixtures/contrast.ts` 只量、不带门槛（00 号文档 4.7：不改）；它返回指示的形状（`outline solid 2px`、`ring 3px`、`ring 2px inset`）。
- **组件**（`W/components/ui/`）：`alert`、`alert-dialog`、`button`、`dialog`、`dropdown-menu`、`input`、`label`、`native-select`、`tabs`；另有 `W/components/form-field.tsx`、`loading.tsx`、`nav-item.tsx`。没有动画库，Radix 的内容没有进退场动画。
- **原生控件**：单选 4 处（`W/pages/settings/profile-page.tsx:62,77`、`W/pages/workspace/mode-options.tsx:28`、`W/pages/notebook/access-options.tsx:34`）；阅读视图任务项的复选框是服务端的 `<input type="checkbox">`，只有可写时才启用（`W/reading/task-toggle.ts:5-6,14`）；裸的 `<select>` 只有令牌期限一处（`W/pages/settings/create-token-dialog.tsx:114-127`，带 `aria-invalid` 与 `w-full`）；`NativeSelect` 有 5 个调用处带 `aria-invalid`，它自己没有无效的样子。
- **标签页**：只有版本页用（`W/pages/page/version-page.tsx`），下划线式；`version-page.test.tsx`、`app/event-stream.test.tsx` 按角色找 tab。
- **本机**：Docker 在；Playwright 1.63 要的 chromium-1243、firefox-1543、webkit-2359 不在（缓存里只有更旧的版本，旧的 WebKit 在 1.63 下启动挂住）；仓库里还没有 `bin/`。

## 2. 目标与范围

做 00 号文档 4.1（颜色、形状与层次、字、动效的变量）、4.3（控件）、4.7（焦点与对比度的改写，三个故事的守卫），以及"总体设计的修订"第 5 项的设计基础一句、第 6 项的第 31 条与新条目、第 7 项的对比度一句。外框、设置、阅读与编辑、其余页面的布局不在 P1；它们用 P1 的变量与组件。

阅读视图的变量（`W/reading/reading.css` 的代码块底、hljs、标签、callout、链接下划线）与编辑器的语法色、链接色、搜索匹配色在 P4（P4/S2、P4/S3）；P1 只改其中页面共用的一个：搜索命中也用的 `--nw-mark-background`（取舍 748）。

**移交**：认领 [M12-polish](../../v0.2/handoffs/M12-polish.md) 第 5 项的最后一小项"Playwright 的 WebKit 里原生下拉只有 1 px 的 `ring` 色边框、没有 3 px 的环"（P1/S2 换装下拉时在 WebKit 里看，4.2.10）；第 11 项"真的 Safari 上原生下拉的焦点"要人工在 Safari 上看，不认领。S2 的文档提交在那份移交的"处理进展"写一行。

### 2.1 对 00 号文档的修订

由一个带 `--check` 与 `--path` 的脚本做（02-workflow 2.3），每处一个带标签的替换，与本文同一个提交。锚点与新文字：

1. **空状态与骨架挪到 P5**（取舍 747）：第 7 节 P1/S2 一格去掉"空状态、骨架（保留 `<output>`）"，P5/S1 一格加"空状态的组件（4.3，第一批用处是两个首页）"，P5/S2 一格加"这些页面的空状态；骨架（只在某个列表确实需要时加，4.3）"；4.3 的"空状态"一条句末加"组件在 P5/S1 随第一批用处建"。P1 里没有用到它们的页面：建了而没人用，knip 报出没人用的导出，Step 不能单独为绿（02-workflow 3.3）。
2. **按钮的 30 px 尺寸与胶囊形、菜单的子菜单与快捷键挪到 P2/S1**（取舍 747）：现有的按钮只用 36 px 与图标两种，菜单没有子菜单、没有快捷键的标注；工具栏的胶囊按钮与头像菜单的"外观"子菜单（4.2.3）是第一批用处。第 7 节 P1/S2 一格注明，P2/S1 一格加"按钮的 30 px 与胶囊形、菜单的子菜单与快捷键（4.3）"；4.3 的"按钮""菜单"两条句末注明在 P2/S1 加。
3. **S1 的边界**（取舍 748）：第 7 节 P1/S1 一格加"对话框的面用 `--surface`、遮罩用 `--overlay`、描边按钮换底、`--nw-mark-background` 换成 reading.html 的值、三处不存在的 `bg-muted` 改成 `--surface-2`、tailwind-merge 登记新名字"；"`grep accent` 的核对"注明由 `design-tokens.test.ts` 守。
4. **4.1 的表**：
   - 加 `--overlay` 一行（亮 `rgb(16 17 20 / 32%)`、暗 `rgb(0 0 0 / 45%)`，对话框的遮罩，"遮罩更淡"）与 `--elevation-popover`、`--elevation-dialog` 一行（值见 4.1.2）；
   - `--surface`（`--popover` 同值）一行改为：`--popover` 亮同 `--surface`、暗 `#25262a`（照样稿 palette.html:169 的暗色菜单，取舍 758）；
   - `--ring` 一行写明"取 `var(--accent)`，两种主题都只有 `--accent` 一个来源"（取舍 755）；
   - "字"一段加"界面的两个常用字号是变量：`--text-compact`（13 px：紧凑的按钮、标签、分组标题）、`--text-row`（13.5 px：侧栏行、菜单项、对话框的说明）"（取舍 752）。
5. **4.7 的门槛**："每种主题各按 P1/S1 实量的值减 0.1 设下限"之后加"；之后改了被量元素的底或颜色的 Step（例如 P2 把搜索框挪到侧栏上），照同一规则重量、改下限，写进它的记录"（审查 A 的 M3）。
6. **4.3 的菜单与 4.7 的焦点**：4.3"菜单"一条的"另有内嵌的 2 px `--ring` 环"改为"键盘操作时另有内嵌的 2 px `--ring` 环（用鼠标时只有底色，同样稿）"（取舍 760）。
7. **第 7 节 P1/S2 的"提示"**注明指 `Alert`：网页没有悬停提示的组件，P1 不加。
8. 第 7 节"总体设计的修订"第 5 项的第一句（"加一句"界面的设计基础（颜色、圆角、阴影、字号、动效）是 `styles.css` 与 `reading.css` 里的 CSS 变量，组件只用语义名（M13）""）之后注明"（实际的写法见 01-P1 8.2 第 1、6 条：S1 写颜色、圆角、阴影、字体与字号，S2 加动效）"。
9. 第 12 节 P1 一行的"Phase 文档"一格链到本文、"审查"一格链到本文的审查记录；第 13 节加一行"2026-11-22：照 P1 的设计修订（01-P1 2.1）"。

### 2.2 需要负责人确认

（无。装 Playwright 的浏览器要下载，第 6 节第 0 步在开工之前单独问负责人；这是环境，不是设计的问题。）

### 2.3 本文的取舍（主持者的决定，负责人可以改判）

- **747 只建有人用的部件**：空状态、骨架、按钮的 30 px 与胶囊形、菜单的子菜单与快捷键不在 P1（2.1 第 1、2 条）。S2 加的唯一新属性是菜单项的 `tone`，用在两处"删除"。
- **748 S1 的边界**：对话框的面、遮罩、描边按钮的底、`--nw-mark-background`、三处 `bg-muted` 与 tailwind-merge 的登记进 S1（2.1 第 3 条）：pg4 量的是对话框里的"删除"，se1 量的是命中的高亮，它们的底在 S1 定下，三个故事的下限只量一次；描边按钮放在暗色对话框（`--surface`）里时，留着旧的 `bg-background` 会比面暗（审查 A 的 N6）。阅读视图的其余颜色进 P4/S2。S2 只动圆角、阴影、字号、间距、输入框的底与动画，不改三个故事量到的颜色（输入框与下拉的环量的是元素外面的底）；若改了，照 4.1.8 的规则重量、改下限，写进 S2 的记录。
- **749 危险按钮的焦点环也是琥珀**：焦点圈的意思是"焦点在这里"，不是"危险"；`focus-visible:ring-destructive`（`button.tsx:15`）去掉。无效字段的环照旧是 `ring-destructive`（se1:369-380 量它）：那里红色说的是"这里不对"。
- **750 门槛的写法**：正文的字固定 7:1 为下限（00 号文档 4.7"正文字至少 7:1"）；其余每一项、每种主题各按 S1 实量的值减 0.1，向下取到 0.01。下限写在故事里一个按主题分的表中，注释写实量值与"M13/P1/S1"；夹具不改。"正文的字"在三个故事里只有 se1 的两处命中字（摘要里的与标题里的）。
- **751 一个守住约定的 vitest**（`W/design-tokens.test.ts`，4.1.7）：五个测试，`styles.css` 用 `readFileSync` 读（`?raw` 在 vitest 里是空串，第 1 节），每个测试先断言读到了东西（05-lessons 第 1 节：自检的样例证明不了遍历真的读到了文件）。13.2 新第 32 条（8.2）的可检查部分由它守，免得只靠审查。
- **752 圆角、阴影与字号怎样进 Tailwind**：探针（`compile`，Tailwind 4.3.3）表明 `@theme { --radius-row: 7px }` 产出 `rounded-row { border-radius: var(--radius-row) }`，且在被工具类或 CSS 的 `var()` 用到时在 `:root` 写出 `--radius-row`；`@theme inline { --shadow-popover: var(--elevation-popover) }` 产出 `shadow-popover`，值取 `:root`、`.dark` 里各自的 `--elevation-popover`。所以：五个圆角与两个字号（与主题无关）放 `@theme`；阴影（亮暗不同）的值放 `:root`、`.dark`，经 `@theme inline` 映射；颜色照旧经 `@theme inline`；`--radius-md`、`--radius-lg` 照旧在 `@theme inline`。TS 里的编辑器主题照旧只用 `var(--radius)`（`:root` 里的，总是写出）。00 号文档 4.1 写了"字号"也是变量：只有界面上反复用的两个（13、13.5 px）做成变量，16 px 正文在 `reading.css`、标题字号由用到它们的 Step 定（P2 起）。
- **753 下拉的箭头用两道渐变画成描边的折线**，不加外层元素：`NativeSelect` 照旧是一个 `<select>`（调用处的 `className`、外层的 `max-w-full min-w-0` 不变），箭头是 `background-image` 的两道四停点 `linear-gradient`，各画一条 1.2 px 的斜线（初稿 1.6 px，比样稿粗大，S2 照审查改细，4.2 的"照实"），拼成 ∨，颜色取 `--faint`（样稿的 `cd` 图标），随主题变；不用写死颜色的 SVG 数据地址。S2 的审查 B 在三种引擎、1x 与 2x 下对照过，与样稿一致。
- **754 标签页仍是下划线式**，只换颜色与间距：当前项是 `--foreground` 的 2 px 下划线与字（不用琥珀：琥珀只给勾选、当前页与焦点）。初稿的分段式在暗色下当前项比轨道还暗（`#212226` 对 `#25262a`），英文 375 px 下三个标签折成两行时 `rounded-full` 的轨道包成大胶囊（审查 B 的 M2、M3）；下划线式折行也整齐。
- **755 `--ring` 取 `var(--accent)`**：`:root` 与 `.dark` 都写 `--ring: var(--accent);`，琥珀只有一个来源。初稿各写十六进制，三个故事量的都是 `--ring`，调淡 `--accent` 时没有一个测试变红，00 号文档 4.8 要求的变异落空（审查 A 的 I1、C 的 I2）。
- **756 tailwind-merge 登记新名字**（`W/lib/cn.ts`）：`extendTailwindMerge({ extend: { theme: { radius: ["row", "control", "popover", "card", "dialog"], shadow: ["popover", "dialog"], text: ["compact", "row"] } } })`。不登记时调用处写的 `rounded-full` 会被组件里的 `rounded-row` 按字母序压过，`text-row` 会被当成颜色、与 `text-muted-foreground` 互相吃掉（审查 A 的 N5、B 的 M7、C 的 M7）。
- **757 描边按钮的悬停叠一层，不换掉自己的底**：`@utility hover-layer { background-image: linear-gradient(var(--hover), var(--hover)); }`，描边按钮写 `hover:hover-layer`。半透明的 `--hover` 直接当底色时，暗色下 `hover:bg-hover`（优先级 (0,2,0)）顶掉不透明的 `dark:bg-surface-2`（经 `:where()`，(0,1,0)），透出按钮下面的底，悬停反而变暗（审查 B 的 M1，Chromium 实量）。名字不以 `bg-` 开头，tailwind-merge 不把它与 `bg-surface` 当成一组。没有自己的底的（幽灵按钮、导航行、菜单项）照旧 `hover:bg-hover`。
- **758 暗色弹出层的底照样稿**：`--popover` 暗色 `#25262a`（palette.html:169），比对话框的面 `#212226` 亮一档；亮色照旧同 `--surface`。00 号文档 4.1 的表写的是同值，样稿的"色值照它"（00 号文档头的表）优先，2.1 第 4 条随之改表。
- **759 输入框与下拉的底是 `--surface-2`，聚焦时 `--surface`**：照 00 号文档 4.1 的表（"`--surface-2`……输入框"）与两份样稿（palette.html:164、settings.html:70-72）。初稿写的 `bg-surface` 与 P3 的卡片同色，只剩一条边框（审查 B 的 M4）。
- **760 菜单项的环只在键盘操作时出现**：新模块 `W/app/input-modality.ts` 记下最近一次输入是键盘还是指针（`<html data-modality="keyboard|pointer">`），菜单项的内嵌环挂在 `in-data-[modality=keyboard]:` 之下。Radix 在指针移过时就把焦点给菜单项，`focus-visible` 在鼠标打开的菜单里也匹配（审查 B 的 Q2 的探针），所以只换成 `focus-visible:` 压不掉；00 号文档 4.7 的"焦点只在键盘操作时出现"与样稿（高亮项只有底色）都要的是这个。快速切换的选项与补全的选中项照旧总带环：那两处的选中由键盘移动，鼠标移过不改选中。
- **761 本机门禁的写法**（取舍 746 在 P1 的落实，7.3）：
  - **开工之前**装好浏览器、在没改的 `m13-ui-refresh` 上跑一遍作基线（第 6 节第 0 步），以后红了分得清是机器还是 Step；
  - **合并之前**带 `CI=1` 跑网页一侧的全部与 `make e2e`：`CI=1` 让留着的 `test.only` 在 Playwright 与 vitest 里都红，与 CI 一样；
  - **Go 一侧不重跑**：Step 在 Go 的测试读到的路径（`server`、`api`、`deploy`、`.github`、`testdata`、`e2e/fixtures/db.ts` 与全部 `*.go`）上没有差异（7.3 的断言），Go 的树与 `main` 上 CI 绿过的相同；
  - **CI 只补在最后推上去的头**：负责人推送时，非 `main` 分支上新的推送会取消较早的运行（`ci.yml:10-12`），只有最后一个头有 CI；每个合并的头另有本机门禁的记录。没有重现的 CI 项在 7.3 列出。
- **763–765**（S1 的实施与审查之后，全文在[S1 的审查记录](reviews/P1-S1-tokens-theme-review.md)）：763 下拉在 S1 就改成 `bg-surface-2`，两处原来的 `bg-muted` 用满色的 `bg-surface-2`；764 PG15 [webkit] 记为本机的第三个已知失败（环境）；765 守约定的测试从严：测试 5 读全部字符串字面量，测试 4 的色值在任何地方都算，误报响亮、漏掉静默。
- **766–768**（S2 的实施与审查之后，全文在[S2 的审查记录](reviews/P1-S2-components-review.md)）：766 亮色 `--accent` 深一档为 `#c2871f`，Chromium 给勾选的复选框与单选画白的勾与心，与样稿同（`#c58a22` 对白不到 3:1，画成深色）；767 单选的方形焦点框接受（原生控件做不圆）；768 控件与菜单项在 `outline-none` 旁加 `focus-visible:outline-hidden`，强制色模式去掉环时焦点仍看得见。
- **762 暗色遮罩 45%**：00 号文档 4.3 写"遮罩更淡"，原来是 `bg-black/50`；亮色 32%、暗色 45%（初稿暗色 55%，比原来重，审查 B 的 N8）。

## 3. 服务端

无。

## 4. 网页

### 4.1 S1 token 与主题

#### 4.1.1 文件

| 文件 | 改什么 |
|---|---|
| `W/styles.css` | 4.1.2 的全部变量、映射、`hover-layer` 与基础层 |
| `W/reading/reading.css:12-18` | 只改 `--nw-mark-background` 的亮暗两个值（4.1.6） |
| 10 个用 `accent` 的文件与 3 处 `var(--accent…)` | 4.1.3 |
| 7 个用 `ring-[3px]` 的文件 | 4.1.4 |
| `W/components/ui/button.tsx` | 危险按钮、描边按钮（4.1.5） |
| `W/components/ui/dialog.tsx:12,15`、`alert-dialog.tsx:16,19` | 面与遮罩（4.1.6） |
| `W/pages/notebook/upgrade-page.tsx:659`、`upgrade-banner.tsx:39`、`transfer-report.tsx:236` | `bg-muted`（后两处 `bg-muted/50`）→ `bg-surface-2`（4.1.6） |
| `W/components/ui/native-select.tsx:16` | `bg-background` → `bg-surface-2`（4.1.6，决定 763） |
| `W/lib/cn.ts`、`W/lib/cn.test.ts`（新） | 取舍 756 |
| `W/app/confirm-dialog.test.tsx:88`、`W/pages/notebook/upgrade-page.test.tsx:1188,1422` | 认危险按钮的类名改成 `text-destructive`，按词认（4.1.5） |
| `W/components/ui/button.test.tsx`（新） | 描边按钮的一格（4.1.5） |
| `W/design-tokens.test.ts`（新） | 4.1.7 |
| `e2e/stories/page/pg4-delete-subtree.spec.ts`、`e2e/stories/search/se1-search.spec.ts`、`e2e/stories/links/l5-completion.spec.ts` | 门槛与注释（4.1.8） |
| `docs/v0.1/v0.1-design.md` | 8.2（Step 的文档提交里改） |

#### 4.1.2 `W/styles.css` 的新内容

颜色的值照 00 号文档 4.1 的表（与 2.1 第 4 条的修订），写十六进制；半透明的写 `rgb(r g b / a%)`。

```css
@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-sidebar: var(--sidebar);
  --color-surface: var(--surface);
  --color-surface-2: var(--surface-2);
  --color-popover: var(--popover);
  --color-popover-foreground: var(--popover-foreground);
  --color-hover: var(--hover);
  --color-active: var(--active);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-muted-foreground: var(--muted-foreground);
  --color-faint: var(--faint);
  --color-accent: var(--accent);
  --color-accent-soft: var(--accent-soft);
  --color-destructive: var(--destructive);
  --color-border: var(--border);
  --color-border-strong: var(--border-strong);
  --color-ring: var(--ring);
  --color-overlay: var(--overlay);
  --radius-md: calc(var(--radius) - 2px);
  --radius-lg: var(--radius);
  --shadow-popover: var(--elevation-popover);
  --shadow-dialog: var(--elevation-dialog);
}

@theme {
  --radius-row: 7px;
  --radius-control: 9px;
  --radius-popover: 12px;
  --radius-card: 14px;
  --radius-dialog: 16px;
  --text-compact: 13px;
  --text-row: 13.5px;
  --font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Noto Sans CJK SC", system-ui, sans-serif;
  --font-mono: "SF Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

/* A hover over what has a background of its own: layered on it, not in its place (M13/P1/S1, decision 757). */
@utility hover-layer {
  background-image: linear-gradient(var(--hover), var(--hover));
}
```

`:root`（亮）与 `.dark`（暗）的值：

| 变量 | 亮 | 暗 |
|---|---|---|
| `--background` | `#ffffff` | `#1a1b1e` |
| `--foreground` | `#18191c` | `#e8e8ea` |
| `--sidebar` | `#f5f5f6` | `#141517` |
| `--surface` | `#ffffff` | `#212226` |
| `--popover` | `#ffffff` | `#25262a` |
| `--popover-foreground` | `#18191c` | `#e8e8ea` |
| `--surface-2` | `#f4f4f5` | `#25262a` |
| `--hover` | `rgb(15 16 20 / 4.5%)` | `rgb(255 255 255 / 4.5%)` |
| `--active` | `rgb(15 16 20 / 7.5%)` | `rgb(255 255 255 / 8%)` |
| `--border` | `#e7e7ea` | `#2b2c31` |
| `--border-strong` | `#d4d4d9` | `#3a3b41` |
| `--muted-foreground` | `#5f6168` | `#a4a5ac` |
| `--faint` | `#91939b` | `#74767e` |
| `--primary` | `#18191c` | `#ececee` |
| `--primary-foreground` | `#ffffff` | `#18191c` |
| `--accent` | `#c2871f`（样稿的 `#c58a22`，决定 766） | `#d9a84e` |
| `--ring` | `var(--accent)` | `var(--accent)` |
| `--accent-soft` | `rgb(197 138 34 / 12%)` | `rgb(217 168 78 / 14%)` |
| `--destructive` | `#d64545` | `#ff7b7b` |
| `--overlay` | `rgb(16 17 20 / 32%)` | `rgb(0 0 0 / 45%)` |
| `--elevation-popover` | `0 1px 2px rgb(0 0 0 / 4%), 0 10px 28px rgb(16 17 20 / 12%)` | `0 0 0 1px rgb(255 255 255 / 6%), 0 14px 36px rgb(0 0 0 / 50%)` |
| `--elevation-dialog` | `0 1px 2px rgb(0 0 0 / 4%), 0 24px 64px rgb(16 17 20 / 18%)` | `0 0 0 1px rgb(255 255 255 / 6%), 0 24px 64px rgb(0 0 0 / 60%)` |

`--radius` 在 `:root` 里改为 `0.5rem`。`--accent-foreground` 删掉（00 号文档决定 745）。`color-scheme` 照旧。基础层的 `:focus-visible` 规则照旧（2 px 实色外框、偏移 2 px，颜色随 `--ring` 成琥珀），注释的"3:1 at least…"改成"solid amber, 2px, shown from the keyboard only (M13/P1/S1; 00-M13 4.7)"。`--elevation-dialog` 是本文定的值（00 号文档只写了弹出层的两层）：对话框比菜单离页面远，远处一层加大。

`--ring: var(--accent)` 在 `<html>` 上算出（`.dark` 也在 `<html>` 上），后代继承算好的值，所以两种主题各自取到自己的琥珀。

#### 4.1.3 `accent` 按意思分开改

| 位置 | 现在 | 改成 |
|---|---|---|
| `W/components/nav-item.tsx:17` | `hover:bg-accent`；当前 `bg-accent font-medium` | 不是当前的 `text-muted-foreground hover:bg-hover`；当前的 `bg-active font-medium`，不加悬停（`hover:bg-hover` 的 (0,2,0) 会压过 `bg-active`，悬停时反而变淡，审查 A 的 M2） |
| `W/components/ui/dropdown-menu.tsx:30,42` | `focus:bg-accent focus:text-accent-foreground` | `focus:bg-active focus:text-foreground`；内嵌的环在 S1 照旧（`focus:ring-2 focus:ring-ring focus:ring-inset`），S2 照 4.2.6 改成只在键盘操作时（S1 里还没有 `data-modality`，照 4.2.6 写会让 l5 抛"no focus indicator"） |
| `W/components/ui/button.tsx:12` | `hover:bg-accent hover:text-accent-foreground` | 4.1.5 的描边按钮 |
| `W/components/ui/button.tsx:13` | 同上 | `hover:bg-hover hover:text-foreground` |
| `W/pages/page/attachment-row.tsx:158`、`W/pages/notebook/page-tree-item.tsx:276,330` | `hover:bg-accent` | `hover:bg-hover` |
| `W/pages/page/subpage-list.tsx:57`、`W/pages/workspace/workspace-home.tsx:46`、`W/pages/notebook/page-tree.tsx:185,193` | `hover:bg-accent` | `hover:bg-hover` |
| `W/pages/notebook/page-tree-item.tsx:290-291` | `hover:bg-accent`；当前 `bg-accent font-medium` | 同 `nav-item`：不是当前的 `hover:bg-hover`，当前的 `bg-active font-medium`（当前页的 `--accent-soft` 底与琥珀图标是 P2/S2 的树） |
| `W/pages/notebook/quick-switch.tsx:173` | `bg-accent ring-2 ring-ring ring-inset` | `bg-active ring-2 ring-ring ring-inset` |
| `W/editor/text-diff.tsx:376` | `bg-background hover:bg-accent hover:text-accent-foreground` | 同 4.1.5 的描边按钮（它照抄描边按钮的样子，`:372-374` 的注释） |
| `W/editor/text-diff.tsx:417` | `background: "var(--accent)"` | `"var(--surface-2)"` |
| `W/editor/theme.ts:34` | `backgroundColor: "var(--accent)"` | `"var(--accent-soft)"` |
| `W/editor/loaded/link-completion.ts:131-132` | `var(--accent)`、`var(--accent-foreground)` | `var(--active)`、`var(--foreground)` |

改完之后 `grep -rn "accent" W/ --include='*.ts' --include='*.tsx' --include='*.css'` 剩下的：`styles.css` 的定义与映射、`theme.ts:34` 的 `--accent-soft`、`slug.ts:37` 与 `password-length.ts:3` 的英文单词（"accents"）、`nav-item.tsx:8` 的注释（改成"the active background"）、`design-tokens.test.ts` 自己。逐条写进 S1 的记录。

#### 4.1.4 控件的焦点环

`focus-visible:ring-[3px]` 改成 `focus-visible:ring-2`（Tailwind 的 `ring-2` 是零偏移、零模糊、2 px 的 `box-shadow`，`contrast.ts:117-129` 认它，读出 `ring 2px`）：`W/components/ui/button.tsx:7`、`tabs.tsx:21,32`、`native-select.tsx:17`、`input.tsx:10`、`W/pages/workspace/workspace-home.tsx:46`、`W/pages/notebook/transfer-job-row.tsx:116`、`W/editor/text-diff.tsx:376`。环的颜色是 `ring-ring`，随 `--ring` 成琥珀。输入框与下拉的 `focus-visible:border-ring` 照旧（边框与环同色）。se1 对搜索框、"搜索"按钮与范围下拉加断言 `indicator` 是 `ring 2px`（4.1.8），守住 3 px → 2 px（审查 C 的 N10）。

#### 4.1.5 危险按钮与描边按钮

`button.tsx:12` 与 `:14-15`：

```
outline:     "border border-border bg-surface hover:hover-layer dark:bg-surface-2"
destructive: "border border-destructive/35 bg-transparent text-destructive hover:bg-destructive/8 dark:hover:bg-destructive/10"
```

- 描边按钮的底由 `bg-background` 换成 `bg-surface`（暗色 `surface-2`，样稿 palette.html:100-102），悬停叠一层（取舍 757）；字色照旧继承 `--foreground`（去掉 `hover:text-accent-foreground`）。
- 危险按钮去掉 `focus-visible:ring-destructive`（取舍 749），焦点环与别的按钮一样是琥珀。静止时红字对它下面的面；悬停时红字对 8%（暗 10%）的红底。预期（由十六进制算出，实量在 4.1.8）：亮约 4.38（静止）、3.93（悬停）；暗在对话框的面 `#212226` 上约 6.33（静止）、5.40（悬停）。

三处 vitest 改认 `text-destructive`，都按词认（`classList.contains`）：`confirm-dialog.test.tsx:88` 的 `[undefined, "bg-destructive"]` 改成 `[undefined, "text-destructive"]`；`upgrade-page.test.tsx:1188` 的 `.className).not.toMatch(/bg-destructive/)` 改成 `.classList.contains("text-destructive")).toBe(false)`；`:1422` 的 `.className.includes("bg-destructive")` 改成 `.classList.contains("text-destructive")`。子串的写法会被 `hover:bg-destructive/8` 骗过（审查 A 的 N7）。默认按钮（`bg-primary`）不带 `text-destructive`，三处的意思不变。

S1 新建 `W/components/ui/button.test.tsx`，先放一格 `"the outline button layers its hover on its own background"`：描边按钮的 `classList` 含 `bg-surface`、`hover:hover-layer`，不含 `hover:bg-hover`（暗色下改回 `hover:bg-hover` 只暗一级，截图看不出，审查 C 的修订核对第 1 轮 M3）。S2 往里加格（7.1）。

#### 4.1.6 面、遮罩、高亮与 `bg-muted`

- `W/components/ui/dialog.tsx:12`、`alert-dialog.tsx:16`：`bg-black/50` → `bg-overlay`。
- 两个内容的 `bg-background` → `bg-surface`（`dialog.tsx:15`、`alert-dialog.tsx:19`）。菜单的 `bg-popover` 照旧（取舍 758 的值）。
- `W/reading/reading.css:13` 的 `--nw-mark-background` 亮改成 `rgba(245, 196, 81, 0.38)`，`:17` 暗改成 `rgba(217, 168, 78, 0.28)`（00 号文档 4.1 照 reading.html）。正文的字对它：亮约 14.6、暗约 8.0。
- `bg-muted` 的三处改成 `bg-surface-2`（00 号文档 4.1："次一级的面：行内代码"）；其中两处原来是 `bg-muted/50`，也用满色，不带 `/50`（决定 763）。
- `NativeSelect` 的 `bg-background` 改成 `bg-surface-2`（决定 763：暗色对话框里留着 `bg-background` 比面暗，理由同 748 对描边按钮的）；聚焦时换 `bg-surface` 照旧在 S2（4.2.4）。

#### 4.1.7 守住约定的 vitest

`W/design-tokens.test.ts`（取舍 751）：

- **读法**：`styles.css` 经 `readFileSync(new FileURL("./styles.css", import.meta.url), "utf8")`，`FileURL` 是 `import { URL as FileURL } from "node:url"`（`W/reading/patch.test.ts:1-2,14` 的先例：jsdom 环境里全局的 `URL` 是 jsdom 的，Vite 还会把 `new URL(…, import.meta.url)` 改写成 `http://localhost:3000/@fs/…`，`readFileSync` 抛 `ERR_INVALID_URL_SCHEME`，审查 A、C 的修订核对第 1 轮）；源码经 `import.meta.glob("./**/*.{ts,tsx}", { query: "?raw", import: "default", eager: true })`，去掉 `*.test.*` 与 `test/` 下的文件。先断言：`styles.css` 的长度大于 1000；去掉测试之后的源码多于 250 个文件（今天 299 个），且含 `./components/ui/button.tsx`。
- **取块**：只取顶层的 `:root { … }` 与 `.dark { … }`（不在任何 `@media`、`@layer` 里的；S2 会在 `@media (prefers-reduced-motion)` 里加一个 `:root`，审查 A 的 M1、C 的 M4）。
- 每个断言失败时列出文件与那一行。

五个测试：

1. `"every colour of the light theme is the dark theme's too"`：两块里 `--名字` 的集合相等（去掉 `--radius`，与主题无关）；先断言亮色的集合含 `--background`、`--accent`、`--ring`、`--sidebar`，且不少于 20 个。旧的 `:root` 去掉 `--radius` 只有 12 个名字、没有 `--sidebar`，所以没改的树上测试 1 因这条前置断言而红，这是预期的。
2. `"no accent-foreground is left"`：源码与 `styles.css` 里没有 `accent-foreground`。
3. `"amber is no fill"`：源码里没有匹配 `/\bbg-accent(?![-\w])/` 的类（`bg-accent-soft` 可以），也没有把 `var(--accent)` 当底色的样式对象（`/background(?:Color)?["']?\s*:\s*["'\x60]var\(--accent\)/`，审查 A 的 M7）。
4. `"components use the theme's colours, not Tailwind's palette nor written values"`：源码里没有匹配 `/\b(?:bg|text|border|ring|outline|fill|stroke|from|to|via|decoration|divide|placeholder|caret|accent|shadow)-(?:black|white|slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)(?:-\d{2,3})?(?:\/\d+)?\b/` 的类，也没有任意值里的色值（`/-\[(?:#|rgb|hsl|oklch|oklab|color-mix)/`）；样式对象里的色值（`#` 加 3、6、8 位十六进制，`rgb(`、`hsl(`、`oklch(`）只许出现在 13.2 第 32 条的例外里：`W/editor/theme.ts`（P4/S3 改它的链接色与匹配色）与 `W/reading/diagrams.ts`（mermaid）。
5. `"every colour class names a colour of the theme"`：取词的方法——取源码里的字符串字面量（`import`、`export … from` 的模块路径不取，例如 `"./text-diff"`、`"../../lib/text-ranges"`），按空白切开，去掉方括号与圆括号之外的 `:` 之前的变体与 `/透明度`，从词头匹配 `(?:bg|text|border|ring|outline|fill|stroke|divide|decoration|caret|placeholder)-NAME`；任意值（`-[…]`、`-(…)`，如 `bg-(--nw-mark-background)`）不取。去掉 Tailwind 的非颜色值（一张写在测试里的表：`text-sm`、`text-left`、`border-b`、`ring-2`、`ring-inset`、`outline-none`、`bg-transparent`……由实施者从现有代码里收齐）与 `transparent`、`current`、`inherit`；`text-NAME` 另接受 `styles.css` 的 `@theme` 里声明的 `--text-NAME`（从同一份原文读出：S2 起用的 `text-compact`、`text-row` 是字号，审查 A、B、C 的修订核对第 1 轮）。剩下的 NAME 必须是 `@theme inline` 里某个 `--color-NAME`。它抓住第 1 节那三处 `bg-muted` 这样的、不产出 CSS 的名字（审查 A 的 M6、C 的 M8）。

在没改的树上：五个测试都失败（测试 1 是前置断言，其余是 `accent-foreground`、`bg-accent`、`bg-black/50`、`text-white`、`bg-muted`）。变异 7 在改完 `styles.css` 之后做（旧的 `.dark` 里本来就没有 `--sidebar`）。

**照实**（S1 的实施与审查，[S1 的审查记录](reviews/P1-S1-tokens-theme-review.md)）：

- 取字符串字面量用 Vite 8 公开导出的 `parseSync`（oxc 的解析器）：仓库的 TypeScript 7.0.2 是原生的版本，没有 JS 的编译器接口；测试文件与 `testing/` 下的假件在 `import.meta.glob` 里以否定的模式去掉。测试 3、4、5 先断言遍历读到了东西（`dialog.tsx` 的 `bg-overlay`、`button.tsx` 的 `bg-primary`）。
- 测试 1 另断言 `@theme inline` 引用的每个 `var()` 都在 `:root` 里声明。
- 测试 2、3 也读 `reading.css`；测试 3 认类名（`bg-accent`、`bg-ring`、`bg-(--accent)`、`bg-[var(--accent)]`）与样式（驼峰、短横线、带引号的键、渐变与 `background-image`，值里有 `var(--accent)`、`var(--color-accent)` 或 `var(--ring)`）两类写法，查源码的字面量与两张样式表（去掉注释）。
- 测试 4：调色板另有 mauve、mist、olive、taupe；色值（3、4、6、8 位十六进制与 `rgba?`、`hsla?`、`hwb`、`lab`、`lch`、`oklab`、`oklch`、`color`、`color-mix(`）在不在词中间的任何地方都算，任意值查整个 `-[…]`，只含 `var()` 的 `color-mix(` 豁免；颜色名不查。
- 测试 5 读全部字符串字面量，只跳过模块路径、`i18n/messages` 与对象的键（决定 765）；剥掉透明度（含 `/[0.5]`）、边（`border-t-` 等）与偏移（`ring-offset-`、`outline-offset-`）；前缀另有 `from-`、`via-`、`to-`、`shadow-`，`shadow-X`、`text-X` 在主题里有同名的变量时放行；非颜色的表 14 项。
- `styles.css` 加 `@source not "./**/*.test.{ts,tsx}"`：测试文件不产出 CSS（审查 A、C 的 N）。
- `button.test.tsx` 三格：描边按钮（含 `dark:bg-surface-2`）、每个变体的环是 `ring-ring`、对比视图的 `buttonClass` 照描边按钮的样子。`cn.test.ts` 另有一格从 `styles.css` 读出全部登记的名字逐个验。

#### 4.1.8 对比度的守卫

三个故事的断言不动形状（量哪个元素、什么状态、`outline solid` 的断言照旧），门槛照取舍 750 换掉；se1 另加 `ring 2px` 的断言（4.1.4）。下表"预期"由 4.1.2 的十六进制算出；S1 在 Chromium 上实量（临时打印 `readings`，不提交），下限 = 实量减 0.1，写进故事与记录。

| 故事 | 量的 | 亮（预期） | 暗（预期） | 下限 |
|---|---|---|---|---|
| pg4:191-205 | 对话框"删除"的红字，静止 | 4.38 | 6.33 | 实量 − 0.1 |
| | 同上，悬停 | 3.93 | 5.40 | 实量 − 0.1 |
| se1:279-308 | 摘要里的命中字（正文） | 14.6 | 8.0 | 7 |
| | 搜索框、"搜索"按钮、范围下拉、命中链接的焦点 | 2.99 | 7.92 | 实量 − 0.1 |
| se1:322-332 | 标题里的命中字（正文） | 14.6 | 8.0 | 7 |
| se1:369-380 | 无效的搜索框的焦点（`ring-destructive`） | 4.38 | 6.86 | 实量 − 0.1 |
| l5:127-140 | 补全选中项的说明字（`--muted-foreground` 在 `--active` 上，下面是 `--popover`） | 5.28 | 4.84 | 实量 − 0.1 |
| | 编辑器的聚焦框 | 2.99 | 7.92 | 实量 − 0.1 |
| | 补全的选中项（内嵌的环在 `--active` 上） | 2.55 | 5.47 | 实量 − 0.1 |
| l5:165-183 | 阅读视图链接的焦点 | 2.99 | 7.92 | 实量 − 0.1 |
| | 页面菜单当前项的指示（键盘打开，取舍 760） | 2.55 | 5.47 | 实量 − 0.1 |

写法：每个故事在量之前定义一个常量表，例如

```ts
// Floors at what M13/P1/S1 measured, less 0.1: no colour is to fade by accident (00-M13 4.7).
const floors = { light: { rest: 4.28, hover: 3.83 }, dark: { rest: 6.23, hover: 5.3 } } as const;
```

断言 `toBeGreaterThanOrEqual(floors[scheme].rest)`，失败信息照旧带 `JSON.stringify` 的读数。pg4:191-192 的注释（"3.45:1 under white"）、se1:279 的"3:1 at least"与 l5:127-128 的同类说法改写成新的事实。量出的值与预期差 0.05 以上时，查清是哪里与 4.1.2 不同再定下限。

#### 4.1.9 tailwind-merge 的登记

`W/lib/cn.ts` 照取舍 756 用 `extendTailwindMerge` 建一个 `twMerge`，`cn` 照旧是 `merge(clsx(inputs))`。`W/lib/cn.test.ts`：

- `"a radius of the theme gives way to the caller's"`：`cn("rounded-row", "rounded-full")` 是 `"rounded-full"`；`cn("rounded-control", "rounded-lg")` 是 `"rounded-lg"`；
- `"a shadow of the theme gives way to the caller's"`：`cn("shadow-popover", "shadow-none")` 是 `"shadow-none"`；
- `"a size of the theme is a size, not a colour"`：`cn("text-row", "text-muted-foreground")` 两个都留；`cn("text-sm", "text-compact")` 是 `"text-compact"`。

### 4.2 S2 基础组件

#### 4.2.1 文件

| 文件 | 改什么 |
|---|---|
| `W/components/ui/button.tsx` | 4.2.2 |
| `W/components/ui/input.tsx`、`label.tsx`、`W/components/form-field.tsx`、`W/components/ui/alert.tsx` | 4.2.3 |
| `W/components/ui/native-select.tsx`、`W/styles.css`、`W/pages/settings/create-token-dialog.tsx:114-127` | 4.2.4 |
| `W/styles.css` | 4.2.5 单选与复选；4.2.9 动画 |
| `W/components/ui/dropdown-menu.tsx`、`W/app/input-modality.ts`（新）、`W/main.tsx`、`W/pages/page/attachment-row.tsx:192`、`W/pages/notebook/page-tree-item.tsx:476` | 4.2.6 |
| `W/components/ui/dialog.tsx`、`alert-dialog.tsx` | 4.2.7 |
| `W/components/ui/tabs.tsx` | 4.2.8 |
| `W/components/ui/*.test.tsx`（新：`dropdown-menu`、`dialog`；`button.test.tsx`（S1 建）与 `native-select.test.tsx` 加格）、`W/app/input-modality.test.ts`（新） | 7.1 |
| `e2e/stories/links/l5-completion.spec.ts` | 关上菜单之后一帧之内没有 `menu`（7.2 第 13 条） |
| `W/design-tokens.test.ts` | 不改：S2 加的 `@media` 里的 `:root` 不在测试 1 取的块里（4.1.7） |

下面写类名的地方写的是完整的类名；"照旧"的部分也照抄（审查 B 的 N1：丢了 `outline-none`，全局的琥珀外框会叠在菜单项的内嵌环外面，`contrast.ts` 先量外框，l5 照绿也抓不到）。

#### 4.2.2 按钮

`buttonVariants`：

- 基础：`inline-flex shrink-0 items-center justify-center gap-1.5 rounded-lg text-sm font-medium whitespace-nowrap transition-colors duration-150 outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 aria-disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 [&_svg]:shrink-0`（`rounded-lg` 现在是 8 px：表单底部的按钮；`gap-1.5` 是样稿的 6 px）。
- `variant`：`default`（主）`bg-primary text-primary-foreground hover:bg-primary/90`；`outline`、`destructive` 照 4.1.5；`ghost` `text-muted-foreground hover:bg-hover hover:text-foreground`（样稿的幽灵按钮是次要的字；14 处用它，没有一处被对比度故事量到，审查 B 核对过）。
- `size`：`default` `h-9 px-4`；`icon` `size-9`。30 px 与胶囊形在 P2/S1（2.1 第 2 条）。
- `aria-disabled` 的按钮照旧在 Tab 的顺序里（13.2 第 26 条）：类名里没有 `aria-disabled:pointer-events-none`。

#### 4.2.3 输入框、标签、表单字段、提示条

- `Input`：`h-9 w-full min-w-0 rounded-control border border-border bg-surface-2 px-3 py-1 text-base text-foreground outline-none transition-[color,background-color,border-color,box-shadow] duration-150 placeholder:text-faint focus-visible:border-ring focus-visible:bg-surface focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive md:text-sm`（取舍 759；去掉 `shadow-xs`；`text-base md:text-sm` 照旧：iOS 的 Safari 在小于 16 px 的输入框聚焦时放大页面）。
- `Label`：`text-compact leading-none font-medium select-none`。
- `FormField`：外层 `space-y-1.5`（原 `space-y-2`）；说明 `text-compact text-muted-foreground`；问题 `text-compact text-destructive`；保留的提示（`kept`）`text-compact text-muted-foreground`；后缀照旧与输入框同大，`text-base text-muted-foreground md:text-sm`（它是 `aria-hidden` 的扩展名，读起来是名字的一部分，审查 B 的修订核对第 2 轮 N1）。说明、问题与保留的提示三种小字同一个字号：提示变成问题时字号不跳（审查 B 的修订核对第 1 轮 N3）。
- `Alert`：`rounded-lg border border-destructive/35 bg-destructive/5 px-4 py-3 text-sm text-destructive`（`role="alert"` 照旧）。

#### 4.2.4 下拉

`NativeSelect` 加一个 `nw-select` 类，`W/styles.css` 的 `@layer components` 里（取舍 753，审查 B 的 M5 给的画法）：

```css
.nw-select {
  appearance: none;
  padding-inline-end: 2rem;
  background-image:
    linear-gradient(45deg, transparent calc(50% - 0.6px), var(--faint) calc(50% - 0.6px) calc(50% + 0.6px), transparent calc(50% + 0.6px)),
    linear-gradient(135deg, transparent calc(50% - 0.6px), var(--faint) calc(50% - 0.6px) calc(50% + 0.6px), transparent calc(50% + 0.6px));
  background-position:
    calc(100% - 16px) 55%,
    calc(100% - 12px) 55%;
  background-size: 5px 5px;
  background-repeat: no-repeat;
}
@media (forced-colors: active) {
  .nw-select {
    appearance: auto;
    padding-inline-end: 0.5rem;
    background-image: none;
  }
}
```

（强制色模式下的右边距写明 `0.5rem`，即原来的 `px-2`；`revert-layer` 会退到 preflight 的 0，审查 B 的 N6。）

类名其余：`h-9 max-w-full rounded-control border border-border bg-surface-2 pl-3 text-base text-foreground outline-none transition-[color,background-color,border-color,box-shadow] duration-150 contain-paint focus-visible:border-ring focus-visible:bg-surface focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive aria-invalid:ring-destructive md:text-sm`（`contain-paint` 照旧，13.2 第 31 条；`text-base md:text-sm` 同输入框；无效的样子与输入框同，5 个带 `aria-invalid` 的调用处随之看得出，审查 B 的 M8）。`bg-*` 只设 `background-color`，与箭头的 `background-image` 不互相覆盖。

令牌期限（`create-token-dialog.tsx:114-127`）的裸 `<select>` 换成 `NativeSelect`：`id`、`aria-describedby`、`aria-invalid`、`value`、`onChange` 与 `className="w-full"` 照旧（它不读东西，不用 `useChoice`）。`W/pages/settings/tokens-page.test.tsx:68` 以 `getByLabelText("Expires")` 找它，照旧通过；`native-select.test.tsx` 加一格钉住 `aria-invalid` 落到 `<select>` 上（7.1）。

#### 4.2.5 单选与复选

`W/styles.css` 的基础层加：

```css
input[type="radio"],
input[type="checkbox"] {
  accent-color: var(--accent);
}
```

焦点照全局的 `:focus-visible`（外框加 2 px 偏移，00 号文档 4.7）。强制色模式下浏览器忽略 `accent-color`，控件照旧可见。阅读视图的任务项：可写时启用，勾是琥珀；读者、版本页与回收站的预览里它们是 `disabled` 的，Chromium 不给禁用的复选框用 `accent-color`，勾是灰的（审查 B 的 M6 截图）。已勾的禁用项怎样显示交给 P4/S2（第 9 节）。

#### 4.2.6 菜单

- `DropdownMenuContent`：`z-50 min-w-[220px] origin-(--radix-dropdown-menu-content-transform-origin) overflow-hidden rounded-popover border border-border bg-popover p-1.25 text-popover-foreground shadow-popover data-[state=open]:animate-pop-in`（`sideOffset` 照旧 4；样稿的宽 220 px、内边距 5 px；从触发它的那一侧弹出，审查 B 的 N5）。
- `DropdownMenuItem`：`relative flex h-8 cursor-default items-center gap-2.5 rounded-row px-2.5 text-row outline-none select-none focus:bg-active focus:text-foreground focus:ring-ring focus:ring-inset in-data-[modality=keyboard]:focus:ring-2 data-disabled:pointer-events-none data-disabled:opacity-50 [&>svg]:size-4 [&>svg]:shrink-0 [&>svg]:text-muted-foreground`（只管直属的图标 `[&>svg]`：`[&_svg]` 会把单选项里勾的颜色也压成灰，审查 B 的 N3）。
- `DropdownMenuRadioItem`：同上，`px-2.5` 换成 `pr-2.5 pl-8`，勾照旧在左边的 `span` 里，写明 `<Check className="size-3.5" />`（`[&>svg]` 管不到 `span` 里的勾，不写尺寸时它是 lucide 默认的 24 px，审查 B 的修订核对第 1 轮 N2）。
- `DropdownMenuItem` 加属性 `tone?: "destructive"`，从 props 里解构掉、不落到 DOM 上：`text-destructive focus:text-destructive [&>svg]:text-destructive`。用在两处"删除"：`attachment-row.tsx:192`、`page-tree-item.tsx:476`。页面菜单（`W/pages/page/page-menu.tsx:51-52`）没有删除项。
- `DropdownMenuSeparator`：`mx-1.5 my-1 h-px bg-border`（样稿 `margin: 5px 6px`）；`DropdownMenuLabel`：`px-2.5 py-1.5 text-xs text-faint`。
- **输入的方式**（取舍 760）：`W/app/input-modality.ts` 导出 `trackInputModality(root: HTMLElement): () => void`：在 `root.ownerDocument` 上以捕获阶段听 `keydown`（设 `root.dataset.modality = "keyboard"`）与 `pointerdown`、`pointermove`（设 `"pointer"`），值没变时不写；返回去掉监听的函数。照 WICG focus-visible 的规则，带 `metaKey`、`ctrlKey` 或 `altKey` 的按键与单独的修饰键（`Shift`、`Meta`、`Control`、`Alt`、`CapsLock`）不算键盘：用鼠标时按 Cmd+C、Shift 加点按、截图的快捷键，不该让鼠标停着的那一项出环（审查 B 的修订核对第 1 轮 M2）；Shift+Tab 的 `key` 是 `Tab`，照旧算。`W/main.tsx` 在渲染之前以 `document.documentElement` 调用它。没有输入过时不设，菜单项没有环；l5 以 `menu.focus()` 加 Enter 打开页面菜单，是键盘，环在，照旧量得到。

#### 4.2.7 对话框

`DialogContent`、`AlertDialogContent`：`fixed top-1/2 left-1/2 z-50 grid w-full max-w-[calc(100%-2rem)] -translate-x-1/2 -translate-y-1/2 gap-4 rounded-dialog border border-border bg-surface p-6 shadow-dialog data-[state=open]:animate-pop-in sm:max-w-md`（`DialogContent` 另照旧有 `grid-cols-[minmax(0,1fr)]`）；遮罩 `fixed inset-0 z-50 bg-overlay data-[state=open]:animate-fade-in`。标题 `text-lg font-semibold [overflow-wrap:anywhere]`（18 px，照旧）；说明 `text-row text-muted-foreground`。按钮行已经右对齐（17 处 `flex justify-end gap-2`），不另加组件。

#### 4.2.8 标签页

取舍 754：

- `TabsList`：`flex flex-wrap gap-x-3 border-b border-border`；
- `TabsTrigger`：`-mb-px rounded-sm border-b-2 border-transparent px-1 pt-1 pb-2 text-compact font-medium text-muted-foreground outline-none transition-colors duration-150 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring data-[state=active]:border-foreground data-[state=active]:text-foreground`；
- `TabsContent`：`rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring`（4.1.4）。

#### 4.2.9 进场动画

`W/styles.css`：

```css
@theme {
  --animate-pop-in: nw-pop-in 150ms cubic-bezier(0.2, 0.8, 0.2, 1);
  --animate-fade-in: nw-fade-in 150ms ease-out;
  @keyframes nw-pop-in {
    from { opacity: 0; scale: 0.97; }
  }
  @keyframes nw-fade-in {
    from { opacity: 0; }
  }
}
@media (prefers-reduced-motion: reduce) {
  :root { --animate-pop-in: nw-fade-in 150ms ease-out; }
}
```

- 减少动效时换的是变量：`@theme`（不是 `inline`）的工具类写成 `animation: var(--animate-pop-in)`，挂在 `data-[state=open]:` 之下的类名不是 `.animate-pop-in`，按类名覆盖管不到它。不在任何层里的 `:root` 压过 `@layer theme`（审查 B 在 Chromium 里实测：减少动效时动画名是 `nw-fade-in`）。S2 照 7.4 再做一次探针，写进记录。
- 用独立的 `scale` 属性，不用 `transform`：对话框的居中是 Tailwind 4 的 `-translate-x-1/2 -translate-y-1/2`，它写的是独立的 `translate` 属性，两者不互相覆盖（审查 B 实测中心不动）。
- **只有进场**：类只挂在 `data-[state=open]` 上。关上时 `data-state` 变成 `closed`、`animation-name` 变成 `none`，Radix 的 `Presence`（`@radix-ui/react-presence` 1.1.10）在布局阶段读到 `none`，立刻卸下（00 号文档决定 737；审查 B 读过源码）。
- 颜色与底色的过渡照 4.1"动效"取 150 ms（`duration-150` 写明）。

#### 4.2.10 三种引擎里核对下拉

00 号文档第 10 节的风险：`appearance: none` 之后 WebKit 可能开始画 `box-shadow`。S2 改完下拉之后、提交之前，在三种引擎里打开搜索页的范围下拉，截图看静止、聚焦、无效与强制色模式（Chromium 的 `emulateMedia({ forcedColors: "active" })`）；另核对 WebKit 在 `appearance: none` 之后是否仍把显示的那一项的文字算进页面宽度（审查 C 留下的探针 `select-probe.mjs` 可以照用；结论决定 7.2 第 12 条由谁守）。看的是 macOS 上的 Playwright 引擎；CI 的 Linux WebKit 画法可能不同，句子与记录都注明平台，负责人推送之后 CI 上 se1 的 `webkit-narrow` 格再核一次。WebKit 画出了环的，13.2 第 31 条那句"Playwright 的 WebKit 不画原生下拉的 `box-shadow`……"照实改（8.2 第 2 条），M12-polish 第 5 项那一小项写进"处理进展"。

**照实**（S2 的实施与审查，[S2 的审查记录](reviews/P1-S2-components-review.md)）：

- 亮色 `--accent` 是 `#c2871f`（决定 766，4.1.2 与 00 号文档 4.1 的表随之改）；se1、l5 量琥珀的亮色下限照 4.1.8 重量：2.88 → 2.99、2.45 → 2.54。
- 控件的 `outline-none` 旁另有 `focus-visible:outline-hidden`（决定 768）：4.2.2 的按钮基础、4.2.3 的输入框、4.2.4 的下拉、4.2.6 的菜单项、4.2.8 的标签页触发与内容，与对比视图的 `buttonClass`。Tailwind 4.3.3 平时写 `outline-style: none`，`contrast.ts` 照旧读环。
- 4.2.3：`Input` 另有 `file:mr-3 file:rounded-md file:border file:border-border file:bg-surface file:px-2 file:font-medium dark:file:bg-active`，"选择文件"读得出是按钮（暗色下比输入框亮一档，与描边按钮的约定同）；对比视图的失败提示（`W/editor/text-diff.tsx`，不引入应用的组件）照抄 `Alert` 的样子；令牌期限那一块的外层是 `space-y-1.5`、问题的小字是 `text-compact`，与夹着它的两个 `FormField` 同。
- 4.2.4 的箭头：上面的代码块已照实改成 `background-size: 5px 5px`、线宽 ±0.6 px、位置 `calc(100% - 16px) 55%` 与 `calc(100% - 12px) 55%`。初稿的 6 px、±0.8 px、17 px 比样稿 14 px 的图标粗大（审查 B 的 S2B-M1）；15.5 px 时两条线在尖端之前交叉、尖下留一个点，16 px 时正好相交（实施者与审查 B 在三种引擎、1x 与 2x 下比过）。
- 4.2.6：菜单项与单选项共用一个类名常量，内边距另外追加（合起来与上面的逐字相同）；两处"删除"之前有 `DropdownMenuSeparator`（样稿的 `.msep`；前面总有别的项）；`trackInputModality` 的监听另带 `passive: true`。
- 4.2.8：`TabsTrigger` 是 `rounded-t-sm`，下划线的两端不往上卷。
- 4.1.9 与取舍 756：tailwind-merge 另登记 `animate: ["pop-in", "fade-in"]`，调用处能覆盖进场动画。
- 4.2.1 写的 `design-tokens.test.ts`"不改"不成立：测试 5 的非颜色表加了 `outline-hidden`（决定 765 的"第一次碰到误报就加一行"）；新加一格断言减少动效的块把 `--animate-pop-in` 设成 `nw-fade-in`。对比视图的 `buttonClass` 跟上 4.2.2 的基础（`gap-1.5`、`rounded-lg`、`duration-150`，去掉 `py-2`）。
- 4.2.10 的结论：三种引擎（macOS）都画出 2 px 的环；WebKit 不带 `contain-paint` 时照旧把页宽撑开，7.2 第 12 条由 vitest 与 se1 的 `@narrow` 一起守。

## 5. e2e

P1 不加故事。

- **S1** 改 pg4、se1、l5 的门槛与注释（4.1.8），se1 加三处 `ring 2px` 的断言（4.1.4）；不改它们量什么。
- **S2** 只在 l5 加一句：`readingShown` 里按 Escape 之后、`expect(item).toBeHidden()`（l5:179-180）之前，`page.evaluate` 等一个 `requestAnimationFrame` 再数 `[role=menu]`，应为 0（放在 `toBeHidden` 之后，有退场动画时也是 0，审查 C 的 N2）。按钮、输入框、下拉、菜单、对话框的名称与角色都不变；定位靠角色与名称，不靠类名（`e2e/perf/assets.perf.ts:227` 的 `output.text-destructive` 是性能脚本，不在门禁里，类名照旧在）。S2 跑全部故事（它改了每一页都有的组件），经 `heavy.sh`。

## 6. 实施步骤

两个 Step 各在自己的 worktree 里，分支从 `m13-ui-refresh` 拉出。每个提交单独为绿：`make lint-web`（四个 `check.mjs`、oxlint、oxfmt、各包的 tsc）、`make knip`、`make test-web`；改到故事的提交另跑那几个故事（Chromium；pg4、se1、l5 都只在 Chromium 量对比度）。

### 6.0 开工之前（第 0 步）

- [ ] 问负责人能否下载 Playwright 1.63 的三个浏览器（`cd e2e && pnpm exec playwright install chromium firefox webkit`，从 Playwright 的 CDN 下载，约几百 MB）。不能下载时，S1 的实量与本机的 `make e2e` 都做不了：停下汇报。
- [ ] `make build`（产出 `bin/nervewiki` 与网页）。
- [ ] 基线：在没改的 `m13-ui-refresh` 上经 `heavy.sh` 跑 `CI=1 make lint-web knip test-web gen-check-web build-web` 与 `CI=1 make e2e`，结果（通过与失败的格、用时）记下，写进 S1 的记录。基线有红的，先查清是机器还是代码，再开 S1。

### 6.1 S1

- [ ] **提交 1 `web, e2e: the design tokens, the amber accent and the contrast guards at its measures (M13/P1/S1)`**：新配色让三个故事的旧门槛（4.5 与 3）不再成立，所以变量与门槛在同一个提交里，这个提交才单独为绿。
  1. 先写 `W/design-tokens.test.ts`（4.1.7）、`W/lib/cn.test.ts`（4.1.9）与 `W/components/ui/button.test.tsx` 的第一格（4.1.5），在没改的树上跑，看到 4.1.7 写的五个失败、`cn.test.ts` 的登记格失败、`button.test.tsx` 那一格失败；
  2. 改 `W/styles.css`（4.1.2）、`reading.css` 的两个值、4.1.3、4.1.4、4.1.5（含三处 vitest）、4.1.6、4.1.9；`make lint-web`、`make knip`、`make test-web` 全绿；做变异 7，看测试 1 变红，复原；`grep` 核对 4.1.3 末尾的剩余项；
  3. 在 Chromium 上跑 pg4、se1、l5，临时打印各项读数（不提交）；照 4.1.8 写下限、注释与 se1 的 `ring 2px`；三个故事再跑一遍为绿。
- [ ] 7.2 的 S1 各条变异；B 审查者的截图之前，S1 的实施者先自己截 7.4 的几张看一遍。
- [ ] **提交 2 `docs: …(M13/P1/S1)`**：8.2 第 1–5 条进 `v0.1-design.md`；S1 的审查记录；本文 10.1 与照实改的各节；00 号文档第 12、13 节（02-workflow 3.5）。

### 6.2 S2

- [ ] **提交 1 `web: the buttons, inputs and selects restyled (M13/P1/S2)`**：`button.test.tsx`、`native-select.test.tsx` 的新格先写、先红；改 4.2.2–4.2.5；改完下拉、提交之前在三种引擎里核对（4.2.10）；绿。
- [ ] **提交 2 `web, e2e: the menus, dialogs and tabs restyled, their entrance, and the menus' ring from the keyboard only (M13/P1/S2)`**：`dropdown-menu.test.tsx`、`dialog.test.tsx`、`input-modality.test.ts` 先写、先红；改 4.2.6–4.2.9 与 l5 的一句；跑 l5；绿。
- [ ] 7.2 的 S2 各条变异；减少动效的探针（7.4）。
- [ ] **提交 3 `docs: …(M13/P1/S2)`**：审查记录；本文 10.2；8.2 第 6 条；M12-polish 的"处理进展"一行（第 2 节"移交"）；00 号文档第 12、13 节；Phase 完成的收尾项（8.1 第 6 条）。

## 7. 测试与验证

### 7.1 每项的测试

| 项 | 测试 |
|---|---|
| 两种主题的变量齐全、没有 `accent-foreground`、没有琥珀实心底、没有调色板与写死的色值、颜色类都有映射 | `design-tokens.test.ts` 的五个测试（S1） |
| tailwind-merge 认新的圆角、阴影与字号 | `cn.test.ts`（S1） |
| 危险按钮的样子 | `confirm-dialog.test.tsx:88`、`upgrade-page.test.tsx:1188,1422`（按词认 `text-destructive`，S1） |
| 新配色的对比度不无意中变淡；控件的环是 2 px | pg4、se1、l5 的下限与 se1 的 `ring 2px`（S1） |
| 描边按钮的悬停叠一层 | `button.test.tsx` 的"layers its hover"（S1） |
| 按钮的变体 | `button.test.tsx`（S2 加格）：`"each variant has its look"`——四个 `variant` 各渲染一次，断言 `classList` 含 4.2.2 写的关键类（`bg-primary`；`bg-surface` 与 `hover:hover-layer`；`text-muted-foreground`；`text-destructive` 且不含 `bg-destructive`）；`"an aria-disabled button stays in the tab order"`——`user.tab()` 走到带 `aria-disabled` 的按钮、它有焦点，`classList` 不含 `pointer-events-none`（jsdom 里 `focus()` 进得去 `tabIndex=-1` 的元素，`pointer-events` 的类也不起作用，只用 `focus()` 判抓不到变异 15，审查 C 的 M5）；`disabled` 的按钮 `user.tab()` 跳过它 |
| 下拉换装、无效的样子、令牌期限换成 `NativeSelect` | `native-select.test.tsx` 加 `"it is drawn by the app, its paint contained"`（`classList` 含 `nw-select`、`contain-paint`）与 `"what is invalid shows on the select"`（`aria-invalid` 落在 `<select>` 上，`classList` 含 `aria-invalid:border-destructive`）；`tokens-page.test.tsx:68` 照旧以 `getByLabelText("Expires")` 找到期限 |
| 菜单的危险项、只有进场、键盘时才有环 | `dropdown-menu.test.tsx`（S2）：`"a destructive item reads red, an ordinary one does not"`（`classList`；菜单项没有 `tone` 属性：`not.toHaveAttribute("tone")`）；`"the menu animates in, never out"`——打开之后内容带 `data-state="open"`，`className` 含 `data-[state=open]:animate-pop-in`、不含任何 `data-[state=closed]:`（jsdom 不跑 CSS 动画，Radix 的 `Presence` 在 jsdom 里读到的动画名总是空的，这里只能守类名；真浏览器里由 l5 守，7.2 第 13 条）；`"the item's ring is the keyboard's"`——菜单项的 `classList` 含 `in-data-[modality=keyboard]:focus:ring-2`，`classList.contains("focus:ring-2")` 为假（按类名逐个判：前一个类名本身就含子串 `focus:ring-2`） |
| 对话框只有进场 | `dialog.test.tsx`（S2）：同上"never out"一格，`Dialog` 与 `AlertDialog` 的内容与遮罩各一 |
| 输入的方式 | `input-modality.test.ts`（S2）：`keydown` 之后 `data-modality` 是 `keyboard`；`pointermove`、`pointerdown` 之后是 `pointer`；在 `pointer` 时单按 `Shift`、`Meta`，或按带 `metaKey` 的 `c`，仍是 `pointer`；`Shift` 加 `Tab` 是 `keyboard`；去掉监听之后不再变 |
| 输入框、标签、表单字段、提示条、标签页的键盘与禁用 | 现有的测试：`W/app/form.test.tsx`、`W/pages/sign-in.test.tsx`（表单字段、密码的"显示"按钮）、`W/pages/page/version-page.test.tsx` 与 `W/app/event-stream.test.tsx`（标签页按角色走）。它们的键盘是平台与 Radix 的，S2 只换类名；00 号文档 4.8 的"每个换装的基础组件有 vitest（变体、禁用、键盘）"对这五个由这些现有的测试满足，不另加（审查 C 的 M11） |
| 下拉在三种引擎里的样子、强制色模式、单选与复选的琥珀、减少动效时只淡入 | 不在 jsdom 里测（没有布局与绘制）：4.2.10 与 7.4 的截图和探针，写进 S2 的记录 |

### 7.2 变异

每条在本机改坏、跑它旁边的测试、看到失败，再从备份复原（02-workflow 3.1；先在没改的树上跑一遍同样的命令）。

| # | Step | 改坏 | 抓住它的测试 |
|---|---|---|---|
| 1 | S1 | 亮色 `--accent` 调淡成 `#e0b468`（`--ring` 随之变，取舍 755） | se1 的焦点（亮）、l5 的聚焦框、链接焦点与菜单项（亮） |
| 2 | S1 | 暗色 `--muted-foreground` 调暗成 `#7f8087` | l5 的补全说明（暗） |
| 3 | S1 | 亮色 `--destructive` 调淡成 `#e57373` | pg4（亮，静止与悬停） |
| 4 | S1 | 危险按钮悬停 `hover:bg-destructive/30` | pg4（悬停） |
| 5 | S1 | 暗色 `--nw-mark-background` 改成不透明的 `#d9a84e` | se1 的命中字（暗，正文 7:1） |
| 6 | S1 | 全局的 `:focus-visible` 外框去掉 | se1 的命中链接（`outline solid` 或"no focus indicator"）；阅读视图的链接有自己的外框，l5 不管这条（审查 A 的 N1、C 的 N1） |
| 7 | S1 | `.dark` 里删掉 `--sidebar` | `design-tokens.test.ts` 测试 1 |
| 8 | S1 | `nav-item.tsx` 的当前项改回 `bg-accent` | 测试 3 |
| 9 | S1 | 对话框的遮罩改回 `bg-black/50` | 测试 4 |
| 10 | S1 | `link-completion.ts:132` 写回 `var(--accent-foreground)` | 测试 2 |
| 11 | S1 | 补全选中项去掉内嵌的环（`boxShadow`） | l5 的选中项（`indicatorContrast` 抛"no focus indicator: none"） |
| 12 | S2 | `NativeSelect` 去掉 `contain-paint` | `native-select.test.tsx` 的"paint contained"；se1 的 `@narrow` 格（`webkit-narrow`，`expectFits`）是否也抓得到，由 4.2.10 的 WebKit 探针定，写进记录 |
| 13 | S2 | 菜单内容加 `data-[state=closed]:animate-fade-in` 当作退场 | `dropdown-menu.test.tsx` 的"never out"；l5 关上菜单之后一帧之内没有 `menu`（退场的动画名与进场的不同，`Presence` 走 `ANIMATION_OUT`，菜单留着，审查 B 核对过） |
| 14 | S2 | 危险项的 `tone` 不加 `text-destructive` | `dropdown-menu.test.tsx` 的危险项 |
| 15 | S2 | 按钮加 `aria-disabled:pointer-events-none` 并给 `aria-disabled` 的按钮 `tabIndex=-1` | `button.test.tsx` 的 `aria-disabled` 格 |
| 16 | S1 | `cn.ts` 去掉 `radius` 的登记 | `cn.test.ts` 的圆角格 |
| 17 | S1 | 一处 `bg-surface-2` 改回 `bg-muted` | 测试 5 |
| 18 | S1 | 一处类名写成 `bg-[#000]` | 测试 4 |
| 19 | S1 | `button.tsx` 的环改回 `ring-[3px]` | se1 的"搜索"按钮 `ring 2px` |
| 20 | S2 | 菜单项的环改成不带前缀的 `focus:ring-2` | `dropdown-menu.test.tsx` 的"keyboard's" |
| 21 | S1 | 描边按钮的悬停改回 `hover:bg-hover` | `button.test.tsx` 的"layers its hover"（S1 建） |
| 22 | S2 | `trackInputModality` 不听 `pointermove` | `input-modality.test.ts` |
| 23 | S2 | 修饰键也算键盘 | `input-modality.test.ts` 的修饰键格 |
| 24 | S2 | 控件或菜单项去掉 `focus-visible:outline-hidden`（审查之后加，下同） | `button.test.tsx` 的"each variant"与对比视图那一格；`dropdown-menu.test.tsx` 的"keyboard's" |
| 25 | S2 | 菜单项另加 `focus-visible:ring-2`、`data-highlighted:ring-2` 或 `in-data-[modality=pointer]:focus:ring-2` | "keyboard's"：环宽的类恰好一个 |
| 26 | S2 | 两处"删除"去掉 `tone` 或前面的分隔线 | `page-tree-writes.test.tsx`、`attachments-section.test.tsx` |
| 27 | S2 | 令牌期限换回裸的 `<select>` | `tokens-page.test.tsx` |
| 28 | S2 | 减少动效时 `--animate-pop-in` 仍是 `nw-pop-in` | `design-tokens.test.ts` 的减少动效一格 |
| 29 | S2 | `stop()` 去掉 `keydown` 时不带 `capture` | `input-modality.test.ts` 停下的一格 |
| 30 | S2 | `cn.ts` 去掉 `animate` 的登记 | `cn.test.ts` |
| 31 | S2 | 对比视图的失败提示换回 S2 之前的类 | `alert.test.tsx` 的拷贝一格 |

00 号文档 4.8 要求的 P1 那条（"把 `--faint` 或 `--accent` 调淡，se1 或 l5 变红"）是第 1 条。

### 7.3 门禁

- **每个提交**：`make lint-web`、`make knip`、`make test-web`；改到故事的另跑那几个故事（Chromium）。
- **合并之前**（取舍 761）：在 Step 分支并入 `m13-ui-refresh` 的树上经 `heavy.sh` 跑 `CI=1 make lint-web knip test-web gen-check-web build-web` 与 `CI=1 make e2e`（三种浏览器与 `webkit-narrow`，`check.mjs` 已在 `lint-web` 里）；`git diff --quiet m13-ui-refresh...HEAD -- server api deploy .github testdata e2e/fixtures/db.ts ':(glob)**/*.go'` 以 0 退出（Go 的测试还读 `deploy/`、`.github/workflows/`、`testdata/` 与 `e2e/fixtures/db.ts`；`--stat` 不论有没有差异都以 0 退出，审查 C 的修订核对第 1 轮 N1）；结果写进 Step 的记录。
- **没有重现的 CI 项**（记录照抄这张表，写明理由）：Go 一侧的全部（`lint-go`、`gen-check-go`、竞态、组合根八份、成本与守卫：Go 的树没变）；`lint-actions`（只在 runner 上）；`deploy-check`、`image-smoke`、`release.yml` 的演练、`vuln.yml`（P1 不改部署、镜像与依赖）；Linux 与 macOS 的差别（引擎的画法、计时）。负责人推送之后 CI 在最后一个头上补跑，红了照 02-workflow 3.1 先停、查清。
- **文档**：`python3 docs/v0.1/claude_to_codex/tools/check_links.py origin/m13-ui-refresh`（远端的分支不在时用 `m13-ui-refresh`）、`git diff --check`。

### 7.4 视觉的核对

B 审查者（00 号文档 4.8）用 Playwright 截取下面这些，亮暗、1440 与 375 各一张，在本机对照样稿看，不进仓库，记录写明看了哪些：

- **S1**：工作区首页、一个页面的阅读视图、页面的编辑、搜索结果、删除的确认框、页面菜单、暗色下描边按钮的悬停（在页面上、在对话框里）。P1 不改布局，看的是颜色、焦点圈（键盘 Tab 过去）、危险按钮与描边按钮。
- **S2**：同上，加个人资料页（单选）、新建令牌的对话框（下拉，含无效）、版本页（标签页，英文 375 px）、树的行菜单（带危险项）；下拉在 Firefox、WebKit 各一张，Chromium 强制色模式一张。
- **S2 的探针**（审查 C 的 M10）：`emulateMedia({ reducedMotion: "reduce" })` 之后打开一个菜单，读计算之后的 `animationName`，应为 `nw-fade-in`；不减少时为 `nw-pop-in`；关上之后菜单已不在。

### 7.5 CI 的余量

不加故事。S2 在 l5 多等一帧；进场动画让打开菜单与对话框的点按多等至多 150 ms 的"稳定"，全部故事里打开菜单与对话框的次数以百计，推断多出不到一分钟，远在 E2E 一步 18 分钟的时限之内（00 号文档第 9 节：今天约 6 分 21 秒）。S2 之后照实记一次时间。

## 8. 完成标准

### 8.1 条件

1. 4.1、4.2 的每一项做到；7.1 的测试通过；7.2 的变异都被抓住。
2. `design-tokens.test.ts` 为绿：`accent-foreground` 一处不留，没有琥珀实心底，没有调色板与写死的色值（除第 32 条的例外），颜色类都有映射（00 号文档 4.1"`grep accent` 的核对"）。
3. pg4、se1、l5 的下限与实量值写进 S1 的记录；正文的两处下限是 7。
4. 7.3 的门禁为绿（含第 0 步的基线）；没有服务端与契约的改动。
5. 7.4 的截图与探针看过，与样稿不同之处写进记录（是有意的，写理由；不是的，修）。
6. Phase 完成（02-workflow 3.7）：00 号文档第 12 节 P1 一行"完成"、头的状态、第 13 节一行；本文头的状态。

### 8.2 总体设计的新文字

S1 的文档提交里改第 1–5 条，S2 的改第 6 条（脚本逐条替换，断言旧文字恰好出现一次）：

1. **9.1 技术栈**，句末"多语言只有 `zh-CN` 与 `en`。"之后加：

   > 界面的设计基础（颜色、圆角、阴影、字体与字号）是 `styles.css` 里的 CSS 变量，亮暗两套，组件只用语义名（13.2 第 32 条，M13）。

2. **13.2 第 31 条**，把"焦点指示对相邻的颜色至少 3:1：……（暗色悬停 `dark:hover:bg-destructive/50`）。"整段换成：

   > 焦点只在键盘操作时出现（`:focus-visible`），是实色 2 px 的琥珀（`--ring` 取 `var(--accent)`，M13）：链接与别的元素是 2 px 的实色外框、偏移 2 px，原生的单选与复选同样；控件（按钮、输入框、下拉、标签页）是零偏移、零模糊的 2 px `ring-ring` 环（`ring-2`；Playwright 的 WebKit 不画原生下拉的 `box-shadow`，它那里只有聚焦时变成 `ring` 色的 1 px 边框）；编辑器是例外，聚焦时（用鼠标点进去也是）有 2 px 的琥珀外框；无效的字段是实色的 `ring-destructive`；菜单项、快速切换的选项与补全的选中项另有看得见的指示（内嵌的 2 px `ring-ring` 环），不只靠底色。对比度按"视觉优先"（负责人 2026-11-22，00-M13 4.7）：正文的字对它的底至少 7:1；destructive 的红字、补全的说明字与焦点、指示的环不设统一的门槛，由 pg4、se1、l5 三个故事按实量的值减 0.1 守住，防止无意中变淡（实量值与下限在 M13/P1/S1 的记录里；之后改了被量元素的底或颜色的 Step 照同一规则重量）；说明文字（`--faint`）不量。destructive 的按钮是红字描边，不再是白字红底。

3. **13.2 新的第 32 条**，接在第 31 条之后：

   > 32. **设计基础的变量**（M13/P1）：颜色、圆角、阴影、字体与字号是 `styles.css` 里的 CSS 变量（亮在 `:root`，暗在 `.dark`；阅读视图的在 `reading.css`）。颜色与阴影经 Tailwind 的 `@theme inline` 映射成工具类（`bg-surface`、`bg-hover`、`bg-active`、`text-faint`、`shadow-popover`……）；与主题无关的圆角与字号在 `@theme` 里（`rounded-row`、`rounded-card`、`text-row`……），并登记进 `lib/cn.ts` 的 tailwind-merge，调用处的覆盖才按"后写的赢"。组件只用这些语义名，不写色值，也不用 Tailwind 的调色板（`bg-black/50` 之类）。`--accent` 是琥珀，只标状态（勾选、当前、焦点、链接下划线），没有琥珀实心底上的字，所以没有 `--accent-foreground`；`--ring` 取 `var(--accent)`；悬停的底是 `--hover`（有自己的底的元素用 `hover-layer` 叠一层），当前与选中的底是 `--active`，编辑器的选区是 `--accent-soft`。例外：mermaid 的 `themeVariables` 只收十六进制，在 `reading/diagrams.ts` 里另写一份亮暗的色值；`editor/theme.ts` 的链接色、搜索匹配色与语法色，`reading.css` 的 hljs 与 callout 各色可以写色值（P4 起注释写明与 00-M13 4.1 的表同源）。`design-tokens.test.ts` 守住其中可检查的部分（两种主题的变量齐全、没有 `accent-foreground`、没有琥珀实心底、没有调色板与写死的色值、颜色类都有映射）。

4. **13.4 第 3 条**，"对比度由 `e2e/fixtures/contrast.ts` 从计算之后的颜色算，量之前先结束元素与它祖先的过渡（M12/P2）。"之后加：

   > M13/P1/S1：pg4、se1、l5 的门槛是新配色的回归守卫：正文的字至少 7:1，其余按实量的值减 0.1；se1 另断言控件的环是 `ring 2px`。

5. **第 15 节**变更记录加一行（照邻行的写法）：`- 2026-11-2x：M13/P1/S1（依据 [P1 设计](M13-ui-refresh/01-P1-foundation.md)）：9.1 加设计基础一句；13.2 第 31 条的焦点与对比度改写，新第 32 条"设计基础的变量"；13.4 第 3 条加对比度守卫一句。`（日期照合并的那天）

6. **S2 的改动**：
   - 9.1 的"设计基础（颜色、圆角、阴影、字体与字号）"改为"设计基础（颜色、圆角、阴影、字体、字号与动效）"，第 32 条的"（M13/P1）：颜色、圆角、阴影、字体与字号是"改为"（M13/P1）：颜色、圆角、阴影、字体、字号与动效是"（锚点各带上下文：S1 之后同一串字出现两次，审查 A 的修订核对第 1 轮 N3），第 32 条另加一句"进场动画只有进场、没有退场（`data-[state=open]:` 之下；`prefers-reduced-motion` 时只淡入）"；
   - 第 31 条的"菜单项、快速切换的选项与补全的选中项另有看得见的指示（内嵌的 2 px `ring-ring` 环）"改为"快速切换的选项与补全的选中项另有看得见的指示（内嵌的 2 px `ring-ring` 环），菜单项在键盘操作时同样（`app/input-modality.ts`；用鼠标时只有底色）"；
   - 4.2.10 的结论：WebKit 画出了环时删掉括号里"Playwright 的 WebKit 不画……1 px 边框"一句，没画时照旧并注明"macOS 上看过，M13/P1/S2"；
   - 第 15 节加一行 S2 的。

## 9. 交给后面的 Phase

- **P2**：外框用 `--sidebar`、`--hover`、`--active`、`--accent-soft`（当前页的行）、`rounded-row`、`shadow-popover`、`text-row`；按钮的 30 px（`size="sm"`、`icon-sm`）与胶囊形、头像菜单的子菜单与快捷键的部件（2.1 第 2 条，一并在 `button.test.tsx`、`dropdown-menu.test.tsx` 加格）；把搜索框挪到 `--sidebar` 上时，se1 的焦点与无效框的下限照 4.1.8 重量（琥珀对 `#f5f5f6` 约 2.74，红约 4.02，审查 A 的 M3；那是换色之前的 `#c58a22`）；亮色下搜索框（`surface-2`）与"搜索"按钮（`surface`）不同色，挪的时候一并处理（S2 的记录）。
- **P3**：设置的卡片 `rounded-card border bg-surface`（不加阴影，00 号文档 4.1）；30 px 的下拉用 P2 加的尺寸或 `className="h-[30px]"`；文件输入框的按钮悬停时没有反馈（S2 给了它底与边框，原生的悬停没了），暗色的底取 `--active` 还是 `--hover` 一并再看（S2 的记录，S2B2-N1、Q1）。P3/S2 给 `Input` 加了 `file:hover:hover-layer`（亮色 `--surface`、暗色 `--active` 的底上叠 `--hover`，暗色的底照旧 `--active`），三种引擎亮暗截图看过；开放程度的下拉是 30 px（`h-7.5`）（[03-P3](03-P3-settings.md) 决定 830，[P3/S2 的审查记录](reviews/P3-S2-workspace-notebook-settings-review.md)）。
- **P4**：阅读视图的其余变量（第 2 节第二段）；已勾的禁用任务项怎样显示（4.2.5）；编辑器的链接色与搜索匹配色（`W/editor/theme.ts:15-26`），选区已在 S1 改成 `--accent-soft`；`reading.css` 的写死的色值加"同源"的注释。P4/S1 不做这四项：阅读视图的其余变量、已勾的禁用任务项与"同源"的注释在 P4/S2，编辑器的两种颜色在 P4/S3（[04-P4](04-P4-reading.md) 第 6 节）。P4/S2 做了其中三项：阅读视图的其余变量（`reading.css` 最外层的 `:root`/`.dark` 各 21 个，`design-tokens.test.ts` 核对两边相同，决定 870）；已勾的禁用任务项（原生的复选框照旧，已完成项自己的字淡并划线；禁用的勾 Chromium、Firefox 灰，WebKit 淡琥珀，决定 872）；`reading.css` 写死的色值的"同源"注释（决定 870）（[04-P4](04-P4-reading.md) 10.2，[P4/S2 的审查记录](reviews/P4-S2-body-code-diagrams-review.md)）。编辑器的两种颜色在 P4/S3。P4/S3 做了最后一项：编辑器的链接色与搜索匹配色（决定 875）。`W/editor/theme.ts` 不再写色值：搜索的匹配用阅读视图的高亮 `--nw-mark-background`，当前一项用 `reading.css` 新加的 `--nw-match-selected`（亮暗成对）；链接是正文色，下划线是 `--nw-link-line`，地址与标记是 `--faint`（`markdown.ts`、`prose-lines.ts`）；CodeMirror 自带的查找面板、输入框、按钮与对比视图的 gutter 原是亮色的样子，也只用变量另写（决定 910）；`design-tokens.test.ts` 写色值的白名单只剩 `W/reading/diagrams.ts`（[04-P4](04-P4-reading.md) 10.3，[P4/S3 的审查记录](reviews/P4-S3-editor-outline-save-review.md)）。P4 那一条的四项都做完了。
- **P5**：空状态与骨架（2.1 第 1 条）。P5/S1 建了空状态的组件（`W/components/empty-state.tsx` 的 `EmptyState`：`aria-hidden` 的方块里一个图标、一句说明在 `p` 里、可选的主操作（调用者给的按钮，与状态页的居中操作一样是 `size="sm"` 的胶囊，决定 949）；没有标题，不是 `status`；`page`、`card` 两种大小），用在两个首页与设置里的五个空列表（令牌、邀请、无主笔记本、审计、任务），S2 再用在搜索（没提交时）、历史、回收站、目录与标签；骨架不加：P5 的列表第一次读取都短，读取中照旧是一行"加载中…"（[05-P5](05-P5-pages.md) 决定 913、914、938，[P5/S1 的审查记录](reviews/P5-S1-homes-entry-review.md)）。
- **P2–P5**：12 处手写的"Label 加控件"组从 `space-y-2` 改为 `space-y-1.5`，与 `FormField` 同，各在所在页的 Phase 改（审查 B 的 S2B-N3，位置列在[S2 的审查记录](reviews/P1-S2-components-review.md)）。P3/S1 改了 `W/pages/settings/create-token-dialog.tsx:184`（`446d7e99` 的行号；`6987baa9` 上是 `:186`）一处（[03-P3](03-P3-settings.md) 决定 830、846，[P3/S1 的审查记录](reviews/P3-S1-settings-mode-review.md)），P3 的其余五处在 S2。P3/S2 改了列表里的四处（`invitations-section.tsx:130`、`import-dialog.tsx:318,361`、`add-member-section.tsx:132`）与列表外的 `add-member-section.tsx:102`（[P3/S2 的审查记录](reviews/P3-S2-workspace-notebook-settings-review.md)）；列表里的 `move-page-dialog.tsx:159,172` 在 P2/S2（`831e50a6`）就已是 `space-y-1.5`。列表里剩下五处：`asset-dialogs.tsx:221`、`search-page.tsx:221`、`index-filters.tsx:60`、`history-filters.tsx:154`、`source-add-dialog.tsx:261`，在 P4、P5（行号是 `ba66557c` 的；连同 03-P3 决定 846 补上的三处，03-P3 第 9 节）。P4/S1 改了 `W/pages/page/asset-dialogs.tsx:221`（`717f5334`；[04-P4](04-P4-reading.md) 决定 882，[P4/S1 的审查记录](reviews/P4-S1-head-strip-panel-review.md)）；列表里剩下四处，连同 03-P3 补上的三处，在 P5。P5/S2 改了六处：`W/pages/search/search-page.tsx:221`、`W/pages/notebook/index-filters.tsx:48,60`、`W/pages/notebook/history-filters.tsx:132,143,154`（`af33b81e` 的行号）换成 `W/app/page-classes.ts` 的 `filterFieldClass`（`flex min-w-0 max-w-full flex-col gap-1.5`：标签总在控件之上，间距 6 px，同 `FormField` 的 `space-y-1.5`），外面一层是筛选条 `filterBarClass`（[05-P5](05-P5-pages.md) 决定 926，[P5/S2 的审查记录](reviews/P5-S2-lists-review.md)）；列表里剩下 `W/pages/notebook/source-add-dialog.tsx:261` 一处，在 S3。 P5/S3 改了最后一处：`W/pages/notebook/source-add-dialog.tsx:261`（`af33b81e` 的行号；最后的树 `8a89e184` 上是 `:266`）的 `space-y-2` 换成 `space-y-1.5`，粘贴来源的"Markdown"标签与文本框同 `FormField` 隔 6 px（[05-P5](05-P5-pages.md) 决定 926，[P5/S3 的审查记录](reviews/P5-S3-versions-review.md)）。S2 的审查记录列的十二处与 03-P3 决定 846 补上的三处都已改完，这一条做完。

## 10. 结果

### 10.1 S1：token 与主题（2026-11-22，合并 `a08bf50f`）

- **提交**：`5a91cb48`（变量、琥珀、守卫，照 6.1 提交 1）；审查的修复 `f7a79a8d`、`ef0261f6`、`08526c82`，修复核对第 1 轮的 `122e9e96`；加本文档提交。[S1 的审查记录](reviews/P1-S1-tokens-theme-review.md)。
- **做完**：4.1.2 的两套变量（`--ring: var(--accent)`，没有 `--accent-foreground`，`--radius` 0.5rem）、`@theme` 的圆角、字号与字体、`hover-layer`；4.1.3 的 22 处类名与 4 处 `var()`；4.1.4 的 2 px 环；4.1.5 的危险按钮与描边按钮；4.1.6 的面、遮罩、高亮、`bg-muted` 与下拉的底；4.1.9 的 tailwind-merge 登记；守约定的五个测试、`cn` 与按钮的测试；三个故事的下限与 se1 的 `ring 2px`。
- **实量**（Chromium，macOS）：与 4.1.8 的预期之差都不到 0.01；下限 = 实量 − 0.1，正文的两处是 7，逐项在审查记录。
- **变异**：7.2 的 S1 各条都被抓住（实施者跑，审查 C 独立重跑）；审查者另写的变异在修复之后都被抓住，除了有意不查的颜色名（决定 765）。
- **门禁**：第 0 步的基线、每个提交的 `make lint-web`、`make knip`、`make test-web`、合并之前带 `CI=1` 的网页一侧与 `make e2e`（只有允许的三格与两个重跑全过的偶发格）都在审查记录；Go 一侧没有差异。
- **与设计不同之处**：4.1.6、4.1.7 的"照实"；决定 763–765。
- **交给 S2**：暗色下搜索框（透明）与"搜索"按钮（`surface-2`）不同色，S2 给输入框 `surface-2` 之后一致；Firefox、WebKit 与强制色模式的外观在 S2 看。

### 10.2 S2：基础组件（2026-11-22，合并 `e2678ff7`）

- **提交**：`77c0499d`（按钮、输入框与下拉，照 6.2 提交 1）、`3ab4e9fc`（菜单、对话框、标签页、进场动画与只在键盘操作时的环，照 6.2 提交 2）；审查的修复 `f6ef65c8`（亮色琥珀深一档）、`463dd983`（其余修复与收紧的测试）、修复核对第 1 轮的 `67d1e970`；加本文档提交。[S2 的审查记录](reviews/P1-S2-components-review.md)。
- **做完**：4.2.2 的按钮；4.2.3 的输入框（含文件按钮）、标签、表单字段与提示条；4.2.4 的 `nw-select`（两道渐变的细折线、强制色模式退回原生、无效的样子），令牌期限换成 `NativeSelect`；4.2.5 单选与复选的 `accent-color`；4.2.6 的菜单、危险项的 `tone`、"删除"前的分隔线、`W/app/input-modality.ts`；4.2.7 的对话框；4.2.8 的下划线式标签页；4.2.9 的进场动画与减少动效时换的变量；强制色模式下控件的焦点（决定 768）；l5 关上菜单之后一帧之内没有 `menu`。
- **测试**：`button.test.tsx`、`native-select.test.tsx` 加格；新的 `dropdown-menu.test.tsx`、`dialog.test.tsx`、`input-modality.test.ts`；审查之后另在 `design-tokens.test.ts`、`cn.test.ts`、`tokens-page.test.tsx`、`page-tree-writes.test.tsx`、`attachments-section.test.tsx` 加断言，新的 `alert.test.tsx` 守 `Alert` 与对比视图照抄它的失败提示；vitest 5100 → 5118 格。新格先写、先红；`aria-disabled` 一格守的是已有的行为，由变异 15 证明会红。
- **三种引擎里的下拉**（4.2.10，macOS，Playwright 1.63）：三种都画出 2 px 的环，13.2 第 31 条的 WebKit 一句删去（8.2 第 6 条），M12-polish 第 5 项的那一小项写进"处理进展"；WebKit 不带 `contain-paint` 时页宽照旧被撑开（375 宽时 762）；Linux 的 WebKit 由负责人推送之后的 CI 再看。
- **实量**：亮色琥珀深一档之后，se1、l5 量琥珀的亮色下限由 2.88、2.45 重量为 2.99、2.54；其余与 S1 的相同。
- **探针与视觉的核对**（7.4）：减少动效时菜单与对话框只淡入，关上之后一帧都已不在；菜单项的环只在键盘操作时出现，三种引擎相同；Chromium 亮暗 × 1440、375 与下拉的三种引擎、强制色模式都截了；与样稿不同之处（勾的颜色、箭头的粗细、下划线的两端、"删除"前的线、文件按钮）都已修。
- **变异**：7.2 的 S2 各条与第 24–31 条都被抓住（实施者跑，审查 C 独立重跑 S2 各条）；审查者另写的变异都被抓住。
- **门禁**：每个提交的 `make lint-web`、`make knip`、`make test-web` 都绿；合并之前带 `CI=1` 的网页一侧与 `make e2e` 只败允许的三格；E2E 2.4–2.8 min（随负载），与基线的 2.4 min 之差在 7.5 推断的"不到一分钟"之内；Go 一侧没有差异。逐项在审查记录。
- **与设计不同之处**：4.2 的"照实"；决定 766–768。
- **交给后面**：第 9 节（12 处手写标签组的间距，亮色下搜索框与"搜索"按钮不同色，文件按钮的悬停）；已勾的禁用任务项在 P4/S2（4.2.5）。

## 附录：核对过的代码

`3c315037` 上读过：`W/styles.css`（全文）、`W/reading/reading.css:1-70,120-130`、`W/components/ui/*.tsx`（全文）、`W/components/form-field.tsx`、`loading.tsx`、`nav-item.tsx`、`W/lib/cn.ts`、`W/editor/theme.ts:1-41`、`W/editor/text-diff.tsx:368-425`、`W/editor/loaded/link-completion.ts:115-140`、`W/app/confirm-dialog.tsx:75-100`、`W/app/confirm-dialog.test.tsx:80-95`、`W/pages/notebook/upgrade-page.test.tsx:1185-1190,1415-1426`、`W/reading/patch.test.ts:1-14`、`web/apps/web/package.json`、`web/apps/web/vite.config.ts`、`web/apps/web/index.html`、`web/apps/web/public/theme-init.js`、`e2e/fixtures/contrast.ts`（全文）、`e2e/stories/page/pg4-delete-subtree.spec.ts:180-210`、`e2e/stories/search/se1-search.spec.ts:270-380`、`e2e/stories/links/l5-completion.spec.ts:120-185`、`Makefile` 的 `lint-web`、`knip`、`gen-check-web`、`build-web`、`e2e`；`grep` 过的：`accent`、`destructive`、`ring-[3px]`、Tailwind 调色板的类、源码里的色值、`type="radio"`、`type="checkbox"`、`<select`、`DropdownMenuItem`、`TabsTrigger`、`justify-end`、空状态的文字、`bg-muted`。三位审查者另读过的在[审查记录](reviews/P1-design-review.md)。
