```yaml
status: open
from: M13
to: v0.2
created: 2026-11-26
```

# 界面焕新：M13 留给 v0.2 的

M13（[00-M13](../../v0.1/M13-ui-refresh/00-M13-design.md)）只改网页的外观与交互。它自己推后的、各 Phase 设计第 9 节交给 v0.2 的与各 Step"另记给 v0.2"的，写成下面编号的条目（M13 收尾审查 C 的 M1、M2 与 Q2，主持者的决定 962）。[M5-polish](../../v0.1/M12-release/handoffs/M5-polish.md) 的条目里 M13 没有认领的照旧在 [M12-polish.md](M12-polish.md)，它的"处理进展"有"M13 收尾"一行指到这里。每条写现象、出处与候选的改法；细节在出处，不整段复制。

1. **搜索摘要露出 Markdown 原文**：搜索结果的摘要里看得到 `--- tags:`、`**`、`[[…]]` 这样的原文。摘要是服务端截的，要改服务端，不在 M13（00-M13 第 2 节，主持者的决定 721）。
2. **像素级的视觉回归**：M13 的视觉核对靠每个 Step 审查时的截图并排，没有像素对比的门禁（决定 725，00-M13 4.8）。要不要做、用什么工具，v0.2 再定。
3. **列表的骨架**：M13 加了空状态的组件，"加载中"照旧一行字，不加骨架（决定 914，[05-P5](../../v0.1/M13-ui-refresh/05-P5-pages.md) 第 9 节）。以后有分批读的长列表时再看。
4. **l5 量编辑器外框的底**：l5 量编辑器聚焦外框的对比度时，底取父元素链上的（`e2e/fixtures/contrast.ts:112-116`；决定 891，[04-P4](../../v0.1/M13-ui-refresh/04-P4-reading.md) 第 9 节）。候选的改法：量元素自己的底。
5. **mermaid 的主题与作者的图**：`themeVariables`、`themeCSS` 照旧可被图自己的指令改（决定 871；指令给的 `themeCSS` 经 mermaid 的 `sanitizeCss`）。作者自己指定浅色主题（`forest`、`neutral`、`default`，或 `base` 加浅色的变量）的图在暗色应用里，直接画在图框底上的边、箭头与时序图的消息字是深色，看不清（M13 之前也这样；P4/S2，04-P4 第 9 节）。候选的改法：图框的底跟着作者的主题。
6. **代码块的头与页内查找、焦点**：页内查找会找到代码块头里的语言名与"复制"，复制之后的 1.5 秒里也找得到藏起的"已复制"（Firefox 的 `window.find` 找得到，Chromium、WebKit 找不到，真的查找栏没有试；决定 869）。同一个顶层块里有几段代码时，在拿着焦点的那段之前增删一段，就地更新之后焦点落到别一段的"复制"上（决定 905）。候选的改法：键先认代码的文字、再认第几个（P4/S2，04-P4 第 9 节）。
7. **带锚点打开的页与"编辑于现在"**：带锚点打开的页，视图第一次到时焦点给锚点，不看读者其间的动作（决定 900，`web/apps/web/src/pages/page/view-anchors.ts:72-80`；总体设计 13.2 第 26 条要异步的答复之后移动焦点时看读者动没动）。刚编辑过的页，头一行读作"<名字> 编辑于现在""Edited by <名字> now"（`page.editedBy` 加相对时间的措辞，"刚刚编辑""just now"更顺）（P4/S1，04-P4 第 9 节）。
8. **编辑器的远跳与 Firefox 的警告**：长文档进编辑之后第一次远跳，Chromium 里一帧移约 157 px，是 CodeMirror 对没量过的行的高度估计在校正，照旧。跳完之后标题停在窗口之外的那一种 M13 收尾修了（M13 收尾审查 B 的 I1）：CodeMirror 报行高变了就再对准，至多 4 次、1 秒之内，读者滚动、按下、触摸或按键就停，对准过之后来了不是它自己引起的滚动也停（`web/apps/web/src/editor/source-editor.tsx` 的 `realign` 与 `scrolled`；4 次与 1 秒是实施者定的，很慢的机器上可能不够；后一条是修复核对第 1 轮补的，审查 B 的 F1-N1）；Chromium 与 WebKit 里有几次第一帧还在旧处、第二帧才对准。一次滚动算它自己引起的，要那一行在窗口顶下 0–48 px、离它上次更新时的位置不到 1 px，或窗口到了底、那一行在窗口里（页尾一节的行到不了顶；修复核对第 2 轮补的，审查 B 的 F2-N1、审查 C 的 F2-N1；"那一行在窗口里"是第 3 轮补的，审查 C 的 F3-N1）。其中四处只由推理守着：拿掉"离上次更新时的位置不到 1 px"、拿掉"窗口到了底"、到位之前的滚动也让它停、更新时不记下位置，这四个变异 PG17 的两格在三种浏览器里都过；只有一条成立的情形只在量行高与再对准之间的一两帧里，造不出确定的格（两条一起拿掉就是修之前的代码，PG17 那一格三种浏览器 9 次全红）。PG17 用 `window.scrollBy` 代替页内查找与读屏的滚动，真的查找栏与读屏没有看过。Firefox 编辑时滚长文档报"scroll-linked positioning effect"（M13 之前就有）：长页跳下去之后按"完成"，与 375 宽时从页下面的大纲往上跳，都稳定出现，PG17 跳转的那一格因此每跳一次重新打开页（P4/S3，04-P4 第 9 节）。
9. **决定 958 没改的七行**：`web/apps/web/src/pages/settings/create-token-dialog.tsx:146`、`web/apps/web/src/pages/page/unsaved-guard.tsx:88`、`web/apps/web/src/pages/page/asset-dialogs.tsx:235`、`web/apps/web/src/pages/page/version-restore.tsx:199`、`web/apps/web/src/pages/workspace/create-notebook-dialog.tsx:132`、`web/apps/web/src/pages/notebook/source-add-dialog.tsx:285`、`web/apps/web/src/pages/notebook/move-page-dialog.tsx:193`（`f5d84965` 的行号）是 `flex justify-end gap-2` 的按钮行，没有 `flex-wrap`，今天在 320 宽中英都放得下（05-P5 第 9 节；M13 收尾审查 C 的 N5）。以后改这些对话框的按钮文字或加按钮时，照 958 加 `flex-wrap`，在 320 宽量一次。
10. **可能用手指点的设备没有在真机上看过**：行内的次要操作（决定 953）与左栏的齿轮、＋、树行的"⋯"（决定 966）在 `any-pointer-coarse` 下静止也显着。主指针能悬停、另有手指的设备（二合一、触屏笔记本）只按代码推：Chromium 的 CDP 模拟不了只有 `any-pointer` 是 coarse 的设备，P5/S2 的审查 B 以 `--blink-settings` 起的浏览器量过（05-P5 第 9 节；M13 收尾审查 A 的 N1 与未核实的一条）。有这样的设备时看一次。
11. **成对的按键重复没有核实**：`swallowHeldKeys`（决定 967）认的是带 `event.repeat` 的 keydown，遇到放开一个键（keyup）或新按下 Enter、空格就结束；有的 Linux X11 配置把自动重复发成一对对的 keyup 与 keydown，第一个 keyup 就结束了它，之后的重复照样按到焦点回去的按钮（菜单项第一次按下时也调它，自己那一道同样只认 `event.repeat`）。没有这样的环境核实（M13 收尾修复核对第 1 轮，审查 A 的未核实一条）。有这样的环境时按读屏清单 H 第 22 项的做法看一次。

## 处理进展

- M13 收尾（2026-11-26）：建这一份，收 00-M13 第 2 节与第 7 节"写给后面的 M"的两件、04-P4 与 05-P5 第 9 节交给 v0.2 而没有去处的、P4 各 Step 记在 M12-polish 处理进展里的六件（那里的几句照旧留着），05-P5 第 9 节收尾没改的一件（决定 958 的七行），与收尾新进的两件（第 10、11 项）。05-P5 第 9 节收尾的另四件（齿轮的 `any-pointer-coarse`、长面包屑单独一行的"/"、按住的键在对话框关上之后、`expectFits` 在弹入中量）收尾修了，不在这里（决定 966、967、968；收尾记录 `docs/v0.1/M13-ui-refresh/reviews/M13-closeout-review.md`）。v0.2 的总体设计开工时与 M12 的五份一起先处理它。
